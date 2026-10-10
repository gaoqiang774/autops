---
name: host-pressure
displayName: 主机压力分析
version: 1.0.0
description: 分析主机 CPU、内存、磁盘、负载和 Top 进程压力。
category: diagnosis
domain: monitor
skillGroup: monitor_skill
runtime: declarative
status: enabled
riskLevel: readonly
readonly: true
source: built-in
tags:
  - host
  - metrics
  - pressure
  - prometheus
triggers:
  - 主机压力
  - CPU 高
  - 内存高
  - 磁盘满
  - 负载高
  - 机器卡顿
tools:
  - get_cmdb_host
  - get_agent_status
  - query_host_metrics
  - query_host_services
permissions:
  - cmdb:host:read
  - monitor:metric:read
  - monitor:agent:read
inputSchema:
  type: object
  required:
    - target
  properties:
    target:
      type: string
      description: hostId、IP、hostname 或 instance
    minutes:
      type: number
      description: 指标查询时间窗口，默认 5 分钟
outputSchema:
  type: object
  properties:
    severity:
      type: string
    evidence:
      type: array
    suggestions:
      type: array
---

## 触发条件

当用户询问某台主机是否资源紧张、CPU/内存/磁盘/负载异常、服务卡顿或容量风险时使用。

## 执行流程

1. 使用 `get_cmdb_host` 定位主机。
2. 使用 `get_agent_status` 判断采集 Agent 是否安装和在线。
3. 使用 `query_host_metrics` 获取 CPU、内存、磁盘、负载和 Top 进程。
4. 使用 `query_host_services` 推断可能受影响服务。
5. 输出风险等级、证据、可能原因和下一步检查建议。

## 边界

- 不执行清理、重启、扩容或 kill 进程。
- 没有监控数据时必须说明证据缺失。
- Top 进程只能作为线索，不能单独作为根因结论。
