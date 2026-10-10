# 任务中心前端开发与 API 对接文档

## 1. 文档目的

本文档用于指导前端实现任务中心模块，并与当前后端接口完成对接。覆盖以下功能：

- 任务模板：Shell、Python、Ansible 模板的新增、编辑、查询和删除
- 普通任务：选择模板和主机后立即执行
- 定时任务：使用五段式 Cron 表达式周期执行
- 任务作业：启动、停止、查询状态和查看日志
- Ansible 任务：手动上传 Playbook、从 Git 仓库导入、实时日志
- K8s 部署任务：基于 Ansible 任务执行体系创建和执行集群部署任务
- 任务监控：队列、调度器、Redis 状态及定时任务控制

文中的请求路径均以 `/api/v1` 为基础路径。若生产环境由网关增加了额外前缀，前端只需在请求客户端的 `baseURL` 中统一配置，不要在业务组件中拼接部署前缀。

---

## 2. 推荐页面结构

建议在“任务中心”菜单下提供以下页面或页签。

### 2.1 任务调度

用于创建和管理普通任务、定时任务，建议包含：

- 顶部筛选：任务名称、任务类型、任务状态
- 表格字段：任务名称、类型、模板、执行主机、状态、执行次数、下次执行时间、耗时、创建时间
- 行操作：详情、执行/启用、暂停、恢复、停止、重新执行、编辑、删除
- 新建/编辑抽屉：基本信息、模板选择、主机选择、调度配置、备注

任务类型筛选不能直接传给分页列表接口。推荐做法是：普通列表使用 `/task/list-with-details`，类型筛选时调用 `/task/query/type`；也可以先加载分页结果后在当前页本地筛选，但这会导致分页总数不准确。

### 2.2 执行历史

当前后端没有独立的“执行历史记录”表查询接口，前端可以基于任务列表展示最近一次执行结果：

- 默认展示状态为成功、异常、已停止的任务
- 详情中展示模板、主机、开始时间、结束时间、耗时、执行次数和日志
- 定时任务的父任务执行完成后仍会保持“运行中”，表示调度处于启用状态，不代表某次脚本仍在执行

不要在界面上虚构多次执行历史。`execute_count` 是累计执行次数，当前接口只提供任务最近一次执行时间和日志状态。

### 2.3 模板库

建议使用表格加编辑抽屉：

- 筛选：模板名称、模板类型
- 表格：名称、类型、备注、创建人、更新人、更新时间
- 编辑器：Shell/Python/Ansible 内容编辑，支持等宽字体、行号和全屏
- 操作：预览、编辑、删除

### 2.4 Ansible 自动化

建议独立页面，不要和普通脚本任务的创建表单混在一起：

- 任务列表：名称、来源类型、Playbook 数量、状态、总耗时、更新时间
- 新建方式：手动上传、Git 导入、K8s 部署
- 任务详情：子任务列表，每个子任务单独显示状态、耗时、日志和停止按钮
- 日志抽屉：黑色等宽日志终端，支持自动滚动、暂停滚动和清屏

### 2.5 运行监控

建议只向管理员开放：

- 队列是否运行、等待数量、累计成功/失败数量、活跃 Worker
- 调度器状态、已注册任务数、最近运行时间
- Redis 连接状态
- 失败队列清空和重试

---

## 3. 通用请求约定

### 3.1 认证

普通 HTTP 请求使用：

```http
Authorization: Bearer <token>
```

SSE 无法通过浏览器原生 `EventSource` 设置请求头时，可以传查询参数：

```text
?token=<token>
```

WebSocket 日志接口必须推荐使用查询参数传递 Token：

```text
ws://host/api/v1/ws/task/ansible/{taskId}/log/{workId}?token=<token>
```

Token 必须使用 `encodeURIComponent` 编码。不要把带 Token 的完整 URL 写入日志或错误上报。

### 3.2 统一响应结构

大部分接口使用以下响应：

```ts
export interface ApiResult<T> {
  code: number
  message: string
  data: T
}

export interface PageResult<T> {
  total: number
  list: T[]
  page: number
  pageSize: number
}
```

成功示例：

```json
{
  "code": 200,
  "message": "成功",
  "data": {}
}
```

后端的统一错误响应通常仍然使用 HTTP 200，业务错误码放在 `code` 中。因此普通接口不能只判断 HTTP 状态，必须同时判断 `code === 200`。

```ts
const api = axios.create({
  baseURL: '/api/v1',
  timeout: 30000
})

api.interceptors.request.use((config) => {
  const token = getToken()
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

api.interceptors.response.use((response) => {
  const body = response.data

  // 只处理使用统一包装的响应。部分 Ansible 接口是直接 JSON。
  if (body && typeof body.code === 'number' && body.code !== 200) {
    return Promise.reject(new Error(body.message || '请求失败'))
  }
  return body
})
```

### 3.3 非统一响应接口

以下 Ansible 接口没有使用统一的 `ApiResult`，前端需要单独适配：

| 接口 | 成功响应 |
| --- | --- |
| `GET /task/ansiblelist` | `{ "data": [...], "total": 10 }` |
| `POST /task/ansible/:id/start` | `{ "message": "任务已开始执行" }` |
| `POST /task/ansible/:id/stop/:work_id` | `{ "message": "任务停止成功" }` |
| `DELETE /task/ansible/:id` | `{ "message": "任务删除成功", "data": {...} }` |

这些接口失败时可能返回真实的 HTTP 4xx/5xx 和 `{ "error": "..." }`。建议统一错误提取逻辑：优先读取 `response.data.message`，其次读取 `response.data.error`。

---

## 4. 枚举和前端类型

### 4.1 枚举

```ts
export enum TemplateType {
  Shell = 1,
  Python = 2,
  Ansible = 3
}

export enum TaskType {
  Immediate = 1,
  Scheduled = 2
}

export enum TaskStatus {
  Pending = 1,
  Running = 2,
  Success = 3,
  Failed = 4,
  Paused = 5,
  Stopped = 6
}

export enum AnsibleTaskType {
  Manual = 1,
  Git = 2,
  K8s = 3
}
```

推荐状态文案和颜色：

| 值 | 文案 | 推荐颜色 | 含义 |
| --- | --- | --- | --- |
| `1` | 等待中 | 灰色 | 已创建，尚未执行或尚未启用调度 |
| `2` | 运行中 | 蓝色 | 普通任务正在执行；定时任务表示调度已启用 |
| `3` | 成功 | 绿色 | 最近一次执行成功 |
| `4` | 异常 | 红色 | 最近一次执行失败 |
| `5` | 已暂停 | 橙色 | 定时调度已暂停 |
| `6` | 已停止 | 灰黑色 | 用户主动停止 |

Ansible 父任务和子任务当前主要使用 `1` 至 `4`。

### 4.2 核心 TypeScript 类型

```ts
export interface TaskTemplate {
  id: number
  name: string
  type: TemplateType
  content: string
  remark: string
  createdBy: string
  updatedBy: string
  createdAt: string
  updatedAt: string
}

export interface TaskHost {
  id: number
  hostName: string
  privateIp: string
  publicIp: string
  sshIp: string
  status: number
}

export interface Task {
  id: number
  name: string
  type: TaskType
  shell: string
  host_ids: string
  cron_expr: string
  tasklog: string
  status: TaskStatus
  duration: number
  task_count: number
  execute_count: number
  next_run_time: string | null
  remark: string
  start_time: string | null
  end_time: string | null
  created_at: string
}

export interface TaskWithDetails extends Task {
  templates: Array<{
    id: number
    name: string
    type: TemplateType
    content: string
  }>
  hosts: TaskHost[]
}

export interface TaskWorkStatus {
  status: TaskStatus
  log: string
  start_time?: string
  end_time?: string
  duration?: number
}

export interface AnsibleWork {
  id: number
  taskID: number
  entryFileName: string
  entryFilePath: string
  logPath: string
  status: number
  startTime?: string
  endTime?: string
  duration: number
  exitCode: number
  errorMsg?: string
}

export interface AnsibleTask {
  id: number
  name: string
  description: string
  type: AnsibleTaskType
  gitRepo: string
  hostGroups: string
  allHostIDs: string
  globalVars: string
  status: number
  errorMsg: string
  taskCount: number
  totalDuration: number
  createdAt: string
  updatedAt: string
  works?: AnsibleWork[]
}
```

注意：`hostGroups`、`allHostIDs`、`globalVars` 在 Ansible 列表响应中是 JSON 字符串，展示前需要安全地 `JSON.parse`，解析失败时显示原始值或空值，不能让整个列表渲染失败。

---

## 5. 模板库接口

### 5.1 接口清单

| 功能 | 方法和路径 | 参数位置 | 参数 |
| --- | --- | --- | --- |
| 新建模板 | `POST /template/add` | JSON Body | `name`、`type`、`content`、`remark` |
| 模板列表 | `GET /template/list` | 无 | 无分页，返回全部模板 |
| 编辑模板 | `PUT /template/update?id={id}` | Query + JSON Body | Query: `id`；Body 同新建 |
| 删除模板 | `DELETE /template/delete?id={id}` | Query | `id` |
| 模板详情 | `GET /template/info/:id` | Path | `id` |
| 获取纯脚本 | `GET /template/content/:id` | Path | `id` |
| 按名称查询 | `GET /template/query/name?name={name}` | Query | `name`，模糊匹配 |
| 按类型查询 | `GET /template/query/type?type={type}` | Query | `type=1/2/3` |

### 5.2 新建模板

```json
{
  "name": "Nginx 服务检查",
  "type": 1,
  "content": "systemctl status nginx",
  "remark": "检查生产主机 Nginx 状态"
}
```

字段规则：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是 | 唯一名称，前端去除首尾空格 |
| `type` | number | 是 | `1=Shell`、`2=Python`、`3=Ansible` |
| `content` | string | 是 | 模板内容，不能为空 |
| `remark` | string | 否 | 备注，建议限制 500 字符 |

创建成功时 `data` 为：

```json
{
  "id": 12,
  "name": "Nginx 服务检查",
  "type": 1,
  "message": "任务模板创建成功"
}
```

`createdBy` 和 `updatedBy` 由后端根据登录用户填写，前端不要提交。

### 5.3 模板内容接口的特殊处理

`GET /template/content/:id` 成功时直接返回 `text/plain`，不包含 `code/message/data`。Axios 调用时应显式设置文本响应：

```ts
const response = await axios.get(`/api/v1/template/content/${id}`, {
  headers: { Authorization: `Bearer ${getToken()}` },
  responseType: 'text'
})
const content = response.data
```

---

## 6. 普通任务和定时任务接口

### 6.1 创建表单设计

建议字段顺序如下：

1. 任务名称：文本框，必填
2. 任务类型：分段选择“立即执行 / 定时任务”
3. 任务模板：多选，数据来自 `/template/list`
4. 执行主机：多选，数据来自现有 CMDB 主机列表 `/cmdb/hostlist`
5. Cron 表达式：仅定时任务显示，必填
6. 备注：多行文本

前端表单可以使用数组，但提交前必须转换为逗号分隔字符串：

```ts
const payload = {
  name: form.name.trim(),
  type: form.type,
  shell: form.templateIds.join(','),
  host_ids: form.hostIds.join(','),
  cron_expr: form.type === TaskType.Scheduled ? form.cronExpr.trim() : '',
  remark: form.remark?.trim() || ''
}
```

`shell` 不是脚本内容，而是模板 ID 字符串。`host_ids` 也是主机 ID 字符串。后端会创建“主机数量 x 模板数量”个执行子任务。

例如选择模板 `[2, 5]`、主机 `[10, 12, 13]`，后端会创建 6 个执行子任务。

### 6.2 接口清单

| 功能 | 方法和路径 | 参数 |
| --- | --- | --- |
| 创建任务 | `POST /task/add` | JSON Body，见下文 |
| 任务详情 | `GET /task/get?id={id}` | Query: `id` |
| 编辑任务 | `PUT /task/update` | JSON Body，必须包含 `id` |
| 删除任务 | `DELETE /task/delete` | JSON Body: `{ "id": 1 }` |
| 分页列表 | `GET /task/list` | `page`、`pageSize`、`name?`、`status?` |
| 分页详情列表 | `GET /task/list-with-details` | `page`、`pageSize`、`name?`、`status?` |
| 按名称查询 | `GET /task/query/name?name={name}` | `name`，模糊匹配 |
| 按类型查询 | `GET /task/query/type?type={type}` | `type=1/2` |
| 按状态查询 | `GET /task/query/status?status={status}` | `status=1..6` |
| 校验 Cron/下次执行 | `GET /task/next-execution?cron={cron}` | `cron` |
| 执行统计 | `GET /task/execution-info?id={id}` | `id` |
| 关联模板状态 | `GET /task/templates?id={id}` | `id` |

分页参数规则：

- `page` 默认 `1`
- `pageSize` 默认 `10`，最大 `100`
- `name` 可选，模糊查询
- `status` 可选，不传或传空字符串表示全部
- 推荐使用 `/task/list-with-details`，其 `templates` 和 `hosts` 可直接用于表格展示

分页响应：

```json
{
  "code": 200,
  "message": "成功",
  "data": {
    "total": 1,
    "list": [
      {
        "id": 21,
        "name": "生产环境巡检",
        "type": 1,
        "shell": "2,5",
        "host_ids": "10,12",
        "cron_expr": "",
        "status": 1,
        "duration": 0,
        "task_count": 4,
        "execute_count": 0,
        "next_run_time": null,
        "remark": "发布后检查",
        "start_time": null,
        "end_time": null,
        "created_at": "2026-07-31T10:00:00+08:00",
        "templates": [],
        "hosts": []
      }
    ],
    "page": 1,
    "pageSize": 10
  }
}
```

### 6.3 创建任务

`POST /task/add`

立即执行任务请求：

```json
{
  "name": "生产环境巡检",
  "type": 1,
  "shell": "2,5",
  "host_ids": "10,12",
  "cron_expr": "",
  "remark": "发布后检查"
}
```

定时任务请求：

```json
{
  "name": "每日凌晨巡检",
  "type": 2,
  "shell": "2,5",
  "host_ids": "10,12",
  "cron_expr": "0 2 * * *",
  "remark": "每天 02:00 执行"
}
```

字段规则：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是 | 任务名称，全局唯一 |
| `type` | number | 是 | `1=立即执行`、`2=定时任务` |
| `shell` | string | 是 | 模板 ID，多个用英文逗号分隔 |
| `host_ids` | string | 是 | CMDB 主机 ID，多个用英文逗号分隔 |
| `cron_expr` | string | 条件必填 | `type=2` 时必填，五段式 Cron |
| `remark` | string | 否 | 备注 |

创建只保存任务，不会自动执行。前端在用户选择“保存并执行”时，需要等待创建成功，取返回的 `data.id`，再调用启动接口。

### 6.4 编辑任务

`PUT /task/update`

编辑接口按完整对象校验，不能只提交被修改的单个字段。推荐先调用 `/task/get?id=...`，再保留原状态提交：

```json
{
  "id": 21,
  "name": "生产环境巡检-更新",
  "type": 1,
  "shell": "2,5",
  "host_ids": "10,12,13",
  "cron_expr": "",
  "status": 1,
  "remark": "更新执行范围"
}
```

运行中的任务不能编辑。不要把前端表单的默认状态 `1` 强行覆盖原任务状态，否则会造成状态回退。`shell`、`host_ids` 或 `type` 改变时，后端会重新生成子任务。

### 6.5 删除任务

`DELETE /task/delete`

```json
{
  "id": 21
}
```

前端删除前应二次确认。定时任务删除后会同时从调度器移除，关联子任务也会删除。

### 6.6 Cron 输入规则

当前后端只接受标准五段式 Cron：

```text
分 时 日 月 周
```

示例：

| 表达式 | 含义 |
| --- | --- |
| `*/5 * * * *` | 每 5 分钟 |
| `0 * * * *` | 每小时整点 |
| `0 2 * * *` | 每天 02:00 |
| `0 9 * * 1-5` | 周一至周五 09:00 |

用户输入后可以防抖调用：

```ts
const result = await api.get('/task/next-execution', {
  params: { cron: form.cronExpr }
}) as ApiResult<{ next_execution_time: string }>
```

使用 Axios 的 `params` 传递表达式，不要手工拼接 URL 中的空格。

---

## 7. 任务执行、状态和日志

### 7.1 接口清单

| 功能 | 方法和路径 | 参数 |
| --- | --- | --- |
| 启动/重新执行/启用调度 | `POST /taskjob/start?taskId={taskId}` | Query: `taskId` |
| 获取日志 | `GET /taskjob/log?taskId={taskId}&templateId={templateId}` | Query: `taskId`、`templateId` |
| 停止任务 | `POST /taskjob/stop?taskId={taskId}&templateId={templateId}` | Query: `taskId`、`templateId` |
| 获取子任务状态 | `GET /taskjob/status?taskId={taskId}&templateId={templateId}` | Query: `taskId`、`templateId` |

### 7.2 立即任务调用流程

```text
创建任务 POST /task/add
  -> 取得 data.id
  -> 启动任务 POST /taskjob/start?taskId=...
  -> 每 2 秒刷新任务列表或查询任务详情
  -> 对选中的模板查询 /taskjob/status
  -> 完成后调用 /taskjob/log 获取完整日志
```

启动成功：

```json
{
  "code": 200,
  "message": "成功",
  "data": {
    "message": "任务已开始执行"
  }
}
```

任务完成后可以再次调用同一个启动接口重新执行。后端会重置已有子任务的状态和日志。

### 7.3 定时任务调用流程

```text
创建 type=2 的任务
  -> POST /taskjob/start?taskId=... 注册到调度器
  -> 父任务状态变为 2（运行中/调度已启用）
  -> 使用暂停、恢复接口控制调度
```

定时任务创建后如果不调用 `/taskjob/start`，只会保存在数据库中，不会进入调度器。

### 7.4 状态轮询

```ts
async function pollWorkStatus(taskId: number, templateId: number) {
  return api.get('/taskjob/status', {
    params: { taskId, templateId }
  }) as Promise<ApiResult<TaskWorkStatus>>
}
```

状态响应示例：

```json
{
  "code": 200,
  "message": "成功",
  "data": {
    "status": 3,
    "log": "检查完成",
    "start_time": "2026-07-31T10:00:00+08:00",
    "end_time": "2026-07-31T10:00:12+08:00",
    "duration": 12
  }
}
```

建议仅在状态为 `1` 或 `2` 时轮询，间隔 2 秒；页面隐藏、抽屉关闭或组件卸载时停止计时器。

### 7.5 普通任务日志

普通任务日志接口返回统一包装，`data` 本身是字符串：

```json
{
  "code": 200,
  "message": "成功",
  "data": "完整日志文本..."
}
```

当前 `taskId + templateId` 接口会查找第一个匹配的执行子任务。当同一模板在多台主机上执行时，该组合不能唯一定位某台主机。因此当前前端应：

- 在任务列表和详情中展示父任务聚合状态
- 日志入口按模板展示，并标注为“当前模板日志”
- 不承诺这是指定主机的日志
- 不要仅靠前端构造 `hostId`，后端接口当前不接收该参数

若后续要展示完整的“主机 x 模板”执行矩阵，需要后端新增按 `workId` 查询状态和日志的接口。

---

## 8. 定时任务控制和监控

### 8.1 查询操作权限

进入任务详情或打开行操作菜单时调用：

```http
GET /api/v1/task/monitor/task/status?task_id=21
```

响应：

```json
{
  "code": 200,
  "message": "成功",
  "data": {
    "task_id": 21,
    "name": "每日凌晨巡检",
    "type": 2,
    "type_name": "定时任务",
    "status": 2,
    "status_name": "运行中",
    "cron_expr": "0 2 * * *",
    "next_run_time": "2026-08-01T02:00:00+08:00",
    "execute_count": 3,
    "created_at": "2026-07-30T09:00:00+08:00",
    "scheduler_status": "scheduled",
    "operations": {
      "can_pause": true,
      "can_resume": false,
      "can_stop": true
    }
  }
}
```

前端按钮是否可用应优先使用 `operations`，不要仅靠本地状态猜测。

### 8.2 控制接口

| 功能 | 方法和路径 | 成功后的前端操作 |
| --- | --- | --- |
| 暂停定时任务 | `POST /task/monitor/scheduled/pause?task_id={id}` | 刷新列表和状态 |
| 恢复定时任务 | `POST /task/monitor/scheduled/resume?task_id={id}` | 刷新列表和下次执行时间 |
| 重置子任务状态 | `POST /task/monitor/scheduled/reset?task_id={id}` | 提示影响行数并刷新详情 |

重置接口是异常修复操作，会把状态为运行中、成功、失败的子任务重置为等待中，建议只放在管理员的“更多操作”中并二次确认。

### 8.3 监控接口

| 功能 | 方法和路径 | 参数 |
| --- | --- | --- |
| 队列指标 | `GET /task/monitor/queue/metrics` | 无 |
| 调度器统计 | `GET /task/monitor/scheduler/stats` | 无 |
| 系统整体状态 | `GET /task/monitor/system/status` | 无 |
| 队列详情 | `GET /task/monitor/queue/details` | 无 |
| 清空失败队列 | `POST /task/monitor/queue/clear-failed` | 无 |
| 重试失败任务 | `POST /task/monitor/queue/retry-failed?limit={limit}` | `limit` 默认 10，范围 1 到 100 |

队列指标字段：

```ts
export interface QueueMetrics {
  enqueued_total: number
  processed_total: number
  failed_total: number
  active_workers: number
  queue_length: number
  last_process_time: string
}
```

调度器统计主要字段：

```ts
export interface SchedulerStats {
  running: boolean
  total_tasks: number
  active_entries: number
  registered_task_ids?: number[]
  next_runs?: Array<{
    entry_id: number
    next_run: string
    prev_run: string
  }>
}
```

队列详情返回 `high`、`normal`、`low`、`retry`、`failed` 五组，每组包含 `length` 和 `key`。队列详情、清空和重试依赖 Redis；Redis 不可用时这些管理接口会失败，但普通任务执行可以回退到本地异步执行。

---

## 9. Ansible 任务

### 9.1 列表和查询接口

| 功能 | 方法和路径 | 参数 |
| --- | --- | --- |
| 分页列表 | `GET /task/ansiblelist?page={page}&size={size}` | `page` 默认 1；`size` 默认 10，最大 100 |
| 名称查询 | `GET /task/ansible/query/name?name={name}` | `name`，模糊匹配 |
| 类型查询 | `GET /task/ansible/query/type?type={type}` | `1=手动`、`2=Git`、`3=K8s` |
| 任务详情 | `GET /task/ansible/:id` | Path: `id` |
| 启动任务 | `POST /task/ansible/:id/start` | Path: `id` |
| 停止子任务 | `POST /task/ansible/:id/stop/:work_id` | Path: `id`、`work_id` |
| 删除任务 | `DELETE /task/ansible/:id` | Path: `id` |

分页列表是直接响应：

```json
{
  "data": [
    {
      "id": 8,
      "name": "批量安装采集器",
      "description": "",
      "type": 1,
      "gitRepo": "",
      "hostGroups": "{\"web\":[10,12]}",
      "allHostIDs": "[10,12]",
      "globalVars": "{\"env\":\"prod\"}",
      "status": 1,
      "errorMsg": "",
      "taskCount": 2,
      "totalDuration": 0,
      "createdAt": "2026-07-31T10:00:00+08:00",
      "updatedAt": "2026-07-31T10:00:00+08:00",
      "works": []
    }
  ],
  "total": 1
}
```

名称和类型查询使用统一包装，但内部又包含一层 `data`：

```json
{
  "code": 200,
  "message": "成功",
  "data": {
    "data": [],
    "total": 0
  }
}
```

因此建议前端把三种查询结果统一转换成 `{ list, total }` 后再交给表格组件。

### 9.2 手动上传 Ansible 任务

`POST /task/ansible`，请求类型必须是 `multipart/form-data`。

| FormData 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | string | 是 | 任务名称，必须唯一 |
| `type` | string | 是 | 固定为 `"1"` |
| `hostGroups` | JSON string | 是 | 主机分组，组名映射主机 ID 数组 |
| `variables` | JSON string | 否 | 全局变量，值必须为字符串 |
| `playbooks` | File，多文件 | 是 | 至少一个 `.yml` 或 `.yaml` 文件 |
| `roles` | File | 否 | 一个有效的 ZIP 压缩包 |

```ts
const formData = new FormData()
formData.append('name', form.name.trim())
formData.append('type', '1')
formData.append('hostGroups', JSON.stringify({
  web: [10, 12],
  db: [13]
}))
formData.append('variables', JSON.stringify({
  env: 'prod',
  app_version: 'v1.6.0'
}))

for (const file of form.playbooks) {
  formData.append('playbooks', file)
}
if (form.rolesZip) formData.append('roles', form.rolesZip)

await api.post('/task/ansible', formData)
```

不要手工设置带 boundary 的 `Content-Type`，让浏览器根据 `FormData` 自动生成。前端应提前校验：

- Playbook 文件名只能是普通文件名，不能包含路径
- Playbook 扩展名只能是 `.yml` 或 `.yaml`
- Roles 文件只能上传 ZIP
- 至少选择一个有效主机
- 当前 Ansible 执行不支持跳板机认证主机

### 9.3 Git 导入 Ansible 任务

同样调用 `POST /task/ansible`，使用 `multipart/form-data`，但不上传文件：

| FormData 字段 | 必填 | 示例 |
| --- | --- | --- |
| `name` | 是 | `Git-生产巡检` |
| `type` | 是 | `2` |
| `hostGroups` | 是 | `{"web":[10,12]}` |
| `gitRepo` | 是 | `https://git.example.com/ops/playbooks.git` |
| `variables` | 否 | `{"env":"prod"}` |

```ts
const formData = new FormData()
formData.append('name', form.name.trim())
formData.append('type', '2')
formData.append('hostGroups', JSON.stringify(form.hostGroups))
formData.append('gitRepo', form.gitRepo.trim())
formData.append('variables', JSON.stringify(form.variables || {}))
```

后端会克隆仓库并扫描 Playbook 文件。创建请求可能耗时较长，建议该请求单独设置 2 到 5 分钟超时，并展示明确的“正在拉取仓库”加载状态，避免用户重复提交。

### 9.4 Ansible 任务详情

`GET /task/ansible/:id`

当前详情接口返回的是精简结构，字段大小写必须按实际响应处理：

```json
{
  "code": 200,
  "message": "成功",
  "data": {
    "task_info": {
      "ID": 8,
      "Name": "批量安装采集器",
      "TaskCount": 2,
      "Works": [
        {
          "workid": 31,
          "EntryFileName": "install.yml",
          "status": 2,
          "Duration": 5
        }
      ]
    }
  }
}
```

```ts
export interface AnsibleTaskDetailResponse {
  task_info: {
    ID: number
    Name: string
    TaskCount: number
    Works: Array<{
      workid: number
      EntryFileName: string
      status: number
      Duration: number
    }>
  }
}
```

`workid` 是日志和停止接口需要的 `work_id`，必须保存，不能使用数组下标代替。

### 9.5 启动、停止和删除

启动整个父任务：

```http
POST /api/v1/task/ansible/8/start
```

后端会按子任务顺序异步执行。启动后立即请求详情，并在存在 `status=1/2` 的子任务时每 2 秒刷新详情。

停止指定子任务：

```http
POST /api/v1/task/ansible/8/stop/31
```

停止按钮只在子任务 `status === 2` 时显示。删除按钮只在父任务非运行中时显示；后端禁止删除运行中的任务。

---

## 10. K8s 部署任务

K8s 部署任务属于 Ansible 任务体系，创建后也使用 Ansible 的详情、启动、停止、删除和日志接口。

### 10.1 创建接口

`POST /task/k8s`，请求类型为 `multipart/form-data`。

| FormData 字段 | 必填 | 说明 |
| --- | --- | --- |
| `name` | 是 | 任务名称 |
| `description` | 否 | 任务描述 |
| `cluster_name` | 是 | 集群名称 |
| `cluster_version` | 是 | Kubernetes 版本，如 `v1.30.2` |
| `deployment_mode` | 是 | `1=单节点`、`2=集群` |
| `master_host_ids` | 是 | JSON 数组字符串，如 `[10,11]` |
| `worker_host_ids` | 否 | JSON 数组字符串 |
| `etcd_host_ids` | 是 | JSON 数组字符串 |
| `enabled_components` | 否 | JSON 字符串数组，空值默认 `calico,coredns` |
| `private_registry` | 否 | 私有镜像仓库地址 |
| `registry_username` | 否 | 私有仓库用户名 |
| `registry_password` | 否 | 私有仓库密码 |

```ts
const formData = new FormData()
formData.append('name', form.name.trim())
formData.append('description', form.description || '')
formData.append('cluster_name', form.clusterName.trim())
formData.append('cluster_version', form.clusterVersion)
formData.append('deployment_mode', String(form.deploymentMode))
formData.append('master_host_ids', JSON.stringify(form.masterHostIds))
formData.append('worker_host_ids', JSON.stringify(form.workerHostIds || []))
formData.append('etcd_host_ids', JSON.stringify(form.etcdHostIds))
if (form.enabledComponents?.length) {
  formData.append('enabled_components', JSON.stringify(form.enabledComponents))
}
formData.append('private_registry', form.privateRegistry || '')
formData.append('registry_username', form.registryUsername || '')
formData.append('registry_password', form.registryPassword || '')

await api.post('/task/k8s', formData)
```

创建成功的任务类型为 `3`。当前成功响应在统一 `data` 内又嵌套了一层 `data`：

```json
{
  "code": 200,
  "message": "成功",
  "data": {
    "data": {
      "id": 20,
      "name": "生产 K8s 集群",
      "type": 3,
      "status": 1
    }
  }
}
```

前端校验建议：

- Master 和 ETCD 至少各选择一台主机
- 单节点模式可以允许同一主机同时属于 Master、ETCD 和 Worker
- 集群模式下提示用户确认节点规划，是否允许角色重叠由业务规则决定
- 仅在填写了私有仓库地址时显示用户名和密码
- 密码字段禁止回显、禁止写入浏览器持久化存储

后端运行环境必须配置 `TASK_K8S_REPOSITORY`。未配置时前端会收到“未配置 TASK_K8S_REPOSITORY”。

---

## 11. Ansible 实时日志

推荐优先使用 SSE，若项目已有统一 WebSocket 管理器也可以使用 WebSocket。

### 11.1 SSE

```text
GET /task/ansible/:id/log/:work_id?token=<token>
```

```ts
function openAnsibleLog(taskId: number, workId: number) {
  const token = encodeURIComponent(getToken())
  const url = `/api/v1/task/ansible/${taskId}/log/${workId}?token=${token}`
  const source = new EventSource(url)

  source.onmessage = (event) => {
    appendLog(event.data)
  }

  source.addEventListener('complete', (event) => {
    appendSystemMessage((event as MessageEvent).data)
    source.close()
    refreshTaskDetail()
  })

  source.addEventListener('error', (event) => {
    // 后端可能发送命名 error 事件，浏览器断线也会进入这里。
    handleLogError(event)
  })

  return () => source.close()
}
```

SSE 数据格式：

```text
data: TASK [check nginx] ********

data: ok: [10.0.0.10]

event: complete
data: Task completed with status 3
```

连接期间后端会发送 `: heartbeat` 注释作为心跳，浏览器会自动忽略。组件卸载、切换子任务或关闭日志抽屉时必须主动 `close()`。

### 11.2 WebSocket

```text
GET /ws/task/ansible/:id/log/:work_id?token=<token>
```

```ts
export interface LogMessage {
  type: 'log' | 'status' | 'complete' | 'error'
  content: string
  line_num: number
  timestamp: string
  task_id: number
  work_id: number
}

function openAnsibleLogWS(taskId: number, workId: number) {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const token = encodeURIComponent(getToken())
  const url = `${protocol}//${location.host}/api/v1/ws/task/ansible/${taskId}/log/${workId}?token=${token}`
  const socket = new WebSocket(url)

  socket.onmessage = (event) => {
    const message = JSON.parse(event.data) as LogMessage
    if (message.type === 'log') appendLog(message.content)
    if (message.type === 'error') showError(message.content)
    if (message.type === 'complete') {
      socket.close()
      refreshTaskDetail()
    }
  }

  return () => socket.close()
}
```

通过反向代理部署时，需要同时为 WebSocket 路径开启 Upgrade/Connection 转发，并关闭 SSE 路径的代理缓冲，否则日志可能不能实时显示。

---

## 12. 按钮显示规则

### 12.1 普通和定时任务

| 条件 | 推荐操作 |
| --- | --- |
| 立即任务，状态不是运行中 | 执行或重新执行、编辑、删除 |
| 立即任务，状态为运行中 | 查看状态、查看日志、停止 |
| 定时任务，状态为等待中 | 启用调度、编辑、删除 |
| 定时任务，状态为运行中且 `can_pause=true` | 暂停、查看执行信息 |
| 定时任务，状态为已暂停且 `can_resume=true` | 恢复、编辑、删除 |
| 状态为成功/异常/已停止 | 查看日志、重新执行；是否编辑/删除以接口结果为准 |

停止普通任务时需要选择一个模板并传 `templateId`。后端会同时把该父任务下等待中或运行中的其他子任务置为停止状态。

### 12.2 Ansible/K8s 任务

| 条件 | 推荐操作 |
| --- | --- |
| 父任务状态为等待、成功或异常 | 启动、详情、删除 |
| 父任务状态为运行中 | 详情，不显示删除和重复启动 |
| 子任务状态为运行中 | 实时日志、停止 |
| 子任务状态为成功或异常 | 查看完整日志 |

所有写操作完成后都应重新请求后端数据，不要只在前端本地修改状态。

---

## 13. 前端实现注意事项

1. 普通任务的 `shell` 必须传模板 ID 字符串，不是脚本内容。
2. `host_ids` 必须传英文逗号分隔的正整数，不能传 JSON 数组。
3. Ansible 的 `hostGroups`、`variables` 和 K8s 的主机数组必须先 `JSON.stringify` 后放入 FormData。
4. 创建普通/定时任务后不会自动启动；“保存并执行”必须串行调用创建和启动两个接口。
5. 定时任务状态 `2` 表示调度已启用，不能直接理解为脚本一直运行。
6. 普通接口即使 HTTP 200 也可能失败，必须检查业务 `code`。
7. Ansible 列表、启动、停止、删除接口是直接 JSON，需要单独兼容 HTTP 状态和 `error` 字段。
8. 模板内容接口返回纯文本，不要用统一 JSON 解包器处理。
9. 日志内容必须按纯文本渲染，禁止使用 `v-html` 或 `dangerouslySetInnerHTML`，避免日志中的内容形成 XSS。
10. 表单提交期间禁用提交按钮，Git/K8s 创建操作尤其要防止重复提交。
11. 所有轮询、SSE 和 WebSocket 连接在页面离开时都必须释放。
12. 时间统一按后端返回值解析并以用户本地时区展示；不要截断 ISO 时间字符串。

---

## 14. 推荐开发顺序

1. 完成枚举、TypeScript 类型、统一请求封装和 Ansible 特殊响应适配。
2. 完成模板库，以便后续任务创建可以选择模板。
3. 接入 CMDB 主机选择器，实现普通任务创建和列表。
4. 实现立即执行、状态轮询、日志和重新执行。
5. 实现 Cron 校验、定时任务启用、暂停和恢复。
6. 实现 Ansible 手动上传、Git 导入、详情和 SSE 日志。
7. 实现 K8s 部署表单。
8. 最后实现管理员运行监控页面。

以上顺序可以保证每一阶段都有可独立手工验证的完整业务闭环。
