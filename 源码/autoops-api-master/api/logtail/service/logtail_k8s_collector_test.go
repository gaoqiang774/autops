package service

import (
	"context"
	"os"
	"strings"
	"testing"

	k8smodel "dodevops-api/api/k8s/model"
	logtailmodel "dodevops-api/api/logtail/model"
	commonconfig "dodevops-api/common/config"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDiagnoseKubernetesVectorLog(t *testing.T) {
	tests := []struct {
		name string
		log  string
		want string
	}{
		{
			name: "victorialogs request timeout",
			log:  `WARN sink{component_type="http"}: Request timed out. If this happens often...`,
			want: "写入VictoriaLogs超时",
		},
		{
			name: "kafka advertised localhost refused",
			log:  `ERROR log: librdkafka: Global error: BrokerTransportFailure (Local: Broker transport failure): localhost:9092/1: Connect to ipv4#127.0.0.1:9092 failed: Connection refused`,
			want: "写入Kafka失败: Kafka advertised.listeners 返回 localhost:9092",
		},
		{
			name: "kafka advertised hostname resolve failed",
			log:  `ERROR librdkafka: librdkafka: FAIL [thrd:kafka-node2-4-210:9092/1]: kafka-node2-4-210:9092/1: Failed to resolve 'kafka-node2-4-210:9092': Name or service not known`,
			want: "Kafka advertised.listeners 返回不可解析地址 kafka-node2-4-210:9092",
		},
		{
			name: "kafka message timed out",
			log:  `ERROR sink{component_type=kafka}: Service call failed. error=KafkaError (Message production error: MessageTimedOut (Local: Message timed out))`,
			want: "写入Kafka超时",
		},
		{
			name: "ndjson codec incompatible",
			log:  "Configuration error. error=unknown variant `ndjson`, expected one of `json`, `native_json`",
			want: "Vector配置不兼容",
		},
		{
			name: "fingerprint file rotated",
			log:  "Failed reading file for fingerprinting. file=/var/log/containers/demo.log error=No such file or directory",
			want: "读取容器日志文件失败",
		},
		{
			name: "container log unavailable",
			log:  "unable to retrieve container logs for docker://0315633bb857945c",
			want: "无法读取Vector容器日志",
		},
		{
			name: "no diagnostic",
			log:  "Vector has started.",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diagnoseKubernetesVectorLog(tt.log)
			if tt.want == "" {
				if got != "" {
					t.Fatalf("diagnoseKubernetesVectorLog() = %q, want empty", got)
				}
				return
			}
			if !strings.Contains(got, tt.want) {
				t.Fatalf("diagnoseKubernetesVectorLog() = %q, want contains %q", got, tt.want)
			}
		})
	}
}

func TestMergeKubernetesDiagnosticMessage(t *testing.T) {
	got := mergeKubernetesDiagnosticMessage("CrashLoopBackOff: previous Error(exitCode=78)", "Vector配置错误: bad config")
	want := "CrashLoopBackOff: previous Error(exitCode=78)；Vector配置错误: bad config"
	if got != want {
		t.Fatalf("mergeKubernetesDiagnosticMessage() = %q, want %q", got, want)
	}

	got = mergeKubernetesDiagnosticMessage("ContainerCreating", "写入VictoriaLogs超时")
	if got != "写入VictoriaLogs超时" {
		t.Fatalf("mergeKubernetesDiagnosticMessage() waiting reason = %q", got)
	}
}

func TestDetermineKubernetesCollectorProgressMarksPartialDaemonSetPending(t *testing.T) {
	status, progress, text := determineKubernetesCollectorProgress(6, 5, 4, 5, "aiops-vector-wp76l: ImagePullBackOff")
	if status != "pending" {
		t.Fatalf("status = %q, want pending during rollout", status)
	}
	if progress != 66 {
		t.Fatalf("progress = %d, want 66", progress)
	}
	if !strings.Contains(text, "滚动更新中") || !strings.Contains(text, "ImagePullBackOff") {
		t.Fatalf("text = %q, want rollout status with pod error", text)
	}
}

func TestDetermineKubernetesCollectorProgressMarksConvergedDaemonSetRunning(t *testing.T) {
	status, progress, text := determineKubernetesCollectorProgress(6, 6, 6, 6, "")
	if status != "running" || progress != 100 || text != "运行中" {
		t.Fatalf("status/progress/text = %q/%d/%q, want running/100/运行中", status, progress, text)
	}
}

func TestDetermineKubernetesCollectorProgressFailsWhenNoPodReady(t *testing.T) {
	status, progress, text := determineKubernetesCollectorProgress(6, 0, 0, 0, "aiops-vector-wp76l: ImagePullBackOff")
	if status != "failed" {
		t.Fatalf("status = %q, want failed", status)
	}
	if progress != -1 {
		t.Fatalf("progress = %d, want -1", progress)
	}
	if !strings.Contains(text, "ImagePullBackOff") {
		t.Fatalf("text = %q, want pod error", text)
	}
}

func TestKubernetesDaemonSetIsRolling(t *testing.T) {
	daemonSet := &appsv1.DaemonSet{
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 16,
			NumberReady:            15,
			UpdatedNumberScheduled: 12,
			NumberAvailable:        15,
		},
	}
	if !kubernetesDaemonSetIsRolling(daemonSet) {
		t.Fatal("kubernetesDaemonSetIsRolling() = false, want true")
	}

	daemonSet.Status.NumberReady = 16
	daemonSet.Status.UpdatedNumberScheduled = 16
	daemonSet.Status.NumberAvailable = 16
	if kubernetesDaemonSetIsRolling(daemonSet) {
		t.Fatal("kubernetesDaemonSetIsRolling() = true, want false after convergence")
	}
}

func TestIsKubernetesClusterLevelCollectorMutation(t *testing.T) {
	stdoutTask := logtailmodel.LogCollectTask{K8SLogSource: logtailmodel.LogK8SLogSourceStdout}
	if !isKubernetesClusterLevelCollectorMutation(stdoutTask, logtailmodel.LogK8SCollectorOperationUpdate) {
		t.Fatal("stdout update should be treated as cluster-level collector mutation")
	}

	containerFileTask := logtailmodel.LogCollectTask{K8SLogSource: logtailmodel.LogK8SLogSourceContainerFile}
	if isKubernetesClusterLevelCollectorMutation(containerFileTask, logtailmodel.LogK8SCollectorOperationUpdate) {
		t.Fatal("container_file update should not be treated as cluster-level collector mutation")
	}
}

func TestFormatKubernetesTerminationExplainsVectorExit78(t *testing.T) {
	got := formatKubernetesTermination(&corev1.ContainerStateTerminated{
		Reason:   "Error",
		ExitCode: 78,
	})
	if !strings.Contains(got, "Vector配置校验失败") {
		t.Fatalf("formatKubernetesTermination() = %q, want Vector config hint", got)
	}
}

func TestDescribeKubernetesContainerStatusIgnoresHealthyHistoricalTermination(t *testing.T) {
	status := corev1.ContainerStatus{
		Ready:        true,
		RestartCount: 3,
		State: corev1.ContainerState{
			Running: &corev1.ContainerStateRunning{},
		},
		LastTerminationState: corev1.ContainerState{
			Terminated: &corev1.ContainerStateTerminated{Reason: "Error", ExitCode: 255},
		},
	}
	if got := describeKubernetesContainerStatus(status); got != "" {
		t.Fatalf("healthy running container status = %q, want empty", got)
	}
}

func TestWriteKubernetesVectorHTTPSinkUsesLegacyNDJSON(t *testing.T) {
	var builder strings.Builder
	writeVectorHTTPSink(&builder, "sink", "input", true)
	got := builder.String()
	if !strings.Contains(got, "codec: ndjson") {
		t.Fatalf("sink config = %q, want legacy ndjson codec", got)
	}
	if strings.Contains(got, "framing:") {
		t.Fatalf("sink config = %q, legacy config should not include framing", got)
	}
}

func TestWriteKubernetesVectorHTTPSinkUsesModernJSONLines(t *testing.T) {
	var builder strings.Builder
	writeVectorHTTPSink(&builder, "sink", "input", false)
	got := builder.String()
	if !strings.Contains(got, "codec: json") {
		t.Fatalf("sink config = %q, want json codec", got)
	}
	if !strings.Contains(got, "framing:") || !strings.Contains(got, "method: newline_delimited") {
		t.Fatalf("sink config = %q, want newline_delimited framing", got)
	}
	if strings.Contains(got, "healthcheck:") {
		t.Fatalf("sink config = %q, should avoid healthcheck for wider Vector compatibility", got)
	}
}

func TestWriteKubernetesVectorHTTPSinkUsesDiskBufferAndBackpressure(t *testing.T) {
	var builder strings.Builder
	writeVectorHTTPSink(&builder, "sink", "input", false)
	got := builder.String()
	for _, want := range []string{
		"batch:",
		"max_events: 1000",
		"request:",
		"timeout_secs: 60",
		"buffer:",
		"type: disk",
		"max_size: 2147483648",
		"when_full: block",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("sink config = %q, want %q", got, want)
		}
	}
}

func TestWriteKubernetesNodeFileUnpackScriptParsesTomcatAppName(t *testing.T) {
	var builder strings.Builder
	writeKubernetesNodeFileUnpackScript(&builder)
	got := builder.String()
	for _, want := range []string{
		`^/data/logs/tomcatlogs/(?P<app_name>[^/]+)/`,
		`.app = .app_name`,
		`.service = .app_name`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("node file unpack script = %q, want %q", got, want)
		}
	}
}

func TestWriteVectorParseScriptNormalizesKubernetesStdoutJSONMessage(t *testing.T) {
	var builder strings.Builder
	writeVectorParseScript(&builder, logtailmodel.LogCollectTask{
		CollectType:       logtailmodel.LogCollectTypeKubernetes,
		K8SLogSource:      logtailmodel.LogK8SLogSourceStdout,
		ParseModeSnapshot: "json",
	}, nil)
	got := builder.String()
	for _, want := range []string{
		"log_text = to_string(.log)",
		".message = log_text",
		"inner, inner_err = parse_json(log_text)",
		".message = to_string(.msg) ?? log_text",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("parse script = %q, want %q", got, want)
		}
	}
}

func TestWriteVectorLogFileNameScriptDoesNotEmitVRL(t *testing.T) {
	var builder strings.Builder
	writeVectorLogFileNameScript(&builder)
	if got := builder.String(); got != "" {
		t.Fatalf("writeVectorLogFileNameScript() = %q, want empty until Vector compatibility is verified", got)
	}
}

func TestBuildKubernetesStdoutDaemonSetMountsDataLogs(t *testing.T) {
	daemonSet := buildKubernetesStdoutDaemonSet("aiops-vector", map[string]string{"app": "aiops-vector"}, "abc123", "configured/vector:latest-debian")
	if len(daemonSet.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("container count = %d, want 1", len(daemonSet.Spec.Template.Spec.Containers))
	}
	container := daemonSet.Spec.Template.Spec.Containers[0]
	if !hasVolumeMount(container.VolumeMounts, "/data/logs") {
		t.Fatalf("vector daemonset should mount /data/logs")
	}
	if !hasHostPathVolume(daemonSet.Spec.Template.Spec.Volumes, "data-logs", "/data/logs") {
		t.Fatalf("vector daemonset should include /data/logs hostPath volume")
	}
}

func TestBuildKubernetesStdoutDaemonSetProjectsConfigDirectory(t *testing.T) {
	daemonSet := buildKubernetesStdoutDaemonSet(
		"aiops-vector",
		map[string]string{"app": "aiops-vector"},
		"abc123",
		"configured/vector:latest-debian",
		kubernetesVectorConfigRef{ConfigMap: "aiops-vector-base", FileName: "00-base.yaml"},
		kubernetesVectorConfigRef{ConfigMap: "aiops-vector-zero-ai-12", FileName: "10-zero-ai-12.yaml"},
	)
	container := daemonSet.Spec.Template.Spec.Containers[0]
	if len(container.Args) != 2 || container.Args[0] != "--config-dir" || container.Args[1] != "/etc/vector" {
		t.Fatalf("vector args = %v, want --config-dir /etc/vector", container.Args)
	}

	for _, volume := range daemonSet.Spec.Template.Spec.Volumes {
		if volume.Name != "config" {
			continue
		}
		if volume.Projected == nil || len(volume.Projected.Sources) != 2 {
			t.Fatalf("config volume = %+v, want two projected ConfigMaps", volume.VolumeSource)
		}
		first := volume.Projected.Sources[0].ConfigMap
		second := volume.Projected.Sources[1].ConfigMap
		if first == nil || first.Name != "aiops-vector-base" || len(first.Items) != 1 || first.Items[0].Path != "00-base.yaml" {
			t.Fatalf("base projection = %+v", first)
		}
		if second == nil || second.Name != "aiops-vector-zero-ai-12" || len(second.Items) != 1 || second.Items[0].Path != "10-zero-ai-12.yaml" {
			t.Fatalf("project projection = %+v", second)
		}
		return
	}
	t.Fatal("config projected volume not found")
}

func TestKubernetesProjectConfigNamesUseProjectName(t *testing.T) {
	project := &logtailmodel.LogProject{ID: 12, Name: "Zero AI / 生产", Code: "zero_ai_2ac2e5b8"}
	if got := kubernetesProjectConfigMapName(project); got != "aiops-vector-zero-ai-12" {
		t.Fatalf("ConfigMap name = %q, want aiops-vector-zero-ai-12", got)
	}
	if got := kubernetesProjectConfigFileName(project); got != "10-zero-ai-12.yaml" {
		t.Fatalf("config file name = %q, want 10-zero-ai-12.yaml", got)
	}
}

func TestBuildKubernetesStdoutDaemonSetUsesKafkaHostAliases(t *testing.T) {
	oldConfig := commonconfig.Config
	defer func() { commonconfig.Config = oldConfig }()

	configPath := writeTempLogtailConfig(t, `
logtail:
  kafka:
    host_aliases:
      - ip: "192.168.4.210"
        hostnames:
          - "kafka-node2-4-210"
`)
	if err := commonconfig.LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	daemonSet := buildKubernetesStdoutDaemonSet("aiops-vector", map[string]string{"app": "aiops-vector"}, "abc123", "configured/vector:latest-debian")
	aliases := daemonSet.Spec.Template.Spec.HostAliases
	if len(aliases) != 1 {
		t.Fatalf("hostAliases count = %d, want 1", len(aliases))
	}
	if aliases[0].IP != "192.168.4.210" || len(aliases[0].Hostnames) != 1 || aliases[0].Hostnames[0] != "kafka-node2-4-210" {
		t.Fatalf("hostAliases = %+v, want kafka-node2-4-210 mapping", aliases)
	}
}

func TestConfiguredKubernetesVectorImageUsesAggregatorImage(t *testing.T) {
	oldConfig := commonconfig.Config
	defer func() { commonconfig.Config = oldConfig }()

	configPath := writeTempLogtailConfig(t, `
logtail:
  vector:
    image: "legacy/vector:0.20.0-debian"
  aggregator:
    image: "configured/vector:latest-debian"
`)
	if err := commonconfig.LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if got := configuredKubernetesVectorImage(); got != "configured/vector:latest-debian" {
		t.Fatalf("configuredKubernetesVectorImage() = %q, want aggregator image", got)
	}
}

func TestKubernetesClusterConfigIncludesTaskPipeline(t *testing.T) {
	task := logtailmodel.LogCollectTask{
		ID:           5,
		Code:         "zeri_ai_agent_d559ec25",
		K8SLogSource: logtailmodel.LogK8SLogSourceNodeFile,
	}
	config := `
sources:
  task_5_zeri_ai_agent_d559ec25_k8s_node_file_0:
    type: file
`
	if !kubernetesClusterConfigIncludesTaskPipeline(task, 0, config) {
		t.Fatal("node_file pipeline should be detected")
	}
	if kubernetesClusterConfigIncludesTaskPipeline(task, 1, config) {
		t.Fatal("different target index should not match")
	}
	if kubernetesClusterConfigIncludesTaskPipeline(task, 0, "sources: {}") {
		t.Fatal("missing task pipeline should not match")
	}
}

func TestRenderKubernetesVectorBaseConfigUsesRotationSafeDefaults(t *testing.T) {
	content := renderKubernetesVectorBaseConfig([]kubernetesStdoutPipelineTarget{{
		Task: logtailmodel.LogCollectTask{Code: "app_stdout"},
		Target: indexedKubernetesCollectTarget{Target: kubernetesCollectTarget{
			Namespace:    "production",
			WorkloadName: "api",
			Containers:   []string{"api"},
		}},
	}}, false)

	for _, want := range []string{
		`- "/var/log/containers/api-*_production_api-*.log"`,
		"fingerprint:\n      strategy: device_and_inode",
		"read_from: beginning",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("kubernetes stdout source = %q, want %q", content, want)
		}
	}
	if strings.Contains(content, "read_from: end") {
		t.Fatalf("kubernetes stdout source = %q, must read new container logs from beginning", content)
	}
}

func TestRenderKubernetesEmptyBaseConfigKeepsPlaceholderLightweight(t *testing.T) {
	content := renderKubernetesVectorBaseConfig(nil, true)
	if !strings.Contains(content, "k8s_empty_file:") || !strings.Contains(content, "read_from: end") {
		t.Fatalf("empty kubernetes base config = %q, want placeholder source", content)
	}
	if strings.Contains(content, "fingerprint:") {
		t.Fatalf("empty kubernetes base config = %q, placeholder must not persist file identity", content)
	}
}

func TestBuildKubernetesCollectorStatusReturnsConfigMapForDisabledTask(t *testing.T) {
	task := logtailmodel.LogCollectTask{
		ID:           6,
		ProjectID:    1,
		StoreID:      5,
		Name:         "bcl日志采集任务",
		Code:         "bcl_ecde009b",
		Status:       logtailmodel.LogStatusDisabled,
		CollectType:  logtailmodel.LogCollectTypeKubernetes,
		K8SLogSource: logtailmodel.LogK8SLogSourceStdout,
	}
	target := indexedKubernetesCollectTarget{
		Index: 0,
		Target: kubernetesCollectTarget{
			ClusterID:    1,
			Namespace:    "leyoujia-itest",
			WorkloadType: "deployment",
			WorkloadName: "zero-ai-bcl",
			Containers:   []string{"zero-ai-bcl"},
		},
	}
	configYAML := "sources:\n  k8s_stdout_file:\n    type: file\n"
	clientset := fake.NewSimpleClientset(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: k8sClusterCollectorName, Namespace: k8sVectorNamespace},
		Data:       map[string]string{k8sVectorConfigFile: configYAML},
	})

	item := baseKubernetesCollectorVO(&task, &k8smodel.KubeCluster{ID: 1, Name: "itest-测试环境"}, target, logtailmodel.LogK8SCollectorOperation{}, "not_installed", 0, "未安装")
	fillKubernetesCollectorConfig(context.Background(), clientset, &item)
	if item.Status != "not_installed" || item.ProgressText != "任务已停止采集" {
		t.Fatalf("status = %q text = %q, want stopped", item.Status, item.ProgressText)
	}
	if item.ConfigMapNamespace != k8sVectorNamespace || item.ConfigMapName != k8sClusterCollectorName {
		t.Fatalf("configmap = %s/%s, want %s/%s", item.ConfigMapNamespace, item.ConfigMapName, k8sVectorNamespace, k8sClusterCollectorName)
	}
	if item.ConfigYAML != configYAML {
		t.Fatalf("configYaml = %q, want %q", item.ConfigYAML, configYAML)
	}
}

func hasHostPathVolume(volumes []corev1.Volume, name, hostPath string) bool {
	for _, volume := range volumes {
		if volume.Name == name && volume.HostPath != nil && volume.HostPath.Path == hostPath {
			return true
		}
	}
	return false
}

func TestWriteVectorDeliverySinkUsesKafkaWhenConfigured(t *testing.T) {
	oldConfig := commonconfig.Config
	defer func() { commonconfig.Config = oldConfig }()

	configPath := writeTempLogtailConfig(t, `
logtail:
  delivery_mode: "auto"
  victorialogs:
    url: "http://127.0.0.1:9428"
  kafka:
    enabled: true
    brokers:
      - "172.16.7.137:9092"
    topic: "aiops-logs"
  vector:
    disk_buffer_max_size_bytes: 123456
`)
	if err := commonconfig.LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	var builder strings.Builder
	if err := writeVectorDeliverySink(&builder, "sink", "input", false); err != nil {
		t.Fatalf("writeVectorDeliverySink() error = %v", err)
	}
	got := builder.String()
	for _, want := range []string{
		"type: kafka",
		`bootstrap_servers: "172.16.7.137:9092"`,
		`topic: "aiops-logs"`,
		"codec: json",
		"compression: gzip",
		"request.required.acks: all",
		"max_size: 123456",
		"when_full: block",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("sink config = %q, want %q", got, want)
		}
	}
	if strings.Contains(got, "uri:") {
		t.Fatalf("sink config = %q, kafka config should not include victorialogs uri", got)
	}
}

func TestWriteVectorDeliverySinkFailsWhenKafkaForcedWithoutBrokers(t *testing.T) {
	oldConfig := commonconfig.Config
	defer func() { commonconfig.Config = oldConfig }()

	configPath := writeTempLogtailConfig(t, `
logtail:
  delivery_mode: "kafka"
  kafka:
    enabled: true
    topic: "aiops-logs"
`)
	if err := commonconfig.LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	var builder strings.Builder
	if err := writeVectorDeliverySink(&builder, "sink", "input", false); err == nil {
		t.Fatal("writeVectorDeliverySink() error = nil, want kafka config error")
	}
}

func writeTempLogtailConfig(t *testing.T, content string) string {
	t.Helper()
	path := t.TempDir() + "/config.yaml"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestDetectKubernetesVectorVersionFromLog(t *testing.T) {
	logText := `2026-07-09T06:43:21.552880Z INFO vector: Vector has started. debug="false" version="0.20.0" arch="x86_64"`
	if got := detectKubernetesVectorVersionFromLog(logText); got != "0.20.0" {
		t.Fatalf("detectKubernetesVectorVersionFromLog() = %q, want 0.20.0", got)
	}
}

func TestMergeKubernetesCombinedCollectorStatusUsesStdoutAsPrimary(t *testing.T) {
	task := logtailmodel.LogCollectTask{
		ID:           5,
		ProjectID:    1,
		StoreID:      5,
		Name:         "zeri-ai-agent",
		Code:         "zeri_ai_agent_d559ec25",
		CollectType:  logtailmodel.LogCollectTypeKubernetes,
		K8SLogSource: logtailmodel.LogK8SLogSourceBoth,
	}
	target := indexedKubernetesCollectTarget{
		Index: 0,
		Target: kubernetesCollectTarget{
			ClusterID:    1,
			Namespace:    "leyoujia-itest",
			WorkloadType: "deployment",
			WorkloadName: "zero-ai-agent",
			Containers:   []string{"zero-ai-agent"},
		},
	}
	cluster := &k8smodel.KubeCluster{ID: 1, Name: "itest-测试环境", Status: k8smodel.ClusterStatusRunning}

	stdoutItem := logtailmodel.LogK8SCollectorVO{
		ResourceNamespace: k8sVectorNamespace,
		ResourceName:      k8sClusterCollectorName,
		Desired:           6,
		Ready:             6,
		Updated:           6,
		Available:         6,
		Pods: []logtailmodel.LogK8SCollectorPod{
			{Name: "aiops-vector-001", Namespace: k8sVectorNamespace, Phase: string(corev1.PodRunning), Ready: true},
			{Name: "aiops-vector-002", Namespace: k8sVectorNamespace, Phase: string(corev1.PodRunning), Ready: true},
			{Name: "aiops-vector-003", Namespace: k8sVectorNamespace, Phase: string(corev1.PodRunning), Ready: true},
			{Name: "aiops-vector-004", Namespace: k8sVectorNamespace, Phase: string(corev1.PodRunning), Ready: true},
			{Name: "aiops-vector-005", Namespace: k8sVectorNamespace, Phase: string(corev1.PodRunning), Ready: true},
			{Name: "aiops-vector-006", Namespace: k8sVectorNamespace, Phase: string(corev1.PodRunning), Ready: true},
		},
	}
	fileItem := logtailmodel.LogK8SCollectorVO{
		ResourceNamespace: target.Target.Namespace,
		ResourceName:      "aiops-vector-5-leyoujia-itest-zero-ai-agent-file",
		Desired:           1,
		Ready:             1,
		Updated:           1,
		Available:         1,
		Pods: []logtailmodel.LogK8SCollectorPod{
			{Name: "zero-ai-agent-001", Namespace: target.Target.Namespace, Phase: string(corev1.PodRunning), Ready: true},
		},
	}

	item := mergeKubernetesCombinedCollectorStatus(task, cluster, target, stdoutItem, fileItem, nil, nil)
	if item.Desired != 6 || item.Ready != 6 || item.Updated != 6 || item.Available != 6 {
		t.Fatalf("replicas = %d/%d/%d/%d, want stdout daemonset 6/6/6/6", item.Desired, item.Ready, item.Updated, item.Available)
	}
	if len(item.Pods) != 6 {
		t.Fatalf("pod count = %d, want stdout daemonset pods: 6", len(item.Pods))
	}
	for _, pod := range item.Pods {
		if !strings.HasPrefix(pod.Name, "aiops-vector") {
			t.Fatalf("combined collector pods should use cluster daemonset pod %q as primary", pod.Name)
		}
	}
	if item.Status != "running" {
		t.Fatalf("status = %q, want running", item.Status)
	}
	if len(item.Components) != 2 {
		t.Fatalf("component count = %d, want stdout and container_file", len(item.Components))
	}
	if item.Components[0].Source != logtailmodel.LogK8SLogSourceStdout || item.Components[0].Desired != 6 || item.Components[0].ReadyPodCount != 6 {
		t.Fatalf("stdout component = %+v, want 6/6", item.Components[0])
	}
	if item.Components[1].Source != logtailmodel.LogK8SLogSourceContainerFile || item.Components[1].Desired != 1 || item.Components[1].ReadyPodCount != 1 {
		t.Fatalf("container_file component = %+v, want 1/1", item.Components[1])
	}
}
