import request from '@/utils/request'

const crud = resource => ({
  list(params = {}) {
    return request({
      url: `logtail/${resource}`,
      method: 'get',
      params
    })
  },
  detail(id) {
    return request({
      url: `logtail/${resource}/${id}`,
      method: 'get'
    })
  },
  create(data) {
    return request({
      url: `logtail/${resource}`,
      method: 'post',
      data
    })
  },
  update(id, data) {
    return request({
      url: `logtail/${resource}/${id}`,
      method: 'put',
      data
    })
  },
  remove(id) {
    return request({
      url: `logtail/${resource}/${id}`,
      method: 'delete'
    })
  }
})

const projects = crud('projects')
const stores = crud('stores')
const parseTemplates = crud('parse-templates')
const collectTasks = crud('collect-tasks')
const alertRules = crud('alert-rules')
const alertEvents = crud('alert-events')

export default {
  getDashboardOverview(params = {}, config = {}) {
    return request({
      ...config,
      url: 'logtail/dashboard/overview',
      method: 'get',
      params,
      timeout: 30000
    })
  },

  getDashboardMetrics(params = {}, config = {}) {
    return request({
      ...config,
      url: 'logtail/dashboard/metrics',
      method: 'get',
      params,
      timeout: 30000
    })
  },

  getNginxDashboardOptions(params = {}, config = {}) {
    return request({
      ...config,
      url: 'logtail/nginx-dashboard/options',
      method: 'get',
      params
    })
  },

  getNginxDashboardOverview(params = {}, config = {}) {
    return request({
      ...config,
      url: 'logtail/nginx-dashboard/overview',
      method: 'get',
      params,
      timeout: 30000
    })
  },

  getNginxDashboardVisitors(params = {}, config = {}) {
    return request({
      ...config,
      url: 'logtail/nginx-dashboard/visitors',
      method: 'get',
      params,
      timeout: 30000
    })
  },

  getNginxDashboardBindings(params = {}) {
    return request({
      url: 'logtail/nginx-dashboard/bindings',
      method: 'get',
      params
    })
  },

  getNginxDashboardBinding(id) {
    return request({
      url: `logtail/nginx-dashboard/bindings/${id}`,
      method: 'get'
    })
  },

  createNginxDashboardBinding(data) {
    return request({
      url: 'logtail/nginx-dashboard/bindings',
      method: 'post',
      data
    })
  },

  updateNginxDashboardBinding(id, data) {
    return request({
      url: `logtail/nginx-dashboard/bindings/${id}`,
      method: 'put',
      data
    })
  },

  deleteNginxDashboardBinding(id) {
    return request({
      url: `logtail/nginx-dashboard/bindings/${id}`,
      method: 'delete'
    })
  },

  getVictoriaLogsHealth() {
    return request({
      url: 'logtail/victorialogs/health',
      method: 'get'
    })
  },

  getProjects: projects.list,
  getProject: projects.detail,
  createProject: projects.create,
  updateProject: projects.update,
  deleteProject: projects.remove,

  getStores: stores.list,
  getStore: stores.detail,
  createStore: stores.create,
  updateStore: stores.update,
  deleteStore: stores.remove,

  getParseTemplates: parseTemplates.list,
  getParseTemplate: parseTemplates.detail,
  createParseTemplate: parseTemplates.create,
  updateParseTemplate: parseTemplates.update,
  deleteParseTemplate: parseTemplates.remove,
  generateParseTemplateByAI(data) {
    return request({
      url: 'logtail/parse-templates/ai-generate',
      method: 'post',
      data,
      timeout: 120000
    })
  },

  getCollectTasks: collectTasks.list,
  getCollectTask: collectTasks.detail,
  createCollectTask: collectTasks.create,
  updateCollectTask: collectTasks.update,
  deleteCollectTask: collectTasks.remove,

  getCollectTaskAgents(taskId, params = {}) {
    return request({
      url: `logtail/collect-tasks/${taskId}/agents`,
      method: 'get',
      params
    })
  },

  applyCollectTask(taskId, data = {}) {
    return request({
      url: `logtail/collect-tasks/${taskId}/apply`,
      method: 'post',
      data
    })
  },

  stopCollectTask(taskId, data = {}) {
    return request({
      url: `logtail/collect-tasks/${taskId}/stop`,
      method: 'post',
      data
    })
  },

  uninstallKubernetesCollector(taskId, data = {}) {
    return request({
      url: `logtail/k8s-collectors/${taskId}/uninstall`,
      method: 'post',
      data
    })
  },

  getAlertRules: alertRules.list,
  getAlertRule: alertRules.detail,
  createAlertRule: alertRules.create,
  updateAlertRule: alertRules.update,
  deleteAlertRule: alertRules.remove,

  getAlertEvents: alertEvents.list,
  getAlertEvent: alertEvents.detail,
  createAlertEvent: alertEvents.create,
  updateAlertEvent: alertEvents.update,
  deleteAlertEvent: alertEvents.remove,

  queryLogs(data) {
    return request({
      url: 'logtail/query/logs',
      method: 'post',
      data,
      timeout: 30000
    })
  },

  queryTraces(data) {
    return request({
      url: 'logtail/query/traces',
      method: 'post',
      data,
      timeout: 30000
    })
  },

  queryTrace(data) {
    return request({
      url: 'logtail/query/trace',
      method: 'post',
      data,
      timeout: 30000
    })
  },

  downloadLogs(data) {
    return request({
      url: 'logtail/query/logs/download',
      method: 'post',
      data,
      responseType: 'blob',
      timeout: 120000
    })
  },

  queryHits(data) {
    return request({
      url: 'logtail/query/hits',
      method: 'post',
      data,
      timeout: 30000
    })
  },

  queryFields(data) {
    return request({
      url: 'logtail/query/fields',
      method: 'post',
      data,
      timeout: 30000
    })
  },

  queryFieldValues(data) {
    return request({
      url: 'logtail/query/field-values',
      method: 'post',
      data,
      timeout: 30000
    })
  },

  queryStatsRange(data) {
    return request({
      url: 'logtail/query/stats-range',
      method: 'post',
      data,
      timeout: 30000
    })
  },

  getMySQLSlowLogInstances() {
    return request({
      url: 'logtail/query/mysql-slow/instances',
      method: 'get'
    })
  },

  getMySQLSlowCollectedFiles(params) {
    return request({
      url: 'logtail/query/mysql-slow/instances/collected-files',
      method: 'get',
      params,
      timeout: 30000
    })
  },

  getMySQLSlowDailyReport(params) {
    return request({
      url: 'logtail/query/mysql-slow/daily-report',
      method: 'get',
      params,
      timeout: 30000
    })
  },

  generateMySQLSlowDailyReport(data) {
    return request({
      url: 'logtail/query/mysql-slow/daily-report',
      method: 'post',
      data,
      timeout: 300000
    })
  },

  cancelMySQLSlowDailyReport(reportId) {
    return request({
      url: `logtail/query/mysql-slow/daily-report/${reportId}/cancel`,
      method: 'post',
      timeout: 30000
    })
  },

  exportMySQLSlowDailyReport(params) {
    return request({
      url: 'logtail/query/mysql-slow/daily-report/export',
      method: 'get',
      params,
      responseType: 'blob',
      timeout: 120000
    })
  },

  getLogtailAgents(params = {}) {
    return request({
      url: 'logtail/agents',
      method: 'get',
      params
    })
  },

  getVectorAgents(params = {}) {
    return request({
      url: 'logtail/vector/agents',
      method: 'get',
      params
    })
  },

  getVectorPackage() {
    return request({
      url: 'logtail/vector/package',
      method: 'get'
    })
  },

  getAggregatorConfigPreview() {
    return request({
      url: 'logtail/vector/aggregator/config-preview',
      method: 'get'
    })
  },

  uploadVectorPackage(data, params = {}) {
    return request({
      url: 'logtail/vector/package',
      method: 'post',
      data,
      params,
      timeout: 600000,
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    })
  },

  getVectorPackageUploadTask(taskId) {
    return request({
      url: `logtail/vector/package/upload/task/${taskId}`,
      method: 'get'
    })
  },

  deleteVectorPackage() {
    return request({
      url: 'logtail/vector/package',
      method: 'delete'
    })
  },

  createAgentOperation(agentId, data) {
    return request({
      url: `logtail/agents/${agentId}/operations`,
      method: 'post',
      data
    })
  },

  getAgentOperations(agentId, params = {}) {
    return request({
      url: `logtail/agents/${agentId}/operations`,
      method: 'get',
      params
    })
  },

  getAgentConfigPreview(agentId) {
    return request({
      url: `logtail/agents/${agentId}/config-preview`,
      method: 'get'
    })
  },

  getAgentCollectTasks(agentId, params = {}) {
    return request({
      url: `logtail/agents/${agentId}/collect-tasks`,
      method: 'get',
      params
    })
  },

  getK8sCollectors(params = {}) {
    return request({
      url: 'logtail/k8s-collectors',
      method: 'get',
      params
    })
  },

  getK8sCollectorStatus(taskId, params = {}) {
    return request({
      url: `logtail/k8s-collectors/${taskId}/status`,
      method: 'get',
      params
    })
  },

  installK8sCollector(taskId, data = {}) {
    return request({
      url: `logtail/k8s-collectors/${taskId}/install`,
      method: 'post',
      data
    })
  },

  updateK8sCollector(taskId, data = {}) {
    return request({
      url: `logtail/k8s-collectors/${taskId}/update`,
      method: 'post',
      data
    })
  },

  uninstallK8sCollector(taskId, data = {}) {
    return request({
      url: `logtail/k8s-collectors/${taskId}/uninstall`,
      method: 'post',
      data
    })
  },

  reinstallK8sCollector(taskId, data = {}) {
    return request({
      url: `logtail/k8s-collectors/${taskId}/reinstall`,
      method: 'post',
      data
    })
  },

  getK8sCollectorOperations(taskId, params = {}) {
    return request({
      url: `logtail/k8s-collectors/${taskId}/operations`,
      method: 'get',
      params
    })
  },

  getK8sClusterCollectors(params = {}) {
    return request({
      url: 'logtail/k8s-cluster-collectors',
      method: 'get',
      params
    })
  },

  getK8sClusterCollectorStatus(clusterId, params = {}) {
    return request({
      url: `logtail/k8s-cluster-collectors/${clusterId}/status`,
      method: 'get',
      params
    })
  },

  installK8sClusterCollector(clusterId, data = {}) {
    return request({
      url: `logtail/k8s-cluster-collectors/${clusterId}/install`,
      method: 'post',
      data
    })
  },

  updateK8sClusterCollector(clusterId, data = {}) {
    return request({
      url: `logtail/k8s-cluster-collectors/${clusterId}/update`,
      method: 'post',
      data
    })
  },

  uninstallK8sClusterCollector(clusterId, data = {}) {
    return request({
      url: `logtail/k8s-cluster-collectors/${clusterId}/uninstall`,
      method: 'post',
      data
    })
  },

  reinstallK8sClusterCollector(clusterId, data = {}) {
    return request({
      url: `logtail/k8s-cluster-collectors/${clusterId}/reinstall`,
      method: 'post',
      data
    })
  },

  getK8sClusterCollectorOperations(clusterId, params = {}) {
    return request({
      url: `logtail/k8s-cluster-collectors/${clusterId}/operations`,
      method: 'get',
      params
    })
  }
}
