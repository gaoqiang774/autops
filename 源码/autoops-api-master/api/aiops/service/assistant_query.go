package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	aiopsdao "dodevops-api/api/aiops/dao"
	aiopsmodel "dodevops-api/api/aiops/model"
	cmdbdao "dodevops-api/api/cmdb/dao"
	cmdbmodel "dodevops-api/api/cmdb/model"
	"dodevops-api/common"
	"dodevops-api/common/config"

	"github.com/gin-gonic/gin"
)

const (
	defaultAssistantLimit           = 10
	maxAssistantLimit               = 30
	defaultPlacementTimeRangeDays   = 7
	defaultReclaimTimeRangeDays     = 30
	defaultInspectionTimeRangeDays  = 7
	defaultPlacementCPUThreshold    = 55
	defaultPlacementMemoryThreshold = 65
	defaultPlacementDiskThreshold   = 80
	defaultReclaimCPUThreshold      = 10
	defaultReclaimMemoryThreshold   = 30
	defaultReclaimDiskThreshold     = 50
	defaultAssistantAgentTimeout    = 5 * time.Minute
)

var (
	cpuRequirementRegex    = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(?:c|core|cores|核)`)
	memoryRequirementRegex = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(?:g|gb|G|GB)(?:\s*(?:内存|mem|memory))?`)
	diskRequirementRegex   = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(?:g|gb|G|GB)\s*(?:磁盘|disk|存储)`)
	compactSpecRegex       = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*c\s*(\d+(?:\.\d+)?)\s*g`)
	daysRegex              = regexp.MustCompile(`(\d+)\s*天`)
	numberWithUnitRegex    = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(tb|t|gb|g|mb|m)?`)
	topNRegex              = regexp.MustCompile(`(?i)(?:top\s*|前\s*)(\d+|一|二|两|三|四|五|六|七|八|九|十)`)
)

type AssistantQueryService struct {
	hostDao cmdbdao.CmdbHostDao
	llmDao  *aiopsdao.LLMProviderDao
	client  *http.Client
}

type assistantPrometheusResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"`
			Values [][]interface{}   `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

type assistantHostProfile struct {
	candidate         aiopsmodel.AssistantHostCandidate
	metricsAvailable  bool
	servicesAvailable bool
}

type assistantAgentInfo struct {
	HostID        uint
	Installed     bool
	Online        bool
	Status        int
	LastHeartbeat time.Time
}

type assistantAgentCoverage struct {
	ByHostID       map[uint]assistantAgentInfo
	InstalledHosts int
	OnlineHosts    int
	OfflineHosts   int
}

func NewAssistantQueryService() *AssistantQueryService {
	return &AssistantQueryService{
		hostDao: cmdbdao.NewCmdbHostDao(),
		llmDao:  aiopsdao.NewLLMProviderDao(),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *AssistantQueryService) Query(c *gin.Context, req *aiopsmodel.AssistantQueryRequest) (*aiopsmodel.AssistantQueryResult, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		return nil, errors.New("question is required")
	}

	intent := detectAssistantIntent(req.Question)
	normalizeAssistantRequest(req, intent)

	hosts, totalHosts, missingInfo, err := s.loadHosts(req)
	if err != nil {
		return nil, err
	}

	agentCoverage, err := s.loadAgentCoverage(hostIDsFromHosts(hosts))
	if err != nil {
		missingInfo = appendMissing(missingInfo, "查询 Agent 覆盖率失败，已继续基于 CMDB 和 Prometheus 指标分析："+err.Error())
		agentCoverage = assistantAgentCoverage{ByHostID: map[uint]assistantAgentInfo{}}
	}

	selectedHosts, selectionMissing := selectAssistantHostsForAnalysis(hosts, agentCoverage)
	for _, item := range selectionMissing {
		missingInfo = appendMissing(missingInfo, item)
	}

	evidence := []aiopsmodel.AssistantEvidence{
		{
			Type:      "cmdb",
			Title:     "CMDB 主机范围",
			Reference: "cmdb_host",
			Content:   fmt.Sprintf("本次查询 CMDB 命中 %d 台主机，全部 %d 台进入扫描。", totalHosts, len(selectedHosts)),
			Raw: map[string]interface{}{
				"totalHosts":    totalHosts,
				"selectedHosts": len(selectedHosts),
				"groupId":       req.Scope.GroupID,
				"keyword":       req.Scope.Keyword,
				"env":           req.Scope.Env,
			},
		},
	}

	profiles := make([]assistantHostProfile, 0, len(selectedHosts))
	for _, host := range selectedHosts {
		profile := s.collectHostProfile(c.Request.Context(), host, req.Scope.TimeRangeDays, agentCoverage.ByHostID[host.ID])
		profiles = append(profiles, profile)
	}

	ranking := detectAssistantQueryRanking(req.Question, req.Scope.Limit)
	if ranking != nil {
		req.Scope.Limit = ranking.Limit
	}
	candidates, summary := buildAssistantCandidates(intent, req, profiles, totalHosts, agentCoverage, ranking)
	missingInfo = appendAssistantCoverageMissingInfo(missingInfo, summary)
	evidence = append(evidence, buildAgentCoverageEvidence(summary))
	if prometheusURL() == "" {
		missingInfo = appendMissing(missingInfo, "未配置 monitor.prometheus.url，资源使用率和服务指标不可用，资源选址、回收和容量巡检不会返回基于负载的候选结论。")
	} else {
		evidence = append(evidence, aiopsmodel.AssistantEvidence{
			Type:      "metric",
			Title:     "Prometheus 指标",
			Reference: "monitor.prometheus.url",
			Content:   fmt.Sprintf("本次扫描全部 %d 台主机，按最近 %d 天统计 CPU、内存、磁盘和服务指标；其中 %d 台有可用指标并参与排名。", summary.SelectedHosts, req.Scope.TimeRangeDays, summary.MetricHosts),
		})
	}

	result := &aiopsmodel.AssistantQueryResult{
		Intent:      intent,
		Question:    req.Question,
		Ranking:     ranking,
		Summary:     summary,
		Candidates:  candidates,
		Evidence:    evidence,
		MissingInfo: missingInfo,
		GeneratedAt: time.Now().Format("2006-01-02 15:04:05"),
	}
	result.Answer = buildAssistantFallbackAnswer(result)

	if ranking != nil {
		return result, nil
	}
	if answer, provider, ok, err := s.generateAssistantAnswer(c.Request.Context(), req, result); err == nil && ok {
		applyAssistantLLMProvider(result, provider)
		result.Answer = ensureAssistantCandidateLead(result, answer)
		result.UsedLLM = true
	} else if err != nil {
		applyAssistantLLMProvider(result, provider)
		result.MissingInfo = appendMissing(result.MissingInfo, "LLM 总结不可用，已返回规则化分析结果："+err.Error())
	} else {
		applyAssistantLLMProvider(result, provider)
	}

	return result, nil
}

func (s *AssistantQueryService) loadHosts(req *aiopsmodel.AssistantQueryRequest) ([]cmdbmodel.CmdbHost, int, []string, error) {
	var hosts []cmdbmodel.CmdbHost
	var err error

	switch {
	case len(req.Scope.HostIDs) > 0:
		hosts, err = s.hostDao.GetCmdbHostsByIds(req.Scope.HostIDs)
		if err != nil {
			return nil, 0, nil, err
		}
	case req.Scope.GroupID > 0:
		hosts = s.hostDao.GetCmdbHostsByGroupId(req.Scope.GroupID)
	case strings.TrimSpace(req.Scope.Keyword) != "":
		hosts = s.hostDao.SearchCmdbHosts(req.Scope.Keyword)
	default:
		hosts = s.hostDao.GetCmdbHostList()
	}

	hosts = filterHostsByEnv(hosts, req.Scope.Env)
	total := len(hosts)
	missing := []string{}
	if total == 0 {
		missing = appendMissing(missing, "未在 CMDB 中找到符合条件的主机。")
	}
	return hosts, total, missing, nil
}

func (s *AssistantQueryService) loadAgentCoverage(hostIDs []uint) (assistantAgentCoverage, error) {
	coverage := assistantAgentCoverage{ByHostID: map[uint]assistantAgentInfo{}}
	if len(hostIDs) == 0 {
		return coverage, nil
	}

	type assistantAgentRow struct {
		HostID        uint         `gorm:"column:host_id"`
		Status        int          `gorm:"column:status"`
		LastHeartbeat sql.NullTime `gorm:"column:last_heartbeat"`
	}

	var rows []assistantAgentRow
	err := common.GetDB().
		Table("monitor_agent").
		Select("host_id, status, last_heartbeat").
		Where("host_id IN ?", hostIDs).
		Order("host_id ASC, update_time DESC, id DESC").
		Scan(&rows).Error
	if err != nil {
		return coverage, err
	}

	for _, row := range rows {
		if _, exists := coverage.ByHostID[row.HostID]; exists {
			continue
		}
		lastHeartbeat := time.Time{}
		if row.LastHeartbeat.Valid {
			lastHeartbeat = row.LastHeartbeat.Time
		}
		info := assistantAgentInfo{
			HostID:        row.HostID,
			Installed:     true,
			Status:        row.Status,
			LastHeartbeat: lastHeartbeat,
		}
		info.Online = isAssistantAgentOnline(info)
		coverage.ByHostID[row.HostID] = info
	}

	for _, hostID := range hostIDs {
		info, ok := coverage.ByHostID[hostID]
		if !ok || !info.Installed {
			continue
		}
		coverage.InstalledHosts++
		if info.Online {
			coverage.OnlineHosts++
		} else {
			coverage.OfflineHosts++
		}
	}
	return coverage, nil
}

func hostIDsFromHosts(hosts []cmdbmodel.CmdbHost) []uint {
	ids := make([]uint, 0, len(hosts))
	seen := make(map[uint]struct{}, len(hosts))
	for _, host := range hosts {
		if host.ID == 0 {
			continue
		}
		if _, ok := seen[host.ID]; ok {
			continue
		}
		seen[host.ID] = struct{}{}
		ids = append(ids, host.ID)
	}
	return ids
}

func selectAssistantHostsForAnalysis(hosts []cmdbmodel.CmdbHost, coverage assistantAgentCoverage) ([]cmdbmodel.CmdbHost, []string) {
	selected := append([]cmdbmodel.CmdbHost(nil), hosts...)
	sort.SliceStable(selected, func(i, j int) bool {
		leftPriority := assistantHostAnalysisPriority(selected[i], coverage)
		rightPriority := assistantHostAnalysisPriority(selected[j], coverage)
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}
		return selected[i].ID < selected[j].ID
	})

	return selected, nil
}

func assistantHostAnalysisPriority(host cmdbmodel.CmdbHost, coverage assistantAgentCoverage) int {
	info, ok := coverage.ByHostID[host.ID]
	if !ok || !info.Installed {
		return 2
	}
	if info.Online {
		return 0
	}
	return 1
}

func isAssistantAgentOnline(info assistantAgentInfo) bool {
	if !info.Installed {
		return false
	}
	if info.Status != cmdbmodel.AgentStatusRunning {
		return false
	}
	if info.LastHeartbeat.IsZero() {
		return false
	}
	return time.Since(info.LastHeartbeat) <= assistantAgentHeartbeatTimeout()
}

func assistantAgentHeartbeatTimeout() time.Duration {
	if config.Config != nil && config.Config.Monitor.Agent.HeartbeatTimeout > 0 {
		return time.Duration(config.Config.Monitor.Agent.HeartbeatTimeout) * time.Second
	}
	return defaultAssistantAgentTimeout
}

func (s *AssistantQueryService) collectHostProfile(ctx context.Context, host cmdbmodel.CmdbHost, days int, agent assistantAgentInfo) assistantHostProfile {
	candidate := aiopsmodel.AssistantHostCandidate{
		HostID:         host.ID,
		HostName:       host.HostName,
		Name:           host.Name,
		GroupID:        host.GroupID,
		GroupName:      host.Group.Name,
		PrivateIP:      host.PrivateIP,
		PublicIP:       host.PublicIP,
		SSHIP:          host.SSHIP,
		IP:             assistantHostIP(host),
		OS:             host.OS,
		Status:         host.Status,
		Vendor:         host.Vendor,
		Region:         host.Region,
		Tags:           host.Tag,
		CPUCores:       parseCPUValue(host.CPU),
		MemoryGB:       parseSizeGB(host.Memory),
		DiskTotalGB:    parseSizeGB(host.Disk),
		AgentInstalled: agent.Installed,
		AgentOnline:    agent.Online,
		AgentStatus:    agent.Status,
	}
	if !agent.LastHeartbeat.IsZero() {
		candidate.AgentLastHeartbeat = agent.LastHeartbeat.Format("2006/01/02 15:04")
	}

	instance := assistantMetricIdentifier(host)
	if instance == "" || prometheusURL() == "" {
		return assistantHostProfile{candidate: candidate}
	}

	rangeSelector := fmt.Sprintf("%dd", days)
	metricsOK := false

	if value, ok := s.queryMetricValue(ctx, fmt.Sprintf(`system_cpu_usage_percent{instance=%q}`, instance)); ok {
		candidate.CurrentCPU = roundAssistantFloat(value)
		metricsOK = true
	}
	if value, ok := s.queryMetricValue(ctx, fmt.Sprintf(`system_memory_usage_percent{instance=%q}`, instance)); ok {
		candidate.CurrentMemory = roundAssistantFloat(value)
		metricsOK = true
	}
	if value, ok := s.queryMetricValue(ctx, fmt.Sprintf(`system_disk_usage_percent{instance=%q,mountpoint="/"}`, instance)); ok {
		candidate.CurrentDisk = roundAssistantFloat(value)
		metricsOK = true
	}
	if value, ok := s.queryMetricValue(ctx, fmt.Sprintf(`avg_over_time(system_cpu_usage_percent{instance=%q}[%s])`, instance, rangeSelector)); ok {
		candidate.AvgCPU = roundAssistantFloat(value)
		metricsOK = true
	}
	if value, ok := s.queryMetricValue(ctx, fmt.Sprintf(`avg_over_time(system_memory_usage_percent{instance=%q}[%s])`, instance, rangeSelector)); ok {
		candidate.AvgMemory = roundAssistantFloat(value)
		metricsOK = true
	}
	if value, ok := s.queryMetricValue(ctx, fmt.Sprintf(`avg_over_time(system_disk_usage_percent{instance=%q,mountpoint="/"}[%s])`, instance, rangeSelector)); ok {
		candidate.AvgDisk = roundAssistantFloat(value)
		metricsOK = true
	}
	if value, ok := s.queryMetricValue(ctx, fmt.Sprintf(`system_cpu_core_count{instance=%q}`, instance)); ok && value > 0 {
		candidate.CPUCores = roundAssistantFloat(value)
	}
	if value, ok := s.queryMetricValue(ctx, fmt.Sprintf(`system_memory_total_gb{instance=%q}`, instance)); ok && value > 0 {
		candidate.MemoryGB = roundAssistantFloat(value)
	}
	if value, ok := s.queryMetricValue(ctx, fmt.Sprintf(`system_disk_total_bytes{instance=%q,mountpoint="/"}`, instance)); ok && value > 0 {
		candidate.DiskTotalGB = roundAssistantFloat(value / 1024 / 1024 / 1024)
	}
	candidate.HasMetrics = metricsOK

	services, servicesOK := s.queryHostServices(ctx, instance)
	candidate.ServiceDetails = services
	candidate.Services = serviceUsageNames(services)
	candidate.ServiceCount = len(services)
	candidate.BusinessServiceCount = countBusinessServices(services)

	return assistantHostProfile{
		candidate:         candidate,
		metricsAvailable:  metricsOK,
		servicesAvailable: servicesOK,
	}
}

func (s *AssistantQueryService) queryMetricValue(ctx context.Context, query string) (float64, bool) {
	resp, err := s.queryPrometheus(ctx, query)
	if err != nil || len(resp.Data.Result) == 0 {
		return 0, false
	}
	item := resp.Data.Result[0]
	if len(item.Value) >= 2 {
		return parsePrometheusFloat(item.Value[1])
	}
	if len(item.Values) > 0 {
		last := item.Values[len(item.Values)-1]
		if len(last) >= 2 {
			return parsePrometheusFloat(last[1])
		}
	}
	return 0, false
}

func (s *AssistantQueryService) queryHostServices(ctx context.Context, instance string) ([]aiopsmodel.AssistantServiceUsage, bool) {
	resp, err := s.queryPrometheus(ctx, fmt.Sprintf(`avg by (service_name) (service_cpu_usage_percent{instance=%q})`, instance))
	if err != nil {
		return nil, false
	}
	services := make([]aiopsmodel.AssistantServiceUsage, 0, len(resp.Data.Result))
	for _, item := range resp.Data.Result {
		name := strings.TrimSpace(item.Metric["service_name"])
		if name != "" {
			cpu := 0.0
			if len(item.Value) >= 2 {
				if value, ok := parsePrometheusFloat(item.Value[1]); ok {
					cpu = roundAssistantFloat(value)
				}
			}
			platform, reason := classifyPlatformService(name)
			services = append(services, aiopsmodel.AssistantServiceUsage{
				Name:     name,
				CPU:      cpu,
				Platform: platform,
				Reason:   reason,
			})
		}
	}

	for i := range services {
		memResp, err := s.queryPrometheus(ctx, fmt.Sprintf(`avg(service_memory_usage_percent{instance=%q,service_name=%q})`, instance, services[i].Name))
		if err != nil || len(memResp.Data.Result) == 0 || len(memResp.Data.Result[0].Value) < 2 {
			continue
		}
		if value, ok := parsePrometheusFloat(memResp.Data.Result[0].Value[1]); ok {
			services[i].Memory = roundAssistantFloat(value)
		}
	}

	sort.SliceStable(services, func(i, j int) bool {
		if services[i].Platform != services[j].Platform {
			return !services[i].Platform
		}
		return services[i].Name < services[j].Name
	})
	return services, true
}

func (s *AssistantQueryService) queryPrometheus(ctx context.Context, query string) (*assistantPrometheusResponse, error) {
	baseURL := prometheusURL()
	if baseURL == "" {
		return nil, errors.New("prometheus URL not configured")
	}
	endpoint := strings.TrimRight(baseURL, "/") + "/api/v1/query"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("query", query)
	q.Set("time", strconv.FormatInt(time.Now().Unix(), 10))
	req.URL.RawQuery = q.Encode()

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, err
	}
	var result assistantPrometheusResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus query failed: %s", truncateString(string(body), 512))
	}
	return &result, nil
}

func buildAssistantCandidates(intent string, req *aiopsmodel.AssistantQueryRequest, profiles []assistantHostProfile, totalHosts int, coverage assistantAgentCoverage, ranking *aiopsmodel.AssistantQueryRanking) ([]aiopsmodel.AssistantHostCandidate, aiopsmodel.AssistantQuerySummary) {
	candidates := make([]aiopsmodel.AssistantHostCandidate, 0, len(profiles))
	metricHosts := 0
	selectedAgentInstalledHosts := 0
	selectedAgentOnlineHosts := 0
	selectedAgentOfflineHosts := 0

	for _, profile := range profiles {
		if profile.metricsAvailable {
			metricHosts++
		}
		if profile.candidate.AgentInstalled {
			selectedAgentInstalledHosts++
			if profile.candidate.AgentOnline {
				selectedAgentOnlineHosts++
			} else {
				selectedAgentOfflineHosts++
			}
		}
		candidate := evaluateAssistantCandidate(intent, req, profile)
		if (ranking != nil && candidate.HasMetrics) || (ranking == nil && shouldKeepAssistantCandidate(intent, candidate)) {
			candidates = append(candidates, candidate)
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if ranking != nil {
			left := assistantRankingValue(candidates[i], ranking)
			right := assistantRankingValue(candidates[j], ranking)
			if left != right {
				return left > right
			}
		}
		return candidates[i].Score > candidates[j].Score
	})
	if len(candidates) > req.Scope.Limit {
		candidates = candidates[:req.Scope.Limit]
	}

	summary := aiopsmodel.AssistantQuerySummary{
		TotalHosts:                  totalHosts,
		SelectedHosts:               len(profiles),
		AnalyzedHosts:               metricHosts,
		MetricHosts:                 metricHosts,
		NoMetricHosts:               len(profiles) - metricHosts,
		AgentInstalledHosts:         coverage.InstalledHosts,
		AgentOnlineHosts:            coverage.OnlineHosts,
		AgentOfflineHosts:           coverage.OfflineHosts,
		SelectedAgentInstalledHosts: selectedAgentInstalledHosts,
		SelectedAgentOnlineHosts:    selectedAgentOnlineHosts,
		SelectedAgentOfflineHosts:   selectedAgentOfflineHosts,
		MatchedHosts:                len(candidates),
		TimeRangeDays:               req.Scope.TimeRangeDays,
		RequiredCPUCores:            req.Options.RequiredCPUCores,
		RequiredMemoryGB:            req.Options.RequiredMemoryGB,
		RequiredDiskGB:              req.Options.RequiredDiskGB,
		CPUThreshold:                req.Options.CPUThreshold,
		MemoryThreshold:             req.Options.MemoryThreshold,
		DiskThreshold:               req.Options.DiskThreshold,
	}
	return candidates, summary
}

func detectAssistantQueryRanking(question string, fallbackLimit int) *aiopsmodel.AssistantQueryRanking {
	question = strings.ToLower(strings.TrimSpace(question))
	if question == "" {
		return nil
	}
	hasRanking := topNRegex.MatchString(question) || containsAnyText(question, "top", "排行", "排名", "最高", "最大")
	if !hasRanking {
		return nil
	}
	metric := ""
	switch {
	case containsAnyText(question, "内存", "memory", "mem"):
		metric = "memory"
	case containsAnyText(question, "磁盘", "disk"):
		metric = "disk"
	case containsAnyText(question, "cpu", "处理器"):
		metric = "cpu"
	default:
		return nil
	}
	limit := fallbackLimit
	if limit <= 0 {
		limit = 5
	}
	if match := topNRegex.FindStringSubmatch(question); len(match) >= 2 {
		if parsed, ok := assistantRankingPositiveInteger(match[1]); ok {
			limit = parsed
		}
	}
	if limit > maxAssistantLimit {
		limit = maxAssistantLimit
	}
	period := "average"
	if containsAnyText(question, "当前", "实时", "现在", "此刻", "current", "realtime", "real-time") {
		period = "current"
	}
	return &aiopsmodel.AssistantQueryRanking{Metric: metric, Period: period, Direction: "desc", Limit: limit}
}

func assistantRankingPositiveInteger(value string) (int, bool) {
	if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && parsed > 0 {
		return parsed, true
	}
	values := map[string]int{"一": 1, "二": 2, "两": 2, "三": 3, "四": 4, "五": 5, "六": 6, "七": 7, "八": 8, "九": 9, "十": 10}
	parsed, ok := values[strings.TrimSpace(value)]
	return parsed, ok
}

func assistantRankingValue(candidate aiopsmodel.AssistantHostCandidate, ranking *aiopsmodel.AssistantQueryRanking) float64 {
	if ranking == nil {
		return candidate.Score
	}
	if ranking.Period == "average" {
		switch ranking.Metric {
		case "memory":
			return candidate.AvgMemory
		case "disk":
			return candidate.AvgDisk
		default:
			return candidate.AvgCPU
		}
	}
	switch ranking.Metric {
	case "memory":
		return candidate.CurrentMemory
	case "disk":
		return candidate.CurrentDisk
	default:
		return candidate.CurrentCPU
	}
}

func appendAssistantCoverageMissingInfo(missing []string, summary aiopsmodel.AssistantQuerySummary) []string {
	if summary.TotalHosts > 0 {
		missing = appendMissing(missing, fmt.Sprintf(
			"资产总数 %d 台，安装监控助手 %d 台（在线 %d 台，离线 %d 台），本次查询 %d 台主机。",
			summary.TotalHosts,
			summary.AgentInstalledHosts,
			summary.AgentOnlineHosts,
			summary.AgentOfflineHosts,
			summary.SelectedHosts,
		))
	}
	if summary.SelectedHosts > 0 {
		switch {
		case summary.MetricHosts == 0:
			missing = appendMissing(missing, "本次查询主机暂无可用监控指标，请先确认监控助手和采集链路。")
		case summary.MetricHosts < summary.SelectedHosts:
			missing = appendMissing(missing, fmt.Sprintf("本次查询 %d 台中，只有 %d 台有可用监控指标。", summary.SelectedHosts, summary.MetricHosts))
		}
	}
	return missing
}

func buildAgentCoverageEvidence(summary aiopsmodel.AssistantQuerySummary) aiopsmodel.AssistantEvidence {
	return aiopsmodel.AssistantEvidence{
		Type:      "agent",
		Title:     "Agent 覆盖率",
		Reference: "monitor_agent",
		Content:   fmt.Sprintf("CMDB 命中范围内安装 Agent %d 台，在线 %d 台，离线 %d 台；本次扫描 %d 台，其中 %d 台有 Prometheus 指标。", summary.AgentInstalledHosts, summary.AgentOnlineHosts, summary.AgentOfflineHosts, summary.SelectedHosts, summary.MetricHosts),
		Raw: map[string]interface{}{
			"totalHosts":                  summary.TotalHosts,
			"selectedHosts":               summary.SelectedHosts,
			"metricHosts":                 summary.MetricHosts,
			"noMetricHosts":               summary.NoMetricHosts,
			"agentInstalledHosts":         summary.AgentInstalledHosts,
			"agentOnlineHosts":            summary.AgentOnlineHosts,
			"agentOfflineHosts":           summary.AgentOfflineHosts,
			"selectedAgentInstalledHosts": summary.SelectedAgentInstalledHosts,
			"selectedAgentOnlineHosts":    summary.SelectedAgentOnlineHosts,
			"selectedAgentOfflineHosts":   summary.SelectedAgentOfflineHosts,
		},
	}
}

func evaluateAssistantCandidate(intent string, req *aiopsmodel.AssistantQueryRequest, profile assistantHostProfile) aiopsmodel.AssistantHostCandidate {
	c := profile.candidate
	c.Reasons = []string{}
	c.Risks = []string{}
	c.NextChecks = []string{}
	c.RequirementMatched = true
	c.RequirementGaps = []string{}

	if !c.AgentInstalled {
		c.Risks = append(c.Risks, "未安装 Agent，无法确认实时资源和服务指标。")
	} else if !c.AgentOnline {
		c.Risks = append(c.Risks, "Agent 当前离线，监控指标可能缺失或滞后。")
	}
	if !profile.metricsAvailable {
		c.Risks = append(c.Risks, "缺少 Prometheus 使用率指标，资源判断可信度降低。")
	}
	if !profile.servicesAvailable {
		c.Risks = append(c.Risks, "未查询到 service_cpu_usage_percent 指标，无法完全确认是否部署业务服务。")
	}
	if c.Status != 1 {
		c.Risks = append(c.Risks, "CMDB 主机状态不是认证成功，请确认主机可登录和可监控。")
	}

	switch intent {
	case aiopsmodel.AssistantIntentResourceReclaim:
		c.Score = reclaimScore(c, req)
		c.Recommendation = "回收候选"
		if c.AvgCPU <= req.Options.CPUThreshold {
			c.Reasons = append(c.Reasons, fmt.Sprintf("近 %d 天 CPU 平均 %.1f%%，低于 %.1f%%。", req.Scope.TimeRangeDays, c.AvgCPU, req.Options.CPUThreshold))
		}
		if c.AvgMemory <= req.Options.MemoryThreshold {
			c.Reasons = append(c.Reasons, fmt.Sprintf("近 %d 天内存平均 %.1f%%，低于 %.1f%%。", req.Scope.TimeRangeDays, c.AvgMemory, req.Options.MemoryThreshold))
		}
		if c.AvgDisk <= req.Options.DiskThreshold {
			c.Reasons = append(c.Reasons, fmt.Sprintf("根分区磁盘平均 %.1f%%，低于 %.1f%%。", c.AvgDisk, req.Options.DiskThreshold))
		}
		if profile.servicesAvailable && c.BusinessServiceCount == 0 {
			c.Reasons = append(c.Reasons, "未发现业务服务指标，仅发现平台基础服务或未发现服务。")
		}
		if c.BusinessServiceCount > 0 {
			c.Risks = append(c.Risks, fmt.Sprintf("发现 %d 个疑似业务服务，回收前必须确认服务归属和迁移方案。", c.BusinessServiceCount))
		}
		c.NextChecks = append(c.NextChecks, "确认 CMDB 是否存在未登记服务。", "确认主机上是否有计划任务或临时任务。", "确认服务指标中的业务服务是否可迁移。", "确认最近登录和变更记录后再执行回收。")
	case aiopsmodel.AssistantIntentCapacityInspection:
		c.Score = capacityPressureScore(c)
		c.Recommendation = "需要关注"
		c.Reasons = append(c.Reasons, fmt.Sprintf("当前 CPU %.1f%%，内存 %.1f%%，磁盘 %.1f%%。", c.CurrentCPU, c.CurrentMemory, c.CurrentDisk))
		c.NextChecks = append(c.NextChecks, "查看主机 Top 进程和业务服务指标。", "确认是否有活跃告警或近期变更。")
	case aiopsmodel.AssistantIntentAssetQuery:
		c.Score = assetQueryScore(c)
		c.Recommendation = "资产概览"
		c.Reasons = append(c.Reasons, fmt.Sprintf("当前 CPU %.1f%%，内存 %.1f%%，磁盘 %.1f%%，服务数 %d，疑似业务服务 %d。", c.CurrentCPU, c.CurrentMemory, c.CurrentDisk, c.ServiceCount, c.BusinessServiceCount))
	default:
		applyPlacementRequirementChecks(&c, req)
		c.Score = placementScore(c, req)
		c.Recommendation = "部署候选"
		if !c.RequirementMatched {
			c.Recommendation = "规格不满足"
		}
		if req.Options.RequiredCPUCores > 0 && c.CPUCores > 0 {
			c.Reasons = append(c.Reasons, fmt.Sprintf("CPU 规格 %.0f 核，需求 %.0f 核。", c.CPUCores, req.Options.RequiredCPUCores))
		}
		if req.Options.RequiredMemoryGB > 0 && c.MemoryGB > 0 {
			c.Reasons = append(c.Reasons, fmt.Sprintf("内存规格 %.1f GB，需求 %.1f GB。", c.MemoryGB, req.Options.RequiredMemoryGB))
		}
		c.Reasons = append(c.Reasons, fmt.Sprintf("近 %d 天平均 CPU %.1f%%、内存 %.1f%%、磁盘 %.1f%%。", req.Scope.TimeRangeDays, c.AvgCPU, c.AvgMemory, c.AvgDisk))
		if c.BusinessServiceCount == 0 && profile.servicesAvailable {
			c.Reasons = append(c.Reasons, "未发现疑似业务服务，现有服务主要为平台基础组件。")
		}
		if c.BusinessServiceCount > 0 {
			c.Risks = append(c.Risks, fmt.Sprintf("当前机器已有 %d 个疑似业务服务，新部署前需要确认资源和端口冲突。", c.BusinessServiceCount))
		}
		c.NextChecks = append(c.NextChecks, "确认目标服务端口、运行时和依赖组件。", "确认机器所属业务组和环境是否允许部署。", "确认现有服务是否与新服务存在端口或资源冲突。")
	}
	if !profile.metricsAvailable {
		c.Score -= 25
	}
	if !c.AgentInstalled {
		c.Score -= 40
	} else if !c.AgentOnline {
		c.Score -= 30
	}
	if !profile.servicesAvailable && intent == aiopsmodel.AssistantIntentResourceReclaim {
		c.Score -= 10
	}
	if c.Status != 1 && !req.Options.IncludeOffline {
		c.Score = 0
		c.Recommendation = "状态异常"
	}
	c.Score = roundAssistantFloat(clampAssistantScore(c.Score))
	return c
}

func shouldKeepAssistantCandidate(intent string, c aiopsmodel.AssistantHostCandidate) bool {
	switch intent {
	case aiopsmodel.AssistantIntentResourceReclaim:
		return c.HasMetrics && c.Score >= 45
	case aiopsmodel.AssistantIntentCapacityInspection:
		return c.HasMetrics && c.Score >= 50
	case aiopsmodel.AssistantIntentAssetQuery:
		return true
	default:
		return c.HasMetrics && c.RequirementMatched && c.Score >= 40
	}
}

func applyPlacementRequirementChecks(c *aiopsmodel.AssistantHostCandidate, req *aiopsmodel.AssistantQueryRequest) {
	if req.Options.RequiredCPUCores > 0 {
		if c.CPUCores <= 0 {
			c.RequirementMatched = false
			c.RequirementGaps = append(c.RequirementGaps, "缺少 CPU 规格数据，无法确认满足部署需求。")
			c.Risks = append(c.Risks, "缺少 CPU 规格数据。")
		} else if c.CPUCores < req.Options.RequiredCPUCores {
			c.RequirementMatched = false
			msg := fmt.Sprintf("CPU 规格 %.0f 核低于需求 %.0f 核。", c.CPUCores, req.Options.RequiredCPUCores)
			c.RequirementGaps = append(c.RequirementGaps, msg)
			c.Risks = append(c.Risks, msg)
		}
	}
	if req.Options.RequiredMemoryGB > 0 {
		if c.MemoryGB <= 0 {
			c.RequirementMatched = false
			c.RequirementGaps = append(c.RequirementGaps, "缺少内存规格数据，无法确认满足部署需求。")
			c.Risks = append(c.Risks, "缺少内存规格数据。")
		} else if c.MemoryGB < req.Options.RequiredMemoryGB {
			c.RequirementMatched = false
			msg := fmt.Sprintf("内存规格 %.1f GB 低于需求 %.1f GB。", c.MemoryGB, req.Options.RequiredMemoryGB)
			c.RequirementGaps = append(c.RequirementGaps, msg)
			c.Risks = append(c.Risks, msg)
		}
	}
	if req.Options.RequiredDiskGB > 0 {
		if c.DiskTotalGB <= 0 {
			c.RequirementMatched = false
			c.RequirementGaps = append(c.RequirementGaps, "缺少磁盘规格数据，无法确认满足部署需求。")
			c.Risks = append(c.Risks, "缺少磁盘规格数据。")
		} else if c.DiskTotalGB < req.Options.RequiredDiskGB {
			c.RequirementMatched = false
			msg := fmt.Sprintf("磁盘规格 %.1f GB 低于需求 %.1f GB。", c.DiskTotalGB, req.Options.RequiredDiskGB)
			c.RequirementGaps = append(c.RequirementGaps, msg)
			c.Risks = append(c.Risks, msg)
		}
	}
}

func placementScore(c aiopsmodel.AssistantHostCandidate, req *aiopsmodel.AssistantQueryRequest) float64 {
	score := 100.0
	score -= c.AvgCPU * 0.35
	score -= c.AvgMemory * 0.35
	score -= c.AvgDisk * 0.2
	score -= float64(c.BusinessServiceCount) * 8
	score -= float64(c.ServiceCount-c.BusinessServiceCount) * 1.5
	if req.Options.RequiredCPUCores > 0 && c.CPUCores > 0 && c.CPUCores < req.Options.RequiredCPUCores {
		score -= 60
	}
	if req.Options.RequiredMemoryGB > 0 && c.MemoryGB > 0 && c.MemoryGB < req.Options.RequiredMemoryGB {
		score -= 60
	}
	if req.Options.RequiredDiskGB > 0 && c.DiskTotalGB > 0 && c.DiskTotalGB < req.Options.RequiredDiskGB {
		score -= 40
	}
	if c.AvgCPU > req.Options.CPUThreshold || c.AvgMemory > req.Options.MemoryThreshold || c.AvgDisk > req.Options.DiskThreshold {
		score -= 25
	}
	if c.Status != 1 {
		score -= 20
	}
	return roundAssistantFloat(clampAssistantScore(score))
}

func reclaimScore(c aiopsmodel.AssistantHostCandidate, req *aiopsmodel.AssistantQueryRequest) float64 {
	score := 100.0
	score -= c.AvgCPU * 2.5
	score -= c.AvgMemory * 1.2
	score -= c.AvgDisk * 0.7
	score -= float64(c.BusinessServiceCount) * 45
	score -= float64(c.ServiceCount-c.BusinessServiceCount) * 4
	if c.AvgCPU > req.Options.CPUThreshold || c.AvgMemory > req.Options.MemoryThreshold || c.AvgDisk > req.Options.DiskThreshold {
		score -= 30
	}
	if c.Status != 1 {
		score -= 10
	}
	return roundAssistantFloat(clampAssistantScore(score))
}

func capacityPressureScore(c aiopsmodel.AssistantHostCandidate) float64 {
	score := c.CurrentCPU*0.35 + c.CurrentMemory*0.35 + c.CurrentDisk*0.3
	if c.CurrentCPU >= 80 || c.CurrentMemory >= 80 || c.CurrentDisk >= 85 {
		score += 20
	}
	return roundAssistantFloat(clampAssistantScore(score))
}

func assetQueryScore(c aiopsmodel.AssistantHostCandidate) float64 {
	score := 100.0
	if c.Status != 1 {
		score -= 20
	}
	return roundAssistantFloat(clampAssistantScore(score))
}

func (s *AssistantQueryService) generateAssistantAnswer(ctx context.Context, req *aiopsmodel.AssistantQueryRequest, result *aiopsmodel.AssistantQueryResult) (string, *aiopsmodel.LLMProvider, bool, error) {
	provider, apiKey, err := s.resolveLLMProvider(req.ProviderID)
	if err != nil {
		return "", nil, false, err
	}
	prompt := buildAssistantLLMPrompt(req, result)
	timeout := time.Duration(provider.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	llmCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	answer, err := callLLMForLogAnalysis(llmCtx, provider, apiKey, prompt)
	if err != nil {
		return "", provider, false, err
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return "", provider, false, nil
	}
	return answer, provider, true, nil
}

func applyAssistantLLMProvider(result *aiopsmodel.AssistantQueryResult, provider *aiopsmodel.LLMProvider) {
	if result == nil || provider == nil {
		return
	}
	result.ProviderID = provider.ID
	result.ProviderName = provider.Name
	result.ProviderType = provider.ProviderType
	result.ModelCode = provider.DefaultModel
}

func (s *AssistantQueryService) resolveLLMProvider(providerID uint) (*aiopsmodel.LLMProvider, string, error) {
	var provider *aiopsmodel.LLMProvider
	var err error
	if providerID > 0 {
		provider, err = s.llmDao.GetByID(providerID)
		if err != nil {
			return nil, "", fmt.Errorf("llm provider not found: %v", err)
		}
		if provider.Status != aiopsmodel.LLMProviderStatusEnabled {
			return nil, "", errors.New("llm provider is disabled")
		}
	} else {
		providers, err := s.llmDao.GetEnabled()
		if err != nil {
			return nil, "", err
		}
		if len(providers) == 0 {
			return nil, "", errors.New("no enabled llm provider")
		}
		provider = &providers[0]
	}

	apiKey, err := provider.DecryptAPIKey()
	if err != nil {
		return nil, "", err
	}
	return provider, apiKey, nil
}

func buildAssistantLLMPrompt(req *aiopsmodel.AssistantQueryRequest, result *aiopsmodel.AssistantQueryResult) string {
	type llmPayload struct {
		Intent      string                              `json:"intent"`
		Question    string                              `json:"question"`
		Summary     aiopsmodel.AssistantQuerySummary    `json:"summary"`
		Candidates  []aiopsmodel.AssistantHostCandidate `json:"candidates"`
		Evidence    []aiopsmodel.AssistantEvidence      `json:"evidence"`
		MissingInfo []string                            `json:"missingInfo"`
	}
	payload := llmPayload{
		Intent:      result.Intent,
		Question:    req.Question,
		Summary:     result.Summary,
		Candidates:  result.Candidates,
		Evidence:    result.Evidence,
		MissingInfo: result.MissingInfo,
	}
	data, _ := json.Marshal(payload)
	return "你是平台内部资源治理和运维查询助手。请基于 JSON 证据用中文回答，不能编造没有证据的数据。输出应包含：结论、推荐资源或候选资源、关键依据、风险和下一步确认项。\n\n重要规则：\n1. candidates 是后端规则筛选后的候选结果，只能推荐 candidates 中的主机。\n2. 回答开头必须先说明扫描范围，固定表达为“本次共扫描 selectedHosts 台主机，其中 metricHosts 台有可用 Prometheus 指标并参与排名”；严禁只写“从 metricHosts 台主机中筛选”而省略实际扫描总数。随后给出候选资源清单。\n3. 候选资源清单必须使用 Markdown 表格，至少包含：主机名、IP、近 N 天平均 CPU、近 N 天平均内存、磁盘使用率、业务服务、综合评分。只展示 candidates 中评分最高的前 5 台作为优先推荐。\n4. totalHosts 是 CMDB 命中范围，selectedHosts 是实际扫描的主机数，metricHosts/analyzedHosts 是有 Prometheus 指标并实际参与资源负载排名的主机数。必须明确区分扫描范围和有效指标范围；不能把 metricHosts 描述成总扫描数，也不能声称无指标主机参与了负载排名。\n5. resource_placement 场景下，只有 requirementMatched=true 且 hasMetrics=true 的主机才能称为部署候选；不得把 requirementMatched=false、hasMetrics=false 或规格不足的主机称为备选部署机器。\n6. 如果 candidates 为空，要明确说明没有找到满足条件的机器，并说明缺少哪些条件或建议如何缩小范围。\n7. risks 和 missingInfo 必须原样纳入风险或边界说明。\n\nJSON:\n" + string(data)
}

func buildAssistantFallbackAnswer(result *aiopsmodel.AssistantQueryResult) string {
	if len(result.Candidates) == 0 {
		if result.Summary.SelectedHosts > 0 && result.Summary.MetricHosts == 0 {
			return fmt.Sprintf("本次 CMDB 命中 %d 台主机，已扫描全部 %d 台，但没有主机具备可用 Prometheus 使用率指标。请先确认 Agent 在线和指标采集链路，再进行资源选址、回收或容量判断。", result.Summary.TotalHosts, result.Summary.SelectedHosts)
		}
		switch result.Intent {
		case aiopsmodel.AssistantIntentResourcePlacement:
			return "未找到同时满足资源规格和负载条件的部署候选主机。建议缩小范围、降低资源需求，或检查 CMDB CPU/内存/磁盘规格是否完整。"
		default:
			return "未找到满足当前条件的候选主机。建议缩小查询范围，或检查 CMDB 主机、Prometheus 指标和服务指标是否完整。"
		}
	}
	switch result.Intent {
	case aiopsmodel.AssistantIntentResourceReclaim:
		return buildAssistantCandidateLead(result) + "\n\n关键提醒：请先确认是否存在未登记服务、计划任务和近期变更，再进入回收流程。"
	case aiopsmodel.AssistantIntentCapacityInspection:
		return buildAssistantCandidateLead(result) + "\n\n关键提醒：建议优先查看 Top 进程、活跃告警和近期变更。"
	case aiopsmodel.AssistantIntentAssetQuery:
		return buildAssistantCandidateLead(result) + "\n\n关键提醒：已附带当前资源使用率和服务指标摘要。"
	default:
		return buildAssistantCandidateLead(result) + "\n\n关键提醒：建议结合业务组、环境、端口和服务依赖做最终确认。"
	}
}

func ensureAssistantCandidateLead(result *aiopsmodel.AssistantQueryResult, answer string) string {
	answer = strings.TrimSpace(answer)
	if result == nil || len(result.Candidates) == 0 || hasAssistantCandidateTable(answer) {
		return answer
	}
	lead := buildAssistantCandidateLead(result)
	if answer == "" {
		return lead
	}
	return lead + "\n\n" + answer
}

func hasAssistantCandidateTable(answer string) bool {
	head := answer
	if len(head) > 600 {
		head = head[:600]
	}
	return strings.Contains(head, "| 主机") &&
		strings.Contains(head, "IP") &&
		strings.Contains(head, "CPU") &&
		strings.Contains(head, "内存") &&
		strings.Contains(head, "磁盘")
}

func buildAssistantCandidateLead(result *aiopsmodel.AssistantQueryResult) string {
	if result == nil || len(result.Candidates) == 0 {
		return ""
	}
	if result.Ranking != nil {
		return buildAssistantMetricRankingLead(result)
	}
	limit := len(result.Candidates)
	if limit > 5 {
		limit = 5
	}

	title := assistantCandidateTitle(result.Intent)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("结论：本次共扫描 %d 台主机，其中 %d 台有可用监控指标并参与排名；找到 %d 台%s，优先看评分最高的前 %d 台。\n\n",
		result.Summary.SelectedHosts,
		result.Summary.MetricHosts,
		len(result.Candidates),
		title,
		limit,
	))
	b.WriteString(fmt.Sprintf("| 排名 | 主机名 | IP | 近 %d 天平均 CPU | 近 %d 天平均内存 | 磁盘使用率 | 业务服务 | 综合评分 |\n",
		result.Summary.TimeRangeDays,
		result.Summary.TimeRangeDays,
	))
	b.WriteString("| --- | --- | --- | ---: | ---: | ---: | --- | ---: |\n")
	for i := 0; i < limit; i++ {
		item := result.Candidates[i]
		b.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s | %s | %s | %.2f |\n",
			i+1,
			escapeAssistantMarkdownCell(firstNonEmptyAssistantText(item.HostName, item.Name, "-")),
			escapeAssistantMarkdownCell(firstNonEmptyAssistantText(item.IP, item.PrivateIP, item.PublicIP, item.SSHIP, "-")),
			formatAssistantPercent(item.AvgCPU),
			formatAssistantPercent(item.AvgMemory),
			formatAssistantPercent(item.AvgDisk),
			escapeAssistantMarkdownCell(formatAssistantBusinessServices(item)),
			item.Score,
		))
	}
	b.WriteString(fmt.Sprintf("\n筛选阈值：CPU <= %s，内存 <= %s，磁盘 <= %s。",
		formatAssistantPercent(result.Summary.CPUThreshold),
		formatAssistantPercent(result.Summary.MemoryThreshold),
		formatAssistantPercent(result.Summary.DiskThreshold),
	))
	return strings.TrimSpace(b.String())
}

func buildAssistantMetricRankingLead(result *aiopsmodel.AssistantQueryResult) string {
	limit := len(result.Candidates)
	if result.Ranking.Limit > 0 && limit > result.Ranking.Limit {
		limit = result.Ranking.Limit
	}
	metricName := assistantRankingMetricName(result.Ranking)
	periodName := "当前"
	if result.Ranking.Period == "average" {
		periodName = fmt.Sprintf("近 %d 天平均", result.Summary.TimeRangeDays)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("本次共扫描 %d 台主机，其中 %d 台有可用 Prometheus 指标并参与排名。以下是%s%s使用率 Top %d：\n\n",
		result.Summary.SelectedHosts,
		result.Summary.MetricHosts,
		periodName,
		metricName,
		limit,
	))
	b.WriteString(fmt.Sprintf("| 排名 | 主机名 | IP | %s%s使用率 | 当前 CPU | 当前内存 | 当前磁盘 |\n", periodName, metricName))
	b.WriteString("| ---: | --- | --- | ---: | ---: | ---: | ---: |\n")
	for index := 0; index < limit; index++ {
		item := result.Candidates[index]
		b.WriteString(fmt.Sprintf("| %d | %s | %s | %s | %s | %s | %s |\n",
			index+1,
			escapeAssistantMarkdownCell(firstNonEmptyAssistantText(item.HostName, item.Name, "-")),
			escapeAssistantMarkdownCell(firstNonEmptyAssistantText(item.IP, item.PrivateIP, item.PublicIP, item.SSHIP, "-")),
			formatAssistantPercent(assistantRankingValue(item, result.Ranking)),
			formatAssistantPercent(item.CurrentCPU),
			formatAssistantPercent(item.CurrentMemory),
			formatAssistantPercent(item.CurrentDisk),
		))
	}
	return strings.TrimSpace(b.String())
}

func assistantRankingMetricName(ranking *aiopsmodel.AssistantQueryRanking) string {
	if ranking == nil {
		return "资源"
	}
	switch ranking.Metric {
	case "memory":
		return "内存"
	case "disk":
		return "磁盘"
	default:
		return "CPU"
	}
}

func assistantCandidateTitle(intent string) string {
	switch intent {
	case aiopsmodel.AssistantIntentResourceReclaim:
		return "回收候选主机"
	case aiopsmodel.AssistantIntentCapacityInspection:
		return "需要关注的主机"
	case aiopsmodel.AssistantIntentAssetQuery:
		return "主机资产"
	default:
		return "部署候选主机"
	}
}

func formatAssistantBusinessServices(item aiopsmodel.AssistantHostCandidate) string {
	if item.BusinessServiceCount > 0 {
		return fmt.Sprintf("疑似业务 %d 个", item.BusinessServiceCount)
	}
	if item.ServiceCount > 0 {
		return fmt.Sprintf("未发现业务服务；基础服务 %d 个", item.ServiceCount)
	}
	return "未发现业务服务"
}

func formatAssistantPercent(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}

func firstNonEmptyAssistantText(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func escapeAssistantMarkdownCell(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return strings.TrimSpace(value)
}

func detectAssistantIntent(question string) string {
	q := strings.ToLower(strings.TrimSpace(question))
	if containsAnyText(q, "部署", "新部署", "发布", "上线", "扩容", "选机器", "找几台", "资源充足", "放到", "安装") {
		return aiopsmodel.AssistantIntentResourcePlacement
	}
	if containsAnyText(q, "回收", "释放", "闲置", "长期低", "长期比较低", "没有部署", "未部署", "空闲机器", "低负载") {
		return aiopsmodel.AssistantIntentResourceReclaim
	}
	if containsAnyText(q, "巡检", "压力", "使用率高", "高负载", "容量", "瓶颈", "哪些主机高", "哪些机器高") {
		return aiopsmodel.AssistantIntentCapacityInspection
	}
	return aiopsmodel.AssistantIntentAssetQuery
}

func normalizeAssistantRequest(req *aiopsmodel.AssistantQueryRequest, intent string) {
	if req.Scope.Limit <= 0 {
		req.Scope.Limit = defaultAssistantLimit
	}
	if req.Scope.Limit > maxAssistantLimit {
		req.Scope.Limit = maxAssistantLimit
	}
	if req.Scope.TimeRangeDays <= 0 {
		req.Scope.TimeRangeDays = defaultTimeRangeForIntent(intent)
	}
	if req.Scope.TimeRangeDays > 90 {
		req.Scope.TimeRangeDays = 90
	}
	applyRequirementsFromQuestion(req)
	applyThresholdDefaults(req, intent)
}

func defaultTimeRangeForIntent(intent string) int {
	switch intent {
	case aiopsmodel.AssistantIntentResourceReclaim:
		return defaultReclaimTimeRangeDays
	case aiopsmodel.AssistantIntentCapacityInspection:
		return defaultInspectionTimeRangeDays
	default:
		return defaultPlacementTimeRangeDays
	}
}

func applyRequirementsFromQuestion(req *aiopsmodel.AssistantQueryRequest) {
	if match := compactSpecRegex.FindStringSubmatch(req.Question); len(match) == 3 {
		if req.Options.RequiredCPUCores <= 0 {
			req.Options.RequiredCPUCores = parseAssistantNumber(match[1])
		}
		if req.Options.RequiredMemoryGB <= 0 {
			req.Options.RequiredMemoryGB = parseAssistantNumber(match[2])
		}
	}
	if match := cpuRequirementRegex.FindStringSubmatch(req.Question); len(match) >= 2 && req.Options.RequiredCPUCores <= 0 {
		req.Options.RequiredCPUCores = parseAssistantNumber(match[1])
	}
	if match := memoryRequirementRegex.FindStringSubmatch(req.Question); len(match) >= 2 && req.Options.RequiredMemoryGB <= 0 {
		req.Options.RequiredMemoryGB = parseAssistantNumber(match[1])
	}
	if match := diskRequirementRegex.FindStringSubmatch(req.Question); len(match) >= 2 && req.Options.RequiredDiskGB <= 0 {
		req.Options.RequiredDiskGB = parseAssistantNumber(match[1])
	}
	if match := daysRegex.FindStringSubmatch(req.Question); len(match) >= 2 {
		if days := int(parseAssistantNumber(match[1])); days > 0 {
			req.Scope.TimeRangeDays = days
		}
	}
}

func applyThresholdDefaults(req *aiopsmodel.AssistantQueryRequest, intent string) {
	switch intent {
	case aiopsmodel.AssistantIntentResourceReclaim:
		if req.Options.CPUThreshold <= 0 {
			req.Options.CPUThreshold = defaultReclaimCPUThreshold
		}
		if req.Options.MemoryThreshold <= 0 {
			req.Options.MemoryThreshold = defaultReclaimMemoryThreshold
		}
		if req.Options.DiskThreshold <= 0 {
			req.Options.DiskThreshold = defaultReclaimDiskThreshold
		}
	default:
		if req.Options.CPUThreshold <= 0 {
			req.Options.CPUThreshold = defaultPlacementCPUThreshold
		}
		if req.Options.MemoryThreshold <= 0 {
			req.Options.MemoryThreshold = defaultPlacementMemoryThreshold
		}
		if req.Options.DiskThreshold <= 0 {
			req.Options.DiskThreshold = defaultPlacementDiskThreshold
		}
	}
}

func filterHostsByEnv(hosts []cmdbmodel.CmdbHost, env string) []cmdbmodel.CmdbHost {
	env = strings.ToLower(strings.TrimSpace(env))
	if env == "" {
		return hosts
	}
	result := make([]cmdbmodel.CmdbHost, 0, len(hosts))
	for _, host := range hosts {
		text := strings.ToLower(strings.Join([]string{host.Tag, host.Remark, host.Group.Name, host.HostName, host.Name}, " "))
		if strings.Contains(text, env) || strings.Contains(text, "env="+env) || strings.Contains(text, "环境="+env) {
			result = append(result, host)
		}
	}
	return result
}

func prometheusURL() string {
	if config.Config == nil {
		return ""
	}
	return strings.TrimSpace(config.Config.Monitor.Prometheus.URL)
}

func assistantMetricIdentifier(host cmdbmodel.CmdbHost) string {
	for _, value := range []string{host.PrivateIP, host.SSHIP, host.Name, host.HostName} {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func assistantHostIP(host cmdbmodel.CmdbHost) string {
	for _, value := range []string{host.PrivateIP, host.SSHIP, host.PublicIP} {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return host.Name
}

func parsePrometheusFloat(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	case float64:
		return typed, true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func parseCPUValue(value string) float64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	match := numberWithUnitRegex.FindStringSubmatch(value)
	if len(match) < 2 {
		return 0
	}
	return parseAssistantNumber(match[1])
}

func serviceUsageNames(services []aiopsmodel.AssistantServiceUsage) []string {
	names := make([]string, 0, len(services))
	for _, service := range services {
		names = append(names, service.Name)
	}
	return names
}

func countBusinessServices(services []aiopsmodel.AssistantServiceUsage) int {
	count := 0
	for _, service := range services {
		if !service.Platform {
			count++
		}
	}
	return count
}

func classifyPlatformService(name string) (bool, string) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return true, "empty service name"
	}
	platformPrefixes := []string{
		"dodevops-",
		"node_exporter",
		"windows_exporter",
		"process-exporter",
		"prometheus",
		"pushgateway",
		"grafana",
		"alertmanager",
		"victoria-",
		"containerd",
		"dockerd",
		"docker",
		"conmon",
		"aardvark-dns",
		"chronyd",
		"systemd",
		"sshd",
		"rsyslog",
		"telegraf",
		"filebeat",
		"metricbeat",
		"cadvisor",
	}
	for _, prefix := range platformPrefixes {
		if strings.HasPrefix(normalized, prefix) || normalized == prefix {
			return true, "platform or infrastructure service"
		}
	}
	return false, ""
}

func parseSizeGB(value string) float64 {
	matches := numberWithUnitRegex.FindAllStringSubmatch(value, -1)
	if len(matches) == 0 {
		return 0
	}
	total := 0.0
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		number := parseAssistantNumber(match[1])
		unit := ""
		if len(match) >= 3 {
			unit = strings.ToLower(match[2])
		}
		switch unit {
		case "tb", "t":
			total += number * 1024
		case "mb", "m":
			total += number / 1024
		default:
			total += number
		}
	}
	return roundAssistantFloat(total)
}

func parseAssistantNumber(value string) float64 {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed
}

func roundAssistantFloat(value float64) float64 {
	parsed, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", value), 64)
	return parsed
}

func clampAssistantScore(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func containsAnyText(value string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(value, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func appendMissing(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}
