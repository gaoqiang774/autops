<script setup>
import { ref, reactive, onBeforeUnmount, onMounted, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Search,
  Refresh,
  Plus
} from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'
import {
  getStoredClusterId,
  saveStoredClusterId,
  resolvePreferredClusterId,
  subscribeStoredClusterId
} from '@/utils/k8sClusterSelection'
import NamespacesTable from './namespaces/NamespacesTable.vue'
import CreateNamespaceDialog from './namespaces/CreateNamespaceDialog.vue'
import NamespaceLabelsDialog from './namespaces/NamespaceLabelsDialog.vue'
import { openYamlDialog } from './services/yamlDialog'

const CLUSTER_SELECTION_SOURCE = 'k8s-namespace'

const loading = ref(false)
const queryParams = reactive({
  namespaceName: '',
  status: ''
})

const statusOptions = [
  { label: '全部', value: '' },
  { label: '活跃', value: 'Active' },
  { label: '终止中', value: 'Terminating' },
  { label: '未知', value: 'Unknown' }
]

const tableData = ref([])
const namespaceNameOptions = ref([])
const namespaceNameOptionsLoading = ref(false)
const namespaceNameFilterKeyword = ref('')
const selectedClusterId = ref(getStoredClusterId())
const clusterList = ref([])
let unsubscribeClusterSelection = null

// 分页
const currentPage = ref(1)
const pageSize = ref(10)
const total = computed(() => tableData.value.length)
const pagedTableData = computed(() =>
  tableData.value.slice((currentPage.value - 1) * pageSize.value, currentPage.value * pageSize.value)
)
const handlePageChange = (page) => { currentPage.value = page }
const handleSizeChange = (size) => { pageSize.value = size; currentPage.value = 1 }

// 对话框状态
const createNamespaceDialogVisible = ref(false)
const namespaceDetailDialogVisible = ref(false)
const resourceQuotaDialogVisible = ref(false)
const limitRangeDialogVisible = ref(false)
const createQuotaDialogVisible = ref(false)
const createLimitRangeDialogVisible = ref(false)
const labelsDialogVisible = ref(false)

// 当前操作的命名空间
const currentNamespace = ref({})

// 创建对话框引用
const createDialogRef = ref(null)

// 资源配额数据 - 确保初始值为空数组
const resourceQuotas = ref([])
const limitRanges = ref([])

// ResourceQuota表单
const quotaForm = reactive({
  name: '',
  cpuLimit: '',
  memoryLimit: '',
  storageLimit: '',
  podLimit: '',
  serviceLimit: ''
})

// LimitRange表单
const limitRangeForm = reactive({
  name: '',
  containerCpuMin: '',
  containerCpuMax: '',
  containerMemoryMin: '',
  containerMemoryMax: '',
  containerCpuDefault: '',
  containerMemoryDefault: ''
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

const fetchNamespaceNameOptions = async () => {
  if (!selectedClusterId.value) {
    namespaceNameOptions.value = []
    return
  }

  try {
    namespaceNameOptionsLoading.value = true
    const response = await k8sApi.getNamespaces(selectedClusterId.value)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      const namespaces = responseData.data?.namespaces || responseData.data || []
      const namespacesArray = Array.isArray(namespaces) ? namespaces : []
      namespaceNameOptions.value = namespacesArray.map(item => ({
        label: item.name,
        value: item.name
      }))

      if (queryParams.namespaceName && !namespaceNameOptions.value.some(option => option.value === queryParams.namespaceName)) {
        queryParams.namespaceName = ''
      }
    } else {
      namespaceNameOptions.value = []
    }
  } catch (error) {
    console.error('获取命名空间下拉选项失败:', error)
    namespaceNameOptions.value = []
  } finally {
    namespaceNameOptionsLoading.value = false
  }
}

const filteredNamespaceNameOptions = computed(() => {
  const keyword = namespaceNameFilterKeyword.value.trim().toLowerCase()
  if (!keyword) return namespaceNameOptions.value

  return namespaceNameOptions.value.filter(option =>
    option.label.toLowerCase().includes(keyword) ||
    option.value.toLowerCase().includes(keyword)
  )
})

const handleNamespaceNameFilter = (keyword) => {
  namespaceNameFilterKeyword.value = keyword
}

const handleNamespaceNameVisibleChange = (visible) => {
  if (visible && selectedClusterId.value && !namespaceNameOptions.value.length) {
    fetchNamespaceNameOptions()
  }
}

const handleQuery = async () => {
  try {
    if (!selectedClusterId.value) {
      ElMessage.warning('请选择一个集群')
      return
    }

    loading.value = true
    currentPage.value = 1

    let response
    if (queryParams.namespaceName) {
      // 有名称时走模糊查询 API
      response = await k8sApi.searchNamespaces(selectedClusterId.value, queryParams.namespaceName)
    } else {
      // 无名称时走完整列表 API（可带状态过滤）
      const params = {}
      if (queryParams.status) params.status = queryParams.status
      response = await k8sApi.getNamespaces(selectedClusterId.value, Object.keys(params).length > 0 ? params : undefined)
    }

    const responseData = response.data || response
    console.log('命名空间列表API响应:', responseData)

    if (responseData.code === 200 || responseData.success) {
      const namespaces = responseData.data?.namespaces || responseData.data || []
      const namespacesArray = Array.isArray(namespaces) ? namespaces : []
      tableData.value = namespacesArray.map(ns => ({
        name: ns.name,
        status: ns.status || 'Active',
        age: formatAge(ns.createdAt),
        createdAt: ns.createdAt,
        labels: ns.labels || {},
        annotations: ns.annotations || {},
        resourceQuotas: ns.resourceQuotas || [],
        limitRanges: ns.limitRanges || [],
        resourceCount: {
          pods: ns.resourceCount?.podCount || 0,
          services: ns.resourceCount?.serviceCount || 0,
          secrets: ns.resourceCount?.secretCount || 0,
          configMaps: ns.resourceCount?.configMapCount || 0
        }
      }))

      console.log('命名空间列表加载成功:', tableData.value)
    } else {
      ElMessage.error(responseData.message || '获取命名空间列表失败')
      tableData.value = []
    }
  } catch (error) {
    console.error('获取命名空间列表失败:', error)

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

const resetQuery = () => {
  queryParams.namespaceName = ''
  queryParams.status = ''
  namespaceNameFilterKeyword.value = ''
  handleQuery()
}

// 集群选择变化处理
const handleClusterChange = () => {
  if (selectedClusterId.value) {
    saveStoredClusterId(selectedClusterId.value, CLUSTER_SELECTION_SOURCE)
    queryParams.namespaceName = ''
    namespaceNameFilterKeyword.value = ''
    fetchNamespaceNameOptions()
    handleQuery()
  } else {
    saveStoredClusterId('', CLUSTER_SELECTION_SOURCE)
    namespaceNameOptions.value = []
    tableData.value = []
  }
}

// 创建命名空间
const showCreateNamespaceDialog = () => {
  createNamespaceDialogVisible.value = true
}

// 处理创建命名空间成功
const handleCreateNamespace = async (data) => {
  try {
    if (createDialogRef.value) {
      createDialogRef.value.setSubmitting(true)
    }

    const response = await k8sApi.createNamespace(selectedClusterId.value, data)
    const responseData = response.data || response
    
    if (responseData.code === 200 || responseData.success) {
      ElMessage.success('命名空间创建成功')
      createNamespaceDialogVisible.value = false
      await Promise.all([
        fetchNamespaceNameOptions(),
        handleQuery()
      ])
    } else {
      ElMessage.error(responseData.message || '创建命名空间失败')
    }
  } catch (error) {
    console.error('创建命名空间失败:', error)
    ElMessage.error('创建命名空间失败，请检查网络连接')
  } finally {
    if (createDialogRef.value) {
      createDialogRef.value.setSubmitting(false)
    }
  }
}

// 查看命名空间详情
const viewNamespaceDetail = async (row) => {
  try {
    loading.value = true
    const response = await k8sApi.getNamespaceDetail(selectedClusterId.value, row.name)
    
    const responseData = response.data || response
    if (responseData.code === 200 || responseData.success) {
      // 如果返回的data中有单个命名空间对象，直接使用；否则使用整个data
      currentNamespace.value = responseData.data?.name ? responseData.data : row
      namespaceDetailDialogVisible.value = true
    } else {
      ElMessage.error(responseData.message || '获取命名空间详情失败')
    }
  } catch (error) {
    console.error('获取命名空间详情失败:', error)
    ElMessage.error('获取命名空间详情失败，请检查网络连接')
  } finally {
    loading.value = false
  }
}

// 管理ResourceQuota
const manageResourceQuotas = async (row) => {
  try {
    currentNamespace.value = row
    
    const response = await k8sApi.getResourceQuotas(selectedClusterId.value, row.name)
    const responseData = response.data || response
    
    if (responseData.code === 200 || responseData.success) {
      // 适配后端返回的数据结构
      const quotaData = responseData.data?.resourceQuotas || responseData.data || []
      resourceQuotas.value = Array.isArray(quotaData) ? quotaData : []
      resourceQuotaDialogVisible.value = true
    } else {
      ElMessage.error(responseData.message || '获取ResourceQuota失败')
    }
  } catch (error) {
    console.error('获取ResourceQuota失败:', error)
    ElMessage.error('获取ResourceQuota失败，请检查网络连接')
  }
}

// 管理LimitRange
const manageLimitRanges = async (row) => {
  try {
    currentNamespace.value = row
    
    const response = await k8sApi.getLimitRanges(selectedClusterId.value, row.name)
    const responseData = response.data || response
    
    if (responseData.code === 200 || responseData.success) {
      // 适配后端返回的数据结构：后端返回的是 limits 直接在对象下，需要转换为前端期望的 spec.limits 结构
      const limitData = responseData.data?.limitRanges || responseData.data || []
      const mappedData = Array.isArray(limitData) ? limitData.map(item => ({
        name: item.name,
        createdAt: item.createdAt,
        spec: {
          limits: item.limits || []  // 将后端的 limits 映射到 spec.limits
        }
      })) : []
      
      limitRanges.value = mappedData
      limitRangeDialogVisible.value = true
      
      console.log('LimitRange数据加载成功:', limitRanges.value)
    } else {
      ElMessage.error(responseData.message || '获取LimitRange失败')
    }
  } catch (error) {
    console.error('获取LimitRange失败:', error)
    ElMessage.error('获取LimitRange失败，请检查网络连接')
  }
}

// 删除命名空间
const deleteNamespace = async (row) => {
  try {
    // 额外检查系统命名空间
    if (isSystemNamespace(row.name)) {
      ElMessage.warning('系统命名空间不可删除')
      return
    }
    
    await ElMessageBox.confirm(
      `确定要删除命名空间 ${row.name} 吗？删除后将无法恢复，且会删除该命名空间下的所有资源！`,
      '删除命名空间确认',
      {
        confirmButtonText: '确定删除',
        cancelButtonText: '取消',
        type: 'error',
      }
    )
    
    const response = await k8sApi.deleteNamespace(selectedClusterId.value, row.name)
    
    const responseData = response.data || response
    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(`命名空间 ${row.name} 删除成功`)
      await Promise.all([
        fetchNamespaceNameOptions(),
        handleQuery()
      ])
    } else {
      ElMessage.error(responseData.message || '删除命名空间失败')
    }
  } catch (error) {
    if (error === 'cancel') {
      ElMessage.info('已取消删除操作')
    } else {
      console.error('删除命名空间失败:', error)
      ElMessage.error('删除命名空间失败，请检查网络连接')
    }
  }
}

const getStatusTag = (status) => {
  const tagMap = {
    'Active': 'success',
    'Terminating': 'warning',
    'Unknown': 'info'
  }
  return tagMap[status] || 'info'
}

const getStatusText = (status) => {
  const textMap = {
    'Active': '活跃',
    'Terminating': '终止中',
    'Unknown': '未知'
  }
  return textMap[status] || '未知'
}

// 集群状态相关方法
const getClusterStatusTag = (status) => {
  const tagMap = {
    1: 'info',
    2: 'success',
    3: 'warning',
    4: 'danger',
    5: 'info'
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

// 获取用户自定义标签
const getUserLabels = (labels) => {
  if (!labels) return {}
  
  const systemLabelPrefixes = [
    'kubernetes.io/',
    'beta.kubernetes.io/',
    'app.kubernetes.io/'
  ]
  
  const userLabels = {}
  Object.entries(labels).forEach(([key, value]) => {
    if (!systemLabelPrefixes.some(prefix => key.startsWith(prefix))) {
      userLabels[key] = value
    }
  })
  
  return userLabels
}

// 获取系统标签
const getSystemLabels = (labels) => {
  if (!labels) return {}
  
  const systemLabelPrefixes = [
    'kubernetes.io/',
    'beta.kubernetes.io/',
    'app.kubernetes.io/'
  ]
  
  const systemLabels = {}
  Object.entries(labels).forEach(([key, value]) => {
    if (systemLabelPrefixes.some(prefix => key.startsWith(prefix))) {
      systemLabels[key] = value
    }
  })
  
  return systemLabels
}

const toLabelTableData = (labels) =>
  Object.entries(labels || {}).map(([key, value]) => ({ key, value }))

// 查看命名空间标签
const viewNamespaceLabels = (row) => {
  viewNamespaceDetail(row)
}

// yaml编辑
const handleEditYaml = (row) => {
  currentNamespace.value = row
  openYamlDialog({
    clusterId: selectedClusterId.value,
    resourceType: 'Namespace',
    resourceName: row.name,
    fetchYaml: k8sApi.getNamespaceYaml,
    updateYaml: k8sApi.updateNamespaceYaml,
    fallbackYaml: `# Namespace ${row.name} YAML\napiVersion: v1\nkind: Namespace\nmetadata:\n  name: ${row.name}`,
    loadErrorMessage: '获取命名空间 YAML 失败',
    saveSuccessMessage: '命名空间 YAML 更新成功',
    saveErrorMessage: '更新命名空间 YAML 失败',
    onSaved: handleQuery
  })
}

// 修改标签
const handleManageLabels = (row) => {
  currentNamespace.value = row
  labelsDialogVisible.value = true
}

// 判断是否为系统命名空间
const isSystemNamespace = (namespaceName) => {
  const systemNamespaces = [
    'default',
    'kube-system', 
    'kube-public',
    'kube-node-lease'
  ]
  return systemNamespaces.includes(namespaceName)
}

// 获取注解数量
const getAnnotationCount = (annotations) => {
  if (!annotations || typeof annotations !== 'object') return 0
  return Object.keys(annotations).length
}

// 显示创建ResourceQuota对话框
const showCreateQuotaDialog = () => {
  quotaForm.name = ''
  quotaForm.cpuLimit = ''
  quotaForm.memoryLimit = ''
  quotaForm.storageLimit = ''
  quotaForm.podLimit = ''
  quotaForm.serviceLimit = ''
  quotaForm.isEditing = false
  quotaForm.originalName = ''
  createQuotaDialogVisible.value = true
}

// 显示创建LimitRange对话框
const showCreateLimitRangeDialog = () => {
  limitRangeForm.name = ''
  limitRangeForm.containerCpuMin = ''
  limitRangeForm.containerCpuMax = ''
  limitRangeForm.containerMemoryMin = ''
  limitRangeForm.containerMemoryMax = ''
  limitRangeForm.containerCpuDefault = ''
  limitRangeForm.containerMemoryDefault = ''
  limitRangeForm.isEditing = false
  limitRangeForm.originalName = ''
  createLimitRangeDialogVisible.value = true
}

// 提交创建ResourceQuota
const submitCreateQuota = async () => {
  try {
    if (!quotaForm.name) {
      ElMessage.warning('请输入配额名称')
      return
    }
    
    const data = {
      name: quotaForm.name,
      hard: {}
    }
    
    // 自动添加单位
    if (quotaForm.cpuLimit) {
      // CPU: 如果是纯数字，自动添加单位 (cores)，如果已有单位则保持
      const cpuValue = quotaForm.cpuLimit.trim()
      data.hard['limits.cpu'] = /^\d+$/.test(cpuValue) ? `${cpuValue}` : cpuValue
    }
    
    if (quotaForm.memoryLimit) {
      // 内存: 如果是纯数字，自动添加Gi单位
      const memoryValue = quotaForm.memoryLimit.trim()
      data.hard['limits.memory'] = /^\d+$/.test(memoryValue) ? `${memoryValue}Gi` : memoryValue
    }
    
    if (quotaForm.storageLimit) {
      // 存储: 如果是纯数字，自动添加Gi单位
      const storageValue = quotaForm.storageLimit.trim()
      data.hard['requests.storage'] = /^\d+$/.test(storageValue) ? `${storageValue}Gi` : storageValue
    }
    
    // 数量类型不需要单位
    if (quotaForm.podLimit) data.hard['count/pods'] = quotaForm.podLimit
    if (quotaForm.serviceLimit) data.hard['count/services'] = quotaForm.serviceLimit
    
    let response, responseData
    
    if (quotaForm.isEditing) {
      // 编辑模式：更新现有配额
      response = await k8sApi.updateResourceQuota(selectedClusterId.value, currentNamespace.value.name, quotaForm.originalName, data)
    } else {
      // 创建模式：创建新配额
      response = await k8sApi.createResourceQuota(selectedClusterId.value, currentNamespace.value.name, data)
    }
    
    responseData = response.data || response
    
    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(quotaForm.isEditing ? 'ResourceQuota更新成功' : 'ResourceQuota创建成功')
      createQuotaDialogVisible.value = false
      // 重新获取ResourceQuota列表
      manageResourceQuotas(currentNamespace.value)
    } else {
      ElMessage.error(responseData.message || (quotaForm.isEditing ? '更新ResourceQuota失败' : '创建ResourceQuota失败'))
    }
  } catch (error) {
    console.error((quotaForm.isEditing ? '更新' : '创建') + 'ResourceQuota失败:', error)
    ElMessage.error((quotaForm.isEditing ? '更新' : '创建') + 'ResourceQuota失败，请检查网络连接')
  }
}

// 提交创建LimitRange
const submitCreateLimitRange = async () => {
  try {
    if (!limitRangeForm.name) {
      ElMessage.warning('请输入限制名称')
      return
    }
    
    const data = {
      name: limitRangeForm.name,
      spec: {
        limits: [
          {
            type: 'Container',
            min: {},
            max: {},
            default: {},
            defaultRequest: {}
          }
        ]
      }
    }
    
    const limit = data.spec.limits[0]
    
    // 自动添加单位 - CPU
    if (limitRangeForm.containerCpuMin) {
      const value = limitRangeForm.containerCpuMin.trim()
      limit.min.cpu = /^\d+$/.test(value) ? `${value}m` : value // 纯数字默认为毫核心
    }
    if (limitRangeForm.containerCpuMax) {
      const value = limitRangeForm.containerCpuMax.trim()
      limit.max.cpu = /^\d+$/.test(value) ? `${value}` : value // 纯数字默认为核心
    }
    if (limitRangeForm.containerCpuDefault) {
      const value = limitRangeForm.containerCpuDefault.trim()
      limit.default.cpu = /^\d+$/.test(value) ? `${value}m` : value // 纯数字默认为毫核心
    }
    
    // 自动添加单位 - Memory
    if (limitRangeForm.containerMemoryMin) {
      const value = limitRangeForm.containerMemoryMin.trim()
      limit.min.memory = /^\d+$/.test(value) ? `${value}Mi` : value // 纯数字默认为Mi
    }
    if (limitRangeForm.containerMemoryMax) {
      const value = limitRangeForm.containerMemoryMax.trim()
      limit.max.memory = /^\d+$/.test(value) ? `${value}Gi` : value // 纯数字默认为Gi
    }
    if (limitRangeForm.containerMemoryDefault) {
      const value = limitRangeForm.containerMemoryDefault.trim()
      limit.default.memory = /^\d+$/.test(value) ? `${value}Mi` : value // 纯数字默认为Mi
    }
    
    let response, responseData
    
    if (limitRangeForm.isEditing) {
      // 编辑模式：更新现有限制
      response = await k8sApi.updateLimitRange(selectedClusterId.value, currentNamespace.value.name, limitRangeForm.originalName, data)
    } else {
      // 创建模式：创建新限制
      response = await k8sApi.createLimitRange(selectedClusterId.value, currentNamespace.value.name, data)
    }
    
    responseData = response.data || response
    
    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(limitRangeForm.isEditing ? 'LimitRange更新成功' : 'LimitRange创建成功')
      createLimitRangeDialogVisible.value = false
      // 重新获取LimitRange列表
      manageLimitRanges(currentNamespace.value)
    } else {
      ElMessage.error(responseData.message || (limitRangeForm.isEditing ? '更新LimitRange失败' : '创建LimitRange失败'))
    }
  } catch (error) {
    console.error((limitRangeForm.isEditing ? '更新' : '创建') + 'LimitRange失败:', error)
    ElMessage.error((limitRangeForm.isEditing ? '更新' : '创建') + 'LimitRange失败，请检查网络连接')
  }
}

// 格式化资源值（添加单位显示）
const formatResourceValue = (value) => {
  if (!value || value === '-') return '-'
  
  // 如果已经有单位，直接返回
  if (typeof value === 'string' && (value.includes('m') || value.includes('Gi') || value.includes('Mi') || value.includes('Ki'))) {
    return value
  }
  
  // 纯数字，尝试添加合适的单位
  if (/^\d+$/.test(value)) {
    // CPU: 纯数字认为是核心数
    return `${value} cores`
  }
  
  return value
}

// 格式化数量值（添加单位显示）
const formatCountValue = (value) => {
  if (!value || value === '-') return '-'
  return `${value} 个`
}

// 编辑ResourceQuota
const editResourceQuota = (quota) => {
  // 填充表单数据
  quotaForm.name = quota.name
  quotaForm.cpuLimit = quota.hard?.['limits.cpu']?.replace(/cores?/i, '').trim() || ''
  quotaForm.memoryLimit = quota.hard?.['limits.memory']?.replace(/Gi?/i, '').trim() || ''
  quotaForm.storageLimit = quota.hard?.['requests.storage']?.replace(/Gi?/i, '').trim() || ''
  quotaForm.podLimit = quota.hard?.['count/pods'] || ''
  quotaForm.serviceLimit = quota.hard?.['count/services'] || ''
  
  // 标记为编辑模式
  quotaForm.isEditing = true
  quotaForm.originalName = quota.name
  
  createQuotaDialogVisible.value = true
}

// 删除ResourceQuota
const deleteResourceQuota = async (quota) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除资源配额 "${quota.name}" 吗？删除后将无法恢复！`,
      '删除资源配额确认',
      {
        confirmButtonText: '确定删除',
        cancelButtonText: '取消',
        type: 'error',
      }
    )
    
    const response = await k8sApi.deleteResourceQuota(selectedClusterId.value, currentNamespace.value.name, quota.name)
    const responseData = response.data || response
    
    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(`资源配额 "${quota.name}" 删除成功`)
      // 重新获取ResourceQuota列表
      manageResourceQuotas(currentNamespace.value)
    } else {
      ElMessage.error(responseData.message || '删除资源配额失败')
    }
  } catch (error) {
    if (error === 'cancel') {
      ElMessage.info('已取消删除操作')
    } else {
      console.error('删除资源配额失败:', error)
      ElMessage.error('删除资源配额失败，请检查网络连接')
    }
  }
}

// 编辑LimitRange
const editLimitRange = (limitRange) => {
  // 填充表单数据
  limitRangeForm.name = limitRange.name
  const limit = limitRange.spec?.limits?.[0] || {}
  
  // 处理CPU值 - 移除单位后缀
  const extractCpuValue = (value) => {
    if (!value) return ''
    return value.toString().replace(/m$/i, '').replace(/cores?$/i, '').trim()
  }
  
  // 处理内存值 - 移除单位后缀
  const extractMemoryValue = (value) => {
    if (!value) return ''
    return value.toString().replace(/Mi$/i, '').replace(/Gi$/i, '').replace(/Ki$/i, '').trim()
  }
  
  limitRangeForm.containerCpuMin = extractCpuValue(limit.min?.cpu)
  limitRangeForm.containerCpuMax = extractCpuValue(limit.max?.cpu)
  limitRangeForm.containerMemoryMin = extractMemoryValue(limit.min?.memory)
  limitRangeForm.containerMemoryMax = extractMemoryValue(limit.max?.memory)
  limitRangeForm.containerCpuDefault = extractCpuValue(limit.default?.cpu)
  limitRangeForm.containerMemoryDefault = extractMemoryValue(limit.default?.memory)
  
  // 标记为编辑模式
  limitRangeForm.isEditing = true
  limitRangeForm.originalName = limitRange.name
  
  console.log('编辑LimitRange表单数据:', limitRangeForm)
  createLimitRangeDialogVisible.value = true
}

// 删除LimitRange
const deleteLimitRange = async (limitRange) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除资源限制 "${limitRange.name}" 吗？删除后将无法恢复！`,
      '删除资源限制确认',
      {
        confirmButtonText: '确定删除',
        cancelButtonText: '取消',
        type: 'error',
      }
    )
    
    const response = await k8sApi.deleteLimitRange(selectedClusterId.value, currentNamespace.value.name, limitRange.name)
    const responseData = response.data || response
    
    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(`资源限制 "${limitRange.name}" 删除成功`)
      // 重新获取LimitRange列表
      manageLimitRanges(currentNamespace.value)
    } else {
      ElMessage.error(responseData.message || '删除资源限制失败')
    }
  } catch (error) {
    if (error === 'cancel') {
      ElMessage.info('已取消删除操作')
    } else {
      console.error('删除资源限制失败:', error)
      ElMessage.error('删除资源限制失败，请检查网络连接')
    }
  }
}

onMounted(async () => {
  unsubscribeClusterSelection = subscribeStoredClusterId((clusterId, detail = {}) => {
    if (detail.source === CLUSTER_SELECTION_SOURCE || String(selectedClusterId.value || '') === String(clusterId || '')) {
      return
    }

    selectedClusterId.value = clusterId
    if (selectedClusterId.value) {
      queryParams.namespaceName = ''
      namespaceNameFilterKeyword.value = ''
      fetchNamespaceNameOptions()
      handleQuery()
    } else {
      namespaceNameOptions.value = []
      tableData.value = []
    }
  })

  await fetchClusterList()
  if (selectedClusterId.value) {
    await Promise.all([
      fetchNamespaceNameOptions(),
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
  <div class="k8s-namespace-management">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/k8s.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>K8s 命名空间管理</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>统一管理命名空间、标签、资源配额和容器默认限制。</p>
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
        <el-button
          type="success"
          :icon="Plus"
          class="primary-action"
          @click="showCreateNamespaceDialog"
        >
          创建命名空间
        </el-button>
      </div>
    </header>

    <section class="namespace-card page-card">
      <!-- 搜索表单 -->
      <div class="search-section">
        <el-form :inline="true" :model="queryParams" class="search-form">
          <el-form-item label="命名空间名称">
            <el-select
              v-model="queryParams.namespaceName"
              placeholder="请选择命名空间名称"
              clearable
              filterable
              :filter-method="handleNamespaceNameFilter"
              :loading="namespaceNameOptionsLoading"
              style="width: 200px"
              @visible-change="handleNamespaceNameVisibleChange"
            >
              <el-option
                v-for="item in filteredNamespaceNameOptions"
                :key="item.value"
                :label="item.label"
                :value="item.value"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="状态">
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

      <!-- 命名空间列表表格 -->
      <NamespacesTable
        :table-data="pagedTableData"
        :loading="loading"
        @viewNamespaceDetail="viewNamespaceDetail"
        @viewNamespaceLabels="viewNamespaceLabels"
        @manageResourceQuotas="manageResourceQuotas"
        @manageLimitRanges="manageLimitRanges"
        @deleteNamespace="deleteNamespace"
        @editYaml="handleEditYaml"
        @manageLabels="handleManageLabels"
      />

      <!-- 分页 -->
      <div class="pagination-wrapper">
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

    <!-- 创建命名空间对话框 -->
    <CreateNamespaceDialog
      v-model:visible="createNamespaceDialogVisible"
      :selected-cluster-id="selectedClusterId"
      @success="handleCreateNamespace"
      ref="createDialogRef"
    />

    <!-- 命名空间标签修改对话框 -->
    <NamespaceLabelsDialog
      v-model:visible="labelsDialogVisible"
      :cluster-id="selectedClusterId"
      :namespace-name="currentNamespace.name || ''"
      :current-labels="currentNamespace.labels || {}"
      @saved="handleQuery"
    />

    <!-- 命名空间详情对话框 -->
    <el-dialog
      v-model="namespaceDetailDialogVisible"
      :title="`命名空间详情 - ${currentNamespace.name || ''}`"
      width="800px"
      class="namespace-detail-dialog"
      modal-class="k8s-namespace-dialog-overlay"
    >
      <div class="namespace-detail-content" v-if="currentNamespace">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="名称">{{ currentNamespace.name }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="getStatusTag(currentNamespace.status)" size="small">
              {{ getStatusText(currentNamespace.status) }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="创建时间">{{ formatAge(currentNamespace.createdAt) }}</el-descriptions-item>
          <el-descriptions-item label="注解数量">{{ getAnnotationCount(currentNamespace.annotations) }}</el-descriptions-item>
        </el-descriptions>

        <div class="labels-view-content">
          <!-- 用户自定义标签 -->
          <div class="labels-section" v-if="Object.keys(getUserLabels(currentNamespace.labels)).length > 0">
            <h4 class="section-title">用户标签</h4>
            <div class="labels-table-wrapper">
              <el-table
                :data="toLabelTableData(getUserLabels(currentNamespace.labels))"
                border
                stripe
                size="small"
                max-height="300"
                class="labels-table"
              >
                <el-table-column prop="key" label="Key" width="250" show-overflow-tooltip />
                <el-table-column prop="value" label="Value" show-overflow-tooltip />
              </el-table>
            </div>
          </div>

          <!-- 系统标签 -->
          <div class="labels-section" v-if="Object.keys(getSystemLabels(currentNamespace.labels)).length > 0">
            <h4 class="section-title">系统标签</h4>
            <div class="labels-table-wrapper">
              <el-table
                :data="toLabelTableData(getSystemLabels(currentNamespace.labels))"
                border
                stripe
                size="small"
                max-height="300"
                class="labels-table"
              >
                <el-table-column prop="key" label="Key" width="250" show-overflow-tooltip />
                <el-table-column prop="value" label="Value" show-overflow-tooltip />
              </el-table>
            </div>
          </div>

          <!-- 没有标签的提示 -->
          <div v-if="!currentNamespace.labels || Object.keys(currentNamespace.labels).length === 0" class="no-labels">
            <el-empty description="该命名空间没有标签" :image-size="60" />
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- ResourceQuota管理对话框 -->
    <el-dialog
      v-model="resourceQuotaDialogVisible"
      :title="`资源配额管理 - ${currentNamespace.name || ''}`"
      width="900px"
      class="resource-quota-dialog"
      modal-class="k8s-namespace-dialog-overlay"
    >
      <div class="quota-content">
        <!-- ResourceQuota列表 -->
        <div class="quota-list-section">
          <div class="section-header">
            <h4>当前资源配额</h4>
            <el-button type="success" size="small" :icon="Plus" @click="showCreateQuotaDialog">添加配额</el-button>
          </div>
          
          <el-table :data="resourceQuotas" size="small" style="width: 100%">
            <el-table-column prop="name" label="配额名称" min-width="150" />
            <el-table-column label="CPU限制" min-width="100">
              <template #default="{ row }">
                <span>{{ formatResourceValue(row.hard?.['limits.cpu']) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="内存限制" min-width="100">
              <template #default="{ row }">
                <span>{{ formatResourceValue(row.hard?.['limits.memory']) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="Pod数量限制" min-width="120">
              <template #default="{ row }">
                <span>{{ formatCountValue(row.hard?.['count/pods']) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="150">
              <template #default="{ row }">
                <div class="quota-operation-buttons">
                  <el-button link size="small" class="operation-text edit" @click="editResourceQuota(row)">编辑</el-button>
                  <el-button link size="small" class="operation-text delete" @click="deleteResourceQuota(row)">删除</el-button>
                </div>
              </template>
            </el-table-column>
          </el-table>
          
          <div v-if="resourceQuotas.length === 0" class="empty-state">
            <el-empty description="暂无资源配额" :image-size="60" />
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- LimitRange管理对话框 -->
    <el-dialog
      v-model="limitRangeDialogVisible"
      :title="`资源限制管理 - ${currentNamespace.name || ''}`"
      width="900px"
      class="limit-range-dialog"
      modal-class="k8s-namespace-dialog-overlay"
    >
      <div class="limit-range-content">
        <!-- LimitRange列表 -->
        <div class="limit-range-list-section">
          <div class="section-header">
            <h4>当前资源限制</h4>
            <el-button type="success" size="small" :icon="Plus" @click="showCreateLimitRangeDialog">添加限制</el-button>
          </div>
          
          <el-table :data="limitRanges" size="small" style="width: 100%">
            <el-table-column prop="name" label="限制名称" min-width="150" />
            <el-table-column label="容器CPU最小值" min-width="120">
              <template #default="{ row }">
                <el-tag size="small" type="info">
                  {{ row.spec?.limits?.[0]?.min?.cpu || '-' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="容器CPU最大值" min-width="120">
              <template #default="{ row }">
                <el-tag size="small" type="warning">
                  {{ row.spec?.limits?.[0]?.max?.cpu || '-' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="容器内存最小值" min-width="130">
              <template #default="{ row }">
                <el-tag size="small" type="info">
                  {{ row.spec?.limits?.[0]?.min?.memory || '-' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="容器内存最大值" min-width="130">
              <template #default="{ row }">
                <el-tag size="small" type="warning">
                  {{ row.spec?.limits?.[0]?.max?.memory || '-' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="创建时间" min-width="120">
              <template #default="{ row }">
                <el-tag size="small">
                  {{ formatAge(row.createdAt) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="150">
              <template #default="{ row }">
                <div class="limit-range-operation-buttons">
                  <el-button link size="small" class="operation-text edit" @click="editLimitRange(row)">编辑</el-button>
                  <el-button link size="small" class="operation-text delete" @click="deleteLimitRange(row)">删除</el-button>
                </div>
              </template>
            </el-table-column>
          </el-table>
          
          <div v-if="limitRanges.length === 0" class="empty-state">
            <el-empty description="暂无资源限制" :image-size="60" />
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- 创建ResourceQuota对话框 -->
    <el-dialog
      v-model="createQuotaDialogVisible"
      :title="quotaForm.isEditing ? '编辑资源配额' : '创建资源配额'"
      width="600px"
      class="create-quota-dialog"
      modal-class="k8s-namespace-dialog-overlay"
    >
      <el-form :model="quotaForm" label-width="120px" class="namespace-dialog-form">
        <el-form-item label="配额名称" required>
          <el-input
            v-model="quotaForm.name"
            placeholder="请输入配额名称，如：compute-quota"
          />
        </el-form-item>
        <el-form-item label="CPU限制">
          <el-input
            v-model="quotaForm.cpuLimit"
            placeholder="如：4（自动添加核心单位）或 4000m（毫核心）"
          >
            <template #append>cores</template>
          </el-input>
        </el-form-item>
        <el-form-item label="内存限制">
          <el-input
            v-model="quotaForm.memoryLimit"
            placeholder="如：8（自动添加Gi单位）或 8192Mi"
          >
            <template #append>Gi</template>
          </el-input>
        </el-form-item>
        <el-form-item label="存储限制">
          <el-input
            v-model="quotaForm.storageLimit"
            placeholder="如：100（自动添加Gi单位）或 100Gi"
          >
            <template #append>Gi</template>
          </el-input>
        </el-form-item>
        <el-form-item label="Pod数量限制">
          <el-input
            v-model="quotaForm.podLimit"
            placeholder="如：50（最多50个Pod）"
          />
        </el-form-item>
        <el-form-item label="Service数量限制">
          <el-input
            v-model="quotaForm.serviceLimit"
            placeholder="如：20（最多20个Service）"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="createQuotaDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitCreateQuota">{{ quotaForm.isEditing ? '更新' : '创建' }}</el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 创建LimitRange对话框 -->
    <el-dialog
      v-model="createLimitRangeDialogVisible"
      :title="limitRangeForm.isEditing ? '编辑资源限制' : '创建资源限制'"
      width="700px"
      class="create-limit-range-dialog"
      modal-class="k8s-namespace-dialog-overlay"
    >
      <el-form :model="limitRangeForm" label-width="150px" class="namespace-dialog-form">
        <el-form-item label="限制名称" required>
          <el-input
            v-model="limitRangeForm.name"
            placeholder="请输入限制名称，如：container-limits"
          />
        </el-form-item>
        <el-divider>容器资源限制</el-divider>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="CPU最小值">
              <el-input
                v-model="limitRangeForm.containerCpuMin"
                placeholder="如：100（自动添加m单位）"
              >
                <template #append>m</template>
              </el-input>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="CPU最大值">
              <el-input
                v-model="limitRangeForm.containerCpuMax"
                placeholder="如：2（核心数）"
              >
                <template #append>cores</template>
              </el-input>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="内存最小值">
              <el-input
                v-model="limitRangeForm.containerMemoryMin"
                placeholder="如：128（自动添加Mi单位）"
              >
                <template #append>Mi</template>
              </el-input>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="内存最大值">
              <el-input
                v-model="limitRangeForm.containerMemoryMax"
                placeholder="如：4（自动添加Gi单位）"
              >
                <template #append>Gi</template>
              </el-input>
            </el-form-item>
          </el-col>
        </el-row>
        <el-divider>默认资源配置</el-divider>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="默认CPU">
              <el-input
                v-model="limitRangeForm.containerCpuDefault"
                placeholder="如：500（自动添加m单位）"
              >
                <template #append>m</template>
              </el-input>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="默认内存">
              <el-input
                v-model="limitRangeForm.containerMemoryDefault"
                placeholder="如：1（自动添加Mi单位）"
              >
                <template #append>Mi</template>
              </el-input>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="createLimitRangeDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitCreateLimitRange">{{ limitRangeForm.isEditing ? '更新' : '创建' }}</el-button>
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

export { getProgressColor }
</script>

<style scoped>
.k8s-namespace-management {
  padding: 5px;
  min-height: 100vh;
  background: #eef1f6;
}

.namespace-card {
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

/* 对话框样式 */
.create-namespace-dialog :deep(.el-dialog),
.namespace-detail-dialog :deep(.el-dialog),
.resource-quota-dialog :deep(.el-dialog),
.limit-range-dialog :deep(.el-dialog),
.create-quota-dialog :deep(.el-dialog),
.create-limit-range-dialog :deep(.el-dialog) {
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
}

.create-namespace-dialog :deep(.el-dialog__header),
.namespace-detail-dialog :deep(.el-dialog__header),
.resource-quota-dialog :deep(.el-dialog__header),
.limit-range-dialog :deep(.el-dialog__header),
.create-quota-dialog :deep(.el-dialog__header),
.create-limit-range-dialog :deep(.el-dialog__header) {
  background: linear-gradient(135deg, #667eea, #764ba2);
  color: white;
  border-top-left-radius: 16px;
  border-top-right-radius: 16px;
  padding: 20px 24px;
}

.create-namespace-dialog :deep(.el-dialog__title),
.create-namespace-dialog :deep(.el-dialog__title),
.namespace-detail-dialog :deep(.el-dialog__title),
.resource-quota-dialog :deep(.el-dialog__title),
.limit-range-dialog :deep(.el-dialog__title),
.create-quota-dialog :deep(.el-dialog__title),
.create-limit-range-dialog :deep(.el-dialog__title) {
  color: white;
  font-weight: 600;
}

.create-namespace-dialog :deep(.el-dialog__body),
.create-namespace-dialog :deep(.el-dialog__body),
.namespace-detail-dialog :deep(.el-dialog__body),
.resource-quota-dialog :deep(.el-dialog__body),
.limit-range-dialog :deep(.el-dialog__body),
.create-quota-dialog :deep(.el-dialog__body),
.create-limit-range-dialog :deep(.el-dialog__body) {
  padding: 24px;
}

.namespace-detail-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.labels-view-content {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.labels-section {
  margin-top: 0;
}

.labels-section h4,
.section-title {
  margin: 0 0 16px 0;
  color: #6b9bd1;
  font-weight: 600;
  font-size: 16px;
  padding-bottom: 8px;
  border-bottom: 2px solid #e4e7ed;
}

.labels-table-wrapper {
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid #dcdfe6;
}

.labels-table {
  width: 100%;
}

.labels-table :deep(.el-table__header-wrapper) {
  background: rgba(103, 126, 234, 0.05);
}

.labels-table :deep(.el-table__header th) {
  background: rgba(103, 126, 234, 0.1) !important;
  color: #6b9bd1;
  font-weight: 600;
}

.labels-table :deep(.el-table__body tr:hover > td) {
  background: rgba(103, 126, 234, 0.08) !important;
}

.labels-table :deep(.el-table td) {
  padding: 10px 12px;
  word-break: break-all;
}

.labels-table :deep(.el-table__body-wrapper) {
  max-height: 300px;
  overflow-y: auto;
}

.no-labels {
  text-align: center;
  padding: 40px 20px;
  color: #909399;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.section-header h4 {
  margin: 0;
  color: #6b9bd1;
  font-weight: 600;
  font-size: 16px;
}

.empty-state {
  text-align: center;
  padding: 40px 20px;
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

.header-actions .el-select {
  min-width: 250px;
}

.header-actions .el-select .el-input__wrapper {
  background: rgba(255, 255, 255, 0.9);
  border: 1px solid rgba(103, 126, 234, 0.3);
}

.header-actions .el-select .el-input__wrapper.is-focus {
  border-color: #667eea;
  box-shadow: 0 0 0 2px rgba(103, 126, 234, 0.2);
}

/* 通用元素样式 */
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

/* ResourceQuota和LimitRange操作按钮样式 */
.quota-operation-buttons,
.limit-range-operation-buttons {
  display: flex;
  gap: 8px;
  justify-content: center;
  align-items: center;
  flex-wrap: nowrap;
  white-space: nowrap;
}

.quota-operation-buttons .el-button,
.limit-range-operation-buttons .el-button {
  transition: all 0.3s ease;
  flex-shrink: 0;
  min-width: auto;
}

.quota-operation-buttons .el-button:hover,
.limit-range-operation-buttons .el-button:hover {
  transform: translateY(-1px);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
}

/* 表单提示样式 */
.form-hint {
  margin-top: 4px;
  padding: 4px 8px;
  background: rgba(103, 126, 234, 0.05);
  border-radius: 4px;
  border-left: 3px solid rgba(103, 126, 234, 0.3);
}

.form-hint .el-text {
  line-height: 1.4;
}

.create-namespace-dialog :deep(.el-divider__text) {
  background: linear-gradient(45deg, #667eea, #764ba2);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  font-weight: 600;
}

.create-namespace-dialog :deep(.el-divider) {
  margin: 20px 0 16px 0;
}

.create-namespace-dialog :deep(.el-textarea__inner) {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 13px;
  line-height: 1.5;
}

/* 响应式设计 */
@media (max-width: 1200px) {
  .operation-buttons {
    gap: 4px;
  }
  
  .operation-buttons .el-button {
    margin: 1px;
  }
  
  .quota-operation-buttons,
  .limit-range-operation-buttons {
    gap: 4px;
  }
  
  .quota-operation-buttons .el-button,
  .limit-range-operation-buttons .el-button {
    font-size: 12px;
    padding: 4px 8px;
  }
  
  .header-actions .el-select {
    min-width: 200px;
  }
}

@media (max-width: 768px) {
  .k8s-namespace-management {
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
  
  .namespace-table :deep(.el-table__row:hover) {
    transform: none;
  }
}

.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
  padding: 12px 0;
}

/* Unified namespace page skin */
.k8s-namespace-management {
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
  --namespace-page-height-offset: 190px;
  min-height: calc(100vh - var(--namespace-page-height-offset));
  padding: 6px;
  box-sizing: border-box;
  background:
    radial-gradient(circle at 16% 10%, rgba(76, 139, 245, 0.06), transparent 26%),
    linear-gradient(180deg, #f7f9fc 0%, var(--aiops-bg-page) 100%);
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.namespace-card {
  position: relative;
  display: flex;
  flex-direction: column;
  height: calc(100vh - 12px - var(--namespace-page-height-offset));
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

.namespace-card::before {
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
  min-width: 116px;
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

.pagination-wrapper {
  flex-shrink: 0;
  margin-top: 10px;
  padding: 8px 4px 0;
  border-top: 1px solid rgba(223, 231, 242, 0.72);
}

.cluster-option {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.cluster-name {
  color: var(--aiops-brand);
  font-weight: 500;
}

.cluster-status-tag {
  margin-left: 8px;
}

.namespace-detail-dialog :deep(.el-dialog),
.resource-quota-dialog :deep(.el-dialog),
.limit-range-dialog :deep(.el-dialog),
.create-quota-dialog :deep(.el-dialog),
.create-limit-range-dialog :deep(.el-dialog) {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 24px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
  box-shadow: 0 36px 110px rgba(15, 23, 42, 0.11);
  backdrop-filter: blur(18px);
}

.namespace-detail-dialog :deep(.el-dialog__header),
.resource-quota-dialog :deep(.el-dialog__header),
.limit-range-dialog :deep(.el-dialog__header),
.create-quota-dialog :deep(.el-dialog__header),
.create-limit-range-dialog :deep(.el-dialog__header) {
  padding: 16px 20px 14px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.84);
}

.namespace-detail-dialog :deep(.el-dialog__title),
.resource-quota-dialog :deep(.el-dialog__title),
.limit-range-dialog :deep(.el-dialog__title),
.create-quota-dialog :deep(.el-dialog__title),
.create-limit-range-dialog :deep(.el-dialog__title) {
  color: var(--aiops-text-primary);
  font-size: 17px;
  font-weight: 700;
}

.namespace-detail-dialog :deep(.el-dialog__body),
.resource-quota-dialog :deep(.el-dialog__body),
.limit-range-dialog :deep(.el-dialog__body),
.create-quota-dialog :deep(.el-dialog__body),
.create-limit-range-dialog :deep(.el-dialog__body) {
  padding: 18px 20px;
  background:
    radial-gradient(circle at top left, rgba(76, 139, 245, 0.04), transparent 22%),
    linear-gradient(180deg, #f8fafc 0%, #f5f8fc 100%);
}

.create-namespace-dialog :deep(.el-dialog__footer),
.namespace-detail-dialog :deep(.el-dialog__footer),
.resource-quota-dialog :deep(.el-dialog__footer),
.limit-range-dialog :deep(.el-dialog__footer),
.create-quota-dialog :deep(.el-dialog__footer),
.create-limit-range-dialog :deep(.el-dialog__footer) {
  padding: 10px 20px 16px;
  background: rgba(247, 250, 255, 0.72);
  border-top: 1px solid rgba(223, 231, 242, 0.76);
}

.namespace-detail-content,
.labels-view-content,
.quota-content,
.limit-range-content {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.section-header h4,
.labels-section h4,
.section-title {
  margin: 0 0 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.9);
  color: var(--aiops-text-primary);
  font-size: 16px;
  font-weight: 700;
}

.section-header h4 {
  margin-bottom: 0;
  padding-bottom: 0;
  border-bottom: none;
}

.labels-table-wrapper,
.quota-list-section,
.limit-range-list-section {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.86);
}

.quota-list-section,
.limit-range-list-section {
  padding: 14px;
}

.labels-table :deep(.el-table__header th),
.resource-quota-dialog :deep(.el-table__header th),
.limit-range-dialog :deep(.el-table__header th) {
  background: rgba(248, 250, 255, 0.98) !important;
  color: var(--aiops-text-muted);
  font-weight: 700;
}

.labels-table :deep(.el-table__body tr:hover > td),
.resource-quota-dialog :deep(.el-table__body tr:hover > td),
.limit-range-dialog :deep(.el-table__body tr:hover > td) {
  background: rgba(76, 139, 245, 0.045) !important;
}

.resource-quota-dialog :deep(.el-table),
.limit-range-dialog :deep(.el-table) {
  border: 1px solid rgba(223, 231, 242, 0.88);
  border-radius: 14px;
  overflow: hidden;
}

.quota-operation-buttons,
.limit-range-operation-buttons {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 16px;
  white-space: nowrap;
}

.quota-operation-buttons :deep(.el-button),
.limit-range-operation-buttons :deep(.el-button) {
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

.quota-operation-buttons :deep(.el-button:hover:not(.is-disabled)),
.limit-range-operation-buttons :deep(.el-button:hover:not(.is-disabled)) {
  transform: none;
  box-shadow: none;
  text-decoration: none;
  opacity: 0.82;
}

.quota-operation-buttons :deep(.operation-text.edit),
.limit-range-operation-buttons :deep(.operation-text.edit) {
  color: #4c8bf5;
}

.quota-operation-buttons :deep(.operation-text.delete),
.limit-range-operation-buttons :deep(.operation-text.delete) {
  color: #ef6b73;
}

.no-labels,
.empty-state {
  padding: 28px 16px;
  color: var(--aiops-text-muted);
  text-align: center;
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
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.k8s-namespace-management .el-button:hover {
  transform: translateY(-1px);
}

.quota-operation-buttons :deep(.operation-text.el-button.el-button),
.limit-range-operation-buttons :deep(.operation-text.el-button.el-button) {
  padding: 0;
  border: none;
  background: transparent;
  box-shadow: none;
}

.quota-operation-buttons :deep(.operation-text.el-button.el-button:hover:not(.is-disabled)),
.limit-range-operation-buttons :deep(.operation-text.el-button.el-button:hover:not(.is-disabled)) {
  transform: none;
  box-shadow: none;
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

.el-textarea :deep(.el-textarea__inner) {
  min-height: 82px;
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 13px;
  background: rgba(252, 253, 255, 0.98);
  box-shadow: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.el-textarea :deep(.el-textarea__inner):focus {
  border-color: rgba(120, 156, 221, 0.9);
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
}

.create-quota-dialog :deep(.el-form-item__label),
.create-limit-range-dialog :deep(.el-form-item__label) {
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

.create-quota-dialog :deep(.el-divider__text),
.create-limit-range-dialog :deep(.el-divider__text) {
  color: var(--aiops-text-primary);
  font-weight: 700;
}

.create-quota-dialog :deep(.el-divider),
.create-limit-range-dialog :deep(.el-divider) {
  margin: 18px 0;
  border-top-color: rgba(223, 231, 242, 0.92);
}

.el-loading-mask {
  background-color: rgba(247, 250, 255, 0.68);
  backdrop-filter: blur(4px);
}

@media (max-width: 768px) {
  .k8s-namespace-management {
    --namespace-page-height-offset: 131px;
    padding: 4px;
  }

  .namespace-card {
    height: calc(100vh - 8px - var(--namespace-page-height-offset));
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

  .section-header {
    align-items: stretch;
    flex-direction: column;
  }
}

/* DESIGN.md compact workbench overrides */
.k8s-namespace-management {
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

.namespace-card {
  height: auto;
  padding: 10px;
}

.namespace-card::before {
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

.primary-action,
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

.primary-action {
  min-width: 116px;
}

.k8s-namespace-management .el-button:hover,
.primary-action:hover,
.primary-action:focus,
.query-button:hover,
.query-button:focus,
.reset-button:hover,
.reset-button:focus {
  transform: none;
}

.query-button:hover,
.query-button:focus,
.reset-button:hover,
.reset-button:focus {
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

.k8s-namespace-management :deep(.el-input__wrapper),
.k8s-namespace-management :deep(.el-select__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.k8s-namespace-management :deep(.el-input__wrapper:hover),
.k8s-namespace-management :deep(.el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.k8s-namespace-management :deep(.el-input__wrapper.is-focus),
.k8s-namespace-management :deep(.el-select__wrapper.is-focused) {
  box-shadow: 0 0 0 1px #409eff inset;
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

.namespace-detail-content,
.labels-view-content,
.quota-content,
.limit-range-content {
  gap: 12px;
}

.labels-table-wrapper,
.quota-list-section,
.limit-range-list-section {
  border: 1px solid #e3ebf5;
  border-radius: 10px;
  background: #fff;
}

.quota-list-section,
.limit-range-list-section {
  padding: 10px;
}

.section-header {
  margin-bottom: 8px;
}

.section-header h4,
.labels-section h4,
.section-title {
  font-size: 14px;
  font-weight: 700;
}

.labels-section h4,
.section-title {
  margin-bottom: 8px;
  padding-bottom: 6px;
}

:global(.k8s-namespace-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 24px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: 15px !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.k8s-namespace-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: linear-gradient(90deg, #fff6e7 0%, #f7fbff 36%, #ecf7ff 100%);
}

:global(.k8s-namespace-dialog-overlay .el-dialog__title) {
  color: #172033;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.k8s-namespace-dialog-overlay .el-dialog__headerbtn) {
  top: 12px;
  right: 14px;
  width: 32px;
  height: 32px;
}

:global(.k8s-namespace-dialog-overlay .el-dialog__body) {
  max-height: calc(100vh - 190px);
  overflow: auto;
  padding: 14px 18px;
  background: #fff;
}

:global(.k8s-namespace-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.k8s-namespace-dialog-overlay .el-form-item) {
  margin-bottom: 14px;
}

:global(.k8s-namespace-dialog-overlay .el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

:global(.k8s-namespace-dialog-overlay .el-input__wrapper),
:global(.k8s-namespace-dialog-overlay .el-select__wrapper),
:global(.k8s-namespace-dialog-overlay .el-textarea__inner) {
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.k8s-namespace-dialog-overlay .el-input__wrapper),
:global(.k8s-namespace-dialog-overlay .el-select__wrapper) {
  min-height: 34px;
}

:global(.k8s-namespace-dialog-overlay .el-input__wrapper:hover),
:global(.k8s-namespace-dialog-overlay .el-select__wrapper:hover),
:global(.k8s-namespace-dialog-overlay .el-textarea__inner:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

:global(.k8s-namespace-dialog-overlay .el-input__wrapper.is-focus),
:global(.k8s-namespace-dialog-overlay .el-select__wrapper.is-focused),
:global(.k8s-namespace-dialog-overlay .el-textarea__inner:focus) {
  box-shadow: 0 0 0 1px #409eff inset;
}

:global(.k8s-namespace-dialog-overlay .el-button) {
  min-height: 34px;
  border-radius: 8px;
  font-weight: 600;
  transform: none !important;
}

:global(.k8s-namespace-dialog-overlay .el-table) {
  overflow: hidden;
  border: 1px solid #e3ebf5;
  border-radius: 9px;
}

:global(.k8s-namespace-dialog-overlay .el-table th.el-table__cell) {
  background: #f3f7fc !important;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

:global(.k8s-namespace-dialog-overlay .el-table td.el-table__cell) {
  padding: 7px 0;
  border-bottom-color: #eef3f9;
}

:global(.k8s-namespace-dialog-overlay .el-descriptions) {
  overflow: hidden;
  border-radius: 9px;
}

@media (max-width: 768px) {
  .k8s-namespace-management {
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

  .primary-action {
    width: auto;
  }

  .page-card,
  .namespace-card {
    padding: 8px;
    border-radius: 12px;
  }

  .search-form :deep(.el-form-item) {
    margin-bottom: 8px;
  }
}
</style>
