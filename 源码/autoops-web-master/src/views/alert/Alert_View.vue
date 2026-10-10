<template>
  <div class="alert-center-page alert-view-page">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/警告.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>告警总览</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>集中查看告警态势、通知趋势与最新活跃事件。</p>
      </div>
      <div class="alert-header-actions">
        <div class="alert-global-range" aria-label="告警统计时间范围">
          <span class="alert-global-range-label">统计时间</span>
          <div class="alert-range-switch">
            <button
              v-for="option in dashboardRangeOptions"
              :key="option.value"
              type="button"
              :class="{ active: dashboardRange === option.value }"
              :disabled="dashboardLoading"
              @click="changeDashboardRange(option.value)"
            >
              {{ option.label }}
            </button>
          </div>
          <span class="alert-custom-range-label">自定义</span>
          <el-date-picker
            v-model="customTimeRange"
            class="alert-custom-range-picker"
            :class="{ 'is-active': dashboardRange === 'custom' }"
            type="datetimerange"
            value-format="YYYY-MM-DD HH:mm:ss"
            range-separator="至"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            :disabled="dashboardLoading"
            @change="changeCustomTimeRange"
          />
        </div>
      </div>
    </header>

    <el-card shadow="never" class="alert-center-card">
      <div class="alert-view-scroll">
        <section class="alert-summary-grid">
          <article class="alert-summary-card alert-summary-card-active">
            <div class="alert-summary-main">
              <span>活跃告警：</span>
              <strong>{{ activeTotal }}</strong>
            </div>
            <div class="alert-summary-icon danger"><el-icon><WarningFilled /></el-icon></div>
          </article>
          <article class="alert-summary-card">
            <div class="alert-summary-main">
              <span>Critical：</span>
              <strong>{{ severityCount.critical }}</strong>
            </div>
            <div class="alert-summary-icon danger"><el-icon><BellFilled /></el-icon></div>
          </article>
          <article class="alert-summary-card">
            <div class="alert-summary-main">
              <span>Warning：</span>
              <strong>{{ severityCount.warning }}</strong>
            </div>
            <div class="alert-summary-icon warning"><el-icon><WarnTriangleFilled /></el-icon></div>
          </article>
          <article class="alert-summary-card">
            <div class="alert-summary-main">
              <span>今日恢复：</span>
              <strong>{{ todayResolved }}</strong>
            </div>
            <div class="alert-summary-icon success"><el-icon><CircleCheckFilled /></el-icon></div>
          </article>
        </section>

        <section class="alert-dashboard-grid">
          <article class="alert-dashboard-card" v-loading="dashboardLoading">
            <div class="alert-dashboard-card-header">
              <div class="alert-dashboard-title is-success">
                <el-icon><Connection /></el-icon>
                <span>告警群组 Top5</span>
              </div>
            </div>
            <div ref="ruleGroupChartRef" class="alert-dashboard-chart"></div>
            <p v-if="!ruleGroupTop.length" class="alert-dashboard-empty is-overlay">暂无告警群组统计</p>
          </article>

          <article class="alert-dashboard-card" v-loading="dashboardLoading">
            <div class="alert-dashboard-card-header">
              <div class="alert-dashboard-title is-danger">
                <el-icon><WarningFilled /></el-icon>
                <span>告警名称 Top5</span>
              </div>
            </div>
            <div v-if="alertNameTop.length" class="alert-dashboard-table">
              <div class="alert-dashboard-table-head">
                <span>排名</span>
                <span>告警名称</span>
                <span>数量</span>
                <span>占比</span>
              </div>
              <div
                v-for="(item, index) in alertNameTop"
                :key="item.key"
                class="alert-dashboard-table-row"
                :class="`rank-tone-${index + 1}`"
              >
                <span><i class="alert-rank-badge">{{ index + 1 }}</i></span>
                <span class="alert-rank-name" :title="item.name">{{ item.name }}</span>
                <strong>{{ formatNumber(item.count) }}</strong>
                <span>{{ formatPercent(item.percent) }}</span>
              </div>
            </div>
            <p v-else class="alert-dashboard-empty">暂无告警名称统计</p>
          </article>

          <article class="alert-dashboard-card" v-loading="dashboardLoading">
            <div class="alert-dashboard-card-header">
              <div class="alert-dashboard-title is-warning">
                <el-icon><BellFilled /></el-icon>
                <span>告警通知统计</span>
              </div>
            </div>
            <div class="alert-notify-summary">
              <span>通知 {{ formatNumber(notifySummary.notifications) }}</span>
              <span>覆盖告警 {{ formatNumber(notifySummary.events) }}</span>
              <span class="is-success">成功率 {{ formatPercent(notifySummary.successRate) }}</span>
              <span v-if="notifySummary.failures" class="is-danger">失败 {{ formatNumber(notifySummary.failures) }}</span>
            </div>
            <div ref="notifyTrendChartRef" class="alert-dashboard-chart"></div>
            <p v-if="!notifyTrend.length" class="alert-dashboard-empty is-overlay">暂无通知趋势数据</p>
          </article>
        </section>

        <section class="alert-screen-grid">
          <article class="alert-screen-card" v-loading="screenLoading">
            <div class="alert-screen-head">
              <h3>告警级别分布</h3>
              <span>活跃 {{ formatNumber(screenData.totalActive) }}</span>
            </div>
            <div ref="severityChartRef" class="alert-screen-chart"></div>
          </article>

          <article class="alert-screen-card" v-loading="screenLoading">
            <div class="alert-screen-head">
              <h3>最新活跃告警 Top5</h3>
              <el-button type="warning" class="alert-reset-button ops-refresh-button" :loading="screenLoading" @click="loadScreenDashboard">
                <el-icon><Refresh /></el-icon>
                <span>刷新</span>
              </el-button>
            </div>
            <div v-if="latestActiveTop5.length" class="latest-alert-list">
              <button
                v-for="(item, index) in latestActiveTop5"
                :key="item.key"
                type="button"
                class="latest-alert-row"
                @click="openDetail(item.raw)"
              >
                <span class="latest-alert-index">{{ index + 1 }}</span>
                <span class="latest-alert-main">
                  <strong>{{ item.name }}</strong>
                  <em>{{ item.source }} / {{ item.instance }}</em>
                </span>
                <el-tag :type="severityTagType(item.severity)" effect="light" size="small">{{ severityLabel(item.severity) }}</el-tag>
                <span class="latest-alert-time">{{ formatDateTime(item.time) }}</span>
              </button>
            </div>
            <p v-else class="alert-dashboard-empty">暂无活跃告警</p>
          </article>

          <article class="alert-screen-card" v-loading="screenLoading">
            <div class="alert-screen-head">
              <h3>告警名称分布</h3>
              <span>Top10</span>
            </div>
            <div ref="alertNameChartRef" class="alert-screen-chart"></div>
          </article>
        </section>
      </div>
    </el-card>

    <el-drawer v-model="detailVisible" title="告警详情" size="520px" class="alert-detail-drawer">
      <div v-if="currentRow" class="alert-detail-stack">
        <section class="alert-detail-panel">
          <h4>基础信息</h4>
          <div class="alert-detail-row"><span>规则</span><span>{{ currentRow.ruleName || currentRow.name || '-' }}</span></div>
          <div class="alert-detail-row"><span>级别</span><span>{{ severityLabel(currentRow.severity) }}</span></div>
          <div class="alert-detail-row"><span>状态</span><span>{{ statusLabel(currentRow.status) }}</span></div>
          <div class="alert-detail-row"><span>业务</span><span>{{ currentRow.businessId || '-' }}</span></div>
          <div class="alert-detail-row"><span>指纹</span><span>{{ currentRow.fingerprint || '-' }}</span></div>
        </section>
        <section class="alert-detail-panel">
          <h4>标签</h4>
          <pre class="alert-json-preview">{{ stringifyJson(currentRow.labels) || '-' }}</pre>
        </section>
        <section class="alert-detail-panel">
          <h4>注解</h4>
          <pre class="alert-json-preview">{{ stringifyJson(currentRow.annotations) || '-' }}</pre>
        </section>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import * as echarts from 'echarts'
import { BellFilled, CircleCheckFilled, Connection, Refresh, WarningFilled, WarnTriangleFilled } from '@element-plus/icons-vue'
import alertApi from '@/api/alertmanager'
import {
  formatDateTime,
  normalizeListResult,
  normalizeSeverity,
  severityLabel,
  severityTagType,
  statusLabel,
  stringifyJson
} from './alertCommon'
import './alert-pages.css'

const loading = ref(false)
const activeRows = ref([])
const historyRows = ref([])
const activeTotal = ref(0)
const historyTotal = ref(0)
const detailVisible = ref(false)
const currentRow = ref(null)
const dashboardLoading = ref(false)
const alertNameTop = ref([])
const notifyTrend = ref([])
const ruleGroupTop = ref([])
const notifyTrendChartRef = ref(null)
const ruleGroupChartRef = ref(null)
const severityChartRef = ref(null)
const alertNameChartRef = ref(null)
const screenLoading = ref(false)
const screenData = ref({
  severityDistribution: [],
  latestActiveTop5: [],
  alertNameTop10: [],
  totalActive: 0,
  source: ''
})
const dashboardRange = ref('today')
const customTimeRange = ref([])
let notifyTrendChart = null
let ruleGroupChart = null
let severityChart = null
let alertNameChart = null

const dashboardRangeOptions = [
  { label: '今天', value: 'today' },
  { label: '昨天', value: 'yesterday' },
  { label: '7天', value: '7d' },
  { label: '30天', value: '30d' }
]

const severityCount = computed(() => activeRows.value.reduce((acc, row) => {
  const key = normalizeSeverity(row.severity) || 'info'
  acc[key] = (acc[key] || 0) + 1
  return acc
}, { critical: 0, warning: 0, info: 0 }))

const todayResolved = computed(() => {
  const today = new Date().toISOString().slice(0, 10)
  return historyRows.value.filter(row => {
    const time = row.occurredAt || row.createdAt || ''
    return String(time).startsWith(today) && ['resolved', 'recover', 'recovered'].includes(String(row.eventType || row.status || '').toLowerCase())
  }).length
})

const severityDistribution = computed(() => normalizeSeverityDistribution(screenData.value.severityDistribution))
const alertNameTop10 = computed(() => normalizeScreenAlertNameTop(screenData.value.alertNameTop10))
const latestActiveTop5 = computed(() => normalizeLatestActiveAlerts(screenData.value.latestActiveTop5))
const notifySummary = computed(() => {
  const totals = notifyTrend.value.reduce((acc, item) => {
    acc.notifications += item.notificationCount
    acc.events += item.eventCount
    acc.successes += item.successCount
    acc.failures += item.failedCount
    return acc
  }, { notifications: 0, events: 0, successes: 0, failures: 0 })
  totals.successRate = totals.notifications ? (totals.successes / totals.notifications) * 100 : 0
  return totals
})

const unwrapPayload = response => {
  const first = response?.data ?? response ?? {}
  if (first && typeof first === 'object' && 'code' in first && 'data' in first) {
    return first.data ?? {}
  }
  return first
}

const extractArray = (source, keys = []) => {
  const payload = unwrapPayload(source)
  if (Array.isArray(payload)) return payload
  if (!payload || typeof payload !== 'object') return []

  const readNestedArray = value => {
    if (Array.isArray(value)) return value
    if (!value || typeof value !== 'object') return null
    return ['items', 'list', 'rows', 'records', 'data'].map(key => value[key]).find(Array.isArray) || null
  }

  for (const key of keys) {
    const value = readNestedArray(payload[key])
    if (value) return value
  }

  return readNestedArray(payload) || []
}

const pickValue = (object, keys, fallback = undefined) => {
  for (const key of keys) {
    const value = object?.[key]
    if (value !== undefined && value !== null && value !== '') return value
  }
  return fallback
}

const toNumber = value => {
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : 0
}

const withPercent = rows => {
  const total = rows.reduce((sum, item) => sum + toNumber(item.count), 0)
  return rows.map(item => ({
    ...item,
    percent: item.percent === undefined || item.percent === null || item.percent === ''
      ? (total ? (toNumber(item.count) / total) * 100 : 0)
      : toNumber(item.percent)
  }))
}

const normalizeAlertNameTop = rows => withPercent(rows.map((item, index) => {
  const name = pickValue(item, ['name', 'ruleName', 'alertName', 'RuleName', 'AlertName'], '未命名告警')
  return {
    key: `alert-${pickValue(item, ['ruleId', 'ruleID', 'id', 'ID'], index)}-${name}`,
    ruleId: pickValue(item, ['ruleId', 'ruleID', 'id', 'ID'], ''),
    name,
    count: toNumber(pickValue(item, ['count', 'total', 'eventCount', 'value', 'Count'], 0)),
    percent: pickValue(item, ['percent', 'ratio', 'Percent'], undefined)
  }
})).sort((a, b) => b.count - a.count).slice(0, 5)

const normalizeRuleGroupTop = rows => withPercent(rows.map((item, index) => {
  const groupName = pickValue(item, ['ruleGroupName', 'groupName', 'name', 'hostGroupName', 'GroupName'], '未分组')
  return {
    key: `rule-group-${pickValue(item, ['ruleGroupId', 'groupId', 'id', 'ID'], index)}-${groupName}`,
    groupId: pickValue(item, ['ruleGroupId', 'groupId', 'id', 'ID'], ''),
    groupName,
    count: toNumber(pickValue(item, ['count', 'total', 'eventCount', 'value', 'Count'], 0)),
    percent: pickValue(item, ['percent', 'ratio', 'Percent'], undefined)
  }
})).sort((a, b) => b.count - a.count).slice(0, 5)

const normalizeNotifyTrend = rows => rows.map((item, index) => ({
  key: `notify-${pickValue(item, ['date', 'day', 'time', 'createdAt'], index)}`,
  date: pickValue(item, ['date', 'day', 'time', 'createdAt'], '-'),
  notificationCount: toNumber(pickValue(item, ['notificationCount', 'notifyCount', 'count', 'NotificationCount'], 0)),
  eventCount: toNumber(pickValue(item, ['eventCount', 'alertCount', 'EventCount'], 0)),
  successCount: toNumber(pickValue(item, ['successCount', 'SuccessCount'], 0)),
  failedCount: toNumber(pickValue(item, ['failedCount', 'failureCount', 'FailedCount'], 0))
}))

const normalizeSeverityDistribution = rows => {
  const sourceRows = Array.isArray(rows) ? rows : []
  const fallback = [
    { level: 'P0', name: 'P0', count: 0, percent: 0 },
    { level: 'P1', name: 'P1', count: 0, percent: 0 },
    { level: 'P2', name: 'P2', count: 0, percent: 0 }
  ]
  const byLevel = new Map(sourceRows.map(item => [String(item.level || item.name || '').toUpperCase(), item]))
  return withPercent(fallback.map(item => {
    const row = byLevel.get(item.level) || {}
    return {
      level: item.level,
      name: pickValue(row, ['name', 'level'], item.name),
      count: toNumber(pickValue(row, ['count', 'total', 'value'], item.count)),
      percent: pickValue(row, ['percent', 'ratio'], item.percent)
    }
  }))
}

const normalizeScreenAlertNameTop = rows => withPercent((Array.isArray(rows) ? rows : []).map((item, index) => {
  const name = pickValue(item, ['name', 'ruleName', 'alertName', 'RuleName', 'AlertName'], '未命名告警')
  return {
    key: `screen-alert-${pickValue(item, ['ruleId', 'ruleID', 'id', 'ID'], index)}-${name}`,
    name,
    count: toNumber(pickValue(item, ['count', 'total', 'eventCount', 'value', 'Count'], 0)),
    percent: pickValue(item, ['percent', 'ratio', 'Percent'], undefined)
  }
})).sort((a, b) => b.count - a.count).slice(0, 10)

const normalizeLatestActiveAlerts = rows => (Array.isArray(rows) ? rows : []).map((item, index) => {
  const name = pickValue(item, ['ruleName', 'name', 'alertName', 'RuleName', 'AlertName'], '未命名告警')
  return {
    key: `latest-${pickValue(item, ['id', 'eventId', 'fingerprint'], index)}-${name}`,
    name,
    severity: pickValue(item, ['severity', 'level', 'priority'], ''),
    source: pickValue(item, ['datasourceType', 'source', 'datasourceName'], '-'),
    instance: pickValue(item, ['businessId', 'instance', 'host', 'resource'], '-'),
    time: pickValue(item, ['lastEvaluatedAt', 'updatedAt', 'startsAt', 'createdAt', 'time'], ''),
    raw: item
  }
}).slice(0, 5)

const formatNumber = value => new Intl.NumberFormat('zh-CN').format(toNumber(value))

const formatPercent = value => {
  const numeric = toNumber(value)
  const percent = numeric > 0 && numeric <= 1 ? numeric * 100 : numeric
  return `${percent.toFixed(2)}%`
}

const formatAxisLabel = value => {
  const text = String(value || '-')
  return text.length > 8 ? `${text.slice(0, 8)}...` : text
}

const formatTrendAxisLabel = value => {
  const text = String(value || '')
  if (text.includes(' ')) return text.split(' ')[1]
  return text.length >= 10 ? text.slice(5) : text
}

const dashboardParams = () => {
  const params = { range: dashboardRange.value, limit: 5 }
  if (dashboardRange.value === 'custom' && customTimeRange.value?.length === 2) {
    params.startTime = customTimeRange.value[0]
    params.endTime = customTimeRange.value[1]
  }
  return params
}

const applyDashboardPayload = payload => {
  alertNameTop.value = normalizeAlertNameTop(extractArray(payload, ['alertNameTop5', 'alertNameTop', 'topAlertNames', 'topAlertName']))
  notifyTrend.value = normalizeNotifyTrend(extractArray(payload, ['notifyTrend', 'notifyTrends', 'notificationTrend', 'notificationStats']))
  ruleGroupTop.value = normalizeRuleGroupTop(extractArray(payload, ['ruleGroupTop5', 'ruleGroupTop', 'alertRuleGroupTop5', 'alertRuleGroupTop', 'hostGroupTop5', 'hostGroupTop']))
}

const loadDashboardCharts = async () => {
  dashboardLoading.value = true
  try {
    const response = await alertApi.getDashboardCharts(dashboardParams())
    applyDashboardPayload(response)
    await nextTick()
    renderDashboardCharts()
  } finally {
    dashboardLoading.value = false
  }
}

const changeDashboardRange = range => {
  if (dashboardRange.value === range) return
  dashboardRange.value = range
  loadDashboardCharts()
}

const changeCustomTimeRange = range => {
  if (!range?.length || range.length !== 2) {
    if (dashboardRange.value === 'custom') {
      dashboardRange.value = 'today'
      loadDashboardCharts()
    }
    return
  }
  dashboardRange.value = 'custom'
  loadDashboardCharts()
}

const ensureChart = (chart, chartRef) => {
  if (!chartRef.value) return null
  return chart || echarts.init(chartRef.value)
}

const renderNotifyTrendChart = () => {
  notifyTrendChart = ensureChart(notifyTrendChart, notifyTrendChartRef)
  if (!notifyTrendChart) return
  if (!notifyTrend.value.length) {
    notifyTrendChart.clear()
    return
  }

  const dates = notifyTrend.value.map(item => item.date)
  notifyTrendChart.setOption({
    color: ['#d98a00', '#2878d0'],
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'line', lineStyle: { color: '#9aa8ba', type: 'dashed' } },
      formatter: params => {
        const index = params?.[0]?.dataIndex
        const item = notifyTrend.value[index]
        if (!item) return ''
        return [
          `<strong>${item.date}</strong>`,
          `通知次数：${formatNumber(item.notificationCount)}`,
          `覆盖告警：${formatNumber(item.eventCount)}`,
          `成功 / 失败：${formatNumber(item.successCount)} / ${formatNumber(item.failedCount)}`
        ].join('<br/>')
      }
    },
    legend: {
      top: 0,
      right: 52,
      itemWidth: 10,
      itemHeight: 6,
      textStyle: { color: '#5f6b7c', fontSize: 12 }
    },
    grid: { top: 34, right: 42, bottom: 26, left: 42 },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: dates,
      axisTick: { show: false },
      axisLine: { lineStyle: { color: '#dfe7f2' } },
      axisLabel: {
        color: '#6b7280',
        hideOverlap: true,
        formatter: formatTrendAxisLabel
      }
    },
    yAxis: [
      {
        type: 'value',
        min: 0,
        minInterval: 1,
        splitLine: { lineStyle: { type: 'dashed', color: '#e8edf5' } },
        axisLabel: { color: '#8a641f' }
      },
      {
        type: 'value',
        min: 0,
        minInterval: 1,
        splitLine: { show: false },
        axisLabel: { color: '#2878d0' }
      }
    ],
    series: [
      {
        name: '通知次数',
        type: 'line',
        smooth: 0.25,
        showSymbol: notifyTrend.value.length <= 12,
        symbol: 'circle',
        symbolSize: 7,
        lineStyle: { width: 3 },
        itemStyle: { borderColor: '#ffffff', borderWidth: 2 },
        emphasis: { focus: 'series' },
        data: notifyTrend.value.map(item => item.notificationCount),
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(217, 138, 0, 0.18)' },
            { offset: 1, color: 'rgba(217, 138, 0, 0.01)' }
          ])
        }
      },
      {
        name: '覆盖告警',
        type: 'line',
        yAxisIndex: 1,
        smooth: 0.25,
        showSymbol: notifyTrend.value.length <= 12,
        symbol: 'circle',
        symbolSize: 7,
        lineStyle: { width: 3 },
        itemStyle: { borderColor: '#ffffff', borderWidth: 2 },
        emphasis: { focus: 'series' },
        data: notifyTrend.value.map(item => item.eventCount)
      }
    ]
  }, true)
}

const renderRuleGroupChart = () => {
  ruleGroupChart = ensureChart(ruleGroupChart, ruleGroupChartRef)
  if (!ruleGroupChart) return
  if (!ruleGroupTop.value.length) {
    ruleGroupChart.clear()
    return
  }

  ruleGroupChart.setOption({
    color: ['#f4c430'],
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      formatter: params => {
        const item = params?.[0]
        if (!item) return ''
        return `${item.name}<br/>告警数量：${formatNumber(item.value)}`
      }
    },
    grid: { top: 18, right: 10, bottom: 48, left: 42 },
    xAxis: {
      type: 'category',
      data: ruleGroupTop.value.map(item => item.groupName),
      axisTick: { show: false },
      axisLine: { lineStyle: { color: '#dfe7f2' } },
      axisLabel: {
        color: '#6b7280',
        interval: 0,
        rotate: 28,
        formatter: formatAxisLabel
      }
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      splitLine: { lineStyle: { type: 'dashed', color: '#e8edf5' } },
      axisLabel: { color: '#6b7280' }
    },
    series: [
      {
        name: '告警数量',
        type: 'bar',
        barMaxWidth: 30,
        data: ruleGroupTop.value.map(item => item.count),
        label: {
          show: true,
          position: 'top',
          color: '#263143',
          fontWeight: 700
        },
        itemStyle: {
          borderRadius: [8, 8, 0, 0],
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: '#f8d94a' },
            { offset: 1, color: '#d18a00' }
          ])
        }
      }
    ]
  }, true)
}

const pieColors = ['#ef5f67', '#f4a825', '#4c8bf5', '#67c23a', '#8b5cf6', '#14b8a6', '#f97316', '#64748b', '#ec4899', '#22c55e']

const renderDonutChart = (chartRef, chart, rows, title, colors = pieColors) => {
  const instance = ensureChart(chart, chartRef)
  if (!instance) return null
  if (!rows.length) {
    instance.clear()
    return instance
  }

  instance.setOption({
    color: colors,
    tooltip: {
      trigger: 'item',
      formatter: params => `${params.name}<br/>数量：${formatNumber(params.value)}<br/>占比：${formatPercent(params.percent)}`
    },
    legend: {
      bottom: 0,
      type: 'scroll',
      itemWidth: 12,
      itemHeight: 8,
      textStyle: { color: '#5f6b7c', fontSize: 12 }
    },
    series: [
      {
        name: title,
        type: 'pie',
        radius: ['48%', '68%'],
        center: ['50%', '44%'],
        minAngle: 4,
        avoidLabelOverlap: true,
        label: {
          formatter: params => `${params.percent.toFixed(1)}%`,
          color: '#263143',
          fontSize: 12
        },
        labelLine: {
          length: 10,
          length2: 8
        },
        itemStyle: {
          borderColor: '#ffffff',
          borderWidth: 3,
          borderRadius: 8
        },
        data: rows.map(item => ({
          name: item.name,
          value: item.count
        }))
      }
    ]
  }, true)
  return instance
}

const renderScreenCharts = () => {
  severityChart = renderDonutChart(
    severityChartRef,
    severityChart,
    severityDistribution.value,
    '告警级别分布',
    ['#ef5f67', '#f4a825', '#4c8bf5']
  )
  alertNameChart = renderDonutChart(alertNameChartRef, alertNameChart, alertNameTop10.value, '告警名称分布')
}

const renderDashboardCharts = () => {
  renderNotifyTrendChart()
  renderRuleGroupChart()
}

const resizeDashboardCharts = () => {
  notifyTrendChart?.resize()
  ruleGroupChart?.resize()
  severityChart?.resize()
  alertNameChart?.resize()
}

const loadScreenDashboard = async () => {
  screenLoading.value = true
  try {
    const response = await alertApi.getDashboardScreen()
    const payload = unwrapPayload(response)
    screenData.value = {
      severityDistribution: Array.isArray(payload.severityDistribution) ? payload.severityDistribution : [],
      latestActiveTop5: Array.isArray(payload.latestActiveTop5) ? payload.latestActiveTop5 : [],
      alertNameTop10: Array.isArray(payload.alertNameTop10) ? payload.alertNameTop10 : [],
      totalActive: toNumber(payload.totalActive),
      source: payload.source || ''
    }
    if (screenData.value.totalActive) {
      activeTotal.value = screenData.value.totalActive
    }
    await nextTick()
    renderScreenCharts()
  } finally {
    screenLoading.value = false
  }
}

const loadOverview = async () => {
  loading.value = true
  try {
    const [activeRes, historyRes] = await Promise.all([
      alertApi.getActiveEvents({ page: 1, pageSize: 10 }),
      alertApi.getHistoryEvents({ page: 1, pageSize: 20 })
    ])
    const active = normalizeListResult(activeRes)
    const history = normalizeListResult(historyRes)
    activeRows.value = active.list
    activeTotal.value = active.total
    historyRows.value = history.list
    historyTotal.value = history.total
  } finally {
    loading.value = false
  }
}

const openDetail = row => {
  currentRow.value = row
  detailVisible.value = true
}

onMounted(() => {
  loadOverview()
  loadDashboardCharts()
  loadScreenDashboard()
  window.addEventListener('resize', resizeDashboardCharts)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', resizeDashboardCharts)
  notifyTrendChart?.dispose()
  ruleGroupChart?.dispose()
  severityChart?.dispose()
  alertNameChart?.dispose()
  notifyTrendChart = null
  ruleGroupChart = null
  severityChart = null
  alertNameChart = null
})
</script>

<style scoped>
:global(.home-container .el-main:has(.alert-view-page)) {
  --ops-shell-gutter: 10px;
  --ops-page-padding: 7px;
  --ops-module-gap: 8px;
  padding-top: var(--ops-shell-gutter);
  padding-right: var(--ops-shell-gutter);
  padding-left: var(--ops-shell-gutter);
  overflow-y: hidden;
}

.alert-view-page {
  --ops-text-heading: #0f172a;
  --ops-text-primary: #172033;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-bg-surface: #ffffff;
  --ops-bg-surface-soft: #fbfdff;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  box-sizing: border-box;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: var(--ops-module-gap, 8px);
  height: calc(100vh - 124px);
  max-height: none;
  padding: var(--ops-page-padding, 7px);
  overflow: hidden;
  border-radius: 0;
  background: transparent;
  color: var(--ops-text-primary);
}

.ops-page-header {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 12px;
  min-width: 0;
  padding: 8px 12px;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 10px 28px rgba(80, 124, 180, 0.1);
}

.ops-page-header__icon {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  place-items: center;
  border: 0;
  background: transparent;
}

.ops-page-header__icon img {
  width: 30px;
  height: 30px;
  object-fit: contain;
}

.ops-page-header__content {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.ops-page-header h1 {
  flex: 0 0 auto;
  margin: 0;
  color: var(--ops-text-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: 0;
  white-space: nowrap;
}

.ops-page-header__divider {
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  background: var(--ops-border-soft);
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

.alert-view-page :deep(.alert-center-card.el-card) {
  position: relative;
  display: flex;
  min-width: 0;
  min-height: 0;
  height: 100%;
  flex-direction: column;
  overflow: hidden !important;
  border: 1px solid var(--ops-border);
  border-radius: 12px !important;
  background: var(--ops-bg-surface);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
  backdrop-filter: none;
}

.alert-view-page :deep(.alert-center-card::before) {
  display: none;
}

.alert-view-page :deep(.alert-center-card .el-card__body) {
  box-sizing: border-box;
  display: flex;
  min-height: 0;
  height: 100%;
  flex: 1;
  padding: 8px 10px 10px;
  overflow: hidden;
  border-radius: 0;
}

.alert-header-actions,
.alert-global-range {
  display: flex;
  align-items: center;
}

.alert-header-actions {
  flex: 0 0 auto;
  min-width: 0;
  justify-content: flex-end;
  gap: 8px;
}

.alert-global-range {
  min-width: 0;
  gap: 8px;
}

.alert-global-range-label {
  flex: 0 0 auto;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  font-weight: 700;
}

.alert-custom-range-label {
  flex: 0 0 auto;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  font-weight: 700;
}

.alert-custom-range-picker {
  width: 330px;
}

.alert-view-page :deep(.alert-custom-range-picker.el-range-editor) {
  height: 34px;
  border-radius: 8px;
  box-shadow: 0 0 0 1px rgba(223, 231, 242, 0.95) inset;
}

.alert-view-page :deep(.alert-custom-range-picker.is-active.el-range-editor) {
  box-shadow: 0 0 0 1px var(--aiops-brand) inset;
}

.alert-view-scroll {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-rows: 72px minmax(250px, 1fr) minmax(256px, 1fr);
  gap: var(--ops-module-gap, 8px);
  overflow: auto;
  padding: 0;
}

.alert-view-page :deep(.alert-summary-grid) {
  height: 100%;
  gap: 8px;
  margin-bottom: 0;
}

.alert-view-page :deep(.alert-summary-card) {
  align-items: center;
  box-sizing: border-box;
  min-width: 0;
  min-height: 0;
  height: 100%;
  padding: 8px 11px;
  border-color: var(--ops-border-soft);
  border-radius: 10px;
  background: linear-gradient(180deg, #ffffff, #fbfdff);
  box-shadow: none;
}

.alert-summary-main {
  display: flex;
  min-width: 0;
  align-items: baseline;
  gap: 0;
  white-space: nowrap;
}

.alert-view-page :deep(.alert-summary-main) {
  min-width: 0;
}

.alert-view-page :deep(.alert-summary-card strong) {
  color: #172133;
  font-size: 20px;
  font-weight: 800;
  line-height: 1.05;
}

.alert-view-page :deep(.alert-summary-card span) {
  display: block;
  overflow: hidden;
  color: #4b5f7a;
  font-size: 13px;
  font-weight: 700;
  line-height: 1.05;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.alert-view-page :deep(.alert-summary-card-active span) {
  color: #286fd6;
}

.alert-view-page :deep(.alert-summary-icon) {
  flex: 0 0 30px;
  width: 30px;
  height: 30px;
  margin-left: 8px;
  border-radius: 10px;
  font-size: 14px;
}
.alert-dashboard-grid {
  display: grid;
  grid-template-columns: minmax(360px, 1fr) minmax(420px, 1fr) minmax(360px, 1fr);
  height: 100%;
  min-height: 0;
  gap: 8px;
  margin: 0;
}

.alert-dashboard-card {
  position: relative;
  box-sizing: border-box;
  min-height: 0;
  height: 100%;
  padding: 8px 10px 6px;
  overflow: hidden;
  border: 1px solid var(--ops-border-soft);
  border-radius: 10px;
  background: var(--ops-bg-surface-soft);
  box-shadow: 0 6px 18px rgba(80, 124, 180, 0.05);
}

.alert-dashboard-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 4px;
}

.alert-dashboard-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--aiops-text-primary);
  font-size: 13px;
  font-weight: 700;
}

.alert-dashboard-title.is-danger {
  color: #cf3f5b;
}

.alert-dashboard-title.is-warning {
  color: #b7791f;
}

.alert-dashboard-title.is-success {
  color: #2f9f69;
}

.alert-range-switch {
  display: inline-flex;
  align-items: center;
  padding: 3px;
  border-radius: 8px;
  background: rgba(244, 247, 252, 0.92);
}

.alert-range-switch button {
  min-width: 56px;
  height: 28px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--aiops-text-secondary);
  font: inherit;
  font-size: 13px;
  cursor: pointer;
  transition: color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease;
}

.alert-range-switch button.active {
  background: #ffffff;
  color: var(--aiops-brand);
  font-weight: 700;
  box-shadow: 0 4px 12px rgba(38, 68, 110, 0.1);
}

.alert-range-switch button:disabled {
  cursor: wait;
  opacity: 0.62;
}

.alert-dashboard-table {
  overflow: hidden;
  border-radius: 8px;
  border: 1px solid rgba(223, 231, 242, 0.72);
}

.alert-dashboard-table-head,
.alert-dashboard-table-row {
  display: grid;
  grid-template-columns: 70px minmax(0, 1fr) 80px 80px;
  align-items: center;
  gap: 10px;
  min-height: 31px;
  padding: 0 14px;
}

.alert-dashboard-table-head {
  background: rgba(248, 250, 255, 0.96);
  color: var(--aiops-text-muted);
  font-size: 13px;
  font-weight: 700;
}

.alert-dashboard-table-row {
  min-height: 33px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  border-top: 1px solid rgba(223, 231, 242, 0.7);
}

.alert-dashboard-table-row strong {
  color: var(--aiops-text-primary);
}

.alert-dashboard-table-row.rank-tone-1 {
  background: rgba(255, 235, 235, 0.72);
}

.alert-dashboard-table-row.rank-tone-2 {
  background: rgba(255, 245, 224, 0.72);
}

.alert-dashboard-table-row.rank-tone-3 {
  background: rgba(255, 250, 222, 0.72);
}

.alert-dashboard-table-row.rank-tone-4 {
  background: rgba(236, 250, 230, 0.72);
}

.alert-dashboard-table-row.rank-tone-5 {
  background: rgba(227, 246, 255, 0.72);
}

.alert-rank-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 999px;
  background: linear-gradient(180deg, #ff7681, #f45864);
  color: #ffffff;
  font-style: normal;
  font-weight: 800;
  box-shadow: 0 6px 12px rgba(244, 88, 100, 0.18);
}

.rank-tone-2 .alert-rank-badge {
  background: linear-gradient(180deg, #ffb47c, #ff8d5d);
}

.rank-tone-3 .alert-rank-badge {
  background: linear-gradient(180deg, #ffd45d, #f5b83d);
}

.rank-tone-4 .alert-rank-badge {
  background: linear-gradient(180deg, #a7e96f, #79d35c);
}

.rank-tone-5 .alert-rank-badge {
  background: linear-gradient(180deg, #70c7ff, #44a8e8);
}

.alert-rank-name {
  overflow: hidden;
  color: var(--aiops-text-primary);
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.alert-dashboard-chart {
  width: 100%;
  height: calc(100% - 26px);
  min-height: 0;
}

.alert-notify-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin: 0 0 3px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.alert-notify-summary span {
  padding: 3px 7px;
  border-radius: 999px;
  background: rgba(255, 248, 231, 0.88);
  font-weight: 700;
}

.alert-notify-summary span.is-success {
  color: #287a51;
  background: rgba(232, 247, 239, 0.92);
}

.alert-notify-summary span.is-danger {
  color: #b9354f;
  background: rgba(255, 235, 239, 0.94);
}

.alert-notify-summary + .alert-dashboard-chart {
  height: calc(100% - 52px);
}

.alert-dashboard-empty {
  margin: 42px 0 0;
  color: var(--aiops-text-muted);
  font-size: 13px;
  text-align: center;
}

.alert-dashboard-empty.is-overlay {
  position: absolute;
  inset: 132px 20px auto;
  margin: 0;
  pointer-events: none;
}

.alert-screen-grid {
  display: grid;
  grid-template-columns: minmax(320px, 1fr) minmax(420px, 1.1fr) minmax(320px, 1fr);
  height: 100%;
  min-height: 0;
  gap: 8px;
  margin: 0;
}

.alert-screen-card {
  position: relative;
  box-sizing: border-box;
  min-height: 0;
  height: 100%;
  padding: 8px 10px 6px;
  overflow: hidden;
  border: 1px solid var(--ops-border-soft);
  border-radius: 10px;
  background: var(--ops-bg-surface-soft);
  box-shadow: 0 6px 18px rgba(80, 124, 180, 0.05);
}

.alert-screen-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 6px;
}

.alert-screen-head h3 {
  margin: 0;
  color: var(--aiops-text-primary);
  font-size: 13px;
  font-weight: 700;
}

.alert-screen-head span {
  color: var(--aiops-brand);
  font-size: 12px;
  font-weight: 700;
}

.alert-screen-chart {
  width: 100%;
  height: calc(100% - 28px);
  min-height: 0;
}

.latest-alert-list {
  display: grid;
  gap: 4px;
  height: calc(100% - 34px);
  align-content: start;
  overflow: hidden;
}

.latest-alert-row {
  display: grid;
  box-sizing: border-box;
  grid-template-columns: 34px minmax(0, 1fr) 82px 132px;
  align-items: center;
  gap: 10px;
  width: 100%;
  min-height: 35px;
  padding: 4px 10px;
  border: 1px solid rgba(223, 231, 242, 0.78);
  border-radius: 8px;
  background: #ffffff;
  color: inherit;
  cursor: pointer;
  text-align: left;
  transition: border-color 0.18s ease, background-color 0.18s ease;
}

.latest-alert-row:hover {
  transform: none;
  border-color: rgba(76, 139, 245, 0.34);
  background: #f7faff;
}

.latest-alert-index {
  display: inline-grid;
  place-items: center;
  width: 24px;
  height: 24px;
  border-radius: 999px;
  background: linear-gradient(180deg, #ff7681, #f45864);
  color: #ffffff;
  font-size: 12px;
  font-weight: 800;
}

.latest-alert-main {
  min-width: 0;
}

.latest-alert-main strong,
.latest-alert-main em {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.latest-alert-main strong {
  color: var(--aiops-text-primary);
  font-size: 13px;
}

.latest-alert-main em {
  margin-top: 1px;
  color: var(--aiops-text-muted);
  font-size: 12px;
  font-style: normal;
}

.latest-alert-time {
  color: var(--aiops-text-secondary);
  font-size: 12px;
  text-align: right;
}

.alert-view-page :deep(.el-button) {
  height: 34px;
  min-height: 34px;
  border-radius: 8px;
  letter-spacing: 0;
}

.alert-view-page :deep(.el-button:hover),
.alert-view-page :deep(.el-button:active) {
  transform: none;
}

.alert-view-page :deep(.ops-refresh-button.el-button--warning) {
  padding: 0 12px;
  border-color: #f3c66b;
  background: #f3c66b;
  color: #ffffff;
  box-shadow: none;
}

.alert-view-page :deep(.ops-refresh-button.el-button--warning:hover),
.alert-view-page :deep(.ops-refresh-button.el-button--warning:focus) {
  border-color: #e8b84f;
  background: #e8b84f;
  color: #ffffff;
}

.alert-range-switch button:focus-visible,
.latest-alert-row:focus-visible {
  outline: 2px solid rgba(64, 158, 255, 0.48);
  outline-offset: 2px;
}

:global(.alert-detail-drawer .el-drawer) {
  border-color: var(--ops-border, #dbe7f4);
  border-radius: 15px 0 0 15px;
  background: #ffffff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.alert-detail-drawer .el-drawer__header) {
  padding: 14px 16px 12px;
  border-bottom-color: var(--ops-border-soft, #e7eef7);
  background: #fbfdff;
}

:global(.alert-detail-drawer .el-drawer__body) {
  padding: 14px 16px;
}

:global(.alert-detail-drawer .alert-detail-stack) {
  gap: 8px;
}

:global(.alert-detail-drawer .alert-detail-panel) {
  padding: 12px;
  border-radius: 10px;
  background: #fbfdff;
}

:global(.alert-detail-drawer .alert-json-preview) {
  border-radius: 8px;
}

@media (max-width: 1180px) {
  .ops-page-header {
    flex-wrap: wrap;
  }

  .alert-header-actions {
    align-items: flex-start;
    flex-direction: column;
  }

  .alert-header-actions {
    width: 100%;
  }

  .alert-global-range {
    width: 100%;
    flex-wrap: wrap;
  }

  .alert-view-scroll {
    display: block;
    overflow: auto;
  }

  .alert-view-page :deep(.alert-summary-grid) {
    height: 72px;
  }

  .alert-dashboard-grid,
  .alert-screen-grid {
    grid-template-columns: 1fr;
    height: auto;
    gap: 8px;
    margin-top: 8px;
  }

  .alert-dashboard-card,
  .alert-screen-card {
    height: 260px;
  }
}

@media (max-width: 768px) {
  .ops-page-header {
    gap: 8px;
  }

  .ops-page-header__content {
    flex-wrap: wrap;
    gap: 2px 8px;
  }

  .ops-page-header__divider {
    display: none;
  }

  .ops-page-header__content p {
    width: 100%;
  }

  .alert-global-range-label {
    width: 100%;
  }

  .alert-custom-range-picker {
    width: 100%;
  }

  .alert-dashboard-card {
    min-height: 280px;
    padding: 10px;
  }

  .alert-dashboard-card-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .alert-range-switch {
    width: 100%;
    justify-content: space-between;
  }

  .alert-range-switch button {
    flex: 1;
    min-width: 0;
  }

  .alert-dashboard-table-head,
  .alert-dashboard-table-row {
    grid-template-columns: 46px minmax(120px, 1fr) 56px 64px;
    padding: 0 10px;
  }

  .latest-alert-row {
    grid-template-columns: 30px minmax(0, 1fr);
  }

  .latest-alert-row .el-tag,
  .latest-alert-time {
    display: none;
  }
}
</style>
