import Bill_Dashboard from '@/views/cost/Bill_Dashboard.vue'
import Bill_Details from '@/views/cost/Bill_Details.vue'
import Bill_Center from '@/views/cost/Bill_Center.vue'

const routes = [
  {
    path: '/cost/bill_dashboard',
    component: Bill_Dashboard,
    meta: { sTitle: '费用中心', tTitle: '账单总览' }
  },
  {
    path: '/cost/cloud_bill',
    component: Bill_Center,
    meta: { sTitle: '费用中心', tTitle: '账单中心' }
  },
  {
    path: '/cost/cloud_view',
    component: Bill_Details,
    meta: { sTitle: '费用中心', tTitle: '账单明细' }
  }
]

export default routes
