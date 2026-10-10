<template>
  <el-dialog
    v-model="visible"
    :title="title"
    :modal="true"
    modal-class="agent-host-selector-mask"
    width="1100px"
    top="5vh"
    class="host-selector-dialog"
    @close="handleClose"
  >
    <div class="dialog-content">
      <aside class="left-panel">
        <div class="panel-title">业务分组</div>

        <el-input
          v-model="groupSearchText"
          placeholder="搜索分组"
          clearable
          size="small"
          class="compact-input"
          @input="handleGroupSearch"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>

        <el-button
          type="primary"
          size="small"
          class="all-hosts-btn"
          :class="{ 'active-group': !currentGroupId }"
          @click="showAllHosts"
        >
          <el-icon><House /></el-icon>
          <span>全部主机</span>
          <span v-if="allHostsCount > 0" class="host-count-badge">{{ allHostsCount }}</span>
        </el-button>

        <div class="tree-container">
          <el-tree
            ref="groupTree"
            :data="filteredGroupList"
            :props="{ children: 'children', label: 'name' }"
            node-key="id"
            :highlight-current="true"
            class="basic-tree"
            @node-click="handleGroupClick"
          >
            <template #default="{ node, data }">
              <div class="tree-node" :class="{ active: currentGroupId === data.id }">
                <span class="node-label">{{ node.label }}</span>
                <span v-if="getGroupHostCount(data) > 0" class="host-count">
                  {{ getGroupHostCount(data) }}
                </span>
              </div>
            </template>
          </el-tree>
        </div>
      </aside>

      <section class="right-panel">
        <div class="table-toolbar">
          <el-input
            v-model="hostSearchKeyword"
            placeholder="搜索主机名称或IP"
            clearable
            size="small"
            class="compact-input host-search"
            @input="handleHostSearch"
          >
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>

          <div class="selection-info">
            <span class="selection-text">已选择</span>
            <span class="selection-count">{{ tempSelectedHosts.length }}</span>
            <span class="selection-text">台主机</span>
          </div>
        </div>

        <el-table
          ref="hostTable"
          v-loading="loading"
          :data="displayHosts"
          :row-class-name="getHostRowClassName"
          border
          max-height="380"
          class="host-table"
          @selection-change="handleSelectionChange"
          :header-cell-style="{
            background: '#ffffff',
            color: '#2c3e50',
            fontWeight: '600',
            borderBottom: '1px solid #e5e7eb'
          }"
        >
          <el-table-column type="selection" width="48" fixed="left" :selectable="isHostSelectable" />

          <el-table-column prop="hostName" label="主机名称" min-width="150">
            <template #default="{ row }">
              <div class="host-cell">
                <el-icon color="#409EFF"><Monitor /></el-icon>
                <div class="host-name-wrap">
                  <el-tooltip
                    :content="row.hostName || row.name || '-'"
                    placement="top"
                  >
                    <span class="host-name">{{ row.hostName || row.name || "-" }}</span>
                  </el-tooltip>
                </div>
                <el-tag
                  size="small"
                  effect="plain"
                  class="host-type-tag"
                  :type="isWindowsHost(row) ? 'info' : 'success'"
                >
                  {{ getHostTypeLabel(row) }}
                </el-tag>
              </div>
            </template>
          </el-table-column>

          <el-table-column prop="privateIp" label="内网IP" width="140">
            <template #default="{ row }">
              <div class="host-cell">
                <el-icon color="#67C23A"><HomeFilled /></el-icon>
                <span>{{ row.privateIp || "-" }}</span>
              </div>
            </template>
          </el-table-column>

          <el-table-column prop="sshIp" label="SSH地址" width="150">
            <template #default="{ row }">
              <div class="host-cell">
                <el-icon color="#E6A23C"><Connection /></el-icon>
                <span>{{ row.sshIp || row.publicIp || "-" }}</span>
              </div>
            </template>
          </el-table-column>

          <el-table-column prop="groupName" label="业务组" width="130" />

          <el-table-column label="Agent状态" width="110">
            <template #default="{ row }">
              <el-tag size="small" :type="getAgentStatusType(row)">
                {{ getAgentStatusText(row) }}
              </el-tag>
            </template>
          </el-table-column>

          <el-table-column prop="remark" label="备注" min-width="120" show-overflow-tooltip />
        </el-table>

        <div class="pagination-wrapper">
          <el-pagination
            :current-page="pagination.pageNum"
            :page-size="pagination.pageSize"
            :page-sizes="[10, 20, 50, 100]"
            :total="pagination.total"
            layout="total, sizes, prev, pager, next, jumper"
            class="modern-pagination"
            @current-change="handlePageChange"
            @size-change="handleSizeChange"
          />
        </div>
      </section>
    </div>

    <template #footer>
      <div class="dialog-footer">
        <div class="footer-info">
          <span v-if="tempSelectedHosts.length > 0">
            已选择 <strong>{{ tempSelectedHosts.length }}</strong> 台主机
          </span>
          <span v-else class="empty-text">请选择主机</span>
        </div>

        <div class="footer-actions">
          <el-button @click="handleClose">取消</el-button>
          <el-button
            type="primary"
            :disabled="tempSelectedHosts.length === 0"
            @click="confirmSelection"
          >
            确认选择
          </el-button>
        </div>
      </div>
    </template>
  </el-dialog>
</template>

<script setup>
/* global defineProps, defineEmits */
import { ref, watch } from 'vue'
import { Search, Monitor, House, HomeFilled, Connection } from '@element-plus/icons-vue'
import cmdbAPI from '@/api/cmdb'

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false
  },
  title: {
    type: String,
    default: '选择主机'
  }
})

const emit = defineEmits(['update:modelValue', 'hosts-selected'])

const visible = ref(false)
const loading = ref(false)
const groupList = ref([])
const visibleGroupList = ref([])
const filteredGroupList = ref([])
const groupSearchText = ref('')
const currentGroupId = ref(null)
const allHostsCount = ref(0)

const displayHosts = ref([])
const hostSearchKeyword = ref('')
const tempSelectedHosts = ref([])

const pagination = ref({
  pageNum: 1,
  pageSize: 10,
  total: 0
})

const groupTree = ref(null)
const hostTable = ref(null)

const getHostType = host => host?.hostType ?? host?.host_type ?? host?.deviceType ?? host?.device_type

const isWindowsHost = host => {
  const hostType = getHostType(host)
  const numericHostType = Number(hostType)

  if (Number.isFinite(numericHostType)) {
    return numericHostType === 2
  }

  return String(hostType || '').trim().toLowerCase() === 'windows'
}

const getHostTypeLabel = host => (isWindowsHost(host) ? 'Windows' : 'Linux')

const extractList = payload => {
  if (Array.isArray(payload)) return payload
  return payload?.list || payload?.items || []
}

const extractTotal = payload => {
  if (Array.isArray(payload)) return payload.length
  if (typeof payload?.total === 'number') return payload.total
  if (typeof payload?.count === 'number') return payload.count
  return extractList(payload).length
}

const isExplicitlyDisabled = host =>
  host?.deployDisabled === true ||
  host?.disabled === true ||
  host?.selectable === false ||
  host?.canDeploy === false

const normalizeCandidateHosts = hosts =>
  hosts.map(host => {
    const agentStatus = Number(host?.agentStatus || 0)
    const deployDisabledReason = String(
      host?.deployDisabledReason ||
      host?.disabledReason ||
      host?.reason ||
      ''
    ).trim()
    const deployDisabled = isExplicitlyDisabled(host)

    return {
      ...host,
      agentStatus,
      deployDisabled,
      deployDisabledReason
    }
  })

const clearHostTable = (options = {}) => {
  const { syncAllHostsCount = false } = options

  displayHosts.value = []
  pagination.value.total = 0

  if (syncAllHostsCount) {
    allHostsCount.value = 0
  }
}

const getAgentStatusText = row => {
  if (row?.agentStatusText) return row.agentStatusText

  const statusTextMap = {
    0: '可部署',
    1: '部署中',
    2: '部署失败',
    3: '运行中',
    4: '启动异常'
  }

  return statusTextMap[Number(row?.agentStatus || 0)] || '可部署'
}

const getAgentStatusType = row => {
  const status = Number(row?.agentStatus || 0)
  if (status === 3) return 'success'
  if (status === 1) return 'warning'
  if (status === 2 || status === 4) return 'danger'
  return 'info'
}

const getGroupHostCount = group => Number(group?.deployableHostCount ?? group?.hostCount ?? 0)

const keepDeployableGroups = groups =>
  groups
    .map(group => {
      const children = Array.isArray(group?.children)
        ? keepDeployableGroups(group.children)
        : []
      const hostCount = getGroupHostCount(group)

      if (hostCount <= 0 && children.length === 0) {
        return null
      }

      return {
        ...group,
        children
      }
    })
    .filter(Boolean)

const fetchGroupList = async () => {
  try {
    const response = await cmdbAPI.getAgentCandidateGroups()
    if (response.data?.code === 200) {
      groupList.value = keepDeployableGroups(extractList(response.data.data))
      visibleGroupList.value = groupList.value
      filteredGroupList.value = visibleGroupList.value
    } else {
      groupList.value = []
      visibleGroupList.value = []
      filteredGroupList.value = []
    }
  } catch (error) {
    console.error('获取可部署主机分组失败:', error)
    groupList.value = []
    visibleGroupList.value = []
    filteredGroupList.value = []
  }
}

const filterGroupsByKeyword = (groups, keyword) =>
  groups
    .map(group => {
      const children = Array.isArray(group.children)
        ? filterGroupsByKeyword(group.children, keyword)
        : []
      const match = String(group.name || '').toLowerCase().includes(keyword)

      if (match || children.length > 0) {
        return {
          ...group,
          children
        }
      }

      return null
    })
    .filter(Boolean)

const handleGroupSearch = () => {
  const keyword = groupSearchText.value.trim().toLowerCase()

  if (!keyword) {
    filteredGroupList.value = visibleGroupList.value
    return
  }

  filteredGroupList.value = filterGroupsByKeyword(visibleGroupList.value, keyword)
}

const fetchCandidateHosts = async (options = {}) => {
  const { syncAllHostsCount = false } = options
  loading.value = true

  try {
    const params = {
      page: pagination.value.pageNum,
      pageSize: pagination.value.pageSize
    }

    const keyword = hostSearchKeyword.value.trim()
    if (keyword) {
      params.keyword = keyword
    }

    if (currentGroupId.value) {
      params.groupId = currentGroupId.value
    }

    const response = await cmdbAPI.getAgentCandidates(params)

    if (response.data?.code !== 200) {
      throw new Error(response.data?.message || '获取可部署主机失败')
    }

    const payload = response.data.data || {}
    const hosts = normalizeCandidateHosts(extractList(payload))
    const total = extractTotal(payload)

    displayHosts.value = hosts
    pagination.value.total = total

    if (syncAllHostsCount) {
      allHostsCount.value = total
    }
  } catch (error) {
    console.error('获取可部署主机失败:', error)
    clearHostTable({ syncAllHostsCount })
  } finally {
    loading.value = false
  }
}

const showAllHosts = async () => {
  currentGroupId.value = null
  hostSearchKeyword.value = ''
  pagination.value.pageNum = 1
  await fetchCandidateHosts({ syncAllHostsCount: true })
}

const handleGroupClick = async data => {
  currentGroupId.value = data.id
  hostSearchKeyword.value = ''
  pagination.value.pageNum = 1
  await fetchCandidateHosts()
}

const handleHostSearch = async (options = {}) => {
  const { resetPage = true } = options

  if (resetPage) {
    pagination.value.pageNum = 1
  }

  await fetchCandidateHosts()
}

const handlePageChange = page => {
  pagination.value.pageNum = page
  fetchCandidateHosts()
}

const handleSizeChange = size => {
  pagination.value.pageSize = size
  pagination.value.pageNum = 1
  fetchCandidateHosts()
}

const isHostSelectable = row => !row.deployDisabled

const getHostRowClassName = ({ row }) => (row.deployDisabled ? 'non-deployable-row' : '')

const handleSelectionChange = selection => {
  tempSelectedHosts.value = selection.filter(isHostSelectable)
}

const confirmSelection = () => {
  emit('hosts-selected', tempSelectedHosts.value.filter(isHostSelectable))
  handleClose()
}

const handleClose = () => {
  visible.value = false
  currentGroupId.value = null
  tempSelectedHosts.value = []
  hostSearchKeyword.value = ''
  groupSearchText.value = ''
  displayHosts.value = []
  pagination.value.pageNum = 1
  pagination.value.total = 0
}

watch(
  () => props.modelValue,
  async value => {
    visible.value = value

    if (value) {
      tempSelectedHosts.value = []
      hostSearchKeyword.value = ''
      groupSearchText.value = ''
      currentGroupId.value = null
      pagination.value.pageNum = 1
      await fetchGroupList()
      await fetchCandidateHosts({ syncAllHostsCount: true })
    }
  }
)

watch(visible, value => {
  emit('update:modelValue', value)
})
</script>

<style scoped>
:global(.agent-host-selector-mask) {
  background: rgba(15, 23, 42, 0.14) !important;
}

.host-selector-dialog :deep(.el-dialog) {
  border-radius: 10px;
  overflow: hidden;
}

.host-selector-dialog :deep(.el-dialog__header) {
  margin-right: 0;
  padding: 14px 16px;
  border-bottom: 1px solid #e5e7eb;
}

.host-selector-dialog :deep(.el-dialog__title) {
  font-size: 16px;
  font-weight: 600;
  color: #1f2937;
}

.host-selector-dialog :deep(.el-dialog__headerbtn) {
  top: 14px;
}

.host-selector-dialog :deep(.el-dialog__body) {
  padding: 0;
}

.dialog-content {
  display: flex;
  height: 450px;
  min-width: 0;
}

.left-panel {
  width: 220px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  background: #f8fafc;
  border-right: 1px solid #e5e7eb;
}

.panel-title {
  font-size: 14px;
  font-weight: 600;
  color: #334155;
}

.compact-input :deep(.el-input__wrapper) {
  border-radius: 6px;
  box-shadow: none;
}

.all-hosts-btn {
  width: 100%;
  justify-content: flex-start;
  gap: 6px;
}

.all-hosts-btn.active-group {
  background: #409eff;
  border-color: #409eff;
}

.host-count-badge {
  margin-left: auto;
  min-width: 20px;
  padding: 0 6px;
  font-size: 12px;
  line-height: 18px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.18);
}

.tree-container {
  flex: 1;
  overflow: auto;
  padding: 6px;
  background: #ffffff;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
}

.basic-tree {
  background: transparent;
}

.basic-tree :deep(.el-tree-node__content) {
  height: 34px;
  border-radius: 6px;
}

.basic-tree :deep(.el-tree-node__content:hover) {
  background: #f1f5f9;
}

.tree-node {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-right: 8px;
}

.tree-node.active {
  color: #2563eb;
  font-weight: 600;
}

.node-label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.host-count {
  flex-shrink: 0;
  font-size: 12px;
  color: #94a3b8;
}

.right-panel {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  padding: 12px;
  display: flex;
  flex-direction: column;
  background: #ffffff;
}

.table-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  min-width: 0;
  gap: 12px;
  margin-bottom: 10px;
}

.host-search {
  width: 280px;
  max-width: 100%;
  flex: 1 1 260px;
}

.selection-info {
  display: inline-flex;
  align-items: center;
  flex-shrink: 0;
  gap: 6px;
  margin-left: auto;
  padding: 6px 10px;
  background: #f8fafc;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  white-space: nowrap;
}

.selection-text {
  font-size: 12px;
  color: #64748b;
}

.selection-count {
  font-size: 16px;
  font-weight: 700;
  color: #2563eb;
}

.host-table {
  flex: 1;
  min-width: 0;
}

.host-table :deep(.el-table__header-wrapper .el-table__header thead th) {
  color: #2c3e50 !important;
  background-color: transparent !important;
}

.host-table :deep(.el-table__header-wrapper .el-table__header thead th .cell) {
  color: #2c3e50 !important;
}

.host-table :deep(.el-table__body tr:hover > td) {
  background: #f8fafc;
}

.host-table :deep(.non-deployable-row > td) {
  color: #9ca3af;
  background: #f8fafc !important;
}

.host-table :deep(.non-deployable-row .el-tag) {
  opacity: 0.7;
}

.host-cell {
  display: flex;
  align-items: center;
  width: 100%;
  gap: 6px;
  min-width: 0;
  flex-wrap: nowrap;
}

.host-name-wrap {
  flex: 1;
  min-width: 0;
}

.host-name {
  display: block;
  width: 100%;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.host-type-tag {
  flex-shrink: 0;
  margin-left: auto;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 10px;
}

.modern-pagination :deep(.el-pager li.is-active) {
  color: #ffffff;
  background: #409eff;
}

.dialog-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-top: 1px solid #e5e7eb;
}

.footer-info {
  font-size: 13px;
  color: #64748b;
}

.footer-info strong {
  color: #2563eb;
}

.empty-text {
  color: #94a3b8;
}

.footer-actions {
  display: flex;
  gap: 8px;
}

.tree-container::-webkit-scrollbar {
  width: 6px;
}

.tree-container::-webkit-scrollbar-thumb {
  background: #cbd5e1;
  border-radius: 999px;
}
/* Agent deployment dialog aligned with DESIGN.md */
:global(.agent-host-selector-mask .el-dialog) {
  --el-dialog-border-radius: 15px;
  width: min(1100px, calc(100vw - 24px)) !important;
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.agent-host-selector-mask .el-dialog__header) {
  position: relative;
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: linear-gradient(90deg, #fff6e7 0%, #f7fbff 36%, #ecf7ff 100%);
}

:global(.agent-host-selector-mask .el-dialog__header::before) {
  content: '';
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 3px;
  background: linear-gradient(90deg, #f5a524 0%, #62a8ff 48%, #55d6be 100%);
}

:global(.agent-host-selector-mask .el-dialog__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.agent-host-selector-mask .el-dialog__headerbtn) {
  top: 10px;
  right: 12px;
}

:global(.agent-host-selector-mask .el-dialog__body) {
  padding: 0;
}

:global(.agent-host-selector-mask .el-dialog__footer) {
  padding: 0;
  border-top: 0;
}

.compact-input :deep(.el-input__wrapper),
.all-hosts-btn,
.selection-info,
.tree-container {
  border-radius: 8px;
}

.dialog-footer {
  padding: 12px 18px 16px;
  border-top-color: #e3edf8;
}

.footer-actions :deep(.el-button) {
  min-width: 78px;
  min-height: 32px;
  border-radius: 8px;
  font-weight: 600;
}

.footer-actions :deep(.el-button--primary) {
  border-color: #409eff;
  color: #fff;
  background: linear-gradient(135deg, #5aa8ff, #2563eb);
}

.host-table :deep(.el-table__row),
.host-table :deep(.el-table__row:hover) {
  box-shadow: none;
  transform: none;
}

@media (max-width: 768px) {
  .dialog-content {
    height: auto;
    min-height: 520px;
    flex-direction: column;
  }

  .left-panel {
    box-sizing: border-box;
    width: 100%;
    max-height: 220px;
    border-right: 0;
    border-bottom: 1px solid #e5e7eb;
  }

  .right-panel {
    min-height: 360px;
  }

  .dialog-footer {
    align-items: stretch;
    flex-direction: column;
    gap: 8px;
  }
}
</style>
