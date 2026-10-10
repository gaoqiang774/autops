import request from '@/utils/request'

export function listBusinessModels(params) {
  return request({
    url: '/cmdb/business-models',
    method: 'get',
    params
  })
}

export function createBusinessModel(data) {
  return request({
    url: '/cmdb/business-models',
    method: 'post',
    data
  })
}

export function deleteBusinessModel(id) {
  if (!id && id !== 0) {
    return Promise.reject(new Error('业务模型 ID 不能为空'))
  }

  return request({
    url: `/cmdb/business-models/${id}`,
    method: 'delete'
  })
}

export function getBusinessOverview(id) {
  if (!id && id !== 0) {
    return Promise.reject(new Error('业务模型 ID 不能为空'))
  }

  return request({
    url: `/cmdb/business-models/${id}/overview`,
    method: 'get'
  })
}

export function getBusinessTopology(id, params) {
  if (!id && id !== 0) {
    return Promise.reject(new Error('业务模型 ID 不能为空'))
  }

  return request({
    url: `/cmdb/business-models/${id}/topology`,
    method: 'get',
    params
  })
}

export function getBusinessObservability(id, params) {
  if (!id && id !== 0) {
    return Promise.reject(new Error('业务模型 ID 不能为空'))
  }

  return request({
    url: `/cmdb/business-models/${id}/observability`,
    method: 'get',
    params
  })
}

export function getBusinessLogAnalysis(id, params) {
  if (!id && id !== 0) {
    return Promise.reject(new Error('业务模型 ID 不能为空'))
  }

  return request({
    url: `/cmdb/business-models/${id}/log-analysis`,
    method: 'get',
    params
  })
}

export function getBusinessMonitorAlerts(id, params) {
  if (!id && id !== 0) {
    return Promise.reject(new Error('业务模型 ID 不能为空'))
  }

  return request({
    url: `/cmdb/business-models/${id}/monitor-alerts`,
    method: 'get',
    params
  })
}

export function getBusinessMonitorAlertConfig(id) {
  if (!id && id !== 0) {
    return Promise.reject(new Error('业务模型 ID 不能为空'))
  }

  return request({
    url: `/cmdb/business-models/${id}/monitor-alerts/config`,
    method: 'get'
  })
}

export function saveBusinessMonitorAlertConfig(id, data) {
  if (!id && id !== 0) {
    return Promise.reject(new Error('业务模型 ID 不能为空'))
  }

  return request({
    url: `/cmdb/business-models/${id}/monitor-alerts/config`,
    method: 'put',
    data
  })
}

export function bindBusinessResources(id, data) {
  if (!id && id !== 0) {
    return Promise.reject(new Error('业务模型 ID 不能为空'))
  }

  return request({
    url: `/cmdb/business-models/${id}/bindings`,
    method: 'post',
    data
  })
}

export function refreshBusinessHealth(id) {
  if (!id && id !== 0) {
    return Promise.reject(new Error('业务模型 ID 不能为空'))
  }

  return request({
    url: `/cmdb/business-models/${id}/health/refresh`,
    method: 'post'
  })
}
