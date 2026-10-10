<template>
  <div class="windows-host-detail-page" v-loading="loading">
    <div class="page-header">
      <div class="header-content">
        <el-button :icon="ArrowLeft" class="back-btn" @click="goBack">返回</el-button>

        <div class="title-block">
          <h2 class="page-title">{{ headerTitle }}</h2>
          <div class="page-subtitle">{{ dashboard.osProduct || 'Windows 主机监控仪表盘' }}</div>
        </div>

        <div class="header-meta">
          <el-tag :type="statusTagType" size="large">{{ statusText }}</el-tag>
          <span class="update-time">更新时间：{{ updateTimeText || '-' }}</span>
        </div>
      </div>
    </div>

    <div class="page-toolbar">
      <el-radio-group v-model="activeDuration" size="small">
        <el-radio-button
          v-for="item in durationOptions"
          :key="item.value"
          :label="item.value"
        >
          {{ item.label }}
        </el-radio-button>
      </el-radio-group>

      <el-button size="small" :icon="Refresh" :loading="refreshing" @click="handleRefresh">
        刷新
      </el-button>
    </div>

    <div class="page-content">
      <el-alert
        v-if="isOffline"
        type="warning"
        :closable="false"
        show-icon
        title="主机当前离线，基础信息仍可查看，概览与列表数据可能为零值或空数组。"
      />

      <el-card shadow="hover" class="section-card">
        <template #header>
          <div class="section-header">
            <el-icon><Monitor /></el-icon>
            <span>基本信息</span>
          </div>
        </template>

        <el-descriptions :column="3" border>
          <el-descriptions-item label="Host ID">{{ dashboard.hostId || hostId || '-' }}</el-descriptions-item>
          <el-descriptions-item label="主机名称">{{ dashboard.hostName || '-' }}</el-descriptions-item>
          <el-descriptions-item label="IP">{{ dashboard.ip || '-' }}</el-descriptions-item>
          <el-descriptions-item label="系统主机名">{{ dashboard.hostname || '-' }}</el-descriptions-item>
          <el-descriptions-item label="操作系统">{{ dashboard.osProduct || '-' }}</el-descriptions-item>
          <el-descriptions-item label="在线状态">{{ statusText }}</el-descriptions-item>
        </el-descriptions>
      </el-card>

      <div class="overview-grid">
        <el-card
          v-for="card in overviewCards"
          :key="card.key"
          shadow="hover"
          class="overview-card"
        >
          <div class="overview-label">{{ card.label }}</div>
          <div class="overview-value">{{ card.value }}</div>
          <div class="overview-extra">{{ card.extra }}</div>
        </el-card>
      </div>

      <el-card shadow="hover" class="section-card">
        <template #header>
          <div class="section-header">
            <el-icon><TrendCharts /></el-icon>
            <span>趋势图</span>
          </div>
        </template>

        <div class="chart-grid">
          <div v-for="chart in chartCards" :key="chart.key" class="chart-panel">
            <div class="chart-panel-header">
              <span class="chart-title">{{ chart.title }}</span>
              <span class="chart-current">{{ getChartCurrentLabel(chart) }}</span>
            </div>
            <div :ref="el => setChartRef(chart.key, el)" class="chart-body"></div>
          </div>
        </div>
      </el-card>

      <el-card shadow="hover" class="section-card">
        <template #header>
          <div class="section-header">
            <el-icon><HardDrive /></el-icon>
            <span>分卷磁盘</span>
          </div>
        </template>

        <el-table :data="disks" stripe :empty-text="emptyText">
          <el-table-column prop="volume" label="盘符" min-width="90" />
          <el-table-column prop="volumeName" label="卷标" min-width="120" show-overflow-tooltip />
          <el-table-column prop="fileSystem" label="文件系统" min-width="110" />
          <el-table-column prop="type" label="类型" min-width="90" />
          <el-table-column label="使用率" min-width="110">
            <template #default="{ row }">{{ formatPercent(row.usageRate) }}</template>
          </el-table-column>
          <el-table-column prop="usedStr" label="已用" min-width="120" />
          <el-table-column prop="freeStr" label="可用" min-width="120" />
          <el-table-column prop="totalStr" label="总量" min-width="120" />
          <el-table-column label="读取" min-width="110">
            <template #default="{ row }">{{ formatRate(row.readKbPerSec) }}</template>
          </el-table-column>
          <el-table-column label="写入" min-width="110">
            <template #default="{ row }">{{ formatRate(row.writeKbPerSec) }}</template>
          </el-table-column>
        </el-table>
      </el-card>

      <div class="list-grid">
        <el-card shadow="hover" class="list-card">
          <template #header>
            <div class="section-header">
              <el-icon><List /></el-icon>
              <span>Top CPU 进程</span>
            </div>
          </template>

          <el-table :data="topCpuProcesses" stripe :empty-text="emptyText">
            <el-table-column prop="pid" label="PID" width="90" />
            <el-table-column prop="name" label="进程名" min-width="180" show-overflow-tooltip />
            <el-table-column label="CPU" width="110">
              <template #default="{ row }">{{ formatPercent(row.cpuUsage) }}</template>
            </el-table-column>
            <el-table-column prop="memoryStr" label="内存" min-width="120" />
          </el-table>
        </el-card>

        <el-card shadow="hover" class="list-card">
          <template #header>
            <div class="section-header">
              <el-icon><List /></el-icon>
              <span>Top 内存进程</span>
            </div>
          </template>

          <el-table :data="topMemoryProcesses" stripe :empty-text="emptyText">
            <el-table-column prop="pid" label="PID" width="90" />
            <el-table-column prop="name" label="进程名" min-width="180" show-overflow-tooltip />
            <el-table-column prop="memoryStr" label="内存" min-width="120" />
            <el-table-column label="CPU" width="110">
              <template #default="{ row }">{{ formatPercent(row.cpuUsage) }}</template>
            </el-table-column>
          </el-table>
        </el-card>

        <el-card shadow="hover" class="list-card">
          <template #header>
            <div class="section-header">
              <el-icon><List /></el-icon>
              <span>关键服务</span>
            </div>
          </template>

          <el-table :data="keyServices" stripe :empty-text="emptyText">
            <el-table-column prop="name" label="服务名" min-width="140" show-overflow-tooltip />
            <el-table-column prop="displayName" label="显示名" min-width="180" show-overflow-tooltip />
            <el-table-column label="运行状态" width="120">
              <template #default="{ row }">
                <el-tag :type="row.running ? 'success' : 'danger'">
                  {{ row.running ? '运行中' : '已停止' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="status" label="状态值" min-width="120" />
          </el-table>
        </el-card>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as echarts from 'echarts'
import { ElMessage } from 'element-plus'
import {
  ArrowLeft,
  HardDrive,
  List,
  Monitor,
  Refresh,
  TrendCharts
} from '@element-plus/icons-vue'
import api from '@/api'

const route = useRoute()
const router = useRouter()

const hostId = computed(() => route.params.id)

const durationOptions = [
  { label: '1小时', value: '1h' },
  { label: '6小时', value: '6h' },
  { label: '24小时', value: '24h' }
]

const chartCards = [
  {
    key: 'cpu',
    title: 'CPU 使用率',
    unit: '%',
    max: 100,
    series: [{ name: 'CPU', field: 'cpu', color: '#409eff' }]
  },
  {
    key: 'cpuQueueLength',
    title: 'CPU 队列长度',
    unit: '',
    series: [{ name: '队列长度', field: 'cpuQueueLength', color: '#e6a23c' }]
  },
  {
    key: 'memoryUsage',
    title: '内存使用率',
    unit: '%',
    max: 100,
    series: [{ name: '内存', field: 'memoryUsage', color: '#67c23a' }]
  },
  {
    key: 'diskUsage',
    title: '磁盘使用率',
    unit: '%',
    max: 100,
    series: [{ name: '磁盘', field: 'diskUsage', color: '#f56c6c' }]
  },
  {
    key: 'diskIo',
    title: '磁盘吞吐',
    unit: 'KB/s',
    series: [
      { name: '读取', field: 'diskReadKb', color: '#909399' },
      { name: '写入', field: 'diskWriteKb', color: '#e6a23c' }
    ]
  },
  {
    key: 'network',
    title: '网络流量',
    unit: 'KB/s',
    series: [
      { name: '接收', field: 'networkReceive', color: '#409eff' },
      { name: '发送', field: 'networkSend', color: '#67c23a' }
    ]
  },
  {
    key: 'processCount',
    title: '进程数',
    unit: '',
    series: [{ name: '进程数', field: 'processCount', color: '#722ed1' }]
  }
]

const createEmptyDashboard = () => ({
  hostId: '',
  hostName: '',
  ip: '',
  hostname: '',
  osProduct: '',
  onlineStatus: 0,
  overview: {},
  trends: {},
  disks: [],
  topCpuProcesses: [],
  topMemoryProcesses: [],
  keyServices: [],
  updateTime: null
})

const loading = ref(false)
const refreshing = ref(false)
const activeDuration = ref('1h')
const dashboard = ref(createEmptyDashboard())

const chartRefs = {}
const chartInstances = {}

let refreshTimer = null
let requestSeq = 0

const overview = computed(() => dashboard.value.overview || {})
const disks = computed(() => (Array.isArray(dashboard.value.disks) ? dashboard.value.disks : []))
const topCpuProcesses = computed(() => (Array.isArray(dashboard.value.topCpuProcesses) ? dashboard.value.topCpuProcesses : []))
const topMemoryProcesses = computed(() => (Array.isArray(dashboard.value.topMemoryProcesses) ? dashboard.value.topMemoryProcesses : []))
const keyServices = computed(() => (Array.isArray(dashboard.value.keyServices) ? dashboard.value.keyServices : []))
const isOffline = computed(() => Number(dashboard.value.onlineStatus) === 1)
const statusText = computed(() => (isOffline.value ? '离线' : '在线'))
const statusTagType = computed(() => (isOffline.value ? 'danger' : 'success'))
const headerTitle = computed(() => dashboard.value.hostName || dashboard.value.hostname || `Windows 主机 ${hostId.value || ''}`)
const updateTimeText = computed(() => formatDateTime(dashboard.value.updateTime))
const emptyText = computed(() => (isOffline.value ? '主机离线，暂无数据' : '暂无数据'))

const overviewCards = computed(() => [
  {
    key: 'cpu',
    label: 'CPU',
    value: formatPercent(overview.value.cpuUsage),
    extra: `${formatNumber(overview.value.cpuCores, 0)} 核 / 队列 ${formatNumber(overview.value.cpuQueueLength, 0)}`
  },
  {
    key: 'memory',
    label: '内存',
    value: formatPercent(overview.value.memoryUsage),
    extra: `${overview.value.memoryUsedStr || '-'} / ${overview.value.memoryTotalStr || '-'}`
  },
  {
    key: 'disk',
    label: '磁盘',
    value: formatPercent(overview.value.diskUsage),
    extra: `${overview.value.diskUsedStr || '-'} / ${overview.value.diskTotalStr || '-'}`
  },
  {
    key: 'network',
    label: '网络',
    value: `↓ ${formatRate(overview.value.networkReceiveKbPerSec)}`,
    extra: `↑ ${formatRate(overview.value.networkSendKbPerSec)}`
  },
  {
    key: 'process',
    label: '进程数',
    value: formatNumber(overview.value.processCount, 0),
    extra: '当前进程总数'
  },
  {
    key: 'service',
    label: '服务数',
    value: `${formatNumber(overview.value.runningServiceCount, 0)} / ${formatNumber(overview.value.serviceCount, 0)}`,
    extra: '运行中 / 总数'
  },
  {
    key: 'uptime',
    label: '运行时长',
    value: overview.value.uptimeStr || '-',
    extra: '最近一次采集结果'
  }
])

const goBack = () => {
  router.back()
}

const toNumber = value => {
  const num = Number(value)
  return Number.isFinite(num) ? num : 0
}

const normalizeTimestamp = value => {
  const ts = Number(value)
  if (!Number.isFinite(ts)) return 0
  return ts > 1e12 ? ts : ts * 1000
}

const normalizeDashboard = payload => ({
  ...createEmptyDashboard(),
  ...(payload || {}),
  overview: payload?.overview || {},
  trends: payload?.trends || {},
  disks: Array.isArray(payload?.disks) ? payload.disks : [],
  topCpuProcesses: Array.isArray(payload?.topCpuProcesses) ? payload.topCpuProcesses : [],
  topMemoryProcesses: Array.isArray(payload?.topMemoryProcesses) ? payload.topMemoryProcesses : [],
  keyServices: Array.isArray(payload?.keyServices) ? payload.keyServices : []
})

const formatNumber = (value, digits = 1) => {
  const num = Number(value)
  if (!Number.isFinite(num)) return digits === 0 ? '0' : '0.0'
  return num.toFixed(digits)
}

const formatPercent = value => {
  const num = Number(value)
  if (!Number.isFinite(num)) return '0.0%'
  return `${num.toFixed(1)}%`
}

const formatRate = value => {
  const num = Number(value)
  if (!Number.isFinite(num)) return '0.00 KB/s'
  return `${num.toFixed(2)} KB/s`
}

const formatDateTime = value => {
  if (!value) return ''

  const date = new Date(normalizeTimestamp(value))
  if (Number.isNaN(date.getTime())) return ''

  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')

  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
}

const formatAxisTime = value => {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''

  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')

  if (activeDuration.value === '24h') {
    return `${month}-${day} ${hours}:${minutes}`
  }

  return `${hours}:${minutes}`
}

const getTrendSeries = field => {
  const series = dashboard.value.trends?.[field]
  if (!Array.isArray(series)) return []

  return series
    .map(item => ({
      timestamp: normalizeTimestamp(item?.timestamp),
      value: toNumber(item?.value)
    }))
    .filter(item => item.timestamp > 0)
}

const formatChartValue = (value, unit) => {
  const num = Number(value)
  if (!Number.isFinite(num)) return '-'

  if (unit === '%') return `${num.toFixed(1)}%`
  if (unit === 'KB/s') return `${num.toFixed(2)} KB/s`
  if (!unit) return `${Math.round(num)}`
  return `${num.toFixed(2)} ${unit}`
}

const getChartCurrentLabel = chart => {
  const values = chart.series
    .map(item => {
      const list = getTrendSeries(item.field)
      const current = list.length ? list[list.length - 1].value : null
      if (current === null || current === undefined) return ''
      return `${item.name} ${formatChartValue(current, chart.unit)}`
    })
    .filter(Boolean)

  return values.join(' / ') || '暂无数据'
}

const setChartRef = (key, el) => {
  if (el) {
    chartRefs[key] = el
  }
}

const buildChartOption = chart => {
  const series = chart.series.map(item => ({
    name: item.name,
    type: 'line',
    smooth: true,
    showSymbol: false,
    data: getTrendSeries(item.field).map(point => [point.timestamp, point.value]),
    lineStyle: {
      width: 2,
      color: item.color
    },
    itemStyle: {
      color: item.color
    },
    areaStyle: chart.series.length === 1
      ? {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: `${item.color}33` },
            { offset: 1, color: `${item.color}05` }
          ])
        }
      : undefined
  }))

  const hasData = series.some(item => item.data.length > 0)

  return {
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(17, 24, 39, 0.92)',
      borderWidth: 0,
      textStyle: {
        color: '#ffffff'
      },
      formatter: params => {
        if (!Array.isArray(params) || params.length === 0) return ''
        const title = formatDateTime(params[0].value?.[0])
        const lines = params.map(item => `${item.marker}${item.seriesName}：${formatChartValue(item.value?.[1], chart.unit)}`)
        return [title, ...lines].join('<br/>')
      }
    },
    legend: {
      top: 0,
      textStyle: {
        color: '#606266'
      }
    },
    grid: {
      left: 40,
      right: 20,
      top: 40,
      bottom: 24
    },
    xAxis: {
      type: 'time',
      boundaryGap: false,
      axisLine: {
        lineStyle: {
          color: '#dcdfe6'
        }
      },
      axisLabel: {
        color: '#909399',
        formatter: value => formatAxisTime(value)
      },
      splitLine: {
        show: false
      }
    },
    yAxis: {
      type: 'value',
      min: 0,
      max: chart.max,
      axisLine: {
        show: false
      },
      axisLabel: {
        color: '#909399',
        formatter: value => formatChartValue(value, chart.unit)
      },
      splitLine: {
        lineStyle: {
          color: '#eef2f6'
        }
      }
    },
    series,
    graphic: hasData
      ? []
      : [
          {
            type: 'text',
            left: 'center',
            top: 'middle',
            style: {
              text: emptyText.value,
              fill: '#c0c4cc',
              fontSize: 14
            }
          }
        ]
  }
}

const renderCharts = () => {
  chartCards.forEach(chart => {
    const chartDom = chartRefs[chart.key]
    if (!chartDom) return

    if (!chartInstances[chart.key]) {
      chartInstances[chart.key] = echarts.init(chartDom)
    }

    chartInstances[chart.key].setOption(buildChartOption(chart), true)
    chartInstances[chart.key].resize()
  })
}

const disposeCharts = () => {
  Object.values(chartInstances).forEach(instance => {
    if (instance) {
      instance.dispose()
    }
  })
}

const handleResize = () => {
  Object.values(chartInstances).forEach(instance => {
    if (instance) {
      instance.resize()
    }
  })
}

const loadDashboard = async ({ silent = false, buttonLoading = false } = {}) => {
  if (!hostId.value) {
    ElMessage.error('缺少主机 ID')
    return
  }

  const currentSeq = ++requestSeq
  const shouldNotify = !silent || buttonLoading

  if (buttonLoading) {
    refreshing.value = true
  } else if (!silent) {
    loading.value = true
  }

  try {
    const { data: res } = await api.getHostWindowsDashboard(hostId.value, { duration: activeDuration.value })
    if (currentSeq !== requestSeq) return

    if (res.code === 200) {
      dashboard.value = normalizeDashboard(res.data)
      await nextTick()
      renderCharts()
      return
    }

    if (shouldNotify) {
      ElMessage.error(res.message || '获取 Windows 主机详情失败')
    }
  } catch (error) {
    if (currentSeq !== requestSeq) return
    console.error('load windows dashboard failed:', error)
    if (shouldNotify) {
      ElMessage.error(error?.response?.data?.message || error?.message || '获取 Windows 主机详情失败')
    }
  } finally {
    if (currentSeq === requestSeq) {
      if (buttonLoading) {
        refreshing.value = false
      } else if (!silent) {
        loading.value = false
      }
    }
  }
}

const handleRefresh = async () => {
  await loadDashboard({ buttonLoading: true })
}

const stopAutoRefresh = () => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
}

const startAutoRefresh = () => {
  stopAutoRefresh()
  refreshTimer = window.setInterval(() => {
    loadDashboard({ silent: true })
  }, 30000)
}

watch(
  () => hostId.value,
  async newId => {
    if (!newId) return
    await loadDashboard()
    startAutoRefresh()
  },
  { immediate: true }
)

watch(
  () => activeDuration.value,
  async () => {
    if (!hostId.value) return
    await loadDashboard()
    startAutoRefresh()
  }
)

onMounted(() => {
  window.addEventListener('resize', handleResize)
})

onBeforeUnmount(() => {
  stopAutoRefresh()
  window.removeEventListener('resize', handleResize)
  disposeCharts()
})
</script>

<style scoped>
.windows-host-detail-page {
  min-height: 100vh;
  background: #f5f7fa;
}

.page-header {
  position: relative;
  z-index: 10;
  padding: 16px 24px;
  background: #ffffff;
  box-shadow: 0 4px 20px rgba(15, 23, 42, 0.08);
}

.header-content {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  min-height: 52px;
  padding-left: 90px;
}

.back-btn {
  position: absolute;
  left: 0;
  border: none;
  color: #ffffff;
  background: #67c23a;
}

.back-btn:hover {
  color: #ffffff;
  background: #5daf34;
}

.title-block {
  flex: 1;
  min-width: 0;
}

.page-title {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
  color: #303133;
}

.page-subtitle {
  margin-top: 6px;
  font-size: 13px;
  color: #909399;
}

.header-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  color: #909399;
  font-size: 13px;
}

.page-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 16px 24px 0;
}

.page-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 16px 24px 24px;
}

.section-card,
.overview-card,
.list-card {
  overflow: hidden;
  border: 1px solid #ebeef5;
  border-radius: 16px;
}

.section-card :deep(.el-card__header),
.list-card :deep(.el-card__header) {
  padding: 14px 18px;
  background: linear-gradient(180deg, #fafcff 0%, #f5f9ff 100%);
  border-bottom: 1px solid #f0f2f5;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}

.overview-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

.overview-card {
  background: linear-gradient(180deg, #ffffff 0%, #f8fbff 100%);
}

.overview-label {
  font-size: 13px;
  color: #909399;
}

.overview-value {
  margin-top: 10px;
  font-size: 28px;
  line-height: 1.2;
  font-weight: 600;
  color: #1f2d3d;
}

.overview-extra {
  margin-top: 10px;
  min-height: 20px;
  font-size: 13px;
  color: #606266;
}

.chart-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 16px;
}

.chart-panel {
  border: 1px solid #ebeef5;
  border-radius: 14px;
  background: #ffffff;
}

.chart-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px 0;
}

.chart-title {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}

.chart-current {
  font-size: 12px;
  color: #909399;
  text-align: right;
}

.chart-body {
  height: 260px;
  padding: 8px 8px 12px;
}

.list-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 16px;
}

@media (max-width: 900px) {
  .header-content {
    flex-direction: column;
    align-items: flex-start;
    padding-left: 0;
  }

  .back-btn {
    position: static;
  }

  .header-meta {
    flex-wrap: wrap;
  }

  .page-toolbar {
    flex-direction: column;
    align-items: flex-start;
  }
}

@media (max-width: 768px) {
  .page-header,
  .page-toolbar,
  .page-content {
    padding-left: 16px;
    padding-right: 16px;
  }

  .page-content {
    padding-bottom: 16px;
  }

  .chart-grid,
  .list-grid,
  .overview-grid {
    grid-template-columns: 1fr;
  }
}
</style>
