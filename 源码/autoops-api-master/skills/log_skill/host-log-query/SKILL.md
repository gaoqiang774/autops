---
name: host-log-query
displayName: 主机日志查询
version: 1.0.0
description: 读取指定主机日志尾部内容，支持关键词、正则和级别过滤。
category: query
domain: log
skillGroup: log_skill
runtime: declarative
status: enabled
riskLevel: medium
readonly: true
source: built-in
tags:
  - log
  - host
  - tail
triggers:
  - 查日志
  - 看错误日志
  - 最近日志
  - 异常堆栈
tools:
  - get_cmdb_host
  - query_host_logs
permissions:
  - cmdb:host:read
  - log:host:read
inputSchema:
  type: object
  required:
    - target
    - path
  properties:
    target:
      type: string
      description: hostId、IP、hostname 或 instance
    path:
      type: string
      description: 远程日志路径
    keyword:
      type: string
    regex:
      type: string
    tailLines:
      type: number
outputSchema:
  type: object
  properties:
    entries:
      type: array
    summary:
      type: string
---

## 触发条件

当用户要求查看主机日志、错误、异常堆栈、最近日志内容或按关键词过滤日志时使用。

## 执行流程

1. 使用 `get_cmdb_host` 定位主机。
2. 检查用户是否提供明确日志路径。
3. 使用 `query_host_logs` 读取日志尾部内容。
4. 按关键字、正则或日志级别总结异常线索。

## 边界

- 必须有明确日志路径。
- 只读日志，不修改文件。
- 日志内容可能包含敏感信息，回答时应避免扩散密钥、token 和密码。
