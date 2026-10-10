package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	aiopsmodel "dodevops-api/api/aiops/model"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/ssh"
)

const (
	defaultMySQLLogTailLines          = 1200
	maxMySQLLogTailLines              = 5000
	defaultMySQLCommandTimeoutSeconds = 12
	maxMySQLCommandTimeoutSeconds     = 60
	defaultMySQLWindowMinutes         = 30
)

var (
	mysqlUserHostRegex = regexp.MustCompile(`^# User@Host:\s+([^\[]+)\[[^\]]*\]\s+@\s+.*\[(.*?)\]`)
	mysqlSchemaRegex   = regexp.MustCompile(`^# Schema:\s*(\S+)`)
	mysqlQueryRegex    = regexp.MustCompile(`^# Query_time:\s*([0-9.]+)\s+Lock_time:\s*([0-9.]+)\s+Rows_sent:\s*(-?\d+)\s+Rows_examined:\s*(-?\d+)\s+Rows_affected:\s*(-?\d+)`)
	mysqlSpaceRegex    = regexp.MustCompile(`\s+`)
	mysqlNumberRegex   = regexp.MustCompile(`\b\d+\b`)
	mysqlStringRegex   = regexp.MustCompile(`'[^']*'|"[^"]*"`)
)

type MySQLAnalysisService struct{}

type mysqlAnalyzeContext struct {
	req       *aiopsmodel.MySQLAnalyzeRequest
	texts     []string
	collected []aiopsmodel.DatabaseCommandEvidence
}

type mysqlSlowEntry struct {
	User         string
	Host         string
	Schema       string
	QueryTime    float64
	LockTime     float64
	RowsSent     int64
	RowsExamined int64
	RowsAffected int64
	SQLLines     []string
}

type mysqlTextSignals struct {
	HasReadyForConnections bool
	HasCrashRecovery       bool
	HasUncleanShutdown     bool
	HasPMMTimeout          bool
	HasStopTimeout         bool
	HasExistingMysqld      bool
	HasRebootOrCrash       bool
	HasIOError             bool
	HasOOM                 bool
	HasTooManyConnections  bool
	HasAbortedConnections  bool
	HasNoSAR               bool
	HasPortMismatch        bool
	MySQLPort              string
}

func NewMySQLAnalysisService() *MySQLAnalysisService {
	return &MySQLAnalysisService{}
}

func (s *MySQLAnalysisService) AnalyzeMySQL(c *gin.Context, req *aiopsmodel.MySQLAnalyzeRequest) (*aiopsmodel.MySQLAnalyzeResult, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	normalizeMySQLAnalyzeRequest(req)
	if req.HostID == 0 {
		return nil, errors.New("hostId is required")
	}

	analysisCtx := &mysqlAnalyzeContext{req: req}
	collected, err := s.collectMySQLEvidence(c, req)
	if err != nil {
		return nil, err
	}
	analysisCtx.collected = collected
	for _, item := range collected {
		analysisCtx.texts = append(analysisCtx.texts, item.Output, item.Error)
	}

	result := s.buildRuleBasedMySQLResult(analysisCtx)
	if req.UseLLM {
		s.enrichMySQLResultWithLLM(c, req, result, analysisCtx.texts)
	}
	return result, nil
}

func normalizeMySQLAnalyzeRequest(req *aiopsmodel.MySQLAnalyzeRequest) {
	if req.WindowMinutes <= 0 {
		req.WindowMinutes = defaultMySQLWindowMinutes
	}
	if req.LogTailLines <= 0 {
		req.LogTailLines = defaultMySQLLogTailLines
	}
	if req.LogTailLines > maxMySQLLogTailLines {
		req.LogTailLines = maxMySQLLogTailLines
	}
	if req.CommandTimeoutSeconds <= 0 {
		req.CommandTimeoutSeconds = defaultMySQLCommandTimeoutSeconds
	}
	if req.CommandTimeoutSeconds > maxMySQLCommandTimeoutSeconds {
		req.CommandTimeoutSeconds = maxMySQLCommandTimeoutSeconds
	}
	if strings.TrimSpace(req.MySQLServiceName) == "" {
		req.MySQLServiceName = "mysql"
	}
	if len(req.SystemLogPaths) == 0 {
		req.SystemLogPaths = []string{"/var/log/messages", "/var/log/syslog", "/var/log/dmesg", "/var/log/dmesg.old"}
	}
}

func (s *MySQLAnalysisService) collectMySQLEvidence(c *gin.Context, req *aiopsmodel.MySQLAnalyzeRequest) ([]aiopsmodel.DatabaseCommandEvidence, error) {
	if c == nil {
		return nil, errors.New("gin context is required for host collection")
	}
	client, _, _, err := NewLogAnalysisService().openHostSSHClient(c, req.HostID, req.AuthID)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	timeout := time.Duration(req.CommandTimeoutSeconds) * time.Second
	commands := buildMySQLCollectionCommands(req)
	results := make([]aiopsmodel.DatabaseCommandEvidence, 0, len(commands))
	for _, command := range commands {
		results = append(results, runMySQLSSHCommand(client, command.Name, command.Command, timeout))
	}
	return results, nil
}

func buildMySQLCollectionCommands(req *aiopsmodel.MySQLAnalyzeRequest) []aiopsmodel.DatabaseCommandEvidence {
	tailLines := strconv.Itoa(req.LogTailLines)
	serviceName := shellQuote(req.MySQLServiceName)
	systemPaths := make([]string, 0, len(req.SystemLogPaths))
	for _, item := range req.SystemLogPaths {
		if strings.TrimSpace(item) != "" {
			systemPaths = append(systemPaths, shellQuote(item))
		}
	}
	if len(systemPaths) == 0 {
		systemPaths = []string{"'/var/log/messages'", "'/var/log/syslog'"}
	}

	errorLogCommand := fmt.Sprintf(`if [ -n %s ]; then files=%s; else files="/data/mysqllog/error.log /var/log/mysqld.log /var/log/mysql/error.log"; fi; for f in $files; do [ -r "$f" ] && echo "== $f ==" && tail -n %s "$f"; done`,
		shellQuote(strings.TrimSpace(req.ErrorLogPath)),
		shellQuote(strings.TrimSpace(req.ErrorLogPath)),
		tailLines,
	)
	slowLogCommand := fmt.Sprintf(`if [ -n %s ]; then files=%s; else files="/data/mysqllog/slow*.log /var/log/mysql/*slow*.log"; fi; for f in $files; do [ -r "$f" ] && echo "== $f ==" && tail -n %s "$f"; done`,
		shellQuote(strings.TrimSpace(req.SlowLogPath)),
		shellQuote(strings.TrimSpace(req.SlowLogPath)),
		tailLines,
	)

	mysqlStatusCommand := buildMySQLClientCommand(req, `SHOW GLOBAL STATUS WHERE Variable_name IN ('Threads_running','Threads_connected','Connections','Aborted_connects','Aborted_clients','Slow_queries','Questions','Innodb_data_pending_fsyncs','Innodb_data_pending_reads','Innodb_data_pending_writes','Innodb_os_log_pending_fsyncs','Innodb_row_lock_waits','Table_locks_waited','Opened_table_definitions','Open_table_definitions','Open_tables'); SHOW GLOBAL VARIABLES WHERE Variable_name IN ('port','socket','bind_address','max_connections','table_open_cache','table_definition_cache','innodb_flush_log_at_trx_commit','sync_binlog','slow_query_log','long_query_time','log_error','slow_query_log_file');`)

	return []aiopsmodel.DatabaseCommandEvidence{
		{Name: "mysql_service_status", Command: fmt.Sprintf("systemctl status %s --no-pager 2>&1 | head -160", serviceName)},
		{Name: "mysql_process", Command: "ps -efww | grep '[m]ysqld' | head -30"},
		{Name: "mysql_listen_ports", Command: "ss -lntp 2>/dev/null | grep -E 'mysqld|:3306|:33060|:33096' || true"},
		{Name: "recent_reboots", Command: "last -x 2>/dev/null | head -30 || true"},
		{Name: "system_mysql_kernel_errors", Command: fmt.Sprintf("grep -iE 'context deadline exceeded|mysqld|mysql|blocked for more|hung task|I/O error|Buffer I/O|blk_update|reset|timeout|megaraid|scsi|sd[a-z]|xfs|ext4|oom|out of memory' %s 2>/dev/null | tail -400 || true", strings.Join(systemPaths, " "))},
		{Name: "mysql_error_log_tail", Command: errorLogCommand},
		{Name: "mysql_slow_log_tail", Command: slowLogCommand},
		{Name: "mysql_runtime_status", Command: mysqlStatusCommand},
	}
}

func buildMySQLClientCommand(req *aiopsmodel.MySQLAnalyzeRequest, query string) string {
	args := []string{}
	if strings.TrimSpace(req.MySQLDefaultsFile) != "" {
		args = append(args, "--defaults-extra-file="+shellQuote(req.MySQLDefaultsFile))
	}
	if strings.TrimSpace(req.MySQLLoginPath) != "" {
		args = append(args, "--login-path="+shellQuote(req.MySQLLoginPath))
	}
	if strings.TrimSpace(req.MySQLSocket) != "" {
		args = append(args, "-S", shellQuote(req.MySQLSocket))
	}
	if req.MySQLPort > 0 {
		args = append(args, "-P", strconv.Itoa(req.MySQLPort))
	}
	args = append(args, "-NBe", shellQuote(query))
	return "mysql " + strings.Join(args, " ") + " 2>&1 || true"
}

func runMySQLSSHCommand(client *ssh.Client, name, command string, timeout time.Duration) aiopsmodel.DatabaseCommandEvidence {
	ev := aiopsmodel.DatabaseCommandEvidence{Name: name, Command: command}
	session, err := client.NewSession()
	if err != nil {
		ev.Error = err.Error()
		return ev
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	if err := session.Start("sh -lc " + shellQuote(command)); err != nil {
		ev.Error = err.Error()
		return ev
	}

	done := make(chan error, 1)
	go func() { done <- session.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			ev.Error = err.Error()
		}
	case <-time.After(timeout):
		_ = session.Signal(ssh.SIGKILL)
		_ = session.Close()
		ev.Error = "command timed out"
	}
	output := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
	ev.Output = truncateForPrompt(output, 60000)
	return ev
}

func (s *MySQLAnalysisService) buildRuleBasedMySQLResult(ctx *mysqlAnalyzeContext) *aiopsmodel.MySQLAnalyzeResult {
	allText := strings.Join(ctx.texts, "\n")
	slow := analyzeMySQLSlowLog(allText)
	signals := analyzeMySQLTextSignals(allText)
	hotspots := aiopsmodel.MySQLHotspots{
		TopHosts:       slow.TopHosts,
		TopUsers:       slow.TopUsers,
		TopSchemas:     slow.TopSchemas,
		TopSQLPatterns: slow.TopSQLPatterns,
	}

	evidence := buildMySQLEvidence(ctx, slow, signals)
	hypotheses := buildMySQLHypotheses(slow, signals)
	nextChecks := buildMySQLNextChecks(slow, signals, ctx.req)
	remediation := buildMySQLRemediation(slow, signals)
	missing := buildMySQLMissingData(ctx, slow)

	status, severity := inferMySQLStatusSeverity(slow, signals)
	directCause := inferMySQLDirectCause(slow, signals)
	conclusion := buildMySQLConclusion(slow, signals, directCause)
	summary := conclusion
	if directCause != "" {
		summary = directCause
	}

	return &aiopsmodel.MySQLAnalyzeResult{
		Status:            status,
		Severity:          severity,
		Summary:           summary,
		Conclusion:        conclusion,
		DirectCause:       directCause,
		ImpactedResources: inferMySQLImpactedResources(slow),
		Evidence:          evidence,
		Hypotheses:        hypotheses,
		Hotspots:          hotspots,
		SlowLogSummary:    slow,
		NextChecks:        nextChecks,
		Remediation:       remediation,
		SafeCommands:      buildMySQLSafeCommands(ctx.req),
		MissingData:       missing,
		CollectedCommands: filterRawCommands(ctx.collected, ctx.req.IncludeRawEvidence),
		Readonly:          true,
		GeneratedAt:       time.Now().Format("2006-01-02 15:04:05"),
	}
}

func analyzeMySQLSlowLog(text string) aiopsmodel.MySQLSlowLogSummary {
	entries := parseMySQLSlowEntries(text)
	summary := aiopsmodel.MySQLSlowLogSummary{}
	schemaCounts := map[string]int{}
	sqlCounts := map[string]int{}
	userCounts := map[string]int{}
	hostCounts := map[string]int{}

	for _, entry := range entries {
		summary.TotalEntries++
		if entry.QueryTime > summary.MaxQueryTimeSeconds {
			summary.MaxQueryTimeSeconds = entry.QueryTime
		}
		if entry.RowsExamined > summary.MaxRowsExamined {
			summary.MaxRowsExamined = entry.RowsExamined
		}
		if entry.RowsExamined >= 100000 {
			summary.LargeScanCount++
		}
		if entry.QueryTime >= 1 && entry.LockTime < 0.01 && entry.RowsExamined <= 10 {
			summary.LowRowsNoLockSlowCount++
		}
		pattern := normalizeMySQLSQLPattern(entry.SQLLines)
		if pattern != "" {
			sqlCounts[pattern]++
			if isMySQLConnectionProbe(pattern) && entry.QueryTime >= 1 {
				summary.SlowConnectionProbeCount++
			}
			if strings.EqualFold(pattern, "COMMIT") && entry.QueryTime >= 1 {
				summary.SlowCommitCount++
			}
		}
		if entry.Schema != "" {
			schemaCounts[entry.Schema]++
		}
		if entry.User != "" {
			userCounts[entry.User]++
		}
		if entry.Host != "" {
			hostCounts[entry.Host]++
		}
	}

	summary.TopSchemas = topCountItems(schemaCounts, 10)
	summary.TopSQLPatterns = topCountItems(sqlCounts, 12)
	summary.TopUsers = topCountItems(userCounts, 10)
	summary.TopHosts = topCountItems(hostCounts, 10)
	return summary
}

func parseMySQLSlowEntries(text string) []mysqlSlowEntry {
	lines := strings.Split(text, "\n")
	entries := make([]mysqlSlowEntry, 0)
	current := mysqlSlowEntry{}
	hasCurrent := false

	flush := func() {
		if !hasCurrent {
			return
		}
		if current.QueryTime > 0 || len(current.SQLLines) > 0 || current.Schema != "" || current.User != "" {
			entries = append(entries, current)
		}
		current = mysqlSlowEntry{}
		hasCurrent = false
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "# Time:") {
			flush()
			hasCurrent = true
			continue
		}
		if !hasCurrent && strings.HasPrefix(line, "# User@Host:") {
			hasCurrent = true
		}
		if !hasCurrent {
			continue
		}
		if matches := mysqlUserHostRegex.FindStringSubmatch(line); len(matches) == 3 {
			current.User = strings.TrimSpace(matches[1])
			current.Host = strings.TrimSpace(matches[2])
			continue
		}
		if matches := mysqlSchemaRegex.FindStringSubmatch(line); len(matches) == 2 {
			schema := strings.TrimSpace(matches[1])
			if schema != "" && schema != "Last_errno:" {
				current.Schema = schema
			}
			continue
		}
		if matches := mysqlQueryRegex.FindStringSubmatch(line); len(matches) == 6 {
			current.QueryTime = parseFloat(matches[1])
			current.LockTime = parseFloat(matches[2])
			current.RowsSent = parseInt64(matches[3])
			current.RowsExamined = parseInt64(matches[4])
			current.RowsAffected = parseInt64(matches[5])
			continue
		}
		if strings.HasPrefix(line, "#") || line == "" || strings.HasPrefix(strings.ToLower(line), "set timestamp") {
			continue
		}
		current.SQLLines = append(current.SQLLines, line)
	}
	flush()
	return entries
}

func normalizeMySQLSQLPattern(lines []string) string {
	statements := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(strings.ToLower(line), "use ") {
			continue
		}
		statements = append(statements, line)
	}
	if len(statements) == 0 {
		return ""
	}
	sql := strings.TrimSpace(strings.Join(statements, " "))
	sql = strings.TrimSuffix(sql, ";")
	lower := strings.ToLower(sql)
	switch {
	case lower == "commit":
		return "COMMIT"
	case strings.Contains(lower, "@@session.tx_read_only"):
		return "SELECT @@session.tx_read_only"
	case strings.Contains(lower, "@@session.transaction_read_only"):
		return "SELECT @@session.transaction_read_only"
	case strings.Contains(lower, "@@version"):
		return "SELECT @@version"
	case strings.HasPrefix(lower, "show variables"):
		return "SHOW VARIABLES"
	}

	normalized := mysqlStringRegex.ReplaceAllString(sql, "?")
	normalized = mysqlNumberRegex.ReplaceAllString(normalized, "?")
	normalized = mysqlSpaceRegex.ReplaceAllString(normalized, " ")
	normalized = strings.TrimSpace(normalized)
	extract := func(prefix, reText string) string {
		re := regexp.MustCompile(reText)
		if matches := re.FindStringSubmatch(normalized); len(matches) == 2 {
			return prefix + " " + strings.Trim(matches[1], "` ")
		}
		return ""
	}
	if value := extract("INSERT INTO", `(?i)^\s*insert\s+into\s+([`+"`"+`\w.]+)`); value != "" {
		return value
	}
	if value := extract("REPLACE INTO", `(?i)^\s*replace\s+into\s+([`+"`"+`\w.]+)`); value != "" {
		return value
	}
	if value := extract("UPDATE", `(?i)^\s*update\s+([`+"`"+`\w.]+)`); value != "" {
		return value
	}
	if value := extract("DELETE FROM", `(?i)^\s*delete\s+from\s+([`+"`"+`\w.]+)`); value != "" {
		return value
	}
	if matches := regexp.MustCompile(`(?i)\bfrom\s+([` + "`" + `\w.]+)`).FindStringSubmatch(normalized); len(matches) == 2 {
		return "SELECT FROM " + strings.Trim(matches[1], "` ")
	}
	if len(normalized) > 180 {
		normalized = normalized[:180]
	}
	return strings.ToUpper(strings.Fields(normalized)[0]) + " " + normalized
}

func isMySQLConnectionProbe(pattern string) bool {
	pattern = strings.ToUpper(pattern)
	return strings.Contains(pattern, "@@SESSION") || strings.Contains(pattern, "@@VERSION") || strings.HasPrefix(pattern, "SHOW VARIABLES")
}

func analyzeMySQLTextSignals(text string) mysqlTextSignals {
	lower := strings.ToLower(text)
	signals := mysqlTextSignals{
		HasReadyForConnections: strings.Contains(lower, "ready for connections"),
		HasCrashRecovery:       strings.Contains(lower, "crash recovery"),
		HasUncleanShutdown:     strings.Contains(lower, "not shutdown normally") || strings.Contains(lower, "database was not shutdown normally"),
		HasPMMTimeout:          strings.Contains(lower, "context deadline exceeded") || strings.Contains(lower, "error pinging mysqld"),
		HasStopTimeout:         strings.Contains(lower, "stopping timed out") || strings.Contains(lower, "stop-sigterm timed out"),
		HasExistingMysqld:      strings.Contains(lower, "a mysqld process already exists"),
		HasRebootOrCrash:       strings.Contains(lower, "system boot") || strings.Contains(lower, " - crash"),
		HasIOError:             strings.Contains(lower, "i/o error") || strings.Contains(lower, "buffer i/o") || strings.Contains(lower, "blk_update_request") || strings.Contains(lower, "hung task") || strings.Contains(lower, "blocked for more"),
		HasOOM:                 strings.Contains(lower, "out of memory") || strings.Contains(lower, "oom-killer") || strings.Contains(lower, "killed process"),
		HasTooManyConnections:  strings.Contains(lower, "too many connections"),
		HasAbortedConnections:  strings.Contains(lower, "aborted connection"),
		HasNoSAR:               strings.Contains(lower, "sar: command not found") || strings.Contains(lower, "cannot access /var/log/sa"),
	}
	portRegex := regexp.MustCompile(`(?i)port:\s*([0-9]{2,5})`)
	if matches := portRegex.FindStringSubmatch(text); len(matches) == 2 {
		signals.MySQLPort = matches[1]
		if matches[1] != "3306" {
			signals.HasPortMismatch = true
		}
	}
	return signals
}

func buildMySQLEvidence(ctx *mysqlAnalyzeContext, slow aiopsmodel.MySQLSlowLogSummary, signals mysqlTextSignals) []aiopsmodel.DatabaseDiagnosisEvidence {
	evidence := []aiopsmodel.DatabaseDiagnosisEvidence{}
	if slow.TotalEntries > 0 {
		evidence = append(evidence, aiopsmodel.DatabaseDiagnosisEvidence{
			Type:    "slow_log",
			Title:   "MySQL slow log summary",
			Content: fmt.Sprintf("发现 %d 条慢日志，最大 Query_time %.3fs，最大 Rows_examined %d；大扫描 %d 条，低扫描/低锁但变慢 %d 条，连接探测变慢 %d 条，慢 commit %d 条。", slow.TotalEntries, slow.MaxQueryTimeSeconds, slow.MaxRowsExamined, slow.LargeScanCount, slow.LowRowsNoLockSlowCount, slow.SlowConnectionProbeCount, slow.SlowCommitCount),
			Raw:     slow,
		})
	}
	if len(slow.TopHosts) > 0 || len(slow.TopUsers) > 0 || len(slow.TopSchemas) > 0 || len(slow.TopSQLPatterns) > 0 {
		evidence = append(evidence, aiopsmodel.DatabaseDiagnosisEvidence{
			Type:  "hotspot",
			Title: "Traffic and SQL hotspots",
			Content: fmt.Sprintf("热点来源=%s；热点用户=%s；热点库=%s；热点SQL=%s。",
				formatCountItems(slow.TopHosts, 5),
				formatCountItems(slow.TopUsers, 5),
				formatCountItems(slow.TopSchemas, 5),
				formatCountItems(slow.TopSQLPatterns, 5)),
		})
	}
	if signals.HasPMMTimeout || signals.HasStopTimeout || signals.HasExistingMysqld {
		evidence = append(evidence, aiopsmodel.DatabaseDiagnosisEvidence{
			Type:    "system_log",
			Title:   "MySQL process unresponsive signals",
			Content: "系统日志出现 MySQL ping 超时、停止超时或 mysqld 进程已存在等信号，支持 mysqld 当时不是正常拒绝连接，而是进程级无响应/卡顿。",
		})
	}
	if signals.HasCrashRecovery || signals.HasUncleanShutdown {
		evidence = append(evidence, aiopsmodel.DatabaseDiagnosisEvidence{
			Type:    "error_log",
			Title:   "Crash recovery after restart",
			Content: "MySQL 错误日志出现非正常关闭或 crash recovery，说明重启前实例没有干净退出；这通常是故障后的恢复证据，不等同于原始根因。",
		})
	}
	if signals.HasIOError {
		evidence = append(evidence, aiopsmodel.DatabaseDiagnosisEvidence{
			Type:    "kernel",
			Title:   "Kernel or storage error keywords",
			Content: "日志中出现 I/O、hung task 或块设备错误关键字，需要优先核查磁盘、RAID、文件系统和内核等待。",
		})
	}
	if signals.HasPortMismatch {
		evidence = append(evidence, aiopsmodel.DatabaseDiagnosisEvidence{
			Type:    "config",
			Title:   "MySQL listening port is not 3306",
			Content: "发现 MySQL 端口为 " + signals.MySQLPort + "，如果业务连接 3306，需要同时检查代理或端口映射。",
		})
	}
	return evidence
}

func buildMySQLHypotheses(slow aiopsmodel.MySQLSlowLogSummary, signals mysqlTextSignals) []aiopsmodel.DatabaseDiagnosisHypothesis {
	hypotheses := []aiopsmodel.DatabaseDiagnosisHypothesis{}
	if slow.SlowConnectionProbeCount > 0 && slow.LowRowsNoLockSlowCount > 0 {
		hypotheses = append(hypotheses, aiopsmodel.DatabaseDiagnosisHypothesis{
			Cause:      "MySQL global response stall / instance-wide latency",
			Confidence: "high",
			SupportingEvidence: []string{
				fmt.Sprintf("%d 条连接初始化或探测 SQL 进入慢日志", slow.SlowConnectionProbeCount),
				fmt.Sprintf("%d 条 Rows_examined 很低且 Lock_time 很低的 SQL 仍耗时超过 1 秒", slow.LowRowsNoLockSlowCount),
			},
		})
	}
	if slow.LargeScanCount > 0 {
		hypotheses = append(hypotheses, aiopsmodel.DatabaseDiagnosisHypothesis{
			Cause:      "inefficient SQL / missing indexes causing large scans",
			Confidence: confidenceByCount(slow.LargeScanCount, 8, 2),
			SupportingEvidence: []string{
				fmt.Sprintf("%d 条慢 SQL 扫描超过 100000 行，最大 Rows_examined=%d", slow.LargeScanCount, slow.MaxRowsExamined),
				"热点 SQL: " + formatCountItems(slow.TopSQLPatterns, 5),
			},
		})
	}
	if slow.SlowCommitCount > 0 {
		hypotheses = append(hypotheses, aiopsmodel.DatabaseDiagnosisHypothesis{
			Cause:              "commit/fsync/binlog flush latency or transaction backlog",
			Confidence:         confidenceByCount(slow.SlowCommitCount, 5, 1),
			SupportingEvidence: []string{fmt.Sprintf("%d 条 COMMIT 进入慢日志", slow.SlowCommitCount)},
		})
	}
	if signals.HasPMMTimeout || signals.HasStopTimeout {
		hypotheses = append(hypotheses, aiopsmodel.DatabaseDiagnosisHypothesis{
			Cause:              "mysqld became unresponsive before restart",
			Confidence:         "high",
			SupportingEvidence: []string{"监控或系统日志出现 MySQL ping 超时/停止超时"},
		})
	}
	if signals.HasIOError {
		hypotheses = append(hypotheses, aiopsmodel.DatabaseDiagnosisHypothesis{
			Cause:              "storage or kernel I/O issue",
			Confidence:         "medium",
			SupportingEvidence: []string{"日志中出现 I/O、块设备或 hung task 关键字"},
		})
	}
	if len(hypotheses) == 0 {
		hypotheses = append(hypotheses, aiopsmodel.DatabaseDiagnosisHypothesis{
			Cause:              "insufficient evidence",
			Confidence:         "low",
			SupportingEvidence: []string{"当前证据不足，需要补充慢日志、错误日志、系统日志和监控指标。"},
		})
	}
	return hypotheses
}

func buildMySQLNextChecks(slow aiopsmodel.MySQLSlowLogSummary, signals mysqlTextSignals, req *aiopsmodel.MySQLAnalyzeRequest) []string {
	checks := []string{
		"核对故障窗口内 Threads_running、Threads_connected、Connections、Slow_queries、QPS 是否突增。",
		"查看同一窗口 CPU load/iowait、磁盘 await/util、InnoDB pending fsync/write，判断是否存在资源瓶颈。",
	}
	if slow.LargeScanCount > 0 {
		checks = append(checks, "对 Top SQL 执行 EXPLAIN，并检查关联字段、过滤字段和排序字段索引。")
	}
	if len(slow.TopHosts) > 0 || len(slow.TopUsers) > 0 {
		checks = append(checks, "将 Top host/user 映射到具体应用服务，检查故障前后的接口并发、重试、定时任务和流量来源。")
	}
	if slow.SlowCommitCount > 0 {
		checks = append(checks, "检查 innodb_flush_log_at_trx_commit、sync_binlog、redo/binlog 所在磁盘 fsync 延迟。")
	}
	if signals.HasCrashRecovery || signals.HasUncleanShutdown {
		checks = append(checks, "把 crash recovery 时间点与人工重启、主机 reboot、systemd stop timeout 对齐，区分恢复证据和根因证据。")
	}
	if signals.HasPortMismatch {
		checks = append(checks, "确认业务实际连接端口和 MySQL 监听端口是否一致；如果走代理，检查代理端口状态。")
	}
	return checks
}

func buildMySQLRemediation(slow aiopsmodel.MySQLSlowLogSummary, signals mysqlTextSignals) []string {
	actions := []string{}
	if slow.LargeScanCount > 0 {
		actions = append(actions, "优先优化 Top 大扫描 SQL：拆分跨表 OR、改写为 UNION、补齐高选择性索引，并在低峰期评估在线 DDL。")
	}
	if len(slow.TopHosts) > 0 {
		actions = append(actions, "对热点应用增加限流、连接池上限、重试退避和批量任务错峰，避免瞬时连接/写入风暴。")
	}
	if slow.SlowCommitCount > 0 {
		actions = append(actions, "评估事务提交路径：减少单点写入风暴、合并小事务，检查 redo/binlog 磁盘延迟。")
	}
	if signals.HasIOError {
		actions = append(actions, "若存在内核 I/O 错误，先保护数据并检查 RAID/磁盘/文件系统，再考虑业务层优化。")
	}
	if len(actions) == 0 {
		actions = append(actions, "先补齐证据后再变更：慢日志样本、错误日志、系统日志、PMM/Grafana 指标和应用访问日志。")
	}
	return actions
}

func buildMySQLMissingData(ctx *mysqlAnalyzeContext, slow aiopsmodel.MySQLSlowLogSummary) []string {
	missing := []string{}
	allText := strings.ToLower(strings.Join(ctx.texts, "\n"))
	if slow.TotalEntries == 0 {
		missing = append(missing, "缺少 MySQL slow log 样本或慢日志统计。")
	}
	if !strings.Contains(allText, "ready for connections") && !strings.Contains(allText, "crash recovery") {
		missing = append(missing, "缺少 MySQL error log。")
	}
	if !strings.Contains(allText, "system boot") && !strings.Contains(allText, "context deadline exceeded") && !strings.Contains(allText, "mysqld") {
		missing = append(missing, "缺少故障窗口系统日志。")
	}
	if !strings.Contains(allText, "threads_running") && !strings.Contains(allText, "threads_connected") {
		missing = append(missing, "缺少 MySQL 运行状态或历史监控指标。")
	}
	return missing
}

func inferMySQLStatusSeverity(slow aiopsmodel.MySQLSlowLogSummary, signals mysqlTextSignals) (string, string) {
	if signals.HasIOError || signals.HasOOM || signals.HasPMMTimeout || signals.HasStopTimeout || (slow.SlowConnectionProbeCount > 0 && slow.LowRowsNoLockSlowCount > 10) {
		return "abnormal", "critical"
	}
	if slow.LargeScanCount > 0 || slow.TotalEntries > 0 || signals.HasCrashRecovery || signals.HasUncleanShutdown {
		return "abnormal", "warning"
	}
	if slow.TotalEntries == 0 {
		return "unknown", "info"
	}
	return "normal", "info"
}

func inferMySQLDirectCause(slow aiopsmodel.MySQLSlowLogSummary, signals mysqlTextSignals) string {
	if signals.HasPMMTimeout || signals.HasStopTimeout {
		return "MySQL 在故障窗口出现进程级无响应或全局卡顿"
	}
	if slow.SlowConnectionProbeCount > 0 && slow.LowRowsNoLockSlowCount > 0 {
		return "MySQL 出现实例级响应延迟，简单探测 SQL 也进入慢日志"
	}
	if slow.LargeScanCount > 0 {
		return "慢 SQL 大范围扫描造成数据库压力升高"
	}
	if signals.HasIOError {
		return "系统或存储层出现 I/O 异常信号"
	}
	return ""
}

func buildMySQLConclusion(slow aiopsmodel.MySQLSlowLogSummary, signals mysqlTextSignals, directCause string) string {
	if directCause == "" {
		return "当前证据不足以确认根因，需要补充慢日志、错误日志、系统日志和监控指标。"
	}
	parts := []string{directCause}
	if slow.LargeScanCount > 0 {
		parts = append(parts, fmt.Sprintf("同时发现 %d 条大扫描慢 SQL", slow.LargeScanCount))
	}
	if slow.SlowCommitCount > 0 {
		parts = append(parts, fmt.Sprintf("%d 条 COMMIT 变慢提示提交链路也受影响", slow.SlowCommitCount))
	}
	if signals.HasCrashRecovery || signals.HasUncleanShutdown {
		parts = append(parts, "重启后出现 crash recovery，说明此前未正常关闭")
	}
	return strings.Join(parts, "；") + "。"
}

func inferMySQLImpactedResources(slow aiopsmodel.MySQLSlowLogSummary) []string {
	set := map[string]struct{}{}
	for _, item := range slow.TopSchemas {
		set[item.Key] = struct{}{}
	}
	for _, item := range slow.TopHosts {
		set[item.Key] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for item := range set {
		if strings.TrimSpace(item) != "" {
			result = append(result, item)
		}
	}
	sort.Strings(result)
	return result
}

func buildMySQLSafeCommands(req *aiopsmodel.MySQLAnalyzeRequest) []aiopsmodel.DatabaseCommandEvidence {
	slowPath := "/path/to/slow.log"
	if req != nil && strings.TrimSpace(req.SlowLogPath) != "" {
		slowPath = strings.TrimSpace(req.SlowLogPath)
	}

	quotedSlowPath := shellQuote(slowPath)
	slowExpr := "tail -n 5000 " + quotedSlowPath
	sourceName := "统计近期慢日志来源 IP"
	userName := "统计近期慢日志用户来源"
	sqlName := "统计近期慢日志 SQL 模板"
	if req != nil && strings.TrimSpace(req.IncidentTime) != "" {
		incident := strings.TrimSpace(req.IncidentTime)
		slowExpr = fmt.Sprintf("awk -v t=%s '/^# Time:/{flag=index($0,t)>0} flag{print}' %s", shellQuote(incident), quotedSlowPath)
		sourceName = "统计故障时间来源 IP"
		userName = "统计故障时间用户来源"
		sqlName = "统计故障时间 SQL 模板"
	}

	return []aiopsmodel.DatabaseCommandEvidence{
		{Name: sourceName, Command: slowExpr + " | awk -F'[][]' '/User@Host/{print $4}' | sort | uniq -c | sort -nr | head -30"},
		{Name: userName, Command: slowExpr + " | awk -F'[][]' '/User@Host/{split($1,a,\": \"); print a[2]\" @\"$4}' | sort | uniq -c | sort -nr | head -30"},
		{Name: sqlName, Command: slowExpr + " | grep -E \"INSERT INTO|UPDATE |REPLACE INTO|commit;|SHOW VARIABLES|SELECT @@|select @@\" | sed -E \"s/[[:space:]]+/ /g\" | cut -c1-220 | sort | uniq -c | sort -nr | head -50"},
		{Name: "检查 MySQL 运行状态", Command: "mysql -NBe \"SHOW GLOBAL STATUS WHERE Variable_name IN ('Threads_running','Threads_connected','Connections','Slow_queries','Innodb_data_pending_fsyncs','Innodb_os_log_pending_fsyncs');\""},
		{Name: "检查内核 I/O 错误", Command: "grep -iE \"blocked for more|hung task|I/O error|Buffer I/O|blk_update|reset|timeout|megaraid|scsi|sd[a-z]|xfs|ext4|oom\" /var/log/messages /var/log/dmesg* 2>/dev/null | tail -300"},
	}
}

func (s *MySQLAnalysisService) enrichMySQLResultWithLLM(c *gin.Context, req *aiopsmodel.MySQLAnalyzeRequest, result *aiopsmodel.MySQLAnalyzeResult, texts []string) {
	logSvc := NewLogAnalysisService()
	provider, apiKey, err := logSvc.resolveLLMProvider(req.ProviderID)
	if err != nil {
		result.MissingData = append(result.MissingData, "LLM 分析未执行: "+err.Error())
		return
	}
	timeout := time.Duration(provider.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	baseCtx := context.Background()
	if c != nil && c.Request != nil {
		baseCtx = c.Request.Context()
	}
	llmCtx, cancel := context.WithTimeout(baseCtx, timeout)
	defer cancel()

	answer, err := callLLMForLogAnalysis(llmCtx, provider, apiKey, buildMySQLAnalysisPrompt(req, result, texts))
	if err != nil {
		result.MissingData = append(result.MissingData, "LLM 分析失败: "+err.Error())
		return
	}
	result.LLMAnalysis = answer
	result.UsedLLM = true
	result.ProviderID = provider.ID
	result.ProviderName = provider.Name
	result.ModelCode = provider.DefaultModel
}

func buildMySQLAnalysisPrompt(req *aiopsmodel.MySQLAnalyzeRequest, result *aiopsmodel.MySQLAnalyzeResult, texts []string) string {
	summaryJSON, _ := json.Marshal(result)
	var builder strings.Builder
	builder.WriteString("You are an experienced MySQL DBA/SRE. Analyze the incident evidence and answer in Chinese.\n")
	builder.WriteString("Use evidence-chain reasoning. Distinguish direct symptom, likely root cause, contributing factors, and missing data. Do not invent facts.\n")
	builder.WriteString("Focus on MySQL unavailable/stall, slow query patterns, connection storms, lock waits, disk/fsync, kernel errors, crash recovery, and application hot spots.\n")
	if req != nil && strings.TrimSpace(req.Prompt) != "" {
		builder.WriteString("\nOperator question:\n")
		builder.WriteString(strings.TrimSpace(req.Prompt))
		builder.WriteString("\n")
	}
	builder.WriteString("\nRule-based preliminary result JSON:\n")
	builder.Write(summaryJSON)
	builder.WriteString("\n\nEvidence snippets:\n")
	for _, text := range limitMySQLPromptTexts(texts, 22000) {
		builder.WriteString(redactSensitiveText(text))
		builder.WriteString("\n---\n")
	}
	return builder.String()
}

func limitMySQLPromptTexts(texts []string, maxChars int) []string {
	result := []string{}
	total := 0
	for _, text := range texts {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		text = truncateForPrompt(text, 4000)
		if total+len(text) > maxChars {
			break
		}
		result = append(result, text)
		total += len(text)
	}
	return result
}

func filterRawCommands(commands []aiopsmodel.DatabaseCommandEvidence, includeRaw bool) []aiopsmodel.DatabaseCommandEvidence {
	if includeRaw {
		return commands
	}
	result := make([]aiopsmodel.DatabaseCommandEvidence, 0, len(commands))
	for _, item := range commands {
		item.Output = truncateForPrompt(item.Output, 1000)
		result = append(result, item)
	}
	return result
}

func topCountItems(counts map[string]int, limit int) []aiopsmodel.CountItem {
	items := make([]aiopsmodel.CountItem, 0, len(counts))
	for key, count := range counts {
		key = strings.TrimSpace(key)
		if key != "" && count > 0 {
			items = append(items, aiopsmodel.CountItem{Key: key, Count: count})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Key < items[j].Key
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items
}

func formatCountItems(items []aiopsmodel.CountItem, limit int) string {
	if len(items) == 0 {
		return "无"
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, fmt.Sprintf("%s(%d)", item.Key, item.Count))
	}
	return strings.Join(parts, ", ")
}

func confidenceByCount(count, highAt, mediumAt int) string {
	if count >= highAt {
		return "high"
	}
	if count >= mediumAt {
		return "medium"
	}
	return "low"
}

func parseFloat(value string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return v
}

func parseInt64(value string) int64 {
	v, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return v
}
