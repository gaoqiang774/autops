---
name: knowledge-search
displayName: 运维知识库检索
version: 1.0.0
description: 检索运维知识库、Runbook、故障复盘和 FAQ，为回答补充文档证据。
category: knowledge
domain: knowledge
skillGroup: knowledge_skill
runtime: declarative
status: enabled
riskLevel: readonly
readonly: true
source: built-in
tags:
  - knowledge
  - runbook
  - faq
  - incident
triggers:
  - 查知识库
  - Runbook
  - 故障案例
  - 操作手册
tools:
  - knowledge_search
permissions:
  - knowledge:document:read
inputSchema:
  type: object
  required:
    - query
  properties:
    query:
      type: string
    limit:
      type: number
    docType:
      type: string
    category:
      type: string
    serviceName:
      type: string
outputSchema:
  type: object
  properties:
    items:
      type: array
    summary:
      type: string
---

## 触发条件

当用户需要查询运维知识库、故障复盘、FAQ、Runbook 或希望回答带文档引用时使用。

## 执行流程

1. 使用 `knowledge_search` 检索相关文档。
2. 优先返回标题、摘要、文档类型、标签和相关片段。
3. 将命中文档作为 evidence 注入其他诊断技能。

## 边界

- 检索结果只能作为参考证据。
- 不把 Runbook 文档等同于已经执行的操作。
