<template>
  <div class="monitor-alert-page">
    <section class="monitor-alert-shell">
      <header class="monitor-alert-header">
        <div class="header-main-row">
          <div class="header-identity">
            <el-button class="icon-command icon-command--plain" text aria-label="返回业务大盘" @click="goDashboard">
              <el-icon><ArrowLeft /></el-icon>
            </el-button>
            <div class="business-heading">
              <div class="business-path">
                <span>业务系统</span>
                <i>/</i>
                <strong>{{ businessTitle }}</strong>
                <em :class="['data-status', `is-${dataStatusTone}`]">{{ dataStatusText }}</em>
              </div>
              <p>{{ diagnosis.message || businessConclusion }}<template v-if="generatedAt"> · 更新于 {{ formatDateTime(generatedAt) }}</template></p>
            </div>
          </div>

          <div class="header-actions">
            <el-tooltip content="刷新业务状态" placement="bottom">
              <el-button class="icon-command" :loading="loading" aria-label="刷新业务状态" @click="loadMonitorAlerts">
                <el-icon><Refresh /></el-icon>
              </el-button>
            </el-tooltip>
            <el-button class="config-command" type="primary" :icon="Setting" @click="openConfigDialog">告警配置</el-button>
          </div>
        </div>

        <nav class="detail-tabs" aria-label="业务详情视图">
          <button type="button" @click="goDashboard">业务大盘</button>
          <button type="button" @click="goTopology">编辑拓扑图</button>
          <button type="button" @click="goLogAnalysis">日志分析</button>
          <button type="button" class="is-active" aria-current="page">监控告警</button>
        </nav>
      </header>

      <div v-if="visibleWarningMessages.length" class="warning-banner" role="alert">
        <el-icon><Warning /></el-icon>
        <span>{{ visibleWarningMessages.join('；') }}</span>
      </div>

      <div v-if="loadError" class="load-error" role="alert">
        <el-icon><Warning /></el-icon>
        <div><strong>监控告警加载失败</strong><span>{{ loadError }}</span></div>
        <el-button type="primary" size="small" @click="loadMonitorAlerts">重新加载</el-button>
      </div>

      <main v-else v-loading="loading" class="monitor-alert-content" aria-label="监控告警">
        <section class="summary-strip business-conclusion" aria-label="业务状态结论">
          <article v-for="item in summaryItems" :key="item.key">
            <span :class="['summary-icon', `is-${item.tone}`]"><el-icon><component :is="item.icon" /></el-icon></span>
            <div><small>{{ item.label }}</small><strong :class="`is-${item.tone}`">{{ item.value }}</strong><em v-if="item.caption">{{ item.caption }}</em></div>
          </article>
        </section>

        <div class="dashboard-grid">
          <div class="dashboard-main">
            <section :class="['primary-problem-card', `is-${businessHealthTone}`]">
              <div class="primary-problem-copy">
                <span>主要问题</span>
                <h2>{{ primaryProblemTitle }}</h2>
                <p>{{ businessConclusion }}</p>
              </div>
              <div class="primary-problem-facts">
                <div class="primary-fact primary-fact--value"><span>当前值</span><strong>{{ primaryMetric ? metricCurrentValue(primaryMetric, true) : '-' }}</strong></div>
                <div><span>严重阈值</span><strong>{{ primaryMetric ? formatMetricThreshold(primaryMetric.criticalThreshold, primaryMetric.unit) : '-' }}</strong></div>
                <div><span>超出</span><strong>{{ primaryMetric ? metricOverageText(primaryMetric) : '-' }}</strong></div>
                <div><span>节点</span><strong>{{ primaryMetric ? metricAffectedNodeText(primaryMetric) : '-' }}</strong></div>
              </div>
              <div class="primary-problem-action"><span>建议</span><strong>{{ primarySuggestedAction }}</strong></div>
            </section>

            <section class="visual-section metric-card-section">
              <div class="visual-section-heading"><div><h3>压力指标</h3><span>{{ currentMetrics.length }} 项</span></div><small>点击指标查看详情</small></div>
              <div class="metric-card-grid">
                <button v-for="metric in currentMetrics" :key="metric.metricKey || metricRuleName(metric)" type="button" :class="['metric-card', `is-${metricStatus(metric)}`, { active: selectedKind === 'metric' && selectedRecord === metric }]" @click="selectMetric(metric)">
                  <div class="metric-card-head"><span>{{ metricShortName(metric) }}</span><em>{{ metricStatusText(metric) }}</em></div>
                  <strong>{{ metricCurrentValue(metric, true) }}</strong>
                  <div class="metric-progress"><i :style="{ width: `${metricProgress(metric)}%` }"></i></div>
                  <small>{{ metricThresholdBrief(metric) }}</small>
                </button>
                <div v-if="!currentMetrics.length" class="visual-empty">当前没有压力指标</div>
              </div>
            </section>

            <section class="visual-section node-card-section">
              <div class="visual-section-heading node-section-heading">
                <div><h3>业务节点状态</h3><span>{{ businessNodes.length }} 个节点</span></div>
                <div class="node-filter" role="group" aria-label="节点状态筛选">
                  <button v-for="item in nodeFilterOptions" :key="item.value" type="button" :class="{ active: nodeStatusFilter === item.value }" @click="nodeStatusFilter = item.value">{{ item.label }} {{ nodeFilterCount(item.value) }}</button>
                </div>
              </div>
              <div class="node-card-list">
                <article v-for="node in visibleBusinessNodes" :key="node.resourceId || node.id || nodeName(node)" :class="['node-card', `is-${nodeHealth(node)}`, { active: selectedKind === 'node' && selectedRecord === node }]" @click="selectNode(node)">
                  <div><strong><i :class="`is-${nodeHealth(node)}`"></i>{{ nodeName(node) }}</strong><span :class="['severity-tag', `is-${nodeSeverity(node)}`]">{{ nodeStatusText(node) }}</span></div>
                  <p>{{ nodeCardSummary(node) }}</p>
                  <small>{{ nodeWorstProblemText(node) }}</small>
                  <div v-if="nodeWorstMetric(node)" class="node-mini-progress"><i :style="{ width: `${metricProgress(nodeWorstMetric(node))}%` }"></i></div>
                  <button type="button" @click.stop="selectNode(node)">查看详情</button>
                </article>
                <div v-if="!visibleBusinessNodes.length" class="visual-empty">当前没有符合筛选的业务节点</div>
              </div>
            </section>

            <details class="detail-mode">
              <summary><span>明细模式</span><small>查看指标、节点和告警原始列表</small></summary>
              <div class="detail-tables">
            <section class="data-panel metrics-panel">
              <div class="panel-heading">
                <div><h3>当前压力指标</h3><span>{{ currentMetricSummary.total || currentMetrics.length }} 项</span></div>
                <div class="metric-summary"><span class="is-critical">严重 {{ currentMetricSummary.critical || 0 }}</span><span class="is-warning">警告 {{ currentMetricSummary.warning || 0 }}</span><span class="is-healthy">正常 {{ currentMetricSummary.healthy || 0 }}</span><span class="is-muted">无数据 {{ currentMetricSummary.noData || 0 }}</span><span class="is-danger">查询失败 {{ currentMetricSummary.queryFailed || 0 }}</span></div>
              </div>
              <el-table :data="currentMetrics" height="100%" size="small" empty-text="当前没有压力指标" highlight-current-row @row-click="selectMetric">
                <el-table-column label="指标" min-width="170" show-overflow-tooltip>
                  <template #default="{ row }"><strong class="primary-cell">{{ metricRuleName(row) }}</strong></template>
                </el-table-column>
                <el-table-column label="当前最差值" width="120" align="right">
                  <template #default="{ row }"><strong :class="['metric-value', `is-${metricStatus(row)}`]">{{ metricCurrentValue(row, true) }}</strong></template>
                </el-table-column>
                <el-table-column label="状态" width="95">
                  <template #default="{ row }"><span :class="['metric-status', `is-${metricStatus(row)}`]">{{ metricStatusText(row) }}</span></template>
                </el-table-column>
                <el-table-column label="阈值" width="155" show-overflow-tooltip>
                  <template #default="{ row }">{{ metricThresholdText(row) }}</template>
                </el-table-column>
                <el-table-column label="关联节点" min-width="180" show-overflow-tooltip>
                  <template #default="{ row }">{{ metricAffectedNodeText(row) }}</template>
                </el-table-column>
                <el-table-column label="建议动作" min-width="220" show-overflow-tooltip>
                  <template #default="{ row }">{{ metricSuggestedAction(row) }}</template>
                </el-table-column>
              </el-table>
            </section>

            <section class="data-panel affected-panel">
              <div class="panel-heading">
                <div><h3>业务节点状态</h3><span>{{ businessNodes.length }} 个节点</span></div>
              </div>
              <el-table
                :data="businessNodes"
                height="210"
                size="small"
                empty-text="当前没有业务节点"
                highlight-current-row
                @row-click="selectNode"
              >
                <el-table-column label="节点名称" min-width="170" show-overflow-tooltip>
                  <template #default="{ row }"><strong class="primary-cell">{{ nodeName(row) }}</strong></template>
                </el-table-column>
                <el-table-column label="节点类型" width="110">
                  <template #default="{ row }">{{ nodeType(row) }}</template>
                </el-table-column>
                <el-table-column label="风险项" width="76" align="right">
                  <template #default="{ row }">{{ nodeRiskCount(row) }}</template>
                </el-table-column>
                <el-table-column label="最严重级别" width="110">
                  <template #default="{ row }"><span :class="['severity-tag', `is-${nodeSeverity(row)}`]">{{ severityText(nodeSeverity(row)) }}</span></template>
                </el-table-column>
                <el-table-column label="健康状态" width="90">
                  <template #default="{ row }"><span :class="['health-text', `is-${nodeHealth(row)}`]">{{ healthText(nodeHealth(row)) }}</span></template>
                </el-table-column>
                <el-table-column label="问题依据" min-width="210" show-overflow-tooltip>
                  <template #default="{ row }">{{ nodeEvidence(row) }}</template>
                </el-table-column>
                <el-table-column label="压力摘要" width="190" show-overflow-tooltip>
                  <template #default="{ row }">{{ nodeMetricSummary(row) }}</template>
                </el-table-column>
              </el-table>
            </section>

            <section class="data-panel alerts-panel">
              <div class="panel-heading">
                <div><h3>辅助活跃告警</h3><span>{{ pagination.total }} 条</span></div>
                <div class="severity-legend"><span class="is-critical">严重 {{ summaryCount('critical') }}</span><span class="is-warning">警告 {{ summaryCount('warning') }}</span></div>
              </div>
              <el-table
                :data="activeAlerts"
                height="calc(100% - 84px)"
                size="small"
                empty-text="当前没有活跃告警"
                highlight-current-row
                @row-click="selectAlert"
              >
                <el-table-column label="告警名称" min-width="210" show-overflow-tooltip>
                  <template #default="{ row }"><strong class="primary-cell">{{ alertName(row) }}</strong></template>
                </el-table-column>
                <el-table-column label="级别" width="78">
                  <template #default="{ row }"><span :class="['severity-tag', `is-${severity(row)}`]">{{ severityText(severity(row)) }}</span></template>
                </el-table-column>
                <el-table-column label="状态" width="84">
                  <template #default="{ row }">{{ alertStatusText(row) }}</template>
                </el-table-column>
                <el-table-column label="关联节点" min-width="150" show-overflow-tooltip>
                  <template #default="{ row }">{{ relatedNodeText(row) }}</template>
                </el-table-column>
                <el-table-column label="首次触发" width="150">
                  <template #default="{ row }">{{ formatDateTime(firstTriggeredAt(row)) }}</template>
                </el-table-column>
                <el-table-column label="持续时间" width="100">
                  <template #default="{ row }">{{ alertDuration(row) }}</template>
                </el-table-column>
                <el-table-column label="告警值" width="100" show-overflow-tooltip>
                  <template #default="{ row }">{{ row.value ?? row.alertValue ?? '-' }}</template>
                </el-table-column>
                <el-table-column label="标记" width="105">
                  <template #default="{ row }">
                    <span v-if="isSilenced(row)" class="state-flag">静默</span>
                    <span v-if="isInhibited(row)" class="state-flag">抑制</span>
                    <span v-if="!isSilenced(row) && !isInhibited(row)">-</span>
                  </template>
                </el-table-column>
              </el-table>
              <div class="pagination-bar">
                <el-pagination
                  v-model:current-page="query.page"
                  v-model:page-size="query.pageSize"
                  small
                  background
                  layout="total, sizes, prev, pager, next"
                  :page-sizes="[10, 20, 50]"
                  :total="pagination.total"
                  @current-change="loadMonitorAlerts"
                  @size-change="handlePageSizeChange"
                />
              </div>
            </section>
              </div>
            </details>
          </div>

          <aside class="detail-panel">
            <div class="panel-heading">
              <div><h3>{{ selectedKind === 'node' ? '节点状态' : (selectedKind === 'metric' ? '问题详情' : '告警详情') }}</h3><span v-if="selectedRecord">{{ selectedKind === 'node' ? nodeName(selectedRecord) : (selectedKind === 'metric' ? metricRuleName(selectedRecord) : alertName(selectedRecord)) }}</span></div>
              <button v-if="selectedRecord" type="button" aria-label="关闭详情" @click="clearSelection"><el-icon><Close /></el-icon></button>
            </div>

            <div v-if="!selectedRecord" class="detail-empty">
              <el-icon><Bell /></el-icon>
              <strong>当前没有需要关注的问题</strong>
              <span>选择压力指标或影响节点查看详情</span>
            </div>

            <div v-else-if="selectedKind === 'node'" class="detail-scroll">
              <div class="detail-title"><span :class="['severity-tag', `is-${nodeSeverity(selectedRecord)}`]">{{ severityText(nodeSeverity(selectedRecord)) }}</span><strong>{{ nodeName(selectedRecord) }}</strong></div>
              <dl class="detail-fields">
                <div><dt>节点类型</dt><dd>{{ nodeType(selectedRecord) }}</dd></div>
                <div><dt>健康状态</dt><dd>{{ healthText(nodeHealth(selectedRecord)) }}</dd></div>
                <div><dt>实时指标</dt><dd>{{ nodeMetrics(selectedRecord).length }}</dd></div>
                <div><dt>风险指标</dt><dd>{{ nodeRiskCount(selectedRecord) }}</dd></div>
              </dl>
              <section class="detail-section"><h4>问题说明</h4><p>{{ nodeMetricSummary(selectedRecord) }}</p></section>
              <section v-if="nodeMetrics(selectedRecord).length" class="detail-section"><h4>实时监控指标</h4><div class="link-list"><button v-for="item in nodeMetrics(selectedRecord)" :key="item.metricKey || metricRuleName(item)" type="button" @click="selectMetric(item)">{{ metricRuleName(item) }}<small>{{ metricCurrentValue(item, true) }} · {{ metricStatusText(item) }}</small></button></div></section>
              <section v-if="nodeAlerts(selectedRecord).length" class="detail-section"><h4>关联告警</h4><div class="link-list"><button v-for="item in nodeAlerts(selectedRecord)" :key="alertKey(item)" type="button" @click="selectAlert(item)">{{ alertName(item) }}</button></div></section>
              <details v-if="objectEntries(selectedRecord.labels).length" class="advanced-detail">
                <summary>高级信息</summary>
                <section class="detail-section"><h4>Labels 证据</h4><div class="kv-list"><div v-for="entry in objectEntries(selectedRecord.labels)" :key="entry[0]"><span>{{ entry[0] }}</span><code>{{ entry[1] }}</code></div></div></section>
              </details>
            </div>

            <div v-else-if="selectedKind === 'metric'" class="detail-scroll">
              <div class="detail-title"><span :class="['metric-status', `is-${metricStatus(selectedRecord)}`]">{{ metricStatusText(selectedRecord) }}</span><strong>{{ metricProblemTitle(selectedRecord) }}</strong></div>
              <dl class="detail-fields">
                <div><dt>当前最差值</dt><dd>{{ metricCurrentValue(selectedRecord, true) }}</dd></div>
                <div><dt>阈值</dt><dd>预警 {{ formatMetricThreshold(selectedRecord.warningThreshold, selectedRecord.unit) }}，严重 {{ formatMetricThreshold(selectedRecord.criticalThreshold, selectedRecord.unit) }}</dd></div>
                <div><dt>影响节点</dt><dd>{{ metricAffectedNodeText(selectedRecord) }}</dd></div>
              </dl>
              <section class="detail-section"><h4>建议动作</h4><p>{{ metricSuggestedAction(selectedRecord) }}</p></section>
              <section class="detail-section"><h4>最近明细</h4><div class="recent-values"><div v-for="sample in metricRecentSamples(selectedRecord)" :key="`${sample.timestamp}-${sample.value}-${sampleNodeName(sample)}`"><span>{{ sampleNodeName(sample) }}</span><strong :class="`is-${sample.status || 'healthy'}`">{{ formatMetricThreshold(sample.value, sample.unit || selectedRecord.unit) }}</strong><em>{{ formatDateTime(sample.timestamp) }}</em></div><span v-if="!metricRecentSamples(selectedRecord).length">暂无最近明细</span></div></section>
              <details class="advanced-detail">
                <summary>高级信息</summary>
                <dl class="advanced-fields">
                  <div><dt>PromQL</dt><dd><code class="expression-code">{{ metricQuery(selectedRecord) }}</code></dd></div>
                  <div><dt>样本数量</dt><dd>{{ selectedRecord.sampleCount ?? selectedRecord.samples?.length ?? 0 }}</dd></div>
                  <div><dt>告警源</dt><dd>{{ datasourceName }}</dd></div>
                  <div><dt>生效规则</dt><dd>{{ enabledRuleCount }} 条</dd></div>
                  <div><dt>指标覆盖</dt><dd>{{ coverageNodeCount }} 个节点，{{ coverageSeriesCount }} 条 series</dd></div>
                </dl>
              </details>
            </div>

            <div v-else class="detail-scroll">
              <div class="detail-title"><span :class="['severity-tag', `is-${severity(selectedRecord)}`]">{{ severityText(severity(selectedRecord)) }}</span><strong>{{ alertName(selectedRecord) }}</strong></div>
              <p class="problem-summary">{{ alertDescription(selectedRecord) }}</p>
              <dl class="detail-fields">
                <div><dt>当前状态</dt><dd>{{ alertStatusText(selectedRecord) }}</dd></div>
                <div><dt>所属告警源</dt><dd>{{ selectedRecord.datasourceName || selectedRecord.datasource?.name || datasourceName }}</dd></div>
                <div><dt>首次触发</dt><dd>{{ formatDateTime(firstTriggeredAt(selectedRecord)) }}</dd></div>
                <div><dt>最近检测</dt><dd>{{ formatDateTime(selectedRecord.lastDetectedAt || selectedRecord.lastSeenAt || selectedRecord.updatedAt) }}</dd></div>
                <div><dt>持续时间</dt><dd>{{ alertDuration(selectedRecord) }}</dd></div>
                <div><dt>告警值</dt><dd>{{ selectedRecord.value ?? selectedRecord.alertValue ?? '-' }}</dd></div>
              </dl>
              <section class="detail-section"><h4>影响节点</h4><p>{{ relatedNodeText(selectedRecord) }}</p></section>
              <section class="detail-section"><h4>命中规则</h4><p>{{ matchedRuleText(selectedRecord) }}</p></section>
              <section v-if="selectedRecord.recommendation || selectedRecord.suggestion" class="detail-section"><h4>处理建议</h4><p>{{ selectedRecord.recommendation || selectedRecord.suggestion }}</p></section>
              <details v-if="alertCondition(selectedRecord) || alertExpression(selectedRecord) || objectEntries(selectedRecord.labels).length || objectEntries(selectedRecord.annotations).length" class="advanced-detail">
                <summary>高级信息</summary>
                <section v-if="alertCondition(selectedRecord)" class="detail-section"><h4>触发条件</h4><p>{{ alertCondition(selectedRecord) }}</p></section>
                <section v-if="alertExpression(selectedRecord)" class="detail-section"><h4>触发表达式</h4><code class="expression-code">{{ alertExpression(selectedRecord) }}</code></section>
                <section v-if="objectEntries(selectedRecord.labels).length" class="detail-section"><h4>Labels</h4><div class="kv-list"><div v-for="entry in objectEntries(selectedRecord.labels)" :key="entry[0]"><span>{{ entry[0] }}</span><code>{{ entry[1] }}</code></div></div></section>
                <section v-if="objectEntries(selectedRecord.annotations).length" class="detail-section"><h4>Annotations</h4><div class="kv-list"><div v-for="entry in objectEntries(selectedRecord.annotations)" :key="entry[0]"><span>{{ entry[0] }}</span><code>{{ entry[1] }}</code></div></div></section>
              </details>
            </div>
          </aside>
        </div>
      </main>
    </section>

    <el-dialog v-model="configVisible" title="告警配置" width="min(920px, calc(100vw - 32px))" class="monitor-alert-config-dialog" destroy-on-close>
      <div v-loading="configLoading" class="config-body">
        <el-form label-position="top" class="config-form">
          <div class="config-basics">
            <el-form-item label="启用业务告警匹配">
              <el-switch v-model="configForm.enabled" />
            </el-form-item>
            <el-form-item label="告警源">
              <el-select v-model="configForm.datasourceId" filterable clearable placeholder="请选择告警源" :disabled="!configForm.enabled">
                <el-option v-for="item in datasourceOptions" :key="datasourceId(item)" :label="datasourceOptionLabel(item)" :value="datasourceId(item)" />
              </el-select>
            </el-form-item>
          </div>

          <section class="rule-section">
            <div class="config-section-heading"><div><h3>告警匹配规则</h3><span>{{ configForm.matchRules.length }} 条规则</span></div><el-button :icon="Plus" @click="addRule">新增规则</el-button></div>
            <div class="rule-list">
              <div v-for="(rule, index) in configForm.matchRules" :key="rule.__key" class="rule-row">
                <div class="rule-row-head"><strong>规则 {{ index + 1 }}</strong><div><el-switch v-model="rule.enabled" size="small" /><el-button text :icon="Delete" aria-label="删除规则" @click="removeRule(index)" /></div></div>
                <el-input v-model="rule.name" placeholder="规则名称" />
                <el-input v-model="rule.selector" class="selector-input" placeholder='{container_name=~"zero-ai.*"}' />
                <el-input v-model="rule.description" placeholder="规则描述" />
              </div>
              <div v-if="!configForm.matchRules.length" class="config-empty">暂未配置匹配规则</div>
            </div>
          </section>

          <section v-if="recommendedRules.length" class="recommend-section">
            <div class="config-section-heading"><div><h3>推荐规则</h3><span>可按需加入当前配置</span></div></div>
            <div class="recommend-list">
              <div v-for="rule in recommendedRules" :key="`${rule.name}-${rule.selector}`">
                <div><strong>{{ rule.name || '推荐规则' }}</strong><code>{{ rule.selector }}</code><span>{{ rule.reason || rule.description || '根据业务拓扑资源推荐' }}</span></div>
                <el-button size="small" :disabled="hasRule(rule.selector)" @click="useRecommendedRule(rule)">{{ hasRule(rule.selector) ? '已添加' : '使用' }}</el-button>
              </div>
            </div>
          </section>
        </el-form>
      </div>
      <template #footer><el-button @click="configVisible = false">取消</el-button><el-button type="primary" :loading="configSaving" @click="saveConfig">保存配置</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, Bell, BellFilled, Close, Connection, Delete, Plus, Refresh, Setting, Warning, WarningFilled, WarnTriangleFilled } from '@element-plus/icons-vue'
import { getBusinessMonitorAlerts, getBusinessMonitorAlertConfig, saveBusinessMonitorAlertConfig } from '@/api/cmdbBusinessModel'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const loadError = ref('')
const pageData = ref({})
const selectedRecord = ref(null)
const selectedKind = ref('alert')
const nodeStatusFilter = ref('all')
const configVisible = ref(false)
const configLoading = ref(false)
const configSaving = ref(false)
const datasourceOptions = ref([])
const recommendedRules = ref([])
let ruleKey = 0

const query = reactive({ page: 1, pageSize: 20, severity: String(route.query.severity || '') })
const configForm = reactive({ enabled: true, datasourceId: '', matchRules: [] })
const businessId = computed(() => route.params.id)
const windowQuery = computed(() => route.query.window ? { window: route.query.window } : {})
const summary = computed(() => pageData.value.summary || {})
const datasource = computed(() => pageData.value.datasource || pageData.value.config?.datasource || {})
const activeAlerts = computed(() => arrayValue(pageData.value.activeAlerts || pageData.value.alerts))
const rawCurrentMetrics = computed(() => arrayValue(pageData.value.currentMetrics))
const pressureOverview = computed(() => pageData.value.pressureOverview || pageData.value.monitorPressureOverview || {})
const currentMetrics = computed(() => [...rawCurrentMetrics.value].sort(metricSorter))
const metricNodes = computed(() => {
  const backendNodes = arrayValue(pageData.value.metricNodes)
  if (backendNodes.length) return backendNodes
  return buildPressureNodes(rawCurrentMetrics.value)
})
const currentMetricSummary = computed(() => ({ total: Number(pageData.value.currentMetricSummary?.total) || currentMetrics.value.length, critical: Number(pageData.value.currentMetricSummary?.critical) || 0, warning: Number(pageData.value.currentMetricSummary?.warning) || 0, firing: Number(pageData.value.currentMetricSummary?.firing) || 0, healthy: Number(pageData.value.currentMetricSummary?.healthy) || 0, noData: Number(pageData.value.currentMetricSummary?.noData) || 0, queryFailed: Number(pageData.value.currentMetricSummary?.queryFailed) || 0 }))
const matchRules = computed(() => arrayValue(pageData.value.matchRules || pageData.value.config?.matchRules))
const diagnosis = computed(() => pageData.value.diagnosis || {})
const metricCoverage = computed(() => pageData.value.metricCoverage || {})
const coverageRules = computed(() => arrayValue(metricCoverage.value.rules))
const coverageNodes = computed(() => {
  if (Array.isArray(pageData.value.coverageNodes)) return pageData.value.coverageNodes
  const nodesById = new Map()
  coverageRules.value.forEach(rule => arrayValue(rule.matchedNodes).forEach(node => {
    const key = String(node.resourceId ?? node.id ?? node.name)
    if (!nodesById.has(key)) nodesById.set(key, node)
  }))
  return [...nodesById.values()]
})
const coverageNodeCount = computed(() => Number(metricCoverage.value.matchedNodeCount ?? coverageNodes.value.length) || 0)
const coverageSeriesCount = computed(() => Number(metricCoverage.value.totalSeries) || 0)
const businessNodes = computed(() => buildBusinessNodes(rawCurrentMetrics.value, [
  ...coverageNodes.value,
  ...arrayValue(pageData.value.nodes || pageData.value.businessNodes || pageData.value.affectedNodes),
  ...metricNodes.value,
  ...arrayValue(pageData.value.alertNodes)
]))
const affectedNodes = computed(() => businessNodes.value.filter(node => ['warning', 'critical', 'error', 'warn'].includes(nodeHealth(node)) || nodeAlertCount(node)))
const visibleBusinessNodes = computed(() => businessNodes.value.filter(node => nodeStatusFilter.value === 'all' || nodeFilterStatus(node) === nodeStatusFilter.value))
const pagination = computed(() => ({
  page: Number(pageData.value.pagination?.page || query.page),
  pageSize: Number(pageData.value.pagination?.pageSize || query.pageSize),
  total: Number(pageData.value.pagination?.total ?? pageData.value.total ?? activeAlerts.value.length)
}))
const generatedAt = computed(() => pageData.value.generatedAt || pageData.value.generated_at || '')
const warningMessages = computed(() => arrayValue(pageData.value.warnings).map(item => typeof item === 'string' ? item : item?.message).filter(Boolean))
const visibleWarningMessages = computed(() => warningMessages.value.filter(message => !/(series|覆盖样例|截断|抽样)/i.test(message)).map(message => /查询.*失败/.test(message) ? '部分监控数据暂不可用，请稍后刷新' : message))
const businessTitle = computed(() => summary.value.displayName || summary.value.businessName || summary.value.name || `业务 ${businessId.value}`)
const datasourceName = computed(() => datasource.value.name || datasource.value.datasourceName || pageData.value.config?.datasourceName || '未绑定告警源')
const enabledRuleCount = computed(() => matchRules.value.filter(item => item.enabled !== false && Number(item.enabled) !== 0).length)
const rawDataStatus = computed(() => typeof pageData.value.dataStatus === 'object' ? pageData.value.dataStatus.status : pageData.value.dataStatus)
const currentMetricHealth = computed(() => {
  if (currentMetricSummary.value.critical || currentMetricSummary.value.firing || currentMetrics.value.some(metric => ['critical', 'firing'].includes(metricStatus(metric)))) return 'critical'
  if (currentMetricSummary.value.warning || currentMetrics.value.some(metric => metricStatus(metric) === 'warning')) return 'warning'
  if (currentMetrics.value.some(metric => metricStatus(metric) === 'query_failed')) return 'warning'
  return ''
})
const businessHealth = computed(() => currentMetricHealth.value || String(summary.value.healthStatus || diagnosis.value.healthStatus || 'healthy').toLowerCase())
const businessHealthTone = computed(() => businessHealth.value === 'critical' ? 'critical' : businessHealth.value === 'warning' ? 'warning' : 'healthy')
const businessStatusText = computed(() => ({ critical: '严重压力', warning: '存在压力', healthy: '运行正常' })[businessHealth.value] || '状态未知')
const pressurePrimaryIssue = computed(() => pressureOverview.value.primaryIssue || pressureOverview.value.primaryMetric || pressureOverview.value.mainIssue || null)
const primaryMetric = computed(() => findMetricRecord(pressurePrimaryIssue.value) || currentMetrics.value.find(metric => ['critical', 'warning', 'firing'].includes(metricStatus(metric))) || currentMetrics.value[0] || null)
const businessConclusion = computed(() => pressureOverview.value.conclusion || pressureOverview.value.message || diagnosis.value.message || (businessHealth.value === 'critical' ? '当前业务存在严重监控压力' : businessHealth.value === 'warning' ? '当前业务存在监控预警' : '当前业务运行正常'))
const primaryProblemTitle = computed(() => primaryMetric.value ? metricProblemTitle(primaryMetric.value) : '当前没有明显问题')
const primarySuggestedAction = computed(() => primaryMetric.value ? metricSuggestedAction(primaryMetric.value) : '保持关注业务运行状态')
const hasDatasource = computed(() => Boolean(datasource.value.id || datasource.value.datasourceId || pageData.value.config?.datasourceId))
const dataStatusTone = computed(() => currentMetricHealth.value === 'critical' ? 'danger' : (currentMetricHealth.value === 'warning' ? 'warning' : (({ normal: 'normal', ready: 'normal', success: 'normal', no_alerts: 'empty', empty: 'empty', metric_warning: 'warning', metric_critical: 'danger', unconfigured: 'muted', not_configured: 'muted', source_error: 'danger', error: 'danger' })[String(rawDataStatus.value || '').toLowerCase()] || (hasDatasource.value ? 'normal' : 'muted'))))
const dataStatusText = computed(() => currentMetricHealth.value === 'critical' ? '严重监控压力' : (currentMetricHealth.value === 'warning' ? '监控预警' : (({ normal: '数据正常', ready: '数据正常', success: '数据正常', no_alerts: '暂无告警', empty: '暂无告警', metric_warning: '监控预警', metric_critical: '严重监控压力', unconfigured: '未配置', not_configured: '未配置', source_error: '告警源异常', error: '告警源异常' })[String(rawDataStatus.value || '').toLowerCase()] || (hasDatasource.value ? '数据正常' : '未配置'))))

const summaryItems = computed(() => [
  { key: 'status', label: '当前状态', value: businessStatusText.value, caption: '', icon: WarningFilled, tone: businessHealthTone.value },
  { key: 'reason', label: '主要原因', value: primaryMetric.value ? `${metricReasonName(primaryMetric.value)} ${metricCurrentValue(primaryMetric.value, true)}` : '暂无明显压力', caption: '', icon: WarnTriangleFilled, tone: primaryMetric.value ? metricStatus(primaryMetric.value) : 'healthy' },
  { key: 'scope', label: '影响范围', value: `${affectedNodes.value.length} 个工作负载`, caption: '', icon: Connection, tone: affectedNodes.value.length ? businessHealthTone.value : 'healthy' },
  { key: 'alerts', label: '活跃告警', value: `${pagination.value.total} 条`, caption: '', icon: BellFilled, tone: pagination.value.total ? 'warning' : 'muted' }
])
const nodeFilterOptions = [
  { value: 'all', label: '全部' },
  { value: 'critical', label: '异常' },
  { value: 'warning', label: '预警' },
  { value: 'healthy', label: '正常' }
]

const summaryKeys = {
  total: ['totalAlerts', 'activeAlertCount', 'currentAlertCount', 'total'],
  critical: ['criticalAlerts', 'criticalCount', 'critical'],
  warning: ['warningAlerts', 'warningCount', 'warning'],
  nodes: ['affectedNodes', 'affectedNodeCount', 'nodeCount'],
  silenced: ['silencedAlerts', 'silencedCount', 'silenced'],
  inhibited: ['inhibitedAlerts', 'inhibitedCount', 'inhibited', 'suppressed']
}
const summaryCount = key => {
  const found = summaryKeys[key].find(field => summary.value[field] !== undefined && summary.value[field] !== null)
  if (found) return Number(summary.value[found]) || 0
  if (key === 'critical' || key === 'warning') return Number(summary.value.severityCounts?.[key]) || 0
  if (key === 'total') return pagination.value.total
  if (key === 'nodes') return affectedNodes.value.length
  return activeAlerts.value.filter(item => key === 'critical' || key === 'warning' ? severity(item) === key : (key === 'silenced' ? isSilenced(item) : isInhibited(item))).length
}

const normalizeResponse = response => response?.data?.code !== undefined ? response.data : response
const responsePayload = response => {
  const res = normalizeResponse(response)
  if (res?.code !== undefined && ![0, 200].includes(Number(res.code))) throw new Error(res.message || '请求失败')
  return res?.data ?? res ?? {}
}
const arrayValue = value => Array.isArray(value) ? value : []
const objectEntries = value => value && typeof value === 'object' && !Array.isArray(value) ? Object.entries(value) : []
const typeLabel = value => ({ prometheus: 'Prometheus', victoriametrics: 'VictoriaMetrics', builtin: '内置告警源' })[String(value || '').toLowerCase()] || value || '未知类型'
const severity = row => String(row?.severity || row?.level || row?.labels?.severity || row?.labels?.level || 'info').toLowerCase()
const severityText = value => ({ critical: '严重', error: '严重', warning: '警告', warn: '警告', info: '提示', unknown: '未知' })[String(value || '').toLowerCase()] || value || '提示'
const alertName = row => row?.name || row?.alertName || row?.alertname || row?.labels?.alertname || '未命名告警'
const alertKey = row => row?.id || row?.fingerprint || `${alertName(row)}-${firstTriggeredAt(row)}`
const alertDescription = row => row?.description || row?.message || row?.annotations?.description || row?.annotations?.summary || '暂无问题说明'
const alertStatusText = row => ({ firing: '触发中', active: '活跃', resolved: '已恢复', pending: '等待中' })[String(row?.status || row?.state || 'active').toLowerCase()] || row?.status || row?.state || '活跃'
const firstTriggeredAt = row => row?.firstTriggeredAt || row?.firstSeenAt || row?.startsAt || row?.startTime
const isSilenced = row => row?.silenced === true || row?.isSilenced === true || Number(row?.silenced) === 1
const isInhibited = row => row?.inhibited === true || row?.isInhibited === true || Number(row?.inhibited) === 1
const alertCondition = row => row?.condition || row?.triggerCondition || row?.threshold || ''
const alertExpression = row => row?.expression || row?.query || row?.promql || row?.triggerQuery || ''
const matchedRuleText = row => row?.matchedRuleName || row?.matchRule?.name || row?.matchedRule?.ruleName || row?.matchedRule || '-'
const nodeName = row => row?.name || row?.displayName || row?.nodeName || row?.label || `节点 ${row?.id || '-'}`
const nodeType = row => row?.resourceLabel || row?.typeName || row?.nodeType || row?.resourceTypeName || row?.resourceType || '-'
const nodeAlertCount = row => Number(row?.alertCount ?? row?.activeAlertCount ?? arrayValue(row?.activeAlerts || row?.alerts).length) || 0
const nodeMetrics = row => arrayValue(row?.metrics || row?.currentMetrics)
const nodeRiskCount = row => nodeMetrics(row).filter(item => ['warning', 'critical', 'firing'].includes(metricStatus(item))).length + nodeAlertCount(row)
const nodeHealth = row => {
  if (nodeMetrics(row).some(item => ['critical', 'firing'].includes(metricStatus(item)))) return 'critical'
  if (nodeMetrics(row).some(item => metricStatus(item) === 'warning')) return 'warning'
  if (nodeAlerts(row).some(item => ['critical', 'error'].includes(severity(item)))) return 'critical'
  if (nodeAlerts(row).some(item => ['warning', 'warn'].includes(severity(item)))) return 'warning'
  const value = String(row?.healthStatus || row?.status || '').toLowerCase()
  if (['critical', 'error', 'warning', 'warn'].includes(value)) return value
  return 'healthy'
}
const nodeSeverity = row => ['critical', 'error'].includes(nodeHealth(row)) ? 'critical' : (['warning', 'warn'].includes(nodeHealth(row)) ? 'warning' : 'info')
const healthText = value => ({ healthy: '正常', normal: '正常', warning: '风险', warn: '风险', critical: '异常', error: '异常' })[String(value || '').toLowerCase()] || '未知'
const nodeStatusText = row => nodeSeverity(row) === 'critical' ? '严重' : (nodeSeverity(row) === 'warning' ? '预警' : '正常')
const nodeFilterStatus = row => nodeSeverity(row) === 'critical' ? 'critical' : (nodeSeverity(row) === 'warning' ? 'warning' : 'healthy')
const nodeFilterCount = value => value === 'all' ? businessNodes.value.length : businessNodes.value.filter(node => nodeFilterStatus(node) === value).length
const metricEvidenceForNode = row => {
  const rowId = String(row?.resourceId ?? row?.id ?? '')
  for (const metric of nodeMetrics(row)) {
    const matched = metricMatchedNodes(metric).find(node => String(node?.resourceId ?? node?.id ?? '') === rowId)
    if (matched) return matched.evidence?.matchedText || [matched.evidence?.labelKey, matched.evidence?.labelValue].filter(Boolean).join(' = ')
  }
  return ''
}
const nodeEvidence = row => row?.evidence?.matchedText || row?.matchedText || [row?.evidence?.labelKey || row?.labelKey, row?.evidence?.labelValue || row?.labelValue].filter(Boolean).join(' = ') || metricEvidenceForNode(row) || row?.matchReason || (typeof row?.evidence === 'string' ? row.evidence : '') || '-'
const nodeAlerts = row => arrayValue(row?.activeAlerts || row?.alerts)
const nodeMetricSummary = row => {
  const risks = nodeMetrics(row).filter(item => ['warning', 'critical', 'firing'].includes(metricStatus(item)))
  if (!risks.length) return nodeMetrics(row).length ? '实时指标正常' : '-'
  return risks.map(item => `${metricRuleName(item)} ${metricCurrentValue(item, true)}`).join(' · ')
}
function upsertBusinessNode(rows, node) {
  const key = String(node?.resourceId ?? node?.id ?? node?.name ?? node?.displayName ?? '')
  if (!key) return null
  const current = rows.get(key)
  if (current) {
    current.metrics = mergeMetrics(current.metrics, nodeMetrics(node))
    current.activeAlerts = [...current.activeAlerts, ...nodeAlerts(node)]
    if (!current.resourceLabel && node?.resourceLabel) current.resourceLabel = node.resourceLabel
    if (!current.resourceType && node?.resourceType) current.resourceType = node.resourceType
    if (metricStatusRank(nodeHealth(node)) > metricStatusRank(nodeHealth(current))) current.healthStatus = nodeHealth(node)
    return current
  }
  const created = { ...node, metrics: [...nodeMetrics(node)], activeAlerts: [...nodeAlerts(node)] }
  rows.set(key, created)
  return created
}
function mergeMetrics(currentMetricsList, incomingMetrics) {
  const rows = [...arrayValue(currentMetricsList)]
  arrayValue(incomingMetrics).forEach(metric => {
    const metricKey = metric.metricKey || metric.ruleId || metricRuleName(metric)
    const existingIndex = rows.findIndex(item => (item.metricKey || item.ruleId || metricRuleName(item)) === metricKey)
    if (existingIndex < 0) rows.push(metric)
    else {
      const current = rows[existingIndex]
      const samples = uniqueSamples([...arrayValue(current.samples), ...arrayValue(metric.samples)])
      const winner = metricStatusRank(metricStatus(metric)) > metricStatusRank(metricStatus(current)) || Number(metric.currentValue) > Number(current.currentValue) ? metric : current
      rows[existingIndex] = { ...winner, samples, sampleCount: samples.length }
    }
  })
  return rows
}
function uniqueSamples(samples) {
  const rows = new Map()
  arrayValue(samples).forEach(sample => {
    const key = [sample?.timestamp, sample?.value, sample?.metric?.pod || sample?.metric?.pod_name, sample?.metric?.container_name || sample?.metric?.container].map(value => String(value ?? '')).join('|')
    if (!rows.has(key)) rows.set(key, sample)
  })
  return [...rows.values()]
}
function buildBusinessNodes(metrics, nodes) {
  const rows = new Map()
  arrayValue(nodes).forEach(node => upsertBusinessNode(rows, node))
  arrayValue(metrics).forEach(metric => {
    arrayValue(metric.samples).forEach(sample => {
      metricMatchedNodes(sample).forEach(node => {
        const current = upsertBusinessNode(rows, node)
        if (!current) return
        const scopedMetric = { ...metric, currentValue: sample.value ?? metric.currentValue, status: sample.status || metric.status, samples: [sample], sampleCount: 1, matchedNodes: [node] }
        current.metrics = mergeMetrics(current.metrics, [scopedMetric])
        if (metricStatusRank(metricStatus(scopedMetric)) > metricStatusRank(nodeHealth(current))) current.healthStatus = metricStatus(scopedMetric) === 'firing' ? 'critical' : metricStatus(scopedMetric)
      })
    })
  })
  return [...rows.values()].sort((left, right) => metricStatusRank(nodeHealth(right)) - metricStatusRank(nodeHealth(left)) || nodeRiskCount(right) - nodeRiskCount(left) || nodeName(left).localeCompare(nodeName(right), 'zh-CN'))
}
function buildPressureNodes(metrics) {
  const rows = new Map()
  metrics.forEach(metric => {
    arrayValue(metric.samples).filter(sample => ['warning', 'critical', 'firing'].includes(metricStatus(sample))).forEach(sample => {
      metricMatchedNodes(sample).forEach(node => {
        const key = String(node?.resourceId ?? node?.id ?? node?.name ?? '')
        if (!key) return
        const current = rows.get(key) || { ...node, healthStatus: 'healthy', metrics: [], activeAlerts: [] }
        const metricKey = metric.metricKey || metricRuleName(metric)
        const scopedMetric = { ...metric, currentValue: sample.value ?? metric.currentValue, status: sample.status || metric.status, samples: [sample], sampleCount: 1, matchedNodes: [node] }
        const existingIndex = current.metrics.findIndex(item => (item.metricKey || metricRuleName(item)) === metricKey)
        if (existingIndex < 0) current.metrics.push(scopedMetric)
        else if (metricStatusRank(metricStatus(scopedMetric)) > metricStatusRank(metricStatus(current.metrics[existingIndex])) || Number(scopedMetric.currentValue) > Number(current.metrics[existingIndex].currentValue)) current.metrics[existingIndex] = scopedMetric
        if (metricStatusRank(metricStatus(scopedMetric)) > metricStatusRank(current.healthStatus)) current.healthStatus = metricStatus(scopedMetric) === 'firing' ? 'critical' : metricStatus(scopedMetric)
        rows.set(key, current)
      })
    })
  })
  return [...rows.values()]
}
const relatedNodeText = row => {
  const rows = arrayValue(row?.nodes || row?.relatedNodes || row?.affectedNodes)
  if (rows.length) return rows.map(item => typeof item === 'string' ? item : nodeName(item)).join('、')
  return row?.nodeName || row?.relatedNodeName || '-'
}
const formatDateTime = value => {
  if (!value) return '-'
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleString('zh-CN', { hour12: false })
}
const alertDuration = row => {
  if (row?.durationText || row?.duration) return row.durationText || row.duration
  const start = new Date(firstTriggeredAt(row)).getTime()
  if (!start) return '-'
  const seconds = Math.max(0, Math.floor((Date.now() - start) / 1000))
  if (seconds >= 86400) return `${Math.floor(seconds / 86400)}天`
  if (seconds >= 3600) return `${Math.floor(seconds / 3600)}小时`
  if (seconds >= 60) return `${Math.floor(seconds / 60)}分钟`
  return `${seconds}秒`
}
const metricRuleName = row => row?.metricName || row?.ruleName || row?.name || row?.metricKey || '未命名监控指标'
const metricIdentity = row => `${row?.metricKey || ''} ${row?.ruleId || ''} ${row?.category || ''} ${metricRuleName(row)}`.toLowerCase()
const metricShortName = row => {
  const identity = metricIdentity(row)
  if (identity.includes('memory') || identity.includes('内存')) return '内存使用率'
  if (identity.includes('throttle') || identity.includes('限流')) return 'CPU 限流'
  if (identity.includes('restart') || identity.includes('重启')) return 'Pod 重启'
  if (identity.includes('cpu')) return 'CPU 使用量'
  return metricRuleName(row)
}
const metricReasonName = row => ({ '内存使用率': '内存', 'CPU 使用量': 'CPU' })[metricShortName(row)] || metricShortName(row)
const metricStatus = row => String(row?.status || 'no_data').toLowerCase()
const metricStatusRank = value => ({ critical: 5, firing: 5, warning: 4, query_failed: 3, no_data: 2, healthy: 1 })[String(value || '').toLowerCase()] || 0
const metricDisplayRank = row => {
  const name = metricShortName(row)
  if (name === '内存使用率') return 4
  if (name === 'CPU 使用量') return 3
  if (name === 'CPU 限流') return 2
  if (name === 'Pod 重启') return 1
  return 0
}
const metricSorter = (left, right) => metricStatusRank(metricStatus(right)) - metricStatusRank(metricStatus(left)) || metricDisplayRank(right) - metricDisplayRank(left)
const metricMatches = (metric, issue) => {
  if (!metric || !issue) return false
  const metricKeys = [metric.metricKey, metric.ruleId, metric.ruleName, metric.metricName, metric.name].filter(Boolean).map(String)
  const issueKeys = [issue.metricKey, issue.ruleId, issue.ruleName, issue.metricName, issue.name].filter(Boolean).map(String)
  return metricKeys.some(left => issueKeys.some(right => left === right))
}
const findMetricRecord = issue => issue ? currentMetrics.value.find(metric => metricMatches(metric, issue)) || issue : null
const metricStatusText = row => ({ healthy: '正常', warning: '警告', critical: '严重', firing: '已触发', no_data: '无数据', query_failed: '查询失败' })[metricStatus(row)] || metricStatus(row)
const formatMetricNumber = value => Number.isFinite(Number(value)) ? new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 2 }).format(Number(value)) : String(value ?? '-')
const metricUnitText = unit => ({ count: '次', cores: ' cores', '%': '%' })[String(unit || '')] ?? (unit ? ` ${unit}` : '')
const metricCurrentValue = (row, withUnit = false) => {
  const value = row?.currentValue ?? row?.value
  if (value === undefined || value === null || value === '') return '-'
  return `${formatMetricNumber(value)}${withUnit ? metricUnitText(row?.unit) : ''}`
}
const formatMetricThreshold = (value, unit) => value === undefined || value === null || value === '' ? '-' : `${formatMetricNumber(value)}${metricUnitText(unit)}`
const metricThresholdForStatus = row => {
  if (['critical', 'firing'].includes(metricStatus(row))) return Number(row?.criticalThreshold)
  if (metricStatus(row) === 'warning') return Number(row?.warningThreshold)
  return Number(row?.criticalThreshold ?? row?.warningThreshold ?? row?.threshold)
}
const metricOverageText = row => {
  const value = Number(row?.currentValue ?? row?.value)
  const threshold = metricThresholdForStatus(row)
  if (!Number.isFinite(value) || !Number.isFinite(threshold)) return '-'
  const diff = value - threshold
  if (diff <= 0) return '未超过'
  return `+${formatMetricNumber(diff)}${metricUnitText(row?.unit)}`
}
const metricProgress = row => {
  const value = Number(row?.currentValue ?? row?.value)
  const threshold = Number(row?.criticalThreshold ?? row?.warningThreshold ?? row?.threshold)
  if (!Number.isFinite(value)) return 0
  if (!Number.isFinite(threshold) || threshold <= 0) return ['critical', 'warning', 'firing'].includes(metricStatus(row)) ? 100 : 0
  return Math.min(100, Math.max(0, (value / threshold) * 100))
}
const metricThresholdBrief = row => {
  if (row?.criticalThreshold !== undefined && row?.criticalThreshold !== null) return `严重阈值 ${formatMetricThreshold(row.criticalThreshold, row.unit)}`
  if (row?.warningThreshold !== undefined && row?.warningThreshold !== null) return `预警阈值 ${formatMetricThreshold(row.warningThreshold, row.unit)}`
  return row?.thresholdText || '暂未设置阈值'
}
const metricThresholdText = row => {
  if (row?.warningThreshold !== undefined || row?.criticalThreshold !== undefined) return `警告 ${formatMetricThreshold(row.warningThreshold, row.unit)} / 严重 ${formatMetricThreshold(row.criticalThreshold, row.unit)}`
  return row?.thresholdText || [row?.operator, row?.threshold].filter(value => value !== undefined && value !== null && value !== '').join(' ') || '-'
}
const metricQuery = row => row?.query || row?.promql || '-'
const metricMatchedNodes = row => arrayValue(row?.matchedNodes || row?.nodes)
const metricAffectedNodes = row => {
  const nodes = new Map()
  arrayValue(row?.samples).filter(sample => ['warning', 'critical', 'firing'].includes(metricStatus(sample))).forEach(sample => metricMatchedNodes(sample).forEach(node => {
    const key = String(node?.resourceId ?? node?.id ?? node?.name ?? '')
    if (key && !nodes.has(key)) nodes.set(key, node)
  }))
  if (nodes.size) return [...nodes.values()]
  if (['warning', 'critical', 'firing'].includes(metricStatus(row))) return metricMatchedNodes(row)
  return []
}
const metricAffectedNodeText = row => metricAffectedNodes(row).map(nodeName).join('、') || '暂无影响'
const metricProblemTitle = row => {
  if (!row) return '当前没有明显问题'
  if (metricStatus(row) === 'healthy') return `${metricRuleName(row)}正常`
  if (String(row.metricKey || '').includes('restart')) return 'Pod 重启次数过多'
  return `${metricRuleName(row)}过高`
}
const metricSuggestedAction = row => {
  const key = String(row?.metricKey || '').toLowerCase()
  if (key.includes('memory')) return '优先检查高内存 Pod 是否需要扩容，并排查内存泄漏'
  if (key.includes('throttle')) return '检查容器 CPU limit 配置，必要时提高限额或扩容'
  if (key.includes('restart')) return '检查 Pod 事件、容器日志、健康检查与 OOM 记录'
  if (key.includes('cpu')) return '检查高 CPU Pod 的负载与热点任务，必要时扩容'
  return metricStatus(row) === 'healthy' ? '保持观察' : '检查受影响工作负载并评估扩容'
}
const metricRecentSamples = row => [...arrayValue(row?.samples)].sort((left, right) => metricStatusRank(metricStatus(right)) - metricStatusRank(metricStatus(left)) || Number(right.value) - Number(left.value)).slice(0, 8)
const sampleNodeName = sample => metricMatchedNodes(sample).map(nodeName).join('、') || sample?.metric?.pod || sample?.metric?.container || sample?.metric?.instance || '业务节点'
const nodeWorstMetric = row => nodeMetrics(row)
  .filter(item => ['warning', 'critical', 'firing'].includes(metricStatus(item)))
  .sort(metricSorter)[0] || null
const nodeWorstProblemText = row => {
  const metric = nodeWorstMetric(row)
  const sampleText = nodeSampleSummary(row)
  if (!metric) return nodeAlertCount(row) ? `最严重问题：活跃告警 · ${sampleText}` : sampleText
  if (['critical', 'firing'].includes(metricStatus(metric))) return `最严重问题：${metricReasonName(metric)}超过严重阈值 · ${sampleText}`
  if (metricStatus(metric) === 'warning') return `最严重问题：${metricReasonName(metric)}超过预警阈值 · ${sampleText}`
  return sampleText
}
const nodeSampleSummary = row => {
  const samples = nodeMetrics(row).flatMap(item => arrayValue(item.samples))
  const abnormalCount = samples.filter(sample => ['warning', 'critical', 'firing'].includes(metricStatus(sample))).length
  const pods = new Set(samples.map(sample => sample?.metric?.pod || sample?.metric?.pod_name || sample?.metric?.container_name || sample?.metric?.container || sampleNodeName(sample)).filter(Boolean))
  if (!samples.length) return nodeRiskCount(row) ? `${nodeRiskCount(row)} 个风险项` : '无异常'
  return abnormalCount ? `${abnormalCount} 个异常样本 / ${pods.size || samples.length} 个 Pod` : '无异常'
}
const nodeCardSummary = row => {
  const items = [...nodeMetrics(row)]
    .sort(metricSorter)
    .slice(0, 3)
    .map(item => `${metricReasonName(item)} ${metricCurrentValue(item, true)}`)
  if (items.length) return items.join('，')
  if (nodeAlertCount(row)) return `${nodeAlertCount(row)} 条活跃告警`
  return '暂无监控指标'
}

const loadMonitorAlerts = async () => {
  loading.value = true
  loadError.value = ''
  try {
    const params = { page: query.page, pageSize: query.pageSize }
    if (query.severity) params.severity = query.severity
    pageData.value = responsePayload(await getBusinessMonitorAlerts(businessId.value, params))
    await nextTick()
    selectPrimaryIssue()
  } catch (error) {
    console.error('监控告警加载失败:', error)
    loadError.value = error.message || '监控告警加载失败'
  } finally {
    loading.value = false
  }
}
const handlePageSizeChange = () => { query.page = 1; loadMonitorAlerts() }
const selectPrimaryIssue = () => {
  if (primaryMetric.value) selectMetric(primaryMetric.value)
  else clearSelection()
}
const selectNode = row => { selectedRecord.value = row; selectedKind.value = 'node' }
const selectMetric = row => { selectedRecord.value = row; selectedKind.value = 'metric' }
const selectAlert = row => { selectedRecord.value = row; selectedKind.value = 'alert' }
const clearSelection = () => { selectedRecord.value = null }

const datasourceId = item => item?.id ?? item?.datasourceId ?? item?.value ?? ''
const datasourceOptionLabel = item => `${item?.name || item?.label || `数据源 ${datasourceId(item)}`} / ${typeLabel(item?.type || item?.datasourceType)}`
const newRule = rule => ({ __key: ++ruleKey, name: rule?.name || '', selector: rule?.selector || '', enabled: rule?.enabled !== false && Number(rule?.enabled) !== 0, description: rule?.description || '' })
const addRule = () => configForm.matchRules.push(newRule())
const removeRule = index => configForm.matchRules.splice(index, 1)
const hasRule = selector => configForm.matchRules.some(item => item.selector === selector)
const useRecommendedRule = rule => { if (!hasRule(rule.selector)) configForm.matchRules.push(newRule(rule)) }
const openConfigDialog = async () => {
  configVisible.value = true
  configLoading.value = true
  try {
    const data = responsePayload(await getBusinessMonitorAlertConfig(businessId.value))
    const config = data.config || data.currentConfig || data
    configForm.enabled = config.enabled !== false && Number(config.enabled) !== 0
    configForm.datasourceId = config.datasourceId ?? config.datasource?.id ?? ''
    configForm.matchRules = arrayValue(config.matchRules || data.matchRules).map(newRule)
    datasourceOptions.value = arrayValue(data.datasourceOptions || data.datasources || data.availableDatasources || data.options)
    recommendedRules.value = arrayValue(data.recommendedRules || data.recommendations || data.suggestedRules)
  } catch (error) {
    console.error('告警配置加载失败:', error)
    ElMessage.error(error.message || '告警配置加载失败')
  } finally {
    configLoading.value = false
  }
}
const saveConfig = async () => {
  if (configForm.enabled && !configForm.datasourceId) return ElMessage.warning('请选择告警源')
  const invalidRule = configForm.matchRules.find(item => item.enabled && !String(item.selector || '').trim())
  if (invalidRule) return ElMessage.warning('已启用规则的 selector 不能为空')
  configSaving.value = true
  try {
    responsePayload(await saveBusinessMonitorAlertConfig(businessId.value, {
      enabled: configForm.enabled,
      datasourceId: configForm.datasourceId ? Number(configForm.datasourceId) : 0,
      matchRules: configForm.matchRules.map(({ name, selector, enabled, description }) => ({ name: String(name || '').trim(), selector: String(selector || '').trim(), enabled, description: String(description || '').trim() }))
    }))
    ElMessage.success('告警配置已保存')
    configVisible.value = false
    query.page = 1
    await loadMonitorAlerts()
  } catch (error) {
    console.error('告警配置保存失败:', error)
    ElMessage.error(error.message || '告警配置保存失败')
  } finally {
    configSaving.value = false
  }
}

const goDashboard = () => router.push({ path: `/cmdb/service_model/${businessId.value}`, query: windowQuery.value })
const goTopology = () => router.push({ path: `/cmdb/service_model/${businessId.value}/topology` })
const goLogAnalysis = () => router.push({ path: `/cmdb/service_model/${businessId.value}/log-analysis`, query: windowQuery.value })

watch(() => route.params.id, () => { query.page = 1; loadMonitorAlerts() })
onMounted(loadMonitorAlerts)
</script>

<style scoped>
.monitor-alert-page { --page-bg:#f2f6fb; --surface:#fff; --border:#d9e2ef; --text:#172033; --text-secondary:#657287; --muted:#8b97a8; --blue:#2563eb; --blue-soft:#edf4ff; width:100%; height:100%; min-height:0; padding:8px; overflow:auto; box-sizing:border-box; color:var(--text); background:var(--page-bg); font-family:"PingFang SC","Microsoft YaHei","Segoe UI",sans-serif; }
:global(.home-container .el-main:has(.monitor-alert-page)) { padding:0; overflow:hidden; }
.monitor-alert-shell { display:flex; flex-direction:column; width:100%; min-width:1180px; min-height:0; height:100%; overflow:hidden; border:1px solid var(--border); border-radius:7px; background:var(--surface); }
.monitor-alert-header { flex:none; padding:0 18px; border-bottom:1px solid var(--border); background:#fff; }
.header-main-row { display:flex; align-items:center; justify-content:space-between; min-height:58px; gap:24px; }
.header-identity,.header-actions,.business-path,.detail-tabs,.panel-heading,.config-section-heading { display:flex; align-items:center; }
.header-identity { min-width:0; gap:8px; }.business-heading { min-width:0; }.business-path { gap:9px; min-width:0; white-space:nowrap; }
.business-path span,.business-path i { color:var(--text-secondary); font-size:13px; font-style:normal; }.business-path i { color:#b0bac8; }.business-path strong { max-width:320px; overflow:hidden; font-size:17px; text-overflow:ellipsis; }
.business-heading p { margin:4px 0 0; color:var(--text-secondary); font-size:11px; }.data-status { padding:3px 7px; border-radius:4px; font-size:10px; font-style:normal; font-weight:700; }.data-status.is-normal { color:#087a55; background:#e9f8f2; }.data-status.is-empty,.data-status.is-muted { color:#657287; background:#eef2f6; }.data-status.is-warning { color:#a15c00; background:#fff3d8; }.data-status.is-danger { color:#c52828; background:#fff0f0; }
.header-actions { justify-content:flex-end; gap:8px; white-space:nowrap; }.severity-select { width:120px; }.icon-command,.config-command { min-height:34px; border-radius:5px; }.icon-command { width:34px; padding:0; border-color:var(--border); color:#526177; background:#fff; }.icon-command--plain { border:0; background:transparent; }.icon-command:hover { color:var(--blue); border-color:#9db9ef; background:var(--blue-soft); }
.detail-tabs { height:36px; gap:28px; padding-left:43px; }.detail-tabs button { position:relative; align-self:stretch; padding:0 2px; border:0; color:var(--text-secondary); background:transparent; cursor:pointer; font-size:13px; }.detail-tabs button:hover,.detail-tabs button.is-active { color:var(--blue); }.detail-tabs button.is-active { font-weight:700; }.detail-tabs button.is-active::after { position:absolute; right:0; bottom:0; left:0; height:2px; background:var(--blue); content:""; }
.warning-banner,.load-error { display:flex; flex:none; align-items:center; gap:10px; min-height:38px; margin:8px 10px 0; padding:5px 10px; box-sizing:border-box; border:1px solid #edc36a; border-radius:5px; color:#8b5700; background:#fff9e9; font-size:12px; }.load-error { border-color:#efb2b2; color:#a12525; background:#fff4f4; }.load-error div { display:flex; flex:1; gap:8px; }
.monitor-alert-content { display:flex; flex:1; flex-direction:column; min-height:0; gap:8px; padding:8px; overflow:hidden; background:var(--page-bg); }
.summary-strip { display:grid; flex:none; grid-template-columns:repeat(6,minmax(150px,1fr)); min-height:92px; overflow:hidden; border:1px solid var(--border); border-radius:6px; background:#fff; }.summary-strip article { display:flex; align-items:center; gap:11px; min-width:0; padding:12px 14px; border-right:1px solid var(--border); }.summary-strip article:last-child { border-right:0; }.summary-icon { display:grid; flex:none; width:38px; height:38px; place-items:center; border-radius:7px; font-size:19px; }.summary-icon.is-primary { color:#1d5fc4; background:#dce9ff; }.summary-icon.is-critical { color:#c92d3e; background:#fde1e5; }.summary-icon.is-warning { color:#a86200; background:#ffedcc; }.summary-icon.is-healthy { color:#087a55; background:#d9f4e9; }.summary-icon.is-muted { color:#66758a; background:#edf1f6; }.summary-strip article div { display:flex; min-width:0; flex-direction:column; }.summary-strip small { color:var(--text-secondary); font-size:11px; }.summary-strip strong { margin:1px 0; font-size:23px; line-height:1.05; }.summary-strip strong.is-critical { color:#d1253e; }.summary-strip strong.is-warning { color:#b66a00; }.summary-strip em { overflow:hidden; color:var(--muted); font-size:10px; font-style:normal; text-overflow:ellipsis; white-space:nowrap; }
.business-conclusion { grid-template-columns:repeat(4,minmax(220px,1fr)); min-height:100px; }.business-conclusion article { padding:13px 16px; }.business-conclusion strong { overflow:hidden; font-size:16px; line-height:1.35; text-overflow:ellipsis; white-space:nowrap; }.business-conclusion em { margin-top:3px; }
.dashboard-grid { display:grid; flex:1; min-height:0; grid-template-columns:minmax(0,1fr) 320px; gap:8px; }
.dashboard-main { display:flex; min-width:0; min-height:0; flex-direction:column; gap:8px; overflow:auto; }
.primary-problem-card { display:grid; flex:none; grid-template-columns:minmax(280px,.9fr) minmax(440px,1.15fr) minmax(260px,.75fr); gap:16px; align-items:stretch; min-height:132px; padding:16px; border:1px solid var(--border); border-left:4px solid #0c8b60; border-radius:7px; background:#fff; box-shadow:0 8px 20px rgb(31 47 70 / 5%); }
.primary-problem-card.is-critical { border-left-color:#d1253e; background:linear-gradient(90deg,#fff5f6 0%,#fff 42%); }
.primary-problem-card.is-warning { border-left-color:#d98900; background:linear-gradient(90deg,#fff9eb 0%,#fff 42%); }
.primary-problem-card.is-healthy { border-left-color:#0c8b60; background:linear-gradient(90deg,#f0fbf6 0%,#fff 42%); }
.primary-problem-copy { min-width:0; }
.primary-problem-copy span,.primary-problem-action span,.primary-problem-facts span { display:block; margin-bottom:6px; color:var(--muted); font-size:11px; }
.primary-problem-copy h2 { margin:0; color:#162033; font-size:26px; line-height:1.25; }
.primary-problem-copy p { margin:10px 0 0; color:#4e5c71; font-size:13px; line-height:1.6; }
.primary-problem-facts { display:grid; grid-template-columns:1.2fr 1fr .9fr minmax(0,1.25fr); gap:8px; }
.primary-problem-facts > div { min-width:0; padding:11px 12px; border:1px solid #e7edf5; border-radius:6px; background:#f8fafc; }
.primary-problem-facts strong { display:block; overflow:hidden; color:#223048; font-size:15px; line-height:1.45; text-overflow:ellipsis; white-space:nowrap; }
.primary-problem-facts .primary-fact--value { border-color:#f0b7bf; background:#fff6f7; }
.primary-problem-facts .primary-fact--value strong { color:#c52828; font-size:24px; line-height:1.15; }
.primary-problem-card.is-warning .primary-problem-facts .primary-fact--value { border-color:#f1d19a; background:#fffaf0; }
.primary-problem-card.is-warning .primary-problem-facts .primary-fact--value strong { color:#a15c00; }
.primary-problem-card.is-healthy .primary-problem-facts .primary-fact--value { border-color:#b9e4d2; background:#f5fcf9; }
.primary-problem-card.is-healthy .primary-problem-facts .primary-fact--value strong { color:#087a55; }
.primary-problem-action { min-width:0; padding:12px 13px; border:1px solid #dbe6f4; border-radius:6px; background:#f6f9fd; }
.primary-problem-action strong { display:block; color:#27364b; font-size:13px; line-height:1.65; }
.visual-section { flex:none; padding:13px; border:1px solid var(--border); border-radius:7px; background:#fff; }
.visual-section-heading { display:flex; align-items:flex-end; justify-content:space-between; gap:12px; margin-bottom:12px; }
.visual-section-heading > div { display:flex; align-items:baseline; gap:8px; min-width:0; }
.visual-section-heading h3 { margin:0; color:#172033; font-size:14px; }
.visual-section-heading span,.visual-section-heading small { color:var(--muted); font-size:11px; }
.node-section-heading { align-items:center; }
.node-filter { display:flex; flex-wrap:wrap; justify-content:flex-end; gap:6px; }
.node-filter button { height:26px; padding:0 9px; border:1px solid #d7e0ed; border-radius:5px; color:#657287; background:#fff; cursor:pointer; font-size:11px; }
.node-filter button:hover,.node-filter button.active { color:#2563eb; border-color:#9bb8e8; background:#edf4ff; }
.metric-card-grid { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:10px; }
.metric-card { display:flex; min-width:0; min-height:120px; flex-direction:column; align-items:stretch; gap:9px; padding:13px; border:1px solid #e2e9f3; border-radius:7px; background:#fbfcfe; text-align:left; cursor:pointer; transition:border-color .16s ease, box-shadow .16s ease, transform .16s ease; }
.metric-card:hover,.metric-card.active { border-color:#9bb8e8; box-shadow:0 8px 18px rgb(38 88 160 / 10%); transform:translateY(-1px); }
.metric-card-head { display:flex; align-items:center; justify-content:space-between; gap:8px; }
.metric-card-head span { overflow:hidden; color:#34445b; font-size:12px; font-weight:700; text-overflow:ellipsis; white-space:nowrap; }
.metric-card-head em { flex:none; padding:2px 6px; border-radius:4px; color:#657287; background:#eef2f6; font-size:10px; font-style:normal; font-weight:700; }
.metric-card > strong { color:#172033; font-size:24px; line-height:1.1; }
.metric-card > small { overflow:hidden; color:var(--muted); font-size:11px; text-overflow:ellipsis; white-space:nowrap; }
.metric-card.is-critical,.metric-card.is-firing { border-color:#f0b7bf; background:#fff7f8; }
.metric-card.is-warning { border-color:#f1d19a; background:#fffaf0; }
.metric-card.is-healthy { border-color:#b9e4d2; background:#f5fcf9; }
.metric-card.is-query_failed { border-color:#efb2b2; background:#fff4f4; }
.metric-card.is-critical .metric-card-head em,.metric-card.is-firing .metric-card-head em { color:#c52828; background:#ffe5e8; }
.metric-card.is-warning .metric-card-head em { color:#a15c00; background:#fff0cf; }
.metric-card.is-healthy .metric-card-head em { color:#087a55; background:#ddf5eb; }
.metric-progress { height:7px; overflow:hidden; border-radius:999px; background:#e8edf4; }
.metric-progress i { display:block; height:100%; min-width:3px; border-radius:inherit; background:#0c8b60; }
.metric-card.is-critical .metric-progress i,.metric-card.is-firing .metric-progress i,.metric-card.is-query_failed .metric-progress i { background:#d1253e; }
.metric-card.is-warning .metric-progress i { background:#d98900; }
.node-card-list { display:grid; grid-template-columns:repeat(auto-fill,minmax(220px,1fr)); gap:10px; }
.node-card { min-width:0; padding:12px; border:1px solid #e2e9f3; border-left:3px solid #cbd5e1; border-radius:7px; background:#fff; cursor:pointer; transition:border-color .16s ease, box-shadow .16s ease, transform .16s ease, opacity .16s ease; }
.node-card:hover,.node-card.active { border-color:#9bb8e8; box-shadow:0 8px 18px rgb(38 88 160 / 8%); transform:translateY(-1px); }
.node-card.is-critical,.node-card.is-error { border-left-color:#d1253e; }
.node-card.is-warning,.node-card.is-warn { border-left-color:#d98900; }
.node-card.is-healthy,.node-card.is-normal { border-left-color:#16a36f; background:#fbfcfe; opacity:.82; }
.node-card > div { display:flex; align-items:center; justify-content:space-between; gap:8px; }
.node-card strong { display:flex; min-width:0; align-items:center; gap:7px; overflow:hidden; color:#1d2a3d; font-size:13px; text-overflow:ellipsis; white-space:nowrap; }
.node-card strong i { display:block; flex:none; width:7px; height:7px; border-radius:50%; background:#16a36f; }
.node-card strong i.is-critical,.node-card strong i.is-error { background:#d1253e; }
.node-card strong i.is-warning,.node-card strong i.is-warn { background:#d98900; }
.node-card p { min-height:34px; margin:9px 0 5px; color:#526177; font-size:12px; line-height:1.45; }
.node-card > small { display:block; overflow:hidden; color:#34445b; font-size:11px; text-overflow:ellipsis; white-space:nowrap; }
.node-mini-progress { height:5px; margin:8px 0 10px; overflow:hidden; border-radius:999px; background:#e8edf4; }
.node-mini-progress i { display:block; height:100%; min-width:3px; border-radius:inherit; background:#d1253e; }
.node-card.is-warning .node-mini-progress i,.node-card.is-warn .node-mini-progress i { background:#d98900; }
.node-card.is-healthy .node-mini-progress i,.node-card.is-normal .node-mini-progress i { background:#0c8b60; }
.node-card button { padding:0; border:0; color:#2563eb; background:transparent; cursor:pointer; font-size:12px; }
.node-card button:hover { color:#174ea6; }
.visual-empty { display:flex; min-height:88px; align-items:center; justify-content:center; grid-column:1/-1; border:1px dashed #d5deea; border-radius:7px; color:var(--muted); background:#f9fbfd; font-size:12px; }
.detail-mode { flex:none; border:1px solid var(--border); border-radius:7px; background:#fff; }
.detail-mode > summary { display:flex; align-items:center; justify-content:space-between; gap:12px; padding:12px 14px; cursor:pointer; list-style:none; }
.detail-mode > summary::-webkit-details-marker { display:none; }
.detail-mode > summary span { color:#27364b; font-size:13px; font-weight:700; }
.detail-mode > summary small { color:var(--muted); font-size:11px; }
.detail-tables { display:grid; grid-template-rows:280px 260px 420px; gap:8px; padding:0 8px 8px; }
.advanced-fields { margin:8px 0 0; }
.advanced-fields div { display:grid; grid-template-columns:72px minmax(0,1fr); gap:8px; padding:7px 0; border-top:1px solid #edf1f5; font-size:10px; }
.advanced-fields div:first-child { border-top:0; }
.advanced-fields dt { color:var(--muted); }
.advanced-fields dd { min-width:0; margin:0; overflow-wrap:anywhere; color:#44556c; }
.problem-panel { display:grid; flex:none; min-height:112px; grid-template-columns:250px minmax(0,1fr); gap:16px; padding:12px 14px 8px; border:1px solid var(--border); border-radius:6px; background:#fff; }.problem-heading { display:flex; align-items:flex-start; gap:10px; }.problem-mark { display:grid; flex:none; width:34px; height:34px; place-items:center; border-radius:6px; }.problem-mark.is-critical { color:#c52828; background:#fff0f0; }.problem-mark.is-warning { color:#a15c00; background:#fff3d8; }.problem-mark.is-healthy { color:#087a55; background:#e9f8f2; }.problem-heading h2 { margin:0 0 4px; font-size:12px; }.problem-heading strong { color:#27364b; font-size:14px; line-height:1.5; }.problem-facts { display:grid; grid-template-columns:1fr 1fr 1.1fr 1.5fr; gap:0; }.problem-facts > div { min-width:0; padding:4px 13px; border-left:1px solid #e8edf4; }.problem-facts span { display:block; margin-bottom:5px; color:var(--muted); font-size:10px; }.problem-facts strong { display:block; overflow:hidden; color:#34445b; font-size:11px; line-height:1.55; text-overflow:ellipsis; white-space:nowrap; }.problem-facts .problem-action strong { white-space:normal; }.advanced-info { grid-column:1/-1; border-top:1px solid #edf1f5; color:var(--text-secondary); font-size:10px; }.advanced-info summary,.advanced-detail summary { padding:5px 0; color:#657287; cursor:pointer; font-size:10px; }.advanced-info > div { display:flex; flex-wrap:wrap; gap:14px; padding:3px 0; }.advanced-info p { margin:3px 0; color:#8a6500; }
.coverage-strip { display:grid; flex:none; min-height:52px; grid-template-columns:minmax(285px,.85fr) minmax(320px,1fr) minmax(250px,.85fr); align-items:center; gap:16px; padding:7px 12px; border:1px solid #c8d9ef; border-radius:6px; box-sizing:border-box; background:#f5f9ff; }.coverage-summary { display:flex; min-width:0; align-items:center; gap:9px; }.coverage-icon { display:grid; flex:none; width:30px; height:30px; place-items:center; border-radius:5px; color:#2563a6; background:#e2edfd; }.coverage-summary div { display:flex; min-width:0; flex-direction:column; }.coverage-summary strong { overflow:hidden; font-size:11px; text-overflow:ellipsis; white-space:nowrap; }.coverage-summary span { overflow:hidden; color:var(--text-secondary); font-size:10px; text-overflow:ellipsis; white-space:nowrap; }.coverage-rules { display:flex; min-width:0; align-items:center; gap:7px; overflow:hidden; }.coverage-rules > span { display:inline-flex; min-width:0; align-items:center; gap:5px; overflow:hidden; color:#44556c; font-size:10px; text-overflow:ellipsis; white-space:nowrap; }.coverage-rules i { flex:none; width:6px; height:6px; border-radius:50%; background:#94a3b8; }.coverage-rules i.is-ready { background:#16a36f; }.coverage-rules em { flex:none; padding:2px 5px; border-radius:3px; color:#6b7481; background:#e8edf4; font-size:9px; font-style:normal; }.coverage-nodes { display:flex; min-width:0; justify-content:flex-end; gap:5px; overflow:hidden; }.coverage-nodes button { max-width:105px; overflow:hidden; padding:3px 6px; border:1px solid #cbd9eb; border-radius:4px; color:#315b91; background:#fff; cursor:pointer; font-size:9px; text-overflow:ellipsis; white-space:nowrap; }.coverage-nodes button:hover { border-color:#8eafe0; background:#edf4ff; }.coverage-nodes > span { align-self:center; color:var(--text-secondary); font-size:9px; }
.workspace-grid { display:grid; flex:1; min-height:0; grid-template-columns:minmax(0,1fr) 320px; gap:8px; }.workspace-main { display:grid; min-height:0; grid-template-rows:235px 220px minmax(300px,1fr); gap:8px; overflow:auto; }.data-panel,.detail-panel { min-height:0; overflow:hidden; border:1px solid var(--border); border-radius:6px; background:#fff; }.panel-heading { justify-content:space-between; min-height:43px; padding:0 11px; border-bottom:1px solid var(--border); }.panel-heading > div:first-child { display:flex; align-items:baseline; gap:8px; min-width:0; }.panel-heading h3 { margin:0; font-size:13px; }.panel-heading span { color:var(--text-secondary); font-size:10px; }.source-summary { white-space:nowrap; }.primary-cell { font-size:12px; }.metric-summary { display:flex; gap:10px; }.metric-summary span { font-size:10px; }.metric-summary .is-critical,.metric-status.is-critical { color:#c52828; }.metric-summary .is-warning,.metric-status.is-warning { color:#a15c00; }.metric-summary .is-firing,.metric-status.is-firing { color:#c52828; }.metric-summary .is-healthy,.metric-status.is-healthy { color:#0c8b60; }.metric-summary .is-muted,.metric-status.is-no_data { color:#7a8798; }.metric-summary .is-danger,.metric-status.is-query_failed { color:#c52828; }.metric-status { font-size:10px; font-weight:700; }.metric-value { font-size:12px; }.metric-value.is-critical,.metric-value.is-firing { color:#c52828; }.metric-value.is-warning { color:#a15c00; }.metric-value.is-healthy { color:#0c8b60; }.metric-value.is-no_data { color:#7a8798; }.metric-value.is-query_failed { color:#c52828; }.metrics-panel { display:flex; flex-direction:column; }.metrics-panel :deep(.el-table) { flex:1; }.severity-tag { display:inline-flex; min-width:42px; justify-content:center; padding:2px 6px; border-radius:4px; font-size:10px; font-weight:700; }.severity-tag.is-critical,.severity-tag.is-error { color:#c52828; background:#fff0f0; }.severity-tag.is-warning,.severity-tag.is-warn { color:#a15c00; background:#fff3d8; }.severity-tag.is-info { color:#2563a6; background:#eaf3ff; }.health-text { font-size:11px; font-weight:700; }.health-text.is-healthy,.health-text.is-normal { color:#0c8b60; }.health-text.is-warning,.health-text.is-warn { color:#b66a00; }.health-text.is-critical,.health-text.is-error { color:#d02b3d; }.severity-legend { display:flex; gap:12px; }.severity-legend .is-critical { color:#c52828; }.severity-legend .is-warning { color:#a15c00; }.alerts-panel { display:flex; flex-direction:column; }.alerts-panel :deep(.el-table) { flex:1; }.state-flag { margin-right:3px; padding:2px 4px; border-radius:3px; color:#536277; background:#edf1f6; font-size:9px; }.pagination-bar { display:flex; flex:none; height:40px; align-items:center; justify-content:flex-end; padding:0 10px; border-top:1px solid var(--border); }
.detail-panel { display:flex; flex-direction:column; }.detail-panel .panel-heading button { display:grid; width:28px; height:28px; padding:0; place-items:center; border:0; border-radius:4px; color:var(--text-secondary); background:transparent; cursor:pointer; }.detail-panel .panel-heading button:hover { color:var(--blue); background:var(--blue-soft); }.detail-empty { display:flex; flex:1; align-items:center; justify-content:center; flex-direction:column; color:var(--muted); }.detail-empty .el-icon { margin-bottom:12px; font-size:30px; }.detail-empty strong { color:#536177; font-size:13px; }.detail-empty span { margin-top:5px; font-size:11px; }.detail-scroll { flex:1; min-height:0; padding:13px; overflow:auto; }.detail-title { display:flex; align-items:flex-start; gap:8px; }.detail-title strong { flex:1; font-size:14px; line-height:1.5; }.problem-summary { margin:10px 0; padding:9px; border-left:3px solid #e05252; color:#536177; background:#fff6f6; font-size:11px; line-height:1.6; }.detail-fields { margin:12px 0; }.detail-fields div { display:grid; grid-template-columns:88px minmax(0,1fr); padding:7px 0; border-bottom:1px solid #edf1f5; font-size:11px; }.detail-fields dt { color:var(--muted); }.detail-fields dd { min-width:0; margin:0; overflow-wrap:anywhere; }.detail-section { padding:10px 0; border-top:1px solid var(--border); }.detail-section h4 { margin:0 0 7px; font-size:11px; }.detail-section p { margin:0; color:#526177; font-size:11px; line-height:1.6; overflow-wrap:anywhere; }.expression-code { display:block; padding:8px; border-radius:4px; color:#2b4366; background:#f4f7fb; font-size:10px; line-height:1.5; overflow-wrap:anywhere; }.recent-values { display:flex; flex-direction:column; }.recent-values > div { display:grid; grid-template-columns:minmax(0,1fr) auto; gap:3px 8px; padding:7px 0; border-top:1px solid #edf1f5; }.recent-values > div:first-child { border-top:0; }.recent-values span { overflow:hidden; color:#44556c; font-size:10px; text-overflow:ellipsis; white-space:nowrap; }.recent-values strong { font-size:11px; }.recent-values strong.is-critical,.recent-values strong.is-firing { color:#c52828; }.recent-values strong.is-warning { color:#a15c00; }.recent-values strong.is-healthy { color:#087a55; }.recent-values em { grid-column:1/-1; color:var(--muted); font-size:9px; font-style:normal; }.advanced-detail { margin-top:4px; border-top:1px solid var(--border); }.kv-list { display:flex; flex-direction:column; gap:5px; }.kv-list div { display:grid; grid-template-columns:minmax(72px,.45fr) minmax(0,1fr); gap:7px; font-size:10px; }.kv-list span { color:var(--muted); overflow-wrap:anywhere; }.kv-list code { color:#2d4a70; overflow-wrap:anywhere; }.link-list { display:flex; flex-direction:column; gap:5px; }.link-list button { display:flex; flex-direction:column; padding:5px 7px; border:1px solid var(--border); border-radius:4px; color:#31588f; background:#f8faff; text-align:left; cursor:pointer; }.link-list button small { margin-top:2px; overflow:hidden; color:var(--muted); font-size:9px; text-overflow:ellipsis; white-space:nowrap; }
.config-body { min-height:250px; }.config-form :deep(.el-form-item) { margin-bottom:12px; }.config-basics { display:grid; grid-template-columns:210px minmax(0,1fr); gap:16px; }.rule-section,.recommend-section { margin-top:5px; padding-top:14px; border-top:1px solid var(--border); }.config-section-heading { justify-content:space-between; margin-bottom:10px; }.config-section-heading > div { display:flex; align-items:baseline; gap:8px; }.config-section-heading h3 { margin:0; font-size:14px; }.config-section-heading span { color:var(--muted); font-size:11px; }.rule-list { display:flex; max-height:330px; flex-direction:column; gap:8px; overflow:auto; }.rule-row { display:grid; grid-template-columns:150px minmax(250px,1.3fr) minmax(180px,1fr); gap:8px; padding:9px; border:1px solid var(--border); border-radius:6px; background:#f8fafc; }.rule-row-head { display:flex; grid-column:1/-1; align-items:center; justify-content:space-between; }.rule-row-head > div { display:flex; align-items:center; gap:5px; }.selector-input :deep(.el-input__inner) { font-family:"Cascadia Mono","Consolas",monospace; }.config-empty { padding:24px; color:var(--muted); text-align:center; font-size:12px; }.recommend-list { display:flex; flex-direction:column; }.recommend-list > div { display:flex; align-items:center; justify-content:space-between; gap:16px; padding:9px 2px; border-top:1px solid #edf1f5; }.recommend-list > div:first-child { border-top:0; }.recommend-list > div > div { display:grid; min-width:0; grid-template-columns:150px minmax(250px,1fr) minmax(180px,1fr); gap:10px; align-items:center; }.recommend-list strong { font-size:11px; }.recommend-list code { color:#2d4a70; font-size:10px; overflow-wrap:anywhere; }.recommend-list span { color:var(--muted); font-size:10px; }
:deep(.el-table) { --el-table-header-bg-color:#f5f7fa; --el-table-row-hover-bg-color:#f2f7ff; color:#334155; font-size:11px; }:deep(.el-table th.el-table__cell) { height:34px; color:#5d6b7e; font-weight:600; }:deep(.el-table td.el-table__cell) { height:34px; padding:3px 0; }
@media (max-width:900px) { .monitor-alert-page { padding:0; }.monitor-alert-shell { min-width:0; border:0; border-radius:0; overflow:auto; }.monitor-alert-header { position:sticky; top:0; z-index:10; padding:0 10px; }.header-main-row { min-height:58px; }.business-path > span,.business-path > i,.business-heading p { display:none; }.business-path strong { max-width:140px; font-size:14px; }.config-command span { display:none; }.detail-tabs { gap:20px; padding-left:42px; overflow-x:auto; }.detail-tabs button { flex:none; }.monitor-alert-content { overflow:visible; }.summary-strip,.business-conclusion { grid-template-columns:repeat(2,minmax(145px,1fr)); }.summary-strip article { border-bottom:1px solid var(--border); }.problem-panel { grid-template-columns:1fr; }.problem-facts { grid-template-columns:1fr 1fr; }.problem-facts > div { border-left:0; border-top:1px solid #edf1f5; }.advanced-info { grid-column:1; }.workspace-grid { display:flex; flex-direction:column; }.workspace-main { display:flex; flex-direction:column; overflow:visible; }.metrics-panel { height:300px; }.affected-panel { height:300px; }.alerts-panel { height:520px; }.detail-panel { min-height:420px; }.config-basics,.rule-row,.recommend-list > div > div { grid-template-columns:1fr; }.rule-row-head { grid-column:1; } }
@media (max-width:1280px) { .primary-problem-card { grid-template-columns:1fr; }.metric-card-grid { grid-template-columns:repeat(2,minmax(0,1fr)); } }
@media (max-width:900px) { .dashboard-grid { display:flex; flex-direction:column; }.dashboard-main { overflow:visible; }.primary-problem-card { min-height:0; padding:13px; }.primary-problem-copy h2 { font-size:20px; }.primary-problem-facts { grid-template-columns:1fr; }.metric-card-grid,.node-card-list { grid-template-columns:1fr; }.detail-mode > summary { align-items:flex-start; flex-direction:column; }.detail-tables { grid-template-rows:300px 300px 520px; }.advanced-fields div { grid-template-columns:1fr; }.detail-panel { min-height:420px; } }
</style>
