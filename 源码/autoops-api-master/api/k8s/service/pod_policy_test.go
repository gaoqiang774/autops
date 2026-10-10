package service

import "testing"

func TestIsProtectedPodOperationTarget(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		podName   string
		want      bool
	}{
		{name: "system namespace", namespace: "kube-system", podName: "custom-agent", want: true},
		{name: "system prefix", namespace: "default", podName: "coredns-custom", want: true},
		{name: "application pod", namespace: "production", podName: "orders-api-123", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isProtectedPodOperationTarget(tt.namespace, tt.podName); got != tt.want {
				t.Fatalf("isProtectedPodOperationTarget(%q, %q) = %v, want %v", tt.namespace, tt.podName, got, tt.want)
			}
		})
	}
}

func TestWorkloadListCacheKeysIncludeNamespaceAndClusterViews(t *testing.T) {
	keys := workloadListCacheKeys(7, "production")
	wantKeys := []string{
		workloadsListCacheKey(7, "production", "pods"),
		workloadsListCacheKey(7, "production", "all"),
		workloadsListCacheKey(7, "", "pods"),
		workloadsListCacheKey(7, "", "deployments"),
	}

	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		keySet[key] = struct{}{}
	}
	for _, key := range wantKeys {
		if _, exists := keySet[key]; !exists {
			t.Fatalf("workloadListCacheKeys() missing %q", key)
		}
	}
}
