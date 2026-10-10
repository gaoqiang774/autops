<template>
  <div class="node-details-container">
    <div class="node-details-header">
      <el-button
        @click="goBack"
        :icon="ArrowLeft"
        type="primary"
        size="small"
        class="back-btn"
      >
        返回
      </el-button>
      <h1 class="page-title">{{ nodeData?.basic?.name || '节点详情' }}</h1>
    </div>

    <div class="node-details-content" v-loading="loading">
      <div v-if="!loading && nodeData" class="main-grid">
        <!-- Row 1: 资源概览卡片（4列） -->
        <div class="stats-grid">
          <!-- CPU使用率 -->
          <div class="stat-card stat-card-resource">
            <div class="stat-icon stat-icon-cpu">
              <img src="@/assets/image/cpu.svg" alt="CPU" class="svg-icon" />
            </div>
            <div class="stat-content stat-content-resource">
              <div class="stat-topline">
                <div class="stat-label">CPU使用率</div>
                <div class="stat-value stat-value-compact" :class="getCpuUsageClass()">{{ formatUsageRate(cpuUsageRate) }}</div>
              </div>
              <div class="resource-visual">
                <div
                  class="usage-donut usage-donut-compact cpu-donut"
                  :style="getDonutStyle(cpuUsageRate, 'cpu')"
                >
                  <div class="usage-donut-center">
                    <strong>{{ formatUsageRate(cpuUsageRate) }}</strong>
                    <span>CPU</span>
                  </div>
                </div>
                <div class="metric-summary">
                  <div class="metric-detail-chip">
                    <span>可分配</span>
                    <strong>{{ formatCpuCores(nodeData.resources?.cpu?.allocatable) }}</strong>
                  </div>
                  <div class="metric-detail-chip">
                    <span>总容量</span>
                    <strong>{{ formatCpuCores(nodeData.resources?.cpu?.capacity) }}</strong>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- 内存使用率 -->
          <div class="stat-card stat-card-resource">
            <div class="stat-icon stat-icon-memory">
              <img src="@/assets/image/内存.svg" alt="内存" class="svg-icon" />
            </div>
            <div class="stat-content stat-content-resource">
              <div class="stat-topline">
                <div class="stat-label">内存使用率</div>
                <div class="stat-value stat-value-compact" :class="getMemoryUsageClass()">{{ formatUsageRate(memoryUsageRate) }}</div>
              </div>
              <div class="resource-visual">
                <div
                  class="usage-donut usage-donut-compact memory-donut"
                  :style="getDonutStyle(memoryUsageRate, 'memory')"
                >
                  <div class="usage-donut-center">
                    <strong>{{ formatUsageRate(memoryUsageRate) }}</strong>
                    <span>内存</span>
                  </div>
                </div>
                <div class="metric-summary">
                  <div class="metric-detail-chip">
                    <span>可分配</span>
                    <strong>{{ formatMemory(nodeData.resources?.memory?.allocatable) }}</strong>
                  </div>
                  <div class="metric-detail-chip">
                    <span>总容量</span>
                    <strong>{{ formatMemory(nodeData.resources?.memory?.capacity) }}</strong>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- 运行Pod数 -->
          <div class="stat-card">
            <div class="stat-icon stat-icon-pods">
              <el-icon><Box /></el-icon>
            </div>
            <div class="stat-content">
              <div class="stat-label">运行Pod数</div>
              <div class="stat-value">{{ nodeData.resources?.pods?.current || 0 }}</div>
              <div class="stat-sub">总容量: {{ nodeData.resources?.pods?.capacity || 0 }}</div>
              <el-progress
                :percentage="podUsageRate"
                :color="getProgressColor(podUsageRate)"
                :show-text="false"
                class="mini-progress"
              />
            </div>
          </div>

          <!-- 运行时间 -->
          <div class="stat-card">
            <div class="stat-icon stat-icon-time">
              <el-icon><Timer /></el-icon>
            </div>
            <div class="stat-content">
              <div class="stat-label">运行时间</div>
              <div class="stat-value">{{ formatTime(nodeData.basic?.creationTime) || 'N/A' }}</div>
              <div class="stat-sub">
                <el-tag
                  :type="getStatusType(nodeData.basic?.status)"
                  size="small"
                >
                  {{ getStatusText(nodeData.basic?.status) }}
                </el-tag>
              </div>
            </div>
          </div>
        </div>

        <!-- Row 2: 节点历史趋势 -->
        <div class="history-card">
          <div class="card-header">
            <el-icon><TrendCharts /></el-icon>
            <h3>节点历史趋势</h3>
            <el-tag v-if="hasHistoryCharts" size="small" type="info">
              {{ historyCharts[0]?.points.length || 0 }} 个采样点
            </el-tag>
          </div>
          <div class="card-content">
            <div v-if="hasHistoryCharts" class="history-grid">
              <div
                v-for="chart in historyCharts"
                :key="chart.key"
                class="history-panel"
              >
                <div class="history-panel-head">
                  <span class="history-title">{{ chart.title }}</span>
                  <span class="history-value">{{ formatHistoryValue(chart.latest, chart) }}</span>
                </div>
                <div
                  class="history-chart"
                  :ref="el => setHistoryChartRef(chart.key, el)"
                ></div>
              </div>
            </div>
            <div v-else class="history-empty">
              暂无 CPU / 内存历史趋势数据
            </div>
          </div>
        </div>

        <!-- Row 2: 基本信息卡片（3列） -->
        <div class="info-grid">
          <!-- 基本信息 -->
          <div class="info-card">
            <div class="card-header">
              <el-icon><InfoFilled /></el-icon>
              <h3>基本信息</h3>
            </div>
            <div class="card-content">
              <div class="info-row">
                <span class="label">节点名称：</span>
                <span class="value">{{ nodeData.basic?.name || 'N/A' }}</span>
              </div>
              <div class="info-row">
                <span class="label">节点角色：</span>
                <span class="value">{{ nodeData.basic?.role || 'N/A' }}</span>
              </div>
              <div class="info-row">
                <span class="label">节点状态：</span>
                <el-tag
                  :type="getStatusType(nodeData.basic?.status)"
                  class="status-tag"
                >
                  {{ getStatusText(nodeData.basic?.status) }}
                </el-tag>
              </div>
              <div class="info-row">
                <span class="label">创建时间：</span>
                <span class="value">{{ formatTime(nodeData.basic?.creationTime) || 'N/A' }}</span>
              </div>
              <div class="info-row">
                <span class="label">K8s版本：</span>
                <span class="value">{{ nodeData.basic?.k8sVersion || 'N/A' }}</span>
              </div>
              <div class="info-row">
                <span class="label">容器运行时：</span>
                <span class="value">{{ nodeData.basic?.containerRuntime || 'N/A' }}</span>
              </div>
            </div>
          </div>

          <!-- 系统信息 -->
          <div class="info-card">
            <div class="card-header">
              <el-icon><Monitor /></el-icon>
              <h3>系统信息</h3>
            </div>
            <div class="card-content">
              <div class="info-row">
                <span class="label">操作系统：</span>
                <span class="value">{{ nodeData.system?.osImage || 'N/A' }}</span>
              </div>
              <div class="info-row">
                <span class="label">内核版本：</span>
                <span class="value">{{ nodeData.system?.kernelVersion || 'N/A' }}</span>
              </div>
              <div class="info-row">
                <span class="label">架构：</span>
                <span class="value">{{ nodeData.system?.architecture || 'N/A' }}</span>
              </div>
              <div class="info-row">
                <span class="label">主机名：</span>
                <span class="value">{{ nodeData.system?.hostname || 'N/A' }}</span>
              </div>
              <div class="info-row">
                <span class="label">机器ID：</span>
                <span class="value">{{ nodeData.system?.machineId || 'N/A' }}</span>
              </div>
            </div>
          </div>

          <!-- 网络信息 -->
          <div class="info-card">
            <div class="card-header">
              <el-icon><Connection /></el-icon>
              <h3>网络信息</h3>
            </div>
            <div class="card-content">
              <div class="info-row">
                <span class="label">内部IP：</span>
                <span class="value">{{ nodeData.network?.internalIP || 'N/A' }}</span>
              </div>
              <div class="info-row">
                <span class="label">外部IP：</span>
                <span class="value">{{ nodeData.network?.externalIP || 'N/A' }}</span>
              </div>
              <div class="info-row">
                <span class="label">Pod CIDR：</span>
                <span class="value">{{ nodeData.network?.podCIDR || 'N/A' }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Row 3: 标签/污点/注解卡片（3列） -->
        <div class="metadata-grid">
          <!-- 标签信息 -->
          <div class="info-card">
            <div class="card-header">
              <el-icon><Setting /></el-icon>
              <h3>标签信息</h3>
            </div>
            <div class="card-content">
              <div v-if="nodeData.labels && Object.keys(nodeData.labels).length > 0" class="labels-table">
                <div v-for="(value, key) in nodeData.labels" :key="key" class="label-row">
                  <div class="label-key">{{ key }}</div>
                  <div class="label-value">{{ value }}</div>
                </div>
              </div>
              <div v-else class="no-data">
                暂无标签信息
              </div>
            </div>
          </div>

          <!-- 污点信息 -->
          <div class="info-card">
            <div class="card-header">
              <el-icon><Warning /></el-icon>
              <h3>污点信息</h3>
            </div>
            <div class="card-content">
              <div v-if="nodeData.taints && nodeData.taints.length > 0" class="taints-table">
                <div v-for="(taint, index) in nodeData.taints" :key="index" class="taint-row">
                  <div class="taint-key">{{ taint.key }}</div>
                  <div class="taint-value">{{ taint.value }}</div>
                  <div class="taint-effect">{{ taint.effect }}</div>
                </div>
              </div>
              <div v-else class="no-data">
                暂无污点信息
              </div>
            </div>
          </div>

          <!-- 注解信息 -->
          <div class="info-card">
            <div class="card-header">
              <el-icon><FolderOpened /></el-icon>
              <h3>注解信息</h3>
            </div>
            <div class="card-content">
              <div v-if="nodeData.annotations && Object.keys(nodeData.annotations).length > 0" class="annotations-table">
                <div v-for="(value, key) in nodeData.annotations" :key="key" class="annotation-row">
                  <div class="annotation-key">{{ key }}</div>
                  <div class="annotation-value">{{ value }}</div>
                </div>
              </div>
              <div v-else class="no-data">
                暂无注解信息
              </div>
            </div>
          </div>
        </div>

        <!-- Row 4: Pod列表（全宽） -->
        <div class="full-width-card">
          <div class="card-header pod-card-header">
            <div class="card-title-group">
              <el-icon><Box /></el-icon>
              <h3>运行中的Pod ({{ podList.length }})</h3>
            </div>
            <div class="pod-toolbar">
              <span class="pod-selection-count">已选 {{ selectedPods.length }} 个</span>
              <el-button
                type="danger"
                size="small"
                plain
                :disabled="!selectedPods.length || batchActionLoading"
                :loading="batchActionLoading && currentBatchAction === 'delete'"
                @click="handleBatchDeletePods"
              >
                批量删除
              </el-button>
              <el-button
                type="warning"
                size="small"
                plain
                :disabled="!selectedPods.length || batchActionLoading"
                :loading="batchActionLoading && currentBatchAction === 'evict'"
                @click="handleBatchEvictPods"
              >
                批量驱逐
              </el-button>
            </div>
          </div>
          <div class="card-content">
            <el-table
              ref="podTableRef"
              :data="podList"
              :row-key="podRowKey"
              style="width: 100%"
              :header-cell-style="{ background: '#6b9bd1', color: '#fff' }"
              @selection-change="handlePodSelectionChange"
            >
              <el-table-column type="selection" width="55" reserve-selection />
              <el-table-column prop="namespace" label="命名空间" width="130" />
              <el-table-column prop="name" label="Pod名称" min-width="200" show-overflow-tooltip />
              <el-table-column prop="status" label="状态" width="100">
                <template #default="scope">
                  <el-tag
                    :type="getPodStatusType(scope.row.status)"
                    size="small"
                  >
                    {{ scope.row.status }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="readyStatus" label="就绪" width="90">
                <template #default="scope">
                  {{ scope.row.readyStatus || '-' }}
                </template>
              </el-table-column>
              <el-table-column prop="containerIP" label="容器IP" width="140" />
              <el-table-column prop="cpuUsage" label="CPU使用" width="100" />
              <el-table-column prop="cpuRequest" label="CPU请求" width="100" />
              <el-table-column prop="cpuLimit" label="CPU限制" width="100" />
              <el-table-column prop="memUsage" label="内存使用" width="110" />
              <el-table-column prop="memoryRequest" label="内存请求" width="110" />
              <el-table-column prop="memoryLimit" label="内存限制" width="110" />
              <el-table-column prop="runningTime" label="运行时间" width="140">
                <template #default="scope">
                  {{ scope.row.runningTime || scope.row.age || '未知' }}
                </template>
              </el-table-column>
              <el-table-column label="操作" width="180" fixed="right">
                <template #default="scope">
                  <div class="pod-actions">
                    <el-button
                      type="danger"
                      link
                      :loading="podActionLoadingMap[getPodActionKey(scope.row, 'delete')]"
                      @click="handleDeletePod(scope.row)"
                    >
                      删除
                    </el-button>
                    <el-button
                      type="warning"
                      link
                      :loading="podActionLoadingMap[getPodActionKey(scope.row, 'evict')]"
                      @click="handleEvictPod(scope.row)"
                    >
                      驱逐
                    </el-button>
                  </div>
                </template>
              </el-table-column>
            </el-table>
            <div v-if="podList.length === 0" class="no-data">
              暂无Pod信息
            </div>
          </div>
        </div>

        <!-- Row 5: 节点状态（全宽） -->
        <div class="full-width-card">
          <div class="card-header">
            <el-icon><CircleCheck /></el-icon>
            <h3>节点状态</h3>
          </div>
          <div class="card-content">
            <el-table
              :data="conditionsList"
              style="width: 100%"
              :header-cell-style="{ background: '#6b9bd1', color: '#fff' }"
            >
              <el-table-column prop="type" label="类型" width="180" />
              <el-table-column prop="status" label="状态" width="100">
                <template #default="scope">
                  <el-tag
                    :type="scope.row.status === 'True' ? 'success' : 'danger'"
                    size="small"
                  >
                    {{ scope.row.status }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="reason" label="原因" width="280" />
              <el-table-column prop="message" label="消息" min-width="200" show-overflow-tooltip />
              <el-table-column prop="lastTransitionTime" label="最后更新时间" width="170">
                <template #default="scope">
                  {{ formatTime(scope.row.lastTransitionTime) }}
                </template>
              </el-table-column>
            </el-table>
            <div v-if="conditionsList.length === 0" class="no-data">
              暂无条件信息
            </div>
          </div>
        </div>

        <!-- Row 6: 节点事件（全宽） -->
        <div class="full-width-card">
          <div class="card-header">
            <el-icon><Bell /></el-icon>
            <h3>节点事件 ({{ eventsList.length }})</h3>
          </div>
          <div class="card-content">
            <el-table
              :data="eventsList"
              style="width: 100%"
              :header-cell-style="{ background: '#6b9bd1', color: '#fff' }"
            >
              <el-table-column prop="type" label="类型" width="100">
                <template #default="scope">
                  <el-tag
                    :type="scope.row.type === 'Normal' ? 'success' : 'warning'"
                    size="small"
                  >
                    {{ scope.row.type }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="reason" label="原因" width="250" />
              <el-table-column prop="message" label="消息" min-width="300" show-overflow-tooltip />
              <el-table-column prop="count" label="次数" width="80" align="center" />
              <el-table-column prop="firstTime" label="首次时间" width="170">
                <template #default="scope">
                  {{ formatTime(scope.row.firstTime) }}
                </template>
              </el-table-column>
              <el-table-column prop="lastTime" label="最后时间" width="170">
                <template #default="scope">
                  {{ formatTime(scope.row.lastTime) }}
                </template>
              </el-table-column>
            </el-table>
            <div v-if="eventsList.length === 0" class="no-data">
              暂无事件信息
            </div>
          </div>
        </div>
      </div>

      <div v-else-if="!loading && !nodeData" class="no-data-container">
        <el-empty description="未找到节点信息" />
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ArrowLeft, InfoFilled, Monitor, Connection, FolderOpened, Setting, Box, CircleCheck, Warning, Timer, Bell, TrendCharts } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import k8sApi from '@/api/k8s'
import * as echarts from 'echarts'
const router = useRouter()
const route = useRoute()

const loading = ref(false)
const nodeData = ref(null)
const podActionLoadingMap = ref({})
const podTableRef = ref(null)
const selectedPods = ref([])
const batchActionLoading = ref(false)
const currentBatchAction = ref('')
const historyChartRefs = {}
const historyChartInstances = {}

const clusterId = computed(() => route.params.clusterId)
const nodeName = computed(() => route.params.nodeName)

// 计算资源使用率
const cpuUsageRate = computed(() => {
  const cpu = nodeData.value?.resources?.cpu
  if (!cpu?.metricsAvailable) return null
  return clampPercent(cpu.usageRate)
})

const memoryUsageRate = computed(() => {
  const memory = nodeData.value?.resources?.memory
  if (!memory?.metricsAvailable) return null
  return clampPercent(memory.usageRate)
})

const podUsageRate = computed(() => {
  if (!nodeData.value?.resources?.pods) return 0
  const { current, capacity } = nodeData.value.resources.pods
  return capacity > 0 ? Math.round((current / capacity) * 100) : 0
})

// Pod列表
const podList = computed(() => nodeData.value?.pods || [])

// 条件列表
const conditionsList = computed(() => nodeData.value?.conditions || [])

// 事件列表
const eventsList = computed(() => nodeData.value?.events || [])

const normalizeHistoryPoint = point => ({
  timestamp: point?.timestamp || '',
  value: Number(point?.value) || 0
})

const historyCharts = computed(() => {
  const history = nodeData.value?.history
  if (!history) return []

  const chartConfig = [
    { key: 'cpu', title: 'CPU 使用量', unit: 'cores', precision: 3, color: '#4c8bf5' },
    { key: 'memory', title: '内存使用量', unit: 'GB', precision: 3, color: '#3fb97f' },
    { key: 'cpuUsageRate', title: 'CPU 使用率', unit: '%', precision: 2, color: '#f3b24b' },
    { key: 'memoryUsageRate', title: '内存使用率', unit: '%', precision: 2, color: '#8b5cf6' }
  ]

  return chartConfig
    .map(config => {
      const points = Array.isArray(history[config.key])
        ? history[config.key].map(normalizeHistoryPoint).filter(point => point.timestamp)
        : []
      return {
        ...config,
        points,
        latest: points.length ? points[points.length - 1].value : null
      }
    })
    .filter(chart => chart.points.length)
})

const hasHistoryCharts = computed(() => historyCharts.value.length > 0)

const goBack = () => {
  router.go(-1)
}

const podRowKey = (row) => `${row.namespace}:${row.name}`

const handlePodSelectionChange = (selection) => {
  selectedPods.value = selection
}

const clearPodSelection = () => {
  selectedPods.value = []
  podTableRef.value?.clearSelection?.()
}

const getPodActionKey = (pod, action) => `${pod?.namespace || ''}:${pod?.name || ''}:${action}`

const getPodDisplayName = (pod) => {
  if (!pod?.name) return '未知Pod'
  return pod.namespace ? `${pod.namespace}/${pod.name}` : pod.name
}

const isSuccessResponse = (response) => {
  const responseData = response?.data || response
  return responseData?.code === 200 || responseData?.code === 0 || response?.status === 200
}

const getResponseData = (response) => {
  const responseData = response?.data || response
  return responseData?.data || responseData
}

const updateNodePods = (pods) => {
  if (!nodeData.value) return

  const nextPods = Array.isArray(pods) ? pods : []
  nodeData.value = {
    ...nodeData.value,
    resources: {
      ...nodeData.value.resources,
      pods: {
        ...(nodeData.value.resources?.pods || {}),
        current: nextPods.length
      }
    },
    pods: nextPods
  }
}

const removePodsFromCurrentList = (pods) => {
  if (!nodeData.value || !Array.isArray(pods) || !pods.length) return

  const removedKeys = new Set(pods.map(podRowKey))
  updateNodePods(podList.value.filter(pod => !removedKeys.has(podRowKey(pod))))
}

const refreshNodePods = async () => {
  const response = await k8sApi.getNodePods(clusterId.value, nodeName.value)
  if (!isSuccessResponse(response)) {
    const responseData = response?.data || response
    throw new Error(responseData?.message || responseData?.msg || '刷新Pod列表失败')
  }

  const data = getResponseData(response)
  updateNodePods(Array.isArray(data) ? data : data?.pods)
}

const setPodActionLoading = (pod, action, value) => {
  const key = getPodActionKey(pod, action)
  podActionLoadingMap.value = {
    ...podActionLoadingMap.value,
    [key]: value
  }
}

const runPodAction = async (pod, action, options = {}) => {
  const namespace = pod?.namespace
  const podName = pod?.name
  const actionText = action === 'evict' ? '驱逐' : '删除'

  if (!namespace || !podName) {
    ElMessage.warning('Pod 信息不完整，无法执行操作')
    return false
  }

  setPodActionLoading(pod, action, true)

  try {
    const response = action === 'evict'
      ? await k8sApi.evictPod(clusterId.value, namespace, podName)
      : await k8sApi.deletePod(clusterId.value, namespace, podName)

    if (isSuccessResponse(response)) {
      if (!options.silent) {
        ElMessage.success(`Pod "${getPodDisplayName(pod)}" ${actionText}成功`)
      }
      if (options.refresh !== false) {
        clearPodSelection()
        removePodsFromCurrentList([pod])
        try {
          await refreshNodePods()
        } catch (refreshError) {
          console.error('刷新节点Pod列表失败:', refreshError)
        }
      }
      return true
    }

    const responseData = response?.data || response
    ElMessage.error(responseData?.message || responseData?.msg || `Pod ${actionText}失败`)
    return false
  } catch (error) {
    console.error(`Pod ${actionText}失败:`, error)
    ElMessage.error(`Pod ${actionText}失败: ${error?.message || '网络错误'}`)
    return false
  } finally {
    setPodActionLoading(pod, action, false)
  }
}

const handleDeletePod = async (pod) => {
  try {
    await ElMessageBox.confirm(`确定要删除 Pod "${getPodDisplayName(pod)}" 吗？`, '删除确认', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await runPodAction(pod, 'delete')
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') {
      console.error('删除 Pod 确认流程异常:', error)
    }
  }
}

const handleEvictPod = async (pod) => {
  try {
    await ElMessageBox.confirm(`确定要驱逐 Pod "${getPodDisplayName(pod)}" 吗？`, '驱逐确认', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await runPodAction(pod, 'evict')
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') {
      console.error('驱逐 Pod 确认流程异常:', error)
    }
  }
}

const runBatchPodAction = async (action) => {
  const pods = [...selectedPods.value]
  const actionText = action === 'evict' ? '驱逐' : '删除'

  if (!pods.length) {
    ElMessage.warning(`请先选择要${actionText}的 Pod`)
    return
  }

  try {
    await ElMessageBox.confirm(`确定要${actionText}选中的 ${pods.length} 个 Pod 吗？`, `批量${actionText}确认`, {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch (error) {
    return
  }

  batchActionLoading.value = true
  currentBatchAction.value = action

  let successCount = 0
  let failedCount = 0
  const succeededPods = []

  try {
    for (const pod of pods) {
      const success = await runPodAction(pod, action, {
        silent: true,
        refresh: false
      })
      if (success) {
        successCount += 1
        succeededPods.push(pod)
      } else {
        failedCount += 1
      }
    }

    if (successCount > 0) {
      ElMessage.success(`已${actionText} ${successCount} 个 Pod${failedCount ? `，失败 ${failedCount} 个` : ''}`)
      clearPodSelection()
      removePodsFromCurrentList(succeededPods)
      try {
        await refreshNodePods()
      } catch (refreshError) {
        console.error('刷新节点Pod列表失败:', refreshError)
      }
    } else {
      ElMessage.error(`批量${actionText}失败`)
    }
  } finally {
    batchActionLoading.value = false
    currentBatchAction.value = ''
  }
}

const handleBatchDeletePods = () => {
  runBatchPodAction('delete')
}

const handleBatchEvictPods = () => {
  runBatchPodAction('evict')
}

const getStatusType = (status) => {
  switch (status?.toLowerCase()) {
    case 'ready': return 'success'
    case 'notready': return 'danger'
    case 'schedulingdisabled': return 'warning'
    default: return 'info'
  }
}

const getStatusText = (status) => {
  switch (status?.toLowerCase()) {
    case 'ready': return '就绪'
    case 'notready': return '未就绪'
    case 'schedulingdisabled': return '禁止调度'
    default: return status || '未知'
  }
}

const getPodStatusType = (status) => {
  switch (status?.toLowerCase()) {
    case 'ready': return 'success'
    case 'not ready': return 'warning'
    case 'running': return 'success'
    case 'pending': return 'warning'
    case 'failed': return 'danger'
    case 'succeeded': return 'success'
    default: return 'info'
  }
}

const getProgressColor = (percentage) => {
  if (percentage < 60) return '#67C23A'
  if (percentage < 80) return '#E6A23C'
  return '#F56C6C'
}

const toNumber = (value) => {
  const number = Number(value)
  return Number.isFinite(number) ? number : 0
}

const clampPercent = (value) => {
  const number = toNumber(value)
  if (number < 0) return 0
  if (number > 100) return 100
  return Math.round(number)
}

const formatUsageRate = (value) => {
  if (value === null || value === undefined || value === '') return '--'
  return `${clampPercent(value)}%`
}

const formatCpuCores = (value) => {
  const number = Number(value)
  if (!Number.isFinite(number)) return '--'
  const text = Number.isInteger(number) ? String(number) : number.toFixed(1)
  return `${text}核`
}

const parseCpuCores = (value, rawMilli) => {
  const raw = Number(rawMilli)
  if (Number.isFinite(raw) && raw > 0) return raw / 1000
  if (value === null || value === undefined || value === '') return 0

  const text = String(value).trim()
  if (text.endsWith('m')) return toNumber(text.slice(0, -1)) / 1000
  return toNumber(text.replace(/cores?/i, '').trim())
}

const parseMemoryBytes = (value, rawBytes) => {
  const raw = Number(rawBytes)
  if (Number.isFinite(raw) && raw > 0) return raw
  if (value === null || value === undefined || value === '') return 0

  const text = String(value).trim()
  const match = text.match(/^([\d.]+)\s*([KMGT]i?|B)?$/i)
  if (!match) return toNumber(text)

  const number = toNumber(match[1])
  const unit = (match[2] || 'B').toLowerCase()
  const multipliers = {
    b: 1,
    k: 1000,
    kb: 1000,
    ki: 1024,
    m: 1000 ** 2,
    mb: 1000 ** 2,
    mi: 1024 ** 2,
    g: 1000 ** 3,
    gb: 1000 ** 3,
    gi: 1024 ** 3,
    t: 1000 ** 4,
    tb: 1000 ** 4,
    ti: 1024 ** 4
  }
  return number * (multipliers[unit] || 1)
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

const getValueByPath = (source, path) => {
  if (!source || !path) return undefined
  return path.split('.').reduce((current, key) => current?.[key], source)
}

const pickFirstValue = (source, paths) => {
  for (const path of paths) {
    const value = getValueByPath(source, path)
    if (value !== undefined && value !== null && value !== '') {
      return value
    }
  }
  return ''
}

const normalizeTimestamp = (timestamp) => {
  if (!timestamp) return ''
  if (typeof timestamp === 'string' || typeof timestamp === 'number') return timestamp
  if (timestamp instanceof Date) return timestamp

  return timestamp.time ||
    timestamp.Time ||
    timestamp.createdAt ||
    timestamp.creationTime ||
    timestamp.creationTimestamp ||
    ''
}

const parseTimestamp = (timestamp) => {
  const value = normalizeTimestamp(timestamp)
  if (!value) return null
  if (value instanceof Date) return Number.isNaN(value.getTime()) ? null : value
  if (typeof value === 'number') {
    const numericDate = new Date(value)
    return Number.isNaN(numericDate.getTime()) ? null : numericDate
  }

  const rawValue = String(value).trim()
  const candidates = [rawValue]

  if (/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}(:\d{2})?$/.test(rawValue)) {
    candidates.push(rawValue.replace(' ', 'T'))
  }

  const goTimeMatch = rawValue.match(/^(\d{4}-\d{2}-\d{2}) (\d{2}:\d{2}:\d{2})(?:\.\d+)? ([+-]\d{2})(\d{2})(?: [A-Z]+)?$/)
  if (goTimeMatch) {
    candidates.push(`${goTimeMatch[1]}T${goTimeMatch[2]}${goTimeMatch[3]}:${goTimeMatch[4]}`)
  }

  for (const candidate of candidates) {
    const date = new Date(candidate)
    if (!Number.isNaN(date.getTime())) return date
  }

  return null
}

const formatTime = (timestamp) => {
  const date = parseTimestamp(timestamp)
  if (!date) return ''

  return date.toLocaleString('zh-CN')
}

const formatMemory = (bytes) => {
  if (!bytes) return '0B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let index = 0
  let size = bytes
  
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024
    index++
  }
  
  return `${size.toFixed(index === 0 ? 0 : 1)}${units[index]}`
}

const setHistoryChartRef = (key, el) => {
  if (el) {
    historyChartRefs[key] = el
  } else {
    delete historyChartRefs[key]
  }
}

const formatHistoryTime = (time, includeDate = false) => {
  if (!time) return ''
  const date = new Date(time)
  if (Number.isNaN(date.getTime())) return time
  return date.toLocaleString('zh-CN', {
    month: includeDate ? '2-digit' : undefined,
    day: includeDate ? '2-digit' : undefined,
    hour: '2-digit',
    minute: '2-digit',
    hour12: false
  })
}

const formatHistoryValue = (value, chart) => {
  if (value === null || value === undefined) return '--'
  const numberValue = Number(value)
  const formatted = Number.isFinite(numberValue)
    ? numberValue.toFixed(chart.precision)
    : value
  return chart.unit ? `${formatted}${chart.unit}` : formatted
}

const buildHistoryChartOption = chart => ({
  color: [chart.color],
  grid: {
    top: 14,
    right: 16,
    bottom: 26,
    left: 42
  },
  tooltip: {
    trigger: 'axis',
    backgroundColor: 'rgba(15, 23, 42, 0.88)',
    borderWidth: 0,
    textStyle: {
      color: '#fff',
      fontSize: 12
    },
    formatter(params) {
      const item = params?.[0]
      if (!item) return ''
      const point = chart.points[item.dataIndex] || {}
      return `${formatHistoryTime(point.timestamp, true)}<br/>${chart.title}: ${formatHistoryValue(item.value, chart)}`
    }
  },
  xAxis: {
    type: 'category',
    boundaryGap: false,
    data: chart.points.map(point => formatHistoryTime(point.timestamp)),
    axisTick: {
      show: false
    },
    axisLine: {
      lineStyle: {
        color: 'rgba(148, 163, 184, 0.55)'
      }
    },
    axisLabel: {
      color: '#8d97a8',
      fontSize: 11,
      hideOverlap: true
    }
  },
  yAxis: {
    type: 'value',
    min: chart.unit === '%' ? 0 : null,
    splitNumber: 3,
    axisLabel: {
      color: '#8d97a8',
      fontSize: 11,
      formatter(value) {
        return chart.unit === '%' ? `${value}%` : value
      }
    },
    splitLine: {
      lineStyle: {
        color: 'rgba(226, 232, 240, 0.88)',
        type: 'dashed'
      }
    }
  },
  series: [
    {
      name: chart.title,
      type: 'line',
      smooth: true,
      symbol: 'none',
      lineStyle: {
        width: 2,
        color: chart.color
      },
      areaStyle: {
        color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
          { offset: 0, color: `${chart.color}33` },
          { offset: 1, color: `${chart.color}05` }
        ])
      },
      data: chart.points.map(point => point.value)
    }
  ]
})

const disposeHistoryCharts = () => {
  Object.values(historyChartInstances).forEach(chart => {
    chart?.dispose()
  })
  Object.keys(historyChartInstances).forEach(key => {
    delete historyChartInstances[key]
  })
}

const renderHistoryCharts = () => {
  nextTick(() => {
    const activeKeys = new Set(historyCharts.value.map(chart => chart.key))
    Object.keys(historyChartInstances).forEach(key => {
      if (!activeKeys.has(key)) {
        historyChartInstances[key]?.dispose()
        delete historyChartInstances[key]
      }
    })

    historyCharts.value.forEach(chart => {
      const chartDom = historyChartRefs[chart.key]
      if (!chartDom) return
      if (!historyChartInstances[chart.key]) {
        historyChartInstances[chart.key] = echarts.init(chartDom, null, { renderer: 'canvas' })
      }
      historyChartInstances[chart.key].setOption(buildHistoryChartOption(chart), true)
      historyChartInstances[chart.key].resize()
    })
  })
}

const handleHistoryResize = () => {
  Object.values(historyChartInstances).forEach(chart => {
    chart?.resize()
  })
}

const fetchNodeDetail = async () => {
  try {
    loading.value = true
    console.log('开始获取节点详情，clusterId:', clusterId.value, 'nodeName:', nodeName.value)
    
    const response = await k8sApi.getNodeDetail(clusterId.value, nodeName.value)
    
    console.log('节点详情API响应:', response)
    console.log('响应类型:', typeof response)
    console.log('响应code:', response?.code)
    console.log('响应data:', response?.data)
    
    // 检查响应格式，可能需要处理嵌套的响应结构
    const responseData = response.data || response
    let actualData = responseData.data || responseData
    
    console.log('处理后的数据:', actualData)
    
    if (response?.code === 200 || responseData?.code === 200) {
      // 兼容后端返回数组或单个对象两种情况
      let data = actualData
      if (Array.isArray(actualData)) {
        data = actualData.find(n => n?.name === nodeName.value || n?.internalIP === nodeName.value)
        if (!data) {
          ElMessage.error('未在返回列表中找到该节点')
          return
        }
      }
      console.log('节点数据:', data)
      
      if (!data) {
        console.error('API响应中没有数据')
        ElMessage.error('获取节点详情失败: API响应中没有数据')
        return
      }
      
      const creationTime = pickFirstValue(data, [
        'createdAt',
        'creationTime',
        'basic.creationTime',
        'metadata.creationTimestamp',
        'metadata.creation_time',
        'CreationTimestamp',
        'creationTimestamp'
      ])

      const cpuResource = data.resources?.cpu || {}
      const memoryResource = data.resources?.memory || {}
      const cpuMetricsAvailable = Boolean(cpuResource.metricsAvailable)
      const memoryMetricsAvailable = Boolean(memoryResource.metricsAvailable)

      // 根据实际API响应结构重新构造nodeData
      nodeData.value = {
        basic: {
          name: data.name,
          role: data.configuration?.role || data.roles,
          status: data.status,
          creationTime,
          k8sVersion: data.runtime?.kubeletVersion,
          containerRuntime: data.runtime?.containerRuntimeVersion
        },
        system: {
          osImage: data.runtime?.osImage || data.configuration?.osImage,
          kernelVersion: data.configuration?.kernelVersion,
          architecture: data.configuration?.architecture,
          hostname: data.name,
          machineId: data.configuration?.labels?.['kubernetes.io/hostname'] || data.name
        },
        network: {
          internalIP: data.internalIP,
          externalIP: data.externalIP || '无',
          podCIDR: data.podCIDR || (Array.isArray(data.podCIDRs) ? data.podCIDRs.join(', ') : '') || '未知'
        },
        resources: {
          cpu: {
            usageRate: cpuResource.usageRate ?? data.metrics?.cpuUsagePercentage ?? 0,
            requestRate: cpuResource.requestRate ?? 0,
            metricsAvailable: cpuMetricsAvailable,
            requests: parseCpuCores(cpuResource.requests, cpuResource.requestsRawMilli),
            capacity: parseCpuCores(cpuResource.capacity, cpuResource.capacityRawMilli),
            allocatable: parseCpuCores(cpuResource.allocatable, cpuResource.allocatableRawMilli)
          },
          memory: {
            usageRate: memoryResource.usageRate ?? data.metrics?.memoryUsagePercentage ?? 0,
            requestRate: memoryResource.requestRate ?? 0,
            metricsAvailable: memoryMetricsAvailable,
            requests: parseMemoryBytes(memoryResource.requests, memoryResource.requestsRawBytes),
            capacity: parseMemoryBytes(memoryResource.capacity, memoryResource.capacityRawBytes),
            allocatable: parseMemoryBytes(memoryResource.allocatable, memoryResource.allocatableRawBytes)
          },
          pods: {
            current: data.podMetrics?.allocated || data.pods?.length || 0,
            capacity: data.podMetrics?.total || 110
          }
        },
        labels: data.configuration?.labels || {},
        annotations: data.configuration?.annotations || {},
        conditions: data.conditions || [],
        events: data.events || [],
        pods: data.pods || [],
        history: data.history || null
      }
      
      renderHistoryCharts()
      console.log('构造的nodeData:', nodeData.value)
      console.log('设置loading为false')
      
    } else {
      const errorMsg = response?.message || response?.msg || responseData?.message || responseData?.msg || '未知错误'
      console.error('API返回错误:', errorMsg)
      ElMessage.error('获取节点详情失败: ' + errorMsg)
    }
  } catch (error) {
    console.error('获取节点详情失败:', error)
    if (error && error.stack) {
      console.error('错误堆栈:', error.stack)
    }
    const errorMsg = error?.message || error?.msg || '网络错误'
    ElMessage.error('获取节点详情失败: ' + errorMsg)
  } finally {
    loading.value = false
  }
}

// 获取CPU使用率样式类
const getCpuUsageClass = () => {
  const usage = cpuUsageRate.value
  if (usage >= 80) return 'usage-high'
  if (usage >= 60) return 'usage-medium'
  return 'usage-normal'
}

// 获取内存使用率样式类
const getMemoryUsageClass = () => {
  const usage = memoryUsageRate.value
  if (usage >= 80) return 'usage-high'
  if (usage >= 60) return 'usage-medium'
  return 'usage-normal'
}

onMounted(() => {
  if (clusterId.value && nodeName.value) {
    fetchNodeDetail()
    window.addEventListener('resize', handleHistoryResize)
  } else {
    ElMessage.error('缺少必要参数')
    goBack()
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleHistoryResize)
  disposeHistoryCharts()
})
</script>

<style scoped>
.node-details-container {
  --aiops-bg-page: #f4f7fb;
  --aiops-surface-strong: rgba(255, 255, 255, 0.92);
  --aiops-surface-warm-strong: rgba(255, 248, 239, 0.96);
  --aiops-text-primary: #172133;
  --aiops-text-secondary: #5f6b7c;
  --aiops-text-muted: #8d97a8;
  --aiops-border-cool: #dfe7f2;
  --aiops-brand: #4c8bf5;
  --aiops-success: #3fb97f;
  --aiops-warning: #f3b24b;
  --aiops-danger: #ef6b73;
  min-height: 100vh;
  background:
    radial-gradient(circle at 16% 10%, rgba(76, 139, 245, 0.06), transparent 26%),
    linear-gradient(180deg, #f7f9fc 0%, var(--aiops-bg-page) 100%);
  padding: 6px;
  position: relative;
  box-sizing: border-box;
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.node-details-header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 12px;
  padding: 16px 18px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  backdrop-filter: blur(18px);
  border-radius: 24px;
  box-shadow: 0 20px 60px rgba(17, 24, 39, 0.07);
  border: 1px solid rgba(223, 231, 242, 0.92);
}

.back-btn {
  height: 32px;
  border-radius: 13px;
  background: linear-gradient(180deg, #f6bd65 0%, #f3a33a 100%) !important;
  border: none !important;
  color: #fff !important;
  font-weight: 600;
  box-shadow: 0 10px 18px rgba(243, 163, 58, 0.16);
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.back-btn:hover {
  transform: translateY(-1px);
  box-shadow: 0 12px 22px rgba(243, 163, 58, 0.22);
}

.page-title {
  color: var(--aiops-text-primary);
  font-size: 22px;
  font-weight: 700;
  margin: 0;
}

.pod-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}

.pod-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.card-title-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.pod-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.pod-selection-count {
  font-size: 12px;
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

.node-details-content {
  width: 100%;
}

.main-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
}

/* Row 1: 资源概览卡片（4列） */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}

.stat-card {
  background: linear-gradient(180deg, rgba(255, 253, 249, 0.88), rgba(247, 250, 255, 0.84));
  backdrop-filter: blur(14px);
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 18px;
  padding: 16px;
  display: flex;
  align-items: center;
  gap: 16px;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
}

.stat-card:hover {
  border-color: rgba(171, 189, 220, 0.95);
  box-shadow: 0 14px 28px rgba(17, 24, 39, 0.06);
}

.stat-icon {
  width: 44px;
  height: 44px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 24px;
  flex-shrink: 0;
}

.stat-icon-cpu,
.stat-icon-memory,
.stat-icon-pods,
.stat-icon-time {
  background: rgba(76, 139, 245, 0.12);
  color: var(--aiops-brand);
  box-shadow: none;
}

.stat-card-resource {
  align-items: flex-start;
}

.stat-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stat-content-resource {
  gap: 12px;
}

.stat-topline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.stat-label {
  font-size: 12px;
  color: var(--aiops-text-muted);
  font-weight: 600;
}

.stat-value {
  font-size: 22px;
  font-weight: 700;
  color: var(--aiops-text-primary);
}

.stat-sub {
  font-size: 12px;
  color: var(--aiops-text-secondary);
}

.stat-value-compact {
  font-size: 18px;
}

.resource-visual {
  display: flex;
  align-items: center;
  gap: 18px;
}

.usage-donut {
  --usage: 0%;
  --donut-primary: #4c8bf5;
  --donut-secondary: #79b1ff;
  --donut-track: rgba(76, 139, 245, 0.14);
  --donut-glow: rgba(76, 139, 245, 0.24);
  position: relative;
  flex: 0 0 112px;
  width: 112px;
  height: 112px;
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
  inset: 11px;
  border-radius: 50%;
  background:
    radial-gradient(circle at 30% 30%, rgba(255, 255, 255, 0.96), rgba(246, 249, 255, 0.98) 68%, rgba(238, 244, 252, 0.98) 100%);
  box-shadow: inset 0 0 0 1px rgba(223, 231, 242, 0.88);
}

.usage-donut::after {
  content: '';
  position: absolute;
  inset: -8px;
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
  font-size: 22px;
  line-height: 1;
  color: var(--aiops-text-primary);
}

.usage-donut-center span {
  font-size: 11px;
  color: var(--aiops-text-muted);
  letter-spacing: 0.04em;
}

.usage-donut-compact {
  flex-basis: 110px;
  width: 110px;
  height: 110px;
}

.metric-summary {
  flex: 1;
  display: grid;
  gap: 10px;
}

.metric-detail-chip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  border-radius: 12px;
  background: linear-gradient(180deg, rgba(249, 251, 255, 0.96), rgba(243, 247, 253, 0.92));
  border: 1px solid rgba(223, 231, 242, 0.92);
}

.metric-detail-chip span {
  font-size: 12px;
  color: var(--aiops-text-muted);
}

.metric-detail-chip strong {
  font-size: 13px;
  font-weight: 700;
  color: var(--aiops-text-primary);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
}

.mini-progress {
  margin-top: 8px;
}

.mini-progress :deep(.el-progress-bar__outer) {
  height: 6px;
  border-radius: 3px;
  background: rgba(223, 231, 242, 0.85);
}

.mini-progress :deep(.el-progress-bar__inner) {
  border-radius: 3px;
}

/* Row 2 & 3: 信息卡片（3列） */
.info-grid,
.metadata-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}

.info-card {
  background: linear-gradient(135deg, var(--aiops-surface-warm-strong), rgba(247, 250, 255, 0.82));
  backdrop-filter: blur(14px);
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 18px;
  overflow: hidden;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
  display: flex;
  flex-direction: column;
}

.metadata-grid .info-card {
  height: 320px;
}

.info-card:hover {
  border-color: rgba(171, 189, 220, 0.95);
  box-shadow: 0 14px 28px rgba(17, 24, 39, 0.06) !important;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 18px;
  background: linear-gradient(90deg, rgba(255, 249, 240, 0.95), rgba(248, 248, 246, 0.86), rgba(237, 242, 253, 0.92));
  color: var(--aiops-text-primary);
  border-bottom: 1px solid rgba(223, 231, 242, 0.84);
  border-radius: 18px 18px 0 0;
}

.card-header h3 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}

.card-header :deep(.el-icon) {
  font-size: 18px;
}

.card-content {
  padding: 14px 18px;
  flex: 1;
  overflow-y: auto;
}

.info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 0;
  border-bottom: 1px solid rgba(15, 23, 42, 0.045);
  font-size: 13px;
}

.info-row:last-child {
  border-bottom: none;
}

.info-row .label {
  color: var(--aiops-text-muted);
  font-weight: 600;
  min-width: 100px;
}

.info-row .value {
  color: var(--aiops-text-primary);
  font-weight: 500;
  text-align: right;
  flex: 1;
}

.status-tag {
  font-size: 12px;
  padding: 4px 12px;
}

.history-card {
  background: linear-gradient(135deg, var(--aiops-surface-warm-strong), rgba(247, 250, 255, 0.82));
  backdrop-filter: blur(14px);
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 18px;
  overflow: hidden;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
}

.history-card:hover {
  border-color: rgba(171, 189, 220, 0.95);
  box-shadow: 0 14px 28px rgba(17, 24, 39, 0.06) !important;
}

.history-card .card-header {
  justify-content: flex-start;
}

.history-card .card-header .el-tag {
  margin-left: auto;
}

.history-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.history-panel {
  min-width: 0;
  border: 1px solid rgba(223, 231, 242, 0.76);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.72);
  padding: 12px;
}

.history-panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-height: 24px;
}

.history-title {
  color: var(--aiops-text-secondary);
  font-size: 13px;
  font-weight: 700;
}

.history-value {
  color: var(--aiops-text-primary);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 14px;
  font-weight: 700;
  white-space: nowrap;
}

.history-chart {
  width: 100%;
  height: 170px;
  margin-top: 8px;
}

.history-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 160px;
  border: 1px dashed rgba(203, 213, 225, 0.9);
  border-radius: 14px;
  color: var(--aiops-text-muted);
  font-size: 14px;
  background: rgba(248, 250, 255, 0.58);
}

/* 标签/污点/注解表格 */
.labels-table,
.taints-table,
.annotations-table {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.label-row,
.taint-row,
.annotation-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  padding: 8px;
  background: rgba(255, 255, 255, 0.68);
  border: 1px solid rgba(223, 231, 242, 0.74);
  border-radius: 12px;
  font-size: 12px;
}

.taint-row {
  grid-template-columns: 1fr 1fr 0.8fr;
}

.label-key,
.taint-key,
.annotation-key {
  color: var(--aiops-brand);
  font-weight: 600;
  word-break: break-all;
}

.label-value,
.taint-value,
.annotation-value {
  color: var(--aiops-text-secondary);
  word-break: break-all;
}

.taint-effect {
  color: var(--aiops-warning);
  font-weight: 500;
}

.no-data {
  text-align: center;
  color: #909399;
  padding: 20px;
  font-size: 13px;
}

/* Row 4 & 5: 全宽卡片 */
.full-width-card {
  background: linear-gradient(135deg, var(--aiops-surface-warm-strong), rgba(247, 250, 255, 0.82));
  backdrop-filter: blur(14px);
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 18px;
  overflow: hidden;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
  display: flex;
  flex-direction: column;
  height: 450px;
}

.full-width-card:hover {
  border-color: rgba(171, 189, 220, 0.95);
  box-shadow: 0 14px 28px rgba(17, 24, 39, 0.06) !important;
}

.full-width-card .card-content {
  padding: 20px;
  flex: 1;
  overflow-x: auto;
  overflow-y: auto;
}

.full-width-card :deep(.el-table) {
  background: transparent;
}

.full-width-card :deep(.el-table__header th) {
  background: rgba(248, 250, 255, 0.98) !important;
  color: var(--aiops-text-muted) !important;
  font-weight: 600;
  border-bottom: 1px solid rgba(223, 231, 242, 0.95);
}

.full-width-card :deep(.el-table__body tr:hover > td) {
  background-color: rgba(76, 139, 245, 0.045) !important;
}

/* Pod名称列样式 */
.full-width-card :deep(.el-table__cell) {
  padding: 8px 0 !important;
}

.full-width-card :deep(td:nth-child(2)) {
  max-width: 200px;
}

.full-width-card :deep(td:nth-child(2) .cell) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: block;
}

/* 使用率颜色 */
.usage-normal {
  color: #67c23a !important;
}

.usage-medium {
  color: #e6a23c !important;
}

.usage-high {
  color: #f56c6c !important;
}

/* SVG图标 */
.svg-icon {
  width: 26px;
  height: 26px;
}

.no-data-container {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 400px;
}

@media (max-width: 1200px) {
  .stats-grid,
  .history-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .info-grid,
  .metadata-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .node-details-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .stats-grid,
  .history-grid {
    grid-template-columns: 1fr;
  }
}
</style>
