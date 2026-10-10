import accountauth from '@/views/configcenter/Universal_Account.vue'
import KeyManage from '@/views/configcenter/Cloud_Account.vue'
import Credentials from '@/views/cmdb/Cmdb_Credentials.vue'
const routes = [
    {
        path: '/config/ecskey',
        component: Credentials,
        meta: {sTitle: '配置中心', tTitle: '主机凭据'}
    },
    {
        path: '/config/accountauth',
        component: accountauth,
        meta: {sTitle: '配置中心', tTitle: '通用凭据'}
    },
    {
        path: '/config/keymanage',
        component: KeyManage,
        meta: {sTitle: '配置中心', tTitle: '密钥管理'}
    }
]

export default routes
