package httpapi

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/yupengsheng/SimRelay/internal/modem"
	_ "modernc.org/sqlite"
)

type fakeModem struct {
	status         modem.DeviceStatus
	messages       []modem.Message
	sent           modem.SendResult
	err            error
	lastSentTo     string
	lastSentText   string
	deletedIndexes []int
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
	f.lastSentTo = to
	f.lastSentText = text
	return f.sent, f.err
}

func (f *fakeModem) DeleteMessage(index int) error {
	f.deletedIndexes = append(f.deletedIndexes, index)
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
	server := New(&fakeModem{status: modem.DeviceStatus{
		Manufacturer:  "Quectel",
		Model:         "EC20F",
		IMEI:          "865860049674642",
		ICCID:         "8944110069316673105",
		IMSI:          "234102156572007",
		SIM:           "READY",
		Operator:      "中国移动",
		NativeSPN:     "giffgaff",
		NetworkMode:   "LTE",
		NetworkDuplex: "FDD",
		RadioBand:     "LTE BAND 122",
		RadioChannel:  1300,
		SignalRSSI:    30,
		SignalDBM:     -50,
		SignalSINR:    16,
		Registered:    true,
		PSAttached:    true,
		BackendMode:   "qmi",
		QMIAvailable:  true,
	}})
	req := httptest.NewRequest(http.MethodGet, "/api/devices", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"devices"`)) ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`EC20F`)) ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`中国移动`)) ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`giffgaff`)) ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`LTE BAND 122`)) ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`8944110069316673105`)) {
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
	fake := &fakeModem{sent: modem.SendResult{Reference: 42}}
	server := New(fake)
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
	if fake.lastSentTo != "+8613800000000" || fake.lastSentText != "中文测试" {
		t.Fatalf("sent = %q %q", fake.lastSentTo, fake.lastSentText)
	}
}

func TestSentSMSAppearsInContactsAndThread(t *testing.T) {
	fake := &fakeModem{
		status: modem.DeviceStatus{Model: "EC20F", IMEI: "865860049674642"},
		sent:   modem.SendResult{Reference: 42},
	}
	server := New(fake)

	sendReq := httptest.NewRequest(http.MethodPost, "/api/sms/send", bytes.NewBufferString(`{"phone":"+8613800000000","message":"刚刚发送的短信"}`))
	sendRec := httptest.NewRecorder()
	server.ServeHTTP(sendRec, sendReq)
	if sendRec.Code != http.StatusOK {
		t.Fatalf("send status = %d, body = %s", sendRec.Code, sendRec.Body.String())
	}

	contactReq := httptest.NewRequest(http.MethodGet, "/api/sms/contacts?limit=20", nil)
	contactRec := httptest.NewRecorder()
	server.ServeHTTP(contactRec, contactReq)
	if contactRec.Code != http.StatusOK {
		t.Fatalf("contacts status = %d", contactRec.Code)
	}
	if !bytes.Contains(contactRec.Body.Bytes(), []byte("刚刚发送的短信")) || !bytes.Contains(contactRec.Body.Bytes(), []byte(`"last_type":2`)) {
		t.Fatalf("contacts body = %s", contactRec.Body.String())
	}

	threadReq := httptest.NewRequest(http.MethodGet, "/api/sms/thread?peer=%2B8613800000000&limit=20", nil)
	threadRec := httptest.NewRecorder()
	server.ServeHTTP(threadRec, threadReq)
	if threadRec.Code != http.StatusOK {
		t.Fatalf("thread status = %d", threadRec.Code)
	}
	var thread []vohiveSMSMessage
	if err := json.NewDecoder(threadRec.Body).Decode(&thread); err != nil {
		t.Fatalf("decode thread: %v", err)
	}
	if len(thread) != 1 {
		t.Fatalf("thread = %#v", thread)
	}
	if thread[0].ID >= 0 || thread[0].Peer != "+8613800000000" || thread[0].Type != 2 || thread[0].Content != "刚刚发送的短信" {
		t.Fatalf("thread = %#v", thread)
	}
}

func TestSentSMSCanBeListedAndDeletedLocally(t *testing.T) {
	fake := &fakeModem{sent: modem.SendResult{Reference: 7}}
	server := New(fake)

	sendReq := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewBufferString(`{"to":"+8613800000000","text":"本地已发送"}`))
	sendRec := httptest.NewRecorder()
	server.ServeHTTP(sendRec, sendReq)
	if sendRec.Code != http.StatusOK {
		t.Fatalf("send status = %d", sendRec.Code)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/messages?box=sent", nil)
	listRec := httptest.NewRecorder()
	server.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d", listRec.Code)
	}
	var messages []modem.Message
	if err := json.NewDecoder(listRec.Body).Decode(&messages); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(messages) != 1 || messages[0].Index >= 0 || messages[0].Text != "本地已发送" {
		t.Fatalf("messages = %#v", messages)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/sms/messages/"+strconv.Itoa(messages[0].Index), nil)
	deleteRec := httptest.NewRecorder()
	server.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", deleteRec.Code, deleteRec.Body.String())
	}
	if len(fake.deletedIndexes) != 0 {
		t.Fatalf("unexpected modem deletes = %#v", fake.deletedIndexes)
	}

	threadReq := httptest.NewRequest(http.MethodGet, "/api/sms/thread?peer=%2B8613800000000", nil)
	threadRec := httptest.NewRecorder()
	server.ServeHTTP(threadRec, threadReq)
	if threadRec.Code != http.StatusOK {
		t.Fatalf("thread status = %d", threadRec.Code)
	}
	var thread []vohiveSMSMessage
	if err := json.NewDecoder(threadRec.Body).Decode(&thread); err != nil {
		t.Fatalf("decode thread: %v", err)
	}
	if len(thread) != 0 {
		t.Fatalf("thread = %#v", thread)
	}
}

func TestVoHiveSQLiteSMSBridge(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vohive.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	for _, statement := range []string{
		`create table devices (imei text primary key, alias text)`,
		`create table sim_cards (iccid text primary key, imsi text, current_imei text)`,
		`create table sim_subscriptions (imsi text primary key, current_iccid text, phone_number text)`,
		`create table sms_contacts (imsi text, peer text, last_sms_id integer, last_timestamp datetime, last_content text, last_type integer, unread_count integer, created_at datetime, updated_at datetime, primary key (imsi, peer))`,
		`create table sms (id integer primary key, imsi text, peer text, local_phone text, sender text, recipient text, content text, type integer, status integer, timestamp datetime, created_at datetime)`,
		`insert into devices (imei, alias) values ('865860049674642', 'wwp0s26f7u1i4')`,
		`insert into sim_cards (iccid, imsi, current_imei) values ('8944110069316673105', '234102156572007', '865860049674642')`,
		`insert into sim_subscriptions (imsi, current_iccid, phone_number) values ('234102156572007', '8944110069316673105', '07907807589')`,
		`insert into sms_contacts (imsi, peer, last_sms_id, last_timestamp, last_content, last_type, unread_count, created_at, updated_at) values ('234102156572007', 'giffgaff', 15, '2026-06-10 12:17:54+01:00', '926687 is your giffgaff verification code.', 1, 14, '2026-06-08 20:20:21.980580887+08:00', '2026-06-10 19:17:55.8272732+08:00')`,
		`insert into sms (id, imsi, peer, local_phone, sender, recipient, content, type, status, timestamp, created_at) values (15, '234102156572007', 'giffgaff', '', 'giffgaff', '', '926687 is your giffgaff verification code.', 1, 0, '2026-06-10 12:17:54+01:00', '2026-06-10 19:17:55.820937471+08:00')`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec sqlite statement %q: %v", statement, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close sqlite: %v", err)
	}

	t.Setenv("SIMRELAY_VOHIVE_DB", dbPath)
	server := New(&fakeModem{status: modem.DeviceStatus{Model: "EC20F", IMEI: "865860049674642"}})

	contactReq := httptest.NewRequest(http.MethodGet, "/api/sms/contacts?limit=20", nil)
	contactRec := httptest.NewRecorder()
	server.ServeHTTP(contactRec, contactReq)
	if contactRec.Code != http.StatusOK {
		t.Fatalf("contacts status = %d", contactRec.Code)
	}
	if !bytes.Contains(contactRec.Body.Bytes(), []byte("giffgaff")) || !bytes.Contains(contactRec.Body.Bytes(), []byte("wwp0s26f7u1i4")) {
		t.Fatalf("contacts body = %s", contactRec.Body.String())
	}
	var contacts []map[string]any
	if err := json.NewDecoder(bytes.NewReader(contactRec.Body.Bytes())).Decode(&contacts); err != nil {
		t.Fatalf("decode contacts: %v", err)
	}
	if len(contacts) == 0 || int(contacts[0]["unread_count"].(float64)) != 1 {
		t.Fatalf("contacts = %#v", contacts)
	}

	threadReq := httptest.NewRequest(http.MethodGet, "/api/sms/thread?peer=giffgaff&limit=20", nil)
	threadRec := httptest.NewRecorder()
	server.ServeHTTP(threadRec, threadReq)
	if threadRec.Code != http.StatusOK {
		t.Fatalf("thread status = %d", threadRec.Code)
	}
	var thread []vohiveSMSMessage
	if err := json.NewDecoder(threadRec.Body).Decode(&thread); err != nil {
		t.Fatalf("decode thread: %v", err)
	}
	if len(thread) != 1 || thread[0].Peer != "giffgaff" || thread[0].DeviceID != "wwp0s26f7u1i4" || thread[0].Content == "" {
		t.Fatalf("thread = %#v", thread)
	}

	contactAfterReadReq := httptest.NewRequest(http.MethodGet, "/api/sms/contacts?limit=20", nil)
	contactAfterReadRec := httptest.NewRecorder()
	server.ServeHTTP(contactAfterReadRec, contactAfterReadReq)
	if contactAfterReadRec.Code != http.StatusOK {
		t.Fatalf("contacts after read status = %d", contactAfterReadRec.Code)
	}
	var contactsAfterRead []map[string]any
	if err := json.NewDecoder(contactAfterReadRec.Body).Decode(&contactsAfterRead); err != nil {
		t.Fatalf("decode contacts after read: %v", err)
	}
	if len(contactsAfterRead) == 0 || int(contactsAfterRead[0]["unread_count"].(float64)) != 0 {
		t.Fatalf("contacts after read = %#v", contactsAfterRead)
	}

	writer, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("reopen sqlite: %v", err)
	}
	defer writer.Close()
	if _, err := writer.Exec(`insert into sms (id, imsi, peer, local_phone, sender, recipient, content, type, status, timestamp, created_at) values (16, '234102156572007', 'giffgaff', '', 'giffgaff', '', 'new verification code', 1, 0, '2026-06-10 12:47:32+01:00', '2026-06-10 19:47:34.376804687+08:00')`); err != nil {
		t.Fatalf("insert new sms: %v", err)
	}
	if _, err := writer.Exec(`update sms_contacts set last_sms_id = 16, last_timestamp = '2026-06-10 12:47:32+01:00', last_content = 'new verification code', unread_count = 2 where imsi = '234102156572007' and peer = 'giffgaff'`); err != nil {
		t.Fatalf("update contact: %v", err)
	}
	contactAfterNewReq := httptest.NewRequest(http.MethodGet, "/api/sms/contacts?limit=20", nil)
	contactAfterNewRec := httptest.NewRecorder()
	server.ServeHTTP(contactAfterNewRec, contactAfterNewReq)
	if contactAfterNewRec.Code != http.StatusOK {
		t.Fatalf("contacts after new status = %d", contactAfterNewRec.Code)
	}
	var contactsAfterNew []map[string]any
	if err := json.NewDecoder(contactAfterNewRec.Body).Decode(&contactsAfterNew); err != nil {
		t.Fatalf("decode contacts after new: %v", err)
	}
	if len(contactsAfterNew) == 0 || int(contactsAfterNew[0]["unread_count"].(float64)) != 1 {
		t.Fatalf("contacts after new = %#v", contactsAfterNew)
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
