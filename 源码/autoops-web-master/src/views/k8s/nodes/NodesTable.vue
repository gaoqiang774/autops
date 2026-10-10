<template>
  <!-- 节点列表表格 -->
  <div class="table-section">
    <el-table
      :data="tableData"
      v-loading="loading"
      stripe
      style="width: 100%"
      class="nodes-table"
      :header-cell-style="{ textAlign: 'left' }"
      :cell-style="{ textAlign: 'left' }"
    >
      <el-table-column prop="nodeName" label="节点名称" min-width="180">
        <template #default="{ row }">
          <div class="node-name-container">
            <img src="@/assets/image/k8s.svg" alt="k8s" class="k8s-icon" />
            <el-button
              type="primary"
              link
              @click="navigateToNodeDetail(row)"
              class="node-name-link"
            >
              {{ row.nodeName }}
            </el-button>
          </div>
        </template>
      </el-table-column>
      
      <el-table-column prop="status" label="状态" min-width="100">
        <template #default="{ row }">
          <el-tag
            :type="getStatusTag(row.status)"
            size="small"
            effect="dark"
          >
            {{ getStatusText(row.status) }}
          </el-tag>
        </template>
      </el-table-column>
      
      <el-table-column prop="role" label="角色" min-width="100">
        <template #default="{ row }">
          <el-tag
            :type="getRoleTag(row.role)"
            size="small"
            effect="plain"
          >
            {{ getRoleText(row.role) }}
          </el-tag>
        </template>
      </el-table-column>

      <el-table-column prop="internalIP" label="节点IP" min-width="140">
        <template #default="{ row }">
          <span class="version-text">{{ row.internalIP || row.externalIP || '--' }}</span>
        </template>
      </el-table-column>
      
      <el-table-column prop="version" label="Kubelet版本" min-width="120">
        <template #default="{ row }">
          <span class="version-text">{{ row.version }}</span>
        </template>
      </el-table-column>
      
      <el-table-column prop="age" label="运行时间" min-width="100">
        <template #default="{ row }">
          <el-tag size="small" type="info">{{ row.age }}</el-tag>
        </template>
      </el-table-column>
      
      <el-table-column label="CPU/内存" min-width="220">
        <template #default="{ row }">
          <div class="resource-usage">
            <div
              v-for="item in getResourceUsageItems(row)"
              :key="item.key"
              class="usage-row"
            >
              <span class="usage-label">{{ item.label }}</span>
              <el-progress
                class="usage-progress"
                :percentage="item.percent"
                :color="getUsageColor(item.percent)"
                :stroke-width="6"
                :show-text="false"
              />
              <el-tooltip
                :content="item.tooltip"
                placement="top"
              >
                <span
                  class="usage-value"
                  :class="{ 'usage-value-muted': !item.available }"
                >
                  {{ item.text }}
                </span>
              </el-tooltip>
            </div>
          </div>
        </template>
      </el-table-column>
      
      <el-table-column prop="pods" label="Pod数量" min-width="100">
        <template #default="{ row }">
          <el-tag size="small" type="warning">{{ row.pods }}</el-tag>
        </template>
      </el-table-column>
      
      <el-table-column label="调度状态" min-width="100">
        <template #default="{ row }">
          <el-tag
            :type="row.schedulable ? 'success' : 'danger'"
            size="small"
            :icon="row.schedulable ? CircleCheck : Lock"
          >
            {{ row.schedulable ? '可调度' : '已封锁' }}
          </el-tag>
        </template>
      </el-table-column>
      
      <el-table-column label="操作" width="160" fixed="right">
        <template #default="{ row }">
          <div class="operation-buttons">
            <button
              type="button"
              class="operation-text detail"
              @click="navigateToNodeDetail(row)"
            >详情</button>

            <el-dropdown @command="(cmd) => handleMoreCommand(row, cmd)" trigger="click">
              <button type="button" class="operation-text more">
                更多
              </button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="yaml">yaml编辑</el-dropdown-item>
                  <el-dropdown-item command="drain" :disabled="!row.schedulable">排空节点</el-dropdown-item>
                  <el-dropdown-item command="labels">标签管理</el-dropdown-item>
                  <el-dropdown-item command="taints">污点管理</el-dropdown-item>
                  <el-dropdown-item command="schedule" divided>
                    {{ row.schedulable ? '封锁节点' : '解封节点' }}
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { useRouter } from 'vue-router'
import {
  CircleCheck,
  Lock,
} from '@element-plus/icons-vue'

// Props
const props = defineProps({
  tableData: {
    type: Array,
    default: () => []
  },
  loading: {
    type: Boolean,
    default: false
  },
  selectedClusterId: {
    type: [String, Number],
    required: true
  }
})

// Emits
const emit = defineEmits([
  'manageTaints',
  'manageLabels',
  'toggleCordon',
  'drainNode',
  'editYaml'
])

const router = useRouter()

// 导航到节点详情页面
const navigateToNodeDetail = (row) => {
  router.push(`/k8s/cluster/${props.selectedClusterId}/node/${row.nodeName}`)
}

// 获取状态标签样式
const getStatusTag = (status) => {
  switch (status) {
    case 'Ready':
      return 'success'
    case 'NotReady':
      return 'danger'
    case 'SchedulingDisabled':
      return 'warning'
    default:
      return 'info'
  }
}

// 获取状态文本
const getStatusText = (status) => {
  switch (status) {
    case 'Ready':
      return '就绪'
    case 'NotReady':
      return '未就绪'
    case 'SchedulingDisabled':
      return '调度禁用'
    default:
      return '未知'
  }
}

// 获取角色标签样式
const getRoleTag = (role) => {
  switch (role) {
    case 'master':
    case 'control-plane':
      return 'danger'
    case 'worker':
      return 'success'
    default:
      return 'info'
  }
}

// 获取角色文本
const getRoleText = (role) => {
  switch (role) {
    case 'master':
    case 'control-plane':
      return 'master'
    case 'worker':
      return 'worker'
    default:
      return '未知'
  }
}

const clampPercent = (value) => {
  const number = Number(value)
  if (!Number.isFinite(number) || number < 0) return 0
  if (number > 100) return 100
  return Math.round(number)
}

const formatPercent = (value) => {
  const number = Number(value)
  if (!Number.isFinite(number)) return '0%'
  return `${clampPercent(number)}%`
}

const buildUsageItem = (resource, key, label) => {
  const usage = resource || {}
  const available = Boolean(usage.metricsAvailable)
  const percent = available ? clampPercent(usage.usageRate) : 0
  const requestText = formatPercent(usage.requestRate)

  return {
    key,
    label,
    percent,
    available,
    text: available ? formatPercent(usage.usageRate) : '暂无',
    tooltip: available
      ? `当前使用 ${usage.used || '--'} / 可分配 ${usage.allocatable || '--'}，请求率 ${requestText}`
      : `未获取到实时指标，可分配 ${usage.allocatable || '--'}，请求率 ${requestText}`
  }
}

const getResourceUsageItems = (row) => [
  buildUsageItem(row.resourceUsage?.cpu, 'cpu', 'CPU'),
  buildUsageItem(row.resourceUsage?.memory, 'memory', '内存')
]

const getUsageColor = (value) => {
  if (value >= 90) return '#ef4444'
  if (value >= 75) return '#f59e0b'
  return '#22c55e'
}

// 事件处理函数
const manageTaints = (row) => {
  emit('manageTaints', row)
}

const manageLabels = (row) => {
  emit('manageLabels', row)
}

const toggleCordon = (row) => {
  emit('toggleCordon', row)
}

const drainNode = (row) => {
  emit('drainNode', row)
}

const handleMoreCommand = (row, command) => {
  switch (command) {
    case 'yaml':     emit('editYaml', row); break
    case 'drain':    drainNode(row); break
    case 'labels':   manageLabels(row); break
    case 'taints':   manageTaints(row); break
    case 'schedule': toggleCordon(row); break
  }
}
</script>

<style scoped>
.table-section {
  flex: 1;
  min-height: 0;
  padding: 0;
  border: 1px solid #e3ebf5;
  border-radius: 10px;
  background: #ffffff;
  overflow: auto;
}

.nodes-table {
  min-width: 990px;
  border: 0;
  border-radius: 9px;
  background: #ffffff;
  overflow: hidden;
  box-shadow: none;
}

.nodes-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.nodes-table :deep(.el-table__header th) {
  height: 38px;
  padding: 6px 0;
  background: #f3f7fc !important;
  border-bottom: 1px solid #e3ebf5;
  text-shadow: none;
}

.nodes-table :deep(.el-table__header th .cell) {
  color: #475569 !important;
  font-size: 12px;
  font-weight: 700 !important;
}

.nodes-table :deep(.el-table__cell) {
  padding: 7px 0;
  border-bottom-color: #eef3f9;
  overflow: visible;
}

.nodes-table :deep(.el-table__cell .cell) {
  overflow: visible;
}

.nodes-table :deep(.el-table__body tr:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

.node-name-container {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
}

.k8s-icon {
  width: 18px;
  height: 18px;
}

.node-name-link {
  min-width: 0;
  padding: 0;
  font-weight: 600;
  color: #2563eb;
  text-decoration: none;
  transition: color 160ms ease;
}

.node-name-link:hover {
  color: #1d4ed8;
  transform: none;
}

.node-name-link :deep(span) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.version-text {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
  color: #53657d;
  font-weight: 500;
}

.resource-usage {
  display: flex;
  flex-direction: column;
  gap: 5px;
  width: 100%;
  max-width: 200px;
}

.usage-row {
  display: grid;
  grid-template-columns: 30px minmax(76px, 1fr) 34px;
  align-items: center;
  gap: 6px;
  min-height: 16px;
}

.usage-label {
  color: #8d97a8;
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
}

.usage-progress {
  min-width: 0;
}

.usage-progress :deep(.el-progress-bar__outer) {
  background-color: #edf2f7;
}

.usage-value {
  color: #172133;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
  text-align: right;
}

.usage-value-muted {
  color: #a4adba;
}

.operation-buttons {
  display: flex;
  gap: 14px;
  flex-wrap: nowrap;
  align-items: center;
  justify-content: flex-end;
}

.operation-buttons :deep(.el-button),
.operation-buttons .operation-text {
  height: auto;
  min-height: 0;
  padding: 0;
  margin-left: 0;
  border: none;
  border-radius: 0;
  background: transparent;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.4;
  box-shadow: none;
  transform: none;
  transition: color 160ms ease, opacity 160ms ease;
  cursor: pointer;
  appearance: none;
}

.operation-buttons :deep(.el-button:hover:not(.is-disabled)),
.operation-buttons .operation-text:hover:not(.is-disabled):not(:disabled) {
  transform: none;
  box-shadow: none;
  text-decoration: none;
  opacity: 0.82;
}

.operation-buttons :deep(.operation-text.detail) {
  color: #4c8bf5;
}

.operation-buttons :deep(.operation-text.more) {
  color: #f3a33a;
}

.nodes-table :deep(.el-tag) {
  border-radius: 999px;
  font-size: 11px;
  font-weight: 500;
}
</style>
