import request from '@/utils/request'

const baseUrl = '/api/v1/alertmanager'

const listRequest = (url, params) => request({ url: `${baseUrl}${url}`, method: 'get', params })
const detailRequest = (url, id) => request({ url: `${baseUrl}${url}/${id}`, method: 'get' })
const createRequest = (url, data) => request({ url: `${baseUrl}${url}`, method: 'post', data })
const updateRequest = (url, id, data) => request({ url: `${baseUrl}${url}/${id}`, method: 'put', data })
const deleteRequest = (url, id) => request({ url: `${baseUrl}${url}/${id}`, method: 'delete' })

export default {
  getAdminUsers(params) {
    return request({ url: 'admin/list', method: 'get', params })
  },

  getDashboardCharts(params) {
    return listRequest('/dashboard/charts', params)
  },
  getDashboardAlertNameTop(params) {
    return listRequest('/dashboard/top-alert-names', params)
  },
  getDashboardNotifyTrend(params) {
    return listRequest('/dashboard/notify-trend', params)
  },
  getDashboardHostGroupTop(params) {
    return listRequest('/dashboard/top-host-groups', params)
  },
  getDashboardBusinessRouteTop(params) {
    return listRequest('/dashboard/top-business-routes', params)
  },
  getDashboardScreen(params) {
    return listRequest('/dashboard/screen', params)
  },

  getActiveEvents(params) {
    return listRequest('/active-events', params)
  },
  getActiveEventBusinessOptions() {
    return listRequest('/active-events/business-options')
  },
  claimActiveEvent(eventId) {
    return createRequest(`/active-events/${eventId}/claim`)
  },
  silenceActiveEvent(eventId, data) {
    return createRequest(`/active-events/${eventId}/silence`, data)
  },
  unsilenceActiveEvent(eventId) {
    return request({ url: `${baseUrl}/active-events/${eventId}/silence`, method: 'delete' })
  },
  getActiveEventTimeline(eventId) {
    return listRequest(`/active-events/${eventId}/timeline`)
  },
  getHistoryEvents(params) {
    return listRequest('/history-events', params)
  },
  getEventDetail(eventType, eventId) {
    return listRequest(`/events/${eventType}/${eventId}`)
  },
  getNotifyRecords(params) {
    return listRequest('/notify-records', params)
  },
  getNotifyRecordStats(params) {
    return listRequest('/notify-records/stats', params)
  },

  getRules(params) {
    return listRequest('/rules', params)
  },
  getRule(id) {
    return detailRequest('/rules', id)
  },
  createRule(data) {
    return createRequest('/rules', data)
  },
  previewRuleQuery(data) {
    return createRequest('/rules/preview-query', data)
  },
  importPrometheusRules(data) {
    return createRequest('/rules/import-prometheus', data)
  },
  batchUpdateRuleDatasource(data) {
    return createRequest('/rules/batch-datasource', data)
  },
  batchUpdateRuleNotifyRules(data) {
    return createRequest('/rules/batch-notify-rules', data)
  },
  updateRule(id, data) {
    return updateRequest('/rules', id, data)
  },
  deleteRule(id) {
    return deleteRequest('/rules', id)
  },
  updateRuleStatus(id, enabled) {
    return request({
      url: `${baseUrl}/rules/${id}/status`,
      method: 'put',
      params: { enabled }
    })
  },

  getRuleGroups(params) {
    return listRequest('/rule-groups', params)
  },
  getRuleGroupTree(params) {
    return listRequest('/rule-groups/all/tree', params)
  },
  getRuleGroup(id) {
    return detailRequest('/rule-groups', id)
  },
  createRuleGroup(data) {
    return createRequest('/rule-groups', data)
  },
  updateRuleGroup(id, data) {
    return updateRequest('/rule-groups', id, data)
  },
  deleteRuleGroup(id) {
    return deleteRequest('/rule-groups', id)
  },

  getBuiltinRules(params) {
    return listRequest('/builtin-rules', params)
  },
  previewImportPrometheusBuiltinRules(params) {
    return listRequest('/builtin-rules/import-prometheus/preview', params)
  },
  getPrometheusBuiltinRuleImportTypes(params) {
    return listRequest('/builtin-rules/import-prometheus/types', params)
  },
  importPrometheusBuiltinRules(data) {
    return createRequest('/builtin-rules/import-prometheus', data)
  },
  getBuiltinRule(id) {
    return detailRequest('/builtin-rules', id)
  },
  createBuiltinRule(data) {
    return createRequest('/builtin-rules', data)
  },
  updateBuiltinRule(id, data) {
    return updateRequest('/builtin-rules', id, data)
  },
  deleteBuiltinRule(id) {
    return deleteRequest('/builtin-rules', id)
  },
  getBuiltinRuleGroupTree(params) {
    return listRequest('/builtin-rule-groups/tree', params)
  },
  getBuiltinRuleGroupOptions(params) {
    return listRequest('/builtin-rule-groups/options', params)
  },
  getBuiltinRuleGroups(params) {
    return listRequest('/builtin-rule-groups', params)
  },
  getBuiltinRuleGroup(id) {
    return detailRequest('/builtin-rule-groups', id)
  },
  createBuiltinRuleGroup(data) {
    return createRequest('/builtin-rule-groups', data)
  },
  updateBuiltinRuleGroup(id, data) {
    return updateRequest('/builtin-rule-groups', id, data)
  },
  deleteBuiltinRuleGroup(id) {
    return deleteRequest('/builtin-rule-groups', id)
  },

  getDatasources(params) {
    return listRequest('/datasources', params)
  },
  getDatasourceOptions(params) {
    return listRequest('/datasources/options', params)
  },
  getDatasourceTypes(params) {
    return listRequest('/datasources/types', params)
  },
  getDatasource(id) {
    return detailRequest('/datasources', id)
  },
  createDatasource(data) {
    return createRequest('/datasources', data)
  },
  updateDatasource(id, data) {
    return updateRequest('/datasources', id, data)
  },
  deleteDatasource(id) {
    return deleteRequest('/datasources', id)
  },
  updateDatasourceStatus(id, enabled) {
    return request({
      url: `${baseUrl}/datasources/${id}/status`,
      method: 'put',
      params: { enabled }
    })
  },
  queryMetricRange(params) {
    return request({ url: `${baseUrl}/metrics/query_range`, method: 'get', params, timeout: 30000 })
  },
  queryMetric(params) {
    return request({ url: `${baseUrl}/metrics/query`, method: 'get', params, timeout: 30000 })
  },
  getMetricTargets(params) {
    return listRequest('/metrics/targets', params)
  },

  getDomains(params) {
    return listRequest('/domains', params)
  },
  getDomain(id) {
    return detailRequest('/domains', id)
  },
  createDomain(data) {
    return createRequest('/domains', data)
  },
  updateDomain(id, data) {
    return updateRequest('/domains', id, data)
  },
  deleteDomain(id) {
    return deleteRequest('/domains', id)
  },
  checkDomain(id) {
    return request({ url: `${baseUrl}/domains/${id}/check`, method: 'post', timeout: 70000 })
  },
  checkDomains(data = {}) {
    return request({ url: `${baseUrl}/domains/check`, method: 'post', data, timeout: 120000 })
  },
  updateDomainStatus(id, enabled) {
    return request({
      url: `${baseUrl}/domains/${id}/status`,
      method: 'put',
      params: { enabled }
    })
  },

  getSSLCertificates(params) {
    return listRequest('/ssl-certificates', params)
  },
  getSSLCertificate(id) {
    return detailRequest('/ssl-certificates', id)
  },
  applySSLCertificate(data) {
    return createRequest('/ssl-certificates/apply', data)
  },
  downloadSSLCertificate(id) {
    return request({
      url: `${baseUrl}/ssl-certificates/${id}/download`,
      method: 'get',
      responseType: 'blob'
    })
  },
  deleteSSLCertificate(id) {
    return deleteRequest('/ssl-certificates', id)
  },
  deploySSLCertificate(id, data) {
    return createRequest(`/ssl-certificates/${id}/deploy`, data)
  },
  getSSLCertificateDeployLogs(id, params) {
    return listRequest(`/ssl-certificates/${id}/deploy-logs`, params)
  },
  getSSLCertificateDeployLog(id, logId) {
    return detailRequest(`/ssl-certificates/${id}/deploy-logs`, logId)
  },

  getSilenceRules(params) {
    return listRequest('/silence-rules', params)
  },
  getSilenceRule(id) {
    return detailRequest('/silence-rules', id)
  },
  createSilenceRule(data) {
    return createRequest('/silence-rules', data)
  },
  updateSilenceRule(id, data) {
    return updateRequest('/silence-rules', id, data)
  },
  deleteSilenceRule(id) {
    return deleteRequest('/silence-rules', id)
  },

  getNotifyChannels(params) {
    return listRequest('/notify-channels', params)
  },
  getNotifyChannel(id) {
    return detailRequest('/notify-channels', id)
  },
  createNotifyChannel(data) {
    return createRequest('/notify-channels', data)
  },
  testNotifyChannel(id, data) {
    return createRequest(`/notify-channels/${id}/test`, data)
  },
  updateNotifyChannel(id, data) {
    return updateRequest('/notify-channels', id, data)
  },
  deleteNotifyChannel(id) {
    return deleteRequest('/notify-channels', id)
  },

  getNotifyGroups(params) {
    return listRequest('/notify-groups', params)
  },
  getNotifyGroup(id) {
    return detailRequest('/notify-groups', id)
  },
  createNotifyGroup(data) {
    return createRequest('/notify-groups', data)
  },
  updateNotifyGroup(id, data) {
    return updateRequest('/notify-groups', id, data)
  },
  deleteNotifyGroup(id) {
    return deleteRequest('/notify-groups', id)
  },

  getNotifyRouteRules(params) {
    return listRequest('/notify-route-rules', params)
  },
  getNotifyRouteRule(id) {
    return detailRequest('/notify-route-rules', id)
  },
  createNotifyRouteRule(data) {
    return createRequest('/notify-route-rules', data)
  },
  updateNotifyRouteRule(id, data) {
    return updateRequest('/notify-route-rules', id, data)
  },
  deleteNotifyRouteRule(id) {
    return deleteRequest('/notify-route-rules', id)
  },

  getNotifyTemplates(params) {
    return listRequest('/notify-templates', params)
  },
  getNotifyTemplate(id) {
    return detailRequest('/notify-templates', id)
  },
  createNotifyTemplate(data) {
    return createRequest('/notify-templates', data)
  },
  updateNotifyTemplate(id, data) {
    return updateRequest('/notify-templates', id, data)
  },
  deleteNotifyTemplate(id) {
    return deleteRequest('/notify-templates', id)
  },
  previewNotifyTemplate(id, data) {
    return createRequest(`/notify-templates/${id}/preview`, data)
  },

  getNotifyRules(params) {
    return listRequest('/notify-rules', params)
  },
  getNotifyRule(id) {
    return detailRequest('/notify-rules', id)
  },
  createNotifyRule(data) {
    return createRequest('/notify-rules', data)
  },
  updateNotifyRule(id, data) {
    return updateRequest('/notify-rules', id, data)
  },
  deleteNotifyRule(id) {
    return deleteRequest('/notify-rules', id)
  },
  testNotifyRule(id, data) {
    return createRequest(`/notify-rules/${id}/test`, data)
  },
  previewNotifyRuleMatch(data) {
    return createRequest('/notify-rules/match-preview', data)
  }
}
