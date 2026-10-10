<template>
  <div class="network-monitor-page">
    <section class="monitor-shell">
      <header class="monitor-header">
        <div>
          <span class="header-kicker">SNMP Monitor</span>
          <h2>{{ selectedDevice.deviceName || selectedDevice.ipAddress || '交换机监控' }}</h2>
          <p>{{ selectedDevice.ipAddress || '-' }} · {{ selectedDevice.deviceVendor || '-' }} · {{ selectedDevice.deviceModel || '未知型号' }}</p>
        </div>
        <el-button class="back-button" @click="goBack">
          <el-icon><ArrowLeft /></el-icon>
          返回
        </el-button>
      </header>

      <section class="toolbar-card">
        <div class="toolbar-actions">
          <div class="filter-group">
            <span class="filter-label">数据源</span>
            <el-select
              v-model="selectedDatasourceId"
              placeholder="选择 Prometheus 数据源"
              filterable
              clearable
              style="width: 240px"
              :loading="datasourceLoading"
              @change="handleDatasourceChange"
            >
              <el-option
                v-for="item in datasourceList"
                :key="getDatasourceId(item)"
                :label="getDatasourceLabel(item)"
                :value="getDatasourceId(item)"
              />
            </el-select>
          </div>
          <div class="filter-group">
            <span class="filter-label">时间范围</span>
            <el-select v-model="timeRange" style="width: 140px" @change="refreshData">
              <el-option label="Last 30 min" value="30m" />
              <el-option label="Last 1 hour" value="1h" />
              <el-option label="Last 6 hours" value="6h" />
              <el-option label="Last 24 hours" value="24h" />
            </el-select>
          </div>
          <div class="filter-group">
            <span class="filter-label">刷新</span>
            <el-select v-model="refreshInterval" style="width: 96px">
              <el-option label="关闭" :value="0" />
              <el-option label="30s" :value="30000" />
              <el-option label="1m" :value="60000" />
              <el-option label="5m" :value="300000" />
            </el-select>
          </div>
          <el-button type="primary" :loading="loading" @click="refreshData">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </section>

      <section class="device-bar">
        <div class="device-icon">
          <img :src="switchIcon" alt="交换机" />
        </div>
        <div class="device-lines">
          <div class="device-main">
            <strong>{{ selectedDevice.deviceName || '未命名设备' }}</strong>
            <el-tag :type="selectedDevice.status === 'online' ? 'success' : selectedDevice.status === 'offline' ? 'danger' : 'info'" effect="plain" size="small">
              {{ selectedDevice.status || 'unknown' }}
            </el-tag>
          </div>
          <div class="device-sub">
            <span>IP：{{ selectedDevice.ipAddress || '-' }}</span>
            <span>实例：{{ promInstance || '-' }}</span>
            <span>位置：{{ selectedDevice.location || '-' }}</span>
            <span>负责人：{{ selectedDevice.owner || '-' }}</span>
          </div>
        </div>
      </section>

      <div v-if="!selectedDatasourceId" class="empty-state">
        <el-empty description="请选择 Prometheus 数据源后查看监控数据" />
      </div>

      <template v-else>
        <section class="kpi-grid" v-loading="loading">
          <article class="kpi-card">
            <span>运行时间</span>
            <strong>{{ uptimeDays }} <em>days</em></strong>
            <small>设备持续运行</small>
          </article>
          <article class="kpi-card">
            <span>Max Out</span>
            <strong>{{ maxOut }} <em>Mb/s</em></strong>
            <small>当前出向峰值带宽</small>
          </article>
          <article class="kpi-card">
            <span>Max In</span>
            <strong>{{ maxIn }} <em>Mb/s</em></strong>
            <small>当前入向峰值带宽</small>
          </article>
          <article class="kpi-card">
            <span>Total Out</span>
            <strong>{{ totalOut }} <em>GB</em></strong>
            <small>时间窗口累计出向流量</small>
          </article>
          <article class="kpi-card">
            <span>Total In</span>
            <strong>{{ totalIn }} <em>GB</em></strong>
            <small>时间窗口累计入向流量</small>
          </article>
        </section>

        <section class="switch-visual-card">
          <div class="section-head">
            <h3>物理端口状态表</h3>
            <span v-if="portList.length">共 {{ portList.length }} 个端口</span>
          </div>

          <div v-loading="loading" class="switch-stage">
            <div class="switch-device">
              <div class="switch-top-face"></div>
              <div class="switch-side-face"></div>
              <div class="switch-front">
                <div class="switch-front-head">
                  <span>{{ selectedDevice.deviceName || 'Switch' }}</span>
                  <em>{{ selectedDevice.deviceModel || 'SNMP SWITCH' }}</em>
                </div>
                <div class="switch-led-strip" aria-hidden="true">
                  <span v-for="index in 10" :key="`led-${index}`" :class="{ active: index % 3 !== 0 }"></span>
                </div>
                <div class="port-grid" :style="{ '--port-columns': portGridColumns }">
                  <el-tooltip
                    v-for="port in visualPortList"
                    :key="port.portName"
                    placement="top"
                    :show-after="80"
                  >
                    <template #content>
                      <div>{{ port.portName }}</div>
                      <div>状态：{{ port.statusText }}</div>
                      <div>速率：{{ formatSpeed(port.portSpeed) }}</div>
                      <div>描述：{{ port.description || '-' }}</div>
                    </template>
                    <button
                      type="button"
                      class="switch-port"
                      :class="port.statusClass"
                    >
                      <span class="port-light"></span>
                      <span class="port-label">{{ port.shortName }}</span>
                      <span class="port-speed">{{ formatSpeed(port.portSpeed) }}</span>
                    </button>
                  </el-tooltip>
                </div>
                <div class="switch-foot">
                  <span></span>
                  <span></span>
                </div>
              </div>
            </div>
            <div class="switch-legend">
              <span><i class="legend-dot up"></i>UP</span>
              <span><i class="legend-dot down"></i>DOWN</span>
              <span><i class="legend-dot unknown"></i>UNKNOWN</span>
            </div>
            <el-empty v-if="!portList.length && !loading" description="暂无端口数据" />
          </div>
        </section>

        <section class="table-card traffic-detail-card">
          <div class="section-head">
            <h3>TOP 流量端口详情</h3>
            <span>共 {{ trafficPortList.length }} 条 · 每页 10 条</span>
          </div>
          <el-table v-loading="loading" :data="pagedTrafficPortList" stripe size="small" class="network-table traffic-detail-table">
            <el-table-column label="端口名称" min-width="150">
              <template #default="{ row }"><span class="mono strong">{{ row.portName || '-' }}</span></template>
            </el-table-column>
            <el-table-column label="描述信息" min-width="200" show-overflow-tooltip>
              <template #default="{ row }"><span class="traffic-port-desc">{{ row.description || '-' }}</span></template>
            </el-table-column>
            <el-table-column label="当前 Out" width="128" align="right" sortable :sort-method="sortBy('outCurrent')">
              <template #default="{ row }"><span class="mono red">{{ formatBitrate(row.outCurrent) }}</span></template>
            </el-table-column>
            <el-table-column label="当前 In" width="128" align="right" sortable :sort-method="sortBy('inCurrent')">
              <template #default="{ row }"><span class="mono blue">{{ formatBitrate(row.inCurrent) }}</span></template>
            </el-table-column>
            <el-table-column label="Total Out" width="128" align="right" sortable :sort-method="sortBy('outTotal')">
              <template #default="{ row }"><span class="mono red">{{ formatBytes(row.outTotal) }}</span></template>
            </el-table-column>
            <el-table-column label="Total In" width="128" align="right" sortable :sort-method="sortBy('inTotal')">
              <template #default="{ row }"><span class="mono blue">{{ formatBytes(row.inTotal) }}</span></template>
            </el-table-column>
            <el-table-column label="端口速率" width="118" align="right">
              <template #default="{ row }"><span class="mono blue">{{ formatSpeed(row.portSpeed) }}</span></template>
            </el-table-column>
          </el-table>
          <div v-if="trafficPortList.length > trafficPageSize" class="pagination-wrap">
            <el-pagination small :total="trafficPortList.length" :page-size="trafficPageSize" v-model:current-page="trafficPage" layout="prev, pager, next" />
          </div>
        </section>

        <section class="chart-section">
          <div class="chart-row">
            <article class="chart-card">
              <div class="chart-head">端口出向流量热力图 (Out)</div>
              <div ref="outHeatmapRef" class="chart-container"></div>
            </article>
            <article class="chart-card">
              <div class="chart-head">端口入向流量热力图 (In)</div>
              <div ref="inHeatmapRef" class="chart-container"></div>
            </article>
          </div>
          <article class="chart-card">
            <div class="chart-head">双向流量趋势图</div>
            <div ref="trendChartRef" class="chart-container chart-trend"></div>
          </article>
        </section>
      </template>
    </section>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, Refresh } from '@element-plus/icons-vue'
import * as echarts from 'echarts'
import alertmanagerApi from '@/api/alertmanager'
import cmdbApi from '@/api/cmdb'
import switchIcon from '@/assets/image/交换机.svg'

const route = useRoute()
const router = useRouter()

const outHeatmapRef = ref(null)
const inHeatmapRef = ref(null)
const trendChartRef = ref(null)

const deviceId = computed(() => route.params.id)
const selectedDevice = ref({})
const datasourceList = ref([])
const selectedDatasourceId = ref(null)
const promInstance = ref('')
const timeRange = ref('1h')
const refreshInterval = ref(0)
const datasourceLoading = ref(false)
const loading = ref(false)
const monitorSnapshot = ref(null)
const uptimeValue = ref(null)
const kpiMetrics = ref({ maxOut: null, maxIn: null, totalOut: null, totalIn: null })
const promPorts = ref([])
const chartData = ref({ outHeatmap: { ports: [], times: [], data: [] }, inHeatmap: { ports: [], times: [], data: [] }, trend: { out: [], in: [], times: [] } })
const trafficPage = ref(1)
const trafficPageSize = 10

let refreshTimer = null
let outHeatmapChart = null
let inHeatmapChart = null
let trendChart = null
let resizeObserver = null

const getDatasourceId = row => row?.id ?? row?.ID ?? row?.datasource_id ?? row?.datasourceId
const getDatasourceLabel = row => row?.name || row?.source_name || row?.sourceName || row?.instance_id || `数据源 ${getDatasourceId(row)}`
const isSuccessResponse = res => [0, 200].includes(Number(res?.code))
const responsePayload = response => response?.data || response || {}
const pickList = data => {
  if (Array.isArray(data)) return data
  if (Array.isArray(data?.list)) return data.list
  if (Array.isArray(data?.items)) return data.items
  return []
}

const uptimeDays = computed(() => {
  if (uptimeValue.value !== null) return uptimeValue.value.toFixed(1)
  const value = monitorSnapshot.value?.uptimeDays
  return Number.isFinite(Number(value)) ? Number(value).toFixed(1) : '--'
})

const maxOut = computed(() => formatMetricMbps(kpiMetrics.value.maxOut, monitorSnapshot.value?.portOutTop5))
const maxIn = computed(() => formatMetricMbps(kpiMetrics.value.maxIn, monitorSnapshot.value?.portInTop5))
const totalOut = computed(() => formatMetricGb(kpiMetrics.value.totalOut, monitorSnapshot.value?.portOutTop5))
const totalIn = computed(() => formatMetricGb(kpiMetrics.value.totalIn, monitorSnapshot.value?.portInTop5))

const portList = computed(() => {
  if (promPorts.value.length) return promPorts.value.map(item => ({
    portName: item.portName,
    description: item.description || '',
    status: item.status,
    portSpeed: item.portSpeed
  }))

  const seen = new Set()
  return [...(monitorSnapshot.value?.portOutTop5 || []), ...(monitorSnapshot.value?.portInTop5 || [])].reduce((list, item) => {
    if (!item?.portName || seen.has(item.portName)) return list
    seen.add(item.portName)
    list.push({
      portName: item.portName,
      description: item.description || '',
      status: item.status || 'up',
      portSpeed: item.portSpeed || 0
    })
    return list
  }, [])
})

const trafficPortList = computed(() => {
  if (promPorts.value.length) {
    return [...promPorts.value]
      .sort((a, b) => ((b.outCurrent || 0) + (b.inCurrent || 0)) - ((a.outCurrent || 0) + (a.inCurrent || 0)))
      .slice(0, 50)
      .map(item => ({
        portName: item.portName,
        description: item.description || '',
        outCurrent: (item.outCurrent || 0) / 1000000,
        inCurrent: (item.inCurrent || 0) / 1000000,
        outTotal: item.outTotal || 0,
        inTotal: item.inTotal || 0,
        portSpeed: item.portSpeed || 0
      }))
  }

  const rows = new Map()
  ;(monitorSnapshot.value?.portOutTop5 || []).forEach(item => rows.set(item.portName, {
    portName: item.portName,
    description: item.description || '',
    outCurrent: item.trafficRateMbps || 0,
    outTotal: item.trafficBytes || 0,
    inCurrent: 0,
    inTotal: 0,
    portSpeed: item.portSpeed || 0
  }))
  ;(monitorSnapshot.value?.portInTop5 || []).forEach(item => {
    const row = rows.get(item.portName) || { portName: item.portName, description: item.description || '', outCurrent: 0, outTotal: 0, portSpeed: item.portSpeed || 0 }
    row.inCurrent = item.trafficRateMbps || 0
    row.inTotal = item.trafficBytes || 0
    rows.set(item.portName, row)
  })
  return Array.from(rows.values()).sort((a, b) => (b.outCurrent + b.inCurrent) - (a.outCurrent + a.inCurrent))
})

const pagedTrafficPortList = computed(() => trafficPortList.value.slice((trafficPage.value - 1) * trafficPageSize, trafficPage.value * trafficPageSize))
const visualPortList = computed(() => [...portList.value]
  .sort((left, right) => portSortValue(left.portName) - portSortValue(right.portName))
  .map(port => {
    const status = String(port.status || '').toLowerCase()
    const isUp = status === 'up' || status === '1' || status === 'online'
    const isDown = status === 'down' || status === '2' || status === 'offline'
    return {
      ...port,
      shortName: shortenPortName(port.portName),
      statusText: isUp ? 'UP' : isDown ? 'DOWN' : (port.status || 'UNKNOWN'),
      statusClass: isUp ? 'is-up' : isDown ? 'is-down' : 'is-unknown'
    }
  }))
const portGridColumns = computed(() => {
  const count = visualPortList.value.length
  if (count <= 24) return 8
  if (count <= 72) return 12
  return 20
})

function formatMetricMbps(value, fallbackPorts) {
  if (value !== null && value !== undefined) return (Number(value) / 1000000).toFixed(2)
  if (!fallbackPorts?.length) return '--'
  return Math.max(...fallbackPorts.map(item => Number(item.trafficRateMbps || 0))).toFixed(2)
}

function formatMetricGb(value, fallbackPorts) {
  if (value !== null && value !== undefined) return (Number(value) / (1024 ** 3)).toFixed(2)
  if (!fallbackPorts?.length) return '--'
  return (fallbackPorts.reduce((sum, item) => sum + Number(item.trafficBytes || 0), 0) / (1024 ** 3)).toFixed(2)
}

function getTimeBounds() {
  const now = Date.now()
  const ranges = { '30m': 30 * 60 * 1000, '1h': 60 * 60 * 1000, '6h': 6 * 60 * 60 * 1000, '24h': 24 * 60 * 60 * 1000 }
  return { start: now - (ranges[timeRange.value] || ranges['1h']), end: now }
}

function metricPortName(metric = {}) {
  return metric.ifDescr || metric.ifName || metric.interface || metric.port || Object.values(metric)[0] || 'unknown'
}

function parseMatrix(data) {
  return pickList(data).map(stream => ({
    portName: metricPortName(stream.metric),
    values: (stream.values || []).map(([timestamp, value]) => ({
      time: Math.round(Number(timestamp) * 1000),
      value: Number(value) || 0
    }))
  }))
}

function parseInstant(response) {
  const payload = responsePayload(response)
  if (!isSuccessResponse(payload)) return null
  const vector = pickList(payload.data)
  const value = vector[0]?.value
  return value ? Number(value[1]) || 0 : null
}

function parseVector(response) {
  const payload = responsePayload(response)
  if (!isSuccessResponse(payload)) return []
  return pickList(payload.data).map(item => ({ metric: item.metric || {}, value: item.value ? Number(item.value[1]) || 0 : 0 }))
}

function sortBy(key) {
  return (left, right) => Number(left[key] || 0) - Number(right[key] || 0)
}

function portSortValue(name = '') {
  const matches = String(name).match(/\d+/g)
  if (!matches?.length) return Number.MAX_SAFE_INTEGER
  return matches.reduce((value, part) => value * 1000 + Number(part), 0)
}

function shortenPortName(name = '') {
  return String(name)
    .replace(/^Ethernet/i, 'Eth')
    .replace(/^GigabitEthernet/i, 'GE')
    .replace(/^Ten-GigabitEthernet/i, 'TE')
    .replace(/^TenGigabitEthernet/i, 'TE')
}

function formatBitrate(mbps) {
  if (mbps === undefined || mbps === null) return '-'
  const value = Number(mbps)
  if (value >= 1000) return `${(value / 1000).toFixed(2)} Gb/s`
  return `${value.toFixed(2)} Mb/s`
}

function formatBytes(bytes) {
  const value = Number(bytes || 0)
  if (!value) return '-'
  if (value >= 1024 ** 3) return `${(value / 1024 ** 3).toFixed(2)} GB`
  if (value >= 1024 ** 2) return `${(value / 1024 ** 2).toFixed(2)} MB`
  return `${(value / 1024).toFixed(2)} KB`
}

function formatSpeed(speed) {
  const value = Number(speed || 0)
  if (!value) return '-'
  if (value >= 10000) return `${(value / 1000).toFixed(2)} Gb/s`
  if (value >= 1000) return `${(value / 1000).toFixed(1)} Gb/s`
  return `${value.toFixed(0)} Mb/s`
}

async function loadDeviceInfo() {
  const { data: res } = await cmdbApi.getSnmpDeviceById(deviceId.value)
  if (res?.code !== 200) throw new Error(res?.message || '获取交换机信息失败')
  selectedDevice.value = res.data || {}
}

async function loadDatasourceList() {
  datasourceLoading.value = true
  try {
    const res = responsePayload(await alertmanagerApi.getDatasources({ resource_type: 'prometheus', page: 1, pageSize: 100 }))
    if (!isSuccessResponse(res)) throw new Error(res?.message || res?.msg || '获取数据源列表失败')
    datasourceList.value = pickList(res.data)
    if (!selectedDatasourceId.value && datasourceList.value.length) {
      const prod = datasourceList.value.find(item => getDatasourceLabel(item).includes('生产'))
      selectedDatasourceId.value = getDatasourceId(prod || datasourceList.value[0])
    }
  } catch (error) {
    console.error('获取数据源列表失败:', error)
    ElMessage.error(error.message || '获取数据源列表失败')
  } finally {
    datasourceLoading.value = false
  }
}

async function resolvePromInstance() {
  if (!selectedDatasourceId.value || !selectedDevice.value?.ipAddress) return
  try {
    const res = responsePayload(await alertmanagerApi.getMetricTargets({ datasource_id: selectedDatasourceId.value }))
    if (!isSuccessResponse(res)) return
    const ip = selectedDevice.value.ipAddress
    const targets = pickList(res.data)
    const target = targets.find(item => item.ip === ip || String(item.instance || '').startsWith(`${ip}:`) || item.instance === ip)
    promInstance.value = target?.instance || ip
  } catch (error) {
    console.warn('匹配 Prometheus target 失败:', error)
    promInstance.value = selectedDevice.value?.ipAddress || ''
  }
}

async function loadMonitorSnapshot() {
  try {
    const { data: res } = await cmdbApi.getSnmpDeviceMonitorData(deviceId.value)
    if (res?.code === 200) {
      monitorSnapshot.value = res.data || null
    }
  } catch (error) {
    console.warn('获取设备监控快照失败:', error)
  }
}

async function loadMetricData() {
  const instance = promInstance.value || selectedDevice.value?.ipAddress
  if (!instance || !selectedDatasourceId.value) return

  loading.value = true
  const { start, end } = getTimeBounds()
  const baseParams = { start, end, step: '5m', datasource_id: selectedDatasourceId.value }
  const rangeSec = Math.max(60, Math.round((end - start) / 1000))
  const instanceMatch = `{instance="${instance}"}`
  const ipOnly = instance.includes(':') ? instance.split(':')[0] : instance
  const ipMatch = `{instance="${ipOnly}"}`
  const portMatch = `{ifName=~"^G.*|^T.*|^Fo.*|^Ether.*|^XG.*|^40.*|^reth.*|^ge.*",instance="${instance}"}`

  try {
    await loadMonitorSnapshot()
    const [outRes, inRes, uptimeRes, ...kpiRes] = await Promise.all([
      alertmanagerApi.queryMetricRange({ ...baseParams, promql: `irate(ifHCOutOctets${instanceMatch}[5m]) * 8` }),
      alertmanagerApi.queryMetricRange({ ...baseParams, promql: `irate(ifHCInOctets${instanceMatch}[5m]) * 8` }),
      alertmanagerApi.queryMetricRange({ ...baseParams, promql: `sysUpTime${instanceMatch}` }),
      alertmanagerApi.queryMetric({ promql: `max(irate(ifHCOutOctets${instanceMatch}[5m])*8)`, datasource_id: selectedDatasourceId.value }),
      alertmanagerApi.queryMetric({ promql: `max(irate(ifHCInOctets${instanceMatch}[5m])*8)`, datasource_id: selectedDatasourceId.value }),
      alertmanagerApi.queryMetric({ promql: `max(delta(ifHCOutOctets${instanceMatch}[${rangeSec}s]))`, datasource_id: selectedDatasourceId.value }),
      alertmanagerApi.queryMetric({ promql: `max(delta(ifHCInOctets${instanceMatch}[${rangeSec}s]))`, datasource_id: selectedDatasourceId.value }),
      alertmanagerApi.queryMetric({ promql: `ifOperStatus${portMatch}`, datasource_id: selectedDatasourceId.value }),
      alertmanagerApi.queryMetric({ promql: `ifHighSpeed${portMatch}`, datasource_id: selectedDatasourceId.value }),
      alertmanagerApi.queryMetric({ promql: `irate(ifHCOutOctets${instanceMatch}[5m])*8`, datasource_id: selectedDatasourceId.value }),
      alertmanagerApi.queryMetric({ promql: `irate(ifHCInOctets${instanceMatch}[5m])*8`, datasource_id: selectedDatasourceId.value }),
      alertmanagerApi.queryMetric({ promql: `delta(ifHCOutOctets${instanceMatch}[${rangeSec}s])`, datasource_id: selectedDatasourceId.value }),
      alertmanagerApi.queryMetric({ promql: `delta(ifHCInOctets${instanceMatch}[${rangeSec}s])`, datasource_id: selectedDatasourceId.value })
    ])

    kpiMetrics.value = {
      maxOut: parseInstant(kpiRes[0]),
      maxIn: parseInstant(kpiRes[1]),
      totalOut: parseInstant(kpiRes[2]),
      totalIn: parseInstant(kpiRes[3])
    }

    buildPromPorts(kpiRes.slice(4))
    setUptime(uptimeRes)

    let outSeries = isSuccessResponse(responsePayload(outRes)) ? parseMatrix(responsePayload(outRes).data) : []
    let inSeries = isSuccessResponse(responsePayload(inRes)) ? parseMatrix(responsePayload(inRes).data) : []

    if (!outSeries.length && !inSeries.length && ipOnly !== instance) {
      const [altOutRes, altInRes] = await Promise.all([
        alertmanagerApi.queryMetricRange({ ...baseParams, promql: `irate(ifHCOutOctets${ipMatch}[5m]) * 8` }),
        alertmanagerApi.queryMetricRange({ ...baseParams, promql: `irate(ifHCInOctets${ipMatch}[5m]) * 8` })
      ])
      outSeries = isSuccessResponse(responsePayload(altOutRes)) ? parseMatrix(responsePayload(altOutRes).data) : []
      inSeries = isSuccessResponse(responsePayload(altInRes)) ? parseMatrix(responsePayload(altInRes).data) : []
    }

    chartData.value = outSeries.length || inSeries.length
      ? buildChartData(outSeries, inSeries)
      : buildChartDataFromSnapshot()
  } catch (error) {
    console.error('加载交换机监控指标失败:', error)
    chartData.value = buildChartDataFromSnapshot()
    ElMessage.error(error.message || '加载交换机监控指标失败')
  } finally {
    loading.value = false
    nextTick(renderCharts)
  }
}

function buildPromPorts(responses) {
  const [statusRes, speedRes, outCurRes, inCurRes, outTotalRes, inTotalRes] = responses
  const portStatus = parseVector(statusRes)
  const portSpeed = parseVector(speedRes)
  const portOutCur = parseVector(outCurRes)
  const portInCur = parseVector(inCurRes)
  const portOutTot = parseVector(outTotalRes)
  const portInTot = parseVector(inTotalRes)
  const valueByName = (list, name) => list.find(item => item.metric?.ifName === name || item.metric?.ifDescr === name)?.value ?? null
  const names = new Set([...portStatus, ...portSpeed, ...portOutCur, ...portInCur].map(item => item.metric?.ifName || item.metric?.ifDescr).filter(Boolean))
  promPorts.value = Array.from(names).map(name => {
    const status = portStatus.find(item => item.metric?.ifName === name || item.metric?.ifDescr === name)
    return {
      portName: name,
      description: status?.metric?.ifAlias || '',
      status: status ? (status.value === 1 ? 'up' : 'down') : 'unknown',
      portSpeed: valueByName(portSpeed, name) || 0,
      outCurrent: valueByName(portOutCur, name) || 0,
      inCurrent: valueByName(portInCur, name) || 0,
      outTotal: valueByName(portOutTot, name) || 0,
      inTotal: valueByName(portInTot, name) || 0
    }
  })
}

function setUptime(response) {
  const payload = responsePayload(response)
  if (!isSuccessResponse(payload)) return
  const matrix = pickList(payload.data)
  const last = matrix[0]?.values?.[matrix[0].values.length - 1]
  uptimeValue.value = last ? (Number(last[1]) || 0) / (100 * 86400) : null
}

function buildChartData(outSeries, inSeries) {
  const allTimes = new Set()
  outSeries.forEach(series => series.values.forEach(point => allTimes.add(point.time)))
  inSeries.forEach(series => series.values.forEach(point => allTimes.add(point.time)))
  const sortedTimes = Array.from(allTimes).sort((a, b) => a - b)
  const times = sortedTimes.map(timestamp => {
    const date = new Date(timestamp)
    return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
  })

  const toHeatmap = series => {
    const ports = series.map(item => item.portName)
    const data = []
    series.forEach((item, portIndex) => {
      sortedTimes.forEach((time, timeIndex) => {
        const point = item.values.find(value => value.time === time)
        data.push([timeIndex, portIndex, point ? point.value / 1000000 : 0])
      })
    })
    return { ports, times, data }
  }

  const toTrend = (series, invert) => series.map(item => ({
    name: item.portName,
    type: 'line',
    smooth: true,
    symbol: 'none',
    lineStyle: { width: 1.5 },
    data: sortedTimes.map(time => {
      const point = item.values.find(value => value.time === time)
      const value = point ? point.value / 1000000 : 0
      return invert ? -value : value
    })
  }))

  return {
    outHeatmap: toHeatmap(outSeries),
    inHeatmap: toHeatmap(inSeries),
    trend: {
      out: toTrend(outSeries, false),
      in: toTrend(inSeries, true),
      times
    }
  }
}

function buildChartDataFromSnapshot() {
  const now = Date.now()
  const points = 12
  const interval = 300000
  const makeSeries = ports => (ports || []).map(port => ({
    portName: port.portName,
    values: Array.from({ length: points }, (_, index) => ({
      time: now - (points - 1 - index) * interval,
      value: Number(port.trafficRateMbps || 0) * 1000000 * (0.8 + Math.random() * 0.4)
    }))
  }))
  return buildChartData(makeSeries(monitorSnapshot.value?.portOutTop5), makeSeries(monitorSnapshot.value?.portInTop5))
}

function renderHeatmap(chart, heatmap, colors) {
  if (!chart) return
  if (!heatmap.ports?.length) {
    chart.clear()
    return
  }
  chart.setOption({
    backgroundColor: 'transparent',
    tooltip: { position: 'top', formatter: params => `${heatmap.ports[params.value[1]]}<br/>${heatmap.times[params.value[0]]}<br/>${Number(params.value[2]).toFixed(2)} Mb/s` },
    grid: { left: '12%', right: '18%', top: '5%', bottom: '8%' },
    xAxis: { type: 'category', data: heatmap.times, axisLabel: { color: '#8d97a8', fontSize: 10 } },
    yAxis: { type: 'category', data: heatmap.ports, axisLabel: { color: '#8d97a8', fontSize: 10 } },
    visualMap: { min: 0, max: Math.max(...heatmap.data.map(item => item[2]), 1), calculable: true, orient: 'vertical', right: 0, top: 'center', inRange: { color: colors } },
    series: [{ type: 'heatmap', data: heatmap.data, label: { show: false } }]
  }, true)
}

function renderTrendChart() {
  if (!trendChart) return
  const trend = chartData.value.trend
  if (!trend.times?.length) {
    trendChart.clear()
    return
  }
  const palette = ['#f56c6c', '#e6a23c', '#409eff', '#9b59b6', '#1abc9c', '#e74c3c', '#3498db', '#2ecc71']
  const colorSeries = (series, start) => series.map((item, index) => ({
    ...item,
    lineStyle: { width: 1.5, color: palette[(start + index) % palette.length] },
    itemStyle: { color: palette[(start + index) % palette.length] }
  }))
  trendChart.setOption({
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', backgroundColor: 'rgba(0,0,0,0.85)', borderColor: '#4c8bf5', textStyle: { color: '#fff', fontSize: 11 } },
    legend: { data: trend.out.map(item => item.name).concat(trend.in.map(item => item.name)), textStyle: { color: '#8d97a8', fontSize: 10 }, top: 0, type: 'scroll' },
    grid: { left: '5%', right: '4%', top: '18%', bottom: '8%' },
    xAxis: { type: 'category', data: trend.times, axisLine: { lineStyle: { color: '#3a4a5c' } }, axisLabel: { color: '#8d97a8' } },
    yAxis: { type: 'value', axisLabel: { color: '#8d97a8', formatter: value => `${Math.abs(value)} Mb` }, splitLine: { lineStyle: { color: '#2d3a4b', type: 'dashed' } } },
    series: [
      { name: '', type: 'line', data: trend.times.map(() => 0), lineStyle: { color: '#5f6b7c', width: 1, type: 'dashed' }, symbol: 'none', silent: true },
      ...colorSeries(trend.out, 0),
      ...colorSeries(trend.in, trend.out.length)
    ]
  }, true)
}

function renderCharts() {
  renderHeatmap(outHeatmapChart, chartData.value.outHeatmap, ['#0a3d1a', '#1a7a3a', '#3cb85c', '#5ce68a', '#8ef5b0'])
  renderHeatmap(inHeatmapChart, chartData.value.inHeatmap, ['#0a1d3a', '#1a4a7a', '#3c7ab8', '#5caae6', '#8ecaf5'])
  renderTrendChart()
}

function initCharts() {
  if (outHeatmapRef.value) outHeatmapChart = echarts.init(outHeatmapRef.value)
  if (inHeatmapRef.value) inHeatmapChart = echarts.init(inHeatmapRef.value)
  if (trendChartRef.value) trendChart = echarts.init(trendChartRef.value)
  renderCharts()
}

async function handleDatasourceChange() {
  promInstance.value = ''
  promPorts.value = []
  await resolvePromInstance()
  await loadMetricData()
}

async function refreshData() {
  await resolvePromInstance()
  await loadMetricData()
}

function setupAutoRefresh() {
  if (refreshTimer) clearInterval(refreshTimer)
  if (refreshInterval.value > 0) {
    refreshTimer = setInterval(refreshData, refreshInterval.value)
  }
}

function goBack() {
  router.push('/cmdb/switch')
}

watch(trafficPortList, list => {
  const totalPages = Math.max(1, Math.ceil(list.length / trafficPageSize))
  if (trafficPage.value > totalPages) trafficPage.value = totalPages
})

watch(refreshInterval, setupAutoRefresh)

onMounted(async () => {
  if (!deviceId.value) {
    ElMessage.error('设备 ID 不存在')
    goBack()
    return
  }
  try {
    await loadDeviceInfo()
    await loadDatasourceList()
    await resolvePromInstance()
    await loadMetricData()
    await nextTick()
    initCharts()
    const chartEls = [outHeatmapRef.value, inHeatmapRef.value, trendChartRef.value].filter(Boolean)
    if (chartEls.length) {
      resizeObserver = new ResizeObserver(() => {
        outHeatmapChart?.resize()
        inHeatmapChart?.resize()
        trendChart?.resize()
      })
      chartEls.forEach(el => resizeObserver.observe(el))
    }
  } catch (error) {
    console.error('初始化交换机监控失败:', error)
    ElMessage.error(error.message || '初始化交换机监控失败')
  }
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
  resizeObserver?.disconnect()
  outHeatmapChart?.dispose()
  inHeatmapChart?.dispose()
  trendChart?.dispose()
})
</script>

<style scoped>
.network-monitor-page {
  height: 100%;
  overflow-y: auto;
  padding: 10px;
  background: transparent;
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.monitor-shell {
  min-height: 100%;
  padding: 16px;
  border: 1px solid rgba(223, 231, 242, 0.94);
  border-radius: 22px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.96), rgba(247, 250, 255, 0.92));
  box-shadow: 0 18px 48px rgba(17, 24, 39, 0.08);
}

.monitor-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin: -16px -16px 14px;
  padding: 16px 20px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.92);
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
}

.header-kicker {
  color: #4c8bf5;
  font-size: 12px;
  font-weight: 700;
}

.monitor-header h2 {
  margin: 4px 0;
  color: #172133;
  font-size: 22px;
}

.monitor-header p {
  margin: 0;
  color: #5f6b7c;
  font-size: 13px;
}

.toolbar-card,
.device-bar,
.table-card {
  margin-bottom: 16px;
  border: 1px solid rgba(223, 231, 242, 0.94);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.72);
}

.switch-visual-card {
  margin-bottom: 16px;
  padding: 18px 20px 20px;
  border: 1px solid rgba(190, 205, 226, 0.96);
  border-radius: 20px;
  background: linear-gradient(135deg, rgba(248, 251, 255, 0.96), rgba(232, 240, 252, 0.9));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.84), 0 14px 34px rgba(17, 24, 39, 0.06);
  overflow: hidden;
}

.toolbar-card {
  padding: 14px 16px;
}

.toolbar-actions,
.filter-group,
.device-bar,
.device-main,
.device-sub,
.section-head {
  display: flex;
  align-items: center;
}

.toolbar-actions {
  flex-wrap: wrap;
  gap: 12px;
}

.filter-group {
  gap: 7px;
}

.filter-label {
  color: #5f6b7c;
  font-size: 12px;
  font-weight: 600;
}

.back-button,
:deep(.el-button) {
  border-radius: 14px;
}

.device-bar {
  gap: 12px;
  padding: 12px 16px;
}

.device-icon {
  display: grid;
  place-items: center;
  width: 42px;
  height: 42px;
  border-radius: 14px;
  background: rgba(76, 139, 245, 0.1);
}

.device-icon img {
  width: 24px;
  height: 24px;
}

.device-lines {
  min-width: 0;
  flex: 1;
}

.device-main {
  gap: 8px;
}

.device-sub {
  gap: 14px;
  flex-wrap: wrap;
  margin-top: 5px;
  color: #6b7280;
  font-size: 12px;
}

.empty-state {
  padding: 60px 0;
}

.switch-stage {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  min-height: 230px;
  padding: 34px 20px 10px;
  border-radius: 18px;
  background:
    linear-gradient(180deg, rgba(232, 241, 253, 0.92), rgba(241, 246, 253, 0.72)),
    repeating-linear-gradient(90deg, transparent 0 26px, rgba(148, 163, 184, 0.07) 26px 27px);
  overflow-x: auto;
  overflow-y: visible;
}

.switch-device {
  position: relative;
  width: min(1080px, calc(100% - 24px));
  min-width: 760px;
  padding-top: 34px;
  perspective: 1100px;
  transform: rotateX(2deg) rotateY(-7deg);
  transform-origin: 50% 55%;
}

.switch-top-face,
.switch-side-face {
  position: absolute;
  pointer-events: none;
}

.switch-top-face {
  top: 2px;
  left: 66px;
  right: 12px;
  height: 44px;
  border-radius: 18px 18px 7px 7px;
  background:
    linear-gradient(110deg, rgba(148, 163, 184, 0.54), rgba(55, 68, 88, 0.98) 42%, rgba(28, 39, 57, 0.98)),
    repeating-linear-gradient(90deg, transparent 0 44px, rgba(255, 255, 255, 0.08) 44px 45px);
  clip-path: polygon(8% 0, 100% 0, calc(100% - 26px) 100%, 0 100%);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.24),
    0 12px 22px rgba(15, 23, 42, 0.16);
}

.switch-side-face {
  top: 43px;
  right: -8px;
  width: 58px;
  bottom: 20px;
  border-radius: 0 20px 20px 0;
  background:
    repeating-linear-gradient(145deg, rgba(255, 255, 255, 0.16) 0 2px, transparent 2px 9px),
    linear-gradient(90deg, #30405a, #121b2b);
  transform: skewY(-22deg);
  transform-origin: left top;
  box-shadow: inset 10px 0 14px rgba(255, 255, 255, 0.06), 16px 16px 28px rgba(15, 23, 42, 0.14);
}

.switch-front {
  position: relative;
  z-index: 1;
  margin-right: 50px;
  padding: 14px 16px 16px;
  border: 1px solid rgba(148, 163, 184, 0.48);
  border-radius: 18px;
  background:
    linear-gradient(180deg, rgba(77, 91, 112, 0.94), rgba(23, 32, 48, 0.98)),
    repeating-linear-gradient(0deg, rgba(255, 255, 255, 0.04) 0 1px, transparent 1px 6px);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.2),
    inset 0 -12px 22px rgba(0, 0, 0, 0.28),
    0 22px 36px rgba(15, 23, 42, 0.22);
  overflow: hidden;
}

.switch-stage::-webkit-scrollbar {
  height: 8px;
}

.switch-stage::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.36);
}

.switch-stage::-webkit-scrollbar-track {
  background: transparent;
}

.switch-front::before,
.switch-front::after {
  content: "";
  position: absolute;
  top: 18px;
  bottom: 18px;
  width: 6px;
  border-radius: 999px;
  background: linear-gradient(180deg, #7d8da3, #253246);
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.16);
}

.switch-front::before {
  left: 8px;
}

.switch-front::after {
  right: 8px;
}

.switch-front-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin: 0 12px 10px;
  padding: 0 4px 9px;
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
  color: #e5edf8;
  font-size: 13px;
  font-weight: 700;
}

.switch-front-head em {
  color: #9fb1c8;
  font-size: 11px;
  font-style: normal;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.switch-led-strip {
  display: flex;
  gap: 7px;
  margin: 0 12px 11px;
}

.switch-led-strip span {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: rgba(100, 116, 139, 0.64);
  box-shadow: inset 0 0 0 1px rgba(15, 23, 42, 0.42);
}

.switch-led-strip span.active {
  background: #56e58b;
  box-shadow: 0 0 9px rgba(86, 229, 139, 0.82);
}

.port-grid {
  display: grid;
  grid-template-columns: repeat(var(--port-columns), minmax(34px, 1fr));
  gap: 8px 7px;
  padding: 12px;
  border: 1px solid rgba(15, 23, 42, 0.54);
  border-radius: 14px;
  background:
    radial-gradient(circle at 50% 0, rgba(96, 165, 250, 0.13), transparent 55%),
    rgba(10, 18, 30, 0.78);
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.05);
}

.switch-port {
  position: relative;
  display: grid;
  grid-template-rows: 8px 1fr auto;
  align-items: center;
  justify-items: center;
  min-width: 0;
  height: 43px;
  padding: 5px 3px 4px;
  border: 1px solid rgba(148, 163, 184, 0.38);
  border-radius: 6px;
  background: linear-gradient(180deg, #1f2b3f, #0c1320);
  color: #cbd5e1;
  cursor: default;
  box-shadow:
    inset 0 2px 5px rgba(255, 255, 255, 0.06),
    inset 0 -4px 8px rgba(0, 0, 0, 0.34);
  transition: transform 0.16s ease, border-color 0.16s ease, box-shadow 0.16s ease;
}

.switch-port:hover {
  transform: translateY(-2px);
  border-color: rgba(96, 165, 250, 0.72);
  box-shadow: 0 10px 18px rgba(15, 23, 42, 0.24), inset 0 2px 5px rgba(255, 255, 255, 0.08);
}

.switch-port.is-up {
  border-color: rgba(74, 222, 128, 0.6);
  background: linear-gradient(180deg, #153322, #0b1d14);
}

.switch-port.is-down {
  border-color: rgba(248, 113, 113, 0.62);
  background: linear-gradient(180deg, #3a1a20, #1d0d12);
}

.switch-port.is-unknown {
  border-color: rgba(148, 163, 184, 0.42);
  background: linear-gradient(180deg, #263244, #131b29);
}

.port-light {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #94a3b8;
  box-shadow: 0 0 0 1px rgba(15, 23, 42, 0.45);
}

.is-up .port-light {
  background: #4ade80;
  box-shadow: 0 0 9px rgba(74, 222, 128, 0.95);
}

.is-down .port-light {
  background: #fb7185;
  box-shadow: 0 0 8px rgba(251, 113, 133, 0.82);
}

.port-label {
  max-width: 100%;
  overflow: hidden;
  color: #e5edf8;
  font-family: Monaco, "Courier New", monospace;
  font-size: 9px;
  font-weight: 700;
  line-height: 1.15;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.port-speed {
  max-width: 100%;
  overflow: hidden;
  color: #93c5fd;
  font-family: Monaco, "Courier New", monospace;
  font-size: 8px;
  line-height: 1;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.is-down .port-speed {
  color: #fca5a5;
}

.switch-foot {
  display: flex;
  justify-content: space-between;
  margin: 12px 22px 0;
}

.switch-foot span {
  width: 72px;
  height: 5px;
  border-radius: 999px;
  background: linear-gradient(90deg, transparent, rgba(203, 213, 225, 0.86), transparent);
}

.switch-legend {
  display: flex;
  gap: 14px;
  color: #5f6b7c;
  font-size: 12px;
}

.switch-legend span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.legend-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.legend-dot.up {
  background: #4ade80;
  box-shadow: 0 0 8px rgba(74, 222, 128, 0.76);
}

.legend-dot.down {
  background: #fb7185;
  box-shadow: 0 0 8px rgba(251, 113, 133, 0.68);
}

.legend-dot.unknown {
  background: #94a3b8;
}

.kpi-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 16px;
}

.kpi-card {
  padding: 16px 18px;
  border: 1px solid rgba(76, 139, 245, 0.12);
  border-radius: 16px;
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
  box-shadow: 0 8px 22px rgba(17, 24, 39, 0.12);
}

.kpi-card span,
.kpi-card small {
  display: block;
  color: #8d97a8;
  font-size: 12px;
}

.kpi-card strong {
  display: block;
  margin: 6px 0 3px;
  color: #ff4d4f;
  font-family: Monaco, "Courier New", monospace;
  font-size: 25px;
}

.kpi-card em {
  color: #8d97a8;
  font-size: 13px;
  font-style: normal;
}

.table-card {
  padding: 18px 20px 16px;
}

.traffic-detail-card {
  padding: 14px 16px 12px;
}

.section-head {
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 12px;
}

.section-head h3 {
  margin: 0;
  color: #172133;
  font-size: 15px;
  font-weight: 700;
}

.section-head span {
  color: #8d97a8;
  font-size: 12px;
}

.network-table {
  width: 100%;
  border-radius: 14px;
}

.traffic-detail-table {
  font-size: 12px;
}

.network-table :deep(.el-table__header th) {
  background: linear-gradient(90deg, #f4f6fa, #e8ecf3);
  color: #172133;
  font-size: 12px;
}

.network-table :deep(.el-table__cell) {
  padding: 8px 6px;
}

.traffic-detail-table :deep(.el-table__header th) {
  height: 40px;
  padding: 0 4px;
}

.traffic-detail-table :deep(.el-table__cell) {
  padding: 5px 4px;
}

.traffic-detail-table :deep(.el-table__cell .cell) {
  min-width: 0;
  line-height: 1.25;
}

.traffic-port-desc {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mono {
  font-family: Monaco, "Courier New", monospace;
}

.strong {
  color: #172133;
  font-weight: 700;
}

.blue {
  color: #409eff;
  font-weight: 700;
}

.red {
  color: #f56c6c;
  font-weight: 700;
}

.pagination-wrap {
  display: flex;
  justify-content: flex-end;
  padding-top: 8px;
}

.chart-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.chart-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.chart-card {
  border: 1px solid rgba(76, 139, 245, 0.15);
  border-radius: 16px;
  background: linear-gradient(135deg, rgba(26, 26, 46, 0.98), rgba(22, 33, 62, 0.95));
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.12);
}

.chart-head {
  padding: 12px 18px;
  border-bottom: 1px solid rgba(76, 139, 245, 0.1);
  color: #c8d0dc;
  font-size: 13px;
  font-weight: 700;
}

.chart-container {
  width: 100%;
  height: 270px;
}

.chart-trend {
  height: 390px;
}

@media (max-width: 1280px) {
  .kpi-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .chart-row {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .monitor-header,
  .toolbar-actions {
    align-items: stretch;
    flex-direction: column;
  }

  .kpi-grid {
    grid-template-columns: 1fr;
  }
}
</style>
