import request from '@/utils/request'
import storage from '@/utils/storage'
import { withAppBasePath } from '@/utils/url'

const API_PREFIX = '/api/v1'

function businessError(message, body) {
  const error = new Error(message || '请求失败')
  error.responseBody = body
  return error
}

async function unified(config) {
  const response = await request(config)
  const body = response?.data
  if (!body || typeof body !== 'object' || typeof body.code !== 'number') {
    throw businessError('接口响应格式不正确', body)
  }
  if (body.code !== 200) {
    throw businessError(body.message || body.error || '请求失败', body)
  }
  return body.data
}

async function direct(config) {
  const response = await request(config)
  const body = response?.data
  if (body?.error) throw businessError(body.error, body)
  if (body && typeof body.code === 'number' && body.code !== 200) {
    throw businessError(body.message || '请求失败', body)
  }
  return body
}

function normalizeAnsiblePage(payload) {
  let body = payload
  if (body && typeof body.code === 'number') {
    if (body.code !== 200) throw businessError(body.message || '请求失败', body)
    body = body.data
  }
  if (body?.data && !Array.isArray(body.data) && (body.data.data || body.data.list)) {
    body = body.data
  }
  const list = Array.isArray(body?.data) ? body.data : (Array.isArray(body?.list) ? body.list : [])
  return { list, total: Number(body?.total || list.length) }
}

const taskApi = {
  // Template library
  getTemplates: () => unified({ url: `${API_PREFIX}/template/list`, method: 'get' }),
  getTemplate: id => unified({ url: `${API_PREFIX}/template/info/${id}`, method: 'get' }),
  getTemplateContent: async id => {
    const response = await request({
      url: `${API_PREFIX}/template/content/${id}`,
      method: 'get',
      responseType: 'text'
    })
    return response?.data ?? ''
  },
  queryTemplatesByName: name => unified({
    url: `${API_PREFIX}/template/query/name`, method: 'get', params: { name }
  }),
  queryTemplatesByType: type => unified({
    url: `${API_PREFIX}/template/query/type`, method: 'get', params: { type }
  }),
  createTemplate: data => unified({ url: `${API_PREFIX}/template/add`, method: 'post', data }),
  updateTemplate: (id, data) => unified({
    url: `${API_PREFIX}/template/update`, method: 'put', params: { id }, data
  }),
  deleteTemplate: id => unified({
    url: `${API_PREFIX}/template/delete`, method: 'delete', params: { id }
  }),

  // Immediate and scheduled tasks
  getTasks: params => unified({
    url: `${API_PREFIX}/task/list-with-details`, method: 'get', params
  }),
  getTask: id => unified({ url: `${API_PREFIX}/task/get`, method: 'get', params: { id } }),
  queryTasksByType: type => unified({
    url: `${API_PREFIX}/task/query/type`, method: 'get', params: { type }
  }),
  queryTasksByStatus: status => unified({
    url: `${API_PREFIX}/task/query/status`, method: 'get', params: { status }
  }),
  createTask: data => unified({ url: `${API_PREFIX}/task/add`, method: 'post', data }),
  updateTask: data => unified({ url: `${API_PREFIX}/task/update`, method: 'put', data }),
  deleteTask: id => unified({
    url: `${API_PREFIX}/task/delete`, method: 'delete', data: { id }
  }),
  getNextExecution: cron => unified({
    url: `${API_PREFIX}/task/next-execution`, method: 'get', params: { cron }
  }),
  getExecutionInfo: id => unified({
    url: `${API_PREFIX}/task/execution-info`, method: 'get', params: { id }
  }),
  getTaskTemplates: id => unified({
    url: `${API_PREFIX}/task/templates`, method: 'get', params: { id }
  }),
  startTask: taskId => unified({
    url: `${API_PREFIX}/taskjob/start`, method: 'post', params: { taskId }
  }),
  getTaskLog: (taskId, templateId) => unified({
    url: `${API_PREFIX}/taskjob/log`, method: 'get', params: { taskId, templateId }
  }),
  stopTask: (taskId, templateId) => unified({
    url: `${API_PREFIX}/taskjob/stop`, method: 'post', params: { taskId, templateId }
  }),
  getTaskWorkStatus: (taskId, templateId) => unified({
    url: `${API_PREFIX}/taskjob/status`, method: 'get', params: { taskId, templateId }
  }),

  // Scheduler and runtime monitoring
  getTaskOperationStatus: taskId => unified({
    url: `${API_PREFIX}/task/monitor/task/status`, method: 'get', params: { task_id: taskId }
  }),
  pauseScheduledTask: taskId => unified({
    url: `${API_PREFIX}/task/monitor/scheduled/pause`, method: 'post', params: { task_id: taskId }
  }),
  resumeScheduledTask: taskId => unified({
    url: `${API_PREFIX}/task/monitor/scheduled/resume`, method: 'post', params: { task_id: taskId }
  }),
  resetScheduledTask: taskId => unified({
    url: `${API_PREFIX}/task/monitor/scheduled/reset`, method: 'post', params: { task_id: taskId }
  }),
  getQueueMetrics: () => unified({ url: `${API_PREFIX}/task/monitor/queue/metrics`, method: 'get' }),
  getSchedulerStats: () => unified({ url: `${API_PREFIX}/task/monitor/scheduler/stats`, method: 'get' }),
  getSystemStatus: () => unified({ url: `${API_PREFIX}/task/monitor/system/status`, method: 'get' }),
  getQueueDetails: () => unified({ url: `${API_PREFIX}/task/monitor/queue/details`, method: 'get' }),
  clearFailedQueue: () => unified({
    url: `${API_PREFIX}/task/monitor/queue/clear-failed`, method: 'post'
  }),
  retryFailedQueue: (limit = 10) => unified({
    url: `${API_PREFIX}/task/monitor/queue/retry-failed`, method: 'post', params: { limit }
  }),

  // Ansible and K8s automation
  getAnsibleTasks: async params => normalizeAnsiblePage(await direct({
    url: `${API_PREFIX}/task/ansiblelist`, method: 'get', params
  })),
  queryAnsibleByName: async name => normalizeAnsiblePage(await direct({
    url: `${API_PREFIX}/task/ansible/query/name`, method: 'get', params: { name }
  })),
  queryAnsibleByType: async type => normalizeAnsiblePage(await direct({
    url: `${API_PREFIX}/task/ansible/query/type`, method: 'get', params: { type }
  })),
  getAnsibleTask: id => unified({ url: `${API_PREFIX}/task/ansible/${id}`, method: 'get' }),
  createAnsibleTask: data => direct({
    url: `${API_PREFIX}/task/ansible`, method: 'post', data, timeout: 300000
  }),
  createK8sTask: data => unified({
    url: `${API_PREFIX}/task/k8s`, method: 'post', data, timeout: 300000
  }),
  startAnsibleTask: id => direct({ url: `${API_PREFIX}/task/ansible/${id}/start`, method: 'post' }),
  stopAnsibleWork: (id, workId) => direct({
    url: `${API_PREFIX}/task/ansible/${id}/stop/${workId}`, method: 'post'
  }),
  deleteAnsibleTask: id => direct({ url: `${API_PREFIX}/task/ansible/${id}`, method: 'delete' }),
  openAnsibleLog(id, workId, handlers = {}) {
    const token = encodeURIComponent(storage.getToken())
    const source = new EventSource(withAppBasePath(
      `${API_PREFIX}/task/ansible/${id}/log/${workId}?token=${token}`
    ))
    source.onmessage = event => handlers.onLog?.(event.data)
    source.addEventListener('complete', event => {
      handlers.onComplete?.(event.data)
      source.close()
    })
    source.addEventListener('error', event => handlers.onError?.(event))
    return () => source.close()
  }
}

export default taskApi
