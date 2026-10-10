<template>
  <div class="nodes-monitoring">
    <el-row :gutter="20" style="margin-bottom: 20px;">
      <el-col :span="8" v-for="node in nodesList" :key="node.name">
        <div class="node-card" v-loading="nodesLoading">
          <!-- 卡片头部 -->
          <div class="card-header">
            <div class="node-info-header">
              <div class="node-icon">
                <img src="@/assets/image/k8s.svg" alt="k8s" class="k8s-icon" />
                <div class="pulse-ring"></div>
              </div>
              <div class="node-details">
                <h4>{{ node.name }}</h4>
                <el-tag 
                  :type="getNodeStatusTag(node.status)" 
                  size="small"
                  :class="['status-tag', node.status.toLowerCase()]"
                >
                  <i class="status-dot"></i>
                  {{ getNodeStatusText(node.status) }}
                </el-tag>
              </div>
            </div>
            <div class="node-ip">{{ node.name }}</div>
          </div>
          
          <div v-if="nodeMetrics[node.name]" class="node-metrics">
            <!-- CPU 使用率 -->
            <div class="metric-item">
              <div class="metric-header">
                <img src="@/assets/image/cpu.svg" alt="CPU" class="metric-icon" />
                <span class="metric-label">CPU使用率</span>
                <span class="metric-value" :class="getUsageLevelClass(getCpuUsageRate(nodeMetrics[node.name]))">
                  {{ getCpuUsageRate(nodeMetrics[node.name]) }}%
                </span>
              </div>
              <div class="metric-visual">
                <div
                  class="usage-donut cpu-donut"
                  :style="getDonutStyle(getCpuUsageRate(nodeMetrics[node.name]), 'cpu')"
                >
                  <div class="usage-donut-center">
                    <strong>{{ getCpuUsageRate(nodeMetrics[node.name]) }}%</strong>
                    <span>已使用</span>
                  </div>
                </div>
                <div class="metric-summary">
                  <div class="metric-detail-chip">
                    <span>已用</span>
                    <strong>{{ formatCpuValue(nodeMetrics[node.name].usage?.cpu) }}</strong>
                  </div>
                  <div class="metric-detail-chip">
                    <span>总量</span>
                    <strong>{{ formatCpuValue(nodeMetrics[node.name].capacity?.cpu) }}</strong>
                  </div>
                </div>
              </div>
            </div>
            
            <!-- 内存使用率 -->
            <div class="metric-item">
              <div class="metric-header">
                <img src="@/assets/image/内存.svg" alt="Memory" class="metric-icon" />
                <span class="metric-label">内存使用率</span>
                <span class="metric-value" :class="getUsageLevelClass(getMemoryUsageRate(nodeMetrics[node.name]))">
                  {{ getMemoryUsageRate(nodeMetrics[node.name]) }}%
                </span>
              </div>
              <div class="metric-visual">
                <div
                  class="usage-donut memory-donut"
                  :style="getDonutStyle(getMemoryUsageRate(nodeMetrics[node.name]), 'memory')"
                >
                  <div class="usage-donut-center">
                    <strong>{{ getMemoryUsageRate(nodeMetrics[node.name]) }}%</strong>
                    <span>已使用</span>
                  </div>
                </div>
                <div class="metric-summary">
                  <div class="metric-detail-chip">
                    <span>已用</span>
                    <strong>{{ formatMemoryValue(nodeMetrics[node.name].usage?.memory) }}</strong>
                  </div>
                  <div class="metric-detail-chip">
                    <span>总量</span>
                    <strong>{{ formatMemoryValue(nodeMetrics[node.name].capacity?.memory) }}</strong>
                  </div>
                </div>
              </div>
            </div>
            
            <!-- 系统信息 -->
            <div class="system-stats">
              <div class="stat-item">
                <div class="stat-icon pods-icon">
                  <i class="icon-dot"></i>
                </div>
                <div class="stat-content">
                  <span class="stat-label">Pod数量</span>
                  <span class="stat-value">{{ nodeMetrics[node.name].podCount || 0 }}</span>
                </div>
              </div>
              <div class="stat-item">
                <div class="stat-icon time-icon">
                  <i class="icon-pulse"></i>
                </div>
                <div class="stat-content">
                  <span class="stat-label">更新时间</span>
                  <span class="stat-value">{{ formatTimestamp(nodeMetrics[node.name].timestamp) }}</span>
                </div>
              </div>
            </div>
          </div>

          <div v-else class="no-metrics">
            <div class="loading-animation">
              <div class="loading-dot"></div>
              <div class="loading-dot"></div>
              <div class="loading-dot"></div>
            </div>
            <span class="no-data-text">正在获取监控数据...</span>
          </div>

          <!-- 操作按钮 -->
          <div class="node-actions">
            <button 
              class="action-btn primary-btn" 
              @click="viewNodeDetail(node)"
            >
              <View class="btn-icon" />
              <span>详细信息</span>
              <div class="btn-glow"></div>
            </button>
            <button 
              class="action-btn secondary-btn" 
              @click="refreshNodeMetrics(node.name)"
            >
              <Refresh class="btn-icon refresh-icon" />
              <span>刷新</span>
            </button>
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- 节点详情弹框 -->
    <el-dialog
      v-model="nodeDetailVisible"
      :title="`节点监控 - ${selectedNode?.name}`"
      width="70%"
      class="node-detail-dialog"
    >
      <div v-if="selectedNodeMetrics" class="node-detail-content">
        <el-row :gutter="16" style="margin-bottom: 16px;">
          <el-col :span="12">
            <div class="resource-card">
              <div class="card-header-detail">
                <img src="@/assets/image/cpu.svg" alt="Resource" class="header-icon" />
                <h3>资源使用情况</h3>
              </div>
              <div class="resource-metrics">
                <div class="metric-row-detail">
                  <div class="metric-info">
                    <img src="@/assets/image/cpu.svg" alt="CPU" class="metric-icon-detail" />
                    <span class="metric-name">CPU:</span>
                    <span class="metric-percentage" :class="getUsageLevelClass(getCpuUsageRate(selectedNodeMetrics))">
                      {{ getCpuUsageRate(selectedNodeMetrics) }}%
                    </span>
                  </div>
                  <div class="metric-row-visual">
                    <div
                      class="usage-donut usage-donut-detail cpu-donut"
                      :style="getDonutStyle(getCpuUsageRate(selectedNodeMetrics), 'cpu')"
                    >
                      <div class="usage-donut-center">
                        <strong>{{ getCpuUsageRate(selectedNodeMetrics) }}%</strong>
                        <span>CPU</span>
                      </div>
                    </div>
                    <div class="metric-summary metric-summary-detail">
                      <div class="metric-detail-chip">
                        <span>已用</span>
                        <strong>{{ formatCpuValue(selectedNodeMetrics.usage?.cpu) }}</strong>
                      </div>
                      <div class="metric-detail-chip">
                        <span>总量</span>
                        <strong>{{ formatCpuValue(selectedNodeMetrics.capacity?.cpu) }}</strong>
                      </div>
                    </div>
                  </div>
                </div>
                <div class="metric-row-detail">
                  <div class="metric-info">
                    <img src="@/assets/image/内存.svg" alt="Memory" class="metric-icon-detail" />
                    <span class="metric-name">内存:</span>
                    <span class="metric-percentage" :class="getUsageLevelClass(getMemoryUsageRate(selectedNodeMetrics))">
                      {{ getMemoryUsageRate(selectedNodeMetrics) }}%
                    </span>
                  </div>
                  <div class="metric-row-visual">
                    <div
                      class="usage-donut usage-donut-detail memory-donut"
                      :style="getDonutStyle(getMemoryUsageRate(selectedNodeMetrics), 'memory')"
                    >
                      <div class="usage-donut-center">
                        <strong>{{ getMemoryUsageRate(selectedNodeMetrics) }}%</strong>
                        <span>内存</span>
                      </div>
                    </div>
                    <div class="metric-summary metric-summary-detail">
                      <div class="metric-detail-chip">
                        <span>已用</span>
                        <strong>{{ formatMemoryValue(selectedNodeMetrics.usage?.memory) }}</strong>
                      </div>
                      <div class="metric-detail-chip">
                        <span>总量</span>
                        <strong>{{ formatMemoryValue(selectedNodeMetrics.capacity?.memory) }}</strong>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </el-col>
          <el-col :span="12">
            <el-card>
              <template #header>系统信息</template>
              <div v-if="selectedNodeMetrics.systemInfo" class="system-info">
                <el-descriptions :column="1" size="small">
                  <el-descriptions-item label="操作系统">
                    {{ selectedNodeMetrics.systemInfo.osImage }}
                  </el-descriptions-item>
                  <el-descriptions-item label="内核版本">
                    {{ selectedNodeMetrics.systemInfo.kernelVersion }}
                  </el-descriptions-item>
                  <el-descriptions-item label="Kubelet版本">
                    {{ selectedNodeMetrics.systemInfo.kubeletVersion }}
                  </el-descriptions-item>
                  <el-descriptions-item label="容器运行时">
                    {{ selectedNodeMetrics.systemInfo.containerRuntimeVersion }}
                  </el-descriptions-item>
                </el-descriptions>
              </div>
            </el-card>
          </el-col>
        </el-row>

        <!-- Pod列表 -->
        <div class="pod-list-card">
          <div class="card-header-detail">
            <div class="pod-icon">
              <i class="icon-pod"></i>
            </div>
            <h3>运行的Pod ({{ getFormattedPodCount(selectedNodeMetrics) }})</h3>
          </div>
          <div class="pod-table-container">
            <div v-if="getFormattedPodList(selectedNodeMetrics).length > 0" class="pod-table">
              <div class="pod-header">
                <div class="pod-col pod-name">Pod名称</div>
                <div class="pod-col pod-namespace">命名空间</div>
                <div class="pod-col pod-cpu">CPU使用</div>
                <div class="pod-col pod-memory">内存使用</div>
                <div class="pod-col pod-cpu-rate">CPU使用率</div>
                <div class="pod-col pod-memory-rate">内存使用率</div>
              </div>
              <div class="pod-body">
                <div 
                  v-for="(pod, index) in getFormattedPodList(selectedNodeMetrics)" 
                  :key="index"
                  class="pod-row"
                >
                  <div class="pod-col pod-name">
                    <div class="pod-name-content">
                      <div class="pod-status-dot"></div>
                      {{ pod.podName }}
                    </div>
                  </div>
                  <div class="pod-col pod-namespace">
                    <span class="namespace-tag">{{ pod.namespace }}</span>
                  </div>
                  <div class="pod-col pod-cpu">
                    <span class="resource-value">{{ formatCpuValue(pod.usage?.cpu) }}</span>
                  </div>
                  <div class="pod-col pod-memory">
                    <span class="resource-value">{{ formatMemoryValue(pod.usage?.memory) }}</span>
                  </div>
                  <div class="pod-col pod-cpu-rate">
                    <div class="usage-rate-container">
                      <div class="mini-progress">
                        <div 
                          class="mini-progress-fill cpu-progress" 
                          :style="{ width: (pod.usageRate?.cpuRate || 0) + '%' }"
                        ></div>
                      </div>
                      <span class="rate-text">{{ Math.round(pod.usageRate?.cpuRate || 0) }}%</span>
                    </div>
                  </div>
                  <div class="pod-col pod-memory-rate">
                    <div class="usage-rate-container">
                      <div class="mini-progress">
                        <div 
                          class="mini-progress-fill memory-progress" 
                          :style="{ width: (pod.usageRate?.memoryRate || 0) + '%' }"
                        ></div>
                      </div>
                      <span class="rate-text">{{ Math.round(pod.usageRate?.memoryRate || 0) }}%</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
            <div v-else class="no-pods">
              <div class="no-pods-icon">
                <i class="icon-empty"></i>
              </div>
              <p class="no-pods-text">暂无运行中的Pod</p>
            </div>
          </div>
        </div>
      </div>
      
      <template #footer>
        <el-button @click="nodeDetailVisible = false">关闭</el-button>
        <el-button type="primary" @click="refreshNodeDetail">刷新数据</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, defineProps, defineExpose } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Refresh,
  View
} from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'

// Props
const props = defineProps({
  selectedClusterId: {
    type: [String, Number],
    required: true
  }
})

// 节点监控相关
const nodesList = ref([])
const nodeMetrics = ref({})
const nodesLoading = ref(false)
const nodeDetailVisible = ref(false)
const selectedNode = ref(null)
const selectedNodeMetrics = ref(null)

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

// 获取节点状态文本
const getNodeStatusText = (status) => {
  switch (status) {
    case 'Ready':
      return '就绪'
    case 'NotReady':
      return '未就绪'
    case 'SchedulingDisabled':
      return '调度禁用'
    default:
      return '未知'
  }
}

// 格式化CPU值
const formatCpuValue = (cpu) => {
  if (!cpu) return '0m'
  
  // 处理纳秒单位 (n)
  if (cpu.includes('n')) {
    const value = parseInt(cpu.replace('n', ''))
    return Math.round(value / 1000000) + 'm'
  }
  
  // 处理毫核单位 (m)  
  if (cpu.includes('m')) {
    return cpu
  }
  
  // 处理整核单位
  const value = parseFloat(cpu)
  if (!isNaN(value)) {
    return Math.round(value * 1000) + 'm'
  }
  
  return cpu
}

// 格式化内存值
const formatMemoryValue = (memory) => {
  if (!memory) return '0Mi'
  
  // 处理Ki单位
  if (memory.includes('Ki')) {
    const value = parseInt(memory.replace('Ki', ''))
    if (value >= 1024) {
      return Math.round(value / 1024) + 'Mi'
    }
    return memory
  }
  
  // 处理Mi单位
  if (memory.includes('Mi')) {
    return memory
  }
  
  // 处理Gi单位
  if (memory.includes('Gi')) {
    const value = parseFloat(memory.replace('Gi', ''))
    return Math.round(value * 1024) + 'Mi'
  }
  
  return memory
}

// 获取CPU使用率
const getCpuUsageRate = (metrics) => {
  if (!metrics) return 0
  
  // 如果有现成的使用率
  if (metrics.usageRate?.cpuRate > 0) {
    return Math.round(metrics.usageRate.cpuRate)
  }
  
  // 计算使用率
  const usage = metrics.usage?.cpu
  const capacity = metrics.capacity?.cpu
  
  if (!usage || !capacity) return 0
  
  try {
    // 将usage转换为毫核
    let usageMillicores = 0
    if (usage.includes('n')) {
      usageMillicores = parseInt(usage.replace('n', '')) / 1000000
    } else if (usage.includes('m')) {
      usageMillicores = parseInt(usage.replace('m', ''))
    } else {
      usageMillicores = parseFloat(usage) * 1000
    }
    
    // 将capacity转换为毫核
    let capacityMillicores = 0
    if (capacity.includes('m')) {
      capacityMillicores = parseInt(capacity.replace('m', ''))
    } else if (capacity.includes('n')) {
      capacityMillicores = parseInt(capacity.replace('n', '')) / 1000000
    } else {
      capacityMillicores = parseFloat(capacity) * 1000
    }
    
    if (capacityMillicores > 0) {
      const rate = Math.round((usageMillicores / capacityMillicores) * 100)
      return Math.min(Math.max(rate, 0), 100)
    }
  } catch (error) {
    console.error('CPU使用率计算错误:', error)
  }
  
  return 0
}

// 获取内存使用率
const getMemoryUsageRate = (metrics) => {
  if (!metrics) return 0
  
  // 如果有现成的使用率
  if (metrics.usageRate?.memoryRate > 0) {
    return Math.round(metrics.usageRate.memoryRate)
  }
  
  // 计算使用率
  const usage = metrics.usage?.memory
  const capacity = metrics.capacity?.memory
  
  if (!usage || !capacity) return 0
  
  try {
    // 将usage转换为Ki
    let usageKi = 0
    if (usage.includes('Ki')) {
      usageKi = parseInt(usage.replace('Ki', ''))
    } else if (usage.includes('Mi')) {
      usageKi = parseInt(usage.replace('Mi', '')) * 1024
    } else if (usage.includes('Gi')) {
      usageKi = parseFloat(usage.replace('Gi', '')) * 1024 * 1024
    }
    
    // 将capacity转换为Ki
    let capacityKi = 0
    if (capacity.includes('Ki')) {
      capacityKi = parseInt(capacity.replace('Ki', ''))
    } else if (capacity.includes('Mi')) {
      capacityKi = parseInt(capacity.replace('Mi', '')) * 1024
    } else if (capacity.includes('Gi')) {
      capacityKi = parseFloat(capacity.replace('Gi', '')) * 1024 * 1024
    }
    
    if (capacityKi > 0) {
      const rate = Math.round((usageKi / capacityKi) * 100)
      return Math.min(Math.max(rate, 0), 100)
    }
  } catch (error) {
    console.error('内存使用率计算错误:', error)
  }
  
  return 0
}

// 格式化Pod数量
const getFormattedPodCount = (metrics) => {
  if (!metrics) return 0
  
  // 如果有Pod数据，返回实际数量
  if (metrics.podMetrics && metrics.podMetrics.length > 0) {
    return metrics.podMetrics.length
  }
  
  // 如果有Pod计数，返回计数
  if (metrics.podCount > 0) {
    return metrics.podCount
  }
  
  return 0
}

// 获取格式化的Pod列表
const getFormattedPodList = (metrics) => {
  if (!metrics) return []
  
  // 如果有实际的Pod数据，返回格式化后的数据
  if (metrics.podMetrics && metrics.podMetrics.length > 0) {
    return metrics.podMetrics.map(pod => ({
      podName: pod.podName || pod.name || 'unknown-pod',
      namespace: pod.namespace || 'default',
      usage: {
        cpu: formatCpuValue(pod.usage?.cpu || '0m'),
        memory: formatMemoryValue(pod.usage?.memory || '0Mi')
      },
      usageRate: {
        cpuRate: Number(pod.usageRate?.cpuRate) || 0,
        memoryRate: Number(pod.usageRate?.memoryRate) || 0
      }
    }))
  }

  return []
}

const getUsageLevelClass = (percentage) => {
  if (percentage >= 90) return 'is-danger'
  if (percentage >= 75) return 'is-warning'
  if (percentage >= 60) return 'is-notice'
  return 'is-healthy'
}

const getDonutPalette = (type, percentage) => {
  if (type === 'memory') {
    if (percentage >= 90) {
      return {
        primary: '#f0717b',
        secondary: '#f59a73',
        track: 'rgba(240, 120, 130, 0.14)',
        glow: 'rgba(240, 120, 130, 0.26)'
      }
    }
    if (percentage >= 75) {
      return {
        primary: '#f2a94f',
        secondary: '#f6c77a',
        track: 'rgba(242, 169, 79, 0.14)',
        glow: 'rgba(242, 169, 79, 0.24)'
      }
    }
    return {
      primary: '#37b87a',
      secondary: '#7dd8a5',
      track: 'rgba(63, 185, 127, 0.14)',
      glow: 'rgba(63, 185, 127, 0.24)'
    }
  }

  if (percentage >= 90) {
    return {
      primary: '#f06d78',
      secondary: '#ff9c8a',
      track: 'rgba(240, 109, 120, 0.14)',
      glow: 'rgba(240, 109, 120, 0.24)'
    }
  }
  if (percentage >= 75) {
    return {
      primary: '#f2a94f',
      secondary: '#f7d27f',
      track: 'rgba(242, 169, 79, 0.14)',
      glow: 'rgba(242, 169, 79, 0.24)'
    }
  }
  return {
    primary: '#4c8bf5',
    secondary: '#79b1ff',
    track: 'rgba(76, 139, 245, 0.14)',
    glow: 'rgba(76, 139, 245, 0.24)'
  }
}

const getDonutStyle = (percentage, type) => {
  const normalized = Math.max(0, Math.min(100, Number(percentage) || 0))
  const palette = getDonutPalette(type, normalized)

  return {
    '--usage': `${normalized}%`,
    '--donut-primary': palette.primary,
    '--donut-secondary': palette.secondary,
    '--donut-track': palette.track,
    '--donut-glow': palette.glow
  }
}

const getNodeStatusTag = (status) => {
  const tagMap = {
    'Ready': 'success',
    'NotReady': 'danger',
    'Unknown': 'warning',
    'SchedulingDisabled': 'info'
  }
  return tagMap[status] || 'info'
}

// API调用函数
const fetchNodesList = async () => {
  if (!props.selectedClusterId) return
  
  try {
    nodesLoading.value = true
    const response = await k8sApi.getClusterNodes(props.selectedClusterId)
    const responseData = response.data || response
    
    if (responseData.code === 200 || responseData.success) {
      const nodes = responseData.data?.list || responseData.data || []
      nodesList.value = nodes.map(node => ({
        name: node.nodeName || node.name,
        status: node.status || 'Unknown',
        role: node.role || 'worker'
      }))
      
      // 为每个节点获取监控数据
      await Promise.all(nodesList.value.map(node => fetchNodeMetrics(node.name)))
    }
  } catch (error) {
    console.error('获取节点列表失败:', error)
    ElMessage.error('获取节点列表失败')
  } finally {
    nodesLoading.value = false
  }
}

const fetchNodeMetrics = async (nodeName) => {
  if (!props.selectedClusterId || !nodeName) return
  
  try {
    const response = await k8sApi.getNodeMetrics(props.selectedClusterId, nodeName)
    const responseData = response.data || response
    
    if (responseData.code === 200 || responseData.success) {
      const metrics = responseData.data || {}
      nodeMetrics.value[nodeName] = {
        usage: {
          cpu: metrics.usage?.cpu || '0m',
          memory: metrics.usage?.memory || '0Mi'
        },
        capacity: {
          cpu: metrics.capacity?.cpu || '0',
          memory: metrics.capacity?.memory || '0Gi'
        },
        usageRate: {
          cpuRate: parseFloat(metrics.cpuUsageRate) || 0,
          memoryRate: parseFloat(metrics.memoryUsageRate) || 0
        },
        podCount: metrics.podCount || 0,
        timestamp: metrics.timestamp || Date.now(),
        systemInfo: metrics.systemInfo,
        podMetrics: metrics.podMetrics || []
      }
    }
  } catch (error) {
    console.error(`获取节点${nodeName}监控数据失败:`, error)
    // 设置默认值
    nodeMetrics.value[nodeName] = {
      usage: { cpu: '0m', memory: '0Mi' },
      capacity: { cpu: '0', memory: '0Gi' },
      usageRate: { cpuRate: 0, memoryRate: 0 },
      podCount: 0,
      timestamp: Date.now()
    }
  }
}

const refreshNodeMetrics = async (nodeName) => {
  await fetchNodeMetrics(nodeName)
  ElMessage.success(`节点 ${nodeName} 监控数据已刷新`)
}

const viewNodeDetail = async (node) => {
  selectedNode.value = node
  selectedNodeMetrics.value = nodeMetrics.value[node.name]
  
  // 如果没有详细监控数据，重新获取
  if (!selectedNodeMetrics.value?.systemInfo) {
    await fetchNodeMetrics(node.name)
    selectedNodeMetrics.value = nodeMetrics.value[node.name]
  }
  
  nodeDetailVisible.value = true
}

const refreshNodeDetail = async () => {
  if (selectedNode.value) {
    await fetchNodeMetrics(selectedNode.value.name)
    selectedNodeMetrics.value = nodeMetrics.value[selectedNode.value.name]
    ElMessage.success('监控数据已刷新')
  }
}

// 暴露给父组件的方法
const refreshAllNodes = async () => {
  await fetchNodesList()
}

// 监听集群变化
const handleClusterChange = async () => {
  await fetchNodesList()
}

// 暴露方法给父组件
defineExpose({
  refreshAllNodes,
  handleClusterChange
})
</script>

<style scoped>
.nodes-monitoring {
  --aiops-text-primary: #172133;
  --aiops-text-secondary: #5f6b7c;
  --aiops-text-muted: #8d97a8;
  --aiops-border-cool: #dfe7f2;
  --aiops-brand: #4c8bf5;
  --aiops-success: #3fb97f;
  --aiops-warning: #f3b24b;
  --aiops-danger: #ef6b73;
  padding: 0;
  background: transparent;
  min-height: 0;
  border-radius: 0;
}

/* 节点卡片 */
.node-card {
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.94), rgba(247, 250, 255, 0.9));
  backdrop-filter: blur(14px);
  border-radius: 18px;
  padding: 18px;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
  border: 1px solid rgba(223, 231, 242, 0.9);
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
  position: relative;
  overflow: hidden;
}

.node-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 36px;
  background: linear-gradient(90deg, rgba(255, 249, 240, 0.95), rgba(248, 248, 246, 0.86), rgba(237, 242, 253, 0.92));
  border-radius: 18px 18px 0 0;
  pointer-events: none;
}

.node-card:hover {
  border-color: rgba(171, 189, 220, 0.95);
  box-shadow: 0 14px 28px rgba(17, 24, 39, 0.06);
}

/* 卡片头部 */
.card-header {
  position: relative;
  z-index: 1;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

.node-info-header {
  display: flex;
  align-items: center;
  gap: 16px;
}

.node-icon {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  background: rgba(76, 139, 245, 0.12);
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 14px;
  box-shadow: none;
}

.k8s-icon {
  width: 24px;
  height: 24px;
  filter: none;
  z-index: 2;
}

.pulse-ring {
  display: none;
}

@keyframes pulse {
  0% { opacity: 1; transform: translate(-50%, -50%) scale(0.8); }
  50% { opacity: 0.5; transform: translate(-50%, -50%) scale(1.1); }
  100% { opacity: 0; transform: translate(-50%, -50%) scale(1.4); }
}

.node-details h4 {
  margin: 0 0 8px 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--aiops-text-primary);
  text-shadow: none;
}

.status-tag {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
  background: rgba(46, 204, 113, 0.1);
  color: #2ecc71;
  border: 1px solid rgba(46, 204, 113, 0.2);
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  animation: statusPulse 2s infinite;
}

@keyframes statusPulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.node-ip {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 13px;
  color: var(--aiops-text-secondary);
  padding: 4px 8px;
  background: rgba(255, 255, 255, 0.72);
  border-radius: 999px;
  border: 1px solid rgba(223, 231, 242, 0.9);
}

/* 指标区域 */
.node-metrics {
  margin-bottom: 20px;
}

.metric-item {
  margin-bottom: 20px;
  background: rgba(255, 255, 255, 0.68);
  border-radius: 14px;
  padding: 12px;
  border: 1px solid rgba(223, 231, 242, 0.78);
  transition: border-color 0.2s ease;
}

.metric-item:hover {
  border-color: rgba(171, 189, 220, 0.95);
}

.metric-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 10px;
}

.metric-icon {
  width: 20px;
  height: 20px;
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.1));
}

@keyframes iconFloat {
  0%, 100% { transform: translateY(0px); }
  50% { transform: translateY(-2px); }
}

.metric-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--aiops-text-secondary);
  flex: 1;
}

.metric-value {
  font-size: 16px;
  font-weight: 700;
  color: var(--aiops-brand);
}

.metric-value.is-healthy {
  color: var(--aiops-success);
}

.metric-value.is-notice {
  color: var(--aiops-brand);
}

.metric-value.is-warning {
  color: var(--aiops-warning);
}

.metric-value.is-danger {
  color: var(--aiops-danger);
}

.metric-visual {
  display: flex;
  align-items: center;
  gap: 12px;
}

.usage-donut {
  --usage: 0%;
  --donut-primary: #4c8bf5;
  --donut-secondary: #79b1ff;
  --donut-track: rgba(76, 139, 245, 0.14);
  --donut-glow: rgba(76, 139, 245, 0.24);
  position: relative;
  flex: 0 0 90px;
  width: 90px;
  height: 90px;
  border-radius: 50%;
  background:
    conic-gradient(from -90deg, var(--donut-primary) 0, var(--donut-secondary) var(--usage), var(--donut-track) var(--usage), rgba(235, 240, 247, 0.92) 100%);
  box-shadow:
    inset 0 0 0 1px rgba(255, 255, 255, 0.7),
    0 16px 28px var(--donut-glow);
}

.usage-donut::before {
  content: '';
  position: absolute;
  inset: 9px;
  border-radius: 50%;
  background:
    radial-gradient(circle at 30% 30%, rgba(255, 255, 255, 0.96), rgba(246, 249, 255, 0.98) 68%, rgba(238, 244, 252, 0.98) 100%);
  box-shadow: inset 0 0 0 1px rgba(223, 231, 242, 0.88);
}

.usage-donut::after {
  content: '';
  position: absolute;
  inset: -7px;
  border-radius: 50%;
  background: radial-gradient(circle, var(--donut-glow) 0%, rgba(255, 255, 255, 0) 70%);
  opacity: 0.7;
  z-index: 0;
}

.usage-donut-center {
  position: absolute;
  inset: 0;
  z-index: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  text-align: center;
}

.usage-donut-center strong {
  font-size: 17px;
  line-height: 1;
  color: var(--aiops-text-primary);
}

.usage-donut-center span {
  font-size: 9px;
  color: var(--aiops-text-muted);
  letter-spacing: 0.04em;
}

.metric-summary {
  flex: 1;
  display: grid;
  gap: 7px;
}

.metric-detail-chip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 10px;
  border-radius: 12px;
  background: linear-gradient(180deg, rgba(249, 251, 255, 0.96), rgba(243, 247, 253, 0.92));
  border: 1px solid rgba(223, 231, 242, 0.92);
}

.metric-detail-chip span {
  font-size: 10px;
  color: var(--aiops-text-muted);
}

.metric-detail-chip strong {
  font-size: 11px;
  font-weight: 700;
  color: var(--aiops-text-primary);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
}

.cpu-progress {
  background: linear-gradient(90deg, #4c8bf5 0%, #79b1ff 100%);
}

.memory-progress {
  background: linear-gradient(90deg, #37b87a 0%, #7dd8a5 100%);
}

/* 系统统计 */
.system-stats {
  display: flex;
  gap: 16px;
  margin-bottom: 20px;
}

.stat-item {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 12px;
  background: rgba(255, 255, 255, 0.68);
  padding: 12px;
  border-radius: 14px;
  border: 1px solid rgba(223, 231, 242, 0.78);
  transition: border-color 0.2s ease;
}

.stat-item:hover {
  border-color: rgba(171, 189, 220, 0.95);
}

.stat-icon {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
}

.pods-icon {
  background: rgba(76, 139, 245, 0.12);
}

.time-icon {
  background: rgba(243, 178, 75, 0.14);
}

.icon-dot {
  width: 8px;
  height: 8px;
  background: white;
  border-radius: 50%;
  animation: iconDot 1.5s infinite;
}

.icon-pulse {
  width: 10px;
  height: 10px;
  background: white;
  border-radius: 50%;
  animation: iconPulse 2s infinite;
}

@keyframes iconDot {
  0%, 100% { transform: scale(1); }
  50% { transform: scale(1.2); }
}

@keyframes iconPulse {
  0% { transform: scale(1); opacity: 1; }
  50% { transform: scale(1.3); opacity: 0.7; }
  100% { transform: scale(1); opacity: 1; }
}

.stat-content {
  flex: 1;
}

.stat-label {
  display: block;
  font-size: 11px;
  color: var(--aiops-text-muted);
  margin-bottom: 2px;
}

.stat-value {
  display: block;
  font-size: 14px;
  font-weight: 600;
  color: var(--aiops-text-primary);
}

/* 无数据状态 */
.no-metrics {
  text-align: center;
  padding: 40px 20px;
  background: rgba(255, 255, 255, 0.68);
  border: 1px solid rgba(223, 231, 242, 0.78);
  border-radius: 14px;
  margin-bottom: 20px;
}

.loading-animation {
  display: flex;
  justify-content: center;
  gap: 4px;
  margin-bottom: 16px;
}

.loading-dot {
  width: 8px;
  height: 8px;
  background: var(--aiops-brand);
  border-radius: 50%;
  animation: loadingDot 1.4s infinite ease-in-out both;
}

.loading-dot:nth-child(1) { animation-delay: -0.32s; }
.loading-dot:nth-child(2) { animation-delay: -0.16s; }

@keyframes loadingDot {
  0%, 80%, 100% { transform: scale(0); }
  40% { transform: scale(1); }
}

.no-data-text {
  color: var(--aiops-text-secondary);
  font-size: 14px;
}

/* 操作按钮 */
.node-actions {
  display: flex;
  gap: 12px;
}

.action-btn {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 12px 16px;
  border: none;
  border-radius: 13px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
  position: relative;
  overflow: hidden;
}

.primary-btn {
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
  color: white;
  box-shadow: 0 10px 18px rgba(76, 139, 245, 0.16);
}

.primary-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 12px 22px rgba(76, 139, 245, 0.2);
}

.secondary-btn {
  background: rgba(255, 255, 255, 0.92);
  color: var(--aiops-text-secondary);
  border: 1px solid rgba(214, 223, 235, 0.96);
}

.secondary-btn:hover {
  background: #ffffff;
  color: var(--aiops-brand);
  border-color: rgba(153, 181, 230, 0.95);
  transform: translateY(-1px);
}

.btn-icon {
  width: 16px;
  height: 16px;
}

.refresh-icon {
  animation: none;
  transition: transform 0.3s ease;
}

.secondary-btn:hover .refresh-icon {
  animation: refreshSpin 0.8s ease-in-out;
}

@keyframes refreshSpin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.btn-glow {
  display: none;
}

/* 对话框样式 */
.node-detail-dialog :deep(.el-dialog) {
  border-radius: 24px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
  backdrop-filter: blur(18px);
  border: 1px solid rgba(223, 231, 242, 0.96);
  max-height: 90vh;
  overflow: hidden;
}

.node-detail-dialog :deep(.el-dialog__header) {
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.84);
  padding: 16px 20px;
}

.node-detail-dialog :deep(.el-dialog__title) {
  color: var(--aiops-text-primary);
  font-weight: 700;
  font-size: 18px;
}

.node-detail-dialog :deep(.el-dialog__body) {
  padding: 24px;
  max-height: 70vh;
  overflow-y: auto;
}

.node-detail-content {
  padding: 0;
}

/* 资源卡片 */
.resource-card {
  background: rgba(255, 255, 255, 0.84);
  border-radius: 18px;
  padding: 20px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  box-shadow: none;
}

.card-header-detail {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(0, 0, 0, 0.1);
}

.header-icon, .metric-icon-detail {
  width: 24px;
  height: 24px;
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.1));
}

.card-header-detail h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: #333;
}

.resource-metrics {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.metric-row-detail {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 16px;
  background: rgba(255, 255, 255, 0.72);
  border-radius: 14px;
  border: 1px solid rgba(223, 231, 242, 0.78);
  transition: border-color 0.2s ease;
}

.metric-row-detail:hover {
  border-color: rgba(171, 189, 220, 0.95);
}

.metric-info {
  display: flex;
  align-items: center;
  gap: 12px;
}

.metric-name {
  font-weight: 500;
  color: var(--aiops-text-secondary);
  flex: 1;
}

.metric-percentage {
  font-size: 18px;
  font-weight: 700;
  color: var(--aiops-brand);
}

.metric-percentage.is-healthy {
  color: var(--aiops-success);
}

.metric-percentage.is-notice {
  color: var(--aiops-brand);
}

.metric-percentage.is-warning {
  color: var(--aiops-warning);
}

.metric-percentage.is-danger {
  color: var(--aiops-danger);
}

.metric-row-visual {
  display: flex;
  align-items: center;
  gap: 14px;
}

.usage-donut-detail {
  flex-basis: 100px;
  width: 100px;
  height: 100px;
}

.metric-summary-detail {
  width: 100%;
}

.system-info {
  padding: 0;
}

/* Pod列表卡片 */
.pod-list-card {
  background: rgba(255, 255, 255, 0.84);
  border-radius: 18px;
  padding: 20px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  box-shadow: none;
  margin-top: 20px;
}

.pod-icon {
  width: 32px;
  height: 32px;
  background: rgba(76, 139, 245, 0.12);
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.icon-pod {
  width: 12px;
  height: 12px;
  background: white;
  border-radius: 2px;
  position: relative;
}

.icon-pod::before {
  content: '';
  position: absolute;
  width: 8px;
  height: 8px;
  background: rgba(255, 255, 255, 0.7);
  border-radius: 1px;
  top: 2px;
  left: 2px;
}

.pod-table-container {
  margin-top: 16px;
}

.pod-table {
  background: rgba(255, 255, 255, 0.86);
  border-radius: 16px;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.9);
}

.pod-header {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr 1fr 1fr 1fr;
  gap: 12px;
  padding: 12px 16px;
  background: rgba(248, 250, 255, 0.98);
  font-weight: 600;
  font-size: 13px;
  color: var(--aiops-text-muted);
  border-bottom: 1px solid rgba(223, 231, 242, 0.9);
}

.pod-body {
  max-height: 300px;
  overflow-y: auto;
}

.pod-row {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr 1fr 1fr 1fr;
  gap: 12px;
  padding: 12px 16px;
  border-bottom: 1px solid rgba(15, 23, 42, 0.045);
  transition: background-color 0.2s ease;
}

.pod-row:hover {
  background: rgba(76, 139, 245, 0.045);
}

.pod-row:last-child {
  border-bottom: none;
}

.pod-col {
  display: flex;
  align-items: center;
  font-size: 12px;
}

.pod-name-content {
  display: flex;
  align-items: center;
  gap: 8px;
}

.pod-status-dot {
  width: 6px;
  height: 6px;
  background: #2ecc71;
  border-radius: 50%;
  animation: statusPulse 2s infinite;
}

.namespace-tag {
  padding: 2px 8px;
  background: rgba(76, 139, 245, 0.08);
  color: var(--aiops-brand);
  border-radius: 12px;
  font-size: 11px;
  border: 1px solid rgba(153, 181, 230, 0.32);
}

.resource-value {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  color: var(--aiops-text-primary);
  font-weight: 500;
}

.usage-rate-container {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}

.mini-progress {
  flex: 1;
  height: 4px;
  background: rgba(0, 0, 0, 0.1);
  border-radius: 2px;
  overflow: hidden;
}

.mini-progress-fill {
  height: 100%;
  border-radius: 2px;
  transition: width 0.8s ease;
}

.rate-text {
  font-size: 11px;
  font-weight: 600;
  color: #333;
  min-width: 35px;
  text-align: right;
}

.no-pods {
  text-align: center;
  padding: 40px 20px;
  color: #666;
}

.no-pods-icon {
  width: 48px;
  height: 48px;
  margin: 0 auto 16px;
  background: rgba(0, 0, 0, 0.1);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.icon-empty {
  width: 20px;
  height: 20px;
  background: rgba(0, 0, 0, 0.3);
  border-radius: 2px;
}

.no-pods-text {
  margin: 0;
  font-size: 14px;
  color: #888;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .nodes-monitoring {
    padding: 12px;
  }
  
  .node-card {
    padding: 16px;
  }
  
  .node-actions {
    flex-direction: column;
    gap: 8px;
  }
  
  .system-stats {
    flex-direction: column;
    gap: 8px;
  }
  
  .metric-visual,
  .metric-row-visual {
    flex-direction: column;
    align-items: stretch;
  }

  .usage-donut,
  .usage-donut-detail {
    margin: 0 auto;
  }
}
</style>
