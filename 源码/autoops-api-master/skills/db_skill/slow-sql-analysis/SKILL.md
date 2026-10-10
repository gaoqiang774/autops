---
name: slow-sql-analysis
displayName: 慢 SQL 分析
version: 1.0.0
description: 分析数据库慢 SQL Top、执行计划、表结构和实例健康证据。
category: diagnosis
domain: database
skillGroup: db_skill
runtime: go
status: enabled
riskLevel: readonly
readonly: true
source: built-in
tags:
  - database
  - mysql
  - slow-sql
  - performance
triggers:
  - 慢 SQL
  - SQL 很慢
  - 查询慢
  - 数据库压力
tools:
  - db_get_slow_sql_top
  - db_get_instance_health
  - db_get_monitor_snapshot
  - db_get_table_schema
  - db_explain_sql
permissions:
  - db:asset:read
  - db:monitor:read
  - db:sql:readonly
inputSchema:
  type: object
  required:
    - assetId
  properties:
    assetId:
      type: number
    database:
      type: string
    sql:
      type: string
    objectName:
      type: string
    authId:
      type: number
outputSchema:
  type: object
  properties:
    summary:
      type: string
    slowSqlTop:
      type: array
    suggestions:
      type: array
---

## 触发条件

当用户询问数据库慢查询、SQL 性能、慢日志 Top、执行计划或实例健康时使用。

## 执行流程

1. 使用 `db_get_instance_health` 获取实例健康背景。
2. 使用 `db_get_monitor_snapshot` 获取监控快照。
3. 如果用户询问慢日志，使用 `db_get_slow_sql_top`。
4. 如果用户提供 SQL，使用 `db_explain_sql` 分析执行计划。
5. 如果用户提供表名，使用 `db_get_table_schema` 获取结构和索引。
6. 输出慢因线索、证据、优化方向和缺失数据。

## 边界

- 只允许只读 SQL 和只读分析工具。
- 不生成或执行 DDL/DML 修复动作。
- 慢日志未配置时必须明确提示配置缺失。
