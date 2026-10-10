<!-- eslint-disable vue/multi-word-component-names -->
<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import * as echarts from 'echarts'
import {
  BellFilled,
  Clock,
  DataAnalysis,
  FullScreen,
  Monitor,
  OfficeBuilding,
  Refresh,
  WarningFilled
} from '@element-plus/icons-vue'
import CockpitGlobe from './components/CockpitGlobe.vue'
import { useCockpitData } from './composables/useCockpitData'
import { formatDateTime, severityLabel } from '@/views/alert/alertCommon'
import physicalIcon from '@/assets/image/物理机.svg'
import criticalIcon from '@/assets/image/严重.svg'
import monitoringIcon from '@/assets/image/监控.svg'
import idcIcon from '@/assets/image/机房.svg'
import cpuIcon from '@/assets/image/cpu.svg'
import memoryIcon from '@/assets/image/内存.svg'
import diskIcon from '@/assets/image/数据采集.svg'

const router = useRouter()
const cockpitRef = ref(null)
const trendChartRef = ref(null)
const brandChartRef = ref(null)
const healthChartRef = ref(null)
const now = ref(new Date())
const isFullscreen = ref(false)
const backgroundUrl = `${process.env.BASE_URL}tv-cockpit/assets/bg.80d620925f810a1b4c41.png`
const titleFrameUrl = `${process.env.BASE_URL}tv-cockpit/assets/title-background.bb83049388536b8a6c56.svg`
let clockTimer
let trendChart
let brandChart
let healthChart

const {
  activeAlerts,
  activeTotal,
  alertNameTop,
  cards,
  distributions,
  errorMessages,
  globeMarkers,
  hasData,
  isLoading,
  lastUpdatedAt,
  metrics,
  notificationTotal,
  notifyTrend,
  refresh
} = useCockpitData()

const fmt = value => Number(value || 0).toLocaleString('zh-CN')
const percent = value => `${Number(value || 0).toFixed(1)}%`
const clampPercent = value => Math.max(0, Math.min(100, Number(value || 0)))

const dateText = computed(() => new Intl.DateTimeFormat('zh-CN', {
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  weekday: 'short'
}).format(now.value))

const timeText = computed(() => new Intl.DateTimeFormat('zh-CN', {
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hour12: false
}).format(now.value))

const updatedText = computed(() => {
  if (!lastUpdatedAt.value) return '等待首次同步'
  return `更新于 ${new Intl.DateTimeFormat('zh-CN', {
    hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false
  }).format(lastUpdatedAt.value)}`
})

const headlineStats = computed(() => [
  { label: '资产总量', value: fmt(cards.value.total), unit: '台', iconUrl: physicalIcon, tone: 'cyan' },
  { label: '活跃告警', value: fmt(activeTotal.value), unit: '条', iconUrl: criticalIcon, tone: 'danger' },
  { label: '监控覆盖', value: percent(cards.value.systemCoverage), unit: '', iconUrl: monitoringIcon, tone: 'success' },
  { label: '机房节点', value: fmt(cards.value.idcs), unit: '个', iconUrl: idcIcon, tone: 'amber' }
])

const metricIcons = Object.freeze({ cpu: cpuIcon, memory: memoryIcon, disk: diskIcon })
const metricIcon = key => metricIcons[key] || monitoringIcon

const coverageState = computed(() => {
  const coverage = Number(cards.value.systemCoverage || 0)
  if (coverage >= 90) return { label: '运行稳定', tone: 'normal' }
  if (coverage >= 70) return { label: '需要关注', tone: 'warning' }
  return { label: '覆盖不足', tone: 'danger' }
})

const assetRows = computed(() => [
  { label: '物理主机', value: cards.value.physical, color: '#4dc9ff' },
  { label: '虚拟主机', value: cards.value.virtual, color: '#58e0ae' },
  { label: '网络设备', value: cards.value.switches, color: '#ffc857' },
  { label: '云上资源', value: cards.value.cloudTotal, color: '#a993ff' },
  { label: '机柜数量', value: cards.value.cabinets, color: '#ff8f70' }
])

const maxAssetValue = computed(() => Math.max(1, ...assetRows.value.map(item => Number(item.value || 0))))
const maxIdcValue = computed(() => Math.max(1, ...distributions.value.idcs.map(item => item.value)))
const markerAssetTotal = computed(() => globeMarkers.value.reduce((sum, item) => sum + item.value, 0))

const alertTone = severity => {
  const key = String(severity || '').toLowerCase()
  if (['critical', 'p0', 'fatal', 'severe'].includes(key)) return 'danger'
  if (['warning', 'warn', 'p1'].includes(key)) return 'warning'
  return 'info'
}

const metricTone = value => {
  const numeric = Number(value || 0)
  if (numeric >= 90) return 'danger'
  if (numeric >= 75) return 'warning'
  return 'normal'
}

const axisTimeLabel = value => {
  const text = String(value || '')
  if (text.includes(' ')) return text.split(' ')[1].slice(0, 5)
  return text.length > 10 ? text.slice(5, 10) : text
}

const emptyGraphic = text => ({
  type: 'text',
  left: 'center',
  top: 'middle',
  style: { text, fill: '#7893a8', fontSize: 12 }
})

const renderTrendChart = () => {
  if (!trendChartRef.value) return
  trendChart ||= echarts.init(trendChartRef.value)
  const rows = notifyTrend.value
  trendChart.setOption({
    animation: !window.matchMedia('(prefers-reduced-motion: reduce)').matches,
    aria: { enabled: true, description: '今日告警与通知趋势' },
    color: ['#45caff', '#ffc857'],
    tooltip: {
      trigger: 'axis',
      confine: true,
      backgroundColor: 'rgba(4, 18, 31, 0.96)',
      borderColor: '#285574',
      textStyle: { color: '#e6f7ff', fontSize: 12 }
    },
    legend: {
      top: 0,
      right: 0,
      itemWidth: 12,
      itemHeight: 4,
      textStyle: { color: '#90aabc', fontSize: 10 }
    },
    grid: { top: 28, right: 8, bottom: 23, left: 32 },
    graphic: rows.length ? [] : [emptyGraphic('暂无趋势数据')],
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: rows.map(item => item.date),
      axisLabel: { color: '#6f8ca2', fontSize: 9, hideOverlap: true, formatter: axisTimeLabel },
      axisLine: { lineStyle: { color: '#244359' } },
      axisTick: { show: false }
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      axisLabel: { color: '#6f8ca2', fontSize: 9 },
      splitLine: { lineStyle: { color: 'rgba(77, 134, 168, 0.14)', type: 'dashed' } }
    },
    series: [
      {
        name: '告警',
        type: 'line',
        smooth: 0.28,
        showSymbol: false,
        lineStyle: { width: 2, shadowBlur: 8, shadowColor: 'rgba(69, 202, 255, 0.55)' },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
            { offset: 0, color: 'rgba(69, 202, 255, 0.34)' },
            { offset: 0.62, color: 'rgba(69, 202, 255, 0.08)' },
            { offset: 1, color: 'rgba(69, 202, 255, 0.01)' }
          ])
        },
        data: rows.map(item => item.alerts)
      },
      {
        name: '通知',
        type: 'line',
        smooth: 0.28,
        showSymbol: false,
        lineStyle: { width: 2, type: 'dashed', shadowBlur: 6, shadowColor: 'rgba(255, 200, 87, 0.4)' },
        data: rows.map(item => item.notifications)
      }
    ]
  }, true)
}

const renderBrandChart = () => {
  if (!brandChartRef.value) return
  brandChart ||= echarts.init(brandChartRef.value)
  const rows = [...distributions.value.brands]
    .sort((left, right) => Number(right.value || 0) - Number(left.value || 0))
    .slice(0, 7)
  brandChart.setOption({
    animation: !window.matchMedia('(prefers-reduced-motion: reduce)').matches,
    animationDuration: 360,
    aria: { enabled: true, description: '资产品牌资产数量横向柱状图' },
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      confine: true,
      backgroundColor: 'rgba(4, 18, 31, 0.96)',
      borderColor: '#285574',
      textStyle: { color: '#e6f7ff', fontSize: 12 },
      formatter: params => `${params[0]?.name || '-'}<br>资产数量：${fmt(params[0]?.value)}`
    },
    grid: { top: 12, right: 40, bottom: 9, left: 10, containLabel: true },
    graphic: rows.length ? [] : [emptyGraphic('暂无品牌数据')],
    xAxis: {
      type: 'value',
      minInterval: 1,
      axisLabel: { show: false },
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { show: false }
    },
    yAxis: {
      type: 'category',
      inverse: true,
      data: rows.map(item => item.name),
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: {
        color: '#9cb8c7',
        fontSize: 10,
        width: 72,
        overflow: 'truncate'
      }
    },
    series: [{
      name: '资产数量',
      type: 'bar',
      barWidth: 8,
      showBackground: true,
      backgroundStyle: { color: 'rgba(63, 118, 143, 0.16)', borderRadius: 4 },
      itemStyle: {
        borderRadius: 4,
        color: new echarts.graphic.LinearGradient(0, 0, 1, 0, [
          { offset: 0, color: '#2c8eff' },
          { offset: 0.55, color: '#45caff' },
          { offset: 1, color: '#58e0ae' }
        ]),
        shadowBlur: 7,
        shadowColor: 'rgba(69, 202, 255, 0.32)'
      },
      label: {
        show: true,
        position: 'right',
        color: '#dff7ff',
        fontFamily: 'Consolas',
        fontSize: 10,
        formatter: params => fmt(params.value)
      },
      data: rows.map(item => Number(item.value || 0))
    }]
  }, true)
}

const renderHealthChart = () => {
  if (!healthChartRef.value) return
  healthChart ||= echarts.init(healthChartRef.value)
  const hardwareCoverage = clampPercent(cards.value.hardwareCoverage)
  const systemCoverage = clampPercent(cards.value.systemCoverage)
  const healthScore = Math.round((hardwareCoverage + systemCoverage) / 2)
  const animation = !window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const gaugeSeries = (name, value, radius, width, color) => ({
    name,
    type: 'gauge',
    radius,
    center: ['50%', '47%'],
    startAngle: 90,
    endAngle: -270,
    min: 0,
    max: 100,
    pointer: { show: false },
    progress: {
      show: true,
      roundCap: true,
      width,
      itemStyle: { color, shadowBlur: animation ? 8 : 0, shadowColor: color }
    },
    axisLine: { lineStyle: { width, color: [[1, 'rgba(79, 132, 157, 0.18)']] } },
    axisTick: { show: false },
    splitLine: { show: false },
    axisLabel: { show: false },
    anchor: { show: false },
    title: { show: false },
    detail: { show: false },
    data: [{ value, name }]
  })

  healthChart.setOption({
    animation,
    animationDuration: 480,
    aria: {
      enabled: true,
      description: `资产监控健康度 ${healthScore}%，硬件覆盖率 ${hardwareCoverage}%，系统覆盖率 ${systemCoverage}%`
    },
    graphic: [
      {
        type: 'text',
        left: 'center',
        top: '35%',
        style: {
          text: `${healthScore}%`,
          textAlign: 'center',
          fill: '#e8f9ff',
          font: '700 18px Consolas'
        }
      },
      {
        type: 'text',
        left: 'center',
        top: '53%',
        style: {
          text: '综合覆盖',
          textAlign: 'center',
          fill: '#7899aa',
          font: '9px Microsoft YaHei'
        }
      }
    ],
    series: [
      gaugeSeries('硬件覆盖', hardwareCoverage, '84%', 7, '#45caff'),
      gaugeSeries('系统覆盖', systemCoverage, '62%', 6, '#58e0ae')
    ]
  }, true)
}

const resizeCharts = () => {
  trendChart?.resize()
  brandChart?.resize()
  healthChart?.resize()
}

const handleFullscreenChange = () => {
  isFullscreen.value = Boolean(document.fullscreenElement)
  window.setTimeout(resizeCharts, 120)
}

const toggleFullscreen = async () => {
  try {
    if (document.fullscreenElement) await document.exitFullscreen()
    else if (cockpitRef.value?.requestFullscreen) await cockpitRef.value.requestFullscreen()
    else if (cockpitRef.value?.webkitRequestFullscreen) cockpitRef.value.webkitRequestFullscreen()
  } catch (error) {
    console.error('切换全屏失败:', error)
  }
}

const refreshDashboard = async () => {
  await refresh()
  await nextTick()
  renderTrendChart()
  renderBrandChart()
  renderHealthChart()
}

watch(notifyTrend, () => nextTick(renderTrendChart), { deep: true })
watch(() => distributions.value.brands, () => nextTick(renderBrandChart), { deep: true })
watch(cards, () => nextTick(renderHealthChart), { deep: true })

onMounted(() => {
  clockTimer = window.setInterval(() => { now.value = new Date() }, 1000)
  window.addEventListener('resize', resizeCharts)
  document.addEventListener('fullscreenchange', handleFullscreenChange)
  refreshDashboard()
})

onBeforeUnmount(() => {
  window.clearInterval(clockTimer)
  window.removeEventListener('resize', resizeCharts)
  document.removeEventListener('fullscreenchange', handleFullscreenChange)
  trendChart?.dispose()
  brandChart?.dispose()
  healthChart?.dispose()
})
</script>

<template>
  <main
    ref="cockpitRef"
    class="ops-cockpit"
    :style="{
      '--cockpit-background-image': `url('${backgroundUrl}')`,
      '--cockpit-title-frame': `url('${titleFrameUrl}')`
    }"
  >
    <div class="ops-cockpit__background" aria-hidden="true"></div>

    <header class="cockpit-header">
      <div class="cockpit-header__status">
        <span class="live-dot"></span>
        <span>系统在线</span>
        <small>{{ updatedText }}</small>
      </div>
      <div class="cockpit-title">
        <span class="cockpit-title__line"></span>
        <div>
          <h1>AutoOps 天枢 · 智能运维驾驶舱</h1>
          <p>AIOPS INTELLIGENT OPERATIONS CENTER</p>
        </div>
        <span class="cockpit-title__line is-right"></span>
      </div>
      <div class="cockpit-header__tools">
        <div class="cockpit-clock">
          <strong>{{ timeText }}</strong>
          <span>{{ dateText }}</span>
        </div>
        <el-tooltip content="刷新数据" placement="bottom">
          <button class="cockpit-icon-button" type="button" :disabled="isLoading" aria-label="刷新驾驶舱数据" @click="refreshDashboard">
            <el-icon :class="{ 'is-spinning': isLoading }"><Refresh /></el-icon>
          </button>
        </el-tooltip>
        <el-tooltip :content="isFullscreen ? '退出全屏' : '进入全屏'" placement="bottom">
          <button class="cockpit-icon-button" type="button" :aria-label="isFullscreen ? '退出全屏' : '进入全屏'" @click="toggleFullscreen">
            <el-icon><FullScreen /></el-icon>
          </button>
        </el-tooltip>
      </div>
    </header>

    <div v-if="errorMessages.length" class="cockpit-error" role="status">
      <el-icon><WarningFilled /></el-icon>
      <span>部分数据加载失败，已保留其余可用模块</span>
      <button type="button" @click="refreshDashboard">重新加载</button>
    </div>

    <div class="cockpit-layout">
      <aside class="cockpit-column cockpit-column--left">
        <section class="cockpit-panel alert-panel">
          <header class="panel-heading">
            <span><el-icon><BellFilled /></el-icon>实时告警</span>
            <button type="button" @click="router.push('/alert/event')">查看全部 {{ fmt(activeTotal) }}</button>
          </header>
          <div v-if="activeAlerts.length" class="alert-list">
            <button
              v-for="item in activeAlerts"
              :key="item.key"
              class="alert-row"
              type="button"
              @click="router.push('/alert/event')"
            >
              <span class="alert-row__level" :class="`is-${alertTone(item.severity)}`">{{ severityLabel(item.severity) }}</span>
              <span class="alert-row__main">
                <strong :title="item.name">{{ item.name }}</strong>
                <small :title="`${item.source} / ${item.instance}`">{{ item.source }} · {{ item.instance }}</small>
              </span>
              <time>{{ formatDateTime(item.time).slice(11, 16) || '-' }}</time>
            </button>
          </div>
          <div v-else class="panel-empty"><span></span>{{ isLoading ? '正在读取告警' : '当前没有活跃告警' }}</div>
        </section>

        <section class="cockpit-panel trend-panel">
          <header class="panel-heading">
            <span><el-icon><DataAnalysis /></el-icon>今日告警趋势</span>
            <em>通知 {{ fmt(notificationTotal) }}</em>
          </header>
          <div ref="trendChartRef" class="panel-chart" aria-label="今日告警趋势图"></div>
        </section>

        <section class="cockpit-panel top-panel">
          <header class="panel-heading">
            <span><el-icon><WarningFilled /></el-icon>告警规则 TOP5</span>
            <em>今日</em>
          </header>
          <div v-if="alertNameTop.length" class="rank-list">
            <div v-for="(item, index) in alertNameTop" :key="item.key" class="rank-row">
              <b>{{ String(index + 1).padStart(2, '0') }}</b>
              <span :title="item.name">{{ item.name }}</span>
              <i><span :style="{ width: `${clampPercent(item.percent)}%` }"></span></i>
              <strong>{{ fmt(item.count) }}</strong>
            </div>
          </div>
          <div v-else class="panel-empty"><span></span>暂无排行数据</div>
        </section>
      </aside>

      <section class="cockpit-center">
        <div class="headline-stats">
          <article v-for="item in headlineStats.slice(0, 2)" :key="item.label" class="headline-stat" :class="`is-${item.tone}`">
            <span class="headline-stat__icon" aria-hidden="true"><img :src="item.iconUrl" alt=""></span>
            <span>{{ item.label }}</span>
            <strong>{{ item.value }}<small>{{ item.unit }}</small></strong>
          </article>
          <article class="headline-radar">
            <div class="headline-radar__body">
              <div class="radar-unit" :class="`is-${coverageState.tone}`" :aria-label="`系统监控覆盖率 ${percent(cards.systemCoverage)}，${coverageState.label}`">
                <div class="radar-scope" aria-hidden="true">
                  <span class="radar-sweep"></span>
                  <i class="radar-point is-one"></i>
                  <i class="radar-point is-two"></i>
                  <i class="radar-point is-three"></i>
                  <strong>{{ Math.round(clampPercent(cards.systemCoverage)) }}%</strong>
                </div>
                <span>监控</span>
                <small>{{ coverageState.label }}</small>
              </div>
              <div class="radar-unit is-resource" :aria-label="`当前纳管资源 ${fmt(cards.total)} 台`">
                <div class="radar-scope" aria-hidden="true">
                  <span class="radar-sweep"></span>
                  <i class="radar-point is-one"></i>
                  <i class="radar-point is-two"></i>
                  <strong>{{ fmt(cards.total) }}</strong>
                </div>
                <span>资源</span>
                <small>{{ fmt(cards.idcs) }} 个节点</small>
              </div>
            </div>
          </article>
          <article v-for="item in headlineStats.slice(2)" :key="item.label" class="headline-stat" :class="`is-${item.tone}`">
            <span class="headline-stat__icon" aria-hidden="true"><img :src="item.iconUrl" alt=""></span>
            <span>{{ item.label }}</span>
            <strong>{{ item.value }}<small>{{ item.unit }}</small></strong>
          </article>
        </div>

        <div class="globe-stage">
          <div class="globe-stage__frame" aria-hidden="true">
            <span class="is-top-left"></span>
            <span class="is-top-right"></span>
            <span class="is-bottom-left"></span>
            <span class="is-bottom-right"></span>
          </div>
          <svg class="globe-stage__flow" viewBox="0 0 1000 500" preserveAspectRatio="none" aria-hidden="true">
            <path class="globe-stage__flow-base" pathLength="100" d="M18 1 H982 L999 18 V482 L982 499 H18 L1 482 V18 Z"></path>
            <path class="globe-stage__flow-line is-primary" pathLength="100" d="M18 1 H982 L999 18 V482 L982 499 H18 L1 482 V18 Z"></path>
            <path class="globe-stage__flow-line is-secondary" pathLength="100" d="M18 1 H982 L999 18 V482 L982 499 H18 L1 482 V18 Z"></path>
          </svg>
          <div class="globe-stage__heading">
            <span>RESOURCE SITUATION</span>
            <strong>资源空间态势</strong>
            <small>已匹配 {{ globeMarkers.length }} 个城市节点 · {{ fmt(markerAssetTotal) }} 台资产</small>
          </div>
          <CockpitGlobe :markers="globeMarkers" />
          <div class="globe-readouts globe-readouts--left">
            <div>
              <span>PHYSICAL HOST</span>
              <strong>{{ fmt(cards.physical) }}</strong>
              <small>物理主机</small>
            </div>
            <div>
              <span>MONITORING</span>
              <strong>{{ percent(cards.hardwareCoverage) }}</strong>
              <small>硬件覆盖率</small>
            </div>
          </div>
          <div class="globe-readouts globe-readouts--right">
            <div>
              <span>ACTIVE ALERT</span>
              <strong class="is-danger">{{ fmt(activeTotal) }}</strong>
              <small>活跃告警</small>
            </div>
            <div>
              <span>AUTH RATE</span>
              <strong>{{ percent(cards.authRate) }}</strong>
              <small>资产认证率</small>
            </div>
          </div>
          <div class="globe-core-label">
            <span>AUTOOPS ASSET NETWORK</span>
            <strong>{{ fmt(cards.total) }}</strong>
            <small>受管资产</small>
          </div>
          <div class="globe-stage__legend">
            <span><i class="is-idc"></i>机房节点</span>
            <span><i class="is-asset"></i>资产规模</span>
            <small>拖动旋转 · 滚轮缩放</small>
          </div>
          <div v-if="isLoading && !hasData" class="globe-loading" aria-live="polite">
            <span></span>正在同步运维数据
          </div>
        </div>

        <div class="metric-strip">
          <article v-for="group in metrics" :key="group.key" class="metric-card">
            <header>
              <span><img :src="metricIcon(group.key)" alt="" aria-hidden="true">{{ group.label }} 热点</span>
              <small>TOP {{ group.rows.length }}</small>
            </header>
            <div v-if="group.rows.length" class="metric-card__body">
              <div v-for="row in group.rows.slice(0, 3)" :key="row.key" class="metric-row">
                <span :title="`${row.name} ${row.ip}`">{{ row.name }}</span>
                <i><span :class="`is-${metricTone(row.value)}`" :style="{ width: `${clampPercent(row.value)}%` }"></span></i>
                <strong :class="`is-${metricTone(row.value)}`">{{ Number(row.value).toFixed(1) }}%</strong>
              </div>
            </div>
            <div v-else class="metric-empty">暂无监控数据</div>
          </article>
        </div>
      </section>

      <aside class="cockpit-column cockpit-column--right">
        <section class="cockpit-panel brand-panel">
          <header class="panel-heading">
            <span><el-icon><DataAnalysis /></el-icon>资产品牌构成</span>
            <em>TOP {{ Math.min(distributions.brands.length, 7) }}</em>
          </header>
          <div ref="brandChartRef" class="panel-chart" aria-label="资产品牌资产数量柱状图"></div>
        </section>

        <section class="cockpit-panel asset-panel">
          <header class="panel-heading">
            <span><el-icon><Monitor /></el-icon>资产资源总览</span>
            <em>共 {{ fmt(cards.total) }} 台</em>
          </header>
          <div class="asset-overview">
            <div class="asset-list">
              <div v-for="item in assetRows" :key="item.label" class="asset-row">
                <span>{{ item.label }}</span>
                <i><span :style="{ width: `${item.value / maxAssetValue * 100}%`, backgroundColor: item.color }"></span></i>
                <strong>{{ fmt(item.value) }}</strong>
              </div>
            </div>
            <div class="health-chart-wrap">
              <div ref="healthChartRef" class="health-chart" aria-label="资产监控覆盖率双环图"></div>
              <div class="health-chart-legend" aria-hidden="true">
                <span><i class="is-hardware"></i>硬件</span>
                <span><i class="is-system"></i>系统</span>
              </div>
            </div>
          </div>
        </section>

        <section class="cockpit-panel idc-panel">
          <header class="panel-heading">
            <span><el-icon><OfficeBuilding /></el-icon>机房资产分布</span>
            <em>{{ fmt(cards.idcs) }} 个机房</em>
          </header>
          <div v-if="distributions.idcs.length" class="idc-list">
            <div v-for="(item, index) in distributions.idcs.slice(0, 6)" :key="item.key" class="idc-row">
              <b>{{ String(index + 1).padStart(2, '0') }}</b>
              <span :title="item.name">{{ item.name }}</span>
              <i><span :style="{ width: `${item.value / maxIdcValue * 100}%` }"></span></i>
              <strong>{{ fmt(item.value) }}</strong>
            </div>
          </div>
          <div v-else class="panel-empty"><span></span>暂无机房分布数据</div>
          <footer class="risk-summary">
            <div><el-icon><Clock /></el-icon><span>即将过保</span><strong>{{ fmt(cards.expiring) }}</strong></div>
            <div><el-icon><WarningFilled /></el-icon><span>已经过保</span><strong>{{ fmt(cards.expired) }}</strong></div>
          </footer>
        </section>
      </aside>
    </div>
  </main>
</template>

<style scoped lang="scss">
.ops-cockpit {
  --cockpit-bg: #030a12;
  --cockpit-surface: rgba(6, 22, 35, 0.88);
  --cockpit-surface-strong: rgba(7, 28, 44, 0.94);
  --cockpit-border: rgba(75, 176, 222, 0.28);
  --cockpit-border-strong: rgba(79, 205, 255, 0.54);
  --cockpit-text: #e6f7ff;
  --cockpit-muted: #89a7ba;
  --cockpit-cyan: #45caff;
  --cockpit-green: #58e0ae;
  --cockpit-amber: #ffc857;
  --cockpit-red: #ff6f6f;
  position: relative;
  box-sizing: border-box;
  width: 100%;
  height: calc(100vh - 104px);
  min-height: 690px;
  overflow: hidden;
  color: var(--cockpit-text);
  font-family: "Segoe UI", "Microsoft YaHei", sans-serif;
  background: var(--cockpit-bg);
}

.ops-cockpit:fullscreen,
.ops-cockpit:-webkit-full-screen {
  width: 100vw;
  height: 100vh;
  min-height: 100vh;
}

:global(.home-container .el-main:has(.ops-cockpit)) {
  padding: 0 !important;
  overflow: hidden;
}

.ops-cockpit__background {
  position: absolute;
  inset: 0;
  z-index: 0;
  background:
    linear-gradient(90deg, rgba(2, 8, 14, 0.48), rgba(3, 13, 23, 0.08) 34%, rgba(3, 13, 23, 0.08) 66%, rgba(2, 8, 14, 0.52)),
    linear-gradient(180deg, rgba(0, 9, 17, 0.05), rgba(0, 7, 13, 0.7)),
    var(--cockpit-background-image) center / cover no-repeat;
}

.ops-cockpit__background::after {
  position: absolute;
  inset: 0;
  content: "";
  pointer-events: none;
  background-image:
    linear-gradient(rgba(70, 176, 221, 0.025) 1px, transparent 1px),
    linear-gradient(90deg, rgba(70, 176, 221, 0.025) 1px, transparent 1px);
  background-size: 40px 40px;
}

.cockpit-header {
  position: relative;
  z-index: 20;
  display: grid;
  grid-template-columns: 1fr minmax(440px, 1.45fr) 1fr;
  align-items: center;
  height: 70px;
  padding: 0 16px;
  border-bottom: 1px solid rgba(70, 185, 231, 0.18);
  background:
    var(--cockpit-title-frame) center bottom / 100% 56px no-repeat,
    linear-gradient(180deg, rgba(3, 13, 24, 0.98), rgba(2, 10, 18, 0.82));
}

.cockpit-header::after {
  position: absolute;
  right: 27%;
  bottom: -1px;
  left: 27%;
  height: 3px;
  content: "";
  background: linear-gradient(90deg, transparent, var(--cockpit-cyan), var(--cockpit-green), transparent);
}

.cockpit-header__status,
.cockpit-header__tools,
.cockpit-title,
.cockpit-title > div,
.panel-heading,
.panel-heading > span,
.headline-stat,
.metric-card header,
.risk-summary > div {
  display: flex;
  align-items: center;
}

.cockpit-header__status {
  gap: 8px;
  color: #b8d8e8;
  font-size: 12px;
}

.cockpit-header__status small {
  padding-left: 8px;
  border-left: 1px solid rgba(126, 177, 200, 0.24);
  color: #708da0;
}

.live-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--cockpit-green);
  box-shadow: 0 0 8px rgba(88, 224, 174, 0.74);
}

.cockpit-title {
  justify-content: center;
  gap: 12px;
  height: 64px;
  min-width: 0;
  text-align: center;
}

.cockpit-title > div {
  flex-direction: column;
  min-width: 0;
}

.cockpit-title h1 {
  margin: 0;
  overflow: hidden;
  color: #f0fbff;
  font-size: 25px;
  font-weight: 750;
  line-height: 1.25;
  letter-spacing: 0;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-shadow: 0 0 6px rgba(232, 250, 255, 0.34), 0 0 22px rgba(69, 202, 255, 0.46);
}

.cockpit-title p {
  margin: 2px 0 0;
  color: #69b8d7;
  font-family: Consolas, monospace;
  font-size: 9px;
  letter-spacing: 0;
}

.cockpit-title__line {
  width: clamp(38px, 5vw, 100px);
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(69, 202, 255, 0.8));
}

.cockpit-title__line.is-right {
  transform: rotate(180deg);
}

.cockpit-header__tools {
  justify-content: flex-end;
  gap: 8px;
}

.cockpit-clock {
  display: grid;
  margin-right: 6px;
  text-align: right;
}

.cockpit-clock strong {
  color: #dff6ff;
  font-family: Consolas, monospace;
  font-size: 18px;
  line-height: 1;
  font-variant-numeric: tabular-nums;
}

.cockpit-clock span {
  margin-top: 3px;
  color: #7997aa;
  font-size: 10px;
}

.cockpit-icon-button {
  display: grid;
  width: 34px;
  height: 34px;
  padding: 0;
  place-items: center;
  color: #a9d9ed;
  border: 1px solid rgba(72, 175, 218, 0.34);
  border-radius: 6px;
  outline: 0;
  cursor: pointer;
  background: rgba(12, 48, 68, 0.54);
  transition: color 160ms ease, border-color 160ms ease, background-color 160ms ease;
}

.cockpit-icon-button:hover {
  color: #fff;
  border-color: var(--cockpit-cyan);
  background: rgba(30, 118, 153, 0.55);
}

.cockpit-icon-button:focus-visible {
  box-shadow: 0 0 0 2px rgba(69, 202, 255, 0.35);
}

.cockpit-icon-button:disabled {
  opacity: 0.55;
  cursor: wait;
}

.is-spinning {
  animation: cockpit-spin 900ms linear infinite;
}

.cockpit-error {
  position: absolute;
  z-index: 40;
  top: 72px;
  left: 50%;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  color: #ffe1b8;
  border: 1px solid rgba(255, 179, 71, 0.42);
  border-radius: 6px;
  font-size: 12px;
  transform: translateX(-50%);
  background: rgba(61, 36, 12, 0.94);
}

.cockpit-error button,
.panel-heading button {
  padding: 0;
  color: var(--cockpit-cyan);
  border: 0;
  cursor: pointer;
  background: transparent;
}

.cockpit-layout {
  position: relative;
  z-index: 10;
  display: grid;
  grid-template-columns: minmax(260px, 22vw) minmax(430px, 1fr) minmax(260px, 22vw);
  gap: 8px;
  box-sizing: border-box;
  height: calc(100% - 70px);
  padding: 8px;
}

.cockpit-column {
  display: grid;
  gap: 8px;
  min-width: 0;
  min-height: 0;
}

.cockpit-column--left {
  grid-template-rows: minmax(210px, 1.25fr) minmax(170px, 0.85fr) minmax(190px, 0.9fr);
}

.cockpit-column--right {
  grid-template-rows: minmax(230px, 1.15fr) minmax(170px, 0.82fr) minmax(210px, 1fr);
}

.cockpit-panel {
  position: relative;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  border: 1px solid rgba(72, 186, 235, 0.46);
  border-radius: 0;
  clip-path: polygon(0 10px, 10px 0, calc(100% - 22px) 0, 100% 22px, 100% calc(100% - 8px), calc(100% - 8px) 100%, 8px 100%, 0 calc(100% - 8px));
  background:
    linear-gradient(135deg, rgba(39, 121, 164, 0.13), transparent 32%),
    linear-gradient(180deg, rgba(7, 29, 48, 0.96), rgba(4, 17, 31, 0.94));
  box-shadow: inset 0 0 28px rgba(28, 145, 202, 0.06), 0 10px 30px rgba(0, 3, 8, 0.18);
}

.cockpit-panel::before {
  position: absolute;
  top: -1px;
  left: 0;
  z-index: 2;
  width: 96px;
  height: 3px;
  content: "";
  background: linear-gradient(90deg, transparent, var(--cockpit-cyan) 38%, #effcff 52%, var(--cockpit-green) 68%, transparent);
  box-shadow: 0 0 9px rgba(69, 202, 255, 0.55);
  transform: translateX(-120px);
  animation: panel-edge-flow 7.2s linear infinite;
}

.panel-heading {
  position: relative;
  justify-content: space-between;
  gap: 10px;
  height: 39px;
  padding: 0 12px;
  border-bottom: 1px solid rgba(73, 175, 218, 0.28);
  background: linear-gradient(90deg, rgba(19, 81, 113, 0.48), rgba(7, 28, 45, 0.2) 64%, transparent);
}

.panel-heading::after {
  position: absolute;
  bottom: -1px;
  left: 12px;
  width: 54px;
  height: 2px;
  content: "";
  background: var(--cockpit-cyan);
  box-shadow: 0 0 8px rgba(69, 202, 255, 0.72);
}

.panel-heading > span {
  gap: 7px;
  min-width: 0;
  color: #cbefff;
  font-size: 13px;
  font-weight: 700;
  text-shadow: 0 0 10px rgba(69, 202, 255, 0.28);
}

.panel-heading .el-icon {
  color: var(--cockpit-cyan);
}

.panel-heading em,
.panel-heading button {
  flex: 0 0 auto;
  font-size: 10px;
  font-style: normal;
}

.panel-heading em {
  color: #708fa2;
}

.panel-heading button:hover {
  color: #fff;
}

.alert-list {
  display: grid;
  gap: 4px;
  height: calc(100% - 39px);
  padding: 7px;
  overflow: auto;
}

.alert-row {
  display: grid;
  grid-template-columns: 50px minmax(0, 1fr) 34px;
  align-items: center;
  gap: 7px;
  min-height: 42px;
  padding: 4px 6px;
  color: inherit;
  border: 1px solid rgba(51, 139, 181, 0.12);
  border-left-color: rgba(69, 202, 255, 0.72);
  border-radius: 2px;
  text-align: left;
  cursor: pointer;
  background: linear-gradient(90deg, rgba(24, 94, 127, 0.24), rgba(11, 36, 53, 0.16));
  transition: border-color 160ms ease, background-color 160ms ease;
}

.alert-row:hover,
.alert-row:focus-visible {
  border-color: rgba(75, 188, 234, 0.38);
  outline: 0;
  background: rgba(35, 94, 121, 0.28);
}

.alert-row__level {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 20px;
  padding: 0 5px;
  border-left: 2px solid currentColor;
  border-radius: 3px;
  font-size: 9px;
  white-space: nowrap;
}

.alert-row__level.is-danger { color: #ff8585; background: rgba(255, 82, 82, 0.12); }
.alert-row__level.is-warning { color: #ffd06a; background: rgba(255, 184, 56, 0.12); }
.alert-row__level.is-info { color: #66d9ff; background: rgba(69, 202, 255, 0.1); }

.alert-row__main {
  display: grid;
  min-width: 0;
}

.alert-row__main strong,
.alert-row__main small,
.rank-row > span,
.asset-row > span,
.idc-row > span,
.metric-row > span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.alert-row__main strong {
  color: #d7edf7;
  font-size: 11px;
  font-weight: 600;
}

.alert-row__main small,
.alert-row time {
  color: #6f8da0;
  font-size: 9px;
}

.panel-chart {
  width: 100%;
  height: calc(100% - 39px);
  min-height: 120px;
}

.rank-list,
.idc-list {
  display: grid;
  align-content: start;
  gap: 7px;
  box-sizing: border-box;
  height: calc(100% - 39px);
  padding: 14px 11px 8px;
}

.rank-row,
.idc-row,
.asset-row,
.metric-row {
  display: grid;
  align-items: center;
  gap: 7px;
  min-width: 0;
}

.rank-row {
  grid-template-columns: 20px minmax(64px, 1fr) minmax(48px, 0.75fr) 28px;
}

.rank-row b,
.idc-row b {
  color: #6d8fa5;
  font-family: Consolas, monospace;
  font-size: 10px;
}

.rank-row > span,
.idc-row > span,
.asset-row > span {
  color: #b9d1de;
  font-size: 10px;
}

.rank-row > i,
.idc-row > i,
.asset-row > i,
.metric-row > i {
  height: 4px;
  overflow: hidden;
  border-radius: 2px;
  background: rgba(83, 133, 157, 0.18);
}

.rank-row > i span,
.idc-row > i span,
.asset-row > i span,
.metric-row > i span {
  display: block;
  height: 100%;
  border-radius: inherit;
}

.rank-row > i span { background: var(--cockpit-red); }
.rank-row strong,
.idc-row strong,
.asset-row strong {
  color: #e5f7ff;
  font-family: Consolas, monospace;
  font-size: 11px;
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.cockpit-center {
  display: grid;
  grid-template-rows: 92px minmax(300px, 1fr) 158px;
  gap: 8px;
  min-width: 0;
  min-height: 0;
}

.headline-stats {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr)) minmax(168px, 1.24fr) repeat(2, minmax(0, 1fr));
  gap: 7px;
}

.headline-stat {
  position: relative;
  display: grid;
  grid-template-columns: 36px minmax(0, 1fr);
  grid-template-rows: 20px 24px;
  column-gap: 8px;
  min-width: 0;
  padding: 10px 11px;
  overflow: hidden;
  border: 1px solid rgba(74, 181, 226, 0.5);
  border-radius: 0;
  clip-path: polygon(0 8px, 8px 0, calc(100% - 16px) 0, 100% 16px, 100% 100%, 0 100%);
  background:
    linear-gradient(135deg, rgba(62, 178, 222, 0.16), transparent 48%),
    rgba(5, 24, 39, 0.94);
  box-shadow: inset 0 0 22px rgba(63, 187, 232, 0.07);
}

.headline-stat::after {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  height: 3px;
  content: "";
  background: linear-gradient(90deg, currentColor, #f2fdff, currentColor, transparent, currentColor);
  background-size: 260% 100%;
  opacity: 0.72;
  animation: edge-energy-flow 4.8s linear infinite;
}

.headline-stat__icon {
  grid-row: 1 / 3;
  align-self: center;
  display: grid;
  width: 34px;
  height: 34px;
  overflow: hidden;
  place-items: center;
  border: 1px solid currentColor;
  clip-path: polygon(0 7px, 7px 0, 100% 0, 100% calc(100% - 7px), calc(100% - 7px) 100%, 0 100%);
  background: radial-gradient(circle, color-mix(in srgb, currentColor 20%, transparent), transparent 68%);
  opacity: 0.94;
  filter: drop-shadow(0 0 6px color-mix(in srgb, currentColor 45%, transparent));
}

.headline-stat__icon img {
  width: 23px;
  height: 23px;
  object-fit: contain;
}

.headline-stat > span:not(.headline-stat__icon) {
  color: #83a2b4;
  font-size: 10px;
}

.headline-stat > strong {
  overflow: hidden;
  color: #effbff;
  font-family: Consolas, monospace;
  font-size: 22px;
  line-height: 1;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.headline-stat > strong small {
  margin-left: 3px;
  color: #7693a5;
  font-family: "Microsoft YaHei", sans-serif;
  font-size: 9px;
  font-weight: 400;
}

.headline-stat.is-cyan { color: var(--cockpit-cyan); }
.headline-stat.is-danger { color: var(--cockpit-red); }
.headline-stat.is-success { color: var(--cockpit-green); }
.headline-stat.is-amber { color: var(--cockpit-amber); }

.headline-radar {
  position: relative;
  display: grid;
  grid-template-rows: minmax(0, 1fr);
  min-width: 0;
  overflow: hidden;
  border: 1px solid rgba(76, 203, 242, 0.62);
  clip-path: polygon(0 8px, 8px 0, calc(100% - 12px) 0, 100% 12px, 100% 100%, 0 100%);
  background:
    radial-gradient(circle at 50% 78%, rgba(51, 204, 207, 0.14), transparent 54%),
    linear-gradient(180deg, rgba(10, 43, 61, 0.98), rgba(4, 22, 35, 0.96));
  box-shadow: inset 0 0 22px rgba(69, 202, 255, 0.1), 0 0 12px rgba(69, 202, 255, 0.08);
}

.headline-radar::after {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  height: 3px;
  content: "";
  background: linear-gradient(90deg, var(--cockpit-green), var(--cockpit-cyan), var(--cockpit-green));
  background-size: 240% 100%;
  opacity: 0.82;
  animation: edge-energy-flow 4.2s linear infinite;
}

.headline-radar__body {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  align-items: center;
  min-height: 0;
  padding: 5px 4px 7px;
}

.headline-radar .radar-scope {
  width: 58px;
  height: 58px;
}

.headline-radar .radar-scope strong {
  max-width: 50px;
  font-size: 11px;
}

.headline-radar .radar-unit > span {
  margin-top: 1px;
  font-size: 8px;
  line-height: 1.1;
}

.headline-radar .radar-unit > small {
  font-size: 7px;
  line-height: 1.1;
}

.headline-radar .radar-point.is-one { top: 14px; left: 18px; }
.headline-radar .radar-point.is-two { right: 13px; bottom: 18px; }
.headline-radar .radar-point.is-three { bottom: 11px; left: 14px; }

.globe-stage {
  position: relative;
  min-height: 0;
  overflow: hidden;
  border-top: 1px solid rgba(69, 202, 255, 0.32);
  border-bottom: 1px solid rgba(69, 202, 255, 0.3);
  clip-path: polygon(0 18px, 18px 0, calc(100% - 18px) 0, 100% 18px, 100% calc(100% - 18px), calc(100% - 18px) 100%, 18px 100%, 0 calc(100% - 18px));
  background:
    linear-gradient(90deg, rgba(4, 22, 38, 0.62), transparent 16%, transparent 84%, rgba(4, 22, 38, 0.62)),
    rgba(1, 9, 16, 0.18);
  box-shadow: inset 0 0 52px rgba(30, 133, 180, 0.09);
}

.globe-stage__frame {
  position: absolute;
  z-index: 3;
  inset: 8px;
  pointer-events: none;
}

.globe-stage__frame span {
  position: absolute;
  width: 42px;
  height: 42px;
  border-color: rgba(72, 207, 255, 0.65);
  border-style: solid;
}

.globe-stage__frame .is-top-left { top: 0; left: 0; border-width: 1px 0 0 1px; }
.globe-stage__frame .is-top-right { top: 0; right: 0; border-width: 1px 1px 0 0; }
.globe-stage__frame .is-bottom-left { bottom: 0; left: 0; border-width: 0 0 1px 1px; }
.globe-stage__frame .is-bottom-right { right: 0; bottom: 0; border-width: 0 1px 1px 0; }

.globe-stage__flow {
  position: absolute;
  z-index: 3;
  inset: 0;
  width: 100%;
  height: 100%;
  overflow: visible;
  pointer-events: none;
}

.globe-stage__flow path {
  fill: none;
  vector-effect: non-scaling-stroke;
}

.globe-stage__flow-base {
  stroke: rgba(69, 202, 255, 0.18);
  stroke-width: 1;
}

.globe-stage__flow-line {
  stroke-linecap: round;
  stroke-dasharray: 8 92;
  stroke-width: 2;
  filter: drop-shadow(0 0 4px currentColor);
  animation: globe-border-flow 5.8s linear infinite;
}

.globe-stage__flow-line.is-primary {
  color: #45caff;
  stroke: #45caff;
}

.globe-stage__flow-line.is-secondary {
  color: #58e0ae;
  stroke: #58e0ae;
  stroke-dasharray: 3 97;
  animation-delay: -2.9s;
  animation-duration: 7.4s;
}

.globe-stage__heading {
  position: absolute;
  z-index: 4;
  top: 13px;
  left: 15px;
  display: grid;
  padding-left: 10px;
  border-left: 2px solid var(--cockpit-green);
  pointer-events: none;
}

.globe-stage__heading span {
  color: #50839e;
  font-family: Consolas, monospace;
  font-size: 8px;
}

.globe-stage__heading strong {
  color: #dff7ff;
  font-size: 15px;
}

.globe-stage__heading small {
  margin-top: 3px;
  color: #7898aa;
  font-size: 9px;
}

.globe-readouts {
  position: absolute;
  z-index: 4;
  top: 50%;
  display: grid;
  gap: 12px;
  width: 116px;
  pointer-events: none;
  transform: translateY(-50%);
}

.globe-readouts--left { left: 16px; }
.globe-readouts--right { right: 16px; text-align: right; }

.globe-readouts > div {
  position: relative;
  display: grid;
  padding: 8px 9px;
  border-top: 1px solid rgba(69, 202, 255, 0.38);
  border-bottom: 1px solid rgba(69, 202, 255, 0.14);
  background: linear-gradient(90deg, rgba(7, 42, 65, 0.68), rgba(4, 18, 30, 0.12));
}

.globe-readouts--right > div {
  background: linear-gradient(270deg, rgba(7, 42, 65, 0.68), rgba(4, 18, 30, 0.12));
}

.globe-readouts > div::before {
  position: absolute;
  top: -3px;
  left: 0;
  width: 24px;
  height: 3px;
  content: "";
  background: var(--cockpit-cyan);
}

.globe-readouts--right > div::before { right: 0; left: auto; }

.globe-readouts span {
  color: #6098b3;
  font-family: Consolas, monospace;
  font-size: 8px;
}

.globe-readouts strong {
  margin: 2px 0;
  color: #dbf8ff;
  font-family: Consolas, monospace;
  font-size: 18px;
  font-variant-numeric: tabular-nums;
  text-shadow: 0 0 10px rgba(69, 202, 255, 0.38);
}

.globe-readouts strong.is-danger { color: #ff7b82; text-shadow: 0 0 10px rgba(255, 77, 91, 0.36); }
.globe-readouts small { color: #7d9aaa; font-size: 9px; }

.globe-core-label {
  position: absolute;
  z-index: 4;
  bottom: 16px;
  left: 50%;
  display: grid;
  min-width: 142px;
  padding: 5px 20px 7px;
  border-right: 1px solid rgba(69, 202, 255, 0.4);
  border-left: 1px solid rgba(69, 202, 255, 0.4);
  text-align: center;
  pointer-events: none;
  transform: translateX(-50%);
  background: linear-gradient(90deg, transparent, rgba(5, 30, 48, 0.78), transparent);
}

.globe-core-label span { color: #5b8ba4; font-family: Consolas, monospace; font-size: 8px; }
.globe-core-label strong { color: #eefbff; font-family: Consolas, monospace; font-size: 22px; line-height: 1.1; }
.globe-core-label small { color: #7d9aaa; font-size: 9px; }

.globe-stage__legend {
  position: absolute;
  z-index: 4;
  right: 12px;
  bottom: 10px;
  display: flex;
  align-items: center;
  gap: 10px;
  color: #8ba9b9;
  font-size: 9px;
  pointer-events: none;
}

.globe-stage__legend span {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.globe-stage__legend i {
  display: block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
}

.globe-stage__legend i.is-idc { background: var(--cockpit-amber); }
.globe-stage__legend i.is-asset { border: 1px solid var(--cockpit-green); }
.globe-stage__legend small { color: #526e7e; }

.globe-loading {
  position: absolute;
  z-index: 5;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: #a7c9d8;
  font-size: 12px;
  background: rgba(2, 12, 20, 0.58);
}

.globe-loading span {
  width: 15px;
  height: 15px;
  border: 2px solid rgba(69, 202, 255, 0.2);
  border-top-color: var(--cockpit-cyan);
  border-radius: 50%;
  animation: cockpit-spin 900ms linear infinite;
}

.metric-strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 7px;
  min-height: 0;
}

.metric-card {
  position: relative;
  min-width: 0;
  overflow: hidden;
  border: 1px solid rgba(75, 186, 230, 0.46);
  border-radius: 0;
  clip-path: polygon(0 10px, 10px 0, calc(100% - 18px) 0, 100% 18px, 100% calc(100% - 8px), calc(100% - 8px) 100%, 0 100%);
  background:
    linear-gradient(135deg, rgba(54, 160, 204, 0.12), transparent 40%),
    rgba(5, 23, 38, 0.94);
  box-shadow: inset 0 0 24px rgba(32, 148, 198, 0.06);
}

.metric-card::after {
  position: absolute;
  z-index: 2;
  top: 0;
  left: 0;
  width: 82px;
  height: 2px;
  content: "";
  background: linear-gradient(90deg, transparent, var(--cockpit-green), #eaffff, var(--cockpit-cyan), transparent);
  box-shadow: 0 0 8px rgba(69, 202, 255, 0.54);
  transform: translateX(-100px);
  animation: panel-edge-flow 8.6s linear infinite;
}

.metric-card header {
  justify-content: space-between;
  height: 35px;
  padding: 0 10px;
  border-bottom: 1px solid rgba(75, 174, 213, 0.25);
  background: linear-gradient(90deg, rgba(17, 78, 108, 0.28), transparent);
}

.metric-card header span {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: #bee0ee;
  font-size: 11px;
  font-weight: 600;
}

.metric-card header img {
  width: 16px;
  height: 16px;
  object-fit: contain;
  filter: drop-shadow(0 0 5px rgba(69, 202, 255, 0.48));
}

.metric-card header small { color: #658397; font-size: 9px; }

.radar-unit {
  --radar-color: #58e0ae;
  display: grid;
  justify-items: center;
  min-width: 0;
  color: var(--radar-color);
}

.radar-unit.is-warning { --radar-color: #ffc857; }
.radar-unit.is-danger { --radar-color: #ff6f6f; }
.radar-unit.is-resource { --radar-color: #45caff; }

.radar-unit > span {
  margin-top: 2px;
  color: #b8d9e7;
  font-size: 9px;
  font-weight: 600;
}

.radar-unit > small {
  max-width: 100%;
  overflow: hidden;
  color: #668698;
  font-size: 8px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.radar-scope {
  position: relative;
  width: 66px;
  height: 66px;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--radar-color) 66%, transparent);
  border-radius: 50%;
  background:
    radial-gradient(circle, transparent 0 22%, color-mix(in srgb, var(--radar-color) 26%, transparent) 23% 24%, transparent 25% 47%, color-mix(in srgb, var(--radar-color) 22%, transparent) 48% 49%, transparent 50%),
    rgba(5, 30, 42, 0.62);
  box-shadow: inset 0 0 18px color-mix(in srgb, var(--radar-color) 13%, transparent), 0 0 9px color-mix(in srgb, var(--radar-color) 14%, transparent);
}

.radar-scope::before,
.radar-scope::after {
  position: absolute;
  z-index: 1;
  content: "";
  background: color-mix(in srgb, var(--radar-color) 24%, transparent);
}

.radar-scope::before {
  top: 50%;
  right: 4px;
  left: 4px;
  height: 1px;
}

.radar-scope::after {
  top: 4px;
  bottom: 4px;
  left: 50%;
  width: 1px;
}

.radar-sweep {
  position: absolute;
  z-index: 2;
  inset: 3px;
  border-radius: 50%;
  background: conic-gradient(from 8deg, transparent 0 76%, color-mix(in srgb, var(--radar-color) 8%, transparent) 82%, color-mix(in srgb, var(--radar-color) 52%, transparent) 100%);
  animation: radar-scan 4.8s linear infinite;
}

.radar-unit.is-resource .radar-sweep {
  animation-duration: 6.2s;
  animation-direction: reverse;
}

.radar-point {
  position: absolute;
  z-index: 3;
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: var(--radar-color);
  box-shadow: 0 0 6px var(--radar-color);
  animation: radar-pulse 2.4s ease-in-out infinite;
}

.radar-point.is-one { top: 17px; left: 22px; }
.radar-point.is-two { right: 15px; bottom: 21px; animation-delay: -0.8s; }
.radar-point.is-three { bottom: 13px; left: 17px; animation-delay: -1.6s; }

.radar-scope strong {
  position: absolute;
  z-index: 4;
  top: 50%;
  left: 50%;
  max-width: 56px;
  overflow: hidden;
  color: #e9faff;
  font-family: Consolas, monospace;
  font-size: 12px;
  line-height: 1;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-shadow: 0 0 7px color-mix(in srgb, var(--radar-color) 65%, transparent);
  transform: translate(-50%, -50%);
}

.metric-card__body {
  display: grid;
  gap: 9px;
  padding: 11px 10px;
}

.metric-row {
  grid-template-columns: minmax(50px, 1fr) minmax(42px, 0.8fr) 39px;
}

.metric-row > span {
  color: #91afbf;
  font-size: 9px;
}

.metric-row > i span.is-normal { background: var(--cockpit-green); }
.metric-row > i span.is-warning { background: var(--cockpit-amber); }
.metric-row > i span.is-danger { background: var(--cockpit-red); }
.metric-row strong { font-family: Consolas, monospace; font-size: 10px; text-align: right; }
.metric-row strong.is-normal { color: var(--cockpit-green); }
.metric-row strong.is-warning { color: var(--cockpit-amber); }
.metric-row strong.is-danger { color: var(--cockpit-red); }
.metric-empty { padding: 24px 8px; color: #607c8e; font-size: 10px; text-align: center; }

.asset-panel {
  display: grid;
  grid-template-rows: 39px minmax(0, 1fr);
}

.asset-overview {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 116px;
  min-height: 0;
}

.asset-list {
  display: grid;
  align-content: center;
  gap: 8px;
  padding: 8px 10px;
}

.asset-row {
  grid-template-columns: 60px minmax(40px, 1fr) 34px;
}

.health-chart-wrap {
  position: relative;
  min-width: 0;
  min-height: 0;
  border-left: 1px solid rgba(75, 153, 187, 0.16);
}

.health-chart {
  width: 100%;
  height: 100%;
  min-height: 128px;
}

.health-chart-legend {
  position: absolute;
  right: 0;
  bottom: 7px;
  left: 0;
  display: flex;
  justify-content: center;
  gap: 8px;
  color: #69899b;
  font-size: 8px;
  pointer-events: none;
}

.health-chart-legend span {
  display: inline-flex;
  align-items: center;
  gap: 3px;
}

.health-chart-legend i {
  width: 5px;
  height: 5px;
  border-radius: 50%;
}

.health-chart-legend i.is-hardware { background: var(--cockpit-cyan); }
.health-chart-legend i.is-system { background: var(--cockpit-green); }

.idc-panel {
  display: grid;
  grid-template-rows: 39px minmax(100px, 1fr) auto;
}

.idc-list { height: auto; }
.idc-row { grid-template-columns: 20px minmax(56px, 0.75fr) minmax(42px, 1fr) 30px; }
.idc-row > i span { background: var(--cockpit-cyan); }

.risk-summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 7px;
  padding: 8px 10px 10px;
  border-top: 1px solid rgba(75, 153, 187, 0.16);
}

.risk-summary > div {
  display: grid;
  grid-template-columns: 18px minmax(0, 1fr) auto;
  gap: 4px;
  min-width: 0;
  padding: 6px;
  border-left: 2px solid var(--cockpit-amber);
  background: rgba(255, 190, 70, 0.06);
}

.risk-summary > div:last-child { border-left-color: var(--cockpit-red); }
.risk-summary .el-icon { color: var(--cockpit-amber); }
.risk-summary > div:last-child .el-icon { color: var(--cockpit-red); }
.risk-summary span { overflow: hidden; color: #8eabba; font-size: 9px; text-overflow: ellipsis; white-space: nowrap; }
.risk-summary strong { color: #e7f7ff; font-family: Consolas, monospace; font-size: 12px; }

.panel-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  height: calc(100% - 39px);
  min-height: 80px;
  color: #658397;
  font-size: 11px;
}

.panel-empty span {
  width: 6px;
  height: 6px;
  border: 1px solid #507991;
  transform: rotate(45deg);
}

@keyframes cockpit-spin {
  to { transform: rotate(360deg); }
}

@keyframes radar-scan {
  to { transform: rotate(360deg); }
}

@keyframes radar-pulse {
  0%, 100% { opacity: 0.36; transform: scale(0.8); }
  50% { opacity: 1; transform: scale(1.22); }
}

@keyframes globe-border-flow {
  to { stroke-dashoffset: -100; }
}

@keyframes panel-edge-flow {
  0% { opacity: 0; transform: translateX(-120px); }
  8% { opacity: 1; }
  72% { opacity: 1; }
  82%, 100% { opacity: 0; transform: translateX(100vw); }
}

@keyframes edge-energy-flow {
  to { background-position: -260% 0; }
}

@media (max-width: 1420px) {
  .cockpit-layout {
    grid-template-columns: minmax(250px, 290px) minmax(400px, 1fr) minmax(250px, 290px);
  }

  .cockpit-header {
    grid-template-columns: 0.8fr minmax(400px, 1.5fr) 0.8fr;
  }

  .cockpit-header__status small,
  .globe-stage__legend small {
    display: none;
  }

  .globe-readouts {
    width: 94px;
  }

  .globe-readouts--left { left: 10px; }
  .globe-readouts--right { right: 10px; }

  .headline-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr)) minmax(150px, 1.2fr) repeat(2, minmax(0, 1fr));
  }

  .headline-stat {
    grid-template-columns: 28px minmax(0, 1fr);
    column-gap: 6px;
    padding-right: 7px;
    padding-left: 7px;
  }

  .headline-stat__icon {
    width: 27px;
    height: 30px;
  }

  .headline-stat__icon img {
    width: 19px;
    height: 19px;
  }

  .headline-stat > strong {
    font-size: 17px;
  }
}

@media (max-width: 1180px) {
  .ops-cockpit {
    height: auto;
    min-height: calc(100vh - 104px);
    overflow: auto;
  }

  :global(.home-container .el-main:has(.ops-cockpit)) {
    overflow: auto;
  }

  .cockpit-layout {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    height: auto;
  }

  .cockpit-center {
    grid-column: 1 / 3;
    grid-row: 1;
    min-height: 720px;
  }

  .cockpit-column {
    min-height: 720px;
  }

  .cockpit-header__status {
    display: none;
  }

  .cockpit-header {
    grid-template-columns: minmax(0, 1fr) auto;
  }
}

@media (max-width: 720px) {
  .cockpit-header {
    height: 58px;
    padding: 0 8px;
  }

  .cockpit-title {
    justify-content: flex-start;
  }

  .cockpit-title__line,
  .cockpit-title p,
  .cockpit-clock,
  .globe-readouts,
  .globe-core-label {
    display: none;
  }

  .cockpit-title h1 {
    font-size: 16px;
  }

  .cockpit-layout {
    grid-template-columns: minmax(0, 1fr);
    padding: 7px;
  }

  .cockpit-center,
  .cockpit-column {
    grid-column: 1;
    min-height: auto;
  }

  .cockpit-center {
    grid-template-rows: auto 430px auto;
  }

  .cockpit-column--left,
  .cockpit-column--right {
    grid-template-rows: repeat(3, minmax(210px, auto));
  }

  .headline-stats,
  .metric-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .headline-radar {
    grid-column: 1 / 3;
    min-height: 92px;
  }

  .metric-card:last-child {
    grid-column: 1 / 3;
  }

  .cockpit-error {
    top: 64px;
    width: calc(100% - 28px);
    box-sizing: border-box;
  }
}

@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    scroll-behavior: auto !important;
    animation-duration: 0.001ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.001ms !important;
  }
}
</style>
