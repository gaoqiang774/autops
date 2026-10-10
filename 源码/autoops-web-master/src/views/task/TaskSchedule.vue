<template>
  <section class="task-view" aria-labelledby="schedule-title">
    <div class="task-panel">
      <header class="task-panel__header">
        <div class="task-panel__title">
          <h2 id="schedule-title">任务调度</h2>
          <p>创建、执行与控制普通任务和定时任务</p>
        </div>
        <div class="task-toolbar">
          <el-input v-model="filters.name" clearable placeholder="任务名称" aria-label="按任务名称筛选" @keyup.enter="search" @clear="search">
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-select v-model="filters.type" clearable placeholder="全部类型" aria-label="按任务类型筛选" @change="search">
            <el-option v-for="(item, value) in TASK_TYPES" :key="value" :label="item.label" :value="Number(value)" />
          </el-select>
          <el-select v-model="filters.status" clearable placeholder="全部状态" aria-label="按任务状态筛选" @change="search">
            <el-option v-for="(item, value) in TASK_STATUSES" :key="value" :label="item.label" :value="Number(value)" />
          </el-select>
          <el-button type="primary" :loading="loading" @click="search"><el-icon><Search /></el-icon>查询</el-button>
          <el-button class="ops-refresh-button" type="warning" :loading="loading" @click="refresh"><el-icon><Refresh /></el-icon>刷新</el-button>
          <el-button type="success" @click="openCreate"><el-icon><Plus /></el-icon>新建任务</el-button>
        </div>
      </header>

      <div class="task-panel__body">
        <div class="task-table-wrap" v-loading="loading">
          <el-table :data="tasks" height="calc(100vh - 300px)" min-height="330" table-layout="fixed">
            <el-table-column label="任务名称" min-width="190" fixed="left">
              <template #default="{ row }">
                <div class="task-name-cell">
                  <strong>{{ row.name || '--' }}</strong>
                  <span>{{ row.remark || `任务 ID: ${row.id}` }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="类型" width="105">
              <template #default="{ row }"><el-tag :type="taskType(row.type).type" effect="light">{{ taskType(row.type).label }}</el-tag></template>
            </el-table-column>
            <el-table-column label="模板" min-width="165">
              <template #default="{ row }">
                <div v-if="rowTemplates(row).length" class="task-chip-list">
                  <el-tooltip v-for="item in rowTemplates(row).slice(0, 2)" :key="item.id" :content="item.name" placement="top">
                    <el-tag size="small" effect="plain">{{ item.name }}</el-tag>
                  </el-tooltip>
                  <el-tag v-if="rowTemplates(row).length > 2" size="small" type="info">+{{ rowTemplates(row).length - 2 }}</el-tag>
                </div>
                <span v-else class="task-empty-inline">{{ row.shell || '--' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="执行主机" min-width="170">
              <template #default="{ row }">
                <div v-if="rowHosts(row).length" class="task-chip-list">
                  <el-tooltip v-for="host in rowHosts(row).slice(0, 2)" :key="host.id" :content="hostLabel(host)" placement="top">
                    <el-tag size="small" type="info" effect="plain">{{ host.hostName || host.name || host.id }}</el-tag>
                  </el-tooltip>
                  <el-tag v-if="rowHosts(row).length > 2" size="small" type="info">+{{ rowHosts(row).length - 2 }}</el-tag>
                </div>
                <span v-else class="task-empty-inline">{{ row.host_ids || '--' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="108">
              <template #default="{ row }">
                <span class="task-status">
                  <el-icon :class="{ 'is-loading': Number(row.status) === 2 && Number(row.type) === 1 }"><component :is="taskStatus(row.status).icon" /></el-icon>
                  {{ taskStatus(row.status).label }}
                </span>
              </template>
            </el-table-column>
            <el-table-column prop="execute_count" label="执行次数" width="92" align="right">
              <template #default="{ row }">{{ Number(row.execute_count || 0) }}</template>
            </el-table-column>
            <el-table-column label="下次执行" width="170">
              <template #default="{ row }">{{ formatDate(row.next_run_time) }}</template>
            </el-table-column>
            <el-table-column label="耗时" width="95" align="right">
              <template #default="{ row }">{{ formatDuration(row.duration) }}</template>
            </el-table-column>
            <el-table-column label="创建时间" width="170">
              <template #default="{ row }">{{ formatDate(row.created_at || row.createdAt) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="215" fixed="right" align="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="openDetail(row)">详情</el-button>
                <el-button
                  v-if="primaryAction(row)"
                  link
                  :type="primaryAction(row).type"
                  :loading="operatingId === row.id"
                  @click="runPrimaryAction(row)"
                >{{ primaryAction(row).label }}</el-button>
                <el-dropdown trigger="click" @command="command => handleMore(command, row)">
                  <el-button link type="primary">更多<el-icon><ArrowDown /></el-icon></el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item v-if="canViewLog(row)" command="log"><el-icon><Document /></el-icon>查看日志</el-dropdown-item>
                      <el-dropdown-item v-if="canStop(row)" command="stop"><el-icon><SwitchButton /></el-icon>停止任务</el-dropdown-item>
                      <el-dropdown-item v-if="canEdit(row)" command="edit"><el-icon><Edit /></el-icon>编辑</el-dropdown-item>
                      <el-dropdown-item v-if="Number(row.type) === 2" command="reset"><el-icon><RefreshRight /></el-icon>重置子任务</el-dropdown-item>
                      <el-dropdown-item v-if="canDelete(row)" command="delete" divided><el-icon><Delete /></el-icon>删除</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </template>
            </el-table-column>
            <template #empty>
              <el-empty description="暂无任务">
                <el-button type="success" @click="openCreate">新建任务</el-button>
              </el-empty>
            </template>
          </el-table>
        </div>
        <div class="task-pagination">
          <el-pagination
            v-model:current-page="pagination.page"
            v-model:page-size="pagination.pageSize"
            background
            layout="total, sizes, prev, pager, next"
            :page-sizes="[10, 20, 50, 100]"
            :total="pagination.total"
            @current-change="loadTasks"
            @size-change="handlePageSize"
          />
        </div>
      </div>
    </div>

    <el-dialog
      v-model="formVisible"
      class="task-dialog"
      modal-class="task-form-dialog-overlay"
      width="820px"
      destroy-on-close
      :close-on-click-modal="false"
    >
      <template #header>
        <div class="task-dialog-title">
          <span class="task-dialog-title__icon"><el-icon><Calendar /></el-icon></span>
          <div><h2>{{ editingId ? '编辑任务' : '新建任务' }}</h2><p>一个任务会生成“主机数量 × 模板数量”个执行单元</p></div>
        </div>
      </template>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" class="task-dialog" @submit.prevent>
        <div class="task-form-grid">
          <el-form-item label="任务名称" prop="name">
            <el-input v-model="form.name" maxlength="100" show-word-limit placeholder="请输入唯一任务名称" />
          </el-form-item>
          <el-form-item label="任务类型" prop="type">
            <el-radio-group v-model="form.type" :disabled="Boolean(editingId)">
              <el-radio-button :value="1">立即执行</el-radio-button>
              <el-radio-button :value="2">定时任务</el-radio-button>
            </el-radio-group>
          </el-form-item>
          <el-form-item class="is-full" label="任务模板" prop="templateIds">
            <el-select v-model="form.templateIds" multiple filterable collapse-tags collapse-tags-tooltip placeholder="请选择一个或多个模板" :loading="optionsLoading">
              <el-option v-for="item in templateOptions" :key="item.id" :label="`${item.name} · ${templateType(item.type).label}`" :value="Number(item.id)" />
            </el-select>
          </el-form-item>
          <el-form-item class="is-full" label="执行主机" prop="hostIds">
            <div style="width:100%">
              <el-button type="primary" plain @click="hostSelectorVisible = true"><el-icon><Plus /></el-icon>选择主机</el-button>
              <span class="task-empty-inline" style="margin-left:10px">已选择 {{ form.hostIds.length }} 台</span>
              <div v-if="form.hosts.length" class="task-chip-list" style="margin-top:8px">
                <el-tag v-for="host in form.hosts" :key="host.id" closable type="info" @close="removeSelectedHost(host.id)">{{ hostLabel(host) }}</el-tag>
              </div>
            </div>
          </el-form-item>
          <el-form-item v-if="form.type === 2" class="is-full" label="Cron 表达式" prop="cronExpr">
            <div style="width:100%">
              <el-input v-model="form.cronExpr" placeholder="分 时 日 月 周，例如：0 2 * * *" @blur="checkCron">
                <template #append><el-button :loading="cronChecking" @click="checkCron">校验</el-button></template>
              </el-input>
              <div v-if="nextExecution" class="task-inline-message" style="margin-top:8px">
                <el-icon><Clock /></el-icon>下次执行：{{ formatDate(nextExecution) }}
              </div>
            </div>
          </el-form-item>
          <el-form-item class="is-full" label="备注" prop="remark">
            <el-input v-model="form.remark" type="textarea" :rows="3" maxlength="500" show-word-limit placeholder="说明任务用途和影响范围" />
          </el-form-item>
        </div>
      </el-form>
      <template #footer>
        <div class="task-dialog-footer">
          <el-button @click="formVisible = false">取消</el-button>
          <el-button :loading="submitting" @click="submitTask(false)">{{ editingId ? '保存' : '仅保存' }}</el-button>
          <el-button v-if="!editingId" type="primary" :loading="submitting" @click="submitTask(true)">保存并执行</el-button>
        </div>
      </template>
    </el-dialog>

    <HostSelectorDialog v-model="hostSelectorVisible" title="选择任务执行主机" @hosts-selected="handleHostsSelected" />

    <el-dialog v-model="detailVisible" class="task-dialog" modal-class="task-drawer-dialog-overlay" width="760px" destroy-on-close>
      <template #header>
        <div class="task-dialog-title">
          <span class="task-dialog-title__icon"><el-icon><View /></el-icon></span>
          <div><h2>{{ detail.name || '任务详情' }}</h2><p>最近一次执行和调度信息</p></div>
        </div>
      </template>
      <dl class="task-detail-list" v-loading="detailLoading">
        <div class="task-detail-item"><dt>任务类型</dt><dd>{{ taskType(detail.type).label }}</dd></div>
        <div class="task-detail-item"><dt>当前状态</dt><dd>{{ taskStatus(detail.status).label }}</dd></div>
        <div class="task-detail-item"><dt>执行次数</dt><dd>{{ Number(detail.execute_count || 0) }}</dd></div>
        <div class="task-detail-item"><dt>执行单元</dt><dd>{{ Number(detail.task_count || 0) }}</dd></div>
        <div class="task-detail-item"><dt>开始时间</dt><dd>{{ formatDate(detail.start_time) }}</dd></div>
        <div class="task-detail-item"><dt>结束时间</dt><dd>{{ formatDate(detail.end_time) }}</dd></div>
        <div class="task-detail-item"><dt>下次执行</dt><dd>{{ formatDate(detail.next_run_time) }}</dd></div>
        <div class="task-detail-item"><dt>Cron</dt><dd>{{ detail.cron_expr || '--' }}</dd></div>
        <div class="task-detail-item"><dt>模板 ID</dt><dd>{{ detail.shell || '--' }}</dd></div>
        <div class="task-detail-item"><dt>主机 ID</dt><dd>{{ detail.host_ids || '--' }}</dd></div>
        <div class="task-detail-item"><dt>备注</dt><dd>{{ detail.remark || '--' }}</dd></div>
        <div class="task-detail-item"><dt>创建时间</dt><dd>{{ formatDate(detail.created_at) }}</dd></div>
      </dl>
    </el-dialog>

    <el-dialog v-model="workDialogVisible" class="task-dialog" modal-class="task-drawer-dialog-overlay" width="760px" destroy-on-close @closed="closeWorkDialog">
      <template #header>
        <div class="task-dialog-title">
          <span class="task-dialog-title__icon"><el-icon><Document /></el-icon></span>
          <div><h2>{{ workMode === 'log' ? '当前模板日志' : '停止任务' }}</h2><p>{{ currentTask.name || '' }}</p></div>
        </div>
      </template>
      <el-form label-position="top" class="task-dialog">
        <el-form-item label="任务模板">
          <el-select v-model="selectedTemplateId" placeholder="请选择模板" @change="workMode === 'log' && loadLog()">
            <el-option v-for="item in currentTemplates" :key="item.id" :label="item.name" :value="Number(item.id)" />
          </el-select>
        </el-form-item>
      </el-form>
      <template v-if="workMode === 'log'">
        <div class="task-log-toolbar"><span>接口按任务和模板返回首个匹配执行单元日志</span><el-button :loading="logLoading" @click="loadLog"><el-icon><Refresh /></el-icon>刷新</el-button></div>
        <pre class="task-log-viewer" v-loading="logLoading">{{ logContent || '暂无日志' }}</pre>
      </template>
      <div v-else class="task-inline-message is-warning"><el-icon><Warning /></el-icon>停止后，同一父任务下等待中或运行中的其他执行单元也会被停止。</div>
      <template #footer>
        <div class="task-dialog-footer">
          <el-button @click="workDialogVisible = false">关闭</el-button>
          <el-button v-if="workMode === 'stop'" type="danger" :disabled="!selectedTemplateId" :loading="workSubmitting" @click="confirmStop">确认停止</el-button>
        </div>
      </template>
    </el-dialog>
  </section>
</template>

<script setup>
import { onBeforeUnmount, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import HostSelectorDialog from '@/components/HostSelectorDialog.vue'
import taskApi from '@/api/task'
import {
  TASK_STATUSES, TASK_TYPES, TEMPLATE_TYPES, errorMessage, formatDate,
  formatDuration, hostLabel, optionOf
} from './taskCommon'

const loading = ref(false)
const submitting = ref(false)
const optionsLoading = ref(false)
const cronChecking = ref(false)
const detailLoading = ref(false)
const logLoading = ref(false)
const workSubmitting = ref(false)
const operatingId = ref(null)
const editingId = ref(null)
const originalStatus = ref(1)
const formVisible = ref(false)
const detailVisible = ref(false)
const hostSelectorVisible = ref(false)
const workDialogVisible = ref(false)
const workMode = ref('log')
const selectedTemplateId = ref(null)
const nextExecution = ref('')
const logContent = ref('')
const tasks = ref([])
const templateOptions = ref([])
const currentTemplates = ref([])
const formRef = ref()
const pollTimer = ref(null)

const filters = reactive({ name: '', type: null, status: null })
const pagination = reactive({ page: 1, pageSize: 10, total: 0 })
const currentTask = reactive({})
const detail = reactive({})
const form = reactive({ name: '', type: 1, templateIds: [], hostIds: [], hosts: [], cronExpr: '', remark: '' })

const rules = {
  name: [{ required: true, message: '请输入任务名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择任务类型', trigger: 'change' }],
  templateIds: [{ type: 'array', required: true, min: 1, message: '请至少选择一个模板', trigger: 'change' }],
  hostIds: [{ type: 'array', required: true, min: 1, message: '请至少选择一台主机', trigger: 'change' }],
  cronExpr: [{ validator: (_, value, callback) => {
    if (form.type === 2 && !String(value || '').trim()) callback(new Error('请输入五段式 Cron 表达式'))
    else callback()
  }, trigger: 'blur' }]
}

const taskType = value => optionOf(TASK_TYPES, value)
const taskStatus = value => optionOf(TASK_STATUSES, value)
const templateType = value => optionOf(TEMPLATE_TYPES, value)
const rowTemplates = row => Array.isArray(row.templates) ? row.templates : []
const rowHosts = row => Array.isArray(row.hosts) ? row.hosts : []
const canEdit = row => Number(row.status) !== 2
const canDelete = row => Number(row.status) !== 2
const canStop = row => Number(row.type) === 1 && Number(row.status) === 2
const canViewLog = row => [2, 3, 4, 6].includes(Number(row.status))

function primaryAction(row) {
  if (Number(row.type) === 1 && Number(row.status) !== 2) return { label: Number(row.execute_count) ? '重新执行' : '执行', action: 'start', type: 'primary' }
  if (Number(row.type) === 2 && Number(row.status) === 1) return { label: '启用', action: 'start', type: 'primary' }
  if (Number(row.type) === 2 && [3, 4, 6].includes(Number(row.status))) return { label: '重新启用', action: 'start', type: 'primary' }
  if (Number(row.type) === 2 && Number(row.status) === 2) return { label: '暂停', action: 'pause', type: 'warning' }
  if (Number(row.type) === 2 && Number(row.status) === 5) return { label: '恢复', action: 'resume', type: 'success' }
  return null
}

function normalizeArray(data) {
  if (Array.isArray(data)) return data
  return data?.list || data?.data || []
}

async function loadOptions() {
  optionsLoading.value = true
  try {
    templateOptions.value = normalizeArray(await taskApi.getTemplates())
  } catch (error) {
    ElMessage.error(errorMessage(error, '模板选项加载失败'))
  } finally {
    optionsLoading.value = false
  }
}

async function loadTasks() {
  loading.value = true
  try {
    if (filters.type) {
      let list = normalizeArray(await taskApi.queryTasksByType(filters.type))
      if (filters.name.trim()) list = list.filter(row => String(row.name || '').includes(filters.name.trim()))
      if (filters.status) list = list.filter(row => Number(row.status) === Number(filters.status))
      pagination.total = list.length
      const start = (pagination.page - 1) * pagination.pageSize
      tasks.value = list.slice(start, start + pagination.pageSize)
    } else {
      const page = await taskApi.getTasks({
        page: pagination.page,
        pageSize: pagination.pageSize,
        name: filters.name.trim() || undefined,
        status: filters.status || undefined
      })
      tasks.value = page?.list || []
      pagination.total = Number(page?.total || 0)
    }
    updatePolling()
  } catch (error) {
    tasks.value = []
    pagination.total = 0
    ElMessage.error(errorMessage(error, '任务列表加载失败'))
  } finally {
    loading.value = false
  }
}

function search() { pagination.page = 1; loadTasks() }
function refresh() { loadTasks() }
function handlePageSize() { pagination.page = 1; loadTasks() }

function resetForm() {
  editingId.value = null
  originalStatus.value = 1
  nextExecution.value = ''
  Object.assign(form, { name: '', type: 1, templateIds: [], hostIds: [], hosts: [], cronExpr: '', remark: '' })
  formRef.value?.clearValidate()
}

function openCreate() { resetForm(); formVisible.value = true }

async function openEdit(row) {
  editingId.value = row.id
  formVisible.value = true
  try {
    const data = await taskApi.getTask(row.id)
    const source = { ...row, ...(data || {}) }
    originalStatus.value = Number(source.status || 1)
    const templates = rowTemplates(source).length ? rowTemplates(source) : rowTemplates(row)
    const hosts = rowHosts(source).length ? rowHosts(source) : rowHosts(row)
    Object.assign(form, {
      name: source.name || '',
      type: Number(source.type || 1),
      templateIds: templates.length ? templates.map(item => Number(item.id)) : String(source.shell || '').split(',').filter(Boolean).map(Number),
      hostIds: hosts.length ? hosts.map(item => Number(item.id)) : String(source.host_ids || '').split(',').filter(Boolean).map(Number),
      hosts,
      cronExpr: source.cron_expr || '',
      remark: source.remark || ''
    })
  } catch (error) {
    formVisible.value = false
    ElMessage.error(errorMessage(error, '任务详情加载失败'))
  }
}

function handleHostsSelected(hosts) {
  const map = new Map(form.hosts.map(host => [Number(host.id), host]))
  hosts.forEach(host => map.set(Number(host.id), host))
  form.hosts = Array.from(map.values())
  form.hostIds = form.hosts.map(host => Number(host.id))
  formRef.value?.validateField('hostIds')
}

function removeSelectedHost(id) {
  form.hosts = form.hosts.filter(host => Number(host.id) !== Number(id))
  form.hostIds = form.hostIds.filter(hostId => Number(hostId) !== Number(id))
}

async function checkCron() {
  nextExecution.value = ''
  if (form.type !== 2 || !form.cronExpr.trim()) return
  cronChecking.value = true
  try {
    const result = await taskApi.getNextExecution(form.cronExpr.trim())
    nextExecution.value = result?.next_execution_time || result?.nextExecutionTime || result
    formRef.value?.clearValidate('cronExpr')
  } catch (error) {
    formRef.value?.validateField('cronExpr').catch(() => {})
    ElMessage.error(errorMessage(error, 'Cron 表达式无效'))
  } finally {
    cronChecking.value = false
  }
}

async function submitTask(startAfterCreate) {
  try { await formRef.value?.validate() } catch (_) { return }
  if (form.type === 2) {
    await checkCron()
    if (!nextExecution.value) return
  }
  submitting.value = true
  const payload = {
    name: form.name.trim(), type: Number(form.type),
    shell: form.templateIds.join(','), host_ids: form.hostIds.join(','),
    cron_expr: form.type === 2 ? form.cronExpr.trim() : '', remark: form.remark.trim()
  }
  try {
    if (editingId.value) {
      await taskApi.updateTask({ id: editingId.value, ...payload, status: originalStatus.value })
      ElMessage.success('任务已更新')
    } else {
      const created = await taskApi.createTask(payload)
      const taskId = created?.id || created?.task_id
      if (startAfterCreate) {
        if (!taskId) throw new Error('创建成功，但响应中缺少任务 ID，无法自动执行')
        await taskApi.startTask(taskId)
        ElMessage.success(form.type === 2 ? '任务已保存并启用调度' : '任务已保存并开始执行')
      } else ElMessage.success('任务已保存')
    }
    formVisible.value = false
    await loadTasks()
  } catch (error) {
    ElMessage.error(errorMessage(error, '任务保存失败'))
  } finally {
    submitting.value = false
  }
}

async function runPrimaryAction(row) {
  const action = primaryAction(row)?.action
  if (!action) return
  operatingId.value = row.id
  try {
    if (action === 'start') await taskApi.startTask(row.id)
    else {
      const state = await taskApi.getTaskOperationStatus(row.id)
      const operations = state?.operations || {}
      if (action === 'pause' && operations.can_pause === false) throw new Error('当前任务不可暂停，请刷新后重试')
      if (action === 'resume' && operations.can_resume === false) throw new Error('当前任务不可恢复，请刷新后重试')
      if (action === 'pause') await taskApi.pauseScheduledTask(row.id)
      if (action === 'resume') await taskApi.resumeScheduledTask(row.id)
    }
    ElMessage.success({ start: '任务已启动', pause: '调度已暂停', resume: '调度已恢复' }[action])
    await loadTasks()
  } catch (error) {
    ElMessage.error(errorMessage(error, '任务操作失败'))
  } finally {
    operatingId.value = null
  }
}

async function openDetail(row) {
  Object.assign(detail, row)
  detailVisible.value = true
  detailLoading.value = true
  try { Object.assign(detail, await taskApi.getTask(row.id) || {}) }
  catch (error) { ElMessage.error(errorMessage(error, '任务详情加载失败')) }
  finally { detailLoading.value = false }
}

async function resolveTemplates(row) {
  let list = rowTemplates(row)
  if (!list.length) {
    try { list = normalizeArray(await taskApi.getTaskTemplates(row.id)) } catch (_) { list = [] }
  }
  if (!list.length && row.shell) {
    const ids = String(row.shell).split(',').filter(Boolean).map(Number)
    list = templateOptions.value.filter(item => ids.includes(Number(item.id)))
  }
  return list
}

async function openWorkDialog(row, mode) {
  Object.keys(currentTask).forEach(key => delete currentTask[key])
  Object.assign(currentTask, row)
  workMode.value = mode
  logContent.value = ''
  selectedTemplateId.value = null
  currentTemplates.value = await resolveTemplates(row)
  selectedTemplateId.value = currentTemplates.value[0]?.id ? Number(currentTemplates.value[0].id) : null
  workDialogVisible.value = true
  if (mode === 'log' && selectedTemplateId.value) loadLog()
}

async function loadLog() {
  if (!selectedTemplateId.value) return
  logLoading.value = true
  try { logContent.value = await taskApi.getTaskLog(currentTask.id, selectedTemplateId.value) || '' }
  catch (error) { ElMessage.error(errorMessage(error, '任务日志加载失败')) }
  finally { logLoading.value = false }
}

async function confirmStop() {
  if (!selectedTemplateId.value) return
  workSubmitting.value = true
  try {
    await taskApi.stopTask(currentTask.id, selectedTemplateId.value)
    ElMessage.success('任务已停止')
    workDialogVisible.value = false
    await loadTasks()
  } catch (error) { ElMessage.error(errorMessage(error, '停止任务失败')) }
  finally { workSubmitting.value = false }
}

function closeWorkDialog() { logContent.value = ''; selectedTemplateId.value = null }

async function resetTask(row) {
  try {
    await ElMessageBox.confirm(`确定重置“${row.name}”的执行单元状态吗？`, '重置子任务', { type: 'warning' })
    const result = await taskApi.resetScheduledTask(row.id)
    ElMessage.success(`子任务状态已重置${result?.affected !== undefined ? `：${result.affected} 条` : ''}`)
    await loadTasks()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(errorMessage(error, '子任务重置失败'))
  }
}

async function removeTask(row) {
  try {
    await ElMessageBox.confirm(`确定删除任务“${row.name}”吗？关联执行单元也会被删除。`, '删除任务', { type: 'warning', confirmButtonText: '删除' })
    await taskApi.deleteTask(row.id)
    ElMessage.success('任务已删除')
    await loadTasks()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(errorMessage(error, '任务删除失败'))
  }
}

function handleMore(command, row) {
  if (command === 'log') openWorkDialog(row, 'log')
  if (command === 'stop') openWorkDialog(row, 'stop')
  if (command === 'edit') openEdit(row)
  if (command === 'reset') resetTask(row)
  if (command === 'delete') removeTask(row)
}

function updatePolling() {
  const needsPolling = tasks.value.some(row => Number(row.type) === 1 && Number(row.status) === 2)
  if (!needsPolling && pollTimer.value) { clearInterval(pollTimer.value); pollTimer.value = null }
  if (needsPolling && !pollTimer.value) pollTimer.value = setInterval(() => { if (!document.hidden) loadTasks() }, 2000)
}

onBeforeUnmount(() => { if (pollTimer.value) clearInterval(pollTimer.value) })
loadOptions()
loadTasks()
</script>
