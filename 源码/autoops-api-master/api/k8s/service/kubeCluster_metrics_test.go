package service

import (
	"testing"
	"time"

	"dodevops-api/api/k8s/model"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestConvertRawPointsMemoryBytesToGiB(t *testing.T) {
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	points := convertRawPoints([]scraperMetricPoint{
		{Timestamp: now, Value: 3 * 1024 * 1024 * 1024},
	}, 1.0/(1024*1024*1024))

	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	if points[0].Timestamp != now.Format(time.RFC3339) {
		t.Fatalf("unexpected timestamp: %s", points[0].Timestamp)
	}
	if points[0].Value != 3 {
		t.Fatalf("expected 3 GiB, got %v", points[0].Value)
	}
}

func TestRatePointsUsesRawTotals(t *testing.T) {
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	totalBytes := int64(8 * 1024 * 1024 * 1024)
	points := ratePoints([]scraperMetricPoint{
		{Timestamp: now, Value: 2 * 1024 * 1024 * 1024},
	}, totalBytes)

	if len(points) != 1 {
		t.Fatalf("expected 1 point, got %d", len(points))
	}
	if points[0].Value != 25 {
		t.Fatalf("expected 25%%, got %v", points[0].Value)
	}
}

func TestMemoryUsageRateDoesNotDependOnCPUPoints(t *testing.T) {
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	allMemory := sumMetricSeries(map[string][]scraperMetricPoint{
		"node-1": {{Timestamp: now, Value: 2 * 1024 * 1024 * 1024}},
	})
	points := ratePoints(allMemory, 8*1024*1024*1024)

	if len(points) != 1 {
		t.Fatalf("expected memory usage rate point, got %d", len(points))
	}
	if points[0].Value != 25 {
		t.Fatalf("expected 25%% memory usage rate, got %v", points[0].Value)
	}
}

func TestGetPodResourceTotalPrefersRequests(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("250m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			},
		},
	}

	total := getPodResourceTotal(pod)
	if total.CPUMilli != 250 {
		t.Fatalf("expected 250m CPU request, got %d", total.CPUMilli)
	}
	if total.MemoryBytes != 512*1024*1024 {
		t.Fatalf("expected 512Mi memory request, got %d", total.MemoryBytes)
	}
}

func TestGetPodResourceTotalFallsBackToLimits(t *testing.T) {
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
		},
	}

	total := getPodResourceTotal(pod)
	if total.CPUMilli != 500 {
		t.Fatalf("expected 500m CPU limit fallback, got %d", total.CPUMilli)
	}
	if total.MemoryBytes != 1024*1024*1024 {
		t.Fatalf("expected 1Gi memory limit fallback, got %d", total.MemoryBytes)
	}
}

func TestPodResourceTotalFromWorkloadResources(t *testing.T) {
	total := podResourceTotalFromWorkloadResources(modelWorkloadResources("80m", "300Mi", "1", "2Gi"))
	if total.CPUMilli != 80 {
		t.Fatalf("expected 80m CPU request, got %d", total.CPUMilli)
	}
	if total.MemoryBytes != 300*1024*1024 {
		t.Fatalf("expected 300Mi memory request, got %d", total.MemoryBytes)
	}
}

func TestBuildPodMetricHistoryIncludesUsageRates(t *testing.T) {
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	history := buildPodMetricHistory("default", "app-0", podHistorySeries{
		CPU: map[string][]scraperMetricPoint{
			"app-0": {{Timestamp: now, Value: 40}},
		},
		Memory: map[string][]scraperMetricPoint{
			"app-0": {{Timestamp: now, Value: 150 * 1024 * 1024}},
		},
	}, podResourceTotal{CPUMilli: 80, MemoryBytes: 300 * 1024 * 1024})

	if history == nil {
		t.Fatal("expected history")
	}
	if history.CPU[0].Value != 0.04 {
		t.Fatalf("expected 0.04 cores, got %v", history.CPU[0].Value)
	}
	if history.CPUUsageRate[0].Value != 50 {
		t.Fatalf("expected 50%% CPU usage rate, got %v", history.CPUUsageRate[0].Value)
	}
	if history.MemoryUsageRate[0].Value != 50 {
		t.Fatalf("expected 50%% memory usage rate, got %v", history.MemoryUsageRate[0].Value)
	}
}

func modelWorkloadResources(requestCPU, requestMemory, limitCPU, limitMemory string) model.WorkloadResources {
	return model.WorkloadResources{
		Requests: model.ResourceSpec{
			CPU:    requestCPU,
			Memory: requestMemory,
		},
		Limits: model.ResourceSpec{
			CPU:    limitCPU,
			Memory: limitMemory,
		},
	}
}
