<template>
  <div class="db-asset-page">
    <div class="page-shell">
      <header class="db-hero-panel">
        <div class="db-hero-copy">
          <span class="db-hero-icon" aria-hidden="true">
            <img src="@/assets/image/数据库.svg" alt="" />
          </span>
          <div class="db-hero-content">
            <h1>数据库资产</h1>
            <span class="db-hero-divider" aria-hidden="true"></span>
            <p>统一维护数据库实例、归属关系、连接信息、安全画像与备份监控状态。</p>
          </div>
        </div>
        <div class="summary-grid" aria-label="数据库资产摘要">
          <article v-for="item in summaryCards" :key="item.key" class="summary-chip">
            <span>{{ item.label }}</span>
            <strong>{{ item.value }}</strong>
          </article>
        </div>
      </header>

      <section class="page-card">
        <div class="content-stack">
          <section class="toolbar-card">
            <el-form ref="queryFormRef" :model="queryParams" :inline="true" class="toolbar-form" @submit.prevent>
              <el-form-item label="关键词" prop="keyword">
                <el-input
                  v-model="queryParams.keyword"
                  clearable
                  placeholder="名称 / IP / Endpoint / 业务系统"
                  @keyup.enter="handleQuery"
                />
              </el-form-item>
              <el-form-item label="数据库类型" prop="type">
                <el-select v-model="queryParams.type" class="filter-select" clearable placeholder="全部类型" @change="handleQuery">
                  <el-option v-for="item in databaseTypes" :key="item.value" :label="item.label" :value="item.value">
                    <div class="db-type-option">
                      <img :src="item.icon" class="db-type-icon" alt="" />
                      <span>{{ item.label }}</span>
                    </div>
                  </el-option>
                </el-select>
              </el-form-item>
              <el-form-item label="环境" prop="environment">
                <el-select v-model="queryParams.environment" class="filter-select filter-select--short" clearable placeholder="全部环境" @change="handleQuery">
                  <el-option v-for="item in environments" :key="item.value" :label="item.label" :value="item.value" />
                </el-select>
              </el-form-item>
              <el-form-item label="状态" prop="status">
                <el-select v-model="queryParams.status" class="filter-select filter-select--short" clearable placeholder="全部状态" @change="handleQuery">
                  <el-option v-for="item in assetStatuses" :key="item.value" :label="item.label" :value="item.value" />
                </el-select>
              </el-form-item>
              <el-form-item class="toolbar-actions-item">
                <div class="toolbar-actions">
                  <el-button type="primary" size="small" @click="handleQuery">
                    <el-icon><Search /></el-icon>
                    查询
                  </el-button>
                  <el-button type="warning" size="small" class="warning-action" @click="resetQuery">
                    <el-icon><Refresh /></el-icon>
                    重置
                  </el-button>
                  <el-button type="success" size="small" class="success-action" @click="openCreateDialog">
                    <el-icon><Plus /></el-icon>
                    新增资产
                  </el-button>
                </div>
              </el-form-item>
            </el-form>
          </section>

          <section class="table-card">
            <div class="table-head">
              <div>
                <h3>资产列表</h3>
              </div>
            </div>

            <div class="table-shell">
              <el-table
                v-loading="loading"
                :data="assetRows"
                class="db-asset-table"
                height="100%"
                @sort-change="handleSortChange"
              >
                <el-table-column prop="instanceName" label="实例名称" min-width="150" show-overflow-tooltip sortable="custom">
                  <template #default="{ row }">
                    <button class="name-link" type="button" @click="openDetailDialog(row)">
                      {{ row.instanceName || row.name || '-' }}
                    </button>
                  </template>
                </el-table-column>
                <el-table-column prop="type" label="类型" width="108" sortable="custom">
                  <template #default="{ row }">
                    <el-tag effect="light" class="type-tag">
                      <img :src="databaseTypeIcon(row.type)" class="db-type-icon" alt="" />
                      {{ databaseTypeText(row.type, row.typeName) }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="version" label="版本" width="96" show-overflow-tooltip />
                <el-table-column prop="hostIp" label="IP:端口" min-width="136" sortable="custom">
                  <template #default="{ row }">
                    <span class="mono-text">{{ row.endpoint || joinEndpoint(row.hostIp, row.port) }}</span>
                  </template>
                </el-table-column>
                <el-table-column prop="environment" label="环境" width="82">
                  <template #default="{ row }">
                    <el-tag effect="light" class="env-tag">{{ environmentText(row.environment) }}</el-tag>
                  </template>
                </el-table-column>

                <el-table-column prop="businessSystem" label="业务系统" min-width="118" show-overflow-tooltip />
                <el-table-column label="监控状态" width="96">
                  <template #default="{ row }">
                    <span :class="['inline-status', monitorStatusClass(row.monitorStatus)]"></span>
                    {{ monitorStatusText(row.monitorStatus) }}
                  </template>
                </el-table-column>
                <el-table-column prop="status" label="存活状态" width="88">
                  <template #default="{ row }">
                    <el-tag effect="light" :type="statusTag(row.status)">{{ statusText(row.status) }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="认证状态" width="104">
                  <template #default="{ row }">
                    <el-tag effect="light" :type="authStatusTag(row.authStatus)">{{ authStatusText(row.authStatus) }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="190" align="center" header-align="center" class-name="operation-column">
                  <template #default="{ row }">
                    <div class="operation-links">
                      <el-button link type="success" :loading="testingId === row.id" @click="handleTestAuth(row)">测试</el-button>
                      <el-button link type="primary" @click="openEditDialog(row)">修改</el-button>
                      <el-button link type="warning" @click="openDetailDialog(row)">详情</el-button>
                      <el-button link type="danger" @click="handleDelete(row)">删除</el-button>
                    </div>
                  </template>
                </el-table-column>
              </el-table>
            </div>

            <div class="pagination-section">
              <el-pagination
                v-model:current-page="queryParams.page"
                v-model:page-size="queryParams.pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="total"
                layout="total, sizes, prev, pager, next, jumper"
                @size-change="handleSizeChange"
                @current-change="handleCurrentChange"
              />
            </div>
          </section>
        </div>

        <el-dialog
          v-model="formDialogVisible"
          :title="dialogTitle"
          width="900px"
          class="db-asset-dialog"
          modal-class="db-asset-dialog-overlay"
          destroy-on-close
        >
          <el-form
            ref="formRef"
            :model="formData"
            :rules="formRules"
            label-position="top"
            class="asset-form"
          >
            <section v-for="section in formSections" :key="section.title" class="dialog-section">
              <div class="section-head">
                <h4>{{ section.title }}</h4>
                <p>{{ section.description }}</p>
              </div>
              <el-row :gutter="16">
                <el-col v-for="field in section.fields" :key="field.prop" :xs="24" :sm="field.span || 12">
                  <el-form-item :label="field.label" :prop="field.prop">
                    <el-select
                      v-if="field.prop === 'accountId'"
                      v-model="formData.accountId"
                      class="full-select"
                      clearable
                      filterable
                      remote
                      reserve-keyword
                      placeholder="请选择通用认证账号"
                      :loading="accountLoading"
                      :remote-method="searchAccountOptions"
                      @visible-change="handleAccountVisibleChange"
                    >
                      <el-option
                        v-for="option in filteredAccountOptions"
                        :key="option.value"
                        :label="option.label"
                        :value="option.value"
                      >
                        <div class="account-option">
                          <strong>{{ option.label }}</strong>
                          <span>{{ option.description || accountTypeText(option.type) }}</span>
                        </div>
                      </el-option>
                    </el-select>
                    <el-select
                      v-else-if="field.prop === 'prometheusAccountId'"
                      v-model="formData.prometheusAccountId"
                      class="full-select"
                      clearable
                      filterable
                      remote
                      reserve-keyword
                      placeholder="请选择 Prometheus 数据源"
                      :loading="prometheusAccountLoading"
                      :remote-method="searchPrometheusAccountOptions"
                      @visible-change="handlePrometheusAccountVisibleChange"
                    >
                      <el-option
                        v-for="option in filteredPrometheusAccountOptions"
                        :key="option.value"
                        :label="option.label"
                        :value="option.value"
                      >
                        <div class="account-option">
                          <strong>{{ option.label }}</strong>
                          <span>{{ option.description || accountTypeText(option.type) }}</span>
                        </div>
                      </el-option>
                    </el-select>
                    <component v-else :is="field.component" v-model="formData[field.prop]" v-bind="field.props" @change="field.onChange && field.onChange($event)">
                      <template v-if="field.prop === 'type'" #prefix>
                        <img :src="databaseTypeIcon(formData.type)" class="db-type-icon" alt="" />
                      </template>
                      <template v-if="field.options">
                        <el-option
                          v-for="option in field.options"
                          :key="option.value"
                          :label="option.label"
                          :value="option.value"
                        >
                          <div class="db-type-option" :class="{ 'is-plain': field.prop !== 'type' }">
                            <img v-if="field.prop === 'type'" :src="option.icon" class="db-type-icon" alt="" />
                            <span>{{ option.label }}</span>
                          </div>
                        </el-option>
                      </template>
                    </component>
                  </el-form-item>
                </el-col>
              </el-row>
            </section>
          </el-form>
          <template #footer>
            <div class="dialog-footer">
              <el-button @click="formDialogVisible = false">取消</el-button>
              <el-button type="primary" :loading="submitting" @click="submitForm">
                {{ isEditing ? '保存修改' : '创建资产' }}
              </el-button>
            </div>
          </template>
        </el-dialog>

        <el-drawer v-model="detailDrawerVisible" title="数据库资产画像" size="760px" class="db-asset-detail-drawer">
          <div v-loading="detailLoading" class="detail-drawer-body">
            <template v-if="currentDetail">
              <div class="detail-hero">
                <div>
                  <p>{{ databaseTypeText(currentDetail.type, currentDetail.typeName) }}</p>
                  <h3>{{ currentDetail.instanceName || currentDetail.name }}</h3>
                  <span>{{ currentDetail.endpoint || joinEndpoint(currentDetail.hostIp, currentDetail.port) }}</span>
                </div>
                <el-tag effect="light" :type="statusTag(currentDetail.status)">{{ statusText(currentDetail.status) }}</el-tag>
              </div>
              <div class="detail-grid">
                <article v-for="item in detailItems" :key="item.label" class="detail-item">
                  <span>{{ item.label }}</span>
                  <strong>{{ item.value || '-' }}</strong>
                </article>
              </div>
            </template>
            <el-empty v-else description="暂无资产详情" />
          </div>
        </el-drawer>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Refresh, Search } from '@element-plus/icons-vue'
import cmdbApi from '@/api/cmdb'
import configApi from '@/api/config'
import mysqlIcon from '@/assets/image/mysqldb.svg'
import postgresqlIcon from '@/assets/image/PostgreSQL.svg'
import commonDatabaseIcon from '@/assets/image/数据库.svg'
import redisIcon from '@/assets/image/redis.svg'
import mongoIcon from '@/assets/image/mongodb.svg'
import elasticsearchIcon from '@/assets/image/Elasticsearch.svg'

const databaseTypes = [
  { value: 1, accountType: 3, label: 'MySQL', port: 3306, icon: mysqlIcon },
  { value: 2, accountType: 6, label: 'PostgreSQL', port: 5432, icon: postgresqlIcon },
  { value: 3, accountType: 7, label: 'Oracle', port: 1521, icon: commonDatabaseIcon },
  { value: 4, accountType: 4, label: 'Redis', port: 6379, icon: redisIcon },
  { value: 5, accountType: 8, label: 'MongoDB', port: 27017, icon: mongoIcon },
  { value: 6, accountType: 9, label: 'SQL Server', port: 1433, icon: commonDatabaseIcon },
  { value: 7, accountType: 10, label: 'Elasticsearch', port: 9200, icon: elasticsearchIcon }
]

const assetStatuses = [
  { value: 1, label: '运行中' },
  { value: 2, label: '已停止' },
  { value: 3, label: '异常' },
  { value: 4, label: '已下线' }
]

const environments = [
  { value: 'prod', label: '生产' },
  { value: 'pre', label: '预发' },
  { value: 'test', label: '测试' },
  { value: 'dev', label: '开发' }
]

const accountTypes = [
  { value: 1, label: 'Prometheus 数据源' },
  { value: 2, label: 'Grafana 数据源' },
  { value: 3, label: 'MySQL 账号' },
  { value: 4, label: 'Redis 账号' },
  { value: 5, label: '通用认证账号' },
  { value: 6, label: 'PostgreSQL 账号' },
  { value: 7, label: 'Oracle 账号' },
  { value: 8, label: 'MongoDB 账号' },
  { value: 9, label: 'SQL Server 账号' },
  { value: 10, label: 'Elasticsearch 账号' }
]

const emptyForm = () => ({
  id: undefined,
  name: '',
  instanceName: '',
  type: 1,
  version: '',
  environment: 'prod',
  status: 1,
  accountId: undefined,
  prometheusAccountId: undefined,
  groupId: undefined,
  hostId: undefined,
  hostIp: '',
  port: 3306,
  endpoint: '',
  region: '',
  idcRoomId: undefined,
  networkArea: '',
  publicAccess: false,
  businessSystem: '',
  project: '',
  owner: '',
  slowLogPath: '',
  dbaOwner: '',
  opsOwner: '',
  department: '',
  cpu: '',
  memory: '',
  storageSizeGb: undefined,
  usedStorageGb: undefined,
  storageType: '',
  deployMode: '',
  nodeCount: 1,
  clusterName: '',
  role: 'single',
  primaryId: undefined,
  authMethod: 'password',
  sslEnabled: false,
  auditEnabled: false,
  securityLevel: '',
  riskTags: '',
  backupPolicy: '',
  lastBackupTime: '',
  monitorStatus: 0,
  alertStatus: 0,
  lastInspectionTime: '',
  expireTime: '',
  tags: '',
  description: ''
})

const queryFormRef = ref(null)
const formRef = ref(null)
const loading = ref(false)
const submitting = ref(false)
const detailLoading = ref(false)
const formDialogVisible = ref(false)
const detailDrawerVisible = ref(false)
const isEditing = ref(false)
const assetRows = ref([])
const total = ref(0)
const statistics = ref(null)
const currentDetail = ref(null)
const accountOptions = ref([])
const accountLoading = ref(false)
const accountKeyword = ref('')
const accountSearchSeq = ref(0)
const accountOptionCache = ref(new Map())
const prometheusAccountOptions = ref([])
const prometheusAccountLoading = ref(false)
const prometheusAccountKeyword = ref('')
const prometheusAccountSearchSeq = ref(0)
const prometheusAccountOptionCache = ref(new Map())
const testingId = ref('')
const formData = reactive(emptyForm())

const queryParams = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  type: '',
  environment: '',
  status: '',
  businessSystem: '',
  owner: '',
  hostIp: '',
  sortBy: 'updatedAt',
  sortOrder: 'desc'
})

const formRules = {
  name: [{ required: true, message: '请输入资产名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择数据库类型', trigger: 'change' }],
  environment: [{ required: true, message: '请选择环境', trigger: 'change' }],
  hostIp: [{ required: true, message: '请输入主机 IP', trigger: 'blur' }],
  port: [{ required: true, message: '请输入端口', trigger: 'blur' }]
}

const summaryCards = computed(() => {
  const detailMap = new Map((statistics.value?.details || []).map(item => [Number(item.type), item.count]))
  return [
    { key: 'total', label: '资产总数', value: statistics.value?.total ?? total.value },
    { key: 'mysql', label: 'MySQL', value: detailMap.get(1) ?? statistics.value?.summary?.mysql ?? 0 },
    { key: 'redis', label: 'Redis', value: detailMap.get(4) ?? statistics.value?.summary?.redis ?? 0 },
    { key: 'mongo', label: 'MongoDB', value: detailMap.get(5) ?? statistics.value?.summary?.mongodb ?? 0 }
  ]
})

const dialogTitle = computed(() => isEditing.value ? '修改数据库资产' : '新增数据库资产')

const filteredAccountOptions = computed(() => {
  const keyword = accountKeyword.value.trim().toLowerCase()
  return accountOptions.value.filter(item => accountMatchesKeyword(item, keyword))
})

const filteredPrometheusAccountOptions = computed(() => {
  const keyword = prometheusAccountKeyword.value.trim().toLowerCase()
  return prometheusAccountOptions.value.filter(item => accountMatchesKeyword(item, keyword))
})

const formSections = computed(() => [
  {
    title: '基础信息',
    description: '创建资产只录入核心台账字段，后续可在详情或治理流程中补充完整画像。',
    fields: [
      inputField('name', '资产名称', '请输入唯一资产名称'),
      selectField('type', '数据库类型', databaseTypes, handleTypeChange),
      selectField('environment', '环境', environments),
      selectField('status', '状态', assetStatuses),
      inputField('hostIp', '主机 IP', '例如 192.168.1.10'),
      numberField('port', '端口', '数据库监听端口'),
      accountSelectField(),
      prometheusAccountSelectField(),
      inputField('version', '版本', '例如 8.0.36'),
      inputField('businessSystem', '业务系统', '例如 订单系统'),
      inputField('owner', '负责人', '资产负责人'),
      inputField('slowLogPath', '慢日志路径', '例如 /var/log/mysql/slow.log', 24),
      inputField('tags', '标签', 'env=prod,app=order', 24),
      inputField('description', '备注', '请输入资产备注', 24)
    ]
  }
])

const detailItems = computed(() => {
  const item = currentDetail.value || {}
  return [
    { label: '资产名称', value: item.name },
    { label: '环境', value: environmentText(item.environment) },
    { label: '业务系统', value: item.businessSystem },
    { label: '负责人', value: item.owner },
    { label: '慢日志路径', value: item.slowLogPath },
    { label: 'DBA 负责人', value: item.dbaOwner },
    { label: '运维负责人', value: item.opsOwner },
    { label: 'CPU / 内存', value: [item.cpu, item.memory].filter(Boolean).join(' / ') },
    { label: '容量', value: formatStorage(item) },
    { label: '部署方式', value: item.deployMode },
    { label: '集群名称', value: item.clusterName },
    { label: '公网暴露', value: booleanText(item.publicAccess) },
    { label: 'SSL / 审计', value: `${booleanText(item.sslEnabled)} / ${booleanText(item.auditEnabled)}` },
    { label: '备份策略', value: item.backupPolicy },
    { label: '最近备份', value: item.lastBackupTime },
    { label: '认证账号 ID', value: item.accountId },
    { label: 'Prometheus 数据源 ID', value: item.prometheusAccountId },
    { label: '认证状态', value: authStatusText(item.authStatus) },
    { label: '监控状态', value: monitorStatusText(item.monitorStatus) },
    { label: '告警状态', value: alertStatusText(item.alertStatus) },
    { label: '风险标签', value: item.riskTags },
    { label: '更新时间', value: item.updatedAt }
  ]
})

function inputField(prop, label, placeholder, span = 12) {
  return { prop, label, span, component: 'el-input', props: { placeholder, clearable: true } }
}

function numberField(prop, label, placeholder, span = 12) {
  return { prop, label, span, component: 'el-input-number', props: { placeholder, min: 0, controlsPosition: 'right', class: 'full-number' } }
}

function selectField(prop, label, options, onChange, span = 12) {
  return { prop, label, span, component: 'el-select', options, onChange, props: { placeholder: `请选择${label}`, clearable: true, class: 'full-select' } }
}

function accountSelectField() {
  return {
    prop: 'accountId',
    label: '认证账号',
    span: 12,
    component: 'el-select',
    options: filteredAccountOptions.value,
    props: {
      placeholder: '请选择通用认证账号',
      clearable: true,
      filterable: true,
      remote: true,
      remoteMethod: searchAccountOptions,
      onVisibleChange: handleAccountVisibleChange,
      loading: accountLoading.value,
      class: 'full-select'
    }
  }
}

function prometheusAccountSelectField() {
  return {
    prop: 'prometheusAccountId',
    label: 'Prometheus 数据源',
    span: 12,
    component: 'el-select',
    options: filteredPrometheusAccountOptions.value,
    props: {
      placeholder: '请选择 Prometheus 数据源',
      clearable: true,
      filterable: true,
      remote: true,
      remoteMethod: searchPrometheusAccountOptions,
      onVisibleChange: handlePrometheusAccountVisibleChange,
      loading: prometheusAccountLoading.value,
      class: 'full-select'
    }
  }
}

function apiPayload(response) {
  const payload = response?.data || response || {}
  return payload.data !== undefined ? payload.data : payload
}

function isSuccess(response) {
  const payload = response?.data || response || {}
  return payload.code === 200 || payload.code === 0 || payload.success === true || payload.message === '成功'
}

function normalizeAccountList(data) {
  if (Array.isArray(data)) return data
  if (Array.isArray(data?.list)) return data.list
  if (Array.isArray(data?.rows)) return data.rows
  if (data && typeof data === 'object' && data.id !== undefined) return [data]
  return []
}

function currentAccountTypes() {
  const currentType = databaseTypes.find(item => item.value === Number(formData.type))
  return [currentType?.accountType, 5].filter(Boolean).map(Number)
}

function accountMatchesKeyword(item, keyword = '') {
  if (!keyword) return true
  return [
    item.label,
    item.description,
    item.alias,
    item.name,
    item.username,
    item.host,
    item.port
  ].some(value => String(value || '').toLowerCase().includes(keyword))
}

function accountToOption(item = {}) {
  const description = [item.host, item.port].filter(Boolean).join(':')
  return {
    ...item,
    value: item.id,
    label: item.alias || item.name || `账号 #${item.id}`,
    description,
    type: item.type
  }
}

function mergeAccountOptions(list) {
  const merged = new Map()
  const selectedId = formData.accountId

  list.forEach(item => {
    if (item?.id !== undefined && item?.id !== null) {
      merged.set(String(item.id), accountToOption(item))
    }
  })

  if (selectedId !== undefined && selectedId !== null && selectedId !== '') {
    const selectedKey = String(selectedId)
    const selectedItem = accountOptionCache.value.get(selectedKey)
    if (selectedItem && !merged.has(selectedKey)) {
      merged.set(selectedKey, selectedItem)
    }
  }

  const options = Array.from(merged.values())
  options.forEach(item => accountOptionCache.value.set(String(item.value), item))
  return options
}

function mergePrometheusAccountOptions(list) {
  const merged = new Map()
  const selectedId = formData.prometheusAccountId

  list.forEach(item => {
    if (item?.id !== undefined && item?.id !== null) {
      merged.set(String(item.id), accountToOption(item))
    }
  })

  if (selectedId !== undefined && selectedId !== null && selectedId !== '') {
    const selectedKey = String(selectedId)
    const selectedItem = prometheusAccountOptionCache.value.get(selectedKey)
    if (selectedItem && !merged.has(selectedKey)) {
      merged.set(selectedKey, selectedItem)
    }
  }

  const options = Array.from(merged.values())
  options.forEach(item => prometheusAccountOptionCache.value.set(String(item.value), item))
  return options
}

function buildQueryParams() {
  const params = {}
  Object.entries(queryParams).forEach(([key, value]) => {
    if (value !== '' && value !== undefined && value !== null) params[key] = value
  })
  return params
}

async function loadList() {
  loading.value = true
  try {
    const response = await cmdbApi.getDatabaseAssetList(buildQueryParams())
    if (!isSuccess(response)) {
      ElMessage.error(response?.data?.message || '获取数据库资产失败')
      return
    }
    const data = apiPayload(response) || {}
    assetRows.value = Array.isArray(data.list) ? data.list : (Array.isArray(data) ? data : [])
    total.value = Number(data.total) || assetRows.value.length
  } catch (error) {
    console.error('获取数据库资产失败:', error)
  } finally {
    loading.value = false
  }
}

async function loadStatistics() {
  try {
    const response = await cmdbApi.getDatabaseAssetStatistics()
    if (isSuccess(response)) statistics.value = apiPayload(response)
  } catch (error) {
    console.warn('获取数据库资产统计失败:', error)
  }
}

async function loadAccountOptions(keyword = '') {
  const searchText = (keyword || '').trim()
  const currentSeq = ++accountSearchSeq.value
  accountLoading.value = true
  accountKeyword.value = searchText
  try {
    const allowedTypes = currentAccountTypes()
    const requests = [
      configApi.listAccountAuth({
        page: 1,
        pageNum: 1,
        pageSize: searchText ? 1000 : 100
      })
    ]

    if (searchText) {
      requests.push(configApi.getAccountAuthByAlias(searchText))
    }

    const responses = await Promise.allSettled(requests)

    if (currentSeq !== accountSearchSeq.value) return

    const list = responses.flatMap(result => {
      if (result.status !== 'fulfilled' || !isSuccess(result.value)) return []
      return normalizeAccountList(apiPayload(result.value))
    })

    const exactMatches = searchText
      ? list.filter(item => String(item.alias || '').toLowerCase() === searchText.toLowerCase())
      : []
    const filteredList = list
      .filter(item => allowedTypes.includes(Number(item.type)) || exactMatches.some(match => String(match.id) === String(item.id)))
      .filter(item => accountMatchesKeyword(item, searchText.toLowerCase()))

    accountOptions.value = mergeAccountOptions(filteredList)
  } catch (error) {
    if (currentSeq !== accountSearchSeq.value) return
    console.warn('获取通用认证账号失败:', error)
    accountOptions.value = mergeAccountOptions([])
  } finally {
    if (currentSeq === accountSearchSeq.value) {
      accountLoading.value = false
    }
  }
}

async function loadSelectedAccountOption() {
  const selectedId = formData.accountId
  if (selectedId === undefined || selectedId === null || selectedId === '') return
  if (accountOptionCache.value.has(String(selectedId))) return

  try {
    const response = await configApi.listAccountAuth({
      page: 1,
      pageNum: 1,
      pageSize: 1000
    })
    if (!isSuccess(response)) return
    const selected = normalizeAccountList(apiPayload(response))
      .find(item => String(item.id) === String(selectedId))
    if (selected) {
      accountOptionCache.value.set(String(selectedId), accountToOption(selected))
      accountOptions.value = mergeAccountOptions(accountOptions.value)
    }
  } catch (error) {
    console.warn('获取当前认证账号失败:', error)
  }
}

async function loadPrometheusAccountOptions(keyword = '') {
  const searchText = (keyword || '').trim()
  const currentSeq = ++prometheusAccountSearchSeq.value
  prometheusAccountLoading.value = true
  prometheusAccountKeyword.value = searchText
  try {
    const requests = [configApi.getAccountAuthByType(1)]

    if (searchText) {
      requests.push(configApi.getAccountAuthByAlias(searchText))
    }

    const responses = await Promise.allSettled(requests)

    if (currentSeq !== prometheusAccountSearchSeq.value) return

    const list = responses.flatMap(result => {
      if (result.status !== 'fulfilled' || !isSuccess(result.value)) return []
      return normalizeAccountList(apiPayload(result.value))
    })

    const filteredList = list
      .filter(item => Number(item.type) === 1)
      .filter(item => accountMatchesKeyword(item, searchText.toLowerCase()))

    prometheusAccountOptions.value = mergePrometheusAccountOptions(filteredList)
  } catch (error) {
    if (currentSeq !== prometheusAccountSearchSeq.value) return
    console.warn('获取 Prometheus 数据源失败:', error)
    prometheusAccountOptions.value = mergePrometheusAccountOptions([])
  } finally {
    if (currentSeq === prometheusAccountSearchSeq.value) {
      prometheusAccountLoading.value = false
    }
  }
}

async function loadSelectedPrometheusAccountOption() {
  const selectedId = formData.prometheusAccountId
  if (selectedId === undefined || selectedId === null || selectedId === '') return
  if (prometheusAccountOptionCache.value.has(String(selectedId))) return

  try {
    const response = await configApi.getAccountAuthByType(1)
    if (!isSuccess(response)) return
    const selected = normalizeAccountList(apiPayload(response))
      .find(item => String(item.id) === String(selectedId))
    if (selected) {
      prometheusAccountOptionCache.value.set(String(selectedId), accountToOption(selected))
      prometheusAccountOptions.value = mergePrometheusAccountOptions(prometheusAccountOptions.value)
    }
  } catch (error) {
    console.warn('获取当前 Prometheus 数据源失败:', error)
  }
}

function searchAccountOptions(keyword = '') {
  loadAccountOptions(keyword)
}

function handleAccountVisibleChange(visible) {
  if (visible) loadAccountOptions(accountKeyword.value)
  if (!visible) accountKeyword.value = ''
}

function searchPrometheusAccountOptions(keyword = '') {
  loadPrometheusAccountOptions(keyword)
}

function handlePrometheusAccountVisibleChange(visible) {
  if (visible) loadPrometheusAccountOptions(prometheusAccountKeyword.value)
  if (!visible) prometheusAccountKeyword.value = ''
}

function accountTypeText(type) {
  return accountTypes.find(item => item.value === Number(type))?.label || '通用认证账号'
}

function handleQuery() {
  queryParams.page = 1
  loadList()
}

function resetQuery() {
  queryFormRef.value?.resetFields()
  Object.assign(queryParams, {
    page: 1,
    pageSize: 10,
    keyword: '',
    type: '',
    environment: '',
    status: '',
    businessSystem: '',
    owner: '',
    hostIp: '',
    sortBy: 'updatedAt',
    sortOrder: 'desc'
  })
  loadList()
}

function handleSizeChange(size) {
  queryParams.page = 1
  queryParams.pageSize = size
  loadList()
}

function handleCurrentChange(page) {
  queryParams.page = page
  loadList()
}

function handleSortChange({ prop, order }) {
  queryParams.sortBy = prop || 'updatedAt'
  queryParams.sortOrder = order === 'ascending' ? 'asc' : 'desc'
  loadList()
}

function resetForm(data = {}) {
  Object.assign(formData, emptyForm(), data)
  if (!formData.instanceName) formData.instanceName = formData.name || ''
  if (!formData.port) handleTypeChange(formData.type)
  loadAccountOptions()
  loadSelectedAccountOption()
  loadPrometheusAccountOptions()
  loadSelectedPrometheusAccountOption()
}

function openCreateDialog() {
  isEditing.value = false
  resetForm()
  formDialogVisible.value = true
}

async function openEditDialog(row) {
  isEditing.value = true
  const detail = await fetchDetail(row.id)
  resetForm(detail || row)
  formDialogVisible.value = true
}

async function openDetailDialog(row) {
  detailDrawerVisible.value = true
  currentDetail.value = null
  currentDetail.value = await fetchDetail(row.id) || row
}

async function fetchDetail(id) {
  if (!id) return null
  detailLoading.value = true
  try {
    const response = await cmdbApi.getDatabaseAssetInfo(id)
    if (!isSuccess(response)) {
      ElMessage.error(response?.data?.message || '获取资产详情失败')
      return null
    }
    return apiPayload(response)
  } catch (error) {
    console.error('获取资产详情失败:', error)
    return null
  } finally {
    detailLoading.value = false
  }
}

function handleTypeChange(type) {
  const item = databaseTypes.find(option => option.value === Number(type))
  if (item && (!formData.port || !isEditing.value)) formData.port = item.port
  const selectedAccount = accountOptions.value.find(option => option.value === formData.accountId)
  if (selectedAccount && Number(selectedAccount.type) !== Number(item?.accountType) && Number(selectedAccount.type) !== 5) {
    formData.accountId = undefined
  }
  loadAccountOptions(accountKeyword.value)
}

function normalizeSubmitData() {
  const coreData = {
    name: formData.name,
    type: formData.type,
    environment: formData.environment,
    status: formData.status,
    version: formData.version,
    hostIp: formData.hostIp,
    port: formData.port,
    accountId: formData.accountId,
    prometheusAccountId: formData.prometheusAccountId,
    endpoint: formData.hostIp && formData.port ? joinEndpoint(formData.hostIp, formData.port) : formData.endpoint,
    businessSystem: formData.businessSystem,
    owner: formData.owner,
    slowLogPath: formData.slowLogPath,
    tags: formData.tags,
    description: formData.description
  }

  if (!isEditing.value) return coreData
  return {
    ...formData,
    ...coreData,
    instanceName: formData.instanceName || formData.name
  }
}

async function submitForm() {
  await formRef.value?.validate()
  submitting.value = true
  try {
    const payload = normalizeSubmitData()
    const response = isEditing.value
      ? await cmdbApi.updateDatabaseAsset(payload)
      : await cmdbApi.createDatabaseAsset(payload)
    if (!isSuccess(response)) {
      ElMessage.error(response?.data?.message || '保存数据库资产失败')
      return
    }
    ElMessage.success(isEditing.value ? '资产已更新' : '资产已创建')
    formDialogVisible.value = false
    await Promise.all([loadList(), loadStatistics()])
  } catch (error) {
    console.error('保存数据库资产失败:', error)
  } finally {
    submitting.value = false
  }
}

async function handleTestAuth(row) {
  if (!row?.id) return
  testingId.value = row.id
  try {
    const response = await cmdbApi.testDatabaseAssetAuth(row.id)
    if (!isSuccess(response)) {
      ElMessage.error(response?.data?.message || '认证测试失败')
      return
    }
    const data = apiPayload(response) || {}
    const success = data.success === true || Number(data.authStatus) === 2
    if (success) {
      ElMessage.success(authMessage(true, data.message))
    } else {
      ElMessage.error(authMessage(false, data.message || authStatusText(data.authStatus)))
    }
    await loadList()
    if (currentDetail.value?.id === row.id) currentDetail.value = await fetchDetail(row.id) || currentDetail.value
  } catch (error) {
    console.error('认证测试失败:', error)
    ElMessage.error(error.response?.data?.message || error.message || '认证测试失败')
  } finally {
    testingId.value = ''
  }
}

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm(
      `确定删除数据库资产「${row.instanceName || row.name}」吗？删除资产不会删除真实数据库实例。`,
      '删除确认',
      {
        type: 'warning',
        confirmButtonText: '删除',
        cancelButtonText: '取消'
      }
    )
    const response = await cmdbApi.deleteDatabaseAsset(row.id)
    if (!isSuccess(response)) {
      ElMessage.error(response?.data?.message || '删除数据库资产失败')
      return
    }
    ElMessage.success('资产已删除')
    await Promise.all([loadList(), loadStatistics()])
  } catch (error) {
    if (error !== 'cancel') console.error('删除数据库资产失败:', error)
  }
}

function databaseTypeText(type, fallback = '') {
  return fallback || databaseTypes.find(item => item.value === Number(type))?.label || '-'
}

function databaseTypeIcon(type) {
  return databaseTypes.find(item => item.value === Number(type))?.icon || commonDatabaseIcon
}

function environmentText(value) {
  return environments.find(item => item.value === value)?.label || value || '-'
}

function statusText(status) {
  return assetStatuses.find(item => item.value === Number(status))?.label || '未知'
}

function statusTag(status) {
  const mapping = { 1: 'success', 2: 'info', 3: 'danger', 4: 'info' }
  return mapping[Number(status)] || 'info'
}

function authMessage(success, message = '') {
  const normalized = String(message || '').trim().toLowerCase()
  if (success) return normalized === 'auth success' ? '认证成功' : (message || '认证成功')
  return normalized === 'auth failed' || normalized === 'auth fail' ? '认证失败' : (message || '认证失败')
}

function authStatusText(status) {
  const mapping = { 0: '未认证', 1: '认证失败', 2: '认证成功' }
  return mapping[Number(status)] || '未认证'
}

function authStatusTag(status) {
  const mapping = { 0: 'info', 1: 'danger', 2: 'success' }
  return mapping[Number(status)] || 'info'
}

function monitorStatusText(status) {
  return Number(status) === 1 ? '已接入' : '未接入'
}

function monitorStatusClass(status) {
  return Number(status) === 1 ? 'is-success' : 'is-muted'
}

function alertStatusText(status) {
  return Number(status) === 1 ? '告警中' : '正常'
}

function booleanText(value) {
  return value === true || value === 1 ? '是' : '否'
}

function joinEndpoint(hostIp, port) {
  return hostIp ? `${hostIp}${port ? `:${port}` : ''}` : '-'
}

function formatStorage(item = {}) {
  const used = item.usedStorageGb ?? '-'
  const totalSize = item.storageSizeGb ?? '-'
  return `${used} / ${totalSize} GB`
}

onMounted(() => {
  loadList()
  loadStatistics()
  loadAccountOptions()
})
</script>

<style scoped>
/* DESIGN.md: primitive -> semantic -> component tokens */
.db-asset-page {
  --db-blue-500: #409eff;
  --db-blue-600: #2563eb;
  --db-green-500: #67c23a;
  --db-yellow-500: #e6a23c;
  --db-red-500: #ef4444;
  --db-slate-950: #0f172a;
  --db-slate-800: #172033;
  --db-slate-600: #53657d;
  --db-slate-500: #64748b;
  --db-slate-400: #7b8aa0;

  --db-color-primary: var(--db-blue-500);
  --db-color-success: var(--db-green-500);
  --db-color-warning: var(--db-yellow-500);
  --db-color-danger: var(--db-red-500);
  --db-text-heading: var(--db-slate-950);
  --db-text-primary: var(--db-slate-800);
  --db-text-secondary: var(--db-slate-600);
  --db-text-muted: var(--db-slate-500);
  --db-bg-surface: #fff;
  --db-bg-surface-soft: #fbfdff;
  --db-bg-subtle: #f3f7fc;
  --db-border: #dbe7f4;
  --db-border-soft: #e7eef7;
  --db-border-hover: #b9d4ff;

  --db-card-radius: 12px;
  --db-inner-radius: 10px;
  --db-control-radius: 8px;
  --db-dialog-radius: 15px;
  --db-control-height: 30px;
  --db-card-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
  --db-hero-shadow: 0 10px 28px rgba(80, 124, 180, 0.1);

  box-sizing: border-box;
  height: calc(100vh - 124px);
  min-height: 0;
  padding: 7px;
  overflow: hidden;
  color: var(--db-text-primary);
  background: transparent;
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

:global(.home-container .el-main:has(.db-asset-page)) {
  padding: 10px;
  overflow: hidden;
}

.page-shell {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  align-content: start;
  gap: 8px;
  width: 100%;
  height: 100%;
  min-height: 0;
}

.db-hero-panel {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
  padding: 8px 12px;
  border: 1px solid var(--db-border);
  border-radius: var(--db-card-radius);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 251, 255, 0.95));
  box-shadow: var(--db-hero-shadow);
}

.db-hero-copy {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  gap: 8px;
  min-width: 0;
}

.db-hero-icon {
  display: grid;
  flex: 0 0 42px;
  width: 42px;
  height: 42px;
  place-items: center;
  border: 0;
  background: transparent;
}

.db-hero-icon img {
  width: 30px;
  height: 30px;
  object-fit: contain;
}

.db-hero-content {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  gap: 10px;
  min-width: 0;
}

.db-hero-content h1 {
  flex: 0 0 auto;
  margin: 0;
  color: var(--db-text-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: 0;
  white-space: nowrap;
}

.db-hero-divider {
  flex: 0 0 1px;
  width: 1px;
  height: 22px;
  background: var(--db-border);
}

.db-hero-content p {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  color: var(--db-text-muted);
  font-size: 12px;
  line-height: 1.5;
  letter-spacing: 0;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.summary-grid {
  display: flex;
  align-items: stretch;
  flex: 0 0 auto;
  min-width: 0;
}

.summary-chip {
  display: grid;
  grid-template-columns: auto auto;
  align-items: baseline;
  gap: 6px;
  min-width: 82px;
  padding: 3px 12px;
  border: 0;
  border-left: 1px solid var(--db-border-soft);
  border-radius: 0;
  background: transparent;
}

.summary-chip span {
  overflow: hidden;
  color: var(--db-slate-400);
  font-size: 11px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.summary-chip strong {
  margin: 0;
  color: var(--db-text-heading);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 18px;
  font-weight: 700;
  line-height: 1;
}

.page-card {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  height: 100%;
  padding: 0;
  overflow: hidden;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.page-card::before {
  display: none;
}

.content-stack {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  min-height: 0;
}

.toolbar-card,
.table-card {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--db-border);
  border-radius: var(--db-card-radius);
  background: rgba(255, 255, 255, 0.97);
  box-shadow: var(--db-card-shadow);
}

.toolbar-card {
  flex: 0 0 auto;
  padding: 8px 10px;
}

.toolbar-form {
  display: flex;
  align-items: center;
  flex-wrap: nowrap;
  gap: 8px;
  min-width: 0;
}

.toolbar-form :deep(.el-form-item) {
  flex: 0 0 auto;
  margin: 0;
}

.toolbar-form :deep(.el-form-item__label) {
  height: var(--db-control-height);
  padding-right: 7px;
  color: var(--db-text-secondary);
  font-size: 12px;
  font-weight: 700;
  line-height: var(--db-control-height);
}

.toolbar-form :deep(.el-form-item__content) {
  min-width: 0;
}

.toolbar-form :deep(.el-input) {
  width: 280px;
}

.filter-select {
  width: 152px;
}

.filter-select--short {
  width: 128px;
}

.toolbar-actions-item {
  margin-left: 0;
}

.toolbar-actions {
  display: flex;
  align-items: center;
  flex-wrap: nowrap;
  gap: 6px;
}

.toolbar-actions :deep(.el-button + .el-button) {
  margin-left: 0;
}

.db-asset-page :deep(.el-input__wrapper),
.db-asset-page :deep(.el-select__wrapper),
.db-asset-page :deep(.el-textarea__inner),
.db-asset-page :deep(.el-input-number .el-input__wrapper),
.asset-form :deep(.el-input__wrapper),
.asset-form :deep(.el-select__wrapper),
.asset-form :deep(.el-textarea__inner),
.asset-form :deep(.el-input-number .el-input__wrapper) {
  min-height: var(--db-control-height);
  border: 0;
  border-radius: var(--db-control-radius);
  background: var(--db-bg-surface-soft);
  box-shadow: 0 0 0 1px #d9e5f2 inset !important;
  transition: background-color 160ms ease, box-shadow 160ms ease;
}

.db-asset-page :deep(.el-input__wrapper:hover),
.db-asset-page :deep(.el-select__wrapper:hover),
.db-asset-page :deep(.el-textarea__inner:hover),
.asset-form :deep(.el-input__wrapper:hover),
.asset-form :deep(.el-select__wrapper:hover),
.asset-form :deep(.el-textarea__inner:hover) {
  box-shadow: 0 0 0 1px var(--db-border-hover) inset !important;
}

.db-asset-page :deep(.el-input__wrapper.is-focus),
.db-asset-page :deep(.el-select__wrapper.is-focused),
.db-asset-page :deep(.el-textarea__inner:focus),
.asset-form :deep(.el-input__wrapper.is-focus),
.asset-form :deep(.el-select__wrapper.is-focused),
.asset-form :deep(.el-textarea__inner:focus) {
  background: #fff;
  box-shadow: 0 0 0 1px var(--db-color-primary) inset !important;
}

.db-asset-page :deep(.el-button) {
  border-radius: var(--db-control-radius);
  font-weight: 600;
  transition: color 160ms ease, background-color 160ms ease, border-color 160ms ease, box-shadow 160ms ease;
}

.db-asset-page :deep(.toolbar-actions .el-button) {
  height: var(--db-control-height);
  min-height: var(--db-control-height);
  padding: 5px 10px;
  font-size: 12px;
}

.db-asset-page :deep(.el-button:not(.is-link):not(.is-text):hover) {
  box-shadow: 0 5px 12px rgba(64, 158, 255, 0.12);
  transform: none;
}

.db-asset-page :deep(.el-button--primary:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: var(--db-color-primary);
  color: #fff;
  background: var(--db-color-primary);
}

.db-asset-page :deep(.el-button--success:not(.is-link):not(.is-text):not(.is-plain)),
.db-asset-page :deep(.success-action) {
  border-color: var(--db-color-success);
  color: #fff;
  background: var(--db-color-success);
}

.db-asset-page :deep(.el-button--warning:not(.is-link):not(.is-text):not(.is-plain)),
.db-asset-page :deep(.warning-action) {
  border-color: var(--db-color-warning);
  color: #fff;
  background: var(--db-color-warning);
}

.db-asset-page :deep(.el-button.is-disabled),
.db-asset-page :deep(.el-button.is-disabled:hover) {
  opacity: 0.55;
  box-shadow: none;
}

.table-card {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  padding: 0;
}

.table-head {
  display: flex;
  align-items: center;
  flex: 0 0 auto;
  justify-content: space-between;
  gap: 12px;
  margin: 0;
  padding: 8px 12px;
  border-bottom: 1px solid var(--db-border-soft);
}

.table-head h3 {
  margin: 0;
  color: var(--db-text-heading);
  font-size: 15px;
  font-weight: 700;
  line-height: 1.25;
}

.table-shell {
  flex: 1 1 auto;
  min-height: 0;
  margin: 8px 10px 0;
  overflow: hidden;
  border: 1px solid #e3ebf5;
  border-radius: var(--db-control-radius);
  background: var(--db-bg-surface);
}

.db-asset-page :deep(.db-asset-table.el-table) {
  --el-table-border-color: #e3ebf5;
  --el-table-header-bg-color: var(--db-bg-subtle);
  --el-table-tr-bg-color: #fff;
  --el-table-row-hover-bg-color: #f8fafc;
  --el-table-current-row-bg-color: #eef6ff;
  --el-table-bg-color: #fff;
  --el-table-text-color: var(--db-text-secondary);
  --el-table-header-text-color: #475569;
  width: 100%;
  height: 100%;
  overflow: hidden;
  border: 0;
  border-radius: var(--db-control-radius);
  background: #fff;
  font-size: 12px;
}

.db-asset-page :deep(.db-asset-table .el-table__inner-wrapper::before) {
  display: none;
}

.db-asset-page :deep(.db-asset-table th.el-table__cell) {
  height: 34px;
  padding: 5px 0;
  color: #475569;
  background: var(--db-bg-subtle) !important;
  font-size: 12px;
  font-weight: 700;
}

.db-asset-page :deep(.db-asset-table td.el-table__cell) {
  padding: 5px 0;
  font-size: 12px;
}

.db-asset-page :deep(.db-asset-table td.el-table__cell),
.db-asset-page :deep(.db-asset-table th.el-table__cell.is-leaf) {
  border-bottom-color: #eef3f9;
}

.db-asset-page :deep(.db-asset-table .el-table__row),
.db-asset-page :deep(.db-asset-table .el-table__row:hover) {
  box-shadow: none;
  transform: none;
}

.db-asset-page :deep(.db-asset-table .el-table__row:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

.db-asset-page :deep(.db-asset-table .el-scrollbar__wrap) {
  overflow-x: auto;
}

.name-link {
  display: block;
  max-width: 100%;
  padding: 0;
  overflow: hidden;
  border: 0;
  color: var(--db-blue-600);
  background: transparent;
  font: inherit;
  font-weight: 700;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
}

.mono-text {
  color: var(--db-text-secondary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  white-space: nowrap;
}

.type-tag,
.env-tag {
  color: var(--db-blue-600);
  background: #eef6ff;
}

.type-tag {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.db-type-option,
.account-option {
  min-width: 0;
}

.db-type-option {
  display: flex;
  align-items: center;
  gap: 8px;
}

.db-type-option.is-plain {
  display: block;
}

.db-type-icon {
  flex: 0 0 auto;
  width: 16px;
  height: 16px;
  object-fit: contain;
}

.account-option {
  display: flex;
  flex-direction: column;
  line-height: 1.35;
}

.account-option strong {
  color: var(--db-text-primary);
  font-size: 12px;
  font-weight: 700;
}

.account-option span {
  color: var(--db-text-muted);
  font-size: 11px;
}

.inline-status {
  display: inline-block;
  width: 7px;
  height: 7px;
  margin-right: 5px;
  border-radius: 999px;
  vertical-align: middle;
}

.inline-status.is-success {
  background: var(--db-color-success);
}

.inline-status.is-muted {
  background: var(--db-slate-400);
}

.operation-links {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  width: 100%;
  white-space: nowrap;
}

.operation-links :deep(.el-button) {
  min-width: auto;
  height: auto;
  margin: 0;
  padding: 0;
  border-radius: 0;
  font-size: 12px;
  box-shadow: none;
  transform: none;
}

.pagination-section {
  display: flex;
  flex: 0 0 auto;
  justify-content: flex-end;
  margin: 0;
  padding: 8px 10px 10px;
  border-top: 0;
}

.db-asset-page :deep(.el-pagination .btn-prev),
.db-asset-page :deep(.el-pagination .btn-next),
.db-asset-page :deep(.el-pagination .el-pager li) {
  border-radius: 7px;
  color: var(--db-text-secondary);
  background: #f8fafc;
}

.db-asset-page :deep(.el-pagination .el-pager li.is-active) {
  color: #fff;
  background: var(--db-blue-600);
}

.db-asset-page :deep(.el-tag) {
  border-color: transparent;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
}

.asset-form {
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
}

.dialog-section {
  padding: 0 0 4px;
}

.dialog-section + .dialog-section {
  margin-top: 12px;
  padding-top: 14px;
  border-top: 1px solid var(--db-border-soft);
}

.dialog-section .section-head {
  display: block;
  margin: 0 0 12px;
  padding: 0;
  border: 0;
}

.dialog-section .section-head h4 {
  margin: 0;
  color: var(--db-text-heading);
  font-size: 14px;
  font-weight: 700;
}

.dialog-section .section-head p {
  margin: 3px 0 0;
  color: var(--db-text-muted);
  font-size: 11px;
  line-height: 1.45;
}

.asset-form :deep(.el-form-item) {
  margin-bottom: 14px;
}

.asset-form :deep(.el-form-item__label) {
  height: auto;
  margin-bottom: 6px;
  padding: 0;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.4;
}

.asset-form :deep(.el-input__wrapper),
.asset-form :deep(.el-select__wrapper),
.asset-form :deep(.el-input-number .el-input__wrapper) {
  min-height: 34px;
}

.full-select,
.full-date,
.full-number {
  width: 100%;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

:global(.db-asset-dialog-overlay) {
  --db-color-primary: #409eff;
  --db-text-heading: #0f172a;
  --db-text-primary: #172033;
  --db-text-muted: #64748b;
  --db-bg-surface-soft: #fbfdff;
  --db-border-soft: #e7eef7;
  --db-border-hover: #b9d4ff;
  --db-control-radius: 8px;
  --db-control-height: 30px;
  background: rgba(15, 23, 42, 0.22) !important;
}

:global(.db-asset-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  width: min(900px, calc(100vw - 24px)) !important;
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.db-asset-dialog-overlay .el-dialog__header) {
  position: relative;
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: linear-gradient(90deg, #fff6e7 0%, #f7fbff 36%, #ecf7ff 100%);
}

:global(.db-asset-dialog-overlay .el-dialog__header::before) {
  content: "";
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 3px;
  background: linear-gradient(90deg, #f5a524 0%, #62a8ff 48%, #55d6be 100%);
}

:global(.db-asset-dialog-overlay .el-dialog__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.db-asset-dialog-overlay .el-dialog__headerbtn) {
  top: 10px;
  right: 12px;
}

:global(.db-asset-dialog-overlay .el-dialog__body) {
  max-height: calc(100vh - 190px);
  padding: 16px 18px 4px;
  overflow-y: auto;
}

:global(.db-asset-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #fff;
}

:global(.db-asset-dialog-overlay .dialog-footer .el-button) {
  min-width: 78px;
  min-height: 32px;
  border-radius: 8px;
  font-weight: 600;
}

:global(.db-asset-dialog-overlay .dialog-footer .el-button--primary) {
  border-color: #409eff;
  color: #fff;
  background: linear-gradient(135deg, #5aa8ff, #2563eb);
}

:global(.db-asset-detail-drawer .el-drawer__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
  background: linear-gradient(90deg, #fff6e7 0%, #f7fbff 36%, #ecf7ff 100%);
}

:global(.db-asset-detail-drawer) {
  --db-blue-600: #2563eb;
  --db-text-heading: #0f172a;
  --db-text-primary: #172033;
  --db-text-secondary: #53657d;
  --db-text-muted: #64748b;
  --db-border: #dbe7f4;
  --db-inner-radius: 10px;
  --db-control-radius: 8px;
}

:global(.db-asset-detail-drawer .el-drawer__body) {
  padding: 14px 16px;
  background: #fbfdff;
}

.detail-drawer-body {
  min-height: 100%;
}

.detail-hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  margin: 0 0 10px;
  padding: 12px 14px;
  border: 1px solid var(--db-border);
  border-radius: var(--db-inner-radius);
  background: #fff;
}

.detail-hero p {
  margin: 0 0 4px;
  color: var(--db-blue-600);
  font-size: 11px;
  font-weight: 700;
}

.detail-hero h3 {
  margin: 0;
  color: var(--db-text-heading);
  font-size: 18px;
  font-weight: 700;
}

.detail-hero span {
  display: block;
  margin-top: 5px;
  color: var(--db-text-secondary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.detail-item {
  min-width: 0;
  padding: 9px 10px;
  border: 1px solid var(--db-border);
  border-radius: var(--db-control-radius);
  background: #fff;
}

.detail-item span {
  display: block;
  color: var(--db-text-muted);
  font-size: 11px;
}

.detail-item strong {
  display: block;
  margin-top: 4px;
  overflow-wrap: anywhere;
  color: var(--db-text-primary);
  font-size: 12px;
  font-weight: 700;
}

@media (max-width: 1420px) {
  .toolbar-form {
    align-items: flex-start;
    flex-wrap: wrap;
  }
}

@media (max-width: 1100px) {
  .db-hero-panel {
    align-items: stretch;
    flex-direction: column;
  }

  .summary-grid {
    border-top: 1px solid var(--db-border-soft);
  }

  .summary-chip {
    flex: 1 1 0;
    justify-content: center;
    padding-top: 7px;
    border-left: 0;
  }
}

@media (max-width: 768px) {
  .db-asset-page {
    height: auto;
    padding: 6px;
    overflow: visible;
  }

  .page-shell {
    grid-template-rows: auto auto;
    height: auto;
  }

  .db-hero-content {
    align-items: flex-start;
    flex-direction: column;
    gap: 3px;
  }

  .db-hero-divider {
    display: none;
  }

  .db-hero-content p {
    white-space: normal;
  }

  .summary-grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .summary-chip {
    justify-content: space-between;
    border-bottom: 1px solid var(--db-border-soft);
  }

  .page-card,
  .content-stack {
    overflow: visible;
  }

  .toolbar-form,
  .toolbar-actions,
  .toolbar-form :deep(.el-form-item),
  .toolbar-form :deep(.el-input),
  .filter-select,
  .filter-select--short {
    width: 100%;
  }

  .toolbar-form {
    align-items: stretch;
    flex-direction: column;
  }

  .toolbar-form :deep(.el-form-item__content) {
    flex: 1;
  }

  .toolbar-actions {
    align-items: stretch;
    flex-direction: column;
  }

  .toolbar-actions :deep(.el-button) {
    width: 100%;
  }

  .table-card,
  .table-shell {
    min-height: 560px;
  }

  .pagination-section {
    overflow-x: auto;
  }

  .detail-grid {
    grid-template-columns: 1fr;
  }

  :global(.db-asset-dialog-overlay .el-dialog__body) {
    padding-right: 14px;
    padding-left: 14px;
  }
}
</style>
