import LoginLog from '@/views/audit/LoginLog.vue'
import Operator from '@/views/audit/Operator.vue'
import Recording from '@/views/audit/recording.vue'

const routes = [
    {
        path: '/audit/loginlog',
        component: LoginLog,
        meta: { sTitle: '审计中心', tTitle: '登录日志' }
    },
    {
        path: '/audit/operator',
        component: Operator,
        meta: { sTitle: '审计中心', tTitle: '操作日志' }
    },
    {
        path: '/audit/video',
        component: Recording,
        meta: { sTitle: '审计中心', tTitle: '会话日志' }
    }
]

export default routes
