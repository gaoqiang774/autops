---
name: host-query
displayName: 主机资产查询
version: 1.0.0
description: 按 hostId、IP、主机名、实例名或关键词查询 CMDB 主机资产。
category: query
domain: cmdb
skillGroup: cmdb_skill
runtime: declarative
status: enabled
riskLevel: readonly
readonly: true
source: built-in
tags:
  - cmdb
  - host
  - asset
triggers:
  - 查主机
  - 主机资产
  - 这台机器是谁
  - 根据 IP 查询
tools:
  - get_cmdb_host
  - search_cmdb_hosts
permissions:
  - cmdb:host:read
inputSchema:
  type: object
  properties:
    target:
      type: string
      description: hostId、IP、hostname、instance 或资产关键词
    limit:
      type: number
      description: 模糊搜索返回数量
outputSchema:
  type: object
  properties:
    host:
      type: object
    candidates:
      type: array
    summary:
      type: string
---

## 触发条件

当用户需要定位主机、查询 IP 归属、确认主机名、实例 ID、所属分组、系统信息或资产上下文时使用。

## 执行流程

1. 如果用户提供明确 hostId、IP、hostname 或 instance，优先调用 `get_cmdb_host`。
2. 如果用户提供的是模糊关键词，调用 `search_cmdb_hosts`。
3. 如果返回多个候选，列出关键字段并提示用户确认目标。
4. 输出主机 ID、名称、私网 IP、公网 IP、系统、厂商、分组和状态。

## 边界

- 只读取 CMDB 资产，不修改资产。
- 不推断不存在的主机信息。
- 查询不到资产时，明确说明缺失并建议补充 IP、主机名或实例 ID。
