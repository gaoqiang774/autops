<template>
  <div class="trace-page">
    <section class="trace-workbench">
      <header class="page-header">
        <div class="page-header-copy">
          <span class="ops-page-header__icon" aria-hidden="true">
            <el-icon><Connection /></el-icon>
          </span>
          <div class="ops-page-header__content">
            <h1>日志链路</h1>
            <span class="ops-page-header__divider" aria-hidden="true"></span>
            <p>按项目与 Trace ID 关联检索日志，定位跨服务请求中的异常事件与观测路径。</p>
          </div>
        </div>
        <div v-if="listQueryDuration" class="header-metric">
          <span>查询耗时</span>
          <strong>{{ formatDuration(listQueryDuration) }}</strong>
        </div>
      </header>

      <section class="query-toolbar" aria-label="链路查询条件">
        <el-form class="query-form" :model="queryForm" inline @submit.prevent>
          <el-form-item label="项目">
            <el-select
              v-model="queryForm.projectId"
              class="project-select"
              placeholder="选择项目"
              filterable
              @change="handleProjectChange"
            >
              <el-option
                v-for="project in projects"
                :key="project.id"
                :label="project.name || project.code"
                :value="project.id"
              />
            </el-select>
          </el-form-item>

          <el-form-item label="Trace ID">
            <el-input
              v-model.trim="queryForm.traceId"
              class="trace-input"
              placeholder="可选，输入后按 Trace ID 精确查询"
              clearable
              @keyup.enter="handleSearch"
            />
          </el-form-item>

          <el-form-item label="时间">
            <el-date-picker
              v-model="queryForm.timeRange"
              class="time-picker"
              type="datetimerange"
              range-separator="至"
              start-placeholder="开始时间"
              end-placeholder="结束时间"
              value-format="YYYY-MM-DD HH:mm:ss"
              :shortcuts="dateShortcuts"
            />
          </el-form-item>

          <el-form-item class="query-action">
            <el-button type="primary" :icon="Search" :loading="listLoading" @click="handleSearch">查询</el-button>
            <el-button type="warning" :icon="RefreshRight" :disabled="listLoading" @click="handleReset">重置</el-button>
          </el-form-item>
        </el-form>
      </section>

      <div class="workbench-body">
        <aside class="trace-list-panel">
          <div class="panel-heading">
            <div>
              <h3>链路列表</h3>
              <span>{{ listCount }} 条</span>
            </div>
            <el-tooltip content="刷新链路列表" placement="top">
              <el-button
                class="panel-refresh-button"
                type="warning"
                :icon="RefreshRight"
                :loading="listLoading"
                aria-label="刷新链路列表"
                @click="refreshList"
              />
            </el-tooltip>
          </div>

          <div v-if="listPartial || listWarnings.length" class="list-warning" role="status">
            <el-icon><WarningFilled /></el-icon>
            <span>列表数据不完整</span>
            <el-tooltip :content="listWarnings.join('；') || '部分日志库查询失败'" placement="right">
              <button type="button">详情</button>
            </el-tooltip>
          </div>

          <div class="trace-list-content">
            <div v-if="listLoading" class="trace-skeletons" aria-label="正在加载链路列表">
              <el-skeleton v-for="index in 5" :key="index" animated>
                <template #template>
                  <div class="trace-skeleton">
                    <el-skeleton-item variant="text" style="width: 72%" />
                    <el-skeleton-item variant="text" style="width: 44%" />
                    <el-skeleton-item variant="text" style="width: 86%" />
                  </div>
                </template>
              </el-skeleton>
            </div>

            <div v-else-if="listError" class="state-block state-error">
              <el-icon><WarningFilled /></el-icon>
              <strong>链路列表加载失败</strong>
              <span>{{ listError }}</span>
              <el-button size="small" @click="refreshList">重新查询</el-button>
            </div>

            <div v-else-if="!traces.length" class="state-block">
              <el-icon><Connection /></el-icon>
              <strong>当前范围没有链路</strong>
              <span>请调整项目、Trace ID 或时间后重新查询。</span>
            </div>

            <div v-else class="trace-list" role="listbox" aria-label="链路列表">
              <button
                v-for="item in traces"
                :key="item.traceId"
                type="button"
                class="trace-list-item"
                :class="{ active: selectedTraceId === item.traceId }"
                :aria-selected="selectedTraceId === item.traceId"
                @click="selectTrace(item.traceId)"
              >
                <span class="operation" :title="item.primaryOperation || item.traceId">
                  {{ item.primaryOperation || '未识别操作' }}
                </span>
                <span class="trace-id-preview">{{ compactTraceId(item.traceId) }}</span>
                <span class="trace-list-meta">
                  <span class="status-label" :class="`is-${normalizeStatus(item.status)}`">
                    {{ statusText(item.status) }}
                  </span>
                  <span>{{ formatDuration(listItemDuration(item)) }}</span>
                  <span>{{ formatTimeRange(item.firstSeen, item.lastSeen) }}</span>
                </span>
                <span class="trace-list-counts">
                  <span>服务 {{ item.serviceCount || 0 }}</span>
                  <span>日志 {{ item.eventCount || 0 }}</span>
                  <span :class="{ danger: item.errorCount > 0 }">异常 {{ item.errorCount || 0 }}</span>
                  <span v-if="item.partial || item.warningCount" class="partial-mark">部分结果</span>
                </span>
              </button>
            </div>
          </div>

          <footer v-if="!listLoading && (listPage > 1 || listHasMore)" class="list-pagination">
            <el-button size="small" :disabled="listPage <= 1" @click="changePage(listPage - 1)">上一页</el-button>
            <span>第 {{ listPage }} 页</span>
            <el-button size="small" :disabled="!listHasMore" @click="changePage(listPage + 1)">下一页</el-button>
          </footer>
        </aside>

        <main class="trace-detail-panel">
          <div v-if="detailLoading" class="detail-loading" aria-label="正在加载链路详情">
            <el-skeleton animated>
              <template #template>
                <el-skeleton-item variant="h3" style="width: 38%" />
                <div class="detail-skeleton-metrics">
                  <el-skeleton-item v-for="index in 4" :key="index" variant="rect" />
                </div>
                <el-skeleton-item variant="rect" style="height: 42px" />
                <el-skeleton-item variant="rect" style="height: 280px; margin-top: 14px" />
              </template>
            </el-skeleton>
          </div>

          <div v-else-if="detailError" class="state-block detail-state state-error">
            <el-icon><WarningFilled /></el-icon>
            <strong>关联日志详情加载失败</strong>
            <span>{{ detailError }}</span>
            <el-button size="small" @click="retryDetail">重新查询</el-button>
          </div>

          <div v-else-if="!traceDetail" class="state-block detail-state">
            <el-icon><Document /></el-icon>
            <strong>选择一条链路查看关联日志</strong>
            <span>详情按日志时间关联，不代表真实的父子调用关系。</span>
          </div>

          <template v-else>
            <section class="detail-summary">
              <div class="detail-title-row">
                <div class="detail-title">
                  <span class="status-label" :class="`is-${normalizeStatus(traceDetail.status)}`">
                    {{ statusText(traceDetail.status) }}
                  </span>
                  <div>
                    <h3>{{ selectedOperation }}</h3>
                    <div class="trace-identity">
                      <span>Trace ID</span>
                      <code :title="traceDetail.traceId">{{ traceDetail.traceId }}</code>
                      <el-button
                        text
                        :icon="CopyDocument"
                        title="复制 Trace ID"
                        aria-label="复制 Trace ID"
                        @click="copyText(traceDetail.traceId, 'Trace ID 已复制')"
                      />
                    </div>
                    <div v-if="summaryStatsDiffer" class="summary-source-note">
                      详情基于完整 Trace 日志重新统计，可能与左侧摘要口径不同。
                    </div>
                  </div>
                </div>
                <span v-if="traceDetail.queryDurationMs" class="query-cost">
                  查询 {{ formatDuration(traceDetail.queryDurationMs) }}
                </span>
              </div>

              <div class="metric-strip">
                <div>
                  <span>日志跨度</span>
                  <strong>{{ formatDuration(traceDetail.observedDurationMs) }}</strong>
                </div>
                <div>
                  <span>服务</span>
                  <strong>{{ traceDetail.serviceCount || 0 }}</strong>
                </div>
                <div>
                  <span>日志事件</span>
                  <strong>{{ traceDetail.eventCount || 0 }}</strong>
                </div>
                <div :class="{ 'error-metric': traceDetail.errorCount > 0 }">
                  <span>异常</span>
                  <strong>{{ traceDetail.errorCount || 0 }}</strong>
                </div>
              </div>
            </section>

            <section v-if="detailWarnings.length" class="warning-banner" role="status">
              <el-icon><WarningFilled /></el-icon>
              <div>
                <strong>{{ warningHeadline }}</strong>
                <div v-if="warningsExpanded" class="warning-details">
                  <span v-for="warning in detailWarnings" :key="warning">{{ warning }}</span>
                </div>
              </div>
              <div class="warning-actions">
                <el-button text @click="warningsExpanded = !warningsExpanded">
                  {{ warningsExpanded ? '收起详情' : '查看详情' }}
                </el-button>
                <el-button text :icon="RefreshRight" @click="retryDetail">重新查询</el-button>
              </div>
            </section>

            <el-tabs v-model="activeView" class="detail-tabs">
              <el-tab-pane label="观测时间线" name="timeline">
                <div class="timeline-note">
                  <el-icon><InfoFilled /></el-icon>
                  <span>时间条基于日志时间和耗时估算，不代表真实父子调用关系；普通内部日志已折叠。</span>
                </div>
                <div class="timeline-shell">
                  <div class="timeline-header">
                    <span>时间</span>
                    <span>服务</span>
                    <span>事件</span>
                    <span>状态</span>
                    <span>观测窗口</span>
                    <span>耗时</span>
                    <span>来源</span>
                  </div>
                  <template v-for="row in timelineRows" :key="row.key">
                    <button
                      type="button"
                      class="timeline-row"
                      :class="{ 'is-internal-group': row.type === 'internal-group' }"
                      @click="handleTimelineRowClick(row)"
                    >
                      <span class="event-time">{{ formatEventTime(row.timestamp) }}</span>
                      <span class="event-service" :title="row.service">{{ row.service || '-' }}</span>
                      <span class="event-operation" :title="timelineRowTitle(row)">
                        <i class="event-status" :class="eventMarkerClass(row)"></i>
                        {{ timelineRowLabel(row) }}
                      </span>
                      <span>
                        <span v-if="row.type === 'event' && normalizeStatus(row.event.status) !== 'unknown'" class="status-label" :class="`is-${normalizeStatus(row.event.status)}`">
                          {{ statusText(row.event.status) }}
                        </span>
                        <span v-else class="status-placeholder">-</span>
                      </span>
                      <span class="timeline-track" aria-hidden="true">
                        <i :class="durationLevelClass(row)" :style="timelineBarStyle(row)"></i>
                      </span>
                      <span class="event-duration" :class="durationLevelClass(row)">
                        {{ formatDuration(timelineRowDuration(row)) }}
                      </span>
                      <span class="event-source" :title="timelineRowSource(row)">{{ timelineRowSource(row) }}</span>
                    </button>
                    <div v-if="row.type === 'internal-group' && expandedInternalGroups.has(row.key)" class="internal-log-list">
                      <button
                        v-for="(event, index) in row.events"
                        :key="`${eventKey(event)}-${index}`"
                        type="button"
                        class="internal-log-row"
                        @click="openEventDetail(event)"
                      >
                        <span>{{ formatEventTime(event.timestamp) }}</span>
                        <span :title="displayLogText(event.message)">{{ displayLogText(event.message) || '普通日志' }}</span>
                        <span :title="eventSource(event)">{{ eventSource(event) }}</span>
                      </button>
                    </div>
                  </template>
                </div>
              </el-tab-pane>

              <el-tab-pane label="服务汇总" name="services">
                <el-table
                  class="service-table"
                  :data="traceDetail.services || []"
                  height="100%"
                  empty-text="暂无服务汇总"
                >
                  <el-table-column prop="service" label="服务" min-width="170" show-overflow-tooltip />
                  <el-table-column label="状态" width="84">
                    <template #default="{ row }">
                      <span class="status-label" :class="`is-${normalizeStatus(row.status)}`">
                        {{ statusText(row.status) }}
                      </span>
                    </template>
                  </el-table-column>
                  <el-table-column label="首次出现" min-width="166">
                    <template #default="{ row }">{{ formatDateTime(row.firstSeen) }}</template>
                  </el-table-column>
                  <el-table-column label="最后出现" min-width="166">
                    <template #default="{ row }">{{ formatDateTime(row.lastSeen) }}</template>
                  </el-table-column>
                  <el-table-column label="最大耗时" width="100" align="right">
                    <template #default="{ row }">{{ formatDuration(row.maxDurationMs) }}</template>
                  </el-table-column>
                  <el-table-column prop="eventCount" label="日志数" width="82" align="right" />
                  <el-table-column label="异常数" width="82" align="right">
                    <template #default="{ row }">
                      <strong :class="{ 'danger-text': row.errorCount > 0 }">{{ row.errorCount || 0 }}</strong>
                    </template>
                  </el-table-column>
                  <el-table-column label="日志库" min-width="150">
                    <template #default="{ row }">{{ (row.storeCodes || []).join(', ') || '-' }}</template>
                  </el-table-column>
                </el-table>
              </el-tab-pane>

              <el-tab-pane label="原始日志" name="raw">
                <div class="raw-toolbar">
                  <el-input v-model.trim="eventFilters.keyword" clearable placeholder="搜索事件或日志内容" :prefix-icon="Search" />
                  <el-select v-model="eventFilters.service" clearable placeholder="全部服务">
                    <el-option v-for="service in serviceOptions" :key="service" :label="service" :value="service" />
                  </el-select>
                  <el-select v-model="eventFilters.status" clearable placeholder="全部状态">
                    <el-option label="成功" value="success" />
                    <el-option label="告警" value="warning" />
                    <el-option label="异常" value="error" />
                    <el-option label="-" value="unknown" />
                  </el-select>
                  <el-select v-model="eventFilters.store" clearable placeholder="全部日志库">
                    <el-option v-for="store in storeOptions" :key="store" :label="store" :value="store" />
                  </el-select>
                  <span>{{ filteredEvents.length }} 条</span>
                </div>

                <el-table
                  class="raw-table"
                  :data="filteredEvents"
                  height="100%"
                  empty-text="没有符合条件的日志"
                  @row-click="openEventDetail"
                >
                  <el-table-column label="时间" width="176">
                    <template #default="{ row }">{{ formatDateTime(row.timestamp, true) }}</template>
                  </el-table-column>
                  <el-table-column prop="service" label="服务" min-width="130" show-overflow-tooltip />
                  <el-table-column label="状态" width="76">
                    <template #default="{ row }">
                      <span class="status-label" :class="`is-${normalizeStatus(row.status)}`">
                        {{ statusText(row.status) }}
                      </span>
                    </template>
                  </el-table-column>
                  <el-table-column prop="storeCode" label="日志库" min-width="130" show-overflow-tooltip />
                  <el-table-column label="事件" min-width="220" show-overflow-tooltip>
                    <template #default="{ row }">{{ explicitEventText(row) || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="日志内容" min-width="300" show-overflow-tooltip>
                    <template #default="{ row }">{{ displayLogText(row.message) || '-' }}</template>
                  </el-table-column>
                  <el-table-column label="耗时" width="88" align="right">
                    <template #default="{ row }">{{ formatDuration(row.durationMs) }}</template>
                  </el-table-column>
                </el-table>
              </el-tab-pane>
            </el-tabs>
          </template>
        </main>
      </div>
    </section>

    <el-drawer
      v-model="eventDrawer.visible"
      title="日志详情"
      size="min(640px, 92vw)"
      class="event-drawer"
      modal-class="trace-event-drawer-overlay"
    >
      <template #header>
        <div class="drawer-title">
          <div>
            <strong>日志详情</strong>
            <span>{{ eventDrawer.event?.service || '-' }} / {{ formatEventTime(eventDrawer.event?.timestamp) }}</span>
          </div>
          <span v-if="eventDrawer.event" class="status-label" :class="`is-${normalizeStatus(eventDrawer.event.status)}`">
            {{ statusText(eventDrawer.event.status) }}
          </span>
        </div>
      </template>

      <el-tabs v-model="eventDrawer.tab" class="drawer-tabs">
        <el-tab-pane label="字段" name="fields">
          <dl class="event-field-list">
            <template v-for="item in drawerFieldRows" :key="item.key">
              <dt>{{ item.key }}</dt>
              <dd>{{ item.value }}</dd>
            </template>
          </dl>
        </el-tab-pane>
        <el-tab-pane label="JSON" name="json">
          <div class="json-toolbar">
            <span>敏感字段已脱敏</span>
            <el-button text :icon="CopyDocument" @click="copyText(drawerJson, 'JSON 已复制')">复制</el-button>
          </div>
          <pre class="json-view">{{ drawerJson }}</pre>
        </el-tab-pane>
      </el-tabs>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  Connection,
  CopyDocument,
  Document,
  InfoFilled,
  RefreshRight,
  Search,
  WarningFilled
} from '@element-plus/icons-vue'
import logtailApi from '@/api/logtail'

const route = useRoute()
const router = useRouter()

const projects = ref([])
const traces = ref([])
const traceDetail = ref(null)
const selectedTraceId = ref('')
const listLoading = ref(false)
const detailLoading = ref(false)
const listError = ref('')
const detailError = ref('')
const listCount = ref(0)
const listHasMore = ref(false)
const listPage = ref(1)
const listPageSize = 50
const listPartial = ref(false)
const listWarnings = ref([])
const listQueryDuration = ref(0)
const activeView = ref('timeline')
const warningsExpanded = ref(false)
const expandedInternalGroups = ref(new Set())
let listRequestId = 0
let detailRequestId = 0

const queryForm = reactive({
  projectId: '',
  traceId: '',
  timeRange: defaultTimeRange(15)
})

const eventFilters = reactive({
  keyword: '',
  service: '',
  status: '',
  store: ''
})

const eventDrawer = reactive({
  visible: false,
  event: null,
  tab: 'fields'
})

const quickRanges = [
  { text: '最近 5 分钟', minutes: 5 },
  { text: '最近 15 分钟', minutes: 15 },
  { text: '最近 1 小时', minutes: 60 },
  { text: '最近 4 小时', minutes: 240 },
  { text: '最近 24 小时', minutes: 1440 }
]

const dateShortcuts = quickRanges.map(item => ({
  text: item.text,
  value: () => {
    const end = new Date()
    return [new Date(end.getTime() - item.minutes * 60 * 1000), end]
  }
}))

const extractData = response => response?.data?.data ?? response?.data ?? response

const selectedListItem = computed(() => traces.value.find(item => item.traceId === selectedTraceId.value) || null)

const selectedOperation = computed(() => {
  if (selectedListItem.value?.primaryOperation) return selectedListItem.value.primaryOperation
  const event = traceDetail.value?.events?.find(item => item.requestMethod || item.requestPath || item.operation)
  return event?.operation || event?.event || '关联日志详情'
})

const detailWarnings = computed(() => {
  if (!traceDetail.value) return []
  if (!traceDetail.value.partial) return []
  const warnings = [...(traceDetail.value.warnings || [])]
  if (traceDetail.value.partial && !warnings.length) warnings.push('部分日志库查询失败，当前数据可能不完整')
  if (traceDetail.value.truncated) warnings.push('日志事件超过查询上限，仅展示部分结果')
  return [...new Set(warnings)]
})

const warningHeadline = computed(() => {
  if (traceDetail.value?.partial) return '部分日志库查询失败，当前结果可能不完整'
  const first = detailWarnings.value[0]
  return first ? `已返回部分结果：${first}` : '已返回部分结果'
})

const summaryStatsDiffer = computed(() => {
  const listItem = selectedListItem.value
  const detail = traceDetail.value
  if (!listItem || !detail) return false
  return [
    ['serviceCount', 'serviceCount'],
    ['eventCount', 'eventCount'],
    ['errorCount', 'errorCount']
  ].some(([listKey, detailKey]) => {
    const listValue = Number(listItem[listKey] || 0)
    const detailValue = Number(detail[detailKey] || 0)
    return listValue !== detailValue
  })
})

const traceEvents = computed(() => traceDetail.value?.events || [])

const timelineRows = computed(() => buildTimelineRows(traceEvents.value))

const durationThresholds = computed(() => {
  const durations = traceEvents.value
    .map(event => Number(event.durationMs))
    .filter(duration => Number.isFinite(duration) && duration > 0)
    .sort((left, right) => left - right)
  if (!durations.length) return { warning: Number.POSITIVE_INFINITY, danger: Number.POSITIVE_INFINITY }
  return {
    warning: durations[Math.floor((durations.length - 1) * 0.5)],
    danger: durations[Math.floor((durations.length - 1) * 0.8)]
  }
})

const serviceOptions = computed(() => uniqueValues(traceEvents.value.map(item => item.service)))
const storeOptions = computed(() => uniqueValues(traceEvents.value.map(item => item.storeCode || item.storeName)))

const filteredEvents = computed(() => {
  const keyword = eventFilters.keyword.toLowerCase()
  return traceEvents.value.filter(event => {
    if (eventFilters.service && event.service !== eventFilters.service) return false
    if (eventFilters.status && normalizeStatus(event.status) !== eventFilters.status) return false
    if (eventFilters.store && (event.storeCode || event.storeName) !== eventFilters.store) return false
    if (!keyword) return true
    return [event.displayName, event.operation, event.event, event.eventType, event.message, event.service, event.storeCode, event.requestId]
      .some(value => String(value || '').toLowerCase().includes(keyword))
  })
})

const sanitizedDrawerEvent = computed(() => sanitizeValue(eventDrawer.event || {}))

const drawerFieldRows = computed(() => Object.entries(sanitizedDrawerEvent.value)
  .filter(([, value]) => value !== '' && value !== null && value !== undefined && (!Array.isArray(value) || value.length))
  .map(([key, value]) => ({
    key,
    value: typeof value === 'object' ? JSON.stringify(value) : String(value)
  })))

const drawerJson = computed(() => JSON.stringify(sanitizedDrawerEvent.value, null, 2))

function defaultTimeRange(minutes) {
  const end = new Date()
  const start = new Date(end.getTime() - minutes * 60 * 1000)
  return [formatPickerDate(start), formatPickerDate(end)]
}

function formatPickerDate(date) {
  const pad = value => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

function parseDate(value) {
  if (!value) return null
  if (value instanceof Date) return Number.isNaN(value.getTime()) ? null : value
  const date = new Date(String(value).replace(' ', 'T'))
  return Number.isNaN(date.getTime()) ? null : date
}

function toRfc3339(value) {
  const date = parseDate(value)
  return date ? date.toISOString() : String(value || '')
}

function formatDateTime(value, withMilliseconds = false) {
  const date = parseDate(value)
  if (!date) return '-'
  const base = formatPickerDate(date)
  return withMilliseconds ? `${base}.${String(date.getMilliseconds()).padStart(3, '0')}` : base
}

function formatClock(value) {
  const date = parseDate(value)
  return date ? date.toLocaleTimeString('zh-CN', { hour12: false }) : '-'
}

function formatTimeRange(startValue, endValue) {
  const start = formatClock(startValue)
  const end = formatClock(endValue)
  if (start === '-' && end === '-') return '-'
  if (start === end || end === '-') return start
  if (start === '-') return end
  return `${start} ~ ${end}`
}

function formatEventTime(value) {
  const date = parseDate(value)
  if (!date) return '-'
  return `${date.toLocaleTimeString('zh-CN', { hour12: false })}.${String(date.getMilliseconds()).padStart(3, '0')}`
}

function formatDuration(value) {
  const duration = Number(value)
  if (!Number.isFinite(duration) || duration <= 0) return '-'
  if (duration < 1) return '<1ms'
  if (duration < 1000) return `${Math.round(duration)}ms`
  if (duration < 60000) {
    const seconds = duration / 1000
    return `${seconds < 10 ? seconds.toFixed(1) : Math.round(seconds)}s`
  }
  const minutes = Math.floor(duration / 60000)
  const seconds = Math.round((duration % 60000) / 1000)
  return seconds ? `${minutes}m ${seconds}s` : `${minutes}m`
}

function normalizeStatus(value) {
  const status = String(value || '').toLowerCase()
  if (['success', 'ok'].includes(status)) return 'success'
  if (['warning', 'warn'].includes(status)) return 'warning'
  if (['error', 'failed', 'failure', 'fatal'].includes(status)) return 'error'
  return 'unknown'
}

function statusText(value) {
  return {
    success: '成功',
    warning: '告警',
    error: '异常',
    unknown: '-'
  }[normalizeStatus(value)]
}

function compactTraceId(value) {
  const text = String(value || '')
  return text.length > 18 ? `${text.slice(0, 10)}...${text.slice(-6)}` : text
}

function listItemDuration(item) {
  return item?.maxDurationMs ?? item?.observedDurationMs
}

function uniqueValues(values) {
  return [...new Set(values.filter(Boolean))].sort((left, right) => left.localeCompare(right))
}

function getTimePayload() {
  const [start, end] = queryForm.timeRange || []
  return { start: toRfc3339(start), end: toRfc3339(end) }
}

function eventSource(event) {
  return event?.file || event?.filename || event?.sourceFile || event?.storeCode || event?.storeName || '-'
}

function stripAnsi(value) {
  const ansiPattern = new RegExp(`${String.fromCharCode(27)}\\[[0-?]*[ -/]*[@-~]`, 'g')
  return String(value || '')
    .replace(ansiPattern, '')
    .replace(/\\u001b\[[0-?]*[ -/]*[@-~]/g, '')
    .replace(/\\x1b\[[0-?]*[ -/]*[@-~]/g, '')
}

function displayLogText(value) {
  return stripAnsi(value)
}

function eventKey(event) {
  return event.id || event.logId || `${event.timestamp}-${event.service}-${event.displayName || event.operation || event.event || displayLogText(event.message)}`
}

function explicitEventText(event) {
  if (event.displayName) return event.displayName
  if (event.operation) return event.operation
  if (event.event) return event.event
  if (event.requestMethod && event.requestPath) return `${event.requestMethod} ${event.requestPath}`
  const message = displayLogText(event.message)
  const httpMatch = message.match(/\b(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+\/[^\s"',}]+/i)
  const modelMatch = message.match(/\bmodel\.(call|usage)\b/i)
  return httpMatch?.[0] || modelMatch?.[0] || ''
}

function isHttpEvent(event) {
  return /\b(GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)\s+\/\S+/i.test(explicitEventText(event))
}

function isModelEvent(event) {
  return /\bmodel\.(call|usage)\b/i.test(explicitEventText(event))
}

function hasDuration(event) {
  const duration = Number(event.durationMs)
  return Number.isFinite(duration) && duration > 0
}

function hasWarnOrErrorLevel(event) {
  return ['warn', 'warning', 'error', 'fatal'].includes(String(event.level || '').toLowerCase())
}

function isKeyTimelineEvent(event) {
  if (typeof event.isKeyEvent === 'boolean') return event.isKeyEvent
  if (event.eventType === 'internal_log') return false
  return Boolean(explicitEventText(event)) ||
    isHttpEvent(event) ||
    isModelEvent(event) ||
    normalizeStatus(event.status) !== 'unknown' ||
    hasDuration(event) ||
    hasWarnOrErrorLevel(event)
}

function buildTimelineRows(events) {
  const rows = []
  let group = null
  const flushGroup = () => {
    if (!group) return
    rows.push(group)
    group = null
  }

  events.forEach((event, index) => {
    if (isKeyTimelineEvent(event)) {
      flushGroup()
      rows.push({
        type: 'event',
        key: `event-${eventKey(event)}-${index}`,
        event,
        timestamp: event.timestamp,
        service: event.service
      })
      return
    }

    const service = event.service || '-'
    const groupKey = `internal-${service}-${rows.length}`
    if (!group || group.service !== service) {
      flushGroup()
      group = {
        type: 'internal-group',
        key: groupKey,
        timestamp: event.timestamp,
        service,
        events: []
      }
    }
    group.events.push(event)
  })

  flushGroup()
  return rows
}

function timelineEvent(row) {
  return row.type === 'event' ? row.event : row.events[0]
}

function timelineRowLabel(row) {
  if (row.type === 'internal-group') {
    return `内部日志 ${row.events.length} 条`
  }
  const event = row.event
  return explicitEventText(event) || displayLogText(event.message) || '日志事件'
}

function timelineRowTitle(row) {
  if (row.type === 'internal-group') {
    return expandedInternalGroups.value.has(row.key)
      ? `普通内部日志 ${row.events.length} 条，点击收起`
      : `普通内部日志 ${row.events.length} 条，点击展开`
  }
  return timelineRowLabel(row)
}

function timelineRowDuration(row) {
  return row.type === 'event' ? row.event.durationMs : null
}

function timelineRowSource(row) {
  if (row.type === 'internal-group') {
    return uniqueValues(row.events.map(event => eventSource(event)).filter(value => value && value !== '-')).join(', ') || '-'
  }
  return eventSource(row.event)
}

function eventMarkerClass(row) {
  if (row.type === 'internal-group') return 'is-internal'
  const status = normalizeStatus(row.event.status)
  return status === 'unknown' ? durationLevelClass(row) : `is-${status}`
}

function durationLevelClass(row) {
  if (row.type !== 'event' || !hasDuration(row.event)) return 'is-internal'
  const duration = Number(row.event.durationMs)
  if (duration >= durationThresholds.value.danger) return 'is-slow'
  if (duration >= durationThresholds.value.warning) return 'is-medium'
  return 'is-fast'
}

function handleTimelineRowClick(row) {
  if (row.type === 'internal-group') {
    const next = new Set(expandedInternalGroups.value)
    if (next.has(row.key)) next.delete(row.key)
    else next.add(row.key)
    expandedInternalGroups.value = next
    return
  }
  openEventDetail(row.event)
}

function readRouteTimeRange() {
  if (route.query.start && route.query.end) {
    const start = parseDate(route.query.start)
    const end = parseDate(route.query.end)
    if (start && end) return [formatPickerDate(start), formatPickerDate(end)]
  }

  if (route.query.time) {
    const center = parseDate(route.query.time)
    if (center) {
      return [
        formatPickerDate(new Date(center.getTime() - 5 * 60 * 1000)),
        formatPickerDate(new Date(center.getTime() + 5 * 60 * 1000))
      ]
    }
  }

  const rangeMatch = String(route.query.range || '').match(/^(\d+)(m|h)$/)
  if (rangeMatch) {
    const minutes = Number(rangeMatch[1]) * (rangeMatch[2] === 'h' ? 60 : 1)
    return defaultTimeRange(minutes)
  }
  return defaultTimeRange(15)
}

async function loadProjects() {
  try {
    const response = await logtailApi.getProjects({ page: 1, pageSize: 100, status: 1 })
    const data = extractData(response)
    projects.value = Array.isArray(data) ? data : data?.list || data?.items || []
    const routeProjectId = Number(route.query.projectId || 0)
    queryForm.projectId = routeProjectId || projects.value[0]?.id || ''
  } catch (error) {
    projects.value = []
    listError.value = errorMessage(error, '日志项目加载失败')
  }
}

async function handleProjectChange() {
  queryForm.traceId = ''
  selectedTraceId.value = ''
  traceDetail.value = null
  listPage.value = 1
  await handleSearch()
}

async function handleSearch() {
  if (!queryForm.projectId) {
    ElMessage.warning('请先选择项目')
    return
  }
  if (!queryForm.timeRange?.[0] || !queryForm.timeRange?.[1]) {
    ElMessage.warning('请选择查询时间范围')
    return
  }
  listPage.value = 1
  selectedTraceId.value = ''
  traceDetail.value = null
  await syncUrl()
  await loadTraceList({ selectFirst: true })
}

async function handleReset() {
  queryForm.traceId = ''
  queryForm.timeRange = defaultTimeRange(15)
  listPage.value = 1
  selectedTraceId.value = ''
  traceDetail.value = null
  resetEventFilters()
  await syncUrl('')
  await loadTraceList({ selectFirst: true })
}

async function refreshList() {
  await loadTraceList({ selectFirst: !selectedTraceId.value })
}

async function changePage(page) {
  if (page < 1) return
  listPage.value = page
  selectedTraceId.value = ''
  traceDetail.value = null
  await loadTraceList({ selectFirst: true })
}

async function loadTraceList({ selectFirst = false } = {}) {
  const requestId = ++listRequestId
  listLoading.value = true
  listError.value = ''
  try {
    const response = await logtailApi.queryTraces({
      projectId: Number(queryForm.projectId),
      traceId: queryForm.traceId || '',
      ...getTimePayload(),
      page: listPage.value,
      pageSize: listPageSize
    })
    if (requestId !== listRequestId) return
    const data = extractData(response) || {}
    traces.value = Array.isArray(data.items) ? data.items : []
    listCount.value = Number(data.count ?? traces.value.length)
    listHasMore.value = Boolean(data.hasMore)
    listPartial.value = Boolean(data.partial)
    listWarnings.value = Array.isArray(data.warnings) ? data.warnings : []
    listQueryDuration.value = Number(data.queryDurationMs || 0)

    const routeSelected = String(route.query.selected || queryForm.traceId || route.query.traceId || '')
    const targetTraceId = traces.value.some(item => item.traceId === routeSelected)
      ? routeSelected
      : traces.value[0]?.traceId
    listLoading.value = false
    if ((selectFirst || routeSelected) && targetTraceId) await selectTrace(targetTraceId, false)
  } catch (error) {
    if (requestId !== listRequestId) return
    traces.value = []
    listCount.value = 0
    listHasMore.value = false
    listError.value = errorMessage(error, '无法获取最近链路')
  } finally {
    if (requestId === listRequestId) listLoading.value = false
  }
}

async function selectTrace(traceId, updateRoute = true) {
  if (!traceId) return
  selectedTraceId.value = traceId
  resetEventFilters()
  if (updateRoute) await syncUrl(traceId)
  await loadTraceDetail(traceId)
}

async function loadTraceDetail(traceId) {
  const requestId = ++detailRequestId
  detailLoading.value = true
  detailError.value = ''
  warningsExpanded.value = false
  try {
    const response = await logtailApi.queryTrace({
      projectId: Number(queryForm.projectId),
      traceId,
      ...getTimePayload(),
      limit: 1000
    })
    if (requestId !== detailRequestId) return
    traceDetail.value = extractData(response) || null
  } catch (error) {
    if (requestId !== detailRequestId) return
    traceDetail.value = null
    detailError.value = errorMessage(error, '无法获取关联日志详情')
  } finally {
    if (requestId === detailRequestId) detailLoading.value = false
  }
}

function retryDetail() {
  if (selectedTraceId.value) loadTraceDetail(selectedTraceId.value)
}

function resetEventFilters() {
  eventFilters.keyword = ''
  eventFilters.service = ''
  eventFilters.status = ''
  eventFilters.store = ''
  expandedInternalGroups.value = new Set()
  activeView.value = 'timeline'
}

async function syncUrl(selected = selectedTraceId.value) {
  const { start, end } = getTimePayload()
  const query = {
    projectId: String(queryForm.projectId),
    start,
    end
  }
  if (queryForm.traceId) query.traceId = queryForm.traceId
  if (selected && selected !== queryForm.traceId) query.selected = selected
  await router.replace({ query })
}

function timelineBarStyle(row) {
  const event = timelineEvent(row)
  const events = traceDetail.value?.events || []
  const first = parseDate(traceDetail.value?.firstSeen || events[0]?.timestamp)
  const last = parseDate(traceDetail.value?.lastSeen || events[events.length - 1]?.timestamp)
  const observedStart = parseDate(event.startedAt || event.timestamp)
  const observedEnd = parseDate(event.endedAt || event.timestamp)
  if (!first || !last || !observedStart) return { left: '0%', width: '3px' }
  const total = Math.max(last.getTime() - first.getTime(), 1)
  const startOffset = Math.max(0, Math.min(total, observedStart.getTime() - first.getTime()))
  const endOffset = observedEnd ? Math.max(startOffset, Math.min(total, observedEnd.getTime() - first.getTime())) : startOffset
  const left = startOffset / total * 100
  const width = Math.max((endOffset - startOffset) / total * 100, 0.8)
  return { left: `${left}%`, width: `${Math.min(width, 100 - left)}%` }
}

function openEventDetail(event) {
  eventDrawer.event = event
  eventDrawer.tab = 'fields'
  eventDrawer.visible = true
}

async function copyText(value, successMessage) {
  try {
    await navigator.clipboard.writeText(String(value || ''))
    ElMessage.success(successMessage)
  } catch {
    ElMessage.error('复制失败，请手动复制')
  }
}

function sanitizeValue(value, key = '') {
  const secretKeyPattern = /(authorization|cookie|password|passwd|secret|token|api[_-]?key)/i
  if (secretKeyPattern.test(key)) return '<redacted>'
  if (Array.isArray(value)) return value.map(item => sanitizeValue(item))
  if (value && typeof value === 'object') {
    return Object.fromEntries(Object.entries(value).map(([childKey, childValue]) => [childKey, sanitizeValue(childValue, childKey)]))
  }
  if (typeof value !== 'string') return value
  return stripAnsi(value)
    .replace(/(authorization\s*[:=]\s*["']?(?:bearer\s+)?)[^\s,"'}]+/ig, '$1<redacted>')
    .replace(/((?:api[_-]?key|session[_-]?token|access[_-]?token|password|secret)\s*[:=]\s*["']?)[^\s,"'}]+/ig, '$1<redacted>')
}

function errorMessage(error, fallback) {
  return error?.response?.data?.message || error?.message || fallback
}

onMounted(async () => {
  queryForm.traceId = String(route.query.traceId || route.query.queryValue || '')
  queryForm.timeRange = readRouteTimeRange()
  await loadProjects()
  if (queryForm.projectId) await loadTraceList({ selectFirst: true })
})
</script>

<style scoped>
:global(.home-container .el-main:has(.trace-page)) {
  padding-top: var(--ops-shell-gutter, 10px);
  padding-right: var(--ops-shell-gutter, 10px);
  padding-left: var(--ops-shell-gutter, 10px);
  overflow: hidden;
}

.trace-page {
  --trace-height-offset: 124px;
  --ops-text-heading: #0f172a;
  --ops-text-primary: #172033;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-text-weak: #7b8aa0;
  --ops-bg-surface: #ffffff;
  --ops-bg-surface-soft: #fbfdff;
  --ops-bg-subtle: #f3f7fc;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  --ops-border-inner: #dce8f5;
  --ops-border-hover: #b9d4ff;
  --ops-color-primary: #2563eb;
  --ops-color-success: #059669;
  --ops-color-warning: #d97706;
  --ops-color-danger: #dc2626;
  --ops-shadow-card: 0 8px 24px rgba(80, 124, 180, 0.07);
  --trace-text: var(--ops-text-primary);
  --trace-secondary: var(--ops-text-secondary);
  --trace-muted: var(--ops-text-weak);
  --trace-border: var(--ops-border-inner);
  --trace-border-strong: var(--ops-border);
  --trace-surface: var(--ops-bg-surface);
  --trace-surface-subtle: var(--ops-bg-surface-soft);
  --trace-accent: var(--ops-color-primary);
  --trace-accent-soft: #eef6ff;
  height: calc(100vh - var(--trace-height-offset));
  min-height: 0;
  padding: var(--ops-page-padding, 7px);
  overflow: hidden;
  box-sizing: border-box;
  color: var(--trace-text);
  background: transparent;
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
  letter-spacing: 0;
}

.trace-workbench {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: var(--trace-surface);
  box-shadow: var(--ops-shadow-card);
}

.page-header {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-surface);
  box-sizing: border-box;
}

.page-header-copy,
.ops-page-header__content {
  display: flex;
  align-items: center;
  min-width: 0;
}

.page-header-copy {
  flex: 1;
  gap: 8px;
}

.ops-page-header__icon {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  place-items: center;
  border: 0;
  color: #14b8a6;
  background: transparent;
}

.ops-page-header__icon :deep(.el-icon) {
  font-size: 30px;
}

.ops-page-header__content {
  flex: 1;
  gap: 10px;
}

.ops-page-header__content h1 {
  flex: 0 0 auto;
  margin: 0;
  color: var(--ops-text-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
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

.header-metric {
  display: inline-flex;
  align-items: baseline;
  gap: 6px;
  flex: 0 0 auto;
  padding: 6px 9px;
  border: 1px solid var(--ops-border);
  border-radius: 8px;
  background: var(--ops-bg-surface-soft);
  white-space: nowrap;
}

.header-metric span {
  color: var(--ops-text-weak);
  font-size: 11px;
}

.header-metric strong {
  color: var(--ops-text-heading);
  font-family: "SFMono-Regular", Consolas, monospace;
  font-size: 14px;
  font-variant-numeric: tabular-nums;
}

.query-toolbar {
  flex: 0 0 auto;
  padding: 8px 10px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-subtle);
}

.query-form {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: nowrap;
}

.query-form :deep(.el-form-item) {
  min-width: 0;
  margin: 0;
}

.query-form :deep(.el-form-item__label) {
  height: 34px;
  padding-right: 6px;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
  line-height: 34px;
}

.project-select {
  width: 150px;
}

.trace-input {
  width: clamp(260px, 28vw, 420px);
}

.time-picker {
  width: 330px !important;
}

.query-action {
  margin-left: auto !important;
  white-space: nowrap;
}

.query-action :deep(.el-form-item__content) {
  flex-wrap: nowrap;
}

.query-action :deep(.el-button) {
  min-height: 30px;
  padding: 0 10px;
  font-size: 12px;
  white-space: nowrap;
}

.workbench-body {
  min-height: 0;
  flex: 1;
  display: flex;
  overflow: hidden;
}

.trace-list-panel {
  min-width: 310px;
  width: 336px;
  max-width: 380px;
  flex: 0 0 auto;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  resize: horizontal;
  border-right: 1px solid var(--trace-border);
  background: var(--ops-bg-surface-soft);
}

.panel-heading {
  flex: 0 0 44px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 12px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-subtle);
}

.panel-heading > div {
  display: flex;
  align-items: baseline;
  gap: 7px;
}

.panel-heading h3 {
  margin: 0;
  color: var(--ops-text-heading);
  font-size: 13px;
  font-weight: 700;
}

.panel-heading span {
  color: var(--ops-text-muted);
  font-size: 11px;
}

.panel-heading :deep(.panel-refresh-button) {
  width: 30px;
  height: 30px;
  margin: 0;
  padding: 0;
  color: #fff;
  border-color: #f3c66b;
  background: #f3c66b;
}

.list-warning {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  border-bottom: 1px solid #f1d69a;
  background: #fff8e8;
  color: #94610a;
  font-size: 11px;
}

.list-warning button {
  margin-left: auto;
  padding: 0;
  border: 0;
  background: transparent;
  color: #7b4d00;
  cursor: pointer;
}

.trace-list-content {
  min-height: 0;
  flex: 1;
  overflow: auto;
  scrollbar-width: thin;
  scrollbar-color: #c5cfdb transparent;
}

.trace-list {
  min-height: 100%;
}

.trace-list-item {
  position: relative;
  width: 100%;
  min-height: 82px;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  grid-template-areas:
    "operation traceid"
    "meta meta"
    "counts counts";
  align-content: center;
  gap: 6px 8px;
  padding: 8px 10px 8px 14px;
  border: 0;
  border-bottom: 1px solid #e3e9f1;
  background: transparent;
  color: var(--trace-text);
  cursor: pointer;
  text-align: left;
  transition: background-color 0.16s ease, box-shadow 0.16s ease;
}

.trace-list-item::before {
  content: "";
  position: absolute;
  inset: 8px auto 8px 6px;
  width: 3px;
  border-radius: 2px;
  background: transparent;
}

.trace-list-item:hover {
  background: #f3f7fd;
}

.trace-list-item:focus-visible {
  outline: 2px solid #8fb5f4;
  outline-offset: -2px;
}

.trace-list-item.active {
  background: var(--trace-accent-soft);
  box-shadow: inset 0 0 0 1px var(--ops-border-hover);
}

.trace-list-item.active::before {
  background: var(--trace-accent);
}

.operation {
  grid-area: operation;
  overflow: hidden;
  color: #1c2b3e;
  font-size: 13px;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.trace-id-preview {
  grid-area: traceid;
  color: #8793a5;
  font-family: "SFMono-Regular", Consolas, monospace;
  font-size: 10px;
}

.trace-list-meta,
.trace-list-counts {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #68768a;
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}

.trace-list-meta {
  grid-area: meta;
}

.trace-list-counts {
  grid-area: counts;
}

.danger,
.danger-text {
  color: #c63f3f !important;
}

.partial-mark {
  margin-left: auto;
  color: #9b6508;
}

.status-label {
  display: inline-flex;
  align-items: center;
  min-height: 20px;
  padding: 0 6px;
  border: 1px solid currentColor;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 700;
  line-height: 18px;
  white-space: nowrap;
}

.status-label.is-success {
  color: var(--ops-color-success);
  background: #eaf8ef;
}

.status-label.is-warning {
  color: var(--ops-color-warning);
  background: #fff4e5;
}

.status-label.is-error {
  color: var(--ops-color-danger);
  background: #fff0f0;
}

.status-label.is-unknown {
  color: #6f7d90;
  background: #f3f5f8;
}

.list-pagination {
  flex: 0 0 42px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  border-top: 1px solid var(--trace-border);
  background: #fff;
}

.list-pagination span {
  color: var(--trace-muted);
  font-size: 11px;
}

.trace-detail-panel {
  min-width: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: #fff;
}

.detail-summary {
  flex: 0 0 auto;
  padding: 10px 12px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-surface);
}

.detail-title-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.detail-title {
  min-width: 0;
  display: flex;
  align-items: flex-start;
  gap: 10px;
}

.detail-title > .status-label {
  margin-top: 2px;
}

.detail-title h3 {
  max-width: min(62vw, 760px);
  margin: 0;
  overflow: hidden;
  color: #162337;
  font-size: 16px;
  line-height: 1.45;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.trace-identity {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 7px;
  margin-top: 2px;
  color: var(--trace-muted);
  font-size: 11px;
}

.trace-identity code {
  max-width: min(46vw, 540px);
  overflow: hidden;
  color: #55647a;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.trace-identity :deep(.el-button) {
  width: 24px;
  height: 24px;
  padding: 0;
}

.summary-source-note {
  margin-top: 2px;
  color: #7a8798;
  font-size: 11px;
  line-height: 1.45;
}

.query-cost {
  color: var(--trace-muted);
  font-size: 10px;
  white-space: nowrap;
}

.metric-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(95px, 1fr));
  gap: 1px;
  max-width: 620px;
  margin-top: 10px;
  overflow: hidden;
  border: 1px solid var(--trace-border);
  border-radius: 10px;
  background: var(--trace-border);
}

.metric-strip > div {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
  min-height: 40px;
  padding: 7px 10px;
  background: #fff;
}

.metric-strip span {
  color: var(--trace-muted);
  font-size: 11px;
}

.metric-strip strong {
  color: #17243a;
  font-size: 16px;
  font-variant-numeric: tabular-nums;
}

.metric-strip .error-metric {
  background: #fff4f4;
}

.metric-strip .error-metric strong {
  color: #c63f3f;
}

.warning-banner {
  flex: 0 0 auto;
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: start;
  gap: 9px;
  padding: 8px 12px;
  border-bottom: 1px solid #efd69e;
  background: #fff8e9;
  color: #7f550c;
  font-size: 12px;
}

.warning-banner > .el-icon {
  margin-top: 2px;
  color: #c17a00;
}

.warning-banner strong {
  font-weight: 700;
}

.warning-details {
  display: grid;
  gap: 3px;
  margin-top: 4px;
  color: #8b6422;
  font-size: 11px;
}

.warning-actions {
  display: flex;
  align-items: center;
}

.warning-actions :deep(.el-button) {
  margin-left: 5px;
  padding: 0 4px;
  color: #875700;
}

.detail-tabs {
  min-height: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 0 12px 10px;
}

.detail-tabs :deep(.el-tabs__header) {
  flex: 0 0 auto;
  margin: 0 0 8px;
}

.detail-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: var(--trace-border);
}

.detail-tabs :deep(.el-tabs__item) {
  height: 38px;
  padding: 0 17px;
  color: #647286;
  font-size: 12px;
}

.detail-tabs :deep(.el-tabs__item.is-active) {
  color: var(--trace-accent);
  font-weight: 700;
}

.detail-tabs :deep(.el-tabs__content),
.detail-tabs :deep(.el-tab-pane) {
  min-height: 0;
  height: 100%;
  flex: 1;
  overflow: hidden;
}

.timeline-note {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 30px;
  padding: 0 9px;
  border: 1px solid #d7e4f5;
  border-radius: 8px;
  background: #f5f9ff;
  color: #5e7290;
  font-size: 11px;
}

.timeline-shell {
  height: calc(100% - 38px);
  margin-top: 8px;
  overflow: auto;
  border: 1px solid var(--trace-border);
  border-radius: 8px;
}

.timeline-header,
.timeline-row {
  display: grid;
  grid-template-columns: 116px minmax(110px, 0.75fr) minmax(190px, 1.25fr) 70px minmax(150px, 1fr) 68px minmax(118px, 0.8fr);
  align-items: center;
  gap: 10px;
  min-width: 970px;
}

.timeline-header {
  position: sticky;
  z-index: 1;
  top: 0;
  min-height: 32px;
  padding: 0 10px;
  border-bottom: 1px solid var(--trace-border);
  background: #f5f7fa;
  color: var(--trace-muted);
  font-size: 10px;
}

.timeline-header span:nth-child(6) {
  text-align: right;
}

.timeline-row {
  width: 100%;
  min-height: 46px;
  padding: 6px 10px;
  border: 0;
  border-bottom: 1px solid #e7ecf2;
  background: #fff;
  color: #344258;
  cursor: pointer;
  font-size: 11px;
  text-align: left;
}

.timeline-row:last-child {
  border-bottom: 0;
}

.timeline-row:hover,
.timeline-row:focus-visible {
  background: #f6f9fd;
}

.timeline-row.is-internal-group {
  background: #fbfcfe;
}

.event-time,
.event-duration {
  font-family: "SFMono-Regular", Consolas, monospace;
  font-variant-numeric: tabular-nums;
}

.event-time {
  color: #56657a;
}

.event-duration {
  color: #9aa6b5;
  font-size: 12px;
  font-weight: 700;
  text-align: right;
}

.event-duration.is-fast {
  color: #16815c;
}

.event-duration.is-medium {
  color: #a66a00;
}

.event-duration.is-slow {
  color: #c83f3f;
}

.event-duration.is-internal {
  color: #9aa6b5;
  font-weight: 600;
}

.event-service,
.event-operation,
.event-source {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.event-service {
  color: #20314a;
  font-weight: 700;
}

.event-operation {
  display: flex;
  align-items: center;
  gap: 7px;
}

.event-source {
  color: #647286;
}

.event-status {
  flex: 0 0 7px;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #a0abba;
}

.event-status.is-success,
.timeline-track i.is-success {
  background: #31a779;
}

.event-status.is-warning,
.timeline-track i.is-warning {
  background: #d99a28;
}

.event-status.is-error,
.timeline-track i.is-error {
  background: #d95454;
}

.event-status.is-fast,
.timeline-track i.is-fast {
  background: #31a779;
}

.event-status.is-medium,
.timeline-track i.is-medium {
  background: #d99a28;
}

.event-status.is-slow,
.timeline-track i.is-slow {
  background: #d95454;
}

.event-status.is-internal,
.timeline-track i.is-internal {
  background: #a6b1c0;
}

.log-type-label {
  display: inline-flex;
  align-items: center;
  min-height: 20px;
  padding: 0 6px;
  border: 1px solid #c8d2df;
  border-radius: 8px;
  background: #f5f7fa;
  color: #657386;
  font-size: 11px;
  font-weight: 700;
  line-height: 18px;
  white-space: nowrap;
}

.status-placeholder {
  color: #7b8798;
  font-family: "SFMono-Regular", Consolas, monospace;
  font-size: 12px;
}

.timeline-track {
  position: relative;
  height: 10px;
  border-left: 1px solid #dfe6ef;
  border-right: 1px solid #dfe6ef;
  background-image: linear-gradient(90deg, transparent 24.5%, #e8edf3 25%, transparent 25.5%, transparent 49.5%, #e8edf3 50%, transparent 50.5%, transparent 74.5%, #e8edf3 75%, transparent 75.5%);
}

.timeline-track i {
  position: absolute;
  top: 2px;
  min-width: 3px;
  height: 6px;
  border-radius: 2px;
  background: #9ca8b7;
}

.internal-log-list {
  min-width: 970px;
  border-bottom: 1px solid #e7ecf2;
  background: #f8fafc;
}

.internal-log-row {
  width: 100%;
  min-height: 34px;
  display: grid;
  grid-template-columns: 116px minmax(0, 1fr) minmax(118px, 0.8fr);
  align-items: center;
  gap: 10px;
  padding: 4px 10px 4px 36px;
  border: 0;
  border-top: 1px dashed #d9e1ec;
  background: transparent;
  color: #647286;
  cursor: pointer;
  font-size: 11px;
  text-align: left;
}

.internal-log-row:hover,
.internal-log-row:focus-visible {
  background: #eef4fb;
}

.internal-log-row span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.internal-log-row span:first-child {
  font-family: "SFMono-Regular", Consolas, monospace;
  font-variant-numeric: tabular-nums;
}

.service-table,
.raw-table {
  height: 100%;
  overflow: hidden;
  border: 1px solid var(--trace-border);
  border-radius: 8px;
  --el-table-border-color: #e0e6ee;
  --el-table-header-bg-color: #f5f7fa;
  --el-table-row-hover-bg-color: #f5f8fc;
  --el-table-text-color: #415066;
  --el-table-header-text-color: #748196;
}

.service-table :deep(.el-table__cell),
.raw-table :deep(.el-table__cell) {
  padding: 6px 0;
  font-size: 12px;
}

.service-table :deep(.el-table__header .el-table__cell),
.raw-table :deep(.el-table__header .el-table__cell) {
  color: var(--ops-text-secondary);
  font-size: 12px;
  font-weight: 700;
}

.raw-toolbar {
  display: grid;
  grid-template-columns: minmax(190px, 1fr) 150px 120px 150px auto;
  align-items: center;
  gap: 7px;
  min-height: 38px;
  margin-bottom: 8px;
}

.raw-toolbar > span {
  color: var(--trace-muted);
  font-size: 11px;
  white-space: nowrap;
}

.raw-table {
  height: calc(100% - 46px);
}

.state-block {
  min-height: 260px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 24px;
  color: var(--trace-muted);
  text-align: center;
}

.state-block > .el-icon {
  font-size: 30px;
  color: #9aa8ba;
}

.state-block strong {
  color: #3a495e;
  font-size: 14px;
}

.state-block span {
  max-width: 400px;
  font-size: 12px;
  line-height: 1.6;
}

.state-error > .el-icon,
.state-error strong {
  color: #c74b4b;
}

.detail-state {
  min-height: 100%;
  flex: 1;
}

.trace-skeletons {
  padding: 0 12px;
}

.trace-skeleton {
  display: grid;
  gap: 9px;
  min-height: 90px;
  align-content: center;
  border-bottom: 1px solid #e5eaf1;
}

.detail-loading {
  padding: 18px;
}

.detail-skeleton-metrics {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
  margin: 18px 0 14px;
}

.detail-skeleton-metrics :deep(.el-skeleton__item) {
  height: 46px;
}

.drawer-title {
  width: 100%;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding-right: 22px;
}

.drawer-title > div {
  min-width: 0;
  display: grid;
  gap: 3px;
}

.drawer-title strong {
  color: #1c2a3d;
  font-size: 16px;
}

.drawer-title span {
  color: var(--trace-muted);
  font-size: 11px;
}

.drawer-tabs {
  height: 100%;
}

.drawer-tabs :deep(.el-tabs__content) {
  height: calc(100% - 54px);
  overflow: auto;
}

.event-field-list {
  display: grid;
  grid-template-columns: 150px minmax(0, 1fr);
  margin: 0;
  overflow: hidden;
  border: 1px solid var(--trace-border);
  border-radius: 8px;
}

.event-field-list dt,
.event-field-list dd {
  min-width: 0;
  margin: 0;
  padding: 8px 10px;
  border-bottom: 1px solid #e5eaf1;
  font-size: 11px;
  line-height: 1.55;
}

.event-field-list dt {
  color: #3568b9;
  background: #f7f9fc;
  font-family: "SFMono-Regular", Consolas, monospace;
}

.event-field-list dd {
  color: #3e4e63;
  overflow-wrap: anywhere;
}

.event-field-list dt:nth-last-of-type(1),
.event-field-list dd:last-child {
  border-bottom: 0;
}

.json-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 34px;
  color: var(--trace-muted);
  font-size: 11px;
}

.json-view {
  margin: 0;
  padding: 14px;
  overflow: auto;
  border: 1px solid var(--trace-border);
  border-radius: 8px;
  background: var(--ops-bg-subtle);
  color: #334259;
  font-family: "SFMono-Regular", Consolas, monospace;
  font-size: 11px;
  line-height: 1.65;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.trace-page :deep(.el-input__wrapper),
.trace-page :deep(.el-select__wrapper),
.trace-page :deep(.el-date-editor.el-input__wrapper) {
  min-height: 34px;
  border-radius: 8px;
  background: var(--ops-bg-surface-soft);
  box-shadow: inset 0 0 0 1px #d9e5f2 !important;
}

.trace-page :deep(.el-input__wrapper:hover),
.trace-page :deep(.el-select__wrapper:hover),
.trace-page :deep(.el-date-editor.el-input__wrapper:hover) {
  box-shadow: inset 0 0 0 1px var(--ops-border-hover) !important;
}

.trace-page :deep(.el-input__wrapper.is-focus),
.trace-page :deep(.el-select__wrapper.is-focused),
.trace-page :deep(.el-date-editor.el-input__wrapper.is-active) {
  box-shadow: inset 0 0 0 1px #409eff, 0 0 0 3px rgba(64, 158, 255, 0.1) !important;
}

.trace-page :deep(.el-button) {
  border-radius: 8px;
  font-weight: 600;
  transition:
    color 160ms ease,
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease,
    transform 160ms ease;
}

.trace-page :deep(.el-button--primary:not(.is-text):not(.is-link)) {
  color: #fff;
  border-color: #409eff;
  background: #409eff;
}

.trace-page :deep(.el-button--warning:not(.is-text):not(.is-link)) {
  color: #fff;
  border-color: #e6a23c;
  background: #e6a23c;
}

.trace-page :deep(.el-button:focus-visible) {
  outline: 2px solid rgba(64, 158, 255, 0.45);
  outline-offset: 2px;
}

.trace-page :deep(.el-button:active) {
  transform: translateY(1px);
}

:global(.event-drawer.el-drawer) {
  --ops-bg-surface: #fff;
  --ops-bg-subtle: #f3f7fc;
  --ops-text-heading: #0f172a;
  --trace-muted: #7b8aa0;
  --trace-border: #dce8f5;
  overflow: hidden;
  border-radius: 15px 0 0 15px;
  background: var(--ops-bg-surface);
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.event-drawer .el-drawer__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
}

:global(.event-drawer .el-drawer__body) {
  padding: 12px 18px 16px;
}

@media (max-width: 1280px) {
  .query-form {
    flex-wrap: wrap;
  }

  .trace-input {
    width: 220px;
  }

  .time-picker {
    width: 310px !important;
  }

  .query-action {
    margin-left: 0 !important;
  }
}

@media (max-width: 980px) {
  .query-form {
    width: 100%;
  }

  .workbench-body {
    min-height: 780px;
  }

  .trace-list-panel {
    min-width: 300px;
    width: 320px;
  }

  .metric-strip {
    max-width: none;
  }

  .raw-toolbar {
    grid-template-columns: minmax(180px, 1fr) 130px 110px;
  }

  .raw-toolbar > :nth-child(4) {
    grid-column: 1 / 3;
  }
}

@media (max-width: 760px) {
  :global(.home-container .el-main:has(.trace-page)) {
    padding-right: 6px;
    padding-left: 6px;
    overflow-y: auto;
  }

  .trace-page {
    height: auto;
    min-height: calc(100vh - var(--trace-height-offset));
    padding: 6px;
    overflow: visible;
  }

  .trace-workbench {
    height: auto;
    min-height: calc(100vh - var(--trace-height-offset));
  }

  .page-header {
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .page-header-copy,
  .header-metric {
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

  .ops-page-header__content p {
    width: 100%;
    white-space: normal;
  }

  .header-metric {
    justify-content: space-between;
  }

  .query-form,
  .query-form :deep(.el-form-item) {
    width: 100%;
  }

  .query-form :deep(.el-form-item) {
    display: grid;
    grid-template-columns: 58px minmax(0, 1fr);
  }

  .project-select,
  .trace-input,
  .time-picker,
  .query-action :deep(.el-button) {
    width: 100% !important;
  }

  .workbench-body {
    min-height: 0;
    overflow: visible;
    flex-direction: column;
  }

  .trace-list-panel {
    width: 100%;
    min-width: 0;
    max-width: none;
    height: 360px;
    resize: none;
    border-right: 0;
    border-bottom: 1px solid var(--trace-border);
  }

  .trace-detail-panel {
    min-height: 660px;
  }

  .detail-title h3 {
    max-width: 68vw;
  }

  .metric-strip {
    grid-template-columns: repeat(2, 1fr);
  }

  .warning-banner {
    grid-template-columns: auto minmax(0, 1fr);
  }

  .warning-actions {
    grid-column: 2;
    justify-content: flex-start;
  }

  .raw-toolbar {
    grid-template-columns: 1fr 1fr;
  }

  .raw-toolbar > :first-child {
    grid-column: 1 / 3;
  }

  .raw-toolbar > :nth-child(4) {
    grid-column: auto;
  }

  .event-field-list {
    grid-template-columns: 110px minmax(0, 1fr);
  }
}
</style>
