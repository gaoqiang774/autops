<template>
  <div class="db-monitor-page">
    <section class="monitor-shell">
      <header class="monitor-header">
        <div class="monitor-header-copy">
          <span class="ops-page-header__icon" aria-hidden="true">
            <img :src="databaseSvg" alt="">
          </span>
          <div class="ops-page-header__content">
            <h1>数据库监控大盘</h1>
            <span class="ops-page-header__divider" aria-hidden="true"></span>
            <p>统一监控 MySQL、PostgreSQL 与 Redis 资产及其 Prometheus Target。</p>
          </div>
        </div>
        <div class="header-actions">
          <el-select
            v-model="selectedAssetId"
            class="asset-select"
            filterable
            remote
            reserve-keyword
            placeholder="请选择数据库资产"
            :loading="assetLoading"
            :remote-method="searchAssets"
            @visible-change="handleAssetVisible"
            @change="handleAssetChange"
          >
            <el-option
              v-for="asset in databaseAssets"
              :key="asset.id"
              :label="assetLabel(asset)"
              :value="asset.id"
            >
              <div class="asset-option">
                <strong>{{ asset.instanceName || asset.name }}</strong>
                <el-tag size="small" effect="light">{{ engineText(asset) }}</el-tag>
                <span>{{ asset.endpoint || joinEndpoint(asset.hostIp, asset.port) }}</span>
                <el-tag v-if="asset.prometheusAccountId" size="small" effect="light">{{ asset.prometheusAccountAlias || 'Prometheus' }}</el-tag>
                <el-tag v-else size="small" type="warning" effect="light">未绑定 Prometheus</el-tag>
              </div>
            </el-option>
          </el-select>
          <el-segmented v-model="query.range" :options="rangeOptions" @change="loadDashboard" />
          <el-select v-model="query.step" class="step-select" placeholder="采样" @change="loadDashboard">
            <el-option v-for="item in stepOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
          <el-button
            class="monitor-refresh-action"
            type="warning"
            :icon="Refresh"
            :loading="dashboardLoading"
            @click="loadDashboard"
          >
            刷新
          </el-button>
        </div>
      </header>

      <section v-if="!selectedAssetId" class="empty-panel">
        <el-empty description="请选择一个已绑定 Prometheus 数据源的 MySQL、PostgreSQL 或 Redis 资产" />
      </section>

      <template v-else>
        <section class="status-grid">
          <article class="target-panel">
            <div class="panel-title">
              <span class="status-dot" :class="targetHealthClass"></span>
              <strong>Prometheus Target</strong>
              <el-tag :type="targetTagType" effect="light">{{ targetStatusText }}</el-tag>
            </div>
            <div class="target-summary">
              <div class="target-summary-item">
                <span>监控源</span>
                <strong>{{ dashboardTarget.prometheus || selectedAsset?.prometheusEndpoint || '-' }}</strong>
              </div>
              <div class="target-summary-item">
                <span>指标地址</span>
                <strong>{{ dashboardTarget.metricsUrl || dashboardTarget.scrapeUrl || '-' }}</strong>
                <el-tag v-if="dashboardTarget.metricsType" size="small" effect="light">{{ dashboardTarget.metricsType }}</el-tag>
              </div>
              <div class="target-summary-item">
                <span>关联主机地址</span>
                <strong>{{ dashboardTarget.hostScrapeUrl || '-' }}</strong>
              </div>
            </div>
            <details class="target-details">
              <summary>详情</summary>
              <div class="target-lines target-lines--detail">
                <span>匹配方式：{{ dashboardTarget.matchBy || '-' }}</span>
                <span class="selector-line">Selector：{{ dashboardTarget.selector || '-' }}</span>
                <span class="selector-line">Host Selector：{{ dashboardTarget.hostSelector || '-' }}</span>
                <span class="selector-line">Labels：{{ formatLabels(dashboardTarget.labels) }}</span>
              </div>
            </details>
          </article>
          <article class="asset-panel">
            <div class="panel-title">
              <strong>{{ selectedAsset?.instanceName || selectedAsset?.name || dashboard.asset?.name || `${currentEngineConfig.label} 资产` }}</strong>
              <el-tag effect="light">{{ currentEngineConfig.label }}</el-tag>
            </div>
            <div class="asset-lines">
              <span>{{ selectedAsset?.endpoint || joinEndpoint(selectedAsset?.hostIp, selectedAsset?.port) || '-' }}</span>
              <span>环境：{{ environmentText(selectedAsset?.environment || dashboard.asset?.environment) }}</span>
              <span>Prometheus：{{ selectedAsset?.prometheusAccountAlias || dashboard.asset?.prometheusAccountAlias || '-' }}</span>
            </div>
          </article>
        </section>

        <section v-if="alertMessages.length" class="warning-stack">
          <el-alert
            v-for="message in alertMessages"
            :key="message"
            :title="message"
            type="warning"
            show-icon
            :closable="false"
          />
        </section>

        <div v-loading="dashboardLoading" class="dashboard-content">
          <section class="overview-grid">
            <article
              v-for="group in overviewGroups"
              :key="group.key"
              class="metric-card"
              :class="`metric-card--${group.key}`"
            >
              <div class="metric-card-head">
                <div>
                  <span class="metric-card-eyebrow">{{ group.eyebrow }}</span>
                  <strong>{{ group.title }}</strong>
                </div>
                <span class="metric-count">{{ group.items.length }} 项</span>
              </div>
              <div class="metric-list">
                <div
                  v-for="item in group.items"
                  :key="item.key || item.name"
                  class="metric-item"
                  :class="`metric-item--${item.status || 'normal'}`"
                >
                  <div class="metric-item-head">
                    <span>{{ item.name || item.key }}</span>
                    <el-tag size="small" effect="light" :type="metricTagType(item.status)">{{ metricStatusText(item.status) }}</el-tag>
                  </div>
                  <strong>{{ formatMetricDisplay(item).value }}<small>{{ formatMetricDisplay(item).unit }}</small></strong>
                  <p>{{ item.description || '暂无说明' }}</p>
                </div>
              </div>
            </article>
          </section>

          <section class="trend-grid">
            <article v-for="panel in trendPanels" :key="panel.key || panel.title" class="trend-panel">
              <div class="panel-title">
                <strong>{{ panel.title || panel.key }}</strong>
                <span>{{ chartUnitLabel(panel) }}</span>
              </div>
              <div :ref="el => setChartRef(panel.key || panel.title, el)" class="chart-box"></div>
            </article>
          </section>

          <section v-if="breakdownPanels.length" class="breakdown-grid">
            <article v-for="panel in breakdownPanels" :key="panel.key || panel.title" class="table-panel">
              <div class="panel-title">
                <strong>{{ panel.title || breakdownTitle(panel.key) }}</strong>
                <span>{{ panel.rows?.length || 0 }} 条</span>
              </div>
              <el-table :data="panel.rows || []" height="260" class="breakdown-table">
                <el-table-column
                  v-for="column in breakdownColumns(panel)"
                  :key="column.key"
                  :prop="column.key"
                  :label="column.title"
                  :min-width="column.minWidth || 120"
                  show-overflow-tooltip
                >
                  <template #default="{ row }">
                    <span>{{ formatBreakdownCell(row[column.key], column) }}</span>
                  </template>
                </el-table-column>
              </el-table>
            </article>
          </section>
        </div>
      </template>
    </section>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { Refresh } from '@element-plus/icons-vue'
import * as echarts from 'echarts'
import dbApi from '@/api/database'
import databaseSvg from '@/assets/image/数据库.svg'

const MYSQL_ENGINE = 1
const POSTGRESQL_ENGINE = 2
const REDIS_ENGINE = 4

const engineConfigs = {
  [MYSQL_ENGINE]: {
    key: 'mysql',
    label: 'MySQL',
    targetName: 'MySQL exporter',
    fetch: (id, params) => dbApi.getMysqlMonitorDashboard(id, params)
  },
  [POSTGRESQL_ENGINE]: {
    key: 'postgresql',
    label: 'PostgreSQL',
    targetName: 'PostgreSQL exporter',
    fetch: (id, params) => dbApi.getPostgresqlMonitorDashboard(id, params)
  },
  [REDIS_ENGINE]: {
    key: 'redis',
    label: 'Redis',
    targetName: 'Redis exporter',
    fetch: (id, params) => dbApi.getRedisMonitorDashboard(id, params)
  }
}

const supportedEngines = Object.keys(engineConfigs).map(Number)
const route = useRoute()

const rangeOptions = [
  { label: '1h', value: '1h' },
  { label: '6h', value: '6h' },
  { label: '24h', value: '24h' }
]

const stepOptions = [
  { label: '1 分钟', value: '1m' },
  { label: '5 分钟', value: '5m' },
  { label: '15 分钟', value: '15m' }
]

const environments = {
  prod: '生产',
  pre: '预发',
  test: '测试',
  dev: '开发'
}

const assetLoading = ref(false)
const dashboardLoading = ref(false)
const assetKeyword = ref('')
const databaseAssets = ref([])
const selectedAssetId = ref('')
const dashboard = ref({
  asset: {},
  target: {},
  overview: [],
  trends: [],
  breakdown: [],
  range: {},
  warnings: []
})
const chartRefs = new Map()
const chartInstances = new Map()
const query = reactive({
  range: '6h',
  step: '1m'
})

const selectedAsset = computed(() => {
  const id = String(selectedAssetId.value || '')
  return databaseAssets.value.find(item => String(item.id) === id) || dashboard.value.asset || null
})

const selectedEngine = computed(() => getAssetEngine(selectedAsset.value))
const currentEngineConfig = computed(() => engineConfigs[selectedEngine.value] || engineConfigs[MYSQL_ENGINE])
const dashboardTarget = computed(() => normalizeMonitorTarget(dashboard.value.target))

const overviewItems = computed(() => Array.isArray(dashboard.value.overview) ? dashboard.value.overview : [])
const overviewGroups = computed(() => {
  const hostItems = []
  const serviceItems = []
  overviewItems.value.forEach(item => {
    if (isHostOverviewMetric(item)) {
      hostItems.push(item)
    } else {
      serviceItems.push(item)
    }
  })
  return [
    { key: 'host', title: '主机指标', eyebrow: 'HOST METRICS', items: hostItems },
    { key: 'service', title: `${currentEngineConfig.value.label} 指标`, eyebrow: `${currentEngineConfig.value.key.toUpperCase()} METRICS`, items: serviceItems }
  ].filter(group => group.items.length)
})
const trendPanels = computed(() => Array.isArray(dashboard.value.trends) ? dashboard.value.trends : [])
const breakdownPanels = computed(() => Array.isArray(dashboard.value.breakdown) ? dashboard.value.breakdown : [])

const targetHealthClass = computed(() => {
  const target = dashboardTarget.value
  if (!target.matched) return 'offline'
  return String(target.health || '').toLowerCase() === 'up' ? 'online' : 'warning'
})

const targetTagType = computed(() => {
  const target = dashboardTarget.value
  if (!target.matched) return 'warning'
  return String(target.health || '').toLowerCase() === 'up' ? 'success' : 'danger'
})

const targetStatusText = computed(() => {
  const target = dashboardTarget.value
  if (!target.matched) return '未匹配'
  return String(target.health || '').toLowerCase() === 'up' ? '正常' : `异常：${target.health || '-'}`
})

const alertMessages = computed(() => {
  const messages = []
  const target = dashboardTarget.value
  if (selectedAssetId.value && !target.matched) messages.push(`未匹配到 ${currentEngineConfig.value.targetName} target`)
  if (target.matched && String(target.health || '').toLowerCase() !== 'up') messages.push('Prometheus target 异常，请检查 exporter 状态')
  if (overviewItems.value.some(item => item.required === true && item.status === 'missing')) {
    messages.push('系统资源指标未采集，请检查 node_exporter 或 Prometheus target 配置')
  }
  const warnings = dashboard.value.warnings || []
  warnings.forEach(item => {
    const text = typeof item === 'string' ? item : (item.message || item.name || item.key)
    if (target.matched && isTargetMatchWarning(text)) return
    if (text) messages.push(text)
  })
  return Array.from(new Set(messages))
})

function unwrap(response) {
  const payload = response?.data || response || {}
  return payload.data !== undefined ? payload.data : payload
}

function normalizeList(payload) {
  if (Array.isArray(payload)) return payload
  if (Array.isArray(payload?.list)) return payload.list
  if (Array.isArray(payload?.rows)) return payload.rows
  return []
}

function normalizeMonitorTarget(target = {}) {
  const labels = target.labels || target.Labels || {}
  const scrapeUrl = target.scrapeUrl || target.scrapeURL || target.scrape_url || target.ScrapeURL || ''
  const metricsUrl = target.metricsUrl || target.metricsURL || target.metrics_url || target.MetricsURL || ''
  const hostScrapeUrl = target.hostScrapeUrl || target.hostScrapeURL || target.host_scrape_url || target.HostScrapeURL || ''
  const selector = target.selector || target.Selector || ''
  const hostSelector = target.hostSelector || target.host_selector || target.HostSelector || ''
  const matched = target.matched === true ||
    target.Matched === true ||
    String(target.matched).toLowerCase() === 'true' ||
    String(target.Matched).toLowerCase() === 'true' ||
    Boolean(selector || scrapeUrl || metricsUrl || hostScrapeUrl || Object.keys(labels).length)

  return {
    ...target,
    matched,
    health: target.health || target.Health || '',
    scrapeUrl,
    metricsUrl,
    metricsType: target.metricsType || target.metrics_type || target.MetricsType || '',
    hostScrapeUrl,
    selector,
    hostSelector,
    matchBy: target.matchBy || target.match_by || target.MatchBy || '',
    prometheus: target.prometheus || target.Prometheus || '',
    labels
  }
}

function normalizeMonitorDashboard(payload = {}) {
  const data = payload || {}
  return {
    ...data,
    asset: data.asset || data.Asset || {},
    target: normalizeMonitorTarget(data.target || data.Target || {}),
    overview: Array.isArray(data.overview) ? data.overview : (Array.isArray(data.Overview) ? data.Overview : []),
    trends: Array.isArray(data.trends) ? data.trends : (Array.isArray(data.Trends) ? data.Trends : []),
    breakdown: Array.isArray(data.breakdown) ? data.breakdown : (Array.isArray(data.Breakdown) ? data.Breakdown : []),
    range: data.range || data.Range || {},
    warnings: Array.isArray(data.warnings) ? data.warnings : (Array.isArray(data.Warnings) ? data.Warnings : [])
  }
}

function joinEndpoint(host, port) {
  return [host, port].filter(Boolean).join(':')
}

function getAssetEngine(asset = {}) {
  return Number(asset.type || asset.engine || asset.dbType || 0)
}

function engineText(asset = {}) {
  return engineConfigs[getAssetEngine(asset)]?.label || '数据库'
}

function assetLabel(asset = {}) {
  return `${engineText(asset)} ${asset.instanceName || asset.name || `资产 #${asset.id}`} ${asset.endpoint || joinEndpoint(asset.hostIp, asset.port) || ''}`.trim()
}

function environmentText(value) {
  return environments[value] || value || '-'
}

function metricTagType(status) {
  return {
    normal: 'success',
    warning: 'warning',
    critical: 'danger',
    missing: 'info'
  }[status] || 'info'
}

function metricStatusText(status) {
  return {
    normal: '正常',
    warning: '告警',
    critical: '严重',
    missing: '缺失'
  }[status] || '未知'
}

function isTargetMatchWarning(text = '') {
  return /metric match failed|target match failed|match failed|未匹配/i.test(String(text))
}

function isDatabaseStatusMetric(item = {}) {
  const key = String(item.key || '').toLowerCase()
  const name = String(item.name || '').toLowerCase()
  const engineLabel = String(currentEngineConfig.value.label || '').toLowerCase()
  return key === 'up' ||
    key === `${currentEngineConfig.value.key}up` ||
    key === `${currentEngineConfig.value.key}_up` ||
    name.includes(`${engineLabel} 状态`.toLowerCase()) ||
    name.includes('状态')
}

function isHostOverviewMetric(item = {}) {
  const key = String(item.key || '').toLowerCase()
  const name = String(item.name || '').toLowerCase()
  const description = String(item.description || '').toLowerCase()
  return item.required === true ||
    name.includes('主机') ||
    description.includes('node_exporter') ||
    description.includes('hostip') ||
    key.startsWith('cpu') ||
    key.startsWith('disk') ||
    key === 'memoryusage' ||
    key.startsWith('swap') ||
    key.startsWith('host') ||
    key.startsWith('os')
}

function isBytesUnit(unit = '') {
  return ['b', 'byte', 'bytes', 'bytes/s', 'b/s'].includes(String(unit).trim().toLowerCase())
}

function isBytesRateUnit(unit = '') {
  return ['bytes/s', 'b/s'].includes(String(unit).trim().toLowerCase())
}

function isMegabytesRateUnit(unit = '') {
  return ['mb/s', 'mib/s'].includes(String(unit).trim().toLowerCase())
}

function formatMegabytes(value, suffix = '') {
  const num = Number(value)
  if (!Number.isFinite(num)) return { value: value ?? '-', unit: `MB${suffix}` }
  const mb = num / 1024 / 1024
  const valueText = Math.abs(mb) >= 100 ? mb.toFixed(0) : Math.abs(mb) >= 10 ? mb.toFixed(1) : mb.toFixed(2)
  return { value: valueText, unit: `MB${suffix}` }
}

function formatBytes(value, suffix = '') {
  const num = Number(value)
  if (!Number.isFinite(num)) return { value: value ?? '-', unit: suffix }
  const abs = Math.abs(num)
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = abs
  let index = 0
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024
    index += 1
  }
  const signedSize = num < 0 ? -size : size
  const valueText = size >= 100 ? signedSize.toFixed(0) : size >= 10 ? signedSize.toFixed(1) : signedSize.toFixed(2)
  return { value: valueText, unit: `${units[index]}${suffix}` }
}

function formatNumber(value) {
  if (value === undefined || value === null || value === '') return '-'
  const num = Number(value)
  if (!Number.isFinite(num)) return value
  if (Math.abs(num) >= 100) return num.toFixed(0)
  if (Math.abs(num) >= 10) return num.toFixed(1)
  return num.toFixed(2)
}

function isDurationMetric(item = {}) {
  const key = String(item.key || '').toLowerCase()
  const name = String(item.name || '').toLowerCase()
  const unit = String(item.unit || '').trim().toLowerCase()
  return unit === 's' && (key.includes('uptime') || name.includes('运行时长'))
}

function formatDurationSeconds(value) {
  const totalSeconds = Number(value)
  if (!Number.isFinite(totalSeconds)) return { value: value ?? '-', unit: '' }

  const days = Math.floor(totalSeconds / 86400)
  const hours = Math.floor((totalSeconds % 86400) / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)

  if (days > 0) return { value: String(days), unit: hours > 0 ? `天 ${hours}小时` : '天' }
  if (hours > 0) return { value: String(hours), unit: minutes > 0 ? `小时 ${minutes}分钟` : '小时' }
  if (minutes > 0) return { value: String(minutes), unit: '分钟' }
  return { value: String(Math.floor(totalSeconds)), unit: '秒' }
}

function formatMetricDisplay(item = {}) {
  if (isDatabaseStatusMetric(item)) {
    return { value: Number(item.value) === 1 ? '正常' : '异常', unit: '' }
  }

  if (isDurationMetric(item)) {
    return formatDurationSeconds(item.value)
  }

  const unit = item.unit || ''
  if (isBytesRateUnit(unit)) {
    return formatMegabytes(item.value, '/s')
  }

  if (isBytesUnit(unit)) {
    return formatBytes(item.value, String(unit).includes('/') ? '/s' : '')
  }

  return { value: formatNumber(item.value), unit }
}

function formatAxisValue(value, unit = '') {
  if (isBytesRateUnit(unit) || isMegabytesRateUnit(unit)) {
    const result = formatMegabytes(value, '/s')
    return `${result.value}${result.unit}`
  }

  if (isBytesUnit(unit)) {
    const result = formatBytes(value, String(unit).includes('/') ? '/s' : '')
    return `${result.value}${result.unit}`
  }
  return `${formatNumber(value)}${unit || ''}`
}

function formatLabels(labels = {}) {
  const entries = Object.entries(labels || {})
  if (!entries.length) return '-'
  return entries.map(([key, value]) => `${key}=${value}`).join(', ')
}

function formatTime(timestamp) {
  if (!timestamp) return ''
  const date = new Date(Number(timestamp))
  if (Number.isNaN(date.getTime())) return ''
  return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
}

function breakdownTitle(key) {
  return {
    processByUser: '连接用户 Top 10',
    processByHost: '连接来源 Top 10',
    databaseSize: '数据库大小 Top 10',
    databaseConnections: '数据库连接 Top 10',
    lockModes: '锁模式分布',
    databaseTransactionRate: '数据库 TPS Top 10',
    dbKeyDistribution: 'DB Key 分布',
    commandTop: '命令 Top',
    commandsTop: '命令 Top',
    keyspace: 'DB Key 分布'
  }[key] || key || '明细'
}

function columnLabel(column) {
  return {
    user: '用户',
    host: '来源',
    count: '连接数',
    value: '值',
    name: '名称',
    database: '数据库',
    datname: '数据库',
    mode: '锁模式',
    size: '大小',
    rate: '速率',
    tps: 'TPS',
    db: 'DB',
    keys: 'Key 数',
    expires: '过期 Key',
    avgTtl: '平均 TTL',
    cmd: '命令',
    command: '命令',
    calls: '调用次数',
    qps: 'QPS',
    usec: '耗时',
    usecPerCall: '单次耗时'
  }[column] || column
}

function normalizeBreakdownColumn(column) {
  if (typeof column === 'string') {
    return {
      key: column,
      title: columnLabel(column),
      minWidth: column === 'labels' ? 240 : 120
    }
  }

  const key = column?.key || column?.prop || column?.field || ''
  return {
    ...column,
    key,
    title: column?.title || column?.label || columnLabel(key),
    minWidth: column?.minWidth || (key === 'labels' ? 240 : 120)
  }
}

function breakdownColumns(panel = {}) {
  const configuredColumns = Array.isArray(panel.columns)
    ? panel.columns.map(normalizeBreakdownColumn).filter(column => column.key)
    : []
  if (configuredColumns.length) return configuredColumns

  const first = panel.rows?.[0]
  if (!first || typeof first !== 'object') {
    return ['name', 'value'].map(normalizeBreakdownColumn)
  }
  return Object.keys(first).map(normalizeBreakdownColumn)
}

function formatBreakdownCell(value, column = {}) {
  if (value === undefined || value === null || value === '') return '-'

  if (column.key === 'labels' && typeof value === 'object') {
    return formatLabels(value)
  }

  if (typeof value === 'object') {
    return formatLabels(value)
  }

  if (typeof value === 'number') {
    if (isBytesUnit(column.unit)) {
      const result = formatBytes(value, String(column.unit).includes('/') ? '/s' : '')
      return `${result.value}${result.unit}`
    }
    return `${formatNumber(value)}${column.unit || ''}`
  }

  return `${value}${column.unit || ''}`
}

function setChartRef(key, el) {
  if (!key || !el) return
  chartRefs.set(String(key), el)
}

function isSystemResourcePanel(panel = {}) {
  return String(panel.key || '').toLowerCase() === 'systemresource' || String(panel.title || '').includes('CPU / 内存')
}

function isLoadSeries(item = {}) {
  return /load/i.test(String(item.name || item.key || ''))
}

function isMixedUnit(unit = '') {
  return String(unit || '').includes('/')
}

function panelAxisUnit(panel = {}) {
  if (isSystemResourcePanel(panel)) return '%'
  const unit = panel.unit || ''
  if (isBytesRateUnit(unit)) return 'MB/s'
  return isMixedUnit(unit) ? '' : unit
}

function chartUnitLabel(panel = {}) {
  if (isBytesRateUnit(panel.unit)) return 'MB/s'
  return panel.unit || ''
}

function inferSeriesUnit(item = {}) {
  const unit = item.unit || item.Unit || ''
  if (unit) return unit

  const text = `${item.key || ''} ${item.name || ''}`.toLowerCase()
  if (text.includes('hit') || text.includes('命中')) return '%'
  if (text.includes('time') || text.includes('耗时') || text.includes('延迟')) return 'ms'
  if (text.includes('bytes') || text.includes('throughput') || text.includes('流量') || text.includes('吞吐')) return 'bytes/s'
  if (text.includes('fsync') || text.includes('pending') || text.includes('buffers') || text.includes('buffer') || text.includes('block') || text.includes('块') || text.includes('缓冲')) return '次/s'
  return ''
}

function seriesUnit(panel = {}, item = {}) {
  if (isSystemResourcePanel(panel)) {
    return isLoadSeries(item) ? ' load' : '%'
  }

  const inferredUnit = inferSeriesUnit(item)
  if (inferredUnit) return inferredUnit

  return panelAxisUnit(panel)
}

function chartTooltipFormatter(panel = {}) {
  return params => {
    const items = Array.isArray(params) ? params : [params]
    if (!items.length) return ''
    const seriesByName = new Map((panel.series || []).map(item => [item.name, item]))
    const lines = [`<strong>${items[0].axisValueLabel || items[0].name || ''}</strong>`]
    items.forEach(item => {
      const source = seriesByName.get(item.seriesName) || {}
      lines.push(`${item.marker}${item.seriesName}: ${formatAxisValue(item.value, seriesUnit(panel, source))}`)
    })
    return lines.join('<br/>')
  }
}

function createValueAxis(unit = '', extra = {}) {
  return {
    type: 'value',
    name: unit,
    nameTextStyle: { color: '#71839a' },
    nameGap: 10,
    splitLine: { lineStyle: { color: '#edf2f7' } },
    axisLabel: {
      color: '#71839a',
      hideOverlap: true,
      formatter: value => formatAxisValue(value, unit)
    },
    ...extra
  }
}

function chartYAxes(panel = {}) {
  if (isSystemResourcePanel(panel)) {
    return [
      createValueAxis('%'),
      createValueAxis('load', {
        position: 'right',
        splitLine: { show: false },
        axisLabel: {
          color: '#71839a',
          hideOverlap: true,
          formatter: value => formatNumber(value)
        }
      })
    ]
  }
  return createValueAxis(panelAxisUnit(panel))
}

function buildChartOption(panel) {
  const series = Array.isArray(panel.series) ? panel.series : []
  const timestamps = Array.from(new Set(series.flatMap(item => (item.points || []).map(point => point.timestamp)))).sort((a, b) => a - b)
  return {
    tooltip: {
      trigger: 'axis',
      confine: true,
      formatter: chartTooltipFormatter(panel)
    },
    legend: {
      top: 0,
      right: 0,
      type: 'scroll',
      textStyle: { color: '#52657a', fontSize: 11 }
    },
    grid: {
      left: 16,
      right: isSystemResourcePanel(panel) ? 44 : 18,
      top: 42,
      bottom: 30,
      containLabel: true
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: timestamps.map(formatTime),
      axisLine: { lineStyle: { color: '#d8e2ef' } },
      axisLabel: { color: '#71839a' }
    },
    yAxis: chartYAxes(panel),
    color: ['#409eff', '#10b981', '#f59e0b', '#64748b', '#ef4444', '#6366f1'],
    series: series.map(item => {
      const pointMap = new Map((item.points || []).map(point => [point.timestamp, point.value]))
      return {
        name: item.name,
        type: 'line',
        yAxisIndex: isSystemResourcePanel(panel) && isLoadSeries(item) ? 1 : 0,
        smooth: true,
        showSymbol: false,
        data: timestamps.map(timestamp => pointMap.get(timestamp) ?? null),
        connectNulls: false,
        lineStyle: { width: 2 },
        areaStyle: { opacity: 0.08 }
      }
    })
  }
}

async function renderCharts() {
  await nextTick()
  const activeKeys = new Set(trendPanels.value.map(panel => String(panel.key || panel.title)))
  chartInstances.forEach((chart, key) => {
    if (!activeKeys.has(key)) {
      chart.dispose()
      chartInstances.delete(key)
    }
  })

  trendPanels.value.forEach(panel => {
    const key = String(panel.key || panel.title)
    const el = chartRefs.get(key)
    if (!el) return
    const chart = chartInstances.get(key) || echarts.init(el)
    chartInstances.set(key, chart)
    chart.setOption(buildChartOption(panel), true)
    chart.resize()
  })
}

async function loadAssets(keyword = '') {
  assetLoading.value = true
  assetKeyword.value = keyword || ''
  try {
    const payload = unwrap(await dbApi.getCrudAssets({
      page: 1,
      pageSize: 100,
      keyword: assetKeyword.value || undefined
    }))
    databaseAssets.value = normalizeList(payload).filter(item => supportedEngines.includes(getAssetEngine(item)))
    if (applyRouteSelection()) {
      await loadDashboard()
    } else if (!selectedAssetId.value && databaseAssets.value.length) {
      selectedAssetId.value = databaseAssets.value[0].id
      await loadDashboard()
    }
  } finally {
    assetLoading.value = false
  }
}

function searchAssets(keyword = '') {
  loadAssets(keyword)
}

function handleAssetVisible(visible) {
  if (visible) loadAssets(assetKeyword.value)
}

function handleAssetChange() {
  loadDashboard()
}

function applyRouteSelection() {
  const routeAssetId = String(route.query.assetId || '')
  if (!routeAssetId) return false
  const matched = databaseAssets.value.find(item => String(item.id) === routeAssetId)
  if (!matched) return false
  if (String(selectedAssetId.value || '') === routeAssetId) return true
  selectedAssetId.value = routeAssetId
  return true
}

async function loadDashboard() {
  if (!selectedAssetId.value) return
  const config = currentEngineConfig.value
  dashboardLoading.value = true
  try {
    const payload = unwrap(await config.fetch(selectedAssetId.value, {
      range: query.range,
      step: query.step
    })) || {}
    dashboard.value = normalizeMonitorDashboard(payload)
    await renderCharts()
  } finally {
    dashboardLoading.value = false
  }
}

function handleResize() {
  chartInstances.forEach(chart => chart.resize())
}

watch(trendPanels, renderCharts)
watch(() => route.query.assetId, async () => {
  if (!databaseAssets.value.length) return
  if (applyRouteSelection()) {
    await loadDashboard()
  }
})

onMounted(() => {
  loadAssets()
  window.addEventListener('resize', handleResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  chartInstances.forEach(chart => chart.dispose())
  chartInstances.clear()
})
</script>

<style scoped>
.db-monitor-page {
  box-sizing: border-box;
  --ops-blue-500: #409eff;
  --ops-blue-600: #2563eb;
  --ops-green-500: #10b981;
  --ops-orange-500: #f59e0b;
  --ops-red-500: #ef4444;
  --ops-button-warning: #e6a23c;
  --ops-button-warning-hover: #d8922f;
  --ops-text-heading: #0f172a;
  --ops-text-primary: #172033;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-text-weak: #7b8aa0;
  --ops-bg-surface: #ffffff;
  --ops-bg-surface-soft: #fbfdff;
  --ops-bg-subtle: #f3f7fc;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  --ops-border-inner: #dce8f5;
  --ops-border-hover: #b9d4ff;
  --ops-shadow-card: 0 8px 24px rgba(80, 124, 180, 0.07);
  --aiops-bg-page: #f6f8fb;
  --aiops-bg-page-soft: var(--ops-bg-subtle);
  --aiops-surface-strong: var(--ops-bg-surface);
  --aiops-surface-warm: var(--ops-bg-surface-soft);
  --aiops-text-primary: var(--ops-text-primary);
  --aiops-text-secondary: var(--ops-text-secondary);
  --aiops-text-muted: var(--ops-text-weak);
  --aiops-border-soft: var(--ops-border-soft);
  --aiops-border-cool: var(--ops-border-soft);
  --aiops-border-mid: var(--ops-border);
  --aiops-brand: var(--ops-blue-600);
  --aiops-brand-active: #1d4ed8;
  --aiops-brand-soft: #eef6ff;
  --aiops-success: var(--ops-green-500);
  --aiops-warning: var(--ops-orange-500);
  --aiops-danger: var(--ops-red-500);
  height: calc(100vh - 124px);
  min-height: 0;
  overflow: auto;
  padding: 7px;
  background: transparent;
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

:global(.home-container .el-main:has(.db-monitor-page)) {
  padding: 10px;
  overflow: hidden;
}

.monitor-shell {
  display: flex;
  box-sizing: border-box;
  height: 100%;
  min-width: 0;
  min-height: 0;
  flex-direction: column;
  gap: 8px;
  overflow: auto;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: var(--ops-bg-surface);
  box-shadow: var(--ops-shadow-card);
}

.monitor-shell::before {
  display: none;
}

.monitor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-shrink: 0;
  padding: 8px 12px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-surface);
}

.monitor-header-copy {
  display: flex;
  min-width: 0;
  align-items: center;
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
  min-width: 0;
  align-items: center;
  gap: 10px;
}

.ops-page-header__divider {
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  background: var(--ops-border-soft);
}

.ops-page-header__content h1 {
  margin: 0;
  color: var(--ops-text-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  white-space: nowrap;
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
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
}

.header-actions :deep(.el-button) {
  height: 30px;
  min-height: 30px;
  padding: 0 10px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  transition:
    color 160ms ease,
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease,
    transform 160ms ease;
}

.monitor-refresh-action {
  border-color: var(--ops-button-warning);
  background: var(--ops-button-warning);
  color: #fff;
}

.monitor-refresh-action:not(.is-disabled):hover,
.monitor-refresh-action:not(.is-disabled):focus-visible {
  border-color: var(--ops-button-warning-hover);
  background: var(--ops-button-warning-hover);
  color: #fff;
  transform: translateY(-1px);
}

.monitor-refresh-action:active {
  transform: translateY(0);
}

.monitor-refresh-action:focus-visible {
  outline: 2px solid rgba(64, 158, 255, 0.35);
  outline-offset: 2px;
}

.asset-select {
  width: 320px;
}

.step-select {
  width: 96px;
}

.asset-select :deep(.el-select__wrapper),
.step-select :deep(.el-select__wrapper) {
  min-height: 30px;
  border: 0;
  border-radius: 8px;
  background: var(--ops-bg-surface-soft);
  box-shadow: 0 0 0 1px var(--ops-border-inner) inset;
}

.asset-select :deep(.el-select__wrapper:hover),
.step-select :deep(.el-select__wrapper:hover),
.asset-select :deep(.is-focused),
.step-select :deep(.is-focused) {
  box-shadow: 0 0 0 1px var(--ops-blue-500) inset;
}

.header-actions :deep(.el-segmented) {
  --el-segmented-item-selected-color: var(--aiops-brand);
  --el-segmented-item-selected-bg-color: #fff;
  min-height: 30px;
  padding: 2px;
  border: 1px solid var(--ops-border-inner);
  border-radius: 8px;
  background: var(--ops-bg-subtle);
}

.header-actions :deep(.el-segmented__item) {
  border-radius: 6px;
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

.asset-option {
  display: grid;
  grid-template-columns: minmax(120px, 1fr) auto minmax(120px, 0.8fr) auto;
  align-items: center;
  gap: 10px;
}

.asset-option strong,
.asset-option span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.asset-option span {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.empty-panel {
  display: grid;
  flex: 1;
  min-height: 360px;
  place-items: center;
}

.status-grid,
.dashboard-content,
.overview-grid,
.trend-grid,
.breakdown-grid,
.warning-stack {
  margin: 0 10px;
}

.status-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.5fr) minmax(280px, 0.7fr);
  gap: 8px;
}

.target-panel,
.asset-panel,
.metric-card,
.trend-panel,
.table-panel {
  min-width: 0;
  border: 1px solid var(--ops-border-inner);
  border-radius: 10px;
  background: var(--ops-bg-surface);
  box-shadow: 0 4px 14px rgba(80, 124, 180, 0.04);
}

.target-panel,
.asset-panel {
  padding: 10px;
}

.panel-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  color: var(--aiops-text-primary);
}

.panel-title strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
  font-weight: 700;
}

.panel-title span {
  flex-shrink: 0;
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.status-dot {
  width: 8px;
  height: 8px;
  flex-shrink: 0;
  border-radius: 999px;
  background: var(--aiops-text-muted);
}

.status-dot.online {
  background: var(--aiops-success);
  box-shadow: 0 0 0 4px rgba(53, 194, 135, 0.14);
}

.status-dot.warning {
  background: var(--aiops-warning);
  box-shadow: 0 0 0 4px rgba(243, 179, 76, 0.15);
}

.status-dot.offline {
  background: var(--aiops-danger);
  box-shadow: 0 0 0 4px rgba(245, 108, 108, 0.12);
}

.target-summary {
  display: grid;
  grid-template-columns: minmax(180px, 0.9fr) minmax(240px, 1.2fr) minmax(200px, 1fr);
  gap: 8px;
  margin-top: 8px;
}

.target-summary-item {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 8px;
  padding: 8px 10px;
  border: 1px solid var(--ops-border-soft);
  border-radius: 8px;
  background: var(--ops-bg-surface-soft);
}

.target-summary-item span {
  flex-shrink: 0;
  padding: 3px 7px;
  border: 1px solid rgba(243, 178, 75, 0.22);
  border-radius: 999px;
  background: rgba(255, 246, 222, 0.86);
  color: var(--aiops-text-muted);
  font-size: 11px;
  font-weight: 700;
}

.target-summary-item strong {
  min-width: 0;
  overflow: hidden;
  color: var(--aiops-text-secondary);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.target-lines,
.asset-lines {
  display: grid;
  gap: 6px;
  margin-top: 8px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.target-lines span,
.asset-lines span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.selector-line {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
}

.target-details {
  margin-top: 8px;
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.target-details summary {
  width: fit-content;
  cursor: pointer;
  color: var(--aiops-brand);
  font-weight: 600;
  outline: none;
}

.target-lines--detail {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid var(--ops-border-soft);
}

.warning-stack {
  display: grid;
  gap: 8px;
}

.warning-stack :deep(.el-alert) {
  border: 1px solid rgba(230, 162, 60, 0.32);
  border-radius: 8px;
  background: #fffaf0;
}

.dashboard-content {
  display: grid;
  gap: 8px;
  min-height: 180px;
}

.overview-grid {
  display: grid;
  grid-template-columns: minmax(420px, 0.9fr) minmax(560px, 1.1fr);
  align-items: start;
  gap: 8px;
}

.metric-card {
  min-height: 0;
  padding: 10px;
  border: 1px solid var(--ops-border-inner);
  border-radius: 10px;
  background: var(--ops-bg-surface-soft);
  box-shadow: none;
}

.metric-card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--ops-border-soft);
}

.metric-card-head strong {
  display: block;
  margin-top: 4px;
  color: var(--aiops-text-primary);
  font-size: 14px;
  font-weight: 700;
  line-height: 1.2;
}

.metric-card-eyebrow {
  display: block;
  color: var(--aiops-text-muted);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0;
}

.metric-count {
  flex-shrink: 0;
  padding: 3px 8px;
  border: 1px solid var(--ops-border-soft);
  border-radius: 999px;
  background: var(--ops-bg-surface);
  color: var(--aiops-text-muted);
  font-size: 11px;
  font-weight: 600;
}

.metric-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid var(--ops-border-soft);
  border-radius: 8px;
  background: var(--ops-bg-surface);
}

.metric-card--service .metric-list {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.metric-card--service {
  padding: 10px;
}

.metric-card--service .metric-card-head {
  margin-bottom: 8px;
  padding-bottom: 8px;
}

.metric-item {
  min-width: 0;
  padding: 10px 12px 11px;
  border-right: 1px solid var(--ops-border-soft);
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-surface);
  box-shadow: inset 3px 0 0 var(--aiops-success);
}

.metric-card--service .metric-item {
  padding: 7px 9px 8px;
}

.metric-card--host .metric-item:nth-child(2n),
.metric-card--service .metric-item:nth-child(3n) {
  border-right: none;
}

.metric-card--host .metric-item:nth-last-child(-n + 2),
.metric-card--service .metric-item:nth-last-child(-n + 3) {
  border-bottom: none;
}

.metric-item--warning {
  box-shadow: inset 3px 0 0 var(--aiops-warning);
}

.metric-item--critical {
  box-shadow: inset 3px 0 0 var(--aiops-danger);
}

.metric-item--missing {
  box-shadow: inset 3px 0 0 var(--aiops-text-muted);
}

.metric-item-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  color: var(--aiops-text-secondary);
  font-size: 11px;
}

.metric-card--service .metric-item-head {
  gap: 6px;
  font-size: 10.5px;
}

.metric-card--service .metric-item-head :deep(.el-tag) {
  height: 18px;
  padding: 0 6px;
  font-size: 10px;
}

.metric-item-head span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.metric-item strong {
  display: block;
  margin-top: 7px;
  color: var(--aiops-text-primary);
  font-size: 21px;
  line-height: 1.1;
}

.metric-card--service .metric-item strong {
  margin-top: 5px;
  font-size: 19px;
}

.metric-item small {
  margin-left: 4px;
  color: var(--aiops-text-muted);
  font-size: 12px;
  font-weight: 700;
}

.metric-item p {
  margin: 6px 0 0;
  overflow: hidden;
  color: var(--aiops-text-muted);
  font-size: 11px;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.metric-card--service .metric-item small {
  font-size: 10.5px;
}

.metric-card--service .metric-item p {
  margin-top: 4px;
  font-size: 10.5px;
  line-height: 1.25;
}

.trend-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.trend-panel,
.table-panel {
  padding: 10px;
}

.chart-box {
  width: 100%;
  height: 260px;
  margin-top: 8px;
}

.breakdown-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  padding-bottom: 10px;
}

.breakdown-table {
  margin-top: 8px;
}

.breakdown-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.breakdown-table :deep(.el-table__header th) {
  height: 36px;
  padding: 4px 0;
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-subtle);
  color: var(--ops-text-heading);
  font-size: 12px;
  font-weight: 700;
}

.breakdown-table :deep(.el-table__body td) {
  height: 36px;
  padding: 4px 0;
  color: #44546a;
  font-size: 12px;
}

.breakdown-table :deep(.cell) {
  line-height: 20px;
}

.breakdown-table :deep(.el-table__row) {
  color: var(--aiops-text-secondary);
}

.breakdown-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: #f8fafc;
}

.db-monitor-page :deep(.el-tag) {
  border-radius: 999px;
  font-weight: 600;
}

@media (max-width: 1200px) {
  .monitor-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .header-actions {
    justify-content: flex-start;
    width: 100%;
  }

  .overview-grid {
    grid-template-columns: 1fr;
  }

  .metric-card--host .metric-list,
  .metric-card--service .metric-list {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .metric-card--host .metric-item:nth-child(2n) {
    border-right: 1px solid var(--ops-border-soft);
  }

  .metric-card--host .metric-item:nth-child(3n) {
    border-right: none;
  }

  .metric-card--host .metric-item:nth-last-child(-n + 2) {
    border-bottom: 1px solid var(--ops-border-soft);
  }

  .metric-card--host .metric-item:nth-last-child(-n + 3) {
    border-bottom: none;
  }

  .target-summary {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 960px) {
  .monitor-shell {
    height: auto;
    min-height: 100%;
    overflow: visible;
  }

  .asset-select {
    width: 100%;
  }

  .header-actions {
    align-items: stretch;
    flex-direction: column;
  }

  .header-actions :deep(.el-segmented),
  .step-select,
  .header-actions :deep(.el-button) {
    width: 100%;
  }

  .status-grid,
  .trend-grid,
  .breakdown-grid {
    grid-template-columns: 1fr;
  }

  .status-grid,
  .dashboard-content,
  .overview-grid,
  .trend-grid,
  .breakdown-grid,
  .warning-stack {
    margin: 0 10px;
  }

  .overview-grid {
    grid-template-columns: 1fr;
  }

  .metric-card--host .metric-list,
  .metric-card--service .metric-list {
    grid-template-columns: 1fr;
  }

  .metric-card--host .metric-item,
  .metric-card--service .metric-item {
    border-right: none;
  }

  .metric-card--host .metric-item:nth-last-child(-n + 3),
  .metric-card--service .metric-item:nth-last-child(-n + 3) {
    border-bottom: 1px solid var(--ops-border-soft);
  }

  .metric-card--host .metric-item:last-child,
  .metric-card--service .metric-item:last-child {
    border-bottom: none;
  }
}

@media (max-width: 768px) {
  .ops-page-header__content {
    align-items: flex-start;
    flex-direction: column;
    gap: 2px;
  }

  .ops-page-header__divider {
    display: none;
  }

  .target-summary-item {
    align-items: flex-start;
    flex-direction: column;
  }
}

@media (max-width: 620px) {
  .db-monitor-page {
    padding: 6px;
  }

  .monitor-header {
    padding: 8px 10px;
  }

  .ops-page-header__icon {
    width: 36px;
    height: 36px;
    flex-basis: 36px;
  }

  .ops-page-header__icon img {
    width: 27px;
    height: 27px;
  }

  .status-grid,
  .dashboard-content,
  .overview-grid,
  .trend-grid,
  .breakdown-grid,
  .warning-stack {
    margin-right: 8px;
    margin-left: 8px;
  }

  .chart-box {
    height: 220px;
  }
}
</style>
