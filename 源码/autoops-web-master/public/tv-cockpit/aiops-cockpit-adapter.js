(function () {
  if (!window.axios || window.__AIOPS_COCKPIT_ADAPTER__) return
  window.__AIOPS_COCKPIT_ADAPTER__ = true

  var originalCreate = window.axios.create.bind(window.axios)
  var defaultAdapter = window.axios.defaults.adapter
  var nowSeconds = function () { return Math.floor(Date.now() / 1000) }
  var random = function (min, max) { return Math.round(min + Math.random() * (max - min)) }
  var json = function (data) {
    return { code: 0, message: '', data: data }
  }
  var debug = /\bcockpitDebug=1\b/.test(window.location.search)
  var log = function () {
    if (!debug || !window.console) return
    window.console.log.apply(window.console, ['[cockpit-adapter]'].concat([].slice.call(arguments)))
  }

  var assets = [
    { label: '物理机', total: 128, alarms: { 1: 4, 2: 2, 3: 1, 4: 0, 5: 0 } },
    { label: '虚拟机', total: 342, alarms: { 1: 7, 2: 4, 3: 2, 4: 1, 5: 0 } },
    { label: '交换机', total: 46, alarms: { 1: 1, 2: 1, 3: 0, 4: 0, 5: 0 } },
    { label: '数据库', total: 38, alarms: { 1: 2, 2: 1, 3: 1, 4: 0, 5: 0 } },
    { label: '中间件', total: 65, alarms: { 1: 3, 2: 2, 3: 0, 4: 0, 5: 0 } },
    { label: 'K8S 节点', total: 24, alarms: { 1: 1, 2: 0, 3: 0, 4: 0, 5: 0 } },
    { label: '云主机', total: 89, alarms: { 1: 4, 2: 2, 3: 1, 4: 0, 5: 0 } },
    { label: '应用服务', total: 156, alarms: { 1: 5, 2: 3, 3: 1, 4: 0, 5: 0 } },
    { label: '存储设备', total: 18, alarms: { 1: 0, 2: 1, 3: 0, 4: 0, 5: 0 } }
  ]

  var topHosts = [
    'prod-app-01',
    'prod-app-02',
    'prod-db-01',
    'k8s-node-03',
    'gateway-01',
    'redis-01',
    'monitor-01',
    'backup-01',
    'web-02',
    'ops-jump-01'
  ]

  var formatTime = function (seconds) {
    var d = new Date(seconds * 1000)
    var pad = function (value) { return String(value).padStart(2, '0') }
    return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()) + ' ' +
      pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds())
  }

  var makeTop = function (type) {
    return {
      title: type === 'memory' ? '内存使用率 TOP10' : 'CPU 使用率 TOP10',
      list: topHosts.map(function (name, index) {
        return {
          name: name,
          desc: '10.10.' + (index + 1) + '.' + (20 + index),
          value: Math.max(18, random(82 - index * 4, 96 - index * 3)),
          unit: '%',
          itemid: type + '-' + (index + 1)
        }
      })
    }
  }

  var trend = function (unit) {
    var end = Date.now()
    var rows = []
    for (var i = 23; i >= 0; i -= 1) {
      rows.push([end - i * 5 * 60 * 1000, random(unit === '%' ? 20 : 1, unit === '%' ? 92 : 10)])
    }
    return rows
  }

  var realtimeAlerts = function () {
    var ts = nowSeconds()
    return [
      { eventid: 'evt-1001', clock: ts - 120, priority: '4', name: 'prod-db-01', ip: '10.10.3.21', description: '数据库连接数过高' },
      { eventid: 'evt-1002', clock: ts - 260, priority: '3', name: 'k8s-node-03', ip: '10.10.4.23', description: '节点 CPU 使用率过高' },
      { eventid: 'evt-1003', clock: ts - 420, priority: '2', name: 'gateway-01', ip: '10.10.5.24', description: '网关响应时间升高' },
      { eventid: 'evt-1004', clock: ts - 680, priority: '2', name: 'redis-01', ip: '10.10.6.25', description: 'Redis 内存使用率过高' }
    ]
  }

  var handlers = {
    '/system/info': function () {
      return { site: { ico: '' } }
    },
    '/cockpit/perfmon/info': function () {
      var total = assets.reduce(function (sum, item) { return sum + item.total }, 0)
      return {
        title: 'AIOps 运维驾驶舱',
        now_time: nowSeconds(),
        is_running: true,
        asset_type: 1,
        main_item_custom_config: 1,
        asset_total: total,
        asset_overview: assets,
        alarm_overview: [
          { label: '信息', priority: '1', count: 24 },
          { label: '警告', priority: '2', count: 16 },
          { label: '次要', priority: '3', count: 6 },
          { label: '严重', priority: '4', count: 2 },
          { label: '紧急', priority: '5', count: 0 }
        ],
        alarm_trends: {},
        hosttypes: {},
        prioritys: {}
      }
    },
    '/cockpit/perfmon/main-item': function () {
      return {
        title: '重点指标监控',
        list: [
          { name: '核心业务可用率', metric_value: '99.98%', status_metric_value: '正常' },
          { name: '主机监控覆盖率', metric_value: '96.3%', status_metric_value: '正常' },
          { name: '今日告警恢复率', metric_value: '91.6%', status_metric_value: '正常' },
          { name: '平均响应时间', metric_value: '128ms', status_metric_value: '正常' }
        ]
      }
    },
    '/cockpit/perfmon/usage-rate': function () {
      return {
        cpu: { value: random(28, 76) },
        ram: { value: random(35, 82) }
      }
    },
    '/cockpit/perfmon/item-top': function (config) {
      return makeTop((config.params || {}).type)
    },
    '/cockpit/perfmon/item-lastest': function (config) {
      var unit = String((config.params || {}).itemid || '').indexOf('memory') === 0 ? '%' : '%'
      return { graph: [[{ unit: unit, data: trend(unit) }]] }
    },
    '/cockpit/perfmon/real-time': function () {
      return { rows: realtimeAlerts() }
    },
    '/cockpit/perfmon/alert-object': function () {
      return {
        rows: topHosts.slice(0, 10).map(function (name, index) {
          return { name: name, total: Math.max(1, 18 - index * 2) }
        })
      }
    },
    '/cockpit/perfmon/problem-info': function (config) {
      var ts = Number((config.params || {}).clock) || nowSeconds()
      return {
        problem: {
          clock: ts,
          description: '监控项触发阈值，请关注资源使用率变化',
          ip: '10.10.3.21',
          name: 'prod-db-01',
          comments: '当前为前端适配数据，可继续接入真实告警接口。'
        },
        history: [
          { item_name: 'CPU 使用率', last_time: formatTime(ts), last_value: random(70, 96) + '%' },
          { item_name: '内存使用率', last_time: formatTime(ts), last_value: random(65, 90) + '%' },
          { item_name: '磁盘使用率', last_time: formatTime(ts), last_value: random(45, 78) + '%' }
        ]
      }
    }
  }

  var normalizeUrl = function (config) {
    var base = config.baseURL || ''
    var url = config.url || ''
    try {
      var full = new URL(url, base || window.location.origin)
      return full.pathname.replace(/^\/api\/v1/, '')
    } catch (e) {
      return String(url).replace(/^\/api\/v1/, '')
    }
  }

  window.addEventListener('error', function (event) {
    if (window.console) window.console.error('[cockpit-error]', event.error || event.message)
  })
  window.addEventListener('unhandledrejection', function (event) {
    if (window.console) window.console.error('[cockpit-promise]', event.reason)
  })

  var makeAdapter = function (fallbackAdapter) {
    return function (config) {
      var path = normalizeUrl(config)
      if (handlers[path]) {
        var payload = json(handlers[path](config))
        log('mock', path, payload.data)
        return Promise.resolve({
          data: payload,
          status: 200,
          statusText: 'OK',
          headers: { 'content-type': 'application/json' },
          config: config,
          request: null
        })
      }
      if (fallbackAdapter) return fallbackAdapter(config)
      return Promise.reject(new Error('No axios adapter for ' + (config.url || 'unknown url')))
    }
  }

  window.axios.defaults.adapter = makeAdapter(defaultAdapter)

  window.axios.create = function () {
    var instance = originalCreate.apply(window.axios, arguments)
    var originalAdapter = instance.defaults.adapter || window.axios.defaults.adapter || defaultAdapter
    instance.defaults.adapter = makeAdapter(originalAdapter)
    if (instance.interceptors && instance.interceptors.request) {
      instance.interceptors.request.use(function (config) {
        log('request', normalizeUrl(config), config.params || {})
        return config
      })
    }
    return instance
  }
})()
