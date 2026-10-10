<script setup>
import { ref, reactive, onBeforeUnmount, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Search,
  Refresh,
  Monitor
} from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'
import NodesTable from './nodes/NodesTable.vue'
import { openYamlDialog } from './services/yamlDialog'
import {
  getStoredClusterId,
  saveStoredClusterId,
  resolvePreferredClusterId,
  subscribeStoredClusterId
} from '@/utils/k8sClusterSelection'

const router = useRouter()
const CLUSTER_SELECTION_SOURCE = 'k8s-nodes'

const loading = ref(false)
const queryParams = reactive({
  nodeName: '',
  status: '',
  role: ''
})

const statusOptions = [
  { label: '全部', value: '' },
  { label: '就绪', value: 'Ready' },
  { label: '未就绪', value: 'NotReady' },
  { label: '不可调度', value: 'SchedulingDisabled' },
  { label: '未知', value: 'Unknown' }
]

const roleOptions = [
  { label: '全部', value: '' },
  { label: 'Master', value: 'master' },
  { label: 'Worker', value: 'worker' },
  { label: 'Control-plane', value: 'control-plane' }
]

const tableData = ref([])
const nodeNameOptions = ref([])
const nodeNameOptionsLoading = ref(false)
const nodeNameFilterKeyword = ref('')
const selectedClusterId = ref(getStoredClusterId())
const clusterList = ref([]) // 集群列表
let unsubscribeClusterSelection = null

// 对话框状态
const taintDialogVisible = ref(false)
const labelDialogVisible = ref(false)
const resourcesDialogVisible = ref(false)
const drainDialogVisible = ref(false)
// 当前操作的节点
const currentNode = ref({})

// 污点管理表单
const taintForm = reactive({
  operation: 'add', // add | remove
  key: '',
  value: '',
  effect: 'NoSchedule', // NoSchedule | PreferNoSchedule | NoExecute
  selectedTaint: null // 选中的要移除的污点
})

// 标签管理表单
const labelForm = reactive({
  operation: 'add', // add | remove
  key: '',
  value: '',
  selectedLabel: null // 选中的要移除的标签
})

// 驱逐配置表单
const drainForm = reactive({
  force: false,
  deleteLocalData: false,
  ignoreDaemonSets: true,
  gracePeriodSeconds: 30
})

// 节点资源详情
const nodeResources = ref({})

const effectOptions = [
  { 
    label: 'NoSchedule - 禁止调度', 
    value: 'NoSchedule',
    description: '不允许新Pod调度到此节点，但不影响已运行的Pod'
  },
  { 
    label: 'PreferNoSchedule - 尽量避免调度', 
    value: 'PreferNoSchedule',
    description: '尽量避免新Pod调度到此节点，但在资源不足时仍可调度'
  },
  { 
    label: 'NoExecute - 禁止执行', 
    value: 'NoExecute',
    description: '不允许新Pod调度，且会驱逐不能容忍此污点的已运行Pod'
  }
]

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

const fetchNodeNameOptions = async () => {
  if (!selectedClusterId.value) {
    nodeNameOptions.value = []
    return
  }

  try {
    nodeNameOptionsLoading.value = true
    const response = await k8sApi.getClusterNodes(selectedClusterId.value)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.code === 0 || responseData.success) {
      const nodes = Array.isArray(responseData.data) ? responseData.data : []
      nodeNameOptions.value = nodes.map(node => {
        const name = node.name || ''
        const internalIP = node.internalIP || ''
        const externalIP = node.externalIP || ''
        const ipText = [internalIP, externalIP].filter(Boolean).join(' / ')

        return {
          label: ipText ? `${name} (${ipText})` : name,
          value: name
        }
      })

      if (queryParams.nodeName && !nodeNameOptions.value.some(option => option.value === queryParams.nodeName)) {
        queryParams.nodeName = ''
      }
    } else {
      nodeNameOptions.value = []
    }
  } catch (error) {
    console.error('获取节点下拉选项失败:', error)
    nodeNameOptions.value = []
  } finally {
    nodeNameOptionsLoading.value = false
  }
}

const filteredNodeNameOptions = computed(() => {
  const keyword = nodeNameFilterKeyword.value.trim().toLowerCase()
  if (!keyword) return nodeNameOptions.value

  return nodeNameOptions.value.filter(option =>
    option.label.toLowerCase().includes(keyword) ||
    option.value.toLowerCase().includes(keyword)
  )
})

const handleNodeNameFilter = (keyword) => {
  nodeNameFilterKeyword.value = keyword
}

const handleNodeNameVisibleChange = (visible) => {
  if (visible && selectedClusterId.value && !nodeNameOptions.value.length) {
    fetchNodeNameOptions()
  }
}

const handleQuery = async () => {
  try {
    // 检查是否选择了集群
    if (!selectedClusterId.value) {
      ElMessage.warning('请选择一个集群')
      return
    }

    loading.value = true

    let response

    // 根据不同的查询条件调用不同的API
    if (queryParams.nodeName) {
      // 根据节点名称搜索
      response = await k8sApi.searchNodesByName(selectedClusterId.value, queryParams.nodeName)
    } else if (queryParams.role) {
      // 根据节点角色搜索
      response = await k8sApi.searchNodesByRole(selectedClusterId.value, queryParams.role)
    } else if (queryParams.status) {
      // 根据节点状态搜索
      response = await k8sApi.searchNodesByStatus(selectedClusterId.value, queryParams.status)
    } else {
      // 都没有条件，获取全部节点
      response = await k8sApi.getClusterNodes(selectedClusterId.value)
    }

    const responseData = response.data || response
    console.log('节点列表API响应:', responseData)

    if (responseData.code === 200 || responseData.code === 0 || responseData.success) {
      const nodes = responseData.data || []
      tableData.value = nodes.map(node => ({
        id: node.name,
        nodeName: node.name,
        status: node.status || 'Unknown',
        role: node.roles === 'control-plane,master' ? 'master' : (node.roles === 'control-plane' ? 'control-plane' : (node.roles === 'master' ? 'master' : 'worker')),
        version: node.runtime?.kubeletVersion || 'Unknown',
        age: formatAge(node.createdAt),
        cpu: `${node.resources?.cpu?.allocatable || 'Unknown'}`,
        memory: formatMemory(node.resources?.memory?.allocatable || '0'),
        resourceUsage: buildNodeResourceUsage(node.resources),
        pods: `${node.podMetrics?.allocated || 0}/${node.podMetrics?.total || 0}`,
        schedulable: !node.scheduling?.unschedulable,
        taints: node.scheduling?.taints || [],
        labels: node.configuration?.labels || {},
        conditions: node.conditions || [],
        nodeInfo: node.runtime || {},
        allocatedResources: node.resources || {},
        internalIP: node.internalIP,
        externalIP: node.externalIP,
        configuration: node.configuration,
        runtime: node.runtime
      }))

      console.log('节点列表加载成功:', tableData.value)
    } else {
      ElMessage.error(responseData.message || '获取节点列表失败')
      tableData.value = []
    }
  } catch (error) {
    console.error('获取节点列表失败:', error)
    
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
    
    tableData.value = []
  } finally {
    loading.value = false
  }
}

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

const formatMemory = (memoryStr) => {
  if (!memoryStr || memoryStr === '0') return 'Unknown'
  
  // 处理Ki单位
  if (memoryStr.endsWith('Ki')) {
    const kb = parseInt(memoryStr.replace('Ki', ''))
    const gb = (kb / 1024 / 1024).toFixed(1)
    return `${gb}GB`
  }
  
  // 处理Mi单位
  if (memoryStr.endsWith('Mi')) {
    const mb = parseInt(memoryStr.replace('Mi', ''))
    const gb = (mb / 1024).toFixed(1)
    return `${gb}GB`
  }
  
  // 处理Gi单位
  if (memoryStr.endsWith('Gi')) {
    const gb = parseInt(memoryStr.replace('Gi', ''))
    return `${gb}GB`
  }
  
  return memoryStr
}

const toNumber = (value) => {
  const number = Number(value)
  return Number.isFinite(number) ? number : 0
}

const clampPercent = (value) => {
  const number = toNumber(value)
  if (number < 0) return 0
  if (number > 100) return 100
  return number
}

const percentFromRaw = (used, total) => {
  const usedNumber = toNumber(used)
  const totalNumber = toNumber(total)
  if (totalNumber <= 0) return 0
  return clampPercent((usedNumber / totalNumber) * 100)
}

const buildResourceUsage = (resource = {}, rawUnit = 'Milli') => {
  const allocatableRaw = rawUnit === 'Milli'
    ? resource.allocatableRawMilli
    : resource.allocatableRawBytes
  const usageRaw = rawUnit === 'Milli'
    ? resource.estimatedRawMilli
    : resource.estimatedRawBytes
  const requestsRaw = rawUnit === 'Milli'
    ? resource.requestsRawMilli
    : resource.requestsRawBytes
  const metricsAvailable = Boolean(resource.metricsAvailable) || toNumber(usageRaw) > 0

  return {
    usageRate: clampPercent(resource.usageRate ?? percentFromRaw(usageRaw, allocatableRaw)),
    requestRate: clampPercent(resource.requestRate ?? percentFromRaw(requestsRaw, allocatableRaw)),
    metricsAvailable,
    used: resource.estimatedUsage || '--',
    requests: resource.requests || '--',
    allocatable: resource.allocatable || '--'
  }
}

const buildNodeResourceUsage = (resources = {}) => ({
  cpu: buildResourceUsage(resources.cpu, 'Milli'),
  memory: buildResourceUsage(resources.memory, 'Bytes')
})

const resetQuery = () => {
  queryParams.nodeName = ''
  queryParams.status = ''
  queryParams.role = ''
  nodeNameFilterKeyword.value = ''
  handleQuery()
}

// 导航到监控仪表板
const navigateToMonitoring = () => {
  router.push('/k8s/monitoring')
}

// 集群选择变化处理
const handleClusterChange = () => {
  if (selectedClusterId.value) {
    saveStoredClusterId(selectedClusterId.value, CLUSTER_SELECTION_SOURCE)
    queryParams.nodeName = ''
    nodeNameFilterKeyword.value = ''
    fetchNodeNameOptions()
    handleQuery()
  } else {
    saveStoredClusterId('', CLUSTER_SELECTION_SOURCE)
    nodeNameOptions.value = []
    tableData.value = []
  }
}


// 查看资源分配详情
const viewResources = async (row) => {
  try {
    loading.value = true
    const response = await k8sApi.getNodeResources(selectedClusterId.value, row.nodeName)
    
    const responseData = response.data || response
    if (responseData.code === 200 || responseData.success) {
      const rawData = responseData.data
      
      // 适配新的API响应格式
      nodeResources.value = {
        cpu: {
          capacity: rawData.capacity?.cpu || '0',
          allocatable: rawData.allocatable?.cpu || '0',
          allocated: rawData.allocated?.cpu || '0'
        },
        memory: {
          capacity: rawData.capacity?.memory || '0',
          allocatable: rawData.allocatable?.memory || '0',
          allocated: formatBytesToKi(rawData.allocated?.memory || '0')
        },
        pods: {
          capacity: parseInt(rawData.capacity?.pods || '0'),
          allocatable: parseInt(rawData.allocatable?.pods || '0'),
          allocated: rawData.podList?.length || 0
        },
        podList: rawData.podList || []
      }
      
      currentNode.value = row
      resourcesDialogVisible.value = true
      console.log('资源详情数据:', nodeResources.value)
    } else {
      ElMessage.error(responseData.message || '获取资源详情失败')
    }
  } catch (error) {
    console.error('获取资源详情失败:', error)
    ElMessage.error('获取资源详情失败，请检查网络连接')
  } finally {
    loading.value = false
  }
}

// 管理污点
const manageTaints = (row) => {
  currentNode.value = row
  taintForm.operation = 'add'
  taintForm.key = ''
  taintForm.value = ''
  taintForm.effect = 'NoSchedule'
  taintForm.selectedTaint = null // 选中的要移除的污点
  taintDialogVisible.value = true
}

// 管理标签
const manageLabels = (row) => {
  currentNode.value = row
  labelForm.operation = 'add'
  labelForm.key = ''
  labelForm.value = ''
  labelForm.selectedLabel = null // 选中的要移除的标签
  labelDialogVisible.value = true
}

// 封锁/解封节点
const toggleCordon = async (row) => {
  try {
    const action = row.schedulable ? '封锁' : '解封'
    await ElMessageBox.confirm(
      `确定要${action}节点 ${row.nodeName} 吗？`,
      `${action}节点确认`,
      {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
    
    const response = await k8sApi.cordonNode(selectedClusterId.value, row.nodeName, {
      unschedulable: row.schedulable,
      reason: `手动${action}节点`
    })
    
    const responseData = response.data || response
    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(`节点 ${row.nodeName} ${action}成功`)
      handleQuery()
    } else {
      ElMessage.error(responseData.message || `${action}节点失败`)
    }
  } catch (error) {
    if (error === 'cancel') {
      ElMessage.info('已取消操作')
    } else {
      console.error('节点封锁操作失败:', error)
      ElMessage.error('操作失败，请检查网络连接')
    }
  }
}

// yaml编辑
const handleEditYaml = (row) => {
  openYamlDialog({
    clusterId: selectedClusterId.value,
    resourceType: 'Node',
    resourceName: row.nodeName,
    fetchYaml: k8sApi.getNodeYaml,
    updateYaml: k8sApi.updateNodeYaml,
    fallbackYaml: `# Node ${row.nodeName} YAML\napiVersion: v1\nkind: Node\nmetadata:\n  name: ${row.nodeName}`,
    loadErrorMessage: '获取节点 YAML 失败',
    saveSuccessMessage: '节点 YAML 更新成功',
    saveErrorMessage: '更新节点 YAML 失败',
    onSaved: handleQuery
  })
}

// 驱逐节点
const drainNode = (row) => {
  currentNode.value = row
  drainForm.force = false
  drainForm.deleteLocalData = false
  drainForm.ignoreDaemonSets = true
  drainForm.gracePeriodSeconds = 30
  drainDialogVisible.value = true
}

// 提交污点操作
const submitTaintOperation = async () => {
  try {
    let params = {}
    
    if (taintForm.operation === 'add') {
      if (!taintForm.key) {
        ElMessage.warning('请输入污点键名')
        return
      }
      params = {
        key: taintForm.key,
        value: taintForm.value,
        effect: taintForm.effect
      }
    } else {
      // 移除操作
      if (!taintForm.selectedTaint) {
        ElMessage.warning('请选择要移除的污点')
        return
      }
      params = {
        key: taintForm.selectedTaint.key,
        effect: taintForm.selectedTaint.effect
      }
      if (taintForm.selectedTaint.value) {
        params.value = taintForm.selectedTaint.value
      }
    }
    
    const apiCall = taintForm.operation === 'add' 
      ? k8sApi.addNodeTaint(selectedClusterId.value, currentNode.value.nodeName, params)
      : k8sApi.removeNodeTaint(selectedClusterId.value, currentNode.value.nodeName, params)
    
    const response = await apiCall
    const responseData = response.data || response
    
    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(`污点${taintForm.operation === 'add' ? '添加' : '移除'}成功`)
      taintDialogVisible.value = false
      handleQuery()
    } else {
      ElMessage.error(responseData.message || '操作失败')
    }
  } catch (error) {
    console.error('污点操作失败:', error)
    ElMessage.error('操作失败，请检查网络连接')
  }
}

// 提交标签操作
const submitLabelOperation = async () => {
  try {
    let params = {}
    
    if (labelForm.operation === 'add') {
      if (!labelForm.key) {
        ElMessage.warning('请输入标签键名')
        return
      }
      params = {
        key: labelForm.key,
        value: labelForm.value
      }
    } else {
      // 移除操作
      if (!labelForm.selectedLabel) {
        ElMessage.warning('请选择要移除的标签')
        return
      }
      params = {
        key: labelForm.selectedLabel.key
      }
    }
    
    const apiCall = labelForm.operation === 'add' 
      ? k8sApi.addNodeLabel(selectedClusterId.value, currentNode.value.nodeName, params)
      : k8sApi.removeNodeLabel(selectedClusterId.value, currentNode.value.nodeName, params)
    
    const response = await apiCall
    const responseData = response.data || response
    
    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(`标签${labelForm.operation === 'add' ? '添加' : '移除'}成功`)
      labelDialogVisible.value = false
      handleQuery()
    } else {
      ElMessage.error(responseData.message || '操作失败')
    }
  } catch (error) {
    console.error('标签操作失败:', error)
    ElMessage.error('操作失败，请检查网络连接')
  }
}

// 提交驱逐操作
const submitDrainOperation = async () => {
  try {
    await ElMessageBox.confirm(
      `确定要驱逐节点 ${currentNode.value.nodeName} 上的所有Pod吗？这将会影响运行中的服务！`,
      '驱逐节点确认',
      {
        confirmButtonText: '确定驱逐',
        cancelButtonText: '取消',
        type: 'warning',
      }
    )
    
    const response = await k8sApi.drainNode(selectedClusterId.value, currentNode.value.nodeName, drainForm)
    
    const responseData = response.data || response
    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(`节点 ${currentNode.value.nodeName} 驱逐成功`)
      drainDialogVisible.value = false
      handleQuery()
    } else {
      ElMessage.error(responseData.message || '驱逐失败')
    }
  } catch (error) {
    if (error === 'cancel') {
      ElMessage.info('已取消驱逐操作')
    } else {
      console.error('节点驱逐失败:', error)
      ElMessage.error('驱逐失败，请检查网络连接')
    }
  }
}


// 获取当前节点的污点选项（用于移除操作）
const currentNodeTaints = computed(() => {
  if (!currentNode.value || !currentNode.value.taints) return []
  
  return currentNode.value.taints.map(taint => ({
    label: `${taint.key}=${taint.value || ''}:${taint.effect}`,
    value: taint,
    taint: taint
  }))
})

// 获取当前节点的标签选项（用于移除操作）
const currentNodeLabels = computed(() => {
  if (!currentNode.value || !currentNode.value.labels) return []
  
  // 过滤掉系统标签，只显示用户可以操作的标签
  const systemLabelPrefixes = [
    'kubernetes.io/',
    'beta.kubernetes.io/',
    'node-role.kubernetes.io/',
    'node.kubernetes.io/'
  ]
  
  return Object.entries(currentNode.value.labels)
    .filter(([key]) => !systemLabelPrefixes.some(prefix => key.startsWith(prefix)))
    .map(([key, value]) => ({
      label: `${key}=${value}`,
      value: { key, value },
      labelData: { key, value }
    }))
})

// 集群状态相关方法
const getClusterStatusTag = (status) => {
  const tagMap = {
    1: 'info',      // 创建中
    2: 'success',   // 运行中
    3: 'warning',   // 已停止
    4: 'danger',    // 异常
    5: 'info'       // 已删除
  }
  return tagMap[status] || 'info'
}

const getClusterStatusText = (status) => {
  const textMap = {
    1: '创建中',
    2: '运行中',
    3: '已停止',
    4: '异常',
    5: '已删除'
  }
  return textMap[status] || '未知'
}

// 字节转换为Ki单位
const formatBytesToKi = (bytes) => {
  if (!bytes || bytes === '0') return '0Ki'
  const bytesNum = parseInt(bytes)
  if (isNaN(bytesNum)) return '0Ki'
  const ki = Math.round(bytesNum / 1024)
  return `${ki}Ki`
}

// 计算CPU使用百分比
const getCpuPercentage = (cpu) => {
  if (!cpu || !cpu.allocatable || !cpu.allocated) return 0
  
  // 统一转换为millicores进行计算
  const parseCpuToMillicores = (cpuStr) => {
    if (!cpuStr) return 0
    if (cpuStr.endsWith('m')) {
      return parseFloat(cpuStr.replace('m', ''))
    }
    // 如果是纯数字，表示cores，需要转换为millicores
    return parseFloat(cpuStr) * 1000
  }
  
  const allocatable = parseCpuToMillicores(cpu.allocatable)
  const allocated = parseCpuToMillicores(cpu.allocated)
  
  console.log('CPU计算:', {
    allocatable: cpu.allocatable,
    allocated: cpu.allocated,
    allocatableMillicores: allocatable,
    allocatedMillicores: allocated,
    percentage: allocatable > 0 ? Math.round((allocated / allocatable) * 100) : 0
  })
  
  if (allocatable === 0) return 0
  return Math.round((allocated / allocatable) * 100)
}

// 计算内存使用百分比
const getMemoryPercentage = (memory) => {
  if (!memory || !memory.allocatable || !memory.allocated) return 0
  
  const getMemoryKi = (memStr) => {
    if (!memStr) return 0
    if (memStr.endsWith('Ki')) return parseInt(memStr.replace('Ki', ''))
    if (memStr.endsWith('Mi')) return parseInt(memStr.replace('Mi', '')) * 1024
    if (memStr.endsWith('Gi')) return parseInt(memStr.replace('Gi', '')) * 1024 * 1024
    return parseInt(memStr) || 0
  }
  
  const allocatable = getMemoryKi(memory.allocatable)
  const allocated = getMemoryKi(memory.allocated)
  
  if (allocatable === 0) return 0
  return Math.round((allocated / allocatable) * 100)
}

// 计算Pod使用百分比 - 适配新的资源数据结构
const getPodPercentage = (pods) => {
  if (!pods || !pods.allocatable || typeof pods.allocated === 'undefined') return 0
  
  const total = parseInt(pods.allocatable) || 0
  const allocated = parseInt(pods.allocated) || 0
  
  if (total === 0) return 0
  return Math.round((allocated / total) * 100)
}

onMounted(async () => {
  unsubscribeClusterSelection = subscribeStoredClusterId((clusterId, detail = {}) => {
    if (detail.source === CLUSTER_SELECTION_SOURCE || String(selectedClusterId.value || '') === String(clusterId || '')) {
      return
    }

    selectedClusterId.value = clusterId
    if (selectedClusterId.value) {
      queryParams.nodeName = ''
      nodeNameFilterKeyword.value = ''
      fetchNodeNameOptions()
      handleQuery()
    } else {
      nodeNameOptions.value = []
      tableData.value = []
    }
  })

  await fetchClusterList()
  if (selectedClusterId.value) {
    await Promise.all([
      fetchNodeNameOptions(),
      handleQuery()
    ])
  }
})

onBeforeUnmount(() => {
  if (unsubscribeClusterSelection) {
    unsubscribeClusterSelection()
  }
})
</script>

<template>
  <div class="k8s-nodes-management">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/k8s.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>K8s 节点管理</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>统一查看节点状态、角色、资源水位与调度维护动作。</p>
      </div>
      <div class="header-actions">
        <el-select
          v-model="selectedClusterId"
          placeholder="请选择集群"
          class="cluster-select"
          @change="handleClusterChange"
          :loading="!clusterList.length"
        >
          <el-option
            v-for="cluster in clusterList"
            :key="cluster.id"
            :label="cluster.name"
            :value="cluster.id"
            :disabled="cluster.status !== 2"
          >
            <div class="cluster-option">
              <span class="cluster-name">{{ cluster.name }}</span>
              <el-tag
                :type="getClusterStatusTag(cluster.status)"
                size="small"
                class="cluster-status-tag"
              >
                {{ getClusterStatusText(cluster.status) }}
              </el-tag>
            </div>
          </el-option>
        </el-select>
        <el-button :icon="Monitor" type="warning" class="monitor-action" @click="navigateToMonitoring">
          监控仪表板
        </el-button>
      </div>
    </header>

    <section class="nodes-card page-card">
      <!-- 搜索表单 -->
      <div class="search-section">
        <el-form :inline="true" :model="queryParams" class="search-form">
          <el-form-item label="节点名称">
            <el-select
              v-model="queryParams.nodeName"
              placeholder="请选择节点名称"
              clearable
              filterable
              :filter-method="handleNodeNameFilter"
              :loading="nodeNameOptionsLoading"
              style="width: 200px"
              @visible-change="handleNodeNameVisibleChange"
            >
              <el-option
                v-for="item in filteredNodeNameOptions"
                :key="item.value"
                :label="item.label"
                :value="item.value"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="节点角色">
            <el-select
              v-model="queryParams.role"
              placeholder="请选择角色"
              clearable
              style="width: 150px"
            >
              <el-option
                v-for="item in roleOptions"
                :key="item.value"
                :label="item.label"
                :value="item.value"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="节点状态">
            <el-select
              v-model="queryParams.status"
              placeholder="请选择状态"
              clearable
              style="width: 150px"
            >
              <el-option
                v-for="item in statusOptions"
                :key="item.value"
                :label="item.label"
                :value="item.value"
              />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :icon="Search" class="query-button" @click="handleQuery">
              查询
            </el-button>
            <el-button type="warning" :icon="Refresh" class="reset-button" @click="resetQuery">
              重置
            </el-button>
          </el-form-item>
        </el-form>
      </div>

      <!-- 节点列表表格 -->
      <NodesTable
        :table-data="tableData"
        :loading="loading"
        :selected-cluster-id="selectedClusterId"
        @view-resources="viewResources"
        @manage-taints="manageTaints"
        @manage-labels="manageLabels"
        @toggle-cordon="toggleCordon"
        @drain-node="drainNode"
        @edit-yaml="handleEditYaml"
      />
    </section>


    <!-- 资源详情对话框 -->
    <el-dialog
      v-model="resourcesDialogVisible"
      :title="`资源详情 - ${currentNode.nodeName || ''}`"
      width="900px"
      class="resources-dialog"
      modal-class="k8s-node-dialog-overlay"
    >
      <div class="resources-content" v-if="nodeResources">
        <!-- 资源概览 -->
        <div class="resources-overview">
          <el-row :gutter="20">
            <el-col :span="8">
              <el-card class="resource-card">
                <div class="resource-header">
                  <span class="resource-title">CPU</span>
                  <span class="resource-unit">cores</span>
                </div>
                <div class="resource-progress">
                  <el-progress
                    :percentage="getCpuPercentage(nodeResources.cpu)"
                    :color="getProgressColor(getCpuPercentage(nodeResources.cpu))"
                  />
                </div>
                <div class="resource-details">
                  <span>已分配: {{ nodeResources.cpu?.allocated || '0' }}</span>
                  <span>总容量: {{ nodeResources.cpu?.allocatable || '0' }}</span>
                </div>
              </el-card>
            </el-col>
            <el-col :span="8">
              <el-card class="resource-card">
                <div class="resource-header">
                  <span class="resource-title">内存</span>
                  <span class="resource-unit">GB</span>
                </div>
                <div class="resource-progress">
                  <el-progress
                    :percentage="getMemoryPercentage(nodeResources.memory)"
                    :color="getProgressColor(getMemoryPercentage(nodeResources.memory))"
                  />
                </div>
                <div class="resource-details">
                  <span>已分配: {{ formatMemory(nodeResources.memory?.allocated || '0') }}</span>
                  <span>总容量: {{ formatMemory(nodeResources.memory?.allocatable || '0') }}</span>
                </div>
              </el-card>
            </el-col>
            <el-col :span="8">
              <el-card class="resource-card">
                <div class="resource-header">
                  <span class="resource-title">Pod</span>
                  <span class="resource-unit">个</span>
                </div>
                <div class="resource-progress">
                  <el-progress
                    :percentage="getPodPercentage(nodeResources.pods)"
                    :color="getProgressColor(getPodPercentage(nodeResources.pods))"
                  />
                </div>
                <div class="resource-details">
                  <span>运行中: {{ nodeResources.pods?.allocated || 0 }}</span>
                  <span>最大容量: {{ nodeResources.pods?.allocatable || 0 }}</span>
                </div>
              </el-card>
            </el-col>
          </el-row>
        </div>
        
        <!-- Pod列表 -->
        <div class="pods-section" v-if="nodeResources.podList && nodeResources.podList.length > 0">
          <h4>Pod列表</h4>
          <el-table :data="nodeResources.podList" size="small" style="width: 100%">
            <el-table-column prop="name" label="Pod名称" min-width="200" />
            <el-table-column prop="namespace" label="命名空间" min-width="120" />
            <el-table-column label="CPU请求" min-width="100">
              <template #default="{ row }">
                <span>{{ row.requests?.cpu || '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="内存请求" min-width="100">
              <template #default="{ row }">
                <span>{{ row.requests?.memory || '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="CPU限制" min-width="100">
              <template #default="{ row }">
                <span>{{ row.limits?.cpu || '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="内存限制" min-width="100">
              <template #default="{ row }">
                <span>{{ row.limits?.memory || '-' }}</span>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </div>
    </el-dialog>

    <!-- 污点管理对话框 -->
    <el-dialog
      v-model="taintDialogVisible"
      :title="`管理污点 - ${currentNode.nodeName || ''}`"
      width="500px"
      class="taint-dialog"
      modal-class="k8s-node-dialog-overlay"
    >
      <el-form :model="taintForm" label-width="100px" class="node-dialog-form">
        <el-form-item label="操作类型" required>
          <el-radio-group v-model="taintForm.operation">
            <el-radio label="add">添加污点</el-radio>
            <el-radio label="remove">移除污点</el-radio>
          </el-radio-group>
        </el-form-item>
        
        <!-- 添加污点的表单 -->
        <template v-if="taintForm.operation === 'add'">
          <el-form-item label="污点键名" required>
            <el-input
              v-model="taintForm.key"
              placeholder="请输入污点键名，如：maintenance"
            />
          </el-form-item>
          <el-form-item label="污点值">
            <el-input
              v-model="taintForm.value"
              placeholder="请输入污点值，如：scheduled（可选）"
            />
          </el-form-item>
          <el-form-item label="污点效果" required>
            <el-select v-model="taintForm.effect" style="width: 100%" placeholder="请选择污点效果">
              <el-option
                v-for="option in effectOptions"
                :key="option.value"
                :label="option.label"
                :value="option.value"
              >
                <div class="effect-option">
                  <div class="effect-label">{{ option.label }}</div>
                  <div class="effect-description">{{ option.description }}</div>
                </div>
              </el-option>
            </el-select>
          </el-form-item>
        </template>
        
        <!-- 移除污点的表单 -->
        <template v-else>
          <el-form-item label="选择污点" required v-if="currentNodeTaints.length > 0">
            <el-select 
              v-model="taintForm.selectedTaint" 
              style="width: 100%" 
              placeholder="请选择要移除的污点"
              value-key="key"
            >
              <el-option
                v-for="option in currentNodeTaints"
                :key="option.label"
                :label="option.label"
                :value="option.taint"
              >
                <div class="taint-option">
                  <el-tag type="warning" size="small">{{ option.label }}</el-tag>
                </div>
              </el-option>
            </el-select>
          </el-form-item>
          <el-form-item v-else>
            <el-alert
              title="该节点没有污点"
              type="info"
              description="当前节点没有污点，无需移除操作"
              show-icon
              :closable="false"
            />
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="taintDialogVisible = false">取消</el-button>
          <el-button 
            type="primary" 
            @click="submitTaintOperation"
            :disabled="taintForm.operation === 'remove' && currentNodeTaints.length === 0"
          >
            {{ taintForm.operation === 'add' ? '添加污点' : '移除污点' }}
          </el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 标签管理对话框 -->
    <el-dialog
      v-model="labelDialogVisible"
      :title="`管理标签 - ${currentNode.nodeName || ''}`"
      width="500px"
      class="label-dialog"
      modal-class="k8s-node-dialog-overlay"
    >
      <el-form :model="labelForm" label-width="100px" class="node-dialog-form">
        <el-form-item label="操作类型" required>
          <el-radio-group v-model="labelForm.operation">
            <el-radio label="add">添加标签</el-radio>
            <el-radio label="remove">移除标签</el-radio>
          </el-radio-group>
        </el-form-item>
        
        <!-- 添加标签的表单 -->
        <template v-if="labelForm.operation === 'add'">
          <el-form-item label="标签键名" required>
            <el-input
              v-model="labelForm.key"
              placeholder="请输入标签键名，如：environment"
            />
          </el-form-item>
          <el-form-item label="标签值" required>
            <el-input
              v-model="labelForm.value"
              placeholder="请输入标签值，如：production"
            />
          </el-form-item>
        </template>
        
        <!-- 移除标签的表单 -->
        <template v-else>
          <el-form-item label="选择标签" required v-if="currentNodeLabels.length > 0">
            <el-select 
              v-model="labelForm.selectedLabel" 
              style="width: 100%" 
              placeholder="请选择要移除的标签"
              value-key="key"
            >
              <el-option
                v-for="option in currentNodeLabels"
                :key="option.label"
                :label="option.label"
                :value="option.labelData"
              >
                <div class="label-option">
                  <el-tag type="info" size="small">{{ option.label }}</el-tag>
                </div>
              </el-option>
            </el-select>
          </el-form-item>
          <el-form-item v-else>
            <el-alert
              title="该节点没有可移除的标签"
              type="info"
              description="当前节点没有用户自定义标签，只有系统标签无法移除"
              show-icon
              :closable="false"
            />
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="labelDialogVisible = false">取消</el-button>
          <el-button 
            type="primary" 
            @click="submitLabelOperation"
            :disabled="labelForm.operation === 'remove' && currentNodeLabels.length === 0"
          >
            {{ labelForm.operation === 'add' ? '添加标签' : '移除标签' }}
          </el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 驱逐节点对话框 -->
    <el-dialog
      v-model="drainDialogVisible"
      :title="`驱逐节点 - ${currentNode.nodeName || ''}`"
      width="500px"
      class="drain-dialog"
      modal-class="k8s-node-dialog-overlay"
    >
      <el-form :model="drainForm" label-width="120px" class="node-dialog-form">
        <el-form-item label="强制驱逐">
          <el-switch
            v-model="drainForm.force"
            active-text="是"
            inactive-text="否"
          />
          <div class="form-tip">强制驱逐会删除不受控制器管理的Pod</div>
        </el-form-item>
        <el-form-item label="删除本地数据">
          <el-switch
            v-model="drainForm.deleteLocalData"
            active-text="是"
            inactive-text="否"
          />
          <div class="form-tip">删除使用emptyDir的Pod</div>
        </el-form-item>
        <el-form-item label="忽略DaemonSet">
          <el-switch
            v-model="drainForm.ignoreDaemonSets"
            active-text="是"
            inactive-text="否"
          />
          <div class="form-tip">忽略DaemonSet管理的Pod</div>
        </el-form-item>
        <el-form-item label="优雅终止时间">
          <el-input-number
            v-model="drainForm.gracePeriodSeconds"
            :min="0"
            :max="300"
            style="width: 100%"
          />
          <div class="form-tip">Pod优雅终止等待时间（秒）</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="drainDialogVisible = false">取消</el-button>
          <el-button type="danger" @click="submitDrainOperation">驱逐</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script>
// 辅助函数
const getProgressColor = (percentage) => {
  if (percentage < 60) return '#67c23a'
  if (percentage < 80) return '#e6a23c'
  return '#f56c6c'
}

// 导出辅助函数以便在模板中使用
export { getProgressColor }
</script>

<style scoped>
.k8s-nodes-management {
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
  --node-page-height-offset: 190px;
  min-height: calc(100vh - var(--node-page-height-offset));
  padding: 6px;
  box-sizing: border-box;
  background:
    radial-gradient(circle at 16% 10%, rgba(76, 139, 245, 0.06), transparent 26%),
    linear-gradient(180deg, #f7f9fc 0%, var(--aiops-bg-page) 100%);
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.nodes-card {
  position: relative;
  display: flex;
  flex-direction: column;
  height: calc(100vh - 12px - var(--node-page-height-offset));
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

.nodes-card::before {
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

.cluster-select {
  width: 250px;
}

.primary-action {
  min-width: 108px;
  height: 32px;
  border: none;
  border-radius: 14px;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
  color: #ffffff;
  font-weight: 600;
  box-shadow: 0 12px 20px rgba(76, 139, 245, 0.18);
}

.search-section {
  flex-shrink: 0;
  margin-bottom: 12px;
  padding: 14px 16px;
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

.query-button {
  min-width: 76px;
  border: none;
  border-radius: 13px;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
  font-weight: 600;
  box-shadow: 0 10px 18px rgba(76, 139, 245, 0.16);
}

.reset-button {
  min-width: 76px;
  border: 1px solid rgba(214, 223, 235, 0.96);
  border-radius: 13px;
  background: rgba(255, 255, 255, 0.92);
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

.reset-button:hover,
.reset-button:focus {
  border-color: rgba(153, 181, 230, 0.95);
  color: var(--aiops-brand);
  background: #ffffff;
}

/* 对话框样式 */
.resources-dialog :deep(.el-dialog),
.taint-dialog :deep(.el-dialog),
.label-dialog :deep(.el-dialog),
.drain-dialog :deep(.el-dialog) {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 24px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
  box-shadow: 0 36px 110px rgba(15, 23, 42, 0.11);
  backdrop-filter: blur(18px);
}

.resources-dialog :deep(.el-dialog__header),
.taint-dialog :deep(.el-dialog__header),
.label-dialog :deep(.el-dialog__header),
.drain-dialog :deep(.el-dialog__header) {
  padding: 16px 20px 14px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.84);
}

.resources-dialog :deep(.el-dialog__title),
.taint-dialog :deep(.el-dialog__title),
.label-dialog :deep(.el-dialog__title),
.drain-dialog :deep(.el-dialog__title) {
  color: var(--aiops-text-primary);
  font-size: 17px;
  font-weight: 700;
}

.resources-dialog :deep(.el-dialog__body),
.taint-dialog :deep(.el-dialog__body),
.label-dialog :deep(.el-dialog__body),
.drain-dialog :deep(.el-dialog__body) {
  padding: 18px 20px;
  background:
    radial-gradient(circle at top left, rgba(76, 139, 245, 0.04), transparent 22%),
    linear-gradient(180deg, #f8fafc 0%, #f5f8fc 100%);
}

.resources-dialog :deep(.el-dialog__footer),
.taint-dialog :deep(.el-dialog__footer),
.label-dialog :deep(.el-dialog__footer),
.drain-dialog :deep(.el-dialog__footer) {
  padding: 10px 20px 16px;
  background: rgba(247, 250, 255, 0.72);
  border-top: 1px solid rgba(223, 231, 242, 0.76);
}

.node-detail-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.taints-section {
  margin-top: 20px;
}

.taints-section h4,
.section-title {
  margin: 0 0 12px 0;
  color: #6b9bd1;
  font-weight: 600;
}

.taints-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.taint-tag {
  cursor: pointer;
  transition: all 0.3s ease;
  display: flex;
  align-items: center;
  gap: 4px;
}

.taint-tag:hover {
  transform: translateY(-2px);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
}

/* 资源详情样式 */
.resources-content {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.resources-overview {
  margin-bottom: 20px;
}

.resource-card {
  border-radius: 18px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  background: rgba(255, 255, 255, 0.84);
  box-shadow: none;
}

.resource-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.resource-title {
  font-weight: 600;
  color: var(--aiops-text-primary);
  font-size: 16px;
}

.resource-unit {
  font-size: 12px;
  color: #909399;
  background: rgba(103, 126, 234, 0.1);
  padding: 2px 8px;
  border-radius: 12px;
}

.resource-progress {
  margin-bottom: 12px;
}

.resource-details {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: #606266;
}

.pods-section {
  margin-top: 20px;
}

.pods-section h4 {
  margin: 0 0 12px 0;
  color: var(--aiops-text-primary);
  font-weight: 600;
}

/* 表单样式 */
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

/* 通用元素样式 */
.el-tag {
  font-weight: 500;
  border-radius: 999px;
}

.el-button {
  border-radius: 13px;
  font-weight: 600;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.el-input :deep(.el-input__wrapper),
.el-select :deep(.el-input__wrapper) {
  min-height: 32px;
  background: rgba(252, 253, 255, 0.98);
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 13px;
  box-shadow: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;
}

.el-input :deep(.el-input__wrapper):hover,
.el-select :deep(.el-input__wrapper):hover {
  border-color: #c0c4cc;
}

.el-input :deep(.el-input__wrapper.is-focus),
.el-select :deep(.el-input__wrapper.is-focus) {
  border-color: rgba(120, 156, 221, 0.9);
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
  background: #ffffff;
}

.el-loading-mask {
  background-color: rgba(247, 250, 255, 0.68);
  backdrop-filter: blur(4px);
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
  color: var(--aiops-brand);
}

.cluster-status-tag {
  margin-left: 8px;
}

/* 污点效果选项样式 */
.effect-option {
  padding: 4px 0;
}

.effect-label {
  font-weight: 500;
  color: var(--aiops-text-primary);
  font-size: 14px;
  margin-bottom: 2px;
}

.effect-description {
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
  margin-left: 8px;
}

/* 污点选项样式 */
.taint-option {
  padding: 4px 0;
  display: flex;
  align-items: center;
}

/* 标签选项样式 */
.label-option {
  padding: 4px 0;
  display: flex;
  align-items: center;
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
    min-width: 200px;
  }
}

@media (max-width: 768px) {
  .k8s-nodes-management {
    --node-page-height-offset: 131px;
    padding: 4px;
  }

  .nodes-card {
    height: calc(100vh - 8px - var(--node-page-height-offset));
    padding: 48px 8px 8px;
    border-radius: 22px;
  }

  .page-hero {
    flex-direction: column;
    align-items: stretch;
  }

  .header-actions,
  .cluster-select,
  .primary-action {
    width: 100%;
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
  .search-form :deep(.el-select) {
    width: 100% !important;
  }

  .operation-buttons {
    flex-direction: column;
    gap: 4px;
  }

}

/* DESIGN.md compact workbench overrides */
.k8s-nodes-management {
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

.nodes-card {
  height: auto;
  padding: 10px;
}

.nodes-card::before {
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

.cluster-select {
  width: 230px;
}

.monitor-action,
.query-button,
.reset-button {
  height: 34px;
  min-width: 76px;
  border-radius: 8px;
  color: #fff;
  font-weight: 600;
  box-shadow: none;
  transition: background-color 160ms ease, border-color 160ms ease, box-shadow 160ms ease;
}

.monitor-action {
  min-width: 108px;
}

.monitor-action:hover,
.monitor-action:focus,
.query-button:hover,
.query-button:focus,
.reset-button:hover,
.reset-button:focus {
  transform: none;
  color: #fff;
}

.search-section {
  flex-shrink: 0;
  margin-bottom: 8px;
  padding: 8px 10px;
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

.k8s-nodes-management :deep(.el-input__wrapper),
.k8s-nodes-management :deep(.el-select__wrapper),
.k8s-nodes-management :deep(.el-input-number) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.k8s-nodes-management :deep(.el-input__wrapper:hover),
.k8s-nodes-management :deep(.el-select__wrapper:hover),
.k8s-nodes-management :deep(.el-input-number:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.k8s-nodes-management :deep(.el-input__wrapper.is-focus),
.k8s-nodes-management :deep(.el-select__wrapper.is-focused) {
  box-shadow: 0 0 0 1px #409eff inset;
}

.resources-content {
  gap: 14px;
}

.resources-overview,
.pods-section {
  margin: 0;
}

.resource-card {
  border: 1px solid #dce8f5;
  border-radius: 10px;
  background: #fbfdff;
}

.resource-card :deep(.el-card__body) {
  padding: 14px;
}

.resource-header,
.resource-progress {
  margin-bottom: 8px;
}

.resource-title {
  font-size: 14px;
  font-weight: 700;
}

.resource-unit {
  padding: 2px 7px;
  border-radius: 999px;
  background: #eef6ff;
  color: #2563eb;
}

.pods-section h4 {
  margin-bottom: 8px;
  font-size: 14px;
  font-weight: 700;
}

.taint-tag:hover {
  transform: none;
}

:global(.k8s-node-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 24px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: 15px !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.k8s-node-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: linear-gradient(90deg, #fff6e7 0%, #f7fbff 36%, #ecf7ff 100%);
}

:global(.k8s-node-dialog-overlay .el-dialog__title) {
  color: #172033;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.k8s-node-dialog-overlay .el-dialog__headerbtn) {
  top: 12px;
  right: 14px;
  width: 32px;
  height: 32px;
}

:global(.k8s-node-dialog-overlay .el-dialog__body) {
  max-height: calc(100vh - 190px);
  overflow: auto;
  padding: 14px 18px;
  background: #fff;
}

:global(.k8s-node-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.k8s-node-dialog-overlay .node-dialog-form .el-form-item) {
  margin-bottom: 14px;
}

:global(.k8s-node-dialog-overlay .node-dialog-form .el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

:global(.k8s-node-dialog-overlay .el-input__wrapper),
:global(.k8s-node-dialog-overlay .el-select__wrapper),
:global(.k8s-node-dialog-overlay .el-input-number) {
  min-height: 34px;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.k8s-node-dialog-overlay .el-button) {
  min-height: 34px;
  border-radius: 8px;
  font-weight: 600;
}

:global(.k8s-node-dialog-overlay .el-table) {
  overflow: hidden;
  border: 1px solid #e3ebf5;
  border-radius: 9px;
}

:global(.k8s-node-dialog-overlay .el-table th.el-table__cell) {
  background: #f3f7fc;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

@media (max-width: 768px) {
  .k8s-nodes-management {
    height: calc(100vh - 110px);
    padding: 6px;
  }

  .ops-page-header {
    align-items: flex-start;
    flex-wrap: wrap;
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

  .header-actions {
    width: 100%;
    margin-left: 0;
  }

  .cluster-select {
    flex: 1;
    width: auto;
  }

  .monitor-action {
    width: auto;
  }

  .page-card,
  .nodes-card {
    padding: 8px;
    border-radius: 12px;
  }

  .search-form :deep(.el-form-item) {
    margin-bottom: 8px;
  }
}
</style>
