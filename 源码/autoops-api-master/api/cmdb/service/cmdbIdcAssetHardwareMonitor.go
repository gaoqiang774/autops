package service

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	cmdbDao "dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/model"
	configDao "dodevops-api/api/configcenter/dao"
	configModel "dodevops-api/api/configcenter/model"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"
	"dodevops-api/common/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CmdbIdcAssetHardwareMonitorService struct {
	assetDao cmdbDao.CmdbIdcAssetDao
	vmDao    cmdbDao.CmdbIdcVmDao
}

type prometheusMetricResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

type prometheusAPIResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []prometheusMetricResult `json:"result"`
	} `json:"data"`
	Error string `json:"error"`
}

type prometheusAccount struct {
	BaseURL  string
	Username string
	Password string
}

type vendorMetricSpec struct {
	vendor         string
	rawVendor      string
	requiredJob    string
	discovery      []string
	overallMetric  string
	cpuMetric      string
	memoryMetric   string
	diskMetric     string
	fanMetric      string
	powerMetric    string
	tempMetric     string
	cpuUsageMetric string
	memUsageMetric string
	powerWatts     string
}

type targetMatch struct {
	vendor       vendorMetricSpec
	label        string
	target       string
	job          string
	matchedValue float64
}

var hardwareVendorSpecs = []vendorMetricSpec{
	{
		vendor:         "huawei",
		rawVendor:      "Huawei/xFusion",
		discovery:      []string{"cpuEntireStatus", "memoryEntireStatus", "fanEntireStatus", "powerSupplyEntireStatus"},
		overallMetric:  "temperatureEntireStatus",
		cpuMetric:      "cpuEntireStatus",
		memoryMetric:   "memoryEntireStatus",
		diskMetric:     "hardDiskEntireStatus",
		fanMetric:      "fanEntireStatus",
		powerMetric:    "powerSupplyEntireStatus",
		tempMetric:     "temperatureEntireStatus",
		cpuUsageMetric: "systemCpuUsage",
		memUsageMetric: "systemMemUsage",
		powerWatts:     "presentSystemPower",
	},
	{
		vendor:        "dell",
		rawVendor:     "Dell",
		discovery:     []string{"chassisStatus", "systemStatePowerSupplyStatusCombined", "systemStateCoolingDeviceStatusCombined", "systemStateTemperatureStatusCombined"},
		overallMetric: "chassisStatus",
		cpuMetric:     "systemStateProcessorDeviceStatusCombined",
		memoryMetric:  "systemStateMemoryDeviceStatusCombined",
		diskMetric:    "controllerComponentStatus",
		fanMetric:     "systemStateCoolingDeviceStatusCombined",
		powerMetric:   "systemStatePowerSupplyStatusCombined",
		tempMetric:    "systemStateTemperatureStatusCombined",
		powerWatts:    "amperageProbeReading",
	},
	{
		vendor:      "ipmi",
		rawVendor:   "IPMI exporter",
		requiredJob: "ipmi",
		discovery:   []string{"ipmi_up"},
	},
}

func NewCmdbIdcAssetHardwareMonitorService() *CmdbIdcAssetHardwareMonitorService {
	return &CmdbIdcAssetHardwareMonitorService{
		assetDao: cmdbDao.NewCmdbIdcAssetDao(),
		vmDao:    cmdbDao.NewCmdbIdcVmDao(),
	}
}

func (s *CmdbIdcAssetHardwareMonitorService) GetAssetHardwareMonitorBySelector(c *gin.Context, assetID uint, remoteManagementCard string, lookbackMinutes int, trendHours int, trendStepMinutes int) {
	remoteManagementCard = strings.TrimSpace(remoteManagementCard)
	if remoteManagementCard != "" {
		asset, err := s.lookupAssetByRemoteManagementCard(remoteManagementCard)
		if err != nil {
			result.Failed(c, constant.IDC_ASSET_MONITOR_FAILED, err.Error())
			return
		}
		if asset == nil {
			result.Failed(c, constant.IDC_ASSET_NOT_FOUND, "asset not found")
			return
		}
		assetID = asset.ID
	}
	if assetID == 0 {
		result.Failed(c, constant.INVALID_PARAMS, "id or remoteManagementCard is required")
		return
	}

	s.GetAssetHardwareMonitor(c, assetID, lookbackMinutes, trendHours, trendStepMinutes)
}

func (s *CmdbIdcAssetHardwareMonitorService) GetAssetHardwareMonitor(c *gin.Context, assetID uint, lookbackMinutes int, trendHours int, trendStepMinutes int) {
	if assetID == 0 {
		result.Failed(c, constant.INVALID_PARAMS, "资产ID不能为空")
		return
	}
	monitor, err := s.BuildAssetHardwareMonitor(assetID, lookbackMinutes, trendHours, trendStepMinutes)
	if err != nil {
		result.Failed(c, constant.IDC_ASSET_MONITOR_FAILED, err.Error())
		return
	}
	result.Success(c, monitor)
}

func (s *CmdbIdcAssetHardwareMonitorService) BuildAssetHardwareMonitor(assetID uint, lookbackMinutes int, trendHours int, trendStepMinutes int) (*model.CmdbIdcAssetHardwareMonitorVo, error) {
	if assetID == 0 {
		return nil, fmt.Errorf("资产ID不能为空")
	}
	if lookbackMinutes <= 0 {
		lookbackMinutes = 5
	}
	if lookbackMinutes > 15 {
		lookbackMinutes = 15
	}

	asset, err := s.assetDao.GetAssetById(assetID)
	if err != nil {
		return nil, fmt.Errorf("资产不存在")
	}
	if strings.TrimSpace(asset.RemoteManagementCard) == "" {
		return nil, fmt.Errorf("资产未配置远程管理卡IP")
	}

	vmCount, err := s.vmDao.CountByPhysicalAssetID(asset.ID)
	if err != nil {
		return nil, err
	}

	monitor := model.CmdbIdcAssetHardwareMonitorVo{
		Asset:            s.buildAssetVO(asset, asset.Room.Name, asset.Cabinet.Name, vmCount),
		MetricsAvailable: false,
		Summary: model.CmdbIdcAssetHardwareSummaryVo{
			OverallStatus:     "unknown",
			CPUStatus:         "unknown",
			MemoryStatus:      "unknown",
			DiskStatus:        "unknown",
			FanStatus:         "unknown",
			PowerStatus:       "unknown",
			TemperatureStatus: "unknown",
		},
		Components: model.CmdbIdcAssetHardwareComponentsVo{
			Temperatures:    []model.CmdbIdcAssetHardwareComponentVo{},
			Fans:            []model.CmdbIdcAssetHardwareComponentVo{},
			PowerSupplies:   []model.CmdbIdcAssetHardwareComponentVo{},
			Voltages:        []model.CmdbIdcAssetHardwareComponentVo{},
			Currents:        []model.CmdbIdcAssetHardwareComponentVo{},
			Sensors:         []model.CmdbIdcAssetHardwareComponentVo{},
			Disks:           []model.CmdbIdcAssetHardwareComponentVo{},
			RaidControllers: []model.CmdbIdcAssetHardwareComponentVo{},
		},
		Trends: model.CmdbIdcAssetHardwareTrendsVo{
			Panels: []model.CmdbIdcAssetHardwareTrendPanelVo{},
		},
		Alerts: []model.CmdbIdcAssetHardwareAlertVo{},
		Health: model.CmdbIdcAssetHardwareHealthVo{
			Score:           0,
			Level:           "unknown",
			Conclusion:      "暂无可用硬件指标",
			RiskCategories:  []string{},
			Recommendations: []string{},
		},
	}

	account, err := s.resolvePrometheusAccount()
	if err != nil {
		return nil, err
	}

	candidates := normalizeManagementTargets(asset.RemoteManagementCard)
	match, err := s.detectTargetMatch(account, candidates, lookbackMinutes)
	if err != nil {
		return nil, err
	}
	if match == nil {
		return &monitor, nil
	}

	monitor.MetricsAvailable = true
	monitor.Vendor = match.vendor.vendor
	monitor.RawVendor = match.vendor.rawVendor
	monitor.Source = model.CmdbIdcAssetHardwareSourceVo{
		Job:              match.job,
		MatchedLabel:     match.label,
		PrometheusTarget: match.target,
	}
	monitor.SampledAt = util.HTime{Time: time.Now()}

	if err := s.fillSummary(account, match, lookbackMinutes, &monitor); err != nil {
		return nil, err
	}
	if err := s.fillComponents(account, match, lookbackMinutes, &monitor); err != nil {
		return nil, err
	}
	if err := s.fillTrendsAndAlerts(account, match, trendHours, trendStepMinutes, &monitor); err != nil {
		return nil, err
	}

	return &monitor, nil
}

func (s *CmdbIdcAssetHardwareMonitorService) buildAssetVO(asset model.CmdbIdcAsset, roomName, cabinetName string, vmCount int64) model.CmdbIdcAssetVo {
	return model.CmdbIdcAssetVo{
		ID:                   asset.ID,
		RoomID:               asset.RoomID,
		RoomName:             roomName,
		CabinetID:            asset.CabinetID,
		CabinetName:          cabinetName,
		CabinetPosition:      asset.CabinetPosition,
		CabinetUSize:         normalizeCabinetUSizeDefault(asset.CabinetUSize),
		VmCount:              vmCount,
		IP:                   asset.IP,
		Brand:                asset.Brand,
		Model:                asset.Model,
		CPU:                  asset.CPU,
		Disk:                 asset.Disk,
		Memory:               asset.Memory,
		Purpose:              asset.Purpose,
		RemoteManagementCard: asset.RemoteManagementCard,
		Remark:               asset.Remark,
		PurchaseDate:         asset.PurchaseDate,
		WarrantyStartDate:    asset.WarrantyStartDate,
		WarrantyEndDate:      asset.WarrantyEndDate,
		UsageYears:           asset.UsageYears,
		AssetEvaluation:      asset.AssetEvaluation,
		CreateTime:           asset.CreateTime,
		UpdateTime:           asset.UpdateTime,
	}
}

func (s *CmdbIdcAssetHardwareMonitorService) resolvePrometheusAccount() (*prometheusAccount, error) {
	dao := configDao.NewAccountAuthDao()
	accounts, _, err := dao.GetAccountsByType(1, 1, 10)
	if err != nil {
		return nil, fmt.Errorf("查询Prometheus凭据失败: %w", err)
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("未找到Prometheus凭据")
	}

	account := accounts[0]
	password, err := account.DecryptPassword()
	if err != nil {
		return nil, fmt.Errorf("解密Prometheus密码失败: %w", err)
	}

	baseURL := buildPrometheusBaseURL(account)
	if baseURL == "" {
		return nil, fmt.Errorf("Prometheus地址为空")
	}

	return &prometheusAccount{
		BaseURL:  baseURL,
		Username: account.Name,
		Password: password,
	}, nil
}

func (s *CmdbIdcAssetHardwareMonitorService) detectTargetMatch(account *prometheusAccount, candidates []string, lookbackMinutes int) (*targetMatch, error) {
	for _, candidate := range candidates {
		for _, label := range []string{"target", "instance"} {
			for _, spec := range hardwareVendorSpecs {
				for _, metric := range spec.discovery {
					query := discoveryMetricQuery(metric, label, candidate, spec.requiredJob, lookbackMinutes)
					response, err := s.queryPrometheusInstant(account, query)
					if err != nil {
						return nil, err
					}
					metricResult, found, err := selectDiscoveryMetricResult(response, spec.requiredJob, spec.requiredJob != "")
					if err != nil {
						return nil, err
					}
					if !found && spec.requiredJob != "" {
						// Compatibility path for old Prometheus data without a job label.
						fallbackQuery := discoveryMetricQuery(metric, label, candidate, "", lookbackMinutes)
						fallbackResponse, fallbackErr := s.queryPrometheusInstant(account, fallbackQuery)
						if fallbackErr != nil {
							return nil, fallbackErr
						}
						metricResult, found, err = selectDiscoveryMetricResult(fallbackResponse, "", true)
						if err != nil {
							return nil, err
						}
					}
					if !found {
						continue
					}
					matchedValue := valueFromVector(metricResult.Value)
					if spec.vendor == "ipmi" && matchedValue <= 0 {
						continue
					}
					metricLabels := metricResult.Metric
					return &targetMatch{
						vendor:       spec,
						label:        label,
						target:       candidate,
						job:          metricLabels["job"],
						matchedValue: matchedValue,
					}, nil
				}
			}
		}
	}
	return nil, nil
}

func discoveryMetricQuery(metric, label, candidate, requiredJob string, lookbackMinutes int) string {
	selector := fmt.Sprintf(`%s=%q`, label, candidate)
	if strings.TrimSpace(requiredJob) != "" {
		selector = fmt.Sprintf(`job=%q,%s`, requiredJob, selector)
	}
	return fmt.Sprintf(`last_over_time(%s{%s}[%dm])`, metric, selector, lookbackMinutes)
}

func selectDiscoveryMetricResult(response *prometheusAPIResponse, requiredJob string, constrainJob bool) (prometheusMetricResult, bool, error) {
	var selected prometheusMetricResult
	if response == nil || len(response.Data.Result) == 0 {
		return selected, false, nil
	}
	jobSet := map[string]struct{}{}
	found := 0
	for _, item := range response.Data.Result {
		job := strings.TrimSpace(item.Metric["job"])
		if constrainJob {
			if requiredJob != "" && job != requiredJob {
				continue
			}
			if requiredJob == "" && job != "" {
				continue
			}
		}
		if job != "" {
			jobSet[job] = struct{}{}
		}
		if found == 0 {
			selected = item
		}
		found++
	}
	if len(jobSet) > 1 {
		return selected, false, fmt.Errorf("Prometheus目标标签存在歧义：匹配到多个 job（%s）", strings.Join(sortedStringSet(jobSet), ", "))
	}
	return selected, found > 0, nil
}

func sortedStringSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func (s *CmdbIdcAssetHardwareMonitorService) fillSummary(account *prometheusAccount, match *targetMatch, lookbackMinutes int, monitor *model.CmdbIdcAssetHardwareMonitorVo) error {
	if match.vendor.vendor == "ipmi" {
		return s.fillIPMISummary(account, match, lookbackMinutes, monitor)
	}
	queries := map[string]string{
		"overall":  match.vendor.overallMetric,
		"cpu":      match.vendor.cpuMetric,
		"memory":   match.vendor.memoryMetric,
		"disk":     match.vendor.diskMetric,
		"fan":      match.vendor.fanMetric,
		"power":    match.vendor.powerMetric,
		"temp":     match.vendor.tempMetric,
		"cpuUsage": match.vendor.cpuUsageMetric,
		"memUsage": match.vendor.memUsageMetric,
	}
	if match.vendor.vendor == "dell" {
		queries["powerWatts"] = fmt.Sprintf(`max(last_over_time(%s{%s=%q,amperageProbeIndex="3"}[%dm]))`, match.vendor.powerWatts, match.label, match.target, lookbackMinutes)
	} else {
		queries["powerWatts"] = match.vendor.powerWatts
	}

	for key, metricName := range queries {
		if strings.TrimSpace(metricName) == "" {
			continue
		}
		query := metricName
		if !strings.Contains(metricName, "(") {
			query = fmt.Sprintf(`last_over_time(%s{%s=%q}[%dm])`, metricName, match.label, match.target, lookbackMinutes)
		}
		response, err := s.queryPrometheusInstant(account, query)
		if err != nil {
			return err
		}
		if len(response.Data.Result) == 0 {
			continue
		}
		value := valueFromVector(response.Data.Result[0].Value)
		s.applySummaryValue(match.vendor.vendor, key, value, &monitor.Summary)
	}

	highestTemperature, ok, err := s.fetchHighestTemperature(account, match, lookbackMinutes)
	if err != nil {
		return err
	}
	if ok {
		monitor.Summary.HighestTemperatureCelsius = highestTemperature
	}

	return nil
}

func (s *CmdbIdcAssetHardwareMonitorService) fillComponents(account *prometheusAccount, match *targetMatch, lookbackMinutes int, monitor *model.CmdbIdcAssetHardwareMonitorVo) error {
	if match.vendor.vendor == "ipmi" {
		return s.fillIPMIComponents(account, match, lookbackMinutes, monitor)
	}
	if match.vendor.vendor == "huawei" {
		monitor.Components.Fans = s.fetchComponentList(account, fmt.Sprintf(`last_over_time(fanStatus{%s=%q}[%dm])`, match.label, match.target, lookbackMinutes), "fanIndex", "Fan", "", "fanStatus", match.vendor.vendor)
		monitor.Components.PowerSupplies = s.fetchComponentList(account, fmt.Sprintf(`last_over_time(powerSupplyStatus{%s=%q}[%dm])`, match.label, match.target, lookbackMinutes), "powerSupplyIndex", "PSU", "", "powerSupplyStatus", match.vendor.vendor)
		monitor.Components.Disks = s.fetchComponentListWithValue(account,
			fmt.Sprintf(`last_over_time(hardDiskStatus{%s=%q}[%dm])`, match.label, match.target, lookbackMinutes),
			fmt.Sprintf(`last_over_time(hardDiskTemperature{%s=%q}[%dm])`, match.label, match.target, lookbackMinutes),
			"hardDiskIndex", "Disk", "°C", "hardDiskStatus", match.vendor.vendor,
		)
		monitor.Components.RaidControllers = s.fetchComponentList(account, fmt.Sprintf(`last_over_time(raidControllerHealthStatus{%s=%q}[%dm])`, match.label, match.target, lookbackMinutes), "raidControllerIndex", "RAID", "", "raidControllerHealthStatus", match.vendor.vendor)
		monitor.Components.Temperatures = s.fetchComponentListWithValue(account,
			fmt.Sprintf(`last_over_time(temperatureIndex{%s=%q}[%dm])`, match.label, match.target, lookbackMinutes),
			fmt.Sprintf(`last_over_time(temperatureReading{%s=%q}[%dm])`, match.label, match.target, lookbackMinutes),
			"temperatureIndex", "Temp", "°C", "temperatureReading", match.vendor.vendor,
		)
		monitor.Components.Disks = filterHuaweiDiskComponents(monitor.Components.Disks)
		return nil
	}

	monitor.Components.Fans = s.fetchComponentListWithValue(account,
		fmt.Sprintf(`last_over_time(coolingDeviceStatus{%s=%q}[%dm])`, match.label, match.target, lookbackMinutes),
		fmt.Sprintf(`last_over_time(coolingDeviceReading{%s=%q}[%dm])`, match.label, match.target, lookbackMinutes),
		"coolingDeviceIndex", "Fan", "RPM", "coolingDeviceReading", match.vendor.vendor,
	)
	monitor.Components.PowerSupplies = s.fetchComponentList(account, fmt.Sprintf(`last_over_time(powerSupplySensorState{%s=%q}[%dm])`, match.label, match.target, lookbackMinutes), "powerSupplyIndex", "PSU", "", "powerSupplySensorState", match.vendor.vendor)
	monitor.Components.Disks = s.fetchComponentList(account, fmt.Sprintf(`last_over_time(physicalDiskComponentStatus{%s=%q}[%dm])`, match.label, match.target, lookbackMinutes), "physicalDiskNumber", "Disk", "", "physicalDiskComponentStatus", match.vendor.vendor)
	monitor.Components.RaidControllers = s.fetchComponentList(account, fmt.Sprintf(`last_over_time(controllerComponentStatus{%s=%q}[%dm])`, match.label, match.target, lookbackMinutes), "controllerNumber", "RAID", "", "controllerComponentStatus", match.vendor.vendor)
	monitor.Components.Temperatures = s.fetchComponentListWithValue(account,
		fmt.Sprintf(`last_over_time(temperatureProbeStatus{%s=%q}[%dm])`, match.label, match.target, lookbackMinutes),
		fmt.Sprintf(`last_over_time(temperatureProbeReading{%s=%q}[%dm])`, match.label, match.target, lookbackMinutes),
		"temperatureProbeIndex", "Temp", "°C", "temperatureProbeReading", match.vendor.vendor,
	)

	return nil
}

func (s *CmdbIdcAssetHardwareMonitorService) fetchComponentList(account *prometheusAccount, statusQuery, indexKey, prefix, unit, rawMetric, vendor string) []model.CmdbIdcAssetHardwareComponentVo {
	response, err := s.queryPrometheusInstant(account, statusQuery)
	if err != nil || len(response.Data.Result) == 0 {
		return []model.CmdbIdcAssetHardwareComponentVo{}
	}
	items := make([]model.CmdbIdcAssetHardwareComponentVo, 0, len(response.Data.Result))
	for _, resultItem := range response.Data.Result {
		index := componentIndex(resultItem.Metric, indexKey)
		name := buildComponentName(prefix, index)
		for labelKey, labelValue := range resultItem.Metric {
			if strings.HasSuffix(strings.ToLower(labelKey), "locationname") || strings.HasSuffix(strings.ToLower(labelKey), "displayname") {
				decoded := strings.TrimSpace(decodeHexMetricLabel(labelValue))
				if decoded != "" {
					name = decoded
					break
				}
			}
		}
		component := model.CmdbIdcAssetHardwareComponentVo{
			Name:      name,
			Index:     index,
			Status:    normalizeStatus(vendor, rawMetric, valueFromVector(resultItem.Value)),
			Unit:      unit,
			RawMetric: rawMetric,
		}
		if shouldSkipComponent(vendor, rawMetric, component, nil) {
			continue
		}
		if shouldSkipComponent(vendor, rawMetric, component, component.Value) {
			continue
		}
		items = append(items, component)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func (s *CmdbIdcAssetHardwareMonitorService) fetchComponentListWithValue(account *prometheusAccount, statusQuery, valueQuery, indexKey, prefix, unit, rawMetric, vendor string) []model.CmdbIdcAssetHardwareComponentVo {
	statusResponse, err := s.queryPrometheusInstant(account, statusQuery)
	if err != nil || len(statusResponse.Data.Result) == 0 {
		return []model.CmdbIdcAssetHardwareComponentVo{}
	}
	valueMap := map[string]float64{}
	valueResponse, err := s.queryPrometheusInstant(account, valueQuery)
	if err == nil {
		for _, item := range valueResponse.Data.Result {
			valueMap[componentIndex(item.Metric, indexKey)] = valueFromVector(item.Value)
		}
	}

	items := make([]model.CmdbIdcAssetHardwareComponentVo, 0, len(statusResponse.Data.Result))
	for _, resultItem := range statusResponse.Data.Result {
		index := componentIndex(resultItem.Metric, indexKey)
		name := buildComponentName(prefix, index)
		for labelKey, labelValue := range resultItem.Metric {
			if strings.HasSuffix(strings.ToLower(labelKey), "locationname") || strings.HasSuffix(strings.ToLower(labelKey), "displayname") {
				decoded := strings.TrimSpace(decodeHexMetricLabel(labelValue))
				if decoded != "" {
					name = decoded
					break
				}
			}
		}
		component := model.CmdbIdcAssetHardwareComponentVo{
			Name:      name,
			Index:     index,
			Status:    normalizeStatus(vendor, rawMetric, valueFromVector(resultItem.Value)),
			Unit:      unit,
			RawMetric: rawMetric,
		}
		if value, ok := valueMap[index]; ok {
			normalized := value
			if unit == "°C" {
				normalized = normalizeTemperatureMetric(vendor, rawMetric, value)
			}
			component.Value = &normalized
		}
		if value, ok := valueMap[index]; ok && (rawMetric == "temperatureReading" || rawMetric == "temperatureProbeReading") {
			if normalized, valid := normalizeComponentValue(vendor, rawMetric, unit, value); valid {
				component.Value = &normalized
			} else {
				component.Value = nil
			}
		}
		if shouldSkipComponent(vendor, rawMetric, component, component.Value) {
			continue
		}
		items = append(items, component)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

func (s *CmdbIdcAssetHardwareMonitorService) applySummaryValue(vendor, key string, value float64, summary *model.CmdbIdcAssetHardwareSummaryVo) {
	switch key {
	case "overall":
		summary.OverallStatus = normalizeStatus(vendor, key, value)
	case "cpu":
		summary.CPUStatus = normalizeStatus(vendor, key, value)
	case "memory":
		summary.MemoryStatus = normalizeStatus(vendor, key, value)
	case "disk":
		summary.DiskStatus = normalizeStatus(vendor, key, value)
	case "fan":
		summary.FanStatus = normalizeStatus(vendor, key, value)
	case "power":
		summary.PowerStatus = normalizeStatus(vendor, key, value)
	case "temp":
		summary.TemperatureStatus = normalizeStatus(vendor, key, value)
	case "cpuUsage":
		summary.CPUUsagePercent = roundFloat(value, 2)
	case "memUsage":
		summary.MemoryUsagePercent = roundFloat(value, 2)
	case "powerWatts":
		summary.PowerWatts = roundFloat(value, 2)
	}
}

func (s *CmdbIdcAssetHardwareMonitorService) queryPrometheusInstant(account *prometheusAccount, query string) (*prometheusAPIResponse, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, strings.TrimSuffix(account.BaseURL, "/")+"/api/v1/query", nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Set("query", query)
	q.Set("time", strconv.FormatInt(time.Now().Unix(), 10))
	q.Set("nocache", "1")
	q.Set("_", strconv.FormatInt(time.Now().UnixNano(), 10))
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	if account.Username != "" || account.Password != "" {
		req.SetBasicAuth(account.Username, account.Password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Prometheus请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取Prometheus响应失败: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("Prometheus认证失败")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Prometheus返回异常状态码: %d", resp.StatusCode)
	}

	var response prometheusAPIResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("解析Prometheus响应失败: %w", err)
	}
	if response.Status != "success" {
		if response.Error != "" {
			return nil, fmt.Errorf("Prometheus查询失败: %s", response.Error)
		}
		return nil, fmt.Errorf("Prometheus查询失败")
	}
	return &response, nil
}

func (s *CmdbIdcAssetHardwareMonitorService) queryPrometheusRange(account *prometheusAccount, query string, start, end time.Time, step time.Duration) (*model.PrometheusQueryResult, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, strings.TrimSuffix(account.BaseURL, "/")+"/api/v1/query_range", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Set("query", query)
	q.Set("start", strconv.FormatInt(start.Unix(), 10))
	q.Set("end", strconv.FormatInt(end.Unix(), 10))
	q.Set("step", strconv.Itoa(int(step.Seconds())))
	q.Set("nocache", "1")
	q.Set("_", strconv.FormatInt(time.Now().UnixNano(), 10))
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	if account.Username != "" || account.Password != "" {
		req.SetBasicAuth(account.Username, account.Password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Prometheus璇锋眰澶辫触: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("璇诲彇Prometheus鍝嶅簲澶辫触: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("Prometheus璁よ瘉澶辫触")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Prometheus杩斿洖寮傚父鐘舵€佺爜: %d", resp.StatusCode)
	}

	var response model.PrometheusQueryResult
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("瑙ｆ瀽Prometheus鍝嶅簲澶辫触: %w", err)
	}
	if response.Status != "success" {
		return nil, fmt.Errorf("Prometheus鏌ヨ澶辫触")
	}
	return &response, nil
}

func (s *CmdbIdcAssetHardwareMonitorService) fillTrendsAndAlerts(account *prometheusAccount, match *targetMatch, trendHours int, trendStepMinutes int, monitor *model.CmdbIdcAssetHardwareMonitorVo) error {
	if match.vendor.vendor == "ipmi" {
		return s.fillIPMITrendsAndAlerts(account, match, trendHours, trendStepMinutes, monitor)
	}
	windowHours, stepMinutes := normalizeTrendWindow(trendHours, trendStepMinutes)
	monitor.Trends.WindowHours = windowHours
	monitor.Trends.StepMinutes = stepMinutes
	monitor.Trends.Panels = []model.CmdbIdcAssetHardwareTrendPanelVo{}
	monitor.Alerts = []model.CmdbIdcAssetHardwareAlertVo{}

	end := time.Now()
	start := end.Add(-time.Duration(windowHours) * time.Hour)
	step := time.Duration(stepMinutes) * time.Minute

	monitor.Trends.Panels = append(monitor.Trends.Panels, s.buildDiskLifeTrendPanel(account, match, start, end, step, windowHours, stepMinutes)...)
	monitor.Trends.Panels = append(monitor.Trends.Panels, s.buildThermalTrendPanel(account, match, start, end, step, windowHours, stepMinutes)...)
	monitor.Trends.Panels = append(monitor.Trends.Panels, s.buildRaidTrendPanel(account, match, start, end, step, windowHours, stepMinutes)...)
	monitor.Trends.Panels = append(monitor.Trends.Panels, s.buildPowerTrendPanel(account, match, start, end, step, windowHours, stepMinutes)...)
	monitor.Trends.Panels = append(monitor.Trends.Panels, s.buildMemoryTrendPanel(account, match, start, end, step, windowHours, stepMinutes)...)

	monitor.Alerts = append(monitor.Alerts, s.buildDiskLifeAlerts(monitor.Trends.Panels)...)
	monitor.Alerts = append(monitor.Alerts, s.buildRaidAlerts(monitor.Trends.Panels, monitor)...)
	monitor.Alerts = append(monitor.Alerts, s.buildPowerAlerts(monitor.Trends.Panels, monitor)...)
	monitor.Alerts = append(monitor.Alerts, s.buildMemoryAlerts(monitor.Trends.Panels, monitor)...)
	monitor.Alerts = append(monitor.Alerts, s.buildFanHardwareAlerts(monitor)...)
	monitor.Alerts = enrichAlertPersistence(monitor.Alerts, monitor.Trends.Panels, stepMinutes)
	monitor.Alerts = dedupeHardwareAlerts(monitor.Alerts)
	monitor.Health = buildHardwareHealth(monitor.Alerts, len(monitor.Trends.Panels))
	applyCriticalStorageSummary(monitor)
	monitor.Trends.Panels = normalizeTrendPanelsForDisplay(monitor.Trends.Panels)
	annotateTrendPanelStatuses(monitor)

	return nil
}

func (s *CmdbIdcAssetHardwareMonitorService) buildDiskLifeTrendPanel(account *prometheusAccount, match *targetMatch, start, end time.Time, step time.Duration, windowHours int, stepMinutes int) []model.CmdbIdcAssetHardwareTrendPanelVo {
	panels := []model.CmdbIdcAssetHardwareTrendPanelVo{}

	if match.vendor.vendor == "huawei" {
		lifeSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("hardDiskPowerOnHours", match, ""), "hardDiskIndex", "Disk", "hours", "", "hardDiskPowerOnHours", match.vendor.vendor)
		wearSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("hardDiskRemnantWearout", match, ""), "hardDiskIndex", "Disk", "%", "", "hardDiskRemnantWearout", match.vendor.vendor)
		statusSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("hardDiskStatus", match, ""), "hardDiskIndex", "Disk", "", "", "hardDiskStatus", match.vendor.vendor)
		lifeSeries, wearSeries, statusSeries = filterHuaweiDiskTrendSeries(lifeSeries, wearSeries, statusSeries)

		if len(lifeSeries) > 0 || len(wearSeries) > 0 || len(statusSeries) > 0 {
			panels = append(panels, model.CmdbIdcAssetHardwareTrendPanelVo{
				Key:         "disk-life",
				Title:       "硬盘寿命趋势",
				Category:    "lifecycle",
				ChartType:   "line",
				WindowHours: windowHours,
				StepMinutes: stepMinutes,
				Series: append(
					append(lifeSeries, wearSeries...),
					statusSeries...,
				),
			})
		}
		return panels
	}

	wearSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("physicalDiskRemainingRatedWriteEndurance", match, ""), "physicalDiskNumber", "Disk", "%", "", "physicalDiskRemainingRatedWriteEndurance", match.vendor.vendor)
	statusSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("physicalDiskComponentStatus", match, ""), "physicalDiskNumber", "Disk", "", "", "physicalDiskComponentStatus", match.vendor.vendor)
	if len(wearSeries) > 0 || len(statusSeries) > 0 {
		title := "SSD 磨损趋势"
		switch {
		case len(wearSeries) > 0 && len(statusSeries) > 0:
			title = "SSD耐久度与磁盘状态"
		case len(wearSeries) > 0:
			title = "SSD耐久度趋势"
		case len(statusSeries) > 0:
			title = "磁盘健康状态"
		}
		panels = append(panels, model.CmdbIdcAssetHardwareTrendPanelVo{
			Key:         "disk-life",
			Title:       title,
			Category:    "lifecycle",
			ChartType:   "line",
			WindowHours: windowHours,
			StepMinutes: stepMinutes,
			Series:      append(wearSeries, statusSeries...),
		})
	}

	return panels
}

func (s *CmdbIdcAssetHardwareMonitorService) buildThermalTrendPanel(account *prometheusAccount, match *targetMatch, start, end time.Time, step time.Duration, windowHours int, stepMinutes int) []model.CmdbIdcAssetHardwareTrendPanelVo {
	panels := []model.CmdbIdcAssetHardwareTrendPanelVo{}

	if match.vendor.vendor == "huawei" {
		fanSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("fanSpeed", match, ""), "fanIndex", "Fan", "RPM", "left", "fanSpeed", match.vendor.vendor)
		tempSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("temperatureReading", match, ""), "temperatureIndex", "Temp", "°C", "right", "temperatureReading", match.vendor.vendor)
		if len(fanSeries) > 0 || len(tempSeries) > 0 {
			panels = append(panels, model.CmdbIdcAssetHardwareTrendPanelVo{
				Key:         "thermal-efficiency",
				Title:       "散热系统效率",
				Category:    "thermal",
				ChartType:   "dual-line",
				WindowHours: windowHours,
				StepMinutes: stepMinutes,
				Series:      append(fanSeries, tempSeries...),
			})
		}
		return panels
	}

	fanSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("coolingDeviceReading", match, ""), "coolingDeviceIndex", "Fan", "RPM", "left", "coolingDeviceReading", match.vendor.vendor)
	tempSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("temperatureProbeReading", match, ""), "temperatureProbeIndex", "Temp", "°C", "right", "temperatureProbeReading", match.vendor.vendor)
	if len(fanSeries) > 0 || len(tempSeries) > 0 {
		panels = append(panels, model.CmdbIdcAssetHardwareTrendPanelVo{
			Key:         "thermal-efficiency",
			Title:       "散热系统效率",
			Category:    "thermal",
			ChartType:   "dual-line",
			WindowHours: windowHours,
			StepMinutes: stepMinutes,
			Series:      append(fanSeries, tempSeries...),
		})
	}
	return panels
}

func (s *CmdbIdcAssetHardwareMonitorService) buildRaidTrendPanel(account *prometheusAccount, match *targetMatch, start, end time.Time, step time.Duration, windowHours int, stepMinutes int) []model.CmdbIdcAssetHardwareTrendPanelVo {
	panels := []model.CmdbIdcAssetHardwareTrendPanelVo{}

	if match.vendor.vendor == "huawei" {
		eccSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("raidControllerDDREccCount", match, ""), "raidControllerIndex", "RAID", "count", "", "raidControllerDDREccCount", match.vendor.vendor)
		statusSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("raidControllerHealthStatus", match, ""), "raidControllerIndex", "RAID", "", "", "raidControllerHealthStatus", match.vendor.vendor)
		logicalSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("logicalDriveConsistencyCheckStatus", match, ""), "logicalDriveIndex", "LD", "", "", "logicalDriveConsistencyCheckStatus", match.vendor.vendor)
		diskSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("hardDiskStatus", match, ""), "hardDiskIndex", "Disk", "", "", "hardDiskStatus", match.vendor.vendor)
		sortTrendSeriesByStatusSeverity(diskSeries)
		if len(eccSeries) > 0 || len(statusSeries) > 0 || len(logicalSeries) > 0 || len(diskSeries) > 0 {
			panels = append(panels, model.CmdbIdcAssetHardwareTrendPanelVo{
				Key:         "raid-health",
				Title:       "RAID/存储链路健康",
				Category:    "error",
				ChartType:   "step-line",
				WindowHours: windowHours,
				StepMinutes: stepMinutes,
				Series: append(
					append(
						append(diskSeries, statusSeries...),
						logicalSeries...,
					),
					eccSeries...,
				),
			})
		}
		return panels
	}

	statusSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("controllerComponentStatus", match, ""), "controllerNumber", "RAID", "", "", "controllerComponentStatus", match.vendor.vendor)
	diskSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("physicalDiskComponentStatus", match, ""), "physicalDiskNumber", "Disk", "", "", "physicalDiskComponentStatus", match.vendor.vendor)
	sortTrendSeriesByStatusSeverity(diskSeries)
	if len(statusSeries) > 0 || len(diskSeries) > 0 {
		panels = append(panels, model.CmdbIdcAssetHardwareTrendPanelVo{
			Key:         "raid-health",
			Title:       "RAID/存储链路健康",
			Category:    "error",
			ChartType:   "step-line",
			WindowHours: windowHours,
			StepMinutes: stepMinutes,
			Series:      append(diskSeries, statusSeries...),
		})
	}
	return panels
}

func (s *CmdbIdcAssetHardwareMonitorService) buildPowerTrendPanel(account *prometheusAccount, match *targetMatch, start, end time.Time, step time.Duration, windowHours int, stepMinutes int) []model.CmdbIdcAssetHardwareTrendPanelVo {
	panels := []model.CmdbIdcAssetHardwareTrendPanelVo{}

	if match.vendor.vendor == "huawei" {
		statusSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("powerSupplyStatus", match, ""), "powerSupplyIndex", "PSU", "", "", "powerSupplyStatus", match.vendor.vendor)
		if len(statusSeries) > 0 {
			panels = append(panels, model.CmdbIdcAssetHardwareTrendPanelVo{
				Key:         "power-redundancy",
				Title:       "电源冗余状态",
				Category:    "power",
				ChartType:   "multi-line",
				WindowHours: windowHours,
				StepMinutes: stepMinutes,
				Series:      statusSeries,
			})
		}
		return panels
	}

	statusSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("powerSupplySensorState", match, ""), "powerSupplyIndex", "PSU", "", "", "powerSupplySensorState", match.vendor.vendor)
	powerSeries := s.fetchTrendSeries(account, start, end, step, dellPowerWattsRangeQuery(match), "amperageProbeIndex", "Power", "W", "", "amperageProbeReading", match.vendor.vendor)
	if len(statusSeries) > 0 || len(powerSeries) > 0 {
		panels = append(panels, model.CmdbIdcAssetHardwareTrendPanelVo{
			Key:         "power-redundancy",
			Title:       "电源冗余状态",
			Category:    "power",
			ChartType:   "multi-line",
			WindowHours: windowHours,
			StepMinutes: stepMinutes,
			Series:      append(statusSeries, powerSeries...),
		})
	}
	return panels
}

func (s *CmdbIdcAssetHardwareMonitorService) buildMemoryTrendPanel(account *prometheusAccount, match *targetMatch, start, end time.Time, step time.Duration, windowHours int, stepMinutes int) []model.CmdbIdcAssetHardwareTrendPanelVo {
	panels := []model.CmdbIdcAssetHardwareTrendPanelVo{}

	if match.vendor.vendor == "huawei" {
		memSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("memoryStatus", match, ""), "memoryDimmIndex", "DIMM", "", "", "memoryStatus", match.vendor.vendor)
		eccSeries := s.fetchTrendSeries(account, start, end, step, metricRangeQuery("raidControllerDDREccCount", match, ""), "raidControllerIndex", "RAID", "count", "", "raidControllerDDREccCount", match.vendor.vendor)
		if len(memSeries) > 0 || len(eccSeries) > 0 {
			panels = append(panels, model.CmdbIdcAssetHardwareTrendPanelVo{
				Key:         "memory-stability",
				Title:       "内存/系统稳定性",
				Category:    "stability",
				ChartType:   "heatmap",
				WindowHours: windowHours,
				StepMinutes: stepMinutes,
				Series:      append(memSeries, eccSeries...),
			})
		}
	}

	return panels
}

func (s *CmdbIdcAssetHardwareMonitorService) fillIPMISummary(account *prometheusAccount, match *targetMatch, lookbackMinutes int, monitor *model.CmdbIdcAssetHardwareMonitorVo) error {
	if response, err := s.queryPrometheusInstant(account, ipmiInstantQuery("ipmi_bmc_info", match, lookbackMinutes, "")); err != nil {
		return err
	} else if len(response.Data.Result) > 0 {
		manufacturer := strings.TrimSpace(response.Data.Result[0].Metric["manufacturer_id"])
		if manufacturer != "" {
			monitor.RawVendor = manufacturer
		}
	}

	cpuStatus, err := s.aggregateIPMIState(account, match, lookbackMinutes, "ipmi_sensor_state", func(labels map[string]string) bool {
		return strings.EqualFold(strings.TrimSpace(labels["type"]), "Processor")
	})
	if err != nil {
		return err
	}
	memoryStatus, err := s.aggregateIPMIState(account, match, lookbackMinutes, "ipmi_sensor_state", func(labels map[string]string) bool {
		return strings.EqualFold(strings.TrimSpace(labels["type"]), "Memory")
	})
	if err != nil {
		return err
	}
	diskStatus, err := s.aggregateIPMIState(account, match, lookbackMinutes, "ipmi_sensor_state", func(labels map[string]string) bool {
		return strings.EqualFold(strings.TrimSpace(labels["type"]), "Drive Slot")
	})
	if err != nil {
		return err
	}
	fanStatus, err := s.aggregateIPMIState(account, match, lookbackMinutes, "ipmi_fan_speed_state", nil)
	if err != nil {
		return err
	}
	powerStatus, err := s.aggregateIPMIState(account, match, lookbackMinutes, "ipmi_power_state", nil)
	if err != nil {
		return err
	}
	for _, metric := range []string{"ipmi_current_state", "ipmi_voltage_state"} {
		status, queryErr := s.aggregateIPMIState(account, match, lookbackMinutes, metric, nil)
		if queryErr != nil {
			return queryErr
		}
		powerStatus = moreSevereStatus(powerStatus, status)
	}
	temperatureStatus, err := s.aggregateIPMIState(account, match, lookbackMinutes, "ipmi_temperature_state", nil)
	if err != nil {
		return err
	}

	if status, ok, queryErr := s.queryIPMIChassisStatus(account, match, lookbackMinutes, "ipmi_chassis_cooling_fault_state"); queryErr != nil {
		return queryErr
	} else if ok {
		fanStatus = moreSevereStatus(fanStatus, status)
	}
	if status, ok, queryErr := s.queryIPMIChassisStatus(account, match, lookbackMinutes, "ipmi_chassis_drive_fault_state"); queryErr != nil {
		return queryErr
	} else if ok {
		diskStatus = moreSevereStatus(diskStatus, status)
	}
	if status, ok, queryErr := s.queryIPMIChassisStatus(account, match, lookbackMinutes, "ipmi_chassis_power_state"); queryErr != nil {
		return queryErr
	} else if ok {
		powerStatus = moreSevereStatus(powerStatus, status)
	}

	monitor.Summary.CPUStatus = cpuStatus
	monitor.Summary.MemoryStatus = memoryStatus
	monitor.Summary.DiskStatus = diskStatus
	monitor.Summary.FanStatus = fanStatus
	monitor.Summary.PowerStatus = powerStatus
	monitor.Summary.TemperatureStatus = temperatureStatus
	monitor.Summary.OverallStatus = aggregateHardwareStatus(cpuStatus, memoryStatus, diskStatus, fanStatus, powerStatus, temperatureStatus)

	if value, ok, queryErr := s.queryIPMIScalar(account, match, lookbackMinutes, "ipmi_power_watts", "name=\"Total_Power\""); queryErr != nil {
		return queryErr
	} else if ok {
		monitor.Summary.PowerWatts = roundFloat(value, 2)
	}
	if value, ok, queryErr := s.queryIPMIMax(account, match, lookbackMinutes, "ipmi_temperature_celsius", ""); queryErr != nil {
		return queryErr
	} else if ok && value > 0 {
		monitor.Summary.HighestTemperatureCelsius = roundFloat(value, 2)
	}

	return nil
}

func (s *CmdbIdcAssetHardwareMonitorService) fillIPMIComponents(account *prometheusAccount, match *targetMatch, lookbackMinutes int, monitor *model.CmdbIdcAssetHardwareMonitorVo) error {
	var err error
	monitor.Components.Temperatures, err = s.fetchIPMIComponentList(account, match, lookbackMinutes, "ipmi_temperature_state", "ipmi_temperature_celsius", "°C", func(labels map[string]string, value *float64) bool {
		return value != nil && *value > 0
	})
	if err != nil {
		return err
	}
	monitor.Components.Fans, err = s.fetchIPMIComponentList(account, match, lookbackMinutes, "ipmi_fan_speed_state", "ipmi_fan_speed_rpm", "RPM", nil)
	if err != nil {
		return err
	}
	monitor.Components.PowerSupplies, err = s.fetchIPMIComponentList(account, match, lookbackMinutes, "ipmi_power_state", "ipmi_power_watts", "W", nil)
	if err != nil {
		return err
	}
	monitor.Components.Voltages, err = s.fetchIPMIComponentList(account, match, lookbackMinutes, "ipmi_voltage_state", "ipmi_voltage_volts", "V", nil)
	if err != nil {
		return err
	}
	monitor.Components.Currents, err = s.fetchIPMIComponentList(account, match, lookbackMinutes, "ipmi_current_state", "ipmi_current_amperes", "A", nil)
	if err != nil {
		return err
	}

	sensors, err := s.fetchIPMIComponentList(account, match, lookbackMinutes, "ipmi_sensor_state", "", "", func(labels map[string]string, value *float64) bool {
		return ipmiSensorParticipatesInHealth(labels)
	})
	if err != nil {
		return err
	}
	for _, sensor := range sensors {
		switch strings.ToLower(strings.TrimSpace(sensor.Type)) {
		case "drive slot":
			monitor.Components.Disks = append(monitor.Components.Disks, sensor)
		default:
			monitor.Components.Sensors = append(monitor.Components.Sensors, sensor)
		}
	}
	return nil
}

func (s *CmdbIdcAssetHardwareMonitorService) fillIPMITrendsAndAlerts(account *prometheusAccount, match *targetMatch, trendHours int, trendStepMinutes int, monitor *model.CmdbIdcAssetHardwareMonitorVo) error {
	windowHours, stepMinutes := normalizeTrendWindow(trendHours, trendStepMinutes)
	monitor.Trends.WindowHours = windowHours
	monitor.Trends.StepMinutes = stepMinutes
	monitor.Trends.Panels = []model.CmdbIdcAssetHardwareTrendPanelVo{}

	end := time.Now()
	start := end.Add(-time.Duration(windowHours) * time.Hour)
	step := time.Duration(stepMinutes) * time.Minute
	temperatures := s.fetchIPMITrendSeries(account, match, start, end, step, "ipmi_temperature_celsius", "", "°C", "right", false)
	fans := s.fetchIPMITrendSeries(account, match, start, end, step, "ipmi_fan_speed_rpm", "", "RPM", "left", false)
	if len(temperatures) > 0 || len(fans) > 0 {
		monitor.Trends.Panels = append(monitor.Trends.Panels, model.CmdbIdcAssetHardwareTrendPanelVo{
			Key: "ipmi-thermal", Title: "IPMI散热与温度", Category: "thermal", ChartType: "dual-line",
			WindowHours: windowHours, StepMinutes: stepMinutes, Series: append(fans, temperatures...),
		})
	}
	power := s.fetchIPMITrendSeries(account, match, start, end, step, "ipmi_power_watts", "name=\"Total_Power\"", "W", "left", false)
	voltage := s.fetchIPMITrendSeries(account, match, start, end, step, "ipmi_voltage_volts", "name=~\"PSU.*_(VIN|VOUT)\"", "V", "right", false)
	if len(power) > 0 || len(voltage) > 0 {
		monitor.Trends.Panels = append(monitor.Trends.Panels, model.CmdbIdcAssetHardwareTrendPanelVo{
			Key: "ipmi-power", Title: "IPMI功耗与电压", Category: "power", ChartType: "dual-line",
			WindowHours: windowHours, StepMinutes: stepMinutes, Series: append(power, voltage...),
		})
	}

	alerts, err := s.buildIPMIAlerts(account, match, monitor, 5)
	if err != nil {
		return err
	}
	monitor.Alerts = dedupeHardwareAlerts(alerts)
	monitor.Health = buildHardwareHealth(monitor.Alerts, len(monitor.Trends.Panels))
	monitor.Trends.Panels = normalizeTrendPanelsForDisplay(monitor.Trends.Panels)
	annotateTrendPanelStatuses(monitor)
	return nil
}

func (s *CmdbIdcAssetHardwareMonitorService) buildIPMIAlerts(account *prometheusAccount, match *targetMatch, monitor *model.CmdbIdcAssetHardwareMonitorVo, lookbackMinutes int) ([]model.CmdbIdcAssetHardwareAlertVo, error) {
	alerts := []model.CmdbIdcAssetHardwareAlertVo{}
	for _, check := range []struct {
		metric   string
		category string
		message  string
	}{
		{metric: "ipmi_chassis_cooling_fault_state", category: "thermal", message: "IPMI reports a chassis cooling fault"},
		{metric: "ipmi_chassis_drive_fault_state", category: "error", message: "IPMI reports a chassis drive fault"},
	} {
		status, ok, err := s.queryIPMIChassisStatus(account, match, lookbackMinutes, check.metric)
		if err != nil {
			return nil, err
		}
		if ok && status == "critical" {
			alerts = append(alerts, buildAlert("critical", check.category, check.metric, "", check.message, nil, nil))
		}
	}

	componentGroups := []struct {
		items    []model.CmdbIdcAssetHardwareComponentVo
		category string
	}{
		{monitor.Components.Temperatures, "thermal"},
		{monitor.Components.Fans, "thermal"},
		{monitor.Components.PowerSupplies, "power"},
		{monitor.Components.Voltages, "power"},
		{monitor.Components.Currents, "power"},
		{monitor.Components.Disks, "error"},
		{monitor.Components.Sensors, "stability"},
	}
	for _, group := range componentGroups {
		for _, component := range group.items {
			if component.Status != "warning" && component.Status != "critical" {
				continue
			}
			alerts = append(alerts, buildAlert(component.Status, group.category, component.RawMetric, component.Name, "IPMI sensor state is abnormal", component.Value, nil))
		}
	}
	return alerts, nil
}

func (s *CmdbIdcAssetHardwareMonitorService) aggregateIPMIState(account *prometheusAccount, match *targetMatch, lookbackMinutes int, metric string, include func(map[string]string) bool) (string, error) {
	response, err := s.queryPrometheusInstant(account, ipmiInstantQuery(metric, match, lookbackMinutes, ""))
	if err != nil {
		return "unknown", err
	}
	status := "unknown"
	for _, item := range response.Data.Result {
		if include != nil && !include(item.Metric) {
			continue
		}
		value := valueFromVector(item.Value)
		if value != value {
			continue
		}
		status = moreSevereStatus(status, normalizeStatus("ipmi", metric, value))
	}
	return status, nil
}

func (s *CmdbIdcAssetHardwareMonitorService) queryIPMIChassisStatus(account *prometheusAccount, match *targetMatch, lookbackMinutes int, metric string) (string, bool, error) {
	value, ok, err := s.queryIPMIScalar(account, match, lookbackMinutes, metric, "")
	if err != nil || !ok {
		return "unknown", ok, err
	}
	switch metric {
	case "ipmi_chassis_cooling_fault_state", "ipmi_chassis_drive_fault_state":
		if value == 0 {
			return "critical", true, nil
		}
		return "normal", true, nil
	case "ipmi_chassis_power_state":
		if value == 0 {
			return "warning", true, nil
		}
		return "normal", true, nil
	default:
		return "unknown", true, nil
	}
}

func (s *CmdbIdcAssetHardwareMonitorService) queryIPMIScalar(account *prometheusAccount, match *targetMatch, lookbackMinutes int, metric, extraSelector string) (float64, bool, error) {
	response, err := s.queryPrometheusInstant(account, ipmiInstantQuery(metric, match, lookbackMinutes, extraSelector))
	if err != nil {
		return 0, false, err
	}
	if len(response.Data.Result) == 0 {
		return 0, false, nil
	}
	value := valueFromVector(response.Data.Result[0].Value)
	if value != value {
		return 0, false, nil
	}
	return value, true, nil
}

func (s *CmdbIdcAssetHardwareMonitorService) queryIPMIMax(account *prometheusAccount, match *targetMatch, lookbackMinutes int, metric, extraSelector string) (float64, bool, error) {
	query := fmt.Sprintf("max(%s)", ipmiInstantQuery(metric, match, lookbackMinutes, extraSelector))
	response, err := s.queryPrometheusInstant(account, query)
	if err != nil {
		return 0, false, err
	}
	if len(response.Data.Result) == 0 {
		return 0, false, nil
	}
	value := valueFromVector(response.Data.Result[0].Value)
	if value != value {
		return 0, false, nil
	}
	return value, true, nil
}

func (s *CmdbIdcAssetHardwareMonitorService) fetchIPMIComponentList(account *prometheusAccount, match *targetMatch, lookbackMinutes int, stateMetric, valueMetric, unit string, include func(map[string]string, *float64) bool) ([]model.CmdbIdcAssetHardwareComponentVo, error) {
	stateResponse, err := s.queryPrometheusInstant(account, ipmiInstantQuery(stateMetric, match, lookbackMinutes, ""))
	if err != nil {
		return nil, err
	}
	values := map[string]float64{}
	if valueMetric != "" {
		valueResponse, valueErr := s.queryPrometheusInstant(account, ipmiInstantQuery(valueMetric, match, lookbackMinutes, ""))
		if valueErr != nil {
			return nil, valueErr
		}
		for _, item := range valueResponse.Data.Result {
			value := valueFromVector(item.Value)
			if value == value {
				values[ipmiComponentKey(item.Metric)] = value
			}
		}
	}

	items := make([]model.CmdbIdcAssetHardwareComponentVo, 0, len(stateResponse.Data.Result))
	for _, item := range stateResponse.Data.Result {
		stateValue := valueFromVector(item.Value)
		if stateValue != stateValue {
			continue
		}
		key := ipmiComponentKey(item.Metric)
		var reading *float64
		if value, ok := values[key]; ok {
			value = roundFloat(value, 2)
			reading = &value
		}
		if include != nil && !include(item.Metric, reading) {
			continue
		}
		name := strings.TrimSpace(item.Metric["name"])
		if name == "" {
			name = buildComponentName("IPMI", key)
		}
		items = append(items, model.CmdbIdcAssetHardwareComponentVo{
			Name: name, Type: strings.TrimSpace(item.Metric["type"]), Status: normalizeStatus("ipmi", stateMetric, stateValue),
			Index: key, Value: reading, Unit: unit, RawMetric: stateMetric,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func (s *CmdbIdcAssetHardwareMonitorService) fetchIPMITrendSeries(account *prometheusAccount, match *targetMatch, start, end time.Time, step time.Duration, metric, extraSelector, unit, axis string, isState bool) []model.CmdbIdcAssetHardwareTrendSeriesVo {
	response, err := s.queryPrometheusRange(account, ipmiRangeQuery(metric, match, extraSelector), start, end, step)
	if err != nil || len(response.Data.Result) == 0 {
		return []model.CmdbIdcAssetHardwareTrendSeriesVo{}
	}
	series := make([]model.CmdbIdcAssetHardwareTrendSeriesVo, 0, len(response.Data.Result))
	for _, item := range response.Data.Result {
		name := strings.TrimSpace(item.Metric["name"])
		if name == "" {
			name = metric
		}
		points := make([]model.TrendPoint, 0, len(item.Values))
		for _, raw := range item.Values {
			if len(raw) < 2 {
				continue
			}
			timestamp, timestampOK := raw[0].(float64)
			valueText, valueOK := raw[1].(string)
			if !timestampOK || !valueOK {
				continue
			}
			value, parseErr := strconv.ParseFloat(valueText, 64)
			if parseErr != nil || value != value || (!isState && metric == "ipmi_temperature_celsius" && value <= 0) {
				continue
			}
			points = append(points, model.TrendPoint{Timestamp: time.Unix(int64(timestamp), 0).Format(time.RFC3339), Value: roundFloat(value, 2)})
		}
		if len(points) == 0 {
			continue
		}
		valueType := "gauge"
		if isState {
			valueType = "status"
		}
		series = append(series, model.CmdbIdcAssetHardwareTrendSeriesVo{
			Name: name, Metric: metric, ValueType: valueType, Unit: unit, Axis: axis,
			Component: ipmiComponentKey(item.Metric), Points: points,
		})
	}
	sort.Slice(series, func(i, j int) bool { return series[i].Name < series[j].Name })
	return series
}

func ipmiInstantQuery(metric string, match *targetMatch, lookbackMinutes int, extraSelector string) string {
	return fmt.Sprintf(`last_over_time(%s[%dm])`, ipmiRangeQuery(metric, match, extraSelector), lookbackMinutes)
}

func ipmiRangeQuery(metric string, match *targetMatch, extraSelector string) string {
	selector := fmt.Sprintf(`%s=%q`, match.label, match.target)
	if strings.TrimSpace(match.job) != "" {
		selector = fmt.Sprintf(`job=%q,%s`, match.job, selector)
	}
	if strings.TrimSpace(extraSelector) != "" {
		selector += "," + extraSelector
	}
	return fmt.Sprintf(`%s{%s}`, metric, selector)
}

func ipmiComponentKey(labels map[string]string) string {
	if value := strings.TrimSpace(labels["id"]); value != "" {
		return value
	}
	return strings.TrimSpace(labels["name"])
}

func ipmiSensorParticipatesInHealth(labels map[string]string) bool {
	switch strings.ToLower(strings.TrimSpace(labels["type"])) {
	case "physical security", "watchdog", "watchdog 2", "oem reserved", "system acpi power state":
		return false
	default:
		return true
	}
}

func aggregateHardwareStatus(statuses ...string) string {
	result := "unknown"
	for _, status := range statuses {
		result = moreSevereStatus(result, status)
	}
	return result
}

func (s *CmdbIdcAssetHardwareMonitorService) fetchTrendSeries(account *prometheusAccount, start, end time.Time, step time.Duration, query string, indexKey, prefix, unit, axis, metricName, vendor string) []model.CmdbIdcAssetHardwareTrendSeriesVo {
	if strings.TrimSpace(query) == "" {
		return []model.CmdbIdcAssetHardwareTrendSeriesVo{}
	}
	response, err := s.queryPrometheusRange(account, query, start, end, step)
	if err != nil || len(response.Data.Result) == 0 {
		return []model.CmdbIdcAssetHardwareTrendSeriesVo{}
	}

	series := make([]model.CmdbIdcAssetHardwareTrendSeriesVo, 0, len(response.Data.Result))
	for _, resultItem := range response.Data.Result {
		component := componentIndex(resultItem.Metric, indexKey)
		name := buildComponentName(prefix, component)
		for labelKey, labelValue := range resultItem.Metric {
			if strings.HasSuffix(strings.ToLower(labelKey), "locationname") || strings.HasSuffix(strings.ToLower(labelKey), "displayname") || strings.HasSuffix(strings.ToLower(labelKey), "fqdd") {
				decoded := strings.TrimSpace(decodeHexMetricLabel(labelValue))
				if decoded != "" {
					name = decoded
					break
				}
			}
		}
		points := make([]model.TrendPoint, 0, len(resultItem.Values))
		for _, value := range resultItem.Values {
			if len(value) < 2 {
				continue
			}
			timestamp, ok := value[0].(float64)
			if !ok {
				continue
			}
			valueString, ok := value[1].(string)
			if !ok {
				continue
			}
			parsed, err := strconv.ParseFloat(valueString, 64)
			if err != nil {
				continue
			}
			normalized := normalizeTrendValue(vendor, metricName, parsed)
			if skipTrendPoint(metricName, normalized) {
				continue
			}
			points = append(points, model.TrendPoint{
				Timestamp: time.Unix(int64(timestamp), 0).Format(time.RFC3339),
				Value:     normalized,
			})
		}
		if len(points) == 0 {
			continue
		}
		series = append(series, model.CmdbIdcAssetHardwareTrendSeriesVo{
			Name:      name,
			Metric:    metricName,
			ValueType: inferTrendValueType(metricName),
			Unit:      unit,
			Axis:      axis,
			Component: component,
			Points:    points,
		})
	}
	sort.Slice(series, func(i, j int) bool { return series[i].Name < series[j].Name })
	return series
}

func (s *CmdbIdcAssetHardwareMonitorService) buildDiskLifeAlerts(panels []model.CmdbIdcAssetHardwareTrendPanelVo) []model.CmdbIdcAssetHardwareAlertVo {
	alerts := []model.CmdbIdcAssetHardwareAlertVo{}
	panel := findTrendPanel(panels, "disk-life")
	if panel == nil {
		return alerts
	}

	for _, series := range panel.Series {
		last, ok := lastTrendPoint(series.Points)
		if !ok {
			continue
		}
		switch series.Metric {
		case "hardDiskPowerOnHours":
			if isDiskPowerOnHoursReset(series.Points) {
				alerts = append(alerts, buildAlert("critical", "lifecycle", series.Metric, series.Name, "disk lifetime telemetry collapsed", &last, nil))
			} else if last >= 50000 {
				alerts = append(alerts, buildAlert("warning", "lifecycle", series.Metric, series.Name, "disk power-on hours is high", &last, nil))
			}
		case "hardDiskRemnantWearout", "physicalDiskRemainingRatedWriteEndurance":
			if last <= 20 {
				threshold := 20.0
				alerts = append(alerts, buildAlert("critical", "lifecycle", series.Metric, series.Name, "disk remaining life is low", &last, &threshold))
			} else if isTrendDroppingFast(series.Points, 5) {
				alerts = append(alerts, buildAlert("warning", "lifecycle", series.Metric, series.Name, "disk wearout is dropping over time", &last, nil))
			}
		case "hardDiskStatus", "physicalDiskComponentStatus":
			status := normalizeStatusFromSeriesMetric(series.Metric, last)
			if status == "warning" || status == "critical" {
				alerts = append(alerts, buildAlert(status, "lifecycle", series.Metric, series.Name, "disk status is abnormal", &last, nil))
			}
		}
	}
	return alerts
}

func (s *CmdbIdcAssetHardwareMonitorService) buildThermalAlerts(panels []model.CmdbIdcAssetHardwareTrendPanelVo) []model.CmdbIdcAssetHardwareAlertVo {
	return []model.CmdbIdcAssetHardwareAlertVo{}
}

func (s *CmdbIdcAssetHardwareMonitorService) buildFanHardwareAlerts(monitor *model.CmdbIdcAssetHardwareMonitorVo) []model.CmdbIdcAssetHardwareAlertVo {
	alerts := []model.CmdbIdcAssetHardwareAlertVo{}
	for _, fan := range monitor.Components.Fans {
		if fan.Status != "critical" {
			continue
		}
		alerts = append(alerts, buildAlert("critical", "thermal", fan.RawMetric, fan.Name, "fan hardware status is abnormal", nil, nil))
	}
	if monitor.Summary.FanStatus == "critical" {
		alerts = append(alerts, buildAlert("critical", "thermal", "fanStatus", "", "fan hardware status is abnormal", nil, nil))
	}
	return alerts
}

func (s *CmdbIdcAssetHardwareMonitorService) buildRaidAlerts(panels []model.CmdbIdcAssetHardwareTrendPanelVo, monitor *model.CmdbIdcAssetHardwareMonitorVo) []model.CmdbIdcAssetHardwareAlertVo {
	alerts := []model.CmdbIdcAssetHardwareAlertVo{}
	panel := findTrendPanel(panels, "raid-health")
	if panel != nil {
		for _, series := range panel.Series {
			last, ok := lastTrendPoint(series.Points)
			if !ok {
				continue
			}
			switch series.Metric {
			case "raidControllerDDREccCount":
				if last > 0 && isTrendIncreasing(series.Points) {
					alerts = append(alerts, buildAlert("warning", "error", series.Metric, series.Name, "RAID ECC count is increasing", &last, nil))
				}
			case "hardDiskStatus", "physicalDiskComponentStatus", "raidControllerHealthStatus", "controllerComponentStatus", "logicalDriveConsistencyCheckStatus":
				status := normalizeStatusFromSeriesMetric(series.Metric, last)
				if status == "warning" || status == "critical" {
					alerts = append(alerts, buildAlert(status, "error", series.Metric, series.Name, "RAID or storage path status is abnormal", &last, nil))
				}
			}
		}
	}
	if monitor.Summary.DiskStatus == "critical" || monitor.Summary.DiskStatus == "warning" {
		alerts = append(alerts, buildAlert(monitor.Summary.DiskStatus, "error", "diskStatus", "", "physical disk status is abnormal", nil, nil))
	}
	return alerts
}

func (s *CmdbIdcAssetHardwareMonitorService) buildPowerAlerts(panels []model.CmdbIdcAssetHardwareTrendPanelVo, monitor *model.CmdbIdcAssetHardwareMonitorVo) []model.CmdbIdcAssetHardwareAlertVo {
	alerts := []model.CmdbIdcAssetHardwareAlertVo{}
	panel := findTrendPanel(panels, "power-redundancy")
	if panel != nil {
		for _, series := range panel.Series {
			last, ok := lastTrendPoint(series.Points)
			if !ok {
				continue
			}
			switch series.Metric {
			case "powerSupplyStatus", "powerSupplySensorState":
				status := normalizeStatusFromSeriesMetric(series.Metric, last)
				if status == "warning" || status == "critical" {
					alerts = append(alerts, buildAlert(status, "power", series.Metric, series.Name, "power supply redundancy is abnormal", &last, nil))
				}
			}
		}
	}
	if monitor.Summary.PowerStatus == "warning" || monitor.Summary.PowerStatus == "critical" {
		alerts = append(alerts, buildAlert(monitor.Summary.PowerStatus, "power", "powerStatus", "", "power status is abnormal", nil, nil))
	}
	return alerts
}

func (s *CmdbIdcAssetHardwareMonitorService) buildMemoryAlerts(panels []model.CmdbIdcAssetHardwareTrendPanelVo, monitor *model.CmdbIdcAssetHardwareMonitorVo) []model.CmdbIdcAssetHardwareAlertVo {
	alerts := []model.CmdbIdcAssetHardwareAlertVo{}
	panel := findTrendPanel(panels, "memory-stability")
	if panel != nil {
		for _, series := range panel.Series {
			last, ok := lastTrendPoint(series.Points)
			if !ok {
				continue
			}
			if series.Metric == "memoryStatus" {
				status := normalizeStatusFromSeriesMetric(series.Metric, last)
				if status == "warning" || status == "critical" {
					alerts = append(alerts, buildAlert(status, "stability", series.Metric, series.Name, "memory DIMM status is abnormal", &last, nil))
				}
			}
		}
	}
	if monitor.Summary.MemoryStatus == "warning" || monitor.Summary.MemoryStatus == "critical" {
		alerts = append(alerts, buildAlert(monitor.Summary.MemoryStatus, "stability", "memoryStatus", "", "memory status is abnormal", nil, nil))
	}
	return alerts
}

func normalizeTrendWindow(trendHours int, trendStepMinutes int) (int, int) {
	if trendHours <= 0 {
		trendHours = 24
	}
	if trendHours > 24*14 {
		trendHours = 24 * 14
	}
	if trendStepMinutes <= 0 {
		trendStepMinutes = 15
	}
	if trendStepMinutes < 1 {
		trendStepMinutes = 1
	}
	if trendStepMinutes > 60 {
		trendStepMinutes = 60
	}
	return trendHours, trendStepMinutes
}

func metricRangeQuery(metric string, match *targetMatch, extraSelector string) string {
	if strings.TrimSpace(extraSelector) != "" {
		return fmt.Sprintf(`%s{%s=%q,%s}`, metric, match.label, match.target, extraSelector)
	}
	return fmt.Sprintf(`%s{%s=%q}`, metric, match.label, match.target)
}

func dellPowerWattsRangeQuery(match *targetMatch) string {
	return fmt.Sprintf(`amperageProbeReading{%s=%q,amperageProbeIndex="3"}`, match.label, match.target)
}

func normalizeTrendValue(vendor, metric string, value float64) float64 {
	switch metric {
	case "temperatureReading", "temperatureProbeReading":
		return normalizeTemperature(vendor, value)
	case "hardDiskPowerOnHours":
		if value < 0 {
			return roundFloat(-value, 2)
		}
	}
	return roundFloat(value, 2)
}

func skipTrendPoint(metric string, value float64) bool {
	switch metric {
	case "hardDiskRemnantWearout", "physicalDiskRemainingRatedWriteEndurance":
		return value <= 0 || value >= 255
	case "hardDiskPowerOnHours":
		return value <= 0
	case "hardDiskTemperature":
		return value >= 255
	case "hardDiskStatus":
		return value <= 0 || value >= 5
	case "memoryStatus":
		return value <= 0 || value >= 5
	case "temperatureReading", "temperatureProbeReading":
		return value <= 0 || value >= 100
	case "raidControllerHealthStatus":
		return value <= 0
	}
	return false
}

func inferTrendValueType(metric string) string {
	switch metric {
	case "hardDiskStatus", "physicalDiskComponentStatus", "memoryStatus", "powerSupplyStatus", "powerSupplySensorState", "raidControllerHealthStatus", "controllerComponentStatus", "logicalDriveConsistencyCheckStatus":
		return "status"
	case "raidControllerDDREccCount":
		return "counter"
	default:
		return "gauge"
	}
}

func normalizeTrendPanelsForDisplay(panels []model.CmdbIdcAssetHardwareTrendPanelVo) []model.CmdbIdcAssetHardwareTrendPanelVo {
	for pi := range panels {
		for si := range panels[pi].Series {
			series := &panels[pi].Series[si]
			if series.ValueType != "status" {
				continue
			}
			if strings.TrimSpace(series.Unit) == "" {
				series.Unit = "状态码(1=正常,2=告警,3=严重)"
			}
			for i := range series.Points {
				series.Points[i].Value = normalizeStatusCodeForMetric(series.Metric, series.Points[i].Value)
			}
		}
	}
	return panels
}

func normalizeStatusCodeForMetric(metric string, value float64) float64 {
	switch normalizeStatusFromSeriesMetric(metric, value) {
	case "normal", "healthy":
		return 1
	case "warning":
		return 2
	case "critical":
		return 3
	default:
		return 0
	}
}

func annotateTrendPanelStatuses(monitor *model.CmdbIdcAssetHardwareMonitorVo) {
	if monitor == nil {
		return
	}
	for i := range monitor.Trends.Panels {
		panel := &monitor.Trends.Panels[i]
		status := "normal"
		if len(panel.Series) == 0 {
			status = "unknown"
		}
		for _, alert := range monitor.Alerts {
			if !panelContainsAlertMetric(*panel, alert) {
				continue
			}
			status = moreSevereStatus(status, alert.Severity)
		}
		panel.Status = status
	}
}

func panelContainsAlertMetric(panel model.CmdbIdcAssetHardwareTrendPanelVo, alert model.CmdbIdcAssetHardwareAlertVo) bool {
	if panel.Key == "raid-health" && alert.Metric == "diskStatus" {
		return true
	}
	for _, series := range panel.Series {
		if series.Metric != alert.Metric {
			continue
		}
		if alert.Component == "" || alert.Component == series.Name || alert.Component == series.Component {
			return true
		}
	}
	return false
}

func sortTrendSeriesByStatusSeverity(series []model.CmdbIdcAssetHardwareTrendSeriesVo) {
	sort.SliceStable(series, func(i, j int) bool {
		return statusSeverityRank(latestSeriesStatus(series[i])) > statusSeverityRank(latestSeriesStatus(series[j]))
	})
}

func latestSeriesStatus(series model.CmdbIdcAssetHardwareTrendSeriesVo) string {
	last, ok := lastTrendPoint(series.Points)
	if !ok {
		return "unknown"
	}
	if series.ValueType == "status" {
		return normalizeStatusFromSeriesMetric(series.Metric, last)
	}
	return "normal"
}

func moreSevereStatus(a, b string) string {
	if statusSeverityRank(b) > statusSeverityRank(a) {
		return b
	}
	return a
}

func statusSeverityRank(status string) int {
	switch status {
	case "critical":
		return 3
	case "warning":
		return 2
	case "normal", "healthy":
		return 1
	default:
		return 0
	}
}

func lastTrendPoint(points []model.TrendPoint) (float64, bool) {
	if len(points) == 0 {
		return 0, false
	}
	return points[len(points)-1].Value, true
}

func isTrendIncreasing(points []model.TrendPoint) bool {
	if len(points) < 2 {
		return false
	}
	return points[len(points)-1].Value > points[0].Value
}

func isTrendDroppingFast(points []model.TrendPoint, delta float64) bool {
	if len(points) < 2 {
		return false
	}
	return points[0].Value-points[len(points)-1].Value >= delta
}

func isDiskPowerOnHoursReset(points []model.TrendPoint) bool {
	if len(points) < 2 {
		return false
	}
	last := points[len(points)-1].Value
	if last > 1 {
		return false
	}
	previousMax := 0.0
	for i := 0; i < len(points)-1; i++ {
		if points[i].Value > previousMax {
			previousMax = points[i].Value
		}
	}
	return previousMax >= 1000
}

func normalizeStatusFromSeriesMetric(metric string, value float64) string {
	if strings.HasPrefix(metric, "ipmi_") {
		return normalizeStatus("ipmi", metric, value)
	}
	switch metric {
	case "physicalDiskComponentStatus", "controllerComponentStatus", "powerSupplySensorState":
		return normalizeStatus("dell", metric, value)
	default:
		return normalizeStatus("huawei", metric, value)
	}
}

func buildAlert(severity, category, metric, component, message string, currentValue *float64, threshold *float64) model.CmdbIdcAssetHardwareAlertVo {
	return model.CmdbIdcAssetHardwareAlertVo{
		Severity:       severity,
		Category:       category,
		Metric:         metric,
		Component:      component,
		Message:        message,
		CurrentValue:   currentValue,
		Threshold:      threshold,
		Recommendation: defaultRecommendationCN(category, metric),
	}
}

func findTrendPanel(panels []model.CmdbIdcAssetHardwareTrendPanelVo, key string) *model.CmdbIdcAssetHardwareTrendPanelVo {
	for i := range panels {
		if panels[i].Key == key {
			return &panels[i]
		}
	}
	return nil
}

func defaultRecommendation(category, metric string) string {
	switch category {
	case "lifecycle":
		return "review disk age and wear, verify RAID redundancy, and prepare replacement if the trend continues"
	case "thermal":
		return "检查气流、风扇模块、积尘情况，以及近期环境温度变化"
	case "error":
		return "review RAID controller logs and storage events, then schedule maintenance if the trend persists"
	case "power":
		return "check PSU redundancy, input power, and whether both supplies are seated and loaded correctly"
	case "stability":
		return "inspect DIMM and controller cache health, then review recent hardware logs"
	default:
		return "review recent hardware metrics and plan manual inspection if the condition persists"
	}
}

func defaultRecommendationCN(category, metric string) string {
	switch category {
	case "lifecycle":
		return "检查磁盘服役年限与磨损情况，核对RAID冗余，并在趋势持续恶化时准备更换。"
	case "thermal":
		return "检查气流、风扇模块、积尘情况，以及近期环境温度变化。"
	case "error":
		return "检查RAID控制器日志和存储事件，若趋势持续请安排维护。"
	case "power":
		return "检查PSU冗余与输入供电，并确认两路电源模块已正确插入且负载正常。"
	case "stability":
		return "检查DIMM与控制器缓存健康状态，并复核近期硬件日志。"
	default:
		return "检查近期硬件指标，若异常持续请安排人工巡检。"
	}
}

func enrichAlertPersistence(alerts []model.CmdbIdcAssetHardwareAlertVo, panels []model.CmdbIdcAssetHardwareTrendPanelVo, stepMinutes int) []model.CmdbIdcAssetHardwareAlertVo {
	if len(alerts) == 0 {
		return alerts
	}

	for i := range alerts {
		series := findTrendSeries(panels, alerts[i].Metric, alerts[i].Component)
		if series == nil || len(series.Points) == 0 {
			continue
		}
		alerts[i].DurationMinutes = sustainedDurationMinutes(*series, alerts[i].Metric, alerts[i].Severity, stepMinutes)
		alerts[i].Evidence = buildAlertEvidence(*series, alerts[i].Metric, alerts[i].Severity, alerts[i].DurationMinutes)
	}

	return alerts
}

func dedupeHardwareAlerts(alerts []model.CmdbIdcAssetHardwareAlertVo) []model.CmdbIdcAssetHardwareAlertVo {
	if len(alerts) == 0 {
		return alerts
	}

	order := map[string]int{
		"critical": 0,
		"warning":  1,
		"info":     2,
		"healthy":  3,
		"normal":   4,
	}
	best := map[string]model.CmdbIdcAssetHardwareAlertVo{}
	for _, alert := range alerts {
		key := alert.Category + "|" + alert.Metric + "|" + alert.Component + "|" + alert.Message
		existing, ok := best[key]
		if !ok || order[alert.Severity] < order[existing.Severity] || alert.DurationMinutes > existing.DurationMinutes {
			best[key] = alert
		}
	}

	merged := make([]model.CmdbIdcAssetHardwareAlertVo, 0, len(best))
	for _, alert := range best {
		merged = append(merged, alert)
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].Severity != merged[j].Severity {
			return order[merged[i].Severity] < order[merged[j].Severity]
		}
		if merged[i].Category != merged[j].Category {
			return merged[i].Category < merged[j].Category
		}
		if merged[i].DurationMinutes != merged[j].DurationMinutes {
			return merged[i].DurationMinutes > merged[j].DurationMinutes
		}
		return merged[i].Message < merged[j].Message
	})
	return merged
}

func buildHardwareHealth(alerts []model.CmdbIdcAssetHardwareAlertVo, panelCount int) model.CmdbIdcAssetHardwareHealthVo {
	health := model.CmdbIdcAssetHardwareHealthVo{
		Score:           100,
		Level:           "healthy",
		Conclusion:      "硬件指标正常",
		RiskCategories:  []string{},
		Recommendations: []string{},
	}
	if panelCount == 0 {
		health.Level = "unknown"
		health.Conclusion = "暂无趋势数据，暂时无法给出健康评估。"
		return health
	}

	categorySet := map[string]struct{}{}
	recommendationSet := map[string]struct{}{}
	criticalDiskCount := 0
	criticalDiskResetCount := 0
	for _, alert := range alerts {
		switch alert.Severity {
		case "critical":
			health.CriticalCount++
			health.Score -= 25
			health.ImmediateAction = true
		case "warning":
			health.WarningCount++
			health.Score -= 10
		default:
			health.InfoCount++
			health.Score -= 2
		}
		if alert.Category != "" {
			categorySet[alert.Category] = struct{}{}
		}
		if alert.Recommendation != "" {
			recommendationSet[alert.Recommendation] = struct{}{}
		}
		if alert.DurationMinutes >= 180 && alert.Severity == "warning" {
			health.Score -= 5
		}
		if alert.Severity == "critical" && (alert.Metric == "hardDiskStatus" || alert.Metric == "physicalDiskComponentStatus" || alert.Metric == "diskStatus") {
			criticalDiskCount++
		}
		if alert.Severity == "critical" && alert.Metric == "hardDiskPowerOnHours" {
			criticalDiskResetCount++
		}
	}

	if health.Score < 0 {
		health.Score = 0
	}
	for category := range categorySet {
		health.RiskCategories = append(health.RiskCategories, category)
	}
	sort.Strings(health.RiskCategories)
	for recommendation := range recommendationSet {
		health.Recommendations = append(health.Recommendations, recommendation)
	}
	sort.Strings(health.Recommendations)
	totalRisks := health.CriticalCount + health.WarningCount + health.InfoCount

	switch {
	case criticalDiskCount >= 2 || criticalDiskResetCount >= 2:
		health.Level = "critical"
		health.ImmediateAction = true
		health.Conclusion = fmt.Sprintf("存在 %d 条严重告警（共 %d 条风险项），多块磁盘或存储寿命指标同时异常，疑似存储/系统级故障，可能导致系统不可用，请立即排查。", health.CriticalCount, totalRisks)
	case health.CriticalCount > 0 || health.Score < 50:
		health.Level = "critical"
		health.Conclusion = fmt.Sprintf("存在 %d 条严重告警（共 %d 条风险项），硬件风险较高，建议立即排查。", health.CriticalCount, totalRisks)
	case health.WarningCount > 0 || health.Score < 80:
		health.Level = "warning"
		health.Conclusion = fmt.Sprintf("存在 %d 条风险项，硬件出现早期预警信号，建议尽快排查。", totalRisks)
	default:
		health.Level = "healthy"
		health.Conclusion = "硬件运行稳定，未发现明显故障趋势。"
	}

	return health
}

func applyCriticalStorageSummary(monitor *model.CmdbIdcAssetHardwareMonitorVo) {
	criticalDiskCount := 0
	criticalDiskResetCount := 0
	for _, alert := range monitor.Alerts {
		if alert.Severity != "critical" {
			continue
		}
		switch alert.Metric {
		case "hardDiskStatus", "physicalDiskComponentStatus", "diskStatus":
			criticalDiskCount++
		case "hardDiskPowerOnHours":
			criticalDiskResetCount++
		}
	}
	if criticalDiskCount >= 2 || criticalDiskResetCount >= 2 {
		monitor.Summary.DiskStatus = "critical"
		monitor.Summary.OverallStatus = "critical"
	}
}

func findTrendSeries(panels []model.CmdbIdcAssetHardwareTrendPanelVo, metric string, component string) *model.CmdbIdcAssetHardwareTrendSeriesVo {
	for i := range panels {
		for j := range panels[i].Series {
			series := &panels[i].Series[j]
			if series.Metric != metric {
				continue
			}
			if component != "" && series.Name != component && series.Component != component {
				continue
			}
			return series
		}
	}
	return nil
}

func sustainedDurationMinutes(series model.CmdbIdcAssetHardwareTrendSeriesVo, metric string, severity string, stepMinutes int) int {
	count := 0
	for i := len(series.Points) - 1; i >= 0; i-- {
		if !pointMatchesSeverity(metric, series.Points[i].Value, severity) {
			break
		}
		count++
	}
	return count * stepMinutes
}

func pointMatchesSeverity(metric string, value float64, severity string) bool {
	switch metric {
	case "hardDiskRemnantWearout", "physicalDiskRemainingRatedWriteEndurance":
		if severity == "critical" {
			return value <= 20
		}
		return value <= 30
	case "hardDiskPowerOnHours":
		if severity == "critical" {
			return value <= 1
		}
		return value >= 50000
	case "fanSpeed", "coolingDeviceReading":
		return value >= 5000
	case "temperatureReading", "temperatureProbeReading":
		if severity == "critical" {
			return value >= 55
		}
		return value >= 45
	case "raidControllerDDREccCount":
		return value > 0
	case "memoryStatus", "hardDiskStatus", "physicalDiskComponentStatus", "powerSupplyStatus", "powerSupplySensorState", "raidControllerHealthStatus", "controllerComponentStatus", "logicalDriveConsistencyCheckStatus":
		status := normalizeStatusFromSeriesMetric(metric, value)
		return status == severity || (severity == "warning" && status == "critical")
	default:
		return false
	}
}

func buildAlertEvidence(series model.CmdbIdcAssetHardwareTrendSeriesVo, metric string, severity string, durationMinutes int) string {
	first, ok1 := firstTrendPoint(series.Points)
	last, ok2 := lastTrendPoint(series.Points)
	if !ok1 || !ok2 {
		return ""
	}
	change := roundFloat(last-first, 2)
	switch metric {
	case "hardDiskRemnantWearout", "physicalDiskRemainingRatedWriteEndurance":
		return fmt.Sprintf("remaining life %.2f%s, change %.2f over %d minutes", last, series.Unit, change, durationMinutes)
	case "hardDiskPowerOnHours":
		if severity == "critical" {
			return fmt.Sprintf("power-on hours collapsed from %.2f%s to %.2f%s over %d minutes", first, series.Unit, last, series.Unit, durationMinutes)
		}
		return fmt.Sprintf("power-on hours %.2f%s, sustained for %d minutes", last, series.Unit, durationMinutes)
	case "fanSpeed", "coolingDeviceReading", "temperatureReading", "temperatureProbeReading":
		return fmt.Sprintf("current %.2f%s, change %.2f over %d minutes", last, series.Unit, change, durationMinutes)
	default:
		return fmt.Sprintf("latest %.2f, sustained for %d minutes", last, durationMinutes)
	}
}

func firstTrendPoint(points []model.TrendPoint) (float64, bool) {
	if len(points) == 0 {
		return 0, false
	}
	return points[0].Value, true
}

func normalizeManagementTargets(remoteManagementCard string) []string {
	raw := strings.TrimSpace(remoteManagementCard)
	if raw == "" {
		return nil
	}
	seen := map[string]struct{}{}
	appendCandidate := func(value string, values *[]string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		*values = append(*values, value)
	}

	values := make([]string, 0, 4)
	appendCandidate(raw, &values)

	host := raw
	if parsed, err := url.Parse(raw); err == nil && parsed.Host != "" {
		host = parsed.Host
	}
	appendCandidate(host, &values)

	withoutPort := host
	if strings.Contains(host, ":") {
		if parsedHost, _, err := net.SplitHostPort(host); err == nil {
			withoutPort = parsedHost
		}
	}
	appendCandidate(withoutPort, &values)
	if withoutPort != "" {
		appendCandidate(withoutPort+":161", &values)
	}

	return values
}

func buildPrometheusBaseURL(account configModel.AccountAuth) string {
	host := strings.TrimSpace(strings.TrimRight(account.Host, "/"))
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return host
	}
	if strings.Contains(host, "/") {
		return "http://" + host
	}
	if _, _, err := net.SplitHostPort(host); err == nil {
		return "http://" + host
	}
	if account.Port == 443 {
		return fmt.Sprintf("https://%s", host)
	}
	if account.Port > 0 {
		return fmt.Sprintf("http://%s:%d", host, account.Port)
	}
	return fmt.Sprintf("http://%s:%d", host, 9090)
}

func valueFromVector(vector []interface{}) float64 {
	if len(vector) < 2 {
		return 0
	}
	value, ok := vector[1].(string)
	if !ok {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func temperatureMaxQuery(match *targetMatch, lookbackMinutes int) string {
	if match.vendor.vendor == "huawei" {
		return fmt.Sprintf(`max(last_over_time(temperatureReading{%s=%q}[%dm]))`, match.label, match.target, lookbackMinutes)
	}
	if match.vendor.vendor == "dell" {
		return fmt.Sprintf(`max(last_over_time(temperatureProbeReading{%s=%q}[%dm]))`, match.label, match.target, lookbackMinutes)
	}
	return ""
}

func normalizeStatus(vendor, metric string, value float64) string {
	switch vendor {
	case "huawei":
		if metric == "hardDiskStatus" {
			if value == 1 || value == 2 {
				return "normal"
			}
			if value >= 3 {
				return "critical"
			}
		}
		if value == 1 || value == 2 {
			return "normal"
		}
		if value == 3 || value == 4 {
			return "warning"
		}
		if value > 4 {
			return "critical"
		}
	case "dell":
		if metric == "powerSupplySensorState" {
			if value == 1 {
				return "normal"
			}
			if value == 2 || value == 4 {
				return "warning"
			}
			if value >= 3 {
				return "critical"
			}
		}
		if value == 3 {
			return "normal"
		}
		if value == 1 || value == 2 || value == 4 {
			return "warning"
		}
		if value >= 5 {
			return "critical"
		}
	case "ipmi":
		if value != value {
			return "unknown"
		}
		if value == 0 {
			return "normal"
		}
		if value == 1 {
			return "warning"
		}
		if value >= 2 {
			return "critical"
		}
	}
	if value == 0 {
		return "unknown"
	}
	return "unknown"
}

func normalizeTemperature(vendor string, value float64) float64 {
	return normalizeTemperatureMetric(vendor, "temperatureReading", value)
}

func normalizeTemperatureMetric(vendor, metric string, value float64) float64 {
	normalized, ok := normalizeTemperatureMetricValue(vendor, metric, value)
	if !ok {
		return 0
	}
	return normalized
}

func normalizeTemperatureMetricValue(vendor, metric string, value float64) (float64, bool) {
	switch metric {
	case "temperatureReading", "temperatureProbeReading":
		if vendor == "huawei" || vendor == "dell" {
			if value <= 0 || value >= 65535 {
				return 0, false
			}
			normalized := roundFloat(value/10, 2)
			if normalized <= 0 || normalized >= 100 {
				return 0, false
			}
			return normalized, true
		}
	case "hardDiskTemperature":
		return roundFloat(value, 2), true
	}
	return roundFloat(value, 2), true
}

func componentIndex(metric map[string]string, preferredKey string) string {
	if preferredKey != "" {
		if value := strings.TrimSpace(metric[preferredKey]); value != "" {
			return value
		}
	}
	for key, value := range metric {
		if strings.HasSuffix(strings.ToLower(key), "index") || strings.HasSuffix(strings.ToLower(key), "number") {
			if strings.TrimSpace(value) != "" {
				return value
			}
		}
	}
	return ""
}

func shouldSkipComponent(vendor, rawMetric string, component model.CmdbIdcAssetHardwareComponentVo, value *float64) bool {
	switch rawMetric {
	case "temperatureReading", "temperatureProbeReading":
		if value == nil {
			return true
		}
		return *value <= 0 || *value >= 100
	}

	if vendor != "huawei" {
		return false
	}

	switch rawMetric {
	case "hardDiskStatus":
		if value == nil {
			return false
		}
		return *value >= 255 || *value <= 0 || *value > 120
	case "memoryStatus":
		return component.Status == "critical"
	case "raidControllerHealthStatus":
		return component.Status == "unknown"
	}

	return false
}

func normalizeComponentValue(vendor, rawMetric, unit string, value float64) (float64, bool) {
	if rawMetric == "temperatureReading" || rawMetric == "temperatureProbeReading" {
		return normalizeTemperatureMetricValue(vendor, rawMetric, value)
	}
	if strings.Contains(unit, "C") || strings.Contains(unit, "°") {
		return roundFloat(value, 2), true
	}
	return roundFloat(value, 2), true
}

func filterHuaweiDiskComponents(items []model.CmdbIdcAssetHardwareComponentVo) []model.CmdbIdcAssetHardwareComponentVo {
	filtered := make([]model.CmdbIdcAssetHardwareComponentVo, 0, len(items))
	for _, item := range items {
		if item.Value != nil && (*item.Value <= 0 || *item.Value >= 120) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func (s *CmdbIdcAssetHardwareMonitorService) fetchHighestTemperature(account *prometheusAccount, match *targetMatch, lookbackMinutes int) (float64, bool, error) {
	query, metricName := temperatureVectorQuery(match, lookbackMinutes)
	if query == "" || metricName == "" {
		return 0, false, nil
	}
	response, err := s.queryPrometheusInstant(account, query)
	if err != nil {
		return 0, false, err
	}
	highest := 0.0
	found := false
	for _, resultItem := range response.Data.Result {
		normalized, ok := normalizeTemperatureMetricValue(match.vendor.vendor, metricName, valueFromVector(resultItem.Value))
		if !ok {
			continue
		}
		if !found || normalized > highest {
			highest = normalized
			found = true
		}
	}
	return highest, found, nil
}

func temperatureVectorQuery(match *targetMatch, lookbackMinutes int) (string, string) {
	switch match.vendor.vendor {
	case "huawei":
		return fmt.Sprintf(`last_over_time(temperatureReading{%s=%q}[%dm])`, match.label, match.target, lookbackMinutes), "temperatureReading"
	case "dell":
		return fmt.Sprintf(`last_over_time(temperatureProbeReading{%s=%q}[%dm])`, match.label, match.target, lookbackMinutes), "temperatureProbeReading"
	default:
		return "", ""
	}
}

func filterHuaweiDiskTrendSeries(lifeSeries, wearSeries, statusSeries []model.CmdbIdcAssetHardwareTrendSeriesVo) ([]model.CmdbIdcAssetHardwareTrendSeriesVo, []model.CmdbIdcAssetHardwareTrendSeriesVo, []model.CmdbIdcAssetHardwareTrendSeriesVo) {
	if len(lifeSeries) == 0 {
		return lifeSeries, wearSeries, statusSeries
	}

	allowed := map[string]struct{}{}
	for _, series := range lifeSeries {
		if huaweiDiskLifeSeriesInstalled(series) {
			key := series.Component
			if key == "" {
				key = series.Name
			}
			allowed[key] = struct{}{}
		}
	}
	if len(allowed) == 0 {
		return []model.CmdbIdcAssetHardwareTrendSeriesVo{}, []model.CmdbIdcAssetHardwareTrendSeriesVo{}, []model.CmdbIdcAssetHardwareTrendSeriesVo{}
	}

	return filterTrendSeriesByComponents(lifeSeries, allowed), filterTrendSeriesByComponents(wearSeries, allowed), filterTrendSeriesByComponents(statusSeries, allowed)
}

func huaweiDiskLifeSeriesInstalled(series model.CmdbIdcAssetHardwareTrendSeriesVo) bool {
	if series.Metric != "hardDiskPowerOnHours" || len(series.Points) == 0 {
		return false
	}
	for _, point := range series.Points {
		if point.Value > 1 {
			return true
		}
	}
	return false
}

func filterTrendSeriesByComponents(series []model.CmdbIdcAssetHardwareTrendSeriesVo, allowed map[string]struct{}) []model.CmdbIdcAssetHardwareTrendSeriesVo {
	filtered := make([]model.CmdbIdcAssetHardwareTrendSeriesVo, 0, len(series))
	for _, item := range series {
		key := item.Component
		if key == "" {
			key = item.Name
		}
		if _, ok := allowed[key]; !ok {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func buildComponentName(prefix, index string) string {
	if index == "" {
		return prefix
	}
	return fmt.Sprintf("%s %s", prefix, index)
}

func decodeHexMetricLabel(value string) string {
	value = strings.TrimSpace(value)
	if !strings.HasPrefix(strings.ToLower(value), "0x") {
		return value
	}
	decoded, err := hex.DecodeString(value[2:])
	if err != nil {
		return value
	}
	return string(decoded)
}

func (s *CmdbIdcAssetHardwareMonitorService) lookupAssetByRemoteManagementCard(remoteManagementCard string) (*model.CmdbIdcAsset, error) {
	asset, err := s.assetDao.GetAssetByRemoteManagementCard(remoteManagementCard)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return asset, nil
}
