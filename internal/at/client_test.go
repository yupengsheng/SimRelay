package at

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type scriptedPort struct {
	reads  []string
	writes []string
}

func (p *scriptedPort) Read(b []byte) (int, error) {
	if len(p.reads) == 0 {
		return 0, io.EOF
	}
	next := p.reads[0]
	p.reads = p.reads[1:]
	copy(b, next)
	return len(next), nil
}

func (p *scriptedPort) Write(b []byte) (int, error) {
	p.writes = append(p.writes, string(b))
	return len(b), nil
}

func (p *scriptedPort) SetReadTimeout(time.Duration) error {
	return nil
}

func TestCommandReturnsResponseLinesBeforeOK(t *testing.T) {
	port := &scriptedPort{
		reads: []string{"\r\nQuectel\r\nEC20\r\nOK\r\n"},
	}
	client := NewClient(port, time.Second)

	lines, err := client.Command("AT+CGMI")
	if err != nil {
		t.Fatalf("Command returned error: %v", err)
	}

	if got, want := strings.Join(lines, "|"), "Quectel|EC20"; got != want {
		t.Fatalf("lines = %q, want %q", got, want)
	}
	if got, want := port.writes[0], "AT+CGMI\r"; got != want {
		t.Fatalf("write = %q, want %q", got, want)
	}
}

func TestCommandReturnsCommandError(t *testing.T) {
	port := &scriptedPort{
		reads: []string{"\r\nERROR\r\n"},
	}
	client := NewClient(port, time.Second)

	_, err := client.Command("AT+BAD")
	if !errors.Is(err, ErrCommand) {
		t.Fatalf("err = %v, want ErrCommand", err)
	}
}

func TestCommandSMSWaitsForPromptAndParsesFinalResponse(t *testing.T) {
	port := &scriptedPort{
		reads: []string{"> ", "\r\n+CMGS: 12\r\nOK\r\n"},
	}
	client := NewClient(port, time.Second)

	lines, err := client.CommandSMS(`AT+CMGS="002B0038003600310033"`, "4E2D6587")
	if err != nil {
		t.Fatalf("CommandSMS returned error: %v", err)
	}

	if got, want := strings.Join(lines, "|"), "+CMGS: 12"; got != want {
		t.Fatalf("lines = %q, want %q", got, want)
	}
	if got, want := strings.Join(port.writes, "|"), "AT+CMGS=\"002B0038003600310033\"\r|4E2D6587\x1a"; got != want {
		t.Fatalf("writes = %q, want %q", got, want)
	}
}
