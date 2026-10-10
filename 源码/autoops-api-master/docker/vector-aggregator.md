# docker安装  aiops-vector-aggregator
# 后端接口也会生成同款命令:
# GET /api/v1/logtail/vector/aggregator/config-preview
# 返回字段:
# - content: vector.yaml
# - docker.command: Docker 一键安装命令
# - kubernetes.yaml: K8s 安装 YAML
# 关键成功信号是：
# Healthcheck passed.
# Vector has started.
# version="0.56.0"
# 现在链路应该是： Host Vector -> Kafka -> Docker Vector Aggregator -> VictoriaLogs
docker rm -f aiops-vector-aggregator 2>/dev/null || true

cat > /opt/aiops/vector-aggregator/vector.yaml <<'EOF'
data_dir: /var/lib/aiops-vector-aggregator

sources:
  kafka_logs:
    type: kafka
    bootstrap_servers: "172.16.7.137:9092"
    group_id: "aiops-victorialogs-writer"
    topics:
      - "aiops-logs"
    auto_offset_reset: earliest
    decoding:
      codec: json

sinks:
  victorialogs:
    type: http
    inputs:
      - kafka_logs
    uri: "http://172.16.7.137:9428/insert/jsonline?_stream_fields=project_code,store_code,env,host_name,collect_task_code&_msg_field=_msg&_time_field=_time"
    method: post
    encoding:
      codec: json
    framing:
      method: newline_delimited
    compression: gzip
    batch:
      max_events: 1000
      max_bytes: 1048576
      timeout_secs: 1
    request:
      timeout_secs: 60
    buffer:
      type: disk
      max_size: 2147483648
      when_full: block
EOF

docker run -d \
  --name aiops-vector-aggregator \
  --restart unless-stopped \
  --network host \
  --security-opt seccomp=unconfined \
  --pids-limit -1 \
  --ulimit nproc=65535:65535 \
  -v /opt/aiops/vector-aggregator/vector.yaml:/etc/vector/vector.yaml:ro \
  -v /var/lib/aiops-vector-aggregator:/var/lib/aiops-vector-aggregator \
  crpi-aj3vgoxp9kzh2jx4.cn-hangzhou.personal.cr.aliyuncs.com/zhangfan_k8s/vector:latest-debian \
  --config /etc/vector/vector.yaml
