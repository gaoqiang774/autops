<template>
  <section class="ticket-view workflow-view" aria-labelledby="workflow-list-title">
    <div class="workflow-summary" aria-label="审批流概览">
      <div v-for="item in summaryItems" :key="item.key" class="workflow-summary__item" :class="`is-${item.tone}`">
        <span class="workflow-summary__icon" aria-hidden="true">
          <el-icon><component :is="item.icon" /></el-icon>
        </span>
        <span class="workflow-summary__content">
          <span>{{ item.label }}</span>
          <small>{{ item.hint }}</small>
        </span>
        <strong>{{ formatCount(item.value) }}</strong>
      </div>
    </div>

    <div class="workflow-toolbar">
      <div class="workflow-toolbar__heading">
        <h2 id="workflow-list-title">审批流配置</h2>
        <p>{{ resultSummary }}</p>
      </div>
      <div class="workflow-toolbar__controls">
        <div class="workflow-toolbar__filters" role="search" aria-label="审批流筛选">
          <el-input
            v-model="query.keyword"
            class="workflow-filter--keyword"
            clearable
            placeholder="搜索流程名称"
            aria-label="按流程名称查询"
            @keyup.enter="search"
            @clear="search"
          >
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-select
            v-model="query.enabled"
            class="workflow-filter--status"
            clearable
            placeholder="全部状态"
            aria-label="按启用状态筛选"
          >
            <el-option label="启用" value="1" />
            <el-option label="禁用" value="0" />
          </el-select>
          <el-button type="primary" :loading="loading" @click="search">
            <el-icon><Search /></el-icon>查询
          </el-button>
          <el-button type="warning" :disabled="loading" @click="resetFilters">
            <el-icon><RefreshRight /></el-icon>重置
          </el-button>
        </div>
        <div class="workflow-toolbar__actions">
          <el-tooltip content="刷新审批流列表" placement="top">
            <el-button class="workflow-refresh-btn" type="warning" :loading="loading" @click="load">
              <el-icon><Refresh /></el-icon>刷新
            </el-button>
          </el-tooltip>
          <el-button type="primary" @click="openCreate">
            <el-icon><Plus /></el-icon>新建审批流
          </el-button>
        </div>
      </div>
    </div>

    <div v-if="loadError" class="workflow-notice" role="alert">
      <el-icon><Warning /></el-icon>
      <span>{{ loadError }}</span>
      <el-button link type="primary" @click="load">重新加载</el-button>
    </div>

    <div class="workflow-table-wrap" v-loading="loading">
      <el-table :data="rows" height="100%" table-layout="fixed" row-key="id">
        <el-table-column label="流程名称" min-width="218" fixed="left">
          <template #default="{ row }">
            <div class="workflow-name-cell">
              <button type="button" class="ticket-link" :title="row.name" @click="openEdit(row)">
                {{ row.name || '未命名审批流' }}
              </button>
              <span>版本 v{{ row.version || 1 }}<template v-if="row.description"> · {{ row.description }}</template></span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="适用工单" min-width="180">
          <template #default="{ row }">
            <div v-if="row.businessTypes?.length" class="workflow-tag-list">
              <el-tag v-for="type in row.businessTypes" :key="type" size="small" effect="light" :type="businessTagType(type)">
                {{ businessLabels[type] || type }}
              </el-tag>
            </div>
            <span v-else class="workflow-empty-value">未配置</span>
          </template>
        </el-table-column>
        <el-table-column label="适用环境" min-width="152">
          <template #default="{ row }">
            <span v-if="!row.environments?.length" class="workflow-all-environments">
              <el-icon><MapLocation /></el-icon>全部环境
            </span>
            <div v-else class="workflow-tag-list">
              <el-tag v-for="env in row.environments" :key="env" size="small" type="info" effect="light">
                {{ environmentLabels[env] || env }}
              </el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="审批节点" min-width="310">
          <template #default="{ row }">
            <el-tooltip :content="nodeTooltip(row)" placement="top" :disabled="!row.nodes?.length">
              <div v-if="row.nodes?.length" class="workflow-node-chain">
                <template v-for="(node, index) in visibleNodes(row)" :key="node.id || `${row.id}-${index}`">
                  <span class="workflow-node-chip"><b>{{ node.nodeOrder || index + 1 }}</b>{{ node.name }}</span>
                  <el-icon v-if="index < visibleNodes(row).length - 1" class="workflow-node-arrow"><ArrowRight /></el-icon>
                </template>
                <span v-if="row.nodes.length > visibleNodeLimit" class="workflow-node-more">+{{ row.nodes.length - visibleNodeLimit }}</span>
              </div>
            </el-tooltip>
            <span v-if="!row.nodes?.length" class="workflow-empty-value">暂无节点</span>
          </template>
        </el-table-column>
        <el-table-column label="维护信息" width="156">
          <template #default="{ row }">
            <div class="workflow-maintainer-cell">
              <span><el-icon><User /></el-icon>{{ row.maintainerName || '-' }}</span>
              <time>{{ formatTime(row.updatedAt) }}</time>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="116" align="center">
          <template #default="{ row }">
            <div class="workflow-status-control">
              <el-switch
                :model-value="Number(row.enabled) === 1"
                :loading="statusChangingId === row.id"
                :disabled="Boolean(statusChangingId)"
                :aria-label="`${row.name}启用状态`"
                @change="value => toggleStatus(row, value)"
              />
              <span :class="Number(row.enabled) === 1 ? 'is-enabled' : 'is-disabled'">
                {{ Number(row.enabled) === 1 ? '启用' : '禁用' }}
              </span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="112" fixed="right" align="right">
          <template #default="{ row }">
            <div class="workflow-row-actions">
              <el-button link type="primary" :loading="editingLoadingId === row.id" @click="openEdit(row)">编辑</el-button>
              <el-button link type="danger" :loading="deletingId === row.id" :disabled="Boolean(deletingId)" @click="remove(row)">删除</el-button>
            </div>
          </template>
        </el-table-column>
        <template #empty>
          <div class="workflow-empty">
            <el-icon><Connection /></el-icon>
            <strong>{{ hasActiveFilters ? '没有匹配的审批流' : '还没有审批流' }}</strong>
            <span>{{ hasActiveFilters ? '请调整流程名称或启用状态后重试' : '创建审批流后，可用于日常事务、应用发布和 SQL 审核' }}</span>
            <el-button v-if="hasActiveFilters" type="primary" plain @click="resetFilters">清除筛选条件</el-button>
            <el-button v-else type="primary" @click="openCreate"><el-icon><Plus /></el-icon>新建审批流</el-button>
          </div>
        </template>
      </el-table>
    </div>

    <div class="workflow-pagination">
      <el-pagination
        v-model:current-page="query.page"
        v-model:page-size="query.pageSize"
        background
        :total="total"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next"
        @current-change="load"
        @size-change="sizeChange"
      />
    </div>

    <el-dialog
      v-model="dialog"
      class="workflow-editor-dialog"
      modal-class="workflow-editor-dialog-overlay"
      width="980px"
      destroy-on-close
      :close-on-click-modal="false"
      :close-on-press-escape="!saving"
      @closed="resetForm"
    >
      <template #header>
        <div class="workflow-dialog-title">
          <span class="workflow-dialog-title__icon"><el-icon><Connection /></el-icon></span>
          <div>
            <h2>{{ editingId ? '编辑审批流' : '新建审批流' }}</h2>
            <p>{{ editingId ? `正在维护「${form.name || '未命名审批流'}」` : '配置适用范围、审批策略和流转节点' }}</p>
          </div>
        </div>
      </template>

      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" class="workflow-form">
        <section class="workflow-form-section" aria-labelledby="workflow-basic-title">
          <div class="workflow-form-section__heading">
            <div><h3 id="workflow-basic-title">基础信息</h3><p>定义审批流名称与适用业务范围</p></div>
          </div>
          <div class="workflow-form-grid">
            <el-form-item label="流程名称" prop="name">
              <el-input v-model="form.name" maxlength="100" show-word-limit placeholder="例如：生产环境应用发布审批" />
            </el-form-item>
            <el-form-item label="适用工单" prop="businessTypes">
              <el-select v-model="form.businessTypes" multiple collapse-tags collapse-tags-tooltip placeholder="请选择工单类型">
                <el-option v-for="(label, value) in businessLabels" :key="value" :label="label" :value="value" />
              </el-select>
            </el-form-item>
            <el-form-item label="适用环境">
              <el-select v-model="form.environments" multiple clearable collapse-tags collapse-tags-tooltip placeholder="不选择表示全部环境">
                <el-option v-for="(label, value) in environmentLabels" :key="value" :label="label" :value="value" />
              </el-select>
            </el-form-item>
            <el-form-item label="流程描述" class="workflow-form-full">
              <el-input v-model="form.description" type="textarea" :rows="2" maxlength="500" show-word-limit resize="none" placeholder="说明该审批流的使用场景或管理要求" />
            </el-form-item>
          </div>
        </section>

        <section class="workflow-form-section" aria-labelledby="workflow-policy-title">
          <div class="workflow-form-section__heading">
            <div><h3 id="workflow-policy-title">流程策略</h3><p>控制流程可用状态和审批隔离规则</p></div>
          </div>
          <div class="workflow-policy-list">
            <label class="workflow-policy-item">
              <span class="workflow-policy-item__icon is-primary"><el-icon><CircleCheck /></el-icon></span>
              <span><strong>启用流程</strong><small>启用后可在对应业务工单中选择此流程</small></span>
              <el-switch v-model="form.enabled" :active-value="1" :inactive-value="0" />
            </label>
            <label class="workflow-policy-item">
              <span class="workflow-policy-item__icon is-warning"><el-icon><Lock /></el-icon></span>
              <span><strong>禁止自审</strong><small>申请人不能审批自己发起的工单</small></span>
              <el-switch v-model="form.preventSelfApproval" :active-value="1" :inactive-value="0" />
            </label>
          </div>
        </section>

        <section class="workflow-form-section workflow-form-section--nodes" aria-labelledby="workflow-nodes-title">
          <div class="workflow-form-section__heading">
            <div><h3 id="workflow-nodes-title">审批节点</h3><p>节点按顺序流转，指定角色会在发起工单时解析为具体用户</p></div>
            <el-button type="primary" plain @click="addNode"><el-icon><Plus /></el-icon>新增节点</el-button>
          </div>

          <div v-if="optionsError" class="workflow-options-notice" role="alert">
            <el-icon><Warning /></el-icon><span>{{ optionsError }}</span>
            <el-button link type="primary" @click="loadOptions">重新加载</el-button>
          </div>

          <div v-if="form.nodes.length" class="workflow-node-editor" v-loading="optionsLoading">
            <div class="workflow-node-editor__columns" aria-hidden="true">
              <span>顺序</span><span>节点名称</span><span>审批对象类型</span><span>审批对象</span><span>通过方式</span><span>排序与删除</span>
            </div>
            <div v-for="(node, index) in form.nodes" :key="node.key" class="workflow-node-config" :class="{ 'is-invalid': nodeErrors[node.key]?.length }">
              <div class="workflow-node-config__main">
                <span class="workflow-node-index">{{ index + 1 }}</span>
                <el-input v-model="node.name" placeholder="例如：部门负责人" @input="clearNodeError(node.key)" />
                <el-select v-model="node.approverType" @change="changeApproverType(node)">
                  <el-option label="指定用户" value="user" />
                  <el-option label="指定角色" value="role" />
                </el-select>
                <el-select
                  v-model="node.approverIds"
                  multiple
                  filterable
                  collapse-tags
                  collapse-tags-tooltip
                  :disabled="optionsLoading"
                  placeholder="选择审批对象"
                  @change="clearNodeError(node.key)"
                >
                  <el-option
                    v-for="option in node.approverType === 'role' ? roleOptions : userOptions"
                    :key="option.id"
                    :label="option.label"
                    :value="Number(option.id)"
                  />
                </el-select>
                <el-select v-model="node.approvalMode">
                  <el-option label="任一通过" value="any" />
                  <el-option label="全部通过" value="all" />
                </el-select>
                <div class="workflow-node-config__actions">
                  <el-tooltip content="上移" placement="top">
                    <el-button text :icon="ArrowUp" :disabled="index === 0" aria-label="上移节点" @click="moveNode(index, -1)" />
                  </el-tooltip>
                  <el-tooltip content="下移" placement="top">
                    <el-button text :icon="ArrowDown" :disabled="index === form.nodes.length - 1" aria-label="下移节点" @click="moveNode(index, 1)" />
                  </el-tooltip>
                  <el-tooltip content="删除节点" placement="top">
                    <el-button text type="danger" :icon="Delete" aria-label="删除节点" @click="removeNode(index)" />
                  </el-tooltip>
                </div>
              </div>
              <div v-if="nodeErrors[node.key]?.length" class="workflow-node-config__errors" role="alert">
                <span v-for="message in nodeErrors[node.key]" :key="message"><el-icon><WarningFilled /></el-icon>{{ message }}</span>
              </div>
            </div>
          </div>
          <div v-else class="workflow-node-empty">
            <el-icon><Stamp /></el-icon><strong>至少配置一个审批节点</strong><span>每个节点需要填写名称并选择审批用户或角色</span>
            <el-button type="primary" plain @click="addNode"><el-icon><Plus /></el-icon>添加审批节点</el-button>
          </div>
        </section>
      </el-form>

      <template #footer>
        <div class="workflow-dialog-footer">
          <span>共 {{ form.nodes.length }} 个审批节点</span>
          <div><el-button :disabled="saving" @click="dialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="save">保存审批流</el-button></div>
        </div>
      </template>
    </el-dialog>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  ArrowDown,
  ArrowRight,
  ArrowUp,
  CircleCheck,
  Connection,
  Delete,
  Lock,
  MapLocation,
  Plus,
  Refresh,
  RefreshRight,
  Search,
  Stamp,
  Tickets,
  User,
  Warning,
  WarningFilled
} from '@element-plus/icons-vue'
import ticketApi from '@/api/ticket'
import systemApi from '@/api/system'
import { businessLabels, ensure, environmentLabels, formatTime, listResult, userLabel } from './ticketCommon'

const visibleNodeLimit = 3
const loading = ref(false)
const saving = ref(false)
const optionsLoading = ref(false)
const rows = ref([])
const total = ref(0)
const dialog = ref(false)
const editingId = ref(null)
const editingLoadingId = ref(null)
const statusChangingId = ref(null)
const deletingId = ref(null)
const formRef = ref()
const userOptions = ref([])
const roleOptions = ref([])
const loadError = ref('')
const optionsError = ref('')
const nodeErrors = ref({})

const query = reactive({ page: 1, pageSize: 10, keyword: '', enabled: '' })
const defaultForm = () => ({ name: '', businessTypes: [], environments: [], description: '', enabled: 1, preventSelfApproval: 1, nodes: [] })
const form = reactive(defaultForm())
const rules = {
  name: [{ required: true, message: '请输入流程名称', trigger: 'blur' }],
  businessTypes: [{ required: true, type: 'array', min: 1, message: '请选择适用工单', trigger: 'change' }]
}

const enabledCount = computed(() => rows.value.filter(item => Number(item.enabled) === 1).length)
const coveredTypes = computed(() => new Set(rows.value.flatMap(item => item.businessTypes || [])).size)
const nodeCount = computed(() => rows.value.reduce((sum, item) => sum + (item.nodes?.length || 0), 0))
const hasActiveFilters = computed(() => Boolean(query.keyword || query.enabled !== ''))
const resultSummary = computed(() => {
  const status = query.enabled === '1' ? '启用状态' : query.enabled === '0' ? '禁用状态' : '全部状态'
  return `${status}，共 ${formatCount(total.value)} 条`
})
const summaryItems = computed(() => [
  { key: 'total', label: '审批流总数', hint: '当前查询结果', value: total.value, icon: Connection, tone: 'primary' },
  { key: 'enabled', label: '本页启用', hint: '当前页可用流程', value: enabledCount.value, icon: CircleCheck, tone: 'success' },
  { key: 'business', label: '本页覆盖类型', hint: '当前页工单类型', value: coveredTypes.value, icon: Tickets, tone: 'warning' },
  { key: 'nodes', label: '本页审批节点', hint: '当前页节点合计', value: nodeCount.value, icon: Stamp, tone: 'danger' }
])

const formatCount = value => Number(value || 0).toLocaleString('zh-CN')
const visibleNodes = row => (row.nodes || []).slice(0, visibleNodeLimit)
const nodeTooltip = row => (row.nodes || []).map((node, index) => `${node.nodeOrder || index + 1}. ${node.name || '未命名节点'}`).join(' → ')
const businessTagType = type => ({ daily: 'primary', release: 'success', sql_review: 'warning' }[type] || 'info')

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    const result = listResult(await ticketApi.workflows(query))
    rows.value = result.list
    total.value = result.total
  } catch (error) {
    loadError.value = error.message || '审批流列表加载失败，请重试'
  } finally {
    loading.value = false
  }
}

async function loadOptions() {
  optionsLoading.value = true
  optionsError.value = ''
  try {
    const [users, roles] = await Promise.all([
      systemApi.queryAdminList({ PageSize: 100, PageNum: 1, Status: 1 }),
      systemApi.querySysRoleVoList()
    ])
    userOptions.value = listResult(users).list.map(item => ({ id: item.id, label: userLabel(item) }))
    roleOptions.value = listResult(roles).list.map(item => ({ id: item.id ?? item.Id, label: item.roleName || item.RoleName }))
  } catch (error) {
    optionsError.value = error.message || '用户或角色加载失败，请重试'
  } finally {
    optionsLoading.value = false
  }
}

function search() {
  query.page = 1
  load()
}

function resetFilters() {
  Object.assign(query, { page: 1, keyword: '', enabled: '' })
  load()
}

function sizeChange() {
  query.page = 1
  load()
}

function resetForm() {
  editingId.value = null
  Object.assign(form, defaultForm())
  nodeErrors.value = {}
  formRef.value?.clearValidate()
}

function createNode(node = {}) {
  return {
    key: node.id ?? `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    name: node.name || '',
    approverType: node.approverType || 'user',
    approverIds: [...(node.approverIds || [])].map(Number),
    approvalMode: node.approvalMode || 'any',
    description: node.description || ''
  }
}

function addNode() {
  form.nodes.push(createNode())
}

function removeNode(index) {
  const [removed] = form.nodes.splice(index, 1)
  if (removed) clearNodeError(removed.key)
}

function moveNode(index, offset) {
  const target = index + offset
  if (target < 0 || target >= form.nodes.length) return
  const [node] = form.nodes.splice(index, 1)
  form.nodes.splice(target, 0, node)
}

function changeApproverType(node) {
  node.approverIds = []
  clearNodeError(node.key)
}

function clearNodeError(key) {
  if (!nodeErrors.value[key]) return
  const next = { ...nodeErrors.value }
  delete next[key]
  nodeErrors.value = next
}

function fillForm(workflow) {
  Object.assign(form, {
    name: workflow.name || '',
    businessTypes: [...(workflow.businessTypes || [])],
    environments: [...(workflow.environments || [])],
    description: workflow.description || '',
    enabled: Number(workflow.enabled ?? 1),
    preventSelfApproval: Number(workflow.preventSelfApproval ?? 1),
    nodes: (workflow.nodes || []).map(createNode)
  })
}

function openCreate() {
  resetForm()
  addNode()
  dialog.value = true
}

async function openEdit(row) {
  if (editingLoadingId.value) return
  editingLoadingId.value = row.id
  try {
    const workflow = ensure(await ticketApi.workflow(row.id), '加载审批流详情失败')
    resetForm()
    editingId.value = row.id
    fillForm(workflow)
    dialog.value = true
  } catch (error) {
    ElMessage.error(error.message || '加载审批流详情失败')
  } finally {
    editingLoadingId.value = null
  }
}

function validateNodes() {
  if (!form.nodes.length) {
    ElMessage.warning('请至少配置一个审批节点')
    return false
  }
  const errors = {}
  form.nodes.forEach(node => {
    const messages = []
    if (!String(node.name || '').trim()) messages.push('请输入节点名称')
    if (!node.approverIds?.length) messages.push('请选择至少一个审批对象')
    if (messages.length) errors[node.key] = messages
  })
  nodeErrors.value = errors
  if (Object.keys(errors).length) {
    ElMessage.warning('请完善标记的审批节点')
    return false
  }
  return true
}

async function save() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid || !validateNodes()) return

  saving.value = true
  try {
    const response = editingId.value
      ? await ticketApi.updateWorkflow(editingId.value, form)
      : await ticketApi.createWorkflow(form)
    ensure(response, '保存审批流失败')
    ElMessage.success(editingId.value ? '审批流已更新' : '审批流已创建')
    dialog.value = false
    await load()
  } catch (error) {
    ElMessage.error(error.message || '保存审批流失败')
  } finally {
    saving.value = false
  }
}

async function toggleStatus(row, value) {
  if (statusChangingId.value) return
  try {
    if (!value) {
      await ElMessageBox.confirm(
        `禁用后，新工单将不能选择审批流「${row.name}」。确认继续？`,
        '禁用审批流',
        { type: 'warning', confirmButtonText: '确认禁用', cancelButtonText: '取消' }
      )
    }
    statusChangingId.value = row.id
    ensure(await ticketApi.updateWorkflowStatus(row.id, value ? 1 : 0), '更新审批流状态失败')
    row.enabled = value ? 1 : 0
    ElMessage.success(value ? '审批流已启用' : '审批流已禁用')
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(error.message || '更新审批流状态失败')
  } finally {
    statusChangingId.value = null
  }
}

async function remove(row) {
  if (deletingId.value) return
  try {
    await ElMessageBox.confirm(
      `删除后无法恢复，且不能再用于新工单。确认删除审批流「${row.name}」？`,
      '删除审批流',
      { type: 'warning', confirmButtonText: '确认删除', cancelButtonText: '取消' }
    )
    deletingId.value = row.id
    ensure(await ticketApi.deleteWorkflow(row.id), '删除审批流失败')
    ElMessage.success('审批流已删除')
    if (rows.value.length === 1 && query.page > 1) query.page -= 1
    await load()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(error.message || '删除审批流失败')
  } finally {
    deletingId.value = null
  }
}

onMounted(() => {
  load()
  loadOptions()
})
</script>

<style scoped>
.workflow-view {
  --workflow-heading: #0f172a;
  --workflow-text: #172033;
  --workflow-secondary: #53657d;
  --workflow-muted: #64748b;
  --workflow-border: #dbe7f4;
  --workflow-border-soft: #e7eef7;
  --workflow-surface: #ffffff;
}

.workflow-summary {
  display: grid;
  flex: 0 0 auto;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid var(--workflow-border);
  border-radius: 10px;
  background: var(--workflow-surface);
}

.workflow-summary__item {
  display: grid;
  min-width: 0;
  min-height: 70px;
  grid-template-columns: 34px minmax(0, 1fr) auto;
  align-items: center;
  gap: 9px;
  padding: 9px 11px;
  border-right: 1px solid var(--workflow-border-soft);
}

.workflow-summary__item:last-child { border-right: 0; }
.workflow-summary__icon {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border-radius: 8px;
  color: #2563eb;
  background: #eef6ff;
  font-size: 17px;
}
.workflow-summary__item.is-success .workflow-summary__icon { color: #047857; background: #eaf8ef; }
.workflow-summary__item.is-warning .workflow-summary__icon { color: #9a5b00; background: #fff4e5; }
.workflow-summary__item.is-danger .workflow-summary__icon { color: #c81e1e; background: #fff0f0; }
.workflow-summary__content { display: grid; min-width: 0; gap: 3px; }
.workflow-summary__content span,
.workflow-summary__content small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.workflow-summary__content span { color: var(--workflow-secondary); font-size: 12px; font-weight: 700; }
.workflow-summary__content small { color: var(--workflow-muted); font-size: 11px; }
.workflow-summary__item strong { color: var(--workflow-heading); font-family: Consolas, "SFMono-Regular", monospace; font-size: 21px; font-variant-numeric: tabular-nums; }

.workflow-toolbar { display: flex; flex: 0 0 auto; align-items: center; justify-content: space-between; gap: 12px; min-width: 0; }
.workflow-toolbar__heading { flex: 0 0 auto; }
.workflow-toolbar__heading h2 { margin: 0; color: var(--workflow-heading); font-size: 15px; line-height: 1.3; }
.workflow-toolbar__heading p { margin: 2px 0 0; color: var(--workflow-muted); font-size: 11px; }
.workflow-toolbar__controls,
.workflow-toolbar__filters,
.workflow-toolbar__actions { display: flex; min-width: 0; align-items: center; justify-content: flex-end; gap: 7px; }
.workflow-filter--keyword { width: 220px; }
.workflow-filter--status { width: 128px; }
.workflow-toolbar :deep(.el-input__wrapper),
.workflow-toolbar :deep(.el-select__wrapper) { min-height: 34px; border-radius: 7px; background: #fbfdff; box-shadow: 0 0 0 1px #d9e5f2 inset; }
.workflow-toolbar :deep(.el-input__wrapper:hover),
.workflow-toolbar :deep(.el-select__wrapper:hover) { box-shadow: 0 0 0 1px #b9d4ff inset; }
.workflow-toolbar :deep(.el-input__wrapper.is-focus),
.workflow-toolbar :deep(.el-select__wrapper.is-focused) { box-shadow: 0 0 0 1px #409eff inset; }
.workflow-toolbar .el-button { min-height: 34px; margin: 0; font-weight: 600; white-space: nowrap; }
.workflow-refresh-btn { border-color: #e5b94e !important; background: #e5b94e !important; color: #fff !important; }

.workflow-notice,
.workflow-options-notice { display: flex; align-items: center; gap: 7px; box-sizing: border-box; border: 1px solid #f3d59b; border-radius: 8px; color: #854d0e; background: #fff9ed; font-size: 12px; }
.workflow-notice { min-height: 32px; flex: 0 0 auto; padding: 5px 9px; }
.workflow-options-notice { margin-bottom: 9px; padding: 7px 9px; }
.workflow-notice span,
.workflow-options-notice span { min-width: 0; flex: 1; }

.workflow-table-wrap { flex: 1; min-height: 280px; overflow: hidden; border: 1px solid #e3ebf5; border-radius: 8px; background: #fff; }
.workflow-table-wrap :deep(.el-table) { height: 100%; color: var(--workflow-text); }
.workflow-table-wrap :deep(.el-table th.el-table__cell) { height: 38px; padding: 0; color: #334155; background: #f3f7fc; font-size: 12px; font-weight: 700; }
.workflow-table-wrap :deep(.el-table td.el-table__cell) { height: 46px; padding: 0; border-bottom-color: #eef3f9; }
.workflow-table-wrap :deep(.el-table .cell) { font-size: 12px; line-height: 1.45; }
.workflow-table-wrap :deep(.el-table__body tr:hover > td.el-table__cell) { background: #f8fafc; }
.workflow-table-wrap :deep(.el-table__empty-block) { min-height: 250px; }

.workflow-name-cell { display: grid; min-width: 0; gap: 2px; }
.workflow-name-cell .ticket-link { overflow: hidden; text-align: left; text-overflow: ellipsis; white-space: nowrap; }
.workflow-name-cell span { overflow: hidden; color: var(--workflow-muted); font-size: 10.5px; text-overflow: ellipsis; white-space: nowrap; }
.workflow-tag-list { display: flex; min-width: 0; flex-wrap: wrap; gap: 4px; }
.workflow-tag-list .el-tag { margin: 0; }
.workflow-empty-value { color: #94a3b8; }
.workflow-all-environments { display: inline-flex; align-items: center; gap: 5px; color: var(--workflow-secondary); font-weight: 600; }
.workflow-all-environments .el-icon { color: #2563eb; }

.workflow-node-chain { display: flex; min-width: 0; align-items: center; gap: 4px; overflow: hidden; white-space: nowrap; }
.workflow-node-chip { display: inline-flex; min-width: 0; max-width: 96px; align-items: center; gap: 4px; overflow: hidden; color: #334155; text-overflow: ellipsis; white-space: nowrap; }
.workflow-node-chip b { display: inline-grid; width: 20px; height: 20px; flex: 0 0 20px; place-items: center; border: 1px solid #cfe0f5; border-radius: 5px; color: #2563eb; background: #f6faff; font-family: Consolas, monospace; font-size: 10px; }
.workflow-node-arrow { flex: 0 0 auto; color: #9aacc1; font-size: 12px; }
.workflow-node-more { flex: 0 0 auto; color: #2563eb; font-family: Consolas, monospace; font-size: 11px; font-weight: 700; }
.workflow-maintainer-cell { display: grid; gap: 3px; }
.workflow-maintainer-cell span { display: flex; min-width: 0; align-items: center; gap: 5px; overflow: hidden; color: #334155; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.workflow-maintainer-cell span .el-icon { flex: 0 0 auto; color: #64748b; }
.workflow-maintainer-cell time { color: var(--workflow-muted); font-family: Consolas, "SFMono-Regular", monospace; font-size: 10.5px; font-variant-numeric: tabular-nums; }
.workflow-status-control { display: inline-flex; align-items: center; gap: 7px; }
.workflow-status-control span { font-size: 11px; font-weight: 700; }
.workflow-status-control .is-enabled { color: #047857; }
.workflow-status-control .is-disabled { color: #64748b; }
.workflow-row-actions { display: flex; justify-content: flex-end; gap: 2px; white-space: nowrap; }
.workflow-row-actions .el-button { min-height: 32px; margin: 0; padding-inline: 5px; }

.workflow-empty { display: grid; min-height: 220px; place-items: center; align-content: center; gap: 7px; color: var(--workflow-muted); }
.workflow-empty > .el-icon { color: #8aa5c6; font-size: 34px; }
.workflow-empty strong { color: #334155; font-size: 13px; }
.workflow-empty span { max-width: 420px; font-size: 12px; text-align: center; }
.workflow-empty .el-button { margin-top: 3px; }
.workflow-pagination { display: flex; min-height: 32px; flex: 0 0 auto; align-items: center; justify-content: flex-end; }

.workflow-dialog-title { display: flex; align-items: center; gap: 10px; }
.workflow-dialog-title__icon { display: grid; width: 38px; height: 38px; flex: 0 0 38px; place-items: center; border-radius: 9px; color: #2563eb; background: #eef6ff; font-size: 20px; }
.workflow-dialog-title h2 { margin: 0; color: var(--workflow-heading); font-size: 17px; line-height: 1.3; }
.workflow-dialog-title p { margin: 2px 0 0; color: var(--workflow-muted); font-size: 11px; }
.workflow-form { min-width: 0; }
.workflow-form-section { padding: 0 0 16px; border-bottom: 1px solid var(--workflow-border-soft); }
.workflow-form-section + .workflow-form-section { padding-top: 16px; }
.workflow-form-section:last-child { padding-bottom: 0; border-bottom: 0; }
.workflow-form-section__heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; margin-bottom: 10px; }
.workflow-form-section__heading h3 { margin: 0; color: var(--workflow-heading); font-size: 14px; line-height: 1.4; }
.workflow-form-section__heading p { margin: 2px 0 0; color: var(--workflow-muted); font-size: 11px; line-height: 1.45; }
.workflow-form-section__heading .el-button { min-height: 30px; margin: 0; }
.workflow-form-grid { display: grid; grid-template-columns: 1.15fr 1fr 1fr; gap: 0 12px; }
.workflow-form-full { grid-column: 1 / -1; }
.workflow-form :deep(.el-form-item) { margin-bottom: 12px; }
.workflow-form :deep(.el-form-item__label) { height: auto; padding: 0 0 5px; color: #334155; font-size: 12px; font-weight: 700; line-height: 1.4; }
.workflow-form :deep(.el-select),
.workflow-form :deep(.el-input) { width: 100%; }
.workflow-form :deep(.el-input__wrapper),
.workflow-form :deep(.el-select__wrapper),
.workflow-form :deep(.el-textarea__inner) { border-radius: 7px; background: #fbfdff; box-shadow: 0 0 0 1px #d9e5f2 inset; }
.workflow-form :deep(.el-input__wrapper:hover),
.workflow-form :deep(.el-select__wrapper:hover),
.workflow-form :deep(.el-textarea__inner:hover) { box-shadow: 0 0 0 1px #b9d4ff inset; }
.workflow-form :deep(.el-input__wrapper.is-focus),
.workflow-form :deep(.el-select__wrapper.is-focused),
.workflow-form :deep(.el-textarea__inner:focus) { box-shadow: 0 0 0 1px #409eff inset; }

.workflow-policy-list { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); overflow: hidden; border: 1px solid var(--workflow-border); border-radius: 8px; }
.workflow-policy-item { display: grid; min-width: 0; min-height: 62px; grid-template-columns: 32px minmax(0, 1fr) auto; align-items: center; gap: 9px; padding: 8px 11px; background: #fbfdff; cursor: pointer; }
.workflow-policy-item + .workflow-policy-item { border-left: 1px solid var(--workflow-border-soft); }
.workflow-policy-item__icon { display: grid; width: 32px; height: 32px; place-items: center; border-radius: 7px; color: #2563eb; background: #eef6ff; }
.workflow-policy-item__icon.is-warning { color: #9a5b00; background: #fff4e5; }
.workflow-policy-item > span:nth-child(2) { display: grid; min-width: 0; gap: 2px; }
.workflow-policy-item strong { color: #334155; font-size: 12px; }
.workflow-policy-item small { overflow: hidden; color: var(--workflow-muted); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }

.workflow-node-editor { overflow: hidden; border: 1px solid var(--workflow-border); border-radius: 8px; background: #fff; }
.workflow-node-editor__columns,
.workflow-node-config__main { display: grid; grid-template-columns: 46px minmax(130px, 1fr) 126px minmax(180px, 1.35fr) 116px 108px; align-items: center; gap: 8px; }
.workflow-node-editor__columns { min-height: 32px; padding: 0 8px; color: #53657d; background: #f3f7fc; font-size: 10.5px; font-weight: 700; }
.workflow-node-config { padding: 8px; border-top: 1px solid var(--workflow-border-soft); background: #fff; }
.workflow-node-config.is-invalid { background: #fffafa; }
.workflow-node-index { display: grid; width: 26px; height: 26px; justify-self: center; place-items: center; border: 1px solid #cfe0f5; border-radius: 6px; color: #2563eb; background: #f6faff; font-family: Consolas, monospace; font-size: 11px; font-weight: 700; }
.workflow-node-config__actions { display: flex; align-items: center; justify-content: flex-end; gap: 1px; }
.workflow-node-config__actions .el-button { width: 30px; height: 30px; margin: 0; padding: 0; }
.workflow-node-config__errors { display: flex; flex-wrap: wrap; gap: 6px 12px; margin: 6px 0 0 54px; color: #c81e1e; font-size: 11px; }
.workflow-node-config__errors span { display: inline-flex; align-items: center; gap: 4px; }
.workflow-node-empty { display: grid; min-height: 150px; place-items: center; align-content: center; gap: 6px; border: 1px dashed #c9d9ea; border-radius: 8px; color: var(--workflow-muted); background: #fbfdff; text-align: center; }
.workflow-node-empty > .el-icon { color: #8aa5c6; font-size: 28px; }
.workflow-node-empty strong { color: #334155; font-size: 12px; }
.workflow-node-empty span { font-size: 11px; }
.workflow-node-empty .el-button { margin-top: 4px; }
.workflow-dialog-footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.workflow-dialog-footer > span { color: var(--workflow-muted); font-size: 11px; }
.workflow-dialog-footer > div { display: flex; gap: 8px; }

:global(.workflow-editor-dialog-overlay .el-dialog) { max-width: calc(100vw - 32px); overflow: hidden; border-radius: 12px !important; box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18); }
:global(.workflow-editor-dialog .el-dialog__header) { margin-right: 0; padding: 15px 18px 12px; border-bottom: 1px solid #e3edf8; }
:global(.workflow-editor-dialog .el-dialog__body) { max-height: calc(100dvh - 190px); padding: 14px 18px; overflow-y: auto; }
:global(.workflow-editor-dialog .el-dialog__footer) { padding: 11px 18px 14px; border-top: 1px solid #e3edf8; }

@media (max-width: 1380px) {
  .workflow-toolbar { align-items: flex-start; flex-direction: column; }
  .workflow-toolbar__controls { width: 100%; justify-content: space-between; }
}

@media (max-width: 980px) {
  .workflow-summary { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .workflow-summary__item:nth-child(2) { border-right: 0; }
  .workflow-summary__item:nth-child(-n + 2) { border-bottom: 1px solid var(--workflow-border-soft); }
  .workflow-toolbar__controls { align-items: flex-start; flex-direction: column; }
  .workflow-toolbar__filters { width: 100%; justify-content: flex-start; flex-wrap: wrap; }
  .workflow-form-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .workflow-node-editor { overflow-x: auto; }
  .workflow-node-editor__columns,
  .workflow-node-config { min-width: 850px; }
}

@media (max-width: 720px) {
  .workflow-summary { grid-template-columns: 1fr; }
  .workflow-summary__item,
  .workflow-summary__item:nth-child(2) { border-right: 0; border-bottom: 1px solid var(--workflow-border-soft); }
  .workflow-summary__item:last-child { border-bottom: 0; }
  .workflow-toolbar__filters { display: grid; grid-template-columns: 1fr 1fr; }
  .workflow-filter--keyword { width: 100%; grid-column: 1 / -1; }
  .workflow-filter--status { width: 100%; }
  .workflow-toolbar__filters .el-button { width: 100%; }
  .workflow-toolbar__actions { width: 100%; }
  .workflow-toolbar__actions .el-button { flex: 1; }
  .workflow-pagination { justify-content: flex-start; overflow-x: auto; }
  .workflow-form-grid,
  .workflow-policy-list { grid-template-columns: 1fr; }
  .workflow-form-full { grid-column: auto; }
  .workflow-policy-item + .workflow-policy-item { border-top: 1px solid var(--workflow-border-soft); border-left: 0; }
}

@media (max-width: 560px) {
  .workflow-toolbar__filters { grid-template-columns: 1fr; }
  .workflow-filter--keyword { grid-column: auto; }
  .workflow-dialog-title p { display: none; }
  .workflow-dialog-footer { align-items: stretch; flex-direction: column; }
  .workflow-dialog-footer > div { display: grid; grid-template-columns: 1fr 1fr; }
  :global(.workflow-editor-dialog-overlay .el-dialog) { width: calc(100% - 24px) !important; max-width: none; margin: 12px auto !important; }
  :global(.workflow-editor-dialog .el-dialog__body) { max-height: calc(100dvh - 164px); padding: 12px 14px; }
}

@media (prefers-reduced-motion: reduce) {
  .workflow-view *,
  .workflow-view *::before,
  .workflow-view *::after { transition-duration: 0.01ms !important; animation-duration: 0.01ms !important; animation-iteration-count: 1 !important; }
}
</style>
