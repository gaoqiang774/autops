---
name: prometheus
description: 用于通过 mcp-server-aiops 的 query_prometheus 工具执行 PromQL 查询、分析 Prometheus 指标、排查告警、服务可用性、资源使用率、延迟、错误率和容量趋势。
---

# Prometheus

当用户需要查询 Prometheus 指标、分析告警、定位服务异常、查看资源趋势、计算错误率或验证 SLO/SLA 时，使用本 skill。

本 skill 负责把用户意图转换成合适的 PromQL 和查询窗口；真正的数据获取只通过 MCP tool `query_prometheus` 完成。

## 可用 MCP Tool

只使用 `mcp-server-aiops` 暴露的这个 tool：

- `query_prometheus`: 对配置好的 Prometheus 执行 PromQL instant query 或 range query。

参数：

- `query`: 必填，PromQL 表达式。
- `time`: 可选，instant query 的评估时间。
- `start`: 可选，range query 开始时间。
- `end`: 可选，range query 结束时间。
- `step`: 可选，range query 步长，例如 `30s`、`1m`、`5m`。
- `timeout_seconds`: 可选，本次请求超时。

返回结构固定为：

- 成功：`{"ok": true, "data": {"endpoint": "...", "query": "...", "result_type": "...", "result": [...], "warnings": ..., "infos": ..., "duration_ms": ...}}`
- 失败：`{"ok": false, "error": {"type": "...", "message": "..."}}`

使用结果前必须先检查 `ok`。

## 查询选择

- 查“当前状态”“现在是否正常”“某个指标当前值”：使用 instant query，只传 `query`，必要时传 `time`。
- 查“过去一段时间趋势”“最近 30 分钟”“昨天到今天”“峰值/波动”：使用 range query，同时传 `start`、`end`、`step`。
- 用户给出相对时间时，把时间窗口转成明确的绝对时间后再查询；如果当前时间不可确定，先说明假设。
- 用户没有给时间窗口时：
  - 告警排查默认看最近 30 分钟。
  - 趋势分析默认看最近 6 小时。
  - 容量趋势默认看最近 24 小时。
- `step` 选择要和窗口匹配：
  - 小于等于 1 小时：`30s` 或 `1m`。
  - 1 到 12 小时：`5m`。
  - 12 到 48 小时：`15m`。
  - 更长窗口：`1h`。

## PromQL 编写原则

- 优先用用户给出的真实 label，例如 `job`、`instance`、`namespace`、`pod`、`service`、`cluster`。
- 不知道 label 名称时，不要臆造精确查询；先用更宽的查询验证指标是否存在，或请用户提供指标/label。
- 计数器使用 `rate()` 或 `increase()`，不要直接比较 counter 原始值。
- 错误率优先写成“错误请求速率 / 总请求速率”。
- 延迟分位数优先使用 `histogram_quantile()` 和 `_bucket` 指标。
- CPU、内存、磁盘、网络指标需要区分主机、容器、Pod、服务维度。
- 查询结果为空时，不要直接判断系统正常；说明可能是指标名、label、时间窗口或采集配置不匹配。

## 常用查询模式

服务可用性：

```promql
up{job="SERVICE"}
```

服务错误率：

```promql
sum(rate(http_requests_total{service="SERVICE",status=~"5.."}[5m]))
/
sum(rate(http_requests_total{service="SERVICE"}[5m]))
```

P95 延迟：

```promql
histogram_quantile(
  0.95,
  sum(rate(http_request_duration_seconds_bucket{service="SERVICE"}[5m])) by (le)
)
```

Pod CPU 使用率：

```promql
sum(rate(container_cpu_usage_seconds_total{pod=~"POD_REGEX",container!=""}[5m])) by (pod)
```

Pod 内存使用：

```promql
sum(container_memory_working_set_bytes{pod=~"POD_REGEX",container!=""}) by (pod)
```

节点磁盘可用率：

```promql
node_filesystem_avail_bytes{mountpoint="/",fstype!~"tmpfs|overlay"}
/
node_filesystem_size_bytes{mountpoint="/",fstype!~"tmpfs|overlay"}
```

这些只是模板。调用 tool 前要把 `SERVICE`、`POD_REGEX` 等占位符替换成用户提供的真实目标；如果目标未知，先询问。

## 工作流

1. 明确目标：指标、服务、实例、命名空间、时间窗口、聚合维度和用户想回答的问题。
2. 选择查询类型：当前值用 instant query，趋势和对比用 range query。
3. 写出 PromQL，并在调用前检查：
   - 是否存在未替换的占位符。
   - counter 是否用了 `rate()` 或 `increase()`。
   - range selector 是否合理，例如 `[5m]`、`[10m]`。
   - 聚合维度是否会丢失用户关心的对象。
4. 调用 `query_prometheus`。
5. 如果失败：
   - 报告 `error.type` 和 `error.message`。
   - 给出一个更小、更可验证的下一步查询。
6. 如果成功：
   - 根据 `result_type` 解读 `vector`、`matrix` 或 `scalar`。
   - 对数值补单位和含义，例如秒、字节、比例、每秒速率。
   - 明确结果是否为空、是否有 `warnings` 或 `infos`。

## 输出格式

默认使用中文回答，并包含：

- `查询目标`: 用户要判断的问题和时间窗口。
- `PromQL`: 实际执行的表达式。
- `结果摘要`: 关键数值、趋势、异常对象和单位。
- `判断`: 正常、异常、证据不足，并给出置信度。
- `下一步`: 一个具体的后续查询或排查动作。

如果用户只要求给 PromQL，不调用 tool；如果用户要求“查一下”“看一下”“验证一下”，应调用 `query_prometheus`。

## 安全与边界

- 这个 skill 只读 Prometheus，不做重启、扩容、回滚、删除或任何生产变更。
- 不把 Prometheus 返回的大量原始序列完整贴出；优先汇总 Top N、异常点和统计结果。
- 不泄露 token、内部账号、客户标识等敏感 label；需要时做脱敏。
- 结论要区分“Prometheus 指标显示”和“根因推断”，不要把相关性直接说成因果。
