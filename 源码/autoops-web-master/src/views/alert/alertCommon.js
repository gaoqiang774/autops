export const severityOptions = [
  { label: 'P0 严重', value: 'critical', type: 'danger' },
  { label: 'P1 警告', value: 'warning', type: 'warning' },
  { label: 'P2 提示', value: 'info', type: 'success' }
]

const severityAliasMap = {
  p0: 'critical',
  critical: 'critical',
  serious: 'critical',
  severe: 'critical',
  fatal: 'critical',
  p1: 'warning',
  warning: 'warning',
  warn: 'warning',
  p2: 'info',
  info: 'info',
  informational: 'info',
  notice: 'info'
}

export const statusOptions = [
  { label: '待触发', value: 'pending' },
  { label: '触发中', value: 'firing' },
  { label: '已恢复', value: 'resolved' },
  { label: '已屏蔽', value: 'silenced' },
  { label: '已抑制', value: 'suppressed' }
]

export const datasourceTypeOptions = [
  { label: 'VictoriaMetrics', value: 'victoriametrics' },
  { label: 'Prometheus', value: 'prometheus' },
  { label: 'Zabbix', value: 'zabbix' },
  { label: 'Loki', value: 'loki' }
]

export const enabledOptions = [
  { label: '启用', value: '1' },
  { label: '禁用', value: '0' }
]

export const matcherOpOptions = [
  { label: '=', value: '=' },
  { label: '!=', value: '!=' },
  { label: '正则', value: 'regex' }
]

export const normalizeResponse = response => response?.data || response || {}

export const isSuccess = response => {
  const res = normalizeResponse(response)
  return res.code === 0 || res.code === 200
}

export const normalizeListResult = response => {
  const res = normalizeResponse(response)
  const payload = res?.data ?? res

  if (Array.isArray(payload)) {
    return { list: payload, total: payload.length }
  }

  if (!payload || typeof payload !== 'object') {
    return { list: [], total: 0 }
  }

  const list = payload.list || payload.rows || payload.records || payload.items || payload.options || payload.types || []
  return {
    list: Array.isArray(list) ? list : [],
    total: Number(payload.total ?? payload.count ?? list.length ?? 0)
  }
}

export const unwrapData = response => {
  const res = normalizeResponse(response)
  return res?.data ?? {}
}

export const formatDateTime = value => {
  if (!value) return '-'
  if (typeof value === 'string') return value.replace('T', ' ').replace(/\.\d+Z?$/, '')
  try {
    const date = new Date(value)
    if (Number.isNaN(date.getTime())) return String(value)
    const pad = number => String(number).padStart(2, '0')
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
  } catch (error) {
    return String(value)
  }
}

const notifyRecordTime = row => row?.createdAt || row?.created_at || row?.sentAt || row?.sent_at

const notifyRecordTimestamp = row => {
  const value = notifyRecordTime(row)
  if (!value) return 0
  if (value instanceof Date) return value.getTime()
  if (typeof value === 'number') return Number.isFinite(value) ? value : 0
  const timestamp = Date.parse(String(value).trim().replace(/\//g, '-').replace(' ', 'T'))
  return Number.isNaN(timestamp) ? 0 : timestamp
}

export const sortNotifyRecordsByTimeDesc = rows => {
  if (!Array.isArray(rows)) return []
  return rows
    .map((row, index) => ({ row, index }))
    .sort((left, right) => {
      const timeDiff = notifyRecordTimestamp(right.row) - notifyRecordTimestamp(left.row)
      if (timeDiff !== 0) return timeDiff
      const idDiff = Number(right.row?.id ?? right.row?.ID ?? 0) - Number(left.row?.id ?? left.row?.ID ?? 0)
      return idDiff !== 0 ? idDiff : left.index - right.index
    })
    .map(item => item.row)
}

export const normalizeSeverity = value => {
  const key = String(value || '').trim().toLowerCase()
  return severityAliasMap[key] || value || ''
}

export const severityLabel = value => {
  const target = severityOptions.find(item => item.value === normalizeSeverity(value))
  return target?.label || value || '-'
}

export const severityTagType = value => {
  const target = severityOptions.find(item => item.value === normalizeSeverity(value))
  return target?.type || 'info'
}

export const statusLabel = value => {
  const target = statusOptions.find(item => item.value === value)
  return target?.label || value || '-'
}

export const enabledLabel = value => Number(value) === 1 || value === true ? '启用' : '禁用'

export const stringifyJson = value => {
  if (value === undefined || value === null || value === '') return ''
  if (typeof value === 'string') {
    try {
      return JSON.stringify(JSON.parse(value), null, 2)
    } catch (error) {
      return value
    }
  }
  return JSON.stringify(value, null, 2)
}

export const parseJsonField = (value, fallback = {}) => {
  if (value === undefined || value === null || value === '') return fallback
  if (typeof value === 'object') return value
  try {
    return JSON.parse(value)
  } catch (error) {
    throw new Error('JSON 格式不正确')
  }
}

export const compactObject = object => {
  const result = {}
  Object.keys(object || {}).forEach(key => {
    const value = object[key]
    if (value !== '' && value !== undefined && value !== null) {
      result[key] = value
    }
  })
  return result
}

export const getRowId = row => row?.id ?? row?.eventId ?? row?.event_id ?? row?.ruleId ?? row?.rule_id

export const summarizeObject = value => {
  if (!value) return '-'
  const object = typeof value === 'string' ? parseSafeJson(value) : value
  if (!object || typeof object !== 'object') return String(value)
  const entries = Array.isArray(object) ? object : Object.entries(object).map(([key, val]) => `${key}=${val}`)
  return entries.slice(0, 3).join(' / ') || '-'
}

export const parseSafeJson = value => {
  if (typeof value !== 'string') return value
  try {
    return JSON.parse(value)
  } catch (error) {
    return value
  }
}
