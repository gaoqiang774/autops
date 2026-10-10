package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/model"
	systemModel "dodevops-api/api/system/model"
	"dodevops-api/common/config"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"
	"dodevops-api/pkg/redis"

	"github.com/gin-gonic/gin"
)

type hostResult struct {
	id      uint
	metrics *model.HostMetrics
}

type MonitorServiceInterface interface {
	GetHostMetrics(c *gin.Context, hostID uint)
	BatchGetHostMetrics(c *gin.Context, hostIDs []uint)
	GetHostMetricsData(hostID uint, minutes int, step string) (*model.HostMetrics, error)
	GetHostMetricHistory(c *gin.Context, hostID uint, metric string,
		start, end string, step string)
	GetHostAllMetricsHistory(c *gin.Context, hostID uint,
		start, end string, step string)
	GetHostServiceMetricsHistory(c *gin.Context, hostID uint, serviceName, start, end, step string)
	GetTopProcesses(c *gin.Context, hostID uint)
	GetTopProcessesData(hostID uint) (*model.TopProcessesResult, error)
	GetHostPorts(c *gin.Context, hostID uint)
	GetTopUsageHosts(c *gin.Context, days, limit int)
	GetCurrentUsageRankings(c *gin.Context, limit int) // current cpu/memory/disk rankings
	GetCompleteHostMetrics(c *gin.Context, hostID uint)
	GetCompleteHostMetricsData(hostID uint) (*model.CompleteHostMetrics, error)
	GetWindowsDashboardMetrics(c *gin.Context, hostID uint, startTime, endTime string, step string)
}

type MonitorServiceImpl struct {
	hostDao       dao.CmdbHostDao
	agentDao      dao.AgentDao
	prometheusURL string
	cacheExpire   time.Duration
}

const (
	hostCacheKey = "monitor:host:%d"
	cacheTimeout = 5 * time.Minute
)

func NewMonitorService() MonitorServiceInterface {
	return &MonitorServiceImpl{
		hostDao:       dao.NewCmdbHostDao(),
		agentDao:      dao.NewAgentDao(),
		prometheusURL: config.Config.Monitor.Prometheus.URL,
		cacheExpire:   cacheTimeout,
	}
}

func (s *MonitorServiceImpl) checkAgentOnline(hostID uint) bool {
	agent, err := s.agentDao.GetByHostID(hostID)
	if err != nil || agent == nil {
		return false
	}
	return isAgentOnline(agent)
}

func zeroHostMetricsWithStatus(onlineStatus int) *model.HostMetrics {
	return &model.HostMetrics{
		CPUUsage:     0,
		MemoryUsage:  0,
		DiskUsage:    0,
		Load5:        0,
		OnlineStatus: onlineStatus,
	}
}

func (s *MonitorServiceImpl) currentMetricsOnlineStatus(hostID uint, host *model.CmdbHost) int {
	if s.isWindowsHost(host) {
		if s.checkWindowsMetricsOnline(s.getWindowsJobIdentifier(host)) {
			return 0
		}
		return 1
	}

	if s.checkAgentOnline(hostID) {
		return 0
	}
	return 1
}

func (s *MonitorServiceImpl) getInstanceIdentifier(host *model.CmdbHost) string {
	if host.PrivateIP != "" {
		return host.PrivateIP
	}
	if host.Name != "" {
		return host.Name
	}
	return ""
}

func (s *MonitorServiceImpl) isWindowsHost(host *model.CmdbHost) bool {
	if host == nil {
		return false
	}
	if host.HostType == model.HostTypeWindows {
		return true
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(host.OS)), "windows")
}

func (s *MonitorServiceImpl) getWindowsJobIdentifier(host *model.CmdbHost) string {
	if host == nil {
		return ""
	}

	candidates := []string{
		strings.TrimSpace(host.PrivateIP),
		strings.TrimSpace(host.SSHIP),
		strings.TrimSpace(host.PublicIP),
		strings.TrimSpace(host.Name),
		strings.TrimSpace(host.HostName),
	}
	for _, candidate := range candidates {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func (s *MonitorServiceImpl) getMetricsQueryIdentifier(host *model.CmdbHost) string {
	if s.isWindowsHost(host) {
		return s.getWindowsJobIdentifier(host)
	}
	return s.getInstanceIdentifier(host)
}

func (s *MonitorServiceImpl) windowsMetricOnlineTimeout() time.Duration {
	timeout := 3 * time.Minute
	if config.Config != nil && config.Config.WindowsExporter.PushMode.Interval > 0 {
		calculated := time.Duration(config.Config.WindowsExporter.PushMode.Interval*3) * time.Second
		if calculated > timeout {
			timeout = calculated
		}
	}
	return timeout
}

func (s *MonitorServiceImpl) checkWindowsMetricsOnline(jobIdentifier string) bool {
	if strings.TrimSpace(jobIdentifier) == "" {
		return false
	}

	pushTime, ok := s.queryLatestMetricValue(
		fmt.Sprintf("max(push_time_seconds{job=%q})", jobIdentifier),
		time.Time{},
		time.Time{},
		"",
	)
	if !ok {
		return false
	}

	lastPushTime := time.Unix(int64(pushTime), 0)
	return time.Since(lastPushTime) <= s.windowsMetricOnlineTimeout()
}

func parsePrometheusValue(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	case float64:
		return v, true
	default:
		return 0, false
	}
}

func extractLatestMetricValue(result *model.PrometheusQueryResult) (float64, bool) {
	if result == nil || len(result.Data.Result) == 0 {
		return 0, false
	}

	item := result.Data.Result[0]
	if len(item.Values) > 0 {
		lastPoint := item.Values[len(item.Values)-1]
		if len(lastPoint) >= 2 {
			return parsePrometheusValue(lastPoint[1])
		}
	}

	if len(item.Value) >= 2 {
		return parsePrometheusValue(item.Value[1])
	}

	return 0, false
}

func (s *MonitorServiceImpl) queryLatestMetricValue(query string, start, end time.Time, step string) (float64, bool) {
	data, err := s.queryPrometheus(query, start, end, step)
	if err != nil {
		return 0, false
	}
	return extractLatestMetricValue(data)
}

func (s *MonitorServiceImpl) queryRoundedLatestMetricValue(query string, start, end time.Time, step string) float64 {
	value, ok := s.queryLatestMetricValue(query, start, end, step)
	if !ok {
		return 0
	}
	return parseFloat(value)
}

func (s *MonitorServiceImpl) querySummaryHostMetrics(
	hostID uint,
	host *model.CmdbHost,
	startTime, endTime time.Time,
	step string,
) *model.HostMetrics {
	metrics := &model.HostMetrics{
		OnlineStatus: s.currentMetricsOnlineStatus(hostID, host),
	}
	if metrics.OnlineStatus == 1 {
		return zeroHostMetricsWithStatus(1)
	}

	if s.isWindowsHost(host) {
		jobIdentifier := s.getWindowsJobIdentifier(host)
		if jobIdentifier == "" {
			return zeroHostMetricsWithStatus(1)
		}
		s.populateWindowsSummaryMetrics(metrics, jobIdentifier, startTime, endTime, step)
		return metrics
	}

	instanceIdentifier := s.getInstanceIdentifier(host)
	if instanceIdentifier == "" {
		return zeroHostMetricsWithStatus(1)
	}

	s.populateLinuxSummaryMetrics(metrics, instanceIdentifier, startTime, endTime, step)
	return metrics
}

func (s *MonitorServiceImpl) populateLinuxSummaryMetrics(
	metrics *model.HostMetrics,
	instanceIdentifier string,
	startTime, endTime time.Time,
	step string,
) {
	if metrics == nil {
		return
	}

	metrics.CPUUsage = s.queryRoundedLatestMetricValue(
		fmt.Sprintf("system_cpu_usage_percent{instance=%q}", instanceIdentifier),
		startTime, endTime, step,
	)
	metrics.MemoryUsage = s.queryRoundedLatestMetricValue(
		fmt.Sprintf("system_memory_usage_percent{instance=%q}", instanceIdentifier),
		startTime, endTime, step,
	)
	metrics.DiskUsage = s.queryRoundedLatestMetricValue(
		fmt.Sprintf("system_disk_usage_percent{instance=%q,mountpoint=%q}", instanceIdentifier, "/"),
		startTime, endTime, step,
	)
	metrics.Load5 = s.queryRoundedLatestMetricValue(
		fmt.Sprintf("system_load_average_5min{instance=%q}", instanceIdentifier),
		startTime, endTime, step,
	)
}

func (s *MonitorServiceImpl) populateWindowsSummaryMetrics(
	metrics *model.HostMetrics,
	jobIdentifier string,
	startTime, endTime time.Time,
	step string,
) {
	if metrics == nil {
		return
	}

	metrics.CPUUsage = s.queryRoundedLatestMetricValue(
		fmt.Sprintf(
			`clamp_min(clamp_max(100 - (avg(rate(windows_cpu_time_total{job=%q,mode="idle"}[5m])) * 100), 100), 0)`,
			jobIdentifier,
		),
		startTime, endTime, step,
	)
	metrics.MemoryUsage = s.queryRoundedLatestMetricValue(
		fmt.Sprintf(
			`clamp_min(clamp_max(100 * (1 - (windows_memory_physical_free_bytes{job=%q} / windows_memory_physical_total_bytes{job=%q})), 100), 0)`,
			jobIdentifier,
			jobIdentifier,
		),
		startTime, endTime, step,
	)
	metrics.DiskUsage = s.queryRoundedLatestMetricValue(
		fmt.Sprintf(
			`max(100 * (1 - (windows_logical_disk_free_bytes{job=%q,volume=~"[A-Z]:"} / windows_logical_disk_size_bytes{job=%q,volume=~"[A-Z]:"})))`,
			jobIdentifier,
			jobIdentifier,
		),
		startTime, endTime, step,
	)
	metrics.Load5 = 0
}

func (s *MonitorServiceImpl) checkHostOnline(host model.CmdbHost) bool {
	if host.SSHIP == "" || host.SSHPort == 0 {
		return false
	}

	isCloudVendor := host.Vendor >= 2 && host.Vendor <= 6

	if isCloudVendor {

		return host.Status == 1 || host.Status == 2
	}

	conn, err := net.DialTimeout("tcp",
		fmt.Sprintf("%s:%d", host.SSHIP, host.SSHPort),
		500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (s *MonitorServiceImpl) isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// 10.0.0.0/8
	// 172.16.0.0/12
	// 192.168.0.0/16
	// 100.64.0.0/10 (Carrier-grade NAT)
	privateIPBlocks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"100.64.0.0/10",
	}

	for _, block := range privateIPBlocks {
		_, subnet, _ := net.ParseCIDR(block)
		if subnet.Contains(ip) {
			return true
		}
	}

	return false
}

func (s *MonitorServiceImpl) getHostFromCache(hostID uint) (*model.CmdbHost, error) {
	cacheKey := fmt.Sprintf(hostCacheKey, hostID)

	cachedData, err := redis.RedisDb.Get(context.Background(), cacheKey).Result()
	if err != nil {

		return nil, err
	}

	var host model.CmdbHost
	if err := json.Unmarshal([]byte(cachedData), &host); err != nil {
		return nil, err
	}

	return &host, nil
}

func (s *MonitorServiceImpl) setHostToCache(hostID uint, host *model.CmdbHost) error {
	cacheKey := fmt.Sprintf(hostCacheKey, hostID)

	hostData, err := json.Marshal(host)
	if err != nil {
		return err
	}

	return redis.RedisDb.Set(context.Background(), cacheKey, hostData, s.cacheExpire).Err()
}

func (s *MonitorServiceImpl) getHostWithCache(hostID uint) (*model.CmdbHost, error) {

	if host, err := s.getHostFromCache(hostID); err == nil && host != nil {
		return host, nil
	}

	host, err := s.hostDao.GetCmdbHostById(hostID)
	if err != nil {
		return nil, err
	}

	go func() {
		_ = s.setHostToCache(hostID, &host)
	}()

	return &host, nil
}

func (s *MonitorServiceImpl) GetHostMetrics(c *gin.Context, hostID uint) {

	if hostID == 0 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid parameters")
		return
	}

	host, err := s.getHostWithCache(hostID)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "request failed")
		return
	}

	metricIdentifier := s.getMetricsQueryIdentifier(host)
	if metricIdentifier == "" {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "request failed")
		return
	}

	// ... existing code ...
	now := time.Now()
	startTime := now.Add(-5 * time.Minute)
	endTime := now
	step := "30s"

	metrics := s.querySummaryHostMetrics(hostID, host, startTime, endTime, step)

	result.Success(c, metrics)
}

func (s *MonitorServiceImpl) GetHostMetricsData(hostID uint, minutes int, step string) (*model.HostMetrics, error) {
	if hostID == 0 {
		return nil, fmt.Errorf("invalid host id")
	}

	host, err := s.getHostWithCache(hostID)
	if err != nil {
		return nil, err
	}

	metricIdentifier := s.getMetricsQueryIdentifier(host)
	if metricIdentifier == "" {
		return nil, fmt.Errorf("host has no metric identifier")
	}

	if minutes <= 0 {
		minutes = 5
	}
	if minutes > 24*60 {
		minutes = 24 * 60
	}
	if strings.TrimSpace(step) == "" {
		step = "30s"
	}

	now := time.Now()
	startTime := now.Add(-time.Duration(minutes) * time.Minute)
	return s.querySummaryHostMetrics(hostID, host, startTime, now, step), nil
}

func (s *MonitorServiceImpl) BatchGetHostMetrics(c *gin.Context, hostIDs []uint) {

	if len(hostIDs) == 0 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid parameters")
		return
	}

	maxConcurrent := 10
	if len(hostIDs) < maxConcurrent {
		maxConcurrent = len(hostIDs)
	}
	semaphore := make(chan struct{}, maxConcurrent)

	var wg sync.WaitGroup
	resultMap := make(map[uint]*model.HostMetrics, len(hostIDs))
	var mu sync.Mutex

	// Dutine
	for _, id := range hostIDs {
		wg.Add(1)
		go func(hostID uint) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			host, err := s.getHostWithCache(hostID)
			if err != nil {

				return
			}

			now := time.Now()
			startTime := now.Add(-3 * time.Minute)
			endTime := now
			step := "60s"

			metrics := s.querySummaryHostMetrics(hostID, host, startTime, endTime, step)

			// p
			mu.Lock()
			resultMap[hostID] = metrics
			mu.Unlock()
		}(id)
	}

	wg.Wait()

	result.Success(c, resultMap)
}

func (s *MonitorServiceImpl) queryPrometheus(query string, start, end time.Time, step string) (*model.PrometheusQueryResult, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	if s.prometheusURL == "" {
		return nil, fmt.Errorf("prometheus URL not configured")
	}

	var req *http.Request
	var q url.Values
	var err error

	if !start.IsZero() && !end.IsZero() {
		req, err = http.NewRequest("GET", strings.TrimSuffix(s.prometheusURL, "/")+"/api/v1/query_range", nil)
		if err != nil {
			return nil, err
		}

		q = req.URL.Query()
		q.Add("query", query)
		q.Add("start", strconv.FormatInt(start.Unix(), 10))
		q.Add("end", strconv.FormatInt(end.Unix(), 10))
		q.Add("step", step)
	} else {

		req, err = http.NewRequest("GET", s.prometheusURL+"/api/v1/query", nil)
		if err != nil {
			return nil, err
		}
		q = req.URL.Query()
		q.Set("query", query)
	}

	q.Add("nocache", "1")
	q.Add("time", fmt.Sprintf("%d", time.Now().Unix()))
	q.Add("_", fmt.Sprintf("%d", time.Now().UnixNano()))
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Expires", "0")
	req.Header.Set("X-Request-ID", fmt.Sprintf("%d", time.Now().UnixNano()))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var result model.PrometheusQueryResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse Prometheus response: %v\nRaw response: %s", err, string(body))
	}

	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus query failed: %s", string(body))
	}

	if result.Data.ResultType != "matrix" && result.Data.ResultType != "vector" {
		return nil, fmt.Errorf("unexpected result type: %s", result.Data.ResultType)
	}

	return &result, nil
}

func (s *MonitorServiceImpl) GetTopProcesses(c *gin.Context, hostID uint) {

	if hostID == 0 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid parameters")
		return
	}

	topResult, err := s.GetTopProcessesData(hostID)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "request failed")
		return
	}

	result.Success(c, topResult)
}

func (s *MonitorServiceImpl) GetTopProcessesData(hostID uint) (*model.TopProcessesResult, error) {
	if hostID == 0 {
		return nil, fmt.Errorf("invalid host id")
	}

	host, err := s.getHostWithCache(hostID)
	if err != nil {
		return nil, err
	}

	instanceIdentifier := s.getInstanceIdentifier(host)
	if instanceIdentifier == "" {
		return nil, fmt.Errorf("host has no metric identifier")
	}

	if !s.checkAgentOnline(hostID) {
		return &model.TopProcessesResult{
			HostID:     hostID,
			HostName:   instanceIdentifier,
			TopCPU:     []model.ProcessInfo{},
			TopMemory:  []model.ProcessInfo{},
			UpdateTime: time.Now().Unix(),
		}, nil
	}

	// ... existing code ...
	var wg sync.WaitGroup
	var topCPU, topMemory []model.ProcessInfo
	var mu sync.Mutex

	wg.Add(1)
	go func() {
		defer wg.Done()
		cpuQuery := fmt.Sprintf("topk(5, process_cpu_percent{host=\"%s\"})", instanceIdentifier)
		cpuResult, err := s.queryPrometheus(cpuQuery, time.Time{}, time.Time{}, "")
		if err == nil && len(cpuResult.Data.Result) > 0 {
			mu.Lock()
			for _, item := range cpuResult.Data.Result {
				if len(item.Value) >= 2 {

					var processInfo model.ProcessInfo
					processInfo.Host = item.Metric["host"]
					processInfo.Name = item.Metric["name"]

					// D
					if pidStr, ok := item.Metric["pid"]; ok {
						if pid, err := strconv.ParseUint(pidStr, 10, 32); err == nil {
							processInfo.PID = uint(pid)
						}
					}

					if valStr, ok := item.Value[1].(string); ok {
						if cpuVal, err := strconv.ParseFloat(valStr, 64); err == nil {
							processInfo.CPUPercent, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", cpuVal), 64)
						}
					}

					topCPU = append(topCPU, processInfo)
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		memQuery := fmt.Sprintf("topk(5, process_memory_percent{host=\"%s\"})", instanceIdentifier)
		memResult, err := s.queryPrometheus(memQuery, time.Time{}, time.Time{}, "")
		if err == nil && len(memResult.Data.Result) > 0 {
			mu.Lock()
			for _, item := range memResult.Data.Result {
				if len(item.Value) >= 2 {

					var processInfo model.ProcessInfo
					processInfo.Host = item.Metric["host"]
					processInfo.Name = item.Metric["name"]

					// D
					if pidStr, ok := item.Metric["pid"]; ok {
						if pid, err := strconv.ParseUint(pidStr, 10, 32); err == nil {
							processInfo.PID = uint(pid)
						}
					}

					if valStr, ok := item.Value[1].(string); ok {
						if memVal, err := strconv.ParseFloat(valStr, 64); err == nil {
							processInfo.MemPercent, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", memVal), 64)
						}
					}

					topMemory = append(topMemory, processInfo)
				}
			}
			mu.Unlock()
		}
	}()

	wg.Wait()

	topResult := &model.TopProcessesResult{
		HostID:     hostID,
		HostName:   instanceIdentifier,
		TopCPU:     topCPU,
		TopMemory:  topMemory,
		UpdateTime: time.Now().Unix(),
	}

	return topResult, nil
}

func (s *MonitorServiceImpl) GetHostAllMetricsHistory(c *gin.Context, hostID uint, startTime, endTime string, step string) {

	if hostID == 0 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid parameters")
		return
	}

	host, err := s.hostDao.GetCmdbHostById(hostID)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "request failed")
		return
	}

	instanceIdentifier := s.getInstanceIdentifier(&host)
	if instanceIdentifier == "" {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "request failed")
		return
	}

	loc, _ := time.LoadLocation("Asia/Shanghai")
	start, err := time.ParseInLocation("2006-01-02 15:04:05", startTime, loc)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid parameters")
		return
	}
	end, err := time.ParseInLocation("2006-01-02 15:04:05", endTime, loc)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid parameters")
		return
	}

	if step == "" {
		step = calculateStep(end.Sub(start))
	} else {
		step = strings.Trim(step, `"`)
	}

	if _, err := time.ParseDuration(step); err != nil {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError),
			"? 15s, 1m, 5m")
		return
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	allMetrics := &model.AllMetricsHistory{
		HostID: hostID,
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("rate(system_cpu_usage_percent{instance=\"%s\"}[1m])", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								allMetrics.CPU = append(allMetrics.CPU, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_memory_usage_percent{instance=\"%s\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								allMetrics.Memory = append(allMetrics.Memory, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_disk_usage_percent{instance=\"%s\",mountpoint=\"/\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								allMetrics.Disk = append(allMetrics.Disk, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_network_receive_kb_per_second{instance=\"%s\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								allMetrics.NetworkReceive = append(allMetrics.NetworkReceive, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_network_send_kb_per_second{instance=\"%s\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								allMetrics.NetworkSend = append(allMetrics.NetworkSend, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_load_average_1min{instance=\"%s\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								allMetrics.Load1min = append(allMetrics.Load1min, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_load_average_5min{instance=\"%s\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								allMetrics.Load5min = append(allMetrics.Load5min, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_load_average_15min{instance=\"%s\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								allMetrics.Load15min = append(allMetrics.Load15min, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_disk_read_kb_per_second{instance=\"%s\",device=\"vda\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								allMetrics.DiskReadKB = append(allMetrics.DiskReadKB, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_disk_write_kb_per_second{instance=\"%s\",device=\"vda\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								allMetrics.DiskWriteKB = append(allMetrics.DiskWriteKB, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_total_processes{instance=\"%s\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								allMetrics.TotalProcesses = append(allMetrics.TotalProcesses, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_cpu_user_percent{instance=\"%s\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								allMetrics.CPUUser = append(allMetrics.CPUUser, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_cpu_system_percent{instance=\"%s\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								allMetrics.CPUSystem = append(allMetrics.CPUSystem, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_memory_used_gb{instance=\"%s\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								allMetrics.MemoryUsed = append(allMetrics.MemoryUsed, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_memory_free_gb{instance=\"%s\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								allMetrics.MemoryFree = append(allMetrics.MemoryFree, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_disk_used_bytes{instance=\"%s\",mountpoint=\"/\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)

								allMetrics.DiskUsed = append(allMetrics.DiskUsed, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_disk_total_bytes{instance=\"%s\",mountpoint=\"/\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)

								allMetrics.DiskTotal = append(allMetrics.DiskTotal, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_tcp_connections{instance=\"%s\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								allMetrics.TCPConnections = append(allMetrics.TCPConnections, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_udp_connections{instance=\"%s\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								allMetrics.UDPConnections = append(allMetrics.UDPConnections, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		query := fmt.Sprintf("system_open_file_descriptors{instance=\"%s\"}", instanceIdentifier)
		promResult, err := s.queryPrometheus(query, start, end, step)
		if err == nil && len(promResult.Data.Result) > 0 {
			mu.Lock()
			for _, resultItem := range promResult.Data.Result {
				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								allMetrics.OpenFiles = append(allMetrics.OpenFiles, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			}
			mu.Unlock()
		}
	}()

	wg.Wait()

	result.Success(c, allMetrics)
}

func (s *MonitorServiceImpl) GetHostServiceMetricsHistory(c *gin.Context, hostID uint, serviceName, startTime, endTime, step string) {
	if hostID == 0 || strings.TrimSpace(serviceName) == "" {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid parameters")
		return
	}
	host, err := s.hostDao.GetCmdbHostById(hostID)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "request failed")
		return
	}
	instanceIdentifier := s.getInstanceIdentifier(&host)
	if instanceIdentifier == "" {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "host has no metric identifier")
		return
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	start, err := time.ParseInLocation("2006-01-02 15:04:05", startTime, loc)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid start time")
		return
	}
	end, err := time.ParseInLocation("2006-01-02 15:04:05", endTime, loc)
	if err != nil || !start.Before(end) {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid end time")
		return
	}
	if step == "" {
		step = calculateStep(end.Sub(start))
	}

	hostMatcher := strconv.Quote(instanceIdentifier)
	nameMatcher := strconv.Quote(strings.TrimSpace(serviceName))
	queries := map[string]string{
		"cpu":    fmt.Sprintf("service_cpu_usage_percent{instance=%s,service_name=%s}", hostMatcher, nameMatcher),
		"memory": fmt.Sprintf("service_memory_usage_percent{instance=%s,service_name=%s}", hostMatcher, nameMatcher),
	}
	response := &model.HostServiceMetricsHistory{
		HostID:      hostID,
		ServiceName: serviceName,
		CPU:         []model.MetricDataPoint{},
		Memory:      []model.MetricDataPoint{},
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	for key, query := range queries {
		key, query := key, query
		wg.Add(1)
		go func() {
			defer wg.Done()
			queryResult, queryErr := s.queryPrometheus(query, start, end, step)
			if queryErr != nil {
				return
			}
			points := prometheusMetricPoints(queryResult)
			mu.Lock()
			if key == "cpu" {
				response.CPU = points
			} else {
				response.Memory = points
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	result.Success(c, response)
}

func prometheusMetricPoints(queryResult *model.PrometheusQueryResult) []model.MetricDataPoint {
	points := make([]model.MetricDataPoint, 0)
	if queryResult == nil {
		return points
	}
	for _, resultItem := range queryResult.Data.Result {
		for _, value := range resultItem.Values {
			if len(value) < 2 {
				continue
			}
			timestamp, timestampOK := value[0].(float64)
			valueText, valueOK := value[1].(string)
			if !timestampOK || !valueOK {
				continue
			}
			metricValue, err := strconv.ParseFloat(valueText, 64)
			if err != nil {
				continue
			}
			points = append(points, model.MetricDataPoint{Timestamp: int64(timestamp), Value: metricValue})
		}
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Timestamp < points[j].Timestamp })
	return points
}

func calculateStep(duration time.Duration) string {
	step := duration / 60
	return fmt.Sprintf("%.0fs", step.Seconds())
}

func (s *MonitorServiceImpl) GetHostMetricHistory(c *gin.Context, hostID uint, metric string, startTime, endTime string, step string) {

	if hostID == 0 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid parameters")
		return
	}

	host, err := s.hostDao.GetCmdbHostById(hostID)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "request failed")
		return
	}

	instanceIdentifier := s.getInstanceIdentifier(&host)
	if instanceIdentifier == "" {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "request failed")
		return
	}

	switch metric {
	case "cpu", "memory", "disk":

	default:
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid parameters")
		return
	}

	loc, _ := time.LoadLocation("Asia/Shanghai")
	start, err := time.ParseInLocation("2006-01-02 15:04:05", startTime, loc)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid parameters")
		return
	}
	end, err := time.ParseInLocation("2006-01-02 15:04:05", endTime, loc)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid parameters")
		return
	}

	if step == "" {
		step = calculateStep(end.Sub(start))
	} else {
		step = strings.Trim(step, `"`)
	}

	if _, err := time.ParseDuration(step); err != nil {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError),
			"? 15s, 1m, 5m")
		return
	}

	var query string
	switch metric {
	case "cpu":
		query = fmt.Sprintf("rate(system_cpu_usage_percent{instance=\"%s\"}[1m])", instanceIdentifier)
	case "memory":
		query = fmt.Sprintf("system_memory_usage_percent{instance=\"%s\"}", instanceIdentifier)
	case "disk":
		query = fmt.Sprintf("system_disk_usage_percent{instance=\"%s\",mountpoint=\"/\"}", instanceIdentifier)
	}

	promResult, err := s.queryPrometheus(query, start, end, step)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED),
			fmt.Sprintf("Prometheus? %v", err))
		return
	}

	history := &model.HostMetricHistory{
		HostID: hostID,
		Metric: metric,
	}

	if len(promResult.Data.Result) > 0 {
		for _, resultItem := range promResult.Data.Result {
			switch promResult.Data.ResultType {
			case "matrix":

				for _, value := range resultItem.Values {
					if len(value) >= 2 {
						if timestamp, ok := value[0].(float64); ok {
							if valStr, ok := value[1].(string); ok {
								val, _ := strconv.ParseFloat(valStr, 64)
								history.TimeData = append(history.TimeData, model.MetricDataPoint{
									Timestamp: int64(timestamp),
									Value:     val,
								})
							}
						}
					}
				}
			case "vector":

				if len(resultItem.Value) >= 2 {
					if timestamp, ok := resultItem.Value[0].(float64); ok {
						if valStr, ok := resultItem.Value[1].(string); ok {
							val, _ := strconv.ParseFloat(valStr, 64)
							history.TimeData = append(history.TimeData, model.MetricDataPoint{
								Timestamp: int64(timestamp),
								Value:     val,
							})
						}
					}
				}
			default:

			}
		}
	}

	result.Success(c, history)
}

func (s *MonitorServiceImpl) GetHostPorts(c *gin.Context, hostID uint) {

	if hostID == 0 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid parameters")
		return
	}

	host, err := s.getHostWithCache(hostID)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "request failed")
		return
	}

	instanceIdentifier := s.getInstanceIdentifier(host)
	if instanceIdentifier == "" {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "request failed")
		return
	}

	if !s.checkAgentOnline(hostID) {
		result.Success(c, &model.HostPortsResult{
			HostID:     hostID,
			HostName:   instanceIdentifier,
			Ports:      []model.PortInfo{},
			Total:      0,
			UpdateTime: time.Now().Unix(),
		})
		return
	}

	cacheKey := fmt.Sprintf("monitor:ports:%d", hostID)
	if cachedData, err := redis.RedisDb.Get(context.Background(), cacheKey).Result(); err == nil {
		var cachedResult model.HostPortsResult
		if err := json.Unmarshal([]byte(cachedData), &cachedResult); err == nil {
			if time.Now().Unix()-cachedResult.UpdateTime < 300 {
				result.Success(c, &cachedResult)
				return
			}
		}
	}

	query := fmt.Sprintf("tcp_port_listening{instance=\"%s\"}", instanceIdentifier)
	now := time.Now()
	startTime := now.Add(-5 * time.Minute)
	endTime := now
	step := "30s"

	promResult, err := s.queryPrometheus(query, startTime, endTime, step)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED),
			fmt.Sprintf("Prometheus? %v", err))
		return
	}

	cpuQuery := fmt.Sprintf("process_cpu_usage_percent{instance=\"%s\"}", instanceIdentifier)
	memQuery := fmt.Sprintf("process_memory_usage_percent{instance=\"%s\"}", instanceIdentifier)

	var cpuResult, memResult *model.PrometheusQueryResult
	var cpuErr, memErr error
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		cpuResult, cpuErr = s.queryPrometheus(cpuQuery, startTime, endTime, step)
	}()
	go func() {
		defer wg.Done()
		memResult, memErr = s.queryPrometheus(memQuery, startTime, endTime, step)
	}()
	wg.Wait()

	cpuMap := make(map[string]float64)
	memMap := make(map[string]float64)
	metricKey := func(metric map[string]string) string {
		if pid := strings.TrimSpace(metric["pid"]); pid != "" {
			return "pid:" + pid
		}
		if name := strings.TrimSpace(metric["name"]); name != "" {
			return "name:" + name
		}
		if service := strings.TrimSpace(metric["service"]); service != "" {
			return "name:" + service
		}
		return ""
	}
	metricLastValue := func(values [][]interface{}, value []interface{}) float64 {
		if len(values) > 0 {
			lastPoint := values[len(values)-1]
			if len(lastPoint) >= 2 {
				if valStr, ok := lastPoint[1].(string); ok {
					val, _ := strconv.ParseFloat(valStr, 64)
					return val
				}
			}
		} else if len(value) >= 2 {
			if valStr, ok := value[1].(string); ok {
				val, _ := strconv.ParseFloat(valStr, 64)
				return val
			}
		}
		return 0
	}

	if cpuErr == nil && len(cpuResult.Data.Result) > 0 {
		for _, resultItem := range cpuResult.Data.Result {
			key := metricKey(resultItem.Metric)
			if key == "" {
				continue
			}
			cpuMap[key] = metricLastValue(resultItem.Values, resultItem.Value)
		}
	}

	if memErr == nil && len(memResult.Data.Result) > 0 {
		for _, resultItem := range memResult.Data.Result {
			key := metricKey(resultItem.Metric)
			if key == "" {
				continue
			}
			memMap[key] = metricLastValue(resultItem.Values, resultItem.Value)
		}
	}

	var ports []model.PortInfo

	if len(promResult.Data.Result) > 0 {
		for _, resultItem := range promResult.Data.Result {
			port := resultItem.Metric["port"]
			pid := resultItem.Metric["pid"]
			service := resultItem.Metric["service"]

			status := int(metricLastValue(resultItem.Values, resultItem.Value))

			metricMatchKey := metricKey(resultItem.Metric)
			cpuUsage := cpuMap[metricMatchKey]
			memUsage := memMap[metricMatchKey]

			ports = append(ports, model.PortInfo{
				Port:     port,
				PID:      pid,
				Service:  service,
				Status:   status,
				CPUUsage: cpuUsage,
				MemUsage: memUsage,
			})
		}
	}

	portResult := &model.HostPortsResult{
		HostID:     hostID,
		HostName:   instanceIdentifier,
		Ports:      ports,
		Total:      len(ports),
		UpdateTime: time.Now().Unix(),
	}

	if cacheData, err := json.Marshal(portResult); err == nil {
		redis.RedisDb.Set(context.Background(), cacheKey, cacheData, 5*time.Minute)
	}

	result.Success(c, portResult)
}

func (s *MonitorServiceImpl) GetTopUsageHosts(c *gin.Context, days, limit int) {

	if days <= 0 || days > 15 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid parameters")
		return
	}
	if limit <= 0 || limit > 20 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid parameters")
		return
	}

	loc, _ := time.LoadLocation("Asia/Shanghai")
	endTime := time.Now().In(loc)
	startTime := endTime.Add(-time.Duration(days) * 24 * time.Hour)

	step := "1h"

	var wg sync.WaitGroup
	var mu sync.Mutex

	var topCPU []model.HostUsageRank
	var topMemory []model.HostUsageRank
	var topDisk []model.HostUsageRank

	wg.Add(1)
	go func() {
		defer wg.Done()

		cpuQuery := "system_cpu_usage_percent"
		cpuResult, err := s.queryPrometheus(cpuQuery, startTime, endTime, step)
		if err != nil {
			fmt.Printf("CPU? %v, query: %s\n", err, cpuQuery)
			return
		}
		if cpuResult != nil && len(cpuResult.Data.Result) > 0 {
			ranks := s.parseTopUsageResult(cpuResult, "cpu")
			mu.Lock()
			topCPU = ranks
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		memQuery := "system_memory_usage_percent"
		memResult, err := s.queryPrometheus(memQuery, startTime, endTime, step)
		if err != nil {
			fmt.Printf("? %v, query: %s\n", err, memQuery)
			return
		}
		if memResult != nil && len(memResult.Data.Result) > 0 {
			ranks := s.parseTopUsageResult(memResult, "memory")
			mu.Lock()
			topMemory = ranks
			mu.Unlock()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		diskQuery := "system_disk_usage_percent{mountpoint=\"/\"}"
		diskResult, err := s.queryPrometheus(diskQuery, startTime, endTime, step)
		if err != nil {
			fmt.Printf("? %v, query: %s\n", err, diskQuery)
			return
		}
		if diskResult != nil && len(diskResult.Data.Result) > 0 {
			ranks := s.parseTopUsageResult(diskResult, "disk")
			mu.Lock()
			topDisk = ranks
			mu.Unlock()
		}
	}()

	wg.Wait()

	topCPU = sortAndLimitRanks(topCPU, limit)
	topMemory = sortAndLimitRanks(topMemory, limit)
	topDisk = sortAndLimitRanks(topDisk, limit)

	topUsageResult := &model.TopUsageHostsResult{
		TopCPU:    topCPU,
		TopMemory: topMemory,
		TopDisk:   topDisk,
		Period:    fmt.Sprintf("%d days", days),
		StartTime: startTime.Format("2006-01-02 15:04:05"),
		EndTime:   endTime.Format("2006-01-02 15:04:05"),
	}

	result.Success(c, topUsageResult)
}

func (s *MonitorServiceImpl) GetCurrentUsageRankings(c *gin.Context, limit int) {
	if limit <= 0 || limit > 10 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid parameters")
		return
	}

	allowedHostIDs, err := s.getCurrentUsageAllowedHostIDs(c)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), err.Error())
		return
	}

	rankings := &model.CurrentUsageRankingsResult{
		Limit:      limit,
		UpdateTime: time.Now().Unix(),
	}

	if allowedHostIDs != nil && len(allowedHostIDs) == 0 {
		result.Success(c, rankings)
		return
	}

	type rankingTask struct {
		key       string
		query     string
		ascending bool
	}

	tasks := []rankingTask{
		{key: "maxCpu", query: fmt.Sprintf("topk(%d, system_cpu_usage_percent)", limit)},
		{key: "minCpu", query: fmt.Sprintf("bottomk(%d, system_cpu_usage_percent)", limit), ascending: true},
		{key: "maxMemory", query: fmt.Sprintf("topk(%d, system_memory_usage_percent)", limit)},
		{key: "minMemory", query: fmt.Sprintf("bottomk(%d, system_memory_usage_percent)", limit), ascending: true},
		{key: "maxDisk", query: fmt.Sprintf("topk(%d, system_disk_usage_percent{mountpoint=\"/\"})", limit)},
		{key: "minDisk", query: fmt.Sprintf("bottomk(%d, system_disk_usage_percent{mountpoint=\"/\"})", limit), ascending: true},
	}
	if allowedHostIDs != nil {
		tasks = []rankingTask{
			{key: "maxCpu", query: "system_cpu_usage_percent"},
			{key: "minCpu", query: "system_cpu_usage_percent", ascending: true},
			{key: "maxMemory", query: "system_memory_usage_percent"},
			{key: "minMemory", query: "system_memory_usage_percent", ascending: true},
			{key: "maxDisk", query: "system_disk_usage_percent{mountpoint=\"/\"}"},
			{key: "minDisk", query: "system_disk_usage_percent{mountpoint=\"/\"}", ascending: true},
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0

	for _, task := range tasks {
		task := task
		wg.Add(1)
		go func() {
			defer wg.Done()

			ranks, err := s.queryCurrentUsageRanks(task.query, task.ascending, limit, allowedHostIDs)
			if err != nil {
				fmt.Printf("current usage ranking query failed, key=%s, query=%s, err=%v\n", task.key, task.query, err)
				return
			}

			mu.Lock()
			defer mu.Unlock()

			switch task.key {
			case "maxCpu":
				rankings.MaxCPU = ranks
			case "minCpu":
				rankings.MinCPU = ranks
			case "maxMemory":
				rankings.MaxMemory = ranks
			case "minMemory":
				rankings.MinMemory = ranks
			case "maxDisk":
				rankings.MaxDisk = ranks
			case "minDisk":
				rankings.MinDisk = ranks
			}

			successCount++
		}()
	}

	wg.Wait()

	if successCount == 0 {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "request failed")
		return
	}

	result.Success(c, rankings)
}

func (s *MonitorServiceImpl) getCurrentUsageAllowedHostIDs(c *gin.Context) (map[uint]struct{}, error) {
	userObj, exists := c.Get(constant.ContextKeyUserObj)
	if !exists {
		return nil, fmt.Errorf("未登录或登录已过期")
	}

	admin, ok := userObj.(*systemModel.JwtAdmin)
	if !ok || admin == nil {
		return nil, fmt.Errorf("无法获取用户信息")
	}

	if admin.ID == 1 || admin.Username == "admin" {
		return nil, nil
	}

	authDao := dao.NewBastionHostAuthorizationDao()
	hostIDs, err := authDao.GetUserAuthorizedHostIds(admin.ID)
	if err != nil {
		return nil, fmt.Errorf("获取授权信息失败: %w", err)
	}

	allowedHostIDs := make(map[uint]struct{}, len(hostIDs))
	for _, hostID := range hostIDs {
		allowedHostIDs[hostID] = struct{}{}
	}
	return allowedHostIDs, nil
}

func (s *MonitorServiceImpl) queryCurrentUsageRanks(query string, ascending bool, limit int, allowedHostIDs map[uint]struct{}) ([]model.HostCurrentUsageRank, error) {
	queryResult, err := s.queryPrometheus(query, time.Time{}, time.Time{}, "")
	if err != nil {
		return nil, err
	}

	rankMap := make(map[string]model.HostCurrentUsageRank)
	for _, item := range queryResult.Data.Result {
		instance := item.Metric["instance"]
		if instance == "" || len(item.Value) < 2 {
			continue
		}

		valueStr, ok := item.Value[1].(string)
		if !ok {
			continue
		}
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			continue
		}

		rank := model.HostCurrentUsageRank{
			HostName: instance,
			Instance: instance,
			IP:       instance,
			Value:    parseFloat(value),
		}

		host, err := s.hostDao.GetCmdbHostByInstanceIdentifier(instance)
		if allowedHostIDs != nil {
			if err != nil {
				continue
			}
			if _, ok := allowedHostIDs[host.ID]; !ok {
				continue
			}
		}

		if err == nil {
			rank.HostID = host.ID
			if host.HostName != "" {
				rank.HostName = host.HostName
			} else if host.Name != "" {
				rank.HostName = host.Name
			}

			if host.PrivateIP != "" {
				rank.IP = host.PrivateIP
			} else if host.SSHIP != "" {
				rank.IP = host.SSHIP
			} else if host.PublicIP != "" {
				rank.IP = host.PublicIP
			}
		}

		existing, exists := rankMap[instance]
		if !exists || (!ascending && rank.Value > existing.Value) || (ascending && rank.Value < existing.Value) {
			rankMap[instance] = rank
		}
	}

	ranks := make([]model.HostCurrentUsageRank, 0, len(rankMap))
	for _, rank := range rankMap {
		ranks = append(ranks, rank)
	}

	sort.Slice(ranks, func(i, j int) bool {
		if ascending {
			return ranks[i].Value < ranks[j].Value
		}
		return ranks[i].Value > ranks[j].Value
	})

	if len(ranks) > limit {
		ranks = ranks[:limit]
	}

	return ranks, nil
}

func (s *MonitorServiceImpl) parseTopUsageResult(queryResult *model.PrometheusQueryResult, metricType string) []model.HostUsageRank {

	hostMap := make(map[string]*model.HostUsageRank)

	for _, item := range queryResult.Data.Result {

		hostName, ok := item.Metric["instance"]
		if !ok {
			continue
		}

		var avgUsage, maxUsage, currentUsage float64
		var valueCount int
		var valueSum float64

		if len(item.Values) > 0 {
			for _, point := range item.Values {
				if len(point) >= 2 {
					if val, err := strconv.ParseFloat(point[1].(string), 64); err == nil {
						valueSum += val
						valueCount++
						if val > maxUsage {
							maxUsage = val
						}
					}
				}
			}

			if lastPoint := item.Values[len(item.Values)-1]; len(lastPoint) >= 2 {
				if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
					currentUsage = val
				}
			}
		} else if len(item.Value) >= 2 {

			if val, err := strconv.ParseFloat(item.Value[1].(string), 64); err == nil {
				avgUsage = val
				maxUsage = val
				currentUsage = val
				valueCount = 1
			}
		}

		if valueCount > 0 {
			avgUsage = valueSum / float64(valueCount)
		}

		if existingRank, exists := hostMap[hostName]; exists {

			if avgUsage > existingRank.AvgUsage {
				existingRank.AvgUsage = parseFloat(avgUsage)
			}
			if maxUsage > existingRank.MaxUsage {
				existingRank.MaxUsage = parseFloat(maxUsage)
			}
			if currentUsage > existingRank.CurrentUsage {
				existingRank.CurrentUsage = parseFloat(currentUsage)
			}
			continue
		}

		host, err := s.hostDao.GetCmdbHostByInstanceIdentifier(hostName)
		var hostID uint
		var ip string

		if err != nil {

			hostID = 0
			ip = hostName
		} else {
			hostID = host.ID
			ip = host.PrivateIP
			if ip == "" {
				ip = host.SSHIP
			}
			if ip == "" {
				ip = hostName
			}
		}

		rank := model.HostUsageRank{
			HostID:       hostID,
			HostName:     hostName,
			IP:           ip,
			AvgUsage:     parseFloat(avgUsage),
			MaxUsage:     parseFloat(maxUsage),
			CurrentUsage: parseFloat(currentUsage),
		}

		hostMap[hostName] = &rank
	}

	var ranks []model.HostUsageRank
	for _, rank := range hostMap {
		ranks = append(ranks, *rank)
	}

	return ranks
}

func sortAndLimitRanks(ranks []model.HostUsageRank, limit int) []model.HostUsageRank {
	if len(ranks) == 0 {
		return ranks
	}

	for i := 0; i < len(ranks)-1; i++ {
		for j := i + 1; j < len(ranks); j++ {
			if ranks[j].AvgUsage > ranks[i].AvgUsage {
				ranks[i], ranks[j] = ranks[j], ranks[i]
			}
		}
	}

	if len(ranks) > limit {
		return ranks[:limit]
	}

	return ranks
}

func parseFloat(value float64) float64 {
	result, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", value), 64)
	return result
}

func (s *MonitorServiceImpl) GetCompleteHostMetrics(c *gin.Context, hostID uint) {
	metrics, err := s.GetCompleteHostMetricsData(hostID)
	if err != nil {
		if strings.Contains(err.Error(), "invalid host id") {
			result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid parameters")
			return
		}
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "request failed")
		return
	}

	result.Success(c, metrics)
}

func (s *MonitorServiceImpl) GetCompleteHostMetricsData(hostID uint) (*model.CompleteHostMetrics, error) {
	if hostID == 0 {
		return nil, fmt.Errorf("invalid host id")
	}

	host, err := s.getHostWithCache(hostID)
	if err != nil {
		return nil, err
	}

	instanceIdentifier := s.getInstanceIdentifier(host)
	if instanceIdentifier == "" {
		return nil, fmt.Errorf("host has no metric identifier")
	}

	if !s.checkAgentOnline(hostID) {
		return &model.CompleteHostMetrics{
			HostID:       hostID,
			HostName:     host.Name,
			IP:           host.PrivateIP,
			Services:     []model.ServiceInfo{},
			UpdateTime:   time.Now().Unix(),
			ProcessCount: 0,
			OpenFiles:    0,
		}, nil
	}

	now := time.Now()
	startTime := now.Add(-5 * time.Minute)
	endTime := now
	step := "30s"

	metrics := &model.CompleteHostMetrics{
		HostID:     hostID,
		HostName:   host.Name,
		IP:         host.PrivateIP,
		UpdateTime: time.Now().Unix(),
	}

	// U
	metrics.CPU = s.queryCPUMetrics(instanceIdentifier, startTime, endTime, step)

	metrics.Memory = s.queryMemoryMetrics(instanceIdentifier, startTime, endTime, step)

	metrics.Disk = s.queryDiskMetrics(instanceIdentifier, startTime, endTime, step)

	metrics.Load = s.queryLoadMetrics(instanceIdentifier, startTime, endTime, step)

	metrics.Connections = s.queryConnectionMetrics(instanceIdentifier, startTime, endTime, step)

	metrics.Network = s.queryNetworkMetrics(instanceIdentifier, startTime, endTime, step)

	metrics.Uptime, metrics.UptimeStr = s.queryUptimeMetrics(instanceIdentifier, startTime, endTime, step)

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_total_processes{instance=\"%s\"}", instanceIdentifier),
		startTime, endTime, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				metrics.ProcessCount = int(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_open_file_descriptors{instance=\"%s\"}", instanceIdentifier),
		startTime, endTime, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				metrics.OpenFiles = int(val)
			}
		}
	}

	metrics.Services = s.queryServiceMetrics(instanceIdentifier, startTime, endTime, step)

	return metrics, nil
}

func (s *MonitorServiceImpl) queryCPUMetrics(instance string, start, end time.Time, step string) model.CPUMetrics {
	cpuMetrics := model.CPUMetrics{}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_cpu_usage_percent{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				cpuMetrics.Total = parseFloat(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_cpu_user_percent{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				cpuMetrics.User = parseFloat(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_cpu_system_percent{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				cpuMetrics.System = parseFloat(val)
			}
		}
	}

	return cpuMetrics
}

func (s *MonitorServiceImpl) queryMemoryMetrics(instance string, start, end time.Time, step string) model.MemoryMetrics {
	memMetrics := model.MemoryMetrics{}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_memory_usage_percent{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				memMetrics.UsageRate = parseFloat(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_memory_used_gb{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {

				memMetrics.Used = uint64(val * 1024 * 1024 * 1024)
				memMetrics.UsedStr = formatBytes(memMetrics.Used)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_memory_total_gb{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {

				memMetrics.Total = uint64(val * 1024 * 1024 * 1024)
				memMetrics.TotalStr = formatBytes(memMetrics.Total)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_memory_free_gb{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {

				memMetrics.Free = uint64(val * 1024 * 1024 * 1024)
				memMetrics.FreeStr = formatBytes(memMetrics.Free)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_swap_used_gb{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {

				memMetrics.SwapUsed = uint64(val * 1024 * 1024 * 1024)
				memMetrics.SwapUsedStr = formatBytes(memMetrics.SwapUsed)
			}
		}
	}

	return memMetrics
}

func (s *MonitorServiceImpl) queryDiskMetrics(instance string, start, end time.Time, step string) model.DiskMetrics {
	diskMetrics := model.DiskMetrics{}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_disk_usage_percent{instance=\"%s\",mountpoint=\"/\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				diskMetrics.UsageRate = parseFloat(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_disk_used_bytes{instance=\"%s\",mountpoint=\"/\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 {

		if len(data.Data.Result[0].Values) > 0 {
			lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
			if len(lastPoint) >= 2 {
				if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
					diskMetrics.Used = uint64(val)
					diskMetrics.UsedStr = formatBytes(uint64(val))
				}
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_disk_total_bytes{instance=\"%s\",mountpoint=\"/\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 {

		if len(data.Data.Result[0].Values) > 0 {
			lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
			if len(lastPoint) >= 2 {
				if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
					diskMetrics.Total = uint64(val)
					diskMetrics.TotalStr = formatBytes(uint64(val))
				}
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("sum(system_disk_read_kb_per_second{instance=\"%s\"})", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				diskMetrics.DiskThroughput.ReadKBPerSec = parseFloat(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("sum(system_disk_write_kb_per_second{instance=\"%s\"})", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				diskMetrics.DiskThroughput.WriteKBPerSec = parseFloat(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("sum(system_disk_read_iops{instance=\"%s\"})", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				diskMetrics.DiskIOPS.ReadPerSec = parseFloat(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("sum(system_disk_write_iops{instance=\"%s\"})", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				diskMetrics.DiskIOPS.WritePerSec = parseFloat(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_disk_inode_usage_percent{instance=\"%s\",mountpoint=\"/\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				diskMetrics.InodeUsage = parseFloat(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_disk_inode_total{instance=\"%s\",mountpoint=\"/\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				diskMetrics.InodeTotal = uint64(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_disk_inode_used{instance=\"%s\",mountpoint=\"/\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				diskMetrics.InodeUsed = uint64(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_disk_inode_free{instance=\"%s\",mountpoint=\"/\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				diskMetrics.InodeFree = uint64(val)
			}
		}
	}

	if diskMetrics.Total > 0 && diskMetrics.Used > 0 {
		diskMetrics.Free = diskMetrics.Total - diskMetrics.Used
		diskMetrics.FreeStr = formatBytes(diskMetrics.Free)
	}

	return diskMetrics
}

func (s *MonitorServiceImpl) queryLoadMetrics(instance string, start, end time.Time, step string) model.LoadMetrics {
	loadMetrics := model.LoadMetrics{}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_load_average_1min{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				loadMetrics.Load1 = parseFloat(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_load_average_5min{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				loadMetrics.Load5 = parseFloat(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_load_average_15min{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				loadMetrics.Load15 = parseFloat(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_load_ratio_1min{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				loadMetrics.LoadRatio1 = parseFloat(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_load_ratio_5min{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				loadMetrics.LoadRatio5 = parseFloat(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_load_ratio_15min{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				loadMetrics.LoadRatio15 = parseFloat(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_cpu_core_count{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				loadMetrics.CPUCores = int(val)
			}
		}
	}

	return loadMetrics
}

func (s *MonitorServiceImpl) queryConnectionMetrics(instance string, start, end time.Time, step string) model.ConnMetrics {
	connMetrics := model.ConnMetrics{}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_tcp_connections{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				connMetrics.TCP = int(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_udp_connections{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				connMetrics.UDP = int(val)
			}
		}
	}

	connMetrics.Total = connMetrics.TCP + connMetrics.UDP

	return connMetrics
}

func (s *MonitorServiceImpl) queryNetworkMetrics(instance string, start, end time.Time, step string) model.NetworkMetrics {
	netMetrics := model.NetworkMetrics{}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_network_receive_kb_per_second{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				netMetrics.ReceiveKBPerSec = parseFloat(val)
			}
		}
	}

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_network_send_kb_per_second{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				netMetrics.SendKBPerSec = parseFloat(val)
			}
		}
	}

	return netMetrics
}

func (s *MonitorServiceImpl) queryUptimeMetrics(instance string, start, end time.Time, step string) (float64, string) {
	var uptimeDays float64

	if data, err := s.queryPrometheus(
		fmt.Sprintf("system_uptime_days{instance=\"%s\"}", instance),
		start, end, step,
	); err == nil && len(data.Data.Result) > 0 && len(data.Data.Result[0].Values) > 0 {
		lastPoint := data.Data.Result[0].Values[len(data.Data.Result[0].Values)-1]
		if len(lastPoint) >= 2 {
			if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
				uptimeDays = val
			}
		}
	}

	return parseFloat(uptimeDays), formatUptimeFromDays(uptimeDays)
}

func (s *MonitorServiceImpl) queryServiceMetrics(instance string, start, end time.Time, step string) []model.ServiceInfo {
	services := []model.ServiceInfo{}

	cpuData, err := s.queryPrometheus(
		fmt.Sprintf("service_cpu_usage_percent{instance=\"%s\"}", instance),
		start, end, step,
	)
	if err != nil {
		return services
	}

	for _, result := range cpuData.Data.Result {
		serviceName := result.Metric["service_name"]
		if serviceName == "" {
			continue
		}

		var cpuUsage, memUsage float64

		if len(result.Values) > 0 {
			lastPoint := result.Values[len(result.Values)-1]
			if len(lastPoint) >= 2 {
				if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
					cpuUsage = parseFloat(val)
				}
			}
		}

		memData, err := s.queryPrometheus(
			fmt.Sprintf("service_memory_usage_percent{instance=\"%s\",service_name=\"%s\"}", instance, serviceName),
			start, end, step,
		)
		if err == nil && len(memData.Data.Result) > 0 && len(memData.Data.Result[0].Values) > 0 {
			lastPoint := memData.Data.Result[0].Values[len(memData.Data.Result[0].Values)-1]
			if len(lastPoint) >= 2 {
				if val, err := strconv.ParseFloat(lastPoint[1].(string), 64); err == nil {
					memUsage = parseFloat(val)
				}
			}
		}

		services = append(services, model.ServiceInfo{
			Name:   serviceName,
			CPU:    cpuUsage,
			Memory: memUsage,
		})
	}

	return services
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatUptime(seconds float64) string {
	days := int(seconds) / 86400
	hours := (int(seconds) % 86400) / 3600
	minutes := (int(seconds) % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%d days %d hours %d minutes", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%d hours %d minutes", hours, minutes)
	} else {
		return fmt.Sprintf("%d minutes", minutes)
	}
}

func formatUptimeFromDays(days float64) string {
	totalDays := int(days)
	hours := int((days - float64(totalDays)) * 24)
	minutes := int(((days-float64(totalDays))*24 - float64(hours)) * 60)

	if totalDays > 0 {
		return fmt.Sprintf("%d days %d hours %d minutes", totalDays, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%d hours %d minutes", hours, minutes)
	} else {
		return fmt.Sprintf("%d minutes", minutes)
	}
}
