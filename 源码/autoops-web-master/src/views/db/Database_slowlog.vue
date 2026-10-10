<template>
  <div class="slowlog-page">
    <section class="slowlog-shell">
      <header class="slowlog-header">
        <div>
          <h1>数据库慢 SQL 异步分析</h1>
          <span>读取数据库资产配置的 MySQL 慢日志路径，异步聚合 SQL 指纹并展示 Top 100。</span>
        </div>
        <div class="header-actions">
          <el-button class="refresh-action" :icon="Refresh" :loading="assetLoading" @click="loadAssets">
            刷新实例
          </el-button>
          <el-button
            class="history-action"
            :icon="Clock"
            :loading="historyLoading && historyDialogVisible"
            :disabled="!query.assetId"
            @click="openHistoryDialog"
          >
            历史记录
          </el-button>
          <el-button
            class="export-action"
            :icon="Download"
            :loading="exporting"
            :disabled="!canExport"
            @click="handleExport"
          >
            下载 CSV
          </el-button>
        </div>
      </header>

      <section class="filter-panel">
        <el-form :inline="true" :model="query" class="filter-form" @submit.prevent>
          <div class="filter-row filter-row-primary">
            <el-form-item label="数据库实例">
              <el-select
                v-model="query.assetId"
                class="asset-select"
                filterable
                remote
                reserve-keyword
                placeholder="请选择 MySQL 资产"
                :loading="assetLoading"
                :remote-method="searchAssets"
                @visible-change="handleAssetVisible"
                @change="handleAssetChange"
              >
                <el-option
                  v-for="asset in assets"
                  :key="getAssetId(asset)"
                  :label="assetLabel(asset)"
                  :value="getAssetId(asset)"
                >
                  <div class="asset-option">
                    <strong>{{ asset.instanceName || asset.name || `资产 #${getAssetId(asset)}` }}</strong>
                    <el-tag size="small" effect="light">{{ engineText(asset) }}</el-tag>
                    <span>{{ asset.endpoint || joinEndpoint(asset.hostIp, asset.port) || '-' }}</span>
                    <el-tag v-if="asset.slowLogPath" size="small" type="success" effect="light">已配置慢日志</el-tag>
                    <el-tag v-else size="small" type="warning" effect="light">未配置慢日志</el-tag>
                  </div>
                </el-option>
              </el-select>
            </el-form-item>

            <el-form-item class="filter-actions">
              <el-button
                class="primary-action"
                :icon="VideoPlay"
                :loading="submitting"
                :disabled="!query.assetId"
                @click="handleStartAnalyze"
              >
                开始分析
              </el-button>
              <el-button
                class="upload-action"
                :icon="Upload"
                :loading="uploadSubmitting"
                :disabled="!query.assetId"
                @click="openUploadDialog"
              >
                上传分析
              </el-button>
              <el-button
                class="refresh-action"
                :icon="Refresh"
                :loading="taskLoading"
                :disabled="!currentTaskId"
                @click="refreshTask"
              >
                刷新状态
              </el-button>
            </el-form-item>
          </div>
        </el-form>
      </section>

      <section class="status-panel" :class="`status-${taskStatus || 'idle'}`">
        <div class="status-topline">
          <div class="status-main">
            <span class="status-dot" :class="taskStatus || 'idle'"></span>
            <div>
              <div class="status-title-row">
                <strong>{{ statusTitle }}</strong>
                <el-tag :type="statusTagType" effect="light" round>{{ statusText }}</el-tag>
              </div>
              <p>{{ statusMessage }}</p>
            </div>
          </div>
          <div class="status-side">
            <span v-if="currentTaskId" class="task-id mono">Task {{ currentTaskId }}</span>
          </div>
        </div>

        <div v-if="currentTask" class="progress-row">
          <el-progress
            :percentage="progressValue"
            :status="progressStatus"
            :stroke-width="6"
            :show-text="true"
          />
          <span>{{ progressSummaryText }}</span>
          <span v-if="currentTask.error" class="progress-error">{{ currentTask.error }}</span>
        </div>

        <div class="overview-matrix">
          <div class="overview-item overview-item--path">
            <span>{{ sourcePathLabel }}</span>
            <strong class="mono">{{ sourcePathText }}</strong>
          </div>
          <div class="overview-item">
            <span>关联主机</span>
            <strong>{{ sourceHostText }}</strong>
          </div>
          <div class="overview-item">
            <span>实例地址</span>
            <strong class="mono">{{ selectedAsset?.endpoint || joinEndpoint(selectedAsset?.hostIp, selectedAsset?.port) || '-' }}</strong>
          </div>
          <div class="overview-item">
            <span>生成时间</span>
            <strong>{{ generatedAt || '-' }}</strong>
          </div>
          <article v-for="card in summaryCards" :key="card.key" class="overview-item overview-item--metric">
            <span>{{ card.label }}</span>
            <strong>{{ card.value }}</strong>
          </article>
        </div>
      </section>

      <section v-if="alertMessages.length" class="warning-stack">
        <el-alert
          v-for="message in alertMessages"
          :key="message"
          :title="message"
          type="warning"
          show-icon
          :closable="false"
        />
      </section>

      <section class="table-panel">
        <div class="table-head">
          <div>
            <h2>慢 SQL Top 100</h2>
            <span>{{ tableHint }}</span>
          </div>
          <div class="table-actions">
            <el-input
              v-model="tableKeyword"
              class="table-search"
              clearable
              :prefix-icon="Search"
              placeholder="过滤当前结果"
            />
          </div>
        </div>

        <div class="table-shell">
          <el-table
            v-loading="(taskLoading && !resultItems.length) || resultLoading"
            :data="filteredResultItems"
            class="slowlog-table"
            height="100%"
            row-key="digest"
            :empty-text="emptyText"
            :default-sort="{ prop: 'queryTimeTotal', order: 'descending' }"
            @sort-change="handleResultSortChange"
          >
            <el-table-column prop="firstSeen" label="执行开始时间" min-width="155" show-overflow-tooltip />
            <el-table-column prop="database" label="数据库" min-width="130" show-overflow-tooltip>
              <template #default="{ row }">
                <span class="db-name-badge">{{ row.database || '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="user" label="用户" min-width="110" show-overflow-tooltip />
            <el-table-column prop="host" label="来源主机" min-width="130" show-overflow-tooltip />
            <el-table-column label="SQL 语句" min-width="320" show-overflow-tooltip>
              <template #default="{ row }">
                <div class="sql-cell">
                  <span class="sql-snippet mono">{{ getRowSql(row) }}</span>
                  <el-tooltip content="复制 SQL" placement="top">
                    <button class="sql-copy-button" type="button" @click.stop="copySql(row)">
                      <el-icon><CopyDocument /></el-icon>
                    </button>
                  </el-tooltip>
                </div>
              </template>
            </el-table-column>
            <el-table-column prop="count" label="次数" width="82" align="right" sortable="custom">
              <template #default="{ row }">{{ formatInteger(row.count) }}</template>
            </el-table-column>
            <el-table-column prop="queryTimeP95" label="P95" width="92" align="right" sortable="custom">
              <template #default="{ row }">{{ formatSeconds(row.queryTimeP95) }}</template>
            </el-table-column>
            <el-table-column prop="queryTimeAvg" label="平均耗时" width="108" align="right" sortable="custom">
              <template #default="{ row }">{{ formatSeconds(row.queryTimeAvg) }}</template>
            </el-table-column>
            <el-table-column prop="queryTimeMax" label="最大耗时" width="108" align="right" sortable="custom">
              <template #default="{ row }">{{ formatSeconds(row.queryTimeMax) }}</template>
            </el-table-column>
            <el-table-column prop="queryTimeTotal" label="总耗时" width="108" align="right" sortable="custom">
              <template #default="{ row }">{{ formatSeconds(row.queryTimeTotal) }}</template>
            </el-table-column>
            <el-table-column prop="lockTimeTotal" label="锁等待" width="100" align="right" sortable="custom">
              <template #default="{ row }">{{ formatSeconds(row.lockTimeTotal) }}</template>
            </el-table-column>
            <el-table-column prop="rowsExaminedTotal" label="扫描行数" min-width="120" align="right" sortable="custom">
              <template #default="{ row }">{{ formatInteger(row.rowsExaminedTotal) }}</template>
            </el-table-column>
            <el-table-column prop="rowsSentTotal" label="返回行数" min-width="110" align="right">
              <template #default="{ row }">{{ formatInteger(row.rowsSentTotal) }}</template>
            </el-table-column>
            <el-table-column label="AI 建议" width="96" align="center" fixed="right">
              <template #default="{ row }">
                <el-button
                  v-if="hasAiSuggestion(row)"
                  class="ai-suggestion-button"
                  link
                  type="primary"
                  :icon="View"
                  @click="openAiSuggestionDialog(row)"
                >
                  查看
                </el-button>
                <span v-else class="empty-cell">-</span>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </section>
    </section>

    <el-dialog
      v-model="historyDialogVisible"
      class="slowlog-history-dialog"
      width="980px"
      align-center
      destroy-on-close
    >
      <template #header>
        <div class="history-dialog-header">
          <div>
            <strong>历史慢日志数据</strong>
            <span>{{ selectedAsset ? assetLabel(selectedAsset) : '请选择数据库实例' }}</span>
          </div>
          <el-button
            class="danger-action"
            :icon="Delete"
            :loading="clearingHistory"
            :disabled="!query.assetId || historyLoading"
            @click="handleClearHistory"
          >
            清空历史
          </el-button>
        </div>
      </template>

      <div class="history-toolbar">
        <el-select
          v-model="historyQuery.status"
          class="history-status-select"
          clearable
          placeholder="全部状态"
          @change="handleHistorySearch"
        >
          <el-option label="等待中" value="pending" />
          <el-option label="分析中" value="running" />
          <el-option label="已完成" value="success" />
          <el-option label="失败" value="failed" />
        </el-select>
        <el-input
          v-model="historyQuery.keyword"
          class="history-keyword"
          clearable
          :prefix-icon="Search"
          placeholder="搜索任务 ID / 消息 / 错误"
          @keyup.enter="handleHistorySearch"
          @clear="handleHistorySearch"
        />
        <el-date-picker
          v-model="historyQuery.timeRange"
          class="history-time-range"
          type="datetimerange"
          range-separator="-"
          start-placeholder="开始时间"
          end-placeholder="结束时间"
          value-format="YYYY-MM-DD HH:mm:ss"
          @change="handleHistorySearch"
        />
        <el-button class="refresh-action" :icon="Refresh" :loading="historyLoading" @click="loadHistoryTasks">
          刷新
        </el-button>
      </div>

      <div class="history-table-shell">
        <el-table
          v-loading="historyLoading"
          :data="historyRows"
          class="history-table"
          height="430"
          row-key="taskId"
          empty-text="暂无历史慢日志任务"
        >
          <el-table-column prop="taskId" label="任务 ID" min-width="150" show-overflow-tooltip>
            <template #default="{ row }">
              <span class="mono">{{ row.taskId || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="status" label="状态" width="92">
            <template #default="{ row }">
              <el-tag :type="getHistoryStatusTag(row.status)" effect="light" round>
                {{ getHistoryStatusText(row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="message" label="进度信息" min-width="210" show-overflow-tooltip>
            <template #default="{ row }">{{ row.message || row.error || '-' }}</template>
          </el-table-column>
          <el-table-column prop="summary.uniqueSql" label="SQL 指纹" width="96" align="right">
            <template #default="{ row }">{{ formatInteger(row.summary?.uniqueSql) }}</template>
          </el-table-column>
          <el-table-column prop="summary.totalQueryTime" label="总耗时" width="105" align="right">
            <template #default="{ row }">{{ formatSeconds(row.summary?.totalQueryTime) }}</template>
          </el-table-column>
          <el-table-column prop="summary.maxQueryTime" label="最大耗时" width="105" align="right">
            <template #default="{ row }">{{ formatSeconds(row.summary?.maxQueryTime) }}</template>
          </el-table-column>
          <el-table-column prop="completedAt" label="完成时间" min-width="150" show-overflow-tooltip>
            <template #default="{ row }">{{ formatDateTime(row.completedAt || row.createdAt) || '-' }}</template>
          </el-table-column>
          <el-table-column label="操作" width="150" fixed="right" align="right">
            <template #default="{ row }">
              <el-button link type="primary" :icon="View" @click="handleViewHistory(row)">查看</el-button>
              <el-button
                link
                type="danger"
                :icon="Delete"
                :loading="deletingTaskId === row.taskId"
                :disabled="!canDeleteHistoryTask(row)"
                @click="handleDeleteHistory(row)"
              >
                删除
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="history-pagination">
        <span>共 {{ historyTotal }} 条</span>
        <el-pagination
          v-model:current-page="historyQuery.page"
          v-model:page-size="historyQuery.pageSize"
          small
          background
          layout="sizes, prev, pager, next"
          :page-sizes="[10, 20, 50]"
          :total="historyTotal"
          @size-change="handleHistoryPageSizeChange"
          @current-change="loadHistoryTasks"
        />
      </div>
    </el-dialog>

    <el-dialog
      v-model="uploadDialogVisible"
      class="slowlog-upload-dialog"
      width="720px"
      align-center
      :close-on-click-modal="!uploadSubmitting"
      :close-on-press-escape="!uploadSubmitting"
      @closed="resetUploadDialog"
    >
      <template #header>
        <div class="upload-dialog-header">
          <div>
            <strong>手动上传慢日志文件分析</strong>
            <span>{{ selectedAsset ? assetLabel(selectedAsset) : '请选择数据库实例' }}</span>
          </div>
        </div>
      </template>

      <el-upload
        class="slowlog-upload-box"
        drag
        :auto-upload="false"
        :show-file-list="false"
        :disabled="uploadSubmitting"
        :on-change="handleUploadFileChange"
      >
        <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
        <div class="el-upload__text">拖拽慢日志文件到此处，或点击选择</div>
        <template #tip>
          <div class="el-upload__tip">最大 150MB，上传后立即创建异步分析任务。</div>
        </template>
      </el-upload>

      <div v-if="uploadFile" class="upload-file-row">
        <div>
          <strong>{{ uploadFile.name }}</strong>
          <span>{{ formatBytes(uploadFile.size) }}</span>
        </div>
        <el-button link type="danger" :disabled="uploadSubmitting" @click="clearUploadFile">移除</el-button>
      </div>

      <div v-if="uploadSubmitting || uploadProgress > 0" class="upload-progress-panel">
        <div class="upload-progress-title">
          <strong>{{ uploadProgress >= 100 ? '文件已上传，等待后台分析' : '文件上传中' }}</strong>
          <span>{{ uploadProgress }}%</span>
        </div>
        <el-progress :percentage="uploadProgress" :stroke-width="8" :status="uploadProgress >= 100 ? 'success' : undefined" />
      </div>

      <template #footer>
        <div class="upload-dialog-footer">
          <el-button :disabled="uploadSubmitting" @click="uploadDialogVisible = false">取消</el-button>
          <el-button
            class="primary-action"
            :icon="Upload"
            :loading="uploadSubmitting"
            :disabled="!uploadFile || !query.assetId"
            @click="handleUploadAnalyze"
          >
            上传并分析
          </el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog
      v-model="aiSuggestionDialogVisible"
      class="slowlog-ai-dialog"
      width="760px"
      align-center
    >
      <template #header>
        <div class="ai-dialog-header">
          <div>
            <strong>AI 优化建议</strong>
            <span>{{ aiSuggestionDialogDigest }}</span>
          </div>
        </div>
      </template>

      <div class="ai-suggestion-summary">
        <span>SQL 语句</span>
        <strong class="mono">{{ getRowSql(aiSuggestionRow) }}</strong>
      </div>

      <div class="ai-suggestion-sections">
        <section v-for="section in aiSuggestionSections" :key="section.key" class="ai-suggestion-section">
          <span>{{ section.label }}</span>
          <p>{{ section.value || '-' }}</p>
        </section>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Clock, CopyDocument, Delete, Download, Refresh, Search, Upload, UploadFilled, VideoPlay, View } from '@element-plus/icons-vue'
import dbApi from '@/api/database'

const MYSQL_ENGINE = 1
const POLL_INTERVAL = 2000

const assets = ref([])
const assetLoading = ref(false)
const assetKeyword = ref('')
const currentTask = ref(null)
const submitting = ref(false)
const taskLoading = ref(false)
const resultLoading = ref(false)
const exporting = ref(false)
const polling = ref(false)
const tableKeyword = ref('')
const uploadDialogVisible = ref(false)
const uploadSubmitting = ref(false)
const uploadProgress = ref(0)
const uploadFile = ref(null)
const aiSuggestionDialogVisible = ref(false)
const aiSuggestionRow = ref({})
const historyDialogVisible = ref(false)
const historyLoading = ref(false)
const clearingHistory = ref(false)
const deletingTaskId = ref('')
const historyTasks = ref([])
const historyTotal = ref(0)
let pollTimer = null

const RESULT_SORT_FIELDS = new Set([
  'count',
  'queryTimeP95',
  'queryTimeAvg',
  'queryTimeMax',
  'queryTimeTotal',
  'lockTimeTotal',
  'rowsExaminedTotal'
])

const query = reactive({
  assetId: '',
  sortBy: 'queryTimeTotal',
  sortOrder: 'desc',
  authId: ''
})

const historyQuery = reactive({
  page: 1,
  pageSize: 10,
  status: '',
  keyword: '',
  timeRange: []
})

const selectedAsset = computed(() => {
  const id = String(query.assetId || '')
  return assets.value.find(item => String(getAssetId(item)) === id) || null
})

const currentTaskId = computed(() => currentTask.value?.taskId || currentTask.value?.id || '')
const taskStatus = computed(() => normalizeStatus(currentTask.value?.status))
const resultItems = computed(() => normalizeSlowLogItems(currentTask.value?.result?.items))
const resultWarnings = computed(() => normalizeStringList(currentTask.value?.result?.warnings))
const generatedAt = computed(() => formatDateTime(currentTask.value?.result?.generatedAt || currentTask.value?.completedAt))
const canExport = computed(() => Boolean(query.assetId && currentTaskId.value && taskStatus.value === 'success'))
const historyRows = computed(() => historyTasks.value.map(item => normalizeSlowLogTask(item)))
const aiSuggestionDialogDigest = computed(() => aiSuggestionRow.value?.digest ? `指纹 ID：${aiSuggestionRow.value.digest}` : '当前慢 SQL')
const aiSuggestionSections = computed(() => {
  const suggestion = normalizeAiSuggestion(aiSuggestionRow.value?.aiSuggestion)
  return [
    { key: 'slowReason', label: '慢查询原因', value: suggestion.slowReason },
    { key: 'schemaOptimization', label: '表结构优化', value: suggestion.schemaOptimization },
    { key: 'indexOptimization', label: '索引优化', value: suggestion.indexOptimization },
    { key: 'sqlOptimization', label: 'SQL 语句优化', value: suggestion.sqlOptimization },
    { key: 'expectedEffect', label: '预期效果', value: suggestion.expectedEffect }
  ]
})

const progressValue = computed(() => {
  const value = Number(currentTask.value?.progress)
  if (Number.isFinite(value)) return Math.max(0, Math.min(100, Math.round(value)))
  if (taskStatus.value === 'success' || taskStatus.value === 'failed') return 100
  if (taskStatus.value === 'running') return 35
  return 0
})

const progressStatus = computed(() => {
  if (taskStatus.value === 'success') return 'success'
  if (taskStatus.value === 'failed') return 'exception'
  return undefined
})

const statusText = computed(() => ({
  pending: '等待中',
  running: '分析中',
  success: '已完成',
  failed: '失败',
  idle: '未开始'
}[taskStatus.value || 'idle'] || '未知'))

const statusTagType = computed(() => ({
  pending: 'info',
  running: 'primary',
  success: 'success',
  failed: 'danger'
}[taskStatus.value] || 'info'))

const statusTitle = computed(() => {
  if (!query.assetId) return '请选择数据库实例'
  if (!currentTask.value) return '准备提交慢 SQL 分析任务'
  return {
    pending: '任务已提交，等待后台执行',
    running: '后台正在解析慢日志',
    success: '分析完成，已生成 Top 100',
    failed: '分析失败'
  }[taskStatus.value] || '任务状态未知'
})

const statusMessage = computed(() => {
  if (!query.assetId) return '仅支持 MySQL 慢日志分析，请先选择已配置 slowLogPath 的数据库资产。'
  if (!currentTask.value) return '提交后前端会每 2 秒刷新任务状态，任务结束后自动停止轮询。'
  if (taskStatus.value === 'failed') return currentTask.value.error || currentTask.value.message || '请检查慢日志路径、主机绑定与 SSH 凭据。'
  return currentTask.value.message || '等待后端返回进度信息。'
})

const sourceHostText = computed(() => {
  const source = currentTask.value?.source || {}
  const host = [source.hostName, source.hostIp].filter(Boolean).join(' / ')
  if (host) return host
  return selectedAsset.value?.hostName || selectedAsset.value?.hostIp || selectedAsset.value?.host || '-'
})

const isUploadedSource = computed(() => currentTask.value?.source?.type === 'uploaded_mysql_slow_log')
const sourcePathLabel = computed(() => (isUploadedSource.value ? '日志来源' : '慢日志路径'))
const sourcePathText = computed(() => {
  if (isUploadedSource.value) return currentTask.value?.source?.path || '上传文件'
  return currentTask.value?.source?.path || selectedAsset.value?.slowLogPath || '-'
})

const sshCredentialOptions = computed(() => normalizeSshCredentialOptions(selectedAsset.value))

const progressSummaryText = computed(() => {
  const parts = []
  if (taskStatus.value === 'pending') parts.push('等待后台分析')
  if (taskStatus.value === 'running') parts.push(`正在解析慢日志 ${progressValue.value}%`)
  if (taskStatus.value === 'success') parts.push('分析完成')
  if (taskStatus.value === 'failed') parts.push('分析失败')
  const message = currentTask.value?.message || ''
  if (message) parts.push(message)
  const summaryValue = summary.value || {}
  const totalEntries = formatInteger(summaryValue.totalEntries)
  const uniqueSql = formatInteger(summaryValue.uniqueSql)
  if (totalEntries !== '-') parts.push(`已发现慢日志 ${totalEntries} 条`)
  if (uniqueSql !== '-') parts.push(`SQL 指纹 ${uniqueSql} 个`)
  return parts.filter(Boolean).join('，')
})

const summary = computed(() => currentTask.value?.summary || {})
const summaryCards = computed(() => [
  {
    key: 'totalEntries',
    label: '慢日志总数',
    value: formatInteger(summary.value.totalEntries),
    hint: '本次读取范围内的慢日志条目'
  },
  {
    key: 'matchedEntries',
    label: '匹配条目',
    value: formatInteger(summary.value.matchedEntries),
    hint: '符合筛选条件的日志数量'
  },
  {
    key: 'uniqueSql',
    label: 'SQL 指纹数',
    value: formatInteger(summary.value.uniqueSql),
    hint: '归一化聚合后的唯一 SQL'
  },
  {
    key: 'totalQueryTime',
    label: '总执行时长',
    value: formatSeconds(summary.value.totalQueryTime),
    hint: 'Query_time 累计值'
  },
  {
    key: 'maxQueryTime',
    label: '最大执行时长',
    value: formatSeconds(summary.value.maxQueryTime),
    hint: '单条慢 SQL 最大耗时'
  },
  {
    key: 'totalRowsExamined',
    label: '总扫描行数',
    value: formatInteger(summary.value.totalRowsExamined),
    hint: 'Rows_examined 累计值'
  }
])

const alertMessages = computed(() => {
  const messages = []
  if (selectedAsset.value && getAssetEngine(selectedAsset.value) !== MYSQL_ENGINE) {
    messages.push('当前慢日志分析第一版仅支持 MySQL 资产。')
  }
  if (selectedAsset.value && !selectedAsset.value.slowLogPath && !uploadDialogVisible.value && !isUploadedSource.value) {
    messages.push('当前实例未配置慢日志路径，请先到数据库资产中配置 slowLogPath。')
  }
  if (selectedAsset.value && !getAssetHostId(selectedAsset.value) && !selectedAsset.value.hostIp) {
    messages.push('当前数据库资产未关联主机，后端可能无法读取慢日志文件。')
  }
  if (taskStatus.value === 'failed' && currentTask.value?.error) {
    messages.push(currentTask.value.error)
  }
  resultWarnings.value.forEach(item => messages.push(item))
  return Array.from(new Set(messages.filter(Boolean)))
})

const filteredResultItems = computed(() => {
  const keyword = tableKeyword.value.trim().toLowerCase()
  if (!keyword) return resultItems.value
  return resultItems.value.filter(row => [
    row.database,
    row.user,
    row.host,
    row.sampleSql,
    row.fingerprint,
    row.aiSuggestion?.slowReason,
    row.aiSuggestion?.schemaOptimization,
    row.aiSuggestion?.indexOptimization,
    row.aiSuggestion?.sqlOptimization,
    row.aiSuggestion?.expectedEffect,
    row.digest
  ].filter(Boolean).join(' ').toLowerCase().includes(keyword))
})

const tableHint = computed(() => {
  if (taskStatus.value === 'success') return `已返回 ${resultItems.value.length} 条聚合结果，当前显示 ${filteredResultItems.value.length} 条`
  if (taskStatus.value === 'running' || taskStatus.value === 'pending') return '任务运行中，完成后展示排序后的 Top 100'
  if (taskStatus.value === 'failed') return '请根据错误信息调整配置后重新提交'
  return '提交任务后在这里查看 SQL 指纹聚合结果'
})

const emptyText = computed(() => {
  if (!currentTask.value) return '请先选择实例并提交慢 SQL 分析任务'
  if (taskStatus.value === 'pending' || taskStatus.value === 'running') return '慢 SQL 分析中，完成后将展示 Top 100'
  if (taskStatus.value === 'success') return '当前筛选条件下未发现慢 SQL'
  if (taskStatus.value === 'failed') return '分析失败，请调整配置后重新提交'
  return '暂无慢 SQL 数据'
})

function getRowSql(row = {}) {
  return row.sampleSql || row.fingerprint || '-'
}

function hasAiSuggestion(row = {}) {
  const suggestion = normalizeAiSuggestion(row.aiSuggestion)
  return Boolean([
    suggestion.slowReason,
    suggestion.schemaOptimization,
    suggestion.indexOptimization,
    suggestion.sqlOptimization,
    suggestion.expectedEffect
  ].some(Boolean))
}

function openAiSuggestionDialog(row = {}) {
  aiSuggestionRow.value = row
  aiSuggestionDialogVisible.value = true
}

async function copySql(row = {}) {
  const text = getRowSql(row)
  if (!text || text === '-') return

  try {
    if (navigator?.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = text
      textarea.setAttribute('readonly', 'readonly')
      textarea.style.position = 'fixed'
      textarea.style.left = '-9999px'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }
    ElMessage.success('SQL 已复制')
  } catch (error) {
    ElMessage.error('复制失败，请手动选择 SQL')
  }
}

function unwrap(response) {
  const body = response?.data || response || {}
  if (body.code && body.code !== 200 && body.success !== true) {
    throw new Error(body.message || '请求失败')
  }
  return body.data !== undefined ? body.data : body
}

function normalizeList(payload) {
  if (Array.isArray(payload)) return payload
  if (Array.isArray(payload?.list)) return payload.list
  if (Array.isArray(payload?.rows)) return payload.rows
  if (Array.isArray(payload?.data)) return payload.data
  if (Array.isArray(payload?.items)) return payload.items
  if (Array.isArray(payload?.records)) return payload.records
  return []
}

function normalizeTotal(payload) {
  const value = payload?.total ?? payload?.count ?? payload?.Total ?? payload?.totalCount
  const num = Number(value)
  return Number.isFinite(num) ? num : normalizeList(payload).length
}

function normalizeStatus(status = '') {
  const value = String(status || '').toLowerCase()
  return ['pending', 'running', 'success', 'failed'].includes(value) ? value : ''
}

function normalizeSlowLogTask(payload = {}) {
  const task = payload || {}
  const result = task.result || task.Result || {}
  return {
    ...task,
    taskId: task.taskId || task.task_id || task.id || task.TaskID,
    assetId: task.assetId || task.asset_id || task.AssetID,
    status: normalizeStatus(task.status || task.Status),
    progress: task.progress ?? task.Progress,
    message: task.message || task.Message || '',
    error: task.error || task.Error || '',
    source: normalizeSource(task.source || task.Source || {}),
    summary: normalizeSummary(task.summary || task.Summary || {}),
    result: {
      ...result,
      items: normalizeSlowLogItems(result.items || result.Items),
      warnings: normalizeStringList(result.warnings || result.Warnings),
      generatedAt: result.generatedAt || result.generated_at || result.GeneratedAt
    },
    createdAt: task.createdAt || task.created_at || task.CreatedAt,
    startedAt: task.startedAt || task.started_at || task.StartedAt,
    completedAt: task.completedAt || task.completed_at || task.CompletedAt
  }
}

function applySlowLogResult(payload = {}) {
  const response = Array.isArray(payload) ? { items: payload } : payload || {}
  const result = response.result || response.Result || response
  const resultItemsPayload = result.items || result.Items || result.list || result.rows || result.data || result
  const nextTask = currentTask.value ? { ...currentTask.value } : {}

  currentTask.value = {
    ...nextTask,
    result: {
      ...(nextTask.result || {}),
      ...result,
      items: normalizeSlowLogItems(resultItemsPayload),
      warnings: normalizeStringList(result.warnings || result.Warnings || nextTask.result?.warnings),
      generatedAt: result.generatedAt || result.generated_at || result.GeneratedAt || nextTask.result?.generatedAt
    }
  }
}

function normalizeSource(source = {}) {
  return {
    configured: source.configured ?? source.Configured,
    type: source.type || source.Type || '',
    path: source.path || source.Path || '',
    hostId: source.hostId || source.host_id || source.HostID,
    hostIp: source.hostIp || source.host_ip || source.HostIP || '',
    hostName: source.hostName || source.host_name || source.HostName || ''
  }
}

function normalizeSummary(value = {}) {
  return {
    totalEntries: value.totalEntries ?? value.total_entries ?? value.TotalEntries,
    matchedEntries: value.matchedEntries ?? value.matched_entries ?? value.MatchedEntries,
    uniqueSql: value.uniqueSql ?? value.unique_sql ?? value.UniqueSQL,
    totalQueryTime: value.totalQueryTime ?? value.total_query_time ?? value.TotalQueryTime,
    maxQueryTime: value.maxQueryTime ?? value.max_query_time ?? value.MaxQueryTime,
    totalLockTime: value.totalLockTime ?? value.total_lock_time ?? value.TotalLockTime,
    totalRowsExamined: value.totalRowsExamined ?? value.total_rows_examined ?? value.TotalRowsExamined,
    totalRowsSent: value.totalRowsSent ?? value.total_rows_sent ?? value.TotalRowsSent
  }
}

function normalizeSlowLogItems(items = []) {
  return normalizeList(items).map((item, index) => ({
    ...item,
    digest: item.digest || item.Digest || `slow-sql-${index}`,
    database: item.database || item.Database || '',
    user: item.user || item.User || '',
    host: item.host || item.Host || '',
    firstSeen: item.firstSeen || item.first_seen || item.FirstSeen || '',
    lastSeen: item.lastSeen || item.last_seen || item.LastSeen || '',
    count: item.count ?? item.Count,
    queryTimeAvg: item.queryTimeAvg ?? item.query_time_avg ?? item.QueryTimeAvg,
    queryTimeP95: item.queryTimeP95 ?? item.query_time_p95 ?? item.QueryTimeP95,
    queryTimeMax: item.queryTimeMax ?? item.query_time_max ?? item.QueryTimeMax,
    queryTimeTotal: item.queryTimeTotal ?? item.query_time_total ?? item.QueryTimeTotal,
    lockTimeTotal: item.lockTimeTotal ?? item.lock_time_total ?? item.LockTimeTotal,
    rowsExaminedTotal: item.rowsExaminedTotal ?? item.rows_examined_total ?? item.RowsExaminedTotal,
    rowsExaminedMax: item.rowsExaminedMax ?? item.rows_examined_max ?? item.RowsExaminedMax,
    rowsSentTotal: item.rowsSentTotal ?? item.rows_sent_total ?? item.RowsSentTotal,
    rowsSentMax: item.rowsSentMax ?? item.rows_sent_max ?? item.RowsSentMax,
    fingerprint: item.fingerprint || item.Fingerprint || '',
    sampleSql: item.sampleSql || item.sample_sql || item.SampleSQL || '',
    aiSuggestion: normalizeAiSuggestion(item.aiSuggestion || item.ai_suggestion || item.AISuggestion || item.AiSuggestion),
    examples: normalizeStringList(item.examples || item.Examples)
  }))
}

function normalizeAiSuggestion(value = {}) {
  if (!value) {
    return {
      slowReason: '',
      schemaOptimization: '',
      indexOptimization: '',
      sqlOptimization: '',
      expectedEffect: ''
    }
  }

  if (typeof value === 'string') {
    return {
      slowReason: value,
      schemaOptimization: '',
      indexOptimization: '',
      sqlOptimization: '',
      expectedEffect: ''
    }
  }

  return {
    slowReason: value.slowReason || value.slow_reason || value.SlowReason || '',
    schemaOptimization: value.schemaOptimization || value.schema_optimization || value.SchemaOptimization || '',
    indexOptimization: value.indexOptimization || value.index_optimization || value.IndexOptimization || '',
    sqlOptimization: value.sqlOptimization || value.sql_optimization || value.SQLOptimization || value.SqlOptimization || '',
    expectedEffect: value.expectedEffect || value.expected_effect || value.ExpectedEffect || ''
  }
}

function normalizeStringList(value = []) {
  if (!value) return []
  if (Array.isArray(value)) return value.map(item => String(item)).filter(Boolean)
  return [String(value)].filter(Boolean)
}

function getAssetId(asset = {}) {
  return asset.id || asset.assetId || asset.dbId
}

function getAssetEngine(asset = {}) {
  return Number(asset.type || asset.engine || asset.dbType || asset.databaseType || 0)
}

function getAssetHostId(asset = {}) {
  return asset.hostId || asset.host_id || asset.cmdbHostId || asset.cmdb_host_id
}

function engineText(asset = {}) {
  return ({
    1: 'MySQL',
    2: 'PostgreSQL',
    3: 'Oracle',
    4: 'Redis',
    5: 'MongoDB',
    6: 'SQL Server',
    7: 'Elasticsearch'
  }[getAssetEngine(asset)] || asset.typeName || '数据库')
}

function assetLabel(asset = {}) {
  return `${engineText(asset)} ${asset.instanceName || asset.name || `资产 #${getAssetId(asset)}`} ${asset.endpoint || joinEndpoint(asset.hostIp, asset.port) || ''}`.trim()
}

function joinEndpoint(host, port) {
  return [host, port].filter(Boolean).join(':')
}

function normalizeSshCredentialOptions(asset = {}) {
  if (!asset) return []
  const auths = []
  const pushAuth = item => {
    if (!item) return
    const authId = item.authId || item.id || item.sshKeyId
    if (!authId) return
    const username = item.username || item.sshName || item.user || ''
    const host = item.hostIp || item.ip || asset.hostIp || asset.privateIp || ''
    const name = item.name || item.alias || item.title || ''
    const title = name || (username && host ? `${username}@${host}` : username || '默认凭据')
    const subtitle = [username && host ? `${username}@${host}` : '', item.typeName || item.kind || item.source].filter(Boolean).join(' · ')
    auths.push({
      authId,
      title,
      subtitle,
      label: subtitle ? `${title} / ${subtitle}` : title
    })
  }

  ;(Array.isArray(asset.sshAuths) ? asset.sshAuths : []).forEach(pushAuth)
  ;(Array.isArray(asset.auths) ? asset.auths : []).forEach(pushAuth)
  ;(Array.isArray(asset.sshKeyIds) ? asset.sshKeyIds : []).forEach(id => pushAuth({ authId: id }))
  ;(Array.isArray(asset.authIds) ? asset.authIds : []).forEach(id => pushAuth({ authId: id }))
  if (asset.sshKeyId) pushAuth({ authId: asset.sshKeyId, name: '默认凭据' })
  if (asset.authId) pushAuth({ authId: asset.authId, name: '默认凭据' })

  const seen = new Set()
  return auths.filter(item => {
    const key = String(item.authId)
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
}

function buildCreatePayload() {
  return {
    authId: normalizeMaybeNumber(query.authId),
    sortBy: query.sortBy || 'queryTimeTotal',
    sortOrder: query.sortOrder || 'desc'
  }
}

function normalizeMaybeNumber(value) {
  if (value === '' || value === null || value === undefined) return undefined
  const num = Number(value)
  return Number.isFinite(num) && String(value).trim() !== '' ? num : value
}

function formatInteger(value) {
  const num = Number(value)
  if (!Number.isFinite(num)) return '-'
  return Math.round(num).toLocaleString()
}

function formatNumber(value, digits = 2) {
  const num = Number(value)
  if (!Number.isFinite(num)) return '-'
  return num.toLocaleString(undefined, {
    minimumFractionDigits: Math.abs(num) >= 100 ? 0 : digits,
    maximumFractionDigits: Math.abs(num) >= 100 ? 0 : digits
  })
}

function formatSeconds(value) {
  const num = Number(value)
  if (!Number.isFinite(num)) return '-'
  if (Math.abs(num) >= 60) return `${formatNumber(num / 60, 2)} min`
  return `${formatNumber(num, 3)} s`
}

function formatBytes(value) {
  const num = Number(value)
  if (!Number.isFinite(num)) return '-'
  if (num >= 1024 * 1024 * 1024) return `${formatNumber(num / 1024 / 1024 / 1024, 2)} GB`
  if (num >= 1024 * 1024) return `${formatNumber(num / 1024 / 1024, 2)} MB`
  if (num >= 1024) return `${formatNumber(num / 1024, 2)} KB`
  return `${Math.round(num)} B`
}

function formatDateTime(value) {
  if (!value) return ''
  return String(value).replace('T', ' ').replace(/\+\d{2}:\d{2}$/, '')
}

function getBlobFilename(response, fallback) {
  const disposition = response?.headers?.['content-disposition'] || response?.headers?.['Content-Disposition'] || ''
  const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i)
  if (utf8Match) return decodeURIComponent(utf8Match[1])
  const match = disposition.match(/filename="?([^"]+)"?/i)
  return match ? decodeURIComponent(match[1]) : fallback
}

function downloadBlob(blob, filename) {
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  polling.value = false
}

function startPolling() {
  stopPolling()
  if (!query.assetId || !currentTaskId.value) return
  polling.value = true
  pollTimer = setInterval(refreshTask, POLL_INTERVAL)
}

async function loadAssets(keyword = assetKeyword.value) {
  assetLoading.value = true
  assetKeyword.value = keyword || ''
  try {
    const payload = unwrap(await dbApi.getCrudAssets({
      page: 1,
      pageSize: 100,
      keyword: assetKeyword.value || undefined
    }))
    assets.value = normalizeList(payload).filter(item => getAssetEngine(item) === MYSQL_ENGINE)
    if (!query.assetId && assets.value.length) {
      query.assetId = getAssetId(assets.value[0])
      applyDefaultAuth()
    } else if (query.assetId && !selectedAsset.value) {
      query.assetId = ''
    }
  } finally {
    assetLoading.value = false
  }
}

function searchAssets(keyword = '') {
  loadAssets(keyword)
}

function handleAssetVisible(visible) {
  if (visible) loadAssets(assetKeyword.value)
}

function handleAssetChange() {
  stopPolling()
  currentTask.value = null
  tableKeyword.value = ''
  historyTasks.value = []
  historyTotal.value = 0
  applyDefaultAuth()
}

function applyDefaultAuth() {
  query.authId = sshCredentialOptions.value.length === 1 ? sshCredentialOptions.value[0].authId : ''
}

function openUploadDialog() {
  if (!query.assetId) {
    ElMessage.warning('请先选择数据库实例')
    return
  }
  resetUploadDialog()
  uploadDialogVisible.value = true
}

function resetUploadDialog() {
  if (uploadSubmitting.value) return
  uploadProgress.value = 0
  uploadFile.value = null
}

function handleUploadFileChange(file) {
  const rawFile = file?.raw || file
  if (!rawFile) return
  if (rawFile.size > 150 * 1024 * 1024) {
    ElMessage.warning('慢日志文件最大支持 150MB')
    uploadFile.value = null
    return
  }
  uploadProgress.value = 0
  uploadFile.value = rawFile
}

function clearUploadFile() {
  uploadFile.value = null
  uploadProgress.value = 0
}

function buildUploadPayload() {
  const formData = new FormData()
  formData.append('file', uploadFile.value)
  formData.append('sortBy', query.sortBy || 'queryTimeTotal')
  formData.append('sortOrder', query.sortOrder || 'desc')
  return formData
}

async function handleUploadAnalyze() {
  if (!query.assetId) {
    ElMessage.warning('请先选择数据库实例')
    return
  }
  if (!uploadFile.value) {
    ElMessage.warning('请选择慢日志文件')
    return
  }

  stopPolling()
  uploadSubmitting.value = true
  uploadProgress.value = 0
  try {
    const payload = unwrap(await dbApi.uploadSlowLogTask(query.assetId, buildUploadPayload(), {
      onUploadProgress: event => {
        const total = Number(event.total || uploadFile.value?.size || 0)
        if (total) uploadProgress.value = Math.min(99, Math.round((Number(event.loaded || 0) / total) * 100))
      }
    }))
    uploadProgress.value = 100
    currentTask.value = normalizeSlowLogTask(payload)
    tableKeyword.value = ''
    uploadDialogVisible.value = false
    ElMessage.success('慢日志文件已上传，分析任务已提交')
    await refreshTask()
    if (!['success', 'failed'].includes(taskStatus.value)) startPolling()
  } catch (error) {
    ElMessage.error(error.message || '上传慢日志文件失败')
  } finally {
    uploadSubmitting.value = false
  }
}

async function handleStartAnalyze() {
  if (!query.assetId) {
    ElMessage.warning('请先选择数据库实例')
    return
  }
  if (selectedAsset.value && getAssetEngine(selectedAsset.value) !== MYSQL_ENGINE) {
    ElMessage.warning('当前慢 SQL 分析仅支持 MySQL 资产')
    return
  }

  stopPolling()
  submitting.value = true
  try {
    const payload = unwrap(await dbApi.createSlowLogTask(query.assetId, buildCreatePayload()))
    currentTask.value = normalizeSlowLogTask(payload)
    tableKeyword.value = ''
    ElMessage.success('慢 SQL 分析任务已提交')
    await refreshTask()
    if (!['success', 'failed'].includes(taskStatus.value)) startPolling()
  } catch (error) {
    ElMessage.error(error.message || '提交慢 SQL 分析任务失败')
  } finally {
    submitting.value = false
  }
}

async function refreshTask() {
  if (!query.assetId || !currentTaskId.value) return
  taskLoading.value = true
  try {
    const payload = unwrap(await dbApi.getSlowLogTask(query.assetId, currentTaskId.value))
    currentTask.value = normalizeSlowLogTask(payload)
    if (['success', 'failed'].includes(taskStatus.value)) stopPolling()
  } catch (error) {
    stopPolling()
    ElMessage.error(error.message || '刷新任务状态失败')
  } finally {
    taskLoading.value = false
  }
}

async function handleResultSortChange({ prop, order } = {}) {
  if (!query.assetId || !currentTaskId.value || taskStatus.value !== 'success') return

  const params = {}
  if (prop && order) {
    if (!RESULT_SORT_FIELDS.has(prop)) return
    params.sortBy = prop
    params.sortOrder = order === 'ascending' ? 'asc' : 'desc'
  }

  resultLoading.value = true
  try {
    const payload = unwrap(await dbApi.getSlowLogTaskResult(query.assetId, currentTaskId.value, params))
    applySlowLogResult(payload)
  } catch (error) {
    ElMessage.error(error.message || '结果排序失败')
  } finally {
    resultLoading.value = false
  }
}

function buildHistoryParams() {
  const [beginTime, endTime] = Array.isArray(historyQuery.timeRange) ? historyQuery.timeRange : []
  return {
    page: historyQuery.page,
    pageSize: historyQuery.pageSize,
    status: historyQuery.status || undefined,
    keyword: historyQuery.keyword?.trim() || undefined,
    beginTime: beginTime || undefined,
    endTime: endTime || undefined
  }
}

async function openHistoryDialog() {
  if (!query.assetId) {
    ElMessage.warning('请先选择数据库实例')
    return
  }
  historyDialogVisible.value = true
  historyQuery.page = 1
  await loadHistoryTasks()
}

function handleHistorySearch() {
  historyQuery.page = 1
  loadHistoryTasks()
}

function handleHistoryPageSizeChange() {
  historyQuery.page = 1
  loadHistoryTasks()
}

async function loadHistoryTasks() {
  if (!query.assetId) return
  historyLoading.value = true
  try {
    const payload = unwrap(await dbApi.getSlowLogTasks(query.assetId, buildHistoryParams()))
    historyTasks.value = normalizeList(payload)
    historyTotal.value = normalizeTotal(payload)
  } catch (error) {
    ElMessage.error(error.message || '加载历史慢日志任务失败')
  } finally {
    historyLoading.value = false
  }
}

function canDeleteHistoryTask(row = {}) {
  return ['success', 'failed'].includes(normalizeStatus(row.status))
}

function getHistoryStatusText(status = '') {
  return ({
    pending: '等待中',
    running: '分析中',
    success: '已完成',
    failed: '失败'
  }[normalizeStatus(status)] || '未知')
}

function getHistoryStatusTag(status = '') {
  return ({
    pending: 'info',
    running: 'primary',
    success: 'success',
    failed: 'danger'
  }[normalizeStatus(status)] || 'info')
}

async function handleViewHistory(row = {}) {
  if (!query.assetId || !row.taskId) return
  taskLoading.value = true
  try {
    const payload = unwrap(await dbApi.getSlowLogTask(query.assetId, row.taskId))
    currentTask.value = normalizeSlowLogTask(payload)
    tableKeyword.value = ''
    historyDialogVisible.value = false
    if (taskStatus.value === 'success' && !resultItems.value.length) {
      const resultPayload = unwrap(await dbApi.getSlowLogTaskResult(query.assetId, row.taskId, {}))
      applySlowLogResult(resultPayload)
    }
    if (['pending', 'running'].includes(taskStatus.value)) startPolling()
  } catch (error) {
    ElMessage.error(error.message || '查看历史任务失败')
  } finally {
    taskLoading.value = false
  }
}

async function handleDeleteHistory(row = {}) {
  if (!query.assetId || !row.taskId || !canDeleteHistoryTask(row)) return
  try {
    await ElMessageBox.confirm('确认删除这条慢日志历史任务吗？删除后不可恢复。', '删除历史记录', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      confirmButtonClass: 'el-button--danger'
    })
    deletingTaskId.value = row.taskId
    await dbApi.deleteSlowLogTask(query.assetId, row.taskId)
    ElMessage.success('历史任务已删除')
    if (String(currentTaskId.value) === String(row.taskId)) {
      currentTask.value = null
      stopPolling()
    }
    await loadHistoryTasks()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') {
      ElMessage.error(error.message || '删除历史任务失败')
    }
  } finally {
    deletingTaskId.value = ''
  }
}

async function handleClearHistory() {
  if (!query.assetId) return
  try {
    await ElMessageBox.confirm('确认清空当前实例下已完成或失败的慢日志历史任务吗？运行中的任务不会受影响。', '清空历史记录', {
      type: 'warning',
      confirmButtonText: '清空',
      cancelButtonText: '取消',
      confirmButtonClass: 'el-button--danger'
    })
    clearingHistory.value = true
    await dbApi.clearSlowLogTasks(query.assetId)
    ElMessage.success('历史任务已清空')
    if (['success', 'failed'].includes(taskStatus.value)) currentTask.value = null
    historyQuery.page = 1
    await loadHistoryTasks()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') {
      ElMessage.error(error.message || '清空历史任务失败')
    }
  } finally {
    clearingHistory.value = false
  }
}

async function handleExport() {
  if (!canExport.value) return
  exporting.value = true
  try {
    const response = await dbApi.exportSlowLogTask(query.assetId, currentTaskId.value)
    const blob = response.data instanceof Blob ? response.data : new Blob([response.data], { type: 'text/csv;charset=utf-8' })
    downloadBlob(blob, getBlobFilename(response, `slow-log-${currentTaskId.value}.csv`))
  } finally {
    exporting.value = false
  }
}

onMounted(() => {
  loadAssets()
})

onBeforeUnmount(() => {
  stopPolling()
})
</script>

<style scoped>
.slowlog-page {
  --aiops-bg-page: #f4f7fb;
  --aiops-bg-page-soft: #eef3f9;
  --aiops-surface-strong: rgba(255, 255, 255, 0.92);
  --aiops-surface-warm: rgba(255, 253, 248, 0.92);
  --aiops-text-primary: #172133;
  --aiops-text-secondary: #5f6b7c;
  --aiops-text-muted: #8d97a8;
  --aiops-border-soft: rgba(22, 34, 56, 0.06);
  --aiops-border-cool: #dfe7f2;
  --aiops-border-mid: rgba(207, 217, 230, 0.92);
  --aiops-brand: #4c8bf5;
  --aiops-brand-active: #4d79d8;
  --aiops-brand-soft: rgba(76, 139, 245, 0.12);
  --aiops-success: #3fb97f;
  --aiops-warning: #f3b24b;
  --aiops-danger: #ef6b73;
  min-height: 100%;
  padding: 6px 8px 10px;
  background:
    radial-gradient(circle at 18% 12%, rgba(76, 139, 245, 0.06), transparent 28%),
    linear-gradient(180deg, var(--aiops-bg-page-soft) 0%, var(--aiops-bg-page) 100%);
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.slowlog-shell {
  position: relative;
  display: flex;
  height: calc(100vh - 96px);
  max-height: calc(100vh - 96px);
  min-height: 0;
  flex-direction: column;
  gap: 8px;
  overflow: hidden;
  border: 1px solid var(--aiops-border-soft);
  border-radius: 28px;
  background:
    linear-gradient(180deg, rgba(255, 253, 248, 0.92) 0%, rgba(247, 250, 255, 0.9) 100%);
  backdrop-filter: blur(16px);
}

.slowlog-shell::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 84px;
  border-radius: 28px 28px 0 0;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  pointer-events: none;
}

.slowlog-header {
  position: relative;
  z-index: 1;
  display: flex;
  flex-shrink: 0;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  min-height: 84px;
  box-sizing: border-box;
  padding: 18px 18px 10px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.72);
}

.header-kicker {
  margin: 0 0 5px;
  color: var(--aiops-brand);
  font-size: 12px;
  font-weight: 700;
}

.slowlog-header h1 {
  margin: 0;
  color: var(--aiops-text-primary);
  font-size: 23px;
  font-weight: 700;
  line-height: 1.25;
}

.slowlog-header span {
  display: block;
  margin-top: 4px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.35;
}

.header-actions,
.table-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 10px;
}

.filter-panel,
.status-panel,
.warning-stack,
.table-panel {
  position: relative;
  z-index: 1;
  margin: 0 12px;
}

.filter-panel {
  flex-shrink: 0;
  padding: 8px 12px 2px;
  border: 1px solid rgba(207, 217, 230, 0.72);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.58);
}

.filter-form {
  display: grid;
  gap: 0;
}

.filter-row {
  display: flex;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 0 10px;
}

.filter-form :deep(.el-form-item) {
  margin: 0 0 6px;
}

.filter-form :deep(.el-form-item__label) {
  color: var(--aiops-text-secondary);
  font-size: 12px;
  font-weight: 700;
}

.asset-select {
  width: 320px;
}

.asset-select :deep(.el-select__wrapper),
.filter-form :deep(.el-input__wrapper),
.table-search :deep(.el-input__wrapper) {
  min-height: 34px;
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 14px;
  background: rgba(252, 253, 255, 0.96);
  box-shadow: none;
}

.asset-select :deep(.el-select__wrapper:hover),
.filter-form :deep(.el-input__wrapper:hover),
.table-search :deep(.el-input__wrapper:hover),
.asset-select :deep(.is-focused),
.filter-form :deep(.el-input__wrapper.is-focus),
.table-search :deep(.el-input__wrapper.is-focus) {
  border-color: rgba(76, 139, 245, 0.42);
  box-shadow: 0 0 0 3px rgba(76, 139, 245, 0.08);
}

.filter-form :deep(.el-segmented) {
  --el-segmented-item-selected-color: var(--aiops-brand);
  --el-segmented-item-selected-bg-color: rgba(76, 139, 245, 0.12);
  min-height: 34px;
  padding: 3px;
  border: 1px solid rgba(214, 223, 235, 0.85);
  border-radius: 14px;
  background: rgba(252, 253, 255, 0.9);
}

.filter-form :deep(.el-segmented__item) {
  border-radius: 11px;
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

.filter-actions :deep(.el-form-item__content) {
  gap: 8px;
}

.header-actions :deep(.el-button),
.filter-actions :deep(.el-button) {
  min-height: 32px;
  border-radius: 12px;
  font-weight: 600;
}

.primary-action {
  border: none;
  background: linear-gradient(180deg, #58c78e 0%, var(--aiops-success) 100%);
  color: #fff;
  box-shadow: 0 10px 22px rgba(63, 185, 127, 0.18);
}

.primary-action:hover,
.primary-action:focus {
  background: linear-gradient(180deg, #64d199 0%, #36aa72 100%);
  color: #fff;
}

.upload-action {
  border: none;
  background: linear-gradient(180deg, #4ebfc4 0%, #2ea3aa 100%);
  color: #fff;
  box-shadow: 0 10px 20px rgba(46, 163, 170, 0.16);
}

.upload-action:hover,
.upload-action:focus {
  background: linear-gradient(180deg, #5bcbd0 0%, #28949b 100%);
  color: #fff;
}

.upload-action.is-disabled,
.upload-action.is-disabled:hover,
.upload-action.is-disabled:focus {
  border: none;
  background: linear-gradient(180deg, #4ebfc4 0%, #2ea3aa 100%);
  color: #fff;
  opacity: 0.62;
  box-shadow: none;
}

.warning-action {
  border: none;
  background: linear-gradient(180deg, #f7c66e 0%, var(--aiops-warning) 100%);
  color: #fff;
}

.warning-action:hover,
.warning-action:focus {
  background: linear-gradient(180deg, #facf82 0%, #e9a741 100%);
  color: #fff;
}

.export-action {
  border: none;
  background: linear-gradient(180deg, #5b96f6 0%, var(--aiops-brand) 100%);
  color: #fff;
  box-shadow: 0 10px 20px rgba(76, 139, 245, 0.16);
}

.export-action:hover,
.export-action:focus {
  background: linear-gradient(180deg, #6aa0f7 0%, var(--aiops-brand-active) 100%);
  color: #fff;
}

.export-action.is-disabled,
.export-action.is-disabled:hover,
.export-action.is-disabled:focus {
  border: none;
  background: linear-gradient(180deg, #5b96f6 0%, var(--aiops-brand) 100%);
  color: #fff;
  opacity: 0.62;
  box-shadow: none;
}

.history-action {
  border: none;
  background: linear-gradient(180deg, #6f9ff0 0%, #5686dc 100%);
  color: #fff;
  box-shadow: 0 10px 20px rgba(86, 134, 220, 0.15);
}

.history-action:hover,
.history-action:focus {
  background: linear-gradient(180deg, #7aabf5 0%, #4f7fd0 100%);
  color: #fff;
}

.history-action.is-disabled,
.history-action.is-disabled:hover,
.history-action.is-disabled:focus {
  border: none;
  background: linear-gradient(180deg, #6f9ff0 0%, #5686dc 100%);
  color: #fff;
  opacity: 0.62;
  box-shadow: none;
}

.refresh-action {
  border: none;
  background: linear-gradient(180deg, #f7c66e 0%, var(--aiops-warning) 100%);
  color: #fff;
  box-shadow: 0 10px 20px rgba(243, 178, 75, 0.18);
}

.refresh-action:hover,
.refresh-action:focus {
  background: linear-gradient(180deg, #facf82 0%, #e9a741 100%);
  color: #fff;
}

.refresh-action.is-disabled,
.refresh-action.is-disabled:hover,
.refresh-action.is-disabled:focus {
  border: none;
  background: linear-gradient(180deg, #f7c66e 0%, var(--aiops-warning) 100%);
  color: #fff;
  opacity: 0.62;
  box-shadow: none;
}

.secondary-action {
  border: 1px solid rgba(214, 223, 235, 0.95);
  background: rgba(255, 255, 255, 0.92);
  color: var(--aiops-text-secondary);
}

.secondary-action:hover,
.secondary-action:focus {
  border-color: rgba(76, 139, 245, 0.36);
  color: var(--aiops-brand);
}

.danger-action {
  min-height: 32px;
  border: none;
  border-radius: 12px;
  background: linear-gradient(180deg, #f1848a 0%, var(--aiops-danger) 100%);
  color: #fff;
  font-weight: 600;
  box-shadow: 0 10px 20px rgba(239, 107, 115, 0.16);
}

.danger-action:hover,
.danger-action:focus {
  background: linear-gradient(180deg, #f39196 0%, #e35b64 100%);
  color: #fff;
}

.asset-option {
  display: grid;
  grid-template-columns: minmax(120px, 1fr) auto minmax(120px, 0.8fr) auto;
  align-items: center;
  gap: 10px;
}

.asset-option strong,
.asset-option span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.asset-option span {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.status-panel {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px 12px;
  border: 1px solid rgba(207, 217, 230, 0.72);
  border-radius: 16px;
  background:
    linear-gradient(135deg, rgba(255, 253, 248, 0.95) 0%, rgba(248, 251, 255, 0.92) 100%);
  box-shadow: 0 12px 34px rgba(17, 24, 39, 0.045);
}

.status-panel.status-running,
.status-panel.status-pending {
  border-color: rgba(76, 139, 245, 0.2);
}

.status-panel.status-success {
  border-color: rgba(63, 185, 127, 0.22);
}

.status-panel.status-failed {
  border-color: rgba(239, 107, 115, 0.24);
}

.status-topline {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: start;
  gap: 12px;
}

.status-main {
  display: flex;
  align-items: flex-start;
  min-width: 0;
  gap: 10px;
}

.status-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.status-main strong {
  display: block;
  color: var(--aiops-text-primary);
  font-size: 14px;
  font-weight: 700;
}

.status-main p {
  margin: 3px 0 0;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.35;
}

.status-dot {
  width: 9px;
  height: 9px;
  flex-shrink: 0;
  margin-top: 5px;
  border-radius: 50%;
  background: var(--aiops-text-muted);
}

.status-dot.running,
.status-dot.pending {
  background: var(--aiops-brand);
  box-shadow: 0 0 0 5px rgba(76, 139, 245, 0.12);
}

.status-dot.success {
  background: var(--aiops-success);
  box-shadow: 0 0 0 5px rgba(63, 185, 127, 0.12);
}

.status-dot.failed {
  background: var(--aiops-danger);
  box-shadow: 0 0 0 5px rgba(239, 107, 115, 0.12);
}

.status-side {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 8px;
}

.task-id {
  max-width: 260px;
  overflow: hidden;
  color: var(--aiops-text-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-panel :deep(.el-progress__text) {
  color: var(--aiops-text-secondary);
  font-size: 12px !important;
  font-weight: 700;
}

.progress-row {
  display: grid;
  grid-template-columns: minmax(220px, 0.42fr) minmax(0, 1fr);
  align-items: center;
  gap: 10px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.35;
}

.progress-row span {
  overflow-wrap: anywhere;
}

.progress-error {
  color: var(--aiops-danger);
  font-weight: 700;
}

.overview-matrix {
  display: grid;
  grid-template-columns: minmax(220px, 1.4fr) repeat(9, minmax(88px, 1fr));
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.76);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.58);
}

.overview-item {
  min-width: 0;
  padding: 7px 10px;
  border-right: 1px solid rgba(223, 231, 242, 0.72);
  background: rgba(255, 255, 255, 0.34);
}

.overview-item:last-child {
  border-right: none;
}

.overview-item span {
  display: block;
  margin-bottom: 4px;
  color: var(--aiops-text-muted);
  font-size: 11px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.overview-item strong {
  display: block;
  min-width: 0;
  overflow: hidden;
  color: var(--aiops-text-primary);
  font-size: 12px;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.overview-item--metric {
  text-align: right;
}

.overview-item--metric span,
.overview-item--metric strong {
  text-align: right;
}

.warning-stack {
  display: grid;
  flex-shrink: 0;
  gap: 6px;
}

.warning-stack :deep(.el-alert) {
  border: 1px solid rgba(243, 178, 75, 0.24);
  border-radius: 16px;
  background: rgba(255, 248, 238, 0.82);
}

.table-panel {
  display: flex;
  min-height: 0;
  flex: 2 1 0;
  flex-direction: column;
  padding: 10px 14px 14px;
  overflow: hidden;
  border: 1px solid rgba(207, 217, 230, 0.72);
  border-radius: 18px;
  background:
    linear-gradient(135deg, rgba(255, 253, 248, 0.96) 0%, rgba(248, 251, 255, 0.92) 100%);
  box-shadow: 0 16px 42px rgba(17, 24, 39, 0.045);
}

.table-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  flex-shrink: 0;
  margin-bottom: 8px;
}

.table-head h2 {
  margin: 0;
  color: var(--aiops-text-primary);
  font-size: 16px;
  font-weight: 700;
}

.table-head span {
  display: block;
  margin-top: 3px;
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.table-search {
  width: 220px;
}

.table-shell {
  min-height: 0;
  flex: 1;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.88);
}

.slowlog-table {
  height: 100%;
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.slowlog-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.slowlog-table :deep(.el-table__header th) {
  background: rgba(247, 250, 255, 0.92);
  color: var(--aiops-text-muted);
  font-size: 11px;
  font-weight: 700;
}

.slowlog-table :deep(.el-table__cell) {
  padding: 5px 0;
}

.slowlog-table :deep(.cell) {
  line-height: 1.35;
}

.slowlog-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: rgba(76, 139, 245, 0.04);
}

.sql-cell {
  position: relative;
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 6px;
}

.db-name-badge {
  display: inline-block;
  max-width: 100%;
  overflow: hidden;
  color: #3567b7;
  font-size: 11px;
  font-weight: 700;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sql-snippet {
  display: block;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  color: #40516a;
  font-size: 11px;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sql-copy-button {
  width: 22px;
  height: 22px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border: none;
  border-radius: 8px;
  background: linear-gradient(180deg, #f7c66e 0%, var(--aiops-warning) 100%);
  color: #fff;
  cursor: pointer;
  opacity: 0;
  box-shadow: 0 8px 14px rgba(243, 178, 75, 0.18);
  transition: opacity 0.18s ease, color 0.18s ease, border-color 0.18s ease, background-color 0.18s ease;
}

.sql-copy-button:hover,
.sql-copy-button:focus {
  background: linear-gradient(180deg, #facf82 0%, #e9a741 100%);
  color: #fff;
}

.slowlog-table :deep(.el-table__row:hover) .sql-copy-button,
.sql-copy-button:focus {
  opacity: 1;
}

.digest-text {
  color: var(--aiops-text-muted);
}

.ai-suggestion-button {
  font-size: 12px;
  font-weight: 700;
}

.empty-cell {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.mono {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
}

.slowlog-page :deep(.el-tag) {
  border-radius: 999px;
  font-weight: 600;
}

:global(.slowlog-history-dialog),
:global(.slowlog-upload-dialog),
:global(.slowlog-ai-dialog) {
  --aiops-text-primary: #172133;
  --aiops-text-secondary: #5f6b7c;
  --aiops-text-muted: #8d97a8;
  --aiops-brand: #4c8bf5;
  --aiops-warning: #f3b24b;
  --aiops-danger: #ef6b73;
  border-radius: 18px;
}

:global(.slowlog-history-dialog .el-dialog__header),
:global(.slowlog-upload-dialog .el-dialog__header),
:global(.slowlog-ai-dialog .el-dialog__header) {
  padding: 14px 16px 10px;
  margin: 0;
  border-bottom: 1px solid rgba(223, 231, 242, 0.86);
}

:global(.slowlog-history-dialog .el-dialog__body),
:global(.slowlog-upload-dialog .el-dialog__body),
:global(.slowlog-ai-dialog .el-dialog__body) {
  padding: 12px 16px 14px;
}

:global(.slowlog-upload-dialog .el-dialog__footer) {
  padding: 10px 16px 14px;
  border-top: 1px solid rgba(223, 231, 242, 0.72);
}

.history-dialog-header,
.upload-dialog-header,
.ai-dialog-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  padding-right: 34px;
}

.history-dialog-header strong,
.upload-dialog-header strong,
.ai-dialog-header strong {
  display: block;
  color: var(--aiops-text-primary);
  font-size: 16px;
  line-height: 1.3;
}

.history-dialog-header span,
.upload-dialog-header span,
.ai-dialog-header span {
  display: block;
  max-width: 640px;
  margin-top: 3px;
  overflow: hidden;
  color: var(--aiops-text-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-suggestion-summary {
  margin-bottom: 10px;
  padding: 9px 10px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  border-radius: 12px;
  background: rgba(247, 250, 255, 0.78);
}

.ai-suggestion-summary span {
  display: block;
  margin-bottom: 4px;
  color: var(--aiops-text-muted);
  font-size: 11px;
  font-weight: 700;
}

.ai-suggestion-summary strong {
  display: block;
  max-height: 54px;
  overflow: auto;
  color: #40516a;
  font-size: 11px;
  line-height: 1.5;
  white-space: pre-wrap;
}

.ai-suggestion-sections {
  display: grid;
  gap: 8px;
}

.ai-suggestion-section {
  padding: 10px 12px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.82);
}

.ai-suggestion-section span {
  display: block;
  margin-bottom: 5px;
  color: #3567b7;
  font-size: 12px;
  font-weight: 800;
}

.ai-suggestion-section p {
  margin: 0;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.55;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.slowlog-upload-box {
  margin-bottom: 10px;
}

.slowlog-upload-box :deep(.el-upload) {
  width: 100%;
}

.slowlog-upload-box :deep(.el-upload-dragger) {
  width: 100%;
  padding: 18px 16px;
  border: 1px dashed rgba(76, 139, 245, 0.36);
  border-radius: 16px;
  background:
    linear-gradient(135deg, rgba(255, 253, 248, 0.94) 0%, rgba(248, 251, 255, 0.92) 100%);
}

.slowlog-upload-box :deep(.el-upload-dragger:hover) {
  border-color: rgba(46, 163, 170, 0.48);
  background: rgba(248, 252, 255, 0.96);
}

.slowlog-upload-box :deep(.el-icon--upload) {
  margin-bottom: 6px;
  color: #2ea3aa;
  font-size: 30px;
}

.slowlog-upload-box :deep(.el-upload__text) {
  color: var(--aiops-text-secondary);
  font-size: 13px;
  font-weight: 700;
}

.slowlog-upload-box :deep(.el-upload__tip) {
  margin-top: 6px;
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.upload-file-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
  padding: 8px 10px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.72);
}

.upload-file-row strong {
  display: block;
  max-width: 520px;
  overflow: hidden;
  color: var(--aiops-text-primary);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.upload-file-row span {
  display: block;
  margin-top: 2px;
  color: var(--aiops-text-muted);
  font-size: 11px;
}

.upload-progress-panel {
  margin-top: 10px;
  padding: 9px 10px 10px;
  border: 1px solid rgba(46, 163, 170, 0.18);
  border-radius: 12px;
  background: rgba(244, 253, 253, 0.76);
}

.upload-progress-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 6px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.upload-progress-title strong {
  color: var(--aiops-text-primary);
  font-size: 12px;
}

.upload-dialog-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}

.history-toolbar {
  display: grid;
  grid-template-columns: 120px minmax(180px, 1fr) 340px auto;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}

.history-status-select,
.history-keyword,
.history-time-range {
  width: 100%;
}

.history-toolbar :deep(.el-select__wrapper),
.history-toolbar :deep(.el-input__wrapper) {
  min-height: 32px;
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 12px;
  background: rgba(252, 253, 255, 0.96);
  box-shadow: none;
}

.history-toolbar :deep(.el-range-editor.el-input__wrapper) {
  padding: 0 10px;
}

.history-table-shell {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.9);
}

.history-table {
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.history-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.history-table :deep(.el-table__header th) {
  background: rgba(247, 250, 255, 0.94);
  color: var(--aiops-text-muted);
  font-size: 11px;
  font-weight: 700;
}

.history-table :deep(.el-table__cell) {
  padding: 6px 0;
}

.history-table :deep(.cell) {
  line-height: 1.35;
}

.history-pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 10px;
  color: var(--aiops-text-muted);
  font-size: 12px;
}

@media (max-width: 1280px) {
  .slowlog-header {
    flex-direction: column;
  }

  .header-actions {
    justify-content: flex-start;
  }

  .overview-matrix {
    grid-template-columns: repeat(5, minmax(0, 1fr));
  }

  .overview-item:nth-child(5n) {
    border-right: none;
  }
}

@media (max-width: 900px) {
  .slowlog-page {
    padding: 4px 6px 8px;
  }

  .slowlog-shell {
    max-height: none;
    overflow: visible;
    border-radius: 22px;
  }

  .slowlog-shell::before {
    border-radius: 22px 22px 0 0;
  }

  .filter-panel,
  .status-panel,
  .warning-stack,
  .table-panel {
    margin: 0 10px;
  }

  .asset-select,
  .table-search,
  .header-actions :deep(.el-button),
  .filter-actions :deep(.el-button) {
    width: 100%;
  }

  .filter-form,
  .filter-row,
  .header-actions,
  .table-head,
  .status-panel,
  .status-topline,
  .status-side {
    align-items: stretch;
    flex-direction: column;
  }

  .status-topline,
  .progress-row {
    display: flex;
  }

  .overview-matrix {
    grid-template-columns: 1fr;
  }

  .filter-form :deep(.el-form-item),
  .filter-form :deep(.el-form-item__content) {
    width: 100%;
  }

  .filter-form :deep(.el-segmented) {
    width: 100%;
  }

  .overview-item,
  .overview-item:nth-child(5n) {
    border-right: none;
    border-bottom: 1px solid rgba(223, 231, 242, 0.72);
  }

  .overview-item:last-child {
    border-bottom: none;
  }

  .overview-item--metric,
  .overview-item--metric span,
  .overview-item--metric strong {
    text-align: left;
  }

  .table-panel {
    min-height: 520px;
  }
}
</style>
