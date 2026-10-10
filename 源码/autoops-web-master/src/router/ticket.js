import ticket from '@/views/ticket/ticket.vue'
import TicketWorkbench from '@/views/ticket/Ticket_Workbench.vue'
import TicketDaily from '@/views/ticket/Ticket_Daily.vue'
import TicketRelease from '@/views/ticket/Ticket_Release.vue'
import TicketWorkflow from '@/views/ticket/Ticket_Workflow.vue'

const routes = [
    {
        path: '/ticket',
        component: ticket,
        redirect: '/ticket/management',
        meta: { sTitle: '工单中心', tTitle: '工单管理' },
        children: [
            {
                path: 'management',
                component: TicketWorkbench,
                meta: { sTitle: '工单中心', tTitle: '审批工作台' }
            },
            {
                path: 'daily',
                component: TicketDaily,
                meta: { sTitle: '工单中心', tTitle: '日常事务' }
            },
            {
                path: 'release',
                component: TicketRelease,
                meta: { sTitle: '工单中心', tTitle: '应用发布' }
            },
            {
                path: 'workflow',
                component: TicketWorkflow,
                meta: { sTitle: '工单中心', tTitle: '审批流配置' }
            }
        ]
    }
]

export default routes
