<template>
  <div class="k8s-inspection-page">
    <section class="page-card">
      <header class="page-hero">
        <div class="hero-title-group">
          <span class="ops-page-header__icon" aria-hidden="true">
            <el-icon><DataAnalysis /></el-icon>
          </span>
          <div class="ops-page-header__content">
            <h1>集群巡检</h1>
            <span class="ops-page-header__divider" aria-hidden="true"></span>
            <p class="page-description">
              对集群、节点、工作负载、网络、存储、安全与容量进行一次性健康扫描，输出可追溯的巡检报告。
            </p>
          </div>
        </div>

        <div class="header-actions">
          <el-select
            v-model="selectedClusterId"
            class="cluster-select"
            size="small"
            filterable
            placeholder="请选择集群"
            :loading="clusterLoading"
            @change="handleClusterChange"
          >
            <template #prefix>
              <el-icon><Connection /></el-icon>
            </template>
            <el-option
              v-for="cluster in clusterOptions"
              :key="cluster.id"
              :label="cluster.name"
              :value="cluster.id"
              :disabled="cluster.status !== 2"
            >
              <div class="cluster-option">
                <span>{{ cluster.name }}</span>
                <el-tag size="small" effect="light" :type="clusterStatusTag(cluster.status)">
                  {{ clusterStatusText(cluster.status) }}
                </el-tag>
              </div>
            </el-option>
          </el-select>

          <el-button class="warning-action" size="small" :icon="Setting" @click="optionDrawerVisible = true">
            检查项
          </el-button>
          <el-button class="secondary-action" size="small" :icon="Tickets" @click="openHistoryDrawer">
            历史记录
          </el-button>
          <el-button
            type="primary"
            size="small"
            class="primary-action"
            :icon="CaretRight"
            :loading="startLoading || isRunning"
            :disabled="!selectedClusterId"
            @click="startClusterInspection"
          >
            {{ isRunning ? `巡检中 ${normalizedProgress}%` : '开始巡检' }}
          </el-button>
        </div>
      </header>

      <main class="report-workspace">
        <section class="status-strip" :class="`is-${progressState.status || 'idle'}`">
          <div class="status-main">
            <el-tag effect="light" :type="inspectionStatusTag(progressState.status)">
              {{ inspectionStatusText(progressState.status) }}
            </el-tag>
            <div>
              <div class="status-title-row">
                <strong>{{ progressState.currentStep || '选择集群后即可启动巡检' }}</strong>
                <em>{{ normalizedProgress }}%</em>
              </div>
              <span>
                {{ selectedClusterName || '未选择集群' }}
                <template v-if="currentInspectionId"> / 任务 #{{ currentInspectionId }}</template>
              </span>
            </div>
          </div>
          <div class="status-progress">
            <el-progress
              :percentage="normalizedProgress"
              :stroke-width="8"
              :color="progressColor"
              :show-text="false"
            />
          </div>
        </section>

        <section v-loading="resultLoading" class="report-panel" :class="{ 'is-stale-report': isRunning && hasReport }">
          <div class="report-head">
            <div>
              <div class="report-title-row">
                <h2>{{ reportTitle }}</h2>
                <el-tag v-if="isRunning && hasReport" class="stale-report-tag" size="small" effect="light" type="danger">
                  展示上次巡检结果
                </el-tag>
              </div>
              <span>{{ reportSubtitle }}</span>
            </div>
          </div>

          <div v-if="hasReport" class="summary-grid">
            <article v-for="item in summaryCards" :key="item.key" class="summary-card" :class="`is-${item.accent}`">
              <div class="summary-icon" :class="`is-${item.accent}`">
                <el-icon><component :is="item.icon" /></el-icon>
              </div>
              <div class="summary-content">
                <span>{{ item.label }}</span>
                <strong :class="item.tone">{{ item.value }}</strong>
                <small v-if="item.note">{{ item.note }}</small>
              </div>
            </article>
          </div>

          <el-empty v-else description="暂无巡检报告，请先选择集群并启动巡检" :image-size="92" class="report-empty" />

          <template v-if="hasReport">
            <el-tabs v-model="activeTab" class="report-tabs">
              <el-tab-pane v-for="tab in reportTabs" :key="tab.key" :label="tab.label" :name="tab.key" />
            </el-tabs>

            <div v-if="activeTab !== 'overview'" class="module-overview">
              <article v-for="metric in moduleMetrics(activeTab)" :key="metric.label" class="module-metric">
                <span>{{ metric.label }}</span>
                <strong>{{ metric.value }}</strong>
              </article>
            </div>

            <section class="details-section">
              <div class="details-head">
                <div>
                  <h3>检查项明细</h3>
                  <p>汇总所有模块的检查项，支持按状态快速过滤。</p>
                </div>
                <el-radio-group v-model="detailStatusFilter" size="small">
                  <el-radio-button label="all">全部</el-radio-button>
                  <el-radio-button label="error">异常</el-radio-button>
                  <el-radio-button label="warning">告警</el-radio-button>
                  <el-radio-button label="success">正常</el-radio-button>
                </el-radio-group>
              </div>

              <div class="table-shell details-table-shell">
                <el-table :data="filteredDetailRows" class="data-table" height="100%">
                  <el-table-column label="状态" width="92">
                    <template #default="{ row }">
                      <el-tag size="small" effect="dark" :type="checkStatusTag(row.status)">
                        {{ checkStatusText(row.status) }}
                      </el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column prop="category" label="类别" width="108" />
                  <el-table-column prop="name" label="检查项" min-width="150" show-overflow-tooltip />
                  <el-table-column prop="value" label="检查值" min-width="120" show-overflow-tooltip />
                  <el-table-column prop="detail" label="详情" min-width="220" show-overflow-tooltip />
                  <el-table-column prop="suggestion" label="建议" min-width="220" show-overflow-tooltip />
                </el-table>
              </div>
            </section>
          </template>
        </section>
      </main>
    </section>

    <el-drawer v-model="historyDrawerVisible" title="巡检历史" size="760px" class="inspection-history-drawer">
      <div class="history-drawer-body">
        <div class="drawer-toolbar">
          <div>
            <strong>历史报告</strong>
            <span>按当前集群筛选，可查看、导出或删除历史报告。</span>
          </div>
          <el-button class="warning-action" size="small" :icon="Refresh" :loading="historyLoading" @click="refreshHistory">
            刷新历史
          </el-button>
        </div>

        <div class="table-shell history-table-shell drawer-history-table-shell">
          <el-table :data="historyRows" v-loading="historyLoading" class="data-table history-table" height="100%">
            <el-table-column label="集群" min-width="120" show-overflow-tooltip>
              <template #default="{ row }">
                <button type="button" class="cluster-report-link" @click="selectHistoryReport(row)">
                  {{ row.clusterName }}
                </button>
              </template>
            </el-table-column>
            <el-table-column prop="createdAt" label="巡检时间" min-width="150" />
            <el-table-column label="评分" width="72" align="center">
              <template #default="{ row }">
                <span class="score-text" :class="scoreClass(row.score)">{{ formatNumber(row.score, '-') }}</span>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="88" align="center">
              <template #default="{ row }">
                <el-tag size="small" effect="light" :type="inspectionStatusTag(row.status)">
                  {{ inspectionStatusText(row.status) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="检查项" width="76" align="center">
              <template #default="{ row }">{{ row.checkCount || 0 }}</template>
            </el-table-column>
            <el-table-column label="问题" width="72" align="center">
              <template #default="{ row }">
                <span class="warning-text">{{ (row.warningCount || 0) + (row.failCount || 0) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="耗时" width="78" align="center">
              <template #default="{ row }">{{ formatDuration(row.duration) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="96" align="center">
              <template #default="{ row }">
                <div class="operation-links">
                  <el-button link class="op-link export" :loading="exportingId === row.id" @click="exportReport(row.id)">导出</el-button>
                  <el-button
                    link
                    class="op-link delete"
                    :disabled="row.status === 'running'"
                    @click="deleteHistory(row)"
                  >
                    删除
                  </el-button>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div class="pagination-row">
          <el-pagination
            v-model:current-page="historyQuery.page"
            v-model:page-size="historyQuery.pageSize"
            small
            background
            layout="total, prev, pager, next"
            :total="historyTotal"
            @current-change="loadHistory"
            @size-change="loadHistory"
          />
        </div>
      </div>
    </el-drawer>

    <el-drawer v-model="optionDrawerVisible" title="检查项配置" size="360px" class="inspection-option-drawer">
      <div class="option-list">
        <label v-for="item in inspectionOptionItems" :key="item.key" class="option-item">
          <div>
            <strong>{{ item.label }}</strong>
            <span>{{ item.description }}</span>
          </div>
          <el-switch v-model="inspectionOptions[item.key]" />
        </label>
      </div>
      <template #footer>
        <div class="drawer-actions">
          <el-button class="secondary-action" @click="selectAllOptions(true)">全选</el-button>
          <el-button class="secondary-action" @click="selectAllOptions(false)">清空</el-button>
          <el-button type="primary" class="primary-action" @click="optionDrawerVisible = false">完成</el-button>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  CaretRight,
  CircleCheck,
  CircleClose,
  Connection,
  DataAnalysis,
  Refresh,
  Setting,
  Tickets,
  Timer,
  WarningFilled
} from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'
import {
  getStoredClusterId,
  resolvePreferredClusterId,
  saveStoredClusterId,
  subscribeStoredClusterId
} from '@/utils/k8sClusterSelection'

const CLUSTER_SELECTION_SOURCE = 'k8s-inspection'

const inspectionOptionItems = [
  { key: 'checkCluster', label: '集群基础', description: '连接、版本与核心组件' },
  { key: 'checkNodes', label: '节点健康', description: 'Ready、压力、污点与容量' },
  { key: 'checkWorkloads', label: '工作负载', description: 'Deployment、Pod 与异常副本' },
  { key: 'checkNetwork', label: '网络', description: 'Service、Endpoint、Ingress' },
  { key: 'checkStorage', label: '存储', description: 'PV、PVC 与 StorageClass' },
  { key: 'checkSecurity', label: '安全', description: 'RBAC、特权容器与 root 容器' },
  { key: 'checkConfig', label: '配置', description: 'ConfigMap、Secret 与资源限制' },
  { key: 'checkCapacity', label: '容量', description: 'CPU、内存与 Pod 水位' },
  { key: 'checkEvents', label: '事件', description: '近期 Warning 与高频事件' }
]

const reportTabs = [
  { key: 'overview', label: '总览' },
  { key: 'nodeHealth', label: '节点健康' },
  { key: 'components', label: '组件状态' },
  { key: 'workloads', label: '工作负载' },
  { key: 'network', label: '网络' },
  { key: 'storage', label: '存储' },
  { key: 'security', label: '安全' },
  { key: 'config', label: '配置' },
  { key: 'capacity', label: '容量' },
  { key: 'events', label: '事件' }
]

const moduleFieldMap = {
  nodeHealth: '节点健康',
  components: '组件状态',
  workloads: '工作负载',
  network: '网络',
  storage: '存储',
  security: '安全',
  config: '配置',
  capacity: '容量',
  events: '事件'
}

const clusterLoading = ref(false)
const startLoading = ref(false)
const resultLoading = ref(false)
const historyLoading = ref(false)
const optionDrawerVisible = ref(false)
const historyDrawerVisible = ref(false)
const activeTab = ref('overview')
const detailStatusFilter = ref('all')
const selectedClusterId = ref(getStoredClusterId())
const currentInspectionId = ref('')
const exportingId = ref('')
const pollTimer = ref(null)
const autoLoadedLatestId = ref('')
let unsubscribeClusterSelection = null

const clusterOptions = ref([])
const historyRows = ref([])
const historyTotal = ref(0)
const reportPayload = ref(null)

const historyQuery = reactive({
  page: 1,
  pageSize: 10
})

const progressState = reactive({
  inspectionId: '',
  status: 'idle',
  progress: 0,
  currentStep: '',
  completedClusters: 0,
  totalClusters: 1
})

const inspectionOptions = reactive(
  inspectionOptionItems.reduce((result, item) => {
    result[item.key] = true
    return result
  }, {})
)

const selectedClusterName = computed(() => {
  const cluster = clusterOptions.value.find(item => String(item.id) === String(selectedClusterId.value))
  return cluster?.name || ''
})

const isRunning = computed(() => progressState.status === 'running')
const normalizedProgress = computed(() => Math.max(0, Math.min(100, Number(progressState.progress) || 0)))
const progressColor = computed(() => {
  if (progressState.status === 'failed') return '#ef6b73'
  if (progressState.status === 'completed') return '#3fb97f'
  return '#4c8bf5'
})

const reportInspection = computed(() => reportPayload.value?.inspection || {})
const reportResult = computed(() => reportPayload.value?.result || {})
const hasReport = computed(() => {
  const payload = reportPayload.value
  if (!payload) return false
  const inspection = payload.inspection || {}
  const result = payload.result || {}
  return !!(
    inspection.id ||
    inspection.inspectionId ||
    result.clusterId ||
    result.clusterName ||
    result.score !== undefined ||
    result.summary
  )
})
const reportScore = computed(() => {
  const backendScore = toNumber(reportResult.value.score, toNumber(reportInspection.value.score, null))
  if (backendScore > 0 || !hasCheckSummary.value) return backendScore
  return calculateFallbackScore(reportSummary.value)
})
const reportTitle = computed(() => hasReport.value ? `${reportInspection.value.clusterName || reportResult.value.clusterName || selectedClusterName.value || 'K8s 集群'} 巡检报告` : '等待巡检报告')
const reportSubtitle = computed(() => {
  if (!hasReport.value) return '完成巡检后会在这里展示评分、模块摘要和检查项明细。'
  return `${formatDateTime(reportInspection.value.createdAt)} / ${formatDuration(reportInspection.value.duration || reportResult.value.summary?.duration)}`
})

const summaryCards = computed(() => {
  const summary = reportSummary.value
  const warningCount = Number(summary.warningChecks ?? reportInspection.value.warningCount ?? 0)
  const failedCount = Number(summary.failedChecks ?? reportInspection.value.failCount ?? 0)
  return [
    { key: 'score', label: '健康评分', value: formatNumber(reportScore.value, '-'), tone: scoreTone.value, icon: DataAnalysis, accent: scoreAccent.value },
    { key: 'total', label: '检查总数', value: summary.totalChecks ?? reportInspection.value.checkCount ?? 0, icon: DataAnalysis, accent: 'brand' },
    { key: 'pass', label: '正常', value: summary.passedChecks ?? reportInspection.value.passCount ?? 0, tone: 'success-text', icon: CircleCheck, accent: 'success' },
    { key: 'problem', label: '发现问题', value: warningCount + failedCount, tone: failedCount > 0 ? 'danger-text' : 'warning-text', icon: failedCount > 0 ? CircleClose : WarningFilled, accent: failedCount > 0 ? 'danger' : 'warning', note: `告警 ${warningCount} / 异常 ${failedCount}` },
    { key: 'duration', label: '耗时', value: formatDuration(summary.duration ?? reportInspection.value.duration), icon: Timer, accent: 'muted' }
  ]
})

const scoreTone = computed(() => {
  const value = Number(reportScore.value)
  if (!Number.isFinite(value)) return ''
  if (value >= 90) return 'success-text'
  if (value >= 70) return 'warning-text'
  return 'danger-text'
})

const scoreAccent = computed(() => {
  const value = Number(reportScore.value)
  if (!Number.isFinite(value)) return 'brand'
  if (value >= 90) return 'success'
  if (value >= 70) return 'warning'
  return 'danger'
})

const reportSummary = computed(() => reportResult.value.summary || {})

const hasCheckSummary = computed(() => {
  const summary = reportSummary.value
  const total = Number(summary.totalChecks ?? reportInspection.value.checkCount)
  return Number.isFinite(total) && total > 0
})

const calculateFallbackScore = (summary = {}) => {
  const total = Number(summary.totalChecks ?? reportInspection.value.checkCount)
  const passed = Number(summary.passedChecks ?? reportInspection.value.passCount ?? 0)
  const warning = Number(summary.warningChecks ?? reportInspection.value.warningCount ?? 0)
  const failed = Number(summary.failedChecks ?? reportInspection.value.failCount ?? 0)
  if (!Number.isFinite(total) || total <= 0) return 0
  const effectivePassed = Number.isFinite(passed) && passed > 0 ? passed : Math.max(total - warning - failed, 0)
  return Math.max(0, Math.round(((effectivePassed * 100) + (warning * 60)) / total))
}

const resolveDisplayScore = ({ score, checkCount, passCount, warningCount, failCount } = {}) => {
  const backendScore = toNumber(score, null)
  const total = Number(checkCount)
  if (backendScore > 0 || !Number.isFinite(total) || total <= 0) return backendScore
  return calculateFallbackScore({
    totalChecks: total,
    passedChecks: Number(passCount) || 0,
    warningChecks: Number(warningCount) || 0,
    failedChecks: Number(failCount) || 0
  })
}

const detailRows = computed(() => {
  const modules = activeTab.value === 'overview'
    ? reportTabs.filter(tab => tab.key !== 'overview').map(tab => reportResult.value[tab.key])
    : [reportResult.value[activeTab.value]]
  return sortDetailRows(modules.flatMap(module => normalizeItems(module?.items)))
})

const filteredDetailRows = computed(() => {
  if (detailStatusFilter.value === 'all') return detailRows.value
  return detailRows.value.filter(item => normalizeCheckStatus(item.status) === detailStatusFilter.value)
})

const apiData = (response) => {
  const payload = response?.data || response || {}
  return payload.data !== undefined ? payload.data : payload
}

const parseJsonObject = (value) => {
  if (!value || typeof value !== 'string') return null
  try {
    const parsed = JSON.parse(value)
    return parsed && typeof parsed === 'object' ? parsed : null
  } catch (error) {
    console.warn('巡检报告 reportData 解析失败:', error)
    return null
  }
}

const normalizeInspectionResultPayload = (payload = {}) => {
  const source = payload && typeof payload === 'object' ? payload : {}
  const inspection = source.inspection || source
  const reportData = parseJsonObject(inspection.reportData)
  const result = source.result || reportData || {}

  return {
    inspection: {
      ...inspection,
      score: inspection.score ?? result.score,
      checkCount: inspection.checkCount ?? result.summary?.totalChecks,
      passCount: inspection.passCount ?? result.summary?.passedChecks,
      warningCount: inspection.warningCount ?? result.summary?.warningChecks,
      failCount: inspection.failCount ?? result.summary?.failedChecks,
      duration: inspection.duration ?? result.summary?.duration,
      clusterId: inspection.clusterId ?? result.clusterId,
      clusterName: inspection.clusterName ?? result.clusterName
    },
    result
  }
}

const isSuccessResponse = (response) => {
  const payload = response?.data || response || {}
  return payload.code === 0 || payload.code === 200 || payload.success === true || payload.message === 'success' || payload.message === '成功'
}

const toNumber = (value, fallback = 0) => {
  const next = Number(value)
  return Number.isFinite(next) ? next : fallback
}

const formatNumber = (value, fallback = '0') => {
  const number = Number(value)
  return Number.isFinite(number) ? Math.round(number) : fallback
}

const formatDuration = (value) => {
  const seconds = Number(value)
  if (!Number.isFinite(seconds) || seconds <= 0) return '-'
  if (seconds < 60) return `${Math.round(seconds)}s`
  const minutes = Math.floor(seconds / 60)
  const rest = Math.round(seconds % 60)
  return rest ? `${minutes}m ${rest}s` : `${minutes}m`
}

const formatDateTime = (value) => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return date.toLocaleString()
}

const normalizeItems = (items) => {
  if (!Array.isArray(items)) return []
  return items.map((item, index) => ({
    key: `${item.category || 'module'}-${item.name || index}`,
    category: item.category || '-',
    name: item.name || '-',
    status: normalizeCheckStatus(item.status),
    value: stringifyValue(item.value) || '-',
    expected: stringifyValue(item.expected) || '-',
    detail: stringifyValue(item.detail) || '-',
    suggestion: stringifyValue(item.suggestion) || '-'
  }))
}

const sortDetailRows = (rows) => {
  const order = { error: 0, warning: 1, success: 2 }
  return [...rows].sort((prev, next) => {
    const prevOrder = order[normalizeCheckStatus(prev.status)] ?? 3
    const nextOrder = order[normalizeCheckStatus(next.status)] ?? 3
    return prevOrder - nextOrder
  })
}

const stringifyValue = (value) => {
  if (value === undefined || value === null || value === '') return ''
  if (typeof value === 'object') {
    try {
      return JSON.stringify(value)
    } catch (error) {
      return '[object]'
    }
  }
  return String(value)
}

const moduleMetrics = (key) => {
  if (key === 'overview') {
    return summaryCards.value.map(item => ({ label: item.label, value: item.value }))
  }

  const module = reportResult.value[key] || {}
  const items = normalizeItems(module.items)
  const successCount = items.filter(item => item.status === 'success').length
  const warningCount = items.filter(item => item.status === 'warning').length
  const errorCount = items.filter(item => item.status === 'error').length

  return [
    { label: '模块', value: moduleFieldMap[key] || key },
    { label: '检查项', value: items.length },
    { label: '正常', value: successCount },
    { label: '告警', value: warningCount },
    { label: '异常', value: errorCount }
  ]
}

const normalizeCheckStatus = (status) => {
  const text = String(status || '').toLowerCase()
  if (['success', 'ok', 'normal', 'passed', 'pass'].includes(text)) return 'success'
  if (['error', 'failed', 'fail', 'critical', 'danger'].includes(text)) return 'error'
  if (['warning', 'warn', 'degraded'].includes(text)) return 'warning'
  return text || 'warning'
}

const checkStatusText = (status) => {
  const mapping = { success: '正常', warning: '告警', error: '异常' }
  return mapping[normalizeCheckStatus(status)] || '未知'
}

const checkStatusTag = (status) => {
  const mapping = { success: 'success', warning: 'warning', error: 'danger' }
  return mapping[normalizeCheckStatus(status)] || 'info'
}

const inspectionStatusText = (status) => {
  const mapping = {
    idle: '未开始',
    running: '运行中',
    completed: '已完成',
    failed: '失败',
    success: '已完成'
  }
  return mapping[String(status || '').toLowerCase()] || '未知'
}

const inspectionStatusTag = (status) => {
  const text = String(status || '').toLowerCase()
  if (text === 'running') return 'primary'
  if (text === 'completed' || text === 'success') return 'success'
  if (text === 'failed') return 'danger'
  return 'info'
}

const clusterStatusText = (status) => {
  const mapping = { 0: '离线', 1: '连接中', 2: '在线', 3: '异常' }
  return mapping[Number(status)] || '未知'
}

const clusterStatusTag = (status) => {
  const mapping = { 0: 'danger', 1: 'info', 2: 'success', 3: 'warning' }
  return mapping[Number(status)] || 'info'
}

const scoreClass = (score) => {
  const value = Number(score)
  if (!Number.isFinite(value)) return ''
  if (value >= 90) return 'is-good'
  if (value >= 70) return 'is-warning'
  return 'is-danger'
}

const clearPollTimer = () => {
  if (pollTimer.value) {
    clearTimeout(pollTimer.value)
    pollTimer.value = null
  }
}

const schedulePoll = () => {
  clearPollTimer()
  pollTimer.value = setTimeout(() => {
    pollInspectionProgress(currentInspectionId.value)
  }, 2000)
}

const loadClusters = async () => {
  clusterLoading.value = true
  try {
    const response = await k8sApi.getClusterList({ page: 1, size: 100 })
    if (!isSuccessResponse(response)) {
      ElMessage.error(response?.data?.message || '获取集群列表失败')
      return
    }

    const data = apiData(response)
    const clusters = Array.isArray(data?.list) ? data.list : (Array.isArray(data) ? data : [])
    clusterOptions.value = clusters.map(cluster => ({
      id: cluster.id,
      name: cluster.name || cluster.clusterName || `集群 ${cluster.id}`,
      status: Number(cluster.status),
      version: cluster.version || ''
    }))

    const resolvedClusterId = resolvePreferredClusterId(
      clusterOptions.value,
      selectedClusterId.value || getStoredClusterId()
    )

    if (resolvedClusterId) {
      selectedClusterId.value = resolvedClusterId
      saveStoredClusterId(selectedClusterId.value, CLUSTER_SELECTION_SOURCE)
    }
  } catch (error) {
    console.error('获取集群列表失败:', error)
    ElMessage.warning('无法获取集群列表，请检查后端服务')
  } finally {
    clusterLoading.value = false
  }
}

const handleClusterChange = (clusterId) => {
  saveStoredClusterId(clusterId, CLUSTER_SELECTION_SOURCE)
  historyQuery.page = 1
  reportPayload.value = null
  autoLoadedLatestId.value = ''
  loadHistory()
}

const loadHistory = async () => {
  historyLoading.value = true
  try {
    const params = {
      page: historyQuery.page,
      pageSize: historyQuery.pageSize
    }
    if (selectedClusterId.value) params.clusterId = selectedClusterId.value

    const response = await k8sApi.getInspectionHistory(params)
    if (!isSuccessResponse(response)) {
      ElMessage.error(response?.data?.message || '获取巡检历史失败')
      return
    }

    const data = apiData(response) || {}
    const list = Array.isArray(data.list) ? data.list : (Array.isArray(data) ? data : [])
    historyRows.value = list.map(item => ({
      id: item.id || item.inspectionId,
      clusterId: item.clusterId,
      clusterName: item.clusterName || '-',
      score: resolveDisplayScore(item),
      status: item.status || 'completed',
      checkCount: item.checkCount,
      passCount: item.passCount,
      warningCount: item.warningCount,
      failCount: item.failCount,
      duration: item.duration,
      createdAt: item.createdAt || item.createTime || '-'
    }))
    historyTotal.value = Number(data.total) || historyRows.value.length
    loadLatestHistoryReport()
  } catch (error) {
    console.error('获取巡检历史失败:', error)
    ElMessage.error('获取巡检历史失败')
  } finally {
    historyLoading.value = false
  }
}

const refreshHistory = () => {
  loadHistory()
}

const openHistoryDrawer = () => {
  historyDrawerVisible.value = true
  loadHistory()
}

const selectHistoryReport = async (row) => {
  if (!row?.id) return
  await loadInspectionResult(row.id)
  historyDrawerVisible.value = false
}

const loadLatestHistoryReport = () => {
  if (historyQuery.page !== 1 || !historyRows.value.length || reportPayload.value) return
  const latest = historyRows.value.find(row => row.id && row.status !== 'running')
  if (!latest || autoLoadedLatestId.value === latest.id) return
  autoLoadedLatestId.value = latest.id
  loadInspectionResult(latest.id, { silent: true })
}

const startClusterInspection = async () => {
  if (!selectedClusterId.value) {
    ElMessage.warning('请选择一个在线集群')
    return
  }

  startLoading.value = true
  try {
    const response = await k8sApi.startInspection(selectedClusterId.value, { ...inspectionOptions })
    if (!isSuccessResponse(response)) {
      ElMessage.error(response?.data?.message || '启动巡检失败')
      return
    }

    const data = apiData(response) || {}
    currentInspectionId.value = data.inspectionId || data.id
    Object.assign(progressState, {
      inspectionId: currentInspectionId.value,
      status: 'running',
      progress: 0,
      currentStep: '巡检任务已启动，正在初始化...',
      completedClusters: 0,
      totalClusters: 1
    })
    ElMessage.success('巡检任务已启动')
    schedulePoll()
  } catch (error) {
    console.error('启动巡检失败:', error)
    ElMessage.error('启动巡检失败，请检查集群凭证或后端服务')
  } finally {
    startLoading.value = false
  }
}

const pollInspectionProgress = async (inspectionId) => {
  if (!inspectionId) return
  try {
    const response = await k8sApi.getInspectionProgress(inspectionId)
    if (!isSuccessResponse(response)) {
      Object.assign(progressState, { status: 'failed', currentStep: response?.data?.message || '巡检进度查询失败' })
      return
    }

    const data = apiData(response) || {}
    Object.assign(progressState, {
      inspectionId: data.inspectionId || inspectionId,
      status: data.status || 'running',
      progress: toNumber(data.progress, progressState.progress),
      currentStep: data.currentStep || progressState.currentStep,
      completedClusters: toNumber(data.completedClusters, progressState.completedClusters),
      totalClusters: toNumber(data.totalClusters, progressState.totalClusters || 1)
    })

    if (progressState.status === 'completed') {
      clearPollTimer()
      await loadInspectionResult(inspectionId)
      await loadHistory()
      return
    }

    if (progressState.status === 'failed') {
      clearPollTimer()
      ElMessage.error(progressState.currentStep || '巡检任务失败')
      return
    }

    schedulePoll()
  } catch (error) {
    console.error('查询巡检进度失败:', error)
    Object.assign(progressState, { status: 'failed', currentStep: '巡检进度查询失败' })
    clearPollTimer()
  }
}

const loadInspectionResult = async (inspectionId, options = {}) => {
  if (!inspectionId) return
  resultLoading.value = true
  try {
    const response = await k8sApi.getInspectionResult(inspectionId)
    if (!isSuccessResponse(response)) {
      ElMessage.error(response?.data?.message || '获取巡检报告失败')
      return
    }

    const normalizedPayload = normalizeInspectionResultPayload(apiData(response))
    reportPayload.value = normalizedPayload
    currentInspectionId.value = inspectionId
    Object.assign(progressState, {
      inspectionId,
      status: normalizedPayload.inspection.status || 'completed',
      progress: toNumber(normalizedPayload.inspection.progress, 100),
      currentStep: normalizedPayload.inspection.currentStep || '巡检报告已加载',
      completedClusters: toNumber(normalizedPayload.inspection.completedClusters, 1),
      totalClusters: toNumber(normalizedPayload.inspection.totalClusters, 1)
    })
    activeTab.value = 'overview'
    detailStatusFilter.value = 'all'
    if (!options.silent) ElMessage.success('巡检报告已加载')
  } catch (error) {
    console.error('获取巡检报告失败:', error)
    ElMessage.error('获取巡检报告失败')
  } finally {
    resultLoading.value = false
  }
}

const deleteHistory = async (row) => {
  try {
    await ElMessageBox.confirm(`确定要删除巡检记录 #${row.id} 吗？`, '删除确认', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消'
    })
    const response = await k8sApi.deleteInspection(row.id)
    if (!isSuccessResponse(response)) {
      ElMessage.error(response?.data?.message || '删除巡检记录失败')
      return
    }
    ElMessage.success('巡检记录已删除')
    if (currentInspectionId.value === row.id) {
      reportPayload.value = null
      currentInspectionId.value = ''
    }
    loadHistory()
  } catch (error) {
    if (error !== 'cancel') console.error('删除巡检记录失败:', error)
  }
}

const exportReport = async (inspectionId) => {
  if (!inspectionId) return
  exportingId.value = inspectionId
  try {
    const response = await k8sApi.exportInspection(inspectionId)
    const blob = response?.data instanceof Blob
      ? response.data
      : new Blob([response?.data || response], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `cluster-inspection-${inspectionId}.xlsx`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)
    ElMessage.success('巡检报告已开始下载')
  } catch (error) {
    console.error('导出巡检报告失败:', error)
    ElMessage.error('导出巡检报告失败')
  } finally {
    exportingId.value = ''
  }
}

const selectAllOptions = (checked) => {
  inspectionOptionItems.forEach(item => {
    inspectionOptions[item.key] = checked
  })
}

onMounted(async () => {
  unsubscribeClusterSelection = subscribeStoredClusterId((clusterId, detail = {}) => {
    if (detail.source === CLUSTER_SELECTION_SOURCE || String(selectedClusterId.value || '') === String(clusterId || '')) {
      return
    }

    selectedClusterId.value = clusterId
    historyQuery.page = 1
    reportPayload.value = null
    autoLoadedLatestId.value = ''
    loadHistory()
  })

  await loadClusters()
  await loadHistory()
})

onBeforeUnmount(() => {
  clearPollTimer()
  if (unsubscribeClusterSelection) {
    unsubscribeClusterSelection()
  }
})
</script>

<style scoped>
.k8s-inspection-page {
  --ops-blue-500: #409eff;
  --ops-blue-600: #2563eb;
  --ops-green-500: #10b981;
  --ops-orange-500: #f59e0b;
  --ops-red-500: #ef4444;
  --ops-slate-950: #0f172a;
  --ops-slate-800: #172033;
  --ops-slate-600: #53657d;
  --ops-slate-500: #64748b;
  --ops-bg-surface: #ffffff;
  --ops-bg-surface-soft: #fbfdff;
  --ops-bg-subtle: #f3f7fc;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  --ops-border-inner: #dce8f5;
  --ops-border-hover: #b9d4ff;
  --ops-shadow-card: 0 8px 24px rgba(80, 124, 180, 0.07);
  --ops-shadow-card-hover: 0 10px 24px rgba(64, 158, 255, 0.1);
  --ops-shadow-hero: 0 10px 28px rgba(80, 124, 180, 0.1);
  --ops-radius-sm: 7px;
  --ops-radius-md: 8px;
  --ops-radius-inner: 10px;
  --ops-radius-card: 12px;
  --ops-radius-dialog: 15px;
  --ops-transition-fast: 160ms ease;
  --ops-text-primary: var(--ops-slate-800);
  --ops-text-heading: var(--ops-slate-950);
  --ops-text-secondary: var(--ops-slate-600);
  --ops-text-muted: var(--ops-slate-500);
  --aiops-bg-page: #f6f8fb;
  --aiops-surface-strong: var(--ops-bg-surface);
  --aiops-surface-warm: var(--ops-bg-surface-soft);
  --aiops-text-primary: var(--ops-text-primary);
  --aiops-text-secondary: var(--ops-text-secondary);
  --aiops-text-muted: var(--ops-text-muted);
  --aiops-border-cool: var(--ops-border-soft);
  --aiops-border-mid: var(--ops-border);
  --aiops-brand: var(--ops-blue-600);
  --aiops-brand-soft: rgba(37, 99, 235, 0.1);
  --aiops-success: var(--ops-green-500);
  --aiops-warning: var(--ops-orange-500);
  --aiops-danger: var(--ops-red-500);
  --inspection-page-height-offset: 124px;
  box-sizing: border-box;
  height: calc(100vh - var(--inspection-page-height-offset));
  min-height: 0;
  overflow: auto;
  padding: 7px;
  background: transparent;
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.page-card {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  padding: 0;
  overflow: hidden;
  border: 1px solid var(--ops-border);
  border-radius: var(--ops-radius-card);
  background: rgba(255, 255, 255, 0.96);
  box-shadow: var(--ops-shadow-card);
}

.page-card::before {
  display: none;
}

.page-hero {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-shrink: 0;
  padding: 8px 12px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-surface);
}

.hero-title-group {
  display: flex;
  align-items: center;
  flex: 1;
  gap: 10px;
  min-width: 0;
}

.ops-page-header__icon {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  place-items: center;
  color: var(--ops-blue-600);
  background: transparent;
}

.ops-page-header__icon .el-icon {
  font-size: 30px;
}

.ops-page-header__content {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.ops-page-header__divider {
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  background: var(--ops-border-soft);
}

.page-hero h1,
.report-head h2,
.details-head h3 {
  margin: 0;
  color: var(--aiops-text-primary);
  font-weight: 700;
  line-height: 1.2;
}

.page-hero h1 {
  flex: 0 0 auto;
  font-size: 20px;
}

.page-description,
.details-head p,
.report-head span {
  margin: 0;
  overflow: hidden;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.cluster-select {
  width: 224px;
}

.cluster-option {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.primary-action,
.brand-action,
.success-action,
.secondary-action,
.warning-action {
  min-width: 88px;
  height: 30px;
  border-radius: var(--ops-radius-md);
  font-size: 12px;
  font-weight: 600;
  transition:
    color var(--ops-transition-fast),
    background-color var(--ops-transition-fast),
    border-color var(--ops-transition-fast),
    box-shadow var(--ops-transition-fast),
    transform var(--ops-transition-fast),
    opacity var(--ops-transition-fast);
}

.primary-action,
.brand-action {
  border-color: var(--ops-blue-600) !important;
  background: linear-gradient(135deg, #5aa8ff, var(--ops-blue-600)) !important;
  color: #ffffff !important;
  box-shadow: 0 8px 16px rgba(64, 158, 255, 0.16);
}

.success-action {
  border-color: #059669 !important;
  background: #059669 !important;
  color: #ffffff !important;
  box-shadow: 0 8px 16px rgba(16, 185, 129, 0.14);
}

.warning-action {
  border-color: #d97706 !important;
  background: #d97706 !important;
  color: #ffffff !important;
  box-shadow: 0 8px 16px rgba(245, 158, 11, 0.14);
}

.secondary-action {
  border: 1px solid var(--ops-border-inner) !important;
  background: #ffffff !important;
  color: var(--aiops-text-secondary);
  box-shadow: none;
}

.primary-action:hover,
.primary-action:focus,
.brand-action:hover,
.brand-action:focus,
.success-action:hover,
.success-action:focus,
.secondary-action:hover,
.secondary-action:focus,
.warning-action:hover,
.warning-action:focus {
  transform: translateY(-1px);
}

.primary-action:hover,
.primary-action:focus,
.brand-action:hover,
.brand-action:focus {
  border-color: #1d4ed8 !important;
  background: linear-gradient(135deg, #409eff, #1d4ed8) !important;
  box-shadow: var(--ops-shadow-card-hover);
}

.success-action:hover,
.success-action:focus {
  border-color: #047857 !important;
  background: #047857 !important;
  box-shadow: 0 10px 20px rgba(16, 185, 129, 0.16);
}

.warning-action:hover,
.warning-action:focus {
  border-color: #b45309 !important;
  background: #b45309 !important;
  box-shadow: 0 10px 20px rgba(245, 158, 11, 0.16);
}

.secondary-action:hover,
.secondary-action:focus {
  border-color: var(--ops-border-hover) !important;
  color: var(--aiops-brand);
  background: #fbfdff !important;
}

.primary-action:active,
.brand-action:active,
.success-action:active,
.secondary-action:active,
.warning-action:active {
  transform: translateY(0);
}

.primary-action:focus-visible,
.brand-action:focus-visible,
.success-action:focus-visible,
.secondary-action:focus-visible,
.warning-action:focus-visible,
.cluster-report-link:focus-visible {
  outline: 2px solid rgba(64, 158, 255, 0.28);
  outline-offset: 2px;
}

.primary-action.is-disabled,
.brand-action.is-disabled,
.success-action.is-disabled,
.secondary-action.is-disabled,
.warning-action.is-disabled,
.primary-action:disabled,
.brand-action:disabled,
.success-action:disabled,
.secondary-action:disabled,
.warning-action:disabled {
  transform: none;
  opacity: 0.58;
  box-shadow: none;
}

.report-workspace {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
  gap: 8px;
  padding: 8px 10px 10px;
}

.status-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-shrink: 0;
  min-height: 46px;
  padding: 8px 12px;
  border: 1px solid var(--ops-border-inner);
  border-left: 3px solid var(--ops-slate-500);
  border-radius: var(--ops-radius-inner);
  background: var(--ops-bg-surface-soft);
}

.status-strip.is-running {
  border-left-color: var(--ops-blue-600);
}

.status-strip.is-completed,
.status-strip.is-success {
  border-left-color: var(--ops-green-500);
}

.status-strip.is-failed {
  border-left-color: var(--ops-red-500);
}

.status-main {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 0;
  min-width: 0;
}

.status-main > div {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.status-title-row,
.report-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.status-main strong {
  overflow: hidden;
  color: var(--aiops-text-primary);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-title-row em {
  flex-shrink: 0;
  color: var(--aiops-brand);
  font-size: 13px;
  font-style: normal;
  font-weight: 800;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
}

.status-main span {
  margin-top: 2px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-progress {
  display: block;
  width: min(280px, 28vw);
  align-items: center;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  font-weight: 700;
}

.report-panel {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
  padding: 10px 12px;
  overflow: hidden;
  border: 1px solid var(--ops-border-inner);
  border-radius: var(--ops-radius-inner);
  background: var(--ops-bg-surface-soft);
}

.report-panel.is-stale-report {
  background: #f8fbff;
}

.report-head,
.details-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  flex-shrink: 0;
}

.report-head h2 {
  font-size: 16px;
}

.report-title-row h2 {
  min-width: 0;
}

.stale-report-tag {
  border-color: rgba(239, 107, 115, 0.36) !important;
  background: rgba(239, 107, 115, 0.12) !important;
  color: var(--aiops-danger) !important;
  font-weight: 800;
}

.details-head h3 {
  font-size: 15px;
}

.table-shell {
  min-height: 0;
  padding: 8px;
  border: 1px solid var(--ops-border-inner);
  border-radius: var(--ops-radius-inner);
  background: #ffffff;
  overflow: auto;
}

.history-table-shell {
  flex: 1;
  margin-top: 14px;
  overflow: hidden;
}

.drawer-history-table-shell {
  min-height: 420px;
}

.details-table-shell {
  flex: 1;
  margin-top: 8px;
  min-height: 460px;
}

.data-table {
  min-width: 860px;
  overflow: hidden;
  border: 1px solid #e3ebf5;
  border-radius: var(--ops-radius-md);
  background: var(--aiops-surface-strong);
}

.history-table {
  width: 100%;
  min-width: 0;
}

.history-table :deep(.el-table__body-wrapper),
.history-table :deep(.el-scrollbar__wrap),
.history-table :deep(.el-scrollbar__view) {
  overflow-x: hidden !important;
}

.history-table :deep(.el-scrollbar__bar.is-horizontal) {
  display: none;
}

.history-table :deep(.el-table__cell) {
  padding: 10px 0;
}

.history-table :deep(.cell) {
  padding: 0 8px;
}

.history-table :deep(.el-tag) {
  height: 22px;
  padding: 0 8px;
}

.data-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.data-table :deep(.el-table__header th),
.data-table :deep(.el-table__fixed-right-patch) {
  background: var(--ops-bg-subtle) !important;
  border-bottom: 1px solid var(--ops-border-soft);
}

.data-table :deep(.el-table__header th .cell) {
  color: var(--aiops-text-muted) !important;
  font-size: 12px;
  font-weight: 700 !important;
}

.data-table :deep(.el-table__cell) {
  border-bottom-color: #eef3f9;
  font-size: 12px;
}

.data-table :deep(.el-table__body tr:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

.data-table :deep(.el-tag--dark.el-tag--success) {
  border-color: var(--aiops-success);
  background: var(--aiops-success);
  color: #ffffff;
}

.data-table :deep(.el-tag--dark.el-tag--warning) {
  border-color: var(--aiops-warning);
  background: var(--aiops-warning);
  color: #ffffff;
}

.data-table :deep(.el-tag--dark.el-tag--danger) {
  border-color: var(--aiops-danger);
  background: var(--aiops-danger);
  color: #ffffff;
}

.operation-links {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
}

.operation-links :deep(.el-button) {
  height: auto;
  min-height: 0;
  padding: 0;
  margin: 0;
  border: none;
  background: transparent;
  font-size: 12px;
  font-weight: 700;
}

.cluster-report-link {
  max-width: 100%;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--aiops-brand);
  font: inherit;
  font-size: 13px;
  font-weight: 700;
  line-height: 1.4;
  text-align: left;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: color var(--ops-transition-fast), opacity var(--ops-transition-fast);
}

.cluster-report-link:hover,
.cluster-report-link:focus {
  color: #3f75df;
  opacity: 0.86;
  outline: none;
}

.op-link.view {
  color: var(--aiops-brand);
}

.op-link.export {
  color: #f3a33a;
}

.op-link.delete {
  color: var(--aiops-danger);
}

.pagination-row {
  display: flex;
  justify-content: flex-end;
  flex-shrink: 0;
  padding-top: 10px;
}

.score-text.is-good {
  color: var(--aiops-success);
}

.score-text.is-warning,
.warning-text {
  color: var(--aiops-warning);
}

.score-text.is-danger,
.danger-text {
  color: var(--aiops-danger);
}

.score-text {
  font-weight: 800;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 8px;
  margin: 8px 0;
}

.summary-card,
.module-metric {
  min-width: 0;
  padding: 8px 11px;
  border: 1px solid var(--ops-border-inner);
  border-radius: var(--ops-radius-inner);
  background: linear-gradient(180deg, #ffffff 0%, #fbfdff 100%);
}

.summary-card {
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr);
  align-items: center;
  gap: 9px;
  min-height: 78px;
  border-top-width: 3px;
  transition:
    border-color var(--ops-transition-fast),
    box-shadow var(--ops-transition-fast),
    transform var(--ops-transition-fast);
}

.summary-card:hover {
  transform: translateY(-1px);
  border-color: var(--ops-border-hover);
  box-shadow: var(--ops-shadow-card-hover);
}

.summary-card.is-brand {
  border-top-color: var(--ops-blue-600);
}

.summary-card.is-success {
  border-top-color: var(--ops-green-500);
}

.summary-card.is-warning {
  border-top-color: var(--ops-orange-500);
}

.summary-card.is-danger {
  border-top-color: var(--ops-red-500);
}

.summary-card.is-muted {
  border-top-color: var(--ops-slate-500);
}

.summary-icon {
  width: 34px;
  height: 34px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--ops-radius-inner);
  border: 1px solid var(--ops-border-inner);
  background: var(--ops-bg-surface-soft);
  color: var(--aiops-brand);
  font-size: 16px;
}

.summary-icon.is-brand {
  color: var(--aiops-brand);
  background: #eef6ff;
  border-color: #cfe2ff;
}

.summary-icon.is-success {
  color: var(--aiops-success);
  background: #eaf8ef;
  border-color: #c8efd8;
}

.summary-icon.is-warning {
  color: var(--aiops-warning);
  background: #fff4e5;
  border-color: #fed7aa;
}

.summary-icon.is-danger {
  color: var(--aiops-danger);
  background: #fff0f0;
  border-color: #fecaca;
}

.summary-icon.is-muted {
  color: var(--ops-slate-500);
  background: var(--ops-bg-subtle);
  border-color: var(--ops-border-soft);
}

.summary-content {
  min-width: 0;
}

.summary-card span,
.module-metric span {
  display: block;
  color: var(--aiops-text-muted);
  font-size: 11.5px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.summary-card strong,
.module-metric strong {
  display: block;
  margin-top: 4px;
  color: var(--aiops-text-primary);
  font-size: 18px;
  font-weight: 800;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  line-height: 1.1;
  word-break: break-word;
}

.summary-card small {
  display: block;
  margin-top: 2px;
  color: var(--aiops-text-muted);
  font-size: 11px;
  font-weight: 600;
  line-height: 1.2;
}

.success-text {
  color: var(--aiops-success) !important;
}

.report-empty {
  flex: 1;
}

.report-tabs {
  flex-shrink: 0;
}

.report-tabs :deep(.el-tabs__header) {
  margin-bottom: 6px;
}

.report-tabs :deep(.el-tabs__item) {
  height: 34px;
  line-height: 34px;
}

.report-tabs :deep(.el-tabs__item) {
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

.report-tabs :deep(.el-tabs__active-bar) {
  background-color: var(--ops-blue-500);
}

.report-tabs :deep(.el-tabs__item.is-active) {
  color: var(--ops-blue-600);
}

.module-overview {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 8px;
}

.details-section {
  display: flex;
  flex: 1.7;
  min-height: 0;
  flex-direction: column;
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--ops-border-soft);
}

.details-head :deep(.el-radio-button__inner) {
  height: 28px;
  padding: 0 10px;
  border-color: var(--ops-border-inner);
  font-size: 12px;
  line-height: 26px;
}

.details-head :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) {
  border-color: var(--ops-blue-500);
  background: var(--ops-blue-500);
  box-shadow: -1px 0 0 0 var(--ops-blue-500);
  color: #ffffff;
}

.history-drawer-body {
  display: flex;
  height: 100%;
  min-height: 0;
  flex-direction: column;
}

.drawer-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  flex-shrink: 0;
}

.drawer-toolbar strong,
.drawer-toolbar span {
  display: block;
}

.drawer-toolbar strong {
  color: var(--aiops-text-primary);
  font-size: 16px;
}

.drawer-toolbar span {
  margin-top: 5px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.option-list {
  display: grid;
  gap: 12px;
}

.option-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 10px 12px;
  border: 1px solid var(--ops-border-inner);
  border-radius: var(--ops-radius-inner);
  background: var(--ops-bg-surface-soft);
}

.option-item strong {
  display: block;
  color: var(--aiops-text-primary);
  font-size: 13px;
}

.option-item span {
  display: block;
  margin-top: 5px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.drawer-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.k8s-inspection-page :deep(.el-input__wrapper),
.k8s-inspection-page :deep(.el-select__wrapper) {
  min-height: 32px;
  border-radius: var(--ops-radius-md);
  background: var(--ops-bg-surface-soft);
  box-shadow: 0 0 0 1px #d9e5f2 inset;
  transition:
    background-color var(--ops-transition-fast),
    box-shadow var(--ops-transition-fast);
}

.k8s-inspection-page :deep(.el-input__wrapper:hover),
.k8s-inspection-page :deep(.el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px var(--ops-border-hover) inset;
}

.k8s-inspection-page :deep(.el-input__wrapper.is-focus),
.k8s-inspection-page :deep(.el-select__wrapper.is-focused) {
  background: #ffffff;
  box-shadow: 0 0 0 1px var(--ops-blue-500) inset, 0 0 0 3px rgba(64, 158, 255, 0.12);
}

.k8s-inspection-page :deep(.el-tag) {
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
}

.k8s-inspection-page :deep(.el-progress-bar__outer) {
  background: #edf3fb;
}

:global(.inspection-history-drawer.el-drawer),
:global(.inspection-option-drawer.el-drawer) {
  overflow: hidden;
  border-left: 1px solid #d7e6f6;
  background: #ffffff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.inspection-history-drawer .el-drawer__header),
:global(.inspection-option-drawer .el-drawer__header) {
  align-items: center;
  margin-bottom: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.inspection-history-drawer .el-drawer__body),
:global(.inspection-option-drawer .el-drawer__body) {
  padding: 14px 16px;
}

:global(.inspection-history-drawer .el-drawer__body) {
  overflow: hidden;
}

:global(.inspection-option-drawer .el-drawer__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
}

@media (max-width: 1180px) {
  .page-card {
    overflow: hidden;
  }

  .summary-grid,
  .module-overview {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 960px) {
  .summary-grid,
  .module-overview {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .ops-page-header__content {
    align-items: flex-start;
    flex-direction: column;
    gap: 3px;
  }

  .ops-page-header__divider {
    display: none;
  }
}

@media (max-width: 768px) {
  .k8s-inspection-page {
    --inspection-page-height-offset: 104px;
    height: calc(100vh - var(--inspection-page-height-offset));
    padding: 6px;
  }

  .page-card {
    height: 100%;
    min-height: 0;
    border-radius: var(--ops-radius-card);
  }

  .page-hero,
  .report-head,
  .details-head,
  .header-actions,
  .status-strip,
  .drawer-toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .cluster-select,
  .header-actions .el-button,
  .drawer-toolbar .el-button {
    width: 100%;
  }

  .status-progress {
    width: 100%;
  }

  .summary-grid,
  .module-overview {
    grid-template-columns: 1fr;
  }

  .report-workspace {
    padding: 8px;
  }

  .report-panel {
    overflow: auto;
  }

  .details-table-shell {
    min-height: 320px;
  }
}
</style>
