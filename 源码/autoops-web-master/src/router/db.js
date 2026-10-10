import database from '@/views/cmdb/Cmdb_DB_management.vue'
import review from   '@/views/db/Database_Review.vue'
import crud from '@/views/db/Database_Crud.vue'
import databaseAi from '@/views/db/Database_Ai.vue'
import backup from '@/views/db/Database_Backup.vue'
import monitoring from '@/views/db/Database_Monitoring.vue'
import slowlog from '@/views/db/Database_slowlog.vue'
const routes = [
    {
        path: '/db/management',
        component: database,
        meta: {sTitle: '数据库管理', tTitle: '数据资产管理'}
    },
    {
        path: '/db/review',
        component: review,
        meta: {sTitle: '数据库管理', tTitle: 'SQL审核'}
    },
    {
        path: '/db/crud',
        component: crud,
        meta: {sTitle: '数据库管理', tTitle: '数据操作'}
    },
    {
        path: '/db/operation_log',
        component: databaseAi,
        meta: {sTitle: '数据库管理', tTitle: '数据库AI助手'}
    }
    ,
    {
        path: '/db/backup',
        component: backup,
        meta: {sTitle: '数据库管理', tTitle: '数据备份'}
    }
    ,
    {
        path: '/db/monitoring',
        component: monitoring,
        meta: {sTitle: '数据库管理', tTitle: '数据库监控'}
    }
    ,
    {
        path: '/db/slowlog',
        component: slowlog,
        meta: {sTitle: '数据库管理', tTitle: '数据库巡检'}
    }
]

export default routes
