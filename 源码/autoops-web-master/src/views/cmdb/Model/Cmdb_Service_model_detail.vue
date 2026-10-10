<template>
  <div class="observability-page">
    <section class="observability-shell">
      <header class="observability-header">
        <div class="header-main-row">
          <div class="header-identity">
            <el-button class="icon-command icon-command--plain" text aria-label="返回业务列表" @click="goBack">
              <el-icon><ArrowLeft /></el-icon>
            </el-button>
            <div class="business-heading">
              <div class="business-path">
                <span>业务系统</span>
                <i>/</i>
                <strong>{{ businessTitle }}</strong>
                <em class="environment-badge">{{ summary.env || currentBusiness?.env || '默认' }}</em>
                <em :class="['health-badge', statusClass(summaryStatus)]">{{ statusText(summaryStatus) }}</em>
              </div>
              <p>{{ summary.problemSummary || '业务运行状态正常' }}</p>
            </div>
          </div>

          <div class="header-actions">
            <span class="generated-time">
              <el-icon><Calendar /></el-icon>
              {{ formatDateTime(observability.generatedAt) }}
            </span>
            <el-select v-model="selectedWindow" class="window-select" aria-label="观测时间窗口" @change="loadObservability">
              <el-option v-for="item in windowOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
            <el-tooltip content="刷新" placement="bottom">
              <el-button class="icon-command" :loading="refreshing" aria-label="刷新观测数据" @click="handleRefresh">
                <el-icon><Refresh /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip content="分享当前视图" placement="bottom">
              <el-button class="share-command" aria-label="分享当前视图" @click="shareDashboard">
                <el-icon><Share /></el-icon><span>分享</span>
              </el-button>
            </el-tooltip>
            <el-dropdown trigger="click">
              <el-button class="icon-command" aria-label="更多操作"><el-icon><MoreFilled /></el-icon></el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="goBack">返回业务列表</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </div>

        <nav class="detail-tabs" aria-label="业务详情视图">
          <button type="button" class="is-active" aria-current="page">业务大盘</button>
          <button type="button" @click="goTopology()">编辑拓扑图</button>
          <button type="button" @click="goLogAnalysis">日志分析</button>
          <button type="button" @click="goMonitorAlerts">监控告警</button>
        </nav>
      </header>

      <div v-if="degradationMessages.length" class="degradation-banner" role="alert">
        <el-icon><Warning /></el-icon>
        <div>
          <strong>{{ observability.partial ? '部分观测数据暂不可用' : '日志源异常' }}</strong>
          <span>{{ degradationMessages.join('；') }}</span>
        </div>
        <button type="button" @click="loadObservability">重试</button>
      </div>

      <div v-if="loadError" class="load-error" role="alert">
        <el-icon><Warning /></el-icon>
        <div><strong>业务可观测数据加载失败</strong><span>{{ loadError }}</span></div>
        <el-button type="primary" @click="loadAll">重新加载</el-button>
      </div>

      <main v-else v-loading="loading" class="observability-grid">
        <aside class="navigator-panel">
          <div class="navigator-heading">
            <div><h3>系统与服务</h3><span>{{ businessModels.length }} 个业务系统</span></div>
            <el-tooltip content="仅显示异常" placement="bottom">
              <button type="button" class="filter-command" :class="{ active: serviceFilter === 'abnormal' }" @click="toggleAbnormalFilter">
                <el-icon><Filter /></el-icon>
              </button>
            </el-tooltip>
          </div>

          <el-input v-model="serviceKeyword" clearable class="service-search" placeholder="搜索系统或服务">
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>

          <div class="service-filter" role="group" aria-label="服务状态筛选">
            <button type="button" :class="{ active: serviceFilter === 'all' }" @click="serviceFilter = 'all'">全部</button>
            <button type="button" :class="{ active: serviceFilter === 'abnormal' }" @click="serviceFilter = 'abnormal'">异常</button>
          </div>

          <div class="system-tree">
            <section v-for="item in filteredBusinessModels" :key="item.id" class="system-tree-group">
              <button type="button" class="system-tree-root" :class="{ 'is-current': String(item.id) === String(businessId) }" @click="switchBusiness(item.id)">
                <el-icon class="tree-chevron"><ArrowRight /></el-icon>
                <span>{{ item.displayName || item.name }}</span>
                <em :class="statusClass(businessModelStatus(item))" title="风险业务节点 / 业务节点">{{ businessStatusCount(item) }}</em>
              </button>

              <div v-if="String(item.id) === String(businessId)" class="service-list">
                <button
                  v-for="node in filteredServiceNodes"
                  :key="node.id"
                  type="button"
                  :class="{ 'is-selected': String(selectedNode?.id) === String(node.id) }"
                  @click="selectNode(node)"
                >
                  <i :class="statusClass(nodeStatus(node))"></i>
                  <span><strong>{{ node.displayName || node.label || node.name }}</strong><small>{{ resourceTypeText(node.resourceType) }}</small></span>
                  <em>{{ nodeErrorCount(node) }}</em>
                </button>
                <div v-if="!filteredServiceNodes.length" class="compact-empty">没有匹配的服务节点</div>
              </div>
            </section>
          </div>
        </aside>

        <section class="kpi-panel" aria-label="业务观测指标">
          <article v-for="item in kpiItems" :key="item.key">
            <span class="kpi-icon" :class="`kpi-icon--${item.key}`" aria-hidden="true"><el-icon><component :is="item.icon" /></el-icon></span>
            <div class="kpi-copy"><span>{{ item.label }}</span><strong :class="item.tone">{{ item.value }}</strong><small>{{ item.caption }}</small></div>
          </article>
        </section>

        <section class="topology-panel">
          <div class="panel-heading topology-heading">
            <div><h3>资源拓扑</h3><span>{{ topologyNodes.length }} 个节点</span></div>
            <div class="topology-tools">
              <div class="topology-legend"><span><i class="healthy"></i>正常</span><span><i class="warning"></i>风险</span><span><i class="critical"></i>异常</span></div>
              <div class="zoom-tools">
                <el-tooltip content="缩小" placement="top"><button type="button" aria-label="缩小拓扑" @click="zoomTopology(-0.1)"><el-icon><ZoomOut /></el-icon></button></el-tooltip>
                <span>{{ topologyZoomLabel }}</span>
                <el-tooltip content="放大" placement="top"><button type="button" aria-label="放大拓扑" @click="zoomTopology(0.1)"><el-icon><ZoomIn /></el-icon></button></el-tooltip>
                <el-tooltip content="适应画布" placement="top"><button type="button" aria-label="拓扑适应画布" @click="fitTopology"><el-icon><FullScreen /></el-icon></button></el-tooltip>
              </div>
            </div>
          </div>
          <div class="topology-stage">
            <div v-if="!topologyNodes.length" class="panel-empty">当前业务暂无拓扑数据</div>
            <div ref="topologyContainer" class="topology-canvas"></div>
          </div>
        </section>

        <aside ref="monitorSection" class="issue-panel">
          <div class="issue-heading">
            <div><h3>{{ selectedError ? '请求日志详情' : (selectedNode ? '异常节点分析' : '问题详情') }}</h3><span>{{ selectedNode ? selectedNodeEvidence.length : recentErrorRows.length }} 条依据</span></div>
            <button v-if="selectedError || selectedNode" type="button" aria-label="关闭详情" @click="clearSelection"><el-icon><Close /></el-icon></button>
          </div>

          <div v-if="selectedError" class="issue-detail">
            <div class="issue-title"><span class="error-code">{{ selectedError.__storeId ? '异常日志' : `请求失败 ${errorStatusCode(selectedError) || '异常'}` }}</span></div>
            <section class="detail-section">
              <h4>{{ selectedError.__storeId ? '日志摘要' : '错误摘要' }}</h4>
              <p class="error-summary">{{ selectedError.__storeId ? logLevel(selectedError) : `${errorMethod(selectedError)} ${errorApiPath(selectedError)}` }}</p>
              <p>{{ errorMessageText(selectedError) }}</p>
            </section>
            <section class="detail-section">
              <h4>影响范围</h4>
              <div class="impact-tags">
                <button v-for="node in selectedErrorImpactNodes" :key="node.id" type="button" @click="selectNode(node)">{{ node.displayName || node.label || node.name }}</button>
                <span v-if="!selectedErrorImpactNodes.length">暂无关联资源</span>
              </div>
            </section>
            <section class="detail-section detail-fields">
              <h4>{{ selectedError.__storeId ? '日志信息' : '请求信息' }}</h4>
              <dl>
                <div><dt>发生时间</dt><dd>{{ formatDateTime(errorTime(selectedError)) }}</dd></div>
                <div><dt>响应时间</dt><dd>{{ formatLatency(selectedError.latencyMs ?? selectedError.durationMs ?? selectedError.responseTimeMs) }}</dd></div>
                <div><dt>Request ID</dt><dd>{{ selectedError.requestId || selectedError.requestID || '-' }}</dd></div>
                <div><dt>Trace ID</dt><dd>{{ selectedError.traceId || selectedError.traceID || '-' }}</dd></div>
                <div><dt>日志来源</dt><dd>{{ selectedError.storeCode || selectedError.__storeCode || selectedError.__storeName || '-' }}</dd></div>
              </dl>
            </section>
          </div>

          <div v-else-if="selectedNode" class="issue-detail node-detail">
            <div class="issue-title">
              <span :class="['node-status-label', statusClass(nodeStatus(selectedNode))]">{{ statusText(nodeStatus(selectedNode)) }}</span>
              <span v-if="selectedNode.isRootCause" class="diagnosis-badge diagnosis-badge--root">根因</span>
              <span v-else-if="selectedNode.isAffected" class="diagnosis-badge diagnosis-badge--affected">受影响</span>
              <strong>{{ selectedNode.displayName || selectedNode.label || selectedNode.name }}</strong>
            </div>
            <div :class="['node-conclusion', statusClass(nodeStatus(selectedNode))]">
              <el-icon><component :is="nodeStatus(selectedNode) === 'healthy' ? CircleCheckFilled : WarningFilled" /></el-icon>
              <div><strong>{{ nodeConclusion(selectedNode) }}</strong><span>{{ nodeConclusionDetail(selectedNode) }}</span></div>
            </div>
            <section v-if="analysisMode !== 'health'" v-loading="nodeAnalysisLoading" class="node-analysis">
              <div class="analysis-judgement">
                <strong>分析判断</strong>
                <p>{{ analysisJudgement }}</p>
              </div>
              <div v-if="nodeAnalysisError" class="analysis-error">{{ nodeAnalysisError }}</div>
              <template v-else>
                <article v-for="(title, index) in analysisChartTitles" :key="title" class="analysis-chart-card">
                  <h4>{{ title }}</h4>
                  <div :ref="el => setAnalysisChartRef(index, el)" class="analysis-chart"></div>
                </article>
              </template>
            </section>
            <section class="detail-section detail-fields">
              <h4>判断依据</h4>
              <dl>
                <div><dt>资源类型</dt><dd>{{ resourceTypeText(selectedNode.resourceType) }}</dd></div>
                <div><dt>健康评分</dt><dd :class="{ 'danger-value': Number(selectedNode.displayHealthScore ?? selectedNode.healthScore) < 60 }">{{ selectedNode.displayHealthScore ?? selectedNode.healthScore ?? '-' }}</dd></div>
                <div><dt>异常请求</dt><dd :class="{ 'danger-value': nodeErrorCount(selectedNode) > 0 }">{{ nodeErrorCount(selectedNode) }}</dd></div>
                <div><dt>4xx / 5xx</dt><dd>{{ nodeHttpErrorText(selectedNode) }}</dd></div>
                <div><dt>请求量</dt><dd>{{ formatCompact(nodeMetric(selectedNode, ['requestTotal', 'requestCount', 'requests'])) }}</dd></div>
                <div><dt>平均响应</dt><dd>{{ formatLatency(nodeMetric(selectedNode, ['avgLatencyMs', 'averageLatencyMs'])) }}</dd></div>
              </dl>
            </section>
            <section class="detail-section node-reasons">
              <h4>异常原因 <span>{{ selectedNodeEvidence.length }}</span></h4>
              <article v-for="item in selectedNodeEvidence" :key="`${item.code || item.name}-${item.message}`" :class="statusClass(item.status)">
                <div><i></i><strong>{{ item.name || '健康检查' }}</strong></div>
                <p>{{ item.message || '检查结果异常' }}</p>
                <small v-if="item.diagnostic">建议：{{ item.diagnostic }}</small>
              </article>
              <p v-if="!selectedNodeEvidence.length" class="no-node-errors">该节点暂无明确异常原因</p>
            </section>
            <section class="detail-section bound-log-summary">
              <h4>关联日志</h4>
              <p v-if="selectedNodeLogSources.length">已关联 {{ selectedNodeLogSources.map(item => item.storeName || item.name || item.storeCode).join('、') }}，当前展示 {{ selectedNodeLogCount }} 条日志。</p>
              <p v-else>该节点未绑定日志，当前仅根据资源健康检查判断。</p>
            </section>
          </div>

          <div v-else class="issue-placeholder"><el-icon><Aim /></el-icon><strong>选择异常节点或 API 请求</strong><span>查看错误、请求标识和关联资源</span></div>
        </aside>

        <section ref="apiSection" class="api-panel">
          <div class="panel-heading api-heading">
            <div><h3>{{ logPanelTitle }}</h3><span class="count-badge">{{ displayedLogRows.length }}</span></div>
            <button v-if="selectedNode" type="button" class="view-all-command" @click="showAllLogs">查看全部</button>
          </div>
          <el-table v-if="httpLogTable" v-loading="nodeLogsLoading" :data="displayedLogRows.slice(0, 12)" height="196" size="small" class="error-table" empty-text="当前节点暂无 4xx / 5xx 日志" highlight-current-row @row-click="selectError">
            <el-table-column label="时间" width="98"><template #default="scope">{{ formatShortTime(errorTime(scope.row)) }}</template></el-table-column>
            <el-table-column label="方法" width="76"><template #default="scope"><span class="method-tag">{{ errorMethod(scope.row) }}</span></template></el-table-column>
            <el-table-column label="接口" min-width="240" show-overflow-tooltip><template #default="scope">{{ errorApiPath(scope.row) }}</template></el-table-column>
            <el-table-column label="状态" width="76"><template #default="scope"><span class="status-code">{{ errorStatusCode(scope.row) || '-' }}</span></template></el-table-column>
            <el-table-column label="响应时间" width="96"><template #default="scope">{{ formatLatency(scope.row.latencyMs ?? scope.row.durationMs ?? scope.row.responseTimeMs) }}</template></el-table-column>
            <el-table-column label="错误信息" min-width="170" show-overflow-tooltip><template #default="scope">{{ errorMessageText(scope.row) }}</template></el-table-column>
            <el-table-column label="服务" min-width="150" show-overflow-tooltip><template #default="scope">{{ errorResourceName(scope.row) }}</template></el-table-column>
          </el-table>
          <el-table v-else v-loading="nodeLogsLoading" :data="displayedLogRows.slice(0, 12)" height="196" size="small" class="error-table" :empty-text="genericLogEmptyText" highlight-current-row @row-click="selectError">
            <el-table-column label="时间" width="150"><template #default="scope">{{ formatDateTime(errorTime(scope.row)) }}</template></el-table-column>
            <el-table-column label="级别" width="82"><template #default="scope"><span class="log-level">{{ logLevel(scope.row) }}</span></template></el-table-column>
            <el-table-column label="日志内容" min-width="420" show-overflow-tooltip><template #default="scope">{{ errorMessageText(scope.row) }}</template></el-table-column>
            <el-table-column label="日志源" min-width="150" show-overflow-tooltip><template #default="scope">{{ scope.row.__storeName || scope.row.__storeCode || '-' }}</template></el-table-column>
          </el-table>
        </section>
      </main>
    </section>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts'
import {
  Aim, ArrowLeft, ArrowRight, Calendar, CircleCheck, CircleCheckFilled, Close, Filter, FullScreen, MoreFilled, Refresh, Search,
  Share, Timer, TrendCharts, Warning, WarningFilled, ZoomIn, ZoomOut
} from '@element-plus/icons-vue'
import G6 from '@antv/g6'
import { getBusinessObservability, listBusinessModels, refreshBusinessHealth } from '@/api/cmdbBusinessModel'
import cmdbApi from '@/api/cmdb'
import k8sApi from '@/api/k8s'
import logtailApi from '@/api/logtail'
import appIcon from '@/assets/image/应用.svg'
import cloudHostIcon from '@/assets/image/云主机服务器.svg'
import databaseIcon from '@/assets/image/数据库.svg'
import gatewayIcon from '@/assets/image/gateway.svg'
import k8sIcon from '@/assets/image/k8s.svg'
import linuxIcon from '@/assets/image/linux.svg'
import logIcon from '@/assets/image/日志.svg'
import middlewareIcon from '@/assets/image/监控.svg'
import mongoIcon from '@/assets/image/mongodb.svg'
import mysqlIcon from '@/assets/image/mysqldb.svg'
import namespaceIcon from '@/assets/image/命名空间.svg'
import nginxIcon from '@/assets/image/nginx.svg'
import physicalServerIcon from '@/assets/image/物理机.svg'
import redisIcon from '@/assets/image/redis.svg'
import virtualIcon from '@/assets/image/虚拟机.svg'
import workloadIcon from '@/assets/image/工作负载.svg'

const route = useRoute()
const router = useRouter()
const topologyContainer = ref(null)
const apiSection = ref(null)
const monitorSection = ref(null)
const topologyZoomLabel = ref('100%')
const loading = ref(false)
const refreshing = ref(false)
const loadError = ref('')
const selectedWindow = ref(String(route.query.window || '15m'))
const serviceKeyword = ref('')
const serviceFilter = ref('all')
const selectedNode = ref(null)
const selectedError = ref(null)
const nodeAnalysisLoading = ref(false)
const nodeAnalysisError = ref('')
const nodeAnalysisData = ref(null)
const nodeLogRows = ref([])
const nodeLogsLoading = ref(false)
const businessModels = ref([])
const observability = ref({
  summary: {}, metrics: {}, topology: { nodes: [], edges: [] }, nodeSignals: [], nodeAnalytics: {}, trafficTrend: [], topApis: [], recentErrors: [], activeAlerts: [], logSources: [], warnings: []
})

const formatWindowLabel = value => {
  const raw = String(value || '').trim().toLowerCase()
  if (raw.startsWith('近 ')) return value
  const labels = { '15m': '近 15 分钟', '30m': '近 30 分钟', '1h': '近 1 小时', '1h0m0s': '近 1 小时', '6h': '近 6 小时', '12h': '近 12 小时', '24h': '近 24 小时', '7d': '近 7 天', '30d': '近 30 天' }
  if (labels[raw]) return labels[raw]
  const hours = raw.match(/^(\d+)h(?:0m0s)?$/)
  if (hours) {
    const count = Number(hours[1])
    return count >= 24 && count % 24 === 0 ? `近 ${count / 24} 天` : `近 ${count} 小时`
  }
  const minutes = raw.match(/^(\d+)m(?:0s)?$/)
  if (minutes) return `近 ${Number(minutes[1])} 分钟`
  return raw === 'custom' ? '自定义时间' : (raw ? `近 ${raw}` : '当前窗口')
}

let topologyGraph = null
let resizeObserver = null
let graphNodeRegistered = false
const analysisChartRefs = []
const analysisCharts = []

const businessId = computed(() => route.params.id)
const summary = computed(() => observability.value.summary || {})
const metrics = computed(() => observability.value.metrics || {})
const businessTitle = computed(() => summary.value.displayName || summary.value.name || currentBusiness.value?.displayName || currentBusiness.value?.name || `业务 ${businessId.value}`)
const currentBusiness = computed(() => businessModels.value.find(item => String(item.id) === String(businessId.value)))
const topologyNodes = computed(() => (observability.value.topology?.nodes || []).filter(node => node.nodeRole !== 'observability' && !['log_store', 'log_project'].includes(node.resourceType)))
const topologyNodeIds = computed(() => new Set(topologyNodes.value.map(node => String(node.id))))
const topologyEdges = computed(() => (observability.value.topology?.edges || []).filter(edge => topologyNodeIds.value.has(String(edge.source ?? edge.sourceId ?? edge.srcId)) && topologyNodeIds.value.has(String(edge.target ?? edge.targetId ?? edge.dstId))))
const nodeSignals = computed(() => observability.value.nodeSignals || [])
const recentErrorRows = computed(() => observability.value.recentErrors || [])
const summaryStatus = computed(() => normalizeStatus(summary.value.overallStatus || summary.value.displayHealthStatus || summary.value.healthStatus))
const windowLabel = computed(() => formatWindowLabel(observability.value.window?.label || selectedWindow.value))

const windowOptions = [
  { label: '近 15 分钟', value: '15m' },
  { label: '近 30 分钟', value: '30m' },
  { label: '近 1 小时', value: '1h' },
  { label: '近 6 小时', value: '6h' },
  { label: '近 12 小时', value: '12h' },
  { label: '近 24 小时', value: '24h' },
  { label: '近 7 天', value: '7d' },
  { label: '近 30 天', value: '30d' }
]

const statusColors = { healthy: '#16a34a', warning: '#d97706', critical: '#dc2626', unknown: '#94a3b8' }
const statusLabels = { healthy: '正常', warning: '风险', critical: '异常', unknown: '未接入' }
const serviceTypes = new Set(['nginx', 'gateway', 'business_service', 'application', 'k8s_workload', 'pod'])
const resourceTypeMap = {
  business_system: '业务系统', business_service: '业务服务', application: '应用', nginx: 'Nginx', gateway: 'Gateway',
  host: '主机', physical_server: '物理机', vm: '虚拟机', virtual: '虚拟机', cloud_host: '云主机',
  k8s_cluster: 'K8S 集群', k8s_namespace: '命名空间', k8s_workload: '工作负载', pod: 'Pod',
  database: '数据库', middleware: '中间件', redis: 'Redis', kafka: 'Kafka'
}

const serviceNodes = computed(() => topologyNodes.value.filter(node => serviceTypes.has(node.resourceType)))
const filteredServiceNodes = computed(() => {
  const keyword = serviceKeyword.value.trim().toLowerCase()
  return serviceNodes.value.filter(node => {
    const matchesKeyword = !keyword || [node.displayName, node.label, node.name, node.sourceKey].filter(Boolean).some(value => String(value).toLowerCase().includes(keyword))
    const matchesStatus = serviceFilter.value === 'all' || nodeIsActionableProblem(node)
    return matchesKeyword && matchesStatus
  })
})

const filteredBusinessModels = computed(() => {
  const keyword = serviceKeyword.value.trim().toLowerCase()
  if (!keyword) return businessModels.value
  return businessModels.value.filter(item => {
    if ([item.displayName, item.name].filter(Boolean).some(value => String(value).toLowerCase().includes(keyword))) return true
    return String(item.id) === String(businessId.value) && filteredServiceNodes.value.length > 0
  })
})

const kpiItems = computed(() => [
  { key: 'requests', icon: TrendCharts, label: '请求量', value: formatCompact(metricValue(['requestTotal', 'requestCount', 'requests'])), caption: windowLabel.value, tone: '' },
  { key: 'errors', icon: Warning, label: '系统错误率', value: formatPercent(metricValue(['systemErrorRate', 'http5xxRate'])), caption: `5xx ${formatCompact(metricValue(['http5xx', 'serverErrorTotal']))} / 4xx 质量 ${formatPercent(metricValue(['http4xxRate']))}`, tone: metricValue(['systemErrorRate', 'http5xxRate']) >= 5 ? 'critical-text' : (metricValue(['systemErrorRate', 'http5xxRate']) >= 1 ? 'warning-text' : '') },
  { key: 'latency', icon: Timer, label: '平均响应', value: formatLatency(metricValue(['avgLatencyMs', 'averageLatencyMs'])), caption: `P95 ${formatLatency(metricValue(['p95LatencyMs', 'latencyP95Ms']))}`, tone: '' },
  { key: 'success', icon: CircleCheck, label: '可用性', value: formatPercent(metricValue(['availabilityRate', 'successRate'])), caption: `SLO ${formatPercent(metricValue(['sloTarget', 'slaTarget'], 99.9))}`, tone: metricValue(['availabilityRate', 'successRate']) >= metricValue(['sloTarget', 'slaTarget'], 99.9) ? 'healthy-text' : 'warning-text' }
])

const degradationMessages = computed(() => {
  const messages = [...(observability.value.warnings || [])].map(item => typeof item === 'string' ? item : item.message || item.name).filter(Boolean)
  const dataStatus = String(observability.value.dataStatus || '').toLowerCase()
  if (observability.value.partial && !messages.length) messages.push('部分数据源查询失败，当前展示可用数据')
  if (dataStatus && !['ready', 'healthy', 'success', 'ok'].includes(dataStatus)) messages.push(`数据状态：${observability.value.dataStatus}`)
  ;(observability.value.logSources || []).forEach(source => {
    const rawState = source.queryStatus ?? source.dataStatus ?? source.status
    const state = Number(rawState) === 1 ? 'ready' : String(rawState || '').toLowerCase()
    if (state && !['ready', 'healthy', 'success', 'active', 'ok'].includes(state)) messages.push(`${source.name || source.storeName || source.sourceKey || '日志源'}：${source.message || state}`)
  })
  return [...new Set(messages)]
})

const signalForNode = node => nodeSignals.value.find(item =>
  String(item.nodeId ?? item.resourceId ?? item.id ?? '') === String(node?.id ?? '') ||
  (item.sourceKey && node?.sourceKey && item.sourceKey === node.sourceKey)
)

const normalizeStatus = value => {
  const status = String(value || '').toLowerCase()
  if (['healthy', 'normal', 'green', 'ok', 'ready'].includes(status)) return 'healthy'
  if (['warning', 'yellow', 'degraded'].includes(status)) return 'warning'
  if (['critical', 'red', 'error', 'firing'].includes(status)) return 'critical'
  return 'unknown'
}
const nodeStatus = node => normalizeStatus(node?.overallStatus || node?.displayHealthStatus || node?.healthStatus || node?.status)
const statusClass = status => `status-${normalizeStatus(status)}`
const statusText = status => statusLabels[normalizeStatus(status)]
const resourceTypeText = type => resourceTypeMap[type] || type || '资源'
const metricValue = (keys, fallback = 0) => {
  for (const key of keys) {
    const value = Number(metrics.value?.[key])
    if (Number.isFinite(value)) return value
  }
  return fallback
}
const formatCompact = value => {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return '--'
  return new Intl.NumberFormat('zh-CN', { notation: Math.abs(numeric) >= 10000 ? 'compact' : 'standard', maximumFractionDigits: 2 }).format(numeric)
}
const formatPercent = value => {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return '--'
  const normalized = numeric > 0 && numeric <= 1 ? numeric * 100 : numeric
  return `${normalized.toFixed(2).replace(/\.00$/, '')}%`
}
const formatLatency = value => {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return '--'
  return numeric >= 1000 ? `${(numeric / 1000).toFixed(2)}s` : `${Math.round(numeric)}ms`
}
const toValidDate = value => {
  if (!value) return null
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}
const formatDateTime = value => toValidDate(value)?.toLocaleString('zh-CN', { hour12: false }) || '--'
const formatShortTime = value => toValidDate(value)?.toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' }) || '--'

const nodeMetric = (node, keys) => {
  const signal = signalForNode(node)
  const analytics = observability.value.nodeAnalytics?.[String(node?.id)]?.metrics
  const sources = [analytics, signal, signal?.observability, node?.observability]
  for (const source of sources) {
    for (const key of keys) {
      const value = Number(source?.[key])
      if (Number.isFinite(value)) return value
    }
  }
  return 0
}
const nodeErrorCount = node => {
  const explicit = nodeMetric(node, ['errorCount', 'serverErrorCount', 'error5xxTotal', 'status5xx', 'errors'])
  if (explicit > 0) return explicit
  return nodeMetric(node, ['http4xx']) + nodeMetric(node, ['http5xx'])
}
const nodeIsActionableProblem = node => ['critical', 'warning'].includes(nodeStatus(node))
const nodeHttpErrorText = node => `${formatCompact(nodeMetric(node, ['http4xx']))} / ${formatCompact(nodeMetric(node, ['http5xx']))}`
const nodeConclusion = node => {
  const status = nodeStatus(node)
  if (status === 'critical') return '该节点当前存在故障'
  if (status === 'warning') return '该节点存在风险，需要关注'
  if (status === 'healthy') return '该节点运行正常'
  return '该节点暂未获得完整状态'
}
const nodeConclusionDetail = node => {
  const errors = nodeErrorCount(node)
  if (node?.problemSummary && nodeStatus(node) !== 'healthy') return node.problemSummary
  if (errors > 0) return `${windowLabel.value}检测到 ${formatCompact(errors)} 个异常请求`
  if (nodeStatus(node) === 'healthy') return `${windowLabel.value}未检测到业务异常`
  return '请检查资源监控和日志数据是否完整'
}
const errorTime = item => item?.timestamp || item?.['@timestamp'] || item?._time || item?.time || item?.occurredAt || item?.createdAt
const errorMethod = item => item?.requestMethod || item?.method || 'HTTP'
const errorApiPath = item => item?.path || item?.api || item?.endpoint || item?.requestPath || '-'
const errorStatusCode = item => item?.statusCode || item?.status || item?.httpStatus
const errorMessageText = item => item?.message || item?._msg || item?.msg || item?.log || item?.errorMessage || item?.summary || `HTTP ${errorStatusCode(item) || '请求异常'}`
const logLevel = item => String(item?.level || item?.severity || item?.log_level || item?.status_text || 'ERROR').toUpperCase()
const errorResourceName = item => {
  const directName = item?.resourceName || item?.nodeName || item?.serviceName
  if (directName) return directName
  const directNode = topologyNodes.value.find(node => String(node.id) === String(item?.nodeId || item?.resourceId))
  if (directNode) return directNode.displayName || directNode.label || directNode.name
  const storeId = item?.storeId || item?.logStoreId || item?.sourceStoreId || item?.__storeId
  const storeCode = item?.storeCode || item?.logStoreCode || item?.sourceKey || item?.__storeCode
  const logSource = (observability.value.logSources || []).find(source =>
    (storeId && String(source.storeId || source.sourceId || source.id) === String(storeId)) ||
    (storeCode && String(source.storeCode || source.code || source.sourceKey) === String(storeCode))
  )
  const names = (logSource?.boundNodeIds || [])
    .map(nodeId => topologyNodes.value.find(node => String(node.id) === String(nodeId)))
    .filter(Boolean)
    .map(node => node.displayName || node.label || node.name)
  return names.length ? names.join('、') : '-'
}

const selectedErrorImpactNodes = computed(() => {
  if (!selectedError.value) return []
  const directId = selectedError.value.nodeId || selectedError.value.resourceId
  if (directId) return topologyNodes.value.filter(node => String(node.id) === String(directId))
  const storeId = selectedError.value.storeId || selectedError.value.logStoreId || selectedError.value.sourceStoreId || selectedError.value.__storeId
  const storeCode = selectedError.value.storeCode || selectedError.value.logStoreCode || selectedError.value.sourceKey || selectedError.value.__storeCode
  const source = (observability.value.logSources || []).find(item =>
    (storeId && String(item.storeId || item.sourceId || item.id) === String(storeId)) ||
    (storeCode && String(item.storeCode || item.code || item.sourceKey) === String(storeCode))
  )
  const ids = new Set((source?.boundNodeIds || []).map(String))
  return topologyNodes.value.filter(node => ids.has(String(node.id)))
})

const errorMatchesNode = (item, node) => {
  if (!item || !node) return false
  const directId = item.nodeId || item.resourceId
  if (directId && String(directId) === String(node.id)) return true
  const storeId = item.storeId || item.logStoreId || item.sourceStoreId
  const storeCode = item.storeCode || item.logStoreCode || item.sourceKey
  const source = (observability.value.logSources || []).find(candidate =>
    (storeId && String(candidate.storeId || candidate.sourceId || candidate.id) === String(storeId)) ||
    (storeCode && String(candidate.storeCode || candidate.code || candidate.sourceKey) === String(storeCode))
  )
  return (source?.boundNodeIds || []).some(nodeId => String(nodeId) === String(node.id))
}
const selectedNodeAnalytics = computed(() => selectedNode.value ? observability.value.nodeAnalytics?.[String(selectedNode.value.id)] || null : null)
const selectedNodeErrors = computed(() => {
  if (!selectedNode.value) return []
  if (Array.isArray(selectedNodeAnalytics.value?.recentErrors)) return selectedNodeAnalytics.value.recentErrors
  return recentErrorRows.value.filter(item => errorMatchesNode(item, selectedNode.value))
})
const selectedNodeLogSources = computed(() => {
  if (!selectedNode.value) return []
  const nodeId = String(selectedNode.value.id)
  const signalStoreIds = new Set((signalForNode(selectedNode.value)?.boundLogStoreIds || []).map(String))
  const properties = normalizeProperties(selectedNode.value.properties || selectedNode.value.Properties)
  const directStoreId = properties.storeId || (selectedNode.value.sourceType === 'log_store' ? selectedNode.value.sourceId : null)
  const hasDirectMetadata = (observability.value.logSources || []).some(source => Array.isArray(source.directNodeIds))
  return (observability.value.logSources || []).filter(source =>
    (Array.isArray(source.directNodeIds) ? source.directNodeIds : (source.boundNodeIds || [])).some(id => String(id) === nodeId) ||
    (!hasDirectMetadata && signalStoreIds.has(String(source.storeId || source.sourceId || source.id))) ||
    (directStoreId && String(source.storeId || source.sourceId || source.id) === String(directStoreId))
  )
})
const reasonEvidence = (code, node) => {
  const statusBySignal = {
    HTTP_5XX_RATE_HIGH: node?.trafficStatus,
    HTTP_4XX_RATE_HIGH: node?.trafficStatus,
    P95_LATENCY_HIGH: node?.trafficStatus,
    LOG_DATA_UNAVAILABLE: 'warning',
    LOG_DATA_PARTIAL: 'warning',
    ACTIVE_ALERT: node?.alertStatus,
    RESOURCE_HEALTH_DEGRADED: node?.resourceStatus,
    AFFECTED_BY_RELATED_RESOURCE: 'warning'
  }
  const evidence = {
    HTTP_5XX_RATE_HIGH: { name: '5xx 错误率过高', message: `服务端错误率 ${formatPercent(nodeMetric(node, ['http5xxRate']))}，业务可用性已受影响`, diagnostic: '优先检查上游服务、网关和应用错误日志' },
    HTTP_4XX_RATE_HIGH: { name: '4xx 请求异常', message: `客户端请求异常 ${formatCompact(nodeMetric(node, ['http4xx']))} 次，属于请求质量问题`, diagnostic: '检查接口参数、鉴权、路由和调用方版本' },
    P95_LATENCY_HIGH: { name: '接口响应过慢', message: `P95 响应时间 ${formatLatency(nodeMetric(node, ['p95LatencyMs']))}`, diagnostic: '结合慢接口 Top 5 和资源趋势定位瓶颈' },
    LOG_DATA_UNAVAILABLE: { name: '日志数据不可用', message: '已绑定日志源，但当前时间窗口查询失败', diagnostic: '检查日志源连接和索引状态' },
    LOG_DATA_PARTIAL: { name: '日志数据不完整', message: '部分日志源未返回完整数据', diagnostic: '当前判断仅基于已返回的数据' },
    ACTIVE_ALERT: { name: '存在活动告警', message: '该节点关联的日志告警正在触发', diagnostic: '查看活动告警和对应异常日志' },
    RESOURCE_HEALTH_DEGRADED: { name: '资源健康异常', message: node?.problemSummary || '资源监控指标已超过健康阈值', diagnostic: '查看右侧资源趋势确认异常指标' },
    AFFECTED_BY_RELATED_RESOURCE: { name: '受关联资源影响', message: '当前节点并非直接根因，请沿高亮链路检查异常节点', diagnostic: '优先处理标记为根因的下游节点' }
  }[code]
  if (!evidence) return null
  return { code, status: normalizeStatus(statusBySignal[code]) === 'unknown' ? 'warning' : statusBySignal[code], ...evidence }
}

const selectedNodeEvidence = computed(() => {
  const node = selectedNode.value
  if (!node) return []
  const checks = (node.evidence?.checks || node.checks || [])
    .filter(item => ['critical', 'warning'].includes(normalizeStatus(item.status)))
    .map(item => ({ ...item, code: item.code || item.checkCode }))
  const signal = signalForNode(node)
  const reasonCodes = [...(node.reasonCodes || []), ...(node.observability?.reasonCodes || []), ...(signal?.reasonCodes || [])]
  const combined = [...checks, ...reasonCodes.map(code => reasonEvidence(code, node)).filter(Boolean)]
  const unique = new Map()
  combined.forEach(item => {
    const key = item.code || `${item.name || ''}:${item.message || ''}`
    if (!unique.has(key)) unique.set(key, item)
  })
  return [...unique.values()]
})
const hostResourceTypes = new Set(['host', 'physical_server', 'vm', 'virtual', 'cloud_host'])
const analysisMode = computed(() => {
  const node = selectedNode.value
  if (!node) return 'health'
  if (['nginx', 'gateway'].includes(node.resourceType)) return 'http'
  if (['k8s_workload', 'pod'].includes(node.resourceType)) return 'pod'
  if (hostResourceTypes.has(node.resourceType)) return 'host'
  const properties = normalizeProperties(node.properties || node.Properties)
  if (['business_service', 'application'].includes(node.resourceType) && (properties.deployMode === 'host' || properties.hostId)) return 'service'
  return 'health'
})
const analysisChartTitles = computed(() => ({
  http: ['请求状态趋势', '响应较慢接口 Top 5', '接口请求分布'],
  pod: ['CPU 使用率', '内存使用率', '容器重启次数'],
  host: ['CPU / 内存使用率', '磁盘 IO', '网络带宽'],
  service: ['服务 CPU 使用率', '服务内存使用率']
})[analysisMode.value] || [])
const displayedLogRows = computed(() => {
  if (!selectedNode.value) return recentErrorRows.value
  return analysisMode.value === 'http' ? selectedNodeErrors.value : nodeLogRows.value
})
const selectedNodeLogCount = computed(() => analysisMode.value === 'http' ? selectedNodeErrors.value.length : nodeLogRows.value.length)
const httpLogTable = computed(() => !selectedNode.value || analysisMode.value === 'http')
const logPanelTitle = computed(() => {
  if (!selectedNode.value) return '最近 4xx / 5xx 日志'
  const name = selectedNode.value.displayName || selectedNode.value.label || selectedNode.value.name
  if (analysisMode.value === 'http') return `${name} · 最近 4xx / 5xx 日志`
  if (analysisMode.value === 'host') return `${name} · 最近系统日志`
  return `${name} · 最近 error / timeout 日志`
})
const genericLogEmptyText = computed(() => analysisMode.value === 'host' ? '当前主机暂无系统日志' : '当前节点暂无 error / timeout 日志')
const metricPointMax = points => Math.max(0, ...(points || []).map(item => Number(item?.value) || 0))
const podChart = key => (nodeAnalysisData.value?.charts || []).find(item => item.key === key)
const podChartMax = key => Math.max(0, ...(podChart(key)?.series || []).flatMap(item => item.points || []).map(item => Number(item?.value) || 0))
const analysisJudgement = computed(() => {
  if (!selectedNode.value) return '-'
  if (nodeAnalysisLoading.value) return '正在加载节点观测数据...'
  if (nodeAnalysisError.value) return nodeAnalysisError.value
  if (analysisMode.value === 'http') {
    const metrics = nodeAnalysisData.value?.metrics || signalForNode(selectedNode.value) || {}
    const topSlow = [...(nodeAnalysisData.value?.topApis || [])].sort((a, b) => Number(b.p95LatencyMs) - Number(a.p95LatencyMs))[0]
    const slowText = topSlow ? `最慢接口 ${topSlow.requestPath}，P95 ${formatLatency(topSlow.p95LatencyMs)}` : '暂无慢接口数据'
    return `请求 ${formatCompact(metrics.requestTotal)}，4xx ${formatPercent(metrics.http4xxRate || 0)}，5xx ${formatPercent(metrics.http5xxRate || 0)}；${slowText}。`
  }
  if (analysisMode.value === 'pod') {
    return `窗口内 CPU 峰值 ${podChartMax('cpuUsageRate').toFixed(2)}%，内存峰值 ${podChartMax('memoryUsageRate').toFixed(2)}%，重启计数峰值 ${podChartMax('restartCount').toFixed(0)} 次。${selectedNode.value.problemSummary || ''}`
  }
  if (analysisMode.value === 'host') {
    const data = nodeAnalysisData.value || {}
    return `整机 CPU 峰值 ${metricPointMax(data.cpu).toFixed(2)}%，内存峰值 ${metricPointMax(data.memory).toFixed(2)}%，磁盘与网络曲线用于判断资源瓶颈。`
  }
  if (analysisMode.value === 'service') {
    const data = nodeAnalysisData.value || {}
    return `服务进程 CPU 峰值 ${metricPointMax(data.cpu).toFixed(2)}%，内存峰值 ${metricPointMax(data.memory).toFixed(2)}%。趋势按绑定的服务进程名称聚合。`
  }
  return selectedNode.value.problemSummary || '暂无进一步分析'
})

const businessModelStatus = item => String(item.id) === String(businessId.value)
  ? summaryStatus.value
  : normalizeStatus(item.overallStatus || item.displayHealthStatus || item.healthStatus)

const businessStatusCount = item => {
  if (String(item.id) === String(businessId.value)) {
    const abnormal = serviceNodes.value.filter(nodeIsActionableProblem).length
    return `${abnormal} / ${serviceNodes.value.length}`
  }
  return businessModelStatus(item) === 'healthy' ? '0' : '1'
}

const toggleAbnormalFilter = () => {
  serviceFilter.value = serviceFilter.value === 'abnormal' ? 'all' : 'abnormal'
}

const selectNode = async node => {
  selectedNode.value = node
  selectedError.value = null
  if (topologyGraph) {
    topologyGraph.getNodes().forEach(item => topologyGraph.clearItemStates(item, ['selected']))
    const graphNode = topologyGraph.findById(String(node.id))
    if (graphNode) {
      topologyGraph.setItemState(graphNode, 'selected', true)
      topologyGraph.focusItem(graphNode, true, { duration: 250 })
    }
  }
  await loadSelectedNodeData(node)
}
const selectError = item => {
  selectedError.value = item
  const node = topologyNodes.value.find(candidate => String(candidate.id) === String(item?.nodeId || item?.resourceId))
  if (node) selectNode(node)
  selectedError.value = item
}
const clearSelection = () => {
  selectedNode.value = null
  selectedError.value = null
  nodeAnalysisData.value = null
  nodeAnalysisError.value = ''
  nodeLogRows.value = []
  disposeAnalysisCharts()
  topologyGraph?.getNodes().forEach(item => topologyGraph.clearItemStates(item, ['selected']))
}

const setAnalysisChartRef = (index, element) => {
  if (element) analysisChartRefs[index] = element
}

const disposeAnalysisCharts = () => {
  analysisCharts.forEach(chart => chart?.dispose())
  analysisCharts.length = 0
}

const analysisChart = index => {
  const element = analysisChartRefs[index]
  if (!element) return null
  if (!analysisCharts[index] || analysisCharts[index].isDisposed()) analysisCharts[index] = echarts.init(element, null, { renderer: 'canvas' })
  return analysisCharts[index]
}

const pointTime = value => {
  const numeric = Number(value)
  const date = Number.isFinite(numeric) ? new Date(numeric > 1e12 ? numeric : numeric * 1000) : new Date(value)
  return Number.isNaN(date.getTime()) ? String(value || '') : date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

const lineChartOption = (series, unit = '') => {
  const timestamps = [...new Set(series.flatMap(item => (item.points || []).map(point => point.timestamp)))].sort((a, b) => new Date(a).getTime() - new Date(b).getTime())
  const valueMaps = series.map(item => new Map((item.points || []).map(point => [point.timestamp, Number(point.value) || 0])))
  return {
    animation: false,
    color: series.map(item => item.color),
    tooltip: { trigger: 'axis', appendToBody: true, confine: false, extraCssText: 'z-index: 10000;', valueFormatter: value => `${Number(value).toFixed(2)}${unit}` },
    legend: { top: 0, left: 0, itemWidth: 10, itemHeight: 5, textStyle: { fontSize: 9, color: '#64748b' } },
    grid: { top: 28, right: 10, bottom: 22, left: 39 },
    xAxis: { type: 'category', data: timestamps.map(pointTime), boundaryGap: false, axisLabel: { fontSize: 8, color: '#8a96a8', hideOverlap: true }, axisLine: { lineStyle: { color: '#dce3ed' } } },
    yAxis: { type: 'value', name: unit, nameTextStyle: { fontSize: 8 }, axisLabel: { fontSize: 8, color: '#8a96a8' }, splitLine: { lineStyle: { color: '#edf1f6' } } },
    series: series.map((item, index) => ({ name: item.name, type: 'line', smooth: true, symbol: 'none', lineStyle: { width: 1.7 }, areaStyle: index === 0 ? { opacity: 0.06 } : undefined, data: timestamps.map(timestamp => valueMaps[index].get(timestamp) ?? null) }))
  }
}

const renderLineAnalysisChart = (index, series, unit) => analysisChart(index)?.setOption(lineChartOption(series, unit), true)

const mergedPodPoints = (key, strategy = 'average') => {
  const series = podChart(key)?.series || []
  const buckets = new Map()
  series.forEach(item => (item.points || []).forEach(point => {
    const bucket = buckets.get(point.timestamp) || []
    bucket.push(Number(point.value) || 0)
    buckets.set(point.timestamp, bucket)
  }))
  return [...buckets.entries()].map(([timestamp, values]) => ({
    timestamp,
    value: strategy === 'sum' ? values.reduce((total, value) => total + value, 0) : values.reduce((total, value) => total + value, 0) / values.length
  }))
}

const renderHTTPAnalysis = () => {
  const data = nodeAnalysisData.value || {}
  const trend = data.trafficTrend || []
  renderLineAnalysisChart(0, [
    { name: '200', color: '#16a34a', points: trend.map(item => ({ timestamp: item.timestamp, value: Number(item.http200) || 0 })) },
    { name: '4xx', color: '#d97706', points: trend.map(item => ({ timestamp: item.timestamp, value: item.http4xx })) },
    { name: '5xx', color: '#dc2626', points: trend.map(item => ({ timestamp: item.timestamp, value: item.http5xx })) }
  ], '')
  const slow = [...(data.topApis || [])].sort((a, b) => Number(b.p95LatencyMs) - Number(a.p95LatencyMs)).slice(0, 5)
  analysisChart(1)?.setOption({
    animation: false,
    tooltip: { trigger: 'axis', appendToBody: true, confine: false, extraCssText: 'z-index: 10000;', axisPointer: { type: 'shadow' }, valueFormatter: value => `${Number(value).toFixed(2)}s` },
    grid: { top: 8, right: 42, bottom: 20, left: 92 },
    xAxis: { type: 'value', axisLabel: { fontSize: 8, formatter: value => `${value}s` }, splitLine: { lineStyle: { color: '#edf1f6' } } },
    yAxis: { type: 'category', inverse: true, data: slow.map(item => item.requestPath), axisLabel: { width: 78, overflow: 'truncate', fontSize: 8 } },
    series: [{ type: 'bar', data: slow.map(item => (Number(item.p95LatencyMs) || 0) / 1000), barWidth: 9, label: { show: true, position: 'right', fontSize: 8, formatter: params => `${Number(params.value).toFixed(2)}s` }, itemStyle: { color: '#ef8b35', borderRadius: [0, 2, 2, 0] } }]
  }, true)
  const distribution = [...(data.topApis || [])].sort((a, b) => Number(b.requestTotal) - Number(a.requestTotal)).slice(0, 7)
  analysisChart(2)?.setOption({
    animation: false,
    tooltip: { trigger: 'item', appendToBody: true, confine: false, extraCssText: 'z-index: 10000;' },
    legend: { type: 'scroll', orient: 'vertical', right: 2, top: 10, bottom: 8, width: 95, textStyle: { fontSize: 8 }, formatter: name => name.length > 16 ? `${name.slice(0, 16)}...` : name },
    series: [{ type: 'pie', radius: ['35%', '64%'], center: ['32%', '52%'], label: { show: false }, data: distribution.map(item => ({ name: item.requestPath, value: item.requestTotal })) }]
  }, true)
}

const renderNodeAnalysis = async () => {
  await nextTick()
  disposeAnalysisCharts()
  await nextTick()
  if (!selectedNode.value || nodeAnalysisError.value) return
  if (analysisMode.value === 'http') return renderHTTPAnalysis()
  if (analysisMode.value === 'pod') {
    renderLineAnalysisChart(0, [{ name: 'CPU', color: '#2563eb', points: mergedPodPoints('cpuUsageRate') }], '%')
    renderLineAnalysisChart(1, [{ name: '内存', color: '#7c3aed', points: mergedPodPoints('memoryUsageRate') }], '%')
    renderLineAnalysisChart(2, [{ name: '重启', color: '#dc2626', points: mergedPodPoints('restartCount', 'sum') }], '次')
    return
  }
  const data = nodeAnalysisData.value || {}
  if (analysisMode.value === 'host') {
    renderLineAnalysisChart(0, [{ name: 'CPU', color: '#2563eb', points: data.cpu || [] }, { name: '内存', color: '#7c3aed', points: data.memory || [] }], '%')
    renderLineAnalysisChart(1, [{ name: '读取', color: '#0891b2', points: data.diskReadKB || [] }, { name: '写入', color: '#d97706', points: data.diskWriteKB || [] }], 'KB/s')
    renderLineAnalysisChart(2, [{ name: '接收', color: '#16a34a', points: data.networkReceive || [] }, { name: '发送', color: '#dc5a35', points: data.networkSend || [] }], 'KB/s')
    return
  }
  if (analysisMode.value === 'service') {
    renderLineAnalysisChart(0, [{ name: 'CPU', color: '#2563eb', points: data.cpu || [] }], '%')
    renderLineAnalysisChart(1, [{ name: '内存', color: '#7c3aed', points: data.memory || [] }], '%')
  }
}

const normalizeProperties = value => {
  if (!value) return {}
  if (typeof value !== 'string') return value
  try {
    return JSON.parse(value)
  } catch (error) {
    return {}
  }
}

const extractAPIData = response => response?.data?.data ?? response?.data ?? response
const monitorDuration = () => ['30m', '1h', '3h', '6h', '12h', '24h'].includes(selectedWindow.value) ? selectedWindow.value : (selectedWindow.value === '15m' ? '30m' : '24h')

const resolvePodAnalysisContext = async node => {
  const properties = normalizeProperties(node?.properties || node?.Properties)
  const checks = node?.evidence?.checks || []
  const evidenceItems = checks.map(item => item?.evidence || {}).filter(Boolean)
  const clusterId = properties.clusterId || evidenceItems.find(item => item.clusterId)?.clusterId
  const namespace = properties.namespace || properties.namespaceName || evidenceItems.find(item => item.namespace)?.namespace
  let podName = properties.podName || evidenceItems.flatMap(item => item.abnormalPods || []).find(item => item?.name)?.name
  if (!podName && node?.resourceType === 'pod') podName = node.name || node.displayName
  if (!podName && clusterId && namespace) {
    const workloadName = properties.workloadName || node.name || node.displayName
    try {
      const response = await k8sApi.getPodList(clusterId, namespace, { keyword: workloadName, page: 1, pageSize: 50 })
      const data = extractAPIData(response) || {}
      const pods = Array.isArray(data) ? data : (data.pods || data.list || data.items || data.records || [])
      podName = pods.find(item => String(item.name || '').startsWith(`${workloadName}-`))?.name || pods[0]?.name
    } catch (error) {
      console.warn('获取工作负载 Pod 列表失败:', error)
    }
  }
  return { clusterId, namespace, podName }
}

const loadNodeAnalysis = async node => {
  const mode = analysisMode.value
  nodeAnalysisData.value = null
  nodeAnalysisError.value = ''
  if (mode === 'health') return
  nodeAnalysisLoading.value = true
  try {
    if (mode === 'http') {
      nodeAnalysisData.value = observability.value.nodeAnalytics?.[String(node.id)] || null
      if (!nodeAnalysisData.value) throw new Error('当前接口未返回该节点的独立日志聚合数据')
    } else if (mode === 'pod') {
      const context = await resolvePodAnalysisContext(node)
      if (!context.clusterId || !context.namespace || !context.podName) throw new Error('节点缺少 clusterId、namespace 或 Pod 名称')
      const response = await k8sApi.getPodPrometheusMetrics(context.clusterId, context.namespace, context.podName, { range: selectedWindow.value, scope: 'workload' })
      nodeAnalysisData.value = extractAPIData(response)
    } else {
      const properties = normalizeProperties(node.properties || node.Properties)
      const hostId = properties.hostId || properties.assetId || properties.instanceId || (hostResourceTypes.has(node.resourceType) ? node.sourceId : null)
      if (!hostId) throw new Error('节点缺少关联主机 ID')
      if (mode === 'host') {
        const response = await cmdbApi.getHostAllMetricsHistory(hostId, { duration: monitorDuration() })
        nodeAnalysisData.value = extractAPIData(response)
      } else {
        const serviceName = properties.serviceName || node.name || node.displayName
        if (!serviceName) throw new Error('服务节点缺少服务进程名称')
        const response = await cmdbApi.getHostServiceMetricsHistory(hostId, serviceName, { duration: monitorDuration() })
        nodeAnalysisData.value = extractAPIData(response)
        if (!(nodeAnalysisData.value?.cpu || []).length && !(nodeAnalysisData.value?.memory || []).length) throw new Error(`未匹配到服务进程 ${serviceName} 的监控指标`)
      }
    }
  } catch (error) {
    console.warn('加载节点分析数据失败:', error)
    nodeAnalysisError.value = error.response?.data?.message || error.message || '节点趋势数据加载失败'
  } finally {
    nodeAnalysisLoading.value = false
  }
}

const loadBoundNodeLogs = async () => {
  nodeLogRows.value = []
  if (analysisMode.value === 'http' || analysisMode.value === 'health') return
  const sources = selectedNodeLogSources.value
  if (!sources.length) return
  nodeLogsLoading.value = true
  try {
    const query = analysisMode.value === 'host' ? '*' : `_msg:~"(?i)(error|timeout|failed|exception|fatal)"`
    const start = observability.value.window?.start
    const end = observability.value.window?.end
    const responses = await Promise.allSettled(sources.map(source => logtailApi.queryLogs({
      storeId: source.storeId || source.sourceId || source.id,
      query,
      start,
      end,
      timeout: '10s',
      limit: 30,
      page: 1,
      pageSize: 30
    })))
    nodeLogRows.value = responses.flatMap((result, index) => {
      if (result.status !== 'fulfilled') return []
      const data = extractAPIData(result.value) || {}
      const source = sources[index]
      const rows = data.logs || data.list || data.items || (Array.isArray(data) ? data : [])
      return rows.map(item => ({
        ...item,
        __storeId: source.storeId || source.sourceId || source.id,
        __storeCode: source.storeCode || source.code,
        __storeName: source.storeName || source.name
      }))
    }).sort((left, right) => new Date(errorTime(right)).getTime() - new Date(errorTime(left)).getTime()).slice(0, 50)
  } catch (error) {
    console.warn('加载节点关联日志失败:', error)
  } finally {
    nodeLogsLoading.value = false
  }
}

const loadSelectedNodeData = async node => {
  const nodeId = String(node.id)
  await Promise.all([loadNodeAnalysis(node), loadBoundNodeLogs()])
  if (String(selectedNode.value?.id || '') !== nodeId) return
  await renderNodeAnalysis()
}

const resolveNodeIcon = node => {
  const source = `${node?.sourceType || ''} ${node?.sourceName || ''} ${node?.name || ''} ${node?.displayName || ''}`.toLowerCase()
  if (source.includes('mysql')) return mysqlIcon
  if (source.includes('mongo')) return mongoIcon
  if (source.includes('redis')) return redisIcon
  const iconMap = {
    business_system: appIcon,
    business_service: appIcon,
    application: appIcon,
    nginx: nginxIcon,
    gateway: gatewayIcon,
    k8s_cluster: k8sIcon,
    k8s_namespace: namespaceIcon,
    k8s_workload: workloadIcon,
    pod: workloadIcon,
    container: workloadIcon,
    host: linuxIcon,
    physical_server: physicalServerIcon,
    vm: virtualIcon,
    virtual: virtualIcon,
    cloud_host: cloudHostIcon,
    database: databaseIcon,
    middleware: middlewareIcon,
    redis: redisIcon,
    log_store: logIcon,
    log_project: logIcon
  }
  return iconMap[node?.resourceType] || appIcon
}

const resolveNodePosition = node => {
  const properties = normalizeProperties(node?.properties || node?.Properties)
  const layout = normalizeProperties(properties.layout)
  const x = Number(node?.x ?? node?.X ?? layout.x)
  const y = Number(node?.y ?? node?.Y ?? layout.y)
  return Number.isFinite(x) && Number.isFinite(y) ? { x, y } : {}
}

const nodeSubtitle = node => {
  const properties = normalizeProperties(node?.properties || node?.Properties)
  return properties.endpoint || properties.privateIp || properties.namespace || properties.workloadType || node?.ip || resourceTypeText(node?.resourceType)
}

const nodeLayerRank = node => {
  const properties = normalizeProperties(node?.properties || node?.Properties)
  const isHostService = node?.resourceType === 'business_service' && (properties.deployMode === 'host' || properties.hostId)
  const rankMap = {
    business_system: 0, business_service: isHostService ? 4 : 1, application: 1, nginx: 1, gateway: 2,
    k8s_cluster: 3, host: 3, physical_server: 3, vm: 3, virtual: 3, cloud_host: 3,
    k8s_namespace: 4, k8s_workload: 5, pod: 5, database: 6, middleware: 6, redis: 6, kafka: 6
  }
  return rankMap[node?.resourceType] ?? 6
}

const nodeSoftColor = status => ({
  healthy: 'rgba(63, 185, 127, 0.13)',
  warning: 'rgba(243, 178, 75, 0.16)',
  critical: 'rgba(239, 107, 115, 0.15)',
  unknown: 'rgba(148, 163, 184, 0.16)'
})[normalizeStatus(status)]

const nodeGraphObservability = node => {
  const nodeId = String(node?.id || '')
  const signal = signalForNode(node)
  const signalStoreIds = new Set((signal?.boundLogStoreIds || []).map(String))
  const logCount = (observability.value.logSources || []).filter(source =>
    (source.boundNodeIds || []).some(id => String(id) === nodeId) ||
    signalStoreIds.has(String(source.storeId || source.sourceId || source.id))
  ).length
  const evidence = node?.evidence || {}
  const alertStatus = normalizeStatus(node?.alertStatus)
  const alertCount = Math.max(Number(evidence.activeAlertCount || evidence.alertCount || node?.activeAlertCount || 0), ['warning', 'critical'].includes(alertStatus) ? 1 : 0)
  const monitorStatus = normalizeStatus(node?.resourceStatus || nodeStatus(node))
  return { logCount, monitorStatus, monitorText: statusText(monitorStatus), alertCount }
}

const registerGraphNode = () => {
  if (graphNodeRegistered) return
  G6.registerNode('observability-workbench-node-v2', {
    draw(cfg, group) {
      const status = normalizeStatus(cfg.healthStatus)
      const color = cfg.healthColor || statusColors[status] || statusColors.unknown
      const label = cfg.displayName || cfg.label || cfg.name || '未命名节点'
      const subtitle = cfg.subtitle || resourceTypeText(cfg.resourceType)
      const scoreText = cfg.healthScore === null || cfg.healthScore === undefined || cfg.healthScore === '' || cfg.healthScore === '--' ? '' : `${cfg.healthScore}`
      const nodeObservability = cfg.graphObservability || { logCount: 0, monitorStatus: 'unknown', monitorText: '未接入', alertCount: 0 }

      group.addShape('rect', {
        attrs: {
          x: -53, y: -69, width: 106, height: 144, radius: 14,
          fill: 'rgba(255, 255, 255, 0.01)',
          stroke: '#f59e0b',
          lineWidth: 1.8,
          lineDash: [5, 5],
          opacity: 0,
          shadowColor: 'rgba(245, 158, 11, 0.24)',
          shadowBlur: 18
        },
        capture: false,
        name: 'selection-focus-frame'
      })
      group.addShape('path', {
        attrs: {
          path: [
            ['M', -58, -51], ['L', -58, -67], ['L', -42, -67],
            ['M', 42, -67], ['L', 58, -67], ['L', 58, -51],
            ['M', -58, 57], ['L', -58, 73], ['L', -42, 73],
            ['M', 42, 73], ['L', 58, 73], ['L', 58, 57]
          ],
          stroke: '#f59e0b',
          lineWidth: 3,
          lineCap: 'round',
          lineJoin: 'round',
          opacity: 0
        },
        capture: false,
        name: 'selection-corners'
      })
      group.addShape('rect', {
        attrs: {
          x: -30, y: -91, width: 60, height: 20, radius: 10,
          fill: '#fff7ed',
          stroke: 'rgba(251, 146, 60, 0.58)',
          lineWidth: 1,
          opacity: 0,
          shadowColor: 'rgba(245, 158, 11, 0.18)',
          shadowBlur: 10,
          shadowOffsetY: 4
        },
        capture: false,
        name: 'selection-label-bg'
      })
      group.addShape('text', {
        attrs: {
          x: 0, y: -81,
          text: '当前节点',
          fill: '#b45309',
          fontSize: 10,
          fontWeight: 700,
          textAlign: 'center',
          textBaseline: 'middle',
          opacity: 0
        },
        capture: false,
        name: 'selection-label'
      })
      const keyShape = group.addShape('circle', {
        attrs: {
          x: 0, y: -12, r: 33, fill: '#ffffff', stroke: color,
          lineWidth: status === 'critical' ? 3.2 : 2.4,
          shadowColor: 'rgba(15, 23, 42, 0.10)', shadowBlur: 18, shadowOffsetY: 10,
          cursor: 'pointer'
        },
        name: 'node-ring'
      })
      group.addShape('circle', { attrs: { x: 0, y: -12, r: 25, fill: nodeSoftColor(status), stroke: 'rgba(255,255,255,0.94)', lineWidth: 1 }, name: 'node-icon-bg' })
      group.addShape('image', { attrs: { x: -17, y: -29, width: 34, height: 34, img: cfg.icon, cursor: 'pointer' }, name: 'node-icon' })
      group.addShape('circle', { attrs: { x: 25, y: -35, r: 5.5, fill: color, stroke: '#ffffff', lineWidth: 2 }, name: 'health-dot' })
      group.addShape('text', { attrs: { x: 0, y: 31, text: label.length > 16 ? `${label.slice(0, 16)}...` : label, fill: '#172133', fontSize: 12, fontWeight: 700, textAlign: 'center', textBaseline: 'middle' }, name: 'title' })
      group.addShape('text', { attrs: { x: 0, y: 47, text: `${subtitle}`.length > 18 ? `${subtitle}`.slice(0, 18) : `${subtitle}`, fill: '#728197', fontSize: 9, textAlign: 'center', textBaseline: 'middle' }, name: 'subtitle' })
      ;[
        { x: -48, text: `日志 ${nodeObservability.logCount}`, fill: nodeObservability.logCount > 0 ? '#475569' : '#94a3b8' },
        { x: 0, text: `监控${nodeObservability.monitorText}`, fill: statusColors[normalizeStatus(nodeObservability.monitorStatus)] },
        { x: 48, text: `告警 ${nodeObservability.alertCount}`, fill: nodeObservability.alertCount > 0 ? '#dc2626' : '#94a3b8' }
      ].forEach(item => group.addShape('text', { attrs: { x: item.x, y: 65, text: item.text, fill: item.fill, fontSize: 8.5, fontWeight: 600, textAlign: 'center', textBaseline: 'middle' }, name: 'observability-summary' }))
      if (scoreText) {
        group.addShape('rect', { attrs: { x: -18, y: -57, width: 36, height: 18, radius: 9, fill: '#ffffff', stroke: 'rgba(223,231,242,0.96)', lineWidth: 1 }, name: 'score-pill' })
        group.addShape('text', { attrs: { x: 0, y: -48, text: scoreText, fill: color, fontSize: 10, fontWeight: 700, textAlign: 'center', textBaseline: 'middle' }, name: 'score' })
      }
      return keyShape
    },
    setState(name, value, item) {
      if (name !== 'selected') return
      const shape = item.getContainer().find(element => element.get('name') === 'node-ring')
      const frame = item.getContainer().find(element => element.get('name') === 'selection-focus-frame')
      const corners = item.getContainer().find(element => element.get('name') === 'selection-corners')
      const labelBg = item.getContainer().find(element => element.get('name') === 'selection-label-bg')
      const label = item.getContainer().find(element => element.get('name') === 'selection-label')
      const model = item.getModel()
      const status = normalizeStatus(model.healthStatus)
      const originalColor = model.healthColor || statusColors[normalizeStatus(model.healthStatus)] || statusColors.unknown
      shape?.attr({
        lineWidth: status === 'critical' ? 3.2 : 2.4,
        stroke: originalColor,
        shadowColor: value ? 'rgba(245, 158, 11, 0.22)' : 'rgba(15,23,42,0.10)',
        shadowBlur: value ? 26 : 18,
        shadowOffsetY: value ? 12 : 10
      })
      frame?.attr({ opacity: value ? 1 : 0 })
      corners?.attr({ opacity: value ? 1 : 0 })
      labelBg?.attr({ opacity: value ? 1 : 0 })
      label?.attr({ opacity: value ? 1 : 0 })
    }
  })
  G6.registerEdge('observability-flow-line-v2', {
    afterDraw(cfg, group) {
      const startPoint = cfg.startPoint || { x: 0, y: 0 }
      const endPoint = cfg.endPoint || startPoint
      const hasLength = Math.hypot(endPoint.x - startPoint.x, endPoint.y - startPoint.y) > 1
      const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
      const keyShape = group.find(element => element.get('name') === 'edge-shape') || group.get('children')?.[0]
      if (!hasLength || reduceMotion || !keyShape?.getPoint) return

      const seed = String(cfg.id || `${cfg.source}-${cfg.target}`)
        .split('')
        .reduce((total, char) => total + char.charCodeAt(0), 0)
      const phaseOffset = (seed % 1000) / 1000
      const initialPoint = keyShape.getPoint(phaseOffset) || startPoint
      const particle = group.addShape('circle', {
        attrs: {
          x: initialPoint.x,
          y: initialPoint.y,
          r: 3.2,
          fill: '#ffffff',
          stroke: '#2563eb',
          lineWidth: 1.8,
          shadowColor: 'rgba(37, 99, 235, 0.72)',
          shadowBlur: 9,
          opacity: 1
        },
        capture: false,
        name: 'traffic-flow-particle'
      })
      particle.animate(ratio => {
        const progress = (ratio + phaseOffset) % 1
        const point = keyShape.getPoint(progress) || initialPoint
        const endpointVisibility = Math.min(1, progress * 8, (1 - progress) * 8)
        return {
          x: point.x,
          y: point.y,
          opacity: 0.45 + endpointVisibility * 0.55
        }
      }, {
        duration: 1900 + (seed % 700),
        easing: 'easeLinear',
        repeat: true
      })
    }
  }, 'line')
  graphNodeRegistered = true
}

const renderTopology = async () => {
  await nextTick()
  if (!topologyContainer.value || !topologyNodes.value.length) {
    topologyGraph?.destroy()
    topologyGraph = null
    return
  }
  registerGraphNode()
  topologyGraph?.destroy()
  const nodes = topologyNodes.value.map(node => ({
    ...node,
    id: String(node.id),
    type: 'observability-workbench-node-v2',
    size: [146, 138],
    healthStatus: nodeStatus(node),
    healthColor: node.displayHealthColor || node.healthColor || statusColors[nodeStatus(node)],
    healthScore: node.displayHealthScore ?? node.healthScore,
    icon: resolveNodeIcon(node),
    subtitle: nodeSubtitle(node),
    graphObservability: nodeGraphObservability(node),
    layerRank: nodeLayerRank(node),
    ...resolveNodePosition(node)
  })).sort((left, right) => left.layerRank - right.layerRank)
  const hasPosition = node => node.x !== null && node.x !== undefined && node.x !== '' && node.y !== null && node.y !== undefined && node.y !== '' && Number.isFinite(Number(node.x)) && Number.isFinite(Number(node.y))
  const nodePositions = new Map(nodes.filter(hasPosition).map(node => [node.id, { x: Number(node.x), y: Number(node.y) }]))
  const hasSavedLayout = nodes.every(hasPosition) && topologyEdges.value.every(edge => {
    const sourcePosition = nodePositions.get(String(edge.source ?? edge.sourceId ?? edge.srcId))
    const targetPosition = nodePositions.get(String(edge.target ?? edge.targetId ?? edge.dstId))
    if (!sourcePosition || !targetPosition) return false
    return Math.hypot(targetPosition.x - sourcePosition.x, targetPosition.y - sourcePosition.y) > 80
  })
  const edges = topologyEdges.value.map(edge => {
    const source = String(edge.source ?? edge.sourceId ?? edge.srcId)
    const target = String(edge.target ?? edge.targetId ?? edge.dstId)
    const rawEdgeId = String(edge.id ?? `${source}-${target}`)
    const graphEdgeId = rawEdgeId.startsWith('edge-') || rawEdgeId.startsWith('draft-edge-')
      ? rawEdgeId
      : `edge-${rawEdgeId}`

    return {
      id: graphEdgeId,
      source,
      target,
      type: 'observability-flow-line-v2',
      label: edge.label || ''
    }
  }).filter(edge => edge.source !== edge.target)
  const tooltip = new G6.Tooltip({
    offsetX: 12,
    offsetY: 12,
    itemTypes: ['node'],
    getContent: event => {
      const model = event.item?.getModel?.() || {}
      return `<div style="padding:10px 12px;max-width:260px;color:#172133"><strong>${model.displayName || model.label || model.name || '未命名节点'}</strong><div style="margin-top:6px;color:#64748b;font-size:11px;line-height:1.7"><div>综合状态：${statusText(model.healthStatus)}</div><div>资源 / 流量：${statusText(model.resourceStatus)} / ${statusText(model.trafficStatus)}</div><div>日志 / 告警：${statusText(model.logStatus)} / ${statusText(model.alertStatus)}</div><div>健康评分：${model.healthScore ?? '-'}</div><div>${model.problemSummary || model.observability?.message || '暂无异常摘要'}</div></div></div>`
    }
  })
  topologyGraph = new G6.Graph({
    container: topologyContainer.value,
    width: topologyContainer.value.clientWidth || 760,
    height: topologyContainer.value.clientHeight || 420,
    fitView: true,
    fitViewPadding: 30,
    minZoom: 0.42,
    maxZoom: 2.2,
    modes: { default: ['drag-canvas', 'zoom-canvas'] },
    layout: hasSavedLayout ? null : { type: 'dagre', rankdir: 'LR', nodesep: 56, ranksep: 128, sortBy: 'layerRank', ranker: 'network-simplex' },
    defaultEdge: { type: 'observability-flow-line-v2', style: { stroke: '#829bbd', lineWidth: 2, lineAppendWidth: 10, endArrow: false } },
    plugins: [tooltip]
  })
  topologyGraph.data({ nodes, edges })
  topologyGraph.render()
  if (selectedNode.value) {
    const selectedItem = topologyGraph.findById(String(selectedNode.value.id))
    if (selectedItem) topologyGraph.setItemState(selectedItem, 'selected', true)
  }
  topologyZoomLabel.value = `${Math.round(topologyGraph.getZoom() * 100)}%`
  topologyGraph.on('node:click', event => {
    const node = topologyNodes.value.find(item => String(item.id) === String(event.item?.getModel?.().id))
    if (node) selectNode(node)
  })
}

const fitTopology = () => {
  topologyGraph?.fitView(30)
  if (topologyGraph) topologyZoomLabel.value = `${Math.round(topologyGraph.getZoom() * 100)}%`
}
const zoomTopology = delta => {
  if (!topologyGraph || !topologyContainer.value) return
  const nextZoom = Math.min(2.2, Math.max(0.42, topologyGraph.getZoom() + delta))
  topologyGraph.zoomTo(nextZoom, { x: topologyContainer.value.clientWidth / 2, y: topologyContainer.value.clientHeight / 2 })
  topologyZoomLabel.value = `${Math.round(nextZoom * 100)}%`
}
const normalizeResponse = response => response?.data?.code !== undefined ? response.data : response

const loadBusinessModels = async () => {
  try {
    const res = normalizeResponse(await listBusinessModels({ page: 1, pageSize: 200 }))
    if (![0, 200].includes(res?.code)) throw new Error(res?.message || '业务列表加载失败')
    const data = res.data || {}
    businessModels.value = Array.isArray(data) ? data : (data.list || data.records || data.rows || [])
  } catch (error) {
    console.error('业务导航加载失败:', error)
    businessModels.value = []
  }
}

const observabilityParams = () => {
  if (route.query.start && route.query.end) return { start: route.query.start, end: route.query.end }
  return { window: selectedWindow.value }
}

const loadObservability = async () => {
  loading.value = true
  loadError.value = ''
  try {
    const res = normalizeResponse(await getBusinessObservability(businessId.value, observabilityParams()))
    if (![0, 200].includes(res?.code)) throw new Error(res?.message || '可观测数据加载失败')
    observability.value = { ...observability.value, ...(res.data || {}) }
    disposeAnalysisCharts()
    selectedError.value = null
    selectedNode.value = null
    nodeAnalysisData.value = null
    nodeLogRows.value = []
    await renderTopology()
    setupResizeObserver()
  } catch (error) {
    console.error('业务可观测数据加载失败:', error)
    loadError.value = error.message || '业务可观测数据加载失败'
  } finally {
    loading.value = false
  }
}

const loadAll = async () => Promise.all([loadBusinessModels(), loadObservability()])
const handleRefresh = async () => {
  refreshing.value = true
  try {
    const res = normalizeResponse(await refreshBusinessHealth(businessId.value))
    if (![0, 200].includes(res?.code)) throw new Error(res?.message || '刷新健康状态失败')
    await loadObservability()
    ElMessage.success('业务观测数据已刷新')
  } catch (error) {
    ElMessage.error(error.message || '刷新观测数据失败')
  } finally {
    refreshing.value = false
  }
}

const setupResizeObserver = () => {
  resizeObserver?.disconnect()
  resizeObserver = new ResizeObserver(() => {
    if (topologyGraph && topologyContainer.value) topologyGraph.changeSize(topologyContainer.value.clientWidth, topologyContainer.value.clientHeight)
    analysisCharts.forEach(chart => chart?.resize())
  })
  if (topologyContainer.value) resizeObserver.observe(topologyContainer.value)
}

const switchBusiness = id => {
  if (String(id) === String(businessId.value)) return
  router.push({ path: `/cmdb/service_model/${id}`, query: { window: selectedWindow.value } })
}
const goBack = () => router.push('/cmdb/service_model')
const goTopology = nodeId => router.push({ path: `/cmdb/service_model/${businessId.value}/topology`, query: nodeId ? { nodeId } : {} })
const goLogAnalysis = () => router.push({
  path: `/cmdb/service_model/${businessId.value}/log-analysis`,
  query: { window: selectedWindow.value }
})
const goMonitorAlerts = () => router.push({
  path: `/cmdb/service_model/${businessId.value}/monitor-alerts`,
  query: { window: selectedWindow.value }
})
const focusMonitorDetails = async () => {
  const selectedResource = selectedNode.value?.resourceType === 'business_system' ? null : selectedNode.value
  const targetNode = selectedResource || topologyNodes.value.find(node => node.resourceType !== 'business_system')
  if (!targetNode) {
    ElMessage.warning('当前业务暂无可查看的资源监控节点')
    return
  }
  if (String(selectedNode.value?.id || '') !== String(targetNode.id)) await selectNode(targetNode)
  await nextTick()
  monitorSection.value?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
}
const showAllLogs = () => {
  clearSelection()
  apiSection.value?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
}
const shareDashboard = async () => {
  const shareData = { title: `${businessTitle.value} 业务可观测`, url: window.location.href }
  try {
    if (navigator.share) await navigator.share(shareData)
    else {
      await navigator.clipboard.writeText(shareData.url)
      ElMessage.success('当前视图链接已复制')
    }
  } catch (error) {
    if (error?.name !== 'AbortError') ElMessage.warning('分享失败，请稍后重试')
  }
}

watch(() => route.params.id, async (next, previous) => {
  if (!next || next === previous) return
  await loadAll()
})

onMounted(async () => {
  await loadAll()
  if (route.query.focus === 'monitor') await focusMonitorDetails()
})
onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  topologyGraph?.destroy()
  disposeAnalysisCharts()
})
</script>

<style scoped>
.observability-page {
  --surface: rgba(255, 255, 255, 0.94);
  --surface-muted: rgba(247, 250, 255, 0.94);
  --border: #d8e1ee;
  --text: #172133;
  --text-secondary: #64748b;
  --text-muted: #8a96a8;
  --blue: #2563eb;
  height: 100%;
  min-height: 0;
  padding: 8px;
  box-sizing: border-box;
  overflow: auto;
  color: var(--text);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

:global(.home-container .el-main:has(.observability-page)) { overflow: hidden; padding: 0; }

.observability-shell {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 1120px;
  min-height: 760px;
  height: 100%;
  padding: 0 10px 10px;
  box-sizing: border-box;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.96), rgba(244, 249, 255, 0.96));
  overflow: hidden;
}

.observability-header {
  display: grid;
  grid-template-columns: minmax(320px, 1fr) auto minmax(320px, 1fr);
  align-items: center;
  gap: 16px;
  min-height: 64px;
  margin: 0 -10px;
  padding: 0 14px;
  border-bottom: 1px solid var(--border);
  background: linear-gradient(90deg, #fffaf3, #f8fafc 48%, #eef5ff);
}

.header-identity, .header-actions, .detail-tabs, .business-path, .topology-tools, .chart-legend { display: flex; align-items: center; }
.header-identity { gap: 8px; min-width: 0; }
.header-identity > div { min-width: 0; }
.business-path { gap: 8px; white-space: nowrap; }
.business-path span { color: var(--text-secondary); font-size: 13px; }
.business-path i { color: #b1bdcc; font-style: normal; }
.business-path strong { max-width: 260px; overflow: hidden; font-size: 17px; text-overflow: ellipsis; }
.business-path em, .node-status-label { padding: 2px 7px; border-radius: 4px; font-size: 11px; font-style: normal; font-weight: 700; }
.header-identity p { margin: 3px 0 0; color: var(--text-secondary); font-size: 11px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.detail-tabs { align-self: stretch; gap: 20px; }
.detail-tabs button { position: relative; align-self: stretch; padding: 0 4px; border: 0; color: var(--text-secondary); background: transparent; cursor: pointer; font-size: 13px; }
.detail-tabs button.is-active { color: var(--blue); font-weight: 700; }
.detail-tabs button.is-active::after { position: absolute; right: 0; bottom: 0; left: 0; height: 2px; background: var(--blue); content: ""; }
.header-actions { justify-content: flex-end; gap: 8px; }
.generated-time { color: var(--text-muted); font-size: 11px; }
.window-select { width: 126px; }
.icon-command { width: 36px; min-height: 36px; padding: 0; border-radius: 6px; }
.icon-command--small { width: 30px; min-height: 30px; }

.degradation-banner, .load-error {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 40px;
  padding: 6px 10px;
  border: 1px solid #f3c76a;
  border-radius: 6px;
  background: #fff8e8;
  color: #8a5200;
  box-sizing: border-box;
}
.degradation-banner > div, .load-error > div { display: flex; flex: 1; flex-direction: column; min-width: 0; }
.degradation-banner strong, .load-error strong { font-size: 12px; }
.degradation-banner span, .load-error span { overflow: hidden; font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.degradation-banner button { border: 0; color: #8a5200; background: transparent; cursor: pointer; font-weight: 700; }
.load-error { margin: auto; max-width: 680px; border-color: #f2aaaa; background: #fff1f1; color: #a61b1b; }

.observability-grid {
  display: grid;
  grid-template-columns: 226px minmax(560px, 1fr) 304px;
  grid-template-rows: 82px minmax(370px, 1fr) 248px;
  flex: 1;
  gap: 8px;
  min-height: 0;
}
.panel-surface { border: 1px solid var(--border); border-radius: 7px; background: linear-gradient(135deg, var(--surface), var(--surface-muted)); overflow: hidden; }
.navigator-panel { grid-row: 1 / 4; padding: 10px; }
.kpi-panel { display: grid; grid-column: 2 / 4; grid-template-columns: repeat(6, minmax(0, 1fr)); }
.topology-panel { grid-column: 2; grid-row: 2; min-height: 0; }
.issue-panel { display: flex; grid-column: 3; grid-row: 2 / 4; flex-direction: column; min-height: 0; padding: 10px; }
.bottom-panel { display: grid; grid-column: 2; grid-row: 3; grid-template-columns: minmax(280px, 0.42fr) minmax(420px, 0.58fr); min-height: 0; }

.panel-heading { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.panel-heading h3 { margin: 0; font-size: 14px; }
.panel-heading span { color: var(--text-muted); font-size: 10px; }
.navigator-panel > .panel-heading { margin-bottom: 10px; }
.service-search { margin-bottom: 8px; }
.business-nav-list { display: grid; gap: 4px; max-height: 132px; margin-bottom: 8px; overflow-y: auto; }
.business-nav-list button, .service-list button, .issue-list-section button { border: 0; background: transparent; cursor: pointer; }
.business-nav-list button { display: flex; align-items: center; justify-content: space-between; min-height: 34px; padding: 0 8px; border-radius: 5px; color: var(--text-secondary); text-align: left; }
.business-nav-list button:hover, .business-nav-list button.is-current { color: var(--blue); background: #edf4ff; }
.business-nav-list button span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.business-nav-list em { font-size: 11px; font-style: normal; }
.service-filter { display: grid; grid-template-columns: 1fr 1fr; margin-bottom: 6px; padding: 2px; border: 1px solid var(--border); border-radius: 6px; background: #f4f7fb; }
.service-filter button { min-height: 28px; border: 0; border-radius: 4px; color: var(--text-secondary); background: transparent; cursor: pointer; }
.service-filter button.active { color: var(--blue); background: #fff; box-shadow: 0 1px 4px rgba(37, 99, 235, 0.12); font-weight: 700; }
.service-list { height: calc(100% - 214px); min-height: 260px; overflow-y: auto; }
.service-list button { display: grid; grid-template-columns: 10px minmax(0, 1fr) auto; align-items: center; gap: 7px; width: 100%; min-height: 46px; padding: 5px 7px; border-radius: 5px; text-align: left; }
.service-list button:hover, .service-list button.is-selected { background: #edf4ff; }
.service-list button > i { width: 7px; height: 7px; border-radius: 50%; }
.service-list button span { display: flex; min-width: 0; flex-direction: column; }
.service-list strong { overflow: hidden; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.service-list small { color: var(--text-muted); font-size: 10px; }
.service-list em { color: var(--text-secondary); font-size: 11px; font-style: normal; }

.kpi-panel article { display: flex; min-width: 0; padding: 10px 14px; border-right: 1px solid var(--border); flex-direction: column; justify-content: center; }
.kpi-panel article:last-child { border-right: 0; }
.kpi-panel span { color: var(--text-secondary); font-size: 11px; }
.kpi-panel strong { margin: 2px 0; font-size: 21px; font-variant-numeric: tabular-nums; }
.kpi-panel small { overflow: hidden; color: var(--text-muted); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.healthy-text { color: #15803d; }
.warning-text { color: #b45309; }
.critical-text { color: #dc2626; }

.topology-heading { height: 46px; padding: 0 10px; border-bottom: 1px solid var(--border); }
.topology-tools { gap: 9px; }
.topology-tools > span, .chart-legend span { display: flex; align-items: center; gap: 4px; color: var(--text-secondary); font-size: 10px; }
.topology-tools i, .chart-legend i { display: inline-block; width: 14px; height: 2px; background: #94a3b8; }
.topology-tools i { width: 7px; height: 7px; border-radius: 50%; }
.topology-tools i.healthy { background: #16a34a; }
.topology-tools i.warning { background: #d97706; }
.topology-tools i.critical { background: #dc2626; }
.topology-stage {
  position: relative;
  height: calc(100% - 47px);
  min-height: 300px;
  background:
    radial-gradient(circle at 24% 18%, rgba(76, 139, 245, 0.055), transparent 30%),
    radial-gradient(circle at 76% 70%, rgba(63, 185, 127, 0.035), transparent 28%),
    linear-gradient(rgba(126, 152, 196, 0.04) 1px, transparent 1px),
    linear-gradient(90deg, rgba(126, 152, 196, 0.04) 1px, transparent 1px),
    rgba(252, 254, 255, 0.82);
  background-size: auto, auto, 28px 28px, 28px 28px, auto;
  overflow: hidden;
}
.topology-stage::before { position: absolute; z-index: 2; inset: 14px; border: 1px dashed rgba(174, 190, 216, 0.3); border-radius: 12px; content: ""; pointer-events: none; }
.topology-canvas { position: relative; z-index: 1; width: 100%; height: 100%; }
.panel-empty { position: absolute; inset: 0; display: grid; z-index: 2; place-items: center; color: var(--text-muted); font-size: 12px; }

.issue-panel > .panel-heading { padding-bottom: 8px; border-bottom: 1px solid var(--border); }
.clear-selection { border: 0; color: var(--blue); background: transparent; cursor: pointer; font-size: 11px; }
.issue-detail { padding: 10px 0; border-bottom: 1px solid var(--border); }
.issue-title { display: flex; align-items: center; gap: 7px; }
.issue-title strong { overflow: hidden; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.error-code { padding: 2px 6px; border: 1px solid #fecaca; border-radius: 4px; color: #dc2626; background: #fff1f2; font-size: 10px; font-weight: 700; }
.issue-detail dl { display: grid; gap: 6px; margin: 9px 0 0; }
.issue-detail dl div { display: grid; grid-template-columns: 72px minmax(0, 1fr); gap: 6px; font-size: 10px; }
.issue-detail dt { color: var(--text-muted); }
.issue-detail dd { margin: 0; overflow-wrap: anywhere; color: var(--text-secondary); }
.issue-placeholder { display: grid; justify-items: center; gap: 5px; padding: 18px 6px; border-bottom: 1px solid var(--border); color: var(--text-muted); text-align: center; }
.issue-placeholder .el-icon { font-size: 22px; }
.issue-placeholder strong { color: var(--text-secondary); font-size: 12px; }
.issue-placeholder span { font-size: 10px; }
.issue-list-section { padding-top: 9px; }
.issue-list-section header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 4px; }
.issue-list-section h4 { margin: 0; font-size: 12px; }
.issue-list-section header span { color: var(--text-muted); font-size: 10px; }
.issue-list-section button { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; width: 100%; min-height: 42px; padding: 5px 3px; border-bottom: 1px solid #edf1f6; text-align: left; }
.issue-list-section button:hover { background: #f4f7fb; }
.issue-list-section button span { display: flex; min-width: 0; flex-direction: column; }
.issue-list-section button strong, .issue-list-section button small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.issue-list-section button strong { font-size: 10px; }
.issue-list-section button small { color: var(--text-muted); font-size: 9px; }
.issue-list-section button em { align-self: center; color: #dc2626; font-size: 10px; font-style: normal; }
.alert-list { margin-top: auto; }

.trend-region, .api-region { position: relative; min-width: 0; min-height: 0; padding: 8px 10px; }
.trend-region { border-right: 1px solid var(--border); }
.trend-chart { width: 100%; height: 198px; }
.panel-empty--chart { top: 36px; }
.chart-legend { gap: 8px; }
.chart-legend i.requests { background: #2563eb; }
.chart-legend i.errors4xx { background: #d97706; }
.chart-legend i.errors5xx { background: #dc2626; }
.api-region .panel-heading { margin-bottom: 6px; }
.error-table { --el-table-border-color: #edf1f6; --el-table-header-bg-color: #f6f8fb; cursor: pointer; }
.method-tag { padding: 2px 4px; border-radius: 3px; color: #1d4ed8; background: #eaf2ff; font-size: 9px; font-weight: 700; }
.status-code { color: #dc2626; font-weight: 700; }
.compact-empty { padding: 12px 4px; color: var(--text-muted); text-align: center; font-size: 10px; }

.status-healthy { color: #15803d; }
.status-warning { color: #a75b00; }
.status-critical { color: #b91c1c; }
.status-unknown { color: #64748b; }
.service-list button > i.status-healthy { background-color: #16a34a; }
.service-list button > i.status-warning { background-color: #d97706; }
.service-list button > i.status-critical { background-color: #dc2626; }
.service-list button > i.status-unknown { background-color: #94a3b8; }
.business-path em.status-healthy, .node-status-label.status-healthy { background: #e8f7ed; }
.business-path em.status-warning, .node-status-label.status-warning { background: #fff5dc; }
.business-path em.status-critical, .node-status-label.status-critical { background: #fff0f0; }
.business-path em.status-unknown, .node-status-label.status-unknown { background: #eef2f6; }

@media (max-width: 1380px) {
  .observability-shell { min-width: 980px; }
  .observability-grid { grid-template-columns: 210px minmax(520px, 1fr); grid-template-rows: 82px 420px 300px 248px; }
  .navigator-panel { grid-row: 1 / 5; }
  .kpi-panel { grid-column: 2; }
  .topology-panel { grid-column: 2; }
  .issue-panel { grid-column: 2; grid-row: 3; display: grid; grid-template-columns: 0.9fr 1fr 1fr; gap: 10px; }
  .issue-panel > .panel-heading { grid-column: 1 / -1; }
  .issue-detail, .issue-placeholder { border-right: 1px solid var(--border); border-bottom: 0; }
  .alert-list { margin-top: 0; }
  .bottom-panel { grid-column: 2; grid-row: 4; }
  .generated-time { display: none; }
}

@media (max-width: 760px) {
  .observability-page { padding: 0; overflow: auto; }
  .observability-shell { min-width: 0; min-height: 100%; height: auto; padding: 0 8px 8px; border-radius: 0; overflow: visible; }
  .observability-header { position: sticky; top: 0; z-index: 20; grid-template-columns: 1fr auto; min-height: auto; padding: 8px; }
  .detail-tabs { grid-column: 1 / -1; grid-row: 2; min-height: 40px; justify-content: center; }
  .header-actions .window-select { width: 112px; }
  .business-path span, .business-path i, .header-identity p { display: none; }
  .business-path strong { max-width: 150px; font-size: 14px; }
  .observability-grid { display: flex; flex-direction: column; }
  .navigator-panel, .kpi-panel, .topology-panel, .issue-panel, .bottom-panel { width: 100%; }
  .navigator-panel { max-height: 360px; }
  .kpi-panel { grid-template-columns: repeat(2, 1fr); }
  .kpi-panel article { border-bottom: 1px solid var(--border); }
  .kpi-panel article:nth-child(even) { border-right: 0; }
  .topology-panel { height: 460px; }
  .issue-panel { display: flex; max-height: none; }
  .bottom-panel { display: flex; flex-direction: column; }
  .trend-region { min-height: 240px; border-right: 0; border-bottom: 1px solid var(--border); }
  .api-region { min-height: 260px; }
  .topology-tools > span { display: none; }
}
</style>

<style scoped>
.observability-page {
  --surface: #ffffff;
  --surface-subtle: #f7f9fc;
  --page-bg: #eef2f7;
  --border: #dce3ed;
  --border-strong: #cbd5e2;
  --text: #182235;
  --text-secondary: #5f6d82;
  --text-muted: #8995a7;
  --blue: #2563eb;
  --blue-soft: #edf4ff;
  width: 100%;
  height: 100%;
  min-height: 0;
  padding: 8px;
  overflow: auto;
  box-sizing: border-box;
  color: var(--text);
  background: var(--page-bg);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

:global(.home-container .el-main:has(.observability-page)) {
  padding: 0;
  overflow: hidden;
}

.observability-shell {
  display: flex;
  flex-direction: column;
  width: 100%;
  min-width: 1180px;
  min-height: 820px;
  height: 100%;
  padding: 0;
  gap: 0;
  overflow: hidden;
  box-sizing: border-box;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--surface);
}

.observability-header {
  flex: none;
  margin: 0;
  padding: 0 18px;
  border-bottom: 1px solid var(--border);
  background: #ffffff;
}

.header-main-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 66px;
  gap: 24px;
}

.header-identity,
.header-actions,
.business-path,
.detail-tabs,
.topology-tools,
.topology-legend,
.zoom-tools,
.navigator-heading,
.panel-heading,
.issue-heading,
.issue-title {
  display: flex;
  align-items: center;
}

.header-identity {
  min-width: 0;
  gap: 8px;
}

.business-heading {
  min-width: 0;
}

.business-path {
  gap: 9px;
  min-width: 0;
  white-space: nowrap;
}

.business-path span,
.business-path i {
  color: var(--text-secondary);
  font-size: 13px;
  font-style: normal;
}

.business-path i {
  color: #b0bac8;
}

.business-path strong {
  max-width: 320px;
  overflow: hidden;
  font-size: 17px;
  text-overflow: ellipsis;
}

.business-heading p {
  max-width: 620px;
  margin: 4px 0 0;
  overflow: hidden;
  color: var(--text-secondary);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.environment-badge,
.health-badge,
.node-status-label,
.error-code,
.count-badge {
  display: inline-flex;
  align-items: center;
  min-height: 20px;
  padding: 0 7px;
  border-radius: 4px;
  font-size: 10px;
  font-style: normal;
  font-weight: 700;
}

.environment-badge {
  color: #087a55;
  background: #e9f8f2;
  border: 1px solid #bce9da;
}

.health-badge.status-healthy,
.node-status-label.status-healthy { color: #087a55; background: #e9f8f2; }
.health-badge.status-warning,
.node-status-label.status-warning { color: #a15c00; background: #fff3d8; }
.health-badge.status-critical,
.node-status-label.status-critical { color: #c52828; background: #fff0f0; }
.health-badge.status-unknown,
.node-status-label.status-unknown { color: #657287; background: #eef2f6; }

.header-actions {
  justify-content: flex-end;
  gap: 8px;
  white-space: nowrap;
}

.generated-time {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--text-muted);
  font-size: 11px;
}

.window-select { width: 126px; }
.icon-command,
.share-command {
  min-height: 34px;
  border-radius: 5px;
  border-color: var(--border);
  color: #526177;
  background: #ffffff;
}
.icon-command { width: 34px; padding: 0; }
.icon-command--plain { border: 0; background: transparent; }
.share-command { display: inline-flex; gap: 5px; padding: 0 11px; }
.icon-command:hover,
.share-command:hover { color: var(--blue); border-color: #9db9ef; background: var(--blue-soft); }

.detail-tabs {
  height: 40px;
  gap: 28px;
  padding-left: 43px;
}

.detail-tabs button {
  position: relative;
  align-self: stretch;
  padding: 0 2px;
  border: 0;
  color: #657287;
  background: transparent;
  cursor: pointer;
  font-size: 13px;
}

.detail-tabs button:hover { color: var(--blue); }
.detail-tabs button.is-active { color: var(--blue); font-weight: 700; }
.detail-tabs button.is-active::after {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  height: 2px;
  background: var(--blue);
  content: "";
}
.detail-tabs button:disabled { color: #aab3c1; cursor: not-allowed; }

.degradation-banner,
.load-error {
  display: flex;
  flex: none;
  align-items: center;
  gap: 10px;
  min-height: 38px;
  margin: 8px 10px 0;
  padding: 5px 10px;
  box-sizing: border-box;
  border: 1px solid #edc36a;
  border-radius: 5px;
  color: #8b5700;
  background: #fff9e9;
}
.degradation-banner > div,
.load-error > div { display: flex; flex: 1; min-width: 0; flex-direction: column; }
.degradation-banner strong,
.load-error strong { font-size: 12px; }
.degradation-banner span,
.load-error span { overflow: hidden; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.degradation-banner button { border: 0; color: inherit; background: transparent; cursor: pointer; font-weight: 700; }
.load-error { margin: auto; max-width: 660px; border-color: #efb0b0; color: #a92323; background: #fff3f3; }

.observability-grid {
  display: grid;
  grid-template-columns: 254px minmax(620px, 1fr) 316px;
  grid-template-rows: 92px minmax(430px, 1fr) 236px;
  flex: 1;
  min-height: 0;
  gap: 8px;
  padding: 8px;
  box-sizing: border-box;
  background: var(--page-bg);
}

.navigator-panel,
.kpi-panel,
.topology-panel,
.issue-panel,
.api-panel {
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: #ffffff;
}

.navigator-panel {
  display: flex;
  grid-column: 1;
  grid-row: 1 / 4;
  flex-direction: column;
  padding: 12px 10px 8px;
}

.navigator-heading,
.panel-heading,
.issue-heading { justify-content: space-between; gap: 10px; }
.navigator-heading { margin-bottom: 12px; padding: 0 3px; }
.navigator-heading h3,
.panel-heading h3,
.issue-heading h3 { margin: 0; font-size: 14px; }
.navigator-heading span,
.panel-heading span,
.issue-heading span { color: var(--text-muted); font-size: 10px; }
.filter-command,
.issue-heading > button,
.zoom-tools button,
.view-all-command {
  border: 0;
  color: #758196;
  background: transparent;
  cursor: pointer;
}
.filter-command { width: 28px; height: 28px; border-radius: 4px; }
.filter-command:hover,
.filter-command.active { color: var(--blue); background: var(--blue-soft); }
.service-search { margin-bottom: 9px; }
:deep(.service-search .el-input__wrapper) { min-height: 34px; border-radius: 5px; box-shadow: 0 0 0 1px var(--border) inset; }

.service-filter {
  display: grid;
  grid-template-columns: 1fr 1fr;
  margin-bottom: 8px;
  padding: 2px;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: #f3f6fa;
}
.service-filter button {
  min-height: 28px;
  border: 0;
  border-radius: 4px;
  color: var(--text-secondary);
  background: transparent;
  cursor: pointer;
}
.service-filter button.active { color: var(--blue); background: #ffffff; box-shadow: 0 1px 3px rgba(34, 67, 113, 0.12); font-weight: 700; }

.system-tree {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: #a7b1bf transparent;
}
.system-tree-group + .system-tree-group { margin-top: 3px; }
.system-tree-root {
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr) auto;
  align-items: center;
  gap: 5px;
  width: 100%;
  min-height: 36px;
  padding: 0 6px;
  border: 0;
  border-radius: 4px;
  color: #3f4d61;
  background: transparent;
  cursor: pointer;
  text-align: left;
}
.system-tree-root:hover,
.system-tree-root.is-current { color: #1f5fd2; background: #eaf2ff; }
.tree-chevron { font-size: 12px; transition: transform 0.16s ease; }
.system-tree-root.is-current .tree-chevron { transform: rotate(90deg); }
.system-tree-root span { overflow: hidden; font-size: 12px; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }
.system-tree-root em { min-width: 22px; color: var(--text-muted); font-size: 10px; font-style: normal; text-align: right; }
.system-tree-root em.status-warning { color: #b96a00; }
.system-tree-root em.status-critical { color: #d32f2f; }

.service-list { margin: 3px 0 7px 12px; }
.service-list button {
  display: grid;
  grid-template-columns: 9px minmax(0, 1fr) auto;
  align-items: center;
  gap: 7px;
  width: 100%;
  min-height: 46px;
  padding: 5px 7px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  cursor: pointer;
  text-align: left;
}
.service-list button:hover,
.service-list button.is-selected { background: #edf4ff; }
.service-list button > i { width: 6px; height: 6px; border-radius: 50%; }
.service-list button > i.status-healthy { background: #19a66f; }
.service-list button > i.status-warning { background: #df8500; }
.service-list button > i.status-critical { background: #df3e3e; }
.service-list button > i.status-unknown { background: #a5afbd; }
.service-list button span { display: flex; min-width: 0; flex-direction: column; }
.service-list strong { overflow: hidden; font-size: 11.5px; text-overflow: ellipsis; white-space: nowrap; }
.service-list small { margin-top: 2px; color: var(--text-muted); font-size: 9.5px; }
.service-list em { color: var(--text-secondary); font-size: 10px; font-style: normal; }

.kpi-panel {
  display: grid;
  grid-column: 2;
  grid-row: 1;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}
.kpi-panel article {
  display: grid;
  grid-template-columns: 40px minmax(0, 1fr);
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  border-right: 1px solid var(--border);
}
.kpi-panel article:last-child { border-right: 0; }
.kpi-panel .kpi-icon {
  display: grid;
  width: 40px;
  height: 40px;
  border: 1px solid transparent;
  border-radius: 8px;
  place-items: center;
  color: #ffffff;
  font-size: 21px;
}
.kpi-panel .kpi-icon :deep(.el-icon) { width: 21px; height: 21px; color: #ffffff; font-size: 21px; }
.kpi-panel .kpi-icon :deep(svg) {
  width: 21px;
  height: 21px;
  color: #ffffff;
  filter: drop-shadow(0 1px 1px rgba(45, 62, 91, 0.16));
}
.kpi-icon--requests {
  background: #9bb8ed;
  box-shadow: 0 2px 6px rgba(92, 127, 191, 0.10), inset 0 1px 0 rgba(255, 255, 255, 0.38);
}
.kpi-icon--errors {
  background: #f0a5ad;
  box-shadow: 0 2px 6px rgba(201, 101, 111, 0.10), inset 0 1px 0 rgba(255, 255, 255, 0.38);
}
.kpi-icon--latency {
  background: #edbd7a;
  box-shadow: 0 2px 6px rgba(191, 133, 53, 0.10), inset 0 1px 0 rgba(255, 255, 255, 0.38);
}
.kpi-icon--success {
  background: #7dc9af;
  box-shadow: 0 2px 6px rgba(60, 151, 120, 0.10), inset 0 1px 0 rgba(255, 255, 255, 0.38);
}
.kpi-copy { display: flex; min-width: 0; flex-direction: column; }
.kpi-copy > span { color: var(--text-secondary); font-size: 10px; }
.kpi-copy strong { margin: 2px 0; font-family: "Segoe UI", "PingFang SC", sans-serif; font-size: 21px; font-variant-numeric: tabular-nums; line-height: 1.05; }
.kpi-copy small { overflow: hidden; color: var(--text-muted); font-size: 9.5px; text-overflow: ellipsis; white-space: nowrap; }
.healthy-text { color: #0c8c5d; }
.critical-text { color: #d92e2e; }

.topology-panel { grid-column: 2; grid-row: 2; }
.topology-heading { height: 45px; padding: 0 11px; border-bottom: 1px solid var(--border); }
.topology-heading > div:first-child { display: flex; align-items: baseline; gap: 8px; }
.topology-tools { justify-content: flex-end; gap: 14px; }
.topology-legend { gap: 10px; }
.topology-legend span { display: inline-flex; align-items: center; gap: 4px; color: var(--text-secondary); font-size: 9.5px; }
.topology-legend i { display: inline-block; width: 6px; height: 6px; border-radius: 50%; }
.topology-legend i.healthy { background: #19a66f; }
.topology-legend i.warning { background: #df8500; }
.topology-legend i.critical { background: #df3e3e; }
.zoom-tools { height: 28px; overflow: hidden; border: 1px solid var(--border); border-radius: 4px; }
.zoom-tools button { display: grid; width: 28px; height: 28px; border-right: 1px solid var(--border); place-items: center; }
.zoom-tools button:last-child { border-right: 0; border-left: 1px solid var(--border); }
.zoom-tools button:hover { color: var(--blue); background: var(--blue-soft); }
.zoom-tools > span { width: 46px; color: var(--text-secondary); text-align: center; font-size: 9.5px; }
.topology-stage {
  position: relative;
  height: calc(100% - 46px);
  min-height: 380px;
  overflow: hidden;
  background-color: #fbfcfe;
  background-image: radial-gradient(circle, #dfe6f0 1px, transparent 1px);
  background-size: 22px 22px;
}
.topology-canvas { width: 100%; height: 100%; }
.panel-empty { position: absolute; z-index: 2; inset: 0; display: grid; place-items: center; color: var(--text-muted); font-size: 11px; }

.issue-panel {
  display: flex;
  grid-column: 3;
  grid-row: 1 / 4;
  flex-direction: column;
}
.issue-heading { min-height: 46px; padding: 0 12px; border-bottom: 1px solid var(--border); }
.issue-heading > div { display: flex; align-items: baseline; gap: 8px; }
.issue-heading > button { display: grid; width: 28px; height: 28px; border-radius: 4px; place-items: center; }
.issue-heading > button:hover { color: var(--blue); background: var(--blue-soft); }
.issue-detail { min-height: 0; padding: 13px 12px; overflow-y: auto; }
.issue-title { gap: 8px; min-width: 0; }
.issue-title strong { overflow: hidden; font-size: 13px; text-overflow: ellipsis; white-space: nowrap; }
.error-code { color: #d73232; border: 1px solid #ffc9c9; background: #fff2f2; }
.diagnosis-badge { flex: none; padding: 2px 6px; border-radius: 3px; font-size: 9px; font-weight: 700; }
.diagnosis-badge--root { color: #b91c1c; border: 1px solid #fecaca; background: #fff1f2; }
.diagnosis-badge--affected { color: #1d4ed8; border: 1px solid #bfdbfe; background: #eff6ff; }
.node-conclusion {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  margin-top: 12px;
  padding: 11px;
  border: 1px solid #dce4ee;
  border-radius: 5px;
  color: #66758a;
  background: #f7f9fc;
}
.node-conclusion > .el-icon { flex: none; margin-top: 1px; font-size: 17px; }
.node-conclusion > div { display: flex; min-width: 0; flex-direction: column; gap: 3px; }
.node-conclusion strong { font-size: 11.5px; }
.node-conclusion span { font-size: 9.5px; line-height: 1.45; }
.node-conclusion.status-critical { color: #c62f2f; border-color: #ffcaca; background: #fff3f3; }
.node-conclusion.status-warning { color: #a76200; border-color: #f2d49d; background: #fff9ea; }
.node-conclusion.status-healthy { color: #0c8159; border-color: #bce8d8; background: #effaf6; }
.node-analysis { margin-top: 12px; }
.analysis-judgement { padding: 10px; border: 1px solid #bfd3f5; border-radius: 4px; background: #f3f7ff; }
.analysis-judgement strong { color: #2357ae; font-size: 11px; }
.analysis-judgement p { margin: 4px 0 0; color: #465873; font-size: 10px; line-height: 1.55; }
.analysis-error { margin-top: 8px; padding: 12px 8px; border: 1px solid #f1c2c2; border-radius: 4px; color: #b62f2f; background: #fff5f5; font-size: 10px; line-height: 1.5; }
.analysis-chart-card { margin-top: 8px; border: 1px solid #e3e9f1; border-radius: 4px; background: #ffffff; }
.analysis-chart-card h4 { height: 30px; margin: 0; padding: 0 9px; border-bottom: 1px solid #edf1f6; color: #3f4d61; font-size: 10.5px; line-height: 30px; }
.analysis-chart { width: 100%; height: 138px; }
.detail-section { padding: 14px 0; border-bottom: 1px solid #edf1f6; }
.detail-section:last-child { border-bottom: 0; }
.detail-section h4 { margin: 0 0 8px; font-size: 11px; }
.detail-section p { margin: 4px 0; color: var(--text-secondary); font-size: 10.5px; line-height: 1.65; overflow-wrap: anywhere; }
.detail-section .error-summary { color: #303d52; font-weight: 700; }
.impact-tags { display: flex; flex-wrap: wrap; gap: 6px; }
.impact-tags button { padding: 0; border: 0; color: var(--blue); background: transparent; cursor: pointer; font-size: 10.5px; text-align: left; }
.impact-tags button:hover { text-decoration: underline; }
.impact-tags span { color: var(--text-muted); font-size: 10px; }
.detail-fields dl { display: grid; gap: 9px; margin: 0; }
.detail-fields dl div { display: grid; grid-template-columns: 70px minmax(0, 1fr); gap: 8px; font-size: 10.5px; }
.detail-fields dt { color: var(--text-muted); }
.detail-fields dd { margin: 0; color: #415068; overflow-wrap: anywhere; }
.detail-fields dd.danger-value { color: #d92e2e; font-weight: 700; }
.node-reasons h4 { display: flex; justify-content: space-between; }
.node-reasons h4 span { color: #d92e2e; font-size: 10px; }
.node-reasons article { margin-top: 7px; padding: 8px; border-left: 3px solid #94a3b8; border-radius: 3px; background: #f7f9fc; }
.node-reasons article.status-warning { border-left-color: #d97706; background: #fff9eb; }
.node-reasons article.status-critical { border-left-color: #dc2626; background: #fff3f3; }
.node-reasons article > div { display: flex; align-items: center; gap: 6px; }
.node-reasons article i { width: 6px; height: 6px; border-radius: 50%; background: #94a3b8; }
.node-reasons article.status-warning i { background: #d97706; }
.node-reasons article.status-critical i { background: #dc2626; }
.node-reasons article strong { color: #344258; font-size: 10.5px; }
.node-reasons article p { margin: 5px 0 0; }
.node-reasons article small { display: block; margin-top: 4px; color: var(--text-muted); font-size: 9.5px; }
.bound-log-summary p { margin: 0; }
.no-node-errors { margin: 0; color: var(--text-muted); font-size: 10px; }
.issue-placeholder { display: grid; flex: 1; align-content: center; justify-items: center; gap: 6px; padding: 30px 16px; color: var(--text-muted); text-align: center; }
.issue-placeholder .el-icon { font-size: 26px; }
.issue-placeholder strong { color: var(--text-secondary); font-size: 12px; }
.issue-placeholder span { font-size: 10px; }

.api-panel { grid-column: 2; grid-row: 3; }
.api-heading { min-height: 39px; padding: 0 11px; border-bottom: 1px solid var(--border); }
.api-heading > div { display: flex; min-width: 0; align-items: center; gap: 7px; }
.api-heading h3 { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.count-badge { min-width: 21px; justify-content: center; padding: 0 5px; color: #53637a; background: #edf1f6; }
.view-all-command { color: var(--blue); font-size: 10px; }
.view-all-command:hover { text-decoration: underline; }
.error-table { --el-table-border-color: #edf1f6; --el-table-header-bg-color: #f6f8fb; --el-table-row-hover-bg-color: #edf4ff; cursor: pointer; }
:deep(.error-table .el-table__header th) { height: 34px; color: #6e7b8f; font-size: 10px; font-weight: 600; }
:deep(.error-table .el-table__row td) { height: 31px; color: #354359; font-size: 10.5px; }
.method-tag { display: inline-flex; min-width: 36px; justify-content: center; padding: 2px 4px; border-radius: 3px; color: #1d5fd0; background: #eaf2ff; font-size: 9px; font-weight: 700; }
.status-code { color: #d92e2e; font-weight: 700; }
.log-level { color: #d92e2e; font-size: 9.5px; font-weight: 700; }
.compact-empty { padding: 14px 6px; color: var(--text-muted); text-align: center; font-size: 10px; }

@media (max-width: 1380px) {
  .observability-shell { min-width: 1080px; }
  .observability-grid { grid-template-columns: 220px minmax(580px, 1fr) 286px; }
  .generated-time { display: none; }
  .kpi-panel article { grid-template-columns: 34px minmax(0, 1fr); gap: 7px; padding: 10px; }
  .kpi-icon { width: 36px; height: 36px; font-size: 20px; }
  .kpi-icon :deep(.el-icon),
  .kpi-icon :deep(svg) { width: 20px; height: 20px; font-size: 20px; }
  .kpi-copy strong { font-size: 18px; }
}

@media (max-width: 760px) {
  .observability-page { padding: 0; overflow: auto; }
  .observability-shell { min-width: 0; min-height: 100%; height: auto; border: 0; border-radius: 0; overflow: visible; }
  .observability-header { position: sticky; top: 0; z-index: 20; padding: 0 10px; }
  .header-main-row { min-height: 58px; }
  .header-actions .share-command,
  .header-actions .generated-time,
  .header-actions > .el-dropdown { display: none; }
  .business-path > span,
  .business-path > i,
  .business-heading p { display: none; }
  .business-path strong { max-width: 150px; font-size: 14px; }
  .detail-tabs { gap: 20px; padding-left: 42px; overflow-x: auto; }
  .observability-grid { display: flex; padding: 7px; flex-direction: column; }
  .navigator-panel,
  .kpi-panel,
  .topology-panel,
  .issue-panel,
  .api-panel { width: 100%; }
  .navigator-panel { max-height: 390px; }
  .kpi-panel { grid-template-columns: repeat(2, 1fr); }
  .kpi-panel article:nth-child(2) { border-right: 0; }
  .kpi-panel article:nth-child(-n + 2) { border-bottom: 1px solid var(--border); }
  .topology-panel { height: 500px; }
  .issue-panel { min-height: 360px; }
  .api-panel { min-height: 240px; }
  .topology-legend { display: none; }
}
</style>
