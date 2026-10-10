<template>
  <div class="alert-center-page alert-event-page">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/事件.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>告警事件</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>统一处理活跃、静默与历史告警事件，完整追踪每次触发的生命周期。</p>
      </div>
      <div class="event-header-summary" aria-label="告警事件统计">
        <span><em>活跃</em><strong class="is-danger">{{ totals.active }}</strong></span>
        <i aria-hidden="true"></i>
        <span><em>静默</em><strong class="is-warning">{{ totals.silenced }}</strong></span>
        <i aria-hidden="true"></i>
        <span><em>历史</em><strong>{{ totals.history }}</strong></span>
      </div>
    </header>

    <el-card shadow="never" class="alert-center-card event-center-card">
      <el-tabs v-model="activeTab" class="alert-tabs event-status-tabs" @tab-change="handleTabChange">
        <el-tab-pane label="活跃告警" name="active" />
        <el-tab-pane label="静默告警" name="silenced" />
        <el-tab-pane label="历史告警" name="history" />
      </el-tabs>

      <section class="event-toolbar">
        <div class="event-filter-row">
          <el-input v-model="query.keyword" class="event-keyword-input" clearable placeholder="输入搜索关键字" @keyup.enter="handleSearch">
            <template #prefix>
              <el-icon><Search /></el-icon>
            </template>
          </el-input>
          <el-select v-model="query.datasourceId" class="event-filter-select" clearable filterable placeholder="数据源">
            <el-option
              v-for="item in datasourceOptionRows"
              :key="datasourceOptionId(item)"
              :label="datasourceOptionLabel(item)"
              :value="datasourceOptionId(item)"
            />
          </el-select>
          <el-select v-model="query.severity" class="event-filter-select" clearable placeholder="告警等级">
            <el-option v-for="item in levelOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
          <el-select
            v-if="activeTab === 'active'"
            v-model="query.businessRouteId"
            class="event-filter-select"
            clearable
            filterable
            placeholder="告警业务"
            @change="handleSearch"
          >
            <el-option v-for="item in businessOptionRows" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
          <el-select v-if="activeTab === 'history'" v-model="query.eventStatus" class="event-filter-select" clearable placeholder="事件状态">
            <el-option v-for="item in eventStatusOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
          <el-button type="primary" class="event-icon-button event-query-button" @click="handleSearch">
            <el-icon><Search /></el-icon>
            <span>查询</span>
          </el-button>
          <el-button type="warning" class="event-icon-button ops-refresh-button" title="刷新" :loading="loading" @click="loadList">
            <el-icon><Refresh /></el-icon>
            <span>刷新</span>
          </el-button>
          <el-button type="success" class="event-icon-button event-export-button" title="导出当前列表" @click="exportRows()">
            <el-icon><Download /></el-icon>
            <span>导出</span>
          </el-button>
          <el-button type="primary" class="event-icon-button event-silence-button" title="管理静默规则" @click="openSilenceRules">
            <el-icon><MuteNotification /></el-icon>
            <span>静默规则</span>
          </el-button>
        </div>
        <el-dropdown trigger="click" :disabled="!selectedRows.length" @command="handleBatchCommand">
          <el-button type="warning" class="event-batch-button" :disabled="!selectedRows.length">
            批量操作
            <el-icon><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="claim" :disabled="activeTab === 'history'">批量认领</el-dropdown-item>
              <el-dropdown-item v-if="activeTab === 'silenced'" command="unsilence">批量取消静默</el-dropdown-item>
              <el-dropdown-item v-else command="silence" :disabled="activeTab === 'history'">批量静默</el-dropdown-item>
              <el-dropdown-item command="export">导出选中</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </section>

      <div class="alert-table-shell event-table-shell">
        <el-table
          class="alert-table event-table"
          :data="rows"
          v-loading="loading"
          empty-text="暂无告警事件"
          @selection-change="selectedRows = $event"
          @row-click="openSingleDetail"
        >
          <el-table-column type="selection" width="48" />
          <el-table-column label="事件信息" min-width="320">
            <template #default="{ row }">
              <div class="event-info-cell">
                <span class="event-level-dot" :class="`level-${levelCode(row).toLowerCase()}`">
                  <el-icon><WarningFilled /></el-icon>
                </span>
                <div class="event-info-main">
                  <div class="event-title-line">
                    <button type="button" class="event-title-link" @click.stop="openSingleDetail(row)">{{ eventTitle(row) }}</button>
                    <el-tag v-if="row.ruleGroupName" effect="light">{{ row.ruleGroupName }}</el-tag>
                  </div>
                  <div class="event-subtitle-line">{{ eventSubtitle(row) }}</div>
                </div>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="资源" min-width="240">
            <template #default="{ row }">
              <div class="event-resource-cell">
                <strong class="event-resource-name">{{ eventResourceName(row) }}</strong>
                <span class="event-resource-meta" :title="eventResourceMeta(row)">{{ eventResourceMeta(row) }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="当前值" width="80">
            <template #default="{ row }">
              <span class="event-current-value">{{ eventCurrentValue(row) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="告警等级" width="110" align="center">
            <template #default="{ row }">
              <el-tag class="event-level-tag" :type="levelTagType(row)" effect="light">{{ levelLabel(row) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="触发时间" width="170">
            <template #default="{ row }">{{ formatDateTime(row.firstTriggeredAt || row.triggeredAt || row.startsAt || row.occurredAt) }}</template>
          </el-table-column>
          <el-table-column label="持续时长" width="180" sortable>
            <template #default="{ row }">
              <div class="event-duration-cell">
                <span class="event-duration-text">
                  <el-icon><Clock /></el-icon>
                  {{ formatDuration(row) }}
                </span>
                <span class="event-duration-bars" aria-hidden="true">
                  <i v-for="index in 10" :key="index" :class="durationBarClass(row, index)" />
                </span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="事件状态" width="80">
            <template #default="{ row }">
              <el-tag :type="eventStatusTagType(row)" effect="light">{{ eventStatusText(row) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="认领人" width="150">
            <template #default="{ row }">
              <div class="event-claim-cell">
                <el-tag :type="row.claimUser ? 'primary' : 'info'" effect="plain">{{ row.claimUser || '未认领' }}</el-tag>
                <span class="event-claim-time">认领时间：{{ claimTimeText(row) }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="数据源" width="150">
            <template #default="{ row }">{{ row.datasourceName || row.datasourceType || row.datasourceId || '-' }}</template>
          </el-table-column>
          <el-table-column label="操作" width="86" fixed="right" align="center">
            <template #default="{ row }">
              <div class="event-operation-cell" @click.stop>
                <el-dropdown trigger="click" @command="command => handleRowCommand(command, row)">
                  <button type="button" class="event-more-button" title="更多操作" @click.stop>
                    <el-icon><MoreFilled /></el-icon>
                  </button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="detail">详情</el-dropdown-item>
                      <el-dropdown-item command="notify">告警推送记录</el-dropdown-item>
                      <el-dropdown-item command="claim" :disabled="activeTab === 'history' || Boolean(row.claimUser)">认领</el-dropdown-item>
                      <el-dropdown-item v-if="eventStatusValue(row) === 'silenced'" command="unsilence">取消静默</el-dropdown-item>
                      <el-dropdown-item v-else command="silence" :disabled="activeTab === 'history'">静默</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="alert-pagination">
        <el-pagination
          v-model:current-page="query.page"
          v-model:page-size="query.pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="currentTotal"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="loadList"
          @current-change="loadList"
        />
      </div>
    </el-card>

    <el-drawer
      v-model="detailVisible"
      title="单事件详情"
      size="50%"
      class="alert-detail-drawer event-detail-drawer"
      modal-class="alert-event-detail-overlay"
      append-to-body
    >
      <div class="event-detail-body" v-loading="detailLoading">
        <template v-if="detailEvent">
          <div class="event-detail-actions">
            <el-button :type="detailPanel === 'detail' ? 'primary' : 'default'" @click="setDetailPanel('detail')">事件详情</el-button>
            <el-button :type="detailPanel === 'notify' ? 'primary' : 'default'" :loading="notifyRecordLoading" @click="setDetailPanel('notify')">
              <el-icon><Bell /></el-icon>
              <span>告警推送记录</span>
            </el-button>
          </div>

          <template v-if="detailPanel === 'detail'">
            <section class="event-detail-table">
              <div class="event-detail-line">
                <span class="event-detail-label">规则名称</span>
                <div class="event-detail-value">
                  <button type="button" class="event-title-link">{{ eventTitle(detailEvent) }}</button>
                </div>
              </div>
              <div class="event-detail-line">
                <span class="event-detail-label">告警指纹</span>
                <div class="event-detail-value">{{ detailEvent.fingerprint || '-' }}</div>
              </div>
              <div class="event-detail-line">
                <span class="event-detail-label">数据源</span>
                <div class="event-detail-value">{{ detailEvent.datasourceName || detailEvent.datasourceType || detailEvent.datasourceId || '-' }}</div>
              </div>
              <div class="event-detail-line">
                <span class="event-detail-label">告警等级</span>
                <div class="event-detail-value">
                  <el-tag :type="levelTagType(detailEvent)" effect="light">{{ levelLabel(detailEvent) }}</el-tag>
                </div>
              </div>
              <div class="event-detail-line">
                <span class="event-detail-label">事件状态</span>
                <div class="event-detail-value">
                  <el-tag :type="eventStatusTagType(detailEvent)" effect="light">{{ eventStatusText(detailEvent) }}</el-tag>
                </div>
              </div>
              <div class="event-detail-line">
                <span class="event-detail-label">事件标签</span>
                <div class="event-detail-value event-label-list">
                  <el-tag v-for="item in detailLabelRows" :key="item.key" effect="light">{{ item.key }}: {{ item.value }}</el-tag>
                  <span v-if="!detailLabelRows.length">-</span>
                </div>
              </div>
              <div class="event-detail-line">
                <span class="event-detail-label">触发条件</span>
                <div class="event-detail-value event-code-text">{{ detailData.triggerCondition || detailData.triggerQuery || '-' }}</div>
              </div>
              <div class="event-detail-line">
                <span class="event-detail-label">触发时间</span>
                <div class="event-detail-value">{{ formatDateTime(detailEvent.firstTriggeredAt || detailEvent.triggeredAt || detailEvent.occurredAt) }}</div>
              </div>
              <div class="event-detail-line">
                <span class="event-detail-label">触发时值</span>
                <div class="event-detail-value">{{ eventCurrentValue(detailEvent) }}</div>
              </div>
              <div class="event-detail-line">
                <span class="event-detail-label">持续时长</span>
                <div class="event-detail-value">{{ formatDuration(detailEvent) }}</div>
              </div>
              <div class="event-detail-line">
                <span class="event-detail-label">认领人</span>
                <div class="event-detail-value">
                  <el-tag :type="detailEvent.claimUser ? 'primary' : 'info'" effect="plain">{{ detailEvent.claimUser || '未认领' }}</el-tag>
                  <span class="event-inline-time">认领时间：{{ claimTimeText(detailEvent) }}</span>
                </div>
              </div>
              <div class="event-detail-line event-detail-textarea-line">
                <span class="event-detail-label">事件详情</span>
                <div class="event-detail-value">
                  <div class="event-detail-textbox">{{ detailData.eventDetail || eventSubtitle(detailEvent) || '-' }}</div>
                </div>
              </div>
            </section>

            <section class="event-detail-section">
              <h4>触发查询</h4>
              <div v-if="detailTriggerRows.length" class="event-query-list">
                <div v-for="(item, index) in detailTriggerRows" :key="index" class="event-query-row" :class="{ matched: Boolean(item.matched) }">
                  <div class="event-query-head">
                    <el-tag :type="item.matched ? 'danger' : 'info'" effect="light">{{ item.matched ? '已命中' : '未命中' }}</el-tag>
                    <strong>{{ item.severity || item.level || item.priority || `条件 ${index + 1}` }}</strong>
                    <span>{{ item.condition || item.triggerCondition || detailData.triggerCondition || '-' }}</span>
                  </div>
                  <pre>{{ item.prom_ql || item.promQl || item.query || item.expr || detailData.triggerQuery || '-' }}</pre>
                </div>
              </div>
              <el-empty v-else description="暂无触发查询" :image-size="64" />
            </section>

            <section class="event-detail-section">
              <h4>时间线</h4>
              <div v-if="detailTimelineRows.length" class="event-comment-list">
                <div v-for="(item, index) in detailTimelineRows" :key="item.id || index" class="event-comment-item">
                  <span class="event-comment-avatar">{{ timelineAvatar(item) }}</span>
                  <div class="event-comment-content">
                    <strong>{{ item.operator || item.user || item.createdBy || item.claimUser || '系统' }}</strong>
                    <p>{{ item.message || item.remark || item.reason || item.eventTypeName || item.eventType || '事件更新' }}</p>
                    <time>{{ formatDateTime(item.occurredAt || item.createdAt || item.updatedAt) }}</time>
                  </div>
                </div>
              </div>
              <el-empty v-else description="暂无时间线" :image-size="64" />
            </section>
          </template>

          <section v-else class="event-notify-record-panel" v-loading="notifyRecordLoading">
            <div class="event-notify-summary">
              <div class="event-notify-stat">
                <span>总数</span>
                <strong>{{ notifyStatsView.total }}</strong>
              </div>
              <div class="event-notify-stat success">
                <span>成功</span>
                <strong>{{ notifyStatsView.successCount }}</strong>
              </div>
              <div class="event-notify-stat failed">
                <span>失败</span>
                <strong>{{ notifyStatsView.failedCount }}</strong>
              </div>
            </div>
            <div class="event-notify-scope">
              {{ notifyRuleId ? `按告警规则统计：${detailEvent.ruleName || detailData.ruleName || notifyRuleId}` : '当前事件详情通知记录' }}
            </div>
            <el-table class="event-notify-table" :data="notifyTableRows" empty-text="暂无告警推送记录">
              <el-table-column label="通知类型" min-width="120">
                <template #default="{ row }">{{ notifyChannelTypeText(row) }}</template>
              </el-table-column>
              <el-table-column label="通知群组" min-width="160" show-overflow-tooltip>
                <template #default="{ row }">{{ notifyGroupText(row) }}</template>
              </el-table-column>
              <el-table-column label="通知对象" min-width="180">
                <template #default="{ row }">
                  <el-tooltip :content="notifyReceiverText(row)" placement="top" :disabled="notifyReceiverText(row) === '-'">
                    <span class="event-notify-target">{{ notifyReceiverText(row) }}</span>
                  </el-tooltip>
                </template>
              </el-table-column>
              <el-table-column label="Request" width="100" align="center">
                <template #default="{ row }">
                  <el-button link type="primary" @click="showNotifyPayload('request', row)">查看详情</el-button>
                </template>
              </el-table-column>
              <el-table-column label="Response" width="100" align="center">
                <template #default="{ row }">
                  <el-button link type="primary" @click="showNotifyPayload('response', row)">查看详情</el-button>
                </template>
              </el-table-column>
              <el-table-column label="状态" width="110" align="center">
                <template #default="{ row }">
                  <el-tag :type="notifyStatusTagType(row)" effect="light">{{ notifyStatusText(row) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="通知时间" width="170">
                <template #default="{ row }">{{ formatDateTime(row.createdAt || row.sentAt) }}</template>
              </el-table-column>
            </el-table>
            <div v-if="notifyRuleId" class="event-notify-pagination">
              <el-pagination
                v-model:current-page="notifyQuery.page"
                v-model:page-size="notifyQuery.pageSize"
                :page-sizes="[10, 20, 50, 100]"
                :total="notifyRecordTotal"
                layout="total, sizes, prev, pager, next"
                @size-change="handleNotifySizeChange"
                @current-change="loadNotifyRecords"
              />
            </div>
          </section>
        </template>
      </div>
    </el-drawer>

    <el-dialog
      v-model="notifyPayloadVisible"
      :title="notifyPayloadTitle"
      width="720px"
      class="alert-dialog notify-payload-dialog"
      modal-class="alert-event-dialog-overlay"
      destroy-on-close
    >
      <pre class="notify-payload-content">{{ notifyPayloadContent }}</pre>
    </el-dialog>

    <el-dialog
      v-model="silenceVisible"
      :title="silenceTargets.length > 1 ? '批量静默告警' : '静默告警'"
      width="720px"
      class="alert-dialog event-silence-dialog"
      modal-class="alert-event-dialog-overlay"
      destroy-on-close
    >
      <el-form ref="silenceFormRef" :model="silenceForm" :rules="silenceRules" label-width="92px">
        <el-form-item label="静默范围">
          <el-radio-group v-model="silenceForm.scope">
            <el-radio-button label="event">仅当前事件</el-radio-button>
            <el-radio-button label="labels" :disabled="silenceTargets.length > 1">按标签匹配</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="silenceForm.scope === 'labels'" label="匹配条件" prop="matchers">
          <div class="silence-matcher-list">
            <div v-for="(matcher, index) in silenceForm.matchers" :key="index" class="silence-matcher-row">
              <el-input v-model="matcher.label" placeholder="标签名" />
              <el-select v-model="matcher.op" class="silence-matcher-op">
                <el-option v-for="item in silenceMatcherOperatorOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
              <el-input v-model="matcher.value" placeholder="匹配值" />
              <el-button type="danger" class="silence-matcher-remove" :icon="Delete" circle title="删除匹配条件" @click="removeSilenceMatcher(index)" />
            </div>
            <el-button type="success" class="silence-matcher-add" :icon="Plus" @click="addSilenceMatcher">添加匹配条件</el-button>
          </div>
        </el-form-item>
        <el-form-item label="静默方式">
          <el-radio-group v-model="silenceForm.mode">
            <el-radio-button label="duration">持续时间</el-radio-button>
            <el-radio-button label="endTime">结束时间</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="silenceForm.mode === 'duration'" label="持续分钟" prop="durationMinutes">
          <el-input-number v-model="silenceForm.durationMinutes" :min="1" :max="43200" controls-position="right" class="full-input-number" />
        </el-form-item>
        <el-form-item v-else label="结束时间" prop="endTime">
          <el-date-picker
            v-model="silenceForm.endTime"
            type="datetime"
            value-format="YYYY-MM-DD HH:mm:ss"
            placeholder="请选择结束时间"
            class="full-date"
          />
        </el-form-item>
        <el-form-item label="原因" prop="reason">
          <el-input v-model="silenceForm.reason" type="textarea" :rows="3" placeholder="例如：临时维护" />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="alert-dialog-footer">
          <el-button type="primary" :loading="silenceSaving" @click="submitSilence">确认</el-button>
          <el-button @click="silenceVisible = false">取消</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown, Bell, Clock, Delete, Download, MoreFilled, MuteNotification, Plus, Refresh, Search, WarningFilled } from '@element-plus/icons-vue'
import alertApi from '@/api/alertmanager'
import {
  compactObject,
  formatDateTime,
  getRowId,
  isSuccess,
  normalizeListResult,
  parseSafeJson,
  sortNotifyRecordsByTimeDesc,
  summarizeObject,
  unwrapData
} from './alertCommon'
import './alert-pages.css'

const eventStatusOptions = [
  { label: '活跃', value: 'active', type: 'danger' },
  { label: '观察中', value: 'pending', type: 'info' },
  { label: '静默', value: 'silenced', type: 'warning' },
  { label: '抑制', value: 'suppressed', type: 'warning' },
  { label: '已恢复', value: 'resolved', type: 'success' }
]

const levelOptions = [
  { label: 'P0 严重', value: 'critical' },
  { label: 'P1 警告', value: 'warning' },
  { label: 'P2 提示', value: 'info' }
]

const silenceMatcherOperatorOptions = [
  { label: '=', value: '=' },
  { label: '!=', value: '!=' },
  { label: '=~', value: '=~' },
  { label: '!~', value: '!~' }
]

const router = useRouter()

const activeTab = ref('active')
const loading = ref(false)
const rows = ref([])
const selectedRows = ref([])
const datasourceOptionRows = ref([])
const businessOptionRows = ref([])
const totals = reactive({ active: 0, silenced: 0, history: 0 })

const detailVisible = ref(false)
const detailLoading = ref(false)
const detailData = ref({})
const detailPanel = ref('detail')
const notifyRecordLoading = ref(false)
const notifyRecordRows = ref([])
const notifyRecordTotal = ref(0)
const notifyStats = ref({ total: 0, successCount: 0, failedCount: 0 })
const notifyPayloadVisible = ref(false)
const notifyPayloadTitle = ref('')
const notifyPayloadContent = ref('')

const silenceVisible = ref(false)
const silenceSaving = ref(false)
const silenceFormRef = ref(null)
const silenceTargets = ref([])

const currentTotal = computed(() => totals[activeTab.value] || 0)
const detailEvent = computed(() => detailData.value?.event || null)
const detailLabelRows = computed(() => objectRows(detailEvent.value?.labels))
const detailTimelineRows = computed(() => arrayValue(detailData.value?.timeline))
const detailNotifyRows = computed(() => arrayValue(detailData.value?.notifyRecords))
const notifyRuleId = computed(() => detailEvent.value?.ruleId ?? detailEvent.value?.rule_id ?? detailData.value?.ruleId ?? detailData.value?.rule_id ?? '')
const notifyTableRows = computed(() => sortNotifyRecordsByTimeDesc(notifyRuleId.value ? notifyRecordRows.value : detailNotifyRows.value))
const notifyStatsView = computed(() => {
  if (notifyRuleId.value) return notifyStats.value
  const rows = detailNotifyRows.value
  return {
    total: rows.length,
    successCount: rows.filter(item => normalizeNotifyStatus(item) === 'success').length,
    failedCount: rows.filter(item => normalizeNotifyStatus(item) === 'failed').length
  }
})
const detailTriggerRows = computed(() => {
  const result = arrayValue(detailData.value?.triggerQueries)
  if (result.length) return result
  if (!detailData.value?.triggerQuery && !detailData.value?.triggerCondition) return []
  return [{
    matched: true,
    query: detailData.value.triggerQuery,
    condition: detailData.value.triggerCondition
  }]
})

const query = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  datasourceId: '',
  severity: '',
  businessRouteId: '',
  eventStatus: ''
})

const notifyQuery = reactive({
  page: 1,
  pageSize: 10
})

const silenceForm = reactive({
  scope: 'event',
  matchers: [],
  mode: 'duration',
  durationMinutes: 60,
  endTime: '',
  reason: ''
})

const silenceRules = {
  matchers: [{
    validator: (_rule, value, callback) => {
      if (silenceForm.scope !== 'labels') return callback()
      const items = arrayValue(value)
      if (!items.length || items.some(item => !String(item?.label || '').trim())) {
        callback(new Error('请至少配置一个完整的标签匹配条件'))
        return
      }
      callback()
    },
    trigger: 'change'
  }],
  durationMinutes: [{ required: true, message: '请输入静默持续分钟', trigger: 'blur' }],
  endTime: [{ required: true, message: '请选择静默结束时间', trigger: 'change' }],
  reason: [{ required: true, message: '请输入静默原因', trigger: 'blur' }]
}

const buildParams = () => compactObject({
  page: query.page,
  pageSize: query.pageSize,
  keyword: query.keyword,
  datasourceId: query.datasourceId,
  severity: query.severity,
  businessRouteId: activeTab.value === 'active' ? query.businessRouteId : '',
  eventStatus: activeTab.value === 'active' || activeTab.value === 'silenced' ? activeTab.value : query.eventStatus
})

const loadList = async () => {
  loading.value = true
  try {
    const response = activeTab.value === 'history'
        ? await alertApi.getHistoryEvents(buildParams())
        : await alertApi.getActiveEvents(buildParams())
    const result = normalizeListResult(response)
    rows.value = result.list
    totals[activeTab.value] = result.total
  } finally {
    loading.value = false
  }
}

const loadDatasourceOptions = async () => {
  const response = await alertApi.getDatasourceOptions()
  datasourceOptionRows.value = normalizeListResult(response).list
}

const loadBusinessOptions = async () => {
  const response = await alertApi.getActiveEventBusinessOptions()
  businessOptionRows.value = normalizeListResult(response).list
}

const handleSearch = () => {
  query.page = 1
  loadList()
}

const openSilenceRules = () => router.push({ path: '/alert/rule', query: { tab: 'silences' } })

const handleTabChange = () => {
  query.page = 1
  rows.value = []
  selectedRows.value = []
  query.eventStatus = ''
  detailVisible.value = false
  loadList()
}

const handleRowCommand = (command, row) => {
  if (command === 'detail') openSingleDetail(row, { focus: command })
  if (command === 'notify') openSingleDetail(row, { focus: command })
  if (command === 'claim') claimRows([row])
  if (command === 'silence') openSilenceDialog([row])
  if (command === 'unsilence') unsilenceRows([row])
}

const handleBatchCommand = command => {
  if (command === 'claim') {
    claimRows(selectedRows.value)
    return
  }
  if (command === 'silence') {
    openSilenceDialog(selectedRows.value)
    return
  }
  if (command === 'unsilence') {
    unsilenceRows(selectedRows.value)
    return
  }
  if (command === 'export') {
    exportRows(selectedRows.value)
  }
}

const openSingleDetail = async (row, options = {}) => {
  const eventId = getRowId(row)
  if (!eventId) {
    ElMessage.warning('当前告警缺少事件 ID，无法查询详情')
    return
  }
  detailVisible.value = true
  detailLoading.value = true
  detailPanel.value = options.focus === 'notify' ? 'notify' : 'detail'
  notifyRecordRows.value = []
  notifyRecordTotal.value = 0
  notifyStats.value = { total: 0, successCount: 0, failedCount: 0 }
  notifyQuery.page = 1
  detailData.value = { event: row, focus: detailPanel.value }
  try {
    const eventType = detailEventType(row)
    if (eventType === 'active') {
      const [detailResponse, timelineResponse] = await Promise.all([
        alertApi.getEventDetail(eventType, eventId),
        alertApi.getActiveEventTimeline(eventId)
      ])
      const payload = unwrapData(detailResponse)
      detailData.value = {
        ...(payload && typeof payload === 'object' ? payload : { event: row }),
        timeline: normalizeListResult(timelineResponse).list,
        focus: detailPanel.value
      }
      await loadNotifyRecordsIfNeeded()
      return
    }
    const response = await alertApi.getEventDetail(eventType, eventId)
    const payload = unwrapData(response)
    detailData.value = {
      ...(payload && typeof payload === 'object' ? payload : { event: row }),
      focus: detailPanel.value
    }
    await loadNotifyRecordsIfNeeded()
  } finally {
    detailLoading.value = false
  }
}

const setDetailPanel = async panel => {
  detailPanel.value = panel
  detailData.value = { ...detailData.value, focus: panel }
  if (panel === 'notify') await loadNotifyRecordsIfNeeded()
}

const loadNotifyRecordsIfNeeded = async () => {
  if (detailPanel.value !== 'notify') return
  await loadNotifyRecords()
}

const loadNotifyRecords = async () => {
  const ruleId = notifyRuleId.value
  if (!ruleId) {
    notifyRecordRows.value = detailNotifyRows.value
    notifyRecordTotal.value = detailNotifyRows.value.length
    return
  }
  notifyRecordLoading.value = true
  try {
    const params = { ruleId, page: notifyQuery.page, pageSize: notifyQuery.pageSize }
    const [recordsResponse, statsResponse] = await Promise.all([
      alertApi.getNotifyRecords(params),
      alertApi.getNotifyRecordStats(params)
    ])
    const result = normalizeListResult(recordsResponse)
    notifyRecordRows.value = result.list
    notifyRecordTotal.value = result.total
    notifyStats.value = normalizeNotifyStats(unwrapNotifyStats(statsResponse))
  } finally {
    notifyRecordLoading.value = false
  }
}

const handleNotifySizeChange = () => {
  notifyQuery.page = 1
  loadNotifyRecords()
}

const claimRows = async targetRows => {
  const targets = uniqueEventRows(targetRows).filter(row => getRowId(row) && !row.claimUser)
  if (!targets.length) {
    ElMessage.warning('请选择可认领的告警事件')
    return
  }
  try {
    await ElMessageBox.confirm(`确认认领 ${targets.length} 条告警事件？`, '认领确认', {
      type: 'warning',
      confirmButtonText: '认领',
      cancelButtonText: '取消'
    })
    await Promise.all(targets.map(row => alertApi.claimActiveEvent(getRowId(row))))
    ElMessage.success('认领成功')
    await refreshAfterMutation()
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') throw error
  }
}

const openSilenceDialog = targetRows => {
  const targets = uniqueEventRows(targetRows).filter(row => getRowId(row) && eventStatusValue(row) !== 'silenced')
  if (!targets.length) {
    ElMessage.warning('请选择可静默的告警事件')
    return
  }
  silenceTargets.value = targets
  Object.assign(silenceForm, {
    scope: 'event',
    matchers: suggestedSilenceMatchers(targets[0]),
    mode: 'duration',
    durationMinutes: 60,
    endTime: '',
    reason: ''
  })
  silenceVisible.value = true
}

const submitSilence = async () => {
  await silenceFormRef.value?.validate()
  const timePayload = silenceForm.mode === 'duration'
    ? { durationMinutes: silenceForm.durationMinutes, reason: silenceForm.reason }
    : { endTime: silenceForm.endTime, reason: silenceForm.reason }
  const payload = silenceForm.scope === 'labels'
    ? { ...timePayload, matchers: { items: silenceForm.matchers.map(item => ({ label: item.label.trim(), op: item.op, value: item.value })) } }
    : timePayload
  silenceSaving.value = true
  try {
    const responses = await Promise.all(silenceTargets.value.map(row => alertApi.silenceActiveEvent(getRowId(row), payload)))
    const ok = responses.every(response => isSuccess(response))
    if (ok) {
      ElMessage.success('静默成功')
      silenceVisible.value = false
      await refreshAfterMutation()
    }
  } finally {
    silenceSaving.value = false
  }
}

const addSilenceMatcher = () => silenceForm.matchers.push({ label: '', op: '=', value: '' })

const removeSilenceMatcher = index => {
  silenceForm.matchers.splice(index, 1)
  if (!silenceForm.matchers.length) addSilenceMatcher()
}

const suggestedSilenceMatchers = row => {
  const labels = parseSafeJson(row?.labels)
  const candidates = ['service', 'instance', 'hostname', 'pod', 'node', 'namespace']
  for (const key of candidates) {
    if (labels?.[key] !== undefined && labels?.[key] !== '') {
      return [{ label: key, op: '=', value: String(labels[key]) }]
    }
  }
  if (row?.ruleName) return [{ label: 'alertname', op: '=', value: String(row.ruleName) }]
  return [{ label: 'severity', op: '=', value: String(row?.severity || '') }]
}

const unsilenceRows = async targetRows => {
  const targets = uniqueEventRows(targetRows).filter(row => getRowId(row) && eventStatusValue(row) === 'silenced')
  if (!targets.length) {
    ElMessage.warning('请选择已静默的告警事件')
    return
  }
  try {
    await ElMessageBox.confirm(`确认取消 ${targets.length} 条告警事件的静默？`, '取消静默', {
      type: 'warning',
      confirmButtonText: '取消静默',
      cancelButtonText: '返回'
    })
    const responses = await Promise.all(targets.map(row => alertApi.unsilenceActiveEvent(getRowId(row))))
    if (responses.every(response => isSuccess(response))) {
      const remaining = responses.map(response => unwrapData(response)).filter(payload => payload?.silenced)
      if (remaining.length) {
        const ruleNames = [...new Set(remaining.flatMap(payload => arrayValue(payload?.remainingRules).map(rule => rule?.name).filter(Boolean)))]
        ElMessage.warning(`${remaining.length} 条事件仍受其他静默规则影响${ruleNames.length ? `：${ruleNames.join('、')}` : ''}`)
      } else {
        ElMessage.success('已取消静默')
      }
      await refreshAfterMutation()
    }
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') throw error
  }
}

const refreshAfterMutation = async () => {
  await loadList()
  await refreshSingleDetail()
}

const refreshSingleDetail = async () => {
  if (!detailVisible.value || !detailEvent.value) return
  const row = latestSingleEventRow(detailEvent.value)
  if (!getRowId(row)) return
  await openSingleDetail(row, { focus: detailData.value?.focus || 'detail' })
}

const exportRows = (targetRows = rows.value) => {
  const exportList = Array.isArray(targetRows) && targetRows.length ? targetRows : rows.value
  if (!exportList.length) {
    ElMessage.warning('暂无数据可导出')
    return
  }
  exportEventRows(exportList)
}

const exportEventRows = exportList => {
  const headers = ['事件名称', '资源', '资源信息', '当前值', '告警等级', '触发时间', '持续时长', '事件状态', '认领人', '认领时间', '数据源', '规则分组', '指纹']
  const csvRows = exportList.map(row => [
    eventTitle(row),
    eventResourceName(row),
    eventResourceMeta(row),
    eventCurrentValue(row),
    levelLabel(row),
    formatDateTime(row.firstTriggeredAt || row.triggeredAt || row.startsAt || row.occurredAt),
    formatDuration(row),
    eventStatusText(row),
    row.claimUser || '',
    claimTimeText(row),
    row.datasourceName || row.datasourceType || '',
    row.ruleGroupName || '',
    row.fingerprint || ''
  ])
  downloadCsv(`alert-events-${activeTab.value}-${Date.now()}.csv`, headers, csvRows)
}

const downloadCsv = (filename, headers, csvRows) => {
  const content = [headers, ...csvRows].map(row => row.map(escapeCsvCell).join(',')).join('\n')
  const blob = new Blob(['\uFEFF' + content], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
  ElMessage.success('导出成功')
}

const escapeCsvCell = value => `"${String(value ?? '').replace(/"/g, '""')}"`

const latestSingleEventRow = currentRow => {
  const currentId = getRowId(currentRow)
  if (!currentId) return currentRow
  return rows.value.find(item => getRowId(item) === currentId) || currentRow
}

const uniqueEventRows = list => {
  const map = new Map()
  ;(Array.isArray(list) ? list : []).forEach(item => {
    const id = getRowId(item)
    if (id && !map.has(id)) map.set(id, item)
  })
  return [...map.values()]
}

const labelObject = row => {
  const labels = parseSafeJson(row?.labels)
  return labels && typeof labels === 'object' && !Array.isArray(labels) ? labels : {}
}

const annotationObject = row => {
  const annotations = parseSafeJson(row?.annotations)
  return annotations && typeof annotations === 'object' && !Array.isArray(annotations) ? annotations : {}
}

const levelCode = row => {
  const raw = String(row?.level || labelObject(row)?.level || row?.severity || '').trim().toLowerCase()
  if (raw === 'p0' || raw === 'critical') return 'P0'
  if (raw === 'p1' || raw === 'warning' || raw === 'warn') return 'P1'
  if (raw === 'p2' || raw === 'info') return 'P2'
  return (String(row?.level || row?.severity || '').trim() || 'P2').toUpperCase()
}

const levelLabel = row => {
  const level = levelCode(row).toLowerCase()
  if (level === 'p0' || level === 'critical') return 'P0'
  if (level === 'p1' || level === 'warning' || level === 'warn') return 'P1'
  if (level === 'p2' || level === 'info') return 'P2'
  return levelCode(row) || '-'
}

const levelTagType = row => {
  const level = levelLabel(row).toLowerCase()
  if (level === 'p0') return 'danger'
  if (level === 'p1') return 'warning'
  return 'primary'
}

const eventTitle = row => row?.ruleName || row?.name || row?.alertName || row?.summary || '-'

const eventSubtitle = row => {
  const annotations = annotationObject(row)
  return eventResourceLine(row) || annotations.description || annotations.summary || row?.description || row?.message || summarizeObject(row?.labels)
}

const labelValue = (row, keys) => {
  const labels = labelObject(row)
  for (const key of keys) {
    const value = labels[key]
    if (value !== undefined && value !== null && String(value).trim()) return String(value).trim()
  }
  return ''
}

const eventResourceName = row => labelValue(row, ['hostname', 'host', 'node', 'pod', 'container', 'name', 'cluster']) || labelValue(row, ['instance']) || `事件 ${getRowId(row) || '-'}`

const eventResourceMetaItems = row => {
  const values = [
    labelValue(row, ['instance']),
    labelValue(row, ['device', 'disk', 'filesystem']),
    labelValue(row, ['mountpoint', 'mount_point', 'path']),
    labelValue(row, ['fstype']),
    labelValue(row, ['job'])
  ]
  return [...new Set(values.filter(Boolean).filter(value => value !== eventResourceName(row)))]
}

const eventResourceMeta = row => eventResourceMetaItems(row).join(' / ') || '-'

const eventResourceLine = row => {
  const name = eventResourceName(row)
  const meta = eventResourceMetaItems(row).join(' / ')
  if (!name || name === '-') return meta || ''
  return meta ? `${name} / ${meta}` : name
}

const eventCurrentValue = row => formatMetricValue(row?.value ?? row?.triggerValue ?? row?.currentValue ?? row?.evalValue)

const formatMetricValue = value => {
  if (value === undefined || value === null || value === '') return '-'
  if (typeof value === 'number') return Number.isFinite(value) ? value.toFixed(2) : '-'
  const text = String(value).trim()
  if (!text) return '-'
  const numeric = Number(text)
  if (Number.isFinite(numeric)) return numeric.toFixed(2)
  return text.replace(/-?\d+(?:\.\d+)?/, match => Number(match).toFixed(2))
}

const claimTimeText = row => isZeroDateTime(row?.claimedAt) ? '-' : formatDateTime(row?.claimedAt)

const isZeroDateTime = value => {
  if (!value) return true
  const text = String(value).trim()
  if (!text || text === '-') return true
  return /^0{4}[-/]0{2}[-/]0{2}/.test(text) || /^0001[-/]0?1[-/]0?1(?:[ T]0{1,2}:0{1,2})?/.test(text)
}

const eventStatusValue = row => String(row?.eventStatus || row?.status || (activeTab.value === 'silenced' ? 'silenced' : '')).toLowerCase()

const eventStatusText = row => {
  if (row?.eventStatusName) return row.eventStatusName
  const target = eventStatusOptions.find(item => item.value === eventStatusValue(row))
  return target?.label || row?.eventStatus || row?.status || '-'
}

const eventStatusTagType = row => {
  const target = eventStatusOptions.find(item => item.value === eventStatusValue(row))
  return target?.type || 'info'
}

const normalizeNotifyStats = value => {
  const stats = value && typeof value === 'object' ? value : {}
  return {
    total: Number(stats.total || 0),
    successCount: Number(stats.successCount || stats.success_count || 0),
    failedCount: Number(stats.failedCount || stats.failed_count || 0)
  }
}

const unwrapNotifyStats = response => {
  const payload = unwrapData(response)
  if (payload && typeof payload === 'object' && ('total' in payload || 'successCount' in payload || 'failedCount' in payload)) return payload
  const data = response?.data
  if (data && typeof data === 'object' && ('total' in data || 'successCount' in data || 'failedCount' in data)) return data
  if (response && typeof response === 'object' && ('total' in response || 'successCount' in response || 'failedCount' in response)) return response
  return payload
}

const normalizeNotifyStatus = row => String(row?.status || row?.notifyStatus || '').trim().toLowerCase()

const notifyGroupText = row => {
  const name = row?.notifyGroupName || row?.notify_group_name
  if (name) return name
  const id = row?.notifyGroupId || row?.notify_group_id
  return id ? `群组 #${id}` : '-'
}

const notifyStatusText = row => {
  const status = normalizeNotifyStatus(row)
  if (status === 'success') return '发送成功'
  if (status === 'failed' || status === 'fail') return '发送失败'
  return row?.statusName || row?.status || '-'
}

const notifyStatusTagType = row => {
  const status = normalizeNotifyStatus(row)
  if (status === 'success') return 'success'
  if (status === 'failed' || status === 'fail') return 'danger'
  return 'info'
}

const notifyChannelTypeText = row => {
  const type = String(row?.channelType || row?.channel_type || row?.channelName || row?.channel || '').trim()
  const map = {
    feishu: '飞书',
    feishu_card: '飞书',
    dingtalk: '钉钉',
    wechat: '企业微信',
    email: '邮件',
    sms: '短信',
    webhook: 'Webhook'
  }
  return map[type] || type || '-'
}

const notifyReceiverText = row => row?.receiver || row?.notifyReceiver || '-'

const showNotifyPayload = (type, row) => {
  const label = type === 'request' ? 'Request' : 'Response'
  notifyPayloadTitle.value = `${label} 详情`
  notifyPayloadContent.value = formatNotifyPayload(row?.[type])
  notifyPayloadVisible.value = true
}

const formatNotifyPayload = value => {
  if (value === undefined || value === null || value === '') return '-'
  const parsed = parseSafeJson(value)
  if (parsed && typeof parsed === 'object') return JSON.stringify(parsed, null, 2)
  return String(parsed)
}

const formatDuration = row => {
  const seconds = Number(row?.durationSeconds ?? inferDurationSeconds(row))
  if (!Number.isFinite(seconds) || seconds < 0) return '-'
  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)
  const restSeconds = Math.floor(seconds % 60)
  if (days > 0) return `${days}天 ${hours}小时 ${minutes}分 ${restSeconds}秒`
  if (hours > 0) return `${hours}小时 ${minutes}分 ${restSeconds}秒`
  if (minutes > 0) return `${minutes}分 ${restSeconds}秒`
  return `${restSeconds}秒`
}

const inferDurationSeconds = row => {
  const start = row?.firstTriggeredAt || row?.triggeredAt || row?.startsAt || row?.occurredAt
  if (!start) return NaN
  const startTime = new Date(start).getTime()
  if (Number.isNaN(startTime)) return NaN
  const end = row?.resolvedAt || row?.recoveredAt || row?.endsAt || row?.updatedAt
  const endTime = end ? new Date(end).getTime() : Date.now()
  return Math.max(0, Math.floor((endTime - startTime) / 1000))
}

const durationBarClass = (row, index) => {
  const level = levelLabel(row).toLowerCase()
  if (level === 'p0') return 'bar-danger'
  if (level === 'p1') return index > 6 ? 'bar-danger' : 'bar-warning'
  return index > 7 ? 'bar-warning' : 'bar-info'
}

const detailEventType = row => activeTab.value === 'history' || eventStatusValue(row) === 'resolved' ? 'history' : 'active'

const arrayValue = value => Array.isArray(value) ? value : []

const objectRows = value => {
  const object = parseSafeJson(value)
  if (!object || typeof object !== 'object' || Array.isArray(object)) return []
  return Object.entries(object).map(([key, val]) => ({ key, value: val }))
}

const timelineAvatar = item => {
  const name = item?.operator || item?.user || item?.createdBy || item?.claimUser || 'S'
  return String(name).trim().slice(0, 1).toUpperCase() || 'S'
}

const datasourceOptionId = item => item?.id ?? item?.value ?? item?.datasourceId ?? ''

const datasourceOptionLabel = item => item?.name || item?.label || datasourceOptionId(item)

onMounted(() => {
  loadList()
  loadDatasourceOptions()
  loadBusinessOptions()
})
</script>

<style scoped>
:global(.home-container .el-main:has(.alert-event-page)) {
  --ops-shell-gutter: 10px;
  --ops-page-padding: 7px;
  --ops-module-gap: 8px;
  padding-top: var(--ops-shell-gutter);
  padding-right: var(--ops-shell-gutter);
  padding-left: var(--ops-shell-gutter);
  overflow-y: hidden;
}

.alert-event-page {
  --ops-text-heading: #0f172a;
  --ops-text-primary: #172033;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-bg-surface: #ffffff;
  --ops-bg-surface-soft: #fbfdff;
  --ops-bg-subtle: #f3f7fc;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  box-sizing: border-box;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: var(--ops-module-gap, 8px);
  height: calc(100vh - 124px);
  max-height: none;
  padding: var(--ops-page-padding, 7px);
  overflow: hidden;
  border-radius: 0;
  background: transparent;
  color: var(--ops-text-primary);
}

.ops-page-header {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 12px;
  min-width: 0;
  padding: 8px 12px;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 10px 28px rgba(80, 124, 180, 0.1);
}

.ops-page-header__icon {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  place-items: center;
  border: 0;
  background: transparent;
}

.ops-page-header__icon img {
  width: 30px;
  height: 30px;
  object-fit: contain;
}

.ops-page-header__content {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.ops-page-header h1 {
  flex: 0 0 auto;
  margin: 0;
  color: var(--ops-text-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: 0;
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

.event-header-summary {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 10px;
  color: var(--ops-text-secondary);
}

.event-header-summary span {
  display: inline-flex;
  align-items: baseline;
  gap: 5px;
  white-space: nowrap;
}

.event-header-summary em {
  color: var(--ops-text-muted);
  font-size: 12px;
  font-style: normal;
}

.event-header-summary strong {
  color: var(--ops-text-heading);
  font-family: Consolas, Monaco, monospace;
  font-size: 16px;
  line-height: 1;
}

.event-header-summary strong.is-danger {
  color: #ef4444;
}

.event-header-summary strong.is-warning {
  color: #f59e0b;
}

.event-header-summary > i {
  width: 1px;
  height: 18px;
  background: var(--ops-border-soft);
}

.alert-event-page :deep(.alert-center-card.el-card) {
  position: relative;
  display: flex;
  min-width: 0;
  min-height: 0;
  height: 100%;
  flex-direction: column;
  overflow: hidden !important;
  border: 1px solid var(--ops-border);
  border-radius: 12px !important;
  background: var(--ops-bg-surface);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
  backdrop-filter: none;
}

.alert-event-page :deep(.alert-center-card::before) {
  display: none;
}

.event-center-card :deep(.el-card__body) {
  box-sizing: border-box;
  display: flex;
  min-height: 0;
  height: 100%;
  flex: 1;
  flex-direction: column;
  padding: 8px 10px 10px;
  overflow: hidden;
  border-radius: 0;
}

.event-status-tabs {
  margin: 0;
}

.event-status-tabs :deep(.el-tabs__header) {
  margin: 0 0 6px;
}

.event-status-tabs :deep(.el-tabs__item) {
  height: 34px;
}
.event-toolbar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
  padding: 8px 10px;
  border: 1px solid var(--ops-border-soft);
  border-radius: 10px;
  background: var(--ops-bg-subtle);
}
.event-filter-row {
  display: flex;
  flex: 1;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.event-keyword-input {
  width: 200px;
  flex: 0 0 200px;
}

.event-filter-select {
  width: 150px;
  flex: 0 0 150px;
}

.silence-matcher-list {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.silence-matcher-row {
  display: grid;
  grid-template-columns: minmax(120px, 1fr) 84px minmax(150px, 1.3fr) 32px;
  align-items: center;
  gap: 8px;
}

.silence-matcher-op {
  width: 84px;
}

.silence-matcher-remove {
  width: 32px;
  height: 32px;
}

.silence-matcher-add {
  align-self: flex-start;
}

@media (max-width: 720px) {
  .silence-matcher-row {
    grid-template-columns: 1fr 76px 32px;
  }

  .silence-matcher-row > :nth-child(3) {
    grid-column: 1 / 3;
    grid-row: 2;
  }

  .silence-matcher-remove {
    grid-column: 3;
    grid-row: 1;
  }
}

.event-icon-button,
.event-batch-button {
  height: 30px;
  min-height: 30px;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 0 10px;
  border-radius: 8px;
  border-color: #e6a23c;
  background: #e6a23c;
  color: #ffffff;
  font-size: 12px;
  box-shadow: none;
}

.event-icon-button :deep(.el-icon),
.event-batch-button :deep(.el-icon) {
  font-size: 14px;
}

.event-icon-button:hover,
.event-icon-button:focus,
.event-batch-button:hover,
.event-batch-button:focus {
  border-color: #cf8d27;
  background: #cf8d27;
  color: #ffffff;
}

.event-query-button,
.event-query-button:hover,
.event-query-button:focus {
  border-color: #409eff;
  background: #409eff;
  color: #ffffff;
}

.event-export-button,
.event-export-button:hover,
.event-export-button:focus {
  border-color: #67c23a;
  background: #67c23a;
  color: #ffffff;
}

.event-silence-button,
.event-silence-button:hover,
.event-silence-button:focus {
  border-color: #409eff;
  background: #409eff;
  color: #ffffff;
}

.ops-refresh-button,
.ops-refresh-button:hover,
.ops-refresh-button:focus {
  border-color: #f3c66b;
  background: #f3c66b;
  color: #ffffff;
}

.event-batch-button {
  flex-shrink: 0;
}

.alert-event-page :deep(.el-button) {
  border-radius: 8px;
  letter-spacing: 0;
}

.alert-event-page :deep(.el-button:hover),
.alert-event-page :deep(.el-button:active) {
  transform: none;
}

.alert-event-page :deep(.el-input__wrapper),
.alert-event-page :deep(.el-select__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 7px;
  background: #ffffff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.alert-event-page :deep(.el-input__wrapper:hover),
.alert-event-page :deep(.el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.alert-event-page :deep(.el-input__wrapper.is-focus),
.alert-event-page :deep(.el-select__wrapper.is-focused) {
  box-shadow: 0 0 0 1px #409eff inset;
}

.event-table-shell {
  border-color: var(--ops-border-soft);
  border-radius: 8px;
  background: #ffffff;
}

.event-table-shell :deep(.el-table),
.event-table-shell :deep(.el-table__inner-wrapper) {
  border-radius: 8px;
}

.event-table :deep(th.el-table__cell) {
  height: 40px;
  padding: 6px 0;
  background: #f3f7fc;
  color: #334155;
  font-size: 13px;
  font-weight: 700;
}

.event-table :deep(td.el-table__cell) {
  height: 56px;
  padding: 6px 0;
}

.event-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: #f7faff;
}

.event-info-cell {
  display: flex;
  align-items: flex-start;
  gap: 7px;
  min-width: 0;
}

.event-level-dot {
  width: 16px;
  height: 20px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 16px;
  color: #f25f45;
}

.event-level-dot.level-p1 {
  color: #f3a329;
}

.event-level-dot.level-p2 {
  color: #4c8bf5;
}

.event-info-main {
  min-width: 0;
  display: grid;
  gap: 2px;
}

.event-title-line {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.event-title-link {
  min-width: 0;
  appearance: none;
  overflow: hidden;
  border: none;
  background: transparent;
  padding: 0;
  color: #2f7df6;
  font: inherit;
  font-size: 13px;
  font-weight: 700;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
}

.event-subtitle-line {
  max-width: 520px;
  overflow: hidden;
  color: var(--aiops-text-muted);
  font-size: 12px;
  line-height: 18px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.event-resource-cell {
  min-width: 0;
  display: grid;
  gap: 3px;
}

.event-resource-name {
  overflow: hidden;
  color: #111827;
  font-size: 13px;
  line-height: 18px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.event-resource-meta {
  overflow: hidden;
  color: #64748b;
  font-size: 12px;
  line-height: 18px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.event-current-value {
  color: #111827;
  font-size: 13px;
  font-weight: 700;
}

.event-duration-cell {
  display: grid;
  gap: 5px;
}

.event-level-tag {
  min-width: 42px;
  justify-content: center;
}

.event-duration-text {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: #111827;
  font-size: 12px;
  font-weight: 700;
}

.event-duration-bars {
  display: inline-flex;
  gap: 3px;
}

.event-duration-bars i {
  width: 8px;
  height: 8px;
  display: inline-block;
  border-radius: 3px;
}

.event-duration-bars .bar-info {
  background: #f2e800;
}

.event-duration-bars .bar-warning {
  background: #ffb000;
}

.event-duration-bars .bar-danger {
  background: #f5293b;
}

.event-claim-cell {
  display: grid;
  justify-items: start;
  gap: 2px;
}

.event-claim-time {
  color: var(--aiops-text-muted);
  font-size: 11px;
}

.event-more-button {
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--aiops-text-secondary);
  cursor: pointer;
}

.event-more-button:hover {
  background: rgba(76, 139, 245, 0.08);
  color: var(--aiops-brand-active);
}

.event-operation-cell {
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

:global(.alert-event-detail-overlay .el-drawer) {
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: 15px 0 0 15px !important;
  background: #ffffff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.alert-event-detail-overlay .el-drawer__header) {
  min-height: 54px;
  margin: 0;
  padding: 14px 16px 12px;
  border-bottom: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.alert-event-detail-overlay .el-drawer__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.alert-event-detail-overlay .el-drawer__body) {
  padding: 12px 0 16px;
  background: #ffffff;
}

:global(.alert-event-detail-overlay .el-button) {
  min-height: 34px;
  border-radius: 8px;
}

:global(.alert-event-detail-overlay .el-button:hover) {
  transform: none;
}

.event-detail-body {
  min-height: 360px;
}

.event-detail-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 16px 10px;
}

.event-detail-actions :deep(.el-button) {
  min-height: 34px;
  border-radius: 8px;
}

.event-detail-table {
  width: auto;
  margin: 0 16px;
  overflow: hidden;
  border: 1px solid #dbe7f4;
  border-radius: 10px;
}
.event-detail-line {
  display: grid;
  grid-template-columns: 124px minmax(0, 1fr);
  min-height: 44px;
  border-bottom: 1px solid #e7eef7;
}

.event-detail-line:last-child {
  border-bottom: none;
}

.event-detail-label {
  display: flex;
  align-items: center;
  padding: 0 12px;
  border-right: 1px solid #e7eef7;
  background: #f3f7fc;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

.event-detail-value {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 7px 12px;
  color: #111827;
  font-size: 13px;
  word-break: break-word;
}

.event-label-list :deep(.el-tag) {
  border-radius: 4px;
  border-color: #8cc4ff;
  background: #eef7ff;
  color: #1677ff;
}

.event-label-list {
  flex-wrap: wrap;
  gap: 8px;
}

.event-code-text {
  font-family: Consolas, Monaco, monospace;
}

.event-inline-time {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.event-detail-textarea-line {
  min-height: auto;
}

.event-detail-textarea-line .event-detail-label {
  align-items: center;
  padding-bottom: 0;
}

.event-detail-textarea-line .event-detail-value {
  align-items: stretch;
  padding-top: 12px;
  padding-bottom: 12px;
}

.event-detail-textbox {
  width: 100%;
  min-height: 38px;
  max-height: 180px;
  padding: 10px 12px;
  overflow: auto;
  border: 1px solid #d7dce3;
  border-radius: 5px;
  background: #fff;
  color: #111827;
  line-height: 1.6;
  white-space: pre-wrap;
}

.event-detail-section {
  margin: 8px 16px 0;
  padding: 10px 12px;
  border: 1px solid #dbe7f4;
  border-radius: 10px;
  background: #fbfdff;
}

.event-detail-section h4 {
  margin: 0;
  margin-bottom: 10px;
  color: #111827;
  font-size: 15px;
  font-weight: 700;
}

.event-notify-section {
  padding-bottom: 10px;
}

.event-compact-empty {
  min-height: 28px;
  display: flex;
  align-items: center;
  color: #8d97a8;
  font-size: 13px;
}

.event-query-list {
  display: grid;
  gap: 10px;
}

.event-query-row {
  padding: 10px 12px;
  border: 1px solid #e5eaf2;
  border-radius: 8px;
  background: #fafcff;
}

.event-query-row.matched {
  border-color: rgba(239, 107, 115, 0.38);
  background: rgba(255, 246, 247, 0.72);
}

.event-query-head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  color: #475569;
  font-size: 13px;
}

.event-query-head strong {
  color: #111827;
}

.event-query-row pre {
  margin: 0;
  overflow: auto;
  color: #1f2937;
  font-family: Consolas, Monaco, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
}

.event-comment-list {
  display: grid;
  gap: 18px;
}

.event-comment-item {
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr);
  gap: 14px;
}

.event-comment-avatar {
  width: 34px;
  height: 34px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: #7561df;
  color: #fff;
  font-size: 15px;
  font-weight: 700;
}

.event-comment-content {
  display: grid;
  gap: 4px;
}

.event-comment-content strong {
  color: #64748b;
  font-size: 14px;
}

.event-comment-content p {
  margin: 0;
  color: #111827;
  font-size: 13px;
  line-height: 1.5;
}

.event-comment-content time {
  color: #8d97a8;
  font-size: 12px;
  font-style: italic;
}

.event-notify-record-panel {
  margin: 0 16px;
  overflow-x: auto;
}

.event-notify-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 10px;
}

.event-notify-stat {
  min-height: 70px;
  display: grid;
  align-content: center;
  gap: 6px;
  padding: 10px 14px;
  border: 1px solid #e6ebf2;
  border-radius: 10px;
  background: #fbfcff;
  color: #475569;
}

.event-notify-stat span {
  font-size: 12px;
}

.event-notify-stat strong {
  color: #111827;
  font-size: 24px;
  line-height: 1;
}

.event-notify-stat.success strong {
  color: #16a34a;
}

.event-notify-stat.failed strong {
  color: #ef4444;
}

.event-notify-scope {
  margin-bottom: 10px;
  color: #64748b;
  font-size: 13px;
}

.event-notify-table {
  min-width: 860px;
}

.event-notify-table :deep(th.el-table__cell) {
  height: 40px;
  background: #f3f7fc;
  color: #334155;
  font-weight: 700;
}

.event-notify-table :deep(td.el-table__cell) {
  height: 46px;
  font-size: 13px;
}

.event-notify-target {
  display: block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.notify-payload-content {
  max-height: 520px;
  margin: 0;
  overflow: auto;
  padding: 12px;
  border: 1px solid #e5eaf2;
  border-radius: 8px;
  background: #f8fafc;
  color: #1f2937;
  font-family: Consolas, Monaco, monospace;
  font-size: 12px;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
}

.full-input-number,
.full-date {
  width: 100%;
}

:global(.alert-event-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: #ffffff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.alert-event-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.alert-event-dialog-overlay .el-dialog__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.alert-event-dialog-overlay .el-dialog__body) {
  padding: 16px 18px;
  color: #53657d;
}

:global(.alert-event-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
}

:global(.alert-event-dialog-overlay .el-button) {
  min-height: 34px;
  border-radius: 8px;
}

:global(.alert-event-dialog-overlay .el-button:hover) {
  transform: none;
}

:global(.alert-event-dialog-overlay .el-input__wrapper),
:global(.alert-event-dialog-overlay .el-select__wrapper),
:global(.alert-event-dialog-overlay .el-textarea__inner) {
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.alert-event-dialog-overlay .el-input__wrapper),
:global(.alert-event-dialog-overlay .el-select__wrapper) {
  min-height: 34px;
}

:global(.alert-event-dialog-overlay .el-input__wrapper:hover),
:global(.alert-event-dialog-overlay .el-select__wrapper:hover),
:global(.alert-event-dialog-overlay .el-textarea__inner:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

:global(.alert-event-dialog-overlay .el-input__wrapper.is-focus),
:global(.alert-event-dialog-overlay .el-select__wrapper.is-focused),
:global(.alert-event-dialog-overlay .el-textarea__inner:focus) {
  box-shadow: 0 0 0 1px #409eff inset;
}

@media (max-width: 1024px) {
  .ops-page-header {
    flex-wrap: wrap;
  }

  .event-toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .event-filter-row {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .event-keyword-input,
  .event-filter-select {
    width: 100%;
    flex: 0 0 auto;
  }

  .event-batch-button {
    align-self: flex-end;
  }

  .event-notify-summary {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .ops-page-header {
    gap: 8px;
  }

  .ops-page-header__content {
    flex-wrap: wrap;
    gap: 2px 8px;
  }

  .ops-page-header__divider {
    display: none;
  }

  .ops-page-header__content p {
    width: 100%;
  }

  .event-header-summary {
    width: 100%;
  }

  .event-filter-row {
    grid-template-columns: 1fr;
  }

  .event-detail-line {
    grid-template-columns: 96px minmax(0, 1fr);
  }

  .event-detail-table,
  .event-detail-section,
  .event-notify-record-panel {
    margin-right: 10px;
    margin-left: 10px;
  }
}
</style>
