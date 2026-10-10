<template>
  <main class="ticket-page">
    <header class="ticket-page-header">
      <div class="ticket-page-header__icon"><el-icon :size="20"><Tickets /></el-icon></div>
      <div class="ticket-page-header__title"><h1>工单中心</h1><p>事务处理、应用发布、SQL 审核与审批流统一协作</p></div>
      <nav class="ticket-nav" aria-label="工单视图">
        <el-button v-for="item in navItems" :key="item.key" :type="activeView === item.key ? 'primary' : 'default'" :plain="activeView !== item.key" :icon="item.icon" @click="openView(item.key)">{{ item.label }}</el-button>
        <el-button :icon="DataAnalysis" @click="router.push('/db/review')">SQL 审核</el-button>
      </nav>
    </header>
    <section class="ticket-workspace">
      <router-view v-slot="{ Component }">
        <component :is="Component" @open-tab="openView" @open-business="openBusiness" />
      </router-view>
    </section>
  </main>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Connection, DataAnalysis, Promotion, Tickets, Tools } from '@element-plus/icons-vue'
import './ticket.css'

const router = useRouter()
const route = useRoute()
const navItems = [{ key: 'workbench', label: '审批工作台', icon: Tools }, { key: 'daily', label: '日常事务', icon: Tickets }, { key: 'release', label: '应用发布', icon: Promotion }, { key: 'workflow', label: '审批流', icon: Connection }]
const viewPaths = { workbench: '/ticket/management', daily: '/ticket/daily', release: '/ticket/release', workflow: '/ticket/workflow' }
const activeView = computed(() => ({ management: 'workbench', daily: 'daily', release: 'release', workflow: 'workflow' }[String(route.path).split('/').pop()] || 'workbench'))
const openView = view => router.push(viewPaths[view] || viewPaths.workbench)
const openBusiness = row => {
  if (row.businessType === 'sql_review') { router.push('/db/review'); return }
  router.push({ path: row.businessType === 'release' ? viewPaths.release : viewPaths.daily, query: { id: row.businessId } })
}
</script>
