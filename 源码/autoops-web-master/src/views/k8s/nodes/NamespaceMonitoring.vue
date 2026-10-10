<template>
  <div class="namespaces-monitoring">
    <el-row :gutter="20" style="margin-bottom: 20px;">
      <el-col :span="8" v-for="namespace in namespaceData" :key="namespace.name">
        <div class="namespace-card" v-loading="namespacesLoading">
          <!-- 卡片头部 -->
          <div class="card-header">
            <div class="namespace-info-header">
              <div class="namespace-icon">
                <img src="@/assets/image/k8s.svg" alt="k8s" class="k8s-icon" />
                <div class="pulse-ring"></div>
              </div>
              <div class="namespace-details">
                <h4>{{ namespace.name }}</h4>
                <el-tag 
                  :type="getNamespaceStatusTag(namespace.status)" 
                  size="small"
                  :class="['status-tag', namespace.status.toLowerCase()]"
                >
                  <i class="status-dot"></i>
                  {{ getNamespaceStatusText(namespace.status) }}
                </el-tag>
              </div>
            </div>
            <div class="namespace-name">{{ namespace.name }}</div>
          </div>
          
          <div v-if="namespaceMetrics[namespace.name]" class="namespace-metrics">
            <!-- CPU 使用率 -->
            <div class="metric-item">
              <div class="metric-header">
                <img src="@/assets/image/cpu.svg" alt="CPU" class="metric-icon" />
                <span class="metric-label">CPU使用率</span>
                <span class="metric-value" :class="getUsageLevelClass(getNamespaceCpuUsageRate(namespaceMetrics[namespace.name]))">
                  {{ getNamespaceCpuUsageRate(namespaceMetrics[namespace.name]) }}%
                </span>
              </div>
              <div class="metric-visual">
                <div
                  class="usage-donut cpu-donut"
                  :style="getDonutStyle(getNamespaceCpuUsageRate(namespaceMetrics[namespace.name]), 'cpu')"
                >
                  <div class="usage-donut-center">
                    <strong>{{ getNamespaceCpuUsageRate(namespaceMetrics[namespace.name]) }}%</strong>
                    <span>已使用</span>
                  </div>
                </div>
                <div class="metric-summary">
                  <div class="metric-detail-chip">
                    <span>已用</span>
                    <strong>{{ formatCpuValue(namespaceMetrics[namespace.name].totalUsage?.cpu) }}</strong>
                  </div>
                  <div class="metric-detail-chip">
                    <span>总量</span>
                    <strong>{{ formatCpuValue(namespaceMetrics[namespace.name].totalCapacity?.cpu) }}</strong>
                  </div>
                </div>
              </div>
            </div>
            
            <!-- 内存使用率 -->
            <div class="metric-item">
              <div class="metric-header">
                <img src="@/assets/image/内存.svg" alt="Memory" class="metric-icon" />
                <span class="metric-label">内存使用率</span>
                <span class="metric-value" :class="getUsageLevelClass(getNamespaceMemoryUsageRate(namespaceMetrics[namespace.name]))">
                  {{ getNamespaceMemoryUsageRate(namespaceMetrics[namespace.name]) }}%
                </span>
              </div>
              <div class="metric-visual">
                <div
                  class="usage-donut memory-donut"
                  :style="getDonutStyle(getNamespaceMemoryUsageRate(namespaceMetrics[namespace.name]), 'memory')"
                >
                  <div class="usage-donut-center">
                    <strong>{{ getNamespaceMemoryUsageRate(namespaceMetrics[namespace.name]) }}%</strong>
                    <span>已使用</span>
                  </div>
                </div>
                <div class="metric-summary">
                  <div class="metric-detail-chip">
                    <span>已用</span>
                    <strong>{{ formatMemoryValue(namespaceMetrics[namespace.name].totalUsage?.memory) }}</strong>
                  </div>
                  <div class="metric-detail-chip">
                    <span>总量</span>
                    <strong>{{ formatMemoryValue(namespaceMetrics[namespace.name].totalCapacity?.memory) }}</strong>
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
                  <span class="stat-value">{{ namespaceMetrics[namespace.name].podCount || 0 }}</span>
                </div>
              </div>
              <div class="stat-item">
                <div class="stat-icon services-icon">
                  <i class="icon-pulse"></i>
                </div>
                <div class="stat-content">
                  <span class="stat-label">更新时间</span>
                  <span class="stat-value">{{ formatTimestamp(namespaceMetrics[namespace.name].timestamp) }}</span>
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
            <p class="no-data-text">正在获取监控数据...</p>
          </div>

          <!-- 操作按钮 -->
          <div class="namespace-actions">
            <button 
              class="action-btn primary-btn"
              @click="viewNamespaceDetail(namespace)"
            >
              <svg class="btn-icon" viewBox="0 0 24 24" width="16" height="16">
                <path d="M12,9A3,3 0 0,0 9,12A3,3 0 0,0 12,15A3,3 0 0,0 15,12A3,3 0 0,0 12,9M12,17A5,5 0 0,1 7,12A5,5 0 0,1 12,7A5,5 0 0,1 17,12A5,5 0 0,1 12,17M12,4.5C7,4.5 2.73,7.61 1,12C2.73,16.39 7,19.5 12,19.5C17,19.5 21.27,16.39 23,12C21.27,7.61 17,4.5 12,4.5Z" fill="currentColor"/>
              </svg>
              <span>详细信息</span>
              <div class="btn-glow"></div>
            </button>
            <button 
              class="action-btn secondary-btn"
              @click="refreshNamespaceMetrics(namespace.name)"
            >
              <svg class="btn-icon refresh-icon" viewBox="0 0 24 24" width="16" height="16">
                <path d="M17.65,6.35C16.2,4.9 14.21,4 12,4A8,8 0 0,0 4,12A8,8 0 0,0 12,20C15.73,20 18.84,17.45 19.73,14H17.65C16.83,16.33 14.61,18 12,18A6,6 0 0,1 6,12A6,6 0 0,1 12,6C13.66,6 15.14,6.69 16.22,7.78L13,11H20V4L17.65,6.35Z" fill="currentColor"/>
              </svg>
              <span>刷新</span>
            </button>
          </div>
        </div>
      </el-col>
    </el-row>

      <!-- 命名空间详情对话框 -->
      <el-dialog
        v-model="namespaceDetailDialogVisible"
        :title="`命名空间监控 - ${selectedNamespace?.name || ''}`"
        width="70%"
        class="namespace-detail-dialog"
        :before-close="closeNamespaceDetailDialog"
      >
        <div class="namespace-detail-content" v-if="selectedNamespace">
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
                      <span class="metric-percentage" :class="getUsageLevelClass(getNamespaceCpuUsageRate(selectedNamespace.metrics))">
                        {{ getNamespaceCpuUsageRate(selectedNamespace.metrics) }}%
                      </span>
                    </div>
                    <div class="metric-row-visual">
                      <div
                        class="usage-donut usage-donut-detail cpu-donut"
                        :style="getDonutStyle(getNamespaceCpuUsageRate(selectedNamespace.metrics), 'cpu')"
                      >
                        <div class="usage-donut-center">
                          <strong>{{ getNamespaceCpuUsageRate(selectedNamespace.metrics) }}%</strong>
                          <span>CPU</span>
                        </div>
                      </div>
                      <div class="metric-summary metric-summary-detail">
                        <div class="metric-detail-chip">
                          <span>已用</span>
                          <strong>{{ formatCpuValue(selectedNamespace.metrics?.totalUsage?.cpu || selectedNamespace.metrics?.cpu?.usage) }}</strong>
                        </div>
                        <div class="metric-detail-chip">
                          <span>总量</span>
                          <strong>{{ formatCpuValue(selectedNamespace.metrics?.totalCapacity?.cpu || selectedNamespace.metrics?.cpu?.capacity) }}</strong>
                        </div>
                      </div>
                    </div>
                  </div>

                  <div class="metric-row-detail">
                    <div class="metric-info">
                      <img src="@/assets/image/内存.svg" alt="Memory" class="metric-icon-detail" />
                      <span class="metric-name">内存:</span>
                      <span class="metric-percentage" :class="getUsageLevelClass(getNamespaceMemoryUsageRate(selectedNamespace.metrics))">
                        {{ getNamespaceMemoryUsageRate(selectedNamespace.metrics) }}%
                      </span>
                    </div>
                    <div class="metric-row-visual">
                      <div
                        class="usage-donut usage-donut-detail memory-donut"
                        :style="getDonutStyle(getNamespaceMemoryUsageRate(selectedNamespace.metrics), 'memory')"
                      >
                        <div class="usage-donut-center">
                          <strong>{{ getNamespaceMemoryUsageRate(selectedNamespace.metrics) }}%</strong>
                          <span>内存</span>
                        </div>
                      </div>
                      <div class="metric-summary metric-summary-detail">
                        <div class="metric-detail-chip">
                          <span>已用</span>
                          <strong>{{ formatMemoryValue(selectedNamespace.metrics?.totalUsage?.memory || selectedNamespace.metrics?.memory?.usage) }}</strong>
                        </div>
                        <div class="metric-detail-chip">
                          <span>总量</span>
                          <strong>{{ formatMemoryValue(selectedNamespace.metrics?.totalCapacity?.memory || selectedNamespace.metrics?.memory?.capacity) }}</strong>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </el-col>

            <el-col :span="12">
              <el-card class="detail-info-card">
                <template #header>命名空间概览</template>
                <div class="system-info">
                  <el-descriptions :column="1" size="small">
                    <el-descriptions-item label="命名空间">
                      {{ selectedNamespace.name }}
                    </el-descriptions-item>
                    <el-descriptions-item label="状态">
                      <el-tag :type="getNamespaceStatusTag(selectedNamespace.status)" size="small">
                        {{ getNamespaceStatusText(selectedNamespace.status) }}
                      </el-tag>
                    </el-descriptions-item>
                    <el-descriptions-item label="创建时间">
                      {{ formatTimestamp(selectedNamespace.createdAt) }}
                    </el-descriptions-item>
                    <el-descriptions-item label="标签数量">
                      {{ Object.keys(selectedNamespace.labels || {}).length }}
                    </el-descriptions-item>
                    <el-descriptions-item label="Pod数量">
                      {{ selectedNamespace.podCount || 0 }}
                    </el-descriptions-item>
                    <el-descriptions-item label="服务数量">
                      {{ selectedNamespace.serviceCount || 0 }}
                    </el-descriptions-item>
                  </el-descriptions>
                </div>
              </el-card>
            </el-col>
          </el-row>

          <div class="pod-list-card">
            <div class="card-header-detail">
              <div class="pod-icon">
                <i class="icon-pod"></i>
              </div>
              <h3>运行的Pod ({{ namespacePods.length }})</h3>
            </div>
            <div class="pod-table-container">
              <div v-if="namespacePods.length > 0" class="pod-table">
                <div class="pod-header namespace-pod-header">
                  <div class="pod-col pod-name">Pod名称</div>
                  <div class="pod-col pod-status-col">状态</div>
                  <div class="pod-col pod-cpu">CPU使用</div>
                  <div class="pod-col pod-memory">内存使用</div>
                </div>
                <div class="pod-body">
                  <div v-for="pod in namespacePods" :key="pod.name" class="pod-row namespace-pod-row">
                    <div class="pod-col pod-name">
                      <div class="pod-name-content">
                        <div class="pod-status-dot"></div>
                        {{ pod.name }}
                      </div>
                    </div>
                    <div class="pod-col pod-status-col">
                      <el-tag :type="getPodStatusTag(pod.status)" size="small" class="pod-status">
                        {{ pod.status }}
                      </el-tag>
                    </div>
                    <div class="pod-col pod-cpu">
                      <span class="resource-value">{{ formatCpuValue(pod.cpu) }}</span>
                    </div>
                    <div class="pod-col pod-memory">
                      <span class="resource-value">{{ formatMemoryValue(pod.memory) }}</span>
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
      </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted, defineProps, defineExpose } from 'vue'
import { ElMessage } from 'element-plus'
import k8sApi from '@/api/k8s'

const props = defineProps({
  selectedClusterId: {
    type: [String, Number],
    required: true
  }
})

const namespaceDetailDialogVisible = ref(false)
const selectedNamespace = ref(null)
const namespacePods = ref([])
const namespacesLoading = ref(false)
const namespaceData = ref([])
const namespaceMetrics = ref({})

// 格式化CPU值
const formatCpuValue = (value) => {
  if (!value) return '0m'
  
  if (typeof value === 'string') {
    if (value.includes('n')) {
      const nanoValue = parseInt(value.replace('n', ''))
      const milliValue = Math.round(nanoValue / 1000000)
      return `${milliValue}m`
    }
    if (value.includes('m')) {
      return value
    }
    return `${parseInt(value) * 1000}m`
  }
  
  return `${Math.round(value / 1000000)}m`
}

// 格式化内存值
const formatMemoryValue = (value) => {
  if (!value) return '0Mi'
  
  if (typeof value === 'string') {
    if (value.includes('Ki')) {
      const kiValue = parseInt(value.replace('Ki', ''))
      const miValue = Math.round(kiValue / 1024)
      return `${miValue}Mi`
    }
    if (value.includes('Mi')) {
      return value
    }
    if (value.includes('Gi')) {
      const giValue = parseInt(value.replace('Gi', ''))
      return `${giValue * 1024}Mi`
    }
  }
  
  const bytes = parseInt(value)
  const miValue = Math.round(bytes / (1024 * 1024))
  return `${miValue}Mi`
}

// 计算命名空间CPU使用率 (适配新的数据结构)
const getNamespaceCpuUsageRate = (metrics) => {
  if (!metrics) {
    return 0
  }

  if (metrics.usageRate && Number.isFinite(Number(metrics.usageRate.cpuRate))) {
    return Math.min(Math.max(Math.round(Number(metrics.usageRate.cpuRate)), 0), 100)
  }

  const usage = normalizeCpuValue(metrics.totalUsage?.cpu || metrics.cpu?.usage)
  const capacity = normalizeCpuValue(metrics.totalCapacity?.cpu || metrics.cpu?.capacity)

  if (capacity > 0) {
    return Math.min(Math.max(Math.round((usage / capacity) * 100), 0), 100)
  }

  return 0
}

// 计算命名空间内存使用率 (适配新的数据结构) 
const getNamespaceMemoryUsageRate = (metrics) => {
  if (!metrics) {
    return 0
  }

  if (metrics.usageRate && Number.isFinite(Number(metrics.usageRate.memoryRate))) {
    return Math.min(Math.max(Math.round(Number(metrics.usageRate.memoryRate)), 0), 100)
  }

  const usage = normalizeMemoryValue(metrics.totalUsage?.memory || metrics.memory?.usage)
  const capacity = normalizeMemoryValue(metrics.totalCapacity?.memory || metrics.memory?.capacity)

  if (capacity > 0) {
    return Math.min(Math.max(Math.round((usage / capacity) * 100), 0), 100)
  }

  return 0
}

const normalizeCpuValue = (value) => {
  if (!value) return 0
  const raw = String(value)
  if (raw.includes('n')) return parseInt(raw.replace('n', ''), 10) / 1000000
  if (raw.includes('m')) return parseInt(raw.replace('m', ''), 10)
  return parseFloat(raw) * 1000
}

const normalizeMemoryValue = (value) => {
  if (!value) return 0
  const raw = String(value)
  if (raw.includes('Ki')) return parseInt(raw.replace('Ki', ''), 10)
  if (raw.includes('Mi')) return parseInt(raw.replace('Mi', ''), 10) * 1024
  if (raw.includes('Gi')) return parseFloat(raw.replace('Gi', '')) * 1024 * 1024
  return parseInt(raw, 10) / 1024
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

const createEmptyNamespaceMetrics = () => ({
  namespace: '',
  timestamp: '',
  podCount: 0,
  runningPods: 0,
  totalUsage: {
    cpu: '0m',
    memory: '0Mi'
  },
  totalCapacity: {
    cpu: '0m',
    memory: '0Mi'
  },
  resourceQuota: {
    hard: {
      cpu: '',
      memory: ''
    },
    used: {
      cpu: '',
      memory: ''
    }
  },
  usageRate: {
    cpuRate: 0,
    memoryRate: 0
  },
  podMetrics: []
})

const mapNamespaceItem = (ns) => ({
  name: ns.name,
  status: ns.status || 'Active',
  createdAt: ns.createdAt,
  labels: ns.labels || {},
  annotations: ns.annotations || {},
  podCount: ns.resourceCount?.podCount || 0,
  serviceCount: ns.resourceCount?.serviceCount || 0
})

const mapNamespaceMetrics = (metrics, baseNamespace = {}) => ({
  ...createEmptyNamespaceMetrics(),
  ...metrics,
  namespace: metrics?.namespace || baseNamespace.name || '',
  timestamp: metrics?.timestamp || '',
  podCount: metrics?.podCount ?? baseNamespace.podCount ?? 0,
  runningPods: metrics?.runningPods ?? 0,
  totalUsage: {
    cpu: metrics?.totalUsage?.cpu || '0m',
    memory: metrics?.totalUsage?.memory || '0Mi'
  },
  totalCapacity: {
    cpu: metrics?.resourceQuota?.hard?.cpu || '',
    memory: metrics?.resourceQuota?.hard?.memory || ''
  },
  resourceQuota: {
    hard: {
      cpu: metrics?.resourceQuota?.hard?.cpu || '',
      memory: metrics?.resourceQuota?.hard?.memory || ''
    },
    used: {
      cpu: metrics?.resourceQuota?.used?.cpu || '',
      memory: metrics?.resourceQuota?.used?.memory || ''
    }
  },
  usageRate: {
    cpuRate: Number(metrics?.usageRate?.cpuRate) || 0,
    memoryRate: Number(metrics?.usageRate?.memoryRate) || 0
  },
  podMetrics: Array.isArray(metrics?.podMetrics) ? metrics.podMetrics : []
})

const normalizeNamespaceList = (responseData) => {
  const namespaces = responseData.data?.namespaces || responseData.data || []
  return Array.isArray(namespaces) ? namespaces : []
}

const fetchNamespaceList = async () => {
  const response = await k8sApi.getNamespaces(props.selectedClusterId)
  const responseData = response.data || response

  if (!(responseData.code === 200 || responseData.success)) {
    throw new Error(responseData.message || '获取命名空间列表失败')
  }

  namespaceData.value = normalizeNamespaceList(responseData).map(mapNamespaceItem)
}

const fetchNamespaceMetrics = async (namespaceName) => {
  try {
    const response = await k8sApi.getNamespaceMetrics(props.selectedClusterId, namespaceName)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      const baseNamespace = namespaceData.value.find(item => item.name === namespaceName) || {}
      namespaceMetrics.value[namespaceName] = mapNamespaceMetrics(responseData.data || {}, baseNamespace)
      return
    }

    throw new Error(responseData.message || '获取命名空间监控数据失败')
  } catch (error) {
    console.error(`获取命名空间 ${namespaceName} 监控数据失败:`, error)
    const baseNamespace = namespaceData.value.find(item => item.name === namespaceName) || {}
    namespaceMetrics.value[namespaceName] = mapNamespaceMetrics({}, baseNamespace)
    throw error
  }
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

// 获取命名空间状态标签样式
const getNamespaceStatusTag = (status) => {
  switch (status) {
    case 'Active':
      return 'success'
    case 'Terminating':
      return 'warning'
    case 'Inactive':
      return 'danger'
    default:
      return 'info'
  }
}

// 获取命名空间状态文本
const getNamespaceStatusText = (status) => {
  switch (status) {
    case 'Active':
      return '活跃'
    case 'Terminating':
      return '终止中'
    case 'Inactive':
      return '非活跃'
    default:
      return '未知'
  }
}

// 获取Pod状态标签样式
const getPodStatusTag = (status) => {
  switch (status) {
    case 'Running':
      return 'success'
    case 'Pending':
      return 'warning'
    case 'Failed':
      return 'danger'
    case 'Succeeded':
      return 'info'
    default:
      return 'info'
  }
}

const buildPodUsageMap = (metrics) => {
  const podMetrics = Array.isArray(metrics?.podMetrics) ? metrics.podMetrics : []
  return podMetrics.reduce((acc, item) => {
    acc[item.podName] = item
    return acc
  }, {})
}

const fetchNamespacePodList = async (namespaceName) => {
  const response = await k8sApi.getPodList(props.selectedClusterId, namespaceName)
  const responseData = response.data || response

  if (!(responseData.code === 200 || responseData.success)) {
    throw new Error(responseData.message || '获取Pod列表失败')
  }

  const payload = responseData.data || {}
  const pods = Array.isArray(payload.pods) ? payload.pods : []
  const usageMap = buildPodUsageMap(namespaceMetrics.value[namespaceName])

  return pods.map(pod => {
    const metrics = usageMap[pod.name]
    return {
      name: pod.name,
      status: pod.phase || pod.status || 'Unknown',
      cpu: metrics?.usage?.cpu || '0m',
      memory: metrics?.usage?.memory || '0Mi'
    }
  })
}

const refreshNamespaceMetrics = async (namespaceName) => {
  if (!props.selectedClusterId || !namespaceName) return

  namespacesLoading.value = true
  try {
    await fetchNamespaceMetrics(namespaceName)

    const namespace = namespaceData.value.find(item => item.name === namespaceName)
    if (namespace) {
      namespace.podCount = namespaceMetrics.value[namespaceName]?.podCount ?? namespace.podCount ?? 0
    }

    if (selectedNamespace.value?.name === namespaceName) {
      selectedNamespace.value = {
        ...selectedNamespace.value,
        podCount: namespaceMetrics.value[namespaceName]?.podCount ?? selectedNamespace.value.podCount ?? 0,
        metrics: namespaceMetrics.value[namespaceName]
      }
      namespacePods.value = await fetchNamespacePodList(namespaceName)
    }

    ElMessage.success(`命名空间 ${namespaceName} 监控数据已刷新`)
  } catch (error) {
    ElMessage.error(error.message || '刷新监控数据失败')
  } finally {
    namespacesLoading.value = false
  }
}

const viewNamespaceDetail = async (namespace) => {
  if (!props.selectedClusterId) return

  namespacesLoading.value = true
  try {
    let metrics = namespaceMetrics.value[namespace.name]
    if (!metrics) {
      await fetchNamespaceMetrics(namespace.name)
      metrics = namespaceMetrics.value[namespace.name]
    }

    selectedNamespace.value = {
      ...namespace,
      podCount: metrics?.podCount ?? namespace.podCount ?? 0,
      serviceCount: namespace.serviceCount ?? 0,
      metrics: metrics || createEmptyNamespaceMetrics()
    }

    namespacePods.value = await fetchNamespacePodList(namespace.name)
    namespaceDetailDialogVisible.value = true
  } catch (error) {
    ElMessage.error(error.message || '获取命名空间详情失败')
  } finally {
    namespacesLoading.value = false
  }
}

const closeNamespaceDetailDialog = () => {
  namespaceDetailDialogVisible.value = false
  selectedNamespace.value = null
  namespacePods.value = []
}

const refreshAllData = async () => {
  if (!props.selectedClusterId) {
    namespaceData.value = []
    namespaceMetrics.value = {}
    closeNamespaceDetailDialog()
    return
  }

  namespacesLoading.value = true
  try {
    closeNamespaceDetailDialog()
    await fetchNamespaceList()

    await Promise.all(namespaceData.value.map(async namespace => {
      try {
        await fetchNamespaceMetrics(namespace.name)
        namespace.podCount = namespaceMetrics.value[namespace.name]?.podCount ?? namespace.podCount ?? 0
      } catch (error) {
        // 保留卡片展示，单个命名空间监控异常不阻断整体页面
      }
    }))
  } catch (error) {
    console.error('获取命名空间监控数据失败:', error)
    namespaceData.value = []
    namespaceMetrics.value = {}
    ElMessage.error(error.message || '获取命名空间监控数据失败')
  } finally {
    namespacesLoading.value = false
  }
}

const handleClusterChange = async () => {
  await refreshAllData()
}

// 暴露方法给父组件
defineExpose({
  refreshAllData,
  handleClusterChange
})

onMounted(() => {
  refreshAllData()
})
</script>

<style scoped>
.namespaces-monitoring {
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

/* 命名空间卡片 */
.namespace-card {
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

.namespace-card::before {
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

.namespace-card:hover {
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
  margin-bottom: 16px;
}

.namespace-info-header {
  display: flex;
  align-items: center;
  gap: 16px;
}

.namespace-icon {
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
}

.pulse-ring {
  display: none;
}

.namespace-details h4 {
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
}

.status-tag.active {
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

.namespace-name {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 13px;
  color: var(--aiops-text-secondary);
  padding: 4px 8px;
  background: rgba(255, 255, 255, 0.72);
  border-radius: 999px;
  border: 1px solid rgba(223, 231, 242, 0.9);
}

/* 监控指标 */
.namespace-metrics {
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

.metric-header .metric-icon {
  width: 20px;
  height: 20px;
  filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.1));
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

.metric-value.is-healthy,
.stat-value.is-healthy {
  color: var(--aiops-success);
}

.metric-value.is-notice,
.stat-value.is-notice {
  color: var(--aiops-brand);
}

.metric-value.is-warning,
.stat-value.is-warning {
  color: var(--aiops-warning);
}

.metric-value.is-danger,
.stat-value.is-danger {
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
}

.pods-icon {
  background: rgba(76, 139, 245, 0.12);
}

.services-icon {
  background: rgba(243, 178, 75, 0.14);
}

.icon-dot, .icon-pulse {
  width: 12px;
  height: 12px;
  background: white;
  border-radius: 50%;
  position: relative;
}

.icon-pulse {
  animation: iconPulse 2s infinite;
}

.stat-content {
  flex: 1;
}

.stat-label {
  display: block;
  font-size: 11px;
  color: var(--aiops-text-muted);
  font-weight: 500;
  margin-bottom: 2px;
}

.stat-value {
  display: block;
  font-size: 16px;
  font-weight: 700;
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

.no-data-text {
  color: var(--aiops-text-secondary);
  font-size: 14px;
  margin: 0;
}

/* 操作按钮 */
.namespace-actions {
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
  color: inherit;
}

.refresh-icon {
  animation: none;
  transition: transform 0.3s ease;
}

.secondary-btn:hover .refresh-icon {
  animation: refreshSpin 0.8s ease-in-out;
}

.btn-glow {
  display: none;
}

/* 对话框样式 */
.namespace-detail-dialog :deep(.el-dialog) {
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
  backdrop-filter: blur(18px);
  border-radius: 24px;
  border: 1px solid rgba(223, 231, 242, 0.96);
  max-height: 90vh;
  overflow: hidden;
}

.namespace-detail-dialog :deep(.el-dialog__header) {
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.84);
  padding: 16px 20px;
}

.namespace-detail-dialog :deep(.el-dialog__title) {
  color: var(--aiops-text-primary);
  font-weight: 700;
  font-size: 18px;
}

.namespace-detail-dialog :deep(.el-dialog__body) {
  padding: 24px;
  max-height: 70vh;
  overflow-y: auto;
}

.namespace-detail-content {
  padding: 0;
}

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

.header-icon,
.metric-icon-detail {
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

.detail-info-card {
  height: 100%;
}

.system-info {
  padding: 0;
}

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
  gap: 12px;
  padding: 12px 16px;
  background: rgba(248, 250, 255, 0.98);
  font-weight: 600;
  font-size: 13px;
  color: var(--aiops-text-muted);
  border-bottom: 1px solid rgba(223, 231, 242, 0.9);
}

.namespace-pod-header,
.namespace-pod-row {
  grid-template-columns: 2.5fr 1fr 1fr 1fr;
}

.pod-body {
  max-height: 300px;
  overflow-y: auto;
}

.pod-row {
  display: grid;
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

.pod-status-col {
  justify-content: flex-start;
}

.pod-status {
  width: fit-content;
  min-width: 70px;
}

.resource-value {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  color: var(--aiops-text-primary);
  font-weight: 500;
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

/* 动画 */
@keyframes pulse {
  0% { opacity: 1; transform: translate(-50%, -50%) scale(0.8); }
  50% { opacity: 0.5; transform: translate(-50%, -50%) scale(1.1); }
  100% { opacity: 0; transform: translate(-50%, -50%) scale(1.4); }
}

@keyframes statusPulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

@keyframes iconFloat {
  0%, 100% { transform: translateY(0px); }
  50% { transform: translateY(-2px); }
}

@keyframes iconPulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.7; transform: scale(1.1); }
}

@keyframes loadingDot {
  0%, 80%, 100% { transform: scale(0); }
  40% { transform: scale(1); }
}

@keyframes refreshSpin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* 响应式设计 */
@media (max-width: 768px) {
  .namespaces-monitoring {
    padding: 12px;
  }
  
  .namespace-card {
    padding: 16px;
  }
  
  .namespace-actions {
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
