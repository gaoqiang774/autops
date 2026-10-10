<template>
  <div class="idc-hardware-panel">
    <div class="toolbar">
      <div class="toolbar-main">
        <div class="toolbar-tags">
          <span>当前视图</span>
          <el-tag type="primary" effect="light">硬件监控</el-tag>
          <el-tag v-if="activeContext.remoteManagementCard" type="success" effect="light">
            远程管理卡：{{ activeContext.remoteManagementCard }}
          </el-tag>
          <el-tag v-if="displayAsset.ip" type="info" effect="light">
            物理机：{{ displayAsset.ip }}
          </el-tag>
          <el-tag v-if="monitor.vendor" :type="getVendorTagType(monitor.vendor)" effect="light">
            厂商：{{ monitor.vendor }}
          </el-tag>
          <el-tag v-if="displaySourceText" type="warning" effect="light">
            来源：{{ displaySourceText }}
          </el-tag>
          <el-tag v-if="trendPanels.length" type="primary" effect="plain">
            趋势面板：{{ trendPanels.length }}
          </el-tag>
        </div>

        <div class="toolbar-actions">
          <el-form :inline="true" :model="queryForm" class="toolbar-form" @submit.prevent>
            <el-form-item label="回看分钟">
              <el-input-number
                v-model="queryForm.lookbackMinutes"
                :min="1"
                :max="60"
                :step="1"
                size="small"
                controls-position="right"
              />
            </el-form-item>
            <el-form-item label="趋势小时">
              <el-input-number
                v-model="queryForm.trendHours"
                :min="1"
                :max="168"
                :step="1"
                size="small"
                controls-position="right"
              />
            </el-form-item>
            <el-form-item label="步长分钟">
              <el-input-number
                v-model="queryForm.trendStepMinutes"
                :min="1"
                :max="120"
                :step="5"
                size="small"
                controls-position="right"
              />
            </el-form-item>
          </el-form>

          <el-button size="small" type="primary" :disabled="!hasTarget" :loading="loading" @click="loadMonitor">
            <el-icon><RefreshRight /></el-icon>刷新监控
          </el-button>
        </div>
      </div>

      <div class="toolbar-divider"></div>

      <div class="view-tabs">
        <button
          type="button"
          class="view-tab"
          :class="{ active: activeView === 'room' }"
          @click="$emit('switch-view', 'room')"
        >
          机房
        </button>
        <button
          type="button"
          class="view-tab"
          :class="{ active: activeView === 'cabinet' }"
          @click="$emit('switch-view', 'cabinet')"
        >
          机柜
        </button>
        <button
          type="button"
          class="view-tab"
          :class="{ active: activeView === 'physical' }"
          @click="$emit('switch-view', 'physical')"
        >
          物理机
        </button>
        <button
          type="button"
          class="view-tab"
          :class="{ active: activeView === 'vm' }"
          @click="$emit('switch-view', 'vm')"
        >
          虚拟机
        </button>
        <button
          type="button"
          class="view-tab"
          :class="{ active: activeView === 'hardware' }"
          @click="$emit('switch-view', 'hardware')"
        >
          硬件监控
        </button>
      </div>
    </div>

    <div class="panel-card">
      <el-empty v-if="!hasTarget" description="点击物理机列表中的远程管理卡 IP 查看硬件监控" />

      <div v-else v-loading="loading" class="panel-body">
        <el-alert
          v-if="loadError"
          :title="loadError"
          type="error"
          show-icon
          :closable="false"
          class="panel-alert"
        />
        <el-alert
          v-else-if="monitorLoaded && !monitor.metricsAvailable"
          title="当前未获取到可用的硬件监控指标"
          type="warning"
          show-icon
          :closable="false"
          class="panel-alert"
        />

        <div class="meta-grid">
          <section class="meta-card">
            <div class="section-head">
              <div>
                <h3>资产信息</h3>
                <p>Asset</p>
              </div>
            </div>

            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="资产 ID">{{ displayAsset.id || '-' }}</el-descriptions-item>
              <el-descriptions-item label="物理机 IP">{{ displayAsset.ip || '-' }}</el-descriptions-item>
              <el-descriptions-item label="远程管理卡">{{ displayAsset.remoteManagementCard || '-' }}</el-descriptions-item>
              <el-descriptions-item label="品牌">{{ displayAsset.brand || '-' }}</el-descriptions-item>
              <el-descriptions-item label="型号">{{ displayAsset.model || '-' }}</el-descriptions-item>
              <el-descriptions-item label="用途">{{ displayAsset.purpose || '-' }}</el-descriptions-item>
            </el-descriptions>
          </section>

          <section class="meta-card">
            <div class="section-head">
              <div>
                <h3>采集信息</h3>
                <p>Prometheus Query</p>
              </div>
            </div>

            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="指标可用">
                <el-tag size="small" effect="light" :type="monitor.metricsAvailable ? 'success' : 'warning'">
                  {{ monitor.metricsAvailable ? '是' : '否' }}
                </el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="识别厂商">{{ monitor.vendor || '-' }}</el-descriptions-item>
              <el-descriptions-item label="采集 Job">{{ monitor.source.job || '-' }}</el-descriptions-item>
              <el-descriptions-item label="匹配标签">{{ monitor.source.matchedLabel || '-' }}</el-descriptions-item>
              <el-descriptions-item label="监控目标">{{ monitor.source.prometheusTarget || '-' }}</el-descriptions-item>
              <el-descriptions-item label="采样时间">{{ formatDateTime(monitor.sampledAt) }}</el-descriptions-item>
              <el-descriptions-item label="趋势范围">{{ trendRangeLabel }}</el-descriptions-item>
            </el-descriptions>
          </section>

          <section class="health-card">
            <div class="section-head">
              <div>
                <h3>健康总评</h3>
                <p>Health</p>
              </div>
              <div class="health-tags">
                <el-tag v-if="healthAvailable" :type="statusTagType(health.level || 'info')" effect="light">
                  {{ normalizeStatusLabel(health.level) }}
                </el-tag>
                <el-tag v-if="health.immediateAction" type="danger" effect="plain">
                  需立即处理
                </el-tag>
              </div>
            </div>

            <div v-if="healthAvailable" class="health-body">
              <div class="health-score">
                <strong>{{ healthScoreText }}</strong>
                <span>/ 100</span>
              </div>

              <div class="health-content">
                <p class="health-conclusion">{{ health.conclusion || '暂无健康结论' }}</p>

                <div class="health-stats">
                  <el-tag v-if="health.criticalCount" type="danger" effect="light">严重 {{ health.criticalCount }}</el-tag>
                  <el-tag v-if="health.warningCount" type="warning" effect="light">告警 {{ health.warningCount }}</el-tag>
                  <el-tag v-if="health.infoCount" type="info" effect="light">提示 {{ health.infoCount }}</el-tag>
                  <el-tag v-for="category in health.riskCategories" :key="category" type="info" effect="plain">
                    风险：{{ humanizeCategory(category) }}
                  </el-tag>
                </div>

                <div v-if="health.recommendations.length" class="health-recommendations">
                  <span>建议</span>
                  <p v-for="(item, index) in health.recommendations" :key="`${index}-${item}`">{{ item }}</p>
                </div>
              </div>
            </div>

            <el-empty v-else description="暂无健康总评" :image-size="60" />
          </section>
        </div>

        <div class="summary-grid">
          <article v-for="card in summaryCards" :key="card.key" class="summary-card" :class="`summary-card-${card.accent}`">
            <div class="summary-card-head">
              <span class="summary-card-icon">
                <img :src="card.icon" :alt="card.label" />
              </span>
              <div class="summary-card-main">
                <span class="summary-label">{{ card.label }}</span>
                <strong class="summary-value">{{ card.value }}</strong>
              </div>
            </div>
            <el-tag
              v-if="card.isStatus && card.value !== '-'"
              size="small"
              effect="light"
              class="summary-state-tag"
              :type="statusTagType(card.rawValue)"
            >
              {{ normalizeStatusLabel(card.rawValue) }}
            </el-tag>
          </article>
        </div>

        <section class="alerts-card" :class="{ clear: !alertRows.length }">
          <div class="section-head">
            <div>
              <h3>硬件预警</h3>
              <p>Alerts</p>
            </div>
            <div class="alert-stats">
              <el-tag :type="alertRows.length ? 'danger' : 'success'" effect="light">
                {{ alertRows.length ? `共 ${alertRows.length} 条` : '暂无预警' }}
              </el-tag>
              <el-tag v-if="alertCriticalCount" type="danger" effect="plain">严重 {{ alertCriticalCount }}</el-tag>
              <el-tag v-if="alertWarningCount" type="warning" effect="plain">告警 {{ alertWarningCount }}</el-tag>
            </div>
          </div>

          <div v-if="alertRows.length" class="alert-list">
            <article v-for="alert in alertRows" :key="alert.key" class="alert-item" :class="`severity-${alert.severity}`">
              <div class="alert-main">
                <div class="alert-title-row">
                  <el-tag size="small" effect="light" :type="alertTagType(alert.severity)">
                    {{ alert.severityLabel }}
                  </el-tag>
                  <strong>{{ alert.title }}</strong>
                </div>
                <p v-if="alert.message" class="alert-message">{{ alert.message }}</p>
                <p v-if="alert.component || alert.category || alert.metric" class="alert-context">
                  {{ [alert.component, alert.category, alert.metric].filter(Boolean).join(' / ') }}
                </p>
              </div>

              <div class="alert-data">
                <span v-if="alert.currentValue" class="alert-chip">
                  <em>当前</em>
                  <strong>{{ alert.currentValue }}</strong>
                </span>
                <span v-if="alert.threshold" class="alert-chip">
                  <em>阈值</em>
                  <strong>{{ alert.threshold }}</strong>
                </span>
                <span v-if="alert.durationMinutes" class="alert-chip">
                  <em>持续</em>
                  <strong>{{ alert.durationMinutes }} 分钟</strong>
                </span>
                <span v-if="alert.lastSeenAt" class="alert-chip">
                  <em>时间</em>
                  <strong>{{ formatDateTime(alert.lastSeenAt) }}</strong>
                </span>
              </div>

              <div v-if="alert.evidence || alert.recommendation" class="alert-extra">
                <p v-if="alert.evidence" class="alert-evidence">证据：{{ alert.evidence }}</p>
                <p v-if="alert.recommendation" class="alert-recommendation">建议：{{ alert.recommendation }}</p>
              </div>
            </article>
          </div>

          <div v-else class="clear-state">
            <span class="clear-dot"></span>
            <div>
              <strong>当前没有触发硬件预警</strong>
              <p>磁盘寿命、磨损、温度、风扇、RAID/ECC、电源和内存状态正常。</p>
            </div>
          </div>
        </section>

        <section class="trends-card">
          <div class="section-head">
            <div>
              <h3>趋势监控</h3>
              <p>{{ trendWindowLabel }}</p>
            </div>
            <el-tag type="info" effect="plain">步长 {{ normalizedTrendStepMinutes }} 分钟</el-tag>
          </div>

          <div v-if="trendPanels.length" class="trend-grid">
            <article v-for="panel in trendPanels" :key="panel.key" class="trend-panel" :class="{ 'trend-panel-status': panel.valueType === 'status' }">
              <div class="trend-panel-head">
                <div>
                  <h4>{{ panel.title }}</h4>
                  <p>{{ panel.description || `${panel.series.length} 条序列` }}</p>
                </div>
                <el-tag size="small" effect="light" :type="statusTagType(panel.status)">
                  {{ normalizeStatusLabel(panel.status) }}
                </el-tag>
              </div>

              <div class="trend-metrics">
                <div v-for="metric in panel.metrics" :key="metric.label" class="trend-metric">
                  <span>{{ metric.label }}</span>
                  <strong :class="metric.tone">{{ metric.value }}</strong>
                </div>
              </div>

              <div
                v-if="panel.hasData"
                :id="chartDomId(panel)"
                class="trend-chart"
                :class="{ 'trend-chart-status': panel.valueType === 'status', 'trend-chart-dashboard': panel.isStatusDashboard }"
              ></div>
              <el-empty v-else description="暂无趋势采样点" :image-size="60" class="trend-empty" />

              <div v-if="panel.isStatusDashboard && panel.statusOverview?.displayItems?.length" class="status-dashboard-list">
                <div v-for="item in panel.statusOverview.displayItems" :key="`${panel.key}-${item.name}`" class="status-dashboard-item">
                  <span class="status-dashboard-name">{{ item.name }}</span>
                  <el-tag size="small" effect="light" :type="item.tagType">{{ item.label }}</el-tag>
                </div>
                <p v-if="panel.statusOverview.moreText" class="status-dashboard-more">{{ panel.statusOverview.moreText }}</p>
              </div>

              <div v-if="panel.metricNames.length" class="metric-tags">
                <span>指标源</span>
                <el-tag v-for="metric in panel.metricNames" :key="metric" size="small" effect="plain">
                  {{ metric }}
                </el-tag>
              </div>
            </article>
          </div>

          <el-empty v-else description="暂无趋势数据" />
        </section>

        <section class="components-card">
          <div class="section-head">
            <div>
              <h3>组件明细</h3>
              <p>Summary + Components</p>
            </div>
            <el-tag type="info" effect="plain">共 {{ flatComponents.length }} 项</el-tag>
          </div>

          <el-empty v-if="!flatComponents.length && !hasComponentPayload" description="暂无组件明细" />

          <template v-else-if="flatComponents.length">
            <el-table :data="flatComponents" border stripe class="components-table" :max-height="560">
              <el-table-column type="index" label="#" width="56" />
              <el-table-column prop="group" label="分类" min-width="140" />
              <el-table-column prop="name" label="组件" min-width="180" />
              <el-table-column label="状态" width="120" align="center">
                <template #default="{ row }">
                  <el-tag size="small" effect="light" :type="statusTagType(row.status)">
                    {{ normalizeStatusLabel(row.status) }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="reading" label="读数" min-width="150" />
              <el-table-column prop="details" label="详情" min-width="320" />
            </el-table>
          </template>

          <pre v-else class="raw-preview">{{ formattedComponentsJson }}</pre>
        </section>
      </div>
    </div>
  </div>
</template>

<script>
import * as echarts from 'echarts'

const makeQueryForm = () => ({
  lookbackMinutes: 5,
  trendHours: 24,
  trendStepMinutes: 15
})

const makeSummary = () => ({
  overallStatus: '',
  cpuStatus: '',
  memoryStatus: '',
  diskStatus: '',
  fanStatus: '',
  powerStatus: '',
  temperatureStatus: '',
  cpuUsagePercent: null,
  memoryUsagePercent: null,
  powerWatts: null,
  highestTemperatureCelsius: null
})

const makeTrends = () => ({
  panels: [],
  from: '',
  to: '',
  hours: null,
  stepMinutes: null
})

const makeHealth = () => ({
  score: null,
  level: '',
  conclusion: '',
  riskCategories: [],
  criticalCount: 0,
  warningCount: 0,
  infoCount: 0,
  immediateAction: false,
  recommendations: []
})

const makeMonitorState = () => ({
  asset: {},
  metricsAvailable: false,
  vendor: '',
  rawVendor: '',
  source: {
    job: '',
    matchedLabel: '',
    prometheusTarget: ''
  },
  summary: makeSummary(),
  components: null,
  flatComponents: [],
  trends: makeTrends(),
  alerts: [],
  health: makeHealth(),
  sampledAt: ''
})

const KNOWN_TREND_LABELS = {
  disk_life: '硬盘寿命趋势',
  hard_disk_life: '硬盘寿命趋势',
  hardDiskLife: '硬盘寿命趋势',
  ssd_wear: 'SSD 磨损趋势',
  ssdWear: 'SSD 磨损趋势',
  cooling_efficiency: '散热系统效率',
  coolingEfficiency: '散热系统效率',
  raid_health: 'RAID 卡健康',
  raidHealth: 'RAID 卡健康',
  power_redundancy: '电源冗余状态',
  powerRedundancy: '电源冗余状态',
  memory_stability: '内存/系统稳定性',
  memoryStability: '内存/系统稳定性'
}

const SUMMARY_ICON_MAP = {
  overallStatus: require('@/assets/image/健康.svg'),
  cpuStatus: require('@/assets/image/cpu.svg'),
  memoryStatus: require('@/assets/image/内存.svg'),
  diskStatus: require('@/assets/image/数据库.svg'),
  fanStatus: require('@/assets/image/监控.svg'),
  powerStatus: require('@/assets/image/状态-copy.svg'),
  temperatureStatus: require('@/assets/image/起火.svg'),
  alertCount: require('@/assets/image/警告.svg'),
  powerWatts: require('@/assets/image/数据采集.svg'),
  highestTemperatureCelsius: require('@/assets/image/环境.svg')
}

const TREND_STATUS_COMPONENT_GROUPS = {
  hardDiskStatus: ['disks'],
  hardDiskPowerOnHours: ['disks'],
  hardDiskRemnantWearout: ['disks'],
  physicalDiskComponentStatus: ['disks'],
  physicalDiskRemainingRatedWriteEndurance: ['disks'],
  raidControllerHealthStatus: ['raidControllers'],
  raidControllerDDREccCount: ['raidControllers'],
  controllerComponentStatus: ['raidControllers'],
  logicalDriveConsistencyCheckStatus: ['raidControllers'],
  powerSupplyStatus: ['powerSupplies'],
  powerSupplySensorState: ['powerSupplies'],
  fanSpeed: ['fans'],
  coolingDeviceReading: ['fans'],
  temperatureReading: ['temperatures'],
  temperatureProbeReading: ['temperatures']
}

const STATUS_DASHBOARD_PANEL_CODES = ['raid-health', 'disk-life']

export default {
  name: 'IdcHardwareMonitorPanel',
  emits: ['switch-view'],
  props: {
    initialContext: {
      type: Object,
      default: () => ({})
    },
    activeView: {
      type: String,
      default: 'hardware'
    }
  },
  data() {
    return {
      activeContext: this.normalizeContext(this.initialContext),
      queryForm: makeQueryForm(),
      loading: false,
      loadError: '',
      monitorLoaded: false,
      monitor: makeMonitorState(),
      chartInstances: {},
      chartSeed: `hardware-${Date.now()}-${Math.floor(Math.random() * 10000)}`
    }
  },
  computed: {
    hasTarget() {
      return !!(this.activeContext.remoteManagementCard || this.activeContext.assetId)
    },
    displayAsset() {
      const asset = this.monitor.asset || {}
      return {
        id: this.num(this.pick(asset, ['id']), this.activeContext.assetId || ''),
        ip: String(this.pick(asset, ['ip'], this.activeContext.ip || '')),
        brand: String(this.pick(asset, ['brand'], this.activeContext.brand || '')),
        model: String(this.pick(asset, ['model'], this.activeContext.model || '')),
        purpose: String(this.pick(asset, ['purpose'], this.activeContext.purpose || '')),
        remoteManagementCard: String(this.pick(asset, ['remoteManagementCard', 'remote_management_card'], this.activeContext.remoteManagementCard || ''))
      }
    },
    flatComponents() {
      return this.monitor.flatComponents || []
    },
    trendPanels() {
      return this.decorateTrendPanels(this.monitor.trends?.panels || [])
    },
    health() {
      return this.monitor.health || makeHealth()
    },
    healthAvailable() {
      return this.health.score !== null || !!this.health.conclusion
    },
    healthScoreText() {
      const score = this.toFiniteNumber(this.health.score, null)
      return Number.isFinite(score) ? Math.round(score) : '-'
    },
    displaySourceText() {
      const source = this.monitor.source || {}
      const parts = [source.job, source.prometheusTarget].filter(Boolean)
      return parts.join(' / ')
    },
    alertRows() {
      return this.monitor.alerts || []
    },
    alertCriticalCount() {
      return this.alertRows.filter(item => item.severity === 'critical').length
    },
    alertWarningCount() {
      return this.alertRows.filter(item => item.severity === 'warning').length
    },
    normalizedTrendHours() {
      return this.num(this.monitor.trends?.hours, this.queryForm.trendHours || 24)
    },
    normalizedTrendStepMinutes() {
      return this.num(this.monitor.trends?.stepMinutes, this.queryForm.trendStepMinutes || 15)
    },
    trendWindowLabel() {
      return `Trend / 最近 ${this.normalizedTrendHours} 小时`
    },
    trendRangeLabel() {
      const from = this.monitor.trends?.from
      const to = this.monitor.trends?.to
      if (!from && !to) return '-'
      return `${this.formatDateTime(from)} 至 ${this.formatDateTime(to)}`
    },
    hasComponentPayload() {
      const payload = this.monitor.components
      if (Array.isArray(payload)) return payload.length > 0
      return !!(payload && typeof payload === 'object' && Object.keys(payload).length)
    },
    formattedComponentsJson() {
      if (!this.hasComponentPayload) return ''
      try {
        return JSON.stringify(this.monitor.components, null, 2)
      } catch (error) {
        return ''
      }
    },
    summaryCards() {
      const summary = this.monitor.summary || makeSummary()
      return [
        { key: 'overallStatus', label: '整机状态', value: this.formatStatusValue(this.resolveSummaryStatus('overallStatus', summary.overallStatus)), rawValue: this.resolveSummaryStatus('overallStatus', summary.overallStatus), isStatus: true, icon: SUMMARY_ICON_MAP.overallStatus, accent: 'health' },
        { key: 'cpuStatus', label: 'CPU 状态', value: this.formatStatusValue(this.resolveSummaryStatus('cpuStatus', summary.cpuStatus)), rawValue: this.resolveSummaryStatus('cpuStatus', summary.cpuStatus), isStatus: true, icon: SUMMARY_ICON_MAP.cpuStatus, accent: 'cpu' },
        { key: 'memoryStatus', label: '内存状态', value: this.formatStatusValue(this.resolveSummaryStatus('memoryStatus', summary.memoryStatus)), rawValue: this.resolveSummaryStatus('memoryStatus', summary.memoryStatus), isStatus: true, icon: SUMMARY_ICON_MAP.memoryStatus, accent: 'memory' },
        { key: 'diskStatus', label: '磁盘状态', value: this.formatStatusValue(this.resolveSummaryStatus('diskStatus', summary.diskStatus)), rawValue: this.resolveSummaryStatus('diskStatus', summary.diskStatus), isStatus: true, icon: SUMMARY_ICON_MAP.diskStatus, accent: 'disk' },
        { key: 'fanStatus', label: '风扇状态', value: this.formatStatusValue(this.resolveSummaryStatus('fanStatus', summary.fanStatus)), rawValue: this.resolveSummaryStatus('fanStatus', summary.fanStatus), isStatus: true, icon: SUMMARY_ICON_MAP.fanStatus, accent: 'fan' },
        { key: 'powerStatus', label: '电源状态', value: this.formatStatusValue(this.resolveSummaryStatus('powerStatus', summary.powerStatus)), rawValue: this.resolveSummaryStatus('powerStatus', summary.powerStatus), isStatus: true, icon: SUMMARY_ICON_MAP.powerStatus, accent: 'power' },
        { key: 'temperatureStatus', label: '温度状态', value: this.formatStatusValue(this.resolveSummaryStatus('temperatureStatus', summary.temperatureStatus)), rawValue: this.resolveSummaryStatus('temperatureStatus', summary.temperatureStatus), isStatus: true, icon: SUMMARY_ICON_MAP.temperatureStatus, accent: 'temperature' },
        { key: 'alertCount', label: '预警数量', value: `${this.alertRows.length} 条`, rawValue: this.alertRows.length, isStatus: false, icon: SUMMARY_ICON_MAP.alertCount, accent: 'alert' },
        { key: 'powerWatts', label: '功耗', value: this.formatMetric(summary.powerWatts, 'W'), rawValue: summary.powerWatts, isStatus: false, icon: SUMMARY_ICON_MAP.powerWatts, accent: 'metric' },
        { key: 'highestTemperatureCelsius', label: '最高温度', value: this.formatMetric(summary.highestTemperatureCelsius, '°C'), rawValue: summary.highestTemperatureCelsius, isStatus: false, icon: SUMMARY_ICON_MAP.highestTemperatureCelsius, accent: 'metric' }
      ]
    }
  },
  watch: {
    initialContext: {
      deep: true,
      handler(value) {
        this.activeContext = this.normalizeContext(value)
        if (this.hasTarget) {
          this.loadMonitor()
        } else {
          this.monitor = makeMonitorState()
          this.disposeTrendCharts()
        }
      }
    }
  },
  created() {
    if (this.hasTarget) this.loadMonitor()
  },
  mounted() {
    window.addEventListener('resize', this.resizeTrendCharts)
  },
  beforeUnmount() {
    window.removeEventListener('resize', this.resizeTrendCharts)
    this.disposeTrendCharts()
  },
  methods: {
    pick(obj, keys, fallback = '') {
      if (!obj || typeof obj !== 'object') return fallback
      for (const key of keys) {
        const value = obj[key]
        if (value !== undefined && value !== null && value !== '') return value
      }
      return fallback
    },
    num(value, fallback = null) {
      if (value === '' || value === undefined || value === null) return fallback
      const next = Number(value)
      return Number.isNaN(next) ? fallback : next
    },
    toFiniteNumber(value, fallback = null) {
      if (value === '' || value === undefined || value === null) return fallback
      const next = typeof value === 'string' ? Number(value.replace(/[^\d.-]/g, '')) : Number(value)
      return Number.isFinite(next) ? next : fallback
    },
    bool(value, fallback = false) {
      if (value === true || value === false) return value
      if (value === 1 || value === '1') return true
      if (value === 0 || value === '0') return false
      if (typeof value === 'string') {
        const normalized = value.trim().toLowerCase()
        if (['true', 'yes', 'ok'].includes(normalized)) return true
        if (['false', 'no'].includes(normalized)) return false
      }
      return fallback
    },
    normalizeContext(context = {}) {
      return {
        assetId: this.num(this.pick(context, ['assetId', 'id']), ''),
        remoteManagementCard: String(this.pick(context, ['remoteManagementCard', 'remote_management_card'], '')).trim(),
        ip: String(this.pick(context, ['ip'], '')).trim(),
        brand: String(this.pick(context, ['brand'], '')).trim(),
        model: String(this.pick(context, ['model'], '')).trim(),
        purpose: String(this.pick(context, ['purpose'], '')).trim()
      }
    },
    normalizeAsset(item = {}) {
      return {
        id: this.num(this.pick(item, ['id']), ''),
        ip: String(this.pick(item, ['ip'], '')),
        brand: String(this.pick(item, ['brand'], '')),
        model: String(this.pick(item, ['model'], '')),
        purpose: String(this.pick(item, ['purpose'], '')),
        remoteManagementCard: String(this.pick(item, ['remoteManagementCard', 'remote_management_card'], ''))
      }
    },
    normalizeSummary(item = {}) {
      return {
        overallStatus: this.pick(item, ['overallStatus', 'overall_status'], ''),
        cpuStatus: this.pick(item, ['cpuStatus', 'cpu_status'], ''),
        memoryStatus: this.pick(item, ['memoryStatus', 'memory_status'], ''),
        diskStatus: this.pick(item, ['diskStatus', 'disk_status'], ''),
        fanStatus: this.pick(item, ['fanStatus', 'fan_status'], ''),
        powerStatus: this.pick(item, ['powerStatus', 'power_status'], ''),
        temperatureStatus: this.pick(item, ['temperatureStatus', 'temperature_status'], ''),
        cpuUsagePercent: this.num(this.pick(item, ['cpuUsagePercent', 'cpu_usage_percent']), null),
        memoryUsagePercent: this.num(this.pick(item, ['memoryUsagePercent', 'memory_usage_percent']), null),
        powerWatts: this.num(this.pick(item, ['powerWatts', 'power_watts']), null),
        highestTemperatureCelsius: this.num(this.pick(item, ['highestTemperatureCelsius', 'highest_temperature_celsius']), null)
      }
    },
    normalizeSource(item = {}) {
      if (!item || typeof item !== 'object') {
        return {
          job: String(item || ''),
          matchedLabel: '',
          prometheusTarget: ''
        }
      }
      return {
        job: String(this.pick(item, ['job'], '')),
        matchedLabel: String(this.pick(item, ['matchedLabel', 'matched_label'], '')),
        prometheusTarget: String(this.pick(item, ['prometheusTarget', 'prometheus_target', 'target', 'instance'], ''))
      }
    },
    normalizeHealth(item = {}) {
      if (!item || typeof item !== 'object') return makeHealth()
      return {
        score: this.toFiniteNumber(this.pick(item, ['score'], null), null),
        level: String(this.pick(item, ['level', 'status'], '')),
        conclusion: String(this.pick(item, ['conclusion', 'summary', 'message'], '')),
        riskCategories: this.normalizeStringList(this.pick(item, ['riskCategories', 'risk_categories'], [])),
        criticalCount: this.num(this.pick(item, ['criticalCount', 'critical_count'], 0), 0),
        warningCount: this.num(this.pick(item, ['warningCount', 'warning_count'], 0), 0),
        infoCount: this.num(this.pick(item, ['infoCount', 'info_count'], 0), 0),
        immediateAction: this.bool(this.pick(item, ['immediateAction', 'immediate_action'], false), false),
        recommendations: this.normalizeStringList(this.pick(item, ['recommendations', 'actions'], []))
      }
    },
    normalizeTrends(payload, components = null) {
      const fallback = makeTrends()
      if (Array.isArray(payload)) {
        fallback.panels = this.normalizeTrendPanels(payload, components)
        const range = this.resolveTrendRange(fallback.panels)
        fallback.from = range.from
        fallback.to = range.to
        return fallback
      }
      if (!payload || typeof payload !== 'object') return fallback
      const panels = this.normalizeTrendPanels(this.pick(payload, ['panels', 'items', 'list', 'charts'], []), components)
      const range = this.resolveTrendRange(panels)
      return {
        panels,
        from: this.pick(payload, ['from', 'start', 'startTime', 'start_time'], range.from),
        to: this.pick(payload, ['to', 'end', 'endTime', 'end_time'], range.to),
        hours: this.num(this.pick(payload, ['hours', 'trendHours', 'trend_hours', 'windowHours', 'window_hours']), null),
        stepMinutes: this.num(this.pick(payload, ['stepMinutes', 'step_minutes', 'trendStepMinutes', 'trend_step_minutes']), null)
      }
    },
    normalizeTrendPanels(payload, components = null) {
      if (Array.isArray(payload)) {
        return payload.map((item, index) => this.normalizeTrendPanel(item, index, '', components)).filter(Boolean)
      }
      if (payload && typeof payload === 'object') {
        return Object.keys(payload)
          .map((key, index) => this.normalizeTrendPanel(payload[key], index, key, components))
          .filter(Boolean)
      }
      return []
    },
    normalizeTrendPanel(item, index, fallbackKey = '', components = null) {
      if (!item || typeof item !== 'object') return null
      const rawKey = String(this.pick(item, ['key', 'code', 'id', 'type'], fallbackKey || `panel-${index}`))
      const title = String(this.pick(item, ['title', 'name', 'label', 'panelName', 'panel_name'], KNOWN_TREND_LABELS[rawKey] || this.humanizeKey(rawKey)))
      const series = this.normalizeTrendSeries(item, title, components)
      const gaugeSeries = series.filter(line => !this.isTrendStatusSeries(line))
      const statusSeries = series.filter(line => this.isTrendStatusSeries(line))
      const primarySeries = gaugeSeries.length ? gaugeSeries : series
      const primaryValueType = gaugeSeries.length ? 'gauge' : (statusSeries.length ? 'status' : 'gauge')
      const primaryUnit = String(this.pick(item, ['unit', 'valueUnit', 'value_unit'], primarySeries[0]?.unit || this.inferTrendUnit(title)))
      const primaryNumbers = this.collectTrendNumbers(primarySeries.length ? primarySeries : series)
      const latest = this.toFiniteNumber(
        this.pick(item, ['latest', 'latestValue', 'latest_value', 'current', 'currentValue', 'current_value']),
        this.lastFinite(primaryNumbers.length ? primaryNumbers : this.collectTrendNumbers(series))
      )
      const first = this.firstFinite(primaryNumbers)
      const delta = primaryValueType === 'status'
        ? null
        : this.toFiniteNumber(this.pick(item, ['delta', 'change', 'changeValue', 'change_value']), this.calcDelta(first, latest))
      const min = this.toFiniteNumber(this.pick(item, ['min', 'minValue', 'min_value']), primaryNumbers.length ? Math.min(...primaryNumbers) : null)
      const max = this.toFiniteNumber(this.pick(item, ['max', 'maxValue', 'max_value']), primaryNumbers.length ? Math.max(...primaryNumbers) : null)
      const avg = primaryValueType === 'status'
        ? null
        : this.toFiniteNumber(this.pick(item, ['avg', 'average', 'avgValue', 'avg_value']), primaryNumbers.length ? primaryNumbers.reduce((sum, value) => sum + value, 0) / primaryNumbers.length : null)
      const backendStatus = String(this.pick(item, ['status', 'state', 'healthStatus', 'health_status'], '')).trim()
      const inferredStatus = this.inferPanelStatus(title, latest, delta, primaryValueType)
      const status = backendStatus || inferredStatus
      const chartType = String(this.pick(item, ['chartType', 'chart_type'], 'line'))
      const resolvedChartType = !gaugeSeries.length && statusSeries.length ? 'heatmap' : chartType
      return {
        key: `${index}-${this.safeKey(rawKey)}`,
        title,
        description: String(this.pick(item, ['description', 'desc', 'remark', 'summary'], '')),
        backendStatus,
        inferredStatus,
        status,
        unit: primaryUnit,
        chartType: resolvedChartType,
        category: String(this.pick(item, ['category'], '')),
        windowHours: this.num(this.pick(item, ['windowHours', 'window_hours']), null),
        stepMinutes: this.num(this.pick(item, ['stepMinutes', 'step_minutes']), null),
        valueType: primaryValueType,
        series,
        hasData: series.some(line => line.data.length),
        metricNames: this.normalizeStringList(this.pick(item, ['metrics', 'metricNames', 'metric_names', 'sourceMetrics', 'source_metrics'], series.map(line => line.metric).filter(Boolean))),
        metrics: this.buildTrendMetrics({
          latest,
          delta,
          min,
          max,
          avg,
          unit: primaryUnit,
          valueType: primaryValueType,
          series,
          stepMinutes: this.num(this.pick(item, ['stepMinutes', 'step_minutes']), null) || this.normalizedTrendStepMinutes
        })
      }
    },
    normalizeTrendSeries(panel, fallbackName = '趋势', components = null) {
      const seriesPayload = this.pick(panel, ['series', 'lines', 'datasets', 'dataSeries', 'data_series'], null)
      if (Array.isArray(seriesPayload)) {
        return seriesPayload.map((item, index) => this.normalizeTrendLine(item, index, fallbackName, components)).filter(Boolean)
      }
      if (seriesPayload && typeof seriesPayload === 'object') {
        return Object.keys(seriesPayload)
          .map((name, index) => this.normalizeTrendLine({ name, data: seriesPayload[name] }, index, fallbackName, components))
          .filter(Boolean)
      }
      const points = this.normalizeTrendPoints(this.pick(panel, ['points', 'samples', 'data', 'values'], []))
      return points.length ? [{ name: fallbackName, data: points }] : []
    },
    normalizeTrendLine(item, index, fallbackName = '趋势', components = null) {
      if (Array.isArray(item)) {
        const points = this.normalizeTrendPoints(item)
        return points.length ? { name: `${fallbackName} ${index + 1}`, data: points, unit: '', axis: 'left', metric: '', valueType: 'gauge' } : null
      }
      if (!item || typeof item !== 'object') return null
      const name = String(this.pick(item, ['name', 'label', 'metric', 'device', 'slot', 'id'], `${fallbackName} ${index + 1}`))
      const metric = String(this.pick(item, ['metric'], ''))
      const valueType = this.normalizeTrendValueType(this.pick(item, ['valueType', 'value_type'], ''), metric)
      const unit = this.normalizeUnit(String(this.pick(item, ['unit'], this.inferMetricUnit(metric, name, valueType))))
      const component = String(this.pick(item, ['component'], ''))
      const axis = String(this.pick(item, ['axis'], valueType === 'status' ? 'right' : 'left'))
      const points = this.normalizeTrendPoints(this.pick(item, ['points', 'samples', 'data', 'values'], []))
        .map(point => ({
          ...point,
          value: this.sanitizeMetricValue(point.value, { metric, unit, name, component })
        }))
      if (!points.length) return null
      if (this.shouldDropTrendLine({ metric, component, points, components })) return null
      return {
        name,
        metric,
        valueType,
        unit,
        axis,
        component,
        data: points
      }
    },
    normalizeTrendPoints(payload) {
      if (!Array.isArray(payload)) return []
      return payload.map((item, index) => this.normalizeTrendPoint(item, index)).filter(point => point.rawValue !== '' || point.value !== null)
    },
    normalizeTrendPoint(point, index) {
      if (Array.isArray(point)) {
        const time = point[0] !== undefined ? point[0] : index
        const value = point[1] !== undefined ? point[1] : null
        return {
          time,
          label: this.formatTrendTime(time, index),
          rawValue: value,
          value: this.toFiniteNumber(value, null),
          originalCode: value,
          stateLabel: ''
        }
      }
      if (point && typeof point === 'object') {
        const time = this.pick(point, ['time', 'timestamp', 'ts', 'sampledAt', 'sampled_at', 'date', 'datetime', 'x'], index)
        const value = this.pick(point, ['value', 'y', 'avg', 'average', 'current', 'reading', 'rate', 'percent', 'score', 'remaining', 'count'], '')
        const originalCode = this.pick(point, ['rawValue', 'raw_value', 'rawStatus', 'raw_status', 'originalValue', 'original_value', 'originValue', 'origin_value'], value)
        const stateLabel = this.stringifyValue(this.pick(point, ['statusLabel', 'status_label', 'statusText', 'status_text', 'meaning', 'displayText', 'display_text', 'description', 'desc'], ''))
        return {
          time,
          label: this.formatTrendTime(time, index),
          rawValue: value,
          value: this.toFiniteNumber(value, null),
          originalCode,
          stateLabel
        }
      }
      return {
        time: index,
        label: this.formatTrendTime(index, index),
        rawValue: point,
        value: this.toFiniteNumber(point, null),
        originalCode: point,
        stateLabel: ''
      }
    },
    normalizeAlerts(payload) {
      if (!payload) return []
      if (Array.isArray(payload)) {
        return payload.map((item, index) => this.normalizeAlert(item, index)).filter(Boolean)
      }
      if (typeof payload === 'object') {
        return Object.keys(payload)
          .map((key, index) => this.normalizeAlert({ key, ...payload[key] }, index))
          .filter(Boolean)
      }
      return []
    },
    normalizeAlert(item, index) {
      if (!item || typeof item !== 'object') return null
      const severity = this.normalizeSeverity(this.pick(item, ['severity', 'level', 'alertLevel', 'alert_level', 'priority', 'status'], 'warning'))
      const code = String(this.pick(item, ['key', 'code', 'type', 'alertType', 'alert_type'], `alert-${index}`))
      const category = String(this.pick(item, ['category', 'group', 'panel'], ''))
      const metric = String(this.pick(item, ['metric', 'metricName', 'metric_name'], ''))
      const component = String(this.pick(item, ['component'], ''))
      const valueUnit = this.inferMetricUnit(metric, component, 'gauge')
      const titleParts = [component, this.humanizeCategory(category), this.humanizeMetric(metric)].filter(Boolean)
      return {
        key: `${index}-${this.safeKey(code)}`,
        severity,
        severityLabel: this.severityLabel(severity),
        title: String(this.pick(item, ['title', 'name', 'summary', 'alertName', 'alert_name'], titleParts.join(' / ') || this.humanizeKey(code))),
        message: this.stringifyValue(this.pick(item, ['message', 'description', 'detail', 'details', 'reason'], '')),
        categoryKey: category,
        category: category ? this.humanizeCategory(category) : '',
        metricKey: metric,
        metric: metric ? this.humanizeMetric(metric) : '',
        component,
        currentValue: this.formatValueWithUnit(this.pick(item, ['currentValue', 'current_value', 'value', 'reading'], ''), valueUnit),
        threshold: this.formatValueWithUnit(this.pick(item, ['threshold', 'limit', 'expected', 'rule'], ''), valueUnit),
        evidence: this.stringifyValue(this.pick(item, ['evidence'], '')),
        recommendation: this.stringifyValue(this.pick(item, ['recommendation', 'suggestion', 'action'], '')),
        durationMinutes: this.num(this.pick(item, ['durationMinutes', 'duration_minutes']), null),
        lastSeenAt: this.pick(item, ['lastSeenAt', 'last_seen_at', 'time', 'timestamp', 'createdAt', 'created_at', 'sampledAt', 'sampled_at'], '')
      }
    },
    normalizeMonitor(item = {}) {
      const components = this.pick(item, ['components'], null)
      const alerts = this.normalizeAlerts(this.pick(item, ['alerts', 'warnings'], []))
      return {
        asset: this.normalizeAsset(this.pick(item, ['asset'], {})),
        metricsAvailable: this.bool(this.pick(item, ['metricsAvailable', 'metrics_available'], false), false),
        vendor: String(this.pick(item, ['vendor'], '')),
        rawVendor: String(this.pick(item, ['rawVendor', 'raw_vendor'], '')),
        source: this.normalizeSource(this.pick(item, ['source'], {})),
        summary: this.normalizeSummary(this.pick(item, ['summary'], {})),
        components,
        flatComponents: this.flattenComponents(components, alerts),
        trends: this.normalizeTrends(this.pick(item, ['trends'], null), components),
        alerts,
        health: this.normalizeHealth(this.pick(item, ['health'], {})),
        sampledAt: this.pick(item, ['sampledAt', 'sampled_at'], '')
      }
    },
    decorateTrendPanels(panels = []) {
      return panels.map(panel => this.decorateTrendPanel(panel)).filter(Boolean)
    },
    decorateTrendPanel(panel = {}) {
      if (!panel || typeof panel !== 'object') return null
      const series = (panel.series || []).map(line => this.decorateTrendLine(line, panel)).filter(Boolean)
      const title = this.resolveDisplayTrendTitle(panel, series)
      const description = this.resolveDisplayTrendDescription(panel, series)
      const gaugeSeries = series.filter(line => !this.isTrendStatusSeries(line))
      const statusSeries = series.filter(line => this.isTrendStatusSeries(line))
      const primarySeries = gaugeSeries.length ? gaugeSeries : series
      const primaryValueType = gaugeSeries.length ? 'gauge' : (statusSeries.length ? 'status' : String(panel.valueType || 'gauge'))
      const unit = String(panel.unit || primarySeries[0]?.unit || this.inferTrendUnit(title))
      const primaryNumbers = this.collectTrendNumbers(primarySeries.length ? primarySeries : series)
      const latest = this.lastFinite(primaryNumbers.length ? primaryNumbers : this.collectTrendNumbers(series))
      const first = this.firstFinite(primaryNumbers)
      const delta = primaryValueType === 'status' ? null : this.calcDelta(first, latest)
      const min = primaryNumbers.length ? Math.min(...primaryNumbers) : null
      const max = primaryNumbers.length ? Math.max(...primaryNumbers) : null
      const avg = primaryValueType === 'status' || !primaryNumbers.length
        ? null
        : primaryNumbers.reduce((sum, value) => sum + value, 0) / primaryNumbers.length
      const panelCode = this.resolveTrendPanelCode(panel)
      const stepMinutes = this.num(panel.stepMinutes, null) || this.normalizedTrendStepMinutes
      const statusOverview = statusSeries.length ? this.summarizeStatusSeries(statusSeries, stepMinutes, panelCode) : null
      const isStatusDashboard = this.shouldUseStatusDashboard(panel, gaugeSeries, statusSeries)
      const inferredStatus = primaryValueType === 'status'
        ? (this.statusCodeToKeyword(statusOverview?.currentState) || 'unknown')
        : this.inferPanelStatus(title, latest, delta, primaryValueType)
      const status = this.resolveTrendPanelStatus(panel, inferredStatus)
      return {
        ...panel,
        title,
        description,
        series,
        unit,
        valueType: primaryValueType,
        chartType: isStatusDashboard ? 'dashboard-gauge' : (!gaugeSeries.length && statusSeries.length ? 'heatmap' : String(panel.chartType || 'line')),
        status,
        metricNames: this.resolveDisplayMetricNames(panel, series),
        metrics: this.buildTrendMetrics({
          latest,
          delta,
          min,
          max,
          avg,
          unit,
          valueType: primaryValueType,
          series,
          stepMinutes,
          panelCode,
          title,
          statusOverview
        }),
        statusOverview,
        isStatusDashboard
      }
    },
    resolveTrendPanelStatus(panel = {}, inferredStatus = 'unknown') {
      const backendStatus = String(this.pick(panel, ['backendStatus', 'status', 'state', 'healthStatus', 'health_status'], '')).trim()
      return backendStatus || inferredStatus || 'unknown'
    },
    resolveTrendPanelCode(panel = {}) {
      const raw = String(this.pick(panel, ['panelCode', 'code'], '') || panel.key || '').trim()
      return raw.replace(/^\d+-/, '')
    },
    resolveDisplayTrendTitle(panel = {}, series = []) {
      const panelCode = this.resolveTrendPanelCode(panel)
      if (panelCode === 'raid-health') return 'RAID 卡健康状态'
      if (panelCode !== 'disk-life') return String(panel.title || '')
      const metrics = series.map(line => String(line.metric || ''))
      const hasWearMetric = metrics.some(metric => ['physicalDiskRemainingRatedWriteEndurance', 'hardDiskRemnantWearout'].includes(metric))
      const hasDellWearMetric = metrics.includes('physicalDiskRemainingRatedWriteEndurance')
      const hasStatusMetric = metrics.some(metric => ['physicalDiskComponentStatus', 'hardDiskStatus'].includes(metric))
      if (hasWearMetric && hasStatusMetric) return hasDellWearMetric ? 'SSD 耐久度与磁盘健康' : '磁盘寿命与健康状态'
      if (hasWearMetric) return hasDellWearMetric ? 'SSD 耐久度趋势' : '磁盘寿命趋势'
      if (hasStatusMetric) return '磁盘健康状态'
      return String(panel.title || '磁盘寿命趋势')
    },
    resolveDisplayTrendDescription(panel = {}, series = []) {
      if (panel.description) return String(panel.description)
      const panelCode = this.resolveTrendPanelCode(panel)
      if (panelCode === 'raid-health') return '基于 RAID 控制器状态码生成的健康仪表盘'
      if (panelCode !== 'disk-life') return ''
      const metrics = series.map(line => String(line.metric || ''))
      const hasWearMetric = metrics.some(metric => ['physicalDiskRemainingRatedWriteEndurance', 'hardDiskRemnantWearout'].includes(metric))
      const hasStatusMetric = metrics.some(metric => ['physicalDiskComponentStatus', 'hardDiskStatus'].includes(metric))
      if (hasStatusMetric && !hasWearMetric) return '当前仅返回磁盘健康状态码，未返回 SSD endurance 百分比'
      if (hasWearMetric && hasStatusMetric) return '同时展示剩余耐久度与磁盘健康状态'
      return ''
    },
    resolveDisplayMetricNames(panel = {}, series = []) {
      const metrics = this.normalizeStringList(this.pick(panel, ['metricNames', 'metric_names'], series.map(line => line.metric).filter(Boolean)))
      return Array.from(new Set(metrics.map(metric => this.humanizeMetric(metric))))
    },
    decorateTrendLine(line = {}, panel = {}) {
      if (!line || typeof line !== 'object') return null
      if (!this.isTrendStatusSeries(line)) return { ...line }
      const shouldNormalizeRaw = this.shouldNormalizeRawStatusLine(line, panel)
      return {
        ...line,
        unit: shouldNormalizeRaw || !String(line.unit || '').trim() ? '状态码(1=正常,2=告警,3=严重)' : line.unit,
        data: (line.data || []).map(point => this.decorateTrendPoint(point, line, shouldNormalizeRaw))
      }
    },
    decorateTrendPoint(point = {}, line = {}, shouldNormalizeRaw = false) {
      const rawValue = this.toFiniteNumber(point?.value, null)
      const value = shouldNormalizeRaw ? this.mapRawStatusCodeToCanonical(line.metric, rawValue) : rawValue
      const normalizedStatus = this.normalizeStatusCode(value)
      return {
        ...point,
        value,
        stateLabel: point?.stateLabel || (Number.isFinite(normalizedStatus) ? this.formatStatusText(normalizedStatus) : '')
      }
    },
    shouldNormalizeRawStatusLine(line = {}, panel = {}) {
      if (!this.isTrendStatusSeries(line)) return false
      const unitText = String(line.unit || panel.unit || '').trim().toLowerCase()
      if (unitText.includes('status code') || /1\s*=\s*.*2\s*=\s*.*3\s*=/.test(unitText)) return false
      const latest = this.lastFinite((line.data || []).map(point => this.toFiniteNumber(point.value, null)))
      if (!Number.isFinite(latest)) return false
      const expected = this.resolveExpectedTrendStatusCode(panel, line)
      const mapped = this.mapRawStatusCodeToCanonical(line.metric, latest)
      if (Number.isFinite(expected) && latest !== expected && mapped === expected) return true
      return latest > 3 && mapped !== latest
    },
    resolveExpectedTrendStatusCode(panel = {}, line = {}) {
      const componentStatus = this.findTrendComponentStatus(line)
      const componentCode = this.statusTextToCode(componentStatus)
      if (Number.isFinite(componentCode)) return componentCode
      const summaryKeyMap = {
        controllerComponentStatus: 'diskStatus',
        raidControllerHealthStatus: 'diskStatus',
        logicalDriveConsistencyCheckStatus: 'diskStatus',
        physicalDiskComponentStatus: 'diskStatus',
        hardDiskStatus: 'diskStatus',
        powerSupplyStatus: 'powerStatus',
        powerSupplySensorState: 'powerStatus',
        memoryStatus: 'memoryStatus'
      }
      const summaryKey = summaryKeyMap[String(line.metric || '')]
      const summaryCode = this.statusTextToCode(summaryKey ? this.monitor.summary?.[summaryKey] : '')
      if (Number.isFinite(summaryCode)) return summaryCode
      if (['raid-health', 'disk-life'].includes(this.resolveTrendPanelCode(panel))) {
        const fallbackCode = this.statusTextToCode(this.monitor.summary?.diskStatus)
        if (Number.isFinite(fallbackCode)) return fallbackCode
      }
      return null
    },
    findTrendComponentStatus(line = {}) {
      const groups = TREND_STATUS_COMPONENT_GROUPS[String(line.metric || '')] || []
      const candidates = this.flatComponents.filter(row => !groups.length || groups.includes(String(row.groupKey || '')))
      const matched = candidates.find(row => this.matchTrendComponentRow(line, row))
      return matched ? matched.status : ''
    },
    matchTrendComponentRow(line = {}, row = {}) {
      const lineName = String(line.name || '').trim().toLowerCase()
      const rowName = String(row.name || '').trim().toLowerCase()
      const lineIndex = this.extractTrailingIndex(line.name || line.component)
      const rowIndex = this.extractTrailingIndex(row.name)
      if (lineName && lineName === rowName) return true
      if (lineIndex && rowIndex && lineIndex === rowIndex) return true
      return false
    },
    extractTrailingIndex(text = '') {
      const match = String(text || '').match(/(\d+)(?!.*\d)/)
      return match ? match[1] : ''
    },
    statusTextToCode(value) {
      const text = String(value || '').trim().toLowerCase()
      if (!text || text === '-') return null
      if (['ok', 'normal', 'healthy', 'success', 'up', 'good', '正常'].includes(text)) return 1
      if (['warning', 'warn', 'degraded', 'minor', '告警', '警告'].includes(text)) return 2
      if (['critical', 'error', 'down', 'failed', 'fault', 'abnormal', 'major', '异常', '严重'].includes(text)) return 3
      if (['unknown', 'n/a', 'na', '未知'].includes(text)) return 0
      return null
    },
    normalizeStatusCode(value) {
      const numeric = this.toFiniteNumber(value, null)
      if (!Number.isFinite(numeric)) return null
      if (numeric >= 3) return 3
      if (numeric === 2) return 2
      if (numeric === 1) return 1
      return 0
    },
    statusCodeToKeyword(value) {
      const code = this.normalizeStatusCode(value)
      if (!Number.isFinite(code)) return ''
      if (code >= 3) return 'critical'
      if (code === 2) return 'warning'
      if (code === 1) return 'normal'
      return 'unknown'
    },
    mapRawStatusCodeToCanonical(metric = '', value = null) {
      const numeric = this.toFiniteNumber(value, null)
      if (!Number.isFinite(numeric)) return numeric
      const vendor = String(this.monitor.vendor || this.monitor.rawVendor || '').trim().toLowerCase()
      const metricName = String(metric || '').trim()
      if (metricName === 'powerSupplySensorState') {
        if (numeric === 1) return 1
        if (numeric === 2 || numeric === 4) return 2
        if (numeric >= 3) return 3
        return 0
      }
      if (vendor === 'dell' || ['physicalDiskComponentStatus', 'controllerComponentStatus'].includes(metricName)) {
        if (numeric === 3) return 1
        if ([1, 2, 4].includes(numeric)) return 2
        if (numeric >= 5) return 3
        return 0
      }
      if (vendor === 'huawei' || ['hardDiskStatus', 'memoryStatus', 'powerSupplyStatus', 'raidControllerHealthStatus', 'logicalDriveConsistencyCheckStatus'].includes(metricName)) {
        if (numeric === 1 || numeric === 2) return 1
        if (numeric === 3 || numeric === 4) return 2
        if (numeric > 4) return 3
        return 0
      }
      return numeric
    },
    shouldUseStatusDashboard(panel = {}, gaugeSeries = [], statusSeries = []) {
      return !gaugeSeries.length && statusSeries.length > 0 && STATUS_DASHBOARD_PANEL_CODES.includes(this.resolveTrendPanelCode(panel))
    },
    resolveStatusSubjectLabel(panelCode = '', title = '') {
      if (panelCode === 'raid-health' || String(title || '').includes('RAID')) return '控制器'
      if (panelCode === 'disk-life' || String(title || '').includes('磁盘')) return '磁盘'
      return '组件'
    },
    formatFraction(value, total) {
      const current = Number(value)
      const count = Number(total)
      if (!Number.isFinite(current) || !Number.isFinite(count) || count <= 0) return '-'
      return `${current}/${count}`
    },
    buildStatusDashboardItems(items = []) {
      const abnormalItems = items
        .filter(item => item.state === 0 || item.state >= 2)
        .sort((a, b) => this.normalizeStatusCode(b.state) - this.normalizeStatusCode(a.state) || String(a.name || '').localeCompare(String(b.name || '')))
      const normalItems = items
        .filter(item => item.state === 1)
        .sort((a, b) => String(a.name || '').localeCompare(String(b.name || '')))
      const ordered = abnormalItems.length ? abnormalItems.concat(normalItems) : normalItems
      const displayItems = ordered.slice(0, 6).map(item => ({
        ...item,
        label: this.formatStatusText(this.normalizeStatusCode(item.state)),
        tagType: this.statusTagType(item.keyword)
      }))
      const remainingCount = Math.max(ordered.length - displayItems.length, 0)
      return {
        displayItems,
        remainingCount,
        moreText: remainingCount > 0 ? `其余 ${remainingCount} 个组件可在组件明细中查看` : ''
      }
    },
    flattenComponents(payload, alerts = []) {
      const rows = []
      this.visitComponentNode(payload, [], rows, alerts)
      return rows
    },
    visitComponentNode(value, path, rows, alerts = []) {
      if (Array.isArray(value)) {
        value.forEach((item, index) => this.visitComponentNode(item, path.concat(index), rows, alerts))
        return
      }

      if (!value || typeof value !== 'object') return

      if (this.isComponentLeaf(value)) {
        rows.push(this.normalizeComponentRow(value, path, rows.length, alerts))
        return
      }

      Object.keys(value).forEach(key => {
        this.visitComponentNode(value[key], path.concat(key), rows, alerts)
      })
    },
    isComponentLeaf(value) {
      if (!value || typeof value !== 'object' || Array.isArray(value)) return false
      const keys = Object.keys(value)
      if (!keys.length) return false
      const componentKeys = ['name', 'label', 'componentName', 'component', 'deviceName', 'status', 'healthStatus', 'state', 'type', 'category', 'reading', 'value', 'slot', 'memberId', 'id']
      if (keys.some(key => componentKeys.includes(key))) return true
      return keys.every(key => {
        const item = value[key]
        return item === null || ['string', 'number', 'boolean'].includes(typeof item)
      })
    },
    normalizeComponentRow(item, path, index, alerts = []) {
      const name = String(this.pick(item, ['name', 'label', 'componentName', 'component', 'deviceName', 'slot', 'memberId', 'id'], `组件 ${index + 1}`))
      const rawStatus = String(this.pick(item, ['status', 'healthStatus', 'health_status', 'state', 'overallStatus', 'componentStatus'], '-'))
      const detailKeys = ['name', 'label', 'componentName', 'component', 'deviceName', 'slot', 'memberId', 'id', 'status', 'healthStatus', 'health_status', 'state', 'overallStatus', 'componentStatus']
      const typeValue = String(this.pick(item, ['type', 'category', 'componentType', 'kind'], '')).trim()
      const reading = this.extractComponentReading(item)
      if (typeValue) detailKeys.push('type', 'category', 'componentType', 'kind')
      const readingKeys = ['reading', 'value', 'temperature', 'temperatureCelsius', 'temperatureProbeReading', 'powerWatts', 'speedRpm', 'rpm', 'usagePercent', 'cpuUsagePercent', 'memoryUsagePercent', 'amperageProbeReading', 'coolingDeviceReading']
      readingKeys.forEach(key => detailKeys.push(key))
      const stringPath = path.filter(itemPath => typeof itemPath === 'string')
      const groupPath = stringPath.length > 1 ? stringPath.slice(0, -1) : stringPath
      const groupKey = String(stringPath[0] || typeValue || 'component')
      const status = this.resolveComponentStatus({ name, rawStatus, groupKey, alerts })
      return {
        key: `component-${index}`,
        groupKey,
        group: groupPath.length ? groupPath.map(this.humanizeKey).join(' / ') : '组件',
        name,
        status,
        reading,
        details: this.formatComponentDetails(item, detailKeys)
      }
    },
    extractComponentReading(item = {}) {
      const explicitUnit = this.normalizeUnit(String(this.pick(item, ['unit'], '')))
      const readers = [
        { keys: ['reading', 'value'], suffix: explicitUnit },
        { keys: ['temperatureCelsius', 'temperature', 'temperatureProbeReading'], suffix: '°C' },
        { keys: ['powerWatts'], suffix: 'W' },
        { keys: ['speedRpm', 'rpm'], suffix: 'RPM' },
        { keys: ['usagePercent', 'cpuUsagePercent', 'memoryUsagePercent'], suffix: '%' },
        { keys: ['remainingRatedWriteEndurance', 'physicalDiskRemainingRatedWriteEndurance'], suffix: '%' },
        { keys: ['remnantWearout', 'hardDiskRemnantWearout'], suffix: '%' },
        { keys: ['powerOnHours', 'hardDiskPowerOnHours'], suffix: 'h' },
        { keys: ['amperageProbeReading'], suffix: 'A' },
        { keys: ['coolingDeviceReading'], suffix: '' }
      ]
      for (const reader of readers) {
        const value = this.pick(item, reader.keys, '')
        if (value === '' || value === undefined || value === null) continue
        return this.formatValueWithUnit(value, reader.suffix)
      }
      return '-'
    },
    formatComponentDetails(item, ignoredKeys = []) {
      const ignoreSet = new Set(ignoredKeys)
      const entries = Object.keys(item)
        .filter(key => !ignoreSet.has(key))
        .map(key => `${this.humanizeKey(key)}: ${this.stringifyValue(item[key])}`)
        .filter(Boolean)
      return entries.length ? entries.join(' | ') : '-'
    },
    resolveTrendRange(panels = []) {
      let from = null
      let to = null
      panels.forEach(panel => {
        panel.series.forEach(line => {
          line.data.forEach(point => {
            const date = this.toDate(point.time)
            if (!date) return
            const time = date.getTime()
            if (from === null || time < from) from = time
            if (to === null || time > to) to = time
          })
        })
      })
      return {
        from: from === null ? '' : new Date(from).toISOString(),
        to: to === null ? '' : new Date(to).toISOString()
      }
    },
    normalizeTrendValueType(value, metric = '') {
      const text = String(value || '').trim().toLowerCase()
      if (['status', 'state', 'health', 'enum'].includes(text)) return 'status'
      const metricText = String(metric || '').trim().toLowerCase()
      if (metricText.includes('status') || metricText.includes('state')) return 'status'
      return 'gauge'
    },
    isTrendStatusSeries(line = {}) {
      return this.normalizeTrendValueType(line.valueType, line.metric) === 'status'
    },
    inferMetricUnit(metric = '', name = '', valueType = '') {
      if (this.normalizeTrendValueType(valueType, metric) === 'status') return ''
      const text = `${metric} ${name}`.toLowerCase()
      if (text.includes('temperature')) return '°C'
      if (text.includes('coolingdevice') || text.includes('fan') || text.includes('rpm')) return 'RPM'
      if (text.includes('power') || text.includes('watt')) return 'W'
      if (text.includes('amperage')) return 'A'
      if (text.includes('percent') || text.includes('usage')) return '%'
      if (text.includes('hour')) return 'h'
      return ''
    },
    formatValueWithUnit(value, unit = '') {
      const text = this.stringifyValue(value)
      if (!text) return ''
      const normalizedUnit = this.normalizeUnit(unit)
      if (!normalizedUnit) return text
      if (/[a-zA-Z%°]/.test(text)) return text
      return `${text}${normalizedUnit}`
    },
    statusRank(value) {
      const text = String(value || '').trim().toLowerCase()
      if (['critical', 'error', 'fatal', 'major', 'high', 'danger', '严重', '异常'].includes(text)) return 3
      if (['warning', 'warn', 'minor', 'medium', 'degraded', '告警', '警告'].includes(text)) return 2
      if (['info', 'notice', 'low', '提示'].includes(text)) return 1
      if (['ok', 'normal', 'healthy', 'success', 'up', 'good', '正常'].includes(text)) return 0
      return -1
    },
    rankToStatus(rank) {
      if (rank >= 3) return 'critical'
      if (rank === 2) return 'warning'
      if (rank === 1) return 'info'
      if (rank === 0) return 'normal'
      return ''
    },
    mergeStatuses(...values) {
      const candidates = values.flat().filter(item => item !== undefined && item !== null && item !== '')
      const rank = candidates.reduce((max, item) => Math.max(max, this.statusRank(item)), -1)
      return this.rankToStatus(rank)
    },
    getComponentStatusesByGroup(groups = []) {
      const normalized = groups.map(item => String(item || '').toLowerCase())
      return this.flatComponents
        .filter(row => normalized.includes(String(row.groupKey || '').toLowerCase()))
        .map(row => row.status)
    },
    getAlertSeveritiesByCategory(categories = []) {
      const normalized = categories.map(item => String(item || '').toLowerCase())
      return this.alertRows
        .filter(row => normalized.includes(String(row.categoryKey || '').toLowerCase()))
        .map(row => row.severity)
    },
    resolveSummaryStatus(key, rawValue) {
      const summary = this.monitor.summary || makeSummary()
      const mapping = {
        cpuStatus: [rawValue].filter(Boolean),
        memoryStatus: [rawValue].concat(this.getAlertSeveritiesByCategory(['memory', 'stability'])),
        diskStatus: [rawValue].concat(this.getComponentStatusesByGroup(['disks'])).concat(this.getAlertSeveritiesByCategory(['storage', 'lifecycle'])),
        fanStatus: [rawValue].concat(this.getComponentStatusesByGroup(['fans'])),
        powerStatus: [rawValue].concat(this.getComponentStatusesByGroup(['powerSupplies'])).concat(this.getAlertSeveritiesByCategory(['power'])),
        temperatureStatus: [rawValue].concat(this.getComponentStatusesByGroup(['temperatures'])).concat(this.getAlertSeveritiesByCategory(['thermal'])),
        overallStatus: [
          rawValue,
          summary.cpuStatus,
          summary.memoryStatus,
          summary.diskStatus,
          summary.fanStatus,
          summary.powerStatus,
          summary.temperatureStatus,
          this.health.level,
          ...this.alertRows.map(item => item.severity),
          ...this.flatComponents.map(item => item.status)
        ]
      }
      return this.mergeStatuses(mapping[key] || [rawValue]) || rawValue
    },
    resolveComponentStatus({ name, rawStatus, groupKey, alerts = [] }) {
      const exactAlerts = alerts
        .filter(alert => String(alert.component || '').trim() === String(name || '').trim())
        .map(alert => alert.severity)
      const categoryAlerts = String(groupKey || '').toLowerCase() === 'temperatures'
        ? alerts.filter(alert => String(alert.categoryKey || '').toLowerCase() === 'thermal' && String(alert.component || '').trim() === String(name || '').trim()).map(alert => alert.severity)
        : []
      return this.mergeStatuses(rawStatus, exactAlerts, categoryAlerts) || rawStatus
    },
    buildTrendMetrics({ latest, delta, min, max, avg, unit = '', valueType = 'gauge', series = [], stepMinutes = 0, panelCode = '', title = '', statusOverview = null }) {
      if (this.normalizeTrendValueType(valueType) === 'status') {
        const summary = statusOverview || this.summarizeStatusSeries(series, stepMinutes, panelCode)
        const subject = this.resolveStatusSubjectLabel(panelCode, title)
        return [
          { label: '当前状态', value: this.formatStatusText(summary.currentState), tone: this.statusMetricTone(summary.currentState) },
          { label: `正常${subject}`, value: this.formatFraction(summary.normalCount, summary.totalCount), tone: summary.affectedCount > 0 ? '' : 'success' },
          { label: '当前健康占比', value: this.formatPercent(summary.currentHealthyRatio), tone: summary.affectedCount > 0 ? 'up' : 'success' },
          { label: '24h正常占比', value: this.formatPercent(summary.normalRatio), tone: '' }
        ]
      }
      return [
        { label: '最新', value: this.formatTrendValue(latest, unit), tone: '' },
        { label: '变化', value: this.formatTrendDelta(delta, unit), tone: this.deltaTone(delta) },
        { label: '最小', value: this.formatTrendValue(min, unit), tone: '' },
        { label: '最大', value: this.formatTrendValue(max, unit), tone: '' },
        { label: '平均', value: this.formatTrendValue(avg, unit), tone: '' }
      ]
    },
    formatTrendState(value) {
      return Number.isFinite(value) ? this.formatStatusNumeric(value) : '-'
    },
    formatStatusText(value) {
      if (!Number.isFinite(value)) return '-'
      const mapping = {
        0: '未知',
        1: '正常',
        2: '告警',
        3: '严重',
        4: '严重'
      }
      return mapping[value] || '状态'
    },
    statusMetricTone(value) {
      if (!Number.isFinite(value)) return ''
      if (value === 0) return ''
      if (value >= 3) return 'danger'
      if (value === 2) return 'up'
      return 'success'
    },
    summarizeStatusSeries(series = [], stepMinutes = 0, panelCode = '') {
      const statusSeries = series.filter(line => this.isTrendStatusSeries(line))
      if (!statusSeries.length) {
        return {
          currentState: null,
          durationMinutes: null,
          normalRatio: null,
          changeCount: 0,
          totalCount: 0,
          normalCount: 0,
          warningCount: 0,
          criticalCount: 0,
          unknownCount: 0,
          affectedCount: 0,
          currentHealthyRatio: null,
          latestItems: [],
          displayItems: [],
          remainingCount: 0,
          moreText: '',
          panelCode
        }
      }

      const latestStates = statusSeries
        .map(line => ({
          line,
          state: this.lastFinite(line.data.map(point => point.value))
        }))
        .filter(item => Number.isFinite(item.state))

      const currentState = latestStates.length ? Math.max(...latestStates.map(item => item.state)) : null
      const representative = latestStates
        .filter(item => item.state === currentState)
        .map(item => ({
          ...item,
          durationMinutes: this.calculateStatusDurationMinutes(item.line, currentState, stepMinutes)
        }))
        .sort((a, b) => (b.durationMinutes || 0) - (a.durationMinutes || 0))[0]

      const allValues = statusSeries.flatMap(line => line.data.map(point => point.value).filter(value => Number.isFinite(value)))
      const normalRatio = allValues.length ? (allValues.filter(value => value === 1).length / allValues.length) * 100 : null
      const changeCount = statusSeries.reduce((sum, line) => sum + this.countStatusChanges(line), 0)
      const latestItems = latestStates.map(item => ({
        name: item.line.name,
        state: item.state,
        keyword: this.statusCodeToKeyword(item.state)
      }))
      const totalCount = latestItems.length
      const normalCount = latestItems.filter(item => item.state === 1).length
      const warningCount = latestItems.filter(item => item.state === 2).length
      const criticalCount = latestItems.filter(item => item.state >= 3).length
      const unknownCount = latestItems.filter(item => item.state === 0).length
      const affectedCount = totalCount - normalCount
      const currentHealthyRatio = totalCount ? (normalCount / totalCount) * 100 : null
      const dashboardItems = this.buildStatusDashboardItems(latestItems)

      return {
        currentState,
        durationMinutes: representative?.durationMinutes ?? null,
        normalRatio,
        changeCount,
        totalCount,
        normalCount,
        warningCount,
        criticalCount,
        unknownCount,
        affectedCount,
        currentHealthyRatio,
        latestItems,
        displayItems: dashboardItems.displayItems,
        remainingCount: dashboardItems.remainingCount,
        moreText: dashboardItems.moreText,
        panelCode
      }
    },
    calculateStatusDurationMinutes(line = {}, state = null, stepMinutes = 0) {
      if (!line?.data?.length || !Number.isFinite(state)) return null
      let streak = 0
      for (let index = line.data.length - 1; index >= 0; index -= 1) {
        if (line.data[index].value !== state) break
        streak += 1
      }
      if (!streak) return null
      const effectiveStep = Number.isFinite(stepMinutes) && stepMinutes > 0 ? stepMinutes : this.normalizedTrendStepMinutes
      return streak * effectiveStep
    },
    countStatusChanges(line = {}) {
      const values = (line?.data || []).map(point => point.value).filter(value => Number.isFinite(value))
      if (!values.length) return 0
      let count = 0
      for (let index = 1; index < values.length; index += 1) {
        if (values[index] !== values[index - 1]) count += 1
      }
      return count
    },
    formatDurationMinutes(value) {
      if (!Number.isFinite(value) || value <= 0) return '-'
      const hours = Math.floor(value / 60)
      const minutes = Math.round(value % 60)
      if (hours && minutes) return `${hours}小时 ${minutes}分`
      if (hours) return `${hours}小时`
      return `${minutes}分`
    },
    formatTrendSeriesValue(value, line = {}) {
      if (!Number.isFinite(Number(value))) return '-'
      if (this.isTrendStatusSeries(line)) return this.formatTrendState(Number(value))
      return this.formatTrendValue(Number(value), line.unit || '')
    },
    formatStatusAxisLabel(value) {
      const mapping = {
        0: '未知',
        1: '正常',
        2: '告警',
        3: '严重',
        4: '严重'
      }
      return mapping[Number(value)] || String(value)
    },
    buildTrendAxisConfig(series = [], fallbackUnit = '') {
      if (!series.length) return null
      const statusOnly = series.every(line => this.isTrendStatusSeries(line))
      if (statusOnly) {
        return {
          type: 'value',
          name: '状态',
          min: 0,
          max: 4,
          interval: 1,
          nameTextStyle: { color: '#98a2b3', fontSize: 11 },
          splitLine: { lineStyle: { color: '#edf2f7', type: 'dashed' } },
          axisLabel: {
            color: '#7b8798',
            fontSize: 10,
            formatter: value => this.formatStatusAxisLabel(value)
          }
        }
      }
      const axisUnit = series.find(line => line.unit)?.unit || fallbackUnit
      return {
        type: 'value',
        name: axisUnit || '',
        nameTextStyle: { color: '#98a2b3', fontSize: 11 },
        splitLine: { lineStyle: { color: '#edf2f7', type: 'dashed' } },
        axisLabel: {
          color: '#7b8798',
          fontSize: 10,
          formatter: value => this.formatTrendValue(Number(value), axisUnit)
        }
      }
    },
    stringifyValue(value) {
      if (value === '' || value === undefined || value === null) return ''
      if (Array.isArray(value)) return value.map(item => this.stringifyValue(item)).filter(Boolean).join(', ')
      if (typeof value === 'object') {
        try {
          return JSON.stringify(value)
        } catch (error) {
          return '[object]'
        }
      }
      return String(value)
    },
    normalizeStringList(value) {
      if (Array.isArray(value)) {
        return value.map(item => this.stringifyValue(item)).filter(Boolean)
      }
      if (typeof value === 'string') {
        return value.split(',').map(item => item.trim()).filter(Boolean)
      }
      if (value && typeof value === 'object') return Object.keys(value)
      return []
    },
    humanizeKey(value) {
      const raw = String(value || '')
      return KNOWN_TREND_LABELS[raw] || raw
        .replace(/_/g, ' ')
        .replace(/-/g, ' ')
        .replace(/([a-z])([A-Z])/g, '$1 $2')
        .replace(/\s+/g, ' ')
        .trim()
        .replace(/^./, char => char.toUpperCase())
    },
    humanizeCategory(value) {
      const mapping = {
        power: '电源',
        thermal: '散热',
        lifecycle: '寿命',
        storage: '存储',
        raid: 'RAID',
        stability: '稳定性',
        memory: '内存'
      }
      return mapping[String(value || '').trim()] || this.humanizeKey(value)
    },
    humanizeMetric(value) {
      const mapping = {
        powerSupplyStatus: '电源冗余状态',
        powerStatus: '电源状态',
        hardDiskPowerOnHours: '磁盘通电时长',
        hardDiskRemnantWearout: '磁盘剩余寿命',
        fanSpeed: '风扇转速',
        temperatureReading: '温度读数',
        raidControllerDDREccCount: 'RAID ECC 计数',
        logicalDriveConsistencyCheckStatus: '逻辑盘一致性检查',
        memoryStatus: '内存状态'
      }
      return mapping[String(value || '').trim()] || this.humanizeKey(value)
    },
    normalizeUnit(value) {
      const mapping = {
        hours: 'h',
        hour: 'h',
        count: '次'
      }
      return mapping[String(value || '').trim()] || String(value || '')
    },
    safeKey(value) {
      return String(value || 'item')
        .replace(/[^a-zA-Z0-9_-]/g, '-')
        .replace(/-+/g, '-')
        .replace(/^-|-$/g, '') || 'item'
    },
    collectTrendNumbers(series) {
      const numbers = []
      series.forEach(line => {
        line.data.forEach(point => {
          if (Number.isFinite(point.value)) numbers.push(point.value)
        })
      })
      return numbers
    },
    firstFinite(values) {
      return values.find(value => Number.isFinite(value)) ?? null
    },
    lastFinite(values) {
      for (let index = values.length - 1; index >= 0; index -= 1) {
        if (Number.isFinite(values[index])) return values[index]
      }
      return null
    },
    calcDelta(first, latest) {
      if (!Number.isFinite(first) || !Number.isFinite(latest)) return null
      return latest - first
    },
    deltaTone(delta) {
      if (!Number.isFinite(delta) || delta === 0) return ''
      return delta > 0 ? 'up' : 'down'
    },
    inferTrendUnit(title) {
      const text = String(title || '').toLowerCase()
      if (text.includes('温度') || text.includes('temperature')) return '°C'
      if (text.includes('时长') || text.includes('hours')) return 'h'
      if (text.includes('ecc') || text.includes('count')) return '次'
      if (text.includes('转速') || text.includes('fan')) return 'RPM'
      if (text.includes('寿命') || text.includes('磨损') || text.includes('效率') || text.includes('冗余') || text.includes('稳定性')) return '%'
      return ''
    },
    inferPanelStatus(title, latest, delta, valueType = 'gauge') {
      const text = String(title || '')
      if (!Number.isFinite(latest)) return 'unknown'
      if (this.normalizeTrendValueType(valueType) === 'status') {
        if (latest >= 3) return 'critical'
        if (latest === 2) return 'warning'
        if (latest === 1) return 'normal'
        return 'unknown'
      }
      if ((text.includes('寿命') || text.includes('耐久') || text.includes('效率')) && latest < 20) return 'critical'
      if ((text.includes('寿命') || text.includes('耐久') || text.includes('效率')) && latest < 40) return 'warning'
      if (text.includes('温度') && latest >= 70) return 'critical'
      if (text.includes('温度') && latest >= 55) return 'warning'
      if ((text.includes('寿命') || text.includes('磨损')) && Number.isFinite(delta) && delta < -10) return 'warning'
      return 'normal'
    },
    sanitizeMetricValue(value, context = {}) {
      if (!Number.isFinite(value)) return value
      const metric = String(context.metric || '').toLowerCase()
      const unit = String(context.unit || '').toLowerCase()
      if ((metric.includes('temperature') || unit.includes('°c')) && (value > 1000 || value < -50)) return null
      return value
    },
    shouldDropTrendLine({ metric, component, points, components }) {
      const metricName = String(metric || '')
      const componentIndex = Number(component)
      const diskCount = Array.isArray(components?.disks) ? components.disks.length : 0
      if (!Number.isFinite(componentIndex) || !diskCount) return false
      if (!['hardDiskPowerOnHours', 'hardDiskRemnantWearout', 'physicalDiskRemainingRatedWriteEndurance'].includes(metricName)) return false
      const values = points.map(point => point.value).filter(value => Number.isFinite(value))
      if (!values.length) return false
      return componentIndex > diskCount && values.every(value => value <= 1)
    },
    formatPercent(value) {
      const next = Number(value)
      if (!Number.isFinite(next)) return '-'
      return `${next.toFixed(1)}%`
    },
    formatMetric(value, suffix = '') {
      const next = Number(value)
      if (!Number.isFinite(next)) return '-'
      return `${next.toFixed(1)}${suffix}`
    },
    formatTrendValue(value, unit = '') {
      if (!Number.isFinite(value)) return '-'
      const decimals = Math.abs(value) >= 100 ? 0 : 1
      return `${value.toFixed(decimals)}${unit}`
    },
    formatTrendDelta(value, unit = '') {
      if (!Number.isFinite(value)) return '-'
      const decimals = Math.abs(value) >= 100 ? 0 : 1
      return `${value > 0 ? '+' : ''}${value.toFixed(decimals)}${unit}`
    },
    formatStatusNumeric(value) {
      if (!Number.isFinite(value)) return '-'
      const mapping = {
        0: '未知',
        1: '正常',
        2: '告警',
        3: '严重',
        4: '严重'
      }
      return `${value} / ${mapping[value] || '状态'}`
    },
    formatDateTime(value) {
      if (!value && value !== 0) return '-'
      const date = this.toDate(value)
      if (date) {
        const pad = item => String(item).padStart(2, '0')
        return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
      }
      return String(value).replace('T', ' ').slice(0, 19)
    },
    formatTrendTime(value, index) {
      const date = this.toDate(value)
      if (date) {
        const pad = item => String(item).padStart(2, '0')
        return `${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
      }
      if (value === '' || value === undefined || value === null) return `#${index + 1}`
      return String(value)
    },
    toDate(value) {
      if (value === '' || value === undefined || value === null) return null
      if (value instanceof Date && !Number.isNaN(value.getTime())) return value
      if (typeof value === 'number') {
        const time = value > 1000000000000 ? value : value > 1000000000 ? value * 1000 : null
        if (!time) return null
        const date = new Date(time)
        return Number.isNaN(date.getTime()) ? null : date
      }
      const date = new Date(value)
      return Number.isNaN(date.getTime()) ? null : date
    },
    normalizeStatusLabel(value) {
      const text = String(value || '').trim()
      if (!text) return '-'
      const normalized = text.toLowerCase()
      if (['ok', 'normal', 'healthy', 'success', 'up', 'good'].includes(normalized)) return '正常'
      if (['warning', 'warn', 'degraded', 'minor'].includes(normalized)) return '告警'
      if (['critical', 'error', 'down', 'failed', 'fault', 'abnormal', 'major'].includes(normalized)) return '异常'
      if (['unknown', 'n/a', 'na'].includes(normalized)) return '未知'
      return text
    },
    formatStatusValue(value) {
      const normalized = this.normalizeStatusLabel(value)
      return normalized || '-'
    },
    statusTagType(value) {
      const text = String(value || '').trim().toLowerCase()
      if (!text || text === '-') return 'info'
      if (['ok', 'normal', 'healthy', 'success', 'up', 'good', '正常'].includes(text)) return 'success'
      if (['warning', 'warn', 'degraded', 'minor', '告警'].includes(text)) return 'warning'
      if (['critical', 'error', 'down', 'failed', 'fault', 'abnormal', 'major', '异常'].includes(text)) return 'danger'
      return 'info'
    },
    normalizeSeverity(value) {
      const text = String(value || '').trim().toLowerCase()
      if (['critical', 'error', 'fatal', 'major', 'high', 'danger', '严重', '异常'].includes(text)) return 'critical'
      if (['warning', 'warn', 'minor', 'medium', 'degraded', '告警', '警告'].includes(text)) return 'warning'
      if (['info', 'notice', 'low', 'normal', 'ok', '提示', '正常'].includes(text)) return 'info'
      return 'warning'
    },
    severityLabel(value) {
      if (value === 'critical') return '严重'
      if (value === 'warning') return '告警'
      if (value === 'info') return '提示'
      return '未知'
    },
    alertTagType(value) {
      if (value === 'critical') return 'danger'
      if (value === 'warning') return 'warning'
      return 'info'
    },
    getVendorTagType(vendor) {
      const normalized = String(vendor || '').trim().toLowerCase()
      if (normalized.includes('huawei') || normalized.includes('xfusion')) return 'success'
      if (normalized.includes('dell')) return 'primary'
      return 'info'
    },
    chartDomId(panel) {
      return `${this.chartSeed}-${panel.key}`
    },
    buildTrendChartOption(panel) {
      const labels = []
      panel.series.forEach(line => {
        line.data.forEach(point => {
          if (!labels.includes(point.label)) labels.push(point.label)
        })
      })
      const colors = ['#4c8bf5', '#3fb97f', '#f3b24b', '#ef6b73', '#14b8a6', '#6366f1', '#f97316']
      if (panel.isStatusDashboard) {
        return this.buildStatusDashboardOption(panel)
      }
      if (panel.chartType === 'heatmap') {
        return this.buildHeatmapOption(panel, labels)
      }
      const leftSeries = panel.series.filter(line => line.axis !== 'right')
      const rightSeries = panel.series.filter(line => line.axis === 'right')
      const hasLeftAxis = leftSeries.length > 0
      const yAxis = [this.buildTrendAxisConfig(leftSeries, panel.unit)].filter(Boolean)
      if (rightSeries.length) {
        const rightAxis = this.buildTrendAxisConfig(rightSeries, rightSeries[0]?.unit || '')
        if (rightAxis) {
          rightAxis.splitLine = { show: false }
          yAxis.push(rightAxis)
        }
      }
      return {
        backgroundColor: 'transparent',
        color: colors,
        tooltip: {
          trigger: 'axis',
          confine: true,
          formatter: params => {
            const title = params?.[0]?.axisValue || ''
            const lines = params.map(item => {
              const line = panel.series[item.seriesIndex] || {}
              return `${item.marker}${item.seriesName}: ${this.formatTrendSeriesValue(item.value, line)}`
            })
            return [title].concat(lines).join('<br/>')
          }
        },
        legend: {
          type: 'scroll',
          top: 0,
          right: 6,
          textStyle: { color: '#667085', fontSize: 11 }
        },
        grid: {
          top: 34,
          left: 44,
          right: 16,
          bottom: 32
        },
        xAxis: {
          type: 'category',
          boundaryGap: false,
          data: labels,
          axisTick: { show: false },
          axisLine: { lineStyle: { color: '#d8e0ec' } },
          axisLabel: { color: '#7b8798', fontSize: 10 }
        },
        yAxis,
        series: panel.series.map((line, index) => {
          const color = colors[index % colors.length]
          const map = line.data.reduce((result, point) => {
            result[point.label] = point.value
            return result
          }, {})
          return {
            name: line.name,
            type: 'line',
            smooth: !this.isTrendStatusSeries(line),
            step: panel.chartType === 'step-line' || this.isTrendStatusSeries(line) ? 'middle' : false,
            showSymbol: !this.isTrendStatusSeries(line) && line.data.length <= 24,
            symbolSize: 5,
            connectNulls: true,
            yAxisIndex: line.axis === 'right' && hasLeftAxis ? 1 : 0,
            data: labels.map(label => map[label] ?? null),
            lineStyle: { color, width: 2 },
            itemStyle: { color },
            areaStyle: index < 2 && !this.isTrendStatusSeries(line) ? {
              color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                { offset: 0, color: `${color}33` },
                { offset: 1, color: `${color}05` }
              ])
            } : undefined
          }
        })
      }
    },
    buildStatusDashboardOption(panel) {
      const summary = panel.statusOverview || this.summarizeStatusSeries(panel.series, this.normalizedTrendStepMinutes, this.resolveTrendPanelCode(panel))
      const ratio = this.toFiniteNumber(summary.currentHealthyRatio, 0) || 0
      const state = this.normalizeStatusCode(summary.currentState)
      const color = state >= 3 ? '#ef4444' : state === 2 || summary.unknownCount > 0 ? '#f59e0b' : '#22c55e'
      return {
        backgroundColor: 'transparent',
        tooltip: {
          confine: true,
          formatter: () => [
            panel.title,
            `当前状态: ${this.formatStatusText(state)}`,
            `当前健康占比: ${this.formatPercent(ratio)}`,
            `正常组件: ${this.formatFraction(summary.normalCount, summary.totalCount)}`,
            `异常组件: ${summary.affectedCount || 0}`
          ].join('<br/>')
        },
        series: [{
          name: panel.title,
          type: 'gauge',
          min: 0,
          max: 100,
          startAngle: 210,
          endAngle: -30,
          splitNumber: 4,
          center: ['50%', '56%'],
          radius: '94%',
          pointer: { show: false },
          anchor: { show: false },
          progress: {
            show: true,
            roundCap: true,
            width: 18,
            itemStyle: { color }
          },
          axisLine: {
            roundCap: true,
            lineStyle: {
              width: 18,
              color: [[1, '#e5e7eb']]
            }
          },
          axisTick: { show: false },
          splitLine: { show: false },
          axisLabel: { show: false },
          title: {
            show: true,
            offsetCenter: [0, '66%'],
            color: '#6b7280',
            fontSize: 12
          },
          detail: {
            offsetCenter: [0, '6%'],
            formatter: () => `{value|${Math.round(ratio)}%}\n{meta|${this.formatFraction(summary.normalCount, summary.totalCount)} 正常}`,
            rich: {
              value: {
                fontSize: 28,
                fontWeight: 700,
                color: '#111827',
                lineHeight: 34
              },
              meta: {
                fontSize: 12,
                color: '#6b7280',
                lineHeight: 18
              }
            }
          },
          data: [{
            value: ratio,
            name: '健康占比'
          }]
        }]
      }
    },
    buildHeatmapOption(panel, labels) {
      const yLabels = panel.series.map(line => line.name)
      const data = []
      panel.series.forEach((line, yIndex) => {
        const map = line.data.reduce((result, point) => {
          result[point.label] = point
          return result
        }, {})
        labels.forEach((label, xIndex) => {
          const point = map[label]
          if (Number.isFinite(point?.value)) {
            data.push({
              value: [xIndex, yIndex, point.value],
              rawValue: point.rawValue,
              originalCode: point.originalCode,
              stateLabel: point.stateLabel,
              seriesName: line.name,
              timeLabel: label
            })
          }
        })
      })
      return {
        backgroundColor: 'transparent',
        tooltip: {
          position: 'top',
          confine: true,
          formatter: params => {
            const [xIndex, yIndex, value] = params.value || []
            const current = params.data || {}
            const rawCode = current.originalCode ?? current.rawValue
            const lines = [
              `${yLabels[yIndex] || ''}`,
              `${labels[xIndex] || ''}`,
              `状态值: ${this.formatStatusNumeric(value)}`
            ]
            if (Number.isFinite(Number(rawCode)) && Number(rawCode) !== Number(value)) {
              lines.push(`原始码: ${rawCode}`)
            }
            if (current.stateLabel) {
              lines.push(`解释: ${current.stateLabel}`)
            } else {
              lines.push(`解释: ${this.formatStatusText(Number(value))}`)
            }
            return lines.join('<br/>')
          }
        },
        grid: {
          top: 24,
          left: 72,
          right: 24,
          bottom: 40
        },
        xAxis: {
          type: 'category',
          data: labels,
          splitArea: { show: true },
          axisTick: { show: false },
          axisLine: { lineStyle: { color: '#d8e0ec' } },
          axisLabel: { color: '#7b8798', fontSize: 10 }
        },
        yAxis: {
          type: 'category',
          data: yLabels,
          splitArea: { show: true },
          axisTick: { show: false },
          axisLine: { lineStyle: { color: '#d8e0ec' } },
          axisLabel: { color: '#7b8798', fontSize: 10 }
        },
        visualMap: {
          min: 0,
          max: 4,
          calculable: false,
          orient: 'horizontal',
          left: 'center',
          bottom: 0,
          textStyle: { color: '#7b8798', fontSize: 10 },
          pieces: [
            { value: 0, label: '未知', color: '#e5e7eb' },
            { value: 1, label: '正常', color: '#86efac' },
            { value: 2, label: '告警', color: '#fdba74' },
            { value: 3, label: '严重', color: '#f87171' },
            { value: 4, label: '严重', color: '#ef4444' }
          ]
        },
        series: [{
          name: panel.title,
          type: 'heatmap',
          data,
          label: { show: false },
          emphasis: {
            itemStyle: {
              shadowBlur: 10,
              shadowColor: 'rgba(0, 0, 0, 0.18)'
            }
          }
        }]
      }
    },
    renderTrendCharts() {
      const activeKeys = new Set(this.trendPanels.filter(panel => panel.hasData).map(panel => panel.key))
      Object.keys(this.chartInstances).forEach(key => {
        if (!activeKeys.has(key)) {
          this.chartInstances[key].dispose()
          delete this.chartInstances[key]
        }
      })
      this.trendPanels.forEach(panel => {
        if (!panel.hasData) return
        const dom = document.getElementById(this.chartDomId(panel))
        if (!dom) return
        if (!this.chartInstances[panel.key]) {
          this.chartInstances[panel.key] = echarts.init(dom)
        }
        this.chartInstances[panel.key].setOption(this.buildTrendChartOption(panel), true)
      })
    },
    resizeTrendCharts() {
      Object.values(this.chartInstances).forEach(chart => chart.resize())
    },
    disposeTrendCharts() {
      Object.values(this.chartInstances).forEach(chart => chart.dispose())
      this.chartInstances = {}
    },
    async loadMonitor() {
      if (!this.hasTarget) return

      this.loading = true
      this.loadError = ''
      try {
        const params = {
          lookbackMinutes: this.queryForm.lookbackMinutes || 5,
          trendHours: this.queryForm.trendHours || 24,
          trendStepMinutes: this.queryForm.trendStepMinutes || 15
        }
        if (this.activeContext.remoteManagementCard) {
          params.remoteManagementCard = this.activeContext.remoteManagementCard
        } else if (this.activeContext.assetId) {
          params.id = this.activeContext.assetId
        }

        const { data: res } = await this.$api.getCmdbIdcAssetHardwareMonitor(params)
        if (res.code !== 200) throw new Error(res.message || '获取硬件监控失败')
        this.monitor = this.normalizeMonitor(res.data || {})
        this.monitorLoaded = true
        this.$nextTick(() => this.renderTrendCharts())
      } catch (error) {
        console.error('loadMonitor failed:', error)
        this.monitor = makeMonitorState()
        this.monitorLoaded = true
        this.disposeTrendCharts()
        this.loadError = error.message || '获取硬件监控失败'
        this.$message.error(this.loadError)
      } finally {
        this.loading = false
      }
    }
  }
}
</script>

<style scoped>
.idc-hardware-panel {
  min-width: 0;
}

.toolbar,
.panel-card {
  border-radius: 24px;
  border: 1px solid #e5e7eb;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: 0 20px 60px rgba(15, 23, 42, 0.08);
}

.toolbar {
  padding: 16px 20px 0;
}

.toolbar-main,
.section-head,
.table-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
}

.toolbar-main {
  flex-wrap: wrap;
}

.toolbar-tags,
.toolbar-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.toolbar-tags {
  color: #6b7280;
  font-size: 13px;
}

.toolbar-form {
  flex: 0 0 auto;
}

.toolbar-divider {
  height: 1px;
  margin: 14px -20px 0;
  background: #e5e7eb;
}

.view-tabs {
  display: flex;
  align-items: center;
  gap: 24px;
  padding-top: 14px;
}

.view-tab {
  padding: 0 0 10px;
  border: none;
  background: transparent;
  color: #6b7280;
  font-size: 15px;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition: color 0.2s ease, border-color 0.2s ease;
}

.view-tab:hover {
  color: #2563eb;
}

.view-tab.active {
  color: #2563eb;
  border-bottom-color: #3b82f6;
  font-weight: 600;
}

.panel-card {
  margin-top: 20px;
  padding: 20px;
}

.panel-body {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.panel-alert {
  margin-bottom: 4px;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 12px;
}

.summary-card,
.health-card,
.meta-card,
.components-card,
.alerts-card,
.trends-card,
.trend-panel {
  border-radius: 18px;
  border: 1px solid #e5e7eb;
  background: #f8fafc;
}

.summary-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 92px;
  padding: 12px 14px;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.65);
}

.summary-card-head {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.summary-card-main {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.summary-card-icon {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border-radius: 12px;
  border: 1px solid rgba(191, 219, 254, 0.82);
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.96), rgba(219, 234, 254, 0.82));
}

.summary-card-icon img {
  width: 18px;
  height: 18px;
  object-fit: contain;
}

.summary-card-health .summary-card-icon,
.summary-card-metric .summary-card-icon {
  border-color: rgba(191, 219, 254, 0.82);
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.98), rgba(219, 234, 254, 0.82));
}

.summary-card-cpu .summary-card-icon,
.summary-card-memory .summary-card-icon {
  border-color: rgba(196, 181, 253, 0.78);
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.98), rgba(237, 233, 254, 0.86));
}

.summary-card-disk .summary-card-icon,
.summary-card-fan .summary-card-icon {
  border-color: rgba(167, 243, 208, 0.82);
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.98), rgba(209, 250, 229, 0.84));
}

.summary-card-power .summary-card-icon,
.summary-card-temperature .summary-card-icon,
.summary-card-alert .summary-card-icon {
  border-color: rgba(253, 230, 138, 0.9);
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.98), rgba(254, 243, 199, 0.88));
}

.summary-label {
  font-size: 12px;
  color: #6b7280;
  line-height: 1.2;
}

.summary-value {
  font-size: 18px;
  font-weight: 700;
  color: #111827;
  line-height: 1.2;
  word-break: break-word;
}

.summary-state-tag {
  align-self: flex-start;
  margin-top: auto;
}

.health-card,
.alerts-card,
.trends-card,
.meta-card,
.components-card {
  padding: 18px;
}

.meta-card,
.health-card {
  padding: 16px;
}

.health-card {
  background: linear-gradient(135deg, rgba(239, 246, 255, 0.96), rgba(255, 255, 255, 0.94));
}

.health-tags {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.health-body {
  display: grid;
  grid-template-columns: minmax(112px, 132px) minmax(0, 1fr);
  gap: 14px;
  margin-top: 12px;
}

.health-score {
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  min-height: 124px;
  padding: 14px 12px;
  border-radius: 16px;
  border: 1px solid rgba(191, 219, 254, 0.9);
  background: radial-gradient(circle at top, rgba(255, 255, 255, 0.96), rgba(219, 234, 254, 0.85));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.7);
}

.health-score strong {
  font-size: 34px;
  line-height: 1;
  font-weight: 800;
  color: #1d4ed8;
}

.health-score span {
  margin-top: 6px;
  color: #64748b;
  font-size: 12px;
  font-weight: 600;
}

.health-content {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}

.health-conclusion {
  margin: 0;
  padding: 10px 12px;
  border-radius: 16px;
  border: 1px solid rgba(226, 232, 240, 0.92);
  background: rgba(255, 255, 255, 0.74);
  color: #334155;
  font-size: 13px;
  line-height: 1.6;
}

.health-stats {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.health-recommendations {
  padding: 10px 12px;
  border-radius: 16px;
  border: 1px solid rgba(226, 232, 240, 0.92);
  background: rgba(255, 255, 255, 0.74);
}

.health-recommendations > span {
  display: inline-flex;
  align-items: center;
  margin-bottom: 6px;
  color: #0f172a;
  font-size: 12px;
  font-weight: 700;
}

.health-recommendations > p {
  margin: 0;
  color: #475569;
  font-size: 12px;
  line-height: 1.6;
}

.health-recommendations > p + p {
  margin-top: 4px;
}

.alerts-card {
  background: linear-gradient(135deg, rgba(255, 251, 235, 0.94), rgba(255, 255, 255, 0.96));
}

.alerts-card.clear {
  background: linear-gradient(135deg, rgba(240, 253, 244, 0.9), rgba(255, 255, 255, 0.96));
}

.alert-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-top: 16px;
  max-height: 420px;
  overflow-y: auto;
  padding-right: 4px;
  scrollbar-gutter: stable;
}

.alert-item {
  display: grid;
  grid-template-columns: minmax(0, 1.45fr) minmax(260px, 1fr);
  gap: 10px 14px;
  padding: 12px 14px;
  border-radius: 14px;
  border: 1px solid rgba(229, 231, 235, 0.95);
  border-left-width: 4px;
  background: rgba(255, 255, 255, 0.78);
}

.alert-item.severity-critical {
  border-color: rgba(239, 107, 115, 0.34);
  background: rgba(254, 242, 242, 0.74);
}

.alert-item.severity-warning {
  border-color: rgba(243, 178, 75, 0.34);
  background: rgba(255, 251, 235, 0.72);
}

.alert-main {
  min-width: 0;
}

.alert-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #1f2937;
  min-width: 0;
}

.alert-title-row strong {
  font-size: 14px;
  line-height: 1.45;
  min-width: 0;
}

.alert-message {
  margin: 6px 0 0;
  color: #4b5563;
  font-size: 13px;
  line-height: 1.5;
}

.alert-context {
  margin: 6px 0 0;
  color: #7b8798;
  font-size: 12px;
  line-height: 1.5;
}

.alert-data {
  display: flex;
  align-items: flex-start;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
}

.alert-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  border-radius: 999px;
  border: 1px solid rgba(226, 232, 240, 0.92);
  background: rgba(248, 250, 252, 0.92);
  color: #64748b;
  font-size: 12px;
  line-height: 1;
}

.alert-chip em {
  font-style: normal;
  color: #94a3b8;
}

.alert-chip strong {
  color: #0f172a;
  font-size: 12px;
}

.alert-extra {
  grid-column: 1 / -1;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px 14px;
  padding-top: 4px;
  border-top: 1px dashed rgba(226, 232, 240, 0.92);
}

.alert-evidence,
.alert-recommendation {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: #6b7280;
  word-break: break-word;
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.alert-recommendation {
  color: #92400e;
}

.alert-list::-webkit-scrollbar {
  width: 6px;
}

.alert-list::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.35);
}

.alert-list::-webkit-scrollbar-track {
  background: transparent;
}

.clear-state {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-top: 16px;
  padding: 18px;
  border-radius: 16px;
  border: 1px solid rgba(187, 247, 208, 0.9);
  background: rgba(255, 255, 255, 0.72);
  color: #166534;
}

.clear-state p {
  margin: 6px 0 0;
  color: #4b7f5b;
  font-size: 13px;
}

.clear-dot {
  width: 14px;
  height: 14px;
  border-radius: 999px;
  background: #22c55e;
  box-shadow: 0 0 0 8px rgba(34, 197, 94, 0.12);
}

.trend-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  margin-top: 16px;
}

.trend-panel {
  min-width: 0;
  padding: 16px;
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.92), rgba(248, 250, 252, 0.96));
}

.trend-panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.trend-panel-head h4 {
  margin: 0;
  color: #1f2937;
  font-size: 16px;
  font-weight: 700;
}

.trend-panel-head p {
  margin: 6px 0 0;
  color: #8a94a6;
  font-size: 12px;
}

.trend-metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 8px;
  margin-top: 14px;
}

.trend-metric {
  min-width: 0;
  padding: 9px 10px;
  border-radius: 12px;
  background: rgba(241, 245, 249, 0.9);
}

.trend-metric span {
  display: block;
  margin-bottom: 4px;
  color: #8a94a6;
  font-size: 11px;
}

.trend-metric strong {
  color: #111827;
  font-size: 14px;
  font-weight: 700;
  word-break: break-word;
}

.trend-metric strong.up {
  color: #dc6803;
}

.trend-metric strong.down {
  color: #2563eb;
}

.trend-metric strong.success {
  color: #16a34a;
}

.trend-metric strong.danger {
  color: #dc2626;
}

.trend-chart {
  width: 100%;
  height: 240px;
  margin-top: 12px;
}

.trend-chart-status {
  height: 168px;
}

.trend-chart-dashboard {
  height: 220px;
}

.trend-empty {
  margin-top: 12px;
}

.status-dashboard-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 8px;
  margin-top: 12px;
}

.status-dashboard-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
  padding: 10px 12px;
  border-radius: 12px;
  background: rgba(241, 245, 249, 0.9);
}

.status-dashboard-name {
  min-width: 0;
  color: #334155;
  font-size: 12px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-dashboard-more {
  grid-column: 1 / -1;
  margin: 0;
  color: #8a94a6;
  font-size: 12px;
  line-height: 1.5;
}

.metric-tags {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 10px;
  color: #8a94a6;
  font-size: 12px;
}

.meta-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  align-items: stretch;
  gap: 16px;
}

:deep(.meta-grid .el-descriptions__body .el-descriptions__cell) {
  padding-top: 8px;
  padding-bottom: 8px;
}

:deep(.meta-grid .el-descriptions__label),
:deep(.meta-grid .el-descriptions__content) {
  font-size: 12px;
}

.section-head h3,
.table-head h3 {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  color: #1f2937;
}

.section-head p,
.table-head p {
  margin: 6px 0 0;
  font-size: 12px;
  color: #8a94a6;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.components-table {
  margin-top: 16px;
}

:deep(.components-table .el-table__cell) {
  vertical-align: top;
}

:deep(.components-table .cell) {
  white-space: normal;
  word-break: break-word;
  line-height: 1.6;
}

.raw-preview {
  margin: 16px 0 0;
  padding: 16px;
  border-radius: 14px;
  background: #0f172a;
  color: #e2e8f0;
  font-size: 12px;
  line-height: 1.6;
  overflow: auto;
}

:deep(.toolbar .el-form-item) {
  margin-bottom: 0;
  margin-right: 0;
}

:deep(.toolbar .el-form-item__label) {
  padding-right: 8px;
  font-size: 13px;
}

:deep(.toolbar .el-button) {
  padding: 7px 12px;
}

:deep(.toolbar .el-input-number) {
  width: 118px;
}

:deep(.el-empty__description p) {
  color: #8a94a6;
}

@media (max-width: 1600px) {
  .trend-metrics {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 1400px) {
  .summary-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

  .trend-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 1200px) {
  .view-tabs {
    gap: 16px;
    overflow-x: auto;
  }

  .meta-grid {
    grid-template-columns: 1fr;
  }

  .alert-item,
  .alert-extra {
    grid-template-columns: 1fr;
  }

  .alert-data {
    justify-content: flex-start;
  }
}

@media (max-width: 900px) {
  .summary-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .health-score {
    min-height: 128px;
  }

  .health-body {
    grid-template-columns: 1fr;
  }

  .trend-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .toolbar {
    padding-left: 16px;
    padding-right: 16px;
  }

  .toolbar-divider {
    margin-left: -16px;
    margin-right: -16px;
  }

  .summary-grid {
    grid-template-columns: 1fr;
  }

  .trend-metrics {
    grid-template-columns: 1fr;
  }

  .panel-card {
    padding: 16px;
  }

  .alert-list {
    max-height: none;
    padding-right: 0;
  }
}
</style>
