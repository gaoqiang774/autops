import request from '@/utils/request'

const accountUrl = '/api/v1/cost-center/accounts'
const balanceUrl = '/api/v1/cost-center/balance'

export default {
  getKeyList(params) {
    return request({ url: '/api/v1/config/keymanage/list', method: 'get', params })
  },

  getAccounts(params) {
    return request({ url: accountUrl, method: 'get', params })
  },

  createAccount(data) {
    return request({ url: accountUrl, method: 'post', data })
  },

  updateAccount(id, data) {
    return request({ url: `${accountUrl}/${id}`, method: 'put', data })
  },

  deleteAccount(id) {
    return request({ url: `${accountUrl}/${id}`, method: 'delete' })
  },

  syncBills(id) {
    return request({ url: `${accountUrl}/${id}/sync-bills`, method: 'post' })
  },

  refreshBalance(id) {
    return request({ url: `${accountUrl}/${id}/refresh-balance`, method: 'post' })
  },

  syncAll() {
    return request({ url: `${accountUrl}/sync-all`, method: 'post' })
  },

  refreshAllBalances() {
    return request({ url: `${accountUrl}/refresh-all-balances`, method: 'post' })
  },

  getBalanceAlert(id) {
    return request({ url: `${accountUrl}/${id}/balance-alert`, method: 'get' })
  },

  saveBalanceAlert(id, data) {
    return request({ url: `${accountUrl}/${id}/balance-alert`, method: 'put', data })
  },

  getBalanceOverview() {
    return request({ url: `${balanceUrl}/overview`, method: 'get' })
  }
}
