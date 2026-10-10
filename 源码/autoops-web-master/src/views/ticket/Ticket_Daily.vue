<template>
  <section class="ticket-view ticket-view--daily">
    <div class="ticket-summary" aria-label="日常事务概览">
      <div v-for="item in summary" :key="item.label" class="ticket-summary__item" :class="`is-${item.tone}`">
        <span class="ticket-summary__icon" aria-hidden="true"><el-icon><component :is="item.icon" /></el-icon></span>
        <span class="ticket-summary__content"><span>{{ item.label }}</span><strong>{{ item.value }}</strong></span>
      </div>
    </div>

    <div class="ticket-toolbar ticket-toolbar--main">
      <div class="ticket-toolbar__filters">
        <el-input v-model="query.keyword" class="ticket-filter-input" clearable placeholder="工单号 / 标题" @keyup.enter="search">
          <template #prefix><el-icon><Search /></el-icon></template>
        </el-input>
        <el-select v-model="query.category" class="ticket-filter-select" clearable placeholder="事务类型" @change="search">
          <el-option v-for="(label, value) in dailyCategoryLabels" :key="value" :label="label" :value="value" />
        </el-select>
        <el-select v-model="query.status" class="ticket-filter-select" clearable placeholder="处理状态" @change="search">
          <el-option v-for="(label, value) in dailyStatusLabels" :key="value" :label="label" :value="value" />
        </el-select>
        <el-select v-model="query.scope" class="ticket-filter-select" clearable placeholder="工单范围" @change="search">
          <el-option label="我发起的" value="created" />
          <el-option label="我处理的" value="handled" />
        </el-select>
      </div>
      <div class="ticket-toolbar__actions">
        <el-button size="small" type="primary" :icon="Search" @click="search">查询</el-button>
        <el-button size="small" type="warning" :icon="RefreshRight" @click="resetQuery">重置</el-button>
        <el-button size="small" type="warning" class="ops-refresh-button" :icon="Refresh" :loading="loading" @click="load">刷新</el-button>
        <el-button size="small" type="success" :icon="Plus" @click="openCreate">新建事务</el-button>
      </div>
    </div>

    <div class="ticket-table-wrap">
      <el-table v-loading="loading" :data="rows" height="100%" size="small" row-key="id" empty-text="暂无日常事务工单">
        <el-table-column label="工单" min-width="240">
          <template #default="{ row }">
            <button type="button" class="ticket-link" @click="openDetail(row.id)">{{ row.ticketNo }}</button>
            <el-tooltip :content="row.title" placement="top-start" :show-after="400">
              <div class="ticket-cell-sub ticket-cell-sub--ellipsis">{{ row.title }}</div>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column label="类型" width="110"><template #default="{ row }">{{ dailyCategoryLabels[row.category] || row.category }}</template></el-table-column>
        <el-table-column label="环境" width="90"><template #default="{ row }">{{ environmentLabels[row.environment] || row.environment }}</template></el-table-column>
        <el-table-column label="优先级" width="90"><template #default="{ row }"><el-tag size="small" :type="priorityTypes[row.priority]">{{ priorityLabels[row.priority] || row.priority }}</el-tag></template></el-table-column>
        <el-table-column prop="applicantName" label="申请人" width="110" show-overflow-tooltip />
        <el-table-column label="处理人" width="110" show-overflow-tooltip><template #default="{ row }">{{ row.handlerName || '待分派' }}</template></el-table-column>
        <el-table-column label="状态" width="100"><template #default="{ row }"><el-tag size="small" :type="statusType(row.status)">{{ dailyStatusLabels[row.status] || row.status }}</el-tag></template></el-table-column>
        <el-table-column label="提交时间" min-width="155"><template #default="{ row }">{{ formatTime(row.createdAt) }}</template></el-table-column>
        <el-table-column label="操作" width="210" fixed="right" align="right">
          <template #default="{ row }">
            <div class="ticket-actions">
              <button type="button" class="ticket-action" @click="openDetail(row.id)">详情</button>
              <button v-if="canAssign(row)" type="button" class="ticket-action" @click="openAssign(row)">分派</button>
              <button v-if="row.status === 'assigned' || row.status === 'approved'" type="button" class="ticket-action warning" @click="perform(row, 'start')">开始</button>
              <button v-if="row.status === 'processing'" type="button" class="ticket-action success" @click="perform(row, 'resolve')">解决</button>
              <button v-if="row.status === 'resolved'" type="button" class="ticket-action success" @click="perform(row, 'confirm')">确认</button>
              <el-dropdown v-if="canMore(row)" trigger="click">
                <button type="button" class="ticket-action">更多</button>
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item v-if="row.status === 'confirmed'" @click="perform(row, 'close')">关闭工单</el-dropdown-item>
                    <el-dropdown-item v-if="canCancel(row)" divided @click="perform(row, 'cancel')">撤销工单</el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div class="ticket-pagination">
      <el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" :total="total" :page-sizes="[10, 20, 50]" layout="total, sizes, prev, pager, next" @current-change="load" @size-change="sizeChange" />
    </div>

    <el-dialog v-model="createDialog" class="ticket-dialog ticket-dialog--form" modal-class="ticket-dialog-overlay" width="820px" destroy-on-close @closed="resetForm">
      <template #header>
        <div class="ticket-dialog-title">
          <span class="ticket-dialog-title__icon"><el-icon><Plus /></el-icon></span>
          <div><h2>新建日常事务工单</h2><p>填写事务范围、审批流程与处理要求</p></div>
        </div>
      </template>
      <el-form ref="formRef" :model="form" :rules="rules" label-width="96px" class="ticket-form">
        <div class="ticket-form-grid">
          <el-form-item label="事务类型" prop="category"><el-select v-model="form.category" @change="categoryChange"><el-option v-for="(label, value) in dailyCategoryLabels" :key="value" :label="label" :value="value" /></el-select></el-form-item>
          <el-form-item label="工单标题" prop="title"><el-input v-model="form.title" maxlength="256" /></el-form-item>
          <el-form-item label="运行环境" prop="environment"><el-select v-model="form.environment"><el-option v-for="(label, value) in environmentLabels" :key="value" :label="label" :value="value" /></el-select></el-form-item>
          <el-form-item label="优先级" prop="priority"><el-select v-model="form.priority"><el-option v-for="(label, value) in priorityLabels" :key="value" :label="label" :value="value" /></el-select></el-form-item>
          <el-form-item label="审批流程" :prop="form.category === 'ops_issue' ? '' : 'workflowId'"><el-select v-model="form.workflowId" clearable placeholder="运维问题可不审批"><el-option v-for="item in applicableWorkflows" :key="item.id" :label="workflowOptionLabel(item)" :value="item.id" :disabled="Boolean(workflowConflict(item))" /></el-select></el-form-item>
          <el-form-item label="期望完成"><el-date-picker v-model="form.expectedAt" type="datetime" value-format="YYYY-MM-DD HH:mm:ss" placeholder="选择时间" /></el-form-item>
          <el-form-item v-if="form.category === 'ops_issue'" label="问题描述" class="ticket-form-full" prop="impactScope"><el-input v-model="form.impactScope" type="textarea" :rows="3" maxlength="1000" show-word-limit placeholder="请描述影响对象、故障现象、复现情况和联系方式" /></el-form-item>
          <el-form-item v-else label="影响范围" class="ticket-form-full"><el-input v-model="form.impactScope" placeholder="受影响系统、用户或服务" /></el-form-item>

          <template v-if="form.category === 'permission'">
            <el-form-item label="目标系统"><el-input v-model="form.formData.system" /></el-form-item>
            <el-form-item label="有效期限"><el-input v-model="form.formData.duration" placeholder="例如 30 天" /></el-form-item>
            <el-form-item label="权限范围" class="ticket-form-full"><el-input v-model="form.formData.permissionScope" /></el-form-item>
            <el-form-item label="申请理由" class="ticket-form-full"><el-input v-model="form.formData.reason" type="textarea" :rows="2" /></el-form-item>
          </template>
          <template v-else-if="form.category === 'config_change'">
            <el-form-item label="配置对象"><el-input v-model="form.formData.target" /></el-form-item>
            <el-form-item label="变更窗口"><el-input v-model="form.formData.window" /></el-form-item>
            <el-form-item label="变更内容" class="ticket-form-full"><el-input v-model="form.formData.changeContent" type="textarea" :rows="2" /></el-form-item>
            <el-form-item label="回退方案" class="ticket-form-full"><el-input v-model="form.formData.rollbackPlan" type="textarea" :rows="2" /></el-form-item>
          </template>
          <template v-else-if="form.category === 'resource_request'">
            <el-form-item label="资源类型"><el-input v-model="form.formData.resourceType" placeholder="主机、数据库、存储等" /></el-form-item>
            <el-form-item label="使用周期"><el-input v-model="form.formData.duration" /></el-form-item>
            <el-form-item label="规格数量" class="ticket-form-full"><el-input v-model="form.formData.specification" placeholder="规格、数量、区域等" /></el-form-item>
            <el-form-item label="使用说明" class="ticket-form-full"><el-input v-model="form.formData.purpose" type="textarea" :rows="2" /></el-form-item>
          </template>
          <el-form-item label="详细说明" class="ticket-form-full" prop="description"><el-input v-model="form.description" type="textarea" :rows="3" maxlength="4000" show-word-limit /></el-form-item>
        </div>
      </el-form>
      <template #footer><el-button @click="createDialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="submit">提交工单</el-button></template>
    </el-dialog>

    <el-dialog v-model="assignDialog" class="ticket-dialog ticket-dialog--compact" modal-class="ticket-dialog-overlay" width="480px">
      <template #header>
        <div class="ticket-dialog-title">
          <span class="ticket-dialog-title__icon is-cyan"><el-icon><UserFilled /></el-icon></span>
          <div><h2>分派处理人</h2><p>指定负责人并补充分派说明</p></div>
        </div>
      </template>
      <el-form label-width="82px" class="ticket-form">
        <el-form-item label="处理人"><el-select v-model="assignForm.handlerId" class="ticket-full-width" filterable><el-option v-for="user in users" :key="user.id" :label="user.label" :value="user.id" /></el-select></el-form-item>
        <el-form-item label="分派说明"><el-input v-model="assignForm.comment" type="textarea" :rows="3" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="assignDialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="assign">确认分派</el-button></template>
    </el-dialog>

    <el-drawer v-model="detailDrawer" title="日常工单详情" class="ticket-detail-drawer" size="640px">
      <div v-loading="detailLoading" class="ticket-detail">
        <template v-if="detail.ticket">
          <section class="ticket-detail-section">
            <h3>{{ detail.ticket.ticketNo }} / {{ detail.ticket.title }}</h3>
            <div class="ticket-detail-grid"><div v-for="item in detailFields" :key="item.label" class="ticket-detail-row"><label>{{ item.label }}</label><span>{{ item.value }}</span></div></div>
          </section>
          <section v-if="formDataRows.length" class="ticket-detail-section">
            <h3>申请内容</h3>
            <div class="ticket-detail-grid"><div v-for="item in formDataRows" :key="item.label" class="ticket-detail-row"><label>{{ item.label }}</label><span>{{ item.value }}</span></div></div>
          </section>
          <section v-if="detail.workflow" class="ticket-detail-section">
            <h3>审批进度</h3>
            <el-timeline><el-timeline-item v-for="task in detail.workflow.tasks || []" :key="task.id" :timestamp="formatTime(task.actedAt || task.createdAt)" :type="statusType(task.status)"><strong>{{ task.nodeOrder }}. {{ task.nodeName }}</strong><div class="ticket-cell-sub">{{ task.assigneeName }} / {{ taskStatus(task.status) }}</div><p v-if="task.comment">{{ task.comment }}</p></el-timeline-item></el-timeline>
          </section>
          <section class="ticket-detail-section">
            <h3>处理记录</h3>
            <el-empty v-if="!detail.actions?.length" description="暂无处理记录" :image-size="64" />
            <el-timeline v-else><el-timeline-item v-for="action in detail.actions" :key="action.id" :timestamp="formatTime(action.createdAt)" :type="statusType(action.toStatus)"><strong>{{ action.operatorName }} · {{ actionLabel(action.action) }}</strong><p v-if="action.comment">{{ action.comment }}</p></el-timeline-item></el-timeline>
          </section>
        </template>
      </div>
    </el-drawer>
  </section>
</template>

<script setup>
/* global defineExpose */
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CircleCheck, Clock, Loading, Plus, Refresh, RefreshRight, Search, Tickets, UserFilled } from '@element-plus/icons-vue'
import systemApi from '@/api/system'
import ticketApi from '@/api/ticket'
import storage from '@/utils/storage'
import { dailyCategoryLabels, dailyStatusLabels, ensure, environmentLabels, formatTime, listResult, priorityLabels, priorityTypes, statusType, userLabel } from './ticketCommon'

const loading = ref(false), saving = ref(false), detailLoading = ref(false), rows = ref([]), total = ref(0)
const route = useRoute()
const createDialog = ref(false), assignDialog = ref(false), detailDrawer = ref(false), formRef = ref()
const workflows = ref([]), users = ref([]), detail = reactive({ ticket: null, actions: [], workflow: null })
const query = reactive({ page: 1, pageSize: 10, keyword: '', category: '', status: '', scope: '' })
const emptyForm = () => ({ title: '', category: 'ops_issue', environment: 'test', priority: 'medium', impactScope: '', description: '', workflowId: '', expectedAt: '', formData: {} })
const form = reactive(emptyForm()), assignForm = reactive({ ticketId: 0, handlerId: '', comment: '' })
const rules = { title: [{ required: true, message: '请输入工单标题' }], category: [{ required: true, message: '请选择事务类型' }], environment: [{ required: true, message: '请选择运行环境' }], priority: [{ required: true, message: '请选择优先级' }], workflowId: [{ required: true, message: '请选择审批流程' }], impactScope: [{ required: true, message: '请输入问题描述' }], description: [{ required: true, message: '请输入详细说明' }] }
const currentUserId = Number(storage.getItem('sysAdmin')?.id || 0)
const applicableWorkflows = computed(() => workflows.value.filter(item => (item.businessTypes || []).includes('daily') && (!(item.environments || []).length || item.environments.includes(form.environment))))
const workflowConflict = item => {
  if (!currentUserId || Number(item.preventSelfApproval) !== 1) return ''
  const node = (item.nodes || []).find(node => node.approverType === 'user' && (node.approverIds || []).length > 0 && node.approverIds.every(id => Number(id) === currentUserId))
  return node ? `节点“${node.name}”只有当前申请人审批，且流程禁止自审` : ''
}
const workflowOptionLabel = item => workflowConflict(item) ? `${item.name}（禁止自审冲突）` : item.name
const summary = computed(() => [
  { label: '当前工单', value: total.value, icon: Tickets, tone: 'primary' },
  { label: '等待审批', value: rows.value.filter(i => i.status === 'pending_approval').length, icon: Clock, tone: 'warning' },
  { label: '处理中', value: rows.value.filter(i => ['assigned', 'processing'].includes(i.status)).length, icon: Loading, tone: 'info' },
  { label: '待确认', value: rows.value.filter(i => i.status === 'resolved').length, icon: CircleCheck, tone: 'success' }
])
const detailFields = computed(() => { const row = detail.ticket || {}; return [{ label: '事务类型', value: dailyCategoryLabels[row.category] || row.category }, { label: '状态', value: dailyStatusLabels[row.status] || row.status }, { label: '运行环境', value: environmentLabels[row.environment] || row.environment }, { label: '优先级', value: priorityLabels[row.priority] || row.priority }, { label: '申请人', value: row.applicantName }, { label: '处理人', value: row.handlerName || '待分派' }, { label: row.category === 'ops_issue' ? '问题描述' : '影响范围', value: row.impactScope || '-' }, { label: '期望完成', value: formatTime(row.expectedAt) }, { label: '详细说明', value: row.description || '-' }, { label: '提交时间', value: formatTime(row.createdAt) }] })
const formFieldLabels = { symptom: '故障现象', reproducible: '复现情况', contact: '联系方式', system: '目标系统', duration: '有效期限', permissionScope: '权限范围', reason: '申请理由', target: '配置对象', window: '变更窗口', changeContent: '变更内容', rollbackPlan: '回退方案', resourceType: '资源类型', specification: '规格数量', purpose: '使用说明' }
const formDataRows = computed(() => Object.entries(detail.ticket?.formData || {}).filter(([, value]) => value !== '').map(([key, value]) => ({ label: formFieldLabels[key] || key, value })))
const taskStatus = value => ({ pending: '待审批', approved: '已通过', rejected: '已驳回', canceled: '已取消' }[value] || value)
const actionLabel = value => ({ create: '创建工单', assigned: '完成分派', processing: '开始处理', resolved: '提交解决', confirmed: '确认结果', closed: '关闭工单', canceled: '撤销工单' }[value] || value)
const canAssign = row => ['submitted', 'approved', 'assigned'].includes(row.status)
const canCancel = row => ['submitted', 'pending_approval', 'approved', 'assigned'].includes(row.status)
const canMore = row => row.status === 'confirmed' || canCancel(row)

const load = async () => { loading.value = true; try { const result = listResult(await ticketApi.dailyTickets(query)); rows.value = result.list; total.value = result.total } catch (error) { ElMessage.error(error.message || '加载日常工单失败') } finally { loading.value = false } }
const loadOptions = async () => { try { const [flowRes, userRes] = await Promise.all([ticketApi.workflows({ page: 1, pageSize: 100, enabled: 1 }), systemApi.queryAdminList({ PageSize: 100, PageNum: 1, Status: 1 })]); workflows.value = listResult(flowRes).list; users.value = listResult(userRes).list.map(item => ({ id: Number(item.id), label: userLabel(item) })) } catch (error) { ElMessage.error('加载审批流或用户失败') } }
const search = () => { query.page = 1; load() }
const resetQuery = () => { Object.assign(query, { page: 1, keyword: '', category: '', status: '', scope: '' }); load() }
const sizeChange = () => { query.page = 1; load() }
const categoryChange = () => { form.formData = {}; if (form.category === 'ops_issue') form.workflowId = '' }
const resetForm = () => { Object.assign(form, emptyForm()); formRef.value?.clearValidate() }
const openCreate = () => { resetForm(); createDialog.value = true }
const submit = async () => { if (!await formRef.value?.validate().catch(() => false)) return; saving.value = true; try { ensure(await ticketApi.createDaily({ ...form, workflowId: Number(form.workflowId || 0) }), '提交工单失败'); ElMessage.success('日常工单已提交'); createDialog.value = false; load() } catch (error) { ElMessage.error(error.message) } finally { saving.value = false } }
const openAssign = row => { Object.assign(assignForm, { ticketId: row.id, handlerId: row.handlerId || '', comment: '' }); assignDialog.value = true }
const assign = async () => { if (!assignForm.handlerId) { ElMessage.warning('请选择处理人'); return } saving.value = true; try { ensure(await ticketApi.assignDaily(assignForm.ticketId, { handlerId: Number(assignForm.handlerId), comment: assignForm.comment }), '分派失败'); ElMessage.success('工单已分派'); assignDialog.value = false; load() } catch (error) { ElMessage.error(error.message) } finally { saving.value = false } }
const perform = async (row, action) => { const labels = { start: '开始处理', resolve: '提交解决结果', confirm: '确认处理结果', close: '关闭工单', cancel: '撤销工单' }; try { const { value = '' } = await ElMessageBox.prompt('请填写本次操作说明', labels[action], { inputType: 'textarea', inputValidator: value => ['start', 'confirm', 'close'].includes(action) || String(value || '').trim() ? true : '操作说明不能为空' }); ensure(await ticketApi[`${action}Daily`](row.id, { comment: value }), `${labels[action]}失败`); ElMessage.success(`${labels[action]}成功`); load(); if (detailDrawer.value) openDetail(row.id) } catch (error) { if (error !== 'cancel' && error !== 'close') ElMessage.error(error.message) } }
const openDetail = async id => { detailDrawer.value = true; detailLoading.value = true; try { const data = ensure(await ticketApi.dailyTicket(id), '加载工单详情失败'); Object.assign(detail, { ticket: data.ticket, actions: data.actions || [], workflow: data.workflow || null }) } catch (error) { ElMessage.error(error.message) } finally { detailLoading.value = false } }
onMounted(() => { load(); loadOptions(); if (route.query.id) openDetail(route.query.id) })
defineExpose({ load, openDetail })
</script>
