package service

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"dodevops-api/api/cmdb/model"
)

func TestBuildSNMPMonitorMetricsFromPrometheusIfMIB(t *testing.T) {
	device := &model.CmdbSnmpDevice{
		ID:          7,
		DeviceName:  "switch-1",
		DeviceType:  "switch",
		IPAddress:   "192.168.1.253",
		DeviceModel: "Nexus9000",
	}
	payload := map[string]interface{}{
		"rawText": `
# HELP ifHCInOctets The total number of octets received on the interface.
ifHCInOctets{ifIndex="1",ifName="Gi1/0/1",ifDescr="GigabitEthernet1/0/1"} 125000000
ifHCInOctets{ifIndex="2",ifName="Gi1/0/2",ifDescr="GigabitEthernet1/0/2"} 500000000
ifHCOutOctets{ifIndex="1",ifName="Gi1/0/1",ifDescr="GigabitEthernet1/0/1"} 225000000
ifHCOutOctets{ifIndex="2",ifName="Gi1/0/2",ifDescr="GigabitEthernet1/0/2"} 100000000
ifHighSpeed{ifIndex="1",ifName="Gi1/0/1"} 1000
ifHighSpeed{ifIndex="2",ifName="Gi1/0/2"} 1000
ifOperStatus{ifIndex="1",ifName="Gi1/0/1"} 1
ifOperStatus{ifIndex="2",ifName="Gi1/0/2"} 2
sysUpTime 8640000
`,
	}

	metrics := buildSNMPMonitorMetrics(device, payload, "monitor")

	if !metrics.MetricsAvailable {
		t.Fatal("metrics should be available")
	}
	if metrics.Status != "online" {
		t.Fatalf("status = %q, want online", metrics.Status)
	}
	if metrics.TotalPortCount != 2 {
		t.Fatalf("TotalPortCount = %d, want 2", metrics.TotalPortCount)
	}
	if metrics.IdlePortCount != 1 {
		t.Fatalf("IdlePortCount = %d, want 1", metrics.IdlePortCount)
	}
	if metrics.UptimeDays != 1 {
		t.Fatalf("UptimeDays = %v, want 1", metrics.UptimeDays)
	}
	if len(metrics.PortInTop5) != 2 {
		t.Fatalf("PortInTop5 length = %d, want 2", len(metrics.PortInTop5))
	}
	if metrics.PortInTop5[0].PortName != "Gi1/0/2" {
		t.Fatalf("top inbound port = %q, want Gi1/0/2", metrics.PortInTop5[0].PortName)
	}
	if metrics.PortInTop5[0].TrafficBytes != 500000000 {
		t.Fatalf("top inbound bytes = %v, want 500000000", metrics.PortInTop5[0].TrafficBytes)
	}
	if metrics.PortInTop5[0].TrafficRateMbps != 0 {
		t.Fatalf("top inbound rate = %v, want 0 when no rate metric is exposed", metrics.PortInTop5[0].TrafficRateMbps)
	}
	if metrics.PortInTop5[0].Utilization != 0 {
		t.Fatalf("top inbound utilization = %v, want 0 when no rate metric is exposed", metrics.PortInTop5[0].Utilization)
	}
	if len(metrics.PortOutTop5) != 2 {
		t.Fatalf("PortOutTop5 length = %d, want 2", len(metrics.PortOutTop5))
	}
	if metrics.PortOutTop5[0].PortName != "Gi1/0/1" {
		t.Fatalf("top outbound port = %q, want Gi1/0/1", metrics.PortOutTop5[0].PortName)
	}
}

func TestBuildSNMPResourceMetricsFromPrometheusVendorMetrics(t *testing.T) {
	device := &model.CmdbSnmpDevice{ID: 8, DeviceName: "switch-2", DeviceType: "switch"}
	payload := map[string]interface{}{
		"rawText": `
systemCpuUsage 37.5
systemMemUsage 62.25
temperatureReading{temperatureIndex="1"} 39
temperatureReading{temperatureIndex="2"} 41
`,
	}

	metrics := buildSNMPResourceMetrics(device, payload, "resource")

	if metrics.CpuUsage != 37.5 {
		t.Fatalf("CpuUsage = %v, want 37.5", metrics.CpuUsage)
	}
	if metrics.MemoryUsage != 62.25 {
		t.Fatalf("MemoryUsage = %v, want 62.25", metrics.MemoryUsage)
	}
	if !metrics.MetricsAvailable {
		t.Fatal("metrics should be available")
	}
}

func TestBuildSNMPMonitorMetricsUsesNestedRawTextAndIgnoresZeroTimestamp(t *testing.T) {
	device := &model.CmdbSnmpDevice{
		ID:         9,
		DeviceName: "switch-3",
		DeviceType: "switch",
		IPAddress:  "192.168.1.254",
	}
	payload := map[string]interface{}{
		"lastUpdateTime": "0001-01-01 00:00:00 +0000 UTC",
		"portInTop5": []interface{}{
			map[string]interface{}{
				"portIndex":       99,
				"portName":        "broken",
				"trafficRateMbps": 33392.28712,
				"trafficBytes":    4174035890.0,
				"utilization":     0.0,
				"portSpeed":       1000.0,
			},
		},
		"monitorPayload": map[string]interface{}{
			"rawText": `
# HELP ifHCInOctets The total number of octets received on the interface.
ifHCInOctets{ifIndex="1",ifName="Gi1/0/1"} 125000000
ifHCInOctets{ifIndex="2",ifName="Gi1/0/2"} 500000000
ifHCOutOctets{ifIndex="1",ifName="Gi1/0/1"} 225000000
ifHCOutOctets{ifIndex="2",ifName="Gi1/0/2"} 100000000
ifHighSpeed{ifIndex="1",ifName="Gi1/0/1"} 1000
ifHighSpeed{ifIndex="2",ifName="Gi1/0/2"} 1000
ifOperStatus{ifIndex="1",ifName="Gi1/0/1"} 1
ifOperStatus{ifIndex="2",ifName="Gi1/0/2"} 2
sysUpTime 8640000
`,
		},
	}

	metrics := buildSNMPMonitorMetrics(device, payload, "monitor")

	if len(metrics.PortInTop5) != 2 {
		t.Fatalf("PortInTop5 length = %d, want 2", len(metrics.PortInTop5))
	}
	if metrics.PortInTop5[0].PortName != "Gi1/0/2" {
		t.Fatalf("top inbound port = %q, want Gi1/0/2", metrics.PortInTop5[0].PortName)
	}
	if metrics.PortInTop5[0].TrafficBytes != 500000000 {
		t.Fatalf("top inbound bytes = %v, want 500000000", metrics.PortInTop5[0].TrafficBytes)
	}
	if metrics.PortInTop5[0].TrafficRateMbps != 0 {
		t.Fatalf("top inbound rate = %v, want 0 after nested rawText reparse", metrics.PortInTop5[0].TrafficRateMbps)
	}
	if strings.HasPrefix(metrics.LastUpdateTime, "0001-01-01") {
		t.Fatalf("LastUpdateTime = %q, should not keep zero timestamp", metrics.LastUpdateTime)
	}
}

func TestBuildSNMPMonitorMetricsFromCiscoVendorMetrics(t *testing.T) {
	device := &model.CmdbSnmpDevice{
		ID:           10,
		DeviceName:   "switch-4",
		DeviceType:   "switch",
		DeviceVendor: "Cisco",
		DeviceModel:  "Nexus9000",
	}
	payload := map[string]interface{}{
		"rawText": `
cpmCPUTotal5minRev{index="1"} 21
ciscoMemoryPoolUsed{ciscoMemoryPoolName="Processor"} 200
ciscoMemoryPoolFree{ciscoMemoryPoolName="Processor"} 300
ciscoEnvMonTemperatureStatusValue{index="1"} 42
`,
	}

	metrics := buildSNMPMonitorMetrics(device, payload, "monitor")

	if metrics.CpuUsage != 21 {
		t.Fatalf("CpuUsage = %v, want 21", metrics.CpuUsage)
	}
	if metrics.MemoryUsage != 40 {
		t.Fatalf("MemoryUsage = %v, want 40", metrics.MemoryUsage)
	}
	if metrics.Temperature != 42 {
		t.Fatalf("Temperature = %v, want 42", metrics.Temperature)
	}
}

func TestBuildSNMPRequestPayloadUsesVendorModule(t *testing.T) {
	device := &model.CmdbSnmpDevice{
		ID:           11,
		DeviceName:   "switch-5",
		DeviceType:   "switch",
		DeviceVendor: "Cisco",
		DeviceModel:  "Catalyst 9300",
		SnmpURL:      "http://127.0.0.1:9116/snmp?module=if_mib&target=192.168.1.10",
	}

	payload := buildSNMPRequestPayload(device, "monitor")
	if payload["module"] != "cisco_switch" {
		t.Fatalf("module = %v, want cisco_switch", payload["module"])
	}

	requestURL, err := buildSNMPRequestURL(device.SnmpURL, payload)
	if err != nil {
		t.Fatalf("buildSNMPRequestURL error = %v", err)
	}

	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		t.Fatalf("url.Parse error = %v", err)
	}
	if parsedURL.Query().Get("module") != "cisco_switch" {
		t.Fatalf("query module = %q, want cisco_switch", parsedURL.Query().Get("module"))
	}
}

func TestSnmpResolveRequestModulesFallsBackToOriginalModule(t *testing.T) {
	device := &model.CmdbSnmpDevice{
		DeviceVendor: "Cisco",
		DeviceModel:  "Nexus9000",
		SnmpURL:      "http://192.168.34.107:9116/snmp?module=if_mib&target=192.168.1.253",
	}

	modules := snmpResolveRequestModules(device)
	if len(modules) < 2 {
		t.Fatalf("module candidates = %v, want vendor module + fallback module", modules)
	}
	if modules[0] != "cisco_switch" {
		t.Fatalf("first module = %q, want cisco_switch", modules[0])
	}
	if modules[1] != "if_mib" {
		t.Fatalf("second module = %q, want if_mib", modules[1])
	}
}

func TestBuildSNMPEndpointCandidatesKeepsExporterSnmpPath(t *testing.T) {
	candidates := buildSNMPEndpointCandidates("http://192.168.34.107:9116/snmp?module=if_mib&target=192.168.1.253", "monitor")

	if len(candidates) != 1 {
		t.Fatalf("candidate count = %d, want 1", len(candidates))
	}
	if strings.Contains(candidates[0], "/snmp/monitor") {
		t.Fatalf("candidate = %q, exporter /snmp path should not append /monitor", candidates[0])
	}
}

func TestSnmpLooksLikeHTML(t *testing.T) {
	if !snmpLooksLikeHTML([]byte("<html><body>SNMP Exporter</body></html>")) {
		t.Fatal("html response should be detected")
	}
	if snmpLooksLikeHTML([]byte("ifHCInOctets{ifIndex=\"1\"} 100")) {
		t.Fatal("prometheus metrics should not be detected as html")
	}
}

func TestSnmpShouldRetryWithNextModule(t *testing.T) {
	if !snmpShouldRetryWithNextModule(assertError("external snmp api returned status 400: Unknown module 'cisco_switch'")) {
		t.Fatal("unknown module error should trigger retry")
	}
	if snmpShouldRetryWithNextModule(assertError("external snmp api returned status 401: unauthorized")) {
		t.Fatal("non-module error should not trigger retry")
	}
}

func TestShouldFetchSNMPLiveMetrics(t *testing.T) {
	tests := []struct {
		name   string
		device *model.CmdbSnmpDevice
		want   bool
	}{
		{
			name: "online device",
			device: &model.CmdbSnmpDevice{
				Status:         "online",
				MonitorEnabled: 1,
				SnmpURL:        "http://127.0.0.1:9116/snmp?module=if_mib&target=192.168.1.10",
			},
			want: true,
		},
		{
			name: "offline device with snmp url",
			device: &model.CmdbSnmpDevice{
				Status:         "offline",
				MonitorEnabled: 1,
				SnmpURL:        "http://127.0.0.1:9116/snmp?module=if_mib&target=192.168.1.10",
			},
			want: true,
		},
		{
			name: "offline device without snmp url",
			device: &model.CmdbSnmpDevice{
				Status:         "offline",
				MonitorEnabled: 1,
			},
			want: false,
		},
		{
			name: "monitor disabled",
			device: &model.CmdbSnmpDevice{
				Status:         "online",
				MonitorEnabled: 0,
			},
			want: false,
		},
		{
			name:   "nil device",
			device: nil,
			want:   false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if got := shouldFetchSNMPLiveMetrics(testCase.device); got != testCase.want {
				t.Fatalf("shouldFetchSNMPLiveMetrics() = %v, want %v", got, testCase.want)
			}
		})
	}
}

func TestGetDeviceMonitorMetricsFetchesOfflineDeviceWithSnmpURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/snmp" {
			t.Fatalf("request path = %q, want /snmp", r.URL.Path)
		}
		if r.URL.Query().Get("target") != "192.168.1.10" {
			t.Fatalf("target = %q, want 192.168.1.10", r.URL.Query().Get("target"))
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(`
ifHCInOctets{ifIndex="1",ifName="Gi1/0/1"} 125000000
ifHCOutOctets{ifIndex="1",ifName="Gi1/0/1"} 225000000
ifOperStatus{ifIndex="1",ifName="Gi1/0/1"} 1
sysUpTime 8640000
`))
	}))
	defer server.Close()

	device := &model.CmdbSnmpDevice{
		ID:             8,
		DeviceName:     "核心交换机n9k-2",
		DeviceType:     "switch",
		DeviceVendor:   "Cisco",
		DeviceModel:    "Nexus9000",
		IPAddress:      "192.168.1.10",
		Status:         "offline",
		MonitorEnabled: 1,
		SnmpURL:        server.URL + "/snmp?module=if_mib&target=192.168.1.10",
	}

	service := &SnmpMonitorService{client: server.Client()}
	metrics, err := service.getDeviceMonitorMetricsForDevice(device)
	if err != nil {
		t.Fatalf("getDeviceMonitorMetricsForDevice error = %v", err)
	}

	if !metrics.MetricsAvailable {
		t.Fatal("metrics should be available")
	}
	if metrics.Status != "online" {
		t.Fatalf("status = %q, want online", metrics.Status)
	}
	if metrics.MetricsSource == "" {
		t.Fatal("metrics source should be set after fetching exporter data")
	}
	if metrics.TotalPortCount != 1 {
		t.Fatalf("TotalPortCount = %d, want 1", metrics.TotalPortCount)
	}
}

func TestGetBatchDeviceResourceMetricsSkipsOfflineLiveFetch(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("systemCpuUsage 37"))
	}))
	defer server.Close()

	device := &model.CmdbSnmpDevice{
		ID:             8,
		DeviceName:     "核心交换机n9k-2",
		DeviceType:     "switch",
		DeviceVendor:   "Cisco",
		DeviceModel:    "Nexus9000",
		IPAddress:      "192.168.1.10",
		Status:         "offline",
		MonitorEnabled: 1,
		SnmpURL:        server.URL + "/snmp?module=if_mib&target=192.168.1.10",
	}

	service := &SnmpMonitorService{client: server.Client()}
	metrics, err := service.getBatchDeviceResourceMetricsForDevice(device)
	if err != nil {
		t.Fatalf("getBatchDeviceResourceMetricsForDevice error = %v", err)
	}

	if called {
		t.Fatal("batch resource should not live fetch offline devices")
	}
	if metrics.MetricsAvailable {
		t.Fatal("offline batch fallback should not report live metrics")
	}
}

func TestUniqueSNMPDeviceIDsPreservesInputOrder(t *testing.T) {
	got := uniqueSNMPDeviceIDs([]uint{3, 1, 3, 2, 1, 4})
	want := []uint{3, 1, 2, 4}

	if len(got) != len(want) {
		t.Fatalf("len(uniqueSNMPDeviceIDs()) = %d, want %d", len(got), len(want))
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("uniqueSNMPDeviceIDs()[%d] = %d, want %d", index, got[index], want[index])
		}
	}
}

func assertError(text string) error {
	return &testError{text: text}
}

type testError struct {
	text string
}

func (e *testError) Error() string {
	return e.text
}
