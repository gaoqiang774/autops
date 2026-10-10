package service

import (
	"context"
	"fmt"
	"strings"
	"testing"

	aiopsmodel "dodevops-api/api/aiops/model"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func TestLimitAnalysisLinesKeepsLatest100(t *testing.T) {
	lines := make([]string, 500)
	for i := range lines {
		lines[i] = fmt.Sprintf("log-%03d", i+1)
	}

	selected := limitAnalysisLines(lines, maxLogAnalysisLines, maxLogAnalysisChars)
	if len(selected) != maxLogAnalysisLines {
		t.Fatalf("selected lines = %d, want %d", len(selected), maxLogAnalysisLines)
	}
	if selected[0] != "log-401" || selected[len(selected)-1] != "log-500" {
		t.Fatalf("selected range = %q..%q, want log-401..log-500", selected[0], selected[len(selected)-1])
	}
}

func TestBuildLogAnalysisPromptUsesLatest100AndConciseConclusion(t *testing.T) {
	lines := make([]string, 500)
	for i := range lines {
		lines[i] = fmt.Sprintf("event-%03d", i+1)
	}

	prompt := buildLogAnalysisPrompt("重点判断数据库连接超时", nil, lines)
	if strings.Contains(prompt, "event-400") || !strings.Contains(prompt, "event-401") || !strings.Contains(prompt, "event-500") {
		t.Fatalf("prompt did not keep the latest 100 lines")
	}
	for _, section := range []string{"### 分析结论", "### 关键证据", "### 处置建议"} {
		if !strings.Contains(prompt, section) {
			t.Fatalf("prompt missing section %q", section)
		}
	}
	for _, outcome := range []string{"问题", "误判", "疑似问题", "证据不足"} {
		if !strings.Contains(prompt, outcome) {
			t.Fatalf("prompt missing outcome %q", outcome)
		}
	}
	if !strings.Contains(prompt, "within 500 Chinese characters") {
		t.Fatal("prompt missing concise output limit")
	}
}

func TestLogLineParserJSON(t *testing.T) {
	parser := NewLogLineParser()
	source := aiopsmodel.LogSource{Type: aiopsmodel.LogSourceTypeHost, HostID: 1, Path: "/var/log/app.log"}
	entry := parser.Parse(`{"time":"2026-05-06T10:00:00Z","level":"error","msg":"database timeout","trace_id":"abc"}`, 7, source, nil)

	if entry.LineNo != 7 {
		t.Fatalf("line no = %d", entry.LineNo)
	}
	if entry.Timestamp != "2026-05-06T18:00:00+08:00" {
		t.Fatalf("timestamp = %q", entry.Timestamp)
	}
	if entry.TimestampUnix != 1778061600 {
		t.Fatalf("timestampUnix = %d", entry.TimestampUnix)
	}
	if entry.Level != "ERROR" {
		t.Fatalf("level = %q", entry.Level)
	}
	if entry.Message != "database timeout" {
		t.Fatalf("message = %q", entry.Message)
	}
	if entry.Fields["trace_id"] != "abc" {
		t.Fatalf("trace_id = %#v", entry.Fields["trace_id"])
	}
	if entry.Attributes["trace_id"] != "abc" {
		t.Fatalf("attribute trace_id = %#v", entry.Attributes["trace_id"])
	}
}

func TestLogLineParserLogfmt(t *testing.T) {
	parser := NewLogLineParser()
	source := aiopsmodel.LogSource{Type: aiopsmodel.LogSourceTypeHost}
	entry := parser.Parse(`ts=2026-05-06T10:00:00Z level=warn msg="high latency" path=/api/v1`, 1, source, nil)

	if entry.Level != "WARN" {
		t.Fatalf("level = %q", entry.Level)
	}
	if entry.Message != "high latency" {
		t.Fatalf("message = %q", entry.Message)
	}
	if entry.Timestamp != "2026-05-06T18:00:00+08:00" {
		t.Fatalf("timestamp = %q", entry.Timestamp)
	}
	if entry.Attributes["path"] != "/api/v1" {
		t.Fatalf("path = %q", entry.Attributes["path"])
	}
}

func TestCustomLogParserYAML(t *testing.T) {
	rawYAML := `
name: nginx-like
type: text
pattern:
  use_regex: true
  main: '^(?P<time>\S+)\s+\[(?P<level>\w+)\]\s+(?P<msg>.*)$'
mapping:
  timestamp:
    field: time
    time_format: rfc3339
  severity:
    field: level
    transform: uppercase
  body:
    field: msg
  attributes:
    origin:
      default: edge
`
	custom, err := NewCustomLogParser(rawYAML)
	if err != nil {
		t.Fatal(err)
	}
	parser := NewLogLineParser()
	entry := parser.Parse(`2026-05-06T10:00:00Z [warn] upstream timeout`, 2, aiopsmodel.LogSource{}, custom)

	if entry.Timestamp != "2026-05-06T18:00:00+08:00" {
		t.Fatalf("timestamp = %q", entry.Timestamp)
	}
	if entry.Level != "WARN" {
		t.Fatalf("level = %q", entry.Level)
	}
	if entry.Message != "upstream timeout" {
		t.Fatalf("message = %q", entry.Message)
	}
	if entry.Attributes["origin"] != "edge" {
		t.Fatalf("origin = %q", entry.Attributes["origin"])
	}
}

func TestLogTimestampParserUsesShanghaiForDisplay(t *testing.T) {
	parser := NewLogLineParser()
	source := aiopsmodel.LogSource{Type: aiopsmodel.LogSourceTypeHost}
	tests := []struct {
		name      string
		raw       string
		wantTime  string
		wantUnix  int64
		checkUnix bool
	}{
		{
			name:      "UTC timestamp",
			raw:       `{"timestamp":"2026-08-07T01:31:42.295162734Z","message":"started"}`,
			wantTime:  "2026-08-07T09:31:42.295162734+08:00",
			wantUnix:  1786066302,
			checkUnix: true,
		},
		{
			name:      "timestamp with Shanghai offset",
			raw:       `{"timestamp":"2026-08-07T09:31:42.295162734+08:00","message":"started"}`,
			wantTime:  "2026-08-07T09:31:42.295162734+08:00",
			wantUnix:  1786066302,
			checkUnix: true,
		},
		{
			name:      "local timestamp without zone",
			raw:       `{"timestamp":"2026-08-07 09:31:42.295162734","message":"started"}`,
			wantTime:  "2026-08-07T09:31:42.295162734+08:00",
			wantUnix:  1786066302,
			checkUnix: true,
		},
		{
			name:     "time only",
			raw:      "09:31:42.295 started",
			wantTime: "09:31:42.295",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := parser.Parse(tt.raw, 1, source, nil)
			if entry.Timestamp != tt.wantTime {
				t.Fatalf("timestamp = %q, want %q", entry.Timestamp, tt.wantTime)
			}
			if tt.checkUnix && entry.TimestampUnix != tt.wantUnix {
				t.Fatalf("timestampUnix = %d, want %d", entry.TimestampUnix, tt.wantUnix)
			}
		})
	}
}

func TestPatternClusterer(t *testing.T) {
	clusterer := NewLogPatternClusterer()
	clusterer.Add(aiopsmodel.LogEntry{Message: "request 123 failed for user 11"})
	clusterer.Add(aiopsmodel.LogEntry{Message: "request 456 failed for user 22"})
	clusterer.Add(aiopsmodel.LogEntry{Message: "request 789 failed for user 33"})

	patterns := clusterer.TopPatterns(10)
	if len(patterns) != 1 {
		t.Fatalf("patterns = %d", len(patterns))
	}
	if patterns[0].Count != 3 {
		t.Fatalf("count = %d", patterns[0].Count)
	}
}

func TestLogAnalysisEngineSessionStatus(t *testing.T) {
	engine := NewLogAnalysisEngine("s1", aiopsmodel.LogSource{Type: aiopsmodel.LogSourceTypeHost, HostID: 1, Path: "/var/log/app.log"}, "auto", false, 100)
	engine.RegisterStream("/var/log/app.log", "host", true)
	engine.Ingest(aiopsmodel.LogEntry{Level: "ERROR", Raw: "database timeout", Message: "database timeout", Source: aiopsmodel.LogSource{Type: aiopsmodel.LogSourceTypeHost, Path: "/var/log/app.log"}})
	engine.Ingest(aiopsmodel.LogEntry{Level: "WARN", Raw: "slow query", Message: "slow query", Source: aiopsmodel.LogSource{Type: aiopsmodel.LogSourceTypeHost, Path: "/var/log/app.log"}})

	status := engine.Status()
	if status.SessionID != "s1" {
		t.Fatalf("session id = %q", status.SessionID)
	}
	if status.TotalLogs != 2 {
		t.Fatalf("total logs = %d", status.TotalLogs)
	}
	if status.LevelCounts["ERROR"] != 1 {
		t.Fatalf("error count = %d", status.LevelCounts["ERROR"])
	}
	if len(status.TopPatterns) == 0 {
		t.Fatal("expected top patterns")
	}
	if len(status.Streams) == 0 || !status.Streams[0].Active {
		t.Fatal("expected active stream")
	}
}

func TestBuildDefaultLogSessionIDForK8s(t *testing.T) {
	source := aiopsmodel.LogSource{
		Type:      aiopsmodel.LogSourceTypeK8s,
		ClusterID: 12,
		Namespace: "prod",
		Pod:       "api-7c6d8f",
		Container: "app",
	}

	sessionID := buildDefaultLogSessionID(source)
	expected := "k8s:cluster:12:ns:prod:pod:api-7c6d8f:container:app"
	if sessionID != expected {
		t.Fatalf("session id = %q", sessionID)
	}
}

func TestTrimK8sContainerDisplayName(t *testing.T) {
	cases := map[string]string{
		"app":               "app",
		"app (init)":        "app",
		"debug (ephemeral)": "debug",
		"  sidecar  ":       "sidecar",
	}

	for input, expected := range cases {
		if actual := trimK8sContainerDisplayName(input); actual != expected {
			t.Fatalf("trim %q = %q", input, actual)
		}
	}
}

func TestMergeK8sPodStepStatusPreservesCollectedCurrentLogs(t *testing.T) {
	if actual := mergeK8sPodStepStatus("succeeded", "skipped"); actual != "succeeded" {
		t.Fatalf("succeeded current logs plus skipped previous logs = %q", actual)
	}
	if actual := mergeK8sPodStepStatus("succeeded", "failed"); actual != "partial" {
		t.Fatalf("succeeded and failed signals = %q", actual)
	}
	if actual := mergeK8sPodStepStatus("skipped", "failed"); actual != "failed" {
		t.Fatalf("skipped and failed signals = %q", actual)
	}
	if actual := mergeK8sPodStepStatus("running", "skipped"); actual != "skipped" {
		t.Fatalf("running step completed as skipped = %q", actual)
	}
}

func TestBuildK8sPodEvidenceSummaryKeepsCollectionErrorsWithoutLogs(t *testing.T) {
	evidence := buildK8sPodEvidenceSummary(1, testPendingPod(), nil, "app", nil, nil, nil, nil, nil, nil, nil, nil, []string{"current logs: container app is waiting"})

	if evidence.Pod.Phase != "Pending" {
		t.Fatalf("phase = %q", evidence.Pod.Phase)
	}
	if evidence.LogLines != 0 {
		t.Fatalf("log lines = %d, want 0", evidence.LogLines)
	}
	if len(evidence.CollectionErrors) != 1 {
		t.Fatalf("collection errors len = %d, want 1", len(evidence.CollectionErrors))
	}
	if evidence.CollectionErrors[0] != "current logs: container app is waiting" {
		t.Fatalf("collection error = %q", evidence.CollectionErrors[0])
	}
	if len(evidence.Pod.Containers) != 1 || evidence.Pod.Containers[0].Reason != "ContainerCreating" {
		t.Fatalf("container state = %#v", evidence.Pod.Containers)
	}
}

func TestBuildK8sPodEvidenceSummaryIncludesApplicationErrorFindings(t *testing.T) {
	parser := NewLogLineParser()
	source := aiopsmodel.LogSource{Type: aiopsmodel.LogSourceTypeK8s, ClusterID: 1, Namespace: "leyoujia-itest", Pod: "purview-web", Container: "app"}
	lines := []string{
		`2026-07-13 11:16:12.378 [http-nio-8080-exec-10] ERROR bigdata_error_log - url:/jjspurview/menu/index error params:{}`,
		`org.apache.dubbo.rpc.RpcException: Failed to invoke the method queryRuleMainList in the service api.leyoujia.platform.platform.service.IMdsRuleMainService. No provider available for the service api.leyoujia.platform.platform.service.IMdsRuleMainService from registry Directory(invokers: 2[172.16.16.10], validInvokers: 0[])`,
		`\tat org.apache.dubbo.rpc.cluster.support.AbstractClusterInvoker.checkInvokers(AbstractClusterInvoker.java:370)`,
	}
	entries := parseRawLinesForSource(parser, lines, source, nil)

	evidence := buildK8sPodEvidenceSummary(1, testRunningPod(), nil, "app", entries, entries, nil, nil, nil, nil, nil, nil, nil)

	if len(evidence.LogFindings) < 2 {
		t.Fatalf("log findings len = %d, want at least 2: %#v", len(evidence.LogFindings), evidence.LogFindings)
	}
	kinds := map[string]bool{}
	for _, finding := range evidence.LogFindings {
		kinds[finding.Kind] = true
	}
	if !kinds["APPLICATION_ERROR"] {
		t.Fatalf("missing APPLICATION_ERROR finding: %#v", evidence.LogFindings)
	}
	if !kinds["DUBBO_NO_PROVIDER"] {
		t.Fatalf("missing DUBBO_NO_PROVIDER finding: %#v", evidence.LogFindings)
	}
	progress := buildK8sPodEvidenceProgressMessage(evidence)
	if !strings.Contains(progress, "DUBBO_NO_PROVIDER") || !strings.Contains(progress, "APPLICATION_ERROR") {
		t.Fatalf("progress message missing finding kinds: %q", progress)
	}
	analysis := ensureK8sPodAnalysisIncludesLogFindings("### 事件总结\nPod Running/Ready。", evidence)
	if !strings.Contains(analysis, "### 关键异常日志") || !strings.Contains(analysis, "No provider available") {
		t.Fatalf("analysis missing forced log findings: %s", analysis)
	}
	summary, _ := extractK8sPodAnalysisSummaryAndRemediation("### 事件总结\nPod Running/Ready。", evidence)
	if !strings.Contains(summary, "Dubbo 无可用 Provider") {
		t.Fatalf("summary did not prefer log finding: %q", summary)
	}
}

func TestK8sPodDiagnosisIdentifiesHTTPRoutedToJDWPPort(t *testing.T) {
	parser := NewLogLineParser()
	line := `2026-07-30T15:13:12.450+08:00 WARN Debugger failed to attach: handshake failed - received >GET /sensitive< - expected >JDWP-Handshake<`
	lines := []string{
		"WARN spring.jpa.open-in-view is enabled",
		"WARN mapper XML scan found no optional mapper",
		"WARN readiness probe startup delay may be short",
		"WARN deprecated configuration one",
		"WARN deprecated configuration two",
		"WARN deprecated configuration three",
		line,
	}
	entries := parseRawLinesForSource(parser, lines, aiopsmodel.LogSource{Type: aiopsmodel.LogSourceTypeK8s}, nil)
	pod := testRunningPod()
	pod.Namespace = "leyoujia-itest"
	pod.Name = "lyj-sensitive-channel-abc"
	pod.Spec.Containers[0].Ports = []corev1.ContainerPort{
		{Name: "debug", ContainerPort: 50143},
		{Name: "http", ContainerPort: 8080},
	}

	evidence := buildK8sPodEvidenceSummary(1, pod, nil, "app", entries, entries, nil, nil, nil, nil, nil, nil, nil)
	evidence.Services = []aiopsmodel.K8sPodAnalyzeServiceInfo{{
		Name: "lyj-sensitive-channel",
		Ports: []aiopsmodel.K8sPodAnalyzeServicePortInfo{
			{Name: "debug", Port: 50143, TargetPort: "50143"},
			{Name: "http", Port: 8080, TargetPort: "8080"},
		},
	}}
	evidence.IngressRoutes = []aiopsmodel.K8sPodAnalyzeIngressRouteInfo{{
		Ingress:     "lyj-sensitive-channel",
		Host:        "itest.example.com",
		Path:        "/sensitive",
		ServiceName: "lyj-sensitive-channel",
		ServicePort: "debug",
	}}
	addK8sPodNetworkSuspectedReasons(&evidence)

	if !hasK8sPodLogFindingKind(evidence, "HTTP_TO_JDWP_PORT") {
		t.Fatalf("JDWP protocol mismatch was not classified: %#v", evidence.LogFindings)
	}
	route, confirmed := findK8sPodJDWPRoute(evidence)
	if !confirmed || !strings.Contains(route, "targetPort 50143") {
		t.Fatalf("JDWP route not confirmed: confirmed=%v route=%q", confirmed, route)
	}

	result := compactK8sPodAnalyzeResult(&aiopsmodel.K8sPodAnalyzeResult{
		Analysis: "### 事件总结\nPod Running。\n\n### 根本原因\nReadiness initialDelaySeconds 可能偏小。\n\n### 解决办法\n1. 增大 Readiness 延迟。\n2. 检查 MyBatis。",
		Severity: "警告",
		Evidence: evidence,
	})
	for _, expected := range []string{"### 问题描述", "### 问题原因", "### 解决方案", "### 总结结论", "HTTP 请求被路由到 JVM JDWP 调试端口", "targetPort 50143", "service.port"} {
		if !strings.Contains(result.Analysis, expected) {
			t.Fatalf("analysis missing %q:\n%s", expected, result.Analysis)
		}
	}
	for _, removedSection := range []string{"### 关键证据", "### 验证步骤"} {
		if strings.Contains(result.Analysis, removedSection) {
			t.Fatalf("analysis should not expose section %q:\n%s", removedSection, result.Analysis)
		}
	}
	if strings.Contains(result.Analysis, "MyBatis") || strings.Contains(result.Analysis, "增大 Readiness") {
		t.Fatalf("generic model advice was not replaced:\n%s", result.Analysis)
	}
	if result.Status != "" {
		t.Fatalf("compact should not invent an unset status, got %q", result.Status)
	}
}

func TestCollectK8sPodNetworkEvidenceFindsServiceAndIngressRoute(t *testing.T) {
	pod := testRunningPod()
	pod.Namespace = "leyoujia-itest"
	pod.Labels = map[string]string{"app": "lyj-sensitive-channel"}
	pod.Spec.Containers[0].Ports = []corev1.ContainerPort{{Name: "debug", ContainerPort: 50143}, {Name: "http", ContainerPort: 8080}}
	pathType := networkingv1.PathTypePrefix
	client := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "lyj-sensitive-channel", Namespace: pod.Namespace},
			Spec: corev1.ServiceSpec{
				Selector: pod.Labels,
				Ports: []corev1.ServicePort{
					{Name: "debug", Port: 50143, TargetPort: intstr.FromInt32(50143)},
					{Name: "http", Port: 8080, TargetPort: intstr.FromInt32(8080)},
				},
			},
		},
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: "lyj-sensitive-channel", Namespace: pod.Namespace},
			Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{
				Host: "itest.example.com",
				IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{Paths: []networkingv1.HTTPIngressPath{{
					Path: "/sensitive", PathType: &pathType,
					Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
						Name: "lyj-sensitive-channel", Port: networkingv1.ServiceBackendPort{Name: "debug"},
					}},
				}}}},
			}}},
		},
	)

	services, routes, manifests, err := collectK8sPodNetworkEvidence(context.Background(), client, pod)
	if err != nil {
		t.Fatalf("collect network evidence: %v", err)
	}
	if len(services) != 1 || len(routes) != 1 {
		t.Fatalf("services=%#v routes=%#v", services, routes)
	}
	if len(manifests) != 2 || manifests[0].Kind != "Service" || manifests[1].Kind != "Ingress" {
		t.Fatalf("unexpected manifests: %#v", manifests)
	}
	if routes[0].ServicePort != "debug" || services[0].Ports[0].TargetPort != "50143" {
		t.Fatalf("unexpected port evidence: services=%#v routes=%#v", services, routes)
	}
}

func TestBuildSanitizedK8sManifestRemovesRuntimeAndSensitiveFields(t *testing.T) {
	object := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":            "demo",
			"namespace":       "default",
			"resourceVersion": "12345",
			"managedFields":   []interface{}{map[string]interface{}{"manager": "kubectl"}},
			"labels":          map[string]interface{}{"app": "demo"},
			"annotations": map[string]interface{}{
				"kubectl.kubernetes.io/last-applied-configuration": "large-json",
				"example.com/token": "plain-token",
				"example.com/mode":  "prod",
			},
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{map[string]interface{}{
						"name":  "app",
						"image": "example/app:1",
						"env": []interface{}{
							map[string]interface{}{"name": "SPRING_PROFILES_ACTIVE", "value": "prod"},
							map[string]interface{}{"name": "DB_PASSWORD", "value": "secret-value"},
						},
					}},
				},
			},
		},
		"status": map[string]interface{}{"readyReplicas": float64(1)},
	}

	manifest, err := buildSanitizedK8sManifest("apps/v1", "Deployment", object)
	if err != nil {
		t.Fatalf("build sanitized manifest: %v", err)
	}
	for _, forbidden := range []string{"managedFields", "resourceVersion", "last-applied-configuration", "readyReplicas", "secret-value", "plain-token"} {
		if strings.Contains(manifest.YAML, forbidden) {
			t.Fatalf("manifest contains %q:\n%s", forbidden, manifest.YAML)
		}
	}
	for _, expected := range []string{"kind: Deployment", "name: demo", "SPRING_PROFILES_ACTIVE", "value: prod", "DB_PASSWORD", "<redacted>"} {
		if !strings.Contains(manifest.YAML, expected) {
			t.Fatalf("manifest missing %q:\n%s", expected, manifest.YAML)
		}
	}
}

func TestBuildK8sPodFinalConclusionUsesEvidenceWithoutTruncatedMarker(t *testing.T) {
	evidence := aiopsmodel.K8sPodAnalyzeEvidenceSummary{
		Workload: &aiopsmodel.K8sPodAnalyzeWorkloadInfo{
			Type:              "Deployment",
			Namespace:         "default",
			Name:              "zero-ai-im",
			AvailableReplicas: 1,
		},
		Pod: aiopsmodel.K8sPodAnalyzePodInfo{
			Namespace: "default",
			Pod:       "zero-ai-im-abc",
			Phase:     string(corev1.PodRunning),
			Conditions: []aiopsmodel.K8sPodAnalyzeCondition{{
				Type:   "Ready",
				Status: "True",
			}},
			Containers: []aiopsmodel.K8sPodAnalyzeContainerInfo{{
				Name:         "app",
				Ready:        true,
				RestartCount: 1,
				LastReason:   "OOMKilled",
				Resources: aiopsmodel.K8sPodAnalyzeResourceInfo{
					MemoryLimit: "8Gi",
				},
			}},
		},
		LogFindings: []aiopsmodel.K8sPodAnalyzeLogFinding{{
			Kind:    "WARN",
			Message: "stream_chunk not found for account x with verbose suffix ...<truncated>",
		}},
		MetricsTrend: &aiopsmodel.K8sPodAnalyzeMetricsTrendInfo{
			Available: true,
			Range:     "6h",
			MemoryWSSUsageRate: &aiopsmodel.K8sPodAnalyzeMetricTrendStat{
				Unit:      "%",
				Min:       15.22,
				Peak:      58.67,
				NearLimit: false,
			},
		},
	}

	conclusion := buildK8sPodFinalConclusion(evidence, strings.Repeat("旧 AI 结论", 80), strings.Repeat("旧根因", 80), "警告")
	for _, expected := range []string{"Deployment default/zero-ai-im", "可用但存在风险", "直接原因", "异常信号", "OOMKilled", "stream_chunk not found", "内存 WSS", "持续 Running/Ready"} {
		if !strings.Contains(conclusion, expected) {
			t.Fatalf("conclusion missing %q: %s", expected, conclusion)
		}
	}
	if strings.Contains(conclusion, "<truncated>") {
		t.Fatalf("conclusion contains truncated marker: %s", conclusion)
	}
}

func testPendingPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "pending-pod",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:  "app",
				Ready: false,
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "ContainerCreating",
						Message: "pulling image",
					},
				},
			}},
		},
	}
}

func testRunningPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "running-pod",
		},
		Spec: corev1.PodSpec{
			NodeName:   "node-a",
			Containers: []corev1.Container{{Name: "app"}},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         "app",
				Ready:        true,
				RestartCount: 0,
				State: corev1.ContainerState{
					Running: &corev1.ContainerStateRunning{},
				},
			}},
		},
	}
}

func TestExtractK8sPodAnalysisSummaryAndRemediation(t *testing.T) {
	analysis := `### 事件总结

Pod ai-data-api-569557457d-6b57z 处于 CrashLoopBackOff，因启动后应用异常退出且探针失败导致服务不可用。

### 严重性评估

高

### 解决办法

1. 先查看 previous 日志，定位最后一次崩溃前的直接报错。
2. 检查 readiness/liveness 探针、端口监听和启动参数。
3. 如果 CPU 已接近上限，适当提高资源配额或排查死循环。`

	summary, remediation := extractK8sPodAnalysisSummaryAndRemediation(analysis, aiopsmodel.K8sPodAnalyzeEvidenceSummary{})

	if summary != "Pod ai-data-api-569557457d-6b57z 处于 CrashLoopBackOff，因启动后应用异常退出且探针失败导致服务不可用。" {
		t.Fatalf("summary = %q", summary)
	}
	if len(remediation) != 3 {
		t.Fatalf("remediation len = %d", len(remediation))
	}
	if remediation[0] != "先查看 previous 日志，定位最后一次崩溃前的直接报错。" {
		t.Fatalf("remediation[0] = %q", remediation[0])
	}
}

func TestExtractK8sPodAnalysisSummaryFallback(t *testing.T) {
	evidence := aiopsmodel.K8sPodAnalyzeEvidenceSummary{
		Pod: aiopsmodel.K8sPodAnalyzePodInfo{
			Pod:          "demo-pod",
			Container:    "app",
			RestartCount: 12,
		},
		ResourceHotspots: []string{"CPU_LIMIT_HIGH"},
		SuspectedReasons: []string{"BackOff"},
	}

	summary, remediation := extractK8sPodAnalysisSummaryAndRemediation("", evidence)

	if summary == "" {
		t.Fatal("summary should not be empty")
	}
	if len(remediation) == 0 {
		t.Fatal("expected remediation")
	}
}

func TestNormalizeIngressAnalyzeHostAndPath(t *testing.T) {
	host, requestPath := normalizeIngressAnalyzeHostAndPath("https://api.example.com/orders?id=1", "")
	if host != "api.example.com" {
		t.Fatalf("host = %q", host)
	}
	if requestPath != "/orders?id=1" {
		t.Fatalf("path = %q", requestPath)
	}

	host, requestPath = normalizeIngressAnalyzeHostAndPath("http://api.example.com:8080", "/health")
	if host != "api.example.com" {
		t.Fatalf("host with port = %q", host)
	}
	if requestPath != "/health" {
		t.Fatalf("explicit path = %q", requestPath)
	}

	host, requestPath = normalizeIngressAnalyzeHostAndPath("api.example.com/foo", "")
	if host != "api.example.com" || requestPath != "/foo" {
		t.Fatalf("host/path = %q %q", host, requestPath)
	}
}

func TestInferK8sIngressDiagnosisStatusPrefersLiveProbeOK(t *testing.T) {
	evidence := aiopsmodel.K8sIngressAnalyzeEvidenceSummary{
		StatusCode: 502,
		HTTPProbe: &aiopsmodel.K8sIngressAnalyzeHTTPProbeInfo{
			StatusCode: 200,
			Status:     "200 OK",
		},
	}
	if status := inferK8sIngressDiagnosisStatus(evidence); status != "normal" {
		t.Fatalf("status = %q", status)
	}
	if severity := inferK8sIngressSeverity(evidence); severity != "info" {
		t.Fatalf("severity = %q", severity)
	}
}

func TestTruncateForPromptKeepsUTF8Valid(t *testing.T) {
	value := "检查后端应用日志和配置，确认会话校验失败原因"
	truncated := truncateForPrompt(value, 7)
	if strings.Contains(truncated, "�") {
		t.Fatalf("truncated contains replacement char: %q", truncated)
	}
	if truncated != "检查后端应用日...<truncated>" {
		t.Fatalf("truncated = %q", truncated)
	}
}
