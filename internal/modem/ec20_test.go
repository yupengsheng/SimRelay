package modem

import (
	"reflect"
	"testing"
)

type fakeAT struct {
	commands []string
	smsCalls []smsCall
	replies  map[string][]string
}

type smsCall struct {
	command string
	payload string
}

func (f *fakeAT) Command(command string) ([]string, error) {
	f.commands = append(f.commands, command)
	if reply, ok := f.replies[command]; ok {
		return reply, nil
	}
	return []string{}, nil
}

func (f *fakeAT) CommandSMS(command string, payload string) ([]string, error) {
	f.smsCalls = append(f.smsCalls, smsCall{command: command, payload: payload})
	if reply, ok := f.replies[command]; ok {
		return reply, nil
	}
	return []string{}, nil
}

func TestInitUsesChineseCapableUCS2Mode(t *testing.T) {
	client := &fakeAT{replies: map[string][]string{}}
	m := NewEC20(client)

	if err := m.Init(); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	want := []string{
		"AT",
		"ATE0",
		"AT+CMGF=1",
		`AT+CSCS="UCS2"`,
		"AT+CSMP=17,167,0,8",
		`AT+CPMS="SM","SM","SM"`,
	}
	if !reflect.DeepEqual(client.commands, want) {
		t.Fatalf("commands = %#v, want %#v", client.commands, want)
	}
}

func TestUCS2EncodeDecodeChineseText(t *testing.T) {
	encoded := EncodeUCS2("中文测试")
	if encoded != "4E2D65876D4B8BD5" {
		t.Fatalf("encoded = %q", encoded)
	}

	decoded, err := DecodeUCS2(encoded)
	if err != nil {
		t.Fatalf("DecodeUCS2 returned error: %v", err)
	}
	if decoded != "中文测试" {
		t.Fatalf("decoded = %q", decoded)
	}
}

func TestDeviceStatusParsesEC20Responses(t *testing.T) {
	client := &fakeAT{replies: map[string][]string{
		"AT+CGMI":  {"Quectel"},
		"AT+CGMM":  {"EC20"},
		"AT+CGSN":  {"867698040000001"},
		"AT+CPIN?": {"+CPIN: READY"},
		"AT+CSQ":   {"+CSQ: 18,99"},
		"AT+CREG?": {"+CREG: 0,1"},
	}}
	m := NewEC20(client)

	status, err := m.DeviceStatus()
	if err != nil {
		t.Fatalf("DeviceStatus returned error: %v", err)
	}

	if status.Manufacturer != "Quectel" || status.Model != "EC20" || status.IMEI != "867698040000001" {
		t.Fatalf("unexpected status identity: %#v", status)
	}
	if status.SIM != "READY" || status.SignalRSSI != 18 || status.Registered != true {
		t.Fatalf("unexpected status values: %#v", status)
	}
}

func TestListMessagesParsesUCS2Messages(t *testing.T) {
	client := &fakeAT{replies: map[string][]string{
		"AT+CMGL=4": {
			`+CMGL: 1,"REC READ","002B0038003600310033003800300030003000300030003000300030",,"26/06/09,15:30:01+32"`,
			"4E2D65876D4B8BD5",
		},
	}}
	m := NewEC20(client)

	messages, err := m.ListMessages(BoxAll)
	if err != nil {
		t.Fatalf("ListMessages returned error: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	msg := messages[0]
	if msg.Index != 1 || msg.Status != "REC READ" || msg.From != "+8613800000000" || msg.Text != "中文测试" {
		t.Fatalf("message = %#v", msg)
	}
}

func TestReadMessageParsesCMGRResponse(t *testing.T) {
	client := &fakeAT{replies: map[string][]string{
		"AT+CMGR=7": {
			`+CMGR: "REC READ","002B0038003600310033003800300030003000300030003000300030",,"26/06/09,15:30:01+32"`,
			"4E2D65876D4B8BD5",
		},
	}}
	m := NewEC20(client)

	message, err := m.ReadMessage(7)
	if err != nil {
		t.Fatalf("ReadMessage returned error: %v", err)
	}

	if message.Index != 7 || message.Status != "REC READ" || message.From != "+8613800000000" || message.Text != "中文测试" {
		t.Fatalf("message = %#v", message)
	}
}

func TestSendMessageEncodesNumberAndTextAsUCS2(t *testing.T) {
	client := &fakeAT{replies: map[string][]string{
		`AT+CMGS="002B0038003600310033003800300030003000300030003000300030"`: {"+CMGS: 42"},
	}}
	m := NewEC20(client)

	result, err := m.SendMessage("+8613800000000", "中文测试")
	if err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}

	if result.Reference != 42 {
		t.Fatalf("reference = %d, want 42", result.Reference)
	}
	if len(client.smsCalls) != 1 {
		t.Fatalf("sms calls = %d, want 1", len(client.smsCalls))
	}
	call := client.smsCalls[0]
	if call.payload != "4E2D65876D4B8BD5" {
		t.Fatalf("payload = %q", call.payload)
	}
}
