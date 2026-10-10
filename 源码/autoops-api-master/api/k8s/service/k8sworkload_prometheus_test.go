package service

import (
	"context"
	"strings"
	"testing"

	"dodevops-api/api/k8s/model"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestBuildPodPrometheusQueriesFallsBackToKubeStateMetricsLimits(t *testing.T) {
	queries := buildPodPrometheusQueries(
		"leyoujia-itest",
		[]string{"zero-ai-im-aaa", "zero-ai-im-bbb"},
		model.PodPrometheusMetricsQuery{},
	)

	cpuQuery := queries["cpuUsageRate"]
	for _, expected := range []string{
		"container_spec_cpu_quota",
		" or ",
		"kube_pod_container_resource_limits",
		`resource=~"^cpu$"`,
		`unit=~"^core$"`,
	} {
		if !strings.Contains(cpuQuery, expected) {
			t.Fatalf("CPU query %q does not contain %q", cpuQuery, expected)
		}
	}

	memoryQuery := queries["memoryUsageRate"]
	for _, expected := range []string{
		"container_spec_memory_limit_bytes",
		" or ",
		"kube_pod_container_resource_limits",
		`resource=~"^memory$"`,
		`unit=~"^byte$"`,
	} {
		if !strings.Contains(memoryQuery, expected) {
			t.Fatalf("memory query %q does not contain %q", memoryQuery, expected)
		}
	}
}

func TestBuildPodPrometheusLimitSelectorKeepsOptionalFilters(t *testing.T) {
	selector := buildPodPrometheusLimitSelector(
		"prod",
		[]string{"api-1"},
		model.PodPrometheusMetricsQuery{
			Container:        "api",
			Node:             "worker-1",
			OriginPrometheus: "cluster-a",
		},
		"cpu",
		"core",
	)

	for _, expected := range []string{
		`namespace=~"^prod$"`,
		`pod=~"^(api-1)$"`,
		`container=~"^api$"`,
		`node=~"^worker-1$"`,
		`origin_prometheus=~"^cluster-a$"`,
	} {
		if !strings.Contains(selector, expected) {
			t.Fatalf("selector %q does not contain %q", selector, expected)
		}
	}
}

func TestPrometheusWorkloadPodNamePattern(t *testing.T) {
	tests := []struct {
		kind string
		name string
		want string
	}{
		{kind: "Deployment", name: "zero-ai-model", want: `zero-ai-model-[a-z0-9]+-[a-z0-9]+`},
		{kind: "StatefulSet", name: "mysql.primary", want: `mysql\.primary-[0-9]+`},
		{kind: "DaemonSet", name: "node-agent", want: `node-agent-[a-z0-9]+`},
		{kind: "Unknown", name: "app", want: ""},
	}

	for _, test := range tests {
		t.Run(test.kind+"/"+test.name, func(t *testing.T) {
			if got := prometheusWorkloadPodNamePattern(test.kind, test.name); got != test.want {
				t.Fatalf("pattern = %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolvePrometheusWorkloadPodNamePatternUsesDeploymentOwner(t *testing.T) {
	controller := true
	clientset := fake.NewSimpleClientset(&appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "zero-ai-model-769f6466f",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				Kind:       "Deployment",
				Name:       "zero-ai-model",
				Controller: &controller,
			}},
		},
	})
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name:      "zero-ai-model-769f6466f-v6xzj",
		Namespace: "default",
		OwnerReferences: []metav1.OwnerReference{{
			Kind:       "ReplicaSet",
			Name:       "zero-ai-model-769f6466f",
			Controller: &controller,
		}},
	}}

	service := &K8sWorkloadServiceImpl{}
	got := service.resolvePrometheusWorkloadPodNamePattern(context.Background(), clientset, "default", pod)
	want := `zero-ai-model-[a-z0-9]+-[a-z0-9]+`
	if got != want {
		t.Fatalf("pattern = %q, want %q", got, want)
	}
}

func TestBuildPodPrometheusQueriesForPodPatternIncludesHistoricalReplicaSets(t *testing.T) {
	queries := buildPodPrometheusQueriesForPodPattern(
		"default",
		prometheusWorkloadPodNamePattern("Deployment", "zero-ai-model"),
		model.PodPrometheusMetricsQuery{},
	)

	for key, query := range queries {
		if !strings.Contains(query, `pod=~"^(zero-ai-model-[a-z0-9]+-[a-z0-9]+)$"`) {
			t.Fatalf("query %s does not use workload pod pattern: %q", key, query)
		}
		if strings.Contains(query, "769f6466f") {
			t.Fatalf("query %s is unexpectedly restricted to the current ReplicaSet: %q", key, query)
		}
	}
}

func TestMergePrometheusChartPodNamesIncludesHistoricalPods(t *testing.T) {
	charts := []model.PodMetricChart{{
		Series: []model.PodMetricSeries{
			{Labels: map[string]string{"pod": "zero-ai-model-new-aaaaa"}},
			{Labels: map[string]string{"pod": "zero-ai-model-old-bbbbb"}},
		},
	}}

	got := mergePrometheusChartPodNames([]string{"zero-ai-model-new-aaaaa"}, charts)
	want := []string{"zero-ai-model-new-aaaaa", "zero-ai-model-old-bbbbb"}
	if len(got) != len(want) {
		t.Fatalf("pod names = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("pod names = %#v, want %#v", got, want)
		}
	}
}
