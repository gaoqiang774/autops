import Ai_Asset from '@/views/aiops/Ai_Asset.vue'
import Ai_Thinktank from '@/views/aiops/Ai_Thinktank.vue'
import Ai_Model from '@/views/aiops/Ai_Model.vue'
import Logs_Analysis from '@/views/aiops/Ai_Logs_Analysis.vue'
import Ai_Mcp_Management from '@/views/aiops/Ai_Mcp_Management.vue'
const routes = [
    {
        path: '/ai/aiasset',
        component: Ai_Asset,
        meta: { sTitle: '智能运维', tTitle: '智能助手' }
    },
    {
        path: '/ai/thinktank',
        component: Ai_Thinktank,
        meta: { sTitle: '智能运维', tTitle: '运维智库' }
    },
    {
        path: '/ai/model',
        component: Ai_Model,
        meta: { sTitle: '智能运维', tTitle: '运维模型' }
    },
    {
        path: '/ai/logs_analysis',
        component: Logs_Analysis,
        meta: { sTitle: '智能运维', tTitle: '日志分析' }
    },
    {
        path: '/ai/mcp-management',
        component: Ai_Mcp_Management,
        meta: { sTitle: '智能运维', tTitle: 'MCP 管理' }
    },
]

export default routes
