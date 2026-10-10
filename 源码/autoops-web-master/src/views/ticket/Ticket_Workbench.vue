<template>
  <section class="ticket-view workbench-view" aria-labelledby="approval-workbench-title">
    <div class="workbench-summary" aria-label="工单概览">
      <button
        v-for="item in summaryItems"
        :key="item.key"
        type="button"
        class="workbench-summary__item"
        :class="`is-${item.tone}`"
        :aria-label="`${item.label} ${overview[item.key] || 0}，${item.actionLabel}`"
        @click="handleSummary(item)"
      >
        <span class="workbench-summary__icon" aria-hidden="true">
          <el-icon><component :is="item.icon" /></el-icon>
        </span>
        <span class="workbench-summary__content">
          <span class="workbench-summary__label">{{ item.label }}</span>
          <span class="workbench-summary__hint">{{ item.hint }}</span>
        </span>
        <strong>{{ formatCount(overview[item.key]) }}</strong>
        <el-icon class="workbench-summary__arrow" aria-hidden="true"><ArrowRight /></el-icon>
      </button>
    </div>

    <div class="workbench-toolbar">
      <div class="workbench-toolbar__heading">
        <div>
          <h2 id="approval-workbench-title">我的审批任务</h2>
          <p>{{ resultSummary }}</p>
        </div>
      </div>
      <div class="workbench-toolbar__filters" role="search" aria-label="审批任务筛选">
        <el-input
          v-model="query.keyword"
          class="workbench-filter--keyword"
          clearable
          placeholder="工单号 / 流程名称"
          aria-label="按工单号或流程名称查询"
          @keyup.enter="search"
          @clear="search"
        >
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-select
          v-model="query.businessType"
          class="workbench-filter--select"
          clearable
          placeholder="全部工单类型"
          aria-label="按工单类型筛选"
        >
          <el-option v-for="(label, value) in businessLabels" :key="value" :label="label" :value="value" />
        </el-select>
        <el-select
          v-model="query.status"
          class="workbench-filter--select"
          clearable
          placeholder="全部审批状态"
          aria-label="按审批状态筛选"
        >
          <el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
        </el-select>
        <el-button type="primary" :loading="loading" @click="search">
          <el-icon><Search /></el-icon>查询
        </el-button>
        <el-button type="warning" @click="resetFilters">
          <el-icon><RefreshRight /></el-icon>重置
        </el-button>
        <el-tooltip content="刷新审批任务和概览" placement="top">
          <el-button
            class="workbench-refresh-btn"
            type="warning"
            :loading="loading"
            aria-label="刷新审批任务和概览"
            @click="load"
          >
            <el-icon><Refresh /></el-icon>刷新
          </el-button>
        </el-tooltip>
      </div>
    </div>

    <div v-if="overviewError" class="workbench-notice" role="status">
      <el-icon><Warning /></el-icon>
      <span>{{ overviewError }}</span>
      <el-button link type="primary" @click="loadOverview">重试概览</el-button>
    </div>

    <div class="workbench-table-wrap" v-loading="loading">
      <el-table
        :data="rows"
        height="100%"
        table-layout="fixed"
        row-key="id"
        :row-class-name="tableRowClass"
      >
        <el-table-column label="业务单号" min-width="178" fixed="left">
          <template #default="{ row }">
            <div class="workbench-business-cell">
              <button type="button" class="ticket-link" :title="businessNo(row)" @click="openBusiness(row)">
                {{ businessNo(row) }}
              </button>
              <span>业务 ID：{{ row.businessId || '-' }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="工单类型" width="112">
          <template #default="{ row }">
            <el-tag :type="businessTypeTag(row.businessType)" effect="light" size="small">
              {{ businessLabels[row.businessType] || row.businessType || '未知类型' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="definitionName" label="审批流" min-width="168" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="workbench-flow-name">{{ row.definitionName || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="当前节点" min-width="180">
          <template #default="{ row }">
            <div class="workbench-node-cell">
              <span class="workbench-node-order">{{ row.nodeOrder || '-' }}</span>
              <span :title="row.nodeName">{{ row.nodeName || '-' }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="到达时间" width="172">
          <template #default="{ row }">
            <span class="workbench-time">{{ formatTime(row.createdAt) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="106" align="center">
          <template #default="{ row }">
            <span class="workbench-status" :class="`is-${row.status || 'unknown'}`">
              <el-icon aria-hidden="true"><component :is="statusIcon(row.status)" /></el-icon>
              {{ taskStatus(row.status) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="178" fixed="right" align="right">
          <template #default="{ row }">
            <div class="workbench-actions">
              <template v-if="row.status === 'pending'">
                <el-button link type="success" :loading="actingId === row.id" @click="openApproval(row, true)">通过</el-button>
                <el-button link type="danger" :disabled="Boolean(actingId)" @click="openApproval(row, false)">驳回</el-button>
              </template>
              <el-button v-else link type="primary" @click="openFlow(row)">流程</el-button>
              <el-dropdown v-if="row.status === 'pending'" trigger="click" @command="openFlow(row)">
                <el-button link type="primary" aria-label="更多审批操作">更多<el-icon><ArrowDown /></el-icon></el-button>
                <template #dropdown>
                  <el-dropdown-menu><el-dropdown-item command="flow"><el-icon><Connection /></el-icon>查看审批流程</el-dropdown-item></el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </template>
        </el-table-column>
        <template #empty>
          <div class="workbench-empty">
            <el-icon><DocumentChecked /></el-icon>
            <strong>{{ emptyTitle }}</strong>
            <span>{{ emptyDescription }}</span>
            <el-button v-if="hasActiveFilters" type="primary" plain @click="resetFilters">清除筛选条件</el-button>
          </div>
        </template>
      </el-table>
    </div>

    <div class="workbench-pagination">
      <el-pagination
        v-model:current-page="query.page"
        v-model:page-size="query.pageSize"
        background
        :total="total"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next"
        @current-change="changePage"
        @size-change="sizeChange"
      />
    </div>

    <el-dialog
      v-model="approvalDialog"
      class="workbench-approval-dialog"
      modal-class="workbench-approval-dialog-overlay"
      width="520px"
      destroy-on-close
      :close-on-click-modal="false"
      @closed="resetApproval"
    >
      <template #header>
        <div class="workbench-dialog-title">
          <span class="workbench-dialog-title__icon" :class="approvalForm.approve ? 'is-success' : 'is-danger'">
            <el-icon><component :is="approvalForm.approve ? CircleCheck : CircleClose" /></el-icon>
          </span>
          <div>
            <h2>{{ approvalForm.approve ? '审批通过' : '驳回工单' }}</h2>
            <p>请确认工单与当前审批节点后提交</p>
          </div>
        </div>
      </template>

      <dl class="workbench-approval-context">
        <div><dt>业务单号</dt><dd>{{ businessNo(approvalForm.row) }}</dd></div>
        <div><dt>工单类型</dt><dd>{{ businessLabels[approvalForm.row?.businessType] || '-' }}</dd></div>
        <div><dt>审批流程</dt><dd>{{ approvalForm.row?.definitionName || '-' }}</dd></div>
        <div><dt>当前节点</dt><dd>{{ approvalForm.row?.nodeName || '-' }}</dd></div>
      </dl>

      <el-form ref="approvalFormRef" :model="approvalForm" :rules="approvalRules" label-position="top" class="workbench-approval-form">
        <el-form-item :label="approvalForm.approve ? '审批意见' : '驳回原因'" prop="comment">
          <el-input
            v-model="approvalForm.comment"
            type="textarea"
            :rows="4"
            maxlength="500"
            show-word-limit
            resize="none"
            :placeholder="approvalForm.approve ? '可填写审批意见' : '请输入明确的驳回原因'"
          />
        </el-form-item>
      </el-form>

      <div v-if="!approvalForm.approve" class="workbench-dialog-warning" role="note">
        <el-icon><Warning /></el-icon>
        <span>驳回后流程将结束，申请人可在对应业务工单中查看原因。</span>
      </div>

      <template #footer>
        <div class="workbench-dialog-footer">
          <el-button :disabled="approvalSubmitting" @click="approvalDialog = false">取消</el-button>
          <el-button
            :type="approvalForm.approve ? 'primary' : 'danger'"
            :loading="approvalSubmitting"
            @click="submitApproval"
          >
            {{ approvalForm.approve ? '确认通过' : '确认驳回' }}
          </el-button>
        </div>
      </template>
    </el-dialog>

    <el-drawer
      v-model="drawer"
      class="workbench-flow-drawer"
      modal-class="workbench-flow-drawer-overlay"
      size="560px"
      destroy-on-close
      @closed="resetFlow"
    >
      <template #header>
        <div class="workbench-drawer-title">
          <span class="workbench-drawer-title__icon"><el-icon><Connection /></el-icon></span>
          <div><h2>审批进度</h2><p>{{ flow?.instance?.businessNo || businessNo(flowRow) }}</p></div>
        </div>
      </template>

      <div class="workbench-flow-content" v-loading="flowLoading">
        <div v-if="flowError" class="workbench-flow-error" role="alert">
          <el-icon><WarningFilled /></el-icon>
          <strong>审批流程加载失败</strong>
          <span>{{ flowError }}</span>
          <el-button type="primary" plain @click="retryFlow">重新加载</el-button>
        </div>

        <template v-else-if="flow">
          <section class="workbench-flow-overview" aria-labelledby="flow-overview-title">
            <div class="workbench-flow-overview__heading">
              <div>
                <h3 id="flow-overview-title">{{ flow.instance?.definitionName || '审批流程' }}</h3>
                <p>{{ flow.instance?.currentNodeName || '流程已结束' }}</p>
              </div>
              <span class="workbench-status" :class="`is-${flow.instance?.status || 'unknown'}`">
                <el-icon><component :is="statusIcon(flow.instance?.status)" /></el-icon>
                {{ taskStatus(flow.instance?.status) }}
              </span>
            </div>
            <dl class="workbench-flow-meta">
              <div><dt>业务单号</dt><dd>{{ flow.instance?.businessNo || '-' }}</dd></div>
              <div><dt>申请人</dt><dd>{{ flow.instance?.applicantName || '-' }}</dd></div>
              <div><dt>发起时间</dt><dd>{{ formatTime(flow.instance?.createdAt) }}</dd></div>
              <div><dt>当前节点</dt><dd>{{ flow.instance?.currentNodeName || '流程已结束' }}</dd></div>
            </dl>
          </section>

          <section class="workbench-flow-timeline" aria-labelledby="flow-nodes-title">
            <div class="workbench-section-heading">
              <h3 id="flow-nodes-title">审批节点</h3>
              <span>共 {{ (flow.tasks || []).length }} 个节点</span>
            </div>
            <el-timeline v-if="flow.tasks?.length">
              <el-timeline-item
                v-for="task in flow.tasks"
                :key="task.id"
                :timestamp="formatTime(task.actedAt || task.createdAt)"
                :type="timelineType(task.status)"
                :hollow="task.status === 'pending'"
              >
                <div class="workbench-timeline-item">
                  <div class="workbench-timeline-item__heading">
                    <strong>{{ task.nodeOrder }}. {{ task.nodeName }}</strong>
                    <span class="workbench-status is-compact" :class="`is-${task.status || 'unknown'}`">
                      {{ taskStatus(task.status) }}
                    </span>
                  </div>
                  <div class="workbench-timeline-item__assignee">
                    <el-icon><User /></el-icon>{{ task.assigneeName || '未指定审批人' }}
                  </div>
                  <p v-if="task.comment">{{ task.comment }}</p>
                </div>
              </el-timeline-item>
            </el-timeline>
            <el-empty v-else :image-size="64" description="暂无审批节点" />
          </section>
        </template>
      </div>
    </el-drawer>
  </section>
</template>

<script setup>
/* global defineEmits, defineExpose */
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  ArrowDown,
  ArrowRight,
  CircleCheck,
  CircleClose,
  Clock,
  Connection,
  DataAnalysis,
  DocumentChecked,
  Promotion,
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
import { businessLabels, ensure, formatTime, listResult } from './ticketCommon'

const emit = defineEmits(['open-tab', 'open-business'])
const router = useRouter()

const loading = ref(false)
const rows = ref([])
const total = ref(0)
const overviewError = ref('')
const drawer = ref(false)
const flow = ref(null)
const flowRow = ref(null)
const flowLoading = ref(false)
const flowError = ref('')
const actingId = ref(null)
const approvalDialog = ref(false)
const approvalSubmitting = ref(false)
const approvalFormRef = ref()

const overview = reactive({ pendingApprovals: 0, dailyOpen: 0, releaseActive: 0, sqlPending: 0 })
const query = reactive({ page: 1, pageSize: 10, keyword: '', businessType: '', status: 'pending' })
const approvalForm = reactive({ approve: true, comment: '', row: null })

const statusOptions = [
  { label: '待审批', value: 'pending' },
  { label: '已通过', value: 'approved' },
  { label: '已驳回', value: 'rejected' }
]

const summaryItems = computed(() => [
  { key: 'pendingApprovals', label: '我的待审批', hint: '需要我处理', icon: Stamp, tone: 'warning', action: 'pending', actionLabel: '筛选待审批任务' },
  { key: 'dailyOpen', label: '进行中事务', hint: '日常事务处理中', icon: Tickets, tone: 'primary', action: 'daily', actionLabel: '打开日常事务' },
  { key: 'releaseActive', label: '发布流程中', hint: '应用发布进行中', icon: Promotion, tone: 'success', action: 'release', actionLabel: '打开应用发布' },
  { key: 'sqlPending', label: 'SQL 待审批', hint: '数据库变更待审', icon: DataAnalysis, tone: 'danger', action: 'sql', actionLabel: '打开 SQL 审核' }
])

const approvalRules = computed(() => ({
  comment: [{
    validator: (_, value, callback) => {
      if (!approvalForm.approve && !String(value || '').trim()) callback(new Error('请输入驳回原因'))
      else callback()
    },
    trigger: 'blur'
  }]
}))

const hasActiveFilters = computed(() => Boolean(query.keyword || query.businessType || (query.status && query.status !== 'pending')))
const resultSummary = computed(() => {
  const status = query.status ? taskStatus(query.status) : '全部状态'
  return `${status}，共 ${formatCount(total.value)} 条`
})
const emptyTitle = computed(() => hasActiveFilters.value ? '没有匹配的审批任务' : query.status === 'pending' ? '当前没有待审批任务' : '暂无审批任务')
const emptyDescription = computed(() => hasActiveFilters.value ? '请调整工单类型、状态或关键字后重试' : '新的审批任务到达后会显示在这里')

const taskStatus = status => ({
  pending: '待审批',
  running: '审批中',
  approved: '已通过',
  rejected: '已驳回',
  canceled: '已取消'
}[status] || status || '未知状态')

const statusIcon = status => ({
  pending: Clock,
  running: Clock,
  approved: CircleCheck,
  rejected: WarningFilled,
  canceled: Warning
}[status] || Warning)

const timelineType = status => ({ approved: 'success', rejected: 'danger', pending: 'primary', canceled: 'info' }[status] || 'primary')
const businessTypeTag = type => ({ daily: 'primary', release: 'success', sql_review: 'warning' }[type] || 'info')
const businessNo = row => row?.businessNo || (row?.businessId ? `#${row.businessId}` : '-')
const formatCount = value => Number(value || 0).toLocaleString('zh-CN')
const tableRowClass = ({ row }) => row.status === 'pending' ? 'is-pending-row' : ''

async function loadOverview() {
  overviewError.value = ''
  try {
    Object.assign(overview, ensure(await ticketApi.overview(), '加载工单概览失败'))
  } catch (error) {
    overviewError.value = error.message || '工单概览加载失败，请重试'
  }
}

async function loadTasks() {
  const result = listResult(await ticketApi.myApprovalTasks(query))
  rows.value = result.list
  total.value = result.total
}

async function load() {
  loading.value = true
  const results = await Promise.allSettled([loadOverview(), loadTasks()])
  if (results[1].status === 'rejected') {
    ElMessage.error(results[1].reason?.message || '加载审批任务失败')
  }
  loading.value = false
}

function search() {
  query.page = 1
  load()
}

function sizeChange() {
  query.page = 1
  changePage()
}

async function changePage() {
  loading.value = true
  try {
    await loadTasks()
  } catch (error) {
    ElMessage.error(error.message || '加载审批任务失败')
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  Object.assign(query, { page: 1, keyword: '', businessType: '', status: 'pending' })
  load()
}

function handleSummary(item) {
  if (item.action === 'pending') {
    query.status = 'pending'
    query.page = 1
    load()
    return
  }
  if (item.action === 'sql') {
    router.push('/db/review')
    return
  }
  emit('open-tab', item.action)
}

function openApproval(row, approve) {
  approvalForm.approve = approve
  approvalForm.comment = ''
  approvalForm.row = row
  approvalDialog.value = true
}

async function submitApproval() {
  const valid = await approvalFormRef.value?.validate().catch(() => false)
  if (!valid || !approvalForm.row) return

  approvalSubmitting.value = true
  actingId.value = approvalForm.row.id
  try {
    const payload = { comment: approvalForm.comment.trim() }
    const response = approvalForm.approve
      ? await ticketApi.approveTask(approvalForm.row.id, payload)
      : await ticketApi.rejectTask(approvalForm.row.id, payload)
    ensure(response, approvalForm.approve ? '审批通过失败' : '驳回工单失败')
    ElMessage.success(approvalForm.approve ? '审批已通过' : '工单已驳回')
    approvalDialog.value = false
    await load()
  } catch (error) {
    ElMessage.error(error.message || '审批操作失败')
  } finally {
    approvalSubmitting.value = false
    actingId.value = null
  }
}

function resetApproval() {
  Object.assign(approvalForm, { approve: true, comment: '', row: null })
  approvalFormRef.value?.clearValidate()
}

async function openFlow(row) {
  flowRow.value = row
  flow.value = null
  flowError.value = ''
  drawer.value = true
  flowLoading.value = true
  try {
    flow.value = ensure(await ticketApi.workflowInstance(row.instanceId), '加载审批流程失败')
  } catch (error) {
    flowError.value = error.message || '审批流程加载失败，请重试'
  } finally {
    flowLoading.value = false
  }
}

function retryFlow() {
  if (flowRow.value) openFlow(flowRow.value)
}

function resetFlow() {
  flow.value = null
  flowRow.value = null
  flowError.value = ''
  flowLoading.value = false
}

const openBusiness = row => emit('open-business', row)

onMounted(load)
defineExpose({ load })
</script>

<style scoped>
.workbench-view {
  --workbench-heading: #0f172a;
  --workbench-text: #172033;
  --workbench-secondary: #53657d;
  --workbench-muted: #64748b;
  --workbench-border: #dbe7f4;
  --workbench-border-soft: #e7eef7;
  --workbench-surface: #ffffff;
  --workbench-surface-soft: #fbfdff;
}

.workbench-summary {
  display: grid;
  flex: 0 0 auto;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid var(--workbench-border);
  border-radius: 10px;
  background: var(--workbench-surface);
}

.workbench-summary__item {
  position: relative;
  display: grid;
  min-width: 0;
  min-height: 70px;
  grid-template-columns: 34px minmax(0, 1fr) auto 16px;
  align-items: center;
  gap: 9px;
  padding: 9px 10px;
  border: 0;
  border-right: 1px solid var(--workbench-border-soft);
  color: var(--workbench-text);
  background: transparent;
  text-align: left;
  cursor: pointer;
  transition: background-color 160ms ease, box-shadow 160ms ease;
}

.workbench-summary__item:last-child { border-right: 0; }
.workbench-summary__item:hover { background: #f7faff; }
.workbench-summary__item:active { background: #eef6ff; }
.workbench-summary__item:focus-visible { z-index: 1; outline: 2px solid #409eff; outline-offset: -2px; }

.workbench-summary__icon {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border-radius: 9px;
  color: #2563eb;
  background: #eef6ff;
  font-size: 17px;
}

.workbench-summary__item.is-warning .workbench-summary__icon { color: #a15c00; background: #fff4e5; }
.workbench-summary__item.is-success .workbench-summary__icon { color: #047857; background: #eaf8ef; }
.workbench-summary__item.is-danger .workbench-summary__icon { color: #c81e1e; background: #fff0f0; }

.workbench-summary__content { display: grid; min-width: 0; gap: 3px; }
.workbench-summary__label { overflow: hidden; color: var(--workbench-secondary); font-size: 12px; font-weight: 700; text-overflow: ellipsis; white-space: nowrap; }
.workbench-summary__hint { overflow: hidden; color: var(--workbench-muted); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; }
.workbench-summary__item strong { color: var(--workbench-heading); font-family: Consolas, "SFMono-Regular", monospace; font-size: 21px; font-weight: 700; font-variant-numeric: tabular-nums; }
.workbench-summary__arrow { color: #94a3b8; transition: transform 160ms ease, color 160ms ease; }
.workbench-summary__item:hover .workbench-summary__arrow { color: #2563eb; transform: translateX(1px); }

.workbench-toolbar {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
  padding: 2px 0;
}

.workbench-toolbar__heading { flex: 0 0 auto; }
.workbench-toolbar__heading h2 { margin: 0; color: var(--workbench-heading); font-size: 15px; line-height: 1.3; }
.workbench-toolbar__heading p { margin: 2px 0 0; color: var(--workbench-muted); font-size: 11px; }

.workbench-toolbar__filters {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  gap: 7px;
}

.workbench-filter--keyword { width: 220px; }
.workbench-filter--select { width: 144px; }
.workbench-toolbar__filters :deep(.el-input__wrapper),
.workbench-toolbar__filters :deep(.el-select__wrapper) { min-height: 34px; border-radius: 7px; background: #fbfdff; box-shadow: 0 0 0 1px #d9e5f2 inset; }
.workbench-toolbar__filters :deep(.el-input__wrapper:hover),
.workbench-toolbar__filters :deep(.el-select__wrapper:hover) { box-shadow: 0 0 0 1px #b9d4ff inset; }
.workbench-toolbar__filters :deep(.el-input__wrapper.is-focus),
.workbench-toolbar__filters :deep(.el-select__wrapper.is-focused) { box-shadow: 0 0 0 1px #409eff inset; }
.workbench-toolbar__filters .el-button { min-height: 34px; margin: 0; font-weight: 600; }
.workbench-refresh-btn { border-color: #e5b94e !important; background: #e5b94e !important; color: #fff !important; }

.workbench-notice {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 7px;
  min-height: 32px;
  box-sizing: border-box;
  padding: 5px 9px;
  border: 1px solid #f3d59b;
  border-radius: 8px;
  color: #854d0e;
  background: #fff9ed;
  font-size: 12px;
}

.workbench-notice span { min-width: 0; flex: 1; }

.workbench-table-wrap {
  flex: 1;
  min-height: 280px;
  overflow: hidden;
  border: 1px solid #e3ebf5;
  border-radius: 8px;
  background: var(--workbench-surface);
}

.workbench-table-wrap :deep(.el-table) { height: 100%; color: var(--workbench-text); }
.workbench-table-wrap :deep(.el-table th.el-table__cell) { height: 38px; padding: 0; color: #334155; background: #f3f7fc; font-size: 12px; font-weight: 700; }
.workbench-table-wrap :deep(.el-table td.el-table__cell) { height: 42px; padding: 0; border-bottom-color: #eef3f9; }
.workbench-table-wrap :deep(.el-table .cell) { font-size: 12px; line-height: 1.45; }
.workbench-table-wrap :deep(.el-table__body tr:hover > td.el-table__cell) { background: #f8fafc; }
.workbench-table-wrap :deep(.el-table__body tr.is-pending-row > td.el-table__cell:first-child) { box-shadow: 3px 0 0 #f59e0b inset; }
.workbench-table-wrap :deep(.el-table__empty-block) { min-height: 250px; }

.workbench-business-cell { display: grid; min-width: 0; gap: 2px; }
.workbench-business-cell .ticket-link { overflow: hidden; text-align: left; text-overflow: ellipsis; white-space: nowrap; }
.workbench-business-cell span { overflow: hidden; color: var(--workbench-muted); font-size: 10.5px; text-overflow: ellipsis; white-space: nowrap; }
.workbench-flow-name { color: #334155; font-weight: 600; }

.workbench-node-cell { display: flex; min-width: 0; align-items: center; gap: 7px; }
.workbench-node-order { display: grid; width: 22px; height: 22px; flex: 0 0 22px; place-items: center; border: 1px solid #cfe0f5; border-radius: 6px; color: #2563eb; background: #f6faff; font-family: Consolas, monospace; font-size: 11px; font-weight: 700; }
.workbench-node-cell > span:last-child { overflow: hidden; color: #334155; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.workbench-time { color: var(--workbench-secondary); font-family: Consolas, "SFMono-Regular", monospace; font-size: 11px; font-variant-numeric: tabular-nums; }

.workbench-status {
  display: inline-flex;
  min-height: 22px;
  align-items: center;
  justify-content: center;
  gap: 4px;
  box-sizing: border-box;
  padding: 2px 7px;
  border: 1px solid #dbe7f4;
  border-radius: 999px;
  color: #64748b;
  background: #f3f7fc;
  font-size: 11px;
  font-weight: 700;
  white-space: nowrap;
}

.workbench-status.is-pending, .workbench-status.is-running { border-color: #f0d29a; color: #9a5b00; background: #fff4e5; }
.workbench-status.is-approved { border-color: #b9e4cc; color: #047857; background: #eaf8ef; }
.workbench-status.is-rejected { border-color: #f4c4c4; color: #c81e1e; background: #fff0f0; }
.workbench-status.is-canceled { border-color: #d7e0eb; color: #64748b; background: #f3f7fc; }
.workbench-status.is-compact { min-height: 20px; padding: 1px 6px; font-size: 10.5px; }

.workbench-actions { display: flex; min-width: 0; align-items: center; justify-content: flex-end; gap: 2px; white-space: nowrap; }
.workbench-actions .el-button { min-height: 32px; margin: 0; padding-inline: 5px; }
.workbench-actions :deep(.el-dropdown) { vertical-align: middle; }

.workbench-empty { display: grid; min-height: 220px; place-items: center; align-content: center; gap: 7px; color: var(--workbench-muted); }
.workbench-empty > .el-icon { color: #8aa5c6; font-size: 34px; }
.workbench-empty strong { color: #334155; font-size: 13px; }
.workbench-empty span { font-size: 12px; }
.workbench-empty .el-button { margin-top: 3px; }

.workbench-pagination { display: flex; flex: 0 0 auto; min-height: 32px; align-items: center; justify-content: flex-end; }

.workbench-dialog-title, .workbench-drawer-title { display: flex; align-items: center; gap: 10px; }
.workbench-dialog-title__icon, .workbench-drawer-title__icon { display: grid; width: 38px; height: 38px; flex: 0 0 38px; place-items: center; border-radius: 10px; color: #2563eb; background: #eef6ff; font-size: 20px; }
.workbench-dialog-title__icon.is-success { color: #047857; background: #eaf8ef; }
.workbench-dialog-title__icon.is-danger { color: #c81e1e; background: #fff0f0; }
.workbench-dialog-title h2, .workbench-drawer-title h2 { margin: 0; color: var(--workbench-heading); font-size: 17px; line-height: 1.3; }
.workbench-dialog-title p, .workbench-drawer-title p { margin: 2px 0 0; color: var(--workbench-muted); font-size: 11px; }

.workbench-approval-context {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1px;
  margin: 0 0 14px;
  overflow: hidden;
  border: 1px solid var(--workbench-border);
  border-radius: 8px;
  background: var(--workbench-border-soft);
}

.workbench-approval-context > div { min-width: 0; padding: 8px 10px; background: #fbfdff; }
.workbench-approval-context dt { margin-bottom: 3px; color: var(--workbench-muted); font-size: 11px; }
.workbench-approval-context dd { margin: 0; overflow: hidden; color: #334155; font-size: 12px; font-weight: 600; text-overflow: ellipsis; white-space: nowrap; }
.workbench-approval-form :deep(.el-form-item) { margin-bottom: 12px; }
.workbench-approval-form :deep(.el-form-item__label) { color: #334155; font-size: 12px; font-weight: 700; }
.workbench-approval-form :deep(.el-textarea__inner) { border-radius: 8px; background: #fbfdff; box-shadow: 0 0 0 1px #d9e5f2 inset; font-size: 12px; line-height: 1.6; }
.workbench-approval-form :deep(.el-textarea__inner:focus) { box-shadow: 0 0 0 1px #409eff inset; }

.workbench-dialog-warning { display: flex; align-items: flex-start; gap: 7px; padding: 8px 10px; border: 1px solid #f3d59b; border-radius: 8px; color: #854d0e; background: #fff9ed; font-size: 12px; line-height: 1.5; }
.workbench-dialog-warning .el-icon { margin-top: 2px; flex: 0 0 auto; }
.workbench-dialog-footer { display: flex; justify-content: flex-end; gap: 8px; }

.workbench-flow-content { min-height: 260px; }
.workbench-flow-error { display: grid; min-height: 260px; place-items: center; align-content: center; gap: 8px; color: var(--workbench-muted); text-align: center; }
.workbench-flow-error > .el-icon { color: #d97706; font-size: 34px; }
.workbench-flow-error strong { color: #334155; font-size: 14px; }
.workbench-flow-error span { max-width: 320px; font-size: 12px; line-height: 1.6; }

.workbench-flow-overview { padding-bottom: 14px; border-bottom: 1px solid var(--workbench-border-soft); }
.workbench-flow-overview__heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.workbench-flow-overview__heading h3 { margin: 0; color: var(--workbench-heading); font-size: 15px; }
.workbench-flow-overview__heading p { margin: 3px 0 0; color: var(--workbench-muted); font-size: 11px; }
.workbench-flow-meta { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px 16px; margin: 14px 0 0; }
.workbench-flow-meta div { min-width: 0; }
.workbench-flow-meta dt { margin-bottom: 3px; color: var(--workbench-muted); font-size: 11px; }
.workbench-flow-meta dd { margin: 0; overflow-wrap: anywhere; color: #334155; font-size: 12px; font-weight: 600; }

.workbench-flow-timeline { padding-top: 14px; }
.workbench-section-heading { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin-bottom: 10px; }
.workbench-section-heading h3 { margin: 0; color: var(--workbench-heading); font-size: 14px; }
.workbench-section-heading > span { color: var(--workbench-muted); font-size: 11px; }
.workbench-flow-timeline :deep(.el-timeline) { padding-left: 4px; }
.workbench-flow-timeline :deep(.el-timeline-item__timestamp) { color: #7b8aa0; font-family: Consolas, monospace; font-size: 10.5px; }
.workbench-timeline-item { padding: 8px 10px; border: 1px solid var(--workbench-border-soft); border-radius: 8px; background: #fbfdff; }
.workbench-timeline-item__heading { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.workbench-timeline-item__heading strong { color: #334155; font-size: 12px; }
.workbench-timeline-item__assignee { display: flex; align-items: center; gap: 5px; margin-top: 5px; color: var(--workbench-muted); font-size: 11px; }
.workbench-timeline-item p { margin: 7px 0 0; padding-top: 7px; border-top: 1px solid var(--workbench-border-soft); color: var(--workbench-secondary); font-size: 12px; line-height: 1.55; white-space: pre-wrap; }

:global(.workbench-approval-dialog-overlay .el-dialog) { overflow: hidden; border-radius: 15px !important; box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18); }
:global(.workbench-approval-dialog .el-dialog__header) { margin-right: 0; padding: 16px 18px 12px; border-bottom: 1px solid #e3edf8; }
:global(.workbench-approval-dialog .el-dialog__body) { padding: 16px 18px; }
:global(.workbench-approval-dialog .el-dialog__footer) { padding: 12px 18px 16px; border-top: 1px solid #e3edf8; }
:global(.workbench-flow-drawer-overlay .el-drawer) { border-radius: 15px 0 0 15px; }
:global(.workbench-flow-drawer .el-drawer__header) { margin-bottom: 0; padding: 16px 18px 12px; border-bottom: 1px solid #e3edf8; }
:global(.workbench-flow-drawer .el-drawer__body) { padding: 16px 18px; }

@media (max-width: 1280px) {
  .workbench-toolbar { align-items: flex-start; flex-direction: column; }
  .workbench-toolbar__filters { width: 100%; justify-content: flex-start; }
}

@media (max-width: 980px) {
  .workbench-summary { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .workbench-summary__item:nth-child(2) { border-right: 0; }
  .workbench-summary__item:nth-child(-n + 2) { border-bottom: 1px solid var(--workbench-border-soft); }
  .workbench-filter--keyword { width: min(100%, 240px); }
  .workbench-filter--select { width: 150px; }
}

@media (max-width: 720px) {
  .workbench-summary { grid-template-columns: 1fr; }
  .workbench-summary__item, .workbench-summary__item:nth-child(2) { border-right: 0; border-bottom: 1px solid var(--workbench-border-soft); }
  .workbench-summary__item:last-child { border-bottom: 0; }
  .workbench-toolbar__filters { display: grid; grid-template-columns: 1fr 1fr; }
  .workbench-filter--keyword { width: 100%; grid-column: 1 / -1; }
  .workbench-filter--select { width: 100%; }
  .workbench-toolbar__filters .el-button { width: 100%; }
  .workbench-pagination { justify-content: flex-start; overflow-x: auto; }
  .workbench-approval-context, .workbench-flow-meta { grid-template-columns: 1fr; }
}

@media (max-width: 560px) {
  .workbench-toolbar__filters { grid-template-columns: 1fr; }
  .workbench-filter--keyword { grid-column: auto; }
  :global(.workbench-approval-dialog-overlay .el-dialog) { width: calc(100% - 24px) !important; }
  :global(.workbench-flow-drawer-overlay .el-drawer) { width: 100% !important; border-radius: 0; }
}

@media (prefers-reduced-motion: reduce) {
  .workbench-view *, .workbench-view *::before, .workbench-view *::after {
    transition-duration: 0.01ms !important;
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
  }
}
</style>
