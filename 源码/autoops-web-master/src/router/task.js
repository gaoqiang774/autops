import TaskCenter from '@/views/task/TaskCenter.vue'

const TaskSchedule = () => import('@/views/task/TaskSchedule.vue')
const TaskTemplate = () => import('@/views/task/TaskTemplate.vue')
const TaskAnsible = () => import('@/views/task/TaskAnsible.vue')
const TaskMonitor = () => import('@/views/task/TaskMonitor.vue')

const routes = [
  {
    path: '/task',
    component: TaskCenter,
    redirect: '/task/schedule',
    meta: { sTitle: '服务作业', tTitle: '任务调度' },
    children: [
      { path: 'schedule', component: TaskSchedule, meta: { sTitle: '任务中心', tTitle: '任务调度' } },
      { path: 'templates', component: TaskTemplate, meta: { sTitle: '任务中心', tTitle: '模板库' } },
      { path: 'ansible', component: TaskAnsible, meta: { sTitle: '任务中心', tTitle: 'Ansible任务' } },
      { path: 'monitor', component: TaskMonitor, meta: { sTitle: '任务中心', tTitle: '运行监控' } }
    ]
  },
  { path: '/task/list', redirect: '/task/schedule' },
  { path: '/task/template', redirect: '/task/templates' }
]

export default routes
