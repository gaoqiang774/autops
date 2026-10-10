<template>
  <div class="log-query-page">
    <section class="page-card">
      <header class="page-header">
        <div class="page-header-copy">
          <span class="ops-page-header__icon" aria-hidden="true">
            <img :src="logSvg" alt="">
          </span>
          <div class="ops-page-header__content">
            <h1>日志查询</h1>
            <span class="ops-page-header__divider" aria-hidden="true"></span>
            <p class="page-description">按项目和日志库检索日志，统一完成趋势分析、字段筛选与日志明细查看。</p>
          </div>
        </div>
        <div class="summary-group">
          <div class="summary-chip">
            <span>命中日志</span>
            <strong>{{ total }}</strong>
          </div>
          <div class="summary-chip">
            <span>本次返回</span>
            <strong>{{ logs.length }}</strong>
          </div>
          <div class="summary-chip">
            <span>耗时</span>
            <strong>{{ durationText }}</strong>
          </div>
        </div>
      </header>

      <div class="query-stack">
          <section class="query-insight-panel">
            <div class="scope-toolbar">
              <el-form class="scope-form" :model="queryForm" inline>
                <el-form-item label="项目">
                  <el-select
                    v-model="queryForm.projectId"
                    placeholder="选择项目"
                    clearable
                    filterable
                    @change="handleProjectChange"
                  >
                    <el-option v-for="item in projects" :key="item.id" :label="item.name" :value="item.id" />
                  </el-select>
                </el-form-item>

                <el-form-item label="日志库">
                  <el-select
                    v-model="queryForm.storeId"
                    placeholder="选择日志库"
                    clearable
                    filterable
                    @change="handleScopeChange"
                  >
                    <el-option
                      v-for="item in stores"
                      :key="item.id"
                      :label="item.name || '-'"
                      :value="item.id"
                    />
                  </el-select>
                </el-form-item>

              </el-form>
            </div>

            <div class="query-toolbar">
              <el-form :model="queryForm" class="query-form" @submit.prevent>
                <el-form-item label="查询语句" class="query-input-item">
                  <el-input
                    v-model.trim="queryForm.query"
                    class="query-main-input"
                    placeholder="请输入关键字或查询语句"
                    clearable
                    @keyup.enter="handleSearch"
                    @clear="clearQueryResults"
                  />
                </el-form-item>
                <el-form-item label="时间范围" class="time-item">
                  <el-date-picker
                    v-model="queryForm.timeRange"
                    type="datetimerange"
                    range-separator="至"
                    start-placeholder="开始时间"
                    end-placeholder="结束时间"
                    value-format="YYYY-MM-DD HH:mm:ss"
                    :shortcuts="dateShortcuts"
                    @change="handleTimeRangeChange"
                  />
                </el-form-item>
                <el-form-item label="粒度">
                  <el-select v-model="queryForm.step" class="step-select" @change="handleSearch">
                    <el-option label="5 秒" value="5s" />
                    <el-option label="30 秒" value="30s" />
                    <el-option label="1 分钟" value="1m" />
                    <el-option label="5 分钟" value="5m" />
                  </el-select>
                </el-form-item>
                <el-form-item label="条数">
                  <el-input-number v-model="queryForm.limit" :min="10" :max="1000" :step="10" controls-position="right" />
                </el-form-item>
                <el-form-item class="actions-item">
                  <div class="toolbar-actions">
                    <el-button type="primary" size="small" :icon="Search" :loading="logsLoading || hitsLoading" @click="handleSearch">查询 / 分析</el-button>
                    <el-button type="warning" plain size="small" @click="clearQueryConditions">清空条件</el-button>
                  </div>
                </el-form-item>
              </el-form>

              <div class="quick-ranges">
                <button
                  v-for="item in quickRanges"
                  :key="item.value"
                  type="button"
                  :class="{ active: activeRange === item.value }"
                  @click="setQuickRange(item.value)"
                >
                  近{{ item.label }}
                </button>
                <button type="button" @click="refreshCurrentRange">刷新</button>
              </div>

              <div v-if="commonQueryActions.length" class="common-query-actions">
                <button
                  v-for="item in commonQueryActions"
                  :key="item.key"
                  type="button"
                  :class="{ 'analysis-action': item.kind === 'analysis' }"
                  :title="item.query"
                  @click="runCommonQueryAction(item)"
                >
                  {{ item.label }}
                </button>
              </div>

              <div v-if="filterEntries.length" class="filter-row">
                <el-tag
                  v-for="item in filterEntries"
                  :key="`${item.mode}-${item.field}`"
                  closable
                  effect="plain"
                  :type="item.mode === 'exclude' ? 'warning' : ''"
                  @close="removeFilter(item)"
                >
                  <span class="mono-text">{{ item.field }}</span> {{ item.mode === 'exclude' ? '!=' : '=' }} {{ item.value }}
                </el-tag>
                <el-button link type="danger" @click="clearFilters">清空过滤</el-button>
              </div>
            </div>

            <div class="trend-panel">
              <div class="panel-head">
                <div>
                  <h3>日志条数：{{ total }}</h3>
                </div>
                <div class="chart-actions">
                  <button type="button">加入仪表盘</button>
                  <button type="button">添加告警</button>
                  <el-dropdown trigger="click">
                    <button type="button" class="download-action" :disabled="downloadLoading" @click="downloadLogs('txt')">
                      {{ downloadLoading ? '下载中' : '下载' }}
                    </button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item @click="downloadLogs('txt')">TXT 原始日志</el-dropdown-item>
                        <el-dropdown-item @click="downloadLogs('csv')">CSV 表格</el-dropdown-item>
                        <el-dropdown-item @click="downloadLogs('jsonl')">JSONL</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </div>
              <div
                ref="trendChartRef"
                v-loading="hitsLoading"
                class="trend-chart"
                role="img"
                aria-label="日志趋势图，可在图表内拖动框选时间范围并查询"
              ></div>
              <span class="sr-only" aria-live="polite">{{ trendSelectionAnnouncement }}</span>
            </div>
          </section>

          <section class="log-result-panel">
            <aside class="parse-panel">
              <div class="panel-head">
                <div>
                  <h3>可用字段</h3>
                </div>
                <el-button link type="primary" :loading="fieldsLoading" @click="loadFields">刷新</el-button>
              </div>

              <div class="field-section">
                <div class="section-title">
                </div>
                <el-input v-model.trim="fieldKeyword" size="small" placeholder="搜索字段" clearable>
                  <template #prefix>
                    <el-icon><Search /></el-icon>
                  </template>
                </el-input>
                <div class="field-list">
                  <div v-for="group in fieldGroups" :key="group.key" class="field-group">
                    <div class="field-group-title">
                      <button
                        type="button"
                        class="field-group-toggle"
                        :aria-expanded="!isFieldGroupCollapsed(group)"
                        @click="toggleFieldGroup(group)"
                      >
                        <span class="field-group-label" @click.stop="toggleFieldGroup(group)">
                          <span
                            class="field-group-caret"
                            :class="{ collapsed: isFieldGroupCollapsed(group) }"
                            aria-hidden="true"
                          ></span>
                          <span class="field-group-name">{{ group.title }}</span>
                        </span>
                        <em>{{ group.fields.length }}</em>
                      </button>
                    </div>
                    <div v-show="!isFieldGroupCollapsed(group)" class="field-group-body">
                      <div v-for="field in group.fields" :key="field.name" class="field-item-wrap">
                        <button
                          type="button"
                          class="field-item"
                          :class="{ active: selectedField === field.name }"
                          :title="fieldTitle(field)"
                          @click="selectField(field.name)"
                        >
                          <span class="field-type-dot" :class="fieldTypeClass(field.name)"></span>
                          <span class="mono-text">{{ field.name }}</span>
                          <em v-if="fieldPercentText(field)" class="field-percent">{{ fieldPercentText(field) }}</em>
                        </button>
                        <div v-if="selectedField === field.name && topNExpanded" v-loading="fieldValuesLoading" class="inline-topn-list">
                          <div v-for="item in fieldValues" :key="`${field.name}-${item.value}-${item.count}-${item.synthetic ? 'other' : 'value'}`" class="inline-topn-item">
                            <span>{{ item.value || '-' }}</span>
                            <strong>{{ item.count || '-' }}</strong>
                            <em v-if="fieldPercentText(item)" class="field-percent">{{ fieldPercentText(item) }}</em>
                            <button v-if="!item.synthetic" type="button" title="加入筛选" @click.stop="addFilter(field.name, item.value)">+</button>
                            <button v-if="!item.synthetic" type="button" title="排除筛选" @click.stop="excludeFilter(field.name, item.value)">-</button>
                          </div>
                          <el-empty v-if="!fieldValues.length && !fieldValuesLoading" description="暂无字段值" :image-size="52" />
                        </div>
                      </div>
                    </div>
                  </div>
                  <el-empty v-if="!fieldGroups.length && !fieldsLoading" description="暂无字段" :image-size="70" />
                </div>
              </div>
            </aside>

            <main class="detail-panel">
            <div class="panel-head">
              <div>
                <h3>{{ resultTitle }}</h3>
              </div>
              <div class="result-head-actions">
                <el-tooltip content="最多分析当前结果中最新的 100 行日志" placement="top">
                  <el-button
                    class="ai-analysis-trigger"
                    type="primary"
                    plain
                    size="small"
                    :icon="MagicStick"
                    :loading="aiAnalysis.loading"
                    :disabled="!logs.length || logsLoading"
                    @click="runAiAnalysis"
                  >
                    AI 分析
                  </el-button>
                </el-tooltip>
                <el-radio-group v-model="logView" size="small">
                  <el-radio-button label="raw">原始日志</el-radio-button>
                  <el-radio-button label="json">JSON</el-radio-button>
                  <el-radio-button label="table">表格</el-radio-button>
                </el-radio-group>
                <span class="muted-text">第 {{ pagination.page }} 页</span>
              </div>
            </div>

            <div class="table-shell">
              <div v-if="logView === 'raw'" v-loading="logsLoading" class="raw-log-list">
                <article v-for="(row, index) in logs" :key="row._rowKey" class="raw-log-item">
                  <div class="raw-log-index">{{ logRowNumber(index) }}</div>
                  <div class="raw-log-body">
                    <div class="raw-log-head">
                      <span class="mono-text raw-log-time">{{ logTime(row) }}</span>
                      <el-tag size="small" effect="plain" :type="levelTagType(logLevel(row))">
                        {{ logLevel(row) }}
                      </el-tag>
                      <div class="row-actions">
                        <el-button link type="primary" @click="openLogDetail(row)">详情</el-button>
                        <el-button link type="primary" @click="copyLog(row)">复制</el-button>
                        <el-button link type="primary" @click="addRowFilter(row)">筛选</el-button>
                        <el-button link type="warning" @click="excludeRowFilter(row)">排除</el-button>
                        <el-button link type="danger" @click="createAlertFromRow(row)">告警</el-button>
                      </div>
                    </div>
                    <div class="raw-log-main-fields">
                      <span v-for="item in coreLogFields(row)" :key="item.key">
                        <strong>{{ item.label }}</strong>{{ item.value }}
                      </span>
                    </div>
                    <div class="raw-log-message">
                      <span
                        v-for="part in highlightText(logMessage(row))"
                        :key="part.key"
                        :class="{ 'query-hit-highlight': part.hit }"
                      >
                        {{ part.text }}
                      </span>
                    </div>
                  </div>
                </article>
                <el-empty v-if="!logs.length && !logsLoading" description="暂无日志" :image-size="96" />
              </div>
              <el-table
                v-else-if="logView === 'table'"
                v-loading="logsLoading"
                :data="logs"
                class="data-table"
                height="100%"
                row-key="_rowKey"
                empty-text="暂无日志"
              >
                <el-table-column
                  v-for="column in resolvedTableColumns"
                  :key="column.key"
                  :label="column.label"
                  :width="column.width || undefined"
                  :min-width="column.minWidth"
                  :align="column.align || 'left'"
                  :header-align="column.align || 'left'"
                  :show-overflow-tooltip="column.overflow"
                >
                  <template #default="{ row }">
                    <span :class="tableCellClass(column)">
                      <template v-if="column.highlight">
                        <span
                          v-for="part in highlightText(tableCellText(row, column))"
                          :key="part.key"
                          :class="{ 'query-hit-highlight': part.hit }"
                        >
                          {{ part.text }}
                        </span>
                      </template>
                      <template v-else>{{ tableCellText(row, column) }}</template>
                    </span>
                  </template>
                </el-table-column>
                <el-table-column v-if="displayMode !== 'aggregate'" label="操作" fixed="right" width="80">
                  <template #default="{ row }">
                    <el-button link type="primary" @click="openLogDetail(row)">详情</el-button>
                  </template>
                </el-table-column>
              </el-table>
              <div v-else v-loading="logsLoading" class="json-log-list">
                <article v-for="(row, index) in logs" :key="row._rowKey" class="json-log-item">
                  <div class="json-log-index">{{ logRowNumber(index) }}</div>
                  <div class="json-log-body">
                    <div class="json-log-head">
                      <span class="mono-text json-log-time">{{ compactLogTime(row) }}</span>
                      <el-tag size="small" effect="plain" :type="levelTagType(logLevel(row))">
                        {{ logLevel(row) }}
                      </el-tag>
                      <div class="row-actions">
                        <el-button link type="primary" @click="openLogDetail(row)">详情</el-button>
                        <el-button link type="primary" @click="copyLog(row)">复制</el-button>
                        <el-button link type="primary" @click="addRowFilter(row)">筛选</el-button>
                        <el-button link type="warning" @click="excludeRowFilter(row)">排除</el-button>
                        <el-button link type="danger" @click="createAlertFromRow(row)">告警</el-button>
                      </div>
                    </div>
                    <div class="json-field-list">
                      <div v-for="item in jsonFieldRows(row)" :key="item.key" class="json-field-row">
                        <span class="mono-text json-field-key">{{ item.key }}:</span>
                        <span
                          class="mono-text json-field-value"
                          :class="{ collapsed: item.collapsible && !isJsonFieldExpanded(row, item.key) }"
                        >
                          <span class="json-field-content">
                            <el-tag v-if="item.isJson" class="json-type-tag" size="small" type="primary" effect="dark">JSON</el-tag>
                            <span
                              v-for="part in highlightText(item.value)"
                              :key="part.key"
                              :class="{ 'query-hit-highlight': part.hit }"
                            >
                              {{ part.text }}
                            </span>
                          </span>
                          <button
                            v-if="item.collapsible"
                            type="button"
                            class="json-collapse-btn"
                            @click="toggleJsonField(row, item.key)"
                          >
                            {{ isJsonFieldExpanded(row, item.key) ? '收起' : '展开' }}
                          </button>
                        </span>
                      </div>
                    </div>
                  </div>
                </article>
                <el-empty v-if="!logs.length && !logsLoading" description="暂无日志" :image-size="96" />
              </div>
            </div>

            <div class="pagination-section">
              <el-pagination
                :current-page="pagination.page"
                :page-size="pagination.pageSize"
                :page-sizes="[50, 100, 200, 500]"
                :total="total"
                layout="total, sizes, prev, pager, next, jumper"
                @size-change="handleSizeChange"
                @current-change="handlePageChange"
              />
            </div>
            </main>
          </section>
      </div>
    </section>

    <el-drawer v-model="detailDrawer.visible" title="日志详情" size="52%" class="log-detail-drawer">
      <el-tabs v-model="detailDrawer.activeTab" class="detail-tabs">
        <el-tab-pane label="字段表" name="fields">
          <div class="detail-field-table">
            <div v-for="item in detailFieldRows(detailDrawer.row)" :key="item.key" class="detail-field-row">
              <span class="mono-text">{{ item.key }}</span>
              <strong>{{ item.value }}</strong>
            </div>
          </div>
        </el-tab-pane>
        <el-tab-pane label="JSON" name="json">
          <pre class="json-view detail-json"><span
            v-for="part in highlightText(stringify(detailDrawer.row || {}))"
            :key="part.key"
            :class="{ 'query-hit-highlight': part.hit }"
          >{{ part.text }}</span></pre>
        </el-tab-pane>
        <el-tab-pane label="原文" name="raw">
          <pre class="json-view detail-json"><span
            v-for="part in highlightText(logMessage(detailDrawer.row || {}))"
            :key="part.key"
            :class="{ 'query-hit-highlight': part.hit }"
          >{{ part.text }}</span></pre>
        </el-tab-pane>
      </el-tabs>
    </el-drawer>

    <el-drawer v-model="aiAnalysis.visible" title="AI 日志分析" size="min(560px, 92vw)" class="ai-analysis-drawer">
      <div class="ai-analysis-content">
        <div class="ai-analysis-meta">
          <el-tag type="primary" effect="plain">最新 {{ aiAnalysis.usedLogLines || aiAnalysis.requestedLogLines }} 行</el-tag>
          <span v-if="aiAnalysis.providerName">{{ aiAnalysis.providerName }}</span>
          <span v-if="aiAnalysis.generatedAt">{{ aiAnalysis.generatedAt }}</span>
          <el-button
            v-if="aiAnalysis.analysis"
            link
            type="primary"
            :icon="CopyDocument"
            @click="copyText(aiAnalysis.analysis)"
          >
            复制
          </el-button>
        </div>

        <el-skeleton v-if="aiAnalysis.loading" :rows="9" animated />
        <el-alert
          v-else-if="aiAnalysis.error"
          :title="aiAnalysis.error"
          type="error"
          show-icon
          :closable="false"
        />
        <div v-else-if="aiAnalysisSections.length" class="ai-analysis-sections">
          <section v-for="section in aiAnalysisSections" :key="section.key" class="ai-analysis-section">
            <h3>{{ section.title }}</h3>
            <div class="ai-analysis-section-body">{{ section.content }}</div>
          </section>
        </div>
        <el-empty v-else description="暂无分析结果" :image-size="96" />
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import * as echarts from 'echarts'
import { ElMessage } from 'element-plus'
import { CopyDocument, MagicStick, Search } from '@element-plus/icons-vue'
import aiopsApi from '@/api/aiops'
import logtailApi from '@/api/logtail'
import logSvg from '@/assets/image/日志.svg'

const route = useRoute()
const LOG_QUERY_PREFERENCES_KEY = 'autoops:log-query:preferences:v1'

const projects = ref([])
const stores = ref([])
const logs = ref([])
const fields = ref([])
const fieldBuckets = ref(createEmptyFieldBuckets())
const logFieldOrder = ref([])
const displayMode = ref('raw')
const responseLogType = ref('')
const tableColumns = ref([])
const quickFilters = ref([])
const analysisPresets = ref([])
const fieldValues = ref([])
const fieldKeyword = ref('')
const selectedField = ref('')
const topNExpanded = ref(false)
const collapsedFieldGroups = ref(new Set())
const logView = ref('raw')
const logsLoading = ref(false)
const hitsLoading = ref(false)
const fieldsLoading = ref(false)
const fieldValuesLoading = ref(false)
const downloadLoading = ref(false)
const total = ref(0)
const logsTotal = ref(null)
const hitsTotal = ref(null)
const backendQuery = ref('')
const durationMs = ref(0)
const activeRange = ref('15m')
const hasSearched = ref(false)
const trendPoints = ref([])
const trendBucketTotal = ref(null)
const trendChartRef = ref()
const trendSelectionAnnouncement = ref('')
const expandedJsonFields = ref(new Set())
let trendChart = null
let suppressTrendClickUntil = 0

const detailDrawer = reactive({
  visible: false,
  row: null,
  activeTab: 'fields'
})

const aiAnalysis = reactive({
  visible: false,
  loading: false,
  error: '',
  analysis: '',
  providerName: '',
  generatedAt: '',
  requestedLogLines: 0,
  usedLogLines: 0
})

const queryForm = reactive({
  projectId: '',
  storeId: '',
  query: route.query.q || '',
  timeRange: defaultTimeRange(15),
  step: '30s',
  limit: 50,
  extraFilters: {},
  excludeFilters: {}
})

const pagination = reactive({
  page: 1,
  pageSize: 50
})

const quickRanges = [
  { label: '1m', value: '1m', minutes: 1 },
  { label: '5m', value: '5m', minutes: 5 },
  { label: '15m', value: '15m', minutes: 15 },
  { label: '1h', value: '1h', minutes: 60 },
  { label: '4h', value: '4h', minutes: 240 },
  { label: '24h', value: '24h', minutes: 1440 },
  { label: '7d', value: '7d', minutes: 10080 }
]

const dateShortcuts = quickRanges.map(item => ({
  text: `最近 ${item.label}`,
  value: () => {
    const end = new Date()
    const start = new Date(end.getTime() - item.minutes * 60 * 1000)
    return [start, end]
  }
}))

const readLogQueryPreferences = () => {
  try {
    const value = window.localStorage.getItem(LOG_QUERY_PREFERENCES_KEY)
    if (!value) return {}
    const preferences = JSON.parse(value)
    return preferences && typeof preferences === 'object' ? preferences : {}
  } catch (error) {
    console.warn('读取日志查询偏好失败:', error)
    return {}
  }
}

const saveLogQueryPreferences = () => {
  try {
    window.localStorage.setItem(LOG_QUERY_PREFERENCES_KEY, JSON.stringify({
      projectId: queryForm.projectId,
      storeId: queryForm.storeId,
      activeRange: activeRange.value,
      timeRange: activeRange.value ? null : [...(queryForm.timeRange || [])]
    }))
  } catch (error) {
    console.warn('保存日志查询偏好失败:', error)
  }
}

const findAvailableOptionId = (options, candidate) => {
  if (candidate === undefined || candidate === null || candidate === '') return ''
  const option = options.find(item => String(item?.id) === String(candidate))
  return option?.id ?? ''
}

const routeQueryValue = key => {
  const value = route.query[key]
  return Array.isArray(value) ? value[0] : value
}

const restoreTimePreference = preferences => {
  const quickRange = quickRanges.find(item => item.value === preferences?.activeRange)
  if (quickRange) {
    activeRange.value = quickRange.value
    queryForm.timeRange = defaultTimeRange(quickRange.minutes)
    return
  }

  const [savedStart, savedEnd] = Array.isArray(preferences?.timeRange) ? preferences.timeRange : []
  const start = parseTimeValue(savedStart)
  const end = parseTimeValue(savedEnd)
  if (preferences?.activeRange === '' && start && end && start <= end) {
    activeRange.value = ''
    queryForm.timeRange = [formatDateTime(start), formatDateTime(end)]
    return
  }

  activeRange.value = '15m'
  queryForm.timeRange = defaultTimeRange(15)
}

const selectedFields = [
  '_time',
  '_msg',
  'severity_text',
  'app',
  'log_source',
  'file_path',
  'host_name',
  'agent_id',
  'project_code',
  'store_code',
  'trace_id',
  'message',
  'cluster',
  'env',
  'log_type',
  'k8s_container_name',
  'k8s_namespace',
  'k8s_pod_name'
]

const displayFieldSet = new Set(selectedFields)

const displayModeSet = new Set(['raw', 'structured', 'aggregate'])
const rawMessageKeys = ['message', '_raw', '_msg', 'msg', 'log', 'content', 'raw']
const structuredTitleMap = {
  json: 'JSON 日志',
  nginx: 'Nginx 日志',
  regex: '正则日志',
  txt: '文本日志',
  db_slow_log: '慢日志'
}
const columnLabelMap = {
  _time: '时间',
  '@timestamp': '时间',
  timestamp: '时间',
  time: '时间',
  host_name: '主机',
  hostname: '主机',
  host: '主机',
  file_path: '路径',
  path: '路径',
  message: '日志内容',
  _msg: '日志内容',
  _raw: '原文',
  raw: '原文',
  level: '级别',
  severity_text: '级别',
  log_source: '来源',
  app: '应用',
  remote_addr: '访问 IP',
  client_ip: '访问 IP',
  request_method: '方法',
  request_uri: '请求 URI',
  request_path: '接口',
  status: '状态码',
  body_bytes_sent: '响应字节',
  http_referer: 'Referer',
  http_user_agent: 'User-Agent',
  requests: '请求数',
  count: '数量',
  total: '数量',
  query_time: '执行耗时',
  lock_time: '锁等待',
  rows_sent: '返回行数',
  rows_examined: '扫描行数',
  db_user: '用户',
  db_host: '来源主机',
  database: '数据库',
  sql_text: 'SQL',
  sql_operation: 'SQL 操作',
  command_type: 'SQL 类型'
}
const defaultRawTableColumns = [
  { key: '_time', label: '时间', keys: ['_time', '@timestamp', 'timestamp', 'time'], minWidth: 190, mono: true, overflow: true },
  { key: 'host_name', label: '主机', keys: ['host_name', 'hostname', 'host'], minWidth: 150, overflow: true },
  { key: 'file_path', label: '路径', keys: ['file_path', 'path'], minWidth: 220, mono: true, overflow: true },
  { key: 'message', label: '原文', keys: rawMessageKeys, minWidth: 420, mono: true, highlight: true, overflow: true }
]
const timestampColumns = new Set(['_time', '@timestamp', 'timestamp', 'time'])
const wideColumnPattern = /(_raw|message|sql|query|statement|user_agent|referer|request_uri|request_path|file_path|path)$/i
const monoColumnPattern = /(^_|time|timestamp|date|path|uri|url|sql|query|trace|id|ip|addr|bytes|rows|count|status|duration|cost|latency|elapsed)/i
const numericColumnPattern = /(^count$|_count$|requests|bytes|rows|status|duration|cost|latency|elapsed|total|num|size|sent|examined|query_time|lock_time)$/i

const fieldGroups = computed(() => {
  const buckets = fieldBuckets.value
  return [
    { key: 'indexFields', title: '重点索引字段', fields: filterFieldList(buckets.indexFields), collapsible: true },
    { key: 'normalFields', title: '普通字段', fields: filterFieldList(buckets.normalFields), collapsible: true }
  ].filter(group => group.fields.length)
})

const filterEntries = computed(() => [
  ...Object.entries(queryForm.extraFilters).map(([field, value]) => ({ field, value, mode: 'include' })),
  ...Object.entries(queryForm.excludeFilters).map(([field, value]) => ({ field, value, mode: 'exclude' }))
])

const commonQueryActions = computed(() => [
  ...quickFilters.value.map(item => ({ ...item, key: `quick-${item.key}`, kind: 'quick' })),
  ...analysisPresets.value.map(item => ({ ...item, key: `analysis-${item.key}`, kind: 'analysis' }))
])

const durationText = computed(() => {
  if (!durationMs.value) return '-'
  if (durationMs.value < 1000) return `${durationMs.value}ms`
  return `${(durationMs.value / 1000).toFixed(1)}s`
})

const resultTitle = computed(() => {
  if (displayMode.value === 'aggregate') return '分析结果'
  if (displayMode.value === 'structured') return structuredTitleMap[responseLogType.value] || '结构化日志'
  return '原始日志'
})

const aiAnalysisSections = computed(() => parseAiAnalysisSections(aiAnalysis.analysis))

function parseAiAnalysisSections(text) {
  const sections = []
  let current = { title: '分析结果', lines: [] }

  String(text || '').replace(/\r/g, '').split('\n').forEach(rawLine => {
    const line = rawLine.trim()
    const heading = line.match(/^#{1,6}\s+(.+)$/) || line.match(/^\*\*(.+?)\*\*$/)
    if (heading) {
      if (current.lines.some(Boolean)) sections.push(current)
      current = { title: heading[1].trim(), lines: [] }
      return
    }
    current.lines.push(rawLine)
  })

  if (current.lines.some(Boolean)) sections.push(current)
  return sections.map((section, index) => ({
    key: `${index}-${section.title}`,
    title: section.title,
    content: section.lines.join('\n').trim()
  }))
}

const resolvedTableColumns = computed(() => {
  if (tableColumns.value.length) return tableColumns.value
  if (displayMode.value === 'raw') return defaultRawTableColumns.map(cloneTableColumn)

  const derivedColumns = buildColumnsFromRows(logs.value)
  return derivedColumns.length ? derivedColumns : defaultRawTableColumns.map(cloneTableColumn)
})

const highlightTerms = computed(() => {
  const query = String(queryForm.query || '').trim()
  if (!query || query === '*') return []

  const terms = new Map()
  const addTerm = value => {
    const term = String(value || '').trim()
    if (!term || term === '*') return
    if (/^(AND|OR|NOT|TO)$/i.test(term)) return
    if (term.length < 2 && !/[\u4e00-\u9fa5]/.test(term)) return
    terms.set(term.toLowerCase(), term)
  }

  const quotedPattern = /"([^"]+)"|'([^']+)'/g
  let match
  while ((match = quotedPattern.exec(query)) !== null) {
    addTerm(match[1] || match[2])
  }

  const queryWithoutQuoted = query.replace(quotedPattern, ' ')
  const hasQuerySyntax = /[:(){}[\]^~*?]|\b(?:AND|OR|NOT|TO)\b/i.test(queryWithoutQuoted)
  if (!hasQuerySyntax) addTerm(queryWithoutQuoted)

  queryWithoutQuoted
    .replace(/[()+{}[\]^~*?]/g, ' ')
    .split(/\s+/)
    .forEach(token => {
      const cleaned = token.replace(/^[+\-!]+|[,;]+$/g, '')
      if (!cleaned) return
      if (cleaned.includes(':')) {
        const [, value = ''] = cleaned.split(/:(.+)/)
        addTerm(value)
        return
      }
      addTerm(cleaned)
    })

  return Array.from(terms.values()).sort((a, b) => b.length - a.length).slice(0, 20)
})

const highlightRegex = computed(() => {
  if (!highlightTerms.value.length) return null
  return new RegExp(highlightTerms.value.map(escapeRegExp).join('|'), 'gi')
})

function defaultTimeRange(minutes) {
  const end = new Date()
  const start = new Date(end.getTime() - minutes * 60 * 1000)
  return [formatDateTime(start), formatDateTime(end)]
}

function formatDateTime(date) {
  const pad = value => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function normalizeDateInput(value) {
  return String(value)
    .replace(' ', 'T')
    .replace(/(\.\d{3})\d+/, '$1')
}

function parseStepMs(step) {
  const match = String(step || '30s').match(/^(\d+)(ms|s|m|h|d)$/)
  if (!match) return 30 * 1000
  const value = Number(match[1])
  const unit = match[2]
  const unitMs = { ms: 1, s: 1000, m: 60 * 1000, h: 60 * 60 * 1000, d: 24 * 60 * 60 * 1000 }
  return value * unitMs[unit]
}

function parseTimeValue(value) {
  if (value === undefined || value === null || value === '') return null
  if (value instanceof Date) return Number.isNaN(value.getTime()) ? null : value
  if (typeof value === 'number') {
    const timestamp = value < 10 ** 12 ? value * 1000 : value
    const date = new Date(timestamp)
    return Number.isNaN(date.getTime()) ? null : date
  }
  const date = new Date(normalizeDateInput(value))
  return Number.isNaN(date.getTime()) ? null : date
}

function toRfc3339(value) {
  if (!value) return ''
  const date = value instanceof Date ? value : new Date(normalizeDateInput(value))
  if (Number.isNaN(date.getTime())) return value
  const offset = -date.getTimezoneOffset()
  const sign = offset >= 0 ? '+' : '-'
  const abs = Math.abs(offset)
  const pad = num => String(num).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}${sign}${pad(Math.floor(abs / 60))}:${pad(abs % 60)}`
}

const extractData = response => response?.data?.data ?? response?.data ?? response

const extractList = payload => {
  const data = extractData(payload)
  if (Array.isArray(data)) return data
  return data?.list || data?.items || data?.logs || data?.fields || data?.values || []
}

const loadProjects = async (preferences = {}) => {
  try {
    const response = await logtailApi.getProjects({ page: 1, pageSize: 100, status: 1 })
    projects.value = extractList(response)
    queryForm.projectId = findAvailableOptionId(projects.value, routeQueryValue('projectId'))
      || findAvailableOptionId(projects.value, preferences.projectId)
      || projects.value[0]?.id
      || ''
    await loadStores([routeQueryValue('storeId'), preferences.storeId])
  } catch (error) {
    console.error('加载日志项目失败:', error)
    projects.value = []
  }
}

const loadStores = async (preferredStoreIds = []) => {
  if (!queryForm.projectId) {
    stores.value = []
    return
  }

  try {
    const response = await logtailApi.getStores({ page: 1, pageSize: 100, projectId: queryForm.projectId, status: 1 })
    stores.value = extractList(response)
    const candidates = Array.isArray(preferredStoreIds) ? preferredStoreIds : [preferredStoreIds]
    queryForm.storeId = candidates
      .map(candidate => findAvailableOptionId(stores.value, candidate))
      .find(id => id !== '')
      || stores.value[0]?.id
      || ''
  } catch (error) {
    console.error('加载日志库失败:', error)
    stores.value = []
  }
}

const buildQueryPayload = () => {
  const [start, end] = queryForm.timeRange || []
  const payload = {
    query: buildQueryText(),
    start: toRfc3339(start),
    end: toRfc3339(end),
    timeout: '10s',
    extraFilters: { ...queryForm.extraFilters }
  }

  if (queryForm.storeId) {
    payload.storeId = queryForm.storeId
  }

  return payload
}

const escapeQueryValue = value => String(value ?? '').replace(/\\/g, '\\\\').replace(/"/g, '\\"')

const buildQueryText = () => {
  const baseQuery = queryForm.query || '*'
  const excludes = Object.entries(queryForm.excludeFilters)
    .filter(([, value]) => value !== undefined && value !== null && value !== '')
    .map(([field, value]) => `NOT ${field}:"${escapeQueryValue(value)}"`)
  return excludes.length ? `${baseQuery} ${excludes.join(' ')}` : baseQuery
}

const downloadExtMap = {
  txt: 'txt',
  csv: 'csv',
  jsonl: 'jsonl'
}

const downloadTypeMap = {
  txt: 'text/plain;charset=utf-8',
  csv: 'text/csv;charset=utf-8',
  jsonl: 'application/x-ndjson;charset=utf-8'
}

const parseDownloadFileName = response => {
  const disposition = response?.headers?.['content-disposition'] || response?.headers?.['Content-Disposition'] || ''
  const encodedMatch = disposition.match(/filename\*=UTF-8''([^;]+)/i)
  if (encodedMatch?.[1]) {
    try {
      return decodeURIComponent(encodedMatch[1])
    } catch (error) {
      console.warn('解析下载文件名失败:', error)
    }
  }

  const plainMatch = disposition.match(/filename="?([^";]+)"?/i)
  return plainMatch?.[1] || ''
}

const defaultDownloadFileName = format => {
  const store = stores.value.find(item => item.id === queryForm.storeId)
  const code = String(store?.code || store?.name || 'logs').replace(/[^\w.-]+/g, '_')
  const timestamp = new Date().toISOString().slice(0, 19).replace(/[-:T]/g, '')
  return `${code}_${timestamp}.${downloadExtMap[format] || 'txt'}`
}

const saveBlob = (blob, filename) => {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.style.display = 'none'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

const downloadLogs = async (format = 'txt') => {
  if (!queryForm.storeId) {
    ElMessage.warning('请先选择日志库')
    return
  }

  downloadLoading.value = true
  try {
    ElMessage.info('正在准备下载...')
    const response = await logtailApi.downloadLogs({
      ...buildQueryPayload(),
      limit: 10000,
      format
    })
    const blob = response?.data instanceof Blob
      ? response.data
      : new Blob([response?.data || ''], { type: downloadTypeMap[format] || downloadTypeMap.txt })
    const filename = parseDownloadFileName(response) || defaultDownloadFileName(format)
    saveBlob(blob, filename)
    ElMessage.success('下载已开始')
  } catch (error) {
    console.error('下载日志失败:', error)
    ElMessage.error('下载失败，请稍后重试')
  } finally {
    downloadLoading.value = false
  }
}

const loadLogs = async () => {
  if (!queryForm.storeId) return
  logsLoading.value = true
  try {
    const response = await logtailApi.queryLogs({
      ...buildQueryPayload(),
      step: queryForm.step,
      limit: queryForm.limit,
      offset: (pagination.page - 1) * pagination.pageSize,
      page: pagination.page,
      pageSize: pagination.pageSize,
      includeTotal: true
    })
    const data = extractData(response)
    const list = data?.logs || data?.list || data?.items || (Array.isArray(data) ? data : [])
    syncResponseMetadata(data)
    logs.value = list.map((item, index) => {
      const rawItem = item && typeof item === 'object' ? item : { message: String(item ?? '') }
      return {
        ...rawItem,
        __raw: rawItem,
        _rowKey: `${pagination.page}-${index}-${rawItem._time || rawItem.time || rawItem.id || Math.random()}`
      }
    })
    expandedJsonFields.value = new Set()
    const responseTotal = Number(data?.total || data?.count || list.length || 0)
    logsTotal.value = Number.isFinite(responseTotal) ? responseTotal : 0
    applyQueryTotal()
    backendQuery.value = data?.query || ''
    durationMs.value = Number(data?.durationMs || data?.duration || 0)
  } catch (error) {
    console.error('查询日志失败:', error)
    logs.value = []
    resetResponseMetadata()
    logsTotal.value = null
    applyQueryTotal()
  } finally {
    logsLoading.value = false
  }
}

const loadHits = async () => {
  if (!queryForm.storeId) return
  hitsLoading.value = true
  try {
    const response = await logtailApi.queryHits({
      ...buildQueryPayload(),
      step: queryForm.step,
      fields: [],
      fieldsLimit: 10
    })
    const data = extractData(response)
    const points = normalizeTrend(data)
    hitsTotal.value = extractHitsTotal(data, points)
    applyQueryTotal()
    renderTrend(points)
  } catch (error) {
    console.error('查询日志趋势失败:', error)
    hitsTotal.value = null
    applyQueryTotal()
    renderTrend([])
  } finally {
    hitsLoading.value = false
  }
}

const loadFields = async () => {
  if (!queryForm.storeId) return
  fieldsLoading.value = true
  try {
    const response = await logtailApi.queryFields({
      ...buildQueryPayload(),
      filter: fieldKeyword.value || ''
    })
    const data = extractData(response)
    const buckets = normalizeFieldBuckets(data)
    fieldBuckets.value = buckets
    fields.value = flattenFieldBuckets(buckets)
    quickFilters.value = normalizeQuickFilters(data?.quickFilters || data?.quick_filters || [])
    if (hasAnalysisPresetPayload(data)) {
      analysisPresets.value = normalizeAnalysisPresets(data.analysisPresets || data.analysis_presets || data.presets || [])
    }
    if (!fields.value.includes(selectedField.value)) {
      fieldValues.value = []
      topNExpanded.value = false
      selectedField.value = ''
    }
    if (!selectedField.value && fields.value.length) {
      selectedField.value = fields.value[0]
      loadFieldValues()
    }
  } catch (error) {
    console.error('查询日志字段失败:', error)
    fieldBuckets.value = createEmptyFieldBuckets()
    fields.value = []
    quickFilters.value = []
  } finally {
    fieldsLoading.value = false
  }
}

const loadFieldValues = async () => {
  if (!selectedField.value || !queryForm.storeId) return
  fieldValuesLoading.value = true
  try {
    const response = await logtailApi.queryFieldValues({
      ...buildQueryPayload(),
      field: selectedField.value,
      filter: '',
      limit: 5
    })
    fieldValues.value = normalizeFieldValues(extractData(response))
  } catch (error) {
    console.error('查询字段值失败:', error)
    fieldValues.value = []
  } finally {
    fieldValuesLoading.value = false
  }
}

const refreshAll = async () => {
  hasSearched.value = true
  logsTotal.value = null
  hitsTotal.value = null
  if (trendBucketTotal.value === null) total.value = 0
  await Promise.all([loadLogs(), loadHits(), loadFields()])
  if (!trendPoints.value.length && logs.value.length) {
    renderTrend(buildTrendFromLogs())
  }
}

const refreshCurrentRange = () => {
  const item = quickRanges.find(range => range.value === activeRange.value)
  if (item) {
    queryForm.timeRange = defaultTimeRange(item.minutes)
  }
  saveLogQueryPreferences()
  refreshAll()
}

const handleSearch = () => {
  saveLogQueryPreferences()
  trendBucketTotal.value = null
  pagination.page = 1
  refreshAll()
}

const handleProjectChange = async () => {
  queryForm.storeId = ''
  await loadStores()
  saveLogQueryPreferences()
  if (queryForm.storeId) {
    pagination.page = 1
    trendBucketTotal.value = null
    refreshCurrentRange()
    return
  }
  clearQueryResults()
}

const handleScopeChange = () => {
  saveLogQueryPreferences()
  if (!queryForm.storeId) {
    clearQueryResults()
    return
  }
  pagination.page = 1
  trendBucketTotal.value = null
  refreshCurrentRange()
}

const setQuickRange = value => {
  const item = quickRanges.find(range => range.value === value)
  if (!item) return
  trendBucketTotal.value = null
  activeRange.value = value
  queryForm.timeRange = defaultTimeRange(item.minutes)
  saveLogQueryPreferences()
  if (hasSearched.value) {
    handleSearch()
  }
}

const handleTimeRangeChange = () => {
  activeRange.value = ''
  handleSearch()
}

const clearQueryResults = () => {
  hasSearched.value = false
  logs.value = []
  logFieldOrder.value = []
  resetResponseMetadata()
  fieldBuckets.value = createEmptyFieldBuckets()
  fields.value = []
  quickFilters.value = []
  fieldValues.value = []
  selectedField.value = ''
  topNExpanded.value = false
  collapsedFieldGroups.value = new Set()
  expandedJsonFields.value = new Set()
  queryForm.excludeFilters = {}
  trendBucketTotal.value = null
  total.value = 0
  logsTotal.value = null
  hitsTotal.value = null
  backendQuery.value = ''
  durationMs.value = 0
  renderTrend([])
}

const handlePageChange = page => {
  pagination.page = page
  loadLogs()
}

const handleSizeChange = size => {
  pagination.pageSize = size
  pagination.page = 1
  queryForm.limit = size
  loadLogs()
}

const isFieldGroupCollapsed = group => Boolean(
  group?.collapsible &&
  collapsedFieldGroups.value.has(group.key)
)

const toggleFieldGroup = group => {
  if (!group?.collapsible) return
  const next = new Set(collapsedFieldGroups.value)
  if (next.has(group.key)) {
    next.delete(group.key)
  } else {
    next.add(group.key)
  }
  collapsedFieldGroups.value = next
}

const selectField = field => {
  if (selectedField.value === field) {
    topNExpanded.value = !topNExpanded.value
    if (topNExpanded.value) loadFieldValues()
    return
  }
  selectedField.value = field
  topNExpanded.value = true
  loadFieldValues()
}

const addFilter = (field, value) => {
  if (!field) return
  queryForm.extraFilters[field] = value
  delete queryForm.excludeFilters[field]
  handleSearch()
}

const excludeFilter = (field, value) => {
  if (!field) return
  queryForm.excludeFilters[field] = value
  delete queryForm.extraFilters[field]
  handleSearch()
}

const removeFilter = item => {
  if (item.mode === 'exclude') {
    delete queryForm.excludeFilters[item.field]
  } else {
    delete queryForm.extraFilters[item.field]
  }
  handleSearch()
}

const clearFilters = () => {
  queryForm.extraFilters = {}
  queryForm.excludeFilters = {}
  handleSearch()
}

const clearQueryConditions = () => {
  queryForm.query = ''
  queryForm.extraFilters = {}
  queryForm.excludeFilters = {}
  handleSearch()
}

const runQuickFilter = filter => {
  if (!filter?.query) {
    ElMessage.warning('该快捷筛选暂无查询语句')
    return
  }

  queryForm.query = filter.query
  pagination.page = 1
  trendBucketTotal.value = null
  logView.value = 'raw'
  refreshAll()
}

const runCommonQueryAction = item => {
  if (item?.kind === 'analysis') {
    runAnalysisPreset(item)
    return
  }
  runQuickFilter(item)
}

const runAnalysisPreset = preset => {
  if (!preset?.query) {
    ElMessage.warning('该预设暂无查询语句')
    return
  }

  queryForm.query = preset.query
  if (preset.limit) {
    const limit = Number(preset.limit)
    if (!Number.isNaN(limit)) {
      queryForm.limit = limit
      pagination.pageSize = limit
    }
  }
  pagination.page = 1
  trendBucketTotal.value = null
  logView.value = 'table'
  refreshAll()
}

function createEmptyFieldBuckets() {
  return {
    indexFields: [],
    normalFields: []
  }
}

function filterFieldList(list = []) {
  const keyword = fieldKeyword.value.trim().toLowerCase()
  if (!keyword) return list
  return list.filter(item => fieldName(item).toLowerCase().includes(keyword))
}

function hasFieldBucketPayload(data) {
  if (!data || Array.isArray(data) || typeof data !== 'object') return false
  return ['indexFields', 'index_fields', 'normalFields', 'normal_fields']
    .some(key => Array.isArray(data[key]))
}

function dedupeFieldBuckets(buckets) {
  const seen = new Set()
  const take = list => normalizeFieldItems(list).filter(field => {
    if (seen.has(field.name)) return false
    seen.add(field.name)
    return true
  })

  return {
    indexFields: take(buckets.indexFields || []),
    normalFields: take(buckets.normalFields || [])
  }
}

function splitLegacyFieldBuckets(list) {
  const normalized = normalizeFieldItems(list)
  return dedupeFieldBuckets({
    indexFields: normalized.filter(field => displayFieldSet.has(field.name) || !field.name.startsWith('_')),
    normalFields: normalized.filter(field => !displayFieldSet.has(field.name) && field.name.startsWith('_'))
  })
}

function normalizeFieldBuckets(data) {
  if (!hasFieldBucketPayload(data)) {
    return splitLegacyFieldBuckets(normalizeFields(data))
  }

  const indexSource = Array.isArray(data.indexFields)
    ? data.indexFields
    : Array.isArray(data.index_fields)
      ? data.index_fields
      : data.values
  const normalSource = Array.isArray(data.normalFields)
    ? data.normalFields
    : Array.isArray(data.normal_fields)
      ? data.normal_fields
      : []

  return dedupeFieldBuckets({
    indexFields: indexSource || [],
    normalFields: normalSource || []
  })
}

function flattenFieldBuckets(buckets) {
  return uniqueFieldList([
    ...(buckets.indexFields || []),
    ...(buckets.normalFields || [])
  ].map(fieldName))
}

function fieldName(item) {
  if (typeof item === 'string') return item
  return String(item?.name || item?.value || item?.field || item?.key || item?.prop || '').trim()
}

function normalizeNumber(value) {
  if (value === undefined || value === null || value === '') return null
  const numberValue = Number(value)
  return Number.isFinite(numberValue) ? numberValue : null
}

function normalizePercentValue(item) {
  const explicitPercent = normalizeNumber(item?.percent)
  if (explicitPercent !== null) return explicitPercent
  const ratio = normalizeNumber(item?.ratio)
  return ratio === null ? null : ratio * 100
}

function normalizeRatioValue(item, percent) {
  const ratio = normalizeNumber(item?.ratio)
  if (ratio !== null) return ratio
  return percent === null ? null : percent / 100
}

function normalizeFieldItem(item) {
  if (typeof item === 'string') {
    const name = item.trim()
    return name ? { name, value: name, hits: null, total: null, ratio: null, percent: null } : null
  }
  if (!item || typeof item !== 'object') return null

  const name = fieldName(item)
  if (!name) return null

  const percent = normalizePercentValue(item)
  return {
    name,
    value: name,
    hits: normalizeNumber(item.hits ?? item.count ?? item.valueCount),
    total: normalizeNumber(item.total),
    ratio: normalizeRatioValue(item, percent),
    percent
  }
}

function normalizeFieldItems(data) {
  const list = Array.isArray(data) ? data : data?.values || data?.fields || data?.list || data?.items || []
  return list.map(normalizeFieldItem).filter(Boolean)
}

function normalizeFields(data) {
  const list = Array.isArray(data) ? data : data?.values || data?.fields || data?.list || data?.items || []
  return list.map(item => {
    if (typeof item === 'string') return item
    if (!item || typeof item !== 'object') return ''
    return item.value || item.name || item.field || item.key || item.prop || ''
  }).filter(Boolean)
}

function normalizeFieldValues(data) {
  const list = Array.isArray(data) ? data : data?.values || data?.list || data?.items || []
  const items = list.map(item => {
    if (typeof item === 'string') return { value: item, count: '', hits: null, total: null, ratio: null, percent: null }
    if (!item || typeof item !== 'object') return null
    const value = item.value ?? item.name ?? item.fieldValue ?? item.key ?? ''
    const count = normalizeNumber(item.count ?? item.hits ?? item.valueCount)
    const percent = normalizePercentValue(item)
    return {
      value,
      count: count ?? '',
      hits: count,
      total: normalizeNumber(item.total),
      ratio: normalizeRatioValue(item, percent),
      percent
    }
  }).filter(item => item && item.value !== '')

  return appendOtherFieldValue(items)
}

function appendOtherFieldValue(items) {
  if (!items.length) return items
  const total = items.find(item => item.total !== null)?.total
  if (!total || total <= 0) return items

  const knownHits = items.reduce((sum, item) => sum + (item.hits || 0), 0)
  const otherHits = Math.max(0, total - knownHits)
  if (!otherHits) return items

  const ratio = otherHits / total
  return [
    ...items,
    {
      value: '其他',
      count: otherHits,
      hits: otherHits,
      total,
      ratio,
      percent: ratio * 100,
      synthetic: true
    }
  ]
}

function formatPercent(value) {
  const percent = normalizeNumber(value)
  if (percent === null) return ''
  if (percent === 0) return '0%'
  if (percent >= 99.995) return '100%'
  if (percent < 1) return `${percent.toFixed(2)}%`
  if (percent < 10) return `${percent.toFixed(1)}%`
  return `${Math.round(percent)}%`
}

function fieldPercentText(item) {
  return formatPercent(item?.percent)
}

function fieldTitle(item) {
  const parts = [fieldName(item)]
  if (item?.hits !== null && item?.hits !== undefined) parts.push(`命中 ${item.hits}`)
  if (item?.total !== null && item?.total !== undefined) parts.push(`总数 ${item.total}`)
  const percent = fieldPercentText(item)
  if (percent) parts.push(`占比 ${percent}`)
  return parts.filter(Boolean).join(' / ')
}

function uniqueFieldList(list) {
  return list
    .map(item => String(item || '').trim())
    .filter((item, index, values) => item && values.indexOf(item) === index)
}

function normalizeDisplayMode(value) {
  const normalized = String(value || '').trim().toLowerCase()
  return displayModeSet.has(normalized) ? normalized : ''
}

function resolveDisplayMode(data) {
  const explicitMode = normalizeDisplayMode(data?.displayMode || data?.display_mode)
  if (explicitMode) return explicitMode
  const columns = data?.tableColumns || data?.table_columns || data?.columns
  return normalizeFields(columns).length ? 'structured' : 'raw'
}

function cloneTableColumn(column) {
  return {
    ...column,
    keys: [...(column.keys || [column.key])]
  }
}

function normalizeWidth(value) {
  if (value === undefined || value === null || value === '') return undefined
  const numberValue = Number(value)
  return Number.isNaN(numberValue) ? value : numberValue
}

function columnLabel(key, item = {}) {
  return item.label || item.title || item.nameLabel || columnLabelMap[key] || key
}

function isNumericColumn(key, type) {
  if (timestampColumns.has(key)) return false
  if (String(type || '').toLowerCase() === 'number') return true
  return numericColumnPattern.test(key)
}

function defaultColumnMinWidth(key) {
  if (timestampColumns.has(key)) return 190
  if (['status', 'count', 'total', 'requests', 'request_method', 'level', 'severity_text'].includes(key)) return 100
  if (wideColumnPattern.test(key)) return /sql|message|_raw|user_agent/i.test(key) ? 420 : 260
  return 150
}

function normalizeTableColumn(item, index, mode = displayMode.value) {
  const rawItem = typeof item === 'string' ? { key: item } : item || {}
  const key = String(rawItem.key || rawItem.field || rawItem.prop || rawItem.value || rawItem.name || '').trim()
  if (!key) return null

  const keys = Array.isArray(rawItem.keys) && rawItem.keys.length
    ? rawItem.keys
    : Array.isArray(rawItem.aliases) && rawItem.aliases.length
      ? rawItem.aliases
      : key === 'message'
        ? rawMessageKeys
        : [key]
  const numeric = isNumericColumn(key, rawItem.type)
  const isRawTextColumn = rawMessageKeys.includes(key)

  return {
    key,
    keys: uniqueFieldList([key, ...keys]),
    label: columnLabel(key, rawItem),
    width: normalizeWidth(rawItem.width),
    minWidth: normalizeWidth(rawItem.minWidth || rawItem.min_width) || defaultColumnMinWidth(key),
    align: rawItem.align || (numeric ? 'right' : 'left'),
    mono: rawItem.mono ?? monoColumnPattern.test(key),
    highlight: rawItem.highlight ?? (isRawTextColumn || ['sql_text', 'request_uri', 'request_path'].includes(key)),
    overflow: rawItem.overflow ?? rawItem.showOverflowTooltip ?? rawItem.show_overflow_tooltip ?? true,
    order: rawItem.order ?? index,
    mode
  }
}

function normalizeTableColumns(data, mode) {
  const list = Array.isArray(data) ? data : data?.columns || data?.fields || data?.list || data?.items || []
  const columns = list
    .map((item, index) => normalizeTableColumn(item, index, mode))
    .filter(Boolean)
    .sort((left, right) => Number(left.order || 0) - Number(right.order || 0))

  return columns.filter((column, index) => columns.findIndex(item => item.key === column.key) === index)
}

function normalizeAnalysisPresets(data) {
  const list = Array.isArray(data)
    ? data
    : data && typeof data === 'object'
      ? Object.entries(data).map(([label, value]) => (typeof value === 'string' ? { label, query: value } : { label, ...value }))
      : []

  return list
    .map((item, index) => {
      if (typeof item === 'string') return null
      const label = item.label || item.name || item.title || item.text || ''
      const query = item.query || item.expr || item.expression || item.statement || item.q || ''
      return label && query
        ? {
            key: item.key || `${label}-${index}`,
            label,
            query,
            limit: item.limit,
            description: item.description || '',
            resultType: item.resultType || item.result_type || '',
            columns: item.columns || []
          }
        : null
    })
    .filter(Boolean)
}

function normalizeQuickFilters(data) {
  const list = Array.isArray(data)
    ? data
    : data && typeof data === 'object'
      ? Object.entries(data).map(([label, value]) => (typeof value === 'string' ? { label, query: value } : { label, ...value }))
      : []

  return list
    .map((item, index) => {
      if (typeof item === 'string') return null
      const label = item.label || item.name || item.title || item.text || ''
      const query = item.query || item.expr || item.expression || item.statement || item.q || ''
      return label && query
        ? {
            key: item.key || `${label}-${index}`,
            label,
            query,
            description: item.description || '',
            logType: item.logType || item.log_type || ''
          }
        : null
    })
    .filter(Boolean)
}

function hasAnalysisPresetPayload(data) {
  if (!data || Array.isArray(data) || typeof data !== 'object') return false
  return ['analysisPresets', 'analysis_presets', 'presets'].some(key => Array.isArray(data[key]) || (data[key] && typeof data[key] === 'object'))
}

function syncResponseMetadata(data) {
  const nextDisplayMode = resolveDisplayMode(data)
  const nextColumns = normalizeTableColumns(data?.tableColumns || data?.table_columns || data?.columns || [], nextDisplayMode)
  const responseFields = normalizeFields(data?.fields || [])
  const columnFields = nextColumns.flatMap(column => [column.key, ...(column.keys || [])])

  displayMode.value = nextDisplayMode
  responseLogType.value = String(data?.logType || data?.log_type || '').trim()
  tableColumns.value = nextColumns
  if (hasAnalysisPresetPayload(data)) {
    analysisPresets.value = normalizeAnalysisPresets(data?.analysisPresets || data?.analysis_presets || data?.presets || [])
  }
  logFieldOrder.value = uniqueFieldList([...columnFields, ...responseFields])

  if (!['raw', 'json', 'table'].includes(logView.value)) logView.value = 'raw'
}

function resetResponseMetadata() {
  displayMode.value = 'raw'
  responseLogType.value = ''
  tableColumns.value = []
  analysisPresets.value = []
  logFieldOrder.value = []
  if (logView.value !== 'json') logView.value = 'raw'
}

function visibleTableKey(key) {
  return key && !String(key).startsWith('__') && key !== '_rowKey'
}

function buildColumnsFromRows(rows) {
  const rowKeys = rows.flatMap(row => Object.keys(row.__raw || row || {}))
  return uniqueFieldList([...logFieldOrder.value, ...rowKeys])
    .filter(visibleTableKey)
    .map((key, index) => normalizeTableColumn(key, index, displayMode.value))
    .filter(Boolean)
}

function rawRow(row) {
  return row?.__raw || row || {}
}

function readRowValue(row, key) {
  const raw = rawRow(row)
  if (Object.prototype.hasOwnProperty.call(raw, key)) return raw[key]
  if (!String(key).includes('.')) return undefined

  return String(key).split('.').reduce((value, part) => {
    if (value && typeof value === 'object' && Object.prototype.hasOwnProperty.call(value, part)) return value[part]
    return undefined
  }, raw)
}

function isPresentValue(value) {
  return value !== undefined && value !== null && value !== ''
}

function tableCellValue(row, column) {
  const keys = Array.isArray(column.keys) && column.keys.length ? column.keys : [column.key]
  for (const key of keys) {
    const value = readRowValue(row, key)
    if (isPresentValue(value)) return value
  }
  return ''
}

function formatTableCellValue(value) {
  if (!isPresentValue(value)) return '-'
  if (typeof value !== 'object') return String(value)
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

const tableCellText = (row, column) => formatTableCellValue(tableCellValue(row, column))

const tableCellClass = column => ({
  'mono-text': column.mono,
  'highlight-line': column.highlight,
  'numeric-cell': column.align === 'right'
})

function extractTrendList(data) {
  if (Array.isArray(data)) return data
  if (Array.isArray(data?.hits) && data.hits[0]?.timestamps && data.hits[0]?.values) {
    return data.hits[0].timestamps.map((time, index) => [time, data.hits[0].values[index]])
  }
  if (Array.isArray(data?.timestamps) && Array.isArray(data?.values)) {
    return data.timestamps.map((time, index) => [time, data.values[index]])
  }
  if (Array.isArray(data?.times) && Array.isArray(data?.counts)) {
    return data.times.map((time, index) => [time, data.counts[index]])
  }
  if (Array.isArray(data?.series)) {
    return data.series.flatMap(item => item.data || item.values || item.points || [])
  }
  if (Array.isArray(data?.result)) {
    return data.result.flatMap(item => item.values || item.data || item.points || [])
  }
  const list = [data?.hits, data?.list, data?.points, data?.values, data?.buckets, data?.histogram].find(Array.isArray)
  if (list) return list
  if (data?.histogram && typeof data.histogram === 'object') {
    return Object.entries(data.histogram)
  }
  return []
}

function trendPointFromItem(item) {
  if (Array.isArray(item)) {
    return {
      time: item[0],
      end: item[2],
      count: Number(item[1] || 0)
    }
  }

  const rawCount = item.count ?? item.value ?? item.logs ?? item.total ?? item.hits ?? item.docCount ?? item.doc_count ?? 0
  return {
    time: item.time || item.timestamp || item.ts || item.t || item.bucket || item.start || item.startTime || item.from,
    end: item.end || item.endTime || item.to,
    count: Number(rawCount || 0)
  }
}

function normalizeTrend(data) {
  const stepMs = parseStepMs(queryForm.step)
  return extractTrendList(data)
    .map(trendPointFromItem)
    .map(item => {
      const startDate = parseTimeValue(item.time)
      if (!startDate) return null
      const endDate = parseTimeValue(item.end) || new Date(startDate.getTime() + stepMs)
      return {
        time: startDate.toISOString(),
        start: startDate,
        end: endDate,
        count: Number(item.count || 0)
      }
    })
    .filter(Boolean)
    .sort((left, right) => left.start.getTime() - right.start.getTime())
}

function extractHitsTotal(data, points) {
  if (points.length) {
    return points.reduce((sum, point) => sum + Number(point.count || 0), 0)
  }
  const explicit = normalizeNumber(
    data?.total ??
    data?.count ??
    data?.hitsTotal ??
    data?.hits_total ??
    data?.logsTotal ??
    data?.logs_total
  )
  if (explicit !== null) return explicit
  return 0
}

function applyQueryTotal() {
  if (trendBucketTotal.value !== null) {
    total.value = Number(trendBucketTotal.value || 0)
    return
  }
  if (hitsTotal.value !== null) {
    total.value = Number(hitsTotal.value || 0)
    return
  }
  total.value = Number(logsTotal.value || 0)
}

function buildTrendFromLogs() {
  const stepMs = parseStepMs(queryForm.step)
  const buckets = new Map()
  logs.value.forEach(row => {
    const date = parseTimeValue(logTime(row))
    if (!date) return
    const bucketStartMs = Math.floor(date.getTime() / stepMs) * stepMs
    buckets.set(bucketStartMs, (buckets.get(bucketStartMs) || 0) + 1)
  })
  return Array.from(buckets.entries())
    .sort(([left], [right]) => left - right)
    .map(([startMs, count]) => ({
      time: new Date(startMs).toISOString(),
      start: new Date(startMs),
      end: new Date(startMs + stepMs),
      count
    }))
}

function trendChartBounds(points) {
  const [rangeStart, rangeEnd] = queryForm.timeRange || []
  const firstPoint = points[0]
  const lastPoint = points[points.length - 1]
  const start = parseTimeValue(rangeStart) || firstPoint?.start || new Date()
  const end = parseTimeValue(rangeEnd) || lastPoint?.end || new Date(start.getTime() + parseStepMs(queryForm.step))
  const startMs = start.getTime()
  const endMs = Math.max(end.getTime(), startMs + 1000)
  return { startMs, endMs }
}

function trendBrushAreas(params) {
  if (Array.isArray(params?.areas)) return params.areas
  if (!Array.isArray(params?.batch)) return []
  return params.batch.flatMap(item => item?.areas || [])
}

function trendBrushRange(params) {
  const area = trendBrushAreas(params).find(item => item?.brushType === 'lineX' && Array.isArray(item.coordRange))
  if (!area) return null
  const range = area.coordRange.map(value => {
    if (typeof value === 'number' && Number.isFinite(value)) return value
    return parseTimeValue(value)?.getTime()
  })
  return range.length === 2 && range.every(Number.isFinite) ? range : null
}

function activateTrendBrush() {
  if (!trendChart) return
  trendChart.dispatchAction({
    type: 'takeGlobalCursor',
    key: 'brush',
    brushOption: {
      brushType: 'lineX',
      brushMode: 'single'
    }
  })
}

function handleTrendBrushEnd(params) {
  const range = trendBrushRange(params)
  if (!range) {
    activateTrendBrush()
    return
  }

  const { startMs: chartStartMs, endMs: chartEndMs } = trendChartBounds(trendPoints.value)
  let startMs = Math.max(chartStartMs, Math.min(...range))
  let endMs = Math.min(chartEndMs, Math.max(...range))
  startMs = Math.floor(startMs / 1000) * 1000
  endMs = Math.ceil(endMs / 1000) * 1000
  if (endMs <= startMs) endMs = Math.min(chartEndMs, startMs + 1000)
  if (endMs <= startMs) {
    activateTrendBrush()
    return
  }

  const start = new Date(startMs)
  const end = new Date(endMs)
  queryForm.timeRange = [formatDateTime(start), formatDateTime(end)]
  trendSelectionAnnouncement.value = `已选择 ${formatDateTime(start)} 至 ${formatDateTime(end)}`
  suppressTrendClickUntil = Date.now() + 300
  activeRange.value = ''
  pagination.page = 1
  handleSearch()
}

function renderTrend(points) {
  trendPoints.value = points
  nextTick(() => {
    if (!trendChartRef.value) return
    if (!trendChart) {
      trendChart = echarts.init(trendChartRef.value)
    }
    trendChart.off('click')
    trendChart.off('brushEnd')
    trendChart.on('click', params => {
      if (Date.now() < suppressTrendClickUntil) return
      const point = trendPoints.value[params.dataIndex]
      if (point) jumpToTrendBucket(point)
    })
    trendChart.on('brushEnd', handleTrendBrushEnd)
    const { startMs, endMs } = trendChartBounds(points)
    trendChart.setOption({
      toolbox: { show: false },
      grid: { left: 36, right: 14, top: 10, bottom: 24 },
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'line',
          snap: false,
          lineStyle: { color: '#4c8bf5', width: 1 },
          label: {
            show: true,
            backgroundColor: '#4b617f',
            formatter: params => formatDateTime(new Date(params.value))
          }
        },
        formatter: params => {
          const point = trendPoints.value[params?.[0]?.dataIndex]
          if (!point) return ''
          return [
            `${formatDateTime(point.start)} ~ ${formatDateTime(point.end)}`,
            `日志数：${point.count}`
          ].join('<br/>')
        }
      },
      xAxis: {
        type: 'time',
        min: startMs,
        max: endMs,
        axisTick: { show: false },
        axisLine: { lineStyle: { color: '#dfe7f2' } },
        axisLabel: {
          color: '#8d97a8',
          formatter: value => formatAxisTime(value)
        }
      },
      yAxis: {
        type: 'value',
        splitLine: { lineStyle: { color: 'rgba(223, 231, 242, 0.74)' } },
        axisLabel: { color: '#8d97a8' }
      },
      brush: {
        xAxisIndex: 0,
        brushType: 'lineX',
        brushMode: 'single',
        transformable: false,
        removeOnClick: true,
        throttleType: 'debounce',
        throttleDelay: 120,
        brushStyle: {
          borderWidth: 1,
          color: 'rgba(76, 139, 245, 0.16)',
          borderColor: '#4c8bf5'
        },
        inBrush: { color: '#85c763' },
        outOfBrush: { color: '#85c763' }
      },
      series: [
        {
          type: 'bar',
          data: points.map(item => [item.start.getTime(), item.count]),
          barMaxWidth: 18,
          barMinHeight: 2,
          itemStyle: {
            color: '#85c763',
            borderRadius: [6, 6, 0, 0]
          }
        }
      ]
    })
    trendChart.dispatchAction({ type: 'brush', command: 'clear', areas: [] })
    activateTrendBrush()
  })
}

const jumpToTrendBucket = point => {
  queryForm.timeRange = [formatDateTime(point.start), formatDateTime(point.end)]
  trendBucketTotal.value = Number(point.count || 0)
  hitsTotal.value = null
  logsTotal.value = null
  applyQueryTotal()
  pagination.page = 1
  activeRange.value = ''
  saveLogQueryPreferences()
  refreshAll()
}

function formatAxisTime(value) {
  const date = parseTimeValue(value)
  if (!date) return String(value || '-')
  return date.toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

const compactLogTime = row => {
  const date = parseTimeValue(logTime(row))
  if (!date) return logTime(row)
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  const time = date.toLocaleTimeString('zh-CN', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
  return `${month}-${day} ${time}`
}

const pick = (row, keys) => {
  for (const key of keys) {
    if (row?.[key] !== undefined && row?.[key] !== null && row?.[key] !== '') return row[key]
  }
  return '-'
}

const logTime = row => pick(row, ['_time', '@timestamp', 'timestamp', 'time'])
const logLevel = row => String(pick(row, ['severity_text', 'level', 'severity', 'log_level'])).toUpperCase()
const logMessage = row => formatTableCellValue(pick(row, ['message', '_raw', '_msg', 'msg', 'log', 'content', 'raw']))
const escapeRegExp = value => String(value).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

const highlightText = value => {
  const text = value === undefined || value === null ? '' : String(value)
  const regex = highlightRegex.value
  if (!text || !regex) return [{ key: 'text-0', text, hit: false }]

  regex.lastIndex = 0
  const parts = []
  let lastIndex = 0
  let index = 0
  let match

  while ((match = regex.exec(text)) !== null) {
    if (match.index > lastIndex) {
      parts.push({
        key: `text-${index++}`,
        text: text.slice(lastIndex, match.index),
        hit: false
      })
    }

    parts.push({
      key: `hit-${index++}`,
      text: match[0],
      hit: true
    })

    lastIndex = match.index + match[0].length
    if (regex.lastIndex === match.index) regex.lastIndex += 1
  }

  if (lastIndex < text.length) {
    parts.push({
      key: `text-${index++}`,
      text: text.slice(lastIndex),
      hit: false
    })
  }

  return parts
}

const coreLogFieldDefs = [
  { key: 'app', label: 'app', keys: ['app'] },
  { key: 'host_name', label: 'host', keys: ['host_name'] },
  { key: 'file_path', label: 'path', keys: ['file_path'] },
  { key: 'env', label: 'env', keys: ['env'] },
  { key: 'log_source', label: 'source', keys: ['log_source'] },
  { key: 'collect_task_code', label: 'task', keys: ['collect_task_code'] }
]

const coreLogFields = row => coreLogFieldDefs.map(item => ({
  key: item.key,
  label: item.label,
  value: pick(row, item.keys)
}))

const rowPrimaryFilter = row => {
  const raw = row.__raw || row
  const field = coreLogFieldDefs.find(item => {
    const value = pick(raw, item.keys)
    return value !== '-'
  })
  if (!field) return null
  return {
    field: field.key,
    value: pick(raw, field.keys)
  }
}

const logRowNumber = index => (pagination.page - 1) * pagination.pageSize + index + 1

const formatRawFieldValue = value => {
  if (value === undefined || value === null || value === '') return '-'
  const text = typeof value === 'object' ? JSON.stringify(value) : String(value)
  return text.length > 180 ? `${text.slice(0, 180)}...` : text
}

const fieldTypeClass = field => {
  if (field.startsWith('_')) return 'system'
  if (/time|date|timestamp/i.test(field)) return 'date'
  if (/count|num|size|bytes|duration|id$/i.test(field)) return 'number'
  return 'text'
}

const detailFieldRows = row => {
  if (!row) return []
  const raw = row.__raw || row
  const orderedKeys = [
    ...logFieldOrder.value,
    ...Object.keys(raw)
  ].filter((key, index, list) => key && !key.startsWith('__') && list.indexOf(key) === index)

  return orderedKeys
    .filter(key => raw[key] !== undefined)
    .map(key => ({
      key,
      value: formatRawFieldValue(raw[key])
    }))
}

const formatJsonFieldValue = value => {
  if (value === undefined || value === null || value === '') return '-'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

const jsonFieldRows = row => {
  if (!row) return []
  const raw = row.__raw || row
  const orderedKeys = [
    ...logFieldOrder.value,
    ...Object.keys(raw)
  ].filter((key, index, list) => key && !key.startsWith('__') && list.indexOf(key) === index)

  return orderedKeys
    .filter(key => raw[key] !== undefined)
    .map(key => ({
      key,
      value: formatJsonFieldValue(raw[key]),
      collapsible: formatJsonFieldValue(raw[key]).length > 96,
      isJson: raw[key] !== null && typeof raw[key] === 'object'
    }))
}

const jsonFieldExpandKey = (row, field) => `${row?._rowKey || logTime(row) || 'row'}::${field}`

const isJsonFieldExpanded = (row, field) => expandedJsonFields.value.has(jsonFieldExpandKey(row, field))

const toggleJsonField = (row, field) => {
  const key = jsonFieldExpandKey(row, field)
  const next = new Set(expandedJsonFields.value)
  if (next.has(key)) {
    next.delete(key)
  } else {
    next.add(key)
  }
  expandedJsonFields.value = next
}

const levelTagType = level => {
  const normalized = String(level || '').toLowerCase()
  if (['error', 'fatal', 'critical', 'panic'].includes(normalized)) return 'danger'
  if (['warn', 'warning'].includes(normalized)) return 'warning'
  if (['info', 'notice'].includes(normalized)) return 'success'
  return 'info'
}

const stringify = value => JSON.stringify(value, null, 2)

const AI_ANALYSIS_MAX_LINES = 100
const aiAnalysisFieldKeys = [
  'env',
  'app',
  'service',
  'host',
  'host_name',
  'file_path',
  'path',
  'log_source',
  'k8s_namespace',
  'k8s_pod_name',
  'k8s_container_name',
  'status',
  'error_code',
  'error',
  'error_msg',
  'stack_trace',
  'request_id',
  'trace_id',
  'x_zai_trace_id'
]

const buildAiAnalysisEntry = (row, index) => {
  const raw = rawRow(row)
  const timestamp = String(logTime(raw) || '')
  const parsedTime = parseTimeValue(timestamp)
  const record = {}
  aiAnalysisFieldKeys.forEach(key => {
    if (isPresentValue(raw[key])) record[key] = raw[key]
  })
  record.message = logMessage(raw)

  return {
    sortTime: parsedTime?.getTime(),
    entry: {
      lineNo: logRowNumber(index),
      timestamp,
      timestampUnix: parsedTime ? Math.floor(parsedTime.getTime() / 1000) : 0,
      level: logLevel(raw),
      message: record.message,
      raw: JSON.stringify(record)
    }
  }
}

const buildLatestAiAnalysisEntries = () => {
  const candidates = logs.value.map(buildAiAnalysisEntry)
  const allHaveTime = candidates.every(item => Number.isFinite(item.sortTime))
  const ordered = allHaveTime
    ? candidates.sort((left, right) => left.sortTime - right.sortTime)
    : candidates.slice(0, AI_ANALYSIS_MAX_LINES).reverse()

  return ordered
    .slice(-AI_ANALYSIS_MAX_LINES)
    .map(item => item.entry)
}

const runAiAnalysis = async () => {
  const entries = buildLatestAiAnalysisEntries()
  if (!entries.length) {
    ElMessage.warning('请先查询日志')
    return
  }

  aiAnalysis.visible = true
  aiAnalysis.loading = true
  aiAnalysis.error = ''
  aiAnalysis.analysis = ''
  aiAnalysis.providerName = ''
  aiAnalysis.generatedAt = ''
  aiAnalysis.requestedLogLines = entries.length
  aiAnalysis.usedLogLines = 0

  const levelCounts = entries.reduce((counts, entry) => {
    const level = entry.level || 'UNKNOWN'
    counts[level] = (counts[level] || 0) + 1
    return counts
  }, {})

  try {
    const response = await aiopsApi.analyzeLogs({
      prompt: `当前日志查询语句：${buildQueryText()}。请结合查询条件分析异常，不要把查询语句本身当作日志证据。`,
      entries,
      summary: {
        total: Number(total.value || entries.length),
        matched: entries.length,
        levelCounts
      }
    })
    const result = extractData(response) || {}
    const analysis = String(result.analysis || result.result || result.message || '').trim()
    if (!analysis) throw new Error('AI 未返回分析内容')

    aiAnalysis.analysis = analysis
    aiAnalysis.providerName = result.providerName || result.modelCode || ''
    aiAnalysis.generatedAt = result.generatedAt || ''
    aiAnalysis.usedLogLines = Number(result.usedLogLines || entries.length)
  } catch (error) {
    console.error('AI 分析日志失败:', error)
    aiAnalysis.error = error.message || 'AI 分析失败，请稍后重试'
  } finally {
    aiAnalysis.loading = false
  }
}

const openLogDetail = row => {
  detailDrawer.row = row.__raw || row
  detailDrawer.activeTab = 'fields'
  detailDrawer.visible = true
}

const fallbackCopyText = text => {
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.top = '-9999px'
  textarea.style.left = '-9999px'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.focus()
  textarea.select()
  textarea.setSelectionRange(0, textarea.value.length)

  try {
    return document.execCommand('copy')
  } finally {
    document.body.removeChild(textarea)
  }
}

const copyText = async text => {
  const content = text === undefined || text === null ? '' : String(text)
  if (!content) {
    ElMessage.warning('暂无可复制内容')
    return
  }

  try {
    if (!navigator.clipboard || !window.isSecureContext) {
      throw new Error('Clipboard API unavailable')
    }
    await navigator.clipboard.writeText(content)
    ElMessage.success('已复制')
  } catch (error) {
    if (fallbackCopyText(content)) {
      ElMessage.success('已复制')
      return
    }

    console.error('复制失败:', error)
    ElMessage.warning('复制失败，请手动复制')
  }
}

const copyLog = row => {
  copyText(logMessage(row))
}

const addRowFilter = row => {
  const filter = rowPrimaryFilter(row)
  if (!filter) {
    ElMessage.warning('当前日志没有可用核心字段')
    return
  }
  addFilter(filter.field, filter.value)
}

const excludeRowFilter = row => {
  const filter = rowPrimaryFilter(row)
  if (!filter) {
    ElMessage.warning('当前日志没有可用核心字段')
    return
  }
  excludeFilter(filter.field, filter.value)
}

const createAlertFromRow = row => {
  const filter = rowPrimaryFilter(row)
  ElMessage.info(filter ? `可基于 ${filter.field}=${filter.value} 创建告警` : '请先选择可用于告警的字段')
}

const handleResize = () => trendChart?.resize()

watch(fieldKeyword, () => {
  if (fields.value.length) return
  loadFields()
})

onMounted(async () => {
  window.addEventListener('resize', handleResize)
  const preferences = readLogQueryPreferences()
  await loadProjects(preferences)
  restoreTimePreference(preferences)
  saveLogQueryPreferences()
  await refreshAll()
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  trendChart?.dispose()
  trendChart = null
})
</script>

<style scoped>
:global(.home-container .el-main:has(.log-query-page)) {
  padding-top: var(--ops-shell-gutter, 10px);
  padding-right: var(--ops-shell-gutter, 10px);
  padding-left: var(--ops-shell-gutter, 10px);
  overflow: hidden;
}

.log-query-page {
  --log-page-height-offset: 124px;
  --log-surface-warm: rgba(255, 253, 248, 0.96);
  --log-surface-cool: rgba(247, 250, 255, 0.92);
  --log-text-primary: #172133;
  --log-text-secondary: #5f6b7c;
  --log-text-muted: #8d97a8;
  --log-border: rgba(214, 223, 235, 0.96);
  --log-brand: #4c8bf5;
  --log-topbar: linear-gradient(90deg, #fef7ef 0%, #f8f8f6 52%, #edf2fd 100%);
  min-height: calc(100vh - var(--log-page-height-offset));
  padding: var(--ops-page-padding, 7px);
  box-sizing: border-box;
  background: transparent;
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.page-card {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: calc(100vh - var(--log-page-height-offset) - 14px);
  max-height: calc(100vh - var(--log-page-height-offset) - 14px);
  padding: 16px 12px 12px;
  overflow: hidden;
  border: 1px solid var(--log-border);
  border-radius: var(--ops-card-radius, 12px);
  background: linear-gradient(135deg, var(--log-surface-warm), var(--log-surface-cool));
  box-shadow: 0 18px 48px rgba(17, 24, 39, 0.08);
}

.page-card::before {
  content: "";
  position: absolute;
  inset: 0 0 auto;
  height: 16px;
  background: var(--log-topbar);
}

.page-header {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin: -16px -12px 6px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--log-border);
  background: rgba(255, 255, 255, 0.96);
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
  background: var(--log-border);
}

.page-header h1 {
  margin: 0;
  flex: 0 0 auto;
  color: var(--log-text-primary);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
}

.page-description {
  margin: 0;
  overflow: hidden;
  color: var(--log-text-muted);
  font-size: 12px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.summary-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: nowrap;
  justify-content: flex-end;
}

.summary-chip {
  display: inline-flex;
  align-items: baseline;
  gap: 6px;
  min-width: 0;
  padding: 6px 9px;
  border: 1px solid var(--log-border);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.88);
  white-space: nowrap;
}

.summary-chip span,
.muted-text {
  color: var(--log-text-muted);
  font-size: 11px;
}

.summary-chip strong {
  display: inline;
  color: var(--log-text-primary);
  font-size: 15px;
  line-height: 1.1;
}

.query-stack {
  display: grid;
  grid-template-rows: minmax(276px, 0.95fr) minmax(0, 2.05fr);
  min-height: 0;
  flex: 1;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.86);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.9);
}

.query-insight-panel,
.log-result-panel {
  background: transparent;
}

.query-insight-panel,
.log-result-panel,
.parse-panel,
.detail-panel {
  min-height: 0;
}

.query-insight-panel {
  display: grid;
  grid-template-rows: auto auto minmax(132px, 1fr);
  overflow: hidden;
  border-bottom: 1px solid rgba(223, 231, 242, 0.9);
}

.log-result-panel {
  display: grid;
  grid-template-columns: 208px minmax(0, 1fr);
  overflow: hidden;
}

.parse-panel,
.detail-panel {
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.parse-panel {
  padding: 7px 8px;
  border-right: 1px solid rgba(223, 231, 242, 0.86);
}

.detail-panel {
  padding: 8px 10px;
}

.scope-toolbar {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  padding: 5px 10px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.86);
}

.panel-head {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  align-items: center;
  margin-bottom: 5px;
}

.panel-head h3 {
  margin: 0;
  color: var(--log-text-primary);
  font-size: 14px;
}

.panel-head p {
  max-width: 720px;
  margin: 3px 0 0;
  color: var(--log-text-muted);
  font-size: 10px;
  letter-spacing: 0.08em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.scope-form {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.scope-form :deep(.el-form-item) {
  margin-right: 0;
  margin-bottom: 0;
}

.scope-form :deep(.el-select),
.scope-form :deep(.el-input) {
  width: 200px;
}

.scope-form :deep(.el-form-item__label),
.query-form :deep(.el-form-item__label) {
  margin-bottom: 2px;
  color: var(--log-text-secondary);
  font-size: 12px;
  font-weight: 600;
}

.field-section {
  min-height: 0;
  padding-top: 7px;
  overflow: hidden;
}

.field-section:first-of-type {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 5px;
  color: var(--log-text-primary);
  font-size: 11px;
  font-weight: 700;
}

.field-list {
  flex: 1 1 auto;
  min-height: 0;
  max-height: 100%;
  overflow-x: hidden;
  overflow-y: auto;
  overscroll-behavior: contain;
  padding-top: 3px;
  padding-right: 4px;
  scrollbar-gutter: stable;
  scrollbar-width: thin;
  scrollbar-color: rgba(148, 163, 184, 0.75) transparent;
}

.field-list::-webkit-scrollbar,
.inline-topn-list::-webkit-scrollbar {
  width: 6px;
}

.field-list::-webkit-scrollbar-thumb,
.inline-topn-list::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.62);
}

.field-list::-webkit-scrollbar-track,
.inline-topn-list::-webkit-scrollbar-track {
  background: transparent;
}

.field-group + .field-group {
  margin-top: 7px;
}

.field-group-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 1px 0 3px;
  color: #64748b;
  font-size: 10px;
  font-weight: 700;
}

.field-group-toggle {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 20px;
  padding: 2px 3px;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: inherit;
  cursor: pointer;
  font: inherit;
}

.field-group-toggle:hover {
  color: var(--log-brand);
  background: rgba(76, 139, 245, 0.06);
}

.field-group-label {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  cursor: pointer;
}

.field-group-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.field-group-caret {
  flex: 0 0 auto;
  width: 0;
  height: 0;
  border-left: 4px solid transparent;
  border-right: 4px solid transparent;
  border-top: 5px solid #94a3b8;
  transform: translateY(1px);
  transition: transform 0.16s ease, border-top-color 0.16s ease;
}

.field-group-caret.collapsed {
  transform: rotate(-90deg) translateX(1px);
}

.field-group-toggle:hover .field-group-caret {
  border-top-color: var(--log-brand);
}

.field-group-title em {
  color: #94a3b8;
  font-style: normal;
  font-weight: 600;
}

.field-item,
.quick-ranges button,
.common-query-actions button {
  border: 1px solid transparent;
  background: transparent;
  color: var(--log-text-secondary);
  cursor: pointer;
  transition: color 0.2s ease, background 0.2s ease;
}

.field-item {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 6px;
  margin-bottom: 1px;
  padding: 4px 6px;
  border-radius: 5px;
  text-align: left;
  font-size: 11px;
}

.field-item:hover,
.field-item.active {
  color: var(--log-brand);
  background: rgba(76, 139, 245, 0.07);
}

.field-type-dot {
  flex: 0 0 auto;
  width: 7px;
  height: 7px;
  border-radius: 2px;
  background: #3b82f6;
}

.field-type-dot.system {
  background: #94a3b8;
}

.field-type-dot.date {
  background: #22c55e;
}

.field-type-dot.number {
  background: #f59e0b;
}

.field-type-dot.text {
  background: #60a5fa;
}

.field-item .mono-text,
.inline-topn-item span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.field-percent {
  flex: 0 0 auto;
  margin-left: auto;
  color: #94a3b8;
  font-style: normal;
  font-size: 10px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.inline-topn-list {
  max-height: 128px;
  margin: 2px 0 6px 14px;
  padding: 4px 0 4px 7px;
  overflow-x: hidden;
  overflow-y: auto;
  overscroll-behavior: contain;
  border-left: 1px solid rgba(148, 163, 184, 0.5);
  scrollbar-width: thin;
  scrollbar-color: rgba(148, 163, 184, 0.75) transparent;
}

.inline-topn-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto 16px 16px;
  align-items: center;
  gap: 5px;
  padding: 2px 3px;
  border-radius: 4px;
  color: #64748b;
  font-size: 10px;
  line-height: 1.4;
}

.inline-topn-item:hover {
  background: rgba(76, 139, 245, 0.06);
}

.inline-topn-item strong {
  margin-left: auto;
  color: var(--log-brand);
  font-weight: 700;
}

.inline-topn-item .field-percent {
  margin-left: 0;
  text-align: right;
}

.inline-topn-item button {
  width: 15px;
  height: 15px;
  padding: 0;
  border: 1px solid transparent;
  border-radius: 4px;
  background: rgba(239, 246, 255, 0.9);
  color: #2563eb;
  cursor: pointer;
  font-size: 10px;
  line-height: 12px;
}

.inline-topn-item button:last-child {
  color: #d97706;
  background: rgba(255, 251, 235, 0.95);
}

.query-toolbar {
  padding: 5px 10px;
}

.query-form {
  display: flex;
  align-items: flex-end;
  flex-wrap: wrap;
  gap: 6px;
}

.query-form :deep(.el-form-item) {
  margin-right: 0;
  margin-bottom: 0;
}

.query-input-item {
  flex: 1 1 420px;
}

.time-item {
  flex: 0 0 300px;
}

.step-select {
  width: 84px;
}

.actions-item {
  margin-left: auto;
}

.toolbar-actions {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.quick-ranges,
.filter-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin-top: 4px;
}

.common-query-actions {
  min-width: 0;
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: 6px;
  margin-top: 4px;
  overflow-x: auto;
  overflow-y: hidden;
  padding-bottom: 2px;
  scrollbar-width: thin;
  scrollbar-color: rgba(148, 163, 184, 0.65) transparent;
}

.common-query-actions::-webkit-scrollbar {
  height: 5px;
}

.common-query-actions::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.55);
}

.common-query-actions::-webkit-scrollbar-track {
  background: transparent;
}

.quick-ranges button {
  height: 20px;
  padding: 0 8px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.76);
  font-size: 11px;
}

.common-query-actions button {
  flex: 0 0 auto;
  height: 22px;
  padding: 0 10px;
  border-color: rgba(76, 139, 245, 0.22);
  border-radius: 8px;
  background: rgba(239, 246, 255, 0.86);
  color: #2563eb;
  font-size: 11px;
  font-weight: 700;
  white-space: nowrap;
}

.common-query-actions button.analysis-action {
  border-color: rgba(96, 165, 250, 0.26);
  background: rgba(239, 246, 255, 0.72);
}

.quick-ranges button.active,
.quick-ranges button:hover,
.common-query-actions button:hover {
  border-color: rgba(76, 139, 245, 0.28);
  background: rgba(76, 139, 245, 0.1);
  color: var(--log-brand);
}

.trend-panel {
  min-height: 132px;
  display: flex;
  flex-direction: column;
  padding: 4px 10px 6px;
  overflow: hidden;
  border-top: 1px solid rgba(223, 231, 242, 0.86);
}

.trend-chart {
  flex: 1;
  min-height: 104px;
  height: 100%;
  cursor: crosshair;
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.table-shell {
  min-height: 0;
  flex: 1;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.86);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.72);
}

.raw-log-list {
  height: 100%;
  overflow: auto;
  background: #fff;
}

.json-log-list {
  height: 100%;
  overflow: auto;
  background: #fff;
}

.json-log-item {
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr);
  gap: 7px;
  padding: 7px 10px 8px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.86);
}

.json-log-item:hover {
  background: rgba(76, 139, 245, 0.035);
}

.json-log-index {
  padding-top: 1px;
  color: #94a3b8;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 12px;
  text-align: right;
}

.json-log-body {
  min-width: 0;
}

.json-log-head {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  margin-bottom: 5px;
}

.json-log-time {
  color: #0f172a;
  font-size: 12px;
  font-weight: 700;
}

.json-field-list {
  display: grid;
  gap: 1px;
}

.json-field-row {
  display: grid;
  grid-template-columns: minmax(96px, max-content) minmax(0, 1fr);
  align-items: start;
  gap: 5px;
  min-width: 0;
  color: #4b5563;
  font-size: 11px;
  line-height: 1.38;
}

.json-field-key {
  color: #111827;
  font-weight: 800;
  white-space: nowrap;
}

.json-field-value {
  min-width: 0;
  display: inline-flex;
  align-items: flex-start;
  gap: 5px;
  overflow-wrap: anywhere;
  color: #64748b;
}

.json-field-content {
  min-width: 0;
  max-width: 100%;
  overflow-wrap: anywhere;
}

.json-field-value.collapsed .json-field-content {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.json-collapse-btn {
  flex: 0 0 auto;
  height: 17px;
  margin-top: 0;
  padding: 0 5px;
  border: 1px solid rgba(147, 197, 253, 0.7);
  border-radius: 5px;
  background: rgba(239, 246, 255, 0.92);
  color: #2563eb;
  cursor: pointer;
  font-size: 10px;
  line-height: 15px;
}

.json-collapse-btn:hover {
  border-color: rgba(37, 99, 235, 0.45);
  background: rgba(219, 234, 254, 0.98);
}

.json-type-tag {
  height: 15px;
  margin-right: 4px;
  padding: 0 4px;
  border: 0;
  border-radius: 3px;
  font-size: 10px;
  line-height: 15px;
}

.result-head-actions,
.row-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.result-head-actions {
  margin-left: auto;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.ai-analysis-trigger {
  flex: 0 0 auto;
}

.ai-analysis-content {
  min-height: 240px;
}

.ai-analysis-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  min-height: 32px;
  margin-bottom: 14px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.92);
  color: var(--log-text-muted);
  font-size: 12px;
}

.ai-analysis-meta :deep(.el-button) {
  margin-left: auto;
}

.ai-analysis-sections {
  display: grid;
}

.ai-analysis-section {
  padding: 14px 0;
  border-bottom: 1px solid rgba(223, 231, 242, 0.82);
}

.ai-analysis-section:first-child {
  padding-top: 0;
}

.ai-analysis-section:last-child {
  border-bottom: 0;
}

.ai-analysis-section h3 {
  margin: 0 0 8px;
  color: var(--log-text-primary);
  font-size: 14px;
  line-height: 1.4;
}

.ai-analysis-section-body {
  color: var(--log-text-secondary);
  font-size: 13px;
  line-height: 1.75;
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}

.raw-log-item {
  display: grid;
  grid-template-columns: 36px minmax(0, 1fr);
  gap: 8px;
  padding: 8px 10px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.86);
}

.raw-log-item:hover {
  background: rgba(76, 139, 245, 0.04);
}

.raw-log-index {
  padding-top: 1px;
  color: var(--log-text-muted);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 12px;
  text-align: right;
}

.raw-log-body {
  min-width: 0;
}

.raw-log-head {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.raw-log-head :deep(.el-button) {
  margin-left: auto;
}

.row-actions {
  margin-left: auto;
  gap: 6px;
}

.row-actions :deep(.el-button) {
  margin-left: 0;
  padding: 0;
  font-size: 12px;
}

.raw-log-time {
  color: #1f2937;
  font-size: 12px;
}

.raw-log-message {
  margin-top: 5px;
  color: #26364b;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 12px;
  line-height: 1.55;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  word-break: break-all;
}

.highlight-line {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
}

.numeric-cell {
  font-variant-numeric: tabular-nums;
}

.query-hit-highlight {
  color: #e11d48;
  font-weight: 800;
  background: rgba(225, 29, 72, 0.12);
  border-radius: 4px;
  padding: 0 2px;
}

.raw-log-main-fields {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 12px;
  margin-top: 5px;
  color: #64748b;
  font-size: 11px;
}

.raw-log-main-fields span {
  min-width: 0;
  display: inline-flex;
  gap: 4px;
  align-items: baseline;
  max-width: 32%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.raw-log-main-fields strong {
  flex: 0 0 auto;
  color: #334155;
  font-weight: 700;
}

.data-table {
  height: 100%;
  --el-table-border-color: rgba(223, 231, 242, 0.92);
  --el-table-header-bg-color: rgba(247, 249, 252, 0.98);
  --el-table-row-hover-bg-color: rgba(76, 139, 245, 0.05);
  --el-table-text-color: var(--log-text-secondary);
  --el-table-header-text-color: var(--log-text-muted);
}

.data-table :deep(.el-table__cell) {
  padding: 6px 0;
}

.data-table :deep(.cell) {
  line-height: 1.35;
}

.chart-actions {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.chart-actions :deep(.el-dropdown) {
  line-height: 1;
}

.chart-actions button {
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--log-text-muted);
  font-size: 12px;
  cursor: pointer;
}

.chart-actions button:hover {
  color: var(--log-brand);
}

.chart-actions .download-action {
  color: #f59e0b;
  font-weight: 700;
}

.chart-actions .download-action:hover {
  color: #d97706;
}

.chart-actions button:disabled {
  color: #cbd5e1;
  cursor: not-allowed;
}

.chart-actions .download-action:disabled {
  color: #fbbf24;
}

.json-view {
  max-height: 360px;
  margin: 0;
  padding: 14px;
  overflow: auto;
  border-radius: 14px;
  background: #f7f9fc;
  color: #344054;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 12px;
  line-height: 1.7;
}

.detail-tabs {
  height: 100%;
}

.detail-tabs :deep(.el-tabs__content) {
  height: calc(100% - 44px);
  overflow: auto;
}

.detail-field-table {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 14px;
  background: #fff;
}

.detail-field-row {
  display: grid;
  grid-template-columns: 180px minmax(0, 1fr);
  border-bottom: 1px solid rgba(223, 231, 242, 0.78);
}

.detail-field-row:last-child {
  border-bottom: 0;
}

.detail-field-row span,
.detail-field-row strong {
  min-width: 0;
  padding: 9px 12px;
  font-size: 12px;
  line-height: 1.5;
}

.detail-field-row span {
  color: #2563eb;
  background: rgba(248, 250, 252, 0.96);
}

.detail-field-row strong {
  color: #334155;
  font-weight: 500;
  overflow-wrap: anywhere;
}

.mono-text {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
}

.pagination-section {
  display: flex;
  justify-content: flex-end;
  flex: 0 0 auto;
  padding-top: 8px;
}

.log-query-page :deep(.el-input__wrapper),
.log-query-page :deep(.el-select__wrapper),
.log-query-page :deep(.el-textarea__inner),
.log-query-page :deep(.el-input-number .el-input__wrapper) {
  border-radius: 14px;
  background: rgba(252, 253, 255, 0.98);
  box-shadow: inset 0 0 0 1px rgba(214, 223, 235, 0.96) !important;
}

.log-query-page :deep(.el-input__wrapper),
.log-query-page :deep(.el-select__wrapper),
.log-query-page :deep(.el-input-number .el-input__wrapper) {
  min-height: 28px;
}

.query-main-input :deep(.el-input__wrapper) {
  background: #e8eef7;
  box-shadow: inset 0 0 0 1px rgba(176, 190, 211, 0.98) !important;
}

.query-main-input :deep(.el-input__inner) {
  color: #243247;
}

.query-main-input :deep(.el-input__inner::placeholder) {
  color: #7b879a;
}

.log-query-page :deep(.el-input__wrapper.is-focus),
.log-query-page :deep(.el-select__wrapper.is-focused) {
  box-shadow: inset 0 0 0 1px rgba(120, 156, 221, 0.96), 0 0 0 4px rgba(76, 139, 245, 0.08) !important;
}

.query-main-input :deep(.el-input__wrapper.is-focus) {
  background: #dde7f3;
}

.log-query-page :deep(.el-button) {
  border-radius: 10px !important;
  font-weight: 600;
  transform: none !important;
}

.log-query-page :deep(.el-button--primary:not(.is-link)) {
  border-color: transparent;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
}

.log-query-page :deep(.el-button--warning:not(.is-link)) {
  border-color: transparent;
  color: #fff;
  background: linear-gradient(180deg, #f6be63 0%, #f3b24b 100%);
}

@media (max-width: 1180px) {
  .log-result-panel {
    grid-template-columns: 1fr;
  }

  .parse-panel {
    max-height: 300px;
    border-right: 0;
    border-bottom: 1px solid rgba(223, 231, 242, 0.86);
  }
}

@media (max-width: 760px) {
  :global(.home-container .el-main:has(.log-query-page)) {
    overflow-y: auto;
  }

  .page-card {
    max-height: none;
  }

  .page-header,
  .panel-head,
  .scope-form {
    flex-direction: column;
    align-items: stretch;
  }

  .page-header-copy,
  .summary-group {
    width: 100%;
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

  .scope-form :deep(.el-select),
  .scope-form :deep(.el-input) {
    width: 100%;
  }

  .time-item {
    flex-basis: 100%;
  }

  .toolbar-actions,
  .toolbar-actions .el-button {
    width: 100%;
  }
}
</style>
