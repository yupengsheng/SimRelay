package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/yupengsheng/SimRelay/internal/at"
	"github.com/yupengsheng/SimRelay/internal/modem"
)

type Modem interface {
	DeviceStatus() (modem.DeviceStatus, error)
	ListMessages(box modem.MessageBox) ([]modem.Message, error)
	ReadMessage(index int) (modem.Message, error)
	SendMessage(to string, text string) (modem.SendResult, error)
}

type Server struct {
	modem Modem
	mux   *http.ServeMux
}

func New(modem Modem) http.Handler {
	server := &Server{
		modem: modem,
		mux:   http.NewServeMux(),
	}
	server.routes()
	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("GET /api/v1/device", s.device)
	s.mux.HandleFunc("GET /api/v1/messages", s.listMessages)
	s.mux.HandleFunc("GET /api/v1/messages/", s.readMessage)
	s.mux.HandleFunc("POST /api/v1/messages", s.sendMessage)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
