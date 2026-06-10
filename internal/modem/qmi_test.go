package modem

import "testing"

func TestParseQMIOutputsEnrichesOperatorAndRadioFields(t *testing.T) {
	var status DeviceStatus

	parseQMIDMSICCID(`[ /dev/cdc-wdm0 ] UIM ICCID retrieved:
	ICCID: '8944110069316673105F'
`, &status)
	parseQMIDMSIMSI(`[ /dev/cdc-wdm0 ] UIM IMSI retrieved:
	IMSI: '234102156572007'
`, &status)
	parseQMINASHomeNetwork(`[ /dev/cdc-wdm0 ] Successfully got home network:
	Home network:
		MCC: '234'
		MNC: '10'
		Description: 'giffgaff'
`, &status)
	parseQMINASServingSystem(`[ /dev/cdc-wdm0 ] Successfully got serving system:
	Registration state: 'registered'
	PS: 'attached'
	Selected network: '3gpp'
	Radio interfaces: 'lte'
	Current PLMN:
		MCC: '460'
		MNC: '00'
		Description: 'China Mobile'
`, &status)
	parseQMINASSignalInfo(`[ /dev/cdc-wdm0 ] Successfully got signal info
	LTE:
		RSSI: '-50 dBm'
		RSRQ: '-7 dB'
		RSRP: '-77 dBm'
		SNR: '16.0 dB'
`, &status)
	parseQMINASRFBandInfo(`[ /dev/cdc-wdm0 ] Successfully got RF band info
	Band 0:
		Radio Interface: 'lte'
		Active Band Class: 'e-utra-operating-band-122'
		Active Channel: '1300'
`, &status)
	deriveSIMIdentity(&status)

	if status.Operator != "中国移动" {
		t.Fatalf("Operator = %q", status.Operator)
	}
	if status.NativeSPN != "giffgaff" || status.HomeOperator != "giffgaff" {
		t.Fatalf("native identity = %#v", status)
	}
	if status.NativeMCC != "234" || status.NativeMNC != "10" || status.ICCID != "8944110069316673105" || status.IMSI != "234102156572007" {
		t.Fatalf("sim identity = %#v", status)
	}
	if status.NetworkMode != "LTE" || status.RadioBand != "LTE BAND 122" || status.RadioChannel != 1300 {
		t.Fatalf("radio = %#v", status)
	}
	if status.SignalDBM != -50 || status.SignalSINR != 16 || status.SignalRSSI == 0 {
		t.Fatalf("signal = %#v", status)
	}
	if !status.Registered || !status.PSAttached {
		t.Fatalf("registration = %#v", status)
	}
}

func TestParseATEnrichmentResponses(t *testing.T) {
	status := DeviceStatus{}
	parseCOPS(`+COPS: 0,2,"46000",7`, &status)
	parseQCCID(`+QCCID: 8944110069316673105`, &status)
	parseCIMI(`234102156572007`, &status)
	parseQSPN(`+QSPN: "giffgaff",0`, &status)
	parseQNWINFO(`+QNWINFO: "FDD LTE","46000","LTE BAND 122",1300`, &status)
	deriveSIMIdentity(&status)

	if status.Operator != "中国移动" || status.NativeSPN != "giffgaff" {
		t.Fatalf("operator fields = %#v", status)
	}
	if status.NetworkMode != "LTE" || status.NetworkDuplex != "FDD" || status.RadioBand != "LTE BAND 122" || status.RadioChannel != 1300 {
		t.Fatalf("network fields = %#v", status)
	}
	if status.ICCID != "8944110069316673105" || status.IMSI != "234102156572007" {
		t.Fatalf("sim fields = %#v", status)
	}
}
