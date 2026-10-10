import event from '@/views/alert/Alert_Event.vue'
import notice from '@/views/alert/Alert_Notice.vue'
import rule from '@/views/alert/Alert_Rule.vue'
import template from '@/views/alert/Alert_Template.vue'
import view from '@/views/alert/Alert_View.vue'
import source from '@/views/alert/Alert_Source.vue'
import domain from '@/views/alert/Alert_Domain.vue'



const routes = [
    {
        path: '/alert/view',
        component: view,
        meta: {sTitle: '告警中心', tTitle: '告警总览'}
    },
     {
        path: '/alert/event',
        component: event,
        meta: {sTitle: '告警中心', tTitle: '告警事件'}
    },
    {
        path: '/alert/rule',
        component: rule,
        meta: {sTitle: '告警中心', tTitle: '告警管理'}
    },
    {
        path: '/alert/template',
        component: template,
        meta: {sTitle: '告警中心', tTitle: '告警模板'}
    },
        {
        path: '/alert/notice',
        component: notice,
        meta: {sTitle: '告警中心', tTitle: '告警通知'}
    },
    {
        path: '/alert/source',
        component: source,
        meta: {sTitle: '告警中心', tTitle: '告警数据源'}
    },
    {
        path: '/alert/domain',
        component: domain,
        meta: {sTitle: '告警中心', tTitle: '域名告警'}
    }
]

export default routes
