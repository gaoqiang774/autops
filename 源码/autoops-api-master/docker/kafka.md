


docker rm -f kafka-broker

docker run -d --name kafka-broker \
  -p 9092:9092 \
  -e KAFKA_NODE_ID=1 \
  -e KAFKA_PROCESS_ROLES=broker,controller \
  -e KAFKA_LISTENERS=PLAINTEXT://:9092,CONTROLLER://:9093 \
  -e KAFKA_ADVERTISED_LISTENERS=PLAINTEXT://172.16.7.137:9092 \
  -e KAFKA_CONTROLLER_LISTENER_NAMES=CONTROLLER \
  -e KAFKA_LISTENER_SECURITY_PROTOCOL_MAP=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT \
  -e KAFKA_CONTROLLER_QUORUM_VOTERS=1@localhost:9093 \
  -e KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR=1 \
  -e KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR=1 \
  -e KAFKA_TRANSACTION_STATE_LOG_MIN_ISR=1 \
  -e KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS=0 \
  docker.1ms.run/apache/kafka:4.0.0


  docker exec -it kafka-broker /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:9092 \
  --create --if-not-exists \
  --topic aiops-logs \
  --partitions 6 \
  --replication-factor 1

  # 再验证主机 Vector 是否真的 reload 到新配置：
 # grep -nE 'type: kafka|bootstrap_servers|topic:' /opt/aiops/vector/current/vector.yaml
# journalctl -u aiops-vector -n 100 --no-pager | egrep -i 'kafka|rdkafka|error|timeout|refused'
# 注意 read_from: end 只读新日志，所以要产生一条新的 nginx 访问日志，再看 Kafka：
# docker exec -it kafka-broker /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic aiops-logs \
  --from-beginning