package service

import (
	"strings"
	"testing"

	commonconfig "dodevops-api/common/config"
)

func TestPreviewVectorAggregatorConfigIncludesInstallArtifacts(t *testing.T) {
	oldConfig := commonconfig.Config
	defer func() { commonconfig.Config = oldConfig }()

	configPath := writeTempLogtailConfig(t, `
logtail:
  delivery_mode: "auto"
  victorialogs:
    url: "http://victorialogs:9428"
  kafka:
    enabled: true
    brokers:
      - "172.16.7.137:9092"
    topic: "aiops-logs"
    host_aliases:
      - ip: "192.168.4.210"
        hostnames:
          - "kafka-node2-4-210"
  vector:
    image: "timberio/vector:0.20.0-debian"
    disk_buffer_max_size_bytes: 654321
  aggregator:
    image: "docker.1ms.run/timberio/vector:latest-debian"
`)
	if err := commonconfig.LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	got, err := (&LogtailService{}).PreviewVectorAggregatorConfig()
	if err != nil {
		t.Fatalf("PreviewVectorAggregatorConfig() error = %v", err)
	}
	if got.Topic != "aiops-logs" {
		t.Fatalf("Topic = %q, want aiops-logs", got.Topic)
	}
	if got.ConsumerGroup != vectorAggregatorConsumerGroup {
		t.Fatalf("ConsumerGroup = %q, want %q", got.ConsumerGroup, vectorAggregatorConsumerGroup)
	}
	if got.ConfigHash == "" {
		t.Fatal("ConfigHash is empty")
	}
	if got.ConfigPath != vectorAggregatorConfigPath {
		t.Fatalf("ConfigPath = %q, want %q", got.ConfigPath, vectorAggregatorConfigPath)
	}
	if got.Image != "docker.1ms.run/timberio/vector:latest-debian" {
		t.Fatalf("Image = %q, want aggregator image", got.Image)
	}

	for _, want := range []string{
		"data_dir: /var/lib/aiops-vector-aggregator",
		"type: kafka",
		`bootstrap_servers: "172.16.7.137:9092"`,
		`group_id: "aiops-victorialogs-writer"`,
		`- "aiops-logs"`,
		"auto_offset_reset: earliest",
		"decoding:",
		"codec: json",
		"type: http",
		`uri: "http://victorialogs:9428/insert/jsonline?_stream_fields=project_code,store_code,env,host_name,collect_task_name,collect_task_code,log_source,k8s_log_source,k8s_cluster_name,k8s_namespace,k8s_workload_name,k8s_pod_name,k8s_container_name,log_file_name&_msg_field=_msg&_time_field=_time"`,
		"framing:",
		"method: newline_delimited",
		"batch:",
		"request:",
		"buffer:",
		"max_size: 654321",
		"when_full: block",
		"compression: gzip",
	} {
		if !strings.Contains(got.Content, want) {
			t.Fatalf("aggregator config = %q, want %q", got.Content, want)
		}
	}

	for _, want := range []string{
		"docker pull 'docker.1ms.run/timberio/vector:latest-debian'",
		"docker rm -f 'aiops-vector-aggregator'",
		"--security-opt seccomp=unconfined",
		"-v /opt/aiops/vector-aggregator/vector.yaml:/etc/vector/vector.yaml:ro",
		"docker.1ms.run/timberio/vector:latest-debian",
	} {
		if !strings.Contains(got.Docker.Command, want) {
			t.Fatalf("docker command = %q, want %q", got.Docker.Command, want)
		}
	}
	if got.Docker.ContainerName != vectorAggregatorName || got.Docker.ConfigPath != vectorAggregatorConfigPath {
		t.Fatalf("docker preview = %+v, want stable names", got.Docker)
	}

	for _, want := range []string{
		"kind: ConfigMap",
		"kind: Deployment",
		"name: aiops-vector-aggregator",
		"namespace: aiops-vector",
		"seccompProfile:",
		"type: Unconfined",
		"hostAliases:",
		`ip: "192.168.4.210"`,
		`- "kafka-node2-4-210"`,
		`image: "docker.1ms.run/timberio/vector:latest-debian"`,
		"vector.yaml: |",
		"bootstrap_servers: \"172.16.7.137:9092\"",
	} {
		if !strings.Contains(got.Kubernetes.YAML, want) {
			t.Fatalf("kubernetes yaml = %q, want %q", got.Kubernetes.YAML, want)
		}
	}
}

func TestRenderVectorAggregatorConfigUsesLegacyHTTPConfig(t *testing.T) {
	oldConfig := commonconfig.Config
	defer func() { commonconfig.Config = oldConfig }()

	configPath := writeTempLogtailConfig(t, `
logtail:
  victorialogs:
    url: "http://victorialogs:9428"
  vector:
    disk_buffer_max_size_bytes: 654321
`)
	if err := commonconfig.LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	got := renderVectorAggregatorConfig(vectorKafkaSinkOptions{
		Brokers: []string{"172.16.7.137:9092"},
		Topic:   "aiops-logs",
	}, true)
	for _, want := range []string{
		"codec: ndjson",
		"batch:",
		"request:",
		"compression: gzip",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("aggregator config = %q, want %q", got, want)
		}
	}
	for _, notWant := range []string{
		"framing:",
		"method: newline_delimited",
		"buffer:",
		"max_size: 654321",
		"when_full: block",
	} {
		if strings.Contains(got, notWant) {
			t.Fatalf("aggregator config = %q, should not contain %q for legacy Vector", got, notWant)
		}
	}
}

func TestConfiguredKafkaHostAliasesAffectVectorConfigHash(t *testing.T) {
	oldConfig := commonconfig.Config
	defer func() { commonconfig.Config = oldConfig }()

	configPath := writeTempLogtailConfig(t, `
logtail:
  kafka:
    enabled: true
    brokers:
      - "192.168.2.23:9092"
    host_aliases:
      - ip: "192.168.2.23"
        hostnames:
          - "kafka-node1-2-23"
`)
	if err := commonconfig.LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	aliases := configuredKafkaHostAliasModels()
	if len(aliases) != 1 || aliases[0].IP != "192.168.2.23" || len(aliases[0].Hostnames) != 1 || aliases[0].Hostnames[0] != "kafka-node1-2-23" {
		t.Fatalf("configuredKafkaHostAliasModels() = %+v, want kafka-node1 mapping", aliases)
	}
	withAliases := vectorConfigHash("data_dir: /var/lib/aiops-vector\n", aliases)
	withoutAliases := vectorConfigHash("data_dir: /var/lib/aiops-vector\n", nil)
	if withAliases == withoutAliases {
		t.Fatalf("vectorConfigHash() should change when host aliases change: %s", withAliases)
	}
}

func TestPreviewVectorAggregatorConfigFailsWithoutKafka(t *testing.T) {
	oldConfig := commonconfig.Config
	defer func() { commonconfig.Config = oldConfig }()

	configPath := writeTempLogtailConfig(t, `
logtail:
  delivery_mode: "auto"
  kafka:
    enabled: false
`)
	if err := commonconfig.LoadConfig(configPath); err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if _, err := (&LogtailService{}).PreviewVectorAggregatorConfig(); err == nil {
		t.Fatal("PreviewVectorAggregatorConfig() error = nil, want kafka config error")
	}
}
