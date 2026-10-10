---
name: alert-diagnosis
displayName: 告警诊断
version: 1.0.0
description: 根据告警补齐 CMDB、指标、日志和知识库证据，生成只读诊断建议。
category: diagnosis
domain: monitor
skillGroup: monitor_skill
runtime: declarative
status: enabled
riskLevel: readonly
readonly: true
source: built-in
tags:
  - alert
  - rca
  - evidence
triggers:
  - 分析告警
  - 告警诊断
  - 这个报警为什么
  - RCA
tools:
  - query_alert_detail
  - query_active_alerts
  - get_cmdb_host
  - query_host_metrics
  - query_host_logs
  - knowledge_search
permissions:
  - monitor:alert:read
  - monitor:metric:read
  - cmdb:host:read
  - log:host:read
  - knowledge:document:read
inputSchema:
  type: object
  properties:
    alertId:
      type: number
    fingerprint:
      type: string
    hostId:
      type: number
    logPath:
      type: string
outputSchema:
  type: object
  properties:
    diagnosis:
      type: object
    evidence:
      type: array
    missingData:
      type: array
---

## 触发条件

当用户要求分析某条告警、补齐证据、判断影响范围或生成值班交接说明时使用。

## 执行流程

1. 使用 `query_alert_detail` 获取告警详情。
2. 如果告警关联主机，使用 `get_cmdb_host` 获取资产上下文。
3. 使用 `query_host_metrics` 查询相关时间窗口的指标。
4. 如果用户提供日志路径，使用 `query_host_logs` 查询日志证据。
5. 使用 `knowledge_search` 检索相关 Runbook、故障复盘或 FAQ。
6. 输出候选原因、证据强度、缺失数据和下一步检查建议。

## 边界

- 不声称已经执行修复。
- 没有日志路径时不要虚构日志证据。
- 根因只能标记为候选，除非证据链充分。
