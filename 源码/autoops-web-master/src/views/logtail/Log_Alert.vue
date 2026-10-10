<template>
  <div class="log-alert-page">
    <section class="page-card">
      <header class="page-header">
        <div class="page-header-copy">
          <span class="ops-page-header__icon" aria-hidden="true">
            <img :src="logSvg" alt="">
          </span>
          <div class="ops-page-header__content">
            <h1>日志告警</h1>
            <span class="ops-page-header__divider" aria-hidden="true"></span>
            <p class="page-description">基于 VictoriaLogs LogsQL 配置日志告警，并统一查看活跃告警与历史告警。</p>
          </div>
        </div>
        <div class="summary-group">
          <div class="summary-chip">
            <span>规则</span>
            <strong>{{ pagination.rules.total }}</strong>
          </div>
          <div class="summary-chip">
            <span>活跃</span>
            <strong>{{ pagination.active.total }}</strong>
          </div>
          <div class="summary-chip">
            <span>历史</span>
            <strong>{{ pagination.history.total }}</strong>
          </div>
        </div>
      </header>

      <el-tabs v-model="activeTab" class="alert-tabs" @tab-change="handleTabChange">
        <el-tab-pane label="告警规则" name="rules" />
        <el-tab-pane label="活跃告警" name="active" />
        <el-tab-pane label="告警历史" name="history" />
      </el-tabs>

      <section v-if="activeTab === 'rules'" class="content-panel">
        <div class="toolbar-card">
          <el-form :model="ruleQuery" inline class="toolbar-form" @submit.prevent>
            <el-form-item label="关键字">
              <el-input v-model.trim="ruleQuery.keyword" placeholder="规则名称 / 查询语句" clearable @keyup.enter="searchRules" />
            </el-form-item>
            <el-form-item label="分组">
              <el-select v-model="ruleQuery.groupId" placeholder="全部分组" clearable filterable @change="searchRules">
                <el-option v-for="item in ruleGroupOptions" :key="item.id" :label="item.name" :value="item.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="级别">
              <el-select v-model="ruleQuery.severity" placeholder="全部" clearable @change="searchRules">
                <el-option v-for="item in severityOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
            <el-form-item label="状态">
              <el-select v-model="ruleQuery.enabled" placeholder="全部" clearable @change="searchRules">
                <el-option label="启用" value="1" />
                <el-option label="禁用" value="0" />
              </el-select>
            </el-form-item>
            <el-form-item>
              <div class="toolbar-actions">
                <el-button type="primary" size="small" :icon="Search" :loading="loading.rules" @click="searchRules">搜索</el-button>
                <el-button type="warning" size="small" :icon="Refresh" @click="resetRules">重置</el-button>
                <el-button type="success" size="small" :icon="Plus" @click="openRuleDialog()">新增规则</el-button>
              </div>
            </el-form-item>
          </el-form>
        </div>

        <div class="table-shell">
          <el-table v-loading="loading.rules" :data="rows.rules" class="data-table" height="100%" empty-text="暂无日志告警规则">
            <el-table-column prop="id" label="ID" width="78" />
            <el-table-column label="规则名称" min-width="220" show-overflow-tooltip>
              <template #default="{ row }">
                <div class="name-cell">
                  <strong>{{ row.name || '-' }}</strong>
                  <span>{{ ruleDescription(row) }}</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="LogsQL" min-width="230" show-overflow-tooltip>
              <template #default="{ row }">
                <span class="mono-text">{{ ruleQueryText(row) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="触发条件" min-width="170">
              <template #default="{ row }">{{ ruleConditionText(row) }}</template>
            </el-table-column>
            <el-table-column label="级别" width="110">
              <template #default="{ row }">
                <el-tag :type="severityTagType(row.severity)" size="small" effect="plain">{{ severityLabel(row.severity) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="启用" width="92">
              <template #default="{ row }">
                <el-switch
                  :model-value="Number(row.enabled) === 1"
                  :loading="statusLoadingId === row.id"
                  active-color="#67c23a"
                  inactive-color="#cbd5e1"
                  @change="value => toggleRuleStatus(row, value)"
                />
              </template>
            </el-table-column>
            <el-table-column label="更新时间" min-width="165" show-overflow-tooltip>
              <template #default="{ row }">{{ formatDateTime(row.updatedAt || row.updated_at || row.createdAt || row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" fixed="right" width="210">
              <template #default="{ row }">
                <div class="operation-links">
                  <el-button link type="primary" :icon="EditPen" @click="openRuleDialog(row)">修改</el-button>
                  <el-button link type="success" :icon="DocumentCopy" @click="cloneRule(row)">克隆</el-button>
                  <el-button link type="danger" :icon="Delete" @click="removeRule(row)">删除</el-button>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div class="pagination-section">
          <el-pagination
            :current-page="pagination.rules.page"
            :page-size="pagination.rules.pageSize"
            :page-sizes="[10, 20, 50, 100]"
            :total="pagination.rules.total"
            layout="total, sizes, prev, pager, next, jumper"
            @size-change="size => handleRuleSizeChange(size)"
            @current-change="page => handleRulePageChange(page)"
          />
        </div>
      </section>

      <section v-else class="content-panel">
        <div class="toolbar-card">
          <el-form :model="currentEventQuery" inline class="toolbar-form" @submit.prevent>
            <el-form-item label="关键字">
              <el-input v-model.trim="currentEventQuery.keyword" placeholder="规则名称 / 条件 / 标签" clearable @keyup.enter="searchEvents(activeTab)" />
            </el-form-item>
            <el-form-item label="规则 ID">
              <el-input v-model.trim="currentEventQuery.ruleId" class="short-input" placeholder="全部" clearable @keyup.enter="searchEvents(activeTab)" />
            </el-form-item>
            <el-form-item label="级别">
              <el-select v-model="currentEventQuery.severity" placeholder="全部" clearable @change="searchEvents(activeTab)">
                <el-option v-for="item in severityOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
            <el-form-item label="状态">
              <el-select v-model="currentEventQuery.status" placeholder="全部" clearable @change="searchEvents(activeTab)">
                <el-option v-for="item in eventStatusOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
            <el-form-item>
              <div class="toolbar-actions">
                <el-button type="primary" size="small" :icon="Search" :loading="loading[activeTab]" @click="searchEvents(activeTab)">搜索</el-button>
                <el-button type="warning" size="small" :icon="Refresh" @click="resetEvents(activeTab)">重置</el-button>
              </div>
            </el-form-item>
          </el-form>
        </div>

        <div class="table-shell">
          <el-table
            v-loading="loading[activeTab]"
            :data="currentEventRows"
            class="data-table"
            height="100%"
            empty-text="暂无日志告警事件"
            @row-click="row => openEventDetail(row, activeTab)"
          >
            <el-table-column prop="id" label="ID" width="86" />
            <el-table-column label="告警事件" min-width="220" show-overflow-tooltip>
              <template #default="{ row }">
                <div class="name-cell">
                  <strong>{{ eventTitle(row) }}</strong>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="级别" width="110">
              <template #default="{ row }">
                <el-tag :type="severityTagType(row.severity || row.level)" size="small" effect="plain">
                  {{ severityLabel(row.severity || row.level) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="状态" width="112">
              <template #default="{ row }">
                <el-tag :type="eventStatusTagType(row, activeTab)" size="small" effect="plain">{{ eventStatusText(row, activeTab) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="命中数" width="92">
              <template #default="{ row }">{{ eventValue(row) }}</template>
            </el-table-column>
            <el-table-column label="触发条件" min-width="360" show-overflow-tooltip>
              <template #default="{ row }">
                <span class="mono-text">{{ eventCondition(row) }}</span>
              </template>
            </el-table-column>
            <el-table-column label="首次触发" min-width="165" show-overflow-tooltip>
              <template #default="{ row }">{{ formatDateTime(row.firstTriggeredAt || row.first_triggered_at || row.triggeredAt || row.startsAt || row.createdAt) }}</template>
            </el-table-column>
            <el-table-column v-if="activeTab === 'active'" label="最近检测" min-width="165" show-overflow-tooltip>
              <template #default="{ row }">{{ formatDateTime(row.lastEvaluatedAt || row.last_evaluated_at || row.updatedAt) }}</template>
            </el-table-column>
            <el-table-column v-else label="恢复时间" min-width="165" show-overflow-tooltip>
              <template #default="{ row }">{{ formatDateTime(row.recoveredAt || row.recovered_at || row.resolvedAt || row.endsAt || row.updatedAt) }}</template>
            </el-table-column>
            <el-table-column label="操作" fixed="right" width="150">
              <template #default="{ row }">
                <div class="operation-links" @click.stop>
                  <el-button link type="primary" :icon="View" @click="openEventDetail(row, activeTab)">详情</el-button>
                  <el-button link type="primary" @click="goLogs(row)">原始日志</el-button>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div class="pagination-section">
          <el-pagination
            :current-page="pagination[activeTab].page"
            :page-size="pagination[activeTab].pageSize"
            :page-sizes="[10, 20, 50, 100]"
            :total="pagination[activeTab].total"
            layout="total, sizes, prev, pager, next, jumper"
            @size-change="size => handleEventSizeChange(activeTab, size)"
            @current-change="page => handleEventPageChange(activeTab, page)"
          />
        </div>
      </section>
    </section>

    <el-dialog
      v-model="ruleDialog.visible"
      :title="ruleDialogTitle"
      width="720px"
      destroy-on-close
      class="log-dialog"
      modal-class="log-alert-dialog-overlay"
      @closed="ruleDialog.preview = null"
    >
      <el-form ref="formRef" :model="ruleDialog.form" :rules="ruleFormRules" label-width="96px">
        <section class="form-section">
          <div class="section-head">
            <h3>基础信息</h3>
          </div>
          <div class="form-grid simple-grid">
            <el-form-item label="规则名称" prop="name">
              <el-input v-model.trim="ruleDialog.form.name" placeholder="最近5分钟 error 日志超过10条" />
            </el-form-item>
            <el-form-item label="数据源">
              <el-input :model-value="fixedDatasourceLabel" disabled />
            </el-form-item>
            <el-form-item label="项目" prop="projectId">
              <el-select
                v-model="ruleDialog.form.projectId"
                placeholder="选择项目"
                filterable
                :loading="projectLoading"
                @change="handleRuleProjectChange"
              >
                <el-option
                  v-for="item in projectOptions"
                  :key="item.id"
                  :label="projectOptionLabel(item)"
                  :value="item.id"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="日志库" prop="storeId">
              <el-select
                v-model="ruleDialog.form.storeId"
                placeholder="选择日志库"
                filterable
                :disabled="!ruleDialog.form.projectId"
                :loading="storeLoading"
                @change="handleRuleStoreChange"
              >
                <el-option
                  v-for="item in storeOptions"
                  :key="item.id"
                  :label="storeOptionLabel(item)"
                  :value="item.id"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="是否启用">
              <el-switch v-model="ruleDialog.form.enabled" :active-value="1" :inactive-value="0" active-text="启用" inactive-text="禁用" />
            </el-form-item>
          </div>
        </section>

        <section class="form-section rule-config-section">
          <div class="section-head">
            <h3>规则配置</h3>
            <span>配置查询语句、统计窗口和触发条件</span>
          </div>
          <div class="compact-rule-editor">
            <div class="compact-rule-line logsql-line">
              <span class="compact-rule-label is-required">LogsQL</span>
              <el-form-item prop="query" label-width="0" class="compact-rule-field logsql-field">
                <el-input
                  v-model.trim="ruleDialog.form.query"
                  type="textarea"
                  :rows="2"
                  placeholder="error 或 log_type:db_slow_log query_time:>10"
                />
              </el-form-item>
            </div>
            <div class="compact-rule-line">
              <span class="compact-rule-label is-required">告警等级</span>
              <el-form-item prop="severity" label-width="0" class="compact-rule-field severity-field">
                <el-select v-model="ruleDialog.form.severity">
                  <el-option v-for="item in severityOptions" :key="item.value" :label="item.label" :value="item.value" />
                </el-select>
              </el-form-item>
              <span class="compact-rule-label is-required">统计窗</span>
              <el-form-item prop="windowSeconds" label-width="0" class="compact-rule-field window-field">
                <el-select v-model="ruleDialog.form.windowSeconds">
                  <el-option v-for="item in windowSecondOptions" :key="item.value" :label="item.label" :value="item.value" />
                </el-select>
              </el-form-item>
              <span class="compact-rule-label is-required">检测频率</span>
              <el-form-item prop="evalIntervalSeconds" label-width="0" class="compact-rule-field interval-field">
                <el-select v-model="ruleDialog.form.evalIntervalSeconds">
                  <el-option v-for="item in evalIntervalSecondOptions" :key="item.value" :label="item.label" :value="item.value" />
                </el-select>
              </el-form-item>
            </div>
            <div class="compact-rule-line">
              <span class="compact-rule-label is-required">命中数量</span>
              <span class="condition-prefix">判断条件</span>
              <el-form-item prop="conditionOperator" label-width="0" class="compact-rule-field operator-field">
                <el-select v-model="ruleDialog.form.conditionOperator">
                  <el-option v-for="item in conditionOperatorOptions" :key="item" :label="item" :value="item" />
                </el-select>
              </el-form-item>
              <el-form-item prop="conditionValue" label-width="0" class="compact-rule-field threshold-field">
                <el-input-number v-model="ruleDialog.form.conditionValue" :min="0" :max="100000000" controls-position="right" />
              </el-form-item>
              <span class="condition-suffix">条</span>
              <el-button type="primary" size="small" :icon="Search" :loading="ruleDialog.previewing" @click="previewRule">预览</el-button>
            </div>
            <div v-if="ruleDialog.preview" class="preview-result">
              <span>最近 {{ formatSeconds(ruleDialog.form.windowSeconds) }} 命中 {{ previewCount }} 条</span>
              <el-tag :type="previewWillTrigger ? 'danger' : 'success'" effect="plain">
                {{ previewWillTrigger ? '当前结果会触发告警' : '当前结果不会触发告警' }}
              </el-tag>
              <span class="muted-text">{{ formatDateTime(ruleDialog.preview.executedAt) }}</span>
            </div>
          </div>
        </section>

        <section class="form-section">
          <div class="section-head">
            <h3>通知配置</h3>
          </div>
          <div class="form-grid simple-grid">
            <el-form-item label="通知策略">
              <el-select
                v-model="ruleDialog.form.notifyRuleId"
                placeholder="不发送通知"
                clearable
                filterable
                :loading="notifyRuleLoading"
              >
                <el-option
                  v-for="item in notifyRuleOptions"
                  :key="notifyRuleId(item)"
                  :label="notifyRuleLabel(item)"
                  :value="notifyRuleId(item)"
                />
              </el-select>
            </el-form-item>
          </div>
        </section>
      </el-form>
      <template #footer>
        <el-button @click="ruleDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="ruleDialog.submitting" @click="submitRule">确认</el-button>
      </template>
    </el-dialog>

    <el-drawer
      v-model="detail.visible"
      :title="detailDrawerTitle"
      direction="rtl"
      size="560px"
      class="log-detail-drawer"
      modal-class="log-alert-drawer-overlay"
    >
      <div class="detail-stack" v-loading="detail.loading">
        <section class="detail-panel">
          <h3>{{ eventTitle(detailEvent) }}</h3>
          <p>{{ eventSubtitle(detailEvent) }}</p>
          <div class="detail-tags">
            <el-tag :type="severityTagType(detailEvent.severity || detailEvent.level)" effect="plain">
              {{ severityLabel(detailEvent.severity || detailEvent.level) }}
            </el-tag>
            <el-tag :type="eventStatusTagType(detailEvent, detail.type)" effect="plain">{{ eventStatusText(detailEvent, detail.type) }}</el-tag>
            <el-tag type="info" effect="plain">命中 {{ eventValue(detailEvent) }}</el-tag>
          </div>
        </section>

        <section class="detail-panel">
          <h4>基础信息</h4>
          <el-descriptions :column="1" border>
            <el-descriptions-item label="事件 ID">{{ getEventId(detailEvent) || '-' }}</el-descriptions-item>
            <el-descriptions-item label="规则 ID">{{ detailEvent.ruleId || detailEvent.rule_id || '-' }}</el-descriptions-item>
            <el-descriptions-item label="规则名称">{{ detailEvent.ruleName || detailEvent.rule_name || detailEvent.name || '-' }}</el-descriptions-item>
            <el-descriptions-item label="首次触发">{{ formatDateTime(detailEvent.firstTriggeredAt || detailEvent.first_triggered_at || detailEvent.triggeredAt || detailEvent.startsAt || detailEvent.createdAt) }}</el-descriptions-item>
            <el-descriptions-item label="最近检测">{{ formatDateTime(detailEvent.lastEvaluatedAt || detailEvent.last_evaluated_at || detailEvent.updatedAt) }}</el-descriptions-item>
            <el-descriptions-item label="恢复时间">{{ formatDateTime(detailEvent.recoveredAt || detailEvent.recovered_at || detailEvent.resolvedAt || detailEvent.endsAt) }}</el-descriptions-item>
          </el-descriptions>
        </section>

        <section class="detail-panel">
          <h4>触发条件</h4>
          <pre class="json-view">{{ eventCondition(detailEvent) }}</pre>
        </section>

        <section class="detail-panel">
          <div class="sample-head">
            <h4>样例日志</h4>
            <el-button link type="primary" @click="goSampleLogs">原始日志</el-button>
          </div>
          <div class="sample-meta">
            <span>查询：<strong class="mono-text">{{ sampleLogsView.query || eventQueryText(detailEvent) || '-' }}</strong></span>
            <span>窗口：{{ formatSeconds(sampleLogsView.windowSeconds) }}</span>
            <span>返回：{{ sampleLogRows.length }} / {{ sampleLogsView.count }}</span>
            <span>时间：{{ sampleLogTimeRange }}</span>
          </div>
          <el-alert
            v-if="sampleLogsView.error"
            :title="sampleLogsView.error"
            type="warning"
            show-icon
            :closable="false"
            class="sample-error"
          />
          <div v-if="sampleLogRows.length" class="sample-log-list">
            <article v-for="(item, index) in sampleLogRows" :key="sampleLogKey(item, index)" class="sample-log-item">
              <div class="sample-log-head">
                <span class="mono-text">{{ sampleLogTime(item) }}</span>
                <el-tag size="small" :type="sampleLogLevelTagType(item)" effect="plain">{{ sampleLogLevel(item) }}</el-tag>
              </div>
              <p class="sample-log-message mono-text">{{ sampleLogMessage(item) }}</p>
              <div v-if="sampleLogFieldRows(item).length" class="sample-log-fields">
                <span v-for="field in sampleLogFieldRows(item)" :key="field.key">
                  {{ field.key }}=<strong>{{ field.value }}</strong>
                </span>
              </div>
            </article>
          </div>
          <el-empty v-else-if="!sampleLogsView.error" description="暂无样例日志" :image-size="64" />
        </section>

        <section class="detail-panel">
          <h4>标签</h4>
          <pre class="json-view">{{ formatJson(detailEvent.labels) }}</pre>
        </section>

        <section class="detail-panel">
          <h4>注解</h4>
          <pre class="json-view">{{ formatJson(detailEvent.annotations) }}</pre>
        </section>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, DocumentCopy, EditPen, Plus, Refresh, Search, View } from '@element-plus/icons-vue'
import alertApi from '@/api/alertmanager'
import logtailApi from '@/api/logtail'
import logSvg from '@/assets/image/日志.svg'
import {
  compactObject,
  formatDateTime,
  normalizeListResult,
  parseSafeJson,
  severityLabel,
  severityTagType,
  unwrapData
} from '@/views/alert/alertCommon'

const LOG_DATASOURCE_TYPE = 'victorialogs'
const DEFAULT_WINDOW_SECONDS = 300
const DEFAULT_EVAL_INTERVAL_SECONDS = 60
const DEFAULT_TIME_ZONE = 'Asia/Shanghai'
const logDatasourceId = ref(0)
const logDatasourceName = ref('VictoriaLogs')
const logDatasourceOptions = ref([])
const logAlertGroupId = ref(0)
const fixedDatasourceLabel = computed(() => {
  const datasourceId = Number(ruleDialog.form?.datasourceId || logDatasourceId.value || 0)
  const datasource = logDatasourceOptions.value.find(item => Number(item?.id || 0) === datasourceId)
  const name = datasource?.name || logDatasourceName.value
  return `${name}${datasourceId ? ` #${datasourceId}` : ''}`
})

const route = useRoute()
const router = useRouter()

const severityOptions = [
  { label: 'P0 严重', value: 'critical' },
  { label: 'P1 警告', value: 'warning' },
  { label: 'P2 提示', value: 'info' }
]
const conditionOperatorOptions = ['>=', '>', '<=', '<', '==', '!=']
const windowSecondOptions = [
  { label: '1 分钟', value: 60 },
  { label: '5 分钟', value: 300 },
  { label: '10 分钟', value: 600 },
  { label: '15 分钟', value: 900 },
  { label: '30 分钟', value: 1800 },
  { label: '1 小时', value: 3600 }
]
const evalIntervalSecondOptions = [
  { label: '30 秒', value: 30 },
  { label: '60 秒', value: 60 },
  { label: '2 分钟', value: 120 },
  { label: '5 分钟', value: 300 },
  { label: '10 分钟', value: 600 },
  { label: '15 分钟', value: 900 }
]
const eventStatusOptions = [
  { label: '触发中', value: 'firing', type: 'danger' },
  { label: '观察中', value: 'pending', type: 'warning' },
  { label: '已恢复', value: 'resolved', type: 'success' },
  { label: '已屏蔽', value: 'silenced', type: 'info' },
  { label: '已抑制', value: 'suppressed', type: 'info' }
]

const activeTab = ref(normalizeTab(route.query.tab))
const ruleGroupOptions = ref([])
const projectOptions = ref([])
const storeOptions = ref([])
const notifyRuleOptions = ref([])
const formRef = ref()
const statusLoadingId = ref(null)
const projectLoading = ref(false)
const storeLoading = ref(false)
const notifyRuleLoading = ref(false)

const rows = reactive({
  rules: [],
  active: [],
  history: []
})
const loading = reactive({
  rules: false,
  active: false,
  history: false
})
const loaded = reactive({
  rules: false,
  active: false,
  history: false
})
const pagination = reactive({
  rules: { page: 1, pageSize: 10, total: 0 },
  active: { page: 1, pageSize: 10, total: 0 },
  history: { page: 1, pageSize: 10, total: 0 }
})

const ruleQuery = reactive({
  keyword: '',
  groupId: '',
  severity: '',
  enabled: ''
})
const eventQuery = reactive({
  active: createEventQuery(),
  history: createEventQuery()
})

const ruleDialog = reactive({
  visible: false,
  submitting: false,
  previewing: false,
  mode: 'create',
  form: createRuleForm(),
  preview: null
})

const detail = reactive({
  visible: false,
  loading: false,
  type: 'active',
  row: null,
  data: {}
})

const currentEventQuery = computed(() => eventQuery[activeTab.value] || eventQuery.active)
const currentEventRows = computed(() => rows[activeTab.value] || [])
const ruleDialogTitle = computed(() => {
  if (ruleDialog.mode === 'edit') return '编辑日志告警规则'
  if (ruleDialog.mode === 'clone') return '克隆日志告警规则'
  return '新增日志告警规则'
})
const previewCount = computed(() => {
  const result = ruleDialog.preview?.result && typeof ruleDialog.preview.result === 'object' ? ruleDialog.preview.result : {}
  return Number(result.count ?? ruleDialog.preview?.count ?? 0)
})
const previewWillTrigger = computed(() => compareThreshold(previewCount.value, ruleDialog.form.conditionOperator, ruleDialog.form.conditionValue))
const detailEvent = computed(() => {
  const data = detail.data && typeof detail.data === 'object' ? detail.data : {}
  const event = data.event && typeof data.event === 'object' ? data.event : {}
  const rootEvent = getEventId(data) || data.ruleName || data.rule_name ? data : {}
  return { ...(detail.row || {}), ...rootEvent, ...event }
})
const detailDrawerTitle = computed(() => detail.type === 'history' ? '历史告警详情' : '活跃告警详情')
const sampleLogsView = computed(() => normalizeSampleLogs(
  detail.data?.sampleLogs ||
  detail.data?.sample_logs ||
  detailEvent.value?.sampleLogs ||
  detailEvent.value?.sample_logs
))
const sampleLogRows = computed(() => sampleLogsView.value.logs.map(normalizeSampleLogRow))
const sampleLogTimeRange = computed(() => {
  if (!sampleLogsView.value.start && !sampleLogsView.value.end) return '-'
  return `${formatDateTime(sampleLogsView.value.start)} - ${formatDateTime(sampleLogsView.value.end)}`
})

const ruleFormRules = {
  name: [{ required: true, message: '请输入规则名称', trigger: 'blur' }],
  projectId: [{ required: true, message: '请选择项目', trigger: 'change' }],
  storeId: [{ required: true, message: '请选择日志库', trigger: 'change' }],
  query: [{ required: true, message: '请输入 LogsQL 查询语句', trigger: 'blur' }],
  severity: [{ required: true, message: '请选择告警等级', trigger: 'change' }],
  windowSeconds: [{ required: true, message: '请选择时间范围', trigger: 'change' }],
  evalIntervalSeconds: [{ required: true, message: '请选择检测频率', trigger: 'change' }],
  conditionOperator: [{ required: true, message: '请选择比较符', trigger: 'change' }],
  conditionValue: [{ required: true, message: '请输入阈值', trigger: 'blur' }]
}

function createEventQuery() {
  return {
    keyword: '',
    ruleId: '',
    severity: '',
    status: ''
  }
}

function normalizeTab(value) {
  return ['rules', 'active', 'history'].includes(String(value || '')) ? String(value) : 'rules'
}

function createRuleForm(row = {}) {
  const ruleConfig = objectValue(row.ruleConfig || row.rule_config)
  const firstQuery = firstRuleConfigQuery(ruleConfig)
  const labels = objectValue(row.labels)
  const annotations = objectValue(row.annotations)
  const datasourceId = numberValue(pickValue(row.datasourceId, row.datasource_id), logDatasourceId.value)
  const notifyRuleIds = normalizeNotifyRuleIds(row)
  const query = pickValue(row.query, firstQuery.query, '')
  const conditionOperator = pickValue(row.conditionOperator, row.condition_operator, firstQuery.operator, '>=')
  const conditionValue = numberValue(pickValue(row.conditionValue, row.condition_value, firstQuery.threshold), 10)
  const projectCode = pickValue(ruleConfig.projectCode, ruleConfig.project_code, row.projectCode, row.project_code, '')
  const storeCode = pickValue(ruleConfig.storeCode, ruleConfig.store_code, row.storeCode, row.store_code, '')

  return {
    id: row.id,
    name: row.name || '',
    note: row.note || row.description || '',
    groupId: pickValue(row.groupId, row.group_id, logAlertGroupId.value, 0),
    projectId: pickValue(row.projectId, row.project_id, ''),
    storeId: pickValue(row.storeId, row.store_id, ''),
    projectCode,
    storeCode,
    datasourceId,
    datasourceType: LOG_DATASOURCE_TYPE,
    query,
    conditionOperator,
    conditionValue,
    evalIntervalSeconds: numberValue(pickValue(row.evalIntervalSeconds, row.eval_interval_seconds), DEFAULT_EVAL_INTERVAL_SECONDS),
    forDurationSeconds: numberValue(pickValue(row.forDurationSeconds, row.for_duration_seconds), 0),
    recoverDurationSeconds: numberValue(pickValue(row.recoverDurationSeconds, row.recover_duration_seconds), 0),
    severity: normalizeSeverityValue(row.severity || firstQuery.severity) || 'warning',
    enabled: numberValue(pickValue(row.enabled, 1), 1),
    timeZone: row.timeZone || row.time_zone || DEFAULT_TIME_ZONE,
    windowSeconds: numberValue(pickValue(ruleConfig.windowSeconds, ruleConfig.window_seconds, firstQuery.windowSeconds, row.windowSeconds, row.window_seconds), DEFAULT_WINDOW_SECONDS),
    labels: { alert_type: 'log', source: LOG_DATASOURCE_TYPE, ...labels },
    annotations,
    notifyRecovered: numberValue(pickValue(row.notifyRecovered, row.notify_recovered), 1),
    notifyRepeatStepMinutes: numberValue(pickValue(row.notifyRepeatStepMinutes, row.notify_repeat_step_minutes), 60),
    notifyMaxNumber: numberValue(pickValue(row.notifyMaxNumber, row.notify_max_number), 0),
    notifyRuleId: pickValue(row.notifyRuleId, row.notify_rule_id, notifyRuleIds[0], ''),
    notifyRuleIds
  }
}

const loadRuleGroups = async () => {
  try {
    const response = await alertApi.getRuleGroups({ page: 1, pageSize: 500 })
    ruleGroupOptions.value = normalizeListResult(response).list
    const logGroup = ruleGroupOptions.value.find(item => String(item?.name || '').trim() === '日志告警')
    if (logGroup) logAlertGroupId.value = Number(logGroup.id || 0)
  } catch (error) {
    console.error('加载规则分组失败:', error)
    ruleGroupOptions.value = []
  }
}

const loadLogDatasource = async () => {
  try {
    const response = await alertApi.getDatasources({ page: 1, pageSize: 500, datasourceType: LOG_DATASOURCE_TYPE, enabled: 1 })
    const list = normalizeListResult(response).list
    logDatasourceOptions.value = list
    const datasource = list.find(item => Number(item?.default || item?.is_default || 0) === 1) || list[0]
    if (datasource?.id) {
      logDatasourceId.value = Number(datasource.id)
      logDatasourceName.value = datasource.name || 'VictoriaLogs'
    }
  } catch (error) {
    console.error('加载 VictoriaLogs 数据源失败:', error)
  }
}

const loadProjectOptions = async () => {
  if (projectOptions.value.length) return
  projectLoading.value = true
  try {
    const response = await logtailApi.getProjects({ page: 1, pageSize: 200, status: 1 })
    projectOptions.value = normalizeListResult(response).list
  } catch (error) {
    console.error('加载日志项目失败:', error)
    projectOptions.value = []
  } finally {
    projectLoading.value = false
  }
}

const loadStoreOptions = async projectId => {
  if (!projectId) {
    storeOptions.value = []
    return
  }
  storeLoading.value = true
  try {
    const response = await logtailApi.getStores({ page: 1, pageSize: 200, projectId, status: 1 })
    storeOptions.value = normalizeListResult(response).list
  } catch (error) {
    console.error('加载日志库失败:', error)
    storeOptions.value = []
  } finally {
    storeLoading.value = false
  }
}

const loadNotifyRuleOptions = async () => {
  if (notifyRuleOptions.value.length) return
  notifyRuleLoading.value = true
  try {
    const response = await alertApi.getNotifyRules({ page: 1, pageSize: 500, enabled: 1 })
    notifyRuleOptions.value = normalizeListResult(response).list
  } catch (error) {
    console.error('加载通知策略失败:', error)
    notifyRuleOptions.value = []
  } finally {
    notifyRuleLoading.value = false
  }
}

const loadRules = async () => {
  loading.rules = true
  try {
    const page = pagination.rules
    const response = await alertApi.getRules(compactObject({
      page: page.page,
      pageSize: page.pageSize,
      datasourceType: LOG_DATASOURCE_TYPE,
      keyword: ruleQuery.keyword,
      groupId: ruleQuery.groupId,
      severity: ruleQuery.severity,
      enabled: ruleQuery.enabled
    }))
    const result = normalizeListResult(response)
    rows.rules = result.list
    page.total = result.total
    loaded.rules = true
  } catch (error) {
    console.error('加载日志告警规则失败:', error)
    rows.rules = []
    pagination.rules.total = 0
  } finally {
    loading.rules = false
  }
}

const loadEvents = async type => {
  const tab = type === 'history' ? 'history' : 'active'
  loading[tab] = true
  try {
    const page = pagination[tab]
    const query = eventQuery[tab]
    const params = compactObject({
      page: page.page,
      pageSize: page.pageSize,
      datasourceType: LOG_DATASOURCE_TYPE,
      ruleId: query.ruleId,
      keyword: query.keyword,
      severity: query.severity,
      status: query.status
    })
    const response = tab === 'history' ? await alertApi.getHistoryEvents(params) : await alertApi.getActiveEvents(params)
    const result = normalizeListResult(response)
    rows[tab] = result.list
    page.total = result.total
    loaded[tab] = true
  } catch (error) {
    console.error(`加载${tab === 'history' ? '历史' : '活跃'}告警失败:`, error)
    rows[tab] = []
    pagination[tab].total = 0
  } finally {
    loading[tab] = false
  }
}

const ensureTabLoaded = tab => {
  const normalized = normalizeTab(tab)
  if (loaded[normalized]) return
  if (normalized === 'rules') loadRules()
  else loadEvents(normalized)
}

const handleTabChange = tabName => {
  const tab = normalizeTab(tabName)
  activeTab.value = tab
  if (route.path === '/logs/alert' && route.query.tab !== tab) {
    router.replace({ path: route.path, query: { ...route.query, tab } })
  }
  ensureTabLoaded(tab)
}

const searchRules = () => {
  pagination.rules.page = 1
  loadRules()
}

const resetRules = () => {
  Object.assign(ruleQuery, { keyword: '', groupId: '', severity: '', enabled: '' })
  pagination.rules.page = 1
  loadRules()
}

const handleRuleSizeChange = size => {
  pagination.rules.pageSize = size
  pagination.rules.page = 1
  loadRules()
}

const handleRulePageChange = page => {
  pagination.rules.page = page
  loadRules()
}

const searchEvents = type => {
  const tab = type === 'history' ? 'history' : 'active'
  pagination[tab].page = 1
  loadEvents(tab)
}

const resetEvents = type => {
  const tab = type === 'history' ? 'history' : 'active'
  Object.assign(eventQuery[tab], createEventQuery())
  pagination[tab].page = 1
  loadEvents(tab)
}

const handleEventSizeChange = (type, size) => {
  const tab = type === 'history' ? 'history' : 'active'
  pagination[tab].pageSize = size
  pagination[tab].page = 1
  loadEvents(tab)
}

const handleEventPageChange = (type, page) => {
  const tab = type === 'history' ? 'history' : 'active'
  pagination[tab].page = page
  loadEvents(tab)
}

const openRuleDialog = async row => {
  ruleDialog.mode = row?.id ? 'edit' : 'create'
  await Promise.all([loadProjectOptions(), loadNotifyRuleOptions()])
  const form = createRuleForm(row || {})
  await hydrateRuleScope(form)
  ruleDialog.form = form
  ruleDialog.preview = null
  ruleDialog.visible = true
}

const cloneRule = async row => {
  ruleDialog.mode = 'clone'
  await Promise.all([loadProjectOptions(), loadNotifyRuleOptions()])
  const form = createRuleForm({
    ...row,
    id: undefined,
    name: `${row.name || '日志告警规则'} Copy`
  })
  await hydrateRuleScope(form)
  ruleDialog.form = form
  ruleDialog.preview = null
  ruleDialog.visible = true
}

const handleRuleProjectChange = async projectId => {
  const project = findProjectOption(projectId, '')
  ruleDialog.form.projectCode = projectCodeOf(project)
  ruleDialog.form.storeId = ''
  ruleDialog.form.storeCode = ''
  ruleDialog.preview = null
  await loadStoreOptions(projectId)
}

const handleRuleStoreChange = storeId => {
  const store = findStoreOption(storeId, '')
  ruleDialog.form.storeCode = storeCodeOf(store)
  ruleDialog.preview = null
}

const previewRule = async () => {
  const form = ruleDialog.form
  if (!form.projectCode) {
    ElMessage.warning('请先选择项目')
    return
  }
  if (!form.storeCode) {
    ElMessage.warning('请先选择日志库')
    return
  }
  if (!String(form.query || '').trim()) {
    ElMessage.warning('请先填写 LogsQL 查询语句')
    return
  }
  ruleDialog.previewing = true
  try {
    const response = await alertApi.previewRuleQuery({
      datasourceId: form.datasourceId || logDatasourceId.value || undefined,
      datasourceType: LOG_DATASOURCE_TYPE,
      projectCode: form.projectCode,
      storeCode: form.storeCode,
      query: String(form.query).trim(),
      windowSeconds: numberValue(form.windowSeconds, DEFAULT_WINDOW_SECONDS)
    })
    ruleDialog.preview = unwrapData(response)
  } finally {
    ruleDialog.previewing = false
  }
}

const submitRule = async () => {
  await formRef.value?.validate()
  if (!ruleDialog.form.projectCode || !ruleDialog.form.storeCode) {
    ElMessage.warning('请选择有效的项目和日志库')
    return
  }
  ruleDialog.submitting = true
  try {
    const payload = buildRulePayload()
    if (payload.id) {
      await alertApi.updateRule(payload.id, payload)
      ElMessage.success('日志告警规则已更新')
    } else {
      await alertApi.createRule(payload)
      ElMessage.success('日志告警规则已创建')
    }
    ruleDialog.visible = false
    await loadRules()
  } finally {
    ruleDialog.submitting = false
  }
}

const buildRulePayload = () => {
  const form = ruleDialog.form
  const query = String(form.query || '').trim()
  const projectCode = form.projectCode || projectCodeOf(findProjectOption(form.projectId, ''))
  const storeCode = form.storeCode || storeCodeOf(findStoreOption(form.storeId, ''))
  const labels = {
    alert_type: 'log',
    source: LOG_DATASOURCE_TYPE,
    ...objectValue(form.labels)
  }
  const annotations = buildDefaultAnnotations(form, query)
  const payload = compactObject({
    id: form.id,
    name: form.name,
    note: form.note,
    groupId: form.groupId,
    datasourceId: form.datasourceId || logDatasourceId.value || undefined,
    datasourceType: LOG_DATASOURCE_TYPE,
    query,
    conditionOperator: form.conditionOperator,
    conditionValue: numberValue(form.conditionValue, 0),
    evalIntervalSeconds: numberValue(form.evalIntervalSeconds, DEFAULT_EVAL_INTERVAL_SECONDS),
    forDurationSeconds: numberValue(form.forDurationSeconds, 0),
    recoverDurationSeconds: numberValue(form.recoverDurationSeconds, 0),
    severity: form.severity,
    enabled: numberValue(form.enabled, 1),
    timeZone: form.timeZone || DEFAULT_TIME_ZONE,
    labels,
    annotations,
    notifyRecovered: numberValue(form.notifyRecovered, 1),
    notifyRepeatStepMinutes: numberValue(form.notifyRepeatStepMinutes, 60),
    notifyMaxNumber: numberValue(form.notifyMaxNumber, 0)
  })
  payload.ruleConfig = {
    projectCode,
    storeCode,
    windowSeconds: numberValue(form.windowSeconds, DEFAULT_WINDOW_SECONDS)
  }
  const selectedNotifyRuleId = pickValue(form.notifyRuleId, Array.isArray(form.notifyRuleIds) ? form.notifyRuleIds[0] : '')
  payload.notifyRuleIds = selectedNotifyRuleId ? [selectedNotifyRuleId] : []
  return payload
}

function buildDefaultAnnotations(form, query) {
  const existing = objectValue(form.annotations)
  const summary = `${form.name || '日志告警'}：${query || '日志查询'} 命中数 ${form.conditionOperator || '>='} ${numberValue(form.conditionValue, 0)}`
  return {
    ...existing,
    summary,
    description: `${summary}，统计窗口：${formatSeconds(form.windowSeconds)}`
  }
}

const toggleRuleStatus = async (row, value) => {
  statusLoadingId.value = row.id
  try {
    await alertApi.updateRuleStatus(row.id, value ? 1 : 0)
    row.enabled = value ? 1 : 0
    ElMessage.success(value ? '日志告警规则已启用' : '日志告警规则已禁用')
  } finally {
    statusLoadingId.value = null
  }
}

const removeRule = async row => {
  try {
    await ElMessageBox.confirm(`确认删除日志告警规则「${row.name || row.id}」吗？`, '删除日志告警规则', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消'
    })
  } catch (error) {
    return
  }
  await alertApi.deleteRule(row.id)
  ElMessage.success('日志告警规则已删除')
  await loadRules()
}

const openEventDetail = async (row, type) => {
  const tab = type === 'history' ? 'history' : 'active'
  const eventId = getEventId(row)
  if (!eventId) {
    ElMessage.warning('当前告警缺少事件 ID，无法查询详情')
    return
  }
  detail.visible = true
  detail.loading = true
  detail.type = tab
  detail.row = row
  detail.data = { event: row }
  try {
    const response = await alertApi.getEventDetail(tab, eventId)
    const payload = unwrapData(response)
    detail.data = payload && typeof payload === 'object' ? payload : { event: row }
  } finally {
    detail.loading = false
  }
}

const goLogs = row => {
  router.push({
    path: '/logs/query',
    query: compactObject({
      q: eventQueryText(row)
    })
  })
}

async function hydrateRuleScope(form) {
  const project = findProjectOption(form.projectId, form.projectCode)
  if (!project) {
    storeOptions.value = []
    return
  }

  form.projectId = project.id
  form.projectCode = projectCodeOf(project)
  await loadStoreOptions(project.id)

  const store = findStoreOption(form.storeId, form.storeCode)
  if (store) {
    form.storeId = store.id
    form.storeCode = storeCodeOf(store)
  }
}

function findProjectOption(id, code) {
  return projectOptions.value.find(item => sameValue(item.id, id) || sameValue(projectCodeOf(item), code))
}

function findStoreOption(id, code) {
  return storeOptions.value.find(item => sameValue(item.id, id) || sameValue(storeCodeOf(item), code))
}

function projectOptionLabel(item) {
  return codeLabel(item?.name, projectCodeOf(item), item?.id)
}

function storeOptionLabel(item) {
  return codeLabel(item?.name, storeCodeOf(item), item?.id)
}

function notifyRuleId(item) {
  return item?.id ?? item?.value ?? item?.notifyRuleId ?? item?.notify_rule_id ?? ''
}

function notifyRuleLabel(item) {
  const id = notifyRuleId(item)
  return item?.name || item?.label || (id ? `策略 ${id}` : '-')
}

function projectCodeOf(item) {
  return item?.code || item?.projectCode || item?.project_code || ''
}

function storeCodeOf(item) {
  return item?.code || item?.storeCode || item?.store_code || ''
}

function codeLabel(name, code, fallback) {
  if (name && code) return `${name} (${code})`
  return name || code || fallback || '-'
}

function sameValue(left, right) {
  return left !== undefined && left !== null && right !== undefined && right !== null && String(left) === String(right)
}

function objectValue(value) {
  const parsed = parseSafeJson(value)
  return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {}
}

function normalizeNotifyRuleIds(row = {}) {
  const raw = pickValue(row.notifyRuleIds, row.notify_rule_ids, row.notifyRuleId, row.notify_rule_id)
  const parsed = parseSafeJson(raw)
  if (Array.isArray(parsed)) return parsed.filter(value => value !== undefined && value !== null && value !== '')
  if (parsed !== undefined && parsed !== null && parsed !== '') return [parsed]
  return []
}

function pickValue(...values) {
  return values.find(value => value !== undefined && value !== null && value !== '')
}

function numberValue(value, fallback = 0) {
  const number = Number(value)
  return Number.isFinite(number) ? number : fallback
}

function normalizeSeverityValue(value) {
  const key = String(value || '').trim().toLowerCase()
  const map = { p0: 'critical', critical: 'critical', serious: 'critical', p1: 'warning', warning: 'warning', warn: 'warning', p2: 'info', info: 'info' }
  return map[key] || ''
}

function firstRuleConfigQuery(ruleConfig) {
  const query = Array.isArray(ruleConfig.queries) ? ruleConfig.queries[0] || {} : {}
  return {
    query: pickValue(query.logsql, query.logql, query.logs_ql, query.query, ''),
    operator: pickValue(query.operator, query.conditionOperator, ''),
    threshold: pickValue(query.threshold, query.value, query.conditionValue, ''),
    windowSeconds: pickValue(query.windowSeconds, query.lookbackSeconds, query.rangeSeconds, query.window, query.lookback, query.range, ''),
    severity: query.severity
  }
}

function ruleConfig(row) {
  return objectValue(row?.ruleConfig || row?.rule_config)
}

function ruleQueryText(row) {
  const config = ruleConfig(row)
  return row?.query || firstRuleConfigQuery(config).query || '-'
}

function ruleWindowSeconds(row) {
  const config = ruleConfig(row)
  return numberValue(pickValue(config.windowSeconds, config.window_seconds, firstRuleConfigQuery(config).windowSeconds, row?.windowSeconds, row?.window_seconds), DEFAULT_WINDOW_SECONDS)
}

function ruleConditionOperator(row) {
  const config = ruleConfig(row)
  return pickValue(row?.conditionOperator, row?.condition_operator, firstRuleConfigQuery(config).operator, '>=')
}

function ruleConditionValue(row) {
  const config = ruleConfig(row)
  return pickValue(row?.conditionValue, row?.condition_value, firstRuleConfigQuery(config).threshold, '-')
}

function ruleConditionText(row) {
  return `最近 ${formatSeconds(ruleWindowSeconds(row))} 内日志数量 ${ruleConditionOperator(row)} ${ruleConditionValue(row)} 条`
}

function ruleDescription(row) {
  const annotations = objectValue(row?.annotations)
  return row?.note || row?.description || annotations.summary || annotations.description || '-'
}

function formatSeconds(value) {
  const seconds = numberValue(value, 0)
  if (seconds <= 0) return '0 秒'
  if (seconds % 3600 === 0) return `${seconds / 3600} 小时`
  if (seconds % 60 === 0) return `${seconds / 60} 分钟`
  return `${seconds} 秒`
}

function compareThreshold(count, operator, threshold) {
  const left = numberValue(count, 0)
  const right = numberValue(threshold, 0)
  if (operator === '>=') return left >= right
  if (operator === '<') return left < right
  if (operator === '<=') return left <= right
  if (operator === '==') return left === right
  if (operator === '!=') return left !== right
  return left > right
}

function getEventId(row) {
  return row?.eventId ?? row?.event_id ?? row?.id ?? row?.fingerprint ?? ''
}

function eventTitle(row) {
  return row?.ruleName || row?.rule_name || row?.alertName || row?.alert_name || row?.name || objectValue(row?.annotations).summary || `事件 ${getEventId(row) || '-'}`
}

function eventSubtitle(row) {
  return eventDescriptionText(row) || eventCondition(row)
}

function eventDescriptionText(row) {
  const annotations = objectValue(row?.annotations)
  return annotations.description || annotations.summary || row?.description || row?.message || row?.summary || ''
}

function eventQueryText(row) {
  const result = objectValue(row?.result)
  const query = pickValue(
    row?.triggerQuery,
    row?.trigger_query,
    row?.query,
    row?.logQuery,
    row?.log_query,
    row?.logsQL,
    row?.logs_ql,
    result.query,
    result.logsql,
    result.logsQL,
    ''
  )
  if (query && String(query).trim() !== '-') return String(query).trim()
  return extractQueryFromEventText(row)
}

function eventCondition(row) {
  const condition = pickValue(row?.triggerCondition, row?.trigger_condition, row?.condition, row?.trigger_cond, '')
  if (condition && String(condition).trim() !== '-') return String(condition).trim()
  const query = eventQueryText(row)
  return query ? `查询：${query}` : '-'
}

function extractQueryFromEventText(row) {
  const text = String(eventDescriptionText(row) || '').trim()
  if (!text) return ''

  const title = String(eventTitle(row) || '').trim()
  if (title && text.startsWith(title)) {
    const candidate = text.slice(title.length).replace(/^[\s:：-]+/, '').trim()
    if (candidate) return candidate
  }

  const markerMatch = text.match(/(?:查询|LogsQL|告警)\s*[:：]\s*(.+)$/i)
  if (markerMatch?.[1]) return markerMatch[1].trim()

  if (/[A-Za-z0-9_./-]+\s*[:=<>!]/.test(text)) return text
  return ''
}

function eventValue(row) {
  return pickValue(row?.value, row?.triggerValue, row?.trigger_value, row?.currentValue, row?.current_value, row?.matchCount, row?.match_count, '-')
}

function eventStatusValue(row, fallbackType = 'active') {
  return String(row?.status || row?.eventStatus || row?.event_status || (fallbackType === 'history' ? 'resolved' : 'firing')).toLowerCase()
}

function eventStatusText(row, fallbackType = 'active') {
  const value = eventStatusValue(row, fallbackType)
  const target = eventStatusOptions.find(item => item.value === value)
  return target?.label || row?.statusName || row?.eventStatusName || value || '-'
}

function eventStatusTagType(row, fallbackType = 'active') {
  const value = eventStatusValue(row, fallbackType)
  const target = eventStatusOptions.find(item => item.value === value)
  return target?.type || (fallbackType === 'history' ? 'success' : 'danger')
}

function formatJson(value) {
  if (!value) return '-'
  const parsed = parseSafeJson(value)
  if (parsed && typeof parsed === 'object') return JSON.stringify(parsed, null, 2)
  return String(parsed)
}

function normalizeSampleLogs(value) {
  const sample = objectValue(value)
  const logs = Array.isArray(sample.logs) ? sample.logs : []
  return {
    query: sample.query || '',
    count: pickValue(sample.count, logs.length, 0),
    limit: pickValue(sample.limit, logs.length, 0),
    start: sample.start || sample.startTime || sample.start_time || '',
    end: sample.end || sample.endTime || sample.end_time || '',
    windowSeconds: numberValue(pickValue(sample.windowSeconds, sample.window_seconds), DEFAULT_WINDOW_SECONDS),
    error: sample.error || '',
    logs
  }
}

function normalizeSampleLogRow(item) {
  const parsed = parseSafeJson(item)
  if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) return parsed
  return { message: String(parsed ?? '') }
}

function sampleLogKey(row, index) {
  return row?._rowKey || row?._time || row?.timestamp || row?.time || row?.id || `${index}-${sampleLogMessage(row).slice(0, 24)}`
}

function sampleLogTime(row) {
  return pickField(row, ['_time', '@timestamp', 'timestamp', 'time', 'datetime'])
}

function sampleLogLevel(row) {
  const level = pickField(row, ['severity_text', 'level', 'severity', 'log_level'])
  return level === '-' ? '-' : String(level).toUpperCase()
}

function sampleLogLevelTagType(row) {
  const level = sampleLogLevel(row).toLowerCase()
  if (['fatal', 'error', 'err', 'critical'].includes(level)) return 'danger'
  if (['warn', 'warning'].includes(level)) return 'warning'
  if (['info', 'notice'].includes(level)) return 'success'
  return 'info'
}

function sampleLogMessage(row) {
  const message = pickField(row, ['message', '_raw', '_msg', 'msg', 'log', 'content', 'raw'])
  if (message !== '-') return compactDisplayValue(message)
  return formatJson(row)
}

function sampleLogFieldRows(row) {
  const hiddenKeys = new Set(['_time', '@timestamp', 'timestamp', 'time', 'datetime', 'severity_text', 'level', 'severity', 'log_level', 'message', '_raw', '_msg', 'msg', 'log', 'content', 'raw'])
  return Object.entries(row || {})
    .filter(([key, value]) => !hiddenKeys.has(key) && value !== undefined && value !== null && value !== '')
    .slice(0, 8)
    .map(([key, value]) => ({ key, value: compactDisplayValue(value) }))
}

function pickField(row, keys) {
  for (const key of keys) {
    const value = row?.[key]
    if (value !== undefined && value !== null && value !== '') return value
  }
  return '-'
}

function compactDisplayValue(value) {
  if (value === undefined || value === null || value === '') return '-'
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

function goSampleLogs() {
  router.push({
    path: '/logs/query',
    query: compactObject({
      q: sampleLogsView.value.query || eventQueryText(detailEvent.value)
    })
  })
}

watch(() => route.query.tab, value => {
  const nextTab = normalizeTab(value)
  if (nextTab !== activeTab.value) {
    activeTab.value = nextTab
    ensureTabLoaded(nextTab)
  }
})

onMounted(async () => {
  await Promise.all([loadRuleGroups(), loadLogDatasource()])
  await Promise.all([loadRules(), loadEvents('active'), loadEvents('history')])
})
</script>

<style scoped>
.log-alert-page {
  --ops-blue-500: #409eff;
  --ops-blue-600: #2563eb;
  --ops-green-500: #10b981;
  --ops-orange-500: #f59e0b;
  --ops-red-500: #ef4444;
  --ops-text-primary: #0f172a;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-bg-surface: #ffffff;
  --ops-bg-surface-soft: #fbfdff;
  --ops-bg-subtle: #f3f7fc;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  --ops-border-inner: #dce8f5;
  --ops-border-hover: #b9d4ff;
  --ops-radius-md: 8px;
  --ops-radius-inner: 10px;
  --ops-radius-card: 12px;
  --ops-radius-dialog: 15px;
  --ops-shadow-card: 0 8px 24px rgba(80, 124, 180, 0.07);
  --ops-shadow-dialog: 0 22px 60px rgba(49, 92, 142, 0.18);
  --log-text-primary: var(--ops-text-primary);
  --log-text-secondary: var(--ops-text-secondary);
  --log-text-muted: var(--ops-text-muted);
  --log-border: var(--ops-border);
  box-sizing: border-box;
  height: calc(100vh - 124px);
  min-height: 0;
  padding: 7px;
  overflow: auto;
  background: transparent;
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

:global(.home-container .el-main:has(.log-alert-page)) {
  padding: 10px;
  overflow: hidden;
}

.page-card {
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--ops-border);
  border-radius: var(--ops-radius-card);
  background: var(--ops-bg-surface);
  box-shadow: var(--ops-shadow-card);
}

.page-card::before {
  content: none;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex: 0 0 auto;
  padding: 8px 12px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-surface);
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
  background: var(--ops-border-soft);
}

.page-header h1 {
  margin: 0;
  flex: 0 0 auto;
  color: var(--ops-text-primary);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
}

.page-description {
  margin: 0;
  overflow: hidden;
  color: var(--ops-text-muted);
  font-size: 12px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.summary-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 0 0 auto;
  justify-content: flex-end;
}

.summary-chip {
  position: relative;
  display: inline-flex;
  min-width: 72px;
  min-height: 34px;
  align-items: center;
  justify-content: space-between;
  gap: 9px;
  box-sizing: border-box;
  padding: 5px 9px;
  border: 1px solid var(--ops-border-soft);
  border-radius: var(--ops-radius-md);
  background: var(--ops-bg-subtle);
}

.summary-chip::before {
  width: 6px;
  height: 6px;
  flex: 0 0 6px;
  border-radius: 999px;
  background: var(--ops-blue-500);
  content: "";
}

.summary-chip:nth-child(2)::before {
  background: var(--ops-red-500);
}

.summary-chip:nth-child(3)::before {
  background: var(--ops-green-500);
}

.summary-chip span {
  color: var(--ops-text-muted);
  font-size: 12px;
}

.summary-chip strong {
  color: var(--ops-text-primary);
  font-size: 16px;
  line-height: 1;
}

.alert-tabs {
  flex: 0 0 auto;
  padding: 0 12px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-surface-soft);
}

.alert-tabs :deep(.el-tabs__header) {
  margin: 0;
}

.alert-tabs :deep(.el-tabs__nav-wrap::after) {
  display: none;
}

.alert-tabs :deep(.el-tabs__active-bar) {
  height: 2px;
  border-radius: 999px;
  background: var(--ops-blue-500);
}

.alert-tabs :deep(.el-tabs__nav-scroll) {
  padding: 0;
}

.alert-tabs :deep(.el-tabs__nav) {
  gap: 24px;
  padding: 0 0 1px;
  border: none;
  background: transparent;
}

.alert-tabs :deep(.el-tabs__item) {
  height: 36px;
  min-width: auto;
  padding: 0;
  color: var(--ops-text-secondary);
  font-size: 13px;
  font-weight: 700;
  line-height: 36px;
  transition: color 0.16s ease;
}

.alert-tabs :deep(.el-tabs__item:hover) {
  color: var(--ops-blue-500);
}

.alert-tabs :deep(.el-tabs__item.is-active) {
  color: var(--ops-blue-600);
}

.content-panel {
  display: flex;
  min-width: 0;
  min-height: 0;
  flex: 1;
  flex-direction: column;
  gap: 8px;
  padding: 8px 10px 10px;
}

.toolbar-card,
.table-shell {
  border: 1px solid var(--ops-border-inner);
  border-radius: var(--ops-radius-md);
  background: var(--ops-bg-surface);
}

.toolbar-card {
  flex: 0 0 auto;
  padding: 8px 10px;
  background: var(--ops-bg-subtle);
}

.toolbar-form {
  display: flex;
  align-items: center;
  gap: 8px 10px;
  flex-wrap: wrap;
}

.toolbar-form :deep(.el-form-item) {
  margin-right: 0;
  margin-bottom: 0;
}

.toolbar-form :deep(.el-form-item__label) {
  height: 30px;
  padding-right: 6px;
  color: var(--ops-text-secondary);
  font-size: 12px;
  font-weight: 700;
  line-height: 30px;
}

.toolbar-form :deep(.el-input),
.toolbar-form :deep(.el-select) {
  width: 176px;
}

.toolbar-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.operation-links {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  flex-wrap: nowrap;
  white-space: nowrap;
}

.short-input {
  width: 112px !important;
}

.table-shell {
  min-height: 0;
  flex: 1;
  overflow: hidden;
}

.data-table {
  height: 100%;
  --el-table-border-color: var(--ops-border-soft);
  --el-table-header-bg-color: var(--ops-bg-subtle);
  --el-table-row-hover-bg-color: #f6faff;
  --el-table-text-color: var(--ops-text-secondary);
  --el-table-header-text-color: #334155;
}

.data-table :deep(.el-table__cell) {
  padding: 5px 0;
  font-size: 12px;
}

.data-table :deep(th.el-table__cell) {
  height: 36px;
  padding: 0;
  font-weight: 700;
}

.data-table :deep(.el-table__body tr) {
  height: 38px;
}

.data-table :deep(.el-tag) {
  border-radius: 999px;
  font-weight: 600;
}

.name-cell strong,
.name-cell span,
.compact-lines span {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.name-cell strong {
  color: var(--ops-text-primary);
  font-size: 13px;
  font-weight: 700;
}

.name-cell span,
.compact-lines span {
  margin-top: 2px;
  color: var(--ops-text-muted);
  font-size: 11px;
}

.mono-text {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
}

.muted-text {
  color: var(--ops-text-muted);
  font-size: 12px;
}

.pagination-section {
  display: flex;
  min-height: 30px;
  align-items: center;
  justify-content: flex-end;
  flex: 0 0 auto;
}

.pagination-section :deep(.el-pagination) {
  --el-pagination-button-bg-color: var(--ops-bg-subtle);
  --el-pagination-hover-color: var(--ops-blue-600);
  font-size: 12px;
}

.form-section {
  min-width: 0;
  padding: 12px 12px 2px;
  margin-bottom: 10px;
  border: 1px solid var(--ops-border-inner);
  border-radius: var(--ops-radius-inner);
  background: var(--ops-bg-surface-soft);
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.section-head h3 {
  margin: 0;
  color: var(--ops-text-primary);
  font-size: 14px;
  font-weight: 700;
}

.section-head span {
  color: var(--ops-text-muted);
  font-size: 12px;
}

.compact-rule-editor {
  display: grid;
  gap: 10px;
  padding: 4px 0 12px;
}

.compact-rule-line {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  color: var(--ops-text-secondary);
  font-size: 13px;
  flex-wrap: wrap;
}

.logsql-line {
  align-items: flex-start;
}

.compact-rule-label,
.condition-prefix,
.condition-suffix {
  flex: 0 0 auto;
  line-height: 32px;
  white-space: nowrap;
}

.compact-rule-label {
  position: relative;
  width: 68px;
  color: #475569;
  font-weight: 700;
  text-align: right;
}

.compact-rule-label::after {
  content: ":";
  margin-left: 2px;
}

.compact-rule-label.is-required::before {
  content: "*";
  margin-right: 3px;
  color: #f56c6c;
}

.compact-rule-field {
  margin-bottom: 0;
}

.compact-rule-field :deep(.el-form-item__content) {
  margin-left: 0 !important;
}

.compact-rule-field :deep(.el-form-item__error) {
  position: absolute;
  top: 100%;
  padding-top: 2px;
}

.logsql-field {
  flex: 1 1 520px;
  min-width: 0;
}

.severity-field {
  width: 136px;
}

.window-field,
.interval-field {
  width: 116px;
}

.operator-field {
  width: 76px;
}

.threshold-field {
  width: 132px;
}

.condition-prefix {
  color: var(--ops-text-muted);
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0 12px;
}

.form-grid > * {
  min-width: 0;
}

.simple-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.preview-result {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-left: 76px;
  color: var(--ops-text-secondary);
  font-size: 13px;
  flex-wrap: wrap;
}

.detail-stack {
  display: grid;
  gap: 8px;
}

.detail-panel {
  min-width: 0;
  padding: 12px;
  border: 1px solid var(--ops-border-inner);
  border-radius: var(--ops-radius-inner);
  background: var(--ops-bg-surface);
}

.detail-panel h3,
.detail-panel h4 {
  margin: 0;
  color: var(--ops-text-primary);
  font-size: 14px;
  font-weight: 700;
}

.detail-panel p {
  margin: 6px 0 0;
  color: var(--ops-text-secondary);
  font-size: 12px;
  line-height: 1.6;
}

.detail-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 8px;
}

.sample-head,
.sample-meta,
.sample-log-head,
.sample-log-fields {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.sample-head {
  justify-content: space-between;
}

.sample-meta {
  margin-top: 8px;
  color: var(--ops-text-muted);
  font-size: 12px;
  line-height: 1.6;
}

.sample-meta span {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sample-meta strong {
  color: var(--ops-text-secondary);
  font-weight: 600;
}

.sample-error {
  margin-top: 10px;
}

.sample-log-list {
  display: grid;
  gap: 8px;
  margin-top: 8px;
}

.sample-log-item {
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid var(--ops-border-soft);
  border-radius: var(--ops-radius-md);
  background: var(--ops-bg-surface-soft);
}

.sample-log-head {
  justify-content: space-between;
  color: var(--ops-text-muted);
  font-size: 12px;
}

.sample-log-message {
  max-height: 96px;
  margin: 8px 0 0;
  overflow: auto;
  color: var(--ops-text-primary);
  font-size: 12px;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
}

.sample-log-fields {
  margin-top: 8px;
  color: var(--ops-text-muted);
  font-size: 12px;
}

.sample-log-fields span {
  max-width: 100%;
  padding: 2px 7px;
  overflow: hidden;
  border-radius: 6px;
  background: var(--ops-bg-surface);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sample-log-fields strong {
  color: var(--ops-text-secondary);
  font-weight: 600;
}

.json-view {
  max-height: 260px;
  margin: 10px 0 0;
  padding: 12px;
  overflow: auto;
  border: 1px solid var(--ops-border-soft);
  border-radius: var(--ops-radius-md);
  background: var(--ops-bg-subtle);
  color: #344054;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 12px;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
}

.log-alert-page :deep(.el-input__wrapper),
.log-alert-page :deep(.el-select__wrapper),
.log-alert-page :deep(.el-textarea__inner),
.log-alert-page :deep(.el-input-number .el-input__wrapper) {
  border-radius: var(--ops-radius-md);
  background: var(--ops-bg-surface);
  box-shadow: inset 0 0 0 1px var(--ops-border-inner) !important;
}

.log-alert-page :deep(.el-input__wrapper.is-focus),
.log-alert-page :deep(.el-select__wrapper.is-focused),
.log-alert-page :deep(.el-textarea__inner:focus) {
  box-shadow: inset 0 0 0 1px var(--ops-blue-500), 0 0 0 3px rgba(64, 158, 255, 0.1) !important;
}

.log-alert-page :deep(.el-button) {
  border-radius: var(--ops-radius-md) !important;
  font-weight: 600;
  transform: none !important;
  transition: color 160ms ease, border-color 160ms ease, background-color 160ms ease, box-shadow 160ms ease;
}

.toolbar-actions :deep(.el-button--primary) {
  border-color: #409eff;
  color: #ffffff;
  background: #409eff;
}

.toolbar-actions :deep(.el-button--primary:hover) {
  border-color: #337ecc;
  background: #337ecc;
}

.toolbar-actions :deep(.el-button--success) {
  border-color: #67c23a;
  color: #ffffff;
  background: #67c23a;
}

.toolbar-actions :deep(.el-button--success:hover) {
  border-color: #58b832;
  background: #58b832;
}

.toolbar-actions :deep(.el-button--warning) {
  border-color: #e6a23c;
  color: #ffffff;
  background: #e6a23c;
}

.toolbar-actions :deep(.el-button--warning:hover) {
  border-color: #d8922f;
  background: #d8922f;
}

.operation-links :deep(.el-button) {
  flex: 0 0 auto;
  margin-left: 0;
  padding: 0;
}

.operation-links :deep(.el-button > span) {
  display: inline-flex;
  align-items: center;
  white-space: nowrap;
}

:global(.log-alert-dialog-overlay),
:global(.log-alert-drawer-overlay) {
  --ops-text-primary: #0f172a;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-bg-surface: #ffffff;
  --ops-bg-surface-soft: #fbfdff;
  --ops-bg-subtle: #f3f7fc;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  --ops-border-inner: #dce8f5;
  --ops-blue-500: #409eff;
  --ops-radius-md: 8px;
  --ops-radius-inner: 10px;
  --ops-shadow-dialog: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.log-alert-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 32px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: var(--ops-bg-surface);
  box-shadow: var(--ops-shadow-dialog);
}

:global(.log-alert-dialog-overlay .el-dialog__header) {
  padding: 16px 18px 12px;
  margin: 0;
  border-bottom: 1px solid #e3edf8;
}

:global(.log-alert-dialog-overlay .el-dialog__title) {
  color: var(--ops-text-primary);
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.log-alert-dialog-overlay .el-dialog__headerbtn) {
  top: 10px;
  right: 12px;
  width: 34px;
  height: 34px;
  border-radius: var(--ops-radius-md);
}

:global(.log-alert-dialog-overlay .el-dialog__headerbtn:hover) {
  background: var(--ops-bg-subtle);
}

:global(.log-alert-dialog-overlay .el-dialog__body) {
  max-height: calc(100vh - 190px);
  overflow: auto;
  padding: 16px 18px;
}

:global(.log-alert-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
}

:global(.log-alert-dialog-overlay .el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

:global(.log-alert-dialog-overlay .el-input-number),
:global(.log-alert-dialog-overlay .el-select) {
  width: 100%;
}

:global(.log-alert-dialog-overlay .el-input__wrapper),
:global(.log-alert-dialog-overlay .el-select__wrapper),
:global(.log-alert-dialog-overlay .el-textarea__inner),
:global(.log-alert-dialog-overlay .el-input-number .el-input__wrapper) {
  border-radius: var(--ops-radius-md);
  background: var(--ops-bg-surface);
  box-shadow: inset 0 0 0 1px var(--ops-border-inner) !important;
}

:global(.log-alert-dialog-overlay .el-input__wrapper.is-focus),
:global(.log-alert-dialog-overlay .el-select__wrapper.is-focused),
:global(.log-alert-dialog-overlay .el-textarea__inner:focus) {
  box-shadow: inset 0 0 0 1px var(--ops-blue-500), 0 0 0 3px rgba(64, 158, 255, 0.1) !important;
}

:global(.log-alert-dialog-overlay .el-button) {
  border-radius: var(--ops-radius-md);
  font-weight: 600;
}

:global(.log-alert-dialog-overlay .el-button--primary) {
  border-color: #409eff;
  background: #409eff;
}

:global(.log-detail-drawer.el-drawer) {
  max-width: calc(100vw - 24px);
  border-left: 1px solid var(--ops-border);
  background: var(--ops-bg-surface-soft);
  box-shadow: -16px 0 48px rgba(49, 92, 142, 0.16);
}

:global(.log-detail-drawer .el-drawer__header) {
  min-height: 56px;
  padding: 16px 18px 12px;
  margin: 0;
  border-bottom: 1px solid var(--ops-border-soft);
  color: var(--ops-text-primary);
  font-size: 17px;
  font-weight: 700;
}

:global(.log-detail-drawer .el-drawer__body) {
  padding: 12px 14px 16px;
  background: var(--ops-bg-surface-soft);
}

:global(.log-detail-drawer .el-drawer__close-btn) {
  width: 34px;
  height: 34px;
  border-radius: var(--ops-radius-md);
}

:global(.log-detail-drawer .el-drawer__close-btn:hover) {
  background: var(--ops-bg-subtle);
}

:global(.log-detail-drawer .el-descriptions__label) {
  width: 104px;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

:global(.log-detail-drawer .el-descriptions__content) {
  color: var(--ops-text-secondary);
  font-size: 12px;
}

@media (max-width: 1100px) {
  .form-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 960px) {
  .page-card {
    height: auto;
    min-height: 100%;
    overflow: visible;
  }

  .page-header {
    flex-wrap: wrap;
  }

  .content-panel {
    min-height: 520px;
  }

  .form-grid {
    grid-template-columns: 1fr;
  }

  .compact-rule-line {
    align-items: flex-start;
  }

  .compact-rule-label {
    width: 72px;
  }

  .logsql-field,
  .severity-field,
  .window-field,
  .interval-field,
  .operator-field,
  .threshold-field {
    flex: 1 1 calc(100% - 84px);
    width: auto;
  }

  .preview-result {
    padding-left: 0;
  }
}

@media (max-width: 768px) {
  .page-header {
    align-items: flex-start;
  }

  .page-header-copy,
  .summary-group {
    width: 100%;
  }

  .summary-group {
    justify-content: flex-start;
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

  .toolbar-form :deep(.el-form-item) {
    width: calc(50% - 5px);
  }

  .toolbar-form :deep(.el-form-item:last-child) {
    width: 100%;
  }
}

@media (max-width: 620px) {
  .log-alert-page {
    padding: 6px;
  }

  .page-header {
    padding: 8px 10px;
  }

  .ops-page-header__icon {
    width: 36px;
    height: 36px;
    flex-basis: 36px;
  }

  .ops-page-header__icon img {
    width: 27px;
    height: 27px;
  }

  .summary-group {
    flex-wrap: wrap;
  }

  .summary-chip {
    flex: 1 1 84px;
  }

  .alert-tabs {
    padding: 0 10px;
  }

  .alert-tabs :deep(.el-tabs__nav) {
    gap: 16px;
  }

  .content-panel {
    padding: 8px;
  }

  .toolbar-form :deep(.el-form-item),
  .toolbar-form :deep(.el-input),
  .toolbar-form :deep(.el-select) {
    width: 100%;
  }

  .toolbar-form :deep(.el-form-item__content) {
    min-width: 0;
  }

  .toolbar-actions {
    display: grid;
    width: 100%;
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .toolbar-actions :deep(.el-button) {
    width: 100%;
    margin-left: 0;
  }

  .pagination-section {
    justify-content: flex-start;
    overflow-x: auto;
  }

  .form-section {
    padding: 10px 10px 2px;
  }

  .section-head {
    align-items: flex-start;
    flex-direction: column;
    gap: 3px;
  }

  .compact-rule-label {
    width: 68px;
  }

  :global(.log-alert-dialog-overlay .el-dialog) {
    width: calc(100% - 24px) !important;
    max-width: none;
  }

  :global(.log-alert-dialog-overlay .el-dialog__body) {
    padding: 14px 12px;
  }
}
</style>
