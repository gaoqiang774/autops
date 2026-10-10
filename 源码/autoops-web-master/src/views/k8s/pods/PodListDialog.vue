<template>
  <el-dialog
    :model-value="visible"
    :title="`Pod列表 - ${workload.name || ''}`"
    width="min(1280px, calc(100vw - 32px))"
    class="pod-list-dialog"
    @update:model-value="handleClose"
  >
    <el-table
      :data="workload.pods || []"
      stripe
      style="width: 100%"
      size="small"
      class="pod-list-table"
    >
      <el-table-column prop="name" label="Pod名称" min-width="260">
        <template #default="{ row }">
          <div class="pod-name-container">
            <img src="@/assets/image/k8s.svg" alt="k8s" class="k8s-icon" />
            <span class="pod-name" :title="row.name">{{ row.name }}</span>
          </div>
        </template>
      </el-table-column>

      <el-table-column prop="status" label="状态" width="112">
        <template #default="{ row }">
          <el-tag
            :type="getPodStatusTag(row.status)"
            size="small"
            effect="dark"
          >
            {{ getPodStatusText(row.status) }}
          </el-tag>
        </template>
      </el-table-column>

      <el-table-column prop="restartCount" label="重启次数" width="104">
        <template #default="{ row }">
          <el-tag size="small" :type="row.restartCount > 0 ? 'warning' : 'success'">
            {{ row.restartCount }}
          </el-tag>
        </template>
      </el-table-column>

      <el-table-column prop="nodeName" label="节点" min-width="190" show-overflow-tooltip>
        <template #default="{ row }">
          <span class="node-name" :title="row.nodeName || '-'">{{ row.nodeName || '-' }}</span>
        </template>
      </el-table-column>

      <el-table-column label="IP地址" min-width="170">
        <template #default="{ row }">
          <div class="ip-info">
            <div class="ip-item">
              <span class="ip-label">Pod:</span>
              <span class="ip-value" :title="row.podIP || '-'">{{ row.podIP || '-' }}</span>
            </div>
            <div class="ip-item">
              <span class="ip-label">Host:</span>
              <span class="ip-value" :title="row.hostIP || '-'">{{ row.hostIP || '-' }}</span>
            </div>
          </div>
        </template>
      </el-table-column>

      <el-table-column label="资源" min-width="150">
        <template #default="{ row }">
          <div class="resource-info">
            <div class="resource-item">
              <span class="resource-label">CPU:</span>
              <span class="resource-value">{{ formatCpu(row.resources?.requests?.cpu) }}</span>
            </div>
            <div class="resource-item">
              <span class="resource-label">Memory:</span>
              <span class="resource-value">{{ formatMemory(row.resources?.requests?.memory) }}</span>
            </div>
          </div>
        </template>
      </el-table-column>

      <el-table-column prop="age" label="运行时间" width="132">
        <template #default="{ row }">
          <div class="time-info">
            <el-tag size="small" type="info">{{ row.age }}</el-tag>
            <div v-if="row.runningTime" class="running-time">
              运行: {{ formatRunningTime(row.runningTime) }}
            </div>
          </div>
        </template>
      </el-table-column>

      <el-table-column label="操作" width="168" fixed="right">
        <template #default="{ row }">
          <div class="operation-buttons">
            <el-button link size="small" class="operation-text detail" @click="$emit('view-logs', row)">日志</el-button>
            <el-button link size="small" class="operation-text yaml" @click="$emit('view-yaml', row)">YAML</el-button>
            <el-button link size="small" class="operation-text rebuild"  @click="$emit('rebuild-pod', row)">重构</el-button>
            <el-button link size="small" class="operation-text events" @click="$emit('view-events', row)">事件</el-button>
          </div>
        </template>
      </el-table-column>
    </el-table>
  </el-dialog>
</template>

<script setup>
import { defineProps, defineEmits } from 'vue'

defineProps({
  visible: {
    type: Boolean,
    default: false
  },
  workload: {
    type: Object,
    default: () => ({})
  }
})

const emit = defineEmits(['update:visible', 'close', 'view-logs', 'view-yaml', 'rebuild-pod', 'view-events'])

const handleClose = () => {
  emit('update:visible', false)
  emit('close')
}

// 工具函数
const formatCpu = (cpuStr) => {
  if (!cpuStr || cpuStr === '0' || cpuStr === '') return '-'
  return cpuStr
}

const formatMemory = (memoryStr) => {
  if (!memoryStr || memoryStr === '0' || memoryStr === '') return '-'

  if (memoryStr.endsWith('Ki')) {
    const kb = parseInt(memoryStr.replace('Ki', ''))
    if (kb < 1024) return memoryStr
    const mb = (kb / 1024).toFixed(1)
    return `${mb}Mi`
  }

  if (memoryStr.endsWith('Mi')) {
    const mi = parseInt(memoryStr.replace('Mi', ''))
    if (mi < 1024) return memoryStr
    const gi = (mi / 1024).toFixed(1)
    return `${gi}Gi`
  }

  if (memoryStr.endsWith('Gi')) {
    return memoryStr
  }

  return memoryStr
}

const formatRunningTime = (runningTimeStr) => {
  if (!runningTimeStr) return '-'

  // 解析 "30h22m1.563771s" 格式
  const match = runningTimeStr.match(/^(\d+h)?(\d+m)?(\d+(?:\.\d+)?s)?$/)
  if (!match) return runningTimeStr

  const [, hours, minutes, seconds] = match
  let result = ''

  if (hours) result += hours
  if (minutes) result += minutes
  if (seconds && !hours && !minutes) {
    const sec = parseFloat(seconds.replace('s', ''))
    result += `${Math.round(sec)}s`
  }

  return result || runningTimeStr
}

const getPodStatusTag = (status) => {
  const tagMap = {
    'Running': 'success',
    'Pending': 'warning',
    'Failed': 'danger',
    'Succeeded': 'success',
    'Unknown': 'info'
  }
  return tagMap[status] || 'info'
}

const getPodStatusText = (status) => {
  const textMap = {
    'Running': '运行中',
    'Pending': '等待中',
    'Failed': '失败',
    'Succeeded': '成功',
    'Unknown': '未知'
  }
  return textMap[status] || status
}
</script>

<style scoped>
.pod-name-container {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.k8s-icon {
  width: 16px;
  height: 16px;
}

.pod-name {
  display: block;
  min-width: 0;
  max-width: 100%;
  font-weight: 500;
  color: #6b9bd1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-name {
  display: block;
  max-width: 100%;
  color: #606266;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ip-info {
  display: flex;
  flex-direction: column;
  gap: 3px;
  font-size: 12px;
}

.ip-item {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}

.ip-label {
  font-weight: 500;
  color: #666;
  min-width: 30px;
}

.ip-value {
  display: block;
  min-width: 0;
  color: #409eff;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.resource-info {
  display: flex;
  flex-direction: column;
  gap: 3px;
  font-size: 12px;
}

.resource-item {
  display: flex;
  align-items: center;
  gap: 4px;
}

.resource-label {
  font-weight: 500;
  color: #666;
  min-width: 45px;
}

.resource-value {
  color: #67c23a;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
}

.time-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.running-time {
  font-size: 11px;
  color: #909399;
}

.operation-buttons {
  display: flex;
  gap: 8px;
  justify-content: flex-start;
  align-items: center;
  flex-wrap: nowrap;
}

.operation-buttons :deep(.el-button) {
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
  transition: color 0.2s ease, opacity 0.2s ease;
}

.operation-buttons :deep(.el-button:hover:not(.is-disabled)) {
  transform: none;
  box-shadow: none;
  opacity: 0.82;
}

.operation-buttons :deep(.operation-text.detail) {
  color: #4c8bf5;
}

.operation-buttons :deep(.operation-text.yaml),
.operation-buttons :deep(.operation-text.rebuild),
.operation-buttons :deep(.operation-text.events) {
  color: #f3a33a;
}

.pod-list-dialog :deep(.el-dialog) {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 24px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
  box-shadow: 0 36px 110px rgba(15, 23, 42, 0.11);
  backdrop-filter: blur(18px);
}

.pod-list-dialog :deep(.el-dialog__header) {
  padding: 16px 20px 14px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.84);
}

.pod-list-dialog :deep(.el-dialog__title) {
  color: #172133;
  font-size: 17px;
  font-weight: 700;
}

.pod-list-dialog :deep(.el-dialog__body) {
  padding: 18px 20px;
  background:
    radial-gradient(circle at top left, rgba(76, 139, 245, 0.04), transparent 22%),
    linear-gradient(180deg, #f8fafc 0%, #f5f8fc 100%);
}

.pod-list-dialog :deep(.el-table) {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.88);
  border-radius: 16px;
}

.pod-list-dialog :deep(.el-table__cell .cell) {
  min-width: 0;
  overflow: hidden;
  padding-left: 8px;
  padding-right: 8px;
}

.pod-list-table {
  min-width: 1260px;
}

.pod-list-dialog :deep(.el-table__header th) {
  background: rgba(248, 250, 255, 0.98) !important;
  color: #8d97a8;
  font-weight: 700;
}

.pod-list-dialog :deep(.el-table__body tr:hover > td) {
  background: rgba(76, 139, 245, 0.045) !important;
}
</style>
