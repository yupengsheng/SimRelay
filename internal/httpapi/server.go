package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yupengsheng/SimRelay/internal/at"
	"github.com/yupengsheng/SimRelay/internal/modem"
	"github.com/yupengsheng/SimRelay/internal/web"
)

type Modem interface {
	DeviceStatus() (modem.DeviceStatus, error)
	ListMessages(box modem.MessageBox) ([]modem.Message, error)
	ReadMessage(index int) (modem.Message, error)
	SendMessage(to string, text string) (modem.SendResult, error)
	DeleteMessage(index int) error
	RawCommand(command string) ([]string, error)
	SendUSSD(code string) (modem.USSDResult, error)
}

type Server struct {
	modem        Modem
	mux          *http.ServeMux
	authUsername string
	authPassword string
}

func New(modem Modem) http.Handler {
	server := &Server{
		modem:        modem,
		mux:          http.NewServeMux(),
		authUsername: envString("SIMRELAY_ADMIN_USERNAME", "admin"),
		authPassword: envString("SIMRELAY_ADMIN_PASSWORD", "admin"),
	}
	server.routes()
	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("POST /api/auth/login", s.login)
	s.mux.HandleFunc("GET /api/dashboard/devices", s.dashboardDevices)
	s.mux.HandleFunc("GET /api/devices", s.vohiveDevices)
	s.mux.HandleFunc("POST /api/devices", s.unsupportedAction)
	s.mux.HandleFunc("GET /api/devices/", s.vohiveDeviceRoute)
	s.mux.HandleFunc("POST /api/devices/", s.vohiveDeviceRoute)
	s.mux.HandleFunc("PATCH /api/devices/", s.vohiveDeviceRoute)
	s.mux.HandleFunc("PUT /api/devices/", s.vohiveDeviceRoute)
	s.mux.HandleFunc("DELETE /api/devices/", s.vohiveDeviceRoute)
	s.mux.HandleFunc("POST /api/devices/actions/rescan", s.acceptedAction)
	s.mux.HandleFunc("GET /api/devices/discovered", s.discoveredDevices)
	s.mux.HandleFunc("POST /api/device-mgmt/discovered/fix-usbnet", s.unsupportedAction)
	s.mux.HandleFunc("GET /api/sms/contacts", s.vohiveSMSContacts)
	s.mux.HandleFunc("GET /api/sms/thread", s.vohiveSMSThread)
	s.mux.HandleFunc("POST /api/sms/send", s.vohiveSMSSend)
	s.mux.HandleFunc("DELETE /api/sms/messages/", s.vohiveSMSDeleteMessage)
	s.mux.HandleFunc("DELETE /api/sms/thread", s.vohiveSMSDeleteThread)
	s.mux.HandleFunc("GET /api/traffic/analysis", s.trafficAnalysis)
	s.mux.HandleFunc("GET /api/logs", s.logs)
	s.mux.HandleFunc("GET /api/logs/stream", s.logsStream)
	s.mux.HandleFunc("GET /api/settings", s.settings)
	s.mux.HandleFunc("PUT /api/settings", s.saveSettings)
	s.mux.HandleFunc("POST /api/settings/password", s.unsupportedAction)
	s.mux.HandleFunc("GET /api/proxies", s.proxyList)
	s.mux.HandleFunc("POST /api/proxies", s.unsupportedAction)
	s.mux.HandleFunc("GET /api/proxy", s.proxyList)
	s.mux.HandleFunc("POST /api/proxy", s.unsupportedAction)
	s.mux.HandleFunc("POST /api/rotateip", s.unsupportedAction)
	s.mux.HandleFunc("GET /api/v1/device", s.device)
	s.mux.HandleFunc("GET /api/v1/messages", s.listMessages)
	s.mux.HandleFunc("GET /api/v1/messages/", s.readMessage)
	s.mux.HandleFunc("POST /api/v1/messages", s.sendMessage)
	s.mux.Handle("GET /assets/", http.FileServerFS(staticFS()))
	s.mux.HandleFunc("GET /", s.index)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVoHiveError(w, http.StatusBadRequest, "请求 JSON 格式非法")
		return
	}
	if req.Username != s.authUsername || req.Password != s.authPassword {
		writeVoHiveError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"token":      fmt.Sprintf("%d.simrelay", time.Now().Unix()),
		"expires_at": time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339),
	})
}

func (s *Server) device(w http.ResponseWriter, _ *http.Request) {
	status, err := s.modem.DeviceStatus()
	if err != nil {
		writeError(w, statusForError(err), "读取设备状态失败")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) listMessages(w http.ResponseWriter, r *http.Request) {
	box := modem.MessageBox(r.URL.Query().Get("box"))
	if box == "" {
		box = modem.BoxAll
	}
	messages, err := s.modem.ListMessages(box)
	if err != nil {
		writeError(w, statusForError(err), "读取短信列表失败")
		return
	}
	writeJSON(w, http.StatusOK, messages)
}

func (s *Server) readMessage(w http.ResponseWriter, r *http.Request) {
	rawIndex := strings.TrimPrefix(r.URL.Path, "/api/v1/messages/")
	index, err := strconv.Atoi(rawIndex)
	if err != nil || index <= 0 {
		writeError(w, http.StatusBadRequest, "短信索引非法")
		return
	}
	message, err := s.modem.ReadMessage(index)
	if err != nil {
		writeError(w, statusForError(err), "读取短信失败")
		return
	}
	writeJSON(w, http.StatusOK, message)
}

func (s *Server) sendMessage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		To   string `json:"to"`
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求 JSON 格式非法")
		return
	}
	req.To = strings.TrimSpace(req.To)
	if req.To == "" || req.Text == "" {
		writeError(w, http.StatusBadRequest, "号码和短信内容不能为空")
		return
	}
	result, err := s.modem.SendMessage(req.To, req.Text)
	if err != nil {
		writeError(w, statusForError(err), "发送短信失败")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) vohiveDevices(w http.ResponseWriter, _ *http.Request) {
	status, err := s.modem.DeviceStatus()
	if err != nil {
		writeVoHiveError(w, statusForError(err), "读取设备列表失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": []any{s.deviceFromStatus(status)}})
}

func (s *Server) dashboardDevices(w http.ResponseWriter, _ *http.Request) {
	status, err := s.modem.DeviceStatus()
	if err != nil {
		writeVoHiveError(w, statusForError(err), "读取仪表盘失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": []any{s.deviceFromStatus(status)}})
}

func (s *Server) vohiveDeviceRoute(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/devices/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeVoHiveError(w, http.StatusNotFound, "API 不存在")
		return
	}
	deviceID := parts[0]
	if len(parts) == 1 {
		s.vohiveDeviceOverview(w, r)
		return
	}
	switch strings.Join(parts[1:], "/") {
	case "overview":
		s.vohiveDeviceOverview(w, r)
	case "overview/stream":
		s.overviewStream(w, r)
	case "config":
		s.deviceConfig(w, deviceID)
	case "actions/at":
		s.sendAT(w, r)
	case "actions/ussd":
		s.sendUSSD(w, r)
	case "actions/ussd/continue", "actions/ussd/cancel":
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": "SimRelay 当前使用单次 USSD 模式"})
	case "actions/reboot":
		_, err := s.modem.RawCommand("AT+CFUN=1,1")
		if err != nil {
			writeVoHiveError(w, statusForError(err), "重启模组指令下发失败")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	case "actions/refresh":
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	case "network", "flight-mode", "usbnet-mode", "vowifi/actions/reconnect":
		s.unsupportedAction(w, r)
	case "esim", "esim/profiles", "esim/notifications":
		writeJSON(w, http.StatusOK, map[string]any{"chip_info": nil, "profiles": []any{}, "items": []any{}})
	case "esim/notifications/actions/retry":
		s.unsupportedAction(w, r)
	default:
		writeVoHiveError(w, http.StatusNotFound, "API 不存在")
	}
}

func (s *Server) vohiveDeviceOverview(w http.ResponseWriter, _ *http.Request) {
	status, err := s.modem.DeviceStatus()
	if err != nil {
		writeVoHiveError(w, statusForError(err), "读取设备详情失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": []any{s.deviceFromStatus(status)}})
}

func (s *Server) deviceConfig(w http.ResponseWriter, deviceID string) {
	writeJSON(w, http.StatusOK, map[string]any{
		"config": map[string]any{
			"id":              deviceID,
			"name":            firstNonEmpty(os.Getenv("SIMRELAY_DEVICE_NAME"), deviceID),
			"interface":       os.Getenv("SIMRELAY_DEVICE"),
			"modem_imei":      "",
			"usb_path":        "",
			"apn":             "",
			"device_backend":  "at",
			"at_port":         os.Getenv("SIMRELAY_DEVICE"),
			"control_device":  "",
			"network_enabled": false,
			"vowifi_enabled":  false,
			"esim_transport":  "at",
		},
	})
}

func (s *Server) discoveredDevices(w http.ResponseWriter, _ *http.Request) {
	device := os.Getenv("SIMRELAY_DEVICE")
	if device == "" {
		device = "/dev/ttyUSB2"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"devices": []map[string]any{
			{
				"discovery_key":      device,
				"control_path":       "",
				"net_interface":      device,
				"usb_path":           "",
				"vendor_id":          0,
				"product_id":         0,
				"driver_name":        "usbserial",
				"at_ports":           []string{device},
				"at_port":            device,
				"mode":               "at",
				"network_capable":    false,
				"configured":         true,
				"configured_id":      "simrelay",
				"path_configured_id": "simrelay",
				"match_kind":         "simrelay",
			},
		},
	})
}

func (s *Server) overviewStream(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("event: overview\n"))
	_, _ = w.Write([]byte(`data: {"devices":[]}` + "\n\n"))
}

func (s *Server) vohiveSMSContacts(w http.ResponseWriter, r *http.Request) {
	messages, err := s.modem.ListMessages(modem.BoxAll)
	if err != nil {
		writeVoHiveError(w, statusForError(err), "读取短信联系人失败")
		return
	}
	status, _ := s.modem.DeviceStatus()
	contacts := summarizeContacts(messages, status)
	writeJSON(w, http.StatusOK, contacts)
}

func (s *Server) vohiveSMSThread(w http.ResponseWriter, r *http.Request) {
	peer := strings.TrimSpace(r.URL.Query().Get("peer"))
	messages, err := s.modem.ListMessages(modem.BoxAll)
	if err != nil {
		writeVoHiveError(w, statusForError(err), "读取短信会话失败")
		return
	}
	thread := make([]vohiveSMSMessage, 0)
	for _, message := range messages {
		if peer != "" && message.From != peer {
			continue
		}
		thread = append(thread, vohiveMessage(message))
	}
	sort.SliceStable(thread, func(i, j int) bool {
		return thread[i].ID < thread[j].ID
	})
	writeJSON(w, http.StatusOK, thread)
}

func (s *Server) vohiveSMSSend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DeviceID string `json:"device_id"`
		Phone    string `json:"phone"`
		To       string `json:"to"`
		Message  string `json:"message"`
		Text     string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVoHiveError(w, http.StatusBadRequest, "请求 JSON 格式非法")
		return
	}
	to := strings.TrimSpace(req.Phone)
	if to == "" {
		to = strings.TrimSpace(req.To)
	}
	body := req.Message
	if body == "" {
		body = req.Text
	}
	if to == "" || body == "" {
		writeVoHiveError(w, http.StatusBadRequest, "号码和短信内容不能为空")
		return
	}
	result, err := s.modem.SendMessage(to, body)
	if err != nil {
		writeVoHiveError(w, statusForError(err), "发送短信失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "reference": result.Reference, "parts_total": 1})
}

func (s *Server) vohiveSMSDeleteMessage(w http.ResponseWriter, r *http.Request) {
	rawIndex := strings.TrimPrefix(r.URL.Path, "/api/sms/messages/")
	index, err := strconv.Atoi(rawIndex)
	if err != nil || index <= 0 {
		writeVoHiveError(w, http.StatusBadRequest, "短信索引非法")
		return
	}
	if err := s.modem.DeleteMessage(index); err != nil {
		writeVoHiveError(w, statusForError(err), "删除短信失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) vohiveSMSDeleteThread(w http.ResponseWriter, r *http.Request) {
	peer := strings.TrimSpace(r.URL.Query().Get("peer"))
	if peer == "" {
		writeVoHiveError(w, http.StatusBadRequest, "缺少 peer 参数")
		return
	}
	messages, err := s.modem.ListMessages(modem.BoxAll)
	if err != nil {
		writeVoHiveError(w, statusForError(err), "读取短信会话失败")
		return
	}
	deleted := 0
	for _, message := range messages {
		if message.From != peer {
			continue
		}
		if err := s.modem.DeleteMessage(message.Index); err != nil {
			writeVoHiveError(w, statusForError(err), "删除会话失败")
			return
		}
		deleted++
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "deleted": deleted})
}

func (s *Server) sendAT(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVoHiveError(w, http.StatusBadRequest, "请求 JSON 格式非法")
		return
	}
	lines, err := s.modem.RawCommand(req.Command)
	if err != nil {
		writeVoHiveError(w, statusForError(err), "AT 命令执行失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "response": strings.Join(lines, "\n"), "lines": lines})
}

func (s *Server) sendUSSD(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVoHiveError(w, http.StatusBadRequest, "请求 JSON 格式非法")
		return
	}
	result, err := s.modem.SendUSSD(req.Code)
	if err != nil {
		writeVoHiveError(w, statusForError(err), "USSD 命令执行失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "result": result})
}

func (s *Server) trafficAnalysis(w http.ResponseWriter, _ *http.Request) {
	now := time.Now().Truncate(time.Hour)
	buckets := make([]map[string]any, 0, 24)
	for i := 23; i >= 0; i-- {
		buckets = append(buckets, map[string]any{
			"bucket":      now.Add(-time.Duration(i) * time.Hour).Format("2006-01-02 15:04"),
			"rx_bytes":    0,
			"tx_bytes":    0,
			"total_bytes": 0,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"buckets": buckets, "chart": nil})
}

func (s *Server) logs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"logs": []map[string]any{s.logEntry("INFO", "SimRelay 控制台已启动")}})
}

func (s *Server) logsStream(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	payload, _ := json.Marshal(s.logEntry("INFO", "SimRelay 日志流已连接"))
	_, _ = w.Write([]byte("event: log\n"))
	_, _ = w.Write([]byte("data: " + string(payload) + "\n\n"))
}

func (s *Server) settings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"version":     "simrelay",
		"build_time":  time.Now().Format(time.RFC3339),
		"config_path": "environment",
		"notify": map[string]any{
			"telegram": map[string]any{"enabled": false},
			"feishu":   map[string]any{"enabled": false},
			"qq":       map[string]any{"enabled": false},
			"bark":     map[string]any{"enabled": false},
			"email":    map[string]any{"enabled": false},
			"pushplus": map[string]any{"enabled": false},
			"webhook":  map[string]any{"enabled": false},
		},
	})
}

func (s *Server) saveSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": "SimRelay 当前仅支持环境变量配置"})
}

func (s *Server) proxyList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"proxies": []any{}, "items": []any{}})
}

func (s *Server) logEntry(level string, message string) map[string]any {
	return map[string]any{
		"ts":      time.Now().Format("2006-01-02 15:04:05"),
		"level":   level,
		"source":  "simrelay/httpapi",
		"message": message,
	}
}

func (s *Server) acceptedAction(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) unsupportedAction(w http.ResponseWriter, _ *http.Request) {
	writeVoHiveError(w, http.StatusNotImplemented, "SimRelay 单设备短信版本暂不支持此操作")
}

func (s *Server) deviceFromStatus(status modem.DeviceStatus) map[string]any {
	deviceID := "ec20"
	if status.IMEI != "" {
		deviceID = "ec20-" + tail(status.IMEI, 6)
	}
	dbm := rssiToDBM(status.SignalRSSI)
	return map[string]any{
		"id":                       deviceID,
		"name":                     firstNonEmpty(status.Model, "EC20"),
		"running":                  true,
		"healthy":                  status.SIM == "READY",
		"control_online":           true,
		"physical_present":         true,
		"worker_running":           true,
		"data_connected":           false,
		"radio_registered":         status.Registered,
		"lifecycle_phase":          "online",
		"lifecycle_reason":         "control_online",
		"public_ip":                "",
		"interface":                os.Getenv("SIMRELAY_DEVICE"),
		"usb_path":                 "",
		"control_device":           "",
		"apn":                      "",
		"esim_transport":           "at",
		"sms_enabled":              true,
		"network_enabled":          false,
		"vowifi_enabled":           false,
		"network_connected":        false,
		"registration_state_label": registrationLabel(status.Registered),
		"backend_mode":             "at",
		"at_port":                  os.Getenv("SIMRELAY_DEVICE"),
		"modem": map[string]any{
			"operator":        "",
			"native_spn":      "",
			"network_mode":    "LTE",
			"network_duplex":  "",
			"radio_band":      "",
			"radio_channel":   0,
			"signal_dbm":      dbm,
			"signal_sinr":     0,
			"imei":            status.IMEI,
			"iccid":           "",
			"imsi":            "",
			"local_phone":     "",
			"home_operator":   "",
			"firmware":        "",
			"reg_status":      boolToRegStatus(status.Registered),
			"reg_status_text": registrationLabel(status.Registered),
			"ps_attached":     status.Registered,
			"manufacturer":    status.Manufacturer,
			"model":           status.Model,
			"sim":             status.SIM,
			"signal_rssi":     status.SignalRSSI,
			"signal_ber":      status.SignalBER,
		},
	}
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	http.ServeFileFS(w, r, web.Static, "static/index.html")
}

func staticFS() fs.FS {
	static, err := fs.Sub(web.Static, "static")
	if err != nil {
		panic(err)
	}
	return static
}

func statusForError(err error) int {
	if errors.Is(err, at.ErrTimeout) {
		return http.StatusGatewayTimeout
	}
	return http.StatusBadGateway
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeVoHiveError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"message": message, "status": "error"})
}

func envString(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

type vohiveSMSMessage struct {
	ID        int    `json:"id"`
	DeviceID  string `json:"device_id"`
	IMSI      string `json:"imsi"`
	Peer      string `json:"peer"`
	Local     string `json:"local_phone"`
	Type      int    `json:"type"`
	Status    string `json:"status"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

func summarizeContacts(messages []modem.Message, status modem.DeviceStatus) []map[string]any {
	deviceID := "ec20"
	if status.IMEI != "" {
		deviceID = "ec20-" + tail(status.IMEI, 6)
	}
	contacts := make(map[string]map[string]any)
	for _, message := range messages {
		peer := message.From
		if peer == "" {
			peer = "未知号码"
		}
		current, ok := contacts[peer]
		if !ok || message.Index >= current["last_sms_id"].(int) {
			contacts[peer] = map[string]any{
				"imsi":           "",
				"peer":           peer,
				"last_sms_id":    message.Index,
				"last_timestamp": message.Timestamp,
				"last_content":   message.Text,
				"last_type":      messageType(message.Status),
				"unread_count":   0,
				"created_at":     message.Timestamp,
				"updated_at":     message.Timestamp,
				"device_id":      deviceID,
				"device_name":    firstNonEmpty(status.Model, "EC20"),
				"local_phone":    "",
			}
		}
		if strings.Contains(message.Status, "UNREAD") {
			contacts[peer]["unread_count"] = contacts[peer]["unread_count"].(int) + 1
		}
	}
	result := make([]map[string]any, 0, len(contacts))
	for _, contact := range contacts {
		result = append(result, contact)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i]["last_sms_id"].(int) > result[j]["last_sms_id"].(int)
	})
	return result
}

func vohiveMessage(message modem.Message) vohiveSMSMessage {
	return vohiveSMSMessage{
		ID:        message.Index,
		DeviceID:  "ec20",
		IMSI:      "",
		Peer:      message.From,
		Type:      messageType(message.Status),
		Status:    message.Status,
		Content:   message.Text,
		Timestamp: message.Timestamp,
	}
}

func messageType(status string) int {
	if strings.Contains(status, "STO") || strings.Contains(status, "SENT") {
		return 2
	}
	return 1
}

func rssiToDBM(rssi int) int {
	if rssi <= 0 || rssi == 99 {
		return -113
	}
	if rssi >= 31 {
		return -51
	}
	return -113 + 2*rssi
}

func boolToRegStatus(registered bool) int {
	if registered {
		return 1
	}
	return 0
}

func registrationLabel(registered bool) string {
	if registered {
		return "registered"
	}
	return "not registered"
}

func tail(value string, length int) string {
	if len(value) <= length {
		return value
	}
	return value[len(value)-length:]
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
