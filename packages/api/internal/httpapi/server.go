package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"smsdock/packages/api/internal/model"
	"smsdock/packages/api/internal/modem"
	"smsdock/packages/api/internal/storage"
)

type Server struct {
	store    *storage.SQLiteStore
	manager  *modem.Manager
	frontend http.Handler
}

func New(store *storage.SQLiteStore, manager *modem.Manager, frontend http.Handler) *Server {
	return &Server{store: store, manager: manager, frontend: frontend}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/modems", s.handleListModems)
	mux.HandleFunc("GET /api/modems/{id}", s.handleGetModem)
	mux.HandleFunc("GET /api/modems/{id}/sms", s.handleListSMS)
	mux.HandleFunc("GET /api/modems/{id}/events", s.handleListEvents)
	mux.HandleFunc("POST /api/modems/scan", s.handleScanModems)
	mux.HandleFunc("POST /api/modems", s.handleCreateModem)
	mux.HandleFunc("DELETE /api/modems/{id}", s.handleDeleteModem)
	mux.HandleFunc("PATCH /api/modems/{id}", s.handleUpdateModem)
	mux.HandleFunc("POST /api/modems/{id}/enable", s.handleEnableModem)
	mux.HandleFunc("POST /api/modems/{id}/disable", s.handleDisableModem)
	mux.HandleFunc("POST /api/modems/{id}/networks/scan", s.handleScanNetworks)
	mux.HandleFunc("POST /api/modems/{id}/networks/select", s.handleSelectNetwork)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /ready", s.handleReady)
	if s.frontend != nil {
		mux.Handle("/", s.frontend)
	}

	return cors(mux)
}

func (s *Server) handleListModems(writer http.ResponseWriter, request *http.Request) {
	modems, err := s.store.ListModems(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}

	summary := make([]model.ModemSummary, 0, len(modems))
	runtime := s.manager.RuntimeMap()
	for _, modemRecord := range modems {
		summary = append(summary, model.BuildModemSummary(modemRecord, runtime[modemRecord.ID]))
	}

	writeJSON(writer, http.StatusOK, map[string]any{"modems": summary})
}

func (s *Server) handleGetModem(writer http.ResponseWriter, request *http.Request) {
	modemID := request.PathValue("id")
	modemRecord, err := s.store.GetModem(request.Context(), modemID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeError(writer, status, err)
		return
	}

	writeJSON(writer, http.StatusOK, map[string]any{
		"modem": model.BuildModemSummary(modemRecord, s.manager.Runtime(modemID)),
	})
}

func (s *Server) handleListSMS(writer http.ResponseWriter, request *http.Request) {
	page, pageSize := readPagination(request, 1, 20)
	sms, total, err := s.store.ListSMSPage(request.Context(), request.PathValue("id"), page, pageSize)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"sms": sms,
		"pagination": map[string]any{
			"page":       page,
			"pageSize":   pageSize,
			"totalItems": total,
			"totalPages": totalPages(total, pageSize),
		},
	})
}

func (s *Server) handleListEvents(writer http.ResponseWriter, request *http.Request) {
	page, pageSize := readPagination(request, 1, 20)
	events, total, err := s.store.ListEventsPage(request.Context(), request.PathValue("id"), page, pageSize)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"events": events,
		"pagination": map[string]any{
			"page":       page,
			"pageSize":   pageSize,
			"totalItems": total,
			"totalPages": totalPages(total, pageSize),
		},
	})
}

func (s *Server) handleScanModems(writer http.ResponseWriter, request *http.Request) {
	modems, err := s.manager.ScanAvailable(request.Context())
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"modems": modems})
}

func (s *Server) handleCreateModem(writer http.ResponseWriter, request *http.Request) {
	var payload createModemRequest
	if err := decodeJSON(request, &payload); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}

	if _, err := s.store.GetModemByIMEI(request.Context(), payload.IMEI); err == nil {
		writeError(writer, http.StatusConflict, fmt.Errorf("modem with imei %s already exists", payload.IMEI))
		return
	} else if !errors.Is(err, sql.ErrNoRows) {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}

	modemRecord := model.Modem{
		LogicalName:           strings.TrimSpace(payload.LogicalName),
		IMEI:                  strings.TrimSpace(payload.IMEI),
		AssignedNetworkMccMnc: strings.TrimSpace(payload.AssignedNetworkMccMnc),
		SMSReadStorage:        model.NormalizeSMSStorage(model.SMSStorage(payload.SMSReadStorage)),
		SMSDeleteThresholdPct: model.NormalizeSMSDeleteThresholdPct(payload.SMSDeleteThresholdPct),
		Enabled:               payload.Enabled,
		PollIntervalSec:       payload.PollIntervalSec,
		ATTimeoutMs:           payload.ATTimeoutMs,
		ScanTimeoutSec:        payload.ScanTimeoutSec,
	}
	if err := validateModem(modemRecord); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}

	created, err := s.store.CreateModem(request.Context(), modemRecord)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	if created.Enabled {
		if err := s.manager.SyncModem(request.Context(), created.ID); err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}
	}
	writeModemResponse(writer, request.Context(), s.store, s.manager, created.ID, http.StatusCreated)
}

func (s *Server) handleDeleteModem(writer http.ResponseWriter, request *http.Request) {
	modemID := request.PathValue("id")
	modemRecord, err := s.store.GetModem(request.Context(), modemID)
	if err != nil {
		writeError(writer, http.StatusNotFound, err)
		return
	}

	s.manager.RemoveModem(modemID)
	if err := s.store.DeleteModem(request.Context(), modemID); err != nil {
		if modemRecord.Enabled {
			_ = s.manager.SyncModem(request.Context(), modemID)
		}
		writeError(writer, http.StatusInternalServerError, err)
		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleUpdateModem(writer http.ResponseWriter, request *http.Request) {
	modemRecord, err := s.store.GetModem(request.Context(), request.PathValue("id"))
	if err != nil {
		writeError(writer, http.StatusNotFound, err)
		return
	}

	var payload updateModemRequest
	if err := decodeJSON(request, &payload); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}

	if payload.LogicalName != nil {
		modemRecord.LogicalName = strings.TrimSpace(*payload.LogicalName)
	}
	if payload.AssignedNetworkMccMnc != nil {
		modemRecord.AssignedNetworkMccMnc = strings.TrimSpace(*payload.AssignedNetworkMccMnc)
	}
	if payload.SMSReadStorage != nil {
		modemRecord.SMSReadStorage = model.NormalizeSMSStorage(model.SMSStorage(*payload.SMSReadStorage))
	}
	if payload.SMSDeleteThresholdPct != nil {
		modemRecord.SMSDeleteThresholdPct = model.NormalizeSMSDeleteThresholdPct(*payload.SMSDeleteThresholdPct)
	}
	if payload.PollIntervalSec != nil {
		modemRecord.PollIntervalSec = *payload.PollIntervalSec
	}
	if payload.ATTimeoutMs != nil {
		modemRecord.ATTimeoutMs = *payload.ATTimeoutMs
	}
	if payload.ScanTimeoutSec != nil {
		modemRecord.ScanTimeoutSec = *payload.ScanTimeoutSec
	}

	if err := validateModem(modemRecord); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}

	if _, err := s.store.UpdateModem(request.Context(), modemRecord); err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	if modemRecord.Enabled {
		if err := s.manager.SyncModem(request.Context(), modemRecord.ID); err != nil {
			writeError(writer, http.StatusInternalServerError, err)
			return
		}
	}
	writeModemResponse(writer, request.Context(), s.store, s.manager, modemRecord.ID, http.StatusOK)
}

func (s *Server) handleEnableModem(writer http.ResponseWriter, request *http.Request) {
	modemRecord, err := s.store.GetModem(request.Context(), request.PathValue("id"))
	if err != nil {
		writeError(writer, http.StatusNotFound, err)
		return
	}
	modemRecord.Enabled = true
	modemRecord.Status = model.ModemStatusOffline
	modemRecord.LastError = ""
	if _, err := s.store.UpdateModem(request.Context(), modemRecord); err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	if err := s.manager.SyncModem(request.Context(), modemRecord.ID); err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	writeModemResponse(writer, request.Context(), s.store, s.manager, modemRecord.ID, http.StatusOK)
}

func (s *Server) handleDisableModem(writer http.ResponseWriter, request *http.Request) {
	modemRecord, err := s.store.GetModem(request.Context(), request.PathValue("id"))
	if err != nil {
		writeError(writer, http.StatusNotFound, err)
		return
	}
	modemRecord.Enabled = false
	modemRecord.Status = model.ModemStatusDisabled
	if _, err := s.store.UpdateModem(request.Context(), modemRecord); err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	if err := s.manager.SyncModem(request.Context(), modemRecord.ID); err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	writeModemResponse(writer, request.Context(), s.store, s.manager, modemRecord.ID, http.StatusOK)
}

func (s *Server) handleScanNetworks(writer http.ResponseWriter, request *http.Request) {
	networks, err := s.manager.ScanNetworks(request.Context(), request.PathValue("id"))
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"modemId":  request.PathValue("id"),
		"networks": networks,
	})
}

func (s *Server) handleSelectNetwork(writer http.ResponseWriter, request *http.Request) {
	var payload selectNetworkRequest
	if err := decodeJSON(request, &payload); err != nil {
		writeError(writer, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(payload.MccMnc) == "" {
		writeError(writer, http.StatusBadRequest, fmt.Errorf("mccMnc is required"))
		return
	}
	if err := s.manager.SelectNetwork(request.Context(), request.PathValue("id"), payload.MccMnc); err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	writeModemResponse(writer, request.Context(), s.store, s.manager, request.PathValue("id"), http.StatusOK)
}

func (s *Server) handleHealth(writer http.ResponseWriter, request *http.Request) {
	status := "ok"
	if err := s.store.Ping(request.Context()); err != nil {
		status = err.Error()
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"ok": status == "ok",
		"checks": map[string]string{
			"database": status,
		},
	})
}

func (s *Server) handleReady(writer http.ResponseWriter, request *http.Request) {
	if err := s.store.Ping(request.Context()); err != nil {
		writeError(writer, http.StatusServiceUnavailable, err)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"ok": true,
		"checks": map[string]string{
			"database": "ok",
		},
	})
}

type createModemRequest struct {
	LogicalName           string `json:"logicalName"`
	IMEI                  string `json:"imei"`
	AssignedNetworkMccMnc string `json:"assignedNetworkMccMnc"`
	SMSReadStorage        string `json:"smsReadStorage"`
	SMSDeleteThresholdPct int    `json:"smsDeleteThresholdPct"`
	PollIntervalSec       int    `json:"pollIntervalSec"`
	ATTimeoutMs           int    `json:"atTimeoutMs"`
	ScanTimeoutSec        int    `json:"scanTimeoutSec"`
	Enabled               bool   `json:"enabled"`
}

type updateModemRequest struct {
	LogicalName           *string `json:"logicalName"`
	AssignedNetworkMccMnc *string `json:"assignedNetworkMccMnc"`
	SMSReadStorage        *string `json:"smsReadStorage"`
	SMSDeleteThresholdPct *int    `json:"smsDeleteThresholdPct"`
	PollIntervalSec       *int    `json:"pollIntervalSec"`
	ATTimeoutMs           *int    `json:"atTimeoutMs"`
	ScanTimeoutSec        *int    `json:"scanTimeoutSec"`
}

type selectNetworkRequest struct {
	MccMnc string `json:"mccMnc"`
}

func writeModemResponse(writer http.ResponseWriter, ctx context.Context, store *storage.SQLiteStore, manager *modem.Manager, modemID string, code int) {
	modemRecord, err := store.GetModem(ctx, modemID)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, err)
		return
	}
	writeJSON(writer, code, map[string]any{
		"modem": model.BuildModemSummary(modemRecord, manager.Runtime(modemID)),
	})
}

func decodeJSON(request *http.Request, target any) error {
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func writeError(writer http.ResponseWriter, status int, err error) {
	writeJSON(writer, status, map[string]any{
		"error": err.Error(),
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !isCORSRoute(request.URL.Path) {
			next.ServeHTTP(writer, request)
			return
		}

		writer.Header().Set("Access-Control-Allow-Origin", "*")
		writer.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func isCORSRoute(path string) bool {
	return strings.HasPrefix(path, "/api/") || path == "/health" || path == "/ready"
}

func readPagination(request *http.Request, defaultPage, defaultPageSize int) (int, int) {
	return readPositiveQueryInt(request, "page", defaultPage, 1, 1000000),
		readPositiveQueryInt(request, "pageSize", defaultPageSize, 1, 100)
}

func readPositiveQueryInt(request *http.Request, name string, fallback int, min int, max int) int {
	value := request.URL.Query().Get(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	if parsed < min {
		return min
	}
	if parsed > max {
		return max
	}
	return parsed
}

func totalPages(totalItems, pageSize int) int {
	if totalItems == 0 {
		return 1
	}
	return (totalItems + pageSize - 1) / pageSize
}

func validateModem(modemRecord model.Modem) error {
	switch {
	case modemRecord.LogicalName == "":
		return fmt.Errorf("logicalName is required")
	case modemRecord.IMEI == "":
		return fmt.Errorf("imei is required")
	case modemRecord.AssignedNetworkMccMnc == "":
		return fmt.Errorf("assignedNetworkMccMnc is required")
	case !model.IsValidSMSStorage(modemRecord.SMSReadStorage):
		return fmt.Errorf("smsReadStorage must be one of SM, ME, MT")
	case modemRecord.SMSDeleteThresholdPct < 1 || modemRecord.SMSDeleteThresholdPct > 100:
		return fmt.Errorf("smsDeleteThresholdPct must be between 1 and 100")
	case modemRecord.PollIntervalSec < 5:
		return fmt.Errorf("pollIntervalSec must be at least 5")
	case modemRecord.ATTimeoutMs < 500:
		return fmt.Errorf("atTimeoutMs must be at least 500")
	case modemRecord.ScanTimeoutSec < 30:
		return fmt.Errorf("scanTimeoutSec must be at least 30")
	default:
		return nil
	}
}
