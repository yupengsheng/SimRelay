package at

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

var (
	ErrCommand = errors.New("AT 命令执行失败")
	ErrTimeout = errors.New("AT 命令响应超时")
)

type Port interface {
	io.Reader
	io.Writer
	SetReadTimeout(time.Duration) error
}

type Client struct {
	port    Port
	timeout time.Duration
	mu      sync.Mutex
}

func NewClient(port Port, timeout time.Duration) *Client {
	return &Client{port: port, timeout: timeout}
}

func (c *Client) Command(command string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.write(command + "\r"); err != nil {
		return nil, err
	}
	return c.readFinalResponse(command, false)
}

func (c *Client) CommandSMS(command string, payload string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.write(command + "\r"); err != nil {
		return nil, err
	}
	if err := c.waitForPrompt(); err != nil {
		return nil, err
	}
	if err := c.write(payload + "\x1a"); err != nil {
		return nil, err
	}
	return c.readFinalResponse(command, false)
}

func (c *Client) write(value string) error {
	if c.timeout > 0 {
		if err := c.port.SetReadTimeout(c.timeout); err != nil {
			return fmt.Errorf("设置串口超时失败: %w", err)
		}
	}
	_, err := c.port.Write([]byte(value))
	if err != nil {
		return fmt.Errorf("写入串口失败: %w", err)
	}
	return nil
}

func (c *Client) waitForPrompt() error {
	var buf bytes.Buffer
	tmp := make([]byte, 128)
	for {
		n, err := c.port.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
			if strings.Contains(buf.String(), ">") {
				return nil
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return ErrTimeout
			}
			return fmt.Errorf("读取短信提示符失败: %w", err)
		}
	}
}

func (c *Client) readFinalResponse(command string, includePrompt bool) ([]string, error) {
	var buf bytes.Buffer
	tmp := make([]byte, 512)
	for {
		n, err := c.port.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
			lines, final, commandErr := parseLines(buf.String(), command, includePrompt)
			if commandErr {
				return nil, ErrCommand
			}
			if final {
				return lines, nil
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, ErrTimeout
			}
			return nil, fmt.Errorf("读取串口响应失败: %w", err)
		}
	}
}

func parseLines(raw string, command string, includePrompt bool) ([]string, bool, bool) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	parts := strings.Split(raw, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		line := strings.TrimSpace(part)
		if line == "" {
			continue
		}
		if !includePrompt && line == ">" {
			continue
		}
		if command != "" && line == command {
			continue
		}
		switch line {
		case "OK":
			return lines, true, false
		case "ERROR":
			return nil, true, true
		}
		if strings.HasPrefix(line, "+CME ERROR:") || strings.HasPrefix(line, "+CMS ERROR:") {
			return nil, true, true
		}
		lines = append(lines, line)
	}
	return lines, false, false
}
