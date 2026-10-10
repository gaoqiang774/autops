---
name: database-overview
displayName: 数据库资产概览
version: 1.0.0
description: 读取数据库资产上下文，用于数据库实例定位、认证状态和平台风险分析。
category: analysis
domain: database
skillGroup: db_skill
runtime: declarative
status: enabled
riskLevel: readonly
readonly: true
source: built-in
tags:
  - database
  - asset
  - context
triggers:
  - 数据库概览
  - 数据库资产
  - 数据库实例
  - DB 风险
tools:
  - aiops_context_database
permissions:
  - db:asset:read
inputSchema:
  type: object
  properties:
    assetId:
      type: number
    keyword:
      type: string
    env:
      type: string
    scope:
      type: string
outputSchema:
  type: object
  properties:
    summary:
      type: string
    evidence:
      type: array
---

## 触发条件

当用户询问数据库资产、实例状态、认证配置、监控登记或数据库整体风险时使用。

## 执行流程

1. 调用 `aiops_context_database` 获取数据库资产上下文。
2. 汇总实例、认证、监控、告警和缺失配置。
3. 输出风险线索和建议补充的数据。

## 边界

- 不执行 SQL。
- 不把资产上下文等同于实时数据库性能诊断。
