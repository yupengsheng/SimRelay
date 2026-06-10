package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yupengsheng/SimRelay/internal/modem"
)

type fakeModem struct {
	status   modem.DeviceStatus
	messages []modem.Message
	sent     modem.SendResult
	err      error
}

func (f *fakeModem) DeviceStatus() (modem.DeviceStatus, error) {
	return f.status, f.err
}

func (f *fakeModem) ListMessages(box modem.MessageBox) ([]modem.Message, error) {
	return f.messages, f.err
}

func (f *fakeModem) ReadMessage(index int) (modem.Message, error) {
	if f.err != nil {
		return modem.Message{}, f.err
	}
	return modem.Message{Index: index, From: "+8613800000000", Text: "中文测试"}, nil
}

func (f *fakeModem) SendMessage(to string, text string) (modem.SendResult, error) {
	return f.sent, f.err
}

func (f *fakeModem) DeleteMessage(index int) error {
	return f.err
}

func (f *fakeModem) RawCommand(command string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []string{"OK"}, nil
}

func (f *fakeModem) SendUSSD(code string) (modem.USSDResult, error) {
	if f.err != nil {
		return modem.USSDResult{}, f.err
	}
	return modem.USSDResult{Status: "0", Text: "余额 10 元"}, nil
}

func TestHealthz(t *testing.T) {
	server := New(&fakeModem{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestServesWebUI(t *testing.T) {
	server := New(&fakeModem{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if contentType := rec.Header().Get("Content-Type"); !bytes.Contains([]byte(contentType), []byte("text/html")) {
		t.Fatalf("content type = %q", contentType)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("SimRelay")) {
		t.Fatalf("body does not contain app title: %s", rec.Body.String())
	}
}

func TestUnknownAPIRouteIsNotWebFallback(t *testing.T) {
	server := New(&fakeModem{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/missing", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestDeviceStatus(t *testing.T) {
	server := New(&fakeModem{status: modem.DeviceStatus{Manufacturer: "Quectel", Model: "EC20", SIM: "READY"}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/device", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body modem.DeviceStatus
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Model != "EC20" || body.SIM != "READY" {
		t.Fatalf("body = %#v", body)
	}
}

func TestVoHiveLogin(t *testing.T) {
	t.Setenv("SIMRELAY_ADMIN_USERNAME", "admin")
	t.Setenv("SIMRELAY_ADMIN_PASSWORD", "secret")
	server := New(&fakeModem{})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"username":"admin","password":"secret"}`))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"token"`)) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestVoHiveDevices(t *testing.T) {
	server := New(&fakeModem{status: modem.DeviceStatus{Manufacturer: "Quectel", Model: "EC20F", IMEI: "865860049674642", SIM: "READY", SignalRSSI: 30, Registered: true}})
	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"devices"`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`EC20F`)) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestVoHiveDashboardAndDeviceAuxiliaryEndpoints(t *testing.T) {
	t.Setenv("SIMRELAY_DEVICE", "/dev/ttyUSB2")
	server := New(&fakeModem{status: modem.DeviceStatus{Manufacturer: "Quectel", Model: "EC20F", IMEI: "865860049674642", SIM: "READY", SignalRSSI: 30, Registered: true}})
	tests := []struct {
		path string
		want string
	}{
		{"/api/dashboard/devices", `"devices"`},
		{"/api/devices/discovered", `"/dev/ttyUSB2"`},
		{"/api/devices/ec20/config", `"config"`},
		{"/api/devices/ec20/esim", `"profiles"`},
		{"/api/devices/ec20/overview/stream", `"devices"`},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tt.path, rec.Code)
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(tt.want)) {
			t.Fatalf("%s body = %s", tt.path, rec.Body.String())
		}
	}
}

func TestVoHiveConsoleSupportEndpoints(t *testing.T) {
	server := New(&fakeModem{})
	tests := []struct {
		path string
		want string
	}{
		{"/api/traffic/analysis?range=day", `"buckets"`},
		{"/api/logs", `"logs"`},
		{"/api/settings", `"notify"`},
		{"/api/proxies", `"proxies"`},
		{"/api/proxy", `"items"`},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tt.path, rec.Code)
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(tt.want)) {
			t.Fatalf("%s body = %s", tt.path, rec.Body.String())
		}
	}
}

func TestListMessages(t *testing.T) {
	server := New(&fakeModem{messages: []modem.Message{{Index: 1, From: "+8613800000000", Text: "中文测试"}}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?box=all", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body []modem.Message
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body) != 1 || body[0].Text != "中文测试" {
		t.Fatalf("body = %#v", body)
	}
}

func TestVoHiveSMSContactsAndThread(t *testing.T) {
	fake := &fakeModem{
		status: modem.DeviceStatus{Model: "EC20F", IMEI: "865860049674642"},
		messages: []modem.Message{
			{Index: 1, Status: "REC READ", From: "+8613800000000", Text: "中文测试", Timestamp: "26/06/09,15:30:01+32"},
		},
	}
	server := New(fake)

	for _, path := range []string{"/api/sms/contacts?limit=20", "/api/sms/thread?peer=%2B8613800000000&limit=20"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, rec.Code)
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte("中文测试")) {
			t.Fatalf("%s body = %s", path, rec.Body.String())
		}
	}
}

func TestReadMessage(t *testing.T) {
	server := New(&fakeModem{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/3", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body modem.Message
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Index != 3 || body.Text != "中文测试" {
		t.Fatalf("body = %#v", body)
	}
}

func TestSendMessageValidatesRequest(t *testing.T) {
	server := New(&fakeModem{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewBufferString(`{"to":"","text":""}`))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("号码和短信内容不能为空")) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestSendMessage(t *testing.T) {
	server := New(&fakeModem{sent: modem.SendResult{Reference: 42}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewBufferString(`{"to":"+8613800000000","text":"中文测试"}`))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body modem.SendResult
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Reference != 42 {
		t.Fatalf("body = %#v", body)
	}
}

func TestVoHiveSendATAndUSSD(t *testing.T) {
	server := New(&fakeModem{})
	tests := []struct {
		path string
		body string
		want string
	}{
		{"/api/devices/ec20/actions/at", `{"command":"AT"}`, `"response"`},
		{"/api/devices/ec20/actions/ussd", `{"code":"*100#"}`, "余额"},
	}
	for _, tt := range tests {
		req := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBufferString(tt.body))
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tt.path, rec.Code)
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(tt.want)) {
			t.Fatalf("%s body = %s", tt.path, rec.Body.String())
		}
	}
}

func TestModemErrorMapsToBadGateway(t *testing.T) {
	server := New(&fakeModem{err: errors.New("modem failed")})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/device", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
}
