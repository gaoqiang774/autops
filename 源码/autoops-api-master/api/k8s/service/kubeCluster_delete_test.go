package service

import (
	"strings"
	"testing"
)

func TestLogCollectTaskReferencesCluster(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		clusterID uint
		want      bool
		wantErr   bool
	}{
		{
			name:      "numeric cluster id",
			raw:       `[{"clusterId":5,"namespace":"default"}]`,
			clusterID: 5,
			want:      true,
		},
		{
			name:      "string cluster id",
			raw:       `[{"clusterId":"5"}]`,
			clusterID: 5,
			want:      true,
		},
		{
			name:      "legacy cluster ID field",
			raw:       `[{"clusterID":5}]`,
			clusterID: 5,
			want:      true,
		},
		{
			name:      "reference in later target",
			raw:       `[{"clusterId":3}, {"clusterId":5}]`,
			clusterID: 5,
			want:      true,
		},
		{
			name:      "different cluster",
			raw:       `[{"clusterId":6}]`,
			clusterID: 5,
			want:      false,
		},
		{
			name:      "host log target",
			raw:       `[{"hostId":5}]`,
			clusterID: 5,
			want:      false,
		},
		{
			name:      "invalid json",
			raw:       `[{"clusterId":5`,
			clusterID: 5,
			wantErr:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := logCollectTaskReferencesCluster(test.raw, test.clusterID)
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, test.wantErr)
			}
			if got != test.want {
				t.Fatalf("referenced = %v, want %v", got, test.want)
			}
		})
	}
}

func TestIsKubernetesLogCollectTask(t *testing.T) {
	tests := []struct {
		name string
		task logCollectTaskClusterReference
		want bool
	}{
		{name: "current collect type", task: logCollectTaskClusterReference{CollectType: "kubernetes"}, want: true},
		{name: "legacy collect type", task: logCollectTaskClusterReference{CollectType: "k8s_workload"}, want: true},
		{name: "k8s source", task: logCollectTaskClusterReference{CollectType: "host", K8SLogSource: "node_file"}, want: true},
		{name: "host task", task: logCollectTaskClusterReference{CollectType: "host"}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isKubernetesLogCollectTask(test.task); got != test.want {
				t.Fatalf("isKubernetesLogCollectTask() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestFormatLogCollectTaskClusterReferenceMessage(t *testing.T) {
	references := []logCollectTaskClusterReference{
		{ID: 1, Name: "task-1"},
		{ID: 2, Name: "task-2"},
		{ID: 3, Name: "task-3"},
		{ID: 4, Name: "task-4"},
		{ID: 5, Name: "task-5"},
		{ID: 6, Name: "task-6"},
	}

	message := formatLogCollectTaskClusterReferenceMessage(references)
	for _, expected := range []string{"禁止删除", "task-1(ID:1)", "另有1个任务"} {
		if !strings.Contains(message, expected) {
			t.Fatalf("message %q does not contain %q", message, expected)
		}
	}
	if strings.Contains(message, "task-6") {
		t.Fatalf("message should limit displayed tasks: %q", message)
	}
}
