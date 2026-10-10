<script setup>
import { computed, nextTick, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import * as echarts from 'echarts'
import {
  BellFilled,
  Clock,
  Cloudy,
  Connection,
  Cpu,
  DataAnalysis,
  Histogram,
  Key,
  Monitor,
  OfficeBuilding,
  PieChart,
  Refresh,
  TrendCharts,
  WarningFilled
} from '@element-plus/icons-vue'
import alertApi from '@/api/alertmanager'
import dashboardApi from '@/api/dashboard'
import cmdbApi from '@/api/cmdb'
import {
  formatDateTime,
  normalizeListResult,
  severityLabel,
  severityTagType
} from '@/views/alert/alertCommon'

const router = useRouter()

const loading = reactive({
  cards: false,
  charts: false,
  tables: false,
  alerts: false
})

const cardsData = ref({})
const chartsData = ref({})
const tablesData = ref({})
const physicalAssets = ref([])
const alertActiveTotal = ref(0)
const alertNameTop = ref([])
const alertRuleGroupTop = ref([])
const alertNotifyTrend = ref([])
const latestActiveAlerts = ref([])

const brandChartRef = ref(null)
const idcChartRef = ref(null)
const usageChartRef = ref(null)
const ageChartRef = ref(null)
const alertRuleGroupChartRef = ref(null)
const alertNotifyTrendChartRef = ref(null)

let brandChart = null
let idcChart = null
let usageChart = null
let ageChart = null
let alertRuleGroupChart = null
let alertNotifyTrendChart = null

const pick = (obj, keys, fallback = 0) => {
  for (const key of keys) {
    const value = obj?.[key]
    if (value !== undefined && value !== null && value !== '') return value
  }
  return fallback
}

const toArray = value => {
  if (Array.isArray(value)) return value
  if (Array.isArray(value?.list)) return value.list
  if (Array.isArray(value?.items)) return value.items
  if (Array.isArray(value?.data)) return value.data
  return []
}

const unwrap = response => response?.data?.data ?? response?.data ?? response ?? {}
const fmt = value => Number(value || 0).toLocaleString()
const percent = value => `${Number(value || 0).toFixed(1)}%`
const progressValue = value => Math.max(0, Math.min(100, Number(value || 0)))

const cards = computed(() => {
  const data = cardsData.value || {}
  const asset = data.asset || {}
  const location = data.location || {}
  const monitor = data.monitor || {}
  const warranty = data.warranty || {}
  const cloudHost = data.cloudHost || {}
  const auth = data.auth || {}

  const physical = pick(asset, ['physicalTotal', 'physicalHosts', 'physicalHostCount', 'physicalMachine', 'physical', 'bareMetalCount'])
  const virtual = pick(asset, ['vmTotal', 'virtualHosts', 'virtualHostCount', 'virtualMachine', 'virtual', 'vmCount'])
  const switches = pick(asset, ['switchTotal', 'switches', 'switchCount', 'networkDevices', 'networkDeviceCount'])
  const idcs = pick(location, ['roomTotal', 'idcs', 'idcCount', 'rooms', 'roomCount'])
  const cabinets = pick(location, ['cabinetTotal', 'cabinets', 'cabinetCount', 'racks', 'rackCount'])
  const hardwareCoverage = pick(monitor, ['hardwareCoverageRate', 'hardwareMonitorCoverage', 'hardwareCoverage'])
  const systemCoverage = pick(monitor, ['systemCoverageRate', 'systemMonitorCoverage', 'systemCoverage'])
  const warrantyExpired = pick(warranty, ['expiredCount', 'warrantyExpired', 'warrantyExpiredCount', 'expiredWarrantyCount'])
  const warrantyExpiring = pick(warranty, ['expiringSoonCount', 'dueCount', 'warrantyExpiring', 'warrantyExpiringCount', 'warrantyExpireSoonCount'])
  const aliyun = pick(cloudHost, ['aliyunTotal', 'aliyun', 'aliyunCount', 'aliCloudCount'])
  const tencent = pick(cloudHost, ['tencentTotal', 'tencent', 'tencentCount', 'tencentCloudCount'])
  const authRate = pick(auth, ['authRate', 'authenticationRate', 'credentialRate'])

  return [
    { title: '物理 / 虚拟 / 交换机', value: `${fmt(physical)} / ${fmt(virtual)} / ${fmt(switches)}`, desc: `总资产 ${fmt(pick(asset, ['total'], Number(physical) + Number(virtual) + Number(switches)))}`, icon: Monitor, className: 'blue', route: '/cmdb/ecs' },
    { title: '机房 / 机柜', value: `${fmt(idcs)} / ${fmt(cabinets)}`, desc: '基础设施位置', icon: OfficeBuilding, className: 'cyan' },
    { title: '硬件监控覆盖率', value: percent(hardwareCoverage), desc: `${fmt(pick(monitor, ['hardwareCovered']))} / ${fmt(pick(monitor, ['hardwareTotal']))} 已覆盖`, icon: Cpu, className: 'green', progress: hardwareCoverage },
    { title: '系统监控覆盖率', value: percent(systemCoverage), desc: `${fmt(pick(monitor, ['systemCovered']))} / ${fmt(pick(monitor, ['systemTotal']))} 已覆盖`, icon: TrendCharts, className: 'purple', progress: systemCoverage },
    { title: '维保到期', value: `${fmt(warrantyExpiring)} / ${fmt(warrantyExpired)}`, desc: `${pick(warranty, ['windowDays'], 90)} 天内 / 已过期`, icon: Clock, className: 'orange' },
    { title: '阿里云 / 腾讯云', value: `${fmt(aliyun)} / ${fmt(tencent)}`, desc: `云主机总数 ${fmt(pick(cloudHost, ['total'], Number(aliyun) + Number(tencent)))}`, icon: Cloudy, className: 'red' },
    { title: '认证率', value: percent(authRate), desc: `${fmt(pick(auth, ['authenticatedTotal']))} / ${fmt(pick(auth, ['hostTotal']))} 已认证`, icon: Key, className: 'indigo', progress: authRate }
  ]
})

const chartList = computed(() => {
  const data = chartsData.value || {}
  return {
    brands: toArray(pick(data, ['brandDistribution', 'brands', 'brand'], [])),
    idcs: toArray(pick(data, ['roomPhysicalAssets', 'idcPhysicalHosts', 'idcPhysicalHostCounts', 'idcPhysicalMachineCount', 'idcs'], [])),
    usages: toArray(pick(data, ['purposeDistribution', 'usageDistribution', 'usages', 'purposes'], []))
  }
})

const parseDate = value => {
  if (!value) return null
  const date = value instanceof Date ? value : new Date(String(value).replace(/-/g, '/'))
  return Number.isNaN(date.getTime()) ? null : date
}

const usageYearsFromPurchaseDate = (purchaseDate, now = new Date()) => {
  const date = parseDate(purchaseDate)
  if (!date || date > now) return null
  return (now.getTime() - date.getTime()) / (365.25 * 24 * 60 * 60 * 1000)
}

const physicalAgeDistribution = computed(() => {
  const buckets = [
    { name: '1-3年', value: 0 },
    { name: '3-5年', value: 0 },
    { name: '5-10年', value: 0 },
    { name: '超过10年', value: 0 }
  ]

  physicalAssets.value.forEach(asset => {
    const years = usageYearsFromPurchaseDate(pick(asset, ['purchaseDate', 'purchase_date'], ''))
    if (years === null || years < 1) return
    if (years < 3) buckets[0].value += 1
    else if (years < 5) buckets[1].value += 1
    else if (years < 10) buckets[2].value += 1
    else buckets[3].value += 1
  })

  return buckets
})

const tables = computed(() => {
  const data = tablesData.value || {}
  const highestUsage = data.highestUsage || {}
  const lowestUsage = data.lowestUsage || {}
  return {
    cpuHigh: toArray(pick(highestUsage, ['cpu'], pick(data, ['cpuHighestTop', 'cpuHighTop', 'cpuTop', 'cpuHighest'], []))),
    cpuLow: toArray(pick(lowestUsage, ['cpu'], pick(data, ['cpuLowestTop', 'cpuLowTop', 'cpuLowest'], []))),
    memoryHigh: toArray(pick(highestUsage, ['memory'], pick(data, ['memoryHighestTop', 'memoryHighTop', 'memoryTop', 'memHighestTop'], []))),
    memoryLow: toArray(pick(lowestUsage, ['memory'], pick(data, ['memoryLowestTop', 'memoryLowTop', 'memLowestTop'], []))),
    diskHigh: toArray(pick(highestUsage, ['disk'], pick(data, ['diskHighestTop', 'diskHighTop', 'diskTop'], []))),
    diskLow: toArray(pick(lowestUsage, ['disk'], pick(data, ['diskLowestTop', 'diskLowTop'], [])))
  }
})

const normalizePie = rows => rows.map(item => ({
  name: pick(item, ['name', 'label', 'brand', 'vendor', 'usage', 'purpose', 'type'], '未知'),
  value: Number(pick(item, ['value', 'count', 'total', 'num'], 0))
})).filter(item => item.value > 0)

const normalizePieTopN = (rows, topN = 10) => {
  const data = normalizePie(rows).sort((prev, next) => next.value - prev.value)
  if (data.length <= topN) return data
  const topItems = data.slice(0, topN)
  const otherValue = data.slice(topN).reduce((sum, item) => sum + item.value, 0)
  return otherValue > 0 ? [...topItems, { name: '其他', value: otherValue }] : topItems
}

const normalizeBar = rows => rows.map(item => ({
  name: pick(item, ['roomName', 'name', 'label', 'idc', 'idcName'], '未知'),
  value: Number(pick(item, ['assetCount', 'value', 'count', 'total', 'physicalCount', 'hostCount'], 0))
}))

const metricValue = row => pick(row, ['value', 'usage', 'usageRate', 'percent', 'rate'], 0)
const hostName = row => pick(row, ['hostName', 'hostname', 'name', 'instanceName', 'ip'], '-')
const hostIp = row => pick(row, ['ip', 'privateIp', 'sshIp', 'publicIp'], '-')
const hostId = row => pick(row, ['hostId', 'id'], '')
const statusClass = status => ({ warning: 'warning', attention: 'attention', critical: 'critical' }[status] || 'normal')
const toNumber = value => {
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : 0
}
const formatPercent = value => `${toNumber(value).toFixed(2)}%`
const alertDashboardParams = () => ({ range: 'today', limit: 5 })
const unwrapAlertPayload = response => {
  const first = response?.data ?? response ?? {}
  if (first && typeof first === 'object' && 'code' in first && 'data' in first) {
    return first.data ?? {}
  }
  return first
}

const extractAlertArray = (source, keys = []) => {
  const payload = unwrapAlertPayload(source)
  if (Array.isArray(payload)) return payload
  if (!payload || typeof payload !== 'object') return []

  const readArray = value => {
    if (Array.isArray(value)) return value
    if (!value || typeof value !== 'object') return null
    return ['items', 'list', 'rows', 'records', 'data'].map(key => value[key]).find(Array.isArray) || null
  }

  for (const key of keys) {
    const value = readArray(payload[key])
    if (value) return value
  }
  return readArray(payload) || []
}

const withAlertPercent = rows => {
  const total = rows.reduce((sum, item) => sum + toNumber(item.count), 0)
  return rows.map(item => ({
    ...item,
    percent: item.percent === undefined || item.percent === null || item.percent === ''
      ? (total ? (toNumber(item.count) / total) * 100 : 0)
      : toNumber(item.percent)
  }))
}

const normalizeAlertNameTop = rows => withAlertPercent(rows.map((item, index) => {
  const name = pick(item, ['name', 'ruleName', 'alertName', 'RuleName', 'AlertName'], '未命名告警')
  return {
    key: `alert-name-${pick(item, ['ruleId', 'ruleID', 'id', 'ID'], index)}-${name}`,
    name,
    count: toNumber(pick(item, ['count', 'total', 'eventCount', 'value', 'Count'], 0)),
    percent: pick(item, ['percent', 'ratio', 'Percent'], undefined)
  }
})).sort((left, right) => right.count - left.count).slice(0, 5)

const normalizeAlertRuleGroupTop = rows => withAlertPercent(rows.map((item, index) => {
  const groupName = pick(item, ['ruleGroupName', 'groupName', 'name', 'hostGroupName', 'GroupName'], '未分组')
  return {
    key: `alert-rule-group-${pick(item, ['ruleGroupId', 'groupId', 'id', 'ID'], index)}-${groupName}`,
    groupName,
    count: toNumber(pick(item, ['count', 'total', 'eventCount', 'value', 'Count'], 0)),
    percent: pick(item, ['percent', 'ratio', 'Percent'], undefined)
  }
})).sort((left, right) => right.count - left.count).slice(0, 5)

const normalizeAlertNotifyTrend = rows => rows.map((item, index) => ({
  key: `notify-trend-${pick(item, ['date', 'day', 'time', 'createdAt'], index)}`,
  date: pick(item, ['date', 'day', 'time', 'createdAt'], '-'),
  notificationCount: toNumber(pick(item, ['notificationCount', 'notifyCount', 'count', 'NotificationCount'], 0)),
  eventCount: toNumber(pick(item, ['eventCount', 'alertCount', 'EventCount'], 0)),
  successCount: toNumber(pick(item, ['successCount', 'SuccessCount'], 0)),
  failedCount: toNumber(pick(item, ['failedCount', 'failureCount', 'FailedCount'], 0))
}))

const normalizeLatestActiveAlerts = rows => (Array.isArray(rows) ? rows : []).map((item, index) => {
  const name = pick(item, ['ruleName', 'name', 'alertName', 'RuleName', 'AlertName'], '未命名告警')
  return {
    key: `latest-alert-${pick(item, ['id', 'eventId', 'fingerprint'], index)}-${name}`,
    name,
    severity: pick(item, ['severity', 'level', 'priority'], ''),
    source: pick(item, ['datasourceType', 'source', 'datasourceName'], '-'),
    instance: pick(item, ['businessId', 'instance', 'host', 'resource'], '-'),
    time: pick(item, ['lastEvaluatedAt', 'updatedAt', 'startsAt', 'createdAt', 'time'], '')
  }
}).slice(0, 5)

const alertNotifySummary = computed(() => alertNotifyTrend.value.reduce((acc, item) => {
  acc.notifications += item.notificationCount
  acc.events += item.eventCount
  acc.successes += item.successCount
  acc.failures += item.failedCount
  return acc
}, { notifications: 0, events: 0, successes: 0, failures: 0 }))

const goHostDetail = async row => {
  const id = hostId(row)
  if (!id) return

  try {
    const { data: res } = await cmdbApi.getCmdbHostById(id)
    const host = res?.data || row
    const targetId = pick(host, ['id', 'hostId'], id)
    const hostType = Number(host?.hostType ?? host?.host_type)
    const path = hostType === 2
      ? `/cmdb/host/windows-detail/${targetId}`
      : `/cmdb/host/detail/${targetId}`
    router.push(path)
  } catch (error) {
    console.error('跳转主机详情失败:', error)
    router.push(`/cmdb/host/detail/${id}`)
  }
}

const handleAssetCardClick = item => {
  if (!item?.route) return
  router.push(item.route)
}

const chartPalettes = {
  brand: ['#4f46e5', '#0ea5e9', '#14b8a6', '#8b5cf6', '#64748b', '#f59e0b', '#ef4444'],
  idc: [
    ['#0ea5e9', '#bae6fd'],
    ['#14b8a6', '#ccfbf1'],
    ['#6366f1', '#ddd6fe'],
    ['#22c55e', '#dcfce7'],
    ['#f59e0b', '#ffedd5']
  ],
  usage: ['#10b981', '#2563eb', '#f59e0b', '#14b8a6', '#8b5cf6', '#ef4444'],
  age: ['#10b981', '#0ea5e9', '#f59e0b', '#ef4444']
}

const barGradient = (start, end) => new echarts.graphic.LinearGradient(0, 0, 0, 1, [
  { offset: 0, color: start },
  { offset: 1, color: end }
])

const emptyChartGraphic = text => ({
  type: 'text',
  left: 'center',
  top: 'middle',
  style: {
    text,
    fill: '#94a3b8',
    fontSize: 12,
    fontWeight: 500
  }
})

const formatAlertAxisLabel = value => {
  const text = String(value || '-')
  return text.length > 8 ? `${text.slice(0, 8)}...` : text
}

const formatAlertTrendLabel = value => {
  const text = String(value || '')
  if (text.includes(' ')) return text.split(' ')[1]
  return text.length >= 10 ? text.slice(5) : text
}

const renderPie = (instance, rows, title, topN = 10, palette = chartPalettes.brand) => {
  const data = normalizePieTopN(rows, topN)
  instance.setOption({
    aria: { enabled: true },
    color: palette,
    tooltip: { trigger: 'item', confine: true },
    legend: { type: 'scroll', bottom: 0, textStyle: { color: '#64748b', fontSize: 11 }, itemWidth: 10, itemHeight: 10 },
    graphic: data.length ? [] : [emptyChartGraphic('暂无分布数据')],
    series: [{
      name: title,
      type: 'pie',
      radius: ['50%', '72%'],
      center: ['50%', '42%'],
      avoidLabelOverlap: true,
      minShowLabelAngle: 10,
      itemStyle: { borderColor: '#ffffff', borderWidth: 2 },
      label: { formatter: '{b}\n{d}%', fontSize: 11, color: '#475569' },
      labelLine: { length: 8, length2: 6 },
      data
    }]
  })
}

const renderBar = (instance, rows) => {
  const data = normalizeBar(rows)
  instance.setOption({
    aria: { enabled: true },
    tooltip: { trigger: 'axis', confine: true, axisPointer: { type: 'shadow' } },
    grid: { left: 36, right: 16, top: 24, bottom: 42 },
    graphic: data.length ? [] : [emptyChartGraphic('暂无机房资产数据')],
    xAxis: {
      type: 'category',
      data: data.map(item => item.name),
      axisLabel: { color: '#64748b', fontSize: 11, interval: 0, rotate: data.length > 5 ? 25 : 0 },
      axisLine: { lineStyle: { color: '#dbe7f4' } },
      axisTick: { alignWithLabel: true, lineStyle: { color: '#dbe7f4' } }
    },
    yAxis: {
      type: 'value',
      splitLine: { lineStyle: { color: '#edf2f7' } },
      axisLabel: { color: '#64748b', fontSize: 11 }
    },
    series: [{
      type: 'bar',
      data: data.map((item, index) => {
        const color = chartPalettes.idc[index % chartPalettes.idc.length]
        return {
          value: item.value,
          itemStyle: {
            color: barGradient(color[0], color[1]),
            borderRadius: [6, 6, 0, 0]
          }
        }
      }),
      barWidth: 22,
      itemStyle: { borderRadius: [6, 6, 0, 0] }
    }]
  })
}

const renderAlertRuleGroupChart = () => {
  if (!alertRuleGroupChartRef.value) return
  alertRuleGroupChart ||= echarts.init(alertRuleGroupChartRef.value)
  if (!alertRuleGroupTop.value.length) {
    alertRuleGroupChart.clear()
    return
  }

  alertRuleGroupChart.setOption({
    color: ['#f4c430'],
    tooltip: {
      trigger: 'axis',
      confine: true,
      axisPointer: { type: 'shadow' },
      formatter: params => {
        const item = params?.[0]
        if (!item) return ''
        return `${item.name}<br/>告警数量：${fmt(item.value)}`
      }
    },
    grid: { top: 18, right: 10, bottom: 42, left: 38 },
    xAxis: {
      type: 'category',
      data: alertRuleGroupTop.value.map(item => item.groupName),
      axisTick: { show: false },
      axisLine: { lineStyle: { color: '#dbe7f4' } },
      axisLabel: {
        color: '#64748b',
        fontSize: 11,
        interval: 0,
        rotate: 28,
        formatter: formatAlertAxisLabel
      }
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      splitLine: { lineStyle: { color: '#edf2f7', type: 'dashed' } },
      axisLabel: { color: '#64748b', fontSize: 11 }
    },
    series: [{
      name: '告警数量',
      type: 'bar',
      barMaxWidth: 28,
      data: alertRuleGroupTop.value.map(item => item.count),
      label: { show: true, position: 'top', color: '#172033', fontWeight: 800 },
      itemStyle: {
        borderRadius: [7, 7, 0, 0],
        color: barGradient('#f8d94a', '#d18a00')
      }
    }]
  }, true)
}

const renderAlertNotifyTrendChart = () => {
  if (!alertNotifyTrendChartRef.value) return
  alertNotifyTrendChart ||= echarts.init(alertNotifyTrendChartRef.value)
  if (!alertNotifyTrend.value.length) {
    alertNotifyTrendChart.clear()
    return
  }

  alertNotifyTrendChart.setOption({
    color: ['#d98a00', '#2563eb'],
    tooltip: {
      trigger: 'axis',
      confine: true,
      axisPointer: { type: 'line', lineStyle: { color: '#9aa8ba', type: 'dashed' } },
      formatter: params => {
        const item = alertNotifyTrend.value[params?.[0]?.dataIndex]
        if (!item) return ''
        return [
          `<strong>${item.date}</strong>`,
          `通知次数：${fmt(item.notificationCount)}`,
          `覆盖告警：${fmt(item.eventCount)}`,
          `成功 / 失败：${fmt(item.successCount)} / ${fmt(item.failedCount)}`
        ].join('<br/>')
      }
    },
    legend: {
      top: 0,
      right: 8,
      itemWidth: 10,
      itemHeight: 6,
      textStyle: { color: '#64748b', fontSize: 11 }
    },
    grid: { top: 32, right: 34, bottom: 28, left: 34 },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: alertNotifyTrend.value.map(item => item.date),
      axisTick: { show: false },
      axisLine: { lineStyle: { color: '#dbe7f4' } },
      axisLabel: { color: '#64748b', fontSize: 11, hideOverlap: true, formatter: formatAlertTrendLabel }
    },
    yAxis: [
      {
        type: 'value',
        min: 0,
        minInterval: 1,
        splitLine: { lineStyle: { color: '#edf2f7', type: 'dashed' } },
        axisLabel: { color: '#9a6a12', fontSize: 11 }
      },
      {
        type: 'value',
        min: 0,
        minInterval: 1,
        splitLine: { show: false },
        axisLabel: { color: '#2563eb', fontSize: 11 }
      }
    ],
    series: [
      {
        name: '通知次数',
        type: 'line',
        smooth: 0.25,
        showSymbol: alertNotifyTrend.value.length <= 12,
        symbolSize: 6,
        lineStyle: { width: 3 },
        data: alertNotifyTrend.value.map(item => item.notificationCount),
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
        showSymbol: alertNotifyTrend.value.length <= 12,
        symbolSize: 6,
        lineStyle: { width: 3 },
        data: alertNotifyTrend.value.map(item => item.eventCount)
      }
    ]
  }, true)
}

const renderAlertCharts = () => {
  renderAlertRuleGroupChart()
  renderAlertNotifyTrendChart()
}

const initCharts = () => {
  if (!brandChartRef.value || !idcChartRef.value || !usageChartRef.value || !ageChartRef.value) return
  brandChart ||= echarts.init(brandChartRef.value)
  idcChart ||= echarts.init(idcChartRef.value)
  usageChart ||= echarts.init(usageChartRef.value)
  ageChart ||= echarts.init(ageChartRef.value)
  renderPie(brandChart, chartList.value.brands, '品牌分布', 8, chartPalettes.brand)
  renderBar(idcChart, chartList.value.idcs)
  renderPie(usageChart, chartList.value.usages, '用途分布', 10, chartPalettes.usage)
  renderPie(ageChart, physicalAgeDistribution.value, '物理机使用年限', 4, chartPalettes.age)
}

const resizeCharts = () => {
  brandChart?.resize()
  idcChart?.resize()
  usageChart?.resize()
  ageChart?.resize()
  alertRuleGroupChart?.resize()
  alertNotifyTrendChart?.resize()
}

const loadCards = async () => {
  loading.cards = true
  try {
    cardsData.value = unwrap(await dashboardApi.getCmdbDashboardCards({ warrantyDays: 90 }))
  } finally {
    loading.cards = false
  }
}

const loadCharts = async () => {
  loading.charts = true
  try {
    const [chartPayload, assets] = await Promise.all([
      dashboardApi.getCmdbDashboardCharts(),
      loadPhysicalAssets()
    ])
    chartsData.value = unwrap(chartPayload)
    physicalAssets.value = assets
    await nextTick()
    initCharts()
  } finally {
    loading.charts = false
  }
}

const listPayload = payload => {
  const rows = toArray(payload)
  if (rows.length) return rows
  return toArray(unwrap(payload))
}

const pageTotal = payload => {
  const data = payload || {}
  return Number(pick(data, ['total', 'count', 'totalCount', 'total_count'], listPayload(data).length))
}

const loadPhysicalAssets = async () => {
  const pageSize = 200
  const first = unwrap(await cmdbApi.getCmdbIdcAssetList({ page: 1, pageSize, sortBy: 'id', sortOrder: 'desc' }))
  const firstRows = listPayload(first)
  const total = pageTotal(first)
  const pageCount = Math.ceil(total / pageSize)
  if (pageCount <= 1) return firstRows

  const requests = []
  for (let page = 2; page <= pageCount; page += 1) {
    requests.push(cmdbApi.getCmdbIdcAssetList({ page, pageSize, sortBy: 'id', sortOrder: 'desc' }))
  }
  const rest = await Promise.all(requests)
  return rest.reduce((rows, response) => rows.concat(listPayload(unwrap(response))), firstRows)
}

const loadTables = async () => {
  loading.tables = true
  try {
    tablesData.value = unwrap(await dashboardApi.getCmdbDashboardTables({ limit: 5, lookbackMinutes: 5 }))
  } finally {
    loading.tables = false
  }
}

const loadAlerts = async () => {
  loading.alerts = true
  try {
    const [chartsResponse, screenResponse, activeResponse, historyResponse] = await Promise.all([
      alertApi.getDashboardCharts(alertDashboardParams()),
      alertApi.getDashboardScreen(),
      alertApi.getActiveEvents({ page: 1, pageSize: 10 }),
      alertApi.getHistoryEvents({ page: 1, pageSize: 20 })
    ])
    const chartsPayload = unwrapAlertPayload(chartsResponse)
    const screenPayload = unwrapAlertPayload(screenResponse)
    const activeResult = normalizeListResult(activeResponse)
    const historyResult = normalizeListResult(historyResponse)

    alertNameTop.value = normalizeAlertNameTop(extractAlertArray(chartsPayload, ['alertNameTop5', 'alertNameTop', 'topAlertNames', 'topAlertName']))
    alertRuleGroupTop.value = normalizeAlertRuleGroupTop(extractAlertArray(chartsPayload, ['ruleGroupTop5', 'ruleGroupTop', 'alertRuleGroupTop5', 'alertRuleGroupTop', 'hostGroupTop5', 'hostGroupTop']))
    alertNotifyTrend.value = normalizeAlertNotifyTrend(extractAlertArray(chartsPayload, ['notifyTrend', 'notifyTrends', 'notificationTrend', 'notificationStats']))
    const latestRows = Array.isArray(screenPayload.latestActiveTop5) && screenPayload.latestActiveTop5.length
      ? screenPayload.latestActiveTop5
      : activeResult.list
    latestActiveAlerts.value = normalizeLatestActiveAlerts(latestRows)
    alertActiveTotal.value = toNumber(screenPayload.totalActive) || activeResult.total
    await nextTick()
    renderAlertCharts()
  } finally {
    loading.alerts = false
  }
}

const refreshAll = () => {
  loadCards()
  loadCharts()
  loadTables()
  loadAlerts()
}

const metricGroups = computed(() => [
  { title: 'CPU 使用率', highTitle: '最高 TOP', lowTitle: '最低 TOP', high: tables.value.cpuHigh, low: tables.value.cpuLow, unit: '%', className: 'cpu' },
  { title: '内存使用率', highTitle: '最高 TOP', lowTitle: '最低 TOP', high: tables.value.memoryHigh, low: tables.value.memoryLow, unit: '%', className: 'memory' },
  { title: '磁盘使用率', highTitle: '最高 TOP', lowTitle: '最低 TOP', high: tables.value.diskHigh, low: tables.value.diskLow, unit: '%', className: 'disk' }
])

onMounted(() => {
  refreshAll()
  window.addEventListener('resize', resizeCharts)
})

onUnmounted(() => {
  window.removeEventListener('resize', resizeCharts)
  brandChart?.dispose()
  idcChart?.dispose()
  usageChart?.dispose()
  ageChart?.dispose()
  alertRuleGroupChart?.dispose()
  alertNotifyTrendChart?.dispose()
})
</script>

<template>
  <div class="cmdb-dashboard-page">
    <div class="cmdb-workbench">
      <header class="hero-panel cmdb-hero-panel">
        <div class="hero-copy">
          <div class="hero-kicker">
            <el-icon><Monitor /></el-icon>
            CMDB 资源控制台
          </div>
          <h1>资产规模、监控覆盖与运行健康总览</h1>
        </div>
        <div class="hero-actions">
          <el-tag type="info" effect="light">维保窗口 90 天</el-tag>
          <el-tag type="primary" effect="plain">运行采样 5 分钟</el-tag>
          <el-button
            type="primary"
            :icon="Refresh"
            :loading="loading.cards || loading.charts || loading.tables || loading.alerts"
            @click="refreshAll"
          >
            刷新数据
          </el-button>
        </div>
      </header>

      <section class="asset-overview-grid" v-loading="loading.cards" aria-label="CMDB 核心指标">
        <article
          v-for="item in cards"
          :key="item.title"
          :class="['health-card', 'asset-summary-card', `is-${item.className}`, { 'asset-summary-card--link': item.route }]"
          :role="item.route ? 'button' : undefined"
          :tabindex="item.route ? 0 : undefined"
          :aria-label="item.route ? `${item.title}，查看主机管理列表` : item.title"
          @click="handleAssetCardClick(item)"
          @keyup.enter="handleAssetCardClick(item)"
          @keyup.space.prevent="handleAssetCardClick(item)"
        >
          <div class="asset-summary-icon" aria-hidden="true">
            <el-icon><component :is="item.icon" /></el-icon>
          </div>
          <div class="asset-summary-body">
            <span class="asset-summary-title">{{ item.title }}</span>
            <strong>{{ item.value }}</strong>
            <p>{{ item.desc }}</p>
            <el-progress
              v-if="item.progress !== undefined"
              class="asset-progress"
              :percentage="progressValue(item.progress)"
              :show-text="false"
              :stroke-width="5"
            />
          </div>
        </article>
      </section>

      <section class="workspace-grid">
        <div class="main-column">
          <section class="ops-card chart-panel" v-loading="loading.charts">
            <header class="card-header compact">
              <div>
                <h2>资产分布</h2>
              </div>
              <el-tag type="info" effect="plain">Top10 + 其他</el-tag>
            </header>
            <div class="chart-card-grid">
              <div class="chart-card chart-card--brand">
                <div class="chart-title">
                  <el-icon><PieChart /></el-icon>
                  品牌分布
                </div>
                <div ref="brandChartRef" class="chart-box"></div>
              </div>
              <div class="chart-card chart-card--idc">
                <div class="chart-title">
                  <el-icon><Histogram /></el-icon>
                  机房物理机数量
                </div>
                <div ref="idcChartRef" class="chart-box"></div>
              </div>
              <div class="chart-card chart-card--usage">
                <div class="chart-title">
                  <el-icon><DataAnalysis /></el-icon>
                  用途分布
                </div>
                <div ref="usageChartRef" class="chart-box"></div>
              </div>
              <div class="chart-card chart-card--age">
                <div class="chart-title">
                  <el-icon><Clock /></el-icon>
                  物理机使用年限
                </div>
                <div ref="ageChartRef" class="chart-box"></div>
              </div>
            </div>
          </section>
        </div>
      </section>

      <section class="ops-card alert-ops-panel" v-loading="loading.alerts">
        <header class="card-header compact">
          <div>
            <h2>告警态势</h2>
            <p>合并今日活跃告警、规则分组、通知趋势和最新风险，辅助资产视角快速判断处置优先级。</p>
          </div>
          <div class="alert-ops-actions">
            <el-tag type="danger" effect="light">活跃 {{ fmt(alertActiveTotal) }}</el-tag>
            <el-button size="small" type="primary" plain @click="router.push('/alert/view')">进入告警大盘</el-button>
          </div>
        </header>

        <div class="alert-ops-grid">
          <article class="alert-ops-card">
            <div class="alert-ops-card-title is-success">
              <el-icon><Connection /></el-icon>
              <span>告警群组 Top5</span>
            </div>
            <div v-if="alertRuleGroupTop.length" ref="alertRuleGroupChartRef" class="alert-ops-chart"></div>
            <div v-else class="empty-mini compact">暂无告警群组统计</div>
          </article>

          <article class="alert-ops-card">
            <div class="alert-ops-card-title is-danger">
              <el-icon><WarningFilled /></el-icon>
              <span>告警名称 Top5</span>
            </div>
            <div v-if="alertNameTop.length" class="alert-ops-table">
              <div class="alert-ops-table-head">
                <span>排名</span>
                <span>告警名称</span>
                <span>数量</span>
                <span>占比</span>
              </div>
              <div v-for="(item, index) in alertNameTop" :key="item.key" class="alert-ops-table-row">
                <span class="alert-ops-rank">{{ index + 1 }}</span>
                <span class="alert-ops-name" :title="item.name">{{ item.name }}</span>
                <strong>{{ fmt(item.count) }}</strong>
                <span>{{ formatPercent(item.percent) }}</span>
              </div>
            </div>
            <div v-else class="empty-mini compact">暂无告警名称统计</div>
          </article>

          <article class="alert-ops-card">
            <div class="alert-ops-card-title is-warning">
              <el-icon><BellFilled /></el-icon>
              <span>告警通知统计</span>
            </div>
            <div class="alert-ops-notify-strip">
              <span>通知 {{ fmt(alertNotifySummary.notifications) }}</span>
              <span>覆盖 {{ fmt(alertNotifySummary.events) }}</span>
              <span v-if="alertNotifySummary.failures" class="is-danger">失败 {{ fmt(alertNotifySummary.failures) }}</span>
            </div>
            <div v-if="alertNotifyTrend.length" ref="alertNotifyTrendChartRef" class="alert-ops-chart"></div>
            <div v-else class="empty-mini compact">暂无通知趋势数据</div>
          </article>

          <article class="alert-ops-card">
            <div class="alert-ops-card-title is-primary">
              <el-icon><TrendCharts /></el-icon>
              <span>最新活跃告警</span>
            </div>
            <div v-if="latestActiveAlerts.length" class="latest-ops-alert-list">
              <button
                v-for="(item, index) in latestActiveAlerts"
                :key="item.key"
                type="button"
                class="latest-ops-alert-row"
                @click="router.push('/alert/view')"
              >
                <span class="latest-ops-alert-index">{{ index + 1 }}</span>
                <span class="latest-ops-alert-main">
                  <strong>{{ item.name }}</strong>
                  <em>{{ item.source }} / {{ item.instance }}</em>
                </span>
                <el-tag :type="severityTagType(item.severity)" effect="light" size="small">{{ severityLabel(item.severity) }}</el-tag>
                <span class="latest-ops-alert-time">{{ formatDateTime(item.time) }}</span>
              </button>
            </div>
            <div v-else class="empty-mini compact">暂无活跃告警</div>
          </article>
        </div>
      </section>

      <section class="ops-card health-panel" v-loading="loading.tables">
        <header class="card-header compact">
          <div>
            <h2>运行健康</h2>
            <p>最近 5 分钟 CPU、内存、磁盘使用率高低位主机。</p>
          </div>
          <el-tag type="primary" effect="plain">可点击主机进入详情</el-tag>
        </header>
        <div class="metric-group-list">
          <div v-for="group in metricGroups" :key="group.title" :class="['metric-group-card', group.className]">
            <div class="metric-group-title">{{ group.title }}</div>
            <div class="metric-columns">
              <div class="metric-column">
                <div class="metric-column-title high">{{ group.highTitle }}</div>
                <div v-if="!group.high.length" class="empty-mini">暂无数据</div>
                <button
                  v-for="(row, index) in group.high"
                  :key="`high-${index}`"
                  type="button"
                  class="metric-row"
                  @click="goHostDetail(row)"
                >
                  <span class="rank-badge">{{ index + 1 }}</span>
                  <span class="metric-host" :title="`${hostName(row)} ${hostIp(row)}`">
                    <i :class="['status-dot', statusClass(row.status)]"></i>{{ hostName(row) }}
                  </span>
                  <span class="metric-value high">{{ Number(metricValue(row) || 0).toFixed(1) }}{{ group.unit }}</span>
                </button>
              </div>
              <div class="metric-column">
                <div class="metric-column-title low">{{ group.lowTitle }}</div>
                <div v-if="!group.low.length" class="empty-mini">暂无数据</div>
                <button
                  v-for="(row, index) in group.low"
                  :key="`low-${index}`"
                  type="button"
                  class="metric-row"
                  @click="goHostDetail(row)"
                >
                  <span class="rank-badge muted">{{ index + 1 }}</span>
                  <span class="metric-host" :title="`${hostName(row)} ${hostIp(row)}`">
                    <i :class="['status-dot', statusClass(row.status)]"></i>{{ hostName(row) }}
                  </span>
                  <span class="metric-value low">{{ Number(metricValue(row) || 0).toFixed(1) }}{{ group.unit }}</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped lang="scss">
.cmdb-dashboard-page {
  box-sizing: border-box;
  height: calc(100vh - 124px);
  min-height: 0;
  overflow: auto;
  padding: 7px;
  background: transparent;
  color: #172033;
}

:global(.home-container .el-main:has(.cmdb-dashboard-page)) {
  padding-top: 10px;
  padding-right: 10px;
  padding-left: 10px;
}

.cmdb-workbench {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.hero-panel {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  border: 1px solid #dbe7f4;
  border-radius: 12px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.97), rgba(248, 251, 255, 0.94)),
    #fff;
  box-shadow: 0 10px 28px rgba(80, 124, 180, 0.1);
}

.hero-copy {
  position: relative;
  max-width: 760px;
  padding-left: 44px;
}

.hero-kicker {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 2px;
  color: #5f718a;
  font-size: 12px;
  font-weight: 700;
}

.hero-kicker .el-icon {
  position: absolute;
  top: 0;
  left: 0;
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border: 1px solid #d5e4ff;
  border-radius: 10px;
  color: #2563eb;
  background: linear-gradient(180deg, #f5f9ff, #ecf4ff);
}

.hero-copy h1 {
  margin: 0;
  color: #172033;
  font-size: 20px;
  line-height: 1.2;
  letter-spacing: 0;
}

.hero-copy p {
  display: none;
  max-width: 680px;
  margin: 7px 0 0;
  color: #64748b;
  font-size: 13px;
  line-height: 1.6;
}

.hero-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  white-space: nowrap;
}

.asset-overview-grid {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 8px;
}

.health-card {
  min-width: 0;
  padding: 8px 11px;
  border: 1px solid #dbe7f4;
  border-radius: 10px;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(250, 253, 255, 0.96)),
    #fff;
  box-shadow: 0 8px 22px rgba(80, 124, 180, 0.06);
}

.asset-summary-card {
  display: grid;
  grid-template-columns: 36px minmax(0, 1fr);
  align-content: start;
  align-items: start;
  gap: 8px;
  min-height: 78px;
  transition: border-color 0.16s ease, box-shadow 0.16s ease, transform 0.16s ease;
}

.asset-summary-card--link {
  cursor: pointer;
}

.asset-summary-card--link:hover,
.asset-summary-card--link:focus-visible {
  border-color: #b9d4ff;
  box-shadow: 0 10px 24px rgba(64, 158, 255, 0.1);
  outline: none;
  transform: translateY(-1px);
}

.asset-summary-icon {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border: 1px solid #d5e4ff;
  border-radius: 10px;
  color: #2563eb;
  background: linear-gradient(180deg, #f5f9ff, #ecf4ff);
}

.asset-summary-icon .el-icon {
  font-size: 17px;
}

.asset-summary-body {
  min-width: 0;
}

.asset-summary-title {
  display: block;
  min-width: 0;
  overflow: hidden;
  color: #53657d;
  font-size: 10.5px;
  font-weight: 800;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.asset-summary-card strong {
  display: block;
  margin-top: 3px;
  color: #0f172a;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 17px;
  line-height: 1.05;
  word-break: break-word;
}

.asset-summary-card p {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 11px;
  line-height: 1.35;
}

.asset-progress {
  margin-top: 5px;
}

.asset-summary-card.is-blue {
  border-top: 3px solid #0ea5e9;
}

.asset-summary-card.is-blue .asset-summary-icon {
  border-color: #cfe0ff;
  color: #2563eb;
  background: #eef6ff;
}

.asset-summary-card.is-cyan {
  border-top: 3px solid #14b8a6;
}

.asset-summary-card.is-cyan .asset-summary-icon {
  border-color: #c7eee9;
  color: #0f766e;
  background: #e8fbf7;
}

.asset-summary-card.is-green {
  border-top: 3px solid #10b981;
}

.asset-summary-card.is-green .asset-summary-icon {
  border-color: #c7ead9;
  color: #15803d;
  background: #eaf8ef;
}

.asset-summary-card.is-purple {
  border-top: 3px solid #8b5cf6;
}

.asset-summary-card.is-purple .asset-summary-icon {
  border-color: #ddd6fe;
  color: #6d5dfc;
  background: #f1efff;
}

.asset-summary-card.is-orange {
  border-top: 3px solid #f59e0b;
}

.asset-summary-card.is-orange .asset-summary-icon {
  border-color: #ffdfb8;
  color: #ea8a00;
  background: #fff4e5;
}

.asset-summary-card.is-red {
  border-top: 3px solid #ef4444;
}

.asset-summary-card.is-red .asset-summary-icon {
  border-color: #ffd0d0;
  color: #dc2626;
  background: #fff0f0;
}

.asset-summary-card.is-indigo {
  border-top: 3px solid #6366f1;
}

.asset-summary-card.is-indigo .asset-summary-icon {
  border-color: #d8ddff;
  color: #4f46e5;
  background: #eef1ff;
}

.alert-ops-panel {
  display: grid;
}

.alert-ops-actions {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  white-space: nowrap;
}

.alert-ops-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.05fr) minmax(0, 1fr) minmax(0, 1.05fr) minmax(0, 1.15fr);
  gap: 8px;
  padding: 8px 10px 10px;
}

.alert-ops-card {
  position: relative;
  min-width: 0;
  min-height: 214px;
  padding: 8px 10px 10px;
  border: 1px solid #dce8f5;
  border-top-width: 3px;
  border-radius: 10px;
  background: #fbfdff;
}

.alert-ops-card:nth-child(1) {
  border-top-color: #10b981;
}

.alert-ops-card:nth-child(2) {
  border-top-color: #ef4444;
}

.alert-ops-card:nth-child(3) {
  border-top-color: #f59e0b;
}

.alert-ops-card:nth-child(4) {
  border-top-color: #2563eb;
}

.alert-ops-card-title {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 6px;
  color: #172033;
  font-size: 13px;
  font-weight: 800;
}

.alert-ops-card-title .el-icon {
  display: grid;
  width: 24px;
  height: 24px;
  place-items: center;
  border-radius: 8px;
  font-size: 14px;
}

.alert-ops-card-title.is-success .el-icon {
  color: #059669;
  background: #eaf8ef;
}

.alert-ops-card-title.is-danger .el-icon {
  color: #dc2626;
  background: #fff0f0;
}

.alert-ops-card-title.is-warning .el-icon {
  color: #d97706;
  background: #fff4e5;
}

.alert-ops-card-title.is-primary .el-icon {
  color: #2563eb;
  background: #eef6ff;
}

.alert-ops-chart {
  height: 160px;
}

.alert-ops-card > .empty-mini.compact {
  height: 160px;
}

.alert-ops-table {
  overflow: hidden;
  border: 1px solid #e3ebf5;
  border-radius: 9px;
  background: #fff;
}

.alert-ops-table-head,
.alert-ops-table-row {
  display: grid;
  grid-template-columns: 50px minmax(0, 1fr) 58px 66px;
  align-items: center;
  gap: 8px;
  min-height: 29px;
  padding: 0 8px;
  font-size: 12px;
}

.alert-ops-table-head {
  color: #6b7d95;
  font-weight: 800;
  background: #f3f7fc;
}

.alert-ops-table-row {
  border-top: 1px solid #eef3f9;
  color: #53657d;
}

.alert-ops-table-row strong {
  color: #172033;
  font-weight: 800;
}

.alert-ops-rank {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 19px;
  height: 19px;
  border-radius: 999px;
  color: #fff;
  font-size: 12px;
  font-weight: 800;
  background: #ef5f67;
}

.alert-ops-name {
  min-width: 0;
  overflow: hidden;
  color: #172033;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.alert-ops-notify-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 2px;
}

.alert-ops-notify-strip span {
  padding: 3px 7px;
  border-radius: 999px;
  color: #53657d;
  background: #f3f7fc;
  font-size: 11px;
  font-weight: 800;
}

.alert-ops-notify-strip .is-danger {
  color: #dc2626;
  background: #fff0f0;
}

.latest-ops-alert-list {
  display: grid;
  gap: 6px;
}

.latest-ops-alert-row {
  display: grid;
  grid-template-columns: 26px minmax(0, 1fr) auto 112px;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 31px;
  padding: 4px 7px;
  border: 1px solid #e3ebf5;
  border-radius: 9px;
  color: inherit;
  background: #fff;
  cursor: pointer;
  text-align: left;
  transition: border-color 0.16s ease, background-color 0.16s ease;
}

.latest-ops-alert-row:hover,
.latest-ops-alert-row:focus-visible {
  border-color: #b9d4ff;
  background: #f5f9ff;
  outline: none;
}

.latest-ops-alert-index {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 999px;
  color: #fff;
  background: #ef5f67;
  font-size: 12px;
  font-weight: 800;
}

.latest-ops-alert-main {
  min-width: 0;
}

.latest-ops-alert-main strong,
.latest-ops-alert-main em {
  display: block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.latest-ops-alert-main strong {
  color: #172033;
  font-size: 11.5px;
  font-weight: 800;
}

.latest-ops-alert-main em {
  margin-top: 1px;
  color: #7b8aa0;
  font-size: 11px;
  font-style: normal;
}

.latest-ops-alert-time {
  overflow: hidden;
  color: #64748b;
  font-size: 11px;
  text-align: right;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.workspace-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 12px;
  align-items: stretch;
}

.main-column {
  display: grid;
  min-width: 0;
}

.ops-card {
  min-width: 0;
  overflow: hidden;
  border: 1px solid #dbe7f4;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
}

.card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  border-bottom: 1px solid #e7eef7;
}

.card-header.compact {
  align-items: center;
}

.card-header h2 {
  margin: 0;
  color: #0f172a;
  font-size: 15px;
  line-height: 1.2;
}

.card-header p {
  margin: 3px 0 0;
  color: #6b7d95;
  font-size: 11.5px;
  line-height: 1.35;
}

.chart-card-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
  padding: 8px 10px 10px;
}

.chart-card {
  min-width: 0;
  padding: 8px 10px 6px;
  border: 1px solid #dce8f5;
  border-top-width: 3px;
  border-radius: 10px;
  background: #fbfdff;
}

.chart-card--brand {
  border-top-color: #6366f1;
}

.chart-card--idc {
  border-top-color: #0ea5e9;
}

.chart-card--usage {
  border-top-color: #10b981;
}

.chart-card--age {
  border-top-color: #f59e0b;
}

.chart-title {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 2px;
  color: #172033;
  font-size: 13px;
  font-weight: 700;
}

.chart-title .el-icon {
  display: grid;
  width: 24px;
  height: 24px;
  place-items: center;
  border: 1px solid #d5e4ff;
  border-radius: 8px;
  color: #2563eb;
  background: #eef6ff;
  font-size: 14px;
}

.chart-card--brand .chart-title .el-icon {
  border-color: #d8ddff;
  color: #4f46e5;
  background: #eef1ff;
}

.chart-card--idc .chart-title .el-icon {
  border-color: #bae6fd;
  color: #0284c7;
  background: #ecf8ff;
}

.chart-card--usage .chart-title .el-icon {
  border-color: #c7ead9;
  color: #15803d;
  background: #eaf8ef;
}

.chart-card--age .chart-title .el-icon {
  border-color: #ffdfb8;
  color: #d97706;
  background: #fff4e5;
}

.chart-box {
  height: clamp(172px, 20vh, 210px);
}

.health-panel {
  min-width: 0;
}

.metric-group-list {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  padding: 8px 10px 10px;
}

.metric-group-card {
  min-width: 0;
  padding: 10px;
  border: 1px solid #dce8f5;
  border-radius: 10px;
  background: #fff;
}

.metric-group-title {
  margin-bottom: 6px;
  font-size: 14px;
  font-weight: 800;
  color: #303133;
}

.metric-columns {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.metric-column {
  min-width: 0;
  padding: 8px;
  border: 1px solid #e3ebf5;
  border-radius: 9px;
  background: #f8fbff;
}

.metric-column-title {
  margin-bottom: 6px;
  font-size: 12px;
  font-weight: 700;
  color: #606266;
}

.metric-column-title.high {
  color: #f56c6c;
}

.metric-column-title.low {
  color: #67c23a;
}

.metric-row {
  display: grid;
  grid-template-columns: 24px minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 24px;
  padding: 0;
  border: 0;
  border-radius: 7px;
  color: inherit;
  background: transparent;
  cursor: pointer;
  font-size: 12px;
  text-align: left;
  transition: background-color 0.16s ease;
}

.metric-row:hover,
.metric-row:focus-visible {
  background: #eef6ff;
  outline: none;
}

.rank-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 999px;
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  background: #ef4444;
}

.rank-badge.muted {
  background: #10b981;
}

.metric-host {
  overflow: hidden;
  color: #303133;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-dot {
  display: inline-block;
  width: 7px;
  height: 7px;
  margin-right: 6px;
  border-radius: 999px;
  vertical-align: 1px;
  background: #67c23a;
}

.status-dot.attention {
  background: #f59e0b;
}

.status-dot.warning,
.status-dot.critical {
  background: #ef4444;
}

.metric-value {
  font-weight: 800;
}

.metric-value.high {
  color: #dc2626;
}

.metric-value.low {
  color: #059669;
}

.empty-mini {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 120px;
  color: #909399;
  font-size: 12px;
}

.empty-mini.compact {
  height: 118px;
  border: 1px dashed #d7e4f2;
  border-radius: 10px;
  background: #f8fbff;
}

:deep(.el-button:hover) {
  transform: none;
}

:deep(.el-button:active) {
  transform: translateY(1px);
}

:deep(.el-progress-bar__outer) {
  background: #e8eef6;
}

:deep(.el-progress-bar__inner) {
  background: #2563eb;
}

:deep(.el-loading-mask) {
  border-radius: inherit;
}

:deep(.el-table) {
  --el-table-border-color: #f0f2f5;
  --el-table-header-bg-color: #f8fafc;
  border-radius: 10px;
  overflow: hidden;
}

:deep(.el-table th) {
  font-size: 12px;
  color: #606266;
  font-weight: 700;
}

:deep(.el-table td) {
  font-size: 12px;
}

@media (prefers-reduced-motion: reduce) {
  .asset-summary-card,
  .latest-ops-alert-row,
  .metric-row {
    transition: none;
  }

  .asset-summary-card--link:hover,
  .asset-summary-card--link:focus-visible {
    transform: none;
  }
}

@media (max-width: 1500px) {
  .asset-overview-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .alert-ops-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .chart-card-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 1200px) {
  .workspace-grid {
    grid-template-columns: 1fr;
  }

  .metric-group-list {
    grid-template-columns: 1fr;
  }

}

@media (max-width: 960px) {
  .chart-card-grid,
  .alert-ops-grid,
  .metric-columns {
    grid-template-columns: 1fr;
  }

  .hero-panel,
  .card-header {
    flex-direction: column;
    align-items: stretch;
  }

  .hero-actions {
    flex-wrap: wrap;
    white-space: normal;
  }

  .alert-ops-actions {
    flex-wrap: wrap;
    white-space: normal;
  }

  .asset-overview-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 620px) {
  .cmdb-dashboard-page {
    padding: 6px;
  }

  .hero-copy {
    padding-left: 0;
  }

  .hero-kicker .el-icon {
    position: static;
    width: 34px;
    height: 34px;
  }

  .asset-overview-grid {
    grid-template-columns: 1fr;
  }

  .latest-ops-alert-row {
    grid-template-columns: 26px minmax(0, 1fr) auto;
  }

  .latest-ops-alert-time {
    display: none;
  }
}
</style>
