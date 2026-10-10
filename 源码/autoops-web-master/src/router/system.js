import Personal from '@/views/system/Personal.vue'
import Admin from '@/views/system/Admin.vue'
import Role from '@/views/system/Role.vue'
import Dept from '@/views/system/user/Dept.vue'
import Post from '@/views/system/HostGroup.vue'
import Menu from '@/views/system/Menu.vue'
import HostAuthorization from '@/views/system/HostAuthorization.vue'
import Dashboard from '@/views/dashboard/systemd-dashboard.vue'
import NAV from '@/views/system/system-nav.vue'
import License from '@/views/system/License.vue'
const routes = [
    {
        path: '/system/personal',
        component: Personal,
        meta: { sTitle: '个人中心', tTitle: '个人信息' }
    },
    {
        path: '/system/admin',
        component: Admin,
        meta: { sTitle: '基础管理', tTitle: '用户信息' }
    },
    {
        path: '/system/role',
        component: Role,
        meta: { sTitle: '基础管理', tTitle: '角色信息' }
    },
    {
        path: '/system/menu',
        component: Menu,
        meta: { sTitle: '基础管理', tTitle: '菜单信息' }
    },
    {
        path: '/system/dept',
        component: Dept,
        meta: { sTitle: '基础管理', tTitle: '部门信息' }
    },
    {
        path: '/system/post',
        component: Post,
        meta: { sTitle: '基础管理', tTitle: '用户组管理' }
    },
    {
        path: '/bastion/authorization',
        name: 'BastionAuthorization',
        component: HostAuthorization,
        meta: { sTitle: '基础管理', tTitle: '资产授权' }
    },
    {
            path: '/system/dashboard',
            component: Dashboard,
            meta: { sTitle: '基础管理', tTitle: '系统视图' }
    },
    {
            path: '/system/nav',
            component: NAV,
            meta: { sTitle: '基础管理', tTitle: '菜单导航' }
    },
    {
        path: '/system/license',
        component: License,
        meta: { sTitle: '基础管理', tTitle: '授权管理' }
    },
    

]

export default routes
