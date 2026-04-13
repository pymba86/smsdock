package model

import "time"

type ModemStatus string

const (
	ModemStatusOffline    ModemStatus = "offline"
	ModemStatusBinding    ModemStatus = "binding"
	ModemStatusReady      ModemStatus = "ready"
	ModemStatusPolling    ModemStatus = "polling"
	ModemStatusScanning   ModemStatus = "scanning"
	ModemStatusRecovering ModemStatus = "recovering"
	ModemStatusDegraded   ModemStatus = "degraded"
	ModemStatusDisabled   ModemStatus = "disabled"
)

type EventLevel string

const (
	EventLevelInfo  EventLevel = "info"
	EventLevelWarn  EventLevel = "warn"
	EventLevelError EventLevel = "error"
)

type Modem struct {
	ID                    string      `json:"id"`
	LogicalName           string      `json:"logicalName"`
	IMEI                  string      `json:"imei"`
	AssignedNetworkMccMnc string      `json:"assignedNetworkMccMnc"`
	Enabled               bool        `json:"enabled"`
	PollIntervalSec       int         `json:"pollIntervalSec"`
	ATTimeoutMs           int         `json:"atTimeoutMs"`
	ScanTimeoutSec        int         `json:"scanTimeoutSec"`
	Status                ModemStatus `json:"status"`
	LastError             string      `json:"lastError"`
	LastSeenAt            *time.Time  `json:"lastSeenAt"`
	CreatedAt             time.Time   `json:"createdAt"`
	UpdatedAt             time.Time   `json:"updatedAt"`
}

type ModemRuntime struct {
	Status               ModemStatus `json:"status"`
	CurrentNetworkMccMnc string      `json:"currentNetworkMccMnc"`
	CurrentNetworkName   string      `json:"currentNetworkName"`
	SignalStrength       int         `json:"signalStrength"`
	SIMState             string      `json:"simState"`
	LastPollAt           *time.Time  `json:"lastPollAt"`
	LastSuccessAt        *time.Time  `json:"lastSuccessAt"`
}

type ModemSummary struct {
	ID                    string      `json:"id"`
	LogicalName           string      `json:"logicalName"`
	IMEI                  string      `json:"imei"`
	AssignedNetworkMccMnc string      `json:"assignedNetworkMccMnc"`
	Enabled               bool        `json:"enabled"`
	PollIntervalSec       int         `json:"pollIntervalSec"`
	ATTimeoutMs           int         `json:"atTimeoutMs"`
	ScanTimeoutSec        int         `json:"scanTimeoutSec"`
	Status                ModemStatus `json:"status"`
	LastError             string      `json:"lastError"`
	LastSeenAt            *time.Time  `json:"lastSeenAt"`
	CurrentNetworkMccMnc  string      `json:"currentNetworkMccMnc"`
	CurrentNetworkName    string      `json:"currentNetworkName"`
	SignalStrength        int         `json:"signalStrength"`
	SIMState              string      `json:"simState"`
	LastPollAt            *time.Time  `json:"lastPollAt"`
	LastSuccessAt         *time.Time  `json:"lastSuccessAt"`
	CreatedAt             time.Time   `json:"createdAt"`
	UpdatedAt             time.Time   `json:"updatedAt"`
}

func BuildModemSummary(modem Modem, runtime ModemRuntime) ModemSummary {
	status := modem.Status
	if runtime.Status != "" {
		status = runtime.Status
	}

	return ModemSummary{
		ID:                    modem.ID,
		LogicalName:           modem.LogicalName,
		IMEI:                  modem.IMEI,
		AssignedNetworkMccMnc: modem.AssignedNetworkMccMnc,
		Enabled:               modem.Enabled,
		PollIntervalSec:       modem.PollIntervalSec,
		ATTimeoutMs:           modem.ATTimeoutMs,
		ScanTimeoutSec:        modem.ScanTimeoutSec,
		Status:                status,
		LastError:             modem.LastError,
		LastSeenAt:            modem.LastSeenAt,
		CurrentNetworkMccMnc:  runtime.CurrentNetworkMccMnc,
		CurrentNetworkName:    runtime.CurrentNetworkName,
		SignalStrength:        runtime.SignalStrength,
		SIMState:              runtime.SIMState,
		LastPollAt:            runtime.LastPollAt,
		LastSuccessAt:         runtime.LastSuccessAt,
		CreatedAt:             modem.CreatedAt,
		UpdatedAt:             modem.UpdatedAt,
	}
}

type SMSMessage struct {
	ID             string     `json:"id"`
	ModemID        string     `json:"modemId"`
	Sender         string     `json:"sender"`
	Body           string     `json:"body"`
	Encoding       string     `json:"encoding"`
	RawPDU         string     `json:"rawPdu"`
	ModemTimestamp *time.Time `json:"modemTimestamp"`
	ReceivedAt     time.Time  `json:"receivedAt"`
	MultipartRef   *int       `json:"multipartRef"`
	MultipartPart  *int       `json:"multipartPart"`
	MultipartTotal *int       `json:"multipartTotal"`
	DedupeKey      string     `json:"dedupeKey"`
}

type ModemEvent struct {
	ID          string     `json:"id"`
	ModemID     string     `json:"modemId"`
	Level       EventLevel `json:"level"`
	Type        string     `json:"type"`
	Message     string     `json:"message"`
	PayloadJSON string     `json:"payloadJson"`
	CreatedAt   time.Time  `json:"createdAt"`
}

type DiscoveredModem struct {
	Path               string `json:"path"`
	IMEI               string `json:"imei"`
	Manufacturer       string `json:"manufacturer"`
	Model              string `json:"model"`
	Firmware           string `json:"firmware"`
	SIMState           string `json:"simState"`
	ICCID              string `json:"iccid"`
	SignalStrength     int    `json:"signalStrength"`
	CurrentNetworkCode string `json:"currentNetworkCode"`
	CurrentNetworkName string `json:"currentNetworkName"`
}

type NetworkOption struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Status string `json:"status"`
}
