package service

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	aiopsservice "dodevops-api/api/aiops/service"
	cmdbdao "dodevops-api/api/cmdb/dao"
	cmdbmodel "dodevops-api/api/cmdb/model"
	dbmodel "dodevops-api/api/db/model"
	"dodevops-api/common/util"
	"dodevops-api/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	defaultSlowLogTailLines  = 10000
	maxSlowLogTailLines      = 50000
	maxSlowLogSQLLength      = 4000
	slowLogTaskTopLimit      = 100
	slowLogAISuggestionLimit = 10
	maxSlowLogTaskReadBytes  = 100 * 1024 * 1024
	maxSlowLogAIReadBytes    = 50 * 1024 * 1024
	maxSlowLogUploadBytes    = 150 * 1024 * 1024
)

const slowLogAISuggestionSystemPrompt = "You are a senior MySQL performance tuning DBA. Answer in Chinese. Return only valid JSON."

var (
	slowLogTimeRegex         = regexp.MustCompile(`^# Time:\s*(.+)$`)
	slowLogUserHostRegex     = regexp.MustCompile(`^# User@Host:\s+([^\[]+)\[[^\]]*\]\s+@\s+.*\[(.*?)\]`)
	slowLogQueryRegex        = regexp.MustCompile(`^# Query_time:\s*([0-9.]+)\s+Lock_time:\s*([0-9.]+)\s+Rows_sent:\s*(-?\d+)\s+Rows_examined:\s*(-?\d+)(?:\s+Rows_affected:\s*(-?\d+))?`)
	slowLogSchemaRegex       = regexp.MustCompile(`^# Schema:\s*(\S+)`)
	slowLogUseRegex          = regexp.MustCompile("(?i)^use\\s+`?([^`;\\s]+)`?\\s*;?$")
	slowLogSetTimestampRegex = regexp.MustCompile(`(?i)^SET\s+timestamp\s*=\s*(\d+)\s*;?$`)
	slowLogStringRegex       = regexp.MustCompile(`'([^'\\]|\\.)*'|"([^"\\]|\\.)*"`)
	slowLogNumberRegex       = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	slowLogInListRegex       = regexp.MustCompile(`(?i)\bin\s*\((?:\s*\?\s*,?)+\)`)
	slowLogWhitespaceRegex   = regexp.MustCompile(`\s+`)
)

type mysqlSlowLogEntry struct {
	Time         time.Time
	User         string
	Host         string
	Schema       string
	QueryTime    float64
	LockTime     float64
	RowsSent     int64
	RowsExamined int64
	RowsAffected int64
	SQL          string
	Fingerprint  string
}

type slowLogAggregate struct {
	item       dbmodel.SlowLogItem
	queryTimes []float64
}

type slowLogFilter struct {
	begin    *time.Time
	end      *time.Time
	keyword  string
	database string
	user     string
}

type slowLogAccumulator struct {
	asset      *cmdbmodel.CmdbSQL
	filter     slowLogFilter
	summary    dbmodel.SlowLogSummary
	aggregates map[string]*slowLogAggregate
}

type mysqlSlowLogStreamParser struct {
	current    mysqlSlowLogEntry
	hasCurrent bool
	lineBuffer []byte
	emit       func(mysqlSlowLogEntry)
}

type slowLogTaskRequest struct {
	dbmodel.SlowLogQuery
	UploadedFilePath string `json:"uploadedFilePath,omitempty"`
	UploadedFileName string `json:"uploadedFileName,omitempty"`
}

func (s *DatabaseService) StartSlowLogTask(ctx *gin.Context, assetID uint, query dbmodel.SlowLogQuery) (*dbmodel.SlowLogTaskCreateResult, error) {
	query.Page = 1
	query.PageSize = slowLogTaskTopLimit
	if strings.TrimSpace(query.SortBy) == "" {
		query.SortBy = "queryTimeTotal"
	}
	if strings.TrimSpace(query.SortOrder) == "" {
		query.SortOrder = "desc"
	}

	asset, err := s.assetDao.GetByID(assetID)
	if err != nil {
		return nil, err
	}
	if asset.Type != cmdbmodel.DatabaseTypeMySQL {
		return nil, fmt.Errorf("slow log analysis currently supports MySQL only")
	}
	if strings.TrimSpace(asset.SlowLogPath) == "" {
		return nil, fmt.Errorf("slowLogPath is not configured for this database asset")
	}
	hostID, fallbackHost, err := resolveSlowLogHost(asset)
	if err != nil {
		return nil, err
	}

	var createdBy uint
	var createdByName string
	if admin, err := jwt.GetAdmin(ctx); err == nil && admin != nil {
		createdBy = admin.ID
		createdByName = admin.Username
	}
	requestJSON, _ := json.Marshal(query)
	now := time.Now()
	task := &dbmodel.SlowLogAnalyzeTask{
		TaskID:        uuid.NewString(),
		AssetID:       asset.ID,
		Engine:        asset.Type,
		Status:        dbmodel.SlowLogTaskStatusPending,
		Progress:      0,
		Message:       "任务已提交，等待后台分析",
		AuthID:        query.AuthID,
		RequestJSON:   string(requestJSON),
		SlowLogPath:   asset.SlowLogPath,
		HostID:        hostID,
		HostIP:        asset.HostIP,
		CreatedBy:     createdBy,
		CreatedByName: createdByName,
		CreatedAt:     util.HTime{Time: now},
		UpdatedAt:     util.HTime{Time: now},
	}
	if fallbackHost != nil {
		task.HostName = firstNonEmpty(fallbackHost.HostName, fallbackHost.Name)
		task.HostIP = firstNonEmpty(fallbackHost.PrivateIP, fallbackHost.SSHIP, fallbackHost.PublicIP, asset.HostIP)
	}
	if err := s.slowLogTaskDao.Create(task); err != nil {
		return nil, err
	}

	bgCtx := ctx.Copy()
	go s.runSlowLogTask(bgCtx, task.TaskID)

	return &dbmodel.SlowLogTaskCreateResult{
		TaskID:    task.TaskID,
		Status:    task.Status,
		ResultURL: fmt.Sprintf("/api/v1/db/assets/%d/slow-log/tasks/%s", asset.ID, task.TaskID),
	}, nil
}

func (s *DatabaseService) StartUploadedSlowLogTask(ctx *gin.Context, assetID uint, fileHeader *multipart.FileHeader, query dbmodel.SlowLogQuery) (*dbmodel.SlowLogTaskCreateResult, error) {
	if fileHeader == nil {
		return nil, fmt.Errorf("file is required")
	}
	if fileHeader.Size <= 0 {
		return nil, fmt.Errorf("uploaded file is empty")
	}
	if fileHeader.Size > maxSlowLogUploadBytes {
		return nil, fmt.Errorf("uploaded slow log file must be smaller than 150MB")
	}
	query.Page = 1
	query.PageSize = slowLogTaskTopLimit
	if strings.TrimSpace(query.SortBy) == "" {
		query.SortBy = "queryTimeTotal"
	}
	if strings.TrimSpace(query.SortOrder) == "" {
		query.SortOrder = "desc"
	}

	asset, err := s.assetDao.GetByID(assetID)
	if err != nil {
		return nil, err
	}
	if asset.Type != cmdbmodel.DatabaseTypeMySQL {
		return nil, fmt.Errorf("slow log analysis currently supports MySQL only")
	}

	taskID := uuid.NewString()
	tempDir := filepath.Join(os.TempDir(), "db_slow_log_uploads")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("create slow log upload temp dir failed: %w", err)
	}
	fileName := sanitizeSlowLogUploadFileName(fileHeader.Filename)
	tempPath := filepath.Join(tempDir, taskID+"_"+fileName)
	if err := saveUploadedSlowLogFile(fileHeader, tempPath, maxSlowLogUploadBytes); err != nil {
		return nil, err
	}

	var createdBy uint
	var createdByName string
	if admin, err := jwt.GetAdmin(ctx); err == nil && admin != nil {
		createdBy = admin.ID
		createdByName = admin.Username
	}
	req := slowLogTaskRequest{
		SlowLogQuery:     query,
		UploadedFilePath: tempPath,
		UploadedFileName: fileName,
	}
	requestJSON, _ := json.Marshal(req)
	now := time.Now()
	task := &dbmodel.SlowLogAnalyzeTask{
		TaskID:        taskID,
		AssetID:       asset.ID,
		Engine:        asset.Type,
		Status:        dbmodel.SlowLogTaskStatusPending,
		Progress:      0,
		Message:       "文件已上传，等待后台分析",
		RequestJSON:   string(requestJSON),
		SlowLogPath:   fileName,
		TotalBytes:    fileHeader.Size,
		CreatedBy:     createdBy,
		CreatedByName: createdByName,
		CreatedAt:     util.HTime{Time: now},
		UpdatedAt:     util.HTime{Time: now},
	}
	if err := s.slowLogTaskDao.Create(task); err != nil {
		_ = os.Remove(tempPath)
		return nil, err
	}

	bgCtx := ctx.Copy()
	go s.runSlowLogTask(bgCtx, task.TaskID)

	return &dbmodel.SlowLogTaskCreateResult{
		TaskID:    task.TaskID,
		Status:    task.Status,
		ResultURL: fmt.Sprintf("/api/v1/db/assets/%d/slow-log/tasks/%s", asset.ID, task.TaskID),
	}, nil
}

func (s *DatabaseService) GetSlowLogTask(taskID string) (*dbmodel.SlowLogTaskStatus, error) {
	task, err := s.slowLogTaskDao.GetByTaskID(taskID)
	if err != nil {
		return nil, err
	}
	return buildSlowLogTaskStatus(task, true), nil
}

func (s *DatabaseService) ListSlowLogTasks(assetID uint, query dbmodel.SlowLogTaskQuery) ([]*dbmodel.SlowLogTaskStatus, int64, error) {
	query.Page, query.PageSize = dbmodel.NormalizePage(query.Page, query.PageSize)
	tasks, total, err := s.slowLogTaskDao.List(assetID, query)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*dbmodel.SlowLogTaskStatus, 0, len(tasks))
	for i := range tasks {
		list = append(list, buildSlowLogTaskStatus(&tasks[i], false))
	}
	return list, total, nil
}

func (s *DatabaseService) DeleteSlowLogTask(assetID uint, taskID string) error {
	task, err := s.slowLogTaskDao.GetByTaskID(taskID)
	if err != nil {
		return err
	}
	if task.AssetID != assetID {
		return fmt.Errorf("slow log task does not belong to current asset")
	}
	if task.Status == dbmodel.SlowLogTaskStatusPending || task.Status == dbmodel.SlowLogTaskStatusRunning {
		return fmt.Errorf("running slow log task cannot be deleted")
	}
	return s.slowLogTaskDao.DeleteByTaskID(taskID)
}

func (s *DatabaseService) ClearSlowLogTasks(assetID uint) (int64, error) {
	return s.slowLogTaskDao.ClearFinished(assetID)
}

func (s *DatabaseService) GetSlowLogTaskResult(taskID, sortBy, sortOrder string) (*dbmodel.SlowLogTaskStatus, error) {
	task, err := s.slowLogTaskDao.GetByTaskID(taskID)
	if err != nil {
		return nil, err
	}
	status := buildSlowLogTaskStatus(task, true)
	if status == nil || status.Result == nil || len(status.Result.Items) == 0 {
		return status, nil
	}
	sortBy = normalizeSlowLogResultSortBy(sortBy)
	sortOrder = normalizeSlowLogResultSortOrder(sortOrder)
	if sortBy != "" {
		sortSlowLogItems(status.Result.Items, sortBy, sortOrder)
	}
	return status, nil
}

func (s *DatabaseService) ExportSlowLogTaskCSV(taskID string, writer io.Writer) (int64, error) {
	task, err := s.slowLogTaskDao.GetByTaskID(taskID)
	if err != nil {
		return 0, err
	}
	if task.Status != dbmodel.SlowLogTaskStatusSuccess {
		return 0, fmt.Errorf("slow log task is not success")
	}
	var result dbmodel.SlowLogAnalyzeResult
	if err := json.Unmarshal([]byte(task.ResultJSON), &result); err != nil {
		return 0, fmt.Errorf("decode slow log task result failed: %w", err)
	}
	return writeSlowLogItemsCSV(result.Items, writer)
}

func (s *DatabaseService) runSlowLogTask(ctx *gin.Context, taskID string) {
	task, err := s.slowLogTaskDao.GetByTaskID(taskID)
	if err != nil {
		return
	}
	var taskReq slowLogTaskRequest
	_ = json.Unmarshal([]byte(task.RequestJSON), &taskReq)
	runningMessage := "正在连接主机并读取慢日志"
	if strings.TrimSpace(taskReq.UploadedFilePath) != "" {
		runningMessage = "正在读取上传的慢日志文件"
	}
	startedAt := time.Now()
	_ = s.slowLogTaskDao.UpdateFields(taskID, map[string]interface{}{
		"status":     dbmodel.SlowLogTaskStatusRunning,
		"progress":   1,
		"message":    runningMessage,
		"started_at": &startedAt,
	})

	asset, err := s.assetDao.GetByID(task.AssetID)
	if err != nil {
		s.failSlowLogTask(taskID, err)
		return
	}
	query := taskReq.SlowLogQuery
	query.Page = 1
	query.PageSize = slowLogTaskTopLimit
	if strings.TrimSpace(query.SortBy) == "" {
		query.SortBy = "queryTimeTotal"
	}
	if strings.TrimSpace(query.SortOrder) == "" {
		query.SortOrder = "desc"
	}
	filter, err := buildSlowLogFilter(query)
	if err != nil {
		s.failSlowLogTask(taskID, err)
		return
	}
	acc := newSlowLogAccumulator(asset, filter)
	parser := newMySQLSlowLogStreamParser(func(entry mysqlSlowLogEntry) {
		acc.add(entry)
	})

	var hostName string
	var hostIP string
	var remoteTotalBytes int64
	lastUpdate := time.Now()
	progressCallback := func(readBytes, totalBytes int64) {
		if time.Since(lastUpdate) < time.Second {
			return
		}
		progress := 5
		progressTotal := totalBytes
		if strings.TrimSpace(taskReq.UploadedFilePath) == "" && (progressTotal <= 0 || progressTotal > maxSlowLogTaskReadBytes) {
			progressTotal = maxSlowLogTaskReadBytes
		}
		if progressTotal > 0 {
			progress = 5 + int(float64(readBytes)/float64(progressTotal)*85)
			if progress > 90 {
				progress = 90
			}
		}
		_ = s.slowLogTaskDao.UpdateFields(taskID, map[string]interface{}{
			"progress":        progress,
			"message":         fmt.Sprintf("正在解析慢日志: %d/%d bytes", readBytes, totalBytes),
			"read_bytes":      readBytes,
			"total_bytes":     totalBytes,
			"total_entries":   acc.summary.TotalEntries,
			"matched_entries": acc.summary.MatchedEntries,
			"unique_sql":      len(acc.aggregates),
		})
		lastUpdate = time.Now()
	}
	if strings.TrimSpace(taskReq.UploadedFilePath) != "" {
		remoteTotalBytes, err = readLocalSlowLogFileChunks(taskReq.UploadedFilePath, 256*1024, func(chunk []byte, readBytes, totalBytes int64) error {
			parser.Write(chunk)
			progressCallback(readBytes, totalBytes)
			return nil
		})
	} else {
		host, readErr := aiopsservice.NewLogAnalysisService().ReadHostFileChunks(ctx, task.HostID, task.AuthID, task.SlowLogPath, 256*1024, maxSlowLogTaskReadBytes, func(chunk []byte, readBytes, totalBytes int64) error {
			remoteTotalBytes = totalBytes
			parser.Write(chunk)
			progressCallback(readBytes, totalBytes)
			return nil
		})
		if readErr != nil {
			err = readErr
		}
		if host != nil {
			hostName = firstNonEmpty(host.HostName, host.Name)
			hostIP = firstNonEmpty(host.PrivateIP, host.SSHIP, host.PublicIP, asset.HostIP)
		}
	}
	if err != nil {
		s.failSlowLogTask(taskID, err)
		if strings.TrimSpace(taskReq.UploadedFilePath) != "" {
			_ = os.Remove(taskReq.UploadedFilePath)
		}
		return
	}
	parser.Finish()

	result := acc.result(query, slowLogTaskTopLimit)
	warnings := make([]string, 0)
	if strings.TrimSpace(taskReq.UploadedFilePath) == "" && remoteTotalBytes > maxSlowLogTaskReadBytes {
		warnings = append(warnings, fmt.Sprintf("日志文件超过 100MB，本次仅分析前 %d MB 内容", maxSlowLogTaskReadBytes/1024/1024))
	}
	if len(result.Items) > 0 {
		_ = s.slowLogTaskDao.UpdateFields(taskID, map[string]interface{}{
			"progress": 92,
			"message":  fmt.Sprintf("正在生成 Top %d 慢 SQL AI 优化建议", minInt(len(result.Items), slowLogAISuggestionLimit)),
		})
		if aiWarnings := s.enrichSlowLogAISuggestions(ctx, asset, &result); len(aiWarnings) > 0 {
			warnings = append(warnings, aiWarnings...)
		}
	}
	sourceType := "mysql_slow_log"
	if strings.TrimSpace(taskReq.UploadedFilePath) != "" {
		sourceType = "uploaded_mysql_slow_log"
	}
	result.Asset = buildAssetVO(*asset)
	result.Source = dbmodel.SlowLogSource{
		Configured: true,
		Type:       sourceType,
		Path:       task.SlowLogPath,
		HostID:     task.HostID,
		HostIP:     firstNonEmpty(hostIP, task.HostIP),
		HostName:   firstNonEmpty(hostName, task.HostName),
		ReadLines:  result.Summary.TotalEntries,
	}
	result.Warnings = warnings
	result.GeneratedAt = time.Now().Format(time.RFC3339)
	resultJSON, _ := json.Marshal(result)
	completedAt := time.Now()
	_ = s.slowLogTaskDao.UpdateFields(taskID, map[string]interface{}{
		"status":          dbmodel.SlowLogTaskStatusSuccess,
		"progress":        100,
		"message":         fmt.Sprintf("分析完成，返回 Top %d 慢 SQL", len(result.Items)),
		"result_json":     string(resultJSON),
		"host_ip":         result.Source.HostIP,
		"host_name":       result.Source.HostName,
		"total_bytes":     remoteTotalBytes,
		"total_entries":   result.Summary.TotalEntries,
		"matched_entries": result.Summary.MatchedEntries,
		"unique_sql":      result.Summary.UniqueSQL,
		"completed_at":    &completedAt,
	})
	if strings.TrimSpace(taskReq.UploadedFilePath) != "" {
		_ = os.Remove(taskReq.UploadedFilePath)
	}
}

func (s *DatabaseService) failSlowLogTask(taskID string, err error) {
	completedAt := time.Now()
	_ = s.slowLogTaskDao.UpdateFields(taskID, map[string]interface{}{
		"status":       dbmodel.SlowLogTaskStatusFailed,
		"progress":     100,
		"message":      "分析失败",
		"error":        err.Error(),
		"completed_at": &completedAt,
	})
}

func buildSlowLogTaskStatus(task *dbmodel.SlowLogAnalyzeTask, includeResult bool) *dbmodel.SlowLogTaskStatus {
	if task == nil {
		return nil
	}
	status := &dbmodel.SlowLogTaskStatus{
		TaskID:        task.TaskID,
		AssetID:       task.AssetID,
		Status:        task.Status,
		Progress:      task.Progress,
		Message:       task.Message,
		Error:         task.Error,
		CreatedBy:     task.CreatedBy,
		CreatedByName: task.CreatedByName,
		CreatedAt:     formatTaskTime(task.CreatedAt.Time),
		Source: dbmodel.SlowLogSource{
			Configured: true,
			Type:       "mysql_slow_log",
			Path:       task.SlowLogPath,
			HostID:     task.HostID,
			HostIP:     task.HostIP,
			HostName:   task.HostName,
		},
		Summary: dbmodel.SlowLogSummary{
			TotalEntries:   task.TotalEntries,
			MatchedEntries: task.MatchedEntries,
			UniqueSQL:      task.UniqueSQL,
		},
	}
	if task.StartedAt != nil {
		status.StartedAt = formatTaskTime(*task.StartedAt)
	}
	if task.CompletedAt != nil {
		status.CompletedAt = formatTaskTime(*task.CompletedAt)
	}
	if includeResult && strings.TrimSpace(task.ResultJSON) != "" {
		var result dbmodel.SlowLogAnalyzeResult
		if err := json.Unmarshal([]byte(task.ResultJSON), &result); err == nil {
			status.Result = &result
			status.Summary = result.Summary
			status.Source = result.Source
		}
	}
	return status
}

func buildSlowLogFilter(query dbmodel.SlowLogQuery) (slowLogFilter, error) {
	begin, err := parseSlowLogQueryTime(query.BeginTime)
	if err != nil {
		return slowLogFilter{}, fmt.Errorf("invalid beginTime: %w", err)
	}
	end, err := parseSlowLogQueryTime(query.EndTime)
	if err != nil {
		return slowLogFilter{}, fmt.Errorf("invalid endTime: %w", err)
	}
	return slowLogFilter{
		begin:    begin,
		end:      end,
		keyword:  strings.ToLower(strings.TrimSpace(query.Keyword)),
		database: strings.ToLower(strings.TrimSpace(query.Database)),
		user:     strings.ToLower(strings.TrimSpace(query.User)),
	}, nil
}

func newSlowLogAccumulator(asset *cmdbmodel.CmdbSQL, filter slowLogFilter) *slowLogAccumulator {
	return &slowLogAccumulator{
		asset:      asset,
		filter:     filter,
		aggregates: map[string]*slowLogAggregate{},
	}
}

func (a *slowLogAccumulator) add(entry mysqlSlowLogEntry) {
	a.summary.TotalEntries++
	if !matchSlowLogEntry(entry, a.filter.begin, a.filter.end, a.filter.keyword, a.filter.database, a.filter.user) {
		return
	}
	a.summary.MatchedEntries++
	a.summary.TotalQueryTime += entry.QueryTime
	a.summary.TotalLockTime += entry.LockTime
	a.summary.TotalRowsExamined += entry.RowsExamined
	a.summary.TotalRowsSent += entry.RowsSent
	if entry.QueryTime > a.summary.MaxQueryTime {
		a.summary.MaxQueryTime = entry.QueryTime
	}
	if entry.RowsExamined >= 100000 {
		a.summary.LargeScanCount++
	}
	if strings.EqualFold(entry.Fingerprint, "COMMIT") {
		a.summary.SlowCommitCount++
	}
	if isConnectionProbeFingerprint(entry.Fingerprint) {
		a.summary.ConnectionProbeCount++
	}

	key := entry.Fingerprint
	if key == "" {
		key = entry.SQL
	}
	digest := slowLogDigest(key)
	agg := a.aggregates[digest]
	if agg == nil {
		assetName := ""
		if a.asset != nil {
			assetName = firstNonEmpty(a.asset.Name, a.asset.InstanceName)
		}
		agg = &slowLogAggregate{
			item: dbmodel.SlowLogItem{
				Digest:      digest,
				Fingerprint: key,
				SampleSQL:   truncateSlowLogSQL(entry.SQL),
				Database:    firstNonEmpty(entry.Schema, assetName),
				User:        entry.User,
				Host:        entry.Host,
				FirstSeen:   formatSlowLogTime(entry.Time),
				LastSeen:    formatSlowLogTime(entry.Time),
				Examples:    []string{},
			},
		}
		a.aggregates[digest] = agg
	}
	applySlowLogEntry(agg, entry)
}

func (a *slowLogAccumulator) result(query dbmodel.SlowLogQuery, limit int) dbmodel.SlowLogAnalyzeResult {
	items := make([]dbmodel.SlowLogItem, 0, len(a.aggregates))
	for _, agg := range a.aggregates {
		agg.item.QueryTimeAvg = roundSlowFloat(agg.item.QueryTimeTotal / float64(agg.item.Count))
		agg.item.QueryTimeP95 = roundSlowFloat(percentileSlowFloat(agg.queryTimes, 0.95))
		agg.item.QueryTimeTotal = roundSlowFloat(agg.item.QueryTimeTotal)
		agg.item.QueryTimeMax = roundSlowFloat(agg.item.QueryTimeMax)
		agg.item.LockTimeTotal = roundSlowFloat(agg.item.LockTimeTotal)
		items = append(items, agg.item)
	}
	a.summary.UniqueSQL = len(items)
	a.summary.TotalQueryTime = roundSlowFloat(a.summary.TotalQueryTime)
	a.summary.TotalLockTime = roundSlowFloat(a.summary.TotalLockTime)
	a.summary.MaxQueryTime = roundSlowFloat(a.summary.MaxQueryTime)
	sortSlowLogItems(items, query.SortBy, query.SortOrder)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return dbmodel.SlowLogAnalyzeResult{
		Summary:  a.summary,
		Items:    items,
		Page:     1,
		PageSize: limit,
		Total:    int64(a.summary.UniqueSQL),
		Warnings: []string{},
	}
}

func newMySQLSlowLogStreamParser(emit func(mysqlSlowLogEntry)) *mysqlSlowLogStreamParser {
	return &mysqlSlowLogStreamParser{emit: emit}
}

func (p *mysqlSlowLogStreamParser) Write(chunk []byte) {
	for len(chunk) > 0 {
		if idx := bytes.IndexByte(chunk, '\n'); idx >= 0 {
			line := append(p.lineBuffer, chunk[:idx]...)
			p.lineBuffer = p.lineBuffer[:0]
			p.processLine(string(line))
			chunk = chunk[idx+1:]
			continue
		}
		p.lineBuffer = append(p.lineBuffer, chunk...)
		break
	}
}

func (p *mysqlSlowLogStreamParser) Finish() {
	if len(p.lineBuffer) > 0 {
		p.processLine(string(p.lineBuffer))
		p.lineBuffer = nil
	}
	p.flush()
}

func (p *mysqlSlowLogStreamParser) flush() {
	if !p.hasCurrent {
		return
	}
	p.current.SQL = normalizeRawSQL(p.current.SQL)
	p.current.Fingerprint = fingerprintMySQLSQL(p.current.SQL)
	if p.current.QueryTime > 0 || p.current.SQL != "" {
		p.emit(p.current)
	}
	p.current = mysqlSlowLogEntry{}
	p.hasCurrent = false
}

func (p *mysqlSlowLogStreamParser) processLine(raw string) {
	line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
	if matches := slowLogTimeRegex.FindStringSubmatch(line); len(matches) == 2 {
		p.flush()
		p.hasCurrent = true
		p.current.Time = parseMySQLSlowLogTime(matches[1])
		return
	}
	if !p.hasCurrent && strings.HasPrefix(line, "# User@Host:") {
		p.hasCurrent = true
	}
	if !p.hasCurrent {
		return
	}
	if matches := slowLogUserHostRegex.FindStringSubmatch(line); len(matches) == 3 {
		p.current.User = strings.TrimSpace(matches[1])
		p.current.Host = strings.TrimSpace(matches[2])
		return
	}
	if matches := slowLogSchemaRegex.FindStringSubmatch(line); len(matches) == 2 {
		p.current.Schema = cleanSlowIdentifier(matches[1])
		return
	}
	if matches := slowLogQueryRegex.FindStringSubmatch(line); len(matches) >= 5 {
		p.current.QueryTime = parseSlowFloat(matches[1])
		p.current.LockTime = parseSlowFloat(matches[2])
		p.current.RowsSent = parseSlowInt64(matches[3])
		p.current.RowsExamined = parseSlowInt64(matches[4])
		if len(matches) > 5 {
			p.current.RowsAffected = parseSlowInt64(matches[5])
		}
		return
	}
	if matches := slowLogUseRegex.FindStringSubmatch(line); len(matches) == 2 {
		p.current.Schema = cleanSlowIdentifier(matches[1])
		return
	}
	if matches := slowLogSetTimestampRegex.FindStringSubmatch(line); len(matches) == 2 {
		if p.current.Time.IsZero() {
			p.current.Time = parseSlowLogUnixTime(matches[1])
		}
		return
	}
	lower := strings.ToLower(line)
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(lower, "set timestamp") {
		return
	}
	if p.current.SQL != "" {
		p.current.SQL += " "
	}
	p.current.SQL += line
}

func writeSlowLogItemsCSV(items []dbmodel.SlowLogItem, writer io.Writer) (int64, error) {
	csvWriter := csv.NewWriter(writer)
	header := []string{
		"指纹ID",
		"数据库",
		"用户",
		"来源主机",
		"首次出现时间",
		"最后出现时间",
		"执行次数",
		"平均执行时长(秒)",
		"P95执行时长(秒)",
		"最大执行时长(秒)",
		"累计执行时长(秒)",
		"累计锁等待时长(秒)",
		"累计扫描行数",
		"最大扫描行数",
		"累计返回行数",
		"最大返回行数",
		"AI慢查询原因",
		"AI表结构优化",
		"AI索引优化",
		"AI SQL语句优化",
		"AI预期效果",
		"SQL指纹",
		"样例SQL",
	}
	if err := csvWriter.Write(header); err != nil {
		return 0, err
	}
	for _, item := range items {
		record := []string{
			item.Digest,
			item.Database,
			item.User,
			item.Host,
			item.FirstSeen,
			item.LastSeen,
			strconv.Itoa(item.Count),
			formatSlowFloat(item.QueryTimeAvg),
			formatSlowFloat(item.QueryTimeP95),
			formatSlowFloat(item.QueryTimeMax),
			formatSlowFloat(item.QueryTimeTotal),
			formatSlowFloat(item.LockTimeTotal),
			strconv.FormatInt(item.RowsExaminedTotal, 10),
			strconv.FormatInt(item.RowsExaminedMax, 10),
			strconv.FormatInt(item.RowsSentTotal, 10),
			strconv.FormatInt(item.RowsSentMax, 10),
			item.AISuggestion.SlowReason,
			item.AISuggestion.SchemaOptimization,
			item.AISuggestion.IndexOptimization,
			item.AISuggestion.SQLOptimization,
			item.AISuggestion.ExpectedEffect,
			item.Fingerprint,
			item.SampleSQL,
		}
		if err := csvWriter.Write(record); err != nil {
			return 0, err
		}
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return 0, err
	}
	return int64(len(items)), nil
}

func (s *DatabaseService) enrichSlowLogAISuggestions(ctx *gin.Context, asset *cmdbmodel.CmdbSQL, result *dbmodel.SlowLogAnalyzeResult) []string {
	if result == nil || len(result.Items) == 0 {
		return nil
	}
	limit := minInt(len(result.Items), slowLogAISuggestionLimit)
	if limit <= 0 {
		return nil
	}

	provider, apiKey, err := aiopsservice.ResolveLLMProvider(0)
	if err != nil {
		return []string{"AI优化建议生成失败: " + err.Error()}
	}

	warnings := make([]string, 0)
	applied := 0
	for i := 0; i < limit; i++ {
		llmCtx, cancel := context.WithTimeout(context.Background(), llmTimeout(provider.TimeoutSeconds))
		prompt := buildSlowLogAISuggestionPrompt(asset, result.Items[i:i+1])
		answer, err := aiopsservice.CallLLM(llmCtx, provider, apiKey, slowLogAISuggestionSystemPrompt, prompt)
		cancel()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("第 %d 条慢SQL AI优化建议生成失败: %v", i+1, err))
			continue
		}
		suggestions, err := parseSlowLogAISuggestionAnswer(answer)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("第 %d 条慢SQL AI优化建议解析失败: %v", i+1, err))
			continue
		}
		if len(suggestions.ByDigest) == 0 && len(suggestions.Ordered) == 0 {
			warnings = append(warnings, fmt.Sprintf("第 %d 条慢SQL AI优化建议为空", i+1))
			continue
		}
		suggestion, ok := suggestions.ByDigest[result.Items[i].Digest]
		if !ok && len(suggestions.Ordered) > 0 {
			suggestion = suggestions.Ordered[0]
			ok = !slowLogAISuggestionEmpty(suggestion)
		}
		if ok && !slowLogAISuggestionEmpty(suggestion) {
			result.Items[i].AISuggestion = suggestion
			applied++
		}
	}
	if applied == 0 {
		return []string{"AI优化建议未匹配到慢SQL结果，已忽略本次AI返回"}
	}
	return warnings
}

func buildSlowLogAISuggestionPrompt(asset *cmdbmodel.CmdbSQL, items []dbmodel.SlowLogItem) string {
	rows := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		rows = append(rows, map[string]interface{}{
			"digest":            item.Digest,
			"database":          item.Database,
			"user":              item.User,
			"host":              item.Host,
			"count":             item.Count,
			"queryTimeAvg":      item.QueryTimeAvg,
			"queryTimeP95":      item.QueryTimeP95,
			"queryTimeMax":      item.QueryTimeMax,
			"queryTimeTotal":    item.QueryTimeTotal,
			"lockTimeTotal":     item.LockTimeTotal,
			"rowsExaminedTotal": item.RowsExaminedTotal,
			"rowsExaminedMax":   item.RowsExaminedMax,
			"rowsSentTotal":     item.RowsSentTotal,
			"rowsSentMax":       item.RowsSentMax,
			"fingerprint":       item.Fingerprint,
			"sampleSql":         truncateForDBAIPrompt(item.SampleSQL, 2500),
		})
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"asset": map[string]interface{}{
			"id":           asset.ID,
			"name":         asset.Name,
			"instanceName": asset.InstanceName,
			"type":         databaseTypeName(asset.Type),
			"hostIp":       asset.HostIP,
			"endpoint":     asset.Endpoint,
			"port":         asset.Port,
		},
		"slowSqlTop": rows,
	})

	var builder strings.Builder
	builder.WriteString("请根据 MySQL 慢日志 Top SQL 的聚合指标和样例 SQL，给每条 SQL 生成分段、可落地的优化建议。\n")
	builder.WriteString("要求：\n")
	builder.WriteString("1. 只分析输入 JSON 中的 SQL 和指标，不要编造表结构、字段类型或已有索引；证据不足时要写“需结合 EXPLAIN/表结构确认”。\n")
	builder.WriteString("2. 每条建议必须拆成：慢查询原因、表结构优化、索引优化、SQL语句优化、预期效果。\n")
	builder.WriteString("3. 如果涉及 UPDATE/DELETE/DDL，只给优化方向和审核建议，不要暗示可直接生产执行。\n")
	builder.WriteString("4. 每个字段控制在 160 字以内，SQL 改写可以稍长但保持简洁。\n")
	builder.WriteString("5. 不要输出 Markdown，不要输出代码块。\n")
	builder.WriteString("6. 必须返回严格 JSON，不要遗漏 items，不要修改 digest。JSON 格式为：{\"items\":[{\"digest\":\"输入里的digest原值\",\"slowReason\":\"\",\"schemaOptimization\":\"\",\"indexOptimization\":\"\",\"sqlOptimization\":\"\",\"expectedEffect\":\"\"}]}\n")
	builder.WriteString("\n输入 JSON：\n")
	builder.WriteString(truncateForDBAIPrompt(string(payload), 30000))
	return builder.String()
}

type slowLogAISuggestionParseResult struct {
	ByDigest map[string]dbmodel.SlowLogAISuggestion
	Ordered  []dbmodel.SlowLogAISuggestion
}

func parseSlowLogAISuggestionAnswer(answer string) (slowLogAISuggestionParseResult, error) {
	raw := extractSlowLogAIJSON(answer)
	var decoded interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		if slowLogAILooksLikeJSONFragment(answer) {
			return slowLogAISuggestionParseResult{}, err
		}
		suggestion := slowLogAISuggestionFromPlainText(answer)
		if slowLogAISuggestionEmpty(suggestion) {
			return slowLogAISuggestionParseResult{}, err
		}
		return slowLogAISuggestionParseResult{
			ByDigest: map[string]dbmodel.SlowLogAISuggestion{},
			Ordered:  []dbmodel.SlowLogAISuggestion{suggestion},
		}, nil
	}
	result := slowLogAISuggestionParseResult{
		ByDigest: map[string]dbmodel.SlowLogAISuggestion{},
		Ordered:  []dbmodel.SlowLogAISuggestion{},
	}
	items, ok := slowLogAIItems(decoded)
	if !ok {
		if item, itemOK := decoded.(map[string]interface{}); itemOK {
			for key, value := range item {
				if suggestion := slowLogAISuggestionFromAny(value); !slowLogAISuggestionEmpty(suggestion) {
					result.ByDigest[strings.TrimSpace(key)] = suggestion
				}
			}
		}
		return result, nil
	}
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]interface{})
		if !ok {
			suggestion := slowLogAISuggestionFromAny(rawItem)
			if !slowLogAISuggestionEmpty(suggestion) {
				result.Ordered = append(result.Ordered, suggestion)
			}
			continue
		}
		digest := slowLogAIString(item, "digest", "fingerprintId", "fingerprintID", "指纹ID", "指纹id")
		suggestion := slowLogAISuggestionFromMap(item)
		if slowLogAISuggestionEmpty(suggestion) {
			continue
		}
		result.Ordered = append(result.Ordered, suggestion)
		if digest != "" {
			result.ByDigest[digest] = suggestion
		}
	}
	return result, nil
}

func extractSlowLogAIJSON(answer string) string {
	raw := strings.TrimSpace(answer)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "{") || strings.HasPrefix(raw, "[") {
		return raw
	}
	objectStart, objectEnd := strings.Index(raw, "{"), strings.LastIndex(raw, "}")
	arrayStart, arrayEnd := strings.Index(raw, "["), strings.LastIndex(raw, "]")
	if objectStart >= 0 && objectEnd > objectStart && (arrayStart < 0 || objectStart < arrayStart) {
		return raw[objectStart : objectEnd+1]
	}
	if arrayStart >= 0 && arrayEnd > arrayStart {
		return raw[arrayStart : arrayEnd+1]
	}
	return raw
}

func slowLogAILooksLikeJSONFragment(answer string) bool {
	text := strings.TrimSpace(answer)
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		return true
	}
	return strings.Contains(lower, `"digest"`) ||
		strings.Contains(lower, `"slowreason"`) ||
		strings.Contains(lower, `"schemaoptimization"`) ||
		strings.Contains(lower, `"indexoptimization"`) ||
		strings.Contains(lower, `"sqloptimization"`) ||
		strings.Contains(lower, `"expectedeffect"`) ||
		strings.Contains(text, `"}, {`) ||
		strings.Contains(text, `},{`)
}

func slowLogAIItems(decoded interface{}) ([]interface{}, bool) {
	switch value := decoded.(type) {
	case []interface{}:
		return value, true
	case map[string]interface{}:
		for _, key := range []string{"items", "data", "suggestions", "aiSuggestions", "优化建议"} {
			if items, ok := value[key].([]interface{}); ok {
				return items, true
			}
		}
		if _, ok := value["aiSuggestion"]; ok || slowLogAIString(value, "digest", "fingerprintId", "指纹ID") != "" || slowLogAIMapLooksLikeSuggestion(value) {
			return []interface{}{value}, true
		}
	}
	return nil, false
}

func slowLogAIMapLooksLikeSuggestion(item map[string]interface{}) bool {
	return slowLogAIString(item,
		"slowReason", "reason", "slowQueryReason", "cause", "suggestion", "summary",
		"schemaOptimization", "tableOptimization", "structureOptimization",
		"indexOptimization", "indexSuggestion",
		"sqlOptimization", "sqlRewrite", "queryOptimization",
		"expectedEffect", "effect",
		"慢查询原因", "原因", "表结构优化", "结构优化", "索引优化", "索引建议",
		"SQL语句优化", "SQL优化", "SQL 语句优化", "预期效果", "优化效果", "效果",
	) != ""
}

func slowLogAISuggestionFromAny(value interface{}) dbmodel.SlowLogAISuggestion {
	switch typed := value.(type) {
	case string:
		if suggestion := slowLogAISuggestionFromPlainText(typed); !slowLogAISuggestionEmpty(suggestion) {
			return suggestion
		}
		return dbmodel.SlowLogAISuggestion{SlowReason: strings.TrimSpace(typed)}
	case map[string]interface{}:
		return slowLogAISuggestionFromMap(typed)
	default:
		return dbmodel.SlowLogAISuggestion{}
	}
}

func slowLogAISuggestionFromPlainText(text string) dbmodel.SlowLogAISuggestion {
	text = strings.TrimSpace(text)
	if text == "" {
		return dbmodel.SlowLogAISuggestion{}
	}
	suggestion := dbmodel.SlowLogAISuggestion{
		SlowReason:         slowLogAISectionFromText(text, "慢查询原因"),
		SchemaOptimization: slowLogAISectionFromText(text, "表结构优化"),
		IndexOptimization:  slowLogAISectionFromText(text, "索引优化"),
		SQLOptimization:    slowLogAISectionFromText(text, "SQL语句优化"),
		ExpectedEffect:     slowLogAISectionFromText(text, "预期效果"),
	}
	if slowLogAISuggestionEmpty(suggestion) {
		suggestion.SlowReason = text
	}
	return suggestion
}

func slowLogAISuggestionFromMap(item map[string]interface{}) dbmodel.SlowLogAISuggestion {
	if nested, ok := item["aiSuggestion"]; ok {
		if suggestion := slowLogAISuggestionFromAny(nested); !slowLogAISuggestionEmpty(suggestion) {
			return suggestion
		}
	}
	return dbmodel.SlowLogAISuggestion{
		SlowReason: strings.TrimSpace(firstNonEmpty(
			slowLogAIString(item, "slowReason", "reason", "slowQueryReason", "cause", "suggestion", "summary", "慢查询原因", "原因"),
			slowLogAISectionFromText(slowLogAIString(item, "content", "text", "answer", "建议"), "慢查询原因"),
		)),
		SchemaOptimization: strings.TrimSpace(firstNonEmpty(
			slowLogAIString(item, "schemaOptimization", "tableOptimization", "structureOptimization", "表结构优化", "结构优化"),
			slowLogAISectionFromText(slowLogAIString(item, "content", "text", "answer", "建议"), "表结构优化"),
		)),
		IndexOptimization: strings.TrimSpace(firstNonEmpty(
			slowLogAIString(item, "indexOptimization", "indexSuggestion", "索引优化", "索引建议"),
			slowLogAISectionFromText(slowLogAIString(item, "content", "text", "answer", "建议"), "索引优化"),
		)),
		SQLOptimization: strings.TrimSpace(firstNonEmpty(
			slowLogAIString(item, "sqlOptimization", "sqlRewrite", "queryOptimization", "SQL语句优化", "SQL优化", "SQL 语句优化"),
			slowLogAISectionFromText(slowLogAIString(item, "content", "text", "answer", "建议"), "SQL语句优化"),
		)),
		ExpectedEffect: strings.TrimSpace(firstNonEmpty(
			slowLogAIString(item, "expectedEffect", "effect", "预期效果", "效果"),
			slowLogAISectionFromText(slowLogAIString(item, "content", "text", "answer", "建议"), "预期效果"),
		)),
	}
}

func slowLogAIString(item map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		value, ok := item[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case fmt.Stringer:
			if strings.TrimSpace(typed.String()) != "" {
				return strings.TrimSpace(typed.String())
			}
		}
	}
	return ""
}

func slowLogAISectionFromText(text, title string) string {
	text = strings.TrimSpace(text)
	if text == "" || title == "" {
		return ""
	}
	aliases := map[string][]string{
		"慢查询原因":   {"慢查询原因", "查询慢原因", "原因"},
		"表结构优化":   {"表结构优化", "结构优化"},
		"索引优化":    {"索引优化", "索引建议"},
		"SQL语句优化": {"SQL语句优化", "SQL优化", "SQL 语句优化"},
		"预期效果":    {"预期效果", "优化效果", "效果"},
	}
	titles := []string{title}
	if values, ok := aliases[title]; ok {
		titles = values
	}
	start := -1
	for _, candidate := range titles {
		if idx := strings.Index(text, candidate); idx >= 0 && (start < 0 || idx < start) {
			start = idx
		}
	}
	if start < 0 {
		return ""
	}
	section := text[start:]
	if colon := strings.IndexAny(section, ":："); colon >= 0 && colon < 12 {
		section = section[colon+1:]
	} else {
		for _, candidate := range titles {
			section = strings.TrimPrefix(section, candidate)
		}
	}
	end := len(section)
	for _, candidates := range aliases {
		for _, candidate := range candidates {
			if idx := strings.Index(section[1:], candidate); idx >= 0 && idx+1 < end {
				end = idx + 1
			}
		}
	}
	return strings.TrimSpace(strings.Trim(section[:end], "\n\r\t -：:"))
}

func slowLogAISuggestionEmpty(value dbmodel.SlowLogAISuggestion) bool {
	return strings.TrimSpace(value.SlowReason) == "" &&
		strings.TrimSpace(value.SchemaOptimization) == "" &&
		strings.TrimSpace(value.IndexOptimization) == "" &&
		strings.TrimSpace(value.SQLOptimization) == "" &&
		strings.TrimSpace(value.ExpectedEffect) == ""
}

func formatTaskTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func (s *DatabaseService) AnalyzeSlowLog(ctx *gin.Context, assetID uint, query dbmodel.SlowLogQuery) (*dbmodel.SlowLogAnalyzeResult, error) {
	return s.analyzeSlowLog(ctx, assetID, query, dbmodel.MaxPageSize)
}

func (s *DatabaseService) analyzeSlowLog(ctx *gin.Context, assetID uint, query dbmodel.SlowLogQuery, maxPageSize int) (result *dbmodel.SlowLogAnalyzeResult, err error) {
	query.Page, query.PageSize = normalizeSlowLogPage(query.Page, query.PageSize, maxPageSize)
	query.TailLines = normalizeSlowLogTailLines(query.TailLines)

	asset, err := s.assetDao.GetByID(assetID)
	if err != nil {
		return nil, err
	}
	recorder := NewAuditRecorder(ctx, asset.ID, asset.Type, dbmodel.ActionSlowLog, query)
	defer func() { recorder.Finish(err) }()

	if asset.Type != cmdbmodel.DatabaseTypeMySQL {
		err = fmt.Errorf("slow log analysis currently supports MySQL only")
		return nil, err
	}
	if strings.TrimSpace(asset.SlowLogPath) == "" {
		err = fmt.Errorf("slowLogPath is not configured for this database asset")
		return nil, err
	}

	hostID, fallbackHost, err := resolveSlowLogHost(asset)
	if err != nil {
		return nil, err
	}

	lines, host, err := aiopsservice.NewLogAnalysisService().ReadHostFileTail(ctx, hostID, query.AuthID, asset.SlowLogPath, query.TailLines)
	if err != nil {
		return nil, err
	}
	hostName := ""
	hostIP := asset.HostIP
	if host != nil {
		hostName = firstNonEmpty(host.HostName, host.Name)
		hostIP = firstNonEmpty(host.PrivateIP, host.SSHIP, host.PublicIP, asset.HostIP)
	} else if fallbackHost != nil {
		hostName = firstNonEmpty(fallbackHost.HostName, fallbackHost.Name)
		hostIP = firstNonEmpty(fallbackHost.PrivateIP, fallbackHost.SSHIP, fallbackHost.PublicIP, asset.HostIP)
	}

	result, err = analyzeMySQLSlowLogLines(asset, lines, query)
	if err != nil {
		return nil, err
	}
	result.Asset = buildAssetVO(*asset)
	result.Source = dbmodel.SlowLogSource{
		Configured: true,
		Type:       "mysql_slow_log",
		Path:       asset.SlowLogPath,
		HostID:     hostID,
		HostIP:     hostIP,
		HostName:   hostName,
		TailLines:  query.TailLines,
		ReadLines:  len(lines),
	}
	result.GeneratedAt = time.Now().Format(time.RFC3339)
	recorder.SetRowsAffected(int64(result.Summary.MatchedEntries))
	return result, nil
}

func (s *DatabaseService) ExportSlowLogCSV(ctx *gin.Context, assetID uint, query dbmodel.SlowLogQuery, writer io.Writer) (int64, error) {
	query.Page = 1
	query.PageSize = maxCSVExportRows
	result, err := s.analyzeSlowLog(ctx, assetID, query, maxCSVExportRows)
	if err != nil {
		return 0, err
	}
	return writeSlowLogItemsCSV(result.Items, writer)
}

func resolveSlowLogHost(asset *cmdbmodel.CmdbSQL) (uint, *cmdbmodel.CmdbHost, error) {
	if asset == nil {
		return 0, nil, fmt.Errorf("database asset is nil")
	}
	if asset.HostID > 0 {
		return asset.HostID, nil, nil
	}
	hostIP := strings.TrimSpace(asset.HostIP)
	if hostIP == "" {
		return 0, nil, fmt.Errorf("database asset has no hostId or hostIp")
	}
	hostDao := cmdbdao.NewCmdbHostDao()
	host, err := hostDao.GetCmdbHostByIP(hostIP)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil, fmt.Errorf("cannot find cmdb host by hostIp %s", hostIP)
		}
		return 0, nil, err
	}
	return host.ID, &host, nil
}

func analyzeMySQLSlowLogLines(asset *cmdbmodel.CmdbSQL, lines []string, query dbmodel.SlowLogQuery) (*dbmodel.SlowLogAnalyzeResult, error) {
	entries := parseMySQLSlowLogEntries(strings.Join(lines, "\n"))
	begin, err := parseSlowLogQueryTime(query.BeginTime)
	if err != nil {
		return nil, fmt.Errorf("invalid beginTime: %w", err)
	}
	end, err := parseSlowLogQueryTime(query.EndTime)
	if err != nil {
		return nil, fmt.Errorf("invalid endTime: %w", err)
	}
	keyword := strings.ToLower(strings.TrimSpace(query.Keyword))
	database := strings.ToLower(strings.TrimSpace(query.Database))
	user := strings.ToLower(strings.TrimSpace(query.User))

	summary := dbmodel.SlowLogSummary{TotalEntries: len(entries)}
	aggregates := map[string]*slowLogAggregate{}
	for _, entry := range entries {
		if !matchSlowLogEntry(entry, begin, end, keyword, database, user) {
			continue
		}
		summary.MatchedEntries++
		summary.TotalQueryTime += entry.QueryTime
		summary.TotalLockTime += entry.LockTime
		summary.TotalRowsExamined += entry.RowsExamined
		summary.TotalRowsSent += entry.RowsSent
		if entry.QueryTime > summary.MaxQueryTime {
			summary.MaxQueryTime = entry.QueryTime
		}
		if entry.RowsExamined >= 100000 {
			summary.LargeScanCount++
		}
		if strings.EqualFold(entry.Fingerprint, "COMMIT") {
			summary.SlowCommitCount++
		}
		if isConnectionProbeFingerprint(entry.Fingerprint) {
			summary.ConnectionProbeCount++
		}

		key := entry.Fingerprint
		if key == "" {
			key = entry.SQL
		}
		digest := slowLogDigest(key)
		agg := aggregates[digest]
		if agg == nil {
			agg = &slowLogAggregate{
				item: dbmodel.SlowLogItem{
					Digest:      digest,
					Fingerprint: key,
					SampleSQL:   truncateSlowLogSQL(entry.SQL),
					Database:    firstNonEmpty(entry.Schema, asset.Name, asset.InstanceName),
					User:        entry.User,
					Host:        entry.Host,
					FirstSeen:   formatSlowLogTime(entry.Time),
					LastSeen:    formatSlowLogTime(entry.Time),
					Examples:    []string{},
				},
			}
			aggregates[digest] = agg
		}
		applySlowLogEntry(agg, entry)
	}

	items := make([]dbmodel.SlowLogItem, 0, len(aggregates))
	for _, agg := range aggregates {
		agg.item.QueryTimeAvg = roundSlowFloat(agg.item.QueryTimeTotal / float64(agg.item.Count))
		agg.item.QueryTimeP95 = roundSlowFloat(percentileSlowFloat(agg.queryTimes, 0.95))
		agg.item.QueryTimeTotal = roundSlowFloat(agg.item.QueryTimeTotal)
		agg.item.QueryTimeMax = roundSlowFloat(agg.item.QueryTimeMax)
		agg.item.LockTimeTotal = roundSlowFloat(agg.item.LockTimeTotal)
		items = append(items, agg.item)
	}
	summary.UniqueSQL = len(items)
	summary.TotalQueryTime = roundSlowFloat(summary.TotalQueryTime)
	summary.TotalLockTime = roundSlowFloat(summary.TotalLockTime)
	summary.MaxQueryTime = roundSlowFloat(summary.MaxQueryTime)

	sortSlowLogItems(items, query.SortBy, query.SortOrder)
	total := int64(len(items))
	pageItems := paginateSlowLogItems(items, query.Page, query.PageSize)
	return &dbmodel.SlowLogAnalyzeResult{
		Summary:  summary,
		Items:    pageItems,
		Page:     query.Page,
		PageSize: query.PageSize,
		Total:    total,
		Warnings: []string{},
	}, nil
}

func parseMySQLSlowLogEntries(text string) []mysqlSlowLogEntry {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	entries := make([]mysqlSlowLogEntry, 0)
	current := mysqlSlowLogEntry{}
	hasCurrent := false

	flush := func() {
		if !hasCurrent {
			return
		}
		current.SQL = normalizeRawSQL(current.SQL)
		current.Fingerprint = fingerprintMySQLSQL(current.SQL)
		if current.QueryTime > 0 || current.SQL != "" {
			entries = append(entries, current)
		}
		current = mysqlSlowLogEntry{}
		hasCurrent = false
	}

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if matches := slowLogTimeRegex.FindStringSubmatch(line); len(matches) == 2 {
			flush()
			hasCurrent = true
			current.Time = parseMySQLSlowLogTime(matches[1])
			continue
		}
		if !hasCurrent && strings.HasPrefix(line, "# User@Host:") {
			hasCurrent = true
		}
		if !hasCurrent {
			continue
		}
		if matches := slowLogUserHostRegex.FindStringSubmatch(line); len(matches) == 3 {
			current.User = strings.TrimSpace(matches[1])
			current.Host = strings.TrimSpace(matches[2])
			continue
		}
		if matches := slowLogSchemaRegex.FindStringSubmatch(line); len(matches) == 2 {
			current.Schema = cleanSlowIdentifier(matches[1])
			continue
		}
		if matches := slowLogQueryRegex.FindStringSubmatch(line); len(matches) >= 5 {
			current.QueryTime = parseSlowFloat(matches[1])
			current.LockTime = parseSlowFloat(matches[2])
			current.RowsSent = parseSlowInt64(matches[3])
			current.RowsExamined = parseSlowInt64(matches[4])
			if len(matches) > 5 {
				current.RowsAffected = parseSlowInt64(matches[5])
			}
			continue
		}
		if matches := slowLogUseRegex.FindStringSubmatch(line); len(matches) == 2 {
			current.Schema = cleanSlowIdentifier(matches[1])
			continue
		}
		if matches := slowLogSetTimestampRegex.FindStringSubmatch(line); len(matches) == 2 {
			if current.Time.IsZero() {
				current.Time = parseSlowLogUnixTime(matches[1])
			}
			continue
		}
		lower := strings.ToLower(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(lower, "set timestamp") {
			continue
		}
		if current.SQL != "" {
			current.SQL += " "
		}
		current.SQL += line
	}
	flush()
	return entries
}

func matchSlowLogEntry(entry mysqlSlowLogEntry, begin, end *time.Time, keyword, database, user string) bool {
	if begin != nil && !entry.Time.IsZero() && entry.Time.Before(*begin) {
		return false
	}
	if end != nil && !entry.Time.IsZero() && entry.Time.After(*end) {
		return false
	}
	if database != "" && strings.ToLower(entry.Schema) != database {
		return false
	}
	if user != "" && !strings.Contains(strings.ToLower(entry.User), user) {
		return false
	}
	if keyword != "" {
		text := strings.ToLower(entry.SQL + "\n" + entry.Fingerprint + "\n" + entry.Schema + "\n" + entry.User + "\n" + entry.Host)
		if !strings.Contains(text, keyword) {
			return false
		}
	}
	return true
}

func applySlowLogEntry(agg *slowLogAggregate, entry mysqlSlowLogEntry) {
	agg.item.Count++
	agg.item.QueryTimeTotal += entry.QueryTime
	agg.item.LockTimeTotal += entry.LockTime
	agg.item.RowsExaminedTotal += entry.RowsExamined
	agg.item.RowsSentTotal += entry.RowsSent
	if entry.QueryTime > agg.item.QueryTimeMax {
		agg.item.QueryTimeMax = entry.QueryTime
	}
	if entry.RowsExamined > agg.item.RowsExaminedMax {
		agg.item.RowsExaminedMax = entry.RowsExamined
	}
	if entry.RowsSent > agg.item.RowsSentMax {
		agg.item.RowsSentMax = entry.RowsSent
	}
	agg.queryTimes = append(agg.queryTimes, entry.QueryTime)
	if agg.item.Database == "" && entry.Schema != "" {
		agg.item.Database = entry.Schema
	}
	if agg.item.User == "" && entry.User != "" {
		agg.item.User = entry.User
	}
	if agg.item.Host == "" && entry.Host != "" {
		agg.item.Host = entry.Host
	}
	if !entry.Time.IsZero() {
		if agg.item.FirstSeen == "" || entry.Time.Before(parseSlowDisplayTime(agg.item.FirstSeen)) {
			agg.item.FirstSeen = formatSlowLogTime(entry.Time)
		}
		if agg.item.LastSeen == "" || entry.Time.After(parseSlowDisplayTime(agg.item.LastSeen)) {
			agg.item.LastSeen = formatSlowLogTime(entry.Time)
		}
	}
	if len(agg.item.Examples) < 3 {
		sample := truncateSlowLogSQL(entry.SQL)
		if sample != "" && !containsSlowLogExample(agg.item.Examples, sample) {
			agg.item.Examples = append(agg.item.Examples, sample)
		}
	}
}

func fingerprintMySQLSQL(sql string) string {
	sql = normalizeRawSQL(sql)
	if sql == "" {
		return ""
	}
	trimmed := strings.TrimSuffix(sql, ";")
	lower := strings.ToLower(trimmed)
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
	normalized := slowLogStringRegex.ReplaceAllString(trimmed, "?")
	normalized = slowLogNumberRegex.ReplaceAllString(normalized, "?")
	normalized = slowLogInListRegex.ReplaceAllString(normalized, "IN (?)")
	normalized = slowLogWhitespaceRegex.ReplaceAllString(normalized, " ")
	return strings.TrimSpace(normalized)
}

func normalizeRawSQL(sql string) string {
	sql = strings.ReplaceAll(sql, "\n", " ")
	sql = strings.ReplaceAll(sql, "\t", " ")
	sql = slowLogWhitespaceRegex.ReplaceAllString(sql, " ")
	return strings.TrimSpace(sql)
}

func sortSlowLogItems(items []dbmodel.SlowLogItem, sortBy, sortOrder string) {
	desc := !strings.EqualFold(strings.TrimSpace(sortOrder), "asc")
	sortBy = strings.TrimSpace(sortBy)
	if sortBy == "" {
		sortBy = "queryTimeTotal"
	}
	lessValue := func(i, j int) bool {
		left, right := slowLogSortValue(items[i], sortBy), slowLogSortValue(items[j], sortBy)
		if left == right {
			return items[i].Count > items[j].Count
		}
		if desc {
			return left > right
		}
		return left < right
	}
	sort.SliceStable(items, lessValue)
}

func normalizeSlowLogResultSortBy(sortBy string) string {
	switch strings.TrimSpace(sortBy) {
	case "":
		return ""
	case "count", "queryTimeP95", "queryTimeAvg", "queryTimeMax", "queryTimeTotal", "lockTimeTotal", "rowsExaminedTotal":
		return strings.TrimSpace(sortBy)
	default:
		return "queryTimeTotal"
	}
}

func normalizeSlowLogResultSortOrder(sortOrder string) string {
	if strings.EqualFold(strings.TrimSpace(sortOrder), "asc") {
		return "asc"
	}
	return "desc"
}

func slowLogSortValue(item dbmodel.SlowLogItem, sortBy string) float64 {
	switch sortBy {
	case "count":
		return float64(item.Count)
	case "queryTimeAvg":
		return item.QueryTimeAvg
	case "queryTimeP95":
		return item.QueryTimeP95
	case "queryTimeMax":
		return item.QueryTimeMax
	case "lockTimeTotal":
		return item.LockTimeTotal
	case "rowsExaminedTotal":
		return float64(item.RowsExaminedTotal)
	case "rowsExaminedMax":
		return float64(item.RowsExaminedMax)
	case "rowsSentTotal":
		return float64(item.RowsSentTotal)
	case "rowsSentMax":
		return float64(item.RowsSentMax)
	default:
		return item.QueryTimeTotal
	}
}

func paginateSlowLogItems(items []dbmodel.SlowLogItem, page, pageSize int) []dbmodel.SlowLogItem {
	if len(items) == 0 {
		return []dbmodel.SlowLogItem{}
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []dbmodel.SlowLogItem{}
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end]
}

func parseSlowLogQueryTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("unsupported time format %q", value)
}

func parseMySQLSlowLogTime(value string) time.Time {
	value = strings.TrimSpace(value)
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999999Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"060102 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, time.Local); err == nil {
			return t
		}
	}
	return time.Time{}
}

func parseSlowLogUnixTime(value string) time.Time {
	seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || seconds <= 0 {
		return time.Time{}
	}
	return time.Unix(seconds, 0)
}

func parseSlowDisplayTime(value string) time.Time {
	t, _ := time.ParseInLocation("2006-01-02 15:04:05", value, time.Local)
	return t
}

func formatSlowLogTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func percentileSlowFloat(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	copied := append([]float64(nil), values...)
	sort.Float64s(copied)
	if len(copied) == 1 {
		return copied[0]
	}
	rank := p * float64(len(copied)-1)
	lower := int(math.Floor(rank))
	upper := int(math.Ceil(rank))
	if lower == upper {
		return copied[lower]
	}
	weight := rank - float64(lower)
	return copied[lower]*(1-weight) + copied[upper]*weight
}

func slowLogDigest(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])[:16]
}

func normalizeSlowLogTailLines(value int) int {
	if value <= 0 {
		return defaultSlowLogTailLines
	}
	if value > maxSlowLogTailLines {
		return maxSlowLogTailLines
	}
	return value
}

func normalizeSlowLogPage(page, pageSize, maxPageSize int) (int, int) {
	if page < 1 {
		page = dbmodel.DefaultPage
	}
	if pageSize < 1 {
		pageSize = dbmodel.DefaultPageSize
	}
	if maxPageSize < 1 {
		maxPageSize = dbmodel.MaxPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

func truncateSlowLogSQL(sql string) string {
	sql = normalizeRawSQL(sql)
	if len(sql) <= maxSlowLogSQLLength {
		return sql
	}
	return sql[:maxSlowLogSQLLength]
}

func cleanSlowIdentifier(value string) string {
	return strings.Trim(strings.TrimSpace(value), "`;")
}

func isConnectionProbeFingerprint(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(lower, "@@session.tx_read_only") ||
		strings.Contains(lower, "@@session.transaction_read_only") ||
		strings.Contains(lower, "@@version")
}

func containsSlowLogExample(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func parseSlowFloat(value string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return v
}

func parseSlowInt64(value string) int64 {
	v, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return v
}

func roundSlowFloat(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func formatSlowFloat(value float64) string {
	return strconv.FormatFloat(roundSlowFloat(value), 'f', 3, 64)
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func sanitizeSlowLogUploadFileName(name string) string {
	name = strings.TrimSpace(filepath.Base(name))
	if name == "." || name == string(filepath.Separator) {
		name = ""
	}
	replacer := strings.NewReplacer("\\", "_", "/", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	name = replacer.Replace(name)
	name = strings.TrimSpace(name)
	if name == "" {
		return "slow-log.log"
	}
	if len(name) > 180 {
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		if len(ext) > 20 {
			ext = ""
		}
		maxBase := 180 - len(ext)
		if maxBase < 1 {
			maxBase = 180
			ext = ""
		}
		if len(base) > maxBase {
			base = base[:maxBase]
		}
		name = base + ext
	}
	return name
}

func saveUploadedSlowLogFile(fileHeader *multipart.FileHeader, destPath string, maxBytes int64) error {
	src, err := fileHeader.Open()
	if err != nil {
		return fmt.Errorf("打开上传文件失败: %w", err)
	}
	defer src.Close()

	dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("创建慢日志临时文件失败: %w", err)
	}
	defer dst.Close()

	buffer := make([]byte, 256*1024)
	var written int64
	for {
		n, readErr := src.Read(buffer)
		if n > 0 {
			written += int64(n)
			if written > maxBytes {
				_ = os.Remove(destPath)
				return fmt.Errorf("上传的慢日志文件不能超过 150MB")
			}
			if _, err := dst.Write(buffer[:n]); err != nil {
				_ = os.Remove(destPath)
				return fmt.Errorf("保存慢日志临时文件失败: %w", err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = os.Remove(destPath)
			return fmt.Errorf("读取上传文件失败: %w", readErr)
		}
	}
	if written == 0 {
		_ = os.Remove(destPath)
		return fmt.Errorf("上传的慢日志文件为空")
	}
	if err := dst.Sync(); err != nil {
		_ = os.Remove(destPath)
		return fmt.Errorf("同步慢日志临时文件失败: %w", err)
	}
	return nil
}

func readLocalSlowLogFileChunks(path string, chunkSize int64, onChunk func([]byte, int64, int64) error) (int64, error) {
	if chunkSize <= 0 {
		chunkSize = 256 * 1024
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("打开上传慢日志文件失败: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("读取上传慢日志文件信息失败: %w", err)
	}
	totalBytes := stat.Size()
	if totalBytes <= 0 {
		return 0, fmt.Errorf("上传慢日志文件为空")
	}
	if totalBytes > maxSlowLogUploadBytes {
		return totalBytes, fmt.Errorf("上传的慢日志文件不能超过 150MB")
	}

	buffer := make([]byte, chunkSize)
	var readBytes int64
	for {
		n, readErr := file.Read(buffer)
		if n > 0 {
			readBytes += int64(n)
			chunk := make([]byte, n)
			copy(chunk, buffer[:n])
			if err := onChunk(chunk, readBytes, totalBytes); err != nil {
				return totalBytes, err
			}
			time.Sleep(5 * time.Millisecond)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return totalBytes, fmt.Errorf("读取上传慢日志文件失败: %w", readErr)
		}
	}
	return totalBytes, nil
}
