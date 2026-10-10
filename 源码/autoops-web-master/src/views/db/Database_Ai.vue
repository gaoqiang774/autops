<template>
  <div class="db-ai-page">
    <section class="db-ai-shell">
      <header class="db-ai-header">
        <div class="header-copy">
          <h1>数据库 AI 助手</h1>
          <p>面向数据库运维诊断的对话助手，自动收集只读证据并给出结论、风险和建议动作。</p>
        </div>
        <div class="header-actions">
          <span class="mode-chip">
            <i></i>
            自动收集证据
          </span>
          <el-button @click="showAdvancedSettings = !showAdvancedSettings">AI 配置</el-button>
          <el-button :icon="Refresh" :loading="toolsLoading" @click="loadTools">刷新工具</el-button>
          <el-button :disabled="!messages.length" @click="showRecords = !showRecords">诊断记录</el-button>
        </div>
      </header>

      <div class="assistant-workspace">
        <aside class="context-panel surface-panel">
          <div class="panel-heading">
            <div>
              <span>TARGET</span>
              <h2>当前诊断对象</h2>
            </div>
            <el-tag effect="light">{{ selectedAssetEngineText }}</el-tag>
          </div>

          <el-select
            v-model="form.assetId"
            filterable
            remote
            reserve-keyword
            placeholder="请选择数据库资产"
            :loading="assetLoading"
            :remote-method="searchAssets"
            @visible-change="handleAssetVisible"
          >
            <el-option
              v-for="asset in assets"
              :key="asset.id"
              :label="assetLabel(asset)"
              :value="asset.id"
            >
              <div class="asset-option">
                <strong>{{ asset.instanceName || asset.name || `资产 #${asset.id}` }}</strong>
                <el-tag size="small" effect="light">{{ engineText(asset) }}</el-tag>
                <span>{{ asset.endpoint || joinEndpoint(asset.hostIp, asset.port) }}</span>
              </div>
            </el-option>
          </el-select>

          <div class="target-card">
            <strong>{{ selectedAssetName }}</strong>
            <p>{{ selectedAssetEndpoint || '选择资产后自动带入实例地址' }}</p>
            <div class="target-meta">
              <span>{{ selectedAssetEngineText }}</span>
              <span>{{ selectedAssetEnv }}</span>
            </div>
          </div>

          <div class="range-card">
            <span>诊断范围</span>
            <div>
              <el-select v-model="form.range" size="small">
                <el-option label="最近 1 小时" value="1h" />
                <el-option label="最近 6 小时" value="6h" />
                <el-option label="最近 24 小时" value="24h" />
              </el-select>
              <el-select v-model="form.step" size="small">
                <el-option label="1 分钟" value="1m" />
                <el-option label="5 分钟" value="5m" />
                <el-option label="15 分钟" value="15m" />
              </el-select>
            </div>
          </div>

          <el-collapse v-model="activeContextPanels" class="context-collapse">
            <el-collapse-item title="上下文" name="context">
              <el-form label-position="top" class="context-form">
                <el-form-item label="数据库 / Schema">
                  <div class="inline-fields">
                    <el-input v-model.trim="form.database" placeholder="database" clearable />
                    <el-input v-model.trim="form.schema" placeholder="schema/public" clearable />
                  </div>
                </el-form-item>
                <el-form-item label="对象">
                  <div class="inline-fields">
                    <el-input v-model.trim="form.objectName" placeholder="表、视图或 Redis Key" clearable />
                    <el-select v-model="form.objectType">
                      <el-option label="table" value="table" />
                      <el-option label="view" value="view" />
                      <el-option label="redis_key" value="redis_key" />
                    </el-select>
                  </div>
                </el-form-item>
              </el-form>
            </el-collapse-item>
            <el-collapse-item v-if="showAdvancedSettings" title="高级设置" name="advanced">
              <el-form label-position="top" class="context-form">
                <el-form-item label="Provider ID">
                  <el-input-number
                    v-model="form.providerId"
                    :min="1"
                    controls-position="right"
                    placeholder="默认模型可留空"
                  />
                </el-form-item>
              </el-form>
            </el-collapse-item>
          </el-collapse>

          <div class="prompt-section">
            <div class="panel-heading compact">
              <div>
                <span>PROMPTS</span>
                <h2>快捷提问</h2>
              </div>
            </div>
            <div class="prompt-buttons">
              <el-button
                v-for="item in promptTemplates"
                :key="item.title"
                :disabled="chatLoading"
                @click="askTemplate(item)"
              >
                {{ item.title }}
              </el-button>
            </div>
          </div>
        </aside>

        <main class="chat-panel surface-panel">
          <div class="chat-toolbar">
            <div>
              <h2>诊断会话</h2>
            </div>
            <div class="toolbar-actions">
              <el-switch v-model="form.includeRawEvidence" size="small" />
              <span>原始证据</span>
              <el-button text :disabled="!messages.length || chatLoading" @click="clearMessages">清空</el-button>
            </div>
          </div>

          <div ref="messageListRef" class="message-list">
            <div v-if="showRecords && messages.length" class="records-strip">
              <button
                v-for="item in recentQuestions"
                :key="item.id"
                type="button"
                @click="fillQuestion(item.content)"
              >
                {{ item.content }}
              </button>
            </div>

            <article
              v-for="message in messages"
              :key="message.id"
              class="message-item"
              :class="message.role"
            >
              <div class="message-avatar" :class="`${message.role}-avatar`">
                <img :src="message.role === 'user' ? userAvatar : aiAvatar" alt="" />
              </div>
              <div class="message-body">
                <div class="message-meta">
                  <strong>{{ message.role === 'user' ? '用户提问' : '数据库 AI' }}</strong>
                  <span>{{ message.time }}</span>
                  <el-tag
                    v-if="message.role === 'assistant'"
                    size="small"
                    effect="light"
                    type="info"
                  >
                    AI 回答
                  </el-tag>
                </div>

                <p v-if="message.role === 'user'" class="question-text">{{ message.content }}</p>

                <template v-else>
                  <div class="answer-summary" :class="{ 'tone-warning': message.isError }">
                    <strong>{{ message.isError ? '诊断失败' : message.payload?.summary || '诊断完成' }}</strong>
                  </div>

                  <div v-if="message.isError" class="error-brief">
                    <strong>{{ message.payload.summary }}</strong>
                    <div class="markdown-content" v-html="renderMarkdown(message.payload.answer)" />
                    <div class="error-actions">
                      <el-button size="small" @click="retryLastQuestion">重新诊断</el-button>
                      <el-button size="small" @click="copyError(message.payload)">复制错误</el-button>
                    </div>
                  </div>

                  <div v-else class="answer-report">
                    <section>
                      <h3>结论</h3>
                      <div
                        class="markdown-content"
                        v-html="renderMarkdown(message.payload?.answer || message.content)"
                      />
                    </section>

                    <section v-if="answerEvidenceSummaries(message.payload).length">
                      <h3>关键证据</h3>
                      <ul>
                        <li v-for="item in answerEvidenceSummaries(message.payload)" :key="item">{{ item }}</li>
                      </ul>
                    </section>

                    <section v-if="normalizeChatTextList(message.payload?.suggestions).length">
                      <h3>建议动作</h3>
                      <ol>
                        <li v-for="suggestion in normalizeChatTextList(message.payload?.suggestions)" :key="suggestion">
                          <button type="button" @click="fillQuestion(suggestion)">{{ suggestion }}</button>
                        </li>
                      </ol>
                    </section>
                  </div>
                </template>
              </div>
            </article>

            <article v-if="chatLoading" class="message-item assistant thinking-message">
              <div class="message-avatar assistant-avatar">
                <img :src="aiAvatar" alt="" />
              </div>
              <div class="message-body thinking-body">
                <div class="thinking-dots">
                  <i></i>
                  <i></i>
                  <i></i>
                </div>
                <strong>正在思考中</strong>
                <span>{{ loadingStageText }}</span>
              </div>
            </article>
          </div>

          <div class="composer">
            <el-input
              v-model="form.question"
              type="textarea"
              resize="none"
              :autosize="{ minRows: 4, maxRows: 8 }"
              placeholder="输入问题，例如：这个 MySQL 最近 6 小时有没有异常？重点看连接数、CPU、磁盘 IO。需要分析 SQL 时，可直接把 SQL 粘贴在问题里。"
              @keydown.enter.exact.prevent="sendQuestion()"
            />
            <div class="composer-actions">
              <div class="safe-note">只读边界由后端统一控制，变更类建议仍走 SQL 审核。</div>
              <el-button type="primary" :icon="Promotion" :loading="chatLoading" @click="sendQuestion()">
                发送
              </el-button>
            </div>
          </div>
        </main>

        <aside class="evidence-panel surface-panel">
          <div class="panel-heading">
            <div>
              <span>EVIDENCE</span>
              <h2>证据链</h2>
            </div>
            <el-button
              v-if="latestAssistant"
              :icon="CopyDocument"
              circle
              title="复制回答"
              @click="copyAnswer"
            />
          </div>

          <template v-if="latestAssistant">
            <div class="status-card" :class="{ 'tone-warning': latestAssistant.isError }">
              <span>摘要</span>
              <strong>{{ latestAssistant.payload?.summary || '暂无摘要' }}</strong>
            </div>

            <div class="agent-trace">
              <div>
                <span>意图识别</span>
                <strong>{{ latestIntent }}</strong>
              </div>
              <div>
                <span>工具计划</span>
                <strong>{{ latestToolPlanText }}</strong>
              </div>
            </div>

            <div class="side-section">
              <h3>已调用工具</h3>
              <div v-if="latestToolCalls.length" class="call-list">
                <div v-for="tool in latestToolCalls" :key="tool">
                  <span>✓</span>
                  <strong>{{ toolLabel(tool) }}</strong>
                </div>
              </div>
              <el-empty v-else description="暂无工具调用记录" :image-size="64" />
            </div>

            <div v-if="missingEvidence.length" class="side-section">
              <h3>缺失证据</h3>
              <div class="missing-list">
                <span v-for="item in missingEvidence" :key="item">{{ item }}</span>
              </div>
            </div>

            <el-collapse class="evidence-collapse">
              <el-collapse-item v-if="latestEvidence.length" title="证据详情" name="evidence">
                <div class="evidence-list">
                  <div v-for="item in latestEvidence" :key="`${item.tool}-${item.summary}`" class="evidence-item">
                    <div>
                      <strong>{{ toolLabel(item.tool) }}</strong>
                      <el-tag size="small" effect="light" :type="item.supported === false ? 'warning' : 'success'">
                        {{ item.supported === false ? '不支持' : '已支持' }}
                      </el-tag>
                    </div>
                    <p>{{ item.summary || '-' }}</p>
                    <small v-if="normalizeChatTextList(item.warnings).length">
                      {{ normalizeChatTextList(item.warnings).join(' / ') }}
                    </small>
                  </div>
                </div>
              </el-collapse-item>

              <el-collapse-item v-if="latestWarnings.length" title="Warnings" name="warnings">
                <div class="warning-stack">
                  <el-alert
                    v-for="warning in latestWarnings"
                    :key="warning"
                    :title="warning"
                    type="warning"
                    show-icon
                    :closable="false"
                  />
                </div>
              </el-collapse-item>

            </el-collapse>
          </template>

          <el-empty v-else description="发送问题后展示证据链" :image-size="90" />

          <el-collapse v-model="activeExpertPanels" class="expert-collapse">
            <el-collapse-item title="专家工具 / 调试" name="tools">
              <div class="tool-picker">
                <button
                  v-for="tool in toolOptions"
                  :key="tool.name"
                  type="button"
                  class="tool-item"
                  :class="{ active: expert.tool === tool.name }"
                  @click="selectTool(tool.name)"
                >
                  <strong>{{ toolLabel(tool.name) }}</strong>
                  <span>{{ tool.description || fallbackToolDescription(tool.name) }}</span>
                </button>
              </div>

              <el-form label-position="top" class="expert-form">
                <el-form-item v-if="needsKeyword" label="关键字">
                  <el-input v-model.trim="expert.keyword" placeholder="对象搜索关键字" clearable />
                </el-form-item>
                <el-form-item v-if="needsSql" label="SQL / Redis 只读命令">
                  <el-input
                    v-model="form.sql"
                    type="textarea"
                    resize="none"
                    :autosize="{ minRows: 4, maxRows: 8 }"
                    placeholder="仅支持 SELECT / WITH，Redis 仅支持只读命令"
                  />
                </el-form-item>
                <div class="expert-grid">
                  <el-form-item label="页码">
                    <el-input-number v-model="expert.page" :min="1" :max="9999" controls-position="right" />
                  </el-form-item>
                  <el-form-item label="每页">
                    <el-input-number v-model="expert.pageSize" :min="1" :max="200" controls-position="right" />
                  </el-form-item>
                </div>
                <div class="expert-actions">
                  <el-switch v-model="expert.confirmed" size="small" />
                  <span>确认执行</span>
                  <el-button type="primary" :loading="toolLoading" @click="runTool()">执行工具</el-button>
                </div>
              </el-form>

              <div v-if="toolResult" class="tool-result" v-loading="toolLoading">
                <div class="tool-result-head">
                  <strong>{{ toolResult.summary || '工具执行完成' }}</strong>
                  <el-button :icon="CopyDocument" circle title="复制结果" @click="copyToolResult" />
                </div>
                <el-tabs v-model="activeToolTab">
                  <el-tab-pane label="概览" name="overview">
                    <div class="kv-grid">
                      <div v-for="item in toolOverviewRows" :key="item.key">
                        <span>{{ item.key }}</span>
                        <strong>{{ item.value }}</strong>
                      </div>
                    </div>
                    <div v-if="toolMetricRows.length" class="metric-grid">
                      <div
                        v-for="item in toolMetricRows"
                        :key="item.key || item.name"
                        class="mini-metric"
                        :class="`tone-${item.status || 'normal'}`"
                      >
                        <span>{{ item.name || item.key }}</span>
                        <strong>{{ formatMetric(item.value, item.unit) }}</strong>
                      </div>
                    </div>
                  </el-tab-pane>
                  <el-tab-pane label="表格" name="table">
                    <el-table v-if="toolTableRows.length" :data="toolTableRows" height="260">
                      <el-table-column
                        v-for="column in toolTableColumns"
                        :key="column"
                        :prop="column"
                        :label="column"
                        min-width="120"
                        show-overflow-tooltip
                      />
                    </el-table>
                    <el-empty v-else description="暂无表格数据" :image-size="70" />
                  </el-tab-pane>
                  <el-tab-pane label="JSON" name="json">
                    <pre class="code-block">{{ formatJson(toolResult) }}</pre>
                  </el-tab-pane>
                </el-tabs>
              </div>
            </el-collapse-item>
          </el-collapse>
        </aside>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { CopyDocument, Promotion, Refresh } from '@element-plus/icons-vue'
import dbApi from '@/api/database'
import aiAvatar from '@/assets/image/AI小助手.svg'
import userAvatar from '@/assets/image/touxiang.svg'

const successCodes = new Set([0, 200])
const supportedEngines = new Set([1, 2, 4])
const loadingStages = ['正在理解问题...', '正在收集数据库证据...', '正在生成诊断结论...']

const fallbackTools = [
  {
    name: 'db_search_objects',
    description: '搜索数据库、schema、表、视图、Redis DB/Key 等对象。',
    readOnly: true,
    requiredFields: ['assetId'],
    optionalFields: ['database', 'schema', 'objectName', 'keyword']
  },
  {
    name: 'db_get_table_schema',
    description: '获取表或视图结构、字段、索引和 DDL。',
    readOnly: true,
    requiredFields: ['assetId', 'objectName'],
    optionalFields: ['database', 'schema', 'objectType']
  },
  {
    name: 'db_execute_readonly_sql',
    description: '执行只读 SQL 或 Redis 只读命令。',
    readOnly: true,
    requiredFields: ['assetId', 'sql'],
    optionalFields: ['database', 'page', 'pageSize', 'confirmed']
  },
  {
    name: 'db_explain_sql',
    description: '获取 SELECT / WITH 的执行计划。',
    readOnly: true,
    requiredFields: ['assetId', 'sql'],
    optionalFields: ['database', 'schema']
  },
  {
    name: 'db_get_monitor_snapshot',
    description: '获取数据库监控快照和核心指标。',
    readOnly: true,
    requiredFields: ['assetId'],
    optionalFields: ['range', 'step']
  },
  {
    name: 'db_get_instance_health',
    description: '获取数据库实例健康摘要。',
    readOnly: true,
    requiredFields: ['assetId'],
    optionalFields: ['range', 'step']
  },
  {
    name: 'db_get_slow_sql_top',
    description: '获取慢 SQL Top，取决于后端慢日志源接入。',
    readOnly: true,
    requiredFields: ['assetId'],
    optionalFields: ['range', 'pageSize']
  }
]

const assets = ref([])
const tools = ref([])
const messages = ref([])
const toolResult = ref(null)
const messageListRef = ref(null)
const assetLoading = ref(false)
const toolsLoading = ref(false)
const chatLoading = ref(false)
const toolLoading = ref(false)
const loadingStage = ref(0)
const activeToolTab = ref('overview')
const activeExpertPanels = ref([])
const activeContextPanels = ref([])
const showAdvancedSettings = ref(false)
const showRecords = ref(false)
const lastQuestion = ref('')
let loadingTimer = null

const form = reactive({
  assetId: '',
  database: '',
  schema: '',
  objectName: '',
  objectType: 'table',
  question: '',
  sql: '',
  range: '6h',
  step: '1m',
  providerId: undefined,
  includeRawEvidence: false
})

const expert = reactive({
  tool: 'db_get_instance_health',
  keyword: '',
  page: 1,
  pageSize: 20,
  confirmed: false
})

const toolOptions = computed(() => tools.value.length ? tools.value : fallbackTools)
const selectedAsset = computed(() => assets.value.find(item => String(item.id) === String(form.assetId)) || null)
const selectedAssetName = computed(() => selectedAsset.value?.instanceName || selectedAsset.value?.name || (form.assetId ? `资产 #${form.assetId}` : '未选择资产'))
const selectedAssetEndpoint = computed(() => selectedAsset.value?.endpoint || joinEndpoint(selectedAsset.value?.hostIp, selectedAsset.value?.port))
const selectedAssetEngineText = computed(() => selectedAsset.value ? engineText(selectedAsset.value) : '未选择')
const selectedAssetEnv = computed(() => selectedAsset.value?.env || selectedAsset.value?.environment || selectedAsset.value?.envName || '生产')
const promptTemplates = computed(() => {
  const engine = getAssetEngine(selectedAsset.value || {})
  const engineName = selectedAssetEngineText.value || '数据库'
  const rangeText = rangeLabel(form.range)
  const basePrompts = [
    {
      title: '健康巡检',
      question: `检查这个 ${engineName} 实例最近 ${rangeText} 是否健康，先给结论，再列出异常指标、证据和排查顺序。`
    },
    {
      title: '监控异常',
      question: `分析这个 ${engineName} 实例最近 ${rangeText} 的监控异常，重点看连接数、QPS、CPU、内存、磁盘 IO 和网络。`
    },
    {
      title: '连接风险',
      question: `分析这个 ${engineName} 实例当前连接数是否异常，判断是否存在连接泄漏、连接池配置不合理或突增流量。`
    },
    {
      title: '主机瓶颈',
      question: `重点分析这个 ${engineName} 关联主机最近 ${rangeText} 的 CPU、内存、磁盘空间和磁盘 IO，判断是否影响数据库。`
    }
  ]

  if (engine === 4) {
    return [
      ...basePrompts,
      {
        title: 'Key 排查',
        question: '帮我搜索 Redis Key 或对象信息，并说明命中结果、类型和可能的业务含义。'
      },
      {
        title: 'Redis 命令',
        question: '帮我执行 Redis 只读命令并解释结果；如果需要命令，我会把只读命令粘贴到问题里。'
      }
    ]
  }

  return [
    ...basePrompts,
    {
      title: '慢 SQL Top',
      question: `查看最近 ${rangeText} 慢 SQL Top，按风险排序，给出 SQL 指纹、耗时、扫描行数、慢查询原因和优化建议。`
    },
    {
      title: 'SQL 解释',
      question: '分析我粘贴的 SQL 为什么慢，重点看执行计划、索引使用、扫描行数和可改写方案。'
    },
    {
      title: '表结构',
      question: '分析当前表结构，说明字段、主键、索引设计，以及可能影响查询性能的风险点。'
    }
  ]
})
const latestAssistant = computed(() => [...messages.value].reverse().find(item => item.role === 'assistant') || null)
const latestEvidence = computed(() => normalizeChatEvidence(latestAssistant.value?.payload?.evidence))
const latestWarnings = computed(() => normalizeChatTextList(latestAssistant.value?.payload?.warnings))
const latestToolCalls = computed(() => latestEvidence.value.map(item => item.tool).filter(Boolean))
const latestIntent = computed(() => inferIntent(latestAssistant.value?.question || lastQuestion.value))
const latestToolPlanText = computed(() => {
  const tools = latestToolCalls.value.length ? latestToolCalls.value : inferToolPlan(latestAssistant.value?.question || lastQuestion.value)
  return tools.map(toolLabel).join('、') || '等待后端返回工具计划'
})
const missingEvidence = computed(() => {
  const payloadMissing = normalizeChatTextList(latestAssistant.value?.payload?.missingEvidence)
  const evidenceMissing = latestEvidence.value.flatMap(item => normalizeChatTextList(item.missingEvidence))
  return [...new Set([...payloadMissing, ...evidenceMissing].filter(Boolean))]
})
const recentQuestions = computed(() => messages.value.filter(item => item.role === 'user').slice(-6).reverse())
const loadingStageText = computed(() => loadingStages[loadingStage.value] || loadingStages[0])
const needsSql = computed(() => ['db_execute_readonly_sql', 'db_explain_sql'].includes(expert.tool))
const needsKeyword = computed(() => expert.tool === 'db_search_objects')
const toolData = computed(() => toolResult.value?.data || {})
const toolMetricRows = computed(() => normalizeList(toolData.value.overview || toolData.value.abnormal))
const toolOverviewRows = computed(() => {
  const rows = [
    ['工具', toolLabel(toolResult.value?.tool)],
    ['支持状态', toolResult.value?.supported === false ? '不支持' : '支持'],
    ['资产', toolData.value.asset?.name || toolData.value.asset?.instanceName],
    ['实例', toolData.value.asset?.endpoint],
    ['健康', toolData.value.status || toolData.value.health],
    ['Prometheus', toolData.value.prometheus || toolData.value.target?.prometheus],
    ['指标地址', toolData.value.metricsUrl || toolData.value.target?.metricsUrl],
    ['主机指标', toolData.value.hostMetricsUrl || toolData.value.target?.hostScrapeUrl],
    ['耗时', toolData.value.durationMs ? `${toolData.value.durationMs}ms` : '']
  ]
  return rows
    .filter(([, value]) => value !== undefined && value !== null && value !== '')
    .map(([key, value]) => ({ key, value: formatDisplay(value) }))
})
const toolTableRows = computed(() => {
  const data = toolData.value
  if (Array.isArray(data.rows)) return data.rows
  if (Array.isArray(data.plan)) return data.plan
  if (Array.isArray(data.items)) return data.items
  if (Array.isArray(data.tables)) return data.tables
  if (Array.isArray(data.columns)) return data.columns
  if (Array.isArray(data.indexes)) return data.indexes
  return []
})
const toolTableColumns = computed(() => {
  const first = toolTableRows.value[0]
  return first && typeof first === 'object' ? Object.keys(first) : []
})

const unwrap = response => {
  const body = response?.data || response || {}
  if (body.code !== undefined && !successCodes.has(Number(body.code))) {
    throw new Error(body.message || body.msg || '请求失败')
  }
  return body.data !== undefined ? body.data : body
}

const normalizeList = value => {
  if (Array.isArray(value)) return value
  if (Array.isArray(value?.list)) return value.list
  if (Array.isArray(value?.rows)) return value.rows
  return []
}

function joinEndpoint(host, port) {
  return [host, port].filter(Boolean).join(':')
}

function getAssetEngine(asset = {}) {
  return Number(asset.type || asset.engine || asset.dbType || 0)
}

function engineText(asset = {}) {
  return {
    1: 'MySQL',
    2: 'PostgreSQL',
    4: 'Redis'
  }[getAssetEngine(asset)] || asset.typeName || '数据库'
}

function assetLabel(asset = {}) {
  return `${engineText(asset)} ${asset.instanceName || asset.name || `资产 #${asset.id}`} ${asset.endpoint || joinEndpoint(asset.hostIp, asset.port) || ''}`.trim()
}

function rangeLabel(value = '') {
  return {
    '1h': '1 小时',
    '6h': '6 小时',
    '24h': '24 小时'
  }[value] || value || '当前时间范围'
}

function toolLabel(name = '') {
  return {
    db_search_objects: '搜索对象',
    db_get_table_schema: '表结构',
    db_execute_readonly_sql: '只读查询',
    db_explain_sql: '执行计划',
    db_get_monitor_snapshot: '监控快照',
    db_get_instance_health: '实例健康',
    db_get_slow_sql_top: '慢 SQL Top'
  }[name] || name || '-'
}

function fallbackToolDescription(name = '') {
  return fallbackTools.find(item => item.name === name)?.description || '数据库 AI 只读工具'
}

function inferIntent(question = '') {
  const text = String(question || '').toLowerCase()
  if (text.includes('慢') || text.includes('slow')) return '慢 SQL / 慢命令分析'
  if (text.includes('sql') || text.includes('执行计划') || text.includes('索引')) return 'SQL 诊断'
  if (text.includes('表') || text.includes('结构') || text.includes('schema')) return '对象结构分析'
  if (text.includes('连接')) return '连接数风险分析'
  if (text.includes('cpu') || text.includes('内存') || text.includes('磁盘') || text.includes('io')) return '主机资源瓶颈诊断'
  if (text.includes('监控') || text.includes('异常')) return '监控异常分析'
  return '实例健康诊断'
}

function inferToolPlan(question = '') {
  const text = String(question || '').toLowerCase()
  if (text.includes('慢') || text.includes('slow')) return ['db_get_slow_sql_top', 'db_get_instance_health']
  if (text.includes('sql') || text.includes('执行计划') || text.includes('索引')) return ['db_explain_sql', 'db_get_table_schema']
  if (text.includes('表') || text.includes('结构') || text.includes('schema')) return ['db_get_table_schema', 'db_search_objects']
  if (text.includes('监控') || text.includes('cpu') || text.includes('内存') || text.includes('磁盘') || text.includes('io')) {
    return ['db_get_instance_health', 'db_get_monitor_snapshot']
  }
  return ['db_get_instance_health', 'db_get_monitor_snapshot']
}

function answerEvidenceSummaries(payload = {}) {
  return normalizeChatEvidence(payload.evidence)
    .map(item => item.summary || `${toolLabel(item.tool)} 已返回证据`)
    .filter(Boolean)
    .slice(0, 5)
}

function pickChatText(value) {
  if (value === undefined || value === null || value === '') return ''
  if (typeof value === 'string' || typeof value === 'number' || typeof value === 'boolean') return String(value)
  if (typeof value !== 'object') return ''
  return String(
    value.summary
      || value.message
      || value.text
      || value.reason
      || value.name
      || value.title
      || ''
  )
}

function normalizeChatTextList(value) {
  const list = Array.isArray(value) ? value : normalizeList(value)
  if (!list.length && pickChatText(value)) return [pickChatText(value)]
  return list.map(pickChatText).filter(Boolean)
}

function normalizeChatEvidence(value) {
  return normalizeList(value)
    .map(item => {
      if (typeof item === 'string') {
        return {
          tool: '',
          summary: item,
          warnings: [],
          missingEvidence: []
        }
      }
      if (!item || typeof item !== 'object') return null
      return {
        tool: String(item.tool || item.toolName || item.name || ''),
        summary: pickChatText(item) || pickChatText(item.data),
        supported: item.supported,
        warnings: normalizeChatTextList(item.warnings),
        missingEvidence: normalizeChatTextList(item.missingEvidence || item.missing)
      }
    })
    .filter(item => item && (item.summary || item.tool || item.warnings.length || item.missingEvidence.length))
}

function normalizeDbAiChatPayload(payload = {}) {
  return {
    summary: pickChatText(payload.summary),
    answer: pickChatText(payload.answer),
    suggestions: normalizeChatTextList(payload.suggestions),
    warnings: normalizeChatTextList(payload.warnings),
    missingEvidence: normalizeChatTextList(payload.missingEvidence || payload.missing_evidence),
    evidence: normalizeChatEvidence(payload.evidence)
  }
}

function buildFriendlyChatError(error) {
  const status = error?.response?.status
  const requestUrl = error?.config?.url || '/api/v1/db/ai/chat'
  if (status === 404) {
    return {
      summary: 'AI-Agent 接口未找到',
      answer: 'AI 助手接口未找到，请确认后端版本已部署并包含 /api/v1/db/ai/chat。',
      warnings: ['接口未找到: /api/v1/db/ai/chat'],
      missingEvidence: [`request: ${requestUrl}`],
      suggestions: ['请确认后端版本已部署并包含 /api/v1/db/ai/chat'],
      evidence: []
    }
  }
  const message = error?.response?.data?.message || error?.response?.data?.msg || error?.message || 'AI 诊断失败'
  return {
    summary: 'AI 诊断失败',
    answer: message,
    warnings: [message],
    missingEvidence: status ? [`HTTP ${status}: ${requestUrl}`] : [],
    suggestions: ['请检查后端服务、网关转发和模型 Provider 配置'],
    evidence: []
  }
}

function formatDisplay(value) {
  if (value === undefined || value === null || value === '') return '-'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function formatJson(value) {
  try {
    return JSON.stringify(value || {}, null, 2)
  } catch (error) {
    return String(value || '')
  }
}

function escapeHtml(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function renderInlineMarkdown(text) {
  return escapeHtml(text)
    .replace(/`([^`]+)`/g, '<code title="$1">$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
}

function renderCodeBlock(codeLines, language = '') {
  const lang = String(language || 'text').trim() || 'text'
  const code = escapeHtml(codeLines.join('\n').replace(/\n+$/g, ''))
  return [
    '<pre class="markdown-code">',
    `<code class="language-${escapeHtml(lang)}">${code}</code>`,
    '</pre>'
  ].join('')
}

function escapeAttribute(value) {
  return escapeHtml(value).replace(/`/g, '&#96;')
}

function isMarkdownTableLine(line = '') {
  const trimmed = String(line || '').trim()
  return trimmed.startsWith('|') && trimmed.endsWith('|') && trimmed.split('|').length >= 4
}

function parseMarkdownTableRow(line = '') {
  return String(line || '')
    .trim()
    .replace(/^\|/, '')
    .replace(/\|$/, '')
    .split('|')
    .map(cell => cell.trim())
}

function isMarkdownTableDivider(line = '') {
  if (!isMarkdownTableLine(line)) return false
  const cells = parseMarkdownTableRow(line)
  return cells.length > 0 && cells.every(cell => /^:?-{3,}:?$/.test(cell))
}

function renderMarkdownTable(headerLine, dividerLine, bodyLines) {
  const headers = parseMarkdownTableRow(headerLine)
  const alignments = parseMarkdownTableRow(dividerLine).map(cell => {
    if (/^:-+:$/.test(cell)) return 'center'
    if (/-+:$/.test(cell)) return 'right'
    return 'left'
  })
  const rows = bodyLines.map(parseMarkdownTableRow)
  const columnType = header => {
    const text = String(header || '').toLowerCase()
    if (text.includes('排名') || text === 'rank') return 'rank'
    if (text.includes('风险') || text.includes('等级')) return 'risk'
    if (text.includes('sql') || text.includes('指纹')) return 'sql'
    if (text.includes('次数') || text.includes('耗时') || text.includes('行数') || text.includes('count')) return 'number'
    return 'text'
  }
  const columnClasses = headers.map((header, index) => [
    `col-${columnType(header)}`,
    alignments[index] && alignments[index] !== 'left' ? `align-${alignments[index]}` : ''
  ].filter(Boolean).join(' '))
  const renderCell = (tag, cell, index) => {
    const text = String(cell || '')
    return `<${tag} class="${columnClasses[index] || 'col-text'}" title="${escapeAttribute(text)}"><span>${renderInlineMarkdown(text)}</span></${tag}>`
  }

  return [
    '<div class="markdown-table-wrap"><table class="markdown-table">',
    '<colgroup>',
    headers.map((header, index) => `<col class="${columnClasses[index] || 'col-text'}">`).join(''),
    '</colgroup>',
    '<thead><tr>',
    headers.map((cell, index) => renderCell('th', cell, index)).join(''),
    '</tr></thead>',
    '<tbody>',
    rows.map(row => [
      '<tr>',
      headers.map((_, index) => renderCell('td', row[index] || '', index)).join(''),
      '</tr>'
    ].join('')).join(''),
    '</tbody></table></div>'
  ].join('')
}

function nextNonEmptyLineIndex(lines, startIndex) {
  let index = startIndex
  while (index < lines.length && !String(lines[index] || '').trim()) index += 1
  return index
}

function renderMarkdown(value) {
  const lines = String(value || '').replace(/\r\n/g, '\n').split('\n')
  const html = []
  let listType = ''
  let inCodeBlock = false
  let codeLanguage = ''
  let codeLines = []

  const closeList = () => {
    if (!listType) return
    html.push(listType === 'ol' ? '</ol>' : '</ul>')
    listType = ''
  }

  const openList = type => {
    if (listType === type) return
    closeList()
    html.push(type === 'ol' ? '<ol>' : '<ul>')
    listType = type
  }

  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index]
    const fenceMatch = line.match(/^\s*```([\w-]*)\s*$/)
    if (fenceMatch) {
      if (inCodeBlock) {
        html.push(renderCodeBlock(codeLines, codeLanguage))
        codeLines = []
        codeLanguage = ''
        inCodeBlock = false
        continue
      }
      closeList()
      inCodeBlock = true
      codeLanguage = fenceMatch[1] || ''
      codeLines = []
      continue
    }

    if (inCodeBlock) {
      codeLines.push(line)
      continue
    }

    const trimmed = line.trim()
    if (!trimmed) {
      closeList()
      continue
    }

    const dividerIndex = nextNonEmptyLineIndex(lines, index + 1)
    if (isMarkdownTableLine(trimmed) && isMarkdownTableDivider(lines[dividerIndex])) {
      closeList()
      const bodyLines = []
      let rowIndex = nextNonEmptyLineIndex(lines, dividerIndex + 1)
      while (rowIndex < lines.length && isMarkdownTableLine(lines[rowIndex].trim())) {
        bodyLines.push(lines[rowIndex])
        rowIndex = nextNonEmptyLineIndex(lines, rowIndex + 1)
      }
      index = rowIndex - 1
      html.push(renderMarkdownTable(trimmed, lines[dividerIndex], bodyLines))
      continue
    }

    const headingMatch = trimmed.match(/^(#{1,4})\s+(.+)$/)
    if (headingMatch) {
      closeList()
      const level = Math.min(headingMatch[1].length + 2, 5)
      html.push(`<h${level}>${renderInlineMarkdown(headingMatch[2])}</h${level}>`)
      continue
    }

    const numberedMatch = trimmed.match(/^(\d+)[.)]\s+(.+)$/)
    if (numberedMatch) {
      openList('ol')
      html.push(`<li>${renderInlineMarkdown(numberedMatch[2])}</li>`)
      continue
    }

    const bulletMatch = trimmed.match(/^[-*]\s+(.+)$/)
    if (bulletMatch) {
      openList('ul')
      html.push(`<li>${renderInlineMarkdown(bulletMatch[1])}</li>`)
      continue
    }

    closeList()
    html.push(`<p>${renderInlineMarkdown(trimmed)}</p>`)
  }

  if (inCodeBlock) html.push(renderCodeBlock(codeLines, codeLanguage))
  closeList()
  return html.join('')
}

function formatMetric(value, unit = '') {
  if (value === undefined || value === null || value === '') return '-'
  const num = Number(value)
  if (!Number.isFinite(num)) return `${value}${unit || ''}`
  if (['bytes/s', 'b/s'].includes(String(unit).toLowerCase())) return `${(num / 1024 / 1024).toFixed(2)}MB/s`
  if (['bytes', 'byte', 'b'].includes(String(unit).toLowerCase())) return `${(num / 1024 / 1024).toFixed(2)}MB`
  const text = Math.abs(num) >= 100 ? num.toFixed(0) : Math.abs(num) >= 10 ? num.toFixed(1) : num.toFixed(2)
  return `${text}${unit || ''}`
}

function buildChatPayload(question = form.question) {
  const payload = {
    assetId: form.assetId,
    database: form.database,
    schema: form.schema,
    objectName: form.objectName,
    objectType: form.objectType,
    question,
    sql: form.sql,
    range: form.range,
    step: form.step,
    providerId: form.providerId,
    includeRawEvidence: form.includeRawEvidence
  }
  Object.keys(payload).forEach(key => {
    if (payload[key] === '' || payload[key] === undefined || payload[key] === null) delete payload[key]
  })
  return payload
}

function buildToolPayload(confirmed = expert.confirmed) {
  const payload = {
    tool: expert.tool,
    assetId: form.assetId,
    database: form.database,
    schema: form.schema,
    objectName: form.objectName,
    objectType: form.objectType,
    keyword: expert.keyword,
    sql: form.sql,
    range: form.range,
    step: form.step,
    page: expert.page,
    pageSize: expert.pageSize,
    confirmed
  }
  Object.keys(payload).forEach(key => {
    if (payload[key] === '' || payload[key] === undefined || payload[key] === null) delete payload[key]
  })
  return payload
}

function startLoadingStages() {
  stopLoadingStages()
  loadingStage.value = 0
  loadingTimer = setInterval(() => {
    loadingStage.value = (loadingStage.value + 1) % loadingStages.length
  }, 1400)
}

function stopLoadingStages() {
  if (loadingTimer) {
    clearInterval(loadingTimer)
    loadingTimer = null
  }
}

async function scrollMessagesToBottom() {
  await nextTick()
  const el = messageListRef.value
  if (el) el.scrollTop = el.scrollHeight
}

async function loadAssets(keyword = '') {
  assetLoading.value = true
  try {
    const payload = unwrap(await dbApi.getCrudAssets({
      page: 1,
      pageSize: 100,
      keyword: keyword || undefined
    }))
    assets.value = normalizeList(payload).filter(item => supportedEngines.has(getAssetEngine(item)))
    if (!form.assetId && assets.value.length) form.assetId = assets.value[0].id
  } catch (error) {
    ElMessage.error(error.message || '资产列表获取失败')
  } finally {
    assetLoading.value = false
  }
}

function searchAssets(keyword = '') {
  loadAssets(keyword)
}

function handleAssetVisible(visible) {
  if (visible) loadAssets()
}

async function loadTools() {
  toolsLoading.value = true
  try {
    tools.value = normalizeList(unwrap(await dbApi.getDbAiTools()))
  } catch (error) {
    tools.value = fallbackTools
    ElMessage.warning(error.message || '工具清单获取失败，已使用内置工具定义')
  } finally {
    toolsLoading.value = false
  }
}

function fillQuestion(question) {
  form.question = question
}

function askTemplate(template) {
  form.question = template.question
  sendQuestion(template.question)
}

function clearMessages() {
  messages.value = []
}

function selectTool(tool) {
  expert.tool = tool
  activeToolTab.value = 'overview'
}

async function sendQuestion(question = form.question) {
  const finalQuestion = String(question || '').trim()
  if (!form.assetId) {
    ElMessage.warning('请先选择数据库资产')
    return
  }
  if (!finalQuestion) {
    ElMessage.warning('请输入要诊断的问题')
    return
  }

  lastQuestion.value = finalQuestion
  messages.value.push({
    id: `user-${Date.now()}`,
    role: 'user',
    content: finalQuestion,
    time: new Date().toLocaleTimeString()
  })
  form.question = ''
  chatLoading.value = true
  startLoadingStages()
  await scrollMessagesToBottom()

  try {
    const payload = normalizeDbAiChatPayload(unwrap(await dbApi.askDbAiChat(buildChatPayload(finalQuestion))))
    messages.value.push({
      id: `assistant-${Date.now()}`,
      role: 'assistant',
      content: payload.answer || payload.summary || '诊断完成',
      payload,
      question: finalQuestion,
      time: new Date().toLocaleTimeString()
    })
  } catch (error) {
    const friendlyError = buildFriendlyChatError(error)
    messages.value.push({
      id: `assistant-${Date.now()}`,
      role: 'assistant',
      content: friendlyError.answer,
      payload: friendlyError,
      isError: true,
      question: finalQuestion,
      time: new Date().toLocaleTimeString()
    })
    ElMessage.error(friendlyError.summary)
  } finally {
    chatLoading.value = false
    stopLoadingStages()
    scrollMessagesToBottom()
  }
}

function retryLastQuestion() {
  if (lastQuestion.value) sendQuestion(lastQuestion.value)
}

async function runTool(confirmed = expert.confirmed) {
  if (!form.assetId) {
    ElMessage.warning('请选择数据库资产')
    return
  }
  if (needsSql.value && !String(form.sql || '').trim()) {
    ElMessage.warning('请输入 SQL 或 Redis 只读命令')
    return
  }

  toolLoading.value = true
  try {
    const payload = unwrap(await dbApi.runDbAiTool(buildToolPayload(confirmed)))
    toolResult.value = payload
    activeToolTab.value = toolTableRows.value.length ? 'table' : 'overview'
    if (payload?.data?.requireConfirm && !confirmed) await confirmHighRiskQuery(payload)
  } catch (error) {
    ElMessage.error(error.message || '工具执行失败')
  } finally {
    toolLoading.value = false
  }
}

async function confirmHighRiskQuery(payload) {
  const message = payload?.data?.estimate?.message || payload.summary || '本次只读查询需要确认后执行'
  try {
    await ElMessageBox.confirm(message, '确认执行只读查询', {
      confirmButtonText: '确认执行',
      cancelButtonText: '取消',
      type: 'warning'
    })
    expert.confirmed = true
    await runTool(true)
  } catch (error) {
    // user cancelled
  }
}

async function copyAnswer() {
  const payload = latestAssistant.value?.payload
  if (!payload) return
  try {
    await navigator.clipboard.writeText([payload.summary, payload.answer].filter(Boolean).join('\n\n'))
    ElMessage.success('回答已复制')
  } catch (error) {
    ElMessage.error('复制失败')
  }
}

async function copyError(payload = {}) {
  try {
    await navigator.clipboard.writeText(formatJson({
      summary: payload.summary,
      answer: payload.answer,
      warnings: payload.warnings,
      suggestions: payload.suggestions,
      missingEvidence: payload.missingEvidence,
      evidence: payload.evidence
    }))
    ElMessage.success('错误信息已复制')
  } catch (error) {
    ElMessage.error('复制失败')
  }
}

async function copyToolResult() {
  try {
    await navigator.clipboard.writeText(formatJson(toolResult.value))
    ElMessage.success('结果已复制')
  } catch (error) {
    ElMessage.error('复制失败')
  }
}

onMounted(() => {
  loadAssets()
  loadTools()
})

onBeforeUnmount(() => {
  stopLoadingStages()
})
</script>

<style scoped>
.db-ai-page {
  --aiops-bg-page: #f4f7fb;
  --aiops-bg-page-soft: #eef3f9;
  --aiops-surface-warm: rgba(255, 253, 248, 0.92);
  --aiops-surface-strong: rgba(255, 255, 255, 0.9);
  --aiops-text-primary: #172133;
  --aiops-text-secondary: #5f6b7c;
  --aiops-text-muted: #8d97a8;
  --aiops-border-soft: rgba(22, 34, 56, 0.06);
  --aiops-border-cool: #dfe7f2;
  --aiops-border-mid: rgba(207, 217, 230, 0.92);
  --aiops-brand: #4c8bf5;
  --aiops-brand-soft: rgba(76, 139, 245, 0.12);
  --aiops-success: #3fb97f;
  --aiops-warning: #f3b24b;
  --aiops-danger: #ef6b73;
  height: calc(100vh - 92px);
  min-height: 640px;
  padding: 8px;
  overflow: hidden;
  background: linear-gradient(180deg, #eef3f9 0%, var(--aiops-bg-page) 100%);
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.db-ai-shell {
  display: flex;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  flex-direction: column;
  border: 1px solid var(--aiops-border-soft);
  border-radius: 26px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.96) 0%, rgba(248, 251, 255, 0.94) 100%);
  box-shadow: 0 20px 60px rgba(17, 24, 39, 0.07);
}

.db-ai-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 18px 22px 14px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.78);
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
}

.header-copy span,
.panel-heading span,
.chat-toolbar span:first-child {
  display: block;
  color: var(--aiops-text-muted);
  font-size: 10px;
  font-weight: 700;
}

.header-copy h1 {
  margin: 4px 0 0;
  font-size: 24px;
  font-weight: 700;
}

.header-copy p {
  margin: 5px 0 0;
  color: var(--aiops-text-secondary);
  font-size: 13px;
}

.header-actions,
.toolbar-actions,
.composer-actions,
.expert-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.mode-chip {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 7px 11px;
  border: 1px solid rgba(63, 185, 127, 0.22);
  border-radius: 999px;
  background: rgba(239, 252, 245, 0.82);
  color: #2e8f63;
  font-size: 12px;
  font-weight: 700;
}

.mode-chip i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--aiops-success);
}

.assistant-workspace {
  display: grid;
  flex: 1;
  grid-template-columns: 260px minmax(620px, 1fr) 300px;
  gap: 12px;
  min-height: 0;
  overflow: hidden;
  padding: 12px;
}

.surface-panel {
  min-height: 0;
  border: 1px solid rgba(207, 217, 230, 0.78);
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.72);
}

.context-panel,
.evidence-panel {
  display: flex;
  overflow: auto;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
}

.chat-panel {
  display: flex;
  overflow: hidden;
  flex-direction: column;
  min-width: 0;
}

.panel-heading,
.chat-toolbar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.panel-heading.compact {
  margin-bottom: 8px;
}

.panel-heading h2,
.chat-toolbar h2 {
  margin: 4px 0 0;
  font-size: 16px;
  font-weight: 700;
}

.context-form :deep(.el-form-item) {
  margin-bottom: 12px;
}

.inline-fields,
.expert-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  width: 100%;
}

.asset-option {
  display: grid;
  grid-template-columns: minmax(92px, 1fr) auto minmax(90px, 0.8fr);
  gap: 8px;
  align-items: center;
}

.asset-option span,
.asset-option strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.target-card,
.range-card,
.status-card,
.tool-result {
  border: 1px solid rgba(223, 231, 242, 0.78);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.7);
  padding: 12px;
}

.target-card {
  background: linear-gradient(135deg, rgba(255, 249, 240, 0.9), rgba(248, 251, 255, 0.82));
}

.target-card strong {
  display: block;
  overflow: hidden;
  color: var(--aiops-text-primary);
  font-size: 15px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.target-card p {
  margin: 6px 0 0;
  overflow: hidden;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.target-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 10px;
}

.target-meta span,
.missing-list span {
  padding: 5px 8px;
  border-radius: 999px;
  background: rgba(76, 139, 245, 0.08);
  color: #3f6fc4;
  font-size: 11px;
  font-weight: 700;
}

.range-card > span {
  display: block;
  margin-bottom: 8px;
  color: var(--aiops-text-muted);
  font-size: 11px;
  font-weight: 700;
}

.range-card > div {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.context-collapse,
.evidence-collapse {
  border-top: 1px solid rgba(223, 231, 242, 0.72);
  border-bottom: 0;
}

.context-collapse :deep(.el-collapse-item__header),
.evidence-collapse :deep(.el-collapse-item__header) {
  height: 38px;
  border-bottom: 0;
  background: transparent;
  color: var(--aiops-text-primary);
  font-weight: 700;
}

.context-collapse :deep(.el-collapse-item__wrap),
.evidence-collapse :deep(.el-collapse-item__wrap) {
  border-bottom: 0;
  background: transparent;
}

.prompt-section {
  padding-top: 4px;
  border-top: 1px solid rgba(223, 231, 242, 0.72);
}

.tool-item {
  width: 100%;
  border: 1px solid rgba(223, 231, 242, 0.78);
  background: rgba(255, 255, 255, 0.68);
  color: inherit;
  cursor: pointer;
  text-align: left;
  transition: border-color 0.2s ease, background 0.2s ease, transform 0.2s ease;
}

.prompt-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.prompt-buttons :deep(.el-button) {
  height: 30px;
  margin: 0;
  border-color: rgba(235, 176, 58, 0.38);
  border-radius: 8px;
  background: linear-gradient(180deg, rgba(255, 250, 232, 0.98) 0%, rgba(255, 241, 196, 0.94) 100%);
  color: #8a5b12;
  font-size: 12px;
  font-weight: 700;
  box-shadow: 0 2px 6px rgba(167, 117, 24, 0.08);
}

.prompt-buttons :deep(.el-button:hover),
.prompt-buttons :deep(.el-button:focus) {
  border-color: rgba(222, 157, 36, 0.62);
  background: linear-gradient(180deg, #fff7d8 0%, #ffe9a6 100%);
  color: #70480d;
  transform: translateY(-1px);
}

.prompt-buttons :deep(.el-button:active) {
  transform: translateY(0);
}

.tool-item:hover {
  border-color: rgba(76, 139, 245, 0.3);
  background: rgba(248, 251, 255, 0.92);
  transform: translateY(-1px);
}

.tool-item strong {
  display: block;
  color: var(--aiops-text-primary);
  font-size: 13px;
  font-weight: 700;
}

.tool-item span {
  display: -webkit-box;
  margin-top: 5px;
  overflow: hidden;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.45;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.chat-toolbar {
  flex: 0 0 auto;
  padding: 14px 16px 10px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.72);
}

.toolbar-actions {
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.message-list {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 12px 14px;
}

.records-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}

.records-strip button {
  max-width: 260px;
  overflow: hidden;
  padding: 6px 9px;
  border: 1px solid rgba(223, 231, 242, 0.78);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.72);
  color: var(--aiops-text-secondary);
  cursor: pointer;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.message-item {
  display: grid;
  grid-template-columns: 30px minmax(0, 1fr);
  gap: 8px;
  margin-top: 10px;
}

.message-avatar {
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border-radius: 12px;
  background: rgba(95, 107, 124, 0.1);
  color: var(--aiops-text-secondary);
  font-size: 12px;
  font-weight: 700;
  overflow: hidden;
}

.message-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

.user-avatar {
  border: 1px solid rgba(76, 139, 245, 0.22);
  background: rgba(76, 139, 245, 0.08);
}

.message-item.assistant .message-avatar {
  background: var(--aiops-brand-soft);
  color: var(--aiops-brand);
}

.assistant-avatar {
  border: 1px solid rgba(76, 139, 245, 0.16);
  background: rgba(248, 251, 255, 0.9);
}

.message-body {
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid rgba(223, 231, 242, 0.72);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.78);
}

.message-item.user .message-body {
  border-color: rgba(76, 139, 245, 0.18);
  background: rgba(238, 246, 255, 0.86);
  box-shadow: inset 3px 0 0 rgba(76, 139, 245, 0.35);
}

.message-item.user .message-meta strong {
  color: #3f6fc4;
}

.message-item.assistant .message-body {
  background: rgba(255, 255, 255, 0.82);
}

.message-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.message-meta strong {
  color: var(--aiops-text-primary);
  font-size: 13px;
}

.question-text {
  margin: 0;
  color: var(--aiops-text-primary);
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
}

.answer-summary,
.status-card {
  margin-bottom: 8px;
  padding: 7px 10px;
  border-radius: 12px;
  background: rgba(239, 252, 245, 0.82);
  color: #2e8f63;
}

.answer-summary {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.answer-summary strong,
.status-card strong {
  font-size: 12px;
}

.answer-summary span {
  flex: 0 0 auto;
  color: currentColor;
  font-size: 11px;
}

.tone-warning {
  background: rgba(255, 248, 238, 0.92);
  color: #b7791f;
}

.tone-critical,
.tone-danger {
  background: rgba(255, 241, 242, 0.92);
  color: #c54852;
}

.tone-unknown {
  background: rgba(244, 247, 251, 0.95);
  color: var(--aiops-text-secondary);
}

.answer-report,
.error-brief {
  display: grid;
  gap: 6px;
}

.answer-report section {
  padding: 7px 9px 7px 10px;
  border: 0;
  border-left: 3px solid rgba(76, 139, 245, 0.18);
  border-radius: 10px;
  background: rgba(248, 251, 255, 0.48);
}

.answer-report h3,
.error-brief strong {
  display: inline-flex;
  align-items: center;
  height: 20px;
  margin: 0 0 4px;
  color: var(--aiops-text-primary);
  font-size: 12px;
  font-weight: 700;
}

.markdown-content {
  color: var(--aiops-text-primary);
  font-size: 13px;
  line-height: 1.62;
}

.markdown-content :deep(p) {
  margin: 0 0 7px;
  white-space: pre-wrap;
}

.markdown-content :deep(p:last-child) {
  margin-bottom: 0;
}

.markdown-content :deep(h3),
.markdown-content :deep(h4),
.markdown-content :deep(h5) {
  margin: 9px 0 5px;
  color: var(--aiops-text-primary);
  font-size: 13px;
  font-weight: 750;
  line-height: 1.45;
}

.markdown-content :deep(h3:first-child),
.markdown-content :deep(h4:first-child),
.markdown-content :deep(h5:first-child) {
  margin-top: 0;
}

.markdown-content :deep(strong) {
  color: var(--aiops-text-primary);
  font-weight: 750;
}

.markdown-content :deep(ul),
.markdown-content :deep(ol) {
  margin: 4px 0 8px;
  padding-left: 18px;
  color: var(--aiops-text-primary);
}

.markdown-content :deep(li) {
  margin: 3px 0;
  line-height: 1.58;
}

.markdown-content :deep(code) {
  display: inline-block;
  max-width: min(100%, 42ch);
  overflow: hidden;
  padding: 1px 5px;
  border-radius: 6px;
  background: rgba(76, 139, 245, 0.08);
  color: #355fa7;
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
  text-overflow: ellipsis;
  vertical-align: bottom;
  white-space: nowrap;
}

.markdown-content :deep(.markdown-code) {
  max-height: 260px;
  margin: 7px 0 9px;
  overflow: auto;
  padding: 9px 10px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  border-radius: 10px;
  background: #f8fafc;
  color: #27364a;
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre;
}

.markdown-content :deep(.markdown-code code) {
  display: block;
  max-width: none;
  overflow: visible;
  padding: 0;
  border-radius: 0;
  background: transparent;
  color: inherit;
  font-size: inherit;
  text-overflow: clip;
  white-space: pre;
}

.markdown-content :deep(.markdown-table-wrap) {
  display: inline-block;
  max-width: 100%;
  margin: 8px 0 10px;
  overflow-x: auto;
  vertical-align: top;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.82);
}

.markdown-content :deep(.markdown-table) {
  width: max-content;
  max-width: none;
  border-collapse: collapse;
  table-layout: fixed;
  color: var(--aiops-text-primary);
  font-size: 12px;
  line-height: 1.45;
}

.markdown-content :deep(.markdown-table th),
.markdown-content :deep(.markdown-table td) {
  overflow: hidden;
  padding: 6px 7px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.78);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.markdown-content :deep(.markdown-table th > span),
.markdown-content :deep(.markdown-table td > span) {
  display: block;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.markdown-content :deep(.markdown-table th) {
  background: rgba(238, 246, 255, 0.86);
  color: #355fa7;
  font-weight: 750;
  text-align: left;
}

.markdown-content :deep(.markdown-table tbody tr:last-child td) {
  border-bottom: 0;
}

.markdown-content :deep(.markdown-table tbody tr:hover td) {
  background: rgba(248, 251, 255, 0.88);
}

.markdown-content :deep(.markdown-table col.col-rank) {
  width: 44px;
}

.markdown-content :deep(.markdown-table col.col-risk) {
  width: 54px;
}

.markdown-content :deep(.markdown-table col.col-number) {
  width: 82px;
}

.markdown-content :deep(.markdown-table col.col-sql) {
  width: 260px;
}

.markdown-content :deep(.markdown-table .col-rank),
.markdown-content :deep(.markdown-table .col-risk),
.markdown-content :deep(.markdown-table .col-number) {
  font-variant-numeric: tabular-nums;
}

.markdown-content :deep(.markdown-table .col-number),
.markdown-content :deep(.markdown-table .align-right) {
  text-align: right;
}

.markdown-content :deep(.markdown-table .col-sql code) {
  max-width: 100%;
  overflow: hidden;
  padding: 0;
  background: transparent;
  color: inherit;
  text-overflow: ellipsis;
}

.answer-report p,
.error-brief p {
  margin: 0;
  color: var(--aiops-text-primary);
  font-size: 13px;
  line-height: 1.58;
  white-space: pre-wrap;
}

.answer-report ul,
.answer-report ol {
  margin: 0;
  padding-left: 16px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.55;
}

.answer-report li + li {
  margin-top: 2px;
}

.answer-report li button {
  padding: 0;
  border: 0;
  background: transparent;
  color: #3f6fc4;
  cursor: pointer;
  font: inherit;
  text-align: left;
}

.error-brief {
  padding: 11px;
  border: 1px solid rgba(255, 192, 120, 0.58);
  border-radius: 14px;
  background: rgba(255, 248, 238, 0.74);
}

.error-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.agent-trace {
  display: grid;
  gap: 8px;
}

.agent-trace div,
.call-list div {
  padding: 10px 11px;
  border: 1px solid rgba(223, 231, 242, 0.78);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.68);
}

.agent-trace span {
  display: block;
  color: var(--aiops-text-muted);
  font-size: 11px;
  font-weight: 700;
}

.agent-trace strong {
  display: block;
  margin-top: 5px;
  color: var(--aiops-text-primary);
  font-size: 13px;
  line-height: 1.5;
}

.call-list,
.missing-list {
  display: grid;
  gap: 8px;
}

.call-list div {
  display: flex;
  align-items: center;
  gap: 8px;
}

.call-list span {
  color: var(--aiops-success);
  font-weight: 800;
}

.call-list strong {
  font-size: 13px;
}

.missing-list {
  align-items: start;
}

.error-detail p {
  margin: 0 0 7px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.error-detail pre {
  max-height: 160px;
  margin: 8px 0;
  overflow: auto;
  padding: 6px 9px;
  border: 1px solid rgba(223, 231, 242, 0.78);
  border-radius: 12px;
  background: #f8fafc;
  color: #27364a;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
  white-space: pre-wrap;
}

.thinking-message {
  margin-top: 10px;
}

.thinking-body {
  display: inline-flex;
  align-items: center;
  width: fit-content;
  gap: 8px;
  padding: 8px 11px;
  border-radius: 999px;
  background: rgba(248, 251, 255, 0.86);
}

.thinking-body strong {
  color: var(--aiops-text-primary);
  font-size: 12px;
}

.thinking-body span {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.thinking-dots {
  display: inline-flex;
  align-items: center;
  gap: 3px;
}

.thinking-dots i {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--aiops-brand);
  animation: thinking-pulse 1.1s ease-in-out infinite;
}

.thinking-dots i:nth-child(2) {
  animation-delay: 0.16s;
}

.thinking-dots i:nth-child(3) {
  animation-delay: 0.32s;
}

@keyframes thinking-pulse {
  0%,
  80%,
  100% {
    opacity: 0.35;
    transform: translateY(0);
  }

  40% {
    opacity: 1;
    transform: translateY(-2px);
  }
}

.composer {
  display: grid;
  flex: 0 0 auto;
  gap: 8px;
  padding: 12px 14px 14px;
  border-top: 1px solid rgba(223, 231, 242, 0.72);
  background: rgba(248, 251, 255, 0.66);
}

.safe-note {
  flex: 1;
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.evidence-panel :deep(.el-empty) {
  padding: 22px 0;
}

.status-card {
  display: grid;
  gap: 6px;
  margin-bottom: 0;
}

.status-card span {
  font-size: 12px;
  font-weight: 700;
}

.side-section {
  padding-top: 12px;
  border-top: 1px solid rgba(223, 231, 242, 0.72);
}

.side-section h3 {
  margin: 0 0 8px;
  color: var(--aiops-text-primary);
  font-size: 14px;
}

.warning-stack {
  display: grid;
  gap: 8px;
}

.evidence-list {
  display: grid;
  gap: 8px;
}

.evidence-item {
  padding: 10px 11px;
  border: 1px solid rgba(223, 231, 242, 0.78);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.68);
}

.evidence-item div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.evidence-item strong {
  color: var(--aiops-text-primary);
  font-size: 13px;
}

.evidence-item p,
.evidence-item small {
  display: block;
  margin: 6px 0 0;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.expert-collapse {
  margin-top: auto;
  border-top: 1px solid rgba(223, 231, 242, 0.72);
  border-bottom: 0;
}

.expert-collapse :deep(.el-collapse-item__header) {
  height: 42px;
  border-bottom: 0;
  background: transparent;
  color: var(--aiops-text-primary);
  font-weight: 700;
}

.expert-collapse :deep(.el-collapse-item__wrap) {
  border-bottom: 0;
  background: transparent;
}

.tool-picker {
  display: grid;
  grid-template-columns: 1fr;
  gap: 8px;
}

.tool-item {
  min-height: 66px;
  padding: 9px 10px;
  border-radius: 14px;
}

.tool-item.active {
  border-color: rgba(76, 139, 245, 0.42);
  background: rgba(76, 139, 245, 0.08);
  box-shadow: inset 3px 0 0 var(--aiops-brand);
}

.expert-form {
  margin-top: 10px;
}

.expert-form :deep(.el-form-item) {
  margin-bottom: 10px;
}

.expert-actions span {
  flex: 1;
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.tool-result {
  margin-top: 12px;
  padding: 11px;
}

.tool-result-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 8px;
}

.tool-result-head strong {
  min-width: 0;
  overflow: hidden;
  color: var(--aiops-text-primary);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.kv-grid,
.metric-grid {
  display: grid;
  gap: 8px;
}

.kv-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.metric-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  margin-top: 8px;
}

.kv-grid div,
.mini-metric {
  min-width: 0;
  padding: 9px 10px;
  border: 1px solid rgba(223, 231, 242, 0.78);
  border-radius: 13px;
  background: rgba(255, 255, 255, 0.62);
}

.kv-grid span,
.mini-metric span {
  display: block;
  color: var(--aiops-text-muted);
  font-size: 11px;
  font-weight: 700;
}

.kv-grid strong,
.mini-metric strong {
  display: block;
  margin-top: 5px;
  overflow: hidden;
  color: var(--aiops-text-primary);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mini-metric {
  box-shadow: inset 3px 0 0 var(--aiops-success);
}

.mini-metric.tone-warning {
  box-shadow: inset 3px 0 0 var(--aiops-warning);
}

.mini-metric.tone-critical,
.mini-metric.tone-danger {
  box-shadow: inset 3px 0 0 var(--aiops-danger);
}

.code-block {
  max-height: 280px;
  margin: 0;
  overflow: auto;
  padding: 12px;
  border: 1px solid rgba(223, 231, 242, 0.78);
  border-radius: 14px;
  background: #f8fafc;
  color: #27364a;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
  line-height: 1.65;
  white-space: pre-wrap;
}

@media (max-width: 1400px) {
  .assistant-workspace {
    grid-template-columns: 270px minmax(0, 1fr);
  }

  .evidence-panel {
    grid-column: 1 / -1;
    max-height: 280px;
  }
}

@media (max-width: 960px) {
  .db-ai-page {
    height: auto;
    min-height: calc(100vh - 92px);
    overflow: visible;
  }

  .db-ai-shell {
    height: auto;
    overflow: visible;
  }

  .db-ai-header,
  .header-actions,
  .composer-actions {
    align-items: flex-start;
    flex-direction: column;
  }

  .assistant-workspace {
    grid-template-columns: 1fr;
    overflow: visible;
  }

  .context-panel,
  .evidence-panel,
  .message-list {
    overflow: visible;
  }

  .kv-grid,
  .metric-grid,
  .inline-fields,
  .expert-grid {
    grid-template-columns: 1fr;
  }
}
</style>
