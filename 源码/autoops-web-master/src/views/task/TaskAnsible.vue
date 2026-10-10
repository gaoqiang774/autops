<template>
  <section class="task-view" aria-labelledby="ansible-title">
    <div class="task-panel">
      <header class="task-panel__header">
        <div class="task-panel__title">
          <h2 id="ansible-title">Ansible 自动化</h2>
          <p>管理 Playbook、Git 仓库和 K8s 部署任务</p>
        </div>
        <div class="task-toolbar">
          <el-input v-model="filters.name" clearable placeholder="任务名称" aria-label="按自动化任务名称筛选" @keyup.enter="search" @clear="search">
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-select v-model="filters.type" clearable placeholder="全部来源" aria-label="按自动化任务来源筛选" @change="search">
            <el-option v-for="(item, value) in ANSIBLE_TYPES" :key="value" :label="item.label" :value="Number(value)" />
          </el-select>
          <el-button type="primary" :loading="loading" @click="search"><el-icon><Search /></el-icon>查询</el-button>
          <el-button class="ops-refresh-button" type="warning" :loading="loading" @click="loadTasks"><el-icon><Refresh /></el-icon>刷新</el-button>
          <el-button type="success" @click="openCreate"><el-icon><Plus /></el-icon>新建自动化任务</el-button>
        </div>
      </header>

      <div class="task-panel__body">
        <div class="task-table-wrap" v-loading="loading">
          <el-table :data="tasks" height="calc(100vh - 300px)" min-height="330" table-layout="fixed">
            <el-table-column label="任务名称" min-width="220" fixed="left">
              <template #default="{ row }">
                <div class="task-name-cell"><strong>{{ row.name || '--' }}</strong><span>{{ row.description || row.gitRepo || `任务 ID: ${row.id}` }}</span></div>
              </template>
            </el-table-column>
            <el-table-column label="来源" width="112">
              <template #default="{ row }"><el-tag :type="ansibleType(row.type).type" effect="light">{{ ansibleType(row.type).label }}</el-tag></template>
            </el-table-column>
            <el-table-column label="主机数量" width="95" align="right">
              <template #default="{ row }">{{ hostCount(row) }}</template>
            </el-table-column>
            <el-table-column label="Playbook" width="95" align="right">
              <template #default="{ row }">{{ Number(row.taskCount || row.task_count || 0) }}</template>
            </el-table-column>
            <el-table-column label="状态" width="108">
              <template #default="{ row }"><span class="task-status"><el-icon :class="{ 'is-loading': Number(row.status) === 2 }"><component :is="taskStatus(row.status).icon" /></el-icon>{{ taskStatus(row.status).label }}</span></template>
            </el-table-column>
            <el-table-column label="总耗时" width="105" align="right">
              <template #default="{ row }">{{ formatDuration(row.totalDuration || row.total_duration) }}</template>
            </el-table-column>
            <el-table-column label="更新时间" width="175">
              <template #default="{ row }">{{ formatDate(row.updatedAt || row.updated_at || row.createdAt || row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="190" fixed="right" align="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="openDetail(row)">详情</el-button>
                <el-button v-if="Number(row.status) !== 2" link type="primary" :loading="operatingId === row.id" @click="startTask(row)">启动</el-button>
                <el-button v-if="Number(row.status) !== 2" link type="danger" @click="removeTask(row)">删除</el-button>
              </template>
            </el-table-column>
            <template #empty><el-empty description="暂无自动化任务"><el-button type="success" @click="openCreate">新建任务</el-button></el-empty></template>
          </el-table>
        </div>
        <div class="task-pagination">
          <el-pagination v-model:current-page="pagination.page" v-model:page-size="pagination.size" background layout="total, sizes, prev, pager, next" :page-sizes="[10, 20, 50, 100]" :total="pagination.total" @current-change="loadTasks" @size-change="handlePageSize" />
        </div>
      </div>
    </div>

    <el-dialog v-model="createVisible" class="task-dialog" modal-class="task-form-dialog-overlay" width="900px" destroy-on-close :close-on-click-modal="false">
      <template #header>
        <div class="task-dialog-title">
          <span class="task-dialog-title__icon"><el-icon><Connection /></el-icon></span>
          <div><h2>新建自动化任务</h2><p>选择上传 Playbook、从 Git 导入或生成 K8s 部署任务</p></div>
        </div>
      </template>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" class="task-dialog" @submit.prevent>
        <el-form-item label="创建方式" prop="type">
          <el-radio-group v-model="form.type" @change="resetTypeFields">
            <el-radio-button :value="1">手动上传</el-radio-button>
            <el-radio-button :value="2">Git 导入</el-radio-button>
            <el-radio-button :value="3">K8s 部署</el-radio-button>
          </el-radio-group>
        </el-form-item>

        <div v-if="form.type !== 3" class="task-form-grid">
          <el-form-item label="任务名称" prop="name"><el-input v-model="form.name" maxlength="100" show-word-limit placeholder="请输入唯一任务名称" /></el-form-item>
          <el-form-item label="主机组名称" prop="groupName"><el-input v-model="form.groupName" maxlength="50" placeholder="例如：all、web、db" /></el-form-item>
          <el-form-item v-if="form.type === 2" class="is-full" label="Git 仓库" prop="gitRepo"><el-input v-model="form.gitRepo" placeholder="https://git.example.com/ops/playbooks.git" /></el-form-item>
          <el-form-item class="is-full" label="执行主机" prop="hostIds">
            <div style="width:100%">
              <el-button type="primary" plain @click="selectHosts('hosts')"><el-icon><Plus /></el-icon>选择主机</el-button>
              <span class="task-empty-inline" style="margin-left:10px">已选择 {{ form.hostIds.length }} 台，不支持跳板机认证主机</span>
              <div v-if="form.hosts.length" class="task-chip-list" style="margin-top:8px">
                <el-tag v-for="host in form.hosts" :key="host.id" closable type="info" @close="removeHost('hosts', host.id)">{{ hostLabel(host) }}</el-tag>
              </div>
            </div>
          </el-form-item>
          <el-form-item v-if="form.type === 1" class="is-full" label="Playbook 文件" required>
            <el-upload v-model:file-list="playbookFiles" action="#" :auto-upload="false" multiple accept=".yml,.yaml" :on-change="validatePlaybookFile">
              <el-button type="primary" plain><el-icon><Upload /></el-icon>选择 YAML 文件</el-button>
              <template #tip><div class="el-upload__tip">至少一个 `.yml` 或 `.yaml` 普通文件名</div></template>
            </el-upload>
          </el-form-item>
          <el-form-item v-if="form.type === 1" class="is-full" label="Roles 压缩包">
            <el-upload v-model:file-list="roleFiles" action="#" :auto-upload="false" :limit="1" accept=".zip" :on-exceed="replaceRoleFile">
              <el-button plain><el-icon><Files /></el-icon>选择 ZIP 文件</el-button>
            </el-upload>
          </el-form-item>
          <el-form-item class="is-full" label="全局变量（JSON）" prop="variables">
            <el-input v-model="form.variables" class="task-code-input" type="textarea" :rows="5" spellcheck="false" placeholder='{"env":"prod","app_version":"v1.6.0"}' />
          </el-form-item>
        </div>

        <div v-else class="task-form-grid">
          <el-form-item label="任务名称" prop="name"><el-input v-model="form.name" maxlength="100" placeholder="例如：生产 K8s 集群部署" /></el-form-item>
          <el-form-item label="集群名称" prop="clusterName"><el-input v-model="form.clusterName" maxlength="63" placeholder="例如：prod-cluster" /></el-form-item>
          <el-form-item label="Kubernetes 版本" prop="clusterVersion"><el-input v-model="form.clusterVersion" placeholder="例如：v1.30.2" /></el-form-item>
          <el-form-item label="部署模式" prop="deploymentMode">
            <el-radio-group v-model="form.deploymentMode"><el-radio :value="1">单节点</el-radio><el-radio :value="2">集群</el-radio></el-radio-group>
          </el-form-item>
          <el-form-item class="is-full" label="Master 节点" required><HostRoleField :hosts="form.masterHosts" label="选择 Master" @select="selectHosts('master')" @remove="id => removeHost('master', id)" /></el-form-item>
          <el-form-item class="is-full" label="ETCD 节点" required><HostRoleField :hosts="form.etcdHosts" label="选择 ETCD" @select="selectHosts('etcd')" @remove="id => removeHost('etcd', id)" /></el-form-item>
          <el-form-item class="is-full" label="Worker 节点"><HostRoleField :hosts="form.workerHosts" label="选择 Worker" @select="selectHosts('worker')" @remove="id => removeHost('worker', id)" /></el-form-item>
          <el-form-item class="is-full" label="启用组件"><el-checkbox-group v-model="form.enabledComponents"><el-checkbox value="calico">Calico</el-checkbox><el-checkbox value="coredns">CoreDNS</el-checkbox><el-checkbox value="metrics-server">Metrics Server</el-checkbox><el-checkbox value="ingress-nginx">Ingress Nginx</el-checkbox></el-checkbox-group></el-form-item>
          <el-form-item class="is-full" label="任务描述"><el-input v-model="form.description" type="textarea" :rows="2" maxlength="500" show-word-limit /></el-form-item>
          <el-form-item class="is-full" label="私有镜像仓库"><el-input v-model="form.privateRegistry" placeholder="留空则使用公共镜像仓库" /></el-form-item>
          <template v-if="form.privateRegistry.trim()">
            <el-form-item label="仓库用户名"><el-input v-model="form.registryUsername" autocomplete="off" /></el-form-item>
            <el-form-item label="仓库密码"><el-input v-model="form.registryPassword" type="password" show-password autocomplete="new-password" /></el-form-item>
          </template>
        </div>
      </el-form>
      <div v-if="form.type === 2" class="task-inline-message"><el-icon><InfoFilled /></el-icon>Git 导入可能持续数分钟，提交期间请勿重复操作。</div>
      <template #footer>
        <div class="task-dialog-footer"><el-button @click="createVisible = false">取消</el-button><el-button type="primary" :loading="submitting" @click="submitTask">创建任务</el-button></div>
      </template>
    </el-dialog>

    <HostSelectorDialog v-model="hostSelectorVisible" :title="hostSelectorTitle" @hosts-selected="handleHostsSelected" />

    <el-dialog v-model="detailVisible" class="task-dialog" modal-class="task-drawer-dialog-overlay" width="800px" destroy-on-close @closed="closeDetail">
      <template #header>
        <div class="task-dialog-title"><span class="task-dialog-title__icon"><el-icon><List /></el-icon></span><div><h2>{{ detail.name || '自动化任务详情' }}</h2><p>每个 Playbook 子任务可独立查看日志或停止</p></div></div>
      </template>
      <dl class="task-detail-list" v-loading="detailLoading">
        <div class="task-detail-item"><dt>任务 ID</dt><dd>{{ detail.id || '--' }}</dd></div>
        <div class="task-detail-item"><dt>任务名称</dt><dd>{{ detail.name || '--' }}</dd></div>
        <div class="task-detail-item"><dt>子任务数量</dt><dd>{{ detail.taskCount || detail.works.length || 0 }}</dd></div>
        <div class="task-detail-item"><dt>当前状态</dt><dd>{{ taskStatus(detail.status).label }}</dd></div>
      </dl>
      <h3 class="task-section-title">执行单元</h3>
      <div v-if="detail.works.length" class="task-work-list">
        <div v-for="work in detail.works" :key="work.workid" class="task-work-row">
          <span class="task-work-row__name">{{ work.EntryFileName || `执行单元 ${work.workid}` }}</span>
          <span class="task-status"><el-icon :class="{ 'is-loading': Number(work.status) === 2 }"><component :is="taskStatus(work.status).icon" /></el-icon>{{ taskStatus(work.status).label }}</span>
          <span>{{ formatDuration(work.Duration) }}</span>
          <span>
            <el-button link type="primary" @click="openLog(work)">日志</el-button>
            <el-button v-if="Number(work.status) === 2" link type="danger" :loading="stoppingWorkId === work.workid" @click="stopWork(work)">停止</el-button>
          </span>
        </div>
      </div>
      <el-empty v-else description="暂无执行单元" />
    </el-dialog>

    <el-dialog v-model="logVisible" class="task-dialog" modal-class="task-drawer-dialog-overlay" width="820px" destroy-on-close @closed="closeLog">
      <template #header><div class="task-dialog-title"><span class="task-dialog-title__icon"><el-icon><Document /></el-icon></span><div><h2>{{ logWork.EntryFileName || '实时日志' }}</h2><p>执行单元 ID: {{ logWork.workid || '--' }}</p></div></div></template>
      <div class="task-log-toolbar"><span>{{ logConnected ? '实时连接中' : '日志连接已结束' }}</span><el-button @click="clearLog"><el-icon><Delete /></el-icon>清屏</el-button></div>
      <pre ref="logViewer" class="task-log-viewer">{{ logContent || '等待日志输出...' }}</pre>
    </el-dialog>
  </section>
</template>

<script setup>
import { computed, defineComponent, h, nextTick, onBeforeUnmount, reactive, ref } from 'vue'
import { ElButton, ElIcon, ElMessage, ElMessageBox, ElTag } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import HostSelectorDialog from '@/components/HostSelectorDialog.vue'
import taskApi from '@/api/task'
import { ANSIBLE_TYPES, TASK_STATUSES, errorMessage, formatDate, formatDuration, hostLabel, optionOf, safeJson } from './taskCommon'

const HostRoleField = defineComponent({
  props: { hosts: { type: Array, default: () => [] }, label: { type: String, default: '选择主机' } },
  emits: ['select', 'remove'],
  setup(props, { emit }) {
    return () => h('div', { style: 'width:100%' }, [
      h(ElButton, { type: 'primary', plain: true, onClick: () => emit('select') }, { default: () => [h(ElIcon, null, { default: () => h(Plus) }), props.label] }),
      h('span', { class: 'task-empty-inline', style: 'margin-left:10px' }, `已选择 ${props.hosts.length} 台`),
      props.hosts.length ? h('div', { class: 'task-chip-list', style: 'margin-top:8px' }, props.hosts.map(host => h(ElTag, {
        key: host.id, closable: true, type: 'info', onClose: () => emit('remove', host.id)
      }, { default: () => hostLabel(host) }))) : null
    ])
  }
})

const loading = ref(false)
const submitting = ref(false)
const detailLoading = ref(false)
const operatingId = ref(null)
const stoppingWorkId = ref(null)
const createVisible = ref(false)
const hostSelectorVisible = ref(false)
const detailVisible = ref(false)
const logVisible = ref(false)
const logConnected = ref(false)
const hostTarget = ref('hosts')
const tasks = ref([])
const playbookFiles = ref([])
const roleFiles = ref([])
const formRef = ref()
const logViewer = ref()
const listPollTimer = ref(null)
const detailPollTimer = ref(null)
let closeLogSource = null

const filters = reactive({ name: '', type: null })
const pagination = reactive({ page: 1, size: 10, total: 0 })
const detail = reactive({ id: null, name: '', taskCount: 0, status: 1, works: [] })
const logWork = reactive({})
const logContent = ref('')
const form = reactive({
  type: 1, name: '', groupName: 'all', hostIds: [], hosts: [], gitRepo: '', variables: '{}',
  description: '', clusterName: '', clusterVersion: 'v1.30.2', deploymentMode: 2,
  masterHosts: [], workerHosts: [], etcdHosts: [], enabledComponents: ['calico', 'coredns'],
  privateRegistry: '', registryUsername: '', registryPassword: ''
})

const rules = {
  type: [{ required: true, message: '请选择创建方式', trigger: 'change' }],
  name: [{ required: true, message: '请输入任务名称', trigger: 'blur' }],
  groupName: [{ validator: (_, value, callback) => form.type !== 3 && !String(value || '').trim() ? callback(new Error('请输入主机组名称')) : callback(), trigger: 'blur' }],
  hostIds: [{ validator: (_, value, callback) => form.type !== 3 && !value.length ? callback(new Error('请至少选择一台主机')) : callback(), trigger: 'change' }],
  gitRepo: [{ validator: (_, value, callback) => form.type === 2 && !String(value || '').trim() ? callback(new Error('请输入 Git 仓库地址')) : callback(), trigger: 'blur' }],
  clusterName: [{ validator: (_, value, callback) => form.type === 3 && !String(value || '').trim() ? callback(new Error('请输入集群名称')) : callback(), trigger: 'blur' }],
  clusterVersion: [{ validator: (_, value, callback) => form.type === 3 && !String(value || '').trim() ? callback(new Error('请输入 Kubernetes 版本')) : callback(), trigger: 'blur' }],
  deploymentMode: [{ required: true, message: '请选择部署模式', trigger: 'change' }]
}

const ansibleType = value => optionOf(ANSIBLE_TYPES, value)
const taskStatus = value => optionOf(TASK_STATUSES, value)
const hostSelectorTitle = computed(() => ({ hosts: '选择 Ansible 执行主机', master: '选择 Master 节点', worker: '选择 Worker 节点', etcd: '选择 ETCD 节点' }[hostTarget.value]))

function hostCount(row) {
  const ids = safeJson(row.allHostIDs || row.all_host_ids, [])
  if (Array.isArray(ids)) return ids.length
  const groups = safeJson(row.hostGroups || row.host_groups, {})
  return Object.values(groups || {}).reduce((sum, list) => sum + (Array.isArray(list) ? list.length : 0), 0)
}

async function loadTasks() {
  loading.value = true
  try {
    let result
    if (filters.name.trim()) result = await taskApi.queryAnsibleByName(filters.name.trim())
    else if (filters.type) result = await taskApi.queryAnsibleByType(filters.type)
    else result = await taskApi.getAnsibleTasks({ page: pagination.page, size: pagination.size })
    let list = result.list || []
    if (filters.name.trim() && filters.type) list = list.filter(row => Number(row.type) === Number(filters.type))
    if (filters.name.trim() || filters.type) {
      const start = (pagination.page - 1) * pagination.size
      tasks.value = list.slice(start, start + pagination.size)
    } else tasks.value = list
    pagination.total = Number(result.total || 0)
    updateListPolling()
  } catch (error) {
    tasks.value = []
    pagination.total = 0
    ElMessage.error(errorMessage(error, '自动化任务列表加载失败'))
  } finally { loading.value = false }
}

function search() { pagination.page = 1; loadTasks() }
function handlePageSize() { pagination.page = 1; loadTasks() }

function resetForm() {
  Object.assign(form, {
    type: 1, name: '', groupName: 'all', hostIds: [], hosts: [], gitRepo: '', variables: '{}',
    description: '', clusterName: '', clusterVersion: 'v1.30.2', deploymentMode: 2,
    masterHosts: [], workerHosts: [], etcdHosts: [], enabledComponents: ['calico', 'coredns'],
    privateRegistry: '', registryUsername: '', registryPassword: ''
  })
  playbookFiles.value = []
  roleFiles.value = []
}

function openCreate() { resetForm(); createVisible.value = true }
function resetTypeFields() { formRef.value?.clearValidate(); playbookFiles.value = []; roleFiles.value = [] }
function selectHosts(target) { hostTarget.value = target; hostSelectorVisible.value = true }

function targetHosts(target) {
  return { hosts: 'hosts', master: 'masterHosts', worker: 'workerHosts', etcd: 'etcdHosts' }[target]
}

function handleHostsSelected(hosts) {
  const key = targetHosts(hostTarget.value)
  const map = new Map(form[key].map(host => [Number(host.id), host]))
  hosts.forEach(host => map.set(Number(host.id), host))
  form[key] = Array.from(map.values())
  if (key === 'hosts') {
    form.hostIds = form.hosts.map(host => Number(host.id))
    formRef.value?.validateField('hostIds')
  }
}

function removeHost(target, id) {
  const key = targetHosts(target)
  form[key] = form[key].filter(host => Number(host.id) !== Number(id))
  if (key === 'hosts') form.hostIds = form.hosts.map(host => Number(host.id))
}

function validatePlaybookFile(uploadFile) {
  const name = uploadFile.name || ''
  if (name.includes('/') || name.includes('\\') || !/\.ya?ml$/i.test(name)) {
    playbookFiles.value = playbookFiles.value.filter(file => file.uid !== uploadFile.uid)
    ElMessage.error('Playbook 只能是普通文件名的 .yml 或 .yaml 文件')
  }
}

function replaceRoleFile(files) {
  roleFiles.value = files.slice(-1).map(file => ({ name: file.name, raw: file }))
}

function parseVariables() {
  let variables
  try { variables = JSON.parse(form.variables || '{}') }
  catch (_) { throw new Error('全局变量必须是有效 JSON') }
  if (!variables || Array.isArray(variables) || typeof variables !== 'object') throw new Error('全局变量必须是 JSON 对象')
  return Object.fromEntries(Object.entries(variables).map(([key, value]) => [key, String(value)]))
}

async function submitTask() {
  try { await formRef.value?.validate() } catch (_) { return }
  if (form.type === 1 && !playbookFiles.value.length) return ElMessage.error('请至少选择一个 Playbook 文件')
  if (form.type === 3 && (!form.masterHosts.length || !form.etcdHosts.length)) return ElMessage.error('Master 和 ETCD 至少各选择一台主机')
  submitting.value = true
  try {
    const data = new FormData()
    if (form.type === 3) {
      data.append('name', form.name.trim())
      data.append('description', form.description.trim())
      data.append('cluster_name', form.clusterName.trim())
      data.append('cluster_version', form.clusterVersion.trim())
      data.append('deployment_mode', String(form.deploymentMode))
      data.append('master_host_ids', JSON.stringify(form.masterHosts.map(host => Number(host.id))))
      data.append('worker_host_ids', JSON.stringify(form.workerHosts.map(host => Number(host.id))))
      data.append('etcd_host_ids', JSON.stringify(form.etcdHosts.map(host => Number(host.id))))
      data.append('enabled_components', JSON.stringify(form.enabledComponents))
      data.append('private_registry', form.privateRegistry.trim())
      data.append('registry_username', form.registryUsername)
      data.append('registry_password', form.registryPassword)
      await taskApi.createK8sTask(data)
    } else {
      data.append('name', form.name.trim())
      data.append('type', String(form.type))
      data.append('hostGroups', JSON.stringify({ [form.groupName.trim()]: form.hostIds }))
      data.append('variables', JSON.stringify(parseVariables()))
      if (form.type === 1) {
        playbookFiles.value.forEach(file => data.append('playbooks', file.raw))
        if (roleFiles.value[0]?.raw) data.append('roles', roleFiles.value[0].raw)
      } else data.append('gitRepo', form.gitRepo.trim())
      await taskApi.createAnsibleTask(data)
    }
    ElMessage.success('自动化任务已创建')
    createVisible.value = false
    await loadTasks()
  } catch (error) { ElMessage.error(errorMessage(error, '自动化任务创建失败')) }
  finally { submitting.value = false }
}

async function startTask(row) {
  operatingId.value = row.id
  try { await taskApi.startAnsibleTask(row.id); ElMessage.success('任务已开始执行'); await loadTasks() }
  catch (error) { ElMessage.error(errorMessage(error, '任务启动失败')) }
  finally { operatingId.value = null }
}

async function removeTask(row) {
  try {
    await ElMessageBox.confirm(`确定删除自动化任务“${row.name}”吗？`, '删除任务', { type: 'warning', confirmButtonText: '删除' })
    await taskApi.deleteAnsibleTask(row.id)
    ElMessage.success('任务已删除')
    await loadTasks()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') ElMessage.error(errorMessage(error, '任务删除失败'))
  }
}

async function openDetail(row) {
  Object.assign(detail, { id: row.id, name: row.name, taskCount: row.taskCount || 0, status: row.status, works: [] })
  detailVisible.value = true
  await loadDetail()
}

async function loadDetail() {
  if (!detail.id) return
  detailLoading.value = true
  try {
    const result = await taskApi.getAnsibleTask(detail.id)
    const info = result?.task_info || result || {}
    detail.id = info.ID || detail.id
    detail.name = info.Name || detail.name
    detail.taskCount = info.TaskCount || 0
    detail.works = Array.isArray(info.Works) ? info.Works : []
    const statuses = detail.works.map(work => Number(work.status))
    detail.status = statuses.includes(2) ? 2 : (statuses.includes(4) ? 4 : (statuses.length && statuses.every(status => status === 3) ? 3 : detail.status))
    updateDetailPolling()
  } catch (error) { ElMessage.error(errorMessage(error, '任务详情加载失败')) }
  finally { detailLoading.value = false }
}

async function stopWork(work) {
  stoppingWorkId.value = work.workid
  try { await taskApi.stopAnsibleWork(detail.id, work.workid); ElMessage.success('执行单元已停止'); await loadDetail() }
  catch (error) { ElMessage.error(errorMessage(error, '执行单元停止失败')) }
  finally { stoppingWorkId.value = null }
}

function openLog(work) {
  Object.assign(logWork, work)
  logContent.value = ''
  logVisible.value = true
  logConnected.value = true
  closeLogSource?.()
  closeLogSource = taskApi.openAnsibleLog(detail.id, work.workid, {
    onLog: line => appendLog(line),
    onComplete: line => { appendLog(line); logConnected.value = false; loadDetail() },
    onError: () => { logConnected.value = false }
  })
}

function appendLog(line) {
  logContent.value += `${logContent.value ? '\n' : ''}${line}`
  nextTick(() => { if (logViewer.value) logViewer.value.scrollTop = logViewer.value.scrollHeight })
}

function clearLog() { logContent.value = '' }
function closeLog() { closeLogSource?.(); closeLogSource = null; logConnected.value = false; logContent.value = '' }
function closeDetail() { if (detailPollTimer.value) clearInterval(detailPollTimer.value); detailPollTimer.value = null }

function updateListPolling() {
  const active = tasks.value.some(row => Number(row.status) === 2)
  if (!active && listPollTimer.value) { clearInterval(listPollTimer.value); listPollTimer.value = null }
  if (active && !listPollTimer.value) listPollTimer.value = setInterval(() => { if (!document.hidden) loadTasks() }, 3000)
}

function updateDetailPolling() {
  const active = detail.works.some(work => [1, 2].includes(Number(work.status)))
  if (!active && detailPollTimer.value) { clearInterval(detailPollTimer.value); detailPollTimer.value = null }
  if (active && !detailPollTimer.value) detailPollTimer.value = setInterval(() => { if (!document.hidden) loadDetail() }, 2000)
}

onBeforeUnmount(() => {
  if (listPollTimer.value) clearInterval(listPollTimer.value)
  if (detailPollTimer.value) clearInterval(detailPollTimer.value)
  closeLogSource?.()
})

loadTasks()
</script>
