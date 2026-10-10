---
name: k8s-context
displayName: K8s 集群上下文
version: 1.0.0
description: 读取 Kubernetes 集群上下文，用于集群总览、资源定位和风险分析。
category: analysis
domain: k8s
skillGroup: k8s_skill
runtime: declarative
status: enabled
riskLevel: readonly
readonly: true
source: built-in
tags:
  - k8s
  - cluster
  - context
triggers:
  - K8s 概况
  - 集群概览
  - Kubernetes 资源
  - 集群风险
tools:
  - aiops_context_k8s
permissions:
  - k8s:cluster:read
inputSchema:
  type: object
  properties:
    clusterId:
      type: number
    keyword:
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

当用户询问 Kubernetes 集群总览、资源上下文、集群风险或需要为其他 K8s 诊断补充背景时使用。

## 执行流程

1. 调用 `aiops_context_k8s` 获取集群上下文。
2. 汇总集群、命名空间、工作负载和异常线索。
3. 输出可继续排查的对象和缺失信息。

## 边界

- 不修改 Kubernetes 资源。
- 上下文摘要不能替代 Pod、Ingress 或事件级诊断。
