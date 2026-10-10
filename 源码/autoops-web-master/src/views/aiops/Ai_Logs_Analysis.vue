<template>
  <div class="log-analysis-page">
    <header class="ops-page-header">
      <div class="ops-page-header__identity">
        <span class="ops-page-header__icon" aria-hidden="true">
          <img :src="logSvg" alt="">
        </span>
        <div class="ops-page-header__content">
          <h1>AI 日志分析</h1>
          <span class="ops-page-header__divider" aria-hidden="true"></span>
          <p>{{ currentSourceLabel }}，{{ currentTargetLabel }}</p>
        </div>
      </div>

      <div class="ops-page-header__status">
        <div class="session-chip" :title="sessionId || '尚未创建会话'">
          <span>会话</span>
          <strong>{{ shortSessionId }}</strong>
        </div>
        <span class="stream-pill" :class="streamStateClass">
          <i aria-hidden="true"></i>
          {{ streamStateText }}
        </span>
        <div class="top-actions">
          <el-tooltip content="刷新会话状态" placement="bottom">
            <el-button
              class="header-icon-button ops-refresh-button"
              type="warning"
              :icon="Refresh"
              :disabled="!sessionId"
              aria-label="刷新会话状态"
              @click="refreshStatus"
            />
          </el-tooltip>
          <el-tooltip content="复制会话 ID" placement="bottom">
            <el-button
              class="header-icon-button"
              :icon="CopyDocument"
              :disabled="!sessionId"
              aria-label="复制会话 ID"
              @click="copySessionId"
            />
          </el-tooltip>
          <el-tooltip content="关闭当前会话" placement="bottom">
            <el-button
              class="header-icon-button"
              type="danger"
              :icon="Close"
              :disabled="!sessionId && !entries.length"
              aria-label="关闭当前会话"
              @click="closeCurrentSession"
            />
          </el-tooltip>
        </div>
      </div>
    </header>

    <div class="workspace">
      <aside class="resource-panel">
        <el-tabs v-model="resourceTab" stretch class="resource-tabs">
          <el-tab-pane label="K8s" name="k8s">
            <K8sLogSourcePanel v-model="k8sSource" @change="handleK8sSourceChange" @query="runQuery" />
          </el-tab-pane>

          <el-tab-pane label="主机" name="host">
            <div class="resource-tools">
              <el-input
                v-model.trim="hostKeyword"
                clearable
                placeholder="主机名 / IP"
                @keyup.enter="loadHosts(hostKeyword)"
                @clear="loadHosts()"
              >
                <template #prefix>
                  <el-icon><Search /></el-icon>
                </template>
              </el-input>
              <el-tooltip content="刷新主机列表" placement="bottom">
                <el-button
                  :icon="Refresh"
                  :loading="hostsLoading"
                  aria-label="刷新主机列表"
                  @click="loadHosts(hostKeyword)"
                />
              </el-tooltip>
            </div>

            <el-scrollbar class="resource-scroll">
              <el-tree
                ref="hostTreeRef"
                :data="hostTreeData"
                node-key="id"
                :props="treeProps"
                :highlight-current="true"
                :expand-on-click-node="false"
                default-expand-all
                class="resource-tree"
                @node-click="handleResourceNodeClick"
              >
                <template #default="{ data }">
                  <div class="tree-node" :class="`type-${data.type}`" @dblclick.stop="handleResourceNodeDoubleClick(data)">
                    <el-icon>
                      <Monitor v-if="data.type === 'host'" />
                      <FolderOpened v-else-if="data.type === 'dir'" />
                      <Document v-else />
                    </el-icon>
                    <span>{{ data.label }}</span>
                    <small v-if="data.meta">{{ data.meta }}</small>
                  </div>
                </template>
              </el-tree>
            </el-scrollbar>

            <div class="scan-box">
              <el-form label-position="top" @submit.prevent>
                <el-form-item label="SSH 凭据" v-if="authOptions.length">
                  <el-select v-model="selectedAuthId" clearable placeholder="默认凭据">
                    <el-option
                      v-for="auth in authOptions"
                      :key="auth.authId"
                      :label="getAuthLabel(auth)"
                      :value="auth.authId"
                    />
                  </el-select>
                </el-form-item>
                <el-form-item label="扫描目录">
                  <el-input v-model.trim="fileQuery.dir" placeholder="/var/log" />
                </el-form-item>
                <el-form-item label="文件搜索">
                  <el-input
                    v-model.trim="fileQuery.keyword"
                    clearable
                    placeholder="messages / nginx / error"
                    @keyup.enter="loadLogFiles"
                  />
                </el-form-item>
                <div class="scan-options">
                  <el-input-number v-model="fileQuery.maxDepth" :min="1" :max="4" controls-position="right" />
                  <el-input-number v-model="fileQuery.limit" :min="20" :max="1000" :step="20" controls-position="right" />
                </div>
                <el-button
                  type="primary"
                  :icon="FolderOpened"
                  :loading="filesLoading"
                  :disabled="!selectedHostId"
                  @click="loadLogFiles"
                >
                  扫描日志文件
                </el-button>
              </el-form>
            </div>
          </el-tab-pane>

          <el-tab-pane label="会话" name="session">
            <div class="resource-tools">
              <el-switch v-model="sessionsActiveOnly" active-text="活跃" inactive-text="全部" @change="loadSessions" />
              <el-tooltip content="刷新会话列表" placement="bottom">
                <el-button
                  :icon="Refresh"
                  :loading="sessionsLoading"
                  aria-label="刷新会话列表"
                  @click="loadSessions"
                />
              </el-tooltip>
            </div>
            <el-scrollbar class="resource-scroll sessions-scroll">
              <button
                v-for="session in sessions"
                :key="session.sessionId"
                type="button"
                class="session-node"
                :class="{ active: sessionId === session.sessionId }"
                @click="useSession(session)"
              >
                <strong>{{ session.sessionId }}</strong>
                <span>{{ getSessionSourceTitle(session) }}</span>
                <small>{{ getSessionSourceTarget(session) }}</small>
                <em>{{ session.totalLogs || 0 }} 条 · {{ session.active ? '运行中' : '已停止' }}</em>
              </button>
              <el-empty v-if="!sessions.length && !sessionsLoading" description="暂无会话" :image-size="72" />
            </el-scrollbar>
          </el-tab-pane>
        </el-tabs>
      </aside>

      <main class="log-workspace">
        <section class="query-toolbar">
          <div class="query-filters">
            <el-input
              v-model.trim="queryForm.keyword"
              clearable
              placeholder="输入关键词检索日志"
              class="toolbar-input"
              @keyup.enter="runQuery"
            >
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>
            <el-select v-model="queryForm.levels" multiple clearable collapse-tags placeholder="日志级别" class="toolbar-level">
              <el-option v-for="level in levelOptions" :key="level" :label="level" :value="level" />
            </el-select>
            <el-select v-model="queryForm.tailLines" class="toolbar-number" placeholder="最近行数">
              <el-option label="最近 100 行" :value="100" />
              <el-option label="最近 500 行" :value="500" />
              <el-option label="最近 1000 行" :value="1000" />
              <el-option label="最近 5000 行" :value="5000" />
            </el-select>
          </div>
          <div class="query-actions">
            <el-button type="primary" :icon="Search" :loading="queryLoading" @click="runQuery">查询</el-button>
            <el-button
              v-if="!streamConnected"
              type="success"
              :icon="VideoPlay"
              :loading="streamConnecting"
              @click="startStream"
            >
              实时流
            </el-button>
            <el-button v-else type="warning" :icon="VideoPause" @click="stopStream">暂停</el-button>
            <el-button class="ai-analysis-button" :icon="MagicStick" :disabled="!canAnalyze" @click="quickAnalyze">
              AI 分析
            </el-button>
            <el-tooltip content="清空当前日志" placement="bottom">
              <el-button
                class="query-icon-button"
                type="danger"
                :icon="Delete"
                aria-label="清空当前日志"
                @click="clearLogs"
              />
            </el-tooltip>
          </div>
        </section>

        <el-collapse class="advanced-collapse">
          <el-collapse-item name="parser">
            <template #title>
              <span class="advanced-title">高级条件</span>
              <small class="advanced-summary">返回上限、正则匹配与日志解析格式</small>
            </template>
            <div class="advanced-grid">
              <label class="advanced-field">
                <span>返回上限</span>
                <el-select v-model="queryForm.limit" placeholder="返回上限">
                  <el-option label="返回 200 条" :value="200" />
                  <el-option label="返回 1000 条" :value="1000" />
                  <el-option label="返回 5000 条" :value="5000" />
                </el-select>
              </label>
              <label class="advanced-field">
                <span>正则表达式</span>
                <el-input v-model.trim="queryForm.regex" clearable placeholder="例如 error|timeout" @keyup.enter="runQuery" />
              </label>
              <label class="advanced-field">
                <span>日志格式</span>
                <el-select v-model="queryForm.formatName" filterable allow-create placeholder="auto">
                  <el-option label="auto" value="auto" />
                  <el-option label="json" value="json" />
                  <el-option label="logfmt" value="logfmt" />
                  <el-option label="text" value="text" />
                </el-select>
              </label>
              <label class="advanced-field advanced-field--parser">
                <span>自定义解析 YAML</span>
                <el-input
                  v-model="queryForm.formatYaml"
                  type="textarea"
                  :rows="5"
                  resize="vertical"
                  placeholder="name: custom-app&#10;type: text&#10;pattern:"
                />
              </label>
            </div>
          </el-collapse-item>
        </el-collapse>

        <section class="log-main">
          <div class="log-head">
            <div>
              <strong>实时日志</strong>
              <span>展示 {{ visibleEntries.length }} / 缓存 {{ entries.length }} 条</span>
            </div>
            <div class="log-head-actions">
              <el-switch v-model="autoScroll" active-text="自动滚动" />
              <el-tooltip content="复制当前日志" placement="bottom">
                <el-button
                  class="copy-icon-button"
                  :icon="CopyDocument"
                  aria-label="复制当前日志"
                  @click="copyVisibleLogs"
                />
              </el-tooltip>
            </div>
          </div>

          <div
            ref="virtualViewport"
            class="virtual-log"
            @scroll="handleLogScroll"
          >
            <div class="virtual-spacer" :style="{ height: `${totalLogHeight}px` }">
              <div class="virtual-window" :style="{ transform: `translateY(${virtualOffset}px)` }">
                <button
                  v-for="item in virtualRows"
                  :key="getEntryKey(item.entry, item.index)"
                  type="button"
                  class="log-line"
                  :class="rowLevelClass(item.entry.level)"
                  @click="openEntryDetail(item.entry)"
                >
                  <span class="line-no">#{{ item.entry.lineNo || item.index + 1 }}</span>
                  <span class="line-level">{{ normalizeLevel(item.entry.level) }}</span>
                  <span class="line-time">{{ item.entry.timestamp || '-' }}</span>
                  <code v-html="highlightLogText(item.entry.message || item.entry.raw || '-')"></code>
                </button>
              </div>
            </div>
            <el-empty v-if="!entries.length" description="请选择日志文件后查询或启动实时流" :image-size="92" />
          </div>
        </section>
      </main>

      <aside class="analysis-panel">
        <el-tabs v-model="analysisTab" class="analysis-tabs">
          <el-tab-pane label="概览" name="overview">
            <div class="metric-grid">
              <div>
                <span>总日志</span>
                <strong>{{ status?.totalLogs ?? summaryTotal }}</strong>
              </div>
              <div>
                <span>匹配数</span>
                <strong>{{ summaryMatched }}</strong>
              </div>
              <div>
                <span>缓冲</span>
                <strong>{{ status?.bufferUsed ?? '-' }}</strong>
              </div>
              <div>
                <span>字节</span>
                <strong>{{ formatBytes(status?.totalBytes) }}</strong>
              </div>
            </div>

            <div class="chart-section">
              <div class="section-head">
                <strong>级别分布</strong>
              </div>
              <div class="level-bars">
                <div v-for="item in levelCountItems" :key="item.level" class="level-bar">
                  <span>{{ item.level }}</span>
                  <div><i :style="{ width: `${item.percent}%` }" :class="levelClass(item.level)"></i></div>
                  <strong>{{ item.count }}</strong>
                </div>
                <el-empty v-if="!levelCountItems.length" description="暂无分布数据" :image-size="64" />
              </div>
            </div>

            <div class="chart-section">
              <div class="section-head">
                <strong>实时流状态</strong>
              </div>
              <div class="stream-list">
                <div v-for="stream in statusStreams" :key="stream.name" class="stream-item">
                  <i :class="{ active: stream.active }"></i>
                  <div>
                    <strong>{{ stream.name }}</strong>
                    <span>{{ stream.logCount || 0 }} 条 · {{ stream.lastSeen || '-' }}</span>
                  </div>
                </div>
                <span v-if="!statusStreams.length" class="empty-note">暂无实时流</span>
              </div>
            </div>
          </el-tab-pane>

          <el-tab-pane label="模式" name="patterns">
            <div class="chart-section">
              <div class="section-head">
                <strong>Top 日志模式</strong>
              </div>
              <button
                v-for="pattern in topPatterns"
                :key="pattern.pattern"
                type="button"
                class="pattern-card"
                @click="applyPatternFilter(pattern)"
              >
                <strong>{{ pattern.pattern }}</strong>
                <span>{{ pattern.count }} 次 · {{ pattern.percentage || 0 }}%</span>
                <small>{{ pattern.sample || '-' }}</small>
              </button>
              <el-empty v-if="!topPatterns.length" description="暂无模式数据" :image-size="64" />
            </div>

            <div class="chart-section">
              <div class="section-head">
                <strong>Top 关键词</strong>
              </div>
              <div class="keyword-list">
                <button
                  v-for="item in topKeywords"
                  :key="item.key"
                  type="button"
                  @click="applyKeywordFilter(item.key)"
                >
                  {{ item.key }}
                  <strong>{{ item.count }}</strong>
                </button>
                <span v-if="!topKeywords.length" class="empty-note">暂无关键词</span>
              </div>
            </div>

            <div class="chart-section">
              <div class="section-head">
                <strong>错误样本</strong>
              </div>
              <button
                v-for="sample in errorSamples"
                :key="sample"
                type="button"
                class="sample-card"
                @click="applyKeywordFilter(sample)"
              >
                {{ sample }}
              </button>
              <span v-if="!errorSamples.length" class="empty-note">暂无错误样本</span>
            </div>
          </el-tab-pane>

          <el-tab-pane label="AI" name="ai">
            <el-form label-position="top" @submit.prevent>
              <el-form-item label="模型配置">
                <el-select v-model="analysisForm.providerId" clearable filterable placeholder="默认 Provider">
                  <el-option
                    v-for="provider in providerOptions"
                    :key="provider.id"
                    :label="getProviderLabel(provider)"
                    :value="provider.id"
                  />
                </el-select>
              </el-form-item>
              <el-form-item label="上下文范围">
                <el-select v-model="analysisForm.contextSize">
                  <el-option label="最近 50 条" :value="50" />
                  <el-option label="最近 200 条" :value="200" />
                  <el-option label="最近 500 条" :value="500" />
                </el-select>
              </el-form-item>
              <el-form-item label="分析问题">
                <el-input
                  v-model="analysisForm.prompt"
                  type="textarea"
                  :rows="5"
                  resize="vertical"
                  placeholder="请分析当前日志的根因、影响范围和排查步骤"
                />
              </el-form-item>
              <el-button
                type="primary"
                :icon="MagicStick"
                :loading="analysisLoading"
                :disabled="!canAnalyze"
                @click="runAnalyze"
              >
                AI 分析日志
              </el-button>
            </el-form>

            <div v-if="analysisResult" class="ai-result">
              <div class="ai-meta">
                <span>{{ analysisResult.providerName || analysisResult.modelCode || 'AI 分析结果' }}</span>
                <small>{{ analysisResult.generatedAt || '-' }}</small>
              </div>
              <div class="ai-rendered">
                <section
                  v-for="(section, index) in aiSections"
                  :key="`${section.title}-${index}`"
                  class="ai-section"
                  :class="`tone-${section.tone}`"
                >
                  <div class="ai-section-head">
                    <span>{{ section.title }}</span>
                  </div>
                  <div class="ai-section-body">
                    <template v-for="(item, itemIndex) in section.items" :key="`${section.title}-${itemIndex}`">
                      <p v-if="item.type === 'paragraph'" class="ai-paragraph" v-html="item.html"></p>
                      <div v-else-if="item.type === 'bullet'" class="ai-list-row bullet">
                        <i></i>
                        <div v-html="item.html"></div>
                      </div>
                      <div v-else-if="item.type === 'ordered'" class="ai-list-row ordered">
                        <strong>{{ item.order }}</strong>
                        <div v-html="item.html"></div>
                      </div>
                    </template>
                  </div>
                </section>
              </div>
            </div>
          </el-tab-pane>
        </el-tabs>
      </aside>
    </div>

    <el-drawer v-model="entryDrawerVisible" title="日志详情" direction="rtl" size="520px" class="entry-drawer">
      <template v-if="selectedEntry">
        <el-descriptions :column="1" border size="small">
          <el-descriptions-item label="时间">{{ selectedEntry.timestamp || '-' }}</el-descriptions-item>
          <el-descriptions-item label="级别">{{ normalizeLevel(selectedEntry.level) }}</el-descriptions-item>
          <el-descriptions-item label="行号">{{ selectedEntry.lineNo || '-' }}</el-descriptions-item>
          <el-descriptions-item label="来源">{{ getEntrySourceLabel(selectedEntry) }}</el-descriptions-item>
        </el-descriptions>

        <div class="detail-block">
          <div>
            <strong>raw 原文</strong>
            <el-button link type="primary" @click="copyText(selectedEntry.raw || selectedEntry.message || '')">复制</el-button>
          </div>
          <pre>{{ selectedEntry.raw || selectedEntry.message || '-' }}</pre>
        </div>

        <div class="detail-block">
          <strong>fields</strong>
          <pre>{{ stringifyJson(selectedEntry.fields) }}</pre>
        </div>

        <div class="detail-block">
          <strong>attributes</strong>
          <pre>{{ stringifyJson(selectedEntry.attributes) }}</pre>
        </div>

        <div class="detail-block">
          <strong>source</strong>
          <pre>{{ stringifyJson(selectedEntry.source) }}</pre>
        </div>

        <el-button type="primary" :icon="MagicStick" :loading="analysisLoading" @click="analyzeSingleEntry">
          分析当前日志
        </el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Close,
  CopyDocument,
  Delete,
  Document,
  FolderOpened,
  MagicStick,
  Monitor,
  Refresh,
  Search,
  VideoPause,
  VideoPlay
} from '@element-plus/icons-vue'
import aiopsApi from '@/api/aiops'
import cmdbApi from '@/api/cmdb'
import logSvg from '@/assets/image/日志.svg'
import K8sLogSourcePanel from './components/K8sLogSourcePanel.vue'

const successCodes = new Set([0, 200])
const levelOptions = ['TRACE', 'DEBUG', 'INFO', 'NOTICE', 'WARN', 'WARNING', 'ERROR', 'FATAL']
const maxBufferedEntries = 5000
const rowHeight = 36
const overscan = 8

const resourceTab = ref('k8s')
const activeSourceType = ref('k8s')
const analysisTab = ref('overview')
const hostsLoading = ref(false)
const filesLoading = ref(false)
const queryLoading = ref(false)
const sessionsLoading = ref(false)
const analysisLoading = ref(false)
const streamConnecting = ref(false)
const streamConnected = ref(false)

const hostKeyword = ref('')
const hostOptions = ref([])
const selectedHostId = ref('')
const selectedAuthId = ref('')
const authOptions = ref([])
const fileList = ref([])
const selectedPath = ref('')
const k8sSource = ref({
  clusterId: '',
  namespace: '',
  pod: '',
  container: '',
  previous: false,
  timestamps: true,
  clusterName: ''
})
const sessionId = ref('')
const entries = ref([])
const summary = ref({})
const status = ref(null)
const sessions = ref([])
const providerOptions = ref([])
const analysisResult = ref(null)
const selectedEntry = ref(null)
const entryDrawerVisible = ref(false)
const autoScroll = ref(true)
const virtualViewport = ref(null)
const scrollTop = ref(0)
const viewportHeight = ref(560)
const sessionsActiveOnly = ref(true)
const hostTreeRef = ref(null)
const lastK8sSourceKey = ref('')

const fileQuery = reactive({
  dir: '/var/log',
  keyword: '',
  maxDepth: 2,
  limit: 200
})

const queryForm = reactive({
  tailLines: 500,
  limit: 1000,
  keyword: '',
  regex: '',
  levels: [],
  formatName: 'auto',
  formatYaml: ''
})

const analysisForm = reactive({
  providerId: '',
  contextSize: 200,
  prompt: '请分析当前日志中的根因判断、影响范围、风险等级和建议排查步骤。'
})

const treeProps = {
  children: 'children',
  label: 'label'
}

let streamSocket = null
let statusTimer = null

const getBody = response => response?.data || response || {}

const assertSuccess = (response, fallbackMessage) => {
  const body = getBody(response)
  if (body.code !== undefined && !successCodes.has(Number(body.code))) {
    throw new Error(body.message || body.msg || fallbackMessage)
  }
  return body
}

const getPayload = (response, fallbackMessage = '请求失败') => {
  const body = assertSuccess(response, fallbackMessage)
  return body.data !== undefined ? body.data : body
}

const toList = payload => {
  if (Array.isArray(payload)) return payload
  if (Array.isArray(payload?.list)) return payload.list
  if (Array.isArray(payload?.items)) return payload.items
  if (Array.isArray(payload?.records)) return payload.records
  if (Array.isArray(payload?.data)) return payload.data
  return []
}

const getHostId = host => host?.id ?? host?.hostId ?? host?.host_id ?? ''
const getHostName = host => host?.hostName || host?.hostname || host?.name || host?.instanceName || `Host ${getHostId(host)}`
const getHostIp = host => host?.sshIp || host?.privateIp || host?.publicIp || host?.ip || ''
const getHostType = host => host?.hostType ?? host?.host_type ?? host?.deviceType ?? host?.device_type

const isAuthSuccessHost = host => Number(host?.status) === 1

const isLinuxHost = host => {
  const hostType = getHostType(host)
  const numericHostType = Number(hostType)

  if (Number.isFinite(numericHostType)) {
    return numericHostType !== 2
  }

  const normalizedHostType = String(hostType || '').trim().toLowerCase()
  if (!normalizedHostType) return true
  return !['windows', 'win', '2'].includes(normalizedHostType)
}

const filterLoggableHosts = hosts => toList(hosts).filter(host => isAuthSuccessHost(host) && isLinuxHost(host))

const currentHost = computed(() =>
  hostOptions.value.find(host => String(getHostId(host)) === String(selectedHostId.value)) || null
)

const analysisText = computed(() =>
  String(analysisResult.value?.analysis || analysisResult.value?.result || analysisResult.value?.message || '').trim()
)

const currentHostLabel = computed(() => {
  if (!currentHost.value) return '未选择主机'
  const ip = getHostIp(currentHost.value)
  return ip ? `${getHostName(currentHost.value)} (${ip})` : getHostName(currentHost.value)
})

const isK8sSource = computed(() => activeSourceType.value === 'k8s')

const selectedK8sTargetLabel = computed(() => {
  const source = k8sSource.value || {}
  if (!source.clusterId) return '未选择 K8s 集群'
  if (!source.namespace) return '未选择命名空间'
  if (!source.pod) return '未选择 Pod'
  return source.container || '默认业务容器'
})

const currentSourceLabel = computed(() => {
  if (isK8sSource.value) {
    return k8sSource.value?.clusterName || (k8sSource.value?.clusterId ? `K8s 集群 ${k8sSource.value.clusterId}` : '未选择 K8s 集群')
  }
  return currentHostLabel.value
})

const currentTargetLabel = computed(() => {
  if (isK8sSource.value) {
    const source = k8sSource.value || {}
    if (!source.namespace && !source.pod) return selectedK8sTargetLabel.value
    return [source.namespace, source.pod, source.container || '默认业务容器'].filter(Boolean).join(' / ')
  }
  return selectedPath.value || '未选择日志文件'
})

const shortSessionId = computed(() => {
  if (!sessionId.value) return '-'
  return sessionId.value.length > 26 ? `${sessionId.value.slice(0, 10)}...${sessionId.value.slice(-10)}` : sessionId.value
})

const summaryTotal = computed(() => summary.value?.total ?? entries.value.length)
const summaryMatched = computed(() => summary.value?.matched ?? entries.value.length)
const statusStreams = computed(() => Array.isArray(status.value?.streams) ? status.value.streams : [])

const mergedLevelCounts = computed(() => status.value?.levelCounts || summary.value?.levelCounts || {})

const levelCountItems = computed(() => {
  const items = Object.entries(mergedLevelCounts.value || {})
    .map(([level, count]) => ({ level: normalizeLevel(level), count: Number(count) || 0 }))
    .sort((a, b) => b.count - a.count)
  const max = Math.max(...items.map(item => item.count), 1)
  return items.map(item => ({ ...item, percent: Math.round((item.count / max) * 100) }))
})

const topKeywords = computed(() =>
  toList(status.value?.topKeywords?.length ? status.value.topKeywords : summary.value?.topKeywords)
)

const topPatterns = computed(() =>
  toList(status.value?.topPatterns?.length ? status.value.topPatterns : summary.value?.topPatterns)
)

const errorSamples = computed(() => toList(summary.value?.errorSamples || status.value?.errorSamples))

const visibleEntries = computed(() => entries.value)
const totalLogHeight = computed(() => visibleEntries.value.length * rowHeight)
const startIndex = computed(() => Math.max(0, Math.floor(scrollTop.value / rowHeight) - overscan))
const visibleCount = computed(() => Math.ceil(viewportHeight.value / rowHeight) + overscan * 2)
const virtualOffset = computed(() => startIndex.value * rowHeight)
const virtualRows = computed(() =>
  visibleEntries.value
    .slice(startIndex.value, startIndex.value + visibleCount.value)
    .map((entry, offset) => ({ entry, index: startIndex.value + offset }))
)

const streamStateText = computed(() => {
  if (streamConnected.value) return '运行中'
  if (streamConnecting.value) return '连接中'
  return '未连接'
})

const streamStateClass = computed(() => ({
  running: streamConnected.value,
  pending: streamConnecting.value,
  stopped: !streamConnected.value && !streamConnecting.value
}))

const canAnalyze = computed(() =>
  entries.value.length > 0 ||
  !!status.value?.sessionId ||
  summary.value?.total !== undefined ||
  summary.value?.matched !== undefined ||
  (isK8sSource.value && !!k8sSource.value?.clusterId && !!k8sSource.value?.namespace && !!k8sSource.value?.pod)
)

const basename = path => String(path || '').split('/').filter(Boolean).pop() || path || '-'

const compactSourceToken = value => String(value || 'unknown').replace(/[^a-zA-Z0-9_-]/g, '-').slice(0, 40) || 'unknown'

const dirname = path => {
  const parts = String(path || '').split('/').filter(Boolean)
  if (parts.length <= 1) return '/'
  return `/${parts.slice(0, -1).join('/')}`
}

const normalizeLevel = level => String(level || 'INFO').trim().toUpperCase()

const levelClass = level => {
  const normalized = normalizeLevel(level)
  if (['ERROR', 'FATAL', 'CRITICAL', 'PANIC'].includes(normalized)) return 'level-danger'
  if (['WARN', 'WARNING'].includes(normalized)) return 'level-warning'
  if (['INFO', 'NOTICE'].includes(normalized)) return 'level-success'
  return 'level-info'
}

const rowLevelClass = level => levelClass(level)

const formatBytes = value => {
  const size = Number(value)
  if (!Number.isFinite(size) || size <= 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let current = size
  let index = 0
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024
    index += 1
  }
  return `${current.toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

const getAuthLabel = auth => {
  const name = auth.name || auth.authName || `凭据 ${auth.authId}`
  const user = auth.username || auth.sshName || auth.user || ''
  return user ? `${name} / ${user}` : name
}

const getProviderLabel = provider => {
  const name = provider.modelName || provider.name || provider.providerName || `Provider ${provider.id}`
  const model = provider.modelCode || provider.model || ''
  return model ? `${name} (${model})` : name
}

const getEntryKey = (entry, index) => `${index}-${entry.lineNo || 0}-${entry.timestampUnix || entry.timestamp || ''}-${entry.raw || entry.message || ''}`

const stringifyJson = value => {
  if (!value || (typeof value === 'object' && Object.keys(value).length === 0)) return '-'
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

const escapeHtml = value =>
  String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')

const escapeRegExp = value => String(value || '').replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

const createHighlightRegex = (pattern, treatAsRegex = false) => {
  const source = String(pattern || '').trim()
  if (!source) return null

  try {
    if (!treatAsRegex) {
      return new RegExp(escapeRegExp(source), 'gi')
    }

    const slashMatch = source.match(/^\/(.+)\/([gimsuy]*)$/)
    if (slashMatch) {
      const flags = slashMatch[2].includes('g') ? slashMatch[2] : `${slashMatch[2]}g`
      return new RegExp(slashMatch[1], flags)
    }

    return new RegExp(source, 'gi')
  } catch {
    return null
  }
}

const collectHighlightRanges = (text, regex) => {
  if (!regex) return []

  const source = String(text || '')
  const ranges = []
  let match = regex.exec(source)

  while (match) {
    const value = match[0] || ''
    if (!value) {
      regex.lastIndex += 1
      match = regex.exec(source)
      continue
    }

    ranges.push([match.index, match.index + value.length])
    match = regex.exec(source)
  }

  return ranges
}

const mergeHighlightRanges = ranges => {
  if (!ranges.length) return []

  const sorted = [...ranges].sort((a, b) => a[0] - b[0])
  const merged = [sorted[0].slice()]

  for (let index = 1; index < sorted.length; index += 1) {
    const [start, end] = sorted[index]
    const last = merged[merged.length - 1]

    if (start <= last[1]) {
      last[1] = Math.max(last[1], end)
    } else {
      merged.push([start, end])
    }
  }

  return merged
}

const highlightLogText = text => {
  const source = String(text ?? '-')
  const keywordRegex = createHighlightRegex(queryForm.keyword)
  const filterRegex = createHighlightRegex(queryForm.regex, true)
  const ranges = mergeHighlightRanges([
    ...collectHighlightRanges(source, keywordRegex),
    ...collectHighlightRanges(source, filterRegex)
  ])

  if (!ranges.length) {
    return escapeHtml(source)
  }

  let cursor = 0
  let html = ''

  ranges.forEach(([start, end]) => {
    if (start > cursor) {
      html += escapeHtml(source.slice(cursor, start))
    }
    html += `<span class="log-highlight">${escapeHtml(source.slice(start, end))}</span>`
    cursor = end
  })

  if (cursor < source.length) {
    html += escapeHtml(source.slice(cursor))
  }

  return html
}

const renderAnalysisInline = text => {
  let html = escapeHtml(text)
  html = html.replace(/`([^`]+)`/g, '<code>$1</code>')
  html = html.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
  html = html.replace(/\b(ERROR|WARN|WARNING|INFO|UNKNOWN|CRITICAL|FATAL)\b/g, (_, token) => {
    return `<span class="ai-token level-${token.toLowerCase()}">${token}</span>`
  })
  html = html.replace(/(\d+(?:\.\d+)?%|\d+条)/g, '<span class="ai-metric">$1</span>')
  html = html.replace(/(低风险|中风险|高风险|需关注|异常|失败|正常|总结|证据不足)/g, '<mark>$1</mark>')
  return html
}

const getAiSectionTone = title => {
  const normalized = String(title || '').replace(/^\d+\.\s*/, '')
  if (/事件摘要|摘要|概览/.test(normalized)) return 'summary'
  if (/根本原因|原因线索|根因/.test(normalized)) return 'cause'
  if (/受影响|服务|组件/.test(normalized)) return 'impact'
  if (/严重性|风险/.test(normalized)) return 'severity'
  if (/下一步检查|排查|检查/.test(normalized)) return 'actions'
  if (/修复步骤|修复/.test(normalized)) return 'repair'
  if (/证据不足/.test(normalized)) return 'evidence'
  if (/总结/.test(normalized)) return 'summary'
  return 'default'
}

const parseAiSections = text => {
  if (!text) return []

  const sections = []
  const lines = String(text).replace(/\r/g, '').split('\n')
  let current = {
    title: '分析结果',
    tone: 'default',
    items: []
  }
  let paragraphBuffer = []

  const flushParagraph = () => {
    if (!paragraphBuffer.length) return
    current.items.push({
      type: 'paragraph',
      html: renderAnalysisInline(paragraphBuffer.join(' '))
    })
    paragraphBuffer = []
  }

  const flushSection = () => {
    flushParagraph()
    if (current.items.length) {
      sections.push(current)
    }
  }

  lines.forEach(rawLine => {
    const line = String(rawLine || '').trim()
    if (!line) {
      flushParagraph()
      return
    }

    const headingMatch = line.match(/^\*\*(.+?)\*\*$/) || line.match(/^#{1,6}\s+(.+)$/)
    if (headingMatch) {
      flushSection()
      current = {
        title: headingMatch[1].trim(),
        tone: getAiSectionTone(headingMatch[1].trim()),
        items: []
      }
      return
    }

    const orderedMatch = line.match(/^(\d+)\.\s+(.+)$/)
    if (orderedMatch) {
      flushParagraph()
      current.items.push({
        type: 'ordered',
        order: orderedMatch[1],
        html: renderAnalysisInline(orderedMatch[2])
      })
      return
    }

    const bulletMatch = line.match(/^[-*]\s+(.+)$/)
    if (bulletMatch) {
      flushParagraph()
      current.items.push({
        type: 'bullet',
        html: renderAnalysisInline(bulletMatch[1])
      })
      return
    }

    paragraphBuffer.push(line)
  })

  flushSection()
  return sections
}

const aiSections = computed(() => parseAiSections(analysisText.value))

const makeSessionId = () => {
  if (isK8sSource.value) {
    const source = k8sSource.value || {}
    return [
      'k8s',
      'cluster',
      compactSourceToken(source.clusterId),
      'ns',
      compactSourceToken(source.namespace),
      'pod',
      compactSourceToken(source.pod),
      'container',
      compactSourceToken(source.container || 'default'),
      Date.now()
    ].join(':')
  }

  const hostId = selectedHostId.value || 'unknown'
  const fileName = basename(selectedPath.value).replace(/[^a-zA-Z0-9_-]/g, '-').slice(0, 32) || 'log'
  return `host-${hostId}-${fileName}-${Date.now()}`
}

const ensureSessionId = () => {
  if (!sessionId.value) {
    sessionId.value = makeSessionId()
  }
  return sessionId.value
}

const normalizeHostAuths = data => {
  const auths = Array.isArray(data?.auths) ? data.auths : toList(data)
  return auths
    .map(auth => ({
      ...auth,
      authId: auth.authId || auth.id || auth.sshKeyId
    }))
    .filter(auth => auth.authId)
}

const resetLogContext = ({ keepSession = false } = {}) => {
  stopStream()
  if (!keepSession) {
    sessionId.value = ''
  }
  entries.value = []
  summary.value = {}
  status.value = null
  selectedEntry.value = null
  analysisResult.value = null
  scrollTop.value = 0
}

const getK8sSourceKey = source => [
  source?.clusterId || '',
  source?.namespace || '',
  source?.pod || '',
  source?.container || '',
  source?.previous ? 'previous' : 'current',
  source?.timestamps === false ? 'no-ts' : 'ts'
].join('|')

const handleK8sSourceChange = source => {
  const previousKey = lastK8sSourceKey.value
  const nextSource = {
    clusterId: source?.clusterId || '',
    namespace: source?.namespace || '',
    pod: source?.pod || '',
    container: source?.container || '',
    previous: !!source?.previous,
    timestamps: source?.timestamps !== false,
    clusterName: source?.clusterName || k8sSource.value?.clusterName || ''
  }
  const nextKey = getK8sSourceKey(nextSource)
  k8sSource.value = {
    ...nextSource
  }
  lastK8sSourceKey.value = nextKey
  if (activeSourceType.value === 'k8s' && previousKey && previousKey !== nextKey) {
    const hasSource = !!(k8sSource.value.clusterId || k8sSource.value.namespace || k8sSource.value.pod)
    resetLogContext({ keepSession: !hasSource })
  }
}

const hostTreeData = computed(() =>
  hostOptions.value.map(host => {
    const hostId = getHostId(host)
    const hostFiles = String(hostId) === String(selectedHostId.value) ? fileList.value : []
    const dirMap = new Map()

    hostFiles.forEach(file => {
      const dirPath = dirname(file.path)
      if (!dirMap.has(dirPath)) {
        dirMap.set(dirPath, {
          id: `dir-${hostId}-${dirPath}`,
          type: 'dir',
          label: dirPath,
          path: dirPath,
          children: []
        })
      }
      dirMap.get(dirPath).children.push({
        id: `file-${hostId}-${file.path}`,
        type: 'file',
        label: file.name || basename(file.path),
        meta: formatBytes(file.size),
        file,
        path: file.path
      })
    })

    return {
      id: `host-${hostId}`,
      type: 'host',
      label: getHostName(host),
      meta: getHostIp(host),
      host,
      children: Array.from(dirMap.values())
    }
  })
)

const loadHosts = async (keyword = '') => {
  hostsLoading.value = true
  try {
    const params = { page: 1, pageSize: 50 }
    const response = keyword
      ? await cmdbApi.GetCmdbHostsByKeyword(keyword, params)
      : await cmdbApi.getCmdbHostList(params)
    hostOptions.value = filterLoggableHosts(getPayload(response, '获取主机列表失败'))
  } catch (error) {
    console.error('load hosts failed:', error)
    ElMessage.error(error.message || '获取主机列表失败')
  } finally {
    hostsLoading.value = false
  }
}

const loadHostAuths = async hostId => {
  authOptions.value = []
  selectedAuthId.value = ''
  if (!hostId) return

  try {
    const response = await cmdbApi.getHostSshAuths(hostId)
    const data = getPayload(response, '获取 SSH 凭据失败')
    authOptions.value = normalizeHostAuths(data)
    const hostDefault = currentHost.value?.sshKeyId || currentHost.value?.authId || ''
    selectedAuthId.value = authOptions.value[0]?.authId || hostDefault || ''
  } catch (error) {
    console.warn('load host auths failed:', error)
  }
}

const handleResourceNodeClick = async node => {
  if (node.type === 'host') {
    await selectHost(node.host)
    return
  }

  if (node.type === 'file') {
    if (!selectedHostId.value) {
      const hostId = String(node.id).split('-')[1]
      const host = hostOptions.value.find(item => String(getHostId(item)) === hostId)
      if (host) await selectHost(host)
    }
    selectLogFile(node.file)
  }
}

const resetQueryFilters = () => {
  queryForm.keyword = ''
  queryForm.regex = ''
  queryForm.levels = []
  queryForm.formatName = 'auto'
  queryForm.formatYaml = ''
}

const resetFileScanFilters = () => {
  fileQuery.dir = '/var/log'
  fileQuery.keyword = ''
}

const handleResourceNodeDoubleClick = async node => {
  if (node.type === 'host' && node.host) {
    await selectHost(node.host)
    resetFileScanFilters()
    await nextTick()
    await loadLogFiles()
    return
  }

  if (node.type !== 'file' || !node.file?.path) return

  if (!selectedHostId.value) {
    const hostId = String(node.id).split('-')[1]
    const host = hostOptions.value.find(item => String(getHostId(item)) === hostId)
    if (host) await selectHost(host)
  }

  if (selectedPath.value !== node.file.path) {
    selectLogFile(node.file)
  } else {
    focusTreeFile(node.file)
  }

  resetQueryFilters()
  await nextTick()
  runQuery()
}

const selectHost = async host => {
  const hostId = getHostId(host)
  if (String(selectedHostId.value) === String(hostId)) return
  activeSourceType.value = 'host'
  stopStream()
  selectedHostId.value = hostId
  selectedPath.value = ''
  sessionId.value = ''
  fileList.value = []
  entries.value = []
  summary.value = {}
  status.value = null
  selectedEntry.value = null
  analysisResult.value = null
  await loadHostAuths(hostId)
}

const focusTreeFile = file => {
  if (!file?.path) return
  nextTick(() => {
    hostTreeRef.value?.setCurrentKey(`file-${selectedHostId.value}-${file.path}`)
  })
}

const loadLogFiles = async () => {
  if (!selectedHostId.value) {
    ElMessage.warning('请先选择主机')
    return
  }

  filesLoading.value = true
  try {
    const response = await aiopsApi.listHostLogFiles({
      hostId: selectedHostId.value,
      authId: selectedAuthId.value || undefined,
      dir: fileQuery.dir || '/var/log',
      maxDepth: fileQuery.maxDepth,
      limit: fileQuery.limit,
      keyword: fileQuery.keyword || undefined
    })
    const payload = getPayload(response, '扫描日志文件失败')
    fileList.value = Array.isArray(payload?.files) ? payload.files : toList(payload)
    const keyword = String(fileQuery.keyword || '').trim()
    const preservedFile = fileList.value.find(file => file.path === selectedPath.value)
    const targetFile = preservedFile || (keyword ? fileList.value[0] : null)

    if (targetFile) {
      if (selectedPath.value === targetFile.path) {
        focusTreeFile(targetFile)
      } else {
        selectLogFile(targetFile)
      }
    } else {
      nextTick(() => {
        hostTreeRef.value?.setCurrentKey(`host-${selectedHostId.value}`)
      })
    }
  } catch (error) {
    console.error('load log files failed:', error)
    ElMessage.error(error.message || '扫描日志文件失败')
  } finally {
    filesLoading.value = false
  }
}

const selectLogFile = file => {
  if (!file?.path || selectedPath.value === file.path) return
  activeSourceType.value = 'host'
  stopStream()
  selectedPath.value = file.path
  sessionId.value = makeSessionId()
  entries.value = []
  summary.value = {}
  status.value = null
  analysisResult.value = null
  selectedEntry.value = null
  nextTick(() => hostTreeRef.value?.setCurrentKey(`file-${selectedHostId.value}-${file.path}`))
}

watch(resourceTab, tab => {
  if (tab === 'host' || tab === 'k8s') {
    const previousSourceType = activeSourceType.value
    activeSourceType.value = tab
    if (previousSourceType !== tab) {
      resetLogContext()
    }
  }
})

const buildHostLogRequest = () => ({
  sourceType: 'host',
  sessionId: ensureSessionId(),
  hostId: selectedHostId.value,
  authId: selectedAuthId.value || undefined,
  path: selectedPath.value,
  tailLines: queryForm.tailLines,
  limit: queryForm.limit,
  keyword: queryForm.keyword || undefined,
  regex: queryForm.regex || undefined,
  levels: queryForm.levels?.length ? queryForm.levels : undefined,
  formatName: queryForm.formatName || 'auto',
  formatYaml: queryForm.formatYaml || undefined
})

const buildK8sLogRequest = () => {
  const source = k8sSource.value || {}
  return {
    sourceType: 'k8s',
    sessionId: ensureSessionId(),
    clusterId: source.clusterId,
    namespace: source.namespace,
    pod: source.pod,
    container: source.container || undefined,
    tailLines: queryForm.tailLines,
    limit: queryForm.limit,
    keyword: queryForm.keyword || undefined,
    regex: queryForm.regex || undefined,
    levels: queryForm.levels?.length ? queryForm.levels : undefined,
    formatName: queryForm.formatName || 'auto',
    formatYaml: queryForm.formatYaml || undefined,
    previous: !!source.previous,
    timestamps: source.timestamps !== false
  }
}

const buildLogRequest = () => (isK8sSource.value ? buildK8sLogRequest() : buildHostLogRequest())

const validateSource = () => {
  if (isK8sSource.value) {
    const source = k8sSource.value || {}
    if (!source.clusterId) {
      ElMessage.warning('请先选择 K8s 集群')
      return false
    }
    if (!source.namespace) {
      ElMessage.warning('请先选择命名空间')
      return false
    }
    if (!source.pod) {
      ElMessage.warning('请先选择 Pod')
      return false
    }
    return true
  }

  if (!selectedHostId.value) {
    ElMessage.warning('请先选择主机')
    return false
  }
  if (!selectedPath.value) {
    ElMessage.warning('请先选择日志文件')
    return false
  }
  return true
}

const hasValidSource = () => {
  if (isK8sSource.value) {
    const source = k8sSource.value || {}
    return !!source.clusterId && !!source.namespace && !!source.pod
  }
  return !!selectedHostId.value && !!selectedPath.value
}

const scrollLogsToBottom = () => {
  if (!autoScroll.value) return
  nextTick(() => {
    const el = virtualViewport.value
    if (el) {
      el.scrollTop = el.scrollHeight
      scrollTop.value = el.scrollTop
    }
  })
}

const handleLogScroll = event => {
  scrollTop.value = event.target.scrollTop
  viewportHeight.value = event.target.clientHeight || viewportHeight.value
}

const setQueryResult = payload => {
  sessionId.value = payload?.sessionId || sessionId.value
  entries.value = toList(payload?.entries)
  summary.value = payload?.summary || {}
  status.value = payload?.status || status.value
  selectedEntry.value = entries.value[entries.value.length - 1] || null
  scrollLogsToBottom()
}

const runQuery = async () => {
  if (!validateSource()) return

  queryLoading.value = true
  try {
    const requestPayload = buildLogRequest()
    const response = isK8sSource.value
      ? await aiopsApi.queryK8sLogs(requestPayload)
      : await aiopsApi.queryHostLogs(requestPayload)
    setQueryResult(getPayload(response, '查询日志失败'))
    startStatusPolling()
    loadSessions()
  } catch (error) {
    console.error('query logs failed:', error)
    ElMessage.error(error.message || '查询日志失败')
  } finally {
    queryLoading.value = false
  }
}

const pushLogEntry = entry => {
  if (!entry) return
  entries.value.push(entry)
  if (entries.value.length > maxBufferedEntries) {
    entries.value.splice(0, entries.value.length - maxBufferedEntries)
  }
  selectedEntry.value = entry
  scrollLogsToBottom()
}

const handleStreamMessage = message => {
  if (message?.type === 'entry') {
    pushLogEntry(message.data)
    return
  }

  if (message?.type === 'open') {
    refreshStatus()
    return
  }

  if (message?.type === 'limit') {
    ElMessage.warning(message.message || '实时流已达到返回上限')
    return
  }

  if (message?.type === 'error') {
    ElMessage.error(message.message || '实时流发生错误')
    return
  }

  if (message?.type === 'close') {
    ElMessage.info(message.message || '实时流已关闭')
  }
}

const startStream = () => {
  if (!validateSource()) return

  stopStream()
  streamConnecting.value = true
  const requestParams = {
    ...buildLogRequest(),
    follow: true
  }

  try {
    streamSocket = isK8sSource.value
      ? aiopsApi.createK8sLogStream(requestParams)
      : aiopsApi.createHostLogStream(requestParams)

    streamSocket.onopen = () => {
      streamConnecting.value = false
      streamConnected.value = true
      startStatusPolling()
    }

    streamSocket.onmessage = async event => {
      let raw = event.data
      if (raw instanceof Blob) {
        raw = await raw.text()
      }
      try {
        handleStreamMessage(JSON.parse(raw))
      } catch {
        pushLogEntry({ raw, message: raw, level: 'INFO', timestamp: new Date().toISOString() })
      }
    }

    streamSocket.onerror = () => {
      streamConnecting.value = false
      streamConnected.value = false
      ElMessage.error('实时流连接失败')
    }

    streamSocket.onclose = () => {
      streamConnecting.value = false
      streamConnected.value = false
      streamSocket = null
      refreshStatus()
    }
  } catch (error) {
    streamConnecting.value = false
    streamConnected.value = false
    ElMessage.error(error.message || '创建实时流失败')
  }
}

const stopStream = () => {
  if (!streamSocket) {
    streamConnecting.value = false
    streamConnected.value = false
    return
  }

  try {
    if (streamSocket.readyState === WebSocket.OPEN) {
      streamSocket.send(JSON.stringify({ type: 'close' }))
    }
    streamSocket.close()
  } catch (error) {
    console.warn('close stream failed:', error)
  } finally {
    streamSocket = null
    streamConnecting.value = false
    streamConnected.value = false
  }
}

const refreshStatus = async () => {
  if (!sessionId.value) return
  try {
    const response = await aiopsApi.getLogSessionStatus(sessionId.value)
    status.value = getPayload(response, '获取会话状态失败')
  } catch (error) {
    console.warn('refresh status failed:', error)
  }
}

const clearStatusPolling = () => {
  if (statusTimer) {
    clearInterval(statusTimer)
    statusTimer = null
  }
}

const startStatusPolling = () => {
  clearStatusPolling()
  if (!sessionId.value) return
  refreshStatus()
  statusTimer = setInterval(refreshStatus, 5000)
}

const clearLogs = () => {
  entries.value = []
  summary.value = {}
  selectedEntry.value = null
  analysisResult.value = null
  scrollTop.value = 0
}

const closeCurrentSession = () => {
  stopStream()
  clearStatusPolling()
  sessionId.value = ''
  entries.value = []
  summary.value = {}
  status.value = null
  analysisResult.value = null
  selectedEntry.value = null
}

const loadSessions = async () => {
  sessionsLoading.value = true
  try {
    const response = await aiopsApi.listLogSessions({ activeOnly: sessionsActiveOnly.value })
    sessions.value = toList(getPayload(response, '获取日志会话失败'))
  } catch (error) {
    console.warn('load sessions failed:', error)
  } finally {
    sessionsLoading.value = false
  }
}

const getSessionSourceType = session => {
  const source = session?.source || {}
  return source.type || source.sourceType || (source.clusterId ? 'k8s' : 'host')
}

const getSessionSourceTitle = session => {
  const source = session?.source || {}
  if (getSessionSourceType(session) === 'k8s') {
    return source.clusterName || source.cluster || (source.clusterId ? `K8s 集群 ${source.clusterId}` : 'K8s')
  }
  return source.hostName || source.hostId || '-'
}

const getSessionSourceTarget = session => {
  const source = session?.source || {}
  if (getSessionSourceType(session) === 'k8s') {
    return [source.namespace, source.pod, source.container || '默认业务容器'].filter(Boolean).join(' / ') || '-'
  }
  return source.path || '-'
}

const getEntrySourceLabel = entry => {
  const source = entry?.source || status.value?.source || {}
  const sourceType = source.type || source.sourceType || (source.clusterId ? 'k8s' : 'host')
  if (sourceType === 'k8s') {
    return [source.namespace, source.pod, source.container || k8sSource.value?.container || '默认业务容器'].filter(Boolean).join(' / ') || currentTargetLabel.value
  }
  return source.path || selectedPath.value || '-'
}

const useSession = async session => {
  stopStream()
  sessionId.value = session.sessionId
  status.value = session
  const source = session.source || {}
  const sourceType = getSessionSourceType(session)

  activeSourceType.value = sourceType
  resourceTab.value = sourceType === 'k8s' ? 'k8s' : 'host'
  entries.value = []
  summary.value = {}
  analysisResult.value = null
  selectedEntry.value = null

  if (sourceType === 'k8s') {
    k8sSource.value = {
      clusterId: source.clusterId || '',
      namespace: source.namespace || '',
      pod: source.pod || '',
      container: source.container || '',
      previous: !!source.previous,
      timestamps: source.timestamps !== false,
      clusterName: source.clusterName || source.cluster || ''
    }
    lastK8sSourceKey.value = getK8sSourceKey(k8sSource.value)
    await refreshStatus()
    startStatusPolling()
    return
  }

  if (source.hostId) {
    selectedHostId.value = source.hostId
    if (!hostOptions.value.some(host => String(getHostId(host)) === String(source.hostId))) {
      const sessionHost = {
        id: source.hostId,
        hostName: source.hostName || `Host ${source.hostId}`,
        sshIp: source.hostIp || '',
        status: 1,
        hostType: source.hostType || source.host_type || 'linux'
      }
      if (isAuthSuccessHost(sessionHost) && isLinuxHost(sessionHost)) {
        hostOptions.value.unshift(sessionHost)
      }
    }
    await loadHostAuths(source.hostId)
  }

  if (source.path) {
    selectedPath.value = source.path
  }

  await refreshStatus()
  startStatusPolling()
}

const loadProviders = async () => {
  try {
    const response = await aiopsApi.getEnabledLLMProviders({ page: 1, pageSize: 100 })
    providerOptions.value = toList(getPayload(response, '获取 Provider 失败'))
  } catch (error) {
    console.warn('load providers failed:', error)
  }
}

const runAnalyze = async (override = {}) => {
  if (!canAnalyze.value && !override.entries?.length && !override.rawLines?.length) {
    ElMessage.warning('请先查询日志或选择会话')
    return
  }

  analysisLoading.value = true
  analysisResult.value = null
  const previousTab = analysisTab.value
  analysisTab.value = 'ai'
  try {
    const contextSize = analysisForm.contextSize || 200
    const hasOverrideLogContext = !!override.entries?.length || !!override.rawLines?.length || !!override.k8sQuery
    const useQueryContext = !hasOverrideLogContext && isK8sSource.value && hasValidSource()
    const payload = {
      sessionId: override.sessionId !== undefined ? override.sessionId : sessionId.value || undefined,
      providerId: analysisForm.providerId || undefined,
      prompt: override.prompt || analysisForm.prompt || undefined,
      k8sQuery: override.k8sQuery || (useQueryContext ? buildK8sLogRequest() : undefined),
      entries: override.entries || (useQueryContext || sessionId.value ? undefined : entries.value.slice(-contextSize)),
      rawLines: override.rawLines,
      summary: summary.value && Object.keys(summary.value).length ? summary.value : undefined
    }
    const response = await aiopsApi.analyzeLogs(payload)
    analysisResult.value = getPayload(response, 'AI 分析失败')
  } catch (error) {
    if (error.message?.includes('请先选择')) {
      analysisTab.value = previousTab
    }
    console.error('analyze logs failed:', error)
    ElMessage.error(error.message || 'AI 分析失败')
  } finally {
    analysisLoading.value = false
  }
}

const quickAnalyze = () => runAnalyze()

const analyzeSingleEntry = () => {
  if (!selectedEntry.value) return
  runAnalyze({
    sessionId: undefined,
    entries: [selectedEntry.value],
    prompt: '请分析这条日志的含义、风险等级、可能原因和下一步排查建议。'
  })
}

const applyKeywordFilter = keyword => {
  queryForm.keyword = String(keyword || '').slice(0, 120)
  runQuery()
}

const applyPatternFilter = pattern => {
  queryForm.keyword = pattern.sample || pattern.pattern || ''
  runQuery()
}

const openEntryDetail = entry => {
  selectedEntry.value = entry
  entryDrawerVisible.value = true
}

const copyText = async text => {
  if (!text) {
    ElMessage.warning('暂无可复制内容')
    return
  }
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制')
  } catch {
    ElMessage.error('复制失败，请手动复制')
  }
}

const copySessionId = () => copyText(sessionId.value)

const copyVisibleLogs = () => {
  const text = visibleEntries.value
    .map(entry => entry.raw || entry.message || '')
    .filter(Boolean)
    .join('\n')
  copyText(text)
}

onMounted(() => {
  loadHosts()
  loadSessions()
  loadProviders()
  nextTick(() => {
    if (virtualViewport.value) {
      viewportHeight.value = virtualViewport.value.clientHeight || viewportHeight.value
    }
  })
})

onBeforeUnmount(() => {
  stopStream()
  clearStatusPolling()
})
</script>

<style scoped>
:global(.home-container .el-main:has(.log-analysis-page)) {
  padding-top: var(--ops-shell-gutter, 10px);
  padding-right: var(--ops-shell-gutter, 10px);
  padding-left: var(--ops-shell-gutter, 10px);
}

.log-analysis-page {
  --log-panel: var(--ops-bg-surface, #ffffff);
  --log-panel-soft: var(--ops-bg-surface-soft, #fbfdff);
  --log-subtle-bg: var(--ops-bg-subtle, #f3f7fc);
  --log-border: var(--ops-border, #dbe7f4);
  --log-border-soft: var(--ops-border-soft, #e7eef7);
  --log-text: var(--ops-text-primary, #172033);
  --log-heading: var(--ops-text-heading, #0f172a);
  --log-muted: var(--ops-text-secondary, #53657d);
  --log-subtle: var(--ops-text-muted, #64748b);
  --log-brand: var(--ops-color-primary, #2563eb);
  --log-success: var(--ops-color-success, #10b981);
  --log-warning: var(--ops-color-warning, #f59e0b);
  --log-danger: var(--ops-color-danger, #ef4444);
  box-sizing: border-box;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: var(--ops-module-gap, 8px);
  height: calc(100vh - 124px);
  min-height: 0;
  padding: var(--ops-page-padding, 7px);
  overflow: hidden;
  color: var(--log-text);
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
  border: 1px solid var(--log-border);
  border-radius: var(--ops-card-radius, 12px);
  background: rgba(255, 255, 255, 0.96);
  box-shadow: var(--ops-shadow-hero, 0 10px 28px rgba(80, 124, 180, 0.10));
}

.ops-page-header__identity,
.ops-page-header__content,
.ops-page-header__status,
.top-actions {
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
  color: var(--log-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
}

.ops-page-header__divider {
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  background: var(--log-border-soft);
}

.ops-page-header p {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  color: var(--log-subtle);
  font-size: 12px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ops-page-header__status {
  flex: 0 0 auto;
  gap: 8px;
}

.session-chip {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  gap: 6px;
  max-width: 180px;
  height: 30px;
  padding: 0 9px;
  border: 1px solid var(--log-border-soft);
  border-radius: 8px;
  background: var(--log-subtle-bg);
}

.session-chip span {
  color: var(--log-subtle);
  font-size: 11px;
}

.session-chip strong {
  overflow: hidden;
  color: var(--log-heading);
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stream-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 24px;
  padding: 0 8px;
  border-radius: 999px;
  background: #f3f7fc;
  color: var(--log-subtle);
  font-size: 11px;
  font-weight: 700;
  white-space: nowrap;
}

.stream-pill i {
  width: 7px;
  height: 7px;
  flex: 0 0 7px;
  border-radius: 50%;
  background: currentColor;
}

.stream-pill.running {
  background: #eaf8ef;
  color: #059669;
}

.stream-pill.pending {
  background: #fff4e5;
  color: #d97706;
}

.stream-pill.stopped {
  background: #f3f7fc;
  color: #64748b;
}

.top-actions {
  gap: 6px;
}

.header-icon-button,
.query-icon-button {
  width: 30px;
  min-width: 30px;
  height: 30px;
  padding: 0;
}

.top-actions :deep(.el-button + .el-button),
.query-actions :deep(.el-button + .el-button) {
  margin-left: 0;
}

.workspace {
  display: grid;
  grid-template-columns: 280px minmax(520px, 1fr) 328px;
  gap: var(--ops-module-gap, 8px);
  min-width: 0;
  min-height: 0;
}

.resource-panel,
.log-workspace,
.analysis-panel {
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--log-border);
  border-radius: var(--ops-card-radius, 12px);
  background: rgba(255, 255, 255, 0.98);
  box-shadow: var(--ops-shadow-card, 0 8px 24px rgba(80, 124, 180, 0.07));
}

.resource-panel {
  display: flex;
  flex-direction: column;
}

.resource-tabs,
.analysis-tabs {
  height: 100%;
}

.resource-tabs :deep(.el-tabs__header),
.analysis-tabs :deep(.el-tabs__header) {
  height: 40px;
  margin: 0;
  padding: 0 10px;
  background: #fff;
}

.resource-tabs :deep(.el-tabs__item),
.analysis-tabs :deep(.el-tabs__item) {
  height: 40px;
  color: var(--log-muted);
  font-size: 12px;
  font-weight: 700;
}

.resource-tabs :deep(.el-tabs__item.is-active),
.analysis-tabs :deep(.el-tabs__item.is-active) {
  color: var(--log-brand);
}

.resource-tabs :deep(.el-tabs__nav-wrap::after),
.analysis-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: var(--log-border-soft);
}

.resource-tabs :deep(.el-tabs__content) {
  height: calc(100% - 40px);
}

.resource-tabs :deep(.el-tab-pane) {
  box-sizing: border-box;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr) auto;
  gap: 8px;
  height: 100%;
  padding: 8px;
}

.resource-tools {
  display: flex;
  gap: 6px;
}

.resource-tools .el-input {
  flex: 1;
  min-width: 0;
}

.resource-tools :deep(.el-button),
.resource-panel :deep(.k8s-tools .el-button) {
  width: 34px;
  min-width: 34px;
  height: 34px;
  padding: 0;
}

.resource-scroll {
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--log-border-soft);
  border-radius: 8px;
  background: var(--log-panel-soft);
}

.resource-tree {
  padding: 4px;
  background: transparent;
}

.resource-tree :deep(.el-tree-node__content) {
  min-height: 30px;
  margin: 1px 0;
  border-radius: 7px;
  transition:
    background-color 160ms ease,
    box-shadow 160ms ease;
}

.resource-tree :deep(.el-tree-node__content:hover) {
  background: #eef6ff;
}

.resource-tree :deep(.el-tree-node.is-current > .el-tree-node__content) {
  background: #e7f1ff;
  box-shadow: inset 0 0 0 1px #b9d4ff;
}

.resource-tree :deep(.el-tree-node.is-current > .el-tree-node__content .tree-node span),
.resource-tree :deep(.el-tree-node.is-current > .el-tree-node__content .tree-node small) {
  color: #17457f;
}

.tree-node {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  min-width: 0;
}

.tree-node span,
.tree-node small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tree-node span {
  color: var(--log-text);
  font-size: 12px;
}

.tree-node small {
  margin-left: auto;
  color: var(--log-subtle);
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 10.5px;
}

.tree-node.type-file span {
  font-family: Consolas, "SFMono-Regular", monospace;
}

.tree-node.type-file .el-icon {
  color: var(--log-brand);
}

.tree-node.type-dir .el-icon {
  color: var(--log-warning);
}

.tree-node.type-host .el-icon {
  color: var(--log-success);
}

.scan-box {
  padding: 8px;
  border: 1px solid var(--log-border-soft);
  border-radius: 8px;
  background: var(--log-panel-soft);
}

.scan-box :deep(.el-form-item),
.resource-panel :deep(.k8s-cascade .el-form-item) {
  margin-bottom: 8px;
}

.scan-box :deep(.el-form-item__label),
.resource-panel :deep(.k8s-cascade .el-form-item__label) {
  height: auto;
  margin-bottom: 5px;
  padding: 0;
  color: #334155;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.4;
}

.scan-box .el-button {
  width: 100%;
}

.scan-options {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px;
  margin-bottom: 8px;
}

.resource-panel :deep(.k8s-log-source-panel) {
  gap: 8px;
}

.resource-panel :deep(.k8s-cascade) {
  padding: 8px;
  background: var(--log-panel-soft);
}

.resource-panel :deep(.k8s-options) {
  margin-bottom: 8px;
}

.resource-panel :deep(.source-summary) {
  margin: 6px;
  padding: 12px 8px;
}

.resource-panel :deep(.selection-list) {
  gap: 6px;
  padding: 0 6px 6px;
}

.resource-panel :deep(.selection-list button) {
  padding: 7px 8px;
}

.sessions-scroll {
  min-height: 0;
}

.session-node {
  width: calc(100% - 12px);
  margin: 6px;
  padding: 8px 9px;
  border: 1px solid var(--log-border-soft);
  border-radius: 8px;
  background: #fff;
  color: inherit;
  cursor: pointer;
  text-align: left;
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    transform 160ms ease;
}

.session-node:hover,
.session-node.active {
  border-color: var(--ops-border-hover, #b9d4ff);
  background: #eef6ff;
}

.session-node:active {
  transform: translateY(1px);
}

.session-node:focus-visible {
  outline: 2px solid var(--log-brand);
  outline-offset: 1px;
}

.session-node strong,
.session-node span,
.session-node small,
.session-node em {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-node strong {
  color: var(--log-heading);
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 11px;
}

.session-node span,
.session-node small,
.session-node em {
  margin-top: 3px;
  color: var(--log-subtle);
  font-size: 10.5px;
  font-style: normal;
}

.log-workspace {
  display: grid;
  grid-template-rows: auto auto minmax(0, 1fr);
}

.query-toolbar {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 8px;
  padding: 7px 8px;
  border-bottom: 1px solid var(--log-border-soft);
  background: var(--log-subtle-bg);
}

.query-filters {
  display: grid;
  grid-template-columns: minmax(170px, 1fr) 116px 108px;
  gap: 6px;
  min-width: 0;
}

.query-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.toolbar-input,
.toolbar-level,
.toolbar-number {
  width: 100%;
  min-width: 0;
}

.query-toolbar :deep(.el-select__wrapper),
.query-toolbar :deep(.el-input__wrapper) {
  min-height: 34px;
}

.query-toolbar :deep(.el-button) {
  height: 30px;
  padding: 0 10px;
  font-size: 12px;
}

.query-toolbar :deep(.el-button .el-icon) {
  font-size: 14px;
}

.query-toolbar .query-icon-button {
  width: 30px;
  padding: 0;
}

.ai-analysis-button {
  color: #fff;
  border-color: #5664d9;
  background: linear-gradient(135deg, #6678e8, #3f57bf);
}

.ai-analysis-button:hover,
.ai-analysis-button:focus {
  color: #fff;
  border-color: #4656c6;
  background: linear-gradient(135deg, #5b6dda, #344aa9);
}

.ai-analysis-button.is-disabled,
.ai-analysis-button.is-disabled:hover {
  color: rgba(255, 255, 255, 0.9);
  border-color: #aeb7e4;
  background: #aeb7e4;
}

.advanced-collapse {
  border-top: 0;
  border-bottom: 1px solid var(--log-border-soft);
}

.advanced-collapse :deep(.el-collapse-item__header) {
  height: 32px;
  padding: 0 10px;
  border-bottom: 0;
  background: #fbfdff;
}

.advanced-collapse :deep(.el-collapse-item__wrap) {
  border-bottom: 0;
}

.advanced-collapse :deep(.el-collapse-item__content) {
  padding: 0 10px 10px;
  background: #fbfdff;
}

.advanced-title {
  color: var(--log-muted);
  font-size: 12px;
  font-weight: 700;
}

.advanced-summary {
  min-width: 0;
  margin-left: 8px;
  overflow: hidden;
  color: var(--log-subtle);
  font-size: 11px;
  font-weight: 400;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.advanced-grid {
  display: grid;
  grid-template-columns: 140px minmax(180px, 1fr) 120px;
  gap: 8px;
  padding-top: 4px;
}

.advanced-field {
  display: grid;
  gap: 5px;
  min-width: 0;
}

.advanced-field > span {
  color: #334155;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.4;
}

.advanced-field--parser {
  grid-column: 1 / -1;
}

.advanced-field--parser :deep(.el-textarea__inner) {
  min-height: 92px;
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 12px;
  line-height: 1.65;
}

.log-main {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  min-height: 0;
}

.log-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 7px 10px;
  border-bottom: 1px solid #2b374a;
  background: #172033;
  color: #f1f5f9;
}

.log-head strong,
.log-head span {
  display: block;
}

.log-head strong {
  font-size: 13px;
}

.log-head span {
  margin-top: 2px;
  color: #a8b3c3;
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 10.5px;
  font-variant-numeric: tabular-nums;
}

.log-head-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.log-head-actions :deep(.el-switch__label) {
  color: #cbd5e1;
  font-size: 11px;
}

.copy-icon-button {
  width: 28px;
  min-width: 28px;
  height: 28px;
  color: #f8c555;
  border-color: rgba(248, 197, 85, 0.38);
  background: rgba(248, 197, 85, 0.10);
}

.copy-icon-button:hover,
.copy-icon-button:focus {
  color: #ffe08a;
  border-color: rgba(255, 224, 138, 0.58);
  background: rgba(248, 197, 85, 0.18);
}

.virtual-log {
  position: relative;
  min-height: 0;
  overflow: auto;
  background: #111a29;
  scrollbar-color: #46566d #111a29;
}

.virtual-log :deep(.el-empty__description p) {
  color: #94a3b8;
}

.virtual-spacer {
  position: relative;
  min-height: 100%;
}

.virtual-window {
  position: absolute;
  inset: 0 0 auto;
}

.log-line {
  display: grid;
  grid-template-columns: 62px 70px 168px minmax(0, 1fr);
  align-items: center;
  width: 100%;
  height: 36px;
  padding: 0 9px;
  border: 0;
  border-left: 3px solid #64748b;
  border-bottom: 1px solid rgba(71, 85, 105, 0.46);
  background: transparent;
  color: #dbeafe;
  cursor: pointer;
  text-align: left;
  transition: background-color 160ms ease;
}

.log-line:hover {
  background: rgba(51, 65, 85, 0.72);
}

.log-line:active {
  background: rgba(71, 85, 105, 0.82);
}

.log-line:focus-visible {
  position: relative;
  outline: 2px solid #60a5fa;
  outline-offset: -2px;
}

.log-line.level-danger {
  border-left-color: #f87171;
}

.log-line.level-warning {
  border-left-color: #fbbf24;
}

.log-line.level-success {
  border-left-color: #34d399;
}

.log-line.level-info {
  border-left-color: #93c5fd;
}

.line-no,
.line-level,
.line-time,
.log-line code {
  overflow: hidden;
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 11.5px;
  font-variant-numeric: tabular-nums;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.line-no,
.line-time {
  color: #9ca3af;
}

.line-level {
  font-weight: 700;
}

.level-danger .line-level {
  color: #fca5a5;
}

.level-warning .line-level {
  color: #fcd34d;
}

.level-success .line-level {
  color: #86efac;
}

.level-info .line-level {
  color: #bfdbfe;
}

.log-line code {
  color: #e5e7eb;
}

.log-line :deep(.log-highlight) {
  padding: 0 2px;
  border-radius: 4px;
  background: rgba(239, 68, 68, 0.22);
  color: #fda4af;
  font-weight: 700;
}

.analysis-panel {
  padding: 0 10px 10px;
}

.analysis-tabs :deep(.el-tabs__content) {
  height: calc(100% - 40px);
  overflow: auto;
}

.analysis-tabs :deep(.el-tab-pane) {
  min-width: 0;
  padding-top: 8px;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px;
}

.metric-grid div {
  position: relative;
  min-width: 0;
  min-height: 62px;
  padding: 8px 9px 7px;
  overflow: hidden;
  border: 1px solid var(--log-border-soft);
  border-radius: 8px;
  background: var(--log-panel-soft);
}

.metric-grid div::before {
  position: absolute;
  inset: 0 auto 0 0;
  width: 3px;
  background: var(--log-brand);
  content: "";
}

.metric-grid div:nth-child(2)::before {
  background: var(--log-success);
}

.metric-grid div:nth-child(3)::before {
  background: var(--log-warning);
}

.metric-grid div:nth-child(4)::before {
  background: #0ea5e9;
}

.metric-grid span,
.metric-grid strong {
  display: block;
}

.metric-grid span {
  color: var(--log-subtle);
  font-size: 10.5px;
  font-weight: 600;
}

.metric-grid strong {
  margin-top: 5px;
  overflow: hidden;
  color: var(--log-heading);
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 18px;
  font-variant-numeric: tabular-nums;
  line-height: 1.05;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chart-section {
  margin-top: 10px;
  padding-top: 9px;
  border-top: 1px solid var(--log-border-soft);
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.section-head strong {
  color: var(--log-heading);
  font-size: 13px;
}

.level-bars {
  display: grid;
  gap: 7px;
}

.level-bar {
  display: grid;
  grid-template-columns: 60px minmax(0, 1fr) 40px;
  align-items: center;
  gap: 7px;
  color: var(--log-muted);
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.level-bar > div {
  height: 6px;
  overflow: hidden;
  border-radius: 999px;
  background: #e8edf5;
}

.level-bar i {
  display: block;
  height: 100%;
  border-radius: inherit;
}

.level-bar i.level-danger {
  background: var(--log-danger);
}

.level-bar i.level-warning {
  background: var(--log-warning);
}

.level-bar i.level-success {
  background: var(--log-success);
}

.level-bar i.level-info {
  background: var(--log-brand);
}

.stream-list,
.keyword-list {
  display: grid;
  gap: 6px;
}

.stream-item {
  display: grid;
  grid-template-columns: 10px minmax(0, 1fr);
  gap: 7px;
}

.stream-item i {
  width: 7px;
  height: 7px;
  margin-top: 5px;
  border-radius: 50%;
  background: var(--log-subtle);
}

.stream-item i.active {
  background: var(--log-success);
}

.stream-item strong,
.stream-item span {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stream-item strong {
  color: var(--log-text);
  font-size: 11.5px;
}

.stream-item span,
.empty-note {
  color: var(--log-subtle);
  font-size: 11px;
}

.pattern-card,
.sample-card,
.keyword-list button {
  width: 100%;
  border: 1px solid var(--log-border-soft);
  border-radius: 8px;
  background: var(--log-panel-soft);
  color: inherit;
  cursor: pointer;
  text-align: left;
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    transform 160ms ease;
}

.pattern-card {
  display: grid;
  gap: 3px;
  margin-bottom: 6px;
  padding: 8px 9px;
}

.pattern-card:hover,
.sample-card:hover,
.keyword-list button:hover {
  border-color: var(--ops-border-hover, #b9d4ff);
  background: #eef6ff;
}

.pattern-card:active,
.sample-card:active,
.keyword-list button:active {
  transform: translateY(1px);
}

.pattern-card:focus-visible,
.sample-card:focus-visible,
.keyword-list button:focus-visible {
  outline: 2px solid var(--log-brand);
  outline-offset: 1px;
}

.pattern-card strong,
.pattern-card span,
.pattern-card small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pattern-card strong {
  color: var(--log-heading);
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 11.5px;
}

.pattern-card span,
.pattern-card small {
  color: var(--log-subtle);
  font-size: 10.5px;
}

.keyword-list {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.keyword-list button {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  padding: 7px 8px;
  font-size: 11px;
}

.sample-card {
  margin-bottom: 6px;
  padding: 8px;
  color: var(--log-muted);
  font-size: 11px;
  line-height: 1.5;
}

.analysis-panel :deep(.el-form-item) {
  margin-bottom: 9px;
}

.analysis-panel :deep(.el-form-item__label) {
  height: auto;
  margin-bottom: 5px;
  padding: 0;
  color: #334155;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.4;
}

.analysis-panel :deep(.el-form > .el-button) {
  width: 100%;
}

.ai-result {
  display: grid;
  gap: 8px;
  margin-top: 10px;
}

.ai-meta {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  color: var(--log-subtle);
  font-size: 11px;
}

.ai-rendered {
  display: grid;
  gap: 8px;
}

.ai-section {
  overflow: hidden;
  border: 1px solid var(--log-border-soft);
  border-left: 3px solid #c9d6ea;
  border-radius: 8px;
  background: #fff;
}

.ai-section.tone-summary {
  border-left-color: var(--log-brand);
}

.ai-section.tone-cause {
  border-left-color: var(--log-danger);
}

.ai-section.tone-impact {
  border-left-color: var(--log-warning);
}

.ai-section.tone-severity {
  border-left-color: #6366f1;
}

.ai-section.tone-actions,
.ai-section.tone-repair {
  border-left-color: var(--log-success);
}

.ai-section.tone-evidence {
  border-left-color: #7b8798;
}

.ai-section-head {
  padding: 7px 9px;
  border-bottom: 1px solid var(--log-border-soft);
  background: var(--log-subtle-bg);
}

.ai-section-head span {
  display: block;
  color: var(--log-heading);
  font-size: 12px;
  font-weight: 700;
}

.ai-section-body {
  display: grid;
  gap: 6px;
  padding: 8px 9px 9px;
}

.ai-paragraph {
  margin: 0;
  color: var(--log-muted);
  font-size: 11.5px;
  line-height: 1.65;
}

.ai-list-row {
  display: grid;
  grid-template-columns: 18px minmax(0, 1fr);
  align-items: start;
  gap: 7px;
  color: var(--log-muted);
  font-size: 11.5px;
  line-height: 1.65;
}

.ai-list-row.bullet i {
  width: 7px;
  height: 7px;
  margin-top: 6px;
  border-radius: 50%;
  background: var(--log-brand);
}

.ai-list-row.ordered strong {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: #e7f1ff;
  color: #17457f;
  font-size: 10.5px;
  font-weight: 700;
}

.ai-section-body :deep(strong) {
  color: var(--log-heading);
  font-weight: 700;
}

.ai-section-body :deep(code) {
  padding: 1px 5px;
  border-radius: 999px;
  background: #eef2f7;
  color: #17457f;
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 10.5px;
}

.ai-section-body :deep(mark) {
  padding: 1px 5px;
  border-radius: 999px;
  background: #fff4c2;
  color: #725200;
}

.ai-section-body :deep(.ai-metric) {
  color: #17457f;
  font-weight: 700;
}

.ai-section-body :deep(.ai-token) {
  display: inline-flex;
  align-items: center;
  padding: 1px 5px;
  border-radius: 999px;
  font-size: 10.5px;
  font-weight: 700;
  line-height: 1.4;
}

.ai-section-body :deep(.ai-token.level-error),
.ai-section-body :deep(.ai-token.level-critical),
.ai-section-body :deep(.ai-token.level-fatal) {
  background: #fff0f0;
  color: #b4232d;
}

.ai-section-body :deep(.ai-token.level-warn),
.ai-section-body :deep(.ai-token.level-warning) {
  background: #fff4e5;
  color: #8c5c00;
}

.ai-section-body :deep(.ai-token.level-info) {
  background: #eaf8ef;
  color: #1b7a52;
}

.ai-section-body :deep(.ai-token.level-unknown) {
  background: #f3f7fc;
  color: #596579;
}

.detail-block {
  display: grid;
  gap: 8px;
  margin-top: 12px;
}

.detail-block > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.detail-block pre {
  margin: 0;
  padding: 10px;
  border: 1px solid var(--log-border-soft);
  border-radius: 8px;
  background: var(--log-panel-soft);
  color: var(--log-muted);
  white-space: pre-wrap;
  word-break: break-word;
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 12px;
  line-height: 1.65;
}

.log-analysis-page :deep(.el-button) {
  border-radius: var(--ops-button-radius, 8px);
  font-weight: 600;
  transition:
    color 160ms ease,
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease,
    transform 160ms ease;
}

.log-analysis-page :deep(.el-button:not(.is-disabled):hover) {
  transform: translateY(-1px);
}

.log-analysis-page :deep(.el-button:not(.is-disabled):active) {
  transform: translateY(0);
}

.log-analysis-page :deep(.el-button:focus-visible) {
  outline: 2px solid var(--log-brand);
  outline-offset: 2px;
}

.log-analysis-page :deep(.el-input__wrapper),
.log-analysis-page :deep(.el-select__wrapper),
.log-analysis-page :deep(.el-textarea__inner),
.log-analysis-page :deep(.el-input-number .el-input__wrapper) {
  border-radius: var(--ops-input-radius, 8px);
  background: var(--log-panel-soft);
  box-shadow: inset 0 0 0 1px #d9e5f2 !important;
}

.log-analysis-page :deep(.el-input__wrapper:hover),
.log-analysis-page :deep(.el-select__wrapper:hover) {
  box-shadow: inset 0 0 0 1px var(--ops-border-hover, #b9d4ff) !important;
}

.log-analysis-page :deep(.el-input__wrapper.is-focus),
.log-analysis-page :deep(.el-select__wrapper.is-focused) {
  box-shadow: inset 0 0 0 1px #409eff !important;
}

.log-analysis-page :deep(.el-input__inner),
.log-analysis-page :deep(.el-select__placeholder),
.log-analysis-page :deep(.el-select__selected-item) {
  font-size: 12px;
}

.log-analysis-page :deep(.el-empty) {
  padding: 16px 0;
}

.log-analysis-page :deep(.el-empty__description) {
  margin-top: 8px;
}

.log-analysis-page :deep(.el-empty__description p) {
  font-size: 11px;
}

:global(.entry-drawer) {
  max-width: calc(100vw - 12px);
}

@media (max-width: 1500px) {
  .workspace {
    grid-template-columns: 260px minmax(480px, 1fr) 310px;
  }

  .query-toolbar {
    grid-template-columns: 1fr;
  }

  .query-actions {
    justify-content: flex-end;
  }

  .log-line {
    grid-template-columns: 56px 64px 146px minmax(0, 1fr);
  }
}

@media (max-width: 1180px) {
  .log-analysis-page {
    height: calc(100vh - 124px);
    overflow: auto;
  }

  .workspace {
    grid-template-columns: 264px minmax(0, 1fr);
  }

  .resource-panel,
  .log-workspace {
    min-height: 580px;
  }

  .analysis-panel {
    grid-column: 1 / -1;
    min-height: 360px;
  }

  .analysis-tabs :deep(.el-tab-pane[name="overview"]) {
    min-height: 280px;
  }

  .metric-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }
}

@media (max-width: 960px) {
  :global(.home-container .el-main:has(.log-analysis-page)) {
    padding-right: 8px;
    padding-left: 8px;
  }

  .log-analysis-page {
    height: auto;
    min-height: calc(100vh - 124px);
    overflow: visible;
  }

  .ops-page-header {
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .ops-page-header__identity,
  .ops-page-header__status {
    width: 100%;
  }

  .ops-page-header__status {
    justify-content: flex-end;
  }

  .workspace {
    grid-template-columns: minmax(0, 1fr);
  }

  .resource-panel,
  .log-workspace,
  .analysis-panel {
    min-height: 520px;
  }

  .analysis-panel {
    grid-column: auto;
  }

  .query-filters {
    grid-template-columns: minmax(160px, 1fr) 116px 108px;
  }

  .metric-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 620px) {
  .log-analysis-page {
    padding: 6px;
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

  .ops-page-header__status {
    align-items: center;
    justify-content: flex-start;
    flex-wrap: wrap;
  }

  .session-chip {
    flex: 1 1 130px;
    max-width: none;
  }

  .query-filters,
  .advanced-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .advanced-field--parser {
    grid-column: auto;
  }

  .query-actions {
    justify-content: flex-start;
    flex-wrap: wrap;
  }

  .query-actions :deep(.el-button:not(.query-icon-button)) {
    flex: 1 1 auto;
  }

  .resource-panel {
    min-height: 500px;
  }

  .log-workspace {
    min-height: 600px;
  }

  .analysis-panel {
    min-height: 520px;
  }

  .log-line {
    grid-template-columns: 48px 58px 120px minmax(90px, 1fr);
    padding: 0 6px;
  }

  .line-no,
  .line-level,
  .line-time,
  .log-line code {
    font-size: 10.5px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .log-analysis-page *,
  .log-analysis-page *::before,
  .log-analysis-page *::after {
    scroll-behavior: auto !important;
    transition-duration: 0.01ms !important;
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
  }
}
</style>
