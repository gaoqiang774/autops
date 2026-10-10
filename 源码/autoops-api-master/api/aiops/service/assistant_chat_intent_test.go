package service

import (
	"strings"
	"testing"

	aiopsmodel "dodevops-api/api/aiops/model"
)

func TestDetectAssistantChatIntentRoutesBatchResourceQuestions(t *testing.T) {
	tests := []struct {
		name     string
		question string
		want     string
	}{
		{
			name:     "mcp tool catalog count",
			question: "你好 检查下当前接入了几个mcp工具",
			want:     aiopsmodel.AssistantChatIntentMCPCatalog,
		},
		{
			name:     "host inventory count",
			question: "你好 我当前有多少主机",
			want:     aiopsmodel.AssistantChatIntentHostInventory,
		},
		{
			name:     "reclaim low load test machines",
			question: "帮我找出长期低负载、可以回收的测试机器",
			want:     aiopsmodel.AssistantChatIntentResourceGovernance,
		},
		{
			name:     "rank machines by memory pressure",
			question: "那些机器的内存压力比较大",
			want:     aiopsmodel.AssistantChatIntentResourceGovernance,
		},
		{
			name:     "rank current system hosts by memory usage",
			question: "帮忙看看我的系统中当前内存使用率前五的主机有哪些",
			want:     aiopsmodel.AssistantChatIntentResourceGovernance,
		},
		{
			name:     "rank current hosts by numeric top memory usage",
			question: "查看当前内存使用率前5的主机",
			want:     aiopsmodel.AssistantChatIntentResourceGovernance,
		},
		{
			name:     "host capacity expansion remains resource governance",
			question: "帮我评估哪些主机适合做容量扩容",
			want:     aiopsmodel.AssistantChatIntentResourceGovernance,
		},
		{
			name:     "specific host pressure remains host intent",
			question: "帮我看一下 172.16.7.138 这台机器最近半小时有没有压力",
			want:     aiopsmodel.AssistantChatIntentHostPressure,
		},
		{
			name:     "specific host abnormal remains host intent",
			question: "帮我看一下 172.16.3.244 这台机器是不是有压力，最近半小时有没有异常？",
			want:     aiopsmodel.AssistantChatIntentHostPressure,
		},
		{
			name:     "specific host services",
			question: "172.16.3.238 这个机器上面部署了什么服务",
			want:     aiopsmodel.AssistantChatIntentHostServices,
		},
		{
			name:     "host pressure without target asks for clarification later",
			question: "帮我看下压力",
			want:     aiopsmodel.AssistantChatIntentHostPressure,
		},
		{
			name:     "alert overview",
			question: "现在有哪些告警",
			want:     aiopsmodel.AssistantChatIntentAlertOverview,
		},
		{
			name:     "platform overall risk",
			question: "帮我看下当前平台整体风险",
			want:     aiopsmodel.AssistantChatIntentPlatformOverview,
		},
		{
			name:     "platform abnormal focus",
			question: "当前有哪些异常需要关注",
			want:     aiopsmodel.AssistantChatIntentPlatformOverview,
		},
		{
			name:     "platform health",
			question: "现在系统健康吗",
			want:     aiopsmodel.AssistantChatIntentPlatformOverview,
		},
		{
			name:     "database overview",
			question: "现在数据库资产状态怎么样",
			want:     aiopsmodel.AssistantChatIntentDatabaseOverview,
		},
		{
			name:     "specific database pressure",
			question: "mysql-100.10 这个数据库是否 有压力",
			want:     aiopsmodel.AssistantChatIntentDatabaseOverview,
		},
		{
			name:     "specific redis pressure",
			question: "192.168.34.107-redis 这个redis当前是否有压力",
			want:     aiopsmodel.AssistantChatIntentDatabaseOverview,
		},
		{
			name:     "k8s cluster overview",
			question: "当前 k8s 集群健康情况怎么样",
			want:     aiopsmodel.AssistantChatIntentK8sOverview,
		},
		{
			name:     "k8s rollout restart",
			question: "帮我重启下 K8S 名为 zero-ai-data-cli 的工作负载",
			want:     aiopsmodel.AssistantChatIntentK8sRolloutRestart,
		},
		{
			name:     "k8s scale workload",
			question: "帮我在 K8S 名为 zero-ai-data-cli 的工作负载扩容到2个副本",
			want:     aiopsmodel.AssistantChatIntentK8sScaleWorkload,
		},
		{
			name:     "k8s relative scale workload",
			question: "在K8S集群中找到这个服务zero-ai-data-cli 工作负载，并增加一个副本数",
			want:     aiopsmodel.AssistantChatIntentK8sScaleWorkload,
		},
		{
			name:     "k8s rollback workload by revision",
			question: "zero-ai-kos 工作负载 帮我回滚到版本号为 14 这个版本",
			want:     aiopsmodel.AssistantChatIntentK8sRollbackWorkload,
		},
		{
			name:     "k8s rollback workload by image version",
			question: "zero-ai-kos工作负载 帮我回滚到 镜像版本为v2026-0805-165529 的版本",
			want:     aiopsmodel.AssistantChatIntentK8sRollbackWorkload,
		},
		{
			name:     "k8s workload replica count query",
			question: "在K8S集群中找到这个服务zero-ai-bcl 工作负载 ,查看下 现在几个副本数",
			want:     aiopsmodel.AssistantChatIntentK8sOverview,
		},
		{
			name:     "k8s workload history query",
			question: "查看 zero-ai-data-cli 工作负载的历史版本",
			want:     aiopsmodel.AssistantChatIntentK8sOverview,
		},
		{
			name:     "k8s workload root cause diagnosis",
			question: "分析下 zero-ai-kos 工作负载异常的原因",
			want:     aiopsmodel.AssistantChatIntentK8sOverview,
		},
		{
			name:     "knowledge search",
			question: "知识库里有没有 nginx 502 的处理方案",
			want:     aiopsmodel.AssistantChatIntentKnowledgeSearch,
		},
		{
			name:     "knowledge search without explicit knowledge base",
			question: "有没有 nginx 502 的处理方案",
			want:     aiopsmodel.AssistantChatIntentKnowledgeSearch,
		},
		{
			name:     "skill catalog",
			question: "查看一下当前支持哪些技能",
			want:     aiopsmodel.AssistantChatIntentSkillCatalog,
		},
		{
			name:     "linux inspection script",
			question: "帮我写一个巡检 linux 的脚本呢",
			want:     aiopsmodel.AssistantChatIntentOpsGeneral,
		},
		{
			name:     "linux inspection script without spaces",
			question: "帮我写一个巡检linux系统的脚本",
			want:     aiopsmodel.AssistantChatIntentOpsGeneral,
		},
		{
			name:     "install nginx on linux",
			question: "如何在linux机器上安装一个nginx服务",
			want:     aiopsmodel.AssistantChatIntentOpsGeneral,
		},
		{
			name:     "install mysql remains ops general",
			question: "帮我写一个安装 mysql 的脚本",
			want:     aiopsmodel.AssistantChatIntentOpsGeneral,
		},
		{
			name:     "general knowledge question",
			question: "中国四大名著是什么",
			want:     aiopsmodel.AssistantChatIntentGeneralQA,
		},
		{
			name:     "career skills question",
			question: "运维开发 需要具备那些技能",
			want:     aiopsmodel.AssistantChatIntentGeneralQA,
		},
		{
			name:     "career skills question with supported word",
			question: "运维平台开发需要支持哪些技能",
			want:     aiopsmodel.AssistantChatIntentGeneralQA,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &aiopsmodel.AssistantChatRequest{Question: tt.question}
			entities := detectAssistantChatEntities(req)
			if got := detectAssistantChatIntent(req, entities); got != tt.want {
				t.Fatalf("detectAssistantChatIntent() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectAssistantChatEntitiesExtractsK8sScaleTarget(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{Question: "帮我在 K8S 名为 zero-ai-data-cli 的工作负载扩容到2个副本"}
	entities := detectAssistantChatEntities(req)
	if got := firstNonEmptyAssistantChatString(entities, "workloadName"); got != "zero-ai-data-cli" {
		t.Fatalf("workloadName = %q, want zero-ai-data-cli", got)
	}
	if got := int(assistantChatNumber(entities["replicas"])); got != 2 {
		t.Fatalf("replicas = %d, want 2", got)
	}
	if got := detectAssistantChatIntent(req, entities); got != aiopsmodel.AssistantChatIntentK8sScaleWorkload {
		t.Fatalf("intent = %q, want k8s_scale_workload", got)
	}
}

func TestDetectAssistantChatEntitiesExtractsRelativeK8sScaleTarget(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{Question: "在K8S集群中找到这个服务zero-ai-data-cli 工作负载，并增加一个副本数"}
	entities := detectAssistantChatEntities(req)
	if got := firstNonEmptyAssistantChatString(entities, "workloadName"); got != "zero-ai-data-cli" {
		t.Fatalf("workloadName = %q, want zero-ai-data-cli", got)
	}
	if _, ok := entities["replicas"]; ok {
		t.Fatalf("relative scale must not be parsed as an absolute replica target: %#v", entities)
	}
	if got := int(assistantChatNumber(entities["replicaDelta"])); got != 1 {
		t.Fatalf("replicaDelta = %d, want 1", got)
	}
	if got := detectAssistantChatIntent(req, entities); got != aiopsmodel.AssistantChatIntentK8sScaleWorkload {
		t.Fatalf("intent = %q, want k8s_scale_workload", got)
	}
	if questions := buildAssistantChatClarifyQuestions(aiopsmodel.AssistantChatIntentK8sScaleWorkload, entities); len(questions) != 0 {
		t.Fatalf("relative scale unexpectedly requires clarification: %#v", questions)
	}
}

func TestDetectAssistantChatEntitiesExtractsWorkloadBeforeObjectPhrase(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{Question: "zero-ai-bcl 对这个工作负载扩容增加一个副本数"}
	entities := detectAssistantChatEntities(req)
	if got := firstNonEmptyAssistantChatString(entities, "workloadName"); got != "zero-ai-bcl" {
		t.Fatalf("workloadName = %q, want zero-ai-bcl", got)
	}
	if got := int(assistantChatNumber(entities["replicaDelta"])); got != 1 {
		t.Fatalf("replicaDelta = %d, want 1", got)
	}
	if got := detectAssistantChatIntent(req, entities); got != aiopsmodel.AssistantChatIntentK8sScaleWorkload {
		t.Fatalf("intent = %q, want k8s_scale_workload", got)
	}
}

func TestDetectAssistantChatEntitiesExtractsWorkloadBeforePoliteObjectPhrase(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{Question: "zero-ai-data-cli 帮我对这个工作负载扩容增加一个副本"}
	entities := detectAssistantChatEntities(req)
	if got := firstNonEmptyAssistantChatString(entities, "workloadName"); got != "zero-ai-data-cli" {
		t.Fatalf("workloadName = %q, want zero-ai-data-cli", got)
	}
	if got := int(assistantChatNumber(entities["replicaDelta"])); got != 1 {
		t.Fatalf("replicaDelta = %d, want 1", got)
	}
	if got := detectAssistantChatIntent(req, entities); got != aiopsmodel.AssistantChatIntentK8sScaleWorkload {
		t.Fatalf("intent = %q, want k8s_scale_workload", got)
	}
	if questions := buildAssistantChatClarifyQuestions(aiopsmodel.AssistantChatIntentK8sScaleWorkload, entities); len(questions) != 0 {
		t.Fatalf("complete relative scale request unexpectedly requires clarification: %#v", questions)
	}
}

func TestDetectAssistantChatEntitiesExtractsK8sRollbackTargets(t *testing.T) {
	tests := []struct {
		question     string
		revision     int64
		imageVersion string
	}{
		{question: "zero-ai-kos 工作负载 帮我回滚到版本号为 14 这个版本", revision: 14},
		{question: "zero-ai-kos工作负载 帮我回滚到 镜像版本为v2026-0805-165529 的版本", imageVersion: "v2026-0805-165529"},
	}
	for _, tt := range tests {
		entities := detectAssistantChatEntities(&aiopsmodel.AssistantChatRequest{Question: tt.question})
		if got := firstNonEmptyAssistantChatString(entities, "workloadName"); got != "zero-ai-kos" {
			t.Fatalf("workloadName = %q, want zero-ai-kos; entities=%#v", got, entities)
		}
		if got := int64(assistantChatNumber(entities["revision"])); got != tt.revision {
			t.Fatalf("revision = %d, want %d; entities=%#v", got, tt.revision, entities)
		}
		if got := firstNonEmptyAssistantChatString(entities, "imageVersion"); got != tt.imageVersion {
			t.Fatalf("imageVersion = %q, want %q; entities=%#v", got, tt.imageVersion, entities)
		}
		if got := detectAssistantChatIntent(&aiopsmodel.AssistantChatRequest{Question: tt.question}, entities); got != aiopsmodel.AssistantChatIntentK8sRollbackWorkload {
			t.Fatalf("intent = %q, want k8s_rollback_workload", got)
		}
	}
}

func TestExplicitK8sRollbackVersionDoesNotInheritPreviousTargetVersion(t *testing.T) {
	tests := []struct {
		question string
		memory   map[string]interface{}
		key      string
		want     interface{}
		absent   string
	}{
		{
			question: "zero-ai-kos 工作负载 帮我回滚到版本号为 14 这个版本",
			memory:   map[string]interface{}{"revision": float64(15), "imageVersion": "v-old"},
			key:      "revision", want: int64(14), absent: "imageVersion",
		},
		{
			question: "zero-ai-kos工作负载 帮我回滚到 镜像版本为v2026-0805-165529 的版本",
			memory:   map[string]interface{}{"revision": float64(15), "imageVersion": "v-old"},
			key:      "imageVersion", want: "v2026-0805-165529", absent: "revision",
		},
	}
	for _, tt := range tests {
		entities := detectAssistantChatEntities(&aiopsmodel.AssistantChatRequest{Question: tt.question})
		mergeAssistantChatMemoryEntities(entities, &aiopsmodel.AssistantChatMemoryContext{Entities: tt.memory}, tt.question)
		switch want := tt.want.(type) {
		case int64:
			if got := int64(assistantChatNumber(entities[tt.key])); got != want {
				t.Fatalf("%s = %d, want %d; entities=%#v", tt.key, got, want, entities)
			}
		case string:
			if got := firstNonEmptyAssistantChatString(entities, tt.key); got != want {
				t.Fatalf("%s = %q, want %q; entities=%#v", tt.key, got, want, entities)
			}
		}
		if _, exists := entities[tt.absent]; exists {
			t.Fatalf("explicit rollback target unexpectedly inherited %s: %#v", tt.absent, entities)
		}
	}
}

func TestDetectAssistantChatEntitiesExtractsReplicaReductionAfterNoun(t *testing.T) {
	question := "zero-ai-data-cli 这个工作负载 副本缩容减少一个"
	entities := detectAssistantChatEntities(&aiopsmodel.AssistantChatRequest{Question: question})
	if got := firstNonEmptyAssistantChatString(entities, "workloadName"); got != "zero-ai-data-cli" {
		t.Fatalf("workloadName = %q, want zero-ai-data-cli", got)
	}
	if got := int(assistantChatNumber(entities["replicaDelta"])); got != -1 {
		t.Fatalf("replicaDelta = %d, want -1", got)
	}
	if got := detectAssistantChatIntent(&aiopsmodel.AssistantChatRequest{Question: question}, entities); got != aiopsmodel.AssistantChatIntentK8sScaleWorkload {
		t.Fatalf("intent = %q, want k8s_scale_workload", got)
	}
}

func TestExplicitReplicaReductionDoesNotInheritPreviousIncrease(t *testing.T) {
	question := "zero-ai-data-cli 这个工作负载 副本缩容减少一个"
	entities := detectAssistantChatEntities(&aiopsmodel.AssistantChatRequest{Question: question})
	memory := &aiopsmodel.AssistantChatMemoryContext{Entities: map[string]interface{}{
		"replicas":     float64(3),
		"replicaDelta": float64(1),
	}}
	mergeAssistantChatMemoryEntities(entities, memory, question)
	if got := int(assistantChatNumber(entities["replicaDelta"])); got != -1 {
		t.Fatalf("replicaDelta = %d, want current question delta -1", got)
	}
	if _, exists := entities["replicas"]; exists {
		t.Fatalf("explicit relative reduction unexpectedly inherited absolute replicas: %#v", entities)
	}
}

func TestExplicitWorkloadDoesNotInheritPreviousK8sTarget(t *testing.T) {
	question := "zero-ai-bcl 对这个工作负载扩容增加一个副本数"
	entities := detectAssistantChatEntities(&aiopsmodel.AssistantChatRequest{Question: question})
	memory := &aiopsmodel.AssistantChatMemoryContext{Entities: map[string]interface{}{
		"workloadName": "zero-ai-data-cli",
		"clusterId":    float64(2),
		"clusterName":  "itest-测试环境",
		"namespace":    "leyoujia-itest",
	}}
	mergeAssistantChatMemoryEntities(entities, memory, question)
	if got := firstNonEmptyAssistantChatString(entities, "workloadName"); got != "zero-ai-bcl" {
		t.Fatalf("workloadName = %q, want current question target zero-ai-bcl", got)
	}
	for _, key := range []string{"clusterId", "clusterName", "namespace"} {
		if _, exists := entities[key]; exists {
			t.Fatalf("explicit workload unexpectedly inherited %s from memory: %#v", key, entities)
		}
	}
}

func TestK8sReplicaCountQueryIsReadonly(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{Question: "在K8S集群中找到这个服务zero-ai-bcl 工作负载 ,查看下 现在几个副本数"}
	entities := detectAssistantChatEntities(req)
	if got := firstNonEmptyAssistantChatString(entities, "workloadName"); got != "zero-ai-bcl" {
		t.Fatalf("workloadName = %q, want zero-ai-bcl", got)
	}
	if got := detectAssistantChatIntent(req, entities); got != aiopsmodel.AssistantChatIntentK8sOverview {
		t.Fatalf("intent = %q, want k8s_overview", got)
	}
	if !assistantChatQuestionAsksK8sReplicaCount(req.Question) {
		t.Fatal("replica count query was not recognized")
	}
	if questions := buildAssistantChatClarifyQuestions(aiopsmodel.AssistantChatIntentK8sOverview, entities); len(questions) != 0 {
		t.Fatalf("readonly replica query requested a target replica count: %#v", questions)
	}
}

func TestRelativeK8sScalePhraseStillRoutesToScale(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{Question: "在K8S集群中找到这个服务zero-ai-data-cli 工作负载，并增加一个副本数"}
	entities := detectAssistantChatEntities(req)
	if got := detectAssistantChatIntent(req, entities); got != aiopsmodel.AssistantChatIntentK8sScaleWorkload {
		t.Fatalf("intent = %q, want k8s_scale_workload", got)
	}
	if got := int(assistantChatNumber(entities["replicaDelta"])); got != 1 {
		t.Fatalf("replicaDelta = %d, want 1", got)
	}
}

func TestBuildAssistantK8sReplicaCountAnswer(t *testing.T) {
	answer := buildAssistantK8sReplicaCountAnswer(assistantK8sWorkloadTarget{
		ClusterName: "itest-测试环境", Namespace: "leyoujia-itest", WorkloadType: "deployment", WorkloadName: "zero-ai-bcl",
	}, map[string]interface{}{
		"desiredReplicas": float64(2), "currentReplicas": float64(2), "readyReplicas": float64(1), "availableReplicas": float64(1),
	})
	for _, want := range []string{"zero-ai-bcl", "期望副本 2", "当前副本 2", "Ready 1", "Available 1"} {
		if !strings.Contains(answer, want) {
			t.Fatalf("answer does not contain %q: %s", want, answer)
		}
	}
}

func TestK8sWorkloadHistoryQueryExtractsTarget(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{Question: "查看 zero-ai-data-cli 工作负载的历史版本"}
	entities := detectAssistantChatEntities(req)
	if got := firstNonEmptyAssistantChatString(entities, "workloadName"); got != "zero-ai-data-cli" {
		t.Fatalf("workloadName = %q, want zero-ai-data-cli", got)
	}
	if !assistantChatBool(entities["workloadHistory"]) {
		t.Fatal("workload history query was not recognized")
	}
	if got := detectAssistantChatIntent(req, entities); got != aiopsmodel.AssistantChatIntentK8sOverview {
		t.Fatalf("intent = %q, want k8s_overview", got)
	}
	plan := buildAssistantChatK8sToolPlan(entities)
	if len(plan) != 3 || plan[2].ToolName != "k8s_get_workload_history" {
		t.Fatalf("unexpected history tool plan: %#v", plan)
	}
}

func TestK8sWorkloadDiagnosisExtractsTargetAndPlan(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{Question: "分析下 zero-ai-kos 工作负载异常的原因"}
	entities := detectAssistantChatEntities(req)
	if got := firstNonEmptyAssistantChatString(entities, "workloadName"); got != "zero-ai-kos" {
		t.Fatalf("workloadName = %q, want zero-ai-kos", got)
	}
	if !assistantChatBool(entities["workloadDiagnosis"]) {
		t.Fatal("workload diagnosis query was not recognized")
	}
	plan := buildAssistantChatK8sToolPlan(entities)
	if len(plan) != 3 || plan[2].ToolName != "k8s_diagnose_workload" {
		t.Fatalf("unexpected diagnosis tool plan: %#v", plan)
	}
}

func TestK8sWorkloadHealthCheckExtractsTargetAndPlan(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{Question: "帮忙看看 hsl-main 这个工作负载 服务当前是否正常"}
	entities := detectAssistantChatEntities(req)
	if got := firstNonEmptyAssistantChatString(entities, "workloadName"); got != "hsl-main" {
		t.Fatalf("workloadName = %q, want hsl-main", got)
	}
	if !assistantChatBool(entities["workloadDiagnosis"]) {
		t.Fatal("workload health check was not recognized as diagnosis")
	}
	if got := detectAssistantChatIntent(req, entities); got != aiopsmodel.AssistantChatIntentK8sOverview {
		t.Fatalf("intent = %q, want k8s_overview", got)
	}
	plan := buildAssistantChatK8sToolPlan(entities)
	if len(plan) != 3 || plan[1].ToolName != "k8s_search_resources" || plan[2].ToolName != "k8s_diagnose_workload" {
		t.Fatalf("unexpected health diagnosis tool plan: %#v", plan)
	}
}

func TestK8sWorkloadStatusCheckExtractsTargetAndStatusPlan(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{Question: "检查下 zero-ai-data-cli   这个工作负载的状态"}
	entities := detectAssistantChatEntities(req)
	if got := firstNonEmptyAssistantChatString(entities, "workloadName"); got != "zero-ai-data-cli" {
		t.Fatalf("workloadName = %q, want zero-ai-data-cli", got)
	}
	if assistantChatBool(entities["workloadDiagnosis"]) {
		t.Fatal("plain workload status check was incorrectly recognized as root-cause diagnosis")
	}
	if !assistantChatBool(entities["workloadStatusQuery"]) {
		t.Fatal("workload status check was not recognized as a status query")
	}
	if got := detectAssistantChatIntent(req, entities); got != aiopsmodel.AssistantChatIntentK8sOverview {
		t.Fatalf("intent = %q, want k8s_overview", got)
	}
	plan := buildAssistantChatK8sToolPlan(entities)
	if len(plan) != 3 || plan[1].ToolName != "k8s_search_resources" || plan[2].ToolName != "k8s_get_workload_status" {
		t.Fatalf("unexpected workload status tool plan: %#v", plan)
	}
	result := &aiopsmodel.AssistantChatResult{Intent: aiopsmodel.AssistantChatIntentK8sOverview, Entities: entities}
	if shouldAssistantChatUseLLM(result) {
		t.Fatal("deterministic workload status answer unexpectedly enabled LLM summarization")
	}
}

func TestBuildAssistantK8sWorkloadStatusAnswerUsesCurrentK8sStateOnly(t *testing.T) {
	status := map[string]interface{}{
		"status":            "Running",
		"desiredReplicas":   float64(1),
		"currentReplicas":   float64(1),
		"readyReplicas":     float64(1),
		"availableReplicas": float64(1),
		"images":            []interface{}{"registry/zero-ai-data-cli:v27"},
		"pods": []interface{}{map[string]interface{}{
			"name": "zero-ai-data-cli-cb88d9f46-g56gk", "phase": "Running", "restartCount": float64(0), "nodeName": "node16-2",
		}},
		"events": []interface{}{},
	}
	answer := buildAssistantK8sWorkloadStatusAnswer(assistantK8sWorkloadTarget{
		ClusterName: "test", Namespace: "leyoujia-itest", WorkloadType: "deployment", WorkloadName: "zero-ai-data-cli",
	}, status)
	for _, want := range []string{"当前状态：正常", "期望 1，当前 1，Ready 1，可用 1", "Running，重启 0 次", "当前详情未返回 Warning 事件"} {
		if !strings.Contains(answer, want) {
			t.Fatalf("answer does not contain %q: %s", want, answer)
		}
	}
	if !assistantK8sWorkloadStatusHealthy(status) {
		t.Fatal("fully ready workload was not classified as healthy")
	}
}

func TestBuildAssistantK8sWorkloadHistoryAnswer(t *testing.T) {
	history := buildAssistantK8sWorkloadHistory(assistantK8sWorkloadTarget{
		ClusterName: "itest-测试环境", Namespace: "leyoujia-itest", WorkloadType: "deployment", WorkloadName: "zero-ai-data-cli",
	}, map[string]interface{}{
		"currentRevision": float64(27),
		"revisions": []interface{}{
			map[string]interface{}{
				"revision": float64(27), "status": "current", "images": []interface{}{"registry/zero-ai-data-cli:v27"},
				"creationTime": "2026-08-12T18:55:35+08:00", "replicasSummary": map[string]interface{}{"desired": float64(1), "current": float64(1), "ready": float64(1)},
			},
			map[string]interface{}{
				"revision": float64(26), "status": "historical", "images": []interface{}{"registry/zero-ai-data-cli:v26"},
				"creationTime": "2026-08-12T17:28:37+08:00", "replicasSummary": map[string]interface{}{"desired": float64(0), "current": float64(0), "ready": float64(0)},
			},
		},
	}, 10)
	answer := buildAssistantK8sWorkloadHistoryAnswer(history)
	for _, want := range []string{"最近 2 个历史版本", "| 时间 | 版本号 | 镜像版本 | 状态 |", "| 2026-08-12 18:55:35 | 27 | v27 | 当前版本 |", "| 2026-08-12 17:28:37 | 26 | v26 | 历史版本 |"} {
		if !strings.Contains(answer, want) {
			t.Fatalf("answer does not contain %q: %s", want, answer)
		}
	}
	if history.Total != 2 || len(history.Items) != 2 || history.Items[0].Revision != 27 || history.Items[0].ImageVersion != "v27" {
		t.Fatalf("unexpected structured history: %#v", history)
	}
	actions := assistantK8sWorkloadHistoryActions(history)
	if len(actions) != 1 || actions[0] != "回滚到 revision 26" {
		t.Fatalf("unexpected rollback actions: %#v", actions)
	}
}

func TestRelativeK8sScaleDoesNotRestoreAbsoluteReplicasFromMemory(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{Question: "这个工作负载增加一个副本"}
	entities := detectAssistantChatEntities(req)
	memory := &aiopsmodel.AssistantChatMemoryContext{Entities: map[string]interface{}{
		"clusterId": uint(7), "namespace": "leyoujia-itest", "workloadName": "zero-ai-data-cli", "replicas": 5,
	}}
	mergeAssistantChatMemoryEntities(entities, memory, req.Question)
	if _, ok := entities["replicas"]; ok {
		t.Fatalf("relative scale restored stale absolute replicas: %#v", entities)
	}
	if got := int(assistantChatNumber(entities["replicaDelta"])); got != 1 {
		t.Fatalf("replicaDelta = %d, want 1", got)
	}
	if got := firstNonEmptyAssistantChatString(entities, "workloadName"); got != "zero-ai-data-cli" {
		t.Fatalf("workloadName = %q, want remembered target", got)
	}
}

func TestRelativeK8sScaleDeltaSurvivesTargetSelectionFollowup(t *testing.T) {
	entities := map[string]interface{}{"pageType": "global_chat", "clusterName": "itest", "namespace": "leyoujia-itest"}
	memory := &aiopsmodel.AssistantChatMemoryContext{Entities: map[string]interface{}{
		"workloadName": "zero-ai-data-cli", "replicaDelta": 1,
	}}
	mergeAssistantChatMemoryEntities(entities, memory, "选择 itest 这个工作负载")
	if got := int(assistantChatNumber(entities["replicaDelta"])); got != 1 {
		t.Fatalf("replicaDelta = %d, want remembered delta 1", got)
	}
	if got := firstNonEmptyAssistantChatString(entities, "workloadName"); got != "zero-ai-data-cli" {
		t.Fatalf("workloadName = %q, want remembered target", got)
	}
}

func TestAssistantK8sDesiredReplicas(t *testing.T) {
	for _, tt := range []struct {
		name   string
		output map[string]interface{}
		want   int
		ok     bool
	}{
		{name: "desired field", output: map[string]interface{}{"desiredReplicas": float64(1)}, want: 1, ok: true},
		{name: "zero desired", output: map[string]interface{}{"desiredReplicas": float64(0)}, want: 0, ok: true},
		{name: "replicas fallback", output: map[string]interface{}{"replicas": float64(2)}, want: 2, ok: true},
		{name: "missing", output: map[string]interface{}{"readyReplicas": float64(1)}, ok: false},
		{name: "invalid type", output: map[string]interface{}{"desiredReplicas": "unknown"}, ok: false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := assistantK8sDesiredReplicas(tt.output)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("assistantK8sDesiredReplicas() = (%d, %v), want (%d, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestApplyAssistantChatActionMode(t *testing.T) {
	preview := &aiopsmodel.AssistantChatResult{
		Readonly: true,
		PendingAction: map[string]interface{}{
			"requiresConfirmation": true,
			"confirmationId":       "signed-preview",
		},
	}
	applyAssistantChatActionMode(preview)
	if preview.Readonly {
		t.Fatal("confirmable action preview remained readonly")
	}

	selection := &aiopsmodel.AssistantChatResult{
		Readonly: true,
		PendingAction: map[string]interface{}{
			"requiresSelection": true,
		},
	}
	applyAssistantChatActionMode(selection)
	if !selection.Readonly {
		t.Fatal("target selection without confirmation credentials became writable")
	}
}

func TestBuildAssistantChatClarifyQuestionsForMissingHost(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{Question: "帮我看下压力"}
	entities := detectAssistantChatEntities(req)
	intent := detectAssistantChatIntent(req, entities)
	questions := buildAssistantChatClarifyQuestions(intent, entities)
	if len(questions) == 0 {
		t.Fatalf("expected clarify question for missing host")
	}
	if questions[0].Field != "host" {
		t.Fatalf("clarify field = %q, want host", questions[0].Field)
	}
}

func TestK8sRolloutRestartOverridesHostPageContextAndDiscoversK8sTarget(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{
		Question: "帮我重启下 K8S 名为 zero-ai-data-cli 的工作负载",
		Context: map[string]interface{}{
			"pageType": "host_detail", "hostId": uint(41), "ip": "172.16.7.138", "hostName": "lyj-zhangfan",
		},
	}
	entities := detectAssistantChatEntities(req)
	normalizeAssistantChatDomainEntities(req.Question, entities)
	if got := detectAssistantChatIntent(req, entities); got != aiopsmodel.AssistantChatIntentK8sRolloutRestart {
		t.Fatalf("intent = %q, want k8s_rollout_restart", got)
	}
	if _, ok := entities["hostId"]; ok {
		t.Fatal("K8s action retained stale hostId")
	}
	if got := firstNonEmptyAssistantChatString(entities, "workloadName"); got != "zero-ai-data-cli" {
		t.Fatalf("workloadName = %q, want zero-ai-data-cli", got)
	}
	questions := buildAssistantChatClarifyQuestions(aiopsmodel.AssistantChatIntentK8sRolloutRestart, entities)
	if len(questions) != 0 {
		t.Fatalf("clarify questions = %#v, want automatic K8s target discovery", questions)
	}
}

func TestK8sScaleOverridesHostPageContextAndExtractsReplicas(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{
		Question: "帮我在 K8S 名为 zero-ai-data-cli 的工作负载扩容到2个副本",
		Context: map[string]interface{}{
			"pageType": "host_detail", "hostId": uint(41), "ip": "172.16.7.138", "hostName": "lyj-zhangfan",
		},
	}
	entities := detectAssistantChatEntities(req)
	normalizeAssistantChatDomainEntities(req.Question, entities)
	if got := detectAssistantChatIntent(req, entities); got != aiopsmodel.AssistantChatIntentK8sScaleWorkload {
		t.Fatalf("intent = %q, want k8s_scale_workload", got)
	}
	if _, ok := entities["hostId"]; ok {
		t.Fatal("K8s scale action retained stale hostId")
	}
	if got := int(assistantChatNumber(entities["replicas"])); got != 2 {
		t.Fatalf("replicas = %d, want 2", got)
	}
	questions := buildAssistantChatClarifyQuestions(aiopsmodel.AssistantChatIntentK8sScaleWorkload, entities)
	if len(questions) != 0 {
		t.Fatalf("clarify questions = %#v, want automatic K8s target discovery", questions)
	}
}

func TestK8sIntentOverridesStaleHostContext(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{Question: "检查下当前k8s集群有没有问题"}
	entities := map[string]interface{}{"hostId": uint(41), "ip": "172.16.3.244", "pageType": "global_chat"}
	normalizeAssistantChatDomainEntities(req.Question, entities)
	if _, ok := entities["hostId"]; ok {
		t.Fatal("stale hostId was not removed from explicit K8s question")
	}
	if got := detectAssistantChatIntent(req, entities); got != aiopsmodel.AssistantChatIntentK8sOverview {
		t.Fatalf("intent = %q, want k8s_overview", got)
	}
}

func TestK8sQuestionDoesNotRestoreHostMemory(t *testing.T) {
	entities := map[string]interface{}{}
	memory := &aiopsmodel.AssistantChatMemoryContext{Entities: map[string]interface{}{
		"hostId": uint(41), "ip": "172.16.3.244", "clusterId": uint(7),
	}}
	mergeAssistantChatMemoryEntities(entities, memory, "继续检查这个k8s集群有没有问题")
	if _, ok := entities["hostId"]; ok {
		t.Fatal("K8s question restored hostId from memory")
	}
	if got := uint(assistantChatNumber(entities["clusterId"])); got != 7 {
		t.Fatalf("clusterId = %d, want 7", got)
	}
}

func TestDetectAssistantChatEntitiesExtractsK8sResources(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{Question: "排查 deployment order-api 在 namespace prod 的异常"}
	entities := detectAssistantChatEntities(req)
	if got := firstNonEmptyAssistantChatString(entities, "workloadType"); got != "deployment" {
		t.Fatalf("workloadType = %q, want deployment", got)
	}
	if got := firstNonEmptyAssistantChatString(entities, "workloadName"); got != "order-api" {
		t.Fatalf("workloadName = %q, want order-api", got)
	}
	if got := firstNonEmptyAssistantChatString(entities, "namespace"); got != "prod" {
		t.Fatalf("namespace = %q, want prod", got)
	}
}

func TestDetectAssistantChatEntitiesExtractsChineseClusterName(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{Question: "检查集群 prod-cluster 是否正常"}
	entities := detectAssistantChatEntities(req)
	if got := firstNonEmptyAssistantChatString(entities, "clusterName"); got != "prod-cluster" {
		t.Fatalf("clusterName = %q, want prod-cluster", got)
	}
}

func TestBuildAssistantK8sRuntimeToolPlanForClusterHealth(t *testing.T) {
	steps := buildAssistantK8sRuntimeToolPlan("检查当前集群是否有问题", map[string]interface{}{}, 7)
	want := map[string]bool{
		"k8s_check_cluster": false, "k8s_list_nodes": false,
		"k8s_list_namespaces": false,
	}
	for _, step := range steps {
		if _, ok := want[step.ToolName]; ok {
			want[step.ToolName] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("cluster health plan omitted %s: %#v", name, steps)
		}
	}
	for _, step := range steps {
		if step.ToolName == "k8s_query_events" {
			t.Fatal("cluster health plan queried cross-Namespace events without an authorized namespace")
		}
	}
}

func TestBuildAssistantK8sRuntimeToolPlanForPodDiagnosis(t *testing.T) {
	steps := buildAssistantK8sRuntimeToolPlan("排查这个 Pod 的异常日志", map[string]interface{}{
		"namespace": "prod", "podName": "api-123", "container": "app",
	}, 7)
	want := map[string]bool{
		"k8s_get_pod_status": false, "k8s_query_events": false,
		"k8s_query_metrics": false, "k8s_query_logs": false, "k8s_diagnose_pod": false,
	}
	for _, step := range steps {
		if _, ok := want[step.ToolName]; ok {
			want[step.ToolName] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("Pod diagnosis plan omitted %s: %#v", name, steps)
		}
	}
}

func TestSelectAssistantK8sClusterIDByQuestionName(t *testing.T) {
	items := []interface{}{
		map[string]interface{}{"id": float64(7), "name": "prod-cluster"},
		map[string]interface{}{"id": float64(8), "name": "test-cluster"},
	}
	if got := selectAssistantK8sClusterID("检查 prod-cluster 是否正常", items); got != 7 {
		t.Fatalf("selected clusterId = %d, want 7", got)
	}
}

func TestSelectAssistantK8sClusterTargetsReturnsAllAuthorizedClusters(t *testing.T) {
	items := []interface{}{
		map[string]interface{}{"id": float64(7), "name": "dev-cluster"},
		map[string]interface{}{"id": float64(8), "name": "test-cluster"},
	}
	targets := selectAssistantK8sClusterTargets(0, items)
	if len(targets) != 2 {
		t.Fatalf("target count = %d, want 2: %#v", len(targets), targets)
	}
	if targets[0].ID != 7 || targets[1].ID != 8 {
		t.Fatalf("unexpected targets: %#v", targets)
	}
}

func TestBroadK8sHealthQuestionIgnoresStaleResourceEntity(t *testing.T) {
	entities := map[string]interface{}{"podName": "old-api-123", "namespace": "old-prod", "pageType": "global_chat"}
	if assistantChatHasSpecificK8sResource("检查下当前 k8s 集群有没有问题", entities) {
		t.Fatal("broad fleet health question was narrowed by a stale Pod entity")
	}
	if !assistantChatHasSpecificK8sResource("继续检查这个 Pod", entities) {
		t.Fatal("explicit Pod continuation should keep the remembered Pod target")
	}
}

func TestAssistantK8sFleetAnswerIncludesAbnormalPodsAndResourceHotspots(t *testing.T) {
	scan := assistantK8sClusterScan{ClusterID: 7, ClusterName: "dev-cluster", LiveChecks: 6, AuthorizedNS: 1, ScannedNS: 1}
	absorbAssistantK8sPods(&scan, map[string]interface{}{
		"total": float64(3), "totalPages": float64(1),
		"workloads": []interface{}{
			map[string]interface{}{"name": "api-1", "namespace": "prod", "status": "Running", "nodeName": "node-1"},
			map[string]interface{}{"name": "worker-1", "namespace": "prod", "status": "Pending", "nodeName": "node-2"},
			map[string]interface{}{"name": "cache-1", "namespace": "prod", "status": "Running", "nodeName": "node-2", "restartCount": float64(8)},
		},
	}, true)
	if ok := absorbAssistantK8sMetrics(&scan, map[string]interface{}{
		"podCount": float64(2),
		"podMetrics": []interface{}{
			map[string]interface{}{
				"podName": "api-1", "namespace": "prod",
				"usage":     map[string]interface{}{"cpu": "850m", "memory": "768Mi"},
				"usageRate": map[string]interface{}{"cpuRate": float64(85), "memoryRate": float64(75)},
			},
		},
	}); !ok {
		t.Fatal("expected metrics to be available")
	}
	finalizeAssistantK8sClusterScan(&scan)
	answer := buildAssistantChatK8sFleetAnswer(assistantK8sScanReport{
		RequestedClusters: 1, ScannedClusters: 1, Clusters: []assistantK8sClusterScan{scan},
	})
	for _, want := range []string{"异常/高重启 Pod 2 个", "prod/worker-1(Pending)", "prod/cache-1(Running，重启 8 次)", "Pod 资源热点", "dev-cluster/prod/api-1 850m"} {
		if !strings.Contains(answer, want) {
			t.Fatalf("fleet answer does not contain %q:\n%s", want, answer)
		}
	}
}

func TestK8sClusterEventQueryDoesNotRequireNamespace(t *testing.T) {
	err := ValidateMCPArguments(mcpReadToolSchemas["k8s_query_events"], map[string]interface{}{"clusterId": uint(7), "limit": 100})
	if err != nil {
		t.Fatalf("cluster-scoped event query was rejected: %v", err)
	}
}

func TestAggregateAssistantK8sUsedToolsCountsInvocationsByTool(t *testing.T) {
	items := []aiopsmodel.AssistantChatUsedTool{
		{Name: "k8s_search_resources", Arguments: map[string]interface{}{"namespace": "prod"}, DurationMS: 10, Success: true},
		{Name: "k8s_search_resources", Arguments: map[string]interface{}{"namespace": "test"}, DurationMS: 20, Success: true},
		{Name: "k8s_query_metrics", Arguments: map[string]interface{}{"namespace": "prod"}, DurationMS: 30, Success: true},
	}
	aggregated := aggregateAssistantK8sUsedTools(items)
	if len(aggregated) != 2 {
		t.Fatalf("aggregated tool count = %d, want 2: %#v", len(aggregated), aggregated)
	}
	if aggregated[0].Name != "k8s_search_resources" || aggregated[0].CallCount != 2 || aggregated[0].DurationMS != 30 {
		t.Fatalf("unexpected resource aggregation: %#v", aggregated[0])
	}
	if aggregated[0].Arguments != nil {
		t.Fatalf("repeated tool arguments should be omitted from summary: %#v", aggregated[0].Arguments)
	}
	if aggregated[1].CallCount != 1 {
		t.Fatalf("single metrics invocation count = %d, want 1", aggregated[1].CallCount)
	}
}

func TestAggregateAssistantK8sTimelineKeepsOneEntryPerTool(t *testing.T) {
	items := []aiopsmodel.AssistantChatTimelineEvent{
		{ToolName: "k8s_query_metrics", Status: aiopsmodel.AssistantChatPlanSucceeded, Message: "查询指标", DurationMS: 10},
		{ToolName: "k8s_query_metrics", Status: aiopsmodel.AssistantChatPlanSucceeded, Message: "查询指标", DurationMS: 20},
	}
	aggregated := aggregateAssistantK8sTimeline(items)
	if len(aggregated) != 1 || aggregated[0].DurationMS != 30 || !strings.Contains(aggregated[0].Message, "调用 2 次") {
		t.Fatalf("unexpected timeline aggregation: %#v", aggregated)
	}
}

func TestEnsureAssistantK8sToolPlanInsertsBeforeSummary(t *testing.T) {
	result := &aiopsmodel.AssistantChatResult{
		Intent: aiopsmodel.AssistantChatIntentK8sOverview,
		ToolPlan: withAssistantChatLifecycleSteps(aiopsmodel.AssistantChatIntentK8sOverview, []aiopsmodel.AssistantChatToolPlanStep{
			{ToolName: "k8s_list_clusters", Purpose: "list"},
		}),
	}
	ensureAssistantChatToolPlanStep(result, "k8s_check_cluster", "check", map[string]interface{}{"clusterId": 7})
	if len(result.ToolPlan) != 4 {
		t.Fatalf("tool plan length = %d, want 4", len(result.ToolPlan))
	}
	if result.ToolPlan[2].ToolName != "k8s_check_cluster" || result.ToolPlan[3].ToolName != "ai_summary" {
		t.Fatalf("runtime tool was not inserted before summary: %#v", result.ToolPlan)
	}
}

func TestDetectAssistantChatEntitiesExplicitIPOverridesContextHost(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{
		Question: "172.16.3.238 这个机器上面部署了什么服务",
		Context: map[string]interface{}{
			"pageType": "host",
			"hostId":   41,
			"host": map[string]interface{}{
				"id":        41,
				"privateIp": "172.16.3.244",
				"hostName":  "idc-fang-redis-3.244",
			},
		},
	}
	entities := detectAssistantChatEntities(req)
	if got := firstNonEmptyAssistantChatString(entities, "ip"); got != "172.16.3.238" {
		t.Fatalf("ip = %q, want explicit question ip", got)
	}
	if got := assistantChatNumber(entities["hostId"]); got != 0 {
		t.Fatalf("hostId = %v, want stale context hostId cleared", got)
	}
	if got := detectAssistantChatIntent(req, entities); got != aiopsmodel.AssistantChatIntentHostServices {
		t.Fatalf("intent = %q, want host_services", got)
	}
}

func TestFinalizeAssistantChatProgressCompletesOpsGeneralPlan(t *testing.T) {
	entities := map[string]interface{}{"pageType": "global_chat"}
	result := &aiopsmodel.AssistantChatResult{
		Intent:   aiopsmodel.AssistantChatIntentOpsGeneral,
		Question: "如何排查 nginx 502",
		Entities: entities,
		ToolPlan: buildAssistantChatToolPlan(aiopsmodel.AssistantChatIntentOpsGeneral, entities),
	}
	result.AssistantPlan = buildAssistantInvestigationPlan(result.Intent, result.Question, result.ToolPlan, result.Entities)

	startAssistantChatPlanStep(result, "understand_requirement")
	finishAssistantChatPlanStep(result, "understand_requirement", "")
	startAssistantChatPlanStep(result, assistantChatLLMStepToolName(result.Intent))
	finishAssistantChatPlanStep(result, assistantChatLLMStepToolName(result.Intent), "")

	if result.Progress == nil || result.Progress.Percent != 66 {
		t.Fatalf("precondition failed: progress percent = %+v, want 66", result.Progress)
	}

	finalizeAssistantChatProgress(result, aiopsmodel.AgentRunStatusSucceeded)

	if result.Progress == nil {
		t.Fatalf("progress is nil")
	}
	if result.Progress.Percent != 100 {
		t.Fatalf("progress percent = %d, want 100", result.Progress.Percent)
	}
	if result.Progress.Status != aiopsmodel.AssistantChatPlanSucceeded {
		t.Fatalf("progress status = %q, want %q", result.Progress.Status, aiopsmodel.AssistantChatPlanSucceeded)
	}
	for _, step := range result.Progress.Steps {
		if step.Status == aiopsmodel.AssistantChatPlanPending || step.Status == aiopsmodel.AssistantChatPlanRunning {
			t.Fatalf("step %s status = %q, want finalized", step.ToolName, step.Status)
		}
	}
}

func TestBuildAssistantChatOpsGeneralFallbackLinuxReadonlyScript(t *testing.T) {
	answer := buildAssistantChatOpsGeneralFallbackAnswer("帮我写一个 Linux 只读巡检脚本")
	for _, want := range []string{"Linux 只读巡检脚本", "#!/usr/bin/env bash", "df -hT", "systemctl --failed"} {
		if !strings.Contains(answer, want) {
			t.Fatalf("fallback answer does not contain %q:\n%s", want, answer)
		}
	}
}

func TestBuildAssistantChatOpsGeneralFallbackCPUHighRunbook(t *testing.T) {
	answer := buildAssistantChatOpsGeneralFallbackAnswer("系统CPU使用率比较高 怎么排查这个问题")
	for _, want := range []string{"CPU 使用率比较高", "ps aux --sort=-%cpu", "iowait", "不要直接 kill"} {
		if !strings.Contains(answer, want) {
			t.Fatalf("fallback answer does not contain %q:\n%s", want, answer)
		}
	}
}

func TestBuildAssistantChatOpsGeneralFallbackDefaultStillAnswers(t *testing.T) {
	answer := buildAssistantChatOpsGeneralFallbackAnswer("服务偶发变慢怎么排查")
	for _, want := range []string{"通用排查框架", "看监控趋势", "查日志"} {
		if !strings.Contains(answer, want) {
			t.Fatalf("fallback answer does not contain %q:\n%s", want, answer)
		}
	}
}

func TestAssistantChatAgentUnavailableAllowsLegacyOnlineAgentPayload(t *testing.T) {
	if assistantChatAgentUnavailable(map[string]interface{}{
		"statusText": "运行中",
		"online":     true,
	}) {
		t.Fatalf("online agent without installed field should be treated as available")
	}
	if !assistantChatAgentUnavailable(map[string]interface{}{
		"statusText": "未安装或未登记",
		"installed":  false,
		"online":     false,
	}) {
		t.Fatalf("explicitly uninstalled agent should be treated as unavailable")
	}
}

func TestDetectAssistantChatEntitiesExtractsDatabaseName(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{Question: "mysql-100.10 这个数据库是否 有压力"}
	entities := detectAssistantChatEntities(req)
	if got := firstNonEmptyAssistantChatString(entities, "databaseName"); got != "mysql-100.10" {
		t.Fatalf("databaseName = %q, want mysql-100.10", got)
	}
	if got := firstNonEmptyAssistantChatString(entities, "keyword"); got != "mysql-100.10" {
		t.Fatalf("keyword = %q, want mysql-100.10", got)
	}
}

func TestDetectAssistantChatEntitiesExtractsIPPrefixedRedisName(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{Question: "192.168.34.107-redis 这个redis当前是否有压力"}
	entities := detectAssistantChatEntities(req)
	if got := firstNonEmptyAssistantChatString(entities, "databaseName"); got != "192.168.34.107-redis" {
		t.Fatalf("databaseName = %q, want 192.168.34.107-redis", got)
	}
	if got := firstNonEmptyAssistantChatString(entities, "ip"); got != "" {
		t.Fatalf("ip = %q, want empty because database name wins", got)
	}
}

func TestDetectAssistantChatIntentDatabaseContextReference(t *testing.T) {
	req := &aiopsmodel.AssistantChatRequest{Question: "查询这个实例"}
	entities := map[string]interface{}{"databaseName": "192.168.34.107-redis"}
	if got := detectAssistantChatIntent(req, entities); got != aiopsmodel.AssistantChatIntentDatabaseOverview {
		t.Fatalf("intent = %q, want database_overview", got)
	}
}

func TestAssistantChatQuestionAsksDatabasePressure(t *testing.T) {
	if !assistantChatQuestionAsksDatabasePressure("mysql-100.10 这个mysql 实例 是否有压力") {
		t.Fatalf("expected database pressure question")
	}
	if assistantChatQuestionAsksDatabasePressure("当前有哪些数据库实例") {
		t.Fatalf("database list question should not be treated as pressure")
	}
}

func TestBuildAssistantChatDatabasePressureAnswerExplainsBlockers(t *testing.T) {
	answer := buildAssistantChatDatabasePressureAnswer(map[string]interface{}{
		"matched": 1,
		"items": []interface{}{
			map[string]interface{}{
				"id":         10,
				"name":       "mysql-100.10",
				"type":       "MySQL",
				"status":     "运行中",
				"authStatus": "未认证",
				"endpoint":   "192.168.100.10:3306",
			},
		},
	})
	for _, want := range []string{"暂时不能可靠判断", "数据库资产未认证", "Threads_running", "QPS/TPS"} {
		if !strings.Contains(answer, want) {
			t.Fatalf("answer does not contain %q:\n%s", want, answer)
		}
	}
}
