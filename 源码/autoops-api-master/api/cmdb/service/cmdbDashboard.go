package service

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	cmdbDao "dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/model"
	"dodevops-api/common"
	"dodevops-api/common/config"

	"gorm.io/gorm"
)

type CmdbDashboardServiceInterface interface {
	GetCards(windowDays int) (*model.CmdbDashboardCards, error)
	GetCharts() (*model.CmdbDashboardCharts, error)
	GetTables(limit, hardwareLimit, lookbackMinutes int) (*model.CmdbDashboardTables, error)
}

type CmdbDashboardServiceImpl struct {
	db *gorm.DB
}

func GetCmdbDashboardService() CmdbDashboardServiceInterface {
	return &CmdbDashboardServiceImpl{db: common.GetDB()}
}

func (s *CmdbDashboardServiceImpl) GetCards(windowDays int) (*model.CmdbDashboardCards, error) {
	if windowDays <= 0 {
		windowDays = 90
	}
	if windowDays > 365 {
		windowDays = 365
	}

	var physicalTotal, vmTotal, switchTotal int64
	var roomTotal, cabinetTotal int64
	var hardwareCovered int64
	var hostTotal, authenticatedTotal, unauthenticatedTotal, failedTotal int64
	var agentTotal, runningAgentTotal int64
	var aliyunTotal, tencentTotal int64

	if err := s.db.Model(&model.CmdbIdcAsset{}).Count(&physicalTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.CmdbIdcVm{}).Count(&vmTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.CmdbSnmpDevice{}).
		Where("LOWER(TRIM(device_type)) = ?", "switch").
		Count(&switchTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.CmdbIdcRoom{}).Count(&roomTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.CmdbIdcCabinet{}).Count(&cabinetTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.CmdbIdcAsset{}).
		Where("TRIM(remote_management_card) <> ''").
		Count(&hardwareCovered).Error; err != nil {
		return nil, err
	}

	if err := s.db.Model(&model.CmdbHost{}).Count(&hostTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.CmdbHost{}).Where("status = ?", 1).Count(&authenticatedTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.CmdbHost{}).Where("status = ?", 2).Count(&unauthenticatedTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.CmdbHost{}).Where("status = ?", 3).Count(&failedTotal).Error; err != nil {
		return nil, err
	}

	if err := s.db.Model(&model.Agent{}).Count(&agentTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.Agent{}).
		Where("status = ?", model.AgentStatusRunning).
		Count(&runningAgentTotal).Error; err != nil {
		return nil, err
	}

	if err := s.db.Model(&model.CmdbHost{}).Where("vendor = ?", 2).Count(&aliyunTotal).Error; err != nil {
		return nil, err
	}
	if err := s.db.Model(&model.CmdbHost{}).Where("vendor = ?", 3).Count(&tencentTotal).Error; err != nil {
		return nil, err
	}

	warranty, err := s.buildWarrantyCard(windowDays, physicalTotal)
	if err != nil {
		return nil, err
	}

	return &model.CmdbDashboardCards{
		Asset: model.CmdbDashboardAssetCard{
			PhysicalTotal: physicalTotal,
			VmTotal:       vmTotal,
			SwitchTotal:   switchTotal,
			Total:         physicalTotal + vmTotal + switchTotal,
		},
		Location: model.CmdbDashboardLocationCard{
			RoomTotal:    roomTotal,
			CabinetTotal: cabinetTotal,
		},
		Monitor: model.CmdbDashboardMonitorCard{
			HardwareCovered:       hardwareCovered,
			HardwareTotal:         physicalTotal,
			HardwareCoverageRate:  dashboardRate(hardwareCovered, physicalTotal),
			SystemCovered:         runningAgentTotal,
			SystemTotal:           hostTotal,
			SystemCoverageRate:    dashboardRate(runningAgentTotal, hostTotal),
			AgentTotal:            agentTotal,
			RunningAgentTotal:     runningAgentTotal,
			AgentInstallCoverRate: dashboardRate(agentTotal, hostTotal),
		},
		Warranty: warranty,
		CloudHost: model.CmdbDashboardCloudHostCard{
			AliyunTotal:  aliyunTotal,
			TencentTotal: tencentTotal,
			Total:        aliyunTotal + tencentTotal,
		},
		Auth: model.CmdbDashboardAuthCard{
			AuthenticatedTotal:   authenticatedTotal,
			UnauthenticatedTotal: unauthenticatedTotal,
			FailedTotal:          failedTotal,
			HostTotal:            hostTotal,
			AuthRate:             dashboardRate(authenticatedTotal, hostTotal),
		},
		UpdateTime: time.Now().Unix(),
	}, nil
}

func (s *CmdbDashboardServiceImpl) GetCharts() (*model.CmdbDashboardCharts, error) {
	type assetFields struct {
		Brand   string
		Purpose string
	}

	var assets []assetFields
	if err := s.db.Model(&model.CmdbIdcAsset{}).
		Select("brand, purpose").
		Find(&assets).Error; err != nil {
		return nil, err
	}

	brandCounter := make(map[string]int64)
	purposeCounter := make(map[string]int64)
	for _, asset := range assets {
		brandCounter[dashboardNormalizeLabel(asset.Brand)]++
		purposeCounter[dashboardNormalizeLabel(asset.Purpose)]++
	}

	roomBars, err := s.buildRoomPhysicalAssetBars()
	if err != nil {
		return nil, err
	}

	return &model.CmdbDashboardCharts{
		BrandDistribution:   buildDashboardChartPoints(brandCounter),
		RoomPhysicalAssets:  roomBars,
		PurposeDistribution: buildDashboardChartPoints(purposeCounter),
		UpdateTime:          time.Now().Unix(),
	}, nil
}

func (s *CmdbDashboardServiceImpl) GetTables(limit, hardwareLimit, lookbackMinutes int) (*model.CmdbDashboardTables, error) {
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}
	if hardwareLimit <= 0 {
		hardwareLimit = 20
	}
	if hardwareLimit > 100 {
		hardwareLimit = 100
	}
	if lookbackMinutes <= 0 {
		lookbackMinutes = 5
	}
	if lookbackMinutes > 15 {
		lookbackMinutes = 15
	}

	highest, lowest, usageWarnings := s.buildCurrentUsageTables(limit)
	hardwareExceptions, hardwareScan, hardwareWarnings := s.buildHardwareExceptionTable(hardwareLimit, lookbackMinutes)
	warnings := append(usageWarnings, hardwareWarnings...)

	return &model.CmdbDashboardTables{
		HighestUsage:       highest,
		LowestUsage:        lowest,
		HardwareExceptions: hardwareExceptions,
		HardwareScan:       hardwareScan,
		Warnings:           warnings,
		UpdateTime:         time.Now().Unix(),
	}, nil
}

func (s *CmdbDashboardServiceImpl) buildWarrantyCard(windowDays int, physicalTotal int64) (model.CmdbDashboardWarrantyCard, error) {
	type warrantyRow struct {
		WarrantyEndDate string
	}

	var rows []warrantyRow
	if err := s.db.Model(&model.CmdbIdcAsset{}).
		Select("warranty_end_date").
		Find(&rows).Error; err != nil {
		return model.CmdbDashboardWarrantyCard{}, err
	}

	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	deadline := today.AddDate(0, 0, windowDays)

	var expiredCount, expiringSoonCount int64
	for _, row := range rows {
		endDate, ok := parseDashboardDate(row.WarrantyEndDate, loc)
		if !ok {
			continue
		}
		if endDate.Before(today) {
			expiredCount++
			continue
		}
		if !endDate.After(deadline) {
			expiringSoonCount++
		}
	}

	dueCount := expiredCount + expiringSoonCount
	return model.CmdbDashboardWarrantyCard{
		WindowDays:        windowDays,
		DueCount:          dueCount,
		DueRate:           dashboardRate(dueCount, physicalTotal),
		ExpiredCount:      expiredCount,
		ExpiringSoonCount: expiringSoonCount,
		Total:             physicalTotal,
		Deadline:          deadline.Format("2006-01-02"),
	}, nil
}

func (s *CmdbDashboardServiceImpl) buildRoomPhysicalAssetBars() ([]model.CmdbDashboardRoomBar, error) {
	type roomBarRow struct {
		RoomID       uint
		RoomName     string
		AssetCount   int64
		CabinetCount int64
	}

	var rows []roomBarRow
	if err := s.db.Table("cmdb_idc_room r").
		Select("r.id AS room_id, r.name AS room_name, COUNT(DISTINCT a.id) AS asset_count, COUNT(DISTINCT c.id) AS cabinet_count").
		Joins("LEFT JOIN cmdb_idc_asset a ON a.room_id = r.id").
		Joins("LEFT JOIN cmdb_idc_cabinet c ON c.room_id = r.id").
		Group("r.id, r.name").
		Order("asset_count DESC, r.name ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	var total int64
	for _, row := range rows {
		total += row.AssetCount
	}

	result := make([]model.CmdbDashboardRoomBar, 0, len(rows))
	for _, row := range rows {
		result = append(result, model.CmdbDashboardRoomBar{
			RoomID:       row.RoomID,
			RoomName:     row.RoomName,
			AssetCount:   row.AssetCount,
			CabinetCount: row.CabinetCount,
			Rate:         dashboardRate(row.AssetCount, total),
		})
	}
	return result, nil
}

func (s *CmdbDashboardServiceImpl) buildCurrentUsageTables(limit int) (model.CmdbDashboardUsageRankings, model.CmdbDashboardUsageRankings, []string) {
	highest := model.CmdbDashboardUsageRankings{}
	lowest := model.CmdbDashboardUsageRankings{}
	warnings := []string{}

	if config.Config == nil || strings.TrimSpace(config.Config.Monitor.Prometheus.URL) == "" {
		return highest, lowest, []string{"Prometheus 未配置，无法获取 CPU/内存/磁盘 TOP 数据"}
	}

	monitorSvc := &MonitorServiceImpl{
		hostDao:       cmdbDao.NewCmdbHostDao(),
		agentDao:      cmdbDao.NewAgentDao(),
		prometheusURL: config.Config.Monitor.Prometheus.URL,
		cacheExpire:   cacheTimeout,
	}

	type task struct {
		name      string
		query     string
		ascending bool
		assign    func([]model.CmdbDashboardUsageRankItem)
	}

	tasks := []task{
		{
			name:  "CPU最高TOP",
			query: fmt.Sprintf("topk(%d, system_cpu_usage_percent)", limit),
			assign: func(items []model.CmdbDashboardUsageRankItem) {
				highest.CPU = items
			},
		},
		{
			name:      "CPU最低TOP",
			query:     fmt.Sprintf("bottomk(%d, system_cpu_usage_percent)", limit),
			ascending: true,
			assign: func(items []model.CmdbDashboardUsageRankItem) {
				lowest.CPU = items
			},
		},
		{
			name:  "内存最高TOP",
			query: fmt.Sprintf("topk(%d, system_memory_usage_percent)", limit),
			assign: func(items []model.CmdbDashboardUsageRankItem) {
				highest.Memory = items
			},
		},
		{
			name:      "内存最低TOP",
			query:     fmt.Sprintf("bottomk(%d, system_memory_usage_percent)", limit),
			ascending: true,
			assign: func(items []model.CmdbDashboardUsageRankItem) {
				lowest.Memory = items
			},
		},
		{
			name:  "磁盘最高TOP",
			query: fmt.Sprintf("topk(%d, system_disk_usage_percent{mountpoint=\"/\"})", limit),
			assign: func(items []model.CmdbDashboardUsageRankItem) {
				highest.Disk = items
			},
		},
		{
			name:      "磁盘最低TOP",
			query:     fmt.Sprintf("bottomk(%d, system_disk_usage_percent{mountpoint=\"/\"})", limit),
			ascending: true,
			assign: func(items []model.CmdbDashboardUsageRankItem) {
				lowest.Disk = items
			},
		},
	}

	for _, item := range tasks {
		ranks, err := monitorSvc.queryCurrentUsageRanks(item.query, item.ascending, limit, nil)
		if err != nil {
			warnings = appendDashboardWarning(warnings, fmt.Sprintf("%s 查询失败: %v", item.name, err))
			continue
		}
		item.assign(convertDashboardUsageRanks(ranks))
	}

	return highest, lowest, warnings
}

func (s *CmdbDashboardServiceImpl) buildHardwareExceptionTable(limit, lookbackMinutes int) ([]model.CmdbDashboardHardwareException, model.CmdbDashboardHardwareScan, []string) {
	scan := model.CmdbDashboardHardwareScan{
		LookbackMinutes: lookbackMinutes,
		Limit:           limit,
	}
	warnings := []string{}

	var assets []model.CmdbIdcAsset
	if err := s.db.Preload("Room").Preload("Cabinet").
		Where("TRIM(remote_management_card) <> ''").
		Find(&assets).Error; err != nil {
		return []model.CmdbDashboardHardwareException{}, scan, []string{"硬件资产查询失败: " + err.Error()}
	}
	scan.EnabledAssetTotal = int64(len(assets))
	if len(assets) == 0 {
		return []model.CmdbDashboardHardwareException{}, scan, warnings
	}

	hardwareSvc := NewCmdbIdcAssetHardwareMonitorService()
	account, err := hardwareSvc.resolvePrometheusAccount()
	if err != nil {
		return []model.CmdbDashboardHardwareException{}, scan, []string{"Prometheus 账号不可用，无法聚合硬件异常: " + err.Error()}
	}

	assetByCandidate := make(map[string]*model.CmdbIdcAsset)
	for i := range assets {
		for _, candidate := range normalizeManagementTargets(assets[i].RemoteManagementCard) {
			assetByCandidate[candidate] = &assets[i]
		}
	}

	accumulators := make(map[uint]*dashboardHardwareExceptionAccumulator)
	for _, spec := range hardwareVendorSpecs {
		seenMetrics := map[string]struct{}{}
		for _, metricItem := range dashboardHardwareStatusMetrics(spec) {
			if metricItem.metric == "" {
				continue
			}
			if _, exists := seenMetrics[metricItem.metric]; exists {
				continue
			}
			seenMetrics[metricItem.metric] = struct{}{}

			selector := ""
			if metricItem.job != "" {
				selector = fmt.Sprintf(`job=%q`, metricItem.job)
			}
			query := fmt.Sprintf(`last_over_time(%s{%s}[%dm])`, metricItem.metric, selector, lookbackMinutes)
			response, err := hardwareSvc.queryPrometheusInstant(account, query)
			if err != nil {
				warnings = appendDashboardWarning(warnings, fmt.Sprintf("硬件指标 %s 查询失败: %v", metricItem.metric, err))
				continue
			}

			for _, resultItem := range response.Data.Result {
				value := valueFromVector(resultItem.Value)
				status := normalizeStatus(spec.vendor, metricItem.metric, value)
				if status != "warning" && status != "critical" {
					continue
				}

				matchedAssetIDs := map[uint]struct{}{}
				for _, candidate := range dashboardCandidatesFromMetricLabels(resultItem.Metric) {
					asset, ok := assetByCandidate[candidate]
					if !ok {
						continue
					}
					if _, exists := matchedAssetIDs[asset.ID]; exists {
						continue
					}
					matchedAssetIDs[asset.ID] = struct{}{}
					acc := ensureDashboardHardwareAccumulator(accumulators, asset, spec)
					acc.addMetric(metricItem.label, status)
				}
			}
		}
	}

	exceptions := make([]model.CmdbDashboardHardwareException, 0, len(accumulators))
	for _, acc := range accumulators {
		acc.finish()
		exceptions = append(exceptions, acc.item)
	}
	sort.Slice(exceptions, func(i, j int) bool {
		if exceptions[i].CriticalCount != exceptions[j].CriticalCount {
			return exceptions[i].CriticalCount > exceptions[j].CriticalCount
		}
		if exceptions[i].WarningCount != exceptions[j].WarningCount {
			return exceptions[i].WarningCount > exceptions[j].WarningCount
		}
		return exceptions[i].AssetID < exceptions[j].AssetID
	})

	scan.ExceptionTotal = int64(len(exceptions))
	if len(exceptions) > limit {
		exceptions = exceptions[:limit]
	}
	scan.ReturnedTotal = len(exceptions)

	return exceptions, scan, warnings
}

type dashboardHardwareStatusMetric struct {
	label  string
	metric string
	job    string
}

func dashboardHardwareStatusMetrics(spec vendorMetricSpec) []dashboardHardwareStatusMetric {
	if spec.vendor == "ipmi" {
		return []dashboardHardwareStatusMetric{
			{label: "风扇状态", metric: "ipmi_fan_speed_state", job: spec.requiredJob},
			{label: "电源状态", metric: "ipmi_power_state", job: spec.requiredJob},
			{label: "电流状态", metric: "ipmi_current_state", job: spec.requiredJob},
			{label: "电压状态", metric: "ipmi_voltage_state", job: spec.requiredJob},
		}
	}
	return []dashboardHardwareStatusMetric{
		{label: "整机状态", metric: spec.overallMetric},
		{label: "CPU状态", metric: spec.cpuMetric},
		{label: "内存状态", metric: spec.memoryMetric},
		{label: "磁盘状态", metric: spec.diskMetric},
		{label: "风扇状态", metric: spec.fanMetric},
		{label: "电源状态", metric: spec.powerMetric},
		{label: "温度状态", metric: spec.tempMetric},
	}
}

type dashboardHardwareExceptionAccumulator struct {
	item       model.CmdbDashboardHardwareException
	metricSeen map[string]struct{}
}

func ensureDashboardHardwareAccumulator(accumulators map[uint]*dashboardHardwareExceptionAccumulator, asset *model.CmdbIdcAsset, spec vendorMetricSpec) *dashboardHardwareExceptionAccumulator {
	if acc, exists := accumulators[asset.ID]; exists {
		if acc.item.Vendor == "" {
			acc.item.Vendor = spec.rawVendor
		}
		return acc
	}

	acc := &dashboardHardwareExceptionAccumulator{
		item: model.CmdbDashboardHardwareException{
			AssetID:              asset.ID,
			IP:                   asset.IP,
			RoomName:             asset.Room.Name,
			CabinetName:          asset.Cabinet.Name,
			Brand:                asset.Brand,
			Model:                asset.Model,
			RemoteManagementCard: asset.RemoteManagementCard,
			Vendor:               spec.rawVendor,
			Level:                "warning",
			Metrics:              []string{},
		},
		metricSeen: map[string]struct{}{},
	}
	accumulators[asset.ID] = acc
	return acc
}

func (a *dashboardHardwareExceptionAccumulator) addMetric(label, status string) {
	if status == "critical" {
		a.item.CriticalCount++
		a.item.Level = "critical"
	} else {
		a.item.WarningCount++
	}

	metric := label + ":" + status
	if _, exists := a.metricSeen[metric]; exists {
		return
	}
	a.metricSeen[metric] = struct{}{}
	a.item.Metrics = append(a.item.Metrics, metric)
}

func (a *dashboardHardwareExceptionAccumulator) finish() {
	if a.item.CriticalCount > 0 {
		a.item.Level = "critical"
		a.item.Message = fmt.Sprintf("存在 %d 项严重硬件异常、%d 项硬件告警", a.item.CriticalCount, a.item.WarningCount)
		return
	}
	a.item.Level = "warning"
	a.item.Message = fmt.Sprintf("存在 %d 项硬件告警", a.item.WarningCount)
}

func dashboardCandidatesFromMetricLabels(labels map[string]string) []string {
	values := make([]string, 0, 6)
	seen := map[string]struct{}{}
	appendValue := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}

	for _, key := range []string{"target", "instance"} {
		raw := labels[key]
		appendValue(raw)
		for _, candidate := range normalizeManagementTargets(raw) {
			appendValue(candidate)
		}
	}
	return values
}

func convertDashboardUsageRanks(ranks []model.HostCurrentUsageRank) []model.CmdbDashboardUsageRankItem {
	items := make([]model.CmdbDashboardUsageRankItem, 0, len(ranks))
	for _, rank := range ranks {
		items = append(items, model.CmdbDashboardUsageRankItem{
			HostID:   rank.HostID,
			HostName: rank.HostName,
			Instance: rank.Instance,
			IP:       rank.IP,
			Value:    dashboardRoundRate(rank.Value),
			Status:   dashboardUsageStatus(rank.Value),
		})
	}
	return items
}

func buildDashboardChartPoints(counter map[string]int64) []model.CmdbDashboardChartPoint {
	var total int64
	for _, count := range counter {
		total += count
	}

	points := make([]model.CmdbDashboardChartPoint, 0, len(counter))
	for name, count := range counter {
		points = append(points, model.CmdbDashboardChartPoint{
			Name:  name,
			Value: count,
			Rate:  dashboardRate(count, total),
		})
	}

	sort.Slice(points, func(i, j int) bool {
		if points[i].Value == points[j].Value {
			return points[i].Name < points[j].Name
		}
		return points[i].Value > points[j].Value
	})
	return points
}

func dashboardNormalizeLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "未知"
	}
	return value
}

func dashboardRate(part, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return dashboardRoundRate(float64(part) * 100 / float64(total))
}

func dashboardRoundRate(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

func dashboardUsageStatus(value float64) string {
	switch {
	case value >= 90:
		return "critical"
	case value >= 80:
		return "warning"
	case value >= 60:
		return "attention"
	default:
		return "normal"
	}
}

func appendDashboardWarning(warnings []string, warning string) []string {
	if len(warnings) >= 5 {
		return warnings
	}
	return append(warnings, warning)
}

func parseDashboardDate(value string, loc *time.Location) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}

	layouts := []string{
		"2006-01-02",
		"2006/01/02",
		"2006.01.02",
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		time.RFC3339,
	}
	for _, layout := range layouts {
		parsed, err := time.ParseInLocation(layout, value, loc)
		if err == nil {
			return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, loc), true
		}
	}

	if timestamp, err := strconv.ParseInt(value, 10, 64); err == nil && timestamp > 0 {
		if timestamp > 1_000_000_000_000 {
			timestamp = timestamp / 1000
		}
		parsed := time.Unix(timestamp, 0).In(loc)
		return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, loc), true
	}
	return time.Time{}, false
}
