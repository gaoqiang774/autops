<template>
  <div class="ai-asset-page">
    <div class="page-shell">
      <div class="page-card">
        <div class="page-header">
          <div>
            <h2>AI 资产分析助手</h2>
            <p class="page-description">
              面向平台内部资产、容量和资源治理的只读查询助手，输出候选主机、判断依据和后续确认项。
            </p>
          </div>
          <div class="summary-group">
            <div v-for="item in headerSummaryCards" :key="item.label" class="summary-chip" :class="item.className">
              <span>{{ item.label }}</span>
              <strong>{{ item.value }}</strong>
            </div>
          </div>
        </div>

        <div class="assistant-layout">
          <aside class="query-panel">
            <div class="panel-head">
              <div>
                <h3>查询条件</h3>
                <p>自然语言 & 能力模板</p>
              </div>
              <el-button type="primary" :icon="Refresh" :loading="providerLoading || groupLoading" @click="reloadOptions">
                刷新
              </el-button>
            </div>

            <el-form :model="queryForm" label-position="top" class="query-form" @submit.prevent>
              <el-form-item label="自然语言问题">
                <el-input
                  v-model.trim="queryForm.question"
                  type="textarea"
                  :rows="3"
                  resize="none"
                  maxlength="500"
                  show-word-limit
                  placeholder="例如：我要部署一个 4C8G 的 Java 服务，帮我找几台资源充足的机器"
                  @keydown.enter.exact.prevent="submitQuery"
                  @keydown.ctrl.enter.prevent="submitQuery"
                />
              </el-form-item>

              <div class="prompt-grid">
                <button
                  v-for="item in promptTemplates"
                  :key="item.title"
                  type="button"
                  class="prompt-card"
                  :class="{ active: inferredIntent === item.intent && queryForm.question === item.question }"
                  @click="applyPrompt(item)"
                >
                  <strong>{{ item.title }}</strong>
                  <span>{{ item.text }}</span>
                </button>
              </div>

              <el-row :gutter="12">
                <el-col :xs="24" :md="12">
                  <el-form-item label="LLM Provider">
                    <el-select
                      v-model="queryForm.providerId"
                      clearable
                      filterable
                      placeholder="默认启用 Provider"
                      :loading="providerLoading"
                    >
                      <el-option
                        v-for="provider in providerOptions"
                        :key="getProviderId(provider)"
                        :label="getProviderLabel(provider)"
                        :value="getProviderId(provider)"
                      />
                    </el-select>
                  </el-form-item>
                </el-col>
                <el-col :xs="24" :md="12">
                  <el-form-item label="CMDB 分组">
                    <el-tree-select
                      v-model="queryForm.groupId"
                      :data="groupOptions"
                      :props="groupTreeProps"
                      node-key="id"
                      clearable
                      filterable
                      check-strictly
                      default-expand-all
                      placeholder="全部分组"
                    />
                  </el-form-item>
                </el-col>
              </el-row>

              <el-collapse class="advanced-collapse">
                <el-collapse-item name="resource">
                  <template #title>
                    <span class="advanced-title">资源需求与阈值</span>
                  </template>
                  <div class="compact-grid">
                    <el-form-item label="CPU 核数">
                      <el-input-number
                        v-model="queryForm.requiredCpuCores"
                        :min="0"
                        :max="512"
                        :step="1"
                        controls-position="right"
                      />
                    </el-form-item>
                    <el-form-item label="内存 GB">
                      <el-input-number
                        v-model="queryForm.requiredMemoryGb"
                        :min="0"
                        :max="4096"
                        :step="1"
                        controls-position="right"
                      />
                    </el-form-item>
                    <el-form-item label="磁盘 GB">
                      <el-input-number
                        v-model="queryForm.requiredDiskGb"
                        :min="0"
                        :max="200000"
                        :step="10"
                        controls-position="right"
                      />
                    </el-form-item>
                    <el-form-item label="CPU 阈值 %">
                      <el-input-number
                        v-model="queryForm.cpuThreshold"
                        :min="0"
                        :max="100"
                        :step="5"
                        controls-position="right"
                      />
                    </el-form-item>
                    <el-form-item label="内存阈值 %">
                      <el-input-number
                        v-model="queryForm.memoryThreshold"
                        :min="0"
                        :max="100"
                        :step="5"
                        controls-position="right"
                      />
                    </el-form-item>
                    <el-form-item label="磁盘阈值 %">
                      <el-input-number
                        v-model="queryForm.diskThreshold"
                        :min="0"
                        :max="100"
                        :step="5"
                        controls-position="right"
                      />
                    </el-form-item>
                  </div>
                  <el-form-item class="switch-item">
                    <el-switch
                      v-model="queryForm.includeOffline"
                      active-text="包含异常主机"
                      inactive-text="仅正常主机"
                    />
                  </el-form-item>
                </el-collapse-item>
              </el-collapse>

              <div class="query-actions">
                <el-button type="primary" :icon="MagicStick" :loading="queryLoading" @click="submitQuery">
                  开始查询
                </el-button>
                <el-button type="warning" :disabled="queryLoading" @click="resetQuery">重置</el-button>
                <el-button text type="danger" :disabled="!result" @click="clearResult">清空结果</el-button>
              </div>
            </el-form>

            <div class="history-panel" v-if="queryHistory.length">
              <div class="history-head">
                <strong>本地查询</strong>
                <button type="button" @click="queryHistory = []">清空</button>
              </div>
              <button
                v-for="item in queryHistory"
                :key="item.id"
                type="button"
                class="history-item"
                @click="reuseHistory(item)"
              >
                <strong>{{ getIntentLabel(item.intent) }}</strong>
                <span>{{ item.question }}</span>
                <small>{{ item.matchedHosts }} 台候选 · {{ item.generatedAt || '-' }}</small>
              </button>
            </div>
          </aside>

          <main class="result-workspace">
            <section class="answer-panel" v-loading="queryLoading">
              <div class="panel-head answer-head">
                <div class="answer-title-block">
                  <h3>分析结论</h3>
                  <p>Answer & summary</p>
                </div>
                <div class="result-tabs">
                  <button
                    v-for="tab in resultTabs"
                    :key="tab.name"
                    type="button"
                    :class="{ active: activeResultTab === tab.name }"
                    @click="activeResultTab = tab.name"
                  >
                    <span>{{ tab.label }}</span>
                    <small v-if="getResultTabCount(tab.name) !== ''">{{ getResultTabCount(tab.name) }}</small>
                  </button>
                </div>
                <span class="intent-pill" :class="`is-${result?.intent || inferredIntent || 'unknown'}`">
                  {{ getIntentLabel(result?.intent || inferredIntent) }}
                </span>
              </div>

              <div class="result-tab-body">
                <template v-if="activeResultTab === 'answer'">
                  <div v-if="result" class="metric-grid">
                    <div
                      v-for="item in summaryStats"
                      :key="item.label"
                      class="metric-card"
                      :class="{ 'metric-card-wide': item.label === '生成时间' }"
                    >
                      <span>{{ item.label }}</span>
                      <strong>{{ item.value }}</strong>
                      <small>{{ item.note }}</small>
                    </div>
                  </div>

                  <div v-if="missingInfo.length" class="missing-info-strip">
                    <strong>数据需确认</strong>
                    <span>{{ missingInfo.join('；') }}</span>
                  </div>

                  <template v-if="result">
                    <div class="answer-card">
                      <div class="answer-icon">
                        <el-icon><CircleCheck /></el-icon>
                      </div>
                      <div>
                        <strong>{{ result.question || queryForm.question }}</strong>
                        <div
                          class="answer-content"
                          v-html="renderAnswer(result.answer || '后端已返回查询结果，请查看下方候选列表与确认项。')"
                        />
                        <div class="answer-meta">
                          <span>{{ resultSourceLabel }}</span>
                          <span>{{ result.generatedAt || '-' }}</span>
                        </div>
                      </div>
                    </div>
                  </template>

                  <el-empty
                    v-else
                    description="输入问题后开始查询"
                    :image-size="96"
                  />
                </template>

                <template v-else-if="activeResultTab === 'candidates'">
                  <el-table
                    v-loading="queryLoading"
                    :data="candidates"
                    stripe
                    class="candidate-table"
                    empty-text="暂无候选主机"
                  >
                    <el-table-column label="主机" min-width="210" fixed="left">
                      <template #default="{ row }">
                        <div class="host-cell">
                          <span class="host-mark">
                            <img :src="linuxIcon" alt="" class="host-os-icon">
                          </span>
                          <div>
                            <strong>{{ getCandidateHostName(row) }}</strong>
                            <span class="mono-text">{{ getCandidateIp(row) }}</span>
                            <small>{{ row.groupName || row.group || '-' }}</small>
                          </div>
                        </div>
                      </template>
                    </el-table-column>

                    <el-table-column label="配置" min-width="150">
                      <template #default="{ row }">
                        <div class="config-stack">
                          <span>{{ formatNumber(row.cpuCores, 0) }} C / {{ formatNumber(row.memoryGb, 0) }} GB</span>
                          <small>磁盘 {{ formatNumber(row.diskTotalGb, 0) }} GB</small>
                        </div>
                      </template>
                    </el-table-column>

                    <el-table-column label="当前负载" min-width="210">
                      <template #default="{ row }">
                        <div class="usage-stack">
                          <MetricLine label="CPU" :value="row.currentCpu" />
                          <MetricLine label="内存" :value="row.currentMemory" />
                          <MetricLine label="磁盘" :value="row.currentDisk" />
                        </div>
                      </template>
                    </el-table-column>

                    <el-table-column label="平均负载" min-width="210">
                      <template #default="{ row }">
                        <div class="usage-stack">
                          <MetricLine label="CPU" :value="row.avgCpu" />
                          <MetricLine label="内存" :value="row.avgMemory" />
                          <MetricLine label="磁盘" :value="row.avgDisk" />
                        </div>
                      </template>
                    </el-table-column>

                    <el-table-column label="服务" min-width="170">
                      <template #default="{ row }">
                        <div class="service-cell">
                          <span>{{ row.serviceCount ?? normalizeStringList(row.services).length }} 个服务指标</span>
                          <div v-if="normalizeStringList(row.services).length" class="tag-row">
                            <el-tag
                              v-for="service in normalizeStringList(row.services).slice(0, 3)"
                              :key="service"
                              size="small"
                              effect="light"
                            >
                              {{ service }}
                            </el-tag>
                          </div>
                        </div>
                      </template>
                    </el-table-column>

                    <el-table-column label="依据 / 风险" min-width="280" show-overflow-tooltip>
                      <template #default="{ row }">
                        <div class="reason-risk-cell">
                          <span v-if="getFirstListText(row.reasons)" class="reason-text">
                            {{ getFirstListText(row.reasons) }}
                          </span>
                          <span v-if="getFirstListText(row.risks)" class="risk-text">
                            {{ getFirstListText(row.risks) }}
                          </span>
                          <span v-if="!getFirstListText(row.reasons) && !getFirstListText(row.risks)" class="muted-text">
                            -
                          </span>
                        </div>
                      </template>
                    </el-table-column>

                    <el-table-column label="评分" width="110" align="center">
                      <template #default="{ row }">
                        <span class="score-pill" :class="getScoreClass(row.score)">
                          {{ formatNumber(row.score, 1) }}
                        </span>
                      </template>
                    </el-table-column>

                    <el-table-column label="建议" min-width="150" show-overflow-tooltip>
                      <template #default="{ row }">
                        <el-tag effect="light" class="recommend-tag">
                          {{ row.recommendation || '-' }}
                        </el-tag>
                      </template>
                    </el-table-column>

                    <el-table-column label="操作" width="100" align="center" fixed="right">
                      <template #default="{ row }">
                        <el-button link type="primary" :icon="View" @click="openCandidateDetail(row)">
                          详情
                        </el-button>
                      </template>
                    </el-table-column>
                  </el-table>
                </template>

                <template v-else-if="activeResultTab === 'evidence'">
                  <el-collapse
                    v-if="evidenceGroups.length"
                    v-model="expandedEvidenceNames"
                    class="evidence-collapse"
                  >
                    <el-collapse-item
                      v-for="group in evidenceGroups"
                      :key="group.name"
                      :name="group.name"
                    >
                      <template #title>
                        <span class="evidence-title">
                          <span>{{ group.title }}</span>
                          <small>{{ group.count }}</small>
                        </span>
                      </template>
                      <div class="support-list">
                        <div
                          v-for="(item, index) in group.items"
                          :key="`${group.name}-${index}`"
                          class="support-item"
                          :class="{ 'is-warning': group.type === 'warning' }"
                        >
                          {{ item }}
                        </div>
                      </div>
                    </el-collapse-item>
                  </el-collapse>
                  <el-empty v-else description="暂无证据项" :image-size="96" />
                </template>

                <template v-else>
                  <div class="boundary-list">
                    <span>不执行部署、回收、关机或命令变更</span>
                    <span>候选结果需要结合业务归属继续确认</span>
                    <span>服务判断依赖已上报的指标数据</span>
                  </div>
                </template>
              </div>
            </section>
          </main>
        </div>
      </div>
    </div>

    <el-drawer
      v-model="detailDrawerVisible"
      :title="selectedCandidate ? getCandidateHostName(selectedCandidate) : '候选详情'"
      size="560px"
      class="candidate-drawer"
    >
      <template v-if="selectedCandidate">
        <div class="drawer-hero">
          <div>
            <span class="mono-text">{{ getCandidateIp(selectedCandidate) }}</span>
            <h3>{{ selectedCandidate.recommendation || '候选主机' }}</h3>
            <p>{{ selectedCandidate.groupName || selectedCandidate.group || '-' }}</p>
          </div>
          <span class="score-pill large" :class="getScoreClass(selectedCandidate.score)">
            {{ formatNumber(selectedCandidate.score, 1) }}
          </span>
        </div>

        <div class="drawer-stats">
          <div>
            <span>CPU</span>
            <strong>{{ formatNumber(selectedCandidate.cpuCores, 0) }} C</strong>
          </div>
          <div>
            <span>内存</span>
            <strong>{{ formatNumber(selectedCandidate.memoryGb, 0) }} GB</strong>
          </div>
          <div>
            <span>磁盘</span>
            <strong>{{ formatNumber(selectedCandidate.diskTotalGb, 0) }} GB</strong>
          </div>
          <div>
            <span>服务指标</span>
            <strong>{{ selectedCandidate.serviceCount ?? normalizeStringList(selectedCandidate.services).length }}</strong>
          </div>
        </div>

        <section class="drawer-section">
          <h4>判断依据</h4>
          <div v-if="normalizeStringList(selectedCandidate.reasons).length" class="reason-list">
            <span v-for="item in normalizeStringList(selectedCandidate.reasons)" :key="item">
              {{ item }}
            </span>
          </div>
          <el-empty v-else description="暂无判断依据" :image-size="64" />
        </section>

        <section class="drawer-section warning">
          <h4>风险提示</h4>
          <div v-if="normalizeStringList(selectedCandidate.risks).length" class="reason-list">
            <span v-for="item in normalizeStringList(selectedCandidate.risks)" :key="item">
              {{ item }}
            </span>
          </div>
          <el-empty v-else description="暂无风险项" :image-size="64" />
        </section>

        <section class="drawer-section">
          <h4>下一步确认</h4>
          <div v-if="normalizeStringList(selectedCandidate.nextChecks).length" class="check-list">
            <span v-for="item in normalizeStringList(selectedCandidate.nextChecks)" :key="item">
              {{ item }}
            </span>
          </div>
          <el-empty v-else description="暂无确认项" :image-size="64" />
        </section>

        <section v-if="normalizeStringList(selectedCandidate.services).length" class="drawer-section">
          <h4>服务指标</h4>
          <div class="tag-row">
            <el-tag
              v-for="service in normalizeStringList(selectedCandidate.services)"
              :key="service"
              effect="light"
            >
              {{ service }}
            </el-tag>
          </div>
        </section>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, defineComponent, h, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  CircleCheck,
  MagicStick,
  Refresh,
  View
} from '@element-plus/icons-vue'
import aiopsApi from '@/api/aiops'
import cmdbApi from '@/api/cmdb'
import linuxIcon from '@/assets/image/linux.svg'

const successCodes = new Set([0, 200])

const MetricLine = defineComponent({
  name: 'MetricLine',
  props: {
    label: { type: String, required: true },
    value: { type: [Number, String], default: null }
  },
  setup(props) {
    const percent = computed(() => {
      const number = Number(props.value)
      if (!Number.isFinite(number)) return null
      return Math.max(0, Math.min(100, number))
    })
    const level = computed(() => {
      if (percent.value === null) return 'muted'
      if (percent.value >= 85) return 'danger'
      if (percent.value >= 70) return 'warning'
      return 'success'
    })
    const text = computed(() => (percent.value === null ? '-' : `${percent.value.toFixed(1)}%`))

    return () => h('div', { class: ['metric-line', `is-${level.value}`] }, [
      h('span', props.label),
      h('div', { class: 'metric-bar' }, [
        h('i', { style: { width: `${percent.value ?? 0}%` } })
      ]),
      h('strong', text.value)
    ])
  }
})

const createQueryForm = () => ({
  question: '',
  providerId: '',
  groupId: '',
  hostIds: [],
  keyword: '',
  env: '',
  timeRangeDays: 7,
  limit: 10,
  requiredCpuCores: null,
  requiredMemoryGb: null,
  requiredDiskGb: null,
  cpuThreshold: null,
  memoryThreshold: null,
  diskThreshold: null,
  includeOffline: false
})

const queryForm = reactive(createQueryForm())
const queryLoading = ref(false)
const providerLoading = ref(false)
const groupLoading = ref(false)
const providerOptions = ref([])
const groupOptions = ref([])
const result = ref(null)
const selectedCandidate = ref(null)
const detailDrawerVisible = ref(false)
const queryHistory = ref([])
const activeResultTab = ref('answer')
const expandedEvidenceNames = ref(['scope', 'metrics', 'backend', 'missing'])

const groupTreeProps = {
  children: 'children',
  label: 'name',
  value: 'id'
}

const promptTemplates = [
  {
    intent: 'resource_placement',
    title: '资源选址',
    text: '推荐可部署目标',
    question: '我要部署一个 4C8G 的 Java 服务，帮我找几台资源充足的机器',
    defaults: {
      timeRangeDays: 7,
      limit: 5,
      requiredCpuCores: 4,
      requiredMemoryGb: 8,
      cpuThreshold: 55,
      memoryThreshold: 65,
      diskThreshold: 80
    }
  },
  {
    intent: 'resource_reclaim',
    title: '资源回收',
    text: '筛出低负载候选',
    question: '当前哪些机器资源使用率长期比较低，而且又没有部署服务，可以作为回收候选？',
    defaults: {
      timeRangeDays: 30,
      limit: 10,
      cpuThreshold: 10,
      memoryThreshold: 30,
      diskThreshold: 50
    }
  },
  {
    intent: 'capacity_inspection',
    title: '容量巡检',
    text: '定位高压力主机',
    question: '当前哪些主机资源压力比较高？',
    defaults: {
      timeRangeDays: 7,
      limit: 10,
      cpuThreshold: 80,
      memoryThreshold: 80,
      diskThreshold: 85
    }
  },
  {
    intent: 'asset_query',
    title: '资产查询',
    text: '汇总配置和负载',
    question: '生产应用组有哪些机器，配置和负载如何？',
    defaults: {
      timeRangeDays: 7,
      limit: 20
    }
  },
  {
    intent: 'resource_placement',
    title: '扩容选址',
    text: '8C16G 资源推荐',
    question: '帮我找几台适合部署 8C16G 服务的机器',
    defaults: {
      timeRangeDays: 7,
      limit: 5,
      requiredCpuCores: 8,
      requiredMemoryGb: 16,
      cpuThreshold: 60,
      memoryThreshold: 70,
      diskThreshold: 80
    }
  },
  {
    intent: 'resource_reclaim',
    title: '空闲治理',
    text: '30 天低负载无服务',
    question: '最近 30 天哪些机器负载低且没有服务？',
    defaults: {
      timeRangeDays: 30,
      limit: 10,
      cpuThreshold: 10,
      memoryThreshold: 30,
      diskThreshold: 50
    }
  },
  {
    intent: 'capacity_inspection',
    title: '磁盘风险',
    text: '查找磁盘快满主机',
    question: '哪些主机磁盘快满了？',
    defaults: {
      timeRangeDays: 7,
      limit: 10,
      diskThreshold: 85
    }
  },
  {
    intent: 'asset_query',
    title: '单机画像',
    text: '按 IP 查配置负载',
    question: '查一下 172.16.3.12 这台机器配置和负载',
    defaults: {
      timeRangeDays: 7,
      limit: 5
    }
  }
]

const intentLabelMap = {
  resource_placement: '资源选址',
  resource_reclaim: '资源回收',
  capacity_inspection: '容量巡检',
  asset_query: '资产查询',
  unknown: '待识别'
}

const resultTabs = [
  { name: 'answer', label: '分析结论' },
  { name: 'candidates', label: '候选主机' },
  { name: 'evidence', label: '全局证据' },
  { name: 'boundary', label: '结果边界' }
]

const getBody = response => response?.data || response || {}

const assertSuccess = (response, fallbackMessage = '请求失败') => {
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
  if (Array.isArray(payload?.rows)) return payload.rows
  if (Array.isArray(payload?.data)) return payload.data
  return []
}

const normalizeStringList = value => {
  if (Array.isArray(value)) {
    return value
      .map(item => (typeof item === 'string' ? item : formatSupportItem(item)))
      .filter(Boolean)
  }
  if (value === undefined || value === null || value === '') return []
  return [String(value)]
}

const isFilled = value => value !== undefined && value !== null && value !== ''

const toMaybeNumber = value => {
  if (!isFilled(value)) return undefined
  const number = Number(value)
  return Number.isFinite(number) ? number : value
}

const compactObject = object => Object.entries(object).reduce((payload, [key, value]) => {
  if (Array.isArray(value)) {
    if (value.length) payload[key] = value
    return payload
  }
  if (isFilled(value)) payload[key] = value
  return payload
}, {})

const candidates = computed(() => toList(result.value?.candidates))
const summary = computed(() => result.value?.summary || {})
const evidenceItems = computed(() => normalizeStringList(result.value?.evidence))
const missingInfo = computed(() => normalizeStringList(result.value?.missingInfo))
const resultProviderLabel = computed(() => {
  const data = result.value || {}
  const name = data.providerName || data.modelName || data.providerType || (data.providerId ? `Provider ${data.providerId}` : '')
  const model = data.modelCode || data.model || ''

  if (name && model) return `${name} (${model})`
  return name || model || 'LLM 总结'
})
const resultSourceLabel = computed(() => (result.value?.usedLlm ? `规则分析 + ${resultProviderLabel.value}` : '规则分析'))
const requestPayload = computed(() => result.value?.requestPayload || {})
const requestOptions = computed(() => requestPayload.value.options || {})
const requestScope = computed(() => requestPayload.value.scope || result.value?.scope || {})

const readAny = (object, keys) => {
  for (const key of keys) {
    if (isFilled(object?.[key])) return object[key]
  }
  return undefined
}

const parsedRequirement = computed(() => parseResourceRequirement(result.value?.question || queryForm.question))

const resolvedRequiredCpuCores = computed(() =>
  readAny(summary.value, ['requiredCpuCores', 'required_cpu_cores', 'cpuCores', 'requiredCpu'])
  ?? readAny(requestOptions.value, ['requiredCpuCores', 'required_cpu_cores', 'cpuCores', 'requiredCpu'])
  ?? queryForm.requiredCpuCores
  ?? parsedRequirement.value.cpu
)

const resolvedRequiredMemoryGb = computed(() =>
  readAny(summary.value, ['requiredMemoryGb', 'required_memory_gb', 'memoryGb', 'requiredMemory'])
  ?? readAny(requestOptions.value, ['requiredMemoryGb', 'required_memory_gb', 'memoryGb', 'requiredMemory'])
  ?? queryForm.requiredMemoryGb
  ?? parsedRequirement.value.memory
)

const resolvedRequiredDiskGb = computed(() =>
  readAny(summary.value, ['requiredDiskGb', 'required_disk_gb', 'diskGb', 'requiredDisk'])
  ?? readAny(requestOptions.value, ['requiredDiskGb', 'required_disk_gb', 'diskGb', 'requiredDisk'])
  ?? queryForm.requiredDiskGb
  ?? parsedRequirement.value.disk
)

const evidenceGroups = computed(() => {
  if (!result.value) return []

  const groups = []
  const scopeItems = []
  const metricItems = []
  const totalHosts = readAny(summary.value, ['totalHosts', 'total_hosts'])
  const analyzedHosts = readAny(summary.value, ['analyzedHosts', 'analyzed_hosts'])
  const matchedHosts = readAny(summary.value, ['matchedHosts', 'matched_hosts'])
  const timeRangeDays = readAny(summary.value, ['timeRangeDays', 'time_range_days'])
    ?? requestScope.value.timeRangeDays
    ?? queryForm.timeRangeDays
  const thresholdText = formatThresholdText()

  if (isFilled(totalHosts) || isFilled(analyzedHosts) || isFilled(matchedHosts)) {
    scopeItems.push(
      `CMDB 主机范围：命中 ${displayValue(totalHosts)} 台，已分析 ${displayValue(analyzedHosts)} 台，候选 ${displayValue(matchedHosts ?? candidates.value.length)} 台`
    )
  }

  const scopeFilters = []
  if (isFilled(requestScope.value.groupId)) scopeFilters.push(`分组 ${requestScope.value.groupId}`)
  if (Array.isArray(requestScope.value.hostIds) && requestScope.value.hostIds.length) {
    scopeFilters.push(`指定 ${requestScope.value.hostIds.length} 台主机`)
  }
  if (isFilled(requestScope.value.keyword)) scopeFilters.push(`关键词 ${requestScope.value.keyword}`)
  if (isFilled(requestScope.value.env)) scopeFilters.push(`环境 ${requestScope.value.env}`)
  if (scopeFilters.length) scopeItems.push(`筛选条件：${scopeFilters.join('，')}`)

  if (scopeItems.length) {
    groups.push({
      name: 'scope',
      title: 'CMDB 范围',
      count: scopeItems.length,
      items: scopeItems
    })
  }

  if (isFilled(timeRangeDays)) {
    metricItems.push(`Prometheus 指标范围：最近 ${timeRangeDays} 天`)
  }
  if (thresholdText !== '- / - / -') {
    metricItems.push(`阈值：CPU / 内存 / 磁盘 = ${thresholdText}`)
  }
  if (metricItems.length) {
    groups.push({
      name: 'metrics',
      title: 'Prometheus 指标',
      count: metricItems.length,
      items: metricItems
    })
  }

  if (evidenceItems.value.length) {
    groups.push({
      name: 'backend',
      title: '后端证据',
      count: evidenceItems.value.length,
      items: evidenceItems.value
    })
  }

  if (missingInfo.value.length) {
    groups.push({
      name: 'missing',
      title: '缺失数据',
      count: missingInfo.value.length,
      items: missingInfo.value,
      type: 'warning'
    })
  }

  return groups
})

const getResultTabCount = name => {
  if (name === 'candidates') return candidates.value.length
  if (name === 'evidence') return evidenceItems.value.length + missingInfo.value.length
  if (name === 'boundary') return '只读'
  return ''
}

const inferredIntent = computed(() => {
  const question = queryForm.question || ''
  if (/部署|上线|扩容|资源充足|选址/.test(question)) return 'resource_placement'
  if (/回收|释放|闲置|空闲|低负载|没有部署|无服务/.test(question)) return 'resource_reclaim'
  if (/巡检|高负载|压力|瓶颈|容量|磁盘|快满|阈值|CPU|内存/.test(question)) return 'capacity_inspection'
  if (/资产|机器|主机|列表|概览/.test(question)) return 'asset_query'
  return 'unknown'
})

const headerSummaryCards = computed(() => [
  {
    label: '分析主机',
    value: displayValue(summary.value.analyzedHosts ?? summary.value.totalHosts)
  },
  {
    label: '命中候选',
    value: displayValue(summary.value.matchedHosts ?? candidates.value.length)
  },
  {
    label: '统计天数',
    value: displayValue(summary.value.timeRangeDays ?? queryForm.timeRangeDays)
  },
  {
    label: '结果来源',
    value: result.value ? resultSourceLabel.value : '-',
    className: 'summary-chip-source'
  }
])

const summaryStats = computed(() => [
  {
    label: '总主机',
    value: displayValue(summary.value.totalHosts),
    note: '命中范围'
  },
  {
    label: '已分析',
    value: displayValue(summary.value.analyzedHosts),
    note: '进入评估'
  },
  {
    label: '候选',
    value: displayValue(summary.value.matchedHosts ?? candidates.value.length),
    note: '返回结果'
  },
  {
    label: '资源需求',
    value: formatResourceDemand(),
    note: 'CPU / 内存 / 磁盘'
  },
  {
    label: '阈值',
    value: formatThresholdText(),
    note: 'CPU / 内存 / 磁盘'
  },
  {
    label: '生成时间',
    value: result.value?.generatedAt || '-',
    note: '后端返回'
  }
])

const displayValue = value => {
  if (value === undefined || value === null || value === '') return '-'
  return value
}

const formatResourceDemand = () => {
  const cpu = displayValue(resolvedRequiredCpuCores.value)
  const memory = displayValue(resolvedRequiredMemoryGb.value)
  const disk = displayValue(resolvedRequiredDiskGb.value)
  const parts = [
    cpu !== '-' ? `${cpu}C` : '',
    memory !== '-' ? `${memory}G` : '',
    disk !== '-' ? `${disk}G` : ''
  ].filter(Boolean)

  return parts.length ? parts.join(' / ') : '-'
}

const formatThresholdText = () => {
  const cpu = displayValue(
    readAny(summary.value, ['cpuThreshold', 'cpu_threshold'])
    ?? readAny(requestOptions.value, ['cpuThreshold', 'cpu_threshold'])
    ?? queryForm.cpuThreshold
  )
  const memory = displayValue(
    readAny(summary.value, ['memoryThreshold', 'memory_threshold'])
    ?? readAny(requestOptions.value, ['memoryThreshold', 'memory_threshold'])
    ?? queryForm.memoryThreshold
  )
  const disk = displayValue(
    readAny(summary.value, ['diskThreshold', 'disk_threshold'])
    ?? readAny(requestOptions.value, ['diskThreshold', 'disk_threshold'])
    ?? queryForm.diskThreshold
  )
  return `${cpu} / ${memory} / ${disk}`
}

const formatNumber = (value, digits = 1) => {
  const number = Number(value)
  if (!Number.isFinite(number)) return '-'
  return number.toFixed(digits)
}

const formatSupportItem = item => {
  if (item === undefined || item === null || item === '') return ''
  if (typeof item === 'string') return item
  try {
    return JSON.stringify(item)
  } catch {
    return String(item)
  }
}

const parseResourceRequirement = value => {
  const text = String(value || '').replace(/\s+/g, '')
  const cpuMemoryMatch = text.match(/(\d+(?:\.\d+)?)C(\d+(?:\.\d+)?)G/i)
  const cpuMatch = text.match(/(?:CPU|cpu|处理器|核数)?(\d+(?:\.\d+)?)(?:核|C|c)/)
  const memoryMatch = text.match(/(\d+(?:\.\d+)?)(?:GB|G|g)(?:内存)?/)
  const diskMatch = text.match(/(?:磁盘|硬盘|存储)(\d+(?:\.\d+)?)(?:GB|G|g)/) || text.match(/(\d+(?:\.\d+)?)(?:GB|G|g)(?:磁盘|硬盘|存储)/)

  return {
    cpu: cpuMemoryMatch ? Number(cpuMemoryMatch[1]) : (cpuMatch ? Number(cpuMatch[1]) : undefined),
    memory: cpuMemoryMatch ? Number(cpuMemoryMatch[2]) : (memoryMatch ? Number(memoryMatch[1]) : undefined),
    disk: diskMatch ? Number(diskMatch[1]) : undefined
  }
}

const escapeHtml = value =>
  String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')

const numericDangerThreshold = computed(() => {
  const values = [
    readAny(summary.value, ['cpuThreshold', 'cpu_threshold']),
    readAny(summary.value, ['memoryThreshold', 'memory_threshold']),
    readAny(summary.value, ['diskThreshold', 'disk_threshold']),
    readAny(requestOptions.value, ['cpuThreshold', 'cpu_threshold']),
    readAny(requestOptions.value, ['memoryThreshold', 'memory_threshold']),
    readAny(requestOptions.value, ['diskThreshold', 'disk_threshold']),
    queryForm.cpuThreshold,
    queryForm.memoryThreshold,
    queryForm.diskThreshold
  ]
    .map(value => Number(value))
    .filter(Number.isFinite)

  if (!values.length) return 80
  return Math.max(80, Math.max(...values) - 2)
})

const isIpv4Token = token => /^(?:\d{1,3}\.){3}\d{1,3}$/.test(token)
const isPercentToken = token => /^\d+(?:\.\d+)?%$/.test(token)

const getNumericTokenClass = token => {
  if (isIpv4Token(token)) return 'num-token is-mid'

  if (isPercentToken(token)) {
    const value = Number(token.replace('%', ''))
    if (!Number.isFinite(value)) return 'num-token is-mid'
    if (value >= numericDangerThreshold.value) return 'num-token is-danger'
    if (value <= 35) return 'num-token is-safe'
    return 'num-token is-mid'
  }

  if (/^\d+(?:\.\d+)?$/.test(token)) return 'num-token is-safe'
  return 'num-token is-mid'
}

const highlightNumericTokens = html => {
  const tokenPattern = /(?:\b(?:\d{1,3}\.){3}\d{1,3}\b|\b\d+(?:\.\d+)?%|\b\d+(?:\.\d+)?\b)/g
  const parts = String(html || '').split(/(<[^>]+>|&#\d+;)/g)
  let insideCode = false

  return parts.map(part => {
    if (!part) return part
    if (part.startsWith('&#')) return part
    if (part.startsWith('<')) {
      if (/^<code>$/i.test(part)) insideCode = true
      if (/^<\/code>$/i.test(part)) insideCode = false
      return part
    }
    if (insideCode) return part
    return part.replace(tokenPattern, token => `<span class="${getNumericTokenClass(token)}">${token}</span>`)
  }).join('')
}

const renderInlineMarkdown = text =>
  highlightNumericTokens(
    escapeHtml(text)
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
  )

const isMarkdownTableRow = value => {
  const text = String(value || '').trim()
  return text.startsWith('|') && text.endsWith('|') && text.split('|').length >= 4
}

const splitMarkdownTableRow = value =>
  String(value || '')
    .trim()
    .replace(/^\|/, '')
    .replace(/\|$/, '')
    .split('|')
    .map(item => item.trim())

const isMarkdownTableSeparator = value => {
  if (!isMarkdownTableRow(value)) return false
  const cells = splitMarkdownTableRow(value)
  return cells.length > 1 && cells.every(cell => /^:?-{2,}:?$/.test(cell))
}

const renderMarkdownTable = rows => {
  const header = splitMarkdownTableRow(rows[0])
  const bodyRows = rows.slice(2).map(splitMarkdownTableRow)
  const columnCount = header.length
  const renderCells = (cells, tag) => Array.from({ length: columnCount }, (_, index) => {
    const content = cells[index] || ''
    return `<${tag}>${renderInlineMarkdown(content)}</${tag}>`
  }).join('')

  return [
    '<div class="markdown-table-wrap">',
    '<table class="markdown-table">',
    '<thead>',
    `<tr>${renderCells(header, 'th')}</tr>`,
    '</thead>',
    '<tbody>',
    ...bodyRows.map(cells => `<tr>${renderCells(cells, 'td')}</tr>`),
    '</tbody>',
    '</table>',
    '</div>'
  ].join('')
}

const renderAnswer = value => {
  const lines = String(value || '').replace(/\r\n/g, '\n').split('\n')
  const html = []
  let listType = ''

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
    const trimmed = line.trim()

    if (!trimmed) {
      closeList()
      continue
    }

    if (
      isMarkdownTableRow(trimmed)
      && index + 1 < lines.length
      && isMarkdownTableSeparator(lines[index + 1])
    ) {
      closeList()
      const tableRows = [trimmed, lines[index + 1]]
      index += 2
      while (index < lines.length && isMarkdownTableRow(lines[index])) {
        tableRows.push(lines[index])
        index += 1
      }
      index -= 1
      html.push(renderMarkdownTable(tableRows))
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

    const sectionLike = trimmed.match(/^(.{2,24})[:：]$/)
    if (sectionLike) {
      closeList()
      html.push(`<h4>${renderInlineMarkdown(sectionLike[1])}</h4>`)
      continue
    }

    closeList()
    html.push(`<p>${renderInlineMarkdown(trimmed)}</p>`)
  }

  closeList()
  return html.join('')
}

const getIntentLabel = intent => intentLabelMap[intent] || intentLabelMap.unknown

const getProviderId = provider => provider?.id ?? provider?.providerId ?? provider?.value ?? ''

const normalizeProviderId = value => (isFilled(value) ? String(value) : '')

const isSameProviderId = (left, right) => normalizeProviderId(left) === normalizeProviderId(right)

const isDefaultProvider = provider => {
  const value = provider?.isDefault
    ?? provider?.is_default
    ?? provider?.defaultProvider
    ?? provider?.default_provider
    ?? provider?.defaultFlag
    ?? provider?.default_flag
    ?? provider?.default

  return value === true || value === 1 || value === '1' || String(value).toLowerCase() === 'true'
}

const getDefaultProvider = () => providerOptions.value.find(isDefaultProvider) || providerOptions.value[0] || null

const syncDefaultProvider = (force = false) => {
  const defaultProviderId = getProviderId(getDefaultProvider())
  if (!isFilled(defaultProviderId)) return

  const currentExists = providerOptions.value.some(provider => isSameProviderId(getProviderId(provider), queryForm.providerId))
  if (force || !isFilled(queryForm.providerId) || !currentExists) {
    queryForm.providerId = defaultProviderId
  }
}

const getProviderLabel = provider => {
  const name = provider?.modelName || provider?.name || provider?.providerName || `Provider ${getProviderId(provider)}`
  const model = provider?.modelCode || provider?.model || ''
  return model ? `${name} (${model})` : name
}

const getCandidateHostName = row => row?.hostName || row?.hostname || row?.name || `Host ${row?.hostId ?? '-'}`

const getCandidateIp = row => row?.ip || row?.privateIp || row?.publicIp || row?.sshIp || '-'

const getScoreClass = score => {
  const number = Number(score)
  if (!Number.isFinite(number)) return 'is-muted'
  if (number >= 75) return 'is-good'
  if (number >= 55) return 'is-normal'
  return 'is-risk'
}

const getFirstListText = value => normalizeStringList(value)[0] || ''

const buildPayload = () => {
  const parsed = parseResourceRequirement(queryForm.question)
  const scope = compactObject({
    groupId: toMaybeNumber(queryForm.groupId),
    hostIds: queryForm.hostIds.map(toMaybeNumber).filter(isFilled),
    keyword: queryForm.keyword,
    env: queryForm.env,
    timeRangeDays: toMaybeNumber(queryForm.timeRangeDays),
    limit: toMaybeNumber(queryForm.limit)
  })

  const options = compactObject({
    requiredCpuCores: toMaybeNumber(queryForm.requiredCpuCores ?? parsed.cpu),
    requiredMemoryGb: toMaybeNumber(queryForm.requiredMemoryGb ?? parsed.memory),
    requiredDiskGb: toMaybeNumber(queryForm.requiredDiskGb ?? parsed.disk),
    cpuThreshold: toMaybeNumber(queryForm.cpuThreshold),
    memoryThreshold: toMaybeNumber(queryForm.memoryThreshold),
    diskThreshold: toMaybeNumber(queryForm.diskThreshold),
    includeOffline: queryForm.includeOffline ? true : undefined
  })

  const payload = {
    question: queryForm.question.trim()
  }

  if (isFilled(queryForm.providerId)) payload.providerId = toMaybeNumber(queryForm.providerId)
  if (Object.keys(scope).length) payload.scope = scope
  if (Object.keys(options).length) payload.options = options

  return payload
}

const submitQuery = async () => {
  if (!queryForm.question.trim()) {
    ElMessage.warning('请输入查询问题')
    return
  }

  queryLoading.value = true
  try {
    const payload = buildPayload()
    const response = await aiopsApi.queryAssistant(payload)
    const data = getPayload(response, 'AI 查询失败') || {}
    result.value = {
      ...(typeof data === 'object' && !Array.isArray(data) ? data : { answer: String(data) }),
      requestPayload: payload
    }
    activeResultTab.value = 'answer'
    addHistory(result.value)
  } catch (error) {
    console.error('query assistant failed:', error)
    ElMessage.error(error.message || 'AI 查询失败')
  } finally {
    queryLoading.value = false
  }
}

const addHistory = data => {
  const historyItem = {
    id: Date.now(),
    question: data.question || queryForm.question,
    intent: data.intent || inferredIntent.value,
    matchedHosts: data.summary?.matchedHosts ?? toList(data.candidates).length,
    generatedAt: data.generatedAt || '',
    result: data
  }

  queryHistory.value = [
    historyItem,
    ...queryHistory.value.filter(item => item.question !== historyItem.question)
  ].slice(0, 8)
}

const applyPrompt = prompt => {
  queryForm.question = prompt.question
  queryForm.requiredCpuCores = null
  queryForm.requiredMemoryGb = null
  queryForm.requiredDiskGb = null
  queryForm.cpuThreshold = null
  queryForm.memoryThreshold = null
  queryForm.diskThreshold = null
  Object.assign(queryForm, prompt.defaults)
}

const reuseHistory = item => {
  queryForm.question = item.question
  result.value = item.result
  activeResultTab.value = 'answer'
}

const resetQuery = () => {
  Object.assign(queryForm, createQueryForm())
  syncDefaultProvider(true)
}

const clearResult = () => {
  result.value = null
  selectedCandidate.value = null
  detailDrawerVisible.value = false
  activeResultTab.value = 'answer'
}

const openCandidateDetail = row => {
  selectedCandidate.value = row
  detailDrawerVisible.value = true
}

const loadProviders = async () => {
  providerLoading.value = true
  try {
    const response = await aiopsApi.getEnabledLLMProviders({ page: 1, pageSize: 100 })
    providerOptions.value = toList(getPayload(response, '获取 Provider 失败'))
    syncDefaultProvider()
  } catch (error) {
    console.warn('load providers failed:', error)
    providerOptions.value = []
  } finally {
    providerLoading.value = false
  }
}

const loadGroups = async () => {
  groupLoading.value = true
  try {
    const response = await cmdbApi.getAllCmdbGroups()
    groupOptions.value = toList(getPayload(response, '获取 CMDB 分组失败'))
  } catch (error) {
    console.warn('load groups failed:', error)
    groupOptions.value = []
  } finally {
    groupLoading.value = false
  }
}

const reloadOptions = () => {
  loadProviders()
  loadGroups()
}

onMounted(() => {
  reloadOptions()
})
</script>

<style scoped>
.ai-asset-page {
  --aiops-surface-cool: rgba(247, 250, 255, 0.84);
  --aiops-surface-warm: rgba(255, 253, 248, 0.94);
  --aiops-text-primary: #172133;
  --aiops-text-secondary: #5f6b7c;
  --aiops-text-muted: #8d97a8;
  --aiops-border-cool: #dfe7f2;
  --aiops-border-mid: rgba(207, 217, 230, 0.92);
  --aiops-brand: #4c8bf5;
  --aiops-brand-active: #4d79d8;
  --aiops-brand-soft: rgba(76, 139, 245, 0.12);
  --aiops-success: #3fb97f;
  --aiops-warning: #f3b24b;
  --aiops-danger: #ef6b73;
  --aiops-topbar-gradient: linear-gradient(90deg, #fef7ef 0%, #f8f8f6 52%, #edf2fd 100%);
  padding: 5px;
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
  background: transparent;
  color: var(--aiops-text-primary);
}

.page-shell {
  width: 100%;
  margin: 0 auto;
}

.page-card {
  position: relative;
  min-width: 0;
  padding: 50px 7px 7px;
  border: 1px solid rgba(223, 231, 242, 0.94);
  border-radius: 28px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.96), rgba(247, 250, 255, 0.92));
  box-shadow: 0 18px 48px rgba(17, 24, 39, 0.08);
  overflow: hidden;
  isolation: isolate;
}

.page-card::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 40px;
  background: var(--aiops-topbar-gradient);
  z-index: 0;
}

.page-header {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  margin: -50px -7px 5px;
  padding: 12px 7px 12px;
  background: var(--aiops-topbar-gradient);
  border-bottom: 1px solid rgba(223, 231, 242, 0.94);
}

.page-kicker {
  margin: 0 0 10px;
  color: #6d7b8f;
  font-size: 12px;
  letter-spacing: 0.12em;
  font-weight: 600;
  text-transform: uppercase;
}

.page-header h2 {
  margin: 0;
  color: var(--aiops-text-primary);
  font-size: 32px;
  font-weight: 700;
  line-height: 1.18;
}

.page-description {
  margin: 12px 0 0;
  max-width: 760px;
  color: var(--aiops-text-secondary);
  font-size: 14px;
  line-height: 1.75;
}

.summary-group {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
}

.summary-chip {
  min-width: 104px;
  padding: 12px 14px 10px;
  border-radius: 16px;
  border: 1px solid rgba(214, 223, 235, 0.96);
  background: rgba(255, 255, 255, 0.88);
}

.summary-chip span {
  display: block;
  color: var(--aiops-text-muted);
  font-size: 11px;
}

.summary-chip strong {
  display: block;
  margin-top: 4px;
  color: var(--aiops-text-primary);
  font-size: 20px;
  font-weight: 700;
  white-space: nowrap;
}

.summary-chip-source strong {
  font-size: 15px;
  line-height: 1.28;
}

.assistant-layout {
  position: relative;
  z-index: 1;
  display: grid;
  grid-template-columns: minmax(320px, 380px) minmax(0, 1fr);
  gap: 16px;
  align-items: stretch;
  height: calc(100vh - 270px);
  min-height: 0;
  max-height: 680px;
  min-width: 0;
  overflow: hidden;
}

.query-panel,
.answer-panel {
  min-width: 0;
  border-radius: 24px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.95), rgba(248, 251, 255, 0.92));
  box-shadow: 0 12px 34px rgba(17, 24, 39, 0.05);
}

.query-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 10px;
  overflow: hidden;
}

.result-workspace {
  display: flex;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}

.panel-head,
.table-head,
.support-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 16px;
  padding-bottom: 14px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.86);
}

.panel-head h3,
.table-head h3 {
  margin: 0;
  color: var(--aiops-text-primary);
  font-size: 18px;
  font-weight: 700;
}

.panel-head p,
.table-head p {
  margin: 6px 0 0;
  color: var(--aiops-text-muted);
  font-size: 12px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.answer-head {
  flex-wrap: nowrap;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
  padding-bottom: 12px;
}

.answer-title-block {
  flex: 0 0 auto;
  min-width: 100px;
}

.answer-head .intent-pill {
  align-self: center;
}

.result-tabs {
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 3px;
  border-radius: 16px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  background: rgba(255, 255, 255, 0.58);
}

.result-tabs button {
  min-width: 0;
  flex: 1 1 0;
  height: 32px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 0 8px;
  border: 0;
  border-radius: 12px;
  background: transparent;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.result-tabs button span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-tabs button:hover,
.result-tabs button.active {
  color: var(--aiops-brand-active);
  background: rgba(76, 139, 245, 0.1);
}

.result-tabs small {
  min-width: 18px;
  padding: 1px 5px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.86);
  color: var(--aiops-text-muted);
  font-size: 10px;
  line-height: 17px;
}

.query-panel .panel-head {
  margin-bottom: 9px;
  padding-bottom: 9px;
}

.query-panel :deep(.el-form-item) {
  margin-bottom: 9px;
}

.query-panel :deep(.el-form-item__label) {
  height: 18px;
  margin-bottom: 3px;
  padding: 0;
  line-height: 18px;
}

.query-panel :deep(.el-input__wrapper),
.query-panel :deep(.el-select__wrapper),
.query-panel :deep(.el-input-number .el-input__wrapper) {
  min-height: 30px;
}

.query-panel :deep(.el-input__inner) {
  height: 30px;
  line-height: 30px;
}

.query-panel :deep(.el-textarea__inner) {
  min-height: 74px !important;
}

.query-panel :deep(.el-collapse-item__header) {
  height: 34px;
  line-height: 34px;
}

.query-panel :deep(.el-collapse-item__content) {
  padding-bottom: 8px;
}

.query-actions :deep(.el-button) {
  height: 32px;
  flex: 0 1 auto;
  min-width: 0;
  padding: 8px 12px;
}

.query-actions :deep(.el-button--primary) {
  flex-shrink: 0;
}

.query-actions :deep(.el-button.is-text) {
  padding-inline: 8px;
}

.query-actions :deep(.el-button--warning:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: transparent;
  background: linear-gradient(180deg, #f6c45a 0%, #f0a93a 100%);
  color: #ffffff;
}

.query-actions :deep(.el-button--danger.is-text) {
  color: var(--aiops-danger);
}

.query-actions :deep(.el-button--danger.is-text:not(.is-disabled):hover),
.query-actions :deep(.el-button--danger.is-text:not(.is-disabled):focus) {
  background: rgba(239, 107, 115, 0.1);
}

.query-form {
  display: grid;
  gap: 0;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 2px;
}

.prompt-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 7px;
  margin: 0 0 8px;
}

.prompt-card {
  width: 100%;
  min-height: 46px;
  padding: 7px 9px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.76);
  text-align: left;
  cursor: pointer;
  transition: border-color 0.2s ease, background-color 0.2s ease, transform 0.2s ease;
}

.prompt-card:hover,
.prompt-card.active {
  border-color: rgba(76, 139, 245, 0.34);
  background: rgba(238, 245, 255, 0.86);
  transform: translateY(-1px);
}

.prompt-card strong,
.prompt-card span {
  display: block;
}

.prompt-card strong {
  color: var(--aiops-text-primary);
  font-size: 12px;
  font-weight: 700;
}

.prompt-card span {
  margin-top: 3px;
  color: var(--aiops-text-muted);
  font-size: 11px;
  line-height: 1.35;
}

.compact-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 12px;
}

.advanced-collapse {
  margin: 0 0 8px;
  border: 1px solid rgba(223, 231, 242, 0.84);
  border-radius: 16px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.62);
}

.advanced-title {
  color: var(--aiops-text-primary);
  font-weight: 700;
}

.switch-item {
  margin-bottom: 0;
}

.query-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: nowrap;
  min-width: 0;
}

.history-panel {
  margin-top: 18px;
  padding-top: 16px;
  border-top: 1px solid rgba(223, 231, 242, 0.86);
}

.history-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.history-head strong {
  color: var(--aiops-text-primary);
  font-size: 13px;
}

.history-head button {
  border: 0;
  background: transparent;
  color: var(--aiops-text-muted);
  cursor: pointer;
}

.history-item {
  width: 100%;
  display: grid;
  gap: 4px;
  margin-top: 8px;
  padding: 10px 12px;
  border-radius: 14px;
  border: 1px solid rgba(223, 231, 242, 0.72);
  background: rgba(255, 255, 255, 0.62);
  text-align: left;
  cursor: pointer;
}

.history-item strong {
  color: var(--aiops-brand-active);
  font-size: 12px;
}

.history-item span,
.history-item small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.history-item span {
  color: var(--aiops-text-secondary);
  font-size: 13px;
}

.history-item small {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.answer-panel {
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 16px;
}

.result-tab-body {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  padding-right: 2px;
}

.intent-pill,
.selection-pill {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  min-height: 26px;
  padding: 0 10px;
  border-radius: 999px;
  border: 1px solid rgba(76, 139, 245, 0.18);
  background: rgba(76, 139, 245, 0.08);
  color: var(--aiops-brand-active);
  font-size: 12px;
  font-weight: 700;
  white-space: nowrap;
}

.intent-pill.is-resource_reclaim {
  border-color: rgba(243, 178, 75, 0.32);
  background: rgba(255, 248, 238, 0.92);
  color: #b86b14;
}

.intent-pill.is-capacity_inspection {
  border-color: rgba(239, 107, 115, 0.24);
  background: rgba(255, 243, 244, 0.92);
  color: #c7565f;
}

.answer-card {
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr);
  gap: 14px;
  padding: 16px;
  border-radius: 20px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  background: rgba(255, 255, 255, 0.76);
}

.answer-icon {
  width: 42px;
  height: 42px;
  border-radius: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--aiops-success);
  background: rgba(63, 185, 127, 0.1);
}

.answer-card > div > strong {
  display: block;
  margin: 0;
}

.answer-card > div > strong {
  color: var(--aiops-text-primary);
  font-size: 15px;
  font-weight: 700;
  line-height: 1.6;
}

.answer-content {
  margin-top: 10px;
  color: var(--aiops-text-secondary);
  font-size: 14px;
  line-height: 1.75;
}

.answer-content :deep(p) {
  margin: 8px 0 0;
  color: var(--aiops-text-secondary);
  font-size: 14px;
  line-height: 1.8;
}

.answer-content :deep(h3),
.answer-content :deep(h4),
.answer-content :deep(h5) {
  position: relative;
  margin: 18px 0 8px;
  padding-left: 12px;
  color: var(--aiops-text-primary);
  font-weight: 700;
  line-height: 1.5;
}

.answer-content :deep(h3) {
  font-size: 16px;
}

.answer-content :deep(h4),
.answer-content :deep(h5) {
  font-size: 15px;
}

.answer-content :deep(h3::before),
.answer-content :deep(h4::before),
.answer-content :deep(h5::before) {
  content: '';
  position: absolute;
  left: 0;
  top: 0.48em;
  width: 4px;
  height: 1em;
  border-radius: 999px;
  background: var(--aiops-brand);
}

.answer-content :deep(strong) {
  color: var(--aiops-text-primary);
  font-weight: 700;
}

.answer-content :deep(ol),
.answer-content :deep(ul) {
  display: grid;
  gap: 8px;
  margin: 10px 0 0;
  padding-left: 22px;
  color: var(--aiops-text-secondary);
}

.answer-content :deep(li) {
  padding-left: 2px;
  line-height: 1.75;
}

.answer-content :deep(ol > li::marker) {
  color: var(--aiops-brand-active);
  font-weight: 700;
}

.answer-content :deep(ul > li::marker) {
  color: var(--aiops-brand-active);
}

.answer-content :deep(code) {
  padding: 2px 6px;
  border-radius: 8px;
  background: rgba(76, 139, 245, 0.08);
  color: var(--aiops-brand-active);
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
}

.answer-content :deep(.markdown-table-wrap) {
  max-width: 100%;
  margin: 10px 0 12px;
  overflow-x: auto;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.78);
}

.answer-content :deep(.markdown-table) {
  width: 100%;
  min-width: 720px;
  border-collapse: collapse;
  color: var(--aiops-text-secondary);
  font-size: 13px;
}

.answer-content :deep(.markdown-table th),
.answer-content :deep(.markdown-table td) {
  padding: 8px 10px;
  border-right: 1px solid rgba(223, 231, 242, 0.68);
  border-bottom: 1px solid rgba(223, 231, 242, 0.78);
  text-align: left;
  vertical-align: top;
  line-height: 1.55;
  white-space: normal;
}

.answer-content :deep(.markdown-table th:last-child),
.answer-content :deep(.markdown-table td:last-child) {
  border-right: 0;
}

.answer-content :deep(.markdown-table tbody tr:last-child td) {
  border-bottom: 0;
}

.answer-content :deep(.markdown-table th) {
  background: rgba(247, 250, 255, 0.96);
  color: var(--aiops-text-primary);
  font-weight: 750;
}

.answer-content :deep(.num-token) {
  display: inline-flex;
  align-items: center;
  padding: 1px 6px;
  border-radius: 7px;
  border: 1px solid transparent;
  font-weight: 600;
  line-height: 1.45;
}

.answer-content :deep(.num-token.is-safe) {
  color: #22784b;
  background: rgba(63, 185, 127, 0.14);
  border-color: rgba(63, 185, 127, 0.28);
}

.answer-content :deep(.num-token.is-mid) {
  color: #335283;
  background: rgba(76, 139, 245, 0.12);
  border-color: rgba(76, 139, 245, 0.24);
}

.answer-content :deep(.num-token.is-danger) {
  color: #b44b54;
  background: rgba(239, 107, 115, 0.14);
  border-color: rgba(239, 107, 115, 0.26);
}

.answer-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
}

.answer-meta span {
  padding: 4px 9px;
  border-radius: 999px;
  background: rgba(76, 139, 245, 0.08);
  color: var(--aiops-brand-active);
  font-size: 12px;
  font-weight: 600;
}

.metric-grid {
  display: grid;
  flex: 1 1 auto;
  grid-template-columns: repeat(6, minmax(86px, 1fr));
  gap: 8px;
  min-width: 0;
  margin: 0;
}

.result-tab-body > .metric-grid {
  margin-bottom: 12px;
}

.missing-info-strip {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 10px;
  align-items: start;
  margin: 0 0 12px;
  padding: 10px 12px;
  border-radius: 14px;
  border: 1px solid rgba(243, 178, 75, 0.34);
  background: rgba(255, 248, 238, 0.88);
  color: #8a5a1d;
  font-size: 13px;
  line-height: 1.6;
}

.missing-info-strip strong {
  color: #a65f12;
  font-weight: 700;
  white-space: nowrap;
}

.missing-info-strip span {
  min-width: 0;
  color: var(--aiops-text-secondary);
}

.evidence-warning {
  margin-bottom: 8px;
}

.metric-card {
  min-width: 0;
  padding: 7px 9px;
  border-radius: 13px;
  border: 1px solid rgba(223, 231, 242, 0.84);
  background: rgba(255, 255, 255, 0.66);
}

.metric-card-wide {
  min-width: 172px;
}

.metric-card span,
.metric-card strong,
.metric-card small {
  display: block;
}

.metric-card span {
  color: var(--aiops-text-muted);
  font-size: 11px;
  line-height: 1.2;
}

.metric-card strong {
  margin-top: 4px;
  color: var(--aiops-text-primary);
  font-size: 16px;
  font-weight: 700;
  word-break: break-all;
  line-height: 1.2;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.metric-card-wide strong {
  font-size: 14px;
}

.metric-card small {
  margin-top: 3px;
  color: var(--aiops-text-muted);
  font-size: 11px;
  line-height: 1.2;
  white-space: nowrap;
}

.host-cell {
  display: grid;
  grid-template-columns: 36px minmax(0, 1fr);
  gap: 10px;
  align-items: center;
}

.host-mark {
  width: 36px;
  height: 36px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  overflow: visible;
  background: transparent;
  border: 0;
  box-shadow: none;
}

.host-os-icon {
  width: 24px;
  height: 24px;
  display: block;
  object-fit: contain;
  transform: translateY(0);
}

.host-cell strong,
.host-cell span,
.host-cell small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.host-cell strong {
  color: var(--aiops-text-primary);
  font-size: 14px;
  font-weight: 700;
}

.host-cell small {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.mono-text {
  color: var(--aiops-text-secondary);
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
}

.config-stack,
.service-cell {
  display: grid;
  gap: 5px;
  color: var(--aiops-text-secondary);
}

.config-stack small {
  color: var(--aiops-text-muted);
}

.reason-risk-cell {
  display: grid;
  gap: 5px;
  min-width: 0;
}

.reason-text,
.risk-text,
.muted-text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12px;
  line-height: 1.45;
}

.reason-text {
  color: var(--aiops-text-secondary);
}

.risk-text {
  color: #c7565f;
}

.muted-text {
  color: var(--aiops-text-muted);
}

.usage-stack {
  display: grid;
  gap: 7px;
}

:deep(.metric-line) {
  display: grid;
  grid-template-columns: 34px minmax(70px, 1fr) 48px;
  gap: 8px;
  align-items: center;
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

:deep(.metric-line .metric-bar) {
  height: 6px;
  border-radius: 999px;
  background: rgba(223, 231, 242, 0.78);
  overflow: hidden;
}

:deep(.metric-line .metric-bar i) {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--aiops-success);
}

:deep(.metric-line.is-warning .metric-bar i) {
  background: var(--aiops-warning);
}

:deep(.metric-line.is-danger .metric-bar i) {
  background: var(--aiops-danger);
}

:deep(.metric-line.is-muted .metric-bar i) {
  background: var(--aiops-text-muted);
}

:deep(.metric-line strong) {
  color: var(--aiops-text-primary);
  font-size: 12px;
  text-align: right;
}

.tag-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.score-pill {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 54px;
  height: 28px;
  padding: 0 10px;
  border-radius: 999px;
  font-size: 13px;
  font-weight: 700;
}

.score-pill.large {
  min-width: 68px;
  height: 38px;
  font-size: 16px;
}

.score-pill.is-good {
  background: rgba(63, 185, 127, 0.12);
  color: #2f9368;
}

.score-pill.is-normal {
  background: rgba(76, 139, 245, 0.1);
  color: var(--aiops-brand-active);
}

.score-pill.is-risk {
  background: rgba(239, 107, 115, 0.12);
  color: #c7565f;
}

.score-pill.is-muted {
  background: rgba(141, 151, 168, 0.12);
  color: var(--aiops-text-muted);
}

.recommend-tag {
  color: var(--aiops-brand-active);
  background: rgba(76, 139, 245, 0.08);
}

.support-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(260px, 0.72fr);
  gap: 16px;
}

.support-head {
  align-items: center;
  margin-bottom: 12px;
}

.support-head strong {
  color: var(--aiops-text-primary);
  font-size: 15px;
}

.support-head span {
  color: var(--aiops-text-muted);
  font-size: 12px;
  font-weight: 700;
}

.support-list,
.boundary-list {
  display: grid;
  gap: 8px;
}

.ai-asset-page .evidence-collapse {
  border: 0;
}

.ai-asset-page .evidence-collapse :deep(.el-collapse-item) {
  margin-bottom: 8px;
  border: 1px solid rgba(223, 231, 242, 0.78);
  border-radius: 16px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.58);
}

.ai-asset-page .evidence-collapse :deep(.el-collapse-item__header) {
  height: 40px;
  padding: 0 12px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.68);
}

.ai-asset-page .evidence-collapse :deep(.el-collapse-item__content) {
  padding: 10px 12px 12px;
}

.evidence-title {
  width: 100%;
  display: inline-flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  color: var(--aiops-text-primary);
  font-size: 13px;
  font-weight: 700;
}

.evidence-title small {
  min-width: 24px;
  padding: 1px 8px;
  border-radius: 999px;
  background: rgba(76, 139, 245, 0.09);
  color: var(--aiops-brand-active);
  font-size: 11px;
  line-height: 18px;
  text-align: center;
}

.support-item,
.boundary-list span {
  padding: 10px 12px;
  border-radius: 14px;
  border: 1px solid rgba(223, 231, 242, 0.72);
  background: rgba(255, 255, 255, 0.62);
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.support-item.is-warning {
  border-color: rgba(243, 178, 75, 0.34);
  background: rgba(255, 248, 238, 0.84);
  color: #8a5a1d;
}

.drawer-hero {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
  padding: 18px;
  border-radius: 22px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
}

.drawer-hero h3 {
  margin: 8px 0 4px;
  color: var(--aiops-text-primary);
  font-size: 20px;
  font-weight: 700;
}

.drawer-hero p {
  margin: 0;
  color: var(--aiops-text-muted);
  font-size: 13px;
}

.drawer-stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 16px;
}

.drawer-stats div {
  padding: 12px;
  border-radius: 16px;
  border: 1px solid rgba(223, 231, 242, 0.84);
  background: rgba(255, 255, 255, 0.68);
}

.drawer-stats span,
.drawer-stats strong {
  display: block;
}

.drawer-stats span {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.drawer-stats strong {
  margin-top: 6px;
  color: var(--aiops-text-primary);
  font-size: 16px;
}

.drawer-section {
  margin-top: 14px;
  padding: 16px;
  border-radius: 20px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  background: rgba(255, 255, 255, 0.66);
}

.drawer-section.warning {
  border-color: rgba(255, 192, 120, 0.62);
  background: rgba(255, 248, 238, 0.84);
}

.drawer-section h4 {
  margin: 0 0 12px;
  color: var(--aiops-text-primary);
  font-size: 15px;
  font-weight: 700;
}

.reason-list,
.check-list {
  display: grid;
  gap: 8px;
}

.reason-list span,
.check-list span {
  position: relative;
  padding-left: 16px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.65;
}

.reason-list span::before,
.check-list span::before {
  content: '';
  position: absolute;
  top: 9px;
  left: 0;
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: var(--aiops-brand);
}

.ai-asset-page :deep(.el-form-item) {
  margin-right: 0;
}

.ai-asset-page :deep(.el-form-item__label) {
  color: var(--aiops-text-primary);
  font-size: 13px;
  font-weight: 600;
}

.ai-asset-page :deep(.el-input),
.ai-asset-page :deep(.el-select),
.ai-asset-page :deep(.el-input-number) {
  width: 100%;
}

.ai-asset-page :deep(.el-input__wrapper),
.ai-asset-page :deep(.el-select__wrapper),
.ai-asset-page :deep(.el-textarea__inner),
.ai-asset-page :deep(.el-input-number .el-input__wrapper) {
  border-radius: 16px;
  background: rgba(252, 253, 255, 0.98);
  border: 1px solid transparent;
  box-shadow: inset 0 0 0 1px rgba(214, 223, 235, 0.96) !important;
  transition: box-shadow 0.2s ease, background-color 0.2s ease;
}

.ai-asset-page :deep(.el-input__wrapper:hover),
.ai-asset-page :deep(.el-select__wrapper:hover),
.ai-asset-page :deep(.el-textarea__inner:hover),
.ai-asset-page :deep(.el-input-number .el-input__wrapper:hover) {
  box-shadow: inset 0 0 0 1px rgba(171, 189, 220, 0.96) !important;
}

.ai-asset-page :deep(.el-input__wrapper.is-focus),
.ai-asset-page :deep(.el-select__wrapper.is-focused),
.ai-asset-page :deep(.el-textarea__inner:focus),
.ai-asset-page :deep(.el-input-number .el-input__wrapper.is-focus) {
  background: #ffffff;
  box-shadow: inset 0 0 0 1px rgba(120, 156, 221, 0.96), 0 0 0 4px rgba(76, 139, 245, 0.08) !important;
}

.ai-asset-page :deep(.el-button) {
  border-radius: 14px;
  font-weight: 600;
  box-shadow: none !important;
  transform: none !important;
}

.ai-asset-page :deep(.el-button--primary:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: transparent;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
}

.ai-asset-page :deep(.el-button + .el-button) {
  margin-left: 0;
}

.ai-asset-page :deep(.el-tag) {
  border-radius: 999px;
  border-color: transparent;
  font-weight: 600;
}

.ai-asset-page :deep(.candidate-table.el-table) {
  --el-table-border-color: rgba(223, 231, 242, 0.92);
  --el-table-header-bg-color: rgba(248, 250, 255, 0.94);
  --el-table-tr-bg-color: rgba(255, 255, 255, 0.78);
  --el-table-row-hover-bg-color: rgba(76, 139, 245, 0.05);
  --el-table-bg-color: transparent;
  --el-table-text-color: var(--aiops-text-secondary);
  --el-table-header-text-color: var(--aiops-text-muted);
  width: 100%;
  max-width: 100%;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 18px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.7);
}

.ai-asset-page :deep(.candidate-table .el-table__inner-wrapper::before) {
  display: none;
}

.ai-asset-page :deep(.candidate-table th.el-table__cell) {
  font-weight: 600;
}

.ai-asset-page :deep(.candidate-table td.el-table__cell),
.ai-asset-page :deep(.candidate-table th.el-table__cell.is-leaf) {
  border-bottom-color: rgba(223, 231, 242, 0.7);
}

.ai-asset-page :deep(.candidate-table .el-table__row:hover > td.el-table__cell) {
  background: rgba(76, 139, 245, 0.05) !important;
}

.ai-asset-page :deep(.el-collapse) {
  --el-collapse-header-bg-color: transparent;
  --el-collapse-content-bg-color: transparent;
  --el-collapse-border-color: transparent;
}

.ai-asset-page :deep(.el-collapse-item__header) {
  padding: 0 14px;
}

.ai-asset-page :deep(.el-collapse-item__content) {
  padding: 0 14px 14px;
}

.ai-asset-page :deep(.el-drawer) {
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.95));
}

.ai-asset-page :deep(.el-drawer__header) {
  margin-bottom: 0;
  padding: 18px 22px 16px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.94);
  color: var(--aiops-text-primary);
  font-weight: 700;
}

.ai-asset-page :deep(.el-drawer__body) {
  padding: 20px 22px;
}

@media (max-width: 1280px) {
  .assistant-layout {
    grid-template-columns: 1fr;
    height: auto;
    max-height: none;
  }

  .query-panel {
    align-self: stretch;
    max-height: none;
  }

  .query-form {
    grid-template-columns: 1fr;
    overflow: visible;
  }
}

@media (max-width: 980px) {
  .page-header {
    flex-direction: column;
    align-items: stretch;
  }

  .summary-group {
    justify-content: flex-start;
  }

  .metric-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 768px) {
  .ai-asset-page {
    padding: 4px;
  }

  .page-card {
    padding: 44px 6px 6px;
    border-radius: 22px;
  }

  .page-card::before {
    height: 30px;
  }

  .page-header {
    margin: -44px -6px 5px;
    padding: 10px 6px 10px;
  }

  .page-header h2 {
    font-size: 24px;
  }

  .summary-chip,
  .query-actions :deep(.el-button) {
    width: 100%;
  }

  .prompt-grid,
  .metric-grid,
  .compact-grid,
  .drawer-stats {
    grid-template-columns: 1fr;
  }

  .answer-card {
    grid-template-columns: 1fr;
  }

  .panel-head,
  .answer-head,
  .drawer-hero {
    flex-direction: column;
    align-items: stretch;
  }

  .result-tabs {
    min-width: 0;
    flex-wrap: wrap;
  }

  .result-tabs button {
    flex-basis: calc(50% - 3px);
  }
}
</style>
