package modem

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"smsdock/packages/api/internal/model"
)

type Manager struct {
	repo      Repository
	discovery Discovery

	mu      sync.RWMutex
	workers map[string]*Worker
}

func NewManager(repo Repository, discovery Discovery) *Manager {
	return &Manager{
		repo:      repo,
		discovery: discovery,
		workers:   make(map[string]*Worker),
	}
}

func (m *Manager) Load(ctx context.Context) error {
	modems, err := m.repo.ListModems(ctx)
	if err != nil {
		return err
	}
	for _, modemRecord := range modems {
		if err := m.syncModem(ctx, modemRecord); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) SyncModem(ctx context.Context, id string) error {
	modemRecord, err := m.repo.GetModem(ctx, id)
	if err != nil {
		return err
	}
	return m.syncModem(ctx, modemRecord)
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, worker := range m.workers {
		worker.Stop()
		delete(m.workers, id)
	}
}

func (m *Manager) RemoveModem(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	worker := m.workers[id]
	if worker == nil {
		return
	}
	worker.Stop()
	delete(m.workers, id)
}

func (m *Manager) Runtime(id string) model.ModemRuntime {
	m.mu.RLock()
	worker := m.workers[id]
	m.mu.RUnlock()
	if worker == nil {
		return model.ModemRuntime{}
	}
	return worker.Runtime()
}

func (m *Manager) RuntimeMap() map[string]model.ModemRuntime {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := make(map[string]model.ModemRuntime, len(m.workers))
	for id, worker := range m.workers {
		snapshot[id] = worker.Runtime()
	}
	return snapshot
}

func (m *Manager) ScanAvailable(ctx context.Context) ([]model.DiscoveredModem, error) {
	discovered, err := m.discovery.ScanAvailable(ctx)
	if err != nil {
		return nil, err
	}
	registered, err := m.repo.ListModems(ctx)
	if err != nil {
		return nil, err
	}

	registeredByIMEI := make(map[string]struct{}, len(registered))
	for _, modemRecord := range registered {
		registeredByIMEI[modemRecord.IMEI] = struct{}{}
	}

	modems := make([]model.DiscoveredModem, 0, len(discovered))
	for _, device := range discovered {
		if _, exists := registeredByIMEI[device.IMEI]; exists {
			continue
		}
		modems = append(modems, model.DiscoveredModem{
			Path:               device.Path,
			IMEI:               device.IMEI,
			Manufacturer:       device.Manufacturer,
			Model:              device.Model,
			Firmware:           device.Firmware,
			SIMState:           device.SIMState,
			ICCID:              device.ICCID,
			SignalStrength:     device.SignalStrength,
			CurrentNetworkCode: device.CurrentNetworkCode,
			CurrentNetworkName: device.CurrentNetworkName,
		})
	}

	return modems, nil
}

func (m *Manager) ScanNetworks(ctx context.Context, modemID string) ([]model.NetworkOption, error) {
	worker, err := m.getWorker(ctx, modemID)
	if err != nil {
		return nil, err
	}
	return worker.ScanNetworks(ctx)
}

func (m *Manager) SelectNetwork(ctx context.Context, modemID, mccMnc string) error {
	worker, err := m.getWorker(ctx, modemID)
	if err != nil {
		return err
	}
	return worker.SelectNetwork(ctx, mccMnc)
}

func (m *Manager) CleanupEvents(ctx context.Context) error {
	infoBefore := time.Now().UTC().Add(-14 * 24 * time.Hour)
	warnBefore := time.Now().UTC().Add(-90 * 24 * time.Hour)
	return m.repo.PurgeEvents(ctx, infoBefore, warnBefore)
}

func (m *Manager) syncModem(ctx context.Context, modemRecord model.Modem) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing := m.workers[modemRecord.ID]
	if !modemRecord.Enabled {
		if existing != nil {
			existing.Stop()
			delete(m.workers, modemRecord.ID)
		}
		return m.repo.UpdateModemRuntime(ctx, modemRecord.ID, model.ModemStatusDisabled, modemRecord.LastError, modemRecord.LastSeenAt)
	}

	if existing != nil {
		existing.UpdateConfig(modemRecord)
		return nil
	}

	worker := NewWorker(modemRecord, m.repo, m.discovery)
	m.workers[modemRecord.ID] = worker
	worker.Start()
	return nil
}

func (m *Manager) getWorker(ctx context.Context, modemID string) (*Worker, error) {
	m.mu.RLock()
	worker := m.workers[modemID]
	m.mu.RUnlock()
	if worker != nil {
		return worker, nil
	}

	modemRecord, err := m.repo.GetModem(ctx, modemID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("modem not found")
		}
		return nil, err
	}
	if !modemRecord.Enabled {
		return nil, fmt.Errorf("modem is disabled")
	}
	if err := m.syncModem(ctx, modemRecord); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	worker = m.workers[modemID]
	if worker == nil {
		return nil, fmt.Errorf("modem worker unavailable")
	}
	return worker, nil
}
