<template>
  <!-- 命名空间列表表格 -->
  <div class="table-section">
    <el-table
      :data="tableData"
      v-loading="loading"
      stripe
      style="width: 100%"
      class="namespace-table"
    >
      <el-table-column prop="name" label="命名空间名称" min-width="180">
        <template #default="{ row }">
          <div class="namespace-name-container">
            <img src="@/assets/image/k8s.svg" alt="k8s" class="k8s-icon" />
            <el-button
              type="primary"
              link
              @click="viewNamespaceDetail(row)"
              class="namespace-name-link"
            >
              {{ row.name }}
            </el-button>
          </div>
        </template>
      </el-table-column>

      <el-table-column label="资源统计" min-width="160">
        <template #default="{ row }">
          <div class="resource-stats">
            <div class="resource-item">
              <span class="resource-label pod-label">Pod</span>
              <span class="resource-value pod-value">{{ row.resourceCount.pods }}</span>
            </div>
            <div class="resource-item">
              <span class="resource-label svc-label">Service</span>
              <span class="resource-value svc-value">{{ row.resourceCount.services }}</span>
            </div>
            <div class="resource-item">
              <span class="resource-label secret-label">Secret</span>
              <span class="resource-value secret-value">{{ row.resourceCount.secrets }}</span>
            </div>
            <div class="resource-item">
              <span class="resource-label cm-label">ConfigMap</span>
              <span class="resource-value cm-value">{{ row.resourceCount.configMaps }}</span>
            </div>
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

      <el-table-column prop="age" label="创建时间" min-width="100">
        <template #default="{ row }">
          <span class="age-text">{{ row.age }}</span>
        </template>
      </el-table-column>

      <el-table-column label="标签" min-width="100" align="center">
        <template #default="{ row }">
          <div class="label-container">
            <button
              type="button"
              class="label-count-button"
              aria-label="查看命名空间标签"
              @click="viewNamespaceLabels(row)"
            >
              <img src="@/assets/image/标签.svg" alt="" width="14" height="14" />
              <span>{{ getLabelCount(row.labels) }}</span>
            </button>
          </div>
        </template>
      </el-table-column>

      <el-table-column label="配额/限制" min-width="120" align="center">
        <template #default="{ row }">
          <div class="quota-limit-info">
            <span class="quota-limit-item" :class="{ 'has-value': row.resourceQuotas.length > 0 }">
              配额 <strong>{{ row.resourceQuotas.length }}</strong>
            </span>
            <span class="quota-limit-divider" aria-hidden="true"></span>
            <span class="quota-limit-item" :class="{ 'has-value': row.limitRanges.length > 0 }">
              限制 <strong>{{ row.limitRanges.length }}</strong>
            </span>
          </div>
        </template>
      </el-table-column>
      
      <el-table-column label="操作" width="190" fixed="right">
        <template #default="{ row }">
          <div class="operation-buttons">
            <button
              type="button"
              class="operation-text detail"
              @click="viewNamespaceDetail(row)"
            >详情</button>

            <el-dropdown @command="(cmd) => handleMoreCommand(row, cmd)" trigger="click">
              <button type="button" class="operation-text more">
                更多
              </button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="yaml">yaml编辑</el-dropdown-item>
                  <el-dropdown-item command="quota">配额管理</el-dropdown-item>
                  <el-dropdown-item command="limit">资源限制</el-dropdown-item>
                  <el-dropdown-item command="labels">修改标签</el-dropdown-item>
                  <el-dropdown-item
                    command="delete"
                    divided
                    :disabled="isSystemNamespace(row.name)"
                  >删除空间</el-dropdown-item>
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
// Props
const props = defineProps({
  tableData: {
    type: Array,
    default: () => []
  },
  loading: {
    type: Boolean,
    default: false
  }
})

// Emits
const emit = defineEmits([
  'viewNamespaceDetail',
  'viewNamespaceLabels',
  'manageResourceQuotas',
  'manageLimitRanges',
  'deleteNamespace',
  'editYaml',
  'manageLabels'
])

// 获取状态标签样式
const getStatusTag = (status) => {
  const tagMap = {
    'Active': 'success',
    'Terminating': 'warning',
    'Unknown': 'info'
  }
  return tagMap[status] || 'info'
}

// 获取状态文本
const getStatusText = (status) => {
  const textMap = {
    'Active': '活跃',
    'Terminating': '终止中',
    'Unknown': '未知'
  }
  return textMap[status] || '未知'
}

// 获取标签数量（过滤系统标签）
const getLabelCount = (labels) => {
  if (!labels) return 0
  
  const systemLabelPrefixes = [
    'kubernetes.io/',
    'beta.kubernetes.io/',
    'app.kubernetes.io/'
  ]
  
  return Object.keys(labels).filter(key => 
    !systemLabelPrefixes.some(prefix => key.startsWith(prefix))
  ).length
}

// 判断是否为系统命名空间
const isSystemNamespace = (namespaceName) => {
  const systemNamespaces = [
    'default',
    'kube-system', 
    'kube-public',
    'kube-node-lease'
  ]
  return systemNamespaces.includes(namespaceName)
}

// 事件处理函数
const viewNamespaceDetail = (row) => {
  emit('viewNamespaceDetail', row)
}

const viewNamespaceLabels = (row) => {
  emit('viewNamespaceLabels', row)
}

const manageResourceQuotas = (row) => {
  emit('manageResourceQuotas', row)
}

const manageLimitRanges = (row) => {
  emit('manageLimitRanges', row)
}

const deleteNamespace = (row) => {
  emit('deleteNamespace', row)
}

const handleMoreCommand = (row, command) => {
  switch (command) {
    case 'yaml':   emit('editYaml', row); break
    case 'quota':  manageResourceQuotas(row); break
    case 'limit':  manageLimitRanges(row); break
    case 'labels': emit('manageLabels', row); break
    case 'delete': deleteNamespace(row); break
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

.namespace-table {
  min-width: 1120px;
  border: 0;
  border-radius: 9px;
  background: #ffffff;
  overflow: hidden;
  box-shadow: none;
}

.namespace-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.namespace-table :deep(.el-table__header th) {
  height: 38px;
  padding: 6px 0;
  background: #f3f7fc !important;
  border-bottom: 1px solid #e3ebf5;
  text-shadow: none;
}

.namespace-table :deep(.el-table__header th .cell) {
  color: #475569 !important;
  font-size: 12px;
  font-weight: 700 !important;
  text-shadow: none;
}

.namespace-table :deep(.el-table__cell) {
  padding: 7px 0;
  border-bottom-color: #eef3f9;
  overflow: visible;
}

.namespace-table :deep(.el-table__cell .cell) {
  overflow: visible;
}

.namespace-table :deep(.el-table__body tr:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

.namespace-name-container {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
}

.k8s-icon {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}

.namespace-name-link {
  min-width: 0;
  padding: 0;
  font-weight: 600;
  color: #2563eb;
  text-decoration: none;
  transition: color 160ms ease;
}

.namespace-name-link:hover {
  color: #1d4ed8;
  transform: none;
}

.namespace-name-link :deep(span) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.resource-stats {
  display: grid;
  grid-template-columns: repeat(2, minmax(72px, 1fr));
  gap: 4px 12px;
  max-width: 210px;
}

.resource-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  min-width: 0;
}

.resource-label {
  min-width: 48px;
  padding: 0;
  color: #64748b;
  font-size: 11px;
  font-weight: 500;
  white-space: nowrap;
}
.resource-value {
  color: #334155;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 11px;
  font-weight: 700;
}
.age-text {
  color: #64748b;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 11px;
  font-weight: 600;
}

.quota-limit-info {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 28px;
}

.label-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 28px;
}

.label-count-button {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-width: 38px;
  height: 28px;
  padding: 0 6px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: #53657d;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 11px;
  font-weight: 700;
  cursor: pointer;
  transition: color 160ms ease, background-color 160ms ease;
}

.label-count-button:hover,
.label-count-button:focus-visible {
  background: #eef6ff;
  color: #2563eb;
  outline: none;
  transform: none;
}

.label-count-button:focus-visible {
  box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.22);
}

.quota-limit-item {
  color: #64748b;
  font-size: 11px;
  white-space: nowrap;
}

.quota-limit-item strong {
  margin-left: 2px;
  color: #94a3b8;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-weight: 700;
}

.quota-limit-item.has-value strong {
  color: #2563eb;
}

.quota-limit-divider {
  width: 1px;
  height: 12px;
  background: #dbe7f4;
}

.operation-buttons {
  display: flex;
  gap: 14px;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: nowrap;
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

.namespace-table :deep(.el-tag) {
  border-radius: 999px;
  font-size: 11px;
  font-weight: 500;
}

@media (max-width: 1200px) {
  .operation-buttons {
    gap: 12px;
  }
}

@media (max-width: 768px) {
  .operation-buttons {
    flex-direction: column;
    gap: 4px;
  }

  .resource-stats {
    grid-template-columns: 1fr;
  }
}
</style>
