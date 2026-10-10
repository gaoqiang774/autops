<template>
  <div class="k8s-cluster-details">
    <!-- 页面头部 -->
    <div class="page-header">
      <div class="header-left">
        <div class="cluster-info">
          <h1 class="cluster-title">
            <img src="@/assets/image/k8s.svg" alt="k8s" class="k8s-icon" />
            {{ clusterDetails.clusterName || '集群详情' }}
          </h1>
          <div class="cluster-meta">
            <span v-if="!clusterDetails.status" class="loading-pill">
              <span class="loading-dot"></span>
              加载中
            </span>
            <el-tag
              v-else
              :type="getStatusTag(clusterDetails.status)" 
              size="small" 
              effect="plain"
            >
              {{ getStatusText(clusterDetails.status) }}
            </el-tag>
            <span v-if="clusterDetails.version" class="cluster-version">{{ clusterDetails.version }}</span>
            <span v-if="clusterDetails.clusterType" class="cluster-type">{{ getClusterTypeText(clusterDetails.clusterType) }}</span>
          </div>
        </div>
      </div>
      <div class="header-actions">
        <el-button 
          :icon="Refresh" 
          size="small" 
          @click="refreshData"
          :loading="loading"
        >
          刷新
        </el-button>
      </div>
    </div>

    <!-- 主要内容区域 -->
    <div class="details-content" v-loading="loading">
      <!-- 集群概览卡片 -->
      <div class="overview-cards">
        <el-row :gutter="16">
          <el-col :span="6">
            <el-card class="overview-card nodes-card">
              <div class="card-content">
                <div class="card-icon">
                  <el-icon size="24"><Monitor /></el-icon>
                </div>
                <div class="card-info">
                  <div class="card-value">{{ clusterDetails.nodeStats?.totalNodes || 0 }}</div>
                  <div class="card-label">节点总数</div>
                  <div class="card-sub">就绪: {{ clusterDetails.nodeStats?.readyNodes || 0 }}</div>
                </div>
              </div>
            </el-card>
          </el-col>
          <el-col :span="6">
            <el-card class="overview-card workloads-card">
              <div class="card-content">
                <div class="card-icon">
                  <el-icon size="24"><Box /></el-icon>
                </div>
                <div class="card-info">
                  <div class="card-value">{{ getTotalWorkloads() }}</div>
                  <div class="card-label">工作负载</div>
                  <div class="card-sub">Pod: {{ clusterDetails.workloadStats?.totalPods || 0 }}</div>
                </div>
              </div>
            </el-card>
          </el-col>
          <el-col :span="6">
            <el-card class="overview-card cpu-card">
              <div class="card-content">
                <div class="card-icon">
                  <img src="@/assets/image/cpu.svg" alt="CPU" class="svg-icon" />
                </div>
                <div class="card-info">
                  <div class="card-value" :class="getCpuUsageClass()">{{ clusterDetails.monitoring?.cpu?.usagePercent || 0 }}%</div>
                  <div class="card-label">CPU使用率</div>
                  <div class="card-sub">{{ clusterDetails.monitoring?.cpu?.availableCores || 0 }}核可用 / {{ clusterDetails.monitoring?.cpu?.totalCores || 0 }}核总量</div>
                </div>
              </div>
            </el-card>
          </el-col>
          <el-col :span="6">
            <el-card class="overview-card memory-card">
              <div class="card-content">
                <div class="card-icon">
                  <img src="@/assets/image/内存.svg" alt="内存" class="svg-icon" />
                </div>
                <div class="card-info">
                  <div class="card-value" :class="getMemoryUsageClass()">{{ clusterDetails.monitoring?.memory?.usagePercent || 0 }}%</div>
                  <div class="card-label">内存使用率</div>
                  <div class="card-sub">{{ clusterDetails.monitoring?.memory?.availableMi || 0 }}Mi可用 / {{ clusterDetails.monitoring?.memory?.totalMi || 0 }}Mi总量</div>
                </div>
              </div>
            </el-card>
          </el-col>
        </el-row>
      </div>

      <!-- 历史监控趋势 -->
      <el-card class="history-card" shadow="hover">
        <template #header>
          <div class="history-header">
            <div class="card-header">
              <el-icon><Histogram /></el-icon>
              <span>用量趋势</span>
              <el-tag v-if="historyMeta.source" size="small" type="info">{{ historyMeta.source }}</el-tag>
            </div>
            <div class="history-range">{{ historyMeta.timeRange || '暂无时间范围' }}</div>
          </div>
        </template>
        <div v-if="hasHistoryCharts" class="history-chart-grid">
          <div
            v-for="chart in historyCharts"
            :key="chart.key"
            class="history-chart-panel"
          >
            <div class="history-chart-head">
              <div>
                <div class="history-chart-title">{{ chart.title }}</div>
                <div class="history-chart-sub">{{ chart.points.length }} 个采样点</div>
              </div>
              <div class="history-chart-value">
                {{ formatChartValue(chart.latest, chart) }}
              </div>
            </div>
            <div
              class="history-chart"
              :ref="el => setHistoryChartRef(chart.key, el)"
            ></div>
          </div>
        </div>
        <div v-else class="history-empty">
          暂无历史监控数据
        </div>
      </el-card>

      <!-- 详细信息区域 -->
      <div class="details-sections">
        <el-row :gutter="16">
          <!-- 左侧列 -->
          <el-col :span="12">
            <!-- 基础信息 -->
            <el-card class="info-card" shadow="hover">
              <template #header>
                <div class="card-header">
                  <el-icon><InfoFilled /></el-icon>
                  <span>基础信息</span>
                </div>
              </template>
              <div class="info-content">
                <div class="info-row">
                  <span class="label">集群名称:</span>
                  <span class="value">{{ clusterDetails.clusterName }}</span>
                </div>
                <div class="info-row">
                  <span class="label">Kubernetes版本:</span>
                  <span class="value">{{ clusterDetails.version }}</span>
                </div>
                <div class="info-row">
                  <span class="label">集群类型:</span>
                  <span class="value">{{ getClusterTypeText(clusterDetails.clusterType) }}</span>
                </div>
                <div class="info-row">
                  <span class="label">创建时间:</span>
                  <span class="value">{{ formatTime(clusterDetails.createdAt) }}</span>
                </div>
                <div class="info-row">
                  <span class="label">最后同步:</span>
                  <span class="value">{{ formatTime(clusterDetails.lastSyncTime) }}</span>
                </div>
                <div class="info-row">
                  <span class="label">运行时间:</span>
                  <span class="value">{{ clusterDetails.runtime?.uptime || '加载中' }}</span>
                </div>
              </div>
            </el-card>

            <!-- 网络配置 -->
            <el-card class="info-card" shadow="hover">
              <template #header>
                <div class="card-header">
                  <el-icon><Connection /></el-icon>
                  <span>网络配置</span>
                </div>
              </template>
              <div class="info-content">
                <div class="info-row">
                  <span class="label">Service CIDR:</span>
                  <span class="value">{{ clusterDetails.networkConfig?.serviceCIDR || '未配置' }}</span>
                </div>
                <div class="info-row">
                  <span class="label">Pod CIDR:</span>
                  <span class="value">{{ clusterDetails.networkConfig?.podCIDR || '未配置' }}</span>
                </div>
                <div class="info-row">
                  <span class="label">API Server:</span>
                  <span class="value">{{ clusterDetails.networkConfig?.apiServerEndpoint || '未配置' }}</span>
                </div>
                <div class="info-row">
                  <span class="label">网络插件:</span>
                  <span class="value">{{ clusterDetails.networkConfig?.networkPlugin || '加载中' }}</span>
                </div>
                <div class="info-row">
                  <span class="label">服务转发:</span>
                  <span class="value">{{ clusterDetails.networkConfig?.proxyMode || '加载中' }}</span>
                </div>
                <div class="info-row">
                  <span class="label">DNS服务:</span>
                  <span class="value">{{ clusterDetails.networkConfig?.dnsService || '加载中' }}</span>
                </div>
              </div>
            </el-card>

            <!-- 存储信息 -->
            <el-card class="info-card" shadow="hover">
              <template #header>
                <div class="card-header">
                  <el-icon><FolderOpened /></el-icon>
                  <span>存储信息</span>
                </div>
              </template>
              <div class="info-content">
                <div class="info-row">
                  <span class="label">PV总数:</span>
                  <span class="value">{{ clusterDetails.monitoring?.storage?.totalPVs || 0 }}</span>
                </div>
                <div class="info-row">
                  <span class="label">PVC总数:</span>
                  <span class="value">{{ clusterDetails.monitoring?.storage?.totalPVCs || 0 }}</span>
                </div>
                <div class="info-row">
                  <span class="label">存储类:</span>
                  <span class="value">{{ clusterDetails.monitoring?.storage?.storageClasses || 0 }}</span>
                </div>
              </div>
            </el-card>
          </el-col>

          <!-- 右侧列 -->
          <el-col :span="12">
            <!-- 工作负载统计 -->
            <el-card class="info-card" shadow="hover">
              <template #header>
                <div class="card-header">
                  <el-icon><Box /></el-icon>
                  <span>工作负载统计</span>
                </div>
              </template>
              <div class="workload-stats">
                <div class="stat-item">
                  <div class="stat-icon deployments">
                    <el-icon><Grid /></el-icon>
                  </div>
                  <div class="stat-info">
                    <div class="stat-value">{{ clusterDetails.workloadStats?.deployments || 0 }}</div>
                    <div class="stat-label">Deployments</div>
                  </div>
                </div>
                <div class="stat-item">
                  <div class="stat-icon statefulsets">
                    <el-icon><Files /></el-icon>
                  </div>
                  <div class="stat-info">
                    <div class="stat-value">{{ clusterDetails.workloadStats?.statefulSets || 0 }}</div>
                    <div class="stat-label">StatefulSets</div>
                  </div>
                </div>
                <div class="stat-item">
                  <div class="stat-icon daemonsets">
                    <el-icon><Service /></el-icon>
                  </div>
                  <div class="stat-info">
                    <div class="stat-value">{{ clusterDetails.workloadStats?.daemonSets || 0 }}</div>
                    <div class="stat-label">DaemonSets</div>
                  </div>
                </div>
                <div class="stat-item">
                  <div class="stat-icon jobs">
                    <el-icon><Timer /></el-icon>
                  </div>
                  <div class="stat-info">
                    <div class="stat-value">{{ clusterDetails.workloadStats?.jobs || 0 }}</div>
                    <div class="stat-label">Jobs</div>
                  </div>
                </div>
                <div class="stat-item">
                  <div class="stat-icon pods">
                    <el-icon><Box /></el-icon>
                  </div>
                  <div class="stat-info">
                    <div class="stat-value">{{ clusterDetails.workloadStats?.totalPods || 0 }}</div>
                    <div class="stat-label">总Pod数</div>
                  </div>
                </div>
                <div class="stat-item">
                  <div class="stat-icon running-pods">
                    <el-icon><CircleCheck /></el-icon>
                  </div>
                  <div class="stat-info">
                    <div class="stat-value">{{ clusterDetails.workloadStats?.runningPods || 0 }}</div>
                    <div class="stat-label">运行中Pod</div>
                  </div>
                </div>
              </div>
            </el-card>

            <!-- 安装的组件 -->
            <el-card class="info-card" shadow="hover">
              <template #header>
                <div class="card-header">
                  <el-icon><Setting /></el-icon>
                  <span>安装的组件</span>
                </div>
              </template>
              <div class="components-grid">
                <div 
                  v-for="component in clusterDetails.installedComponents || []" 
                  :key="component.name"
                  class="component-item"
                >
                  <div class="component-status" :class="component.status">
                    <el-icon v-if="component.status === 'Running'"><CircleCheck /></el-icon>
                    <el-icon v-else><Warning /></el-icon>
                  </div>
                  <div class="component-info">
                    <div class="component-name">{{ component.name }}</div>
                    <div class="component-version">{{ component.version || '加载中' }}</div>
                  </div>
                </div>
              </div>
            </el-card>

            <!-- 运行时信息 -->
            <el-card class="info-card" shadow="hover">
              <template #header>
                <div class="card-header">
                  <el-icon><Cpu /></el-icon>
                  <span>运行时信息</span>
                </div>
              </template>
              <div class="info-content">
                <div class="info-row">
                  <span class="label">容器运行时:</span>
                  <span class="value">{{ clusterDetails.runtime?.containerRuntimeVersion || '加载中' }}</span>
                </div>
                <div class="info-row">
                  <span class="label">API Server版本:</span>
                  <span class="value">{{ clusterDetails.runtime?.apiServerVersion || '加载中' }}</span>
                </div>
                <div class="info-row">
                  <span class="label">ETCD版本:</span>
                  <span class="value">{{ clusterDetails.runtime?.etcdVersion || '加载中' }}</span>
                </div>
                <div class="info-row">
                  <span class="label">CoreDNS版本:</span>
                  <span class="value">{{ clusterDetails.runtime?.coreDNSVersion || '加载中' }}</span>
                </div>
                <div class="info-row">
                  <span class="label">Kube-proxy版本:</span>
                  <span class="value">{{ clusterDetails.runtime?.kubeProxyVersion || '加载中' }}</span>
                </div>
              </div>
            </el-card>
          </el-col>
        </el-row>

        <!-- 节点信息 -->
        <el-card class="nodes-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <el-icon><Monitor /></el-icon>
              <span>节点信息</span>
              <el-tag size="small" type="info">{{ clusterDetails.nodeList?.length || 0 }}个节点</el-tag>
            </div>
          </template>
          <el-table 
            :data="clusterDetails.nodeList || []" 
            size="small"
            class="nodes-table"
            max-height="300"
          >
            <el-table-column prop="name" label="节点名称" width="200" />
            <el-table-column prop="role" label="角色" width="100">
              <template #default="{ row }">
                <el-tag 
                  :type="row.role === 'master' ? 'danger' : 'primary'" 
                  size="small"
                >
                  {{ row.role === 'master' ? 'Master' : 'Worker' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="status" label="状态" width="100">
              <template #default="{ row }">
                <el-tag 
                  :type="row.status === 'Ready' ? 'success' : 'warning'" 
                  size="small"
                >
                  {{ row.status }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="internalIP" label="内部IP" width="150">
              <template #default="{ row }">
                <el-button 
                  type="text" 
                  size="small" 
                  @click="goToNodeDetail(row.name, row.internalIP)"
                  class="ip-link"
                >
                  {{ row.internalIP }}
                </el-button>
              </template>
            </el-table-column>
            <el-table-column prop="externalIP" label="外部IP" width="150" />
            <el-table-column prop="version" label="K8s版本" width="120" />
            <el-table-column prop="osImage" label="操作系统" width="200" show-overflow-tooltip />
            <el-table-column prop="createdAt" label="创建时间" width="160">
              <template #default="{ row }">
                {{ formatTime(row.createdAt) }}
              </template>
            </el-table-column>
          </el-table>
          <div v-if="!clusterDetails.nodeList || clusterDetails.nodeList.length === 0" class="no-data">
            暂无节点信息
          </div>
        </el-card>

        <!-- 集群事件 -->
        <el-card class="events-card" shadow="hover">
          <template #header>
            <div class="card-header">
              <el-icon><DocumentCopy /></el-icon>
              <span>最近事件</span>
              <el-tag size="small" type="info">最近50条</el-tag>
            </div>
          </template>
          <el-table 
            :data="clusterDetails.events || []" 
            size="small"
            class="events-table"
            max-height="300"
          >
            <el-table-column prop="type" label="类型" width="80">
              <template #default="{ row }">
                <el-tag 
                  :type="row.type === 'Warning' ? 'warning' : 'success'" 
                  size="small"
                >
                  {{ row.type }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="reason" label="原因" width="120" />
            <el-table-column prop="message" label="消息" min-width="200" show-overflow-tooltip />
            <el-table-column prop="source" label="来源" width="150" show-overflow-tooltip />
            <el-table-column prop="count" label="次数" width="60" />
            <el-table-column prop="lastTimestamp" label="最后发生时间" width="160">
              <template #default="{ row }">
                {{ formatTime(row.lastTimestamp) }}
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onBeforeUnmount, computed, nextTick } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts'
import {
  Refresh,
  Monitor,
  Box,
  Cpu,
  Connection,
  InfoFilled,
  FolderOpened,
  Setting,
  Grid,
  Files,
  Service,
  Timer,
  CircleCheck,
  Warning,
  DocumentCopy
} from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'

const router = useRouter()
const route = useRoute()

const loading = ref(false)
const clusterId = ref(route.params.clusterId)
const historyChartRefs = {}
const historyChartInstances = {}

// 集群详情数据
const clusterDetails = reactive({
  clusterName: '',
  version: '',
  status: '',
  clusterType: '',
  createdAt: '',
  lastSyncTime: '',
  nodeStats: {
    totalNodes: 0,
    readyNodes: 0,
    masterNodes: 0,
    workerNodes: 0
  },
  workloadStats: {
    deployments: 0,
    statefulSets: 0,
    daemonSets: 0,
    jobs: 0,
    cronJobs: 0,
    totalPods: 0,
    runningPods: 0
  },
  installedComponents: [],
  networkConfig: {
    serviceCIDR: '',
    podCIDR: '',
    apiServerEndpoint: '',
    networkPlugin: '',
    proxyMode: '',
    dnsService: ''
  },
  monitoring: {
    cpu: {
      totalCores: 0,
      availableCores: 0,
      usagePercent: 0
    },
    memory: {
      totalMi: 0,
      availableMi: 0,
      usagePercent: 0
    },
    network: {
      inboundTraffic: '',
      outboundTraffic: ''
    },
    storage: {
      totalPVs: 0,
      totalPVCs: 0,
      storageClasses: 0
    },
    history: {
      available: false,
      source: '',
      timeRange: '',
      charts: [],
      nodes: [],
      pods: []
    }
  },
  runtime: {
    kubernetesVersion: '',
    containerRuntimeVersion: '',
    apiServerVersion: '',
    etcdVersion: '',
    coreDNSVersion: '',
    kubeProxyVersion: '',
    uptime: ''
  },
  events: [],
  nodeList: []
})

const getValueByPath = (source, path) => {
  if (!source || !path) return undefined
  return path.split('.').reduce((current, key) => current?.[key], source)
}

const pickFirstValue = (source, paths) => {
  for (const path of paths) {
    const value = getValueByPath(source, path)
    if (value !== undefined && value !== null && value !== '' && value !== '未知') {
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
  if (!value || value === '未知') return null
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

const normalizeVersion = (version) => {
  if (!version || version === 'unknown' || version === 'Unknown' || version === '未知') return ''
  return version
}

const fillComponentVersionsFromRuntime = () => {
  const runtimeVersionMap = {
    'kube-apiserver': clusterDetails.runtime.apiServerVersion,
    'etcd': clusterDetails.runtime.etcdVersion,
    'coredns': clusterDetails.runtime.coreDNSVersion,
    'kube-proxy': clusterDetails.runtime.kubeProxyVersion
  }

  clusterDetails.installedComponents = (clusterDetails.installedComponents || []).map(component => ({
    ...component,
    version: normalizeVersion(component.version) || normalizeVersion(runtimeVersionMap[component.name]) || '加载中'
  }))
}

const normalizeMetricPoint = point => ({
  timestamp: point?.timestamp || point?.time || '',
  value: Number(point?.value || 0)
})

const historyCharts = computed(() => {
  const charts = Array.isArray(clusterDetails.monitoring?.history?.charts)
    ? clusterDetails.monitoring.history.charts
    : []

  return charts.map(chart => {
    const points = Array.isArray(chart?.points)
      ? chart.points.map(normalizeMetricPoint).filter(point => point.timestamp)
      : []

    return {
      key: chart?.key || chart?.title || `chart-${points.length}`,
      title: chart?.title || chart?.key || '指标',
      unit: chart?.unit || '',
      precision: Number.isInteger(chart?.precision) ? chart.precision : 2,
      points,
      latest: points.length ? points[points.length - 1] : null
    }
  }).filter(chart => chart.points.length > 0)
})

const hasHistoryCharts = computed(() => historyCharts.value.length > 0)

const historyMeta = computed(() => ({
  source: clusterDetails.monitoring?.history?.source || '',
  timeRange: clusterDetails.monitoring?.history?.timeRange || ''
}))

const setHistoryChartRef = (key, el) => {
  if (!key) return
  if (el) {
    historyChartRefs[key] = el
  } else {
    delete historyChartRefs[key]
  }
}

const formatChartValue = (point, chart) => {
  if (!point) return '-'
  const precision = Number.isInteger(chart?.precision) ? chart.precision : 2
  return `${Number(point.value || 0).toFixed(precision)}${chart?.unit || ''}`
}

const renderHistoryCharts = async () => {
  await nextTick()

  const activeKeys = new Set(historyCharts.value.map(chart => chart.key))
  Object.keys(historyChartInstances).forEach(key => {
    if (!activeKeys.has(key)) {
      historyChartInstances[key]?.dispose()
      delete historyChartInstances[key]
    }
  })

  historyCharts.value.forEach(chart => {
    const el = historyChartRefs[chart.key]
    if (!el) return

    const instance = historyChartInstances[chart.key] || echarts.init(el)
    historyChartInstances[chart.key] = instance
    instance.setOption({
      grid: { left: 36, right: 12, top: 18, bottom: 28 },
      tooltip: { trigger: 'axis' },
      xAxis: {
        type: 'category',
        boundaryGap: false,
        data: chart.points.map(point => formatTime(point.timestamp))
      },
      yAxis: {
        type: 'value',
        axisLabel: { formatter: value => `${value}${chart.unit || ''}` }
      },
      series: [{
        name: chart.title,
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: chart.points.map(point => Number(point.value || 0)),
        lineStyle: { width: 2 },
        areaStyle: { opacity: 0.08 }
      }]
    })
  })
}

const handleHistoryChartResize = () => {
  Object.values(historyChartInstances).forEach(chart => chart?.resize())
}

// 获取集群详情
const fetchClusterDetails = async () => {
  try {
    loading.value = true
    console.log('获取集群详情，ID:', clusterId.value)
    
    const response = await k8sApi.getClusterDetail(clusterId.value)
    const responseData = response.data || response
    
    console.log('集群详情API响应:', responseData)
    
    if (responseData.code === 200 || responseData.success) {
      const data = responseData.data || {}
      const cluster = data.cluster || {}
      
      console.log('解析集群数据:', cluster)
      
      // 填充基础信息
      clusterDetails.clusterName = cluster.name || ''
      clusterDetails.version = cluster.version || ''
      clusterDetails.status = cluster.status || ''
      clusterDetails.clusterType = cluster.clusterType || ''
      clusterDetails.createdAt = cluster.createdAt || ''
      clusterDetails.lastSyncTime = cluster.lastSyncAt || ''
      
      // 填充节点统计 - 使用summary数据
      if (data.summary) {
        clusterDetails.nodeStats.totalNodes = data.summary.totalNodes || 0
        clusterDetails.nodeStats.readyNodes = data.summary.readyNodes || 0
        clusterDetails.nodeStats.masterNodes = data.summary.masterNodes || 0
        clusterDetails.nodeStats.workerNodes = data.summary.workerNodes || 0
      }
      
      // 填充工作负载统计 - 使用workloads数据
      if (data.workloads) {
        clusterDetails.workloadStats.deployments = data.workloads.totalDeployments || 0
        clusterDetails.workloadStats.statefulSets = data.workloads.totalStatefulSets || 0
        clusterDetails.workloadStats.daemonSets = data.workloads.totalDaemonSets || 0
        clusterDetails.workloadStats.jobs = data.workloads.totalJobs || 0
        clusterDetails.workloadStats.cronJobs = data.workloads.totalCronJobs || 0
        clusterDetails.workloadStats.totalPods = data.workloads.totalPods || 0
        clusterDetails.workloadStats.runningPods = data.workloads.runningPods || 0
      }
      
      clusterDetails.installedComponents = data.components ? data.components.map(component => ({
        name: component.name || component.componentName || '',
        version: normalizeVersion(component.version ||
          component.componentVersion ||
          component.currentVersion ||
          component.serverVersion),
        status: component.status || component.state || 'Unknown'
      })) : []
      
      // 填充网络配置
      if (data.network) {
        clusterDetails.networkConfig.serviceCIDR = data.network.serviceCIDR || ''
        clusterDetails.networkConfig.podCIDR = data.network.podCIDR || ''
        clusterDetails.networkConfig.apiServerEndpoint = data.network.apiServerEndpoint || ''
        clusterDetails.networkConfig.networkPlugin = data.network.networkPlugin || ''
        clusterDetails.networkConfig.proxyMode = data.network.proxyMode || ''
        clusterDetails.networkConfig.dnsService = data.network.dnsService || ''
      }
      
      // 填充监控信息
      if (data.monitoring) {
        // CPU信息
        if (data.monitoring.cpu) {
          const totalCores = parseFloat(String(data.monitoring.cpu.total || '').replace(' cores', '')) || 0
          const availableCores = parseFloat(String(data.monitoring.cpu.available || '').replace(' cores', '')) || 0
          
          clusterDetails.monitoring.cpu.totalCores = totalCores
          clusterDetails.monitoring.cpu.availableCores = availableCores
          
          // 计算CPU使用率：如果API提供了usageRate使用API值，否则计算 (total - available) / total * 100
          let cpuUsage = data.monitoring.cpu.usageRate || 0
          if (cpuUsage === 0 && totalCores > 0 && availableCores > 0) {
            cpuUsage = Math.round(((totalCores - availableCores) / totalCores) * 100)
          }
          clusterDetails.monitoring.cpu.usagePercent = cpuUsage
        }
        
        // 内存信息
        if (data.monitoring.memory) {
          const totalMi = parseFloat(String(data.monitoring.memory.total || '').replace(' Mi', '')) || 0
          const availableMi = parseFloat(String(data.monitoring.memory.available || '').replace(' Mi', '')) || 0
          
          clusterDetails.monitoring.memory.totalMi = totalMi
          clusterDetails.monitoring.memory.availableMi = availableMi
          
          // 计算内存使用率：如果API提供了usageRate使用API值，否则计算 (total - available) / total * 100
          let memoryUsage = data.monitoring.memory.usageRate || 0
          if (memoryUsage === 0 && totalMi > 0 && availableMi > 0) {
            memoryUsage = Math.round(((totalMi - availableMi) / totalMi) * 100)
          }
          clusterDetails.monitoring.memory.usagePercent = memoryUsage
        }
        
        // 网络信息
        if (data.monitoring.network) {
          clusterDetails.monitoring.network.inboundTraffic = data.monitoring.network.inboundTraffic || ''
          clusterDetails.monitoring.network.outboundTraffic = data.monitoring.network.outboundTraffic || ''
        }
        
        // 存储信息
        if (data.monitoring.storage) {
          clusterDetails.monitoring.storage.totalPVs = data.monitoring.storage.totalPVs || 0
          clusterDetails.monitoring.storage.totalPVCs = data.monitoring.storage.totalPVCs || 0
          clusterDetails.monitoring.storage.storageClasses = data.monitoring.storage.storageClasses?.length || 0
        }

        const history = data.monitoring.history || {}
        clusterDetails.monitoring.history.available = Boolean(history.available)
        clusterDetails.monitoring.history.source = history.source || ''
        clusterDetails.monitoring.history.timeRange = history.timeRange || ''
        clusterDetails.monitoring.history.charts = Array.isArray(history.charts) ? history.charts : []
        clusterDetails.monitoring.history.nodes = Array.isArray(history.nodes) ? history.nodes : []
        clusterDetails.monitoring.history.pods = Array.isArray(history.pods) ? history.pods : []
      }
      
      // 填充运行时信息
      if (data.runtime) {
        clusterDetails.runtime.kubernetesVersion = data.runtime.kubernetesVersion || data.runtime.k8sVersion || ''
        clusterDetails.runtime.containerRuntimeVersion = data.runtime.containerRuntime || data.runtime.containerRuntimeVersion || ''
        clusterDetails.runtime.apiServerVersion = data.runtime.apiServerVersion || data.runtime.apiServer || ''
        // 尝试多种可能的etcd版本字段名，处理空字符串
        const etcdVersion = data.runtime.etcdVersion || 
                          data.runtime.etcd?.version || 
                          data.runtime.etcd?.serverVersion ||
                          data.runtime.etcdServerVersion ||
                          ''
        clusterDetails.runtime.etcdVersion = etcdVersion || '加载中'
        clusterDetails.runtime.coreDNSVersion = data.runtime.coreDNSVersion || data.runtime.coreDns?.version || ''
        clusterDetails.runtime.kubeProxyVersion = data.runtime.kubeProxyVersion || data.runtime.kubeProxy?.version || ''
        clusterDetails.runtime.uptime = data.runtime.upTime || data.runtime.uptime || ''
      }
      fillComponentVersionsFromRuntime()
      
      // 填充事件信息
      clusterDetails.events = data.events || []
      
      // 填充节点列表 - 映射正确的字段名
      if (data.nodes) {
        clusterDetails.nodeList = data.nodes.map(node => ({
          name: node.name || '',
          role: node.role || 'unknown',
          status: node.status || 'Unknown',
          internalIP: node.internalIP || '',
          externalIP: node.externalIP || '无',  // 处理空的外部IP
          version: node.version || '',
          osImage: node.os || node.osImage || '加载中',  // 使用 os 字段
          createdAt: pickFirstValue(node, [
            'createdAt',
            'creationTime',
            'creationTimestamp',
            'metadata.creationTimestamp',
            'metadata.creation_time',
            'CreationTimestamp'
          ]),
          // 保持其他可能需要的原始字段
          capacity: node.capacity,
          allocatable: node.allocatable,
          conditions: node.conditions
        }))
      } else {
        clusterDetails.nodeList = []
      }

      renderHistoryCharts().catch(error => {
        console.warn('渲染历史监控图表失败:', error)
      })
      
    } else {
      ElMessage.error(responseData.message || '获取集群详情失败')
    }
  } catch (error) {
    console.error('获取集群详情失败:', error)
    
    if (error.response?.status === 404) {
      ElMessage.error('集群不存在或已被删除')
      router.push('/k8s/list')
    } else if (error.response?.status === 401) {
      ElMessage.error('权限不足，无法访问集群详情')
    } else if (error.code === 'ERR_NETWORK') {
      ElMessage.error('网络连接失败，请检查网络状态')
    } else {
      ElMessage.error('获取集群详情失败，请重试')
    }
  } finally {
    loading.value = false
  }
}

// 刷新数据
const refreshData = () => {
  fetchClusterDetails()
}

// 获取状态标签类型
const getStatusTag = (status) => {
  const tagMap = {
    1: 'info',      // 创建中
    2: 'success',   // 运行中
    3: 'warning',   // 离线
    'creating': 'info',
    'online': 'success', 
    'offline': 'warning'
  }
  return tagMap[status] || 'info'
}

// 获取状态文本
const getStatusText = (status) => {
  const textMap = {
    1: '创建中',
    2: '运行中', 
    3: '离线',
    'creating': '创建中',
    'online': '运行中',
    'offline': '离线'
  }
  return textMap[status] || '加载中'
}

// 获取集群类型文本
const getClusterTypeText = (type) => {
  const typeMap = {
    1: '自建集群',
    2: '导入集群'
  }
  return typeMap[type] || '加载中'
}

// 获取工作负载总数
const getTotalWorkloads = () => {
  const stats = clusterDetails.workloadStats
  return (stats.deployments || 0) + 
         (stats.statefulSets || 0) + 
         (stats.daemonSets || 0) + 
         (stats.jobs || 0) + 
         (stats.cronJobs || 0)
}

// 格式化时间
const formatTime = (time) => {
  const date = parseTimestamp(time)
  return date ? date.toLocaleString('zh-CN') : '加载中'
}

// 获取CPU使用率样式类
const getCpuUsageClass = () => {
  const usage = clusterDetails.monitoring?.cpu?.usagePercent || 0
  if (usage >= 80) return 'usage-high'
  if (usage >= 60) return 'usage-medium'
  return 'usage-normal'
}

// 获取内存使用率样式类
const getMemoryUsageClass = () => {
  const usage = clusterDetails.monitoring?.memory?.usagePercent || 0
  if (usage >= 80) return 'usage-high'
  if (usage >= 60) return 'usage-medium'
  return 'usage-normal'
}

// 跳转到节点详情
const goToNodeDetail = (nodeName, nodeIP) => {
  console.log('跳转到节点详情:', nodeName, nodeIP)
  router.push(`/k8s/cluster/${clusterId.value}/node/${nodeName}`)
}

onMounted(() => {
  fetchClusterDetails()
  window.addEventListener('resize', handleHistoryChartResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleHistoryChartResize)
  Object.values(historyChartInstances).forEach(chart => {
    chart?.dispose()
  })
})
</script>

<style scoped>
.k8s-cluster-details {
  --aiops-bg-page: #f4f7fb;
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
  padding: 6px;
  min-height: 100vh;
  box-sizing: border-box;
  position: relative;
  background:
    radial-gradient(circle at 16% 10%, rgba(76, 139, 245, 0.06), transparent 26%),
    linear-gradient(180deg, #f7f9fc 0%, var(--aiops-bg-page) 100%);
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.k8s-cluster-details::before {
  content: '';
  position: absolute;
  inset: 6px;
  border: 1px solid rgba(207, 217, 230, 0.72);
  border-radius: 28px;
  background: linear-gradient(
    135deg,
    rgba(255, 253, 248, 0.58),
    rgba(247, 250, 255, 0.56)
  );
  pointer-events: none;
}

.page-header {
  position: relative;
  z-index: 1;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 18px;
  margin-bottom: 12px;
  padding: 18px 20px;
  border: 1px solid var(--aiops-border-mid);
  border-radius: 26px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  box-shadow: 0 18px 50px rgba(17, 24, 39, 0.06);
  backdrop-filter: blur(18px);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 16px;
  min-width: 0;
}

.cluster-info {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.cluster-title {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  color: var(--aiops-text-primary);
  line-height: 1.2;
}

.k8s-icon {
  width: 32px;
  height: 32px;
}

.cluster-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.loading-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-height: 20px;
  padding: 2px 8px;
  border: 1px solid rgba(153, 181, 230, 0.55);
  border-radius: 999px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.92), rgba(237, 242, 253, 0.82));
  color: var(--aiops-brand);
  font-size: 11px;
  font-weight: 700;
  box-shadow: 0 8px 18px rgba(76, 139, 245, 0.08);
}

.loading-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--aiops-brand);
  box-shadow: 0 0 0 3px rgba(76, 139, 245, 0.12);
  animation: loading-dot-pulse 1.2s ease-in-out infinite;
}

@keyframes loading-dot-pulse {
  0%,
  100% {
    opacity: 0.55;
    transform: scale(0.88);
  }

  50% {
    opacity: 1;
    transform: scale(1);
  }
}

.cluster-version {
  color: var(--aiops-text-secondary);
  font-size: 14px;
  padding: 5px 10px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.74);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
}

.cluster-type {
  color: var(--aiops-text-muted);
  font-size: 12px;
  font-weight: 600;
}

.header-actions :deep(.el-button) {
  height: 32px;
  border: 1px solid rgba(214, 223, 235, 0.96);
  border-radius: 13px;
  background: rgba(255, 255, 255, 0.9);
  color: var(--aiops-text-secondary);
  font-weight: 600;
  transition: transform 0.2s ease, border-color 0.2s ease, color 0.2s ease, box-shadow 0.2s ease;
}

.header-actions :deep(.el-button:hover:not(.is-disabled)) {
  transform: translateY(-1px);
  border-color: rgba(153, 181, 230, 0.95);
  color: var(--aiops-brand);
  box-shadow: 0 10px 18px rgba(17, 24, 39, 0.07);
}

.details-content {
  position: relative;
  z-index: 1;
}

.overview-cards {
  margin-bottom: 12px;
  position: relative;
  z-index: 1;
}

.overview-card {
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 20px;
  background: linear-gradient(180deg, rgba(255, 253, 249, 0.88), rgba(247, 250, 255, 0.84));
  box-shadow: none;
  backdrop-filter: blur(10px);
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.overview-card:hover,
.info-card:hover {
  transform: translateY(-1px);
  border-color: rgba(196, 210, 231, 0.96);
  box-shadow: 0 18px 42px rgba(17, 24, 39, 0.07);
}

.overview-card :deep(.el-card__body) {
  padding: 16px;
}

.card-content {
  display: flex;
  align-items: center;
  gap: 14px;
}

.card-icon {
  width: 44px;
  height: 44px;
  border-radius: 15px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--aiops-brand);
  background: var(--aiops-brand-soft);
  border: 1px solid rgba(203, 218, 244, 0.9);
}

.nodes-card .card-icon,
.workloads-card .card-icon,
.cpu-card .card-icon,
.memory-card .card-icon {
  background: rgba(76, 139, 245, 0.1);
}

.svg-icon {
  width: 24px;
  height: 24px;
  opacity: 0.82;
}

.card-info {
  flex: 1;
  min-width: 0;
}

.card-value {
  font-size: 28px;
  font-weight: 700;
  color: var(--aiops-text-primary);
  line-height: 1;
}

.card-label {
  font-size: 14px;
  color: var(--aiops-text-secondary);
  margin: 4px 0;
}

.card-sub {
  font-size: 12px;
  color: var(--aiops-text-muted);
  line-height: 1.5;
}

.usage-normal {
  color: var(--aiops-success) !important;
}

.usage-medium {
  color: var(--aiops-warning) !important;
}

.usage-high {
  color: var(--aiops-danger) !important;
}

.history-card {
  margin-bottom: 12px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.88);
  box-shadow: none;
  overflow: hidden;
  backdrop-filter: blur(10px);
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.history-card:hover {
  transform: translateY(-1px);
  border-color: rgba(196, 210, 231, 0.96);
  box-shadow: 0 18px 42px rgba(17, 24, 39, 0.07);
}

.history-card :deep(.el-card__header) {
  padding: 14px 16px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.84);
  background: rgba(248, 250, 255, 0.58);
}

.history-card :deep(.el-card__body) {
  padding: 16px;
}

.history-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.history-range {
  color: var(--aiops-text-muted);
  font-size: 12px;
  font-weight: 600;
}

.history-chart-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.history-chart-panel {
  min-width: 0;
  border: 1px solid rgba(223, 231, 242, 0.76);
  border-radius: 16px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.92), rgba(247, 250, 255, 0.72));
  padding: 12px;
}

.history-chart-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  min-height: 42px;
}

.history-chart-title {
  color: var(--aiops-text-primary);
  font-size: 14px;
  font-weight: 700;
  line-height: 1.3;
}

.history-chart-sub {
  margin-top: 3px;
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.history-chart-value {
  color: var(--aiops-text-primary);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 16px;
  font-weight: 700;
  white-space: nowrap;
}

.history-chart {
  width: 100%;
  height: 180px;
  margin-top: 8px;
}

.history-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 180px;
  border: 1px dashed rgba(203, 213, 225, 0.9);
  border-radius: 16px;
  color: var(--aiops-text-muted);
  font-size: 14px;
  background: rgba(248, 250, 255, 0.58);
}

.details-sections {
  display: flex;
  flex-direction: column;
  gap: 12px;
  position: relative;
  z-index: 1;
}

.info-card {
  margin-bottom: 12px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.86);
  box-shadow: none;
  backdrop-filter: blur(10px);
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.info-card :deep(.el-card__header),
.nodes-card :deep(.el-card__header),
.events-card :deep(.el-card__header) {
  padding: 14px 16px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.84);
  background: rgba(248, 250, 255, 0.58);
}

.info-card :deep(.el-card__body),
.nodes-card :deep(.el-card__body),
.events-card :deep(.el-card__body) {
  padding: 16px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  font-weight: 600;
  color: var(--aiops-text-primary);
}

.card-header .el-icon {
  color: var(--aiops-brand);
}

.info-content {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 10px 0;
  border-bottom: 1px solid rgba(223, 231, 242, 0.72);
}

.info-row:last-child {
  border-bottom: none;
}

.info-row .label {
  color: var(--aiops-text-secondary);
  font-weight: 600;
  min-width: 120px;
}

.info-row .value {
  color: var(--aiops-text-primary);
  text-align: right;
  word-break: break-all;
  font-size: 13px;
}

.info-row .value:empty::before {
  content: '加载中';
  color: var(--aiops-text-muted);
}

.workload-stats {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 10px;
}

.stat-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border: 1px solid rgba(223, 231, 242, 0.72);
  border-radius: 16px;
  background: rgba(247, 250, 255, 0.7);
  transition: background-color 0.2s ease, border-color 0.2s ease;
}

.stat-item:hover {
  border-color: rgba(196, 210, 231, 0.96);
  background: rgba(255, 255, 255, 0.88);
}

.stat-icon {
  width: 34px;
  height: 34px;
  border-radius: 13px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--aiops-brand);
  font-size: 16px;
  background: var(--aiops-brand-soft);
  border: 1px solid rgba(203, 218, 244, 0.88);
}

.stat-icon.deployments,
.stat-icon.statefulsets,
.stat-icon.daemonsets,
.stat-icon.jobs,
.stat-icon.pods,
.stat-icon.running-pods {
  background: var(--aiops-brand-soft);
}

.stat-value {
  font-size: 20px;
  font-weight: 700;
  color: var(--aiops-text-primary);
  line-height: 1;
}

.stat-label {
  font-size: 12px;
  color: var(--aiops-text-secondary);
  margin-top: 2px;
}

.components-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

.component-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border: 1px solid rgba(223, 231, 242, 0.76);
  border-radius: 16px;
  background: rgba(247, 250, 255, 0.7);
}

.component-item .component-status.Running {
  color: var(--aiops-success);
}

.component-item .component-status:not(.Running) {
  color: var(--aiops-danger);
}

.component-status {
  font-size: 16px;
}

.component-name {
  font-weight: 500;
  color: var(--aiops-text-primary);
  font-size: 14px;
}

.component-version {
  font-size: 12px;
  color: var(--aiops-text-muted);
}

.details-sections > .nodes-card {
  margin-bottom: 12px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.86);
  box-shadow: none;
  overflow: hidden;
}

.nodes-table,
.events-table {
  border: 1px solid rgba(223, 231, 242, 0.88);
  border-radius: 16px;
  overflow: hidden;
}

.nodes-table :deep(.el-table__inner-wrapper::before),
.events-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.nodes-table :deep(.el-table__header th),
.events-table :deep(.el-table__header th) {
  background: rgba(248, 250, 255, 0.98) !important;
  border-bottom: 1px solid rgba(223, 231, 242, 0.95);
  color: var(--aiops-text-muted);
  font-size: 12px;
  font-weight: 700;
}

.nodes-table :deep(.el-table__cell),
.events-table :deep(.el-table__cell) {
  border-bottom-color: rgba(15, 23, 42, 0.045);
}

.nodes-table :deep(.el-table__body tr:hover > td.el-table__cell),
.events-table :deep(.el-table__body tr:hover > td.el-table__cell) {
  background: rgba(76, 139, 245, 0.045) !important;
}

.ip-link {
  color: var(--aiops-brand) !important;
  font-weight: 500;
  transition: color 0.2s ease;
}

.ip-link:hover {
  color: #3f75df !important;
  text-decoration: underline;
}

.no-data {
  text-align: center;
  color: var(--aiops-text-muted);
  padding: 40px;
  font-size: 14px;
}

.events-card {
  margin-top: 0;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.86);
  box-shadow: none;
  overflow: hidden;
}

.k8s-cluster-details :deep(.el-tag) {
  border-radius: 999px;
  font-weight: 600;
}

.k8s-cluster-details :deep(.el-loading-mask) {
  background: rgba(244, 247, 251, 0.58);
  backdrop-filter: blur(4px);
}

.k8s-cluster-details :deep(.el-loading-spinner .circular) {
  width: 28px;
  height: 28px;
}

.k8s-cluster-details :deep(.el-loading-spinner .el-loading-text) {
  margin-top: 4px;
  font-size: 12px;
}

@media (max-width: 1200px) {
  .overview-cards .el-col {
    margin-bottom: 12px;
  }
  
  .workload-stats {
    grid-template-columns: 1fr;
  }
  
  .components-grid {
    grid-template-columns: 1fr;
  }

  .history-chart-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .k8s-cluster-details {
    padding: 4px;
  }

  .k8s-cluster-details::before {
    inset: 4px;
    border-radius: 22px;
  }
  
  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 14px;
    border-radius: 22px;
    padding: 16px;
  }

  .header-left {
    flex-direction: column;
    align-items: flex-start;
    width: 100%;
  }

  .header-actions,
  .header-actions :deep(.el-button) {
    width: 100%;
  }
  
  .cluster-title {
    font-size: 20px;
  }
  
  .cluster-meta {
    flex-wrap: wrap;
  }

  .overview-card,
  .info-card,
  .history-card,
  .details-sections > .nodes-card,
  .events-card {
    border-radius: 18px;
  }

  .history-chart-grid {
    grid-template-columns: 1fr;
  }

  .history-chart {
    height: 170px;
  }
  
  .details-sections .el-row {
    flex-direction: column;
  }
  
  .info-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }
  
  .info-row .value {
    text-align: left;
  }
}
</style>
