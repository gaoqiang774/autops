import Host from '@/views/cmdb/cmdbHost.vue'
import Switch from '@/views/cmdb/Cmdb_Switch.vue'
import SSH from '@/views/cmdb/Host/SSH.vue'
import HostDetail from '@/views/cmdb/Host/HostDetail.vue'
import WindowsHostDetail from '@/views/cmdb/Host/WindowsHostDetail.vue'
import NetworkSSH from '@/views/cmdb/Network/NetworkSSH.vue'
import NetworkMonitor from '@/views/cmdb/Network/NetworkMonitor.vue'
import Credentials from '@/views/cmdb/Cmdb_Credentials.vue'
import Agent from '@/views/cmdb/Cmdb_Agent.vue'
import Idc from '@/views/cmdb/Cmdb_Idc.vue'
import Dashboard from '@/views/dashboard/Dashboard.vue'
import RDP from '@/views/rdp/index.vue'
import Ai_Cmdb_Asset from '@/views/cmdb/Ai_Cmdb_Asset.vue'
import Cmdb_DB_management from '@/views/cmdb/Cmdb_DB_management.vue'
import Cmdb_Service_model from '@/views/cmdb/Cmdb_Service_model.vue'
import Cmdb_Service_model_detail from '@/views/cmdb/Model/Cmdb_Service_model_detail.vue'
import Cmdb_Service_model_topology from '@/views/cmdb/Model/Cmdb_Service_model_topology.vue'
import Cmdb_Service_model_log_analysis from '@/views/cmdb/Model/Cmdb_Service_model_log_analysis.vue'
import Cmdb_Service_model_monitor_alerts from '@/views/cmdb/Model/Cmdb_Service_model_monitor_alerts.vue'
const routes = [
    {
        path: '/cmdb/ecs',
        component: Host,
        meta: { sTitle: '资产管理', tTitle: '主机管理' }
    },
    {
        path: '/cmdb/switch',
        component: Switch,
        meta: { sTitle: '资产管理', tTitle: '网络设备' }
    },
    {
        path: '/cmdb/switch/:id',
        component: NetworkMonitor,
        meta: { sTitle: '资产管理', tTitle: '交换机监控' }
    },
    {
        path: '/cmdb/ssh',
        component: SSH,
        meta: { sTitle: '资产管理', tTitle: '终端登录' }
    },
    {
        path: '/cmdb/host/detail/:id',
        component: HostDetail,
        meta: { sTitle: '资产管理', tTitle: '主机详情' }
    },
    {
        path: '/cmdb/host/windows-detail/:id',
        component: WindowsHostDetail,
        meta: { sTitle: '资产管理', tTitle: 'Windows主机详情' }
    },
    {
        path: '/cmdb/network/ssh-terminal',
        component: NetworkSSH,
        meta: { sTitle: '资产管理', tTitle: '网络设备终端' }
    },
    {
        path: '/cmdb/credentials',
        component: Credentials,
        meta: { sTitle: '资产管理', tTitle: '凭据管理' }
    },
    {
        path: '/cmdb/agent',
        component: Agent,
        meta: { sTitle: '资产管理', tTitle: '探针管理' }
    },
    {
        path: '/cmdb/idc',
        component: Idc,
        meta: { sTitle: '资产管理', tTitle: '机房管理' }
    },
    {
        path: '/cmdb/db-management',
        component: Cmdb_DB_management,
        meta: { sTitle: '资产管理', tTitle: '数据库管理' }
    },
    {
        path: '/cmdb/dashboard',
        component: Dashboard,
        meta: { sTitle: '资产管理', tTitle: '仪表盘' }
    },
    {
        path: '/cmdb/service_model',
        component: Cmdb_Service_model,
        meta: { sTitle: '资产管理', tTitle: '业务模型' }
    },
    {
        path: '/cmdb/service_model/:id',
        component: Cmdb_Service_model_detail,
        meta: { sTitle: '资产管理', tTitle: '业务详情' }
    },
    {
        path: '/cmdb/service_model/:id/topology',
        component: Cmdb_Service_model_topology,
        meta: { sTitle: '资产管理', tTitle: '业务拓扑' }
    },
    {
        path: '/cmdb/service_model/:id/log-analysis',
        component: Cmdb_Service_model_log_analysis,
        meta: { sTitle: '资产管理', tTitle: '日志分析' }
    },
    {
        path: '/cmdb/service_model/:id/monitor-alerts',
        component: Cmdb_Service_model_monitor_alerts,
        meta: { sTitle: '资产管理', tTitle: '监控告警' }
    },
    {
        path: '/cmdb/assistant',
        component: Ai_Cmdb_Asset,
        meta: { sTitle: '智能运维', tTitle: '智能助手' }
    },
    { path: '/rdp', component: RDP, meta: { public: true }
    
 }
]

export default routes
