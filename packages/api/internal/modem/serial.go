package modem

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"

	"smsdock/packages/api/internal/model"
	"smsdock/packages/api/internal/pdu"
)

var (
	cmglHeaderPattern = regexp.MustCompile(`^\+CMGL:\s*(\d+)`)
	copsScanPattern   = regexp.MustCompile(`\((\d+),"([^"]*)","([^"]*)","([^"]*)"(?:,"([^"]*)")?\)`)
	copsStatusPattern = regexp.MustCompile(`^\+COPS:\s*(\d+)(?:,(\d+),"([^"]*)")?`)
	cpinPattern       = regexp.MustCompile(`^\+CPIN:\s*(.+)$`)
	cpmsPattern       = regexp.MustCompile(`^\+CPMS:\s*"([^"]+)",(\d+),(\d+)`)
	csqPattern        = regexp.MustCompile(`^\+CSQ:\s*(\d+),`)
)

type SerialDiscovery struct {
	globs []string
	baud  int
}

func NewSerialDiscovery(globs []string) *SerialDiscovery {
	return &SerialDiscovery{
		globs: globs,
		baud:  115200,
	}
}

func (d *SerialDiscovery) ScanAvailable(ctx context.Context) ([]DeviceInfo, error) {
	paths, err := d.listPaths()
	if err != nil {
		return nil, err
	}

	discovered := make([]DeviceInfo, 0, len(paths))
	seenIMEI := make(map[string]struct{})

	for _, path := range paths {
		adapter, err := d.open(path, 3*time.Second)
		if err != nil {
			continue
		}

		info, err := adapter.Info(ctx)
		_ = adapter.Close()
		if err != nil || info.IMEI == "" {
			continue
		}
		if _, exists := seenIMEI[info.IMEI]; exists {
			continue
		}
		seenIMEI[info.IMEI] = struct{}{}
		discovered = append(discovered, info)
	}

	return discovered, nil
}

func (d *SerialDiscovery) BindByIMEI(ctx context.Context, imei string, timeout time.Duration) (Adapter, DeviceInfo, error) {
	paths, err := d.listPaths()
	if err != nil {
		return nil, DeviceInfo{}, err
	}

	for _, path := range paths {
		adapter, err := d.open(path, timeout)
		if err != nil {
			continue
		}

		info, err := adapter.Info(ctx)
		if err != nil {
			_ = adapter.Close()
			continue
		}
		if info.IMEI == imei {
			return adapter, info, nil
		}

		_ = adapter.Close()
	}

	return nil, DeviceInfo{}, fmt.Errorf("modem with imei %s not found", imei)
}

func (d *SerialDiscovery) listPaths() ([]string, error) {
	paths := make([]string, 0)
	seen := make(map[string]struct{})

	for _, pattern := range d.globs {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("glob %s: %w", pattern, err)
		}
		for _, match := range matches {
			if _, err := os.Stat(match); err != nil {
				continue
			}
			if _, exists := seen[match]; exists {
				continue
			}
			seen[match] = struct{}{}
			paths = append(paths, match)
		}
	}

	slices.Sort(paths)
	return paths, nil
}

func (d *SerialDiscovery) open(path string, timeout time.Duration) (*SerialAdapter, error) {
	mode := &serial.Mode{
		BaudRate: d.baud,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}
	port, err := serial.Open(path, mode)
	if err != nil {
		return nil, fmt.Errorf("open serial %s: %w", path, err)
	}
	_ = port.SetReadTimeout(150 * time.Millisecond)

	adapter := &SerialAdapter{
		path:    path,
		port:    port,
		timeout: timeout,
	}
	if _, err := adapter.command(context.Background(), "AT"); err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("probe serial %s: %w", path, err)
	}
	_, _ = adapter.command(context.Background(), "ATE0")
	_, _ = adapter.command(context.Background(), "AT+CMGF=0")

	return adapter, nil
}

type SerialAdapter struct {
	path    string
	port    serial.Port
	timeout time.Duration
	mu      sync.Mutex
}

func (a *SerialAdapter) Path() string {
	return a.path
}

func (a *SerialAdapter) Close() error {
	return a.port.Close()
}

func (a *SerialAdapter) Info(ctx context.Context) (DeviceInfo, error) {
	info := DeviceInfo{Path: a.path}

	if imei, err := a.singleLine(ctx, "AT+CGSN"); err == nil {
		info.IMEI = digitsOnly(imei)
	}
	if manufacturer, err := a.singleLine(ctx, "AT+CGMI"); err == nil {
		info.Manufacturer = manufacturer
	}
	if modelName, err := a.singleLine(ctx, "AT+CGMM"); err == nil {
		info.Model = modelName
	}
	if firmware, err := a.singleLine(ctx, "AT+CGMR"); err == nil {
		info.Firmware = firmware
	}
	if iccid, err := a.singleLine(ctx, "AT+CCID"); err == nil {
		info.ICCID = digitsOnly(iccid)
	}
	status, err := a.Status(ctx)
	if err == nil {
		info.SIMState = status.SIMState
		info.SignalStrength = status.SignalStrength
		info.CurrentNetworkCode = status.CurrentNetworkCode
		info.CurrentNetworkName = status.CurrentNetworkName
	}

	return info, nil
}

func (a *SerialAdapter) Status(ctx context.Context) (StatusSnapshot, error) {
	status := StatusSnapshot{}

	if line, err := a.singleLine(ctx, "AT+CPIN?"); err == nil {
		matches := cpinPattern.FindStringSubmatch(line)
		if len(matches) == 2 {
			status.SIMState = strings.TrimSpace(matches[1])
		} else {
			status.SIMState = strings.TrimSpace(strings.TrimPrefix(line, "+CPIN:"))
		}
	}

	if line, err := a.singleLine(ctx, "AT+CSQ"); err == nil {
		matches := csqPattern.FindStringSubmatch(line)
		if len(matches) == 2 {
			level, _ := strconv.Atoi(matches[1])
			if level == 99 {
				status.SignalStrength = -1
			} else {
				status.SignalStrength = level
			}
		}
	}

	if line, err := a.singleLine(ctx, "AT+COPS?"); err == nil {
		matches := copsStatusPattern.FindStringSubmatch(line)
		if len(matches) >= 4 {
			format := matches[2]
			operator := matches[3]
			if format == "2" {
				status.CurrentNetworkCode = operator
				status.CurrentNetworkName = operator
			} else {
				status.CurrentNetworkName = operator
			}
		}
	}

	return status, nil
}

func (a *SerialAdapter) PollSMS(ctx context.Context, storage model.SMSStorage) ([]ReceivedSMS, error) {
	storage = model.NormalizeSMSStorage(storage)
	if err := a.setSMSStorage(ctx, storage); err != nil {
		return nil, err
	}
	if _, err := a.command(ctx, "AT+CMGF=0"); err != nil {
		return nil, err
	}
	lines, err := a.command(ctx, "AT+CMGL=4")
	if err != nil {
		return nil, err
	}

	messages := make([]ReceivedSMS, 0)
	for index := 0; index < len(lines); index++ {
		header := lines[index]
		matches := cmglHeaderPattern.FindStringSubmatch(header)
		if len(matches) != 2 {
			continue
		}
		if index+1 >= len(lines) {
			break
		}
		storageIndex, _ := strconv.Atoi(matches[1])
		rawPDU := strings.TrimSpace(lines[index+1])
		index++

		decoded, err := pdu.Decode(rawPDU)
		if err != nil {
			return nil, fmt.Errorf("decode pdu %s: %w", rawPDU, err)
		}
		messages = append(messages, ReceivedSMS{
			StorageIndex:   storageIndex,
			Storage:        storage,
			Sender:         decoded.Sender,
			Body:           decoded.Body,
			Encoding:       decoded.Encoding,
			RawPDU:         rawPDU,
			Timestamp:      decoded.Timestamp,
			MultipartRef:   decoded.MultipartRef,
			MultipartPart:  decoded.MultipartPart,
			MultipartTotal: decoded.MultipartTotal,
		})
	}

	return messages, nil
}

func (a *SerialAdapter) SMSStorageStatus(ctx context.Context, storage model.SMSStorage) (SMSStorageUsage, error) {
	storage = model.NormalizeSMSStorage(storage)
	if err := a.setSMSStorage(ctx, storage); err != nil {
		return SMSStorageUsage{}, err
	}

	lines, err := a.command(ctx, "AT+CPMS?")
	if err != nil {
		return SMSStorageUsage{}, err
	}
	for _, line := range lines {
		matches := cpmsPattern.FindStringSubmatch(line)
		if len(matches) != 4 {
			continue
		}

		used, err := strconv.Atoi(matches[2])
		if err != nil {
			return SMSStorageUsage{}, fmt.Errorf("parse cpms used from %q: %w", line, err)
		}
		total, err := strconv.Atoi(matches[3])
		if err != nil {
			return SMSStorageUsage{}, fmt.Errorf("parse cpms total from %q: %w", line, err)
		}

		return SMSStorageUsage{
			Storage: model.NormalizeSMSStorage(model.SMSStorage(matches[1])),
			Used:    used,
			Total:   total,
		}, nil
	}

	return SMSStorageUsage{}, fmt.Errorf("cpms status not found for %s", storage)
}

func (a *SerialAdapter) DeleteSMS(ctx context.Context, storage model.SMSStorage, index int) error {
	if err := a.setSMSStorage(ctx, storage); err != nil {
		return err
	}
	_, err := a.command(ctx, fmt.Sprintf("AT+CMGD=%d", index))
	return err
}

func (a *SerialAdapter) setSMSStorage(ctx context.Context, storage model.SMSStorage) error {
	storage = model.NormalizeSMSStorage(storage)
	_, err := a.command(ctx, fmt.Sprintf(`AT+CPMS="%s"`, storage))
	return err
}

func (a *SerialAdapter) ScanNetworks(ctx context.Context) ([]model.NetworkOption, error) {
	lines, err := a.command(ctx, "AT+COPS=?")
	if err != nil {
		return nil, err
	}

	networks := make([]model.NetworkOption, 0)
	seen := make(map[string]struct{})
	for _, line := range lines {
		matches := copsScanPattern.FindAllStringSubmatch(line, -1)
		for _, match := range matches {
			code := match[4]
			if code == "" {
				continue
			}
			if _, exists := seen[code]; exists {
				continue
			}
			seen[code] = struct{}{}
			name := match[2]
			if name == "" {
				name = match[3]
			}
			networks = append(networks, model.NetworkOption{
				Code:   code,
				Name:   name,
				Status: parseNetworkStatus(match[1]),
			})
		}
	}

	return networks, nil
}

func (a *SerialAdapter) SelectNetwork(ctx context.Context, mccMnc string) error {
	_, err := a.command(ctx, fmt.Sprintf(`AT+COPS=1,2,"%s"`, mccMnc))
	return err
}

func (a *SerialAdapter) singleLine(ctx context.Context, command string) (string, error) {
	lines, err := a.command(ctx, command)
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", nil
	}
	return strings.TrimSpace(lines[len(lines)-1]), nil
}

func (a *SerialAdapter) command(ctx context.Context, command string) ([]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	commandCtx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	if _, err := a.port.Write([]byte(command + "\r")); err != nil {
		return nil, fmt.Errorf("write command %s: %w", command, err)
	}

	buffer := make([]byte, 256)
	var builder strings.Builder
	lines := make([]string, 0)

	flushLine := func() (bool, error) {
		line := strings.TrimSpace(builder.String())
		builder.Reset()
		if line == "" || line == command {
			return false, nil
		}
		switch {
		case line == "OK":
			return true, nil
		case strings.HasPrefix(line, "ERROR"),
			strings.HasPrefix(line, "+CMS ERROR"),
			strings.HasPrefix(line, "+CME ERROR"):
			return true, errors.New(line)
		default:
			lines = append(lines, line)
			return false, nil
		}
	}

	for {
		select {
		case <-commandCtx.Done():
			return nil, fmt.Errorf("command timeout for %s", command)
		default:
		}

		count, err := a.port.Read(buffer)
		if err != nil {
			return nil, fmt.Errorf("read command %s: %w", command, err)
		}
		if count == 0 {
			continue
		}

		for _, value := range buffer[:count] {
			switch value {
			case '\r', '\n':
				done, lineErr := flushLine()
				if lineErr != nil {
					return nil, lineErr
				}
				if done {
					return lines, nil
				}
			default:
				builder.WriteByte(value)
			}
		}
	}
}

func parseNetworkStatus(code string) string {
	switch code {
	case "0":
		return "unknown"
	case "1":
		return "available"
	case "2":
		return "current"
	case "3":
		return "forbidden"
	default:
		return "unknown"
	}
}

func digitsOnly(value string) string {
	var builder strings.Builder
	for _, item := range value {
		if item >= '0' && item <= '9' {
			builder.WriteRune(item)
		}
	}
	return builder.String()
}
