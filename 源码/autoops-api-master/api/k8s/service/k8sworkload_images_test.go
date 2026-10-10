package service

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestExtractImages(t *testing.T) {
	service := &K8sWorkloadServiceImpl{}
	podSpec := &corev1.PodSpec{
		InitContainers: []corev1.Container{
			{Name: "init-db", Image: "registry.example.com/init-db:v1"},
			{Name: "init-empty", Image: "  "},
		},
		Containers: []corev1.Container{
			{Name: "app", Image: "registry.example.com/app:v2"},
			{Name: "sidecar", Image: "registry.example.com/sidecar:v3"},
			{Name: "duplicate", Image: "registry.example.com/app:v2"},
		},
		EphemeralContainers: []corev1.EphemeralContainer{
			{EphemeralContainerCommon: corev1.EphemeralContainerCommon{
				Name:  "debugger",
				Image: "registry.example.com/debug:v4",
			}},
		},
	}

	want := []string{
		"registry.example.com/init-db:v1",
		"registry.example.com/app:v2",
		"registry.example.com/sidecar:v3",
		"registry.example.com/debug:v4",
	}

	if got := service.extractImages(podSpec); !reflect.DeepEqual(got, want) {
		t.Fatalf("extractImages() = %#v, want %#v", got, want)
	}
}

func TestExtractImagesWithNilPodSpec(t *testing.T) {
	service := &K8sWorkloadServiceImpl{}

	if got := service.extractImages(nil); len(got) != 0 {
		t.Fatalf("extractImages(nil) = %#v, want empty image list", got)
	}
}

func TestExtractImageContainers(t *testing.T) {
	service := &K8sWorkloadServiceImpl{}
	podSpec := &corev1.PodSpec{
		InitContainers: []corev1.Container{{Name: "init", Image: "init:v1"}},
		Containers:     []corev1.Container{{Name: "app", Image: "app:v2"}},
		EphemeralContainers: []corev1.EphemeralContainer{{
			EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "debug", Image: "debug:v3"},
		}},
	}

	got := service.extractImageContainers(podSpec)
	if len(got) != 3 {
		t.Fatalf("extractImageContainers() returned %d containers, want 3", len(got))
	}
	if got[0].Name != "init" || got[0].Image != "init:v1" ||
		got[1].Name != "app" || got[1].Image != "app:v2" ||
		got[2].Name != "debug" || got[2].Image != "debug:v3" {
		t.Fatalf("extractImageContainers() = %#v", got)
	}
}
