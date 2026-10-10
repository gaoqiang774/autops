import request from '@/utils/request'

const base = '/api/v1/ticket'
const get = (url, params) => request({ url: `${base}${url}`, method: 'get', params })
const post = (url, data = {}) => request({ url: `${base}${url}`, method: 'post', data })
const put = (url, data = {}, params) => request({ url: `${base}${url}`, method: 'put', data, params })
const remove = url => request({ url: `${base}${url}`, method: 'delete' })

export default {
  overview: () => get('/overview'),
  myApprovalTasks: params => get('/approval-tasks/my', params),
  approveTask: (id, data) => post(`/approval-tasks/${id}/approve`, data),
  rejectTask: (id, data) => post(`/approval-tasks/${id}/reject`, data),

  workflows: params => get('/workflows', params),
  workflow: id => get(`/workflows/${id}`),
  createWorkflow: data => post('/workflows', data),
  updateWorkflow: (id, data) => put(`/workflows/${id}`, data),
  deleteWorkflow: id => remove(`/workflows/${id}`),
  updateWorkflowStatus: (id, enabled) => put(`/workflows/${id}/status`, {}, { enabled }),
  workflowInstance: id => get(`/workflows/instances/${id}`),

  dailyTickets: params => get('/daily', params),
  dailyTicket: id => get(`/daily/${id}`),
  createDaily: data => post('/daily', data),
  assignDaily: (id, data) => post(`/daily/${id}/assign`, data),
  startDaily: (id, data) => post(`/daily/${id}/start`, data),
  resolveDaily: (id, data) => post(`/daily/${id}/resolve`, data),
  confirmDaily: (id, data) => post(`/daily/${id}/confirm`, data),
  closeDaily: (id, data) => post(`/daily/${id}/close`, data),
  cancelDaily: (id, data) => post(`/daily/${id}/cancel`, data),

  applications: params => get('/release-applications', params),
  createApplication: data => post('/release-applications', data),
  updateApplication: (id, data) => put(`/release-applications/${id}`, data),
  deleteApplication: id => remove(`/release-applications/${id}`),
  releases: params => get('/releases', params),
  release: id => get(`/releases/${id}`),
  createRelease: data => post('/releases', data),
  executeRelease: id => post(`/releases/${id}/execute`),
  retryReleaseItem: (id, itemId) => post(`/releases/${id}/items/${itemId}/retry`),
  cancelReleaseItem: (id, itemId) => post(`/releases/${id}/items/${itemId}/cancel`),
  releaseItemLog: (id, itemId) => get(`/releases/${id}/items/${itemId}/log`)
}
