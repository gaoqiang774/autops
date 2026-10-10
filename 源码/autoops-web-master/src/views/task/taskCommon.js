export const TEMPLATE_TYPES = {
  1: { label: 'Shell', type: 'primary' },
  2: { label: 'Python', type: 'success' },
  3: { label: 'Ansible', type: 'warning' }
}

export const TASK_TYPES = {
  1: { label: '立即执行', type: 'primary' },
  2: { label: '定时任务', type: 'warning' }
}

export const ANSIBLE_TYPES = {
  1: { label: '手动上传', type: 'primary' },
  2: { label: 'Git 导入', type: 'success' },
  3: { label: 'K8s 部署', type: 'warning' }
}

export const TASK_STATUSES = {
  1: { label: '等待中', type: 'info', icon: 'Clock' },
  2: { label: '运行中', type: 'primary', icon: 'Loading' },
  3: { label: '成功', type: 'success', icon: 'CircleCheck' },
  4: { label: '异常', type: 'danger', icon: 'CircleClose' },
  5: { label: '已暂停', type: 'warning', icon: 'VideoPause' },
  6: { label: '已停止', type: 'info', icon: 'SwitchButton' }
}

export function optionOf(map, value) {
  return map[Number(value)] || { label: '未知', type: 'info', icon: 'QuestionFilled' }
}

export function formatDate(value) {
  if (!value) return '--'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false
  }).format(date).replaceAll('/', '-')
}

export function formatDuration(value) {
  const seconds = Number(value || 0)
  if (!seconds) return '--'
  if (seconds < 60) return `${seconds.toFixed(seconds < 10 ? 1 : 0)} 秒`
  const minutes = Math.floor(seconds / 60)
  const rest = Math.round(seconds % 60)
  if (minutes < 60) return `${minutes} 分 ${rest} 秒`
  return `${Math.floor(minutes / 60)} 时 ${minutes % 60} 分`
}

export function safeJson(value, fallback) {
  if (value === null || value === undefined || value === '') return fallback
  if (typeof value === 'object') return value
  try {
    return JSON.parse(value)
  } catch (_) {
    return fallback
  }
}

export function hostLabel(host) {
  const name = host?.hostName || host?.name || `主机 ${host?.id}`
  const ip = host?.sshIp || host?.privateIp || host?.ip || host?.publicIp
  return ip ? `${name} (${ip})` : name
}

export function errorMessage(error, fallback = '操作失败') {
  return error?.response?.data?.message || error?.response?.data?.error || error?.message || fallback
}

