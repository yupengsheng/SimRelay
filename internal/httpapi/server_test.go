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

func TestHealthz(t *testing.T) {
	server := New(&fakeModem{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
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

func TestModemErrorMapsToBadGateway(t *testing.T) {
	server := New(&fakeModem{err: errors.New("modem failed")})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/device", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
}
