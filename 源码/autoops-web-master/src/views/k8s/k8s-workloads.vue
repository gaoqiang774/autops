<script setup>
import { ref, reactive, onMounted, onUnmounted, computed, watch, defineComponent, h } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import yaml from 'js-yaml'
import {
  Search,
  Refresh,
  Plus,
  Warning,
  Monitor,
  DocumentCopy,
  Connection,
  MagicStick
} from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'
import aiopsApi from '@/api/aiops'
import {
  getStoredClusterId,
  saveStoredClusterId,
  resolvePreferredClusterId
} from '@/utils/k8sClusterSelection'
import {
  DEFAULT_K8S_NAMESPACE,
  getStoredNamespace
} from '@/utils/k8sNamespaceSelection'
import { getYamlContentFromApiResponse } from './utils/yaml'
import { checkAuthority } from '@/utils/authority'
import storage from '@/utils/storage'

const CLUSTER_SELECTION_SOURCE = 'k8s-workloads'

// Import modular components
import ClusterSelector from './pods/ClusterSelector.vue'
import NamespaceSelector from './pods/NamespaceSelector.vue'
import CreatePodDialog from './pods/CreatePodDialog.vue'
import { openYamlDialog } from './services/yamlDialog'

const router = useRouter()
const route = useRoute()
const ALL_WORKLOAD_TYPE = 'all'
const PODS_TYPE = 'pods'
const WORKLOAD_SCOPE_ALL = 'all'
const WORKLOAD_SCOPE_ABNORMAL = 'abnormal'
const WORKLOAD_TAB_QUERY_KEY = 'tab'
const WORKLOAD_TAB_STORAGE_KEY = 'k8s-workloads-active-tab'
const WORKLOAD_SCOPE_STORAGE_KEY = 'k8s-workloads-scope'
const initialClusterId = getStoredClusterId()

const loading = ref(false)

// 工作负载类型选项
const workloadTypeOptions = [
  { label: '全部', value: ALL_WORKLOAD_TYPE },
  { label: 'Deployment', value: 'deployments' },
  { label: 'StatefulSet', value: 'statefulsets' },
  { label: 'DaemonSet', value: 'daemonsets' },
  { label: 'Job', value: 'jobs' },
  { label: 'CronJob', value: 'cronjobs' },
  { label: 'Pods', value: PODS_TYPE }
]

const validWorkloadTypes = new Set(workloadTypeOptions.map(option => option.value))

const getRouteTab = () => {
  const routeTab = route.query[WORKLOAD_TAB_QUERY_KEY]
  return Array.isArray(routeTab) ? routeTab[0] : routeTab
}

const normalizeWorkloadType = (type) =>
  validWorkloadTypes.has(type) ? type : ALL_WORKLOAD_TYPE

const getStoredTab = () => {
  if (typeof window === 'undefined') return ''
  return window.sessionStorage.getItem(WORKLOAD_TAB_STORAGE_KEY) || ''
}

const getInitialWorkloadType = () =>
  normalizeWorkloadType(getRouteTab() || getStoredTab())

const getStoredScope = () => {
  if (typeof window === 'undefined') return WORKLOAD_SCOPE_ALL

  const storedScope = window.sessionStorage.getItem(WORKLOAD_SCOPE_STORAGE_KEY)
  return [WORKLOAD_SCOPE_ALL, WORKLOAD_SCOPE_ABNORMAL].includes(storedScope)
    ? storedScope
    : WORKLOAD_SCOPE_ALL
}

const persistScope = (scope) => {
  if (typeof window === 'undefined') return

  if ([WORKLOAD_SCOPE_ALL, WORKLOAD_SCOPE_ABNORMAL].includes(scope)) {
    window.sessionStorage.setItem(WORKLOAD_SCOPE_STORAGE_KEY, scope)
  }
}

const initialWorkloadType = getInitialWorkloadType()
const activeTab = ref(initialWorkloadType)
const queryParams = reactive({
  name: '',
  type: initialWorkloadType,
  namespace: getStoredNamespace(initialClusterId) || DEFAULT_K8S_NAMESPACE,
  scope: getStoredScope(),
  nodeName: ''
})

const persistTab = (tabName) => {
  if (typeof window === 'undefined') return

  const normalizedTab = normalizeWorkloadType(tabName)

  if (normalizedTab === ALL_WORKLOAD_TYPE) {
    window.sessionStorage.removeItem(WORKLOAD_TAB_STORAGE_KEY)
    return
  }

  window.sessionStorage.setItem(WORKLOAD_TAB_STORAGE_KEY, normalizedTab)
}

const syncTabState = (tabName) => {
  const normalizedTab = normalizeWorkloadType(tabName)
  const currentRouteTab = getRouteTab()
  const nextQuery = { ...route.query }

  persistTab(normalizedTab)

  if (normalizedTab === ALL_WORKLOAD_TYPE) {
    if (!currentRouteTab) return
    delete nextQuery[WORKLOAD_TAB_QUERY_KEY]
  } else {
    if (currentRouteTab === normalizedTab) return
    nextQuery[WORKLOAD_TAB_QUERY_KEY] = normalizedTab
  }

  router.replace({
    path: route.path,
    query: nextQuery
  })
}

const restoreTabState = () => {
  const routeTab = getRouteTab()
  const restoredTab = normalizeWorkloadType(routeTab || getStoredTab())

  activeTab.value = restoredTab
  queryParams.type = restoredTab
  syncTabState(restoredTab)
}

const isPodsTab = computed(() => getRequestWorkloadType() === PODS_TYPE)
const isAbnormalMode = computed(() => queryParams.scope === WORKLOAD_SCOPE_ABNORMAL)
const workloadScopeOptions = computed(() => (
  isPodsTab.value
    ? [
        { label: '全部 Pod', value: WORKLOAD_SCOPE_ALL },
        { label: '异常 Pod', value: WORKLOAD_SCOPE_ABNORMAL }
      ]
    : [
        { label: '全部工作负载', value: WORKLOAD_SCOPE_ALL },
        { label: '仅看异常', value: WORKLOAD_SCOPE_ABNORMAL }
      ]
))
const workloadNameLabel = computed(() => (isPodsTab.value ? 'Pod 名称' : '工作负载名称'))
const workloadNamePlaceholder = computed(() => (
  isPodsTab.value
    ? '请输入 Pod 名称，支持模糊搜索'
    : '请输入工作负载名称，支持模糊搜索'
))
const replicaColumnLabel = computed(() => (isPodsTab.value ? 'Pod 状态' : '容器组数量'))
const workloadTableEmptyText = computed(() => {
  if (isPodsTab.value) {
    if (isAbnormalMode.value) {
      return '当前条件下暂无异常 Pod'
    }

    return queryParams.name?.trim() ? '未匹配到 Pod' : '暂无 Pod 数据'
  }

  if (isAbnormalMode.value) {
    return '当前条件下暂无异常工作负载'
  }

  return queryParams.name?.trim() ? '未匹配到工作负载' : '暂无工作负载数据'
})

const tableData = ref([])
const workloadTableRef = ref(null)
const selectedPods = ref([])
const batchDeleteLoading = ref(false)
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)
const selectedClusterId = ref(initialClusterId)
const clusterList = ref([])
const nodeOptions = ref([])
const nodesLoading = ref(false)
let latestNodeOptionsRequestId = 0
let latestWorkloadListRequestId = 0
// clusterList 已移至 ClusterSelector 组件
// namespaceList 和 namespaceLoading 已移至 NamespaceSelector 组件

// 对话框状态
const scaleDialogVisible = ref(false)
const allImagesDialogVisible = ref(false)
const schedulingDialogVisible = ref(false)
const createPodDialogVisible = ref(false)
const deploymentHistoryDialogVisible = ref(false)
const deploymentHistoryLoading = ref(false)
const rollbackLoading = ref(false)
const aiAnalysisVisible = ref(false)
const aiAnalysisLoading = ref(false)
const selectedAiAnalysisTarget = ref(null)
const aiAnalysisResult = ref(null)
const aiAnalysisTaskId = ref('')
const aiAnalysisTaskStatus = ref('')
const aiAnalysisProgress = ref(0)
const aiAnalysisMessage = ref('')
const aiAnalysisEvents = ref([])
const aiAnalysisPrompt = ref('请分析该工作负载或 Pod 是否存在异常重启、事件 Warning、应用日志错误或 CPU、内存飙高问题，并给出下一步容器内实时诊断建议。')
let aiAnalysisEventSource = null
let aiAnalysisPollingTimer = null
const deploymentHistory = ref({
  deploymentName: '',
  namespace: '',
  currentRevision: 0,
  revisionHistoryLimit: 10,
  totalRevisions: 0,
  revisions: []
})

// 当前操作的工作负载或Pod
const currentWorkload = ref({})
const createPodDialogRef = ref(null)

const normalizeAiWorkloadType = type => {
  const normalized = String(type || '').trim().toLowerCase()
  const typeMap = {
    deployment: 'deployment',
    deployments: 'deployment',
    statefulset: 'statefulset',
    statefulsets: 'statefulset',
    daemonset: 'daemonset',
    daemonsets: 'daemonset',
    job: 'job',
    jobs: 'job',
    cronjob: 'cronjob',
    cronjobs: 'cronjob',
    pod: 'pod',
    pods: 'pod'
  }
  return typeMap[normalized] || normalized
}

const aiAnalysisStepDefinitions = [
  { key: 'task_created', title: '任务创建' },
  { key: 'start', title: '开始分析' },
  { key: 'status_check', title: '状态检查' },
  { key: 'log_scan', title: '日志扫描' },
  { key: 'signal_scan', title: '事件与资源分析' },
  { key: 'ai_diagnosis', title: 'AI 诊断' }
]

const createAiAnalysisSteps = () => aiAnalysisStepDefinitions.map(step => ({
  ...step,
  status: 'pending',
  message: ''
}))

const aiTaskStepStatusText = {
  pending: '未开始',
  running: '进行中',
  success: '已完成',
  failed: '失败',
  skipped: '跳过'
}

const getAiTaskStepStatusText = status => aiTaskStepStatusText[status] || status || '-'

const aiAnalysisSteps = ref(createAiAnalysisSteps())

const aiAnalysisTitle = computed(() => {
  const target = selectedAiAnalysisTarget.value
  if (!target?.name) return 'AI分析诊断'
  return `AI分析诊断 - ${target.name}`
})

const aiAnalysisTargetMeta = computed(() => {
  const target = selectedAiAnalysisTarget.value || {}
  const workloadType = normalizeAiWorkloadType(target.type || '')
  const isPod = workloadType === 'pod'
  const cluster = clusterList.value.find(item => String(item.id) === String(selectedClusterId.value))
  return {
    cluster: cluster?.name || selectedClusterId.value || '-',
    clusterId: selectedClusterId.value || '-',
    namespace: getResourceNamespace(target) || queryParams.namespace || '-',
    type: getWorkloadTypeName(workloadType) || 'Workload',
    name: target.name || '-',
    workloadType: isPod ? '-' : (getWorkloadTypeName(workloadType) || workloadType || '-'),
    workloadName: isPod ? '-' : (target.name || '-'),
    selectedPod: isPod ? (target.name || '-') : '-'
  }
})

const aiAnalysisObjectFields = computed(() => {
  const meta = aiAnalysisTargetMeta.value
  return [
    { label: '集群', value: meta.cluster },
    { label: 'namespace', value: meta.namespace },
    { label: 'workloadType', value: meta.workloadType },
    { label: 'workloadName', value: meta.workloadName },
    { label: 'selectedPod', value: meta.selectedPod }
  ].filter(item => item.value && item.value !== '-')
})

const aiAnalysisQuestion = computed(() => {
  const meta = aiAnalysisTargetMeta.value
  const objectType = meta.selectedPod !== '-' ? 'Pod' : '工作负载'
  return `这个${objectType} ${meta.namespace}/${meta.name} 正常吗？`
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

const pickAiResultValue = (...keys) => {
  const result = aiAnalysisResult.value || {}
  const data = result.data || {}
  for (const key of keys) {
    if (result[key] !== undefined && result[key] !== null && result[key] !== '') return result[key]
    if (data[key] !== undefined && data[key] !== null && data[key] !== '') return data[key]
  }
  return ''
}

const normalizeAiReasonText = value => {
  if (value === undefined || value === null) return ''
  if (Array.isArray(value)) {
    return value
      .map(item => normalizeAiReasonText(item))
      .filter(Boolean)
      .join('；')
  }
  if (typeof value === 'object') {
    return normalizeAiReasonText(value.message || value.reason || value.title || value.summary || JSON.stringify(value))
  }
  return String(value || '')
    .replace(/\r/g, '')
    .replace(/(?:\.\.\.)?<truncated>/gi, '...')
    .replace(/^#+\s*/gm, '')
    .replace(/\*\*/g, '')
    .replace(/`/g, '')
    .trim()
}

const normalizeAiList = value => {
  if (!value) return []
  const source = Array.isArray(value) ? value : String(value).split(/\n|；|;/)
  return source
    .map(item => normalizeAiReasonText(item).replace(/^[-*\d.、\s]+/, '').trim())
    .filter(Boolean)
}

const AI_RICH_TEXT_PATTERN = /(`[^`\n]+`|(?:kubectl|curl)\s+[^，。；\n]+|\/[A-Za-z0-9._~:/-]+|(?:[A-Za-z][A-Za-z0-9_.-]*\/)+[A-Za-z0-9_.-]+|[A-Za-z][A-Za-z0-9_.-]*-[A-Za-z0-9_.-]+|[A-Za-z][A-Za-z0-9_]*\d+[A-Za-z0-9_]*|\b(?:Deployment|StatefulSet|DaemonSet|CronJob|Job|Pod|Service|Ingress|ConfigMap|Secret|PVC|PV|Node|Namespace|Pending|Running|Ready|ContainerCreating|CrashLoopBackOff|OOMKilled|ImagePullBackOff|Warning|volumeMounts|volumes|targetPort|nodePort)\b|\b\d+(?:\.\d+)*(?:%|ms|Mi|Gi)?\b)/g
const AI_WARNING_TERMS = /^(?:Pending|ContainerCreating|CrashLoopBackOff|OOMKilled|ImagePullBackOff|Warning)$/i
const AI_SUCCESS_TERMS = /^(?:Running|Ready|Succeeded)$/i

const tokenizeAiRichText = value => {
  const text = String(value || '')
  const tokens = []
  let cursor = 0
  let match
  AI_RICH_TEXT_PATTERN.lastIndex = 0
  while ((match = AI_RICH_TEXT_PATTERN.exec(text)) !== null) {
    if (match.index > cursor) tokens.push({ type: 'text', text: text.slice(cursor, match.index) })
    const raw = match[0]
    const clean = raw.startsWith('`') && raw.endsWith('`') ? raw.slice(1, -1) : raw
    let type = 'keyword'
    if (AI_WARNING_TERMS.test(clean)) type = 'warning'
    else if (AI_SUCCESS_TERMS.test(clean)) type = 'success'
    else if (/^\d/.test(clean)) type = 'number'
    else if (raw.startsWith('`') || /^(?:kubectl|curl)\s/i.test(clean) || clean.includes('/') || clean.includes('-')) type = 'code'
    tokens.push({ type, text: clean })
    cursor = match.index + raw.length
  }
  if (cursor < text.length) tokens.push({ type: 'text', text: text.slice(cursor) })
  return tokens
}

const AiRichText = defineComponent({
  name: 'AiRichText',
  props: {
    text: { type: String, default: '' }
  },
  setup(props) {
    return () => h('span', { class: 'ai-rich-text' }, tokenizeAiRichText(props.text).map((token, index) => {
      const tag = token.type === 'code' ? 'code' : 'span'
      return h(tag, { class: `ai-rich-token is-${token.type}`, key: `${index}-${token.text}` }, token.text)
    }))
  }
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

const isAiAnalysisFinished = computed(() => {
  return aiAnalysisTaskStatus.value === 'succeeded' && !!aiAnalysisResult.value
})

const aiAnalysisStatusMeta = computed(() => {
  if (aiAnalysisTaskStatus.value === 'failed') return { label: 'AI 分析失败', type: 'danger' }
  if (!isAiAnalysisFinished.value) return { label: aiAnalysisLoading.value ? 'AI 分析中' : '等待分析', type: 'primary' }

  const status = String(
    aiAnalysisResult.value?.podStatus ||
    aiAnalysisResult.value?.pod_status ||
    aiAnalysisResult.value?.status ||
    ''
  ).toLowerCase()
  if (status === 'normal' || status === '正常') return { label: '分析结果正常', type: 'success' }
  if (status === 'abnormal' || status === '异常') return { label: '分析结果异常', type: 'danger' }
  return { label: '证据不足或状态未知', type: 'info' }
})

const aiAnalysisProgressStatus = computed(() => {
  if (aiAnalysisTaskStatus.value === 'failed') return 'exception'
  if (aiAnalysisTaskStatus.value === 'succeeded' || aiAnalysisProgress.value >= 100) return 'success'
  return undefined
})

const getAiAnalysisSectionLines = (...headings) => {
  const lines = String(aiAnalysisText.value || '').replace(/\r/g, '').split('\n')
  const normalizedHeadings = headings.map(item => String(item).trim().toLowerCase())
  const start = lines.findIndex(line => {
    const heading = line.replace(/^\s*#{1,6}\s*/, '').trim().toLowerCase()
    return /^\s*#{1,6}\s+/.test(line) && normalizedHeadings.some(item => heading.includes(item))
  })
  if (start < 0) return []

  const result = []
  for (let index = start + 1; index < lines.length; index += 1) {
    const line = lines[index].trim()
    if (/^#{1,6}\s+/.test(line)) break
    if (!line) continue
    result.push(line.replace(/^[-*]\s+/, '').replace(/^\d+[.)、]\s*/, '').trim())
  }
  return result.filter(Boolean)
}

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

  const summaryLine = lines.find(line => /事件摘要|事件总结|摘要|结论/.test(line) && /[:：]/.test(line)) || ''
  const severityLine = lines.find(line => /严重性|严重级别|severity/.test(line)) || ''
  const rootCauseLine = lines.find(line => /根本原因|根因|可能的根本原因|root cause/i.test(line)) || ''

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

const shouldShowAiAnalysisResult = computed(() => isAiAnalysisFinished.value)

const aiFinalConclusion = computed(() => normalizeAiReasonText(
  pickAiResultValue('conclusion', 'finalConclusion', 'final_conclusion')
  || getAiAnalysisSectionLines('总结结论', '最终结论')[0]
  || aiAnalysisSummaryMeta.value.summary
  || aiAnalysisText.value
).replace(/^(?:总结结论|最终结论)[:：]\s*/, ''))

const aiFinalSummary = computed(() => normalizeAiReasonText(
  pickAiResultValue('summary', 'diagnosisSummary', 'diagnosis_summary')
))

const aiFinalRootCause = computed(() => normalizeAiReasonText(
  pickAiResultValue('rootCause', 'root_cause', 'cause')
  || aiAnalysisSummaryMeta.value.rootCause
  || getAiAnalysisSectionLines('问题原因', '根本原因', '可能的根本原因')[0]
))

const aiFinalSeverity = computed(() => normalizeAiReasonText(
  pickAiResultValue('severity', 'level')
  || aiAnalysisSummaryMeta.value.severity
))

const aiFinalRemediation = computed(() => normalizeAiList(
  pickAiResultValue('remediation', 'remediations', 'suggestions', 'recommendations', 'nextChecks', 'next_checks')
  || getAiAnalysisSectionLines('解决方案', '解决办法', '处理建议')
))

const aiLogFindings = computed(() => {
  const findings = aiAnalysisEvidence.value?.logFindings || aiAnalysisEvidence.value?.log_findings || []
  if (!Array.isArray(findings)) return []
  return findings.map(item => ({
    kind: item?.kind || 'LOG',
    level: item?.level || '',
    lineNo: item?.lineNo || item?.line_no || '',
    timestamp: item?.timestamp || '',
    message: normalizeAiReasonText(item?.message || item?.summary || item),
    context: normalizeAiList(item?.context)
  })).filter(item => item.message)
})

const aiRelatedServices = computed(() => {
  const services = aiAnalysisEvidence.value?.services || []
  return Array.isArray(services) ? services : []
})

const aiIngressRoutes = computed(() => {
  const routes = aiAnalysisEvidence.value?.ingressRoutes || aiAnalysisEvidence.value?.ingress_routes || []
  return Array.isArray(routes) ? routes : []
})

const aiAnalysisManifests = computed(() => {
  const manifests = aiAnalysisEvidence.value?.manifests || []
  if (!Array.isArray(manifests)) return []
  return manifests.map((item, index) => ({
    kind: item?.kind || 'Manifest',
    namespace: item?.namespace || aiAnalysisTargetMeta.value.namespace,
    name: item?.name || `config-${index + 1}`,
    yaml: String(item?.yaml || '').trim()
  })).filter(item => item.yaml)
})

const aiManifestSummary = computed(() => {
  const counts = new Map()
  aiAnalysisManifests.value.forEach(item => counts.set(item.kind, (counts.get(item.kind) || 0) + 1))
  return [...counts.entries()].map(([kind, count]) => `${kind} ${count}`).join('、')
})

const aiManifestCollectionErrors = computed(() => {
  const errors = aiAnalysisEvidence.value?.collectionErrors || aiAnalysisEvidence.value?.collection_errors || []
  if (!Array.isArray(errors)) return []
  return errors
    .map(item => normalizeAiReasonText(item))
    .filter(item => /manifest|network routes|service|ingress/i.test(item))
})

const isDebugPortName = value => /debug|jdwp/i.test(String(value || ''))
const isHttpPortName = value => /^(http|https|web|tomcat)(-|$)/i.test(String(value || ''))

const aiPortDiagnosis = computed(() => {
  const hasJdwpFinding = aiLogFindings.value.some(item => item.kind === 'HTTP_TO_JDWP_PORT' || /JDWP-Handshake|Debugger failed to attach/i.test(item.message))
  if (!hasJdwpFinding) return null

  let service = null
  let debugPort = null
  let httpPort = null
  for (const item of aiRelatedServices.value) {
    const ports = Array.isArray(item.ports) ? item.ports : []
    const matchedDebug = ports.find(port => isDebugPortName(port.name) || isDebugPortName(port.targetPort || port.target_port))
    const matchedHttp = ports.find(port => isHttpPortName(port.name)) || ports.find(port => Number(port.port) === 8080 || Number(port.targetPort || port.target_port) === 8080)
    if (matchedDebug || matchedHttp) {
      service = item
      debugPort = matchedDebug || debugPort
      httpPort = matchedHttp || httpPort
      if (debugPort && httpPort) break
    }
  }

  const route = aiIngressRoutes.value.find(item => {
    if (!service || item.serviceName !== service.name) return false
    const selectedPort = String(item.servicePort || item.service_port || '')
    return selectedPort === String(debugPort?.name || '') || selectedPort === String(debugPort?.port || '')
  })
  const debugValue = debugPort?.targetPort || debugPort?.target_port || debugPort?.port || ''
  const httpValue = httpPort?.targetPort || httpPort?.target_port || httpPort?.port || ''

  return {
    confirmed: !!route,
    service,
    route,
    debugPort,
    httpPort,
    debugValue,
    httpValue,
    title: debugValue && httpValue
      ? `HTTP 请求被转发到了 JDWP 调试端口 ${debugValue}，而不是应用端口 ${httpValue}`
      : 'HTTP 请求到达了 JVM JDWP 调试端口，而不是应用 HTTP 端口',
    explanation: 'JDWP 收到了 HTTP 请求，协议握手必然失败，Ingress 或网关无法获得合法的 HTTP 响应，因此会向客户端返回 502。'
  }
})

const aiProblemDescription = computed(() => normalizeAiReasonText(
  aiPortDiagnosis.value?.title
  || aiFinalSummary.value
  || getAiAnalysisSectionLines('问题描述', '事件总结')[0]
  || aiFinalConclusion.value
  || '未返回明确的问题描述'
))

const aiProblemCause = computed(() => normalizeAiReasonText(
  aiFinalRootCause.value
  || getAiAnalysisSectionLines('问题原因', '根本原因', '可能的根本原因')[0]
  || aiPortDiagnosis.value?.explanation
  || '当前证据不足以确认明确原因'
))

const aiSummaryConclusion = computed(() => normalizeAiReasonText(
  aiFinalConclusion.value
  || getAiAnalysisSectionLines('总结结论', '最终结论')[0]
  || aiFinalSummary.value
  || aiProblemDescription.value
))

const aiSummaryConclusionLines = computed(() => {
  const matches = aiSummaryConclusion.value.match(/[^。]+。?/g) || []
  return matches.map(item => item.trim()).filter(Boolean)
})

const formatServicePortsYaml = service => {
  if (!service || !Array.isArray(service.ports) || !service.ports.length) return ''
  const lines = ['ports:']
  service.ports.forEach(port => {
    lines.push(`  - name: ${port.name || 'unnamed'}`)
    lines.push(`    port: ${port.port}`)
    lines.push(`    targetPort: ${port.targetPort || port.target_port || port.port}`)
    if (port.nodePort || port.node_port) lines.push(`    nodePort: ${port.nodePort || port.node_port}`)
  })
  return lines.join('\n')
}

const aiCurrentServiceYaml = computed(() => formatServicePortsYaml(aiPortDiagnosis.value?.service))

const aiRemainingServicePorts = computed(() => {
  const ports = aiPortDiagnosis.value?.service?.ports || []
  return ports.filter(port => !isDebugPortName(port.name) && !isDebugPortName(port.targetPort || port.target_port))
})

const aiServiceFixYaml = computed(() => {
  const service = aiPortDiagnosis.value?.service
  if (!service || !aiRemainingServicePorts.value.length) return ''
  return formatServicePortsYaml({ ...service, ports: aiRemainingServicePorts.value })
})

const aiIngressFixYaml = computed(() => {
  const diagnosis = aiPortDiagnosis.value
  if (!diagnosis?.service || !diagnosis?.httpPort) return ''
  const httpPort = diagnosis.httpPort
  const portKey = httpPort.name ? 'name' : 'number'
  const portValue = httpPort.name || httpPort.port
  return [
    'backend:',
    '  service:',
    `    name: ${diagnosis.service.name}`,
    '    port:',
    `      ${portKey}: ${portValue}`
  ].join('\n')
})

const aiIngressFixAlternative = computed(() => {
  const httpPort = aiPortDiagnosis.value?.httpPort
  if (!httpPort?.name || !httpPort?.port) return ''
  return `也可以将 service.port 显式配置为 number: ${httpPort.port}。`
})

const formatAiServiceExposure = port => {
  const name = String(port?.name || '').toLowerCase()
  let label = port?.name || '端口'
  if (isHttpPortName(name)) label = name === 'https' ? 'HTTPS' : 'HTTP'
  if (name.includes('dubbo')) label = 'Dubbo'
  const targetPort = port?.targetPort || port?.target_port || port?.port
  const nodePort = port?.nodePort || port?.node_port
  return `${label}：${port?.port} -> Pod ${targetPort}${nodePort ? `（NodePort ${nodePort}）` : ''}`
}

const aiServiceExposureLines = computed(() => aiRemainingServicePorts.value.map(formatAiServiceExposure))

const aiObservedRequestPath = computed(() => {
  for (const finding of aiLogFindings.value) {
    const matched = finding.message.match(/received\s+>(?:GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+([^\s<]+)/i)
    if (matched?.[1]) return matched[1]
  }
  return '原故障路径'
})

const aiNodePortSwitchText = computed(() => {
  const debugPort = aiPortDiagnosis.value?.debugPort
  const httpPort = aiPortDiagnosis.value?.httpPort
  const oldNodePort = debugPort?.nodePort || debugPort?.node_port
  const httpNodePort = httpPort?.nodePort || httpPort?.node_port
  if (oldNodePort && httpNodePort) return `如果入口平台直接配置的是 NodePort，需要从 ${oldNodePort} 改为 ${httpNodePort}。`
  if (oldNodePort) return `如果入口平台直接引用了 debug NodePort ${oldNodePort}，需要改为 HTTP 端口对应的 NodePort。`
  return ''
})

const aiVerificationSteps = computed(() => {
  const sectionSteps = getAiAnalysisSectionLines('验证步骤')
  if (sectionSteps.length) return sectionSteps
  if (!aiPortDiagnosis.value) return []
  const namespace = aiAnalysisTargetMeta.value.namespace
  const serviceName = aiPortDiagnosis.value.service?.name
  return [
    `执行 kubectl -n ${namespace} get ingress -o yaml，确认 backend service.port 指向 HTTP 端口。`,
    serviceName ? `执行 kubectl -n ${namespace} get svc ${serviceName} -o yaml，确认 HTTP targetPort 与容器监听端口一致。` : '',
    '重新请求原 URL，确认状态码恢复且日志不再出现 expected >JDWP-Handshake<。'
  ].filter(Boolean)
})

const aiKeyEvidence = computed(() => {
  const evidence = aiAnalysisEvidence.value
  const result = aiAnalysisResult.value || {}
  const candidates = []
  const append = value => {
    if (!value) return
    if (Array.isArray(value)) {
      value.forEach(append)
      return
    }
    if (typeof value === 'object') {
      candidates.push(normalizeAiReasonText(
        value.message ||
        value.reason ||
        value.summary ||
        value.title ||
        value.name ||
        value.code ||
        JSON.stringify(value)
      ))
      return
    }
    candidates.push(normalizeAiReasonText(value))
  }

  if (evidence) {
    append(evidence.keyEvidence || evidence.key_evidence)
    append(evidence.findings)
    append(evidence.logErrors || evidence.log_errors)
    append(evidence.warningEvents || evidence.warning_events)
    append(evidence.resourceHotspots || evidence.resource_hotspots)
    append(evidence.collectionErrors || evidence.collection_errors)
  }
  append(result.keyEvidence || result.key_evidence)

  getAiAnalysisSectionLines('关键证据', '证据').forEach(append)

  return [...new Set(candidates.filter(Boolean))].slice(0, 8)
})

const aiAnalysisCopyText = computed(() => {
  const lines = []
  lines.push(`1. 问题描述\n${aiProblemDescription.value}`)
  lines.push(`2. 问题原因\n${aiProblemCause.value}`)
  lines.push('3. 解决方案')
  if (aiServiceFixYaml.value) lines.push(`修改 Service YAML，删除 debug/JDWP 端口：\n${aiServiceFixYaml.value}`)
  if (aiFinalRemediation.value.length) {
    lines.push(...aiFinalRemediation.value.map((item, index) => `${index + 1}. ${item}`))
  }
  if (aiIngressFixYaml.value) lines.push(`Ingress 修复示例：\n${aiIngressFixYaml.value}`)
  if (aiNodePortSwitchText.value) lines.push(aiNodePortSwitchText.value)
  if (aiPortDiagnosis.value) lines.push(`修改后重新访问 ${aiObservedRequestPath.value}，JDWP 日志不应再出现 HTTP 请求握手失败。`)
  lines.push(`4. 总结结论\n${aiSummaryConclusion.value}`)
  if (aiServiceExposureLines.value.length) {
    lines.push('Service 更新后只暴露：')
    lines.push(...aiServiceExposureLines.value)
    lines.push('Service 更新后无需重启 Pod。')
  }
  return lines.join('\n')
})

const aiAnalysisStepGroupMap = {
  queued: 'task_created',
  task_created: 'task_created',
  start: 'start',
  started: 'start',
  connect_cluster: 'start',
  workload_pods: 'start',
  pod_status: 'status_check',
  status_check: 'status_check',
  logs_current: 'log_scan',
  logs_previous: 'log_scan',
  log_scan: 'log_scan',
  events: 'signal_scan',
  node: 'signal_scan',
  quota: 'signal_scan',
  storage: 'signal_scan',
  probes: 'signal_scan',
  metrics: 'signal_scan',
  metrics_trend: 'signal_scan',
  evidence: 'signal_scan',
  signal_scan: 'signal_scan',
  ai_analysis: 'ai_diagnosis',
  ai_diagnosis: 'ai_diagnosis',
  result: 'ai_diagnosis'
}

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
    const payload = getAiResponsePayload(response, '获取 AI Provider 失败')
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
    console.warn('获取默认 AI Provider 失败:', error)
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
  aiAnalysisProgress.value = 0
  aiAnalysisMessage.value = ''
  aiAnalysisEvents.value = []
  aiAnalysisSteps.value = createAiAnalysisSteps()
}

const closeAiAnalysisStream = () => {
  if (!aiAnalysisEventSource) return
  aiAnalysisEventSource.close()
  aiAnalysisEventSource = null
}

const stopAiAnalysisPolling = () => {
  if (!aiAnalysisPollingTimer) return
  window.clearInterval(aiAnalysisPollingTimer)
  aiAnalysisPollingTimer = null
}

const normalizeAiStepStatus = status => {
  const normalized = String(status || '').toLowerCase()
  if (['success', 'succeeded', 'completed', 'done'].includes(normalized)) return 'success'
  if (['failed', 'error'].includes(normalized)) return 'failed'
  if (normalized === 'skipped') return 'skipped'
  return 'running'
}

const updateAiAnalysisSteps = (eventData = {}) => {
  const rawStep = String(eventData.step || eventData.currentStep || eventData.current_step || eventData.type || '')
    .trim()
    .toLowerCase()
  const stepKey = aiAnalysisStepGroupMap[rawStep]
  const stepIndex = aiAnalysisSteps.value.findIndex(item => item.key === stepKey)
  if (stepIndex < 0) return

  aiAnalysisSteps.value.forEach((step, index) => {
    if (index < stepIndex && ['pending', 'running'].includes(step.status)) {
      step.status = 'success'
    }
  })

  const step = aiAnalysisSteps.value[stepIndex]
  const terminalStatus = eventData.stepStatus || eventData.step_status
  step.status = normalizeAiStepStatus(
    terminalStatus
    || (eventData.type === 'result' || eventData.status === 'succeeded' ? 'success' : eventData.status)
  )
  if (eventData.message) step.message = eventData.message
}

const appendAiAnalysisEvent = (eventData = {}) => {
  const progress = Number(eventData.progress)
  if (Number.isFinite(progress)) {
    aiAnalysisProgress.value = Math.max(0, Math.min(100, Math.round(progress)))
  }
  if (eventData.status) aiAnalysisTaskStatus.value = String(eventData.status).toLowerCase()
  if (eventData.message) aiAnalysisMessage.value = eventData.message
  updateAiAnalysisSteps(eventData)

  const normalizedEvent = {
    ...eventData,
    step: eventData.step || eventData.currentStep || eventData.current_step || eventData.type || 'step',
    message: eventData.message || '',
    timestamp: eventData.timestamp || new Date().toLocaleString('zh-CN')
  }
  const duplicate = aiAnalysisEvents.value.some(item =>
    item.step === normalizedEvent.step
    && item.message === normalizedEvent.message
    && item.timestamp === normalizedEvent.timestamp
  )
  if (!duplicate) {
    aiAnalysisEvents.value.unshift(normalizedEvent)
    aiAnalysisEvents.value = aiAnalysisEvents.value.slice(0, 50)
  }
}

const normalizeAiTaskResult = payload => {
  if (!payload) return null
  return payload.result || payload.data?.result || payload
}

const finishAiAnalysisFromTask = payload => {
  const result = normalizeAiTaskResult(payload)
  if (result) aiAnalysisResult.value = result
  aiAnalysisTaskStatus.value = 'succeeded'
  aiAnalysisProgress.value = 100
  aiAnalysisMessage.value = payload?.message || 'AI 分析完成'
  aiAnalysisSteps.value.forEach(step => {
    if (['pending', 'running'].includes(step.status)) step.status = 'success'
  })
  aiAnalysisLoading.value = false
  stopAiAnalysisPolling()
  closeAiAnalysisStream()
}

const failAiAnalysisTask = (message, { notify = true } = {}) => {
  aiAnalysisTaskStatus.value = 'failed'
  aiAnalysisMessage.value = message || 'AI 分析失败'
  const runningStep = aiAnalysisSteps.value.find(step => step.status === 'running')
  if (runningStep) {
    runningStep.status = 'failed'
    runningStep.message = aiAnalysisMessage.value
  }
  aiAnalysisLoading.value = false
  stopAiAnalysisPolling()
  closeAiAnalysisStream()
  if (notify) ElMessage.error(aiAnalysisMessage.value)
}

const fetchAiAnalysisTaskStatus = async (taskId, options = {}) => {
  if (!taskId) return
  try {
    const response = await aiopsApi.getK8sPodAnalyzeTask(taskId)
    const payload = getAiResponsePayload(response, '查询 AI 分析任务失败')
    if (!options.silentEvents && Array.isArray(payload.events)) {
      payload.events.forEach(appendAiAnalysisEvent)
    }
    appendAiAnalysisEvent({
      type: 'task_status',
      step: payload.currentStep || payload.current_step,
      status: payload.status,
      progress: payload.progress,
      message: payload.message
    })

    if (payload.status === 'succeeded' && payload.result) {
      finishAiAnalysisFromTask(payload)
    } else if (payload.status === 'failed') {
      failAiAnalysisTask(payload.error || payload.message || 'AI 分析失败')
    }
  } catch (error) {
    console.warn('查询 AI 分析任务状态失败:', error)
  }
}

const startAiAnalysisPolling = taskId => {
  stopAiAnalysisPolling()
  if (!taskId) return
  aiAnalysisPollingTimer = window.setInterval(async () => {
    if (!aiAnalysisVisible.value || aiAnalysisTaskId.value !== taskId) return
    if (['succeeded', 'failed'].includes(aiAnalysisTaskStatus.value)) {
      stopAiAnalysisPolling()
      return
    }
    await fetchAiAnalysisTaskStatus(taskId, { silentEvents: true })
  }, 1500)
}

const subscribeAiAnalysisTaskStream = taskId => {
  closeAiAnalysisStream()
  const source = aiopsApi.createK8sPodAnalyzeTaskStream(taskId)
  aiAnalysisEventSource = source
  const trackedSource = source

  const handleEvent = (rawType, event) => {
    if (aiAnalysisEventSource !== trackedSource) return
    const data = parseAiSseData(event)
    const eventData = { type: data.type || rawType, ...data }
    appendAiAnalysisEvent(eventData)

    if (eventData.type === 'result' || eventData.step === 'result') {
      finishAiAnalysisFromTask(eventData)
    } else if (eventData.type === 'error' || eventData.status === 'failed') {
      failAiAnalysisTask(eventData.error || eventData.message || 'AI 分析失败')
    }
  }

  source.addEventListener('open', () => {
    if (aiAnalysisEventSource !== trackedSource) return
    aiAnalysisMessage.value = '进度流已连接，等待分析结果...'
  })
  source.addEventListener('started', event => handleEvent('started', event))
  source.addEventListener('step', event => handleEvent('step', event))
  source.addEventListener('result', event => handleEvent('result', event))
  source.addEventListener('error', event => {
    if (event?.data) handleEvent('error', event)
  })
  source.onmessage = event => handleEvent('message', event)
  source.onerror = () => {
    if (aiAnalysisEventSource !== trackedSource || ['succeeded', 'failed'].includes(aiAnalysisTaskStatus.value)) return
    aiAnalysisMessage.value = '进度连接中断，正在通过任务查询继续获取结果...'
    fetchAiAnalysisTaskStatus(taskId, { silentEvents: true })
  }
}

const runAiAnalysis = async target => {
  const analysisTarget = target || selectedAiAnalysisTarget.value
  if (!analysisTarget?.name) {
    ElMessage.warning('未找到要分析的工作负载或 Pod')
    return
  }
  if (!selectedClusterId.value) {
    ElMessage.warning('请先选择集群')
    return
  }

  const namespace = getResourceNamespace(analysisTarget)
  if (!namespace) {
    ElMessage.warning('未找到资源所属的命名空间')
    return
  }

  selectedAiAnalysisTarget.value = analysisTarget
  aiAnalysisVisible.value = true
  aiAnalysisResult.value = null
  aiAnalysisLoading.value = true
  closeAiAnalysisStream()
  stopAiAnalysisPolling()
  resetAiAnalysisTaskState()

  try {
    const providerId = await getDefaultAiProviderId()
    const workloadType = normalizeAiWorkloadType(analysisTarget.type || analysisTarget.rawData?.kind)
    const isPod = workloadType === 'pod'
    const payload = {
      providerId: providerId || undefined,
      clusterId: Number(selectedClusterId.value) || selectedClusterId.value,
      namespace,
      pod: isPod ? analysisTarget.name : undefined,
      workloadType: isPod ? undefined : workloadType,
      workloadName: isPod ? undefined : analysisTarget.name,
      tailLines: 500,
      limit: 200,
      eventLimit: 50,
      contextLines: 5,
      includePrevious: true,
      includeEvents: true,
      includeMetrics: true,
      includeNode: true,
      includeQuota: true,
      includeStorage: true,
      includeProbe: true,
      allContainers: true,
      prompt: aiAnalysisPrompt.value
    }

    const response = await aiopsApi.createK8sPodAnalyzeTask(payload)
    const task = getAiResponsePayload(response, '创建 AI 分析任务失败')
    const taskId = task.taskId || task.task_id || task.id
    if (!taskId) throw new Error('后端未返回 AI 分析任务 ID')

    aiAnalysisTaskId.value = taskId
    aiAnalysisTaskStatus.value = task.status || 'running'
    appendAiAnalysisEvent({
      type: 'started',
      step: 'task_created',
      stepStatus: 'succeeded',
      status: aiAnalysisTaskStatus.value,
      progress: 0,
      message: '分析任务已创建'
    })
    startAiAnalysisPolling(taskId)
    try {
      subscribeAiAnalysisTaskStream(taskId)
    } catch (streamError) {
      console.warn('连接 AI 分析进度流失败，将使用轮询获取结果:', streamError)
      aiAnalysisMessage.value = '进度流不可用，正在通过任务查询获取结果...'
    }
  } catch (error) {
    console.error('创建 AI 分析任务失败:', error)
    failAiAnalysisTask(error.message || 'AI 分析任务创建失败')
  }
}

const openAiAnalysisDialog = row => runAiAnalysis(row)

const rerunAiAnalysis = () => runAiAnalysis(selectedAiAnalysisTarget.value)

const handleAiAnalysisDialogClosed = () => {
  aiAnalysisLoading.value = false
  closeAiAnalysisStream()
  stopAiAnalysisPolling()
}

const normalizeWorkloadImages = (workload = {}) => {
  const candidates = [
    workload.images,
    workload.image,
    workload.containers,
    workload.initContainers,
    workload.ephemeralContainers
  ]
  const images = []

  const appendImage = item => {
    if (!item) return
    if (Array.isArray(item)) {
      item.forEach(appendImage)
      return
    }

    const image = String(typeof item === 'string' ? item : item.image || '').trim()
    if (image && !images.includes(image)) images.push(image)
  }

  candidates.forEach(appendImage)
  return images
}

// 扩容缩容表单
const scaleForm = reactive({
  replicas: 1
})

// 获取集群列表
const fetchClusterList = async () => {
  try {
    const response = await k8sApi.getClusterList()
    const responseData = response.data || response
    
    if (responseData.code === 200 || responseData.success) {
      const clusters = responseData.data?.list || responseData.data || []
      clusterList.value = clusters.map(cluster => ({
        id: cluster.id,
        name: cluster.name,
        status: cluster.status
      }))

      const resolvedClusterId = resolvePreferredClusterId(
        clusterList.value,
        selectedClusterId.value || getStoredClusterId()
      )

      if (resolvedClusterId) {
        selectedClusterId.value = resolvedClusterId
        saveStoredClusterId(resolvedClusterId, CLUSTER_SELECTION_SOURCE)
      }
      
      console.log('集群列表加载成功:', clusterList.value)
    } else {
      ElMessage.error(responseData.message || '获取集群列表失败')
    }
  } catch (error) {
    console.error('获取集群列表失败:', error)
    ElMessage.warning('无法获取集群列表，请检查后端服务')
  }
}

// namespaceRequestPromise 已移至 NamespaceSelector 组件

// fetchNamespaceList 已移至 NamespaceSelector 组件

// 处理标签页切换
const handleTabChange = (tabName) => {
  const normalizedTab = normalizeWorkloadType(tabName)
  console.log('标签页切换到:', normalizedTab)
  activeTab.value = normalizedTab
  queryParams.type = normalizedTab
  clearPodSelection()
  syncTabState(normalizedTab)

  if (normalizedTab === ALL_WORKLOAD_TYPE) {
    queryParams.name = ''
    queryParams.scope = WORKLOAD_SCOPE_ALL
  }

  currentPage.value = 1
  handleQuery()
}

// 监听activeTab变化，同步到queryParams.type
watch(activeTab, (newType) => {
  const normalizedType = normalizeWorkloadType(newType)
  queryParams.type = normalizedType
  persistTab(normalizedType)
})

watch(
  () => route.query[WORKLOAD_TAB_QUERY_KEY],
  (newTab) => {
    const routeTab = Array.isArray(newTab) ? newTab[0] : newTab
    const normalizedTab = normalizeWorkloadType(routeTab)

    if (normalizedTab === activeTab.value) {
      return
    }

    activeTab.value = normalizedTab
    queryParams.type = normalizedTab
    currentPage.value = 1

    if (!selectedClusterId.value) {
      resetTableData()
      return
    }

    if (!queryParams.namespace) {
      resetTableData()
      return
    }

    handleQuery()
  }
)

const resetTableData = () => {
  latestWorkloadListRequestId++
  loading.value = false
  tableData.value = []
  total.value = 0
  clearPodSelection()
}

const getRequestWorkloadType = () =>
  !queryParams.type || queryParams.type === ALL_WORKLOAD_TYPE ? ALL_WORKLOAD_TYPE : queryParams.type

const normalizeClusterId = (clusterId) => clusterId ? String(clusterId) : ''
const getKeyword = () => (queryParams.name || '').trim()
const getSelectedNodeName = () => (queryParams.nodeName || '').trim()
const getResourceNamespace = (resource = {}) =>
  resource?.namespace || currentWorkload.value?.namespace || queryParams.namespace
const getWorkloadRowKey = (row) => `${row?.namespace || ''}:${row?.name || row?.id || ''}:${row?.type || ''}`
const currentClusterNodeOptions = computed(() => {
  const currentClusterId = normalizeClusterId(selectedClusterId.value)
  if (!currentClusterId) return []

  return nodeOptions.value.filter(option => option.clusterId === currentClusterId)
})

const clearInvalidSelectedNode = () => {
  const nodeName = getSelectedNodeName()
  if (!nodeName) return

  const existsInCurrentCluster = currentClusterNodeOptions.value.some(option => option.value === nodeName)
  if (!existsInCurrentCluster) {
    queryParams.nodeName = ''
  }
}

const fetchNodeOptions = async (clusterId = selectedClusterId.value) => {
  const normalizedClusterId = normalizeClusterId(clusterId)
  const requestId = ++latestNodeOptionsRequestId

  if (!normalizedClusterId) {
    nodeOptions.value = []
    nodesLoading.value = false
    return
  }

  try {
    nodesLoading.value = true
    const response = await k8sApi.getClusterNodes(normalizedClusterId)
    const responseData = response.data || response

    if (
      requestId !== latestNodeOptionsRequestId ||
      normalizedClusterId !== normalizeClusterId(selectedClusterId.value)
    ) {
      return
    }

    if (responseData.code === 200 || responseData.success) {
      const nodes = Array.isArray(responseData.data) ? responseData.data : []
      nodeOptions.value = nodes
        .map(node => ({
          label: node.name,
          value: node.name,
          clusterId: normalizedClusterId
        }))
        .sort((a, b) => a.label.localeCompare(b.label))
      clearInvalidSelectedNode()
      return
    }

    nodeOptions.value = []
  } catch (error) {
    if (
      requestId !== latestNodeOptionsRequestId ||
      normalizedClusterId !== normalizeClusterId(selectedClusterId.value)
    ) {
      return
    }

    console.error('获取节点列表失败:', error)
    nodeOptions.value = []
  } finally {
    if (requestId === latestNodeOptionsRequestId) {
      nodesLoading.value = false
    }
  }
}

const isLatestWorkloadListRequest = (requestId) => requestId === latestWorkloadListRequestId

// 查询工作负载列表
const handleQuery = async () => {
  const requestId = ++latestWorkloadListRequestId
  const queryStartTime = Date.now()
  
  try {
    const clusterId = selectedClusterId.value
    const namespace = queryParams.namespace
    const scope = queryParams.scope
    const requestType = getRequestWorkloadType()
    const keyword = getKeyword()
    const nodeName = getSelectedNodeName()
    const page = currentPage.value
    const size = pageSize.value

    if (!clusterId) {
      ElMessage.warning('请选择一个集群')
      return
    }
    
    if (!namespace) {
      ElMessage.warning('请选择命名空间')
      return
    }
    
    console.log('🔍 开始查询工作负载:', {
      clusterId,
      namespace,
      scope,
      type: requestType,
      nodeName,
      keyword,
      page,
      pageSize: size
    })
    
    loading.value = true
    
    const params = {
      page,
      pageSize: size
    }
    let response

    if (scope === WORKLOAD_SCOPE_ABNORMAL) {
      response = await k8sApi.getAbnormalWorkloadList(
        clusterId,
        namespace,
        {
          ...params,
          type: requestType,
          ...(nodeName ? { nodeName } : {}),
          ...(keyword ? { keyword } : {})
        }
      )
    } else if (keyword) {
      response = await k8sApi.searchWorkloadList(
        clusterId,
        namespace,
        {
          ...params,
          type: requestType,
          ...(nodeName ? { nodeName } : {}),
          keyword
        }
      )
    } else {
      response = await k8sApi.getWorkloadList(clusterId, namespace, {
        ...params,
        type: requestType,
        ...(nodeName ? { nodeName } : {})
      })
    }

    if (!isLatestWorkloadListRequest(requestId)) {
      console.log('忽略过期的工作负载查询响应:', {
        requestId,
        latestRequestId: latestWorkloadListRequestId,
        clusterId,
        namespace,
        scope,
        type: requestType,
        keyword
      })
      return
    }
    
    const responseData = response.data || response
    console.log('工作负载查询API响应:', responseData)
    
    if (responseData.code === 200 || responseData.success) {
      const workloadData = responseData.data || {}
      const workloads = Array.isArray(workloadData) ? workloadData : (workloadData.workloads || [])
      // 确保workloads是数组
      const workloadList = Array.isArray(workloads) ? workloads : []
      const serverTotal = Array.isArray(workloadData)
        ? workloadList.length
        : Number(workloadData.total ?? workloadList.length)
      const serverPage = Array.isArray(workloadData)
        ? page
        : Number(workloadData.page || page)
      const serverPageSize = Array.isArray(workloadData)
        ? size
        : Number(workloadData.pageSize || size)
      const serverTotalPages = Array.isArray(workloadData)
        ? (serverTotal > 0 ? Math.ceil(serverTotal / serverPageSize) : 0)
        : Number(workloadData.totalPages || (serverTotal > 0 ? Math.ceil(serverTotal / serverPageSize) : 0))

      if (serverTotalPages > 0 && serverPage > serverTotalPages) {
        if (!isLatestWorkloadListRequest(requestId)) {
          return
        }

        currentPage.value = serverTotalPages
        await handleQuery()
        return
      }

      tableData.value = workloadList.map(workload => ({
        id: workload.name,
        name: workload.name,
        type: workload.type?.toLowerCase() || workload.kind?.toLowerCase(),
        namespace: workload.namespace,
        nodeName: workload.nodeName || '',
        replicas: `${workload.readyReplicas || 0}/${workload.currentReplicas ?? workload.replicas ?? 0}`,
        readyReplicas: workload.readyReplicas || 0,
        totalReplicas: workload.currentReplicas ?? workload.replicas ?? 0,
        desiredReplicas: workload.desiredReplicas ?? workload.replicas ?? 0,
        currentReplicas: workload.currentReplicas ?? workload.replicas ?? 0,
        availableReplicas: workload.availableReplicas ?? workload.readyReplicas ?? 0,
        images: normalizeWorkloadImages(workload),
        labels: workload.labels || {},
        status: workload.status || getWorkloadStatus(workload),
        age: formatAge(workload.createdAt),
        updateTime: workload.createdAt,
        updatedAt: workload.updatedAt,
        conditions: workload.conditions || [],
        resources: workload.resources || {
          requests: { cpu: '0', memory: '0' },
          limits: { cpu: '0', memory: '0' }
        },
        rawData: workload
      }))

      total.value = serverTotal
      currentPage.value = serverPage
      pageSize.value = serverPageSize
      
      console.log('工作负载列表加载成功:', tableData.value)
    } else {
      const errorMsg = responseData.message || '获取工作负载列表失败'

      // 特殊处理资源不存在的错误
      if (errorMsg.includes('the server could not find the requested resource')) {
        if (requestType === 'cronjobs') {
          ElMessage.warning('当前集群不支持CronJob资源，可能是Kubernetes版本过低')
        } else if (requestType && requestType !== ALL_WORKLOAD_TYPE) {
          ElMessage.warning(`当前集群不支持${requestType}资源类型`)
        } else {
          ElMessage.warning('请求的资源不存在，请检查集群配置')
        }
      } else {
        ElMessage.error(errorMsg)
      }

      resetTableData()
    }
  } catch (error) {
    if (!isLatestWorkloadListRequest(requestId)) {
      console.log('忽略过期的工作负载查询错误:', {
        requestId,
        latestRequestId: latestWorkloadListRequestId,
        error
      })
      return
    }

    console.error('获取工作负载列表失败:', error)
    
    if (error.code === 'ERR_NETWORK' || 
        error.message?.includes('ERR_CONNECTION_REFUSED') ||
        error.message?.includes('Failed to fetch')) {
      ElMessage.warning('后端服务连接失败，请检查服务状态')
    } else if (error.response?.status === 401) {
      ElMessage.error('认证失败，请重新登录')
    } else if (error.response?.status === 403) {
      ElMessage.error('权限不足，请联系管理员')
    } else {
      console.warn('API调用异常，但可能数据已正确加载')
    }
    
    resetTableData()
  } finally {
    if (isLatestWorkloadListRequest(requestId)) {
      loading.value = false
    }
    console.log('✅ 工作负载查询完成，耗时:', Date.now() - queryStartTime + 'ms')
  }
}

// 获取工作负载状态
const getWorkloadStatus = (workload) => {
  // 如果后端直接返回了状态，优先使用
  if (workload.status) return workload.status
  
  const replicas = workload.replicas || 0
  const readyReplicas = workload.readyReplicas || 0
  
  if (workload.type === 'job' || workload.kind === 'Job') {
    return workload.succeeded ? 'Completed' : 
           workload.failed ? 'Failed' : 'Running'
  }
  
  if (workload.type === 'cronjob' || workload.kind === 'CronJob') {
    return workload.lastScheduleTime ? 'Active' : 'Suspended'
  }
  
  if (replicas === 0) return 'Stopped'
  if (readyReplicas === 0) return 'Pending'
  if (readyReplicas < replicas) return 'Partial'
  return 'Running'
}

const getStatusTagType = (status) => {
  const statusTypeMap = {
    Running: 'success',
    Completed: 'info',
    Succeeded: 'info',
    Active: 'success',
    Suspended: 'info',
    Stopped: 'info',
    Pending: 'warning',
    NotReady: 'warning',
    Partial: 'warning',
    Terminating: 'warning',
    Failed: 'danger',
    Error: 'danger',
    CrashLoopBackOff: 'danger',
    ImagePullBackOff: 'danger'
  }

  return statusTypeMap[status] || 'warning'
}

const getStatusTagTypeByRow = (row) => {
  if (isPodsTab.value) {
    return getStatusTagType(row.status)
  }

  return getPodStatusTagByReplicas(row.readyReplicas, row.totalReplicas)
}

const getReplicaColumnValue = (row) => (isPodsTab.value ? row.status : row.replicas)

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

const formatDateTime = (timestamp) => {
  if (!timestamp) return '-'
  
  const date = new Date(timestamp)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  
  return `${year}-${month}-${day} ${hours}:${minutes}`
}

const handleRefresh = () => {
  clearPodSelection()
  handleQuery()
}

const handleScopeChange = (scope) => {
  queryParams.scope = scope
  persistScope(scope)
  currentPage.value = 1
  clearPodSelection()
  handleQuery()
}

const handleNodeChange = () => {
  queryParams.nodeName = getSelectedNodeName()
  clearInvalidSelectedNode()
  currentPage.value = 1
  clearPodSelection()
  handleQuery()
}

const handleSearch = () => {
  queryParams.name = getKeyword()
  currentPage.value = 1
  clearPodSelection()
  handleQuery()
}

const handlePageChange = (page) => {
  currentPage.value = page
  clearPodSelection()
  handleQuery()
}

const handleSizeChange = (size) => {
  pageSize.value = size
  currentPage.value = 1
  clearPodSelection()
  handleQuery()
}

// 导航到监控仪表板
const navigateToMonitoring = () => {
  router.push('/k8s/monitoring')
}

// 创建工作负载相关函数
const showCreatePodDialog = () => {
  if (!selectedClusterId.value) {
    ElMessage.warning('请先选择集群')
    return
  }
  if (!queryParams.namespace) {
    ElMessage.warning('请先选择命名空间')
    return
  }

  createPodDialogVisible.value = true
}

// 处理YAML创建校验
const handlePodPreview = async (data) => {
  try {
    createPodDialogRef.value?.setLoading(true)

    // 使用validateYaml来校验YAML格式
    const response = await k8sApi.validateYaml(selectedClusterId.value, data.yamlContent)
    const responseData = response.data || response

    const result = {
      success: responseData.code === 200,
      message: responseData.message || (responseData.code === 200 ? '可以创建工作负载' : '创建预览失败'),
      details: responseData.data
    }

    createPodDialogRef.value?.setDryRunResult(result)

    if (result.success) {
      ElMessage.success(result.message)
    } else {
      ElMessage.error(result.message)
    }
  } catch (error) {
    const result = {
      success: false,
      message: error.message || '工作负载创建预览失败',
      details: null
    }
    createPodDialogRef.value?.setDryRunResult(result)
    ElMessage.error('工作负载创建预览失败: ' + (error.message || '网络错误'))
  } finally {
    createPodDialogRef.value?.setLoading(false)
  }
}

// 处理工作负载创建
const handlePodCreate = async (data) => {
  try {
    createPodDialogRef.value?.setLoading(true)

    const response = data?.mode === 'form'
      ? await k8sApi.createDeployment(selectedClusterId.value, queryParams.namespace, data.payload)
      : await k8sApi.createPodFromYaml(selectedClusterId.value, queryParams.namespace, data)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success('工作负载创建成功!')
      createPodDialogVisible.value = false
      handleQuery() // 刷新工作负载列表
    } else {
      ElMessage.error(responseData.message || '工作负载创建失败')
    }
  } catch (error) {
    console.error('工作负载创建失败:', error)
    ElMessage.error('工作负载创建失败: ' + (error.message || '网络错误'))
  } finally {
    createPodDialogRef.value?.setLoading(false)
  }
}


// 集群选择变化处理
const handleClusterChange = async () => {
  // 清空数据，NamespaceSelector 组件会自动处理命名空间列表加载
  resetTableData()
  currentPage.value = 1
  queryParams.nodeName = ''
  nodeOptions.value = []
  latestNodeOptionsRequestId++

  saveStoredClusterId(selectedClusterId.value, CLUSTER_SELECTION_SOURCE)
  await fetchNodeOptions(selectedClusterId.value)
}

// 命名空间选择变化处理
const handleNamespaceChange = async () => {
  if (selectedClusterId.value && queryParams.namespace) {
    currentPage.value = 1
    clearPodSelection()
    await fetchNodeOptions(selectedClusterId.value)
    handleQuery()
  } else {
    resetTableData()
  }
}

const openPendingTab = () => {
  const openedTab = window.open('', '_blank')
  if (!openedTab) {
    ElMessage.warning('浏览器已拦截新标签页，请允许弹出窗口后重试')
    return null
  }

  openedTab.opener = null
  return openedTab
}

const openRouteInTab = (openedTab, routeLocation) => {
  openedTab.location.href = router.resolve(routeLocation).href
}

const resolveTargetPod = async (row, clusterId) => {
  const targetNamespace = getResourceNamespace(row)
  if (isPodRow(row)) {
    return {
      namespace: targetNamespace,
      podName: row.name,
      workloadQuery: {}
    }
  }

  const workloadType = String(row.type || '').toLowerCase()
  const response = await k8sApi.getWorkloadPods(
    clusterId,
    targetNamespace,
    workloadType,
    row.name
  )
  const responseData = response.data || response
  const firstPod = responseData.code === 200 && Array.isArray(responseData.data)
    ? responseData.data[0]
    : null

  if (!firstPod?.name) return null

  return {
    namespace: targetNamespace,
    podName: firstPod.name,
    workloadQuery: {
      workloadType,
      workloadName: row.name
    }
  }
}

// 名称、日志和终端均在独立标签页打开，保留当前工作负载列表状态。
const openPodRoute = async (row, routeType, extraQuery = {}) => {
  const openedTab = openPendingTab()
  if (!openedTab) return

  try {
    const clusterId = selectedClusterId.value
    const target = await resolveTargetPod(row, clusterId)
    if (!target) {
      openedTab.close()
      ElMessage.warning('该工作负载下暂无Pod或Pod信息获取失败')
      return
    }

    openRouteInTab(openedTab, {
      path: `/k8s/${routeType}/${clusterId}/${target.namespace}/${target.podName}`,
      query: {
        ...target.workloadQuery,
        ...extraQuery
      }
    })
  } catch (error) {
    openedTab.close()
    console.error('获取工作负载Pod信息失败:', error)
    ElMessage.error('获取工作负载Pod信息失败，请检查网络连接')
  }
}

const navigateToPodDetail = (row) => openPodRoute(row, 'pod')
const navigateToPodLogs = (row) => openPodRoute(row, 'pod', { tab: 'logs' })
const navigateToPodTerminal = (row) => openPodRoute(row, 'terminal', { autoConnect: '1' })

const hasPodDeletePermission = () => checkAuthority(
  ['k8s:workload:poddelete'],
  storage.getItem('permissionList') || []
)

const isPodSelectable = (row) => isPodsTab.value && canDelete(row) && hasPodDeletePermission()

const handlePodSelectionChange = (selection) => {
  selectedPods.value = Array.isArray(selection) ? selection : []
}

const clearPodSelection = () => {
  selectedPods.value = []
  workloadTableRef.value?.clearSelection?.()
}

const deletePodResource = async (row, { confirm = true, clusterId = selectedClusterId.value } = {}) => {
  if (!canDelete(row)) {
    ElMessage.warning('系统 Pod 不允许删除')
    return false
  }

  const targetNamespace = getResourceNamespace(row)

  try {
    if (confirm) {
      await ElMessageBox.confirm(
        `确定要删除 Pod "${row.name}" 吗？此操作不可恢复！`,
        '删除确认',
        {
          confirmButtonText: '确定删除',
          cancelButtonText: '取消',
          type: 'warning',
          dangerouslyUseHTMLString: false
        }
      )
    }

    const response = await k8sApi.deletePod(clusterId, targetNamespace, row.name)
    const responseData = response.data || response
    if (responseData.code === 200 || responseData.success) {
      return true
    }

    ElMessage.error(responseData.message || `删除 Pod "${row.name}" 失败`)
    return false
  } catch (error) {
    if (error === 'cancel') {
      ElMessage.info('已取消删除操作')
      return false
    }

    console.error('删除 Pod 失败:', error)
    ElMessage.error(`删除失败: ${error.message || '请检查网络连接'}`)
    return false
  }
}


const handleMoreCommand = (row, command) => {
  switch (command) {
    case 'terminal':
      navigateToPodTerminal(row)
      break
    case 'scale':
      scaleWorkload(row)
      break
    case 'restart':
      restartWorkload(row)
      break
    case 'schedule':
      updateScheduling(row)
      break
    case 'history':
      openDeploymentHistory(row)
      break
    case 'yaml':
      if (row.type === 'pod') {
        viewYaml(row.rawData || row)
      } else {
        editWorkloadYaml(row)
      }
      break
    case 'delete':
      deleteWorkload(row)
      break
  }
}

const isPodRow = (row) => {
  const type = String(row?.type || row?.rawData?.type || row?.rawData?.kind || '').toLowerCase()
  return type === 'pod' || type === 'pods'
}

// 查看YAML
const viewYaml = async (pod) => {
  const targetNamespace = getResourceNamespace(pod)

  openYamlDialog({
    clusterId: selectedClusterId.value,
    resourceType: 'Pod',
    resourceName: pod.name,
    editable: true,
    fetchYaml: (clusterId, resourceName) => k8sApi.getPodYaml(clusterId, targetNamespace, resourceName),
    updateYaml: async (clusterId, resourceName, yamlContent) => {
      const response = await k8sApi.updatePodYaml(clusterId, targetNamespace, resourceName, yamlContent)
      const responseData = response.data || response
      const message = responseData.message || ''

      if (responseData.code === 200 || responseData.success || !message.includes('force=true')) {
        return response
      }

      await ElMessageBox.confirm(
        `更新 Pod "${resourceName}" 需要删除并重建，期间可能造成服务中断。是否继续？`,
        '重建 Pod 确认',
        {
          confirmButtonText: '继续更新',
          cancelButtonText: '取消',
          type: 'warning'
        }
      )
      return k8sApi.updatePodYaml(clusterId, targetNamespace, resourceName, yamlContent, { force: true })
    },
    fallbackYaml: `apiVersion: v1\nkind: Pod\nmetadata:\n  name: ${pod.name}`,
    loadErrorMessage: '获取Pod YAML失败',
    saveSuccessMessage: `${pod.name} YAML更新成功`,
    saveErrorMessage: 'Pod YAML更新失败',
    onSaved: handleQuery
  })
}

// 扩容缩容
const scaleWorkload = (row) => {
  if (!canScale(row)) {
    ElMessage.warning('该工作负载不支持扩缩容操作')
    return
  }
  
  currentWorkload.value = row
  scaleForm.replicas = row.desiredReplicas ?? row.totalReplicas ?? 1
  scaleDialogVisible.value = true
}

// 提交扩容缩容
const submitScale = async () => {
  try {
    const response = await k8sApi.scaleDeployment(
      selectedClusterId.value, 
      getResourceNamespace(currentWorkload.value), 
      currentWorkload.value.name, 
      { replicas: scaleForm.replicas }
    )
    
    const responseData = response.data || response
    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(`${currentWorkload.value.name} 扩缩容成功`)
      scaleDialogVisible.value = false
      handleQuery()
    } else {
      ElMessage.error(responseData.message || '扩缩容失败')
    }
  } catch (error) {
    console.error('扩缩容失败:', error)
    ElMessage.error('扩缩容失败，请检查网络连接')
  }
}

// 重启工作负载
const restartWorkload = async (row) => {
  if (!canRestart(row)) {
    ElMessage.warning('该工作负载不支持重启操作')
    return
  }
  
  try {
    const targetNamespace = getResourceNamespace(row)
    await ElMessageBox.confirm(
      `确定要重启 ${row.type} "${row.name}" 吗？`,
      '重启确认',
      {
        confirmButtonText: '确定重启',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
    
    const response = await k8sApi.restartDeployment(selectedClusterId.value, targetNamespace, row.name)
    
    const responseData = response.data || response
    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(`${row.name} 重启成功`)
      handleQuery()
    } else {
      ElMessage.error(responseData.message || '重启失败')
    }
  } catch (error) {
    if (error === 'cancel') {
      ElMessage.info('已取消重启操作')
    } else {
      console.error('重启失败:', error)
      ElMessage.error('重启失败，请检查网络连接')
    }
  }
}

const getWorkloadTypeName = (type) => {
  const nameMap = {
    'deployment': 'Deployment',
    'statefulset': 'StatefulSet',
    'daemonset': 'DaemonSet',
    'job': 'Job',
    'cronjob': 'CronJob',
    'pod': 'Pod'
  }
  return nameMap[type] || type
}

// 复制内容到剪贴板
const copyToClipboard = async (text, successMessage = '已复制到剪贴板') => {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success(successMessage)
  } catch (error) {
    console.error('复制失败:', error)
    const textArea = document.createElement('textarea')
    textArea.value = text
    document.body.appendChild(textArea)
    textArea.select()
    try {
      document.execCommand('copy')
      ElMessage.success(successMessage)
    } catch (fallbackError) {
      ElMessage.error('复制失败，请手动复制')
    }
    document.body.removeChild(textArea)
  }
}

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

const renderAiInline = value => {
  let html = escapeAiHtml(value)
  html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
  html = html.replace(/`([^`]+)`/g, '<code>$1</code>')
  return html
}

const renderAiAnalysisText = text => {
  if (!text) return '暂无分析文本'
  return String(text)
    .replace(/\r/g, '')
    .split('\n')
    .map(rawLine => {
      const line = rawLine.trim()
      if (!line) return '<div class="ai-analysis-gap"></div>'

      const heading = line.match(/^#{1,6}\s+(.+)$/)
      if (heading) return `<h4>${escapeAiHtml(heading[1])}</h4>`

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

// 判断是否为系统命名空间或系统工作负载
const isSystemWorkload = (workload) => {
  // 系统命名空间
  const systemNamespaces = ['kube-system', 'kube-public', 'kube-node-lease', 'calico-system', 'tigera-operator']
  if (systemNamespaces.includes(workload.namespace)) {
    return true
  }
  
  // 系统工作负载名称前缀
  const systemPrefixes = ['kube-', 'calico-', 'coredns', 'metrics-server', 'node-local-dns', 'kubernetes-dashboard']
  if (systemPrefixes.some(prefix => workload.name.startsWith(prefix))) {
    return true
  }
  
  return false
}

// 判断是否可以扩缩容
const canScale = (workload) => {
  // 只有 Deployment 和 StatefulSet 支持扩缩容
  if (!['deployment', 'statefulset'].includes(workload.type)) {
    return false
  }
  
  // 系统工作负载不允许扩缩容
  return !isSystemWorkload(workload)
}

// 判断是否可以重启
const canRestart = (workload) => {
  // 只有 Deployment 支持重启
  if (workload.type !== 'deployment') {
    return false
  }
  
  // 系统工作负载不允许重启
  return !isSystemWorkload(workload)
}

const canViewDeploymentHistory = (workload) => workload.type === 'deployment'

// 判断是否可以删除
const canDelete = (workload) => {
  // 系统工作负载不允许删除
  return !isSystemWorkload(workload)
}

const getMoreActionPermissions = workload => (
  isPodRow(workload)
    ? ['k8s:workload:edityaml', 'k8s:workload:poddelete']
    : ['k8s:workload:actions']
)

const getDeletePermission = workload => [
  isPodRow(workload) ? 'k8s:workload:poddelete' : 'k8s:workload:delete'
]

const handleBatchDeletePods = async () => {
  const operationClusterId = selectedClusterId.value
  const pods = selectedPods.value.filter(pod => canDelete(pod))

  if (!pods.length) {
    ElMessage.warning('请先选择要删除的 Pod')
    return
  }

  try {
    await ElMessageBox.confirm(
      `确定要删除选中的 ${pods.length} 个 Pod 吗？此操作不可恢复！`,
      '批量删除确认',
      {
        confirmButtonText: '确定删除',
        cancelButtonText: '取消',
        type: 'warning',
        dangerouslyUseHTMLString: false
      }
    )
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') {
      console.error('批量删除 Pod 确认流程异常:', error)
    }
    return
  }

  if (String(selectedClusterId.value) !== String(operationClusterId)) {
    ElMessage.warning('集群已切换，批量删除已取消')
    return
  }

  batchDeleteLoading.value = true
  let successCount = 0
  let failedCount = 0

  try {
    for (const pod of pods) {
      const success = await deletePodResource(pod, {
        confirm: false,
        clusterId: operationClusterId
      })
      if (success) {
        successCount += 1
      } else {
        failedCount += 1
      }
    }

    if (successCount > 0) {
      ElMessage.success(
        `已删除 ${successCount} 个 Pod${failedCount ? `，失败 ${failedCount} 个` : ''}`
      )
      clearPodSelection()
      await handleQuery()
    } else {
      ElMessage.error('批量删除失败')
    }
  } finally {
    batchDeleteLoading.value = false
  }
}

// 判断是否可以更新调度
const canUpdateScheduling = (workload) => {
  // 只有 Deployment、StatefulSet 支持调度更新
  if (!['deployment', 'statefulset'].includes(workload.type)) {
    return false
  }
  
  // 系统工作负载不允许调度更新
  return !isSystemWorkload(workload)
}

// 判断是否可以编辑YAML
const canEditYaml = (workload) => {
  // 系统工作负载不允许编辑YAML
  return !isSystemWorkload(workload)
}

const getRevisionImages = revision => {
  if (Array.isArray(revision?.containers)) {
    return revision.containers
      .map(container => (typeof container === 'string' ? container : container?.image))
      .filter(Boolean)
  }

  if (Array.isArray(revision?.images)) {
    return revision.images.filter(Boolean)
  }

  if (revision?.image) {
    return String(revision.image)
      .split(',')
      .map(image => image.trim())
      .filter(Boolean)
  }

  return []
}

const normalizeDeploymentHistory = (data = {}) => ({
  deploymentName: data.deploymentName || currentWorkload.value?.name || '',
  namespace: data.namespace || getResourceNamespace(currentWorkload.value),
  currentRevision: Number(data.currentRevision || 0),
  revisionHistoryLimit: Number(data.revisionHistoryLimit ?? 10),
  totalRevisions: Number(data.totalRevisions ?? data.revisions?.length ?? 0),
  revisions: Array.isArray(data.revisions)
    ? data.revisions.map(revision => ({
        ...revision,
        revision: Number(revision.revision || 0),
        images: getRevisionImages(revision),
        status: revision.status || 'historical',
        replicasSummary: revision.replicasSummary || {}
      }))
    : []
})

const fetchDeploymentHistory = async (row = currentWorkload.value) => {
  if (!row?.name || !canViewDeploymentHistory(row)) {
    return
  }

  deploymentHistoryLoading.value = true
  try {
    const response = await k8sApi.getDeploymentHistory(
      selectedClusterId.value,
      getResourceNamespace(row),
      row.name
    )
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      deploymentHistory.value = normalizeDeploymentHistory(responseData.data || {})
      return
    }

    ElMessage.error(responseData.message || '获取Deployment历史版本失败')
  } catch (error) {
    console.error('获取Deployment历史版本失败:', error)
    ElMessage.error('获取Deployment历史版本失败，请检查网络连接')
  } finally {
    deploymentHistoryLoading.value = false
  }
}

const openDeploymentHistory = async (row) => {
  if (!canViewDeploymentHistory(row)) {
    ElMessage.warning('只有Deployment支持历史版本')
    return
  }

  currentWorkload.value = row
  deploymentHistory.value = normalizeDeploymentHistory({
    deploymentName: row.name,
    namespace: getResourceNamespace(row),
    revisions: []
  })
  deploymentHistoryDialogVisible.value = true
  await fetchDeploymentHistory(row)
}

const canRollbackDeploymentRevision = (revision) =>
  revision?.revision &&
  revision.status !== 'current' &&
  Number(revision.revision) !== Number(deploymentHistory.value.currentRevision)

const rollbackDeploymentToRevision = async (revision) => {
  if (!canRollbackDeploymentRevision(revision)) {
    ElMessage.warning('当前版本不需要回滚')
    return
  }

  try {
    await ElMessageBox.confirm(
      `确定要将 Deployment "${deploymentHistory.value.deploymentName}" 回滚到版本 ${revision.revision} 吗？`,
      '回滚确认',
      {
        confirmButtonText: '确定回滚',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )

    rollbackLoading.value = true
    const response = await k8sApi.rollbackDeployment(
      selectedClusterId.value,
      deploymentHistory.value.namespace,
      deploymentHistory.value.deploymentName,
      { toRevision: revision.revision }
    )
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(responseData.data?.message || `已回滚到版本 ${revision.revision}`)
      await fetchDeploymentHistory(currentWorkload.value)
      await handleQuery()
      return
    }

    ElMessage.error(responseData.message || 'Deployment回滚失败')
  } catch (error) {
    if (error === 'cancel') {
      ElMessage.info('已取消回滚操作')
      return
    }

    console.error('Deployment回滚失败:', error)
    ElMessage.error(`Deployment回滚失败: ${error.message || '请检查网络连接'}`)
  } finally {
    rollbackLoading.value = false
  }
}

// 根据副本数获取Pod状态标签类型
const getPodStatusTagByReplicas = (readyReplicas, totalReplicas) => {
  if (totalReplicas === 0) return 'info'
  if (readyReplicas === 0) return 'danger'
  if (readyReplicas < totalReplicas) return 'warning'
  return 'success'
}

// 查看所有镜像
const viewAllImages = (row) => {
  currentWorkload.value = row
  allImagesDialogVisible.value = true
}

// 更新调度
const updateScheduling = (row) => {
  if (!canUpdateScheduling(row)) {
    ElMessage.warning('该工作负载不支持调度更新')
    return
  }
  
  currentWorkload.value = row
  schedulingDialogVisible.value = true
}

// 编辑工作负载YAML
const editWorkloadYaml = async (row) => {
  if (!canEditYaml(row)) {
    ElMessage.warning('系统工作负载不允许编辑YAML')
    return
  }

  currentWorkload.value = row
  const targetNamespace = getResourceNamespace(row)

  openYamlDialog({
    clusterId: selectedClusterId.value,
    resourceType: row.type || 'Workload',
    resourceName: row.name,
    editable: true,
    fetchYaml: async (clusterId, resourceName) => {
      console.log('🔍 开始获取工作负载YAML...', row)

      try {
        const response = await k8sApi.getWorkloadYaml(clusterId, targetNamespace, row.type, resourceName)
        const responseData = response.data || response

        if (responseData.code === 200 || responseData.success) {
          return response
        }
      } catch (workloadError) {
        console.log('工作负载YAML获取失败，尝试通过Pod获取:', workloadError)
      }

      console.log('🔍 通过获取工作负载Pod列表来获取Pod YAML...')
      const detailResponse = await k8sApi.getWorkloadPods(clusterId, targetNamespace, row.type.toLowerCase(), resourceName)
      const detailData = detailResponse.data || detailResponse

      if ((detailData.code === 200 || detailData.success) && detailData.data && detailData.data.length > 0) {
        const firstPod = detailData.data[0]
        console.log('🔍 获取Pod YAML:', firstPod.name)

        const podYamlResponse = await k8sApi.getPodYaml(clusterId, targetNamespace, firstPod.name)
        const podYamlData = podYamlResponse.data || podYamlResponse

        if (podYamlData.code === 200 || podYamlData.success) {
          return {
            code: 200,
            data: {
              yamlContent: getYamlContentFromApiResponse(
                podYamlData,
                `# Pod YAML for ${firstPod.name}\n# 工作负载: ${resourceName} (${row.type})\n`
              )
            }
          }
        }

        throw new Error(podYamlData.message || 'Pod YAML获取失败')
      }

      console.log('⚠️ 没有找到Pod，生成基础YAML模板')
      ElMessage.warning('未找到实际YAML内容，显示基础模板')
      return {
        code: 200,
        data: {
          yamlContent: generateWorkloadYamlTemplate(row)
        }
      }
    },
    updateYaml: async (clusterId, resourceName, yamlContent) => {
      const { workloadType, workloadName } = parseWorkloadFromYaml(yamlContent)
      return k8sApi.updateWorkloadYaml(
        clusterId,
        targetNamespace,
        workloadType,
        workloadName || resourceName,
        yamlContent
      )
    },
    fallbackYaml: generateWorkloadYamlTemplate(row),
    loadErrorMessage: '获取工作负载YAML失败',
    saveSuccessMessage: `${row.name} YAML配置保存成功`,
    saveErrorMessage: 'YAML配置保存失败',
    onSaved: handleQuery
  })
}

// 生成工作负载YAML模板
const generateWorkloadYamlTemplate = (workload) => {
  const kind = workload.type.charAt(0).toUpperCase() + workload.type.slice(1)
  const targetNamespace = getResourceNamespace(workload)
  return `apiVersion: apps/v1
kind: ${kind}
metadata:
  name: ${workload.name}
  namespace: ${targetNamespace}
  labels:
    app: ${workload.name}
spec:
  replicas: ${workload.totalReplicas || 1}
  selector:
    matchLabels:
      app: ${workload.name}
  template:
    metadata:
      labels:
        app: ${workload.name}
    spec:
      containers:
      - name: ${workload.name}
        image: nginx:latest
        ports:
        - containerPort: 80
---
# 注意: 这是一个基础模板，请根据实际需求修改
# 工作负载类型: ${workload.type}
# 当前状态: ${workload.status}
# 副本数: ${workload.replicas}`
}

onMounted(async () => {
  try {
    console.log('🚀 开始加载k8s工作负载页面')
    const startTime = Date.now()
    restoreTabState()
    
    // 加载集群列表
    console.log('📡 正在加载集群列表...')
    await fetchClusterList()
    console.log('✅ 集群列表加载完成，耗时:', Date.now() - startTime + 'ms')
    
    // 如果有选中的集群，立即开始加载数据
    if (selectedClusterId.value) {
      console.log('🔄 开始并行加载命名空间和工作负载数据')
      fetchNodeOptions(selectedClusterId.value).catch(error => {
        console.error('节点列表初始化加载失败:', error)
      })
      
      // 命名空间加载已移至 NamespaceSelector 组件
      
      // 如果有默认命名空间，立即开始查询工作负载
      if (queryParams.namespace) {
        console.log('📦 立即开始查询工作负载:', queryParams.namespace)
        // 不等待命名空间，直接查询工作负载
        handleQuery().catch(error => {
          console.error('工作负载初始查询失败:', error)
        })
      }
    }
    
    console.log('🎉 页面初始化完成，总耗时:', Date.now() - startTime + 'ms')
  } catch (error) {
    console.error('页面初始化失败:', error)
  }
})

onUnmounted(() => {
  closeAiAnalysisStream()
  stopAiAnalysisPolling()
})

watch(isPodsTab, (visible) => {
  if (!visible) {
    queryParams.nodeName = ''
    return
  }

  if (selectedClusterId.value && currentClusterNodeOptions.value.length === 0) {
    fetchNodeOptions(selectedClusterId.value).catch(error => {
      console.error('切换到 Pods 标签后加载节点列表失败:', error)
    })
    return
  }

  clearInvalidSelectedNode()
})

// 辅助函数
const formatCpu = (cpuStr) => {
  if (!cpuStr || cpuStr === '0' || cpuStr === '') return '-'
  return cpuStr
}

const formatMemory = (memoryStr) => {
  if (!memoryStr || memoryStr === '0' || memoryStr === '') return '-'

  if (memoryStr.endsWith('Ki')) {
    const kb = parseInt(memoryStr.replace('Ki', ''))
    if (kb < 1024) return memoryStr
    const mb = (kb / 1024).toFixed(1)
    return `${mb}Mi`
  }

  if (memoryStr.endsWith('Mi')) {
    const mb = parseInt(memoryStr.replace('Mi', ''))
    if (mb < 1024) return memoryStr
    const gb = (mb / 1024).toFixed(1)
    return `${gb}Gi`
  }

  if (memoryStr.endsWith('Gi')) {
    return memoryStr
  }

  // 如果没有单位，假设是字节
  const bytes = parseInt(memoryStr)
  if (!isNaN(bytes) && bytes > 0) {
    if (bytes < 1024) return `${bytes}B`
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}Ki`
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)}Mi`
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)}Gi`
  }

  return memoryStr || '-'
}

const getImageTag = (image) => {
  if (!image) return 'latest'
  const parts = image.split(':')
  return parts.length > 1 ? parts[parts.length - 1] : 'latest'
}

const getImageRegistry = (image) => {
  if (!image) return 'docker.io'
  const parts = image.split('/')
  if (parts.length === 1) return 'docker.io'
  if (parts[0].includes('.') || parts[0].includes(':')) {
    return parts[0]
  }
  return 'docker.io'
}

// 提交调度配置
const submitScheduling = async () => {
  try {
    const updateData = {
      template: {
        nodeSelector: currentWorkload.value.nodeSelector || {},
        tolerations: currentWorkload.value.tolerations || []
      }
    }

    const response = await k8sApi.updateDeployment(
      selectedClusterId.value,
      getResourceNamespace(currentWorkload.value),
      currentWorkload.value.name,
      updateData
    )

    const responseData = response.data || response
    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(`${currentWorkload.value.name} 调度配置更新成功`)
      schedulingDialogVisible.value = false
      handleQuery() // 刷新列表
    } else {
      ElMessage.error(responseData.message || '调度配置更新失败')
    }
  } catch (error) {
    console.error('调度配置更新失败:', error)
    ElMessage.error('调度配置更新失败，请检查网络连接')
  }
}

// 从YAML内容中解析工作负载信息
const parseWorkloadFromYaml = (yamlContent) => {
  try {
    const yamlObj = yaml.load(yamlContent)
    if (!yamlObj || typeof yamlObj !== 'object') {
      throw new Error('无效的YAML格式')
    }

    const workloadType = yamlObj.kind?.toLowerCase()
    const workloadName = yamlObj.metadata?.name

    if (!workloadType || !workloadName) {
      throw new Error('YAML中缺少kind或metadata.name字段')
    }

    return { workloadType, workloadName }
  } catch (error) {
    console.error('解析YAML失败:', error)
    throw error
  }
}

const deleteWorkload = async (row) => {
  if (!canDelete(row)) {
    ElMessage.warning('系统工作负载不允许删除')
    return
  }

  if (isPodRow(row)) {
    const success = await deletePodResource(row)
    if (success) {
      ElMessage.success(`Pod "${row.name}" 删除成功`)
      await handleQuery()
    }
    return
  }

  try {
    await ElMessageBox.confirm(
      `确定要删除 ${getWorkloadTypeName(row.type)} "${row.name}" 吗？此操作不可恢复！`,
      '删除确认',
      {
        confirmButtonText: '确定删除',
        cancelButtonText: '取消',
        type: 'warning',
        dangerouslyUseHTMLString: false
      }
    )

    console.log('🗑️ 开始删除工作负载:', {
      clusterId: selectedClusterId.value,
      namespace: getResourceNamespace(row),
      workloadType: row.type.toLowerCase(),
      workloadName: row.name
    })

    const response = row.type.toLowerCase() === 'deployment'
      ? await k8sApi.deleteDeployment(selectedClusterId.value, getResourceNamespace(row), row.name)
      : await k8sApi.deleteWorkload(
          selectedClusterId.value,
          getResourceNamespace(row),
          row.type.toLowerCase(),
          row.name
        )

    console.log('📤 删除API响应:', response)

    const responseData = response.data || response
    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(`${getWorkloadTypeName(row.type)} "${row.name}" 删除成功`)
      // 刷新工作负载列表
      await handleQuery()
    } else {
      ElMessage.error(responseData.message || `删除 ${getWorkloadTypeName(row.type)} 失败`)
    }
  } catch (error) {
    if (error === 'cancel') {
      ElMessage.info('已取消删除操作')
    } else {
      console.error('删除工作负载失败:', error)
      ElMessage.error(`删除失败: ${error.message || '请检查网络连接'}`)
    }
  }
}
</script>

<template>
  <div class="k8s-workloads-management">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/k8s.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>K8s 工作负载管理</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>统一查看 Deployment、StatefulSet、DaemonSet、Job、CronJob 与 Pod 状态。</p>
      </div>
      <div class="header-actions">
        <ClusterSelector
          v-model="selectedClusterId"
          width="220px"
          size="small"
          margin-right="0"
          theme="cluster"
          :disabled="batchDeleteLoading"
          @change="handleClusterChange"
        />

        <NamespaceSelector
          v-model="queryParams.namespace"
          :cluster-id="selectedClusterId"
          filterable
          width="240px"
          size="small"
          theme="namespace"
          :disabled="batchDeleteLoading"
          @change="handleNamespaceChange"
        />
      </div>
    </header>

    <section class="workloads-card page-card">
      
        <!-- 搜索表单 -->
        <div class="search-section">
          <el-form :inline="true" :model="queryParams" class="search-form">
          <el-form-item :label="workloadNameLabel">
            <el-input
              v-model="queryParams.name"
              :placeholder="workloadNamePlaceholder"
              clearable
              class="search-name-input"
              style="width: 250px"
              @keyup.enter="handleSearch"
              @clear="handleSearch"
            >
              <template #prefix>
                <el-icon class="search-name-icon"><Search /></el-icon>
              </template>
            </el-input>
          </el-form-item>
          <el-form-item label="展示范围">
            <el-select
              v-model="queryParams.scope"
              placeholder="请选择展示范围"
              class="scope-select"
              style="width: 220px"
              @change="handleScopeChange"
            >
              <template #prefix>
                <el-icon class="scope-select-icon"><Warning /></el-icon>
              </template>
              <el-option
                v-for="option in workloadScopeOptions"
                :key="option.value"
                :label="option.label"
                :value="option.value"
              />
            </el-select>
          </el-form-item>
          <el-form-item v-if="isPodsTab" label="节点">
            <el-select
              v-model="queryParams.nodeName"
              placeholder="请选择节点"
              clearable
              filterable
              :loading="nodesLoading"
              class="scope-select"
              style="width: 240px"
              @change="handleNodeChange"
              @clear="handleNodeChange"
            >
              <el-option
                v-for="option in currentClusterNodeOptions"
                :key="`${option.clusterId}-${option.value}`"
                :label="option.label"
                :value="option.value"
              />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :icon="Search" class="query-button" @click="handleSearch">
              查询
            </el-button>
            <el-button type="warning" :icon="Refresh" class="refresh-button" @click="handleRefresh">
              刷新
            </el-button>
            <el-button type="warning" :icon="Monitor" class="secondary-action" @click="navigateToMonitoring">
              监控仪表板
            </el-button>
            <el-button
              :icon="Plus"
              type="success"
              class="primary-action"
              @click="showCreatePodDialog"
            >
              创建工作负载
            </el-button>
          </el-form-item>
        </el-form>

        <!-- 工作负载类型标签页 -->
        <div class="workload-type-section">
          <el-tabs v-model="activeTab" @tab-change="handleTabChange">
            <el-tab-pane
              v-for="option in workloadTypeOptions"
              :key="option.value"
              :label="option.label"
              :name="option.value"
            />
          </el-tabs>
        </div>
      </div>

      <!-- 工作负载列表表格 -->
      <div class="table-section">
        <div v-if="isPodsTab" class="table-toolbar">
          <div class="table-toolbar-left">
            <span class="selection-text">已选 {{ selectedPods.length }} 个 Pod</span>
          </div>
          <div class="table-toolbar-right">
            <el-button
              type="danger"
              size="small"
              :disabled="!selectedPods.length || batchDeleteLoading"
              :loading="batchDeleteLoading"
              @click="handleBatchDeletePods"
            >
              批量删除
            </el-button>
          </div>
        </div>
        <el-table
          ref="workloadTableRef"
          :data="tableData"
          v-loading="loading"
          :empty-text="workloadTableEmptyText"
          stripe
          style="width: 100%"
          :row-key="getWorkloadRowKey"
          :class="['workloads-table', { 'pods-workloads-table': isPodsTab }]"
          @selection-change="handlePodSelectionChange"
        >
          <el-table-column
            v-if="isPodsTab"
            type="selection"
            width="52"
            :selectable="isPodSelectable"
            reserve-selection
          />
          <el-table-column
            prop="name"
            label="名称"
            :min-width="isPodsTab ? 280 : 220"
          >
            <template #default="{ row }">
              <div class="workload-name-container">
                <img src="@/assets/image/k8s.svg" alt="k8s" class="k8s-icon" />
                <div class="workload-info">
                  <button
                    type="button"
                    class="workload-name workload-name-link"
                    :title="row.name"
                    @click="navigateToPodDetail(row)"
                  >
                    {{ row.name }}
                  </button>
                </div>
              </div>
            </template>
          </el-table-column>
          
          <el-table-column
            :label="replicaColumnLabel"
            :width="isPodsTab ? 132 : undefined"
            min-width="120"
            align="left"
            header-align="left"
          >
            <template #default="{ row }">
              <div class="pod-status-container">
                <el-tag
                  :type="getStatusTagTypeByRow(row)"
                  size="small"
                  class="pod-count-tag"
                >
                  {{ getReplicaColumnValue(row) }}
                </el-tag>
              </div>
            </template>
          </el-table-column>

          <el-table-column
            v-if="isPodsTab"
            prop="nodeName"
            label="所在节点"
            width="188"
            show-overflow-tooltip
            class-name="workload-node-column"
          >
            <template #default="{ row }">
              <span class="node-name-cell" :title="row.nodeName || '-'">{{ row.nodeName || '-' }}</span>
            </template>
          </el-table-column>
          
          <el-table-column v-if="!isPodsTab" label="Request/Limits" min-width="180">
            <template #default="{ row }">
              <div class="resource-info">
                <div class="resource-row">
                  <span class="resource-type">CPU:</span>
                  <span class="resource-values">
                    <span class="request-value">{{ formatCpu(row.resources?.requests?.cpu) }}</span>
                    <span class="separator">/</span>
                    <span class="limit-value">{{ formatCpu(row.resources?.limits?.cpu) }}</span>
                  </span>
                </div>
                <div class="resource-row">
                  <span class="resource-type">Mem:</span>
                  <span class="resource-values">
                    <span class="request-value">{{ formatMemory(row.resources?.requests?.memory) }}</span>
                    <span class="separator">/</span>
                    <span class="limit-value">{{ formatMemory(row.resources?.limits?.memory) }}</span>
                  </span>
                </div>
              </div>
            </template>
          </el-table-column>
          
          <el-table-column
            label="镜像"
            :min-width="isPodsTab ? 310 : 300"
            class-name="workload-image-column"
          >
            <template #default="{ row }">
              <div class="images-list">
                <span v-if="!row.images.length" class="no-update">-</span>
                <div
                  v-for="(image, index) in row.images.slice(0, 1)"
                  :key="index"
                  class="image-tag-wrapper"
                  @click="copyToClipboard(image, '镜像地址已复制')"
                >
                  <el-icon class="copy-icon"><DocumentCopy /></el-icon>
                  <span class="full-image-name" :title="image">{{ image }}</span>
                </div>
                <el-button
                  v-if="row.images.length > 1"
                  type="text"
                  size="small"
                  class="more-images-btn"
                  @click="viewAllImages(row)"
                >
                  +{{ row.images.length - 1 }}个镜像
                </el-button>
              </div>
            </template>
          </el-table-column>
          
          <el-table-column
            label="创建时间"
            :width="isPodsTab ? 168 : undefined"
            min-width="150"
            align="left"
            header-align="left"
            class-name="workload-time-column"
          >
            <template #default="{ row }">
              <div class="time-info">
                <span class="datetime-text">{{ formatDateTime(row.updateTime) }}</span>
              </div>
            </template>
          </el-table-column>
          
          <el-table-column
            label="更新时间"
            :width="isPodsTab ? 168 : undefined"
            min-width="150"
            align="left"
            header-align="left"
            class-name="workload-time-column"
          >
            <template #default="{ row }">
              <div class="time-info">
                <span v-if="row.updatedAt" class="datetime-text">
                  {{ formatDateTime(row.updatedAt) }}
                </span>
                <span v-else class="no-update">-</span>
              </div>
            </template>
          </el-table-column>
          
          <el-table-column
            label="操作"
            :width="isPodsTab ? 210 : 250"
            fixed="right"
            align="left"
            header-align="left"
            class-name="workload-operation-column"
          >
            <template #default="{ row }">
              <div class="operation-buttons">
                <button
                  type="button"
                  class="operation-text logs"
                  @click="navigateToPodLogs(row)"
                >日志</button>
                <button
                  type="button"
                  class="operation-text ai"
                  @click="openAiAnalysisDialog(row)"
                >AI分析</button>

                <el-dropdown @command="(cmd) => handleMoreCommand(row, cmd)" trigger="click">
                  <button type="button" class="operation-text more" v-authority="getMoreActionPermissions(row)">
                    更多
                  </button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="terminal">终端</el-dropdown-item>
                      <el-dropdown-item
                        v-if="!isPodRow(row)"
                        command="scale"
                        :disabled="!canScale(row)"
                      >伸缩</el-dropdown-item>
                      <el-dropdown-item
                        v-if="!isPodRow(row)"
                        command="restart"
                        :disabled="!canRestart(row)"
                      >重构</el-dropdown-item>
                      <el-dropdown-item
                        v-if="!isPodRow(row)"
                        command="schedule"
                        :disabled="!canUpdateScheduling(row)"
                      >调度更新</el-dropdown-item>
                      <el-dropdown-item
                        v-if="!isPodRow(row)"
                        command="history"
                        :disabled="!canViewDeploymentHistory(row)"
                      >历史版本</el-dropdown-item>
                      <el-dropdown-item
                        command="yaml"
                        :disabled="!canEditYaml(row)"
                      >YAML编辑</el-dropdown-item>
                      <el-dropdown-item
                        command="delete"
                        v-authority="getDeletePermission(row)"
                        :disabled="!canDelete(row)"
                        divided
                      >删除</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="pagination-wrapper" v-if="total > 0">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="total"
          layout="total, sizes, prev, pager, next, jumper"
          background
          @current-change="handlePageChange"
          @size-change="handleSizeChange"
        />
      </div>
    </section>

    <el-dialog
      v-model="aiAnalysisVisible"
      :title="aiAnalysisTitle"
      width="min(980px, calc(100vw - 32px))"
      top="4vh"
      class="workload-ai-analysis-dialog"
      modal-class="workload-ai-dialog-overlay"
      append-to-body
      @closed="handleAiAnalysisDialogClosed"
    >
      <div class="workload-ai-analysis">
        <section class="ai-task-question-panel">
          <div class="ai-task-question-head">
            <div class="ai-analysis-title-wrap">
              <div class="ai-analysis-mark">
                <el-icon><MagicStick /></el-icon>
              </div>
              <div>
                <div class="ai-analysis-title">AI 分析任务</div>
                <div class="ai-analysis-subtitle">{{ aiAnalysisQuestion }}</div>
              </div>
            </div>
            <el-tag :type="aiAnalysisStatusMeta.type" effect="plain">
              {{ aiAnalysisStatusMeta.label }}
            </el-tag>
          </div>

          <div class="ai-task-object-grid">
            <div
              v-for="item in aiAnalysisObjectFields"
              :key="item.label"
              class="ai-task-object-item"
            >
              <span>{{ item.label }}</span>
              <strong>{{ item.value }}</strong>
            </div>
          </div>

          <div class="ai-task-prompt-row">
            <div>
              <div class="ai-analysis-eyebrow">QUESTION</div>
              <div class="ai-analysis-section-title">分析关注点</div>
            </div>
            <el-input
              v-model="aiAnalysisPrompt"
              placeholder="补充你希望 AI 重点关注的问题"
              class="ai-analysis-prompt"
            />
          </div>
        </section>

        <section class="ai-task-stream-panel">
          <div class="ai-analysis-progress-head">
            <div>
              <div class="ai-analysis-eyebrow">TASK FLOW</div>
              <div class="ai-analysis-section-title">实时任务流</div>
            </div>
            <span class="ai-task-stream-status">{{ aiAnalysisMessage || '等待任务进度' }}</span>
          </div>

          <el-progress
            :percentage="aiAnalysisProgress"
            :status="aiAnalysisProgressStatus"
            :stroke-width="8"
            class="ai-analysis-progress"
          />

          <div class="ai-task-step-list">
            <div
              v-for="(step, index) in aiAnalysisSteps"
              :key="step.key"
              class="ai-task-step"
              :class="`is-${step.status}`"
            >
              <div class="ai-task-step-index">
                <span>{{ index + 1 }}</span>
              </div>
              <div class="ai-task-step-main">
                <strong>{{ step.title }}</strong>
              </div>
              <span class="ai-task-step-status">{{ getAiTaskStepStatusText(step.status) }}</span>
            </div>
          </div>
        </section>

        <section class="ai-final-result-panel">
          <template v-if="shouldShowAiAnalysisResult">
            <div class="ai-analysis-section-head">
              <div>
                <div class="ai-analysis-eyebrow">DIAGNOSIS</div>
                <div class="ai-analysis-section-title">最终结果</div>
              </div>
              <el-tag type="primary" effect="plain">AI 结果</el-tag>
            </div>

            <div class="ai-final-diagnosis-card">
              <div class="ai-final-card-head">
                <div>
                  <strong>{{ aiPortDiagnosis ? '问题已定位' : '诊断结论' }}</strong>
                  <span>{{ aiPortDiagnosis ? '日志协议特征与 Kubernetes 端口配置综合判断' : '基于状态、日志、事件和资源信号综合判断' }}</span>
                </div>
                <el-tag v-if="aiFinalSeverity" size="small" type="warning" effect="plain">
                  {{ aiFinalSeverity }}
                </el-tag>
              </div>

              <div class="ai-final-conclusion-block" :class="{ 'is-confirmed': aiPortDiagnosis?.confirmed }">
                <div class="ai-result-section-title">
                  <span>1</span>
                  <strong>问题描述</strong>
                </div>
                <p class="ai-final-conclusion-text">
                  <AiRichText :text="aiProblemDescription" />
                </p>
              </div>

              <div class="ai-final-detail-section ai-cause-section">
                <div class="ai-result-section-title">
                  <span>2</span>
                  <strong>问题原因</strong>
                </div>
                <p class="ai-outcome-text"><AiRichText :text="aiProblemCause" /></p>
              </div>

              <div class="ai-final-detail-section ai-remediation-section">
                <div class="ai-result-section-title">
                  <span>3</span>
                  <strong>解决方案</strong>
                </div>
                <p v-if="aiPortDiagnosis" class="ai-remediation-lead">
                  修改 Service YAML，删除 debug/JDWP 端口，只保留实际业务需要的 HTTP、Dubbo 等端口。
                </p>
                <div v-if="aiCurrentServiceYaml || aiServiceFixYaml" class="ai-config-comparison">
                  <div v-if="aiCurrentServiceYaml" class="ai-config-block is-current">
                    <div class="ai-config-block-head">
                      <strong>修改前</strong>
                      <span>包含 debug/JDWP</span>
                    </div>
                    <pre><code>{{ aiCurrentServiceYaml }}</code></pre>
                  </div>
                  <div v-if="aiServiceFixYaml" class="ai-config-block is-fixed">
                    <div class="ai-config-block-head">
                      <strong>修改后</strong>
                      <span>删除 debug/JDWP</span>
                    </div>
                    <pre><code>{{ aiServiceFixYaml }}</code></pre>
                  </div>
                </div>
                <div v-if="aiIngressFixYaml" class="ai-ingress-fix-row">
                  <div>
                    <strong>同时核对 Ingress / 网关</strong>
                    <p>后端端口应显式引用 Service 的 HTTP 端口，不能继续引用已删除的 debug 端口。</p>
                  </div>
                  <pre><code>{{ aiIngressFixYaml }}</code></pre>
                  <p v-if="aiIngressFixAlternative" class="ai-config-alternative">{{ aiIngressFixAlternative }}</p>
                </div>
                <ol v-if="aiFinalRemediation.length" class="ai-remediation-list">
                  <li
                    v-for="(item, index) in aiFinalRemediation"
                    :key="index"
                  ><AiRichText :text="item" /></li>
                </ol>
                <p v-else-if="!aiServiceFixYaml && !aiIngressFixYaml" class="ai-outcome-text">暂未返回明确的解决方案。</p>
                <p v-if="aiNodePortSwitchText" class="ai-outcome-text is-warning">{{ aiNodePortSwitchText }}</p>
                <p v-if="aiPortDiagnosis" class="ai-outcome-text">
                  修改后重新访问 <code>{{ aiObservedRequestPath }}</code>；JDWP 日志不应再出现 HTTP 请求握手失败。
                </p>
              </div>

              <div class="ai-final-detail-section ai-final-outcome-section">
                <div class="ai-result-section-title">
                  <span>4</span>
                  <strong>总结结论</strong>
                </div>
                <ul class="ai-summary-conclusion-list">
                  <li v-for="(item, index) in aiSummaryConclusionLines" :key="index">
                    <AiRichText :text="item" />
                  </li>
                </ul>
                <template v-if="aiPortDiagnosis">
                  <p class="ai-outcome-lead">Service 更新后只暴露业务端口：</p>
                  <ul v-if="aiServiceExposureLines.length" class="ai-service-exposure-list">
                    <li v-for="item in aiServiceExposureLines" :key="item">{{ item }}</li>
                  </ul>
                  <p class="ai-outcome-text">Service 更新后无需重启 Pod，但必须确认 Ingress / 网关没有继续引用原 debug Service 端口或 NodePort。</p>
                </template>
              </div>
            </div>
          </template>

          <div v-else class="ai-final-waiting">
            <div class="ai-final-waiting-icon">
              <el-icon><MagicStick /></el-icon>
            </div>
            <div>
              <strong>{{ aiAnalysisLoading ? '正在生成最终结论' : '等待分析结果' }}</strong>
              <p>任务流会先实时更新进度，收到 result 后在这里展示精简诊断结论。</p>
            </div>
          </div>
        </section>
      </div>

      <template #footer>
        <div class="ai-analysis-footer">
          <el-button @click="aiAnalysisVisible = false">关闭</el-button>
          <el-button
            :disabled="!aiAnalysisCopyText"
            @click="copyToClipboard(aiAnalysisCopyText, 'AI分析结果已复制')"
          >
            复制结果
          </el-button>
          <el-button
            type="primary"
            :icon="MagicStick"
            :loading="aiAnalysisLoading"
            @click="rerunAiAnalysis"
          >
            重新分析
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 扩缩容对话框 -->
    <el-dialog
      v-model="scaleDialogVisible"
      :title="`扩缩容 - ${currentWorkload.name || ''}`"
      width="400px"
      class="scale-dialog"
      modal-class="k8s-workload-dialog-overlay"
    >
      <el-form :model="scaleForm" label-width="80px" class="workload-dialog-form">
        <el-form-item label="副本数" required>
          <el-input-number
            v-model="scaleForm.replicas"
            :min="0"
            :max="100"
            style="width: 100%"
          />
          <div class="form-tip">期望副本数: {{ currentWorkload.desiredReplicas ?? currentWorkload.totalReplicas }}</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="scaleDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitScale">确定</el-button>
        </span>
      </template>
    </el-dialog>
    <!-- 所有镜像查看对话框 -->
    <el-dialog
      v-model="allImagesDialogVisible"
      :title="`镜像列表 - ${currentWorkload.name || ''}`"
      width="800px"
      class="all-images-dialog"
      modal-class="k8s-workload-dialog-overlay"
    >
      <div class="images-view-content" v-if="currentWorkload.images">
        <div class="images-section">
          <h4>容器镜像 ({{ currentWorkload.images?.length || 0 }}个)</h4>
          <div class="all-images-list">
            <el-card
              v-for="(image, index) in currentWorkload.images"
              :key="index"
              class="image-card"
              shadow="hover"
            >
              <div class="image-info">
                <div class="image-name">
                  <el-icon class="image-icon"><Connection /></el-icon>
                  <span class="full-image-name">{{ image }}</span>
                </div>
                <div class="image-actions">
                  <el-button
                    type="primary"
                    size="small"
                    :icon="DocumentCopy"
                    @click="copyToClipboard(image, '镜像地址已复制')"
                  >
                    复制
                  </el-button>
                </div>
              </div>
              <div class="image-details">
                <el-tag size="small" type="info">{{ getImageTag(image) }}</el-tag>
                <el-tag size="small" type="success">{{ getImageRegistry(image) }}</el-tag>
              </div>
            </el-card>
          </div>
        </div>
        
        <!-- 没有镜像的提示 -->
        <div v-if="!currentWorkload.images || currentWorkload.images.length === 0" class="no-images">
          <el-empty description="该工作负载没有镜像" :image-size="60" />
        </div>
      </div>
    </el-dialog>

    <!-- Deployment历史版本对话框 -->
    <el-dialog
      v-model="deploymentHistoryDialogVisible"
      :title="`历史版本 - ${deploymentHistory.deploymentName || ''}`"
      width="980px"
      class="deployment-history-dialog"
      modal-class="k8s-workload-dialog-overlay"
    >
      <div class="history-summary">
        <div class="summary-item">
          <span class="summary-label">命名空间</span>
          <span class="summary-value">{{ deploymentHistory.namespace || '-' }}</span>
        </div>
        <div class="summary-item">
          <span class="summary-label">当前版本</span>
          <span class="summary-value">Revision {{ deploymentHistory.currentRevision || '-' }}</span>
        </div>
        <div class="summary-item">
          <span class="summary-label">实际保留</span>
          <span class="summary-value">{{ deploymentHistory.totalRevisions }} 个</span>
        </div>
        <div class="summary-item">
          <span class="summary-label">保留上限</span>
          <span class="summary-value">{{ deploymentHistory.revisionHistoryLimit }} 个</span>
        </div>
      </div>

      <el-table
        :data="deploymentHistory.revisions"
        v-loading="deploymentHistoryLoading"
        empty-text="暂无Deployment历史版本"
        class="history-table"
        style="width: 100%"
      >
        <el-table-column label="版本" width="120">
          <template #default="{ row }">
            <div class="revision-cell">
              <span>Revision {{ row.revision }}</span>
              <el-tag v-if="row.status === 'current'" size="small" type="success">当前</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">
            <span>{{ formatDateTime(row.creationTime) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="镜像" min-width="260">
          <template #default="{ row }">
            <div class="revision-images">
              <el-tag
                v-for="image in row.images"
                :key="image"
                size="small"
                class="revision-image-tag"
                @click="copyToClipboard(image, '镜像地址已复制')"
              >
                {{ image }}
              </el-tag>
              <span v-if="!row.images.length" class="no-update">-</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="副本" width="160">
          <template #default="{ row }">
            <span>
              {{ row.replicasSummary.ready || 0 }}/{{ row.replicasSummary.desired || 0 }}
              <span class="replica-extra">可用 {{ row.replicasSummary.available || 0 }}</span>
            </span>
          </template>
        </el-table-column>
        <el-table-column label="变更原因" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">
            <span>{{ row.changeReason || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="110" fixed="right">
          <template #default="{ row }">
            <el-button
              link
              type="primary"
              size="small"
              :loading="rollbackLoading"
              :disabled="!canRollbackDeploymentRevision(row)"
              @click="rollbackDeploymentToRevision(row)"
            >回滚</el-button>
          </template>
        </el-table-column>
      </el-table>

      <template #footer>
        <span class="dialog-footer">
          <el-button @click="deploymentHistoryDialogVisible = false">关闭</el-button>
          <el-button
            type="warning"
            class="ops-refresh-button"
            :loading="deploymentHistoryLoading"
            @click="fetchDeploymentHistory(currentWorkload)"
          >刷新</el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 调度更新对话框 -->
    <el-dialog
      v-model="schedulingDialogVisible"
      :title="`更新调度 - ${currentWorkload.name || ''}`"
      width="500px"
      class="scheduling-dialog"
      modal-class="k8s-workload-dialog-overlay"
    >
      <el-form :model="currentWorkload" label-width="120px" class="workload-dialog-form">
        <el-form-item label="节点选择器">
          <el-input
            v-model="currentWorkload.nodeSelector"
            placeholder="如: kubernetes.io/arch=amd64"
            style="width: 100%"
          />
          <div class="form-tip">格式: key=value，多个用逗号分隔</div>
        </el-form-item>
        <el-form-item label="节点亲和性">
          <el-select
            v-model="currentWorkload.nodeAffinity"
            placeholder="请选择节点亲和性"
            style="width: 100%"
          >
            <el-option label="无要求" value="none" />
            <el-option label="偏好调度" value="preferred" />
            <el-option label="必须调度" value="required" />
          </el-select>
        </el-form-item>
        <el-form-item label="Pod反亲和性">
          <el-switch
            v-model="currentWorkload.podAntiAffinity"
            active-text="启用"
            inactive-text="禁用"
          />
          <div class="form-tip">避免Pod调度到同一节点</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="schedulingDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitScheduling">更新调度</el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 创建工作负载对话框 -->
    <CreatePodDialog
      ref="createPodDialogRef"
      :visible="createPodDialogVisible"
      :cluster-id="selectedClusterId"
      :cluster-name="clusterList.find(c => c.id === selectedClusterId)?.name"
      :namespace="queryParams.namespace"
      @update:visible="createPodDialogVisible = $event"
      @close="createPodDialogVisible = false"
      @preview="handlePodPreview"
      @create="handlePodCreate"
    />
  </div>
</template>


<style scoped>
.k8s-workloads-management {
  padding: 5px;
  min-height: 100vh;
  background: #eef1f6;
}

.workloads-card {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-radius: 16px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.title {
  font-size: 20px;
  font-weight: 600;
  color: #6b9bd1;
  background: linear-gradient(45deg, #667eea, #764ba2);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.header-actions {
  display: flex;
  gap: 12px;
  align-items: center;
}

.search-section {
  margin-bottom: 24px;
  padding: 20px;
  background: rgba(103, 126, 234, 0.05);
  border-radius: 12px;
  border: 1px solid rgba(103, 126, 234, 0.1);
}

.search-form .el-form-item {
  margin-bottom: 0;
  margin-right: 16px;
}

.search-form .el-form-item__label {
  color: #606266;
  font-weight: 500;
}

.search-name-icon {
  color: #409eff;
  font-size: 16px;
}

.search-name-input :deep(.el-input__wrapper) {
  min-height: 40px;
  padding: 0 12px;
  background: rgba(64, 158, 255, 0.06);
  border-color: rgba(64, 158, 255, 0.22);
}

.search-name-input :deep(.el-input__wrapper:hover) {
  border-color: #409eff;
}

.search-name-input :deep(.el-input__wrapper.is-focus) {
  border-color: #409eff;
  box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.15);
}

.search-name-input :deep(.el-input__wrapper.is-disabled) {
  background: rgba(245, 247, 250, 0.95);
  border-color: rgba(230, 162, 60, 0.18);
  box-shadow: none;
}

.scope-select-icon {
  color: #e6a23c;
}

.scope-select :deep(.el-input__wrapper) {
  min-height: 40px;
  padding: 0 12px;
  background: rgba(230, 162, 60, 0.08);
  border-color: rgba(230, 162, 60, 0.25);
}

.scope-select :deep(.el-input__wrapper:hover) {
  border-color: #e6a23c;
}

.scope-select :deep(.el-input__wrapper.is-focus) {
  border-color: #e6a23c;
  box-shadow: 0 0 0 2px rgba(230, 162, 60, 0.18);
}

.search-mode-hint {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-top: 12px;
  padding: 8px 12px;
  border-radius: 999px;
  background: rgba(230, 162, 60, 0.12);
  border: 1px solid rgba(230, 162, 60, 0.2);
  color: #b88230;
  font-size: 12px;
  font-weight: 500;
}

/* 工作负载类型标签页样式 */
.workload-type-section {
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid rgba(103, 126, 234, 0.1);
}

.table-section {
  margin-top: 20px;
}

.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
  padding: 12px 0;
}

.workloads-table {
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
}

.workloads-table :deep(.el-table__header) {
  background: linear-gradient(135deg, #667eea, #764ba2);
}

.workloads-table :deep(.el-table__header th) {
  background: transparent !important;
  color: #000000 !important;
  font-weight: 700 !important;
  border-bottom: none;
  text-shadow: none;
}

.workloads-table :deep(.el-table__header th .cell) {
  color: #000000 !important;
  font-weight: 700 !important;
}

.workloads-table :deep(.el-table__row) {
  transition: background-color 0.2s ease;
}

.workloads-table :deep(.el-table__row:hover) {
  background-color: rgba(103, 126, 234, 0.05) !important;
  transform: none;
  box-shadow: none;
}

.workload-name-container {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: default;
}

.workload-name-container:hover {
  transform: none !important;
  background-color: transparent !important;
}

.workload-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.workload-name {
  font-weight: 600;
  color: #6b9bd1;
  font-size: 14px;
}

.workload-type-label {
  font-size: 12px;
  color: #E6A23C;
  font-weight: 500;
  pointer-events: none;
  user-select: none;
}

.workload-namespace-label {
  font-size: 12px;
  color: #909399;
  font-weight: 500;
}

.workload-type-tag {
  font-size: 11px;
  height: 18px;
  line-height: 16px;
  padding: 0 6px;
}

.pod-name-container {
  display: flex;
  align-items: center;
  gap: 10px;
}

.k8s-icon {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.1));
}

.workload-name-link {
  appearance: none;
  border: 0;
  background: transparent;
  padding: 0;
  font: inherit;
  color: #667eea;
  text-align: left;
  cursor: pointer;
  text-decoration: none;
  transition: all 0.3s ease;
}

.workload-name-link:hover,
.workload-name-link:focus {
  color: #764ba2;
  text-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.workload-name-link:focus {
  outline: none;
}

.pod-name {
  font-weight: 500;
  color: #6b9bd1;
}

.resource-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.resource-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

.resource-type {
  font-size: 12px;
  color: #909399;
  min-width: 35px;
  font-weight: 500;
}

.resource-values {
  display: flex;
  align-items: center;
  gap: 2px;
}

.request-value {
  font-size: 12px;
  color: #67c23a;
  font-weight: 500;
}

.separator {
  font-size: 12px;
  color: #dcdfe6;
  margin: 0 2px;
}

.limit-value {
  font-size: 12px;
  color: #e6a23c;
  font-weight: 500;
}

.resource-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.resource-label {
  font-size: 12px;
  color: #909399;
  min-width: 55px;
}

.resource-value {
  font-size: 12px;
  color: #606266;
  font-weight: 500;
}

.ip-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.ip-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.ip-label {
  font-size: 12px;
  color: #909399;
  min-width: 35px;
}

.ip-value {
  font-size: 12px;
  color: #606266;
  font-weight: 500;
}

.images-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.image-tag {
  cursor: pointer;
  transition: all 0.3s ease;
  display: flex;
  align-items: center;
  gap: 4px;
  max-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.image-tag:hover {
  transform: translateY(-2px);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
}

.copy-icon {
  font-size: 10px;
}

.more-images {
  cursor: default;
}

/* Pod状态样式 */
.pod-status-container {
  display: flex;
  align-items: center;
  justify-content: flex-start;
}

.pod-count-tag {
  transition: all 0.3s ease;
  padding: 6px 12px;
  font-weight: 600;
}

/* 镜像显示优化 */
.image-tag-wrapper {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  transition: all 0.3s ease;
  margin-bottom: 4px;
}

.image-tag-wrapper:hover {
  transform: translateY(-1px);
}

.image-tag-wrapper .full-image-name {
  font-family: 'Monaco', 'Courier New', monospace;
  font-size: 11px;
  color: #6b9bd1;
  word-break: break-all;
  line-height: 1.4;
  white-space: normal;
}

.image-tag-wrapper .copy-icon {
  color: #666;
  font-size: 12px;
  flex-shrink: 0;
}

.more-images-btn {
  color: #409eff;
  font-size: 12px;
  padding: 2px 6px;
  margin-left: 4px;
}

.more-images-btn:hover {
  color: #66b1ff;
  background-color: rgba(64, 158, 255, 0.1);
}

/* 时间显示 */
.time-info {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  text-align: left;
}

.datetime-text {
  font-family: 'Monaco', 'Courier New', monospace;
  font-size: 12px;
  color: #6b9bd1;
}

.no-update {
  color: #909399;
  font-size: 12px;
}

.running-time {
  font-size: 10px;
  color: #909399;
  line-height: 1.2;
}

.no-update {
  color: #c0c4cc;
  font-size: 12px;
}


.operation-buttons {
  display: flex;
  gap: 6px;
  align-items: center;
  justify-content: center;
  flex-wrap: nowrap;
}

.operation-buttons .el-button {
  transition: all 0.3s ease;
}

.operation-buttons :deep(.el-dropdown) {
  display: inline-flex;
}

.operation-buttons .el-button:hover:not(.is-disabled) {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

.operation-buttons .el-button.is-disabled {
  cursor: not-allowed;
  opacity: 0.5;
  background-color: #f5f7fa !important;
  border-color: #e4e7ed !important;
  color: #c0c4cc !important;
}

.operation-buttons .el-button.is-disabled:hover {
  transform: none;
  box-shadow: none;
}

/* 对话框样式 */
.yaml-dialog :deep(.el-dialog),
.scale-dialog :deep(.el-dialog),
.all-images-dialog :deep(.el-dialog),
.deployment-history-dialog :deep(.el-dialog) {
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
}

.yaml-dialog :deep(.el-dialog__header),
.scale-dialog :deep(.el-dialog__header),
.all-images-dialog :deep(.el-dialog__header),
.deployment-history-dialog :deep(.el-dialog__header) {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: white;
  border-top-left-radius: 16px;
  border-top-right-radius: 16px;
  padding: 20px 24px;
}

.yaml-dialog :deep(.el-dialog__title),
.scale-dialog :deep(.el-dialog__title),
.all-images-dialog :deep(.el-dialog__title),
.deployment-history-dialog :deep(.el-dialog__title) {
  color: white;
  font-weight: 600;
}

.log-controls,
.yaml-controls {
  margin-bottom: 16px;
  padding: 16px;
  background: rgba(103, 126, 234, 0.05);
  border-radius: 8px;
}

.log-content,
.yaml-content {
  background: #6b9bd1;
  color: #ecf0f1;
  padding: 16px;
  border-radius: 8px;
  max-height: 400px;
  overflow: auto;
}

.log-content pre,
.yaml-content pre {
  margin: 0;
  font-family: 'Courier New', monospace;
  font-size: 12px;
  line-height: 1.4;
  white-space: pre-wrap;
  word-wrap: break-word;
}

.form-tip {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
  line-height: 1.4;
}

.dialog-footer {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
}

/* 集群选择样式 */
.cluster-option {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.cluster-name {
  font-weight: 500;
  color: #6b9bd1;
}

.cluster-status-tag {
  margin-left: 8px;
}

/* 命名空间选择样式 */
.namespace-option {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.namespace-name {
  font-weight: 500;
  color: #6b9bd1;
}

.namespace-status-tag {
  margin-left: 8px;
}

/* 通用样式 */
.el-tag {
  font-weight: 500;
  border-radius: 8px;
  border: none;
}

.el-button {
  border-radius: 8px;
  font-weight: 500;
  transition: all 0.3s ease;
}

.el-button:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.el-input :deep(.el-input__wrapper),
.el-select :deep(.el-input__wrapper) {
  background: rgba(255, 255, 255, 0.8);
  border: 1px solid rgba(103, 126, 234, 0.2);
  border-radius: 8px;
  box-shadow: none;
  transition: all 0.3s ease;
}

.el-input :deep(.el-input__wrapper):hover,
.el-select :deep(.el-input__wrapper):hover {
  border-color: #c0c4cc;
}

.el-input :deep(.el-input__wrapper.is-focus),
.el-select :deep(.el-input__wrapper.is-focus) {
  border-color: #667eea;
  box-shadow: 0 0 0 2px rgba(103, 126, 234, 0.2);
  background: rgba(255, 255, 255, 1);
}

.el-loading-mask {
  background-color: rgba(103, 126, 234, 0.1);
  backdrop-filter: blur(4px);
}

/* 响应式设计 */
@media (max-width: 1200px) {
  .operation-buttons {
    gap: 4px;
  }
  
  .operation-buttons .el-button {
    margin: 1px;
  }
  
  .header-actions .el-select {
    min-width: 180px;
  }
}

@media (max-width: 768px) {
  .k8s-workloads-management {
    padding: 10px;
  }
  
  .search-form {
    flex-direction: column;
  }
  
  .search-form .el-form-item {
    margin-right: 0;
    margin-bottom: 12px;
  }
  
  .operation-buttons {
    flex-direction: column;
    gap: 4px;
  }
  
  .workloads-table :deep(.el-table__row:hover) {
    transform: none;
  }
}

/* 镜像对话框样式 */
.images-view-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.no-images {
  text-align: center;
  padding: 40px 20px;
  color: #909399;
}

/* 镜像对话框特殊样式 */
.all-images-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.image-card {
  border: 1px solid rgba(103, 126, 234, 0.2);
  border-radius: 8px;
  transition: all 0.3s ease;
}

.image-card:hover {
  border-color: #667eea;
  box-shadow: 0 4px 12px rgba(103, 126, 234, 0.15);
}

.image-info {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.image-name {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
}

.image-icon {
  color: #667eea;
  font-size: 16px;
  flex-shrink: 0;
}

.full-image-name {
  font-family: 'Monaco', 'Courier New', monospace;
  font-size: 13px;
  color: #6b9bd1;
  word-break: break-all;
  line-height: 1.4;
}

.image-actions {
  flex-shrink: 0;
}

.image-details {
  display: flex;
  gap: 8px;
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid #f0f0f0;
}

/* Unified workloads page skin */
.k8s-workloads-management {
  --aiops-bg-page: #f4f7fb;
  --aiops-surface-strong: rgba(255, 255, 255, 0.92);
  --aiops-text-primary: #172133;
  --aiops-text-secondary: #5f6b7c;
  --aiops-text-muted: #8d97a8;
  --aiops-border-cool: #dfe7f2;
  --aiops-border-mid: rgba(207, 217, 230, 0.92);
  --aiops-brand: #4c8bf5;
  --aiops-success: #3fb97f;
  --aiops-warning: #f3b24b;
  --aiops-danger: #ef6b73;
  --aiops-shadow-soft: 0 20px 60px rgba(17, 24, 39, 0.07);
  --workload-page-height-offset: 190px;
  min-height: calc(100vh - var(--workload-page-height-offset));
  padding: 6px;
  box-sizing: border-box;
  background:
    radial-gradient(circle at 16% 10%, rgba(76, 139, 245, 0.06), transparent 26%),
    linear-gradient(180deg, #f7f9fc 0%, var(--aiops-bg-page) 100%);
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.workloads-card {
  position: relative;
  display: flex;
  flex-direction: column;
  height: calc(100vh - 12px - var(--workload-page-height-offset));
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

.workloads-card::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 40px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.82);
  pointer-events: none;
}

.page-hero {
  position: relative;
  z-index: 1;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 18px;
  flex-shrink: 0;
  margin-bottom: 12px;
  padding: 0 8px 12px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.72);
}

.hero-title-group {
  min-width: 0;
}

.page-eyebrow {
  margin: 0 0 6px;
  color: var(--aiops-brand);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.6px;
}

.page-hero h1 {
  margin: 0;
  color: var(--aiops-text-primary);
  font-size: 24px;
  font-weight: 700;
  line-height: 1.2;
}

.page-description {
  margin: 7px 0 0;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.7;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.search-section {
  flex-shrink: 0;
  margin-bottom: 12px;
  padding: 14px 16px 8px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 20px;
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.82), rgba(247, 250, 255, 0.76));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
}

.search-form {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  row-gap: 12px;
}

.search-form :deep(.el-form-item) {
  margin-bottom: 0;
  margin-right: 14px;
}

.search-form :deep(.el-form-item__label) {
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

.search-name-input :deep(.el-input__wrapper),
.scope-select :deep(.el-input__wrapper) {
  min-height: 32px;
  padding: 0 11px;
  background: rgba(252, 253, 255, 0.98);
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 13px;
  box-shadow: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;
}

.search-name-input :deep(.el-input__wrapper:hover),
.scope-select :deep(.el-input__wrapper:hover) {
  border-color: #c0c4cc;
}

.search-name-input :deep(.el-input__wrapper.is-focus),
.scope-select :deep(.el-input__wrapper.is-focus) {
  border-color: rgba(120, 156, 221, 0.9);
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
  background: #ffffff;
}

.search-name-icon {
  color: var(--aiops-brand);
  font-size: 15px;
}

.scope-select-icon {
  color: var(--aiops-warning);
}

.query-button,
.primary-action {
  min-width: 76px;
  border: none;
  border-radius: 13px;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
  font-weight: 600;
  box-shadow: 0 10px 18px rgba(76, 139, 245, 0.16);
}

.reset-button,
.secondary-action {
  min-width: 76px;
  border: 1px solid rgba(214, 223, 235, 0.96);
  border-radius: 13px;
  background: rgba(255, 255, 255, 0.92);
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

.reset-button:hover,
.reset-button:focus,
.secondary-action:hover,
.secondary-action:focus {
  border-color: rgba(153, 181, 230, 0.95);
  color: var(--aiops-brand);
  background: #ffffff;
}

.workload-type-section {
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px solid rgba(223, 231, 242, 0.72);
}

.workload-type-section :deep(.el-tabs__header) {
  margin: 0;
}

.workload-type-section :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: rgba(223, 231, 242, 0.78);
}

.workload-type-section :deep(.el-tabs__item) {
  height: 34px;
  padding: 0 14px;
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

.workload-type-section :deep(.el-tabs__item.is-active) {
  color: var(--aiops-brand);
}

.workload-type-section :deep(.el-tabs__active-bar) {
  height: 2px;
  border-radius: 999px;
  background: var(--aiops-brand);
}

.table-section {
  flex: 1;
  min-height: 0;
  margin-top: 0;
  padding: 8px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.64);
  overflow: auto;
}

.table-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
  padding: 0 6px;
}

.table-toolbar-left,
.table-toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.selection-text {
  color: var(--aiops-text-secondary);
  font-size: 12px;
  font-weight: 600;
}

.workloads-table {
  min-width: 1320px;
  border: 1px solid rgba(223, 231, 242, 0.88);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.92);
  overflow: hidden;
  box-shadow: none;
}

.pods-workloads-table {
  min-width: 1360px;
}

.workloads-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.workloads-table :deep(.el-table__header) {
  background: transparent;
}

.workloads-table :deep(.el-table__header th),
.workloads-table :deep(.el-table__fixed-right-patch) {
  background: rgba(248, 250, 255, 0.98) !important;
  border-bottom: 1px solid rgba(223, 231, 242, 0.95);
  text-shadow: none;
}

.workloads-table :deep(.el-table__header th .cell) {
  color: var(--aiops-text-muted) !important;
  font-size: 12px;
  font-weight: 700 !important;
}

.workloads-table :deep(.el-table__cell) {
  border-bottom-color: rgba(15, 23, 42, 0.045);
  overflow: visible;
}

.workloads-table :deep(.el-table__cell .cell) {
  min-width: 0;
  overflow: hidden;
  padding-left: 8px;
  padding-right: 8px;
}

.workloads-table :deep(.el-table__row) {
  transition: background-color 0.2s ease;
}

.workloads-table :deep(.el-table__body tr:hover > td.el-table__cell) {
  background: rgba(76, 139, 245, 0.045) !important;
}

.pods-workloads-table :deep(.workload-node-column .cell) {
  padding-right: 6px;
}

.pods-workloads-table :deep(.workload-image-column .cell) {
  padding-left: 6px;
}

.workload-name-container {
  gap: 6px;
  min-width: 0;
}

.k8s-icon {
  width: 18px;
  height: 18px;
  filter: none;
}

.workload-info {
  gap: 3px;
  min-width: 0;
}

.workload-name {
  display: block;
  max-width: 100%;
  color: var(--aiops-brand);
  font-size: 13px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.workload-type-label {
  color: var(--aiops-warning);
  font-size: 11px;
  font-weight: 700;
}

.workload-namespace-label {
  color: var(--aiops-text-muted);
  font-size: 11px;
  font-weight: 600;
}

.resource-info {
  display: inline-flex;
  flex-direction: row;
  align-items: center;
  gap: 10px;
  max-width: 100%;
  white-space: nowrap;
}

.resource-row {
  gap: 6px;
  white-space: nowrap;
}

.resource-type {
  min-width: 30px;
  color: var(--aiops-text-muted);
  font-weight: 600;
}

.resource-values,
.request-value,
.limit-value,
.separator,
.datetime-text,
.full-image-name,
.image-tag-wrapper .full-image-name {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
}

.request-value {
  color: var(--aiops-success);
  font-size: 12px;
  font-weight: 700;
}

.limit-value {
  color: var(--aiops-warning);
  font-size: 12px;
  font-weight: 700;
}

.separator {
  color: #c0cad8;
}

.pod-status-container {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  max-width: 100%;
  white-space: nowrap;
}

.pod-count-tag {
  max-width: 100%;
  padding: 3px 9px;
  border-radius: 999px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
}

.node-name-cell {
  display: block;
  max-width: 100%;
  color: var(--aiops-text-secondary);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.images-list {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
}

.image-tag-wrapper {
  max-width: 100%;
  min-width: 0;
  margin-bottom: 0;
  padding: 2px 0;
  transition: opacity 0.2s ease;
}

.image-tag-wrapper:hover {
  transform: none;
  opacity: 0.82;
}

.image-tag-wrapper .full-image-name {
  display: block;
  min-width: 0;
  max-width: 100%;
  color: var(--aiops-text-secondary);
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  word-break: normal;
}

.image-tag-wrapper .copy-icon {
  color: var(--aiops-brand);
  flex: 0 0 auto;
}

.more-images-btn {
  height: auto;
  min-height: 0;
  padding: 0;
  color: var(--aiops-brand);
  font-size: 12px;
  font-weight: 700;
}

.more-images-btn:hover {
  background: transparent;
  color: #3f75df;
}

.datetime-text {
  display: block;
  max-width: 100%;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.operation-buttons {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: flex-start;
  flex-wrap: nowrap;
}

.operation-buttons :deep(.el-dropdown) {
  display: inline-flex;
}

.operation-buttons :deep(.el-button),
.operation-buttons .operation-text {
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
  cursor: pointer;
  appearance: none;
}

.operation-buttons :deep(.el-button:hover:not(.is-disabled)),
.operation-buttons .operation-text:hover:not(.is-disabled):not(:disabled) {
  transform: none;
  box-shadow: none;
  text-decoration: none;
  opacity: 0.82;
}

.operation-buttons :deep(.operation-text.logs) {
  color: var(--aiops-brand);
}

.operation-buttons :deep(.operation-text.ai) {
  color: var(--aiops-success);
}

.operation-buttons :deep(.operation-text.more) {
  color: #f3a33a;
}

.operation-buttons :deep(.operation-text.is-disabled) {
  color: var(--aiops-text-muted);
  opacity: 0.48;
}

.workload-ai-analysis-dialog :deep(.el-dialog) {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 8px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
}

.workload-ai-analysis-dialog :deep(.el-dialog__body) {
  max-height: calc(92vh - 132px);
  padding: 16px;
  overflow-y: auto;
  background:
    radial-gradient(circle at top left, rgba(76, 139, 245, 0.05), transparent 26%),
    linear-gradient(180deg, #f8fafc 0%, #f5f8fc 100%);
}

.workload-ai-analysis {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.ai-analysis-hero-panel,
.ai-analysis-input-panel,
.ai-analysis-progress-panel,
.ai-analysis-section {
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.86);
}

.ai-analysis-hero-panel {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 16px;
}

.ai-analysis-title-wrap,
.ai-analysis-meta,
.ai-analysis-actions,
.ai-analysis-progress-head,
.ai-analysis-section-head,
.ai-analysis-footer {
  display: flex;
  align-items: center;
}

.ai-analysis-title-wrap {
  min-width: 0;
  gap: 12px;
}

.ai-analysis-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  flex: 0 0 auto;
  border: 1px solid rgba(76, 139, 245, 0.25);
  border-radius: 12px;
  color: var(--aiops-brand);
  background: rgba(76, 139, 245, 0.06);
}

.ai-analysis-title {
  color: var(--aiops-text-primary);
  font-size: 16px;
  font-weight: 700;
}

.ai-analysis-subtitle {
  margin-top: 4px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
}

.ai-analysis-meta {
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
}

.ai-analysis-input-panel {
  padding: 12px;
}

.ai-analysis-prompt :deep(.el-textarea__inner) {
  border: 0;
  border-radius: 10px;
  box-shadow: none;
  resize: none;
}

.ai-analysis-prompt :deep(.el-input__wrapper) {
  min-height: 32px;
  border-radius: 8px;
  box-shadow: 0 0 0 1px rgba(223, 231, 242, 0.86) inset;
}

.ai-analysis-actions,
.ai-analysis-footer {
  justify-content: flex-end;
  gap: 10px;
  margin-top: 10px;
}

.ai-analysis-progress-panel,
.ai-analysis-section {
  padding: 14px;
}

.ai-analysis-progress-head,
.ai-analysis-section-head {
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.ai-analysis-eyebrow {
  color: var(--aiops-text-muted);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.4px;
}

.ai-analysis-section-title {
  color: var(--aiops-text-primary);
  font-size: 14px;
  font-weight: 700;
}

.ai-analysis-progress {
  margin-bottom: 12px;
}

.ai-analysis-summary-strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 12px;
}

.ai-analysis-summary-item {
  min-width: 0;
  padding: 12px 14px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 12px;
  background: rgba(248, 250, 252, 0.86);
}

.ai-analysis-summary-item.is-primary {
  border-color: rgba(76, 139, 245, 0.25);
  background: rgba(76, 139, 245, 0.06);
}

.ai-analysis-summary-item.is-warning {
  border-color: rgba(243, 178, 75, 0.34);
  background: rgba(243, 178, 75, 0.08);
}

.ai-analysis-summary-item span {
  display: block;
  margin-bottom: 6px;
  color: var(--aiops-text-muted);
  font-size: 12px;
  font-weight: 700;
}

.ai-analysis-summary-item strong {
  display: block;
  color: var(--aiops-text-primary);
  font-size: 13px;
  line-height: 1.55;
}

.ai-analysis-step-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.ai-analysis-step {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 132px;
  min-height: 30px;
  padding: 0 10px;
  border: 1px solid rgba(223, 231, 242, 0.95);
  border-radius: 999px;
  color: var(--aiops-text-muted);
  background: rgba(248, 250, 252, 0.86);
  font-size: 12px;
}

.ai-analysis-step span {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  flex: 0 0 auto;
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.16);
  font-weight: 800;
}

.ai-analysis-step strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-analysis-step.is-done,
.ai-analysis-step.is-active {
  border-color: rgba(63, 185, 127, 0.34);
  color: var(--aiops-success);
  background: rgba(63, 185, 127, 0.08);
}

.ai-analysis-step.is-active {
  border-color: rgba(76, 139, 245, 0.42);
  color: var(--aiops-brand);
  background: rgba(76, 139, 245, 0.08);
}

.ai-analysis-current-message {
  margin-top: 12px;
  padding: 10px 12px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 10px;
  color: var(--aiops-text-secondary);
  background: rgba(248, 250, 252, 0.9);
  font-size: 13px;
}

.ai-analysis-event-list {
  margin-top: 10px;
  border-top: 1px dashed rgba(223, 231, 242, 0.95);
}

.ai-analysis-event-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 170px;
  gap: 12px;
  padding: 10px 2px;
  border-bottom: 1px dashed rgba(223, 231, 242, 0.7);
}

.ai-analysis-event-item strong {
  color: var(--aiops-text-primary);
  font-size: 13px;
}

.ai-analysis-event-item p,
.ai-analysis-event-item time {
  margin: 4px 0 0;
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.ai-analysis-event-item time {
  text-align: right;
}

.ai-analysis-empty {
  padding: 14px 0;
}

.ai-analysis-text {
  max-height: min(420px, calc(92vh - 390px));
  padding: 14px 16px 16px;
  overflow: auto;
  color: var(--aiops-text-primary);
  font-size: 13px;
  line-height: 1.75;
  word-break: normal;
  overflow-wrap: break-word;
}

.ai-analysis-text :deep(h4) {
  margin: 14px 0 8px;
  font-size: 14px;
  font-weight: 700;
  color: var(--aiops-text-primary);
}

.ai-analysis-text :deep(h4:first-child) {
  margin-top: 0;
}

.ai-analysis-text :deep(.ai-analysis-line) {
  margin: 6px 0;
}

.ai-analysis-text :deep(.ai-analysis-line.is-ordered),
.ai-analysis-text :deep(.ai-analysis-line.is-bullet) {
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
  background: var(--aiops-brand);
}

.ai-analysis-text :deep(.ai-analysis-line.is-ordered .ai-analysis-marker) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 999px;
  color: var(--aiops-brand);
  background: rgba(76, 139, 245, 0.08);
  font-size: 11px;
  font-weight: 800;
}

.ai-analysis-text :deep(.ai-analysis-gap) {
  height: 6px;
}

.ai-analysis-text :deep(strong) {
  font-weight: 700;
  color: var(--aiops-text-primary);
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
  border-radius: 12px;
}

.ai-analysis-evidence-collapse {
  margin-top: 0;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 12px;
  overflow: hidden;
}

.ai-analysis-evidence-collapse :deep(.el-collapse-item__header) {
  padding: 0 14px;
  height: 42px;
  color: var(--aiops-text-primary);
  font-weight: 700;
  background: rgba(247, 250, 255, 0.86);
}

.ai-analysis-evidence-collapse :deep(.el-collapse-item__content) {
  padding: 0;
}

.ai-analysis-json {
  max-height: 260px;
  margin: 0;
  padding: 12px;
  overflow: auto;
  color: var(--aiops-text-secondary);
  background: #f8fafc;
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
}

.ai-task-question-panel,
.ai-task-stream-panel,
.ai-final-result-panel {
  padding: 14px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.9);
}

.ai-task-question-panel {
  padding: 10px 14px;
}

.ai-task-question-head,
.ai-task-prompt-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.ai-task-question-panel .ai-analysis-mark {
  width: 30px;
  height: 30px;
  border-radius: 9px;
}

.ai-task-question-panel .ai-analysis-title {
  font-size: 15px;
  line-height: 1.25;
}

.ai-task-question-panel .ai-analysis-subtitle {
  margin-top: 2px;
  font-size: 12px;
  line-height: 1.35;
}

.ai-task-object-grid {
  display: flex;
  align-items: center;
  flex-wrap: nowrap;
  gap: 12px;
  margin-top: 10px;
  padding: 8px 0;
  overflow-x: auto;
  border-top: 1px dashed rgba(223, 231, 242, 0.86);
  border-bottom: 1px dashed rgba(223, 231, 242, 0.86);
}

.ai-task-object-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  flex: 0 0 auto;
  padding: 0;
  border: 0;
  background: transparent;
}

.ai-task-object-item span {
  display: block;
  margin-bottom: 0;
  color: var(--aiops-text-muted);
  font-size: 12px;
  font-weight: 700;
}

.ai-task-object-item strong {
  display: block;
  min-width: 0;
  color: var(--aiops-text-primary);
  font-size: 13px;
  font-weight: 700;
  line-height: 1.45;
  max-width: 230px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-task-prompt-row {
  margin-top: 8px;
  padding-top: 0;
  border-top: 0;
}

.ai-task-prompt-row .ai-analysis-prompt {
  flex: 1;
  min-width: 0;
}

.ai-task-prompt-row .ai-analysis-section-title {
  white-space: nowrap;
}

.ai-task-stream-status {
  max-width: 58%;
  overflow: hidden;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  text-align: right;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-task-step-list {
  display: flex;
  align-items: flex-start;
  gap: 0;
  padding: 8px 2px 0;
  overflow-x: auto;
  overflow-y: hidden;
}

.ai-task-step {
  position: relative;
  flex: 1 1 0;
  min-width: 112px;
  padding: 0 10px;
  text-align: center;
}

.ai-task-step::before {
  content: "";
  position: absolute;
  top: 13px;
  left: 0;
  right: 0;
  height: 2px;
  background: rgba(223, 231, 242, 0.96);
}

.ai-task-step:first-child::before {
  left: 50%;
}

.ai-task-step:last-child::before {
  right: 50%;
}

.ai-task-step.is-success::before,
.ai-task-step.is-running::before {
  background: rgba(63, 185, 127, 0.42);
}

.ai-task-step.is-failed::before {
  background: rgba(245, 108, 108, 0.42);
}

.ai-task-step-index {
  position: relative;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 999px;
  color: var(--aiops-text-muted);
  background: #f4f7fb;
  box-shadow: 0 0 0 4px #ffffff;
  font-size: 12px;
  font-weight: 800;
}

.ai-task-step-main {
  min-width: 0;
  margin-top: 9px;
}

.ai-task-step-main strong {
  display: block;
  color: var(--aiops-text-primary);
  font-size: 13px;
  font-weight: 700;
  line-height: 1.3;
}

.ai-task-step-status {
  display: inline-flex;
  margin-top: 8px;
  padding: 2px 7px;
  border-radius: 999px;
  color: var(--aiops-text-muted);
  background: rgba(148, 163, 184, 0.14);
  font-size: 11px;
  font-weight: 700;
}

.ai-task-step.is-running {
  color: var(--aiops-brand);
}

.ai-task-step.is-running .ai-task-step-index,
.ai-task-step.is-running .ai-task-step-status {
  color: var(--aiops-brand);
  background: rgba(76, 139, 245, 0.12);
}

.ai-task-step.is-success {
  color: var(--aiops-success);
}

.ai-task-step.is-success .ai-task-step-index,
.ai-task-step.is-success .ai-task-step-status {
  color: var(--aiops-success);
  background: rgba(63, 185, 127, 0.12);
}

.ai-task-step.is-failed {
  color: #f56c6c;
}

.ai-task-step.is-failed .ai-task-step-index,
.ai-task-step.is-failed .ai-task-step-status {
  color: #f56c6c;
  background: rgba(245, 108, 108, 0.12);
}

.ai-task-step.is-skipped {
  opacity: 0.72;
}

.ai-task-step.is-running .ai-task-step-main strong {
  color: var(--aiops-brand);
}

.ai-task-step.is-success .ai-task-step-main strong {
  color: var(--aiops-success);
}

.ai-task-step.is-failed .ai-task-step-main strong {
  color: #f56c6c;
}

.ai-final-diagnosis-card {
  padding: 14px 16px 16px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 10px;
  background: linear-gradient(180deg, rgba(248, 250, 252, 0.92), rgba(255, 255, 255, 0.96));
}

.ai-final-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 10px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.74);
}

.ai-final-card-head strong {
  display: block;
  color: var(--aiops-text-primary);
  font-size: 14px;
  font-weight: 700;
}

.ai-final-card-head span {
  display: block;
  margin-top: 3px;
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.ai-final-conclusion-block {
  margin-top: 12px;
  padding: 12px 14px;
  border-left: 3px solid #e6a23c;
  border-radius: 8px;
  background: rgba(230, 162, 60, 0.08);
}

.ai-final-conclusion-block.is-confirmed {
  border-left-color: #f56c6c;
  background: rgba(245, 108, 108, 0.07);
}

.ai-result-section-title,
.ai-final-subtitle {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--aiops-text-primary);
  font-size: 14px;
  font-weight: 700;
  line-height: 1.4;
}

.ai-result-section-title > span {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  flex: 0 0 20px;
  border-radius: 4px;
  color: #3569ad;
  background: rgba(76, 139, 245, 0.1);
  font-family: Consolas, "Courier New", monospace;
  font-size: 11px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.ai-result-section-title > strong {
  font-weight: 700;
}

.ai-final-conclusion-text {
  margin: 6px 0 0;
  color: var(--aiops-text-primary);
  font-size: 14px;
  font-weight: 500;
  line-height: 1.8;
}

.ai-final-conclusion-explanation {
  margin: 6px 0 0;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.7;
}

.ai-route-diagnosis,
.ai-final-detail-section {
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px dashed rgba(223, 231, 242, 0.92);
}

.ai-route-chain {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 22px minmax(0, 1fr) 22px minmax(0, 1fr) 22px minmax(0, 1fr);
  gap: 6px;
  align-items: center;
  margin-top: 10px;
}

.ai-route-node {
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 8px;
  background: #fff;
}

.ai-route-node span,
.ai-route-node small {
  display: block;
  color: var(--aiops-text-muted);
  font-size: 11px;
}

.ai-route-node strong {
  display: block;
  margin: 4px 0;
  overflow: hidden;
  color: var(--aiops-text-primary);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-route-node.is-error {
  border-color: rgba(245, 108, 108, 0.42);
  background: rgba(245, 108, 108, 0.06);
}

.ai-route-node.is-error strong,
.ai-route-arrow.is-error {
  color: #d95050;
}

.ai-route-node.is-correct {
  border-color: rgba(63, 185, 127, 0.42);
  background: rgba(63, 185, 127, 0.07);
}

.ai-route-node.is-correct strong,
.ai-route-arrow.is-correct {
  color: #248a5b;
}

.ai-route-arrow {
  color: var(--aiops-text-muted);
  font-size: 18px;
  text-align: center;
}

.ai-final-fact-list {
  display: grid;
  gap: 0;
  margin: 12px 0 0;
  padding: 0;
  border: 1px solid rgba(223, 231, 242, 0.82);
  border-radius: 8px;
  overflow: hidden;
  list-style: none;
  background: rgba(255, 255, 255, 0.72);
}

.ai-final-fact-list li {
  display: grid;
  grid-template-columns: 76px minmax(0, 1fr);
  gap: 12px;
  align-items: start;
  padding: 10px 12px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.65;
}

.ai-final-fact-list li + li {
  border-top: 1px dashed rgba(223, 231, 242, 0.86);
}

.ai-final-fact-list span {
  color: var(--aiops-text-muted);
  font-weight: 700;
}

.ai-final-fact-list strong {
  color: var(--aiops-text-primary);
  font-weight: 600;
}

.ai-final-remediation {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px dashed rgba(223, 231, 242, 0.92);
}

.ai-evidence-lead,
.ai-config-note,
.ai-remediation-lead,
.ai-outcome-lead,
.ai-outcome-text {
  margin: 8px 0 0;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.7;
}

.ai-remediation-lead,
.ai-outcome-lead {
  color: var(--aiops-text-primary);
  font-weight: 600;
}

.ai-log-finding-list {
  display: grid;
  gap: 8px;
  margin-top: 10px;
}

.ai-log-finding-item {
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid rgba(245, 108, 108, 0.24);
  border-radius: 8px;
  background: rgba(245, 108, 108, 0.04);
}

.ai-log-finding-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 6px;
  color: var(--aiops-text-muted);
  font-size: 11px;
}

.ai-log-finding-meta strong {
  color: #d95050;
  font-size: 11px;
}

.ai-log-finding-item code {
  display: block;
  overflow-wrap: anywhere;
  color: #3f4652;
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
  line-height: 1.65;
  white-space: pre-wrap;
}

.ai-manifest-evidence-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.ai-manifest-evidence-head p {
  margin: 5px 0 0;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.6;
}

.ai-manifest-evidence-head > span {
  flex: 0 0 auto;
  padding: 3px 8px;
  border-radius: 6px;
  color: #3f6fb6;
  background: rgba(76, 139, 245, 0.09);
  font-size: 11px;
  font-weight: 700;
}

.ai-manifest-error-list {
  display: grid;
  gap: 6px;
  margin-top: 9px;
}

.ai-manifest-error-list > div {
  display: grid;
  grid-template-columns: 18px minmax(0, 1fr);
  gap: 6px;
  align-items: start;
  padding: 8px 10px;
  border-left: 3px solid #e6a23c;
  border-radius: 6px;
  color: #8a5a16;
  background: rgba(230, 162, 60, 0.07);
  font-size: 12px;
  line-height: 1.55;
}

.ai-manifest-error-list .el-icon {
  margin-top: 2px;
}

.ai-manifest-collapse {
  margin-top: 10px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 8px;
  overflow: hidden;
}

.ai-manifest-collapse :deep(.el-collapse-item__header) {
  min-height: 42px;
  height: auto;
  padding: 7px 12px;
  border-bottom-color: rgba(223, 231, 242, 0.8);
  background: #fff;
}

.ai-manifest-collapse :deep(.el-collapse-item__content) {
  padding-bottom: 0;
}

.ai-manifest-collapse :deep(.el-collapse-item:last-child .el-collapse-item__header) {
  border-bottom: 0;
}

.ai-manifest-title {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 9px;
}

.ai-manifest-title strong {
  min-width: 0;
  overflow: hidden;
  color: var(--aiops-text-primary);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-manifest-yaml {
  max-height: 360px;
  margin: 0;
  padding: 12px 14px;
  overflow: auto;
  color: #344054;
  background: #f7f9fc;
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre;
}

.ai-config-comparison {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-top: 10px;
}

.ai-config-block {
  min-width: 0;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 8px;
  background: #f8fafc;
}

.ai-config-block.is-current {
  border-color: rgba(230, 162, 60, 0.34);
}

.ai-config-block.is-fixed {
  border-color: rgba(63, 185, 127, 0.4);
}

.ai-config-block-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 9px 11px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.8);
  background: rgba(255, 255, 255, 0.88);
}

.ai-config-block-head strong {
  color: var(--aiops-text-primary);
  font-size: 12px;
}

.ai-config-block-head span {
  color: var(--aiops-text-muted);
  font-size: 11px;
}

.ai-config-block pre {
  margin: 0;
  padding: 11px 12px;
  overflow: auto;
  color: #344054;
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
  line-height: 1.55;
  white-space: pre;
}

.ai-config-alternative {
  margin: 0;
  padding: 8px 11px;
  border-top: 1px solid rgba(223, 231, 242, 0.8);
  color: var(--aiops-text-secondary);
  background: rgba(63, 185, 127, 0.05);
  font-size: 11px;
  line-height: 1.55;
}

.ai-ingress-fix-row {
  display: grid;
  grid-template-columns: minmax(180px, 0.75fr) minmax(280px, 1.25fr);
  gap: 10px 16px;
  align-items: start;
  margin-top: 10px;
  padding: 11px 12px;
  border: 1px solid rgba(76, 139, 245, 0.28);
  border-radius: 8px;
  background: rgba(76, 139, 245, 0.04);
}

.ai-ingress-fix-row strong {
  color: var(--aiops-text-primary);
  font-size: 12px;
}

.ai-ingress-fix-row p {
  margin: 5px 0 0;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.6;
}

.ai-ingress-fix-row pre {
  margin: 0;
  padding: 10px 12px;
  overflow: auto;
  border-radius: 6px;
  color: #344054;
  background: #f8fafc;
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
  line-height: 1.55;
  white-space: pre;
}

.ai-ingress-fix-row .ai-config-alternative {
  grid-column: 2;
  padding: 0;
  border: 0;
  background: transparent;
}

.ai-remediation-list {
  margin: 10px 0 0;
  padding-left: 22px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.85;
}

.ai-remediation-list li {
  padding-left: 3px;
}

.ai-remediation-list li + li {
  margin-top: 6px;
}

.ai-remediation-list li::marker {
  color: #3f6fb6;
  font-family: Consolas, "Courier New", monospace;
  font-weight: 700;
}

.ai-summary-conclusion-list {
  display: grid;
  gap: 7px;
  margin: 10px 0 0;
  padding-left: 20px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.75;
}

.ai-summary-conclusion-list li {
  padding-left: 3px;
}

.ai-summary-conclusion-list li::marker {
  color: #3f6fb6;
}

.ai-rich-text {
  overflow-wrap: anywhere;
}

.ai-rich-text :deep(.ai-rich-token.is-code) {
  padding: 1px 4px;
  border: 1px solid rgba(76, 139, 245, 0.16);
  border-radius: 3px;
  color: #315f9f;
  background: rgba(76, 139, 245, 0.07);
  box-decoration-break: clone;
  font-family: Consolas, "Courier New", monospace;
  font-size: 0.94em;
  font-weight: 600;
  overflow-wrap: anywhere;
  -webkit-box-decoration-break: clone;
}

.ai-rich-text :deep(.ai-rich-token.is-keyword) {
  color: #30445f;
  font-weight: 700;
}

.ai-rich-text :deep(.ai-rich-token.is-number) {
  color: #265ea8;
  font-family: Consolas, "Courier New", monospace;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.ai-rich-text :deep(.ai-rich-token.is-warning) {
  color: #b45f16;
  font-weight: 700;
}

.ai-rich-text :deep(.ai-rich-token.is-success) {
  color: #237a52;
  font-weight: 700;
}

.ai-service-exposure-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin: 8px 0 0;
  padding: 0;
  list-style: none;
}

.ai-service-exposure-list li {
  padding: 9px 11px;
  border-left: 3px solid #3fb97f;
  border-radius: 6px;
  color: var(--aiops-text-primary);
  background: rgba(63, 185, 127, 0.07);
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
  font-weight: 600;
}

.ai-outcome-text code {
  padding: 2px 5px;
  border-radius: 4px;
  color: #3f6fb6;
  background: rgba(76, 139, 245, 0.1);
  font-family: Consolas, "Courier New", monospace;
}

.ai-outcome-text.is-warning {
  color: #b66a16;
  font-weight: 600;
}

.ai-verification-section {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px dashed rgba(223, 231, 242, 0.92);
}

.ai-verification-section > strong {
  color: var(--aiops-text-primary);
  font-size: 12px;
}

.ai-verification-section ol {
  margin: 8px 0 0;
  padding-left: 22px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.8;
}

.ai-final-remediation ol {
  margin: 8px 0 0;
  padding-left: 22px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.8;
}

.ai-final-remediation li + li {
  margin-top: 3px;
}

.ai-key-evidence-list {
  margin: 0;
  padding: 10px 4px 12px 22px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.7;
}

.ai-final-waiting {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 82px;
  color: var(--aiops-text-secondary);
}

.ai-final-waiting-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  flex: 0 0 auto;
  border-radius: 10px;
  color: var(--aiops-brand);
  background: rgba(76, 139, 245, 0.08);
}

.ai-final-waiting strong {
  display: block;
  color: var(--aiops-text-primary);
  font-size: 14px;
}

.ai-final-waiting p {
  margin: 4px 0 0;
  font-size: 13px;
}

@media (max-width: 1200px) {
  .ai-task-step-list {
    padding-bottom: 4px;
  }

  .ai-task-step {
    flex: 0 0 128px;
  }
}

@media (max-width: 768px) {
  .ai-task-question-head,
  .ai-task-prompt-row {
    flex-direction: column;
  }

  .ai-task-step-list {
    margin: 0 -4px;
    padding-right: 4px;
    padding-left: 4px;
  }

  .ai-task-step {
    flex-basis: 124px;
  }

  .ai-task-stream-status {
    max-width: 100%;
    text-align: left;
    white-space: normal;
  }

  .ai-route-chain {
    grid-template-columns: 1fr;
  }

  .ai-route-arrow {
    transform: rotate(90deg);
  }

  .ai-config-comparison {
    grid-template-columns: 1fr;
  }

  .ai-ingress-fix-row,
  .ai-service-exposure-list {
    grid-template-columns: 1fr;
  }

  .ai-ingress-fix-row .ai-config-alternative {
    grid-column: 1;
  }

  .ai-final-fact-list li {
    grid-template-columns: 1fr;
    gap: 4px;
  }

  .ai-manifest-evidence-head {
    flex-direction: column;
  }

  .ai-manifest-yaml {
    max-height: 300px;
    padding: 10px;
    font-size: 11px;
  }
}

.pagination-wrapper {
  flex-shrink: 0;
  margin-top: 10px;
  padding: 8px 4px 0;
  border-top: 1px solid rgba(223, 231, 242, 0.72);
}

.scale-dialog :deep(.el-dialog),
.all-images-dialog :deep(.el-dialog),
.deployment-history-dialog :deep(.el-dialog),
.scheduling-dialog :deep(.el-dialog) {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 24px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
  box-shadow: 0 36px 110px rgba(15, 23, 42, 0.11);
  backdrop-filter: blur(18px);
}

.scale-dialog :deep(.el-dialog__header),
.all-images-dialog :deep(.el-dialog__header),
.deployment-history-dialog :deep(.el-dialog__header),
.scheduling-dialog :deep(.el-dialog__header) {
  padding: 16px 20px 14px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.84);
}

.scale-dialog :deep(.el-dialog__title),
.all-images-dialog :deep(.el-dialog__title),
.deployment-history-dialog :deep(.el-dialog__title),
.scheduling-dialog :deep(.el-dialog__title) {
  color: var(--aiops-text-primary);
  font-size: 17px;
  font-weight: 700;
}

.scale-dialog :deep(.el-dialog__body),
.all-images-dialog :deep(.el-dialog__body),
.deployment-history-dialog :deep(.el-dialog__body),
.scheduling-dialog :deep(.el-dialog__body) {
  padding: 18px 20px;
  background:
    radial-gradient(circle at top left, rgba(76, 139, 245, 0.04), transparent 22%),
    linear-gradient(180deg, #f8fafc 0%, #f5f8fc 100%);
}

.scale-dialog :deep(.el-dialog__footer),
.deployment-history-dialog :deep(.el-dialog__footer),
.scheduling-dialog :deep(.el-dialog__footer) {
  padding: 10px 20px 16px;
  background: rgba(247, 250, 255, 0.72);
  border-top: 1px solid rgba(223, 231, 242, 0.76);
}

.scale-dialog :deep(.el-form),
.scheduling-dialog :deep(.el-form) {
  padding: 14px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.82);
}

.images-view-content {
  gap: 18px;
}

.images-section h4 {
  margin: 0 0 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.9);
  color: var(--aiops-text-primary);
  font-size: 16px;
  font-weight: 700;
}

.all-images-list {
  gap: 10px;
}

.image-card {
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.84);
  box-shadow: none;
  transition: border-color 0.2s ease, background-color 0.2s ease;
}

.image-card:hover {
  border-color: rgba(153, 181, 230, 0.95);
  box-shadow: none;
  background: #ffffff;
}

.image-icon {
  color: var(--aiops-brand);
}

.full-image-name {
  color: var(--aiops-text-secondary);
}

.image-details {
  border-top: 1px solid rgba(223, 231, 242, 0.9);
}

.no-images {
  padding: 28px 16px;
  color: var(--aiops-text-muted);
}

.history-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 14px;
}

.summary-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.82);
}

.summary-label {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.summary-value {
  overflow: hidden;
  color: var(--aiops-text-primary);
  font-size: 14px;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.history-table {
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 12px;
  overflow: hidden;
}

.revision-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.revision-images {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.revision-image-tag {
  max-width: 100%;
  cursor: pointer;
}

.revision-image-tag :deep(.el-tag__content) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.replica-extra {
  margin-left: 6px;
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.form-tip {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.el-tag {
  border-radius: 999px;
  font-weight: 500;
}

.el-button {
  border-radius: 13px;
  font-weight: 600;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease, opacity 0.2s ease;
}

.k8s-workloads-management .el-button:hover {
  transform: translateY(-1px);
}

.el-input :deep(.el-input__wrapper),
.el-select :deep(.el-input__wrapper),
.el-input-number :deep(.el-input__wrapper) {
  min-height: 32px;
  background: rgba(252, 253, 255, 0.98);
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 13px;
  box-shadow: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;
}

.el-input :deep(.el-input__wrapper):hover,
.el-select :deep(.el-input__wrapper):hover,
.el-input-number :deep(.el-input__wrapper):hover {
  border-color: #c0c4cc;
}

.el-input :deep(.el-input__wrapper.is-focus),
.el-select :deep(.el-input__wrapper.is-focus),
.el-input-number :deep(.el-input__wrapper.is-focus) {
  border-color: rgba(120, 156, 221, 0.9);
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
  background: #ffffff;
}

.el-loading-mask {
  background-color: rgba(247, 250, 255, 0.68);
  backdrop-filter: blur(4px);
}

@media (max-width: 1200px) {
  .header-actions {
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .operation-buttons {
    gap: 12px;
  }
}

@media (max-width: 768px) {
  .k8s-workloads-management {
    --workload-page-height-offset: 131px;
    padding: 4px;
  }

  .workloads-card {
    height: calc(100vh - 8px - var(--workload-page-height-offset));
    padding: 48px 8px 8px;
    border-radius: 22px;
  }

  .page-hero {
    flex-direction: column;
    align-items: stretch;
  }

  .header-actions {
    width: 100%;
    align-items: stretch;
    flex-direction: column;
  }

  .header-actions :deep(.cluster-selector),
  .header-actions :deep(.namespace-selector) {
    width: 100% !important;
  }

  .search-form {
    display: block;
  }

  .search-form :deep(.el-form-item) {
    margin-right: 0;
    margin-bottom: 12px;
  }

  .search-form :deep(.el-form-item__content),
  .search-form :deep(.el-input),
  .search-form :deep(.el-select),
  .search-name-input,
  .scope-select,
  .query-button,
  .reset-button,
  .secondary-action,
  .primary-action {
    width: 100% !important;
  }

  .operation-buttons {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }

  .history-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

/* DESIGN.md compact workbench overrides */
.k8s-workloads-management {
  --ops-text-primary: #172033;
  --ops-text-muted: #64748b;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: 8px;
  height: calc(100vh - 124px);
  min-height: 0;
  padding: 7px;
  overflow: auto;
  background: transparent;
  color: var(--ops-text-primary);
}

.page-card {
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 10px;
  overflow: hidden;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
  backdrop-filter: none;
}

.workloads-card {
  height: auto;
  padding: 10px;
}

.workloads-card::before {
  display: none;
}

.ops-page-header {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 12px;
  min-width: 0;
  padding: 8px 12px;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 10px 28px rgba(80, 124, 180, 0.1);
}

.ops-page-header__icon {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  place-items: center;
}

.ops-page-header__icon img {
  width: 30px;
  height: 30px;
  object-fit: contain;
}

.ops-page-header__content {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.ops-page-header h1 {
  margin: 0;
  color: #0f172a;
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  white-space: nowrap;
}

.ops-page-header__divider {
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  background: var(--ops-border-soft);
}

.ops-page-header__content p {
  margin: 0;
  overflow: hidden;
  color: var(--ops-text-muted);
  font-size: 12px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.header-actions {
  gap: 8px;
  margin-left: auto;
}

.header-actions :deep(.el-input__wrapper),
.header-actions :deep(.el-select__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.search-section {
  flex-shrink: 0;
  margin-bottom: 8px;
  padding: 8px 10px 0;
  border: 1px solid var(--ops-border-soft);
  border-radius: 10px;
  background: #f8fbff;
  box-shadow: none;
}

.search-form {
  row-gap: 8px;
}

.search-form :deep(.el-form-item) {
  margin-right: 12px;
}

.search-form :deep(.el-form-item:last-child) {
  margin-right: 0;
}

.search-form :deep(.el-form-item__label) {
  height: 34px;
  padding-right: 8px;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
  line-height: 34px;
}

.search-name-input :deep(.el-input__wrapper),
.scope-select :deep(.el-input__wrapper),
.scope-select :deep(.el-select__wrapper) {
  min-height: 34px;
  padding: 0 11px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.search-name-input :deep(.el-input__wrapper:hover),
.scope-select :deep(.el-input__wrapper:hover),
.scope-select :deep(.el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.search-name-input :deep(.el-input__wrapper.is-focus),
.scope-select :deep(.el-input__wrapper.is-focus),
.scope-select :deep(.el-select__wrapper.is-focused) {
  box-shadow: 0 0 0 1px #409eff inset;
}

.query-button,
.refresh-button,
.secondary-action,
.primary-action {
  height: 34px;
  min-width: 76px;
  border-radius: 8px;
  color: #fff;
  font-weight: 600;
  box-shadow: none;
  transition: background-color 160ms ease, border-color 160ms ease, box-shadow 160ms ease;
}

.refresh-button {
  border-color: #f3c66b;
  background: #f3c66b;
}

.secondary-action {
  min-width: 108px;
}

.primary-action {
  min-width: 116px;
}

.k8s-workloads-management .el-button:hover,
.query-button:hover,
.query-button:focus,
.refresh-button:hover,
.refresh-button:focus,
.secondary-action:hover,
.secondary-action:focus,
.primary-action:hover,
.primary-action:focus {
  transform: none;
}

.query-button:hover,
.query-button:focus,
.refresh-button:hover,
.refresh-button:focus,
.secondary-action:hover,
.secondary-action:focus {
  color: #fff;
}

.workload-type-section {
  margin-top: 6px;
  padding-top: 6px;
}

.workload-type-section :deep(.el-tabs__item) {
  height: 32px;
  padding: 0 12px;
  font-size: 12px;
}

.table-section {
  flex: 1;
  min-height: 0;
  padding: 0;
  border: 1px solid #e3ebf5;
  border-radius: 10px;
  background: #fff;
  overflow: auto;
}

.table-toolbar {
  min-height: 38px;
  margin-bottom: 0;
  padding: 5px 10px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: #fbfdff;
}

.workloads-table,
.pods-workloads-table {
  border: 0;
  border-radius: 9px;
  background: #fff;
}

.workloads-table :deep(.el-table__header th),
.workloads-table :deep(.el-table__fixed-right-patch) {
  height: 38px;
  padding: 6px 0;
  background: #f3f7fc !important;
  border-bottom: 1px solid #e3ebf5;
}

.workloads-table :deep(.el-table__header th .cell) {
  color: #475569 !important;
  font-size: 12px;
}

.workloads-table :deep(.el-table__cell) {
  padding: 7px 0;
  border-bottom-color: #eef3f9;
}

.workloads-table :deep(.el-table__body tr:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

.workload-name-link {
  color: #2563eb;
  transition: color 160ms ease;
}

.workload-name-link:hover,
.image-tag-wrapper:hover,
.more-images-btn:hover {
  transform: none;
}

.pod-count-tag {
  padding: 1px 7px;
  font-size: 11px;
}

.resource-type,
.request-value,
.limit-value,
.datetime-text,
.node-name-cell {
  font-size: 11px;
}

.operation-buttons {
  gap: 12px;
  justify-content: flex-end;
}

.operation-buttons :deep(.el-button),
.operation-buttons .operation-text {
  transform: none;
  transition: color 160ms ease, opacity 160ms ease;
}

.pagination-wrapper {
  min-height: 34px;
  margin-top: 8px;
  padding: 8px 2px 0;
  border-top: 1px solid var(--ops-border-soft);
}

.pagination-wrapper :deep(.el-pagination) {
  --el-pagination-button-height: 28px;
  --el-pagination-button-width: 28px;
  font-size: 12px;
}

.images-view-content {
  gap: 12px;
}

.images-section h4 {
  margin-bottom: 8px;
  padding-bottom: 6px;
  font-size: 14px;
}

.all-images-list {
  gap: 8px;
}

.image-card {
  border-color: #e3ebf5;
  border-radius: 10px;
  background: #fbfdff;
}

.history-summary {
  gap: 8px;
  margin-bottom: 10px;
}

.summary-item {
  padding: 8px 10px;
  border-color: #dce8f5;
  border-radius: 10px;
  background: #fbfdff;
}

.history-table {
  border-color: #e3ebf5;
  border-radius: 9px;
}

:global(.k8s-workload-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 24px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: 15px !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.k8s-workload-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: linear-gradient(90deg, #fff6e7 0%, #f7fbff 36%, #ecf7ff 100%);
}

:global(.k8s-workload-dialog-overlay .el-dialog__title) {
  color: #172033;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.k8s-workload-dialog-overlay .el-dialog__headerbtn) {
  top: 12px;
  right: 14px;
  width: 32px;
  height: 32px;
}

:global(.k8s-workload-dialog-overlay .el-dialog__body) {
  max-height: calc(100vh - 190px);
  overflow: auto;
  padding: 14px 18px;
  background: #fff;
}

:global(.k8s-workload-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.k8s-workload-dialog-overlay .workload-dialog-form) {
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
}

:global(.k8s-workload-dialog-overlay .el-form-item) {
  margin-bottom: 14px;
}

:global(.k8s-workload-dialog-overlay .el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

:global(.k8s-workload-dialog-overlay .el-input__wrapper),
:global(.k8s-workload-dialog-overlay .el-select__wrapper),
:global(.k8s-workload-dialog-overlay .el-input-number) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.k8s-workload-dialog-overlay .el-input__wrapper:hover),
:global(.k8s-workload-dialog-overlay .el-select__wrapper:hover),
:global(.k8s-workload-dialog-overlay .el-input-number:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

:global(.k8s-workload-dialog-overlay .el-button) {
  min-height: 34px;
  border-radius: 8px;
  font-weight: 600;
  transform: none !important;
}

:global(.k8s-workload-dialog-overlay .ops-refresh-button) {
  color: #fff;
  border-color: #f3c66b;
  background: #f3c66b;
}

:global(.k8s-workload-dialog-overlay .el-table) {
  overflow: hidden;
  border: 1px solid #e3ebf5;
  border-radius: 9px;
}

:global(.k8s-workload-dialog-overlay .el-table th.el-table__cell) {
  background: #f3f7fc !important;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

:global(.workload-ai-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 20px;
  overflow: hidden;
  border: 1px solid #cfe2f8;
  border-radius: 20px !important;
  background: #fff;
  box-shadow: 0 28px 80px rgba(39, 70, 110, 0.24);
}

:global(.workload-ai-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-top: 3px solid transparent;
  border-bottom: 1px solid #e3edf8;
  background:
    linear-gradient(#fff, #fff) padding-box,
    linear-gradient(90deg, #f5a524 0%, #62a8ff 48%, #55d6be 100%) border-box;
}

:global(.workload-ai-dialog-overlay .el-dialog__title) {
  color: #172033;
  font-size: 18px;
  font-weight: 700;
}

:global(.workload-ai-dialog-overlay .el-dialog__body) {
  max-height: calc(92vh - 132px);
  overflow-y: auto;
  padding: 16px;
  background: #f8fbff;
}

:global(.workload-ai-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #fff;
}

:global(.workload-ai-dialog-overlay .el-button) {
  min-height: 34px;
  border-radius: 8px;
  font-weight: 600;
  transform: none !important;
}

@media (max-width: 960px) {
  .ops-page-header {
    flex-wrap: wrap;
  }

  .header-actions {
    width: 100%;
    margin-left: 0;
  }
}

@media (max-width: 768px) {
  .k8s-workloads-management {
    height: calc(100vh - 110px);
    padding: 6px;
  }

  .ops-page-header {
    align-items: flex-start;
    padding: 8px 10px;
  }

  .ops-page-header__content {
    flex-direction: column;
    align-items: stretch;
    gap: 2px;
  }

  .ops-page-header__divider,
  .ops-page-header__content p {
    display: none;
  }

  .page-card,
  .workloads-card {
    padding: 8px;
    border-radius: 12px;
  }

  .search-form :deep(.el-form-item) {
    margin-bottom: 8px;
  }
}

</style>
