<script setup>
import { ref, reactive, onMounted, computed, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Search,
  Refresh,
  Plus,
  Delete,
  ArrowDown,
  Connection,
  Monitor,
  CopyDocument,
  DataAnalysis
} from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'
import PodYamlDialog from './pods/PodYamlDialog.vue'
import ClusterSelector from './pods/ClusterSelector.vue'
import NamespaceSelector from './pods/NamespaceSelector.vue'
import { getYamlContentFromApiResponse } from './utils/yaml'

// 集群和命名空间选择
const selectedClusterId = ref('')
const queryParams = reactive({
  namespace: 'default'
})

// 页面状态
const loading = ref(false)
const activeTab = ref('service')
const searchKeyword = ref('')

// Service 相关状态
const serviceList = ref([])
const serviceLoading = ref(false)
const serviceCurrentPage = ref(1)
const servicePageSize = ref(10)
const serviceTotal = ref(0)
const serviceTotalPages = ref(0)

// 监听 serviceList 的变化，确保它始终是数组
watch(serviceList, (newValue) => {
  if (!Array.isArray(newValue)) {
    console.warn('serviceList 被设置为非数组值:', newValue, '重置为空数组')
    serviceList.value = []
  }
}, { immediate: true })
const serviceDialogVisible = ref(false)
const editServiceDialogVisible = ref(false)
const serviceDetailDialogVisible = ref(false)
const serviceYamlDialogVisible = ref(false)
const serviceEventsDialogVisible = ref(false)
const currentService = reactive({
  name: '',
  labels: {},
  type: 'ClusterIP',
  selector: {},
  ports: [{ name: '', port: 80, targetPort: 80, protocol: 'TCP' }],
  description: ''
})
const currentServiceForDetail = ref({})
const currentServiceYaml = ref('')
const currentServiceForEvents = ref({})
const serviceEvents = ref([])

// Ingress 相关状态
const ingressList = ref([])
const ingressLoading = ref(false)
const ingressCurrentPage = ref(1)
const ingressPageSize = ref(10)
const ingressTotal = ref(0)
const ingressTotalPages = ref(0)

// 监听 ingressList 的变化，确保它始终是数组
watch(ingressList, (newValue) => {
  if (!Array.isArray(newValue)) {
    console.warn('ingressList 被设置为非数组值:', newValue, '重置为空数组')
    ingressList.value = []
  }
}, { immediate: true })
const ingressDialogVisible = ref(false)
const editIngressDialogVisible = ref(false)
const ingressDetailDialogVisible = ref(false)
const ingressYamlDialogVisible = ref(false)
const ingressEventsDialogVisible = ref(false)
const currentIngress = reactive({
  name: '',
  annotations: {},
  rules: [{ host: '', paths: [{ path: '/', pathType: 'Prefix', serviceName: '', servicePort: 80 }] }],
  tls: [],
  description: ''
})
const currentIngressForDetail = ref({})
const currentIngressYaml = ref('')
const currentIngressForEvents = ref({})
const ingressEvents = ref([])

// Ingress 测试相关状态
const ingressTestDialogVisible = ref(false)
const ingressTestLoading = ref(false)
const ingressTestResult = ref(null)
const ingressTestForm = reactive({
  serviceName: '',
  servicePort: 80,
  path: '/',
  host: '',
  method: 'GET',
  timeout: 10
})

// Ingress AI 诊断相关状态
const ingressAiDialogVisible = ref(false)
const ingressAiLoading = ref(false)
const currentIngressForAi = ref({})
const ingressAiCandidateIngresses = ref([])
const ingressAiResult = ref(null)
const ingressAiStreamSource = ref(null)
const ingressAiProgress = ref(0)
const ingressAiMessage = ref('')
const ingressAiForm = reactive({
  host: '',
  path: '/',
  statusCode: '502',
  observedError: ''
})

// Service 类型选项
const serviceTypeOptions = [
  { label: 'ClusterIP', value: 'ClusterIP', description: '集群内部访问' },
  { label: 'NodePort', value: 'NodePort', description: '节点端口访问' },
  { label: 'LoadBalancer', value: 'LoadBalancer', description: '负载均衡器访问' },
  { label: 'ExternalName', value: 'ExternalName', description: '外部名称映射' }
]

// 协议选项
const protocolOptions = [
  { label: 'TCP', value: 'TCP' },
  { label: 'UDP', value: 'UDP' },
  { label: 'SCTP', value: 'SCTP' }
]

// 路径类型选项
const pathTypeOptions = [
  { label: 'Prefix', value: 'Prefix', description: '前缀匹配' },
  { label: 'Exact', value: 'Exact', description: '精确匹配' },
  { label: 'ImplementationSpecific', value: 'ImplementationSpecific', description: '实现特定' }
]

const ingressStatusCodeOptions = ['403', '404', '500', '502', '503', '504']

const ingressAiStepDefinitions = [
  { step: 'start', label: '开始分析 Ingress' },
  { step: 'connect_cluster', label: '连接 Kubernetes 集群' },
  { step: 'ingress_rules', label: '读取并匹配规则' },
  { step: 'ingress_events', label: '采集 Ingress 事件' },
  { step: 'service', label: '检查后端 Service' },
  { step: 'endpoints', label: '检查 Endpoints' },
  { step: 'backend_pods', label: '查找后端 Pod' },
  { step: 'backend_logs', label: '采集后端日志' },
  { step: 'evidence', label: '整理诊断证据' },
  { step: 'ai_analysis', label: '调用 AI 分析' },
  { step: 'result', label: '生成诊断结论' }
]

// 过滤后的列表
const filteredServiceList = computed(() => {
  // 确保 serviceList.value 是数组
  return Array.isArray(serviceList.value) ? serviceList.value : []
})

const filteredIngressList = computed(() => {
  // 确保 ingressList.value 是数组
  const list = Array.isArray(ingressList.value) ? ingressList.value : []

  if (!searchKeyword.value) return list
  return list.filter(item =>
    item.name?.toLowerCase().includes(searchKeyword.value.toLowerCase()) ||
    (item.rules && Array.isArray(item.rules) && item.rules.some(rule =>
      rule.host?.toLowerCase().includes(searchKeyword.value.toLowerCase())
    ))
  )
})

const getCurrentResourceTotal = () => (
  activeTab.value === 'service' ? serviceTotal.value : ingressTotal.value
)

const currentResourceCountText = computed(() => {
  const resourceName = activeTab.value === 'service' ? 'Service' : 'Ingress'
  return `共 ${getCurrentResourceTotal()} 个 ${resourceName}`
})

const getCurrentResourcePageSize = () => (
  activeTab.value === 'service' ? servicePageSize.value : ingressPageSize.value
)

const getCurrentResourceCurrentPage = () => (
  activeTab.value === 'service' ? serviceCurrentPage.value : ingressCurrentPage.value
)

const updatePaginationFromResponse = (resource, data = {}, list = []) => {
  const currentPageRef = resource === 'service' ? serviceCurrentPage : ingressCurrentPage
  const pageSizeRef = resource === 'service' ? servicePageSize : ingressPageSize
  const totalRef = resource === 'service' ? serviceTotal : ingressTotal
  const totalPagesRef = resource === 'service' ? serviceTotalPages : ingressTotalPages

  const responseTotal = Number(data?.total ?? list.length)
  const responsePage = Number(data?.page ?? currentPageRef.value)
  const responsePageSize = Number(data?.pageSize ?? pageSizeRef.value)
  const responseTotalPages = Number(data?.totalPages ?? (responsePageSize > 0 ? Math.ceil(responseTotal / responsePageSize) : 0))

  totalRef.value = Number.isFinite(responseTotal) ? responseTotal : list.length
  currentPageRef.value = Number.isFinite(responsePage) && responsePage > 0 ? responsePage : currentPageRef.value
  pageSizeRef.value = Number.isFinite(responsePageSize) && responsePageSize > 0 ? responsePageSize : pageSizeRef.value
  totalPagesRef.value = Number.isFinite(responseTotalPages) ? responseTotalPages : 0
}

const resetServicePagination = () => {
  serviceCurrentPage.value = 1
  serviceTotal.value = 0
  serviceTotalPages.value = 0
}

const resetIngressPagination = () => {
  ingressCurrentPage.value = 1
  ingressTotal.value = 0
  ingressTotalPages.value = 0
}

const resetActivePagination = () => {
  if (activeTab.value === 'service') {
    serviceCurrentPage.value = 1
  } else {
    ingressCurrentPage.value = 1
  }
}

// 处理集群选择变化
const handleClusterChange = (clusterId) => {
  selectedClusterId.value = clusterId
  console.log('集群选择变化:', clusterId)
  if (clusterId && queryParams.namespace) {
    loadAllNetworkResources()
  }
}

// 处理命名空间选择变化
const handleNamespaceChange = (namespace) => {
  queryParams.namespace = namespace
  console.log('命名空间选择变化:', namespace)
  if (selectedClusterId.value && namespace) {
    loadAllNetworkResources()
  }
}

// 加载所有网络资源
const loadAllNetworkResources = () => {
  console.log('加载网络资源 - 集群:', selectedClusterId.value, '命名空间:', queryParams.namespace)

  // 清空现有数据
  serviceList.value = []
  ingressList.value = []
  resetServicePagination()
  resetIngressPagination()

  // 重新加载数据
  if (activeTab.value === 'service') {
    fetchServiceList()
  } else {
    fetchIngressList()
  }
}

// 监听集群和命名空间变化，自动加载数据
watch(
  [selectedClusterId, () => queryParams.namespace],
  ([clusterId, namespace]) => {
    console.log('监听到变化 - 集群ID:', clusterId, '命名空间:', namespace)
    if (clusterId && namespace) {
      console.log('集群和命名空间都已选择，开始加载网络资源')
      loadAllNetworkResources()
    }
  },
  { immediate: true }
)

// 获取 Service 列表
const fetchServiceList = async () => {
  if (!selectedClusterId.value) {
    console.warn('集群ID为空，无法获取 Service 列表')
    return
  }

  try {
    serviceLoading.value = true
    const params = {
      page: serviceCurrentPage.value,
      pageSize: servicePageSize.value
    }
    const keyword = searchKeyword.value.trim()
    if (keyword) {
      params.keyword = keyword
    }
    console.log('正在获取 Service 列表，集群ID:', selectedClusterId.value, '命名空间:', queryParams.namespace, '分页:', params)

    const response = await k8sApi.getServiceList(selectedClusterId.value, queryParams.namespace, params)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      // 后端返回的数据结构是 { data: { services: [...] } }
      // 处理 services 为 null 的情况
      const rawData = responseData.data?.services || responseData.data
      // 确保获取到的数据是数组格式，处理 null 和 undefined
      serviceList.value = Array.isArray(rawData) ? rawData : []
      updatePaginationFromResponse('service', responseData.data, serviceList.value)
      console.log('获取到 Service 列表:', serviceList.value.length, '个，总数:', serviceTotal.value)
    } else {
      ElMessage.error(responseData.message || '获取 Service 列表失败')
      serviceList.value = [] // 失败时确保是空数组
      serviceTotal.value = 0
      serviceTotalPages.value = 0
    }
  } catch (error) {
    console.error('获取 Service 列表失败:', error)
    ElMessage.error('获取 Service 列表失败，请检查网络连接')
    serviceList.value = [] // 异常时确保是空数组
    serviceTotal.value = 0
    serviceTotalPages.value = 0
  } finally {
    serviceLoading.value = false
  }
}

// 获取 Ingress 列表
const fetchIngressList = async () => {
  if (!selectedClusterId.value) {
    console.warn('集群ID为空，无法获取 Ingress 列表')
    return
  }

  try {
    ingressLoading.value = true
    const params = {
      page: ingressCurrentPage.value,
      pageSize: ingressPageSize.value
    }
    const keyword = searchKeyword.value.trim()
    if (keyword) {
      params.keyword = keyword
    }
    console.log('正在获取 Ingress 列表，集群ID:', selectedClusterId.value, '命名空间:', queryParams.namespace, '分页:', params)

    const response = await k8sApi.getIngressList(selectedClusterId.value, queryParams.namespace, params)
    console.log('Ingress API原始响应:', response)

    const responseData = response.data || response
    console.log('Ingress API响应数据:', responseData)

    if (responseData.code === 200 || responseData.success) {
      // 后端返回的数据结构可能是 { data: { ingresses: [...] } } 或类似结构
      // 处理 ingresses 为 null 的情况
      const rawData = responseData.data?.ingresses || responseData.data
      // 确保获取到的数据是数组格式，处理 null 和 undefined
      ingressList.value = Array.isArray(rawData) ? rawData : []
      updatePaginationFromResponse('ingress', responseData.data, ingressList.value)
      console.log('获取到 Ingress 列表:', ingressList.value.length, '个，总数:', ingressTotal.value)
    } else {
      console.error('Ingress API返回错误:', responseData)
      ElMessage.error(responseData.message || '获取 Ingress 列表失败')
      ingressList.value = [] // 失败时确保是空数组
      ingressTotal.value = 0
      ingressTotalPages.value = 0
    }
  } catch (error) {
    console.error('获取 Ingress 列表失败:', error)
    console.error('错误详情:', {
      message: error?.message,
      response: error?.response,
      request: error?.request
    })
    ElMessage.error(error?.response?.data?.message || '获取 Ingress 列表失败，请检查网络连接')
    ingressList.value = [] // 异常时确保是空数组
    ingressTotal.value = 0
    ingressTotalPages.value = 0
  } finally {
    ingressLoading.value = false
  }
}

// 刷新数据
const handleRefresh = () => {
  resetActivePagination()
  if (activeTab.value === 'service') {
    fetchServiceList()
  } else {
    fetchIngressList()
  }
}

// 标签页切换
const handleTabChange = (tab) => {
  activeTab.value = tab
  searchKeyword.value = ''
  resetActivePagination()
  if (tab === 'service') {
    fetchServiceList()
  } else {
    fetchIngressList()
  }
}

const handleNetworkSizeChange = (size) => {
  if (activeTab.value === 'service') {
    servicePageSize.value = size
    serviceCurrentPage.value = 1
    fetchServiceList()
    return
  }

  ingressPageSize.value = size
  ingressCurrentPage.value = 1
  fetchIngressList()
}

const handleNetworkCurrentChange = (page) => {
  if (activeTab.value === 'service') {
    serviceCurrentPage.value = page
    fetchServiceList()
    return
  }

  ingressCurrentPage.value = page
  fetchIngressList()
}

const refreshServiceListFromFirstPage = () => {
  serviceCurrentPage.value = 1
  fetchServiceList()
}

const refreshIngressListFromFirstPage = () => {
  ingressCurrentPage.value = 1
  fetchIngressList()
}

// Service 操作
const handleCreateService = () => {
  if (!selectedClusterId.value) {
    ElMessage.warning('请先选择集群')
    return
  }
  if (!queryParams.namespace) {
    ElMessage.warning('请先选择命名空间')
    return
  }

  // 设置默认的Service YAML模板
  const defaultYaml = `apiVersion: v1
kind: Service
metadata:
  name: new-service
  namespace: ${queryParams.namespace}
spec:
  type: ClusterIP
  ports:
  - port: 80
    targetPort: 8080
    protocol: TCP
    name: http
  selector:
    app: my-app`

  currentServiceYaml.value = defaultYaml
  currentServiceForDetail.value = { name: 'new-service', namespace: queryParams.namespace }
  serviceYamlDialogVisible.value = true
}

// 查看 Service 详情
const handleViewService = async (row) => {
  try {
    const response = await k8sApi.getServiceDetail(selectedClusterId.value, queryParams.namespace, row.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      currentServiceForDetail.value = responseData.data || row
      serviceDetailDialogVisible.value = true
    } else {
      ElMessage.error(responseData.message || '获取 Service 详情失败')
    }
  } catch (error) {
    console.error('获取 Service 详情失败:', error)
    ElMessage.error('获取 Service 详情失败')
  }
}

// 编辑 Service YAML
const handleEditServiceYaml = async (row) => {
  try {
    loading.value = true
    const response = await k8sApi.getServiceYaml(selectedClusterId.value, queryParams.namespace, row.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      currentServiceYaml.value = getYamlContentFromApiResponse(
        responseData,
        `# Service ${row.name} YAML\napiVersion: v1\nkind: Service\nmetadata:\n  name: ${row.name}`
      )
      currentServiceForDetail.value = row
      serviceYamlDialogVisible.value = true
    } else {
      ElMessage.error(responseData.message || '获取 Service YAML 失败')
    }
  } catch (error) {
    console.error('获取 Service YAML 失败:', error)
    ElMessage.error('获取 Service YAML 失败')
  } finally {
    loading.value = false
  }
}

// 查看 Service 事件
const handleViewServiceEvents = async (row) => {
  try {
    currentServiceForEvents.value = row
    const response = await k8sApi.getServiceEvents(selectedClusterId.value, queryParams.namespace, row.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      serviceEvents.value = responseData.data || []
    } else {
      serviceEvents.value = []
      ElMessage.warning('暂无事件数据')
    }
    serviceEventsDialogVisible.value = true
  } catch (error) {
    console.error('获取 Service 事件失败:', error)
    serviceEvents.value = []
    serviceEventsDialogVisible.value = true
  }
}

const handleEditService = async (row) => {
  try {
    const response = await k8sApi.getServiceDetail(selectedClusterId.value, queryParams.namespace, row.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      const serviceData = responseData.data
      Object.assign(currentService, {
        name: serviceData.name,
        labels: serviceData.labels || {},
        type: serviceData.type,
        selector: serviceData.selector || {},
        ports: serviceData.ports || [{ name: '', port: 80, targetPort: 80, protocol: 'TCP' }],
        description: serviceData.description || ''
      })
      editServiceDialogVisible.value = true
    }
  } catch (error) {
    ElMessage.error('获取 Service 详情失败')
  }
}

const handleDeleteService = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除 Service "${row.name}" 吗？此操作不可恢复。`,
      '删除确认',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )

    const response = await k8sApi.deleteService(selectedClusterId.value, queryParams.namespace, row.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success('Service 删除成功')
      fetchServiceList()
    } else {
      ElMessage.error(responseData.message || 'Service 删除失败')
    }
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('Service 删除失败')
    }
  }
}

const getIngressRulePaths = (rule = {}) => {
  if (Array.isArray(rule.http?.paths)) {
    return rule.http.paths
  }
  if (Array.isArray(rule.paths)) {
    return rule.paths
  }
  return []
}

const getIngressPathServiceName = (path = {}) => (
  path.backend?.service?.name || path.backend?.serviceName || path.serviceName || ''
)

const getIngressPathServicePort = (path = {}) => (
  path.backend?.service?.port?.number || path.backend?.servicePort || path.servicePort || 80
)

const getHostFromEndpoint = (endpoint = '') => {
  if (!endpoint) return ''
  const endpointText = String(endpoint).trim()
  const rawHost = endpointText.replace(/^https?:\/\//i, '').split('/')[0]
  if (rawHost.includes('*')) return rawHost
  try {
    const urlText = /^https?:\/\//i.test(endpointText) ? endpointText : `http://${endpointText}`
    return decodeURIComponent(new URL(urlText).host)
  } catch (error) {
    return rawHost
  }
}

const getDefaultIngressHost = (row = {}) => {
  const ruleHost = (row.rules || []).map(rule => rule.host).find(host => host && host !== '*')
  if (ruleHost) return ruleHost
  if (Array.isArray(row.endpoints) && row.endpoints.length > 0) {
    return getHostFromEndpoint(row.endpoints[0])
  }
  return ''
}

const getDefaultIngressPath = (row = {}, host = '') => {
  const normalizedHost = getHostFromEndpoint(host) || host
  for (const rule of (row.rules || [])) {
    if (normalizedHost && rule.host && rule.host !== normalizedHost) {
      continue
    }
    const firstPath = getIngressRulePaths(rule)[0]
    if (firstPath?.path) {
      return firstPath.path
    }
  }
  return '/'
}

const normalizeIngressPath = (path = '/') => {
  const value = String(path || '/').trim()
  return value.startsWith('/') ? value : `/${value}`
}

const getUniqueIngressRows = (rows = []) => {
  const uniqueMap = new Map()
  rows.forEach((row) => {
    if (!row?.name) return
    uniqueMap.set(`${row.namespace || queryParams.namespace || ''}/${row.name}`, row)
  })
  return [...uniqueMap.values()]
}

const getRelatedIngressRows = (row = {}) => {
  const rowName = row?.name
  if (!rowName) return []

  return getUniqueIngressRows([
    row,
    ...ingressAiCandidateIngresses.value.filter(item => item?.name === rowName),
    ...ingressList.value.filter(item => item?.name === rowName)
  ])
}

const getIngressHostCandidates = (row = {}) => {
  const hosts = []
  getRelatedIngressRows(row).forEach((ingress) => {
    const rules = ingress.rules || []
    const endpoints = ingress.endpoints || []

    rules.forEach((rule) => {
      if (getIngressRulePaths(rule).length && rule.host && rule.host !== '*') {
        hosts.push(rule.host)
      }
    })
    endpoints.forEach((endpoint) => {
      const host = getHostFromEndpoint(endpoint)
      if (host && host !== '*') {
        hosts.push(host)
      }
    })
  })
  return [...new Set(hosts)]
}

const getIngressPathCandidates = (row = {}, host = '') => {
  const selectedHost = getHostFromEndpoint(host) || host
  const paths = []
  getRelatedIngressRows(row).forEach((ingress) => {
    const rules = ingress.rules || []
    rules.forEach((rule) => {
      const hostMatched = !selectedHost || !rule.host || rule.host === selectedHost
      if (!hostMatched) return
      getIngressRulePaths(rule).forEach((path) => {
        paths.push(path.path || '/')
      })
    })
  })

  if (!paths.length) {
    getRelatedIngressRows(row).forEach((ingress) => {
      const rules = ingress.rules || []
      rules.forEach((rule) => {
        getIngressRulePaths(rule).forEach((path) => {
          paths.push(path.path || '/')
        })
      })
    })
  }

  return [...new Set(paths)]
}

const ingressAiHostOptions = computed(() => getIngressHostCandidates(currentIngressForAi.value))
const ingressAiPathOptions = computed(() => getIngressPathCandidates(currentIngressForAi.value, ingressAiForm.host))

const findIngressByHostAndPath = (host = '', path = '') => {
  const selectedHost = getHostFromEndpoint(host) || host
  const selectedPath = normalizeIngressPath(path || '/')
  const rows = getRelatedIngressRows(currentIngressForAi.value)
  return rows.find((ingress) => (
    (ingress.rules || []).some((rule) => (
      rule.host === selectedHost &&
      getIngressRulePaths(rule).some(item => normalizeIngressPath(item.path || '/') === selectedPath)
    ))
  )) || rows.find((ingress) => (
    (ingress.rules || []).some(rule => rule.host === selectedHost)
  ))
}

const handleIngressAiHostChange = () => {
  const pathOptions = getIngressPathCandidates(currentIngressForAi.value, ingressAiForm.host)
  if (pathOptions.length && !pathOptions.includes(ingressAiForm.path)) {
    ingressAiForm.path = pathOptions[0]
  }
  handleIngressAiPathChange()
}

const handleIngressAiPathChange = () => {
  const matchedIngress = findIngressByHostAndPath(ingressAiForm.host, ingressAiForm.path)
  if (matchedIngress) {
    currentIngressForAi.value = matchedIngress
  }
}

const closeIngressAiStream = () => {
  if (ingressAiStreamSource.value) {
    ingressAiStreamSource.value.close()
    ingressAiStreamSource.value = null
  }
}

const getIngressAiStepLabel = (step) => {
  return ingressAiStepDefinitions.find(item => item.step === step)?.label || step || '诊断步骤'
}

const resetIngressAiProgress = () => {
  ingressAiProgress.value = 0
  ingressAiMessage.value = ''
}

const updateIngressAiProgress = (event = {}) => {
  const progress = Number(event.progress)
  if (Number.isFinite(progress)) {
    ingressAiProgress.value = Math.max(0, Math.min(100, progress))
  }
  ingressAiMessage.value = event.message || getIngressAiStepLabel(event.step || event.type)
}

const handleIngressAiStreamEvent = (event = {}) => {
  if (!event || typeof event !== 'object') return

  updateIngressAiProgress(event)

  if (event.type === 'result' || event.step === 'result') {
    ingressAiProgress.value = 100
    ingressAiResult.value = event.result || event.data || event
    ingressAiLoading.value = false
    closeIngressAiStream()
    ElMessage.success('AI 诊断完成')
  }
}

const parseIngressAiStreamMessage = (messageEvent) => {
  if (!messageEvent?.data) return
  try {
    handleIngressAiStreamEvent(JSON.parse(messageEvent.data))
  } catch (error) {
    console.error('解析 Ingress AI 诊断流失败:', error, messageEvent.data)
  }
}

const loadIngressAiCandidateIngresses = async () => {
  try {
    const response = await k8sApi.getIngressList(selectedClusterId.value, queryParams.namespace, {
      page: 1,
      pageSize: 1000
    })
    const responseData = response.data || response
    if (responseData.code === 200 || responseData.success) {
      const rawData = responseData.data?.ingresses || responseData.data
      ingressAiCandidateIngresses.value = Array.isArray(rawData) ? rawData : []
    } else {
      ingressAiCandidateIngresses.value = Array.isArray(ingressList.value) ? ingressList.value : []
    }
  } catch (error) {
    console.error('获取 Ingress AI 候选域名失败:', error)
    ingressAiCandidateIngresses.value = Array.isArray(ingressList.value) ? ingressList.value : []
  }
}

// Ingress 操作
const handleCreateIngress = () => {
  if (!selectedClusterId.value) {
    ElMessage.warning('请先选择集群')
    return
  }
  if (!queryParams.namespace) {
    ElMessage.warning('请先选择命名空间')
    return
  }

  // 设置默认的Ingress YAML模板（使用v1beta1版本以兼容旧版本K8s）
  const defaultYaml = `apiVersion: networking.k8s.io/v1beta1
kind: Ingress
metadata:
  name: new-ingress
  namespace: ${queryParams.namespace}
spec:
  rules:
  - host: example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          serviceName: my-service
          servicePort: 80`

  currentIngressYaml.value = defaultYaml
  currentIngressForDetail.value = { name: 'new-ingress', namespace: queryParams.namespace }
  ingressYamlDialogVisible.value = true
}

// 查看 Ingress 详情
const handleViewIngress = async (row) => {
  try {
    const response = await k8sApi.getIngressDetail(selectedClusterId.value, queryParams.namespace, row.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      currentIngressForDetail.value = responseData.data || row
      ingressDetailDialogVisible.value = true
    } else {
      ElMessage.error(responseData.message || '获取 Ingress 详情失败')
    }
  } catch (error) {
    console.error('获取 Ingress 详情失败:', error)
    ElMessage.error('获取 Ingress 详情失败')
  }
}

// 编辑 Ingress YAML
const handleEditIngressYaml = async (row) => {
  try {
    loading.value = true
    const response = await k8sApi.getIngressYaml(selectedClusterId.value, queryParams.namespace, row.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      currentIngressYaml.value = getYamlContentFromApiResponse(
        responseData,
        `# Ingress ${row.name} YAML\napiVersion: networking.k8s.io/v1\nkind: Ingress\nmetadata:\n  name: ${row.name}`
      )
      currentIngressForDetail.value = row
      ingressYamlDialogVisible.value = true
    } else {
      ElMessage.error(responseData.message || '获取 Ingress YAML 失败')
    }
  } catch (error) {
    console.error('获取 Ingress YAML 失败:', error)
    ElMessage.error('获取 Ingress YAML 失败')
  } finally {
    loading.value = false
  }
}

// 查看 Ingress 事件
const handleViewIngressEvents = async (row) => {
  try {
    currentIngressForEvents.value = row
    const response = await k8sApi.getIngressEvents(selectedClusterId.value, queryParams.namespace, row.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ingressEvents.value = responseData.data || []
    } else {
      ingressEvents.value = []
      ElMessage.warning('暂无事件数据')
    }
    ingressEventsDialogVisible.value = true
  } catch (error) {
    console.error('获取 Ingress 事件失败:', error)
    ingressEvents.value = []
    ingressEventsDialogVisible.value = true
  }
}

// 测试 Ingress 后端服务
const handleTestIngress = async (row) => {
  try {
    // 从 Ingress 数据中提取第一个规则的信息作为默认值
    if (row.rules && row.rules.length > 0) {
      const firstRule = row.rules[0]
      ingressTestForm.host = firstRule.host || ''

      const rulePaths = getIngressRulePaths(firstRule)
      if (rulePaths.length > 0) {
        const firstPath = rulePaths[0]
        ingressTestForm.path = firstPath.path || '/'
        ingressTestForm.serviceName = getIngressPathServiceName(firstPath)
        ingressTestForm.servicePort = getIngressPathServicePort(firstPath)
      }
    }

    ingressTestResult.value = null
    ingressTestDialogVisible.value = true
  } catch (error) {
    console.error('打开测试对话框失败:', error)
    ElMessage.error('打开测试对话框失败')
  }
}

const resetIngressAiForm = () => {
  Object.assign(ingressAiForm, {
    host: '',
    path: '/',
    statusCode: '502',
    observedError: ''
  })
  ingressAiResult.value = null
  currentIngressForAi.value = {}
  ingressAiCandidateIngresses.value = []
  closeIngressAiStream()
  resetIngressAiProgress()
}

const handleAnalyzeIngress = async (row) => {
  if (!selectedClusterId.value) {
    ElMessage.warning('请先选择集群')
    return
  }
  if (!queryParams.namespace) {
    ElMessage.warning('请先选择命名空间')
    return
  }

  currentIngressForAi.value = row || {}
  ingressAiCandidateIngresses.value = Array.isArray(ingressList.value) ? ingressList.value : []
  await loadIngressAiCandidateIngresses()
  const hostOptions = getIngressHostCandidates(row)
  const defaultHost = hostOptions[0] || getDefaultIngressHost(row)
  const pathOptions = getIngressPathCandidates(row, defaultHost)
  Object.assign(ingressAiForm, {
    host: defaultHost,
    path: pathOptions[0] || getDefaultIngressPath(row, defaultHost),
    statusCode: '502',
    observedError: ''
  })
  ingressAiResult.value = null
  resetIngressAiProgress()
  ingressAiDialogVisible.value = true
}

const executeIngressAiAnalyze = async () => {
  const host = ingressAiForm.host.trim()
  if (!host) {
    ElMessage.warning('请输入访问域名')
    return
  }

  try {
    closeIngressAiStream()
    resetIngressAiProgress()
    ingressAiLoading.value = true
    ingressAiResult.value = null
    const matchedIngress = findIngressByHostAndPath(host, ingressAiForm.path)
    const requestIngress = matchedIngress?.name || currentIngressForAi.value?.name || ''
    const streamParams = {
      clusterId: selectedClusterId.value,
      namespace: queryParams.namespace,
      ingress: requestIngress,
      host,
      path: normalizeIngressPath(ingressAiForm.path),
      statusCode: ingressAiForm.statusCode || undefined,
      observedError: ingressAiForm.observedError.trim(),
      includeBackend: true,
      includeLogs: true,
      includeEvents: true
    }

    const source = k8sApi.createIngressAnalyzeStream(streamParams)
    ingressAiStreamSource.value = source
    source.onmessage = parseIngressAiStreamMessage
    source.addEventListener('started', parseIngressAiStreamMessage)
    source.addEventListener('step', parseIngressAiStreamMessage)
    source.addEventListener('result', parseIngressAiStreamMessage)
    source.addEventListener('error', parseIngressAiStreamMessage)
    source.onerror = () => {
      if (ingressAiResult.value) {
        closeIngressAiStream()
        return
      }
      if (ingressAiLoading.value) {
        ingressAiLoading.value = false
        updateIngressAiProgress({
          step: 'result',
          status: 'failed',
          progress: ingressAiProgress.value,
          message: '诊断流连接异常，请稍后重试'
        })
        ElMessage.error('AI 诊断流连接异常，请稍后重试')
      }
      closeIngressAiStream()
    }
  } catch (error) {
    console.error('AI 诊断 Ingress 失败:', error)
    ElMessage.error(error.response?.data?.message || 'AI 诊断失败，请稍后重试')
    ingressAiLoading.value = false
  }
}

const getIngressAiStatusTagType = (status = '') => {
  const statusText = String(status).toLowerCase()
  if (['normal', 'healthy', 'success'].includes(statusText)) return 'success'
  if (['abnormal', 'error', 'failed', 'critical'].includes(statusText)) return 'danger'
  if (['warning', 'warn'].includes(statusText)) return 'warning'
  return 'info'
}

const getIngressAiSeverityTagType = (severity = '') => {
  const severityText = String(severity).toLowerCase()
  if (['critical', 'high', 'error', 'fatal'].includes(severityText)) return 'danger'
  if (['warning', 'medium', 'warn'].includes(severityText)) return 'warning'
  if (['low', 'info', 'normal'].includes(severityText)) return 'info'
  return 'primary'
}

const hasIngressAiValue = (value) => {
  if (Array.isArray(value)) return value.length > 0
  if (value && typeof value === 'object') return Object.keys(value).length > 0
  return Boolean(value)
}

const formatIngressAiJson = (value) => {
  try {
    return JSON.stringify(value, null, 2)
  } catch (error) {
    return String(value)
  }
}

// 执行 Ingress 后端测试
const executeIngressTest = async () => {
  if (!ingressTestForm.serviceName) {
    ElMessage.warning('请输入服务名称')
    return
  }

  if (!ingressTestForm.servicePort) {
    ElMessage.warning('请输入服务端口')
    return
  }

  try {
    ingressTestLoading.value = true
    ingressTestResult.value = null

    const response = await k8sApi.testIngressBackend(
      selectedClusterId.value,
      queryParams.namespace,
      {
        serviceName: ingressTestForm.serviceName,
        servicePort: ingressTestForm.servicePort,
        path: ingressTestForm.path || '/',
        host: ingressTestForm.host || '',
        method: ingressTestForm.method || 'GET',
        timeout: ingressTestForm.timeout || 10
      }
    )

    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ingressTestResult.value = responseData.data || responseData

      if (responseData.data?.success || responseData.success) {
        ElMessage.success('后端服务测试完成')
      } else {
        ElMessage.warning('后端服务测试发现问题,请查看详细信息')
      }
    } else {
      ElMessage.error(responseData.message || '测试失败')
      ingressTestResult.value = {
        success: false,
        status: '异常',
        message: responseData.message || '测试失败'
      }
    }
  } catch (error) {
    console.error('测试 Ingress 后端失败:', error)
    ElMessage.error(error.response?.data?.message || '测试失败,请检查网络连接')
    ingressTestResult.value = {
      success: false,
      status: '异常',
      message: error.response?.data?.message || error.message || '测试失败'
    }
  } finally {
    ingressTestLoading.value = false
  }
}

// 重置测试表单
const resetTestForm = () => {
  Object.assign(ingressTestForm, {
    serviceName: '',
    servicePort: 80,
    path: '/',
    host: '',
    method: 'GET',
    timeout: 10
  })
  ingressTestResult.value = null
}

// 保存 Service YAML
const handleServiceYamlSave = async (data) => {
  try {
    // 判断是否是创建新Service
    // 条件：资源名为 'new-service' 或者 currentServiceForDetail 没有 name 属性
    const isCreating = data.resourceName === 'new-service' || !currentServiceForDetail.value?.name

    let response
    if (isCreating) {
      // 创建新Service，使用createPodFromYaml API（支持多种资源类型）
      response = await k8sApi.createPodFromYaml(selectedClusterId.value, queryParams.namespace, { yamlContent: data.yamlContent })
    } else {
      // 更新现有Service，使用 PUT 接口
      response = await k8sApi.updateServiceYaml(selectedClusterId.value, queryParams.namespace, data.resourceName, data.yamlContent)
    }

    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(isCreating ? 'Service 创建成功' : 'Service YAML 更新成功')
      serviceYamlDialogVisible.value = false
      isCreating ? refreshServiceListFromFirstPage() : fetchServiceList()
    } else {
      ElMessage.error(responseData.message || (isCreating ? 'Service 创建失败' : 'Service YAML 更新失败'))
    }
  } catch (error) {
    console.error('Service 操作失败:', error)
    ElMessage.error(error.response?.data?.message || 'Service 操作失败')
  }
}

// 保存 Ingress YAML
const handleIngressYamlSave = async (data) => {
  try {
    // 判断是否是创建新Ingress
    // 条件：资源名为 'new-ingress' 或者 currentIngressForDetail 没有 name 属性
    const isCreating = data.resourceName === 'new-ingress' || !currentIngressForDetail.value?.name

    let response
    if (isCreating) {
      // 创建新Ingress，使用createPodFromYaml API（支持多种资源类型）
      response = await k8sApi.createPodFromYaml(selectedClusterId.value, queryParams.namespace, { yamlContent: data.yamlContent })
    } else {
      // 更新现有Ingress，使用 PUT 接口
      response = await k8sApi.updateIngressYaml(selectedClusterId.value, queryParams.namespace, data.resourceName, data.yamlContent)
    }

    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(isCreating ? 'Ingress 创建成功' : 'Ingress YAML 更新成功')
      ingressYamlDialogVisible.value = false
      isCreating ? refreshIngressListFromFirstPage() : fetchIngressList()
    } else {
      ElMessage.error(responseData.message || (isCreating ? 'Ingress 创建失败' : 'Ingress YAML 更新失败'))
    }
  } catch (error) {
    console.error('Ingress 操作失败:', error)
    ElMessage.error(error.response?.data?.message || 'Ingress 操作失败')
  }
}

const handleEditIngress = async (row) => {
  try {
    const response = await k8sApi.getIngressDetail(selectedClusterId.value, queryParams.namespace, row.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      const ingressData = responseData.data
      Object.assign(currentIngress, {
        name: ingressData.name,
        annotations: ingressData.annotations || {},
        rules: ingressData.rules || [{ host: '', paths: [{ path: '/', pathType: 'Prefix', serviceName: '', servicePort: 80 }] }],
        tls: ingressData.tls || [],
        description: ingressData.description || ''
      })
      editIngressDialogVisible.value = true
    }
  } catch (error) {
    ElMessage.error('获取 Ingress 详情失败')
  }
}

const handleDeleteIngress = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除 Ingress "${row.name}" 吗？此操作不可恢复。`,
      '删除确认',
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )

    const response = await k8sApi.deleteIngress(selectedClusterId.value, queryParams.namespace, row.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success('Ingress 删除成功')
      fetchIngressList()
    } else {
      ElMessage.error(responseData.message || 'Ingress 删除失败')
    }
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('Ingress 删除失败')
    }
  }
}

// 表单重置
const resetServiceForm = () => {
  Object.assign(currentService, {
    name: '',
    labels: {},
    type: 'ClusterIP',
    selector: {},
    ports: [{ name: '', port: 80, targetPort: 80, protocol: 'TCP' }],
    description: ''
  })
}

const resetIngressForm = () => {
  Object.assign(currentIngress, {
    name: '',
    annotations: {},
    rules: [{ host: '', paths: [{ path: '/', pathType: 'Prefix', serviceName: '', servicePort: 80 }] }],
    tls: [],
    description: ''
  })
}

// Service 端口管理
const addServicePort = () => {
  currentService.ports.push({ name: '', port: 80, targetPort: 80, protocol: 'TCP' })
}

const removeServicePort = (index) => {
  if (currentService.ports.length > 1) {
    currentService.ports.splice(index, 1)
  }
}

// Ingress 规则管理
const addIngressRule = () => {
  currentIngress.rules.push({ host: '', paths: [{ path: '/', pathType: 'Prefix', serviceName: '', servicePort: 80 }] })
}

const removeIngressRule = (index) => {
  if (currentIngress.rules.length > 1) {
    currentIngress.rules.splice(index, 1)
  }
}

const addIngressPath = (ruleIndex) => {
  currentIngress.rules[ruleIndex].paths.push({ path: '/', pathType: 'Prefix', serviceName: '', servicePort: 80 })
}

const removeIngressPath = (ruleIndex, pathIndex) => {
  if (currentIngress.rules[ruleIndex].paths.length > 1) {
    currentIngress.rules[ruleIndex].paths.splice(pathIndex, 1)
  }
}

// 提交表单
const submitServiceForm = async () => {
  try {
    const response = await k8sApi.createService(selectedClusterId.value, queryParams.namespace, currentService)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success('Service 创建成功')
      serviceDialogVisible.value = false
      refreshServiceListFromFirstPage()
    } else {
      ElMessage.error(responseData.message || 'Service 创建失败')
    }
  } catch (error) {
    ElMessage.error('Service 创建失败')
  }
}

const submitEditServiceForm = async () => {
  try {
    const response = await k8sApi.updateService(selectedClusterId.value, queryParams.namespace, currentService.name, currentService)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success('Service 更新成功')
      editServiceDialogVisible.value = false
      fetchServiceList()
    } else {
      ElMessage.error(responseData.message || 'Service 更新失败')
    }
  } catch (error) {
    ElMessage.error('Service 更新失败')
  }
}

const submitIngressForm = async () => {
  try {
    const response = await k8sApi.createIngress(selectedClusterId.value, queryParams.namespace, currentIngress)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success('Ingress 创建成功')
      ingressDialogVisible.value = false
      refreshIngressListFromFirstPage()
    } else {
      ElMessage.error(responseData.message || 'Ingress 创建失败')
    }
  } catch (error) {
    ElMessage.error('Ingress 创建失败')
  }
}

const submitEditIngressForm = async () => {
  try {
    const response = await k8sApi.updateIngress(selectedClusterId.value, queryParams.namespace, currentIngress.name, currentIngress)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success('Ingress 更新成功')
      editIngressDialogVisible.value = false
      fetchIngressList()
    } else {
      ElMessage.error(responseData.message || 'Ingress 更新失败')
    }
  } catch (error) {
    ElMessage.error('Ingress 更新失败')
  }
}

// 复制名称到剪贴板
const copyToClipboard = async (text, resourceType = '资源') => {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success(`${resourceType}名称已复制到剪贴板`)
  } catch (error) {
    console.error('复制失败:', error)
    // 降级方案：使用传统的复制方法
    try {
      const textArea = document.createElement('textarea')
      textArea.value = text
      document.body.appendChild(textArea)
      textArea.select()
      document.execCommand('copy')
      document.body.removeChild(textArea)
      ElMessage.success(`${resourceType}名称已复制到剪贴板`)
    } catch (fallbackError) {
      ElMessage.error('复制失败，请手动复制')
    }
  }
}

// 组件挂载时初始化
onMounted(async () => {
  console.log('K8s网络管理页面初始化完成')
})
</script>

<template>
  <div class="k8s-network-management">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/k8s.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>K8s 网络管理</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>统一查看 Service 与 Ingress，保持集群、命名空间和入口链路的可扫描状态。</p>
      </div>
      <div class="header-actions">
        <ClusterSelector
          v-model="selectedClusterId"
          width="220px"
          size="small"
          margin-right="0"
          theme="cluster"
          @change="handleClusterChange"
        />
        <NamespaceSelector
          v-model="queryParams.namespace"
          :cluster-id="selectedClusterId"
          filterable
          width="240px"
          size="small"
          theme="namespace"
          @change="handleNamespaceChange"
        />
      </div>
    </header>

    <section class="network-card page-card">

      <!-- 搜索表单和资源类型选择 -->
      <div class="search-section">
        <el-form :inline="true" class="search-form" @submit.prevent>
          <el-form-item label="网络资源名称">
            <el-input
              v-model="searchKeyword"
              placeholder="请输入名称、类型或域名"
              clearable
              class="search-name-input"
              @keyup.enter="handleRefresh"
            >
              <template #prefix>
                <el-icon class="search-name-icon"><Search /></el-icon>
              </template>
            </el-input>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :icon="Search" class="query-button" @click="handleRefresh">
              查询
            </el-button>
            <el-button type="warning" :icon="Refresh" class="refresh-button" @click="handleRefresh">
              刷新
            </el-button>
          </el-form-item>
        </el-form>

        <!-- 资源类型选择器和创建按钮 -->
        <div class="resource-controls">
          <el-tabs v-model="activeTab" @tab-change="handleTabChange" class="network-tabs">
            <el-tab-pane label="Service" name="service" />
            <el-tab-pane label="Ingress" name="ingress" />
          </el-tabs>
        </div>
      </div>

      <!-- 网络资源表格 -->
      <section class="table-section">
        <div class="content-header">
          <span class="resource-count">{{ currentResourceCountText }}</span>
          <div class="create-actions">
            <el-button
              v-if="activeTab === 'service'"
              type="success"
              :icon="Plus"
              class="primary-action"
              @click="handleCreateService"
            >
              创建 Service
            </el-button>
            <el-button
              v-if="activeTab === 'ingress'"
              type="success"
              :icon="Plus"
              class="primary-action"
              @click="handleCreateIngress"
            >
              创建 Ingress
            </el-button>
          </div>
        </div>
        <div class="table-content">
        <!-- Service 表格 -->
        <div v-if="activeTab === 'service'" class="tab-content">
          <div class="table-shell">
            <el-table
              :data="filteredServiceList"
              v-loading="serviceLoading"
              element-loading-text="加载中..."
              class="resource-table"
              empty-text="暂无 Service 资源"
            >
              <el-table-column prop="name" label="名称" min-width="180">
                <template #default="{ row }">
                  <div class="resource-name">
                    <el-icon class="resource-icon"><Connection /></el-icon>
                    <span class="resource-name-link" @click="handleViewService(row)">{{ row.name }}</span>
                    <el-tooltip content="复制名称" placement="top">
                      <el-button
                        :icon="CopyDocument"
                        size="small"
                        type="text"
                        class="copy-button"
                        @click.stop="copyToClipboard(row.name, 'Service')"
                      />
                    </el-tooltip>
                  </div>
                </template>
              </el-table-column>

              <el-table-column prop="type" label="类型" width="120">
                <template #default="{ row }">
                  <el-tag
                    :type="row.type === 'ClusterIP' ? 'success' : row.type === 'NodePort' ? 'warning' : 'primary'"
                    size="small"
                  >
                    {{ row.type }}
                  </el-tag>
                </template>
              </el-table-column>

              <el-table-column prop="clusterIP" label="集群 IP" width="140" />

              <el-table-column label="端口" min-width="200">
                <template #default="{ row }">
                  <div class="ports-info">
                    <el-tag
                      v-for="(port, index) in (row.ports || [])"
                      :key="index"
                      size="small"
                      class="port-tag"
                      :type="row.type === 'NodePort' && port.nodePort ? 'warning' : 'primary'"
                    >
                      {{ port.port }}{{ port.name ? ':' + port.name : '' }}/{{ port.protocol }}
                      <span v-if="row.type === 'NodePort' && port.nodePort" class="nodeport-info">
                        →{{ port.nodePort }}
                      </span>
                    </el-tag>
                  </div>
                </template>
              </el-table-column>

              <el-table-column label="选择器" min-width="150">
                <template #default="{ row }">
                  <div class="selector-info">
                    <el-tag
                      v-for="(value, key) in (row.selector || {})"
                      :key="key"
                      size="small"
                      type="info"
                      class="selector-tag"
                    >
                      {{ key }}={{ value }}
                    </el-tag>
                  </div>
                </template>
              </el-table-column>

              <el-table-column label="创建时间" width="160">
                <template #default="{ row }">
                  {{ row.createdAt ? new Date(row.createdAt).toLocaleString() : '-' }}
                </template>
              </el-table-column>

              <el-table-column label="操作" width="190" fixed="right">
                <template #default="{ row }">
                  <div class="operation-buttons">
                    <button
                      type="button"
                      class="table-action action-primary"
                      @click="handleViewService(row)"
                    >详情</button>
                    <el-dropdown trigger="click">
                      <button type="button" class="table-action action-more">
                        更多<el-icon class="el-icon--right"><ArrowDown /></el-icon>
                      </button>
                      <template #dropdown>
                        <el-dropdown-menu>
                          <el-dropdown-item
                            @click="handleEditService(row)"
                          >编辑 Service</el-dropdown-item>
                          <el-dropdown-item
                            @click="handleEditServiceYaml(row)"
                          >编辑 YAML</el-dropdown-item>
                          <el-dropdown-item
                            @click="handleViewServiceEvents(row)"
                          >查看事件</el-dropdown-item>
                          <el-dropdown-item
                            @click="handleDeleteService(row)"
                          >删除</el-dropdown-item>
                        </el-dropdown-menu>
                      </template>
                    </el-dropdown>
                  </div>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </div>

        <!-- Ingress 表格 -->
        <div v-if="activeTab === 'ingress'" class="tab-content">
          <div class="table-shell">
            <el-table
              :data="filteredIngressList"
              v-loading="ingressLoading"
              element-loading-text="加载中..."
              class="resource-table"
              empty-text="暂无 Ingress 资源"
            >
              <el-table-column prop="name" label="名称" min-width="200">
                <template #default="{ row }">
                  <div class="resource-name">
                    <el-icon class="resource-icon"><Monitor /></el-icon>
                    <span class="resource-name-link" @click="handleViewIngress(row)">{{ row.name }}</span>
                    <el-tooltip content="复制名称" placement="top">
                      <el-button
                        :icon="CopyDocument"
                        size="small"
                        type="text"
                        class="copy-button"
                        @click.stop="copyToClipboard(row.name, 'Ingress')"
                      />
                    </el-tooltip>
                  </div>
                </template>
              </el-table-column>

              <el-table-column label="类型" width="120">
                <template #default="{ row }">
                  <el-tag type="primary" size="small">
                    {{ row.type || '-' }}
                  </el-tag>
                </template>
              </el-table-column>

              <el-table-column label="访问 IP" width="140">
                <template #default="{ row }">
                  <span v-if="row.loadBalancer && row.loadBalancer.ingress && row.loadBalancer.ingress.length > 0">
                    {{ row.loadBalancer.ingress[0].ip || row.loadBalancer.ingress[0].hostname || '-' }}
                  </span>
                  <span v-else>-</span>
                </template>
              </el-table-column>

              <el-table-column label="Service:端口" min-width="200">
                <template #default="{ row }">
                  <div class="services-info">
                    <template v-for="(rule, ruleIndex) in (row.rules || [])" :key="ruleIndex">
                      <el-tag
                        v-for="(path, pathIndex) in getIngressRulePaths(rule)"
                        :key="`${ruleIndex}-${pathIndex}`"
                        size="small"
                        type="success"
                        class="service-tag"
                      >
                        {{ getIngressPathServiceName(path) || '-' }}:{{ getIngressPathServicePort(path) || '-' }}
                      </el-tag>
                    </template>
                  </div>
                </template>
              </el-table-column>

              <el-table-column label="Ingress 域名" min-width="250">
                <template #default="{ row }">
                  <div class="endpoints-info">
                    <template v-if="row.endpoints && row.endpoints.length > 0">
                      <div
                        v-for="(endpoint, index) in row.endpoints"
                        :key="index"
                        class="endpoint-item"
                      >
                        <a :href="endpoint" target="_blank" class="endpoint-link">
                          {{ endpoint }}
                        </a>
                        <el-tooltip content="复制链接" placement="top">
                          <el-button
                            :icon="CopyDocument"
                            size="small"
                            type="text"
                            class="copy-button"
                            @click.stop="copyToClipboard(endpoint, '域名')"
                          />
                        </el-tooltip>
                      </div>
                    </template>
                    <template v-else>
                      <el-tag
                        v-for="(rule, index) in (row.rules || [])"
                        :key="index"
                        size="small"
                        type="info"
                        class="host-tag"
                      >
                        {{ rule.host || '*' }}
                      </el-tag>
                    </template>
                  </div>
                </template>
              </el-table-column>

              <el-table-column label="TLS" width="80">
                <template #default="{ row }">
                  <el-tag
                    v-if="row.tls && row.tls.length > 0"
                    type="success"
                    size="small"
                  >
                    启用
                  </el-tag>
                  <el-tag
                    v-else
                    type="info"
                    size="small"
                  >
                    未启用
                  </el-tag>
                </template>
              </el-table-column>

              <el-table-column label="创建时间" width="160">
                <template #default="{ row }">
                  {{ row.createdAt ? new Date(row.createdAt).toLocaleString() : '-' }}
                </template>
              </el-table-column>

              <el-table-column label="操作" width="270" fixed="right">
                <template #default="{ row }">
                  <div class="operation-buttons">
                    <button
                      type="button"
                      class="table-action action-primary"
                      @click="handleViewIngress(row)"
                    >详情</button>
                    <button
                      type="button"
                      class="table-action action-ai"
                      @click="handleAnalyzeIngress(row)"
                    >AI 诊断</button>
                    <el-dropdown trigger="click">
                      <button type="button" class="table-action action-more"  >
                        更多<el-icon class="el-icon--right"><ArrowDown /></el-icon>
                      </button>
                      <template #dropdown>
                        <el-dropdown-menu>
                          <el-dropdown-item
                            @click="handleEditIngress(row)"
                          >编辑 Ingress</el-dropdown-item>
                          <el-dropdown-item
                            @click="handleEditIngressYaml(row)"
                          >编辑 YAML</el-dropdown-item>
                          <el-dropdown-item
                            @click="handleTestIngress(row)"
                          >测试后端服务</el-dropdown-item>
                          <el-dropdown-item
                            @click="handleViewIngressEvents(row)"
                          >查看事件</el-dropdown-item>
                          <el-dropdown-item
                            @click="handleDeleteIngress(row)"
                          >删除</el-dropdown-item>
                        </el-dropdown-menu>
                      </template>
                    </el-dropdown>
                  </div>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </div>

        <div class="pagination-wrapper">
          <el-pagination
            :current-page="getCurrentResourceCurrentPage()"
            :page-size="getCurrentResourcePageSize()"
            :total="getCurrentResourceTotal()"
            :page-sizes="[10, 20, 50, 100]"
            layout="total, sizes, prev, pager, next, jumper"
            background
            @size-change="handleNetworkSizeChange"
            @current-change="handleNetworkCurrentChange"
          />
        </div>
        </div>
      </section>
    </section>

    <!-- Service 详情对话框 -->
    <el-dialog
      v-model="serviceDetailDialogVisible"
      title="Service 详情"
      width="800px"
      class="detail-dialog"
      modal-class="k8s-network-dialog-overlay"
    >
      <div class="detail-content">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="名称">{{ currentServiceForDetail.name }}</el-descriptions-item>
          <el-descriptions-item label="命名空间">{{ currentServiceForDetail.namespace }}</el-descriptions-item>
          <el-descriptions-item label="类型">
            <el-tag :type="currentServiceForDetail.type === 'ClusterIP' ? 'success' : 'primary'">
              {{ currentServiceForDetail.type }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="集群IP">{{ currentServiceForDetail.clusterIP }}</el-descriptions-item>
          <el-descriptions-item label="创建时间" :span="2">
            {{ currentServiceForDetail.createdAt ? new Date(currentServiceForDetail.createdAt).toLocaleString() : '-' }}
          </el-descriptions-item>
          <el-descriptions-item label="标签" :span="2">
            <div class="labels-info">
              <el-tag
                v-for="(value, key) in (currentServiceForDetail.labels || {})"
                :key="key"
                size="small"
                type="info"
                style="margin-right: 4px;"
              >
                {{ key }}={{ value }}
              </el-tag>
            </div>
          </el-descriptions-item>
          <el-descriptions-item label="端口" :span="2">
            <div class="ports-info">
              <el-tag
                v-for="(port, index) in (currentServiceForDetail.ports || [])"
                :key="index"
                size="small"
                style="margin-right: 4px;"
                :type="currentServiceForDetail.type === 'NodePort' && port.nodePort ? 'warning' : 'primary'"
              >
                {{ port.port }}{{ port.name ? ':' + port.name : '' }}/{{ port.protocol }}
                {{ port.targetPort ? ' → ' + port.targetPort : '' }}
                <span v-if="currentServiceForDetail.type === 'NodePort' && port.nodePort" class="nodeport-info">
                  (NodePort: {{ port.nodePort }})
                </span>
              </el-tag>
            </div>
          </el-descriptions-item>
        </el-descriptions>
      </div>
    </el-dialog>

    <!-- Service YAML 编辑对话框 -->
    <PodYamlDialog
      v-model:visible="serviceYamlDialogVisible"
      :yaml-content="currentServiceYaml"
      :resource-name="currentServiceForDetail?.name || ''"
      resource-type="Service"
      :editable="true"
      @save="handleServiceYamlSave"
      @close="serviceYamlDialogVisible = false"
    />

    <!-- Service 事件对话框 -->
    <el-dialog
      v-model="serviceEventsDialogVisible"
      title="Service 事件"
      width="1000px"
      class="events-dialog"
      modal-class="k8s-network-dialog-overlay"
    >
      <div class="events-content">
        <el-table :data="serviceEvents" stripe style="width: 100%">
          <el-table-column prop="type" label="类型" width="80">
            <template #default="{ row }">
              <el-tag :type="row.type === 'Warning' ? 'warning' : 'success'" size="small">
                {{ row.type }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="reason" label="原因" width="120" />
          <el-table-column prop="message" label="消息" min-width="300" />
          <el-table-column prop="firstTimestamp" label="首次时间" width="160">
            <template #default="{ row }">
              {{ row.firstTimestamp ? new Date(row.firstTimestamp).toLocaleString() : '-' }}
            </template>
          </el-table-column>
          <el-table-column prop="count" label="次数" width="80" />
        </el-table>
      </div>
    </el-dialog>

    <!-- Ingress 详情对话框 -->
    <el-dialog
      v-model="ingressDetailDialogVisible"
      title="Ingress 详情"
      width="800px"
      class="detail-dialog"
      modal-class="k8s-network-dialog-overlay"
    >
      <div class="detail-content">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="名称">{{ currentIngressForDetail.name }}</el-descriptions-item>
          <el-descriptions-item label="命名空间">{{ currentIngressForDetail.namespace }}</el-descriptions-item>
          <el-descriptions-item label="TLS" :span="2">
            <el-tag :type="currentIngressForDetail.tls && currentIngressForDetail.tls.length > 0 ? 'success' : 'info'" size="small">
              {{ currentIngressForDetail.tls && currentIngressForDetail.tls.length > 0 ? '启用' : '未启用' }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="创建时间" :span="2">
            {{ currentIngressForDetail.createdAt ? new Date(currentIngressForDetail.createdAt).toLocaleString() : '-' }}
          </el-descriptions-item>
          <el-descriptions-item label="规则" :span="2">
            <div class="rules-info">
              <div v-for="(rule, index) in (currentIngressForDetail.rules || [])" :key="index" style="margin-bottom: 8px;">
                <strong>{{ rule.host || '*' }}</strong>
                <div v-for="(path, pathIndex) in getIngressRulePaths(rule)" :key="pathIndex" style="margin-left: 16px;">
                  <el-tag size="small" type="info">
                    {{ path.path }} → {{ getIngressPathServiceName(path) }}:{{ getIngressPathServicePort(path) }}
                  </el-tag>
                </div>
              </div>
            </div>
          </el-descriptions-item>
        </el-descriptions>
      </div>
    </el-dialog>

    <!-- Ingress YAML 编辑对话框 -->
    <PodYamlDialog
      v-model:visible="ingressYamlDialogVisible"
      :yaml-content="currentIngressYaml"
      :resource-name="currentIngressForDetail?.name || ''"
      resource-type="Ingress"
      :editable="true"
      @save="handleIngressYamlSave"
      @close="ingressYamlDialogVisible = false"
    />

    <!-- Ingress 事件对话框 -->
    <el-dialog
      v-model="ingressEventsDialogVisible"
      title="Ingress 事件"
      width="1000px"
      class="events-dialog"
      modal-class="k8s-network-dialog-overlay"
    >
      <div class="events-content">
        <el-table :data="ingressEvents" stripe style="width: 100%">
          <el-table-column prop="type" label="类型" width="80">
            <template #default="{ row }">
              <el-tag :type="row.type === 'Warning' ? 'warning' : 'success'" size="small">
                {{ row.type }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="reason" label="原因" width="120" />
          <el-table-column prop="message" label="消息" min-width="300" />
          <el-table-column prop="firstTimestamp" label="首次时间" width="160">
            <template #default="{ row }">
              {{ row.firstTimestamp ? new Date(row.firstTimestamp).toLocaleString() : '-' }}
            </template>
          </el-table-column>
          <el-table-column prop="count" label="次数" width="80" />
        </el-table>
      </div>
    </el-dialog>

    <!-- 创建 Service 对话框 -->
    <el-dialog
      v-model="serviceDialogVisible"
      title="创建 Service"
      width="700px"
      class="create-dialog compact-dialog"
      modal-class="k8s-network-dialog-overlay"
      @close="resetServiceForm"
    >
      <el-form :model="currentService" label-width="90px" class="compact-form">
        <div class="form-row">
          <el-form-item label="名称" required class="form-item-half">
            <el-input
              v-model="currentService.name"
              placeholder="请输入 Service 名称"
              size="small"
            />
          </el-form-item>
          <el-form-item label="类型" required class="form-item-half">
            <el-select v-model="currentService.type" style="width: 100%" size="small">
              <el-option
                v-for="option in serviceTypeOptions"
                :key="option.value"
                :label="option.label"
                :value="option.value"
              >
                <div class="option-content">
                  <span>{{ option.label }}</span>
                  <span class="option-desc">{{ option.description }}</span>
                </div>
              </el-option>
            </el-select>
          </el-form-item>
        </div>

        <el-form-item label="端口配置" required>
          <div class="ports-config">
            <div
              v-for="(port, index) in currentService.ports"
              :key="index"
              class="port-item"
            >
              <div class="port-fields">
                <el-input
                  v-model="port.name"
                  placeholder="端口d(可选)"
                  size="small"
                  style="width: 120px;"
                />
                <el-input-number
                  v-model="port.port"
                  :min="1"
                  :max="65535"
                  placeholder="端口"
                  size="small"
                  style="width: 100px;"
                />
                <el-input-number
                  v-model="port.targetPort"
                  :min="1"
                  :max="65535"
                  placeholder="目标端口"
                  size="small"
                  style="width: 100px;"
                />
                <el-select v-model="port.protocol" size="small" style="width: 80px;">
                  <el-option
                    v-for="protocol in protocolOptions"
                    :key="protocol.value"
                    :label="protocol.label"
                    :value="protocol.value"
                  />
                </el-select>
                <el-button
                  type="danger"
                  :icon="Delete"
                  size="small"
                  plain
                  @click="removeServicePort(index)"
                  :disabled="currentService.ports.length === 1"
                />
              </div>
            </div>
            <el-button type="success" :icon="Plus" size="small" @click="addServicePort">
              添加端口
            </el-button>
          </div>
        </el-form-item>

        <el-form-item label="描述">
          <el-input
            v-model="currentService.description"
            type="textarea"
            :rows="2"
            placeholder="请输入服务描述"
            size="small"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="serviceDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitServiceForm">创建</el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 编辑 Service 对话框 -->
    <el-dialog
      v-model="editServiceDialogVisible"
      title="编辑 Service"
      width="700px"
      class="edit-dialog compact-dialog"
      modal-class="k8s-network-dialog-overlay"
      @close="resetServiceForm"
    >
      <el-form :model="currentService" label-width="90px" class="compact-form">
        <div class="form-row">
          <el-form-item label="名称" class="form-item-half">
            <el-input
              v-model="currentService.name"
              disabled
              size="small"
            />
          </el-form-item>
          <el-form-item label="类型" required class="form-item-half">
            <el-select v-model="currentService.type" style="width: 100%" size="small">
              <el-option
                v-for="option in serviceTypeOptions"
                :key="option.value"
                :label="option.label"
                :value="option.value"
              >
                <div class="option-content">
                  <span>{{ option.label }}</span>
                  <span class="option-desc">{{ option.description }}</span>
                </div>
              </el-option>
            </el-select>
          </el-form-item>
        </div>

        <el-form-item label="端口配置" required>
          <div class="ports-config">
            <div
              v-for="(port, index) in currentService.ports"
              :key="index"
              class="port-item"
            >
              <div class="port-fields">
                <el-input
                  v-model="port.name"
                  placeholder="端口名称(可选)"
                  size="small"
                  style="width: 120px;"
                />
                <el-input-number
                  v-model="port.port"
                  :min="1"
                  :max="65535"
                  placeholder="端口"
                  size="small"
                  style="width: 100px;"
                />
                <el-input-number
                  v-model="port.targetPort"
                  :min="1"
                  :max="65535"
                  placeholder="目标端口"
                  size="small"
                  style="width: 100px;"
                />
                <el-select v-model="port.protocol" size="small" style="width: 80px;">
                  <el-option
                    v-for="protocol in protocolOptions"
                    :key="protocol.value"
                    :label="protocol.label"
                    :value="protocol.value"
                  />
                </el-select>
                <el-button
                  type="danger"
                  :icon="Delete"
                  size="small"
                  plain
                  @click="removeServicePort(index)"
                  :disabled="currentService.ports.length === 1"
                />
              </div>
            </div>
            <el-button type="success" :icon="Plus" size="small" @click="addServicePort">
              添加端口
            </el-button>
          </div>
        </el-form-item>

        <el-form-item label="描述">
          <el-input
            v-model="currentService.description"
            type="textarea"
            :rows="2"
            placeholder="请输入服务描述"
            size="small"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="editServiceDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitEditServiceForm">更新</el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 创建 Ingress 对话框 -->
    <el-dialog
      v-model="ingressDialogVisible"
      title="创建 Ingress"
      width="800px"
      class="create-dialog compact-dialog"
      modal-class="k8s-network-dialog-overlay"
      @close="resetIngressForm"
    >
      <el-form :model="currentIngress" label-width="90px" class="compact-form">
        <el-form-item label="名称" required>
          <el-input
            v-model="currentIngress.name"
            placeholder="请输入 Ingress 名称"
            size="small"
            style="width: 300px;"
          />
        </el-form-item>

        <el-form-item label="规则配置" required>
          <div class="rules-config">
            <div
              v-for="(rule, ruleIndex) in currentIngress.rules"
              :key="ruleIndex"
              class="rule-item"
            >
              <div class="rule-header">
                <el-input
                  v-model="rule.host"
                  placeholder="主机名 (如: example.com)"
                  size="small"
                  style="width: 250px;"
                />
                <el-button
                  type="danger"
                  :icon="Delete"
                  size="small"
                  plain
                  @click="removeIngressRule(ruleIndex)"
                  :disabled="currentIngress.rules.length === 1"
                >
                  删除规则
                </el-button>
              </div>

              <div class="paths-config">
                <div
                  v-for="(path, pathIndex) in rule.paths"
                  :key="pathIndex"
                  class="path-item"
                >
                  <div class="path-fields">
                    <el-input
                      v-model="path.path"
                      placeholder="路径"
                      size="small"
                      style="width: 120px;"
                    />
                    <el-select v-model="path.pathType" size="small" style="width: 150px;">
                      <el-option
                        v-for="pathType in pathTypeOptions"
                        :key="pathType.value"
                        :label="pathType.label"
                        :value="pathType.value"
                      >
                        <div class="option-content">
                          <span>{{ pathType.label }}</span>
                          <span class="option-desc">{{ pathType.description }}</span>
                        </div>
                      </el-option>
                    </el-select>
                    <el-input
                      v-model="path.serviceName"
                      placeholder="服务名"
                      size="small"
                      style="width: 120px;"
                    />
                    <el-input-number
                      v-model="path.servicePort"
                      :min="1"
                      :max="65535"
                      placeholder="端口"
                      size="small"
                      style="width: 100px;"
                    />
                    <el-button
                      type="danger"
                      :icon="Delete"
                      size="small"
                      plain
                      @click="removeIngressPath(ruleIndex, pathIndex)"
                      :disabled="rule.paths.length === 1"
                    />
                  </div>
                </div>
                <el-button
                  type="success"
                  :icon="Plus"
                  size="small"
                  @click="addIngressPath(ruleIndex)"
                >
                  添加路径
                </el-button>
              </div>
            </div>
            <el-button type="success" :icon="Plus" size="small" @click="addIngressRule">
              添加规则
            </el-button>
          </div>
        </el-form-item>

        <el-form-item label="描述">
          <el-input
            v-model="currentIngress.description"
            type="textarea"
            :rows="2"
            placeholder="请输入 Ingress 描述"
            size="small"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="ingressDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitIngressForm">创建</el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 编辑 Ingress 对话框 -->
    <el-dialog
      v-model="editIngressDialogVisible"
      title="编辑 Ingress"
      width="800px"
      class="edit-dialog compact-dialog"
      modal-class="k8s-network-dialog-overlay"
      @close="resetIngressForm"
    >
      <el-form :model="currentIngress" label-width="90px" class="compact-form">
        <el-form-item label="名称">
          <el-input
            v-model="currentIngress.name"
            disabled
            size="small"
            style="width: 300px;"
          />
        </el-form-item>

        <el-form-item label="规则配置" required>
          <div class="rules-config">
            <div
              v-for="(rule, ruleIndex) in currentIngress.rules"
              :key="ruleIndex"
              class="rule-item"
            >
              <div class="rule-header">
                <el-input
                  v-model="rule.host"
                  placeholder="主机名 (如: example.com)"
                  size="small"
                  style="width: 250px;"
                />
                <el-button
                  type="danger"
                  :icon="Delete"
                  size="small"
                  plain
                  @click="removeIngressRule(ruleIndex)"
                  :disabled="currentIngress.rules.length === 1"
                >
                  删除规则
                </el-button>
              </div>

              <div class="paths-config">
                <div
                  v-for="(path, pathIndex) in rule.paths"
                  :key="pathIndex"
                  class="path-item"
                >
                  <div class="path-fields">
                    <el-input
                      v-model="path.path"
                      placeholder="路径"
                      size="small"
                      style="width: 120px;"
                    />
                    <el-select v-model="path.pathType" size="small" style="width: 150px;">
                      <el-option
                        v-for="pathType in pathTypeOptions"
                        :key="pathType.value"
                        :label="pathType.label"
                        :value="pathType.value"
                      >
                        <div class="option-content">
                          <span>{{ pathType.label }}</span>
                          <span class="option-desc">{{ pathType.description }}</span>
                        </div>
                      </el-option>
                    </el-select>
                    <el-input
                      v-model="path.serviceName"
                      placeholder="服务名"
                      size="small"
                      style="width: 120px;"
                    />
                    <el-input-number
                      v-model="path.servicePort"
                      :min="1"
                      :max="65535"
                      placeholder="端口"
                      size="small"
                      style="width: 100px;"
                    />
                    <el-button
                      type="danger"
                      :icon="Delete"
                      size="small"
                      plain
                      @click="removeIngressPath(ruleIndex, pathIndex)"
                      :disabled="rule.paths.length === 1"
                    />
                  </div>
                </div>
                <el-button
                  type="success"
                  :icon="Plus"
                  size="small"
                  @click="addIngressPath(ruleIndex)"
                >
                  添加路径
                </el-button>
              </div>
            </div>
            <el-button type="success" :icon="Plus" size="small" @click="addIngressRule">
              添加规则
            </el-button>
          </div>
        </el-form-item>

        <el-form-item label="描述">
          <el-input
            v-model="currentIngress.description"
            type="textarea"
            :rows="2"
            placeholder="请输入 Ingress 描述"
            size="small"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="editIngressDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitEditIngressForm">更新</el-button>
        </span>
      </template>
    </el-dialog>

    <!-- Ingress AI 诊断对话框 -->
    <el-dialog
      v-model="ingressAiDialogVisible"
      title="AI 诊断 Ingress"
      width="900px"
      class="ai-diagnosis-dialog"
      modal-class="k8s-network-ai-dialog-overlay"
      @close="resetIngressAiForm"
    >
      <div class="ai-diagnosis-content">
        <div class="ai-diagnosis-target">
          <div>
            <span>当前 Ingress</span>
            <strong>{{ currentIngressForAi.name || '-' }}</strong>
          </div>
          <div>
            <span>命名空间</span>
            <strong>{{ queryParams.namespace || '-' }}</strong>
          </div>
        </div>

        <el-form :model="ingressAiForm" label-width="84px" size="small" class="ai-diagnosis-form">
          <el-row :gutter="12">
            <el-col :span="12">
              <el-form-item label="访问域名" required>
                <el-select
                  v-model="ingressAiForm.host"
                  filterable
                  allow-create
                  default-first-option
                  placeholder="支持纯域名、带协议或完整 URL"
                  style="width: 100%;"
                  clearable
                  @change="handleIngressAiHostChange"
                >
                  <el-option
                    v-for="host in ingressAiHostOptions"
                    :key="host"
                    :label="host"
                    :value="host"
                  />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="访问路径">
                <el-select
                  v-model="ingressAiForm.path"
                  filterable
                  allow-create
                  default-first-option
                  placeholder="如 / 或 /api/health"
                  style="width: 100%;"
                  clearable
                  @change="handleIngressAiPathChange"
                >
                  <el-option
                    v-for="path in ingressAiPathOptions"
                    :key="path"
                    :label="path"
                    :value="path"
                  />
                </el-select>
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="12">
            <el-col :span="12">
              <el-form-item label="故障状态码">
                <el-select
                  v-model="ingressAiForm.statusCode"
                  filterable
                  allow-create
                  default-first-option
                  placeholder="如 502"
                  style="width: 100%;"
                >
                  <el-option
                    v-for="code in ingressStatusCodeOptions"
                    :key="code"
                    :label="code"
                    :value="code"
                  />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="故障现象">
                <el-input
                  v-model="ingressAiForm.observedError"
                  placeholder="如 访问页面返回 502"
                  clearable
                />
              </el-form-item>
            </el-col>
          </el-row>

        </el-form>

        <div v-if="ingressAiLoading || ingressAiProgress > 0 || ingressAiMessage" class="ai-diagnosis-flow">
          <div class="ai-flow-head">
            <div>
              <h4>诊断进度</h4>
              <p>{{ ingressAiMessage || '准备开始诊断' }}</p>
            </div>
            <el-tag :type="ingressAiLoading ? 'warning' : (ingressAiResult ? 'success' : 'info')">
              {{ ingressAiLoading ? '诊断中' : (ingressAiResult ? '已完成' : '待开始') }}
            </el-tag>
          </div>
          <el-progress
            :percentage="ingressAiProgress"
            :status="ingressAiResult ? 'success' : undefined"
            :stroke-width="8"
            class="ai-flow-progress"
          />
        </div>

        <div v-if="ingressAiResult" class="ai-diagnosis-result">
          <div class="ai-result-header">
            <div>
              <h3>{{ ingressAiResult.conclusion || ingressAiResult.summary || 'AI 诊断完成' }}</h3>
              <p v-if="ingressAiResult.summary && ingressAiResult.summary !== ingressAiResult.conclusion">
                {{ ingressAiResult.summary }}
              </p>
            </div>
            <div class="ai-result-tags">
              <el-tag v-if="ingressAiResult.status" :type="getIngressAiStatusTagType(ingressAiResult.status)">
                {{ ingressAiResult.status }}
              </el-tag>
              <el-tag v-if="ingressAiResult.severity" :type="getIngressAiSeverityTagType(ingressAiResult.severity)">
                {{ ingressAiResult.severity }}
              </el-tag>
            </div>
          </div>

          <el-descriptions :column="3" border class="ai-result-meta">
            <el-descriptions-item label="模型">
              {{ ingressAiResult.providerName || '-' }} / {{ ingressAiResult.modelCode || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="日志行数">
              {{ ingressAiResult.usedLogLines ?? ingressAiResult.logLines ?? 0 }}
            </el-descriptions-item>
            <el-descriptions-item label="事件数">
              {{ ingressAiResult.usedEventCount ?? ingressAiResult.eventCount ?? 0 }}
            </el-descriptions-item>
            <el-descriptions-item label="域名">
              {{ ingressAiResult.host || ingressAiForm.host || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="路径">
              {{ ingressAiResult.path || ingressAiForm.path || '/' }}
            </el-descriptions-item>
            <el-descriptions-item label="生成时间">
              {{ ingressAiResult.generatedAt || '-' }}
            </el-descriptions-item>
          </el-descriptions>

          <section v-if="ingressAiResult.remediation && ingressAiResult.remediation.length" class="ai-result-section">
            <h4>处理建议</h4>
            <ol class="ai-remediation-list">
              <li v-for="(item, index) in ingressAiResult.remediation" :key="index">{{ item }}</li>
            </ol>
          </section>

          <section v-if="ingressAiResult.analysis" class="ai-result-section">
            <div class="ai-section-header">
              <h4>完整分析</h4>
              <el-button
                size="small"
                text
                :icon="CopyDocument"
                @click="copyToClipboard(ingressAiResult.analysis, 'AI诊断结果')"
              >
                复制
              </el-button>
            </div>
            <div class="ai-analysis-text">{{ ingressAiResult.analysis }}</div>
          </section>

          <el-collapse class="ai-evidence-collapse">
            <el-collapse-item
              v-if="hasIngressAiValue(ingressAiResult.suspectedReasons)"
              title="疑似原因"
              name="suspectedReasons"
            >
              <pre class="code-block">{{ formatIngressAiJson(ingressAiResult.suspectedReasons) }}</pre>
            </el-collapse-item>
            <el-collapse-item
              v-if="hasIngressAiValue(ingressAiResult.matchedBackends)"
              title="匹配后端"
              name="matchedBackends"
            >
              <pre class="code-block">{{ formatIngressAiJson(ingressAiResult.matchedBackends) }}</pre>
            </el-collapse-item>
            <el-collapse-item
              v-if="hasIngressAiValue(ingressAiResult.collectionErrors)"
              title="采集提示"
              name="collectionErrors"
            >
              <pre class="code-block">{{ formatIngressAiJson(ingressAiResult.collectionErrors) }}</pre>
            </el-collapse-item>
            <el-collapse-item
              v-if="hasIngressAiValue(ingressAiResult.evidence)"
              title="诊断证据"
              name="evidence"
            >
              <pre class="code-block">{{ formatIngressAiJson(ingressAiResult.evidence) }}</pre>
            </el-collapse-item>
          </el-collapse>
        </div>
      </div>

      <template #footer>
        <span class="dialog-footer">
          <el-button @click="ingressAiDialogVisible = false">取消</el-button>
          <el-button
            type="primary"
            :loading="ingressAiLoading"
            :icon="DataAnalysis"
            @click="executeIngressAiAnalyze"
          >
            {{ ingressAiLoading ? '诊断中...' : (ingressAiResult ? '重新诊断' : '开始诊断') }}
          </el-button>
        </span>
      </template>
    </el-dialog>

    <!-- Ingress 测试对话框 -->
    <el-dialog
      v-model="ingressTestDialogVisible"
      title="测试 Ingress 后端服务"
      width="900px"
      class="test-dialog"
      modal-class="k8s-network-dialog-overlay"
      @close="resetTestForm"
    >
      <div class="test-content">
        <el-form :model="ingressTestForm" label-width="100px" class="test-form">
          <el-row :gutter="20">
            <el-col :span="12">
              <el-form-item label="服务名称" required>
                <el-input
                  v-model="ingressTestForm.serviceName"
                  placeholder="请输入Service名称"
                  size="small"
                />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="服务端口" required>
                <el-input-number
                  v-model="ingressTestForm.servicePort"
                  :min="1"
                  :max="65535"
                  placeholder="请输入端口"
                  size="small"
                  style="width: 100%;"
                />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="20">
            <el-col :span="12">
              <el-form-item label="测试路径">
                <el-input
                  v-model="ingressTestForm.path"
                  placeholder="请输入测试路径,如:/api/health"
                  size="small"
                />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="Host域名">
                <el-input
                  v-model="ingressTestForm.host"
                  placeholder="请输入Host域名(可选)"
                  size="small"
                />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="20">
            <el-col :span="12">
              <el-form-item label="请求方法">
                <el-select v-model="ingressTestForm.method" size="small" style="width: 100%;">
                  <el-option label="GET" value="GET" />
                  <el-option label="POST" value="POST" />
                  <el-option label="PUT" value="PUT" />
                  <el-option label="DELETE" value="DELETE" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="超时时间(秒)">
                <el-input-number
                  v-model="ingressTestForm.timeout"
                  :min="1"
                  :max="60"
                  size="small"
                  style="width: 100%;"
                />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item>
            <el-button
              type="primary"
              :loading="ingressTestLoading"
              @click="executeIngressTest"
            >
              {{ ingressTestLoading ? '测试中...' : '开始测试' }}
            </el-button>
          </el-form-item>
        </el-form>

        <!-- 测试结果显示区域 -->
        <div v-if="ingressTestResult" class="test-result">
          <el-divider content-position="left">测试结果</el-divider>

          <el-alert
            :title="ingressTestResult.status || '测试完成'"
            :type="ingressTestResult.success ? 'success' : 'error'"
            :closable="false"
            show-icon
          >
            <template #default>
              <div class="result-message">
                <p><strong>{{ ingressTestResult.message }}</strong></p>
                <p v-if="ingressTestResult.suggestion" class="suggestion">
                  💡 建议: {{ ingressTestResult.suggestion }}
                </p>
              </div>
            </template>
          </el-alert>

          <el-descriptions :column="2" border class="result-details" v-if="ingressTestResult.details">
            <el-descriptions-item label="服务名称">
              {{ ingressTestResult.details.serviceName || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="服务端口">
              {{ ingressTestResult.details.servicePort || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="测试路径">
              {{ ingressTestResult.details.testPath || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="Host域名">
              {{ ingressTestResult.details.testHost || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="请求方法">
              {{ ingressTestResult.details.method || '-' }}
            </el-descriptions-item>
            <el-descriptions-item label="HTTP状态码">
              <el-tag
                :type="ingressTestResult.statusCode >= 200 && ingressTestResult.statusCode < 300 ? 'success' : 'danger'"
                size="small"
              >
                {{ ingressTestResult.statusCode || '-' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="响应时间">
              <el-tag
                :type="ingressTestResult.responseTime < 1000 ? 'success' : 'warning'"
                size="small"
              >
                {{ ingressTestResult.responseTime }}ms
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="Endpoints状态">
              {{ ingressTestResult.details.endpointsReady || 0 }} / {{ ingressTestResult.details.endpointsTotal || 0 }} 就绪
            </el-descriptions-item>
          </el-descriptions>

          <!-- 显示已测试的Pod列表 -->
          <div v-if="ingressTestResult.details?.podsTested && ingressTestResult.details.podsTested.length > 0" class="pods-tested">
            <h4>已测试的Pod:</h4>
            <el-tag
              v-for="pod in ingressTestResult.details.podsTested"
              :key="pod"
              size="small"
              type="info"
              style="margin-right: 8px; margin-bottom: 8px;"
            >
              {{ pod }}
            </el-tag>
          </div>

          <!-- 显示响应头 -->
          <div v-if="ingressTestResult.details?.responseHeaders" class="response-headers">
            <el-divider content-position="left">响应头</el-divider>
            <el-scrollbar max-height="200px">
              <pre class="code-block">{{ JSON.stringify(ingressTestResult.details.responseHeaders, null, 2) }}</pre>
            </el-scrollbar>
          </div>

          <!-- 显示响应体 -->
          <div v-if="ingressTestResult.details?.responseBody" class="response-body">
            <el-divider content-position="left">响应体</el-divider>
            <el-scrollbar max-height="300px">
              <pre class="code-block">{{ ingressTestResult.details.responseBody }}</pre>
            </el-scrollbar>
          </div>
        </div>
      </div>

      <template #footer>
        <span class="dialog-footer">
          <el-button @click="ingressTestDialogVisible = false">关闭</el-button>
          <el-button type="primary" :loading="ingressTestLoading" @click="executeIngressTest">
            重新测试
          </el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.k8s-network-management {
  --network-page-height-offset: 190px;
  padding: 5px;
  min-height: calc(100vh - var(--network-page-height-offset));
  background: #eef1f6;
}

.network-card {
  display: flex;
  flex-direction: column;
  max-height: calc(100vh - 10px - var(--network-page-height-offset));
  min-height: 0;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-radius: 16px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.network-card :deep(.el-card__body) {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
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
  background-clip: text;
  -webkit-text-fill-color: transparent;
}

.header-actions {
  display: flex;
  gap: 12px;
}

.search-section {
  margin-bottom: 20px;
  padding: 16px;
  background: rgba(255, 255, 255, 0.5);
  border-radius: 12px;
  border: 1px solid rgba(103, 126, 234, 0.1);
}

.search-form {
  margin: 0;
}

.resource-controls {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid rgba(103, 126, 234, 0.1);
}

.network-tabs {
  flex: 1;
  margin: 0;
}

.network-tabs :deep(.el-tabs__header) {
  margin-bottom: 0;
}

.network-tabs :deep(.el-tabs__item) {
  font-weight: 500;
  color: #606266;
}

.network-tabs :deep(.el-tabs__item.is-active) {
  color: #409EFF;
  font-weight: 600;
}

.create-actions {
  flex-shrink: 0;
  margin-top: 8px;
}

.cluster-option,
.namespace-option {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.cluster-name {
  font-weight: 500;
  color: #6b9bd1;
}

.cluster-status {
  margin-left: 8px;
}

.namespace-name {
  font-weight: 500;
  color: #6b9bd1;
}

.namespace-status-tag {
  margin-left: 8px;
}

.selector-section .el-select {
  border-radius: 8px;
}

.selector-section .el-select :deep(.el-input__wrapper) {
  background: rgba(255, 255, 255, 0.9);
  border: 1px solid rgba(103, 126, 234, 0.3);
  border-radius: 8px;
  transition: all 0.3s ease;
}

.selector-section .el-select :deep(.el-input__wrapper):hover {
  border-color: #667eea;
}

.selector-section .el-select :deep(.el-input__wrapper.is-focus) {
  border-color: #667eea;
  box-shadow: 0 0 0 2px rgba(103, 126, 234, 0.2);
}

.header-actions {
  display: flex;
  align-items: center;
}

.tab-content {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  padding: 0;
}

.table-content {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  overflow: auto;
}

.pagination-wrapper {
  flex-shrink: 0;
  display: flex;
  justify-content: flex-end;
  padding: 14px 4px 0;
}


.resource-table {
  border-radius: 8px;
  overflow: hidden;
}

.resource-table :deep(.el-table__header) {
  background: rgba(103, 126, 234, 0.1);
}

.resource-table :deep(.el-table__header th) {
  background: rgba(103, 126, 234, 0.1);
  color: #6b9bd1;
  font-weight: 600;
  border: none;
  text-align: left;
}

.resource-table :deep(.el-table__header th .cell),
.resource-table :deep(.el-table__body td .cell) {
  text-align: left;
}

.resource-table :deep(.el-table__row) {
  transition: all 0.3s ease;
}

.resource-table :deep(.el-table__row:hover) {
  background: rgba(103, 126, 234, 0.05);
}

.resource-name {
  display: flex;
  align-items: center;
  gap: 8px;
}

.resource-icon {
  color: #667eea;
  font-size: 16px;
}

.resource-name-link {
  color: #667eea;
  cursor: pointer;
  font-weight: 500;
  font-size: 14px;
  transition: all 0.3s ease;
  text-decoration: none;
  border-bottom: 1px solid transparent;
  flex: 1;
}

.resource-name-link:hover {
  color: #409eff;
  border-bottom-color: #409eff;
  transform: translateY(-1px);
}

.copy-button {
  color: #909399;
  padding: 2px 4px;
  margin-left: 4px;
  opacity: 0;
  transition: all 0.3s ease;
  min-width: auto;
  height: auto;
}

.resource-name:hover .copy-button {
  opacity: 1;
}

.copy-button:hover {
  color: #409eff;
  background-color: rgba(64, 158, 255, 0.1);
  transform: scale(1.1);
}

.ports-info,
.hosts-info,
.paths-info,
.selector-info,
.services-info {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.endpoints-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.port-tag,
.host-tag,
.path-tag,
.selector-tag,
.service-tag {
  font-size: 11px;
  border-radius: 4px;
  margin: 1px;
}

.endpoint-item {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-bottom: 4px;
}

.endpoint-link {
  color: #409eff;
  text-decoration: none;
  font-size: 12px;
  transition: all 0.3s ease;
  border-bottom: 1px solid transparent;
}

.endpoint-link:hover {
  color: #66b1ff;
  border-bottom-color: #66b1ff;
}

.nodeport-info {
  font-weight: 600;
  color: #E6A23C;
  margin-left: 2px;
}

/* 操作按钮样式 */
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

/* 兼容旧的 action-buttons 样式 */
.action-buttons {
  display: flex;
  gap: 4px;
  align-items: center;
}

.action-buttons .el-button {
  padding: 4px 8px;
  font-size: 11px;
}

.action-buttons .el-dropdown {
  margin-right: 4px;
}

.action-buttons .el-dropdown .el-button {
  font-size: 11px;
  padding: 4px 8px;
}

/* 详情对话框样式 */
.detail-dialog :deep(.el-dialog) {
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
}

.detail-dialog :deep(.el-dialog__header) {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: white;
  border-top-left-radius: 16px;
  border-top-right-radius: 16px;
  padding: 20px 24px;
}

.detail-dialog :deep(.el-dialog__title) {
  color: white;
  font-weight: 600;
}

.detail-content {
  padding: 20px;
}

/* YAML 对话框样式 */
.yaml-dialog :deep(.el-dialog) {
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
}

.yaml-dialog :deep(.el-dialog__header) {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: white;
  border-top-left-radius: 16px;
  border-top-right-radius: 16px;
  padding: 20px 24px;
}

.yaml-dialog :deep(.el-dialog__title) {
  color: white;
  font-weight: 600;
}

.yaml-content {
  padding: 20px;
}

.yaml-content .el-textarea :deep(.el-textarea__inner) {
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 8px;
  font-family: 'Courier New', 'Consolas', monospace;
  line-height: 1.4;
}

/* 事件对话框样式 */
.events-dialog :deep(.el-dialog) {
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
}

.events-dialog :deep(.el-dialog__header) {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: white;
  border-top-left-radius: 16px;
  border-top-right-radius: 16px;
  padding: 20px 24px;
}

.events-dialog :deep(.el-dialog__title) {
  color: white;
  font-weight: 600;
}

.events-content {
  padding: 20px;
}

/* 对话框样式 */
.create-dialog :deep(.el-dialog),
.edit-dialog :deep(.el-dialog) {
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
}

.create-dialog :deep(.el-dialog__header),
.edit-dialog :deep(.el-dialog__header) {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: white;
  border-top-left-radius: 16px;
  border-top-right-radius: 16px;
  padding: 20px 24px;
}

.create-dialog :deep(.el-dialog__title),
.edit-dialog :deep(.el-dialog__title) {
  color: white;
  font-weight: 600;
}

.create-dialog :deep(.el-dialog__body),
.edit-dialog :deep(.el-dialog__body) {
  padding: 24px;
}

.compact-dialog :deep(.el-dialog__body) {
  padding: 15px 20px;
}

.compact-form {
  margin: 0;
}

.compact-form :deep(.el-form-item) {
  margin-bottom: 12px;
}

.form-row {
  display: flex;
  gap: 16px;
  margin-bottom: 12px;
}

.form-item-half {
  flex: 1;
  margin-bottom: 0 !important;
}

.option-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.option-desc {
  font-size: 11px;
  color: #909399;
  margin-left: 8px;
}

/* 端口配置样式 */
.ports-config {
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  padding: 12px;
  background: rgba(249, 250, 251, 0.5);
}

.port-item {
  margin-bottom: 8px;
  padding: 8px;
  border: 1px solid #e4e7ed;
  border-radius: 6px;
  background: white;
}

.port-fields {
  display: flex;
  gap: 8px;
  align-items: center;
}

/* 规则配置样式 */
.rules-config {
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  padding: 12px;
  background: rgba(249, 250, 251, 0.5);
}

.rule-item {
  margin-bottom: 16px;
  padding: 12px;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  background: white;
}

.rule-header {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid #f0f0f0;
}

.paths-config {
  padding-left: 12px;
}

.path-item {
  margin-bottom: 8px;
  padding: 8px;
  border: 1px solid #f0f0f0;
  border-radius: 4px;
  background: rgba(249, 250, 251, 0.3);
}

.path-fields {
  display: flex;
  gap: 8px;
  align-items: center;
}

.dialog-footer {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
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

.el-textarea :deep(.el-textarea__inner) {
  border-radius: 8px;
  border: 1px solid rgba(103, 126, 234, 0.2);
  transition: all 0.3s ease;
}

.el-textarea :deep(.el-textarea__inner):focus {
  border-color: #667eea;
  box-shadow: 0 0 0 2px rgba(103, 126, 234, 0.2);
}

/* 响应式设计 */
@media (max-width: 768px) {
  .k8s-network-management {
    --network-page-height-offset: 131px;
    padding: 10px;
  }

  .network-card {
    max-height: calc(100vh - 20px - var(--network-page-height-offset));
    min-height: 0;
  }

  .card-header {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }

  .header-actions {
    justify-content: flex-end;
    flex-wrap: wrap;
  }

  .header-actions .el-select {
    width: 100% !important;
    margin-bottom: 8px;
  }

  .search-section {
    padding: 12px;
  }

  .search-form {
    flex-direction: column;
    align-items: stretch;
  }

  .search-form :deep(.el-form-item) {
    margin-bottom: 8px;
  }

  .resource-controls {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }

  .network-tabs,
  .create-actions {
    width: 100%;
  }

  .create-actions {
    justify-content: center;
    margin-top: 12px;
  }


  .form-row {
    flex-direction: column;
    gap: 8px;
  }

  .port-fields,
  .path-fields {
    flex-direction: column;
    gap: 8px;
  }

  .rule-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .operation-buttons {
    gap: 4px;
  }

  .operation-buttons .el-button {
    width: 32px;
    height: 32px;
  }

  .action-buttons {
    flex-direction: column;
    gap: 2px;
  }

  .action-buttons .el-button {
    width: 100%;
    margin: 0;
  }

  .action-buttons .el-dropdown {
    width: 100%;
    margin-right: 0;
    margin-bottom: 2px;
  }

  .action-buttons .el-dropdown .el-button {
    width: 100%;
  }

  .resource-name-link {
    font-size: 13px;
    line-height: 1.2;
  }
}

/* AI 诊断对话框样式 */
.ai-diagnosis-dialog :deep(.el-dialog) {
  border: 1px solid rgba(15, 23, 42, 0.06);
  border-radius: 22px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98) 0%, rgba(247, 250, 255, 0.96) 100%);
  box-shadow: 0 36px 110px rgba(15, 23, 42, 0.11);
  overflow: hidden;
}

.ai-diagnosis-dialog :deep(.el-dialog__header) {
  margin: 0;
  padding: 16px 20px 14px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(15, 23, 42, 0.06);
}

.ai-diagnosis-dialog :deep(.el-dialog__title) {
  color: #172133;
  font-size: 17px;
  font-weight: 700;
}

.ai-diagnosis-dialog :deep(.el-dialog__body) {
  padding: 0;
}

.ai-diagnosis-dialog :deep(.el-dialog__footer) {
  padding: 10px 20px 14px;
  background: rgba(255, 253, 248, 0.78);
  border-top: 1px solid rgba(15, 23, 42, 0.06);
}

.ai-diagnosis-dialog :deep(.el-dialog__headerbtn) {
  top: 13px;
  right: 16px;
}

.ai-diagnosis-dialog :deep(.dialog-footer .el-button) {
  min-width: 92px;
  height: 30px;
}

.ai-diagnosis-content {
  max-height: 70vh;
  overflow-y: auto;
  padding: 14px 20px 16px;
}

.ai-diagnosis-target {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 12px;
}

.ai-diagnosis-target > div {
  padding: 9px 12px;
  border: 1px solid rgba(207, 217, 230, 0.82);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.74);
}

.ai-diagnosis-target span {
  display: block;
  margin-bottom: 4px;
  color: #8d97a8;
  font-size: 11px;
}

.ai-diagnosis-target strong {
  display: block;
  color: #172133;
  font-size: 13px;
  overflow-wrap: anywhere;
}

.ai-diagnosis-form {
  padding: 12px 14px 0;
  border: 1px solid rgba(207, 217, 230, 0.78);
  border-radius: 16px;
  background: rgba(255, 253, 248, 0.72);
}

.ai-diagnosis-form :deep(.el-form-item) {
  margin-bottom: 12px;
}

.ai-diagnosis-form :deep(.el-form-item__label) {
  color: #5f6b7c;
  font-size: 13px;
}

.ai-diagnosis-flow {
  margin-top: 12px;
  padding: 12px 14px;
  border: 1px solid rgba(207, 217, 230, 0.78);
  border-radius: 16px;
  background: rgba(247, 250, 255, 0.78);
}

.ai-flow-head {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 8px;
}

.ai-flow-head h4 {
  margin: 0;
  color: #172133;
  font-size: 14px;
}

.ai-flow-head p {
  margin: 4px 0 0;
  color: #5f6b7c;
  font-size: 13px;
  line-height: 1.4;
}

.ai-flow-progress {
  margin-bottom: 0;
}

.ai-diagnosis-result {
  margin-top: 12px;
  padding: 14px 16px;
  border: 1px solid rgba(207, 217, 230, 0.78);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.76);
}

.ai-result-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.ai-result-header h3 {
  margin: 0;
  color: #172133;
  font-size: 16px;
  line-height: 1.35;
}

.ai-result-header p {
  margin: 6px 0 0;
  color: #5f6b7c;
  font-size: 13px;
  line-height: 1.55;
}

.ai-result-tags {
  display: flex;
  flex-shrink: 0;
  gap: 8px;
}

.ai-result-meta {
  margin-bottom: 12px;
}

.ai-result-section {
  margin-top: 12px;
}

.ai-result-section h4,
.ai-section-header h4 {
  margin: 0 0 8px;
  color: #172133;
  font-size: 14px;
}

.ai-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.ai-remediation-list {
  margin: 0;
  padding-left: 18px;
  color: #3d4858;
  font-size: 13px;
  line-height: 1.58;
}

.ai-analysis-text {
  padding: 12px;
  border: 1px solid rgba(207, 217, 230, 0.78);
  border-radius: 12px;
  background: rgba(247, 250, 255, 0.82);
  color: #2f3b4c;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.ai-evidence-collapse {
  margin-top: 12px;
  border-top: 1px solid #e4e7ed;
  border-bottom: 1px solid #e4e7ed;
}

.ai-result-meta :deep(.el-descriptions__cell) {
  padding: 7px 10px;
  font-size: 13px;
}

.ai-evidence-collapse :deep(.el-collapse-item__header) {
  height: 34px;
  font-size: 13px;
}

.ai-evidence-collapse :deep(.el-collapse-item__content) {
  padding-bottom: 10px;
}

@media (max-width: 768px) {
  .ai-diagnosis-content {
    padding: 14px;
  }

  .ai-diagnosis-target {
    grid-template-columns: 1fr;
  }

  .ai-diagnosis-dialog :deep(.el-col-12) {
    flex: 0 0 100%;
    max-width: 100%;
  }

  .ai-result-header {
    flex-direction: column;
  }

}

/* 测试对话框样式 */
.test-dialog :deep(.el-dialog) {
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
}

.test-dialog :deep(.el-dialog__header) {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: white;
  border-top-left-radius: 16px;
  border-top-right-radius: 16px;
  padding: 20px 24px;
}

.test-dialog :deep(.el-dialog__title) {
  color: white;
  font-weight: 600;
}

.test-content {
  padding: 20px;
  max-height: 70vh;
  overflow-y: auto;
}

.test-form {
  margin-bottom: 20px;
}

.test-result {
  margin-top: 20px;
  padding: 20px;
  background: rgba(249, 250, 251, 0.5);
  border-radius: 12px;
  border: 1px solid #e4e7ed;
}

.result-message {
  margin: 0;
}

.result-message p {
  margin: 8px 0;
}

.suggestion {
  color: #409eff;
  font-size: 14px;
  margin-top: 12px;
}

.result-details {
  margin-top: 16px;
}

.pods-tested {
  margin-top: 16px;
}

.pods-tested h4 {
  margin-bottom: 12px;
  font-size: 14px;
  color: #606266;
}

.response-headers,
.response-body {
  margin-top: 16px;
}

.code-block {
  background: #f8f9fa;
  border: 1px solid #e9ecef;
  border-radius: 8px;
  padding: 12px;
  font-family: 'Courier New', 'Consolas', monospace;
  font-size: 12px;
  line-height: 1.5;
  overflow-x: auto;
  white-space: pre-wrap;
  word-wrap: break-word;
  color: #333;
}

/* New DESIGN.md aligned visual layer */
.k8s-network-management {
  --network-page-height-offset: 190px;
  --network-bg-page: #f4f7fb;
  --network-surface: rgba(247, 250, 255, 0.84);
  --network-surface-strong: rgba(255, 255, 255, 0.94);
  --network-surface-warm: rgba(255, 253, 248, 0.94);
  --network-surface-warm-strong: rgba(255, 248, 239, 0.96);
  --network-text-primary: #172133;
  --network-text-secondary: #5f6b7c;
  --network-text-muted: #8d97a8;
  --network-border-soft: rgba(15, 23, 42, 0.06);
  --network-border-cool: #dfe7f2;
  --network-brand: #4c8bf5;
  --network-brand-active: #4d79d8;
  --network-brand-soft: rgba(76, 139, 245, 0.12);
  --network-success: #3fb97f;
  --network-warning: #f3b24b;
  --network-danger: #ef6b73;
  padding: 6px;
  min-height: calc(100vh - var(--network-page-height-offset));
  background:
    radial-gradient(circle at 16% 14%, rgba(76, 139, 245, 0.07), transparent 28%),
    linear-gradient(180deg, #f7f9fc 0%, var(--network-bg-page) 100%);
  color: var(--network-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.detail-dialog,
.events-dialog,
.create-dialog,
.edit-dialog,
.test-dialog,
.ai-diagnosis-dialog {
  --network-text-primary: #172133;
  --network-text-secondary: #5f6b7c;
  --network-text-muted: #8d97a8;
  --network-brand: #4c8bf5;
  --network-brand-active: #4d79d8;
  --network-brand-soft: rgba(76, 139, 245, 0.12);
}

.network-card {
  position: relative;
  display: flex;
  flex-direction: column;
  max-height: calc(100vh - 12px - var(--network-page-height-offset));
  min-height: 0;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 24px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.96), rgba(247, 250, 255, 0.92));
  box-shadow: 0 20px 60px rgba(17, 24, 39, 0.07);
  backdrop-filter: blur(18px);
}

.network-card::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 116px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  pointer-events: none;
}

.network-card :deep(.el-card__header) {
  position: relative;
  z-index: 1;
  padding: 18px 20px 16px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.78);
  background: transparent;
}

.network-card :deep(.el-card__body) {
  position: relative;
  z-index: 1;
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  padding: 14px 16px 16px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 18px;
}

.title-block {
  min-width: 0;
}

.title-kicker {
  display: block;
  margin-bottom: 6px;
  color: var(--network-brand);
  font-size: 12px;
  font-weight: 700;
}

.title {
  display: block;
  margin: 0;
  color: var(--network-text-primary);
  font-size: 24px;
  font-weight: 700;
  line-height: 1.2;
  background: none;
  -webkit-text-fill-color: currentColor;
}

.title-block p {
  margin: 8px 0 0;
  color: var(--network-text-secondary);
  font-size: 13px;
  line-height: 1.55;
}

.header-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 10px;
}

.search-section {
  margin: 0 0 12px;
  padding: 14px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 20px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.86), rgba(247, 250, 255, 0.82));
}

.resource-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 12px;
}

.summary-item {
  padding: 11px 12px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.72);
}

.summary-item span {
  display: block;
  margin-bottom: 5px;
  color: var(--network-text-muted);
  font-size: 12px;
}

.summary-item strong {
  display: block;
  color: var(--network-text-primary);
  font-size: 16px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.filter-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-top: 2px;
}

.search-form {
  margin: 0;
}

.search-form :deep(.el-form-item) {
  margin: 0 12px 10px 0;
}

.search-form :deep(.el-form-item__label) {
  color: var(--network-text-secondary);
  font-size: 13px;
  font-weight: 600;
}

.search-input {
  width: 240px;
}

.resource-controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 14px;
  margin-top: 2px;
  padding-top: 12px;
  border-top: 1px solid var(--network-border-soft);
}

.network-tabs {
  flex: 1;
  min-width: 0;
  margin: 0;
}

.network-tabs :deep(.el-tabs__header) {
  margin: 0;
}

.network-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: rgba(223, 231, 242, 0.9);
}

.network-tabs :deep(.el-tabs__active-bar) {
  height: 2px;
  border-radius: 999px;
  background: var(--network-brand);
}

.network-tabs :deep(.el-tabs__item) {
  height: 34px;
  color: var(--network-text-secondary);
  font-size: 13px;
  font-weight: 600;
}

.network-tabs :deep(.el-tabs__item.is-active) {
  color: var(--network-brand-active);
}

.create-actions {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  margin-top: 0;
}

.table-content,
.tab-content {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
}

.table-content {
  gap: 10px;
}

.table-shell {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.94);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.88);
}

.resource-table {
  flex: 1;
  min-height: 0;
  border-radius: 18px;
  background: transparent;
}

.resource-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.resource-table :deep(.el-table__header-wrapper th),
.resource-table :deep(.el-table__fixed-header-wrapper th) {
  height: 42px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.9);
  background: rgba(248, 250, 255, 0.96);
  color: var(--network-text-muted);
  font-size: 12px;
  font-weight: 700;
}

.resource-table :deep(.el-table__body-wrapper td),
.resource-table :deep(.el-table__fixed-body-wrapper td) {
  border-bottom: 1px solid rgba(15, 23, 42, 0.04);
  color: var(--network-text-secondary);
  font-size: 13px;
}

.resource-table :deep(.el-table__row) {
  transition: background-color 0.2s ease;
}

.resource-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: rgba(76, 139, 245, 0.045);
}

.resource-table :deep(.el-table__fixed-right) {
  box-shadow: -8px 0 18px rgba(15, 23, 42, 0.04);
}

.pagination-wrapper {
  flex-shrink: 0;
  display: flex;
  justify-content: flex-end;
  padding: 2px 4px 0;
}

.resource-name,
.endpoint-item,
.ports-info,
.hosts-info,
.paths-info,
.selector-info,
.services-info {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  min-width: 0;
}

.endpoints-info {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
}

.resource-icon {
  color: var(--network-brand);
  font-size: 15px;
}

.resource-name-link,
.endpoint-link {
  color: var(--network-brand-active);
  font-weight: 600;
  text-decoration: none;
  cursor: pointer;
  overflow-wrap: anywhere;
  transition: color 0.2s ease;
}

.resource-name-link:hover,
.endpoint-link:hover {
  color: #2f63c8;
  border-bottom-color: transparent;
  transform: none;
}

.copy-button {
  min-width: 22px;
  height: 22px;
  padding: 0;
  margin-left: 0;
  border-radius: 8px;
  color: var(--network-text-muted);
  opacity: 0;
  transition: opacity 0.2s ease, color 0.2s ease, background-color 0.2s ease;
}

.resource-name:hover .copy-button,
.endpoint-item:hover .copy-button {
  opacity: 1;
}

.copy-button:hover {
  color: var(--network-brand);
  background: var(--network-brand-soft);
  transform: none;
}

.port-tag,
.host-tag,
.path-tag,
.selector-tag,
.service-tag {
  max-width: 100%;
  margin: 0;
  border-radius: 999px;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 11px;
  font-weight: 600;
}

.nodeport-info {
  margin-left: 2px;
  color: #b9791e;
  font-weight: 700;
}

.operation-buttons {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  flex-wrap: nowrap;
  gap: 12px;
  white-space: nowrap;
}

.operation-buttons .el-button,
.operation-buttons .table-action,
.operation-buttons .el-button:hover:not(.is-disabled),
.operation-buttons .table-action:hover:not(.is-disabled):not(:disabled),
.operation-buttons .el-button.is-disabled:hover {
  padding: 0;
  border: none;
  box-shadow: none;
  transform: none;
}

.table-action {
  display: inline-flex;
  align-items: center;
  margin: 0;
  border: none;
  background: transparent;
  height: auto;
  min-width: auto;
  font-size: 13px;
  font-weight: 600;
  line-height: 1.4;
  cursor: pointer;
  appearance: none;
}

.action-primary {
  color: var(--network-brand-active);
}

.action-ai {
  color: #3f9f77;
}

.action-more {
  color: #ad7926;
}

.table-action:hover {
  opacity: 0.82;
}

.k8s-network-management :deep(.el-input__wrapper),
.k8s-network-management :deep(.el-select .el-input__wrapper),
.compact-dialog :deep(.el-input__wrapper),
.compact-dialog :deep(.el-select .el-input__wrapper),
.test-dialog :deep(.el-input__wrapper),
.test-dialog :deep(.el-select .el-input__wrapper),
.ai-diagnosis-dialog :deep(.el-input__wrapper),
.ai-diagnosis-dialog :deep(.el-select .el-input__wrapper) {
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 14px;
  background: rgba(252, 253, 255, 0.98);
  box-shadow: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;
}

.k8s-network-management :deep(.el-input__wrapper:hover),
.k8s-network-management :deep(.el-select .el-input__wrapper:hover),
.compact-dialog :deep(.el-input__wrapper:hover),
.compact-dialog :deep(.el-select .el-input__wrapper:hover),
.test-dialog :deep(.el-input__wrapper:hover),
.test-dialog :deep(.el-select .el-input__wrapper:hover),
.ai-diagnosis-dialog :deep(.el-input__wrapper:hover),
.ai-diagnosis-dialog :deep(.el-select .el-input__wrapper:hover) {
  border-color: rgba(171, 189, 220, 0.95);
}

.k8s-network-management :deep(.el-input__wrapper.is-focus),
.k8s-network-management :deep(.el-select .el-input__wrapper.is-focus),
.compact-dialog :deep(.el-input__wrapper.is-focus),
.compact-dialog :deep(.el-select .el-input__wrapper.is-focus),
.test-dialog :deep(.el-input__wrapper.is-focus),
.test-dialog :deep(.el-select .el-input__wrapper.is-focus),
.ai-diagnosis-dialog :deep(.el-input__wrapper.is-focus),
.ai-diagnosis-dialog :deep(.el-select .el-input__wrapper.is-focus) {
  border-color: rgba(120, 156, 221, 0.9);
  background: #fff;
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.08);
}

.k8s-network-management :deep(.el-textarea__inner),
.compact-dialog :deep(.el-textarea__inner),
.test-dialog :deep(.el-textarea__inner),
.ai-diagnosis-dialog :deep(.el-textarea__inner) {
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 14px;
  background: rgba(252, 253, 255, 0.98);
  box-shadow: none;
}

.k8s-network-management :deep(.el-textarea__inner:focus),
.compact-dialog :deep(.el-textarea__inner:focus),
.test-dialog :deep(.el-textarea__inner:focus),
.ai-diagnosis-dialog :deep(.el-textarea__inner:focus) {
  border-color: rgba(120, 156, 221, 0.9);
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.08);
}

.k8s-network-management :deep(.el-button),
.detail-dialog :deep(.el-button),
.events-dialog :deep(.el-button),
.compact-dialog :deep(.el-button),
.test-dialog :deep(.el-button),
.ai-diagnosis-dialog :deep(.el-button) {
  border-radius: 14px;
  font-weight: 600;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, color 0.2s ease, background-color 0.2s ease, opacity 0.2s ease;
}

.k8s-network-management :deep(.el-button:hover),
.detail-dialog :deep(.el-button:hover),
.events-dialog :deep(.el-button:hover),
.compact-dialog :deep(.el-button:hover),
.test-dialog :deep(.el-button:hover),
.ai-diagnosis-dialog :deep(.el-button:hover) {
  transform: none;
}

.k8s-network-management :deep(.el-button--primary),
.test-dialog :deep(.el-button--primary),
.ai-diagnosis-dialog :deep(.el-button--primary),
.compact-dialog :deep(.el-button--primary) {
  border-color: transparent;
  background: linear-gradient(180deg, #69a0fa 0%, #5a93f6 100%);
  color: #fff;
  box-shadow: 0 10px 18px rgba(76, 139, 245, 0.16);
}

.k8s-network-management :deep(.el-button--success),
.compact-dialog :deep(.el-button--success) {
  border-color: transparent;
  background: linear-gradient(180deg, #55c993 0%, #3fb97f 100%);
  color: #fff;
  box-shadow: 0 10px 18px rgba(63, 185, 127, 0.14);
}

.k8s-network-management :deep(.el-button--warning),
.compact-dialog :deep(.el-button--warning) {
  border-color: transparent;
  background: linear-gradient(180deg, #f5bf61 0%, #f3b24b 100%);
  color: #fff;
  box-shadow: 0 10px 18px rgba(243, 178, 75, 0.14);
}

.k8s-network-management :deep(.el-button--danger),
.compact-dialog :deep(.el-button--danger) {
  border-color: transparent;
  background: linear-gradient(180deg, #f47b82 0%, #ef6b73 100%);
  color: #fff;
}

.k8s-network-management :deep(.el-button.is-disabled),
.detail-dialog :deep(.el-button.is-disabled),
.events-dialog :deep(.el-button.is-disabled),
.compact-dialog :deep(.el-button.is-disabled),
.test-dialog :deep(.el-button.is-disabled),
.ai-diagnosis-dialog :deep(.el-button.is-disabled) {
  opacity: 0.55;
  box-shadow: none;
}

.k8s-network-management :deep(.el-tag),
.detail-dialog :deep(.el-tag),
.events-dialog :deep(.el-tag),
.compact-dialog :deep(.el-tag),
.test-dialog :deep(.el-tag),
.ai-diagnosis-dialog :deep(.el-tag) {
  border-radius: 999px;
  border: 1px solid transparent;
  font-weight: 600;
}

.k8s-network-management :deep(.el-tag--primary),
.detail-dialog :deep(.el-tag--primary),
.test-dialog :deep(.el-tag--primary),
.ai-diagnosis-dialog :deep(.el-tag--primary) {
  border-color: rgba(76, 139, 245, 0.12);
  background: rgba(76, 139, 245, 0.1);
  color: var(--network-brand-active);
}

.k8s-network-management :deep(.el-tag--success),
.detail-dialog :deep(.el-tag--success),
.events-dialog :deep(.el-tag--success),
.test-dialog :deep(.el-tag--success),
.ai-diagnosis-dialog :deep(.el-tag--success) {
  border-color: rgba(63, 185, 127, 0.14);
  background: rgba(63, 185, 127, 0.12);
  color: #31946a;
}

.k8s-network-management :deep(.el-tag--warning),
.detail-dialog :deep(.el-tag--warning),
.events-dialog :deep(.el-tag--warning),
.test-dialog :deep(.el-tag--warning),
.ai-diagnosis-dialog :deep(.el-tag--warning) {
  border-color: rgba(243, 178, 75, 0.16);
  background: rgba(243, 178, 75, 0.13);
  color: #aa741d;
}

.k8s-network-management :deep(.el-tag--danger),
.detail-dialog :deep(.el-tag--danger),
.events-dialog :deep(.el-tag--danger),
.test-dialog :deep(.el-tag--danger),
.ai-diagnosis-dialog :deep(.el-tag--danger) {
  border-color: rgba(239, 107, 115, 0.16);
  background: rgba(239, 107, 115, 0.12);
  color: #c64f59;
}

.k8s-network-management :deep(.el-tag--info),
.detail-dialog :deep(.el-tag--info),
.events-dialog :deep(.el-tag--info),
.test-dialog :deep(.el-tag--info),
.ai-diagnosis-dialog :deep(.el-tag--info) {
  border-color: rgba(141, 151, 168, 0.14);
  background: rgba(237, 242, 251, 0.96);
  color: var(--network-text-secondary);
}

.detail-dialog :deep(.el-dialog),
.events-dialog :deep(.el-dialog),
.create-dialog :deep(.el-dialog),
.edit-dialog :deep(.el-dialog),
.test-dialog :deep(.el-dialog),
.ai-diagnosis-dialog :deep(.el-dialog) {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 26px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98) 0%, rgba(247, 250, 255, 0.96) 100%);
  box-shadow: 0 36px 110px rgba(15, 23, 42, 0.11);
  backdrop-filter: blur(18px);
}

.detail-dialog :deep(.el-dialog__header),
.events-dialog :deep(.el-dialog__header),
.create-dialog :deep(.el-dialog__header),
.edit-dialog :deep(.el-dialog__header),
.test-dialog :deep(.el-dialog__header),
.ai-diagnosis-dialog :deep(.el-dialog__header) {
  margin: 0;
  padding: 18px 22px 15px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.94);
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
}

.detail-dialog :deep(.el-dialog__title),
.events-dialog :deep(.el-dialog__title),
.create-dialog :deep(.el-dialog__title),
.edit-dialog :deep(.el-dialog__title),
.test-dialog :deep(.el-dialog__title),
.ai-diagnosis-dialog :deep(.el-dialog__title) {
  color: var(--network-text-primary);
  font-size: 17px;
  font-weight: 700;
}

.detail-dialog :deep(.el-dialog__body),
.events-dialog :deep(.el-dialog__body),
.create-dialog :deep(.el-dialog__body),
.edit-dialog :deep(.el-dialog__body),
.test-dialog :deep(.el-dialog__body),
.ai-diagnosis-dialog :deep(.el-dialog__body) {
  padding: 0;
}

.detail-dialog :deep(.el-dialog__footer),
.events-dialog :deep(.el-dialog__footer),
.create-dialog :deep(.el-dialog__footer),
.edit-dialog :deep(.el-dialog__footer),
.test-dialog :deep(.el-dialog__footer),
.ai-diagnosis-dialog :deep(.el-dialog__footer) {
  padding: 12px 22px 16px;
  border-top: 1px solid rgba(223, 231, 242, 0.9);
  background: rgba(255, 253, 248, 0.78);
}

.detail-content,
.events-content,
.test-content,
.ai-diagnosis-content {
  max-height: 70vh;
  overflow-y: auto;
  padding: 18px 22px 20px;
}

.compact-dialog :deep(.compact-form) {
  padding: 18px 22px 20px;
}

.compact-form :deep(.el-form-item) {
  margin-bottom: 14px;
}

.compact-form :deep(.el-form-item__label),
.test-form :deep(.el-form-item__label),
.ai-diagnosis-form :deep(.el-form-item__label) {
  color: var(--network-text-secondary);
  font-size: 13px;
  font-weight: 600;
}

.form-row {
  display: flex;
  gap: 14px;
  margin-bottom: 0;
}

.form-item-half {
  flex: 1;
  margin-bottom: 0 !important;
}

.ports-config,
.rules-config {
  width: 100%;
  padding: 12px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 18px;
  background: rgba(255, 253, 248, 0.74);
}

.port-item,
.rule-item,
.path-item {
  margin-bottom: 10px;
  padding: 10px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.76);
}

.rule-item {
  padding: 12px;
}

.port-fields,
.path-fields,
.rule-header {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.rule-header {
  margin-bottom: 10px;
  padding-bottom: 10px;
  border-bottom: 1px solid rgba(15, 23, 42, 0.05);
}

.paths-config {
  padding-left: 0;
}

.option-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.option-desc {
  margin-left: 8px;
  color: var(--network-text-muted);
  font-size: 11px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.detail-dialog :deep(.el-descriptions__table),
.test-dialog :deep(.el-descriptions__table),
.ai-diagnosis-dialog :deep(.el-descriptions__table) {
  border-radius: 16px;
  overflow: hidden;
}

.detail-dialog :deep(.el-descriptions__label),
.test-dialog :deep(.el-descriptions__label),
.ai-diagnosis-dialog :deep(.el-descriptions__label) {
  color: var(--network-text-secondary);
  font-weight: 600;
  background: rgba(248, 250, 255, 0.92);
}

.detail-dialog :deep(.el-descriptions__content),
.test-dialog :deep(.el-descriptions__content),
.ai-diagnosis-dialog :deep(.el-descriptions__content) {
  color: var(--network-text-primary);
  background: rgba(255, 255, 255, 0.78);
}

.events-dialog :deep(.el-table),
.test-dialog :deep(.el-table) {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.94);
  border-radius: 16px;
}

.test-form {
  margin: 0;
  padding: 14px 16px 2px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 18px;
  background: rgba(255, 253, 248, 0.74);
}

.test-result,
.ai-diagnosis-form,
.ai-diagnosis-flow,
.ai-diagnosis-result {
  margin-top: 14px;
  padding: 14px 16px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.76);
}

.ai-diagnosis-form {
  margin-top: 0;
}

.ai-diagnosis-target {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 12px;
}

.ai-diagnosis-target > div {
  padding: 11px 12px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.74);
}

.ai-diagnosis-target span {
  display: block;
  margin-bottom: 5px;
  color: var(--network-text-muted);
  font-size: 12px;
}

.ai-diagnosis-target strong {
  display: block;
  color: var(--network-text-primary);
  font-size: 14px;
  overflow-wrap: anywhere;
}

.ai-flow-head,
.ai-result-header,
.ai-section-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.ai-flow-head h4,
.ai-result-section h4,
.ai-section-header h4,
.pods-tested h4 {
  margin: 0 0 8px;
  color: var(--network-text-primary);
  font-size: 14px;
  font-weight: 700;
}

.ai-flow-head p,
.ai-result-header p,
.result-message,
.suggestion {
  margin: 4px 0 0;
  color: var(--network-text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.ai-result-header h3 {
  margin: 0;
  color: var(--network-text-primary);
  font-size: 16px;
  line-height: 1.4;
}

.ai-result-tags {
  display: flex;
  flex-shrink: 0;
  gap: 8px;
}

.ai-result-meta,
.result-details {
  margin-top: 12px;
  margin-bottom: 0;
}

.ai-result-section,
.response-headers,
.response-body,
.pods-tested {
  margin-top: 14px;
}

.ai-remediation-list {
  margin: 0;
  padding-left: 18px;
  color: #3d4858;
  font-size: 13px;
  line-height: 1.65;
}

.ai-analysis-text,
.code-block {
  padding: 12px;
  border: 1px solid rgba(207, 217, 230, 0.82);
  border-radius: 14px;
  background: rgba(247, 250, 255, 0.82);
  color: #2f3b4c;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 12px;
  line-height: 1.65;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.ai-evidence-collapse {
  margin-top: 14px;
  border-top: 1px solid rgba(223, 231, 242, 0.9);
  border-bottom: 1px solid rgba(223, 231, 242, 0.9);
}

.ai-evidence-collapse :deep(.el-collapse-item__header) {
  height: 36px;
  color: var(--network-text-secondary);
  font-size: 13px;
  font-weight: 600;
}

.ai-evidence-collapse :deep(.el-collapse-item__content) {
  padding-bottom: 10px;
}

.result-message p {
  margin: 6px 0;
}

.suggestion {
  color: var(--network-brand-active);
}

@media (max-width: 980px) {
  .card-header,
  .resource-controls,
  .filter-toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .header-actions {
    justify-content: flex-start;
  }

  .resource-summary {
    grid-template-columns: 1fr;
  }

  .create-actions {
    width: 100%;
  }

  .create-actions :deep(.el-button) {
    width: 100%;
  }
}

@media (max-width: 768px) {
  .k8s-network-management {
    --network-page-height-offset: 131px;
    padding: 6px;
  }

  .network-card {
    max-height: calc(100vh - 12px - var(--network-page-height-offset));
    border-radius: 20px;
  }

  .network-card::before {
    height: 150px;
  }

  .network-card :deep(.el-card__header),
  .network-card :deep(.el-card__body) {
    padding-left: 12px;
    padding-right: 12px;
  }

  .title {
    font-size: 21px;
  }

  .search-input,
  .search-form :deep(.el-input) {
    width: 100%;
  }

  .search-form :deep(.el-form-item) {
    display: block;
    margin-right: 0;
  }

  .form-row,
  .port-fields,
  .path-fields,
  .rule-header,
  .dialog-footer,
  .ai-flow-head,
  .ai-result-header,
  .ai-section-header {
    flex-direction: column;
    align-items: stretch;
  }

  .ai-diagnosis-target {
    grid-template-columns: 1fr;
  }

  .compact-dialog :deep(.el-col-12),
  .test-dialog :deep(.el-col-12),
  .ai-diagnosis-dialog :deep(.el-col-12) {
    flex: 0 0 100%;
    max-width: 100%;
  }

  .detail-content,
  .events-content,
  .test-content,
  .ai-diagnosis-content,
  .compact-dialog :deep(.compact-form) {
    padding: 14px;
  }

  .operation-buttons {
    gap: 9px;
  }
}

/* Final page-shell alignment with K8s config/storage pages. */
.k8s-network-management {
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
  --network-page-height-offset: 190px;
  min-height: calc(100vh - var(--network-page-height-offset));
  padding: 6px;
  box-sizing: border-box;
  background:
    radial-gradient(circle at 16% 10%, rgba(76, 139, 245, 0.06), transparent 26%),
    linear-gradient(180deg, #f7f9fc 0%, var(--aiops-bg-page) 100%);
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.network-card {
  position: relative;
  display: flex;
  flex-direction: column;
  max-height: calc(100vh - 12px - var(--network-page-height-offset));
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

.network-card::before {
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

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
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

.search-section {
  flex-shrink: 0;
  margin: 0 0 12px;
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
  margin: 0;
}

.search-form :deep(.el-form-item) {
  margin: 0 14px 0 0;
}

.search-form :deep(.el-form-item__label) {
  color: var(--aiops-text-secondary);
  font-size: 14px;
  font-weight: 600;
}

.search-name-input :deep(.el-input__wrapper) {
  min-height: 32px;
  padding: 0 11px;
  background: rgba(252, 253, 255, 0.98);
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 13px;
  box-shadow: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;
}

.search-name-input :deep(.el-input__wrapper:hover) {
  border-color: #c0c4cc;
}

.search-name-input :deep(.el-input__wrapper.is-focus) {
  border-color: rgba(120, 156, 221, 0.9);
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
  background: #ffffff;
}

.search-name-icon {
  color: var(--aiops-brand);
  font-size: 15px;
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

.secondary-action {
  min-width: 76px;
  border: 1px solid rgba(214, 223, 235, 0.96);
  border-radius: 13px;
  background: rgba(255, 255, 255, 0.92);
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

.secondary-action:hover,
.secondary-action:focus {
  border-color: rgba(153, 181, 230, 0.95);
  color: var(--aiops-brand);
  background: #ffffff;
}

.resource-controls {
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px solid rgba(223, 231, 242, 0.72);
}

.network-tabs :deep(.el-tabs__header) {
  margin: 0;
}

.network-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: rgba(223, 231, 242, 0.78);
}

.network-tabs :deep(.el-tabs__item) {
  height: 34px;
  padding: 0 14px;
  color: var(--aiops-text-secondary);
  font-size: 14px;
  font-weight: 600;
}

.network-tabs :deep(.el-tabs__item.is-active) {
  color: var(--aiops-brand);
}

.network-tabs :deep(.el-tabs__active-bar) {
  height: 2px;
  border-radius: 999px;
  background: var(--aiops-brand);
}

.table-section {
  flex: 1 1 auto;
  min-height: 0;
  padding: 8px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.64);
  overflow: auto;
}

.content-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
  padding: 4px 4px 10px;
}

.resource-count {
  color: var(--aiops-text-secondary);
  font-size: 13px;
  font-weight: 600;
}

.table-content {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  gap: 10px;
  overflow: visible;
}

.tab-content {
  flex: none;
  min-width: 0;
}

.table-shell {
  display: block;
  min-height: 0;
  overflow: hidden;
  border-radius: 16px;
  background: transparent;
}

.resource-table {
  border: 1px solid rgba(223, 231, 242, 0.88);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.92);
  overflow: hidden;
  box-shadow: none;
}

.resource-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.resource-table :deep(.el-table__header th),
.resource-table :deep(.el-table__fixed-right-patch) {
  background: rgba(248, 250, 255, 0.98) !important;
  border-bottom: 1px solid rgba(223, 231, 242, 0.95);
}

.resource-table :deep(.el-table__header th .cell) {
  color: var(--aiops-text-muted) !important;
  font-size: 12px;
  font-weight: 700 !important;
}

.resource-table :deep(.el-table__cell) {
  border-bottom-color: rgba(15, 23, 42, 0.045);
  color: var(--aiops-text-secondary);
  font-size: 13px;
}

.resource-table :deep(.el-table__body tr:hover > td.el-table__cell) {
  background: rgba(76, 139, 245, 0.045) !important;
}

.resource-icon {
  color: var(--aiops-brand);
  font-size: 16px;
}

.resource-name-link,
.endpoint-link {
  color: var(--aiops-brand);
  cursor: pointer;
  font-size: 13px;
  font-weight: 700;
  transition: opacity 0.2s ease;
  text-decoration: none;
}

.resource-name-link:hover,
.endpoint-link:hover {
  opacity: 0.82;
}

.operation-buttons {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: center;
  flex-wrap: nowrap;
}

.operation-buttons .table-action {
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

.operation-buttons .table-action:hover {
  transform: none;
  box-shadow: none;
  opacity: 0.82;
}

.action-primary {
  color: var(--aiops-brand);
}

.action-ai {
  color: #3f9f77;
}

.action-more {
  color: #f3a33a;
}

.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  padding: 2px 4px 0;
}

.k8s-network-management .el-button:hover {
  transform: translateY(-1px);
}

@media (max-width: 768px) {
  .k8s-network-management {
    --network-page-height-offset: 131px;
    padding: 4px;
  }

  .network-card {
    max-height: calc(100vh - 8px - var(--network-page-height-offset));
    min-height: 0;
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
  .search-name-input,
  .query-button,
  .secondary-action,
  .primary-action {
    width: 100% !important;
  }

  .content-header {
    align-items: stretch;
    flex-direction: column;
  }

  .operation-buttons {
    flex-direction: column;
    gap: 4px;
  }
}

/* DESIGN.md compact workbench overrides */
.k8s-network-management {
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

.network-card {
  height: auto;
  max-height: none;
  padding: 10px;
}

.network-card::before {
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
  margin: 0 0 8px;
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

.search-name-input :deep(.el-input__wrapper) {
  min-height: 34px;
  padding: 0 11px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.search-name-input :deep(.el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.search-name-input :deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 1px #409eff inset;
}

.query-button,
.refresh-button,
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

.primary-action {
  min-width: 112px;
}

.k8s-network-management .el-button:hover,
.query-button:hover,
.query-button:focus,
.refresh-button:hover,
.refresh-button:focus,
.primary-action:hover,
.primary-action:focus {
  transform: none;
}

.query-button:hover,
.query-button:focus,
.refresh-button:hover,
.refresh-button:focus {
  color: #fff;
}

.resource-controls {
  margin-top: 6px;
  padding-top: 6px;
}

.network-tabs :deep(.el-tabs__item) {
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

.content-header {
  min-height: 42px;
  margin-bottom: 0;
  padding: 5px 10px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: #fbfdff;
}

.resource-count {
  color: #64748b;
  font-size: 12px;
}

.table-content {
  gap: 0;
}

.table-shell {
  border-radius: 9px;
}

.resource-table {
  min-width: 1080px;
  border: 0;
  border-radius: 9px;
  background: #fff;
}

.resource-table :deep(.el-table__header th),
.resource-table :deep(.el-table__fixed-right-patch) {
  height: 38px;
  padding: 6px 0;
  background: #f3f7fc !important;
  border-bottom: 1px solid #e3ebf5;
}

.resource-table :deep(.el-table__header th .cell) {
  color: #475569 !important;
  font-size: 12px;
}

.resource-table :deep(.el-table__cell) {
  padding: 7px 0;
  border-bottom-color: #eef3f9;
  color: #53657d;
  font-size: 12px;
}

.resource-table :deep(.el-table__body tr:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

.resource-name-link,
.endpoint-link {
  color: #2563eb;
  font-size: 12px;
  transition: color 160ms ease, opacity 160ms ease;
}

.resource-name-link:hover,
.endpoint-link:hover,
.copy-button:hover {
  transform: none;
}

.copy-button {
  width: 26px;
  height: 26px;
  min-height: 26px;
  padding: 0;
  border-radius: 7px;
}

.ports-info,
.selector-info,
.services-info,
.endpoints-info {
  gap: 4px;
}

.resource-table :deep(.el-tag) {
  font-size: 11px;
}

.operation-buttons {
  gap: 14px;
  justify-content: flex-end;
}

.operation-buttons .table-action {
  transform: none;
  transition: color 160ms ease, opacity 160ms ease;
}

.pagination-wrapper {
  min-height: 36px;
  padding: 8px 4px;
  border-top: 1px solid var(--ops-border-soft);
}

.pagination-wrapper :deep(.el-pagination) {
  --el-pagination-button-height: 28px;
  --el-pagination-button-width: 28px;
  font-size: 12px;
}

.compact-form :deep(.el-form-item),
.test-form :deep(.el-form-item),
.ai-diagnosis-form :deep(.el-form-item) {
  margin-bottom: 14px;
}

.ports-container,
.rules-container,
.rule-item,
.test-result,
.ai-diagnosis-target,
.ai-diagnosis-flow,
.ai-diagnosis-result {
  border-radius: 10px;
}

:global(.k8s-network-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 24px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: 15px !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.k8s-network-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: linear-gradient(90deg, #fff6e7 0%, #f7fbff 36%, #ecf7ff 100%);
}

:global(.k8s-network-dialog-overlay .el-dialog__title) {
  color: #172033;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.k8s-network-dialog-overlay .el-dialog__headerbtn) {
  top: 12px;
  right: 14px;
  width: 32px;
  height: 32px;
}

:global(.k8s-network-dialog-overlay .el-dialog__body) {
  max-height: calc(100vh - 190px);
  overflow: auto;
  padding: 14px 18px;
  background: #fff;
}

:global(.k8s-network-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.k8s-network-dialog-overlay .el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

:global(.k8s-network-dialog-overlay .el-input__wrapper),
:global(.k8s-network-dialog-overlay .el-select__wrapper),
:global(.k8s-network-dialog-overlay .el-input-number),
:global(.k8s-network-dialog-overlay .el-textarea__inner) {
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.k8s-network-dialog-overlay .el-input__wrapper),
:global(.k8s-network-dialog-overlay .el-select__wrapper),
:global(.k8s-network-dialog-overlay .el-input-number) {
  min-height: 34px;
}

:global(.k8s-network-dialog-overlay .el-input__wrapper:hover),
:global(.k8s-network-dialog-overlay .el-select__wrapper:hover),
:global(.k8s-network-dialog-overlay .el-textarea__inner:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

:global(.k8s-network-dialog-overlay .el-button) {
  min-height: 34px;
  border-radius: 8px;
  font-weight: 600;
  transform: none !important;
}

:global(.k8s-network-dialog-overlay .el-table) {
  overflow: hidden;
  border: 1px solid #e3ebf5;
  border-radius: 9px;
}

:global(.k8s-network-dialog-overlay .el-table th.el-table__cell) {
  background: #f3f7fc !important;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

:global(.k8s-network-dialog-overlay .el-descriptions) {
  overflow: hidden;
  border-radius: 9px;
}

:global(.k8s-network-ai-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 20px;
  max-width: calc(100vw - 24px);
  overflow: hidden;
  border: 1px solid #cfe2f8;
  border-radius: 20px !important;
  background: #fff;
  box-shadow: 0 28px 80px rgba(39, 70, 110, 0.24);
}

:global(.k8s-network-ai-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-top: 3px solid transparent;
  border-bottom: 1px solid #e3edf8;
  background:
    linear-gradient(#fff, #fff) padding-box,
    linear-gradient(90deg, #f5a524 0%, #62a8ff 48%, #55d6be 100%) border-box;
}

:global(.k8s-network-ai-dialog-overlay .el-dialog__title) {
  color: #172033;
  font-size: 18px;
  font-weight: 700;
}

:global(.k8s-network-ai-dialog-overlay .el-dialog__body) {
  max-height: calc(100vh - 190px);
  overflow: auto;
  padding: 16px 18px;
  background: #f8fbff;
}

:global(.k8s-network-ai-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #fff;
}

:global(.k8s-network-ai-dialog-overlay .el-input__wrapper),
:global(.k8s-network-ai-dialog-overlay .el-select__wrapper) {
  min-height: 34px;
  border-radius: 8px;
  background: #fff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.k8s-network-ai-dialog-overlay .el-button) {
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
  .k8s-network-management {
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
  .network-card {
    padding: 8px;
    border-radius: 12px;
  }

  .search-form :deep(.el-form-item) {
    margin-bottom: 8px;
  }
}

</style>
