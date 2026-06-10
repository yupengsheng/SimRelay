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
	Manufacturer  string `json:"manufacturer"`
	Model         string `json:"model"`
	Firmware      string `json:"firmware,omitempty"`
	IMEI          string `json:"imei"`
	ICCID         string `json:"iccid,omitempty"`
	IMSI          string `json:"imsi,omitempty"`
	LocalPhone    string `json:"local_phone,omitempty"`
	SIM           string `json:"sim"`
	Operator      string `json:"operator,omitempty"`
	NativeSPN     string `json:"native_spn,omitempty"`
	NativeMCC     string `json:"native_mcc,omitempty"`
	NativeMNC     string `json:"native_mnc,omitempty"`
	HomeOperator  string `json:"home_operator,omitempty"`
	NetworkMode   string `json:"network_mode,omitempty"`
	NetworkDuplex string `json:"network_duplex,omitempty"`
	RadioBand     string `json:"radio_band,omitempty"`
	RadioChannel  int    `json:"radio_channel,omitempty"`
	SignalRSSI    int    `json:"signal_rssi"`
	SignalBER     int    `json:"signal_ber"`
	SignalDBM     int    `json:"signal_dbm,omitempty"`
	SignalSINR    int    `json:"signal_sinr,omitempty"`
	Registered    bool   `json:"registered"`
	PSAttached    bool   `json:"ps_attached,omitempty"`
	Interface     string `json:"interface,omitempty"`
	ControlDevice string `json:"control_device,omitempty"`
	BackendMode   string `json:"backend_mode,omitempty"`
	QMIAvailable  bool   `json:"qmi_available,omitempty"`
	QMIError      string `json:"qmi_error,omitempty"`
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

type USSDResult struct {
	Status string `json:"status"`
	Text   string `json:"text"`
	Raw    string `json:"raw_text,omitempty"`
}

func (m *EC20) Init() error {
	commands := []string{
		"AT",
		"ATE0",
		"AT+CMGF=1",
		`AT+CSCS="UCS2"`,
		"AT+CSMP=17,167,0,8",
		`AT+CPMS="MT","MT","MT"`,
		"AT+CNMI=2,1,2,1,0",
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
	m.enrichDeviceStatus(&status)
	if status.BackendMode == "" {
		status.BackendMode = "at"
	}
	return status, nil
}

func (m *EC20) enrichDeviceStatus(status *DeviceStatus) {
	if value, err := firstLine(m.at.Command("AT+COPS?")); err == nil {
		parseCOPS(value, status)
	}
	if value, err := firstLine(m.at.Command("AT+QCCID")); err == nil {
		parseQCCID(value, status)
	}
	if value, err := firstLine(m.at.Command("AT+CIMI")); err == nil {
		parseCIMI(value, status)
	}
	if value, err := firstLine(m.at.Command("AT+QSPN")); err == nil {
		parseQSPN(value, status)
	}
	if value, err := firstLine(m.at.Command("AT+QNWINFO")); err == nil {
		parseQNWINFO(value, status)
	}
	if value, err := firstLine(m.at.Command("AT+CGMR")); err == nil {
		status.Firmware = strings.TrimSpace(value)
	}
	deriveSIMIdentity(status)
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

func (m *EC20) DeleteMessage(index int) error {
	_, err := m.at.Command("AT+CMGD=" + strconv.Itoa(index))
	return err
}

func (m *EC20) RawCommand(command string) ([]string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, ErrParse
	}
	return m.at.Command(command)
}

func (m *EC20) SendUSSD(code string) (USSDResult, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return USSDResult{}, ErrParse
	}
	lines, err := m.at.Command(fmt.Sprintf(`AT+CUSD=1,"%s",15`, code))
	if err != nil {
		return USSDResult{}, err
	}
	result := USSDResult{Status: "ok"}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "+CUSD:") {
			continue
		}
		result.Raw = line
		parts := splitCSV(strings.TrimSpace(strings.TrimPrefix(line, "+CUSD:")))
		if len(parts) > 0 {
			result.Status = strings.TrimSpace(parts[0])
		}
		if len(parts) > 1 {
			value := unquote(parts[1])
			if decoded, decodeErr := DecodeUCS2(value); decodeErr == nil {
				result.Text = decoded
			} else {
				result.Text = value
			}
		}
		return result, nil
	}
	result.Raw = strings.Join(lines, "\n")
	return result, nil
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

func parseCOPS(line string, status *DeviceStatus) {
	values := strings.TrimSpace(strings.TrimPrefix(line, "+COPS:"))
	parts := splitCSV(values)
	if len(parts) < 3 {
		return
	}
	format := strings.TrimSpace(parts[1])
	operator := unquote(parts[2])
	if format == "2" {
		if len(operator) >= 5 {
			status.Operator = operatorFromMCCMNC(operator[:3], operator[3:])
		}
	} else {
		status.Operator = normalizeOperatorName(operator)
	}
	if len(parts) >= 4 {
		status.NetworkMode = networkModeFromAccessTechnology(strings.TrimSpace(parts[3]))
	}
}

func parseQCCID(line string, status *DeviceStatus) {
	value := strings.TrimSpace(strings.TrimPrefix(line, "+QCCID:"))
	value = strings.Trim(value, `"'`)
	if value != "" {
		status.ICCID = normalizeICCID(value)
	}
}

func parseCIMI(line string, status *DeviceStatus) {
	value := strings.TrimSpace(line)
	if value != "" && allDigits(value) {
		status.IMSI = value
	}
}

func parseQSPN(line string, status *DeviceStatus) {
	values := strings.TrimSpace(strings.TrimPrefix(line, "+QSPN:"))
	parts := splitCSV(values)
	for _, part := range parts {
		value := strings.TrimSpace(unquote(part))
		if value == "" || value == "0" || value == "1" {
			continue
		}
		status.NativeSPN = value
		if status.HomeOperator == "" {
			status.HomeOperator = normalizeOperatorName(value)
		}
		return
	}
}

func parseQNWINFO(line string, status *DeviceStatus) {
	values := strings.TrimSpace(strings.TrimPrefix(line, "+QNWINFO:"))
	parts := splitCSV(values)
	if len(parts) < 4 {
		return
	}
	mode := strings.ToUpper(unquote(parts[0]))
	switch {
	case strings.Contains(mode, "LTE"):
		status.NetworkMode = "LTE"
	case strings.Contains(mode, "WCDMA") || strings.Contains(mode, "UMTS"):
		status.NetworkMode = "UMTS"
	case strings.Contains(mode, "GSM"):
		status.NetworkMode = "GSM"
	}
	if strings.Contains(mode, "FDD") {
		status.NetworkDuplex = "FDD"
	} else if strings.Contains(mode, "TDD") {
		status.NetworkDuplex = "TDD"
	}
	plmn := unquote(parts[1])
	if status.Operator == "" && len(plmn) >= 5 {
		status.Operator = operatorFromMCCMNC(plmn[:3], plmn[3:])
	}
	status.RadioBand = unquote(parts[2])
	if channel, err := strconv.Atoi(strings.TrimSpace(unquote(parts[3]))); err == nil {
		status.RadioChannel = channel
	}
}

func boxCode(box MessageBox) string {
	switch box {
	case BoxInbox:
		return `"REC READ"`
	case BoxSent:
		return `"STO SENT"`
	case BoxUnread:
		return `"REC UNREAD"`
	default:
		return `"ALL"`
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

func deriveSIMIdentity(status *DeviceStatus) {
	if len(status.IMSI) >= 5 {
		if status.NativeMCC == "" {
			status.NativeMCC = status.IMSI[:3]
		}
		homeOperator, nativeMNC := homeOperatorFromIMSI(status.IMSI, status.NativeMNC)
		if nativeMNC != "" {
			status.NativeMNC = nativeMNC
		}
		if homeOperator != "" {
			status.HomeOperator = homeOperator
			if status.NativeSPN == "" || normalizeOperatorName(status.NativeSPN) == normalizeOperatorName(status.Operator) {
				status.NativeSPN = homeOperator
			}
		}
		if status.HomeOperator == "" {
			status.HomeOperator = operatorFromMCCMNC(status.NativeMCC, status.NativeMNC)
		}
	}
	if status.Operator == "" {
		status.Operator = status.HomeOperator
	}
	if status.NetworkMode == "" && status.Registered {
		status.NetworkMode = "LTE"
	}
	if status.SignalDBM == 0 && status.SignalRSSI > 0 {
		status.SignalDBM = rssiToDBM(status.SignalRSSI)
	}
	if status.PSAttached == false && status.Registered {
		status.PSAttached = true
	}
}

func normalizeOperatorName(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "unknown":
		return ""
	case "china mobile", "chinamobile", "cmcc", "46000", "46002", "46004", "46007", "46008":
		return "中国移动"
	case "china unicom", "chinaunicom", "cucc", "46001", "46006", "46009":
		return "中国联通"
	case "china telecom", "chinatelecom", "ctcc", "46003", "46005", "46011":
		return "中国电信"
	default:
		return strings.TrimSpace(value)
	}
}

func operatorFromMCCMNC(mcc string, mnc string) string {
	mcc = strings.TrimSpace(mcc)
	mnc = strings.TrimSpace(mnc)
	if mcc == "460" && len(mnc) == 1 {
		mnc = "0" + mnc
	}
	switch mcc + mnc {
	case "46000", "46002", "46004", "46007", "46008":
		return "中国移动"
	case "46001", "46006", "46009":
		return "中国联通"
	case "46003", "46005", "46011":
		return "中国电信"
	case "23410":
		return "giffgaff"
	case "23415":
		return "Vodafone UK"
	case "23420":
		return "Three UK"
	case "23430", "23433":
		return "EE"
	default:
		return ""
	}
}

func homeOperatorFromIMSI(imsi string, currentMNC string) (string, string) {
	if len(imsi) < 5 {
		return "", strings.TrimSpace(currentMNC)
	}
	mcc := imsi[:3]
	candidates := []string{strings.TrimSpace(currentMNC), imsi[3:5]}
	if len(imsi) >= 6 {
		candidates = append(candidates, imsi[3:6])
	}
	for _, mnc := range candidates {
		if mnc == "" {
			continue
		}
		if operator := operatorFromMCCMNC(mcc, mnc); operator != "" {
			return operator, normalizeMNC(mnc)
		}
	}
	if currentMNC != "" {
		return "", normalizeMNC(currentMNC)
	}
	return "", imsi[3:5]
}

func normalizeMNC(value string) string {
	value = strings.TrimSpace(value)
	if len(value) == 1 {
		return "0" + value
	}
	return value
}

func normalizeICCID(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimRight(value, "Ff")
	return value
}

func networkModeFromAccessTechnology(value string) string {
	switch strings.TrimSpace(value) {
	case "0":
		return "GSM"
	case "2":
		return "UMTS"
	case "7":
		return "LTE"
	default:
		return ""
	}
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func rssiToDBM(rssi int) int {
	if rssi <= 0 || rssi == 99 {
		return 0
	}
	if rssi > 31 {
		return rssi
	}
	return -113 + 2*rssi
}
