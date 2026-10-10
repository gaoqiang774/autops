---
name: async-task-status
displayName: 异步任务状态查询
version: 1.0.0
description: 查询平台异步任务状态，例如 K8s Pod 分析、慢日志分析、文件上传和火焰图任务。
category: task
domain: task
skillGroup: task_skill
runtime: go
status: draft
riskLevel: readonly
readonly: true
source: built-in
tags:
  - task
  - async
  - status
triggers:
  - 任务状态
  - taskId
  - 分析进度
  - 查询任务
tools:
  - GetK8sPodAnalyzeTask
  - GetSlowLogTask
  - GetUploadTaskStatus
  - GetFlameGraphTask
permissions:
  - task:read
inputSchema:
  type: object
  required:
    - taskId
  properties:
    taskId:
      type: string
    taskType:
      type: string
      description: k8s_pod_analyze、db_slow_log、host_upload、rdp_upload、arthas_flamegraph
outputSchema:
  type: object
  properties:
    status:
      type: string
    progress:
      type: number
    resultUrl:
      type: string
---

## 触发条件

当用户提供 taskId 并询问分析进度、任务是否完成、任务结果或失败原因时使用。

## 执行流程

1. 根据 taskType 或 taskId 上下文选择对应任务查询能力。
2. 查询任务状态、进度、错误信息和结果地址。
3. 如果任务已完成，提示用户查看结果或继续总结结果。

## 边界

- 只查询任务，不取消或删除任务。
- 当前为 draft，需要后续统一不同模块的任务查询接口后启用。
