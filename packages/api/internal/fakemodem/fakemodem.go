package fakemodem

import (
	"context"
	"fmt"
	"sync"
	"time"

	"smsdock/packages/api/internal/model"
	"smsdock/packages/api/internal/modem"
)

type Registry struct {
	mu     sync.RWMutex
	modems map[string]*FakeModem
}

type FakeModem struct {
	mu sync.Mutex

	Path               string
	IMEI               string
	Manufacturer       string
	Model              string
	Firmware           string
	SIMState           string
	ICCID              string
	SignalStrength     int
	CurrentNetworkCode string
	CurrentNetworkName string
	Networks           []model.NetworkOption
	Messages           []modem.ReceivedSMS
	Available          bool

	StatusError error
	PollError   error
	ScanError   error
	SelectError error
}

func NewRegistry() *Registry {
	return &Registry{modems: make(map[string]*FakeModem)}
}

func (r *Registry) Add(fake *FakeModem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if fake.Available == false {
		fake.Available = true
	}
	r.modems[fake.IMEI] = fake
}

func (r *Registry) ScanAvailable(context.Context) ([]modem.DeviceInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	devices := make([]modem.DeviceInfo, 0, len(r.modems))
	for _, fake := range r.modems {
		if !fake.Available {
			continue
		}
		devices = append(devices, modem.DeviceInfo{
			Path:               fake.Path,
			IMEI:               fake.IMEI,
			Manufacturer:       fake.Manufacturer,
			Model:              fake.Model,
			Firmware:           fake.Firmware,
			SIMState:           fake.SIMState,
			ICCID:              fake.ICCID,
			SignalStrength:     fake.SignalStrength,
			CurrentNetworkCode: fake.CurrentNetworkCode,
			CurrentNetworkName: fake.CurrentNetworkName,
		})
	}
	return devices, nil
}

func (r *Registry) BindByIMEI(ctx context.Context, imei string, _ time.Duration) (modem.Adapter, modem.DeviceInfo, error) {
	r.mu.RLock()
	fake := r.modems[imei]
	r.mu.RUnlock()
	if fake == nil || !fake.Available {
		return nil, modem.DeviceInfo{}, fmt.Errorf("fake modem %s unavailable", imei)
	}

	info := modem.DeviceInfo{
		Path:               fake.Path,
		IMEI:               fake.IMEI,
		Manufacturer:       fake.Manufacturer,
		Model:              fake.Model,
		Firmware:           fake.Firmware,
		SIMState:           fake.SIMState,
		ICCID:              fake.ICCID,
		SignalStrength:     fake.SignalStrength,
		CurrentNetworkCode: fake.CurrentNetworkCode,
		CurrentNetworkName: fake.CurrentNetworkName,
	}
	return &Adapter{fake: fake}, info, nil
}

func (r *Registry) MovePath(imei, path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if fake := r.modems[imei]; fake != nil {
		fake.Path = path
	}
}

type Adapter struct {
	fake *FakeModem
}

func (a *Adapter) Path() string {
	a.fake.mu.Lock()
	defer a.fake.mu.Unlock()
	return a.fake.Path
}

func (a *Adapter) Info(context.Context) (modem.DeviceInfo, error) {
	a.fake.mu.Lock()
	defer a.fake.mu.Unlock()
	return modem.DeviceInfo{
		Path:               a.fake.Path,
		IMEI:               a.fake.IMEI,
		Manufacturer:       a.fake.Manufacturer,
		Model:              a.fake.Model,
		Firmware:           a.fake.Firmware,
		SIMState:           a.fake.SIMState,
		ICCID:              a.fake.ICCID,
		SignalStrength:     a.fake.SignalStrength,
		CurrentNetworkCode: a.fake.CurrentNetworkCode,
		CurrentNetworkName: a.fake.CurrentNetworkName,
	}, nil
}

func (a *Adapter) Status(context.Context) (modem.StatusSnapshot, error) {
	a.fake.mu.Lock()
	defer a.fake.mu.Unlock()
	if a.fake.StatusError != nil {
		return modem.StatusSnapshot{}, a.fake.StatusError
	}
	return modem.StatusSnapshot{
		SIMState:           a.fake.SIMState,
		SignalStrength:     a.fake.SignalStrength,
		CurrentNetworkCode: a.fake.CurrentNetworkCode,
		CurrentNetworkName: a.fake.CurrentNetworkName,
	}, nil
}

func (a *Adapter) PollSMS(_ context.Context, storage model.SMSStorage) ([]modem.ReceivedSMS, error) {
	a.fake.mu.Lock()
	defer a.fake.mu.Unlock()
	if a.fake.PollError != nil {
		return nil, a.fake.PollError
	}
	normalized := model.NormalizeSMSStorage(storage)
	messages := make([]modem.ReceivedSMS, 0, len(a.fake.Messages))
	for _, message := range a.fake.Messages {
		if model.NormalizeSMSStorage(message.Storage) != normalized {
			continue
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func (a *Adapter) DeleteSMS(_ context.Context, storage model.SMSStorage, index int) error {
	a.fake.mu.Lock()
	defer a.fake.mu.Unlock()
	normalized := model.NormalizeSMSStorage(storage)
	filtered := a.fake.Messages[:0]
	for _, message := range a.fake.Messages {
		if message.StorageIndex != index || model.NormalizeSMSStorage(message.Storage) != normalized {
			filtered = append(filtered, message)
		}
	}
	a.fake.Messages = filtered
	return nil
}

func (a *Adapter) ScanNetworks(_ context.Context) ([]model.NetworkOption, error) {
	a.fake.mu.Lock()
	defer a.fake.mu.Unlock()
	if a.fake.ScanError != nil {
		return nil, a.fake.ScanError
	}
	networks := make([]model.NetworkOption, len(a.fake.Networks))
	copy(networks, a.fake.Networks)
	return networks, nil
}

func (a *Adapter) SelectNetwork(_ context.Context, mccMnc string) error {
	a.fake.mu.Lock()
	defer a.fake.mu.Unlock()
	if a.fake.SelectError != nil {
		return a.fake.SelectError
	}
	a.fake.CurrentNetworkCode = mccMnc
	a.fake.CurrentNetworkName = mccMnc
	return nil
}

func (a *Adapter) Close() error {
	return nil
}
