<template>
  <div class="alert-center-page alert-source-page">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/数据采集.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>告警数据源</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>统一维护告警查询数据源、连接认证和扩展配置。</p>
      </div>
      <div class="source-header-summary" aria-label="告警数据源统计">
        <span><em>数据源</em><strong>{{ total }}</strong></span>
        <i aria-hidden="true"></i>
        <span><em>启用</em><strong>{{ enabledCount }}</strong></span>
        <i aria-hidden="true"></i>
        <span><em>内置</em><strong>{{ builtinCount }}</strong></span>
      </div>
    </header>

    <el-card shadow="never" class="alert-center-card">
      <section class="alert-search-section">
        <el-form :model="query" :inline="true" class="alert-search-form" @submit.prevent>
          <el-form-item label="关键字">
            <el-input v-model="query.keyword" class="alert-filter-input" clearable placeholder="名称 / 地址" @keyup.enter="handleSearch" />
          </el-form-item>
          <el-form-item label="类型">
            <el-select v-model="query.type" class="alert-filter-small" clearable filterable placeholder="全部">
              <el-option v-for="item in datasourceTypeItems" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="query.enabled" class="alert-filter-small" clearable placeholder="全部">
              <el-option v-for="item in enabledOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" size="small" class="alert-search-button source-toolbar-button" @click="handleSearch">
              <el-icon><Search /></el-icon>
              <span>查询</span>
            </el-button>
            <el-button type="warning" size="small" class="alert-reset-button source-toolbar-button" @click="handleReset">
              <el-icon><RefreshRight /></el-icon>
              <span>重置</span>
            </el-button>
            <el-button type="success" size="small" class="alert-create-button source-toolbar-button" @click="openCreateDialog">
              <el-icon><Plus /></el-icon>
              <span>新增数据源</span>
            </el-button>
          </el-form-item>
        </el-form>
      </section>

      <section class="alert-summary-grid source-summary-grid">
        <article class="alert-summary-card">
          <div>
            <strong>{{ total }}</strong>
            <span>全部数据源</span>
          </div>
        </article>
        <article class="alert-summary-card">
          <div>
            <strong>{{ enabledCount }}</strong>
            <span>已启用</span>
          </div>
        </article>
        <article class="alert-summary-card">
          <div>
            <strong>{{ prometheusCount }}</strong>
            <span>Prometheus 兼容</span>
          </div>
        </article>
        <article class="alert-summary-card">
          <div>
            <strong>{{ builtinCount }}</strong>
            <span>平台内置</span>
          </div>
        </article>
      </section>

      <div class="alert-section-header">
        <span>数据源列表</span>
        <span class="alert-muted-text">规则页通过 options 接口引用这里维护的数据源</span>
      </div>

      <div class="alert-table-shell">
        <el-table class="alert-table" :data="rows" v-loading="loading" empty-text="暂无告警数据源">
          <el-table-column label="数据源名称" min-width="220">
            <template #default="{ row }">
              <div class="alert-main-text">{{ row.name || '-' }}</div>
              <div class="alert-muted-text">{{ row.description || row.remark || '用于告警规则查询与事件归属' }}</div>
            </template>
          </el-table-column>
          <el-table-column label="类型" width="150">
            <template #default="{ row }">
              <el-tag effect="light">{{ datasourceTypeLabel(getDatasourceType(row)) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="连接地址" min-width="260" show-overflow-tooltip>
            <template #default="{ row }">
              <span class="alert-code">{{ datasourceEndpoint(row) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="认证方式" width="120">
            <template #default="{ row }">{{ authTypeLabel(getAuthType(row)) }}</template>
          </el-table-column>
          <el-table-column label="来源" width="100">
            <template #default="{ row }">
              <el-tag :type="isBuiltinDatasource(row) ? 'primary' : 'success'" effect="light">
                {{ isBuiltinDatasource(row) ? '内置' : '自定义' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="isEnabled(row) ? 'success' : 'info'" effect="light">{{ isEnabled(row) ? '启用' : '禁用' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="更新时间" width="170">
            <template #default="{ row }">{{ formatDateTime(row.updatedAt || row.updated_at || row.createdAt || row.created_at) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="220" fixed="right">
            <template #default="{ row }">
              <div class="alert-operation-links">
                <button type="button" class="alert-operation-link alert-operation-warning" @click="openDetailDrawer(row)">查看</button>
                <button type="button" class="alert-operation-link" @click="openEditDialog(row)">编辑</button>
                <button type="button" class="alert-operation-link alert-operation-success" @click="toggleStatus(row)">
                  {{ isEnabled(row) ? '禁用' : '启用' }}
                </button>
                <button
                  v-if="!isBuiltinDatasource(row)"
                  type="button"
                  class="alert-operation-link alert-operation-danger"
                  @click="handleDelete(row)"
                >
                  删除
                </button>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="alert-pagination">
        <el-pagination
          v-model:current-page="query.page"
          v-model:page-size="query.pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="loadList"
          @current-change="loadList"
        />
      </div>
    </el-card>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="860px"
      class="alert-dialog source-dialog"
      modal-class="alert-source-dialog-overlay"
      destroy-on-close
    >
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="132px">
        <section class="source-form-section">
          <h3>基础信息</h3>
          <div class="alert-form-grid">
            <el-form-item label="数据源名称" prop="name">
              <el-input v-model="form.name" placeholder="例如：生产 Prometheus" />
            </el-form-item>
            <el-form-item label="数据源类型" prop="type">
              <el-select
                v-model="form.type"
                class="full-select"
                clearable
                filterable
                placeholder="请选择数据源类型"
                :loading="typeLoading"
                @visible-change="handleTypeDropdownVisible"
                @focus="loadTypes"
              >
                <el-option
                  v-for="item in datasourceTypeItems"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value"
                  :disabled="item.disabled || (item.value === 'builtin' && !editingId)"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="连接地址" prop="endpoint" class="alert-form-full">
              <el-input v-model="form.endpoint" placeholder="http://prometheus:9090 或 http://victoriametrics:8428" />
            </el-form-item>
            <el-form-item label="启用状态">
              <el-switch v-model="form.enabled" :active-value="1" :inactive-value="0" active-text="启用" inactive-text="禁用" />
            </el-form-item>
            <el-form-item label="排序">
              <el-input-number v-model="form.sort" :min="0" controls-position="right" class="full-input-number" />
            </el-form-item>
            <el-form-item label="描述" class="alert-form-full">
              <el-input v-model="form.description" type="textarea" :rows="3" placeholder="用途、环境、归属团队等说明" />
            </el-form-item>
          </div>
        </section>

        <section class="source-form-section">
          <h3>连接与认证</h3>
          <div class="alert-form-grid">
            <el-form-item label="认证方式">
              <el-select v-model="form.authType" class="full-select">
                <el-option v-for="item in authTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
            <el-form-item label="超时时间">
              <el-input-number v-model="form.timeoutSeconds" :min="1" :max="300" controls-position="right" class="full-input-number" />
            </el-form-item>
            <el-form-item v-if="form.authType === 'basic'" label="用户名">
              <el-input v-model="form.username" autocomplete="off" placeholder="username" />
            </el-form-item>
            <el-form-item v-if="form.authType === 'basic'" label="密码">
              <el-input v-model="form.password" autocomplete="new-password" show-password placeholder="留空表示不修改" />
            </el-form-item>
            <el-form-item v-if="form.authType === 'bearer' || form.authType === 'token'" label="Token" class="alert-form-full">
              <el-input v-model="form.token" autocomplete="new-password" show-password placeholder="留空表示不修改" />
            </el-form-item>
            <el-form-item label="Headers JSON" class="alert-form-full">
              <el-input v-model="form.headersText" class="alert-json-textarea" type="textarea" :rows="4" placeholder='{"X-Scope-OrgID":"tenant-a"}' />
            </el-form-item>
          </div>
        </section>

        <el-collapse class="source-advanced-collapse">
          <el-collapse-item title="高级扩展配置" name="advanced">
            <div class="alert-form-grid">
              <el-form-item label="Config JSON" class="alert-form-full">
                <el-input v-model="form.configText" class="alert-json-textarea" type="textarea" :rows="6" placeholder='{"queryPath":"/api/v1/query","headers":{}}' />
              </el-form-item>
              <el-form-item label="Tags JSON" class="alert-form-full">
                <el-input v-model="form.tagsText" class="alert-json-textarea" type="textarea" :rows="3" placeholder='{"env":"prod","team":"sre"}' />
              </el-form-item>
            </div>
          </el-collapse-item>
        </el-collapse>
      </el-form>

      <template #footer>
        <div class="alert-dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="saving" @click="submitForm">确认</el-button>
        </div>
      </template>
    </el-dialog>

    <el-drawer
      v-model="detailVisible"
      title="数据源详情"
      size="560px"
      class="alert-detail-drawer"
      modal-class="alert-source-detail-overlay"
    >
      <div v-if="currentRow" class="alert-detail-stack">
        <section class="alert-detail-panel">
          <h4>{{ currentRow.name }}</h4>
          <div class="alert-detail-row"><span>类型</span><span>{{ datasourceTypeLabel(getDatasourceType(currentRow)) }}</span></div>
          <div class="alert-detail-row"><span>连接地址</span><span>{{ datasourceEndpoint(currentRow) }}</span></div>
          <div class="alert-detail-row"><span>认证方式</span><span>{{ authTypeLabel(getAuthType(currentRow)) }}</span></div>
          <div class="alert-detail-row"><span>来源</span><span>{{ isBuiltinDatasource(currentRow) ? '内置' : '自定义' }}</span></div>
          <div class="alert-detail-row"><span>状态</span><span>{{ isEnabled(currentRow) ? '启用' : '禁用' }}</span></div>
        </section>
        <section class="alert-detail-panel">
          <h4>扩展配置</h4>
          <pre class="alert-json-preview">{{ stringifyJson(datasourceConfig(currentRow)) || '-' }}</pre>
        </section>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, RefreshRight, Search } from '@element-plus/icons-vue'
import alertApi from '@/api/alertmanager'
import {
  compactObject,
  enabledOptions,
  formatDateTime,
  isSuccess,
  normalizeListResult,
  parseJsonField,
  parseSafeJson,
  stringifyJson,
  unwrapData
} from './alertCommon'
import './alert-pages.css'

const baseDatasourceTypes = [
  { label: '平台内置', value: 'builtin' },
  { label: 'Prometheus', value: 'prometheus' },
  { label: 'VictoriaMetrics', value: 'victoriametrics' },
  { label: 'Loki', value: 'loki' },
  { label: 'Zabbix', value: 'zabbix' },
  { label: 'Elasticsearch', value: 'elasticsearch' },
  { label: 'ClickHouse', value: 'clickhouse' },
  { label: '自定义', value: 'custom' }
]

const authTypeOptions = [
  { label: '无认证', value: 'none' },
  { label: 'Basic Auth', value: 'basic' },
  { label: 'Bearer Token', value: 'bearer' },
  { label: 'Token', value: 'token' }
]

const loading = ref(false)
const saving = ref(false)
const typeLoading = ref(false)
const dialogVisible = ref(false)
const detailVisible = ref(false)
const editingId = ref(null)
const rows = ref([])
const typeRows = ref([])
const total = ref(0)
const formRef = ref()
const currentRow = ref(null)
const dialogTitle = ref('新增数据源')
const query = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  type: '',
  enabled: ''
})
const form = reactive({})

const formRules = {
  name: [{ required: true, message: '请输入数据源名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择或输入数据源类型', trigger: 'change' }],
  endpoint: [{ required: true, message: '请输入连接地址', trigger: 'blur' }]
}

const datasourceTypeItems = computed(() => {
  const map = new Map()
  const appendType = item => {
    const rawValue = typeof item === 'string' ? item : (item?.value || item?.type || item?.datasourceType || item?.code)
    const rawLabel = typeof item === 'string' ? item : (item?.label || item?.name || item?.description)
    const disabled = typeof item === 'object' && (item.disabled === true || item.enabled === false)
    const description = typeof item === 'object' ? item.description : ''
    const builtin = typeof item === 'object' ? item.builtin : undefined
    const sort = typeof item === 'object' ? item.sort : undefined
    const type = String(rawValue || '').trim()
    if (!type || map.has(type)) return
    map.set(type, {
      label: rawLabel || type,
      value: type,
      disabled,
      description,
      builtin,
      sort
    })
  }
  const sourceTypes = typeRows.value.length ? typeRows.value : baseDatasourceTypes
  sourceTypes.forEach(item => appendType(item))
  rows.value.forEach(row => appendType(getDatasourceType(row)))
  return Array.from(map.values()).sort((a, b) => Number(a.sort ?? 100) - Number(b.sort ?? 100))
})

const enabledCount = computed(() => rows.value.filter(row => isEnabled(row)).length)
const builtinCount = computed(() => rows.value.filter(row => isBuiltinDatasource(row)).length)
const prometheusCount = computed(() => rows.value.filter(row => {
  const type = getDatasourceType(row)
  return type === 'prometheus' || type === 'victoriametrics' || type === 'builtin'
}).length)

const loadList = async () => {
  loading.value = true
  try {
    const response = await alertApi.getDatasources(compactObject({
      page: query.page,
      pageSize: query.pageSize,
      keyword: query.keyword,
      type: query.type,
      datasourceType: query.type,
      enabled: query.enabled
    }))
    const result = normalizeListResult(response)
    rows.value = result.list
    total.value = result.total
  } finally {
    loading.value = false
  }
}

const loadTypes = async () => {
  if (typeLoading.value) return
  typeLoading.value = true
  try {
    const response = await alertApi.getDatasourceTypes()
    typeRows.value = normalizeDatasourceTypes(response)
  } catch (error) {
    typeRows.value = []
  } finally {
    typeLoading.value = false
  }
}

const handleTypeDropdownVisible = visible => {
  if (visible) loadTypes()
}

const handleSearch = () => {
  query.page = 1
  loadList()
}

const handleReset = () => {
  Object.assign(query, {
    page: 1,
    pageSize: 10,
    keyword: '',
    type: '',
    enabled: ''
  })
  loadList()
}

const resetForm = row => {
  Object.keys(form).forEach(key => delete form[key])
  const config = datasourceConfig(row)
  Object.assign(form, {
    name: '',
    type: 'prometheus',
    endpoint: '',
    description: '',
    authType: 'none',
    username: '',
    password: '',
    token: '',
    enabled: 1,
    sort: 100,
    timeoutSeconds: 30,
    headersText: '{}',
    configText: '{}',
    tagsText: '{}',
    ...row,
    type: getDatasourceType(row) || 'prometheus',
    endpoint: datasourceEndpoint(row, ''),
    description: row?.description || row?.remark || '',
    authType: getAuthType(row),
    username: row?.username || config.username || '',
    password: '',
    token: '',
    enabled: row ? (isEnabled(row) ? 1 : 0) : 1,
    sort: row?.sort ?? config.sort ?? 100,
    timeoutSeconds: row?.timeoutSeconds ?? row?.timeout_seconds ?? config.timeoutSeconds ?? config.timeout_seconds ?? 30,
    headersText: stringifyJson(row?.headers || config.headers) || '{}',
    configText: stringifyJson(config) || '{}',
    tagsText: stringifyJson(row?.tags || config.tags) || '{}'
  })
}

const openCreateDialog = async () => {
  await loadTypes()
  editingId.value = null
  dialogTitle.value = '新增数据源'
  resetForm()
  dialogVisible.value = true
}

const openEditDialog = async row => {
  await loadTypes()
  editingId.value = row.id
  dialogTitle.value = '编辑数据源'
  try {
    const response = await alertApi.getDatasource(row.id)
    resetForm(unwrapData(response) || row)
  } catch (error) {
    resetForm(row)
  }
  dialogVisible.value = true
}

const openDetailDrawer = row => {
  currentRow.value = row
  detailVisible.value = true
}

const buildPayload = () => {
  const config = parseJsonField(form.configText, {})
  const headers = parseJsonField(form.headersText, {})
  const tags = parseJsonField(form.tagsText, {})
  const payload = {
    name: form.name,
    type: form.type,
    datasourceType: form.type,
    endpoint: form.endpoint,
    url: form.endpoint,
    description: form.description,
    authType: form.authType,
    username: form.authType === 'basic' ? form.username : '',
    enabled: form.enabled,
    sort: form.sort,
    timeoutSeconds: form.timeoutSeconds,
    headers,
    tags,
    config: {
      ...config,
      endpoint: form.endpoint,
      url: form.endpoint,
      authType: form.authType,
      timeoutSeconds: form.timeoutSeconds,
      headers,
      tags
    }
  }
  if (form.password) payload.password = form.password
  if (form.token) payload.token = form.token
  return compactObject(payload)
}

const submitForm = async () => {
  await formRef.value?.validate()
  saving.value = true
  try {
    const response = editingId.value
      ? await alertApi.updateDatasource(editingId.value, buildPayload())
      : await alertApi.createDatasource(buildPayload())
    if (isSuccess(response)) {
      ElMessage.success(editingId.value ? '修改成功' : '创建成功')
      dialogVisible.value = false
      loadList()
      loadTypes()
    }
  } catch (error) {
    if (error?.message === 'JSON 格式不正确') ElMessage.error(error.message)
  } finally {
    saving.value = false
  }
}

const toggleStatus = async row => {
  const nextEnabled = isEnabled(row) ? 0 : 1
  const response = await alertApi.updateDatasourceStatus(row.id, nextEnabled)
  if (isSuccess(response)) {
    row.enabled = nextEnabled
    row.status = nextEnabled === 1 ? 'active' : 'inactive'
    ElMessage.success(nextEnabled === 1 ? '数据源已启用' : '数据源已禁用')
  }
}

const handleDelete = async row => {
  if (isBuiltinDatasource(row)) {
    ElMessage.warning('内置数据源不允许删除')
    return
  }
  await ElMessageBox.confirm(`确认删除数据源「${row.name || row.id}」吗？已被告警规则引用的数据源将由后端拒绝删除。`, '删除确认', {
    type: 'warning',
    confirmButtonText: '确认删除',
    cancelButtonText: '取消'
  })
  const response = await alertApi.deleteDatasource(row.id)
  if (isSuccess(response)) {
    ElMessage.success('删除成功')
    loadList()
    loadTypes()
  }
}

const datasourceConfig = row => {
  const value = row?.config || row?.properties || row?.settings || row?.extra || {}
  const parsed = parseSafeJson(value)
  return parsed && typeof parsed === 'object' ? parsed : {}
}

const normalizeDatasourceTypes = response => {
  const payload = unwrapData(response)
  if (Array.isArray(payload)) return payload
  if (Array.isArray(payload?.types)) return payload.types
  if (Array.isArray(payload?.list)) return payload.list
  if (Array.isArray(payload?.options)) return payload.options
  if (payload && typeof payload === 'object') {
    return Object.entries(payload).map(([value, item]) => {
      if (item && typeof item === 'object') {
        return { value, ...item }
      }
      return { value, label: String(item || value) }
    })
  }
  return normalizeListResult(response).list
}

const getDatasourceType = row => String(row?.type || row?.datasourceType || row?.datasource_type || '').trim().toLowerCase()

const datasourceEndpoint = (row, fallback = '-') => {
  const config = datasourceConfig(row)
  return row?.endpoint || row?.url || row?.baseUrl || row?.base_url || row?.address || config.endpoint || config.url || config.baseUrl || fallback
}

const getAuthType = row => {
  const config = datasourceConfig(row)
  return String(row?.authType || row?.auth_type || config.authType || config.auth_type || (row?.username || config.username ? 'basic' : 'none')).toLowerCase()
}

const isEnabled = row => {
  if (row?.enabled !== undefined && row?.enabled !== null) return Number(row.enabled) === 1 || row.enabled === true
  if (row?.status) return row.status === 'active' || row.status === 'enabled'
  return false
}

const isBuiltinDatasource = row => {
  const source = String(row?.source || row?.templateSource || '').toLowerCase()
  return source === 'builtin' || source === 'platform' || getDatasourceType(row) === 'builtin' || Number(row?.builtin) === 1 || row?.isBuiltin === true
}

const datasourceTypeLabel = value => {
  const target = datasourceTypeItems.value.find(item => item.value === value)
  return target?.label || value || '-'
}

const authTypeLabel = value => {
  const target = authTypeOptions.find(item => item.value === value)
  return target?.label || value || '无认证'
}

onMounted(async () => {
  await Promise.all([loadList(), loadTypes()])
})
</script>

<style scoped>
:global(.home-container .el-main:has(.alert-source-page)) {
  --ops-shell-gutter: 10px;
  --ops-page-padding: 7px;
  --ops-module-gap: 8px;
  padding-top: var(--ops-shell-gutter);
  padding-right: var(--ops-shell-gutter);
  padding-left: var(--ops-shell-gutter);
  overflow-y: hidden;
}

.alert-source-page {
  --ops-text-heading: #0f172a;
  --ops-text-primary: #172033;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-bg-surface: #ffffff;
  --ops-bg-surface-soft: #fbfdff;
  --ops-bg-subtle: #f3f7fc;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  box-sizing: border-box;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: var(--ops-module-gap, 8px);
  height: calc(100vh - 124px);
  max-height: none;
  padding: var(--ops-page-padding, 7px);
  overflow: hidden;
  border-radius: 0;
  background: transparent;
  color: var(--ops-text-primary);
}

.ops-page-header {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 12px;
  min-width: 0;
  padding: 8px 12px;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 10px 28px rgba(80, 124, 180, 0.1);
}

.ops-page-header__icon {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  place-items: center;
  border: 0;
  background: transparent;
}

.ops-page-header__icon img {
  width: 30px;
  height: 30px;
  object-fit: contain;
}

.ops-page-header__content {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.ops-page-header h1 {
  flex: 0 0 auto;
  margin: 0;
  color: var(--ops-text-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: 0;
  white-space: nowrap;
}

.ops-page-header__divider {
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  background: var(--ops-border-soft);
}

.ops-page-header__content p {
  margin: 0;
  overflow: hidden;
  color: var(--ops-text-muted);
  font-size: 12px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.source-header-summary {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 10px;
  color: var(--ops-text-secondary);
}

.source-header-summary span {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  white-space: nowrap;
}

.source-header-summary em {
  color: var(--ops-text-muted);
  font-size: 12px;
  font-style: normal;
}

.source-header-summary strong {
  color: var(--ops-text-heading);
  font-family: Consolas, Monaco, monospace;
  font-size: 16px;
  line-height: 1;
}

.source-header-summary > i {
  width: 1px;
  height: 18px;
  background: var(--ops-border-soft);
}

.alert-source-page :deep(.alert-center-card.el-card) {
  position: relative;
  display: flex;
  min-width: 0;
  min-height: 0;
  height: 100%;
  flex-direction: column;
  overflow: hidden !important;
  border: 1px solid var(--ops-border);
  border-radius: 12px !important;
  background: var(--ops-bg-surface);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
  backdrop-filter: none;
}

.alert-source-page :deep(.alert-center-card::before) {
  display: none;
}

.alert-source-page :deep(.alert-center-card .el-card__body) {
  box-sizing: border-box;
  display: flex;
  min-height: 0;
  height: 100%;
  flex: 1;
  flex-direction: column;
  padding: 8px 10px 10px;
  overflow: hidden;
  border-radius: 0;
}

.alert-source-page .alert-search-section {
  margin-bottom: 8px;
  padding: 8px 10px 2px;
  overflow: hidden;
  border: 1px solid var(--ops-border-soft);
  border-radius: 10px;
  background: var(--ops-bg-subtle);
}

.alert-source-page :deep(.alert-search-form .el-form-item) {
  margin-right: 10px;
  margin-bottom: 6px;
}

.alert-source-page :deep(.alert-search-form .el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

.alert-source-page :deep(.alert-search-form .el-input__wrapper),
.alert-source-page :deep(.alert-search-form .el-select__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 7px;
  background: #ffffff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.alert-source-page :deep(.alert-search-form .el-input__wrapper:hover),
.alert-source-page :deep(.alert-search-form .el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.alert-source-page :deep(.alert-search-form .el-input__wrapper.is-focus),
.alert-source-page :deep(.alert-search-form .el-select__wrapper.is-focused) {
  box-shadow: 0 0 0 1px #409eff inset;
}

.alert-source-page :deep(.source-toolbar-button.el-button) {
  height: 30px;
  min-height: 30px;
  padding: 0 10px;
  border-radius: 8px;
  color: #ffffff;
  font-size: 12px;
  box-shadow: none;
}

.alert-source-page :deep(.alert-search-button.el-button) {
  border-color: #409eff;
  background: #409eff;
}

.alert-source-page :deep(.alert-reset-button.el-button) {
  border-color: #e6a23c;
  background: #e6a23c;
}

.alert-source-page :deep(.alert-create-button.el-button) {
  border-color: #67c23a;
  background: #67c23a;
}

.alert-source-page :deep(.source-toolbar-button .el-icon) {
  font-size: 14px;
}

.source-summary-grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
  margin-bottom: 8px;
}

.alert-source-page .alert-summary-card {
  min-width: 0;
  padding: 9px 12px;
  border: 1px solid var(--ops-border-soft);
  border-radius: 10px;
  background: var(--ops-bg-surface-soft);
}

.alert-source-page .alert-summary-card strong {
  font-size: 20px;
  line-height: 1.05;
}

.alert-source-page .alert-summary-card span {
  margin-top: 2px;
  color: var(--ops-text-secondary);
  font-size: 12px;
}

.alert-source-page .alert-section-header {
  padding: 0 2px 6px;
  color: var(--ops-text-secondary);
  font-size: 12px;
}

.alert-source-page .alert-table-shell,
.alert-source-page :deep(.alert-table-shell .el-table),
.alert-source-page :deep(.alert-table-shell .el-table__inner-wrapper) {
  border-radius: 8px;
}

.alert-source-page .alert-table-shell {
  border-color: var(--ops-border-soft);
  background: #ffffff;
}

.alert-source-page :deep(.alert-table th.el-table__cell) {
  height: 40px;
  padding: 6px 0;
  background: var(--ops-bg-subtle);
}

.alert-source-page :deep(.alert-table td.el-table__cell) {
  height: 42px;
  padding: 6px 0;
}

.alert-source-page .alert-pagination {
  padding-top: 6px;
}

.alert-source-page :deep(.el-button:hover),
.alert-source-page :deep(.el-button:active),
.alert-source-page :deep(.alert-operation-link:hover),
.alert-source-page :deep(.alert-operation-link:active) {
  transform: none;
}

.full-select,
.full-input-number {
  width: 100%;
}

.source-form-section {
  margin-bottom: 10px;
  padding: 12px 12px 2px;
  border: 1px solid #e7eef7;
  border-radius: 10px;
  background: #fbfdff;
}

.source-form-section h3 {
  margin: 0 0 10px;
  color: #172033;
  font-size: 14px;
}

.source-advanced-collapse {
  border-top: none;
  border-bottom: none;
}

.source-advanced-collapse :deep(.el-collapse-item__header) {
  height: 40px;
  padding: 0 12px;
  border: 1px solid #e7eef7;
  border-radius: 10px;
  background: #fbfdff;
  color: #172033;
  font-weight: 700;
}

.source-advanced-collapse :deep(.el-collapse-item__wrap) {
  border-bottom: none;
  background: transparent;
}

.source-advanced-collapse :deep(.el-collapse-item__content) {
  padding: 12px 0 0;
}

:global(.alert-source-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 32px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: #ffffff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.alert-source-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.alert-source-dialog-overlay .el-dialog__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.alert-source-dialog-overlay .el-dialog__body) {
  box-sizing: border-box;
  max-height: 68vh;
  padding: 14px 18px;
  overflow: auto;
  color: #53657d;
}

:global(.alert-source-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #ffffff;
}

:global(.alert-source-dialog-overlay .el-dialog__footer .el-button) {
  min-height: 34px;
  border-radius: 8px;
}

:global(.alert-source-dialog-overlay .el-form-item) {
  margin-bottom: 16px;
}

:global(.alert-source-dialog-overlay .el-form-item__label) {
  color: #53657d;
  font-size: 13px;
  font-weight: 600;
}

:global(.alert-source-dialog-overlay .el-input__wrapper),
:global(.alert-source-dialog-overlay .el-select__wrapper),
:global(.alert-source-dialog-overlay .el-input-number .el-input__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: #ffffff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.alert-source-dialog-overlay .el-textarea__inner) {
  border: 0;
  border-radius: 8px;
  background: #ffffff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.alert-source-dialog-overlay .el-input__wrapper:hover),
:global(.alert-source-dialog-overlay .el-select__wrapper:hover),
:global(.alert-source-dialog-overlay .el-textarea__inner:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

:global(.alert-source-dialog-overlay .el-input__wrapper.is-focus),
:global(.alert-source-dialog-overlay .el-select__wrapper.is-focused),
:global(.alert-source-dialog-overlay .el-textarea__inner:focus) {
  box-shadow: 0 0 0 1px #409eff inset;
}

:global(.alert-source-dialog-overlay .el-button:hover),
:global(.alert-source-dialog-overlay .el-button:active) {
  transform: none;
}

:global(.alert-source-detail-overlay .el-drawer) {
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: 15px 0 0 15px !important;
  background: #ffffff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.alert-source-detail-overlay .el-drawer__header) {
  min-height: 54px;
  margin: 0;
  padding: 14px 16px 12px;
  border-bottom: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.alert-source-detail-overlay .el-drawer__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.alert-source-detail-overlay .el-drawer__body) {
  padding: 14px 16px;
  background: #ffffff;
}

:global(.alert-source-detail-overlay .alert-detail-stack) {
  gap: 8px;
}

:global(.alert-source-detail-overlay .alert-detail-panel) {
  padding: 12px;
  border: 1px solid #e7eef7;
  border-radius: 10px;
  background: #fbfdff;
}

:global(.alert-source-detail-overlay .alert-json-preview) {
  border-radius: 8px;
}

@media (max-width: 1200px) {
  .ops-page-header {
    flex-wrap: wrap;
  }

  .source-header-summary {
    width: 100%;
    padding-left: 54px;
  }
}

@media (max-width: 768px) {
  :global(.home-container .el-main:has(.alert-source-page)) {
    padding-right: 6px;
    padding-left: 6px;
  }

  .alert-source-page {
    height: auto;
    min-height: calc(100vh - 124px);
    padding: 6px;
    overflow: auto;
  }

  .ops-page-header {
    gap: 8px;
  }

  .ops-page-header__content {
    flex-wrap: wrap;
    gap: 2px 8px;
  }

  .ops-page-header__divider {
    display: none;
  }

  .ops-page-header__content p {
    width: 100%;
    white-space: normal;
  }

  .source-header-summary {
    padding-left: 50px;
    overflow-x: auto;
  }

  .source-summary-grid {
    grid-template-columns: 1fr;
  }

  .alert-source-page :deep(.alert-center-card .el-card__body) {
    overflow: visible;
  }

  .alert-source-page .alert-table-shell {
    min-height: 420px;
  }

  :global(.alert-source-detail-overlay .el-drawer) {
    width: calc(100vw - 20px) !important;
  }
}
</style>
