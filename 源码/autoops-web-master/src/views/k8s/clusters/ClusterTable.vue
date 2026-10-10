<template>
  <div class="cluster-table-container">
    <!-- 搜索表单 -->
    <div class="search-section">
      <el-form :inline="true" :model="queryParams" class="search-form">
        <el-form-item label="集群名称">
          <el-select
            v-model="queryParams.clusterName"
            placeholder="请选择集群名称"
            clearable
            filterable
            size="small"
            class="search-control"
          >
            <el-option
              v-for="item in clusterNameOptions"
              :key="item.value"
              :label="item.label"
              :value="item.value"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="集群状态">
          <el-select
            v-model="queryParams.status"
            placeholder="请选择状态"
            clearable
            size="small"
            class="search-control"
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
          <el-button type="primary" :icon="Search" size="small" class="query-button" @click="handleQuery">
            查询
          </el-button>
          <el-button type="warning" :icon="Refresh" size="small" class="reset-button" @click="resetQuery">
            重置
          </el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 集群列表表格 -->
    <div class="table-section">
      <el-table
        :data="tableData"
        v-loading="loading"
        style="width: 100%"
        class="cluster-table"
      >
        <el-table-column prop="clusterName" label="集群名称" min-width="180">
          <template #default="{ row }">
            <div class="cluster-name-container">
              <img src="@/assets/image/k8s.svg" alt="k8s" class="k8s-icon" />
              <el-button
                type="primary"
                link
                @click="handleClusterNameClick(row)"
                class="cluster-name-link"
              >
                {{ row.clusterName }}
              </el-button>
              <el-tooltip content="复制集群名称" placement="top">
                <el-button
                  type="info"
                  :icon="DocumentCopy"
                  size="small"
                  circle
                  @click.stop="copyClusterName(row)"
                  class="copy-name-btn"
                />
              </el-tooltip>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="version" label="集群版本" min-width="140">
          <template #default="{ row }">
            <div class="version-container">
              <span class="version-text">{{ row.version }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="nodeCount" label="节点数量" min-width="110">
          <template #default="{ row }">
            <el-tag size="small" type="info">
              {{ row.nodeCount }} 个
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="集群状态" min-width="110">
          <template #default="{ row }">
            <el-tag
              :type="getStatusTag(row.status)"
              size="small"
              effect="plain"
              class="status-tag"
            >
              {{ getStatusText(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="monitorDatasourceName" label="监控源" min-width="170">
          <template #default="{ row }">
            <div class="monitor-source-cell">
              <el-tag v-if="row.monitorDatasourceName" size="small" effect="light">
                {{ row.monitorDatasourceName }}
              </el-tag>
              <el-tag v-else size="small" type="info" effect="plain">未绑定</el-tag>
              <span v-if="row.monitorDatasourceType" class="monitor-source-type">
                {{ row.monitorDatasourceType }}
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column
          label="集群凭证"
          width="120"
          align="left"
          header-align="left"
          class-name="credential-column"
          label-class-name="credential-column-header"
        >
          <template #default="{ row }">
            <div class="credential-cell" :class="{ 'is-empty': !(row.credential && row.credential.trim()) }">
              <template v-if="row.credential && row.credential.trim()">
                <span class="credential-action">
                  <el-button
                    type="primary"
                    size="small"
                    link
                    class="credential-view-button"
                    @click="viewCredential(row)"
                  >
                    查看
                  </el-button>
                </span>
              </template>
              <el-tag v-else size="small" type="info">未设置</el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="备注信息" min-width="160">
          <template #default="{ row }">
            <div class="description-container">
              <el-tooltip 
                :content="row.description || '无'"
                placement="top"
                :disabled="!row.description || row.description.length <= 20"
              >
                <span class="description-text">
                  {{ row.description ? (row.description.length > 20 ? row.description.substring(0, 20) + '...' : row.description) : '无' }}
                </span>
              </el-tooltip>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="createTime" label="创建时间" min-width="180" />
        <el-table-column label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <div class="operation-buttons">
              <el-button
                link
                size="small"
                class="operation-text sync"
                :disabled="!row.credential || !row.credential.trim()"
                @click="handleSync(row)"
              >同步</el-button>
              <el-button
                link
                size="small"
                class="operation-text metrics"
                :loading="metricsCheckingClusterId === row.id"
                :disabled="!row.credential || !row.credential.trim()"
                @click="handleMetricsCheck(row)"
              >监控检查</el-button>
              <el-button
                link
                size="small"
                class="operation-text edit"
                @click="handleEdit(row)"
              >编辑</el-button>
              <el-button
                link
                size="small"
                class="operation-text delete"
                @click="handleDelete(row)"
              >删除</el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-dialog
      v-model="metricsCheckVisible"
      title="监控组件检查"
      width="820px"
      class="metrics-check-dialog"
      modal-class="k8s-metrics-dialog-overlay"
      destroy-on-close
    >
      <div v-loading="metricsCheckLoading" class="metrics-check-content">
        <div class="metrics-hero-panel">
          <div class="metrics-hero-copy">
            <span class="metrics-eyebrow">Metrics Components</span>
            <h3>{{ currentMetricsCluster.clusterName || 'K8s 集群' }}</h3>
            <p>
              {{ metricsCheckResult.allInstalled ? '监控组件已安装完成，可以查看实时和历史 CPU/内存指标。' : '存在未安装或不可用的监控组件，请按组件提示处理。' }}
            </p>
          </div>
          <el-tag
            :type="metricsCheckResult.allInstalled ? 'success' : 'warning'"
            effect="light"
            round
            class="metrics-result-tag"
          >
            {{ metricsCheckResult.allInstalled ? '整体通过' : '需要处理' }}
          </el-tag>
        </div>

        <div class="metrics-table-shell">
          <el-table
            :data="metricsCheckResult.components"
            class="metrics-components-table"
            size="small"
          >
            <el-table-column prop="displayName" label="组件" min-width="150">
              <template #default="{ row }">
                <span class="component-name">{{ row.displayName || row.name }}</span>
              </template>
            </el-table-column>
            <el-table-column label="安装状态" width="104" align="center">
              <template #default="{ row }">
                <el-tag :type="row.installed ? 'success' : 'danger'" effect="light" round>
                  {{ row.installed ? '已安装' : '未安装' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="可用状态" width="104" align="center">
              <template #default="{ row }">
                <el-tag :type="row.available ? 'success' : 'warning'" effect="light" round>
                  {{ row.available ? '可用' : '不可用' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="说明" min-width="260">
              <template #default="{ row }">
                <div class="component-description">
                  <span>{{ row.message || row.description || '-' }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="132" align="center">
              <template #default="{ row }">
                <div class="component-actions">
                  <el-button
                    v-if="row.installCommand"
                    link
                    size="small"
                    type="primary"
                    @click="copyInstallCommand(row)"
                  >
                    复制命令
                  </el-button>
                  <el-link
                    v-if="row.installUrl"
                    type="primary"
                    :href="row.installUrl"
                    target="_blank"
                    :underline="false"
                  >
                    去安装
                  </el-link>
                  <span v-if="!row.installCommand && !row.installUrl" class="empty-action">-</span>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div class="metrics-dialog-footer">
          <el-button size="small" class="metrics-close-button" @click="metricsCheckVisible = false">
            关闭
          </el-button>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Search,
  Refresh,
  DocumentCopy
} from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'

const router = useRouter()

const props = defineProps({
  refreshTrigger: {
    type: Number,
    default: 0
  }
})

const emit = defineEmits(['cluster-sync', 'cluster-edit', 'cluster-delete', 'view-credential'])

const loading = ref(false)
const metricsCheckLoading = ref(false)
const metricsCheckVisible = ref(false)
const metricsCheckingClusterId = ref(null)
const currentMetricsCluster = reactive({
  id: '',
  clusterName: ''
})
const metricsCheckResult = reactive({
  clusterId: '',
  allInstalled: false,
  components: []
})
const queryParams = reactive({
  clusterName: '',
  status: ''
})

const statusOptions = [
  { label: '全部', value: '' },
  { label: '创建中', value: 1 },
  { label: '运行中', value: 2 },
  { label: '离线', value: 3 }
]

const tableData = ref([])
const clusterNameOptions = ref([])

const mapClusterRow = (cluster) => ({
  id: cluster.id,
  clusterName: cluster.name,
  version: cluster.version || '',
  nodeCount: cluster.nodeCount || 0,
  status: getClusterStatus(cluster.status),
  statusCode: cluster.status,
  credential: cluster.credential || '',  // 保留原始凭证数据
  monitorDatasourceId: cluster.monitorDatasourceId ?? '',
  monitorDatasourceType: cluster.monitorDatasourceType || '',
  monitorDatasourceName: cluster.monitorDatasourceName || '',
  description: cluster.description || cluster.remark || '', // 备注信息
  createTime: cluster.createdAt ? new Date(cluster.createdAt).toLocaleString() : cluster.createTime
})

const updateClusterNameOptions = (clusters) => {
  const options = clusters
    .map(cluster => cluster.name)
    .filter(Boolean)
    .map(name => ({ label: name, value: name }))

  clusterNameOptions.value = Array.from(
    new Map(options.map(option => [option.value, option])).values()
  )
}

const filterClusterRows = (rows) => {
  const clusterName = queryParams.clusterName?.trim().toLowerCase()
  const status = queryParams.status

  return rows.filter(row => {
    const matchName = !clusterName || row.clusterName?.toLowerCase().includes(clusterName)
    const matchStatus = !status || row.statusCode === status
    return matchName && matchStatus
  })
}

const handleQuery = async () => {
  try {
    loading.value = true
    
    const response = await k8sApi.getClusterList({ size: 100 })
    
    // response可能是axios响应对象，需要获取data部分
    const responseData = response.data || response
    console.log('API响应数据:', responseData)
    
    if (responseData.code === 200 || responseData.success) {
      // 根据实际API响应结构处理数据
      const clusters = responseData.data?.list || responseData.data || []
      const rows = clusters.map(mapClusterRow)
      updateClusterNameOptions(clusters)
      tableData.value = filterClusterRows(rows)
      
      console.log('集群列表加载成功:', tableData.value)
    } else {
      ElMessage.error(responseData.message || '获取集群列表失败')
      tableData.value = []
    }
  } catch (error) {
    console.error('获取集群列表失败:', error)
    
    // 检查是否是网络连接问题
    if (error.code === 'ERR_NETWORK' || 
        error.message?.includes('ERR_CONNECTION_REFUSED') ||
        error.message?.includes('Failed to fetch')) {
      ElMessage.warning('后端服务连接失败，请检查服务状态')
    } else if (error.response?.status === 401) {
      ElMessage.error('认证失败，请重新登录')
    } else if (error.response?.status === 403) {
      ElMessage.error('权限不足，请联系管理员')
    } else {
      // 只在真正异常时才显示错误
      console.warn('API调用异常，但可能数据已正确加载')
    }
    
    // 设置空数据，避免界面显示异常
    tableData.value = []
  } finally {
    loading.value = false
  }
}

const getClusterStatus = (statusCode) => {
  const statusMap = {
    1: 'creating',
    2: 'online', 
    3: 'offline'
  }
  return statusMap[statusCode] || 'unknown'
}

const resetQuery = () => {
  queryParams.clusterName = ''
  queryParams.status = ''
  // 重置后自动刷新数据
  handleQuery()
}

const handleClusterNameClick = (row) => {
  // 跳转到集群详情页面
  router.push(`/k8s/cluster/${row.id}`)
}

// 复制集群名称
const copyClusterName = async (row) => {
  try {
    await navigator.clipboard.writeText(row.clusterName)
    ElMessage.success(`集群名称 "${row.clusterName}" 已复制到剪贴板`)
  } catch (error) {
    console.error('复制失败:', error)
    // 降级处理：使用传统方式复制
    try {
      const textArea = document.createElement('textarea')
      textArea.value = row.clusterName
      textArea.style.position = 'fixed'
      textArea.style.opacity = '0'
      document.body.appendChild(textArea)
      textArea.select()
      document.execCommand('copy')
      document.body.removeChild(textArea)
      ElMessage.success(`集群名称 "${row.clusterName}" 已复制到剪贴板`)
    } catch (fallbackError) {
      console.error('降级复制也失败:', fallbackError)
      ElMessage.error('复制失败，请手动复制')
    }
  }
}

const copyTextToClipboard = async (text, successMessage = '已复制到剪贴板') => {
  if (!text) {
    ElMessage.warning('没有内容可以复制')
    return
  }

  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success(successMessage)
  } catch (error) {
    try {
      const textArea = document.createElement('textarea')
      textArea.value = text
      textArea.style.position = 'fixed'
      textArea.style.opacity = '0'
      document.body.appendChild(textArea)
      textArea.select()
      document.execCommand('copy')
      document.body.removeChild(textArea)
      ElMessage.success(successMessage)
    } catch (fallbackError) {
      console.error('复制失败:', fallbackError)
      ElMessage.error('复制失败，请手动复制')
    }
  }
}

const copyInstallCommand = (component) => {
  copyTextToClipboard(component.installCommand, `${component.displayName || component.name} 安装命令已复制`)
}

const resetMetricsCheckResult = () => {
  metricsCheckResult.clusterId = ''
  metricsCheckResult.allInstalled = false
  metricsCheckResult.components = []
}

const handleMetricsCheck = async (row) => {
  if (!row.credential || !row.credential.trim()) {
    ElMessage.warning('该集群没有配置凭证，无法检查监控组件')
    return
  }

  currentMetricsCluster.id = row.id
  currentMetricsCluster.clusterName = row.clusterName
  resetMetricsCheckResult()
  metricsCheckVisible.value = true
  metricsCheckLoading.value = true
  metricsCheckingClusterId.value = row.id

  try {
    const response = await k8sApi.checkMetricsComponents(row.id)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      const data = responseData.data || {}
      metricsCheckResult.clusterId = data.clusterId || row.id
      metricsCheckResult.allInstalled = Boolean(data.allInstalled)
      metricsCheckResult.components = Array.isArray(data.components) ? data.components : []

      if (metricsCheckResult.allInstalled) {
        ElMessage.success(`集群 ${row.clusterName} 监控组件检查通过`)
      } else {
        ElMessage.warning(`集群 ${row.clusterName} 存在未安装或不可用的监控组件`)
      }
    } else {
      ElMessage.error(responseData.message || '监控组件检查失败')
      metricsCheckVisible.value = false
    }
  } catch (error) {
    console.error('监控组件检查失败:', error)
    ElMessage.error('监控组件检查失败，请检查网络连接或集群凭证')
    metricsCheckVisible.value = false
  } finally {
    metricsCheckLoading.value = false
    metricsCheckingClusterId.value = null
  }
}

const handleSync = async (row) => {
  try {
    // 检查是否有集群凭证
    if (!row.credential || !row.credential.trim()) {
      ElMessage.warning('该集群没有配置凭证，无法进行同步操作')
      return
    }
    
    const response = await k8sApi.syncCluster(row.id)
    
    // response可能是axios响应对象，需要获取data部分
    const responseData = response.data || response
    console.log('同步集群API响应:', responseData)
    
    if (responseData.code === 200 || responseData.success || responseData.message === '成功') {
      ElMessage.success(`集群 ${row.clusterName} 同步成功`)
      // 触发父组件刷新
      emit('cluster-sync', row)
      // 立即刷新一次
      await handleQuery()
    } else if (responseData.code === 500) {
      // 处理超时或连接失败的情况
      const isTimeoutError = responseData.message && (
        responseData.message.includes('timeout') ||
        responseData.message.includes('Timeout exceeded') ||
        responseData.message.includes('connection') ||
        responseData.message.includes('已将集群状态设为离线')
      )
      
      if (isTimeoutError) {
        ElMessage.warning(`集群 ${row.clusterName} 连接超时，已设为离线状态`)
        console.log('检测到超时错误，准备延迟刷新状态')
        
        // 立即刷新一次，可能状态还没更新
        await handleQuery()
        
        // 延迟刷新，确保后端状态已更新
        setTimeout(async () => {
          console.log('延迟刷新集群状态')
          await handleQuery()
        }, 1000) // 1秒后再次刷新
      } else {
        ElMessage.error(responseData.message || '同步失败')
      }
      
      // 触发父组件刷新
      emit('cluster-sync', row)
    } else {
      ElMessage.error(responseData.message || '同步失败')
      // 即使失败也要刷新，可能状态有变化
      await handleQuery()
      emit('cluster-sync', row)
    }
  } catch (error) {
    console.error('同步集群失败:', error)
    
    // 检查错误信息是否包含超时相关内容
    const errorMessage = error.message || error.toString()
    const isNetworkTimeout = errorMessage.includes('timeout') || 
                            errorMessage.includes('Network Error') ||
                            errorMessage.includes('ERR_NETWORK')
    
    if (isNetworkTimeout) {
      ElMessage.warning(`集群 ${row.clusterName} 网络连接超时`)
      // 网络超时也要延迟刷新状态
      setTimeout(async () => {
        console.log('网络超时后延迟刷新')
        await handleQuery()
      }, 1000)
    } else {
      ElMessage.error('同步集群失败，请检查网络连接')
    }
    
    emit('cluster-sync', row)
  }
}

const handleEdit = async (row) => {
  emit('cluster-edit', row)
}

const viewCredential = (row) => {
  emit('view-credential', row)
}

const handleDelete = (row) => {
  ElMessageBox.confirm(
    `确定要删除集群 ${row.clusterName} 吗？此操作不可逆！`,
    '删除确认',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    }
  )
  .then(async () => {
    try {
      const response = await k8sApi.deleteCluster(row.id)
      
      // response可能是axios响应对象，需要获取data部分
      const responseData = response.data || response
      console.log('删除集群API响应:', responseData)
      
      if (responseData.code === 200 || responseData.success || responseData.message === '成功') {
        ElMessage.success(`集群 ${row.clusterName} 已删除`)
        emit('cluster-delete', row)
        handleQuery() // 刷新列表
      } else {
        ElMessage.error(responseData.message || '删除失败')
      }
    } catch (error) {
      console.error('删除集群失败:', error)
      ElMessage.error('删除集群失败，请检查网络连接')
    }
  })
  .catch(() => {
    ElMessage.info('已取消删除')
  })
}

const getStatusTag = (status) => {
  const tagMap = {
    'creating': 'info',
    'online': 'success',
    'offline': 'warning'
  }
  return tagMap[status] || 'info'
}

const getStatusText = (status) => {
  const textMap = {
    'creating': '创建中',
    'online': '运行中',
    'offline': '离线'
  }
  return textMap[status] || '未知'
}

// 监听刷新触发器
watch(() => props.refreshTrigger, (newVal, oldVal) => {
  console.log(`表格刷新触发器变化: ${oldVal} -> ${newVal}`)
  handleQuery()
})

// 暴露方法给父组件调用
defineExpose({
  handleQuery,
  resetQuery
})

onMounted(() => {
  handleQuery()
})
</script>

<style scoped>
.cluster-table-container {
  --aiops-surface-strong: rgba(255, 255, 255, 0.92);
  --aiops-surface-soft: rgba(247, 250, 255, 0.78);
  --aiops-text-primary: #172133;
  --aiops-text-secondary: #5f6b7c;
  --aiops-text-muted: #8d97a8;
  --aiops-border-cool: #dfe7f2;
  --aiops-border-mid: rgba(207, 217, 230, 0.92);
  --aiops-brand: #4c8bf5;
  --aiops-brand-soft: rgba(76, 139, 245, 0.12);
  --aiops-success: #3fb97f;
  --aiops-warning: #f3b24b;
  --aiops-danger: #ef6b73;
  position: relative;
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
  width: 100%;
  color: var(--aiops-text-primary);
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

.table-section {
  flex: 1;
  min-height: 0;
  padding: 8px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.64);
  overflow: auto;
}

.cluster-table {
  min-width: 1080px;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.88);
  border-radius: 16px;
  background: var(--aiops-surface-strong);
  box-shadow: none;
}

.cluster-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.cluster-table :deep(.el-table__header th),
.cluster-table :deep(.el-table__fixed-right-patch) {
  background: rgba(248, 250, 255, 0.98) !important;
  border-bottom: 1px solid rgba(223, 231, 242, 0.95);
}

.cluster-table :deep(.el-table__header th .cell) {
  color: var(--aiops-text-muted) !important;
  font-size: 12px;
  font-weight: 700 !important;
  text-shadow: none;
}

.cluster-table :deep(.el-table__cell) {
  border-bottom-color: rgba(15, 23, 42, 0.045);
}

.cluster-table :deep(.el-table__body tr) {
  transition: background-color 0.2s ease;
}

.cluster-table :deep(.el-table__body tr:hover > td.el-table__cell) {
  background: rgba(76, 139, 245, 0.045) !important;
}

.cluster-name-container {
  display: flex;
  align-items: center;
  gap: 9px;
  position: relative;
  min-width: 0;
}

.k8s-icon {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
}

.version-container {
  display: flex;
  align-items: center;
  gap: 8px;
}

.version-text {
  color: var(--aiops-text-secondary);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 13px;
  font-weight: 600;
}

.cluster-name-link {
  font-weight: 600;
  color: var(--aiops-brand);
  text-decoration: none;
  transition: color 0.2s ease;
}

.cluster-name-link:hover {
  color: #3f75df;
}

.copy-name-btn {
  opacity: 0;
  transform: scale(0.92);
  transition: opacity 0.2s ease, transform 0.2s ease, border-color 0.2s ease;
}

.cluster-name-container:hover .copy-name-btn {
  opacity: 1;
  transform: scale(1);
}

.copy-name-btn:hover {
  background-color: rgba(76, 139, 245, 0.1) !important;
  border-color: rgba(76, 139, 245, 0.3) !important;
  color: var(--aiops-brand) !important;
}

.operation-buttons {
  display: flex;
  gap: 14px;
  justify-content: center;
  align-items: center;
  flex-wrap: nowrap;
}

.operation-buttons :deep(.el-button) {
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

.operation-buttons :deep(.el-button:hover:not(.is-disabled)) {
  transform: none;
  box-shadow: none;
  text-decoration: none;
  opacity: 0.82;
}

.operation-buttons :deep(.operation-text.sync) {
  color: #4c8bf5;
}

.operation-buttons :deep(.operation-text.metrics) {
  color: #3fb97f;
}

.operation-buttons :deep(.operation-text.edit) {
  color: #f3a33a;
}

.operation-buttons :deep(.operation-text.delete) {
  color: #ef6b73;
}

.operation-buttons :deep(.operation-text.is-disabled) {
  color: var(--aiops-text-muted);
  opacity: 0.48;
}

.cluster-table :deep(.el-tag) {
  font-weight: 500;
  border-radius: 999px;
}

.status-tag {
  min-width: 56px;
  justify-content: center;
}

.monitor-source-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.monitor-source-cell :deep(.el-tag) {
  max-width: 118px;
}

.monitor-source-cell :deep(.el-tag__content) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.monitor-source-type {
  overflow: hidden;
  color: var(--aiops-text-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.credential-cell {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  width: 100%;
  min-height: 32px;
  text-align: left;
}

.credential-action {
  display: inline-flex;
  align-items: center;
  justify-content: flex-start;
}

.cluster-table :deep(.credential-column .cell),
.cluster-table :deep(.credential-column-header .cell) {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  width: 100%;
  text-align: left;
}

.cluster-table :deep(.credential-column .cell) {
  padding-left: 12px;
  padding-right: 12px;
}

.cluster-table :deep(td.credential-column),
.cluster-table :deep(th.credential-column-header) {
  text-align: left;
}

.credential-cell :deep(.el-tooltip__trigger),
.credential-cell :deep(.el-button),
.credential-cell :deep(.el-tag) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.credential-cell :deep(.credential-view-button) {
  height: 24px;
  margin: 0;
  padding: 0;
  line-height: 24px;
  border: none !important;
  background: transparent !important;
  box-shadow: none !important;
  outline: none !important;
}

.credential-cell :deep(.credential-view-button:hover),
.credential-cell :deep(.credential-view-button:focus),
.credential-cell :deep(.credential-view-button:focus-visible),
.credential-cell :deep(.credential-view-button:active),
.credential-cell :deep(.credential-view-button.is-focus) {
  border: none !important;
  background: transparent !important;
  box-shadow: none !important;
  outline: none !important;
}

.credential-cell :deep(.el-tag) {
  height: 24px;
  line-height: 22px;
}

.query-button {
  min-width: 76px;
  border: none;
  border-radius: 13px;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
  font-weight: 600;
  box-shadow: 0 10px 18px rgba(76, 139, 245, 0.16);
}

.query-button:hover,
.query-button:focus {
  transform: translateY(-1px);
  box-shadow: 0 12px 22px rgba(76, 139, 245, 0.2);
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

.search-control {
  width: 150px;
}

.search-form :deep(.el-input__wrapper) {
  min-height: 32px;
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 13px;
  background: rgba(252, 253, 255, 0.98);
  box-shadow: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;
}

.search-form :deep(.el-input__wrapper:hover) {
  border-color: rgba(171, 189, 220, 0.95);
}

.search-form :deep(.el-input__wrapper.is-focus) {
  border-color: rgba(120, 156, 221, 0.9);
  background: #ffffff;
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
}

.search-form :deep(.el-input__inner) {
  background: transparent;
  border: none;
  color: var(--aiops-text-primary);
}

.cluster-table-container :deep(.el-loading-mask) {
  background-color: rgba(103, 126, 234, 0.1);
  backdrop-filter: blur(4px);
}

.description-container {
  max-width: 160px;
}

.description-text {
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.4;
  display: inline-block;
  max-width: 100%;
  word-break: break-word;
  cursor: default;
}

.description-text:empty::before {
  content: '无';
  color: var(--aiops-text-muted);
  font-style: italic;
}

.metrics-check-dialog :deep(.el-dialog) {
  overflow: hidden;
  border: 1px solid rgba(207, 217, 230, 0.92);
  border-radius: 26px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.98) 0%, rgba(248, 248, 246, 0.98) 48%, rgba(237, 242, 253, 0.98) 100%);
  box-shadow: 0 24px 70px rgba(17, 24, 39, 0.16);
}

.metrics-check-dialog :deep(.el-dialog__header) {
  margin: 0;
  padding: 18px 22px 0;
}

.metrics-check-dialog :deep(.el-dialog__title) {
  color: var(--aiops-text-primary);
  font-size: 18px;
  font-weight: 700;
}

.metrics-check-dialog :deep(.el-dialog__headerbtn) {
  top: 15px;
  right: 18px;
  width: 30px;
  height: 30px;
  border-radius: 999px;
}

.metrics-check-dialog :deep(.el-dialog__headerbtn:hover) {
  background: rgba(76, 139, 245, 0.08);
}

.metrics-check-dialog :deep(.el-dialog__body) {
  padding: 16px 22px 20px;
}

.metrics-check-content {
  min-height: 248px;
}

.metrics-hero-panel {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  margin-bottom: 14px;
  padding: 18px 18px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  border-radius: 18px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.74);
}

.metrics-hero-copy {
  min-width: 0;
}

.metrics-eyebrow {
  display: inline-flex;
  margin-bottom: 7px;
  color: var(--aiops-brand);
  font-size: 12px;
  font-weight: 700;
}

.metrics-hero-copy h3 {
  margin: 0;
  color: var(--aiops-text-primary);
  font-size: 19px;
  font-weight: 700;
  line-height: 1.25;
}

.metrics-hero-copy p {
  margin: 7px 0 0;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.metrics-result-tag {
  flex-shrink: 0;
  margin-top: 2px;
  padding: 0 12px;
  font-weight: 700;
}

.metrics-table-shell {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.72);
}

.metrics-components-table {
  width: 100%;
  background: transparent;
}

.metrics-components-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.metrics-components-table :deep(.el-table__header th) {
  background: rgba(248, 250, 255, 0.96) !important;
  border-bottom: 1px solid rgba(223, 231, 242, 0.95);
}

.metrics-components-table :deep(.el-table__header th .cell) {
  color: var(--aiops-text-muted);
  font-size: 12px;
  font-weight: 700;
}

.metrics-components-table :deep(.el-table__cell) {
  border-bottom-color: rgba(15, 23, 42, 0.055);
}

.metrics-components-table :deep(.el-table__body tr:hover > td.el-table__cell) {
  background: rgba(76, 139, 245, 0.04) !important;
}

.metrics-components-table :deep(.el-tag) {
  min-width: 58px;
  justify-content: center;
  font-weight: 700;
}

.component-name {
  color: var(--aiops-text-primary);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 13px;
  font-weight: 700;
}

.component-description {
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.55;
}

.component-actions {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  width: 100%;
}

.component-actions :deep(.el-button),
.component-actions :deep(.el-link) {
  height: auto;
  min-height: 0;
  padding: 0;
  border: none;
  background: transparent;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.4;
}

.component-actions :deep(.el-button:hover),
.component-actions :deep(.el-link:hover) {
  opacity: 0.82;
}

.empty-action {
  color: var(--aiops-text-muted);
}

.metrics-dialog-footer {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.metrics-close-button {
  min-width: 92px;
  height: 32px;
  border: 1px solid rgba(214, 223, 235, 0.96);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.92);
  color: var(--aiops-text-secondary);
  font-weight: 700;
}

.metrics-close-button:hover,
.metrics-close-button:focus {
  border-color: rgba(153, 181, 230, 0.95);
  color: var(--aiops-brand);
  background: #ffffff;
}

@media (max-width: 768px) {
  .metrics-check-dialog :deep(.el-dialog) {
    width: calc(100vw - 24px) !important;
    border-radius: 22px;
  }

  .metrics-check-dialog :deep(.el-dialog__header) {
    padding: 16px 16px 0;
  }

  .metrics-check-dialog :deep(.el-dialog__body) {
    padding: 14px 16px 16px;
  }

  .metrics-hero-panel {
    flex-direction: column;
    gap: 12px;
    padding: 16px;
  }

  .metrics-result-tag {
    align-self: flex-start;
  }

  .metrics-table-shell {
    overflow-x: auto;
  }

  .metrics-components-table {
    min-width: 720px;
  }

  .metrics-dialog-footer,
  .metrics-close-button {
    width: 100%;
  }

  .search-section,
  .table-section {
    border-radius: 16px;
  }

  .search-form {
    display: block;
  }

  .search-form :deep(.el-form-item) {
    display: block;
    margin-right: 0;
    margin-bottom: 12px;
  }

  .search-form :deep(.el-form-item__content),
  .search-form :deep(.el-input),
  .search-form :deep(.el-select) {
    width: 100% !important;
  }

  .query-button,
  .reset-button {
    width: calc(50% - 4px);
  }
}

/* DESIGN.md: compact operational workspace overrides */
.cluster-table-container {
  --aiops-text-primary: #172033;
  --aiops-text-secondary: #53657d;
  --aiops-text-muted: #64748b;
  --aiops-border-cool: #dbe7f4;
  --aiops-brand: #2563eb;
}

.search-section {
  margin-bottom: 8px;
  padding: 8px 10px;
  border-color: #e3ebf5;
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

.search-form :deep(.el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

.search-form :deep(.el-input__wrapper),
.search-form :deep(.el-select__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 7px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.search-form :deep(.el-input__wrapper:hover),
.search-form :deep(.el-select__wrapper:hover) {
  border-color: transparent;
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.search-form :deep(.el-input__wrapper.is-focus),
.search-form :deep(.el-select__wrapper.is-focused) {
  border-color: transparent;
  box-shadow: 0 0 0 1px #409eff inset;
}

.query-button,
.reset-button {
  min-width: 72px;
  height: 32px;
  border-radius: 8px;
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  box-shadow: none;
  transition: background-color 160ms ease, border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease;
}

.query-button {
  border-color: #409eff;
  background: #409eff;
}

.reset-button {
  border-color: #e6a23c;
  background: #e6a23c;
}

.query-button:hover,
.query-button:focus,
.reset-button:hover,
.reset-button:focus {
  color: #fff;
  transform: translateY(-1px);
}

.table-section {
  padding: 0;
  border-color: #e3ebf5;
  border-radius: 10px;
  background: #fff;
}

.cluster-table {
  border: 0;
  border-radius: 9px;
}

.cluster-table :deep(.el-table__header th),
.cluster-table :deep(.el-table__fixed-right-patch) {
  height: 38px;
  background: #f3f7fc !important;
}

.cluster-table :deep(.el-table__header th .cell) {
  color: #475569 !important;
}

.cluster-table :deep(.el-table__cell) {
  height: 40px;
  padding: 5px 0;
  border-bottom-color: #eef3f9;
}

.cluster-table :deep(.el-table__body tr:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

.copy-name-btn {
  transform: none;
}

.cluster-name-container:hover .copy-name-btn {
  transform: none;
}

.operation-buttons {
  justify-content: flex-end;
  gap: 12px;
}

.cluster-table :deep(.el-tag) {
  border-radius: 999px;
  font-size: 11px;
}

:global(.k8s-metrics-dialog-overlay .el-dialog) {
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: 15px !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.k8s-metrics-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
}

:global(.k8s-metrics-dialog-overlay .el-dialog__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.k8s-metrics-dialog-overlay .el-dialog__body) {
  padding: 14px 18px 16px;
  background: #fbfdff;
}

.metrics-check-content {
  min-height: 0;
}

.metrics-hero-panel {
  gap: 12px;
  margin-bottom: 10px;
  padding: 10px 12px;
  border-color: #dce8f5;
  border-radius: 10px;
  background: #f6faff;
  box-shadow: none;
}

.metrics-eyebrow {
  margin-bottom: 3px;
  color: #64748b;
  font-size: 10px;
}

.metrics-hero-copy h3 {
  font-size: 16px;
}

.metrics-hero-copy p {
  margin-top: 4px;
  font-size: 12px;
}

.metrics-table-shell {
  border-color: #e3ebf5;
  border-radius: 9px;
  background: #fff;
}

.metrics-dialog-footer {
  margin-top: 12px;
}

.metrics-close-button {
  height: 32px;
  border-radius: 8px;
}

@media (max-width: 768px) {
  .search-section,
  .table-section {
    border-radius: 9px;
  }
}
</style>
