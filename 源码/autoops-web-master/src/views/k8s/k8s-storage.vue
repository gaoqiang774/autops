<script setup>
import { ref, reactive, onMounted, computed, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Plus,
  Refresh,
  Connection,
  DataLine,
  Cpu,
  Search
} from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'
import PodYamlDialog from './pods/PodYamlDialog.vue'
import ClusterSelector from './pods/ClusterSelector.vue'
import NamespaceSelector from './pods/NamespaceSelector.vue'
import { getYamlContentFromApiResponse } from './utils/yaml'

// 基础状态
const loading = ref(false)
const activeTab = ref('pvc')
const searchKeyword = ref('')

// 集群和命名空间状态
const selectedClusterId = ref('')
const queryParams = reactive({
  namespace: 'default'
})

// 存储资源数据状态
const pvcList = ref([])
const pvList = ref([])
const storageClassList = ref([])

// 对话框状态
const pvcYamlDialogVisible = ref(false)
const pvYamlDialogVisible = ref(false)
const storageClassYamlDialogVisible = ref(false)

// 详情对话框状态
const pvcDetailDialogVisible = ref(false)
const pvDetailDialogVisible = ref(false)
const storageClassDetailDialogVisible = ref(false)

// 当前操作的资源
const currentPVCYaml = ref('')
const currentPVYaml = ref('')
const currentStorageClassYaml = ref('')
const currentResourceName = ref('')
const currentResourceType = ref('')

// 当前查看的资源详情
const currentPVCForDetail = ref({})
const currentPVForDetail = ref({})
const currentStorageClassForDetail = ref({})

// 过滤后的列表
const filteredPVCList = computed(() => {
  const list = Array.isArray(pvcList.value) ? pvcList.value : []
  if (!searchKeyword.value) return list
  return list.filter(item =>
    item.name?.toLowerCase().includes(searchKeyword.value.toLowerCase()) ||
    item.status?.toLowerCase().includes(searchKeyword.value.toLowerCase()) ||
    item.storageClassName?.toLowerCase().includes(searchKeyword.value.toLowerCase())
  )
})

const filteredPVList = computed(() => {
  const list = Array.isArray(pvList.value) ? pvList.value : []
  if (!searchKeyword.value) return list
  return list.filter(item =>
    item.name?.toLowerCase().includes(searchKeyword.value.toLowerCase()) ||
    item.status?.toLowerCase().includes(searchKeyword.value.toLowerCase()) ||
    item.storageClassName?.toLowerCase().includes(searchKeyword.value.toLowerCase())
  )
})

const filteredStorageClassList = computed(() => {
  const list = Array.isArray(storageClassList.value) ? storageClassList.value : []
  if (!searchKeyword.value) return list
  return list.filter(item =>
    item.name?.toLowerCase().includes(searchKeyword.value.toLowerCase()) ||
    item.provisioner?.toLowerCase().includes(searchKeyword.value.toLowerCase())
  )
})

const currentResourceCountText = computed(() => {
  if (activeTab.value === 'pv') {
    return `共 ${filteredPVList.value.length} 个 PV`
  }

  if (activeTab.value === 'storageclass') {
    return `共 ${filteredStorageClassList.value.length} 个 StorageClass`
  }

  return `共 ${filteredPVCList.value.length} 个 PVC`
})

// 处理集群选择变化
const handleClusterChange = (clusterId) => {
  selectedClusterId.value = clusterId
  console.log('集群选择变化:', clusterId)
  if (clusterId && queryParams.namespace) {
    loadAllStorageResources()
  }
}

// 处理命名空间选择变化
const handleNamespaceChange = (namespace) => {
  queryParams.namespace = namespace
  console.log('命名空间选择变化:', namespace)
  if (selectedClusterId.value && namespace) {
    loadAllStorageResources()
  }
}

// 获取命名空间列表
const fetchNamespaceList = async () => {
  if (!selectedClusterId.value) return

  try {
    const response = await k8sApi.getNamespaceList(selectedClusterId.value)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      namespaceList.value = responseData.data || []
      // 如果当前选中的命名空间不在新列表中，重置为default
      if (!namespaceList.value.some(ns => ns.name === queryParams.namespace)) {
        queryParams.namespace = namespaceList.value.length > 0 ? namespaceList.value[0].name : 'default'
      }
    } else {
      ElMessage.error(responseData.message || '获取命名空间列表失败')
    }
  } catch (error) {
    console.error('获取命名空间列表失败:', error)
    ElMessage.error('获取命名空间列表失败')
  }
}

// 加载所有存储资源
const loadAllStorageResources = async () => {
  if (!selectedClusterId.value) {
    console.warn('集群ID为空，无法加载存储资源')
    return
  }

  console.log('开始加载存储资源，集群ID:', selectedClusterId.value, '命名空间:', queryParams.namespace)

  loading.value = true
  try {
    // 并发加载所有存储资源
    await Promise.all([
      fetchPVCList(),
      fetchPVList(),
      fetchStorageClassList()
    ])
  } catch (error) {
    console.error('加载存储资源失败:', error)
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  searchKeyword.value = searchKeyword.value.trim()
}


// 标签页切换处理
const handleTabChange = (tabName) => {
  console.log('标签页切换到:', tabName)
  activeTab.value = tabName
}

// 获取PVC列表
const fetchPVCList = async () => {
  if (!selectedClusterId.value || !queryParams.namespace) {
    console.warn('集群ID或命名空间为空，无法获取 PVC 列表')
    return
  }

  try {
    console.log('正在获取 PVC 列表，集群ID:', selectedClusterId.value, '命名空间:', queryParams.namespace)

    const response = await k8sApi.getPVCList(selectedClusterId.value, queryParams.namespace)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      const rawData = responseData.data?.pvcs || responseData.data
      pvcList.value = Array.isArray(rawData) ? rawData : []
      console.log('获取到 PVC 列表:', pvcList.value.length, '个')
    } else {
      ElMessage.error(responseData.message || '获取 PVC 列表失败')
      pvcList.value = []
    }
  } catch (error) {
    console.error('获取 PVC 列表失败:', error)
    ElMessage.error('获取 PVC 列表失败，请检查网络连接')
    pvcList.value = []
  }
}

// 获取PV列表
const fetchPVList = async () => {
  if (!selectedClusterId.value) {
    console.warn('集群ID为空，无法获取 PV 列表')
    return
  }

  try {
    console.log('正在获取 PV 列表，集群ID:', selectedClusterId.value)

    const response = await k8sApi.getPVList(selectedClusterId.value)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      const rawData = responseData.data?.pvs || responseData.data
      pvList.value = Array.isArray(rawData) ? rawData : []
      console.log('获取到 PV 列表:', pvList.value.length, '个')
    } else {
      ElMessage.error(responseData.message || '获取 PV 列表失败')
      pvList.value = []
    }
  } catch (error) {
    console.error('获取 PV 列表失败:', error)
    ElMessage.error('获取 PV 列表失败，请检查网络连接')
    pvList.value = []
  }
}

// 获取StorageClass列表
const fetchStorageClassList = async () => {
  if (!selectedClusterId.value) {
    console.warn('集群ID为空，无法获取 StorageClass 列表')
    return
  }

  try {
    console.log('正在获取 StorageClass 列表，集群ID:', selectedClusterId.value)

    const response = await k8sApi.getStorageClassList(selectedClusterId.value)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      const rawData = responseData.data?.storageClasses || responseData.data
      storageClassList.value = Array.isArray(rawData) ? rawData : []
      console.log('获取到 StorageClass 列表:', storageClassList.value.length, '个')
    } else {
      ElMessage.error(responseData.message || '获取 StorageClass 列表失败')
      storageClassList.value = []
    }
  } catch (error) {
    console.error('获取 StorageClass 列表失败:', error)
    ElMessage.error('获取 StorageClass 列表失败，请检查网络连接')
    storageClassList.value = []
  }
}

// 刷新数据
const handleRefresh = () => {
  loadAllStorageResources()
}

const formatDateTime = (value) => {
  if (!value) return '-'

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  return date.toLocaleString()
}

// PVC 操作
const handleCreatePVC = () => {
  if (!selectedClusterId.value) {
    ElMessage.warning('请先选择集群')
    return
  }
  if (!queryParams.namespace) {
    ElMessage.warning('请先选择命名空间')
    return
  }

  // 设置默认的PVC YAML模板
  const defaultYaml = `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: new-pvc
  namespace: ${queryParams.namespace}
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
  storageClassName: standard`

  currentPVCYaml.value = defaultYaml
  currentPVCForDetail.value = { name: 'new-pvc', namespace: queryParams.namespace }
  pvcYamlDialogVisible.value = true
}

// 查看 PVC 详情
const handleViewPVC = async (row) => {
  try {
    console.log('🔍 获取PVC详情:', {
      clusterId: selectedClusterId.value,
      namespace: queryParams.namespace,
      pvcName: row?.name,
      row: row
    })

    const response = await k8sApi.getPVCDetail(selectedClusterId.value, queryParams.namespace, row?.name)
    const responseData = response.data || response

    console.log('📥 PVC详情API响应:', responseData)

    if (responseData.code === 200 || responseData.success) {
      currentPVCForDetail.value = responseData.data || row
      console.log('✅ PVC详情数据已设置，打开对话框')
      pvcDetailDialogVisible.value = true
    } else {
      console.error('❌ API返回失败:', responseData)
      ElMessage.error(responseData.message || '获取 PVC 详情失败')
    }
  } catch (error) {
    console.error('❌ 获取 PVC 详情失败:', error)
    ElMessage.error('获取 PVC 详情失败: ' + (error.message || '未知错误'))
  }
}

// 编辑 PVC YAML
const handleEditPVCYaml = async (row) => {
  try {
    loading.value = true
    const response = await k8sApi.getPVCYaml(selectedClusterId.value, queryParams.namespace, row?.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      currentPVCYaml.value = getYamlContentFromApiResponse(
        responseData,
        `# PVC ${row?.name} YAML\napiVersion: v1\nkind: PersistentVolumeClaim\nmetadata:\n  name: ${row?.name}`
      )
      currentPVCForDetail.value = row
      pvcYamlDialogVisible.value = true
    } else {
      ElMessage.error(responseData.message || '获取 PVC YAML 失败')
    }
  } catch (error) {
    console.error('获取 PVC YAML 失败:', error)
    ElMessage.error('获取 PVC YAML 失败')
  } finally {
    loading.value = false
  }
}

// 保存 PVC YAML
const handlePVCYamlSave = async (data) => {
  try {
    // 检查是否是创建新PVC (new-pvc表示新建)
    const isCreating = data.resourceName === 'new-pvc' || !currentPVCForDetail.value?.creationTimestamp

    let response
    if (isCreating) {
      // 创建新PVC，使用createPodFromYaml API（支持多种资源类型）
      response = await k8sApi.createPodFromYaml(selectedClusterId.value, queryParams.namespace, { yamlContent: data.yamlContent })
    } else {
      // 更新现有PVC
      response = await k8sApi.updatePVCYaml(selectedClusterId.value, queryParams.namespace, data.resourceName, data.yamlContent)
    }

    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(isCreating ? 'PVC 创建成功' : 'PVC YAML 更新成功')
      pvcYamlDialogVisible.value = false
      fetchPVCList() // 刷新列表
    } else {
      ElMessage.error(responseData.message || (isCreating ? 'PVC 创建失败' : 'PVC YAML 更新失败'))
    }
  } catch (error) {
    console.error('PVC 操作失败:', error)
    ElMessage.error('PVC 操作失败')
  }
}

// 删除 PVC
const handleDeletePVC = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确认删除 PVC "${row?.name}"？删除后数据将无法恢复。`,
      '删除确认',
      {
        confirmButtonText: '确认删除',
        cancelButtonText: '取消',
        type: 'warning',
        confirmButtonClass: 'el-button--danger'
      }
    )

    const response = await k8sApi.deletePVC(selectedClusterId.value, queryParams.namespace, row?.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success('PVC 删除成功')
      fetchPVCList()
    } else {
      ElMessage.error(responseData.message || 'PVC 删除失败')
    }
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除 PVC 失败:', error)
      ElMessage.error('删除 PVC 失败')
    }
  }
}

// PV 操作
const handleCreatePV = () => {
  if (!selectedClusterId.value) {
    ElMessage.warning('请先选择集群')
    return
  }

  // 设置默认的PV YAML模板
  const defaultYaml = `apiVersion: v1
kind: PersistentVolume
metadata:
  name: new-pv
spec:
  capacity:
    storage: 20Gi
  accessModes:
    - ReadWriteOnce
  persistentVolumeReclaimPolicy: Retain
  storageClassName: standard
  hostPath:
    path: /data/storage`

  currentPVYaml.value = defaultYaml
  currentPVForDetail.value = { name: 'new-pv' }
  pvYamlDialogVisible.value = true
}

// 查看 PV 详情
const handleViewPV = async (row) => {
  try {
    console.log('🔍 获取PV详情:', {
      clusterId: selectedClusterId.value,
      pvName: row?.name,
      row: row
    })

    const response = await k8sApi.getPVDetail(selectedClusterId.value, row?.name)
    const responseData = response.data || response

    console.log('📥 PV详情API响应:', responseData)

    if (responseData.code === 200 || responseData.success) {
      currentPVForDetail.value = responseData.data || row
      console.log('✅ PV详情数据已设置，打开对话框')
      pvDetailDialogVisible.value = true
    } else {
      console.error('❌ API返回失败:', responseData)
      ElMessage.error(responseData.message || '获取 PV 详情失败')
    }
  } catch (error) {
    console.error('❌ 获取 PV 详情失败:', error)
    ElMessage.error('获取 PV 详情失败: ' + (error.message || '未知错误'))
  }
}

// 编辑 PV YAML
const handleEditPVYaml = async (row) => {
  try {
    loading.value = true
    const response = await k8sApi.getPVYaml(selectedClusterId.value, row?.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      currentPVYaml.value = getYamlContentFromApiResponse(
        responseData,
        `# PV ${row?.name} YAML\napiVersion: v1\nkind: PersistentVolume\nmetadata:\n  name: ${row?.name}`
      )
      currentPVForDetail.value = row
      pvYamlDialogVisible.value = true
    } else {
      ElMessage.error(responseData.message || '获取 PV YAML 失败')
    }
  } catch (error) {
    console.error('获取 PV YAML 失败:', error)
    ElMessage.error('获取 PV YAML 失败')
  } finally {
    loading.value = false
  }
}

// 保存 PV YAML
const handlePVYamlSave = async (data) => {
  try {
    // 检查是否是创建新PV (new-pv表示新建)
    const isCreating = data.resourceName === 'new-pv' || !currentPVForDetail.value?.creationTimestamp

    let response
    if (isCreating) {
      // 创建新PV，使用createPodFromYaml API（支持多种资源类型）
      response = await k8sApi.createPodFromYaml(selectedClusterId.value, queryParams.namespace, { yamlContent: data.yamlContent })
    } else {
      // 更新现有PV
      response = await k8sApi.updatePVYaml(selectedClusterId.value, data.resourceName, data.yamlContent)
    }

    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(isCreating ? 'PV 创建成功' : 'PV YAML 更新成功')
      pvYamlDialogVisible.value = false
      fetchPVList() // 刷新列表
    } else {
      ElMessage.error(responseData.message || (isCreating ? 'PV 创建失败' : 'PV YAML 更新失败'))
    }
  } catch (error) {
    console.error('PV 操作失败:', error)
    ElMessage.error('PV 操作失败')
  }
}

// 删除 PV
const handleDeletePV = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确认删除 PV "${row?.name}"？删除后数据将无法恢复。`,
      '删除确认',
      {
        confirmButtonText: '确认删除',
        cancelButtonText: '取消',
        type: 'warning',
        confirmButtonClass: 'el-button--danger'
      }
    )

    const response = await k8sApi.deletePV(selectedClusterId.value, row?.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success('PV 删除成功')
      fetchPVList()
    } else {
      ElMessage.error(responseData.message || 'PV 删除失败')
    }
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除 PV 失败:', error)
      ElMessage.error('删除 PV 失败')
    }
  }
}

// StorageClass 操作
const handleCreateStorageClass = () => {
  if (!selectedClusterId.value) {
    ElMessage.warning('请先选择集群')
    return
  }

  // 设置默认的StorageClass YAML模板
  const defaultYaml = `apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: new-storageclass
provisioner: kubernetes.io/no-provisioner
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer`

  currentStorageClassYaml.value = defaultYaml
  currentStorageClassForDetail.value = { name: 'new-storageclass' }
  storageClassYamlDialogVisible.value = true
}

// 查看 StorageClass 详情
const handleViewStorageClass = async (row) => {
  try {
    console.log('🔍 获取StorageClass详情:', {
      clusterId: selectedClusterId.value,
      storageClassName: row?.name,
      row: row
    })

    const response = await k8sApi.getStorageClassDetail(selectedClusterId.value, row?.name)
    const responseData = response.data || response

    console.log('📥 StorageClass详情API响应:', responseData)

    if (responseData.code === 200 || responseData.success) {
      currentStorageClassForDetail.value = responseData.data || row
      console.log('✅ StorageClass详情数据已设置，打开对话框')
      storageClassDetailDialogVisible.value = true
    } else {
      console.error('❌ API返回失败:', responseData)
      ElMessage.error(responseData.message || '获取 StorageClass 详情失败')
    }
  } catch (error) {
    console.error('❌ 获取 StorageClass 详情失败:', error)
    ElMessage.error('获取 StorageClass 详情失败: ' + (error.message || '未知错误'))
  }
}

// 编辑 StorageClass YAML
const handleEditStorageClassYaml = async (row) => {
  try {
    loading.value = true
    const response = await k8sApi.getStorageClassYaml(selectedClusterId.value, row?.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      currentStorageClassYaml.value = getYamlContentFromApiResponse(
        responseData,
        `# StorageClass ${row?.name} YAML\napiVersion: storage.k8s.io/v1\nkind: StorageClass\nmetadata:\n  name: ${row?.name}`
      )
      currentStorageClassForDetail.value = row
      storageClassYamlDialogVisible.value = true
    } else {
      ElMessage.error(responseData.message || '获取 StorageClass YAML 失败')
    }
  } catch (error) {
    console.error('获取 StorageClass YAML 失败:', error)
    ElMessage.error('获取 StorageClass YAML 失败')
  } finally {
    loading.value = false
  }
}

// 保存 StorageClass YAML
const handleStorageClassYamlSave = async (data) => {
  try {
    // 检查是否是创建新StorageClass (new-storageclass表示新建)
    const isCreating = data.resourceName === 'new-storageclass' || !currentStorageClassForDetail.value?.creationTimestamp

    let response
    if (isCreating) {
      // 创建新StorageClass，使用createPodFromYaml API（支持多种资源类型）
      response = await k8sApi.createPodFromYaml(selectedClusterId.value, queryParams.namespace, { yamlContent: data.yamlContent })
    } else {
      // 更新现有StorageClass
      response = await k8sApi.updateStorageClassYaml(selectedClusterId.value, data.resourceName, data.yamlContent)
    }

    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(isCreating ? 'StorageClass 创建成功' : 'StorageClass YAML 更新成功')
      storageClassYamlDialogVisible.value = false
      fetchStorageClassList() // 刷新列表
    } else {
      ElMessage.error(responseData.message || (isCreating ? 'StorageClass 创建失败' : 'StorageClass YAML 更新失败'))
    }
  } catch (error) {
    console.error('StorageClass 操作失败:', error)
    ElMessage.error('StorageClass 操作失败')
  }
}

// 删除 StorageClass
const handleDeleteStorageClass = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确认删除 StorageClass "${row?.name}"？删除后可能影响使用该存储类的PVC。`,
      '删除确认',
      {
        confirmButtonText: '确认删除',
        cancelButtonText: '取消',
        type: 'warning',
        confirmButtonClass: 'el-button--danger'
      }
    )

    const response = await k8sApi.deleteStorageClass(selectedClusterId.value, row?.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success('StorageClass 删除成功')
      fetchStorageClassList()
    } else {
      ElMessage.error(responseData.message || 'StorageClass 删除失败')
    }
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除 StorageClass 失败:', error)
      ElMessage.error('删除 StorageClass 失败')
    }
  }
}

// 工具函数
const getClusterStatusText = (status) => {
  const statusMap = {
    1: '(运行中)',
    2: '(正常)',
    3: '(异常)',
    4: '(连接失败)'
  }
  return statusMap[status] || '(未知状态)'
}

const getClusterStatusTag = (status) => {
  const tagMap = {
    1: 'success',
    2: 'success',
    3: 'warning',
    4: 'danger'
  }
  return tagMap[status] || 'info'
}

const getStatusTagType = (status) => {
  const statusMap = {
    'Available': 'success',
    'Bound': 'success',
    'Released': 'warning',
    'Failed': 'danger',
    'Pending': 'info'
  }
  return statusMap[status] || 'info'
}

// 监听集群和命名空间变化，自动加载数据
watch(
  [selectedClusterId, () => queryParams.namespace],
  ([clusterId, namespace]) => {
    console.log('监听到变化 - 集群ID:', clusterId, '命名空间:', namespace)
    if (clusterId && namespace) {
      console.log('集群和命名空间都已选择，开始加载存储资源')
      loadAllStorageResources()
    }
  },
  { immediate: true }
)

// 页面初始化
onMounted(async () => {
  console.log('🚀 开始加载k8s存储管理页面')
  const startTime = Date.now()

  try {
    console.log('🎉 页面初始化完成，总耗时:', Date.now() - startTime + 'ms')
  } catch (error) {
    console.error('页面初始化失败:', error)
  }
})
</script>

<template>
  <div class="k8s-storage-management">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/k8s.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>K8s 存储管理</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>统一查看 PVC、PV 与 StorageClass 资源状态，处理持久化存储配置与 YAML 维护。</p>
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

    <section class="storage-card page-card">

      <div class="search-section">
        <el-form :inline="true" class="search-form">
          <el-form-item label="存储资源名称">
            <el-input
              v-model="searchKeyword"
              placeholder="请输入名称，支持模糊搜索"
              clearable
              class="search-name-input"
              @keyup.enter="handleSearch"
            >
              <template #prefix>
                <el-icon class="search-name-icon"><Search /></el-icon>
              </template>
            </el-input>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :icon="Search" class="query-button" @click="handleSearch">
              查询
            </el-button>
            <el-button type="warning" :icon="Refresh" class="refresh-button" @click="handleRefresh">
              刷新
            </el-button>
          </el-form-item>
        </el-form>

        <div class="storage-type-section">
          <el-tabs v-model="activeTab" @tab-change="handleTabChange" class="storage-tabs">
            <el-tab-pane label="PVC" name="pvc" />
            <el-tab-pane label="PV" name="pv" />
            <el-tab-pane label="StorageClass" name="storageclass" />
          </el-tabs>
        </div>
      </div>

      <section class="table-section">
        <div class="content-header">
          <span class="resource-count">{{ currentResourceCountText }}</span>
          <div class="create-actions">
            <el-button
              v-if="activeTab === 'pvc'"
              type="success"
              :icon="Plus"
              class="primary-action"
              @click="handleCreatePVC"
            >
              创建 PVC
            </el-button>
            <el-button
              v-else-if="activeTab === 'pv'"
              type="success"
              :icon="Plus"
              class="primary-action"
              @click="handleCreatePV"
            >
              创建 PV
            </el-button>
            <el-button
              v-else
              type="success"
              :icon="Plus"
              class="primary-action"
              @click="handleCreateStorageClass"
            >
              创建 StorageClass
            </el-button>
          </div>
        </div>

        <div v-if="activeTab === 'pvc'" class="tab-content">
          <el-table
            :data="filteredPVCList"
            v-loading="loading"
            element-loading-text="加载中..."
            class="resource-table"
            empty-text="暂无 PVC 资源"
          >
            <el-table-column prop="name" label="名称" min-width="180">
              <template #default="{ row }">
                <div class="resource-name">
                  <el-icon class="resource-icon"><DataLine /></el-icon>
                  <span class="resource-name-link" @click="handleViewPVC(row)">{{ row?.name || '-' }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column prop="status" label="状态" width="120">
              <template #default="{ row }">
                <el-tag
                  :type="getStatusTagType(row?.status)"
                  size="small"
                >
                  {{ row?.status || '未知' }}
                </el-tag>
              </template>
            </el-table-column>

            <el-table-column prop="capacity" label="容量" width="110">
              <template #default="{ row }">
                <span>{{ row?.capacity || '-' }}</span>
              </template>
            </el-table-column>

            <el-table-column prop="accessModes" label="访问模式" width="160">
              <template #default="{ row }">
                <div class="access-modes">
                  <el-tag
                    v-for="mode in (row?.accessModes || [])"
                    :key="mode"
                    size="small"
                    type="info"
                    class="mode-tag"
                  >
                    {{ mode }}
                  </el-tag>
                </div>
              </template>
            </el-table-column>

            <el-table-column prop="storageClassName" label="存储类" width="160">
              <template #default="{ row }">
                <span>{{ row?.storageClassName || '-' }}</span>
              </template>
            </el-table-column>

            <el-table-column prop="creationTimestamp" label="创建时间" width="180">
              <template #default="{ row }">
                <span class="datetime-text">{{ formatDateTime(row?.creationTimestamp || row?.createdAt) }}</span>
              </template>
            </el-table-column>

            <el-table-column label="操作" width="160" fixed="right">
              <template #default="{ row }">
                <div class="operation-buttons">
                  <button
                    type="button"
                    class="operation-text detail"
                    @click="handleViewPVC(row)"
                  >
                    详情
                  </button>
                  <el-dropdown trigger="click">
                    <button type="button" class="operation-text more">
                      更多
                    </button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item @click="handleEditPVCYaml(row)">编辑 YAML</el-dropdown-item>
                        <el-dropdown-item @click="handleDeletePVC(row)">删除</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div v-else-if="activeTab === 'pv'" class="tab-content">
          <el-table
            :data="filteredPVList"
            v-loading="loading"
            element-loading-text="加载中..."
            class="resource-table"
            empty-text="暂无 PV 资源"
          >
            <el-table-column prop="name" label="名称" min-width="180">
              <template #default="{ row }">
                <div class="resource-name">
                  <el-icon class="resource-icon"><Cpu /></el-icon>
                  <span class="resource-name-link" @click="handleViewPV(row)">{{ row?.name || '-' }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column prop="status" label="状态" width="120">
              <template #default="{ row }">
                <el-tag
                  :type="getStatusTagType(row?.status)"
                  size="small"
                >
                  {{ row?.status || '未知' }}
                </el-tag>
              </template>
            </el-table-column>

            <el-table-column prop="capacity" label="容量" width="110">
              <template #default="{ row }">
                <span>{{ row?.capacity || '-' }}</span>
              </template>
            </el-table-column>

            <el-table-column prop="accessModes" label="访问模式" width="160">
              <template #default="{ row }">
                <div class="access-modes">
                  <el-tag
                    v-for="mode in (row?.accessModes || [])"
                    :key="mode"
                    size="small"
                    type="info"
                    class="mode-tag"
                  >
                    {{ mode }}
                  </el-tag>
                </div>
              </template>
            </el-table-column>

            <el-table-column prop="reclaimPolicy" label="回收策略" width="120">
              <template #default="{ row }">
                <span>{{ row?.reclaimPolicy || '-' }}</span>
              </template>
            </el-table-column>

            <el-table-column prop="storageClassName" label="存储类" width="160">
              <template #default="{ row }">
                <span>{{ row?.storageClassName || '-' }}</span>
              </template>
            </el-table-column>

            <el-table-column prop="creationTimestamp" label="创建时间" width="180">
              <template #default="{ row }">
                <span class="datetime-text">{{ formatDateTime(row?.creationTimestamp || row?.createdAt) }}</span>
              </template>
            </el-table-column>

            <el-table-column label="操作" width="160" fixed="right">
              <template #default="{ row }">
                <div class="operation-buttons">
                  <button
                    type="button"
                    class="operation-text detail"
                    @click="handleViewPV(row)"
                  >
                    详情
                  </button>
                  <el-dropdown trigger="click">
                    <button type="button" class="operation-text more">
                      更多
                    </button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item @click="handleEditPVYaml(row)">编辑 YAML</el-dropdown-item>
                        <el-dropdown-item @click="handleDeletePV(row)">删除</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div v-else class="tab-content">
          <el-table
            :data="filteredStorageClassList"
            v-loading="loading"
            element-loading-text="加载中..."
            class="resource-table"
            empty-text="暂无 StorageClass 资源"
          >
            <el-table-column prop="name" label="名称" min-width="180">
              <template #default="{ row }">
                <div class="resource-name">
                  <el-icon class="resource-icon"><Connection /></el-icon>
                  <span class="resource-name-link" @click="handleViewStorageClass(row)">{{ row?.name || '-' }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column prop="provisioner" label="供应商" min-width="220">
              <template #default="{ row }">
                <span>{{ row?.provisioner || '-' }}</span>
              </template>
            </el-table-column>

            <el-table-column prop="reclaimPolicy" label="回收策略" width="120">
              <template #default="{ row }">
                <span>{{ row?.reclaimPolicy || '-' }}</span>
              </template>
            </el-table-column>

            <el-table-column prop="allowVolumeExpansion" label="支持扩容" width="110">
              <template #default="{ row }">
                <el-tag
                  :type="row?.allowVolumeExpansion ? 'success' : 'info'"
                  size="small"
                >
                  {{ row?.allowVolumeExpansion ? '是' : '否' }}
                </el-tag>
              </template>
            </el-table-column>

            <el-table-column prop="volumeBindingMode" label="绑定模式" width="160">
              <template #default="{ row }">
                <span>{{ row?.volumeBindingMode || '-' }}</span>
              </template>
            </el-table-column>

            <el-table-column prop="creationTimestamp" label="创建时间" width="180">
              <template #default="{ row }">
                <span class="datetime-text">{{ formatDateTime(row?.creationTimestamp || row?.createdAt) }}</span>
              </template>
            </el-table-column>

            <el-table-column label="操作" width="160" fixed="right">
              <template #default="{ row }">
                <div class="operation-buttons">
                  <button
                    type="button"
                    class="operation-text detail"
                    @click="handleViewStorageClass(row)"
                  >
                    详情
                  </button>
                  <el-dropdown trigger="click">
                    <button type="button" class="operation-text more">
                      更多
                    </button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item @click="handleEditStorageClassYaml(row)">编辑 YAML</el-dropdown-item>
                        <el-dropdown-item @click="handleDeleteStorageClass(row)">删除</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </section>
    </section>

    <!-- PVC 详情对话框 -->
    <el-dialog
      v-model="pvcDetailDialogVisible"
      title="PVC 详情"
      width="720px"
      class="detail-dialog"
      modal-class="k8s-storage-dialog-overlay"
    >
      <div class="detail-content">
        <div class="detail-item">
          <span class="detail-label">名称</span>
          <span class="detail-value">{{ currentPVCForDetail.name || '-' }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">命名空间</span>
          <span class="detail-value">{{ currentPVCForDetail.namespace || '-' }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">状态</span>
          <el-tag :type="getStatusTagType(currentPVCForDetail.status)" size="small">
            {{ currentPVCForDetail.status || 'Unknown' }}
          </el-tag>
        </div>
        <div class="detail-item">
          <span class="detail-label">容量</span>
          <span class="detail-value">{{ currentPVCForDetail.capacity || '-' }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">访问模式</span>
          <div class="access-modes">
            <el-tag
              v-for="mode in (currentPVCForDetail.accessModes || [])"
              :key="mode"
              size="small"
              type="info"
              class="mode-tag"
            >
              {{ mode }}
            </el-tag>
          </div>
        </div>
        <div class="detail-item">
          <span class="detail-label">存储类</span>
          <span class="detail-value">{{ currentPVCForDetail.storageClass || currentPVCForDetail.storageClassName || '-' }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">创建时间</span>
          <span class="detail-value">{{ formatDateTime(currentPVCForDetail.createdAt || currentPVCForDetail.creationTimestamp) }}</span>
        </div>
        <div class="detail-item" v-if="currentPVCForDetail.volumeName">
          <span class="detail-label">绑定的 PV</span>
          <span class="detail-value">{{ currentPVCForDetail.volumeName }}</span>
        </div>
        <div class="detail-item" v-if="currentPVCForDetail.volumeMode">
          <span class="detail-label">卷模式</span>
          <span class="detail-value">{{ currentPVCForDetail.volumeMode }}</span>
        </div>
      </div>
    </el-dialog>

    <!-- PV 详情对话框 -->
    <el-dialog
      v-model="pvDetailDialogVisible"
      title="PV 详情"
      width="720px"
      class="detail-dialog"
      modal-class="k8s-storage-dialog-overlay"
    >
      <div class="detail-content">
        <div class="detail-item">
          <span class="detail-label">名称</span>
          <span class="detail-value">{{ currentPVForDetail.name || '-' }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">状态</span>
          <el-tag :type="getStatusTagType(currentPVForDetail.status)" size="small">
            {{ currentPVForDetail.status || 'Unknown' }}
          </el-tag>
        </div>
        <div class="detail-item">
          <span class="detail-label">容量</span>
          <span class="detail-value">{{ currentPVForDetail.capacity || '-' }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">访问模式</span>
          <div class="access-modes">
            <el-tag
              v-for="mode in (currentPVForDetail.accessModes || [])"
              :key="mode"
              size="small"
              type="info"
              class="mode-tag"
            >
              {{ mode }}
            </el-tag>
          </div>
        </div>
        <div class="detail-item">
          <span class="detail-label">回收策略</span>
          <span class="detail-value">{{ currentPVForDetail.reclaimPolicy || '-' }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">存储类</span>
          <span class="detail-value">{{ currentPVForDetail.storageClass || currentPVForDetail.storageClassName || '-' }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">创建时间</span>
          <span class="detail-value">{{ formatDateTime(currentPVForDetail.createdAt || currentPVForDetail.creationTimestamp) }}</span>
        </div>
        <div class="detail-item" v-if="currentPVForDetail.volumeMode">
          <span class="detail-label">卷模式</span>
          <span class="detail-value">{{ currentPVForDetail.volumeMode }}</span>
        </div>
        <div class="detail-item" v-if="currentPVForDetail.persistentVolumeSource">
          <span class="detail-label">存储源</span>
          <div class="storage-source">
            <div v-if="currentPVForDetail.persistentVolumeSource.hostPath">
              <strong>HostPath:</strong> {{ currentPVForDetail.persistentVolumeSource.hostPath.path }}
              <br>
              <small>类型: {{ currentPVForDetail.persistentVolumeSource.hostPath.type }}</small>
            </div>
            <div v-else-if="currentPVForDetail.persistentVolumeSource.nfs">
              <strong>NFS:</strong> {{ currentPVForDetail.persistentVolumeSource.nfs.server }}:{{ currentPVForDetail.persistentVolumeSource.nfs.path }}
            </div>
            <div v-else>
              {{ Object.keys(currentPVForDetail.persistentVolumeSource)[0] }}
            </div>
          </div>
        </div>
        <div class="detail-item" v-if="currentPVForDetail.claimRef">
          <span class="detail-label">绑定的 PVC</span>
          <span class="detail-value">{{ currentPVForDetail.claimRef.namespace }}/{{ currentPVForDetail.claimRef.name }}</span>
        </div>
      </div>
    </el-dialog>

    <!-- StorageClass 详情对话框 -->
    <el-dialog
      v-model="storageClassDetailDialogVisible"
      title="StorageClass 详情"
      width="720px"
      class="detail-dialog"
      modal-class="k8s-storage-dialog-overlay"
    >
      <div class="detail-content">
        <div class="detail-item">
          <span class="detail-label">名称</span>
          <span class="detail-value">{{ currentStorageClassForDetail.name || '-' }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">供应商</span>
          <span class="detail-value">{{ currentStorageClassForDetail.provisioner || '-' }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">回收策略</span>
          <span class="detail-value">{{ currentStorageClassForDetail.reclaimPolicy || '-' }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">支持扩容</span>
          <el-tag
            :type="currentStorageClassForDetail.allowVolumeExpansion ? 'success' : 'info'"
            size="small"
          >
            {{ currentStorageClassForDetail.allowVolumeExpansion ? '是' : '否' }}
          </el-tag>
        </div>
        <div class="detail-item">
          <span class="detail-label">绑定模式</span>
          <span class="detail-value">{{ currentStorageClassForDetail.volumeBindingMode || '-' }}</span>
        </div>
        <div class="detail-item">
          <span class="detail-label">创建时间</span>
          <span class="detail-value">{{ formatDateTime(currentStorageClassForDetail.creationTimestamp || currentStorageClassForDetail.createdAt) }}</span>
        </div>
      </div>
    </el-dialog>

    <!-- PVC YAML 编辑对话框 -->
    <PodYamlDialog
      :visible="pvcYamlDialogVisible"
      :yaml-content="currentPVCYaml"
      :resource-name="currentPVCForDetail.name || 'new-pvc'"
      :resource-type="'PVC'"
      :editable="true"
      dialog-width="900px"
      editor-height="430px"
      @update:visible="pvcYamlDialogVisible = $event"
      @close="pvcYamlDialogVisible = false"
      @save="handlePVCYamlSave"
    />

    <!-- PV YAML 编辑对话框 -->
    <PodYamlDialog
      :visible="pvYamlDialogVisible"
      :yaml-content="currentPVYaml"
      :resource-name="currentPVForDetail.name || 'new-pv'"
      :resource-type="'PV'"
      :editable="true"
      dialog-width="900px"
      editor-height="430px"
      @update:visible="pvYamlDialogVisible = $event"
      @close="pvYamlDialogVisible = false"
      @save="handlePVYamlSave"
    />

    <!-- StorageClass YAML 编辑对话框 -->
    <PodYamlDialog
      :visible="storageClassYamlDialogVisible"
      :yaml-content="currentStorageClassYaml"
      :resource-name="currentStorageClassForDetail.name || 'new-storageclass'"
      :resource-type="'StorageClass'"
      :editable="true"
      dialog-width="900px"
      editor-height="430px"
      @update:visible="storageClassYamlDialogVisible = $event"
      @close="storageClassYamlDialogVisible = false"
      @save="handleStorageClassYamlSave"
    />
  </div>
</template>

<style scoped>
.k8s-storage-management {
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
  --storage-page-height-offset: 190px;
  min-height: calc(100vh - var(--storage-page-height-offset));
  padding: 6px;
  box-sizing: border-box;
  background:
    radial-gradient(circle at 16% 10%, rgba(76, 139, 245, 0.06), transparent 26%),
    linear-gradient(180deg, #f7f9fc 0%, var(--aiops-bg-page) 100%);
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.storage-card {
  position: relative;
  display: flex;
  flex-direction: column;
  max-height: calc(100vh - 12px - var(--storage-page-height-offset));
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

.storage-card::before {
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

.storage-type-section {
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px solid rgba(223, 231, 242, 0.72);
}

.storage-tabs :deep(.el-tabs__header) {
  margin: 0;
}

.storage-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: rgba(223, 231, 242, 0.78);
}

.storage-tabs :deep(.el-tabs__item) {
  height: 34px;
  padding: 0 14px;
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

.storage-tabs :deep(.el-tabs__item.is-active) {
  color: var(--aiops-brand);
}

.storage-tabs :deep(.el-tabs__active-bar) {
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

.tab-content {
  min-width: 0;
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
}

.resource-table :deep(.el-table__body tr:hover > td.el-table__cell) {
  background: rgba(76, 139, 245, 0.045) !important;
}

.resource-name {
  display: flex;
  align-items: center;
  gap: 8px;
}

.resource-icon {
  color: var(--aiops-brand);
  font-size: 16px;
}

.resource-name-link {
  color: var(--aiops-brand);
  cursor: pointer;
  font-size: 13px;
  font-weight: 700;
  transition: opacity 0.2s ease;
  text-decoration: none;
}

.resource-name-link:hover {
  opacity: 0.82;
}

.access-modes {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.mode-tag {
  font-size: 11px;
}

.operation-buttons {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: center;
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
  opacity: 0.82;
}

.operation-buttons :deep(.operation-text.detail) {
  color: var(--aiops-brand);
}

.operation-buttons :deep(.operation-text.more) {
  color: #f3a33a;
}

.datetime-text {
  color: var(--aiops-text-secondary);
  font-size: 12px;
  font-weight: 600;
}

.detail-dialog :deep(.el-dialog) {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 24px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
  box-shadow: 0 36px 110px rgba(15, 23, 42, 0.11);
  backdrop-filter: blur(18px);
}

.detail-dialog :deep(.el-dialog__header) {
  padding: 16px 20px 14px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.84);
}

.detail-dialog :deep(.el-dialog__title) {
  color: var(--aiops-text-primary);
  font-size: 17px;
  font-weight: 700;
}

.detail-dialog :deep(.el-dialog__body) {
  padding: 18px 20px;
  background:
    radial-gradient(circle at top left, rgba(76, 139, 245, 0.04), transparent 22%),
    linear-gradient(180deg, #f8fafc 0%, #f5f8fc 100%);
}

.detail-content {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.detail-item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  min-height: 44px;
  padding: 10px 0;
  border-bottom: 1px solid rgba(223, 231, 242, 0.68);
}

.detail-label {
  min-width: 88px;
  color: var(--aiops-text-muted);
  font-size: 12px;
  font-weight: 700;
}

.detail-value {
  color: var(--aiops-text-primary);
  font-size: 13px;
  font-weight: 600;
  word-break: break-all;
}

.storage-source {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.84);
  font-size: 13px;
  line-height: 1.6;
}

.storage-source strong {
  color: var(--aiops-brand);
}

.storage-source small {
  color: var(--aiops-text-muted);
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

.k8s-storage-management .el-button:hover {
  transform: translateY(-1px);
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

@media (max-width: 768px) {
  .k8s-storage-management {
    --storage-page-height-offset: 131px;
    padding: 4px;
  }

  .storage-card {
    max-height: calc(100vh - 8px - var(--storage-page-height-offset));
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

  .detail-content {
    grid-template-columns: 1fr;
  }

  .header-actions :deep(.cluster-selector),
  .header-actions :deep(.namespace-selector) {
    width: 100% !important;
  }

  .operation-buttons {
    flex-direction: column;
    gap: 4px;
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
}

/* DESIGN.md compact workbench overrides */
.k8s-storage-management {
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

.storage-card {
  height: auto;
  max-height: none;
  padding: 10px;
}

.storage-card::before {
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
  min-width: 104px;
}

.k8s-storage-management .el-button:hover,
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

.storage-type-section {
  margin-top: 6px;
  padding-top: 6px;
}

.storage-tabs :deep(.el-tabs__item) {
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

.resource-table {
  min-width: 900px;
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

.resource-name {
  gap: 7px;
  min-width: 0;
}

.resource-name-link {
  overflow: hidden;
  color: #2563eb;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: color 160ms ease, opacity 160ms ease;
}

.resource-name-link:hover {
  color: #1d4ed8;
  transform: none;
}

.resource-table :deep(.el-tag) {
  font-size: 11px;
}

.datetime-text {
  color: #64748b;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 11px;
}

.operation-buttons {
  gap: 14px;
  justify-content: flex-end;
}

.operation-buttons :deep(.el-button),
.operation-buttons .operation-text {
  transform: none;
  transition: color 160ms ease, opacity 160ms ease;
}

.detail-content {
  gap: 8px 16px;
}

.detail-item {
  min-height: 38px;
  padding: 7px 0;
  border-bottom-color: #e7eef7;
}

.detail-label {
  min-width: 76px;
  color: #64748b;
}

.detail-value {
  color: #172033;
  font-size: 12px;
}

.storage-source {
  padding: 8px 10px;
  border-color: #dce8f5;
  border-radius: 8px;
  background: #f8fbff;
  font-size: 12px;
}

:global(.k8s-storage-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 24px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: 15px !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.k8s-storage-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: linear-gradient(90deg, #fff6e7 0%, #f7fbff 36%, #ecf7ff 100%);
}

:global(.k8s-storage-dialog-overlay .el-dialog__title) {
  color: #172033;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.k8s-storage-dialog-overlay .el-dialog__headerbtn) {
  top: 12px;
  right: 14px;
  width: 32px;
  height: 32px;
}

:global(.k8s-storage-dialog-overlay .el-dialog__body) {
  max-height: calc(100vh - 190px);
  overflow: auto;
  padding: 14px 18px;
  background: #fff;
}

:global(.k8s-storage-dialog-overlay .el-button) {
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
  .k8s-storage-management {
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
  .storage-card {
    padding: 8px;
    border-radius: 12px;
  }

  .search-form :deep(.el-form-item) {
    margin-bottom: 8px;
  }
}
</style>
