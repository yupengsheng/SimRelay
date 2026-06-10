package modem

import (
	"context"
	"errors"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type FullModem interface {
	DeviceStatus() (DeviceStatus, error)
	ListMessages(box MessageBox) ([]Message, error)
	ReadMessage(index int) (Message, error)
	SendMessage(to string, text string) (SendResult, error)
	DeleteMessage(index int) error
	RawCommand(command string) ([]string, error)
	SendUSSD(code string) (USSDResult, error)
}

type QMIConfig struct {
	Device           string
	Command          string
	NetworkInterface string
	Timeout          time.Duration
	CacheTTL         time.Duration
	UseProxy         bool
}

type QMIEnhanced struct {
	base   FullModem
	config QMIConfig
	mu     sync.Mutex
	cache  qmiCache
}

type qmiCache struct {
	status    DeviceStatus
	expiresAt time.Time
	err       error
}

func NewQMIEnhanced(base FullModem, config QMIConfig) *QMIEnhanced {
	if config.Command == "" {
		config.Command = "qmicli"
	}
	if config.Timeout <= 0 {
		config.Timeout = time.Second
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = 15 * time.Second
	}
	return &QMIEnhanced{base: base, config: config}
}

func (m *QMIEnhanced) DeviceStatus() (DeviceStatus, error) {
	status, err := m.base.DeviceStatus()
	if err != nil {
		return status, err
	}
	status.Interface = firstNonEmptyString(status.Interface, m.config.NetworkInterface)
	status.ControlDevice = firstNonEmptyString(status.ControlDevice, m.config.Device)
	if m.config.Device == "" {
		deriveSIMIdentity(&status)
		return status, nil
	}

	qmiStatus, qmiErr := m.cachedQMIStatus()
	if qmiErr != nil {
		status.QMIError = qmiErr.Error()
		deriveSIMIdentity(&status)
		return status, nil
	}
	mergeDeviceStatus(&status, qmiStatus)
	status.QMIAvailable = true
	status.BackendMode = "qmi"
	status.Interface = firstNonEmptyString(status.Interface, m.config.NetworkInterface)
	status.ControlDevice = firstNonEmptyString(status.ControlDevice, m.config.Device)
	deriveSIMIdentity(&status)
	return status, nil
}

func (m *QMIEnhanced) ListMessages(box MessageBox) ([]Message, error) {
	return m.base.ListMessages(box)
}

func (m *QMIEnhanced) ReadMessage(index int) (Message, error) {
	return m.base.ReadMessage(index)
}

func (m *QMIEnhanced) SendMessage(to string, text string) (SendResult, error) {
	return m.base.SendMessage(to, text)
}

func (m *QMIEnhanced) DeleteMessage(index int) error {
	return m.base.DeleteMessage(index)
}

func (m *QMIEnhanced) RawCommand(command string) ([]string, error) {
	return m.base.RawCommand(command)
}

func (m *QMIEnhanced) SendUSSD(code string) (USSDResult, error) {
	return m.base.SendUSSD(code)
}

func (m *QMIEnhanced) cachedQMIStatus() (DeviceStatus, error) {
	now := time.Now()
	m.mu.Lock()
	if now.Before(m.cache.expiresAt) {
		status, err := m.cache.status, m.cache.err
		m.mu.Unlock()
		return status, err
	}
	m.mu.Unlock()

	status, err := m.queryQMIStatus()

	m.mu.Lock()
	m.cache = qmiCache{status: status, err: err, expiresAt: now.Add(m.config.CacheTTL)}
	m.mu.Unlock()
	return status, err
}

func (m *QMIEnhanced) queryQMIStatus() (DeviceStatus, error) {
	var status DeviceStatus
	queries := []struct {
		args  []string
		parse func(string, *DeviceStatus)
	}{
		{[]string{"--nas-get-serving-system"}, parseQMINASServingSystem},
		{[]string{"--nas-get-signal-info"}, parseQMINASSignalInfo},
		{[]string{"--nas-get-rf-band-info"}, parseQMINASRFBandInfo},
		{[]string{"--nas-get-home-network"}, parseQMINASHomeNetwork},
	}

	var errs []string
	for _, query := range queries {
		output, err := m.runQMI(query.args...)
		if err != nil {
			errs = append(errs, strings.TrimSpace(err.Error()))
			continue
		}
		query.parse(output, &status)
	}
	status.ControlDevice = m.config.Device
	status.Interface = m.config.NetworkInterface
	status.BackendMode = "qmi"
	deriveSIMIdentity(&status)
	if isEmptyQMIStatus(status) && len(errs) > 0 {
		return status, errors.New(firstNonEmptyString(errs...))
	}
	return status, nil
}

func (m *QMIEnhanced) runQMI(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.config.Timeout)
	defer cancel()

	commandArgs := []string{"-d", m.config.Device}
	if m.config.UseProxy {
		commandArgs = append(commandArgs, "--device-open-proxy")
	}
	commandArgs = append(commandArgs, args...)
	cmd := exec.CommandContext(ctx, m.config.Command, commandArgs...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(output), ctx.Err()
	}
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return string(output), errors.New(message)
	}
	return string(output), nil
}

func mergeDeviceStatus(base *DeviceStatus, extra DeviceStatus) {
	base.Manufacturer = firstNonEmptyString(extra.Manufacturer, base.Manufacturer)
	base.Model = firstNonEmptyString(extra.Model, base.Model)
	base.Firmware = firstNonEmptyString(extra.Firmware, base.Firmware)
	base.IMEI = firstNonEmptyString(extra.IMEI, base.IMEI)
	base.ICCID = firstNonEmptyString(extra.ICCID, base.ICCID)
	base.IMSI = firstNonEmptyString(extra.IMSI, base.IMSI)
	base.LocalPhone = firstNonEmptyString(extra.LocalPhone, base.LocalPhone)
	base.Operator = firstNonEmptyString(extra.Operator, base.Operator)
	base.NativeSPN = firstNonEmptyString(extra.NativeSPN, base.NativeSPN)
	base.NativeMCC = firstNonEmptyString(extra.NativeMCC, base.NativeMCC)
	base.NativeMNC = firstNonEmptyString(extra.NativeMNC, base.NativeMNC)
	base.HomeOperator = firstNonEmptyString(extra.HomeOperator, base.HomeOperator)
	base.NetworkMode = firstNonEmptyString(extra.NetworkMode, base.NetworkMode)
	base.NetworkDuplex = firstNonEmptyString(extra.NetworkDuplex, base.NetworkDuplex)
	base.RadioBand = firstNonEmptyString(extra.RadioBand, base.RadioBand)
	if extra.RadioChannel != 0 {
		base.RadioChannel = extra.RadioChannel
	}
	if extra.SignalRSSI != 0 {
		base.SignalRSSI = extra.SignalRSSI
	}
	if extra.SignalBER != 0 {
		base.SignalBER = extra.SignalBER
	}
	if extra.SignalDBM != 0 {
		base.SignalDBM = extra.SignalDBM
	}
	if extra.SignalSINR != 0 {
		base.SignalSINR = extra.SignalSINR
	}
	base.Registered = base.Registered || extra.Registered
	base.PSAttached = base.PSAttached || extra.PSAttached
	base.Interface = firstNonEmptyString(extra.Interface, base.Interface)
	base.ControlDevice = firstNonEmptyString(extra.ControlDevice, base.ControlDevice)
	base.BackendMode = firstNonEmptyString(extra.BackendMode, base.BackendMode)
}

func parseQMIDMSManufacturer(output string, status *DeviceStatus) {
	status.Manufacturer = firstNonEmptyString(extractQMIQuoted(output, "Manufacturer"), status.Manufacturer)
}

func parseQMIDMSModel(output string, status *DeviceStatus) {
	status.Model = firstNonEmptyString(extractQMIQuoted(output, "Model"), status.Model)
}

func parseQMIDMSRevision(output string, status *DeviceStatus) {
	status.Firmware = firstNonEmptyString(extractQMIQuoted(output, "Revision"), status.Firmware)
}

func parseQMIDMSIDs(output string, status *DeviceStatus) {
	status.IMEI = firstNonEmptyString(extractQMIQuoted(output, "IMEI"), status.IMEI)
}

func parseQMIDMSICCID(output string, status *DeviceStatus) {
	if value := extractQMIQuoted(output, "ICCID"); value != "" {
		status.ICCID = normalizeICCID(value)
	}
}

func parseQMIDMSIMSI(output string, status *DeviceStatus) {
	status.IMSI = firstNonEmptyString(extractQMIQuoted(output, "IMSI"), status.IMSI)
}

func parseQMIDMSMSISDN(output string, status *DeviceStatus) {
	status.LocalPhone = firstNonEmptyString(extractQMIQuoted(output, "Voice number"), status.LocalPhone)
	status.LocalPhone = firstNonEmptyString(extractQMIQuoted(output, "MSISDN"), status.LocalPhone)
}

func parseQMINASServingSystem(output string, status *DeviceStatus) {
	if value := extractQMIQuoted(output, "Registration state"); value != "" {
		status.Registered = strings.EqualFold(value, "registered")
	}
	if value := extractQMIQuoted(output, "PS"); value != "" {
		status.PSAttached = strings.Contains(strings.ToLower(value), "attached")
	}
	if value := extractQMIQuoted(output, "Radio interfaces"); value != "" {
		status.NetworkMode = qmiNetworkMode(value)
	}
	if mcc := extractQMIQuoted(output, "MCC"); mcc != "" {
		mnc := extractQMIQuoted(output, "MNC")
		if operator := operatorFromMCCMNC(mcc, mnc); operator != "" {
			status.Operator = operator
		}
	}
	if value := extractQMIQuoted(output, "Description"); value != "" {
		status.Operator = normalizeOperatorName(value)
	}
	if value := extractQMIQuoted(output, "Roaming status"); value != "" && strings.EqualFold(value, "off") && status.Operator == "" {
		status.Operator = status.HomeOperator
	}
}

func parseQMINASSystemInfo(output string, status *DeviceStatus) {
	if strings.Contains(output, "LTE service") {
		status.NetworkMode = "LTE"
	}
	if value := extractQMIQuoted(output, "MCC"); value != "" {
		status.NativeMCC = value
	}
	if value := extractQMIQuoted(output, "MNC"); value != "" {
		status.NativeMNC = value
	}
	if value := extractQMIQuoted(output, "Description"); value != "" {
		status.Operator = normalizeOperatorName(value)
	}
	if value := extractQMIQuoted(output, "Service capability"); value != "" {
		status.NetworkMode = qmiNetworkMode(value)
	}
	if value := extractQMIQuoted(output, "Tracking Area Code"); value != "" {
		_ = value
	}
}

func parseQMINASSignalInfo(output string, status *DeviceStatus) {
	status.SignalDBM = firstNonZeroInt(extractFirstInt(output, "RSSI"), status.SignalDBM)
	status.SignalSINR = firstNonZeroInt(extractFirstInt(output, "SINR"), extractFirstInt(output, "SNR"), status.SignalSINR)
	status.SignalRSSI = firstNonZeroInt(dbmToRSSI(status.SignalDBM), status.SignalRSSI)
	if strings.Contains(output, "LTE") {
		status.NetworkMode = "LTE"
	}
}

func parseQMINASSignalStrength(output string, status *DeviceStatus) {
	status.SignalDBM = firstNonZeroInt(extractFirstInt(output, "Current"), status.SignalDBM)
	status.SignalRSSI = firstNonZeroInt(dbmToRSSI(status.SignalDBM), status.SignalRSSI)
	if value := extractQMIQuoted(output, "Radio interface"); value != "" {
		status.NetworkMode = qmiNetworkMode(value)
	}
}

func parseQMINASRFBandInfo(output string, status *DeviceStatus) {
	if value := extractQMIQuoted(output, "Radio Interface"); value != "" {
		status.NetworkMode = qmiNetworkMode(value)
	}
	if value := extractQMIQuoted(output, "Active Band Class"); value != "" {
		status.RadioBand = normalizeBand(value)
	}
	status.RadioChannel = firstNonZeroInt(extractFirstInt(output, "Active Channel"), status.RadioChannel)
}

func parseQMINASHomeNetwork(output string, status *DeviceStatus) {
	mcc := extractQMIQuoted(output, "MCC")
	mnc := extractQMIQuoted(output, "MNC")
	if mcc != "" {
		status.NativeMCC = mcc
	}
	if mnc != "" {
		status.NativeMNC = mnc
	}
	status.NativeSPN = firstNonEmptyString(extractQMIQuoted(output, "Description"), status.NativeSPN)
	if status.HomeOperator == "" {
		status.HomeOperator = firstNonEmptyString(operatorFromMCCMNC(status.NativeMCC, status.NativeMNC), normalizeOperatorName(status.NativeSPN))
	}
}

func parseQMINASOperatorName(output string, status *DeviceStatus) {
	for _, key := range []string{"Service Provider Name", "Operator PLMN Name", "PLMN Name", "Name"} {
		if value := extractQMIQuoted(output, key); value != "" {
			status.Operator = normalizeOperatorName(value)
			return
		}
	}
}

func extractQMIQuoted(output string, key string) string {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?mi)^\s*` + regexp.QuoteMeta(key) + `\s*:\s*'([^']*)'`),
		regexp.MustCompile(`(?mi)^\s*` + regexp.QuoteMeta(key) + `\s*:\s*"([^"]*)"`),
		regexp.MustCompile(`(?mi)^\s*` + regexp.QuoteMeta(key) + `\s*:\s*([^\r\n]+)`),
	}
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(output)
		if len(match) == 2 {
			return strings.TrimSpace(strings.Trim(match[1], `"'`))
		}
	}
	return ""
}

func extractFirstInt(output string, key string) int {
	pattern := regexp.MustCompile(`(?mi)^\s*` + regexp.QuoteMeta(key) + `[^:\r\n]*:\s*['"]?(-?\d+)`)
	match := pattern.FindStringSubmatch(output)
	if len(match) != 2 {
		return 0
	}
	value, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	return value
}

func qmiNetworkMode(value string) string {
	value = strings.ToUpper(value)
	switch {
	case strings.Contains(value, "LTE"):
		return "LTE"
	case strings.Contains(value, "WCDMA"), strings.Contains(value, "UMTS"):
		return "UMTS"
	case strings.Contains(value, "GSM"), strings.Contains(value, "GERAN"):
		return "GSM"
	default:
		return strings.TrimSpace(value)
	}
}

func normalizeBand(value string) string {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "e-utra-operating-band-") {
		return "LTE BAND " + strings.TrimPrefix(lower, "e-utra-operating-band-")
	}
	if strings.HasPrefix(lower, "e-utra band ") {
		return "LTE BAND " + strings.TrimSpace(strings.TrimPrefix(lower, "e-utra band "))
	}
	value = strings.TrimPrefix(value, "E-UTRA ")
	value = strings.ReplaceAll(value, "Class ", "")
	return value
}

func dbmToRSSI(dbm int) int {
	if dbm == 0 {
		return 0
	}
	rssi := (dbm + 113) / 2
	if rssi < 0 {
		return 0
	}
	if rssi > 31 {
		return 31
	}
	return rssi
}

func firstNonZeroInt(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func isEmptyQMIStatus(status DeviceStatus) bool {
	return status.Manufacturer == "" &&
		status.Model == "" &&
		status.Firmware == "" &&
		status.IMEI == "" &&
		status.ICCID == "" &&
		status.IMSI == "" &&
		status.Operator == "" &&
		status.NativeSPN == "" &&
		status.NetworkMode == "" &&
		status.RadioBand == "" &&
		status.SignalDBM == 0 &&
		!status.Registered &&
		!status.PSAttached
}
