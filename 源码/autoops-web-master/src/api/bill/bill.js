import request from '@/utils/request'

const baseUrl = '/api/v1/cost-center'

export default {
  getOverview(params) {
    return request({ url: `${baseUrl}/overview`, method: 'get', params })
  },

  getProviderDetail(operator, params) {
    return request({ url: `${baseUrl}/providers/${operator}`, method: 'get', params })
  },

  getDetails(params) {
    return request({ url: `${baseUrl}/details`, method: 'get', params })
  },

  getDetail(id) {
    return request({ url: `${baseUrl}/details/${id}`, method: 'get' })
  },

  exportDetails(params) {
    return request({
      url: `${baseUrl}/details/export`,
      method: 'get',
      params,
      responseType: 'blob'
    })
  }
}
