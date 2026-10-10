package service

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	monitormodel "dodevops-api/api/cmdb/model"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

var defaultWindowsDashboardServices = []string{
	"windows_exporter",
	"WinRM",
	"Winmgmt",
	"EventLog",
	"W32Time",
	"wuauserv",
}

func (s *MonitorServiceImpl) GetWindowsDashboardMetrics(c *gin.Context, hostID uint, startTime, endTime string, step string) {
	if hostID == 0 {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid parameters")
		return
	}

	host, err := s.getHostWithCache(hostID)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "request failed")
		return
	}
	if !s.isWindowsHost(host) {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "host is not windows")
		return
	}

	jobIdentifier := s.getWindowsJobIdentifier(host)
	if jobIdentifier == "" {
		result.FailedWithCode(c, int(result.ApiCode.FAILED), "request failed")
		return
	}

	start, end, stepValue, err := normalizeMetricsQueryRange(startTime, endTime, step)
	if err != nil {
		result.FailedWithCode(c, int(result.ApiCode.ValidationParameterError), "invalid parameters")
		return
	}

	dashboard := &monitormodel.WindowsDashboardMetrics{
		HostID:             hostID,
		HostName:           chooseString(strings.TrimSpace(host.Name), strings.TrimSpace(host.HostName)),
		IP:                 chooseString(strings.TrimSpace(host.PrivateIP), strings.TrimSpace(host.SSHIP), strings.TrimSpace(host.PublicIP)),
		Hostname:           chooseString(strings.TrimSpace(host.Name), strings.TrimSpace(host.HostName)),
		OSProduct:          strings.TrimSpace(host.OS),
		OnlineStatus:       s.currentMetricsOnlineStatus(hostID, host),
		Disks:              []monitormodel.WindowsDiskVolume{},
		TopCPUProcesses:    []monitormodel.WindowsProcessItem{},
		TopMemoryProcesses: []monitormodel.WindowsProcessItem{},
		KeyServices:        []monitormodel.WindowsServiceStatus{},
		UpdateTime:         time.Now().Unix(),
	}

	if dashboard.OnlineStatus == 1 {
		result.Success(c, dashboard)
		return
	}

	if hostname := s.queryWindowsHostname(jobIdentifier); hostname != "" {
		dashboard.Hostname = hostname
	}
	if osProduct := s.queryWindowsOSProduct(jobIdentifier); osProduct != "" {
		dashboard.OSProduct = osProduct
	}

	dashboard.Overview = s.queryWindowsDashboardOverview(jobIdentifier)
	dashboard.Trends = s.queryWindowsDashboardTrends(jobIdentifier, start, end, stepValue)
	dashboard.Disks = s.queryWindowsDiskVolumes(jobIdentifier)
	dashboard.TopCPUProcesses = s.queryWindowsTopCPUProcesses(jobIdentifier)
	dashboard.TopMemoryProcesses = s.queryWindowsTopMemoryProcesses(jobIdentifier)
	dashboard.KeyServices = s.queryWindowsKeyServices(jobIdentifier)

	result.Success(c, dashboard)
}

func normalizeMetricsQueryRange(startTime, endTime, step string) (time.Time, time.Time, string, error) {
	loc, _ := time.LoadLocation("Asia/Shanghai")

	start, err := time.ParseInLocation("2006-01-02 15:04:05", startTime, loc)
	if err != nil {
		return time.Time{}, time.Time{}, "", err
	}
	end, err := time.ParseInLocation("2006-01-02 15:04:05", endTime, loc)
	if err != nil {
		return time.Time{}, time.Time{}, "", err
	}

	if step == "" {
		step = calculateStep(end.Sub(start))
	} else {
		step = strings.Trim(step, `"`)
	}

	if _, err := time.ParseDuration(step); err != nil {
		return time.Time{}, time.Time{}, "", err
	}

	return start, end, step, nil
}

func (s *MonitorServiceImpl) queryMetricSeries(query string, start, end time.Time, step string) []monitormodel.MetricDataPoint {
	points := []monitormodel.MetricDataPoint{}

	data, err := s.queryPrometheus(query, start, end, step)
	if err != nil || len(data.Data.Result) == 0 {
		return points
	}

	for _, value := range data.Data.Result[0].Values {
		if len(value) < 2 {
			continue
		}

		timestamp, ok := value[0].(float64)
		if !ok {
			continue
		}

		metricValue, ok := parsePrometheusValue(value[1])
		if !ok {
			continue
		}

		points = append(points, monitormodel.MetricDataPoint{
			Timestamp: int64(timestamp),
			Value:     parseFloat(metricValue),
		})
	}

	return points
}

func (s *MonitorServiceImpl) queryCurrentMetricValue(query string) (float64, bool) {
	return s.queryLatestMetricValue(query, time.Time{}, time.Time{}, "")
}

func (s *MonitorServiceImpl) queryCurrentMetricFloat(query string) float64 {
	return s.queryRoundedLatestMetricValue(query, time.Time{}, time.Time{}, "")
}

func (s *MonitorServiceImpl) queryCurrentMetricInt(query string) int {
	value, ok := s.queryCurrentMetricValue(query)
	if !ok {
		return 0
	}
	return int(value)
}

func (s *MonitorServiceImpl) queryMetricLabels(query string) map[string]string {
	data, err := s.queryPrometheus(query, time.Time{}, time.Time{}, "")
	if err != nil || len(data.Data.Result) == 0 {
		return nil
	}
	return data.Data.Result[0].Metric
}

func (s *MonitorServiceImpl) queryWindowsHostname(jobIdentifier string) string {
	labels := s.queryMetricLabels(fmt.Sprintf(`windows_os_hostname{job=%q}`, jobIdentifier))
	if len(labels) == 0 {
		return ""
	}
	return chooseString(strings.TrimSpace(labels["fqdn"]), strings.TrimSpace(labels["hostname"]))
}

func (s *MonitorServiceImpl) queryWindowsOSProduct(jobIdentifier string) string {
	labels := s.queryMetricLabels(fmt.Sprintf(`windows_os_info{job=%q}`, jobIdentifier))
	if len(labels) == 0 {
		return ""
	}
	return strings.TrimSpace(labels["product"])
}

func (s *MonitorServiceImpl) queryWindowsDashboardOverview(jobIdentifier string) monitormodel.WindowsDashboardOverview {
	overview := monitormodel.WindowsDashboardOverview{}

	overview.CPUUsage = s.queryCurrentMetricFloat(
		fmt.Sprintf(
			`clamp_min(clamp_max(100 - (avg(rate(windows_cpu_time_total{job=%q,mode="idle"}[5m])) * 100), 100), 0)`,
			jobIdentifier,
		),
	)
	overview.CPUCores = s.queryCurrentMetricInt(
		fmt.Sprintf(`max(windows_cpu_logical_processor{job=%q})`, jobIdentifier),
	)
	overview.CPUQueueLength = s.queryCurrentMetricFloat(
		fmt.Sprintf(`windows_system_processor_queue_length{job=%q}`, jobIdentifier),
	)
	overview.MemoryUsage = s.queryCurrentMetricFloat(
		fmt.Sprintf(
			`clamp_min(clamp_max(100 * (1 - (windows_memory_physical_free_bytes{job=%q} / windows_memory_physical_total_bytes{job=%q})), 100), 0)`,
			jobIdentifier,
			jobIdentifier,
		),
	)

	memoryTotal, _ := s.queryCurrentMetricValue(
		fmt.Sprintf(`windows_memory_physical_total_bytes{job=%q}`, jobIdentifier),
	)
	memoryFree, _ := s.queryCurrentMetricValue(
		fmt.Sprintf(`windows_memory_physical_free_bytes{job=%q}`, jobIdentifier),
	)
	if memoryTotal > 0 {
		overview.MemoryTotal = uint64(memoryTotal)
		overview.MemoryTotalStr = formatBytes(overview.MemoryTotal)
	}
	if memoryTotal >= memoryFree && memoryFree >= 0 {
		overview.MemoryUsed = uint64(memoryTotal - memoryFree)
		overview.MemoryUsedStr = formatBytes(overview.MemoryUsed)
	}

	overview.DiskUsage = s.queryCurrentMetricFloat(
		fmt.Sprintf(
			`clamp_min(clamp_max(100 * (1 - (sum(windows_logical_disk_free_bytes{job=%q,volume=~"[A-Z]:"}) / sum(windows_logical_disk_size_bytes{job=%q,volume=~"[A-Z]:"}))), 100), 0)`,
			jobIdentifier,
			jobIdentifier,
		),
	)

	diskTotal, _ := s.queryCurrentMetricValue(
		fmt.Sprintf(`sum(windows_logical_disk_size_bytes{job=%q,volume=~"[A-Z]:"})`, jobIdentifier),
	)
	diskFree, _ := s.queryCurrentMetricValue(
		fmt.Sprintf(`sum(windows_logical_disk_free_bytes{job=%q,volume=~"[A-Z]:"})`, jobIdentifier),
	)
	if diskTotal > 0 {
		overview.DiskTotal = uint64(diskTotal)
		overview.DiskTotalStr = formatBytes(overview.DiskTotal)
	}
	if diskTotal >= diskFree && diskFree >= 0 {
		overview.DiskUsed = uint64(diskTotal - diskFree)
		overview.DiskUsedStr = formatBytes(overview.DiskUsed)
	}

	overview.NetworkReceiveKBPerSec = s.queryCurrentMetricFloat(
		fmt.Sprintf(
			`sum(rate(windows_net_bytes_received_total{job=%q,nic!="Software Loopback Interface 1"}[5m])) / 1024`,
			jobIdentifier,
		),
	)
	overview.NetworkSendKBPerSec = s.queryCurrentMetricFloat(
		fmt.Sprintf(
			`sum(rate(windows_net_bytes_sent_total{job=%q,nic!="Software Loopback Interface 1"}[5m])) / 1024`,
			jobIdentifier,
		),
	)

	overview.ProcessCount = s.queryCurrentMetricInt(
		fmt.Sprintf(`count(windows_process_info{job=%q})`, jobIdentifier),
	)
	overview.ServiceCount = s.queryCurrentMetricInt(
		fmt.Sprintf(`count(windows_service_info{job=%q})`, jobIdentifier),
	)
	overview.RunningServiceCount = s.queryCurrentMetricInt(
		fmt.Sprintf(`sum(windows_service_state{job=%q,state="running"})`, jobIdentifier),
	)

	uptimeSeconds, ok := s.queryCurrentMetricValue(
		fmt.Sprintf(`time() - windows_system_boot_time_timestamp{job=%q}`, jobIdentifier),
	)
	if ok {
		overview.Uptime = parseFloat(uptimeSeconds)
		overview.UptimeStr = formatUptime(uptimeSeconds)
	}

	return overview
}

func (s *MonitorServiceImpl) queryWindowsDashboardTrends(jobIdentifier string, start, end time.Time, step string) monitormodel.WindowsDashboardTrends {
	return monitormodel.WindowsDashboardTrends{
		CPU: s.queryMetricSeries(
			fmt.Sprintf(
				`clamp_min(clamp_max(100 - (avg(rate(windows_cpu_time_total{job=%q,mode="idle"}[5m])) * 100), 100), 0)`,
				jobIdentifier,
			),
			start, end, step,
		),
		CPUQueueLength: s.queryMetricSeries(
			fmt.Sprintf(`windows_system_processor_queue_length{job=%q}`, jobIdentifier),
			start, end, step,
		),
		MemoryUsage: s.queryMetricSeries(
			fmt.Sprintf(
				`clamp_min(clamp_max(100 * (1 - (windows_memory_physical_free_bytes{job=%q} / windows_memory_physical_total_bytes{job=%q})), 100), 0)`,
				jobIdentifier,
				jobIdentifier,
			),
			start, end, step,
		),
		MemoryUsedGB: s.queryMetricSeries(
			fmt.Sprintf(
				`(windows_memory_physical_total_bytes{job=%q} - windows_memory_physical_free_bytes{job=%q}) / 1024 / 1024 / 1024`,
				jobIdentifier,
				jobIdentifier,
			),
			start, end, step,
		),
		DiskUsage: s.queryMetricSeries(
			fmt.Sprintf(
				`clamp_min(clamp_max(100 * (1 - (sum(windows_logical_disk_free_bytes{job=%q,volume=~"[A-Z]:"}) / sum(windows_logical_disk_size_bytes{job=%q,volume=~"[A-Z]:"}))), 100), 0)`,
				jobIdentifier,
				jobIdentifier,
			),
			start, end, step,
		),
		DiskReadKB: s.queryMetricSeries(
			fmt.Sprintf(
				`sum(rate(windows_logical_disk_read_bytes_total{job=%q,volume=~"[A-Z]:"}[5m])) / 1024`,
				jobIdentifier,
			),
			start, end, step,
		),
		DiskWriteKB: s.queryMetricSeries(
			fmt.Sprintf(
				`sum(rate(windows_logical_disk_write_bytes_total{job=%q,volume=~"[A-Z]:"}[5m])) / 1024`,
				jobIdentifier,
			),
			start, end, step,
		),
		NetworkReceive: s.queryMetricSeries(
			fmt.Sprintf(
				`sum(rate(windows_net_bytes_received_total{job=%q,nic!="Software Loopback Interface 1"}[5m])) / 1024`,
				jobIdentifier,
			),
			start, end, step,
		),
		NetworkSend: s.queryMetricSeries(
			fmt.Sprintf(
				`sum(rate(windows_net_bytes_sent_total{job=%q,nic!="Software Loopback Interface 1"}[5m])) / 1024`,
				jobIdentifier,
			),
			start, end, step,
		),
		ProcessCount: s.queryMetricSeries(
			fmt.Sprintf(`count(windows_process_info{job=%q})`, jobIdentifier),
			start, end, step,
		),
	}
}

func (s *MonitorServiceImpl) queryWindowsDiskVolumes(jobIdentifier string) []monitormodel.WindowsDiskVolume {
	volumes := make(map[string]*monitormodel.WindowsDiskVolume)
	ensureVolume := func(name string) *monitormodel.WindowsDiskVolume {
		volume, ok := volumes[name]
		if ok {
			return volume
		}
		volume = &monitormodel.WindowsDiskVolume{Volume: name}
		volumes[name] = volume
		return volume
	}

	if infoData, err := s.queryPrometheus(
		fmt.Sprintf(`windows_logical_disk_info{job=%q,volume=~"[A-Z]:"}`, jobIdentifier),
		time.Time{}, time.Time{}, "",
	); err == nil {
		for _, item := range infoData.Data.Result {
			volumeName := item.Metric["volume"]
			if volumeName == "" {
				continue
			}
			volume := ensureVolume(volumeName)
			volume.VolumeName = strings.TrimSpace(item.Metric["volume_name"])
			volume.FileSystem = strings.TrimSpace(item.Metric["filesystem"])
			volume.Type = strings.TrimSpace(item.Metric["type"])
		}
	}

	if sizeData, err := s.queryPrometheus(
		fmt.Sprintf(`windows_logical_disk_size_bytes{job=%q,volume=~"[A-Z]:"}`, jobIdentifier),
		time.Time{}, time.Time{}, "",
	); err == nil {
		for _, item := range sizeData.Data.Result {
			volumeName := item.Metric["volume"]
			if volumeName == "" || len(item.Value) < 2 {
				continue
			}
			value, ok := parsePrometheusValue(item.Value[1])
			if !ok {
				continue
			}
			volume := ensureVolume(volumeName)
			volume.Total = uint64(value)
			volume.TotalStr = formatBytes(volume.Total)
		}
	}

	if freeData, err := s.queryPrometheus(
		fmt.Sprintf(`windows_logical_disk_free_bytes{job=%q,volume=~"[A-Z]:"}`, jobIdentifier),
		time.Time{}, time.Time{}, "",
	); err == nil {
		for _, item := range freeData.Data.Result {
			volumeName := item.Metric["volume"]
			if volumeName == "" || len(item.Value) < 2 {
				continue
			}
			value, ok := parsePrometheusValue(item.Value[1])
			if !ok {
				continue
			}
			volume := ensureVolume(volumeName)
			volume.Free = uint64(value)
			volume.FreeStr = formatBytes(volume.Free)
		}
	}

	if readData, err := s.queryPrometheus(
		fmt.Sprintf(`rate(windows_logical_disk_read_bytes_total{job=%q,volume=~"[A-Z]:"}[5m]) / 1024`, jobIdentifier),
		time.Time{}, time.Time{}, "",
	); err == nil {
		for _, item := range readData.Data.Result {
			volumeName := item.Metric["volume"]
			if volumeName == "" || len(item.Value) < 2 {
				continue
			}
			value, ok := parsePrometheusValue(item.Value[1])
			if !ok {
				continue
			}
			ensureVolume(volumeName).ReadKBPerSec = parseFloat(value)
		}
	}

	if writeData, err := s.queryPrometheus(
		fmt.Sprintf(`rate(windows_logical_disk_write_bytes_total{job=%q,volume=~"[A-Z]:"}[5m]) / 1024`, jobIdentifier),
		time.Time{}, time.Time{}, "",
	); err == nil {
		for _, item := range writeData.Data.Result {
			volumeName := item.Metric["volume"]
			if volumeName == "" || len(item.Value) < 2 {
				continue
			}
			value, ok := parsePrometheusValue(item.Value[1])
			if !ok {
				continue
			}
			ensureVolume(volumeName).WriteKBPerSec = parseFloat(value)
		}
	}

	list := make([]monitormodel.WindowsDiskVolume, 0, len(volumes))
	for _, volume := range volumes {
		if volume.Total > 0 {
			if volume.Total >= volume.Free {
				volume.Used = volume.Total - volume.Free
				volume.UsedStr = formatBytes(volume.Used)
			}
			volume.UsageRate = parseFloat(float64(volume.Used) * 100 / float64(volume.Total))
		}
		list = append(list, *volume)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Volume < list[j].Volume
	})

	return list
}

func (s *MonitorServiceImpl) queryWindowsTopCPUProcesses(jobIdentifier string) []monitormodel.WindowsProcessItem {
	processes := []monitormodel.WindowsProcessItem{}

	data, err := s.queryPrometheus(
		fmt.Sprintf(
			`topk(5, clamp_min(clamp_max(100 * sum by (process, process_id) (rate(windows_process_cpu_time_total{job=%q,process!="Idle"}[5m])) / scalar(max(windows_cpu_logical_processor{job=%q})), 100), 0))`,
			jobIdentifier,
			jobIdentifier,
		),
		time.Time{}, time.Time{}, "",
	)
	if err != nil {
		return processes
	}

	for _, item := range data.Data.Result {
		if len(item.Value) < 2 {
			continue
		}

		value, ok := parsePrometheusValue(item.Value[1])
		if !ok {
			continue
		}

		process := monitormodel.WindowsProcessItem{
			Name:     item.Metric["process"],
			CPUUsage: parseFloat(value),
		}
		if pid, err := strconv.ParseUint(item.Metric["process_id"], 10, 64); err == nil {
			process.PID = uint(pid)
		}

		processes = append(processes, process)
	}

	return processes
}

func (s *MonitorServiceImpl) queryWindowsTopMemoryProcesses(jobIdentifier string) []monitormodel.WindowsProcessItem {
	processes := []monitormodel.WindowsProcessItem{}

	data, err := s.queryPrometheus(
		fmt.Sprintf(`topk(5, windows_process_working_set_bytes{job=%q,process!="Idle"})`, jobIdentifier),
		time.Time{}, time.Time{}, "",
	)
	if err != nil {
		return processes
	}

	for _, item := range data.Data.Result {
		if len(item.Value) < 2 {
			continue
		}

		value, ok := parsePrometheusValue(item.Value[1])
		if !ok {
			continue
		}

		process := monitormodel.WindowsProcessItem{
			Name:        item.Metric["process"],
			MemoryBytes: uint64(value),
			MemoryStr:   formatBytes(uint64(value)),
		}
		if pid, err := strconv.ParseUint(item.Metric["process_id"], 10, 64); err == nil {
			process.PID = uint(pid)
		}

		processes = append(processes, process)
	}

	return processes
}

func (s *MonitorServiceImpl) queryWindowsKeyServices(jobIdentifier string) []monitormodel.WindowsServiceStatus {
	services := make([]monitormodel.WindowsServiceStatus, 0, len(defaultWindowsDashboardServices))

	for _, name := range defaultWindowsDashboardServices {
		service := monitormodel.WindowsServiceStatus{
			Name:        name,
			DisplayName: name,
			Status:      "stopped",
		}

		if labels := s.queryMetricLabels(fmt.Sprintf(`windows_service_info{job=%q,name=%q}`, jobIdentifier, name)); len(labels) > 0 {
			service.DisplayName = chooseString(strings.TrimSpace(labels["display_name"]), name)
		}

		if value, ok := s.queryCurrentMetricValue(
			fmt.Sprintf(`windows_service_state{job=%q,name=%q,state="running"}`, jobIdentifier, name),
		); ok && value > 0 {
			service.Running = true
			service.Status = "running"
		}

		services = append(services, service)
	}

	return services
}
