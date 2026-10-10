import { createRouter, createWebHistory } from 'vue-router'
import Login from '@/views/Login.vue'
import Home from '@/views/Home.vue'
import Dashboard from '@/views/dashboard/index.vue'
import DesignLoginDemo from '@/views/demo/DesignLoginDemo.vue'
import systemRoutes from './system'
import cmdbRoutes from './cmdb'
import configRoutes from './config'
import storage from '@/utils/storage'
import Audit from './audit'
import k8s from './k8s'
import aiops from './aiops'
import cost from './cost'
import db from './db'
import alert from './alert'
import logs from './logs'
import ticket from './ticket'
import work from './work'
import task from './task'
const routes = [
    { path: '/', redirect: '/login' },
    { path: '/login', component: Login, meta: { public: true } },
    { path: '/design-login-demo', component: DesignLoginDemo, meta: { public: true } },
    {
        path: '/home',
        component: Home,
        redirect: '/dashboard',
        children: [
            {
                path: '/dashboard',
                component: Dashboard,
                meta: { tTitle: '仪表盘' }
            },
            ...systemRoutes,
            ...cmdbRoutes,
            ...configRoutes,
            ...Audit,
            ...k8s,
            ...aiops,
            ...cost,
            ...db,
            ...alert,
            ...logs,
            ...ticket,
            ...work,
            ...task,
        ]
    }
]

const router = createRouter({
    history: createWebHistory(process.env.BASE_URL),
    routes
})

router.beforeEach((to, from, next) => {
    const token = storage.getItem('token')
    const sysAdmin = storage.getItem('sysAdmin')
    const isPublicRoute = to.matched.some(record => record.meta && record.meta.public)

    if (to.path === '/login' && token && sysAdmin) {
        return next('/dashboard')
    }

    if (!isPublicRoute && (!token || !sysAdmin)) {
        return next('/login')
    }

    next()
})

export default router
