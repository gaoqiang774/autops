<script setup>
import { computed, nextTick, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Bell,
  ChatDotRound,
  CircleCheck,
  Clock,
  Connection,
  Cpu,
  DataAnalysis,
  DocumentChecked,
  Files,
  Monitor,
  Operation,
  Plus,
  Promotion,
  Refresh,
  Search,
  Setting,
  Tickets,
  Warning
} from '@element-plus/icons-vue'

const queryForm = reactive({
  datasource: 'prod-prometheus',
  cluster: 'prod-k8s-a',
  namespace: 'aiops',
  metric: 'container_cpu_usage_seconds_total',
  promql: 'sum(rate(container_cpu_usage_seconds_total{namespace="aiops"}[5m])) by (pod)',
  range: '30m',
  step: '30s',
  alertLevel: 'warning',
  autoAnalyze: true
})

const scrapeForm = reactive({
  jobName: 'k8s-node-exporter',
  targetGroup: '生产集群节点',
  interval: '15s',
  timeout: '10s',
  relabel: true,
  tlsVerify: true,
  authType: 'token'
})

const activeTab = ref('form')
const dashboardTab = ref('monitor')
const taskView = ref('history')
const ruleDialogVisible = ref(false)
const chatDialogVisible = ref(false)
const resourceDialogVisible = ref(false)
const resourceFormRef = ref()
const selectedTasks = ref([])

const taskFilters = reactive({
  keyword: '',
  executionType: '',
  targetType: '',
  result: '',
  source: '',
  risk: ''
})

const taskDispatchForm = reactive({
  name: 'Ansible Playbook',
  executionType: 'Ansible Playbook',
  executionMethod: 'Ansible 分发',
  description: '执行结构化 Playbook，适合固化编排流程。',
  fileName: 'service-health.yml',
  timeout: 60,
  script: `- hosts: targets
  gather_facts: false
  tasks:
    - name: check app process
      shell: ps -ef | grep myapp | grep -v grep
      changed_when: false
    - name: tail recent log
      shell: journalctl -u myapp -n 50 --no-pager
      changed_when: false`
})

const templateFilters = reactive({
  keyword: '',
  executionType: ''
})

const resourceForm = reactive({
  resourceType: 'host',
  status: 'available',
  environment: 'supply-chain-test',
  system: '',
  k8sCluster: '',
  name: '',
  ip: '',
  sshPort: 22,
  sshUser: 'root',
  sshPassword: '',
  description: ''
})

const resourceRules = {
  resourceType: [{ required: true, message: '请选择资源类型', trigger: 'change' }],
  status: [{ required: true, message: '请选择状态', trigger: 'change' }],
  environment: [{ required: true, message: '请选择环境', trigger: 'change' }],
  k8sCluster: [{ required: true, message: '请选择 K8s 集群', trigger: 'change' }],
  name: [{ required: true, message: '请输入资源名称', trigger: 'blur' }],
  ip: [{ required: true, message: '请输入 IP 地址', trigger: 'blur' }]
}

const ruleForm = reactive({
  name: 'Pod 重启次数异常',
  objectType: 'K8S 工作负载',
  cluster: 'prod-k8s-a',
  severity: 'critical',
  duration: '5m',
  owner: 'SRE 一线值班',
  notify: ['企业微信', '短信'],
  expr: 'increase(kube_pod_container_status_restarts_total[15m]) > 3',
  autoRemediation: true
})

const chatForm = reactive({
  message: '分析 prod-k8s-a 最近 30 分钟 CPU 峰值，并给出处置建议'
})

const healthCards = [
  { label: '数据源在线', value: '4/5', tone: 'success', meta: '1 个边缘环境延迟偏高' },
  { label: '采集目标', value: '128', tone: 'primary', meta: '12 个新增目标待确认' },
  { label: '活跃告警', value: '17', tone: 'danger', meta: '3 个 P1 需要处理' },
  { label: 'AI 建议', value: '6', tone: 'warning', meta: '可自动生成修复步骤' }
]

const targets = [
  { name: 'node-exporter-prod-01', endpoint: '10.12.4.21:9100', type: '主机', status: 'up', latency: '18ms', cluster: 'prod-k8s-a' },
  { name: 'kube-state-metrics', endpoint: '10.12.8.33:8080', type: 'K8S', status: 'up', latency: '24ms', cluster: 'prod-k8s-a' },
  { name: 'snmp-core-switch-01', endpoint: '10.10.1.2:9116', type: '网络设备', status: 'warn', latency: '86ms', cluster: 'core-network' },
  { name: 'mysql-exporter-billing', endpoint: '10.21.3.18:9104', type: '数据库', status: 'down', latency: '-', cluster: 'prod-db' }
]

const alertRules = [
  { name: '节点 CPU 持续高负载', level: 'warning', expr: 'avg(rate(node_cpu_seconds_total{mode!="idle"}[5m])) > 0.82', duration: '10m' },
  { name: 'Pod 重启次数异常', level: 'critical', expr: 'increase(kube_pod_container_status_restarts_total[15m]) > 3', duration: '5m' },
  { name: '核心交换机丢包率升高', level: 'warning', expr: 'rate(ifHCInErrors[5m]) > 0.03', duration: '8m' }
]

const aiFindings = [
  'prod-k8s-a 集群 CPU 峰值与发布窗口重合，建议优先核对 deployment rollout 记录。',
  'snmp-core-switch-01 延迟抖动集中在上联端口，建议拉取接口错误包和光模块功率。',
  'billing 数据库 exporter 已断联，最近一次成功采集在 12 分钟前。'
]

const chatMessages = [
  {
    role: 'user',
    name: '平台管理员',
    text: 'prod-k8s-a 的 CPU 峰值为什么突然升高？'
  },
  {
    role: 'assistant',
    name: 'AI 运维助手',
    text: '峰值与 01:12 的 deployment rollout 时间重合。建议先查看 aiops 命名空间下最近发布的 workload，再对比 Pod 重启和 HPA 扩缩容事件。'
  },
  {
    role: 'assistant',
    name: 'AI 运维助手',
    text: '可执行查询：sum(rate(container_cpu_usage_seconds_total{namespace="aiops"}[5m])) by (pod)，并按 pod 排序定位热点实例。'
  }
]

const chatSessions = [
  '帮我分析下告警id 632 api-gateway pod CPU usage is elevated',
  '分析下电商测试环境订单服务最近一次异常',
  '分析下电商测试环境 k8s 集群的异常趋势',
  '帮我在电商测试环境得 k8s 集群里安装 exporter',
  '帮我在个人测试环境装个 helm 命令行',
  '帮我分析下电商测试环境网关服务最近的告警',
  '帮我分析下电商测试环境最近 1 小时订单异常',
  '当前有哪些活跃告警',
  '分析电商测试环境 K8S 集群异常',
  '帮我生成 PromQL 查询 CPU 峰值',
  '排查 node-exporter 采集中断',
  '查看最近一次发布影响'
]

const statusMap = {
  up: { label: '正常', type: 'success' },
  warn: { label: '关注', type: 'warning' },
  down: { label: '中断', type: 'danger' }
}

const taskStats = [
  { label: '全部任务', value: 38, tone: 'primary' },
  { label: '人工创建', value: 16, tone: 'neutral' },
  { label: 'AIOps 生成', value: 19, tone: 'success' },
  { label: '计划任务', value: 3, tone: 'warning' }
]

const taskRows = [
  { id: 1001, name: '安装 Docker 脚本任务', targetType: '主机资源', source: 'AIOps 生成', risk: '极高', environment: '电商测试环境', executionType: 'Shell 脚本', executor: 'admin', targetCount: 1, result: 'failed', completedAt: '2026-06-22 22:02:06' },
  { id: 1002, name: 'K8s 部署 xxl-job-admin', targetType: 'K8S 资源', source: 'AIOps 生成', risk: '高', environment: '电商测试环境', executionType: 'K8S 脚本', executor: 'admin', targetCount: 1, result: 'failed', completedAt: '2026-06-21 21:39:39' },
  { id: 1003, name: '安装 Helm 脚本任务', targetType: '主机资源', source: 'AIOps 生成', risk: '极高', environment: '个人测试环境', executionType: 'Shell 脚本', executor: 'admin', targetCount: 1, result: 'success', completedAt: '2026-06-21 19:04:42' },
  { id: 1004, name: '安装 mysql', targetType: '主机资源', source: 'AIOps 生成', risk: '中', environment: '个人测试环境', executionType: 'Shell 脚本', executor: 'admin', targetCount: 1, result: 'success', completedAt: '2026-06-20 21:18:50' },
  { id: 1005, name: '安装 Docker 脚本任务', targetType: '主机资源', source: 'AIOps 生成', risk: '中', environment: '个人测试环境', executionType: 'Shell 脚本', executor: 'admin', targetCount: 1, result: 'success', completedAt: '2026-06-20 19:05:41' },
  { id: 1006, name: 'docker 服务状态巡检', targetType: '主机资源', source: 'AIOps 生成', risk: '低', environment: '个人测试环境', executionType: 'Shell 脚本', executor: 'admin', targetCount: 1, result: 'success', completedAt: '2026-06-20 16:19:01' },
  { id: 1007, name: '给 loki-minio 加 ingress', targetType: '主机资源', source: 'AIOps 生成', risk: '低', environment: '电商测试环境', executionType: 'Shell 脚本', executor: 'admin', targetCount: 1, result: 'failed', completedAt: '2026-06-14 21:44:44' },
  { id: 1008, name: '给 prometheus 的 svc 暴露成 nodeport', targetType: 'K8S 资源', source: 'AIOps 生成', risk: '高', environment: '电商测试环境', executionType: 'K8S 脚本', executor: 'admin', targetCount: 1, result: 'success', completedAt: '2026-06-14 19:54:37' },
  { id: 1009, name: '修改 monitoring 命名空间服务配置', targetType: 'K8S 资源', source: '人工创建', risk: '低', environment: '电商测试环境', executionType: 'Shell 脚本', executor: 'admin', targetCount: 1, result: 'failed', completedAt: '2026-06-13 13:28:16' },
  { id: 1010, name: 'K8S 集群巡检任务', targetType: 'K8S 资源', source: '计划任务', risk: '中', environment: '电商测试环境', executionType: 'Ansible Playbook', executor: 'admin', targetCount: 1, result: 'failed', completedAt: '2026-06-12 12:43:53' }
]

const dispatchTypes = [
  {
    value: 'Shell 脚本',
    title: 'Shell 脚本',
    description: '在主机资源上执行 Shell 脚本。',
    method: 'SSH',
    fileName: 'service-health.sh',
    language: 'SHELL',
    script: '#!/bin/bash\nsystemctl status myapp\njournalctl -u myapp -n 50 --no-pager'
  },
  {
    value: 'Python 脚本',
    title: 'Python 脚本',
    description: '通过 Python 解释器执行诊断、巡检或自动化脚本。',
    method: 'SSH',
    fileName: 'service_health.py',
    language: 'PYTHON',
    script: 'import subprocess\n\nsubprocess.run(["systemctl", "status", "myapp"], check=False)'
  },
  {
    value: 'Ansible Playbook',
    title: 'Ansible Playbook',
    description: '执行结构化 Playbook，适合固化编排流程。',
    method: 'Ansible 分发',
    fileName: 'service-health.yml',
    language: 'YAML',
    script: `- hosts: targets
  gather_facts: false
  tasks:
    - name: check app process
      shell: ps -ef | grep myapp | grep -v grep
      changed_when: false
    - name: tail recent log
      shell: journalctl -u myapp -n 50 --no-pager
      changed_when: false`
  },
  {
    value: 'K8S 脚本',
    title: 'K8S 脚本',
    description: '在目标集群执行 kubectl/Helm 任务，适合 K8S 资源变更。',
    method: 'K8S API',
    fileName: 'workload-check.sh',
    language: 'SHELL',
    script: 'kubectl get nodes\nkubectl get pods -A --field-selector=status.phase!=Running'
  }
]

const templateRows = [
  { id: 1, name: 'Ansible 批量基线采集', type: 'Shell 脚本', description: '通过 Ansible 统一下发基线采集命令，适合变更前存档', method: 'Ansible', policy: '失败继续', timeout: '30s', source: '内置' },
  { id: 2, name: 'Nginx 配置一致性 Playbook', type: 'Ansible Playbook', description: '通过 Playbook 统一校验 Nginx 配置、语法与端口监听状态', method: 'Ansible', policy: '失败即停', timeout: '45s', source: '内置' },
  { id: 3, name: '常用服务巡检', type: 'Shell 脚本', description: '适合检查 nginx / docker / sshd 等基础服务状态', method: 'SSH', policy: '失败继续', timeout: '15s', source: '内置' },
  { id: 4, name: '生产 SSH 巡检', type: 'Shell 脚本', description: '批量校验 SSH 登录能力与基础连通性', method: 'SSH', policy: '失败继续', timeout: '15s', source: '内置' }
]

const activeDispatchType = computed(() => {
  return dispatchTypes.find(item => item.value === taskDispatchForm.executionType) || dispatchTypes[0]
})

const filteredTemplateRows = computed(() => {
  const keyword = templateFilters.keyword.trim().toLowerCase()
  return templateRows.filter(item => {
    const matchesKeyword = !keyword || `${item.name} ${item.description}`.toLowerCase().includes(keyword)
    return matchesKeyword && (!templateFilters.executionType || item.type === templateFilters.executionType)
  })
})

const filteredTaskRows = computed(() => {
  const keyword = taskFilters.keyword.trim().toLowerCase()
  return taskRows.filter(task => {
    const matchesKeyword = !keyword || `${task.name} ${task.executor}`.toLowerCase().includes(keyword)
    return matchesKeyword &&
      (!taskFilters.executionType || task.executionType === taskFilters.executionType) &&
      (!taskFilters.targetType || task.targetType === taskFilters.targetType) &&
      (!taskFilters.result || task.result === taskFilters.result) &&
      (!taskFilters.source || task.source === taskFilters.source) &&
      (!taskFilters.risk || task.risk === taskFilters.risk)
  })
})

const resetTaskFilters = () => {
  Object.assign(taskFilters, {
    keyword: '',
    executionType: '',
    targetType: '',
    result: '',
    source: '',
    risk: ''
  })
}

const selectDispatchType = type => {
  if (!type) return
  taskDispatchForm.name = type.title
  taskDispatchForm.executionType = type.value
  taskDispatchForm.executionMethod = type.method
  taskDispatchForm.description = type.description
  taskDispatchForm.fileName = type.fileName
  taskDispatchForm.script = type.script
}

const resetTemplateFilters = () => {
  templateFilters.keyword = ''
  templateFilters.executionType = ''
}

const applyTaskTemplate = template => {
  const type = dispatchTypes.find(item => item.value === template.type) || dispatchTypes[0]
  selectDispatchType(type)
  taskDispatchForm.name = template.name
  taskDispatchForm.description = template.description
  taskDispatchForm.timeout = Number.parseInt(template.timeout, 10) || 60
  taskView.value = 'dispatch'
}

const openResourceDialog = () => {
  Object.assign(resourceForm, {
    resourceType: 'host',
    status: 'available',
    environment: 'supply-chain-test',
    system: '',
    k8sCluster: '',
    name: '',
    ip: '',
    sshPort: 22,
    sshUser: 'root',
    sshPassword: '',
    description: ''
  })
  resourceDialogVisible.value = true
}

const handleResourceTypeChange = async type => {
  resourceForm.name = ''
  resourceForm.k8sCluster = ''
  if (type === 'host') {
    resourceForm.ip = ''
    resourceForm.sshPort = 22
    resourceForm.sshUser = 'root'
    resourceForm.sshPassword = ''
  }
  await nextTick()
  resourceFormRef.value?.clearValidate()
}

const handleK8sClusterChange = cluster => {
  const clusterNames = {
    'supply-chain-test-k8s': '供应链测试环境 K8s 集群',
    'ecommerce-test-k8s': '电商测试环境 K8s 集群',
    'personal-test-k8s': '个人测试环境 K8s 集群'
  }
  resourceForm.name = clusterNames[cluster] || ''
}

const saveResource = async () => {
  const valid = await resourceFormRef.value?.validate().catch(() => false)
  if (!valid) return
  resourceDialogVisible.value = false
  ElMessage.success('执行资源保存成功')
}
</script>

<template>
  <div class="prometheus-demo-page">
    <nav class="dashboard-view-tabs" aria-label="工作台切换">
      <el-segmented
        v-model="dashboardTab"
        :options="[
          { label: '监控工作台', value: 'monitor' },
          { label: '任务工作台', value: 'tasks' }
        ]"
      />
    </nav>

    <div v-if="dashboardTab === 'monitor'" class="monitor-workbench">
      <section class="hero-panel">
      <div class="hero-copy">
        <div class="hero-kicker">
          <el-icon><Monitor /></el-icon>
          Prometheus 运维控制台 Demo
        </div>
        <h1>高密度监控配置与 AI 分析参考页</h1>
        <p>
          面向主机、K8S、网络设备和告警处理的日常工作台。重点展示表单、状态、查询、规则和智能建议的统一视觉语言。
        </p>
      </div>
      <div class="hero-actions">
        <el-button type="primary" :icon="Search">执行查询</el-button>
        <el-button :icon="Plus" @click="ruleDialogVisible = true">新建规则</el-button>
        <el-button :icon="ChatDotRound" @click="chatDialogVisible = true">AI 助手</el-button>
        <el-button :icon="Refresh">刷新目标</el-button>
      </div>
      </section>

      <section class="health-grid">
        <article v-for="item in healthCards" :key="item.label" class="health-card" :class="`is-${item.tone}`">
          <span>{{ item.label }}</span>
          <strong>{{ item.value }}</strong>
          <p>{{ item.meta }}</p>
        </article>
      </section>

      <section class="workspace-grid">
      <div class="main-column">
        <el-card class="ops-card form-card" shadow="never">
          <template #header>
            <div class="card-header">
              <div>
                <h2>监控查询表单</h2>
                <p>高频运维操作区，优先保证字段清晰、扫描效率和错误容忍。</p>
              </div>
              <el-segmented v-model="activeTab" :options="[
                { label: '查询', value: 'form' },
                { label: '采集', value: 'scrape' },
                { label: '规则', value: 'rule' }
              ]" />
            </div>
          </template>

          <el-form v-if="activeTab === 'form'" :model="queryForm" label-position="top" class="ops-form">
            <div class="form-grid">
              <el-form-item label="数据源">
                <el-select v-model="queryForm.datasource">
                  <el-option label="生产 Prometheus" value="prod-prometheus" />
                  <el-option label="测试 Prometheus" value="test-prometheus" />
                  <el-option label="边缘 Prometheus" value="edge-prometheus" />
                </el-select>
              </el-form-item>
              <el-form-item label="集群">
                <el-select v-model="queryForm.cluster">
                  <el-option label="prod-k8s-a" value="prod-k8s-a" />
                  <el-option label="prod-k8s-b" value="prod-k8s-b" />
                  <el-option label="core-network" value="core-network" />
                </el-select>
              </el-form-item>
              <el-form-item label="命名空间">
                <el-input v-model="queryForm.namespace" placeholder="namespace" />
              </el-form-item>
              <el-form-item label="指标">
                <el-input v-model="queryForm.metric" placeholder="metric name" />
              </el-form-item>
              <el-form-item label="时间范围">
                <el-radio-group v-model="queryForm.range">
                  <el-radio-button label="15m" />
                  <el-radio-button label="30m" />
                  <el-radio-button label="1h" />
                  <el-radio-button label="6h" />
                </el-radio-group>
              </el-form-item>
              <el-form-item label="步长">
                <el-select v-model="queryForm.step">
                  <el-option label="15s" value="15s" />
                  <el-option label="30s" value="30s" />
                  <el-option label="1m" value="1m" />
                </el-select>
              </el-form-item>
            </div>
            <el-form-item label="PromQL">
              <el-input v-model="queryForm.promql" type="textarea" :rows="4" />
            </el-form-item>
            <div class="form-footer">
              <el-checkbox v-model="queryForm.autoAnalyze">查询后自动生成 AI 分析</el-checkbox>
              <div>
                <el-button>保存模板</el-button>
                <el-button type="primary">查询并分析</el-button>
              </div>
            </div>
          </el-form>

          <el-form v-else-if="activeTab === 'scrape'" :model="scrapeForm" label-position="top" class="ops-form">
            <div class="form-grid">
              <el-form-item label="采集任务">
                <el-input v-model="scrapeForm.jobName" />
              </el-form-item>
              <el-form-item label="目标分组">
                <el-input v-model="scrapeForm.targetGroup" />
              </el-form-item>
              <el-form-item label="采集间隔">
                <el-select v-model="scrapeForm.interval">
                  <el-option label="10s" value="10s" />
                  <el-option label="15s" value="15s" />
                  <el-option label="30s" value="30s" />
                </el-select>
              </el-form-item>
              <el-form-item label="超时">
                <el-select v-model="scrapeForm.timeout">
                  <el-option label="5s" value="5s" />
                  <el-option label="10s" value="10s" />
                  <el-option label="20s" value="20s" />
                </el-select>
              </el-form-item>
              <el-form-item label="认证方式">
                <el-select v-model="scrapeForm.authType">
                  <el-option label="Bearer Token" value="token" />
                  <el-option label="Basic Auth" value="basic" />
                  <el-option label="无认证" value="none" />
                </el-select>
              </el-form-item>
              <el-form-item label="采集策略">
                <div class="switch-stack">
                  <el-switch v-model="scrapeForm.relabel" active-text="启用 relabel" />
                  <el-switch v-model="scrapeForm.tlsVerify" active-text="TLS 校验" />
                </div>
              </el-form-item>
            </div>
          </el-form>

          <div v-else class="rule-list">
            <article v-for="rule in alertRules" :key="rule.name" class="rule-item">
              <div>
                <h3>{{ rule.name }}</h3>
                <code>{{ rule.expr }}</code>
              </div>
              <div class="rule-meta">
                <el-tag :type="rule.level === 'critical' ? 'danger' : 'warning'" effect="light">
                  {{ rule.level }}
                </el-tag>
                <span>{{ rule.duration }}</span>
              </div>
            </article>
          </div>
        </el-card>

        <el-card class="ops-card table-card" shadow="never">
          <template #header>
            <div class="card-header compact">
              <div>
                <h2>采集目标状态</h2>
                <p>表格保留紧凑密度，状态色只表达真实语义。</p>
              </div>
              <el-button size="small" :icon="Operation">批量操作</el-button>
            </div>
          </template>
          <el-table :data="targets" class="target-table">
            <el-table-column prop="name" label="目标名称" min-width="190" />
            <el-table-column prop="endpoint" label="Endpoint" min-width="150" />
            <el-table-column prop="type" label="类型" width="100" />
            <el-table-column prop="cluster" label="归属" width="130" />
            <el-table-column prop="latency" label="延迟" width="90" />
            <el-table-column label="状态" width="90">
              <template #default="{ row }">
                <el-tag :type="statusMap[row.status].type" effect="light">
                  {{ statusMap[row.status].label }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </div>

      <aside class="side-column">
        <el-card class="ops-card ai-card" shadow="never">
          <template #header>
            <div class="side-title">
              <el-icon><DataAnalysis /></el-icon>
              AI 诊断摘要
            </div>
          </template>
          <div class="ai-score">
            <span>风险评分</span>
            <strong>72</strong>
            <em>需要关注</em>
          </div>
          <ul class="ai-list">
            <li v-for="finding in aiFindings" :key="finding">{{ finding }}</li>
          </ul>
          <el-button class="full-button" type="primary" plain>生成处置步骤</el-button>
        </el-card>

        <el-card class="ops-card quick-card" shadow="never">
          <template #header>
            <div class="side-title">
              <el-icon><Setting /></el-icon>
              运维对象覆盖
            </div>
          </template>
          <div class="object-grid">
            <div>
              <el-icon><Cpu /></el-icon>
              <span>主机</span>
              <strong>64</strong>
            </div>
            <div>
              <el-icon><Connection /></el-icon>
              <span>K8S</span>
              <strong>12</strong>
            </div>
            <div>
              <el-icon><Warning /></el-icon>
              <span>网络</span>
              <strong>18</strong>
            </div>
            <div>
              <el-icon><Bell /></el-icon>
              <span>告警</span>
              <strong>17</strong>
            </div>
          </div>
        </el-card>

        <el-card class="ops-card checklist-card" shadow="never">
          <template #header>
            <div class="side-title">
              <el-icon><DocumentChecked /></el-icon>
              发布前检查
            </div>
          </template>
          <el-timeline>
            <el-timeline-item type="success" timestamp="已完成">数据源连通性校验</el-timeline-item>
            <el-timeline-item type="warning" timestamp="待确认">网络设备 SNMP 权限</el-timeline-item>
            <el-timeline-item type="primary" timestamp="进行中">告警规则去重</el-timeline-item>
          </el-timeline>
        </el-card>
      </aside>
      </section>
    </div>

    <section v-else class="task-workbench">
      <header class="task-hero">
        <span class="task-hero-icon"><el-icon><Operation /></el-icon></span>
        <div>
          <h1>任务工作台</h1>
          <p>集中处理任务下发、AIOps 建议联动、模板复用与执行回溯，提供直接的控制台操作入口。</p>
        </div>
      </header>

      <nav class="task-section-tabs" aria-label="任务工作台视图">
        <button :class="{ active: taskView === 'history' }" type="button" @click="taskView = 'history'">
          <el-icon><Clock /></el-icon>
          任务历史
        </button>
        <button :class="{ active: taskView === 'dispatch' }" type="button" @click="taskView = 'dispatch'">
          <el-icon><Promotion /></el-icon>
          任务下发
        </button>
        <button :class="{ active: taskView === 'templates' }" type="button" @click="taskView = 'templates'">
          <el-icon><Files /></el-icon>
          模板库
        </button>
      </nav>

      <template v-if="taskView === 'history'">
        <section class="task-stat-grid">
          <article v-for="item in taskStats" :key="item.label" :class="`is-${item.tone}`">
            <span>{{ item.label }}</span>
            <strong>{{ item.value }}</strong>
          </article>
        </section>

        <section class="task-history-panel">
          <header class="task-panel-header">
            <h2>任务历史</h2>
            <div class="task-panel-actions">
              <span>已选 {{ selectedTasks.length }} 个任务</span>
              <el-button size="small" :disabled="selectedTasks.length === 0">批量终止</el-button>
              <el-button size="small" @click="resetTaskFilters">重置筛选</el-button>
            </div>
          </header>

          <div class="task-filterbar">
            <el-input v-model="taskFilters.keyword" :prefix-icon="Search" clearable placeholder="搜索任务名称 / 执行人" />
            <el-select v-model="taskFilters.executionType" clearable placeholder="执行类型">
              <el-option label="Shell 脚本" value="Shell 脚本" />
              <el-option label="K8S 脚本" value="K8S 脚本" />
              <el-option label="Ansible Playbook" value="Ansible Playbook" />
            </el-select>
            <el-select v-model="taskFilters.targetType" clearable placeholder="目标类型">
              <el-option label="主机资源" value="主机资源" />
              <el-option label="K8S 资源" value="K8S 资源" />
            </el-select>
            <el-select v-model="taskFilters.result" clearable placeholder="执行结果">
              <el-option label="执行成功" value="success" />
              <el-option label="执行失败" value="failed" />
            </el-select>
            <el-select v-model="taskFilters.source" clearable placeholder="触发来源">
              <el-option label="人工创建" value="人工创建" />
              <el-option label="AIOps 生成" value="AIOps 生成" />
              <el-option label="计划任务" value="计划任务" />
            </el-select>
            <el-select v-model="taskFilters.risk" clearable placeholder="风险">
              <el-option v-for="risk in ['低', '中', '高', '极高']" :key="risk" :label="risk" :value="risk" />
            </el-select>
          </div>

          <el-table
            :data="filteredTaskRows"
            class="task-table"
            row-key="id"
            @selection-change="selectedTasks = $event"
          >
            <el-table-column type="selection" width="42" />
            <el-table-column label="任务名称" min-width="310">
              <template #default="{ row }">
                <div class="task-name-cell">
                  <strong>{{ row.name }}</strong>
                  <span>{{ row.targetType }} · {{ row.source }} · {{ row.risk }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column prop="environment" label="环境" min-width="130" />
            <el-table-column prop="executionType" label="执行类型" width="130" />
            <el-table-column prop="executor" label="执行人" width="90" />
            <el-table-column prop="targetCount" label="目标数" width="78" />
            <el-table-column label="执行结果" width="105">
              <template #default="{ row }">
                <el-tag :type="row.result === 'success' ? 'success' : 'danger'" effect="light">
                  {{ row.result === 'success' ? '执行成功' : '执行失败' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="completedAt" label="完成时间" width="165" />
            <el-table-column label="操作" width="165" fixed="right">
              <template #default>
                <el-button link type="primary">详情</el-button>
                <el-button link type="warning">重跑</el-button>
                <el-dropdown trigger="click">
                  <el-button link>更多</el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item>复制任务</el-dropdown-item>
                      <el-dropdown-item>查看日志</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </template>
            </el-table-column>
          </el-table>
        </section>
      </template>

      <section v-else-if="taskView === 'dispatch'" class="task-dispatch-layout">
        <aside class="dispatch-type-panel">
          <header>
            <h2>执行类型</h2>
            <el-tag size="small" type="info" effect="light">{{ taskDispatchForm.executionType }}</el-tag>
          </header>
          <button
            v-for="item in dispatchTypes"
            :key="item.value"
            type="button"
            :class="{ active: taskDispatchForm.executionType === item.value }"
            @click="selectDispatchType(item)"
          >
            <strong>{{ item.title }}</strong>
            <span>{{ item.description }}</span>
          </button>
        </aside>

        <section class="task-editor-panel">
          <header class="task-editor-header">
            <h2>新建任务</h2>
            <div>
              <el-tag size="small" type="info" effect="light">已选目标 0 个</el-tag>
              <el-button size="small">保存为模板</el-button>
              <el-button size="small" type="primary" :icon="Promotion" disabled>立即执行</el-button>
            </div>
          </header>

          <div class="dispatch-notice">
            任务中心适合批量巡检、统一刷新与命令分发；当前为串行执行，建议单次控制在 20 台以内。
          </div>

          <el-form :model="taskDispatchForm" label-position="left" label-width="88px" class="task-dispatch-form">
            <el-form-item label="任务名称">
              <el-input v-model="taskDispatchForm.name" />
            </el-form-item>

            <div class="task-form-grid">
              <el-form-item label="执行类型">
                <el-select v-model="taskDispatchForm.executionType" @change="selectDispatchType(dispatchTypes.find(item => item.value === $event))">
                  <el-option v-for="item in dispatchTypes" :key="item.value" :label="item.title" :value="item.value" />
                </el-select>
              </el-form-item>
              <el-form-item label="执行方式">
                <el-select v-model="taskDispatchForm.executionMethod" disabled>
                  <el-option :label="taskDispatchForm.executionMethod" :value="taskDispatchForm.executionMethod" />
                </el-select>
              </el-form-item>
            </div>

            <el-form-item label="任务说明">
              <el-input v-model="taskDispatchForm.description" />
            </el-form-item>

            <div class="dispatch-method-note">
              <span>方式说明</span>
              <p>{{ activeDispatchType.title }} 任务仅支持 {{ activeDispatchType.method }} 执行，会自动锁定为 {{ activeDispatchType.method }}。</p>
            </div>

            <div class="task-form-grid">
              <el-form-item label="文件名">
                <el-input v-model="taskDispatchForm.fileName" />
              </el-form-item>
              <el-form-item label="超时(秒)">
                <el-input-number v-model="taskDispatchForm.timeout" :min="10" :max="3600" />
              </el-form-item>
            </div>

            <el-form-item label="执行脚本" class="script-form-item">
              <div class="script-editor">
                <header>
                  <span>{{ activeDispatchType.title }}</span>
                  <strong>{{ activeDispatchType.language }}</strong>
                </header>
                <textarea v-model="taskDispatchForm.script" aria-label="执行脚本" spellcheck="false"></textarea>
              </div>
            </el-form-item>
          </el-form>
        </section>
      </section>

      <section v-else class="task-template-panel">
        <header class="template-panel-header">
          <div>
            <h2>模板库</h2>
            <p>将常用巡检、变更和诊断编排沉淀为模板；任务执行类型只保留 Shell、Python、Ansible Playbook 和 K8S API。</p>
          </div>
          <div>
            <el-button size="small" type="primary" :icon="Plus" @click="openResourceDialog">新增执行资源</el-button>
            <el-button size="small" link type="primary">保存当前配置</el-button>
          </div>
        </header>

        <div class="template-filterbar">
          <el-input v-model="templateFilters.keyword" :prefix-icon="Search" clearable placeholder="搜索模板名称 / 说明" />
          <el-select v-model="templateFilters.executionType" clearable placeholder="执行类型">
            <el-option v-for="item in dispatchTypes" :key="item.value" :label="item.title" :value="item.value" />
          </el-select>
          <el-tag type="info" effect="plain">模板数 {{ filteredTemplateRows.length }}</el-tag>
          <el-button size="small" @click="resetTemplateFilters">重置筛选</el-button>
        </div>

        <el-table :data="filteredTemplateRows" class="template-table" row-key="id">
          <el-table-column prop="name" label="任务名称" min-width="210" />
          <el-table-column label="执行类型" width="130">
            <template #default="{ row }">
              <el-tag size="small" effect="plain">{{ row.type }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="description" label="任务说明" min-width="320" />
          <el-table-column prop="method" label="执行方式" width="120" />
          <el-table-column prop="policy" label="执行策略" width="110" />
          <el-table-column prop="timeout" label="超时(秒)" width="100" />
          <el-table-column label="来源" width="90">
            <template #default="{ row }">
              <el-tag size="small" type="success" effect="light">{{ row.source }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="150" fixed="right">
            <template #default="{ row }">
              <el-button size="small" link>查看详情</el-button>
              <el-button size="small" link type="primary" @click="applyTaskTemplate(row)">套用</el-button>
            </template>
          </el-table-column>
        </el-table>
      </section>
    </section>

    <el-dialog
      v-model="resourceDialogVisible"
      class="resource-dialog"
      width="620px"
      align-center
      destroy-on-close
      modal-class="systemd-form-dialog-overlay"
      title="新增执行资源"
    >
      <el-form
        ref="resourceFormRef"
        :model="resourceForm"
        :rules="resourceRules"
        label-position="right"
        label-width="96px"
        class="resource-form"
      >
        <div class="resource-form-grid">
          <el-form-item label="资源类型" prop="resourceType">
            <el-radio-group v-model="resourceForm.resourceType">
              <el-radio value="host">主机</el-radio>
              <el-radio value="k8s">K8s</el-radio>
            </el-radio-group>
          </el-form-item>

          <el-form-item label="状态" prop="status">
            <el-select v-model="resourceForm.status">
              <el-option label="可用" value="available" />
              <el-option label="停用" value="disabled" />
            </el-select>
          </el-form-item>

          <el-form-item label="环境" prop="environment">
            <el-select v-model="resourceForm.environment">
              <el-option label="供应链测试环境" value="supply-chain-test" />
              <el-option label="电商测试环境" value="ecommerce-test" />
              <el-option label="个人测试环境" value="personal-test" />
              <el-option label="生产环境" value="production" />
            </el-select>
          </el-form-item>

          <el-form-item label="系统">
            <el-select v-model="resourceForm.system" clearable placeholder="请选择">
              <el-option label="供应链系统" value="supply-chain" />
              <el-option label="电商系统" value="ecommerce" />
              <el-option label="监控系统" value="monitoring" />
            </el-select>
          </el-form-item>

          <el-form-item label="资源名称" prop="name" class="resource-form-full">
            <el-input v-model="resourceForm.name" placeholder="请输入资源名称" />
          </el-form-item>

          <el-form-item label="IP 地址" prop="ip">
            <el-input v-model="resourceForm.ip" placeholder="192.168.1.10" />
          </el-form-item>

          <el-form-item label="SSH 端口">
            <el-input-number v-model="resourceForm.sshPort" :min="1" :max="65535" />
          </el-form-item>

          <el-form-item label="SSH 用户">
            <el-input v-model="resourceForm.sshUser" />
          </el-form-item>

          <el-form-item label="SSH 密码">
            <el-input v-model="resourceForm.sshPassword" type="password" show-password placeholder="不修改可留空" />
          </el-form-item>

          <el-form-item label="说明" class="resource-form-full">
            <el-input v-model="resourceForm.description" />
          </el-form-item>
        </div>
      </el-form>

      <template #footer>
        <div class="resource-dialog-footer">
          <el-button @click="resourceDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="saveResource">保存</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog
      v-model="ruleDialogVisible"
      class="ops-dialog"
      width="760px"
      align-center
      destroy-on-close
      modal-class="systemd-form-dialog-overlay"
    >
      <template #header>
        <div class="dialog-title">
          <span class="dialog-icon"><el-icon><Tickets /></el-icon></span>
          <div>
            <h2>新建 Prometheus 告警规则</h2>
            <p>用于展示运维配置弹框的字段组织、风险提示和提交区。</p>
          </div>
        </div>
      </template>

      <div class="dialog-body-grid">
        <el-form :model="ruleForm" label-position="top" class="ops-form dialog-form">
          <div class="form-grid two-col">
            <el-form-item label="规则名称">
              <el-input v-model="ruleForm.name" />
            </el-form-item>
            <el-form-item label="对象类型">
              <el-select v-model="ruleForm.objectType">
                <el-option label="K8S 工作负载" value="K8S 工作负载" />
                <el-option label="主机节点" value="主机节点" />
                <el-option label="网络设备" value="网络设备" />
              </el-select>
            </el-form-item>
            <el-form-item label="集群">
              <el-select v-model="ruleForm.cluster">
                <el-option label="prod-k8s-a" value="prod-k8s-a" />
                <el-option label="prod-k8s-b" value="prod-k8s-b" />
                <el-option label="core-network" value="core-network" />
              </el-select>
            </el-form-item>
            <el-form-item label="等级">
              <el-select v-model="ruleForm.severity">
                <el-option label="critical" value="critical" />
                <el-option label="warning" value="warning" />
                <el-option label="info" value="info" />
              </el-select>
            </el-form-item>
            <el-form-item label="持续时间">
              <el-input v-model="ruleForm.duration" />
            </el-form-item>
            <el-form-item label="负责人">
              <el-input v-model="ruleForm.owner" />
            </el-form-item>
          </div>
          <el-form-item label="PromQL 表达式">
            <el-input v-model="ruleForm.expr" type="textarea" :rows="3" />
          </el-form-item>
          <div class="dialog-options">
            <el-checkbox-group v-model="ruleForm.notify">
              <el-checkbox label="企业微信" />
              <el-checkbox label="短信" />
              <el-checkbox label="邮件" />
            </el-checkbox-group>
            <el-switch v-model="ruleForm.autoRemediation" active-text="启用自动处置建议" />
          </div>
        </el-form>

        <aside class="dialog-insight">
          <div class="insight-title">
            <el-icon><CircleCheck /></el-icon>
            规则质量检查
          </div>
          <ul>
            <li>表达式包含 15m 时间窗口，适合过滤短时抖动。</li>
            <li>critical 等级建议绑定负责人和至少两种通知渠道。</li>
            <li>自动处置建议会生成排查步骤，不会直接执行变更。</li>
          </ul>
        </aside>
      </div>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="ruleDialogVisible = false">取消</el-button>
          <el-button>保存草稿</el-button>
          <el-button type="primary" @click="ruleDialogVisible = false">创建规则</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog
      v-model="chatDialogVisible"
      class="ops-dialog ai-assistant-dialog"
      width="864px"
      align-center
      destroy-on-close
      :show-close="false"
    >
      <div class="assistant-shell">
        <header class="assistant-topbar">
          <div class="assistant-brand">
            <span class="assistant-bot-icon"><el-icon><ChatDotRound /></el-icon></span>
            <div>
              <h2>AIOps 智能助手</h2>
              <p>
                <el-tag size="small" type="success" effect="light">deepseek-v4-flash</el-tag>
                你好，我可以帮你结合平台上下文查询资源、根因分析、生成待执行任务等。
              </p>
            </div>
          </div>
          <el-button class="assistant-collapse-button" size="small" @click="chatDialogVisible = false">收起</el-button>
        </header>

        <div class="assistant-body">
          <aside class="assistant-sidebar">
            <div class="assistant-history-head">
              <span>会话历史</span>
              <el-button class="assistant-new-button" size="small" :icon="Plus">新建</el-button>
            </div>
            <div class="assistant-session-list">
              <button
                v-for="session in chatSessions"
                :key="session"
                class="session-item"
                :class="{ active: session === '帮我分析下告警id 632 api-gateway pod CPU usage is elevated' }"
              >
                {{ session }}
              </button>
            </div>
          </aside>

          <section class="assistant-main">
            <div class="assistant-filterbar">
              <strong>帮我分析下告警id 632 api-gateway pod CPU usage is elevated</strong>
              <el-tag type="warning" effect="light">未指定环境</el-tag>
              <span>只分析</span>
              <el-switch />
              <em>具备权限时可生成待执行动作</em>
            </div>

            <div class="assistant-conversation">
              <article class="user-question">
                <span>你 06/20 23:25</span>
                <p><mark>帮我分析下告警id 632 api-gateway pod CPU usage is elevated 这个告警的根因</mark></p>
              </article>

              <article class="assistant-reasoning">
                <div class="reasoning-head">
                  <span>思考过程</span>
                  <em>已思考 1 秒 · 2 个步骤</em>
                  <el-tag size="small" type="info" effect="light">已完成</el-tag>
                </div>
                <div class="reasoning-warning">
                  <div class="reasoning-status-row">
                    <el-tag type="danger" effect="plain">问答未完成</el-tag>
                    <el-tag type="info" effect="plain">环境前置</el-tag>
                  </div>
                  <strong>必须先指定环境</strong>
                  <p>AIOps 分析需要确认 <mark class="mark-blue">知识图谱环境</mark>。请在问题中带上环境名称，或从候选环境中选择后继续。</p>
                  <div>
                    <el-tag effect="plain">个人测试环境</el-tag>
                    <el-tag effect="plain" type="warning">电商测试环境</el-tag>
                  </div>
                </div>
                <div class="assistant-tool-chip">AIOps 知识图谱环境</div>
              </article>

              <article class="user-question">
                <span>你 06/20 23:26</span>
                <p>使用 <mark class="mark-orange">电商测试环境</mark> 继续分析：帮我分析下告警id 632 api-gateway pod CPU usage is elevated 这个告警的根因</p>
              </article>

              <article class="assistant-answer">
                <div class="reasoning-head">
                  <span>智能助手</span>
                  <em>已思考 6 秒 · 调用 4 个工具</em>
                  <el-tag size="small" type="success" effect="light">已完成</el-tag>
                </div>
                <p>
                  结论：<mark class="mark-orange">电商测试环境</mark> 当前 <mark class="mark-green">无活跃告警</mark>。
                  已查询告警中心、事件墙、系统态势，未发现异常。
                  K8S 集群 <mark class="mark-blue">ecommerce</mark> 命名空间最近 30 分钟有 CPU 峰值，但未达到告警阈值。
                </p>
                <div class="assistant-evidence">
                  <el-tag type="primary" effect="light">告警中心 0 条</el-tag>
                  <el-tag type="success" effect="light">事件墙正常</el-tag>
                  <el-tag type="warning" effect="light">CPU 峰值待观察</el-tag>
                </div>
              </article>
            </div>

            <div class="assistant-composer">
              <el-input
                v-model="chatForm.message"
                type="textarea"
                :rows="2"
                resize="none"
                placeholder="输入你的问题，Enter 发送，Shift + Enter 换行，Esc 收起"
              />
              <div class="assistant-composer-actions">
                <span>Enter 发送，Shift + Enter 换行</span>
                <div>
                  <el-button>清空</el-button>
                  <el-button type="primary" :icon="Promotion">发送</el-button>
                </div>
              </div>
            </div>
          </section>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
.prometheus-demo-page {
  min-height: 100%;
  padding: 7px;
  color: #172033;
}

:global(.home-container .el-main:has(.prometheus-demo-page)) {
  padding-top: 10px;
  padding-right: 10px;
  padding-left: 10px;
}

.dashboard-view-tabs {
  display: flex;
  justify-content: flex-start;
  margin-bottom: 7px;
}

.dashboard-view-tabs :deep(.el-segmented) {
  padding: 3px;
  border: 1px solid #d8e5f3;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.88);
  box-shadow: 0 5px 16px rgba(70, 116, 170, 0.06);
}

.dashboard-view-tabs :deep(.el-segmented__item) {
  min-width: 104px;
  font-size: 13px;
  font-weight: 700;
}

.monitor-workbench,
.task-workbench {
  min-width: 0;
}

.task-workbench {
  display: grid;
  gap: 10px;
}

.task-hero {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 16px;
  border: 1px solid #dbe7f4;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
}

.task-hero-icon {
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  width: 40px;
  height: 40px;
  border: 1px solid #cfe0ff;
  border-radius: 8px;
  color: #2563eb;
  background: #eff6ff;
}

.task-hero h1,
.task-panel-header h2,
.dispatch-type-panel h2,
.task-editor-header h2,
.template-panel-header h2 {
  margin: 0;
  color: #172033;
  letter-spacing: 0;
}

.task-hero h1 {
  font-size: 22px;
  line-height: 1.25;
}

.task-hero p,
.task-panel-header p,
.template-panel-header p {
  margin: 5px 0 0;
  color: #64748b;
  font-size: 12px;
  line-height: 1.55;
}

.task-section-tabs {
  display: flex;
  align-items: stretch;
  min-height: 44px;
  padding: 0 8px;
  border: 1px solid #dbe7f4;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 6px 18px rgba(80, 124, 180, 0.05);
}

.task-section-tabs button {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-width: 112px;
  padding: 0 14px;
  border: 0;
  color: #52647b;
  background: transparent;
  cursor: pointer;
  font: inherit;
  font-size: 13px;
  font-weight: 700;
}

.task-section-tabs button::after {
  position: absolute;
  right: 14px;
  bottom: 0;
  left: 14px;
  height: 3px;
  border-radius: 3px 3px 0 0;
  background: transparent;
  content: '';
}

.task-section-tabs button:hover,
.task-section-tabs button.active {
  color: #2563eb;
}

.task-section-tabs button.active::after {
  background: #409eff;
}

.task-stat-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.task-stat-grid article {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 66px;
  padding: 12px 16px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 7px 20px rgba(80, 124, 180, 0.05);
}

.task-stat-grid article.is-primary {
  border-color: #b9d3ff;
  box-shadow: 0 0 0 1px rgba(64, 158, 255, 0.12), 0 7px 20px rgba(64, 158, 255, 0.08);
}

.task-stat-grid article.is-success {
  border-top: 3px solid #10b981;
}

.task-stat-grid article.is-warning {
  border-top: 3px solid #f59e0b;
}

.task-stat-grid span {
  color: #475569;
  font-size: 13px;
  font-weight: 700;
}

.task-stat-grid strong {
  color: #172033;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 24px;
  line-height: 1;
}

.task-history-panel {
  min-width: 0;
  padding: 12px;
  border: 1px solid #dbe7f4;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.06);
}

.task-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 10px;
}

.task-panel-header h2 {
  font-size: 16px;
}

.task-panel-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.task-panel-actions > span {
  color: #7b8da5;
  font-size: 12px;
}

.task-filterbar {
  display: grid;
  grid-template-columns: minmax(190px, 1.4fr) repeat(5, minmax(120px, 0.82fr));
  gap: 8px;
  margin-bottom: 8px;
  padding: 9px;
  border: 1px solid #e3ebf5;
  border-radius: 8px;
  background: #f8fbff;
}

.task-filterbar :deep(.el-input__wrapper),
.task-filterbar :deep(.el-select__wrapper) {
  border-radius: 7px;
  background: #fff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.task-table {
  width: 100%;
  border: 1px solid #e3ebf5;
  border-radius: 8px;
  overflow: hidden;
}

.task-table :deep(.el-table__header th) {
  height: 34px;
  color: #475569;
  background: #f5f8fc;
  font-size: 12px;
}

.task-table :deep(.el-table__cell) {
  padding: 5px 0;
  font-size: 12px;
}

.task-name-cell {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.task-name-cell strong {
  overflow: hidden;
  color: #172033;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.task-name-cell span {
  overflow: hidden;
  color: #8a9ab0;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.task-table :deep(.el-button + .el-button) {
  margin-left: 7px;
}

.task-dispatch-layout {
  display: grid;
  grid-template-columns: 232px minmax(0, 1fr);
  gap: 10px;
  min-height: 570px;
}

.dispatch-type-panel,
.task-editor-panel,
.task-template-panel {
  min-width: 0;
  border: 1px solid #dbe7f4;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.06);
}

.dispatch-type-panel {
  display: grid;
  align-content: start;
  gap: 8px;
  padding: 12px;
}

.dispatch-type-panel header,
.task-editor-header,
.template-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.dispatch-type-panel header {
  margin-bottom: 2px;
}

.dispatch-type-panel h2,
.task-editor-header h2,
.template-panel-header h2 {
  font-size: 16px;
}

.dispatch-type-panel > button {
  display: grid;
  gap: 7px;
  width: 100%;
  padding: 12px;
  border: 1px solid #dfe7f0;
  border-radius: 8px;
  color: #172033;
  background: #fff;
  cursor: pointer;
  text-align: left;
  transition: border-color 0.16s ease, background-color 0.16s ease, box-shadow 0.16s ease;
}

.dispatch-type-panel > button:hover {
  border-color: #b9d4ff;
}

.dispatch-type-panel > button.active {
  border-color: #a9c9ff;
  background: #f1f6ff;
  box-shadow: 0 0 0 1px rgba(64, 158, 255, 0.08) inset;
}

.dispatch-type-panel button strong {
  font-size: 13px;
  line-height: 1.2;
}

.dispatch-type-panel button span {
  color: #6b7d95;
  font-size: 12px;
  line-height: 1.45;
}

.task-editor-panel {
  padding: 12px 14px 14px;
}

.task-editor-header {
  margin-bottom: 8px;
}

.task-editor-header > div,
.template-panel-header > div:last-child {
  display: flex;
  align-items: center;
  gap: 8px;
}

.dispatch-notice {
  margin-bottom: 8px;
  padding: 8px 10px;
  border: 1px solid #d9e7fb;
  border-radius: 8px;
  color: #6b7d95;
  background: #f7faff;
  font-size: 12px;
  line-height: 1.5;
}

.task-dispatch-form :deep(.el-form-item) {
  margin-bottom: 9px;
}

.task-dispatch-form :deep(.el-form-item__label) {
  color: #475569;
  font-size: 12px;
}

.task-dispatch-form :deep(.el-input__wrapper),
.task-dispatch-form :deep(.el-select__wrapper),
.task-dispatch-form :deep(.el-input-number),
.template-filterbar :deep(.el-input__wrapper),
.template-filterbar :deep(.el-select__wrapper) {
  border-radius: 7px;
  background: #fff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.task-form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 14px;
}

.task-dispatch-form :deep(.el-input-number) {
  width: 100%;
}

.dispatch-method-note {
  display: grid;
  grid-template-columns: 88px minmax(0, 1fr);
  align-items: center;
  min-height: 38px;
  margin-bottom: 8px;
}

.dispatch-method-note span {
  padding-right: 12px;
  color: #475569;
  font-size: 12px;
  text-align: right;
}

.dispatch-method-note p {
  margin: 0;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.script-form-item :deep(.el-form-item__content) {
  min-width: 0;
}

.script-editor {
  width: 100%;
  overflow: hidden;
  border: 1px solid #17233c;
  border-radius: 8px;
  background: #10182b;
}

.script-editor header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 28px;
  padding: 0 10px;
  border-bottom: 1px solid #26324a;
  color: #cdd7e8;
  background: #111b31;
  font-size: 11px;
}

.script-editor header strong {
  color: #86efac;
  font-size: 10px;
}

.script-editor textarea {
  display: block;
  width: 100%;
  min-height: 270px;
  padding: 12px 14px;
  resize: vertical;
  border: 0;
  outline: 0;
  color: #d7e2f2;
  background: #10182b;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  line-height: 1.65;
  tab-size: 2;
}

.task-template-panel {
  padding: 12px 14px 14px;
}

.template-panel-header {
  align-items: flex-start;
  margin-bottom: 9px;
}

.template-filterbar {
  display: grid;
  grid-template-columns: minmax(220px, 0.9fr) 160px auto 1fr;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  padding: 9px;
  border: 1px solid #e3ebf5;
  border-radius: 8px;
  background: #f8fbff;
}

.template-filterbar > .el-button {
  justify-self: start;
}

.template-table {
  width: 100%;
  overflow: hidden;
  border: 1px solid #e3ebf5;
  border-radius: 8px;
}

.template-table :deep(.el-table__header th) {
  height: 34px;
  color: #475569;
  background: #f5f8fc;
  font-size: 12px;
}

.template-table :deep(.el-table__cell) {
  padding: 6px 0;
  font-size: 12px;
}

.template-table :deep(.el-button + .el-button) {
  margin-left: 8px;
}

.hero-panel {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 12px;
  padding: 14px 16px;
  border: 1px solid #dbe7f4;
  border-radius: 12px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(248, 251, 255, 0.94)),
    #fff;
  box-shadow: 0 10px 28px rgba(80, 124, 180, 0.1);
}

.hero-copy {
  position: relative;
  max-width: 760px;
  padding-left: 52px;
}

.hero-kicker {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
  color: #5f718a;
  font-size: 13px;
  font-weight: 700;
}

.hero-kicker .el-icon {
  position: absolute;
  left: 0;
  top: 0;
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  border: 1px solid #d5e4ff;
  border-radius: 12px;
  color: #2563eb;
  background: linear-gradient(180deg, #f5f9ff, #ecf4ff);
}

.hero-copy h1 {
  margin: 0;
  color: #172033;
  font-size: 22px;
  line-height: 1.25;
  letter-spacing: 0;
}

.hero-copy p {
  max-width: 680px;
  margin: 7px 0 0;
  color: #64748b;
  font-size: 13px;
  line-height: 1.6;
}

.hero-actions {
  display: flex;
  gap: 10px;
  white-space: nowrap;
}

.health-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 12px;
}

.health-card {
  padding: 12px 14px;
  border: 1px solid #dbe7f4;
  border-radius: 10px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(250, 253, 255, 0.96)),
    #fff;
  box-shadow: 0 8px 22px rgba(80, 124, 180, 0.06);
}

.health-card span {
  color: #64748b;
  font-size: 12px;
  font-weight: 700;
}

.health-card strong {
  display: block;
  margin-top: 7px;
  color: #0f172a;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 26px;
  line-height: 1;
}

.health-card p {
  margin: 8px 0 0;
  color: #64748b;
  font-size: 12px;
}

.health-card.is-primary {
  border-top: 3px solid #0ea5e9;
}

.health-card.is-success {
  border-top: 3px solid #10b981;
}

.health-card.is-warning {
  border-top: 3px solid #f59e0b;
}

.health-card.is-danger {
  border-top: 3px solid #ef4444;
}

.workspace-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  gap: 12px;
}

.main-column,
.side-column {
  display: grid;
  align-content: start;
  gap: 12px;
}

.ops-card {
  border: 1px solid #dbe7f4;
  border-radius: 12px;
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
}

.ops-card :deep(.el-card__header) {
  padding: 13px 16px;
  border-bottom-color: #e7eef7;
}

.ops-card :deep(.el-card__body) {
  padding: 14px 16px;
}

.card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.card-header.compact {
  align-items: center;
}

.card-header h2 {
  margin: 0;
  color: #0f172a;
  font-size: 16px;
  line-height: 1.25;
}

.card-header p {
  margin: 5px 0 0;
  color: #6b7d95;
  font-size: 12px;
}

.ops-form {
  --el-component-size: 34px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px 14px;
}

.form-grid.two-col {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.ops-form :deep(.el-form-item) {
  margin-bottom: 14px;
}

.ops-form :deep(.el-form-item__label) {
  margin-bottom: 6px;
  color: #334155;
  font-size: 12px;
  font-weight: 700;
}

.ops-form :deep(.el-input__wrapper),
.ops-form :deep(.el-textarea__inner),
.ops-form :deep(.el-select__wrapper) {
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.ops-form :deep(.el-input__wrapper:hover),
.ops-form :deep(.el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.ops-form :deep(.el-textarea__inner) {
  color: #0f172a;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  line-height: 1.7;
}

.form-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-top: 4px;
}

.switch-stack {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
}

.rule-list {
  display: grid;
  gap: 10px;
}

.rule-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 14px;
  padding: 12px;
  border: 1px solid #dce8f5;
  border-radius: 10px;
  background: #f8fbff;
}

.rule-item h3 {
  margin: 0 0 7px;
  color: #0f172a;
  font-size: 13px;
}

.rule-item code {
  color: #475569;
  font-size: 12px;
  white-space: normal;
}

.rule-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #64748b;
  font-size: 12px;
}

.target-table :deep(.el-table__header th) {
  background: #f6f9fd;
  color: #475569;
  font-size: 12px;
}

.target-table :deep(.el-table__row td) {
  font-size: 12px;
}

.ai-card {
  border-color: #cfe2ff;
  background:
    linear-gradient(180deg, rgba(246, 250, 255, 0.98), rgba(255, 255, 255, 0.98)),
    #fff;
}

.ai-card :deep(.el-card__header) {
  border-bottom-color: #dce9f8;
}

.side-title {
  display: flex;
  align-items: center;
  gap: 8px;
  color: inherit;
  font-size: 14px;
  font-weight: 800;
}

.ai-score {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 4px 10px;
  align-items: end;
  padding-bottom: 12px;
  border-bottom: 1px solid #e4edf8;
}

.ai-score span {
  color: #6b7d95;
  font-size: 12px;
}

.ai-score strong {
  grid-row: span 2;
  color: #2563eb;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 38px;
  line-height: 1;
}

.ai-score em {
  color: #d97706;
  font-size: 12px;
  font-style: normal;
}

.ai-list {
  display: grid;
  gap: 10px;
  margin: 14px 0;
  padding: 0;
  list-style: none;
}

.ai-list li {
  padding: 10px 11px;
  border: 1px solid #d9e8fb;
  border-radius: 9px;
  color: #304156;
  background: #f7fbff;
  font-size: 12px;
  line-height: 1.6;
}

.full-button {
  width: 100%;
}

.object-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.object-grid div {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 6px 8px;
  padding: 12px;
  border: 1px solid #dce8f5;
  border-radius: 10px;
  background: #f8fbff;
}

.object-grid .el-icon {
  color: #0ea5e9;
}

.object-grid span {
  color: #64748b;
  font-size: 12px;
}

.object-grid strong {
  grid-column: 1 / -1;
  color: #0f172a;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 22px;
}

.checklist-card :deep(.el-timeline) {
  padding-left: 4px;
}

.checklist-card :deep(.el-timeline-item__content) {
  color: #334155;
  font-size: 12px;
}

:global(.resource-dialog) {
  overflow: hidden;
  border: 1px solid #dbe4ef;
  border-radius: 12px;
  background: #fff;
  box-shadow: 0 24px 70px rgba(15, 23, 42, 0.24);
}

:global(.resource-dialog .el-dialog__header) {
  margin: 0;
  padding: 17px 18px 12px;
}

:global(.resource-dialog .el-dialog__title) {
  color: #273244;
  font-size: 18px;
  font-weight: 500;
}

:global(.resource-dialog .el-dialog__headerbtn) {
  top: 13px;
  right: 12px;
}

:global(.resource-dialog .el-dialog__body) {
  padding: 14px 28px 26px 22px;
}

:global(.resource-dialog .el-dialog__footer) {
  padding: 12px 16px 16px;
}

.resource-form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  column-gap: 24px;
}

.resource-form-full {
  grid-column: 1 / -1;
}

.resource-form :deep(.el-form-item) {
  margin-bottom: 13px;
}

.resource-form :deep(.el-form-item__label) {
  padding-right: 12px;
  color: #606b7a;
  font-size: 13px;
}

.resource-form :deep(.el-input__wrapper),
.resource-form :deep(.el-select__wrapper),
.resource-form :deep(.el-input-number) {
  min-height: 34px;
  border-radius: 8px;
  background: #fff;
  box-shadow: 0 0 0 1px #dfe6ee inset;
}

.resource-form :deep(.el-select),
.resource-form :deep(.el-input-number) {
  width: 100%;
}

.resource-form :deep(.el-radio-group) {
  display: flex;
  align-items: center;
  gap: 22px;
  min-height: 34px;
}

.resource-form :deep(.el-radio) {
  margin-right: 0;
}

.resource-form :deep(.el-radio__label) {
  padding-left: 7px;
}

.resource-dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

:global(.ops-dialog) {
  border: 1px solid #d7e6f6;
  border-radius: 14px;
  overflow: hidden;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 251, 255, 0.98)),
    #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.ops-dialog .el-dialog__header) {
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
}

:global(.ops-dialog .el-dialog__body) {
  padding: 16px 18px;
}

:global(.ops-dialog .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
}

:global(.systemd-form-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  overflow: hidden;
  border-radius: var(--el-dialog-border-radius) !important;
}

.dialog-title {
  display: flex;
  align-items: center;
  gap: 12px;
}

.dialog-icon {
  display: grid;
  place-items: center;
  flex: 0 0 38px;
  width: 38px;
  height: 38px;
  border: 1px solid #d5e4ff;
  border-radius: 12px;
  color: #2563eb;
  background: linear-gradient(180deg, #f5f9ff, #ecf4ff);
}

.dialog-title h2 {
  margin: 0;
  color: #172033;
  font-size: 17px;
  line-height: 1.3;
}

.dialog-title p {
  margin: 4px 0 0;
  color: #6b7d95;
  font-size: 12px;
}

.dialog-body-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 230px;
  gap: 14px;
}

.dialog-form :deep(.el-form-item) {
  margin-bottom: 12px;
}

.dialog-options {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid #dce8f5;
  border-radius: 10px;
  background: #f8fbff;
}

.dialog-insight {
  padding: 13px;
  border: 1px solid #cfe2ff;
  border-radius: 12px;
  background: #f6faff;
}

.insight-title {
  display: flex;
  align-items: center;
  gap: 7px;
  color: #2563eb;
  font-size: 13px;
  font-weight: 800;
}

.dialog-insight ul {
  display: grid;
  gap: 10px;
  margin: 12px 0 0;
  padding: 0;
  list-style: none;
}

.dialog-insight li {
  color: #40536b;
  font-size: 12px;
  line-height: 1.55;
}

.dialog-footer,
.chat-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

:global(.ai-assistant-dialog .el-dialog__header),
:global(.ai-assistant-dialog .el-dialog__footer) {
  display: none;
}

:global(.ai-assistant-dialog .el-dialog__body) {
  padding: 0;
  overflow: hidden;
  border-radius: 20px;
  background: transparent;
}

:global(.ai-assistant-dialog) {
  max-width: calc(100vw - 64px);
  border: 1px solid rgba(198, 216, 237, 0.9);
  border-radius: 20px !important;
  overflow: hidden;
  background:
    linear-gradient(#ffffff, #ffffff) padding-box,
    linear-gradient(90deg, #f5a524 0%, #4ea7ff 42%, #55d6be 100%) border-box;
  box-shadow:
    0 28px 80px rgba(39, 70, 110, 0.24),
    0 0 0 1px rgba(255, 255, 255, 0.72) inset;
}

.assistant-shell {
  position: relative;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  height: min(560px, 78vh);
  overflow: hidden;
  border-radius: 19px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.96), rgba(248, 251, 255, 0.98)),
    #fff;
}

.assistant-body {
  display: grid;
  grid-template-columns: 206px minmax(0, 1fr);
  min-height: 0;
}

.assistant-sidebar {
  display: grid;
  grid-template-rows: auto 1fr;
  border-right: 1px solid #d7e6f6;
  background:
    linear-gradient(180deg, rgba(248, 252, 255, 0.96), rgba(239, 246, 255, 0.94)),
    #f4f8fe;
}

.assistant-history-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 10px 11px;
  border-bottom: 1px solid #dce8f5;
}

.assistant-history-head span {
  color: #172033;
  font-size: 13px;
  font-weight: 800;
}

.assistant-new-button,
.assistant-collapse-button {
  border-color: #d6e2ef;
  border-radius: 9px;
  color: #334155;
  background: rgba(255, 255, 255, 0.82);
}

.assistant-session-list {
  display: grid;
  align-content: start;
  gap: 5px;
  padding: 8px;
  overflow: auto;
}

.session-item {
  width: 100%;
  padding: 9px 10px;
  overflow: hidden;
  border: 1px solid transparent;
  border-radius: 10px;
  color: #304156;
  background: transparent;
  cursor: pointer;
  font-size: 13px;
  line-height: 1.35;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-item:hover,
.session-item.active {
  border-color: #bfd8f5;
  color: #172033;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(245, 250, 255, 0.96)),
    #fff;
  box-shadow:
    0 8px 20px rgba(79, 128, 190, 0.08),
    3px 0 0 #4ea7ff inset;
}

.assistant-main {
  display: grid;
  grid-template-rows: auto 1fr auto;
  min-width: 0;
  min-height: 0;
  background:
    radial-gradient(circle at 100% 0%, rgba(219, 235, 255, 0.52), transparent 30%),
    linear-gradient(180deg, #fbfdff 0%, #f8fbff 100%);
}

.assistant-topbar {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  overflow: hidden;
  padding: 15px 14px 12px;
  border-bottom: 1px solid rgba(203, 220, 240, 0.86);
  border-radius: 19px 19px 0 0;
  background:
    radial-gradient(circle at 2% 0%, rgba(255, 186, 95, 0.32), transparent 22%),
    radial-gradient(circle at 82% 0%, rgba(91, 168, 255, 0.22), transparent 28%),
    linear-gradient(90deg, rgba(255, 246, 231, 0.92) 0%, rgba(247, 251, 255, 0.96) 36%, rgba(236, 247, 255, 0.94) 100%);
}

.assistant-topbar::before {
  position: absolute;
  inset: 0 0 auto;
  height: 3px;
  background: linear-gradient(90deg, #f5a524 0%, #62a8ff 48%, #55d6be 100%);
  content: '';
}

.assistant-topbar::after {
  position: absolute;
  inset: 3px 0 auto;
  height: 1px;
  background: linear-gradient(90deg, rgba(255, 255, 255, 0.82), rgba(255, 255, 255, 0));
  content: '';
}

.assistant-brand {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  gap: 10px;
}

.assistant-bot-icon {
  position: relative;
  display: grid;
  place-items: center;
  flex: 0 0 34px;
  width: 34px;
  height: 34px;
  border: 1px solid #bed8ff;
  border-radius: 13px;
  color: #2563eb;
  background:
    radial-gradient(circle at 32% 20%, rgba(255, 255, 255, 0.95), transparent 35%),
    linear-gradient(180deg, #f3f8ff, #e7f1ff);
  box-shadow: 0 8px 18px rgba(63, 131, 248, 0.12);
}

.assistant-bot-icon::before {
  position: absolute;
  inset: -8px;
  z-index: -1;
  border-radius: 18px;
  background: radial-gradient(circle, rgba(245, 165, 36, 0.24), transparent 64%);
  content: '';
}

.assistant-brand h2 {
  margin: 0;
  color: #172033;
  font-size: 15px;
  line-height: 1.3;
}

.assistant-brand p {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 3px 0 0;
  color: #6b7d95;
  font-size: 12px;
}

.assistant-collapse-button {
  position: relative;
  z-index: 1;
}

.assistant-filterbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 13px;
  border-bottom: 1px solid #e4edf8;
  background:
    linear-gradient(180deg, rgba(248, 251, 255, 0.94), rgba(242, 247, 253, 0.92)),
    #f8fbff;
}

.assistant-filterbar strong {
  color: #172033;
  font-size: 13px;
}

.assistant-filterbar span,
.assistant-filterbar em {
  color: #6b7d95;
  font-size: 12px;
  font-style: normal;
}

.assistant-filterbar em {
  margin-left: auto;
}

.assistant-conversation {
  display: grid;
  align-content: start;
  gap: 10px;
  overflow: auto;
  padding: 12px 11px 10px;
}

.user-question {
  display: grid;
  justify-items: end;
  gap: 6px;
}

.user-question span {
  color: #7b8da5;
  font-size: 12px;
}

.user-question p {
  max-width: 440px;
  margin: 0;
  padding: 8px 11px;
  border: 1px solid #c7ddfb;
  border-radius: 16px 16px 4px 16px;
  color: #172033;
  background:
    linear-gradient(180deg, #eef6ff, #e6f1ff),
    #eaf3ff;
  box-shadow: 0 8px 18px rgba(70, 125, 190, 0.08);
  font-size: 13px;
  line-height: 1.55;
}

.user-question mark,
.assistant-answer mark,
.reasoning-warning mark {
  padding: 1px 5px;
  border-radius: 6px;
  color: #14539f;
  background: #e7f1ff;
}

.user-question mark.mark-blue,
.assistant-answer mark.mark-blue,
.reasoning-warning mark.mark-blue {
  color: #14539f;
  background: #e7f1ff;
}

.user-question mark.mark-orange,
.assistant-answer mark.mark-orange,
.reasoning-warning mark.mark-orange {
  color: #b45309;
  background: #fff3df;
}

.user-question mark.mark-green,
.assistant-answer mark.mark-green,
.reasoning-warning mark.mark-green {
  color: #15803d;
  background: #eaf8ef;
}

.assistant-reasoning,
.assistant-answer {
  width: calc(100% - 18px);
  max-width: 624px;
  padding: 10px;
  border: 1px solid #cfe2f8;
  border-radius: 16px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(250, 253, 255, 0.96)),
    #fff;
  box-shadow:
    0 12px 28px rgba(70, 116, 170, 0.07),
    0 0 0 1px rgba(255, 255, 255, 0.86) inset;
}

.assistant-answer {
  margin-top: 3px;
}

.reasoning-head {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 6px 8px;
  border: 1px solid #d8e8f8;
  border-radius: 10px;
  background:
    linear-gradient(180deg, #f9fcff, #f2f7fd),
    #f8fbff;
}

.reasoning-head span {
  color: #304156;
  font-size: 13px;
  font-weight: 800;
}

.reasoning-head em {
  color: #7b8da5;
  font-size: 12px;
  font-style: normal;
}

.reasoning-head .el-tag {
  margin-left: auto;
}

.reasoning-warning {
  margin-top: 9px;
  padding: 10px;
  border: 1px solid #ffc785;
  border-radius: 12px;
  background:
    linear-gradient(180deg, rgba(255, 251, 245, 0.98), rgba(255, 247, 237, 0.96)),
    #fff8f1;
  box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.82) inset;
}

.reasoning-status-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 9px;
}

.reasoning-warning strong {
  color: #c2410c;
  font-size: 13px;
}

.reasoning-warning p,
.assistant-answer p {
  margin: 8px 0 0;
  color: #334155;
  font-size: 13px;
  line-height: 1.7;
}

.reasoning-warning > div:not(.reasoning-status-row) {
  display: flex;
  gap: 8px;
  margin-top: 10px;
}

.assistant-tool-chip {
  display: inline-flex;
  width: fit-content;
  margin-top: 9px;
  padding: 4px 8px;
  border: 1px solid #bdeee7;
  border-radius: 999px;
  color: #0f766e;
  background: #e8fbf7;
  font-size: 12px;
  line-height: 1;
}

.assistant-evidence {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.assistant-composer {
  display: grid;
  gap: 8px;
  padding: 7px 10px 8px;
  border-top: 1px solid #dce8f5;
  background:
    linear-gradient(180deg, rgba(249, 252, 255, 0.94), rgba(255, 255, 255, 0.98)),
    #fff;
}

.assistant-composer :deep(.el-textarea__inner) {
  border-radius: 10px;
  background: #fbfdff;
  min-height: 48px !important;
  font-size: 12px;
  line-height: 1.45;
  box-shadow:
    0 0 0 1px #cfe0f3 inset,
    0 8px 18px rgba(70, 116, 170, 0.05);
}

.assistant-composer-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.assistant-composer-actions span {
  color: #7b8da5;
  font-size: 12px;
}

@media (max-width: 1180px) {
  .workspace-grid {
    grid-template-columns: 1fr;
  }

  .side-column {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .task-filterbar {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .task-dispatch-layout {
    grid-template-columns: 210px minmax(0, 1fr);
  }

  .template-filterbar {
    grid-template-columns: minmax(220px, 1fr) 160px auto;
  }

  .template-filterbar > .el-button {
    grid-column: 1 / -1;
  }
}

@media (max-width: 860px) {
  .prometheus-demo-page {
    padding: 6px;
  }

  .hero-panel,
  .card-header,
  .form-footer {
    flex-direction: column;
    align-items: stretch;
  }

  .hero-actions {
    flex-wrap: wrap;
  }

  .health-grid,
  .form-grid,
  .dialog-body-grid,
  .resource-form-grid,
  .side-column,
  .task-stat-grid,
  .task-filterbar,
  .task-form-grid,
  .template-filterbar {
    grid-template-columns: 1fr;
  }

  .resource-form-full {
    grid-column: auto;
  }

  .dashboard-view-tabs :deep(.el-segmented),
  .dashboard-view-tabs :deep(.el-segmented__group) {
    width: 100%;
  }

  .dashboard-view-tabs :deep(.el-segmented__item) {
    flex: 1;
    min-width: 0;
  }

  .task-section-tabs {
    overflow-x: auto;
  }

  .task-section-tabs button {
    flex: 1 0 auto;
  }

  .task-hero,
  .task-panel-header,
  .task-editor-header,
  .template-panel-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .task-panel-actions {
    flex-wrap: wrap;
  }

  .task-dispatch-layout {
    grid-template-columns: 1fr;
  }

  .dispatch-type-panel {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dispatch-type-panel header {
    grid-column: 1 / -1;
  }

  .dispatch-method-note {
    grid-template-columns: 1fr;
    gap: 5px;
  }

  .dispatch-method-note span {
    padding: 0;
    text-align: left;
  }

  .script-form-item {
    display: block;
  }

  .task-template-panel {
    overflow-x: auto;
  }

  .template-table {
    min-width: 980px;
  }

  .dialog-options,
  .assistant-filterbar,
  .assistant-composer-actions {
    align-items: flex-start;
    flex-direction: column;
  }

  .assistant-shell {
    height: min(640px, 86vh);
  }

  .assistant-body {
    grid-template-columns: 1fr;
  }

  .assistant-sidebar {
    display: none;
  }

  .assistant-brand,
  .assistant-brand p,
  .assistant-filterbar {
    flex-wrap: wrap;
  }

  .assistant-filterbar em {
    margin-left: 0;
  }
}
</style>
