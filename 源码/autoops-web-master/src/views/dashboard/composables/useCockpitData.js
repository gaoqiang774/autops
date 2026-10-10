import { computed, reactive, ref } from 'vue'
import alertApi from '@/api/alertmanager'
import dashboardApi from '@/api/dashboard'
import { normalizeListResult } from '@/views/alert/alertCommon'

const cityCoordinates = [
  { aliases: ['深圳', 'shenzhen'], city: '深圳', lat: 22.5431, lon: 114.0579 },
  { aliases: ['广州', 'guangzhou'], city: '广州', lat: 23.1291, lon: 113.2644 },
  { aliases: ['北京', 'beijing'], city: '北京', lat: 39.9042, lon: 116.4074 },
  { aliases: ['上海', 'shanghai'], city: '上海', lat: 31.2304, lon: 121.4737 },
  { aliases: ['杭州', 'hangzhou'], city: '杭州', lat: 30.2741, lon: 120.1551 },
  { aliases: ['成都', 'chengdu'], city: '成都', lat: 30.5728, lon: 104.0668 },
  { aliases: ['武汉', 'wuhan'], city: '武汉', lat: 30.5928, lon: 114.3055 },
  { aliases: ['南京', 'nanjing'], city: '南京', lat: 32.0603, lon: 118.7969 },
  { aliases: ['重庆', 'chongqing'], city: '重庆', lat: 29.4316, lon: 106.9123 },
  { aliases: ['西安', 'xian', "xi'an"], city: '西安', lat: 34.3416, lon: 108.9398 },
  { aliases: ['长沙', 'changsha'], city: '长沙', lat: 28.2282, lon: 112.9388 },
  { aliases: ['东莞', 'dongguan'], city: '东莞', lat: 23.0207, lon: 113.7518 },
  { aliases: ['佛山', 'foshan'], city: '佛山', lat: 23.0215, lon: 113.1214 },
  { aliases: ['香港', 'hong kong', 'hongkong'], city: '香港', lat: 22.3193, lon: 114.1694 }
]

const pick = (object, keys, fallback = 0) => {
  for (const key of keys) {
    const value = object?.[key]
    if (value !== undefined && value !== null && value !== '') return value
  }
  return fallback
}

const toArray = value => {
  if (Array.isArray(value)) return value
  if (Array.isArray(value?.list)) return value.list
  if (Array.isArray(value?.items)) return value.items
  if (Array.isArray(value?.rows)) return value.rows
  if (Array.isArray(value?.data)) return value.data
  return []
}

const unwrap = response => response?.data?.data ?? response?.data ?? response ?? {}

const unwrapAlert = response => {
  const first = response?.data ?? response ?? {}
  if (first && typeof first === 'object' && 'code' in first && 'data' in first) return first.data ?? {}
  return first
}

const toNumber = value => {
  const number = Number(value)
  return Number.isFinite(number) ? number : 0
}

const normalizeDistribution = (rows, nameKeys) => toArray(rows)
  .map((item, index) => ({
    key: `${pick(item, ['id', 'ID'], index)}-${pick(item, nameKeys, '未知')}`,
    name: String(pick(item, nameKeys, '未知')),
    value: toNumber(pick(item, ['assetCount', 'value', 'count', 'total', 'num', 'physicalCount', 'hostCount'], 0))
  }))
  .filter(item => item.value > 0)
  .sort((left, right) => right.value - left.value)

const extractAlertArray = (source, keys = []) => {
  const payload = unwrapAlert(source)
  if (Array.isArray(payload)) return payload
  for (const key of keys) {
    const rows = toArray(payload?.[key])
    if (rows.length) return rows
  }
  return toArray(payload)
}

const normalizeAlertNameTop = rows => {
  const normalized = rows.map((item, index) => ({
    key: `${pick(item, ['ruleId', 'ruleID', 'id', 'ID'], index)}-${pick(item, ['name', 'ruleName', 'alertName'], '未命名告警')}`,
    name: String(pick(item, ['name', 'ruleName', 'alertName', 'RuleName', 'AlertName'], '未命名告警')),
    count: toNumber(pick(item, ['count', 'total', 'eventCount', 'value', 'Count'], 0)),
    percent: pick(item, ['percent', 'ratio', 'Percent'], null)
  })).sort((left, right) => right.count - left.count).slice(0, 5)
  const total = normalized.reduce((sum, item) => sum + item.count, 0)
  return normalized.map(item => ({
    ...item,
    percent: item.percent === null ? (total ? item.count / total * 100 : 0) : toNumber(item.percent)
  }))
}

const normalizeTrend = rows => rows.map((item, index) => ({
  key: `${pick(item, ['date', 'day', 'time', 'createdAt'], index)}`,
  date: String(pick(item, ['date', 'day', 'time', 'createdAt'], '-')),
  notifications: toNumber(pick(item, ['notificationCount', 'notifyCount', 'count'], 0)),
  alerts: toNumber(pick(item, ['eventCount', 'alertCount'], 0)),
  failures: toNumber(pick(item, ['failedCount', 'failureCount'], 0))
}))

const normalizeAlerts = rows => rows.slice(0, 6).map((item, index) => ({
  key: `${pick(item, ['id', 'eventId', 'fingerprint'], index)}-${pick(item, ['ruleName', 'name', 'alertName'], '未命名告警')}`,
  name: String(pick(item, ['ruleName', 'name', 'alertName', 'RuleName', 'AlertName'], '未命名告警')),
  severity: String(pick(item, ['severity', 'level', 'priority'], 'info')).toLowerCase(),
  source: String(pick(item, ['datasourceType', 'source', 'datasourceName'], '-')),
  instance: String(pick(item, ['businessId', 'instance', 'host', 'resource'], '-')),
  time: pick(item, ['lastEvaluatedAt', 'updatedAt', 'startsAt', 'createdAt', 'time'], '')
}))

export function useCockpitData() {
  const cardsData = ref({})
  const chartsData = ref({})
  const tablesData = ref({})
  const alertScreen = ref({})
  const activeAlerts = ref([])
  const alertNameTop = ref([])
  const notifyTrend = ref([])
  const loading = reactive({ cards: false, charts: false, tables: false, alerts: false })
  const errors = reactive({ cards: '', charts: '', tables: '', alerts: '' })
  const lastUpdatedAt = ref(null)

  const cards = computed(() => {
    const data = cardsData.value || {}
    const asset = data.asset || {}
    const location = data.location || {}
    const monitor = data.monitor || {}
    const warranty = data.warranty || {}
    const cloud = data.cloudHost || {}
    const auth = data.auth || {}
    const physical = toNumber(pick(asset, ['physicalTotal', 'physicalHosts', 'physicalHostCount', 'physicalMachine', 'physical', 'bareMetalCount']))
    const virtual = toNumber(pick(asset, ['vmTotal', 'virtualHosts', 'virtualHostCount', 'virtualMachine', 'virtual', 'vmCount']))
    const switches = toNumber(pick(asset, ['switchTotal', 'switches', 'switchCount', 'networkDevices', 'networkDeviceCount']))
    const aliyun = toNumber(pick(cloud, ['aliyunTotal', 'aliyun', 'aliyunCount', 'aliCloudCount']))
    const tencent = toNumber(pick(cloud, ['tencentTotal', 'tencent', 'tencentCount', 'tencentCloudCount']))
    return {
      physical,
      virtual,
      switches,
      total: toNumber(pick(asset, ['total'], physical + virtual + switches)),
      idcs: toNumber(pick(location, ['roomTotal', 'idcs', 'idcCount', 'rooms', 'roomCount'])),
      cabinets: toNumber(pick(location, ['cabinetTotal', 'cabinets', 'cabinetCount', 'racks', 'rackCount'])),
      hardwareCoverage: toNumber(pick(monitor, ['hardwareCoverageRate', 'hardwareMonitorCoverage', 'hardwareCoverage'])),
      systemCoverage: toNumber(pick(monitor, ['systemCoverageRate', 'systemMonitorCoverage', 'systemCoverage'])),
      expiring: toNumber(pick(warranty, ['expiringSoonCount', 'dueCount', 'warrantyExpiring', 'warrantyExpiringCount'])),
      expired: toNumber(pick(warranty, ['expiredCount', 'warrantyExpired', 'warrantyExpiredCount'])),
      cloudTotal: toNumber(pick(cloud, ['total'], aliyun + tencent)),
      authRate: toNumber(pick(auth, ['authRate', 'authenticationRate', 'credentialRate']))
    }
  })

  const distributions = computed(() => ({
    brands: normalizeDistribution(pick(chartsData.value, ['brandDistribution', 'brands', 'brand'], []), ['name', 'label', 'brand', 'vendor']),
    idcs: normalizeDistribution(pick(chartsData.value, ['roomPhysicalAssets', 'idcPhysicalHosts', 'idcPhysicalHostCounts', 'idcs'], []), ['roomName', 'name', 'label', 'idc', 'idcName']),
    usages: normalizeDistribution(pick(chartsData.value, ['purposeDistribution', 'usageDistribution', 'usages', 'purposes'], []), ['name', 'label', 'usage', 'purpose', 'type'])
  }))

  const metrics = computed(() => {
    const data = tablesData.value || {}
    const highest = data.highestUsage || {}
    const groups = [
      { key: 'cpu', label: 'CPU', rows: toArray(pick(highest, ['cpu'], pick(data, ['cpuHighestTop', 'cpuHighTop', 'cpuTop'], []))) },
      { key: 'memory', label: '内存', rows: toArray(pick(highest, ['memory'], pick(data, ['memoryHighestTop', 'memoryHighTop', 'memoryTop'], []))) },
      { key: 'disk', label: '磁盘', rows: toArray(pick(highest, ['disk'], pick(data, ['diskHighestTop', 'diskHighTop', 'diskTop'], []))) }
    ]
    return groups.map(group => ({
      ...group,
      rows: group.rows.slice(0, 5).map((row, index) => ({
        key: `${group.key}-${pick(row, ['hostId', 'id'], index)}`,
        name: String(pick(row, ['hostName', 'hostname', 'name', 'instanceName', 'ip'], '-')),
        ip: String(pick(row, ['ip', 'privateIp', 'sshIp', 'publicIp'], '-')),
        value: toNumber(pick(row, ['value', 'usage', 'usageRate', 'percent', 'rate'], 0))
      }))
    }))
  })

  const globeMarkers = computed(() => distributions.value.idcs.reduce((markers, item) => {
    const name = item.name.toLowerCase()
    const match = cityCoordinates.find(city => city.aliases.some(alias => name.includes(alias)))
    if (!match) return markers
    const existing = markers.find(marker => marker.city === match.city)
    if (existing) existing.value += item.value
    else markers.push({ ...match, name: item.name, value: item.value })
    return markers
  }, []))

  const activeTotal = computed(() => toNumber(pick(alertScreen.value, ['totalActive', 'activeTotal', 'activeCount'], activeAlerts.value.length)))
  const notificationTotal = computed(() => notifyTrend.value.reduce((sum, item) => sum + item.notifications, 0))
  const hasData = computed(() => cards.value.total > 0 || distributions.value.idcs.length > 0 || activeAlerts.value.length > 0)
  const isLoading = computed(() => Object.values(loading).some(Boolean))
  const errorMessages = computed(() => Object.values(errors).filter(Boolean))

  const runLoad = async (key, loader) => {
    loading[key] = true
    errors[key] = ''
    try {
      await loader()
    } catch (error) {
      console.error(`驾驶舱 ${key} 数据加载失败:`, error)
      errors[key] = error?.message || '接口请求失败'
    } finally {
      loading[key] = false
    }
  }

  const loadCards = () => runLoad('cards', async () => {
    cardsData.value = unwrap(await dashboardApi.getCmdbDashboardCards({ warrantyDays: 90 }))
  })

  const loadCharts = () => runLoad('charts', async () => {
    chartsData.value = unwrap(await dashboardApi.getCmdbDashboardCharts())
  })

  const loadTables = () => runLoad('tables', async () => {
    tablesData.value = unwrap(await dashboardApi.getCmdbDashboardTables({ limit: 5, lookbackMinutes: 5 }))
  })

  const loadAlerts = () => runLoad('alerts', async () => {
    const [chartsResponse, screenResponse, activeResponse] = await Promise.all([
      alertApi.getDashboardCharts({ range: 'today', limit: 5 }),
      alertApi.getDashboardScreen(),
      alertApi.getActiveEvents({ page: 1, pageSize: 10, eventStatus: 'active' })
    ])
    const chartsPayload = unwrapAlert(chartsResponse)
    const screenPayload = unwrapAlert(screenResponse)
    const activeResult = normalizeListResult(activeResponse)
    alertScreen.value = screenPayload
    activeAlerts.value = normalizeAlerts(
      Array.isArray(screenPayload.latestActiveTop5) && screenPayload.latestActiveTop5.length
        ? screenPayload.latestActiveTop5
        : activeResult.list
    )
    alertNameTop.value = normalizeAlertNameTop(extractAlertArray(chartsPayload, ['alertNameTop5', 'alertNameTop', 'topAlertNames']))
    notifyTrend.value = normalizeTrend(extractAlertArray(chartsPayload, ['notifyTrend', 'notifyTrends', 'notificationTrend']))
  })

  const refresh = async () => {
    await Promise.all([loadCards(), loadCharts(), loadTables(), loadAlerts()])
    lastUpdatedAt.value = new Date()
  }

  return {
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
    loading,
    metrics,
    notificationTotal,
    notifyTrend,
    refresh
  }
}
