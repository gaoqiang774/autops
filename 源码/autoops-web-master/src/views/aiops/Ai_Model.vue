<template>
  <div class="ai-model-page">
    <header class="ops-page-header">
      <div class="ops-page-header__identity">
        <span class="ops-page-header__icon" aria-hidden="true">
          <img :src="aiModelSvg" alt="">
        </span>
        <div class="ops-page-header__content">
          <h1>AI 模型配置</h1>
          <span class="ops-page-header__divider" aria-hidden="true"></span>
          <p>统一管理 LLM Provider 的接入、模型和默认配置。</p>
        </div>
      </div>
      <div class="summary-group">
        <div class="summary-chip">
          <span>配置总数</span>
          <strong>{{ total }}</strong>
        </div>
        <div class="summary-chip">
          <span>本页启用</span>
          <strong>{{ enabledCount }}</strong>
        </div>
        <div class="summary-chip summary-chip-wide" :title="defaultProviderName">
          <span>本页默认</span>
          <strong>{{ defaultProviderName }}</strong>
        </div>
      </div>
    </header>

    <div class="content-stack">
      <section class="toolbar-card">
        <el-form :model="queryParams" :inline="true" class="toolbar-form" @submit.prevent>
          <el-form-item label="关键字">
            <el-input
              v-model="queryParams.keyword"
              clearable
              placeholder="名称 / 模型"
              @keyup.enter="handleQuery"
              @clear="handleQuery"
            >
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>
          </el-form-item>
          <el-form-item class="toolbar-actions-item">
            <div class="toolbar-actions">
              <el-button type="primary" :icon="Search" @click="handleQuery">查询</el-button>
              <el-button type="warning" :icon="RefreshLeft" @click="resetQuery">重置</el-button>
              <el-button class="ops-refresh-button" type="warning" :icon="Refresh" :loading="loading" @click="fetchList">
                刷新
              </el-button>
              <el-button type="success" :icon="Plus" @click="openCreateDialog">新增配置</el-button>
            </div>
          </el-form-item>
        </el-form>
      </section>

      <section class="table-card">
        <div class="table-head">
          <h2>Provider 列表</h2>
          <span>共 {{ total }} 项</span>
        </div>

        <el-table v-loading="loading" :data="tableData" height="100%" stripe class="model-table">
          <el-table-column label="配置名称" min-width="190" fixed="left">
            <template #default="{ row }">
              <div class="provider-name-cell">
                <div class="provider-mark">
                  <el-icon><Connection /></el-icon>
                </div>
                <div class="provider-name-meta">
                  <strong>{{ getRowModelName(row) }}</strong>
                  <span>ID: {{ row.id || '-' }}</span>
                </div>
              </div>
            </template>
          </el-table-column>

          <el-table-column label="类型" min-width="150">
            <template #default="{ row }">
              <el-tag effect="light" class="type-tag">
                {{ getModelTypeLabel(getRowModelType(row)) }}
              </el-tag>
            </template>
          </el-table-column>

          <el-table-column label="模型编号" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">
              <span class="mono-text">{{ getRowModelCode(row) }}</span>
            </template>
          </el-table-column>

          <el-table-column label="接入地址" min-width="250" show-overflow-tooltip>
            <template #default="{ row }">
              <div class="endpoint-cell">
                <span>{{ getRowApiUrl(row) }}</span>
                <small>{{ row.protocol || getPresetValue(getRowModelType(row), 'protocol') || 'backend default' }}</small>
              </div>
            </template>
          </el-table-column>

          <el-table-column label="密钥状态" width="126" align="center">
            <template #default="{ row }">
              <div class="secret-cell">
                <el-tag :type="isApiKeySet(row) ? 'success' : 'info'" size="small" effect="light">
                  {{ isApiKeySet(row) ? '已配置' : '未配置' }}
                </el-tag>
                <span v-if="getApiKeyPreview(row)" class="secret-preview">{{ getApiKeyPreview(row) }}</span>
              </div>
            </template>
          </el-table-column>

          <el-table-column label="状态" width="88" align="center">
            <template #default="{ row }">
              <span class="status-pill" :class="{ disabled: Number(row.status) !== 1 }">
                <i aria-hidden="true"></i>
                {{ Number(row.status) === 1 ? '启用' : '禁用' }}
              </span>
            </template>
          </el-table-column>

          <el-table-column label="默认" width="78" align="center">
            <template #default="{ row }">
              <el-tag v-if="isDefaultProvider(row)" type="success" size="small" effect="light">默认</el-tag>
              <span v-else class="muted-text">-</span>
            </template>
          </el-table-column>

          <el-table-column label="更新时间" min-width="160" show-overflow-tooltip>
            <template #default="{ row }">
              {{ row.updateTime || row.updatedAt || row.updated_at || row.createTime || '-' }}
            </template>
          </el-table-column>

          <el-table-column label="操作" width="166" align="right" fixed="right">
            <template #default="{ row }">
              <div class="operation-links">
                <el-button
                  link
                  type="success"
                  :loading="testingId === row.id"
                  @click="handleTestSaved(row)"
                >
                  测试
                </el-button>
                <el-button link type="primary" @click="openEditDialog(row)">编辑</el-button>
                <el-dropdown trigger="click" @command="command => handleRowCommand(command, row)">
                  <span class="more-action-trigger">
                    <el-tooltip content="更多操作" placement="top">
                      <el-button class="more-action-button" link :icon="MoreFilled" aria-label="更多操作" />
                    </el-tooltip>
                  </span>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="secret">查看密钥</el-dropdown-item>
                      <el-dropdown-item v-if="!isDefaultProvider(row)" command="default">设为默认</el-dropdown-item>
                      <el-dropdown-item command="delete" divided class="danger-menu-item">删除配置</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>
            </template>
          </el-table-column>

          <template #empty>
            <div class="table-empty">
              <span>暂无模型配置</span>
              <el-button type="success" :icon="Plus" @click="openCreateDialog">新增配置</el-button>
            </div>
          </template>
        </el-table>

        <div class="pagination-section">
          <el-pagination
            v-model:current-page="queryParams.page"
            v-model:page-size="queryParams.pageSize"
            :page-sizes="[10, 20, 50, 100]"
            :total="total"
            layout="total, sizes, prev, pager, next, jumper"
            @size-change="handleSizeChange"
            @current-change="fetchList"
          />
        </div>
      </section>
    </div>

    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="760px"
      class="model-dialog"
      modal-class="model-form-dialog-overlay"
      @closed="handleDialogClosed"
    >
      <el-form
        ref="formRef"
        :model="formData"
        :rules="formRules"
        label-position="top"
        class="provider-form-panel"
      >
        <div class="form-section-head">
          <h4>基础配置</h4>
          <p>选择 Provider 类型后填写凭据和默认模型，高级协议参数使用后端预设。</p>
        </div>

            <el-row :gutter="16">
              <el-col :xs="24" :md="12">
                <el-form-item label="模型名称" prop="modelName">
                  <el-input v-model.trim="formData.modelName" clearable placeholder="例如 DeepSeek 默认模型" />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :md="12">
                <el-form-item label="模型类型" prop="modelType">
                  <el-select
                    v-model="formData.modelType"
                    filterable
                    placeholder="请选择模型类型"
                    style="width: 100%"
                    @change="handleModelTypeChange"
                  >
                    <el-option
                      v-for="item in providerTypeOptions"
                      :key="item.value"
                      :label="item.label"
                      :value="item.value"
                    >
                      <div class="provider-type-option">
                        <span>{{ item.label }}</span>
                        <small>{{ item.value }}</small>
                      </div>
                    </el-option>
                  </el-select>
                </el-form-item>
              </el-col>
            </el-row>

            <div v-if="currentPreset" class="preset-panel">
              <div>
                <strong>{{ currentPreset.label }}</strong>
                <span>{{ currentPreset.description || '已加载后端模型厂商预设，未填写的协议和路径由后端补齐。' }}</span>
              </div>
              <div class="preset-meta">
                <span>{{ currentPreset.protocol || 'backend default' }}</span>
                <span>{{ currentPreset.authType || 'auth default' }}</span>
                <span v-if="currentPreset.baseUrl">{{ currentPreset.baseUrl }}</span>
              </div>
            </div>

            <el-row :gutter="16">
              <el-col :xs="24" :md="12">
                <el-form-item label="API Key" prop="api_key" :required="apiKeyRequired">
                  <el-input
                    v-model="formData.api_key"
                    show-password
                    clearable
                    :placeholder="isEdit ? '留空表示不修改密钥' : '请输入 API Key'"
                  />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :md="12">
                <el-form-item label="模型编号" prop="modelCode">
                  <el-select
                    v-model="formData.modelCode"
                    allow-create
                    filterable
                    default-first-option
                    clearable
                    placeholder="例如 deepseek-chat"
                    style="width: 100%"
                  >
                    <el-option
                      v-for="model in modelOptions"
                      :key="model"
                      :label="model"
                      :value="model"
                    />
                  </el-select>
                </el-form-item>
              </el-col>
            </el-row>

            <el-form-item label="API 地址" prop="apiUrl">
              <el-input
                v-model.trim="formData.apiUrl"
                clearable
                placeholder="openai_compatible 必填，其他类型可留空使用默认地址"
              />
            </el-form-item>

            <el-row :gutter="16">
              <el-col :xs="24" :md="8">
                <el-form-item label="请求超时（秒）" prop="timeoutSeconds">
                  <el-input-number
                    v-model="formData.timeoutSeconds"
                    :min="1"
                    :max="600"
                    controls-position="right"
                    style="width: 100%"
                  />
                </el-form-item>
              </el-col>
              <el-col :xs="24" :md="8">
                <el-form-item label="状态" prop="status">
                  <el-radio-group v-model="formData.status">
                    <el-radio :label="1">启用</el-radio>
                    <el-radio :label="2">禁用</el-radio>
                  </el-radio-group>
                </el-form-item>
              </el-col>
              <el-col :xs="24" :md="8">
                <el-form-item label="默认配置">
                  <el-switch
                    v-model="formData.isDefault"
                    active-text="设为默认"
                    inactive-text="普通配置"
                  />
                </el-form-item>
              </el-col>
            </el-row>

          </el-form>

          <template #footer>
            <div class="dialog-footer">
              <el-button :loading="testingForm" @click="handleTestForm">
                连通性测试
              </el-button>
              <el-button @click="dialogVisible = false">取消</el-button>
              <el-button type="primary" :loading="submitLoading" @click="submitForm">
                {{ isEdit ? '保存修改' : '创建配置' }}
              </el-button>
            </div>
          </template>
        </el-dialog>

        <el-dialog
          v-model="secretDialogVisible"
          title="查看 Provider 密钥"
          width="620px"
          class="model-dialog"
          modal-class="model-secret-dialog-overlay"
          @closed="resetSecretDialog"
        >
          <div v-loading="secretLoading" class="secret-dialog-body">
            <div class="secret-meta">
              <span>配置名称</span>
              <strong>{{ secretTarget ? getRowModelName(secretTarget) : '-' }}</strong>
            </div>

            <div v-if="secretItems.length" class="secret-list">
              <div v-for="item in secretItems" :key="item.label" class="secret-row">
                <span>{{ item.label }}</span>
                <div class="secret-value">
                  <code>{{ item.value }}</code>
                  <el-tooltip :content="`复制 ${item.label}`" placement="top">
                    <el-button
                      class="secret-copy-button"
                      :icon="CopyDocument"
                      aria-label="复制密钥"
                      @click="copyText(item.value, item.label)"
                    />
                  </el-tooltip>
                </div>
              </div>
            </div>
            <el-empty v-else-if="!secretLoading" description="接口未返回可展示的密钥字段" :image-size="80" />
          </div>

          <template #footer>
            <div class="dialog-footer">
              <el-button type="primary" @click="secretDialogVisible = false">关闭</el-button>
            </div>
          </template>
        </el-dialog>

        <el-dialog
          v-model="testDialogVisible"
          title="模型连通性测试"
          width="640px"
          class="model-dialog"
          modal-class="model-test-dialog-overlay"
        >
          <div class="test-dialog-body">
            <div class="test-result-card" :class="{ failed: !testResult.success }">
              <div class="test-result-icon">
                <el-icon><CircleCheck v-if="testResult.success" /><CircleClose v-else /></el-icon>
              </div>
              <div>
                <strong>{{ testResult.success ? '测试通过' : '测试失败' }}</strong>
                <span>{{ testResult.message }}</span>
              </div>
            </div>

            <div v-if="testResult.reply" class="test-reply">
              <span>模型响应</span>
              <pre>{{ testResult.reply }}</pre>
            </div>
          </div>

          <template #footer>
            <div class="dialog-footer">
              <el-button type="primary" @click="testDialogVisible = false">关闭</el-button>
            </div>
          </template>
        </el-dialog>
  </div>
</template>

<script setup>
import { computed, getCurrentInstance, nextTick, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  CircleCheck,
  CircleClose,
  Connection,
  CopyDocument,
  MoreFilled,
  Plus,
  Refresh,
  RefreshLeft,
  Search
} from '@element-plus/icons-vue'
import aiModelSvg from '@/assets/image/智能ai.svg'

const { proxy } = getCurrentInstance()

const loading = ref(false)
const submitLoading = ref(false)
const providerTypes = ref([])
const tableData = ref([])
const total = ref(0)
const dialogVisible = ref(false)
const secretDialogVisible = ref(false)
const secretLoading = ref(false)
const secretTarget = ref(null)
const secretData = ref({})
const testDialogVisible = ref(false)
const testResult = reactive({
  success: false,
  message: '',
  reply: ''
})
const testingId = ref(null)
const testingForm = ref(false)
const formRef = ref()
const isEdit = ref(false)

const queryParams = reactive({
  keyword: '',
  page: 1,
  pageSize: 10
})

const createFormData = () => ({
  id: '',
  modelName: '',
  modelType: '',
  modelCode: '',
  apiUrl: '',
  api_key: '',
  timeoutSeconds: 180,
  status: 1,
  isDefault: false
})

const formData = reactive(createFormData())

const apiKeyRequired = computed(() => {
  if (isEdit.value) return false
  const preset = providerTypes.value.find(item => item.value === formData.modelType)
  return preset?.needApiKey !== false
})

const formRules = {
  modelName: [{ required: true, message: '请输入模型名称', trigger: 'blur' }],
  modelType: [{ required: true, message: '请选择模型类型', trigger: 'change' }],
  modelCode: [{ required: true, message: '请输入模型编号', trigger: 'change' }],
  api_key: [
    {
      validator: (_, value, callback) => {
        if (apiKeyRequired.value && !value) {
          callback(new Error('请输入 API Key'))
          return
        }
        callback()
      },
      trigger: 'blur'
    }
  ],
  apiUrl: [
    {
      validator: (_, value, callback) => {
        if (formData.modelType === 'openai_compatible' && !value) {
          callback(new Error('openai_compatible 类型需要填写 API 地址'))
          return
        }
        callback()
      },
      trigger: 'blur'
    }
  ]
}

const providerTypeOptions = computed(() => providerTypes.value.map(item => ({
  label: item.label,
  value: item.value
})))

const currentPreset = computed(() => providerTypes.value.find(item => item.value === formData.modelType))

const modelOptions = computed(() => {
  const presetModels = currentPreset.value?.models || []
  return Array.from(new Set([...presetModels, formData.modelCode].filter(Boolean)))
})

const enabledCount = computed(() => tableData.value.filter(item => Number(item.status) === 1).length)

const defaultProviderName = computed(() => {
  const item = tableData.value.find(row => isDefaultProvider(row))
  return item ? getRowModelName(item) : '-'
})

const dialogTitle = computed(() => (isEdit.value ? '修改模型配置' : '新增模型配置'))

const secretItems = computed(() => {
  const data = secretData.value || {}
  const items = []
  const keyValue = data.api_key || data.apiKey || data.key
  const secretValue = data.apiSecret || data.api_secret || data.secret
  if (keyValue) items.push({ label: 'API Key', value: keyValue })
  if (secretValue) items.push({ label: 'API Secret', value: secretValue })
  return items
})

const successCodes = new Set([0, 200])

const getBody = response => response?.data || response || {}

const assertSuccess = (response, fallbackMessage) => {
  const body = getBody(response)
  if (body.code !== undefined && !successCodes.has(Number(body.code))) {
    throw new Error(body.message || body.msg || fallbackMessage)
  }
  return body
}

const getPayload = response => {
  const body = assertSuccess(response, '请求失败')
  return body.data !== undefined ? body.data : body
}

const getResponseMessage = response => {
  const body = getBody(response)
  return body.message || body.msg || ''
}

const normalizeProviderTypes = data => {
  const raw = data?.types || data?.list || data?.items || data || []
  const list = Array.isArray(raw)
    ? raw
    : Object.keys(raw).map(key => ({ value: key, ...(raw[key] || {}) }))

  return list.map(item => {
    const value = item.modelType || item.providerType || item.type || item.value || item.key || item.name
    const models = item.models || item.defaultModels || item.modelList || []
    return {
      ...item,
      value,
      label: item.label || item.displayName || item.title || item.name || value,
      protocol: item.protocol || item.defaultProtocol || '',
      baseUrl: item.baseUrl || item.baseURL || item.defaultBaseUrl || item.defaultBaseURL || '',
      authType: item.authType || item.defaultAuthType || '',
      defaultModel: item.defaultModel || item.modelCode || item.model || '',
      models: Array.isArray(models) ? models : String(models || '').split(',').map(model => model.trim()).filter(Boolean),
      description: item.description || item.remark || ''
    }
  }).filter(item => item.value)
}

const normalizeList = payload => {
  const listSource = Array.isArray(payload)
    ? payload
    : payload?.list || payload?.records || payload?.items || payload?.rows || []
  const list = Array.isArray(listSource) ? listSource : []
  const count = payload?.total ?? payload?.count ?? payload?.totalCount ?? list.length
  return { list, count }
}

const getModels = models => {
  if (Array.isArray(models)) return models.filter(Boolean)
  if (!models) return []
  if (typeof models === 'string') {
    try {
      const parsed = JSON.parse(models)
      if (Array.isArray(parsed)) return parsed.filter(Boolean)
    } catch (error) {
      return models.split(',').map(item => item.trim()).filter(Boolean)
    }
  }
  return []
}

const getRowModelName = row => row.modelName || row.name || '-'

const getRowModelType = row => row.modelType || row.providerType || ''

const getRowModelCode = row => row.modelCode || row.defaultModel || getModels(row.models)[0] || '-'

const getRowApiUrl = row => row.apiUrl || row.baseUrl || getPresetValue(getRowModelType(row), 'baseUrl') || '-'

const isApiKeySet = row => row.apiKeySet || row.api_key_set || row.apiKeyPreview || row.api_key_preview

const getApiKeyPreview = row => row.apiKeyPreview || row.api_key_preview || ''

const getModelTypeLabel = type => providerTypes.value.find(item => item.value === type)?.label || type || '-'

const getPresetValue = (type, key) => providerTypes.value.find(item => item.value === type)?.[key] || ''

const isDefaultProvider = row => row.isDefault === true || row.isDefault === 1 || row.isDefault === '1'

const resetFormData = () => {
  Object.assign(formData, createFormData())
  nextTick(() => formRef.value?.clearValidate())
}

const fetchProviderTypes = async () => {
  try {
    const response = await proxy.$api.getLLMProviderTypes()
    providerTypes.value = normalizeProviderTypes(getPayload(response))
  } catch (error) {
    console.error('获取模型类型失败:', error)
    ElMessage.error(error.message || '获取模型类型失败')
  }
}

const fetchList = async () => {
  loading.value = true
  try {
    const params = {
      page: queryParams.page,
      pageSize: queryParams.pageSize
    }
    if (queryParams.keyword) params.keyword = queryParams.keyword

    const response = await proxy.$api.getLLMProviderList(params)
    const { list, count } = normalizeList(getPayload(response))
    tableData.value = list
    total.value = count
  } catch (error) {
    console.error('获取模型配置列表失败:', error)
    ElMessage.error(error.message || '获取模型配置列表失败')
  } finally {
    loading.value = false
  }
}

const handleQuery = () => {
  queryParams.page = 1
  fetchList()
}

const resetQuery = () => {
  queryParams.keyword = ''
  queryParams.page = 1
  fetchList()
}

const handleSizeChange = size => {
  queryParams.pageSize = size
  queryParams.page = 1
  fetchList()
}

const handleModelTypeChange = type => {
  const preset = providerTypes.value.find(item => item.value === type)
  if (!preset) return

  formData.modelCode = formData.modelCode || preset.defaultModel || preset.models?.[0] || ''
}

const openCreateDialog = () => {
  isEdit.value = false
  resetFormData()
  dialogVisible.value = true
}

const fillForm = data => {
  const normalized = data || {}
  Object.assign(formData, createFormData(), {
    id: normalized.id || '',
    modelName: normalized.modelName || normalized.name || '',
    modelType: normalized.modelType || normalized.providerType || '',
    modelCode: normalized.modelCode || normalized.defaultModel || getModels(normalized.models)[0] || '',
    apiUrl: normalized.apiUrl || normalized.baseUrl || '',
    timeoutSeconds: normalized.timeoutSeconds ?? normalized.timeout_seconds ?? 60,
    status: normalized.status ?? 1,
    isDefault: isDefaultProvider(normalized)
  })
}

const openEditDialog = async row => {
  isEdit.value = true
  resetFormData()
  dialogVisible.value = true

  try {
    const response = await proxy.$api.getLLMProviderById(row.id)
    fillForm(getPayload(response))
  } catch (error) {
    console.error('获取模型配置详情失败，使用列表数据回填:', error)
    fillForm(row)
  }
}

const buildPayload = () => {
  const payload = {
    modelName: formData.modelName,
    modelType: formData.modelType,
    modelCode: formData.modelCode,
    timeoutSeconds: Number(formData.timeoutSeconds || 180),
    status: formData.status
  }

  if (isEdit.value) payload.id = formData.id
  if (formData.apiUrl) payload.apiUrl = formData.apiUrl
  if (formData.api_key) payload.api_key = formData.api_key
  if (formData.isDefault) payload.isDefault = true

  return payload
}

const buildTestPayload = () => ({
  ...buildPayload(),
  message: '请只回复 OK'
})

const normalizeTestResult = response => {
  const body = assertSuccess(response, '模型连通性测试失败')
  const data = body.data || body
  const success = data.success ?? data.ok ?? true
  return {
    success: success !== false,
    message: data.message || data.msg || body.message || body.msg || '模型连通性测试完成',
    reply: data.reply || data.response || data.content || data.output || ''
  }
}

const showTestResult = result => {
  testResult.success = result.success
  testResult.message = result.message
  testResult.reply = result.reply
  testDialogVisible.value = true
}

const runProviderTest = async payload => {
  try {
    const response = await proxy.$api.testLLMProvider(payload)
    const result = normalizeTestResult(response)
    showTestResult(result)
    if (result.success) {
      ElMessage.success('模型连通性测试通过')
    } else {
      ElMessage.error(result.message || '模型连通性测试失败')
    }
  } catch (error) {
    showTestResult({
      success: false,
      message: error.message || '模型连通性测试失败',
      reply: ''
    })
  }
}

const handleTestSaved = async row => {
  if (!row.id) {
    ElMessage.warning('缺少配置 ID，无法测试')
    return
  }

  testingId.value = row.id
  try {
    await runProviderTest({ id: row.id })
  } finally {
    testingId.value = null
  }
}

const handleTestForm = async () => {
  try {
    await formRef.value?.validate()

    testingForm.value = true
    await runProviderTest(formData.id && !formData.api_key ? { id: formData.id } : buildTestPayload())
  } catch (error) {
    if (error?.message) {
      ElMessage.error(error.message)
    }
  } finally {
    testingForm.value = false
  }
}

const submitForm = async () => {
  try {
    await formRef.value?.validate()

    submitLoading.value = true
    const payload = buildPayload()
    const response = isEdit.value
      ? await proxy.$api.updateLLMProvider(payload)
      : await proxy.$api.createLLMProvider(payload)

    assertSuccess(response, isEdit.value ? '更新失败' : '创建失败')
    ElMessage.success(isEdit.value ? '更新成功' : '创建成功')
    dialogVisible.value = false
    fetchList()
  } catch (error) {
    if (error?.message) {
      ElMessage.error(error.message || getResponseMessage(error))
    }
  } finally {
    submitLoading.value = false
  }
}

const handleDialogClosed = () => {
  submitLoading.value = false
  resetFormData()
}

const handleRowCommand = (command, row) => {
  if (command === 'secret') {
    openSecretDialog(row)
    return
  }
  if (command === 'default') {
    handleSetDefault(row)
    return
  }
  if (command === 'delete') {
    handleDelete(row)
  }
}

const handleSetDefault = async row => {
  try {
    await ElMessageBox.confirm(`确定将「${getRowModelName(row)}」设为默认模型配置吗？`, '设置默认配置', {
      type: 'warning',
      confirmButtonText: '确定',
      cancelButtonText: '取消'
    })
    const response = await proxy.$api.setDefaultLLMProvider({ id: row.id })
    assertSuccess(response, '设置默认配置失败')
    ElMessage.success('默认配置已更新')
    fetchList()
  } catch (error) {
    if (error !== 'cancel' && error?.message) {
      ElMessage.error(error.message)
    }
  }
}

const handleDelete = async row => {
  try {
    await ElMessageBox.confirm(`确定删除模型配置「${getRowModelName(row)}」吗？`, '删除确认', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      confirmButtonClass: 'el-button--danger'
    })
    const response = await proxy.$api.deleteLLMProvider(row.id)
    assertSuccess(response, '删除失败')
    ElMessage.success('删除成功')
    fetchList()
  } catch (error) {
    if (error !== 'cancel' && error?.message) {
      ElMessage.error(error.message)
    }
  }
}

const openSecretDialog = async row => {
  secretTarget.value = row
  secretData.value = {}
  secretDialogVisible.value = true
  secretLoading.value = true
  try {
    const response = await proxy.$api.getLLMProviderSecret(row.id)
    secretData.value = getPayload(response) || {}
  } catch (error) {
    console.error('解密密钥失败:', error)
    ElMessage.error(error.message || '解密密钥失败')
  } finally {
    secretLoading.value = false
  }
}

const resetSecretDialog = () => {
  secretTarget.value = null
  secretData.value = {}
  secretLoading.value = false
}

const copyText = async (text, label) => {
  const value = String(text ?? '')
  try {
    if (navigator.clipboard?.writeText && window.isSecureContext) {
      await navigator.clipboard.writeText(value)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = value
      textarea.setAttribute('readonly', '')
      textarea.style.position = 'fixed'
      textarea.style.top = '-9999px'
      textarea.style.left = '-9999px'
      document.body.appendChild(textarea)
      textarea.select()
      const copied = document.execCommand('copy')
      document.body.removeChild(textarea)
      if (!copied) {
        throw new Error('document.execCommand copy failed')
      }
    }
    ElMessage.success(`${label} 已复制`)
  } catch (error) {
    console.error('复制失败:', error)
    ElMessage.error('复制失败，请手动复制')
  }
}

onMounted(async () => {
  await fetchProviderTypes()
  fetchList()
})
</script>

<style scoped>
:global(.home-container .el-main:has(.ai-model-page)) {
  padding-top: var(--ops-shell-gutter, 10px);
  padding-right: var(--ops-shell-gutter, 10px);
  padding-left: var(--ops-shell-gutter, 10px);
}

.ai-model-page {
  --model-heading: var(--ops-text-heading, #0f172a);
  --model-text: var(--ops-text-primary, #172033);
  --model-secondary: var(--ops-text-secondary, #53657d);
  --model-muted: var(--ops-text-muted, #64748b);
  --model-primary: var(--ops-color-primary, #2563eb);
  --model-success: var(--ops-color-success, #10b981);
  --model-warning: var(--ops-color-warning, #f59e0b);
  --model-danger: var(--ops-color-danger, #ef4444);
  --model-surface: var(--ops-bg-surface, #ffffff);
  --model-surface-soft: var(--ops-bg-surface-soft, #fbfdff);
  --model-subtle: var(--ops-bg-subtle, #f3f7fc);
  --model-border: var(--ops-border, #dbe7f4);
  --model-border-soft: var(--ops-border-soft, #e7eef7);
  box-sizing: border-box;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: var(--ops-module-gap, 8px);
  height: calc(100vh - 124px);
  min-height: 0;
  padding: var(--ops-page-padding, 7px);
  overflow: hidden;
  color: var(--model-text);
  background: transparent;
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
  letter-spacing: 0;
}

.ops-page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
  padding: 8px 12px;
  border: 1px solid var(--model-border);
  border-radius: var(--ops-card-radius, 12px);
  background: rgba(255, 255, 255, 0.96);
  box-shadow: var(--ops-shadow-hero, 0 10px 28px rgba(80, 124, 180, 0.10));
}

.ops-page-header__identity,
.ops-page-header__content,
.summary-group {
  display: flex;
  align-items: center;
  min-width: 0;
}

.ops-page-header__identity {
  flex: 1 1 auto;
  gap: 8px;
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
  flex: 1 1 auto;
  gap: 10px;
}

.ops-page-header h1 {
  flex: 0 0 auto;
  margin: 0;
  color: var(--model-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
}

.ops-page-header__divider {
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  background: var(--model-border-soft);
}

.ops-page-header p {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  color: var(--model-muted);
  font-size: 12px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.summary-group {
  flex: 0 0 auto;
  justify-content: flex-end;
  gap: 6px;
}

.summary-chip {
  display: grid;
  grid-template-columns: auto auto;
  align-items: baseline;
  gap: 6px;
  min-width: 84px;
  height: 32px;
  padding: 0 9px;
  border: 1px solid var(--model-border-soft);
  border-radius: 8px;
  background: var(--model-subtle);
}

.summary-chip-wide {
  grid-template-columns: auto minmax(0, 1fr);
  width: 170px;
}

.summary-chip span {
  color: var(--model-muted);
  font-size: 10.5px;
  white-space: nowrap;
}

.summary-chip strong {
  overflow: hidden;
  color: var(--model-heading);
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 14px;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.content-stack {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: var(--ops-module-gap, 8px);
  min-width: 0;
  min-height: 0;
}

.toolbar-card,
.table-card {
  min-width: 0;
  border: 1px solid var(--model-border);
  border-radius: var(--ops-card-radius, 12px);
  background: rgba(255, 255, 255, 0.98);
  box-shadow: var(--ops-shadow-card, 0 8px 24px rgba(80, 124, 180, 0.07));
}

.toolbar-card {
  padding: 7px 10px;
}

.toolbar-form {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px 12px;
  min-width: 0;
}

.toolbar-form :deep(.el-form-item) {
  margin: 0;
}

.toolbar-form :deep(.el-form-item:first-child) {
  flex: 1 1 280px;
  max-width: 420px;
}

.toolbar-form :deep(.el-form-item__label) {
  height: 34px;
  padding-right: 8px;
  color: #334155;
  font-size: 12px;
  font-weight: 700;
  line-height: 34px;
}

.toolbar-form :deep(.el-input) {
  width: 100%;
}

.toolbar-actions-item {
  margin-left: auto !important;
}

.toolbar-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

.toolbar-actions :deep(.el-button) {
  height: 30px;
  margin-left: 0;
  padding: 0 10px;
  font-size: 12px;
}

.toolbar-actions :deep(.el-button .el-icon) {
  font-size: 14px;
}

.table-card {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  min-height: 0;
  overflow: hidden;
}

.table-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--model-border-soft);
}

.table-head h2 {
  margin: 0;
  color: var(--model-heading);
  font-size: 15px;
  font-weight: 700;
  line-height: 1.25;
}

.table-head > span {
  color: var(--model-muted);
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.provider-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.provider-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  flex: 0 0 30px;
  border-radius: 8px;
  color: var(--model-primary);
  background: #eef6ff;
}

.provider-name-meta {
  min-width: 0;
}

.provider-name-meta strong,
.provider-name-meta span {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.provider-name-meta strong {
  color: var(--model-heading);
  font-size: 12px;
  font-weight: 700;
}

.provider-name-meta span {
  margin-top: 2px;
  color: var(--model-muted);
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 10.5px;
  font-variant-numeric: tabular-nums;
}

.type-tag {
  max-width: 100%;
  color: #2459a6;
  background: #eef6ff;
}

.type-tag :deep(.el-tag__content) {
  overflow: hidden;
  text-overflow: ellipsis;
}

.mono-text,
.secret-preview {
  font-family: Consolas, "SFMono-Regular", monospace;
  font-variant-numeric: tabular-nums;
}

.mono-text {
  color: var(--model-secondary);
}

.muted-text {
  color: var(--model-muted);
  font-size: 12px;
}

.endpoint-cell {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.endpoint-cell span,
.endpoint-cell small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.endpoint-cell span {
  color: var(--model-secondary);
}

.endpoint-cell small {
  color: var(--model-muted);
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 10.5px;
}

.secret-cell {
  display: grid;
  justify-items: center;
  gap: 2px;
}

.secret-preview {
  overflow: hidden;
  max-width: 104px;
  color: var(--model-muted);
  font-size: 10.5px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: #059669;
  font-size: 11px;
  font-weight: 700;
}

.status-pill i {
  width: 7px;
  height: 7px;
  flex: 0 0 7px;
  border-radius: 50%;
  background: currentColor;
}

.status-pill.disabled {
  color: #64748b;
}

.operation-links {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  width: 100%;
  white-space: nowrap;
}

.operation-links :deep(.el-button) {
  margin: 0;
  padding: 0 2px;
  font-size: 12px;
}

.more-action-button {
  width: 26px;
  height: 26px;
  padding: 0 !important;
  color: var(--model-secondary);
}

.more-action-trigger {
  display: inline-flex;
  align-items: center;
}

.table-empty {
  display: grid;
  justify-items: center;
  gap: 8px;
  padding: 18px 0;
  color: var(--model-muted);
  font-size: 12px;
}

.pagination-section {
  display: flex;
  justify-content: flex-end;
  padding: 9px 10px;
  overflow-x: auto;
  border-top: 1px solid var(--model-border-soft);
}

.provider-form-panel {
  min-width: 0;
}

.form-section-head {
  margin-bottom: 12px;
  padding-bottom: 9px;
  border-bottom: 1px solid var(--model-border-soft);
}

.form-section-head h4 {
  margin: 0;
  color: var(--model-heading);
  font-size: 15px;
  font-weight: 700;
}

.form-section-head p {
  margin: 4px 0 0;
  color: var(--model-muted);
  font-size: 11px;
  line-height: 1.5;
}

.provider-type-option {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.provider-type-option small {
  color: var(--model-muted);
}

.preset-panel {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(180px, auto);
  align-items: start;
  gap: 12px;
  margin: 0 0 12px;
  padding: 10px 12px;
  border: 1px solid #cfe2ff;
  border-radius: 10px;
  background: #f6faff;
}

.preset-panel strong,
.preset-panel span {
  display: block;
}

.preset-panel strong {
  color: var(--model-heading);
  font-size: 12px;
}

.preset-panel span {
  margin-top: 3px;
  color: var(--model-secondary);
  font-size: 11px;
  line-height: 1.5;
}

.preset-meta {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 5px;
  min-width: 0;
}

.preset-meta span {
  max-width: 100%;
  margin: 0;
  overflow: hidden;
  padding: 3px 7px;
  border-radius: 999px;
  background: #e7f1ff;
  color: #2459a6;
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 10.5px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.dialog-footer :deep(.el-button + .el-button) {
  margin-left: 0;
}

.secret-dialog-body,
.test-dialog-body {
  display: grid;
  gap: 10px;
}

.secret-meta {
  display: grid;
  grid-template-columns: 84px minmax(0, 1fr);
  align-items: center;
  gap: 8px;
  padding: 9px 10px;
  border: 1px solid var(--model-border-soft);
  border-radius: 8px;
  background: var(--model-subtle);
}

.secret-meta span {
  color: var(--model-muted);
  font-size: 11px;
}

.secret-meta strong {
  overflow: hidden;
  color: var(--model-heading);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.secret-list {
  display: grid;
  gap: 8px;
}

.secret-row {
  display: grid;
  grid-template-columns: 84px minmax(0, 1fr);
  align-items: center;
  gap: 8px;
}

.secret-row > span {
  color: var(--model-text);
  font-size: 12px;
  font-weight: 700;
}

.secret-value {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 30px;
  align-items: center;
  gap: 6px;
  min-width: 0;
  padding: 5px 5px 5px 9px;
  border: 1px solid #d9e5f2;
  border-radius: 8px;
  background: var(--model-surface-soft);
}

.secret-value code {
  overflow: hidden;
  color: var(--model-secondary);
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.secret-copy-button {
  width: 28px;
  min-width: 28px;
  height: 28px;
  padding: 0;
}

.test-result-card {
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr);
  align-items: start;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid #b9e8d2;
  border-radius: 10px;
  background: #eefaf4;
}

.test-result-card.failed {
  border-color: #f2c7cb;
  background: #fff3f4;
}

.test-result-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border-radius: 8px;
  color: #059669;
  background: #fff;
}

.test-result-card.failed .test-result-icon {
  color: #dc2626;
}

.test-result-card strong,
.test-result-card span {
  display: block;
}

.test-result-card strong {
  color: var(--model-heading);
  font-size: 13px;
  font-weight: 700;
}

.test-result-card span {
  margin-top: 4px;
  color: var(--model-secondary);
  font-size: 12px;
  line-height: 1.6;
  word-break: break-word;
}

.test-reply {
  display: grid;
  gap: 6px;
}

.test-reply span {
  color: var(--model-text);
  font-size: 12px;
  font-weight: 700;
}

.test-reply pre {
  margin: 0;
  padding: 10px;
  border: 1px solid var(--model-border-soft);
  border-radius: 8px;
  background: var(--model-surface-soft);
  color: var(--model-secondary);
  white-space: pre-wrap;
  word-break: break-word;
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 11px;
  line-height: 1.65;
}

.ai-model-page :deep(.el-button) {
  border-radius: var(--ops-button-radius, 8px);
  font-weight: 600;
  transition:
    color 160ms ease,
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease,
    transform 160ms ease;
}

.ai-model-page :deep(.el-button:not(.is-disabled):hover) {
  transform: translateY(-1px);
}

.ai-model-page :deep(.el-button:not(.is-disabled):active) {
  transform: translateY(0);
}

.ai-model-page :deep(.el-button:focus-visible) {
  outline: 2px solid var(--model-primary);
  outline-offset: 2px;
}

.ai-model-page :deep(.el-input__wrapper),
.ai-model-page :deep(.el-select__wrapper),
.ai-model-page :deep(.el-textarea__inner),
.ai-model-page :deep(.el-input-number .el-input__wrapper) {
  min-height: 34px;
  border-radius: var(--ops-input-radius, 8px);
  background: var(--model-surface-soft);
  box-shadow: inset 0 0 0 1px #d9e5f2 !important;
}

.ai-model-page :deep(.el-input__wrapper:hover),
.ai-model-page :deep(.el-select__wrapper:hover),
.ai-model-page :deep(.el-textarea__inner:hover) {
  box-shadow: inset 0 0 0 1px var(--ops-border-hover, #b9d4ff) !important;
}

.ai-model-page :deep(.el-input__wrapper.is-focus),
.ai-model-page :deep(.el-select__wrapper.is-focused),
.ai-model-page :deep(.el-textarea__inner:focus) {
  box-shadow: inset 0 0 0 1px #409eff !important;
}

.ai-model-page :deep(.el-input__inner),
.ai-model-page :deep(.el-select__placeholder),
.ai-model-page :deep(.el-select__selected-item) {
  font-size: 12px;
}

.ai-model-page :deep(.el-tag) {
  max-width: 100%;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
}

.ai-model-page :deep(.model-table.el-table) {
  --el-table-border-color: #eef3f9;
  --el-table-header-bg-color: #f3f7fc;
  --el-table-tr-bg-color: #ffffff;
  --el-table-row-hover-bg-color: #f8fafc;
  --el-table-bg-color: #ffffff;
  --el-table-text-color: var(--model-secondary);
  --el-table-header-text-color: var(--model-secondary);
  width: 100%;
  max-width: 100%;
  height: 100%;
  font-size: 12px;
}

.ai-model-page :deep(.model-table .el-table__inner-wrapper::before) {
  display: none;
}

.ai-model-page :deep(.model-table th.el-table__cell) {
  height: 36px;
  padding: 0;
  font-size: 12px;
  font-weight: 700;
}

.ai-model-page :deep(.model-table td.el-table__cell) {
  height: 42px;
  padding: 3px 0;
}

.ai-model-page :deep(.model-table td.el-table__cell),
.ai-model-page :deep(.model-table th.el-table__cell.is-leaf) {
  border-bottom-color: #eef3f9;
}

.ai-model-page :deep(.model-table .el-table__row:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

.ai-model-page :deep(.el-pagination) {
  flex-wrap: nowrap;
}

.ai-model-page :deep(.el-pagination .btn-prev),
.ai-model-page :deep(.el-pagination .btn-next),
.ai-model-page :deep(.el-pagination .el-pager li) {
  border-radius: 7px;
}

:global(.model-form-dialog-overlay),
:global(.model-secret-dialog-overlay),
:global(.model-test-dialog-overlay) {
  --model-heading: #0f172a;
  --model-text: #172033;
  --model-secondary: #53657d;
  --model-muted: #64748b;
  --model-primary: #2563eb;
  --model-surface-soft: #fbfdff;
  --model-subtle: #f3f7fc;
  --model-border: #dbe7f4;
  --model-border-soft: #e7eef7;
}

:global(.model-form-dialog-overlay .model-dialog),
:global(.model-secret-dialog-overlay .model-dialog),
:global(.model-test-dialog-overlay .model-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 24px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.model-dialog .el-dialog__header) {
  margin-right: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
}

:global(.model-dialog .el-dialog__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.model-dialog .el-dialog__headerbtn) {
  top: 12px;
  right: 14px;
  width: 34px;
  height: 34px;
}

:global(.model-dialog .el-dialog__body) {
  max-height: min(68vh, 720px);
  padding: 14px 18px;
  overflow: auto;
  color: #53657d;
}

:global(.model-dialog .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
}

:global(.model-form-dialog-overlay .el-input__wrapper),
:global(.model-form-dialog-overlay .el-select__wrapper),
:global(.model-form-dialog-overlay .el-textarea__inner) {
  min-height: 34px;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: inset 0 0 0 1px #d9e5f2 !important;
}

:global(.model-form-dialog-overlay .el-input__wrapper:hover),
:global(.model-form-dialog-overlay .el-select__wrapper:hover),
:global(.model-form-dialog-overlay .el-textarea__inner:hover) {
  box-shadow: inset 0 0 0 1px #b9d4ff !important;
}

:global(.model-form-dialog-overlay .el-input__wrapper.is-focus),
:global(.model-form-dialog-overlay .el-select__wrapper.is-focused),
:global(.model-form-dialog-overlay .el-textarea__inner:focus) {
  box-shadow: inset 0 0 0 1px #409eff !important;
}

:global(.model-form-dialog-overlay .el-form-item) {
  margin-bottom: 12px;
}

:global(.model-form-dialog-overlay .el-form-item__label) {
  height: auto;
  margin-bottom: 6px;
  padding: 0;
  color: #334155;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.4;
}

:global(.model-form-dialog-overlay .el-button),
:global(.model-secret-dialog-overlay .el-button),
:global(.model-test-dialog-overlay .el-button) {
  border-radius: 8px;
  font-weight: 600;
}

:global(.danger-menu-item) {
  color: #dc2626 !important;
}

@media (max-width: 1100px) {
  .ops-page-header {
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .ops-page-header__identity,
  .summary-group {
    width: 100%;
  }

  .summary-group {
    justify-content: flex-end;
  }
}

@media (max-width: 760px) {
  :global(.home-container .el-main:has(.ai-model-page)) {
    padding-right: 8px;
    padding-left: 8px;
  }

  .ai-model-page {
    height: auto;
    min-height: calc(100vh - 124px);
    padding: 6px;
    overflow: visible;
  }

  .ops-page-header {
    padding: 8px 10px;
  }

  .ops-page-header__content {
    align-items: flex-start;
    flex-direction: column;
    gap: 2px;
  }

  .ops-page-header__divider {
    display: none;
  }

  .ops-page-header h1 {
    font-size: 18px;
  }

  .summary-group {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .summary-chip,
  .summary-chip-wide {
    width: auto;
    min-width: 0;
  }

  .summary-chip-wide {
    grid-column: 1 / -1;
  }

  .toolbar-form {
    align-items: stretch;
    flex-direction: column;
  }

  .toolbar-form :deep(.el-form-item:first-child) {
    width: 100%;
    max-width: none;
  }

  .toolbar-actions-item {
    width: 100%;
    margin-left: 0 !important;
  }

  .toolbar-actions {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    width: 100%;
  }

  .toolbar-actions :deep(.el-button) {
    width: 100%;
  }

  .preset-panel,
  .secret-row {
    grid-template-columns: minmax(0, 1fr);
  }

  .preset-meta {
    justify-content: flex-start;
  }

  .secret-meta {
    grid-template-columns: minmax(0, 1fr);
  }

  .pagination-section {
    justify-content: flex-start;
  }
}

@media (max-width: 480px) {
  .summary-group {
    grid-template-columns: minmax(0, 1fr);
  }

  .summary-chip-wide {
    grid-column: auto;
  }

  .dialog-footer {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dialog-footer :deep(.el-button) {
    width: 100%;
  }

  .dialog-footer :deep(.el-button:first-child:nth-last-child(3)) {
    grid-column: 1 / -1;
  }
}

@media (prefers-reduced-motion: reduce) {
  .ai-model-page *,
  .ai-model-page *::before,
  .ai-model-page *::after {
    scroll-behavior: auto !important;
    transition-duration: 0.01ms !important;
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
  }
}
</style>
