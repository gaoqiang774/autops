package service

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	logtailmodel "dodevops-api/api/logtail/model"

	"github.com/go-sql-driver/mysql"
	corev1 "k8s.io/api/core/v1"
)

func TestShouldAutoApplyCollectTask(t *testing.T) {
	tests := []struct {
		name    string
		task    *logtailmodel.LogCollectTask
		wantOK  bool
		wantMsg string
	}{
		{
			name: "enabled host task with selected targets auto applies",
			task: &logtailmodel.LogCollectTask{
				CollectType:        "host",
				ExecuteMode:        "selected_agents",
				ExecuteTargetsJSON: `[{"agentId":1,"hostId":1}]`,
				Status:             logtailmodel.LogStatusEnabled,
			},
			wantOK: true,
		},
		{
			name: "host all agents skips auto apply",
			task: &logtailmodel.LogCollectTask{
				CollectType: "host",
				ExecuteMode: "all_agents",
				Status:      logtailmodel.LogStatusEnabled,
			},
			wantMsg: "主机日志采集不再支持全部 Agent 自动下发，请选择具体 Agent",
		},
		{
			name: "host without targets skips auto apply",
			task: &logtailmodel.LogCollectTask{
				CollectType: "host",
				ExecuteMode: "selected_agents",
				Status:      logtailmodel.LogStatusEnabled,
			},
			wantMsg: "主机日志采集未选择执行 Agent，跳过自动下发",
		},
		{
			name: "disabled task skips auto apply",
			task: &logtailmodel.LogCollectTask{
				CollectType: "host",
				Status:      logtailmodel.LogStatusDisabled,
			},
			wantMsg: disabledCollectTaskConfigMessage(),
		},
		{
			name: "k8s task without target skips auto apply",
			task: &logtailmodel.LogCollectTask{
				CollectType: "k8s",
				Status:      logtailmodel.LogStatusEnabled,
			},
			wantMsg: "K8S日志采集未选择目标，跳过自动下发",
		},
		{
			name: "k8s task with workload target auto applies",
			task: &logtailmodel.LogCollectTask{
				CollectType:        "k8s",
				ExecuteTargetsJSON: `[{"clusterId":1,"namespace":"prod","workloadType":"deployment","workloadName":"api-service"}]`,
				Status:             logtailmodel.LogStatusEnabled,
			},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOK, gotMsg := shouldAutoApplyCollectTask(tt.task)
			if gotOK != tt.wantOK {
				t.Fatalf("shouldAutoApplyCollectTask() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if gotMsg != tt.wantMsg {
				t.Fatalf("shouldAutoApplyCollectTask() msg = %q, want %q", gotMsg, tt.wantMsg)
			}
		})
	}
}

func TestCollectTaskAutoApplyStatus(t *testing.T) {
	tests := []struct {
		name    string
		summary logtailmodel.LogCollectTaskApplySummary
		want    string
	}{
		{
			name: "created when operations were generated",
			summary: logtailmodel.LogCollectTaskApplySummary{
				MatchedAgents:  2,
				InstallCreated: 1,
				ReloadCreated:  1,
			},
			want: "created",
		},
		{
			name: "skipped when no agents matched",
			summary: logtailmodel.LogCollectTaskApplySummary{
				MatchedAgents: 0,
			},
			want: "skipped",
		},
		{
			name: "partial failed when mixed results exist",
			summary: logtailmodel.LogCollectTaskApplySummary{
				MatchedAgents: 3,
				ReloadCreated: 1,
				Skipped:       1,
				Failed:        1,
			},
			want: "partial_failed",
		},
		{
			name: "failed when all matched agents failed",
			summary: logtailmodel.LogCollectTaskApplySummary{
				MatchedAgents: 2,
				Failed:        2,
			},
			want: "failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectTaskAutoApplyStatus(tt.summary)
			if got != tt.want {
				t.Fatalf("collectTaskAutoApplyStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEnsureCollectTaskStoppedForDelete(t *testing.T) {
	tests := []struct {
		name    string
		task    *logtailmodel.LogCollectTask
		wantErr error
	}{
		{
			name:    "disabled task can be deleted",
			task:    &logtailmodel.LogCollectTask{Status: logtailmodel.LogStatusDisabled},
			wantErr: nil,
		},
		{
			name:    "enabled task cannot be deleted",
			task:    &logtailmodel.LogCollectTask{Status: logtailmodel.LogStatusEnabled},
			wantErr: ErrCollectTaskDeleteRequiresStopped,
		},
		{
			name:    "zero status cannot be deleted",
			task:    &logtailmodel.LogCollectTask{},
			wantErr: ErrCollectTaskDeleteRequiresStopped,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureCollectTaskStoppedForDelete(tt.task)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("ensureCollectTaskStoppedForDelete() error = %v, want nil", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("ensureCollectTaskStoppedForDelete() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnsureHostCollectTaskDeletable(t *testing.T) {
	tests := []struct {
		name    string
		agents  []logtailmodel.LogAgentVO
		wantErr error
	}{
		{
			name: "running applied host task cannot be deleted",
			agents: []logtailmodel.LogAgentVO{{
				VectorStatus: logtailmodel.VectorStatusRunning,
				ConfigStatus: logtailmodel.LogAgentConfigStatusApplied,
			}},
			wantErr: ErrCollectTaskDeleteRequiresStopped,
		},
		{
			name: "not installed pending host task can be deleted",
			agents: []logtailmodel.LogAgentVO{{
				VectorStatus: logtailmodel.VectorStatusNotInstalled,
				ConfigStatus: logtailmodel.LogAgentConfigStatusPending,
			}},
			wantErr: nil,
		},
		{
			name: "config failed host task cannot be deleted",
			agents: []logtailmodel.LogAgentVO{{
				VectorStatus: logtailmodel.VectorStatusRunning,
				ConfigStatus: logtailmodel.LogAgentConfigStatusFailed,
			}},
			wantErr: errors.New("采集任务命中主机存在异常状态，请先处理或卸载后再删除"),
		},
		{
			name:    "no matched agents can be deleted",
			agents:  nil,
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureHostCollectTaskDeletable(tt.agents)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("ensureHostCollectTaskDeletable() error = %v, want nil", err)
			}
			if tt.wantErr != nil && err == nil {
				t.Fatalf("ensureHostCollectTaskDeletable() error = nil, want %v", tt.wantErr)
			}
			if tt.wantErr != nil && errors.Is(tt.wantErr, ErrCollectTaskDeleteRequiresStopped) && !errors.Is(err, tt.wantErr) {
				t.Fatalf("ensureHostCollectTaskDeletable() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseAgentOperationCollectTaskID(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want uint
	}{
		{name: "valid request json", raw: `{"collectTaskId":3,"configVersion":2}`, want: 3},
		{name: "missing collect task id", raw: `{"configVersion":2}`, want: 0},
		{name: "invalid json", raw: `{`, want: 0},
		{name: "empty json", raw: ``, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseAgentOperationCollectTaskID(tt.raw); got != tt.want {
				t.Fatalf("parseAgentOperationCollectTaskID() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNormalizeCollectTaskForCreate(t *testing.T) {
	task := &logtailmodel.LogCollectTask{
		Name:   "Nginx Access Collect",
		Status: logtailmodel.LogStatusEnabled,
	}

	normalizeCollectTaskForCreate(task)

	if task.Code == "" {
		t.Fatal("normalizeCollectTaskForCreate() should generate code")
	}
	if !strings.HasPrefix(task.Code, "nginx_access_collect_") {
		t.Fatalf("normalizeCollectTaskForCreate() code = %q, want prefix %q", task.Code, "nginx_access_collect_")
	}
	if task.ParseModeSnapshot != "raw" {
		t.Fatalf("normalizeCollectTaskForCreate() parseModeSnapshot = %q, want %q", task.ParseModeSnapshot, "raw")
	}
	if task.ConfigVersion != 1 {
		t.Fatalf("normalizeCollectTaskForCreate() configVersion = %d, want %d", task.ConfigVersion, 1)
	}
}

func TestCollectTaskNameDedupKey(t *testing.T) {
	first := &logtailmodel.LogCollectTask{ProjectID: 7, Name: "  Zero AI JSM  "}
	second := &logtailmodel.LogCollectTask{ProjectID: 7, Name: "zero   ai jsm"}
	if collectTaskNameDedupKey(first) != collectTaskNameDedupKey(second) {
		t.Fatal("name key should ignore case and redundant whitespace in the same project")
	}
	second.ProjectID = 8
	if collectTaskNameDedupKey(first) == collectTaskNameDedupKey(second) {
		t.Fatal("name key should allow the same task name in different projects")
	}
}

func TestCollectTaskConfigDedupKeyCanonicalizesCollectionTarget(t *testing.T) {
	first := &logtailmodel.LogCollectTask{
		ProjectID:          7,
		StoreID:            9,
		CollectType:        "kubernetes",
		ExecuteMode:        "selected_workloads",
		K8SLogSource:       logtailmodel.LogK8SLogSourceContainerFile,
		ExecuteTargetsJSON: `[{"clusterId":2,"namespace":"prod","workloadType":"Deployment","workloadName":"api","containers":["worker","app"]},{"clusterId":1,"namespace":"prod","workloadType":"deployment","workloadName":"web","containers":["web"]}]`,
		PathPatterns:       []string{"/app/logs/error.log", "/app/logs/access.log"},
		ParseModeSnapshot:  "json",
	}
	second := &logtailmodel.LogCollectTask{
		ProjectID:          7,
		StoreID:            9,
		CollectType:        "k8s",
		ExecuteMode:        "SELECTED_WORKLOADS",
		K8SLogSource:       "file",
		ExecuteTargetsJSON: `[{"workloadName":"web","workloadType":"Deployment","namespace":"prod","clusterId":"1","containers":["web"]},{"workloadName":"api","workloadType":"deployment","namespace":"prod","clusterId":"2","containers":["app","worker"]}]`,
		PathPatterns:       []string{"/app/logs/access.log", "/app/logs/error.log"},
		ParseModeSnapshot:  "json",
	}
	normalizeCollectTaskForCreate(first)
	normalizeCollectTaskForCreate(second)
	if collectTaskConfigDedupKey(first) != collectTaskConfigDedupKey(second) {
		t.Fatal("config key should be stable across equivalent target, container, and path ordering")
	}
}

func TestCollectTaskConfigDedupKeyIncludesProcessingConfig(t *testing.T) {
	first := &logtailmodel.LogCollectTask{
		ProjectID:          7,
		StoreID:            9,
		CollectType:        "host",
		ExecuteMode:        "selected_agents",
		ExecuteTargetsJSON: `[{"agentId":1,"hostId":2}]`,
		PathPatterns:       []string{"/var/log/app.log"},
		ParseModeSnapshot:  "raw",
	}
	second := *first
	second.ParseModeSnapshot = "json"
	if collectTaskConfigDedupKey(first) == collectTaskConfigDedupKey(&second) {
		t.Fatal("config key should distinguish different processing configurations")
	}
}

func TestIsCollectTaskUniqueConstraintError(t *testing.T) {
	if !isCollectTaskUniqueConstraintError(&mysql.MySQLError{
		Number: 1062, Message: "Duplicate entry for key 'uk_log_collect_task_config_key'",
	}) {
		t.Fatal("config key collision should be recognized as a collect task conflict")
	}
	if isCollectTaskUniqueConstraintError(&mysql.MySQLError{
		Number: 1062, Message: "Duplicate entry for key 'uk_log_collect_task_code'",
	}) {
		t.Fatal("code collision should not be reported as a duplicate collection config")
	}
}

func TestNextCollectTaskConfigVersion(t *testing.T) {
	existing := &logtailmodel.LogCollectTask{
		ProjectID:                1,
		StoreID:                  1,
		Name:                     "old",
		Code:                     "old_code",
		CollectType:              "host",
		ExecuteMode:              "selected_agents",
		ExecuteTargetsJSON:       `[{"agentId":1,"hostId":1}]`,
		PathPattern:              "/var/log/a.log",
		ParseModeSnapshot:        "raw",
		MultilinePatternSnapshot: "",
		LabelsJSON:               "",
		FiltersJSON:              "",
		ConfigVersion:            3,
		Status:                   logtailmodel.LogStatusEnabled,
	}

	t.Run("keep existing version when config does not change", func(t *testing.T) {
		incoming := *existing
		incoming.ConfigVersion = 0

		got := nextCollectTaskConfigVersion(existing, &incoming)
		if got != 3 {
			t.Fatalf("nextCollectTaskConfigVersion() = %d, want %d", got, 3)
		}
	})

	t.Run("bump version when config changes and version omitted", func(t *testing.T) {
		incoming := *existing
		incoming.Name = "new"
		incoming.ConfigVersion = 0

		got := nextCollectTaskConfigVersion(existing, &incoming)
		if got != 4 {
			t.Fatalf("nextCollectTaskConfigVersion() = %d, want %d", got, 4)
		}
	})

	t.Run("respect explicit version", func(t *testing.T) {
		incoming := *existing
		incoming.ConfigVersion = 9

		got := nextCollectTaskConfigVersion(existing, &incoming)
		if got != 9 {
			t.Fatalf("nextCollectTaskConfigVersion() = %d, want %d", got, 9)
		}
	})
}

func TestValidateCollectTaskRequiresExplicitHostTargets(t *testing.T) {
	base := logtailmodel.LogCollectTask{
		ProjectID:     1,
		StoreID:       1,
		Name:          "syslog",
		Code:          "syslog",
		CollectType:   "host",
		PathPattern:   "/var/log/messages",
		ConfigVersion: 1,
		Status:        logtailmodel.LogStatusEnabled,
	}

	t.Run("reject all agents mode", func(t *testing.T) {
		task := base
		task.ExecuteMode = "all_agents"

		err := validateCollectTask(&task)
		if err == nil || err.Error() != "主机日志采集不支持全部 Agent，请选择具体 Agent" {
			t.Fatalf("validateCollectTask() error = %v", err)
		}
	})

	t.Run("reject selected agents without targets", func(t *testing.T) {
		task := base
		task.ExecuteMode = "selected_agents"

		err := validateCollectTask(&task)
		if err == nil || err.Error() != "主机日志采集必须选择执行 Agent" {
			t.Fatalf("validateCollectTask() error = %v", err)
		}
	})

	t.Run("accept selected targets", func(t *testing.T) {
		task := base
		task.ExecuteMode = "selected_agents"
		task.ExecuteTargetsJSON = `[{"agentId":1,"hostId":1}]`

		if err := validateCollectTask(&task); err != nil {
			t.Fatalf("validateCollectTask() error = %v", err)
		}
	})
}

func TestValidateKubernetesCollectTask(t *testing.T) {
	base := logtailmodel.LogCollectTask{
		ProjectID:          1,
		StoreID:            1,
		Name:               "api workload logs",
		Code:               "api_workload_logs",
		CollectType:        "kubernetes",
		ExecuteMode:        "selected_workloads",
		ExecuteTargetsJSON: `[{"clusterId":1,"namespace":"prod","workloadType":"deployment","workloadName":"api-service","containers":["app"]}]`,
		ParseModeSnapshot:  "json",
		ConfigVersion:      1,
		Status:             logtailmodel.LogStatusEnabled,
	}

	t.Run("accept stdout without path", func(t *testing.T) {
		task := base
		task.K8SLogSource = logtailmodel.LogK8SLogSourceStdout

		if err := validateCollectTask(&task); err != nil {
			t.Fatalf("validateCollectTask() error = %v", err)
		}
	})

	t.Run("reject container file without path", func(t *testing.T) {
		task := base
		task.K8SLogSource = logtailmodel.LogK8SLogSourceContainerFile

		err := validateCollectTask(&task)
		if err == nil || err.Error() != "容器内文件日志采集必须填写日志路径" {
			t.Fatalf("validateCollectTask() error = %v", err)
		}
	})

	t.Run("accept container file with path", func(t *testing.T) {
		task := base
		task.K8SLogSource = logtailmodel.LogK8SLogSourceContainerFile
		task.PathPatterns = []string{"/app/logs/*.log"}
		normalizeCollectTaskPaths(&task)

		if err := validateCollectTask(&task); err != nil {
			t.Fatalf("validateCollectTask() error = %v", err)
		}
	})

	t.Run("accept node file with cluster target and path", func(t *testing.T) {
		task := base
		task.K8SLogSource = logtailmodel.LogK8SLogSourceNodeFile
		task.ExecuteTargetsJSON = `[{"clusterId":1}]`
		task.PathPatterns = []string{"/data/logs/tomcatlogs/**/*.log"}
		normalizeCollectTaskPaths(&task)

		if err := validateCollectTask(&task); err != nil {
			t.Fatalf("validateCollectTask() error = %v", err)
		}
	})

	t.Run("reject node file without path", func(t *testing.T) {
		task := base
		task.K8SLogSource = logtailmodel.LogK8SLogSourceNodeFile
		task.ExecuteTargetsJSON = `[{"clusterId":1}]`

		err := validateCollectTask(&task)
		if err == nil || err.Error() != "节点文件日志采集必须填写宿主机日志路径" {
			t.Fatalf("validateCollectTask() error = %v", err)
		}
	})

	t.Run("accept both with path", func(t *testing.T) {
		task := base
		task.K8SLogSource = logtailmodel.LogK8SLogSourceBoth
		task.PathPatterns = []string{"/app/logs/*.log"}
		normalizeCollectTaskPaths(&task)

		if err := validateCollectTask(&task); err != nil {
			t.Fatalf("validateCollectTask() error = %v", err)
		}
	})

	t.Run("reject both without path", func(t *testing.T) {
		task := base
		task.K8SLogSource = logtailmodel.LogK8SLogSourceBoth

		err := validateCollectTask(&task)
		if err == nil || err.Error() != "同时采集标准输出和容器内文件时必须填写容器内日志路径" {
			t.Fatalf("validateCollectTask() error = %v", err)
		}
	})

	t.Run("reject missing workload target", func(t *testing.T) {
		task := base
		task.K8SLogSource = logtailmodel.LogK8SLogSourceStdout
		task.ExecuteTargetsJSON = `[{"clusterId":1,"namespace":"prod"}]`

		err := validateCollectTask(&task)
		if err == nil || err.Error() != "K8S日志采集目标第1项缺少workloadType" {
			t.Fatalf("validateCollectTask() error = %v", err)
		}
	})
}

func TestNormalizeKubernetesCollectTaskForCreate(t *testing.T) {
	task := &logtailmodel.LogCollectTask{
		Name:               "api workload logs",
		CollectType:        "k8s",
		ExecuteTargetsJSON: `[{"clusterId":1,"namespace":"prod","workloadType":"deployment","workloadName":"api-service"}]`,
	}

	normalizeCollectTaskForCreate(task)

	if task.CollectType != logtailmodel.LogCollectTypeKubernetes {
		t.Fatalf("CollectType = %q, want %q", task.CollectType, logtailmodel.LogCollectTypeKubernetes)
	}
	if task.K8SLogSource != logtailmodel.LogK8SLogSourceStdout {
		t.Fatalf("K8SLogSource = %q, want stdout", task.K8SLogSource)
	}
}

func TestNormalizeKubernetesLogSourceBothAliases(t *testing.T) {
	for _, raw := range []string{"both", "all", "stdout+container_file", "stdout,container_file", "container_file,stdout"} {
		if got := normalizeKubernetesLogSource(raw); got != logtailmodel.LogK8SLogSourceBoth {
			t.Fatalf("normalizeKubernetesLogSource(%q) = %q, want both", raw, got)
		}
	}
	if !kubernetesLogSourceIncludes(logtailmodel.LogK8SLogSourceBoth, logtailmodel.LogK8SLogSourceStdout) {
		t.Fatal("both should include stdout")
	}
	if !kubernetesLogSourceIncludes(logtailmodel.LogK8SLogSourceBoth, logtailmodel.LogK8SLogSourceContainerFile) {
		t.Fatal("both should include container_file")
	}
}

func TestNormalizeKubernetesLogSourceNodeFileAliases(t *testing.T) {
	for _, raw := range []string{"node_file", "host_path", "hostpath", "host_path_file", "node_path"} {
		if got := normalizeKubernetesLogSource(raw); got != logtailmodel.LogK8SLogSourceNodeFile {
			t.Fatalf("normalizeKubernetesLogSource(%q) = %q, want node_file", raw, got)
		}
	}
}

func TestParseKubernetesCollectTargets(t *testing.T) {
	targets, err := parseKubernetesCollectTargets(`[{"clusterId":"2","namespace":"prod","workloadType":"Deployment","workloadName":"api-service","containers":["app","worker"]}]`)
	if err != nil {
		t.Fatalf("parseKubernetesCollectTargets() error = %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("targets len = %d, want 1", len(targets))
	}
	target := targets[0]
	if target.ClusterID != 2 || target.Namespace != "prod" || target.WorkloadType != "deployment" || target.WorkloadName != "api-service" {
		t.Fatalf("target = %+v, want normalized deployment target", target)
	}
	if strings.Join(target.Containers, ",") != "app,worker" {
		t.Fatalf("containers = %v, want app,worker", target.Containers)
	}
}

func TestParseKubernetesNodeFileTargetAllowsClusterOnly(t *testing.T) {
	targets, err := parseIndexedKubernetesCollectTargetsBySource(`[{"clusterId":"2"}]`, logtailmodel.LogK8SLogSourceNodeFile)
	if err != nil {
		t.Fatalf("parseIndexedKubernetesCollectTargetsBySource() error = %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("targets len = %d, want 1", len(targets))
	}
	target := targets[0].Target
	if target.ClusterID != 2 || target.Namespace != k8sVectorNamespace || target.WorkloadType != logtailmodel.LogK8SLogSourceNodeFile || target.WorkloadName != k8sClusterCollectorName {
		t.Fatalf("target = %+v, want cluster-level node_file target", target)
	}
}

func TestKubernetesContainerFileVRLPattern(t *testing.T) {
	pattern := kubernetesContainerFileVRLPattern(kubernetesCollectTarget{
		Namespace:    "prod",
		WorkloadName: "api-service",
		Containers:   []string{"app"},
	})
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("regexp.Compile() error = %v", err)
	}
	if !compiled.MatchString("/var/log/containers/api-service-7dbd9-prod_prod_app-cid.log") {
		t.Fatalf("pattern %q should match selected workload container log", pattern)
	}
	if compiled.MatchString("/var/log/containers/api-service-7dbd9-prod_prod_worker-cid.log") {
		t.Fatalf("pattern %q should not match unselected container", pattern)
	}
}

func TestMutatePodTemplateForContainerFileCollector(t *testing.T) {
	task := logtailmodel.LogCollectTask{
		ID:                9,
		Code:              "collect_api",
		K8SLogSource:      logtailmodel.LogK8SLogSourceContainerFile,
		PathPatterns:      []string{"/app/logs/*.log"},
		ConfigVersion:     1,
		ParseModeSnapshot: "json",
	}
	normalizeCollectTaskPaths(&task)
	template := &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app"},
				{Name: "metrics"},
			},
		},
	}

	mutated, err := mutatePodTemplateForContainerFileCollector(template, task, kubernetesCollectTarget{
		Namespace:    "prod",
		WorkloadType: "deployment",
		WorkloadName: "api-service",
		Containers:   []string{"app"},
	}, "aiops-vector-9", "abc123")
	if err != nil {
		t.Fatalf("mutatePodTemplateForContainerFileCollector() error = %v", err)
	}
	if !mutated {
		t.Fatalf("mutated = false, want true")
	}
	if len(template.Spec.Containers) != 3 {
		t.Fatalf("container count = %d, want app + metrics + sidecar", len(template.Spec.Containers))
	}
	app := template.Spec.Containers[0]
	if !hasVolumeMount(app.VolumeMounts, "/app/logs") {
		t.Fatalf("app container should mount /app/logs shared volume")
	}
	metrics := template.Spec.Containers[1]
	if hasVolumeMount(metrics.VolumeMounts, "/app/logs") {
		t.Fatalf("unselected metrics container should not mount /app/logs")
	}
	sidecar := template.Spec.Containers[2]
	if sidecar.Name != "aiops-vector-9" {
		t.Fatalf("sidecar name = %q, want aiops-vector-9", sidecar.Name)
	}
	if !hasVolumeMount(sidecar.VolumeMounts, "/app/logs") {
		t.Fatalf("sidecar should mount /app/logs readonly")
	}
	if !hasVolumeMount(sidecar.VolumeMounts, k8sVectorDataDir) {
		t.Fatalf("sidecar should mount vector data dir")
	}
	if !hasVolume(template.Spec.Volumes, k8sVectorDataVolume) {
		t.Fatalf("pod template should include vector data volume")
	}
	if template.Annotations["aiops.dev/log-collector-config-hash"] != "abc123" {
		t.Fatalf("config hash annotation = %q, want abc123", template.Annotations["aiops.dev/log-collector-config-hash"])
	}
}

func hasVolume(volumes []corev1.Volume, name string) bool {
	for _, volume := range volumes {
		if volume.Name == name {
			return true
		}
	}
	return false
}

func hasVolumeMount(mounts []corev1.VolumeMount, mountPath string) bool {
	for _, mount := range mounts {
		if mount.MountPath == mountPath {
			return true
		}
	}
	return false
}

func TestNormalizeVectorStatus(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty defaults to not installed", raw: "", want: logtailmodel.VectorStatusNotInstalled},
		{name: "installed stays installed", raw: "installed", want: logtailmodel.VectorStatusInstalled},
		{name: "running stays running", raw: "running", want: logtailmodel.VectorStatusRunning},
		{name: "stopped is normalized to error", raw: "stopped", want: logtailmodel.VectorStatusError},
		{name: "unknown is normalized to error", raw: "unknown", want: logtailmodel.VectorStatusError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeVectorStatus(tt.raw)
			if got != tt.want {
				t.Fatalf("normalizeVectorStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildCollectTaskAgentSummaryCountsInstalled(t *testing.T) {
	summary := buildCollectTaskAgentSummary([]logtailmodel.LogAgentVO{
		{VectorStatus: logtailmodel.VectorStatusInstalled},
		{VectorStatus: logtailmodel.VectorStatusRunning},
		{VectorStatus: logtailmodel.VectorStatusNotInstalled},
		{VectorStatus: logtailmodel.VectorStatusError},
	})

	if summary.VectorInstalled != 1 {
		t.Fatalf("VectorInstalled = %d, want 1", summary.VectorInstalled)
	}
	if summary.VectorRunning != 1 {
		t.Fatalf("VectorRunning = %d, want 1", summary.VectorRunning)
	}
	if summary.VectorNotInstalled != 1 {
		t.Fatalf("VectorNotInstalled = %d, want 1", summary.VectorNotInstalled)
	}
	if summary.VectorError != 1 {
		t.Fatalf("VectorError = %d, want 1", summary.VectorError)
	}
}

func TestApplyCollectTaskAgentPresentationDisabledTask(t *testing.T) {
	task := &logtailmodel.LogCollectTask{Status: logtailmodel.LogStatusDisabled}
	items := []logtailmodel.LogAgentVO{{
		ConfigStatus: logtailmodel.LogAgentConfigStatusPending,
	}}

	applyCollectTaskAgentPresentation(task, items)
	if items[0].ConfigStatus != logtailmodel.LogAgentConfigStatusDisabled {
		t.Fatalf("ConfigStatus = %q, want %q", items[0].ConfigStatus, logtailmodel.LogAgentConfigStatusDisabled)
	}
	if items[0].ConfigStatusText != disabledCollectTaskConfigMessage() {
		t.Fatalf("ConfigStatusText = %q, want %q", items[0].ConfigStatusText, disabledCollectTaskConfigMessage())
	}

	summary := buildCollectTaskAgentSummary(items)
	if summary.ConfigDisabled != 1 || summary.ConfigPending != 0 {
		t.Fatalf("summary disabled=%d pending=%d, want disabled=1 pending=0", summary.ConfigDisabled, summary.ConfigPending)
	}
}

func TestLogAgentConfigStatusText(t *testing.T) {
	tests := []struct {
		name string
		item logtailmodel.LogAgentVO
		want string
	}{
		{
			name: "running failed new config not applied",
			item: logtailmodel.LogAgentVO{
				VectorStatus:         logtailmodel.VectorStatusRunning,
				ConfigStatus:         logtailmodel.LogAgentConfigStatusFailed,
				DesiredConfigVersion: 3,
				AppliedConfigVersion: 2,
			},
			want: "运行中，新配置未生效",
		},
		{
			name: "failed without running vector",
			item: logtailmodel.LogAgentVO{
				VectorStatus:         logtailmodel.VectorStatusError,
				ConfigStatus:         logtailmodel.LogAgentConfigStatusFailed,
				DesiredConfigVersion: 3,
				AppliedConfigVersion: 2,
			},
			want: "配置应用失败",
		},
		{
			name: "pending new config",
			item: logtailmodel.LogAgentVO{
				ConfigStatus:         logtailmodel.LogAgentConfigStatusPending,
				DesiredConfigVersion: 3,
				AppliedConfigVersion: 2,
			},
			want: "新配置待生效",
		},
		{
			name: "applied",
			item: logtailmodel.LogAgentVO{
				ConfigStatus: logtailmodel.LogAgentConfigStatusApplied,
			},
			want: "配置已生效",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := logAgentConfigStatusText(tt.item)
			if got != tt.want {
				t.Fatalf("logAgentConfigStatusText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildDisabledCollectTaskApplyResponse(t *testing.T) {
	resp := buildDisabledCollectTaskApplyResponse(logtailmodel.LogCollectTask{ID: 7}, []logtailmodel.LogAgentVO{{
		AgentID:      354,
		HostID:       57,
		HostName:     "itest-nginx",
		VectorStatus: logtailmodel.VectorStatusInstalled,
	}}, 1)

	if resp.Summary.MatchedAgents != 1 || resp.Summary.Skipped != 1 {
		t.Fatalf("summary = %+v, want matched=1 skipped=1", resp.Summary)
	}
	if len(resp.Results) != 1 || resp.Results[0].Status != "skipped" {
		t.Fatalf("results = %+v, want one skipped result", resp.Results)
	}
	if resp.Results[0].Message != disabledCollectTaskConfigMessage() {
		t.Fatalf("message = %q, want %q", resp.Results[0].Message, disabledCollectTaskConfigMessage())
	}
}

func TestShouldReinstallVector(t *testing.T) {
	tests := []struct {
		name  string
		agent *logtailmodel.LogAgentVO
		want  bool
	}{
		{
			name: "error with empty version should reinstall",
			agent: &logtailmodel.LogAgentVO{
				VectorStatus: "error",
			},
			want: true,
		},
		{
			name: "error with zero config versions should reinstall",
			agent: &logtailmodel.LogAgentVO{
				VectorStatus:         "error",
				VectorVersion:        "0.56.0",
				InstallPath:          "/opt/aiops/vector/current",
				DesiredConfigVersion: 0,
				AppliedConfigVersion: 0,
			},
			want: true,
		},
		{
			name: "running should not reinstall",
			agent: &logtailmodel.LogAgentVO{
				VectorStatus:         "running",
				VectorVersion:        "0.56.0",
				InstallPath:          "/opt/aiops/vector/current",
				DesiredConfigVersion: 1,
				AppliedConfigVersion: 1,
			},
			want: false,
		},
		{
			name: "error with installed config should prefer reload",
			agent: &logtailmodel.LogAgentVO{
				VectorStatus:         "error",
				VectorVersion:        "0.56.0",
				InstallPath:          "/opt/aiops/vector/current",
				DesiredConfigVersion: 2,
				AppliedConfigVersion: 1,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldReinstallVector(tt.agent)
			if got != tt.want {
				t.Fatalf("shouldReinstallVector() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollectTaskApplyOperationTypesForceReloadsAppliedConfig(t *testing.T) {
	got := collectTaskApplyOperationTypes(&logtailmodel.LogAgentVO{
		VectorStatus:         logtailmodel.VectorStatusRunning,
		VectorVersion:        "0.56.0",
		InstallPath:          "/opt/aiops/vector/current",
		ConfigStatus:         logtailmodel.LogAgentConfigStatusApplied,
		AppliedConfigVersion: 2,
		AppliedConfigHash:    "sha256:old",
	}, &logtailmodel.LogAgentConfigPreview{
		ConfigVersion: 2,
		ConfigHash:    "sha256:old",
	}, true)

	if len(got) != 1 || got[0] != logtailmodel.LogAgentOperationReloadVector {
		t.Fatalf("collectTaskApplyOperationTypes(force=true) = %v, want [reload_vector]", got)
	}
}

func TestIsAllowedAgentOperation(t *testing.T) {
	if !isAllowedAgentOperation(logtailmodel.LogAgentOperationUninstallVector) {
		t.Fatal("isAllowedAgentOperation() should allow uninstall_vector")
	}
}

func TestShouldReplaceActiveAgentOperation(t *testing.T) {
	tests := []struct {
		name   string
		req    logtailmodel.LogAgentOperationRequest
		active *logtailmodel.LogAgentOperation
		want   bool
	}{
		{
			name: "pending install can be replaced",
			req:  logtailmodel.LogAgentOperationRequest{OpType: logtailmodel.LogAgentOperationInstallVector},
			active: &logtailmodel.LogAgentOperation{
				OpType:   logtailmodel.LogAgentOperationInstallVector,
				OpStatus: logtailmodel.LogAgentOperationStatusPending,
			},
			want: true,
		},
		{
			name: "running install cannot be replaced",
			req:  logtailmodel.LogAgentOperationRequest{OpType: logtailmodel.LogAgentOperationInstallVector},
			active: &logtailmodel.LogAgentOperation{
				OpType:   logtailmodel.LogAgentOperationInstallVector,
				OpStatus: logtailmodel.LogAgentOperationStatusRunning,
			},
			want: false,
		},
		{
			name: "pending reload needs force",
			req:  logtailmodel.LogAgentOperationRequest{OpType: logtailmodel.LogAgentOperationReloadVector},
			active: &logtailmodel.LogAgentOperation{
				OpType:   logtailmodel.LogAgentOperationReloadVector,
				OpStatus: logtailmodel.LogAgentOperationStatusPending,
			},
			want: false,
		},
		{
			name: "force replaces pending reload",
			req:  logtailmodel.LogAgentOperationRequest{OpType: logtailmodel.LogAgentOperationReloadVector, Force: true},
			active: &logtailmodel.LogAgentOperation{
				OpType:   logtailmodel.LogAgentOperationReloadVector,
				OpStatus: logtailmodel.LogAgentOperationStatusPending,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldReplaceActiveAgentOperation(tt.req, tt.active)
			if got != tt.want {
				t.Fatalf("shouldReplaceActiveAgentOperation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldUseLegacyHTTPSinkConfig(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{version: "0.20.0", want: true},
		{version: "vector 0.20.1", want: true},
		{version: "0.22.9", want: true},
		{version: "0.23.0", want: false},
		{version: "0.56.0", want: false},
		{version: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := shouldUseLegacyHTTPSinkConfig(tt.version)
			if got != tt.want {
				t.Fatalf("shouldUseLegacyHTTPSinkConfig(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestShouldDropVectorSelfLogs(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/var/log/messages", want: true},
		{path: "/var/log/syslog", want: true},
		{path: "/var/log/nginx/access.log", want: false},
		{path: "/data/logs/app.log", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := shouldDropVectorSelfLogs(logtailmodel.LogCollectTask{PathPattern: tt.path})
			if got != tt.want {
				t.Fatalf("shouldDropVectorSelfLogs(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestRenderVectorConfigWithoutTasksReturnsEmpty(t *testing.T) {
	service := &LogtailService{}

	content, version, err := service.renderVectorConfig(&logtailmodel.LogAgentVO{
		AgentID:  1,
		HostID:   1,
		HostName: "node-1",
	}, nil, "0.20.0")
	if err != nil {
		t.Fatalf("renderVectorConfig() error = %v", err)
	}
	if content != "" {
		t.Fatalf("renderVectorConfig() content = %q, want empty", content)
	}
	if version != 0 {
		t.Fatalf("renderVectorConfig() version = %d, want 0", version)
	}
}

func TestWriteVectorHostFileSourceUsesRotationSafeDefaults(t *testing.T) {
	var builder strings.Builder
	writeVectorHostFileSource(&builder, logtailmodel.LogCollectTask{
		Code:        "mysql_slow",
		PathPattern: "/data/logs/slow*.log",
	})

	content := builder.String()
	for _, want := range []string{
		"type: file",
		`- "/data/logs/slow*.log"`,
		"fingerprint:\n      strategy: device_and_inode",
		"read_from: beginning",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("host file source = %q, want %q", content, want)
		}
	}
	if strings.Contains(content, "read_from: end") {
		t.Fatalf("host file source = %q, must not skip the beginning of rotated files", content)
	}
}

func TestCollectTaskPathPatterns(t *testing.T) {
	task := &logtailmodel.LogCollectTask{
		PathPattern: "/var/log/messages\n/var/log/secure\n/var/log/messages",
	}

	got := collectTaskPathPatterns(task)
	want := []string{"/var/log/messages", "/var/log/secure"}
	if len(got) != len(want) {
		t.Fatalf("collectTaskPathPatterns() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("collectTaskPathPatterns()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNormalizeCollectTaskPaths(t *testing.T) {
	task := &logtailmodel.LogCollectTask{
		PathPattern: "/var/log/messages\n/var/log/*.log",
	}

	normalizeCollectTaskPaths(task)

	if len(task.PathPatterns) != 2 {
		t.Fatalf("normalizeCollectTaskPaths() pathPatterns len = %d, want %d", len(task.PathPatterns), 2)
	}
	if !strings.Contains(task.PathPatternsJSON, "/var/log/messages") {
		t.Fatalf("normalizeCollectTaskPaths() pathPatternsJson = %q", task.PathPatternsJSON)
	}
	if task.PathPattern != "/var/log/messages\n/var/log/*.log" {
		t.Fatalf("normalizeCollectTaskPaths() pathPattern = %q", task.PathPattern)
	}
}

func TestShouldClearRuntimeLastError(t *testing.T) {
	tests := []struct {
		name         string
		status       string
		configStatus string
		lastError    string
		want         bool
	}{
		{
			name:         "running and applied clears empty last error",
			status:       logtailmodel.VectorStatusRunning,
			configStatus: logtailmodel.LogAgentConfigStatusApplied,
			lastError:    "",
			want:         true,
		},
		{
			name:         "running and applied clears stale reported error",
			status:       logtailmodel.VectorStatusRunning,
			configStatus: logtailmodel.LogAgentConfigStatusApplied,
			lastError:    "boom",
			want:         true,
		},
		{
			name:         "running failed config should not clear",
			status:       logtailmodel.VectorStatusRunning,
			configStatus: logtailmodel.LogAgentConfigStatusFailed,
			lastError:    "boom",
			want:         false,
		},
		{
			name:         "not installed clears empty last error",
			status:       logtailmodel.VectorStatusNotInstalled,
			configStatus: logtailmodel.LogAgentConfigStatusPending,
			lastError:    "",
			want:         true,
		},
		{
			name:         "error status should not clear",
			status:       logtailmodel.VectorStatusError,
			configStatus: logtailmodel.LogAgentConfigStatusFailed,
			lastError:    "",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldClearRuntimeLastError(tt.status, tt.configStatus, tt.lastError)
			if got != tt.want {
				t.Fatalf("shouldClearRuntimeLastError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldPreserveRuntimeConfigFailure(t *testing.T) {
	agent := &logtailmodel.LogAgentVO{
		ConfigStatus:         logtailmodel.LogAgentConfigStatusFailed,
		LastError:            "validate failed",
		DesiredConfigVersion: 2,
		AppliedConfigVersion: 1,
	}

	if !shouldPreserveRuntimeConfigFailure(agent, logtailmodel.LogAgentStatusReportRequest{
		DesiredConfigVersion: 1,
		AppliedConfigVersion: 1,
	}, logtailmodel.LogAgentConfigStatusApplied, "") {
		t.Fatal("should preserve failed config when heartbeat reports old applied config")
	}

	if shouldPreserveRuntimeConfigFailure(agent, logtailmodel.LogAgentStatusReportRequest{
		DesiredConfigVersion: 2,
		AppliedConfigVersion: 2,
	}, logtailmodel.LogAgentConfigStatusApplied, "") {
		t.Fatal("should not preserve failed config after desired version is applied")
	}

	if shouldPreserveRuntimeConfigFailure(agent, logtailmodel.LogAgentStatusReportRequest{
		AppliedConfigVersion: 2,
	}, logtailmodel.LogAgentConfigStatusApplied, "") {
		t.Fatal("should not preserve failed config when applied version catches desired version")
	}

	hashAgent := &logtailmodel.LogAgentVO{
		ConfigStatus:      logtailmodel.LogAgentConfigStatusFailed,
		LastError:         "validate failed",
		DesiredConfigHash: "sha256:abc",
	}
	if shouldPreserveRuntimeConfigFailure(hashAgent, logtailmodel.LogAgentStatusReportRequest{
		AppliedConfigHash: "sha256:abc",
	}, logtailmodel.LogAgentConfigStatusApplied, "") {
		t.Fatal("should not preserve failed config when applied hash matches desired hash")
	}
}

func TestShouldTouchRuntimeLastReload(t *testing.T) {
	agent := &logtailmodel.LogAgentVO{
		VectorStatus:         logtailmodel.VectorStatusRunning,
		ConfigStatus:         logtailmodel.LogAgentConfigStatusApplied,
		AppliedConfigVersion: 2,
		AppliedConfigHash:    "sha256:old",
	}

	if shouldTouchRuntimeLastReload(agent, logtailmodel.VectorStatusRunning, logtailmodel.LogAgentConfigStatusApplied, logtailmodel.LogAgentStatusReportRequest{
		AppliedConfigVersion: 2,
		AppliedConfigHash:    "sha256:old",
	}) {
		t.Fatal("should not touch reload time for unchanged healthy heartbeat")
	}

	if !shouldTouchRuntimeLastReload(agent, logtailmodel.VectorStatusRunning, logtailmodel.LogAgentConfigStatusApplied, logtailmodel.LogAgentStatusReportRequest{
		AppliedConfigVersion: 3,
		AppliedConfigHash:    "sha256:new",
	}) {
		t.Fatal("should touch reload time when applied config changes")
	}

	if !shouldTouchRuntimeLastReload(&logtailmodel.LogAgentVO{
		VectorStatus: logtailmodel.VectorStatusInstalled,
		ConfigStatus: logtailmodel.LogAgentConfigStatusPending,
	}, logtailmodel.VectorStatusRunning, logtailmodel.LogAgentConfigStatusApplied, logtailmodel.LogAgentStatusReportRequest{
		AppliedConfigVersion: 2,
	}) {
		t.Fatal("should touch reload time when state becomes running/applied")
	}
}

func TestShouldTreatRuntimeErrorAsInstalled(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		req       logtailmodel.LogAgentStatusReportRequest
		lastError string
		want      bool
	}{
		{
			name:   "inactive service without config is installed",
			status: logtailmodel.VectorStatusError,
			req: logtailmodel.LogAgentStatusReportRequest{
				VectorVersion: "0.20.0",
				InstallPath:   "/opt/aiops/vector/current",
			},
			lastError: "systemctl status:\nActive: inactive (dead)",
			want:      true,
		},
		{
			name:   "applied config inactive remains error",
			status: logtailmodel.VectorStatusError,
			req: logtailmodel.LogAgentStatusReportRequest{
				VectorVersion:        "0.20.0",
				InstallPath:          "/opt/aiops/vector/current",
				AppliedConfigVersion: 1,
				AppliedConfigHash:    "sha256:abc",
			},
			lastError: "systemctl status:\nActive: inactive (dead)",
			want:      false,
		},
		{
			name:   "binary compatibility error remains error",
			status: logtailmodel.VectorStatusError,
			req: logtailmodel.LogAgentStatusReportRequest{
				InstallPath: "/opt/aiops/vector/current",
			},
			lastError: "vector binary check failed: GLIBC_2.18 not found",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldTreatRuntimeErrorAsInstalled(tt.status, tt.req, tt.lastError)
			if got != tt.want {
				t.Fatalf("shouldTreatRuntimeErrorAsInstalled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClearHealthyLogAgentLastErrorSkipsUnhealthyState(t *testing.T) {
	service := &LogtailService{}
	item := &logtailmodel.LogAgentVO{
		AgentID:      1,
		VectorStatus: logtailmodel.VectorStatusError,
		ConfigStatus: logtailmodel.LogAgentConfigStatusFailed,
		LastError:    "boom",
	}

	if err := service.clearHealthyLogAgentLastError(item); err != nil {
		t.Fatalf("clearHealthyLogAgentLastError() error = %v", err)
	}
	if item.LastError != "boom" {
		t.Fatalf("clearHealthyLogAgentLastError() lastError = %q, want boom", item.LastError)
	}
}

func TestShouldClearLogAgentLastError(t *testing.T) {
	tests := []struct {
		name string
		item *logtailmodel.LogAgentVO
		want bool
	}{
		{
			name: "running applied clears stale last error",
			item: &logtailmodel.LogAgentVO{
				VectorStatus: logtailmodel.VectorStatusRunning,
				ConfigStatus: logtailmodel.LogAgentConfigStatusApplied,
				LastError:    "old validate error",
			},
			want: true,
		},
		{
			name: "running failed config keeps stale last error",
			item: &logtailmodel.LogAgentVO{
				VectorStatus: logtailmodel.VectorStatusRunning,
				ConfigStatus: logtailmodel.LogAgentConfigStatusFailed,
				LastError:    "old validate error",
			},
			want: false,
		},
		{
			name: "empty last error skips database update",
			item: &logtailmodel.LogAgentVO{
				VectorStatus: logtailmodel.VectorStatusRunning,
				ConfigStatus: logtailmodel.LogAgentConfigStatusApplied,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldClearLogAgentLastError(tt.item)
			if got != tt.want {
				t.Fatalf("shouldClearLogAgentLastError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOperationSuccessClearsLastError(t *testing.T) {
	tests := []struct {
		opType string
		want   bool
	}{
		{opType: logtailmodel.LogAgentOperationInstallVector, want: true},
		{opType: logtailmodel.LogAgentOperationReloadVector, want: true},
		{opType: logtailmodel.LogAgentOperationRestartVector, want: true},
		{opType: logtailmodel.LogAgentOperationUpgradeVector, want: true},
		{opType: logtailmodel.LogAgentOperationValidateConfig, want: true},
		{opType: logtailmodel.LogAgentOperationStopVector, want: true},
		{opType: logtailmodel.LogAgentOperationUninstallVector, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.opType, func(t *testing.T) {
			got := operationSuccessClearsLastError(tt.opType)
			if got != tt.want {
				t.Fatalf("operationSuccessClearsLastError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOperationFailureMarksConfigFailed(t *testing.T) {
	tests := []struct {
		opType string
		want   bool
	}{
		{opType: logtailmodel.LogAgentOperationValidateConfig, want: true},
		{opType: logtailmodel.LogAgentOperationReloadVector, want: true},
		{opType: logtailmodel.LogAgentOperationRestartVector, want: true},
		{opType: logtailmodel.LogAgentOperationStopVector, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.opType, func(t *testing.T) {
			got := operationFailureMarksConfigFailed(tt.opType)
			if got != tt.want {
				t.Fatalf("operationFailureMarksConfigFailed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOperationSuccessMarksConfigApplied(t *testing.T) {
	tests := []struct {
		opType string
		want   bool
	}{
		{opType: logtailmodel.LogAgentOperationReloadVector, want: true},
		{opType: logtailmodel.LogAgentOperationRestartVector, want: true},
		{opType: logtailmodel.LogAgentOperationValidateConfig, want: false},
		{opType: logtailmodel.LogAgentOperationInstallVector, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.opType, func(t *testing.T) {
			got := operationSuccessMarksConfigApplied(tt.opType)
			if got != tt.want {
				t.Fatalf("operationSuccessMarksConfigApplied() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOperationCreationMarksConfigPending(t *testing.T) {
	tests := []struct {
		opType string
		want   bool
	}{
		{opType: logtailmodel.LogAgentOperationValidateConfig, want: true},
		{opType: logtailmodel.LogAgentOperationReloadVector, want: true},
		{opType: logtailmodel.LogAgentOperationRestartVector, want: true},
		{opType: logtailmodel.LogAgentOperationInstallVector, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.opType, func(t *testing.T) {
			got := operationCreationMarksConfigPending(tt.opType)
			if got != tt.want {
				t.Fatalf("operationCreationMarksConfigPending() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTerminalOperationStatus(t *testing.T) {
	tests := []struct {
		status string
		want   bool
	}{
		{status: logtailmodel.LogAgentOperationStatusSuccess, want: true},
		{status: logtailmodel.LogAgentOperationStatusFailed, want: true},
		{status: logtailmodel.LogAgentOperationStatusCanceled, want: true},
		{status: logtailmodel.LogAgentOperationStatusPending, want: false},
		{status: logtailmodel.LogAgentOperationStatusRunning, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := isTerminalOperationStatus(tt.status)
			if got != tt.want {
				t.Fatalf("isTerminalOperationStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOperationFailureMessage(t *testing.T) {
	tests := []struct {
		name         string
		errorMsg     string
		progressText string
		want         string
	}{
		{
			name:     "uses reported error first",
			errorMsg: "validate failed",
			want:     "validate failed",
		},
		{
			name:         "falls back to progress text",
			progressText: "安装失败",
			want:         "安装失败",
		},
		{
			name: "uses default message",
			want: "执行失败",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := operationFailureMessage(tt.errorMsg, tt.progressText)
			if got != tt.want {
				t.Fatalf("operationFailureMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}
