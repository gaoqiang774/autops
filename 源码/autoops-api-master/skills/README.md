# AutoOps Skills

This directory contains declarative Skill packages for the AIOps agent layer.
Each Skill is a standalone directory with a `SKILL.md` file. The file uses YAML
frontmatter for machine-readable metadata and Markdown body for human-readable
execution rules.

## Directory Layout

```text
skills/
  cmdb_skill/       CMDB and asset context skills
  k8s_skill/        Kubernetes diagnosis and context skills
  monitor_skill/    Alert, metrics and pressure analysis skills
  log_skill/        Host and Kubernetes log skills
  db_skill/         Database context, health and slow SQL skills
  knowledge_skill/  Runbook, FAQ and incident knowledge skills
  task_skill/       Async task and controlled runbook task skills
  skill_all/        Built-in skill catalog and registration order
```

## Standard Metadata

Every `SKILL.md` should provide these fields:

```yaml
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
tools:
  - get_cmdb_host
permissions:
  - cmdb:host:read
---
```

## Categories

- `query`: read-only lookup.
- `analysis`: summarize evidence and explain status.
- `diagnosis`: collect evidence, infer risks and suggest next checks.
- `inspection`: periodic or checklist-style inspection.
- `task`: async task status, dry-run or controlled execution flow.
- `knowledge`: knowledge base, runbook and incident document retrieval.

## Runtime Types

- `declarative`: orchestrates existing Tool Gateway tools.
- `go`: backed by built-in Go implementation.
- `script`: calls bundled scripts after policy checks.
- `http`: calls an external service through a controlled gateway.
- `mcp`: delegates to an MCP tool.

## Safety Rule

Skills declare intent and workflow. They do not own production execution
authority. Real execution must go through the AutoOps Tool Gateway, policy
checks, audit records and, for non-readonly actions, approval.
