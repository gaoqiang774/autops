<script setup>
import { ref, reactive, onMounted, computed, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Plus,
  Refresh,
  Setting,
  Key,
  Search,
  Lock
} from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'
import PodYamlDialog from './pods/PodYamlDialog.vue'
import ConfigMapEditorDialog from './components/ConfigMapEditorDialog.vue'
import ClusterSelector from './pods/ClusterSelector.vue'
import NamespaceSelector from './pods/NamespaceSelector.vue'
import { getYamlContentFromApiResponse } from './utils/yaml'

const CONFIG_RESOURCE_PAGE_SIZE = 100
const DEFAULT_TABLE_PAGE_SIZE = 10

// 基础状态
const loading = ref(false)
const activeTab = ref('configmap')
const searchKeyword = ref('')

// 集群和命名空间状态
const selectedClusterId = ref('')
const queryParams = reactive({
  namespace: 'default'
})

// 配置资源数据状态
const configMapList = ref([])
const secretList = ref([])
const configMapPagination = reactive({ page: 1, pageSize: DEFAULT_TABLE_PAGE_SIZE })
const secretPagination = reactive({ page: 1, pageSize: DEFAULT_TABLE_PAGE_SIZE })

// 对话框状态
const configMapYamlDialogVisible = ref(false)
const secretYamlDialogVisible = ref(false)
const configMapDetailDialogVisible = ref(false)
const secretDetailDialogVisible = ref(false)
const isCreatingConfigMap = ref(false)

// 当前操作的资源
const currentConfigMapYaml = ref('')
const currentSecretYaml = ref('')
const currentResourceName = ref('')
const currentResourceType = ref('')

// 当前查看的资源详情
const currentConfigMapForDetail = ref({})
const currentSecretForDetail = ref({})

// 过滤后的列表
const filteredConfigMapList = computed(() => {
  const list = Array.isArray(configMapList.value) ? configMapList.value : []
  if (!searchKeyword.value) return list
  return list.filter(item =>
    item.name?.toLowerCase().includes(searchKeyword.value.toLowerCase()) ||
    item.namespace?.toLowerCase().includes(searchKeyword.value.toLowerCase())
  )
})

const filteredSecretList = computed(() => {
  const list = Array.isArray(secretList.value) ? secretList.value : []
  if (!searchKeyword.value) return list
  return list.filter(item =>
    item.name?.toLowerCase().includes(searchKeyword.value.toLowerCase()) ||
    item.type?.toLowerCase().includes(searchKeyword.value.toLowerCase()) ||
    item.namespace?.toLowerCase().includes(searchKeyword.value.toLowerCase())
  )
})

const paginateList = (rows, pagination) => {
  const start = (pagination.page - 1) * pagination.pageSize
  return rows.slice(start, start + pagination.pageSize)
}

const paginatedConfigMapList = computed(() => paginateList(filteredConfigMapList.value, configMapPagination))
const paginatedSecretList = computed(() => paginateList(filteredSecretList.value, secretPagination))
const currentPagination = computed(() => activeTab.value === 'secret' ? secretPagination : configMapPagination)
const currentResourceTotal = computed(() => (
  activeTab.value === 'secret'
    ? filteredSecretList.value.length
    : filteredConfigMapList.value.length
))

const resetConfigResourcePagination = () => {
  configMapPagination.page = 1
  secretPagination.page = 1
}

const clampPaginationPage = (total, pagination) => {
  const maxPage = Math.max(1, Math.ceil(total / pagination.pageSize))
  if (pagination.page > maxPage) pagination.page = maxPage
}

// 处理集群选择变化
const handleClusterChange = (clusterId) => {
  selectedClusterId.value = clusterId
  resetConfigResourcePagination()
  console.log('集群选择变化:', clusterId)
  if (clusterId && queryParams.namespace) {
    loadAllConfigResources()
  }
}

// 处理命名空间选择变化
const handleNamespaceChange = (namespace) => {
  queryParams.namespace = namespace
  resetConfigResourcePagination()
  console.log('命名空间选择变化:', namespace)
  if (selectedClusterId.value && namespace) {
    loadAllConfigResources()
  }
}

// 加载所有配置资源
const loadAllConfigResources = async () => {
  if (!selectedClusterId.value) {
    console.warn('集群ID为空，无法加载配置资源')
    return
  }

  console.log('开始加载配置资源，集群ID:', selectedClusterId.value, '命名空间:', queryParams.namespace)

  loading.value = true
  try {
    // 并发加载所有配置资源
    await Promise.all([
      fetchConfigMapList(),
      fetchSecretList()
    ])
  } catch (error) {
    console.error('加载配置资源失败:', error)
  } finally {
    loading.value = false
  }
}

// 重置搜索
const handleSearch = () => {
  searchKeyword.value = searchKeyword.value.trim()
  resetConfigResourcePagination()
}

// 标签页切换处理
const handleTabChange = (tabName) => {
  console.log('标签页切换到:', tabName)
  activeTab.value = tabName
}

const handleResourceSizeChange = (size) => {
  currentPagination.value.pageSize = size
  currentPagination.value.page = 1
}

const handleResourceCurrentChange = (page) => {
  currentPagination.value.page = page
}

const getResponseData = response => response?.data || response || {}

const extractConfigResourceRows = (responseData, listKey) => {
  const data = responseData?.data
  if (Array.isArray(data?.[listKey])) return data[listKey]
  if (Array.isArray(data)) return data
  if (Array.isArray(responseData?.[listKey])) return responseData[listKey]
  return []
}

const extractConfigResourceTotal = (responseData, listKey) => {
  const rows = extractConfigResourceRows(responseData, listKey)
  const total = responseData?.data?.total ?? responseData?.total
  const totalNumber = Number(total)
  return Number.isFinite(totalNumber) ? totalNumber : rows.length
}

const fetchPagedConfigResources = async (fetcher, listKey) => {
  const firstResponse = await fetcher({ page: 1, pageSize: CONFIG_RESOURCE_PAGE_SIZE })
  const firstData = getResponseData(firstResponse)

  if (!(firstData.code === 200 || firstData.success)) {
    throw new Error(firstData.message || '获取配置资源列表失败')
  }

  const firstRows = extractConfigResourceRows(firstData, listKey)
  const total = extractConfigResourceTotal(firstData, listKey)
  const pageCount = Math.ceil(total / CONFIG_RESOURCE_PAGE_SIZE)
  if (pageCount <= 1) return firstRows

  const requests = []
  for (let page = 2; page <= pageCount; page += 1) {
    requests.push(fetcher({ page, pageSize: CONFIG_RESOURCE_PAGE_SIZE }))
  }

  const restResponses = await Promise.all(requests)
  return restResponses.reduce((rows, response) => {
    const responseData = getResponseData(response)
    if (!(responseData.code === 200 || responseData.success)) {
      throw new Error(responseData.message || '获取配置资源列表失败')
    }
    return rows.concat(extractConfigResourceRows(responseData, listKey))
  }, firstRows)
}

// 获取ConfigMap列表
const fetchConfigMapList = async () => {
  if (!selectedClusterId.value || !queryParams.namespace) {
    console.warn('集群ID或命名空间为空，无法获取 ConfigMap 列表')
    return
  }

  try {
    console.log('正在获取 ConfigMap 列表，集群ID:', selectedClusterId.value, '命名空间:', queryParams.namespace)

    configMapList.value = await fetchPagedConfigResources(
      params => k8sApi.getConfigMaps(selectedClusterId.value, queryParams.namespace, params),
      'configMaps'
    )
    console.log('获取到 ConfigMap 列表:', configMapList.value.length, '个')
  } catch (error) {
    console.error('获取 ConfigMap 列表失败:', error)
    ElMessage.error(error.message || '获取 ConfigMap 列表失败，请检查网络连接')
    configMapList.value = []
  }
}

// 获取Secret列表
const fetchSecretList = async () => {
  if (!selectedClusterId.value || !queryParams.namespace) {
    console.warn('集群ID或命名空间为空，无法获取 Secret 列表')
    return
  }

  try {
    console.log('正在获取 Secret 列表，集群ID:', selectedClusterId.value, '命名空间:', queryParams.namespace)

    secretList.value = await fetchPagedConfigResources(
      params => k8sApi.getSecrets(selectedClusterId.value, queryParams.namespace, params),
      'secrets'
    )
    console.log('获取到 Secret 列表:', secretList.value.length, '个')
  } catch (error) {
    console.error('获取 Secret 列表失败:', error)
    ElMessage.error(error.message || '获取 Secret 列表失败，请检查网络连接')
    secretList.value = []
  }
}

// 刷新数据
const handleRefresh = () => {
  loadAllConfigResources()
}

// ConfigMap 操作
const handleCreateConfigMap = () => {
  if (!selectedClusterId.value) {
    ElMessage.warning('请先选择集群')
    return
  }
  if (!queryParams.namespace) {
    ElMessage.warning('请先选择命名空间')
    return
  }

  // 设置默认的ConfigMap YAML模板
  const defaultYaml = `apiVersion: v1
kind: ConfigMap
metadata:
  name: new-configmap
  namespace: ${queryParams.namespace}
data:
  # 配置键值对
  config.properties: |
    database.url=localhost:5432
    database.user=admin
  app.yaml: |
    app:
      name: my-app
      version: 1.0.0`

  currentConfigMapYaml.value = defaultYaml
  currentConfigMapForDetail.value = { name: 'new-configmap', namespace: queryParams.namespace }
  isCreatingConfigMap.value = true
  configMapYamlDialogVisible.value = true
}

// 查看 ConfigMap 详情
const handleViewConfigMap = async (row) => {
  try {
    const response = await k8sApi.getConfigMapDetail(selectedClusterId.value, queryParams.namespace, row?.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      currentConfigMapForDetail.value = responseData.data || row
      configMapDetailDialogVisible.value = true
    } else {
      ElMessage.error(responseData.message || '获取 ConfigMap 详情失败')
    }
  } catch (error) {
    console.error('获取 ConfigMap 详情失败:', error)
    ElMessage.error('获取 ConfigMap 详情失败')
  }
}

// 编辑 ConfigMap
const handleEditConfigMapYaml = async (row) => {
  try {
    loading.value = true
    const response = await k8sApi.getConfigMapYaml(selectedClusterId.value, queryParams.namespace, row?.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      currentConfigMapYaml.value = getYamlContentFromApiResponse(
        responseData,
        `# ConfigMap ${row?.name} YAML\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: ${row?.name}\n  namespace: ${queryParams.namespace}`
      )
      currentConfigMapForDetail.value = row
      isCreatingConfigMap.value = false
      configMapYamlDialogVisible.value = true
    } else {
      ElMessage.error(responseData.message || '获取 ConfigMap YAML 失败')
    }
  } catch (error) {
    console.error('获取 ConfigMap YAML 失败:', error)
    ElMessage.error('获取 ConfigMap YAML 失败')
  } finally {
    loading.value = false
  }
}

// 保存 ConfigMap
const handleConfigMapYamlSave = async (data) => {
  try {
    const isCreating = isCreatingConfigMap.value
    const targetResourceName = currentConfigMapForDetail.value?.name || data.resourceName

    let response
    if (isCreating) {
      // 创建新ConfigMap
      response = await k8sApi.createConfigMap(selectedClusterId.value, queryParams.namespace, { yamlContent: data.yamlContent })
    } else {
      // 更新现有ConfigMap
      response = await k8sApi.updateConfigMapYaml(selectedClusterId.value, queryParams.namespace, targetResourceName, data.yamlContent)
    }

    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(isCreating ? 'ConfigMap 创建成功' : 'ConfigMap 更新成功')
      configMapYamlDialogVisible.value = false
      fetchConfigMapList() // 刷新列表
    } else {
      ElMessage.error(responseData.message || (isCreating ? 'ConfigMap 创建失败' : 'ConfigMap 更新失败'))
    }
  } catch (error) {
    console.error('ConfigMap 操作失败:', error)
    ElMessage.error('ConfigMap 操作失败')
  }
}

// 删除 ConfigMap
const handleDeleteConfigMap = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确认删除 ConfigMap "${row?.name}"？删除后相关配置将无法恢复。`,
      '删除确认',
      {
        confirmButtonText: '确认删除',
        cancelButtonText: '取消',
        type: 'warning',
        confirmButtonClass: 'el-button--danger'
      }
    )

    const response = await k8sApi.deleteConfigMap(selectedClusterId.value, queryParams.namespace, row?.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success('ConfigMap 删除成功')
      fetchConfigMapList()
    } else {
      ElMessage.error(responseData.message || 'ConfigMap 删除失败')
    }
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除 ConfigMap 失败:', error)
      ElMessage.error('删除 ConfigMap 失败')
    }
  }
}

// Secret 操作
const handleCreateSecret = () => {
  if (!selectedClusterId.value) {
    ElMessage.warning('请先选择集群')
    return
  }
  if (!queryParams.namespace) {
    ElMessage.warning('请先选择命名空间')
    return
  }

  // 设置默认的Secret YAML模板
  const defaultYaml = `apiVersion: v1
kind: Secret
metadata:
  name: new-secret
  namespace: ${queryParams.namespace}
type: Opaque
data:
  # Base64编码的数据
  username: YWRtaW4=  # admin
  password: MWYyZDFlMmU2N2Rm  # password
stringData:
  # 明文数据，k8s会自动编码
  config.json: |
    {
      "database": {
        "host": "localhost",
        "port": 5432
      }
    }`

  currentSecretYaml.value = defaultYaml
  currentSecretForDetail.value = { name: 'new-secret', namespace: queryParams.namespace }
  secretYamlDialogVisible.value = true
}

// 查看 Secret 详情
const handleViewSecret = async (row) => {
  try {
    const response = await k8sApi.getSecretDetail(selectedClusterId.value, queryParams.namespace, row?.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      currentSecretForDetail.value = responseData.data || row
      secretDetailDialogVisible.value = true
    } else {
      ElMessage.error(responseData.message || '获取 Secret 详情失败')
    }
  } catch (error) {
    console.error('获取 Secret 详情失败:', error)
    ElMessage.error('获取 Secret 详情失败')
  }
}

// 编辑 Secret YAML
const handleEditSecretYaml = async (row) => {
  try {
    loading.value = true
    const response = await k8sApi.getSecretYaml(selectedClusterId.value, queryParams.namespace, row?.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      currentSecretYaml.value = getYamlContentFromApiResponse(
        responseData,
        `# Secret ${row?.name} YAML\napiVersion: v1\nkind: Secret\nmetadata:\n  name: ${row?.name}\n  namespace: ${queryParams.namespace}\ntype: Opaque`
      )
      currentSecretForDetail.value = row
      secretYamlDialogVisible.value = true
    } else {
      ElMessage.error(responseData.message || '获取 Secret YAML 失败')
    }
  } catch (error) {
    console.error('获取 Secret YAML 失败:', error)
    ElMessage.error('获取 Secret YAML 失败')
  } finally {
    loading.value = false
  }
}

// 保存 Secret YAML
const handleSecretYamlSave = async (data) => {
  try {
    // 检查是否是创建新Secret (只有 new-secret 表示新建)
    const isCreating = data.resourceName === 'new-secret'

    let response
    if (isCreating) {
      // 创建新Secret
      response = await k8sApi.createSecret(selectedClusterId.value, queryParams.namespace, { yamlContent: data.yamlContent })
    } else {
      // 更新现有Secret
      response = await k8sApi.updateSecretYaml(selectedClusterId.value, queryParams.namespace, data.resourceName, data.yamlContent)
    }

    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success(isCreating ? 'Secret 创建成功' : 'Secret YAML 更新成功')
      secretYamlDialogVisible.value = false
      fetchSecretList() // 刷新列表
    } else {
      ElMessage.error(responseData.message || (isCreating ? 'Secret 创建失败' : 'Secret YAML 更新失败'))
    }
  } catch (error) {
    console.error('Secret 操作失败:', error)
    ElMessage.error('Secret 操作失败')
  }
}

// 删除 Secret
const handleDeleteSecret = async (row) => {
  try {
    await ElMessageBox.confirm(
      `确认删除 Secret "${row?.name}"？删除后敏感数据将无法恢复。`,
      '删除确认',
      {
        confirmButtonText: '确认删除',
        cancelButtonText: '取消',
        type: 'warning',
        confirmButtonClass: 'el-button--danger'
      }
    )

    const response = await k8sApi.deleteSecret(selectedClusterId.value, queryParams.namespace, row?.name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success('Secret 删除成功')
      fetchSecretList()
    } else {
      ElMessage.error(responseData.message || 'Secret 删除失败')
    }
  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除 Secret 失败:', error)
      ElMessage.error('删除 Secret 失败')
    }
  }
}

// 工具函数
const getSecretTypeTag = (type) => {
  const typeMap = {
    'Opaque': 'primary',
    'kubernetes.io/service-account-token': 'success',
    'kubernetes.io/dockercfg': 'warning',
    'kubernetes.io/dockerconfigjson': 'warning',
    'kubernetes.io/basic-auth': 'info',
    'kubernetes.io/ssh-auth': 'info',
    'kubernetes.io/tls': 'danger'
  }
  return typeMap[type] || 'info'
}

const formatSecretType = (type) => {
  if (type?.startsWith('kubernetes.io/')) {
    return type.replace('kubernetes.io/', 'k8s/')
  }
  return type || 'Opaque'
}

const formatDataKeys = (data) => {
  if (!data || typeof data !== 'object') return '无数据'
  const keys = Object.keys(data)
  if (keys.length === 0) return '无数据'
  if (keys.length <= 3) return keys.join(', ')
  return `${keys.slice(0, 3).join(', ')}... (${keys.length}项)`
}

const currentResourceCountText = computed(() => (
  activeTab.value === 'secret'
    ? `共 ${filteredSecretList.value.length} 个 Secret`
    : `共 ${filteredConfigMapList.value.length} 个 ConfigMap`
))

const formatDateTime = (value) => {
  if (!value) return '-'

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  return date.toLocaleString()
}

// 监听集群和命名空间变化，自动加载数据
watch(
  [selectedClusterId, () => queryParams.namespace],
  ([clusterId, namespace]) => {
    console.log('监听到变化 - 集群ID:', clusterId, '命名空间:', namespace)
    if (clusterId && namespace) {
      console.log('集群和命名空间都已选择，开始加载配置资源')
      loadAllConfigResources()
    }
  },
  { immediate: true }
)

watch(searchKeyword, () => {
  resetConfigResourcePagination()
})

watch(
  [() => filteredConfigMapList.value.length, () => configMapPagination.pageSize],
  ([total]) => {
    clampPaginationPage(total, configMapPagination)
  }
)

watch(
  [() => filteredSecretList.value.length, () => secretPagination.pageSize],
  ([total]) => {
    clampPaginationPage(total, secretPagination)
  }
)

// 页面初始化
onMounted(async () => {
  console.log('🚀 开始加载k8s配置管理页面')
  const startTime = Date.now()

  try {
    console.log('🎉 页面初始化完成，总耗时:', Date.now() - startTime + 'ms')
  } catch (error) {
    console.error('页面初始化失败:', error)
  }
})
</script>

<template>
  <div class="k8s-config-management">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/k8s.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>K8s 配置管理</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>集中查看 ConfigMap 与 Secret，统一处理配置数据、敏感信息和 YAML 维护。</p>
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

    <section class="config-card page-card">

      <div class="search-section">
        <el-form :inline="true" class="search-form">
          <el-form-item label="配置资源名称">
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

        <div class="config-type-section">
          <el-tabs v-model="activeTab" @tab-change="handleTabChange" class="config-tabs">
            <el-tab-pane label="ConfigMap" name="configmap" />
            <el-tab-pane label="Secret" name="secret" />
          </el-tabs>
        </div>
      </div>

      <section class="table-section">
        <div class="content-header">
          <span class="resource-count">{{ currentResourceCountText }}</span>
          <div class="create-actions">
            <el-button
              v-if="activeTab === 'configmap'"
              type="success"
              :icon="Plus"
              class="primary-action"
              @click="handleCreateConfigMap"
            >
              创建 ConfigMap
            </el-button>
            <el-button
              v-else
              type="success"
              :icon="Plus"
              class="primary-action"
              @click="handleCreateSecret"
            >
              创建 Secret
            </el-button>
          </div>
        </div>

        <div v-if="activeTab === 'configmap'" class="tab-content">
          <el-table
            :data="paginatedConfigMapList"
            v-loading="loading"
            element-loading-text="加载中..."
            class="resource-table"
            empty-text="暂无 ConfigMap 资源"
          >
            <el-table-column prop="name" label="名称" min-width="180">
              <template #default="{ row }">
                <div class="resource-name">
                  <el-icon class="resource-icon"><Setting /></el-icon>
                  <span class="resource-name-link" @click="handleViewConfigMap(row)">{{ row?.name || '-' }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column prop="namespace" label="命名空间" width="130">
              <template #default="{ row }">
                <el-tag size="small" type="info">
                  {{ row?.namespace || '-' }}
                </el-tag>
              </template>
            </el-table-column>

            <el-table-column label="数据键" min-width="220">
              <template #default="{ row }">
                <div class="data-keys">
                  <span class="keys-text">{{ formatDataKeys(row?.data) }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column label="标签" min-width="170">
              <template #default="{ row }">
                <div class="labels-info">
                  <el-tag
                    v-for="(value, key) in (row?.labels || {})"
                    :key="key"
                    size="small"
                    type="primary"
                    class="label-tag"
                  >
                    {{ key }}={{ value }}
                  </el-tag>
                  <span v-if="!row?.labels || Object.keys(row?.labels).length === 0" class="no-labels">无标签</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column prop="createdTime" label="创建时间" width="180">
              <template #default="{ row }">
                <span class="datetime-text">{{ formatDateTime(row?.createdTime || row?.createdAt || row?.creationTimestamp) }}</span>
              </template>
            </el-table-column>

            <el-table-column label="操作" width="160" fixed="right">
              <template #default="{ row }">
                <div class="operation-buttons">
                  <button
                    type="button"
                    class="operation-text detail"
                    @click="handleViewConfigMap(row)"
                  >
                    详情
                  </button>
                  <el-dropdown trigger="click">
                    <button type="button" class="operation-text more">
                      更多
                    </button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item @click="handleEditConfigMapYaml(row)">编辑</el-dropdown-item>
                        <el-dropdown-item @click="handleDeleteConfigMap(row)">删除</el-dropdown-item>
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
            :data="paginatedSecretList"
            v-loading="loading"
            element-loading-text="加载中..."
            class="resource-table"
            empty-text="暂无 Secret 资源"
          >
            <el-table-column prop="name" label="名称" min-width="180">
              <template #default="{ row }">
                <div class="resource-name">
                  <el-icon class="resource-icon"><Key /></el-icon>
                  <span class="resource-name-link" @click="handleViewSecret(row)">{{ row?.name || '-' }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column prop="namespace" label="命名空间" width="130">
              <template #default="{ row }">
                <el-tag size="small" type="info">
                  {{ row?.namespace || '-' }}
                </el-tag>
              </template>
            </el-table-column>

            <el-table-column prop="type" label="类型" width="150">
              <template #default="{ row }">
                <el-tag
                  :type="getSecretTypeTag(row?.type)"
                  size="small"
                >
                  {{ formatSecretType(row?.type) }}
                </el-tag>
              </template>
            </el-table-column>

            <el-table-column label="数据键" min-width="220">
              <template #default="{ row }">
                <div class="data-keys">
                  <span class="keys-text">
                    <el-icon class="secret-icon"><Lock /></el-icon>
                    {{ formatDataKeys(row?.data) }}
                  </span>
                </div>
              </template>
            </el-table-column>

            <el-table-column label="标签" min-width="170">
              <template #default="{ row }">
                <div class="labels-info">
                  <el-tag
                    v-for="(value, key) in (row?.labels || {})"
                    :key="key"
                    size="small"
                    type="primary"
                    class="label-tag"
                  >
                    {{ key }}={{ value }}
                  </el-tag>
                  <span v-if="!row?.labels || Object.keys(row?.labels).length === 0" class="no-labels">无标签</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column prop="createdTime" label="创建时间" width="180">
              <template #default="{ row }">
                <span class="datetime-text">{{ formatDateTime(row?.createdTime || row?.createdAt || row?.creationTimestamp) }}</span>
              </template>
            </el-table-column>

            <el-table-column label="操作" width="160" fixed="right">
              <template #default="{ row }">
                <div class="operation-buttons">
                  <button
                    type="button"
                    class="operation-text detail"
                    @click="handleViewSecret(row)"
                  >
                    详情
                  </button>
                  <el-dropdown trigger="click">
                    <button type="button" class="operation-text more" >
                      更多
                    </button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item @click="handleEditSecretYaml(row)">编辑</el-dropdown-item>
                        <el-dropdown-item @click="handleDeleteSecret(row)">删除</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div v-if="currentResourceTotal > 0" class="pagination-wrapper">
          <el-pagination
            :current-page="currentPagination.page"
            :page-size="currentPagination.pageSize"
            :total="currentResourceTotal"
            :page-sizes="[10, 20, 50, 100]"
            layout="total, sizes, prev, pager, next, jumper"
            background
            @size-change="handleResourceSizeChange"
            @current-change="handleResourceCurrentChange"
          />
        </div>
      </section>
    </section>

    <!-- ConfigMap 详情对话框 -->
    <el-dialog
      v-model="configMapDetailDialogVisible"
      title="ConfigMap 详情"
      width="760px"
      class="detail-dialog"
      modal-class="k8s-config-dialog-overlay"
    >
      <div class="detail-content">
        <div class="detail-grid">
          <div class="detail-item">
            <span class="detail-label">名称</span>
            <span class="detail-value">{{ currentConfigMapForDetail.name || '-' }}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">命名空间</span>
            <span class="detail-value">{{ currentConfigMapForDetail.namespace || '-' }}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">创建时间</span>
            <span class="detail-value">{{ formatDateTime(currentConfigMapForDetail.createdTime || currentConfigMapForDetail.createdAt || currentConfigMapForDetail.creationTimestamp) }}</span>
          </div>
          <div class="detail-item detail-item-wide">
            <span class="detail-label">标签</span>
            <div class="labels-info">
              <el-tag
                v-for="(value, key) in (currentConfigMapForDetail.labels || {})"
                :key="key"
                size="small"
                type="primary"
                class="label-tag"
              >
                {{ key }}={{ value }}
              </el-tag>
              <span v-if="!currentConfigMapForDetail.labels || Object.keys(currentConfigMapForDetail.labels).length === 0" class="no-labels">无标签</span>
            </div>
          </div>
        </div>

        <div v-if="currentConfigMapForDetail.data" class="data-section">
          <h4 class="section-title">配置数据</h4>
          <div class="data-list">
            <div
              v-for="(value, key) in currentConfigMapForDetail.data"
              :key="key"
              class="data-item"
            >
              <div class="data-key">{{ key }}</div>
              <pre class="data-value">{{ value }}</pre>
            </div>
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- Secret 详情对话框 -->
    <el-dialog
      v-model="secretDetailDialogVisible"
      title="Secret 详情"
      width="760px"
      class="detail-dialog"
      modal-class="k8s-config-dialog-overlay"
    >
      <div class="detail-content">
        <div class="detail-grid">
          <div class="detail-item">
            <span class="detail-label">名称</span>
            <span class="detail-value">{{ currentSecretForDetail.name || '-' }}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">命名空间</span>
            <span class="detail-value">{{ currentSecretForDetail.namespace || '-' }}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">类型</span>
            <el-tag
              :type="getSecretTypeTag(currentSecretForDetail.type)"
              size="small"
            >
              {{ formatSecretType(currentSecretForDetail.type) }}
            </el-tag>
          </div>
          <div class="detail-item">
            <span class="detail-label">创建时间</span>
            <span class="detail-value">{{ formatDateTime(currentSecretForDetail.createdTime || currentSecretForDetail.createdAt || currentSecretForDetail.creationTimestamp) }}</span>
          </div>
          <div class="detail-item detail-item-wide">
            <span class="detail-label">标签</span>
            <div class="labels-info">
              <el-tag
                v-for="(value, key) in (currentSecretForDetail.labels || {})"
                :key="key"
                size="small"
                type="primary"
                class="label-tag"
              >
                {{ key }}={{ value }}
              </el-tag>
              <span v-if="!currentSecretForDetail.labels || Object.keys(currentSecretForDetail.labels).length === 0" class="no-labels">无标签</span>
            </div>
          </div>
        </div>

        <div v-if="currentSecretForDetail.data" class="data-section">
          <h4 class="section-title">
            <el-icon class="secret-icon"><Lock /></el-icon>
            敏感数据 (已脱敏)
          </h4>
          <div class="data-list">
            <div
              v-for="(value, key) in currentSecretForDetail.data"
              :key="key"
              class="data-item secret-item"
            >
              <div class="data-key">{{ key }}</div>
              <div class="data-value secret-value">
                <el-icon><Lock /></el-icon>
                *** ({{ String(value).length }} 字符)
              </div>
            </div>
          </div>
        </div>
      </div>
    </el-dialog>

    <!-- ConfigMap 编辑对话框 -->
    <ConfigMapEditorDialog
      :visible="configMapYamlDialogVisible"
      :yaml-content="currentConfigMapYaml"
      :resource-name="currentConfigMapForDetail.name || 'new-configmap'"
      :resource-type="'ConfigMap'"
      :editable="true"
      @update:visible="configMapYamlDialogVisible = $event"
      @close="configMapYamlDialogVisible = false"
      @save="handleConfigMapYamlSave"
    />

    <!-- Secret YAML 编辑对话框 -->
    <PodYamlDialog
      :visible="secretYamlDialogVisible"
      :yaml-content="currentSecretYaml"
      :resource-name="currentSecretForDetail.name || 'new-secret'"
      :resource-type="'Secret'"
      :editable="true"
      dialog-width="900px"
      editor-height="430px"
      @update:visible="secretYamlDialogVisible = $event"
      @close="secretYamlDialogVisible = false"
      @save="handleSecretYamlSave"
    />
  </div>
</template>

<style scoped>
.k8s-config-management {
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
  --config-page-height-offset: 190px;
  min-height: calc(100vh - var(--config-page-height-offset));
  padding: 6px;
  box-sizing: border-box;
  background:
    radial-gradient(circle at 16% 10%, rgba(76, 139, 245, 0.06), transparent 26%),
    linear-gradient(180deg, #f7f9fc 0%, var(--aiops-bg-page) 100%);
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.config-card {
  position: relative;
  display: flex;
  flex-direction: column;
  max-height: calc(100vh - 12px - var(--config-page-height-offset));
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

.config-card::before {
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

.config-type-section {
  margin-top: 12px;
  padding-top: 10px;
  border-top: 1px solid rgba(223, 231, 242, 0.72);
}

.config-tabs :deep(.el-tabs__header) {
  margin: 0;
}

.config-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: rgba(223, 231, 242, 0.78);
}

.config-tabs :deep(.el-tabs__item) {
  height: 34px;
  padding: 0 14px;
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

.config-tabs :deep(.el-tabs__item.is-active) {
  color: var(--aiops-brand);
}

.config-tabs :deep(.el-tabs__active-bar) {
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

.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  padding: 14px 4px 0;
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

.data-keys {
  display: flex;
  align-items: center;
  gap: 6px;
}

.keys-text {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  font-weight: 600;
}

.secret-icon {
  color: var(--aiops-warning);
  font-size: 14px;
}

.labels-info {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  align-items: center;
}

.label-tag {
  font-size: 11px;
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.no-labels {
  color: var(--aiops-text-muted);
  font-size: 12px;
  font-style: italic;
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
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px 18px;
}

.detail-item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  min-height: 44px;
  padding: 10px 0;
  border-bottom: 1px solid rgba(223, 231, 242, 0.68);
}

.detail-item-wide {
  grid-column: 1 / -1;
}

.detail-label {
  min-width: 72px;
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

.data-section {
  margin-top: 2px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.9);
  color: var(--aiops-text-primary);
  font-size: 16px;
  font-weight: 700;
}

.data-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.data-item {
  background: rgba(255, 255, 255, 0.84);
  border-radius: 16px;
  padding: 12px;
  border: 1px solid rgba(223, 231, 242, 0.9);
}

.data-item.secret-item {
  background: rgba(255, 248, 236, 0.9);
  border-color: rgba(243, 178, 75, 0.34);
}

.data-key {
  font-weight: 700;
  color: var(--aiops-brand);
  margin-bottom: 8px;
  font-size: 13px;
}

.data-value {
  margin: 0;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.5;
  background: transparent;
  white-space: pre-wrap;
  word-break: break-word;
}

.data-value.secret-value {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--aiops-warning);
  font-weight: 600;
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

.k8s-config-management .el-button:hover {
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
  .k8s-config-management {
    --config-page-height-offset: 131px;
    padding: 4px;
  }

  .config-card {
    max-height: calc(100vh - 8px - var(--config-page-height-offset));
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

  .detail-grid {
    grid-template-columns: 1fr;
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

  .data-list {
    gap: 8px;
  }

  .data-item {
    padding: 8px;
  }

  .labels-info {
    gap: 2px;
  }

  .label-tag {
    max-width: 100px;
  }
}

/* DESIGN.md compact workbench overrides */
.k8s-config-management {
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

.config-card {
  height: auto;
  max-height: none;
  padding: 10px;
}

.config-card::before {
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

.search-name-input {
  width: 300px;
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
  border: 0;
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.search-name-input :deep(.el-input__wrapper.is-focus) {
  border: 0;
  background: #fff;
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

.refresh-button:hover,
.refresh-button:focus {
  border-color: #e8b94f;
  background: #e8b94f;
  color: #fff;
}

.primary-action {
  min-width: 118px;
}

.k8s-config-management .el-button:hover,
.k8s-config-management .el-button:focus,
.resource-name-link:hover {
  transform: none;
}

.config-type-section {
  margin-top: 6px;
  padding-top: 6px;
  border-top-color: var(--ops-border-soft);
}

.config-tabs :deep(.el-tabs__item) {
  height: 32px;
  padding: 0 12px;
  font-size: 12px;
}

.table-section {
  flex: 1;
  min-height: 0;
  padding: 0;
  overflow: auto;
  border: 1px solid #e3ebf5;
  border-radius: 10px;
  background: #fff;
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

.tab-content {
  overflow-x: auto;
}

.resource-table {
  min-width: 900px;
  overflow: hidden;
  border: 0;
  border-radius: 9px;
  background: #fff;
}

.resource-table :deep(.el-table__header th),
.resource-table :deep(.el-table__fixed-right-patch) {
  height: 38px;
  padding: 6px 0;
  border-bottom: 1px solid #e3ebf5;
  background: #f3f7fc !important;
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
}

.keys-text,
.datetime-text {
  color: #64748b;
  font-size: 11px;
  font-weight: 500;
}

.datetime-text {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
}

.labels-info {
  flex-wrap: nowrap;
  max-width: 100%;
  overflow: hidden;
}

.label-tag {
  flex: 0 1 auto;
  border-radius: 5px;
}

.resource-table :deep(.el-tag) {
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

.pagination-wrapper {
  padding: 8px 10px;
  border-top: 1px solid #eef3f9;
}

.pagination-wrapper :deep(.el-pagination) {
  --el-pagination-button-width: 28px;
  --el-pagination-button-height: 28px;
  font-size: 12px;
}

.detail-content {
  gap: 8px;
}

.detail-grid {
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

.section-title {
  gap: 6px;
  margin-bottom: 8px;
  padding-bottom: 6px;
  font-size: 14px;
}

.data-list {
  gap: 8px;
}

.data-item {
  padding: 8px 10px;
  border-color: #dce8f5;
  border-radius: 8px;
  background: #f8fbff;
}

.data-key {
  margin-bottom: 5px;
  font-size: 12px;
}

.data-value {
  font-size: 12px;
}

:global(.k8s-config-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 24px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: 15px !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.k8s-config-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: linear-gradient(90deg, #fff6e7 0%, #f7fbff 36%, #ecf7ff 100%);
}

:global(.k8s-config-dialog-overlay .el-dialog__title) {
  color: #172033;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.k8s-config-dialog-overlay .el-dialog__headerbtn) {
  top: 12px;
  right: 14px;
  width: 32px;
  height: 32px;
}

:global(.k8s-config-dialog-overlay .el-dialog__body) {
  max-height: calc(100vh - 190px);
  overflow: auto;
  padding: 14px 18px;
  background: #fff;
}

:global(.k8s-config-dialog-overlay .el-button) {
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
  .k8s-config-management {
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
  .config-card {
    max-height: none;
    padding: 8px;
    border-radius: 12px;
  }

  .search-form :deep(.el-form-item) {
    margin-bottom: 8px;
  }

  .search-name-input {
    width: 100%;
  }

  .query-button,
  .refresh-button,
  .primary-action {
    width: 100% !important;
  }

  .content-header {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
