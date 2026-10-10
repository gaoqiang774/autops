import alertApi from '@/api/alertmanager'

const ALLOWED_MONITOR_DATASOURCE_TYPES = new Set(['prometheus', 'victoriametrics', 'builtin'])

const normalizeResponse = response => response?.data || response || {}

const normalizeListResult = response => {
  const res = normalizeResponse(response)
  const payload = res?.data ?? res

  if (Array.isArray(payload)) {
    return payload
  }

  if (!payload || typeof payload !== 'object') {
    return []
  }

  const list = payload.list || payload.rows || payload.records || payload.items || payload.options || []
  return Array.isArray(list) ? list : []
}

export const monitorDatasourceId = item => item?.id ?? item?.datasourceId ?? item?.value ?? ''

export const monitorDatasourceType = item => item?.datasourceType || item?.type || ''

export const monitorDatasourceName = item => item?.name || item?.label || monitorDatasourceId(item)

export const isEnabledMonitorDatasource = item => {
  if (item?.enabled === undefined || item?.enabled === null) return true
  return Number(item.enabled) === 1 || item.enabled === true
}

export const isAllowedMonitorDatasource = item => (
  ALLOWED_MONITOR_DATASOURCE_TYPES.has(monitorDatasourceType(item)) && isEnabledMonitorDatasource(item)
)

export const monitorDatasourceTypeLabel = type => {
  const labels = {
    builtin: '内置',
    prometheus: 'Prometheus',
    victoriametrics: 'VictoriaMetrics'
  }
  return labels[type] || type || '-'
}

export const monitorDatasourceLabel = item => {
  const name = monitorDatasourceName(item)
  const type = monitorDatasourceTypeLabel(monitorDatasourceType(item))
  return name ? `${name} / ${type}` : type
}

export const fetchMonitorDatasourceOptions = async () => {
  const response = await alertApi.getDatasources({
    page: 1,
    pageSize: 1000,
    enabled: 1
  })

  return normalizeListResult(response).filter(isAllowedMonitorDatasource)
}
