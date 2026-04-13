package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"smsdock/packages/api/internal/model"
)

type SQLiteStore struct {
	db *sql.DB
}

func Open(ctx context.Context, path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) migrate(ctx context.Context) error {
	statements := []string{
		`PRAGMA journal_mode = WAL;`,
		`CREATE TABLE IF NOT EXISTS modems (
			id TEXT PRIMARY KEY,
			logical_name TEXT NOT NULL,
			imei TEXT NOT NULL UNIQUE,
			assigned_network_mcc_mnc TEXT NOT NULL,
			enabled INTEGER NOT NULL,
			poll_interval_sec INTEGER NOT NULL,
			at_timeout_ms INTEGER NOT NULL,
			scan_timeout_sec INTEGER NOT NULL,
			status TEXT NOT NULL,
			last_error TEXT NOT NULL DEFAULT '',
			last_seen_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS sms (
			id TEXT PRIMARY KEY,
			modem_id TEXT NOT NULL,
			sender TEXT NOT NULL,
			body TEXT NOT NULL,
			encoding TEXT NOT NULL,
			raw_pdu TEXT NOT NULL,
			modem_timestamp TEXT,
			received_at TEXT NOT NULL,
			multipart_ref INTEGER,
			multipart_part INTEGER,
			multipart_total INTEGER,
			dedupe_key TEXT NOT NULL UNIQUE,
			FOREIGN KEY(modem_id) REFERENCES modems(id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_sms_modem_id_received_at ON sms(modem_id, received_at DESC);`,
		`CREATE TABLE IF NOT EXISTS modem_events (
			id TEXT PRIMARY KEY,
			modem_id TEXT NOT NULL,
			level TEXT NOT NULL,
			type TEXT NOT NULL,
			message TEXT NOT NULL,
			payload_json TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			FOREIGN KEY(modem_id) REFERENCES modems(id)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_modem_events_modem_id_created_at ON modem_events(modem_id, created_at DESC);`,
	}

	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate sqlite: %w", err)
		}
	}

	return nil
}

func (s *SQLiteStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *SQLiteStore) ListModems(ctx context.Context) ([]model.Modem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, logical_name, imei, assigned_network_mcc_mnc, enabled,
		       poll_interval_sec, at_timeout_ms, scan_timeout_sec, status,
		       last_error, last_seen_at, created_at, updated_at
		FROM modems
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list modems: %w", err)
	}
	defer rows.Close()

	var modems []model.Modem
	for rows.Next() {
		modem, err := scanModem(rows.Scan)
		if err != nil {
			return nil, err
		}
		modems = append(modems, modem)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate modems: %w", err)
	}

	return modems, nil
}

func (s *SQLiteStore) GetModem(ctx context.Context, id string) (model.Modem, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, logical_name, imei, assigned_network_mcc_mnc, enabled,
		       poll_interval_sec, at_timeout_ms, scan_timeout_sec, status,
		       last_error, last_seen_at, created_at, updated_at
		FROM modems
		WHERE id = ?
	`, id)

	modem, err := scanModem(row.Scan)
	if err != nil {
		return model.Modem{}, err
	}
	return modem, nil
}

func (s *SQLiteStore) GetModemByIMEI(ctx context.Context, imei string) (model.Modem, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, logical_name, imei, assigned_network_mcc_mnc, enabled,
		       poll_interval_sec, at_timeout_ms, scan_timeout_sec, status,
		       last_error, last_seen_at, created_at, updated_at
		FROM modems
		WHERE imei = ?
	`, imei)

	modem, err := scanModem(row.Scan)
	if err != nil {
		return model.Modem{}, err
	}
	return modem, nil
}

func (s *SQLiteStore) CreateModem(ctx context.Context, modem model.Modem) (model.Modem, error) {
	now := time.Now().UTC()
	if modem.ID == "" {
		modem.ID = model.NewID("modem")
	}
	modem.CreatedAt = now
	modem.UpdatedAt = now
	if modem.Status == "" {
		if modem.Enabled {
			modem.Status = model.ModemStatusOffline
		} else {
			modem.Status = model.ModemStatusDisabled
		}
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO modems (
			id, logical_name, imei, assigned_network_mcc_mnc, enabled,
			poll_interval_sec, at_timeout_ms, scan_timeout_sec, status,
			last_error, last_seen_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		modem.ID,
		modem.LogicalName,
		modem.IMEI,
		modem.AssignedNetworkMccMnc,
		boolToInt(modem.Enabled),
		modem.PollIntervalSec,
		modem.ATTimeoutMs,
		modem.ScanTimeoutSec,
		modem.Status,
		modem.LastError,
		formatTime(modem.LastSeenAt),
		modem.CreatedAt.Format(time.RFC3339Nano),
		modem.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return model.Modem{}, fmt.Errorf("create modem: %w", err)
	}

	return modem, nil
}

func (s *SQLiteStore) UpdateModem(ctx context.Context, modem model.Modem) (model.Modem, error) {
	modem.UpdatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
		UPDATE modems
		SET logical_name = ?,
		    assigned_network_mcc_mnc = ?,
		    enabled = ?,
		    poll_interval_sec = ?,
		    at_timeout_ms = ?,
		    scan_timeout_sec = ?,
		    status = ?,
		    last_error = ?,
		    last_seen_at = ?,
		    updated_at = ?
		WHERE id = ?
	`,
		modem.LogicalName,
		modem.AssignedNetworkMccMnc,
		boolToInt(modem.Enabled),
		modem.PollIntervalSec,
		modem.ATTimeoutMs,
		modem.ScanTimeoutSec,
		modem.Status,
		modem.LastError,
		formatTime(modem.LastSeenAt),
		modem.UpdatedAt.Format(time.RFC3339Nano),
		modem.ID,
	)
	if err != nil {
		return model.Modem{}, fmt.Errorf("update modem: %w", err)
	}
	return modem, nil
}

func (s *SQLiteStore) UpdateModemRuntime(ctx context.Context, id string, status model.ModemStatus, lastError string, lastSeenAt *time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE modems
		SET status = ?, last_error = ?, last_seen_at = ?, updated_at = ?
		WHERE id = ?
	`,
		status,
		lastError,
		formatTime(lastSeenAt),
		time.Now().UTC().Format(time.RFC3339Nano),
		id,
	)
	if err != nil {
		return fmt.Errorf("update modem runtime: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteModem(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete modem tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `DELETE FROM sms WHERE modem_id = ?`, id); err != nil {
		return fmt.Errorf("delete modem sms: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM modem_events WHERE modem_id = ?`, id); err != nil {
		return fmt.Errorf("delete modem events: %w", err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM modems WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete modem: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete modem rows: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit delete modem tx: %w", err)
	}
	return nil
}

func (s *SQLiteStore) SaveSMS(ctx context.Context, message model.SMSMessage) error {
	if message.ID == "" {
		message.ID = model.NewID("sms")
	}
	if message.ReceivedAt.IsZero() {
		message.ReceivedAt = time.Now().UTC()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sms (
			id, modem_id, sender, body, encoding, raw_pdu, modem_timestamp,
			received_at, multipart_ref, multipart_part, multipart_total, dedupe_key
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		message.ID,
		message.ModemID,
		message.Sender,
		message.Body,
		message.Encoding,
		message.RawPDU,
		formatTime(message.ModemTimestamp),
		message.ReceivedAt.Format(time.RFC3339Nano),
		nullableInt(message.MultipartRef),
		nullableInt(message.MultipartPart),
		nullableInt(message.MultipartTotal),
		message.DedupeKey,
	)
	if err != nil {
		if isSQLiteConstraint(err) {
			return nil
		}
		return fmt.Errorf("save sms: %w", err)
	}

	return nil
}

func (s *SQLiteStore) ListSMS(ctx context.Context, modemID string, limit int) ([]model.SMSMessage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, modem_id, sender, body, encoding, raw_pdu, modem_timestamp,
		       received_at, multipart_ref, multipart_part, multipart_total, dedupe_key
		FROM sms
		WHERE modem_id = ?
		ORDER BY received_at DESC
		LIMIT ?
	`, modemID, limit)
	if err != nil {
		return nil, fmt.Errorf("list sms: %w", err)
	}
	defer rows.Close()

	sms := make([]model.SMSMessage, 0)
	for rows.Next() {
		item, err := scanSMS(rows.Scan)
		if err != nil {
			return nil, err
		}
		sms = append(sms, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sms: %w", err)
	}

	return sms, nil
}

func (s *SQLiteStore) ListSMSPage(ctx context.Context, modemID string, page, pageSize int) ([]model.SMSMessage, int, error) {
	total, err := s.countRows(ctx, `SELECT COUNT(1) FROM sms WHERE modem_id = ?`, modemID)
	if err != nil {
		return nil, 0, fmt.Errorf("count sms: %w", err)
	}
	items, err := s.listSMSWithOffset(ctx, modemID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *SQLiteStore) AppendEvent(ctx context.Context, event model.ModemEvent) error {
	if event.ID == "" {
		event.ID = model.NewID("evt")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO modem_events (
			id, modem_id, level, type, message, payload_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		event.ID,
		event.ModemID,
		event.Level,
		event.Type,
		event.Message,
		event.PayloadJSON,
		event.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("append event: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListEvents(ctx context.Context, modemID string, limit int) ([]model.ModemEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, modem_id, level, type, message, payload_json, created_at
		FROM modem_events
		WHERE modem_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, modemID, limit)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	events := make([]model.ModemEvent, 0)
	for rows.Next() {
		event, err := scanEvent(rows.Scan)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	return events, nil
}

func (s *SQLiteStore) ListEventsPage(ctx context.Context, modemID string, page, pageSize int) ([]model.ModemEvent, int, error) {
	total, err := s.countRows(ctx, `SELECT COUNT(1) FROM modem_events WHERE modem_id = ?`, modemID)
	if err != nil {
		return nil, 0, fmt.Errorf("count events: %w", err)
	}
	items, err := s.listEventsWithOffset(ctx, modemID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *SQLiteStore) PurgeEvents(ctx context.Context, infoBefore, warnBefore time.Time) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM modem_events
		WHERE (level = 'info' AND created_at < ?)
		   OR (level IN ('warn', 'error') AND created_at < ?)
	`,
		infoBefore.UTC().Format(time.RFC3339Nano),
		warnBefore.UTC().Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("purge events: %w", err)
	}
	return nil
}

func (s *SQLiteStore) listSMSWithOffset(ctx context.Context, modemID string, page, pageSize int) ([]model.SMSMessage, error) {
	offset := pageOffset(page, pageSize)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, modem_id, sender, body, encoding, raw_pdu, modem_timestamp,
		       received_at, multipart_ref, multipart_part, multipart_total, dedupe_key
		FROM sms
		WHERE modem_id = ?
		ORDER BY received_at DESC
		LIMIT ? OFFSET ?
	`, modemID, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("list sms page: %w", err)
	}
	defer rows.Close()

	sms := make([]model.SMSMessage, 0)
	for rows.Next() {
		item, err := scanSMS(rows.Scan)
		if err != nil {
			return nil, err
		}
		sms = append(sms, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sms page: %w", err)
	}
	return sms, nil
}

func (s *SQLiteStore) listEventsWithOffset(ctx context.Context, modemID string, page, pageSize int) ([]model.ModemEvent, error) {
	offset := pageOffset(page, pageSize)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, modem_id, level, type, message, payload_json, created_at
		FROM modem_events
		WHERE modem_id = ?
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, modemID, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("list events page: %w", err)
	}
	defer rows.Close()

	events := make([]model.ModemEvent, 0)
	for rows.Next() {
		event, err := scanEvent(rows.Scan)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events page: %w", err)
	}
	return events, nil
}

func (s *SQLiteStore) countRows(ctx context.Context, query string, args ...any) (int, error) {
	var total int
	if err := s.db.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func pageOffset(page, pageSize int) int {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 1
	}
	return (page - 1) * pageSize
}

func scanModem(scan func(dest ...any) error) (model.Modem, error) {
	var modem model.Modem
	var enabled int
	var lastSeen sql.NullString
	var status string
	var createdAt string
	var updatedAt string

	err := scan(
		&modem.ID,
		&modem.LogicalName,
		&modem.IMEI,
		&modem.AssignedNetworkMccMnc,
		&enabled,
		&modem.PollIntervalSec,
		&modem.ATTimeoutMs,
		&modem.ScanTimeoutSec,
		&status,
		&modem.LastError,
		&lastSeen,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Modem{}, err
		}
		return model.Modem{}, fmt.Errorf("scan modem: %w", err)
	}

	modem.Enabled = enabled == 1
	modem.Status = model.ModemStatus(status)
	if lastSeen.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, lastSeen.String)
		if err != nil {
			return model.Modem{}, fmt.Errorf("parse modem last seen: %w", err)
		}
		modem.LastSeenAt = &parsed
	}
	modem.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return model.Modem{}, fmt.Errorf("parse modem created at: %w", err)
	}
	modem.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return model.Modem{}, fmt.Errorf("parse modem updated at: %w", err)
	}

	return modem, nil
}

func scanSMS(scan func(dest ...any) error) (model.SMSMessage, error) {
	var message model.SMSMessage
	var modemTimestamp sql.NullString
	var multipartRef sql.NullInt64
	var multipartPart sql.NullInt64
	var multipartTotal sql.NullInt64
	var receivedAt string

	err := scan(
		&message.ID,
		&message.ModemID,
		&message.Sender,
		&message.Body,
		&message.Encoding,
		&message.RawPDU,
		&modemTimestamp,
		&receivedAt,
		&multipartRef,
		&multipartPart,
		&multipartTotal,
		&message.DedupeKey,
	)
	if err != nil {
		return model.SMSMessage{}, fmt.Errorf("scan sms: %w", err)
	}

	if modemTimestamp.Valid {
		parsed, err := time.Parse(time.RFC3339Nano, modemTimestamp.String)
		if err != nil {
			return model.SMSMessage{}, fmt.Errorf("parse modem timestamp: %w", err)
		}
		message.ModemTimestamp = &parsed
	}
	parsedReceivedAt, err := time.Parse(time.RFC3339Nano, receivedAt)
	if err != nil {
		return model.SMSMessage{}, fmt.Errorf("parse received at: %w", err)
	}
	message.ReceivedAt = parsedReceivedAt
	if multipartRef.Valid {
		value := int(multipartRef.Int64)
		message.MultipartRef = &value
	}
	if multipartPart.Valid {
		value := int(multipartPart.Int64)
		message.MultipartPart = &value
	}
	if multipartTotal.Valid {
		value := int(multipartTotal.Int64)
		message.MultipartTotal = &value
	}

	return message, nil
}

func scanEvent(scan func(dest ...any) error) (model.ModemEvent, error) {
	var event model.ModemEvent
	var createdAt string
	var level string

	err := scan(
		&event.ID,
		&event.ModemID,
		&level,
		&event.Type,
		&event.Message,
		&event.PayloadJSON,
		&createdAt,
	)
	if err != nil {
		return model.ModemEvent{}, fmt.Errorf("scan event: %w", err)
	}

	event.Level = model.EventLevel(level)
	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return model.ModemEvent{}, fmt.Errorf("parse event created at: %w", err)
	}
	event.CreatedAt = parsedCreatedAt
	return event, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func formatTime(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func isSQLiteConstraint(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "constraint") || strings.Contains(err.Error(), "UNIQUE"))
}
