package modem

import (
	"context"
	"time"

	"smsdock/packages/api/internal/model"
)

type Repository interface {
	ListModems(ctx context.Context) ([]model.Modem, error)
	GetModem(ctx context.Context, id string) (model.Modem, error)
	GetModemByIMEI(ctx context.Context, imei string) (model.Modem, error)
	CreateModem(ctx context.Context, modem model.Modem) (model.Modem, error)
	UpdateModem(ctx context.Context, modem model.Modem) (model.Modem, error)
	UpdateModemRuntime(ctx context.Context, id string, status model.ModemStatus, lastError string, lastSeenAt *time.Time) error
	SaveSMS(ctx context.Context, message model.SMSMessage) error
	ListSMS(ctx context.Context, modemID string, limit int) ([]model.SMSMessage, error)
	AppendEvent(ctx context.Context, event model.ModemEvent) error
	ListEvents(ctx context.Context, modemID string, limit int) ([]model.ModemEvent, error)
	PurgeEvents(ctx context.Context, infoBefore, warnBefore time.Time) error
	Ping(ctx context.Context) error
}

type DeviceInfo struct {
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
}

type StatusSnapshot struct {
	SIMState           string
	SignalStrength     int
	CurrentNetworkCode string
	CurrentNetworkName string
}

type ReceivedSMS struct {
	StorageIndex    int
	Sender          string
	Body            string
	Encoding        string
	RawPDU          string
	Timestamp       *time.Time
	MultipartRef    *int
	MultipartPart   *int
	MultipartTotal  *int
	DedupeKeySuffix string
}

type Adapter interface {
	Path() string
	Info(ctx context.Context) (DeviceInfo, error)
	Status(ctx context.Context) (StatusSnapshot, error)
	PollSMS(ctx context.Context) ([]ReceivedSMS, error)
	DeleteSMS(ctx context.Context, index int) error
	ScanNetworks(ctx context.Context) ([]model.NetworkOption, error)
	SelectNetwork(ctx context.Context, mccMnc string) error
	Close() error
}

type Discovery interface {
	ScanAvailable(ctx context.Context) ([]DeviceInfo, error)
	BindByIMEI(ctx context.Context, imei string, timeout time.Duration) (Adapter, DeviceInfo, error)
}
