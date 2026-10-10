---
name: pod-diagnosis
displayName: K8s Pod 诊断
version: 1.0.0
description: 分析 K8s Pod 状态、事件和日志，支持同步或异步诊断任务。
category: diagnosis
domain: k8s
skillGroup: k8s_skill
runtime: go
status: enabled
riskLevel: readonly
readonly: true
source: built-in
tags:
  - k8s
  - pod
  - event
  - log
triggers:
  - Pod 异常
  - Pod 起不来
  - CrashLoopBackOff
  - K8s Pod 分析
tools:
  - AnalyzeK8sPod
  - StartK8sPodAnalyzeTask
permissions:
  - k8s:pod:read
  - k8s:event:read
  - k8s:log:read
inputSchema:
  type: object
  required:
    - clusterId
    - namespace
    - podName
  properties:
    clusterId:
      type: number
    namespace:
      type: string
    podName:
      type: string
    container:
      type: string
outputSchema:
  type: object
  properties:
    summary:
      type: string
    evidence:
      type: array
    remediation:
      type: array
---

## 触发条件

当用户询问 Pod Pending、CrashLoopBackOff、镜像拉取失败、探针失败、重启频繁或 Pod 日志异常时使用。

## 执行流程

1. 调用现有 K8s Pod 分析能力收集状态、事件和日志。
2. 识别常见异常：调度失败、镜像失败、探针失败、OOM、配置错误。
3. 输出证据摘要、可能原因、验证步骤和处理建议。
4. 任务较重时使用异步任务接口并返回 taskId、streamUrl 和 resultUrl。

## 边界

- 不删除、重启或扩缩容 Pod。
- 没有事件或日志时必须说明证据缺失。
