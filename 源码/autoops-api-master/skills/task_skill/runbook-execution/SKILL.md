---
name: runbook-execution
displayName: Runbook 受控执行
version: 0.1.0
description: 根据知识库 Runbook 生成执行计划、dry-run 和审批建议；当前不直接执行生产变更。
category: task
domain: task
skillGroup: task_skill
runtime: declarative
status: planned
riskLevel: high
readonly: false
source: built-in
tags:
  - runbook
  - approval
  - task
  - remediation
triggers:
  - 执行 Runbook
  - 按手册处理
  - 自动修复
  - 生成执行计划
tools:
  - knowledge_search
permissions:
  - knowledge:document:read
  - runbook:plan
  - approval:request
inputSchema:
  type: object
  required:
    - goal
  properties:
    goal:
      type: string
    target:
      type: string
    runbookId:
      type: number
outputSchema:
  type: object
  properties:
    plan:
      type: array
    risk:
      type: string
    approvalRequired:
      type: boolean
---

## 触发条件

当用户希望根据 Runbook 生成处理计划、dry-run、影响范围说明或审批单草案时使用。

## 执行流程

1. 使用 `knowledge_search` 检索相关 Runbook。
2. 解析目标、前置条件、执行步骤、回滚步骤和验证步骤。
3. 输出执行计划、风险等级、所需权限、审批建议和人工确认点。
4. 后续平台具备审批和受控执行能力后，再接入真实执行工具。

## 边界

- 当前不直接执行生产变更。
- 高风险动作必须审批。
- 没有明确目标、回滚方案和验证步骤时，不应建议执行。
