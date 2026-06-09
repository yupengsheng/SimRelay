package modem

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
)

var ErrParse = errors.New("模组响应格式无法解析")

type ATClient interface {
	Command(command string) ([]string, error)
	CommandSMS(command string, payload string) ([]string, error)
}

type EC20 struct {
	at ATClient
}

func NewEC20(client ATClient) *EC20 {
	return &EC20{at: client}
}

type DeviceStatus struct {
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	IMEI         string `json:"imei"`
	SIM          string `json:"sim"`
	SignalRSSI   int    `json:"signal_rssi"`
	SignalBER    int    `json:"signal_ber"`
	Registered   bool   `json:"registered"`
}

type MessageBox string

const (
	BoxAll    MessageBox = "all"
	BoxInbox  MessageBox = "inbox"
	BoxSent   MessageBox = "sent"
	BoxUnread MessageBox = "unread"
)

type Message struct {
	Index     int    `json:"index"`
	Status    string `json:"status"`
	From      string `json:"from"`
	Timestamp string `json:"timestamp,omitempty"`
	Text      string `json:"text"`
}

type SendResult struct {
	Reference int `json:"reference"`
}

func (m *EC20) Init() error {
	commands := []string{
		"AT",
		"ATE0",
		"AT+CMGF=1",
		`AT+CSCS="UCS2"`,
		"AT+CSMP=17,167,0,8",
		`AT+CPMS="SM","SM","SM"`,
	}
	for _, command := range commands {
		if _, err := m.at.Command(command); err != nil {
			return err
		}
	}
	return nil
}

func (m *EC20) DeviceStatus() (DeviceStatus, error) {
	var status DeviceStatus
	if value, err := firstLine(m.at.Command("AT+CGMI")); err != nil {
		return status, err
	} else {
		status.Manufacturer = value
	}
	if value, err := firstLine(m.at.Command("AT+CGMM")); err != nil {
		return status, err
	} else {
		status.Model = value
	}
	if value, err := firstLine(m.at.Command("AT+CGSN")); err != nil {
		return status, err
	} else {
		status.IMEI = value
	}
	if value, err := firstLine(m.at.Command("AT+CPIN?")); err != nil {
		return status, err
	} else {
		status.SIM = strings.TrimSpace(strings.TrimPrefix(value, "+CPIN:"))
	}
	if value, err := firstLine(m.at.Command("AT+CSQ")); err != nil {
		return status, err
	} else if err := parseCSQ(value, &status); err != nil {
		return status, err
	}
	if value, err := firstLine(m.at.Command("AT+CREG?")); err != nil {
		return status, err
	} else if err := parseCREG(value, &status); err != nil {
		return status, err
	}
	return status, nil
}

func (m *EC20) ListMessages(box MessageBox) ([]Message, error) {
	lines, err := m.at.Command("AT+CMGL=" + boxCode(box))
	if err != nil {
		return nil, err
	}
	return parseMessages(lines)
}

func (m *EC20) ReadMessage(index int) (Message, error) {
	lines, err := m.at.Command("AT+CMGR=" + strconv.Itoa(index))
	if err != nil {
		return Message{}, err
	}
	if len(lines) < 2 {
		return Message{}, ErrParse
	}
	message, err := parseMessageHeader("+CMGL: "+strconv.Itoa(index)+","+strings.TrimPrefix(lines[0], "+CMGR: "), lines[1])
	if err != nil {
		return Message{}, err
	}
	message.Index = index
	return message, nil
}

func (m *EC20) SendMessage(to string, text string) (SendResult, error) {
	command := fmt.Sprintf(`AT+CMGS="%s"`, EncodeUCS2(to))
	lines, err := m.at.CommandSMS(command, EncodeUCS2(text))
	if err != nil {
		return SendResult{}, err
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "+CMGS:") {
			reference, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "+CMGS:")))
			if err != nil {
				return SendResult{}, ErrParse
			}
			return SendResult{Reference: reference}, nil
		}
	}
	return SendResult{}, ErrParse
}

func EncodeUCS2(value string) string {
	units := utf16.Encode([]rune(value))
	var builder strings.Builder
	for _, unit := range units {
		builder.WriteString(fmt.Sprintf("%04X", unit))
	}
	return builder.String()
}

func DecodeUCS2(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	raw, err := hex.DecodeString(value)
	if err != nil {
		return "", err
	}
	if len(raw)%2 != 0 {
		return "", ErrParse
	}
	units := make([]uint16, 0, len(raw)/2)
	for i := 0; i < len(raw); i += 2 {
		units = append(units, uint16(raw[i])<<8|uint16(raw[i+1]))
	}
	return string(utf16.Decode(units)), nil
}

func firstLine(lines []string, err error) (string, error) {
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", ErrParse
	}
	return strings.TrimSpace(lines[0]), nil
}

func parseCSQ(line string, status *DeviceStatus) error {
	values := strings.TrimSpace(strings.TrimPrefix(line, "+CSQ:"))
	parts := strings.Split(values, ",")
	if len(parts) != 2 {
		return ErrParse
	}
	rssi, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return ErrParse
	}
	ber, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return ErrParse
	}
	status.SignalRSSI = rssi
	status.SignalBER = ber
	return nil
}

func parseCREG(line string, status *DeviceStatus) error {
	values := strings.TrimSpace(strings.TrimPrefix(line, "+CREG:"))
	parts := strings.Split(values, ",")
	if len(parts) < 2 {
		return ErrParse
	}
	stat, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return ErrParse
	}
	status.Registered = stat == 1 || stat == 5
	return nil
}

func boxCode(box MessageBox) string {
	switch box {
	case BoxInbox:
		return "1"
	case BoxSent:
		return "3"
	case BoxUnread:
		return "0"
	default:
		return "4"
	}
}

func parseMessages(lines []string) ([]Message, error) {
	messages := make([]Message, 0)
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(lines[i], "+CMGL:") {
			continue
		}
		if i+1 >= len(lines) {
			return nil, ErrParse
		}
		message, err := parseMessageHeader(lines[i], lines[i+1])
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
		i++
	}
	return messages, nil
}

func parseMessageHeader(header string, body string) (Message, error) {
	values := strings.TrimSpace(strings.TrimPrefix(header, "+CMGL:"))
	parts := splitCSV(values)
	if len(parts) < 3 {
		return Message{}, ErrParse
	}
	index, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return Message{}, ErrParse
	}
	from, err := DecodeUCS2(unquote(parts[2]))
	if err != nil {
		return Message{}, err
	}
	text, err := DecodeUCS2(body)
	if err != nil {
		return Message{}, err
	}
	message := Message{
		Index:  index,
		Status: unquote(parts[1]),
		From:   from,
		Text:   text,
	}
	if len(parts) >= 5 {
		message.Timestamp = unquote(parts[4])
	}
	return message, nil
}

func splitCSV(value string) []string {
	var parts []string
	var builder strings.Builder
	inQuote := false
	for _, r := range value {
		switch r {
		case '"':
			inQuote = !inQuote
			builder.WriteRune(r)
		case ',':
			if inQuote {
				builder.WriteRune(r)
				continue
			}
			parts = append(parts, strings.TrimSpace(builder.String()))
			builder.Reset()
		default:
			builder.WriteRune(r)
		}
	}
	parts = append(parts, strings.TrimSpace(builder.String()))
	return parts
}

func unquote(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1]
	}
	return value
}
