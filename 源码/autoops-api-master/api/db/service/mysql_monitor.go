package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	cmdbmodel "dodevops-api/api/cmdb/model"
	accountmodel "dodevops-api/api/configcenter/model"
	dbmodel "dodevops-api/api/db/model"
	"dodevops-api/pkg/log"
)

const mysqlMonitorDefaultRange = 6 * time.Hour
const mysqlMonitorDefaultStep = "1m"

type prometheusClient struct {
	baseURL  string
	username string
	password string
	client   *http.Client
}

type prometheusAPIResponse struct {
	Status string          `json:"status"`
	Error  string          `json:"error"`
	Data   json.RawMessage `json:"data"`
}

type prometheusQueryData struct {
	ResultType string                  `json:"resultType"`
	Result     []prometheusQueryResult `json:"result"`
}

type prometheusQueryResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
	Values [][]interface{}   `json:"values"`
}

type prometheusTargetsData struct {
	ActiveTargets []prometheusTarget `json:"activeTargets"`
}

type prometheusTarget struct {
	DiscoveredLabels map[string]string `json:"discoveredLabels"`
	Labels           map[string]string `json:"labels"`
	ScrapePool       string            `json:"scrapePool"`
	ScrapeURL        string            `json:"scrapeUrl"`
	GlobalURL        string            `json:"globalUrl"`
	Health           string            `json:"health"`
	LastError        string            `json:"lastError"`
}

type mysqlMonitorMetricDef struct {
	Key            string
	Name           string
	Unit           string
	PromQL         string
	FallbackPromQL string
	Status         func(float64) string
	Description    string
	Required       bool
}

func (d mysqlMonitorMetricDef) optionalMissing() bool {
	switch d.Key {
	case "replicationLag", "slaveIORunning", "slaveSQLRunning":
		return true
	default:
		return false
	}
}

type mysqlMonitorTrendDef struct {
	Key    string
	Title  string
	Unit   string
	Series []mysqlMonitorMetricDef
}

func (s *DatabaseService) MySQLMonitorDashboard(ctx context.Context, assetID uint, query dbmodel.MySQLMonitorQuery) (*dbmodel.MySQLMonitorDashboard, error) {
	logger := log.Log().WithField("module", "mysql_monitor").WithField("assetID", assetID)
	logger.WithFields(map[string]interface{}{
		"range": query.Range,
		"step":  query.Step,
		"start": query.Start,
		"end":   query.End,
	}).Info("mysql monitor request started")

	asset, err := s.assetDao.GetByID(assetID)
	if err != nil {
		logger.WithError(err).Error("get database asset failed")
		return nil, err
	}
	logger = logger.WithFields(map[string]interface{}{
		"assetName":           asset.Name,
		"instanceName":        asset.InstanceName,
		"hostIP":              asset.HostIP,
		"endpoint":            asset.Endpoint,
		"port":                asset.Port,
		"prometheusAccountID": asset.PrometheusAccountID,
	})
	logger.Info("mysql monitor asset loaded")
	if asset.Type != cmdbmodel.DatabaseTypeMySQL {
		logger.WithField("assetType", asset.Type).Warn("database asset is not mysql")
		return nil, fmt.Errorf("database asset is not MySQL")
	}
	if asset.PrometheusAccountID == 0 {
		logger.Warn("database asset has no prometheus account")
		return nil, fmt.Errorf("database asset has no prometheusAccountId")
	}

	account, err := s.accountDao.GetByID(asset.PrometheusAccountID)
	if err != nil {
		logger.WithError(err).Error("get prometheus account failed")
		return nil, fmt.Errorf("get prometheus account failed: %w", err)
	}
	password, err := account.DecryptPassword()
	if err != nil {
		logger.WithError(err).Error("decrypt prometheus account password failed")
		return nil, fmt.Errorf("decrypt prometheus account password failed: %w", err)
	}

	client := newPrometheusClient(account, password)
	logger = logger.WithField("prometheus", client.baseURL)
	start, end, step := normalizeMySQLMonitorRange(query)
	target, warnings := s.resolveMySQLPrometheusTarget(ctx, client, asset)
	selector := target.Selector
	if selector == "" {
		selector = strings.TrimSpace(asset.PrometheusLabelSelector)
	}
	logger.WithFields(map[string]interface{}{
		"matched":   target.Matched,
		"health":    target.Health,
		"matchBy":   target.MatchBy,
		"scrapeURL": target.ScrapeURL,
		"selector":  selector,
		"warnings":  warnings,
	}).Info("mysql monitor target resolved")

	dashboard := &dbmodel.MySQLMonitorDashboard{
		Asset:    buildAssetVO(*asset),
		Target:   target,
		Warnings: warnings,
		Range: dbmodel.MonitorQueryRange{
			Start: start.UnixMilli(),
			End:   end.UnixMilli(),
			Step:  step,
		},
	}
	dashboard.Asset.PrometheusAccountAlias = account.Alias
	dashboard.Asset.PrometheusEndpoint = client.baseURL

	if !target.Matched || strings.TrimSpace(selector) == "" {
		dashboard.Warnings = append(dashboard.Warnings, "mysql monitor target is not matched, fallback to global mysql metrics")
		logger.WithField("warnings", dashboard.Warnings).Warn("mysql monitor target not matched, fallback to global mysql metrics")
	}

	overview, overviewWarnings := s.queryMySQLOverview(ctx, client, asset, selector)
	dashboard.Overview = overview
	dashboard.Warnings = append(dashboard.Warnings, overviewWarnings...)

	trends, trendWarnings := s.queryMySQLTrends(ctx, client, asset, selector, start, end, step)
	dashboard.Trends = trends
	dashboard.Warnings = append(dashboard.Warnings, trendWarnings...)

	breakdown, breakdownWarnings := s.queryMySQLBreakdown(ctx, client, selector)
	dashboard.Breakdown = breakdown
	dashboard.Warnings = append(dashboard.Warnings, breakdownWarnings...)
	logger.WithFields(map[string]interface{}{
		"overviewCount":  len(dashboard.Overview),
		"trendsCount":    len(dashboard.Trends),
		"breakdownCount": len(dashboard.Breakdown),
		"warnings":       dashboard.Warnings,
	}).Info("mysql monitor request finished")
	return dashboard, nil
}

func newPrometheusClient(account *accountmodel.AccountAuth, password string) *prometheusClient {
	return &prometheusClient{
		baseURL:  buildPrometheusEndpoint(account.Host, account.Port),
		username: account.Name,
		password: password,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

func normalizeMySQLMonitorRange(query dbmodel.MySQLMonitorQuery) (time.Time, time.Time, string) {
	now := time.Now()
	end := now
	if query.End > 0 {
		end = time.UnixMilli(query.End)
	}
	start := end.Add(-mysqlMonitorDefaultRange)
	if query.Start > 0 {
		start = time.UnixMilli(query.Start)
	} else if duration, err := time.ParseDuration(strings.TrimSpace(query.Range)); err == nil && duration > 0 {
		start = end.Add(-duration)
	}
	step := strings.TrimSpace(query.Step)
	if step == "" {
		step = mysqlMonitorDefaultStep
	}
	if !start.Before(end) {
		start = end.Add(-mysqlMonitorDefaultRange)
	}
	return start, end, step
}

func (s *DatabaseService) resolveMySQLPrometheusTarget(ctx context.Context, client *prometheusClient, asset *cmdbmodel.CmdbSQL) (dbmodel.MySQLMonitorTarget, []string) {
	logger := log.Log().WithFields(map[string]interface{}{
		"module":     "mysql_monitor",
		"assetID":    asset.ID,
		"assetName":  asset.Name,
		"hostIP":     asset.HostIP,
		"endpoint":   asset.Endpoint,
		"prometheus": client.baseURL,
	})
	target := dbmodel.MySQLMonitorTarget{Prometheus: client.baseURL}
	if selector := strings.TrimSpace(asset.PrometheusLabelSelector); selector != "" {
		target.Matched = true
		target.Selector = selector
		target.MatchBy = "prometheusLabelSelector"
		target.MetricsURL = defaultDatabaseMetricsURL(asset, "mysql")
		target.ScrapeURL = target.MetricsURL
		enrichMonitorTargetDisplay(client, asset, &target, "mysql")
		logger.WithField("selector", selector).Info("mysql monitor target matched by manual selector")
		return target, nil
	}

	assetLabelTarget, err := resolveMySQLTargetFromAssetLabels(ctx, client, asset)
	if err == nil {
		logger.WithFields(map[string]interface{}{
			"selector": assetLabelTarget.Selector,
			"labels":   assetLabelTarget.Labels,
		}).Info("mysql monitor target matched by asset labels")
		return assetLabelTarget, nil
	}
	logger.WithError(err).Warn("mysql monitor asset label match failed")
	warnings := []string{"asset label match failed: " + err.Error()}

	metricTarget, err := resolveMySQLTargetFromMetric(ctx, client, asset)
	if err == nil {
		logger.WithFields(map[string]interface{}{
			"selector": metricTarget.Selector,
			"labels":   metricTarget.Labels,
		}).Info("mysql monitor target matched by mysql_up labels")
		return metricTarget, nil
	}
	logger.WithError(err).Warn("mysql monitor mysql_up metric match failed")
	warnings = append(warnings, "mysql_up metric match failed: "+err.Error())

	targets, err := client.targets(ctx)
	if err != nil {
		logger.WithError(err).Error("query prometheus targets failed")
		return target, append(warnings, "query prometheus targets failed: "+err.Error())
	}
	logger.WithField("targetCount", len(targets)).Info("mysql monitor prometheus targets loaded")

	if exact, ok := findTargetByAssetAddress(asset, targets); ok {
		labels := mergeTargetLabels(exact)
		logger.WithFields(map[string]interface{}{
			"address":   labels["__address__"],
			"scrapeURL": exact.ScrapeURL,
			"labels":    labels,
		}).Info("mysql monitor target matched by exact address")
		return buildMonitorTargetFromPrometheusTarget(client, asset, exact), warnings
	}

	bestScore := 0
	var best prometheusTarget
	for _, item := range targets {
		score := scoreMySQLTarget(asset, item)
		if score > bestScore {
			bestScore = score
			best = item
		}
	}
	logger.WithField("bestScore", bestScore).Info("mysql monitor target scored")

	if bestScore == 0 {
		logger.WithField("warnings", warnings).Warn("mysql monitor no matching target found")
		return target, append(warnings, "no matching mysql prometheus target found")
	}

	target = buildMonitorTargetFromPrometheusTarget(client, asset, best)
	if target.Selector == "" {
		logger.WithField("labels", target.Labels).Warn("mysql monitor matched target has no stable selector")
		return target, append(warnings, "matched target has no stable labels for selector")
	}
	logger.WithFields(map[string]interface{}{
		"selector":  target.Selector,
		"matchBy":   target.MatchBy,
		"scrapeURL": target.ScrapeURL,
		"labels":    target.Labels,
	}).Info("mysql monitor target matched by score")
	return target, warnings
}

func findTargetByAssetAddress(asset *cmdbmodel.CmdbSQL, targets []prometheusTarget) (prometheusTarget, bool) {
	for _, item := range targets {
		labels := mergeTargetLabels(item)
		haystack := prometheusTargetHaystack(item, labels)
		if !isMySQLTargetCandidate(labels, haystack) {
			continue
		}
		if targetAddressMatchesAsset(asset, strings.TrimSpace(labels["__address__"])) {
			return item, true
		}
		if targetAddressMatchesAsset(asset, item.ScrapeURL) {
			return item, true
		}
		if targetAddressMatchesAsset(asset, item.GlobalURL) {
			return item, true
		}
	}
	return prometheusTarget{}, false
}

func buildMonitorTargetFromPrometheusTarget(client *prometheusClient, asset *cmdbmodel.CmdbSQL, item prometheusTarget) dbmodel.MySQLMonitorTarget {
	labels := mergeTargetLabels(item)
	metricsURL := firstNonEmpty(item.ScrapeURL, item.GlobalURL)
	target := dbmodel.MySQLMonitorTarget{
		Matched:    true,
		Health:     item.Health,
		ScrapeURL:  metricsURL,
		MetricsURL: metricsURL,
		LastError:  item.LastError,
		Labels:     labels,
		Selector:   buildPrometheusSelector(labels),
		MatchBy:    mysqlTargetMatchBy(asset, item),
		Prometheus: client.baseURL,
	}
	enrichMonitorTargetDisplay(client, asset, &target, "mysql")
	return target
}

func resolveMySQLTargetFromAssetLabels(ctx context.Context, client *prometheusClient, asset *cmdbmodel.CmdbSQL) (dbmodel.MySQLMonitorTarget, error) {
	target := dbmodel.MySQLMonitorTarget{Prometheus: client.baseURL}
	labels := buildMySQLAssetLabels(asset)
	if len(labels) == 0 {
		return target, fmt.Errorf("asset hostIp is not a valid IPv4 address")
	}
	selector := buildPrometheusSelector(labels)
	if selector == "" {
		return target, fmt.Errorf("empty asset label selector")
	}
	query := mysqlMetricQuery("mysql_up", selector)
	log.Log().WithFields(map[string]interface{}{
		"module":    "mysql_monitor",
		"assetID":   asset.ID,
		"assetName": asset.Name,
		"query":     query,
		"selector":  selector,
		"labels":    labels,
	}).Info("mysql monitor verifying asset label selector")
	if _, err := client.queryInstantValue(ctx, query); err != nil {
		return target, fmt.Errorf("%s: %w", query, err)
	}
	target.Matched = true
	target.Health = "up"
	target.Labels = labels
	target.Selector = selector
	target.MatchBy = "assetLabels"
	enrichMySQLAssetLabelTarget(ctx, client, asset, &target)
	return target, nil
}

func enrichMySQLAssetLabelTarget(ctx context.Context, client *prometheusClient, asset *cmdbmodel.CmdbSQL, target *dbmodel.MySQLMonitorTarget) {
	if target == nil {
		return
	}
	targets, err := client.targets(ctx)
	if err != nil {
		log.Log().WithFields(map[string]interface{}{
			"module":    "mysql_monitor",
			"assetID":   asset.ID,
			"assetName": asset.Name,
			"error":     err.Error(),
		}).Warn("mysql monitor enrich asset label target failed")
		if target.ScrapeURL == "" {
			target.ScrapeURL = fmt.Sprintf("http://%s:9104/metrics", strings.TrimSpace(asset.HostIP))
		}
		enrichMonitorTargetDisplay(client, asset, target, "mysql")
		return
	}
	for _, item := range targets {
		labels := mergeTargetLabels(item)
		if !isMySQLTargetCandidate(labels, prometheusTargetHaystack(item, labels)) {
			continue
		}
		if !targetLabelsContain(labels, target.Labels) {
			continue
		}
		target.Health = item.Health
		target.ScrapeURL = firstNonEmpty(item.ScrapeURL, item.GlobalURL)
		target.MetricsURL = target.ScrapeURL
		target.LastError = item.LastError
		target.Labels = labels
		target.Selector = buildPrometheusSelector(labels)
		target.MatchBy = mysqlTargetMatchBy(asset, item)
		if target.MatchBy == "auto" {
			target.MatchBy = "assetLabels"
		}
		log.Log().WithFields(map[string]interface{}{
			"module":    "mysql_monitor",
			"assetID":   asset.ID,
			"assetName": asset.Name,
			"scrapeURL": target.ScrapeURL,
			"labels":    target.Labels,
		}).Info("mysql monitor asset label target enriched from targets api")
		enrichMonitorTargetDisplay(client, asset, target, "mysql")
		return
	}
	if target.ScrapeURL == "" && strings.TrimSpace(asset.HostIP) != "" {
		target.ScrapeURL = fmt.Sprintf("http://%s:9104/metrics", strings.TrimSpace(asset.HostIP))
	}
	target.MetricsURL = target.ScrapeURL
	enrichMonitorTargetDisplay(client, asset, target, "mysql")
}

func targetLabelsContain(labels, expected map[string]string) bool {
	if expectedJob := strings.TrimSpace(expected["job"]); expectedJob != "" {
		if job := strings.TrimSpace(labels["job"]); job != "" && job != expectedJob {
			return false
		}
	}
	for _, key := range []string{"instance", "alias", "hostname"} {
		if expectedValue := strings.TrimSpace(expected[key]); expectedValue != "" && strings.TrimSpace(labels[key]) == expectedValue {
			return true
		}
	}
	for _, key := range []string{"__address__", "scrape_url"} {
		if strings.TrimSpace(labels[key]) == "" {
			continue
		}
		for _, expectedValue := range expected {
			if expectedValue != "" && strings.Contains(strings.ToLower(labels[key]), strings.ToLower(expectedValue)) {
				return true
			}
		}
	}
	return false
}

func buildMySQLAssetLabels(asset *cmdbmodel.CmdbSQL) map[string]string {
	ipParts := strings.Split(strings.TrimSpace(asset.HostIP), ".")
	if len(ipParts) != 4 {
		return nil
	}
	suffix := ipParts[2] + "_" + ipParts[3]
	hostSuffix := ipParts[2] + "-" + ipParts[3]
	return map[string]string{
		"job":      "mysql",
		"instance": "db" + suffix,
		"alias":    "mysql" + suffix,
		"hostname": "node" + hostSuffix,
	}
}

func resolveMySQLTargetFromMetric(ctx context.Context, client *prometheusClient, asset *cmdbmodel.CmdbSQL) (dbmodel.MySQLMonitorTarget, error) {
	target := dbmodel.MySQLMonitorTarget{Prometheus: client.baseURL}
	rows, err := client.queryInstantMetricRows(ctx, "mysql_up")
	if err != nil {
		return target, err
	}

	bestScore := 0
	bestLabels := map[string]string{}
	for _, row := range rows {
		score := scoreMySQLMetricLabels(asset, row.Metric)
		if score > bestScore {
			bestScore = score
			bestLabels = row.Metric
		}
	}
	if bestScore == 0 {
		return target, fmt.Errorf("no matching mysql_up series found")
	}

	labels := clonePrometheusLabels(bestLabels)
	target.Matched = true
	target.Health = "up"
	target.Labels = labels
	target.Selector = buildPrometheusSelector(labels)
	target.MatchBy = "mysql_up"
	target.ScrapeURL = buildScrapeURLFromMetricLabels(labels)
	target.MetricsURL = target.ScrapeURL
	enrichMonitorTargetDisplay(client, asset, &target, "mysql")
	if target.Selector == "" {
		return target, fmt.Errorf("matched mysql_up series has no stable labels")
	}
	return target, nil
}

func buildScrapeURLFromMetricLabels(labels map[string]string) string {
	address := strings.TrimSpace(labels["__address__"])
	if address == "" {
		return ""
	}
	path := strings.TrimSpace(labels["__metrics_path__"])
	if path == "" {
		path = "/metrics"
	}
	scheme := strings.TrimSpace(labels["__scheme__"])
	if scheme == "" {
		scheme = "http"
	}
	return scheme + "://" + address + path
}

func scoreMySQLMetricLabels(asset *cmdbmodel.CmdbSQL, labels map[string]string) int {
	haystackParts := make([]string, 0, len(labels))
	for key, value := range labels {
		haystackParts = append(haystackParts, key+"="+value)
	}
	haystack := strings.ToLower(strings.Join(haystackParts, " "))

	score := 0
	for _, key := range []string{"instance", "alias", "hostname", "job"} {
		if targetAddressMatchesAsset(asset, strings.TrimSpace(labels[key])) {
			score += 80
		}
	}
	for _, needle := range mysqlTargetNeedles(asset) {
		if needle != "" && strings.Contains(haystack, strings.ToLower(needle)) {
			score += 10
		}
	}
	return score
}

func clonePrometheusLabels(labels map[string]string) map[string]string {
	result := make(map[string]string, len(labels))
	for key, value := range labels {
		result[key] = value
	}
	return result
}

func scoreMySQLTarget(asset *cmdbmodel.CmdbSQL, target prometheusTarget) int {
	labels := mergeTargetLabels(target)
	address := strings.TrimSpace(labels["__address__"])
	haystack := prometheusTargetHaystack(target, labels)
	if !isMySQLTargetCandidate(labels, haystack) {
		return 0
	}

	score := 0
	if strings.Contains(haystack, "mysql") {
		score += 5
	}
	if targetAddressMatchesAsset(asset, address) {
		score += 100
	}
	for _, needle := range mysqlTargetNeedles(asset) {
		if needle != "" && strings.Contains(haystack, strings.ToLower(needle)) {
			score += 10
		}
	}
	return score
}

func isMySQLTargetCandidate(labels map[string]string, haystack string) bool {
	if isNodeExporterTarget(labels, haystack) {
		return false
	}
	for _, key := range []string{"job", "role", "service", "auth_module", "__param_auth_module"} {
		if strings.Contains(strings.ToLower(strings.TrimSpace(labels[key])), "mysql") {
			return true
		}
	}
	return strings.Contains(haystack, "mysql")
}

func prometheusTargetHaystack(target prometheusTarget, labels map[string]string) string {
	haystackParts := []string{target.ScrapePool, target.ScrapeURL, target.GlobalURL}
	for key, value := range labels {
		haystackParts = append(haystackParts, key+"="+value)
	}
	return strings.ToLower(strings.Join(haystackParts, " "))
}

func enrichMonitorTargetDisplay(client *prometheusClient, asset *cmdbmodel.CmdbSQL, target *dbmodel.MySQLMonitorTarget, metricsType string) {
	if target == nil {
		return
	}
	target.Prometheus = client.baseURL
	target.MetricsType = metricsType
	target.TargetAddress = monitorTargetAddress(asset, target, metricsType)
	target.TargetHost = monitorTargetHost(asset, target.TargetAddress)
	if target.MetricsURL == "" {
		target.MetricsURL = target.ScrapeURL
	}
	if target.ScrapeURL == "" {
		target.ScrapeURL = target.MetricsURL
	}
	if target.MetricsURL == "" {
		target.MetricsURL = defaultDatabaseMetricsURL(asset, metricsType)
	}
	if target.ScrapeURL == "" {
		target.ScrapeURL = target.MetricsURL
	}
	target.HostSelector = nodeInstanceSelector(target.TargetHost)
	if target.HostScrapeURL == "" {
		target.HostScrapeURL = resolveNodeExporterScrapeURL(client, asset, target.TargetHost)
	}
}

func resolveNodeExporterScrapeURL(client *prometheusClient, asset *cmdbmodel.CmdbSQL, targetHost string) string {
	targets, err := client.targets(context.Background())
	if err != nil {
		return defaultNodeExporterScrapeURL(asset, targetHost)
	}
	bestScore := 0
	bestURL := ""
	for _, item := range targets {
		labels := mergeTargetLabels(item)
		haystack := prometheusTargetHaystack(item, labels)
		if !isNodeExporterTarget(labels, haystack) {
			continue
		}
		score := 0
		for _, key := range []string{"instance", "__address__"} {
			if hostMatchesAddress(targetHost, strings.TrimSpace(labels[key])) {
				score += 100
			}
		}
		if hostMatchesAddress(targetHost, item.ScrapeURL) || hostMatchesAddress(targetHost, item.GlobalURL) {
			score += 50
		}
		if score > bestScore {
			bestScore = score
			bestURL = firstNonEmpty(item.ScrapeURL, item.GlobalURL)
		}
	}
	if bestURL != "" {
		return bestURL
	}
	return defaultNodeExporterScrapeURL(asset, targetHost)
}

func defaultNodeExporterScrapeURL(asset *cmdbmodel.CmdbSQL, targetHost string) string {
	host := strings.TrimSpace(targetHost)
	if host == "" {
		host = strings.TrimSpace(asset.HostIP)
	}
	if host == "" {
		return ""
	}
	return fmt.Sprintf("http://%s:9100/metrics", host)
}

func defaultDatabaseMetricsURL(asset *cmdbmodel.CmdbSQL, metricsType string) string {
	host := strings.TrimSpace(asset.HostIP)
	if host == "" {
		if endpointHost, _, ok := strings.Cut(strings.TrimSpace(asset.Endpoint), ":"); ok {
			host = endpointHost
		}
	}
	if host == "" {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(metricsType)) {
	case "postgresql", "postgres", "pgsql":
		return fmt.Sprintf("http://%s:9187/probe", host)
	case "redis":
		return fmt.Sprintf("http://%s:9121/scrape", host)
	case "mysql":
		return fmt.Sprintf("http://%s:9104/metrics", host)
	default:
		return ""
	}
}

func monitorTargetAddress(asset *cmdbmodel.CmdbSQL, target *dbmodel.MySQLMonitorTarget, metricsType string) string {
	if target != nil && target.Labels != nil {
		for _, key := range []string{"__address__", "target", "__param_target", "addr", "instance"} {
			if value := strings.TrimSpace(target.Labels[key]); value != "" {
				switch strings.ToLower(strings.TrimSpace(metricsType)) {
				case "postgresql", "postgres", "pgsql", "redis":
					return value
				case "mysql":
					if key == "__address__" || key == "instance" {
						return value
					}
				default:
					return value
				}
			}
		}
	}
	if strings.TrimSpace(asset.Endpoint) != "" {
		return strings.TrimSpace(asset.Endpoint)
	}
	return strings.TrimSpace(asset.HostIP)
}

func monitorTargetHost(asset *cmdbmodel.CmdbSQL, address string) string {
	if host := extractHostFromAddress(address); host != "" {
		return host
	}
	if host := extractHostFromAddress(asset.Endpoint); host != "" {
		return host
	}
	return strings.TrimSpace(asset.HostIP)
}

func extractHostFromAddress(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	if parsed, err := url.Parse(address); err == nil && parsed.Host != "" {
		if host, _, err := net.SplitHostPort(parsed.Host); err == nil {
			return strings.Trim(host, "[]")
		}
		return strings.Trim(parsed.Hostname(), "[]")
	}
	if host, _, err := net.SplitHostPort(address); err == nil {
		return strings.Trim(host, "[]")
	}
	if strings.Contains(address, ":") {
		host, _, ok := strings.Cut(address, ":")
		if ok {
			return strings.Trim(host, "[]")
		}
	}
	return strings.Trim(address, "[]")
}

func hostMatchesAddress(host, address string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	address = strings.ToLower(strings.TrimSpace(address))
	if address == "" {
		return false
	}
	if address == host || strings.HasPrefix(address, host+":") || strings.Contains(address, host) {
		return true
	}
	return extractHostFromAddress(address) == host
}

func mysqlTargetMatchBy(asset *cmdbmodel.CmdbSQL, target prometheusTarget) string {
	labels := mergeTargetLabels(target)
	if targetAddressMatchesAsset(asset, strings.TrimSpace(labels["__address__"])) {
		return "__address__"
	}
	for _, key := range []string{"instance", "alias", "hostname", "job"} {
		value := strings.TrimSpace(labels[key])
		if value == "" {
			continue
		}
		for _, needle := range mysqlTargetNeedles(asset) {
			if needle != "" && strings.Contains(strings.ToLower(value), strings.ToLower(needle)) {
				return key
			}
		}
	}
	return "auto"
}

func targetAddressMatchesAsset(asset *cmdbmodel.CmdbSQL, address string) bool {
	address = strings.ToLower(strings.TrimSpace(address))
	if address == "" {
		return false
	}
	for _, host := range assetHostCandidates(asset) {
		host = strings.ToLower(strings.TrimSpace(host))
		if host == "" {
			continue
		}
		if address == host || strings.HasPrefix(address, host+":") || strings.Contains(address, host) {
			return true
		}
	}
	return false
}

func assetHostCandidates(asset *cmdbmodel.CmdbSQL) []string {
	candidates := []string{asset.HostIP}
	if host, _, ok := strings.Cut(strings.TrimSpace(asset.Endpoint), ":"); ok {
		candidates = append(candidates, host)
	}
	return candidates
}

func mysqlTargetNeedles(asset *cmdbmodel.CmdbSQL) []string {
	needles := []string{asset.HostIP, asset.Endpoint, asset.Name, asset.InstanceName}
	ipParts := strings.Split(asset.HostIP, ".")
	if len(ipParts) == 4 {
		needles = append(needles,
			ipParts[2]+"."+ipParts[3],
			ipParts[2]+"_"+ipParts[3],
			"mysql"+ipParts[2]+"_"+ipParts[3],
			"db"+ipParts[2]+"_"+ipParts[3],
			"node"+ipParts[2]+"-"+ipParts[3],
		)
	}
	return needles
}

func mergeTargetLabels(target prometheusTarget) map[string]string {
	labels := make(map[string]string)
	for key, value := range target.DiscoveredLabels {
		labels[key] = value
	}
	for key, value := range target.Labels {
		labels[key] = value
	}
	return labels
}

func buildPrometheusSelector(labels map[string]string) string {
	for _, keys := range [][]string{
		{"job", "instance"},
		{"instance"},
		{"alias"},
		{"hostname"},
		{"job"},
	} {
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			if value := strings.TrimSpace(labels[key]); value != "" {
				parts = append(parts, fmt.Sprintf(`%s="%s"`, key, escapePrometheusLabelValue(value)))
			}
		}
		if len(parts) == len(keys) {
			return strings.Join(parts, ",")
		}
	}
	return ""
}

func escapePrometheusLabelValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}

func (s *DatabaseService) queryMySQLOverview(ctx context.Context, client *prometheusClient, asset *cmdbmodel.CmdbSQL, selector string) ([]dbmodel.MonitorMetricCard, []string) {
	nodeSelector := nodeInstanceSelector(asset.HostIP)
	defs := []mysqlMonitorMetricDef{
		{Key: "up", Name: "MySQL 状态", Unit: "", PromQL: mysqlMaxMetricQuery("mysql_up", selector), FallbackPromQL: mysqlMaxMetricQuery("mysql_up", ""), Status: statusUp, Description: "1 表示 exporter 可连接 MySQL"},
		{Key: "cpuUsage", Name: "主机 CPU 使用率", Unit: "%", PromQL: nodeCPUUsageQuery(nodeSelector), Status: statusPercent, Description: "资产 HostIP 对应 node_exporter 采集的主机 CPU 使用率", Required: true},
		{Key: "memoryUsage", Name: "主机内存使用率", Unit: "%", PromQL: nodeMemoryUsageQuery(nodeSelector), Status: statusPercent, Description: "资产 HostIP 对应 node_exporter 采集的主机内存使用率", Required: true},
		{Key: "diskUsage", Name: "主机磁盘使用率", Unit: "%", PromQL: nodeDiskUsageQuery(nodeSelector), Status: statusPercent, Description: "资产 HostIP 对应主机的数据盘或根分区最高使用率", Required: true},
		{Key: "diskReadIOPS", Name: "主机磁盘读 IOPS", Unit: "ops/s", PromQL: nodeDiskReadIOPSQuery(nodeSelector), Status: statusNormal, Description: "node_exporter 采集的主机磁盘读 IOPS", Required: true},
		{Key: "diskWriteIOPS", Name: "主机磁盘写 IOPS", Unit: "ops/s", PromQL: nodeDiskWriteIOPSQuery(nodeSelector), Status: statusNormal, Description: "node_exporter 采集的主机磁盘写 IOPS", Required: true},
		{Key: "threadsConnected", Name: "当前连接数", Unit: "个", PromQL: mysqlSumMetricQuery("mysql_global_status_threads_connected", selector), FallbackPromQL: mysqlSumMetricQuery("mysql_global_status_threads_connected", ""), Status: statusNormal, Description: "当前已连接线程数"},
		{Key: "threadsRunning", Name: "运行线程", Unit: "个", PromQL: mysqlSumMetricQuery("mysql_global_status_threads_running", selector), FallbackPromQL: mysqlSumMetricQuery("mysql_global_status_threads_running", ""), Status: statusNormal, Description: "当前正在执行的线程数"},
		{Key: "connectionsUsage", Name: "连接使用率", Unit: "%", PromQL: mysqlSumBinaryQuery("mysql_global_status_threads_connected", "mysql_global_variables_max_connections", selector, "* 100"), FallbackPromQL: mysqlSumBinaryQuery("mysql_global_status_threads_connected", "mysql_global_variables_max_connections", "", "* 100"), Status: statusPercent, Description: "threads_connected / max_connections"},
		{Key: "qps", Name: "QPS", Unit: "次/s", PromQL: mysqlSumRateQuery("mysql_global_status_questions", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_questions", ""), Status: statusNormal, Description: "Questions 每秒增长速率"},
		{Key: "tps", Name: "TPS", Unit: "次/s", PromQL: mysqlTPSQuery(selector), FallbackPromQL: mysqlTPSQuery(""), Status: statusNormal, Description: "commit + rollback 每秒增长速率"},
		{Key: "slowQueries", Name: "慢查询速率", Unit: "次/s", PromQL: mysqlSumRateQuery("mysql_global_status_slow_queries", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_slow_queries", ""), Status: statusWarnAboveZero, Description: "Slow_queries 每秒增长速率"},
		{Key: "selectScan", Name: "全表扫描速率", Unit: "次/s", PromQL: mysqlSumRateQuery("mysql_global_status_select_scan", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_select_scan", ""), Status: statusWarnAboveZero, Description: "Select_scan 每秒增长速率"},
		{Key: "abortedConnects", Name: "异常连接速率", Unit: "次/s", PromQL: mysqlSumRateQuery("mysql_global_status_aborted_connects", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_aborted_connects", ""), Status: statusWarnAboveZero, Description: "Aborted_connects 每秒增长速率"},
		{Key: "tmpDiskTables", Name: "磁盘临时表速率", Unit: "次/s", PromQL: mysqlSumRateQuery("mysql_global_status_created_tmp_disk_tables", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_created_tmp_disk_tables", ""), Status: statusWarnAboveZero, Description: "Created_tmp_disk_tables 每秒增长速率"},
		{Key: "bufferPoolHitRate", Name: "Buffer Pool 命中率", Unit: "%", PromQL: mysqlBufferPoolHitRateQuery(selector), FallbackPromQL: mysqlBufferPoolHitRateQuery(""), Status: statusHighPercent, Description: "1 - reads / read_requests"},
		{Key: "rowLockWaits", Name: "行锁等待速率", Unit: "次/s", PromQL: mysqlSumRateQuery("mysql_global_status_innodb_row_lock_waits", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_innodb_row_lock_waits", ""), Status: statusWarnAboveZero, Description: "Innodb_row_lock_waits 每秒增长速率"},
		{Key: "innodbReadIOPS", Name: "InnoDB 读 IOPS", Unit: "ops/s", PromQL: mysqlSumRateQuery("mysql_global_status_innodb_data_reads", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_innodb_data_reads", ""), Status: statusNormal, Description: "InnoDB data_reads 每秒增长速率"},
		{Key: "innodbWriteIOPS", Name: "InnoDB 写 IOPS", Unit: "ops/s", PromQL: mysqlSumRateQuery("mysql_global_status_innodb_data_writes", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_innodb_data_writes", ""), Status: statusNormal, Description: "InnoDB data_writes 每秒增长速率"},
		{Key: "innodbReadThroughput", Name: "InnoDB 读吞吐", Unit: "bytes/s", PromQL: mysqlSumRateQuery("mysql_global_status_innodb_data_read", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_innodb_data_read", ""), Status: statusNormal, Description: "InnoDB data_read 每秒增长字节数"},
		{Key: "innodbWriteThroughput", Name: "InnoDB 写吞吐", Unit: "bytes/s", PromQL: mysqlSumRateQuery("mysql_global_status_innodb_data_written", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_innodb_data_written", ""), Status: statusNormal, Description: "InnoDB data_written 每秒增长字节数"},
		{Key: "redoWritten", Name: "Redo 写入量", Unit: "bytes/s", PromQL: mysqlSumRateQuery("mysql_global_status_innodb_os_log_written", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_innodb_os_log_written", ""), Status: statusNormal, Description: "InnoDB redo log 每秒写入字节数"},
		{Key: "uptime", Name: "运行时长", Unit: "s", PromQL: mysqlMaxMetricQuery("mysql_global_status_uptime", selector), FallbackPromQL: mysqlMaxMetricQuery("mysql_global_status_uptime", ""), Status: statusNormal, Description: "MySQL uptime"},
	}

	cards := make([]dbmodel.MonitorMetricCard, 0, len(defs))
	warnings := make([]string, 0)
	for _, def := range defs {
		if isRequiredSystemMetric(def.Key) {
			def.Required = true
		}
		promql := def.PromQL
		value, err := client.queryInstantValue(ctx, promql)
		if err != nil && shouldFallbackPromQL(def.PromQL, def.FallbackPromQL) {
			fallbackValue, fallbackErr := client.queryInstantValue(ctx, def.FallbackPromQL)
			if fallbackErr == nil {
				promql = def.FallbackPromQL
				value = fallbackValue
				err = nil
			}
		}
		errorMessage := ""
		if err != nil {
			errorMessage = err.Error()
			if !def.optionalMissing() {
				warnings = append(warnings, def.Key+": "+errorMessage)
			}
			continue
		}
		status := "normal"
		if def.Status != nil {
			status = def.Status(value)
		}
		cards = append(cards, dbmodel.MonitorMetricCard{
			Key:         def.Key,
			Name:        def.Name,
			Value:       roundFloat(value, 2),
			Unit:        def.Unit,
			Status:      status,
			Description: def.Description,
			PromQL:      promql,
			Required:    def.Required,
			Error:       errorMessage,
		})
	}
	return cards, warnings
}

func (s *DatabaseService) queryMySQLTrends(ctx context.Context, client *prometheusClient, asset *cmdbmodel.CmdbSQL, selector string, start, end time.Time, step string) ([]dbmodel.MonitorTrendPanel, []string) {
	nodeSelector := nodeInstanceSelector(asset.HostIP)
	defs := []mysqlMonitorTrendDef{
		{
			Key:   "systemResource",
			Title: "主机 CPU / 内存",
			Unit:  "% / load",
			Series: []mysqlMonitorMetricDef{
				{Name: "主机 CPU 使用率", PromQL: nodeCPUUsageQuery(nodeSelector), Required: true},
				{Name: "Load 1m", PromQL: nodeLoad1Query(nodeSelector), Required: true},
				{Name: "主机内存使用率", PromQL: nodeMemoryUsageQuery(nodeSelector), Required: true},
				{Name: "Swap 使用率", PromQL: nodeSwapUsageQuery(nodeSelector), Required: true},
			},
		},
		{
			Key:   "diskIO",
			Title: "主机磁盘 IO",
			Unit:  "bytes/s / ops/s / ms",
			Series: []mysqlMonitorMetricDef{
				{Name: "读吞吐", PromQL: nodeDiskReadBytesQuery(nodeSelector), Required: true},
				{Name: "写吞吐", PromQL: nodeDiskWriteBytesQuery(nodeSelector), Required: true},
				{Name: "读 IOPS", PromQL: nodeDiskReadIOPSQuery(nodeSelector), Required: true},
				{Name: "写 IOPS", PromQL: nodeDiskWriteIOPSQuery(nodeSelector), Required: true},
				{Name: "读延迟", PromQL: nodeDiskReadLatencyQuery(nodeSelector), Required: true},
				{Name: "写延迟", PromQL: nodeDiskWriteLatencyQuery(nodeSelector), Required: true},
			},
		},
		{
			Key:   "osNetwork",
			Title: "主机网络",
			Unit:  "bytes/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "接收", PromQL: nodeNetworkReceiveQuery(nodeSelector), Required: true},
				{Name: "发送", PromQL: nodeNetworkTransmitQuery(nodeSelector), Required: true},
			},
		},
		{
			Key:   "throughput",
			Title: "吞吐量",
			Unit:  "次/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "QPS", PromQL: mysqlSumRateQuery("mysql_global_status_questions", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_questions", "")},
				{Name: "TPS", PromQL: mysqlTPSQuery(selector), FallbackPromQL: mysqlTPSQuery("")},
				{Name: "连接创建", PromQL: mysqlSumRateQuery("mysql_global_status_connections", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_connections", "")},
				{Name: "慢查询", PromQL: mysqlSumRateQuery("mysql_global_status_slow_queries", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_slow_queries", "")},
				{Name: "全表扫描", PromQL: mysqlSumRateQuery("mysql_global_status_select_scan", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_select_scan", "")},
				{Name: "异常连接", PromQL: mysqlSumRateQuery("mysql_global_status_aborted_connects", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_aborted_connects", "")},
				{Name: "磁盘临时表", PromQL: mysqlSumRateQuery("mysql_global_status_created_tmp_disk_tables", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_created_tmp_disk_tables", "")},
			},
		},
		{
			Key:   "connections",
			Title: "连接与线程",
			Unit:  "个",
			Series: []mysqlMonitorMetricDef{
				{Name: "当前连接", PromQL: mysqlSumMetricQuery("mysql_global_status_threads_connected", selector), FallbackPromQL: mysqlSumMetricQuery("mysql_global_status_threads_connected", "")},
				{Name: "运行线程", PromQL: mysqlSumMetricQuery("mysql_global_status_threads_running", selector), FallbackPromQL: mysqlSumMetricQuery("mysql_global_status_threads_running", "")},
				{Name: "缓存线程", PromQL: mysqlSumMetricQuery("mysql_global_status_threads_cached", selector), FallbackPromQL: mysqlSumMetricQuery("mysql_global_status_threads_cached", "")},
				{Name: "创建线程", PromQL: mysqlSumRateQuery("mysql_global_status_threads_created", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_threads_created", "")},
			},
		},
		{
			Key:   "files",
			Title: "文件句柄",
			Unit:  "个 / 次/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "文件打开速率", PromQL: mysqlSumRateQuery("mysql_global_status_opened_files", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_opened_files", "")},
				{Name: "当前打开文件", PromQL: mysqlSumMetricQuery("mysql_global_status_open_files", selector), FallbackPromQL: mysqlSumMetricQuery("mysql_global_status_open_files", "")},
				{Name: "打开文件上限", PromQL: mysqlSumMetricQuery("mysql_global_variables_open_files_limit", selector), FallbackPromQL: mysqlSumMetricQuery("mysql_global_variables_open_files_limit", "")},
			},
		},
		{
			Key:   "tables",
			Title: "表缓存",
			Unit:  "个 / 次/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "打开表", PromQL: mysqlSumMetricQuery("mysql_global_status_open_tables", selector), FallbackPromQL: mysqlSumMetricQuery("mysql_global_status_open_tables", "")},
				{Name: "打开表缓存", PromQL: mysqlSumMetricQuery("mysql_global_variables_table_open_cache", selector), FallbackPromQL: mysqlSumMetricQuery("mysql_global_variables_table_open_cache", "")},
				{Name: "表打开速率", PromQL: mysqlSumRateQuery("mysql_global_status_opened_tables", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_opened_tables", "")},
				{Name: "表定义打开速率", PromQL: mysqlSumRateQuery("mysql_global_status_opened_table_definitions", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_opened_table_definitions", "")},
			},
		},
		{
			Key:   "innodbHealth",
			Title: "InnoDB 健康度",
			Unit:  "% / 次/s / bytes/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "Buffer Pool 命中率", PromQL: mysqlBufferPoolHitRateQuery(selector), FallbackPromQL: mysqlBufferPoolHitRateQuery("")},
				{Name: "Buffer Pool 使用率", PromQL: mysqlSumBinaryQuery("mysql_global_status_innodb_buffer_pool_bytes_data", "mysql_global_variables_innodb_buffer_pool_size", selector, "* 100"), FallbackPromQL: mysqlSumBinaryQuery("mysql_global_status_innodb_buffer_pool_bytes_data", "mysql_global_variables_innodb_buffer_pool_size", "", "* 100")},
				{Name: "行锁等待", PromQL: mysqlSumRateQuery("mysql_global_status_innodb_row_lock_waits", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_innodb_row_lock_waits", "")},
				{Name: "Redo 写入", PromQL: mysqlSumRateQuery("mysql_global_status_innodb_os_log_written", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_innodb_os_log_written", "")},
			},
		},
		{
			Key:   "innodbIOPS",
			Title: "InnoDB IOPS",
			Unit:  "ops/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "读 IOPS", PromQL: mysqlSumRateQuery("mysql_global_status_innodb_data_reads", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_innodb_data_reads", "")},
				{Name: "写 IOPS", PromQL: mysqlSumRateQuery("mysql_global_status_innodb_data_writes", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_innodb_data_writes", "")},
				{Name: "fsync", PromQL: mysqlSumRateQuery("mysql_global_status_innodb_data_fsyncs", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_innodb_data_fsyncs", "")},
				{Name: "Redo fsync", PromQL: mysqlSumRateQuery("mysql_global_status_innodb_os_log_fsyncs", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_innodb_os_log_fsyncs", "")},
			},
		},
		{
			Key:   "innodbThroughput",
			Title: "InnoDB 吞吐",
			Unit:  "bytes/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "读吞吐", PromQL: mysqlSumRateQuery("mysql_global_status_innodb_data_read", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_innodb_data_read", "")},
				{Name: "写吞吐", PromQL: mysqlSumRateQuery("mysql_global_status_innodb_data_written", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_innodb_data_written", "")},
				{Name: "Redo 写入", PromQL: mysqlSumRateQuery("mysql_global_status_innodb_os_log_written", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_innodb_os_log_written", "")},
			},
		},
		{
			Key:   "innodbPendingIO",
			Title: "InnoDB Pending IO",
			Unit:  "个",
			Series: []mysqlMonitorMetricDef{
				{Name: "Pending Read", PromQL: mysqlSumMetricQuery("mysql_global_status_innodb_data_pending_reads", selector), FallbackPromQL: mysqlSumMetricQuery("mysql_global_status_innodb_data_pending_reads", "")},
				{Name: "Pending Write", PromQL: mysqlSumMetricQuery("mysql_global_status_innodb_data_pending_writes", selector), FallbackPromQL: mysqlSumMetricQuery("mysql_global_status_innodb_data_pending_writes", "")},
				{Name: "Pending fsync", PromQL: mysqlSumMetricQuery("mysql_global_status_innodb_data_pending_fsyncs", selector), FallbackPromQL: mysqlSumMetricQuery("mysql_global_status_innodb_data_pending_fsyncs", "")},
			},
		},
		{
			Key:   "handlers",
			Title: "Handlers",
			Unit:  "次/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "read_rnd_next", PromQL: mysqlHandlerSumRateQuery("read_rnd_next", selector), FallbackPromQL: mysqlHandlerSumRateQuery("read_rnd_next", "")},
				{Name: "read_next", PromQL: mysqlHandlerSumRateQuery("read_next", selector), FallbackPromQL: mysqlHandlerSumRateQuery("read_next", "")},
				{Name: "read_key", PromQL: mysqlHandlerSumRateQuery("read_key", selector), FallbackPromQL: mysqlHandlerSumRateQuery("read_key", "")},
				{Name: "write", PromQL: mysqlHandlerSumRateQuery("write", selector), FallbackPromQL: mysqlHandlerSumRateQuery("write", "")},
				{Name: "delete", PromQL: mysqlHandlerSumRateQuery("delete", selector), FallbackPromQL: mysqlHandlerSumRateQuery("delete", "")},
			},
		},
		{
			Key:   "commands",
			Title: "SQL 命令",
			Unit:  "次/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "Select", PromQL: mysqlCommandSumRateQuery("select", selector), FallbackPromQL: mysqlCommandSumRateQuery("select", "")},
				{Name: "Insert", PromQL: mysqlCommandSumRateQuery("insert", selector), FallbackPromQL: mysqlCommandSumRateQuery("insert", "")},
				{Name: "Update", PromQL: mysqlCommandSumRateQuery("update", selector), FallbackPromQL: mysqlCommandSumRateQuery("update", "")},
				{Name: "Delete", PromQL: mysqlCommandSumRateQuery("delete", selector), FallbackPromQL: mysqlCommandSumRateQuery("delete", "")},
			},
		},
		{
			Key:   "transactions",
			Title: "事务命令",
			Unit:  "次/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "commit", PromQL: mysqlCommandSumRateQuery("commit", selector), FallbackPromQL: mysqlCommandSumRateQuery("commit", "")},
				{Name: "rollback", PromQL: mysqlCommandSumRateQuery("rollback", selector), FallbackPromQL: mysqlCommandSumRateQuery("rollback", "")},
				{Name: "savepoint", PromQL: mysqlCommandSumRateQuery("savepoint", selector), FallbackPromQL: mysqlCommandSumRateQuery("savepoint", "")},
			},
		},
		{
			Key:   "sorts",
			Title: "排序",
			Unit:  "次/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "Sort Rows", PromQL: mysqlSumRateQuery("mysql_global_status_sort_rows", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_sort_rows", "")},
				{Name: "Sort Range", PromQL: mysqlSumRateQuery("mysql_global_status_sort_range", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_sort_range", "")},
				{Name: "Sort Scan", PromQL: mysqlSumRateQuery("mysql_global_status_sort_scan", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_sort_scan", "")},
				{Name: "Sort Merge Passes", PromQL: mysqlSumRateQuery("mysql_global_status_sort_merge_passes", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_sort_merge_passes", "")},
			},
		},
		{
			Key:   "locks",
			Title: "锁",
			Unit:  "次/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "Table Locks Immediate", PromQL: mysqlSumRateQuery("mysql_global_status_table_locks_immediate", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_table_locks_immediate", "")},
				{Name: "Table Locks Waited", PromQL: mysqlSumRateQuery("mysql_global_status_table_locks_waited", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_table_locks_waited", "")},
			},
		},
		{
			Key:   "aborted",
			Title: "Aborted",
			Unit:  "次/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "Aborted Clients", PromQL: mysqlSumRateQuery("mysql_global_status_aborted_clients", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_aborted_clients", "")},
				{Name: "Aborted Connects", PromQL: mysqlSumRateQuery("mysql_global_status_aborted_connects", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_aborted_connects", "")},
			},
		},
		{
			Key:   "queryCache",
			Title: "Query Cache",
			Unit:  "bytes / 次/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "Query Cache Size", PromQL: mysqlSumMetricQuery("mysql_global_variables_query_cache_size", selector), FallbackPromQL: mysqlSumMetricQuery("mysql_global_variables_query_cache_size", "")},
				{Name: "QCache Hits", PromQL: mysqlSumRateQuery("mysql_global_status_qcache_hits", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_qcache_hits", "")},
				{Name: "QCache Inserts", PromQL: mysqlSumRateQuery("mysql_global_status_qcache_inserts", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_qcache_inserts", "")},
				{Name: "QCache Not Cached", PromQL: mysqlSumRateQuery("mysql_global_status_qcache_not_cached", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_qcache_not_cached", "")},
			},
		},
		{
			Key:   "network",
			Title: "网络流量",
			Unit:  "bytes/s",
			Series: []mysqlMonitorMetricDef{
				{Name: "接收", PromQL: mysqlSumRateQuery("mysql_global_status_bytes_received", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_bytes_received", "")},
				{Name: "发送", PromQL: mysqlSumRateQuery("mysql_global_status_bytes_sent", selector), FallbackPromQL: mysqlSumRateQuery("mysql_global_status_bytes_sent", "")},
			},
		},
	}

	panels := make([]dbmodel.MonitorTrendPanel, 0, len(defs))
	warnings := make([]string, 0)
	for _, panelDef := range defs {
		panel := dbmodel.MonitorTrendPanel{Key: panelDef.Key, Title: panelDef.Title, Unit: panelDef.Unit}
		for _, seriesDef := range panelDef.Series {
			promql := seriesDef.PromQL
			points, err := client.queryRangePoints(ctx, promql, start, end, step)
			if (err != nil || len(points) == 0) && shouldFallbackPromQL(seriesDef.PromQL, seriesDef.FallbackPromQL) {
				fallbackPoints, fallbackErr := client.queryRangePoints(ctx, seriesDef.FallbackPromQL, start, end, step)
				if fallbackErr == nil && len(fallbackPoints) > 0 {
					promql = seriesDef.FallbackPromQL
					points = fallbackPoints
					err = nil
				}
			}
			errorMessage := ""
			if err != nil {
				errorMessage = err.Error()
				warnings = append(warnings, panelDef.Key+"/"+seriesDef.Name+": "+errorMessage)
			}
			if err != nil || len(points) == 0 {
				continue
			}
			panel.Series = append(panel.Series, dbmodel.MonitorMetricSeries{
				Name:     seriesDef.Name,
				PromQL:   promql,
				Points:   points,
				Required: seriesDef.Required,
				Error:    errorMessage,
			})
		}
		if len(panel.Series) > 0 {
			panels = append(panels, panel)
		}
	}
	return panels, warnings
}

func (s *DatabaseService) queryMySQLBreakdown(ctx context.Context, client *prometheusClient, selector string) ([]dbmodel.MonitorTablePanel, []string) {
	return []dbmodel.MonitorTablePanel{}, nil
}

func mysqlMetricQuery(metric, selector string) string {
	if strings.TrimSpace(selector) == "" {
		return metric
	}
	return metric + "{" + selector + "}"
}

func mysqlRateQuery(metric, selector string) string {
	return "rate(" + mysqlMetricQuery(metric, selector) + "[5m])"
}

func mysqlSumMetricQuery(metric, selector string) string {
	return "sum(" + mysqlMetricQuery(metric, selector) + ")"
}

func mysqlMaxMetricQuery(metric, selector string) string {
	return "max(" + mysqlMetricQuery(metric, selector) + ")"
}

func mysqlSumRateQuery(metric, selector string) string {
	return "sum(" + mysqlRateQuery(metric, selector) + ")"
}

func mysqlSumBinaryQuery(left, right, selector, suffix string) string {
	return "(sum(" + mysqlMetricQuery(left, selector) + ") / clamp_min(sum(" + mysqlMetricQuery(right, selector) + "), 1)) " + suffix
}

func shouldFallbackPromQL(primary, fallback string) bool {
	return strings.TrimSpace(fallback) != "" && strings.TrimSpace(primary) != strings.TrimSpace(fallback)
}

func mysqlCommandRateQuery(command, selector string) string {
	extra := `command="` + escapePrometheusLabelValue(command) + `"`
	if strings.TrimSpace(selector) != "" {
		extra = selector + "," + extra
	}
	return "rate(mysql_global_status_commands_total{" + extra + "}[5m])"
}

func mysqlCommandSumRateQuery(command, selector string) string {
	return "sum(" + mysqlCommandRateQuery(command, selector) + ")"
}

func mysqlHandlerRateQuery(handler, selector string) string {
	extra := `handler="` + escapePrometheusLabelValue(handler) + `"`
	if strings.TrimSpace(selector) != "" {
		extra = selector + "," + extra
	}
	return "rate(mysql_global_status_handlers_total{" + extra + "}[5m])"
}

func mysqlHandlerSumRateQuery(handler, selector string) string {
	return "sum(" + mysqlHandlerRateQuery(handler, selector) + ")"
}

func mysqlBinaryQuery(left, right, selector, suffix string) string {
	return "(" + mysqlMetricQuery(left, selector) + " / clamp_min(" + mysqlMetricQuery(right, selector) + ", 1)) " + suffix
}

func mysqlTPSQuery(selector string) string {
	return mysqlCommandSumRateQuery("commit", selector) + " + " + mysqlCommandSumRateQuery("rollback", selector)
}

func mysqlBufferPoolHitRateQuery(selector string) string {
	readMisses := mysqlSumRateQuery("mysql_global_status_innodb_buffer_pool_reads", selector)
	readRequests := mysqlSumRateQuery("mysql_global_status_innodb_buffer_pool_read_requests", selector)
	return "(1 - (" + readMisses + " / clamp_min(" + readRequests + ", 1))) * 100"
}

func mysqlDirtyPageRatioQuery(selector string) string {
	dirtyPages := mysqlMetricQuery("mysql_global_status_buffer_pool_dirty_pages", selector)
	totalPages := mysqlMetricQuery("mysql_global_status_buffer_pool_pages", appendPrometheusSelector(selector, `state=~"data|free|misc"`))
	return "(" + dirtyPages + " / clamp_min(sum(" + totalPages + "), 1)) * 100"
}

func appendPrometheusSelector(selector string, extra ...string) string {
	parts := make([]string, 0, len(extra)+1)
	if strings.TrimSpace(selector) != "" {
		parts = append(parts, strings.TrimSpace(selector))
	}
	for _, item := range extra {
		if strings.TrimSpace(item) != "" {
			parts = append(parts, strings.TrimSpace(item))
		}
	}
	return strings.Join(parts, ",")
}

func nodeInstanceSelector(hostIP string) string {
	hostIP = strings.TrimSpace(hostIP)
	if hostIP == "" {
		return ""
	}
	return fmt.Sprintf(`instance=~"%s(:.*)?"`, escapePrometheusLabelValue(regexp.QuoteMeta(hostIP)))
}

func nodeMetricQuery(metric, selector string, extra ...string) string {
	selector = appendPrometheusSelector(selector, extra...)
	if selector == "" {
		return metric
	}
	return metric + "{" + selector + "}"
}

func nodeCPUUsageQuery(selector string) string {
	return "100 * (1 - avg(rate(" + nodeMetricQuery("node_cpu_seconds_total", selector, `mode="idle"`) + "[5m])))"
}

func nodeLoad1Query(selector string) string {
	return nodeMetricQuery("node_load1", selector)
}

func nodeMemoryUsageQuery(selector string) string {
	available := nodeMetricQuery("node_memory_MemAvailable_bytes", selector)
	total := nodeMetricQuery("node_memory_MemTotal_bytes", selector)
	return "(1 - (" + available + " / clamp_min(" + total + ", 1))) * 100"
}

func nodeSwapUsageQuery(selector string) string {
	free := nodeMetricQuery("node_memory_SwapFree_bytes", selector)
	total := nodeMetricQuery("node_memory_SwapTotal_bytes", selector)
	return "(1 - (" + free + " / clamp_min(" + total + ", 1))) * 100"
}

func nodeDiskUsageQuery(selector string) string {
	fsFilter := `fstype!~"tmpfs|devtmpfs|overlay|squashfs|proc|sysfs|cgroup2?"`
	avail := nodeMetricQuery("node_filesystem_avail_bytes", selector, fsFilter)
	size := nodeMetricQuery("node_filesystem_size_bytes", selector, fsFilter)
	return "max((1 - (" + avail + " / clamp_min(" + size + ", 1))) * 100)"
}

func nodeDiskReadIOPSQuery(selector string) string {
	return "sum(rate(" + nodeMetricQuery("node_disk_reads_completed_total", selector, `device!~"loop.*|ram.*"`) + "[5m]))"
}

func nodeDiskWriteIOPSQuery(selector string) string {
	return "sum(rate(" + nodeMetricQuery("node_disk_writes_completed_total", selector, `device!~"loop.*|ram.*"`) + "[5m]))"
}

func nodeDiskReadBytesQuery(selector string) string {
	return "sum(rate(" + nodeMetricQuery("node_disk_read_bytes_total", selector, `device!~"loop.*|ram.*"`) + "[5m]))"
}

func nodeDiskWriteBytesQuery(selector string) string {
	return "sum(rate(" + nodeMetricQuery("node_disk_written_bytes_total", selector, `device!~"loop.*|ram.*"`) + "[5m]))"
}

func nodeDiskReadLatencyQuery(selector string) string {
	seconds := "rate(" + nodeMetricQuery("node_disk_read_time_seconds_total", selector, `device!~"loop.*|ram.*"`) + "[5m])"
	ops := "rate(" + nodeMetricQuery("node_disk_reads_completed_total", selector, `device!~"loop.*|ram.*"`) + "[5m])"
	return "sum(" + seconds + ") / clamp_min(sum(" + ops + "), 0.001) * 1000"
}

func nodeDiskWriteLatencyQuery(selector string) string {
	seconds := "rate(" + nodeMetricQuery("node_disk_write_time_seconds_total", selector, `device!~"loop.*|ram.*"`) + "[5m])"
	ops := "rate(" + nodeMetricQuery("node_disk_writes_completed_total", selector, `device!~"loop.*|ram.*"`) + "[5m])"
	return "sum(" + seconds + ") / clamp_min(sum(" + ops + "), 0.001) * 1000"
}

func nodeNetworkReceiveQuery(selector string) string {
	return "sum(rate(" + nodeMetricQuery("node_network_receive_bytes_total", selector, `device!~"lo|docker.*|veth.*|br.*"`) + "[5m]))"
}

func nodeNetworkTransmitQuery(selector string) string {
	return "sum(rate(" + nodeMetricQuery("node_network_transmit_bytes_total", selector, `device!~"lo|docker.*|veth.*|br.*"`) + "[5m]))"
}

func topKQuery(query string, count int) string {
	return fmt.Sprintf("topk(%d, %s)", count, query)
}

func statusUp(value float64) string {
	if value >= 1 {
		return "normal"
	}
	return "critical"
}

func statusPercent(value float64) string {
	switch {
	case value >= 90:
		return "critical"
	case value >= 80:
		return "warning"
	default:
		return "normal"
	}
}

func statusDiskLatency(value float64) string {
	switch {
	case value >= 20:
		return "critical"
	case value >= 10:
		return "warning"
	default:
		return "normal"
	}
}

func statusHighPercent(value float64) string {
	switch {
	case value < 95:
		return "critical"
	case value < 99:
		return "warning"
	default:
		return "normal"
	}
}

func statusReplicationLag(value float64) string {
	switch {
	case value >= 300:
		return "critical"
	case value >= 60:
		return "warning"
	default:
		return "normal"
	}
}

func statusWarnAboveZero(value float64) string {
	if value > 0 {
		return "warning"
	}
	return "normal"
}

func statusNormal(float64) string {
	return "normal"
}

func isRequiredSystemMetric(key string) bool {
	switch key {
	case "cpuUsage",
		"memoryUsage",
		"swapUsage",
		"diskUsage",
		"diskReadIOPS",
		"diskWriteIOPS",
		"diskReadLatency",
		"diskWriteLatency",
		"osNetworkReceive",
		"osNetworkTransmit":
		return true
	default:
		return false
	}
}

func (c *prometheusClient) targets(ctx context.Context) ([]prometheusTarget, error) {
	var data prometheusTargetsData
	if err := c.get(ctx, "/api/v1/targets", url.Values{"state": []string{"active"}}, &data); err != nil {
		return nil, err
	}
	return data.ActiveTargets, nil
}

func (c *prometheusClient) queryInstantValue(ctx context.Context, promql string) (float64, error) {
	var data prometheusQueryData
	if err := c.get(ctx, "/api/v1/query", url.Values{"query": []string{promql}}, &data); err != nil {
		log.Log().WithFields(map[string]interface{}{
			"module":     "mysql_monitor",
			"prometheus": c.baseURL,
			"promql":     promql,
			"error":      err.Error(),
		}).Warn("prometheus instant query failed")
		return 0, err
	}
	if len(data.Result) == 0 {
		log.Log().WithFields(map[string]interface{}{
			"module":     "mysql_monitor",
			"prometheus": c.baseURL,
			"promql":     promql,
		}).Warn("prometheus instant query empty result")
		return 0, fmt.Errorf("empty result")
	}
	return parsePrometheusValue(data.Result[0].Value)
}

func (c *prometheusClient) queryInstantRows(ctx context.Context, promql, nameLabel string) ([]map[string]interface{}, error) {
	var data prometheusQueryData
	if err := c.get(ctx, "/api/v1/query", url.Values{"query": []string{promql}}, &data); err != nil {
		return nil, err
	}
	rows := make([]map[string]interface{}, 0, len(data.Result))
	for _, item := range data.Result {
		value, err := parsePrometheusValue(item.Value)
		if err != nil {
			continue
		}
		name := item.Metric[nameLabel]
		if name == "" {
			name = item.Metric["instance"]
		}
		rows = append(rows, map[string]interface{}{
			"name":   name,
			"value":  roundFloat(value, 2),
			"labels": item.Metric,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left, _ := rows[i]["value"].(float64)
		right, _ := rows[j]["value"].(float64)
		return left > right
	})
	return rows, nil
}

func (c *prometheusClient) queryInstantMetricRows(ctx context.Context, promql string) ([]prometheusQueryResult, error) {
	var data prometheusQueryData
	if err := c.get(ctx, "/api/v1/query", url.Values{"query": []string{promql}}, &data); err != nil {
		return nil, err
	}
	if len(data.Result) == 0 {
		return nil, fmt.Errorf("empty result")
	}
	return data.Result, nil
}

func (c *prometheusClient) queryRangePoints(ctx context.Context, promql string, start, end time.Time, step string) ([]dbmodel.MonitorMetricPoint, error) {
	values := url.Values{}
	values.Set("query", promql)
	values.Set("start", strconv.FormatFloat(float64(start.UnixMilli())/1000, 'f', 3, 64))
	values.Set("end", strconv.FormatFloat(float64(end.UnixMilli())/1000, 'f', 3, 64))
	values.Set("step", step)

	var data prometheusQueryData
	if err := c.get(ctx, "/api/v1/query_range", values, &data); err != nil {
		return nil, err
	}
	if len(data.Result) == 0 {
		return []dbmodel.MonitorMetricPoint{}, nil
	}
	points := make([]dbmodel.MonitorMetricPoint, 0, len(data.Result[0].Values))
	for _, value := range data.Result[0].Values {
		ts, val, err := parsePrometheusSample(value)
		if err != nil {
			continue
		}
		points = append(points, dbmodel.MonitorMetricPoint{
			Timestamp: ts,
			Value:     roundFloat(val, 4),
		})
	}
	return points, nil
}

func (c *prometheusClient) get(ctx context.Context, path string, values url.Values, out interface{}) error {
	endpoint := strings.TrimRight(c.baseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.URL.RawQuery = values.Encode()
	req.Header.Set("Accept", "application/json")
	if c.username != "" || c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("prometheus returned HTTP %d", resp.StatusCode)
	}

	var apiResp prometheusAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return err
	}
	if apiResp.Status != "success" {
		if apiResp.Error != "" {
			return fmt.Errorf("%s", apiResp.Error)
		}
		return fmt.Errorf("prometheus query failed")
	}
	if err := json.Unmarshal(apiResp.Data, out); err != nil {
		return err
	}
	return nil
}

func parsePrometheusValue(value []interface{}) (float64, error) {
	_, val, err := parsePrometheusSample(value)
	return val, err
}

func parsePrometheusSample(value []interface{}) (int64, float64, error) {
	if len(value) < 2 {
		return 0, 0, fmt.Errorf("invalid sample")
	}
	ts, ok := value[0].(float64)
	if !ok {
		return 0, 0, fmt.Errorf("invalid timestamp")
	}
	raw, ok := value[1].(string)
	if !ok {
		return 0, 0, fmt.Errorf("invalid value")
	}
	val, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, 0, err
	}
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return 0, 0, fmt.Errorf("invalid finite value")
	}
	return int64(ts * 1000), val, nil
}

func roundFloat(value float64, places int) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	if places < 0 {
		return value
	}
	pow := math.Pow10(places)
	return math.Round(value*pow) / pow
}
