package modem

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"go.bug.st/serial"
)

func TestSerialAdapterCommandUsesDefaultTimeout(t *testing.T) {
	t.Parallel()

	command := "AT+COPS=?"
	port := newScriptedPort(command, `+COPS: (2,"MegaFon RUS","","25002")`, 120*time.Millisecond)
	adapter := &SerialAdapter{
		port:    port,
		timeout: 50 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	_, err := adapter.command(ctx, command)
	if err == nil || !strings.Contains(err.Error(), "command timeout for "+command) {
		t.Fatalf("command() error = %v", err)
	}
}

func TestSerialAdapterCommandWithCallerDeadlineAllowsLongCommands(t *testing.T) {
	t.Parallel()

	command := "AT+COPS=?"
	port := newScriptedPort(command, `+COPS: (2,"MegaFon RUS","","25002")`, 120*time.Millisecond)
	adapter := &SerialAdapter{
		port:    port,
		timeout: 50 * time.Millisecond,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	lines, err := adapter.commandWithCallerDeadline(ctx, command)
	if err != nil {
		t.Fatalf("commandWithCallerDeadline() error = %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("lines length = %d", len(lines))
	}
	if !strings.Contains(lines[0], "25002") {
		t.Fatalf("lines = %#v", lines)
	}
}

type scriptedPort struct {
	mu            sync.Mutex
	response      []byte
	responseAfter time.Duration
	startedAt     time.Time
	offset        int
}

func newScriptedPort(command, line string, responseAfter time.Duration) *scriptedPort {
	return &scriptedPort{
		response:      []byte(fmt.Sprintf("%s\r\n%s\r\nOK\r\n", command, line)),
		responseAfter: responseAfter,
	}
}

func (p *scriptedPort) SetMode(*serial.Mode) error {
	return nil
}

func (p *scriptedPort) Read(buffer []byte) (int, error) {
	p.mu.Lock()
	startedAt := p.startedAt
	if !startedAt.IsZero() && time.Since(startedAt) >= p.responseAfter && p.offset < len(p.response) {
		count := copy(buffer, p.response[p.offset:])
		p.offset += count
		p.mu.Unlock()
		return count, nil
	}
	p.mu.Unlock()

	time.Sleep(5 * time.Millisecond)
	return 0, nil
}

func (p *scriptedPort) Write(buffer []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.startedAt.IsZero() {
		p.startedAt = time.Now()
	}
	return len(buffer), nil
}

func (p *scriptedPort) Drain() error {
	return nil
}

func (p *scriptedPort) ResetInputBuffer() error {
	return nil
}

func (p *scriptedPort) ResetOutputBuffer() error {
	return nil
}

func (p *scriptedPort) SetDTR(bool) error {
	return nil
}

func (p *scriptedPort) SetRTS(bool) error {
	return nil
}

func (p *scriptedPort) GetModemStatusBits() (*serial.ModemStatusBits, error) {
	return &serial.ModemStatusBits{}, nil
}

func (p *scriptedPort) SetReadTimeout(time.Duration) error {
	return nil
}

func (p *scriptedPort) Close() error {
	return nil
}

func (p *scriptedPort) Break(time.Duration) error {
	return nil
}
