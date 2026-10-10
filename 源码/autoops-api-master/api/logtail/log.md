# 日志模块设计对接方案（Vector 正式版）

## 1. 背景与目标

当前平台已经具备以下基础能力：

- 前端：`Vue`
- 后端：`Gin / Go API`
- 关系数据存储：`MySQL`
- 缓存与异步基础设施：`Redis`
- 指标监控存储：`VictoriaMetrics`
- 自研主机 `agent`
- `CMDB` 主机资产管理
- `Kubernetes` 管理与巡检能力
- 现有 `/aiops/log/*` 主机日志与 K8s 日志即席查询能力

本方案的目标不是再自研一个日志采集器，而是在现有平台基础上建设一套可平台化运营的中心化日志模块，形成类似云厂商 `SLS` 的能力底座：

1. 平台统一管理日志来源、采集策略和权限
2. 自动化下发日志采集配置到主机和 K8s 集群
3. 稳定采集并写入 `VictoriaLogs`
4. 提供结构化检索、聚合分析和图表展示
5. 基于日志建设告警能力
6. 为后续日志与指标、链路、AI 分析联动打基础

一期聚焦以下能力：

- 主机日志采集
- K8s 容器日志采集
- 统一字段模型
- 基础检索与聚合
- 日志告警
- 平台化配置与状态管理

一期不包含以下内容：

- 自研日志存储引擎
- 自研日志采集内核
- 完整 Trace 采集链路
- 热冷分层归档
- 多地域跨机房复制
- 面向第三方公网的大规模日志接入网关

## 2. 选型结论

### 2.1 最终决策

本方案最终推荐采用：

```text
自研 Agent / K8s 配置下发 + Vector + VictoriaLogs + vmalert + 现有运维平台
```

其中：

- `Vector` 作为日志采集、解析、转换、缓冲和转发的数据面组件
- 自研平台作为控制面，负责配置管理、资产关联、权限、审计、状态展示和自动化下发
- `VictoriaLogs` 作为统一日志存储与查询引擎
- `vmalert` 作为日志告警计算引擎
- 现有告警中心作为通知、收敛、升级和事件展示入口

### 2.2 为什么最终选择 Vector

结合当前平台现状和“类 SLS”目标，`Vector` 是最适合的一档选择，原因如下：

1. 更强的采集端处理能力  
   `Vector` 的 `VRL` 适合做日志清洗、字段提取、脱敏、标准化和动态路由，能够把大量“平台规则”前移到采集端或边缘节点执行。

2. 更适合平台自动化下发  
   `Vector` 是单一二进制，部署简单，配置文件结构清晰，适合由自研 `agent` 自动安装、升级、校验和重载。

3. 与 `VictoriaLogs` 组合顺畅  
   官方文档已有 `Vector -> VictoriaLogs` 的接入方式，适合直接构建统一日志链路。

4. 更接近“类 SLS”的产品目标  
   你的目标不只是“把日志送进去”，而是要做来源管理、字段标准化、聚合图表、告警和后续联动。`Vector` 在这类平台化场景里比单纯 shipper 更有优势。

5. 便于后续扩展  
   后续即便需要引入聚合层、脱敏规则中心、多目标路由，或与 `OTel` 体系结合，`Vector` 也更容易承接。

### 2.3 其他候选不作为主方案的原因

### `Fluent Bit`

优点：

- 资源占用低
- `K8s` 场景成熟
- 多行、K8s 元数据、容器日志采集都比较成熟

不作为首选的原因：

- 更偏高性能 shipper
- 复杂处理和规则编排的可维护性通常不如 `Vector`
- 面向“平台级规则下发”的可编程能力弱一档

结论：

- 可作为极致轻量场景下的备选
- 不作为本平台正式主方案

### `Promtail`

不推荐新上，原因：

- 官方已进入生命周期尾声
- 更偏 `Loki` 生态
- 不适合作为新的长期技术基座

### `iLogtail / LoongCollector`

优点：

- 能力很强
- 阿里生态经验丰富
- 处理器和插件体系完整

不作为首选的原因：

- 更适合围绕阿里系观测体系展开
- 与 `VictoriaLogs` 的组合不是其最原生主路径
- 引入后平台会更偏向特定生态，而不是中立可控的数据面

结论：

- 如果未来强依赖阿里观测生态，可以单独评估
- 当前阶段不作为主方案

## 3. 设计原则

### 3.1 控制面与数据面分离

- 平台负责配置、权限、审计、资产关联和展示
- `Vector` 负责采集、处理、缓冲和转发
- `VictoriaLogs` 负责日志存储和查询

### 3.2 不重复造采集器轮子

- 不自研日志采集内核
- 不自研高吞吐日志接入服务作为一期主链路
- 复用成熟开源采集器，把研发精力放在平台控制面和业务闭环上

### 3.3 与现有能力协同而不是替代

当前平台已经有：

- `/aiops/log/host/query`
- `/aiops/log/k8s/query`
- `/aiops/log/analyze`

这些能力保留为：

- 即席排障
- 临时远程日志查看
- AI 辅助分析入口

新的中心化日志模块负责：

- 长期留存
- 聚合查询
- 图表分析
- 日志告警
- 标准化审计和权限控制

### 3.4 先打通标准化日志平台，再做链路闭环

日志平台先解决：

- 接入
- 治理
- 查询
- 告警

链路分析部分通过统一 `trace_id / span_id / service / env` 先完成日志可关联，真正的分布式链路采集建议后续接入 `OTLP + VictoriaTraces`。

## 4. 总体架构

### 4.1 一期标准架构

一期推荐主链路如下：

```text
                           +----------------------+
                           |      Vue Console     |
                           +----------+-----------+
                                      |
                                      v
                           +----------------------+
                           |      Go API          |
                           | 配置/权限/查询/审计  |
                           +-----+-----------+----+
                                 |           |
                     +-----------+           +-------------------+
                     v                                           v
              +-------------+                              +-------------+
              |    MySQL    |                              |    Redis    |
              | 配置/规则/审计|                              | 缓存/任务状态 |
              +-------------+                              +-------------+
                     |
                     v
     +---------------------------------------------------------------+
     | 平台控制面                                                     |
     | 日志库 / 来源 / 解析模板 / 路由规则 / 告警规则 / 权限 / 审计   |
     +--------------------------+------------------------------------+
                                |
                 +--------------+------------------+
                 |                                 |
                 v                                 v
      +-------------------------+       +---------------------------+
      | 自研 Agent              |       | K8s Config/DaemonSet 管理 |
      | 安装/升级/校验/重载Vector |       | 下发 Vector 配置          |
      +------------+------------+       +-------------+-------------+
                   |                                        |
                   v                                        v
         +------------------+                     +------------------+
         | Host Vector      |                     | K8s Vector       |
         | file/journald    |                     | kubernetes_logs  |
         | parse/VRL/buffer |                     | parse/VRL/buffer |
         +--------+---------+                     +--------+---------+
                  |                                        |
                  +--------------------+-------------------+
                                       |
                                       v
                            +----------------------+
                            | vmauth(可选)         |
                            | 鉴权/路由/限流       |
                            +----------+-----------+
                                       |
                                       v
                            +----------------------+
                            |    VictoriaLogs      |
                            | 存储/检索/聚合       |
                            +-----+-----------+----+
                                  |           |
                                  v           v
                        +---------------+   +-------------------+
                        |   vmalert     |   | Go API Query      |
                        |  vlogs rules  |   | 封装查询与图表接口 |
                        +-------+-------+   +---------+---------+
                                |                     |
                                v                     v
                    +-----------------------+   +------------------+
                    | 现有告警中心 / AM     |   | Vue 日志中心      |
                    | 通知/升级/收敛/静默   |   | 检索/聚合/图表    |
                    +-----------------------+   +------------------+
```

### 4.2 架构说明

- `Vector` 是日志数据面的核心组件
- `Go API` 是日志模块的控制面核心
- `MySQL` 存储日志模块配置、规则、审计和状态
- `Redis` 不承担日志热路径队列，主要用于缓存、异步任务和状态协调
- `VictoriaLogs` 承担统一日志落库、查询和聚合
- `vmauth` 不是必须，但生产上建议加，用于写入鉴权、路由和隔离
- `vmalert` 负责日志告警计算
- 现有告警中心负责通知、值班、升级、抑制和告警事件呈现

### 4.3 扩展架构

当后续规模增大、跨网络或多机房接入需求增强时，可以平滑扩展为两层架构：

```text
Host/K8s Vector Agent -> Aggregator Vector -> VictoriaLogs
```

适用场景：

- 单机房写入量很大
- 需要统一边缘脱敏
- 需要跨网络做缓冲与汇聚
- 需要多目标分发

一期默认不引入聚合层，先采用边缘 `Vector` 直写 `VictoriaLogs`，保持链路简单。

## 5. 平台职责边界

| 模块 | 职责 |
| --- | --- |
| `Vue 日志中心` | 日志库管理、来源管理、解析模板、检索页面、聚合图表、告警规则页面 |
| `Go API` | 权限控制、配置管理、资产绑定、配置渲染、查询封装、审计记录、对接 `VictoriaLogs` / `vmalert` |
| `MySQL` | 日志库、来源、采集策略、解析模板、Token、规则、审计、Agent 状态 |
| `Redis` | 配置缓存、下发任务状态、异步重试、前端查询缓存 |
| 自研 `agent` | 主机侧安装、升级、配置写入、`Vector` 校验与重载、健康状态回传 |
| `K8s` 管理模块 | 集群侧 `DaemonSet`、`ConfigMap`、命名空间策略下发 |
| `Vector` | 日志采集、解析、标准化、脱敏、缓冲、转发 |
| `VictoriaLogs` | 存储、全文检索、聚合分析 |
| `vmalert` | 日志规则计算和告警触发 |
| 现有告警中心 | 告警通知、静默、升级、收敛、告警列表和历史 |

## 6. 主机与 K8s 部署策略

### 6.1 主机部署策略

主机侧推荐采用：

```text
自研 agent + Vector system service
```

落地方式：

1. 自研 `agent` 负责安装 `Vector` 二进制
2. `agent` 定时拉取平台生成的目标配置
3. 本地执行配置校验
4. 校验通过后写入配置并重载 `Vector`
5. 回传版本号、运行状态、错误信息和采集指标

主机侧推荐采集源：

- `file`
- `journald`
- 容器宿主机日志目录
- `syslog`

主机侧推荐能力：

- 支持多行合并
- 本地磁盘缓冲
- 边缘脱敏
- 按文件路径或服务归属打标签
- 暴露 `Vector` 自身运行指标供现有监控抓取

适合的日志类型：

- 应用文件日志
- `Nginx` / `Apache` 访问日志
- 系统日志
- `Java` 堆栈多行日志
- 中间件文件日志

### 6.2 K8s 部署策略

`K8s` 场景推荐采用：

```text
每集群一个 Vector DaemonSet
```

理由：

- 统一采集节点上的容器标准输出日志
- 便于自动补充命名空间、Pod、容器、节点、工作负载元数据
- 便于和现有 `K8s` 管理模块打通

`K8s` 侧推荐源：

- `kubernetes_logs`
- 节点系统日志
- 必要时的宿主机挂载目录文件日志

建议策略：

1. 默认优先采集容器标准输出，不鼓励业务在容器内额外写文件日志
2. 对确有文件日志需求的存量应用，可通过挂载目录定向采集
3. 除特殊场景外，不推荐大规模使用 sidecar 采集
4. 解析模板和路由规则按命名空间、工作负载、注解或标签下发

适用场景：

- 统一采集 `stdout/stderr`
- 统一做 K8s 元数据补全
- 支持按 `cluster / namespace / workload / pod / container` 检索和聚合

### 6.3 配置下发策略

主机与 `K8s` 分别采用不同的控制方式：

### 主机

- 平台生成来源对应的 `Vector` 配置片段
- 自研 `agent` 拉取配置版本
- 本地校验后写入并重载

### K8s

- 集群级 `Vector` 使用 `--config-dir /etc/vector` 加载配置目录
- 公共参数和共享标准输出 source 写入 `aiops-vector-base` ConfigMap
- 每个日志项目生成一个 `aiops-vector-<projectName>-<projectId>` ConfigMap，聚合该项目下的采集任务
- `DaemonSet` 通过 projected volume 将基础配置和项目配置挂载为多个 YAML 文件
- 项目配置中的 Vector 组件名称包含任务 ID，确保跨项目、跨文件全局唯一
- 更新配置集合后通过 `aiops.dev/config-hash` 触发 `DaemonSet` 滚动生效
- 项目已无启用任务时，控制器自动清理对应的项目 ConfigMap
- 以滚动方式生效

### 6.4 平台不直接管理的内容

以下内容不建议放到平台控制面做复杂在线编排：

- 任意用户自定义 `VRL` 脚本
- 任意多目标转发
- 任意 shell 级插件执行

平台应提供受控模板和白名单能力，而不是无限开放原始采集器能力。

## 7. 日志字段模型

### 7.1 设计目标

日志字段模型应满足以下要求：

- 主机与 `K8s` 来源统一
- 检索字段稳定
- 告警字段稳定
- 对后续 `trace` 关联友好
- 控制高基数字段带来的查询与存储成本

### 7.2 核心标准字段

| 字段 | 含义 | 是否必填 | 说明 |
| --- | --- | --- | --- |
| `_time` | 日志发生时间 | 是 | 统一转为 `RFC3339` |
| `_msg` | 日志正文 | 是 | 标准化消息体 |
| `_raw` | 原始日志 | 否 | 保留原始内容便于追溯 |
| `event_id` | 事件唯一 ID | 是 | 用于详情查看和幂等去重 |
| `received_at` | 平台接收时间 | 是 | 记录进入平台时间 |
| `collector` | 采集器标识 | 是 | 固定为 `vector` |
| `store_id` | 日志库 ID | 是 | 平台内的逻辑日志库 |
| `store_code` | 日志库编码 | 是 | 查询与规则引用更稳定 |
| `source_id` | 日志来源 ID | 是 | 来源主键 |
| `source_name` | 日志来源名称 | 是 | 便于前端展示 |
| `source_type` | 来源类型 | 是 | `host_file / host_journald / k8s_stdout / k8s_file / middleware` |
| `env` | 环境 | 是 | `prod / test / staging` |
| `service_name` | 服务名 | 否 | 与应用或工作负载关联 |
| `severity_text` | 日志级别文本 | 否 | `trace/debug/info/warn/error/fatal` |
| `severity_number` | 日志级别数值 | 否 | 便于排序与告警 |
| `trace_id` | 链路追踪 ID | 否 | 用于与 Trace 关联 |
| `span_id` | Span ID | 否 | 用于与 Trace 关联 |

### 7.3 主机来源字段

| 字段 | 说明 |
| --- | --- |
| `host_id` | CMDB 主机 ID |
| `host_name` | 主机名 |
| `host_ip` | 主 IP |
| `host_group_id` | 主机分组 |
| `file_path` | 文件路径 |
| `inode` | 文件 inode，可选 |
| `systemd_unit` | `journald` 单元名，可选 |

### 7.4 K8s 来源字段

| 字段 | 说明 |
| --- | --- |
| `cluster_id` | 集群 ID |
| `cluster_name` | 集群名 |
| `namespace` | 命名空间 |
| `workload_kind` | `Deployment / StatefulSet / DaemonSet / Job` |
| `workload_name` | 工作负载名 |
| `pod_name` | Pod 名 |
| `container_name` | 容器名 |
| `node_name` | 节点名 |
| `pod_ip` | Pod IP |
| `image` | 镜像名，可选 |

### 7.5 业务扩展字段

建议约定两类扩展字段：

- `labels.*`
- `fields.*`

含义：

- `labels.*`：平台白名单内的稳定标签，适合检索和聚合
- `fields.*`：动态解析出的业务字段，用于补充展示和局部查询

建议治理规则：

1. 平台查询、聚合、告警默认只保证标准字段和 `labels.*` 白名单字段稳定可用
2. `fields.*` 不鼓励无限扩张
3. 动态字段数量、键长度、值长度应有限制
4. 超长字段应截断或转 `_raw`

### 7.6 幂等与 `event_id` 生成规则

如果业务日志自身没有稳定事件 ID，建议由 `Vector` 或平台按以下信息生成哈希：

```text
store_code + source_id + _time + _raw + file_path/pod_name + line fingerprint
```

设计目标：

- 重试时尽量不重复
- 详情查询可按 `event_id` 回看
- 死信或补写时有稳定主键

### 7.7 样例

### 主机日志样例

```json
{
  "_time": "2026-07-02T10:20:30Z",
  "_msg": "error connect db timeout",
  "_raw": "2026-07-02 18:20:30 ERROR error connect db timeout",
  "event_id": "f2d0f408c0d84fd8a3b57c4e38d5f8e1",
  "received_at": "2026-07-02T10:20:31Z",
  "collector": "vector",
  "store_id": 3001,
  "store_code": "order-prod",
  "source_id": 4001,
  "source_name": "order-api-file",
  "source_type": "host_file",
  "env": "prod",
  "service_name": "order-api",
  "severity_text": "error",
  "severity_number": 17,
  "host_id": 101,
  "host_name": "node-01",
  "host_ip": "10.0.0.12",
  "file_path": "/data/logs/order/app.log",
  "trace_id": "6e0f0a0f48f24f1c8e6ef9346aeb9c81",
  "fields.thread": "http-8080-exec-1",
  "labels.team": "trade"
}
```

### K8s 日志样例

```json
{
  "_time": "2026-07-02T10:20:30Z",
  "_msg": "POST /api/pay timeout",
  "_raw": "{\"time\":\"2026-07-02T10:20:30Z\",\"level\":\"error\",\"msg\":\"POST /api/pay timeout\"}",
  "event_id": "07a5db0bbf734ca79dc48c889cb34f8f",
  "received_at": "2026-07-02T10:20:31Z",
  "collector": "vector",
  "store_id": 5001,
  "store_code": "payment-prod",
  "source_id": 6001,
  "source_name": "payment-k8s",
  "source_type": "k8s_stdout",
  "env": "prod",
  "service_name": "payment-api",
  "severity_text": "error",
  "severity_number": 17,
  "cluster_id": 11,
  "cluster_name": "prod-cluster",
  "namespace": "payment",
  "workload_kind": "Deployment",
  "workload_name": "payment-api",
  "pod_name": "payment-api-7bd9f64b9c-b9fgh",
  "container_name": "app",
  "node_name": "k8s-node-03",
  "trace_id": "1f9d8c4036d7464f89e7cb0af468eb1e",
  "labels.team": "finance"
}
```

## 8. 查询、聚合与图表设计

### 8.1 查询策略

平台不直接把底层原生查询语言暴露给普通前端页面，一期推荐由后端封装结构化查询：

- 关键字
- 时间范围
- 日志库
- 来源
- 主机
- 集群
- 命名空间
- 服务
- 级别
- `trace_id`

查询结果支持：

- 列表检索
- 详情查看
- 上下文日志
- 保存查询
- 常用筛选

### 8.2 聚合能力

一期建议提供以下聚合能力：

- 时间序列数量趋势
- `error/warn/info` 级别分布
- TopN 主机
- TopN 服务
- TopN 命名空间
- TopN 异常关键字
- TopN Trace

这些能力足够支撑：

- 日志中心图表
- 问题定位入口
- 日志告警规则构建

### 8.3 图表展示策略

前端建议在“日志检索”之外增加“日志分析”页，提供：

- 时间趋势图
- 级别饼图
- 服务或主机 TopN 柱状图
- 命名空间或工作负载聚合图

平台要做的是：

- 管理日志库和来源
- 统一封装查询接口
- 把图表体验做成运维平台的一部分

而不是一开始就重做一个独立的通用 BI 系统。

## 9. 日志告警链路设计

### 9.1 正式告警链路

推荐链路如下：

```text
Vector -> VictoriaLogs -> vmalert(vlogs rule) -> 现有告警中心/Alertmanager -> 通知渠道
```

说明：

- `Vector` 只负责把标准化日志送到 `VictoriaLogs`
- `vmalert` 负责按日志规则做计算
- 告警通知、静默、升级、值班轮值等能力继续复用现有告警中心

### 9.2 一期支持的规则类型

一期建议支持以下规则类型：

1. 关键字命中次数阈值  
   例：5 分钟内 `timeout` 超过 100 次

2. 错误级别日志阈值  
   例：10 分钟内 `severity_text=error` 超过 50 次

3. 错误率阈值  
   例：5 分钟内 `error / total` 超过 5%

4. 特定来源异常突增  
   例：某服务、某主机、某命名空间日志量突增

5. 日志缺失告警  
   例：关键服务 10 分钟没有任何心跳日志

一期不建议直接上复杂异常检测模型，先把规则型告警做好。

### 9.3 平台侧职责

平台负责：

- 告警规则管理
- 通知对象绑定
- 日志库与来源范围控制
- 告警事件展示
- 审计规则变更
- 与现有告警中心联动

`vmalert` 负责：

- 周期执行日志查询规则
- 触发告警事件
- 输出统一告警标签和注释

### 9.4 告警事件建议附带的标签

建议至少附带：

- `store_code`
- `source_name`
- `env`
- `service_name`
- `host_name`
- `cluster_name`
- `namespace`
- `severity_text`

这样告警中心可以直接展示“问题来自哪个业务对象”。

## 10. 链路分析协同设计

### 10.1 先关联日志，再补齐 Trace

要实现“链路分析”，需要明确：

- `Vector` 是日志采集器，不是完整的分布式 Trace 方案
- 日志平台可以为链路分析做准备，但不能单靠日志采集器替代 `Trace`

推荐路线：

1. 一期统一要求日志尽可能带 `trace_id / span_id`
2. 日志查询页支持按 `trace_id` 检索
3. 后续引入 `OTLP + VictoriaTraces`
4. 日志、指标、链路按 `service / env / trace_id` 打通

### 10.2 后续目标架构

```text
App Logs -> Vector -> VictoriaLogs
App Traces -> OTel Collector -> VictoriaTraces
App Metrics -> Agent / Exporter -> VictoriaMetrics

Vue / Go API -> 统一把 Logs / Metrics / Traces 关联展示
```

## 11. 安全与审计设计

### 11.1 写入鉴权

推荐采用以下方式：

- 每个日志库或来源签发独立写入凭证
- 凭证由平台管理
- 通过 `vmauth` 或受控写入入口完成鉴权

建议：

- `MySQL` 中不保存明文 token，只保存摘要或加密态
- 支持禁用、过期、轮换
- 支持按日志库或来源粒度授权

### 11.2 数据权限

查询侧继续复用平台现有权限体系，并增加日志维度控制：

- 日志库权限
- 主机权限
- 应用或服务权限
- `K8s` 集群或命名空间权限

### 11.3 脱敏

建议在采集侧优先做第一层脱敏：

- `password`
- `token`
- `secret`
- `authorization`
- `api_key`

平台查询和 AI 分析侧再做第二层脱敏，避免敏感数据直接进入模型或前端。

### 11.4 审计

一期就建议纳入审计的操作：

- 创建、修改、删除日志来源
- 创建、禁用、轮换写入凭证
- 修改解析模板
- 修改告警规则
- 查询日志
- 导出日志

## 12. 可靠性与可观测性设计

### 12.1 可靠性策略

一期主链路推荐：

- `Vector` 本地内存 + 磁盘缓冲
- `Vector` 直写 `VictoriaLogs`
- 不在日志热路径引入 `Redis Streams`

原因：

- 避免在高吞吐链路中再维护一层自研接入服务
- 降低系统复杂度
- 充分利用成熟采集器的缓冲与重试能力

### 12.2 异常场景处理

### `VictoriaLogs` 短时不可用

- `Vector` 本地缓冲积压
- 平台展示采集延迟和缓冲状态
- 超阈值时触发基础设施告警

### 配置错误

- 下发前先做配置校验
- 校验失败不覆盖旧配置
- 回传错误详情到平台

### 节点异常

- 平台展示 `Vector` 在线状态
- 采集空窗按主机、集群、来源维度告警

### 12.3 基础监控指标

建议纳入现有监控体系：

- `Vector` 进程存活
- 输入速率
- 输出速率
- 丢弃数
- 缓冲大小
- 缓冲落盘数
- 重试次数
- `VictoriaLogs` 写入错误数
- `VictoriaLogs` 查询耗时
- `vmalert` 规则执行失败数

## 13. 平台数据模型建议

### 13.1 先明确存储边界

一期要先把边界定死，否则后端很容易越做越重：

- `VictoriaLogs`：只存真实日志数据、聚合查询结果的底层来源
- `MySQL`：只存控制面数据，包括日志库、来源、模板、告警规则、凭证、状态、审计
- `Redis`：只存缓存、异步任务状态、短期查询缓存，不存日志正文

也就是说：

- 不要在 `MySQL` 再建 `log_record`、`log_detail` 这种“日志正文表”
- 不要把每条日志落一份到 `MySQL`
- `MySQL` 只承担“日志平台元数据中心”的职责

### 13.2 根据当前需求收敛后的功能对象

你当前明确的需求可以抽象成 5 个后端对象：

1. 项目  
   一个项目下面可以有多个日志库

2. 日志库  
   日志查询、聚合分析、告警规则都围绕日志库展开

3. 采集任务  
   选择执行 Agent、目标日志库、日志路径、解析模式

4. 告警规则  
   支持关键字、错误数量、字段条件等触发方式

5. 告警历史  
   支持列表展示、跳转原始日志、后续接 AI 诊断报告

所以这次模型设计不再优先用“通用 source 概念”做中心，而是直接围绕你前端页面里的：

- 项目
- 日志库
- 采集任务
- 告警规则
- 告警历史

来设计表结构。

### 13.3 一期推荐表数

结合当前需求和“先上线打通流程”的目标，当前后端 MVP 先落 **6 张核心表**：

1. `log_project`
2. `log_store`
3. `log_parse_template`
4. `log_collect_task`
5. `log_alert_rule`
6. `log_alert_event`

这个拆法已经能覆盖当前产品主线：

- `log_project` 对应“用户创建项目”
- `log_store` 对应“项目下多个日志库”
- `log_collect_task` 对应“采集任务”
- `log_alert_rule` 对应“告警配置”
- `log_alert_event` 对应“告警历史”

以下能力先不在一期 MVP 单独拆表：

- 采集任务执行目标先并入 `log_collect_task.execute_targets_json`
- 写入凭证先不单独建表
- Agent 状态先不单独建表
- 查询审计先不单独建表
- 保存查询先不单独建表

### 13.4 表设计建议

#### 1. `log_project`

用途：

- 日志模块里的一级业务对象
- 一个项目下面包含多个日志库

建议字段：

- `id`
- `name`
- `code`
- `description`
- `owner_type`
- `owner_id`
- `owner_name`
- `status`
- `env`
- `extra_json`
- `created_by`
- `updated_by`
- `created_at`
- `updated_at`

字段说明：

- `code`：项目稳定编码，建议唯一索引
- `owner_type / owner_id`：归属团队、应用、业务线等
- `env`：项目默认环境，可为空

建议索引：

- `uk_log_project_code(code)`
- `idx_log_project_status(status)`

#### 2. `log_store`

用途：

- 定义项目下的日志库
- 日志查询、聚合、告警规则主要围绕日志库展开

建议字段：

- `id`
- `project_id`
- `name`
- `code`
- `description`
- `store_type`
- `retention_days`
- `status`
- `sort`
- `vlogs_tenant`
- `extra_json`
- `created_by`
- `updated_by`
- `created_at`
- `updated_at`

字段说明：

- `project_id`：一个项目下多个日志库
- `store_type`：`app / access / audit / system / middleware`
- `retention_days`：保留天数
- `vlogs_tenant`：为后续 `vmauth` 隔离预留

建议索引：

- `uk_log_store_code(code)`
- `idx_log_store_project_id(project_id)`
- `idx_log_store_status(status)`

#### 3. `log_parse_template`

用途：

- 保存日志解析模式
- 对应你页面里的“原始文本(raw) / 正则 / JSON / Nginx / Java 多行”等解析模式

建议字段：

- `id`
- `project_id`
- `name`
- `code`
- `parse_mode`
- `input_format`
- `multiline_enabled`
- `multiline_pattern`
- `time_key`
- `time_format`
- `level_key`
- `message_key`
- `trace_id_key`
- `span_id_key`
- `fields_mapping_json`
- `labels_mapping_json`
- `vrl_script`
- `status`
- `version`
- `remark`
- `created_by`
- `updated_by`
- `created_at`
- `updated_at`

字段说明：

- `parse_mode`：`raw / json / regex / logfmt / nginx / java / custom`
- `multiline_pattern`：对接你截图里的“多行合并正则”
- `vrl_script`：平台托管的受控 `VRL`

建议索引：

- `uk_log_parse_template_code(code)`
- `idx_log_parse_template_project_id(project_id)`
- `idx_log_parse_template_status(status)`

#### 4. `log_collect_task`

用途：

- 对应“采集任务”主表
- 负责描述一条采集配置

建议字段：

- `id`
- `project_id`
- `store_id`
- `name`
- `code`
- `collect_type`
- `execute_mode`
- `execute_targets_json`
- `path_pattern`
- `parse_template_id`
- `parse_mode_snapshot`
- `multiline_pattern_snapshot`
- `labels_json`
- `filters_json`
- `config_version`
- `status`
- `remark`
- `created_by`
- `updated_by`
- `created_at`
- `updated_at`

字段说明：

- `store_id`：目标日志库
- `collect_type`：`host_file / host_journald / k8s_stdout / k8s_file`
- `execute_mode`：`all_agents / selected_agents / host_group / k8s_cluster / namespace`
- `execute_targets_json`：一期先保存执行目标 JSON，不单独拆目标表
- `path_pattern`：日志路径，支持 glob，如 `/var/log/nginx/*.log`
- `parse_template_id`：解析模式模板
- `parse_mode_snapshot`：保留任务创建时的解析模式快照，便于历史回溯
- `multiline_pattern_snapshot`：保留任务侧的多行正则快照
- `config_version`：用于 Agent 配置下发版本控制

建议索引：

- `uk_log_collect_task_code(code)`
- `idx_log_collect_task_project_id(project_id)`
- `idx_log_collect_task_store_id(store_id)`
- `idx_log_collect_task_status(status)`

#### 二期预留：`log_collect_task_target`

用途：

- 对应“采集任务执行目标”
- 支撑你页面里的“所有 Agent / 指定 Agent / 指定主机 / 指定集群”

建议字段：

- `id`
- `task_id`
- `target_type`
- `agent_id`
- `host_id`
- `host_group_id`
- `cluster_id`
- `namespace`
- `workload_kind`
- `workload_name`
- `status`
- `extra_json`
- `created_at`
- `updated_at`

字段说明：

- `target_type`：`agent / host / host_group / cluster / namespace / workload`
- 如果 `execute_mode=all_agents`，这一张表可以不落明细，只在任务表标记
- 如果是“指定 Agent”，这里记录具体执行对象

建议索引：

- `idx_log_collect_task_target_task_id(task_id)`
- `idx_log_collect_task_target_agent_id(agent_id)`
- `idx_log_collect_task_target_host_id(host_id)`
- `idx_log_collect_task_target_cluster_id(cluster_id)`

#### 二期预留：`log_write_token`

用途：

- 保存写入 `VictoriaLogs` 的凭证
- 给主机 Agent 或 `K8s Vector` 使用

建议字段：

- `id`
- `project_id`
- `store_id`
- `task_id`
- `name`
- `token_hash`
- `token_prefix`
- `scope_type`
- `expired_at`
- `last_used_at`
- `last_used_ip`
- `status`
- `remark`
- `created_by`
- `updated_by`
- `created_at`
- `updated_at`

字段说明：

- `scope_type`：`project / store / task`
- 一期建议以 `store` 或 `task` 为主
- `token_hash`：只存哈希，不存明文

建议索引：

- `idx_log_write_token_project_id(project_id)`
- `idx_log_write_token_store_id(store_id)`
- `idx_log_write_token_task_id(task_id)`
- `idx_log_write_token_status(status)`

#### 7. `log_alert_rule`

用途：

- 对应“日志告警配置”
- 支持关键字告警、错误阈值、字段条件

建议字段：

- `id`
- `project_id`
- `store_id`
- `name`
- `code`
- `rule_type`
- `severity`
- `scan_interval_minutes`
- `window_minutes`
- `threshold`
- `silence_minutes`
- `keywords_json`
- `field_conditions_json`
- `query_json`
- `notify_channels_json`
- `labels_json`
- `annotations_json`
- `status`
- `last_eval_at`
- `remark`
- `created_by`
- `updated_by`
- `created_at`
- `updated_at`

字段说明：

- `rule_type`：`keyword_count / error_count / field_match / error_rate`
- `keywords_json`：如 `timeout / OOM / panic / fatal`
- `field_conditions_json`：对应你截图里的“字段条件”
- `threshold`：阈值
- `scan_interval_minutes`：扫描频率
- `silence_minutes`：静默时间

建议索引：

- `uk_log_alert_rule_code(code)`
- `idx_log_alert_rule_project_id(project_id)`
- `idx_log_alert_rule_store_id(store_id)`
- `idx_log_alert_rule_status(status)`

#### 8. `log_alert_event`

用途：

- 对应“告警历史”
- 保存每次触发记录，支持查看原始日志、后续接诊断报告

建议字段：

- `id`
- `rule_id`
- `project_id`
- `store_id`
- `source_name`
- `severity`
- `status`
- `trigger_reason`
- `summary`
- `match_count`
- `first_match_time`
- `last_match_time`
- `fired_at`
- `recovered_at`
- `sample_query_json`
- `sample_event_ids_json`
- `diagnosis_report_id`
- `notify_status`
- `notify_result`
- `created_at`
- `updated_at`

字段说明：

- `status`：`firing / resolved / ignored`
- `trigger_reason`：触发原因摘要
- `summary`：你截图里“告警内容”
- `sample_query_json`：便于“原始日志”跳转回放
- `sample_event_ids_json`：保存命中的示例日志 ID
- `diagnosis_report_id`：后续可挂 AI 分析报告

建议索引：

- `idx_log_alert_event_rule_id(rule_id)`
- `idx_log_alert_event_project_id(project_id)`
- `idx_log_alert_event_store_id(store_id)`
- `idx_log_alert_event_status(status)`
- `idx_log_alert_event_fired_at(fired_at)`

#### 二期预留：`log_agent_status`

用途：

- 对应“Agent 管理”或采集执行状态
- 记录每个 Agent / 节点当前配置和健康情况

建议字段：

- `id`
- `task_id`
- `agent_id`
- `host_id`
- `cluster_id`
- `node_name`
- `vector_version`
- `config_version`
- `collect_status`
- `health_status`
- `last_heartbeat_at`
- `last_collect_at`
- `last_error`
- `last_error_at`
- `buffer_usage_bytes`
- `retry_count`
- `metrics_json`
- `created_at`
- `updated_at`

字段说明：

- `collect_status`：`running / stopped / failed`
- `health_status`：`healthy / warning / error`
- `config_version`：判断任务配置是否已生效
- `last_collect_at`：最后一次实际采集时间

建议索引：

- `idx_log_agent_status_task_id(task_id)`
- `idx_log_agent_status_agent_id(agent_id)`
- `idx_log_agent_status_host_id(host_id)`
- `idx_log_agent_status_cluster_id(cluster_id)`
- `idx_log_agent_status_collect_status(collect_status)`

#### 二期预留：`log_query_audit`

用途：

- 审计日志查询、原始日志跳转、导出

建议字段：

- `id`
- `user_id`
- `username`
- `project_id`
- `store_id`
- `action`
- `query_json`
- `result_count`
- `duration_ms`
- `client_ip`
- `user_agent`
- `trace_id`
- `status`
- `error_msg`
- `created_at`

字段说明：

- `action`：`query / detail / context / export / alert_jump`
- `trace_id`：后端链路追踪 ID

建议索引：

- `idx_log_query_audit_user_id(user_id)`
- `idx_log_query_audit_project_id(project_id)`
- `idx_log_query_audit_store_id(store_id)`
- `idx_log_query_audit_created_at(created_at)`

#### 二期预留：`log_saved_query`（可选）

用途：

- 保存用户常用查询条件

建议字段：

- `id`
- `user_id`
- `project_id`
- `store_id`
- `name`
- `query_json`
- `is_favorite`
- `is_public`
- `created_at`
- `updated_at`

### 13.5 你的 4 个需求分别对应哪些表

#### 需求 1：用户创建项目，项目下面包含多个日志库

对应表：

- `log_project`
- `log_store`

关系：

```text
log_project 1 --- n log_store
```

#### 需求 2：日志查询，可聚合展示查询

对应表：

- `log_store`

说明：

- 真正的日志数据和聚合结果来自 `VictoriaLogs`
- `MySQL` 一期只保存“日志库元数据”，查询审计和保存查询放到二期

#### 需求 3：日志告警，可配置关键字告警

对应表：

- `log_alert_rule`
- `log_alert_event`

说明：

- `log_alert_rule` 存规则
- `log_alert_event` 存触发历史

#### 需求 4：配置日志采集，选择日志库、日志路径、解析模式

对应表：

- `log_collect_task`
- `log_parse_template`

说明：

- `log_collect_task` 是采集配置主表
- `execute_targets_json` 先放在 `log_collect_task` 中
- `log_parse_template` 是解析模式

### 13.6 一期建议先不建的表

以下表建议先不建：

- `log_collect_task_target`
- `log_route_policy`
- `log_alert_rule_binding`
- `log_export_task`
- `log_write_token`
- `log_agent_status`
- `log_query_audit`
- `log_saved_query`
- `log_alert_notify_record`
- `log_dead_letter`
- `log_ingest_record`

原因：

1. 你当前主链路是 `Vector -> VictoriaLogs`
2. 一期重点是项目、日志库、采集任务、告警规则、告警历史
3. 采集目标、凭证、审计等能力可以先放宽，保证主流程先跑起来
4. 规则和通知链路先复用现有告警中心，不必在日志模块重复建太细

### 13.7 一期最小后端落地顺序

如果现在马上开发表，我建议顺序如下：

1. `log_project`
2. `log_store`
3. `log_parse_template`
4. `log_collect_task`
5. `log_alert_rule`
6. `log_alert_event`

这样第一阶段就能支撑：

- 项目管理
- 日志库管理
- 采集任务管理
- 解析模式管理
- 日志查询
- 关键字告警
- 告警历史

## 14. 迭代规划

### 14.1 一期

- 完成主机和 `K8s` 的 `Vector` 采集闭环
- 完成 `VictoriaLogs` 存储与检索
- 完成日志库、来源、模板和权限管理
- 完成基础聚合图表
- 完成基础日志告警

### 14.2 二期

- 增加上下文日志
- 增加更多聚合维度
- 增加来源健康看板
- 增加日志导出审计
- 增加边缘聚合层选项

### 14.3 三期

- 接入 `VictoriaTraces`
- 打通 `Logs + Metrics + Traces`
- 接入 AI 日志诊断
- 实现故障对象自动关联

## 15. 最终决策结论

结合当前平台现状，日志模块正式推荐采用以下方案：

```text
平台控制面：
Vue + Go API + MySQL + Redis + CMDB + K8s 管理 + 自研 Agent

日志数据面：
Host Vector / K8s Vector DaemonSet -> vmauth(可选) -> VictoriaLogs

日志告警：
VictoriaLogs -> vmalert -> 现有告警中心

链路扩展：
后续通过 OTLP -> VictoriaTraces 与日志打通
```

该方案的核心优势：

- 不重复造日志采集轮子
- 最大限度复用你现有平台和自研 `agent`
- 更适合“类 SLS”的平台化建设方向
- 更适合做字段治理、聚合图表和日志告警
- 能平滑扩展到 `Logs + Metrics + Traces + AI` 的一体化运维平台

这应作为当前日志模块的正式落地方案。
