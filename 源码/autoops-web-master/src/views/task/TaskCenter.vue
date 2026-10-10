<template>
  <div class="task-center-page">
    <header class="task-page-header">
      <span class="task-page-header__icon" aria-hidden="true">
        <el-icon><Operation /></el-icon>
      </span>
      <div class="task-page-header__content">
        <h1>任务中心</h1>
        <span class="task-page-header__divider" aria-hidden="true"></span>
        <p>统一管理脚本调度、模板、自动化作业与运行状态</p>
      </div>
      <nav class="task-page-nav" aria-label="任务中心导航">
        <router-link v-for="item in navigation" :key="item.path" :to="item.path">
          <el-icon><component :is="item.icon" /></el-icon>
          <span>{{ item.label }}</span>
        </router-link>
      </nav>
    </header>
    <router-view />
  </div>
</template>

<script setup>
const navigation = [
  { path: '/task/schedule', label: '任务调度', icon: 'Calendar' },
  { path: '/task/templates', label: '模板库', icon: 'Document' },
  { path: '/task/ansible', label: 'Ansible 自动化', icon: 'Connection' },
  { path: '/task/monitor', label: '运行监控', icon: 'DataAnalysis' }
]
</script>

<style src="./task-center.css"></style>

