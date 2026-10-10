---
name: ingress-diagnosis
displayName: K8s Ingress 链路诊断
version: 1.0.0
description: 分析 Ingress 域名、Service、Endpoint 和后端 Pod 链路。
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
  - ingress
  - service
  - endpoint
triggers:
  - Ingress 异常
  - 域名访问失败
  - 入口链路
  - K8s 访问 502
tools:
  - AnalyzeK8sIngress
  - StreamK8sIngressAnalyze
permissions:
  - k8s:ingress:read
  - k8s:service:read
  - k8s:endpoint:read
  - k8s:pod:read
inputSchema:
  type: object
  properties:
    clusterId:
      type: number
    namespace:
      type: string
    ingressName:
      type: string
    host:
      type: string
outputSchema:
  type: object
  properties:
    summary:
      type: string
    evidence:
      type: array
    nextChecks:
      type: array
---

## 触发条件

当用户询问 Kubernetes 域名访问失败、Ingress 规则异常、Service 无 Endpoint、后端 Pod 不健康或 502/503 问题时使用。

## 执行流程

1. 调用 Ingress 分析能力获取 Ingress、Service、Endpoint 和 Pod 链路。
2. 检查域名规则、后端服务、Endpoint 数量和后端 Pod 状态。
3. 结合 HTTP 探测结果输出异常环节。
4. 返回链路证据、候选原因和下一步验证建议。

## 边界

- 不修改 Ingress、Service 或 Pod。
- 网络外部依赖无法确认时，应标记为待验证。
