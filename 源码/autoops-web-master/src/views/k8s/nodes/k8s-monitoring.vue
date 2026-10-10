<template>
  <div class="k8s-monitoring-dashboard">
    <!-- 页面头部 -->
    <div class="page-header">
      <div class="header-title">
        <h2>K8s 监控仪表板</h2>
        <p class="header-desc">实时监控集群资源使用情况</p>
      </div>
      <div class="header-controls">
        <el-select v-model="selectedClusterId" placeholder="选择集群" @change="handleClusterChange">
          <el-option 
            v-for="cluster in clusterList" 
            :key="cluster.id" 
            :label="cluster.name" 
            :value="cluster.id"
          />
        </el-select>
        <el-button :icon="Refresh" @click="refreshAllData" :loading="loading">刷新数据</el-button>
      </div>
    </div>

    <!-- 监控标签页 -->
    <el-tabs v-model="activeTab" @tab-change="handleTabChange">
      <!-- 节点监控 -->
      <el-tab-pane label="节点监控" name="nodes">
        <NodesMonitoring 
          :selected-cluster-id="selectedClusterId"
          ref="nodesMonitoringRef"
        />
      </el-tab-pane>

      <!-- 命名空间监控 -->
      <el-tab-pane label="命名空间监控" name="namespaces">
        <NamespaceMonitoring 
          :selected-cluster-id="selectedClusterId"
          ref="namespaceMonitoringRef"
        />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup>
import { ref, reactive, onBeforeUnmount, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  Refresh,
  View,
  Monitor
} from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'
import NodesMonitoring from './NodesMonitoring.vue'
import NamespaceMonitoring from './NamespaceMonitoring.vue'
import {
  getStoredClusterId,
  saveStoredClusterId,
  resolvePreferredClusterId,
  subscribeStoredClusterId
} from '@/utils/k8sClusterSelection'

const router = useRouter()
const CLUSTER_SELECTION_SOURCE = 'k8s-monitoring'

// 响应式数据
const loading = ref(false)
const activeTab = ref('nodes')
const selectedClusterId = ref(getStoredClusterId())
const clusterList = ref([])
let unsubscribeClusterSelection = null

// 组件引用
const nodesMonitoringRef = ref(null)
const namespaceMonitoringRef = ref(null)

// 工具函数
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

const getProgressColor = (percentage) => {
  if (percentage >= 90) return '#f56c6c'
  if (percentage >= 75) return '#e6a23c'
  if (percentage >= 60) return '#409eff'
  return '#67c23a'
}

// API调用函数
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
    } else {
      ElMessage.error(responseData.message || '获取集群列表失败')
    }
  } catch (error) {
    console.error('获取集群列表失败:', error)
    ElMessage.warning('无法获取集群列表，请检查后端服务')
  }
}


// 事件处理函数
const handleClusterChange = async () => {
  if (selectedClusterId.value) {
    saveStoredClusterId(selectedClusterId.value, CLUSTER_SELECTION_SOURCE)
    await loadTabData()
    // 通知组件集群变更
    if (nodesMonitoringRef.value) {
      nodesMonitoringRef.value.handleClusterChange()
    }
    if (namespaceMonitoringRef.value) {
      namespaceMonitoringRef.value.handleClusterChange()
    }
  }
}

const handleTabChange = async (tabName) => {
  activeTab.value = tabName
  await loadTabData()
}

const loadTabData = async () => {
  if (!selectedClusterId.value) return
  
  if (activeTab.value === 'nodes') {
    // 节点监控由 NodesMonitoring 组件处理
    if (nodesMonitoringRef.value) {
      nodesMonitoringRef.value.refreshAllNodes()
    }
  } else if (activeTab.value === 'namespaces') {
    // 命名空间监控由 NamespaceMonitoring 组件处理
    if (namespaceMonitoringRef.value) {
      namespaceMonitoringRef.value.refreshAllData()
    }
  }
}

const refreshAllData = async () => {
  loading.value = true
  try {
    if (activeTab.value === 'nodes' && nodesMonitoringRef.value) {
      await nodesMonitoringRef.value.refreshAllNodes()
    } else if (activeTab.value === 'namespaces' && namespaceMonitoringRef.value) {
      await namespaceMonitoringRef.value.refreshAllData()
    } else {
      await loadTabData()
    }
    ElMessage.success('数据刷新成功')
  } catch (error) {
    ElMessage.error('数据刷新失败')
  } finally {
    loading.value = false
  }
}


// 组件挂载时初始化
onMounted(async () => {
  try {
    unsubscribeClusterSelection = subscribeStoredClusterId(async (clusterId, detail = {}) => {
      if (detail.source === CLUSTER_SELECTION_SOURCE || String(selectedClusterId.value || '') === String(clusterId || '')) {
        return
      }

      selectedClusterId.value = clusterId
      if (selectedClusterId.value) {
        await loadTabData()
      }
    })

    await fetchClusterList()
    if (selectedClusterId.value) {
      await loadTabData()
    }
  } catch (error) {
    console.error('页面初始化失败:', error)
  }
})

onBeforeUnmount(() => {
  if (unsubscribeClusterSelection) {
    unsubscribeClusterSelection()
  }
})
</script>

<style scoped>
.k8s-monitoring-dashboard {
  --aiops-bg-page: #f4f7fb;
  --aiops-text-primary: #172133;
  --aiops-text-secondary: #5f6b7c;
  --aiops-text-muted: #8d97a8;
  --aiops-border-cool: #dfe7f2;
  --aiops-brand: #4c8bf5;
  --aiops-shadow-soft: 0 20px 60px rgba(17, 24, 39, 0.07);
  padding: 6px;
  background:
    radial-gradient(circle at 16% 10%, rgba(76, 139, 245, 0.06), transparent 26%),
    linear-gradient(180deg, #f7f9fc 0%, var(--aiops-bg-page) 100%);
  min-height: 100vh;
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 18px;
  margin-bottom: 12px;
  padding: 16px 18px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 24px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  box-shadow: var(--aiops-shadow-soft);
}

.header-title h2 {
  margin: 0 0 4px 0;
  font-size: 24px;
  font-weight: 700;
  color: var(--aiops-text-primary);
}

.header-desc {
  margin: 0;
  color: var(--aiops-text-secondary);
  font-size: 14px;
}

.header-controls {
  display: flex;
  gap: 12px;
  align-items: center;
}

.header-controls .el-select {
  width: 200px;
}

.header-controls :deep(.el-input__wrapper) {
  min-height: 32px;
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 13px;
  background: rgba(252, 253, 255, 0.98);
  box-shadow: none;
}

.header-controls :deep(.el-button) {
  height: 32px;
  border-radius: 13px;
  border-color: rgba(214, 223, 235, 0.96);
  background: rgba(255, 255, 255, 0.92);
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

.k8s-monitoring-dashboard :deep(.el-tabs) {
  padding: 10px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 24px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.95), rgba(247, 250, 255, 0.92));
  box-shadow: var(--aiops-shadow-soft);
}

.k8s-monitoring-dashboard :deep(.el-tabs__header) {
  margin-bottom: 12px;
}

.k8s-monitoring-dashboard :deep(.el-tabs__item) {
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

.k8s-monitoring-dashboard :deep(.el-tabs__item.is-active) {
  color: var(--aiops-brand);
}

.k8s-monitoring-dashboard :deep(.el-tabs__active-bar) {
  background-color: var(--aiops-brand);
}

@media (max-width: 900px) {
  .page-header {
    flex-direction: column;
    align-items: stretch;
  }

  .header-controls {
    flex-direction: column;
    align-items: stretch;
  }

  .header-controls .el-select {
    width: 100%;
  }
}

</style>
