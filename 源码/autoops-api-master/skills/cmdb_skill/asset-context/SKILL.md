---
name: asset-context
displayName: 主机资产上下文
version: 1.0.0
description: 汇总平台主机资产、认证状态和近期资产上下文，用于平台态势分析。
category: analysis
domain: cmdb
skillGroup: cmdb_skill
runtime: declarative
status: enabled
riskLevel: readonly
readonly: true
source: built-in
tags:
  - cmdb
  - inventory
  - context
triggers:
  - 主机资产概览
  - 资产覆盖
  - CMDB 概况
  - 平台有多少主机
tools:
  - aiops_context_hosts
permissions:
  - cmdb:host:read
inputSchema:
  type: object
  properties:
    limit:
      type: number
      description: 最近资产返回数量
outputSchema:
  type: object
  properties:
    summary:
      type: string
    evidence:
      type: array
---

## 触发条件

当用户询问平台主机规模、资产覆盖、认证配置、近期主机资产变化或总体主机上下文时使用。

## 执行流程

1. 调用 `aiops_context_hosts` 获取资产摘要。
2. 汇总主机数量、认证覆盖、最近资产和明显缺口。
3. 输出可作为其他诊断技能的背景证据。

## 边界

- 只做资产上下文总结，不执行巡检命令。
- 不把资产数量异常直接判断为故障，必须标明是风险线索。
