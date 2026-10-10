export const normalizeResponse = response => response?.data || response || {}
export const unwrap = response => normalizeResponse(response)?.data ?? {}
export const success = response => [0, 200].includes(Number(normalizeResponse(response)?.code))
export const listResult = response => {
  const data = unwrap(response)
  const list = Array.isArray(data) ? data : data.list || data.rows || []
  return { list: Array.isArray(list) ? list : [], total: Number(data.total || list.length || 0) }
}
export const compact = value => Object.fromEntries(Object.entries(value || {}).filter(([, item]) => item !== '' && item !== null && item !== undefined))
export const formatTime = value => !value ? '-' : String(value).replace('T', ' ').replace(/\.\d+Z?$/, '')
export const userLabel = user => user?.nickname || user?.username || `用户 ${user?.id || ''}`
export const ensure = (response, fallback = '操作失败') => {
  if (success(response)) return unwrap(response)
  throw new Error(normalizeResponse(response)?.message || fallback)
}

export const businessLabels = { daily: '日常事务', release: '应用发布', sql_review: 'SQL 审核' }
export const environmentLabels = { prod: '生产', test: '测试', dev: '开发', staging: '预发布' }
export const dailyCategoryLabels = { ops_issue: '运维问题', permission: '权限申请', config_change: '配置变更', resource_request: '资源申请' }
export const priorityLabels = { low: '低', medium: '中', high: '高', urgent: '紧急' }
export const priorityTypes = { low: 'info', medium: '', high: 'warning', urgent: 'danger' }
export const workflowStatusLabels = { running: '审批中', approved: '已通过', rejected: '已驳回', canceled: '已撤销' }
export const dailyStatusLabels = {
  submitted: '待分派', pending_approval: '待审批', approved: '已审批', rejected: '已驳回', assigned: '已分派',
  processing: '处理中', resolved: '待确认', confirmed: '已确认', closed: '已关闭', canceled: '已撤销'
}
export const releaseStatusLabels = { pending_approval: '待审批', approved: '待执行', rejected: '已驳回', running: '执行中', succeeded: '发布成功', failed: '发布失败', canceled: '已撤销' }
export const itemStatusLabels = { pending: '待执行', queued: '排队中', running: '构建中', succeeded: '成功', failed: '失败', canceled: '已中止' }
export const statusType = status => {
  if (['approved', 'succeeded', 'resolved', 'confirmed', 'closed'].includes(status)) return 'success'
  if (['rejected', 'failed', 'urgent'].includes(status)) return 'danger'
  if (['running', 'processing', 'queued', 'pending_approval', 'submitted', 'assigned'].includes(status)) return 'warning'
  return 'info'
}
