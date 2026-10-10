---
name: alert-overview
displayName: 活跃告警概览
version: 1.0.0
description: 查询当前活跃告警，按严重级别、关键词和分页条件汇总告警态势。
category: query
domain: monitor
skillGroup: monitor_skill
runtime: declarative
status: enabled
riskLevel: readonly
readonly: true
source: built-in
tags:
  - alert
  - prometheus
  - overview
triggers:
  - 当前告警
  - 活跃告警
  - 有哪些报警
  - 告警概览
tools:
  - query_active_alerts
  - query_alert_detail
permissions:
  - monitor:alert:read
inputSchema:
  type: object
  properties:
    severity:
      type: string
    keyword:
      type: string
    page:
      type: number
    pageSize:
      type: number
outputSchema:
  type: object
  properties:
    total:
      type: number
    items:
      type: array
    summary:
      type: string
---

## 触发条件

当用户询问当前告警数量、严重告警、某关键词告警或平台告警态势时使用。

## 执行流程

1. 调用 `query_active_alerts` 获取活跃告警列表。
2. 如果用户指定 alertId 或 fingerprint，调用 `query_alert_detail`。
3. 按严重级别、资源对象和告警名称汇总。
4. 给出需要优先排查的告警列表。

## 边界

- 只查询告警，不确认、不屏蔽、不关闭告警。
- 告警列表不是根因，需要结合指标、日志和 CMDB 继续诊断。
