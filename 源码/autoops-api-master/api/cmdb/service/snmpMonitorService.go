package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/model"
)

const (
	defaultSnmpMonitorTimeout      = 15 * time.Second
	batchSnmpMonitorMaxConcurrency = 16
)

type SnmpMonitorService struct {
	deviceDao *dao.CmdbSnmpDeviceDao
	client    *http.Client
}

func NewSnmpMonitorService() *SnmpMonitorService {
	return &SnmpMonitorService{
		deviceDao: dao.NewCmdbSnmpDeviceDao(),
		client: &http.Client{
			Timeout: defaultSnmpMonitorTimeout,
		},
	}
}

func (s *SnmpMonitorService) GetDeviceMonitorMetrics(deviceID uint) (*model.DeviceMonitorMetrics, error) {
	device, err := s.loadDevice(deviceID)
	if err != nil {
		return nil, err
	}

	return s.getDeviceMonitorMetricsForDevice(device)
}

func (s *SnmpMonitorService) getDeviceMonitorMetricsForDevice(device *model.CmdbSnmpDevice) (*model.DeviceMonitorMetrics, error) {
	if !shouldFetchSNMPLiveMetrics(device) {
		return buildSNMPMonitorMetrics(device, map[string]interface{}{}, ""), nil
	}

	payload, endpoint, err := s.fetchPayload(device, "monitor")
	if err != nil {
		return nil, err
	}

	return buildSNMPMonitorMetrics(device, payload, endpoint), nil
}

func (s *SnmpMonitorService) GetDeviceResourceMetrics(deviceID uint) (*model.DeviceResourceMetrics, error) {
	device, err := s.loadDevice(deviceID)
	if err != nil {
		return nil, err
	}

	return s.getDeviceResourceMetricsForDevice(device)
}

func (s *SnmpMonitorService) getDeviceResourceMetricsForDevice(device *model.CmdbSnmpDevice) (*model.DeviceResourceMetrics, error) {
	if !shouldFetchSNMPLiveMetrics(device) {
		return buildSNMPResourceMetrics(device, map[string]interface{}{}, ""), nil
	}

	payload, endpoint, err := s.fetchPayload(device, "resource")
	if err != nil {
		return nil, err
	}

	return buildSNMPResourceMetrics(device, payload, endpoint), nil
}

func (s *SnmpMonitorService) GetBatchDeviceResourceMetrics(deviceIDs []uint) (*model.BatchResourceMetricsResult, error) {
	if len(deviceIDs) == 0 {
		return &model.BatchResourceMetricsResult{
			Items: []model.DeviceResourceMetrics{},
			Total: 0,
		}, nil
	}

	deviceMap, err := s.loadDevices(deviceIDs)
	if err != nil {
		return nil, err
	}

	uniqueIDs := uniqueSNMPDeviceIDs(deviceIDs)
	resultsByID := make(map[uint]model.DeviceResourceMetrics, len(uniqueIDs))
	semaphore := make(chan struct{}, batchSnmpMonitorConcurrency(len(uniqueIDs)))
	var waitGroup sync.WaitGroup
	var resultLock sync.Mutex

	for _, deviceID := range uniqueIDs {
		device := deviceMap[deviceID]
		if device == nil {
			continue
		}

		waitGroup.Add(1)
		go func(id uint, device *model.CmdbSnmpDevice) {
			defer waitGroup.Done()
			semaphore <- struct{}{}
			defer func() {
				<-semaphore
			}()

			metrics, metricsErr := s.getBatchDeviceResourceMetricsForDevice(device)
			if metricsErr != nil || metrics == nil {
				metrics = buildSNMPResourceMetrics(device, map[string]interface{}{}, "")
			}

			resultLock.Lock()
			resultsByID[id] = *metrics
			resultLock.Unlock()
		}(deviceID, device)
	}

	waitGroup.Wait()

	items := make([]model.DeviceResourceMetrics, 0, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		item, exists := resultsByID[deviceID]
		if !exists || item.DeviceID == 0 {
			continue
		}
		items = append(items, item)
	}

	return &model.BatchResourceMetricsResult{
		Items: items,
		Total: len(items),
	}, nil
}

func (s *SnmpMonitorService) loadDevice(deviceID uint) (*model.CmdbSnmpDevice, error) {
	device, err := s.deviceDao.GetSnmpDeviceById(deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found")
	}
	return device, nil
}

func (s *SnmpMonitorService) loadDevices(deviceIDs []uint) (map[uint]*model.CmdbSnmpDevice, error) {
	devices, err := s.deviceDao.GetSnmpDevicesByIDs(uniqueSNMPDeviceIDs(deviceIDs))
	if err != nil {
		return nil, err
	}

	deviceMap := make(map[uint]*model.CmdbSnmpDevice, len(devices))
	for index := range devices {
		deviceMap[devices[index].ID] = &devices[index]
	}
	return deviceMap, nil
}

func uniqueSNMPDeviceIDs(deviceIDs []uint) []uint {
	unique := make([]uint, 0, len(deviceIDs))
	seen := make(map[uint]struct{}, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		if _, exists := seen[deviceID]; exists {
			continue
		}
		seen[deviceID] = struct{}{}
		unique = append(unique, deviceID)
	}
	return unique
}

func batchSnmpMonitorConcurrency(deviceCount int) int {
	switch {
	case deviceCount <= 1:
		return 1
	case deviceCount < batchSnmpMonitorMaxConcurrency:
		return deviceCount
	default:
		return batchSnmpMonitorMaxConcurrency
	}
}

func shouldFetchSNMPLiveMetrics(device *model.CmdbSnmpDevice) bool {
	if device == nil {
		return false
	}
	if device.MonitorEnabled == 0 {
		return false
	}
	return strings.TrimSpace(device.SnmpURL) != ""
}

func shouldFetchSNMPBatchLiveMetrics(device *model.CmdbSnmpDevice) bool {
	if !shouldFetchSNMPLiveMetrics(device) {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(device.Status), "offline")
}

func (s *SnmpMonitorService) getBatchDeviceResourceMetricsForDevice(device *model.CmdbSnmpDevice) (*model.DeviceResourceMetrics, error) {
	if !shouldFetchSNMPBatchLiveMetrics(device) {
		return buildSNMPResourceMetrics(device, map[string]interface{}{}, ""), nil
	}

	payload, endpoint, err := s.fetchPayload(device, "resource")
	if err != nil {
		return nil, err
	}

	return buildSNMPResourceMetrics(device, payload, endpoint), nil
}

func (s *SnmpMonitorService) fetchPayload(device *model.CmdbSnmpDevice, endpoint string) (map[string]interface{}, string, error) {
	if device == nil {
		return nil, "", fmt.Errorf("device not found")
	}
	if strings.TrimSpace(device.SnmpURL) == "" {
		return nil, "", fmt.Errorf("snmpUrl is required")
	}

	raw, resolvedEndpoint, err := s.requestExternal(device, http.MethodGet, endpoint)
	if err != nil && endpoint == "resource" {
		raw, resolvedEndpoint, err = s.requestExternal(device, http.MethodGet, "monitor")
	}
	if err != nil {
		return nil, "", err
	}
	return snmpNormalizePayload(raw), resolvedEndpoint, nil
}

func (s *SnmpMonitorService) requestExternal(device *model.CmdbSnmpDevice, method, endpoint string) (interface{}, string, error) {
	candidates := buildSNMPEndpointCandidates(device.SnmpURL, endpoint)
	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("invalid snmpUrl")
	}

	moduleCandidates := snmpResolveRequestModules(device)
	var lastErr error

	for _, candidate := range candidates {
		for _, module := range moduleCandidates {
			requestPayload := buildSNMPRequestPayload(device, endpoint)
			requestPayload["module"] = module

			raw, err := s.doRequest(method, candidate, requestPayload)
			if err == nil {
				return raw, candidate, nil
			}
			lastErr = err
			if !snmpShouldRetryWithNextModule(err) {
				break
			}
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("failed to call external snmp api")
	}
	return nil, "", lastErr
}

func snmpShouldRetryWithNextModule(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "unknown module")
}

func (s *SnmpMonitorService) doRequest(method, rawURL string, requestPayload map[string]interface{}) (interface{}, error) {
	requestURL, err := buildSNMPRequestURL(rawURL, requestPayload)
	if err != nil {
		return nil, err
	}

	var body io.Reader
	if method != http.MethodGet {
		bodyBytes, err := json.Marshal(requestPayload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewBuffer(bodyBytes)
	}

	req, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-CMDB-SNMP-Device-ID", fmt.Sprintf("%v", requestPayload["deviceId"]))
	req.Header.Set("X-CMDB-SNMP-Metric-Type", fmt.Sprintf("%v", requestPayload["metricType"]))
	if method != http.MethodGet {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("external snmp api returned status %d: %s", resp.StatusCode, snmpTruncate(string(bodyBytes), 240))
	}
	if len(bytes.TrimSpace(bodyBytes)) == 0 {
		return map[string]interface{}{}, nil
	}
	if snmpLooksLikeHTML(bodyBytes) {
		return nil, fmt.Errorf("external snmp api returned html instead of metrics: %s", snmpTruncate(string(bodyBytes), 240))
	}

	var decoded interface{}
	if err := json.Unmarshal(bodyBytes, &decoded); err != nil {
		return map[string]interface{}{
			"rawText": strings.TrimSpace(string(bodyBytes)),
		}, nil
	}
	return decoded, nil
}

func buildSNMPRequestURL(rawURL string, requestPayload map[string]interface{}) (string, error) {
	normalized := ensureSNMPScheme(rawURL)
	parsedURL, err := url.Parse(normalized)
	if err != nil {
		return "", err
	}

	query := parsedURL.Query()
	for key, value := range requestPayload {
		if value == nil {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if text == "" {
			continue
		}
		query.Set(key, text)
	}

	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func buildSNMPRequestPayload(device *model.CmdbSnmpDevice, endpoint string) map[string]interface{} {
	profile := snmpResolveVendorProfile(device)

	payload := map[string]interface{}{
		"deviceId":      device.ID,
		"deviceName":    strings.TrimSpace(device.DeviceName),
		"deviceType":    strings.TrimSpace(device.DeviceType),
		"deviceVendor":  strings.TrimSpace(device.DeviceVendor),
		"deviceModel":   strings.TrimSpace(device.DeviceModel),
		"ipAddress":     strings.TrimSpace(device.IPAddress),
		"metricType":    strings.TrimSpace(endpoint),
		"vendorProfile": profile.Name,
		"module":        snmpResolveRequestModule(device, snmpCurrentURLModule(device)),
	}
	return payload
}

func buildSNMPEndpointCandidates(rawURL, endpoint string) []string {
	normalized := ensureSNMPScheme(strings.TrimSpace(rawURL))
	if normalized == "" {
		return nil
	}

	parsedURL, err := url.Parse(normalized)
	if err != nil {
		return []string{normalized}
	}
	if isSNMPExporterURL(parsedURL) {
		return []string{parsedURL.String()}
	}
	if endpoint == "" {
		return []string{parsedURL.String()}
	}

	trimmedPath := strings.TrimRight(parsedURL.Path, "/")
	lastSegment := path.Base(trimmedPath)
	explicitEndpoint := isSNMPEndpointSegment(lastSegment)

	candidates := make([]string, 0, 3)
	seen := make(map[string]struct{})
	addCandidate := func(candidate string) {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			return
		}
		if _, exists := seen[candidate]; exists {
			return
		}
		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
	}

	appendEndpoint := func() {
		candidate := *parsedURL
		candidate.Path = appendSNMPEndpoint(trimmedPath, endpoint)
		addCandidate(candidate.String())
	}
	replaceEndpoint := func() {
		if !explicitEndpoint {
			return
		}
		candidate := *parsedURL
		candidate.Path = replaceSNMPEndpoint(trimmedPath, endpoint)
		addCandidate(candidate.String())
	}

	switch {
	case explicitEndpoint && lastSegment == endpoint:
		addCandidate(parsedURL.String())
	case explicitEndpoint:
		replaceEndpoint()
		addCandidate(parsedURL.String())
	case parsedURL.RawQuery != "":
		addCandidate(parsedURL.String())
		appendEndpoint()
	default:
		appendEndpoint()
		addCandidate(parsedURL.String())
	}

	return candidates
}

func isSNMPExporterURL(parsedURL *url.URL) bool {
	if parsedURL == nil {
		return false
	}
	lastSegment := strings.ToLower(strings.TrimSpace(path.Base(strings.TrimRight(parsedURL.Path, "/"))))
	if lastSegment == "snmp" {
		return true
	}
	query := parsedURL.Query()
	return query.Get("target") != "" || query.Get("module") != "" || query.Get("auth") != ""
}

func snmpLooksLikeHTML(body []byte) bool {
	trimmed := strings.TrimSpace(strings.ToLower(string(body)))
	return strings.HasPrefix(trimmed, "<!doctype html") || strings.HasPrefix(trimmed, "<html")
}

func appendSNMPEndpoint(basePath, endpoint string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" || basePath == "/" {
		return "/" + endpoint
	}
	return strings.TrimRight(basePath, "/") + "/" + endpoint
}

func replaceSNMPEndpoint(basePath, endpoint string) string {
	basePath = strings.TrimRight(strings.TrimSpace(basePath), "/")
	if basePath == "" {
		return "/" + endpoint
	}
	parts := strings.Split(basePath, "/")
	parts[len(parts)-1] = endpoint
	return strings.Join(parts, "/")
}

func isSNMPEndpointSegment(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "monitor", "resource":
		return true
	default:
		return false
	}
}

func ensureSNMPScheme(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	if strings.Contains(rawURL, "://") {
		return rawURL
	}
	return "http://" + rawURL
}

func buildSNMPMonitorMetrics(device *model.CmdbSnmpDevice, payload map[string]interface{}, endpoint string) *model.DeviceMonitorMetrics {
	payload = snmpEnrichPayloadForDevice(payload, device)

	deviceName := snmpFirstNonEmpty(
		snmpExtractString(payload, "deviceName", "device_name", "name", "device.name"),
		device.DeviceName,
	)
	deviceType := snmpFirstNonEmpty(
		snmpExtractString(payload, "deviceType", "device_type", "device.type"),
		device.DeviceType,
	)
	deviceVendor := snmpFirstNonEmpty(
		snmpExtractString(payload, "deviceVendor", "device_vendor", "vendor", "device.vendor"),
		device.DeviceVendor,
	)
	deviceModel := snmpFirstNonEmpty(
		snmpExtractString(payload, "deviceModel", "device_model", "model", "device.model"),
		device.DeviceModel,
	)
	ipAddress := snmpFirstNonEmpty(
		snmpExtractString(payload, "ipAddress", "ip_address", "ip", "device.ipAddress", "device.ip"),
		device.IPAddress,
	)
	template := model.ResolveSnmpMonitorTemplate(deviceVendor, deviceModel)

	metrics := &model.DeviceMonitorMetrics{
		DeviceID:         device.ID,
		DeviceName:       deviceName,
		DeviceType:       deviceType,
		DeviceVendor:     deviceVendor,
		DeviceModel:      deviceModel,
		IPAddress:        ipAddress,
		Status:           snmpResolveStatus(payload, device.Status),
		MetricsAvailable: snmpHasUsefulPayload(payload),
		MetricsSource:    strings.TrimSpace(endpoint),
		MonitorTemplate:  template,
		PortInTop5:       []model.PortTrafficStat{},
		PortOutTop5:      []model.PortTrafficStat{},
		CpuTrend:         []model.TrendPoint{},
		MemoryTrend:      []model.TrendPoint{},
		TemperatureTrend: []model.TrendPoint{},
		LastUpdateTime:   snmpResolveTimestamp(payload, snmpFallbackTimestamp(device.LastSeen.Time)),
	}

	if value, ok := snmpExtractNumber(payload,
		"cpuUsage", "cpu_usage", "cpuPercent", "cpu_percent", "cpu",
		"systemCpuUsage", "system_cpu_usage", "overview.cpuUsage", "overview.cpu_usage",
	); ok {
		metrics.CpuUsage = value
	}
	if value, ok := snmpExtractNumber(payload,
		"memoryUsage", "memory_usage", "memoryPercent", "memory_percent", "memory",
		"systemMemUsage", "system_mem_usage", "overview.memoryUsage", "overview.memory_usage",
	); ok {
		metrics.MemoryUsage = value
	}
	if value, ok := snmpExtractNumber(payload,
		"temperature", "temp", "temperatureCelsius", "temperature_celsius",
		"overview.temperature", "environment.temperature",
	); ok {
		metrics.Temperature = value
	}
	if value, ok := snmpExtractNumber(payload,
		"uptimeDays", "uptime_days", "overview.uptimeDays", "overview.uptime_days",
	); ok {
		metrics.UptimeDays = value
	} else if seconds, ok := snmpExtractNumber(payload, "uptimeSeconds", "uptime_seconds", "uptimeSec", "uptime_sec"); ok {
		metrics.UptimeDays = seconds / 86400
	}

	metrics.PortInTop5 = snmpParsePortStats(payload,
		"portInTop5", "port_in_top5", "ports.inboundTop", "ports.topInbound",
		"traffic.portInTop5", "traffic.inboundTop", "interfaceTrafficTop",
	)
	metrics.PortOutTop5 = snmpParsePortStats(payload,
		"portOutTop5", "port_out_top5", "ports.outboundTop", "ports.topOutbound",
		"traffic.portOutTop5", "traffic.outboundTop", "interfaceTrafficTop",
	)

	if value, ok := snmpExtractInt(payload,
		"idlePortCount", "idle_port_count", "portStats.idlePortCount", "portStats.idle_port_count",
	); ok {
		metrics.IdlePortCount = value
	}
	if value, ok := snmpExtractInt(payload,
		"totalPortCount", "total_port_count", "portCount", "port_count",
		"portStats.totalPortCount", "portStats.total_port_count",
	); ok {
		metrics.TotalPortCount = value
	}

	metrics.CpuTrend = snmpParseTrendPoints(payload,
		"cpuTrend", "cpu_trend", "trends.cpu", "history.cpu", "overview.cpuTrend",
	)
	metrics.MemoryTrend = snmpParseTrendPoints(payload,
		"memoryTrend", "memory_trend", "trends.memory", "history.memory", "overview.memoryTrend",
	)
	metrics.TemperatureTrend = snmpParseTrendPoints(payload,
		"temperatureTrend", "temperature_trend", "trends.temperature", "history.temperature",
	)

	if snmpHasUsefulPayload(payload) {
		metrics.MonitorPayload = payload
	}

	return metrics
}

func buildSNMPResourceMetrics(device *model.CmdbSnmpDevice, payload map[string]interface{}, endpoint string) *model.DeviceResourceMetrics {
	payload = snmpEnrichPayloadForDevice(payload, device)

	deviceName := snmpFirstNonEmpty(
		snmpExtractString(payload, "deviceName", "device_name", "name", "device.name"),
		device.DeviceName,
	)
	deviceType := snmpFirstNonEmpty(
		snmpExtractString(payload, "deviceType", "device_type", "device.type"),
		device.DeviceType,
	)
	deviceVendor := snmpFirstNonEmpty(
		snmpExtractString(payload, "deviceVendor", "device_vendor", "vendor", "device.vendor"),
		device.DeviceVendor,
	)
	deviceModel := snmpFirstNonEmpty(
		snmpExtractString(payload, "deviceModel", "device_model", "model", "device.model"),
		device.DeviceModel,
	)
	ipAddress := snmpFirstNonEmpty(
		snmpExtractString(payload, "ipAddress", "ip_address", "ip", "device.ipAddress", "device.ip"),
		device.IPAddress,
	)
	template := model.ResolveSnmpMonitorTemplate(deviceVendor, deviceModel)

	resource := &model.DeviceResourceMetrics{
		DeviceID:         device.ID,
		DeviceName:       deviceName,
		DeviceType:       deviceType,
		DeviceVendor:     deviceVendor,
		DeviceModel:      deviceModel,
		IPAddress:        ipAddress,
		Status:           snmpResolveStatus(payload, device.Status),
		UpdateTime:       snmpResolveTimestamp(payload, snmpFallbackTimestamp(device.LastSeen.Time)),
		MetricsAvailable: snmpHasUsefulPayload(payload),
		MetricsSource:    strings.TrimSpace(endpoint),
		MonitorTemplate:  template.Summary(),
	}

	if value, ok := snmpExtractNumber(payload,
		"cpuUsage", "cpu_usage", "cpuPercent", "cpu_percent", "cpu",
		"systemCpuUsage", "system_cpu_usage", "overview.cpuUsage", "overview.cpu_usage",
	); ok {
		resource.CpuUsage = value
	}
	if value, ok := snmpExtractNumber(payload,
		"memoryUsage", "memory_usage", "memoryPercent", "memory_percent", "memory",
		"systemMemUsage", "system_mem_usage", "overview.memoryUsage", "overview.memory_usage",
	); ok {
		resource.MemoryUsage = value
	}

	return resource
}

func snmpNormalizePayload(raw interface{}) map[string]interface{} {
	current := snmpUnwrapPayload(raw)
	switch value := current.(type) {
	case map[string]interface{}:
		return value
	case []interface{}:
		return map[string]interface{}{"items": value}
	case nil:
		return map[string]interface{}{}
	default:
		return map[string]interface{}{"value": value}
	}
}

func snmpEnrichPayload(payload map[string]interface{}) map[string]interface{} {
	return snmpEnrichPayloadForDevice(payload, nil)
}

func snmpEnrichPayloadForDevice(payload map[string]interface{}, device *model.CmdbSnmpDevice) map[string]interface{} {
	if payload == nil {
		return map[string]interface{}{}
	}

	rawText := snmpFindPrometheusRawText(payload, 3)
	if rawText == "" {
		return payload
	}

	parsed := snmpParsePrometheusTextWithProfile(rawText, snmpResolveVendorProfile(device))
	if len(parsed) == 0 {
		return payload
	}

	merged := make(map[string]interface{}, len(payload)+len(parsed))
	for key, value := range payload {
		merged[key] = value
	}
	for key, value := range parsed {
		if key == "rawText" || key == "raw_text" {
			continue
		}
		merged[key] = value
	}
	return merged
}

func snmpFindPrometheusRawText(payload map[string]interface{}, depth int) string {
	if payload == nil {
		return ""
	}

	if rawText := snmpExtractString(payload, "rawText", "raw_text"); rawText != "" {
		return rawText
	}
	if depth <= 0 {
		return ""
	}

	for _, value := range payload {
		switch current := value.(type) {
		case map[string]interface{}:
			if rawText := snmpFindPrometheusRawText(current, depth-1); rawText != "" {
				return rawText
			}
		case []interface{}:
			for _, item := range current {
				nested, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				if rawText := snmpFindPrometheusRawText(nested, depth-1); rawText != "" {
					return rawText
				}
			}
		}
	}

	return ""
}

func snmpUnwrapPayload(raw interface{}) interface{} {
	current := raw
	for i := 0; i < 4; i++ {
		object, ok := current.(map[string]interface{})
		if !ok {
			return current
		}
		if snmpHasMetricFields(object) {
			return object
		}
		next, ok := snmpPickNestedPayload(object)
		if !ok {
			return object
		}
		current = next
	}
	return current
}

func snmpPickNestedPayload(object map[string]interface{}) (interface{}, bool) {
	for _, key := range []string{"data", "result", "payload", "metrics"} {
		if value, ok := object[key]; ok && value != nil {
			return value, true
		}
	}
	return nil, false
}

func snmpHasMetricFields(object map[string]interface{}) bool {
	keys := []string{
		"cpuUsage", "cpu_usage", "memoryUsage", "memory_usage",
		"temperature", "temperatureCelsius", "uptimeDays", "uptime_days",
		"portInTop5", "portOutTop5", "cpuTrend", "memoryTrend", "temperatureTrend",
		"overview", "interfaces", "ports", "trends", "history", "deviceStatus",
		"monitorStatus", "interfaceTrafficTop", "fanStatus", "powerStatus",
	}

	for _, key := range keys {
		if _, ok := snmpLookup(object, key); ok {
			return true
		}
	}
	return false
}

func snmpHasUsefulPayload(payload map[string]interface{}) bool {
	if len(payload) == 0 {
		return false
	}
	if snmpHasMetricFields(payload) {
		return true
	}

	generic := map[string]struct{}{
		"code":    {},
		"msg":     {},
		"message": {},
		"status":  {},
		"success": {},
		"error":   {},
		"rawtext": {},
	}

	for key := range payload {
		if _, ok := generic[strings.ToLower(strings.TrimSpace(key))]; !ok {
			return true
		}
	}
	return false
}

func snmpExtractString(object map[string]interface{}, paths ...string) string {
	for _, candidate := range paths {
		value, ok := snmpLookup(object, candidate)
		if !ok {
			continue
		}
		text := strings.TrimSpace(snmpString(value))
		if text != "" {
			return text
		}
	}
	return ""
}

func snmpExtractNumber(object map[string]interface{}, paths ...string) (float64, bool) {
	for _, candidate := range paths {
		value, ok := snmpLookup(object, candidate)
		if !ok {
			continue
		}
		number, ok := snmpParseNumber(value)
		if ok {
			return number, true
		}
	}
	return 0, false
}

func snmpExtractInt(object map[string]interface{}, paths ...string) (int, bool) {
	value, ok := snmpExtractNumber(object, paths...)
	if !ok {
		return 0, false
	}
	return int(value), true
}

func snmpExtractNumberOrZero(object map[string]interface{}, paths ...string) float64 {
	value, ok := snmpExtractNumber(object, paths...)
	if !ok {
		return 0
	}
	return value
}

func snmpLookup(object map[string]interface{}, keyPath string) (interface{}, bool) {
	parts := strings.Split(strings.TrimSpace(keyPath), ".")
	if len(parts) == 0 {
		return nil, false
	}

	var current interface{} = object
	for _, part := range parts {
		asMap, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}

		value, exists := asMap[part]
		if !exists {
			for currentKey, currentValue := range asMap {
				if strings.EqualFold(strings.TrimSpace(currentKey), strings.TrimSpace(part)) {
					value = currentValue
					exists = true
					break
				}
			}
		}
		if !exists {
			return nil, false
		}
		current = value
	}

	return current, true
}

func snmpString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case json.Number:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprint(value)
	}
}

func snmpParseNumber(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint64:
		return float64(v), true
	case json.Number:
		number, err := v.Float64()
		if err == nil {
			return number, true
		}
	case string:
		return snmpParseNumberString(v)
	}
	return 0, false
}

func snmpParseNumberString(value string) (float64, bool) {
	cleaned := strings.TrimSpace(value)
	if cleaned == "" {
		return 0, false
	}

	cleaned = strings.ReplaceAll(cleaned, ",", "")
	cleaned = strings.TrimSuffix(cleaned, "%")
	cleaned = strings.TrimSuffix(cleaned, "C")

	if number, err := strconv.ParseFloat(cleaned, 64); err == nil {
		return number, true
	}

	var builder strings.Builder
	for index, char := range cleaned {
		switch {
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case char == '.' || char == 'e' || char == 'E':
			builder.WriteRune(char)
		case (char == '-' || char == '+') && index == 0:
			builder.WriteRune(char)
		}
	}

	if builder.Len() == 0 {
		return 0, false
	}

	number, err := strconv.ParseFloat(builder.String(), 64)
	if err != nil {
		return 0, false
	}
	return number, true
}

type snmpPrometheusSample struct {
	Name   string
	Labels map[string]string
	Value  float64
}

type snmpPrometheusPort struct {
	Index        int
	Name         string
	InOctets     float64
	OutOctets    float64
	InRateMbps   float64
	OutRateMbps  float64
	InRateSet    bool
	OutRateSet   bool
	SpeedMbps    float64
	AdminStatus  float64
	OperStatus   float64
	HasAdminStat bool
	HasOperStat  bool
}

type snmpPrometheusMemoryPool struct {
	Used float64
	Free float64
}

func snmpParsePrometheusText(raw string) map[string]interface{} {
	return snmpParsePrometheusTextWithProfile(raw, snmpResolveVendorProfile(nil))
}

func snmpParsePrometheusTextWithProfile(raw string, profile snmpVendorProfile) map[string]interface{} {
	samples := snmpParsePrometheusSamples(raw)
	if len(samples) == 0 {
		return nil
	}

	result := map[string]interface{}{
		"metricsFormat": "prometheus",
		"deviceStatus":  "online",
	}
	ports := make(map[string]*snmpPrometheusPort)
	memoryPools := make(map[string]*snmpPrometheusMemoryPool)
	cpuUsage := 0.0
	cpuSet := false
	memoryUsage := 0.0
	memorySet := false
	temperatureSet := false
	temperature := 0.0

	for _, sample := range samples {
		normalizedName := snmpNormalizePrometheusMetricName(sample.Name)

		switch normalizedName {
		case "up":
			if sample.Value <= 0 {
				result["deviceStatus"] = "offline"
			}
		case "ifnumber":
			result["totalPortCount"] = int(sample.Value)
		case "sysuptime", "sysuptimeinstance":
			result["uptimeDays"] = sample.Value / 100 / 86400
		}

		if snmpMetricIn(normalizedName, profile.CPUMetricNames) || snmpPrometheusLooksLikeCPU(normalizedName) {
			if !cpuSet || sample.Value > cpuUsage {
				cpuUsage = sample.Value
				cpuSet = true
			}
		}
		if snmpMetricIn(normalizedName, profile.MemoryMetricNames) || snmpPrometheusLooksLikeMemory(normalizedName) {
			if !memorySet || sample.Value > memoryUsage {
				memoryUsage = sample.Value
				memorySet = true
			}
		}
		if snmpMetricIn(normalizedName, profile.TempMetricNames) || snmpPrometheusLooksLikeTemperature(normalizedName) {
			if !temperatureSet || sample.Value > temperature {
				temperature = sample.Value
				temperatureSet = true
			}
		}
		if snmpMetricIn(normalizedName, profile.MemoryUsedMetrics) || snmpMetricIn(normalizedName, profile.MemoryFreeMetrics) {
			key := snmpPrometheusMemoryPoolKey(sample)
			if key != "" {
				pool := snmpEnsurePrometheusMemoryPool(memoryPools, key)
				snmpApplyPrometheusMemoryPoolSample(pool, normalizedName, sample.Value, profile)
			}
		}

		if portKey := snmpPrometheusPortKey(sample); portKey != "" {
			port := snmpEnsurePrometheusPort(ports, portKey, sample)
			snmpApplyPrometheusPortSample(port, normalizedName, sample.Value)
		}
	}

	if cpuSet {
		result["cpuUsage"] = cpuUsage
	}
	if memorySet {
		result["memoryUsage"] = memoryUsage
	} else if poolUsage, ok := snmpBuildPrometheusMemoryUsage(memoryPools); ok {
		result["memoryUsage"] = poolUsage
	}
	if temperatureSet {
		result["temperature"] = temperature
	}
	if len(ports) > 0 {
		if _, exists := result["totalPortCount"]; !exists {
			result["totalPortCount"] = len(ports)
		}
		result["idlePortCount"] = snmpCountIdlePrometheusPorts(ports)
		result["portInTop5"] = snmpBuildPrometheusPortStats(ports, "in")
		result["portOutTop5"] = snmpBuildPrometheusPortStats(ports, "out")
	}

	return result
}

func snmpParsePrometheusSamples(raw string) []snmpPrometheusSample {
	lines := strings.Split(raw, "\n")
	samples := make([]snmpPrometheusSample, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		sample, ok := snmpParsePrometheusSampleLine(line)
		if ok {
			samples = append(samples, sample)
		}
	}

	return samples
}

func snmpParsePrometheusSampleLine(line string) (snmpPrometheusSample, bool) {
	var sample snmpPrometheusSample
	line = strings.TrimSpace(line)
	if line == "" {
		return sample, false
	}

	nameEnd := strings.IndexAny(line, "{ \t")
	if nameEnd < 0 {
		return sample, false
	}
	sample.Name = strings.TrimSpace(line[:nameEnd])
	if sample.Name == "" {
		return sample, false
	}

	rest := strings.TrimSpace(line[nameEnd:])
	sample.Labels = map[string]string{}
	if strings.HasPrefix(rest, "{") {
		labelEnd := snmpFindPrometheusLabelEnd(rest)
		if labelEnd < 0 {
			return sample, false
		}
		sample.Labels = snmpParsePrometheusLabels(rest[1:labelEnd])
		rest = strings.TrimSpace(rest[labelEnd+1:])
	}

	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return sample, false
	}
	value, ok := snmpParseNumberString(fields[0])
	if !ok {
		return sample, false
	}
	sample.Value = value
	return sample, true
}

func snmpFindPrometheusLabelEnd(value string) int {
	inQuote := false
	escaped := false
	for idx, char := range value {
		if idx == 0 {
			continue
		}
		if escaped {
			escaped = false
			continue
		}
		switch char {
		case '\\':
			if inQuote {
				escaped = true
			}
		case '"':
			inQuote = !inQuote
		case '}':
			if !inQuote {
				return idx
			}
		}
	}
	return -1
}

func snmpParsePrometheusLabels(raw string) map[string]string {
	labels := map[string]string{}
	for _, part := range snmpSplitPrometheusLabelParts(raw) {
		if part == "" {
			continue
		}
		key, value, found := strings.Cut(part, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		} else {
			value = strings.Trim(value, `"`)
		}
		labels[key] = value
	}
	return labels
}

func snmpSplitPrometheusLabelParts(raw string) []string {
	parts := []string{}
	var builder strings.Builder
	inQuote := false
	escaped := false
	for _, char := range raw {
		if escaped {
			builder.WriteRune(char)
			escaped = false
			continue
		}
		switch char {
		case '\\':
			builder.WriteRune(char)
			if inQuote {
				escaped = true
			}
		case '"':
			builder.WriteRune(char)
			inQuote = !inQuote
		case ',':
			if inQuote {
				builder.WriteRune(char)
				continue
			}
			parts = append(parts, strings.TrimSpace(builder.String()))
			builder.Reset()
		default:
			builder.WriteRune(char)
		}
	}
	if builder.Len() > 0 {
		parts = append(parts, strings.TrimSpace(builder.String()))
	}
	return parts
}

func snmpNormalizePrometheusMetricName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	replacer := strings.NewReplacer("_", "", "-", "", ":", "")
	return replacer.Replace(name)
}

func snmpPrometheusLooksLikeCPU(name string) bool {
	return strings.Contains(name, "cpu") &&
		(strings.Contains(name, "usage") || strings.Contains(name, "utilization")) &&
		!strings.HasPrefix(name, "if")
}

func snmpPrometheusLooksLikeMemory(name string) bool {
	return (strings.Contains(name, "memory") || strings.Contains(name, "mem")) &&
		(strings.Contains(name, "usage") || strings.Contains(name, "utilization")) &&
		!strings.HasPrefix(name, "if")
}

func snmpPrometheusLooksLikeTemperature(name string) bool {
	return strings.Contains(name, "temperature") &&
		(strings.Contains(name, "reading") || strings.Contains(name, "celsius") || name == "temperature")
}

func snmpPrometheusPortKey(sample snmpPrometheusSample) string {
	if value := snmpPrometheusLabel(sample.Labels, "ifIndex", "if_index", "index"); value != "" {
		return value
	}
	if value := snmpPrometheusLabel(sample.Labels, "ifName", "if_name", "ifDescr", "if_descr"); value != "" {
		return value
	}
	return ""
}

func snmpPrometheusLabel(labels map[string]string, names ...string) string {
	for _, name := range names {
		for key, value := range labels {
			if strings.EqualFold(strings.TrimSpace(key), strings.TrimSpace(name)) {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func snmpPrometheusMemoryPoolKey(sample snmpPrometheusSample) string {
	return snmpFirstNonEmpty(
		snmpPrometheusLabel(sample.Labels, "ciscoMemoryPoolName", "cisco_memory_pool_name", "name"),
		snmpPrometheusLabel(sample.Labels, "ciscoMemoryPoolIndex", "cisco_memory_pool_index", "index"),
		snmpPrometheusLabel(sample.Labels, "entPhysicalIndex", "ent_physical_index"),
	)
}

func snmpEnsurePrometheusMemoryPool(pools map[string]*snmpPrometheusMemoryPool, key string) *snmpPrometheusMemoryPool {
	if pool, exists := pools[key]; exists {
		return pool
	}
	pool := &snmpPrometheusMemoryPool{}
	pools[key] = pool
	return pool
}

func snmpApplyPrometheusMemoryPoolSample(pool *snmpPrometheusMemoryPool, name string, value float64, profile snmpVendorProfile) {
	if pool == nil {
		return
	}
	switch {
	case snmpMetricIn(name, profile.MemoryUsedMetrics):
		pool.Used = value
	case snmpMetricIn(name, profile.MemoryFreeMetrics):
		pool.Free = value
	}
}

func snmpBuildPrometheusMemoryUsage(pools map[string]*snmpPrometheusMemoryPool) (float64, bool) {
	totalUsed := 0.0
	totalFree := 0.0
	for _, pool := range pools {
		if pool == nil {
			continue
		}
		totalUsed += pool.Used
		totalFree += pool.Free
	}
	if totalUsed <= 0 || totalFree < 0 {
		return 0, false
	}
	total := totalUsed + totalFree
	if total <= 0 {
		return 0, false
	}
	return totalUsed / total * 100, true
}

func snmpEnsurePrometheusPort(ports map[string]*snmpPrometheusPort, key string, sample snmpPrometheusSample) *snmpPrometheusPort {
	if port, exists := ports[key]; exists {
		snmpUpdatePrometheusPortIdentity(port, sample)
		return port
	}

	port := &snmpPrometheusPort{}
	if index, err := strconv.Atoi(strings.TrimSpace(key)); err == nil {
		port.Index = index
	}
	snmpUpdatePrometheusPortIdentity(port, sample)
	ports[key] = port
	return port
}

func snmpUpdatePrometheusPortIdentity(port *snmpPrometheusPort, sample snmpPrometheusSample) {
	if port == nil {
		return
	}
	if port.Index == 0 {
		if indexText := snmpPrometheusLabel(sample.Labels, "ifIndex", "if_index", "index"); indexText != "" {
			if index, err := strconv.Atoi(indexText); err == nil {
				port.Index = index
			}
		}
	}
	if port.Name == "" {
		port.Name = snmpFirstNonEmpty(
			snmpPrometheusLabel(sample.Labels, "ifName", "if_name"),
			snmpPrometheusLabel(sample.Labels, "ifDescr", "if_descr"),
			snmpPrometheusLabel(sample.Labels, "ifAlias", "if_alias"),
		)
	}
}

func snmpApplyPrometheusPortSample(port *snmpPrometheusPort, name string, value float64) {
	if port == nil {
		return
	}

	switch name {
	case "ifhcinoctets", "ifinoctets":
		port.InOctets = value
	case "ifhcoutoctets", "ifoutoctets":
		port.OutOctets = value
	case "ifinoctetsrate", "ifhcinoctetsrate", "ifinoctetspersecond", "ifhcinoctetspersecond":
		port.InRateMbps = value * 8 / 1_000_000
		port.InRateSet = true
	case "ifinbps", "ifhcinbps", "ifinbitspersecond":
		port.InRateMbps = value / 1_000_000
		port.InRateSet = true
	case "ifoutoctetsrate", "ifhcoutoctetsrate", "ifoutoctetspersecond", "ifhcoutoctetspersecond":
		port.OutRateMbps = value * 8 / 1_000_000
		port.OutRateSet = true
	case "ifoutbps", "ifhcoutbps", "ifoutbitspersecond":
		port.OutRateMbps = value / 1_000_000
		port.OutRateSet = true
	case "ifhighspeed":
		port.SpeedMbps = value
	case "ifspeed":
		if port.SpeedMbps == 0 {
			port.SpeedMbps = value / 1_000_000
		}
	case "ifadminstatus":
		port.AdminStatus = value
		port.HasAdminStat = true
	case "ifoperstatus":
		port.OperStatus = value
		port.HasOperStat = true
	}
}

func snmpCountIdlePrometheusPorts(ports map[string]*snmpPrometheusPort) int {
	idle := 0
	for _, port := range ports {
		if port == nil {
			continue
		}
		switch {
		case port.HasOperStat && int(port.OperStatus) != 1:
			idle++
		case !port.HasOperStat && port.HasAdminStat && int(port.AdminStatus) != 1:
			idle++
		case !port.HasOperStat && !port.HasAdminStat && port.InOctets == 0 && port.OutOctets == 0:
			idle++
		}
	}
	return idle
}

func snmpBuildPrometheusPortStats(ports map[string]*snmpPrometheusPort, direction string) []model.PortTrafficStat {
	stats := make([]model.PortTrafficStat, 0, len(ports))
	for _, port := range ports {
		if port == nil {
			continue
		}

		trafficBytes := port.InOctets
		trafficRate := port.InRateMbps
		rateSet := port.InRateSet
		if direction == "out" {
			trafficBytes = port.OutOctets
			trafficRate = port.OutRateMbps
			rateSet = port.OutRateSet
		}

		utilization := 0.0
		if rateSet && port.SpeedMbps > 0 {
			utilization = trafficRate / port.SpeedMbps * 100
		}

		stats = append(stats, model.PortTrafficStat{
			PortIndex:       port.Index,
			PortName:        port.Name,
			TrafficRateMbps: trafficRate,
			TrafficBytes:    trafficBytes,
			Utilization:     utilization,
			PortSpeed:       port.SpeedMbps,
		})
	}

	sort.Slice(stats, func(i, j int) bool {
		return snmpPortStatSortValue(stats[i]) > snmpPortStatSortValue(stats[j])
	})
	if len(stats) > 5 {
		stats = stats[:5]
	}
	return stats
}

func snmpPortStatSortValue(stat model.PortTrafficStat) float64 {
	if stat.TrafficRateMbps > 0 {
		return stat.TrafficRateMbps
	}
	return stat.TrafficBytes
}

func snmpParsePortStats(payload map[string]interface{}, paths ...string) []model.PortTrafficStat {
	for _, candidate := range paths {
		value, ok := snmpLookup(payload, candidate)
		if !ok {
			continue
		}

		switch typed := value.(type) {
		case []model.PortTrafficStat:
			return typed
		case []interface{}:
			items := typed
			stats := make([]model.PortTrafficStat, 0, len(items))
			for _, item := range items {
				object, ok := item.(map[string]interface{})
				if !ok {
					continue
				}

				traffic, trafficOK := snmpExtractNumber(object,
					"trafficRateMbps", "traffic_rate_mbps", "rateMbps", "rate_mbps",
					"value", "rate", "traffic", "mbps", "inRateMbps", "outRateMbps",
					"in_rate_mbps", "out_rate_mbps",
				)
				utilization, _ := snmpExtractNumber(object,
					"utilization", "utilizationPct", "utilization_pct", "usageRate", "usage_rate",
				)
				speed, _ := snmpExtractNumber(object,
					"portSpeed", "port_speed", "speedMbps", "speed_mbps", "speed",
				)
				index, _ := snmpExtractInt(object,
					"portIndex", "port_index", "ifIndex", "if_index", "index",
				)
				name := snmpExtractString(object,
					"portName", "port_name", "ifName", "if_name", "name", "interface", "port",
				)
				trafficBytes := snmpExtractNumberOrZero(object, "trafficBytes", "traffic_bytes", "bytes", "octets")

				if !trafficOK && trafficBytes == 0 && name == "" && index == 0 {
					continue
				}

				stats = append(stats, model.PortTrafficStat{
					PortIndex:       index,
					PortName:        name,
					TrafficRateMbps: traffic,
					TrafficBytes:    trafficBytes,
					Utilization:     utilization,
					PortSpeed:       speed,
				})
			}

			sort.Slice(stats, func(i, j int) bool {
				return snmpPortStatSortValue(stats[i]) > snmpPortStatSortValue(stats[j])
			})

			if len(stats) > 5 {
				stats = stats[:5]
			}
			return stats
		default:
			continue
		}
	}

	return []model.PortTrafficStat{}
}

func snmpParseTrendPoints(payload map[string]interface{}, paths ...string) []model.TrendPoint {
	for _, candidate := range paths {
		value, ok := snmpLookup(payload, candidate)
		if !ok {
			continue
		}

		items, ok := value.([]interface{})
		if !ok {
			continue
		}

		points := make([]model.TrendPoint, 0, len(items))
		for _, item := range items {
			switch current := item.(type) {
			case []interface{}:
				if len(current) < 2 {
					continue
				}

				timestamp := snmpFormatTimestamp(current[0])
				metricValue, ok := snmpParseNumber(current[1])
				if !ok {
					continue
				}
				if timestamp == "" {
					timestamp = time.Now().Format(time.RFC3339)
				}

				points = append(points, model.TrendPoint{
					Timestamp: timestamp,
					Value:     metricValue,
				})
			case map[string]interface{}:
				timestamp := snmpFirstNonEmpty(
					snmpFormatTimestamp(current["timestamp"]),
					snmpFormatTimestamp(current["time"]),
					snmpFormatTimestamp(current["ts"]),
					snmpFormatTimestamp(current["date"]),
				)
				metricValue, ok := snmpExtractNumber(current,
					"value", "metric", "usage", "rate", "cpuUsage", "memoryUsage", "temperature",
				)
				if !ok {
					continue
				}
				if timestamp == "" {
					timestamp = time.Now().Format(time.RFC3339)
				}

				points = append(points, model.TrendPoint{
					Timestamp: timestamp,
					Value:     metricValue,
				})
			}
		}

		return points
	}

	return []model.TrendPoint{}
}

func snmpResolveStatus(payload map[string]interface{}, fallback string) string {
	for _, key := range []string{"deviceStatus", "device_status", "monitorStatus", "monitor_status", "healthStatus", "health_status"} {
		value, ok := snmpLookup(payload, key)
		if !ok {
			continue
		}
		if status := snmpNormalizeStatus(value); status != "" {
			return status
		}
	}

	for _, key := range []string{"online", "available", "reachable"} {
		value, ok := snmpLookup(payload, key)
		if !ok {
			continue
		}
		if boolean, ok := value.(bool); ok {
			if boolean {
				return "online"
			}
			return "offline"
		}
	}

	fallback = strings.TrimSpace(fallback)
	if fallback != "" {
		return fallback
	}
	return "unknown"
}

func snmpNormalizeStatus(value interface{}) string {
	switch strings.ToLower(strings.TrimSpace(snmpString(value))) {
	case "online", "up", "healthy":
		return "online"
	case "offline", "down", "unreachable", "critical":
		return "offline"
	case "maintenance":
		return "maintenance"
	default:
		return ""
	}
}

func snmpResolveTimestamp(payload map[string]interface{}, fallback string) string {
	for _, key := range []string{
		"lastUpdateTime", "last_update_time", "updateTime", "update_time",
		"updatedAt", "updated_at", "timestamp", "time", "sampledAt", "sampled_at",
		"collectedAt", "collected_at",
	} {
		value, ok := snmpLookup(payload, key)
		if !ok {
			continue
		}
		if timestamp := snmpFormatTimestamp(value); timestamp != "" {
			return timestamp
		}
	}

	fallback = strings.TrimSpace(fallback)
	if fallback != "" {
		return fallback
	}
	return time.Now().Format(time.RFC3339)
}

func snmpFormatTimestamp(value interface{}) string {
	switch current := value.(type) {
	case string:
		text := strings.TrimSpace(current)
		if text == "" || strings.HasPrefix(text, "0001-01-01") {
			return ""
		}
		if parsed, ok := snmpParseTimestampString(text); ok {
			if parsed.IsZero() {
				return ""
			}
			return parsed.Format(time.RFC3339)
		}
		return text
	case float64:
		return snmpUnixToRFC3339(int64(current))
	case int64:
		return snmpUnixToRFC3339(current)
	case int:
		return snmpUnixToRFC3339(int64(current))
	case json.Number:
		if parsed, err := current.Int64(); err == nil {
			return snmpUnixToRFC3339(parsed)
		}
	}
	return ""
}

func snmpParseTimestampString(value string) (time.Time, bool) {
	if number, ok := snmpParseNumberString(value); ok {
		return snmpNormalizeUnix(int64(number)), true
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006/01/02 15:04",
		"2006-01-02",
	}

	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, true
		}
	}

	return time.Time{}, false
}

func snmpUnixToRFC3339(value int64) string {
	if value == 0 {
		return ""
	}
	return snmpNormalizeUnix(value).Format(time.RFC3339)
}

func snmpNormalizeUnix(value int64) time.Time {
	switch {
	case value > 1_000_000_000_000_000_000:
		return time.Unix(0, value)
	case value > 1_000_000_000_000_000:
		return time.Unix(0, value*1000)
	case value > 1_000_000_000_000:
		return time.Unix(0, value*int64(time.Millisecond))
	default:
		return time.Unix(value, 0)
	}
}

func snmpFirstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func snmpFallbackTimestamp(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func snmpTruncate(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "..."
}
