<template>
  <div class="log-analysis-page">
    <section class="log-analysis-shell">
      <header class="analysis-header">
        <div class="header-main-row">
          <div class="header-identity">
            <el-button class="icon-command icon-command--plain" text aria-label="返回业务大盘" @click="goDashboard">
              <el-icon><ArrowLeft /></el-icon>
            </el-button>
            <div class="business-heading">
              <div class="business-path">
                <span>业务系统</span>
                <i>/</i>
                <strong>{{ businessTitle }}</strong>
                <em class="environment-badge">{{ summary.env || '默认' }}</em>
                <em :class="['health-badge', statusClass(overallStatus)]">{{ statusText(overallStatus) }}</em>
              </div>
              <p>{{ conclusion }}</p>
            </div>
          </div>

          <div class="header-actions">
            <span class="generated-time">
              <el-icon><Calendar /></el-icon>
              {{ formatDateTime(analysis.generatedAt) }}
            </span>
            <el-select v-model="filters.window" class="window-select" aria-label="日志分析时间窗口" @change="loadAnalysis">
              <el-option v-for="item in windowOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
            <el-tooltip content="刷新" placement="bottom">
              <el-button class="icon-command" :loading="loading" aria-label="刷新日志分析" @click="loadAnalysis">
                <el-icon><Refresh /></el-icon>
              </el-button>
            </el-tooltip>
            <el-tooltip content="分享当前视图" placement="bottom">
              <el-button class="share-command" aria-label="分享当前视图" @click="sharePage">
                <el-icon><Share /></el-icon><span>分享</span>
              </el-button>
            </el-tooltip>
          </div>
        </div>

        <nav class="detail-tabs" aria-label="业务详情视图">
          <button type="button" @click="goDashboard">业务大盘</button>
          <button type="button" @click="goTopology">编辑拓扑图</button>
          <button type="button" class="is-active" aria-current="page">日志分析</button>
          <button type="button" @click="goMonitor">监控告警</button>
        </nav>
      </header>

      <div v-if="warningMessages.length" class="degradation-banner" role="alert">
        <el-icon><Warning /></el-icon>
        <div>
          <strong>{{ analysis.partial ? '部分日志数据暂不可用' : '日志源提示' }}</strong>
          <span>{{ warningMessages.join('；') }}</span>
        </div>
        <button type="button" @click="loadAnalysis">重试</button>
      </div>

      <div v-if="loadError" class="load-error" role="alert">
        <el-icon><Warning /></el-icon>
        <div><strong>业务日志分析加载失败</strong><span>{{ loadError }}</span></div>
        <el-button type="primary" @click="loadAnalysis">重新加载</el-button>
      </div>

      <main v-else v-loading="loading" class="analysis-grid">
        <aside class="source-panel">
          <div class="panel-heading">
            <div><h3>入口日志源</h3><span>{{ sourceBreakdown.length }} 个来源</span></div>
          </div>
          <div class="source-switch" role="group" aria-label="日志来源筛选">
            <button type="button" :class="{ active: filters.sourceType === 'all' }" @click="setSourceType('all')">全部</button>
            <button type="button" :class="{ active: filters.sourceType === 'nginx' }" @click="setSourceType('nginx')">Nginx</button>
            <button type="button" :class="{ active: filters.sourceType === 'gateway' }" @click="setSourceType('gateway')">Gateway</button>
          </div>
          <div class="source-list">
            <button
              v-for="source in sourceBreakdown"
              :key="`${source.storeId}-${source.storeCode}`"
              type="button"
              :class="{ active: selectedSourceKey === sourceKey(source) }"
              @click="selectSource(source)"
            >
              <i :class="sourceTone(source)"></i>
              <span><strong>{{ sourceTitle(source) }}</strong><small>{{ source.storeName || source.storeCode || '-' }}</small></span>
              <em>{{ formatCompact(source.requestTotal) }}</em>
            </button>
            <div v-if="!sourceBreakdown.length" class="compact-empty">当前业务没有 Nginx / Gateway 日志源</div>
          </div>
        </aside>

        <section class="filter-panel">
          <el-select v-model="filters.domain" clearable filterable placeholder="域名" @change="loadAnalysis">
            <el-option v-for="item in domainOptions" :key="item" :label="item" :value="item" />
          </el-select>
          <el-select v-model="filters.statusClass" clearable placeholder="状态码" @change="loadAnalysis">
            <el-option label="全部状态" value="" />
            <el-option label="2xx 成功" value="2xx" />
            <el-option label="4xx 客户端错误" value="4xx" />
            <el-option label="5xx 服务端错误" value="5xx" />
            <el-option v-for="item in statusOptions" :key="item" :label="item" :value="item" />
          </el-select>
          <el-select v-model="filters.requestPath" clearable filterable placeholder="接口路径" @change="loadAnalysis">
            <el-option v-for="item in apiPathOptions" :key="item" :label="item" :value="item" />
          </el-select>
          <el-select v-model="filters.clientIp" clearable filterable placeholder="客户端 IP" @change="loadAnalysis">
            <el-option v-for="item in clientOptions" :key="item" :label="item" :value="item" />
          </el-select>
          <el-button class="query-command" type="primary" :icon="Search" @click="loadAnalysis">查询</el-button>
          <el-button class="reset-command" :icon="RefreshLeft" @click="resetFilters">重置</el-button>
        </section>

        <section class="kpi-panel" aria-label="日志分析指标">
          <article v-for="item in kpiItems" :key="item.key">
            <span class="kpi-label">{{ item.label }}</span>
            <strong :class="item.tone">{{ item.value }}</strong>
            <small>{{ item.caption }}</small>
          </article>
        </section>

        <section class="conclusion-panel">
          <div :class="['conclusion-status', statusClass(overallStatus)]">
            <el-icon><component :is="overallStatus === 'healthy' ? CircleCheckFilled : WarningFilled" /></el-icon>
          </div>
          <div>
            <h3>{{ conclusionTitle }}</h3>
            <p>{{ conclusion }}</p>
          </div>
          <dl>
            <div><dt>5xx</dt><dd :class="{ danger: Number(metrics.http5xx) > 0 }">{{ formatCompact(metrics.http5xx) }}</dd></div>
            <div><dt>P95</dt><dd :class="{ danger: Number(metrics.p95LatencyMs) >= 1000 }">{{ formatLatency(metrics.p95LatencyMs) }}</dd></div>
            <div><dt>慢接口</dt><dd :class="{ danger: Number(metrics.slowRequestCount) > 0 }">{{ formatCompact(metrics.slowRequestCount) }}</dd></div>
          </dl>
        </section>

        <section class="chart-card chart-card--trend">
          <div class="panel-heading">
            <div><h3>请求状态趋势</h3><span>200 / 4xx / 5xx</span></div>
          </div>
          <div ref="trendChartRef" class="chart"></div>
        </section>

        <section class="chart-card chart-card--status">
          <div class="panel-heading">
            <div><h3>状态码分布</h3><span>{{ statuses.length }} 类</span></div>
          </div>
          <div ref="statusChartRef" class="chart"></div>
        </section>

        <section class="chart-card chart-card--domain">
          <div class="panel-heading">
            <div><h3>域名请求分布</h3><span>{{ domains.length }} 个域名</span></div>
          </div>
          <div ref="domainChartRef" class="chart"></div>
        </section>

        <section class="chart-card chart-card--slow">
          <div class="panel-heading">
            <div><h3>慢接口 Top5</h3><span>P95 响应时间</span></div>
          </div>
          <div ref="slowApiChartRef" class="chart"></div>
        </section>

        <section class="chart-card chart-card--client">
          <div class="panel-heading">
            <div><h3>客户端 Top10</h3><span>请求量</span></div>
          </div>
          <div ref="clientChartRef" class="chart"></div>
        </section>

        <section class="api-panel">
          <div class="panel-heading">
            <div><h3>接口分析</h3><span>{{ topApis.length }} 个接口</span></div>
          </div>
          <el-table :data="topApis.slice(0, 12)" height="196" size="small" class="analysis-table" empty-text="当前窗口没有接口数据" @row-click="filterByApi">
            <el-table-column label="方法" width="72"><template #default="scope"><span class="method-tag">{{ scope.row.requestMethod || '-' }}</span></template></el-table-column>
            <el-table-column label="接口" min-width="280" show-overflow-tooltip><template #default="scope">{{ scope.row.requestPath || '-' }}</template></el-table-column>
            <el-table-column label="PV" width="86" align="right"><template #default="scope">{{ formatCompact(scope.row.requestTotal) }}</template></el-table-column>
            <el-table-column label="UV" width="76" align="right"><template #default="scope">{{ formatCompact(scope.row.uniqueClients) }}</template></el-table-column>
            <el-table-column label="平均" width="86" align="right"><template #default="scope">{{ formatLatency(scope.row.avgLatencyMs) }}</template></el-table-column>
            <el-table-column label="P95" width="86" align="right"><template #default="scope">{{ formatLatency(scope.row.p95LatencyMs) }}</template></el-table-column>
            <el-table-column label="4xx" width="72" align="right"><template #default="scope"><span :class="{ danger: Number(scope.row.http4xx) > 0 }">{{ formatCompact(scope.row.http4xx) }}</span></template></el-table-column>
            <el-table-column label="5xx" width="72" align="right"><template #default="scope"><span :class="{ danger: Number(scope.row.http5xx) > 0 }">{{ formatCompact(scope.row.http5xx) }}</span></template></el-table-column>
            <el-table-column label="流量" width="92" align="right"><template #default="scope">{{ formatBytes(scope.row.responseBytes) }}</template></el-table-column>
          </el-table>
        </section>

        <section class="error-panel">
          <div class="panel-heading">
            <div><h3>最近异常请求</h3><span>{{ recentErrors.length }} 条</span></div>
          </div>
          <el-table :data="recentErrors.slice(0, 12)" height="196" size="small" class="analysis-table" empty-text="当前窗口没有 4xx / 5xx 请求">
            <el-table-column label="时间" width="138"><template #default="scope">{{ formatDateTime(scope.row.timestamp) }}</template></el-table-column>
            <el-table-column label="域名" min-width="170" show-overflow-tooltip><template #default="scope">{{ scope.row.domain || '-' }}</template></el-table-column>
            <el-table-column label="接口" min-width="260" show-overflow-tooltip><template #default="scope">{{ scope.row.requestPath || '-' }}</template></el-table-column>
            <el-table-column label="状态" width="72"><template #default="scope"><span class="status-code">{{ scope.row.status || '-' }}</span></template></el-table-column>
            <el-table-column label="响应" width="82" align="right"><template #default="scope">{{ formatLatency(scope.row.latencyMs) }}</template></el-table-column>
            <el-table-column label="客户端" width="126" show-overflow-tooltip><template #default="scope">{{ scope.row.clientIp || '-' }}</template></el-table-column>
            <el-table-column label="上游" width="142" show-overflow-tooltip><template #default="scope">{{ scope.row.upstreamAddr || '-' }}</template></el-table-column>
            <el-table-column label="RequestID" width="170" show-overflow-tooltip><template #default="scope">{{ scope.row.requestId || '-' }}</template></el-table-column>
            <el-table-column label="TraceID" width="170" show-overflow-tooltip><template #default="scope">{{ scope.row.traceId || '-' }}</template></el-table-column>
          </el-table>
        </section>
      </main>
    </section>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import * as echarts from 'echarts'
import { ArrowLeft, Calendar, CircleCheckFilled, Refresh, RefreshLeft, Search, Share, Warning, WarningFilled } from '@element-plus/icons-vue'
import { getBusinessLogAnalysis } from '@/api/cmdbBusinessModel'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const loadError = ref('')
const analysis = ref({
  summary: {}, metrics: {}, derived: {}, trafficTrend: [], domains: [], statuses: [], topClients: [], topApis: [], sourceBreakdown: [], recentErrors: [], warnings: [], window: {}
})
const filters = ref({
  window: String(route.query.window || '15m'),
  sourceType: String(route.query.sourceType || 'all'),
  storeId: String(route.query.storeId || ''),
  domain: String(route.query.domain || ''),
  statusClass: String(route.query.statusClass || ''),
  requestPath: String(route.query.requestPath || ''),
  clientIp: String(route.query.clientIp || '')
})
const selectedSourceKey = ref(String(route.query.storeId || ''))

const trendChartRef = ref(null)
const statusChartRef = ref(null)
const domainChartRef = ref(null)
const slowApiChartRef = ref(null)
const clientChartRef = ref(null)
const charts = new Map()
let resizeObserver = null

const windowOptions = [
  { label: '近 15 分钟', value: '15m' },
  { label: '近 30 分钟', value: '30m' },
  { label: '近 1 小时', value: '1h' },
  { label: '近 6 小时', value: '6h' },
  { label: '近 12 小时', value: '12h' },
  { label: '近 24 小时', value: '24h' },
  { label: '近 7 天', value: '7d' },
  { label: '近 30 天', value: '30d' }
]

const businessId = computed(() => route.params.id)
const summary = computed(() => analysis.value.summary || {})
const metrics = computed(() => analysis.value.metrics || {})
const domains = computed(() => analysis.value.domains || [])
const statuses = computed(() => analysis.value.statuses || [])
const topClients = computed(() => analysis.value.topClients || [])
const topApis = computed(() => analysis.value.topApis || [])
const sourceBreakdown = computed(() => analysis.value.sourceBreakdown || [])
const recentErrors = computed(() => analysis.value.recentErrors || [])
const businessTitle = computed(() => summary.value.displayName || summary.value.name || `业务 ${businessId.value}`)
const overallStatus = computed(() => {
  if (Number(metrics.value.http5xx) > 0 || Number(metrics.value.systemErrorRate) >= 1) return 'critical'
  if (Number(metrics.value.http4xx) > 0 || Number(metrics.value.slowRequestCount) > 0 || Number(metrics.value.p95LatencyMs) >= 1000) return 'warning'
  return normalizeStatus(summary.value.overallStatus || summary.value.healthStatus || 'healthy')
})
const warningMessages = computed(() => (analysis.value.warnings || []).map(item => typeof item === 'string' ? item : item?.message).filter(Boolean))
const domainOptions = computed(() => domains.value.map(item => item.domain).filter(Boolean))
const statusOptions = computed(() => statuses.value.map(item => String(item.status)).filter(Boolean))
const apiPathOptions = computed(() => topApis.value.map(item => item.requestPath).filter(Boolean))
const clientOptions = computed(() => topClients.value.map(item => item.clientIp).filter(Boolean))

const conclusionTitle = computed(() => {
  if (overallStatus.value === 'critical') return '入口请求存在服务端异常'
  if (overallStatus.value === 'warning') return '入口请求存在质量风险'
  return '入口请求整体正常'
})
const conclusion = computed(() => {
  const requestText = `${formatCompact(metrics.value.requestTotal)} 次请求`
  const errorText = `${formatPercent(metrics.value.errorRate)} 错误率`
  const slowApi = [...topApis.value].sort((a, b) => Number(b.p95LatencyMs || 0) - Number(a.p95LatencyMs || 0))[0]
  const status = statuses.value[0]
  const domain = domains.value[0]
  if (!Number(metrics.value.requestTotal)) return '当前时间窗口没有入口请求数据，优先确认 Nginx / Gateway 日志源是否已绑定并有日志写入。'
  if (Number(metrics.value.http5xx) > 0) return `最近窗口 ${requestText}，5xx ${formatCompact(metrics.value.http5xx)} 次，最高频状态 ${status?.status || '-'}，优先检查 ${slowApi?.requestPath || domain?.domain || '入口服务'}。`
  if (Number(metrics.value.http4xx) > 0) return `最近窗口 ${requestText}，4xx ${formatCompact(metrics.value.http4xx)} 次，主要影响 ${domain?.domain || '入口域名'}，可结合客户端 IP 和接口分布判断是否为调用方问题。`
  if (Number(metrics.value.slowRequestCount) > 0) return `最近窗口 ${requestText}，慢请求 ${formatCompact(metrics.value.slowRequestCount)} 次，P95 ${formatLatency(metrics.value.p95LatencyMs)}，重点关注 ${slowApi?.requestPath || '慢接口 Top5'}。`
  return `最近窗口 ${requestText}，${errorText}，平均响应 ${formatLatency(metrics.value.avgLatencyMs)}，入口日志未发现明显异常。`
})
const kpiItems = computed(() => [
  { key: 'requests', label: '请求量', value: formatCompact(metrics.value.requestTotal), caption: `${formatQps(analysis.value.derived?.averageQps)} QPS` },
  { key: 'availability', label: '可用性', value: formatPercent(metrics.value.availabilityRate || metrics.value.successRate), caption: `目标 ${formatPercent(metrics.value.sloTarget || 99.9)}`, tone: metrics.value.sloMet === false ? 'danger' : 'healthy' },
  { key: 'latency', label: '平均响应', value: formatLatency(metrics.value.avgLatencyMs), caption: `P95 ${formatLatency(metrics.value.p95LatencyMs)}`, tone: Number(metrics.value.p95LatencyMs) >= 1000 ? 'danger' : '' },
  { key: 'clients', label: '独立客户端', value: formatCompact(metrics.value.uniqueClientCount), caption: `${formatBytes(metrics.value.responseBytes)} 响应流量` },
  { key: 'http4xx', label: '4xx', value: formatCompact(metrics.value.http4xx), caption: formatPercent(metrics.value.http4xxRate), tone: Number(metrics.value.http4xx) > 0 ? 'warning' : '' },
  { key: 'http5xx', label: '5xx', value: formatCompact(metrics.value.http5xx), caption: formatPercent(metrics.value.http5xxRate), tone: Number(metrics.value.http5xx) > 0 ? 'danger' : '' }
])

const normalizeResponse = response => response?.data?.code !== undefined ? response.data : response
const normalizeStatus = value => {
  const status = String(value || '').toLowerCase()
  if (['healthy', 'normal', 'ok', 'success'].includes(status)) return 'healthy'
  if (['warning', 'warn', 'degraded'].includes(status)) return 'warning'
  if (['critical', 'error', 'failed', 'fatal'].includes(status)) return 'critical'
  return 'unknown'
}
const statusText = status => ({ healthy: '正常', warning: '风险', critical: '异常', unknown: '未接入' })[normalizeStatus(status)]
const statusClass = status => `status-${normalizeStatus(status)}`
const toNumber = value => Number.isFinite(Number(value)) ? Number(value) : 0
const formatCompact = value => new Intl.NumberFormat('zh-CN', { notation: Math.abs(toNumber(value)) >= 10000 ? 'compact' : 'standard', maximumFractionDigits: 1 }).format(toNumber(value))
const formatPercent = value => `${toNumber(value).toFixed(toNumber(value) >= 99 ? 2 : 1)}%`
const formatQps = value => toNumber(value).toFixed(toNumber(value) >= 10 ? 1 : 2)
const formatLatency = value => {
  const ms = toNumber(value)
  if (!ms) return '0ms'
  return ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${ms.toFixed(ms >= 10 ? 0 : 1)}ms`
}
const formatBytes = value => {
  const bytes = toNumber(value)
  if (!bytes) return '0B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = bytes
  let index = 0
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024
    index += 1
  }
  return `${size.toFixed(size >= 10 || index === 0 ? 0 : 1)}${units[index]}`
}
const formatDateTime = value => {
  if (!value) return '--'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '--' : date.toLocaleString('zh-CN', { hour12: false })
}
const sourceKey = source => String(source.storeId || '')
const sourceTitle = source => {
  const names = Array.isArray(source.nodeNames) ? source.nodeNames.filter(Boolean) : []
  return names.join('、') || source.storeCode || source.storeName || '日志源'
}
const sourceTone = source => Number(source.http5xx) > 0 ? 'critical' : (Number(source.http4xx) > 0 || Number(source.p95LatencyMs) >= 1000 ? 'warning' : 'healthy')

const queryParams = () => {
  const params = { window: filters.value.window, sourceType: filters.value.sourceType }
  ;['storeId', 'domain', 'statusClass', 'requestPath', 'clientIp'].forEach(key => {
    if (String(filters.value[key] || '').trim()) params[key] = String(filters.value[key]).trim()
  })
  return params
}
const syncRouteQuery = () => {
  router.replace({ path: route.path, query: queryParams() }).catch(() => {})
}
const loadAnalysis = async () => {
  loading.value = true
  loadError.value = ''
  try {
    const res = normalizeResponse(await getBusinessLogAnalysis(businessId.value, queryParams()))
    if (![0, 200].includes(res?.code)) throw new Error(res?.message || '日志分析加载失败')
    analysis.value = { ...analysis.value, ...(res.data || {}) }
    syncRouteQuery()
    await nextTick()
    renderCharts()
    setupResizeObserver()
  } catch (error) {
    console.error('业务日志分析加载失败:', error)
    loadError.value = error.message || '业务日志分析加载失败'
  } finally {
    loading.value = false
  }
}
const setSourceType = value => {
  filters.value.sourceType = value
  filters.value.storeId = ''
  selectedSourceKey.value = ''
  loadAnalysis()
}
const selectSource = source => {
  const key = sourceKey(source)
  const active = selectedSourceKey.value === key
  selectedSourceKey.value = active ? '' : key
  filters.value.storeId = active ? '' : key
  loadAnalysis()
}
const filterByApi = row => {
  if (!row?.requestPath) return
  filters.value.requestPath = row.requestPath
  loadAnalysis()
}
const resetFilters = () => {
  filters.value = { window: filters.value.window, sourceType: 'all', storeId: '', domain: '', statusClass: '', requestPath: '', clientIp: '' }
  selectedSourceKey.value = ''
  loadAnalysis()
}
const goDashboard = () => router.push({ path: `/cmdb/service_model/${businessId.value}`, query: { window: filters.value.window } })
const goTopology = () => router.push({ path: `/cmdb/service_model/${businessId.value}/topology` })
const goMonitor = () => router.push({ path: `/cmdb/service_model/${businessId.value}/monitor-alerts`, query: { window: filters.value.window } })
const sharePage = async () => {
  try {
    await navigator.clipboard.writeText(window.location.href)
    ElMessage.success('当前日志分析链接已复制')
  } catch (error) {
    ElMessage.error('复制链接失败')
  }
}

const getChart = (key, el) => {
  if (!el) return null
  const current = charts.get(key)
  if (current && !current.isDisposed()) return current
  const chart = echarts.init(el, null, { renderer: 'canvas' })
  charts.set(key, chart)
  return chart
}
const emptyGraphic = text => ({ type: 'text', left: 'center', top: 'middle', style: { text, fill: '#94a3b8', fontSize: 12 } })
const chartBase = hasData => ({
  animation: false,
  color: ['#2563eb', '#d97706', '#dc2626', '#16a34a', '#64748b'],
  grid: { top: 28, right: 18, bottom: 28, left: 42, containLabel: true },
  tooltip: { trigger: 'axis', confine: true },
  graphic: hasData ? null : emptyGraphic('暂无数据')
})
const trendTime = item => item.timestamp || item.time || item.bucket || item.window || ''
const trendValue = (item, keys) => keys.reduce((sum, key) => sum + toNumber(item[key]), 0)
const renderTrendChart = () => {
  const rows = analysis.value.trafficTrend || []
  const chart = getChart('trend', trendChartRef.value)
  if (!chart) return
  chart.setOption({
    ...chartBase(rows.length),
    legend: { top: 0, right: 8, itemWidth: 10, itemHeight: 6, textStyle: { color: '#64748b', fontSize: 10 } },
    xAxis: { type: 'category', data: rows.map(trendTime), axisLabel: { color: '#7b8797', fontSize: 10 }, axisLine: { lineStyle: { color: '#dce3ed' } } },
    yAxis: { type: 'value', axisLabel: { color: '#7b8797', fontSize: 10 }, splitLine: { lineStyle: { color: '#edf1f6' } } },
    series: [
      { name: '200', type: 'line', smooth: true, symbol: 'none', areaStyle: { opacity: 0.08 }, data: rows.map(item => trendValue(item, ['http200', 'status200', 'successCount', 'success'])) },
      { name: '4xx', type: 'line', smooth: true, symbol: 'none', data: rows.map(item => trendValue(item, ['http4xx', 'status4xx'])) },
      { name: '5xx', type: 'line', smooth: true, symbol: 'none', data: rows.map(item => trendValue(item, ['http5xx', 'status5xx'])) }
    ]
  }, true)
}
const renderStatusChart = () => {
  const rows = statuses.value.slice(0, 10)
  const chart = getChart('status', statusChartRef.value)
  if (!chart) return
  chart.setOption({
    animation: false,
    color: ['#16a34a', '#2563eb', '#d97706', '#dc2626', '#94a3b8'],
    tooltip: { trigger: 'item', confine: true },
    legend: { bottom: 0, itemWidth: 10, itemHeight: 6, textStyle: { color: '#64748b', fontSize: 10 } },
    graphic: rows.length ? null : emptyGraphic('暂无数据'),
    series: [{ type: 'pie', radius: ['46%', '70%'], center: ['50%', '45%'], label: { formatter: '{b}', color: '#475569', fontSize: 10 }, data: rows.map(item => ({ name: String(item.status), value: toNumber(item.requestTotal) })) }]
  }, true)
}
const renderDomainChart = () => {
  const rows = domains.value.slice(0, 8)
  const chart = getChart('domain', domainChartRef.value)
  if (!chart) return
  chart.setOption({
    ...chartBase(rows.length),
    xAxis: { type: 'value', axisLabel: { color: '#7b8797', fontSize: 10 }, splitLine: { lineStyle: { color: '#edf1f6' } } },
    yAxis: { type: 'category', data: rows.map(item => item.domain), axisLabel: { color: '#475569', fontSize: 10, width: 120, overflow: 'truncate' }, axisLine: { show: false } },
    series: [{ type: 'bar', barWidth: 10, data: rows.map(item => toNumber(item.requestTotal)), itemStyle: { borderRadius: [0, 3, 3, 0], color: '#2563eb' } }]
  }, true)
}
const renderSlowApiChart = () => {
  const rows = [...topApis.value].sort((a, b) => toNumber(b.p95LatencyMs) - toNumber(a.p95LatencyMs)).slice(0, 5)
  const chart = getChart('slowApi', slowApiChartRef.value)
  if (!chart) return
  chart.setOption({
    ...chartBase(rows.length),
    xAxis: { type: 'category', data: rows.map(item => item.requestPath || '-'), axisLabel: { color: '#7b8797', fontSize: 10, width: 72, overflow: 'truncate', interval: 0 }, axisLine: { lineStyle: { color: '#dce3ed' } } },
    yAxis: { type: 'value', axisLabel: { color: '#7b8797', fontSize: 10, formatter: value => formatLatency(value) }, splitLine: { lineStyle: { color: '#edf1f6' } } },
    series: [{ type: 'bar', barWidth: 18, data: rows.map(item => toNumber(item.p95LatencyMs)), itemStyle: { borderRadius: [3, 3, 0, 0], color: '#d97706' } }]
  }, true)
}
const renderClientChart = () => {
  const rows = topClients.value.slice(0, 10)
  const chart = getChart('client', clientChartRef.value)
  if (!chart) return
  chart.setOption({
    ...chartBase(rows.length),
    xAxis: { type: 'value', axisLabel: { color: '#7b8797', fontSize: 10 }, splitLine: { lineStyle: { color: '#edf1f6' } } },
    yAxis: { type: 'category', data: rows.map(item => item.clientIp), axisLabel: { color: '#475569', fontSize: 10 }, axisLine: { show: false } },
    series: [{ type: 'bar', barWidth: 10, data: rows.map(item => toNumber(item.requestTotal)), itemStyle: { borderRadius: [0, 3, 3, 0], color: '#64748b' } }]
  }, true)
}
const renderCharts = () => {
  renderTrendChart()
  renderStatusChart()
  renderDomainChart()
  renderSlowApiChart()
  renderClientChart()
}
const setupResizeObserver = () => {
  resizeObserver?.disconnect()
  resizeObserver = new ResizeObserver(() => charts.forEach(chart => chart?.resize()))
  ;[trendChartRef, statusChartRef, domainChartRef, slowApiChartRef, clientChartRef].forEach(item => {
    if (item.value) resizeObserver.observe(item.value)
  })
}
const disposeCharts = () => {
  resizeObserver?.disconnect()
  charts.forEach(chart => chart?.dispose())
  charts.clear()
}

watch(() => route.params.id, () => loadAnalysis())
onMounted(loadAnalysis)
onBeforeUnmount(disposeCharts)
</script>

<style scoped>
.log-analysis-page {
  --surface: #ffffff;
  --surface-subtle: #f7f9fc;
  --page-bg: #eef2f7;
  --border: #dce3ed;
  --border-strong: #cbd5e2;
  --text: #182235;
  --text-secondary: #5f6d82;
  --text-muted: #8995a7;
  --blue: #2563eb;
  --blue-soft: #edf4ff;
  width: 100%;
  height: 100%;
  min-height: 0;
  padding: 8px;
  overflow: auto;
  box-sizing: border-box;
  color: var(--text);
  background: var(--page-bg);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}
:global(.home-container .el-main:has(.log-analysis-page)) { padding: 0; overflow: hidden; }
.log-analysis-shell {
  display: flex;
  flex-direction: column;
  width: 100%;
  min-width: 1180px;
  min-height: 0;
  height: 100%;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--surface);
}
.analysis-header {
  flex: none;
  padding: 0 18px;
  border-bottom: 1px solid var(--border);
  background: #ffffff;
}
.header-main-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 58px;
  gap: 24px;
}
.header-identity,
.header-actions,
.business-path,
.detail-tabs,
.panel-heading {
  display: flex;
  align-items: center;
}
.header-identity { min-width: 0; gap: 8px; }
.business-heading { min-width: 0; }
.business-path { gap: 9px; min-width: 0; white-space: nowrap; }
.business-path span,
.business-path i {
  color: var(--text-secondary);
  font-size: 13px;
  font-style: normal;
}
.business-path i { color: #b0bac8; }
.business-path strong {
  max-width: 320px;
  overflow: hidden;
  font-size: 17px;
  text-overflow: ellipsis;
}
.business-heading p {
  max-width: 720px;
  margin: 4px 0 0;
  overflow: hidden;
  color: var(--text-secondary);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.environment-badge,
.health-badge {
  display: inline-flex;
  align-items: center;
  min-height: 20px;
  padding: 0 7px;
  border-radius: 4px;
  font-size: 10px;
  font-style: normal;
  font-weight: 700;
}
.environment-badge { color: #087a55; border: 1px solid #bce9da; background: #e9f8f2; }
.health-badge.status-healthy { color: #087a55; background: #e9f8f2; }
.health-badge.status-warning { color: #a15c00; background: #fff3d8; }
.health-badge.status-critical { color: #c52828; background: #fff0f0; }
.health-badge.status-unknown { color: #657287; background: #eef2f6; }
.header-actions {
  justify-content: flex-end;
  gap: 8px;
  white-space: nowrap;
}
.generated-time {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--text-muted);
  font-size: 11px;
}
.window-select { width: 126px; }
.icon-command,
.share-command {
  min-height: 34px;
  border-radius: 5px;
  border-color: var(--border);
  color: #526177;
  background: #ffffff;
}
.icon-command { width: 34px; padding: 0; }
.icon-command--plain { border: 0; background: transparent; }
.share-command { display: inline-flex; gap: 5px; padding: 0 11px; }
.icon-command:hover,
.share-command:hover { color: var(--blue); border-color: #9db9ef; background: var(--blue-soft); }
.detail-tabs {
  height: 36px;
  gap: 28px;
  padding-left: 43px;
}
.detail-tabs button {
  position: relative;
  align-self: stretch;
  padding: 0 2px;
  border: 0;
  color: #657287;
  background: transparent;
  cursor: pointer;
  font-size: 13px;
}
.detail-tabs button:hover,
.detail-tabs button.is-active { color: var(--blue); }
.detail-tabs button.is-active { font-weight: 700; }
.detail-tabs button.is-active::after {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  height: 2px;
  background: var(--blue);
  content: "";
}
.degradation-banner,
.load-error {
  display: flex;
  flex: none;
  align-items: center;
  gap: 10px;
  min-height: 38px;
  margin: 8px 10px 0;
  padding: 5px 10px;
  box-sizing: border-box;
  border: 1px solid #edc36a;
  border-radius: 5px;
  color: #8b5700;
  background: #fff9e9;
}
.degradation-banner > div,
.load-error > div { display: flex; flex: 1; min-width: 0; flex-direction: column; }
.degradation-banner strong,
.load-error strong { font-size: 12px; }
.degradation-banner span,
.load-error span { overflow: hidden; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.degradation-banner button { border: 0; color: inherit; background: transparent; cursor: pointer; font-weight: 700; }
.load-error { margin: auto; max-width: 660px; border-color: #efb0b0; color: #a92323; background: #fff3f3; }
.analysis-grid {
  display: grid;
  grid-template-columns: 230px repeat(6, minmax(0, 1fr));
  grid-template-rows: 42px 76px 70px minmax(156px, 1fr) 238px;
  flex: 1;
  min-height: 0;
  gap: 6px;
  padding: 6px;
  box-sizing: border-box;
  background: var(--page-bg);
}
.source-panel,
.filter-panel,
.kpi-panel,
.conclusion-panel,
.chart-card,
.api-panel,
.error-panel {
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: #ffffff;
}
.source-panel {
  display: flex;
  grid-column: 1;
  grid-row: 1 / 6;
  flex-direction: column;
  padding: 12px 10px 8px;
}
.panel-heading {
  min-height: 42px;
  justify-content: space-between;
  gap: 10px;
  padding: 0 11px;
  border-bottom: 1px solid var(--border);
}
.source-panel .panel-heading { min-height: 34px; padding: 0 3px 8px; border-bottom: 0; }
.panel-heading h3 { margin: 0; font-size: 14px; }
.panel-heading span { color: var(--text-muted); font-size: 10px; }
.source-switch {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  padding: 2px;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: #f3f6fa;
}
.source-switch button {
  min-height: 28px;
  border: 0;
  border-radius: 4px;
  color: var(--text-secondary);
  background: transparent;
  cursor: pointer;
  font-size: 11px;
}
.source-switch button.active { color: var(--blue); background: #ffffff; box-shadow: 0 1px 3px rgba(34, 67, 113, 0.12); font-weight: 700; }
.source-list { flex: 1; min-height: 0; margin-top: 10px; overflow-y: auto; scrollbar-width: thin; }
.source-list button {
  display: grid;
  grid-template-columns: 8px minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 50px;
  padding: 6px 7px;
  border: 0;
  border-radius: 4px;
  background: transparent;
  cursor: pointer;
  text-align: left;
}
.source-list button:hover,
.source-list button.active { background: #edf4ff; }
.source-list i { width: 6px; height: 6px; border-radius: 50%; }
.source-list i.healthy { background: #19a66f; }
.source-list i.warning { background: #df8500; }
.source-list i.critical { background: #df3e3e; }
.source-list span { display: flex; min-width: 0; flex-direction: column; }
.source-list strong { overflow: hidden; font-size: 11.5px; text-overflow: ellipsis; white-space: nowrap; }
.source-list small { margin-top: 3px; color: var(--text-muted); font-size: 9.5px; }
.source-list em { color: var(--text-secondary); font-size: 10px; font-style: normal; }
.filter-panel {
  display: grid;
  grid-column: 2 / 8;
  grid-row: 1;
  grid-template-columns: 1fr 150px 1.2fr 160px 82px 82px;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
}
:deep(.filter-panel .el-input__wrapper),
:deep(.filter-panel .el-select__wrapper) { min-height: 32px; border-radius: 5px; box-shadow: 0 0 0 1px var(--border) inset; }
.query-command,
.reset-command { min-height: 32px; border-radius: 5px; }
.kpi-panel {
  display: grid;
  grid-column: 2 / 8;
  grid-row: 2;
  grid-template-columns: repeat(6, minmax(0, 1fr));
}
.kpi-panel article {
  display: flex;
  min-width: 0;
  padding: 10px 14px;
  border-right: 1px solid var(--border);
  flex-direction: column;
  justify-content: center;
}
.kpi-panel article:last-child { border-right: 0; }
.kpi-label { color: var(--text-secondary); font-size: 10px; }
.kpi-panel strong { margin: 3px 0; font-family: "Segoe UI", "PingFang SC", sans-serif; font-size: 21px; font-variant-numeric: tabular-nums; line-height: 1.05; }
.kpi-panel small { overflow: hidden; color: var(--text-muted); font-size: 9.5px; text-overflow: ellipsis; white-space: nowrap; }
.healthy { color: #0c8c5d; }
.warning { color: #b96a00; }
.danger { color: #d92e2e; }
.conclusion-panel {
  display: grid;
  grid-column: 2 / 8;
  grid-row: 3;
  grid-template-columns: 44px minmax(0, 1fr) 300px;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
}
.conclusion-status {
  display: grid;
  width: 40px;
  height: 40px;
  border-radius: 8px;
  place-items: center;
  color: #ffffff;
  background: #94a3b8;
}
.conclusion-status.status-healthy { background: #16a34a; }
.conclusion-status.status-warning { background: #d97706; }
.conclusion-status.status-critical { background: #dc2626; }
.conclusion-panel h3 { margin: 0; font-size: 14px; }
.conclusion-panel p {
  margin: 5px 0 0;
  overflow: hidden;
  color: var(--text-secondary);
  font-size: 11px;
  line-height: 1.55;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.conclusion-panel dl {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
  margin: 0;
}
.conclusion-panel dl div { padding-left: 10px; border-left: 1px solid var(--border); }
.conclusion-panel dt { color: var(--text-muted); font-size: 9.5px; }
.conclusion-panel dd { margin: 2px 0 0; color: #344258; font-size: 16px; font-weight: 700; }
.chart-card--trend { grid-column: 2 / 4; grid-row: 4; }
.chart-card--status { grid-column: 4; grid-row: 4; }
.chart-card--domain { grid-column: 5; grid-row: 4; }
.chart-card--slow { grid-column: 6; grid-row: 4; }
.chart-card--client { grid-column: 7; grid-row: 4; }
.chart { width: 100%; height: calc(100% - 43px); min-height: 110px; }
.api-panel { grid-column: 2 / 5; grid-row: 5; }
.error-panel { grid-column: 5 / 8; grid-row: 5; }
.analysis-table { --el-table-border-color: #edf1f6; --el-table-header-bg-color: #f6f8fb; --el-table-row-hover-bg-color: #edf4ff; cursor: pointer; }
:deep(.analysis-table .el-table__header th) { height: 34px; color: #6e7b8f; font-size: 10px; font-weight: 600; }
:deep(.analysis-table .el-table__row td) { height: 31px; color: #354359; font-size: 10.5px; }
.method-tag {
  display: inline-flex;
  min-width: 36px;
  justify-content: center;
  padding: 2px 4px;
  border-radius: 3px;
  color: #1d5fd0;
  background: #eaf2ff;
  font-size: 9px;
  font-weight: 700;
}
.status-code { color: #d92e2e; font-weight: 700; }
.compact-empty { padding: 14px 6px; color: var(--text-muted); text-align: center; font-size: 10px; }
@media (max-width: 1380px) {
  .log-analysis-shell { min-width: 1080px; }
  .analysis-grid { grid-template-columns: 210px repeat(6, minmax(0, 1fr)); }
  .generated-time { display: none; }
  .kpi-panel article { padding: 10px; }
  .kpi-panel strong { font-size: 18px; }
  .filter-panel { grid-template-columns: 1fr 132px 1fr 138px 78px 78px; }
}
@media (max-width: 760px) {
  .log-analysis-page { padding: 0; overflow: auto; }
  .log-analysis-shell { min-width: 0; min-height: 100%; height: auto; border: 0; border-radius: 0; overflow: visible; }
  .analysis-header { position: sticky; top: 0; z-index: 20; padding: 0 10px; }
  .header-main-row { min-height: 58px; }
  .header-actions .share-command,
  .header-actions .generated-time { display: none; }
  .business-path > span,
  .business-path > i,
  .business-heading p { display: none; }
  .business-path strong { max-width: 150px; font-size: 14px; }
  .detail-tabs { gap: 20px; padding-left: 42px; overflow-x: auto; }
  .analysis-grid { display: flex; padding: 7px; flex-direction: column; }
  .source-panel,
  .filter-panel,
  .kpi-panel,
  .conclusion-panel,
  .chart-card,
  .api-panel,
  .error-panel { width: 100%; }
  .source-panel { max-height: 320px; }
  .filter-panel { display: grid; grid-template-columns: 1fr; }
  .kpi-panel { grid-template-columns: repeat(2, 1fr); }
  .kpi-panel article { border-bottom: 1px solid var(--border); }
  .kpi-panel article:nth-child(even) { border-right: 0; }
  .conclusion-panel { grid-template-columns: 36px minmax(0, 1fr); }
  .conclusion-panel dl { grid-column: 1 / 3; }
  .conclusion-panel p { white-space: normal; }
  .chart-card { height: 260px; }
  .api-panel,
  .error-panel { min-height: 300px; }
}
</style>
