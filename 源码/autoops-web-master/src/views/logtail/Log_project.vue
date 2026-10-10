<template>
  <div class="log-page log-manage-page">
    <section class="page-card">
      <header class="page-header">
        <div class="page-header-copy">
          <span class="ops-page-header__icon" aria-hidden="true">
            <img :src="logSvg" alt="">
          </span>
          <div class="ops-page-header__content">
            <h1>日志库管理</h1>
            <span class="ops-page-header__divider" aria-hidden="true"></span>
            <p class="page-description">按项目组织日志库，统一维护日志查询、采集与告警所需的标准日志集合。</p>
          </div>
        </div>
        <div class="summary-group">
          <div class="summary-chip">
            <span>项目数</span>
            <strong>{{ projectPagination.total }}</strong>
          </div>
          <div class="summary-chip">
            <span>当前日志库</span>
            <strong>{{ storePagination.total }}</strong>
          </div>
        </div>
      </header>

      <div class="manage-layout">
        <aside class="project-panel">
          <div class="panel-head">
            <h3>项目列表</h3>
            <div class="project-head-actions">
              <el-tooltip content="新增项目" placement="top">
                <el-button
                  class="project-action-btn"
                  type="success"
                  size="small"
                  :icon="Plus"
                  aria-label="新增项目"
                  @click="openProjectDialog()"
                />
              </el-tooltip>
              <el-tooltip content="刷新项目" placement="top">
                <el-button
                  class="project-action-btn refresh-action-btn"
                  type="warning"
                  size="small"
                  :icon="Refresh"
                  :loading="projectLoading"
                  aria-label="刷新项目"
                  @click="refreshProjects"
                />
              </el-tooltip>
            </div>
          </div>

          <el-form :model="projectQuery" class="compact-form" @submit.prevent>
            <el-form-item>
              <el-input
                v-model.trim="projectQuery.keyword"
                placeholder="项目名称 / 编码 / 负责人"
                clearable
                @keyup.enter="handleProjectSearch"
                @clear="resetProjectSearch"
              >
                <template #prefix>
                  <el-icon><Search /></el-icon>
                </template>
              </el-input>
            </el-form-item>
          </el-form>

          <div v-loading="projectLoading" class="project-list">
            <button
              v-for="project in projects"
              :key="project.id"
              type="button"
              class="project-item"
              :class="{ active: selectedProjectId === project.id }"
              :title="project.name || '-'"
              @click="selectProject(project)"
            >
              <img
                :src="selectedProjectId === project.id ? openFolderIcon : closedFolderIcon"
                alt=""
                class="project-folder-icon"
              />
              <span class="project-node-label">{{ project.name || '-' }}</span>
              <span class="project-node-count" :title="`日志库 ${projectStoreCount(project)} 个，启用 ${projectEnabledStoreCount(project)} 个`">
                {{ projectStoreCount(project) }}
              </span>
              <span class="project-status-dot" :class="{ disabled: Number(project.status) === 2 }"></span>
            </button>
            <el-empty v-if="!projects.length && !projectLoading" description="暂无项目" :image-size="88" />
          </div>

        </aside>

        <main class="store-panel">
          <div class="panel-head store-head">
            <div>
              <h3>{{ currentProject?.name || '日志库列表' }}</h3>
            </div>
            <div class="store-head-actions">
              <el-button
                type="primary"
                size="small"
                :icon="Edit"
                :disabled="!currentProject"
                @click="openProjectDialog(currentProject)"
              >
                编辑项目
              </el-button>
              <el-button
                type="danger"
                size="small"
                :icon="Delete"
                :disabled="!currentProject"
                @click="removeProject(currentProject)"
              >
                删除项目
              </el-button>
              <el-button
                type="success"
                size="small"
                :icon="Plus"
                :disabled="!currentProject"
                @click="openStoreDialog()"
              >
                新增日志库
              </el-button>
            </div>
          </div>

          <div class="toolbar-card">
            <el-form :model="storeQuery" inline class="toolbar-form" @submit.prevent>
              <el-form-item label="日志库">
                <el-input
                  v-model.trim="storeQuery.keyword"
                  placeholder="名称 / 编码 / 描述"
                  clearable
                  @keyup.enter="loadStores"
                />
              </el-form-item>
              <el-form-item label="状态">
                <el-select v-model="storeQuery.status" placeholder="全部" clearable @change="loadStores">
                  <el-option label="启用" :value="1" />
                  <el-option label="禁用" :value="2" />
                </el-select>
              </el-form-item>
              <el-form-item>
                <div class="toolbar-actions">
                  <el-button type="primary" size="small" :icon="Search" :loading="storeLoading" @click="handleStoreSearch">搜索</el-button>
                  <el-button type="warning" size="small" :icon="Refresh" @click="resetStoreSearch">重置</el-button>
                </div>
              </el-form-item>
            </el-form>
          </div>

          <div class="table-shell">
            <el-table
              v-loading="storeLoading"
              :data="stores"
              class="data-table"
              height="100%"
              empty-text="暂无日志库"
            >
              <el-table-column prop="name" label="日志库名称" min-width="150" show-overflow-tooltip>
                <template #default="{ row }">
                  <div class="name-cell">
                    <strong>{{ row.name || '-' }}</strong>
                  </div>
                </template>
              </el-table-column>
              <el-table-column prop="code" label="日志库编码" min-width="140">
                <template #default="{ row }">
                  <span class="mono-text">{{ row.code || '-' }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="storeType" label="类型" width="84">
                <template #default="{ row }">
                  <el-tag size="small" effect="plain">{{ row.storeType || 'app' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="retentionDays" label="保留" width="74">
                <template #default="{ row }">{{ row.retentionDays || 7 }} 天</template>
              </el-table-column>
              <el-table-column prop="status" label="状态" width="78">
                <template #default="{ row }">
                  <el-switch
                    :model-value="Number(row.status) === 1"
                    :loading="statusUpdatingId === row.id"
                    active-color="#67c23a"
                    inactive-color="#cbd5e1"
                    @change="toggleStoreStatus(row)"
                  />
                </template>
              </el-table-column>
              <el-table-column prop="updatedAt" label="更新时间" min-width="145" show-overflow-tooltip>
                <template #default="{ row }">{{ formatTime(row.updatedAt || row.updateTime) }}</template>
              </el-table-column>
              <el-table-column label="操作" fixed="right" width="138">
                <template #default="{ row }">
                  <div class="operation-links">
                    <el-button link type="primary" @click="goQuery(row)">查询</el-button>
                    <el-button link type="primary" @click="openStoreDialog(row)">修改</el-button>
                    <el-button link type="danger" @click="removeStore(row)">删除</el-button>
                  </div>
                </template>
              </el-table-column>
            </el-table>
          </div>

          <div class="pagination-section">
            <el-pagination
              :current-page="storePagination.page"
              :page-size="storePagination.pageSize"
              :page-sizes="[10, 20, 50, 100]"
              :total="storePagination.total"
              layout="total, sizes, prev, pager, next, jumper"
              @size-change="handleStoreSizeChange"
              @current-change="handleStorePageChange"
            />
          </div>
        </main>
      </div>
    </section>

    <el-dialog
      v-model="projectDialog.visible"
      :title="projectDialog.form.id ? '编辑项目' : '新增项目'"
      width="620px"
      destroy-on-close
      class="log-dialog"
      modal-class="log-project-dialog-overlay"
    >
      <el-form ref="projectFormRef" :model="projectDialog.form" :rules="projectRules" label-width="96px">
        <el-form-item label="项目名称" prop="name">
          <el-input v-model.trim="projectDialog.form.name" placeholder="例如：订单项目" />
        </el-form-item>
        <el-form-item label="负责人">
          <el-input v-model.trim="projectDialog.form.ownerName" placeholder="负责人或团队名称" />
        </el-form-item>
        <el-form-item label="环境">
          <el-select v-model="projectDialog.form.env" placeholder="选择环境" clearable>
            <el-option label="生产" value="prod" />
            <el-option label="测试" value="test" />
            <el-option label="开发" value="dev" />
          </el-select>
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="projectDialog.form.status">
            <el-radio-button :label="1">启用</el-radio-button>
            <el-radio-button :label="2">禁用</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="projectDialog.form.description" type="textarea" :rows="3" placeholder="项目说明" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="projectDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="projectDialog.submitting" @click="submitProject">确认</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="storeDialog.visible"
      :title="storeDialog.form.id ? '编辑日志库' : '新增日志库'"
      width="680px"
      destroy-on-close
      class="log-dialog"
      modal-class="log-project-dialog-overlay"
    >
      <el-form ref="storeFormRef" :model="storeDialog.form" :rules="storeRules" label-width="104px">
        <el-form-item label="所属项目">
          <el-input :model-value="currentProject?.name || '-'" disabled />
        </el-form-item>
        <el-form-item label="日志库名称" prop="name">
          <el-input v-model.trim="storeDialog.form.name" placeholder="例如：应用日志" />
        </el-form-item>
        <el-form-item label="类型">
          <el-select v-model="storeDialog.form.storeType" placeholder="选择类型">
            <el-option label="应用日志" value="app" />
            <el-option label="访问日志" value="access" />
            <el-option label="审计日志" value="audit" />
            <el-option label="系统日志" value="system" />
            <el-option label="Kubernetes" value="k8s" />
          </el-select>
        </el-form-item>
        <el-form-item label="保留天数">
          <el-input-number v-model="storeDialog.form.retentionDays" :min="1" :max="365" controls-position="right" />
        </el-form-item>
        <el-form-item label="状态">
          <el-radio-group v-model="storeDialog.form.status">
            <el-radio-button :label="1">启用</el-radio-button>
            <el-radio-button :label="2">禁用</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="storeDialog.form.description" type="textarea" :rows="3" placeholder="日志库说明" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="storeDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="storeDialog.submitting" @click="submitStore">确认</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Edit, Plus, Refresh, Search } from '@element-plus/icons-vue'
import logtailApi from '@/api/logtail'
import logSvg from '@/assets/image/日志.svg'

const router = useRouter()
const openFolderIcon = new URL('@/assets/image/打开文件夹.svg', import.meta.url).href
const closedFolderIcon = new URL('@/assets/image/关闭文件夹.svg', import.meta.url).href

const projects = ref([])
const stores = ref([])
const projectLoading = ref(false)
const storeLoading = ref(false)
const selectedProjectId = ref(null)
const projectFormRef = ref()
const storeFormRef = ref()
const statusUpdatingId = ref(null)

const projectQuery = reactive({
  keyword: ''
})

const storeQuery = reactive({
  keyword: '',
  status: ''
})

const projectPagination = reactive({
  page: 1,
  pageSize: 200,
  total: 0
})

const storePagination = reactive({
  page: 1,
  pageSize: 10,
  total: 0
})

const projectDialog = reactive({
  visible: false,
  submitting: false,
  form: createProjectForm()
})

const storeDialog = reactive({
  visible: false,
  submitting: false,
  form: createStoreForm()
})

const currentProject = computed(() => projects.value.find(item => item.id === selectedProjectId.value) || null)

const projectRules = {
  name: [{ required: true, message: '请输入项目名称', trigger: 'blur' }]
}

const storeRules = {
  name: [{ required: true, message: '请输入日志库名称', trigger: 'blur' }]
}

function createProjectForm(row = {}) {
  return {
    id: row.id,
    name: row.name || '',
    code: row.code || '',
    description: row.description || '',
    ownerType: row.ownerType || 'team',
    ownerId: row.ownerId || 0,
    ownerName: row.ownerName || '',
    env: row.env || 'prod',
    status: Number(row.status || 1),
    extraJson: row.extraJson || '',
    createdBy: row.createdBy || 'admin',
    updatedBy: 'admin'
  }
}

function createStoreForm(row = {}) {
  return {
    id: row.id,
    projectId: row.projectId || selectedProjectId.value,
    name: row.name || '',
    code: row.code || '',
    description: row.description || '',
    storeType: row.storeType || 'app',
    retentionDays: Number(row.retentionDays || 7),
    status: Number(row.status || 1),
    sort: Number(row.sort || 1),
    vlogsTenant: row.vlogsTenant || '',
    extraJson: row.extraJson || '',
    createdBy: row.createdBy || 'admin',
    updatedBy: 'admin'
  }
}

const extractPage = payload => {
  const data = payload?.data?.data ?? payload?.data ?? payload
  if (Array.isArray(data)) {
    return {
      list: data,
      total: data.length,
      page: 1,
      pageSize: data.length || 10
    }
  }
  return {
    list: data?.list || data?.items || [],
    total: Number(data?.total || data?.count || 0),
    page: Number(data?.page || 1),
    pageSize: Number(data?.pageSize || data?.page_size || 10)
  }
}

const loadProjects = async ({ keepSelection = false } = {}) => {
  projectLoading.value = true
  try {
    const response = await logtailApi.getProjects({
      page: projectPagination.page,
      pageSize: projectPagination.pageSize,
      keyword: projectQuery.keyword || undefined
    })
    const page = extractPage(response)
    projects.value = page.list
    projectPagination.total = page.total

    if (!keepSelection || !projects.value.some(item => item.id === selectedProjectId.value)) {
      selectedProjectId.value = projects.value[0]?.id || null
    }
    await loadStores()
  } catch (error) {
    console.error('加载项目失败:', error)
    projects.value = []
    stores.value = []
  } finally {
    projectLoading.value = false
  }
}

const loadStores = async () => {
  if (!selectedProjectId.value) {
    stores.value = []
    storePagination.total = 0
    return
  }

  storeLoading.value = true
  try {
    const response = await logtailApi.getStores({
      page: storePagination.page,
      pageSize: storePagination.pageSize,
      projectId: selectedProjectId.value,
      keyword: storeQuery.keyword || undefined,
      status: storeQuery.status || undefined
    })
    const page = extractPage(response)
    stores.value = page.list
    storePagination.total = page.total
  } catch (error) {
    console.error('加载日志库失败:', error)
    stores.value = []
    storePagination.total = 0
  } finally {
    storeLoading.value = false
  }
}

const selectProject = project => {
  selectedProjectId.value = project.id
  storePagination.page = 1
  loadStores()
}

const handleProjectSearch = () => {
  projectPagination.page = 1
  loadProjects()
}

const resetProjectSearch = () => {
  projectQuery.keyword = ''
  projectPagination.page = 1
  loadProjects()
}

const refreshProjects = () => {
  projectPagination.page = 1
  loadProjects({ keepSelection: true })
}

const handleStoreSearch = () => {
  storePagination.page = 1
  loadStores()
}

const resetStoreSearch = () => {
  storeQuery.keyword = ''
  storeQuery.status = ''
  storePagination.page = 1
  loadStores()
}

const handleStoreSizeChange = size => {
  storePagination.pageSize = size
  storePagination.page = 1
  loadStores()
}

const handleStorePageChange = page => {
  storePagination.page = page
  loadStores()
}

const openProjectDialog = row => {
  Object.assign(projectDialog.form, createProjectForm(row || {}))
  projectDialog.visible = true
}

const submitProject = async () => {
  await projectFormRef.value?.validate()
  projectDialog.submitting = true
  try {
    const payload = { ...projectDialog.form }
    if (!payload.id) delete payload.code
    if (payload.id) {
      await logtailApi.updateProject(payload.id, payload)
      ElMessage.success('项目已更新')
    } else {
      await logtailApi.createProject(payload)
      ElMessage.success('项目已创建')
    }
    projectDialog.visible = false
    await loadProjects({ keepSelection: Boolean(payload.id) })
  } finally {
    projectDialog.submitting = false
  }
}

const removeProject = async row => {
  if (!row?.id) return
  await ElMessageBox.confirm(`确认删除项目「${row.name}」吗？`, '删除项目', {
    type: 'warning',
    confirmButtonText: '删除',
    cancelButtonText: '取消'
  })
  await logtailApi.deleteProject(row.id)
  ElMessage.success('项目已删除')
  selectedProjectId.value = null
  await loadProjects()
}

const openStoreDialog = row => {
  Object.assign(storeDialog.form, createStoreForm(row || {}))
  storeDialog.visible = true
}

const submitStore = async () => {
  await storeFormRef.value?.validate()
  storeDialog.submitting = true
  try {
    const payload = {
      ...storeDialog.form,
      projectId: selectedProjectId.value
    }
    if (!payload.id) delete payload.code
    if (payload.id) {
      await logtailApi.updateStore(payload.id, payload)
      ElMessage.success('日志库已更新')
    } else {
      await logtailApi.createStore(payload)
      ElMessage.success('日志库已创建')
    }
    storeDialog.visible = false
    await loadStores()
  } finally {
    storeDialog.submitting = false
  }
}

const toggleStoreStatus = async row => {
  statusUpdatingId.value = row.id
  try {
    const nextStatus = Number(row.status) === 1 ? 2 : 1
    await logtailApi.updateStore(row.id, {
      ...row,
      status: nextStatus,
      updatedBy: 'admin'
    })
    row.status = nextStatus
    ElMessage.success(nextStatus === 1 ? '日志库已启用' : '日志库已禁用')
  } finally {
    statusUpdatingId.value = null
  }
}

const removeStore = async row => {
  await ElMessageBox.confirm(`确认删除日志库「${row.name}」吗？`, '删除日志库', {
    type: 'warning',
    confirmButtonText: '删除',
    cancelButtonText: '取消'
  })
  await logtailApi.deleteStore(row.id)
  ElMessage.success('日志库已删除')
  await loadStores()
}

const goQuery = row => {
  router.push({
    path: '/logs/query',
    query: {
      projectId: selectedProjectId.value,
      storeId: row.id
    }
  })
}

const projectStoreCount = project => Number(project?.storeCount ?? project?.logStoreCount ?? project?.storeTotal ?? project?.storesCount ?? 0)
const projectEnabledStoreCount = project => Number(project?.enabledStoreCount ?? project?.enabledStoresCount ?? project?.enabled_store_count ?? 0)

const formatTime = value => {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false }).replace(/\//g, '-')
}

onMounted(() => {
  loadProjects()
})
</script>

<style scoped>
.log-page {
  --ops-blue-500: #409eff;
  --ops-blue-600: #2563eb;
  --ops-green-500: #10b981;
  --ops-orange-500: #f59e0b;
  --ops-red-500: #ef4444;
  --ops-text-primary: #0f172a;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-bg-surface: #ffffff;
  --ops-bg-surface-soft: #fbfdff;
  --ops-bg-subtle: #f3f7fc;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  --ops-border-inner: #dce8f5;
  --ops-border-hover: #b9d4ff;
  --ops-radius-md: 8px;
  --ops-radius-inner: 10px;
  --ops-radius-card: 12px;
  --ops-radius-dialog: 15px;
  --ops-shadow-card: 0 8px 24px rgba(80, 124, 180, 0.07);
  --ops-shadow-dialog: 0 22px 60px rgba(49, 92, 142, 0.18);
  --log-text-primary: var(--ops-text-primary);
  --log-text-secondary: var(--ops-text-secondary);
  --log-text-muted: var(--ops-text-muted);
  --log-border: var(--ops-border);
  box-sizing: border-box;
  height: calc(100vh - 124px);
  min-height: 0;
  padding: 7px;
  overflow: auto;
  color: var(--ops-text-secondary);
  background: transparent;
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

:global(.home-container .el-main:has(.log-manage-page)) {
  padding: 10px;
  overflow: hidden;
}

.page-card {
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--ops-border);
  border-radius: var(--ops-radius-card);
  background: var(--ops-bg-surface);
  box-shadow: var(--ops-shadow-card);
}

.page-card::before {
  content: none;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex: 0 0 auto;
  padding: 8px 12px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-surface);
}

.page-header-copy,
.ops-page-header__content {
  display: flex;
  align-items: center;
  min-width: 0;
}

.page-header-copy {
  gap: 8px;
  flex: 1;
}

.ops-page-header__icon {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  place-items: center;
}

.ops-page-header__icon img {
  width: 30px;
  height: 30px;
  object-fit: contain;
}

.ops-page-header__content {
  gap: 10px;
  flex: 1;
}

.ops-page-header__divider {
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  background: var(--ops-border-soft);
}

.page-header h1 {
  margin: 0;
  flex: 0 0 auto;
  color: var(--ops-text-primary);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
}

.page-description {
  margin: 0;
  overflow: hidden;
  color: var(--ops-text-muted);
  font-size: 12px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.summary-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 0 0 auto;
  justify-content: flex-end;
}

.summary-chip {
  position: relative;
  display: inline-flex;
  min-width: 98px;
  min-height: 34px;
  align-items: center;
  justify-content: space-between;
  gap: 9px;
  box-sizing: border-box;
  padding: 5px 9px;
  border: 1px solid var(--ops-border-soft);
  border-radius: var(--ops-radius-md);
  background: var(--ops-bg-subtle);
}

.summary-chip::before {
  width: 6px;
  height: 6px;
  flex: 0 0 6px;
  border-radius: 999px;
  background: var(--ops-blue-500);
  content: "";
}

.summary-chip:nth-child(2)::before {
  background: var(--ops-green-500);
}

.summary-chip span {
  color: var(--ops-text-muted);
  font-size: 12px;
}

.summary-chip strong {
  color: var(--ops-text-primary);
  font-size: 16px;
  line-height: 1;
}

.manage-layout {
  display: grid;
  grid-template-columns: 252px minmax(0, 1fr);
  gap: 8px;
  min-height: 0;
  flex: 1;
  padding: 8px 10px 10px;
}

.toolbar-card,
.table-shell {
  border: 1px solid var(--ops-border-inner);
  border-radius: var(--ops-radius-md);
  background: var(--ops-bg-surface);
}

.project-panel,
.store-panel {
  display: flex;
  min-width: 0;
  min-height: 0;
  flex-direction: column;
}

.project-panel {
  padding: 8px;
  border: 1px solid var(--ops-border-inner);
  border-radius: var(--ops-radius-inner);
  background: var(--ops-bg-surface-soft);
}

.store-panel {
  overflow: hidden;
}

.panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  min-height: 30px;
  margin-bottom: 6px;
}

.panel-head h3 {
  margin: 0;
  color: var(--ops-text-primary);
  font-size: 15px;
  font-weight: 700;
  line-height: 30px;
}

.store-head {
  align-items: center;
  padding: 0 2px;
}

.store-head-actions,
.project-head-actions,
.toolbar-actions,
.operation-links {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.project-action-btn {
  width: 28px !important;
  height: 28px !important;
  min-height: 28px !important;
  padding: 0 !important;
  border-radius: var(--ops-radius-md) !important;
  flex: 0 0 28px;
}

.project-head-actions :deep(.project-action-btn .el-icon) {
  width: 14px;
  height: 14px;
  font-size: 14px;
  line-height: 1;
}

.project-action-btn.refresh-action-btn {
  border-color: #f3c66b !important;
  color: #ffffff !important;
  background: #f3c66b !important;
}

.project-action-btn.refresh-action-btn:hover {
  border-color: #e8b953 !important;
  background: #e8b953 !important;
}

.project-head-actions :deep(.el-button + .el-button) {
  margin-left: 0;
}

.compact-form {
  padding-bottom: 8px;
}

.compact-form :deep(.el-form-item) {
  margin-bottom: 0;
}

.project-list {
  min-height: 0;
  flex: 1;
  overflow: auto;
  padding: 1px 0;
}

.project-item {
  width: 100%;
  min-height: 32px;
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 2px;
  padding: 4px 7px;
  border: 1px solid transparent;
  border-radius: 7px;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.18s ease, background 0.18s ease, color 0.18s ease;
}

.project-item:hover {
  border-color: var(--ops-border-hover);
  background: #f3f8ff;
}

.project-item.active {
  border-color: #cfe2ff;
  background: #eaf3ff;
  color: var(--ops-blue-600);
  box-shadow: inset 3px 0 0 var(--ops-blue-500);
}

.project-folder-icon {
  width: 15px;
  height: 15px;
  flex: 0 0 auto;
  display: inline-block;
  object-fit: contain;
}

.project-node-label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.project-node-count {
  min-width: 18px;
  height: 16px;
  flex: 0 0 auto;
  padding: 0 5px;
  border-radius: 999px;
  background: #e2edff;
  color: var(--ops-blue-600);
  font-size: 10px;
  font-weight: 800;
  line-height: 16px;
  text-align: center;
}

.project-status-dot {
  width: 6px;
  height: 6px;
  flex: 0 0 auto;
  border-radius: 999px;
  background: #67c23a;
  box-shadow: 0 0 0 2px rgba(103, 194, 58, 0.12);
}

.project-status-dot.disabled {
  background: #a8b2c3;
  box-shadow: 0 0 0 2px rgba(168, 178, 195, 0.12);
}

.project-item.active .project-folder-icon,
.project-item.active .project-node-label {
  color: var(--ops-blue-600);
}

.project-item.active .project-node-count {
  background: #d8e9ff;
  color: var(--ops-blue-600);
}

.project-item.active .project-status-dot {
  background: #67c23a;
  box-shadow: 0 0 0 2px rgba(103, 194, 58, 0.14);
}

.name-cell strong {
  display: block;
  color: var(--ops-text-primary);
  font-size: 13px;
  line-height: 1.25;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.name-cell span {
  display: block;
  margin-top: 2px;
  color: var(--ops-text-muted);
  font-size: 11px;
  line-height: 1.25;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.toolbar-card {
  flex: 0 0 auto;
  padding: 8px 10px;
  margin-bottom: 8px;
  background: var(--ops-bg-subtle);
}

.toolbar-form {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.toolbar-form :deep(.el-form-item) {
  margin-right: 0;
  margin-bottom: 0;
}

.toolbar-form :deep(.el-form-item__label) {
  height: 30px;
  padding-right: 6px;
  color: var(--ops-text-secondary);
  font-size: 12px;
  font-weight: 700;
  line-height: 30px;
}

.toolbar-form :deep(.el-input) {
  width: 190px;
}

.toolbar-form :deep(.el-select) {
  width: 120px;
}

.table-shell {
  min-height: 0;
  flex: 1;
  overflow: hidden;
}

.data-table {
  height: 100%;
  --el-table-border-color: var(--ops-border-soft);
  --el-table-header-bg-color: var(--ops-bg-subtle);
  --el-table-row-hover-bg-color: #f6faff;
  --el-table-text-color: var(--ops-text-secondary);
  --el-table-header-text-color: #334155;
}

.data-table :deep(.el-table__cell) {
  padding: 5px 0;
  font-size: 12px;
}

.data-table :deep(th.el-table__cell) {
  height: 36px;
  padding: 0;
  font-weight: 700;
}

.data-table :deep(.el-table__body tr) {
  height: 38px;
}

.data-table :deep(.cell) {
  line-height: 1.35;
}

.data-table :deep(.el-tag) {
  border-radius: 999px;
  font-weight: 600;
}

.mono-text {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  color: var(--ops-text-primary);
}

.pagination-section {
  display: flex;
  justify-content: flex-end;
  flex: 0 0 auto;
  min-height: 30px;
  align-items: center;
  padding-top: 8px;
}

.pagination-section :deep(.el-pagination) {
  --el-pagination-button-bg-color: var(--ops-bg-subtle);
  --el-pagination-hover-color: var(--ops-blue-600);
  font-size: 12px;
}

.log-page :deep(.el-input__wrapper),
.log-page :deep(.el-select__wrapper),
.log-page :deep(.el-textarea__inner),
.log-page :deep(.el-input-number .el-input__wrapper) {
  min-height: 30px;
  border-radius: var(--ops-radius-md);
  background: var(--ops-bg-surface);
  box-shadow: inset 0 0 0 1px var(--ops-border-inner) !important;
}

.log-page :deep(.el-input__wrapper.is-focus),
.log-page :deep(.el-select__wrapper.is-focused),
.log-page :deep(.el-textarea__inner:focus) {
  box-shadow: inset 0 0 0 1px var(--ops-blue-500), 0 0 0 3px rgba(64, 158, 255, 0.1) !important;
}

.log-page :deep(.el-button) {
  border-radius: var(--ops-radius-md) !important;
  font-weight: 600;
  transform: none !important;
  transition: color 160ms ease, border-color 160ms ease, background-color 160ms ease, box-shadow 160ms ease;
}

.store-head-actions :deep(.el-button),
.toolbar-actions :deep(.el-button) {
  height: 28px;
  padding: 0 10px;
  font-size: 12px;
}

.log-page :deep(.el-button--primary:not(.is-link)) {
  border-color: #409eff;
  color: #ffffff;
  background: #409eff;
}

.log-page :deep(.el-button--primary:not(.is-link):hover) {
  border-color: #337ecc;
  background: #337ecc;
}

.log-page :deep(.el-button--success:not(.is-link)) {
  border-color: #67c23a;
  color: #ffffff;
  background: #67c23a;
}

.log-page :deep(.el-button--success:not(.is-link):hover) {
  border-color: #58b832;
  background: #58b832;
}

.log-page :deep(.el-button--warning:not(.is-link)) {
  border-color: #e6a23c;
  color: #ffffff;
  background: #e6a23c;
}

.log-page :deep(.el-button--warning:not(.is-link):hover) {
  border-color: #d8922f;
  background: #d8922f;
}

.log-page :deep(.el-button--danger:not(.is-link)) {
  border-color: #f56c6c;
  color: #ffffff;
  background: #f56c6c;
}

.log-page :deep(.el-button--danger:not(.is-link):hover) {
  border-color: #e45656;
  background: #e45656;
}

.operation-links :deep(.el-button) {
  margin-left: 0;
  padding: 0;
}

:global(.log-project-dialog-overlay) {
  --ops-text-primary: #0f172a;
  --ops-text-secondary: #53657d;
  --ops-bg-surface: #ffffff;
  --ops-bg-subtle: #f3f7fc;
  --ops-border-soft: #e7eef7;
  --ops-border-inner: #dce8f5;
  --ops-blue-500: #409eff;
  --ops-radius-md: 8px;
  --ops-shadow-dialog: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.log-project-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 32px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: var(--ops-bg-surface);
  box-shadow: var(--ops-shadow-dialog);
}

:global(.log-project-dialog-overlay .el-dialog__header) {
  padding: 16px 18px 12px;
  margin: 0;
  border-bottom: 1px solid #e3edf8;
}

:global(.log-project-dialog-overlay .el-dialog__title) {
  color: var(--ops-text-primary);
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.log-project-dialog-overlay .el-dialog__headerbtn) {
  top: 10px;
  right: 12px;
  width: 34px;
  height: 34px;
  border-radius: var(--ops-radius-md);
}

:global(.log-project-dialog-overlay .el-dialog__headerbtn:hover) {
  background: var(--ops-bg-subtle);
}

:global(.log-project-dialog-overlay .el-dialog__body) {
  max-height: calc(100vh - 190px);
  padding: 16px 18px;
  overflow: auto;
}

:global(.log-project-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
}

:global(.log-project-dialog-overlay .el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

:global(.log-project-dialog-overlay .el-select),
:global(.log-project-dialog-overlay .el-input-number) {
  width: 100%;
}

:global(.log-project-dialog-overlay .el-input__wrapper),
:global(.log-project-dialog-overlay .el-select__wrapper),
:global(.log-project-dialog-overlay .el-textarea__inner),
:global(.log-project-dialog-overlay .el-input-number .el-input__wrapper) {
  border-radius: var(--ops-radius-md);
  background: var(--ops-bg-surface);
  box-shadow: inset 0 0 0 1px var(--ops-border-inner) !important;
}

:global(.log-project-dialog-overlay .el-input__wrapper.is-focus),
:global(.log-project-dialog-overlay .el-select__wrapper.is-focused),
:global(.log-project-dialog-overlay .el-textarea__inner:focus) {
  box-shadow: inset 0 0 0 1px var(--ops-blue-500), 0 0 0 3px rgba(64, 158, 255, 0.1) !important;
}

:global(.log-project-dialog-overlay .el-button) {
  border-radius: var(--ops-radius-md);
  font-weight: 600;
}

:global(.log-project-dialog-overlay .el-button--primary) {
  border-color: #409eff;
  color: #ffffff;
  background: #409eff;
}

@media (max-width: 1180px) {
  .page-card {
    height: auto;
    min-height: 100%;
    overflow: visible;
  }

  .manage-layout {
    grid-template-columns: 1fr;
  }

  .project-panel {
    max-height: 280px;
  }

  .store-panel {
    min-height: 520px;
  }
}

@media (max-width: 768px) {
  .page-header {
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .page-header-copy,
  .summary-group {
    width: 100%;
  }

  .summary-group {
    justify-content: flex-start;
  }

  .ops-page-header__content {
    align-items: flex-start;
    flex-direction: column;
    gap: 2px;
  }

  .ops-page-header__divider {
    display: none;
  }

  .page-description {
    width: 100%;
    white-space: normal;
  }

  .store-head {
    flex-direction: column;
    align-items: stretch;
  }

  .store-head-actions,
  .toolbar-actions {
    width: 100%;
  }

  .store-head-actions :deep(.el-button),
  .toolbar-actions :deep(.el-button) {
    flex: 1;
  }
}

@media (max-width: 620px) {
  .log-page {
    padding: 6px;
  }

  .page-header {
    padding: 8px 10px;
  }

  .ops-page-header__icon {
    width: 36px;
    height: 36px;
    flex-basis: 36px;
  }

  .ops-page-header__icon img {
    width: 27px;
    height: 27px;
  }

  .summary-group {
    flex-wrap: wrap;
  }

  .summary-chip {
    flex: 1 1 112px;
  }

  .manage-layout {
    padding: 8px;
  }

  .toolbar-form :deep(.el-form-item),
  .toolbar-form :deep(.el-input),
  .toolbar-form :deep(.el-select) {
    width: 100%;
  }

  .toolbar-actions {
    display: grid;
    width: 100%;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .toolbar-actions :deep(.el-button) {
    width: 100%;
    margin-left: 0;
  }

  .store-head-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .store-head-actions :deep(.el-button) {
    width: 100%;
    margin-left: 0;
  }

  .store-head-actions :deep(.el-button:last-child) {
    grid-column: 1 / -1;
  }

  .pagination-section {
    justify-content: flex-start;
    overflow-x: auto;
  }

  :global(.log-project-dialog-overlay .el-dialog) {
    width: calc(100% - 24px) !important;
    max-width: none;
  }

  :global(.log-project-dialog-overlay .el-dialog__body) {
    padding: 14px 12px;
  }
}
</style>
