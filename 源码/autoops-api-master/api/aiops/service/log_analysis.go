package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
	"unicode"

	aiopsdao "dodevops-api/api/aiops/dao"
	aiopsmodel "dodevops-api/api/aiops/model"
	alertmodel "dodevops-api/api/alertmanager/model"
	cmdbdao "dodevops-api/api/cmdb/dao"
	cmdbmodel "dodevops-api/api/cmdb/model"
	configmodel "dodevops-api/api/configcenter/model"
	k8sdao "dodevops-api/api/k8s/dao"
	k8smodel "dodevops-api/api/k8s/model"
	k8sservice "dodevops-api/api/k8s/service"
	"dodevops-api/common"
	"dodevops-api/common/util"
	"dodevops-api/pkg/jwt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/pkg/sftp"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	prommodel "github.com/prometheus/common/model"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

const (
	defaultLogTailLines = 500
	maxLogTailLines     = 5000
	defaultLogLimit     = 1000
	maxLogLimit         = 5000
	maxLogTailBytes     = 8 * 1024 * 1024
	defaultFileLimit    = 200
	maxFileLimit        = 1000
	defaultFileDepth    = 2
	maxFileDepth        = 4

	defaultEngineBufferSize = 5000
	maxEngineBufferSize     = 20000
	maxLogAnalysisLines     = 100
	maxLogAnalysisChars     = 16000
)

var (
	logLevelRegex = regexp.MustCompile(`(?i)\b(trace|debug|info|warn|warning|error|err|fatal|panic|critical)\b`)
	timeRegexes   = []*regexp.Regexp{
		regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ][0-9:.+-:Z]{8,}`),
		regexp.MustCompile(`[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`),
	}
	patternRegexes = []struct {
		re   *regexp.Regexp
		repl string
	}{
		{regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ][0-9:.+-:Z]{8,}`), "<ts>"},
		{regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`), "<uuid>"},
		{regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}\b`), "<ip>"},
		{regexp.MustCompile(`0x[0-9a-fA-F]+`), "<hex>"},
		{regexp.MustCompile(`"[^"]*"`), `"<str>"`},
		{regexp.MustCompile(`'[^']*'`), `'<str>'`},
		{regexp.MustCompile(`\b\d+\b`), "<num>"},
		{regexp.MustCompile(`\s+`), " "},
	}
	secretRegexes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(authorization:\s*bearer\s+)[^\s,;]+`),
		regexp.MustCompile(`(?i)((?:password|passwd|pwd|token|secret|api[_-]?key|access[_-]?key|secret[_-]?key)\s*[=:]\s*)[^\s,;]+`),
	}
	k8sEvidenceKeywordRegex = regexp.MustCompile(`(?i)\b(error|exception|panic|fatal|timeout|timed out|refused|failed|failure|denied|forbidden|unauthorized|back-?off|oom|oomkilled|killed|unhealthy|crash|crashloopbackoff|deadline exceeded|connection reset|readiness|liveness|imagepullbackoff|errimagepull|failedmount|failedscheduling|evicted|jdwp|jdwp-handshake)\b`)
	k8sWarningEventReasons  = map[string]struct{}{
		"BackOff":            {},
		"ErrImagePull":       {},
		"Failed":             {},
		"FailedAttachVolume": {},
		"FailedCreate":       {},
		"FailedMount":        {},
		"FailedPull":         {},
		"FailedScheduling":   {},
		"ImagePullBackOff":   {},
		"Killing":            {},
		"NodeNotReady":       {},
		"OOMKilled":          {},
		"Unhealthy":          {},
	}
	stopWords = map[string]struct{}{
		"the": {}, "and": {}, "for": {}, "with": {}, "from": {}, "this": {}, "that": {},
		"true": {}, "false": {}, "null": {}, "info": {}, "debug": {}, "warn": {},
		"error": {}, "trace": {}, "fatal": {}, "time": {}, "date": {}, "host": {},
		"service": {}, "message": {}, "level": {},
	}
	timestampRegex             = regexp.MustCompile(`(\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2}(?:[,.]\d{3,9})?(?:Z|[+-]\d{2}:?\d{2})?|\w{3}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}(?:[,.]\d{3,6})?|\[\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}(?:[,.]\d{3,6})?\]|\d{2}:\d{2}:\d{2}(?:[,.]\d{3,6})?)`)
	severityPrefixRegex        = regexp.MustCompile(`^(?:\s*\[)?(TRACE|DEBUG|INFO|WARN|WARNING|ERROR|FATAL|CRITICAL)(?:\])?\s*[:>-]?\s*(.*)$`)
	manifestSensitiveNameRegex = regexp.MustCompile(`(?i)(password|passwd|pwd|token|secret|api[_-]?key|access[_-]?key|secret[_-]?key|credential|private[_-]?key)`)
)

type LogAnalysisService struct {
	llmDao   *aiopsdao.LLMProviderDao
	sessions *LogSessionManager
	parser   *LogLineParser
}

type k8sPodAnalyzeReportFunc func(aiopsmodel.K8sPodAnalyzeProgressEvent)
type k8sIngressAnalyzeReportFunc func(aiopsmodel.K8sIngressAnalyzeProgressEvent)

type k8sPodAnalyzeTask struct {
	mu          sync.RWMutex
	taskID      string
	status      string
	podStatus   string
	currentStep string
	progress    int
	message     string
	result      *aiopsmodel.K8sPodAnalyzeResult
	err         string
	events      []aiopsmodel.K8sPodAnalyzeProgressEvent
	createdAt   time.Time
	updatedAt   time.Time
}

type K8sPodAnalyzeTaskManager struct {
	tasks sync.Map
}

type logPatternCluster struct {
	Template string
	Sample   string
	Count    int
}

type logSessionStream struct {
	Name     string
	Source   string
	Active   bool
	LogCount int
	LastSeen time.Time
}

type LogPatternClusterer struct {
	clusters []*logPatternCluster
}

type LogAnalysisEngine struct {
	mu           sync.RWMutex
	sessionID    string
	source       aiopsmodel.LogSource
	format       string
	customFormat bool
	startedAt    time.Time
	lastSeen     time.Time
	bufferSize   int
	entries      []aiopsmodel.LogEntry
	totalLogs    int
	totalBytes   int64
	levelCounts  map[string]int
	wordCounts   map[string]int
	streams      map[string]*logSessionStream
	clusterer    *LogPatternClusterer
}

type LogSessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*LogAnalysisEngine
}

type LogTimestampParser struct {
	zonedLayouts     []string
	localDateLayouts []string
	partialLayouts   []string
}

type LogParseResult struct {
	Timestamp     string
	TimestampUnix int64
	Level         string
	Message       string
	Fields        map[string]interface{}
	Attributes    map[string]string
}

type LogFormatDefinition struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	Pattern struct {
		Main     string            `yaml:"main"`
		UseRegex bool              `yaml:"use_regex"`
		Fields   map[string]string `yaml:"fields"`
	} `yaml:"pattern"`
	JSON struct {
		Fields map[string]string `yaml:"fields"`
	} `yaml:"json"`
	Mapping struct {
		Timestamp        LogFieldExtractor            `yaml:"timestamp"`
		Severity         LogFieldExtractor            `yaml:"severity"`
		Body             LogFieldExtractor            `yaml:"body"`
		Attributes       map[string]LogFieldExtractor `yaml:"attributes"`
		AutoMapRemaining bool                         `yaml:"auto_map_remaining"`
	} `yaml:"mapping"`
}

type LogFieldExtractor struct {
	Field      string `yaml:"field"`
	Template   string `yaml:"template"`
	Pattern    string `yaml:"pattern"`
	Transform  string `yaml:"transform"`
	TimeFormat string `yaml:"time_format"`
	Default    string `yaml:"default"`
}

type CustomLogParser struct {
	format     *LogFormatDefinition
	mainRegex  *regexp.Regexp
	fieldRegex map[string]*regexp.Regexp
	templates  map[string]*template.Template
}

type LogLineParser struct {
	timestampParser *LogTimestampParser
}

var (
	sharedLogSessionManager        = NewLogSessionManager()
	sharedK8sPodAnalyzeTaskManager = NewK8sPodAnalyzeTaskManager()
)

func NewLogAnalysisService() *LogAnalysisService {
	return &LogAnalysisService{
		llmDao:   aiopsdao.NewLLMProviderDao(),
		sessions: sharedLogSessionManager,
		parser:   NewLogLineParser(),
	}
}

func (s *LogAnalysisService) ReadHostFileTail(c *gin.Context, hostID, authID uint, remotePath string, tailLines int) ([]string, *cmdbmodel.CmdbHost, error) {
	remotePath = strings.TrimSpace(remotePath)
	if remotePath == "" {
		return nil, nil, errors.New("remote path is required")
	}
	if tailLines <= 0 {
		tailLines = defaultLogTailLines
	}
	if tailLines > 50000 {
		tailLines = 50000
	}

	client, host, _, err := s.openHostSSHClient(c, hostID, authID)
	if err != nil {
		return nil, nil, err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return nil, nil, fmt.Errorf("create sftp client failed: %v", err)
	}
	defer sftpClient.Close()

	lines, err := readRemoteTailLines(sftpClient, remotePath, tailLines)
	if err != nil {
		return nil, nil, err
	}
	return lines, host, nil
}

func (s *LogAnalysisService) ReadHostFileChunks(c *gin.Context, hostID, authID uint, remotePath string, chunkSize, maxReadBytes int64, onChunk func([]byte, int64, int64) error) (*cmdbmodel.CmdbHost, error) {
	remotePath = strings.TrimSpace(remotePath)
	if remotePath == "" {
		return nil, errors.New("remote path is required")
	}
	if chunkSize <= 0 {
		chunkSize = 256 * 1024
	}
	if chunkSize > 1024*1024 {
		chunkSize = 1024 * 1024
	}
	if onChunk == nil {
		return nil, errors.New("onChunk is required")
	}

	client, host, _, err := s.openHostSSHClient(c, hostID, authID)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return nil, fmt.Errorf("create sftp client failed: %v", err)
	}
	defer sftpClient.Close()

	file, err := sftpClient.Open(remotePath)
	if err != nil {
		return nil, fmt.Errorf("open remote log file failed: %v", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat remote log file failed: %v", err)
	}
	if info.IsDir() {
		return nil, errors.New("remote path is a directory")
	}
	total := info.Size()
	if maxReadBytes <= 0 || maxReadBytes > total {
		maxReadBytes = total
	}
	buf := make([]byte, chunkSize)
	var readBytes int64
	for readBytes < maxReadBytes {
		remaining := maxReadBytes - readBytes
		readBuf := buf
		if remaining < int64(len(readBuf)) {
			readBuf = readBuf[:remaining]
		}
		n, readErr := file.Read(readBuf)
		if n > 0 {
			readBytes += int64(n)
			if err := onChunk(buf[:n], readBytes, total); err != nil {
				return nil, err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("read remote log file failed: %v", readErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return host, nil
}

func (s *LogAnalysisService) ReadHostFileTailChunks(c *gin.Context, hostID, authID uint, remotePath string, chunkSize, maxReadBytes int64, onChunk func([]byte, int64, int64) error) (*cmdbmodel.CmdbHost, error) {
	remotePath = strings.TrimSpace(remotePath)
	if remotePath == "" {
		return nil, errors.New("remote path is required")
	}
	if chunkSize <= 0 {
		chunkSize = 256 * 1024
	}
	if chunkSize > 1024*1024 {
		chunkSize = 1024 * 1024
	}
	if maxReadBytes <= 0 {
		maxReadBytes = 100 * 1024 * 1024
	}
	if onChunk == nil {
		return nil, errors.New("onChunk is required")
	}

	client, host, _, err := s.openHostSSHClient(c, hostID, authID)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return nil, fmt.Errorf("create sftp client failed: %v", err)
	}
	defer sftpClient.Close()

	file, err := sftpClient.Open(remotePath)
	if err != nil {
		return nil, fmt.Errorf("open remote log file failed: %v", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat remote log file failed: %v", err)
	}
	if info.IsDir() {
		return nil, errors.New("remote path is a directory")
	}
	total := info.Size()
	if total <= 0 {
		return host, nil
	}
	readLimit := maxReadBytes
	if readLimit > total {
		readLimit = total
	}
	start := total - readLimit
	if start > 0 {
		buf := make([]byte, 64*1024)
		n, readErr := file.ReadAt(buf, start)
		if readErr != nil && readErr != io.EOF {
			return nil, fmt.Errorf("read remote log file failed: %v", readErr)
		}
		if idx := bytes.IndexByte(buf[:n], '\n'); idx >= 0 {
			start += int64(idx + 1)
		}
	}

	buf := make([]byte, chunkSize)
	var readBytes int64
	for offset := start; offset < total; {
		remaining := total - offset
		readBuf := buf
		if remaining < int64(len(readBuf)) {
			readBuf = readBuf[:remaining]
		}
		n, readErr := file.ReadAt(readBuf, offset)
		if n > 0 {
			offset += int64(n)
			readBytes += int64(n)
			if err := onChunk(readBuf[:n], readBytes, total); err != nil {
				return nil, err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("read remote log file failed: %v", readErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
	return host, nil
}

func NewK8sPodAnalyzeTaskManager() *K8sPodAnalyzeTaskManager {
	return &K8sPodAnalyzeTaskManager{}
}

func (m *K8sPodAnalyzeTaskManager) Store(task *k8sPodAnalyzeTask) {
	if task == nil || strings.TrimSpace(task.taskID) == "" {
		return
	}
	m.tasks.Store(task.taskID, task)
}

func (m *K8sPodAnalyzeTaskManager) Get(taskID string) *k8sPodAnalyzeTask {
	value, ok := m.tasks.Load(strings.TrimSpace(taskID))
	if !ok {
		return nil
	}
	task, _ := value.(*k8sPodAnalyzeTask)
	return task
}

func newK8sPodAnalyzeTask(taskID string) *k8sPodAnalyzeTask {
	now := time.Now()
	task := &k8sPodAnalyzeTask{
		taskID:      taskID,
		status:      "running",
		podStatus:   "unknown",
		currentStep: "queued",
		progress:    0,
		message:     "任务已创建",
		events:      make([]aiopsmodel.K8sPodAnalyzeProgressEvent, 0, 16),
		createdAt:   now,
		updatedAt:   now,
	}
	task.addEvent(aiopsmodel.K8sPodAnalyzeProgressEvent{
		Type:     "started",
		Step:     "queued",
		Status:   "running",
		Progress: 0,
		Message:  "任务已创建",
	})
	return task
}

func (t *k8sPodAnalyzeTask) addEvent(event aiopsmodel.K8sPodAnalyzeProgressEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	if event.Type == "" {
		event.Type = "step"
	}
	if event.TaskID == "" {
		event.TaskID = t.taskID
	}
	if event.Status == "" {
		event.Status = t.status
	}
	if event.PodStatus == "" {
		event.PodStatus = t.podStatus
	}
	if event.Timestamp == "" {
		event.Timestamp = now.Format("2006-01-02 15:04:05")
	}
	if event.Progress < 0 {
		event.Progress = 0
	}
	if event.Progress > 100 {
		event.Progress = 100
	}

	t.status = event.Status
	if event.PodStatus != "" {
		t.podStatus = event.PodStatus
	}
	if event.Step != "" {
		t.currentStep = event.Step
	}
	if event.Progress >= t.progress || event.Status == "failed" || event.Status == "succeeded" {
		t.progress = event.Progress
	}
	if event.Message != "" {
		t.message = event.Message
	}
	if event.Result != nil {
		t.result = event.Result
	}
	if event.Error != "" {
		t.err = event.Error
	}
	t.updatedAt = now
	event = compactK8sPodProgressEvent(event)
	if len(t.events) > 0 && mergeK8sPodProgressEvent(&t.events[len(t.events)-1], event) {
		return
	}
	t.events = append(t.events, event)
}

func (t *k8sPodAnalyzeTask) succeed(result *aiopsmodel.K8sPodAnalyzeResult) {
	t.mu.RLock()
	status := t.status
	t.mu.RUnlock()
	if status == "succeeded" {
		return
	}
	reportK8sPodAnalyzeResult(t.addEvent, result)
}

func (t *k8sPodAnalyzeTask) fail(err error) {
	if err == nil {
		err = errors.New("analysis failed")
	}
	t.mu.RLock()
	step := t.currentStep
	progress := t.progress
	podStatus := t.podStatus
	t.mu.RUnlock()

	t.addEvent(aiopsmodel.K8sPodAnalyzeProgressEvent{
		Type:      "error",
		Step:      step,
		Status:    "failed",
		Progress:  progress,
		Message:   "分析失败",
		PodStatus: podStatus,
		Error:     err.Error(),
	})
}

func (t *k8sPodAnalyzeTask) snapshot(includeEvents bool) *aiopsmodel.K8sPodAnalyzeTaskStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var events []aiopsmodel.K8sPodAnalyzeProgressEvent
	if includeEvents {
		events = append([]aiopsmodel.K8sPodAnalyzeProgressEvent(nil), t.events...)
		for i := range events {
			events[i].Result = nil
		}
	}
	return &aiopsmodel.K8sPodAnalyzeTaskStatus{
		TaskID:      t.taskID,
		Status:      t.status,
		PodStatus:   t.podStatus,
		CurrentStep: t.currentStep,
		Progress:    t.progress,
		Message:     t.message,
		Result:      t.result,
		Error:       t.err,
		Events:      events,
		CreatedAt:   t.createdAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   t.updatedAt.Format("2006-01-02 15:04:05"),
	}
}

func (t *k8sPodAnalyzeTask) eventsFrom(index int) ([]aiopsmodel.K8sPodAnalyzeProgressEvent, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if index < 0 {
		index = 0
	}
	if index > len(t.events) {
		index = len(t.events)
	}
	events := append([]aiopsmodel.K8sPodAnalyzeProgressEvent(nil), t.events[index:]...)
	terminal := (t.status == "succeeded" || t.status == "failed") && index+len(events) >= len(t.events)
	return events, terminal
}

func reportK8sPodAnalyzeProgress(report k8sPodAnalyzeReportFunc, eventType, step string, progress int, message, podStatus string) {
	if report == nil {
		return
	}
	report(aiopsmodel.K8sPodAnalyzeProgressEvent{
		Type:      eventType,
		Step:      step,
		Status:    "running",
		Progress:  progress,
		Message:   message,
		PodStatus: podStatus,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	})
}

func reportK8sPodAnalyzeStepDone(report k8sPodAnalyzeReportFunc, step string, progress int, stepStatus, message, podStatus string, startedAt time.Time) {
	if report == nil {
		return
	}
	durationMs := int64(0)
	if !startedAt.IsZero() {
		durationMs = time.Since(startedAt).Milliseconds()
	}
	report(aiopsmodel.K8sPodAnalyzeProgressEvent{
		Type:       "step",
		Step:       step,
		Status:     "running",
		StepStatus: stepStatus,
		Progress:   progress,
		Message:    message,
		PodStatus:  podStatus,
		DurationMs: durationMs,
		Timestamp:  time.Now().Format("2006-01-02 15:04:05"),
	})
}

func reportK8sPodAnalyzeResult(report k8sPodAnalyzeReportFunc, result *aiopsmodel.K8sPodAnalyzeResult) {
	if report == nil || result == nil {
		return
	}
	message := "分析完成"
	if result.Status != "" {
		message = "分析完成，Pod 状态：" + result.Status
	}
	compactResult := compactK8sPodAnalyzeResult(result)
	report(aiopsmodel.K8sPodAnalyzeProgressEvent{
		Type:      "result",
		Step:      "result",
		Status:    "succeeded",
		Progress:  100,
		Message:   message,
		PodStatus: result.Status,
		Result:    compactResult,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	})
}

func compactK8sPodProgressEvent(event aiopsmodel.K8sPodAnalyzeProgressEvent) aiopsmodel.K8sPodAnalyzeProgressEvent {
	switch event.Step {
	case "queued":
		event.Step = "task_created"
		event.Message = "1. 任务创建"
	case "start":
		event.Step = "start"
		event.Message = "2. 开始分析"
	case "connect_cluster", "workload_pods", "pod_status":
		event.Step = "status_check"
		event.Message = "3. 定位对象并检查状态"
	case "logs_current", "logs_previous":
		event.Step = "log_scan"
		event.Message = "4. 采集关键日志"
	case "events", "node", "quota", "storage", "probes", "network_routes", "metrics", "metrics_trend":
		event.Step = "signal_scan"
		event.Message = "5. 汇总事件与资源信号"
	case "evidence", "ai_analysis":
		event.Step = "ai_diagnosis"
		event.Message = "6. AI 生成诊断结论"
	case "result":
		event.Step = "result"
		event.Message = "完成"
	}
	return event
}

func mergeK8sPodProgressEvent(last *aiopsmodel.K8sPodAnalyzeProgressEvent, next aiopsmodel.K8sPodAnalyzeProgressEvent) bool {
	if last == nil {
		return false
	}
	if next.Type == "result" || next.Type == "error" || last.Type == "result" || last.Type == "error" {
		return false
	}
	if last.Step != next.Step {
		return false
	}
	stepStatus := mergeK8sPodStepStatus(last.StepStatus, next.StepStatus)
	durationMs := last.DurationMs + next.DurationMs
	*last = next
	last.StepStatus = stepStatus
	if durationMs > 0 {
		last.DurationMs = durationMs
	}
	return true
}

func mergeK8sPodStepStatus(left, right string) string {
	left = strings.ToLower(strings.TrimSpace(left))
	right = strings.ToLower(strings.TrimSpace(right))
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	if left == "partial" || right == "partial" {
		return "partial"
	}
	if (left == "failed" && right == "succeeded") || (left == "succeeded" && right == "failed") {
		return "partial"
	}
	if left == "failed" || right == "failed" {
		return "failed"
	}
	if left == "succeeded" || right == "succeeded" {
		return "succeeded"
	}
	if left == "skipped" || right == "skipped" {
		return "skipped"
	}
	if left == "running" || right == "running" {
		return "running"
	}
	return right
}

func reportK8sIngressAnalyzeProgress(report k8sIngressAnalyzeReportFunc, eventType, step string, progress int, message string) {
	if report == nil {
		return
	}
	report(aiopsmodel.K8sIngressAnalyzeProgressEvent{
		Type:      eventType,
		Step:      step,
		Status:    "running",
		Progress:  progress,
		Message:   message,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	})
}

func reportK8sIngressAnalyzeResult(report k8sIngressAnalyzeReportFunc, result *aiopsmodel.K8sIngressAnalyzeResult) {
	if report == nil || result == nil {
		return
	}
	message := "分析完成"
	if result.Status != "" {
		message = "分析完成，Ingress 状态：" + result.Status
	}
	report(aiopsmodel.K8sIngressAnalyzeProgressEvent{
		Type:      "result",
		Step:      "result",
		Status:    "succeeded",
		Progress:  100,
		Message:   message,
		Result:    result,
		Timestamp: time.Now().Format("2006-01-02 15:04:05"),
	})
}

func (s *LogAnalysisService) ListSessions(query aiopsmodel.LogSessionsQuery) []aiopsmodel.LogSessionListItem {
	return s.sessions.List(query.ActiveOnly)
}

func (s *LogAnalysisService) GetSessionStatus(sessionID string) (*aiopsmodel.LogSessionStatus, error) {
	engine := s.sessions.Get(strings.TrimSpace(sessionID))
	if engine == nil {
		return nil, errors.New("log session not found")
	}
	return engine.Status(), nil
}

func (s *LogAnalysisService) ListHostLogFiles(c *gin.Context, query aiopsmodel.HostLogFilesQuery) (*aiopsmodel.HostLogFilesResult, error) {
	query.Dir = strings.TrimSpace(query.Dir)
	if query.Dir == "" {
		query.Dir = "/var/log"
	}
	remoteDir, err := cleanRemotePath(query.Dir)
	if err != nil {
		return nil, err
	}
	if query.MaxDepth <= 0 {
		query.MaxDepth = defaultFileDepth
	}
	if query.MaxDepth > maxFileDepth {
		query.MaxDepth = maxFileDepth
	}
	if query.Limit <= 0 {
		query.Limit = defaultFileLimit
	}
	if query.Limit > maxFileLimit {
		query.Limit = maxFileLimit
	}

	client, _, _, err := s.openHostSSHClient(c, query.HostID, query.AuthID)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return nil, fmt.Errorf("create sftp client failed: %v", err)
	}
	defer sftpClient.Close()

	files := make([]aiopsmodel.HostLogFileInfo, 0, query.Limit)
	keyword := strings.ToLower(strings.TrimSpace(query.Keyword))
	if err := walkRemoteLogFiles(sftpClient, remoteDir, query.MaxDepth, query.Limit, keyword, &files); err != nil {
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime > files[j].ModTime
	})

	return &aiopsmodel.HostLogFilesResult{
		HostID: query.HostID,
		Dir:    remoteDir,
		Files:  files,
		Total:  len(files),
	}, nil
}

func (s *LogAnalysisService) QueryHostLogs(c *gin.Context, req *aiopsmodel.HostLogQueryRequest) (*aiopsmodel.LogQueryResult, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	req.Path = strings.TrimSpace(req.Path)
	remotePath, err := cleanRemotePath(req.Path)
	if err != nil {
		return nil, err
	}
	req.Path = remotePath
	req.TailLines = normalizePositiveLimit(req.TailLines, defaultLogTailLines, maxLogTailLines)
	req.Limit = normalizePositiveLimit(req.Limit, defaultLogLimit, maxLogLimit)

	client, host, _, err := s.openHostSSHClient(c, req.HostID, req.AuthID)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return nil, fmt.Errorf("create sftp client failed: %v", err)
	}
	defer sftpClient.Close()

	lines, err := readRemoteTailLines(sftpClient, remotePath, req.TailLines)
	if err != nil {
		return nil, err
	}

	source := aiopsmodel.LogSource{
		Type:     aiopsmodel.LogSourceTypeHost,
		HostID:   host.ID,
		HostName: host.HostName,
		Path:     remotePath,
	}
	return s.buildLogQueryResultWithOptions(source, lines, &logQueryOptions{
		SessionID:  req.SessionID,
		Keyword:    req.Keyword,
		Regex:      req.Regex,
		Levels:     req.Levels,
		Limit:      req.Limit,
		FormatName: req.FormatName,
		FormatYAML: req.FormatYAML,
	})
}

func (s *LogAnalysisService) QueryK8sLogs(c *gin.Context, req *aiopsmodel.K8sLogQueryRequest) (*aiopsmodel.LogQueryResult, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	req.Namespace = strings.TrimSpace(req.Namespace)
	req.Pod = strings.TrimSpace(req.Pod)
	req.Container = trimK8sContainerDisplayName(req.Container)
	req.TailLines = normalizePositiveLimit(req.TailLines, defaultLogTailLines, maxLogTailLines)
	req.Limit = normalizePositiveLimit(req.Limit, defaultLogLimit, maxLogLimit)

	clientset, cluster, err := s.createK8sClient(req.ClusterID)
	if err != nil {
		return nil, err
	}

	containerName, err := resolveK8sPodContainer(c.Request.Context(), clientset, req.Namespace, req.Pod, req.Container)
	if err != nil {
		return nil, err
	}
	req.Container = containerName

	lines, err := s.readK8sTailLines(c.Request.Context(), clientset, req)
	if err != nil {
		return nil, err
	}

	source := aiopsmodel.LogSource{
		Type:      aiopsmodel.LogSourceTypeK8s,
		ClusterID: cluster.ID,
		Namespace: req.Namespace,
		Pod:       req.Pod,
		Container: containerName,
	}
	return s.buildLogQueryResultWithOptions(source, lines, &logQueryOptions{
		SessionID:  req.SessionID,
		Keyword:    req.Keyword,
		Regex:      req.Regex,
		Levels:     req.Levels,
		Limit:      req.Limit,
		FormatName: req.FormatName,
		FormatYAML: req.FormatYAML,
	})
}

func (s *LogAnalysisService) StreamHostLogs(c *gin.Context, req *aiopsmodel.HostLogStreamQuery, wsConn *websocket.Conn) error {
	if req == nil {
		return errors.New("request is nil")
	}
	remotePath, err := cleanRemotePath(req.Path)
	if err != nil {
		return err
	}
	req.Path = remotePath
	req.TailLines = normalizePositiveLimit(req.TailLines, defaultLogTailLines, maxLogTailLines)
	if req.Limit <= 0 {
		req.Limit = maxLogLimit
	}
	if req.Limit > maxLogLimit {
		req.Limit = maxLogLimit
	}

	client, host, _, err := s.openHostSSHClient(c, req.HostID, req.AuthID)
	if err != nil {
		return err
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return fmt.Errorf("create ssh session failed: %v", err)
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		client.Close()
		return fmt.Errorf("open stdout pipe failed: %v", err)
	}

	tailFlag := "-F"
	if !req.Follow {
		tailFlag = ""
	}
	command := strings.TrimSpace(fmt.Sprintf("tail -n %d %s -- %s 2>&1", req.TailLines, tailFlag, shellQuote(remotePath)))

	writer := &lockedWSWriter{conn: wsConn}
	source := aiopsmodel.LogSource{
		Type:     aiopsmodel.LogSourceTypeHost,
		HostID:   host.ID,
		HostName: host.HostName,
		Path:     remotePath,
	}
	filter, err := newLogFilter(&logQueryOptions{
		SessionID:  req.SessionID,
		Keyword:    req.Keyword,
		Regex:      req.Regex,
		Levels:     req.Levels,
		Limit:      req.Limit,
		FormatName: req.FormatName,
		FormatYAML: req.FormatYAML,
	})
	if err != nil {
		session.Close()
		client.Close()
		return err
	}
	engine := s.ensureSessionEngine(req.SessionID, source, req.FormatName, req.FormatYAML)
	engine.RegisterStream(source.Path, string(source.Type), req.Follow)
	customParser, err := NewCustomLogParser(req.FormatYAML)
	if err != nil {
		return err
	}

	var closeOnce sync.Once
	closeRemote := func() {
		closeOnce.Do(func() {
			_ = session.Signal(ssh.SIGTERM)
			_ = session.Close()
			_ = client.Close()
		})
	}
	defer closeRemote()
	defer engine.MarkStreamInactive(source.Path, string(source.Type))

	go func() {
		for {
			_, msg, err := wsConn.ReadMessage()
			if err != nil {
				closeRemote()
				return
			}
			var control struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(msg, &control) == nil && strings.EqualFold(control.Type, "close") {
				closeRemote()
				return
			}
		}
	}()

	if err := session.Start(command); err != nil {
		return fmt.Errorf("start tail command failed: %v", err)
	}

	_ = writer.WriteJSON(aiopsmodel.LogStreamEvent{
		Type:    "open",
		Message: "log stream started",
		Data:    source,
	})

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNo := 0
	sent := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		entry := s.parser.Parse(line, lineNo, source, customParser)
		if !filter.Match(entry) {
			continue
		}
		if sent >= req.Limit {
			_ = writer.WriteJSON(aiopsmodel.LogStreamEvent{
				Type:    "limit",
				Message: "stream entry limit reached",
			})
			closeRemote()
			break
		}
		sent++
		engine.Ingest(entry)
		if err := writer.WriteJSON(aiopsmodel.LogStreamEvent{Type: "entry", Data: entry}); err != nil {
			closeRemote()
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		_ = writer.WriteJSON(aiopsmodel.LogStreamEvent{Type: "error", Message: err.Error()})
	}

	waitErr := session.Wait()
	if waitErr != nil && sent == 0 {
		return fmt.Errorf("tail command failed: %v", waitErr)
	}
	_ = writer.WriteJSON(aiopsmodel.LogStreamEvent{Type: "close", Message: "log stream closed"})
	return nil
}

func (s *LogAnalysisService) StreamK8sLogs(c *gin.Context, req *aiopsmodel.K8sLogStreamQuery, wsConn *websocket.Conn) error {
	if req == nil {
		return errors.New("request is nil")
	}
	req.Namespace = strings.TrimSpace(req.Namespace)
	req.Pod = strings.TrimSpace(req.Pod)
	req.Container = trimK8sContainerDisplayName(req.Container)
	req.TailLines = normalizePositiveLimit(req.TailLines, defaultLogTailLines, maxLogTailLines)
	if req.Limit <= 0 {
		req.Limit = maxLogLimit
	}
	if req.Limit > maxLogLimit {
		req.Limit = maxLogLimit
	}

	clientset, cluster, err := s.createK8sStreamingClient(req.ClusterID)
	if err != nil {
		return err
	}

	containerName, err := resolveK8sPodContainer(c.Request.Context(), clientset, req.Namespace, req.Pod, req.Container)
	if err != nil {
		return err
	}
	req.Container = containerName

	logOptions := &corev1.PodLogOptions{
		Container:  containerName,
		Follow:     req.Follow,
		Previous:   req.Previous,
		Timestamps: true,
	}
	tailLines := int64(req.TailLines)
	logOptions.TailLines = &tailLines

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	stream, err := clientset.CoreV1().Pods(req.Namespace).GetLogs(req.Pod, logOptions).Stream(ctx)
	if err != nil {
		return fmt.Errorf("stream k8s pod logs failed: %v", err)
	}
	defer stream.Close()

	writer := &lockedWSWriter{conn: wsConn}
	source := aiopsmodel.LogSource{
		Type:      aiopsmodel.LogSourceTypeK8s,
		ClusterID: cluster.ID,
		Namespace: req.Namespace,
		Pod:       req.Pod,
		Container: containerName,
	}
	filter, err := newLogFilter(&logQueryOptions{
		SessionID:  req.SessionID,
		Keyword:    req.Keyword,
		Regex:      req.Regex,
		Levels:     req.Levels,
		Limit:      req.Limit,
		FormatName: req.FormatName,
		FormatYAML: req.FormatYAML,
	})
	if err != nil {
		return err
	}
	engine := s.ensureSessionEngine(req.SessionID, source, req.FormatName, req.FormatYAML)
	engine.RegisterStream(k8sLogStreamName(source), string(source.Type), req.Follow)
	customParser, err := NewCustomLogParser(req.FormatYAML)
	if err != nil {
		return err
	}
	defer engine.MarkStreamInactive(k8sLogStreamName(source), string(source.Type))

	go func() {
		for {
			_, msg, err := wsConn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
			var control struct {
				Type string `json:"type"`
			}
			if json.Unmarshal(msg, &control) == nil && strings.EqualFold(control.Type, "close") {
				cancel()
				return
			}
		}
	}()

	_ = writer.WriteJSON(aiopsmodel.LogStreamEvent{
		Type:    "open",
		Message: "log stream started",
		Data:    source,
	})

	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lineNo := 0
	sent := 0
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			_ = writer.WriteJSON(aiopsmodel.LogStreamEvent{Type: "close", Message: "log stream closed"})
			return nil
		default:
		}

		lineNo++
		line := scanner.Text()
		entry := s.parser.Parse(line, lineNo, source, customParser)
		if !filter.Match(entry) {
			continue
		}
		if sent >= req.Limit {
			_ = writer.WriteJSON(aiopsmodel.LogStreamEvent{
				Type:    "limit",
				Message: "stream entry limit reached",
			})
			cancel()
			break
		}
		sent++
		engine.Ingest(entry)
		if err := writer.WriteJSON(aiopsmodel.LogStreamEvent{Type: "entry", Data: entry}); err != nil {
			cancel()
			return err
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
		_ = writer.WriteJSON(aiopsmodel.LogStreamEvent{Type: "error", Message: err.Error()})
		return err
	}
	_ = writer.WriteJSON(aiopsmodel.LogStreamEvent{Type: "close", Message: "log stream closed"})
	return nil
}

func (s *LogAnalysisService) AnalyzeLogs(c *gin.Context, req *aiopsmodel.LogAnalyzeRequest) (*aiopsmodel.LogAnalyzeResult, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}

	entries := req.Entries
	var summary *aiopsmodel.LogSummary
	if len(entries) == 0 && strings.TrimSpace(req.SessionID) != "" {
		if engine := s.sessions.Get(strings.TrimSpace(req.SessionID)); engine != nil {
			snapshotEntries, engineSummary := engine.Snapshot(500)
			entries = snapshotEntries
			summary = &engineSummary
		}
	}
	if req.Summary != nil {
		summary = req.Summary
	}
	if len(entries) == 0 && req.Query != nil {
		queryResult, err := s.QueryHostLogs(c, req.Query)
		if err != nil {
			return nil, err
		}
		entries = queryResult.Entries
		summary = &queryResult.Summary
	}
	if len(entries) == 0 && req.K8sQuery != nil {
		queryResult, err := s.QueryK8sLogs(c, req.K8sQuery)
		if err != nil {
			return nil, err
		}
		entries = queryResult.Entries
		summary = &queryResult.Summary
	}

	lines := make([]string, 0, len(entries)+len(req.RawLines))
	for _, entry := range entries {
		lines = append(lines, formatLogEntryForPrompt(entry))
	}
	lines = append(lines, req.RawLines...)
	lines = limitAnalysisLines(lines, maxLogAnalysisLines, maxLogAnalysisChars)
	if len(lines) == 0 {
		return nil, errors.New("no logs to analyze")
	}

	provider, apiKey, err := s.resolveLLMProvider(req.ProviderID)
	if err != nil {
		return nil, err
	}
	prompt := buildLogAnalysisPrompt(req.Prompt, summary, lines)

	timeout := time.Duration(provider.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()

	analysis, err := callLLMForLogAnalysis(ctx, provider, apiKey, prompt)
	if err != nil {
		return nil, err
	}

	return &aiopsmodel.LogAnalyzeResult{
		ProviderID:   provider.ID,
		ProviderName: provider.Name,
		ModelCode:    provider.DefaultModel,
		Analysis:     analysis,
		UsedLogLines: len(lines),
		GeneratedAt:  time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}

func (s *LogAnalysisService) AnalyzeK8sPod(c *gin.Context, req *aiopsmodel.K8sPodAnalyzeRequest) (*aiopsmodel.K8sPodAnalyzeResult, error) {
	if c == nil || c.Request == nil {
		return s.analyzeK8sPodWithReporter(context.Background(), req, nil)
	}
	return s.analyzeK8sPodWithReporter(c.Request.Context(), req, nil)
}

func (s *LogAnalysisService) AnalyzeK8sIngress(c *gin.Context, req *aiopsmodel.K8sIngressAnalyzeRequest) (*aiopsmodel.K8sIngressAnalyzeResult, error) {
	ctx := context.Background()
	if c != nil && c.Request != nil {
		ctx = c.Request.Context()
	}
	return s.analyzeK8sIngressWithReporter(ctx, req, nil)
}

func (s *LogAnalysisService) StreamK8sIngressAnalyze(ctx context.Context, req *aiopsmodel.K8sIngressAnalyzeRequest, emit func(aiopsmodel.K8sIngressAnalyzeProgressEvent) error) error {
	report := func(event aiopsmodel.K8sIngressAnalyzeProgressEvent) {
		if emit != nil {
			_ = emit(event)
		}
	}
	_, err := s.analyzeK8sIngressWithReporter(ctx, req, report)
	if err != nil {
		event := aiopsmodel.K8sIngressAnalyzeProgressEvent{
			Type:      "error",
			Step:      "error",
			Status:    "failed",
			Progress:  100,
			Message:   "分析失败",
			Error:     err.Error(),
			Timestamp: time.Now().Format("2006-01-02 15:04:05"),
		}
		if emit != nil {
			return emit(event)
		}
		return err
	}
	return nil
}

func (s *LogAnalysisService) analyzeK8sIngressWithReporter(ctx context.Context, req *aiopsmodel.K8sIngressAnalyzeRequest, report k8sIngressAnalyzeReportFunc) (*aiopsmodel.K8sIngressAnalyzeResult, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	reportK8sIngressAnalyzeProgress(report, "started", "start", 3, "开始分析 Ingress")
	req.Namespace = strings.TrimSpace(req.Namespace)
	req.Ingress = strings.TrimSpace(req.Ingress)
	req.Path = strings.TrimSpace(req.Path)
	req.Host, req.Path = normalizeIngressAnalyzeHostAndPath(req.Host, req.Path)
	req.TailLines = normalizePositiveLimit(req.TailLines, defaultLogTailLines, maxLogTailLines)
	req.Limit = normalizePositiveLimit(req.Limit, 120, maxLogLimit)
	if req.EventLimit <= 0 {
		req.EventLimit = 50
	}
	if req.EventLimit > 200 {
		req.EventLimit = 200
	}
	includeLogs := boolValue(req.IncludeLogs, true)
	includeEvents := boolValue(req.IncludeEvents, true)
	includeBackend := boolValue(req.IncludeBackend, true)
	includeHTTPProbe := boolValue(req.IncludeHTTPProbe, true)

	reportK8sIngressAnalyzeProgress(report, "step", "connect_cluster", 8, "连接 Kubernetes 集群")
	clientset, cluster, _, err := s.createK8sClientWithConfig(req.ClusterID)
	if err != nil {
		return nil, err
	}
	_ = cluster

	customParser, err := NewCustomLogParser(req.FormatYAML)
	if err != nil {
		return nil, err
	}

	evidence, selectedLogs, selectedEvents, err := s.collectK8sIngressEvidence(ctx, clientset, req, customParser, includeBackend, includeLogs, includeEvents, includeHTTPProbe, report)
	if err != nil {
		return nil, err
	}
	reportK8sIngressAnalyzeProgress(report, "step", "ai_analysis", 90, "调用 AI 生成诊断结论")
	prompt := buildK8sIngressAnalysisPrompt(req.Prompt, evidence, selectedLogs, selectedEvents)
	provider, apiKey, err := s.resolveLLMProvider(req.ProviderID)
	if err != nil {
		return nil, err
	}
	timeout := time.Duration(provider.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	llmCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	analysis, err := callLLMForLogAnalysis(llmCtx, provider, apiKey, prompt)
	if err != nil {
		return nil, err
	}
	summary, remediation := extractK8sIngressAnalysisSummaryAndRemediation(analysis, evidence)
	status := inferK8sIngressDiagnosisStatus(evidence)
	severity := inferK8sIngressSeverity(evidence)
	result := &aiopsmodel.K8sIngressAnalyzeResult{
		ProviderID:     provider.ID,
		ProviderName:   provider.Name,
		ModelCode:      provider.DefaultModel,
		Analysis:       analysis,
		Status:         localizeDiagnosisStatus(status),
		Severity:       localizeDiagnosisSeverity(severity),
		Summary:        summary,
		Remediation:    remediation,
		Conclusion:     firstNonEmptyLine(analysis),
		Evidence:       evidence,
		UsedLogLines:   len(selectedLogs),
		UsedEventCount: len(selectedEvents),
		GeneratedAt:    time.Now().Format("2006-01-02 15:04:05"),
	}
	reportK8sIngressAnalyzeResult(report, result)
	return result, nil
}

func (s *LogAnalysisService) analyzeK8sPodWithReporter(ctx context.Context, req *aiopsmodel.K8sPodAnalyzeRequest, report k8sPodAnalyzeReportFunc) (*aiopsmodel.K8sPodAnalyzeResult, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	reportK8sPodAnalyzeProgress(report, "started", "start", 3, "开始分析 Pod", "")
	req.Namespace = strings.TrimSpace(req.Namespace)
	req.Pod = strings.TrimSpace(req.Pod)
	req.WorkloadType = strings.TrimSpace(req.WorkloadType)
	req.WorkloadName = strings.TrimSpace(req.WorkloadName)
	req.Container = trimK8sContainerDisplayName(req.Container)
	req.TailLines = normalizePositiveLimit(req.TailLines, defaultLogTailLines, maxLogTailLines)
	req.Limit = normalizePositiveLimit(req.Limit, 200, maxLogLimit)
	if req.EventLimit <= 0 {
		req.EventLimit = 50
	}
	if req.EventLimit > 200 {
		req.EventLimit = 200
	}
	if req.ContextLines <= 0 {
		req.ContextLines = 5
	}
	if req.ContextLines > 20 {
		req.ContextLines = 20
	}
	if req.Namespace == "" {
		return nil, errors.New("namespace is required")
	}
	if req.Pod == "" && (req.WorkloadType == "" || req.WorkloadName == "") {
		return nil, errors.New("pod or workloadType/workloadName is required")
	}
	includePrevious := boolValue(req.IncludePrevious, true)
	includeEvents := boolValue(req.IncludeEvents, true)
	includeMetrics := boolValue(req.IncludeMetrics, true)
	includeNode := boolValue(req.IncludeNode, true)
	includeQuota := boolValue(req.IncludeQuota, true)
	includeStorage := boolValue(req.IncludeStorage, true)
	includeProbe := boolValue(req.IncludeProbe, true)

	stepStartedAt := time.Now()
	reportK8sPodAnalyzeProgress(report, "step", "connect_cluster", 8, "连接 Kubernetes 集群", "")
	clientset, cluster, restConfig, err := s.createK8sClientWithConfig(req.ClusterID)
	if err != nil {
		reportK8sPodAnalyzeStepDone(report, "connect_cluster", 8, "failed", "连接 Kubernetes 集群失败："+err.Error(), "", stepStartedAt)
		return nil, err
	}
	reportK8sPodAnalyzeStepDone(report, "connect_cluster", 12, "succeeded", fmt.Sprintf("集群连接成功：clusterId=%d，name=%s", cluster.ID, cluster.Name), "", stepStartedAt)

	var workloadInfo *aiopsmodel.K8sPodAnalyzeWorkloadInfo
	if req.Pod == "" {
		stepStartedAt = time.Now()
		reportK8sPodAnalyzeProgress(report, "step", "workload_pods", 14, fmt.Sprintf("解析工作负载 %s/%s 下的 Pod", req.WorkloadType, req.WorkloadName), "")
		selectedPod, info, err := resolveK8sAnalyzeWorkloadPod(ctx, clientset, req.Namespace, req.WorkloadType, req.WorkloadName)
		if err != nil {
			reportK8sPodAnalyzeStepDone(report, "workload_pods", 14, "failed", "解析工作负载 Pod 失败："+err.Error(), "", stepStartedAt)
			return nil, err
		}
		req.Pod = selectedPod
		workloadInfo = info
		reportK8sPodAnalyzeStepDone(report, "workload_pods", 15, "succeeded", fmt.Sprintf("已选择 Pod %s 作为分析样本：%s", selectedPod, firstNonEmptyString(info.SelectedReason, "代表性 Pod")), "", stepStartedAt)
	}

	stepStartedAt = time.Now()
	reportK8sPodAnalyzeProgress(report, "step", "pod_status", 16, "读取 Pod 状态与容器状态", "")
	pod, err := clientset.CoreV1().Pods(req.Namespace).Get(ctx, req.Pod, metav1.GetOptions{})
	if err != nil {
		reportK8sPodAnalyzeStepDone(report, "pod_status", 16, "failed", "读取 Pod 状态失败："+err.Error(), "", stepStartedAt)
		return nil, fmt.Errorf("get k8s pod failed: %v", err)
	}

	containerNames, err := resolveK8sPodAnalyzeContainersFromPod(pod, req.Container, req.AllContainers)
	if err != nil {
		reportK8sPodAnalyzeStepDone(report, "pod_status", 18, "failed", "定位容器失败："+err.Error(), string(pod.Status.Phase), stepStartedAt)
		return nil, err
	}
	containerName := containerNames[0]
	if req.AllContainers && len(containerNames) > 1 {
		containerName = "all"
	}
	req.Container = containerName
	reportK8sPodAnalyzeProgress(report, "step", "pod_status", 22, fmt.Sprintf("已定位容器 %s", strings.Join(containerNames, ",")), "")
	reportK8sPodAnalyzeStepDone(report, "pod_status", 24, "succeeded", buildK8sPodStatusProgressMessage(pod, containerNames), string(pod.Status.Phase), stepStartedAt)

	customParser, err := NewCustomLogParser(req.FormatYAML)
	if err != nil {
		return nil, err
	}

	collectionErrors := make([]string, 0, 4)
	stepStartedAt = time.Now()
	reportK8sPodAnalyzeProgress(report, "step", "logs_current", 32, fmt.Sprintf("采集当前日志，最近 %d 行", req.TailLines), "")
	var mainEntries []aiopsmodel.LogEntry
	for _, targetContainer := range containerNames {
		mainLines, err := s.readK8sTailLines(ctx, clientset, &aiopsmodel.K8sLogQueryRequest{
			ClusterID:  req.ClusterID,
			Namespace:  req.Namespace,
			Pod:        req.Pod,
			Container:  targetContainer,
			TailLines:  req.TailLines,
			Previous:   false,
			Timestamps: true,
		})
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			collectionErrors = append(collectionErrors, fmt.Sprintf("current logs %s: %v", targetContainer, err))
			continue
		}
		source := aiopsmodel.LogSource{
			Type:      aiopsmodel.LogSourceTypeK8s,
			ClusterID: cluster.ID,
			Namespace: req.Namespace,
			Pod:       req.Pod,
			Container: targetContainer,
		}
		mainEntries = append(mainEntries, parseRawLinesForSource(s.parser, mainLines, source, customParser)...)
	}
	if len(mainEntries) == 0 && len(collectionErrors) > 0 {
		reportK8sPodAnalyzeProgress(report, "step", "logs_current", 42, "当前日志不可用，继续基于 Pod 状态和事件分析", string(pod.Status.Phase))
		reportK8sPodAnalyzeStepDone(report, "logs_current", 42, "failed", fmt.Sprintf("当前日志采集失败：目标容器 %d 个，错误 %d 个", len(containerNames), len(collectionErrors)), string(pod.Status.Phase), stepStartedAt)
	} else {
		reportK8sPodAnalyzeProgress(report, "step", "logs_current", 42, fmt.Sprintf("当前日志采集完成，共 %d 行", len(mainEntries)), "")
		stepStatus := "succeeded"
		if len(collectionErrors) > 0 {
			stepStatus = "partial"
		}
		reportK8sPodAnalyzeStepDone(report, "logs_current", 42, stepStatus, fmt.Sprintf("当前日志采集完成：容器 %d 个，日志 %d 行，采集错误 %d 个", len(containerNames), len(mainEntries), len(collectionErrors)), string(pod.Status.Phase), stepStartedAt)
	}

	var previousEntries []aiopsmodel.LogEntry
	previousTargets := make([]string, 0, len(containerNames))
	for _, targetContainer := range containerNames {
		if shouldCollectPreviousLogs(pod, targetContainer) {
			previousTargets = append(previousTargets, targetContainer)
		}
	}
	if includePrevious && len(previousTargets) > 0 {
		stepStartedAt = time.Now()
		reportK8sPodAnalyzeProgress(report, "step", "logs_previous", 48, "检测到容器重启，采集上一次退出前日志", "")
		beforeErrors := len(collectionErrors)
		for _, targetContainer := range previousTargets {
			previousLines, previousErr := s.readK8sTailLines(ctx, clientset, &aiopsmodel.K8sLogQueryRequest{
				ClusterID:  req.ClusterID,
				Namespace:  req.Namespace,
				Pod:        req.Pod,
				Container:  targetContainer,
				TailLines:  req.TailLines,
				Previous:   true,
				Timestamps: true,
			})
			if previousErr != nil {
				collectionErrors = append(collectionErrors, fmt.Sprintf("previous logs %s: %v", targetContainer, previousErr))
				continue
			}
			source := aiopsmodel.LogSource{
				Type:      aiopsmodel.LogSourceTypeK8s,
				ClusterID: cluster.ID,
				Namespace: req.Namespace,
				Pod:       req.Pod,
				Container: targetContainer,
			}
			previousEntries = append(previousEntries, parseRawLinesForSource(s.parser, previousLines, source, customParser)...)
		}
		reportK8sPodAnalyzeProgress(report, "step", "logs_previous", 54, fmt.Sprintf("previous 日志采集完成，共 %d 行", len(previousEntries)), "")
		stepStatus := "succeeded"
		if len(collectionErrors) > beforeErrors {
			if len(previousEntries) == 0 {
				stepStatus = "failed"
			} else {
				stepStatus = "partial"
			}
		}
		reportK8sPodAnalyzeStepDone(report, "logs_previous", 54, stepStatus, fmt.Sprintf("previous 日志采集完成：目标容器 %d 个，日志 %d 行，采集错误 %d 个", len(previousTargets), len(previousEntries), len(collectionErrors)-beforeErrors), string(pod.Status.Phase), stepStartedAt)
	} else if includePrevious {
		reportK8sPodAnalyzeProgress(report, "step", "logs_previous", 54, "未发现重启记录，跳过 previous 日志", "")
		reportK8sPodAnalyzeStepDone(report, "logs_previous", 54, "skipped", "容器无重启记录，previous 日志无需采集", string(pod.Status.Phase), time.Now())
	} else {
		reportK8sPodAnalyzeStepDone(report, "logs_previous", 54, "skipped", "请求未开启 previous 日志采集", string(pod.Status.Phase), time.Now())
	}

	var events []corev1.Event
	if includeEvents {
		stepStartedAt = time.Now()
		reportK8sPodAnalyzeProgress(report, "step", "events", 62, "采集并排序 Pod 事件", "")
		events, err = listPodEvents(ctx, clientset, req.Namespace, req.Pod, req.EventLimit)
		if err != nil {
			collectionErrors = append(collectionErrors, "events: "+err.Error())
			reportK8sPodAnalyzeStepDone(report, "events", 68, "failed", "Pod 事件采集失败："+err.Error(), string(pod.Status.Phase), stepStartedAt)
		} else {
			reportK8sPodAnalyzeProgress(report, "step", "events", 68, fmt.Sprintf("事件采集完成，共 %d 条", len(events)), "")
			reportK8sPodAnalyzeStepDone(report, "events", 68, "succeeded", fmt.Sprintf("Pod 事件采集完成：事件 %d 条，Warning %d 条", len(events), countK8sWarningEvents(events)), string(pod.Status.Phase), stepStartedAt)
		}
	} else {
		reportK8sPodAnalyzeStepDone(report, "events", 68, "skipped", "请求未开启 Pod 事件采集", string(pod.Status.Phase), time.Now())
	}

	var nodeInfo *aiopsmodel.K8sPodAnalyzeNodeInfo
	if includeNode {
		stepStartedAt = time.Now()
		reportK8sPodAnalyzeProgress(report, "step", "node", 70, "采集 Node Ready、压力和污点信息", "")
		nodeInfo, err = collectK8sPodNodeInfo(ctx, clientset, pod)
		if err != nil {
			collectionErrors = append(collectionErrors, "node: "+err.Error())
			reportK8sPodAnalyzeStepDone(report, "node", 70, "failed", "Node 信息采集失败："+err.Error(), string(pod.Status.Phase), stepStartedAt)
		} else if nodeInfo == nil {
			reportK8sPodAnalyzeStepDone(report, "node", 70, "skipped", "Pod 未绑定 Node，跳过 Node 信息采集", string(pod.Status.Phase), stepStartedAt)
		} else {
			reportK8sPodAnalyzeStepDone(report, "node", 70, "succeeded", fmt.Sprintf("Node 信息采集完成：node=%s，Ready=%s，条件 %d 条", nodeInfo.Name, nodeInfo.Ready, len(nodeInfo.Conditions)), string(pod.Status.Phase), stepStartedAt)
		}
	} else {
		reportK8sPodAnalyzeStepDone(report, "node", 70, "skipped", "请求未开启 Node 信息采集", string(pod.Status.Phase), time.Now())
	}

	var quotaInfo []aiopsmodel.K8sPodAnalyzeQuotaInfo
	if includeQuota {
		stepStartedAt = time.Now()
		reportK8sPodAnalyzeProgress(report, "step", "quota", 71, "采集 Namespace ResourceQuota", "")
		quotaInfo, err = collectK8sPodQuotaInfo(ctx, clientset, req.Namespace)
		if err != nil {
			collectionErrors = append(collectionErrors, "quota: "+err.Error())
			reportK8sPodAnalyzeStepDone(report, "quota", 71, "failed", "ResourceQuota 采集失败："+err.Error(), string(pod.Status.Phase), stepStartedAt)
		} else {
			reportK8sPodAnalyzeStepDone(report, "quota", 71, "succeeded", fmt.Sprintf("ResourceQuota 采集完成：%d 条", len(quotaInfo)), string(pod.Status.Phase), stepStartedAt)
		}
	} else {
		reportK8sPodAnalyzeStepDone(report, "quota", 71, "skipped", "请求未开启 ResourceQuota 采集", string(pod.Status.Phase), time.Now())
	}

	var pvcInfo []aiopsmodel.K8sPodAnalyzePVCInfo
	if includeStorage {
		stepStartedAt = time.Now()
		reportK8sPodAnalyzeProgress(report, "step", "storage", 72, "采集 PVC/PV 绑定与挂载线索", "")
		pvcInfo, err = collectK8sPodPVCInfo(ctx, clientset, pod)
		if err != nil {
			collectionErrors = append(collectionErrors, "storage: "+err.Error())
			reportK8sPodAnalyzeStepDone(report, "storage", 72, "failed", "PVC/PV 信息采集失败："+err.Error(), string(pod.Status.Phase), stepStartedAt)
		} else {
			reportK8sPodAnalyzeStepDone(report, "storage", 72, "succeeded", fmt.Sprintf("PVC/PV 信息采集完成：PVC %d 个", len(pvcInfo)), string(pod.Status.Phase), stepStartedAt)
		}
	} else {
		reportK8sPodAnalyzeStepDone(report, "storage", 72, "skipped", "请求未开启 PVC/PV 采集", string(pod.Status.Phase), time.Now())
	}

	var probes []aiopsmodel.K8sPodAnalyzeProbeInfo
	if includeProbe {
		stepStartedAt = time.Now()
		reportK8sPodAnalyzeProgress(report, "step", "probes", 73, "解析 readiness/liveness/startup 探针配置", "")
		probes = collectK8sPodProbeInfo(pod)
		reportK8sPodAnalyzeStepDone(report, "probes", 73, "succeeded", fmt.Sprintf("探针配置解析完成：%d 条", len(probes)), string(pod.Status.Phase), stepStartedAt)
	} else {
		reportK8sPodAnalyzeStepDone(report, "probes", 73, "skipped", "请求未开启探针解析", string(pod.Status.Phase), time.Now())
	}

	stepStartedAt = time.Now()
	reportK8sPodAnalyzeProgress(report, "step", "network_routes", 74, "检查 Pod 关联的 Service 与 Ingress 端口路由", "")
	manifests := make([]aiopsmodel.K8sPodAnalyzeManifestInfo, 0, 4)
	workloadManifest, workloadManifestErr := collectK8sPodWorkloadManifest(ctx, clientset, req, pod)
	if workloadManifestErr != nil {
		collectionErrors = append(collectionErrors, "workload manifest: "+workloadManifestErr.Error())
	} else if workloadManifest != nil {
		manifests = append(manifests, *workloadManifest)
	}
	services, ingressRoutes, networkManifests, networkErr := collectK8sPodNetworkEvidence(ctx, clientset, pod)
	manifests = append(manifests, networkManifests...)
	if networkErr != nil {
		collectionErrors = append(collectionErrors, "network routes: "+networkErr.Error())
		reportK8sPodAnalyzeStepDone(report, "network_routes", 74, "failed", "Service/Ingress 路由采集失败："+networkErr.Error(), string(pod.Status.Phase), stepStartedAt)
	} else {
		reportK8sPodAnalyzeStepDone(report, "network_routes", 74, "succeeded", fmt.Sprintf("配置证据采集完成：工作负载/Pod、Service、Ingress 清单共 %d 份", len(manifests)), string(pod.Status.Phase), stepStartedAt)
	}

	var podMetrics *metricsv1beta1.PodMetrics
	if includeMetrics {
		stepStartedAt = time.Now()
		reportK8sPodAnalyzeProgress(report, "step", "metrics", 74, "采集 Pod CPU/内存使用率", "")
		metricsClient, metricsErr := metricsclientset.NewForConfig(restConfig)
		if metricsErr != nil {
			collectionErrors = append(collectionErrors, "metrics client: "+metricsErr.Error())
			reportK8sPodAnalyzeStepDone(report, "metrics", 78, "failed", "Metrics Client 初始化失败："+metricsErr.Error(), string(pod.Status.Phase), stepStartedAt)
		} else {
			podMetrics, metricsErr = metricsClient.MetricsV1beta1().PodMetricses(req.Namespace).Get(ctx, req.Pod, metav1.GetOptions{})
			if metricsErr != nil {
				collectionErrors = append(collectionErrors, "metrics: "+metricsErr.Error())
				reportK8sPodAnalyzeStepDone(report, "metrics", 78, "failed", "Pod CPU/内存指标采集失败："+metricsErr.Error(), string(pod.Status.Phase), stepStartedAt)
			} else {
				reportK8sPodAnalyzeProgress(report, "step", "metrics", 78, "Pod CPU/内存使用率采集完成", "")
				reportK8sPodAnalyzeStepDone(report, "metrics", 78, "succeeded", fmt.Sprintf("Pod CPU/内存指标采集完成：容器指标 %d 条", len(podMetrics.Containers)), string(pod.Status.Phase), stepStartedAt)
			}
		}
	} else {
		reportK8sPodAnalyzeStepDone(report, "metrics", 78, "skipped", "请求未开启 CPU/内存指标采集", string(pod.Status.Phase), time.Now())
	}

	var metricsTrend *aiopsmodel.K8sPodAnalyzeMetricsTrendInfo
	if includeMetrics {
		stepStartedAt = time.Now()
		reportK8sPodAnalyzeProgress(report, "step", "metrics_trend", 80, "采集近 6 小时 CPU、内存 WSS 和网络趋势摘要", "")
		podNames := resolveK8sPodAnalyzeMetricPodNames(ctx, clientset, req.Namespace, pod, workloadInfo)
		metricsTrend, err = s.collectK8sPodMetricsTrend(ctx, cluster, req.Namespace, podNames, req.Container)
		if err != nil {
			collectionErrors = append(collectionErrors, "metrics trend: "+err.Error())
			reportK8sPodAnalyzeStepDone(report, "metrics_trend", 82, "failed", "监控趋势采集失败："+err.Error(), string(pod.Status.Phase), stepStartedAt)
		} else if metricsTrend == nil || !metricsTrend.Available {
			reportK8sPodAnalyzeStepDone(report, "metrics_trend", 82, "skipped", "监控趋势无可用数据", string(pod.Status.Phase), stepStartedAt)
		} else {
			reportK8sPodAnalyzeStepDone(report, "metrics_trend", 82, "succeeded", buildK8sPodMetricsTrendProgressMessage(metricsTrend), string(pod.Status.Phase), stepStartedAt)
		}
	} else {
		reportK8sPodAnalyzeStepDone(report, "metrics_trend", 82, "skipped", "请求未开启监控趋势采集", string(pod.Status.Phase), time.Now())
	}

	stepStartedAt = time.Now()
	reportK8sPodAnalyzeProgress(report, "step", "evidence", 84, "筛选错误日志、Warning 事件和资源热点", "")
	selectedMain := selectK8sEvidenceEntries(mainEntries, req.Limit, req.ContextLines)
	selectedPrevious := selectK8sEvidenceEntries(previousEntries, minInt(req.Limit, 120), req.ContextLines)
	selectedEvents := selectK8sEvidenceEvents(events, req.EventLimit)
	evidenceSummary := buildK8sPodEvidenceSummary(req.ClusterID, pod, podMetrics, containerName, mainEntries, selectedMain, selectedPrevious, selectedEvents, nodeInfo, quotaInfo, pvcInfo, probes, collectionErrors)
	evidenceSummary.Workload = workloadInfo
	evidenceSummary.Services = services
	evidenceSummary.IngressRoutes = ingressRoutes
	evidenceSummary.Manifests = manifests
	addK8sPodNetworkSuspectedReasons(&evidenceSummary)
	evidenceSummary.MetricsTrend = metricsTrend
	reportK8sPodAnalyzeStepDone(report, "evidence", 86, "succeeded", buildK8sPodEvidenceProgressMessage(evidenceSummary), string(pod.Status.Phase), stepStartedAt)

	stepStartedAt = time.Now()
	reportK8sPodAnalyzeProgress(report, "step", "ai_analysis", 90, "调用 AI 生成诊断结论", "")
	prompt := buildK8sPodAnalysisPrompt(req.Prompt, evidenceSummary, selectedMain, selectedPrevious, selectedEvents)
	provider, apiKey, err := s.resolveLLMProvider(req.ProviderID)
	if err != nil {
		return nil, err
	}

	timeout := time.Duration(provider.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	llmCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	analysis, err := callLLMForLogAnalysis(llmCtx, provider, apiKey, prompt)
	if err != nil {
		reportK8sPodAnalyzeStepDone(report, "ai_analysis", 90, "failed", "AI 诊断失败："+err.Error(), string(pod.Status.Phase), stepStartedAt)
		return nil, err
	}
	analysis = ensureK8sPodAnalysisIncludesLogFindings(analysis, evidenceSummary)
	reportK8sPodAnalyzeStepDone(report, "ai_analysis", 96, "succeeded", fmt.Sprintf("AI 诊断完成：provider=%s，model=%s", provider.Name, provider.DefaultModel), string(pod.Status.Phase), stepStartedAt)
	summary, remediation := extractK8sPodAnalysisSummaryAndRemediation(analysis, evidenceSummary)

	status := inferK8sPodDiagnosisStatus(evidenceSummary)
	severity := inferK8sPodSeverity(evidenceSummary)
	result := &aiopsmodel.K8sPodAnalyzeResult{
		ProviderID:     provider.ID,
		ProviderName:   provider.Name,
		ModelCode:      provider.DefaultModel,
		Analysis:       analysis,
		Status:         localizeDiagnosisStatus(status),
		Severity:       localizeDiagnosisSeverity(severity),
		Summary:        summary,
		Remediation:    remediation,
		Conclusion:     firstNonEmptyLine(analysis),
		Evidence:       evidenceSummary,
		UsedLogLines:   len(selectedMain) + len(selectedPrevious),
		UsedEventCount: len(selectedEvents),
		GeneratedAt:    time.Now().Format("2006-01-02 15:04:05"),
	}
	result = compactK8sPodAnalyzeResult(result)
	reportK8sPodAnalyzeResult(report, result)
	return result, nil
}

func (s *LogAnalysisService) StartK8sPodAnalyzeTask(req *aiopsmodel.K8sPodAnalyzeRequest) (*aiopsmodel.K8sPodAnalyzeTaskCreateResult, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	task := newK8sPodAnalyzeTask(uuid.NewString())
	sharedK8sPodAnalyzeTaskManager.Store(task)

	reqCopy := *req
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		result, err := NewLogAnalysisService().analyzeK8sPodWithReporter(ctx, &reqCopy, task.addEvent)
		if err != nil {
			task.fail(err)
			return
		}
		task.succeed(result)
	}()

	snapshot := task.snapshot(false)
	return &aiopsmodel.K8sPodAnalyzeTaskCreateResult{
		TaskID:    task.taskID,
		Status:    snapshot.Status,
		StreamURL: fmt.Sprintf("/api/v1/aiops/log/k8s/pod/analyze/tasks/%s/stream", task.taskID),
		ResultURL: fmt.Sprintf("/api/v1/aiops/log/k8s/pod/analyze/tasks/%s", task.taskID),
	}, nil
}

func (s *LogAnalysisService) GetK8sPodAnalyzeTask(taskID string) (*aiopsmodel.K8sPodAnalyzeTaskStatus, error) {
	task := sharedK8sPodAnalyzeTaskManager.Get(taskID)
	if task == nil {
		return nil, errors.New("pod analyze task not found")
	}
	return task.snapshot(true), nil
}

func (s *LogAnalysisService) StreamK8sPodAnalyzeTask(ctx context.Context, taskID string, emit func(aiopsmodel.K8sPodAnalyzeProgressEvent) error) error {
	task := sharedK8sPodAnalyzeTaskManager.Get(taskID)
	if task == nil {
		return errors.New("pod analyze task not found")
	}
	nextIndex := 0
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()

	for {
		events, terminal := task.eventsFrom(nextIndex)
		for _, event := range events {
			if err := emit(event); err != nil {
				return err
			}
			nextIndex++
		}
		if terminal && len(events) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (s *LogAnalysisService) buildLogQueryResult(source aiopsmodel.LogSource, lines []string, req *aiopsmodel.HostLogQueryRequest) (*aiopsmodel.LogQueryResult, error) {
	return s.buildLogQueryResultWithOptions(source, lines, &logQueryOptions{
		SessionID:  req.SessionID,
		Keyword:    req.Keyword,
		Regex:      req.Regex,
		Levels:     req.Levels,
		Limit:      req.Limit,
		FormatName: req.FormatName,
		FormatYAML: req.FormatYAML,
	})
}

func (s *LogAnalysisService) buildLogQueryResultWithOptions(source aiopsmodel.LogSource, lines []string, opts *logQueryOptions) (*aiopsmodel.LogQueryResult, error) {
	filter, err := newLogFilter(opts)
	if err != nil {
		return nil, err
	}
	customParser, err := NewCustomLogParser(opts.FormatYAML)
	if err != nil {
		return nil, err
	}
	engine := s.ensureSessionEngine(opts.SessionID, source, opts.FormatName, opts.FormatYAML)
	entries := make([]aiopsmodel.LogEntry, 0, minInt(len(lines), opts.Limit))
	matched := 0
	for i, line := range lines {
		entry := s.parser.Parse(line, i+1, source, customParser)
		engine.Ingest(entry)
		if !filter.Match(entry) {
			continue
		}
		matched++
		if len(entries) < opts.Limit {
			entries = append(entries, entry)
		}
	}
	status := engine.Status()
	snapshotEntries, snapshotSummary := engine.Snapshot(opts.Limit)
	errorSamples := collectErrorSamples(entries, 10)
	if len(errorSamples) == 0 {
		errorSamples = snapshotSummary.ErrorSamples
	}
	return &aiopsmodel.LogQueryResult{
		SessionID: opts.SessionID,
		Source:    source,
		Entries:   entries,
		Summary: aiopsmodel.LogSummary{
			Total:        len(lines),
			Matched:      matched,
			LevelCounts:  cloneIntMap(snapshotSummary.LevelCounts),
			TopKeywords:  snapshotSummary.TopKeywords,
			TopPatterns:  snapshotSummary.TopPatterns,
			ErrorSamples: errorSamples,
		},
		Status:    status,
		Truncated: matched > len(entries) || len(snapshotEntries) > len(entries),
	}, nil
}

func (s *LogAnalysisService) ensureSessionEngine(sessionID string, source aiopsmodel.LogSource, formatName, formatYAML string) *LogAnalysisEngine {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		sessionID = buildDefaultLogSessionID(source)
	}
	formatName = strings.TrimSpace(formatName)
	if formatName == "" {
		formatName = "auto"
	}
	return s.sessions.GetOrCreate(sessionID, source, formatName, strings.TrimSpace(formatYAML) != "")
}

func buildDefaultLogSessionID(source aiopsmodel.LogSource) string {
	base := string(source.Type)
	if source.HostID > 0 {
		base += fmt.Sprintf(":host:%d", source.HostID)
	}
	if source.ClusterID > 0 {
		base += fmt.Sprintf(":cluster:%d", source.ClusterID)
	}
	if source.Namespace != "" {
		base += ":ns:" + strings.ReplaceAll(source.Namespace, "/", "_")
	}
	if source.Pod != "" {
		base += ":pod:" + strings.ReplaceAll(source.Pod, "/", "_")
	}
	if source.Container != "" {
		base += ":container:" + strings.ReplaceAll(source.Container, "/", "_")
	}
	if source.Path != "" {
		base += ":" + strings.ReplaceAll(source.Path, "/", "_")
	}
	return base
}

func NewLogPatternClusterer() *LogPatternClusterer {
	return &LogPatternClusterer{
		clusters: make([]*logPatternCluster, 0, 128),
	}
}

func (c *LogPatternClusterer) Add(entry aiopsmodel.LogEntry) {
	message := strings.TrimSpace(entry.Message)
	if message == "" {
		message = strings.TrimSpace(entry.Raw)
	}
	if message == "" {
		return
	}

	tokens := tokenizePattern(message)
	if len(tokens) == 0 {
		return
	}
	normalizedTokens := normalizePatternTokens(tokens)

	bestIndex := -1
	bestScore := 0.0
	for i, cluster := range c.clusters {
		existingTokens := strings.Fields(cluster.Template)
		score := comparePatternTokens(normalizedTokens, existingTokens)
		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}

	if bestIndex >= 0 && bestScore >= 0.55 {
		cluster := c.clusters[bestIndex]
		cluster.Count++
		if cluster.Sample == "" {
			cluster.Sample = truncateForPrompt(message, 300)
		}
		cluster.Template = mergePatternTemplates(cluster.Template, normalizedTokens)
		return
	}

	c.clusters = append(c.clusters, &logPatternCluster{
		Template: strings.Join(normalizedTokens, " "),
		Sample:   truncateForPrompt(message, 300),
		Count:    1,
	})
}

func (c *LogPatternClusterer) TopPatterns(limit int) []aiopsmodel.PatternClusterItem {
	total := 0
	for _, cluster := range c.clusters {
		total += cluster.Count
	}
	items := make([]aiopsmodel.PatternClusterItem, 0, len(c.clusters))
	for _, cluster := range c.clusters {
		if strings.TrimSpace(cluster.Template) == "" {
			continue
		}
		percentage := 0.0
		if total > 0 {
			percentage = float64(cluster.Count) * 100.0 / float64(total)
		}
		items = append(items, aiopsmodel.PatternClusterItem{
			Pattern:    cluster.Template,
			Count:      cluster.Count,
			Percentage: percentage,
			Sample:     cluster.Sample,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Pattern < items[j].Pattern
		}
		return items[i].Count > items[j].Count
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func tokenizePattern(value string) []string {
	return strings.Fields(strings.TrimSpace(value))
}

func normalizePatternTokens(tokens []string) []string {
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		normalized := token
		for _, item := range patternRegexes {
			normalized = item.re.ReplaceAllString(normalized, item.repl)
		}
		if normalized == "" {
			continue
		}
		result = append(result, normalized)
	}
	return result
}

func comparePatternTokens(left, right []string) float64 {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	maxLen := len(left)
	if len(right) > maxLen {
		maxLen = len(right)
	}
	minLen := len(left)
	if len(right) < minLen {
		minLen = len(right)
	}
	match := 0
	for i := 0; i < minLen; i++ {
		if left[i] == right[i] {
			match++
		}
	}
	return float64(match) / float64(maxLen)
}

func mergePatternTemplates(existing string, incoming []string) string {
	existingTokens := strings.Fields(existing)
	maxLen := len(existingTokens)
	if len(incoming) > maxLen {
		maxLen = len(incoming)
	}
	merged := make([]string, 0, maxLen)
	for i := 0; i < maxLen; i++ {
		left := "***"
		right := "***"
		if i < len(existingTokens) {
			left = existingTokens[i]
		}
		if i < len(incoming) {
			right = incoming[i]
		}
		if left == right {
			merged = append(merged, left)
			continue
		}
		merged = append(merged, "***")
	}
	return strings.Join(merged, " ")
}

func NewLogAnalysisEngine(sessionID string, source aiopsmodel.LogSource, format string, customFormat bool, bufferSize int) *LogAnalysisEngine {
	if bufferSize <= 0 {
		bufferSize = defaultEngineBufferSize
	}
	if bufferSize > maxEngineBufferSize {
		bufferSize = maxEngineBufferSize
	}
	now := time.Now()
	return &LogAnalysisEngine{
		sessionID:    sessionID,
		source:       source,
		format:       strings.TrimSpace(format),
		customFormat: customFormat,
		startedAt:    now,
		lastSeen:     now,
		bufferSize:   bufferSize,
		entries:      make([]aiopsmodel.LogEntry, 0, bufferSize),
		levelCounts:  make(map[string]int),
		wordCounts:   make(map[string]int),
		streams:      make(map[string]*logSessionStream),
		clusterer:    NewLogPatternClusterer(),
	}
}

func (e *LogAnalysisEngine) Ingest(entry aiopsmodel.LogEntry) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.entries = append(e.entries, entry)
	if len(e.entries) > e.bufferSize {
		e.entries = e.entries[len(e.entries)-e.bufferSize:]
	}
	e.totalLogs++
	e.totalBytes += int64(len(entry.Raw))
	e.lastSeen = time.Now()

	level := normalizeLogLevel(entry.Level)
	if level == "" {
		level = "UNKNOWN"
	}
	e.levelCounts[level]++

	for _, item := range extractKeywordTokens(entry.Raw) {
		e.wordCounts[item]++
	}

	e.clusterer.Add(entry)
	e.updateStream(entry)
}

func (e *LogAnalysisEngine) RegisterStream(name, source string, active bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := source + ":" + name
	stream, ok := e.streams[key]
	if !ok {
		stream = &logSessionStream{
			Name:   name,
			Source: source,
		}
		e.streams[key] = stream
	}
	stream.Active = active
	stream.LastSeen = time.Now()
}

func (e *LogAnalysisEngine) MarkStreamInactive(name, source string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := source + ":" + name
	if stream, ok := e.streams[key]; ok {
		stream.Active = false
		stream.LastSeen = time.Now()
	}
}

func (e *LogAnalysisEngine) updateStream(entry aiopsmodel.LogEntry) {
	name := entry.Source.Path
	source := string(entry.Source.Type)
	if name == "" {
		name = entry.Source.HostName
	}
	if name == "" {
		name = "default"
	}
	key := source + ":" + name
	stream, ok := e.streams[key]
	if !ok {
		stream = &logSessionStream{
			Name:   name,
			Source: source,
			Active: false,
		}
		e.streams[key] = stream
	}
	stream.LogCount++
	stream.LastSeen = time.Now()
}

func (e *LogAnalysisEngine) Snapshot(limit int) ([]aiopsmodel.LogEntry, aiopsmodel.LogSummary) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if limit <= 0 || limit > len(e.entries) {
		limit = len(e.entries)
	}
	start := len(e.entries) - limit
	if start < 0 {
		start = 0
	}
	entries := make([]aiopsmodel.LogEntry, len(e.entries[start:]))
	copy(entries, e.entries[start:])

	return entries, aiopsmodel.LogSummary{
		Total:        len(e.entries),
		Matched:      len(e.entries),
		LevelCounts:  cloneIntMap(e.levelCounts),
		TopKeywords:  topCountItemsFromIntMap(e.wordCounts, 20),
		TopPatterns:  e.clusterer.TopPatterns(10),
		ErrorSamples: collectErrorSamples(entries, 10),
	}
}

func (e *LogAnalysisEngine) Status() *aiopsmodel.LogSessionStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	streams := make([]aiopsmodel.LogStreamStatus, 0, len(e.streams))
	for _, stream := range e.streams {
		streams = append(streams, aiopsmodel.LogStreamStatus{
			Name:     stream.Name,
			Source:   stream.Source,
			Active:   stream.Active,
			LogCount: stream.LogCount,
			LastSeen: stream.LastSeen.Format("2006-01-02 15:04:05"),
		})
	}
	sort.Slice(streams, func(i, j int) bool {
		return streams[i].LastSeen > streams[j].LastSeen
	})

	return &aiopsmodel.LogSessionStatus{
		SessionID:       e.sessionID,
		Source:          e.source,
		Format:          e.format,
		HasCustomFormat: e.customFormat,
		StartedAt:       e.startedAt.Format("2006-01-02 15:04:05"),
		LastSeen:        e.lastSeen.Format("2006-01-02 15:04:05"),
		TotalLogs:       e.totalLogs,
		TotalBytes:      e.totalBytes,
		BufferSize:      e.bufferSize,
		BufferUsed:      len(e.entries),
		LevelCounts:     cloneIntMap(e.levelCounts),
		TopKeywords:     topCountItemsFromIntMap(e.wordCounts, 20),
		TopPatterns:     e.clusterer.TopPatterns(10),
		Streams:         streams,
	}
}

func (e *LogAnalysisEngine) SessionListItem() aiopsmodel.LogSessionListItem {
	status := e.Status()
	active := false
	for _, stream := range status.Streams {
		if stream.Active {
			active = true
			break
		}
	}
	return aiopsmodel.LogSessionListItem{
		SessionID:   status.SessionID,
		Source:      status.Source,
		Format:      status.Format,
		StartedAt:   status.StartedAt,
		LastSeen:    status.LastSeen,
		TotalLogs:   status.TotalLogs,
		BufferUsed:  status.BufferUsed,
		Active:      active,
		StreamCount: len(status.Streams),
	}
}

func NewLogSessionManager() *LogSessionManager {
	return &LogSessionManager{
		sessions: make(map[string]*LogAnalysisEngine),
	}
}

func (m *LogSessionManager) Get(sessionID string) *LogAnalysisEngine {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[sessionID]
}

func (m *LogSessionManager) GetOrCreate(sessionID string, source aiopsmodel.LogSource, format string, customFormat bool) *LogAnalysisEngine {
	m.mu.Lock()
	defer m.mu.Unlock()
	if engine, ok := m.sessions[sessionID]; ok {
		return engine
	}
	engine := NewLogAnalysisEngine(sessionID, source, format, customFormat, defaultEngineBufferSize)
	m.sessions[sessionID] = engine
	return engine
}

func (m *LogSessionManager) List(activeOnly bool) []aiopsmodel.LogSessionListItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]aiopsmodel.LogSessionListItem, 0, len(m.sessions))
	for _, engine := range m.sessions {
		item := engine.SessionListItem()
		if activeOnly && !item.Active {
			continue
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastSeen > result[j].LastSeen
	})
	return result
}

func cloneIntMap(src map[string]int) map[string]int {
	result := make(map[string]int, len(src))
	for key, value := range src {
		result[key] = value
	}
	return result
}

func topCountItemsFromIntMap(counts map[string]int, limit int) []aiopsmodel.CountItem {
	items := make([]aiopsmodel.CountItem, 0, len(counts))
	for key, count := range counts {
		items = append(items, aiopsmodel.CountItem{Key: key, Count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Key < items[j].Key
		}
		return items[i].Count > items[j].Count
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func collectErrorSamples(entries []aiopsmodel.LogEntry, limit int) []string {
	result := make([]string, 0, limit)
	for _, entry := range entries {
		level := normalizeLogLevel(entry.Level)
		if level == "ERROR" || level == "FATAL" || level == "PANIC" {
			result = append(result, truncateForPrompt(entry.Raw, 500))
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}

func (s *LogAnalysisService) createK8sClient(clusterID uint) (*kubernetes.Clientset, *k8smodel.KubeCluster, error) {
	clientset, cluster, _, err := s.createK8sClientWithConfig(clusterID)
	return clientset, cluster, err
}

func (s *LogAnalysisService) createK8sClientWithConfig(clusterID uint) (*kubernetes.Clientset, *k8smodel.KubeCluster, *rest.Config, error) {
	cluster, err := k8sdao.NewKubeClusterDao(common.GetDB()).GetByID(clusterID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get k8s cluster failed: %v", err)
	}
	clientset, config, err := newK8sClientAndConfigFromKubeconfig(cluster.Credential, 12*time.Second)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("connect k8s cluster failed: %v", err)
	}
	return clientset, cluster, config, nil
}

func (s *LogAnalysisService) collectK8sPodMetricsTrend(ctx context.Context, cluster *k8smodel.KubeCluster, namespace string, podNames []string, containerName string) (*aiopsmodel.K8sPodAnalyzeMetricsTrendInfo, error) {
	info := &aiopsmodel.K8sPodAnalyzeMetricsTrendInfo{
		Available: false,
		Source:    "prometheus",
		Range:     "6h",
		Step:      "2m",
		Scope:     "pod",
		Pods:      compactStringSlice(podNames, 8, 120),
		PodCount:  len(podNames),
	}
	if cluster == nil {
		return info, errors.New("cluster is nil")
	}
	if cluster.MonitorDatasourceID == 0 {
		info.Warnings = append(info.Warnings, "当前集群未配置监控数据源")
		return info, nil
	}
	podNames = normalizeK8sPodAnalyzePodNames(podNames)
	if len(podNames) == 0 {
		info.Warnings = append(info.Warnings, "未定位到可查询的 Pod")
		return info, nil
	}
	info.Pods = compactStringSlice(podNames, 8, 120)
	info.PodCount = len(podNames)
	if len(podNames) > 1 {
		info.Scope = "workload"
	}

	ds, err := resolveK8sPodAnalyzeMonitorDatasource(cluster.MonitorDatasourceID)
	if err != nil {
		return info, err
	}
	info.DatasourceID = ds.ID
	info.DatasourceType = ds.DatasourceType
	info.DatasourceName = ds.Name
	info.Source = ds.DatasourceType

	promAPI, err := newK8sPodAnalyzePromAPI(*ds)
	if err != nil {
		return info, err
	}

	queryCtx, cancel := context.WithTimeout(ctx, 18*time.Second)
	defer cancel()

	start := time.Now().Add(-6 * time.Hour)
	end := time.Now()
	step := 2 * time.Minute
	queries := buildK8sPodAnalyzeMetricsTrendQueries(namespace, podNames, containerName)
	type queryDef struct {
		name      string
		query     string
		unit      string
		precision int
		nearLimit bool
		assign    func(*aiopsmodel.K8sPodAnalyzeMetricTrendStat)
	}
	defs := []queryDef{
		{name: "CPU使用率", query: queries["cpuUsageRate"], unit: "%", precision: 2, nearLimit: true, assign: func(stat *aiopsmodel.K8sPodAnalyzeMetricTrendStat) { info.CPUUsageRate = stat }},
		{name: "内存WSS使用率", query: queries["memoryWssUsageRate"], unit: "%", precision: 2, nearLimit: true, assign: func(stat *aiopsmodel.K8sPodAnalyzeMetricTrendStat) { info.MemoryWSSUsageRate = stat }},
		{name: "网络流入", query: queries["networkReceiveMiBps"], unit: "MiB/s", precision: 3, assign: func(stat *aiopsmodel.K8sPodAnalyzeMetricTrendStat) { info.NetworkReceiveMiBps = stat }},
		{name: "网络流出", query: queries["networkTransmitMiBps"], unit: "MiB/s", precision: 3, assign: func(stat *aiopsmodel.K8sPodAnalyzeMetricTrendStat) { info.NetworkTransmitMiBps = stat }},
	}
	for _, def := range defs {
		value, warnings, queryErr := promAPI.QueryRange(queryCtx, def.query, v1.Range{Start: start, End: end, Step: step})
		if queryErr != nil {
			info.Warnings = append(info.Warnings, fmt.Sprintf("%s 查询失败: %v", def.name, queryErr))
			continue
		}
		if len(warnings) > 0 {
			info.Warnings = append(info.Warnings, fmt.Sprintf("%s 查询告警: %s", def.name, strings.Join(warnings, "; ")))
		}
		stat := summarizeK8sPodAnalyzePromRange(value, def.unit, def.precision, def.nearLimit)
		if stat != nil {
			def.assign(stat)
			info.Available = true
		}
	}
	info.Warnings = compactStringSlice(info.Warnings, 4, 160)
	return info, nil
}

func resolveK8sPodAnalyzeMonitorDatasource(id uint) (*alertmodel.AlertDatasource, error) {
	var ds alertmodel.AlertDatasource
	if err := common.GetDB().First(&ds, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("监控数据源不存在")
		}
		return nil, fmt.Errorf("查询监控数据源失败: %w", err)
	}
	if ds.Enabled != alertmodel.RuleStatusEnabled {
		return nil, fmt.Errorf("监控数据源已禁用")
	}
	if !isK8sPodAnalyzePromQLDatasource(ds.DatasourceType) {
		return nil, fmt.Errorf("监控数据源仅支持 Prometheus/VictoriaMetrics")
	}
	if strings.TrimSpace(ds.URL) == "" {
		return nil, fmt.Errorf("监控数据源 URL 为空")
	}
	return &ds, nil
}

func isK8sPodAnalyzePromQLDatasource(datasourceType string) bool {
	switch datasourceType {
	case alertmodel.AlertDatasourcePrometheus, alertmodel.AlertDatasourceVictoriaMetrics, alertmodel.AlertDatasourceBuiltin:
		return true
	default:
		return false
	}
}

func newK8sPodAnalyzePromAPI(ds alertmodel.AlertDatasource) (v1.API, error) {
	roundTripper := http.DefaultTransport
	switch strings.ToLower(strings.TrimSpace(ds.AuthType)) {
	case "", "none":
	case "basic":
		roundTripper = &k8sPodAnalyzeBasicAuthRoundTripper{
			username: ds.Username,
			password: ds.Password,
			next:     roundTripper,
		}
	case "bearer":
		roundTripper = &k8sPodAnalyzeBearerAuthRoundTripper{
			token: ds.Token,
			next:  roundTripper,
		}
	default:
		return nil, fmt.Errorf("不支持的认证方式: %s", ds.AuthType)
	}
	client, err := api.NewClient(api.Config{
		Address:      normalizeK8sPodAnalyzePromQueryURL(ds.URL),
		RoundTripper: roundTripper,
	})
	if err != nil {
		return nil, err
	}
	return v1.NewAPI(client), nil
}

func normalizeK8sPodAnalyzePromQueryURL(rawURL string) string {
	url := strings.TrimRight(strings.TrimSpace(rawURL), "/")
	for _, suffix := range []string{"/api/v1/import/prometheus", "/api/v1/write", "/write"} {
		if strings.HasSuffix(url, suffix) {
			return strings.TrimRight(strings.TrimSuffix(url, suffix), "/")
		}
	}
	return url
}

type k8sPodAnalyzeBasicAuthRoundTripper struct {
	username string
	password string
	next     http.RoundTripper
}

func (rt *k8sPodAnalyzeBasicAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.SetBasicAuth(rt.username, rt.password)
	return rt.next.RoundTrip(req)
}

type k8sPodAnalyzeBearerAuthRoundTripper struct {
	token string
	next  http.RoundTripper
}

func (rt *k8sPodAnalyzeBearerAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	if strings.TrimSpace(rt.token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(rt.token))
	}
	return rt.next.RoundTrip(req)
}

func buildK8sPodAnalyzeMetricsTrendQueries(namespace string, podNames []string, containerName string) map[string]string {
	resourceSelector := buildK8sPodAnalyzePromResourceSelector(namespace, podNames, containerName)
	networkSelector := buildK8sPodAnalyzePromNetworkSelector(namespace, podNames)
	cpuCores := fmt.Sprintf(`sum by (pod) (irate(container_cpu_usage_seconds_total%s[2m]))`, resourceSelector)
	return map[string]string{
		"cpuUsageRate":         fmt.Sprintf(`%s / sum by (pod) ((container_spec_cpu_quota%s > 0) / 100000) * 100`, cpuCores, resourceSelector),
		"memoryWssUsageRate":   fmt.Sprintf(`sum by (pod) (container_memory_working_set_bytes%s) / sum by (pod) (container_spec_memory_limit_bytes%s > 0) * 100`, resourceSelector, resourceSelector),
		"networkReceiveMiBps":  fmt.Sprintf(`sum by (pod) (irate(container_network_receive_bytes_total%s[2m])) / 1024 / 1024`, networkSelector),
		"networkTransmitMiBps": fmt.Sprintf(`sum by (pod) (irate(container_network_transmit_bytes_total%s[2m])) / 1024 / 1024`, networkSelector),
	}
}

func buildK8sPodAnalyzePromResourceSelector(namespace string, podNames []string, containerName string) string {
	matchers := []string{
		k8sPodAnalyzeRegexMatcher("namespace", namespace),
		k8sPodAnalyzeRegexValuesMatcher("pod", podNames),
		`container!=""`,
		`container!="POD"`,
	}
	if containerName = strings.TrimSpace(containerName); containerName != "" && containerName != "all" {
		matchers = append(matchers, k8sPodAnalyzeRegexMatcher("container", containerName))
	}
	return "{" + strings.Join(matchers, ",") + "}"
}

func buildK8sPodAnalyzePromNetworkSelector(namespace string, podNames []string) string {
	matchers := []string{
		k8sPodAnalyzeRegexMatcher("namespace", namespace),
		k8sPodAnalyzeRegexValuesMatcher("pod", podNames),
		`interface!="lo"`,
	}
	return "{" + strings.Join(matchers, ",") + "}"
}

func k8sPodAnalyzeRegexMatcher(label, value string) string {
	return fmt.Sprintf(`%s=~"^%s$"`, label, regexp.QuoteMeta(strings.TrimSpace(value)))
}

func k8sPodAnalyzeRegexValuesMatcher(label string, values []string) string {
	parts := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		parts = append(parts, regexp.QuoteMeta(value))
	}
	if len(parts) == 0 {
		return fmt.Sprintf(`%s=~"^$"`, label)
	}
	return fmt.Sprintf(`%s=~"^(%s)$"`, label, strings.Join(parts, "|"))
}

type k8sPodAnalyzePromSample struct {
	timestamp time.Time
	value     float64
}

func summarizeK8sPodAnalyzePromRange(value prommodel.Value, unit string, precision int, nearLimit bool) *aiopsmodel.K8sPodAnalyzeMetricTrendStat {
	matrix, ok := value.(prommodel.Matrix)
	if !ok || len(matrix) == 0 {
		return nil
	}
	samples := make([]k8sPodAnalyzePromSample, 0)
	currentValues := make([]float64, 0, len(matrix))
	for _, stream := range matrix {
		if stream == nil || len(stream.Values) == 0 {
			continue
		}
		lastSet := false
		lastValue := 0.0
		lastTime := time.Time{}
		for _, sample := range stream.Values {
			rawValue := float64(sample.Value)
			if math.IsNaN(rawValue) || math.IsInf(rawValue, 0) {
				continue
			}
			ts := sample.Timestamp.Time()
			samples = append(samples, k8sPodAnalyzePromSample{timestamp: ts, value: rawValue})
			if !lastSet || ts.After(lastTime) {
				lastSet = true
				lastTime = ts
				lastValue = rawValue
			}
		}
		if lastSet {
			currentValues = append(currentValues, lastValue)
		}
	}
	if len(samples) == 0 {
		return nil
	}
	sort.Slice(samples, func(i, j int) bool {
		return samples[i].timestamp.Before(samples[j].timestamp)
	})

	minValue := samples[0].value
	peak := samples[0].value
	total := 0.0
	for _, sample := range samples {
		if sample.value < minValue {
			minValue = sample.value
		}
		if sample.value > peak {
			peak = sample.value
		}
		total += sample.value
	}
	current := averageFloat64(currentValues)
	if len(currentValues) == 0 {
		current = samples[len(samples)-1].value
	}
	average := total / float64(len(samples))
	trend, rising := inferK8sPodAnalyzeMetricTrend(samples)
	return &aiopsmodel.K8sPodAnalyzeMetricTrendStat{
		Unit:        unit,
		Current:     roundK8sPodAnalyzeMetric(current, precision),
		Peak:        roundK8sPodAnalyzeMetric(peak, precision),
		Average:     roundK8sPodAnalyzeMetric(average, precision),
		Min:         roundK8sPodAnalyzeMetric(minValue, precision),
		Trend:       trend,
		Rising:      rising,
		NearLimit:   nearLimit && (current >= 80 || peak >= 90),
		SampleCount: len(samples),
	}
}

func inferK8sPodAnalyzeMetricTrend(samples []k8sPodAnalyzePromSample) (string, bool) {
	if len(samples) < 6 {
		return "样本不足", false
	}
	third := len(samples) / 3
	headAvg := averageK8sPodAnalyzeSamples(samples[:third])
	tailAvg := averageK8sPodAnalyzeSamples(samples[len(samples)-third:])
	delta := tailAvg - headAvg
	threshold := math.Max(math.Abs(headAvg)*0.15, 1)
	if delta > threshold {
		return "持续升高", true
	}
	if -delta > threshold {
		return "下降", false
	}
	return "基本平稳", false
}

func averageK8sPodAnalyzeSamples(samples []k8sPodAnalyzePromSample) float64 {
	if len(samples) == 0 {
		return 0
	}
	total := 0.0
	for _, sample := range samples {
		total += sample.value
	}
	return total / float64(len(samples))
}

func averageFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func roundK8sPodAnalyzeMetric(value float64, precision int) float64 {
	factor := math.Pow(10, float64(precision))
	return math.Round(value*factor) / factor
}

func normalizeK8sPodAnalyzePodNames(podNames []string) []string {
	result := make([]string, 0, len(podNames))
	seen := map[string]struct{}{}
	for _, name := range podNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func resolveK8sPodAnalyzeMetricPodNames(ctx context.Context, clientset *kubernetes.Clientset, namespace string, pod *corev1.Pod, workloadInfo *aiopsmodel.K8sPodAnalyzeWorkloadInfo) []string {
	if pod == nil {
		return nil
	}
	if workloadInfo == nil {
		return []string{pod.Name}
	}
	selector := resolveK8sPodAnalyzeOwnerSelector(ctx, clientset, namespace, pod)
	if selector == "" {
		return []string{pod.Name}
	}
	list, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil || len(list.Items) == 0 {
		return []string{pod.Name}
	}
	names := make([]string, 0, len(list.Items))
	for i := range list.Items {
		if list.Items[i].Name != "" {
			names = append(names, list.Items[i].Name)
		}
	}
	return normalizeK8sPodAnalyzePodNames(names)
}

func resolveK8sPodAnalyzeOwnerSelector(ctx context.Context, clientset *kubernetes.Clientset, namespace string, pod *corev1.Pod) string {
	owner := firstControllerOwnerRef(pod.OwnerReferences)
	if owner == nil {
		return ""
	}
	switch owner.Kind {
	case "ReplicaSet":
		rs, err := clientset.AppsV1().ReplicaSets(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
		if err != nil {
			return ""
		}
		if deploymentOwner := firstControllerOwnerRef(rs.OwnerReferences); deploymentOwner != nil && deploymentOwner.Kind == "Deployment" {
			deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentOwner.Name, metav1.GetOptions{})
			if err == nil {
				return k8sAnalyzeSelectorString(deployment.Spec.Selector)
			}
		}
		return k8sAnalyzeSelectorString(rs.Spec.Selector)
	case "Deployment":
		deployment, err := clientset.AppsV1().Deployments(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
		if err != nil {
			return ""
		}
		return k8sAnalyzeSelectorString(deployment.Spec.Selector)
	case "StatefulSet":
		statefulSet, err := clientset.AppsV1().StatefulSets(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
		if err != nil {
			return ""
		}
		return k8sAnalyzeSelectorString(statefulSet.Spec.Selector)
	case "DaemonSet":
		daemonSet, err := clientset.AppsV1().DaemonSets(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
		if err != nil {
			return ""
		}
		return k8sAnalyzeSelectorString(daemonSet.Spec.Selector)
	case "Job":
		job, err := clientset.BatchV1().Jobs(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
		if err != nil {
			return ""
		}
		return k8sAnalyzeSelectorString(job.Spec.Selector)
	default:
		return ""
	}
}

func firstControllerOwnerRef(refs []metav1.OwnerReference) *metav1.OwnerReference {
	for i := range refs {
		if refs[i].Controller != nil && *refs[i].Controller {
			return &refs[i]
		}
	}
	if len(refs) > 0 {
		return &refs[0]
	}
	return nil
}

func k8sAnalyzeSelectorString(selector *metav1.LabelSelector) string {
	if selector == nil {
		return ""
	}
	parsed, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return ""
	}
	return parsed.String()
}

func (s *LogAnalysisService) createK8sStreamingClient(clusterID uint) (*kubernetes.Clientset, *k8smodel.KubeCluster, error) {
	cluster, err := k8sdao.NewKubeClusterDao(common.GetDB()).GetByID(clusterID)
	if err != nil {
		return nil, nil, fmt.Errorf("get k8s cluster failed: %v", err)
	}
	clientset, err := newK8sClientFromKubeconfig(cluster.Credential, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("connect k8s cluster failed: %v", err)
	}
	return clientset, cluster, nil
}

func newK8sClientFromKubeconfig(kubeconfig string, timeout time.Duration) (*kubernetes.Clientset, error) {
	clientset, _, err := newK8sClientAndConfigFromKubeconfig(kubeconfig, timeout)
	return clientset, err
}

func newK8sClientAndConfigFromKubeconfig(kubeconfig string, timeout time.Duration) (*kubernetes.Clientset, *rest.Config, error) {
	config, err := clientcmd.RESTConfigFromKubeConfig([]byte(kubeconfig))
	if err != nil {
		return nil, nil, err
	}
	config.Timeout = timeout
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	return clientset, config, nil
}

func (s *LogAnalysisService) readK8sTailLines(ctx context.Context, clientset *kubernetes.Clientset, req *aiopsmodel.K8sLogQueryRequest) ([]string, error) {
	logOptions := &corev1.PodLogOptions{
		Container:  req.Container,
		Follow:     false,
		Previous:   req.Previous,
		Timestamps: req.Timestamps,
	}
	tailLines := int64(req.TailLines)
	logOptions.TailLines = &tailLines

	stream, err := clientset.CoreV1().Pods(req.Namespace).GetLogs(req.Pod, logOptions).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("read k8s pod logs failed: %v", err)
	}
	defer stream.Close()

	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	lines := make([]string, 0, req.TailLines)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan k8s pod logs failed: %v", err)
	}
	return lines, nil
}

func resolveK8sPodContainer(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName, containerName string) (string, error) {
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get k8s pod failed: %v", err)
	}
	return resolveK8sPodContainerFromPod(pod, containerName)
}

func resolveK8sPodContainerFromPod(pod *corev1.Pod, containerName string) (string, error) {
	containerName = trimK8sContainerDisplayName(containerName)
	if containerName == "" {
		if len(pod.Spec.Containers) == 0 {
			return "", errors.New("pod has no regular containers")
		}
		return pod.Spec.Containers[0].Name, nil
	}
	for _, container := range pod.Spec.Containers {
		if container.Name == containerName {
			return container.Name, nil
		}
	}
	for _, container := range pod.Spec.InitContainers {
		if container.Name == containerName {
			return container.Name, nil
		}
	}
	for _, container := range pod.Spec.EphemeralContainers {
		if container.Name == containerName {
			return container.Name, nil
		}
	}
	return "", fmt.Errorf("container not found: %s", containerName)
}

func resolveK8sPodAnalyzeContainersFromPod(pod *corev1.Pod, containerName string, allContainers bool) ([]string, error) {
	if allContainers {
		names := make([]string, 0, len(pod.Spec.Containers))
		for _, container := range pod.Spec.Containers {
			names = append(names, container.Name)
		}
		if len(names) == 0 {
			return nil, errors.New("pod has no regular containers")
		}
		return names, nil
	}
	name, err := resolveK8sPodContainerFromPod(pod, containerName)
	if err != nil {
		return nil, err
	}
	return []string{name}, nil
}

func parseRawLinesForSource(parser *LogLineParser, lines []string, source aiopsmodel.LogSource, customParser *CustomLogParser) []aiopsmodel.LogEntry {
	entries := make([]aiopsmodel.LogEntry, 0, len(lines))
	for i, line := range lines {
		entries = append(entries, parser.Parse(line, i+1, source, customParser))
	}
	return entries
}

func shouldCollectPreviousLogs(pod *corev1.Pod, containerName string) bool {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == containerName {
			return status.RestartCount > 0 || status.LastTerminationState.Terminated != nil
		}
	}
	return false
}

func resolveK8sAnalyzeWorkloadPod(ctx context.Context, clientset *kubernetes.Clientset, namespace, workloadType, workloadName string) (string, *aiopsmodel.K8sPodAnalyzeWorkloadInfo, error) {
	namespace = strings.TrimSpace(namespace)
	workloadName = strings.TrimSpace(workloadName)
	normalizedType := normalizeK8sAnalyzeWorkloadType(workloadType)
	if namespace == "" || workloadName == "" || normalizedType == "" {
		return "", nil, errors.New("workload namespace, type and name are required")
	}

	info := &aiopsmodel.K8sPodAnalyzeWorkloadInfo{
		Type:      normalizedType,
		Name:      workloadName,
		Namespace: namespace,
	}
	var pods []corev1.Pod
	var err error

	switch normalizedType {
	case "Deployment":
		item, getErr := clientset.AppsV1().Deployments(namespace).Get(ctx, workloadName, metav1.GetOptions{})
		if getErr != nil {
			return "", nil, getErr
		}
		info.DesiredReplicas = int32Value(item.Spec.Replicas)
		info.CurrentReplicas = item.Status.Replicas
		info.ReadyReplicas = item.Status.ReadyReplicas
		info.AvailableReplicas = item.Status.AvailableReplicas
		pods, err = listPodsByLabelSelector(ctx, clientset, namespace, item.Spec.Selector)
	case "StatefulSet":
		item, getErr := clientset.AppsV1().StatefulSets(namespace).Get(ctx, workloadName, metav1.GetOptions{})
		if getErr != nil {
			return "", nil, getErr
		}
		info.DesiredReplicas = int32Value(item.Spec.Replicas)
		info.CurrentReplicas = item.Status.Replicas
		info.ReadyReplicas = item.Status.ReadyReplicas
		info.AvailableReplicas = item.Status.AvailableReplicas
		pods, err = listPodsByLabelSelector(ctx, clientset, namespace, item.Spec.Selector)
	case "DaemonSet":
		item, getErr := clientset.AppsV1().DaemonSets(namespace).Get(ctx, workloadName, metav1.GetOptions{})
		if getErr != nil {
			return "", nil, getErr
		}
		info.DesiredReplicas = item.Status.DesiredNumberScheduled
		info.CurrentReplicas = item.Status.CurrentNumberScheduled
		info.ReadyReplicas = item.Status.NumberReady
		info.AvailableReplicas = item.Status.NumberAvailable
		pods, err = listPodsByLabelSelector(ctx, clientset, namespace, item.Spec.Selector)
	case "Job":
		item, getErr := clientset.BatchV1().Jobs(namespace).Get(ctx, workloadName, metav1.GetOptions{})
		if getErr != nil {
			return "", nil, getErr
		}
		info.DesiredReplicas = int32Value(item.Spec.Completions)
		info.CurrentReplicas = item.Status.Active + item.Status.Succeeded + item.Status.Failed
		info.ReadyReplicas = item.Status.Succeeded
		info.AvailableReplicas = item.Status.Succeeded
		pods, err = listPodsForJob(ctx, clientset, namespace, item)
	case "CronJob":
		item, getErr := clientset.BatchV1beta1().CronJobs(namespace).Get(ctx, workloadName, metav1.GetOptions{})
		if getErr != nil {
			return "", nil, getErr
		}
		info.DesiredReplicas = int32Value(item.Spec.JobTemplate.Spec.Completions)
		info.CurrentReplicas = int32(len(item.Status.Active))
		info.ReadyReplicas = int32(len(item.Status.Active))
		info.AvailableReplicas = int32(len(item.Status.Active))
		pods, err = listPodsForCronJob(ctx, clientset, namespace, workloadName)
	default:
		return "", nil, fmt.Errorf("unsupported workload type: %s", workloadType)
	}
	if err != nil {
		return "", nil, err
	}
	if len(pods) == 0 {
		return "", nil, fmt.Errorf("workload %s/%s has no pods", normalizedType, workloadName)
	}

	selectedPod, reason := selectK8sAnalyzePodFromWorkload(pods)
	info.PodCount = len(pods)
	info.AbnormalPodCount = countK8sAnalyzeAbnormalPods(pods)
	info.SelectedPod = selectedPod
	info.SelectedReason = reason
	return selectedPod, info, nil
}

func normalizeK8sAnalyzeWorkloadType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "deployment", "deployments", "deploy":
		return "Deployment"
	case "statefulset", "statefulsets", "sts":
		return "StatefulSet"
	case "daemonset", "daemonsets", "ds":
		return "DaemonSet"
	case "job", "jobs":
		return "Job"
	case "cronjob", "cronjobs", "cj":
		return "CronJob"
	default:
		return ""
	}
}

func int32Value(value *int32) int32 {
	if value == nil {
		return 0
	}
	return *value
}

func listPodsByLabelSelector(ctx context.Context, clientset *kubernetes.Clientset, namespace string, selector *metav1.LabelSelector) ([]corev1.Pod, error) {
	if selector == nil {
		return nil, errors.New("workload selector is empty")
	}
	labelSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, err
	}
	list, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector.String()})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func listPodsForJob(ctx context.Context, clientset *kubernetes.Clientset, namespace string, job *batchv1.Job) ([]corev1.Pod, error) {
	if job == nil {
		return nil, errors.New("job is nil")
	}
	if job.Spec.Selector != nil {
		pods, err := listPodsByLabelSelector(ctx, clientset, namespace, job.Spec.Selector)
		if err == nil && len(pods) > 0 {
			return pods, nil
		}
	}
	list, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	pods := make([]corev1.Pod, 0)
	for _, pod := range list.Items {
		if podOwnedBy(pod.OwnerReferences, "Job", job.Name) {
			pods = append(pods, pod)
		}
	}
	return pods, nil
}

func listPodsForCronJob(ctx context.Context, clientset *kubernetes.Clientset, namespace, cronJobName string) ([]corev1.Pod, error) {
	jobs, err := clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	jobNames := map[string]struct{}{}
	for _, job := range jobs.Items {
		if podOwnedBy(job.OwnerReferences, "CronJob", cronJobName) {
			jobNames[job.Name] = struct{}{}
		}
	}
	if len(jobNames) == 0 {
		return nil, nil
	}
	list, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	pods := make([]corev1.Pod, 0)
	for _, pod := range list.Items {
		for _, owner := range pod.OwnerReferences {
			if owner.Kind != "Job" {
				continue
			}
			if _, ok := jobNames[owner.Name]; ok {
				pods = append(pods, pod)
				break
			}
		}
	}
	return pods, nil
}

func podOwnedBy(owners []metav1.OwnerReference, kind, name string) bool {
	for _, owner := range owners {
		if owner.Kind == kind && owner.Name == name {
			return true
		}
	}
	return false
}

func selectK8sAnalyzePodFromWorkload(pods []corev1.Pod) (string, string) {
	if len(pods) == 0 {
		return "", ""
	}
	type candidate struct {
		pod    corev1.Pod
		score  int
		reason string
	}
	items := make([]candidate, 0, len(pods))
	for _, pod := range pods {
		score, reason := scoreK8sAnalyzePod(pod)
		items = append(items, candidate{pod: pod, score: score, reason: reason})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].score != items[j].score {
			return items[i].score > items[j].score
		}
		return items[i].pod.CreationTimestamp.After(items[j].pod.CreationTimestamp.Time)
	})
	reason := items[0].reason
	if reason == "" {
		reason = "选择最新 Pod 作为代表样本"
	}
	return items[0].pod.Name, reason
}

func scoreK8sAnalyzePod(pod corev1.Pod) (int, string) {
	score := 0
	reasons := make([]string, 0, 4)
	if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded {
		score += 100
		reasons = append(reasons, "Pod phase="+string(pod.Status.Phase))
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status != corev1.ConditionTrue {
			score += 30
			reasons = append(reasons, "Pod Ready="+string(condition.Status))
			break
		}
	}
	for _, status := range pod.Status.ContainerStatuses {
		if !status.Ready {
			score += 20
		}
		if status.RestartCount > 0 {
			score += minInt(int(status.RestartCount)*10, 80)
			reasons = append(reasons, fmt.Sprintf("%s restarted %d times", status.Name, status.RestartCount))
		}
		if status.State.Waiting != nil {
			score += 80
			reasons = append(reasons, status.Name+" waiting: "+status.State.Waiting.Reason)
		}
		if status.State.Terminated != nil {
			score += 60
			reasons = append(reasons, status.Name+" terminated: "+status.State.Terminated.Reason)
		}
		if status.LastTerminationState.Terminated != nil {
			score += 40
			reasons = append(reasons, status.Name+" last terminated: "+status.LastTerminationState.Terminated.Reason)
		}
	}
	return score, strings.Join(dedupeNonEmptyStrings(reasons), "; ")
}

func countK8sAnalyzeAbnormalPods(pods []corev1.Pod) int {
	count := 0
	for _, pod := range pods {
		score, _ := scoreK8sAnalyzePod(pod)
		if score > 0 {
			count++
		}
	}
	return count
}

func dedupeNonEmptyStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func selectK8sEvidenceEntries(entries []aiopsmodel.LogEntry, limit, contextLines int) []aiopsmodel.LogEntry {
	if len(entries) == 0 || limit <= 0 {
		return nil
	}
	selected := make(map[int]struct{})
	addRange := func(index int) {
		start := index - contextLines
		if start < 0 {
			start = 0
		}
		end := index + contextLines
		if end >= len(entries) {
			end = len(entries) - 1
		}
		for i := start; i <= end; i++ {
			selected[i] = struct{}{}
		}
	}
	for i, entry := range entries {
		level := normalizeLogLevel(entry.Level)
		if level == "ERROR" || level == "FATAL" || level == "PANIC" || level == "WARN" || k8sEvidenceKeywordRegex.MatchString(entry.Raw) {
			addRange(i)
		}
	}
	result := make([]aiopsmodel.LogEntry, 0, minInt(len(selected), limit))
	for i := 0; i < len(entries) && len(result) < limit; i++ {
		if _, ok := selected[i]; ok {
			result = append(result, entries[i])
		}
	}
	if len(result) > 0 {
		return result
	}
	start := len(entries) - limit
	if start < 0 {
		start = 0
	}
	return append([]aiopsmodel.LogEntry(nil), entries[start:]...)
}

func listPodEvents(ctx context.Context, clientset *kubernetes.Clientset, namespace, podName string, limit int) ([]corev1.Event, error) {
	return listObjectEvents(ctx, clientset, namespace, "Pod", podName, limit)
}

func listObjectEvents(ctx context.Context, clientset *kubernetes.Clientset, namespace, kind, name string, limit int) ([]corev1.Event, error) {
	events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.kind=%s,involvedObject.name=%s", kind, name),
		Limit:         int64(limit),
	})
	if err != nil {
		return nil, err
	}
	return events.Items, nil
}

func selectK8sEvidenceEvents(events []corev1.Event, limit int) []corev1.Event {
	if len(events) == 0 || limit <= 0 {
		return nil
	}
	items := append([]corev1.Event(nil), events...)
	sort.SliceStable(items, func(i, j int) bool {
		leftScore := k8sEventScore(items[i])
		rightScore := k8sEventScore(items[j])
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		return k8sEventLastTime(items[i]).After(k8sEventLastTime(items[j]))
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func k8sEventScore(event corev1.Event) int {
	score := 0
	if strings.EqualFold(event.Type, corev1.EventTypeWarning) {
		score += 100
	}
	if _, ok := k8sWarningEventReasons[event.Reason]; ok {
		score += 50
	}
	if k8sEvidenceKeywordRegex.MatchString(event.Message + " " + event.Reason) {
		score += 30
	}
	if event.Count > 1 {
		score += minInt(int(event.Count), 20)
	}
	return score
}

func k8sEventLastTime(event corev1.Event) time.Time {
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	if !event.EventTime.IsZero() {
		return event.EventTime.Time
	}
	return event.CreationTimestamp.Time
}

func buildK8sPodEvidenceSummary(clusterID uint, pod *corev1.Pod, podMetrics *metricsv1beta1.PodMetrics, containerName string, allEntries, selectedEntries, previousEntries []aiopsmodel.LogEntry, selectedEvents []corev1.Event, nodeInfo *aiopsmodel.K8sPodAnalyzeNodeInfo, quotaInfo []aiopsmodel.K8sPodAnalyzeQuotaInfo, pvcInfo []aiopsmodel.K8sPodAnalyzePVCInfo, probes []aiopsmodel.K8sPodAnalyzeProbeInfo, collectionErrors []string) aiopsmodel.K8sPodAnalyzeEvidenceSummary {
	warningEvents := 0
	normalEvents := 0
	reasons := map[string]struct{}{}
	for _, event := range selectedEvents {
		if strings.EqualFold(event.Type, corev1.EventTypeWarning) {
			warningEvents++
		} else if strings.EqualFold(event.Type, corev1.EventTypeNormal) {
			normalEvents++
		}
		if strings.TrimSpace(event.Reason) != "" && (strings.EqualFold(event.Type, corev1.EventTypeWarning) || k8sEvidenceKeywordRegex.MatchString(event.Reason+" "+event.Message)) {
			reasons[event.Reason] = struct{}{}
		}
	}
	keywords := collectK8sMatchedKeywords(append(append([]aiopsmodel.LogEntry{}, selectedEntries...), previousEntries...))
	for _, entry := range append(append([]aiopsmodel.LogEntry{}, selectedEntries...), previousEntries...) {
		if level := normalizeLogLevel(entry.Level); level == "ERROR" || level == "FATAL" || level == "PANIC" || level == "WARN" {
			reasons[level] = struct{}{}
		}
	}
	podInfo := buildK8sPodAnalyzeInfo(clusterID, pod, podMetrics, containerName)
	hotspots := detectK8sResourceHotspots(podInfo)
	logFindings := collectK8sPodLogFindings(append(append([]aiopsmodel.LogEntry{}, selectedEntries...), previousEntries...), 5)
	for _, hotspot := range hotspots {
		reasons[hotspot] = struct{}{}
	}
	if nodeInfo != nil {
		for _, reason := range nodeInfo.SuspectedReasons {
			reasons[reason] = struct{}{}
		}
	}
	for _, quota := range quotaInfo {
		for _, reason := range quota.SuspectedReasons {
			reasons[reason] = struct{}{}
		}
	}
	for _, pvc := range pvcInfo {
		for _, reason := range pvc.SuspectedReasons {
			reasons[reason] = struct{}{}
		}
	}
	if len(probes) > 0 {
		for _, event := range selectedEvents {
			if strings.EqualFold(event.Reason, "Unhealthy") || strings.Contains(strings.ToLower(event.Message), "probe") {
				reasons["PROBE_UNHEALTHY"] = struct{}{}
				break
			}
		}
	}
	return aiopsmodel.K8sPodAnalyzeEvidenceSummary{
		Pod:              podInfo,
		LogLines:         len(allEntries),
		MatchedLogLines:  len(selectedEntries),
		RecentLogLines:   len(selectedEntries),
		PreviousLogLines: len(previousEntries),
		EventCount:       len(selectedEvents),
		WarningEvents:    warningEvents,
		NormalEvents:     normalEvents,
		MatchedKeywords:  sortedStringSet(keywords),
		SuspectedReasons: sortedStringSet(reasons),
		LogFindings:      logFindings,
		ResourceHotspots: hotspots,
		Node:             nodeInfo,
		Quotas:           quotaInfo,
		PVCs:             pvcInfo,
		Probes:           probes,
		CollectionErrors: append([]string(nil), collectionErrors...),
	}
}

func collectK8sPodLogFindings(entries []aiopsmodel.LogEntry, limit int) []aiopsmodel.K8sPodAnalyzeLogFinding {
	if len(entries) == 0 || limit <= 0 {
		return nil
	}
	findings := make([]aiopsmodel.K8sPodAnalyzeLogFinding, 0, limit)
	seen := map[string]struct{}{}
	for i, entry := range entries {
		kind := classifyK8sPodLogFinding(entry)
		if kind == "" {
			continue
		}
		message := summarizeK8sPodLogFinding(entry)
		key := kind + "\x00" + normalizePatternTokensForKey(message)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		findings = append(findings, aiopsmodel.K8sPodAnalyzeLogFinding{
			LineNo:    entry.LineNo,
			Timestamp: entry.Timestamp,
			Level:     normalizeLogLevel(entry.Level),
			Kind:      kind,
			Message:   message,
			Context:   buildK8sPodLogFindingContext(entries, i, 2),
		})
	}
	sort.SliceStable(findings, func(i, j int) bool {
		return k8sPodLogFindingPriority(findings[i].Kind) > k8sPodLogFindingPriority(findings[j].Kind)
	})
	if len(findings) > limit {
		findings = findings[:limit]
	}
	return findings
}

func classifyK8sPodLogFinding(entry aiopsmodel.LogEntry) string {
	raw := strings.TrimSpace(entry.Raw)
	if raw == "" {
		raw = strings.TrimSpace(entry.Message)
	}
	lower := strings.ToLower(raw)
	level := normalizeLogLevel(entry.Level)
	if (strings.Contains(lower, "debugger failed to attach") && strings.Contains(lower, "handshake failed")) ||
		(strings.Contains(lower, "jdwp-handshake") && strings.Contains(lower, "received >") && containsHTTPMethodToken(lower)) {
		return "HTTP_TO_JDWP_PORT"
	}
	if strings.Contains(lower, "no provider available") || strings.Contains(lower, "available invokers : empty") {
		return "DUBBO_NO_PROVIDER"
	}
	if strings.Contains(lower, "rpcException") || strings.Contains(lower, "rpcexception") {
		return "RPC_EXCEPTION"
	}
	if strings.Contains(lower, "bigdata_error_log") || level == "ERROR" || level == "FATAL" || level == "PANIC" {
		return "APPLICATION_ERROR"
	}
	if strings.Contains(lower, "exception") {
		return "EXCEPTION"
	}
	if strings.Contains(lower, "timeout") || strings.Contains(lower, "timed out") {
		return "TIMEOUT"
	}
	if strings.Contains(lower, "connection refused") || strings.Contains(lower, "refused") {
		return "CONNECTION_REFUSED"
	}
	if strings.Contains(lower, "failed") || strings.Contains(lower, "failure") {
		return "FAILURE"
	}
	if level == "WARN" || level == "WARNING" {
		return "WARN"
	}
	return ""
}

func containsHTTPMethodToken(value string) bool {
	for _, method := range []string{"get", "post", "put", "patch", "delete", "head", "options"} {
		if strings.Contains(value, ">"+method+" ") || strings.Contains(value, "received "+method+" ") {
			return true
		}
	}
	return false
}

func summarizeK8sPodLogFinding(entry aiopsmodel.LogEntry) string {
	raw := strings.TrimSpace(entry.Raw)
	if raw == "" {
		raw = strings.TrimSpace(entry.Message)
	}
	if raw == "" {
		return ""
	}
	return truncateForPrompt(redactSensitiveText(raw), 300)
}

func buildK8sPodLogFindingContext(entries []aiopsmodel.LogEntry, index, maxLines int) []string {
	context := make([]string, 0, maxLines)
	for i := index; i < len(entries) && len(context) < maxLines; i++ {
		line := strings.TrimSpace(formatLogEntryForPrompt(entries[i]))
		if line == "" {
			continue
		}
		context = append(context, truncateForPrompt(redactSensitiveText(line), 240))
	}
	return context
}

func normalizePatternTokensForKey(value string) string {
	tokens := normalizePatternTokens(tokenizePattern(value))
	if len(tokens) > 20 {
		tokens = tokens[:20]
	}
	return strings.ToLower(strings.Join(tokens, " "))
}

func buildK8sPodEvidenceProgressMessage(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) string {
	parts := []string{
		fmt.Sprintf("日志筛选完成：匹配日志 %d 行", evidence.MatchedLogLines),
	}
	if len(evidence.LogFindings) > 0 {
		kinds := make([]string, 0, len(evidence.LogFindings))
		seen := map[string]struct{}{}
		for _, finding := range evidence.LogFindings {
			if finding.Kind == "" {
				continue
			}
			if _, ok := seen[finding.Kind]; ok {
				continue
			}
			seen[finding.Kind] = struct{}{}
			kinds = append(kinds, finding.Kind)
		}
		sort.Strings(kinds)
		parts = append(parts, fmt.Sprintf("关键异常 %d 条：%s", len(evidence.LogFindings), strings.Join(kinds, "、")))
	}
	if evidence.WarningEvents > 0 {
		parts = append(parts, fmt.Sprintf("Warning 事件 %d 条", evidence.WarningEvents))
	}
	if len(evidence.ResourceHotspots) > 0 {
		parts = append(parts, "资源热点 "+strings.Join(evidence.ResourceHotspots, "、"))
	}
	if evidence.MetricsTrend != nil && evidence.MetricsTrend.Available {
		parts = append(parts, "监控趋势已汇总")
	}
	return strings.Join(parts, "；")
}

func buildK8sPodMetricsTrendProgressMessage(metricsTrend *aiopsmodel.K8sPodAnalyzeMetricsTrendInfo) string {
	if metricsTrend == nil || !metricsTrend.Available {
		return "监控趋势无可用数据"
	}
	parts := []string{
		fmt.Sprintf("近 %s 趋势采集完成：Pod %d 个", firstNonEmptyString(metricsTrend.Range, "6h"), metricsTrend.PodCount),
	}
	if metricsTrend.CPUUsageRate != nil {
		parts = append(parts, "CPU "+formatK8sPodMetricRange(metricsTrend.CPUUsageRate))
	}
	if metricsTrend.MemoryWSSUsageRate != nil {
		parts = append(parts, "内存WSS "+formatK8sPodMetricRange(metricsTrend.MemoryWSSUsageRate))
	}
	if len(metricsTrend.Warnings) > 0 {
		parts = append(parts, fmt.Sprintf("告警 %d 条", len(metricsTrend.Warnings)))
	}
	return strings.Join(parts, "；")
}

func buildK8sPodStatusProgressMessage(pod *corev1.Pod, containerNames []string) string {
	ready := 0
	restarts := int32(0)
	states := make([]string, 0, len(pod.Status.ContainerStatuses))
	selected := map[string]struct{}{}
	for _, name := range containerNames {
		selected[name] = struct{}{}
	}
	for _, status := range pod.Status.ContainerStatuses {
		if len(selected) > 0 {
			if _, ok := selected[status.Name]; !ok {
				continue
			}
		}
		if status.Ready {
			ready++
		}
		restarts += status.RestartCount
		state := "unknown"
		switch {
		case status.State.Running != nil:
			state = "running"
		case status.State.Waiting != nil:
			state = "waiting:" + status.State.Waiting.Reason
		case status.State.Terminated != nil:
			state = "terminated:" + status.State.Terminated.Reason
		}
		states = append(states, status.Name+"="+state)
	}
	return fmt.Sprintf("Pod 状态读取完成：phase=%s，容器 %d 个，Ready %d 个，重启 %d 次，状态=%s",
		pod.Status.Phase,
		len(containerNames),
		ready,
		restarts,
		strings.Join(states, "、"),
	)
}

func countK8sWarningEvents(events []corev1.Event) int {
	count := 0
	for _, event := range events {
		if strings.EqualFold(event.Type, corev1.EventTypeWarning) {
			count++
		}
	}
	return count
}

func buildK8sPodAnalyzeInfo(clusterID uint, pod *corev1.Pod, podMetrics *metricsv1beta1.PodMetrics, containerName string) aiopsmodel.K8sPodAnalyzePodInfo {
	info := aiopsmodel.K8sPodAnalyzePodInfo{
		ClusterID: clusterID,
		Namespace: pod.Namespace,
		Pod:       pod.Name,
		Node:      pod.Spec.NodeName,
		Phase:     string(pod.Status.Phase),
		QoSClass:  string(pod.Status.QOSClass),
		Container: containerName,
	}
	if pod.Status.StartTime != nil {
		info.StartTime = pod.Status.StartTime.Format(time.RFC3339)
	}
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			info.ContainerPorts = append(info.ContainerPorts, aiopsmodel.K8sPodAnalyzeContainerPortInfo{
				Container: container.Name,
				Name:      port.Name,
				Port:      port.ContainerPort,
				Protocol:  string(port.Protocol),
			})
		}
	}
	for _, condition := range pod.Status.Conditions {
		info.Conditions = append(info.Conditions, aiopsmodel.K8sPodAnalyzeCondition{
			Type:    string(condition.Type),
			Status:  string(condition.Status),
			Reason:  condition.Reason,
			Message: condition.Message,
		})
	}
	for _, status := range pod.Status.ContainerStatuses {
		item := aiopsmodel.K8sPodAnalyzeContainerInfo{
			Name:         status.Name,
			Ready:        status.Ready,
			RestartCount: status.RestartCount,
		}
		if spec := findPodContainerSpec(pod, status.Name); spec != nil {
			item.Resources = buildContainerResourceInfo(*spec, findPodMetricUsage(podMetrics, status.Name))
			info.Resources = mergePodResourceInfo(info.Resources, item.Resources)
		}
		if status.Name == containerName {
			info.RestartCount = status.RestartCount
		}
		fillContainerStateInfo(&item, status.State, false)
		fillContainerStateInfo(&item, status.LastTerminationState, true)
		info.Containers = append(info.Containers, item)
	}
	return info
}

func fillContainerStateInfo(item *aiopsmodel.K8sPodAnalyzeContainerInfo, state corev1.ContainerState, last bool) {
	stateName := ""
	reason := ""
	message := ""
	switch {
	case state.Waiting != nil:
		stateName = "waiting"
		reason = state.Waiting.Reason
		message = state.Waiting.Message
	case state.Running != nil:
		stateName = "running"
	case state.Terminated != nil:
		stateName = "terminated"
		reason = state.Terminated.Reason
		message = state.Terminated.Message
	default:
		return
	}
	if last {
		item.LastState = stateName
		item.LastReason = reason
		item.LastMessage = message
		return
	}
	item.State = stateName
	item.Reason = reason
	item.Message = message
}

func findPodContainerSpec(pod *corev1.Pod, containerName string) *corev1.Container {
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == containerName {
			return &pod.Spec.Containers[i]
		}
	}
	for i := range pod.Spec.InitContainers {
		if pod.Spec.InitContainers[i].Name == containerName {
			return &pod.Spec.InitContainers[i]
		}
	}
	return nil
}

func findPodMetricUsage(podMetrics *metricsv1beta1.PodMetrics, containerName string) corev1.ResourceList {
	if podMetrics == nil {
		return nil
	}
	for _, metric := range podMetrics.Containers {
		if metric.Name == containerName {
			return metric.Usage
		}
	}
	return nil
}

func buildContainerResourceInfo(container corev1.Container, usage corev1.ResourceList) aiopsmodel.K8sPodAnalyzeResourceInfo {
	info := aiopsmodel.K8sPodAnalyzeResourceInfo{}
	cpuRequest := container.Resources.Requests.Cpu()
	cpuLimit := container.Resources.Limits.Cpu()
	memoryRequest := container.Resources.Requests.Memory()
	memoryLimit := container.Resources.Limits.Memory()
	if cpuRequest != nil {
		info.CPURequest = cpuRequest.String()
	}
	if cpuLimit != nil {
		info.CPULimit = cpuLimit.String()
	}
	if memoryRequest != nil {
		info.MemoryRequest = memoryRequest.String()
	}
	if memoryLimit != nil {
		info.MemoryLimit = memoryLimit.String()
	}
	if usage != nil {
		if cpuUsage, ok := usage[corev1.ResourceCPU]; ok {
			info.CPUUsage = cpuUsage.String()
			info.CPUUsageMilli = cpuUsage.MilliValue()
			if cpuRequest != nil && cpuRequest.MilliValue() > 0 {
				info.CPURequestUsageRate = float64(info.CPUUsageMilli) * 100 / float64(cpuRequest.MilliValue())
			}
			if cpuLimit != nil && cpuLimit.MilliValue() > 0 {
				info.CPULimitUsageRate = float64(info.CPUUsageMilli) * 100 / float64(cpuLimit.MilliValue())
			}
		}
		if memoryUsage, ok := usage[corev1.ResourceMemory]; ok {
			info.MemoryUsage = memoryUsage.String()
			info.MemoryUsageBytes = memoryUsage.Value()
			if memoryRequest != nil && memoryRequest.Value() > 0 {
				info.MemoryRequestUsageRate = float64(info.MemoryUsageBytes) * 100 / float64(memoryRequest.Value())
			}
			if memoryLimit != nil && memoryLimit.Value() > 0 {
				info.MemoryLimitUsageRate = float64(info.MemoryUsageBytes) * 100 / float64(memoryLimit.Value())
			}
		}
	}
	return info
}

func mergePodResourceInfo(total, item aiopsmodel.K8sPodAnalyzeResourceInfo) aiopsmodel.K8sPodAnalyzeResourceInfo {
	total.CPUUsageMilli += item.CPUUsageMilli
	total.MemoryUsageBytes += item.MemoryUsageBytes
	total.CPUUsage = formatMilliCPU(total.CPUUsageMilli)
	total.MemoryUsage = formatBytes(total.MemoryUsageBytes)
	return total
}

func detectK8sResourceHotspots(podInfo aiopsmodel.K8sPodAnalyzePodInfo) []string {
	hotspots := map[string]struct{}{}
	for _, container := range podInfo.Containers {
		res := container.Resources
		if res.CPULimitUsageRate >= 90 {
			hotspots["CPU_LIMIT_HIGH"] = struct{}{}
		} else if res.CPURequestUsageRate >= 150 {
			hotspots["CPU_REQUEST_HIGH"] = struct{}{}
		}
		if res.MemoryLimitUsageRate >= 90 {
			hotspots["MEMORY_LIMIT_HIGH"] = struct{}{}
		} else if res.MemoryRequestUsageRate >= 150 {
			hotspots["MEMORY_REQUEST_HIGH"] = struct{}{}
		}
	}
	return sortedStringSet(hotspots)
}

func collectK8sPodNodeInfo(ctx context.Context, clientset *kubernetes.Clientset, pod *corev1.Pod) (*aiopsmodel.K8sPodAnalyzeNodeInfo, error) {
	if pod.Spec.NodeName == "" {
		return &aiopsmodel.K8sPodAnalyzeNodeInfo{
			CollectionSkipped: true,
			SuspectedReasons:  []string{"POD_NOT_SCHEDULED"},
		}, nil
	}
	node, err := clientset.CoreV1().Nodes().Get(ctx, pod.Spec.NodeName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	info := &aiopsmodel.K8sPodAnalyzeNodeInfo{
		Name:        node.Name,
		Capacity:    resourceListToStringMap(node.Status.Capacity),
		Allocatable: resourceListToStringMap(node.Status.Allocatable),
	}
	reasons := map[string]struct{}{}
	for _, condition := range node.Status.Conditions {
		item := aiopsmodel.K8sPodAnalyzeCondition{
			Type:    string(condition.Type),
			Status:  string(condition.Status),
			Reason:  condition.Reason,
			Message: condition.Message,
		}
		info.Conditions = append(info.Conditions, item)
		if condition.Type == corev1.NodeReady {
			info.Ready = string(condition.Status)
			if condition.Status != corev1.ConditionTrue {
				reasons["NODE_NOT_READY"] = struct{}{}
			}
			continue
		}
		if condition.Status == corev1.ConditionTrue {
			switch condition.Type {
			case corev1.NodeMemoryPressure:
				reasons["NODE_MEMORY_PRESSURE"] = struct{}{}
			case corev1.NodeDiskPressure:
				reasons["NODE_DISK_PRESSURE"] = struct{}{}
			case corev1.NodePIDPressure:
				reasons["NODE_PID_PRESSURE"] = struct{}{}
			case corev1.NodeNetworkUnavailable:
				reasons["NODE_NETWORK_UNAVAILABLE"] = struct{}{}
			}
		}
	}
	for _, taint := range node.Spec.Taints {
		value := taint.Key
		if taint.Value != "" {
			value += "=" + taint.Value
		}
		value += ":" + string(taint.Effect)
		info.Taints = append(info.Taints, value)
		if taint.Effect == corev1.TaintEffectNoSchedule || taint.Effect == corev1.TaintEffectNoExecute {
			if !podToleratesTaint(pod, taint) {
				reasons["NODE_TAINT_NOT_TOLERATED"] = struct{}{}
			}
		}
	}
	info.SuspectedReasons = sortedStringSet(reasons)
	return info, nil
}

func collectK8sPodQuotaInfo(ctx context.Context, clientset *kubernetes.Clientset, namespace string) ([]aiopsmodel.K8sPodAnalyzeQuotaInfo, error) {
	list, err := clientset.CoreV1().ResourceQuotas(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	result := make([]aiopsmodel.K8sPodAnalyzeQuotaInfo, 0, len(list.Items))
	for _, quota := range list.Items {
		item := aiopsmodel.K8sPodAnalyzeQuotaInfo{
			Name: quota.Name,
			Hard: resourceListToStringMap(quota.Status.Hard),
			Used: resourceListToStringMap(quota.Status.Used),
		}
		reasons := map[string]struct{}{}
		for resourceName, hard := range quota.Status.Hard {
			used, ok := quota.Status.Used[resourceName]
			if ok && !hard.IsZero() && used.Cmp(hard) >= 0 {
				reasons["QUOTA_EXCEEDED_"+strings.ToUpper(strings.ReplaceAll(string(resourceName), ".", "_"))] = struct{}{}
			}
		}
		item.SuspectedReasons = sortedStringSet(reasons)
		result = append(result, item)
	}
	return result, nil
}

func collectK8sPodPVCInfo(ctx context.Context, clientset *kubernetes.Clientset, pod *corev1.Pod) ([]aiopsmodel.K8sPodAnalyzePVCInfo, error) {
	result := make([]aiopsmodel.K8sPodAnalyzePVCInfo, 0)
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim == nil {
			continue
		}
		claimName := volume.PersistentVolumeClaim.ClaimName
		pvc, err := clientset.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(ctx, claimName, metav1.GetOptions{})
		if err != nil {
			result = append(result, aiopsmodel.K8sPodAnalyzePVCInfo{
				Name:             claimName,
				SuspectedReasons: []string{"PVC_GET_FAILED"},
				Events:           []string{err.Error()},
			})
			continue
		}
		item := aiopsmodel.K8sPodAnalyzePVCInfo{
			Name:        pvc.Name,
			Phase:       string(pvc.Status.Phase),
			VolumeName:  pvc.Spec.VolumeName,
			Requests:    resourceListToStringMap(pvc.Spec.Resources.Requests),
			AccessModes: accessModesToStrings(pvc.Spec.AccessModes),
		}
		if pvc.Spec.StorageClassName != nil {
			item.StorageClass = *pvc.Spec.StorageClassName
		}
		reasons := map[string]struct{}{}
		if pvc.Status.Phase != corev1.ClaimBound {
			reasons["PVC_"+strings.ToUpper(string(pvc.Status.Phase))] = struct{}{}
		}
		if pvc.Spec.VolumeName != "" {
			pv, err := clientset.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
			if err != nil {
				reasons["PV_GET_FAILED"] = struct{}{}
				item.Events = append(item.Events, err.Error())
			} else {
				item.PVPhase = string(pv.Status.Phase)
				if pv.Status.Phase != corev1.VolumeBound {
					reasons["PV_"+strings.ToUpper(string(pv.Status.Phase))] = struct{}{}
				}
				item.PVNodeAffinity = describePVNodeAffinity(pv)
			}
		}
		events, err := listObjectEvents(ctx, clientset, pod.Namespace, "PersistentVolumeClaim", pvc.Name, 8)
		if err == nil {
			for _, event := range events {
				item.Events = append(item.Events, formatK8sEventOneLine(event))
				if strings.EqualFold(event.Type, corev1.EventTypeWarning) || k8sEvidenceKeywordRegex.MatchString(event.Reason+" "+event.Message) {
					reasons["PVC_EVENT_"+strings.ToUpper(event.Reason)] = struct{}{}
				}
			}
		}
		item.SuspectedReasons = sortedStringSet(reasons)
		result = append(result, item)
	}
	return result, nil
}

func collectK8sPodProbeInfo(pod *corev1.Pod) []aiopsmodel.K8sPodAnalyzeProbeInfo {
	var probes []aiopsmodel.K8sPodAnalyzeProbeInfo
	for _, container := range pod.Spec.Containers {
		probes = appendProbeInfo(probes, container.Name, "readiness", container.ReadinessProbe)
		probes = appendProbeInfo(probes, container.Name, "liveness", container.LivenessProbe)
		probes = appendProbeInfo(probes, container.Name, "startup", container.StartupProbe)
	}
	return probes
}

func collectK8sPodWorkloadManifest(ctx context.Context, clientset kubernetes.Interface, req *aiopsmodel.K8sPodAnalyzeRequest, pod *corev1.Pod) (*aiopsmodel.K8sPodAnalyzeManifestInfo, error) {
	kind := normalizeK8sAnalyzeWorkloadType(req.WorkloadType)
	name := strings.TrimSpace(req.WorkloadName)
	if kind == "" || name == "" {
		kind, name = resolveK8sPodManifestOwner(ctx, clientset, pod)
	}
	if kind == "" || name == "" {
		manifest, err := buildSanitizedK8sManifest("v1", "Pod", pod)
		return manifest, err
	}

	var object interface{}
	var apiVersion string
	var err error
	switch kind {
	case "Deployment":
		apiVersion = "apps/v1"
		object, err = clientset.AppsV1().Deployments(pod.Namespace).Get(ctx, name, metav1.GetOptions{})
	case "StatefulSet":
		apiVersion = "apps/v1"
		object, err = clientset.AppsV1().StatefulSets(pod.Namespace).Get(ctx, name, metav1.GetOptions{})
	case "DaemonSet":
		apiVersion = "apps/v1"
		object, err = clientset.AppsV1().DaemonSets(pod.Namespace).Get(ctx, name, metav1.GetOptions{})
	case "Job":
		apiVersion = "batch/v1"
		object, err = clientset.BatchV1().Jobs(pod.Namespace).Get(ctx, name, metav1.GetOptions{})
	case "CronJob":
		apiVersion = "batch/v1beta1"
		object, err = clientset.BatchV1beta1().CronJobs(pod.Namespace).Get(ctx, name, metav1.GetOptions{})
	default:
		return nil, fmt.Errorf("unsupported workload kind %s", kind)
	}
	if err != nil {
		return nil, err
	}
	return buildSanitizedK8sManifest(apiVersion, kind, object)
}

func resolveK8sPodManifestOwner(ctx context.Context, clientset kubernetes.Interface, pod *corev1.Pod) (string, string) {
	if pod == nil {
		return "", ""
	}
	for _, owner := range pod.OwnerReferences {
		switch owner.Kind {
		case "ReplicaSet":
			rs, err := clientset.AppsV1().ReplicaSets(pod.Namespace).Get(ctx, owner.Name, metav1.GetOptions{})
			if err == nil {
				for _, rsOwner := range rs.OwnerReferences {
					if rsOwner.Kind == "Deployment" {
						return "Deployment", rsOwner.Name
					}
				}
			}
		case "StatefulSet", "DaemonSet":
			return owner.Kind, owner.Name
		case "Job":
			job, err := clientset.BatchV1().Jobs(pod.Namespace).Get(ctx, owner.Name, metav1.GetOptions{})
			if err == nil {
				for _, jobOwner := range job.OwnerReferences {
					if jobOwner.Kind == "CronJob" {
						return "CronJob", jobOwner.Name
					}
				}
			}
			return "Job", owner.Name
		}
	}
	return "", ""
}

func buildSanitizedK8sManifest(apiVersion, kind string, object interface{}) (*aiopsmodel.K8sPodAnalyzeManifestInfo, error) {
	raw, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}
	var source map[string]interface{}
	if err := json.Unmarshal(raw, &source); err != nil {
		return nil, err
	}
	metadata, _ := source["metadata"].(map[string]interface{})
	name, _ := metadata["name"].(string)
	namespace, _ := metadata["namespace"].(string)

	manifest := map[string]interface{}{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   sanitizeK8sManifestMetadata(metadata),
	}
	if spec, ok := source["spec"]; ok {
		manifest["spec"] = sanitizeK8sManifestValue(spec)
	}
	yamlBytes, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	return &aiopsmodel.K8sPodAnalyzeManifestInfo{
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
		YAML:      redactSensitiveText(strings.TrimSpace(string(yamlBytes))),
	}, nil
}

func sanitizeK8sManifestMetadata(metadata map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, key := range []string{"name", "namespace", "labels"} {
		if value, ok := metadata[key]; ok {
			result[key] = sanitizeK8sManifestValue(value)
		}
	}
	if rawAnnotations, ok := metadata["annotations"].(map[string]interface{}); ok {
		annotations := make(map[string]interface{})
		for key, value := range rawAnnotations {
			if strings.Contains(strings.ToLower(key), "last-applied-configuration") {
				continue
			}
			if manifestSensitiveNameRegex.MatchString(key) {
				annotations[key] = "<redacted>"
			} else {
				annotations[key] = sanitizeK8sManifestValue(value)
			}
		}
		if len(annotations) > 0 {
			result["annotations"] = annotations
		}
	}
	return result
}

func sanitizeK8sManifestValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(typed))
		envName, _ := typed["name"].(string)
		for key, item := range typed {
			if key == "managedFields" || key == "status" {
				continue
			}
			if (key == "value" && manifestSensitiveNameRegex.MatchString(envName)) || manifestSensitiveManifestKey(key) {
				result[key] = "<redacted>"
				continue
			}
			result[key] = sanitizeK8sManifestValue(item)
		}
		return result
	case []interface{}:
		result := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			result = append(result, sanitizeK8sManifestValue(item))
		}
		return result
	default:
		return value
	}
}

func manifestSensitiveManifestKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "password", "passwd", "pwd", "token", "secret", "apikey", "api_key", "accesskey", "access_key", "secretkey", "secret_key", "credential", "privatekey", "private_key":
		return true
	default:
		return false
	}
}

func collectK8sPodNetworkEvidence(ctx context.Context, clientset kubernetes.Interface, pod *corev1.Pod) ([]aiopsmodel.K8sPodAnalyzeServiceInfo, []aiopsmodel.K8sPodAnalyzeIngressRouteInfo, []aiopsmodel.K8sPodAnalyzeManifestInfo, error) {
	serviceList, err := clientset.CoreV1().Services(pod.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("list services failed: %v", err)
	}

	services := make([]aiopsmodel.K8sPodAnalyzeServiceInfo, 0)
	manifests := make([]aiopsmodel.K8sPodAnalyzeManifestInfo, 0)
	serviceNames := make(map[string]struct{})
	podLabels := labels.Set(pod.Labels)
	for _, service := range serviceList.Items {
		if len(service.Spec.Selector) == 0 || !labels.SelectorFromSet(service.Spec.Selector).Matches(podLabels) {
			continue
		}
		item := aiopsmodel.K8sPodAnalyzeServiceInfo{
			Name:     service.Name,
			Type:     string(service.Spec.Type),
			Selector: service.Spec.Selector,
		}
		for _, port := range service.Spec.Ports {
			item.Ports = append(item.Ports, aiopsmodel.K8sPodAnalyzeServicePortInfo{
				Name:       port.Name,
				Port:       port.Port,
				TargetPort: port.TargetPort.String(),
				NodePort:   port.NodePort,
				Protocol:   string(port.Protocol),
			})
		}
		services = append(services, item)
		serviceNames[service.Name] = struct{}{}
		if manifest, manifestErr := buildSanitizedK8sManifest("v1", "Service", &service); manifestErr == nil {
			manifests = append(manifests, *manifest)
		}
	}
	if len(serviceNames) == 0 {
		return services, nil, manifests, nil
	}

	ingressList, err := clientset.NetworkingV1().Ingresses(pod.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return services, nil, manifests, fmt.Errorf("list ingresses failed: %v", err)
	}
	routes := make([]aiopsmodel.K8sPodAnalyzeIngressRouteInfo, 0)
	for _, ingress := range ingressList.Items {
		beforeRouteCount := len(routes)
		if ingress.Spec.DefaultBackend != nil {
			routes = appendK8sPodIngressRoute(routes, serviceNames, ingress.Name, "", "/", ingress.Spec.DefaultBackend.Service)
		}
		for _, rule := range ingress.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}
			for _, ingressPath := range rule.HTTP.Paths {
				routes = appendK8sPodIngressRoute(routes, serviceNames, ingress.Name, rule.Host, ingressPath.Path, ingressPath.Backend.Service)
			}
		}
		if len(routes) > beforeRouteCount {
			if manifest, manifestErr := buildSanitizedK8sManifest("networking.k8s.io/v1", "Ingress", &ingress); manifestErr == nil {
				manifests = append(manifests, *manifest)
			}
		}
	}
	return services, routes, manifests, nil
}

func appendK8sPodIngressRoute(routes []aiopsmodel.K8sPodAnalyzeIngressRouteInfo, serviceNames map[string]struct{}, ingressName, host, routePath string, backend *networkingv1.IngressServiceBackend) []aiopsmodel.K8sPodAnalyzeIngressRouteInfo {
	if backend == nil {
		return routes
	}
	if _, ok := serviceNames[backend.Name]; !ok {
		return routes
	}
	servicePort := backend.Port.Name
	if servicePort == "" && backend.Port.Number > 0 {
		servicePort = strconv.Itoa(int(backend.Port.Number))
	}
	return append(routes, aiopsmodel.K8sPodAnalyzeIngressRouteInfo{
		Ingress:     ingressName,
		Host:        host,
		Path:        routePath,
		ServiceName: backend.Name,
		ServicePort: servicePort,
	})
}

func addK8sPodNetworkSuspectedReasons(evidence *aiopsmodel.K8sPodAnalyzeEvidenceSummary) {
	if evidence == nil || !hasK8sPodLogFindingKind(*evidence, "HTTP_TO_JDWP_PORT") {
		return
	}
	evidence.SuspectedReasons = append(evidence.SuspectedReasons, "HTTP_TO_JDWP_PORT")
	if _, confirmed := findK8sPodJDWPRoute(*evidence); confirmed {
		evidence.SuspectedReasons = append(evidence.SuspectedReasons, "INGRESS_ROUTES_TO_DEBUG_PORT")
	}
	evidence.SuspectedReasons = dedupeK8sStrings(evidence.SuspectedReasons, 20)
}

func hasK8sPodLogFindingKind(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary, kind string) bool {
	for _, finding := range evidence.LogFindings {
		if finding.Kind == kind {
			return true
		}
	}
	return false
}

func findK8sPodJDWPRoute(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) (string, bool) {
	for _, route := range evidence.IngressRoutes {
		for _, service := range evidence.Services {
			if service.Name != route.ServiceName {
				continue
			}
			for _, port := range service.Ports {
				if route.ServicePort != port.Name && route.ServicePort != strconv.Itoa(int(port.Port)) {
					continue
				}
				if !isK8sPodDebugPort(evidence.Pod.ContainerPorts, port) {
					continue
				}
				location := route.Host + route.Path
				if strings.TrimSpace(location) == "" {
					location = "<default>"
				}
				return fmt.Sprintf("Ingress %s(%s) -> Service %s:%s -> targetPort %s", route.Ingress, location, service.Name, route.ServicePort, port.TargetPort), true
			}
		}
	}
	return "", false
}

func isK8sPodDebugPort(containerPorts []aiopsmodel.K8sPodAnalyzeContainerPortInfo, servicePort aiopsmodel.K8sPodAnalyzeServicePortInfo) bool {
	if containsK8sDebugPortName(servicePort.Name) || containsK8sDebugPortName(servicePort.TargetPort) {
		return true
	}
	for _, containerPort := range containerPorts {
		if servicePort.TargetPort == containerPort.Name || servicePort.TargetPort == strconv.Itoa(int(containerPort.Port)) {
			return containsK8sDebugPortName(containerPort.Name)
		}
	}
	return false
}

func containsK8sDebugPortName(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(value, "debug") || strings.Contains(value, "jdwp")
}

func (s *LogAnalysisService) collectK8sIngressEvidence(ctx context.Context, clientset *kubernetes.Clientset, req *aiopsmodel.K8sIngressAnalyzeRequest, customParser *CustomLogParser, includeBackend, includeLogs, includeEvents, includeHTTPProbe bool, report k8sIngressAnalyzeReportFunc) (aiopsmodel.K8sIngressAnalyzeEvidenceSummary, []aiopsmodel.LogEntry, []corev1.Event, error) {
	var ingresses []networkingv1.Ingress
	collectionErrors := make([]string, 0)
	reportK8sIngressAnalyzeProgress(report, "step", "ingress_rules", 16, "读取 Ingress 并匹配 host/path 规则")
	if req.Ingress != "" {
		ing, err := clientset.NetworkingV1().Ingresses(req.Namespace).Get(ctx, req.Ingress, metav1.GetOptions{})
		if err != nil {
			return aiopsmodel.K8sIngressAnalyzeEvidenceSummary{}, nil, nil, fmt.Errorf("get ingress failed: %v", err)
		}
		ingresses = append(ingresses, *ing)
	} else {
		list, err := clientset.NetworkingV1().Ingresses(req.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return aiopsmodel.K8sIngressAnalyzeEvidenceSummary{}, nil, nil, fmt.Errorf("list ingress failed: %v", err)
		}
		for _, ing := range list.Items {
			if ingressMayMatchRequest(&ing, req.Host, req.Path) {
				ingresses = append(ingresses, ing)
			}
		}
		if len(ingresses) == 0 {
			collectionErrors = append(collectionErrors, "no ingress rule matched host/path")
		}
	}

	evidence := aiopsmodel.K8sIngressAnalyzeEvidenceSummary{
		ClusterID:     req.ClusterID,
		Namespace:     req.Namespace,
		Ingress:       req.Ingress,
		Host:          req.Host,
		Path:          req.Path,
		StatusCode:    req.StatusCode,
		ObservedError: req.ObservedError,
	}
	reasons := map[string]struct{}{}
	if req.StatusCode >= 500 {
		reasons["HTTP_5XX"] = struct{}{}
	} else if req.StatusCode >= 400 {
		reasons["HTTP_4XX"] = struct{}{}
	}

	if includeHTTPProbe && req.Host != "" {
		reportK8sIngressAnalyzeProgress(report, "step", "http_probe", 22, "执行域名 HTTP HEAD 探测")
		probe := probeIngressHTTP(ctx, req)
		evidence.HTTPProbe = &probe
		if probe.Error != "" {
			reasons["HTTP_PROBE_FAILED"] = struct{}{}
			collectionErrors = append(collectionErrors, "http probe: "+probe.Error)
		} else if probe.StatusCode >= 500 {
			reasons["LIVE_HTTP_5XX"] = struct{}{}
		} else if probe.StatusCode >= 400 {
			reasons["LIVE_HTTP_4XX"] = struct{}{}
		} else if probe.StatusCode >= 200 && probe.StatusCode < 400 {
			reasons["LIVE_HTTP_OK"] = struct{}{}
			if req.StatusCode >= 400 {
				reasons["INPUT_STATUS_MISMATCH"] = struct{}{}
			}
		}
	}

	var selectedEvents []corev1.Event
	var selectedLogs []aiopsmodel.LogEntry
	backendSeen := map[string]struct{}{}
	for i := range ingresses {
		ing := &ingresses[i]
		if evidence.Ingress == "" {
			evidence.Ingress = ing.Name
		}
		if evidence.IngressClass == "" {
			evidence.IngressClass = ingressClassName(ing)
		}
		for _, lb := range ing.Status.LoadBalancer.Ingress {
			if lb.IP != "" {
				evidence.LoadBalancers = append(evidence.LoadBalancers, lb.IP)
			}
			if lb.Hostname != "" {
				evidence.LoadBalancers = append(evidence.LoadBalancers, lb.Hostname)
			}
		}
		if includeEvents {
			reportK8sIngressAnalyzeProgress(report, "step", "ingress_events", 28, "采集 Ingress 事件")
			events, err := listObjectEvents(ctx, clientset, req.Namespace, "Ingress", ing.Name, req.EventLimit)
			if err != nil {
				collectionErrors = append(collectionErrors, "ingress events "+ing.Name+": "+err.Error())
			} else {
				selectedEvents = append(selectedEvents, events...)
			}
		}
		rules := extractIngressRules(ing, req.Host, req.Path)
		evidence.Rules = append(evidence.Rules, rules...)
		for _, rule := range rules {
			if !rule.Matched || rule.ServiceName == "" || !includeBackend {
				continue
			}
			reportK8sIngressAnalyzeProgress(report, "step", "service", 40, fmt.Sprintf("检查后端 Service %s:%s", rule.ServiceName, rule.ServicePort))
			key := rule.ServiceName + ":" + rule.ServicePort
			if _, ok := backendSeen[key]; ok {
				continue
			}
			backendSeen[key] = struct{}{}
			backend, logs, err := s.collectK8sIngressBackendEvidence(ctx, clientset, req, rule, customParser, includeLogs, includeEvents, report)
			if err != nil {
				backend = aiopsmodel.K8sIngressAnalyzeBackendInfo{
					ServiceName:      rule.ServiceName,
					ServicePort:      rule.ServicePort,
					CollectionErrors: []string{err.Error()},
				}
				reasons["BACKEND_COLLECT_FAILED"] = struct{}{}
			}
			for _, reason := range backend.SuspectedReasons {
				reasons[reason] = struct{}{}
			}
			evidence.MatchedBackends = append(evidence.MatchedBackends, backend)
			selectedLogs = append(selectedLogs, logs...)
		}
	}
	reportK8sIngressAnalyzeProgress(report, "step", "evidence", 82, "整理 Ingress、Service、Endpoint、Pod 和日志证据")
	for _, event := range selectedEvents {
		if strings.EqualFold(event.Type, corev1.EventTypeWarning) {
			evidence.WarningEvents++
			reasons["INGRESS_WARNING_EVENT"] = struct{}{}
		}
		if k8sEvidenceKeywordRegex.MatchString(event.Reason + " " + event.Message) {
			reasons[event.Reason] = struct{}{}
		}
	}
	selectedLogs = selectK8sEvidenceEntries(selectedLogs, req.Limit, 3)
	for _, entry := range selectedLogs {
		level := normalizeLogLevel(entry.Level)
		if level == "ERROR" || level == "FATAL" || level == "PANIC" || level == "WARN" {
			reasons[level] = struct{}{}
		}
	}
	evidence.EventCount = len(selectedEvents)
	evidence.LogLines = len(selectedLogs)
	evidence.MatchedLogLines = len(selectedLogs)
	evidence.SuspectedReasons = sortedStringSet(reasons)
	evidence.CollectionErrors = append(evidence.CollectionErrors, collectionErrors...)
	return evidence, selectedLogs, selectedEvents, nil
}

func (s *LogAnalysisService) collectK8sIngressBackendEvidence(ctx context.Context, clientset *kubernetes.Clientset, req *aiopsmodel.K8sIngressAnalyzeRequest, rule aiopsmodel.K8sIngressAnalyzeRuleInfo, customParser *CustomLogParser, includeLogs, includeEvents bool, report k8sIngressAnalyzeReportFunc) (aiopsmodel.K8sIngressAnalyzeBackendInfo, []aiopsmodel.LogEntry, error) {
	svc, err := clientset.CoreV1().Services(req.Namespace).Get(ctx, rule.ServiceName, metav1.GetOptions{})
	if err != nil {
		return aiopsmodel.K8sIngressAnalyzeBackendInfo{}, nil, fmt.Errorf("get service %s failed: %v", rule.ServiceName, err)
	}
	backend := aiopsmodel.K8sIngressAnalyzeBackendInfo{
		ServiceName: rule.ServiceName,
		ServiceType: string(svc.Spec.Type),
		ServicePort: rule.ServicePort,
		Selector:    svc.Spec.Selector,
	}
	reasons := map[string]struct{}{}
	if len(svc.Spec.Selector) == 0 {
		reasons["SERVICE_WITHOUT_SELECTOR"] = struct{}{}
	}
	if port := findServicePortForIngressRule(svc, rule.ServicePort); port != nil {
		backend.TargetPort = port.TargetPort.String()
	} else {
		reasons["SERVICE_PORT_NOT_FOUND"] = struct{}{}
	}
	endpointSummary, err := k8sservice.GetServiceEndpointSummary(ctx, clientset, req.Namespace, svc.Name)
	reportK8sIngressAnalyzeProgress(report, "step", "endpoints", 52, fmt.Sprintf("检查 Service %s 的 Endpoints", svc.Name))
	if err != nil {
		reasons["ENDPOINTS_GET_FAILED"] = struct{}{}
		backend.CollectionErrors = append(backend.CollectionErrors, err.Error())
	} else {
		backend.EndpointReady = endpointSummary.Ready
		backend.EndpointNotReady = endpointSummary.NotReady
		if backend.EndpointReady == 0 {
			if backend.EndpointNotReady > 0 {
				reasons["ENDPOINTS_NOT_READY"] = struct{}{}
			} else {
				reasons["ENDPOINTS_EMPTY"] = struct{}{}
			}
		}
	}
	if includeEvents {
		events, err := listObjectEvents(ctx, clientset, req.Namespace, "Service", svc.Name, req.EventLimit)
		if err == nil {
			for _, event := range events {
				backend.Events = append(backend.Events, formatK8sEventOneLine(event))
				if strings.EqualFold(event.Type, corev1.EventTypeWarning) {
					reasons["SERVICE_WARNING_EVENT"] = struct{}{}
				}
			}
		}
	}
	var pods []corev1.Pod
	if len(svc.Spec.Selector) > 0 {
		reportK8sIngressAnalyzeProgress(report, "step", "backend_pods", 64, fmt.Sprintf("根据 Service selector 查找后端 Pod"))
		podList, err := clientset.CoreV1().Pods(req.Namespace).List(ctx, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(svc.Spec.Selector).String()})
		if err != nil {
			backend.CollectionErrors = append(backend.CollectionErrors, "list service pods: "+err.Error())
		} else {
			pods = podList.Items
		}
	}
	var logs []aiopsmodel.LogEntry
	for i := range pods {
		pod := &pods[i]
		backend.Pods = append(backend.Pods, buildK8sPodAnalyzeInfo(req.ClusterID, pod, nil, ""))
		if pod.Status.Phase != corev1.PodRunning {
			reasons["BACKEND_POD_"+strings.ToUpper(string(pod.Status.Phase))] = struct{}{}
		}
		for _, status := range pod.Status.ContainerStatuses {
			if !status.Ready {
				reasons["BACKEND_POD_NOT_READY"] = struct{}{}
			}
			if status.RestartCount > 0 {
				reasons["BACKEND_POD_RESTARTED"] = struct{}{}
			}
			if reasonLooksAbnormal(containerStateReason(status.State)) || reasonLooksAbnormal(containerStateReason(status.LastTerminationState)) {
				reasons["BACKEND_CONTAINER_ABNORMAL"] = struct{}{}
			}
		}
		if includeLogs && len(logs) < req.Limit {
			reportK8sIngressAnalyzeProgress(report, "step", "backend_logs", 74, fmt.Sprintf("采集后端 Pod %s 最近日志", pod.Name))
			targets, _ := resolveK8sPodAnalyzeContainersFromPod(pod, "", true)
			for _, containerName := range targets {
				lines, err := s.readK8sTailLines(ctx, clientset, &aiopsmodel.K8sLogQueryRequest{
					ClusterID: req.ClusterID, Namespace: req.Namespace, Pod: pod.Name, Container: containerName, TailLines: minInt(req.TailLines, 300), Timestamps: true,
				})
				if err != nil {
					continue
				}
				source := aiopsmodel.LogSource{Type: aiopsmodel.LogSourceTypeK8s, ClusterID: req.ClusterID, Namespace: req.Namespace, Pod: pod.Name, Container: containerName}
				logs = append(logs, parseRawLinesForSource(s.parser, lines, source, customParser)...)
			}
		}
	}
	if len(pods) == 0 && len(svc.Spec.Selector) > 0 {
		reasons["SERVICE_SELECTOR_MATCHES_NO_PODS"] = struct{}{}
	}
	backend.SuspectedReasons = sortedStringSet(reasons)
	return backend, logs, nil
}

func appendProbeInfo(probes []aiopsmodel.K8sPodAnalyzeProbeInfo, containerName, probeType string, probe *corev1.Probe) []aiopsmodel.K8sPodAnalyzeProbeInfo {
	if probe == nil {
		return probes
	}
	item := aiopsmodel.K8sPodAnalyzeProbeInfo{
		Container:              containerName,
		Type:                   probeType,
		InitialDelaySeconds:    probe.InitialDelaySeconds,
		PeriodSeconds:          probe.PeriodSeconds,
		TimeoutSeconds:         probe.TimeoutSeconds,
		FailureThreshold:       probe.FailureThreshold,
		SuccessThreshold:       probe.SuccessThreshold,
		TerminationGracePeriod: probe.TerminationGracePeriodSeconds,
	}
	switch {
	case probe.HTTPGet != nil:
		item.Handler = "httpGet"
		item.Path = probe.HTTPGet.Path
		item.Port = probe.HTTPGet.Port.String()
	case probe.TCPSocket != nil:
		item.Handler = "tcpSocket"
		item.Port = probe.TCPSocket.Port.String()
	case probe.Exec != nil:
		item.Handler = "exec"
		item.Command = append([]string(nil), probe.Exec.Command...)
	case probe.GRPC != nil:
		item.Handler = "grpc"
		item.Port = strconv.Itoa(int(probe.GRPC.Port))
	default:
		item.Handler = "unknown"
	}
	return append(probes, item)
}

func resourceListToStringMap(values corev1.ResourceList) map[string]string {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]string, len(values))
	for name, quantity := range values {
		result[string(name)] = quantity.String()
	}
	return result
}

func accessModesToStrings(modes []corev1.PersistentVolumeAccessMode) []string {
	result := make([]string, 0, len(modes))
	for _, mode := range modes {
		result = append(result, string(mode))
	}
	return result
}

func describePVNodeAffinity(pv *corev1.PersistentVolume) []string {
	if pv.Spec.NodeAffinity == nil || pv.Spec.NodeAffinity.Required == nil {
		return nil
	}
	var result []string
	for _, term := range pv.Spec.NodeAffinity.Required.NodeSelectorTerms {
		for _, expression := range term.MatchExpressions {
			result = append(result, fmt.Sprintf("%s %s [%s]", expression.Key, expression.Operator, strings.Join(expression.Values, ",")))
		}
	}
	return result
}

func podToleratesTaint(pod *corev1.Pod, taint corev1.Taint) bool {
	for _, toleration := range pod.Spec.Tolerations {
		if toleration.Key != taint.Key && toleration.Operator != corev1.TolerationOpExists {
			continue
		}
		if toleration.Effect != "" && toleration.Effect != taint.Effect {
			continue
		}
		if toleration.Operator == corev1.TolerationOpExists || toleration.Value == taint.Value {
			return true
		}
	}
	return false
}

func formatMilliCPU(value int64) string {
	if value <= 0 {
		return ""
	}
	return fmt.Sprintf("%dm", value)
}

func formatBytes(value int64) string {
	if value <= 0 {
		return ""
	}
	return fmt.Sprintf("%dMi", value/(1024*1024))
}

func collectK8sMatchedKeywords(entries []aiopsmodel.LogEntry) map[string]struct{} {
	keywords := make(map[string]struct{})
	for _, entry := range entries {
		matches := k8sEvidenceKeywordRegex.FindAllString(strings.ToLower(entry.Raw), -1)
		for _, match := range matches {
			keywords[strings.TrimSpace(match)] = struct{}{}
		}
	}
	return keywords
}

func containerStateReason(state corev1.ContainerState) string {
	switch {
	case state.Waiting != nil:
		return state.Waiting.Reason
	case state.Terminated != nil:
		return state.Terminated.Reason
	default:
		return ""
	}
}

func normalizeIngressAnalyzeHostAndPath(rawHost, rawPath string) (string, string) {
	host := strings.TrimSpace(rawHost)
	requestPath := strings.TrimSpace(rawPath)
	if requestPath == "" {
		requestPath = "/"
	}
	if host == "" {
		return "", requestPath
	}
	parseValue := host
	if strings.Contains(parseValue, "://") {
		if parsed, err := url.Parse(parseValue); err == nil {
			if parsed.Hostname() != "" {
				host = parsed.Hostname()
			}
			if strings.TrimSpace(rawPath) == "" && parsed.EscapedPath() != "" {
				requestPath = parsed.EscapedPath()
				if parsed.RawQuery != "" {
					requestPath += "?" + parsed.RawQuery
				}
			}
		}
	} else {
		if idx := strings.IndexAny(parseValue, "/?"); idx >= 0 {
			host = parseValue[:idx]
			if strings.TrimSpace(rawPath) == "" {
				requestPath = parseValue[idx:]
			}
		}
		host = strings.TrimPrefix(host, "//")
		if strings.Contains(host, ":") {
			if parsed, err := url.Parse("//" + host); err == nil && parsed.Hostname() != "" {
				host = parsed.Hostname()
			}
		}
	}
	host = strings.TrimSpace(strings.TrimSuffix(host, "."))
	if requestPath == "" || requestPath[0] != '/' {
		requestPath = "/" + requestPath
	}
	return host, requestPath
}

func probeIngressHTTP(ctx context.Context, req *aiopsmodel.K8sIngressAnalyzeRequest) aiopsmodel.K8sIngressAnalyzeHTTPProbeInfo {
	scheme := strings.ToLower(strings.TrimSpace(req.ProbeScheme))
	if scheme == "" {
		scheme = "http"
	}
	if scheme != "http" && scheme != "https" {
		scheme = "http"
	}
	probePath := req.Path
	if probePath == "" {
		probePath = "/"
	}
	if !strings.HasPrefix(probePath, "/") {
		probePath = "/" + probePath
	}
	timeout := time.Duration(req.ProbeTimeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if timeout > 30*time.Second {
		timeout = 30 * time.Second
	}
	targetURL := scheme + "://" + req.Host + probePath
	result := aiopsmodel.K8sIngressAnalyzeHTTPProbeInfo{
		URL:    targetURL,
		Method: http.MethodHead,
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(probeCtx, http.MethodHead, targetURL, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	httpReq.Header.Set("User-Agent", "AIOps-Ingress-Diagnosis/1.0")
	start := time.Now()
	resp, err := http.DefaultClient.Do(httpReq)
	result.ResponseTime = time.Since(start).Milliseconds()
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()
	result.StatusCode = resp.StatusCode
	result.Status = resp.Status
	result.Headers = map[string]string{}
	for key, values := range resp.Header {
		if len(values) > 0 {
			result.Headers[key] = values[0]
		}
	}
	if req.StatusCode > 0 {
		result.MatchedInput = req.StatusCode == resp.StatusCode
	} else {
		result.MatchedInput = resp.StatusCode >= 200 && resp.StatusCode < 400
	}
	return result
}

func ingressClassName(ing *networkingv1.Ingress) string {
	if ing.Spec.IngressClassName != nil {
		return *ing.Spec.IngressClassName
	}
	if ing.Annotations != nil {
		return ing.Annotations["kubernetes.io/ingress.class"]
	}
	return ""
}

func ingressMayMatchRequest(ing *networkingv1.Ingress, host, requestPath string) bool {
	for _, rule := range extractIngressRules(ing, host, requestPath) {
		if rule.Matched {
			return true
		}
	}
	return false
}

func extractIngressRules(ing *networkingv1.Ingress, host, requestPath string) []aiopsmodel.K8sIngressAnalyzeRuleInfo {
	var result []aiopsmodel.K8sIngressAnalyzeRuleInfo
	if requestPath == "" {
		requestPath = "/"
	}
	for _, rule := range ing.Spec.Rules {
		hostMatched := host == "" || rule.Host == "" || strings.EqualFold(rule.Host, host) || wildcardHostMatches(rule.Host, host)
		if rule.HTTP == nil {
			continue
		}
		for _, pathRule := range rule.HTTP.Paths {
			item := aiopsmodel.K8sIngressAnalyzeRuleInfo{
				Host:     rule.Host,
				Path:     pathRule.Path,
				PathType: "ImplementationSpecific",
				Matched:  hostMatched && ingressPathMatches(pathRule, requestPath),
			}
			if pathRule.PathType != nil {
				item.PathType = string(*pathRule.PathType)
			}
			if pathRule.Backend.Service != nil {
				item.ServiceName = pathRule.Backend.Service.Name
				item.ServicePort = serviceBackendPortString(pathRule.Backend.Service.Port)
			}
			result = append(result, item)
		}
	}
	if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
		result = append(result, aiopsmodel.K8sIngressAnalyzeRuleInfo{
			Path:        "<defaultBackend>",
			ServiceName: ing.Spec.DefaultBackend.Service.Name,
			ServicePort: serviceBackendPortString(ing.Spec.DefaultBackend.Service.Port),
			Matched:     len(result) == 0,
		})
	}
	return result
}

func wildcardHostMatches(pattern, host string) bool {
	if !strings.HasPrefix(pattern, "*.") || host == "" {
		return false
	}
	return strings.HasSuffix(host, strings.TrimPrefix(pattern, "*"))
}

func ingressPathMatches(pathRule networkingv1.HTTPIngressPath, requestPath string) bool {
	rulePath := pathRule.Path
	if rulePath == "" {
		rulePath = "/"
	}
	if pathRule.PathType != nil && *pathRule.PathType == networkingv1.PathTypeExact {
		return requestPath == rulePath
	}
	return requestPath == rulePath || strings.HasPrefix(requestPath, strings.TrimRight(rulePath, "/")+"/") || rulePath == "/"
}

func serviceBackendPortString(port networkingv1.ServiceBackendPort) string {
	if port.Name != "" {
		return port.Name
	}
	if port.Number > 0 {
		return strconv.Itoa(int(port.Number))
	}
	return ""
}

func findServicePortForIngressRule(svc *corev1.Service, servicePort string) *corev1.ServicePort {
	for i := range svc.Spec.Ports {
		port := &svc.Spec.Ports[i]
		if servicePort == port.Name || servicePort == strconv.Itoa(int(port.Port)) {
			return port
		}
	}
	return nil
}

func formatK8sEventOneLine(event corev1.Event) string {
	return fmt.Sprintf("type=%s reason=%s count=%d last=%s message=%s", event.Type, event.Reason, event.Count, k8sEventLastTime(event).Format(time.RFC3339), event.Message)
}

func sortedStringSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func trimK8sContainerDisplayName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, " (init)")
	name = strings.TrimSuffix(name, " (ephemeral)")
	return strings.TrimSpace(name)
}

func k8sLogStreamName(source aiopsmodel.LogSource) string {
	return fmt.Sprintf("%d/%s/%s/%s", source.ClusterID, source.Namespace, source.Pod, source.Container)
}

func (s *LogAnalysisService) openHostSSHClient(c *gin.Context, hostID, authID uint) (*ssh.Client, *cmdbmodel.CmdbHost, configmodel.EcsAuth, error) {
	host, err := cmdbdao.NewCmdbHostSSHDao().GetHostSSHInfo(hostID)
	if err != nil {
		return nil, nil, configmodel.EcsAuth{}, fmt.Errorf("get host failed: %v", err)
	}
	if strings.TrimSpace(host.SSHIP) == "" || host.SSHPort == 0 {
		return nil, nil, configmodel.EcsAuth{}, errors.New("host ssh info is incomplete")
	}

	auth, err := s.resolveHostLoginAuth(c, host, authID)
	if err != nil {
		return nil, nil, configmodel.EcsAuth{}, err
	}
	username := strings.TrimSpace(auth.Username)
	if username == "" {
		username = strings.TrimSpace(host.SSHName)
	}
	if username == "" {
		return nil, nil, configmodel.EcsAuth{}, errors.New("ssh username is empty")
	}

	if auth.Type == 4 {
		bastionAuth := &util.SSHConfig{
			IP:        auth.BastionHost,
			Port:      auth.BastionPort,
			Username:  auth.BastionUsername,
			Type:      auth.BastionAuthType,
			Password:  auth.BastionPassword,
			PublicKey: auth.BastionPrivateKey,
		}
		targetAuth := &util.SSHConfig{
			IP:        host.SSHIP,
			Port:      host.SSHPort,
			Username:  username,
			Type:      auth.TargetAuthType,
			Password:  auth.TargetPassword,
			PublicKey: auth.TargetPrivateKey,
		}
		client, err := util.NewSSHUtil().CreateBastionSSHClient(bastionAuth, targetAuth)
		if err != nil {
			return nil, nil, configmodel.EcsAuth{}, fmt.Errorf("create bastion ssh client failed: %v", err)
		}
		return client, host, auth, nil
	}

	authMethod, err := buildSSHAuthMethod(auth)
	if err != nil {
		return nil, nil, configmodel.EcsAuth{}, err
	}
	sshConfig := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}
	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host.SSHIP, host.SSHPort), sshConfig)
	if err != nil {
		return nil, nil, configmodel.EcsAuth{}, fmt.Errorf("ssh dial failed: %v", err)
	}
	return client, host, auth, nil
}

func (s *LogAnalysisService) resolveHostLoginAuth(c *gin.Context, host *cmdbmodel.CmdbHost, selectedAuthID uint) (configmodel.EcsAuth, error) {
	var auth configmodel.EcsAuth
	authIDs, defaultAuthID, err := boundHostAuthIDs(host)
	if err != nil {
		return auth, err
	}
	if len(authIDs) == 0 {
		return auth, errors.New("host has no ssh credential")
	}
	if len(authIDs) == 1 {
		selectedAuthID = authIDs[0]
	}
	if selectedAuthID == 0 && defaultAuthID > 0 {
		selectedAuthID = defaultAuthID
	}
	if selectedAuthID == 0 {
		return auth, errors.New("host has multiple ssh credentials, authId is required")
	}
	if !containsUint(authIDs, selectedAuthID) {
		return auth, errors.New("selected ssh credential is not bound to host")
	}
	if err := checkHostAuthPermission(c, host.ID, selectedAuthID); err != nil {
		return auth, err
	}

	if err := common.GetDB().Table("config_ecsauth").Where("id = ?", selectedAuthID).First(&auth).Error; err != nil {
		return auth, fmt.Errorf("get ssh credential failed: %v", err)
	}
	if err := auth.DecryptSecrets(); err != nil {
		return auth, fmt.Errorf("decrypt ssh credential failed: %v", err)
	}
	return auth, nil
}

func ResolveLLMProvider(providerID uint) (*aiopsmodel.LLMProvider, string, error) {
	var provider *aiopsmodel.LLMProvider
	var err error
	llmDao := aiopsdao.NewLLMProviderDao()
	if providerID > 0 {
		provider, err = llmDao.GetByID(providerID)
		if err != nil {
			return nil, "", fmt.Errorf("llm provider not found: %v", err)
		}
		if provider.Status != aiopsmodel.LLMProviderStatusEnabled {
			return nil, "", errors.New("llm provider is disabled")
		}
	} else {
		providers, err := llmDao.GetEnabled()
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

func (s *LogAnalysisService) resolveLLMProvider(providerID uint) (*aiopsmodel.LLMProvider, string, error) {
	return ResolveLLMProvider(providerID)
}

func boundHostAuthIDs(host *cmdbmodel.CmdbHost) ([]uint, uint, error) {
	rows, err := cmdbdao.NewCmdbHostAuthDao().GetHostAuths(host.ID)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]uint, 0, len(rows)+1)
	seen := map[uint]struct{}{}
	var defaultAuthID uint
	add := func(id uint) {
		if id == 0 {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(rows) == 0 {
		add(host.SSHKeyID)
		return ids, host.SSHKeyID, nil
	}
	for _, row := range rows {
		add(row.AuthID)
		if row.IsDefault == 1 && defaultAuthID == 0 {
			defaultAuthID = row.AuthID
		}
	}
	return ids, defaultAuthID, nil
}

func checkHostAuthPermission(c *gin.Context, hostID, authID uint) error {
	admin, err := jwt.GetAdmin(c)
	if err != nil || admin == nil {
		return errors.New("user context is required")
	}
	if admin.ID == 1 || admin.Username == "admin" {
		return nil
	}
	authDao := cmdbdao.NewBastionHostAuthorizationDao()
	allowed, err := authDao.CheckUserHostAuthPermission(admin.ID, hostID, authID)
	if err != nil {
		return err
	}
	if !allowed {
		return errors.New("current user has no permission to access this host credential")
	}
	return nil
}

func buildSSHAuthMethod(auth configmodel.EcsAuth) (ssh.AuthMethod, error) {
	switch auth.Type {
	case 1:
		return ssh.Password(auth.Password), nil
	case 2:
		return util.NewSSHUtil().PublicKeyAuth(auth.PublicKey)
	case 3:
		if userKeyAuth, err := util.NewSSHUtil().UserKeyAuth(); err == nil {
			return userKeyAuth, nil
		}
		if defaultKeyAuth, err := util.NewSSHUtil().DefaultKeyAuth(); err == nil {
			return defaultKeyAuth, nil
		}
		return nil, errors.New("type=3 public key login failed: local private key not found")
	default:
		return nil, errors.New("unsupported ssh auth type")
	}
}

type logFilter struct {
	keyword string
	regex   *regexp.Regexp
	levels  map[string]struct{}
}

type logQueryOptions struct {
	SessionID  string
	Keyword    string
	Regex      string
	Levels     []string
	Limit      int
	FormatName string
	FormatYAML string
}

func newLogFilter(req *logQueryOptions) (*logFilter, error) {
	filter := &logFilter{
		keyword: strings.ToLower(strings.TrimSpace(req.Keyword)),
		levels:  map[string]struct{}{},
	}
	if strings.TrimSpace(req.Regex) != "" {
		re, err := regexp.Compile(req.Regex)
		if err != nil {
			return nil, fmt.Errorf("invalid regex: %v", err)
		}
		filter.regex = re
	}
	for _, level := range expandLevelValues(req.Levels) {
		normalized := normalizeLogLevel(level)
		if normalized != "" {
			filter.levels[normalized] = struct{}{}
		}
	}
	return filter, nil
}

func (f *logFilter) Match(entry aiopsmodel.LogEntry) bool {
	if len(f.levels) > 0 {
		if _, ok := f.levels[normalizeLogLevel(entry.Level)]; !ok {
			return false
		}
	}
	if f.keyword != "" {
		text := strings.ToLower(entry.Raw + "\n" + entry.Message)
		if !strings.Contains(text, f.keyword) {
			return false
		}
	}
	if f.regex != nil && !f.regex.MatchString(entry.Raw) {
		return false
	}
	return true
}

func readRemoteTailLines(sftpClient *sftp.Client, remotePath string, tailLines int) ([]string, error) {
	file, err := sftpClient.Open(remotePath)
	if err != nil {
		return nil, fmt.Errorf("open remote log file failed: %v", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat remote log file failed: %v", err)
	}
	if info.IsDir() {
		return nil, errors.New("remote path is a directory")
	}
	if info.Size() == 0 {
		return []string{}, nil
	}

	readBytes := int64(tailLines * 512)
	if readBytes < 64*1024 {
		readBytes = 64 * 1024
	}
	if readBytes > maxLogTailBytes {
		readBytes = maxLogTailBytes
	}
	if readBytes > info.Size() {
		readBytes = info.Size()
	}
	start := info.Size() - readBytes
	buf := make([]byte, readBytes)
	n, err := file.ReadAt(buf, start)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("read remote log file failed: %v", err)
	}
	buf = buf[:n]
	if start > 0 {
		if idx := bytes.IndexByte(buf, '\n'); idx >= 0 && idx+1 < len(buf) {
			buf = buf[idx+1:]
		}
	}

	rawLines := strings.Split(strings.ReplaceAll(string(buf), "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) > tailLines {
		lines = lines[len(lines)-tailLines:]
	}
	return lines, nil
}

func walkRemoteLogFiles(sftpClient *sftp.Client, dir string, maxDepth, limit int, keyword string, files *[]aiopsmodel.HostLogFileInfo) error {
	if len(*files) >= limit {
		return nil
	}
	entries, err := sftpClient.ReadDir(dir)
	if err != nil {
		if maxDepth == defaultFileDepth {
			return fmt.Errorf("read remote dir failed: %v", err)
		}
		return nil
	}
	for _, entry := range entries {
		if len(*files) >= limit {
			return nil
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		fullPath := path.Join(dir, name)
		if entry.IsDir() {
			if maxDepth > 0 {
				_ = walkRemoteLogFiles(sftpClient, fullPath, maxDepth-1, limit, keyword, files)
			}
			continue
		}
		if !looksLikeLogFile(name) {
			continue
		}
		if keyword != "" && !strings.Contains(strings.ToLower(fullPath), keyword) {
			continue
		}
		*files = append(*files, aiopsmodel.HostLogFileInfo{
			Path:    fullPath,
			Name:    name,
			Size:    entry.Size(),
			ModTime: entry.ModTime().Format("2006-01-02 15:04:05"),
		})
	}
	return nil
}

func looksLikeLogFile(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".log") ||
		strings.HasSuffix(lower, ".out") ||
		strings.HasSuffix(lower, ".err") ||
		strings.HasSuffix(lower, ".txt") ||
		strings.HasSuffix(lower, ".json") ||
		strings.Contains(lower, ".log.") {
		return true
	}
	switch lower {
	case "messages", "syslog", "dmesg", "kern.log", "auth.log", "secure", "access.log", "error.log":
		return true
	default:
		return false
	}
}

func cleanRemotePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("path is required")
	}
	if strings.ContainsAny(value, "\x00\r\n") {
		return "", errors.New("path contains invalid characters")
	}
	value = strings.ReplaceAll(value, "\\", "/")
	if !strings.HasPrefix(value, "/") {
		return "", errors.New("path must be absolute")
	}
	return path.Clean(value), nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func parseLogfmt(value string) map[string]interface{} {
	result := map[string]interface{}{}
	i := 0
	for i < len(value) {
		for i < len(value) && value[i] == ' ' {
			i++
		}
		keyStart := i
		for i < len(value) && value[i] != '=' && value[i] != ' ' {
			i++
		}
		if i >= len(value) || value[i] != '=' {
			for i < len(value) && value[i] != ' ' {
				i++
			}
			continue
		}
		key := value[keyStart:i]
		i++
		if key == "" {
			continue
		}
		var val string
		if i < len(value) && value[i] == '"' {
			i++
			start := i
			for i < len(value) && value[i] != '"' {
				if value[i] == '\\' && i+1 < len(value) {
					i += 2
					continue
				}
				i++
			}
			val = value[start:i]
			if i < len(value) && value[i] == '"' {
				i++
			}
		} else {
			start := i
			for i < len(value) && value[i] != ' ' {
				i++
			}
			val = value[start:i]
		}
		result[key] = val
	}
	return result
}

func NewLogTimestampParser() *LogTimestampParser {
	return &LogTimestampParser{
		zonedLayouts: []string{
			time.RFC3339Nano,
			time.RFC3339,
		},
		localDateLayouts: []string{
			"2006-01-02T15:04:05.999999999",
			"2006-01-02 15:04:05.000000000",
			"2006-01-02 15:04:05.000000",
			"2006-01-02 15:04:05.000",
			"2006-01-02 15:04:05",
			"2006-01-02 15:04:05,000000000",
			"2006-01-02 15:04:05,000000",
			"2006-01-02 15:04:05,000",
			"[2006-01-02 15:04:05.000000]",
			"[2006-01-02 15:04:05.000]",
			"[2006-01-02 15:04:05]",
		},
		partialLayouts: []string{
			"Jan 02 15:04:05.000000",
			"Jan 02 15:04:05.000",
			"Jan 02 15:04:05",
			"Jan _2 15:04:05.000000",
			"Jan _2 15:04:05.000",
			"Jan _2 15:04:05",
			"15:04:05.000000000",
			"15:04:05.000000",
			"15:04:05.000",
			"15:04:05",
			"15:04:05,000000000",
			"15:04:05,000000",
			"15:04:05,000",
		},
	}
}

func (p *LogTimestampParser) ParseFromText(text string) (string, int64) {
	match := timestampRegex.FindString(text)
	if match == "" {
		return "", 0
	}
	if parsed, fullDate, ok := p.parseTimestampString(match); ok {
		if fullDate {
			return formatLogTimestampForDisplay(parsed), parsed.Unix()
		}
		return match, parsed.Unix()
	}
	return match, 0
}

func (p *LogTimestampParser) ParseTimestampValue(value interface{}) (time.Time, bool) {
	switch v := value.(type) {
	case string:
		v = strings.TrimSpace(v)
		if v == "" {
			return time.Time{}, false
		}
		if parsed, _, ok := p.parseTimestampString(v); ok {
			return parsed, true
		}
		if unixTime, err := strconv.ParseFloat(v, 64); err == nil {
			return parseUnixTimestamp(unixTime), true
		}
	case float64:
		return parseUnixTimestamp(v), true
	case int64:
		return parseUnixTimestamp(float64(v)), true
	case int:
		return parseUnixTimestamp(float64(v)), true
	}
	return time.Time{}, false
}

func (p *LogTimestampParser) parseTimestampString(value string) (time.Time, bool, bool) {
	normalized := strings.Replace(strings.TrimSpace(value), ",", ".", 1)
	for _, layout := range p.zonedLayouts {
		if parsed, err := time.Parse(layout, normalized); err == nil {
			return parsed, true, true
		}
	}
	for _, layout := range p.localDateLayouts {
		if parsed, err := time.ParseInLocation(layout, normalized, logDisplayLocation); err == nil {
			return parsed, true, true
		}
	}
	for _, layout := range p.partialLayouts {
		if parsed, err := time.Parse(layout, normalized); err == nil {
			return parsed, false, true
		}
	}
	return time.Time{}, false, false
}

var logDisplayLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

func formatLogTimestampForDisplay(value time.Time) string {
	return value.In(logDisplayLocation).Format(time.RFC3339Nano)
}

func normalizeLogTimestampForDisplay(raw string, parser *LogTimestampParser) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || parser == nil {
		return raw
	}
	parsed, fullDate, ok := parser.parseTimestampString(raw)
	if !ok || !fullDate {
		return raw
	}
	return formatLogTimestampForDisplay(parsed)
}

func parseUnixTimestamp(unixTime float64) time.Time {
	if unixTime > 1e15 {
		return time.Unix(0, int64(unixTime))
	}
	if unixTime > 1e12 {
		return time.Unix(0, int64(unixTime*1e3))
	}
	if unixTime > 1e9 {
		return time.Unix(0, int64(unixTime*1e6))
	}
	return time.Unix(int64(unixTime), 0)
}

func NewCustomLogParser(raw string) (*CustomLogParser, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var format LogFormatDefinition
	if err := yaml.Unmarshal([]byte(raw), &format); err != nil {
		return nil, fmt.Errorf("parse format yaml failed: %v", err)
	}
	parser := &CustomLogParser{
		format:     &format,
		fieldRegex: make(map[string]*regexp.Regexp),
		templates:  make(map[string]*template.Template),
	}
	if format.Pattern.UseRegex && strings.TrimSpace(format.Pattern.Main) != "" {
		re, err := regexp.Compile(format.Pattern.Main)
		if err != nil {
			return nil, fmt.Errorf("compile format regex failed: %v", err)
		}
		parser.mainRegex = re
	}
	for key, value := range format.Pattern.Fields {
		re, err := regexp.Compile(value)
		if err != nil {
			return nil, fmt.Errorf("compile field regex failed for %s: %v", key, err)
		}
		parser.fieldRegex[key] = re
	}
	collectTemplate := func(name string, extractor LogFieldExtractor) error {
		if strings.TrimSpace(extractor.Template) == "" {
			return nil
		}
		tmpl, err := template.New(name).Parse(extractor.Template)
		if err != nil {
			return err
		}
		parser.templates[name] = tmpl
		return nil
	}
	if err := collectTemplate("timestamp", format.Mapping.Timestamp); err != nil {
		return nil, err
	}
	if err := collectTemplate("severity", format.Mapping.Severity); err != nil {
		return nil, err
	}
	if err := collectTemplate("body", format.Mapping.Body); err != nil {
		return nil, err
	}
	for key, extractor := range format.Mapping.Attributes {
		if err := collectTemplate("attr_"+key, extractor); err != nil {
			return nil, err
		}
	}
	return parser, nil
}

func (p *CustomLogParser) Parse(raw string, timestampParser *LogTimestampParser) (*LogParseResult, error) {
	if p == nil || p.format == nil {
		return nil, nil
	}
	data, err := p.parseRaw(raw)
	if err != nil {
		return nil, err
	}

	result := &LogParseResult{
		Fields:     data,
		Attributes: make(map[string]string),
		Message:    strings.TrimSpace(raw),
	}
	result.Timestamp = p.extractValue(data, p.format.Mapping.Timestamp, "timestamp")
	result.Level = normalizeLogLevel(p.extractValue(data, p.format.Mapping.Severity, "severity"))
	result.Message = p.extractValue(data, p.format.Mapping.Body, "body")
	if result.Message == "" {
		result.Message = strings.TrimSpace(raw)
	}
	for key, extractor := range p.format.Mapping.Attributes {
		value := p.extractValue(data, extractor, "attr_"+key)
		if strings.TrimSpace(value) != "" {
			result.Attributes[key] = value
		}
	}
	if p.format.Mapping.AutoMapRemaining {
		for key, value := range data {
			if _, exists := result.Attributes[key]; exists {
				continue
			}
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				result.Attributes[key] = text
			}
		}
	}
	if result.Timestamp != "" {
		if parsed, ok := p.parseTimestamp(result.Timestamp, p.format.Mapping.Timestamp.TimeFormat, timestampParser); ok {
			result.TimestampUnix = parsed.Unix()
		}
	}
	return result, nil
}

func (p *CustomLogParser) parseRaw(raw string) (map[string]interface{}, error) {
	switch strings.ToLower(strings.TrimSpace(p.format.Type)) {
	case "json":
		return p.parseJSON(raw)
	default:
		return p.parseText(raw)
	}
}

func (p *CustomLogParser) parseJSON(raw string) (map[string]interface{}, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, err
	}
	if len(p.format.JSON.Fields) == 0 {
		return data, nil
	}
	result := make(map[string]interface{})
	for key, field := range p.format.JSON.Fields {
		if value := extractJSONField(data, field); value != nil {
			result[key] = value
		}
	}
	return result, nil
}

func (p *CustomLogParser) parseText(raw string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	if p.mainRegex != nil {
		matches := p.mainRegex.FindStringSubmatch(raw)
		if matches == nil {
			return nil, fmt.Errorf("line does not match custom regex")
		}
		for i, name := range p.mainRegex.SubexpNames() {
			if i > 0 && i < len(matches) && name != "" {
				result[name] = matches[i]
			}
		}
	}
	for key, re := range p.fieldRegex {
		if matches := re.FindStringSubmatch(raw); len(matches) > 1 {
			result[key] = matches[1]
		}
	}
	if len(result) == 0 {
		result["raw"] = raw
	}
	return result, nil
}

func (p *CustomLogParser) extractValue(data map[string]interface{}, extractor LogFieldExtractor, key string) string {
	var value interface{}
	if strings.TrimSpace(extractor.Field) != "" {
		value = extractJSONField(data, extractor.Field)
	} else if strings.TrimSpace(extractor.Template) != "" {
		if tmpl, ok := p.templates[key]; ok {
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err == nil {
				value = buf.String()
			}
		}
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		text = strings.TrimSpace(extractor.Default)
	}
	if strings.TrimSpace(extractor.Pattern) != "" && text != "" {
		if re, err := regexp.Compile(extractor.Pattern); err == nil {
			if matches := re.FindStringSubmatch(text); len(matches) > 1 {
				text = matches[1]
			}
		}
	}
	switch strings.ToLower(strings.TrimSpace(extractor.Transform)) {
	case "uppercase":
		text = strings.ToUpper(text)
	case "lowercase":
		text = strings.ToLower(text)
	case "trim":
		text = strings.TrimSpace(text)
	}
	return text
}

func (p *CustomLogParser) parseTimestamp(raw, format string, timestampParser *LogTimestampParser) (time.Time, bool) {
	if strings.TrimSpace(raw) == "" {
		return time.Time{}, false
	}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "auto":
		return timestampParser.ParseTimestampValue(raw)
	case "rfc3339":
		parsed, err := time.Parse(time.RFC3339, raw)
		return parsed, err == nil
	case "unix":
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return time.Time{}, false
		}
		return time.Unix(v, 0), true
	case "unix_ms":
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return time.Time{}, false
		}
		return time.Unix(v/1000, (v%1000)*1e6), true
	case "unix_ns":
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return time.Time{}, false
		}
		return time.Unix(0, v), true
	default:
		parsed, err := time.ParseInLocation(format, raw, logDisplayLocation)
		return parsed, err == nil
	}
}

func extractJSONField(data map[string]interface{}, field string) interface{} {
	field = strings.TrimSpace(field)
	if field == "" {
		return nil
	}
	if !strings.Contains(field, ".") {
		return data[field]
	}
	current := interface{}(data)
	for _, part := range strings.Split(field, ".") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil
		}
		asMap, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = asMap[part]
		if current == nil {
			return nil
		}
	}
	return current
}

func NewLogLineParser() *LogLineParser {
	return &LogLineParser{
		timestampParser: NewLogTimestampParser(),
	}
}

func (p *LogLineParser) Parse(raw string, lineNo int, source aiopsmodel.LogSource, custom *CustomLogParser) aiopsmodel.LogEntry {
	if custom != nil {
		if parsed, err := custom.Parse(raw, p.timestampParser); err == nil && parsed != nil {
			return buildEntryFromParseResult(raw, lineNo, source, parsed, p.timestampParser)
		}
	}

	fields := map[string]interface{}{}
	attributes := map[string]string{}
	message := strings.TrimSpace(raw)
	level := ""
	timestamp := ""
	timestampUnix := int64(0)

	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		var data map[string]interface{}
		if json.Unmarshal([]byte(trimmed), &data) == nil {
			fields = data
			level = firstFieldString(data, "level", "severity", "log.level", "severity_text")
			timestamp = firstFieldString(data, "timestamp", "@timestamp", "time", "ts", "datetime")
			message = firstFieldString(data, "message", "msg", "log", "body")
			if message == "" {
				message = raw
			}
			attributes = flattenAttributes(data, "")
		}
	} else if strings.Contains(trimmed, "=") {
		if data := parseLogfmt(trimmed); len(data) > 0 {
			fields = data
			level = firstFieldString(data, "level", "severity", "lvl")
			timestamp = firstFieldString(data, "timestamp", "time", "ts")
			message = firstFieldString(data, "message", "msg", "event")
			if message == "" {
				message = raw
			}
			for key, value := range data {
				attributes[key] = strings.TrimSpace(fmt.Sprint(value))
			}
		}
	}

	if level == "" {
		level = detectLevel(raw)
	}
	level = normalizeLogLevel(level)
	if timestamp == "" {
		timestamp, timestampUnix = p.timestampParser.ParseFromText(raw)
	} else if parsed, ok := p.timestampParser.ParseTimestampValue(timestamp); ok {
		timestampUnix = parsed.Unix()
		timestamp = normalizeLogTimestampForDisplay(timestamp, p.timestampParser)
	}
	if message == raw {
		if matches := severityPrefixRegex.FindStringSubmatch(strings.TrimSpace(message)); len(matches) > 2 {
			message = strings.TrimSpace(matches[2])
		}
	}

	return aiopsmodel.LogEntry{
		LineNo:        lineNo,
		Timestamp:     timestamp,
		TimestampUnix: timestampUnix,
		Level:         level,
		Message:       strings.TrimSpace(message),
		Raw:           raw,
		Fields:        fields,
		Attributes:    attributes,
		Source:        source,
	}
}

func buildEntryFromParseResult(raw string, lineNo int, source aiopsmodel.LogSource, parsed *LogParseResult, timestampParser *LogTimestampParser) aiopsmodel.LogEntry {
	level := normalizeLogLevel(parsed.Level)
	timestamp := strings.TrimSpace(parsed.Timestamp)
	timestampUnix := parsed.TimestampUnix
	if timestampUnix == 0 && timestamp != "" {
		if value, ok := timestampParser.ParseTimestampValue(timestamp); ok {
			timestampUnix = value.Unix()
		}
	}
	timestamp = normalizeLogTimestampForDisplay(timestamp, timestampParser)
	if level == "" {
		level = detectLevel(raw)
	}
	message := strings.TrimSpace(parsed.Message)
	if message == "" {
		message = strings.TrimSpace(raw)
	}
	return aiopsmodel.LogEntry{
		LineNo:        lineNo,
		Timestamp:     timestamp,
		TimestampUnix: timestampUnix,
		Level:         level,
		Message:       message,
		Raw:           raw,
		Fields:        parsed.Fields,
		Attributes:    parsed.Attributes,
		Source:        source,
	}
}

func flattenAttributes(input map[string]interface{}, prefix string) map[string]string {
	result := make(map[string]string)
	for key, value := range input {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}
		switch typed := value.(type) {
		case map[string]interface{}:
			for nestedKey, nestedValue := range flattenAttributes(typed, fullKey) {
				result[nestedKey] = nestedValue
			}
		default:
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				result[fullKey] = text
			}
		}
	}
	return result
}

func extractKeywordTokens(value string) []string {
	words := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-')
	})
	result := make([]string, 0, len(words))
	for _, word := range words {
		word = strings.Trim(word, "_-")
		if len(word) < 3 || isDigits(word) {
			continue
		}
		if _, ok := stopWords[word]; ok {
			continue
		}
		result = append(result, word)
	}
	return result
}

func firstFieldString(fields map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := fields[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func detectLevel(value string) string {
	match := logLevelRegex.FindString(value)
	return normalizeLogLevel(match)
}

func normalizeLogLevel(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	switch value {
	case "WARNING":
		return "WARN"
	case "ERR", "CRITICAL":
		return "ERROR"
	default:
		return value
	}
}

func expandLevelValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				result = append(result, part)
			}
		}
	}
	return result
}

func normalizePositiveLimit(value, fallback, max int) int {
	if value <= 0 {
		value = fallback
	}
	if value > max {
		value = max
	}
	return value
}

func containsUint(values []uint, target uint) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

type lockedWSWriter struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func (w *lockedWSWriter) WriteJSON(value interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return w.conn.WriteJSON(value)
}

func buildLogAnalysisPrompt(extraPrompt string, summary *aiopsmodel.LogSummary, lines []string) string {
	lines = limitAnalysisLines(lines, maxLogAnalysisLines, maxLogAnalysisChars)
	var builder strings.Builder
	builder.WriteString("You are an experienced SRE. Analyze the following operation logs and answer in Chinese, concise and operational.\n")
	builder.WriteString("The logs are ordered from older to newer and contain at most the latest 100 lines. Prioritize direct evidence such as ERROR/WARN levels, exceptions, failed status codes, error codes and repeated failures.\n")
	builder.WriteString("First decide one conclusion: 问题, 误判, 疑似问题, or 证据不足. Use 问题 only when logs directly prove a failure or harmful effect. Use 误判 when the query keyword merely appears in a URL, request body or normal business field while the request is successful and no failure evidence exists. Use 疑似问题 when behavior is suspicious but impact is unproven.\n")
	builder.WriteString("A suspicious payload with only a 2xx/3xx access log proves an attempt at most, not successful exploitation. Empty authentication fields in an access log do not prove missing authentication because authentication may be handled upstream or omitted from logging.\n")
	builder.WriteString("Use exactly these Markdown sections and nothing else:\n")
	builder.WriteString("### 分析结论\n结论：问题/误判/疑似问题/证据不足；严重性：高/中/低\nOne-sentence reason, no more than 80 Chinese characters.\n")
	builder.WriteString("### 关键证据\nAt most 3 bullets, each no more than 60 Chinese characters. Include only evidence that changes the conclusion.\n")
	builder.WriteString("### 处置建议\nAt most 3 numbered actions, each no more than 60 Chinese characters. For 误判, explain how to refine the query or verification instead of proposing broad remediation.\n")
	builder.WriteString("Keep the entire answer within 500 Chinese characters. Do not include long log excerpts, generic security hardening lists, repeated explanations or unsupported claims. Clearly distinguish confirmed evidence from inference.\n")
	if strings.TrimSpace(extraPrompt) != "" {
		builder.WriteString("\nAdditional operator question:\n")
		builder.WriteString(strings.TrimSpace(extraPrompt))
		builder.WriteString("\n")
	}
	if summary != nil {
		summaryJSON, _ := json.Marshal(summary)
		builder.WriteString("\nLog summary JSON:\n")
		builder.Write(summaryJSON)
		builder.WriteString("\n")
	}
	builder.WriteString("\nLogs:\n")
	for _, line := range lines {
		builder.WriteString(redactSensitiveText(line))
		builder.WriteByte('\n')
	}
	return builder.String()
}

func buildK8sPodAnalysisPrompt(extraPrompt string, evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary, logs, previousLogs []aiopsmodel.LogEntry, events []corev1.Event) string {
	logs = limitEntriesForPrompt(logs, 60, 6000)
	previousLogs = limitEntriesForPrompt(previousLogs, 30, 3000)
	if len(events) > 20 {
		events = events[:20]
	}

	promptEvidence := evidence
	promptEvidence.Manifests = nil
	summaryJSON, _ := json.Marshal(promptEvidence)
	var builder strings.Builder
	builder.WriteString("You are an experienced Kubernetes SRE. Analyze this Pod/workload using the supplied evidence. Answer in Chinese with an evidence-driven, operational diagnosis.\n")
	builder.WriteString("Use exactly these four Markdown sections and do not add any other section:\n")
	builder.WriteString("### 问题描述\nState the symptom, affected object/path and current workload availability in 1-2 sentences.\n")
	builder.WriteString("### 问题原因\nState the confirmed root cause first. Distinguish likely causes only when evidence is insufficient, and explain the relevant Kubernetes object relationship.\n")
	builder.WriteString("### 解决方案\nGive 3-5 numbered actions, naming the exact Kubernetes object and desired value when evidence permits. Include the checks needed to prove the repair worked inside these actions.\n")
	builder.WriteString("### 总结结论\nGive one concise final conclusion covering current impact, the decisive cause and the expected state after repair.\n")
	builder.WriteString("Do not list every stack frame, every event, or every resource field. Do not give generic advice unrelated to the strongest evidence. If evidence is insufficient, identify the exact missing object or field and lower the confidence explicitly.\n")
	builder.WriteString("Prioritize application ERROR/FATAL/PANIC/RPC_EXCEPTION/DUBBO_NO_PROVIDER findings over normal Running state. Do not downgrade an application ERROR just because the Pod is Running.\n")
	builder.WriteString("Treat 'Debugger failed to attach' with 'received >GET ...< expected >JDWP-Handshake<' as a protocol mismatch: an HTTP request reached a JVM JDWP debug port. This is not a generic application WARN. Correlate it with containerPorts, Service ports/targetPorts and Ingress backend ports.\n")
	builder.WriteString("When this JDWP mismatch is present and a Service exposes a debug/JDWP port, recommend removing that port from Service spec.ports and routing Ingress explicitly to the HTTP port. If debugging is still required, recommend controlled kubectl port-forward instead of exposing JDWP through Service or Ingress.\n")
	builder.WriteString("Sanitized workload/Pod, Service and Ingress manifests are supplied below when collection succeeds. Cross-check workload container ports, commands, args, probes and resources against Service selector/ports/targetPorts/nodePorts and Ingress backend service.port. Prefer these live manifests over assumptions. Do not claim an Ingress selects the first Service port unless the manifest proves it.\n")
	if evidence.Workload != nil {
		builder.WriteString("This request was initiated from a Kubernetes workload. Treat the selected Pod as the representative or most abnormal sample and distinguish workload-level evidence from single-Pod evidence.\n")
	}
	if evidence.MetricsTrend != nil && evidence.MetricsTrend.Available {
		builder.WriteString("Prometheus/VictoriaMetrics metricsTrend is already collected from the cluster-bound monitor datasource. Use evidence.metricsTrend for CPU, memory WSS and network trend. Do not recommend installing or补采 metrics-server, cAdvisor, Prometheus, or monitoring data just because metrics.k8s.io current metrics failed.\n")
	}
	if strings.TrimSpace(extraPrompt) != "" {
		builder.WriteString("\nAdditional operator question:\n")
		builder.WriteString(strings.TrimSpace(extraPrompt))
		builder.WriteString("\n")
	}
	builder.WriteString("\nEvidence summary JSON:\n")
	builder.Write(summaryJSON)
	builder.WriteString("\n")
	if len(evidence.Manifests) > 0 {
		builder.WriteString("\nSanitized live Kubernetes manifests (status, managedFields and sensitive values removed):\n")
		remainingChars := 18000
		for _, manifest := range evidence.Manifests {
			if remainingChars <= 0 {
				break
			}
			yamlText := manifest.YAML
			if len([]rune(yamlText)) > remainingChars {
				yamlText = truncateForPrompt(yamlText, remainingChars)
			}
			builder.WriteString(fmt.Sprintf("\n--- # %s %s/%s\n", manifest.Kind, manifest.Namespace, manifest.Name))
			builder.WriteString(yamlText)
			builder.WriteByte('\n')
			remainingChars -= len([]rune(yamlText))
		}
	}
	if len(evidence.LogFindings) > 0 {
		builder.WriteString("\nHigh-priority application log findings, use only the top ones:\n")
		for _, finding := range firstK8sPodLogFindings(evidence.LogFindings, 3) {
			builder.WriteString(fmt.Sprintf("line=%d time=%s level=%s kind=%s message=%s\n",
				finding.LineNo,
				finding.Timestamp,
				finding.Level,
				finding.Kind,
				redactSensitiveText(truncateForPrompt(finding.Message, 900)),
			))
			for _, line := range firstStrings(finding.Context, 1) {
				builder.WriteString("  context: ")
				builder.WriteString(redactSensitiveText(truncateForPrompt(line, 900)))
				builder.WriteByte('\n')
			}
		}
	}
	if len(events) > 0 {
		builder.WriteString("\nKubernetes events, priority ordered:\n")
		for _, event := range events {
			builder.WriteString(fmt.Sprintf("[%s] type=%s reason=%s count=%d last=%s message=%s\n",
				event.InvolvedObject.Name,
				event.Type,
				event.Reason,
				event.Count,
				k8sEventLastTime(event).Format(time.RFC3339),
				redactSensitiveText(truncateForPrompt(event.Message, 800)),
			))
		}
	}
	if len(logs) > 0 {
		builder.WriteString("\nSelected current container logs:\n")
		for _, entry := range logs {
			builder.WriteString(redactSensitiveText(formatLogEntryForPrompt(entry)))
			builder.WriteByte('\n')
		}
	}
	if len(previousLogs) > 0 {
		builder.WriteString("\nSelected previous container logs:\n")
		for _, entry := range previousLogs {
			builder.WriteString(redactSensitiveText(formatLogEntryForPrompt(entry)))
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}

func buildK8sIngressAnalysisPrompt(extraPrompt string, evidence aiopsmodel.K8sIngressAnalyzeEvidenceSummary, logs []aiopsmodel.LogEntry, events []corev1.Event) string {
	logs = limitEntriesForPrompt(logs, 120, 10000)
	if len(events) > 80 {
		events = events[:80]
	}
	summaryJSON, _ := json.Marshal(evidence)
	var builder strings.Builder
	builder.WriteString("You are an experienced Kubernetes SRE. Diagnose this Ingress/domain access issue using Ingress rules, Service, Endpoints, backend Pod status, events and selected logs. Answer in Chinese.\n")
	builder.WriteString("Use this flow: 1. Determine whether host/path matches an Ingress rule. 2. Verify backend Service name and port. 3. Check Endpoints readiness and Service selector. 4. Check backend Pod phase/readiness/restarts. 5. For HTTP 4xx, distinguish route/path/host/auth/method issues from backend application responses. 6. For HTTP 5xx, decide whether the error is likely from Ingress controller, Service routing, or backend application. 7. Recommend concrete next checks.\n")
	builder.WriteString("If live HTTP HEAD probe returns 2xx/3xx while the operator entered 4xx/5xx, state clearly that the current live probe is normal and the entered symptom may be stale, environment-specific, intermittent, or caused by client-side/external path differences.\n")
	builder.WriteString("Output format must include these Markdown sections in Chinese: ### 事件总结, ### 严重性评估, ### 可能的根本原因, ### 解决办法. Under ### 事件总结, use one sentence only. Under ### 解决办法, use 3 to 5 numbered items.\n")
	builder.WriteString("Do not invent facts. If external DNS/LB/firewall evidence is missing, explicitly say it needs external verification.\n")
	if strings.TrimSpace(extraPrompt) != "" {
		builder.WriteString("\nAdditional operator question:\n")
		builder.WriteString(strings.TrimSpace(extraPrompt))
		builder.WriteString("\n")
	}
	builder.WriteString("\nIngress evidence summary JSON:\n")
	builder.Write(summaryJSON)
	builder.WriteString("\n")
	if len(events) > 0 {
		builder.WriteString("\nIngress/Service events:\n")
		for _, event := range events {
			builder.WriteString(redactSensitiveText(formatK8sEventOneLine(event)))
			builder.WriteByte('\n')
		}
	}
	if len(logs) > 0 {
		builder.WriteString("\nSelected backend Pod logs:\n")
		for _, entry := range logs {
			builder.WriteString(redactSensitiveText(formatLogEntryForPrompt(entry)))
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}

func limitEntriesForPrompt(entries []aiopsmodel.LogEntry, maxLines, maxChars int) []aiopsmodel.LogEntry {
	if len(entries) > maxLines {
		entries = entries[len(entries)-maxLines:]
	}
	result := make([]aiopsmodel.LogEntry, 0, len(entries))
	total := 0
	for _, entry := range entries {
		line := formatLogEntryForPrompt(entry)
		if total+len(line) > maxChars {
			break
		}
		result = append(result, entry)
		total += len(line) + 1
	}
	return result
}

func formatLogEntryForPrompt(entry aiopsmodel.LogEntry) string {
	prefix := ""
	if entry.Timestamp != "" || entry.Level != "" {
		prefix = strings.TrimSpace(entry.Timestamp + " " + entry.Level)
	}
	raw := truncateForPrompt(entry.Raw, 1000)
	if prefix == "" {
		return raw
	}
	return strings.TrimSpace(prefix + " " + raw)
}

func inferK8sPodSeverity(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) string {
	if len(criticalK8sPodLogFindings(evidence)) > 0 {
		return "warning"
	}
	if evidence.Pod.Phase == string(corev1.PodFailed) || evidence.WarningEvents > 0 || evidence.PreviousLogLines > 0 {
		return "warning"
	}
	if evidence.MetricsTrend != nil && evidence.MetricsTrend.Available {
		if (evidence.MetricsTrend.CPUUsageRate != nil && evidence.MetricsTrend.CPUUsageRate.NearLimit) ||
			(evidence.MetricsTrend.MemoryWSSUsageRate != nil && evidence.MetricsTrend.MemoryWSSUsageRate.NearLimit) {
			return "warning"
		}
	}
	for _, container := range evidence.Pod.Containers {
		if container.State == "waiting" || container.State == "terminated" || container.RestartCount > 0 {
			return "warning"
		}
	}
	for _, reason := range evidence.SuspectedReasons {
		switch normalizeLogLevel(reason) {
		case "ERROR", "FATAL", "PANIC":
			return "warning"
		}
	}
	return "info"
}

func inferK8sPodDiagnosisStatus(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) string {
	if len(criticalK8sPodLogFindings(evidence)) > 0 {
		return "abnormal"
	}
	switch evidence.Pod.Phase {
	case string(corev1.PodFailed), string(corev1.PodPending), string(corev1.PodUnknown):
		return "abnormal"
	}
	if evidence.WarningEvents > 0 || evidence.PreviousLogLines > 0 || len(evidence.ResourceHotspots) > 0 {
		return "abnormal"
	}
	if evidence.MetricsTrend != nil && evidence.MetricsTrend.Available {
		if (evidence.MetricsTrend.CPUUsageRate != nil && evidence.MetricsTrend.CPUUsageRate.NearLimit) ||
			(evidence.MetricsTrend.MemoryWSSUsageRate != nil && evidence.MetricsTrend.MemoryWSSUsageRate.NearLimit) {
			return "abnormal"
		}
	}
	if len(evidence.PVCs) > 0 || len(evidence.Quotas) > 0 {
		for _, reason := range evidence.SuspectedReasons {
			if strings.HasPrefix(reason, "PVC_") || strings.HasPrefix(reason, "PV_") || strings.HasPrefix(reason, "QUOTA_") || strings.HasPrefix(reason, "NODE_") {
				return "abnormal"
			}
		}
	}
	for _, container := range evidence.Pod.Containers {
		if container.State == "waiting" || container.State == "terminated" || container.RestartCount > 0 {
			return "abnormal"
		}
		if evidence.Pod.Phase == string(corev1.PodRunning) && !container.Ready {
			return "abnormal"
		}
		if reasonLooksAbnormal(container.Reason) || reasonLooksAbnormal(container.LastReason) {
			return "abnormal"
		}
	}
	for _, reason := range evidence.SuspectedReasons {
		if reasonLooksAbnormal(reason) {
			return "abnormal"
		}
	}
	if evidence.Pod.Phase == string(corev1.PodRunning) || evidence.Pod.Phase == string(corev1.PodSucceeded) {
		return "normal"
	}
	return "unknown"
}

func inferK8sIngressSeverity(evidence aiopsmodel.K8sIngressAnalyzeEvidenceSummary) string {
	if evidence.HTTPProbe != nil && evidence.HTTPProbe.StatusCode >= 200 && evidence.HTTPProbe.StatusCode < 400 && evidence.WarningEvents == 0 {
		return "info"
	}
	if evidence.StatusCode >= 500 || evidence.WarningEvents > 0 {
		return "warning"
	}
	for _, reason := range evidence.SuspectedReasons {
		if reasonLooksAbnormal(reason) || strings.Contains(reason, "ENDPOINTS") || strings.Contains(reason, "SERVICE_") || strings.Contains(reason, "BACKEND_") {
			return "warning"
		}
	}
	return "info"
}

func inferK8sIngressDiagnosisStatus(evidence aiopsmodel.K8sIngressAnalyzeEvidenceSummary) string {
	if evidence.HTTPProbe != nil && evidence.HTTPProbe.StatusCode >= 200 && evidence.HTTPProbe.StatusCode < 400 && evidence.WarningEvents == 0 {
		return "normal"
	}
	if evidence.StatusCode >= 500 || evidence.WarningEvents > 0 {
		return "abnormal"
	}
	for _, backend := range evidence.MatchedBackends {
		if backend.EndpointReady == 0 || len(backend.SuspectedReasons) > 0 {
			return "abnormal"
		}
	}
	for _, reason := range evidence.SuspectedReasons {
		if reasonLooksAbnormal(reason) || strings.Contains(reason, "NO_") || strings.Contains(reason, "EMPTY") {
			return "abnormal"
		}
	}
	if len(evidence.MatchedBackends) > 0 {
		return "normal"
	}
	return "unknown"
}

func localizeDiagnosisStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "normal":
		return "正常"
	case "abnormal":
		return "异常"
	case "unknown":
		return "未知"
	default:
		return status
	}
}

func localizeDiagnosisSeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "info":
		return "提示"
	case "warning":
		return "警告"
	case "critical":
		return "严重"
	default:
		return severity
	}
}

func reasonLooksAbnormal(reason string) bool {
	reason = strings.TrimSpace(reason)
	if reason == "" || strings.EqualFold(reason, "Completed") {
		return false
	}
	if _, ok := k8sWarningEventReasons[reason]; ok {
		return true
	}
	switch normalizeLogLevel(reason) {
	case "ERROR", "FATAL", "PANIC", "WARN":
		return true
	}
	return k8sEvidenceKeywordRegex.MatchString(reason)
}

func firstNonEmptyLine(value string) string {
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(strings.Trim(line, "#*- "))
		if line != "" {
			return truncateForPrompt(line, 300)
		}
	}
	return ""
}

func compactK8sPodAnalyzeResult(result *aiopsmodel.K8sPodAnalyzeResult) *aiopsmodel.K8sPodAnalyzeResult {
	if result == nil {
		return nil
	}
	compact := *result
	compact.Evidence = compactK8sPodEvidenceSummary(result.Evidence)

	summary := strings.TrimSpace(result.Summary)
	if summary == "" {
		summary = extractK8sPodAnalysisSectionLine(result.Analysis, "问题描述")
	}
	if summary == "" {
		summary = extractK8sPodAnalysisSectionLine(result.Analysis, "事件总结")
	}
	if summary == "" {
		summary = buildK8sPodFallbackSummary(compact.Evidence)
	}
	summary = cleanK8sPodDisplayText(summary, 420)

	rootCause := buildK8sPodKnownRootCause(compact.Evidence)
	if rootCause == "" {
		rootCause = extractK8sPodAnalysisSectionLine(result.Analysis, "问题原因")
	}
	if rootCause == "" {
		rootCause = extractK8sPodAnalysisSectionLine(result.Analysis, "根本原因")
	}
	if rootCause == "" {
		rootCause = extractK8sPodAnalysisSectionLine(result.Analysis, "可能的根本原因")
	}
	if rootCause == "" {
		rootCause = buildK8sPodCompactRootCause(compact.Evidence)
	}
	rootCause = cleanK8sPodDisplayText(rootCause, 420)

	severity := strings.TrimSpace(result.Severity)
	if severity == "" {
		severity = localizeDiagnosisSeverity(inferK8sPodSeverity(compact.Evidence))
	}
	remediation := result.Remediation
	if hasK8sPodLogFindingKind(compact.Evidence, "HTTP_TO_JDWP_PORT") {
		remediation = buildK8sPodFallbackRemediation(compact.Evidence)
	}
	if len(remediation) == 0 {
		remediation = extractK8sPodAnalysisSectionBullets(result.Analysis, "解决方案", 3)
	}
	if len(remediation) == 0 {
		remediation = extractK8sPodAnalysisSectionBullets(result.Analysis, "解决办法", 3)
	}
	if len(remediation) == 0 {
		remediation = buildK8sPodFallbackRemediation(compact.Evidence)
	}
	compact.Remediation = compactK8sPodRemediation(remediation, 5)

	compact.Summary = summary
	compact.Conclusion = buildK8sPodFinalConclusion(compact.Evidence, summary, rootCause, severity)
	compact.Analysis = buildK8sPodCompactAnalysis(summary, rootCause, compact.Remediation, compact.Conclusion)
	return &compact
}

func compactK8sPodEvidenceSummary(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) aiopsmodel.K8sPodAnalyzeEvidenceSummary {
	compact := evidence
	compact.LogFindings = compactK8sPodLogFindings(evidence.LogFindings, 3)
	compact.CollectionErrors = compactStringSlice(evidence.CollectionErrors, 3, 120)
	if compact.MetricsTrend != nil {
		metricsTrend := *compact.MetricsTrend
		metricsTrend.Pods = compactStringSlice(metricsTrend.Pods, 5, 120)
		metricsTrend.Warnings = compactStringSlice(metricsTrend.Warnings, 3, 120)
		compact.MetricsTrend = &metricsTrend
	}
	if compact.Node != nil {
		node := *compact.Node
		node.Conditions = nil
		node.Capacity = nil
		node.Allocatable = nil
		node.SuspectedReasons = compactStringSlice(node.SuspectedReasons, 3, 80)
		compact.Node = &node
	}
	if len(compact.Quotas) > 3 {
		compact.Quotas = compact.Quotas[:3]
	}
	if len(compact.PVCs) > 3 {
		compact.PVCs = compact.PVCs[:3]
	}
	if len(compact.Probes) > 3 {
		compact.Probes = compact.Probes[:3]
	}
	if len(compact.Services) > 5 {
		compact.Services = compact.Services[:5]
	}
	if len(compact.IngressRoutes) > 5 {
		compact.IngressRoutes = compact.IngressRoutes[:5]
	}
	if len(evidence.Manifests) > 0 {
		compact.Manifests = make([]aiopsmodel.K8sPodAnalyzeManifestInfo, 0, minInt(len(evidence.Manifests), 5))
		for _, manifest := range evidence.Manifests {
			manifest.YAML = truncateForPrompt(manifest.YAML, 6000)
			compact.Manifests = append(compact.Manifests, manifest)
			if len(compact.Manifests) >= 5 {
				break
			}
		}
	}
	compact.MatchedKeywords = compactStringSlice(compact.MatchedKeywords, 6, 40)
	compact.SuspectedReasons = compactStringSlice(compact.SuspectedReasons, 6, 60)
	compact.ResourceHotspots = compactStringSlice(compact.ResourceHotspots, 4, 60)
	return compact
}

func compactK8sPodLogFindings(findings []aiopsmodel.K8sPodAnalyzeLogFinding, limit int) []aiopsmodel.K8sPodAnalyzeLogFinding {
	if len(findings) == 0 || limit <= 0 {
		return nil
	}
	result := make([]aiopsmodel.K8sPodAnalyzeLogFinding, 0, minInt(len(findings), limit))
	for _, finding := range findings {
		item := finding
		item.Message = truncateForPrompt(item.Message, 220)
		item.Context = compactStringSlice(item.Context, 1, 180)
		result = append(result, item)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func compactStringSlice(values []string, limit, maxLen int) []string {
	if len(values) == 0 || limit <= 0 {
		return nil
	}
	result := make([]string, 0, minInt(len(values), limit))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		value = truncateForPrompt(value, maxLen)
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func compactK8sPodRemediation(items []string, limit int) []string {
	if len(items) == 0 || limit <= 0 {
		return nil
	}
	result := make([]string, 0, minInt(len(items), limit))
	seen := make(map[string]struct{})
	for _, item := range items {
		item = cleanK8sPodDisplayText(item, 500)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func buildK8sPodCompactAnalysis(summary, rootCause string, remediation []string, conclusion string) string {
	var builder strings.Builder
	builder.WriteString("### 问题描述\n")
	builder.WriteString(summary)
	builder.WriteString("\n\n### 问题原因\n")
	builder.WriteString(rootCause)
	builder.WriteString("\n\n### 解决方案\n")
	for i, item := range remediation {
		builder.WriteString(fmt.Sprintf("%d. %s\n", i+1, item))
	}
	builder.WriteString("\n### 总结结论\n")
	builder.WriteString(conclusion)
	return strings.TrimSpace(builder.String())
}

func buildK8sPodKnownRootCause(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) string {
	if !hasK8sPodLogFindingKind(evidence, "HTTP_TO_JDWP_PORT") {
		return ""
	}
	if route, confirmed := findK8sPodJDWPRoute(evidence); confirmed {
		return "已确认 HTTP 请求被路由到 JVM JDWP 调试端口，而不是应用 HTTP 端口。JDWP 收到 GET 后无法完成二进制握手，入口层因此返回 502。实际链路：" + route + "。"
	}
	return "日志已确认 HTTP GET 到达 JVM JDWP 调试端口，而不是应用 HTTP 监听端口，因此入口层无法获得合法 HTTP 响应并返回 502。当前未采到能锁定具体对象的 Ingress/Service 后端配置，端口误配位置仍需核对。"
}

func buildK8sPodKeyEvidence(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) []string {
	items := make([]string, 0, 5)
	for _, finding := range firstK8sPodLogFindings(evidence.LogFindings, 3) {
		items = append(items, fmt.Sprintf("日志 `%s`：%s", finding.Kind, cleanK8sPodDisplayText(finding.Message, 220)))
	}
	if route, confirmed := findK8sPodJDWPRoute(evidence); confirmed {
		items = append(items, "路由配置："+route)
	}
	items = append(items, fmt.Sprintf("Pod 状态：phase=%s，restartCount=%d，Ready=%s", evidence.Pod.Phase, evidence.Pod.RestartCount, k8sPodReadyValue(evidence.Pod)))
	if evidence.WarningEvents > 0 {
		items = append(items, fmt.Sprintf("Kubernetes Warning 事件：%d 条", evidence.WarningEvents))
	}
	return dedupeK8sStrings(items, 5)
}

func k8sPodReadyValue(pod aiopsmodel.K8sPodAnalyzePodInfo) string {
	for _, condition := range pod.Conditions {
		if condition.Type == "Ready" {
			return condition.Status
		}
	}
	return "unknown"
}

func buildK8sPodVerificationSteps(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) []string {
	namespace := firstNonEmptyString(evidence.Pod.Namespace, "<namespace>")
	if hasK8sPodLogFindingKind(evidence, "HTTP_TO_JDWP_PORT") {
		steps := []string{
			fmt.Sprintf("执行 `kubectl -n %s get ingress -o yaml`，确认 backend service.port 明确指向 HTTP 端口而非 debug/JDWP。", namespace),
		}
		if len(evidence.Services) > 0 {
			steps = append(steps, fmt.Sprintf("执行 `kubectl -n %s get svc %s -o yaml`，确认业务端口的 targetPort 指向应用 HTTP 监听端口。", namespace, evidence.Services[0].Name))
		} else {
			steps = append(steps, fmt.Sprintf("执行 `kubectl -n %s get svc -o yaml`，核对 selector 命中该 Pod 的 Service 端口映射。", namespace))
		}
		steps = append(steps, "修复后重新请求原 URL，确认 HTTP 状态恢复且容器日志不再出现 `received >GET ...< expected >JDWP-Handshake<`。")
		return steps
	}
	return []string{
		fmt.Sprintf("执行 `kubectl -n %s describe pod %s`，复核 Ready、重启次数和最新事件。", namespace, evidence.Pod.Pod),
		fmt.Sprintf("执行 `kubectl -n %s logs %s --tail=200`，确认关键异常不再出现。", namespace, evidence.Pod.Pod),
	}
}

func buildK8sPodCompactRootCause(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) string {
	if len(evidence.LogFindings) > 0 {
		finding := evidence.LogFindings[0]
		rootCause := fmt.Sprintf("最强证据来自应用日志：%s，需优先排查对应调用链或代码路径。", truncateForPrompt(finding.Message, 140))
		if trend := formatK8sPodMetricsTrendRootCause(evidence); trend != "" {
			rootCause = strings.TrimRight(rootCause, "。") + "；" + trend + "。"
		}
		return rootCause
	}
	if evidence.WarningEvents > 0 {
		rootCause := "Kubernetes Warning 事件显示 Pod 存在探针、调度、挂载或运行时异常，需要结合事件原因定位。"
		if trend := formatK8sPodMetricsTrendRootCause(evidence); trend != "" {
			rootCause = strings.TrimRight(rootCause, "。") + "；" + trend + "。"
		}
		return rootCause
	}
	if len(evidence.ResourceHotspots) > 0 {
		return "资源指标存在热点，可能导致应用响应慢、探针失败或线程阻塞。"
	}
	if trend := formatK8sPodMetricsTrendRootCause(evidence); trend != "" {
		return trend + "。"
	}
	return "当前证据不足以确认单一根因，需要补充日志、事件或指标。"
}

func buildK8sPodFinalConclusion(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary, summary, rootCause, severity string) string {
	target := evidence.Pod.Pod
	if evidence.Workload != nil && evidence.Workload.Name != "" {
		target = evidence.Workload.Type + " " + evidence.Workload.Namespace + "/" + evidence.Workload.Name
	}
	state := "需要关注"
	if k8sPodEvidenceCurrentlyAvailable(evidence) && (evidence.WarningEvents > 0 || len(evidence.LogFindings) > 0 || evidence.PreviousLogLines > 0) {
		state = "可用但存在风险"
	} else if strings.Contains(severity, "高") || strings.Contains(evidence.Pod.Phase, "Pending") || evidence.WarningEvents > 0 || len(evidence.LogFindings) > 0 {
		state = "异常"
	} else if strings.Contains(severity, "低") && evidence.Pod.Phase == string(corev1.PodRunning) {
		state = "基本正常"
	}
	parts := []string{fmt.Sprintf("最终结论：%s 当前%s", target, state)}
	if impact := buildK8sPodImpactConclusion(evidence); impact != "" {
		parts = append(parts, impact)
	}
	cause := cleanK8sPodDisplayText(rootCause, 260)
	if cause != "" {
		parts = append(parts, "直接原因："+cause)
	} else if symptom := cleanK8sPodDisplayText(summary, 220); symptom != "" {
		parts = append(parts, "现象："+symptom)
	}
	reasons := buildK8sPodFinalConclusionReasons(evidence)
	if cause != "" {
		filtered := reasons[:0]
		for _, reason := range reasons {
			if strings.Contains(reason, "Warning 事件") {
				continue
			}
			filtered = append(filtered, reason)
		}
		reasons = filtered
	}
	if len(reasons) > 0 {
		parts = append(parts, "异常信号："+strings.Join(reasons, "；"))
	}
	if cause == "" && len(reasons) == 0 {
		parts = append(parts, "当前证据不足以确认单一根因")
	}
	parts = append(parts, buildK8sPodRecoveryConclusion(evidence, rootCause))
	return cleanK8sPodDisplayText(strings.Join(parts, "。")+"。", 1000)
}

func buildK8sPodImpactConclusion(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) string {
	if evidence.Workload != nil {
		total := evidence.Workload.PodCount
		if total <= 0 {
			total = int(evidence.Workload.DesiredReplicas)
		}
		desired := evidence.Workload.DesiredReplicas
		if desired <= 0 {
			desired = int32(total)
		}
		if total > 0 {
			return fmt.Sprintf("影响范围：%d/%d 个 Pod 异常，Ready 副本为 %d/%d", evidence.Workload.AbnormalPodCount, total, evidence.Workload.ReadyReplicas, desired)
		}
	}
	return fmt.Sprintf("影响范围：Pod %s phase=%s，Ready=%s", evidence.Pod.Pod, evidence.Pod.Phase, k8sPodReadyValue(evidence.Pod))
}

func buildK8sPodRecoveryConclusion(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary, rootCause string) string {
	if hasK8sPodLogFindingKind(evidence, "HTTP_TO_JDWP_PORT") {
		return "恢复标准：Service 和 Ingress 明确指向 HTTP 端口，业务请求恢复正常，且 JDWP 日志不再出现 HTTP 握手失败"
	}
	lowerCause := strings.ToLower(rootCause)
	if strings.Contains(lowerCause, "configmap") {
		return "恢复标准：缺失的 ConfigMap 已补齐，volume 挂载成功，所有期望副本均进入 Running/Ready，且相关 Warning 不再新增"
	}
	if strings.Contains(lowerCause, "secret") {
		return "恢复标准：缺失的 Secret 已补齐，引用和挂载成功，所有期望副本均进入 Running/Ready"
	}
	switch evidence.Pod.Phase {
	case string(corev1.PodPending):
		return "恢复标准：配置、挂载或调度问题已修复，所有期望副本均进入 Running/Ready，且相关 Warning 不再新增"
	case string(corev1.PodRunning):
		return "恢复标准：Pod 持续 Running/Ready、业务请求正常，且关键异常不再出现"
	default:
		return "恢复标准：Pod 恢复 Running/Ready、业务可用且异常事件不再新增"
	}
}

func buildK8sPodFinalConclusionReasons(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) []string {
	reasons := make([]string, 0, 4)
	if restartSignal := k8sPodFinalRestartSignal(evidence); restartSignal != "" {
		reasons = append(reasons, restartSignal)
	}
	if logSignal := k8sPodFinalLogSignal(evidence); logSignal != "" {
		reasons = append(reasons, logSignal)
	}
	if trend := formatK8sPodMetricsTrendConclusion(evidence); trend != "" {
		reasons = append(reasons, trend)
	}
	if len(reasons) == 0 && evidence.WarningEvents > 0 {
		reasons = append(reasons, fmt.Sprintf("存在 %d 条 Warning 事件", evidence.WarningEvents))
	}
	return reasons
}

func k8sPodFinalRestartSignal(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) string {
	restarts := int32(0)
	oomKilled := false
	for _, container := range evidence.Pod.Containers {
		restarts += container.RestartCount
		if strings.EqualFold(container.LastReason, "OOMKilled") || strings.EqualFold(container.Reason, "OOMKilled") {
			oomKilled = true
		}
	}
	if restarts == 0 {
		restarts = evidence.Pod.RestartCount
	}
	if oomKilled && restarts > 0 {
		return fmt.Sprintf("历史发生过 OOMKilled，容器重启 %d 次", restarts)
	}
	if oomKilled {
		return "历史发生过 OOMKilled"
	}
	if restarts > 0 {
		return fmt.Sprintf("容器历史重启 %d 次", restarts)
	}
	return ""
}

func k8sPodFinalLogSignal(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) string {
	if len(evidence.LogFindings) == 0 {
		return ""
	}
	finding := evidence.LogFindings[0]
	message := cleanK8sPodDisplayText(finding.Message, 70)
	lower := strings.ToLower(message)
	switch {
	case finding.Kind == "HTTP_TO_JDWP_PORT":
		return "HTTP 请求到达 JDWP 调试端口，入口端口路由错误"
	case strings.Contains(lower, "stream_chunk") && strings.Contains(lower, "not found"):
		return "当前日志有 stream_chunk not found"
	case strings.Contains(lower, "not found"):
		return "当前日志有 not found 异常"
	case finding.Kind != "":
		return "当前日志存在 " + finding.Kind + "：" + message
	default:
		return "当前日志存在异常：" + message
	}
}

func cleanK8sPodDisplayText(value string, limit int) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "...<truncated>", "")
	value = strings.ReplaceAll(value, "<truncated>", "")
	value = strings.Join(strings.Fields(value), " ")
	value = strings.Trim(value, "。；;,.， ")
	if limit <= 0 || value == "" {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return strings.TrimRight(string(runes[:limit]), "。；;,.， ") + "..."
}

func k8sPodEvidenceCurrentlyAvailable(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) bool {
	if evidence.Workload != nil && evidence.Workload.AvailableReplicas > 0 {
		return true
	}
	if evidence.Pod.Phase != string(corev1.PodRunning) {
		return false
	}
	for _, condition := range evidence.Pod.Conditions {
		if condition.Type == "Ready" && strings.EqualFold(condition.Status, "True") {
			return true
		}
	}
	for _, container := range evidence.Pod.Containers {
		if container.Ready {
			return true
		}
	}
	return false
}

func formatK8sPodMetricsTrendRootCause(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) string {
	if !k8sPodMetricsTrendAvailable(evidence) {
		return ""
	}
	trend := evidence.MetricsTrend
	parts := make([]string, 0, 2)
	if trend.MemoryWSSUsageRate != nil {
		parts = append(parts, "近 "+firstNonEmptyString(trend.Range, "6h")+" 内存 WSS "+formatK8sPodMetricRange(trend.MemoryWSSUsageRate)+formatK8sPodMetricLimitText(trend.MemoryWSSUsageRate, firstK8sPodMemoryLimit(evidence.Pod)))
	}
	if trend.CPUUsageRate != nil {
		parts = append(parts, "CPU "+formatK8sPodMetricRange(trend.CPUUsageRate)+formatK8sPodMetricLimitText(trend.CPUUsageRate, ""))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "，")
}

func formatK8sPodMetricsTrendConclusion(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) string {
	if !k8sPodMetricsTrendAvailable(evidence) {
		return ""
	}
	trend := evidence.MetricsTrend
	if trend.MemoryWSSUsageRate != nil {
		return "近 " + firstNonEmptyString(trend.Range, "6h") + " 内存 WSS 约 " + formatK8sPodMetricRange(trend.MemoryWSSUsageRate) + formatK8sPodMetricLimitText(trend.MemoryWSSUsageRate, firstK8sPodMemoryLimit(evidence.Pod))
	}
	if trend.CPUUsageRate != nil {
		return "近 " + firstNonEmptyString(trend.Range, "6h") + " CPU 约 " + formatK8sPodMetricRange(trend.CPUUsageRate) + formatK8sPodMetricLimitText(trend.CPUUsageRate, "")
	}
	return ""
}

func k8sPodMetricsTrendAvailable(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) bool {
	return evidence.MetricsTrend != nil && evidence.MetricsTrend.Available
}

func formatK8sPodMetricRange(stat *aiopsmodel.K8sPodAnalyzeMetricTrendStat) string {
	if stat == nil {
		return ""
	}
	unit := stat.Unit
	if unit == "" {
		unit = "%"
	}
	if stat.Min == stat.Peak {
		return formatK8sPodMetricNumber(stat.Peak) + unit
	}
	return formatK8sPodMetricNumber(stat.Min) + unit + "-" + formatK8sPodMetricNumber(stat.Peak) + unit
}

func formatK8sPodMetricLimitText(stat *aiopsmodel.K8sPodAnalyzeMetricTrendStat, limit string) string {
	if stat == nil || stat.Unit != "%" {
		return ""
	}
	limit = strings.TrimSpace(limit)
	if limit != "" {
		limit = " " + limit
	}
	if stat.NearLimit {
		return "，已接近" + limit + " limit"
	}
	return "，未接近" + limit + " limit"
}

func formatK8sPodMetricNumber(value float64) string {
	text := strconv.FormatFloat(value, 'f', 2, 64)
	text = strings.TrimRight(text, "0")
	text = strings.TrimRight(text, ".")
	if text == "" || text == "-0" {
		return "0"
	}
	return text
}

func firstK8sPodMemoryLimit(pod aiopsmodel.K8sPodAnalyzePodInfo) string {
	if strings.TrimSpace(pod.Resources.MemoryLimit) != "" {
		return pod.Resources.MemoryLimit
	}
	for _, container := range pod.Containers {
		if strings.TrimSpace(container.Resources.MemoryLimit) != "" {
			return container.Resources.MemoryLimit
		}
	}
	return ""
}

func firstK8sPodLogFindings(findings []aiopsmodel.K8sPodAnalyzeLogFinding, limit int) []aiopsmodel.K8sPodAnalyzeLogFinding {
	if len(findings) <= limit {
		return findings
	}
	return findings[:limit]
}

func firstStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

func ensureK8sPodAnalysisIncludesLogFindings(analysis string, evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) string {
	critical := criticalK8sPodLogFindings(evidence)
	if len(critical) == 0 || analysisMentionsK8sPodLogFindings(analysis, critical) {
		return analysis
	}

	var builder strings.Builder
	builder.WriteString("### 关键异常日志\n")
	for _, finding := range critical {
		builder.WriteString(fmt.Sprintf("- `%s` line=%d", finding.Kind, finding.LineNo))
		if finding.Level != "" {
			builder.WriteString(fmt.Sprintf(" level=%s", finding.Level))
		}
		if finding.Timestamp != "" {
			builder.WriteString(fmt.Sprintf(" time=%s", finding.Timestamp))
		}
		builder.WriteString("：")
		builder.WriteString(truncateForPrompt(finding.Message, 500))
		builder.WriteByte('\n')
	}
	builder.WriteString("\n")
	builder.WriteString(strings.TrimSpace(analysis))
	return builder.String()
}

func criticalK8sPodLogFindings(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) []aiopsmodel.K8sPodAnalyzeLogFinding {
	items := make([]aiopsmodel.K8sPodAnalyzeLogFinding, 0, len(evidence.LogFindings))
	for _, finding := range evidence.LogFindings {
		switch finding.Kind {
		case "HTTP_TO_JDWP_PORT", "APPLICATION_ERROR", "DUBBO_NO_PROVIDER", "RPC_EXCEPTION", "EXCEPTION", "TIMEOUT", "CONNECTION_REFUSED":
			items = append(items, finding)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return k8sPodLogFindingPriority(items[i].Kind) > k8sPodLogFindingPriority(items[j].Kind)
	})
	if len(items) > 5 {
		items = items[:5]
	}
	return items
}

func k8sPodLogFindingPriority(kind string) int {
	switch kind {
	case "HTTP_TO_JDWP_PORT":
		return 110
	case "DUBBO_NO_PROVIDER":
		return 100
	case "RPC_EXCEPTION":
		return 90
	case "TIMEOUT", "CONNECTION_REFUSED":
		return 80
	case "APPLICATION_ERROR":
		return 70
	case "EXCEPTION":
		return 60
	default:
		return 0
	}
}

func analysisMentionsK8sPodLogFindings(analysis string, findings []aiopsmodel.K8sPodAnalyzeLogFinding) bool {
	lower := strings.ToLower(analysis)
	for _, finding := range findings {
		if finding.Kind != "" && strings.Contains(lower, strings.ToLower(finding.Kind)) {
			return true
		}
		for _, token := range []string{"jdwp-handshake", "http_to_jdwp_port", "debugger failed to attach", "no provider available", "rpcexception", "bigdata_error_log", "application_error", "dubbo_no_provider"} {
			if strings.Contains(lower, token) {
				return true
			}
		}
	}
	return false
}

func extractK8sPodAnalysisSummaryAndRemediation(analysis string, evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) (string, []string) {
	summary := extractK8sPodAnalysisSectionLine(analysis, "问题描述")
	if summary == "" {
		summary = extractK8sPodAnalysisSectionLine(analysis, "事件总结")
	}
	if summary == "" {
		summary = buildK8sPodFallbackSummary(evidence)
	}
	if logSummary := buildK8sPodLogFindingSummary(evidence); logSummary != "" && !summaryMentionsK8sPodLogFindings(summary, evidence) {
		summary = logSummary
	}

	remediation := extractK8sPodAnalysisSectionBullets(analysis, "解决方案", 5)
	if len(remediation) == 0 {
		remediation = extractK8sPodAnalysisSectionBullets(analysis, "解决办法", 5)
	}
	if len(remediation) == 0 {
		remediation = buildK8sPodFallbackRemediation(evidence)
	}
	return summary, remediation
}

func buildK8sPodLogFindingSummary(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) string {
	critical := criticalK8sPodLogFindings(evidence)
	if len(critical) == 0 {
		return ""
	}
	finding := critical[0]
	podState := k8sPodReadinessSummary(evidence.Pod)
	switch finding.Kind {
	case "HTTP_TO_JDWP_PORT":
		if route, confirmed := findK8sPodJDWPRoute(evidence); confirmed {
			return fmt.Sprintf("Pod %s %s，但 HTTP 请求被转发到 JVM JDWP 调试端口；路由证据：%s。", evidence.Pod.Pod, podState, route)
		}
		return fmt.Sprintf("Pod %s %s，但 JDWP 日志收到 HTTP 请求，业务流量很可能被错误转发到 JVM 调试端口。", evidence.Pod.Pod, podState)
	case "DUBBO_NO_PROVIDER":
		return fmt.Sprintf("Pod %s %s，应用日志存在 Dubbo 无可用 Provider 错误，需优先排查下游服务注册与可用实例。", evidence.Pod.Pod, podState)
	case "RPC_EXCEPTION":
		return fmt.Sprintf("Pod %s %s，应用日志存在 RPC 调用异常，需优先排查下游服务可用性和注册中心状态。", evidence.Pod.Pod, podState)
	case "APPLICATION_ERROR":
		return fmt.Sprintf("Pod %s %s，应用日志存在 ERROR 级业务异常，需优先定位错误接口和异常栈。", evidence.Pod.Pod, podState)
	case "TIMEOUT":
		return fmt.Sprintf("Pod %s %s，应用日志存在超时异常，需优先排查下游依赖、网络和接口耗时。", evidence.Pod.Pod, podState)
	case "CONNECTION_REFUSED":
		return fmt.Sprintf("Pod %s %s，应用日志存在连接拒绝异常，需优先排查目标服务监听、Endpoint 和网络连通性。", evidence.Pod.Pod, podState)
	default:
		return fmt.Sprintf("Pod %s %s，应用日志存在关键异常 %s，需优先排查应用调用链。", evidence.Pod.Pod, podState, finding.Kind)
	}
}

func k8sPodReadinessSummary(pod aiopsmodel.K8sPodAnalyzePodInfo) string {
	ready := false
	for _, condition := range pod.Conditions {
		if condition.Type == "Ready" && strings.EqualFold(condition.Status, "True") {
			ready = true
			break
		}
	}
	if pod.Phase == string(corev1.PodRunning) && ready {
		return "当前 Running/Ready"
	}
	if pod.Phase == string(corev1.PodRunning) {
		return "当前 Running 但未 Ready"
	}
	if pod.Phase != "" {
		return "当前 " + pod.Phase
	}
	return "当前状态未知"
}

func summaryMentionsK8sPodLogFindings(summary string, evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) bool {
	return analysisMentionsK8sPodLogFindings(summary, criticalK8sPodLogFindings(evidence))
}

func extractK8sIngressAnalysisSummaryAndRemediation(analysis string, evidence aiopsmodel.K8sIngressAnalyzeEvidenceSummary) (string, []string) {
	summary := extractK8sPodAnalysisSectionLine(analysis, "事件总结")
	if summary == "" {
		summary = buildK8sIngressFallbackSummary(evidence)
	}
	remediation := extractK8sPodAnalysisSectionBullets(analysis, "解决办法", 5)
	if len(remediation) == 0 {
		remediation = buildK8sIngressFallbackRemediation(evidence)
	}
	return summary, remediation
}

func buildK8sIngressFallbackSummary(evidence aiopsmodel.K8sIngressAnalyzeEvidenceSummary) string {
	target := evidence.Host
	if target == "" {
		target = evidence.Ingress
	}
	if evidence.StatusCode >= 500 {
		return fmt.Sprintf("Ingress %s 访问出现 HTTP %d，需要沿 Ingress、Service、Endpoint 和后端 Pod 链路定位。", target, evidence.StatusCode)
	}
	for _, backend := range evidence.MatchedBackends {
		if backend.EndpointReady == 0 {
			return fmt.Sprintf("Ingress %s 的后端 Service %s 当前没有就绪 Endpoint。", target, backend.ServiceName)
		}
	}
	return fmt.Sprintf("Ingress %s 的链路证据已采集，需结合规则匹配、后端服务和应用日志确认异常位置。", target)
}

func buildK8sIngressFallbackRemediation(evidence aiopsmodel.K8sIngressAnalyzeEvidenceSummary) []string {
	items := []string{
		"先确认域名和路径是否命中预期 Ingress 规则，并核对 backend serviceName/servicePort。",
		"检查 Service selector、Endpoints/EndpointSlice 是否存在就绪后端。",
		"查看后端 Pod 的 Ready 状态、重启次数和最近错误日志，判断 5xx 是否由应用返回。",
	}
	if len(evidence.LoadBalancers) == 0 {
		items = append(items, "补充确认外部 DNS、LoadBalancer 和防火墙配置是否正确。")
	}
	return dedupeK8sStrings(items, 5)
}

func buildK8sPodFallbackSummary(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) string {
	pod := evidence.Pod
	container := pod.Container
	if container == "" && len(pod.Containers) > 0 {
		container = pod.Containers[0].Name
	}

	switch {
	case pod.RestartCount > 0 && len(evidence.ResourceHotspots) > 0:
		return fmt.Sprintf("Pod %s 的容器 %s 存在重启并伴随资源热点，当前已表现为异常。", pod.Pod, container)
	case pod.RestartCount > 0:
		return fmt.Sprintf("Pod %s 的容器 %s 存在持续重启，当前服务不可用风险较高。", pod.Pod, container)
	case evidence.WarningEvents > 0 || len(evidence.SuspectedReasons) > 0:
		return fmt.Sprintf("Pod %s 存在 Warning 事件或异常线索，建议优先排查容器状态、探针和依赖。", pod.Pod)
	case pod.Phase == string(corev1.PodRunning):
		return fmt.Sprintf("Pod %s 当前处于 Running，但仍需结合日志和事件确认是否存在潜在异常。", pod.Pod)
	default:
		return fmt.Sprintf("Pod %s 当前状态异常，需结合日志、事件和资源信息继续排查。", pod.Pod)
	}
}

func buildK8sPodFallbackRemediation(evidence aiopsmodel.K8sPodAnalyzeEvidenceSummary) []string {
	items := make([]string, 0, 3)
	if hasK8sPodLogFindingKind(evidence, "HTTP_TO_JDWP_PORT") {
		items = append(items,
			"从 Service spec.ports 删除 debug/JDWP 端口，只保留 HTTP、Dubbo 等实际业务端口。",
			"将 Ingress backend service.port 显式改为 Service 的 HTTP 端口名或端口号，并核对 targetPort 与容器应用监听端口一致。",
			"确需远程调试时使用受控的 kubectl port-forward，不要通过 Service 或 Ingress 暴露 JDWP。",
		)
		return items
	}
	if evidence.Pod.RestartCount > 0 || len(evidence.SuspectedReasons) > 0 {
		items = append(items, "先查看 previous 日志和最近的错误日志，确认容器退出前的直接报错。")
		items = append(items, "检查 readiness/liveness 探针、启动参数和端口监听是否正常。")
	}
	if len(evidence.ResourceHotspots) > 0 {
		items = append(items, "结合资源热点检查 CPU/内存限制是否过低，必要时调整 requests/limits。")
	}
	if k8sPodMetricsTrendAvailable(evidence) {
		if evidence.MetricsTrend.MemoryWSSUsageRate != nil && evidence.MetricsTrend.MemoryWSSUsageRate.Rising {
			items = append(items, "结合内存 WSS 上升趋势检查应用堆、缓存、线程和 fd 是否持续增长。")
		} else if evidence.MetricsTrend.MemoryWSSUsageRate != nil && !evidence.MetricsTrend.MemoryWSSUsageRate.NearLimit {
			items = append(items, "内存 WSS 未接近 limit，优先排查应用状态丢失、缓存或业务逻辑异常。")
		}
	}
	for _, reason := range evidence.SuspectedReasons {
		if strings.HasPrefix(reason, "PVC_") || strings.HasPrefix(reason, "PV_") {
			items = append(items, "检查 PVC/PV 绑定、StorageClass provisioner、云盘可用区和挂载权限。")
			break
		}
	}
	for _, reason := range evidence.SuspectedReasons {
		if strings.HasPrefix(reason, "QUOTA_") || strings.HasPrefix(reason, "NODE_") || strings.HasPrefix(reason, "POD_NOT_SCHEDULED") {
			items = append(items, "核对 Namespace 配额、Node Ready/压力/污点和调度事件，确认 Pending 是否由调度或资源约束导致。")
			break
		}
	}
	if len(evidence.CollectionErrors) > 0 && !k8sPodMetricsTrendAvailable(evidence) {
		items = append(items, "补采集缺失的事件或指标数据，避免因为证据不足误判根因。")
	}
	if len(items) == 0 {
		items = append(items, "先核对容器状态、事件和日志，再决定是否需要重启或扩容。")
		items = append(items, "如果是依赖异常，继续排查下游服务、配置中心和注册中心。")
	}
	return dedupeK8sStrings(items, 3)
}

func extractK8sPodAnalysisSectionLine(content, heading string) string {
	items := extractK8sPodAnalysisSectionBullets(content, heading, 1)
	if len(items) > 0 {
		return items[0]
	}
	return ""
}

func extractK8sPodAnalysisSectionBullets(content, heading string, limit int) []string {
	section := extractK8sPodAnalysisSection(content, heading)
	if section == "" {
		return nil
	}

	lines := strings.Split(section, "\n")
	result := make([]string, 0, limit)
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		line = strings.TrimLeft(line, "-*•0123456789.)、 ")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, "###") && !strings.Contains(line, heading) {
			continue
		}
		result = append(result, truncateForPrompt(line, 240))
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

func extractK8sPodAnalysisSection(content, heading string) string {
	lines := strings.Split(content, "\n")
	start := -1
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "###") && strings.Contains(line, heading) {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}

	end := len(lines)
	for i := start; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "###") && !strings.Contains(line, heading) {
			end = i
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}

func dedupeK8sStrings(items []string, limit int) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func limitAnalysisLines(lines []string, maxLines, maxChars int) []string {
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	result := make([]string, 0, len(lines))
	total := 0
	for _, line := range lines {
		line = truncateForPrompt(line, 1000)
		if total+len(line) > maxChars {
			break
		}
		result = append(result, line)
		total += len(line) + 1
	}
	return result
}

func redactSensitiveText(value string) string {
	for _, re := range secretRegexes {
		value = re.ReplaceAllString(value, "${1}<redacted>")
	}
	return value
}

func truncateForPrompt(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 {
		return ""
	}
	if len(value) <= limit {
		return value
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "...<truncated>"
}

func CallLLM(ctx context.Context, provider *aiopsmodel.LLMProvider, apiKey, systemPrompt, userPrompt string) (string, error) {
	if provider.ProviderType == aiopsmodel.LLMProviderTypeCodex && strings.TrimSpace(provider.BaseURL) == "" {
		return "", errors.New("codex cli provider cannot be called by backend HTTP analysis")
	}
	switch provider.ProviderType {
	case aiopsmodel.LLMProviderTypeAnthropic:
		return callAnthropic(ctx, provider, apiKey, systemPrompt, userPrompt)
	case aiopsmodel.LLMProviderTypeCodex:
		normalizeCodexProvider(provider)
		return callResponsesAPI(ctx, provider, apiKey, systemPrompt, userPrompt)
	default:
		return callOpenAICompatible(ctx, provider, apiKey, systemPrompt, userPrompt)
	}
}

func callLLMForLogAnalysis(ctx context.Context, provider *aiopsmodel.LLMProvider, apiKey, prompt string) (string, error) {
	return CallLLM(ctx, provider, apiKey, "You are a pragmatic SRE assistant for log investigation.", prompt)
}

func callOpenAICompatible(ctx context.Context, provider *aiopsmodel.LLMProvider, apiKey, systemPrompt, userPrompt string) (string, error) {
	if err := validateHTTPProvider(provider, apiKey); err != nil {
		return "", err
	}
	extra := aiopsmodel.DecodeAnyMap(provider.ExtraJSON)
	maxTokensField := openAITestMaxTokensField(provider, extra)
	maxTokens := provider.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1200
	}
	payload := map[string]interface{}{
		"model": provider.DefaultModel,
		"messages": []map[string]string{
			{"role": "system", "content": strings.TrimSpace(systemPrompt)},
			{"role": "user", "content": userPrompt},
		},
		"temperature": provider.Temperature,
	}
	if provider.Temperature == 0 {
		payload["temperature"] = 0.2
	}
	if maxTokensField != "" {
		payload[maxTokensField] = maxTokens
	}
	if shouldDisableThinkingByDefault(provider) {
		payload["thinking"] = map[string]string{"type": "disabled"}
	}
	mergePayloadMap(payload, anyMapValue(extra["requestBody"]))
	mergePayloadMap(payload, anyMapValue(extra["analysisRequestBody"]))

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildModelEndpoint(provider.BaseURL, provider.ChatPath), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	applyProviderHeaders(req, provider)
	applyProviderAuth(req, provider, apiKey)
	return doLLMAnalysisRequest(req, parseOpenAICompatibleResponse)
}

func callAnthropic(ctx context.Context, provider *aiopsmodel.LLMProvider, apiKey, systemPrompt, userPrompt string) (string, error) {
	if err := validateHTTPProvider(provider, apiKey); err != nil {
		return "", err
	}
	maxTokens := provider.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1200
	}
	payload := map[string]interface{}{
		"model":       provider.DefaultModel,
		"max_tokens":  maxTokens,
		"temperature": provider.Temperature,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
	}
	if strings.TrimSpace(systemPrompt) != "" {
		payload["system"] = strings.TrimSpace(systemPrompt)
	}
	if provider.Temperature == 0 {
		payload["temperature"] = 0.2
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildModelEndpoint(provider.BaseURL, provider.ChatPath), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	applyProviderHeaders(req, provider)
	applyProviderAuth(req, provider, apiKey)
	return doLLMAnalysisRequest(req, parseAnthropicResponse)
}

func callResponsesAPI(ctx context.Context, provider *aiopsmodel.LLMProvider, apiKey, systemPrompt, userPrompt string) (string, error) {
	if err := validateHTTPProvider(provider, apiKey); err != nil {
		return "", err
	}
	extra := aiopsmodel.DecodeAnyMap(provider.ExtraJSON)
	maxOutputTokens := provider.MaxTokens
	if maxOutputTokens <= 0 {
		maxOutputTokens = positiveIntFromAnyMap(extra,
			"maxOutputTokens",
			"max_output_tokens",
			"maxCompletionTokens",
			"max_completion_tokens",
		)
	}
	if maxOutputTokens <= 0 {
		maxOutputTokens = 1200
	}
	input := strings.TrimSpace(userPrompt)
	if strings.TrimSpace(systemPrompt) != "" {
		input = strings.TrimSpace(systemPrompt) + "\n\n" + input
	}
	payload := map[string]interface{}{
		"model":             provider.DefaultModel,
		"input":             input,
		"max_output_tokens": maxOutputTokens,
	}
	if effort := responsesReasoningEffort(provider, extra); effort != "" {
		payload["reasoning"] = map[string]string{"effort": effort}
	}
	mergePayloadMap(payload, anyMapValue(extra["requestBody"]))
	mergePayloadMap(payload, anyMapValue(extra["analysisRequestBody"]))

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, buildModelEndpoint(provider.BaseURL, provider.ChatPath), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	applyProviderHeaders(req, provider)
	applyProviderAuth(req, provider, apiKey)
	return doLLMAnalysisRequest(req, parseResponsesAPIResponse)
}

func responsesReasoningEffort(provider *aiopsmodel.LLMProvider, extra map[string]interface{}) string {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(provider.DefaultModel)), "gpt-5") {
		return ""
	}
	if effort := stringFromAnyMap(extra,
		"analysisReasoningEffort",
		"analysis_reasoning_effort",
		"reasoningEffort",
		"reasoning_effort",
	); effort != "" {
		return effort
	}
	// Keep synchronous analysis endpoints responsive by default. Operators can
	// still override this with extra.analysisRequestBody.reasoning or extra.reasoningEffort.
	return "low"
}

func callOpenAICompatibleLogAnalysis(ctx context.Context, provider *aiopsmodel.LLMProvider, apiKey, prompt string) (string, error) {
	return callOpenAICompatible(ctx, provider, apiKey, "You are a pragmatic SRE assistant for log investigation.", prompt)
}

func callAnthropicLogAnalysis(ctx context.Context, provider *aiopsmodel.LLMProvider, apiKey, prompt string) (string, error) {
	return callAnthropic(ctx, provider, apiKey, "You are a pragmatic SRE assistant for log investigation.", prompt)
}

func doLLMAnalysisRequest(req *http.Request, parser func([]byte) (string, error)) (string, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("llm api returned status %d: %s", resp.StatusCode, truncateForPrompt(string(body), 2000))
	}
	content, err := parser(body)
	if err != nil {
		return "", fmt.Errorf("llm response parse failed: %w; body=%s", err, truncateForPrompt(string(body), 2000))
	}
	return strings.TrimSpace(content), nil
}
