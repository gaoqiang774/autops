<script setup>
import { ref, reactive, onMounted, onUnmounted, computed, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Search,
  Refresh,
  View,
  Edit,
  More,
  ArrowLeft,
  Delete,
  CopyDocument,
  ArrowUp,
  ArrowDown,
  MagicStick,
  TrendCharts,
  FullScreen
} from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'
import aiopsApi from '@/api/aiops'
import CodeEditor from '@/components/CodeEditor.vue'
import PodDetailTabs from './PodDetailTabs.vue'
import PodFileManagerDialog from './PodFileManagerDialog.vue'
import storage from '@/utils/storage'
import { buildWebSocketUrl } from '@/utils/url'
import { getYamlContentFromApiResponse } from '../utils/yaml'
import * as echarts from 'echarts'

const route = useRoute()
const router = useRouter()

// Pod状态相关函数
const getPodStatusTag = (status) => {
  const statusMap = {
    'Running': 'success',
    'Pending': 'warning', 
    'Succeeded': 'success',
    'Failed': 'danger',
    'Unknown': 'info'
  }
  return statusMap[status] || 'info'
}

const getPodStatusText = (status) => {
  const textMap = {
    'Running': '运行中',
    'Pending': '等待中',
    'Succeeded': '成功',
    'Failed': '失败',
    'Unknown': '未知'
  }
  return textMap[status] || status
}

// 格式化运行时间 - 转换为可读格式 (如: 4d8h, 2h30m, 45m)
const formatRunningTime = (runningTimeStr) => {
  if (!runningTimeStr) return '-'

  // 尝试匹配格式: 117h40m 或 4d 或 40m 等
  let totalMinutes = 0

  // 匹配天数
  const daysMatch = runningTimeStr.match(/(\d+)d/)
  if (daysMatch) {
    totalMinutes += parseInt(daysMatch[1]) * 24 * 60
  }

  // 匹配小时
  const hoursMatch = runningTimeStr.match(/(\d+)h/)
  if (hoursMatch) {
    totalMinutes += parseInt(hoursMatch[1]) * 60
  }

  // 匹配分钟
  const minutesMatch = runningTimeStr.match(/(\d+)m/)
  if (minutesMatch) {
    totalMinutes += parseInt(minutesMatch[1])
  }

  // 如果没有匹配到任何时间,返回原始字符串
  if (totalMinutes === 0 && !daysMatch && !hoursMatch && !minutesMatch) {
    return runningTimeStr
  }

  // 转换为天、小时、分钟
  const days = Math.floor(totalMinutes / (24 * 60))
  const hours = Math.floor((totalMinutes % (24 * 60)) / 60)
  const minutes = totalMinutes % 60

  // 构建可读格式
  let result = ''
  if (days > 0) {
    result += `${days}d`
  }
  if (hours > 0) {
    result += `${hours}h`
  }
  if (minutes > 0 && days === 0) {
    // 只有在没有天数时才显示分钟,避免太长
    result += `${minutes}m`
  }

  return result || '0m'
}

// 格式化时间戳
const formatTimestamp = (timestamp) => {
  if (!timestamp) return '-'
  
  try {
    const date = new Date(timestamp)
    return date.toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit'
    })
  } catch (error) {
    return timestamp
  }
}


const loading = ref(false)
const activeTab = ref('containers')

const getRequestedTab = () => {
  const tab = Array.isArray(route.query.tab) ? route.query.tab[0] : route.query.tab
  return tab === 'logs' ? tab : ''
}

// 路由参数
const routeParams = reactive({
  clusterId: route.params.clusterId,
  namespace: route.params.namespace,
  podName: route.params.podName,
  workloadType: route.query.workloadType,
  workloadName: route.query.workloadName
})

// 响应式数据
const podDetail = ref({})
const containers = ref([])
const allRelatedPods = ref([]) // 工作负载下的所有相关Pod
const events = ref([])
const historyVersions = ref([])
const createEmptyRelatedResources = () => ({
  pod: null,
  network: {
    services: [],
    ingresses: []
  },
  storage: {
    pvcs: [],
    pvs: [],
    storageClasses: []
  },
  config: {
    configMaps: [],
    secrets: []
  },
  warnings: []
})

const relatedResources = ref(createEmptyRelatedResources())
const relatedResourcesLoading = reactive({
  network: false,
  storage: false,
  config: false
})
const relatedResourcesLoaded = reactive({
  network: false,
  storage: false,
  config: false
})
const logs = ref('')
const yamlContent = ref('')
const podFileBrowserVisible = ref(false)
const currentFileBrowserPod = ref({
  namespace: '',
  podName: '',
  containerName: '',
  containerNames: []
})

// YAML搜索相关状态
const yamlSearchText = ref('')
const yamlSearchResults = ref([])
const yamlSearchCurrentIndex = ref(-1)

// 日志搜索相关状态
const logsSearchText = ref('')
const logsSearchResults = ref([])
const logsSearchCurrentIndex = ref(-1)

// 日志相关状态
const selectedContainerForLogs = ref('')
const selectedLogContainer = ref('')
const followLogs = ref(getRequestedTab() === 'logs')
const currentLogs = ref('')
const logsLoading = ref(false)
const lastLogRefreshTime = ref(null)
const showPreviousLogs = ref(false)
const logsUpdateKey = ref(0) // 用于强制CodeEditor重新渲染
const logsEditorRef = ref(null) // CodeEditor组件引用
let openingLogsFromAction = false

const scrollLogsIntoView = async () => {
  await nextTick()
  logsEditorRef.value?.scrollIntoView({ behavior: 'auto', block: 'start' })
}

const getLogContainerName = container => (
  typeof container === 'string' ? container : container?.name
)

const escapeCharacter = String.fromCharCode(27)
const controlSequenceIntroducer = String.fromCharCode(155)
const ansiOscPattern = new RegExp(`${escapeCharacter}\\][^\\u0007]*(?:\\u0007|${escapeCharacter}\\\\)`, 'g')
const ansiCsiPattern = new RegExp(`${escapeCharacter}\\[[0-?]*[ -/]*[@-~]`, 'g')
const ansiCsi8BitPattern = new RegExp(`${controlSequenceIntroducer}[0-?]*[ -/]*[@-~]`, 'g')
const ansiSingleEscapePattern = new RegExp(`${escapeCharacter}[@-_]`, 'g')

const sanitizeLogText = value => String(value ?? '')
  .replace(ansiOscPattern, '')
  .replace(ansiCsiPattern, '')
  .replace(ansiCsi8BitPattern, '')
  .replace(ansiSingleEscapePattern, '')

const getPodLogContainerNames = pod => {
  const podContainers = pod?.containers?.length
    ? pod.containers
    : (pod?.rawData?.spec?.containers || [])

  return [...new Set(podContainers.map(getLogContainerName).filter(Boolean))]
}

const selectDefaultLogContainer = podName => {
  const selectedPod = allRelatedPods.value.find(pod => pod.name === podName)
  const containerNames = getPodLogContainerNames(selectedPod)

  if (!containerNames.includes(selectedLogContainer.value)) {
    selectedLogContainer.value = containerNames[0] || ''
  }

  return selectedLogContainer.value
}

// WebSocket 相关状态
const wsConnection = ref(null)
const wsStatus = ref('disconnected') // disconnected, connecting, connected, error
const autoScroll = ref(true) // 自动滚动到底部
const logsBuffer = ref([]) // 日志缓冲区
let flushTimer = null // 刷新定时器

// 日志轮转配置
const maxLogLines = ref(500) // 最大保留日志行数，固定500行
const logRotationEnabled = ref(true) // 是否启用日志轮转

// 弹框控制
const dialogStates = reactive({
  logsVisible: false,
  yamlVisible: false,
  terminalVisible: false,
  monitoringVisible: false,
  rollbackVisible: false,
  aiAnalysisVisible: false
})

// 回滚相关数据
const rollbackForm = reactive({
  targetVersion: null,
  versionInfo: {}
})
const rollbackLoading = ref(false)

// 监控相关状态
const selectedPodForMonitoring = ref(null)
const monitoringData = ref({
  cpu: { used: '-', limit: '-', percentage: 0 },
  memory: { used: '-', limit: '-', percentage: 0 },
  network: { rx: '-', tx: '-' },
  disk: { used: '-', limit: '-', percentage: 0 },
  containers: [],
  timestamp: null
})
const monitoringLoading = ref(false)
const detailMetricsLoading = ref(false)
const detailMetricsReady = ref(false)
const detailMetricsError = ref('')
const podMetricHistory = ref(null)
const podHistoryChartRefs = {}
const podHistoryChartInstances = {}
const historyChartDialogVisible = ref(false)
const expandedHistoryChart = ref(null)
const expandedHistoryChartRef = ref(null)
let expandedHistoryChartInstance = null
const podMetricRange = ref('6h')
const podMetricScope = 'workload'
const podMetricRangeOptions = [
  { label: '近 1 小时', value: '1h' },
  { label: '近 3 小时', value: '3h' },
  { label: '近 6 小时', value: '6h' },
  { label: '近 24 小时', value: '24h' },
  { label: '近 3 天', value: '3d' },
  { label: '近 7 天', value: '7d' },
  { label: '近 50 天', value: '50d' }
]
const podMetricChartOrder = ['cpuUsageRate', 'memoryUsageRate', 'network']
const podMetricChartMeta = {
  cpuUsageRate: { title: 'Pod容器CPU使用率', unit: '%', precision: 2, color: '#f3b24b' },
  memoryUsageRate: { title: 'Pod容器内存WSS使用率', unit: '%', precision: 2, color: '#8b5cf6' },
  network: { title: 'Pod每秒网络带宽', unit: 'MiB/s', precision: 3, color: '#4c8bf5' }
}
const historyChartPalette = ['#4c8bf5', '#3fb97f', '#f3b24b', '#8b5cf6', '#ef6b73', '#14b8a6', '#6366f1', '#f97316']

// 当前选中的容器
const selectedContainer = ref(null)
const selectedPodForAiAnalysis = ref(null)
const aiAnalysisLoading = ref(false)
const aiAnalysisResult = ref(null)
const aiAnalysisPrompt = ref('请分析该 Pod 是否存在异常重启、事件 Warning、应用日志错误或 CPU、内存飙高问题，并给出下一步容器内实时诊断建议。')
const aiAnalysisTaskId = ref('')
const aiAnalysisTaskStatus = ref('')
const aiAnalysisPodStatus = ref('unknown')
const aiAnalysisCurrentStep = ref('queued')
const aiAnalysisProgress = ref(0)
const aiAnalysisMessage = ref('')
const aiAnalysisEvents = ref([])
let aiAnalysisEventSource = null
let aiAnalysisPollingTimer = null

// 标签展开状态
const labelsExpanded = ref(false)

// 计算属性
const podStatus = computed(() => {
  return podDetail.value.status || podDetail.value.phase || 'Unknown'
})

const podLabels = computed(() => {
  return podDetail.value.labels || {}
})

const podCreationTime = computed(() => {
  return podDetail.value.createdAt || podDetail.value.creationTimestamp || ''
})

const aiAnalysisText = computed(() => {
  const result = aiAnalysisResult.value || {}
  const directText = String(
    result.analysis ||
    result.result ||
    result.message ||
    result.summary ||
    result.conclusion ||
    result.diagnosis ||
    ''
  ).trim()
  if (directText) return directText

  const lines = []
  if (result.status) lines.push(`状态：${result.status}`)
  if (result.severity) lines.push(`严重级别：${result.severity}`)
  if (result.rootCause || result.root_cause) lines.push(`根因：${result.rootCause || result.root_cause}`)
  const nextChecks = result.nextChecks || result.next_checks
  if (Array.isArray(nextChecks) && nextChecks.length) {
    lines.push('下一步检查：')
    lines.push(...nextChecks.map(item => `- ${typeof item === 'string' ? item : item.title || item.message || JSON.stringify(item)}`))
  }
  const remediation = result.remediation || result.remediations
  if (Array.isArray(remediation) && remediation.length) {
    lines.push('处理建议：')
    lines.push(...remediation.map(item => `- ${typeof item === 'string' ? item : item.title || item.message || JSON.stringify(item)}`))
  } else if (remediation) {
    lines.push(`处理建议：${remediation}`)
  }
  return lines.join('\n').trim()
})

const aiAnalysisEvidence = computed(() => {
  return aiAnalysisResult.value?.evidence || aiAnalysisResult.value?.data?.evidence || null
})

const aiAnalysisHotspots = computed(() => {
  const evidence = aiAnalysisEvidence.value || {}
  return evidence.resourceHotspots || evidence.resource_hotspots || aiAnalysisResult.value?.resourceHotspots || []
})

const aiAnalysisCollectionErrors = computed(() => {
  const evidence = aiAnalysisEvidence.value || {}
  return evidence.collectionErrors || evidence.collection_errors || aiAnalysisResult.value?.collectionErrors || []
})

const aiAnalysisContainerName = computed(() => {
  return selectedPodForAiAnalysis.value?.containers?.[0]?.name || containers.value?.[0]?.name || '-'
})

const isAiAnalysisFinished = computed(() => {
  return aiAnalysisTaskStatus.value === 'succeeded' && !!aiAnalysisResult.value
})

const normalizeAiPodAnalysisStatus = value => {
  const status = String(value || '').toLowerCase()
  return ['normal', 'abnormal', 'unknown'].includes(status) ? status : ''
}

const normalizeAiTaskStatus = value => {
  const status = String(value || '').toLowerCase()
  return ['queued', 'running', 'succeeded', 'failed'].includes(status) ? status : ''
}

const aiAnalysisStatusMeta = computed(() => {
  if (aiAnalysisTaskStatus.value === 'failed') {
    return { label: 'AI 分析诊断失败', type: 'danger', className: 'is-abnormal' }
  }

  if (!isAiAnalysisFinished.value) {
    return { label: 'AI 分析诊断中', type: 'primary', className: 'is-running' }
  }

  const status = normalizeAiPodAnalysisStatus(
    aiAnalysisResult.value?.podStatus
    || aiAnalysisResult.value?.pod_status
    || aiAnalysisResult.value?.status
    || aiAnalysisPodStatus.value
  ) || 'unknown'
  const map = {
    normal: { label: 'Pod 分析结果正常', type: 'success', className: 'is-normal' },
    abnormal: { label: 'Pod 分析结果异常', type: 'danger', className: 'is-abnormal' },
    unknown: { label: '证据不足或状态未知', type: 'info', className: 'is-unknown' }
  }
  return map[status] || map.unknown
})

const aiAnalysisStepList = [
  { key: 'queued', label: '任务创建' },
  { key: 'start', label: '开始分析' },
  { key: 'connect_cluster', label: '连接集群' },
  { key: 'pod_status', label: 'Pod 状态' },
  { key: 'logs_current', label: '当前日志' },
  { key: 'logs_previous', label: 'Previous 日志' },
  { key: 'events', label: 'Pod 事件' },
  { key: 'metrics', label: '资源指标' },
  { key: 'evidence', label: '异常证据' },
  { key: 'ai_analysis', label: 'AI 诊断' },
  { key: 'result', label: '完成' }
]

const aiAnalysisActiveStep = computed(() => {
  const index = aiAnalysisStepList.findIndex(item => item.key === aiAnalysisCurrentStep.value)
  if (index >= 0) return index
  const progress = Number(aiAnalysisProgress.value) || 0
  if (progress >= 100) return aiAnalysisStepList.length - 1
  return Math.max(0, Math.min(aiAnalysisStepList.length - 1, Math.floor(progress / 10)))
})

const aiAnalysisProgressStatus = computed(() => {
  if (aiAnalysisTaskStatus.value === 'failed') return 'exception'
  if (aiAnalysisTaskStatus.value === 'succeeded' || aiAnalysisProgress.value >= 100) return 'success'
  return undefined
})

const normalizeAiReasonText = value => String(value || '')
  .replace(/\r/g, '')
  .replace(/^#+\s*/gm, '')
  .replace(/\*\*/g, '')
  .replace(/`/g, '')
  .trim()

const aiAnalysisSummaryMeta = computed(() => {
  if (!isAiAnalysisFinished.value) {
    return {
      summary: '',
      severity: '',
      rootCause: ''
    }
  }

  const result = aiAnalysisResult.value || {}
  const text = aiAnalysisText.value
  const lines = String(text || '').split('\n').map(item => item.trim()).filter(Boolean)
  const getValueAfterColon = line => {
    if (!line) return ''
    const matched = line.match(/^[^:：]+[:：]\s*(.+)$/)
    return matched?.[1] || ''
  }

  const summaryLine = lines.find(line => /事件摘要|摘要|结论/.test(line) && /[:：]/.test(line)) || ''
  const severityLine = lines.find(line => /严重性|严重级别|severity/.test(line)) || ''
  const rootCauseLine = lines.find(line => /根本原因|根因|root cause/i.test(line)) || ''

  const summary = normalizeAiReasonText(
    getValueAfterColon(summaryLine)
    || result.summary
    || result.conclusion
    || ''
  )
  const severity = normalizeAiReasonText(
    getValueAfterColon(severityLine)
    || result.severity
    || ''
  )
  const rootCause = normalizeAiReasonText(
    getValueAfterColon(rootCauseLine)
    || result.rootCause
    || result.root_cause
    || ''
  )

  return {
    summary,
    severity,
    rootCause
  }
})

// 期望pod数量
const expectedPodCount = computed(() => {
  // 如果有相关Pod数据，返回实际运行的Pod数量
  if (allRelatedPods.value && allRelatedPods.value.length > 0) {
    return allRelatedPods.value.length
  }

  // 否则尝试从pod详情中获取
  if (podDetail.value.spec?.ownerReferences) {
    const deployment = podDetail.value.spec.ownerReferences.find(ref =>
      ref.kind === 'ReplicaSet' || ref.kind === 'Deployment'
    )
    if (deployment) {
      return podDetail.value.expectedReplicas || 1
    }
  }
  return podDetail.value.expectedReplicas || 1
})

// 显示的标签数量限制
const maxVisibleLabels = 3

// 可见的标签
const visibleLabels = computed(() => {
  const labelEntries = Object.entries(podLabels.value)
  if (!labelsExpanded.value && labelEntries.length > maxVisibleLabels) {
    return Object.fromEntries(labelEntries.slice(0, maxVisibleLabels))
  }
  return podLabels.value
})

// 是否有更多标签
const hasMoreLabels = computed(() => {
  return Object.keys(podLabels.value).length > maxVisibleLabels
})

// 隐藏的标签数量
const hiddenLabelsCount = computed(() => {
  const totalLabels = Object.keys(podLabels.value).length
  return Math.max(0, totalLabels - maxVisibleLabels)
})

// 工作负载名称
const normalizeWorkloadTypeForApi = type => {
  const normalized = String(type || '').toLowerCase()
  const map = {
    deployments: 'deployment',
    deployment: 'deployment',
    statefulsets: 'statefulset',
    statefulset: 'statefulset',
    daemonsets: 'daemonset',
    daemonset: 'daemonset',
    jobs: 'job',
    job: 'job',
    cronjobs: 'cronjob',
    cronjob: 'cronjob'
  }
  return map[normalized] || normalized
}

const normalizePodForTable = (pod = {}) => ({
  name: pod.name || pod.metadata?.name,
  status: pod.phase || pod.status?.phase || pod.status || 'Unknown',
  restartCount: pod.restarts || pod.restartCount || pod.containerStatuses?.reduce((sum, item) => sum + (item.restartCount || 0), 0) || 0,
  nodeName: pod.nodeName || pod.spec?.nodeName || 'Unknown',
  podIP: pod.podIP || pod.status?.podIP || 'Unknown',
  hostIP: pod.hostIP || pod.status?.hostIP || 'Unknown',
  age: pod.age || formatAge(pod.createdAt || pod.creationTimestamp || pod.metadata?.creationTimestamp),
  runningTime: pod.runningTime || '',
  containers: pod.containers || pod.spec?.containers || [],
  resources: pod.resources || { requests: { cpu: '', memory: '' }, limits: { cpu: '', memory: '' } },
  resourceLimits: pod.resourceLimits || pod.resources?.limits || {},
  currentUsage: pod.currentUsage || {},
  labels: pod.labels || pod.metadata?.labels || {},
  conditions: pod.conditions || pod.status?.conditions || [],
  rawData: pod
})

const workloadName = computed(() => {
  if (routeParams.workloadName) return routeParams.workloadName

  // 先尝试从ownerReferences获取
  const ownerRefs = podDetail.value.metadata?.ownerReferences ||
                   podDetail.value.spec?.ownerReferences ||
                   podDetail.value.ownerReferences || []

  // 查找Deployment
  const deployment = ownerRefs.find(ref => ref.kind === 'Deployment')
  if (deployment) {
    return deployment.name
  }

  // 查找ReplicaSet，然后推断Deployment名称
  const directOwner = ownerRefs.find(ref => ['StatefulSet', 'DaemonSet', 'Job'].includes(ref.kind))
  if (directOwner) {
    return directOwner.name
  }

  const replicaSet = ownerRefs.find(ref => ref.kind === 'ReplicaSet')
  if (replicaSet) {
    // ReplicaSet命名规则: deployment-name-pod-template-hash
    const parts = replicaSet.name.split('-')
    if (parts.length >= 2) {
      return parts.slice(0, -1).join('-')
    }
  }

  // 最后从Pod名称推断Deployment名称
  const podNameParts = routeParams.podName.split('-')
  if (podNameParts.length >= 3) {
    // Pod命名规律: deployment-name-replicaset-hash-pod-hash
    return podNameParts.slice(0, -2).join('-')
  }

  // 如果都无法推断，返回Pod名称
  return routeParams.podName
})

// 当前Pod信息（用于表格显示）
const currentPodInfo = computed(() => {
  if (!podDetail.value || Object.keys(podDetail.value).length === 0) {
    return {
      name: routeParams.podName,
      status: 'Loading...',
      restartCount: '-',
      nodeName: '-',
      podIP: '-',
      hostIP: '-',
      age: '-',
      runningTime: '-',
      containers: [],
      resources: { requests: { cpu: '', memory: '' }, limits: { cpu: '', memory: '' } },
      labels: {},
      conditions: []
    }
  }

  return {
    name: routeParams.podName,
    status: podDetail.value.status || podDetail.value.phase || 'Unknown',
    restartCount: podDetail.value.restartCount || 0,
    nodeName: podDetail.value.spec?.nodeName || podDetail.value.nodeName || 'Unknown',
    podIP: podDetail.value.podIP || podDetail.value.status?.podIP ||
           (podDetail.value.status === 'Pending' ? '等待分配' :
            podDetail.value.status === 'Terminating' ? '已释放' : 'Unknown'),
    hostIP: podDetail.value.hostIP || podDetail.value.status?.hostIP || 'Unknown',
    age: formatAge(podDetail.value.metadata?.creationTimestamp || podDetail.value.createdAt),
    runningTime: podDetail.value.runningTime || '',
    containers: containers.value || podDetail.value.spec?.containers || [],
    resources: podDetail.value.resources || { requests: { cpu: '', memory: '' }, limits: { cpu: '', memory: '' } },
    labels: podDetail.value.labels || {},
    conditions: podDetail.value.conditions || []
  }
})

const normalizeHistoryTimestamp = value => {
  if (!value && value !== 0) return ''
  if (typeof value === 'number') {
    return new Date(value > 100000000000 ? value : value * 1000).toISOString()
  }
  return String(value)
}

const normalizeHistoryPoint = point => {
  if (Array.isArray(point)) {
    return {
      timestamp: normalizeHistoryTimestamp(point[0]),
      value: Number(point[1]) || 0
    }
  }

  return {
    timestamp: normalizeHistoryTimestamp(point?.timestamp || point?.time || point?.ts),
    value: Number(point?.value ?? point?.val ?? 0) || 0
  }
}

const getHistoryChartColor = key => {
  const colors = {
    cpu: '#4c8bf5',
    cpuCores: '#4c8bf5',
    memory: '#3fb97f',
    memoryWorkingSet: '#3fb97f',
    cpuUsageRate: '#f3b24b',
    memoryUsageRate: '#8b5cf6',
    network: '#4c8bf5'
  }
  return colors[key] || '#4c8bf5'
}

const normalizePrometheusHistoryCharts = charts => {
  if (!Array.isArray(charts)) return []

  return charts
    .filter(chart => podMetricChartOrder.includes(chart.key || chart.name || chart.title))
    .map(chart => {
      const key = chart.key || chart.name || chart.title
      const meta = podMetricChartMeta[key] || {}
      const rawSeries = Array.isArray(chart.series) && chart.series.length
        ? chart.series
        : [{ name: chart.title || chart.key, points: chart.points || chart.values || [] }]

      const series = rawSeries
        .map(item => ({
          name: item.name || item.labels?.container || item.labels?.pod || chart.title || chart.key,
          labels: item.labels || {},
          points: (Array.isArray(item.points) ? item.points : item.values || [])
            .map(normalizeHistoryPoint)
            .filter(point => point.timestamp)
        }))
        .filter(item => item.points.length)

      const primaryPoints = series[0]?.points || []
      const latest = primaryPoints.length ? primaryPoints[primaryPoints.length - 1].value : null

      return {
        key,
        title: chart.title || meta.title || chart.name || chart.key,
        unit: chart.unit || meta.unit || '',
        precision: Number.isInteger(chart.precision) ? chart.precision : (meta.precision ?? 2),
        color: meta.color || getHistoryChartColor(key),
        series,
        points: primaryPoints,
        latest
      }
    })
    .filter(chart => chart.key && chart.series.length)
    .sort((left, right) => podMetricChartOrder.indexOf(left.key) - podMetricChartOrder.indexOf(right.key))
}

const podHistoryCharts = computed(() => {
  const history = podMetricHistory.value
  if (!history) return []

  const prometheusCharts = normalizePrometheusHistoryCharts(Array.isArray(history) ? history : history.charts)
  if (prometheusCharts.length) return prometheusCharts

  const chartConfig = [
    { key: 'cpu', title: 'CPU 使用量', unit: 'cores', precision: 3, color: '#4c8bf5' },
    { key: 'memory', title: '内存使用量', unit: 'GB', precision: 3, color: '#3fb97f' },
    { key: 'cpuUsageRate', title: 'CPU 使用率', unit: '%', precision: 2, color: '#f3b24b' },
    { key: 'memoryUsageRate', title: '内存使用率', unit: '%', precision: 2, color: '#8b5cf6' }
  ]

  return chartConfig
    .map(config => {
      const points = Array.isArray(history[config.key])
        ? history[config.key].map(normalizeHistoryPoint).filter(point => point.timestamp)
        : []
      return {
        ...config,
        series: [{ name: config.title, points }],
        points,
        latest: points.length ? points[points.length - 1].value : null
      }
    })
    .filter(chart => chart.points.length)
})

const hasPodHistoryCharts = computed(() => podHistoryCharts.value.length > 0)

const podMetricPodNames = computed(() => {
  const names = podMetricHistory.value?.podNames
  return Array.isArray(names) ? names : []
})

const podMetricScopeLabel = computed(() => {
  const scope = podMetricHistory.value?.scope || podMetricScope
  return scope === 'pod' ? '单 Pod' : '工作负载'
})

// 切换标签展开状态
const toggleLabelsExpanded = () => {
  labelsExpanded.value = !labelsExpanded.value
}

const setPodHistoryChartRef = (key, el) => {
  if (el) {
    podHistoryChartRefs[key] = el
  } else {
    delete podHistoryChartRefs[key]
  }
}

const formatHistoryTime = (time, includeDate = false) => {
  if (!time) return ''
  const date = new Date(time)
  if (Number.isNaN(date.getTime())) return time
  return date.toLocaleString('zh-CN', {
    month: includeDate ? '2-digit' : undefined,
    day: includeDate ? '2-digit' : undefined,
    hour: '2-digit',
    minute: '2-digit',
    hour12: false
  })
}

const shouldShowHistoryDate = () => String(podMetricRange.value || '').includes('d')

const formatHistoryValue = (value, chart) => {
  if (value === null || value === undefined) return '--'
  const numberValue = Number(value)
  const formatted = Number.isFinite(numberValue)
    ? numberValue.toFixed(chart.precision)
    : value
  return chart.unit ? `${formatted}${chart.unit}` : formatted
}

const historySeriesColor = (chart, seriesName, index) => {
  if (chart.key === 'network' && String(seriesName || '').startsWith('流入:')) {
    return '#3fb97f'
  }
  if (chart.key === 'network' && String(seriesName || '').startsWith('流出:')) {
    return '#4c8bf5'
  }
  return historyChartPalette[index % historyChartPalette.length]
}

const buildPodHistoryChartOption = (chart, options = {}) => ({
  color: (chart.series || []).map((series, index) => historySeriesColor(chart, series.name, index)),
  legend: !options.expanded && chart.series?.length > 1 ? {
    top: 0,
    right: 6,
    itemWidth: 8,
    itemHeight: 8,
    textStyle: {
      color: '#8d97a8',
      fontSize: 11
    }
  } : undefined,
  grid: {
    top: options.expanded ? 18 : (chart.series?.length > 1 ? 28 : 14),
    right: options.expanded ? 26 : 16,
    bottom: options.expanded ? 44 : 26,
    left: options.expanded ? 54 : 42
  },
  tooltip: {
    trigger: 'axis',
    backgroundColor: 'rgba(15, 23, 42, 0.88)',
    borderWidth: 0,
    textStyle: {
      color: '#fff',
      fontSize: 12
    },
    formatter(params) {
      const first = params?.[0]
      if (!first) return ''
      const lines = params
        .filter(item => item.value !== null && item.value !== undefined)
        .map(item => `${item.marker}${item.seriesName}: ${formatHistoryValue(item.value, chart)}`)
      return `${formatHistoryTime(first.axisValue, true)}<br/>${lines.join('<br/>')}`
    }
  },
  xAxis: {
    type: 'category',
    boundaryGap: false,
    data: getHistoryChartTimestamps(chart),
    axisTick: {
      show: false
    },
    axisLine: {
      lineStyle: {
        color: 'rgba(148, 163, 184, 0.55)'
      }
    },
    axisLabel: {
      color: '#8d97a8',
      fontSize: options.expanded ? 12 : 11,
      hideOverlap: true,
      formatter(value) {
        return formatHistoryTime(value, options.expanded || shouldShowHistoryDate())
      }
    }
  },
  yAxis: {
    type: 'value',
    min: chart.unit === '%' ? 0 : null,
    splitNumber: 3,
    axisLabel: {
      color: '#8d97a8',
      fontSize: options.expanded ? 12 : 11,
      formatter(value) {
        return chart.unit === '%' ? `${value}%` : value
      }
    },
    splitLine: {
      lineStyle: {
        color: 'rgba(226, 232, 240, 0.88)',
        type: 'dashed'
      }
    }
  },
  dataZoom: options.expanded ? [
    { type: 'inside', throttle: 50 },
    {
      type: 'slider',
      height: 18,
      bottom: 8,
      borderColor: 'rgba(203, 213, 225, 0.8)',
      fillerColor: 'rgba(59, 130, 246, 0.12)',
      handleStyle: { color: '#94a3b8' },
      textStyle: { color: '#64748b' }
    }
  ] : undefined,
  series: [
    ...getHistoryChartSeries(chart, options)
  ]
})

const getHistoryChartTimestamps = chart => {
  const timestamps = new Set()
  ;(chart.series || []).forEach(series => {
    series.points.forEach(point => timestamps.add(point.timestamp))
  })
  return Array.from(timestamps).sort((left, right) => new Date(left) - new Date(right))
}

const getHistoryChartSeries = (chart, options = {}) => {
  const timestamps = getHistoryChartTimestamps(chart)

  return (chart.series || [{ name: chart.title, points: chart.points || [] }]).map((series, index) => {
    const pointMap = new Map(series.points.map(point => [point.timestamp, point.value]))
    const color = historySeriesColor(chart, series.name, index)

    return {
      name: series.name || chart.title,
      type: 'line',
      smooth: true,
      symbol: 'none',
      connectNulls: false,
      lineStyle: {
        width: options.expanded ? 1 : 1.2,
        color,
        type: chart.key === 'network' && String(series.name || '').startsWith('流出:') ? 'dashed' : 'solid'
      },
      areaStyle: !options.expanded && index === 0 ? {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: `${color}18` },
          { offset: 1, color: `${color}02` }
        ])
      } : undefined,
      data: timestamps.map(timestamp => pointMap.has(timestamp) ? pointMap.get(timestamp) : null)
    }
  })
}

const disposePodHistoryCharts = () => {
  Object.values(podHistoryChartInstances).forEach(chart => {
    chart?.dispose()
  })
  Object.keys(podHistoryChartInstances).forEach(key => {
    delete podHistoryChartInstances[key]
  })
}

const renderPodHistoryCharts = () => {
  nextTick(() => {
    const activeKeys = new Set(podHistoryCharts.value.map(chart => chart.key))
    Object.keys(podHistoryChartInstances).forEach(key => {
      if (!activeKeys.has(key)) {
        podHistoryChartInstances[key]?.dispose()
        delete podHistoryChartInstances[key]
      }
    })

    podHistoryCharts.value.forEach(chart => {
      const chartDom = podHistoryChartRefs[chart.key]
      if (!chartDom) return
      if (!podHistoryChartInstances[chart.key]) {
        podHistoryChartInstances[chart.key] = echarts.init(chartDom, null, { renderer: 'canvas' })
      }
      podHistoryChartInstances[chart.key].setOption(buildPodHistoryChartOption(chart), true)
      podHistoryChartInstances[chart.key].resize()
    })

    if (historyChartDialogVisible.value && expandedHistoryChart.value) {
      const latestExpandedChart = podHistoryCharts.value.find(chart => chart.key === expandedHistoryChart.value.key)
      if (latestExpandedChart) {
        expandedHistoryChart.value = latestExpandedChart
      }
    }
    renderExpandedHistoryChart()
  })
}

const handlePodHistoryResize = () => {
  Object.values(podHistoryChartInstances).forEach(chart => {
    chart?.resize()
  })
  expandedHistoryChartInstance?.resize()
}

const getSeriesNumericValues = series => (
  (series?.points || [])
    .map(point => Number(point.value))
    .filter(value => Number.isFinite(value))
)

const getHistorySeriesCurrent = series => {
  const values = getSeriesNumericValues(series)
  return values.length ? values[values.length - 1] : null
}

const getHistorySeriesMax = series => {
  const values = getSeriesNumericValues(series)
  return values.length ? Math.max(...values) : null
}

const getHistorySeriesAvg = series => {
  const values = getSeriesNumericValues(series)
  if (!values.length) return null
  return values.reduce((sum, value) => sum + value, 0) / values.length
}

const expandedHistorySeriesStats = computed(() => {
  const chart = expandedHistoryChart.value
  if (!chart) return []
  return (chart.series || []).map((series, index) => ({
    name: series.name || chart.title,
    color: historySeriesColor(chart, series.name, index),
    max: getHistorySeriesMax(series),
    avg: getHistorySeriesAvg(series),
    current: getHistorySeriesCurrent(series)
  }))
})

const renderExpandedHistoryChart = () => {
  if (!historyChartDialogVisible.value || !expandedHistoryChart.value) return

  nextTick(() => {
    const chartDom = expandedHistoryChartRef.value
    if (!chartDom) return
    if (!expandedHistoryChartInstance) {
      expandedHistoryChartInstance = echarts.init(chartDom, null, { renderer: 'canvas' })
    }
    expandedHistoryChartInstance.setOption(buildPodHistoryChartOption(expandedHistoryChart.value, { expanded: true }), true)
    expandedHistoryChartInstance.resize()
  })
}

const openHistoryChartDialog = chart => {
  expandedHistoryChart.value = chart
  historyChartDialogVisible.value = true
  renderExpandedHistoryChart()
}

const disposeExpandedHistoryChart = () => {
  expandedHistoryChartInstance?.dispose()
  expandedHistoryChartInstance = null
}

const handleHistoryChartDialogClosed = () => {
  disposeExpandedHistoryChart()
  expandedHistoryChart.value = null
}

const mergeRelatedResources = (data = {}) => {
  const current = relatedResources.value || createEmptyRelatedResources()
  relatedResources.value = {
    pod: data.pod || current.pod,
    network: {
      services: data.network ? (data.network.services || []) : (current.network?.services || []),
      ingresses: data.network ? (data.network.ingresses || []) : (current.network?.ingresses || [])
    },
    storage: {
      pvcs: data.storage ? (data.storage.pvcs || []) : (current.storage?.pvcs || []),
      pvs: data.storage ? (data.storage.pvs || []) : (current.storage?.pvs || []),
      storageClasses: data.storage ? (data.storage.storageClasses || []) : (current.storage?.storageClasses || [])
    },
    config: {
      configMaps: data.config ? (data.config.configMaps || []) : (current.config?.configMaps || []),
      secrets: data.config ? (data.config.secrets || []) : (current.config?.secrets || [])
    },
    warnings: Array.isArray(data.warnings) ? data.warnings : (current.warnings || [])
  }
}

const getRelatedResources = async (include = 'network,storage,config', options = {}) => {
  const includeList = String(include || 'network,storage,config')
    .split(',')
    .map(item => item.trim())
    .filter(Boolean)
  const loadingKeys = includeList.filter(key => relatedResourcesLoading[key] !== undefined)

  try {
    loadingKeys.forEach(key => {
      relatedResourcesLoading[key] = true
      relatedResourcesLoaded[key] = false
    })

    const response = await k8sApi.getPodRelatedResources(
      routeParams.clusterId,
      routeParams.namespace,
      routeParams.podName,
      {
        include: includeList.join(','),
        detail: options.detail || 'summary'
      }
    )
    const responseData = response.data || response
    if (responseData.code === 200 || responseData.success || responseData.data) {
      const data = responseData.data || responseData
      mergeRelatedResources(data)
      loadingKeys.forEach(key => {
        relatedResourcesLoaded[key] = true
      })
    } else {
      throw new Error(responseData.message || '获取Pod关联资源失败')
    }
  } catch (error) {
    console.error('获取Pod关联资源失败:', error)
    ElMessage.error(error.message || '获取Pod关联资源失败')
  } finally {
    loadingKeys.forEach(key => {
      relatedResourcesLoading[key] = false
    })
  }
}

const getTabName = tab => {
  if (typeof tab === 'string') return tab
  if (typeof tab === 'number') return String(tab)
  return tab?.paneName || tab?.name || tab?.props?.name || ''
}

const loadRelatedResourceTab = tab => {
  const tabName = getTabName(tab)
  if (!['network', 'storage', 'config'].includes(tabName)) return
  if (relatedResourcesLoaded[tabName] || relatedResourcesLoading[tabName]) return
  getRelatedResources(tabName)
}

const refreshRelatedResources = include => {
  if (!include) return
  getRelatedResources(include)
}

const resetRelatedResources = () => {
  relatedResources.value = createEmptyRelatedResources()
  Object.keys(relatedResourcesLoaded).forEach(key => {
    relatedResourcesLoaded[key] = false
  })
}

// 监听标签页切换
const handleTabChange = (tabName) => {
  const currentTab = getTabName(tabName)
  // 切换标签页时清空搜索状态
  if (currentTab !== 'logs') {
    clearLogsSearch()
  }
  loadRelatedResourceTab(currentTab)
}

// 页面初始化
onMounted(async () => {
  if (routeParams.clusterId && routeParams.namespace && routeParams.podName) {
    const prioritizeLogs = getRequestedTab() === 'logs'
    if (prioritizeLogs) {
      activeTab.value = 'logs'
    }

    await handleQuery({ prioritizeLogs })

    if (prioritizeLogs) {
      await scrollLogsIntoView()
    }
  }
  window.addEventListener('resize', handlePodHistoryResize)
})

// 页面卸载时清理
onUnmounted(() => {
  window.removeEventListener('resize', handlePodHistoryResize)
  disposePodHistoryCharts()
  disposeExpandedHistoryChart()
  // 断开 WebSocket 连接
  disconnectWebSocket()
  stopAiAnalysisPolling()
  closeAiAnalysisStream()
})

// 监听路由参数变化
watch(() => [route.params.clusterId, route.params.namespace, route.params.podName, route.query.workloadType, route.query.workloadName, route.query.tab], () => {
  console.log('路由参数变化，重新加载数据')
  // 断开 WebSocket 连接
  routeParams.clusterId = route.params.clusterId
  routeParams.namespace = route.params.namespace
  routeParams.podName = route.params.podName
  routeParams.workloadType = route.query.workloadType
  routeParams.workloadName = route.query.workloadName
  disconnectWebSocket()
  if (routeParams.clusterId && routeParams.namespace && routeParams.podName) {
    const prioritizeLogs = getRequestedTab() === 'logs'
    if (prioritizeLogs) {
      activeTab.value = 'logs'
    }
    handleQuery({ prioritizeLogs })
  }
}, { immediate: false })

// 监听实时跟踪选项变化
watch(() => followLogs.value, (newValue) => {
  console.log('🔄 实时跟踪选项变化:', newValue)
  if (!newValue) {
    // 取消实时跟踪时断开 WebSocket
    console.log('🔌 关闭实时跟踪,断开WebSocket连接')
    disconnectWebSocket()
    // 确保状态被重置
    wsStatus.value = 'disconnected'
  } else if (selectedContainerForLogs.value && !openingLogsFromAction) {
    // 启用实时跟踪时重新加载日志
    console.log('✅ 启用实时跟踪,重新加载日志')
    handleRefreshLogs()
  }
})

// 监听日志更新key变化，强制CodeEditor更新
watch(() => logsUpdateKey.value, () => {
  // 触发CodeEditor的强制更新
  if (logsEditorRef.value) {
    // 尝试调用CodeEditor的内部更新方法
    nextTick(() => {
      // 通过修改一个无关的属性来触发重渲染
      const container = document.querySelector('.logs-editor-container textarea')
      if (container) {
        // 触发输入事件，让CodeEditor知道内容变化了
        container.dispatchEvent(new Event('input', { bubbles: true }))
      }
    })
  }
})

// 监听标签页切换 - 当切换到日志标签页时自动加载日志
watch(() => activeTab.value, (newTab) => {
  console.log('🔄 标签页切换到:', newTab)
  loadRelatedResourceTab(newTab)

  if (newTab === 'logs') {
    console.log('🔄 切换到日志标签页，检查是否需要加载日志')
    const realtimeRefreshPending = !followLogs.value && !!selectedContainerForLogs.value
    followLogs.value = true

    if (openingLogsFromAction) {
      return
    }

    scrollLogsIntoView()

    // 延迟一点确保DOM已更新
    nextTick(() => {
      // 开启实时后由 followLogs 监听器负责加载，避免重复建立连接。
      if (realtimeRefreshPending) {
        return
      }

      if (selectedContainerForLogs.value && !currentLogs.value) {
        console.log('🔄 自动加载日志，因为当前无日志内容')
        handleRefreshLogs()
      } else if (!selectedContainerForLogs.value && allRelatedPods.value.length > 0) {
        console.log('🔄 自动选择Pod并加载日志')
        selectedContainerForLogs.value = allRelatedPods.value[0].name
        setTimeout(() => {
          handleRefreshLogs()
        }, 100)
      }
    })
  }
})

const loadSecondaryPodData = async () => {
  // 日志页首屏不依赖这些数据，后台加载即可。
  await getEvents()

  if (allRelatedPods.value.length === 0) {
    console.log('🔄 [Pod详情页面] 开始获取相关Pod列表')
    await getRelatedPods()
  } else {
    console.log('✅ 已有相关Pod数据，跳过重复获取')
  }

  await getHistoryVersions()
  await loadDetailMetrics()
}

// 获取Pod详情
const handleQuery = async (options = false) => {
  const forceRefresh = typeof options === 'boolean' ? options : !!options.forceRefresh
  const prioritizeLogs = typeof options === 'object' && !!options.prioritizeLogs

  try {
    loading.value = true
    detailMetricsReady.value = false
    detailMetricsError.value = ''
    podMetricHistory.value = null
    disposePodHistoryCharts()
    resetRelatedResources()
    console.log(forceRefresh ? '🔄 强制刷新Pod详情数据...' : '📥 获取Pod详情数据...')
    console.log('🔍 [Pod详情页面] 请求参数:', {
      clusterId: routeParams.clusterId,
      namespace: routeParams.namespace,
      podName: routeParams.podName
    })

    // 从Pod名称推断Deployment名称
    const podNameParts = routeParams.podName.split('-')
    let deploymentName = null
    if (podNameParts.length >= 3) {
      // Pod命名规律: deployment-name-replicaset-hash-pod-hash
      deploymentName = podNameParts.slice(0, -2).join('-')
      console.log('🔍 从Pod名称推断Deployment:', deploymentName)
    }

    let response, responseData
    let loadedFromWorkloadQuery = false
    const queryWorkloadType = normalizeWorkloadTypeForApi(routeParams.workloadType)

    if (queryWorkloadType && routeParams.workloadName) {
      try {
        response = await k8sApi.getWorkloadPods(
          routeParams.clusterId,
          routeParams.namespace,
          queryWorkloadType,
          routeParams.workloadName
        )
        responseData = response.data || response
        const pods = Array.isArray(responseData.data)
          ? responseData.data
          : (responseData.data?.pods || responseData.data?.list || [])

        if ((responseData.code === 200 || responseData.success) && pods.length > 0) {
          const currentPod = pods.find(pod => (pod.name || pod.metadata?.name) === routeParams.podName) || pods[0]
          podDetail.value = currentPod
          containers.value = currentPod.containers || currentPod.spec?.containers || []
          allRelatedPods.value = pods.map(normalizePodForTable)
          loadedFromWorkloadQuery = true

          if (!selectedContainerForLogs.value) {
            selectedContainerForLogs.value = routeParams.podName
          }
        } else {
          throw new Error(responseData.message || 'Workload pod list is empty')
        }
      } catch (workloadQueryError) {
        console.log('⚠️ 通过路由工作负载信息获取Pod列表失败，回退到原逻辑:', workloadQueryError.message)
      }
    }

    if (!loadedFromWorkloadQuery && deploymentName) {
      try {
        console.log('🔍 使用工作负载API获取Pod详情...')
        response = await k8sApi.getWorkloadPods(
          routeParams.clusterId,
          routeParams.namespace,
          'deployment',
          deploymentName
        )
        responseData = response.data || response
        console.log('工作负载Pod列表API响应:', responseData)

        if (responseData.code === 200 && responseData.data) {
          // 从Pod列表中找到当前Pod
          const currentPod = responseData.data.find(pod => pod.name === routeParams.podName)
          if (currentPod) {
            podDetail.value = currentPod
            containers.value = currentPod.containers || []
            console.log('✅ 从工作负载Pod列表中找到目标Pod:', currentPod.name)

            // 同时设置所有相关Pod
            allRelatedPods.value = responseData.data.map(pod => ({
              name: pod.name,
              status: pod.status || pod.phase || 'Unknown',
              restartCount: pod.restartCount || 0,
              nodeName: pod.nodeName || 'Unknown',
              podIP: pod.podIP || 'Unknown',
              hostIP: pod.hostIP || 'Unknown',
              age: pod.age || formatAge(pod.createdAt),
              runningTime: pod.runningTime || '',
              containers: pod.containers || [],
              resources: pod.resources || { requests: { cpu: '', memory: '' }, limits: { cpu: '', memory: '' } },
              resourceLimits: pod.resourceLimits || pod.resources?.limits || {},
              currentUsage: pod.currentUsage || {},
              labels: pod.labels || {},
              conditions: pod.conditions || [],
              rawData: pod
            }))
            console.log(`✅ 设置了 ${allRelatedPods.value.length} 个相关Pod`)

            // 自动选择当前Pod用于日志显示
            if (!selectedContainerForLogs.value && allRelatedPods.value.length > 0) {
              selectedContainerForLogs.value = routeParams.podName
              console.log('🔄 自动选择当前Pod用于日志:', selectedContainerForLogs.value)
            }
          } else {
            throw new Error('在工作负载Pod列表中未找到目标Pod')
          }
        } else {
          throw new Error(responseData.message || '工作负载API调用失败')
        }
      } catch (workloadError) {
        console.log('⚠️ 工作负载API失败，尝试其他方式:', workloadError.message)
        // 如果工作负载API失败，构造基本的Pod信息
        podDetail.value = {
          name: routeParams.podName,
          status: 'Unknown',
          metadata: { name: routeParams.podName, namespace: routeParams.namespace }
        }
        containers.value = []
      }
    } else if (!loadedFromWorkloadQuery) {
      console.log('⚠️ 无法从Pod名称推断Deployment，构造基本信息')
      podDetail.value = {
        name: routeParams.podName,
        status: 'Unknown',
        metadata: { name: routeParams.podName, namespace: routeParams.namespace }
      }
      containers.value = []
    }

    if (podDetail.value && podDetail.value.name) {
      if (prioritizeLogs) {
        if (!selectedContainerForLogs.value) {
          selectedContainerForLogs.value = routeParams.podName
        }

        // 日志页只需要 Pod 列表和容器名即可开始拉日志。
        if (allRelatedPods.value.length === 0) {
          console.log('🔄 [日志页] 先获取相关Pod列表用于日志容器选择')
          await getRelatedPods()
        }

        selectDefaultLogContainer(selectedContainerForLogs.value)
        activeTab.value = 'logs'
        await nextTick()
        await handleRefreshLogs()

        loadSecondaryPodData().catch(error => {
          console.warn('后台加载Pod补充数据失败:', error)
        })
      } else {
        await loadSecondaryPodData()
      }
    } else {
      ElMessage.error('获取Pod详情失败')
    }
  } catch (error) {
    console.error('获取Pod详情失败:', error)
    ElMessage.error('获取Pod详情失败，请检查网络连接')
  } finally {
    loading.value = false
  }
}

const loadDetailMetrics = async () => {
  if (!routeParams.clusterId || !routeParams.namespace || !routeParams.podName) {
    detailMetricsReady.value = false
    detailMetricsError.value = ''
    return
  }

  detailMetricsLoading.value = true
  detailMetricsError.value = ''

  try {
    const response = await k8sApi.getPodPrometheusMetrics(routeParams.clusterId, routeParams.namespace, routeParams.podName, {
      range: podMetricRange.value,
      scope: podMetricScope
    })
    const responseData = response.data || response

    if (responseData.code === 200 && responseData.data) {
      const metrics = responseData.data
      podMetricHistory.value = Array.isArray(metrics.charts) && metrics.charts.length
        ? metrics
        : metrics.history || null
      detailMetricsReady.value = hasPodHistoryCharts.value
      if (Array.isArray(metrics.warnings) && metrics.warnings.length) {
        console.warn('Pod Prometheus趋势指标查询告警:', metrics.warnings)
      }
      renderPodHistoryCharts()
      return
    }
    detailMetricsError.value = responseData.message || '暂无 CPU / 内存监控数据'
  } catch (error) {
    console.warn('获取 Pod 详情资源使用量失败:', error)
    detailMetricsError.value = error.response?.data?.message || error.message || '暂无 CPU / 内存监控数据'
  } finally {
    detailMetricsLoading.value = false
  }

  detailMetricsReady.value = false
  podMetricHistory.value = null
  disposePodHistoryCharts()
}

// 获取工作负载下的所有相关Pod
const getRelatedPods = async () => {
  try {
    console.log('🔍 获取相关Pod列表...')
    console.log('📊 当前Pod详情数据:', {
      podName: routeParams.podName,
      metadata: podDetail.value.metadata,
      labels: podDetail.value.metadata?.labels || podDetail.value.labels,
      ownerReferences: podDetail.value.metadata?.ownerReferences || podDetail.value.ownerReferences,
      // 调试: 显示Pod详情的所有顶级字段
      allFields: Object.keys(podDetail.value)
    })

    console.log('📊 Pod详情完整数据结构:', podDetail.value)
    
    // 从当前Pod的ownerReferences获取工作负载信息
    const ownerRefs = podDetail.value.ownerReferences || podDetail.value.metadata?.ownerReferences || []
    console.log('🔍 [Pod详情页面] Pod的ownerReferences:', ownerRefs)
    let labelSelector = ''
    let replicaSetRef = null
    let deploymentRef = null
    
    if (ownerRefs.length === 0) {
      console.log('⚠️ 当前Pod没有ownerReferences，尝试通过标签获取相关Pod')
      // 尝试通过app标签获取相关Pod
      const labels = podDetail.value.labels || podDetail.value.metadata?.labels || {}
      console.log('🔍 [Pod详情页面] 可用的标签:', labels)

      const appLabel = labels['app']
      const nameLabel = labels['app.kubernetes.io/name']
      
      if (appLabel) {
        labelSelector = `app=${appLabel}`
      } else if (nameLabel) {
        labelSelector = `app.kubernetes.io/name=${nameLabel}`
      }
      
      if (!labelSelector) {
        console.log('⚠️ 无法构建标签选择器，使用当前Pod')
        // 构造当前Pod数据结构
        allRelatedPods.value = [{
          name: routeParams.podName,
          status: podDetail.value.status || podDetail.value.phase || 'Unknown',
          restartCount: podDetail.value.restartCount || 0,
          nodeName: podDetail.value.spec?.nodeName || 'Unknown',
          podIP: podDetail.value.podIP || podDetail.value.status?.podIP || (podDetail.value.status === 'Pending' ? '等待分配' : podDetail.value.status === 'Terminating' ? '已释放' : 'Unknown'),
          hostIP: podDetail.value.hostIP || podDetail.value.status?.hostIP || 'Unknown',
          age: formatAge(podDetail.value.metadata?.creationTimestamp),
          runningTime: podDetail.value.runningTime || '',
          containers: containers.value || podDetail.value.spec?.containers || [],
          resources: { requests: { cpu: '', memory: '' }, limits: { cpu: '', memory: '' } },
          labels: podDetail.value.metadata?.labels || {},
          conditions: podDetail.value.status?.conditions || [],
          rawData: podDetail.value
        }]
        return
      }
    } else {
      // 查找ReplicaSet或直接的Deployment引用
      replicaSetRef = ownerRefs.find(ref => ref.kind === 'ReplicaSet')
      deploymentRef = ownerRefs.find(ref => ref.kind === 'Deployment')

      console.log('🔍 [Pod详情页面] 找到的owner引用:', {
        replicaSet: replicaSetRef?.name,
        deployment: deploymentRef?.name
      })
      
      if (replicaSetRef) {
        console.log('📋 通过ReplicaSet获取相关Pod:', replicaSetRef.name)
      } else if (deploymentRef) {
        console.log('📋 通过Deployment获取相关Pod:', deploymentRef.name)
      }
      
      // 优先使用app标签，这样可以获取所有相关的Pod（不仅仅是相同版本）
      const labels = podDetail.value.labels || podDetail.value.metadata?.labels || {}
      console.log('🔍 [Pod详情页面] 工作负载Pod的可用标签:', labels)

      const appLabel = labels['app']
      const nameLabel = labels['app.kubernetes.io/name']
      const instanceLabel = labels['app.kubernetes.io/instance']

      if (appLabel) {
        labelSelector = `app=${appLabel}`
        console.log('🔍 [Pod详情页面] 使用app标签选择器:', labelSelector)
      } else if (nameLabel) {
        labelSelector = `app.kubernetes.io/name=${nameLabel}`
        console.log('🔍 [Pod详情页面] 使用name标签选择器:', labelSelector)
      } else if (instanceLabel) {
        labelSelector = `app.kubernetes.io/instance=${instanceLabel}`
        console.log('🔍 [Pod详情页面] 使用instance标签选择器:', labelSelector)
      } else {
        // 最后选择：通过pod-template-hash（只会找到相同版本的Pod）
        const podTemplateHash = labels['pod-template-hash']
        if (podTemplateHash) {
          labelSelector = `pod-template-hash=${podTemplateHash}`
          console.log('🔍 [Pod详情页面] 使用pod-template-hash标签选择器:', labelSelector)
        }
      }
    }
    
    // 统一的标签选择器处理
    
    if (!labelSelector) {
      console.log('⚠️ 无法构建标签选择器，使用当前Pod')
      // 构造当前Pod数据结构
      allRelatedPods.value = [{
        name: routeParams.podName,
        status: podDetail.value.status || podDetail.value.phase || 'Unknown',
        restartCount: podDetail.value.restartCount || 0,
        nodeName: podDetail.value.spec?.nodeName || 'Unknown',
        podIP: podDetail.value.podIP || podDetail.value.status?.podIP || (podDetail.value.status === 'Pending' ? '等待分配' : podDetail.value.status === 'Terminating' ? '已释放' : 'Unknown'),
        hostIP: podDetail.value.hostIP || podDetail.value.status?.hostIP || 'Unknown',
        age: formatAge(podDetail.value.metadata?.creationTimestamp),
        runningTime: podDetail.value.runningTime || '',
        containers: containers.value || podDetail.value.spec?.containers || [],
        resources: {
          requests: { cpu: '', memory: '' },
          limits: { cpu: '', memory: '' }
        },
        labels: podDetail.value.metadata?.labels || {},
        conditions: podDetail.value.status?.conditions || [],
        rawData: podDetail.value
      }]
      return
    }
    
    // 尝试使用新的工作负载API获取Pod列表
    let response
    let useNewApi = false

    // 从Pod名称智能推断Deployment名称
    let inferredDeploymentName = null
    const podNameParts = routeParams.podName.split('-')
    if (podNameParts.length >= 3) {
      // Pod命名规律: deployment-name-replicaset-hash-pod-hash
      // 例如: zf-nginx-test-5d9f8cbcdb-8b94f
      // Deployment名称通常是去掉最后两个部分
      inferredDeploymentName = podNameParts.slice(0, -2).join('-')
      console.log('🔍 [Pod详情页面] 从Pod名称推断Deployment:', inferredDeploymentName)
    }

    // 检查是否可以推断出工作负载信息
    if (deploymentRef) {
      console.log('🔍 [Pod详情页面] 使用新API - Deployment:', deploymentRef.name)
      try {
        response = await k8sApi.getWorkloadPods(
          routeParams.clusterId,
          routeParams.namespace,
          'deployment',
          deploymentRef.name
        )
        useNewApi = true
      } catch (error) {
        console.log('⚠️ 新API调用失败，回退到标签选择器:', error.message)
      }
    } else if (replicaSetRef) {
      // 尝试从ReplicaSet名称推断Deployment名称
      const replicaSetName = replicaSetRef.name
      // ReplicaSet命名规则通常是: deployment-name-pod-template-hash
      const parts = replicaSetName.split('-')
      if (parts.length >= 2) {
        const deploymentName = parts.slice(0, -1).join('-')
        console.log('🔍 [Pod详情页面] 从ReplicaSet推断Deployment名称:', deploymentName)
        try {
          response = await k8sApi.getWorkloadPods(
            routeParams.clusterId,
            routeParams.namespace,
            'deployment',
            deploymentName
          )
          useNewApi = true
        } catch (error) {
          console.log('⚠️ 推断的Deployment名称无效，回退到标签选择器:', error.message)
        }
      }
    }

    // 如果新API没有成功，回退到旧的标签选择器方式
    if (!useNewApi) {
      console.log('🔍 [Pod详情页面] 使用旧API - 标签选择器:', labelSelector)
      response = await k8sApi.getPodList(routeParams.clusterId, routeParams.namespace, {
        labelSelector: labelSelector
      })
    }
    
    console.log('📡 Pod列表API响应:', response)
    const responseData = response.data || response
    console.log('📊 Pod列表响应数据:', responseData)

    if (responseData.code === 200 || responseData.success) {
      // 处理不同API的响应格式
      let pods = []
      if (useNewApi) {
        // 新API直接返回Pod数组
        pods = responseData.data || []
        console.log('📋 [新API] 获取到Pod列表:', pods.length, '个')
      } else {
        // 旧API返回 data.pods 或 data
        pods = responseData.data?.pods || responseData.data || []
        console.log('📋 [旧API] 获取到Pod列表:', pods.length, '个')
      }
      console.log('📋 原始Pod数据:', pods)
      console.log('📊 Pod数量:', pods.length)
      allRelatedPods.value = pods.map(pod => ({
        name: pod.name,
        status: pod.status || pod.phase || 'Unknown',
        restartCount: useNewApi ? (pod.restarts || pod.restartCount || 0) : (pod.restartCount || 0),
        nodeName: pod.nodeName || 'Unknown',
        podIP: pod.podIP || pod.status?.podIP || (pod.status === 'Pending' ? '等待分配' : pod.status === 'Terminating' ? '已释放' : 'Unknown'),
        hostIP: pod.hostIP || pod.status?.hostIP || 'Unknown',
        age: useNewApi ? (pod.age || formatAge(pod.createdAt)) : formatAge(pod.createdAt),
        runningTime: pod.runningTime || '',
        containers: pod.containers || [],
        resources: pod.resources || {
          requests: { cpu: '', memory: '' },
          limits: { cpu: '', memory: '' }
        },
        resourceLimits: pod.resourceLimits || pod.resources?.limits || {},
        currentUsage: pod.currentUsage || {},
        labels: pod.labels || {},
        conditions: pod.conditions || [],
        rawData: pod
      }))
      
      console.log(`✅ 成功获取 ${allRelatedPods.value.length} 个相关Pod`)
      
      // 自动选择第一个Pod用于日志显示
      if (allRelatedPods.value.length > 0 && !selectedContainerForLogs.value) {
        selectedContainerForLogs.value = allRelatedPods.value[0].name
        console.log('🔄 自动选择第一个Pod用于日志:', selectedContainerForLogs.value)
        // 自动加载第一个Pod的日志
        setTimeout(() => {
          handleRefreshLogs()
        }, 100)
      }
    } else {
      console.log('❌ 获取Pod列表失败:', responseData.message)
      // 构造当前Pod数据结构
      allRelatedPods.value = [{
        name: routeParams.podName,
        status: podDetail.value.status || podDetail.value.phase || 'Unknown',
        restartCount: podDetail.value.restartCount || 0,
        nodeName: podDetail.value.spec?.nodeName || 'Unknown',
        podIP: podDetail.value.podIP || podDetail.value.status?.podIP || (podDetail.value.status === 'Pending' ? '等待分配' : podDetail.value.status === 'Terminating' ? '已释放' : 'Unknown'),
        hostIP: podDetail.value.hostIP || podDetail.value.status?.hostIP || 'Unknown',
        age: formatAge(podDetail.value.metadata?.creationTimestamp),
        runningTime: podDetail.value.runningTime || '',
        containers: containers.value || podDetail.value.spec?.containers || [],
        resources: { requests: { cpu: '', memory: '' }, limits: { cpu: '', memory: '' } },
        labels: podDetail.value.metadata?.labels || {},
        conditions: podDetail.value.status?.conditions || [],
        rawData: podDetail.value
      }]
    }
  } catch (error) {
    console.error('❌ 获取相关Pod失败:', error)
    // 构造当前Pod数据结构
    allRelatedPods.value = [{
      name: routeParams.podName,
      status: podDetail.value.status || podDetail.value.phase || 'Unknown',
      restartCount: podDetail.value.restartCount || 0,
      nodeName: podDetail.value.spec?.nodeName || 'Unknown',
      podIP: podDetail.value.podIP || podDetail.value.status?.podIP || (podDetail.value.status === 'Pending' ? '等待分配' : podDetail.value.status === 'Terminating' ? '已释放' : 'Unknown'),
      hostIP: podDetail.value.hostIP || podDetail.value.status?.hostIP || 'Unknown',
      age: formatAge(podDetail.value.metadata?.creationTimestamp),
      runningTime: podDetail.value.runningTime || '',
      containers: podDetail.value.spec?.containers || [],
      resources: { requests: { cpu: '', memory: '' }, limits: { cpu: '', memory: '' } },
      labels: podDetail.value.metadata?.labels || {},
      conditions: podDetail.value.status?.conditions || [],
      rawData: podDetail.value
    }]
  }
  
  // 自动选择第一个Pod用于日志显示
  if (allRelatedPods.value.length > 0 && !selectedContainerForLogs.value) {
    selectedContainerForLogs.value = allRelatedPods.value[0].name
    console.log('🔄 自动选择第一个Pod用于日志:', selectedContainerForLogs.value)
    // 自动加载第一个Pod的日志
    setTimeout(() => {
      handleRefreshLogs()
    }, 100)
  }
}

// 格式化时间
const formatAge = (createdTimestamp) => {
  if (!createdTimestamp) return 'Unknown'
  
  const now = new Date()
  const created = new Date(createdTimestamp)
  const diff = Math.floor((now - created) / 1000)
  
  if (diff < 60) return `${diff}s`
  if (diff < 3600) return `${Math.floor(diff / 60)}m`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h`
  return `${Math.floor(diff / 86400)}d`
}

// 获取相关的所有事件（Pod、ReplicaSet、Deployment）
const getEvents = async () => {
  try {
    console.log('开始获取事件数据...')
    console.log('请求参数:', {
      clusterId: routeParams.clusterId,
      namespace: routeParams.namespace,
      podName: routeParams.podName
    })
    
    // 获取最近1小时的时间戳
    const oneHourAgo = new Date(Date.now() - 60 * 60 * 1000)
    console.log('过滤时间: 最近1小时内的事件 (从', oneHourAgo.toISOString(), '开始)')
    
    // 根据后端API文档，构建事件查询参数
    console.log('构建事件查询参数...')
    
    // 从Pod名称中推断相关资源名称
    const podName = routeParams.podName
    let deploymentName = ''
    let replicaSetName = ''
    
    // 解析Pod名称格式：nginx-deployment-79df99db77-85qr5
    if (podName.includes('-')) {
      const parts = podName.split('-')
      if (parts.length >= 3) {
        // 去掉最后两个部分（ReplicaSet hash 和 Pod hash）
        deploymentName = parts.slice(0, -2).join('-')
        // 去掉最后一个部分（Pod hash）
        replicaSetName = parts.slice(0, -1).join('-')
      }
    }
    
    console.log('推断的资源名称:', {
      podName,
      replicaSetName,
      deploymentName
    })
    
    // 构建查询参数，获取Pod相关的所有事件
    const eventParams = {
      kind: 'Pod', // 首先获取Pod类型的事件
      name: podName, // 过滤特定Pod名称
      limit: 100 // 限制返回数量
    }
    
    console.log('事件API查询参数:', eventParams)
    const response = await k8sApi.getNamespaceEvents(routeParams.clusterId, routeParams.namespace, eventParams)
    const responseData = response.data || response
    
    console.log('事件API完整响应结构:', {
      success: !!responseData,
      code: responseData.code,
      dataType: typeof responseData.data,
      hasEvents: !!responseData.data?.events,
      eventsLength: responseData.data?.events?.length || 0,
      total: responseData.data?.total,
      namespace: responseData.data?.namespace
    })
    
    if (responseData.code === 200) {
      // 根据新的API格式，事件数据在 data.events 中
      let podEvents = responseData.data?.events || []
      
      console.log('Pod事件数据:', {
        count: podEvents.length,
        total: responseData.data?.total,
        namespace: responseData.data?.namespace
      })
      
      // 如果没有Pod事件，尝试获取相关的Deployment和ReplicaSet事件
      if (podEvents.length === 0 && (deploymentName || replicaSetName)) {
        console.log('未找到Pod事件，尝试获取相关资源事件...')
        
        // 获取Deployment事件
        if (deploymentName) {
          try {
            const deploymentParams = { kind: 'Deployment', name: deploymentName, limit: 50 }
            const deploymentResponse = await k8sApi.getNamespaceEvents(routeParams.clusterId, routeParams.namespace, deploymentParams)
            if (deploymentResponse.data?.code === 200) {
              const deploymentEvents = deploymentResponse.data?.data?.events || []
              podEvents = podEvents.concat(deploymentEvents)
              console.log(`添加了 ${deploymentEvents.length} 个Deployment事件`)
            }
          } catch (error) {
            console.log('获取Deployment事件失败:', error.message)
          }
        }
        
        // 获取ReplicaSet事件
        if (replicaSetName) {
          try {
            const replicaSetParams = { kind: 'ReplicaSet', name: replicaSetName, limit: 50 }
            const replicaSetResponse = await k8sApi.getNamespaceEvents(routeParams.clusterId, routeParams.namespace, replicaSetParams)
            if (replicaSetResponse.data?.code === 200) {
              const replicaSetEvents = replicaSetResponse.data?.data?.events || []
              podEvents = podEvents.concat(replicaSetEvents)
              console.log(`添加了 ${replicaSetEvents.length} 个ReplicaSet事件`)
            }
          } catch (error) {
            console.log('获取ReplicaSet事件失败:', error.message)
          }
        }
      }
      
      // 检查事件数据结构
      console.log('处理后的事件数据结构检查:')
      if (podEvents.length > 0) {
        const sampleEvent = podEvents[0]
        console.log('第一个事件的字段:', Object.keys(sampleEvent))
        console.log('事件示例:', {
          type: sampleEvent.type,
          reason: sampleEvent.reason,
          message: sampleEvent.message,
          source: sampleEvent.source,
          count: sampleEvent.count,
          firstTime: sampleEvent.firstTime,
          lastTime: sampleEvent.lastTime
        })
      } else {
        console.log('没有获取到任何相关事件数据')
      }
      
      // 从Pod名称中推断相关资源名称
      const podNameParts = routeParams.podName.split('-')
      console.log('Pod名称分析:', {
        fullName: routeParams.podName,
        parts: podNameParts,
        partsCount: podNameParts.length
      })
      
      // 生成多种可能的名称匹配模式
      const matchPatterns = []
      
      // 精确匹配Pod名称
      matchPatterns.push({
        name: routeParams.podName,
        kind: 'Pod',
        pattern: 'exact_pod'
      })
      
      // 如果Pod名称有足够的段数，推断ReplicaSet名称 (去掉最后一段随机字符)
      if (podNameParts.length >= 3) {
        const replicaSetName = podNameParts.slice(0, -1).join('-')
        matchPatterns.push({
          name: replicaSetName,
          kind: 'ReplicaSet', 
          pattern: 'replicaset_inferred'
        })
        
        // 推断Deployment名称 (去掉最后两段：hash和随机字符)
        if (podNameParts.length >= 4) {
          const deploymentName = podNameParts.slice(0, -2).join('-')
          matchPatterns.push({
            name: deploymentName,
            kind: 'Deployment',
            pattern: 'deployment_inferred'
          })
        }
      }
      
      console.log('匹配模式:', matchPatterns)
      
      // 过滤事件（这部分代码已经不需要，因为新API已经在后端过滤）
      const filteredEvents = podEvents.filter(event => {
        // 检查基本数据结构
        if (!event.involvedObject || !event.involvedObject.name || !event.involvedObject.kind) {
          return false
        }
        
        const eventObject = event.involvedObject
        const eventTime = new Date(event.lastTimestamp || event.lastTime || event.firstTimestamp || 0)
        
        // 时间过滤：只显示最近1小时内的事件
        if (eventTime < oneHourAgo) {
          return false
        }
        
        // 命名空间过滤
        if (eventObject.namespace !== routeParams.namespace) {
          return false  
        }
        
        // 资源名称匹配
        const isMatched = matchPatterns.some(pattern => {
          // 精确匹配
          if (pattern.pattern === 'exact_pod') {
            return eventObject.name === pattern.name && eventObject.kind === 'Pod'
          }
          
          // ReplicaSet匹配
          if (pattern.pattern === 'replicaset_inferred') {
            return eventObject.name === pattern.name && eventObject.kind === 'ReplicaSet'
          }
          
          // Deployment匹配
          if (pattern.pattern === 'deployment_inferred') {
            return eventObject.name === pattern.name && eventObject.kind === 'Deployment'
          }
          
          // 前缀匹配 - 相关的资源
          if (eventObject.name.startsWith(pattern.name)) {
            return ['Pod', 'ReplicaSet', 'Deployment', 'Service'].includes(eventObject.kind)
          }
          
          return false
        })
        
        if (isMatched) {
          console.log('✅ 匹配事件:', {
            name: eventObject.name,
            kind: eventObject.kind,
            namespace: eventObject.namespace,
            type: event.type,
            reason: event.reason,
            time: eventTime.toLocaleString('zh-CN')
          })
        }
        
        return isMatched
      })
      
      console.log(`过滤结果: ${filteredEvents.length}/${podEvents.length} 个事件匹配`)
      
      // 按时间排序（最新的在前）
      filteredEvents.sort((a, b) => {
        const timeA = new Date(a.lastTimestamp || a.lastTime || a.firstTimestamp || 0)
        const timeB = new Date(b.lastTimestamp || b.lastTime || b.firstTimestamp || 0)
        return timeB - timeA
      })
      
      // 直接使用新API返回的已过滤事件
      events.value = podEvents
      
      // 结果处理
      if (events.value.length === 0) {
        console.log('❌ 未找到相关事件数据')
        // 移除警告提示，静默处理无事件的情况
      } else {
        console.log(`✅ 成功加载 ${events.value.length} 个相关事件`)
        // 静默加载，不显示成功提示
      }
    } else {
      console.warn('❌ 获取集群事件API失败:', responseData)
      ElMessage.error(responseData.message || '获取事件失败')
    }
  } catch (error) {
    console.error('❌ 获取事件异常:', error)
    
    // 404错误表示没有事件数据，这是正常情况
    if (error.response && error.response.status === 404) {
      console.log('📋 集群暂无事件数据，这是正常情况')
      events.value = []
      return
    }
    
    // 其他错误才显示错误消息
    ElMessage.error('获取事件失败: ' + (error.message || '网络连接异常'))
  }
}

// 直接显示Pod详情中的事件数据
const showPodDetailEvents = () => {
  console.log('尝试直接显示Pod详情中的事件数据')
  console.log('podDetail.value:', podDetail.value)
  
  // 检查Pod详情中是否有events字段
  if (podDetail.value.events && Array.isArray(podDetail.value.events)) {
    events.value = podDetail.value.events
    console.log('直接使用Pod详情中的事件数据:', events.value.length, '个事件')
    ElMessage.success(`已加载 ${events.value.length} 个事件`)
  } else {
    // 尝试其他可能的事件数据位置
    const possibleEventFields = ['events', 'eventList', 'podEvents', 'conditions']
    let foundEvents = false
    
    for (const field of possibleEventFields) {
      if (podDetail.value[field] && Array.isArray(podDetail.value[field])) {
        events.value = podDetail.value[field]
        console.log(`在 ${field} 字段找到事件数据:`, events.value.length, '个')
        ElMessage.success(`从 ${field} 字段加载了 ${events.value.length} 个事件`)
        foundEvents = true
        break
      }
    }
    
    if (!foundEvents) {
      console.log('Pod详情中未找到任何事件数据')
      console.log('Pod详情的所有字段:', Object.keys(podDetail.value))
      ElMessage.warning('Pod详情中未找到事件数据')
    }
  }
}

const normalizeHistoryImages = (...sources) => {
  const containers = []

  const addImage = item => {
    if (!item) return

    if (typeof item === 'string') {
      const image = item.trim()
      if (image) containers.push({ name: '', image })
      return
    }

    if (Array.isArray(item)) {
      item.forEach(addImage)
      return
    }

    if (typeof item !== 'object') return

    const image = String(item.image || item.imageName || item.containerImage || item.currentImage || '').trim()
    if (image) {
      containers.push({
        name: String(item.name || item.containerName || item.container?.name || '').trim(),
        image
      })
      return
    }

    addImage(item.containers)
    addImage(item.initContainers)
    addImage(item.ephemeralContainers)
    addImage(item.containerStatuses)
    addImage(item.spec?.containers)
    addImage(item.spec?.initContainers)
    addImage(item.spec?.ephemeralContainers)
    addImage(item.template?.spec?.containers)
    addImage(item.template?.spec?.initContainers)
    addImage(item.template?.spec?.ephemeralContainers)
    addImage(item.podTemplate?.containers)
    addImage(item.podTemplate?.initContainers)
  }

  sources.forEach(source => addImage(source))

  const seen = new Set()
  const result = containers.filter(container => {
    if (seen.has(container.image)) return false
    seen.add(container.image)
    return true
  })

  return {
    images: result.map(container => container.image),
    containers: result
  }
}

const getHistoryVersionImageInfo = version => {
  const containerSources = [
    version?.containers,
    version?.initContainers,
    version?.ephemeralContainers,
    version?.podTemplate?.containers,
    version?.podTemplate?.initContainers,
    version?.template?.containers,
    version?.template?.initContainers,
    version?.spec?.template?.spec?.containers,
    version?.spec?.template?.spec?.initContainers,
    version?.spec?.containers
  ]

  return normalizeHistoryImages(...containerSources, version?.images, version?.image)
}

const getCurrentPodHistoryImageInfo = () => normalizeHistoryImages(
  containers.value,
  podDetail.value.containers,
  podDetail.value.initContainers,
  podDetail.value.ephemeralContainers,
  podDetail.value.spec?.containers,
  podDetail.value.spec?.initContainers,
  podDetail.value.spec?.ephemeralContainers
)

// 获取历史版本（通过workload获取）
const getHistoryVersions = async () => {
  try {
    console.log('📋 开始获取历史版本数据...')
    console.log('Pod详情数据:', podDetail.value)
    
    // 尝试从Pod的ownerReferences获取工作负载信息
    const ownerRefs = podDetail.value.metadata?.ownerReferences || 
                     podDetail.value.spec?.ownerReferences || 
                     podDetail.value.ownerReferences || []
    
    console.log('Pod ownerReferences:', ownerRefs)
    
    // 查找ReplicaSet或Deployment
    const replicaSet = ownerRefs.find(ref => ref.kind === 'ReplicaSet')
    const deployment = ownerRefs.find(ref => ref.kind === 'Deployment')
    let deploymentName = deployment?.name || routeParams.workloadName

    if (!deploymentName && replicaSet?.name) {
      const parts = replicaSet.name.split('-')
      if (parts.length >= 2) {
        deploymentName = parts.slice(0, -1).join('-')
      }
    }

    if (!deploymentName && routeParams.podName) {
      const podNameParts = routeParams.podName.split('-')
      if (podNameParts.length >= 3) {
        deploymentName = podNameParts.slice(0, -2).join('-')
      }
    }
    
    console.log('识别的控制器:', {
      hasReplicaSet: !!replicaSet,
      hasDeployment: !!deployment,
      replicaSetName: replicaSet?.name,
      deploymentName
    })
    
    if (deploymentName) {
      // 如果是Deployment管理的Pod，获取Deployment的历史版本
      console.log('🔍 [历史版本] 获取Deployment历史版本:', deploymentName)
      console.log('🔍 [历史版本] 调用getDeploymentHistory API')
      try {
        const response = await k8sApi.getDeploymentHistory(routeParams.clusterId, routeParams.namespace, deploymentName)
        const responseData = response.data || response
        const revisions = responseData.data?.revisions || []
        
        console.log('Deployment详情API响应:', {
          success: responseData.code === 200,
          hasRevisionHistory: revisions.length > 0,
          historyLength: revisions.length
        })
        
        if (responseData.code === 200 && revisions.length > 0) {
          historyVersions.value = revisions.map((version, index) => {
            const imageInfo = getHistoryVersionImageInfo(version)

            return {
              revision: version.revision || (index + 1),
              creationTime: version.creationTime || version.createdAt,
              replicas: version.replicasSummary?.desired ?? version.replicas ?? 1,
              description: `Deployment: ${deploymentName} - Rev ${version.revision || (index + 1)}`,
              changeDescription: version.changeReason || version.changeDescription || version.description,
              image: imageInfo.images.join(', '),
              images: imageInfo.images,
              containers: imageInfo.containers,
              status: version.status === 'current' ? 'Current' : 'Historical'
            }
          })
          console.log('✅ 获取到Deployment历史版本:', historyVersions.value.length, '个版本')
        } else {
          throw new Error('Deployment API返回失败或无历史版本数据')
        }
      } catch (deploymentError) {
        console.log('⚠️  获取Deployment详情失败，生成基础版本信息')
        const imageInfo = getCurrentPodHistoryImageInfo()
        historyVersions.value = [{
          revision: 1,
          creationTime: podDetail.value.createdAt || podDetail.value.creationTimestamp,
          replicas: 1,
          description: `Deployment: ${deploymentName} (详情获取失败)`,
          image: imageInfo.images.join(', '),
          images: imageInfo.images,
          containers: imageInfo.containers,
          status: 'Current'
        }]
      }
    } else if (replicaSet) {
      // 如果是ReplicaSet管理的Pod
      console.log('📝 生成ReplicaSet版本信息:', replicaSet.name)
      
      // 尝试获取ReplicaSet的详细信息
      console.log('🔍 [历史版本] 调用getWorkloadDetail API - ReplicaSet')
      try {
        const response = await k8sApi.getWorkloadDetail(routeParams.clusterId, routeParams.namespace, 'replicaset', replicaSet.name)
        const responseData = response.data || response
        
        if (responseData.code === 200 && responseData.data) {
          const imageInfo = normalizeHistoryImages(
            responseData.data.containers,
            responseData.data.podTemplate?.containers,
            responseData.data.template?.containers,
            responseData.data.spec?.template?.spec?.containers,
            responseData.data.spec?.containers,
            responseData.data.images,
            responseData.data.image
          )

          historyVersions.value = [{
            revision: 1,
            creationTime: responseData.data.createdAt || responseData.data.creationTimestamp,
            replicas: responseData.data.replicas || 1,
            description: `ReplicaSet: ${replicaSet.name}`,
            image: imageInfo.images.join(', '),
            images: imageInfo.images,
            containers: imageInfo.containers,
            status: 'Active'
          }]
        } else {
          throw new Error('ReplicaSet API返回失败')
        }
      } catch (replicaSetError) {
        console.log('⚠️  获取ReplicaSet详情失败，使用基础信息')
        const imageInfo = getCurrentPodHistoryImageInfo()
        historyVersions.value = [{
          revision: 1,
          creationTime: podDetail.value.createdAt || podDetail.value.creationTimestamp,
          replicas: 1,
          description: `ReplicaSet: ${replicaSet.name} (详情获取失败)`,
          image: imageInfo.images.join(', '),
          images: imageInfo.images,
          containers: imageInfo.containers,
          status: 'Current'
        }]
      }
    } else {
      // 如果没有控制器，显示Pod自身信息
      console.log('🔹 独立Pod，显示Pod自身信息')
      const imageInfo = getCurrentPodHistoryImageInfo()
      historyVersions.value = [{
        revision: 1,
        creationTime: podDetail.value.createdAt || podDetail.value.creationTimestamp,
        replicas: 1,
        description: `独立Pod: ${routeParams.podName}`,
        image: imageInfo.images.join(', '),
        images: imageInfo.images,
        containers: imageInfo.containers,
        status: podDetail.value.status || podDetail.value.phase || 'Unknown'
      }]
    }
    
    console.log('✅ 历史版本数据处理完成:', historyVersions.value)
    
  } catch (error) {
    console.error('❌ 获取历史版本异常:', error)
    // 如果获取失败，至少显示当前Pod信息
    const imageInfo = getCurrentPodHistoryImageInfo()
    historyVersions.value = [{
      revision: 1,
      creationTime: podDetail.value.createdAt || podDetail.value.creationTimestamp || new Date().toISOString(),
      replicas: 1,
      description: `当前Pod: ${routeParams.podName} (历史版本获取失败)`,
      image: imageInfo.images.join(', '),
      images: imageInfo.images,
      containers: imageInfo.containers,
      status: 'Current',
      error: error.message
    }]
    
    ElMessage.warning('历史版本获取失败，显示当前Pod信息')
  }
}

// 获取容器日志 - 切换到日志标签页
const handleViewLogs = async (pod) => {
  console.log('🔄 点击日志按钮，切换到日志标签页')
  console.log('🔍 传入的pod参数:', pod)
  console.log('🔍 当前allRelatedPods数量:', allRelatedPods.value.length)
  
  openingLogsFromAction = true

  try {
    // 确保有可用的Pod数据
    if (allRelatedPods.value.length === 0) {
      console.log('⚠️  allRelatedPods为空，尝试重新获取相关Pod')
      await getRelatedPods()
    }

    // 如果传入了具体的pod参数，优先使用它
    if (pod && pod.name) {
      selectedContainerForLogs.value = pod.name
      selectDefaultLogContainer(pod.name)
      console.log('🔄 选择传入的Pod用于日志:', selectedContainerForLogs.value)
    }

    followLogs.value = true
    activeTab.value = 'logs'
    await scrollLogsIntoView()
    await handleRefreshLogs()
  } finally {
    openingLogsFromAction = false
  }
}

// 处理容器选择变化
const handleContainerChange = (podName) => {
  console.log('🔄 选择Pod:', podName)
  selectedContainerForLogs.value = podName
  selectDefaultLogContainer(podName)
  if (podName) {
    handleRefreshLogs()
  }
}

const handleLogContainerChange = (containerName) => {
  console.log('🔄 选择容器:', containerName)
  selectedLogContainer.value = containerName
  if (containerName) {
    handleRefreshLogs()
  }
}

// 断开 WebSocket 连接
const disconnectWebSocket = () => {
  // 先刷新剩余的缓冲区日志
  if (logsBuffer.value.length > 0) {
    console.log('🔄 断开前刷新剩余日志:', logsBuffer.value.length)
    const bufferedLogs = logsBuffer.value.join('')
    currentLogs.value = currentLogs.value + bufferedLogs
    logsBuffer.value = []
  }

  // 清除刷新定时器
  if (flushTimer) {
    clearInterval(flushTimer)
    flushTimer = null
  }

  // 断开WebSocket连接
  if (wsConnection.value) {
    console.log('🔌 断开 WebSocket 连接')
    try {
      wsConnection.value.close()
    } catch (error) {
      console.warn('⚠️ 关闭WebSocket时出错:', error)
    }
    wsConnection.value = null
    wsStatus.value = 'disconnected'
  }

  // 重置更新key
  logsUpdateKey.value = 0
}

// 刷新日志缓冲区到显示区域
const flushLogsBuffer = () => {
  if (logsBuffer.value.length > 0) {
    const bufferedLogs = logsBuffer.value.join('')

    // 创建全新字符串触发响应式更新
    let newLogs = currentLogs.value + bufferedLogs

    // 日志轮转：如果启用且超过最大行数，保留最新的日志
    if (logRotationEnabled.value) {
      const lines = newLogs.split('\n')
      if (lines.length > maxLogLines.value) {
        const excessLines = lines.length - maxLogLines.value
        console.log(`🔄 日志轮转: 删除前 ${excessLines} 行，保留最新 ${maxLogLines.value} 行`)
        // 保留最新的maxLogLines行
        newLogs = lines.slice(-maxLogLines.value).join('\n')
      }
    }

    // 强制创建新字符串引用，确保Vue能检测到变化
    currentLogs.value = (' ' + newLogs).slice(1)

    // 清空缓冲区
    logsBuffer.value = []

    // 更新key触发重新渲染，直接滚动到底部避免闪烁
    nextTick(() => {
      logsUpdateKey.value++

      if (autoScroll.value) {
        forceLogsScrollToBottom()
      }
    })
  }
}

const forceLogsScrollToBottom = () => {
  nextTick(() => {
    requestAnimationFrame(() => {
      logsEditorRef.value?.scrollToBottom?.()

      requestAnimationFrame(() => {
        logsEditorRef.value?.scrollToBottom?.()
      })
    })
  })
}

// 建立 WebSocket 连接获取实时日志
const connectWebSocket = (podName, container) => {
  // 先断开现有连接
  disconnectWebSocket()

  console.log('🔄 开始建立 WebSocket 连接...')
  wsStatus.value = 'connecting'

  // 获取 token - 使用 storage 而不是 localStorage
  const token = storage.getItem('token') || ''
  if (!token) {
    ElMessage.error('未找到token，请先登录')
    wsStatus.value = 'error'
    return
  }

  // 构建 WebSocket URL，使用 containerName 参数（参考终端实现）
  const wsUrl = buildWebSocketUrl(`/api/v1/k8s/cluster/${routeParams.clusterId}/namespaces/${routeParams.namespace}/pods/${podName}/logs/stream?containerName=${container}&token=${encodeURIComponent(token)}`)

  console.log('🔗 WebSocket URL:', wsUrl.replace(token, '***'))
  console.log('🔑 使用token:', token ? '***' + token.slice(-4) : 'null')

  try {
    const ws = new WebSocket(wsUrl)

    // 添加连接超时处理（10秒）
    const connectionTimeout = setTimeout(() => {
      if (ws.readyState === WebSocket.CONNECTING) {
        console.warn('⏰ WebSocket连接超时，主动关闭')
        ws.close()
        wsStatus.value = 'error'
        ElMessage.error('连接超时，请检查网络或服务器状态')
      }
    }, 10000)

    ws.onopen = () => {
      clearTimeout(connectionTimeout)
      console.log('✅ WebSocket 连接成功')
      wsStatus.value = 'connected'
      wsConnection.value = ws
      lastLogRefreshTime.value = new Date()

      // 启动定时刷新缓冲区（每300毫秒刷新一次）
      flushTimer = setInterval(flushLogsBuffer, 300)

      // 连接成功后延迟滚动到底部，等待初始日志加载
      setTimeout(() => {
        if (autoScroll.value) {
          forceLogsScrollToBottom()
        }
      }, 500)
    }

    ws.onmessage = (event) => {
      // 实时接收日志并加入缓冲区
      const logLine = sanitizeLogText(event.data)

      if (logLine && logLine.trim()) {
        // 过滤掉后端的调试信息
        const trimmedLine = logLine.trim()
        if (
          trimmedLine.startsWith('===') ||
          trimmedLine.startsWith('集群ID:') ||
          trimmedLine.startsWith('命名空间:') ||
          trimmedLine.startsWith('Pod:') ||
          trimmedLine.startsWith('容器:') ||
          trimmedLine.startsWith('最后') ||
          trimmedLine.startsWith('调试:') ||
          trimmedLine === '================'
        ) {
          // 跳过这些调试行
          return
        }

        // 添加到缓冲区
        logsBuffer.value.push((currentLogs.value && !currentLogs.value.endsWith('\n') && logsBuffer.value.length === 0 ? '\n' : '') + logLine)

        // 如果缓冲区过大(超过50条),立即刷新
        if (logsBuffer.value.length >= 50) {
          flushLogsBuffer()
        }
      }
    }

    ws.onerror = (error) => {
      console.error('❌ WebSocket 错误:', error)
      wsStatus.value = 'error'
      ElMessage.error('WebSocket 连接错误')
    }

    ws.onclose = (event) => {
      console.log('🔌 WebSocket 连接已关闭', event.code, event.reason)
      wsStatus.value = 'disconnected'
      wsConnection.value = null
    }
  } catch (error) {
    console.error('❌ 创建 WebSocket 连接失败:', error)
    wsStatus.value = 'error'
    ElMessage.error('无法建立实时日志连接: ' + error.message)
  }
}

// 刷新日志
const handleRefreshLogs = async () => {
  if (!selectedContainerForLogs.value) {
    ElMessage.warning('请先选择Pod')
    return
  }

  console.log('🔄 开始刷新日志...')
  console.log('🔍 selectedContainerForLogs.value:', selectedContainerForLogs.value)
  console.log('🔍 allRelatedPods.value:', allRelatedPods.value.map(p => p.name))

  // 根据选中的Pod名称找到对应的Pod和容器
  const selectedPod = allRelatedPods.value.find(pod => pod.name === selectedContainerForLogs.value)
  if (!selectedPod) {
    console.error('❌ 未找到选中的Pod:', selectedContainerForLogs.value)
    console.error('❌ 可用的Pod列表:', allRelatedPods.value.map(p => p.name))
    ElMessage.error(`未找到选中的Pod: ${selectedContainerForLogs.value}`)
    return
  }

  console.log('✅ 找到选中的Pod:', selectedPod)

  const containerNames = getPodLogContainerNames(selectedPod)
  const targetContainer = containerNames.includes(selectedLogContainer.value)
    ? selectedLogContainer.value
    : containerNames[0]

  if (!targetContainer) {
    console.error('❌ Pod中未找到容器:', selectedPod)
    console.error('❌ containers字段:', selectedPod.containers)
    ElMessage.error(`Pod "${selectedContainerForLogs.value}" 中未找到容器`)
    return
  }

  selectedLogContainer.value = targetContainer
  console.log('✅ 选择日志容器:', targetContainer)

  // 如果勾选了实时跟踪，使用 WebSocket 连接
  if (followLogs.value) {
    console.log('🔄 启动实时日志跟踪 (WebSocket)')
    // 先断开旧连接并清理所有状态
    disconnectWebSocket()
    // 清空日志显示
    currentLogs.value = ''
    logsLoading.value = false

    // 短暂延迟确保状态清理完成
    await new Promise(resolve => setTimeout(resolve, 100))

    connectWebSocket(selectedContainerForLogs.value, targetContainer)
    return
  }

  // 否则使用传统的 HTTP 方式获取日志
  try {
    // 先断开可能存在的 WebSocket 连接
    disconnectWebSocket()

    // 清空当前日志显示
    currentLogs.value = ''

    logsLoading.value = true
    console.log('🔍 获取Pod日志:', {
      podName: selectedContainerForLogs.value,
      container: targetContainer,
      previous: showPreviousLogs.value
    })

    // 构建日志查询参数
    const logParams = {
      container: targetContainer
    }

    // 如果要显示上个容器的日志
    if (showPreviousLogs.value) {
      logParams.previous = true
    }

    const response = await k8sApi.getPodLogs(routeParams.clusterId, routeParams.namespace, selectedContainerForLogs.value, logParams)
    const responseData = response.data || response

    console.log('日志API响应:', {
      success: responseData.code === 200,
      dataLength: responseData.data?.length || 0,
      dataType: typeof responseData.data
    })

    if (responseData.code === 200) {
      // 处理嵌套的日志数据结构：data.logs 或直接 data
      const logData = sanitizeLogText(responseData.data?.logs || responseData.data || '')
      console.log('📋 日志数据结构解析:', {
        hasDataLogs: !!responseData.data?.logs,
        hasDirectData: !!responseData.data && typeof responseData.data === 'string',
        logDataType: typeof logData,
        logDataLength: logData?.length || 0
      })

      currentLogs.value = logData
      lastLogRefreshTime.value = new Date()

      const logLineCount = currentLogs.value.split('\n').length
      console.log('✅ 日志获取成功:', logLineCount, '行')
      console.log('📄 日志内容预览:', currentLogs.value.substring(0, 200) + '...')

      if (currentLogs.value.trim()) {
        // 静默加载日志，不显示成功提示
        console.log(`✅ 日志加载成功，共 ${logLineCount} 行`)

        // HTTP方式获取日志后，延迟滚动到底部
        setTimeout(() => {
          forceLogsScrollToBottom()
        }, 200)
      } else {
        ElMessage.info('该容器暂无日志输出')
        currentLogs.value = '# 该容器暂无日志输出\n# Container has no log output'
      }
    } else {
      console.error('❌ 日志API返回错误:', responseData.message)
      ElMessage.error(responseData.message || '获取容器日志失败')
      currentLogs.value = ''
    }
  } catch (error) {
    console.error('❌ 获取容器日志异常:', error)

    if (error.response?.status === 404) {
      ElMessage.error('容器不存在或日志不可用')
      currentLogs.value = '# 容器不存在或日志不可用\n# Container not found or logs not available'
    } else if (error.response?.status === 400) {
      ElMessage.error('日志参数错误，请检查容器状态')
      currentLogs.value = '# 日志参数错误\n# Invalid log parameters'
    } else {
      ElMessage.error('获取日志失败: ' + (error.message || '网络连接异常'))
      currentLogs.value = '# 日志获取失败\n# Failed to retrieve logs\n# Error: ' + (error.message || 'Network error')
    }
  } finally {
    logsLoading.value = false
  }
}

// 下载日志文件
const handleDownloadLogs = () => {
  if (!selectedContainerForLogs.value) {
    ElMessage.warning('请先选择Pod')
    return
  }

  if (!selectedLogContainer.value) {
    ElMessage.warning('请先选择容器')
    return
  }
  
  if (!currentLogs.value) {
    ElMessage.warning('请先获取日志')
    return
  }
  
  try {
    // 创建日志文件内容
    const logContent = `# Container Logs
# Pod: ${selectedContainerForLogs.value}
# Namespace: ${routeParams.namespace}
# Container: ${selectedLogContainer.value}
# Generated: ${new Date().toLocaleString('zh-CN')}
# Previous: ${showPreviousLogs.value ? 'Yes' : 'No'}

${currentLogs.value}`
    
    // 创建下载链接
    const blob = new Blob([logContent], { type: 'text/plain;charset=utf-8' })
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    
    // 生成文件名
    const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, -5)
    const filename = `${selectedContainerForLogs.value}_${selectedLogContainer.value}_${timestamp}.log`
    
    link.href = url
    link.download = filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)
    
    ElMessage.success(`日志文件已下载: ${filename}`)
    console.log('✅ 日志下载成功:', filename)
    
  } catch (error) {
    console.error('❌ 日志下载失败:', error)
    ElMessage.error('日志下载失败: ' + error.message)
  }
}

// 获取YAML内容
const handleViewYaml = async () => {
  try {
    loading.value = true
    console.log('🔍 开始获取YAML内容...')
    console.log('请求参数:', {
      clusterId: routeParams.clusterId,
      namespace: routeParams.namespace,
      podName: routeParams.podName
    })
    
    const response = await k8sApi.getPodYaml(routeParams.clusterId, routeParams.namespace, routeParams.podName)
    const responseData = response.data || response
    
    console.log('YAML API响应结构:', {
      hasResponse: !!responseData,
      code: responseData.code,
      hasData: !!responseData.data,
      dataType: typeof responseData.data,
      dataLength: responseData.data?.length || 0
    })
    
    if (responseData.code === 200) {
      yamlContent.value = getYamlContentFromApiResponse(
        responseData,
        `apiVersion: v1\nkind: Pod\nmetadata:\n  name: ${routeParams.podName}`
      )
      console.log('YAML内容检查:', {
        contentLength: yamlContent.value.length,
        firstLine: yamlContent.value.split('\n')[0],
        isString: typeof yamlContent.value === 'string',
        isEmpty: yamlContent.value.trim() === ''
      })
      
      if (yamlContent.value && yamlContent.value.trim()) {
        dialogStates.yamlVisible = true
        console.log('✅ YAML内容获取成功，打开对话框')
        ElMessage.success('YAML内容加载成功')
      } else {
        console.log('⚠️ YAML内容为空')
        ElMessage.warning('YAML内容为空，可能Pod资源不存在')
      }
    } else {
      console.log('❌ YAML API返回错误:', responseData)
      ElMessage.error(responseData.message || '获取YAML内容失败')
    }
  } catch (error) {
    console.error('❌ 获取YAML内容异常:', error)
    
    // 详细的错误处理
    if (error.response) {
      const status = error.response.status
      const statusText = error.response.statusText
      console.log('HTTP错误详情:', {
        status,
        statusText,
        data: error.response.data
      })
      
      switch (status) {
        case 404:
          ElMessage.error('Pod不存在或已被删除')
          break
        case 403:
          ElMessage.error('没有权限访问该Pod的YAML')
          break
        case 500:
          ElMessage.error('服务器内部错误，请稍后重试')
          break
        default:
          ElMessage.error(`获取YAML失败: ${status} ${statusText}`)
      }
    } else if (error.request) {
      console.log('网络请求错误:', error.request)
      ElMessage.error('网络连接失败，请检查网络连接')
    } else {
      console.log('其他错误:', error.message)
      ElMessage.error('获取YAML内容失败: ' + error.message)
    }
  } finally {
    loading.value = false
  }
}

// 查看Pod的YAML (只读)
const handleViewYamlForPod = async (pod) => {
  try {
    loading.value = true
    console.log('🔍 开始获取Pod YAML内容...', pod.name)

    // 使用pod参数中的信息或者从路由参数获取
    const clusterId = routeParams.clusterId
    const namespace = pod.namespace || routeParams.namespace
    const podName = pod.name

    console.log('请求参数:', { clusterId, namespace, podName })

    const response = await k8sApi.getPodYaml(clusterId, namespace, podName)
    const responseData = response.data || response

    if (responseData.code === 200) {
      yamlContent.value = getYamlContentFromApiResponse(
        responseData,
        `apiVersion: v1\nkind: Pod\nmetadata:\n  name: ${podName}\n  namespace: ${namespace}`
      )

      // 打开YAML查看对话框(只读)
      dialogStates.yamlVisible = true
      console.log('✅ YAML内容加载成功，打开查看对话框')
    } else {
      console.error('❌ 获取YAML内容失败:', responseData)
      ElMessage.error(responseData.message || '获取YAML内容失败')
    }
  } catch (error) {
    console.error('❌ 获取YAML内容异常:', error)
    ElMessage.error('获取YAML内容失败: ' + (error.message || '网络连接异常'))
  } finally {
    loading.value = false
  }
}

const handleOpenPodFileManager = async pod => {
  const containerNames = getPodLogContainerNames(pod)
  currentFileBrowserPod.value = {
    namespace: pod.namespace || routeParams.namespace,
    podName: pod.name,
    containerName: containerNames[0] || '',
    containerNames
  }
  await nextTick()
  podFileBrowserVisible.value = true
}

// 处理回滚操作
const handleRollback = (version) => {
  rollbackForm.targetVersion = version.revision
  rollbackForm.versionInfo = version
  dialogStates.rollbackVisible = true
}

// 确认回滚
const handleConfirmRollback = async () => {
  if (!rollbackForm.targetVersion) {
    ElMessage.error('请选择回滚版本')
    return
  }

  // 获取部署名称
  const ownerRefs = podDetail.value.metadata?.ownerReferences ||
                   podDetail.value.spec?.ownerReferences ||
                   podDetail.value.ownerReferences || []

  console.log('🔍 [回滚] 检查ownerReferences:', ownerRefs)

  // 首先查找直接的Deployment引用
  let deployment = ownerRefs.find(ref => ref.kind === 'Deployment')
  let deploymentName = null

  if (deployment) {
    deploymentName = deployment.name
    console.log('✅ [回滚] 找到直接的Deployment:', deploymentName)
  } else {
    // 查找ReplicaSet，然后推断Deployment名称
    const replicaSet = ownerRefs.find(ref => ref.kind === 'ReplicaSet')
    if (replicaSet) {
      // ReplicaSet命名规则: deployment-name-pod-template-hash
      const parts = replicaSet.name.split('-')
      if (parts.length >= 2) {
        deploymentName = parts.slice(0, -1).join('-')
        console.log('✅ [回滚] 从ReplicaSet推断Deployment:', deploymentName)
      }
    }
  }

  // 如果还没找到，尝试从Pod名称推断
  if (!deploymentName) {
    const podNameParts = routeParams.podName.split('-')
    if (podNameParts.length >= 3) {
      deploymentName = podNameParts.slice(0, -2).join('-')
      console.log('✅ [回滚] 从Pod名称推断Deployment:', deploymentName)
    }
  }

  if (!deploymentName) {
    ElMessage.error('无法找到关联的Deployment，无法执行回滚')
    return
  }

  try {
    rollbackLoading.value = true
    console.log(`🔄 开始回滚Deployment ${deploymentName} 到版本 ${rollbackForm.targetVersion}`)

    const rollbackData = {
      revision: rollbackForm.targetVersion
    }

    const response = await k8sApi.rollbackDeployment(
      routeParams.clusterId,
      routeParams.namespace,
      deploymentName,
      rollbackData
    )

    const responseData = response.data || response
    if (responseData.code === 200) {
      ElMessage.success(`成功回滚到版本 ${rollbackForm.targetVersion}`)
      dialogStates.rollbackVisible = false

      // 刷新数据
      await handleQuery(true)
    } else {
      throw new Error(responseData.message || '回滚失败')
    }
  } catch (error) {
    console.error('❌ 回滚失败:', error)
    ElMessage.error(error.message || '回滚操作失败')
  } finally {
    rollbackLoading.value = false
  }
}

// 显示监控弹框
const handleShowMonitoring = async (pod) => {
  selectedPodForMonitoring.value = pod
  dialogStates.monitoringVisible = true
  
  // 加载监控数据
  await loadMonitoringData(pod)
}

// 加载监控数据
const getAiResponsePayload = (response, fallbackMessage = '请求失败') => {
  const body = response?.data || response || {}
  if (body.code !== undefined && Number(body.code) !== 200) {
    throw new Error(body.message || body.msg || fallbackMessage)
  }
  return body.data !== undefined ? body.data : body
}

const getDefaultAiProviderId = async () => {
  try {
    const response = await aiopsApi.getEnabledLLMProviders({ page: 1, pageSize: 100 })
    const payload = getAiResponsePayload(response, '获取 Provider 失败')
    const providers = Array.isArray(payload)
      ? payload
      : payload?.list || payload?.items || payload?.records || payload?.data || []
    const provider = providers.find(item => {
      const value = item?.isDefault
        ?? item?.is_default
        ?? item?.defaultProvider
        ?? item?.default_provider
        ?? item?.defaultFlag
        ?? item?.default_flag
        ?? item?.default
      return value === true || value === 1 || value === '1' || String(value).toLowerCase() === 'true'
    }) || providers[0]
    return provider?.id ?? provider?.providerId ?? provider?.value
  } catch (error) {
    console.warn('load default ai provider failed:', error)
    return undefined
  }
}

const parseAiSseData = event => {
  if (!event?.data) return {}
  try {
    return JSON.parse(event.data)
  } catch (error) {
    return { message: event.data }
  }
}

const resetAiAnalysisTaskState = () => {
  aiAnalysisTaskId.value = ''
  aiAnalysisTaskStatus.value = ''
  aiAnalysisPodStatus.value = 'unknown'
  aiAnalysisCurrentStep.value = 'queued'
  aiAnalysisProgress.value = 0
  aiAnalysisMessage.value = ''
  aiAnalysisEvents.value = []
}

const closeAiAnalysisStream = () => {
  if (aiAnalysisEventSource) {
    aiAnalysisEventSource.close()
    aiAnalysisEventSource = null
  }
}

const stopAiAnalysisPolling = () => {
  if (aiAnalysisPollingTimer) {
    window.clearInterval(aiAnalysisPollingTimer)
    aiAnalysisPollingTimer = null
  }
}

const startAiAnalysisPolling = taskId => {
  stopAiAnalysisPolling()
  if (!taskId) return
  aiAnalysisPollingTimer = window.setInterval(async () => {
    if (!dialogStates.aiAnalysisVisible) return
    if (!aiAnalysisTaskId.value || aiAnalysisTaskId.value !== taskId) return
    if (aiAnalysisTaskStatus.value === 'succeeded' || aiAnalysisTaskStatus.value === 'failed') {
      stopAiAnalysisPolling()
      return
    }
    await fetchAiAnalysisTaskStatus(taskId, { silentEvents: true })
  }, 1500)
}

const normalizeAiTaskResult = payload => {
  if (!payload) return null
  if (payload.result) return payload.result
  if (payload.data?.result) return payload.data.result
  return payload
}

const appendAiAnalysisEvent = (eventData = {}) => {
  const step = eventData.step || eventData.currentStep || eventData.type || 'step'
  const progress = Number(eventData.progress)
  aiAnalysisCurrentStep.value = step
  if (Number.isFinite(progress)) {
    aiAnalysisProgress.value = Math.max(0, Math.min(100, Math.round(progress)))
  }
  if (eventData.status) aiAnalysisTaskStatus.value = eventData.status
  if (eventData.podStatus) aiAnalysisPodStatus.value = eventData.podStatus
  if (eventData.message) aiAnalysisMessage.value = eventData.message

  const normalizedEvent = {
    ...eventData,
    step,
    message: eventData.message || getAiAnalysisStepLabel(step),
    timestamp: eventData.timestamp || new Date().toLocaleString('zh-CN')
  }
  const sameEventExists = aiAnalysisEvents.value.some(item =>
    item.step === normalizedEvent.step
    && item.message === normalizedEvent.message
    && item.timestamp === normalizedEvent.timestamp
  )
  if (sameEventExists) return

  aiAnalysisEvents.value.unshift(normalizedEvent)
  aiAnalysisEvents.value = aiAnalysisEvents.value.slice(0, 30)
}

const getAiAnalysisStepLabel = step => {
  return aiAnalysisStepList.find(item => item.key === step)?.label || step || '-'
}

const shouldShowAiAnalysisResult = computed(() => {
  return isAiAnalysisFinished.value
})

const finishAiAnalysisFromTask = payload => {
  const result = normalizeAiTaskResult(payload)
  if (result) {
    aiAnalysisResult.value = result
  }
  const podStatus = normalizeAiPodAnalysisStatus(
    payload?.podStatus
    || payload?.pod_status
    || result?.podStatus
    || result?.pod_status
    || result?.status
  )
  if (podStatus) {
    aiAnalysisPodStatus.value = podStatus
  }
  aiAnalysisTaskStatus.value = normalizeAiTaskStatus(payload?.status) || 'succeeded'
  aiAnalysisCurrentStep.value = 'result'
  aiAnalysisProgress.value = 100
  aiAnalysisLoading.value = false
  stopAiAnalysisPolling()
  closeAiAnalysisStream()
}

const fetchAiAnalysisTaskStatus = async (taskId, options = {}) => {
  if (!taskId) return
  try {
    const response = await aiopsApi.getK8sPodAnalyzeTask(taskId)
    const payload = getAiResponsePayload(response, '查询 AI 分析任务失败')
    const taskEvent = {
      type: 'task_status',
      step: payload.currentStep || payload.current_step || payload.step || aiAnalysisCurrentStep.value,
      status: payload.status,
      progress: payload.progress,
      message: payload.message,
      podStatus: payload.podStatus || payload.pod_status
    }
    appendAiAnalysisEvent(taskEvent)
    if (!options.silentEvents && Array.isArray(payload.events) && payload.events.length) {
      aiAnalysisEvents.value = payload.events.slice(-30).reverse()
    }
    if (payload.status === 'succeeded' && payload.result) {
      finishAiAnalysisFromTask(payload)
    } else if (payload.status === 'failed') {
      aiAnalysisTaskStatus.value = 'failed'
      aiAnalysisLoading.value = false
      stopAiAnalysisPolling()
      closeAiAnalysisStream()
    }
  } catch (error) {
    console.warn('fetch ai analysis task status failed:', error)
  }
}

const subscribeAiAnalysisTaskStream = (taskId) => {
  closeAiAnalysisStream()
  const source = aiopsApi.createK8sPodAnalyzeTaskStream(taskId)
  aiAnalysisEventSource = source
  const trackedSource = source

  const handleEvent = (rawType, event) => {
    if (aiAnalysisEventSource !== trackedSource) return
    const data = parseAiSseData(event)
    const eventData = { type: data.type || rawType, ...data }
    appendAiAnalysisEvent(eventData)

    if ((eventData.type || rawType) === 'result' || eventData.step === 'result') {
      finishAiAnalysisFromTask(eventData)
    }

    if ((eventData.type || rawType) === 'error' || eventData.status === 'failed') {
      aiAnalysisTaskStatus.value = 'failed'
      aiAnalysisLoading.value = false
      closeAiAnalysisStream()
      ElMessage.error(eventData.message || 'AI 分析诊断失败')
    }
  }

  source.addEventListener('open', () => {
    if (aiAnalysisEventSource !== trackedSource) return
    appendAiAnalysisEvent({
      type: 'open',
      step: 'queued',
      status: 'running',
      progress: aiAnalysisProgress.value,
      message: '进度流已连接，等待后端推送...'
    })
  })
  source.addEventListener('started', event => handleEvent('started', event))
  source.addEventListener('step', event => handleEvent('step', event))
  source.addEventListener('result', event => handleEvent('result', event))
  source.addEventListener('error', event => {
    if (event?.data) handleEvent('error', event)
  })
  source.onmessage = event => handleEvent('message', event)
  source.onerror = () => {
    if (aiAnalysisEventSource !== trackedSource) return
    appendAiAnalysisEvent({
      type: 'error',
      step: aiAnalysisCurrentStep.value,
      status: aiAnalysisTaskStatus.value || 'running',
      progress: aiAnalysisProgress.value,
      message: 'SSE 连接中断，正在查询任务状态...'
    })
    fetchAiAnalysisTaskStatus(taskId, { silentEvents: true })
  }
}

const handleAiAnalyzePod = async (pod) => {
  const targetPod = pod || selectedPodForAiAnalysis.value || { name: routeParams.podName }
  if (!targetPod?.name) {
    ElMessage.warning('未找到 Pod 信息')
    return
  }

  closeAiAnalysisStream()
  selectedPodForAiAnalysis.value = targetPod
  aiAnalysisResult.value = null
  resetAiAnalysisTaskState()
  dialogStates.aiAnalysisVisible = true
  aiAnalysisLoading.value = true

  try {
    const providerId = await getDefaultAiProviderId()
    const firstContainer = targetPod.containers?.[0]?.name || containers.value?.[0]?.name || ''
    const payload = {
      providerId: providerId || undefined,
      clusterId: Number(routeParams.clusterId) || routeParams.clusterId,
      namespace: targetPod.namespace || routeParams.namespace,
      pod: targetPod.name,
      container: firstContainer || undefined,
      tailLines: 500,
      limit: 200,
      eventLimit: 50,
      contextLines: 5,
      includePrevious: true,
      includeEvents: true,
      includeMetrics: true,
      prompt: aiAnalysisPrompt.value
    }

    const response = await aiopsApi.createK8sPodAnalyzeTask(payload)
    const task = getAiResponsePayload(response, '创建 AI 分析任务失败')
    const taskId = task.taskId || task.task_id || task.id
    if (!taskId) {
      throw new Error('后端未返回 taskId')
    }

    aiAnalysisTaskId.value = taskId
    aiAnalysisTaskStatus.value = task.status || 'running'
    aiAnalysisMessage.value = '任务已创建，正在连接进度流...'
    appendAiAnalysisEvent({
      type: 'started',
      step: 'queued',
      status: aiAnalysisTaskStatus.value,
      progress: 0,
      message: '任务已创建'
    })
    startAiAnalysisPolling(taskId)
    subscribeAiAnalysisTaskStream(taskId)
  } catch (error) {
    console.error('AI analyze pod failed:', error)
    ElMessage.error(error.message || 'AI 分析诊断失败')
    aiAnalysisTaskStatus.value = 'failed'
    aiAnalysisLoading.value = false
    stopAiAnalysisPolling()
  }
}

const loadMonitoringData = async (pod) => {
  monitoringLoading.value = true
  
  try {
    console.log('🔍 加载Pod监控数据:', pod.name)
    
    // 调用真实的监控API
    const response = await k8sApi.getPodMetrics(routeParams.clusterId, routeParams.namespace, pod.name)
    const responseData = response.data || response
    
    if (responseData.code === 200 && responseData.data) {
      const metrics = responseData.data
      
      // 转换API响应数据格式
      monitoringData.value = {
        cpu: {
          used: metrics.totalUsage?.cpu || '-',
          limit: metrics.resourceQuota?.cpu || metrics.resourceQuota?.limits?.cpu || podDetail.value.resources?.limits?.cpu || '-',
          percentage: Math.round(metrics.usageRate?.cpuRate || 0)
        },
        memory: {
          used: metrics.totalUsage?.memory || '-',
          limit: metrics.resourceQuota?.memory || metrics.resourceQuota?.limits?.memory || podDetail.value.resources?.limits?.memory || '-',
          percentage: Math.round(metrics.usageRate?.memoryRate || 0)
        },
        network: {
          rx: metrics.network?.rx || metrics.network?.receive || '-',
          tx: metrics.network?.tx || metrics.network?.transmit || '-'
        },
        disk: {
          used: metrics.disk?.used || '-',
          limit: metrics.disk?.limit || '-',
          percentage: Math.round(metrics.disk?.percentage || 0)
        },
        containers: metrics.containers || [],
        timestamp: metrics.timestamp
      }
      
      console.log('✅ 监控数据加载完成:', monitoringData.value)
    } else {
      console.warn('⚠️ 监控API返回异常:', responseData.message)
      ElMessage.warning(responseData.message || '暂无 Pod 监控数据')
    }
    
  } catch (error) {
    console.warn('⚠️ 监控API调用失败:', error)
    ElMessage.warning('暂无 Pod 监控数据')
  } finally {
    monitoringLoading.value = false
  }
}

// 保存YAML
const handleSaveYaml = async () => {
  try {
    loading.value = true
    console.log('💾 开始保存YAML内容...')
    
    if (!yamlContent.value || !yamlContent.value.trim()) {
      ElMessage.warning('YAML内容不能为空')
      return
    }
    
    // 验证YAML格式
    try {
      // 简单的YAML格式验证
      if (!yamlContent.value.includes('apiVersion') || !yamlContent.value.includes('kind')) {
        ElMessage.warning('YAML格式不正确，缺少必要的apiVersion或kind字段')
        return
      }
    } catch (yamlError) {
      ElMessage.error('YAML格式错误: ' + yamlError.message)
      return
    }
    
    // 尝试使用kubectl apply方式更新
    console.log('使用kubectl apply方式更新资源...')
    const response = await k8sApi.applyYaml(routeParams.clusterId, yamlContent.value)
    const responseData = response.data || response
    
    if (responseData.code === 200) {
      ElMessage.success('YAML保存成功！')
      dialogStates.yamlVisible = false
      
      // 刷新页面数据
      console.log('⚡ 刷新页面数据...')
      await loadPodDetail()
      await getRelatedPods()
      
      console.log('✅ YAML保存并刷新完成')
    } else {
      console.error('❌ 保存YAML失败:', responseData)
      ElMessage.error(responseData.message || '保存YAML失败')
    }
  } catch (error) {
    console.error('❌ 保存YAML异常:', error)
    
    if (error.response && error.response.status === 404) {
      ElMessage.error('kubectl apply接口未实现，请联系管理员')
    } else {
      ElMessage.error('保存YAML失败: ' + (error.message || '网络连接异常'))
    }
  } finally {
    loading.value = false
  }
}

// 打开终端
const handleTerminal = (container) => {
  selectedContainer.value = container
  ElMessage.success('正在跳转到终端页面...')
  
  // 跳转到终端页面
  router.push({
    path: `/k8s/terminal/${routeParams.clusterId}/${routeParams.namespace}/${routeParams.podName}`,
    query: {
      container: container.name || container,
      autoConnect: '1'
    }
  })
}

// 删除Pod
const handleDelete = async (podRow) => {
  const podToDelete = podRow || { name: routeParams.podName }

  try {
    await ElMessageBox.confirm(`确定要删除Pod "${podToDelete.name}" 吗？`, '警告', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })

    loading.value = true
    const response = await k8sApi.deletePod(routeParams.clusterId, routeParams.namespace, podToDelete.name)
    const responseData = response.data || response

    if (responseData.code === 200) {
      ElMessage.success(`Pod "${podToDelete.name}" 删除成功`)

      // 如果删除的是当前页面的Pod，则返回上一页
      if (podToDelete.name === routeParams.podName) {
        handleGoBack()
      } else {
        // 否则刷新页面数据
        handleQuery(true)
      }
    } else {
      ElMessage.error(responseData.message || 'Pod删除失败')
    }
  } catch (error) {
    if (error !== 'cancel') {
      console.error('Pod删除失败:', error)
      ElMessage.error('Pod删除失败，请检查网络连接')
    }
  } finally {
    loading.value = false
  }
}

// 获取状态标签类型
const getStatusType = (status) => {
  const statusMap = {
    'Running': 'success',
    'Pending': 'warning',
    'Failed': 'danger',
    'Succeeded': 'success',
    'Unknown': 'info'
  }
  return statusMap[status] || 'info'
}

// 获取资源类型颜色
const getResourceTypeColor = (resourceKind) => {
  const colorMap = {
    'Pod': 'success',
    'ReplicaSet': 'primary', 
    'Deployment': 'warning',
    'Service': 'info',
    'ConfigMap': '',
    'Secret': 'danger',
    'Ingress': 'success'
  }
  return colorMap[resourceKind] || ''
}

// 格式化时间
const formatTime = (timestamp) => {
  if (!timestamp) return '-'
  return new Date(timestamp).toLocaleString('zh-CN')
}

// 复制到剪贴板
const formatAiJson = value => {
  if (!value) return '-'
  try {
    return JSON.stringify(value, null, 2)
  } catch (error) {
    return String(value)
  }
}

const escapeAiHtml = value => String(value ?? '')
  .replace(/&/g, '&amp;')
  .replace(/</g, '&lt;')
  .replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;')
  .replace(/'/g, '&#39;')

const renderAiAnalysisText = text => {
  if (!text) return '暂无分析文本'
  return String(text)
    .replace(/\r/g, '')
    .split('\n')
    .map(rawLine => {
      const line = rawLine.trim()
      if (!line) return '<div class="ai-analysis-gap"></div>'

      const heading = line.match(/^#{1,6}\s+(.+)$/)
      if (heading) {
        return `<h4>${escapeAiHtml(heading[1])}</h4>`
      }

      const ordered = line.match(/^(\d+)\.\s+(.+)$/)
      if (ordered) {
        return `<p class="ai-analysis-line is-ordered"><span class="ai-analysis-marker">${ordered[1]}</span><span class="ai-analysis-line-content">${renderAiInline(ordered[2])}</span></p>`
      }

      const bullet = line.match(/^[-*]\s+(.+)$/)
      if (bullet) {
        return `<p class="ai-analysis-line is-bullet"><span class="ai-analysis-marker"></span><span class="ai-analysis-line-content">${renderAiInline(bullet[1])}</span></p>`
      }

      return `<p class="ai-analysis-line">${renderAiInline(line)}</p>`
    })
    .join('')
}

const renderAiInline = value => {
  let html = escapeAiHtml(value)
  html = html.replace(/`([^`]+)`/g, '<code>$1</code>')
  html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
  return html
}

const copyToClipboard = (text, message = '已复制到剪贴板') => {
  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success(message)
  }).catch(() => {
    ElMessage.error('复制失败')
  })
}

// YAML搜索功能
const searchYamlContent = () => {
  if (!yamlSearchText.value.trim()) {
    yamlSearchResults.value = []
    yamlSearchCurrentIndex.value = -1
    return
  }

  const searchText = yamlSearchText.value.toLowerCase()
  const content = yamlContent.value
  const lines = content.split('\n')
  
  yamlSearchResults.value = []
  
  lines.forEach((line, index) => {
    const lowerLine = line.toLowerCase()
    let startIndex = 0
    
    while (true) {
      const matchIndex = lowerLine.indexOf(searchText, startIndex)
      if (matchIndex === -1) break
      
      yamlSearchResults.value.push({
        lineIndex: index,
        lineNumber: index + 1,
        matchIndex: matchIndex,
        line: line,
        preview: line.trim()
      })
      
      startIndex = matchIndex + searchText.length
    }
  })
  
  yamlSearchCurrentIndex.value = yamlSearchResults.value.length > 0 ? 0 : -1
  
  if (yamlSearchResults.value.length > 0) {
    ElMessage.success(`找到 ${yamlSearchResults.value.length} 个匹配项`)
  } else {
    ElMessage.info('未找到匹配内容')
  }
}

// 跳转到上一个搜索结果
const goToPreviousSearchResult = () => {
  if (yamlSearchResults.value.length === 0) return
  
  yamlSearchCurrentIndex.value = yamlSearchCurrentIndex.value <= 0 
    ? yamlSearchResults.value.length - 1 
    : yamlSearchCurrentIndex.value - 1
    
  scrollToSearchResult()
}

// 跳转到下一个搜索结果
const goToNextSearchResult = () => {
  if (yamlSearchResults.value.length === 0) return
  
  yamlSearchCurrentIndex.value = yamlSearchCurrentIndex.value >= yamlSearchResults.value.length - 1
    ? 0 
    : yamlSearchCurrentIndex.value + 1
    
  scrollToSearchResult()
}

// 滚动到当前搜索结果
const scrollToSearchResult = () => {
  if (yamlSearchCurrentIndex.value === -1 || yamlSearchResults.value.length === 0) return
  
  const currentResult = yamlSearchResults.value[yamlSearchCurrentIndex.value]
  console.log('📍 跳转到搜索结果:', currentResult.lineNumber, '行')
  
  // 通过事件通知CodeEditor滚动到指定行
  nextTick(() => {
    const yamlEditor = document.querySelector('.yaml-dialog .code-editor__textarea')
    if (yamlEditor) {
      const lines = yamlContent.value.split('\n')
      const targetLine = currentResult.lineIndex
      const lineHeight = 21 // 估算行高
      const scrollTop = targetLine * lineHeight
      
      yamlEditor.scrollTop = Math.max(0, scrollTop - 100) // 留一些上边距
    }
  })
}

// 清空搜索
const clearYamlSearch = () => {
  yamlSearchText.value = ''
  yamlSearchResults.value = []
  yamlSearchCurrentIndex.value = -1
}

// 日志搜索功能
const searchLogsContent = () => {
  if (!logsSearchText.value.trim()) {
    logsSearchResults.value = []
    logsSearchCurrentIndex.value = -1
    return
  }

  const searchText = logsSearchText.value.toLowerCase()
  const content = currentLogs.value
  const lines = content.split('\n')
  
  logsSearchResults.value = []
  
  lines.forEach((line, index) => {
    const lowerLine = line.toLowerCase()
    let startIndex = 0
    
    while (true) {
      const matchIndex = lowerLine.indexOf(searchText, startIndex)
      if (matchIndex === -1) break
      
      logsSearchResults.value.push({
        lineIndex: index,
        lineNumber: index + 1,
        matchIndex: matchIndex,
        line: line,
        preview: line.trim()
      })
      
      startIndex = matchIndex + searchText.length
    }
  })
  
  logsSearchCurrentIndex.value = logsSearchResults.value.length > 0 ? 0 : -1
  
  if (logsSearchResults.value.length > 0) {
    ElMessage.success(`找到 ${logsSearchResults.value.length} 个匹配项`)
    scrollToLogsSearchResult()
  } else {
    ElMessage.info('未找到匹配内容')
  }
}

// 跳转到上一个日志搜索结果
const goToPreviousLogsSearchResult = () => {
  if (logsSearchResults.value.length === 0) return
  
  logsSearchCurrentIndex.value = logsSearchCurrentIndex.value <= 0 
    ? logsSearchResults.value.length - 1 
    : logsSearchCurrentIndex.value - 1
    
  scrollToLogsSearchResult()
}

// 跳转到下一个日志搜索结果
const goToNextLogsSearchResult = () => {
  if (logsSearchResults.value.length === 0) return
  
  logsSearchCurrentIndex.value = logsSearchCurrentIndex.value >= logsSearchResults.value.length - 1
    ? 0 
    : logsSearchCurrentIndex.value + 1
    
  scrollToLogsSearchResult()
}

// 滚动到当前日志搜索结果
const scrollToLogsSearchResult = () => {
  if (logsSearchCurrentIndex.value === -1 || logsSearchResults.value.length === 0) return
  
  const currentResult = logsSearchResults.value[logsSearchCurrentIndex.value]
  console.log('📍 跳转到日志搜索结果:', currentResult.lineNumber, '行')
  
  // 通过事件通知CodeEditor滚动到指定行
  nextTick(() => {
    logsEditorRef.value?.scrollToLine?.(currentResult.lineIndex + 1)
  })
}

// 清空日志搜索
const clearLogsSearch = () => {
  logsSearchText.value = ''
  logsSearchResults.value = []
  logsSearchCurrentIndex.value = -1
}

// 从Pod详情生成YAML内容
const generateYamlFromDetail = () => {
  try {
    console.log('🔧 从Pod详情生成YAML...')
    console.log('Pod详情数据:', podDetail.value)
    
    if (!podDetail.value || Object.keys(podDetail.value).length === 0) {
      ElMessage.warning('Pod详情数据为空，无法生成YAML')
      return
    }
    
    // 构造基本的Pod YAML结构
    const yamlObject = {
      apiVersion: 'v1',
      kind: 'Pod',
      metadata: {
        name: routeParams.podName,
        namespace: routeParams.namespace,
        labels: podDetail.value.labels || {},
        annotations: podDetail.value.annotations || {}
      },
      spec: {
        containers: (containers.value || []).map(container => ({
          name: container.name,
          image: container.image,
          ports: container.ports || [],
          env: container.env || [],
          resources: container.resources || {},
          volumeMounts: container.volumeMounts || []
        })),
        volumes: podDetail.value.volumes || [],
        restartPolicy: podDetail.value.spec?.restartPolicy || 'Always',
        nodeName: podDetail.value.nodeName,
        serviceAccount: podDetail.value.spec?.serviceAccount,
        serviceAccountName: podDetail.value.spec?.serviceAccountName,
        nodeSelector: podDetail.value.spec?.nodeSelector || {},
        tolerations: podDetail.value.spec?.tolerations || [],
        affinity: podDetail.value.spec?.affinity
      },
      status: {
        phase: podDetail.value.status || podDetail.value.phase,
        podIP: podDetail.value.podIP,
        hostIP: podDetail.value.hostIP,
        startTime: podDetail.value.startTime || podDetail.value.createdAt,
        conditions: podDetail.value.conditions || []
      }
    }
    
    // 清理undefined值
    const cleanObject = (obj) => {
      if (Array.isArray(obj)) {
        return obj.map(cleanObject).filter(item => item !== undefined)
      } else if (obj !== null && typeof obj === 'object') {
        const cleaned = {}
        Object.keys(obj).forEach(key => {
          const value = cleanObject(obj[key])
          if (value !== undefined && value !== null && value !== '' && 
              !(Array.isArray(value) && value.length === 0) &&
              !(typeof value === 'object' && Object.keys(value).length === 0)) {
            cleaned[key] = value
          }
        })
        return Object.keys(cleaned).length > 0 ? cleaned : undefined
      }
      return obj
    }
    
    const cleanedYaml = cleanObject(yamlObject)
    
    // 转换为YAML字符串
    const yamlString = `# Pod YAML (从详情生成)
# 名称: ${routeParams.podName}
# 命名空间: ${routeParams.namespace}
# 生成时间: ${new Date().toLocaleString('zh-CN')}

apiVersion: v1
kind: Pod
metadata:
  name: ${routeParams.podName}
  namespace: ${routeParams.namespace}
  labels:${Object.keys(cleanedYaml.metadata.labels || {}).length > 0 ? 
    Object.entries(cleanedYaml.metadata.labels).map(([k, v]) => `\n    ${k}: ${v}`).join('') : 
    '\n    # No labels'}
spec:
  containers:${cleanedYaml.spec.containers.map((container, index) => `
  - name: ${container.name}
    image: ${container.image}${container.ports?.length > 0 ? `
    ports:${container.ports.map(port => `
    - containerPort: ${port.containerPort}
      protocol: ${port.protocol || 'TCP'}`).join('')}` : ''}${container.env?.length > 0 ? `
    env:${container.env.map(envVar => `
    - name: ${envVar.name}
      value: ${envVar.value || envVar.valueFrom ? JSON.stringify(envVar.valueFrom) : ''}`).join('')}` : ''}`).join('')}
  restartPolicy: ${cleanedYaml.spec.restartPolicy || 'Always'}${cleanedYaml.spec.nodeName ? `
  nodeName: ${cleanedYaml.spec.nodeName}` : ''}${Object.keys(cleanedYaml.spec.nodeSelector || {}).length > 0 ? `
  nodeSelector:${Object.entries(cleanedYaml.spec.nodeSelector).map(([k, v]) => `
    ${k}: ${v}`).join('')}` : ''}
status:
  phase: ${cleanedYaml.status.phase || 'Unknown'}${cleanedYaml.status.podIP ? `
  podIP: ${cleanedYaml.status.podIP}` : ''}${cleanedYaml.status.hostIP ? `
  hostIP: ${cleanedYaml.status.hostIP}` : ''}${cleanedYaml.status.startTime ? `
  startTime: ${cleanedYaml.status.startTime}` : ''}
`

    // 设置YAML对话框内容
    yamlContent.value = yamlString
    dialogStates.yamlVisible = true
    ElMessage.success('YAML已从Pod详情生成')
    console.log('✅ YAML对话框生成成功')

  } catch (error) {
    console.error('❌ 生成YAML失败:', error)
    ElMessage.error('生成YAML失败: ' + error.message)
  }
}

// 返回上一页
const handleGoBack = () => {
  router.back()
}
</script>

<template>
  <div class="k8s-pod-management">
    <el-card shadow="never" class="pod-card">
      <!-- 页面头部 -->
      <template #header>
        <div class="page-hero">
          <div class="header-left">
            <span class="page-title">容器详情 - {{ workloadName }}</span>
          </div>
          <div class="header-actions">
            <el-button class="back-button" :icon="ArrowLeft" size="small" @click="handleGoBack">返回</el-button>
            <el-button class="refresh-button" :icon="Refresh" size="small" @click="() => handleQuery(true)">刷新</el-button>
            <el-button class="yaml-button" :icon="View" size="small" @click="handleViewYaml">查看YAML</el-button>
            <el-button class="ai-button" :icon="MagicStick" size="small" @click="handleAiAnalyzePod(currentPodInfo)">AI诊断</el-button>
          </div>
        </div>
      </template>

      <!-- 基本信息区域 -->
      <div class="info-section">
        <el-row :gutter="24">
          <!-- 左侧基本信息 -->
          <el-col :span="12">
            <el-card class="info-card" header="基本信息">
              <el-descriptions class="info-descriptions" :column="1" border size="small">
                <el-descriptions-item label="名称">{{ workloadName }}</el-descriptions-item>
                <el-descriptions-item label="命名空间">{{ routeParams.namespace }}</el-descriptions-item>
                <el-descriptions-item label="节点选择器">
                  <div v-if="podDetail.spec?.nodeSelector && Object.keys(podDetail.spec.nodeSelector).length > 0">
                    <el-tag
                      v-for="(value, key) in podDetail.spec.nodeSelector"
                      :key="key"
                      size="small"
                      style="margin: 2px;"
                      @click="copyToClipboard(`${key}=${value}`, '节点选择器已复制')"
                    >
                      {{ key }}={{ value }}
                    </el-tag>
                  </div>
                  <span v-else>-</span>
                </el-descriptions-item>
                <el-descriptions-item label="标签">
                  <div v-if="Object.keys(podLabels).length > 0" class="labels-container">
                    <div class="labels-display">
                      <el-tag
                        v-for="(value, key) in visibleLabels"
                        :key="key"
                        size="small"
                        style="margin: 2px;"
                        @click="copyToClipboard(`${key}=${value}`, '标签已复制')"
                      >
                        {{ key }}={{ value }}
                      </el-tag>
                      
                      <el-button
                        v-if="hasMoreLabels"
                        type="text"
                        size="small"
                        @click="toggleLabelsExpanded"
                        style="margin-left: 8px; color: #409eff;"
                      >
                        {{ labelsExpanded ? '折叠' : `展开 (+${hiddenLabelsCount})` }}
                      </el-button>
                    </div>
                  </div>
                  <span v-else>-</span>
                </el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>

          <!-- 右侧状态信息 -->
          <el-col :span="12">
            <el-card class="info-card" header="状态信息">
              <el-descriptions class="info-descriptions" :column="1" border size="small">
                <el-descriptions-item label="创建时间">{{ formatTime(podCreationTime) }}</el-descriptions-item>
                <el-descriptions-item label="重启策略">{{ podDetail.spec?.restartPolicy || '-' }}</el-descriptions-item>
                <el-descriptions-item label="运行时间">{{ formatRunningTime(podDetail.runningTime) }}</el-descriptions-item>
                <el-descriptions-item label="期望Pod数量">{{ expectedPodCount }}</el-descriptions-item>
                <el-descriptions-item label="状态">
                  <el-tag :type="getStatusType(podStatus)" effect="dark">{{ podStatus }}</el-tag>
                </el-descriptions-item>
              </el-descriptions>
            </el-card>
          </el-col>
        </el-row>
      </div>

      <div class="info-section">
        <el-card class="info-card resource-usage-card">
          <template #header>
            <div class="resource-history-header">
              <div class="resource-history-title">
                <el-icon><TrendCharts /></el-icon>
                <span>Pod历史趋势</span>
              </div>
              <div class="resource-history-actions">
                <el-select
                  v-model="podMetricRange"
                  size="small"
                  class="resource-history-range"
                  :disabled="detailMetricsLoading"
                  @change="loadDetailMetrics"
                >
                  <el-option
                    v-for="item in podMetricRangeOptions"
                    :key="item.value"
                    :label="item.label"
                    :value="item.value"
                  />
                </el-select>
                <el-tag v-if="podMetricHistory" size="small" type="success" effect="plain">
                  {{ podMetricScopeLabel }}{{ podMetricPodNames.length ? ` / ${podMetricPodNames.length} Pod` : '' }}
                </el-tag>
                <el-tag v-if="hasPodHistoryCharts" size="small" type="info">
                  {{ podHistoryCharts[0]?.points.length || 0 }} 个采样点
                </el-tag>
              </div>
            </div>
          </template>
          <div v-loading="detailMetricsLoading" class="resource-usage-grid">
            <template v-if="hasPodHistoryCharts">
              <div
                v-for="chart in podHistoryCharts"
                :key="chart.key"
                class="resource-history-panel"
              >
                <div class="resource-history-panel-head">
                  <span class="resource-history-panel-title">{{ chart.title }}</span>
                  <div class="resource-history-panel-tools">
                    <el-button
                      :icon="FullScreen"
                      class="resource-history-expand"
                      size="small"
                      text
                      title="放大查看"
                      @click="openHistoryChartDialog(chart)"
                    />
                    <span class="resource-history-panel-value">{{ formatHistoryValue(chart.latest, chart) }}</span>
                  </div>
                </div>
                <div
                  class="resource-history-chart"
                  :ref="el => setPodHistoryChartRef(chart.key, el)"
                ></div>
              </div>
            </template>
            <el-empty v-else :description="detailMetricsError || '暂无 CPU / 内存监控数据'" :image-size="64" />
          </div>
        </el-card>
      </div>

      <el-dialog
        v-model="historyChartDialogVisible"
        class="history-chart-dialog"
        width="92%"
        top="5vh"
        append-to-body
        destroy-on-close
        :title="expandedHistoryChart?.title || '趋势详情'"
        @opened="renderExpandedHistoryChart"
        @closed="handleHistoryChartDialogClosed"
      >
        <div v-if="expandedHistoryChart" class="history-chart-dialog-body">
          <div class="history-chart-dialog-meta">
            <div class="history-chart-dialog-tags">
              <el-tag size="small" type="success" effect="plain">
                {{ podMetricScopeLabel }}{{ podMetricPodNames.length ? ` / ${podMetricPodNames.length} Pod` : '' }}
              </el-tag>
              <el-tag size="small" type="info" effect="plain">
                单位: {{ expandedHistoryChart.unit || '-' }}
              </el-tag>
            </div>
            <el-select
              v-model="podMetricRange"
              size="small"
              class="history-chart-dialog-range"
              :disabled="detailMetricsLoading"
              @change="loadDetailMetrics"
            >
              <el-option
                v-for="item in podMetricRangeOptions"
                :key="item.value"
                :label="item.label"
                :value="item.value"
              />
            </el-select>
          </div>
          <div ref="expandedHistoryChartRef" class="history-chart-expanded"></div>
          <div class="history-chart-stat-table">
            <div class="history-chart-stat-head">
              <span>曲线</span>
              <span>max</span>
              <span>avg</span>
              <span>current</span>
            </div>
            <div
              v-for="item in expandedHistorySeriesStats"
              :key="item.name"
              class="history-chart-stat-row"
            >
              <span class="history-chart-stat-name">
                <i :style="{ backgroundColor: item.color }"></i>
                {{ item.name }}
              </span>
              <span>{{ formatHistoryValue(item.max, expandedHistoryChart) }}</span>
              <span>{{ formatHistoryValue(item.avg, expandedHistoryChart) }}</span>
              <span>{{ formatHistoryValue(item.current, expandedHistoryChart) }}</span>
            </div>
          </div>
        </div>
      </el-dialog>

      <PodDetailTabs
        ref="logsEditorRef"
        v-model="activeTab"
        v-model:selected-container-for-logs="selectedContainerForLogs"
        v-model:selected-log-container="selectedLogContainer"
        v-model:follow-logs="followLogs"
        v-model:auto-scroll="autoScroll"
        v-model:show-previous-logs="showPreviousLogs"
        v-model:logs-search-text="logsSearchText"
        v-model:current-logs="currentLogs"
        :cluster-id="routeParams.clusterId"
        :namespace="routeParams.namespace"
        :loading="loading"
        :all-related-pods="allRelatedPods"
        :history-versions="historyVersions"
        :events="events"
        :related-resources="relatedResources"
        :related-resources-loading="relatedResourcesLoading"
        :logs-search-results="logsSearchResults"
        :logs-search-current-index="logsSearchCurrentIndex"
        :logs-loading="logsLoading"
        :logs-update-key="logsUpdateKey"
        :max-log-lines="maxLogLines"
        :last-log-refresh-time="lastLogRefreshTime"
        :ws-status="wsStatus"
        :get-pod-status-tag="getPodStatusTag"
        :get-pod-status-text="getPodStatusText"
        :get-status-type="getStatusType"
        :format-running-time="formatRunningTime"
        :format-time="formatTime"
        @tab-change="handleTabChange"
        @show-monitoring="handleShowMonitoring"
        @view-yaml="handleViewYamlForPod"
        @view-logs="handleViewLogs"
        @terminal="handleTerminal"
        @file-manager="handleOpenPodFileManager"
        @ai-analyze="handleAiAnalyzePod"
        @delete="handleDelete"
        @rollback="handleRollback"
        @refresh-history="getHistoryVersions"
        @refresh-events="getEvents"
        @show-pod-detail-events="showPodDetailEvents"
        @refresh-related-resources="refreshRelatedResources"
        @container-change="handleContainerChange"
        @log-container-change="handleLogContainerChange"
        @search-logs="searchLogsContent"
        @clear-logs-search="clearLogsSearch"
        @previous-logs-search-result="goToPreviousLogsSearchResult"
        @next-logs-search-result="goToNextLogsSearchResult"
        @refresh-logs="handleRefreshLogs"
        @download-logs="handleDownloadLogs"
      />

      <PodFileManagerDialog
        v-if="currentFileBrowserPod.podName"
        v-model:visible="podFileBrowserVisible"
        :cluster-id="routeParams.clusterId"
        :namespace-name="currentFileBrowserPod.namespace"
        :pod-name="currentFileBrowserPod.podName"
        :container-name="currentFileBrowserPod.containerName"
        :container-names="currentFileBrowserPod.containerNames"
      />
    </el-card>

    <!-- 日志查看对话框 -->
    <el-dialog
      v-model="dialogStates.logsVisible"
      :title="`容器日志 - ${selectedContainer?.name || ''}`"
      width="80%"
      class="logs-dialog"
    >
      <div class="logs-container">
        <pre class="logs-content">{{ logs }}</pre>
      </div>
      <template #footer>
        <el-button @click="copyToClipboard(logs, '日志已复制')">复制日志</el-button>
        <el-button type="primary" @click="handleRefreshLogs">刷新日志</el-button>
      </template>
    </el-dialog>

    <!-- YAML查看对话框(只读) -->
    <el-dialog
      v-model="dialogStates.yamlVisible"
      title="Pod YAML (只读)"
      width="900px"
      class="yaml-dialog"
      modal-class="k8s-main-content-dialog-modal"
      append-to-body
      @closed="clearYamlSearch"
    >
      <!-- 搜索工具栏 -->
      <div class="yaml-search-toolbar">
        <el-row :gutter="12" type="flex" align="middle">
          <el-col :span="8">
            <el-input
              v-model="yamlSearchText"
              placeholder="搜索YAML内容..."
              :prefix-icon="Search"
              clearable
              @keyup.enter="searchYamlContent"
              @clear="clearYamlSearch"
              size="small"
              style="width: 280px;"
            />
          </el-col>
          <el-col :span="8">
            <el-button-group size="small">
              <el-button type="warning" :icon="Search" @click="searchYamlContent">搜索</el-button>
              <el-button type="success" :icon="ArrowUp" @click="goToPreviousSearchResult" :disabled="yamlSearchResults.length === 0"></el-button>
              <el-button type="primary" :icon="ArrowDown" @click="goToNextSearchResult" :disabled="yamlSearchResults.length === 0"></el-button>
            </el-button-group>
          </el-col>
          <el-col :span="8">
            <el-text v-if="yamlSearchResults.length > 0" type="info" size="small">
              {{ yamlSearchCurrentIndex + 1 }} / {{ yamlSearchResults.length }}
            </el-text>
          </el-col>
        </el-row>
      </div>

      <el-alert
        title="提示: Pod的YAML配置为只读"
        type="info"
        description="Pod是由工作负载(Deployment/StatefulSet等)管理的,如需修改请编辑对应的工作负载YAML"
        :closable="false"
        style="margin-bottom: 16px;"
      />

      <div class="yaml-container">
        <CodeEditor
          v-model="yamlContent"
          language="yaml"
          height="430px"
          fontSize="12px"
          :readonly="true"
          :searchText="yamlSearchText"
          :searchResults="yamlSearchResults"
          :currentSearchIndex="yamlSearchCurrentIndex"
        />
      </div>
      <template #footer>
        <el-button @click="dialogStates.yamlVisible = false">关闭</el-button>
        <el-button type="primary" :icon="CopyDocument" @click="copyToClipboard(yamlContent, 'YAML已复制')">复制YAML</el-button>
      </template>
    </el-dialog>

    <!-- 回滚确认对话框 -->
    <el-dialog
      v-model="dialogStates.rollbackVisible"
      title="回滚确认"
      width="500px"
    >
      <div class="rollback-confirmation">
        <el-alert
          title="回滚操作风险提示"
          type="warning"
          description="回滚操作将会替换当前运行的版本，请确认您要执行此操作。"
          show-icon
          :closable="false"
          style="margin-bottom: 20px;"
        />

        <el-descriptions title="回滚详情" :column="1" border>
          <el-descriptions-item label="目标版本">{{ rollbackForm.targetVersion }}</el-descriptions-item>
          <el-descriptions-item label="版本描述">{{ rollbackForm.versionInfo.description }}</el-descriptions-item>
          <el-descriptions-item label="创建时间">{{ formatTime(rollbackForm.versionInfo.creationTime) }}</el-descriptions-item>
          <el-descriptions-item label="镜像" v-if="rollbackForm.versionInfo.image">{{ rollbackForm.versionInfo.image }}</el-descriptions-item>
        </el-descriptions>
      </div>

      <template #footer>
        <el-button @click="dialogStates.rollbackVisible = false">取消</el-button>
        <el-button
          type="danger"
          :loading="rollbackLoading"
          @click="handleConfirmRollback"
        >
          确认回滚
        </el-button>
      </template>
    </el-dialog>

    <!-- 终端对话框 -->
    <el-dialog
      v-model="dialogStates.terminalVisible"
      :title="`终端 - ${selectedContainer?.name || ''}`"
      width="80%"
      class="terminal-dialog"
    >
      <div class="terminal-container">
        <div class="terminal-content">
          <div class="terminal-header">终端连接功能开发中...</div>
          <div class="terminal-body">
            <p>即将支持Web终端连接功能</p>
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- 监控弹框 -->
    <el-dialog
      v-model="dialogStates.monitoringVisible"
      :title="`Pod监控 - ${selectedPodForMonitoring?.name || ''}`"
      width="60%"
      class="monitoring-dialog"
    >
      <div class="monitoring-container" v-loading="monitoringLoading">
        <el-row :gutter="16" style="margin-bottom: 16px;">
          <el-col :span="12">
            <el-card class="metric-card">
              <div class="metric-header">
                <span class="metric-title">CPU使用率</span>
                <span class="metric-value">{{ monitoringData.cpu.percentage }}%</span>
              </div>
              <el-progress 
                :percentage="monitoringData.cpu.percentage" 
                :color="monitoringData.cpu.percentage > 80 ? '#F56C6C' : monitoringData.cpu.percentage > 60 ? '#E6A23C' : '#67C23A'"
                :stroke-width="8"
              />
              <div class="metric-details">
                <span>已用: {{ monitoringData.cpu.used }}</span>
                <span>限制: {{ monitoringData.cpu.limit }}</span>
              </div>
            </el-card>
          </el-col>
          <el-col :span="12">
            <el-card class="metric-card">
              <div class="metric-header">
                <span class="metric-title">内存使用率</span>
                <span class="metric-value">{{ monitoringData.memory.percentage }}%</span>
              </div>
              <el-progress 
                :percentage="monitoringData.memory.percentage" 
                :color="monitoringData.memory.percentage > 80 ? '#F56C6C' : monitoringData.memory.percentage > 60 ? '#E6A23C' : '#67C23A'"
                :stroke-width="8"
              />
              <div class="metric-details">
                <span>已用: {{ monitoringData.memory.used }}</span>
                <span>限制: {{ monitoringData.memory.limit }}</span>
              </div>
            </el-card>
          </el-col>
        </el-row>
        
        <el-row :gutter="16">
          <el-col :span="12">
            <el-card class="metric-card">
              <div class="metric-header">
                <span class="metric-title">网络流量</span>
              </div>
              <div class="network-metrics">
                <div class="network-item">
                  <span class="network-label">入站:</span>
                  <span class="network-value">{{ monitoringData.network.rx }}</span>
                </div>
                <div class="network-item">
                  <span class="network-label">出站:</span>
                  <span class="network-value">{{ monitoringData.network.tx }}</span>
                </div>
              </div>
            </el-card>
          </el-col>
          <el-col :span="12">
            <el-card class="metric-card">
              <div class="metric-header">
                <span class="metric-title">磁盘使用率</span>
                <span class="metric-value">{{ monitoringData.disk.percentage }}%</span>
              </div>
              <el-progress 
                :percentage="monitoringData.disk.percentage" 
                :color="monitoringData.disk.percentage > 80 ? '#F56C6C' : monitoringData.disk.percentage > 60 ? '#E6A23C' : '#67C23A'"
                :stroke-width="8"
              />
              <div class="metric-details">
                <span>已用: {{ monitoringData.disk.used }}</span>
                <span>限制: {{ monitoringData.disk.limit }}</span>
              </div>
            </el-card>
          </el-col>
        </el-row>

        <!-- 容器级别监控数据 -->
        <el-card v-if="monitoringData.containers && monitoringData.containers.length > 0" class="monitoring-detail-card" style="margin-top: 16px;">
          <template #header>
            <div class="monitoring-card-header">
              <span>容器监控详情</span>
              <el-tag v-if="monitoringData.timestamp" type="info" size="small">
                更新时间: {{ formatTimestamp(monitoringData.timestamp) }}
              </el-tag>
            </div>
          </template>
          
          <el-table class="detail-table monitoring-table" :data="monitoringData.containers" size="small" stripe>
            <el-table-column prop="name" label="容器名称" min-width="120" />
            <el-table-column label="CPU使用" min-width="100">
              <template #default="{ row }">
                <div class="container-metric">
                  <span class="metric-text">{{ row.usage?.cpu || '0m' }}</span>
                  <el-progress 
                    v-if="row.usageRate?.cpuRate" 
                    :percentage="Math.round(row.usageRate.cpuRate)" 
                    :stroke-width="6"
                    :show-text="false"
                    :color="row.usageRate.cpuRate > 80 ? '#F56C6C' : row.usageRate.cpuRate > 60 ? '#E6A23C' : '#67C23A'"
                  />
                </div>
              </template>
            </el-table-column>
            <el-table-column label="内存使用" min-width="100">
              <template #default="{ row }">
                <div class="container-metric">
                  <span class="metric-text">{{ row.usage?.memory || '0Mi' }}</span>
                  <el-progress 
                    v-if="row.usageRate?.memoryRate" 
                    :percentage="Math.round(row.usageRate.memoryRate)" 
                    :stroke-width="6"
                    :show-text="false"
                    :color="row.usageRate.memoryRate > 80 ? '#F56C6C' : row.usageRate.memoryRate > 60 ? '#E6A23C' : '#67C23A'"
                  />
                </div>
              </template>
            </el-table-column>
            <el-table-column label="CPU使用率" min-width="80">
              <template #default="{ row }">
                <el-tag 
                  :type="row.usageRate?.cpuRate > 80 ? 'danger' : row.usageRate?.cpuRate > 60 ? 'warning' : 'success'"
                  size="small"
                >
                  {{ Math.round(row.usageRate?.cpuRate || 0) }}%
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="内存使用率" min-width="80">
              <template #default="{ row }">
                <el-tag 
                  :type="row.usageRate?.memoryRate > 80 ? 'danger' : row.usageRate?.memoryRate > 60 ? 'warning' : 'success'"
                  size="small"
                >
                  {{ Math.round(row.usageRate?.memoryRate || 0) }}%
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="状态" min-width="80">
              <template #default="{ row }">
                <el-tag 
                  :type="row.state === 'Running' ? 'success' : row.state === 'Waiting' ? 'warning' : 'danger'"
                  size="small"
                >
                  {{ row.state || 'Unknown' }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </div>
      
      <template #footer>
        <el-button @click="dialogStates.monitoringVisible = false">关闭</el-button>
        <el-button type="primary" @click="loadMonitoringData(selectedPodForMonitoring)" :loading="monitoringLoading">刷新数据</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="dialogStates.aiAnalysisVisible"
      :title="`AI分析诊断 - ${selectedPodForAiAnalysis?.name || routeParams.podName}`"
      width="min(980px, calc(100vw - 32px))"
      top="4vh"
      class="ai-analysis-dialog"
      append-to-body
    >
      <div class="ai-analysis-container">
        <section class="ai-analysis-hero">
          <div class="ai-analysis-title-wrap">
            <div class="ai-analysis-mark">
              <el-icon><MagicStick /></el-icon>
            </div>
            <div>
              <div class="ai-analysis-title">一键 Pod AI 分析</div>
              <div class="ai-analysis-subtitle">
                自动采集状态、事件、当前日志、previous 日志和 CPU/内存指标。
              </div>
            </div>
          </div>
          <div class="ai-analysis-meta">
            <span>{{ routeParams.namespace }}</span>
            <span>{{ aiAnalysisContainerName }}</span>
            <span>{{ aiAnalysisTaskId || '未创建任务' }}</span>
          </div>
        </section>

        <section class="ai-analysis-input-panel">
          <el-input
            v-model="aiAnalysisPrompt"
            type="textarea"
            :rows="3"
            resize="none"
            placeholder="补充你希望 AI 重点关注的问题"
            class="ai-analysis-prompt"
          />
          <div class="ai-analysis-actions">
            <el-button
              type="primary"
              :icon="MagicStick"
              :loading="aiAnalysisLoading"
              @click="handleAiAnalyzePod(selectedPodForAiAnalysis)"
            >
              重新分析
            </el-button>
            <el-button
              :disabled="!aiAnalysisText"
              @click="copyToClipboard(aiAnalysisText, 'AI分析结果已复制')"
            >
              复制结果
            </el-button>
          </div>
        </section>

        <section class="ai-analysis-progress-panel">
          <div class="ai-analysis-progress-head">
            <div>
              <div class="ai-analysis-eyebrow">TASK FLOW</div>
              <div class="ai-analysis-section-title">实时分析进度</div>
            </div>
            <el-tag :type="aiAnalysisStatusMeta.type" effect="plain">
              {{ aiAnalysisStatusMeta.label }}
            </el-tag>
          </div>

          <el-progress
            :percentage="aiAnalysisProgress"
            :status="aiAnalysisProgressStatus"
            :stroke-width="8"
            class="ai-analysis-progress"
          />

          <div
            v-if="aiAnalysisSummaryMeta.rootCause || aiAnalysisSummaryMeta.summary || aiAnalysisSummaryMeta.severity"
            class="ai-analysis-summary-strip"
          >
            <div v-if="aiAnalysisSummaryMeta.rootCause" class="ai-analysis-summary-item is-primary">
              <span>根本原因</span>
              <strong>{{ aiAnalysisSummaryMeta.rootCause }}</strong>
            </div>
            <div v-if="aiAnalysisSummaryMeta.summary" class="ai-analysis-summary-item">
              <span>事件摘要</span>
              <strong>{{ aiAnalysisSummaryMeta.summary }}</strong>
            </div>
            <div v-if="aiAnalysisSummaryMeta.severity" class="ai-analysis-summary-item is-warning">
              <span>严重性</span>
              <strong>{{ aiAnalysisSummaryMeta.severity }}</strong>
            </div>
          </div>

          <div class="ai-analysis-step-grid">
            <div
              v-for="(step, index) in aiAnalysisStepList"
              :key="step.key"
              class="ai-analysis-step"
              :class="{
                'is-done': index < aiAnalysisActiveStep,
                'is-active': index === aiAnalysisActiveStep,
                'is-pending': index > aiAnalysisActiveStep
              }"
            >
              <span>{{ index + 1 }}</span>
              <strong>{{ step.label }}</strong>
            </div>
          </div>

          <div v-if="aiAnalysisMessage" class="ai-analysis-current-message">
            {{ aiAnalysisMessage }}
          </div>

          <div v-if="aiAnalysisEvents.length" class="ai-analysis-event-list">
            <div
              v-for="(item, index) in aiAnalysisEvents.slice(0, 6)"
              :key="`${item.step}-${item.timestamp}-${index}`"
              class="ai-analysis-event-item"
            >
              <div>
                <strong>{{ getAiAnalysisStepLabel(item.step) }}</strong>
                <p>{{ item.message || '-' }}</p>
              </div>
              <time>{{ item.timestamp || '-' }}</time>
            </div>
          </div>
        </section>

        <el-empty
          v-if="!aiAnalysisLoading && !aiAnalysisResult && !aiAnalysisTaskId"
          description="等待分析结果"
          :image-size="72"
          class="ai-analysis-empty"
        />

        <template v-else-if="shouldShowAiAnalysisResult">
          <section class="ai-analysis-section ai-analysis-result-section">
            <div class="ai-analysis-section-head">
              <div>
                <div class="ai-analysis-eyebrow">DIAGNOSIS</div>
                <div class="ai-analysis-section-title">诊断结论</div>
              </div>
              <el-tag type="primary" effect="plain">AI 结果</el-tag>
            </div>
            <div class="ai-analysis-text" v-html="renderAiAnalysisText(aiAnalysisText)"></div>
          </section>

          <section v-if="aiAnalysisHotspots.length" class="ai-analysis-section">
            <div class="ai-analysis-section-head">
              <div>
                <div class="ai-analysis-eyebrow">RESOURCE</div>
                <div class="ai-analysis-section-title">资源热点</div>
              </div>
            </div>
            <div class="ai-analysis-tags">
              <el-tag
                v-for="(item, index) in aiAnalysisHotspots"
                :key="index"
                type="warning"
                effect="plain"
              >
                {{ item.code || item.type || item.name || item }}
              </el-tag>
            </div>
          </section>

          <section v-if="aiAnalysisCollectionErrors.length" class="ai-analysis-section">
            <div class="ai-analysis-section-head">
              <div>
                <div class="ai-analysis-eyebrow">COLLECTION</div>
                <div class="ai-analysis-section-title">采集提示</div>
              </div>
            </div>
            <div class="ai-analysis-alert-list">
              <el-alert
                v-for="(item, index) in aiAnalysisCollectionErrors"
                :key="index"
                type="warning"
                :closable="false"
                show-icon
                :title="String(item.message || item.reason || item)"
              />
            </div>
          </section>

          <el-collapse v-if="aiAnalysisEvidence" class="ai-analysis-evidence-collapse">
            <el-collapse-item title="诊断证据" name="evidence">
              <pre class="ai-analysis-json">{{ formatAiJson(aiAnalysisEvidence) }}</pre>
            </el-collapse-item>
          </el-collapse>
        </template>
      </div>

      <template #footer>
        <div class="ai-analysis-footer">
          <el-button @click="dialogStates.aiAnalysisVisible = false">关闭</el-button>
          <el-button
            type="primary"
            :icon="MagicStick"
            :loading="aiAnalysisLoading"
            @click="handleAiAnalyzePod(selectedPodForAiAnalysis)"
          >
            重新分析
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.k8s-pod-management {
  --aiops-bg-page: #f4f7fb;
  --aiops-surface-cool: rgba(247, 250, 255, 0.84);
  --aiops-surface-strong: rgba(255, 255, 255, 0.92);
  --aiops-surface-warm: rgba(255, 253, 248, 0.94);
  --aiops-text-primary: #172133;
  --aiops-text-secondary: #5f6b7c;
  --aiops-text-muted: #8d97a8;
  --aiops-border-cool: #dfe7f2;
  --aiops-border-mid: rgba(207, 217, 230, 0.92);
  --aiops-brand: #4c8bf5;
  --aiops-brand-soft: rgba(76, 139, 245, 0.12);
  --aiops-warning: #f3b24b;
  --aiops-success: #3fb97f;
  --aiops-danger: #ef6b73;
  --aiops-shadow-soft: 0 20px 60px rgba(17, 24, 39, 0.07);
  --pod-page-height-offset: 190px;
  min-height: calc(100vh - var(--pod-page-height-offset));
  padding: 6px;
  box-sizing: border-box;
  background:
    radial-gradient(circle at 16% 10%, rgba(76, 139, 245, 0.06), transparent 26%),
    linear-gradient(180deg, #f7f9fc 0%, var(--aiops-bg-page) 100%);
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.pod-card {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 50px 10px 10px;
  overflow: hidden;
  border: 1px solid var(--aiops-border-mid);
  border-radius: 28px;
  background: linear-gradient(
    135deg,
    rgba(255, 253, 248, 0.95) 0%,
    rgba(255, 249, 241, 0.92) 38%,
    rgba(247, 250, 255, 0.92) 100%
  );
  box-shadow: var(--aiops-shadow-soft);
  backdrop-filter: blur(18px);
}

.pod-card::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 40px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.82);
  pointer-events: none;
}

.pod-card :deep(.el-card__header) {
  position: relative;
  z-index: 1;
  padding: 0 8px 12px;
  border-bottom: none;
  background: transparent;
}

.pod-card :deep(.el-card__body) {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 0;
}

.page-hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.72);
}

.header-left {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  min-width: 0;
}

.header-left :deep(.el-button) {
  min-width: 76px;
  height: 32px;
  border: 1px solid rgba(214, 223, 235, 0.96);
  border-radius: 13px;
  background: rgba(255, 255, 255, 0.92);
  color: var(--aiops-text-secondary);
  font-weight: 600;
  box-shadow: none;
}

.header-left :deep(.el-button:hover),
.header-left :deep(.el-button:focus) {
  border-color: rgba(153, 181, 230, 0.95);
  color: var(--aiops-brand);
  background: #ffffff;
}

.page-title {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
  color: var(--aiops-text-primary);
  font-size: 24px;
  font-weight: 700;
  line-height: 1.2;
  background: none;
  -webkit-text-fill-color: currentColor;
}

.page-title::before {
  content: 'Kubernetes / Pod Detail';
  color: var(--aiops-brand);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.6px;
}

.page-title::after {
  content: '状态、事件、日志与 AI 诊断';
  color: var(--aiops-text-secondary);
  font-size: 13px;
  font-weight: 400;
  line-height: 1.7;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.header-actions :deep(.el-button) {
  min-width: 88px;
  height: 32px;
  border-radius: 13px;
  font-weight: 600;
  box-shadow: none;
}

.header-actions :deep(.el-button:not(:last-child)) {
  border: 1px solid rgba(214, 223, 235, 0.96);
  background: rgba(255, 255, 255, 0.92);
  color: var(--aiops-text-secondary);
}

.header-actions :deep(.el-button:not(:last-child):hover),
.header-actions :deep(.el-button:not(:last-child):focus) {
  border-color: rgba(153, 181, 230, 0.95);
  color: var(--aiops-brand);
  background: #ffffff;
}

.header-actions :deep(.el-button:last-child) {
  border: none;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
  color: #ffffff;
  box-shadow: 0 10px 18px rgba(76, 139, 245, 0.16);
}

.info-section {
  margin-bottom: 14px;
}

.info-card {
  height: 100%;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 20px;
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.84), rgba(247, 250, 255, 0.8));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
}

.info-card :deep(.el-card__header) {
  padding: 14px 18px 12px;
  color: var(--aiops-text-primary);
  font-weight: 700;
  border-bottom: 1px solid rgba(223, 231, 242, 0.82);
  background: linear-gradient(90deg, rgba(255, 249, 240, 0.92), rgba(237, 242, 253, 0.72));
}

.info-card :deep(.el-card__body) {
  padding: 0;
}

.info-descriptions :deep(.el-descriptions__table) {
  border-color: rgba(223, 231, 242, 0.86);
}

.info-descriptions :deep(.el-descriptions__label.el-descriptions__cell) {
  width: 112px;
  color: var(--aiops-text-muted);
  font-weight: 700;
  background: rgba(247, 250, 255, 0.86);
}

.info-descriptions :deep(.el-descriptions__content.el-descriptions__cell) {
  color: var(--aiops-text-secondary);
  background: rgba(255, 255, 255, 0.72);
}

.detail-tabs :deep(.el-tabs__header) {
  margin: 0;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 20px;
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.8), rgba(247, 250, 255, 0.74));
  padding: 10px 10px 8px;
}

.detail-tabs :deep(.el-tabs__nav-wrap::after),
.detail-tabs :deep(.el-tabs__header.is-top) {
  border-bottom: none;
}

.detail-tabs :deep(.el-tabs__nav) {
  border: none;
  background: rgba(255, 255, 255, 0.54);
  border-radius: 16px;
  padding: 4px;
}

.detail-tabs :deep(.el-tabs__item) {
  height: 34px;
  padding: 0 14px;
  border: none !important;
  border-radius: 12px;
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

.detail-tabs :deep(.el-tabs__item:hover),
.detail-tabs :deep(.el-tabs__item.is-active) {
  color: var(--aiops-brand);
}

.detail-tabs :deep(.el-tabs__item.is-active) {
  background: rgba(255, 255, 255, 0.96);
  box-shadow: inset 0 0 0 1px rgba(223, 231, 242, 0.92);
}

.detail-tabs :deep(.el-tabs__content) {
  margin-top: 10px;
  padding: 12px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.64);
}

.detail-table {
  border: 1px solid rgba(223, 231, 242, 0.88);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.92);
  overflow: hidden;
  box-shadow: none;
}

.detail-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.detail-table :deep(.el-table__header th),
.detail-table :deep(.el-table__fixed-right-patch) {
  background: rgba(248, 250, 255, 0.98) !important;
  border-bottom: 1px solid rgba(223, 231, 242, 0.95);
}

.detail-table :deep(.el-table__header th .cell) {
  color: var(--aiops-text-muted) !important;
  font-size: 12px;
  font-weight: 700 !important;
}

.detail-table :deep(.el-table__header th),
.detail-table :deep(.el-table__header th .cell),
.detail-table :deep(.el-table__body td),
.detail-table :deep(.el-table__body td .cell) {
  text-align: left !important;
}

.detail-table :deep(.el-table__cell) {
  border-bottom-color: rgba(15, 23, 42, 0.045);
}

.detail-table :deep(.el-table__body tr:hover > td.el-table__cell) {
  background: rgba(76, 139, 245, 0.045) !important;
}

.labels-container {
  max-width: 100%;
}

/* Pod名称容器样式 */
.pod-name-container {
  display: flex;
  align-items: center;
  gap: 8px;
  transition: all 0.3s ease;
}

.pod-name-container:hover {
  color: #409eff;
  transform: translateY(-1px);
}

.k8s-icon {
  flex-shrink: 0;
}

.pod-name {
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.labels-display {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 2px;
}

.image-link {
  color: #409eff;
  padding: 0;
  font-size: 12px;
  text-decoration: none;
}

.image-link:hover {
  text-decoration: underline;
}

.logs-dialog .logs-container {
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  background-color: #000;
  color: #fff;
  height: 400px;
  overflow: auto;
}

.logs-content {
  padding: 12px;
  margin: 0;
  white-space: pre-wrap;
  font-family: 'Courier New', monospace;
  font-size: 12px;
}

.yaml-search-toolbar {
  padding: 12px;
  border-bottom: 1px solid #ebeef5;
  background-color: #fafafa;
  margin-bottom: 16px;
}

.yaml-search-toolbar .el-button-group .el-button {
  padding: 7px 12px;
}

.yaml-search-toolbar .el-input {
  font-size: 14px;
}

.yaml-dialog .yaml-container {
  margin-top: 0;
}

.logs-editor-container {
  background-color: #1e1e1e;
  border-radius: 4px;
  overflow-y: auto;
  overflow-x: auto;
  max-height: 680px;
  height: 680px;
}

/* 自定义滚动条样式 - 黑色主题 */
.logs-editor-container::-webkit-scrollbar {
  width: 12px;
  height: 12px;
}

.logs-editor-container::-webkit-scrollbar-track {
  background: #2b2b2b;
  border-radius: 6px;
}

.logs-editor-container::-webkit-scrollbar-thumb {
  background: #4a4a4a;
  border-radius: 6px;
  border: 2px solid #2b2b2b;
}

.logs-editor-container::-webkit-scrollbar-thumb:hover {
  background: #5a5a5a;
}

.logs-editor-container::-webkit-scrollbar-corner {
  background: #2b2b2b;
}

.yaml-dialog .yaml-container {
  border: 1px solid #dcdfe6;
  border-radius: 4px;
}

.yaml-editor :deep(.el-textarea__inner) {
  font-family: 'Courier New', monospace;
  font-size: 12px;
  background-color: #f8f9fa;
}

.terminal-dialog .terminal-container {
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  background-color: #000;
  color: #fff;
  height: 400px;
  padding: 12px;
}

.terminal-header {
  color: #00ff00;
  margin-bottom: 10px;
}

.terminal-body {
  font-family: 'Courier New', monospace;
  font-size: 12px;
}

:deep(.el-descriptions-item__label) {
  font-weight: 600;
}

:deep(.el-tag) {
  cursor: pointer;
}

:deep(.el-tag):hover {
  transform: scale(1.05);
}

:deep(.el-card) {
  border-radius: 8px;
}

:deep(.el-table) {
  border-radius: 8px;
}

/* 日志标签页样式 */
.logs-tab-content {
  padding: 16px;
}

.logs-controls {
  background-color: #f8f9fa;
  padding: 16px;
  border-radius: 8px;
  border: 1px solid #e9ecef;
}

.logs-actions {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.logs-display {
  border: 1px solid #dcdfe6;
  border-radius: 8px;
  background-color: #000;
  height: 680px;
  overflow: hidden;
}

.logs-placeholder {
  padding: 40px;
  text-align: center;
  color: #00ff00;
  background-color: #000;
  height: 100%;
  border-radius: 8px;
}

.logs-content-display {
  height: 100%;
}

.logs-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background-color: #1a1a1a;
  border-bottom: 1px solid #333;
  border-radius: 8px 8px 0 0;
}

.container-name {
  font-weight: 600;
  color: #00ff00;
  font-size: 14px;
}

.logs-info {
  color: #888;
  font-size: 12px;
}

.ws-status {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 6px;
}

.logs-text {
  padding: 16px;
  margin: 0;
  font-family: 'Courier New', Monaco, 'Lucida Console', monospace;
  font-size: 12px;
  line-height: 1.4;
  background-color: #000;
  color: #00ff00;
  white-space: pre-wrap;
  word-break: break-all;
  height: calc(500px - 48px);
  overflow-y: auto;
  border-radius: 0 0 8px 8px;
}

.logs-empty {
  padding: 40px;
  text-align: center;
  color: #00ff00;
  background-color: #000;
  height: 100%;
  border-radius: 8px;
}

.image-info {
  font-family: 'Courier New', Monaco, monospace;
  font-size: 12px;
  color: #666;
}

/* 镜像单元格样式 */
.image-cell {
  word-break: break-all;
  white-space: normal;
  line-height: 1.4;
  max-width: 300px;
}

.image-cell .image-link {
  word-break: break-all;
  white-space: normal;
  text-align: left;
  height: auto;
  line-height: 1.4;
  padding: 4px 8px;
}

/* 操作按钮布局样式 */
.operation-buttons-container {
  display: flex;
  flex-direction: row;
  gap: 14px;
  flex-wrap: nowrap;
  justify-content: center;
  align-items: center;
}

.operation-buttons-container :deep(.el-button) {
  height: auto;
  min-height: 0;
  padding: 0;
  margin-left: 0;
  border: none;
  border-radius: 0;
  background: transparent;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.4;
  box-shadow: none;
  transition: color 0.2s ease, opacity 0.2s ease;
}

.operation-buttons-container :deep(.el-button:hover:not(.is-disabled)) {
  transform: none;
  box-shadow: none;
  opacity: 0.82;
}

.operation-buttons-container :deep(.operation-text.logs) {
  color: #4c8bf5;
}

.operation-buttons-container :deep(.operation-text.ai) {
  color: #7c3aed;
}

.operation-buttons-container :deep(.operation-text.yaml),
.operation-buttons-container :deep(.operation-text.terminal),
.operation-buttons-container :deep(.operation-text.rollback) {
  color: #f3a33a;
}

.operation-buttons-container :deep(.operation-text.delete) {
  color: #ef6b73;
}

.ai-analysis-dialog :deep(.el-dialog) {
  max-width: calc(100vw - 32px);
}

:deep(.ai-analysis-dialog.el-dialog) {
  border-radius: 28px;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
  box-shadow: 0 36px 110px rgba(15, 23, 42, 0.11);
}

:deep(.ai-analysis-dialog .el-dialog__header) {
  padding: 14px 24px 0;
  margin: 0;
}

:deep(.ai-analysis-dialog .el-dialog__title) {
  font-size: 18px;
  font-weight: 700;
  color: #172133;
}

:deep(.ai-analysis-dialog .el-dialog__body) {
  padding: 10px 24px 14px;
  max-height: calc(92vh - 126px);
  overflow: hidden;
  background:
    radial-gradient(circle at top left, rgba(76, 139, 245, 0.05), transparent 22%),
    linear-gradient(180deg, #f8fafc 0%, #f5f8fc 100%);
}

:deep(.ai-analysis-dialog .el-dialog__footer) {
  padding: 0 24px 16px;
  background: #f5f8fc;
}

.ai-analysis-container {
  max-height: calc(92vh - 150px);
  min-height: 260px;
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 4px;
  color: #172133;
}

.ai-analysis-hero {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 16px;
  border-radius: 22px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border: 1px solid rgba(223, 231, 242, 0.92);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.72),
    0 18px 40px rgba(17, 24, 39, 0.05);
}

.ai-analysis-title-wrap {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.ai-analysis-mark {
  width: 38px;
  height: 38px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  color: #4c8bf5;
  background: rgba(255, 255, 255, 0.72);
  border: 1px solid rgba(207, 217, 230, 0.92);
}

.ai-analysis-title {
  font-size: 17px;
  font-weight: 700;
  color: #172133;
}

.ai-analysis-subtitle {
  margin-top: 4px;
  font-size: 13px;
  line-height: 1.5;
  color: #5f6b7c;
}

.ai-analysis-meta {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
  max-width: 360px;
}

.ai-analysis-meta span {
  max-width: 180px;
  padding: 5px 9px;
  border-radius: 999px;
  border: 1px solid rgba(207, 217, 230, 0.92);
  background: rgba(255, 255, 255, 0.68);
  color: #5f6b7c;
  font-size: 11px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-analysis-input-panel,
.ai-analysis-progress-panel,
.ai-analysis-section {
  margin-top: 10px;
  border-radius: 20px;
  border: 1px solid #dfe7f2;
  background: linear-gradient(135deg, rgba(255, 248, 239, 0.96), rgba(247, 250, 255, 0.86));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.78),
    0 16px 38px rgba(17, 24, 39, 0.04);
}

.ai-analysis-input-panel {
  padding: 12px;
}

.ai-analysis-progress-panel {
  padding: 14px;
}

.ai-analysis-progress-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 12px;
}

.ai-analysis-progress-head :deep(.el-tag) {
  max-width: 220px;
  height: auto;
  min-height: 24px;
  white-space: normal;
  line-height: 1.4;
  border-radius: 999px;
  font-weight: 700;
}

.ai-analysis-progress {
  margin-bottom: 10px;
}

.ai-analysis-progress :deep(.el-progress-bar__outer) {
  background: rgba(223, 231, 242, 0.78);
}

.ai-analysis-progress :deep(.el-progress-bar__inner) {
  background: linear-gradient(90deg, #6da0f7 0%, #4c8bf5 100%);
}

.ai-analysis-step-grid {
  display: grid;
  grid-template-columns: repeat(11, minmax(74px, 1fr));
  gap: 6px;
  margin-top: 10px;
  overflow-x: auto;
  overflow-y: hidden;
  padding-bottom: 3px;
}

.ai-analysis-step {
  display: flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
  box-sizing: border-box;
  padding: 7px 6px;
  border-radius: 12px;
  border: 1px solid rgba(207, 217, 230, 0.92);
  background: rgba(255, 255, 255, 0.66);
  color: #8d97a8;
}

.ai-analysis-step span {
  width: 20px;
  height: 20px;
  border-radius: 999px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  font-size: 11px;
  font-weight: 700;
  background: rgba(141, 151, 168, 0.12);
}

.ai-analysis-step strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
  font-weight: 700;
}

.ai-analysis-step.is-done {
  color: #3fb97f;
  border-color: rgba(63, 185, 127, 0.24);
  background: rgba(63, 185, 127, 0.08);
}

.ai-analysis-step.is-active {
  color: #4c8bf5;
  border-color: rgba(76, 139, 245, 0.26);
  background: rgba(76, 139, 245, 0.1);
}

.ai-analysis-summary-strip {
  display: grid;
  grid-template-columns: minmax(0, 1.45fr) minmax(0, 1fr) minmax(120px, 0.55fr);
  gap: 8px;
  margin-top: 10px;
}

.ai-analysis-summary-item {
  min-width: 0;
  padding: 10px 12px;
  border-radius: 14px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  background: rgba(255, 255, 255, 0.68);
}

.ai-analysis-summary-item span {
  display: block;
  margin-bottom: 5px;
  color: #8d97a8;
  font-size: 11px;
  font-weight: 700;
}

.ai-analysis-summary-item strong {
  display: block;
  color: #172133;
  font-size: 13px;
  font-weight: 700;
  line-height: 1.5;
  word-break: normal;
  overflow-wrap: break-word;
}

.ai-analysis-summary-item.is-primary {
  border-color: rgba(76, 139, 245, 0.28);
  background: linear-gradient(135deg, rgba(76, 139, 245, 0.1), rgba(255, 255, 255, 0.72));
}

.ai-analysis-summary-item.is-warning {
  border-color: rgba(239, 151, 74, 0.28);
  background: linear-gradient(135deg, rgba(239, 151, 74, 0.11), rgba(255, 255, 255, 0.7));
}

.ai-analysis-current-message {
  margin-top: 10px;
  padding: 9px 11px;
  border-radius: 14px;
  color: #5f6b7c;
  background: rgba(255, 255, 255, 0.68);
  border: 1px solid rgba(223, 231, 242, 0.86);
  font-size: 13px;
  line-height: 1.5;
}

.ai-analysis-event-list {
  position: relative;
  display: flex;
  flex-direction: column;
  margin-top: 12px;
  padding-left: 18px;
}

.ai-analysis-event-list::before {
  content: '';
  position: absolute;
  left: 5px;
  top: 9px;
  bottom: 9px;
  width: 1px;
  background: linear-gradient(180deg, rgba(76, 139, 245, 0.48), rgba(207, 217, 230, 0.24));
}

.ai-analysis-event-item {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: start;
  padding: 7px 0 9px 10px;
  border-bottom: 1px dashed rgba(207, 217, 230, 0.76);
}

.ai-analysis-event-item::before {
  content: '';
  position: absolute;
  left: -16px;
  top: 14px;
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: #4c8bf5;
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.1);
}

.ai-analysis-event-item:first-child::before {
  background: #3fb97f;
  box-shadow: 0 0 0 4px rgba(63, 185, 127, 0.12);
}

.ai-analysis-event-item:last-child {
  border-bottom: none;
}

.ai-analysis-event-item strong {
  display: block;
  color: #172133;
  font-size: 12px;
}

.ai-analysis-event-item p {
  margin: 2px 0 0;
  color: #5f6b7c;
  font-size: 12px;
  line-height: 1.45;
  word-break: normal;
  overflow-wrap: break-word;
}

.ai-analysis-event-item time {
  color: #8d97a8;
  font-size: 11px;
  white-space: nowrap;
}

.ai-analysis-prompt {
  width: 100%;
}

.ai-analysis-prompt :deep(.el-textarea__inner) {
  border-radius: 14px;
  border-color: rgba(207, 217, 230, 0.92);
  box-shadow: none;
  color: #172133;
  background: rgba(255, 255, 255, 0.82);
  line-height: 1.6;
}

.ai-analysis-actions,
.ai-analysis-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
}

.ai-analysis-actions {
  margin-top: 10px;
}

.ai-analysis-actions :deep(.el-button),
.ai-analysis-footer :deep(.el-button) {
  height: 32px;
  border-radius: 12px;
  font-weight: 600;
}

.ai-analysis-actions :deep(.el-button--primary),
.ai-analysis-footer :deep(.el-button--primary) {
  background: linear-gradient(180deg, #6da0f7 0%, #4c8bf5 100%);
  border-color: transparent;
  box-shadow: 0 6px 12px rgba(76, 139, 245, 0.15);
}

.ai-analysis-empty {
  margin-top: 10px;
  border-radius: 20px;
  border: 1px dashed rgba(207, 217, 230, 0.92);
  background: rgba(255, 255, 255, 0.62);
}

.ai-analysis-section {
  overflow: hidden;
}

.ai-analysis-section-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  padding: 12px 14px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.82);
  background: linear-gradient(90deg, rgba(255, 249, 240, 0.92), rgba(237, 242, 253, 0.72));
}

.ai-analysis-eyebrow {
  margin-bottom: 4px;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.9px;
  color: #8d97a8;
}

.ai-analysis-section-title {
  font-size: 16px;
  font-weight: 700;
  color: #172133;
}

.ai-analysis-text {
  padding: 14px 16px 16px;
  max-height: min(420px, calc(92vh - 390px));
  overflow: auto;
  color: #2f3b52;
  font-size: 13px;
  line-height: 1.75;
  word-break: normal;
  overflow-wrap: break-word;
}

.ai-analysis-text :deep(h4) {
  margin: 14px 0 8px;
  font-size: 14px;
  font-weight: 700;
  color: #172133;
}

.ai-analysis-text :deep(h4:first-child) {
  margin-top: 0;
}

.ai-analysis-text :deep(.ai-analysis-line) {
  margin: 6px 0;
}

.ai-analysis-text :deep(.ai-analysis-line.is-bullet),
.ai-analysis-text :deep(.ai-analysis-line.is-ordered) {
  display: grid;
  grid-template-columns: 22px 1fr;
  gap: 6px;
  align-items: start;
}

.ai-analysis-text :deep(.ai-analysis-line-content) {
  min-width: 0;
  white-space: normal;
  word-break: normal;
  overflow-wrap: break-word;
}

.ai-analysis-text :deep(.ai-analysis-line.is-bullet .ai-analysis-marker) {
  width: 6px;
  height: 6px;
  margin-top: 9px;
  border-radius: 999px;
  background: #4c8bf5;
}

.ai-analysis-text :deep(.ai-analysis-line.is-ordered .ai-analysis-marker) {
  width: 20px;
  height: 20px;
  border-radius: 999px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #4c8bf5;
  background: rgba(76, 139, 245, 0.1);
  font-size: 11px;
  font-weight: 700;
}

.ai-analysis-text :deep(strong) {
  font-weight: 700;
  color: #172133;
}

.ai-analysis-text :deep(code) {
  padding: 2px 5px;
  border-radius: 6px;
  color: #3f6fb6;
  background: rgba(76, 139, 245, 0.1);
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
  white-space: normal;
  word-break: normal;
  overflow-wrap: break-word;
}

.ai-analysis-text :deep(.ai-analysis-gap) {
  height: 6px;
}

.ai-analysis-tags,
.ai-analysis-alert-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 12px 14px 14px;
}

.ai-analysis-tags :deep(.el-tag) {
  max-width: 100%;
  height: auto;
  min-height: 24px;
  border-radius: 999px;
  white-space: normal;
  line-height: 1.4;
  font-weight: 600;
}

.ai-analysis-alert-list {
  flex-direction: column;
}

.ai-analysis-alert-list :deep(.el-alert) {
  border-radius: 14px;
}

.ai-analysis-evidence-collapse {
  margin-top: 10px;
  border-radius: 18px;
  overflow: hidden;
  border: 1px solid #dfe7f2;
  background: rgba(255, 255, 255, 0.72);
}

.ai-analysis-evidence-collapse :deep(.el-collapse-item__header) {
  padding: 0 14px;
  height: 42px;
  color: #172133;
  font-weight: 700;
  background: rgba(247, 250, 255, 0.86);
}

.ai-analysis-evidence-collapse :deep(.el-collapse-item__content) {
  padding: 0;
}

.ai-analysis-json {
  margin: 0;
  padding: 12px 14px;
  max-height: 260px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  overflow-wrap: anywhere;
  color: #46556c;
  background: #f8fafc;
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
  line-height: 1.65;
}

@media (max-width: 768px) {
  :deep(.ai-analysis-dialog .el-dialog__header) {
    padding: 12px 16px 0;
  }

  :deep(.ai-analysis-dialog .el-dialog__body) {
    padding: 10px 16px 12px;
  }

  :deep(.ai-analysis-dialog .el-dialog__footer) {
    padding: 0 16px 14px;
  }

  .ai-analysis-hero {
    flex-direction: column;
  }

  .ai-analysis-meta {
    justify-content: flex-start;
    max-width: none;
  }

  .ai-analysis-text {
    max-height: 380px;
  }

  .ai-analysis-step-grid {
    grid-template-columns: repeat(11, minmax(82px, 1fr));
  }

  .ai-analysis-summary-strip {
    grid-template-columns: 1fr;
  }

  .ai-analysis-event-item {
    grid-template-columns: minmax(0, 1fr);
    gap: 4px;
  }

  .ai-analysis-event-item time {
    grid-column: 1;
  }
}

/* 自定义图标样式 */
.custom-icon {
  width: 16px;
  height: 16px;
  margin-right: 4px;
  vertical-align: middle;
  filter: brightness(0) invert(1);
}

/* 圆形按钮中的小图标样式 */
.custom-icon-small {
  width: 14px;
  height: 14px;
  vertical-align: middle;
  filter: brightness(0) invert(1);
}

/* 监控弹框样式 */
.monitoring-dialog .el-dialog__body {
  padding: 20px;
}

.monitoring-container {
  min-height: 300px;
}

.metric-card {
  height: 140px;
  margin-bottom: 16px;
}

.metric-card :deep(.el-card__body) {
  padding: 16px;
  height: 100%;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
}

.metric-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.metric-title {
  font-size: 14px;
  font-weight: 500;
  color: #333;
}

.metric-value {
  font-size: 16px;
  font-weight: bold;
  color: #409EFF;
}

.metric-details {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: #666;
  margin-top: 8px;
}

.network-metrics {
  display: flex;
  flex-direction: column;
  justify-content: center;
  height: 60px;
}

.network-item {
  display: flex;
  justify-content: space-between;
  margin-bottom: 8px;
}

.network-label {
  font-size: 14px;
  color: #666;
}

.network-value {
  font-size: 14px;
  font-weight: 500;
  color: #333;
}

/* 容器监控表格样式 */
.container-metric {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.metric-text {
  font-size: 12px;
  color: #606266;
  margin-bottom: 2px;
}刚才修改

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center
  ;
}

.card-header span {
  font-weight: 500;
}

/* 回滚确认对话框样式 */
.rollback-confirmation {
  padding: 10px 0;
}

.rollback-confirmation .el-descriptions {
  margin-top: 10px;
}

.rollback-confirmation .el-alert {
  border-radius: 8px;
}

.pod-card {
  padding: 0;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.76);
  border: 1px solid rgba(223, 231, 242, 0.72);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
  backdrop-filter: none;
}

.pod-card::before {
  display: none;
}

.pod-card :deep(.el-card__header) {
  padding: 14px 16px 12px;
  background: linear-gradient(90deg, #fef7ef 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.82);
}

.pod-card :deep(.el-card__body) {
  padding: 8px;
}

.page-hero {
  align-items: flex-start;
  gap: 14px;
  margin-bottom: 0;
  padding: 6px 2px 2px;
  border-bottom: none;
}

.header-left {
  flex: 1;
  min-width: 0;
  align-items: flex-start;
  gap: 12px;
}

.header-left :deep(.el-button) {
  flex: 0 0 auto;
}

.page-title {
  display: block;
  margin: 18px 0 0;
  color: var(--aiops-text-primary);
  font-size: 24px;
  font-weight: 700;
  line-height: 1.2;
  background: none;
  -webkit-text-fill-color: currentColor;
}

.page-title::before {
  content: 'Kubernetes / Pod Detail';
  display: block;
  margin-bottom: 6px;
  color: var(--aiops-brand);
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0;
}

.page-title::after {
  content: '状态、事件、日志与 AI 诊断';
  display: block;
  margin-top: 8px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  font-weight: 400;
  line-height: 1.6;
}

.header-actions {
  gap: 12px;
  padding-top: 18px;
}

.header-actions :deep(.el-button) {
  min-width: 92px;
  height: 32px;
  border-radius: 12px;
  font-weight: 600;
  box-shadow: none;
}

.header-actions :deep(.back-button) {
  border: none !important;
  background: linear-gradient(180deg, #7da8fb 0%, #5a93f6 100%) !important;
  color: #ffffff !important;
  box-shadow: 0 10px 18px rgba(76, 139, 245, 0.18);
}

.header-actions :deep(.back-button:hover),
.header-actions :deep(.back-button:focus) {
  background: linear-gradient(180deg, #89b1fc 0%, #6a9cf7 100%) !important;
  color: #ffffff !important;
}

.header-actions :deep(.refresh-button) {
  border: none !important;
  background: linear-gradient(180deg, #57c486 0%, #3fb97f 100%) !important;
  color: #ffffff !important;
  box-shadow: 0 10px 18px rgba(63, 185, 127, 0.18);
}

.header-actions :deep(.refresh-button:hover),
.header-actions :deep(.refresh-button:focus) {
  background: linear-gradient(180deg, #63cf92 0%, #4bc189 100%) !important;
  color: #ffffff !important;
}

.header-actions :deep(.yaml-button) {
  border: none !important;
  background: linear-gradient(180deg, #f5be58 0%, #efaa35 100%) !important;
  color: #ffffff !important;
  box-shadow: 0 10px 18px rgba(243, 178, 75, 0.18);
}

.header-actions :deep(.yaml-button:hover),
.header-actions :deep(.yaml-button:focus) {
  background: linear-gradient(180deg, #f6c76c 0%, #f0b445 100%) !important;
  color: #ffffff !important;
}

.header-actions :deep(.ai-button) {
  border: none !important;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%) !important;
  color: #ffffff !important;
  box-shadow: 0 10px 18px rgba(76, 139, 245, 0.18);
}

.header-actions :deep(.ai-button:hover),
.header-actions :deep(.ai-button:focus) {
  background: linear-gradient(180deg, #74a8fb 0%, #6299f6 100%) !important;
  color: #ffffff !important;
}

.info-section {
  margin-bottom: 10px;
}

.info-card {
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.82);
  box-shadow: none;
}

.info-card :deep(.el-card__header) {
  padding: 13px 18px 11px;
}

.resource-usage-card :deep(.el-card__body) {
  padding: 18px;
}

.resource-history-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.resource-history-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--aiops-text-primary);
  font-size: 14px;
  font-weight: 700;
}

.resource-history-title .el-icon {
  color: #1f2d3d;
}

.resource-history-actions {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  flex-shrink: 0;
}

.resource-history-range {
  width: 116px;
}

.resource-history-range :deep(.el-input__wrapper) {
  min-height: 28px;
  border-radius: 10px;
  box-shadow: none;
}

.resource-usage-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  min-height: 226px;
}

.resource-history-panel {
  min-width: 0;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.72);
  padding: 14px 12px 10px;
}

.resource-history-panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-height: 24px;
  padding: 0 2px;
}

.resource-history-panel-title {
  color: var(--aiops-text-secondary);
  font-size: 13px;
  font-weight: 700;
  min-width: 0;
}

.resource-history-panel-tools {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex-shrink: 0;
}

.resource-history-expand {
  width: 24px;
  height: 24px;
  padding: 0;
  color: #d97706;
  font-weight: 800;
}

.resource-history-expand :deep(.el-icon) {
  font-size: 16px;
  font-weight: 800;
}

.resource-history-expand :deep(svg) {
  stroke-width: 3;
}

.resource-history-expand:hover {
  color: #b45309;
  background: rgba(245, 158, 11, 0.12);
}

.resource-history-panel-value {
  color: var(--aiops-text-primary);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
}

.resource-history-chart {
  width: 100%;
  height: 170px;
  margin-top: 8px;
}

.history-chart-dialog :deep(.el-dialog) {
  max-width: 1600px;
  border-radius: 8px;
}

.history-chart-dialog :deep(.el-dialog__header) {
  padding: 14px 18px 10px;
  margin-right: 0;
  border-bottom: 1px solid rgba(226, 232, 240, 0.9);
}

.history-chart-dialog :deep(.el-dialog__title) {
  color: #0f172a;
  font-size: 15px;
  font-weight: 700;
}

.history-chart-dialog :deep(.el-dialog__body) {
  padding: 12px 18px 18px;
}

.history-chart-dialog-body {
  min-height: 620px;
}

.history-chart-dialog-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 10px;
}

.history-chart-dialog-tags {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.history-chart-dialog-range {
  width: 126px;
  flex-shrink: 0;
}

.history-chart-dialog-range :deep(.el-input__wrapper) {
  min-height: 28px;
  border-radius: 8px;
  box-shadow: none;
}

.history-chart-expanded {
  width: 100%;
  height: 520px;
}

.history-chart-stat-table {
  margin-top: 12px;
  border: 1px solid rgba(226, 232, 240, 0.95);
  border-radius: 6px;
  overflow: hidden;
  font-size: 12px;
}

.history-chart-stat-head,
.history-chart-stat-row {
  display: grid;
  grid-template-columns: minmax(260px, 1fr) 96px 96px 108px;
  align-items: center;
  column-gap: 12px;
  min-height: 28px;
  padding: 0 12px;
}

.history-chart-stat-head {
  color: #2563eb;
  font-weight: 700;
  background: #f8fafc;
  border-bottom: 1px solid rgba(226, 232, 240, 0.95);
}

.history-chart-stat-row {
  color: #334155;
  background: #fff;
}

.history-chart-stat-row:nth-child(odd) {
  background: #f8fafc;
}

.history-chart-stat-name {
  display: inline-flex;
  align-items: center;
  min-width: 0;
  gap: 8px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.history-chart-stat-name i {
  display: inline-block;
  width: 14px;
  height: 3px;
  flex-shrink: 0;
  border-radius: 999px;
}

@media (max-width: 1280px) {
  .resource-usage-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

.labels-display :deep(.el-tag) {
  margin: 2px;
  border-radius: 999px;
  border-color: rgba(207, 217, 230, 0.92);
  background: rgba(76, 139, 245, 0.08);
  color: var(--aiops-brand);
  font-weight: 600;
}

.pod-name-container {
  transition: color 0.2s ease;
}

.pod-name-container:hover {
  color: var(--aiops-brand);
  transform: none;
}

.pod-name {
  color: var(--aiops-brand);
  font-size: 13px;
  font-weight: 700;
}

.image-link {
  color: var(--aiops-brand);
}

.yaml-search-toolbar {
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 16px;
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.82), rgba(247, 250, 255, 0.76));
}

.logs-tab-content {
  padding: 0;
}

.logs-controls {
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.82), rgba(247, 250, 255, 0.76));
  border-radius: 16px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
}

.logs-display {
  margin-top: 12px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 18px;
}

.operation-buttons-container :deep(.operation-text.ai) {
  color: var(--aiops-brand);
}

.operation-buttons-container :deep(.operation-text.yaml),
.operation-buttons-container :deep(.operation-text.terminal),
.operation-buttons-container :deep(.operation-text.rollback) {
  color: var(--aiops-warning);
}

.operation-buttons-container :deep(.operation-text.delete) {
  color: var(--aiops-danger);
}

.monitoring-detail-card {
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 20px;
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.82), rgba(247, 250, 255, 0.78));
}

.monitoring-detail-card :deep(.el-card__header) {
  padding: 14px 18px 12px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.82);
  background: linear-gradient(90deg, rgba(255, 249, 240, 0.92), rgba(237, 242, 253, 0.72));
}

.monitoring-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.metric-card {
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 20px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.84), rgba(247, 250, 255, 0.82));
}

.metric-title {
  font-weight: 600;
  color: var(--aiops-text-secondary);
}

.metric-value {
  font-weight: 700;
  color: var(--aiops-brand);
}

.metric-details {
  color: var(--aiops-text-muted);
}

.network-label {
  color: var(--aiops-text-secondary);
}

.network-value {
  font-weight: 600;
  color: var(--aiops-text-primary);
}

.metric-text {
  color: var(--aiops-text-secondary);
}

@media (max-width: 768px) {
  .k8s-pod-management {
    --pod-page-height-offset: 131px;
    padding: 4px;
  }

  .pod-card {
    border-radius: 16px;
  }

  .pod-card :deep(.el-card__header) {
    padding: 12px 12px 10px;
  }

  .pod-card :deep(.el-card__body) {
    padding: 8px;
  }

  .page-hero {
    flex-direction: column;
    align-items: stretch;
  }

  .header-left {
    align-items: stretch;
  }

  .page-title {
    margin-top: 12px;
    font-size: 22px;
  }

  .header-actions {
    width: 100%;
    flex-wrap: wrap;
    padding-top: 0;
  }

  .detail-table :deep(.el-table__body-wrapper) {
    max-height: 360px;
  }

  .header-actions :deep(.el-button),
  .header-left :deep(.el-button) {
    width: 100%;
  }

  .resource-usage-grid {
    grid-template-columns: 1fr;
  }
}

.k8s-pod-management,
.pod-card,
.pod-card :deep(.el-card__body),
.detail-tabs,
.detail-tabs :deep(.el-tabs__content),
.detail-tabs :deep(.el-tab-pane) {
  min-width: 0;
  max-width: 100%;
  overflow-x: hidden;
}

.k8s-pod-management,
.pod-card {
  width: 100%;
  box-sizing: border-box;
}

.detail-tabs :deep(.el-tabs__header) {
  margin: 0 0 16px;
  padding: 0 18px;
  border: 0;
  border-radius: 0;
  border-top: 1px solid rgba(223, 231, 242, 0.88);
  border-bottom: 1px solid rgba(223, 231, 242, 0.88);
  background: rgba(247, 250, 255, 0.56);
  box-shadow: none;
}

.detail-tabs :deep(.el-tabs__nav-wrap) {
  padding: 0;
  overflow: visible;
}

.detail-tabs :deep(.el-tabs__nav-wrap::after) {
  display: block;
  height: 1px;
  background-color: rgba(223, 231, 242, 0.95);
}

.detail-tabs :deep(.el-tabs__active-bar) {
  height: 2px;
  border-radius: 999px;
  background-color: var(--aiops-brand);
}

.detail-tabs :deep(.el-tabs__nav-scroll) {
  overflow: visible;
}

.detail-tabs :deep(.el-tabs__nav) {
  display: inline-flex;
  gap: 22px;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.detail-tabs :deep(.el-tabs__item) {
  height: 48px;
  padding: 0 !important;
  border: 0 !important;
  border-radius: 0;
  color: var(--aiops-text-primary);
  font-weight: 600;
  line-height: 48px;
  transition: color 0.18s ease;
}

.detail-tabs :deep(.el-tabs__item:hover),
.detail-tabs :deep(.el-tabs__item.is-active) {
  color: var(--aiops-brand);
}

.detail-tabs :deep(.el-tabs__item.is-active) {
  background: transparent;
  box-shadow: none;
}

.detail-tabs :deep(.el-tabs__content) {
  height: auto !important;
  min-height: 0 !important;
  margin-top: 8px;
  padding: 0;
  overflow-x: hidden !important;
  overflow-y: visible !important;
  border: 0;
  border-radius: 0;
  background: transparent;
}

.detail-tabs :deep(.el-tab-pane) {
  min-height: 0 !important;
  overflow-x: hidden !important;
  overflow-y: visible !important;
}

.detail-table {
  flex: none;
  width: 100%;
  max-width: 100%;
  border: 1px solid rgba(223, 231, 242, 0.88);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.92);
  overflow: hidden;
  box-shadow: none;
}

.detail-table :deep(.el-table__inner-wrapper),
.detail-table :deep(.el-table__header-wrapper),
.detail-table :deep(.el-table__body-wrapper),
.detail-table :deep(.el-scrollbar),
.detail-table :deep(.el-scrollbar__view) {
  max-width: 100%;
  overflow-x: hidden !important;
}

.detail-table :deep(.el-table__body-wrapper) {
  max-height: clamp(220px, calc(100vh - 390px), 430px);
  overflow-x: hidden !important;
  overflow-y: auto;
}

.detail-table :deep(.el-scrollbar__wrap) {
  max-height: clamp(220px, calc(100vh - 390px), 430px);
  margin-bottom: 0 !important;
  padding-bottom: 0 !important;
  overflow-x: hidden !important;
  overflow-y: auto !important;
}

.detail-table :deep(.el-scrollbar__bar.is-horizontal) {
  display: none !important;
  height: 0 !important;
  opacity: 0 !important;
  pointer-events: none;
}

.detail-table :deep(.el-scrollbar__bar.is-horizontal .el-scrollbar__thumb) {
  display: none !important;
}

.detail-table :deep(.el-scrollbar__wrap::-webkit-scrollbar) {
  width: 10px;
  height: 0;
}

.detail-tabs :deep(.el-tabs__content::-webkit-scrollbar),
.detail-tabs :deep(.el-tab-pane::-webkit-scrollbar),
.detail-table :deep(.el-table__inner-wrapper::-webkit-scrollbar),
.detail-table :deep(.el-table__body-wrapper::-webkit-scrollbar),
.detail-table :deep(.el-scrollbar__wrap::-webkit-scrollbar) {
  width: 10px;
  height: 0 !important;
}

.detail-tabs :deep(.el-tabs__content::-webkit-scrollbar:horizontal),
.detail-tabs :deep(.el-tab-pane::-webkit-scrollbar:horizontal),
.detail-table :deep(.el-table__inner-wrapper::-webkit-scrollbar:horizontal),
.detail-table :deep(.el-table__body-wrapper::-webkit-scrollbar:horizontal),
.detail-table :deep(.el-scrollbar__wrap::-webkit-scrollbar:horizontal) {
  display: none;
  height: 0 !important;
}

.detail-table :deep(.el-table__body-wrapper::-webkit-scrollbar) {
  width: 10px;
  height: 0;
}

.detail-table :deep(.el-table__body-wrapper::-webkit-scrollbar-track) {
  background: rgba(237, 242, 250, 0.72);
  border-radius: 999px;
}

.detail-table :deep(.el-table__body-wrapper::-webkit-scrollbar-thumb) {
  background: rgba(165, 177, 195, 0.86);
  border: 2px solid rgba(237, 242, 250, 0.86);
  border-radius: 999px;
}

.detail-table :deep(.el-table__body-wrapper::-webkit-scrollbar-thumb:hover) {
  background: rgba(134, 148, 170, 0.92);
}
</style>
