<template>
  <div class="log-page mysql-slow-report-page">
    <section class="page-card">
      <header class="page-header">
        <div>
          <h2>数据库慢日志</h2>
        </div>
      </header>

      <div class="report-content">
        <section class="filter-card">
          <el-form inline class="filter-form" @submit.prevent>
            <el-form-item label="数据库实例">
              <el-select
                v-model="selectedKey"
                class="instance-select"
                filterable
                placeholder="请选择已绑定慢日志采集的 MySQL 实例"
                :loading="instanceLoading"
                @change="handleSourceChange"
              >
                <el-option
                  v-for="item in instances"
                  :key="instanceKey(item)"
                  :label="instanceLabel(item)"
                  :value="instanceKey(item)"
                  :disabled="!item.ready"
                >
                  <div class="instance-option">
                    <strong>{{ item.instanceName || item.assetName }}</strong>
                    <span>{{ endpointText(item) }}</span>
                    <el-tag size="small" effect="plain">{{ item.storeName }}</el-tag>
                    <el-tag v-if="!item.ready" size="small" type="warning">不可生成</el-tag>
                  </div>
                </el-option>
              </el-select>
            </el-form-item>
            <el-form-item label="报告日期">
              <el-date-picker
                v-model="reportDate"
                type="date"
                value-format="YYYY-MM-DD"
                format="YYYY-MM-DD"
                :disabled-date="disableFutureDate"
                @change="loadExistingReport"
              />
            </el-form-item>
            <el-form-item class="filter-actions">
              <el-button size="small" type="primary" plain :icon="Search" :loading="reportLoading" :disabled="!canQuery" @click="loadExistingReport">查询日报</el-button>
              <el-button size="small" type="primary" :icon="DataAnalysis" :loading="generateLoading" :disabled="!canQuery || isReportActive" @click="generateReport">
                {{ reportReady ? '重新生成' : (isReportActive ? '分析中' : '生成日报') }}
              </el-button>
              <el-tooltip :content="reportReady ? '将当前日报明细导出为 Excel' : '请先生成日报后再导出'" placement="top">
                <el-button size="small" class="export-button" type="primary" plain :icon="Download" :disabled="!reportReady" :loading="exportLoading" @click="exportReport">导出 Excel</el-button>
              </el-tooltip>
              <el-tooltip content="刷新实例列表" placement="top">
                <el-button size="small" :icon="Refresh" plain circle :loading="instanceLoading" aria-label="刷新实例" @click="loadInstances" />
              </el-tooltip>
            </el-form-item>
          </el-form>
          <div v-if="selectedInstance" class="source-context">
            <span><b>日志库</b>{{ selectedInstance.storeName || '-' }}</span>
            <span><b>采集任务</b>{{ selectedInstance.collectTaskName || '-' }}</span>
            <span v-if="collectedFilesLoading"><b>采集文件</b>查询中…</span>
            <el-popover
              v-else-if="collectedFileItems.length"
              placement="bottom-start"
              :width="460"
              trigger="hover"
              :show-after="150"
              popper-class="slow-report-collected-files-popper"
            >
              <template #reference>
                <span class="collected-files">
                  <b>采集文件</b>
                  <code v-for="file in collectedFileItems.slice(0, 3)" :key="file.path">{{ file.path }}</code>
                  <code v-if="collectedFileItems.length > 3">+{{ collectedFileItems.length - 3 }} 个文件</code>
                  <el-icon class="collected-files-hint"><InfoFilled /></el-icon>
                </span>
              </template>
              <div class="collected-files-panel">
                <header>
                  <strong>采集文件（近 7 天）</strong>
                  <span>{{ collectedFileItems.length }} 个</span>
                </header>
                <div class="collected-files-list">
                  <div v-for="(file, index) in collectedFileItems" :key="file.path" class="collected-files-item">
                    <div class="collected-files-top">
                      <em>{{ index + 1 }}</em>
                      <code>{{ file.path }}</code>
                    </div>
                    <div class="collected-files-meta">
                      <span class="collected-files-count">慢日志条数 <b>{{ formatInteger(file.count) }}</b></span>
                      <span v-if="file.lastSeen" class="collected-files-seen">最近写入 <b>{{ formatDateTime(file.lastSeen) }}</b></span>
                    </div>
                  </div>
                </div>
              </div>
            </el-popover>
            <span v-else><b>日志路径</b><code>{{ selectedInstance.slowLogPath || '按采集任务和主机匹配' }}</code></span>
          </div>
        </section>

        <el-alert
          v-if="selectedInstance?.warning"
          class="context-alert"
          :title="selectedInstance.warning"
          type="warning"
          show-icon
          :closable="false"
        />

        <section v-if="reportLoading && !report" class="loading-card" v-loading="true"></section>

        <template v-else-if="reportVisible">
          <section class="report-overview">
            <div>
              <span class="report-date">{{ report.reportDate }}</span>
              <h3>{{ report.instance?.instanceName || report.instance?.assetName || 'MySQL 实例' }} 慢日志日报</h3>
              <p>{{ reportRangeText }} · 数据来源 {{ report.instance?.storeName || '-' }}</p>
            </div>
            <div class="report-status">
              <div class="report-status-head">
                <el-tag :type="reportStatusTagType" effect="light" round>
                  {{ reportStatusLabel }}
                </el-tag>
                <el-popover
                  v-if="report.warnings?.length"
                  placement="bottom-end"
                  :width="400"
                  trigger="hover"
                  :show-after="150"
                  popper-class="slow-report-notification-popper"
                >
                  <template #reference>
                    <el-button class="report-notification-button" text circle aria-label="查看报告消息">
                      <el-icon><Bell /></el-icon>
                      <span class="report-notification-count">{{ Math.min(report.warnings.length, 99) }}</span>
                    </el-button>
                  </template>
                  <div class="report-notification-panel">
                    <header>
                      <strong>报告消息</strong>
                      <span>{{ report.warnings.length }} 条</span>
                    </header>
                    <div class="report-notification-list">
                      <div v-for="warning in report.warnings" :key="warning" class="report-notification-item">
                        <el-icon><WarningFilled /></el-icon>
                        <span>{{ warning }}</span>
                      </div>
                    </div>
                  </div>
                </el-popover>
              </div>
              <span v-if="isReportActive">{{ report.message || '后台分析中' }}</span>
              <span v-else>生成于 {{ formatDateTime(report.generatedAt) }}</span>
              <el-button link type="primary" @click="openRawLogs">查看原始慢日志</el-button>
              <el-button v-if="isReportActive" link type="danger" :loading="cancelLoading" :disabled="report.cancelRequested" @click="cancelReport">
                {{ report.cancelRequested ? '取消中' : '取消任务' }}
              </el-button>
            </div>
            <div v-if="report.dataQuality?.rawEntries || comparisonVisible" class="overview-meta">
              <template v-if="report.dataQuality?.rawEntries">
                <span><b>原始记录</b>{{ formatInteger(report.dataQuality.rawEntries) }}</span>
                <span><b>有效慢 SQL</b>{{ formatInteger(report.dataQuality.validEntries) }}</span>
                <span class="is-business"><b>业务 SQL</b>{{ formatInteger(report.dataQuality.businessEntries) }}</span>
                <span class="is-monitoring"><b>监控/系统</b>{{ formatInteger(report.dataQuality.monitoringEntries) }}</span>
                <span class="is-invalid"><b>无效片段</b>{{ formatInteger(report.dataQuality.invalidEntries) }}</span>
                <span><b>解析成功率</b>{{ formatPercent(report.dataQuality.parseSuccessRate) }}</span>
              </template>
              <template v-if="comparisonVisible">
                <span class="comparison-title">较 {{ report.comparison.previousDate }}</span>
                <template v-if="report.comparison.available">
                  <span class="is-new"><b>新增</b>{{ report.comparison.newCount }}</span>
                  <span class="is-worse"><b>恶化</b>{{ report.comparison.worsenedCount }}</span>
                  <span class="is-better"><b>好转</b>{{ report.comparison.improvedCount }}</span>
                  <span><b>持平</b>{{ report.comparison.stableCount }}</span>
                  <el-popover
                    v-if="report.comparison.goneTopSql?.length"
                    placement="bottom"
                    :width="520"
                    trigger="click"
                    popper-class="slow-report-gone-sql-popper"
                  >
                    <template #reference>
                      <el-button link type="primary" size="small">消失 Top SQL {{ report.comparison.goneTopSql.length }} 条</el-button>
                    </template>
                    <div class="gone-sql-panel">
                      <header>
                        <strong>前一日 Top SQL 今日未再进入 Top 10</strong>
                        <span>{{ report.comparison.goneTopSql.length }} 条</span>
                      </header>
                      <div class="gone-sql-list">
                        <div v-for="(item, index) in report.comparison.goneTopSql" :key="item.digest" class="gone-sql-item">
                          <div class="gone-sql-top">
                            <em>{{ index + 1 }}</em>
                            <code :title="item.fingerprint">{{ item.fingerprint }}</code>
                          </div>
                          <div class="gone-sql-meta">
                            <span class="gone-sql-db">{{ item.database || '无默认库' }}</span>
                            <span class="gone-sql-user">{{ item.user || '-' }}</span>
                            <span class="gone-sql-metric gone-sql-count">前一日 <b>{{ formatInteger(item.count) }}</b> 次</span>
                            <span class="gone-sql-metric gone-sql-p95">P95 <b>{{ formatSeconds(item.queryTimeP95) }}</b></span>
                          </div>
                        </div>
                      </div>
                    </div>
                  </el-popover>
                </template>
                <span v-else class="comparison-missing">{{ comparisonMissingText }}</span>
              </template>
            </div>
          </section>

          <section v-if="isReportActive" class="report-progress-card">
            <div class="progress-head">
              <strong>大模型逐条分析进度</strong>
              <span>{{ formatInteger(report.analysisCompleted) }} / {{ formatInteger(report.analysisTotal) }} 条</span>
            </div>
            <el-progress :percentage="report.progress || 0" :status="report.analysisFailed ? 'warning' : undefined" />
            <p>{{ report.message || '任务排队中，完成一条后立即保存结果' }}</p>
          </section>

          <section class="metric-grid">
            <article v-for="card in summaryCards" :key="card.key" class="metric-card" :class="card.tone">
              <span>{{ card.label }}</span>
              <strong>{{ card.value }}</strong>
              <small>
                <i v-if="card.delta" class="metric-delta" :class="card.delta.cls">{{ card.delta.text }}</i>
                <span class="metric-hint">{{ card.hint }}</span>
              </small>
            </article>
          </section>

          <section class="chart-grid">
            <article class="panel-card trend-card">
              <header class="panel-head">
                <div>
                  <h3>小时趋势</h3>
                  <p>慢 SQL 数量与平均/P95 执行耗时</p>
                </div>
              </header>
              <div ref="trendChartRef" class="chart"></div>
            </article>

            <article class="panel-card distribution-card">
              <header class="panel-head">
                <div>
                  <h3>数据库分布</h3>
                  <p>按库统计慢 SQL 数量 Top 10</p>
                </div>
              </header>
              <div ref="distributionChartRef" class="chart"></div>
            </article>
          </section>

          <section class="dimension-grid">
            <article class="panel-card dimension-card">
              <header class="panel-head"><h3>SQL 类型</h3></header>
              <div class="dimension-list">
                <div v-for="item in report.operationDistribution" :key="item.name" class="dimension-row">
                  <span>{{ item.name }}</span>
                  <div><i :style="{ width: `${Math.min(item.ratio, 100)}%` }"></i></div>
                  <strong>{{ formatInteger(item.count) }}</strong>
                  <em>{{ formatPercent(item.ratio) }}</em>
                </div>
                <el-empty v-if="!report.operationDistribution?.length" description="暂无类型分布" :image-size="48" />
              </div>
            </article>
            <article class="panel-card dimension-card">
              <header class="panel-head"><h3>执行用户</h3></header>
              <div class="dimension-list">
                <div v-for="item in report.userDistribution?.slice(0, 8)" :key="item.name" class="dimension-row">
                  <span>{{ item.name }}</span>
                  <div><i :style="{ width: `${Math.min(item.ratio, 100)}%` }"></i></div>
                  <strong>{{ formatInteger(item.count) }}</strong>
                  <em>{{ formatPercent(item.ratio) }}</em>
                </div>
                <el-empty v-if="!report.userDistribution?.length" description="暂无用户分布" :image-size="48" />
              </div>
            </article>
          </section>

          <section class="panel-card top-sql-card">
            <header class="panel-head top-sql-head">
              <div>
                <h3>业务慢 SQL Top 10</h3>
                <p>全量慢 SQL 分析后按耗时、频次、P95、扫描比等综合评分选出，相同 SQL 参数归一化为同一指纹</p>
              </div>
              <div class="top-sql-tools">
                <el-input v-model.trim="tableKeyword" class="table-search" clearable :prefix-icon="Search" placeholder="搜索 SQL、数据库或用户" />
              </div>
            </header>
            <el-table :data="filteredTopSQL" row-key="digest" class="top-sql-table" empty-text="当日无慢 SQL 数据">
              <el-table-column type="index" label="#" width="54" />
              <el-table-column label="风险" width="76" align="center">
                <template #default="{ row }">
                  <el-tag :type="riskTagType(row.riskLevel)" size="small" effect="light">{{ riskText(row.riskLevel) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="数据库" min-width="120" show-overflow-tooltip>
                <template #default="{ row }">{{ row.database || '无默认库' }}</template>
              </el-table-column>
              <el-table-column prop="user" label="用户" min-width="105" show-overflow-tooltip />
              <el-table-column prop="operation" label="类型" width="88" />
              <el-table-column label="SQL 指纹 / 样例" min-width="360">
                <template #default="{ row }">
                  <div class="sql-content">
                    <code>{{ row.fingerprint || '-' }}</code>
                    <span>{{ row.sampleSql || '-' }}</span>
                  </div>
                </template>
              </el-table-column>
              <el-table-column prop="count" label="次数" width="88" align="right" sortable>
                <template #default="{ row }">{{ formatInteger(row.count) }}</template>
              </el-table-column>
              <el-table-column label="较前一日" width="92" align="center">
                <template #default="{ row }">
                  <el-tooltip
                    v-if="row.comparison"
                    :content="comparisonTooltip(row)"
                    placement="top"
                    effect="light"
                    :show-after="250"
                    :disabled="!comparisonTooltip(row)"
                    popper-class="optimization-audit-tooltip"
                  >
                    <el-tag :type="comparisonTagType(row.comparison.status)" size="small" effect="light">{{ comparisonText(row.comparison.status) }}</el-tag>
                  </el-tooltip>
                  <span v-else class="comparison-none">-</span>
                </template>
              </el-table-column>
              <el-table-column prop="queryTimeAvg" label="平均耗时" width="104" align="right" sortable>
                <template #default="{ row }">{{ formatSeconds(row.queryTimeAvg) }}</template>
              </el-table-column>
              <el-table-column prop="queryTimeP95" label="P95" width="96" align="right" sortable>
                <template #default="{ row }">{{ formatSeconds(row.queryTimeP95) }}</template>
              </el-table-column>
              <el-table-column prop="queryTimeMax" label="最大耗时" width="104" align="right" sortable>
                <template #default="{ row }">{{ formatSeconds(row.queryTimeMax) }}</template>
              </el-table-column>
              <el-table-column prop="queryTimeTotal" label="总耗时" width="104" align="right" sortable>
                <template #default="{ row }">{{ formatSeconds(row.queryTimeTotal) }}</template>
              </el-table-column>
              <el-table-column prop="rowsExaminedTotal" label="扫描行数" width="112" align="right" sortable>
                <template #default="{ row }">{{ formatCompact(row.rowsExaminedTotal) }}</template>
              </el-table-column>
              <el-table-column label="优化建议" min-width="430">
                <template #default="{ row }">
                  <el-tooltip
                    :content="optimizationAdviceTitle(row)"
                    placement="top"
                    effect="light"
                    :show-after="250"
                    :disabled="!optimizationAdviceTitle(row)"
                    popper-class="optimization-audit-tooltip"
                  >
                    <div class="optimization-advice">
                      <div class="optimization-meta">
                        <el-tag v-if="row.optimizationConfidence" :type="optimizationConfidenceType(row.optimizationConfidence)" size="small" effect="light">
                          {{ optimizationConfidenceText(row.optimizationConfidence) }}
                        </el-tag>
                        <span>{{ optimizationSourceText(row.optimizationSource) }}</span>
                        <el-tag v-if="row.analysisCacheHit" type="info" size="small" effect="plain">缓存</el-tag>
                      </div>
                      <div v-if="row.optimizationSections" class="optimization-sections">
                        <p><b>慢查询原因</b>{{ row.optimizationSections.slowReason }}</p>
                        <p v-if="row.optimizationSections.schemaOptimization"><b>表结构优化</b>{{ row.optimizationSections.schemaOptimization }}</p>
                        <p v-if="row.optimizationSections.indexOptimization"><b>索引优化</b>{{ row.optimizationSections.indexOptimization }}</p>
                        <p v-if="row.optimizationSections.sqlOptimization"><b>SQL 优化</b>{{ row.optimizationSections.sqlOptimization }}</p>
                        <p v-if="row.optimizationSections.expectedEffect"><b>预期效果</b>{{ row.optimizationSections.expectedEffect }}</p>
                      </div>
                      <p v-else-if="row.analysisStatus === 'pending' || row.analysisStatus === 'running'" class="analysis-pending">正在等待模型分析</p>
                      <p v-else class="optimization-error">{{ row.optimizationError || '大模型暂未返回优化建议' }}</p>
                      <small v-if="row.optimizationCitations?.length">引用：{{ row.optimizationCitations[0] }}</small>
                    </div>
                  </el-tooltip>
                </template>
              </el-table-column>
            </el-table>
          </section>
        </template>

        <section v-else-if="report" class="report-state-card">
          <el-result :icon="reportStatusIcon" :title="reportStatusTitle" :sub-title="reportStatusMessage">
            <template #extra>
              <el-button :icon="Refresh" :loading="reportLoading" @click="loadExistingReport">刷新状态</el-button>
              <el-button v-if="isReportActive" type="danger" plain :loading="cancelLoading" :disabled="report.cancelRequested" @click="cancelReport">
                {{ report.cancelRequested ? '取消中' : '取消任务' }}
              </el-button>
              <el-button v-else type="primary" :icon="DataAnalysis" :loading="generateLoading" @click="generateReport">重新生成</el-button>
            </template>
          </el-result>
        </section>

        <section v-else class="empty-card">
          <el-empty :description="emptyDescription" :image-size="96">
            <el-button v-if="canQuery" type="primary" :icon="DataAnalysis" :loading="generateLoading" @click="generateReport">生成该日慢日志报告</el-button>
          </el-empty>
        </section>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import * as echarts from 'echarts'
import { Bell, DataAnalysis, Download, InfoFilled, Refresh, Search, WarningFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import logtailApi from '@/api/logtail'

const route = useRoute()
const router = useRouter()

const instances = ref([])
const selectedKey = ref('')
const reportDate = ref(defaultReportDate())
const report = ref(null)
const instanceLoading = ref(false)
const reportLoading = ref(false)
const generateLoading = ref(false)
const cancelLoading = ref(false)
const exportLoading = ref(false)
const collectedFiles = ref(null)
const collectedFilesLoading = ref(false)
let collectedFilesSeq = 0
const tableKeyword = ref('')
const TOP_SQL_LIMIT = 10
const trendChartRef = ref(null)
const distributionChartRef = ref(null)
let trendChart = null
let distributionChart = null
let reportPollTimer = null

const selectedInstance = computed(() => instances.value.find(item => instanceKey(item) === selectedKey.value) || null)
const collectedFileItems = computed(() => collectedFiles.value?.files || [])
const canQuery = computed(() => Boolean(selectedInstance.value?.bindingKey && selectedInstance.value?.collectTaskId && reportDate.value))
const reportReady = computed(() => ['success', 'partial'].includes(report.value?.status))
const isReportActive = computed(() => ['pending', 'running'].includes(report.value?.status))
const reportVisible = computed(() => reportReady.value || Boolean(report.value?.topSql?.length))
const reportStatusLabel = computed(() => {
  if (report.value?.status === 'partial') return '部分完成'
  if (report.value?.status === 'success') return '生成完成'
  if (report.value?.status === 'failed') return '生成失败'
  if (report.value?.status === 'cancelled') return '已取消'
  if (report.value?.status === 'running') return '分析中'
  return '排队中'
})
const reportStatusTagType = computed(() => {
  if (report.value?.status === 'success') return 'success'
  if (report.value?.status === 'partial') return 'warning'
  if (report.value?.status === 'failed') return 'danger'
  if (report.value?.status === 'cancelled') return 'info'
  return 'warning'
})
const reportStatusIcon = computed(() => report.value?.status === 'failed' ? 'error' : 'warning')
const reportStatusTitle = computed(() => {
  if (report.value?.status === 'failed') return '慢日志日报生成失败'
  if (report.value?.status === 'cancelled') return '慢日志分析任务已取消'
  if (report.value?.status === 'running') return '慢日志日报生成中'
  return '慢日志日报等待生成'
})
const reportStatusMessage = computed(() => report.value?.error || report.value?.message || '日报暂未生成完成')
const summary = computed(() => report.value?.summary || {})

const comparisonVisible = computed(() => Boolean(report.value?.comparison?.previousDate))
const comparison = computed(() => report.value?.comparison || {})
const comparisonAvailable = computed(() => comparisonVisible.value && Boolean(comparison.value.available))

const summaryCards = computed(() => {
  const c = comparison.value
  const delta = (value, format) => (comparisonAvailable.value && value !== null && value !== undefined
    ? { text: format(value), cls: deltaClass(value) }
    : null)
  return [
    { key: 'total', label: '业务慢 SQL 总次数', value: formatInteger(summary.value.totalEntries), hint: `唯一指纹 ${formatInteger(summary.value.uniqueSql)}`, delta: delta(c.totalEntriesChange, formatSignedInteger), tone: 'tone-blue' },
    { key: 'totalTime', label: '累计执行耗时', value: formatSeconds(summary.value.totalQueryTime), hint: `平均 ${formatSeconds(summary.value.avgQueryTime)}`, delta: delta(c.totalQueryTimeChange, formatSignedSeconds), tone: 'tone-cyan' },
    { key: 'p95', label: 'P95 执行耗时', value: formatSeconds(summary.value.p95QueryTime), hint: `最大 ${formatSeconds(summary.value.maxQueryTime)}`, delta: delta(c.p95QueryTimeChange, formatSignedSeconds), tone: 'tone-orange' },
    { key: 'gt10', label: '耗时超过 10s', value: formatInteger(summary.value.greaterThan10sCount), hint: `超过 20s：${formatInteger(summary.value.greaterThan20sCount)}`, tone: 'tone-red' },
    { key: 'lock', label: '累计锁等待', value: formatSeconds(summary.value.totalLockTime), hint: `锁等待 > 1s：${formatInteger(summary.value.lockWaitCount)}`, tone: 'tone-purple' },
    { key: 'rows', label: '扫描总行数', value: formatCompact(summary.value.totalRowsExamined), hint: `扫描/返回比 ${formatNumber(summary.value.rowsExaminedSentRatio)}`, tone: 'tone-green' }
  ]
})

const comparisonMissingText = computed(() => {
  const comparison = report.value?.comparison
  if (!comparison) return ''
  if (comparison.previousStatus === 'failed' || comparison.previousStatus === 'cancelled') {
    return `前一日日报${comparison.previousStatus === 'failed' ? '生成失败' : '已取消'}，无法进行跨日对比`
  }
  if (comparison.previousStatus === 'pending' || comparison.previousStatus === 'running') {
    return '前一日日报仍在生成中，暂无法进行跨日对比'
  }
  return '前一日日报尚未生成，无法进行跨日对比'
})

const filteredTopSQL = computed(() => {
  const keyword = tableKeyword.value.toLowerCase()
  const rows = (report.value?.topSql || []).slice(0, TOP_SQL_LIMIT)
  if (!keyword) return rows
  return rows.filter(row => [
    row.fingerprint,
    row.sampleSql,
    row.database,
    row.user,
    row.operation,
    row.optimizationAdvice,
    row.optimizationError,
    ...(row.optimizationEvidence || []),
    ...(row.optimizationCitations || []),
    ...(row.optimizationLimitations || [])
  ]
    .some(value => String(value || '').toLowerCase().includes(keyword)))
})

const reportRangeText = computed(() => {
  if (!report.value) return '-'
  return `${formatDateTime(report.value.start)} 至 ${formatDateTime(report.value.end)}`
})

const emptyDescription = computed(() => {
  if (instanceLoading.value) return '正在加载数据库实例'
  if (!instances.value.length) return '暂无已绑定 db_slow_log 采集任务的 MySQL 实例'
  if (!selectedInstance.value) return '请选择数据库实例'
  return `${reportDate.value} 暂无日报快照`
})

function defaultReportDate() {
  const date = new Date()
  date.setDate(date.getDate() - 1)
  return formatDate(date)
}

function formatDate(date) {
  const pad = value => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

function disableFutureDate(date) {
  const today = new Date()
  today.setHours(23, 59, 59, 999)
  return date.getTime() > today.getTime()
}

function instanceKey(item) {
  return item?.bindingKey || `${item?.assetId || 0}:${item?.collectTaskId || 0}`
}

function endpointText(item) {
  if (item?.endpoint) return item.endpoint
  if (item?.hostIp && item?.port) return `${item.hostIp}:${item.port}`
  return item?.hostIp || '-'
}

function instanceLabel(item) {
  return `${item.instanceName || item.assetName || `实例 #${item.assetId}`} · ${endpointText(item)} · ${item.storeName || '未绑定日志库'}`
}

const extractData = response => response?.data?.data ?? response?.data ?? response

async function loadInstances() {
  instanceLoading.value = true
  try {
    const response = await logtailApi.getMySQLSlowLogInstances()
    const data = extractData(response)
    instances.value = data?.items || data?.list || (Array.isArray(data) ? data : [])
    const routeAssetID = Number(route.query.assetId || 0)
    const routeTaskID = Number(route.query.collectTaskId || 0)
    const routed = instances.value.find(item => item.assetId === routeAssetID && (!routeTaskID || item.collectTaskId === routeTaskID))
    const current = instances.value.find(item => instanceKey(item) === selectedKey.value)
    const fallback = instances.value.find(item => item.ready) || instances.value[0]
    selectedKey.value = instanceKey(routed || current || fallback)
    syncRouteQuery()
    loadCollectedFiles()
    if (selectedInstance.value) await loadExistingReport(false)
  } catch (error) {
    console.error('加载 MySQL 慢日志实例失败:', error)
    instances.value = []
    report.value = null
    ElMessage.error('加载数据库实例失败')
  } finally {
    instanceLoading.value = false
  }
}

function reportParams() {
  return {
    bindingKey: selectedInstance.value?.bindingKey,
    assetId: selectedInstance.value?.assetId,
    collectTaskId: selectedInstance.value?.collectTaskId,
    reportDate: reportDate.value
  }
}

async function loadExistingReport(showMessage = true) {
  if (!canQuery.value) {
    report.value = null
    return
  }
  reportLoading.value = true
  stopReportPolling()
  try {
    const response = await logtailApi.getMySQLSlowDailyReport(reportParams())
    const data = extractData(response)
    report.value = data?.exists ? data.report : null
    tableKeyword.value = ''
    await renderCharts()
    if (isReportActive.value) startReportPolling()
    if (showMessage && !report.value) ElMessage.info('该日期尚未生成慢日志日报')
  } catch (error) {
    console.error('查询 MySQL 慢日志日报失败:', error)
    report.value = null
    ElMessage.error('查询慢日志日报失败')
  } finally {
    reportLoading.value = false
  }
}

function syncRouteQuery() {
  const instance = selectedInstance.value
  if (!instance) return
  router.replace({
    path: route.path,
    query: {
      ...route.query,
      assetId: instance.assetId,
      collectTaskId: instance.collectTaskId
    }
  }).catch(() => {})
}

async function handleSourceChange() {
  syncRouteQuery()
  report.value = null
  stopReportPolling()
  loadCollectedFiles()
  await loadExistingReport(false)
}

async function loadCollectedFiles() {
  collectedFilesSeq += 1
  const seq = collectedFilesSeq
  const bindingKey = selectedInstance.value?.bindingKey
  collectedFiles.value = null
  if (!bindingKey) {
    collectedFilesLoading.value = false
    return
  }
  collectedFilesLoading.value = true
  try {
    const response = await logtailApi.getMySQLSlowCollectedFiles({ bindingKey, days: 7 })
    if (seq === collectedFilesSeq) collectedFiles.value = extractData(response)
  } catch (error) {
    console.error('查询慢日志采集文件失败:', error)
  } finally {
    if (seq === collectedFilesSeq) collectedFilesLoading.value = false
  }
}

async function generateReport() {
  if (!canQuery.value) return
  const force = reportReady.value
  if (force) {
    try {
      await ElMessageBox.confirm(`确认重新生成 ${reportDate.value} 的慢日志日报吗？`, '重新生成日报', {
        confirmButtonText: '重新生成',
        cancelButtonText: '取消',
        type: 'warning'
      })
    } catch {
      return
    }
  }
  generateLoading.value = true
  try {
    const response = await logtailApi.generateMySQLSlowDailyReport({ ...reportParams(), force })
    report.value = extractData(response)
    tableKeyword.value = ''
    await renderCharts()
    if (isReportActive.value) {
      startReportPolling()
      ElMessage.success('任务已创建，后台正在逐条分析优化建议')
    } else {
      ElMessage.success('慢日志日报生成完成')
    }
  } catch (error) {
    console.error('生成 MySQL 慢日志日报失败:', error)
    ElMessage.error(error?.response?.data?.message || '生成慢日志日报失败')
    await loadExistingReport(false)
  } finally {
    generateLoading.value = false
  }
}

function stopReportPolling() {
  if (reportPollTimer) {
    clearTimeout(reportPollTimer)
    reportPollTimer = null
  }
}

function startReportPolling() {
  stopReportPolling()
  const poll = async () => {
    if (!isReportActive.value || !canQuery.value) {
      reportPollTimer = null
      return
    }
    try {
      const response = await logtailApi.getMySQLSlowDailyReport(reportParams())
      const data = extractData(response)
      if (data?.exists) {
        report.value = data.report
        await renderCharts()
      }
    } catch (error) {
      console.error('轮询 MySQL 慢日志日报状态失败:', error)
    }
    if (isReportActive.value) {
      reportPollTimer = setTimeout(poll, 2000)
    } else {
      reportPollTimer = null
      if (report.value?.status === 'success') ElMessage.success('慢日志日报生成完成')
      else if (report.value?.status === 'partial') ElMessage.warning('慢日志日报已完成，但部分优化建议失败')
    }
  }
  reportPollTimer = setTimeout(poll, 500)
}

async function cancelReport() {
  if (!report.value?.id || !isReportActive.value) return
  cancelLoading.value = true
  try {
    const response = await logtailApi.cancelMySQLSlowDailyReport(report.value.id)
    const data = extractData(response)
    if (data) report.value = data
    ElMessage.info('已请求取消，正在等待当前模型调用结束')
    startReportPolling()
  } catch (error) {
    ElMessage.error(error?.response?.data?.message || '取消分析任务失败')
  } finally {
    cancelLoading.value = false
  }
}

async function exportReport() {
  if (!reportReady.value) return
  exportLoading.value = true
  try {
    const response = await logtailApi.exportMySQLSlowDailyReport(reportParams())
    const blob = response?.data instanceof Blob ? response.data : new Blob([response?.data || ''], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `mysql_slow_report_${reportDate.value}_${selectedInstance.value.assetId || selectedInstance.value.hostId}.xlsx`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch (error) {
    console.error('导出 MySQL 慢日志日报失败:', error)
    ElMessage.error('导出慢日志日报失败')
  } finally {
    exportLoading.value = false
  }
}

function openRawLogs() {
  const instance = report.value?.instance
  if (!instance) return
  const queryParts = ['log_type:db_slow_log']
  if (instance.collectTaskCode) queryParts.push(`collect_task_code:"${escapeQuery(instance.collectTaskCode)}"`)
  if (instance.hostId) queryParts.push(`host_id:"${instance.hostId}"`)
  if (instance.slowLogPath) queryParts.push(`file_path:"${escapeQuery(instance.slowLogPath)}"`)
  router.push({
    path: '/logs/query',
    query: {
      projectId: instance.projectId,
      storeId: instance.storeId,
      q: queryParts.join(' '),
      start: localDateTime(report.value.start),
      end: localDateTime(report.value.end)
    }
  })
}

function escapeQuery(value) {
  return String(value || '').replace(/\\/g, '\\\\').replace(/"/g, '\\"')
}

function localDateTime(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value || '')
  const pad = number => String(number).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

async function renderCharts() {
  await nextTick()
  if (!reportVisible.value) {
    trendChart?.dispose()
    distributionChart?.dispose()
    trendChart = null
    distributionChart = null
    return
  }
  renderTrendChart()
  renderDistributionChart()
}

function ensureChartInstance(instance, el) {
  if (instance && !instance.isDisposed() && instance.getDom() === el) return instance
  if (instance && !instance.isDisposed()) instance.dispose()
  return echarts.init(el)
}

function renderTrendChart() {
  if (!trendChartRef.value || !report.value) return
  trendChart = ensureChartInstance(trendChart, trendChartRef.value)
  const rows = report.value.trend || []
  trendChart.setOption({
    animationDuration: 450,
    tooltip: { trigger: 'axis' },
    legend: { top: 4, left: 'center', itemGap: 18, data: ['慢 SQL 数', '平均耗时', 'P95'] },
    grid: { left: 48, right: 58, top: 42, bottom: 34 },
    xAxis: { type: 'category', data: rows.map(item => String(item.time || '').slice(11, 16)) },
    yAxis: [
      { type: 'value', name: '次数', minInterval: 1, nameTextStyle: { align: 'right', padding: [0, 8, 0, 0] }, splitLine: { lineStyle: { color: '#edf1f7' } } },
      { type: 'value', name: '秒', nameTextStyle: { align: 'left', padding: [0, 0, 0, 8] }, splitLine: { show: false } }
    ],
    series: [
      { name: '慢 SQL 数', type: 'bar', data: rows.map(item => item.slowCount || 0), barMaxWidth: 22, itemStyle: { color: '#4f7cff', borderRadius: [4, 4, 0, 0] } },
      { name: '平均耗时', type: 'line', yAxisIndex: 1, smooth: true, symbolSize: 5, data: rows.map(item => item.avgQueryTime || 0), lineStyle: { color: '#16a6a1', width: 2 }, itemStyle: { color: '#16a6a1' } },
      { name: 'P95', type: 'line', yAxisIndex: 1, smooth: true, symbolSize: 5, data: rows.map(item => item.p95QueryTime || 0), lineStyle: { color: '#f59e0b', width: 2 }, itemStyle: { color: '#f59e0b' }, areaStyle: { color: 'rgba(245,158,11,.08)' } }
    ]
  }, true)
}

function renderDistributionChart() {
  if (!distributionChartRef.value || !report.value) return
  distributionChart = ensureChartInstance(distributionChart, distributionChartRef.value)
  const rows = (report.value.databaseDistribution || []).slice(0, 10).reverse()
  distributionChart.setOption({
    animationDuration: 450,
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' } },
    grid: { left: 92, right: 28, top: 12, bottom: 24 },
    xAxis: { type: 'value', minInterval: 1, splitLine: { lineStyle: { color: '#edf1f7' } } },
    yAxis: { type: 'category', data: rows.map(item => item.name), axisLabel: { width: 80, overflow: 'truncate' } },
    series: [{ type: 'bar', data: rows.map(item => item.count), barMaxWidth: 18, itemStyle: { color: '#7c5cff', borderRadius: [0, 5, 5, 0] }, label: { show: true, position: 'right' } }]
  }, true)
}

function handleResize() {
  trendChart?.resize()
  distributionChart?.resize()
}

function formatInteger(value) {
  return new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 0 }).format(Number(value) || 0)
}

function formatNumber(value) {
  return new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 }).format(Number(value) || 0)
}

function formatCompact(value) {
  return new Intl.NumberFormat('zh-CN', { notation: 'compact', maximumFractionDigits: 2 }).format(Number(value) || 0)
}

function formatSeconds(value) {
  const number = Number(value) || 0
  if (number >= 3600) return `${(number / 3600).toFixed(2)}h`
  if (number >= 60) return `${(number / 60).toFixed(2)}m`
  if (number >= 1) return `${number.toFixed(2)}s`
  return `${(number * 1000).toFixed(0)}ms`
}

function formatPercent(value) {
  return `${(Number(value) || 0).toFixed(1)}%`
}

function formatDateTime(value) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return date.toLocaleString('zh-CN', { hour12: false })
}

function riskTagType(level) {
  if (level === 'high') return 'danger'
  if (level === 'medium') return 'warning'
  if (level === 'low') return 'success'
  return 'info'
}

function deltaClass(value) {
  const numeric = Number(value) || 0
  if (numeric > 0) return 'is-up'
  if (numeric < 0) return 'is-down'
  return ''
}

function formatSignedInteger(value) {
  const numeric = Number(value) || 0
  return `${numeric > 0 ? '+' : ''}${formatInteger(numeric)}`
}

function formatSignedSeconds(value) {
  const numeric = Number(value) || 0
  if (numeric === 0) return '0ms'
  return `${numeric > 0 ? '+' : '-'}${formatSeconds(Math.abs(numeric))}`
}

function comparisonText(status) {
  if (status === 'new') return '新增'
  if (status === 'worsened') return '恶化'
  if (status === 'improved') return '好转'
  if (status === 'stable') return '持平'
  return status || '-'
}

function comparisonTagType(status) {
  if (status === 'worsened') return 'danger'
  if (status === 'new') return 'warning'
  if (status === 'improved') return 'success'
  return 'info'
}

function comparisonTooltip(row) {
  const comparison = row?.comparison
  if (!comparison) return ''
  if (comparison.status === 'new') return '未进入前一日 Top SQL'
  const pct = value => `${value > 0 ? '+' : ''}${(Number(value) * 100).toFixed(1)}%`
  return `前一日：执行 ${formatInteger(comparison.prevCount)} 次，平均 ${formatSeconds(comparison.prevQueryTimeAvg)}，P95 ${formatSeconds(comparison.prevQueryTimeP95)}；执行次数变化 ${pct(comparison.countChangeRatio)}，P95 变化 ${pct(comparison.p95ChangeRatio)}`
}

function riskText(level) {
  if (level === 'high') return '高'
  if (level === 'medium') return '中'
  if (level === 'low') return '低'
  return '未知'
}

function optimizationConfidenceText(level) {
  if (level === 'high') return '高可信'
  if (level === 'medium') return '中可信'
  return '低可信'
}

function optimizationConfidenceType(level) {
  if (level === 'high') return 'success'
  if (level === 'medium') return 'warning'
  return 'info'
}

function optimizationSourceText(source) {
  if (source?.startsWith('llm+')) return 'AI分析'
  return '待AI分析'
}

function optimizationSectionsText(row = {}) {
  const sections = row.optimizationSections
  if (!sections) return ''
  return [
    sections.slowReason && `慢查询原因：${sections.slowReason}`,
    sections.schemaOptimization && `表结构优化：${sections.schemaOptimization}`,
    sections.indexOptimization && `索引优化：${sections.indexOptimization}`,
    sections.sqlOptimization && `SQL 优化：${sections.sqlOptimization}`,
    sections.expectedEffect && `预期效果：${sections.expectedEffect}`
  ].filter(Boolean).join('\n')
}

function optimizationAdviceTitle(row = {}) {
  const evidence = Array.isArray(row.optimizationEvidence) ? row.optimizationEvidence.join('\n') : ''
  const citations = Array.isArray(row.optimizationCitations) ? row.optimizationCitations.join('\n') : ''
  const limitations = Array.isArray(row.optimizationLimitations) ? row.optimizationLimitations.join('\n') : ''
  return [optimizationSectionsText(row), row.optimizationError, citations && `模型引用：\n${citations}`, limitations && `分析限制：\n${limitations}`, evidence && `输入证据：\n${evidence}`]
    .filter(Boolean)
    .join('\n\n')
}

onMounted(async () => {
  window.addEventListener('resize', handleResize)
  await loadInstances()
})

onBeforeUnmount(() => {
  stopReportPolling()
  window.removeEventListener('resize', handleResize)
  trendChart?.dispose()
  distributionChart?.dispose()
  trendChart = null
  distributionChart = null
})
</script>

<style scoped>
.log-page {
  --log-page-height-offset: 114px;
  --log-surface-warm: rgba(255, 253, 248, 0.96);
  --log-surface-cool: rgba(247, 250, 255, 0.92);
  --log-text-primary: #172133;
  --log-text-secondary: #5f6b7c;
  --log-text-muted: #8d97a8;
  --log-border: rgba(214, 223, 235, 0.96);
  --log-brand: #4c8bf5;
  --log-topbar: linear-gradient(90deg, #fef7ef 0%, #f8f8f6 52%, #edf2fd 100%);
  min-height: calc(100vh - var(--log-page-height-offset));
  padding: 6px;
  box-sizing: border-box;
  color: var(--log-text-secondary);
  background: transparent;
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.page-card {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: calc(100vh - var(--log-page-height-offset) - 12px);
  max-height: calc(100vh - var(--log-page-height-offset) - 12px);
  padding: 24px 14px 12px;
  overflow: hidden;
  border: 1px solid var(--log-border);
  border-radius: 22px;
  background: linear-gradient(135deg, var(--log-surface-warm), var(--log-surface-cool));
  box-shadow: 0 18px 48px rgba(17, 24, 39, 0.08);
}

.page-card::before {
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 24px;
  background: var(--log-topbar);
  content: "";
}

.page-header {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  min-height: 64px;
  justify-content: space-between;
  gap: 14px;
  margin: -24px -14px 8px;
  padding: 0 14px;
  background: var(--log-topbar);
}

.page-header h2 {
  margin: 0;
  color: var(--log-text-primary);
  font-size: 22px;
  line-height: 1.15;
}

.filter-form,
.filter-actions {
  display: flex;
  align-items: center;
}

.report-content {
  min-height: 0;
  flex: 1;
  padding: 0 2px 4px;
  overflow-x: hidden;
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: rgba(148, 163, 184, 0.65) transparent;
}

.report-content::-webkit-scrollbar {
  width: 6px;
}

.report-content::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: rgba(148, 163, 184, 0.55);
}

.filter-card,
.report-overview,
.panel-card,
.empty-card,
.loading-card,
.report-state-card {
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.82);
  box-shadow: 0 8px 24px rgba(17, 24, 39, 0.05);
}

.filter-card {
  position: sticky;
  top: 0;
  z-index: 6;
  padding: 9px 10px 8px;
  background: rgba(255, 255, 255, 0.95);
}

.filter-form {
  flex-wrap: wrap;
  gap: 8px;
}

.filter-form :deep(.el-form-item) {
  margin-right: 0;
  margin-bottom: 0;
}

.filter-form :deep(.el-form-item__label) {
  color: var(--log-text-secondary);
  font-size: 12px;
  font-weight: 600;
}

.instance-select {
  width: 420px;
}

.filter-actions {
  margin-left: auto;
}

.instance-option {
  display: flex;
  align-items: center;
  gap: 9px;
  width: 100%;
}

.instance-option strong {
  min-width: 116px;
  color: var(--log-text-primary);
}

.instance-option span {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  color: var(--log-text-secondary);
  font-family: "SFMono-Regular", Consolas, monospace;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.source-context {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 18px;
  margin-top: 8px;
  padding: 6px 4px 0;
  color: var(--log-text-secondary);
  font-size: 11px;
}

.source-context span {
  display: inline-flex;
  min-width: 0;
  align-items: center;
  gap: 6px;
}

.source-context b {
  color: var(--log-text-muted);
  font-weight: 600;
}

.source-context code {
  display: inline-block;
  max-width: 430px;
  overflow: hidden;
  color: var(--log-text-primary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.collected-files code {
  max-width: 300px;
}

.collected-files-hint {
  flex: none;
  color: var(--log-text-muted);
  font-size: 12px;
  opacity: 0.65;
}

.context-alert,
.report-progress-card,
.metric-grid,
.chart-grid,
.dimension-grid,
.top-sql-card,
.report-overview,
.loading-card,
.empty-card,
.report-state-card {
  margin-top: 10px;
}

.report-progress-card {
  padding: 12px 16px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: 0 8px 24px rgba(17, 24, 39, 0.05);
}

.progress-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  color: var(--log-text-primary);
  font-size: 12px;
}

.report-progress-card p {
  margin: 5px 0 0;
  color: var(--log-text-muted);
  font-size: 11px;
}

.analysis-pending {
  color: var(--log-brand);
}

.context-alert :deep(.el-alert) {
  border-radius: 10px;
}

.context-alert {
  overflow: hidden;
  border-radius: 10px;
}

.loading-card {
  height: 300px;
}

.empty-card,
.report-state-card {
  display: grid;
  min-height: 300px;
  place-items: center;
}

.report-overview {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 0 16px;
  align-items: center;
  padding: 12px 16px 10px 18px;
  overflow: hidden;
  background: linear-gradient(110deg, rgba(239, 246, 255, 0.96), rgba(255, 252, 246, 0.96));
}

.report-overview::before {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: 4px;
  background: linear-gradient(180deg, #6aa1fb, #4d79d8);
  content: "";
}

.report-date {
  color: var(--log-brand);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.12em;
}

.report-overview h3 {
  margin: 4px 0 3px;
  color: var(--log-text-primary);
  font-size: 17px;
}

.report-overview p,
.panel-head p {
  margin: 0;
  color: var(--log-text-muted);
  font-size: 11px;
}

.report-status {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
  color: var(--log-text-muted);
  font-size: 11px;
}

.report-status-head {
  display: flex;
  min-height: 26px;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}

.report-notification-button {
  position: relative;
  width: 26px;
  height: 26px;
  color: #c77b1c;
  font-size: 17px;
  overflow: visible;
}

.report-notification-button:hover,
.report-notification-button:focus-visible {
  color: #a85f08;
  background: rgba(227, 155, 50, 0.13);
}

.report-notification-count {
  position: absolute;
  top: -5px;
  right: -7px;
  display: grid;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  place-items: center;
  box-sizing: border-box;
  border: 2px solid #fff;
  border-radius: 8px;
  color: #fff;
  background: #e65d62;
  font-size: 9px;
  font-weight: 700;
  line-height: 12px;
}

.report-status :deep(.el-button) {
  min-height: auto;
  padding: 0;
}

:global(.slow-report-notification-popper.el-popover) {
  max-width: calc(100vw - 24px);
  padding: 0;
  overflow: hidden;
  border-radius: 10px;
}

:global(.report-notification-panel header) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px;
  border-bottom: 1px solid #e5eaf2;
  color: #172133;
}

:global(.report-notification-panel header strong) {
  font-size: 13px;
}

:global(.report-notification-panel header span) {
  color: #8d97a8;
  font-size: 12px;
}

:global(.report-notification-list) {
  max-height: min(360px, 60vh);
  padding: 10px;
  overflow-y: auto;
  background: rgba(247, 250, 255, 0.6);
}

:global(.report-notification-item) {
  display: grid;
  grid-template-columns: 18px minmax(0, 1fr);
  gap: 8px;
  align-items: start;
  padding: 8px 10px;
  border: 1px solid rgba(214, 223, 235, 0.85);
  border-radius: 10px;
  background: #fff;
  box-shadow: 0 2px 8px rgba(23, 33, 51, 0.04);
  color: #5f6b7c;
  font-size: 12px;
  line-height: 1.55;
}

:global(.report-notification-item + .report-notification-item) {
  margin-top: 8px;
}

:global(.report-notification-item .el-icon) {
  display: inline-flex;
  width: 18px;
  height: 18px;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  background: rgba(227, 155, 50, 0.12);
  color: #e39b32;
  font-size: 12px;
}

:global(.slow-report-gone-sql-popper.el-popover),
:global(.slow-report-collected-files-popper.el-popover) {
  max-width: calc(100vw - 24px);
  padding: 0;
  overflow: hidden;
  border-radius: 10px;
}

:global(.gone-sql-panel header),
:global(.collected-files-panel header) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px;
  border-bottom: 1px solid #e5eaf2;
  color: #172133;
}

:global(.gone-sql-panel header strong),
:global(.collected-files-panel header strong) {
  font-size: 13px;
}

:global(.gone-sql-panel header span),
:global(.collected-files-panel header span) {
  color: #8d97a8;
  font-size: 12px;
}

:global(.gone-sql-list),
:global(.collected-files-list) {
  max-height: min(380px, 60vh);
  padding: 10px;
  overflow-y: auto;
  background: rgba(247, 250, 255, 0.6);
}

:global(.gone-sql-item),
:global(.collected-files-item) {
  padding: 8px 10px;
  border: 1px solid rgba(214, 223, 235, 0.85);
  border-radius: 10px;
  background: #fff;
  box-shadow: 0 2px 8px rgba(23, 33, 51, 0.04);
}

:global(.gone-sql-item + .gone-sql-item),
:global(.collected-files-item + .collected-files-item) {
  margin-top: 8px;
}

:global(.gone-sql-top),
:global(.collected-files-top) {
  display: flex;
  gap: 8px;
  align-items: flex-start;
}

:global(.gone-sql-top em),
:global(.collected-files-top em) {
  flex: none;
  display: inline-flex;
  width: 18px;
  height: 18px;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  background: rgba(76, 139, 245, 0.12);
  color: #4c8bf5;
  font-size: 11px;
  font-style: normal;
  font-weight: 600;
}

:global(.gone-sql-top code),
:global(.collected-files-top code) {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
  color: #172133;
  font-family: "SFMono-Regular", Consolas, Menlo, monospace;
  font-size: 11px;
  line-height: 1.6;
  word-break: break-all;
}

:global(.gone-sql-meta),
:global(.collected-files-meta) {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
  margin-top: 7px;
}

:global(.collected-files-meta span) {
  color: #8d97a8;
  font-size: 10px;
}

:global(.collected-files-meta b) {
  font-weight: 600;
}

:global(.collected-files-count b) {
  color: #4c8bf5;
}

:global(.collected-files-seen) {
  margin-left: auto;
}

:global(.collected-files-seen b) {
  color: #e39b32;
}

:global(.gone-sql-db),
:global(.gone-sql-user) {
  max-width: 160px;
  overflow: hidden;
  padding: 1px 8px;
  border-radius: 999px;
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:global(.gone-sql-db) {
  background: rgba(128, 103, 207, 0.1);
  color: #6f58b5;
}

:global(.gone-sql-user) {
  background: rgba(44, 166, 164, 0.1);
  color: #22807e;
}

:global(.gone-sql-metric) {
  color: #8d97a8;
  font-size: 10px;
}

:global(.gone-sql-metric b) {
  font-weight: 600;
}

:global(.gone-sql-count) {
  margin-left: auto;
}

:global(.gone-sql-count b) {
  color: #4c8bf5;
}

:global(.gone-sql-p95 b) {
  color: #e39b32;
}

.overview-meta {
  grid-column: 1 / -1;
  display: flex;
  flex-wrap: wrap;
  gap: 4px 18px;
  align-items: center;
  margin-top: 9px;
  padding-top: 6px;
  color: var(--log-text-secondary);
  font-size: 11px;
}

.overview-meta span {
  display: inline-flex;
  gap: 5px;
  align-items: baseline;
}

.overview-meta b {
  color: var(--log-text-muted);
  font-weight: 600;
}

.overview-meta .is-business { color: #238b64; }
.overview-meta .is-monitoring { color: #6f58b5; }
.overview-meta .is-invalid { color: #c77b1c; }
.overview-meta .comparison-title {
  margin-left: 6px;
  color: var(--log-text-primary);
  font-weight: 600;
}
.overview-meta .is-new { color: #c77b1c; }
.overview-meta .is-worse { color: #d15b5b; }
.overview-meta .is-better { color: #238b64; }
.overview-meta .comparison-missing { color: var(--log-text-muted); }
.overview-meta :deep(.el-button) { font-size: 11px; }

.comparison-none { color: var(--log-text-muted); }

.metric-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 9px;
}

.metric-card {
  position: relative;
  min-height: 78px;
  padding: 9px 12px;
  overflow: hidden;
  border-radius: 13px;
  background: rgba(255, 255, 255, 0.88);
  box-shadow: 0 6px 18px rgba(17, 24, 39, 0.04);
}

.metric-card::before {
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 3px;
  background: var(--card-color);
  opacity: 0.78;
  content: "";
}

.metric-card span {
  display: block;
  color: var(--log-text-muted);
  font-size: 11px;
}

.metric-card strong {
  display: block;
  margin-top: 4px;
  color: var(--card-color);
  font-size: 20px;
  line-height: 1.15;
}

.metric-card small {
  display: flex;
  gap: 5px;
  align-items: baseline;
  margin-top: 4px;
  overflow: hidden;
  color: var(--log-text-muted);
  font-size: 10px;
  white-space: nowrap;
}

.metric-card .metric-delta {
  flex: none;
  font-style: normal;
  font-weight: 700;
}

.metric-card .metric-delta.is-up { color: #d15b5b; }
.metric-card .metric-delta.is-down { color: #238b64; }

.metric-card .metric-hint {
  overflow: hidden;
  text-overflow: ellipsis;
}

.tone-blue { --card-color: #4c8bf5; }
.tone-cyan { --card-color: #2ca6a4; }
.tone-orange { --card-color: #e39b32; }
.tone-red { --card-color: #ef6b73; }
.tone-purple { --card-color: #8067cf; }
.tone-green { --card-color: #3fb97f; }

.chart-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.65fr) minmax(300px, 0.75fr);
  gap: 9px;
}

.panel-card {
  overflow: hidden;
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 12px 13px 7px;
}

.panel-head h3 {
  margin: 0 0 2px;
  color: var(--log-text-primary);
  font-size: 14px;
}

.chart {
  height: 240px;
}

.dimension-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(0, 0.8fr);
  gap: 9px;
}

.dimension-card {
  min-height: 170px;
  padding-bottom: 10px;
}

.dimension-list {
  display: grid;
  gap: 8px;
  padding: 8px 13px 2px;
}

.dimension-row {
  display: grid;
  grid-template-columns: 100px minmax(80px, 1fr) 66px 50px;
  align-items: center;
  gap: 8px;
  font-size: 11px;
}

.dimension-row > span {
  overflow: hidden;
  color: var(--log-text-secondary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dimension-row > div {
  height: 6px;
  overflow: hidden;
  border-radius: 999px;
  background: #e9eef6;
}

.dimension-row i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, #6aa1fb, #7d72d8);
}

.dimension-row strong {
  color: var(--log-text-primary);
  text-align: right;
}

.dimension-row em {
  color: var(--log-text-muted);
  font-style: normal;
  text-align: right;
}

.top-sql-card {
  padding-bottom: 4px;
}

.top-sql-head {
  padding-bottom: 9px;
}

.top-sql-tools {
  display: flex;
  align-items: center;
  gap: 10px;
}

.table-search {
  width: 260px;
}

.top-sql-table {
  width: 100%;
  --el-table-border-color: rgba(223, 231, 242, 0.92);
  --el-table-header-bg-color: rgba(248, 250, 255, 0.94);
  --el-table-row-hover-bg-color: rgba(76, 139, 245, 0.05);
  --el-table-text-color: var(--log-text-secondary);
  --el-table-header-text-color: var(--log-text-muted);
}

.top-sql-table :deep(.el-table__cell) {
  padding: 4px 0;
}

.top-sql-table :deep(.cell) {
  line-height: 1.35;
}

.sql-content {
  display: grid;
  gap: 3px;
}

.sql-content code {
  overflow: hidden;
  color: #4d79d8;
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sql-content span {
  display: -webkit-box;
  overflow: hidden;
  color: var(--log-text-secondary);
  font-family: "SFMono-Regular", Consolas, monospace;
  font-size: 11px;
  line-height: 1.4;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 1;
}

.optimization-advice {
  color: var(--log-text-secondary);
  font-size: 11px;
  line-height: 1.2;
  white-space: normal;
  word-break: break-word;
  cursor: help;
}

.optimization-meta {
  display: flex;
  align-items: center;
  gap: 7px;
  margin-bottom: 4px;
}

.optimization-meta span {
  overflow: hidden;
  color: var(--log-text-muted);
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.optimization-advice p {
  display: -webkit-box;
  margin: 0;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.optimization-sections p {
  display: -webkit-box;
  margin: 0 0 2px;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 1;
}

.optimization-sections b {
  margin-right: 4px;
  color: var(--log-text-muted);
  font-weight: 600;
}

.optimization-advice .optimization-error {
  color: #b45309;
}

.optimization-advice small {
  display: block;
  margin-top: 4px;
  overflow: hidden;
  color: #7b879b;
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:global(.optimization-audit-tooltip) {
  max-width: min(680px, calc(100vw - 32px));
  max-height: 360px;
  overflow-y: auto;
  border: 1px solid #d9e3ee !important;
  border-radius: 8px !important;
  background: #f8fafc !important;
  box-shadow: 0 8px 24px rgba(30, 55, 80, 0.16) !important;
  color: #334155 !important;
  font-size: 12px;
  line-height: 1.7;
  white-space: pre-line;
  word-break: break-word;
}

:global(.optimization-audit-tooltip .el-popper__arrow::before) {
  border: 1px solid #d9e3ee !important;
  background: #f8fafc !important;
}

.log-page :deep(.el-input__wrapper),
.log-page :deep(.el-select__wrapper),
.log-page :deep(.el-date-editor.el-input__wrapper) {
  min-height: 30px;
  border-radius: 10px;
  background: rgba(252, 253, 255, 0.98);
  box-shadow: inset 0 0 0 1px var(--log-border) !important;
}

.log-page :deep(.el-input__wrapper.is-focus),
.log-page :deep(.el-select__wrapper.is-focused),
.log-page :deep(.el-date-editor.el-input__wrapper.is-active) {
  box-shadow: inset 0 0 0 1px rgba(120, 156, 221, 0.96), 0 0 0 4px rgba(76, 139, 245, 0.08) !important;
}

.log-page :deep(.el-button) {
  border-radius: 9px !important;
  font-weight: 600;
  transform: none !important;
}

.log-page :deep(.el-button.is-link:hover),
.log-page :deep(.el-button.is-link:focus-visible),
.log-page :deep(.el-button.is-text:hover),
.log-page :deep(.el-button.is-text:focus-visible) {
  box-shadow: none;
}

.filter-actions :deep(.el-form-item__content) {
  gap: 10px;
}

.filter-actions :deep(.el-button) {
  height: 32px;
  margin-left: 0;
  padding: 0 14px;
  font-size: 13px;
}

.filter-actions :deep(.el-button.is-circle) {
  width: 32px;
  padding: 0;
}

.log-page :deep(.el-button--primary:not(.is-link):not(.is-plain)) {
  border-color: transparent;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
  box-shadow: 0 2px 8px rgba(76, 139, 245, 0.28);
}

@media (max-width: 1500px) {
  .metric-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

}

@media (max-width: 1080px) {
  .page-card {
    max-height: none;
  }

  .page-header,
  .report-overview {
    align-items: flex-start;
    flex-direction: column;
  }

  .filter-form {
    align-items: stretch;
    flex-direction: column;
  }

  .filter-actions {
    margin-left: 0;
  }

  .instance-select {
    width: min(100%, 520px);
  }

  .report-status {
    align-items: flex-start;
  }

  .chart-grid,
  .dimension-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .metric-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .top-sql-head {
    align-items: flex-start;
    flex-direction: column;
  }

  .top-sql-tools {
    width: 100%;
    flex-wrap: wrap;
  }

  .table-search {
    width: 100%;
  }
}
</style>
