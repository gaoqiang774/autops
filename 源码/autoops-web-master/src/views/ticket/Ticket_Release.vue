<template>
  <section class="ticket-view ticket-view--release">
    <div class="ticket-summary" aria-label="应用发布概览">
      <div v-for="item in summary" :key="item.label" class="ticket-summary__item" :class="`is-${item.tone}`">
        <span class="ticket-summary__icon" aria-hidden="true"><el-icon><component :is="item.icon" /></el-icon></span>
        <span class="ticket-summary__content"><span>{{ item.label }}</span><strong>{{ item.value }}</strong></span>
      </div>
    </div>

    <div class="ticket-toolbar ticket-toolbar--main">
      <div class="ticket-toolbar__filters">
        <el-input v-model="query.keyword" class="ticket-filter-input" clearable placeholder="发布单号 / 标题" @keyup.enter="search"><template #prefix><el-icon><Search /></el-icon></template></el-input>
        <el-select v-model="query.environment" class="ticket-filter-select" clearable placeholder="发布环境" @change="search"><el-option v-for="(label, value) in environmentLabels" :key="value" :label="label" :value="value" /></el-select>
        <el-select v-model="query.status" class="ticket-filter-select" clearable placeholder="发布状态" @change="search"><el-option v-for="(label, value) in releaseStatusLabels" :key="value" :label="label" :value="value" /></el-select>
        <el-select v-model="query.scope" class="ticket-filter-select" clearable placeholder="工单范围" @change="search"><el-option label="我发起的" value="created" /><el-option label="我执行的" value="executed" /></el-select>
      </div>
      <div class="ticket-toolbar__actions">
        <el-button size="small" type="primary" :icon="Search" @click="search">查询</el-button>
        <el-button size="small" type="warning" :icon="RefreshRight" @click="resetQuery">重置</el-button>
        <el-button size="small" type="primary" :icon="Setting" @click="openApplications">应用配置</el-button>
        <el-button size="small" type="warning" class="ops-refresh-button" :icon="Refresh" :loading="loading" @click="load">刷新</el-button>
        <el-button size="small" type="success" :icon="Plus" @click="openCreate">新建发布</el-button>
      </div>
    </div>

    <div class="ticket-table-wrap">
      <el-table v-loading="loading" :data="rows" height="100%" size="small" row-key="id" empty-text="暂无发布工单">
        <el-table-column label="发布工单" min-width="245">
          <template #default="{ row }">
            <button type="button" class="ticket-link" @click="openDetail(row.id)">{{ row.ticketNo }}</button>
            <el-tooltip :content="row.title" placement="top-start" :show-after="400"><div class="ticket-cell-sub ticket-cell-sub--ellipsis">{{ row.title }}</div></el-tooltip>
          </template>
        </el-table-column>
        <el-table-column label="环境" width="90"><template #default="{ row }">{{ environmentLabels[row.environment] || row.environment }}</template></el-table-column>
        <el-table-column label="策略" width="80"><template #default="{ row }">{{ row.strategy === 'parallel' ? '并行' : '串行' }}</template></el-table-column>
        <el-table-column label="应用" min-width="180" show-overflow-tooltip><template #default="{ row }"><span>{{ (row.items || []).map(item => item.applicationName).join('、') || '-' }}</span></template></el-table-column>
        <el-table-column prop="executorName" label="执行人" width="110" show-overflow-tooltip />
        <el-table-column label="状态" width="105"><template #default="{ row }"><el-tag size="small" :type="statusType(row.status)">{{ releaseStatusLabels[row.status] || row.status }}</el-tag></template></el-table-column>
        <el-table-column label="创建时间" min-width="155"><template #default="{ row }">{{ formatTime(row.createdAt) }}</template></el-table-column>
        <el-table-column label="操作" width="120" fixed="right" align="right"><template #default="{ row }"><div class="ticket-actions"><button type="button" class="ticket-action" @click="openDetail(row.id)">详情</button><button v-if="['approved', 'failed'].includes(row.status)" type="button" class="ticket-action success" @click="execute(row)">执行</button></div></template></el-table-column>
      </el-table>
    </div>

    <div class="ticket-pagination"><el-pagination v-model:current-page="query.page" v-model:page-size="query.pageSize" :total="total" :page-sizes="[10, 20, 50]" layout="total, sizes, prev, pager, next" @current-change="load" @size-change="sizeChange" /></div>

    <el-dialog v-model="createDialog" class="ticket-dialog ticket-dialog--form" modal-class="ticket-dialog-overlay" width="920px" destroy-on-close @closed="resetReleaseForm">
      <template #header>
        <div class="ticket-dialog-title"><span class="ticket-dialog-title__icon is-green"><el-icon><Promotion /></el-icon></span><div><h2>新建应用发布工单</h2><p>配置发布范围、执行策略与审批流程</p></div></div>
      </template>
      <el-form ref="releaseFormRef" :model="releaseForm" :rules="releaseRules" label-width="92px" class="ticket-form">
        <div class="ticket-form-grid">
          <el-form-item label="发布标题" prop="title"><el-input v-model="releaseForm.title" /></el-form-item>
          <el-form-item label="发布环境" prop="environment"><el-select v-model="releaseForm.environment" @change="environmentChange"><el-option v-for="(label, value) in environmentLabels" :key="value" :label="label" :value="value" /></el-select></el-form-item>
          <el-form-item label="发布策略" prop="strategy"><el-radio-group v-model="releaseForm.strategy"><el-radio-button value="serial">串行</el-radio-button><el-radio-button value="parallel">并行</el-radio-button></el-radio-group></el-form-item>
          <el-form-item label="执行人" prop="executorId"><el-select v-model="releaseForm.executorId" filterable><el-option v-for="user in users" :key="user.id" :label="user.label" :value="user.id" /></el-select></el-form-item>
          <el-form-item label="审批流程" prop="workflowId"><el-select v-model="releaseForm.workflowId"><el-option v-for="flow in applicableWorkflows" :key="flow.id" :label="flow.name" :value="flow.id" /></el-select></el-form-item>
          <el-form-item label="发布说明" class="ticket-form-full"><el-input v-model="releaseForm.description" type="textarea" :rows="2" /></el-form-item>
          <el-form-item label="回滚方案" class="ticket-form-full" prop="rollbackPlan"><el-input v-model="releaseForm.rollbackPlan" type="textarea" :rows="2" /></el-form-item>
          <div class="ticket-items-editor">
            <div class="ticket-item-editor__header"><div><strong>发布应用</strong><span>按顺序配置应用、版本与目标</span></div><el-button size="small" type="success" :icon="Plus" @click="addReleaseItem">添加应用</el-button></div>
            <div v-for="(item, index) in releaseForm.items" :key="item.key" class="ticket-item-row">
              <el-select v-model="item.applicationId" filterable placeholder="选择应用" @change="value => selectApplication(item, value)"><el-option v-for="app in availableApplications" :key="app.id" :label="`${app.name} (${app.jenkinsJobName})`" :value="app.id" /></el-select>
              <el-input v-model="item.version" placeholder="版本 / Tag" />
              <el-input v-model="item.target" placeholder="目标集群 / 节点" />
              <el-input v-model="item.parametersText" class="ticket-code-input" placeholder="参数 JSON" />
              <el-tooltip content="删除应用" placement="top"><el-button class="ticket-icon-button" text type="danger" :icon="Delete" aria-label="删除应用" @click="releaseForm.items.splice(index, 1)" /></el-tooltip>
            </div>
          </div>
        </div>
      </el-form>
      <template #footer><el-button @click="createDialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="submitRelease">提交发布工单</el-button></template>
    </el-dialog>

    <el-dialog v-model="applicationsDialog" class="ticket-dialog ticket-dialog--wide" modal-class="ticket-dialog-overlay" width="980px">
      <template #header>
        <div class="ticket-dialog-title"><span class="ticket-dialog-title__icon is-cyan"><el-icon><Setting /></el-icon></span><div><h2>发布应用配置</h2><p>维护应用与 Jenkins Job 的执行映射</p></div></div>
      </template>
      <div class="ticket-toolbar ticket-dialog-toolbar">
        <div class="ticket-toolbar__filters">
          <el-input v-model="appQuery.keyword" class="ticket-filter-input" clearable placeholder="应用名称 / 编码" @keyup.enter="loadApplications"><template #prefix><el-icon><Search /></el-icon></template></el-input>
          <el-select v-model="appQuery.environment" class="ticket-filter-select" clearable placeholder="环境" @change="loadApplications"><el-option v-for="(label, value) in environmentLabels" :key="value" :label="label" :value="value" /></el-select>
        </div>
        <div class="ticket-toolbar__actions"><el-button size="small" type="primary" :icon="Search" @click="loadApplications">查询</el-button><el-button size="small" type="success" :icon="Plus" @click="openApplicationForm()">新增应用</el-button></div>
      </div>
      <div class="ticket-dialog-table"><el-table v-loading="appLoading" :data="applications" height="420" size="small" row-key="id" empty-text="暂无应用配置"><el-table-column label="应用" min-width="170"><template #default="{ row }"><strong>{{ row.name }}</strong><div class="ticket-cell-sub">{{ row.code }} / {{ row.systemName || '未归属系统' }}</div></template></el-table-column><el-table-column label="环境" width="90"><template #default="{ row }">{{ environmentLabels[row.environment] || row.environment }}</template></el-table-column><el-table-column prop="jenkinsJobName" label="Jenkins Job" min-width="220" show-overflow-tooltip /><el-table-column label="状态" width="80"><template #default="{ row }"><el-tag size="small" :type="Number(row.enabled) === 1 ? 'success' : 'info'">{{ Number(row.enabled) === 1 ? '启用' : '禁用' }}</el-tag></template></el-table-column><el-table-column label="操作" width="120" align="right"><template #default="{ row }"><div class="ticket-actions"><button type="button" class="ticket-action" @click="openApplicationForm(row)">编辑</button><button type="button" class="ticket-action danger" @click="removeApplication(row)">删除</button></div></template></el-table-column></el-table></div>
    </el-dialog>

    <el-dialog v-model="applicationFormDialog" class="ticket-dialog ticket-dialog--form" modal-class="ticket-dialog-overlay" width="680px">
      <template #header>
        <div class="ticket-dialog-title"><span class="ticket-dialog-title__icon"><el-icon><Setting /></el-icon></span><div><h2>{{ applicationForm.id ? '编辑发布应用' : '新增发布应用' }}</h2><p>配置应用标识、环境与 Jenkins 执行入口</p></div></div>
      </template>
      <el-form ref="applicationFormRef" :model="applicationForm" :rules="applicationRules" label-width="110px" class="ticket-form">
        <div class="ticket-form-grid"><el-form-item label="应用名称" prop="name"><el-input v-model="applicationForm.name" /></el-form-item><el-form-item label="应用编码" prop="code"><el-input v-model="applicationForm.code" /></el-form-item><el-form-item label="所属系统"><el-input v-model="applicationForm.systemName" /></el-form-item><el-form-item label="运行环境" prop="environment"><el-select v-model="applicationForm.environment"><el-option v-for="(label, value) in environmentLabels" :key="value" :label="label" :value="value" /></el-select></el-form-item><el-form-item label="Jenkins 账号" prop="jenkinsAccountId"><el-select v-model="applicationForm.jenkinsAccountId" filterable><el-option v-for="account in accounts" :key="account.id" :label="account.label" :value="account.id" /></el-select></el-form-item><el-form-item label="Jenkins Job" prop="jenkinsJobName"><el-input v-model="applicationForm.jenkinsJobName" placeholder="folder/service-build" /></el-form-item><el-form-item label="默认参数" class="ticket-form-full"><el-input v-model="applicationForm.defaultParametersText" type="textarea" :rows="3" placeholder='例如 {"BRANCH":"main"}' /></el-form-item><el-form-item label="应用说明" class="ticket-form-full"><el-input v-model="applicationForm.description" type="textarea" :rows="2" /></el-form-item><el-form-item label="启用"><el-switch v-model="applicationForm.enabled" :active-value="1" :inactive-value="0" /></el-form-item></div>
      </el-form>
      <template #footer><el-button @click="applicationFormDialog = false">取消</el-button><el-button type="primary" :loading="saving" @click="saveApplication">保存应用</el-button></template>
    </el-dialog>

    <el-drawer v-model="detailDrawer" title="发布工单详情" class="ticket-detail-drawer" size="720px">
      <div v-loading="detailLoading" class="ticket-detail">
        <template v-if="detail.ticket">
          <section class="ticket-detail-section">
            <div class="ticket-detail-heading"><h3>{{ detail.ticket.ticketNo }} / {{ detail.ticket.title }}</h3><el-button v-if="['approved', 'failed'].includes(detail.ticket.status)" size="small" type="primary" :icon="VideoPlay" @click="execute(detail.ticket)">执行发布</el-button></div>
            <div class="ticket-detail-grid"><div v-for="item in detailFields" :key="item.label" class="ticket-detail-row"><label>{{ item.label }}</label><span>{{ item.value }}</span></div></div>
          </section>
          <section class="ticket-detail-section">
            <h3>发布项</h3>
            <el-table :data="detail.ticket.items || []" size="small"><el-table-column prop="applicationName" label="应用" min-width="140" show-overflow-tooltip /><el-table-column prop="version" label="版本" min-width="105" show-overflow-tooltip /><el-table-column label="状态" width="90"><template #default="{ row }"><el-tag size="small" :type="statusType(row.status)">{{ itemStatusLabels[row.status] || row.status }}</el-tag></template></el-table-column><el-table-column label="构建" min-width="110"><template #default="{ row }">{{ row.buildNumber ? `#${row.buildNumber}` : row.queueId ? `队列 #${row.queueId}` : '-' }}</template></el-table-column><el-table-column label="操作" width="150" align="right"><template #default="{ row }"><div class="ticket-actions"><button v-if="row.buildNumber" type="button" class="ticket-action" @click="showLog(row)">日志</button><button v-if="['failed', 'canceled'].includes(row.status)" type="button" class="ticket-action warning" @click="retryItem(row)">重试</button><button v-if="['queued', 'running'].includes(row.status)" type="button" class="ticket-action danger" @click="cancelItem(row)">中止</button></div></template></el-table-column></el-table>
          </section>
          <section v-if="detail.workflow" class="ticket-detail-section"><h3>审批进度</h3><el-timeline><el-timeline-item v-for="task in detail.workflow.tasks || []" :key="task.id" :timestamp="formatTime(task.actedAt || task.createdAt)" :type="statusType(task.status)"><strong>{{ task.nodeOrder }}. {{ task.nodeName }}</strong><div class="ticket-cell-sub">{{ task.assigneeName }} / {{ taskStatus(task.status) }}</div><p v-if="task.comment">{{ task.comment }}</p></el-timeline-item></el-timeline></section>
        </template>
      </div>
    </el-drawer>

    <el-dialog v-model="logDialog" :title="logTitle" class="ticket-dialog ticket-dialog--log" modal-class="ticket-dialog-overlay" width="900px"><pre v-loading="logLoading" class="ticket-log">{{ logContent || '暂无构建日志' }}</pre></el-dialog>
  </section>
</template>

<script setup>
/* global defineExpose */
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CircleCheck, Clock, Delete, Loading, Plus, Promotion, Refresh, RefreshRight, Search, Setting, VideoPlay } from '@element-plus/icons-vue'
import configApi from '@/api/config'
import systemApi from '@/api/system'
import ticketApi from '@/api/ticket'
import { ensure, environmentLabels, formatTime, itemStatusLabels, listResult, releaseStatusLabels, statusType, userLabel } from './ticketCommon'

const loading = ref(false), saving = ref(false), detailLoading = ref(false), appLoading = ref(false), logLoading = ref(false)
const route = useRoute()
const rows = ref([]), total = ref(0), applications = ref([]), workflows = ref([]), users = ref([]), accounts = ref([])
const createDialog = ref(false), applicationsDialog = ref(false), applicationFormDialog = ref(false), detailDrawer = ref(false), logDialog = ref(false)
const releaseFormRef = ref(), applicationFormRef = ref(), logContent = ref(''), logTitle = ref('Jenkins 构建日志')
const query = reactive({ page: 1, pageSize: 10, keyword: '', environment: '', status: '', scope: '' }), appQuery = reactive({ page: 1, pageSize: 100, keyword: '', environment: '' })
const emptyRelease = () => ({ title: '', environment: 'test', strategy: 'serial', description: '', rollbackPlan: '', workflowId: '', executorId: '', items: [] })
const emptyApplication = () => ({ id: 0, name: '', code: '', systemName: '', environment: 'test', jenkinsAccountId: '', jenkinsJobName: '', defaultParametersText: '{}', description: '', enabled: 1 })
const releaseForm = reactive(emptyRelease()), applicationForm = reactive(emptyApplication()), detail = reactive({ ticket: null, workflow: null })
const releaseRules = { title: [{ required: true, message: '请输入发布标题' }], environment: [{ required: true, message: '请选择发布环境' }], strategy: [{ required: true }], executorId: [{ required: true, message: '请选择执行人' }], workflowId: [{ required: true, message: '请选择审批流程' }], rollbackPlan: [{ required: true, message: '请输入回滚方案' }] }
const applicationRules = { name: [{ required: true, message: '请输入应用名称' }], code: [{ required: true, message: '请输入应用编码' }], environment: [{ required: true, message: '请选择运行环境' }], jenkinsAccountId: [{ required: true, message: '请选择 Jenkins 账号' }], jenkinsJobName: [{ required: true, message: '请输入 Jenkins Job' }] }
const applicableWorkflows = computed(() => workflows.value.filter(item => (item.businessTypes || []).includes('release') && (!(item.environments || []).length || item.environments.includes(releaseForm.environment))))
const availableApplications = computed(() => applications.value.filter(item => item.environment === releaseForm.environment && Number(item.enabled) === 1))
const summary = computed(() => [
  { label: '当前发布单', value: total.value, icon: Promotion, tone: 'primary' },
  { label: '等待审批', value: rows.value.filter(i => i.status === 'pending_approval').length, icon: Clock, tone: 'warning' },
  { label: '执行中', value: rows.value.filter(i => i.status === 'running').length, icon: Loading, tone: 'info' },
  { label: '本页成功', value: rows.value.filter(i => i.status === 'succeeded').length, icon: CircleCheck, tone: 'success' }
])
const detailFields = computed(() => { const row = detail.ticket || {}; return [{ label: '状态', value: releaseStatusLabels[row.status] || row.status }, { label: '环境', value: environmentLabels[row.environment] || row.environment }, { label: '发布策略', value: row.strategy === 'parallel' ? '并行' : '串行' }, { label: '申请人', value: row.applicantName }, { label: '执行人', value: row.executorName }, { label: '发布说明', value: row.description || '-' }, { label: '回滚方案', value: row.rollbackPlan || '-' }, { label: '开始时间', value: formatTime(row.startedAt) }] })
const taskStatus = value => ({ pending: '待审批', approved: '已通过', rejected: '已驳回', canceled: '已取消' }[value] || value)
const parseObject = (text, label) => { try { const value = JSON.parse(String(text || '{}')); if (!value || Array.isArray(value) || typeof value !== 'object') throw new Error(); return value } catch { throw new Error(`${label}必须是有效 JSON 对象`) } }

const load = async () => { loading.value = true; try { const result = listResult(await ticketApi.releases(query)); rows.value = result.list; total.value = result.total } catch (error) { ElMessage.error(error.message || '加载发布工单失败') } finally { loading.value = false } }
const loadApplications = async () => { appLoading.value = true; try { const result = listResult(await ticketApi.applications(appQuery)); applications.value = result.list } catch (error) { ElMessage.error(error.message || '加载应用配置失败') } finally { appLoading.value = false } }
const loadOptions = async () => { try { const [flowRes, userRes, accountRes] = await Promise.all([ticketApi.workflows({ page: 1, pageSize: 100, enabled: 1 }), systemApi.queryAdminList({ PageSize: 100, PageNum: 1, Status: 1 }), configApi.listAccountAuth({ type: 4, page: 1, pageSize: 100 })]); workflows.value = listResult(flowRes).list; users.value = listResult(userRes).list.map(item => ({ id: Number(item.id), label: userLabel(item) })); accounts.value = listResult(accountRes).list.filter(item => Number(item.type ?? item.accountType) === 4).map(item => ({ id: Number(item.id), label: item.alias || item.name || item.username || `Jenkins #${item.id}` })) } catch (error) { ElMessage.error('加载审批流、用户或 Jenkins 账号失败') } }
const search = () => { query.page = 1; load() }, sizeChange = () => { query.page = 1; load() }
const resetQuery = () => { Object.assign(query, { page: 1, keyword: '', environment: '', status: '', scope: '' }); load() }
const resetReleaseForm = () => { Object.assign(releaseForm, emptyRelease()); addReleaseItem(); releaseFormRef.value?.clearValidate() }
const addReleaseItem = () => releaseForm.items.push({ key: `${Date.now()}-${releaseForm.items.length}`, applicationId: '', version: '', target: '', parametersText: '{}' })
const environmentChange = () => { releaseForm.workflowId = ''; releaseForm.items = []; addReleaseItem() }
const selectApplication = (item, id) => { const app = applications.value.find(value => Number(value.id) === Number(id)); item.parametersText = JSON.stringify(app?.defaultParameters || {}, null, 0) }
const openCreate = async () => { if (!applications.value.length) await loadApplications(); resetReleaseForm(); createDialog.value = true }
const submitRelease = async () => { if (!await releaseFormRef.value?.validate().catch(() => false)) return; if (!releaseForm.items.length || releaseForm.items.some(item => !item.applicationId || !item.version.trim())) { ElMessage.warning('请完整填写发布应用和版本'); return } let items; try { items = releaseForm.items.map((item, index) => ({ applicationId: Number(item.applicationId), version: item.version, target: item.target, parameters: parseObject(item.parametersText, `第 ${index + 1} 个应用参数`), executeOrder: index + 1 })) } catch (error) { ElMessage.error(error.message); return } saving.value = true; try { ensure(await ticketApi.createRelease({ ...releaseForm, workflowId: Number(releaseForm.workflowId), executorId: Number(releaseForm.executorId), items }), '提交发布工单失败'); ElMessage.success('发布工单已提交审批'); createDialog.value = false; load() } catch (error) { ElMessage.error(error.message) } finally { saving.value = false } }
const openApplications = async () => { applicationsDialog.value = true; await loadApplications() }
const openApplicationForm = row => { Object.assign(applicationForm, row ? { ...row, id: row.id, defaultParametersText: JSON.stringify(row.defaultParameters || {}, null, 2) } : emptyApplication()); applicationFormDialog.value = true }
const saveApplication = async () => { if (!await applicationFormRef.value?.validate().catch(() => false)) return; let defaultParameters; try { defaultParameters = parseObject(applicationForm.defaultParametersText, '默认参数') } catch (error) { ElMessage.error(error.message); return } const payload = { ...applicationForm, jenkinsAccountId: Number(applicationForm.jenkinsAccountId), defaultParameters }; delete payload.id; delete payload.defaultParametersText; saving.value = true; try { ensure(applicationForm.id ? await ticketApi.updateApplication(applicationForm.id, payload) : await ticketApi.createApplication(payload), '保存应用失败'); ElMessage.success('应用配置已保存'); applicationFormDialog.value = false; loadApplications() } catch (error) { ElMessage.error(error.message) } finally { saving.value = false } }
const removeApplication = async row => { try { await ElMessageBox.confirm(`确认删除应用「${row.name}」？`, '删除应用', { type: 'warning' }); ensure(await ticketApi.deleteApplication(row.id), '删除应用失败'); ElMessage.success('应用已删除'); loadApplications() } catch (error) { if (error !== 'cancel' && error !== 'close') ElMessage.error(error.message) } }
const openDetail = async id => { detailDrawer.value = true; detailLoading.value = true; try { const data = ensure(await ticketApi.release(id), '加载发布详情失败'); Object.assign(detail, { ticket: data.ticket, workflow: data.workflow || null }) } catch (error) { ElMessage.error(error.message) } finally { detailLoading.value = false } }
const execute = async row => { try { await ElMessageBox.confirm('确认执行该发布工单？', '执行发布', { type: 'warning' }); ensure(await ticketApi.executeRelease(row.id), '启动发布失败'); ElMessage.success('发布任务已启动'); load(); openDetail(row.id) } catch (error) { if (error !== 'cancel' && error !== 'close') ElMessage.error(error.message) } }
const retryItem = async item => { try { await ElMessageBox.confirm(`确认重试 ${item.applicationName}？`, '重试发布项', { type: 'warning' }); ensure(await ticketApi.retryReleaseItem(detail.ticket.id, item.id), '重试失败'); ElMessage.success('发布项已重新触发'); openDetail(detail.ticket.id); load() } catch (error) { if (error !== 'cancel' && error !== 'close') ElMessage.error(error.message) } }
const cancelItem = async item => { try { await ElMessageBox.confirm(`确认中止 ${item.applicationName}？`, '中止发布项', { type: 'warning' }); ensure(await ticketApi.cancelReleaseItem(detail.ticket.id, item.id), '中止失败'); ElMessage.success('发布项已中止'); openDetail(detail.ticket.id); load() } catch (error) { if (error !== 'cancel' && error !== 'close') ElMessage.error(error.message) } }
const showLog = async item => { logDialog.value = true; logLoading.value = true; logTitle.value = `${item.applicationName} #${item.buildNumber} 构建日志`; logContent.value = ''; try { const data = ensure(await ticketApi.releaseItemLog(detail.ticket.id, item.id), '加载构建日志失败'); logContent.value = data.content || '' } catch (error) { ElMessage.error(error.message) } finally { logLoading.value = false } }
onMounted(() => { load(); loadApplications(); loadOptions(); if (route.query.id) openDetail(route.query.id) })
defineExpose({ load, openDetail })
</script>
