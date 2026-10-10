import aliyunIcon from '@/assets/image/aliyun.png'
import tencentcloudIcon from '@/assets/image/tengxun.png'
import baiduIcon from '@/assets/image/baidu.svg'
import huaweiIcon from '@/assets/image/huaweiyun.svg'
import volcengineIcon from '@/assets/image/火山引擎.svg'

export const responseData = response => response?.data?.data ?? null

export const BILLING_PROVIDER_OPTIONS = Object.freeze([
  { keyType: 1, code: 'aliyun', label: '阿里云', icon: aliyunIcon },
  { keyType: 2, code: 'tencentcloud', label: '腾讯云', icon: tencentcloudIcon },
  { keyType: 3, code: 'baidu', label: '百度云', icon: baiduIcon },
  { keyType: 4, code: 'huawei', label: '华为云', icon: huaweiIcon },
  { keyType: 6, code: 'volcengine', label: '火山引擎', icon: volcengineIcon }
])

export const BILLING_KEY_TYPES = Object.freeze(BILLING_PROVIDER_OPTIONS.map(item => item.keyType))

export const responsePage = response => {
  const data = responseData(response) || {}
  return {
    list: Array.isArray(data.list) ? data.list : [],
    total: Number(data.total) || 0,
    page: Number(data.page) || 1,
    pageSize: Number(data.pageSize) || 10
  }
}

export const requestError = (error, fallback) =>
  error?.response?.data?.message || error?.message || fallback

export const formatCurrency = (value, currency = 'CNY') => {
  const amount = Number(value) || 0
  try {
    return new Intl.NumberFormat('zh-CN', {
      style: 'currency',
      currency: currency || 'CNY',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2
    }).format(amount)
  } catch (_) {
    return `${currency || 'CNY'} ${amount.toFixed(2)}`
  }
}

export const formatRate = value => value === null || value === undefined
  ? '暂无基准'
  : `${Number(value) > 0 ? '+' : ''}${Number(value).toFixed(2)}%`

export const rateClass = value => Number(value) > 0
  ? 'is-increase'
  : Number(value) < 0 ? 'is-decrease' : 'is-neutral'

export const providerName = value => ({
  ...Object.fromEntries(BILLING_PROVIDER_OPTIONS.map(item => [item.code, item.label])),
  tencent: '腾讯云',
  huaweicloud: '华为云',
  volc: '火山引擎'
}[value] || value || '未知厂商')

export const providerIcon = value => BILLING_PROVIDER_OPTIONS.find(item =>
  item.code === value || item.keyType === Number(value)
)?.icon || ''

export const syncStatusLabel = value => ({
  success: '同步成功',
  partial_success: '部分成功',
  failed: '同步失败',
  syncing: '同步中',
  pending: '待同步'
}[value] || '未知')

export const syncStatusType = value => ({
  success: 'success',
  partial_success: 'warning',
  failed: 'danger',
  syncing: 'warning',
  pending: 'info'
}[value] || 'info')

export const balanceStatusLabel = value => ({
  normal: '余额正常',
  low: '余额偏低',
  error: '查询失败',
  refreshing: '刷新中',
  disabled: '未监控',
  unknown: '待刷新'
}[value] || '未知')

export const balanceStatusType = value => ({
  normal: 'success',
  low: 'danger',
  error: 'danger',
  refreshing: 'warning',
  disabled: 'info',
  unknown: 'info'
}[value] || 'info')
