package modem_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"smsdock/packages/api/internal/fakemodem"
	"smsdock/packages/api/internal/model"
	"smsdock/packages/api/internal/modem"
	"smsdock/packages/api/internal/storage"
)

func TestManagerPollsAndStoresSMS(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store, err := storage.Open(ctx, filepath.Join(t.TempDir(), "smsdock.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	registry := fakemodem.NewRegistry()
	registry.Add(&fakemodem.FakeModem{
		Path:               "/dev/fake-modem-01",
		IMEI:               "123456789012345",
		Manufacturer:       "fake",
		Model:              "M1",
		Firmware:           "1.0.0",
		SIMState:           "READY",
		ICCID:              "8901000000000000001",
		SignalStrength:     18,
		CurrentNetworkCode: "25001",
		CurrentNetworkName: "operator-a",
		Networks: []model.NetworkOption{
			{Code: "25001", Name: "operator-a", Status: "current"},
			{Code: "25002", Name: "operator-b", Status: "available"},
		},
		Messages: []modem.ReceivedSMS{
			{
				StorageIndex: 1,
				Sender:       "+79990000001",
				Body:         "hello from fake",
				Encoding:     "gsm7",
				RawPDU:       "abcd",
				Timestamp:    ptrTime(time.Now().UTC()),
			},
		},
		Available: true,
	})

	modemRecord, err := store.CreateModem(ctx, model.Modem{
		LogicalName:           "fake-01",
		IMEI:                  "123456789012345",
		AssignedNetworkMccMnc: "25001",
		Enabled:               true,
		PollIntervalSec:       1,
		ATTimeoutMs:           1000,
		ScanTimeoutSec:        30,
	})
	if err != nil {
		t.Fatalf("CreateModem() error = %v", err)
	}

	manager := modem.NewManager(store, registry)
	if err := manager.Load(ctx); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	defer manager.StopAll()

	waitFor(t, 3*time.Second, func() bool {
		sms, listErr := store.ListSMS(ctx, modemRecord.ID, 10)
		return listErr == nil && len(sms) == 1
	})

	sms, err := store.ListSMS(ctx, modemRecord.ID, 10)
	if err != nil {
		t.Fatalf("ListSMS() error = %v", err)
	}
	if len(sms) != 1 {
		t.Fatalf("sms length = %d", len(sms))
	}
	if sms[0].Body != "hello from fake" {
		t.Fatalf("sms body = %q", sms[0].Body)
	}

	networks, err := manager.ScanNetworks(ctx, modemRecord.ID)
	if err != nil {
		t.Fatalf("ScanNetworks() error = %v", err)
	}
	if len(networks) != 2 {
		t.Fatalf("networks length = %d", len(networks))
	}
	if err := manager.SelectNetwork(ctx, modemRecord.ID, "25002"); err != nil {
		t.Fatalf("SelectNetwork() error = %v", err)
	}

	updated, err := store.GetModem(ctx, modemRecord.ID)
	if err != nil {
		t.Fatalf("GetModem() error = %v", err)
	}
	if updated.AssignedNetworkMccMnc != "25002" {
		t.Fatalf("assigned network = %q", updated.AssignedNetworkMccMnc)
	}
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("condition not met in %s", timeout)
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
