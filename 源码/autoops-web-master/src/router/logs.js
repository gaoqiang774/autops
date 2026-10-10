import Log_project from '@/views/logtail/Log_project.vue'
import Log_query from '@/views/logtail/Log_query.vue'
import Log_Agent from '@/views/logtail/Log_Agent.vue'
import Log_Alert from '@/views/logtail/Log_Alert.vue'
import Log_link from '@/views/logtail/Log_link.vue'
import Log_nginx from '@/views/logtail/Log_nginx.vue'
import Log_mysql from '@/views/logtail/Log_mysql.vue'
const routes = [
    {
        path: '/logs/manage',
        component: Log_project,
        meta: { sTitle: '日志中心', tTitle: '日志库管理' }
    },
    {
        path: '/logs/query',
        component: Log_query,
        meta: { sTitle: '日志中心', tTitle: '日志查询' }
    },
    {
        path: '/logs/query/mysql-slow',
        component: Log_mysql,
        meta: { sTitle: '日志中心', tTitle: '数据库慢日志' }
    },
    {
        path: '/logs/collect',
        component: Log_Agent,
        meta: { sTitle: '日志中心', tTitle: '日志采集' }
    },
    {
        path: '/logs/alert',
        component: Log_Alert,
        meta: { sTitle: '日志中心', tTitle: '日志告警' }
    },
    {
        path: '/logs/link',
        component: Log_link,
        meta: { sTitle: '日志中心', tTitle: '日志链路' }
    },
    {
        path: '/logs/project',
        redirect: '/logs/manage'
    },
    {
        path: '/logs/nginx',
        component: Log_nginx,
        meta: { sTitle: '日志中心', tTitle: '业务访问日志' }
    },
]

export default routes
