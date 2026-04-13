package modem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"smsdock/packages/api/internal/model"
)

type Worker struct {
	repo      Repository
	discovery Discovery

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	modemMu sync.RWMutex
	modem   model.Modem

	stateMu sync.RWMutex
	state   model.ModemRuntime

	opMu sync.Mutex

	adapter Adapter

	failures     int
	lastEventKey string
	lastEventAt  time.Time
	lastBoundAt  time.Time
	currentPath  string
}

func NewWorker(modemRecord model.Modem, repo Repository, discovery Discovery) *Worker {
	ctx, cancel := context.WithCancel(context.Background())
	return &Worker{
		repo:      repo,
		discovery: discovery,
		ctx:       ctx,
		cancel:    cancel,
		done:      make(chan struct{}),
		modem:     modemRecord,
		state: model.ModemRuntime{
			Status: model.ModemStatusOffline,
		},
	}
}

func (w *Worker) Start() {
	go w.loop()
}

func (w *Worker) Stop() {
	w.cancel()
	<-w.done
}

func (w *Worker) UpdateConfig(modemRecord model.Modem) {
	w.modemMu.Lock()
	defer w.modemMu.Unlock()
	w.modem = modemRecord
}

func (w *Worker) Runtime() model.ModemRuntime {
	w.stateMu.RLock()
	defer w.stateMu.RUnlock()
	return w.state
}

func (w *Worker) ScanNetworks(ctx context.Context) ([]model.NetworkOption, error) {
	w.opMu.Lock()
	defer w.opMu.Unlock()

	modemRecord := w.currentModem()
	previous := w.Runtime()
	w.setRuntime(func(state *model.ModemRuntime) {
		state.Status = model.ModemStatusScanning
	})
	_ = w.appendEvent(ctx, model.EventLevelInfo, "network_scan_started", "Manual network scan started", "")

	defer w.setRuntime(func(state *model.ModemRuntime) {
		state.Status = previous.Status
	})

	if err := w.ensureBound(ctx, modemRecord); err != nil {
		return nil, err
	}

	scanCtx, cancel := w.networkOperationContext(ctx, modemRecord)
	defer cancel()

	networks, err := w.adapter.ScanNetworks(scanCtx)
	if err != nil {
		w.fail(ctx, "network_scan_failed", err)
		return nil, err
	}
	_ = w.appendEvent(ctx, model.EventLevelInfo, "network_scan_finished", fmt.Sprintf("Found %d networks", len(networks)), "")
	return networks, nil
}

func (w *Worker) SelectNetwork(ctx context.Context, mccMnc string) error {
	w.opMu.Lock()
	defer w.opMu.Unlock()

	modemRecord := w.currentModem()
	if err := w.ensureBound(ctx, modemRecord); err != nil {
		return err
	}

	w.setRuntime(func(state *model.ModemRuntime) {
		state.Status = model.ModemStatusScanning
	})
	defer w.setRuntime(func(state *model.ModemRuntime) {
		state.Status = model.ModemStatusReady
	})

	selectCtx, cancel := w.networkOperationContext(ctx, modemRecord)
	defer cancel()

	if err := w.adapter.SelectNetwork(selectCtx, mccMnc); err != nil {
		w.fail(ctx, "network_select_failed", err)
		return err
	}

	modemRecord.AssignedNetworkMccMnc = mccMnc
	modemRecord.LastError = ""
	if _, err := w.repo.UpdateModem(ctx, modemRecord); err != nil {
		return err
	}
	w.UpdateConfig(modemRecord)
	_ = w.appendEvent(ctx, model.EventLevelInfo, "network_selected", "Network changed manually", fmt.Sprintf(`{"mccMnc":"%s"}`, mccMnc))
	return nil
}

func (w *Worker) networkOperationContext(ctx context.Context, modemRecord model.Modem) (context.Context, context.CancelFunc) {
	timeout := time.Duration(modemRecord.ScanTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

func (w *Worker) loop() {
	defer close(w.done)
	defer w.closeAdapter()

	for {
		modemRecord := w.currentModem()
		if err := w.pollOnce(modemRecord); err != nil && w.ctx.Err() == nil {
			// pollOnce already persisted failure state
		}

		interval := time.Duration(modemRecord.PollIntervalSec) * time.Second
		if interval <= 0 {
			interval = 10 * time.Second
		}

		timer := time.NewTimer(interval)
		select {
		case <-w.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (w *Worker) pollOnce(modemRecord model.Modem) error {
	w.opMu.Lock()
	defer w.opMu.Unlock()

	ctx, cancel := context.WithTimeout(w.ctx, time.Duration(modemRecord.ATTimeoutMs)*time.Millisecond*6)
	defer cancel()

	if err := w.ensureBound(ctx, modemRecord); err != nil {
		w.fail(ctx, "bind_failed", err)
		return err
	}

	now := time.Now().UTC()
	w.setRuntime(func(state *model.ModemRuntime) {
		state.Status = model.ModemStatusPolling
		state.LastPollAt = &now
	})

	status, err := w.adapter.Status(ctx)
	if err != nil {
		w.fail(ctx, "status_failed", err)
		return err
	}
	lastSeen := time.Now().UTC()
	w.setRuntime(func(state *model.ModemRuntime) {
		state.CurrentNetworkMccMnc = status.CurrentNetworkCode
		state.CurrentNetworkName = status.CurrentNetworkName
		state.SignalStrength = status.SignalStrength
		state.SIMState = status.SIMState
	})
	if err := w.repo.UpdateModemRuntime(ctx, modemRecord.ID, model.ModemStatusPolling, "", &lastSeen); err != nil {
		return err
	}

	messages, err := w.adapter.PollSMS(ctx, modemRecord.SMSReadStorage)
	if err != nil {
		w.fail(ctx, "poll_failed", err)
		return err
	}

	for _, message := range messages {
		storageIndex := message.StorageIndex
		record := model.SMSMessage{
			ModemID:        modemRecord.ID,
			Storage:        model.NormalizeSMSStorage(message.Storage),
			StorageIndex:   &storageIndex,
			Sender:         message.Sender,
			Body:           message.Body,
			Encoding:       message.Encoding,
			RawPDU:         message.RawPDU,
			ModemTimestamp: message.Timestamp,
			ReceivedAt:     time.Now().UTC(),
			MultipartRef:   message.MultipartRef,
			MultipartPart:  message.MultipartPart,
			MultipartTotal: message.MultipartTotal,
			DedupeKey:      w.dedupeKey(modemRecord.ID, message),
		}
		if err := w.repo.SaveSMS(ctx, record); err != nil {
			w.fail(ctx, "save_sms_failed", err)
			return err
		}
	}

	if err := w.cleanupStoredSMS(ctx, modemRecord, messages); err != nil {
		w.fail(ctx, "cleanup_sms_failed", err)
		return err
	}

	successAt := time.Now().UTC()
	w.failures = 0
	w.setRuntime(func(state *model.ModemRuntime) {
		state.Status = model.ModemStatusReady
		state.LastSuccessAt = &successAt
	})
	if err := w.repo.UpdateModemRuntime(ctx, modemRecord.ID, model.ModemStatusReady, "", &successAt); err != nil {
		return err
	}

	return nil
}

func (w *Worker) ensureBound(ctx context.Context, modemRecord model.Modem) error {
	if w.adapter != nil {
		return nil
	}

	w.setRuntime(func(state *model.ModemRuntime) {
		state.Status = model.ModemStatusBinding
	})
	if err := w.repo.UpdateModemRuntime(ctx, modemRecord.ID, model.ModemStatusBinding, "", modemRecord.LastSeenAt); err != nil {
		return err
	}

	adapter, info, err := w.discovery.BindByIMEI(ctx, modemRecord.IMEI, time.Duration(modemRecord.ATTimeoutMs)*time.Millisecond)
	if err != nil {
		return err
	}
	w.adapter = adapter
	w.lastBoundAt = time.Now().UTC()
	w.currentPath = info.Path
	w.setRuntime(func(state *model.ModemRuntime) {
		state.Status = model.ModemStatusReady
		state.CurrentNetworkMccMnc = info.CurrentNetworkCode
		state.CurrentNetworkName = info.CurrentNetworkName
		state.SignalStrength = info.SignalStrength
		state.SIMState = info.SIMState
	})
	lastSeen := time.Now().UTC()
	if err := w.repo.UpdateModemRuntime(ctx, modemRecord.ID, model.ModemStatusReady, "", &lastSeen); err != nil {
		return err
	}

	return w.appendEvent(ctx, model.EventLevelInfo, "modem_bound", "Modem bound to runtime", fmt.Sprintf(`{"path":"%s"}`, info.Path))
}

func (w *Worker) fail(ctx context.Context, eventType string, err error) {
	w.failures++
	status := model.ModemStatusRecovering
	if w.failures >= 3 {
		status = model.ModemStatusDegraded
	}
	message := err.Error()
	now := time.Now().UTC()

	w.setRuntime(func(state *model.ModemRuntime) {
		state.Status = status
	})

	_ = w.repo.UpdateModemRuntime(ctx, w.currentModem().ID, status, message, &now)
	_ = w.rateLimitedEvent(ctx, eventType+":"+message, model.EventLevelWarn, eventType, message, "")

	w.closeAdapter()
}

func (w *Worker) rateLimitedEvent(ctx context.Context, key string, level model.EventLevel, eventType, message, payload string) error {
	if w.lastEventKey == key && time.Since(w.lastEventAt) < 30*time.Second {
		return nil
	}
	w.lastEventKey = key
	w.lastEventAt = time.Now()
	return w.appendEvent(ctx, level, eventType, message, payload)
}

func (w *Worker) appendEvent(ctx context.Context, level model.EventLevel, eventType, message, payload string) error {
	return w.repo.AppendEvent(ctx, model.ModemEvent{
		ModemID:     w.currentModem().ID,
		Level:       level,
		Type:        eventType,
		Message:     message,
		PayloadJSON: payload,
	})
}

func (w *Worker) currentModem() model.Modem {
	w.modemMu.RLock()
	defer w.modemMu.RUnlock()
	return w.modem
}

func (w *Worker) setRuntime(update func(*model.ModemRuntime)) {
	w.stateMu.Lock()
	defer w.stateMu.Unlock()
	update(&w.state)
}

func (w *Worker) closeAdapter() {
	if w.adapter != nil {
		_ = w.adapter.Close()
		w.adapter = nil
	}
}

func (w *Worker) dedupeKey(modemID string, message ReceivedSMS) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%v|%s", modemID, message.Sender, message.RawPDU, message.Timestamp, message.DedupeKeySuffix)))
	return hex.EncodeToString(hash[:])
}

func (w *Worker) cleanupStoredSMS(ctx context.Context, modemRecord model.Modem, messages []ReceivedSMS) error {
	if len(messages) == 0 {
		return nil
	}

	usage, err := w.adapter.SMSStorageStatus(ctx, modemRecord.SMSReadStorage)
	if err != nil {
		return err
	}
	thresholdSlots := thresholdSlots(usage.Total, modemRecord.SMSDeleteThresholdPct)
	if usage.Used <= thresholdSlots {
		return nil
	}

	deleteCount := usage.Used - thresholdSlots
	if deleteCount > len(messages) {
		deleteCount = len(messages)
	}
	if deleteCount <= 0 {
		return nil
	}

	candidates := append([]ReceivedSMS(nil), messages...)
	sort.SliceStable(candidates, func(i, j int) bool {
		left := smsSortTime(candidates[i])
		right := smsSortTime(candidates[j])
		if !left.Equal(right) {
			return left.Before(right)
		}
		return candidates[i].StorageIndex < candidates[j].StorageIndex
	})

	for _, message := range candidates[:deleteCount] {
		if err := w.adapter.DeleteSMS(ctx, message.Storage, message.StorageIndex); err != nil {
			return err
		}
	}

	return w.rateLimitedEvent(
		ctx,
		fmt.Sprintf("sms_cleanup:%s:%d:%d:%d", usage.Storage, usage.Used, usage.Total, thresholdSlots),
		model.EventLevelInfo,
		"sms_cleanup",
		fmt.Sprintf("Cleaned %d SMS from %s to keep storage usage at %d/%d", deleteCount, usage.Storage, thresholdSlots, usage.Total),
		fmt.Sprintf(`{"storage":"%s","deleted":%d,"used":%d,"total":%d,"threshold":%d}`, usage.Storage, deleteCount, usage.Used, usage.Total, thresholdSlots),
	)
}

func thresholdSlots(total int, percent int) int {
	if total <= 0 {
		return 0
	}
	percent = model.NormalizeSMSDeleteThresholdPct(percent)
	return max(1, int(math.Ceil(float64(total*percent)/100)))
}

func smsSortTime(message ReceivedSMS) time.Time {
	if message.Timestamp == nil {
		return time.Time{}
	}
	return message.Timestamp.UTC()
}
