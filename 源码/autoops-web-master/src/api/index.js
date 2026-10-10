/**
 * 基础API接口管理
 *
 * @author xiaoRui
 */

import request from "@/utils/request"
import systemAPI from './system'
import cmdbAPI from './cmdb'
import dashboardAPI from './dashboard'
import configAPI from './config'
import aiopsAPI from './aiops'
import logtailAPI from './logtail'
import taskAPI from './task'

export default {
    // 基础API
    captcha() {
        return request({
            url: 'captcha',
            method: 'get'
        })
    },
    login(params) {
        return request({
            url: 'login',
            method: 'post',
            data: params
        })
    },
    // 系统管理API - 扩展到根级别以兼容现有代码
    ...systemAPI,
    // CMDB管理API - 扩展到根级别以兼容现有代码
    ...cmdbAPI,
    // Dashboard管理API - 扩展到根级别以兼容现有代码
    ...dashboardAPI,
    // 配置中心API - 扩展到根级别以兼容现有代码
    ...configAPI,
    // AIOps管理API - 扩展到根级别以兼容现有代码
    ...aiopsAPI,
    // 日志中心API
    ...logtailAPI,
    // 主机监控兼容接口
    getHostCompleteMonitorData(hostId, params = {}) {
        return cmdbAPI.getCompleteHostMetrics(hostId, params)
    },
    // 主机Top进程
    getHostTopProcesses: cmdbAPI.getHostTopProcesses,

    // 同时提供命名空间访问
    system: systemAPI,
    cmdb: cmdbAPI,
    dashboard: dashboardAPI,
    config: configAPI,
    aiops: aiopsAPI,
    logtail: logtailAPI,
    task: taskAPI,
}
