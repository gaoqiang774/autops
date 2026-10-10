<template>
  <div class="business-health-page">
    <header class="service-hero-panel">
      <div class="service-hero-copy">
        <span class="service-hero-icon" aria-hidden="true">
          <img src="@/assets/image/业务组.svg" alt="" />
        </span>
        <div class="service-hero-content">
          <h1>业务健康驾驶舱</h1>
          <span class="service-hero-divider" aria-hidden="true"></span>
          <p>统一维护业务模型、健康评分规则与运行风险状态。</p>
        </div>
      </div>
    </header>

    <section class="page-card">
      <section class="overview-toolbar-panel">
        <div class="overview-strip">
          <article v-for="item in overviewCards" :key="item.key" class="overview-card">
            <span>{{ item.label }}</span>
            <strong :style="{ color: item.color }">{{ item.value }}</strong>
          </article>
        </div>
        <div class="overview-toolbar-divider" aria-hidden="true"></div>
        <el-form :model="queryParams" :inline="true" class="toolbar-form" @submit.prevent>
          <el-form-item label="关键词">
            <el-input
              v-model="queryParams.keyword"
              clearable
              placeholder="业务名称 / 编码 / 负责人"
              @keyup.enter="handleQuery"
            />
          </el-form-item>
          <el-form-item label="状态" class="status-filter-item">
            <el-select v-model="queryParams.healthStatus" clearable placeholder="全部状态" @change="handleQuery">
              <el-option v-for="item in healthStatusOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item class="toolbar-actions-item">
            <div class="toolbar-actions">
              <el-button type="primary" size="small" @click="handleQuery">
                <el-icon><Search /></el-icon>
                查询
              </el-button>
              <el-button type="warning" size="small" class="reset-button" @click="resetQuery">
                <el-icon><Refresh /></el-icon>
                重置
              </el-button>
              <el-button type="success" size="small" @click="openCreateDialog">
                <el-icon><Plus /></el-icon>
                新建业务
              </el-button>
              <el-button type="warning" size="small" class="rule-button" @click="openRuleDialog">
                <el-icon><Setting /></el-icon>
                编辑规则
              </el-button>
            </div>
          </el-form-item>
        </el-form>
      </section>

      <main class="dashboard-grid">
        <section class="business-wall">
          <div class="section-head">
            <div>
              <h3>业务状态卡</h3>
            </div>
            <span class="section-pill">共 {{ total }} 项</span>
          </div>

          <div v-loading="listLoading" class="business-card-grid">
            <article
              v-for="item in businessList"
              :key="item.id"
              class="health-card"
              :class="scoreState(item).key"
              :style="{
                '--status-color': scoreState(item).color,
                '--status-soft': scoreState(item).softColor,
                '--status-border': scoreState(item).borderColor,
                '--status-icon-bg': scoreState(item).iconBg,
                '--status-glow': scoreState(item).glowColor
              }"
              @click="goDetail(item)"
            >
              <el-tooltip content="删除业务" placement="top">
                <el-button
                  class="card-delete-button"
                  type="danger"
                  :icon="Delete"
                  circle
                  plain
                  aria-label="删除业务"
                  :loading="deletingId === item.id"
                  @click.stop="handleDeleteBusiness(item)"
                />
              </el-tooltip>

              <div class="health-orb">
                <div class="orb-pulse-ring"></div>
                <div class="orb-pulse-ring orb-pulse-ring--delay"></div>
                <div class="score-text">
                  <strong>{{ formatScore(item.healthScore) }}</strong>
                  <span>分</span>
                </div>
                <div class="flame-badge">
                  <svg viewBox="0 0 64 64" aria-hidden="true">
                    <defs>
                      <radialGradient id="flameGlow" cx="50%" cy="54%" r="54%">
                        <stop offset="0%" stop-color="currentColor" stop-opacity="0.55" />
                        <stop offset="62%" stop-color="currentColor" stop-opacity="0.16" />
                        <stop offset="100%" stop-color="currentColor" stop-opacity="0" />
                      </radialGradient>
                    </defs>
                    <circle class="flame-halo" cx="32" cy="35" r="24" fill="url(#flameGlow)" />
                    <path class="flame-main" d="M35.8 8.2c1.1 8.5 8.8 12.8 12.4 20.4 4.8 10.1-.9 23.2-13.1 25.4-10.8 2-20.2-4.5-21.4-15.2-.8-6.9 2.4-12.7 7.8-18.5.7 5.6 3.3 8.5 7.1 9.8-1.8-8.5.4-15.8 7.2-21.9z" />
                    <path class="flame-inner" d="M31.7 31.5c5.3 4.1 8.2 8 8.2 12.4 0 5.5-4.1 9.3-9.2 9.3-5.2 0-8.9-3.6-8.9-8.9 0-4.8 3.5-8.8 9.9-12.8z" />
                    <path class="flame-shine" d="M28.6 24.2c-.1 4.7 1.6 7.9 5.4 10.8" />
                  </svg>
                </div>
              </div>

              <div class="health-card-body">
                <div class="health-card-head">
                  <div>
                    <h4>{{ item.displayName || item.name || '未命名业务' }}</h4>
                    <p>{{ envText(item.env) }} · {{ item.owner || '负责人未定' }}</p>
                  </div>
                  <span class="health-pill">{{ scoreState(item).label }}</span>
                </div>

                <div class="card-meta">
                  <span>刷新 {{ formatShortTime(item.updatedAt) }}</span>
                  <span>{{ item.bizCode || item.name || '-' }}</span>
                </div>

                <div class="card-actions">
                  <button type="button" @click.stop="goDetail(item)">详情</button>
                </div>
              </div>
            </article>

            <div v-if="!listLoading && !businessList.length" class="empty-state">
              <el-empty description="暂无业务健康数据">
                <el-button type="primary" @click="openCreateDialog">新建业务</el-button>
              </el-empty>
            </div>
          </div>

          <div class="pagination-section">
            <el-pagination
              v-model:current-page="queryParams.page"
              v-model:page-size="queryParams.pageSize"
              :page-sizes="[15, 30, 50, 100]"
              :total="total"
              layout="total, sizes, prev, pager, next, jumper"
              @size-change="handleSizeChange"
              @current-change="handleCurrentChange"
            />
          </div>
        </section>
      </main>
    </section>

    <el-dialog
      v-model="createDialogVisible"
      title="新建业务模型"
      width="560px"
      class="business-dialog"
      modal-class="business-create-dialog-overlay"
      destroy-on-close
    >
      <el-form
        ref="createFormRef"
        :model="createForm"
        :rules="createRules"
        label-position="top"
        class="business-form"
      >
        <el-row :gutter="14">
          <el-col :xs="24" :sm="12">
            <el-form-item label="业务唯一名称" prop="name">
              <el-input v-model="createForm.name" placeholder="lechat" clearable />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item label="展示名称" prop="displayName">
              <el-input v-model="createForm.displayName" placeholder="乐聊系统" clearable />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="8">
            <el-form-item label="环境" prop="env">
              <el-select v-model="createForm.env" placeholder="环境">
                <el-option v-for="item in envOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="8">
            <el-form-item label="负责人" prop="owner">
              <el-input v-model="createForm.owner" placeholder="张帆" clearable />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="8">
            <el-form-item label="业务编码" prop="bizCode">
              <el-input v-model="createForm.bizCode" placeholder="LECHAT" clearable />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>

      <template #footer>
        <el-button @click="createDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="submitCreate">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="ruleDialogVisible"
      title="编辑健康评分规则"
      width="1120px"
      class="business-dialog rule-dialog"
      modal-class="business-rule-dialog-overlay"
      destroy-on-close
    >
      <div class="rule-dialog-toolbar">
        <el-select
          v-model="selectedRuleBusinessId"
          filterable
          placeholder="选择业务"
          class="rule-business-select"
          @change="loadBusinessRules"
        >
          <el-option
            v-for="item in businessList"
            :key="item.id"
            :label="item.displayName || item.name"
            :value="item.id"
          />
        </el-select>
        <div class="rule-dialog-actions">
          <el-button :disabled="ruleSaving || ruleLoading" @click="setAllRulesEnabled(false)">全部停用</el-button>
          <el-button :disabled="ruleSaving || ruleLoading" @click="setAllRulesEnabled(true)">全部启用</el-button>
          <el-button :loading="ruleResetting" @click="resetRules">恢复默认</el-button>
          <el-button type="primary" :loading="ruleSaving" @click="saveRules">保存规则</el-button>
        </div>
      </div>

      <div v-loading="ruleLoading" class="rule-editor">
        <el-empty v-if="!ruleLoading && !healthRules.length" description="暂无规则数据" />
        <el-tabs v-else v-model="activeRuleGroup" class="rule-tabs">
          <el-tab-pane
            v-for="group in ruleGroups"
            :key="group.key"
            :label="`${group.label} (${group.items.length})`"
            :name="group.key"
          >
            <el-table :data="group.items" height="438" class="rule-table" row-key="ruleKey">
              <el-table-column label="规则项" min-width="220">
                <template #default="{ row }">
                  <div class="rule-name-cell">
                    <strong>{{ row.ruleName || row.ruleKey }}</strong>
                    <span>{{ row.ruleKey }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="维度" width="92">
                <template #default="{ row }">
                  <el-tag size="small" effect="plain">{{ dimensionLabel(row.dimension) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="启用" width="78" align="center">
                <template #default="{ row }">
                  <el-switch
                    v-model="row.enabled"
                    inline-prompt
                    active-text="启"
                    inactive-text="停"
                    :width="46"
                    @change="value => handleRuleEnabledChange(row, value)"
                  />
                </template>
              </el-table-column>
              <el-table-column label="分数权重" width="126" align="center">
                <template #default="{ row }">
                  <el-select v-model="row.weight" size="small" class="rule-weight-select">
                    <el-option v-for="item in ruleWeightOptions" :key="item.value" :label="item.label" :value="item.value" />
                  </el-select>
                </template>
              </el-table-column>
              <el-table-column label="告警阈值" width="132">
                <template #default="{ row }">
                  <el-input-number
                    v-model="row.warningThreshold"
                    :controls="false"
                    :precision="thresholdPrecision(row)"
                    size="small"
                    placeholder="-"
                  />
                </template>
              </el-table-column>
              <el-table-column label="严重阈值" width="132">
                <template #default="{ row }">
                  <el-input-number
                    v-model="row.criticalThreshold"
                    :controls="false"
                    :precision="thresholdPrecision(row)"
                    size="small"
                    placeholder="-"
                  />
                </template>
              </el-table-column>
              <el-table-column label="单位" width="70">
                <template #default="{ row }">
                  <span class="rule-unit">{{ row.unit || '-' }}</span>
                </template>
              </el-table-column>
              <el-table-column label="说明" min-width="220">
                <template #default="{ row }">
                  <span class="rule-description">{{ row.description || '-' }}</span>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>
        </el-tabs>
      </div>

      <template #footer>
        <el-button @click="ruleDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="ruleSaving" @click="saveRules">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Plus, Refresh, Search, Setting } from '@element-plus/icons-vue'
import {
  createBusinessModel,
  deleteBusinessModel,
  getBusinessHealthRules,
  listBusinessModels,
  refreshBusinessHealth,
  resetBusinessHealthRules,
  saveBusinessHealthRules
} from '@/api/cmdbBusinessModel'

const router = useRouter()
const createFormRef = ref(null)

const listLoading = ref(false)
const creating = ref(false)
const deletingId = ref(null)
const createDialogVisible = ref(false)
const ruleDialogVisible = ref(false)
const ruleLoading = ref(false)
const ruleSaving = ref(false)
const ruleResetting = ref(false)
const selectedRuleBusinessId = ref('')
const healthRules = ref([])
const activeRuleGroup = ref('')
const total = ref(0)
const businessList = ref([])

const queryParams = reactive({
  page: 1,
  pageSize: 15,
  keyword: '',
  healthStatus: ''
})

const createForm = reactive({
  name: '',
  displayName: '',
  env: 'prod',
  owner: '',
  bizCode: ''
})

const createRules = {
  name: [{ required: true, message: '请输入业务唯一名称', trigger: 'blur' }]
}

const envOptions = [
  { label: '生产', value: 'prod' },
  { label: '预发', value: 'uat' },
  { label: '测试', value: 'test' },
  { label: '开发', value: 'dev' }
]

const healthStatusOptions = [
  { label: '正常', value: 'healthy' },
  { label: '黄色风险', value: 'warning' },
  { label: '红色异常', value: 'critical' },
  { label: '未接入', value: 'unknown' }
]

const dimensionTextMap = {
  overall: '整体',
  infra: '主机',
  middleware: '中间件',
  container: 'K8S',
  business: '业务',
  log: '日志'
}

const ruleGroupTextMap = {
  business: '业务',
  business_service: '服务',
  nginx: '入口',
  log: '日志',
  host: '主机',
  k8s_workload: 'K8S',
  database: '数据库'
}

const ruleGroupOrder = ['business', 'business_service', 'nginx', 'host', 'k8s_workload', 'database', 'log']

const ruleWeightOptions = [
  { label: '1份', value: 1 },
  { label: '2份', value: 2 },
  { label: '3份', value: 3 },
  { label: '4份', value: 4 }
]

const logRuleKeys = new Set(['log.coverage', 'host.system_log', 'service.log_error', 'k8s.log_error'])

const scoreStateMap = {
  healthy: {
    label: '健康',
    color: '#16a34a',
    softColor: 'rgba(88, 204, 122, 0.20)',
    borderColor: 'rgba(88, 204, 122, 0.46)',
    iconBg: 'rgba(88, 204, 122, 0.20)',
    glowColor: 'rgba(88, 204, 122, 0.42)'
  },
  warning: {
    label: '有问题',
    color: '#f59e0b',
    softColor: 'rgba(248, 186, 68, 0.22)',
    borderColor: 'rgba(248, 186, 68, 0.48)',
    iconBg: 'rgba(248, 186, 68, 0.20)',
    glowColor: 'rgba(248, 186, 68, 0.42)'
  },
  critical: {
    label: '异常',
    color: '#ef5f67',
    softColor: 'rgba(239, 95, 103, 0.22)',
    borderColor: 'rgba(239, 95, 103, 0.48)',
    iconBg: 'rgba(239, 95, 103, 0.20)',
    glowColor: 'rgba(239, 95, 103, 0.42)'
  },
  unknown: {
    label: '未接入',
    color: '#94a3b8',
    softColor: 'rgba(148, 163, 184, 0.18)',
    borderColor: 'rgba(148, 163, 184, 0.34)',
    iconBg: 'rgba(148, 163, 184, 0.12)',
    glowColor: 'rgba(148, 163, 184, 0.26)'
  }
}

const overviewCards = computed(() => {
  const counts = businessList.value.reduce((acc, item) => {
    const status = scoreState(item).key
    acc[status] = (acc[status] || 0) + 1
    return acc
  }, {})

  return [
    { key: 'total', label: '业务总数', value: total.value, color: '#172133' },
    { key: 'healthy', label: '正常业务', value: counts.healthy || 0, color: '#16a34a' },
    { key: 'warning', label: '黄色业务', value: counts.warning || 0, color: '#f59e0b' },
    { key: 'critical', label: '红色业务', value: counts.critical || 0, color: '#dc2626' },
    { key: 'unknown', label: '未接入健康数据', value: counts.unknown || 0, color: '#94a3b8' }
  ]
})

const ruleGroups = computed(() => {
  const groupMap = healthRules.value.reduce((acc, item) => {
    const key = item.resourceType || 'other'
    if (!acc[key]) {
      acc[key] = []
    }
    acc[key].push(item)
    return acc
  }, {})

  return Object.keys(groupMap)
    .sort((left, right) => {
      const leftIndex = ruleGroupOrder.indexOf(left)
      const rightIndex = ruleGroupOrder.indexOf(right)
      if (leftIndex !== -1 || rightIndex !== -1) {
        return (leftIndex === -1 ? 99 : leftIndex) - (rightIndex === -1 ? 99 : rightIndex)
      }
      return left.localeCompare(right)
    })
    .map(key => ({
      key,
      label: ruleGroupTextMap[key] || key,
      items: groupMap[key].sort((left, right) => Number(left.sortOrder || 0) - Number(right.sortOrder || 0))
    }))
})

const normalizeResponse = response => {
  const axiosResponse = response?.data ? response : { data: response }
  return axiosResponse.data || null
}

const envText = env => envOptions.find(item => item.value === env)?.label || env || '未设置环境'

const formatScore = value => {
  if (value === null || value === undefined || value === '') return '--'
  return Number(value)
}

const scoreState = item => {
  const score = Number(item?.healthScore)
  if (!Number.isFinite(score)) return { key: 'unknown', ...scoreStateMap.unknown }
  if (score >= 80) return { key: 'healthy', ...scoreStateMap.healthy }
  if (score >= 60) return { key: 'warning', ...scoreStateMap.warning }
  return { key: 'critical', ...scoreStateMap.critical }
}

const formatShortTime = value => {
  if (!value) return '暂无刷新'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  const pad = number => `${number}`.padStart(2, '0')
  return `${pad(date.getHours())}:${pad(date.getMinutes())}`
}

const dimensionLabel = value => dimensionTextMap[value] || value || '-'

const thresholdPrecision = row => {
  if (row?.unit === '%') return 1
  const values = [row?.warningThreshold, row?.criticalThreshold]
  return values.some(value => Number(value) > 0 && Number(value) < 1) ? 2 : 0
}

const isLogRule = rule => {
  const ruleKey = String(rule?.ruleKey || '')
  return rule?.dimension === 'log' || logRuleKeys.has(ruleKey) || ruleKey.startsWith('log.') || ruleKey.includes('.log_')
}

const handleRuleEnabledChange = (row, enabled) => {
  row.enabled = enabled === true
  if (row.ruleKey === 'log.coverage' && !row.enabled) {
    healthRules.value.forEach(rule => {
      if (isLogRule(rule)) {
        rule.enabled = false
      }
    })
  }
}

const setAllRulesEnabled = enabled => {
  healthRules.value.forEach(rule => {
    rule.enabled = enabled === true
  })
}

const sortBusinessList = list => {
  const priority = { critical: 0, warning: 1, healthy: 2, unknown: 3 }
  return [...list].sort((left, right) => {
    const leftPriority = priority[scoreState(left).key] ?? 9
    const rightPriority = priority[scoreState(right).key] ?? 9
    if (leftPriority !== rightPriority) return leftPriority - rightPriority
    return Number(right.healthScore ?? -1) - Number(left.healthScore ?? -1)
  })
}

const fetchBusinessList = async () => {
  listLoading.value = true
  try {
    const res = normalizeResponse(await listBusinessModels({
      page: queryParams.page,
      pageSize: queryParams.pageSize,
      keyword: queryParams.keyword || undefined,
      status: queryParams.healthStatus || undefined,
      healthStatus: queryParams.healthStatus || undefined
    }))

    if (res?.code !== 0) {
      throw new Error(res?.message || '获取业务健康列表失败')
    }

    const data = res.data || {}
    const list = Array.isArray(data.list) ? data.list : []
    businessList.value = sortBusinessList(list)
    total.value = Number(data.total || list.length || 0)
  } catch (error) {
    console.error('获取业务健康列表失败:', error)
    ElMessage.error(error.message || '获取业务健康列表失败')
  } finally {
    listLoading.value = false
  }
}

const handleQuery = () => {
  queryParams.page = 1
  fetchBusinessList()
}

const resetQuery = () => {
  queryParams.page = 1
  queryParams.pageSize = 15
  queryParams.keyword = ''
  queryParams.healthStatus = ''
  fetchBusinessList()
}

const handleSizeChange = size => {
  queryParams.pageSize = size
  queryParams.page = 1
  fetchBusinessList()
}

const handleCurrentChange = page => {
  queryParams.page = page
  fetchBusinessList()
}

const resetCreateForm = () => {
  createForm.name = ''
  createForm.displayName = ''
  createForm.env = 'prod'
  createForm.owner = ''
  createForm.bizCode = ''
}

const openCreateDialog = () => {
  createDialogVisible.value = true
  resetCreateForm()
}

const normalizeRule = rule => ({
  id: rule.id,
  businessId: rule.businessId,
  resourceType: rule.resourceType || 'other',
  ruleKey: rule.ruleKey,
  ruleName: rule.ruleName,
  dimension: rule.dimension,
  enabled: rule.enabled !== false,
  weight: Number(rule.weight || 1),
  operator: rule.operator,
  warningThreshold: rule.warningThreshold === null || rule.warningThreshold === undefined ? null : Number(rule.warningThreshold),
  criticalThreshold: rule.criticalThreshold === null || rule.criticalThreshold === undefined ? null : Number(rule.criticalThreshold),
  unit: rule.unit,
  params: rule.params || {},
  description: rule.description,
  sortOrder: Number(rule.sortOrder || 0)
})

const ensureActiveRuleGroup = () => {
  const groups = ruleGroups.value
  if (!groups.length) {
    activeRuleGroup.value = ''
    return
  }
  if (!groups.some(group => group.key === activeRuleGroup.value)) {
    activeRuleGroup.value = groups[0].key
  }
}

const loadBusinessRules = async () => {
  if (!selectedRuleBusinessId.value) return
  ruleLoading.value = true
  try {
    const res = normalizeResponse(await getBusinessHealthRules(selectedRuleBusinessId.value))
    if (res?.code !== 0) {
      throw new Error(res?.message || '获取健康规则失败')
    }
    healthRules.value = Array.isArray(res.data?.rules) ? res.data.rules.map(normalizeRule) : []
    ensureActiveRuleGroup()
  } catch (error) {
    console.error('获取健康规则失败:', error)
    ElMessage.error(error.message || '获取健康规则失败')
  } finally {
    ruleLoading.value = false
  }
}

const openRuleDialog = async () => {
  if (!businessList.value.length) {
    await fetchBusinessList()
  }
  if (!businessList.value.length) {
    ElMessage.warning('暂无可编辑的业务')
    return
  }
  ruleDialogVisible.value = true
  if (!businessList.value.some(item => item.id === selectedRuleBusinessId.value)) {
    selectedRuleBusinessId.value = businessList.value[0].id
  }
  await loadBusinessRules()
}

const buildRulePayload = () => ({
  rules: healthRules.value.map(rule => ({
    ruleKey: rule.ruleKey,
    ruleName: rule.ruleName,
    resourceType: rule.resourceType,
    dimension: rule.dimension,
    enabled: rule.enabled === true,
    weight: Number(rule.weight || 1),
    operator: rule.operator,
    warningThreshold: rule.warningThreshold === '' || rule.warningThreshold === null || rule.warningThreshold === undefined ? null : Number(rule.warningThreshold),
    criticalThreshold: rule.criticalThreshold === '' || rule.criticalThreshold === null || rule.criticalThreshold === undefined ? null : Number(rule.criticalThreshold),
    unit: rule.unit,
    params: rule.params || {},
    description: rule.description,
    sortOrder: Number(rule.sortOrder || 0)
  }))
})

const refreshSelectedBusinessHealth = async () => {
  if (!selectedRuleBusinessId.value) return
  try {
    await refreshBusinessHealth(selectedRuleBusinessId.value)
  } catch (error) {
    console.warn('刷新业务健康失败:', error)
  }
}

const saveRules = async () => {
  if (!selectedRuleBusinessId.value) {
    ElMessage.warning('请选择业务')
    return
  }
  ruleSaving.value = true
  try {
    const res = normalizeResponse(await saveBusinessHealthRules(selectedRuleBusinessId.value, buildRulePayload()))
    if (res?.code !== 0) {
      throw new Error(res?.message || '保存健康规则失败')
    }
    healthRules.value = Array.isArray(res.data?.rules) ? res.data.rules.map(normalizeRule) : healthRules.value
    ensureActiveRuleGroup()
    await fetchBusinessList()
    ElMessage.success('健康规则已保存')
  } catch (error) {
    console.error('保存健康规则失败:', error)
    ElMessage.error(error.message || '保存健康规则失败')
  } finally {
    ruleSaving.value = false
  }
}

const resetRules = async () => {
  if (!selectedRuleBusinessId.value) {
    ElMessage.warning('请选择业务')
    return
  }
  try {
    await ElMessageBox.confirm('确定恢复该业务的默认健康评分规则吗？', '恢复默认规则', {
      confirmButtonText: '恢复默认',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch (error) {
    return
  }

  ruleResetting.value = true
  try {
    const res = normalizeResponse(await resetBusinessHealthRules(selectedRuleBusinessId.value))
    if (res?.code !== 0) {
      throw new Error(res?.message || '恢复默认规则失败')
    }
    healthRules.value = Array.isArray(res.data?.rules) ? res.data.rules.map(normalizeRule) : []
    ensureActiveRuleGroup()
    await refreshSelectedBusinessHealth()
    await fetchBusinessList()
    ElMessage.success('已恢复默认规则')
  } catch (error) {
    console.error('恢复默认规则失败:', error)
    ElMessage.error(error.message || '恢复默认规则失败')
  } finally {
    ruleResetting.value = false
  }
}

const submitCreate = async () => {
  try {
    await createFormRef.value?.validate()
  } catch (error) {
    return
  }

  creating.value = true
  try {
    const res = normalizeResponse(await createBusinessModel({
      name: createForm.name,
      displayName: createForm.displayName || createForm.name,
      env: createForm.env || undefined,
      owner: createForm.owner || undefined,
      bizCode: createForm.bizCode || undefined,
      properties: {}
    }))

    if (res?.code !== 0) {
      throw new Error(res?.message || '创建业务模型失败')
    }

    ElMessage.success('业务模型创建成功')
    createDialogVisible.value = false
    queryParams.page = 1
    await fetchBusinessList()
  } catch (error) {
    console.error('创建业务模型失败:', error)
    ElMessage.error(error.message || '创建业务模型失败')
  } finally {
    creating.value = false
  }
}

const handleDeleteBusiness = async item => {
  if (!item?.id) {
    ElMessage.warning('业务模型 ID 不能为空')
    return
  }

  const businessName = item.displayName || item.name || '未命名业务'

  try {
    await ElMessageBox.confirm(
      `确定删除业务模型「${businessName}」吗？该操作会删除业务根节点、健康/诊断快照和该业务拓扑关系，绑定的资产资源不会删除。`,
      '删除业务模型',
      {
        confirmButtonText: '确定删除',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )

    deletingId.value = item.id
    const res = normalizeResponse(await deleteBusinessModel(item.id))

    if (res?.code !== 0) {
      throw new Error(res?.message || '删除业务模型失败')
    }

    ElMessage.success('业务模型删除成功')

    if (businessList.value.length === 1 && queryParams.page > 1) {
      queryParams.page -= 1
    }

    await fetchBusinessList()
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    console.error('删除业务模型失败:', error)
    ElMessage.error(error.message || '删除业务模型失败')
  } finally {
    deletingId.value = null
  }
}

const goDetail = item => {
  router.push(`/cmdb/service_model/${item.id}`)
}

onMounted(() => {
  fetchBusinessList()
})
</script>

<style scoped>
/* DESIGN.md: primitive -> semantic -> component tokens */
.business-health-page {
  --service-blue-500: #409eff;
  --service-blue-600: #2563eb;
  --service-green-500: #67c23a;
  --service-yellow-500: #e6a23c;
  --service-red-500: #ef4444;
  --service-slate-950: #0f172a;
  --service-slate-800: #172033;
  --service-slate-600: #53657d;
  --service-slate-500: #64748b;
  --service-slate-400: #7b8aa0;

  --service-color-primary: var(--service-blue-500);
  --service-color-success: var(--service-green-500);
  --service-color-warning: var(--service-yellow-500);
  --service-color-danger: var(--service-red-500);
  --service-text-heading: var(--service-slate-950);
  --service-text-primary: var(--service-slate-800);
  --service-text-secondary: var(--service-slate-600);
  --service-text-muted: var(--service-slate-500);
  --text-primary: #172133;
  --text-secondary: #5f6b7c;
  --text-muted: #8d97a8;
  --service-bg-surface: #fff;
  --service-bg-surface-soft: #fbfdff;
  --service-bg-subtle: #f3f7fc;
  --service-border: #dbe7f4;
  --service-border-soft: #e7eef7;
  --service-border-hover: #b9d4ff;

  --service-card-radius: 12px;
  --service-inner-radius: 10px;
  --service-control-radius: 8px;
  --service-dialog-radius: 15px;
  --service-control-height: 30px;
  --service-card-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
  --service-hero-shadow: 0 10px 28px rgba(80, 124, 180, 0.1);

  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  align-content: start;
  gap: 8px;
  box-sizing: border-box;
  height: calc(100vh - 124px);
  min-height: 0;
  padding: 7px;
  overflow: hidden;
  color: var(--service-text-primary);
  background: transparent;
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

:global(.home-container .el-main:has(.business-health-page)) {
  padding: 10px;
  overflow: hidden;
}

.service-hero-panel {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
  padding: 8px 12px;
  border: 1px solid var(--service-border);
  border-radius: var(--service-card-radius);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 251, 255, 0.95));
  box-shadow: var(--service-hero-shadow);
}

.service-hero-copy {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  gap: 8px;
  min-width: 0;
}

.service-hero-icon {
  display: grid;
  flex: 0 0 42px;
  width: 42px;
  height: 42px;
  place-items: center;
  border: 0;
  background: transparent;
}

.service-hero-icon img {
  width: 30px;
  height: 30px;
  object-fit: contain;
}

.service-hero-content {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  gap: 10px;
  min-width: 0;
}

.service-hero-content h1 {
  flex: 0 0 auto;
  margin: 0;
  color: var(--service-text-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: 0;
  white-space: nowrap;
}

.service-hero-divider {
  flex: 0 0 1px;
  width: 1px;
  height: 22px;
  background: var(--service-border);
}

.service-hero-content p {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  color: var(--service-text-muted);
  font-size: 12px;
  line-height: 1.5;
  letter-spacing: 0;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.page-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
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

.overview-toolbar-panel,
.business-wall {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--service-border);
  border-radius: var(--service-card-radius);
  background: rgba(255, 255, 255, 0.97);
  box-shadow: var(--service-card-shadow);
}

.overview-toolbar-panel {
  flex: 0 0 auto;
}

.overview-strip {
  display: grid;
  grid-template-columns: repeat(5, minmax(100px, 1fr));
  padding: 6px 0;
  background: linear-gradient(180deg, #fff, #fbfdff);
}

.overview-card {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
  padding: 2px 12px;
  border-right: 1px solid var(--service-border-soft);
}

.overview-card:last-child {
  border-right: 0;
}

.overview-card span {
  overflow: hidden;
  color: var(--service-text-muted);
  font-size: 11px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.overview-card strong {
  margin: 0;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 18px;
  font-weight: 700;
  line-height: 1;
}

.overview-toolbar-divider {
  height: 1px;
  background: var(--service-border-soft);
}

.toolbar-form {
  display: flex;
  align-items: center;
  flex-wrap: nowrap;
  gap: 8px;
  padding: 8px 10px;
}

.toolbar-form :deep(.el-form-item) {
  flex: 0 0 auto;
  margin: 0;
}

.toolbar-form :deep(.el-form-item__label) {
  height: var(--service-control-height);
  padding-right: 7px;
  color: var(--service-text-secondary);
  font-size: 12px;
  font-weight: 700;
  line-height: var(--service-control-height);
}

.toolbar-form :deep(.el-form-item__content) {
  min-width: 0;
}

.toolbar-form :deep(.el-input) {
  width: 280px;
}

.status-filter-item :deep(.el-select) {
  width: 168px;
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

.business-health-page :deep(.el-input__wrapper),
.business-health-page :deep(.el-select__wrapper),
.business-health-page :deep(.el-textarea__inner),
.business-health-page :deep(.el-input-number .el-input__wrapper) {
  min-height: var(--service-control-height);
  border: 0;
  border-radius: var(--service-control-radius);
  background: var(--service-bg-surface-soft);
  box-shadow: 0 0 0 1px #d9e5f2 inset !important;
  transition: background-color 160ms ease, box-shadow 160ms ease;
}

.business-health-page :deep(.el-input__wrapper:hover),
.business-health-page :deep(.el-select__wrapper:hover),
.business-health-page :deep(.el-textarea__inner:hover) {
  box-shadow: 0 0 0 1px var(--service-border-hover) inset !important;
}

.business-health-page :deep(.el-input__wrapper.is-focus),
.business-health-page :deep(.el-select__wrapper.is-focused),
.business-health-page :deep(.el-textarea__inner:focus) {
  background: #fff;
  box-shadow: 0 0 0 1px var(--service-color-primary) inset !important;
}

.business-health-page :deep(.el-button) {
  border-radius: var(--service-control-radius);
  font-weight: 600;
  transition: color 160ms ease, background-color 160ms ease, border-color 160ms ease, box-shadow 160ms ease;
}

.business-health-page :deep(.toolbar-actions .el-button) {
  height: var(--service-control-height);
  min-height: var(--service-control-height);
  padding: 5px 10px;
  font-size: 12px;
}

.business-health-page :deep(.el-button:not(.is-link):not(.is-text):hover) {
  box-shadow: 0 5px 12px rgba(64, 158, 255, 0.12);
  transform: none;
}

.business-health-page :deep(.el-button--primary:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: var(--service-color-primary);
  color: #fff;
  background: var(--service-color-primary);
}

.business-health-page :deep(.el-button--success:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: var(--service-color-success);
  color: #fff;
  background: var(--service-color-success);
}

.business-health-page :deep(.el-button--warning:not(.is-link):not(.is-text):not(.is-plain)),
.reset-button,
.rule-button {
  border-color: var(--service-color-warning);
  color: #fff;
  background: var(--service-color-warning);
}

.reset-button:hover,
.reset-button:focus,
.rule-button:hover,
.rule-button:focus {
  border-color: #cf8f2d;
  color: #fff;
  background: #cf8f2d;
}

.dashboard-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  flex: 1 1 auto;
  min-width: 0;
  min-height: 0;
}

.business-wall {
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 0;
}

.section-head {
  display: flex;
  align-items: center;
  flex: 0 0 auto;
  justify-content: space-between;
  gap: 12px;
  margin: 0;
  padding: 8px 12px;
  border-bottom: 1px solid var(--service-border-soft);
}

.section-head h3 {
  margin: 0;
  color: var(--service-text-heading);
  font-size: 15px;
  font-weight: 700;
  line-height: 1.25;
}

.section-pill {
  height: 22px;
  padding: 0 8px;
  border: 1px solid #cfe0ff;
  border-radius: 999px;
  color: var(--service-blue-600);
  background: #eef6ff;
  font-size: 11px;
  font-weight: 600;
  line-height: 20px;
}

.business-card-grid {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 14px;
  flex: 0 0 auto;
  min-height: auto;
  align-content: start;
}

.health-card {
  position: relative;
  display: grid;
  grid-template-columns: 78px minmax(0, 1fr);
  gap: 12px;
  min-height: 122px;
  padding: 14px;
  border: 1px solid rgba(207, 217, 230, 0.78);
  border-radius: 18px;
  background:
    radial-gradient(circle at 42px 46px, var(--status-soft), transparent 58px),
    linear-gradient(135deg, rgba(255, 255, 255, 0.94), rgba(247, 250, 255, 0.82));
  box-shadow: 0 12px 28px rgba(17, 24, 39, 0.06);
  cursor: pointer;
  overflow: hidden;
  transition: transform 0.18s ease, border-color 0.18s ease, box-shadow 0.18s ease;
}

.health-card::before {
  content: "";
  position: absolute;
  inset: 0 auto 0 0;
  width: 4px;
  background: var(--status-color);
}

.health-card:hover {
  transform: translateY(-2px);
  border-color: var(--status-border);
  box-shadow: 0 18px 34px rgba(17, 24, 39, 0.1);
}

.card-delete-button {
  position: absolute;
  top: 9px;
  right: 10px;
  z-index: 6;
  width: 24px;
  height: 24px;
  min-width: 24px;
  min-height: 24px;
  padding: 0;
  border-color: transparent;
  background: transparent;
  color: rgba(100, 116, 139, 0.42);
  box-shadow: none;
  transition: background 0.16s ease, border-color 0.16s ease, color 0.16s ease, box-shadow 0.16s ease;
}

.health-card:hover .card-delete-button,
.card-delete-button:focus,
.card-delete-button.is-loading {
  border-color: rgba(239, 95, 103, 0.44);
  background: rgba(255, 255, 255, 0.9);
  color: #ef5f67;
  box-shadow: 0 8px 18px rgba(239, 95, 103, 0.16);
}

.card-delete-button:hover,
.card-delete-button:focus {
  border-color: rgba(239, 95, 103, 0.62);
  background: rgba(255, 241, 242, 0.96);
  color: #dc2626;
}

.health-orb {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 78px;
}

.health-orb::before,
.health-orb::after {
  content: "";
  position: absolute;
  border-radius: 999px;
}

.health-orb::before {
  width: 62px;
  height: 62px;
  border: 3px solid var(--status-border);
  background:
    radial-gradient(circle at 49% 46%, rgba(255, 255, 255, 0.94) 0 14%, var(--status-soft) 38%, rgba(255, 255, 255, 0.78) 70%),
    rgba(255, 255, 255, 0.76);
  box-shadow:
    inset 0 0 0 7px rgba(15, 23, 42, 0.045),
    inset 0 -14px 24px rgba(15, 23, 42, 0.08),
    0 14px 28px rgba(15, 23, 42, 0.13),
    0 0 0 8px rgba(255, 255, 255, 0.62);
  animation: healthOrbBreath 3.2s ease-in-out infinite;
}

.health-orb::after {
  width: 42px;
  height: 42px;
  border: 1px solid rgba(255, 255, 255, 0.82);
  background:
    radial-gradient(circle at 42% 28%, rgba(255, 255, 255, 0.84), transparent 34%),
    var(--status-soft);
  box-shadow: inset 0 10px 18px rgba(255, 255, 255, 0.42);
  animation: healthCoreBreath 2.7s ease-in-out infinite;
}

.orb-pulse-ring {
  position: absolute;
  width: 62px;
  height: 62px;
  border: 2px solid var(--status-border);
  border-radius: 999px;
  box-shadow: 0 0 18px var(--status-glow);
  opacity: 0;
  pointer-events: none;
  animation: healthPulseRing 2.8s ease-out infinite;
}

.orb-pulse-ring--delay {
  animation-delay: 1.4s;
}

.health-card:hover .orb-pulse-ring {
  animation-duration: 2.1s;
}

.score-text {
  position: absolute;
  top: -2px;
  z-index: 2;
  display: inline-flex;
  align-items: baseline;
  justify-content: center;
  height: 20px;
  color: var(--status-color);
  text-shadow: 0 5px 12px var(--status-glow);
}

.score-text strong {
  font-size: 14px;
  line-height: 1;
}

.score-text span {
  margin-left: 1px;
  font-size: 10px;
  font-weight: 700;
}

.flame-badge {
  position: relative;
  z-index: 3;
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  border-radius: 50%;
  color: var(--status-color);
  background:
    radial-gradient(circle at 40% 30%, rgba(255, 255, 255, 0.68), transparent 34%),
    var(--status-icon-bg);
  box-shadow:
    inset 0 -8px 16px rgba(15, 23, 42, 0.10),
    0 9px 20px var(--status-glow);
  animation: flameFloat 3s ease-in-out infinite;
}

.flame-badge::before {
  content: "";
  position: absolute;
  inset: -7px;
  border: 1px solid var(--status-border);
  border-radius: 50%;
  opacity: 0;
  animation: flameRipple 2.8s ease-out infinite;
}

.flame-badge svg {
  width: 29px;
  height: 29px;
  overflow: visible;
  filter: drop-shadow(0 6px 9px var(--status-glow));
}

.flame-main {
  fill: currentColor;
  animation: flameFlicker 2.4s ease-in-out infinite;
}

.flame-inner {
  fill: rgba(255, 255, 255, 0.55);
  animation: flameCore 2.2s ease-in-out infinite;
}

.flame-shine {
  fill: none;
  stroke: rgba(255, 255, 255, 0.62);
  stroke-width: 3.2;
  stroke-linecap: round;
}

@keyframes healthPulseRing {
  0% {
    transform: scale(0.82);
    opacity: 0.48;
  }
  64% {
    opacity: 0.16;
  }
  100% {
    transform: scale(1.34);
    opacity: 0;
  }
}

@keyframes healthOrbBreath {
  0%, 100% {
    transform: scale(1);
    opacity: 0.96;
  }
  50% {
    transform: scale(1.04);
    opacity: 1;
  }
}

@keyframes healthCoreBreath {
  0%, 100% {
    transform: scale(0.95);
    opacity: 0.72;
  }
  50% {
    transform: scale(1.04);
    opacity: 1;
  }
}

@keyframes flameFloat {
  0%, 100% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-2px);
  }
}

@keyframes flameRipple {
  0% {
    transform: scale(0.72);
    opacity: 0.58;
  }
  72%, 100% {
    transform: scale(1.34);
    opacity: 0;
  }
}

@keyframes flameFlicker {
  0%, 100% {
    opacity: 0.92;
    transform: scale(1);
  }
  45% {
    opacity: 1;
    transform: scale(1.06) translateY(-1px);
  }
  72% {
    opacity: 0.86;
    transform: scale(0.98);
  }
}

@keyframes flameCore {
  0%, 100% {
    opacity: 0.46;
  }
  50% {
    opacity: 0.74;
  }
}

@media (prefers-reduced-motion: reduce) {
  .orb-pulse-ring,
  .health-orb::before,
  .health-orb::after,
  .flame-badge,
  .flame-badge::before,
  .flame-main,
  .flame-inner {
    animation: none;
  }
}

.health-card-body {
  display: flex;
  min-width: 0;
  flex-direction: column;
  justify-content: space-between;
  gap: 10px;
}

.health-card-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 8px;
  padding-right: 44px;
}

.health-card h4 {
  margin: 0;
  color: var(--text-primary);
  font-size: 16px;
  font-weight: 700;
  line-height: 1.25;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.health-card-head p {
  margin: 4px 0 0;
  color: var(--text-muted);
  font-size: 11px;
  line-height: 1.35;
}

.health-pill {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  height: 24px;
  padding: 0 9px;
  border: 1px solid var(--status-border);
  border-radius: 999px;
  background: var(--status-soft);
  color: var(--status-color);
  font-size: 11px;
  font-weight: 700;
}

.card-meta {
  display: grid;
  gap: 3px;
  color: var(--text-muted);
  font-size: 11px;
}

.card-actions {
  display: flex;
  justify-content: space-between;
  gap: 6px;
}

.card-actions button {
  flex: 1;
  height: 28px;
  border: 1px solid rgba(207, 217, 230, 0.8);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.72);
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
  transition: color 0.16s ease, border-color 0.16s ease, background 0.16s ease;
}

.card-actions button:hover {
  border-color: var(--status-border);
  background: var(--status-soft);
  color: var(--status-color);
}

.pagination-section {
  display: flex;
  flex: 0 0 auto;
  justify-content: flex-end;
  margin: 0;
  padding: 8px 10px 10px;
  border-top: 1px solid var(--service-border-soft);
}

.business-health-page :deep(.el-pagination .btn-prev),
.business-health-page :deep(.el-pagination .btn-next),
.business-health-page :deep(.el-pagination .el-pager li) {
  border-radius: 7px;
  color: var(--service-text-secondary);
  background: #f8fafc;
}

.business-health-page :deep(.el-pagination .el-pager li.is-active) {
  color: #fff;
  background: var(--service-blue-600);
}

.empty-state {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 220px;
}

.business-form :deep(.el-form-item) {
  margin-bottom: 14px;
}

.business-form :deep(.el-form-item__label) {
  height: auto;
  margin-bottom: 6px;
  padding: 0;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.4;
}

.business-form :deep(.el-input__wrapper),
.business-form :deep(.el-select__wrapper) {
  min-height: 34px;
  border-radius: var(--service-control-radius);
  background: var(--service-bg-surface-soft);
  box-shadow: 0 0 0 1px #d9e5f2 inset !important;
}

.rule-dialog-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 10px;
  padding: 8px 10px;
  border: 1px solid var(--service-border-soft);
  border-radius: var(--service-control-radius);
  background: var(--service-bg-subtle);
}

.rule-business-select {
  width: 280px;
}

.rule-dialog-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.rule-dialog-actions :deep(.el-button + .el-button) {
  margin-left: 0;
}

.rule-editor {
  min-height: 472px;
}

.rule-tabs :deep(.el-tabs__header) {
  margin-bottom: 8px;
}

.rule-tabs :deep(.el-tabs__item) {
  font-size: 12px;
  font-weight: 600;
}

.rule-table {
  overflow: hidden;
  border: 1px solid var(--service-border-soft);
  border-radius: var(--service-control-radius);
}

.rule-table :deep(.el-table__header th) {
  height: 34px;
  padding: 5px 0;
  color: #475569;
  background: var(--service-bg-subtle) !important;
  font-size: 12px;
  font-weight: 700;
}

.rule-table :deep(.el-table__cell) {
  padding: 5px 0;
  font-size: 12px;
}

.rule-table :deep(.el-table__row),
.rule-table :deep(.el-table__row:hover) {
  box-shadow: none;
  transform: none;
}

.rule-name-cell {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.rule-name-cell strong {
  overflow: hidden;
  color: var(--service-text-primary);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rule-name-cell span,
.rule-description,
.rule-unit {
  color: var(--service-text-muted);
  font-size: 11px;
  line-height: 1.35;
}

.rule-description {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rule-table :deep(.el-input-number) {
  width: 100%;
}

.rule-weight-select {
  width: 96px;
}

:global(.business-create-dialog-overlay),
:global(.business-rule-dialog-overlay) {
  background: rgba(15, 23, 42, 0.22) !important;
}

:global(.business-create-dialog-overlay .el-dialog),
:global(.business-rule-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.business-create-dialog-overlay .el-dialog) {
  width: min(620px, calc(100vw - 24px)) !important;
}

:global(.business-rule-dialog-overlay .el-dialog) {
  width: min(1120px, calc(100vw - 24px)) !important;
}

:global(.business-create-dialog-overlay .el-dialog__header),
:global(.business-rule-dialog-overlay .el-dialog__header) {
  position: relative;
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: linear-gradient(90deg, #fff6e7 0%, #f7fbff 36%, #ecf7ff 100%);
}

:global(.business-create-dialog-overlay .el-dialog__header::before),
:global(.business-rule-dialog-overlay .el-dialog__header::before) {
  content: "";
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 3px;
  background: linear-gradient(90deg, #f5a524 0%, #62a8ff 48%, #55d6be 100%);
}

:global(.business-create-dialog-overlay .el-dialog__title),
:global(.business-rule-dialog-overlay .el-dialog__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.business-create-dialog-overlay .el-dialog__headerbtn),
:global(.business-rule-dialog-overlay .el-dialog__headerbtn) {
  top: 10px;
  right: 12px;
}

:global(.business-create-dialog-overlay .el-dialog__body),
:global(.business-rule-dialog-overlay .el-dialog__body) {
  max-height: calc(100vh - 190px);
  padding: 16px 18px 4px;
  overflow-y: auto;
}

:global(.business-create-dialog-overlay .el-dialog__footer),
:global(.business-rule-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #fff;
}

:global(.business-create-dialog-overlay .el-dialog__footer .el-button),
:global(.business-rule-dialog-overlay .el-dialog__footer .el-button) {
  min-width: 78px;
  min-height: 32px;
  border-radius: 8px;
  font-weight: 600;
}

:global(.business-create-dialog-overlay .el-dialog__footer .el-button--primary),
:global(.business-rule-dialog-overlay .el-dialog__footer .el-button--primary) {
  border-color: #409eff;
  color: #fff;
  background: linear-gradient(135deg, #5aa8ff, #2563eb);
}

@media (max-width: 1100px) {
  .toolbar-form {
    align-items: flex-start;
    flex-wrap: wrap;
  }
}

@media (max-width: 1280px) {
  .business-card-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .business-health-page {
    grid-template-rows: auto auto;
    height: auto;
    padding: 6px;
    overflow: visible;
  }

  .service-hero-content {
    align-items: flex-start;
    flex-direction: column;
    gap: 3px;
  }

  .service-hero-divider {
    display: none;
  }

  .service-hero-content p {
    white-space: normal;
  }

  .page-card {
    overflow: visible;
  }

  .overview-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    padding: 0;
  }

  .overview-card {
    padding: 8px 10px;
    border-bottom: 1px solid var(--service-border-soft);
  }

  .overview-card:nth-child(even) {
    border-right: 0;
  }

  .overview-card:last-child {
    grid-column: 1 / -1;
    border-bottom: 0;
  }

  .toolbar-form,
  .toolbar-actions,
  .toolbar-form :deep(.el-form-item),
  .toolbar-form :deep(.el-input),
  .status-filter-item :deep(.el-select) {
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

  .business-card-grid {
    grid-template-columns: 1fr;
  }

  .rule-dialog-toolbar,
  .rule-dialog-actions {
    align-items: stretch;
    flex-direction: column;
  }

  .rule-business-select {
    width: 100%;
  }

  .pagination-section {
    overflow-x: auto;
  }
}
</style>
