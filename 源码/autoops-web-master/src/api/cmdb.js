import request from "@/utils/request"
import { buildWebSocketUrl } from "@/utils/url"
const normalizeIdsParam = ids => Array.isArray(ids) ? ids.join(',') : ids

export default {
    // 鑾峰彇TOP璧勬簮浣跨敤涓绘満
    getTopUsageHosts(params = {}) {
        return request({
            url: 'cmdb/hosts/top-usage',
            method: 'get',
            params
        })
    },

    // 获取主机实时排名（CPU/内存/根分区）
    getHostsCurrentRankings(params = {}) {
        return request({
            url: 'cmdb/hosts/current-rankings',
            method: 'get',
            params: {
                limit: params.limit || 10,
                ...params
            }
        })
    },

    // 鑾峰彇鍗曚釜涓绘満鐨勫綋鍓嶇洃鎺ф憳瑕?
    getHostMetrics(id, params = {}) {
        return request({
            url: `cmdb/hosts/${id}`,
            method: 'get',
            params
        })
    },

    // 鑾峰彇涓绘満鍘嗗彶鐩戞帶鏁版嵁
    getHostMetricHistory(id, params = {}) {
        return request({
            url: `cmdb/hosts/${id}/history`,
            method: 'get',
            params
        })
    },
    // 鑾峰彇涓绘満鐩戞帶鍏ㄩ噺鏁版嵁
    getHostMonitorData(params) {
        console.log('璇锋眰鐩戞帶鏁版嵁鍙傛暟:', JSON.stringify(params, null, 2))
        const { id, duration, start, end } = params
        let url = `cmdb/hosts/${id}/all-metrics`
        const queryParams = {}
        
        if (duration) {
            queryParams.duration = duration
        } else if (start && end) {
            queryParams.start = start
            queryParams.end = end
        }
        
        return request({
            url,
            method: 'get',
            params: queryParams
        })
    },

    // 鎵归噺鑾峰彇涓绘満绠€瑕佺洃鎺ф暟鎹?
    getHostsMonitorData(ids) {
        console.log('璇锋眰鎵归噺鐩戞帶鏁版嵁锛屼富鏈篒Ds:', ids)
        return request({
            url: 'cmdb/hosts',
            method: 'get',
            params: { ids: normalizeIdsParam(ids) }
        })
    },

    // 鑾峰彇涓绘満鍏ㄩ噺鍘嗗彶鐩戞帶鏁版嵁
    getHostAllMetricsHistory(id, params = {}) {
        return request({
            url: `cmdb/hosts/${id}/all-metrics`,
            method: 'get',
            params
        })
    },

    getHostServiceMetricsHistory(id, serviceName, params = {}) {
        return request({
            url: `cmdb/hosts/${id}/services/${encodeURIComponent(serviceName)}/metrics`,
            method: 'get',
            params
        })
    },

    // 鑾峰彇涓绘満瀹屾暣鐩戞帶鎽樿
    getCompleteHostMetrics(id, params = {}) {
        return request({
            url: `cmdb/hosts/${id}/complete`,
            method: 'get',
            params
        })
    },

    getHostWindowsDashboard(id, params = {}) {
        return request({
            url: `cmdb/hosts/${id}/windows-dashboard`,
            method: 'get',
            params
        })
    },

    // 鑾峰彇涓绘満TOP杩涚▼鐩戞帶鏁版嵁
    getHostTopProcesses(hostId) {
        return request({
            url: `cmdb/hosts/${hostId}/top-processes`,
            method: 'get'
        })
    },

    // 鑾峰彇涓绘満TCP绔彛鐩戝惉鐘舵€?
    getHostTcpPorts(hostId) {
        return request({
            url: `cmdb/hosts/${hostId}/ports`,
            method: 'get'
        })
    },    // 鑾峰彇涓绘満鍒楄〃
    getHostList(params) {
        return request({
            url: 'cmdb/host/list',
            method: 'get',
            params
        })
    },
    getAllCmdbGroups() {
        return request({
            url: 'cmdb/grouplist',
            method: 'get'
        })
    },
    getCmdbGroupByName(name) {
        return request({
            url: 'cmdb/groupbyname',
            method: 'get',
            params: {
                name: name
            }
        })
    },
    createCmdbGroup(data) {
        return request({
            url: 'cmdb/groupadd',
            method: 'post',
            data: data
        })
    },
    updateCmdbGroup(data) {
        return request({
            url: 'cmdb/groupupdate',
            method: 'put',
            data: data
        })
    },
    deleteCmdbGroup(id) {
        const data = {
            id
        }
        return request({
            url: 'cmdb/groupdelete',
            method: 'delete',
            data: data
        })
    },

    // 涓绘満绠＄悊
    createCmdbHost(data) {
        return request({
            url: 'cmdb/hostcreate',
            method: 'post',
            data: data
        })
    },
    updateCmdbHost(data) {
        return request({
            url: 'cmdb/hostupdate',
            method: 'put',
            data: data
        })
    },
    deleteCmdbHost(id) {
        return request({
            url: 'cmdb/hostdelete',
            method: 'delete',
            data: { id }
        })
    },
    // 鎵归噺鍒犻櫎涓绘満
    batchDeleteCmdbHost(hostIds) {
        return request({
            url: 'cmdb/hostbatchdelete',
            method: 'delete',
            data: { hostIds }
        })
    },
    getCmdbHostList(params) {
        // 纭繚鍙傛暟鍚嶄笌鍚庣API涓€鑷?
        const queryParams = {
            page: params.page || 1,
            pageSize: params.pageSize || 10,
            sortBy: 'update_time',        // 鎸夋洿鏂版椂闂存帓搴?
            sortOrder: 'desc'    // 闄嶅簭鎺掑垪锛堟渶鏂扮殑鍦ㄥ墠闈級
        }

        // 娣诲姞鎼滅储鏉′欢锛岀‘淇濆弬鏁板悕涓庡悗绔疉PI涓€鑷?
        if (params.name) queryParams.name = params.name
        if (params.ip) queryParams.ip = params.ip
        if (params.status) queryParams.status = params.status
        if (params.groupId) queryParams.groupId = params.groupId

        console.log('鏈€缁圓PI鏌ヨ鍙傛暟:', JSON.stringify(queryParams, null, 2))
        return request({
            url: 'cmdb/hostlist',
            method: 'get',
            params: queryParams
        })
    },
    getCmdbHostById(id) {
        return request({
            url: 'cmdb/hostinfo',
            method: 'get',
            params: { id }
        })
    },
    GetCmdbHostsByKeyword(keyword, params) {
        return request({
            url: 'cmdb/hostsearch',
            method: 'get',
            params: {
                keyword,
                sortBy: 'id',
                sortOrder: 'desc',
                ...params
            }
        })
    },
    GetCmdbHostsByHostNameLike(hostName, params) {
        return request({
            url: '/cmdb/hostbyname',
            method: 'get',
            params: {
                name: hostName,
                sortBy: 'id',
                sortOrder: 'desc',
                ...params
            }
        })
    },
    GetCmdbHostsByIP(ip, params) {
        return request({
            url: 'cmdb/hostbyip',
            method: 'get',
            params: {
                ip,
                sortBy: 'id',
                sortOrder: 'desc',
                ...params
            }
        })
    },
    GetCmdbHostsByStatus(status, params) {
        return request({
            url: 'cmdb/hostbystatus',
            method: 'get',
            params: {
                status,
                sortBy: 'id',
                sortOrder: 'desc',
                ...params
            }
        })
    },
    GetCmdbHostsByVendor(vendor, params = {}) {
        return request({
            url: 'cmdb/hostbytype',
            method: 'get',
            params: {
                vendor,
                sortBy: 'id',
                sortOrder: 'desc',
                ...params
            }
        })
    },
    GetCmdbHostsByHostType(hostType, params = {}) {
        return request({
            url: 'cmdb/hostbyhosttype',
            method: 'get',
            params: {
                hostType,
                sortBy: 'id',
                sortOrder: 'desc',
                ...params
            }
        })
    },
    getCmdbHostsByGroupId(groupId, params) {
        return request({
            url: 'cmdb/hostgroup',
            method: 'get',
            params: {
                groupId,
                sortBy: 'id',
                sortOrder: 'desc',
                ...params
            }
        })
    },
    hostcloudcreatealiyun(data) {
        return request({
            url: 'cmdb/hostcloudcreatealiyun',
            method: 'post',
            data: data
        })
    },
    hostcloudcreatetencent(data) {
        return request({
            url: 'cmdb/hostcloudcreatetencent',
            method: 'post',
            data: data
        })
    },
    hostcloudcreatehuawei(data) {
        return request({
            url: 'cmdb/hostcloudcreatehuawei',
            method: 'post',
            data: data
        })
    },
    hostcloudcreateaws(data) {
        return request({
            url: 'cmdb/hostcloudcreateaws',
            method: 'post',
            data: data
        })
    },
    hostcloudcreatetianyi(data) {
        return request({
            url: 'cmdb/hostcloudcreatetianyi',
            method: 'post',
            data: data
        })
    },
    // 鑾峰彇浜戜富鏈虹粺璁′俊鎭?
    getHostCloudStatistics() {
        return request({
            url: 'cmdb/hostcloudstatistics',
            method: 'get'
        })
    },

    // 鑾峰彇甯︿富鏈哄垪琛ㄧ殑鍒嗙粍鏁版嵁
    getGroupListWithHosts() {
        return request({
            url: 'cmdb/grouplistwithhosts',
            method: 'get'
        })
    },

    // WebSocket杩炴帴
    getHostSSHWebSocketUrl(hostId, authId) {
        const authQuery = authId ? `?authId=${encodeURIComponent(authId)}` : ''
        return buildWebSocketUrl(`/api/v1/cmdb/hostssh/connect/${hostId}${authQuery}`)
    },

    // 获取主机可用SSH认证账号
    getHostSshAuths(hostId) {
        return request({
            url: `cmdb/hostssh/auths/${hostId}`,
            method: 'get'
        })
    },

    // 鏂囦欢涓婁紶鍒颁富鏈?
    uploadFileToHost(hostId, formData, config = {}) {
        return request({
            url: `cmdb/hostssh/upload/${hostId}`,
            method: 'post',
            data: formData,
            headers: {
                'Content-Type': 'multipart/form-data'
            },
            ...config
        })
    },

    // 打开或创建 RDP 会话
    openRDPSession(rdpid, data = {}) {
        return request({
            url: 'rdp/session/open',
            method: 'post',
            data: {
                rdpid: String(rdpid),
                ...data
            }
        })
    },

    // 上传文件到当前 RDP 会话
    uploadFileToRDPSession(sessionId, formData, config = {}) {
        return request({
            url: `rdp/upload/${sessionId}`,
            method: 'post',
            data: formData,
            headers: {
                'Content-Type': 'multipart/form-data'
            },
            timeout: 600000,
            ...config
        })
    },

    // 上传文件到 Windows 主机 Downloads 目录
    uploadFileToWindowsHost(hostId, formData, config = {}) {
        return request({
            url: `rdp/upload/${hostId}`,
            method: 'post',
            data: formData,
            headers: {
                'Content-Type': 'multipart/form-data'
            },
            timeout: 600000,
            ...config
        })
    },

    getWindowsUploadTaskStatus(taskId, config = {}) {
        return request({
            url: `rdp/upload/task/${taskId}`,
            method: 'get',
            ...config
        })
    },

    // 鏌ヨ涓绘満鏂囦欢涓婁紶浠诲姟鐘舵€?
    getUploadFileTaskStatus(taskId, config = {}) {
        return request({
            url: `cmdb/hostssh/upload/task/${taskId}`,
            method: 'get',
            ...config
        })
    },

    // 浠庝富鏈轰笅杞芥枃浠?
    downloadFileFromHost(hostId, remotePath, config = {}) {
        return request({
            url: `cmdb/hostssh/download/${hostId}`,
            method: 'get',
            params: {
                remotePath: remotePath
            },
            responseType: 'blob',
            timeout: 600000, // 10鍒嗛挓瓒呮椂锛屾敮鎸佸ぇ鏂囦欢涓嬭浇
            ...config
        })
    },

    // 鍒楀嚭涓绘満杩滅▼鐩綍鏂囦欢
    listRemoteFiles(hostId, remotePath = '/') {
        return request({
            url: `cmdb/hostssh/listfiles/${hostId}`,
            method: 'get',
            params: {
                remotePath: remotePath
            }
        })
    },

    // SSH鎵ц鍛戒护
    executeHostCommand(hostId, command, authId) {
        return request({
            url: `cmdb/hostssh/command/${hostId}`,
            method: 'get',
            params: {
                command: command,
                ...(authId ? { authId } : {})
            }
        })
    },

    // 鏁版嵁搴撶鐞?
    createDatabase(data) {
        return request({
            url: 'cmdb/database',
            method: 'post',
            data: data
        })
    },
    updateDatabase(data) {
        return request({
            url: 'cmdb/database',
            method: 'put',
            data: data
        })
    },
    deleteDatabase(id) {
        return request({
            url: 'cmdb/database',
            method: 'delete',
            data: { id }
        })
    },
    getDatabase(id) {
        return request({
            url: 'cmdb/database/info',
            method: 'get',
            params: { id }
        })
    },
    listDatabases(params = {}) {
        return request({
            url: 'cmdb/databaselist',
            method: 'get',
            params: params
        })
    },
    getDatabasesByName(name, params = {}) {
        return request({
            url: 'cmdb/database/byname',
            method: 'get',
            params: {
                name,
                ...params
            }
        })
    },
    getDatabasesByType(type, params = {}) {
        return request({
            url: 'cmdb/database/bytype',
            method: 'get',
            params: {
                type,
                ...params
            }
        })
    },

    // 数据库资产台账
    getDatabaseAssetList(params = {}) {
        return request({
            url: 'cmdb/database-asset/list',
            method: 'get',
            params
        })
    },
    createDatabaseAsset(data) {
        return request({
            url: 'cmdb/database-asset/create',
            method: 'post',
            data
        })
    },
    updateDatabaseAsset(data) {
        return request({
            url: 'cmdb/database-asset/update',
            method: 'put',
            data
        })
    },
    deleteDatabaseAsset(id) {
        return request({
            url: 'cmdb/database-asset/delete',
            method: 'delete',
            params: { id }
        })
    },
    getDatabaseAssetInfo(id) {
        return request({
            url: 'cmdb/database-asset/info',
            method: 'get',
            params: { id }
        })
    },
    getDatabaseAssetByName(name) {
        return request({
            url: 'cmdb/database-asset/byname',
            method: 'get',
            params: { name }
        })
    },
    getDatabaseAssetsByType(type) {
        return request({
            url: 'cmdb/database-asset/bytype',
            method: 'get',
            params: { type }
        })
    },
    getDatabaseAssetStatistics() {
        return request({
            url: 'cmdb/database-asset/statistics',
            method: 'get'
        })
    },
    testDatabaseAssetAuth(id) {
        return request({
            url: 'cmdb/database-asset/test-auth',
            method: 'post',
            params: { id }
        })
    },

    // SQL鏌ヨ
    executeSelectSQL(data) {
        console.log('executeSelectSQL璇锋眰鏁版嵁:', JSON.stringify(data, null, 2))
        return request({
            url: 'cmdb/sql/select',
            method: 'post',
            data: data,
            headers: {
                'Content-Type': 'application/json'
            }
        })
    },
    // SQL鎻掑叆
    executeInsertSQL(data) {
        console.log('executeInsertSQL璇锋眰鏁版嵁:', JSON.stringify(data, null, 2))
        return request({
            url: 'cmdb/sql',
            method: 'post',
            data: data,
            headers: {
                'Content-Type': 'application/json'
            }
        })
    },
    // SQL鏇存柊
    executeUpdateSQL(data) {
        console.log('executeUpdateSQL璇锋眰鏁版嵁:', JSON.stringify(data, null, 2))
        return request({
            url: 'cmdb/sql',
            method: 'put',
            data: data,
            headers: {
                'Content-Type': 'application/json'
            }
        })
    },
    executeDeleteSQL(data) {
        console.log('executeDeleteSQL璇锋眰鏁版嵁:', JSON.stringify(data, null, 2))
        return request({
            url: 'cmdb/sql',
            method: 'delete',
            data: data,
            headers: {
                'Content-Type': 'application/json'
            }
        })
    },
    executeRawSQL(data) {
        console.log('executeRawSQL璇锋眰鏁版嵁:', JSON.stringify(data, null, 2))
        return request({
            url: 'cmdb/sql/execute',
            method: 'post',
            data: data,
            headers: {
                'Content-Type': 'application/json'
            }
        })
    },
    // 鑾峰彇鏁版嵁搴撳垪琛?
    executeDatabase(data) {
        console.log('executeDatabase璇锋眰鏁版嵁:', JSON.stringify(data, null, 2))
        return request({
            url: 'cmdb/sql/databaselist',
            method: 'post',
            data: data,
            headers: {
                'Content-Type': 'application/json'
            }
        })
    },

    // SQL鏃ュ織绠＄悊
    GetCmdbSqlLogList(params) {
        return request({
            url: 'cmdb/sqlLog/list',
            method: 'get',
            params: params
        })
    },
    DeleteCmdbSqlLogById(id) {
        return request({
            url: 'cmdb/sqlLog/delete',
            method: 'delete',
            data: { id }
        })
    },
    CleanCmdbSqlLog() {
        return request({
            url: 'cmdb/sqlLog/clean',
            method: 'delete'
        })
    },

    // 浠嶦xcel瀵煎叆涓绘満
    ImportHostsFromExcel(data, config = {}) {
        return request({
            url: 'cmdb/hostimport',
            method: 'post',
            data: data,
            headers: {
                'Content-Type': 'multipart/form-data'
            },
            ...config
        })
    },

    // 涓嬭浇涓绘満瀵煎叆妯℃澘
    DownloadHostTemplate() {
        return request({
            url: 'cmdb/hosttemplate',
            method: 'get',
            responseType: 'blob'
        })
    },
    exportCmdbHosts(params = {}, config = {}) {
        return request({
            url: 'cmdb/hostexport',
            method: 'get',
            params,
            responseType: 'blob',
            ...config
        })
    },

    // IDC 鏈烘埧绠＄悊
    createCmdbIdcRoom(data) {
        return request({
            url: 'cmdb/idc-room/create',
            method: 'post',
            data
        })
    },
    updateCmdbIdcRoom(data) {
        return request({
            url: 'cmdb/idc-room/update',
            method: 'put',
            data
        })
    },
    deleteCmdbIdcRoom(id) {
        return request({
            url: 'cmdb/idc-room/delete',
            method: 'delete',
            data: { id }
        })
    },
    getCmdbIdcRoomInfo(id) {
        return request({
            url: 'cmdb/idc-room/info',
            method: 'get',
            params: { id }
        })
    },
    getCmdbIdcRoomList(params = {}) {
        return request({
            url: 'cmdb/idc-room/list',
            method: 'get',
            params
        })
    },
    getCmdbIdcRoomLayout(params = {}) {
        return request({
            url: 'cmdb/idc-room/layout',
            method: 'get',
            params
        })
    },

    // IDC 鏈烘煖绠＄悊
    createCmdbIdcCabinet(data) {
        return request({
            url: 'cmdb/idc-cabinet/create',
            method: 'post',
            data
        })
    },
    updateCmdbIdcCabinet(data) {
        return request({
            url: 'cmdb/idc-cabinet/update',
            method: 'put',
            data
        })
    },
    deleteCmdbIdcCabinet(id) {
        return request({
            url: 'cmdb/idc-cabinet/delete',
            method: 'delete',
            data: { id }
        })
    },
    getCmdbIdcCabinetList(roomId) {
        return request({
            url: 'cmdb/idc-cabinet/list',
            method: 'get',
            params: roomId ? { roomId } : {}
        })
    },
    getCmdbIdcCabinetLayout(params = {}) {
        return request({
            url: 'cmdb/idc-cabinet/layout',
            method: 'get',
            params
        })
    },

    createCmdbIdcAsset(data) {
        return request({
            url: 'cmdb/idc-asset/create',
            method: 'post',
            data
        })
    },
    updateCmdbIdcAsset(data) {
        return request({
            url: 'cmdb/idc-asset/update',
            method: 'put',
            data
        })
    },
    deleteCmdbIdcAsset(id) {
        return request({
            url: 'cmdb/idc-asset/delete',
            method: 'delete',
            data: { id }
        })
    },
    getCmdbIdcAssetInfo(id) {
        return request({
            url: 'cmdb/idc-asset/info',
            method: 'get',
            params: { id }
        })
    },
    getCmdbIdcAssetList(params = {}) {
        return request({
            url: 'cmdb/idc-asset/list',
            method: 'get',
            params
        })
    },
    getCmdbIdcAssetHardwareMonitor(params = {}) {
        return request({
            url: 'cmdb/idc-asset/hardware-monitor',
            method: 'get',
            params
        })
    },
    updateCmdbIdcAssetPosition(data) {
        return request({
            url: 'cmdb/idc-asset/position',
            method: 'put',
            data
        })
    },
    importCmdbIdcAssetsFromExcel(data, config = {}) {
        return request({
            url: 'cmdb/idc-asset/import',
            method: 'post',
            data,
            headers: {
                'Content-Type': 'multipart/form-data'
            },
            ...config
        })
    },
    downloadCmdbIdcAssetTemplate() {
        return request({
            url: 'cmdb/idc-asset/template',
            method: 'get',
            responseType: 'blob'
        })
    },

    // IDC 虚拟机关联管理
    createCmdbIdcVm(data) {
        return request({
            url: 'cmdb/idc-vm/create',
            method: 'post',
            data
        })
    },
    updateCmdbIdcVm(data) {
        return request({
            url: 'cmdb/idc-vm/update',
            method: 'put',
            data
        })
    },
    deleteCmdbIdcVm(id) {
        return request({
            url: 'cmdb/idc-vm/delete',
            method: 'delete',
            data: { id }
        })
    },
    getCmdbIdcVmInfo(id) {
        return request({
            url: 'cmdb/idc-vm/info',
            method: 'get',
            params: { id }
        })
    },
    getCmdbIdcVmList(params = {}) {
        return request({
            url: 'cmdb/idc-vm/list',
            method: 'get',
            params
        })
    },
    importCmdbIdcVmsFromExcel(data, config = {}) {
        return request({
            url: 'cmdb/idc-vm/import',
            method: 'post',
            data,
            headers: {
                'Content-Type': 'multipart/form-data'
            },
            ...config
        })
    },
    downloadCmdbIdcVmTemplate() {
        return request({
            url: 'cmdb/idc-vm/template',
            method: 'get',
            responseType: 'blob'
        })
    },

    // 鍚屾涓绘満閰嶇疆淇℃伅
    syncHostConfig(id) {
        return request({
            url: 'cmdb/hostsync',
            method: 'post',
            data: {
                id: id
            },
            headers: {
                'Content-Type': 'application/json'
            }
        })
    },

    // Agent绠＄悊
    // 閮ㄧ讲agent鍒版寚瀹氫富鏈?(鏀寔鍗曚釜鎴栨壒閲?
    deployAgent(hostIds, version, options = {}) {
        // 纭繚hostIds鏄暟缁勬牸寮?
        const ids = Array.isArray(hostIds) ? hostIds : [hostIds]
        return request({
            url: 'cmdb/agent/deploy',
            method: 'post',
            data: {
                ...options,
                hostIds: ids,
                version: version
            }
        })
    },
    // 获取 uploadDir 下可选的离线 Agent 制品
    getAgentOfflineAssets() {
        return request({
            url: 'cmdb/agent/offline-assets',
            method: 'get'
        })
    },
    // 上传预编译的离线 Agent 二进制
    uploadAgentOfflineBinary(file, onUploadProgress) {
        const formData = new FormData()
        formData.append('file', file)
        return request({
            url: 'cmdb/agent/offline-assets/upload',
            method: 'post',
            data: formData,
            headers: {
                'Content-Type': 'multipart/form-data'
            },
            onUploadProgress
        })
    },
    // 检查 x86_64 与 arm64 离线 Agent 是否已上传
    checkAgentOfflineAssets() {
        return request({
            url: 'cmdb/agent/offline-assets/check',
            method: 'get'
        })
    },
    // 检查目标主机架构以及对应离线二进制是否可用
    precheckAgentOfflineDeployment(payload) {
        return request({
            url: 'cmdb/agent/offline-assets/precheck',
            method: 'post',
            data: payload
        })
    },
    redeployAgent(hostIds) {
        const ids = Array.isArray(hostIds) ? hostIds : [hostIds]
        return request({
            url: 'cmdb/agent/redeploy',
            method: 'post',
            data: {
                hostIds: ids
            }
        })
    },
    
    // 鍗歌浇鎸囧畾涓绘満鐨刟gent (鏀寔鍗曚釜鎴栨壒閲?
    uninstallAgent(hostIds) {
        // 纭繚hostIds鏄暟缁勬牸寮?
        const ids = Array.isArray(hostIds) ? hostIds : [hostIds]
        return request({
            url: 'cmdb/agent/uninstall',
            method: 'delete',
            data: {
                hostIds: ids
            }
        })
    },
    
    // 鏍规嵁涓绘満id鑾峰彇agent鐘舵€?
    getAgentStatus(id) {
        return request({
            url: `cmdb/agent/status/${id}`,
            method: 'get'
        })
    },
    
    // 閲嶅惎agent
    restartAgent(id) {
        return request({
            url: `cmdb/agent/restart/${id}`,
            method: 'post'
        })
    },
    
    // 鑾峰彇agent鍒楄〃
    getAgentList(params) {
        return request({
            url: 'cmdb/agent/list',
            method: 'get',
            params
        })
    },

    // 获取可部署 Agent 的候选主机
    getAgentCandidates(params = {}) {
        return request({
            url: 'cmdb/agent/candidates',
            method: 'get',
            params
        })
    },

    // 获取存在可部署 Agent 主机的分组
    getAgentCandidateGroups(params = {}) {
        return request({
            url: 'cmdb/agent/candidate-groups',
            method: 'get',
            params
        })
    },

    // 鎼滅储 agent
    queryAgent(params) {
        return request({
            url: 'cmdb/agent/query',
            method: 'get',
            params
        })
    },

    // 根据版本查询 agent
    queryAgentByVersion(version, params = {}) {
        return request({
            url: 'cmdb/agent/query',
            method: 'get',
            params: {
                version,
                ...params
            }
        })
    },

    // 获取 agent 版本列表
    getAgentVersions(params = {}) {
        return request({
            url: 'cmdb/agent/versions',
            method: 'get',
            params
        })
    },

    // 鑾峰彇agent缁熻淇℃伅
    getAgentStatistics(params = {}) {
        return request({
            url: 'cmdb/agent/statistics',
            method: 'get',
            params
        })
    },

    // 鍒犻櫎agent鏁版嵁 (鐢ㄤ簬绂荤嚎鏈嶅姟鍣?
    deleteAgent(id) {
        return request({
            url: `cmdb/agent/delete/${id}`,
            method: 'delete'
        })
    },

    // 鎵归噺鏇存柊涓绘満鍒嗙粍
    batchUpdateHostGroup(hostIds, groupId) {
        return request({
            url: 'cmdb/hostbatchupdategroup',
            method: 'put',
            data: {
                hostIds: hostIds,
                groupId: groupId
            }
        })
    },

    // SNMP璁惧绠＄悊
    // 鑾峰彇SNMP璁惧鍒楄〃
    getSnmpDevices(config) {
        return request({
            url: 'cmdb/snmp-devices',
            method: 'get',
            ...config
        })
    },

    // 鏍规嵁ID鑾峰彇SNMP璁惧璇︽儏
    getSnmpDeviceById(id) {
        return request({
            url: `cmdb/snmp-devices/${id}`,
            method: 'get'
        })
    },

    // 鍒涘缓SNMP璁惧
    createSnmpDevice(data) {
        return request({
            url: 'cmdb/snmp-devices',
            method: 'post',
            data: data
        })
    },

    // 鏇存柊SNMP璁惧
    updateSnmpDevice(id, data) {
        return request({
            url: `cmdb/snmp-devices/${id}`,
            method: 'put',
            data: data
        })
    },

    // 鍒犻櫎SNMP璁惧
    deleteSnmpDevice(id) {
        return request({
            url: `cmdb/snmp-devices/${id}`,
            method: 'delete'
        })
    },

    // SNMP璁惧SSH缁堢杩炴帴
    getSnmpDeviceSSHWebSocketUrl(id) {
        const token = localStorage.getItem('token') || sessionStorage.getItem('token') || ''
        return buildWebSocketUrl(`/api/v1/cmdb/snmp-devices/${id}/ssh/connect?token=${encodeURIComponent(token)}`)
    },

    // SNMP璁惧SSH鎵ц鍛戒护
    executeSnmpDeviceCommand(id, command) {
        return request({
            url: `cmdb/snmp-devices/${id}/ssh/command`,
            method: 'get',
            params: {
                command: command
            }
        })
    },

    // Windows Exporter 绠＄悊
    // 鏌ヨ Windows Exporter 瀹夎鐘舵€?
    getWindowsExporterStatus(deviceId) {
        return request({
            url: `cmdb/windows-exporter/status/${deviceId}`,
            method: 'get'
        })
    },

    // 瀹夎 Windows Exporter
    installWindowsExporter(data) {
        return request({
            url: 'cmdb/windows-exporter/install',
            method: 'post',
            data: data
        })
    },

    precheckWindowsExporter(data) {
        return request({
            url: 'cmdb/windows-exporter/precheck',
            method: 'get',
            params: data,
            timeout: 60000
        })
    },

    // 鍗歌浇 Windows Exporter
    uninstallWindowsExporter(deviceId) {
        return request({
            url: `cmdb/windows-exporter/uninstall/${deviceId}`,
            method: 'delete'
        })
    },

    batchInstallWindowsExporter(data) {
        return request({
            url: 'cmdb/windows-exporter/batch-install',
            method: 'post',
            data: data
        })
    },

    getAllWindowsExporterStatus() {
        return request({
            url: 'cmdb/windows-exporter/status-all',
            method: 'get'
        })
    },

    downloadWindowsExporterInitScript() {
        return request({
            url: 'cmdb/windows-exporter/init-winrm-script',
            method: 'get',
            responseType: 'blob'
        })
    },

    getWindowsExporterInstallLog(deviceId) {
        return request({
            url: `cmdb/windows-exporter/install-log/${deviceId}`,
            method: 'get',
            timeout: 60000
        })
    },

    // 鑾峰彇SNMP璁惧鐩戞帶鏁版嵁
    getSnmpDeviceMonitorData(id) {
        return request({
            url: `cmdb/snmp-devices/${id}/monitor`,
            method: 'get'
        })
    },

    // 鑾峰彇 Windows 璁惧鐩戞帶鏁版嵁锛圥rometheus windows_exporter 鎸囨爣鑱氬悎锛?
    getWindowsDeviceMonitor(id) {
        return request({
            url: `cmdb/windows-devices/${id}/monitor`,
            method: 'get'
        })
    },

    // 鎵归噺鑾峰彇SNMP璁惧璧勬簮浣跨敤鎯呭喌
    getSnmpDevicesBatchResource(deviceIds) {
        return request({
            url: 'cmdb/snmp-devices/batch-resource',
            method: 'get',
            params: {
                deviceIds: deviceIds
            }
        })
    },

    // JumpServer涓绘満鍚屾
    syncHostFromJumpServer(data) {
        return request({
            url: 'cmdb/hostjumpserversync',
            method: 'post',
            data: data
        })
    }
}
