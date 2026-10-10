<template>
  <div class="nginx-dashboard-page log-page" :class="{ 'is-embedded': embedded }">
    <section class="page-card" :class="{ 'is-embedded-card': embedded }">
    <header class="page-header dashboard-header">
      <div>
        <h2>{{ embedded ? '业务监控' : '业务访问日志' }}</h2>
      </div>
      <div class="header-actions">
        <span v-if="overview.dataStatus === 'ready'" class="health-state is-ready">数据正常</span>
        <span v-else-if="overview.dataStatus === 'partial' || overview.partial" class="health-state is-partial">部分可用</span>
        <span class="scope-text">{{ scopeDescription }}</span>
        <span class="updated-at">{{ lastUpdatedText }}</span>
        <el-switch v-model="autoRefresh" inline-prompt active-text="自动" inactive-text="手动" @change="toggleAutoRefresh" />
        <el-tooltip content="管理域名与日志库绑定" placement="bottom">
          <el-button  :icon="Setting" plain aria-label="管理域名绑定" @click="openBindingDrawer">
            绑定管理
          </el-button>
        </el-tooltip>
        <el-tooltip content="刷新大盘" placement="bottom">
          <el-button :icon="Refresh" circle plain :loading="overviewLoading" aria-label="刷新大盘" @click="loadOverview" />
        </el-tooltip>
      </div>
    </header>

    <div class="report-content">
    <section class="filter-panel" aria-label="大盘筛选条件">
      <el-form class="filter-grid" :model="filters" @submit.prevent>
        <el-form-item label="域名">
          <el-select v-model="selectedBindingId" filterable placeholder="请选择业务域名" :loading="optionsLoading" :disabled="!domains.length" @change="handleDomainChange">
            <el-option v-for="item in domains" :key="item.id || `${item.storeId}-${item.domain}`" :label="domainLabel(item)" :value="item.id">
              <div class="option-row"><span>{{ domainLabel(item) }}</span><small>{{ item.storeName || item.storeCode }}</small></div>
            </el-option>
          </el-select>
        </el-form-item>
        <div class="time-filter">
          <span class="filter-label">时间范围</span>
          <div class="time-filter-content">
            <div class="time-presets" role="group" aria-label="时间范围">
              <el-button
                v-for="item in windowOptions"
                :key="item.value"
                class="time-preset"
                :class="{ 'is-active': filters.window === item.value }"
                :type="filters.window === item.value ? 'primary' : 'default'"
                plain
                @click="selectWindow(item.value)"
              >
                {{ item.label }}
              </el-button>
            </div>
            <el-date-picker
              v-if="filters.window === 'custom'"
              v-model="filters.customRange"
              class="custom-range-picker"
              type="datetimerange"
              range-separator="至"
              start-placeholder="开始时间"
              end-placeholder="结束时间"
              :clearable="false"
              :disabled-date="disableFutureDate"
              @change="handleCustomRangeChange"
            />
          </div>
        </div>
      </el-form>
    </section>

    <el-alert
      v-if="warningMessages.length"
      class="status-alert"
      type="warning"
      show-icon
      :closable="false"
      title="部分数据暂时不可用"
    >
      <template #default>
        <details>
          <summary>查看 {{ warningMessages.length }} 条后端提示</summary>
          <p v-for="(warning, index) in warningMessages" :key="index">{{ warning }}</p>
        </details>
      </template>
    </el-alert>

    <section v-if="unauthorized" class="page-state">
      <el-result icon="warning" title="暂无查看权限" sub-title="请联系管理员开通 logs:nginx-dashboard:view 权限" />
    </section>

    <section v-else-if="overviewError || overview.dataStatus === 'unavailable'" class="page-state">
      <el-result icon="error" title="大盘数据暂时不可用" :sub-title="unavailableDescription">
        <template #extra><el-button type="primary" :icon="Refresh" @click="loadOverview">重新加载</el-button></template>
      </el-result>
    </section>

    <section v-else-if="overview.dataStatus === 'unconfigured'" class="page-state">
      <el-result icon="info" title="尚未配置 NGINX 日志源" sub-title="请先将业务域名绑定到对应的 NGINX 日志库">
        <template #extra>
          <el-button v-authority="['logs:nginx-dashboard:manage']" type="primary" :icon="Setting" @click="openBindingDrawer">新增域名绑定</el-button>
        </template>
      </el-result>
    </section>

    <div v-else-if="overviewLoading && !hasLoaded" class="dashboard-skeleton" aria-label="大盘加载中">
      <el-skeleton animated :rows="12" />
    </div>

    <section v-else-if="hasLoaded && !hasDashboardData" class="page-state">
      <el-empty description="当前时间范围暂无日志">
        <el-button type="primary" plain :icon="Refresh" @click="loadOverview">刷新数据</el-button>
      </el-empty>
    </section>

    <main v-else class="dashboard-content" :class="{ 'is-refreshing': overviewLoading }">
      <section class="metric-grid" aria-label="核心指标">
        <article v-for="metric in metricCards" :key="metric.key" class="metric-card" :class="`tone-${metric.tone}`">
          <div class="metric-label">
            <span>{{ metric.label }}</span>
            <el-icon><component :is="metric.icon" /></el-icon>
          </div>
          <strong>{{ metric.value }}</strong>
          <small>{{ metric.hint }}</small>
          <div v-if="metric.comparison && (hasValue(metric.comparison.previousChangePercent) || hasValue(metric.comparison.weekAgoChangePercent))" class="metric-comparison">
            <span v-if="hasValue(metric.comparison.previousChangePercent)" :class="changeTone(metric.comparison.previousChangePercent)">环比 {{ formatChange(metric.comparison.previousChangePercent) }}</span>
            <span v-if="hasValue(metric.comparison.weekAgoChangePercent)" :class="changeTone(metric.comparison.weekAgoChangePercent)">同比 {{ formatChange(metric.comparison.weekAgoChangePercent) }}</span>
          </div>
        </article>
      </section>

      <section class="primary-grid">
        <article class="dashboard-panel trend-panel">
          <header class="panel-header">
            <div><h3>请求趋势</h3><p>{{ windowDescription }}</p></div>
            <span class="panel-unit">单位：请求数</span>
          </header>
          <div v-if="overview.trend.length" ref="trendChartRef" class="chart chart-large" role="img" aria-label="NGINX 请求趋势折线图"></div>
          <el-empty v-else description="暂无趋势数据" :image-size="72" />
        </article>

        <article class="dashboard-panel status-panel">
          <header class="panel-header"><div><h3>状态码分布</h3><p>按 HTTP 状态码聚合</p></div></header>
          <div v-if="overview.statuses.length" ref="statusChartRef" class="chart chart-large" role="img" aria-label="HTTP 状态码分布环形图"></div>
          <el-empty v-else description="日志未提供状态码字段" :image-size="72" />
        </article>
      </section>

      <section class="analytics-grid">
        <div class="analytics-column analytics-column-left">
          <article class="dashboard-panel api-panel">
            <header class="panel-header">
              <div><h3>Top API</h3><p>按请求量展示热门接口</p></div>
              <span class="panel-count">{{ overview.topApis.length }} 条</span>
            </header>
            <el-table v-if="overview.topApis.length" class="data-table" :data="overview.topApis" :max-height="topApiTableHeight">
              <el-table-column label="方法" width="78"><template #default="{ row }"><el-tag size="small" effect="plain" :type="methodTagType(row.requestMethod)">{{ row.requestMethod || '-' }}</el-tag></template></el-table-column>
              <el-table-column prop="requestPath" label="接口路径" min-width="230" show-overflow-tooltip />
              <el-table-column label="PV" width="86" align="right"><template #default="{ row }">{{ formatInteger(row.requestTotal) }}</template></el-table-column>
              <el-table-column label="4xx" width="72" align="right"><template #default="{ row }">{{ formatInteger(row.http4xx) }}</template></el-table-column>
              <el-table-column label="5xx" width="72" align="right"><template #default="{ row }"><span :class="{ 'danger-value': Number(row.http5xx) > 0 }">{{ formatInteger(row.http5xx) }}</span></template></el-table-column>
              <el-table-column label="错误率" width="92" align="right"><template #default="{ row }">{{ formatPercent(row.errorRate) }}</template></el-table-column>
              <el-table-column label="平均响应" width="104" align="right"><template #default="{ row }">{{ formatLatency(row.avgLatencyMs) }}</template></el-table-column>
              <el-table-column label="P95" width="92" align="right"><template #default="{ row }">{{ formatLatency(row.p95LatencyMs) }}</template></el-table-column>
              <el-table-column label="响应字节" width="104" align="right"><template #default="{ row }">{{ formatBytes(row.responseBytes) }}</template></el-table-column>
            </el-table>
            <el-empty v-else description="暂无 API 聚合数据" :image-size="70" />
          </article>

          <article v-if="overview.ipRegions.length" class="dashboard-panel geo-panel">
            <header class="panel-header">
              <div><h3>访问用户 IP 分布（中国）</h3><p>按客户端 IP 归属省份聚合</p></div>
              <span class="panel-unit">单位：请求数</span>
            </header>
            <div ref="ipRegionChartRef" class="chart chart-geo" role="img" aria-label="中国省份访问用户 IP 分布图"></div>
          </article>
        </div>

        <div class="analytics-column analytics-column-right">
          <div class="rank-column">
            <article v-for="panel in primaryRankPanels" :key="panel.key" class="dashboard-panel rank-panel">
              <header class="panel-header"><div><h3>{{ panel.title }}</h3><p>{{ panel.description }}</p></div></header>
              <div v-if="panel.items.length" class="rank-list">
                <div v-for="(item, index) in panel.items.slice(0, 6)" :key="`${panel.key}-${index}-${rankName(item, panel.keys)}`" class="rank-item">
                  <div class="rank-copy"><span :title="rankName(item, panel.keys)"><i>{{ index + 1 }}</i>{{ rankName(item, panel.keys) }}</span><strong>{{ formatInteger(rankValue(item)) }}</strong></div>
                  <div class="rank-track"><span :style="{ width: rankWidth(item, panel.items) }"></span></div>
                </div>
              </div>
              <el-empty v-else :description="`${panel.title}暂无数据`" :image-size="46" />
            </article>
          </div>

          <article v-if="overview.ipRegions.length" class="dashboard-panel geo-top-panel">
            <header class="panel-header"><div><h3>Top5 省份</h3><p>访问请求量最高的省份</p></div></header>
            <div class="geo-rank-list">
              <div v-for="(item, index) in overview.ipRegions.slice(0, 5)" :key="`geo-${item.province}`" class="geo-rank-item">
                <span><i>{{ index + 1 }}</i>{{ item.province }}</span>
                <strong>{{ formatInteger(item.requestTotal) }}</strong>
                <div class="geo-rank-track"><span :style="{ width: rankWidth(item, overview.ipRegions) }"></span></div>
              </div>
            </div>
          </article>
        </div>
      </section>

      <section class="rank-grid">
        <article v-for="panel in secondaryRankPanels" :key="panel.key" class="dashboard-panel rank-panel rank-panel-tall">
          <header class="panel-header"><div><h3>{{ panel.title }}</h3><p>{{ panel.description }}</p></div></header>
          <div v-if="panel.items.length" class="rank-list">
            <div v-for="(item, index) in panel.items.slice(0, 8)" :key="`${panel.key}-${index}-${rankName(item, panel.keys)}`" class="rank-item">
              <div class="rank-copy"><span :title="rankName(item, panel.keys)"><i>{{ index + 1 }}</i>{{ rankName(item, panel.keys) }}</span><strong>{{ formatInteger(rankValue(item)) }}</strong></div>
              <div class="rank-track"><span :style="{ width: rankWidth(item, panel.items) }"></span></div>
            </div>
          </div>
          <el-empty v-else :description="`${panel.title}暂无数据`" :image-size="54" />
        </article>

        <article class="dashboard-panel rank-panel rank-panel-tall">
          <header class="panel-header"><div><h3>请求耗时分布</h3><p>按 NGINX 总响应时间分组</p></div></header>
          <div v-if="overview.latency.length" class="latency-list">
            <div v-for="(item, index) in overview.latency.slice(0, 8)" :key="`latency-${index}`" class="latency-row">
              <span>{{ latencyLabel(item, index) }}</span>
              <strong>{{ latencyValue(item) }}</strong>
              <div class="latency-track"><span :style="{ width: rankWidth(item, overview.latency) }"></span></div>
            </div>
          </div>
          <el-empty v-else description="日志未提供延迟分布字段" :image-size="54" />
        </article>
      </section>

      <article v-if="showDomainsPanel" class="dashboard-panel table-panel">
        <header class="panel-header"><div><h3>域名分布</h3><p>全部可见日志库汇总</p></div><span class="panel-count">{{ overview.domains.length }} 个域名</span></header>
        <el-table class="data-table" :data="overview.domains" max-height="300">
          <el-table-column prop="domain" label="域名" min-width="220" show-overflow-tooltip />
          <el-table-column prop="storeName" label="日志库" min-width="150" show-overflow-tooltip />
          <el-table-column label="请求量" width="120" align="right"><template #default="{ row }">{{ formatInteger(row.requestTotal) }}</template></el-table-column>
          <el-table-column label="成功率" width="120" align="right"><template #default="{ row }">{{ formatPercent(row.successRate) }}</template></el-table-column>
          <el-table-column label="平均响应" width="120" align="right"><template #default="{ row }">{{ formatLatency(row.avgLatencyMs) }}</template></el-table-column>
          <el-table-column label="P95" width="120" align="right"><template #default="{ row }">{{ formatLatency(row.p95LatencyMs) }}</template></el-table-column>
        </el-table>
      </article>

      <article class="dashboard-panel table-panel">
        <header class="panel-header"><div><h3>最近异常请求</h3><p>按后端返回顺序展示，点击记录进入日志查询</p></div><span class="panel-count">{{ overview.recentErrors.length }} 条</span></header>
        <el-table v-if="overview.recentErrors.length" class="data-table clickable-table" :data="overview.recentErrors" max-height="360" @row-click="openErrorLog">
          <el-table-column label="时间" width="170"><template #default="{ row }">{{ formatDateTime(row.timestamp) }}</template></el-table-column>
          <el-table-column prop="domain" label="域名" min-width="150" show-overflow-tooltip />
          <el-table-column label="方法" width="80"><template #default="{ row }"><el-tag size="small" effect="plain" :type="methodTagType(row.requestMethod)">{{ row.requestMethod || '-' }}</el-tag></template></el-table-column>
          <el-table-column prop="requestPath" label="接口路径" min-width="220" show-overflow-tooltip />
          <el-table-column label="状态" width="80" align="center"><template #default="{ row }"><el-tag size="small" :type="statusTagType(row.status)">{{ row.status ?? '-' }}</el-tag></template></el-table-column>
          <el-table-column label="耗时" width="100" align="right"><template #default="{ row }">{{ formatLatency(row.latencyMs) }}</template></el-table-column>
          <el-table-column prop="clientIp" label="客户端 IP" width="140" />
          <el-table-column prop="upstreamAddr" label="Upstream" min-width="150" show-overflow-tooltip />
        </el-table>
        <el-empty v-else description="当前范围内没有异常请求" :image-size="68" />
      </article>

      <article v-if="overview.sources.length" class="dashboard-panel table-panel source-panel">
        <header class="panel-header"><div><h3>日志源状态</h3><p>单个日志源失败不会清空其他域名数据</p></div></header>
        <el-table class="data-table" :data="overview.sources" max-height="280">
          <el-table-column label="日志源" min-width="200"><template #default="{ row }">{{ row.domain || row.storeName || row.name || '-' }}</template></el-table-column>
          <el-table-column label="日志库" min-width="160"><template #default="{ row }">{{ row.storeName || row.storeCode || row.storeId || '-' }}</template></el-table-column>
          <el-table-column label="状态" width="110"><template #default="{ row }"><el-tag size="small" effect="light" :type="sourceTagType(row)">{{ sourceStatusText(row) }}</el-tag></template></el-table-column>
          <el-table-column label="说明" min-width="260"><template #default="{ row }">{{ row.message || row.warning || row.error || '-' }}</template></el-table-column>
        </el-table>
      </article>
    </main>
    </div>
    </section>

    <el-drawer v-model="bindingDrawerVisible" class="binding-drawer" title="域名绑定管理" size="760px" @open="loadBindings">
      <div class="binding-toolbar">
        <div><strong>域名与 NGINX 日志库</strong><span>一个业务域名对应一个独立日志库</span></div>
        <el-button type="primary" :icon="Plus" @click="openBindingDialog()">新增绑定</el-button>
      </div>
      <el-table v-loading="bindingLoading" class="data-table" :data="bindings" height="calc(100vh - 220px)">
        <el-table-column prop="domain" label="域名" min-width="220" show-overflow-tooltip />
        <el-table-column label="日志库" min-width="200" show-overflow-tooltip><template #default="{ row }">{{ row.storeName || row.storeCode || row.storeId || '-' }}</template></el-table-column>
        <el-table-column label="操作" width="112" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" :icon="Edit" aria-label="编辑绑定" @click="openBindingDialog(row)" />
            <el-button link type="danger" :icon="Delete" aria-label="删除绑定" @click="removeBinding(row)" />
          </template>
        </el-table-column>
      </el-table>
      <el-pagination
        v-model:current-page="bindingPagination.page"
        v-model:page-size="bindingPagination.pageSize"
        class="binding-pagination"
        background
        layout="total, prev, pager, next"
        :total="bindingPagination.total"
        @current-change="loadBindings"
      />
    </el-drawer>

    <el-dialog v-model="bindingDialogVisible" :title="bindingForm.id ? '编辑域名绑定' : '新增域名绑定'" width="560px" append-to-body destroy-on-close>
      <el-form ref="bindingFormRef" :model="bindingForm" :rules="bindingRules" label-width="86px">
        <el-form-item label="域名" prop="domain"><el-input v-model.trim="bindingForm.domain" placeholder="www.example.com" @blur="normalizeBindingDomain" /></el-form-item>
        <el-form-item label="日志库" prop="storeId">
          <el-select
            v-model="bindingForm.storeId"
            filterable
            remote
            reserve-keyword
            placeholder="按日志库或采集任务名称搜索"
            :remote-method="searchBindingStores"
            :loading="bindingStoreLoading"
          >
            <el-option v-for="item in bindingStoreOptions" :key="item.id" :label="storeLabel(item)" :value="item.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="bindingDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="bindingSubmitting" @click="submitBinding">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, defineProps, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import * as echarts from 'echarts'
import chinaGeoJSON from '@/assets/china.geo.json'
import { CircleCheck, Clock, DataLine, Delete, Edit, Odometer, Plus, Refresh, Setting, Timer, TrendCharts, User, WarningFilled } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import logtailApi from '@/api/logtail'

defineProps({
  embedded: {
    type: Boolean,
    default: false
  }
})

const router = useRouter()
const optionsLoading = ref(false)
const overviewLoading = ref(false)
const visitorLoading = ref(false)
const overviewError = ref('')
const unauthorized = ref(false)
const hasLoaded = ref(false)
const autoRefresh = ref(false)
const domains = ref([])
const selectedBindingId = ref('')
const trendChartRef = ref(null)
const statusChartRef = ref(null)
const ipRegionChartRef = ref(null)
let trendChart = null
let statusChart = null
let ipRegionChart = null
let bindingStoreSearchSequence = 0
let overviewController = null
let visitorController = null
let refreshTimer = null
let resizeObserver = null

const ipRegionTrafficLevels = [
  { min: 1, max: 499, label: '1-499', lightness: 80, saturation: 58 },
  { min: 500, max: 999, label: '500-999', lightness: 69, saturation: 62 },
  { min: 1000, max: 4999, label: '1千-4999', lightness: 58, saturation: 66 },
  { min: 5000, max: 9999, label: '5千-9999', lightness: 48, saturation: 70 },
  { min: 10000, max: Infinity, label: '1万+', lightness: 38, saturation: 74 }
]
const ipRegionHueByName = new Map(
  (chinaGeoJSON.features || [])
    .map((feature, index) => [feature?.properties?.name, Math.round((210 + index * 137.508) % 360)])
    .filter(([name]) => name)
)

const filters = reactive(createDefaultFilters())
const overview = reactive(createEmptyOverview())

const windowOptions = [
  { label: '最近 3 小时', value: '3h' },
  { label: '最近 6 小时', value: '6h' },
  { label: '最近 12 小时', value: '12h' },
  { label: '最近 24 小时', value: '24h' },
  { label: '自定义时间', value: 'custom' }
]

const metricCards = computed(() => {
  const cards = overview.cards || {}
  const comparisons = overview.comparisons || {}
  return [
    { key: 'requestTotal', label: '请求总量', value: formatInteger(cards.requestTotal), hint: '当前筛选范围', comparison: comparisons.requestTotal, icon: DataLine, tone: 'blue' },
    { key: 'uniqueVisitors', label: '独立访客 UV', value: visitorLoading.value ? '加载中...' : formatInteger(cards.uniqueVisitors), hint: cards.visitorStrategy || '按身份策略去重估算', comparison: comparisons.uniqueVisitors, icon: User, tone: 'teal' },
    { key: 'currentQps', label: '当前 QPS', value: formatDecimal(cards.currentQps), hint: '最近聚合点', icon: Odometer, tone: 'cyan' },
    { key: 'maxQps', label: '最大 QPS', value: formatDecimal(cards.maxQps), hint: '范围内峰值', icon: TrendCharts, tone: 'indigo' },
    { key: 'successRate', label: '2xx 成功率', value: formatPercent(cards.successRate), hint: 'HTTP 2xx / 总请求', comparison: comparisons.successRate, icon: CircleCheck, tone: 'green' },
    { key: 'http4xx', label: '4xx 数量', value: formatInteger(cards.http4xx), hint: `错误率 ${formatPercent(cards.errorRate)}`, comparison: comparisons.http4xx, icon: WarningFilled, tone: 'amber' },
    { key: 'http5xx', label: '5xx 数量', value: formatInteger(cards.http5xx), hint: '服务端异常', comparison: comparisons.http5xx, icon: WarningFilled, tone: 'red' },
    { key: 'avgLatencyMs', label: '平均响应时间', value: formatLatency(cards.avgLatencyMs), hint: '请求平均耗时', comparison: comparisons.avgLatencyMs, icon: Clock, tone: 'cyan' },
    { key: 'p95LatencyMs', label: 'P95 响应时间', value: formatLatency(cards.p95LatencyMs), hint: '95% 请求低于此值', comparison: comparisons.p95LatencyMs, icon: Timer, tone: 'violet' }
  ]
})

const primaryRankPanels = computed(() => [
  { key: 'methods', title: 'HTTP 方法', description: '请求方法分布', items: overview.methods, keys: ['method', 'name'] },
  { key: 'upstreams', title: 'Upstream Top', description: '后端节点请求量', items: overview.upstreams, keys: ['upstreamAddr', 'upstream', 'name'] }
])

const topApiTableHeight = computed(() => overview.topApis.length > 10 ? 360 : undefined)

const secondaryRankPanels = computed(() => [
  { key: 'clients', title: '客户端 IP Top', description: '访问量最高的客户端', items: overview.clients, keys: ['clientIp', 'ip', 'name'] },
  { key: 'referers', title: 'Referer Top', description: '主要访问来源', items: overview.referers, keys: ['referer', 'name'] }
])

const warningMessages = computed(() => {
  const values = Array.isArray(overview.warnings) ? overview.warnings : []
  return values.map(item => typeof item === 'string' ? item : item?.message || JSON.stringify(item)).filter(Boolean)
})
const unavailableDescription = computed(() => overviewError.value || warningMessages.value[0] || '日志服务当前不可用，请稍后重试')

const hasDashboardData = computed(() => {
  const cardValues = Object.values(overview.cards || {})
  return cardValues.some(value => value !== null && value !== undefined) || [
    overview.trend,
    overview.domains,
    overview.statuses,
    overview.topApis,
    overview.upstreams,
    overview.methods,
    overview.clients,
    overview.referers,
    overview.latency,
    overview.ipRegions,
    overview.recentErrors,
    overview.sources
  ].some(items => Array.isArray(items) && items.length)
})

const showDomainsPanel = computed(() => !filters.domain && overview.domains.length > 0)
const scopeDescription = computed(() => {
  const scope = overview.domain || filters.domain || '全部业务域名'
  return `${scope} · ${windowDescription.value}`
})
const windowDescription = computed(() => {
  const serverWindow = overview.window || {}
  if (serverWindow.start && serverWindow.end) return `${formatDateTime(serverWindow.start)} 至 ${formatDateTime(serverWindow.end)}`
  return windowOptions.find(item => item.value === filters.window)?.label || '最近 30 分钟'
})
const lastUpdatedText = computed(() => overview.generatedAt ? `更新于 ${formatTimeOnly(overview.generatedAt)}` : '尚未刷新')

const bindingDrawerVisible = ref(false)
const bindingDialogVisible = ref(false)
const bindingLoading = ref(false)
const bindingSubmitting = ref(false)
const bindingStoreLoading = ref(false)
const bindingFormRef = ref(null)
const bindings = ref([])
const bindingPagination = reactive({ page: 1, pageSize: 20, total: 0 })
const managementOptions = reactive({ stores: [] })
const bindingForm = reactive(createBindingForm())
const bindingRules = {
  storeId: [{ required: true, message: '请选择 NGINX 日志库', trigger: 'change' }],
  domain: [
    { required: true, message: '请输入域名', trigger: 'blur' },
    { pattern: /^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9][a-z0-9-]{0,62}\.?$/i, message: '请输入有效域名', trigger: 'blur' }
  ]
}
const bindingStoreOptions = computed(() => managementOptions.stores)

function createDefaultFilters() {
  return { domain: '', storeId: '', window: '30m', customRange: [] }
}

function createEmptyOverview() {
  return {
    domain: '',
    storeIds: [],
    window: {},
    filters: {},
    cards: {},
    comparisons: {},
    ipRegions: [],
    trend: [],
    domains: [],
    statuses: [],
    topApis: [],
    upstreams: [],
    methods: [],
    clients: [],
    referers: [],
    latency: [],
    recentErrors: [],
    sources: [],
    dataStatus: '',
    partial: false,
    warnings: [],
    generatedAt: ''
  }
}

function createBindingForm(row = {}) {
  return {
    id: row.id,
    storeId: row.storeId || '',
    domain: row.domain || ''
  }
}

function unwrapResponse(response) {
  const body = response?.data ?? response
  if (body && typeof body === 'object' && Object.prototype.hasOwnProperty.call(body, 'code')) {
    // 平台接口存在 code: 0 和 code: 200 两种成功约定，均按成功处理。
    const code = Number(body.code)
    if (![0, 200].includes(code)) {
      const error = new Error(body.message || '请求失败')
      error.businessCode = code
      throw error
    }
    return body.data
  }
  return body?.data ?? body
}

function normalizeOverview(data = {}) {
  const empty = createEmptyOverview()
  const result = { ...empty, ...data, cards: data.cards || {}, comparisons: data.comparisons || {}, window: data.window || {}, filters: data.filters || {} }
  Object.keys(empty).forEach(key => {
    if (Array.isArray(empty[key])) result[key] = Array.isArray(data[key]) ? data[key] : []
  })
  return result
}

async function loadOptions() {
  optionsLoading.value = true
  try {
    const data = unwrapResponse(await logtailApi.getNginxDashboardOptions()) || {}
    domains.value = Array.isArray(data.domains) ? data.domains : []
    const selected = domains.value.find(item => String(item.id) === String(selectedBindingId.value))
      || domains.value.find(item => item.domain === filters.domain && Number(item.storeId) === Number(filters.storeId))
      || domains.value[0]
    applySelectedBinding(selected)
    return true
  } catch (error) {
    if (isForbidden(error)) unauthorized.value = true
    else ElMessage.error(error.message || '加载筛选选项失败')
    return false
  } finally {
    optionsLoading.value = false
  }
}

function buildOverviewParams() {
  const params = {
    storeId: filters.storeId || undefined,
    domain: filters.domain || undefined
  }
  if (filters.window === 'custom' && filters.customRange?.length === 2) {
    params.start = new Date(filters.customRange[0]).toISOString()
    params.end = new Date(filters.customRange[1]).toISOString()
  } else {
    params.window = filters.window
  }
  return params
}

async function loadOverview() {
  if (filters.window === 'custom' && filters.customRange?.length !== 2) {
    overviewController?.abort()
    visitorController?.abort()
    overviewLoading.value = false
    visitorLoading.value = false
    overviewError.value = ''
    return
  }
  if (!filters.domain || !filters.storeId) {
    overviewController?.abort()
    visitorController?.abort()
    overviewLoading.value = false
    visitorLoading.value = false
    overviewError.value = ''
    Object.assign(overview, createEmptyOverview(), { dataStatus: 'unconfigured' })
    hasLoaded.value = true
    return
  }
  overviewController?.abort()
  visitorController?.abort()
  const controller = new AbortController()
  overviewController = controller
  overviewLoading.value = true
  overviewError.value = ''
  unauthorized.value = false
  try {
    const response = await logtailApi.getNginxDashboardOverview(buildOverviewParams(), {
      signal: controller.signal,
      skipErrorMessage: true
    })
    const data = normalizeOverview(unwrapResponse(response) || {})
    Object.assign(overview, data)
    hasLoaded.value = true
    loadVisitors(data.window).catch(() => {})
  } catch (error) {
    if (isCanceled(error)) return
    hasLoaded.value = true
    if (isForbidden(error)) unauthorized.value = true
    else overviewError.value = getErrorMessage(error)
  } finally {
    if (overviewController === controller) overviewLoading.value = false
  }
}

async function loadVisitors(windowOverride = {}) {
  visitorController?.abort()
  const controller = new AbortController()
  visitorController = controller
  visitorLoading.value = true
  try {
    const params = buildOverviewParams()
    if (windowOverride?.start && windowOverride?.end) {
      delete params.window
      params.start = windowOverride.start
      params.end = windowOverride.end
    }
    const response = await logtailApi.getNginxDashboardVisitors(params, {
      signal: controller.signal,
      skipErrorMessage: true
    })
    const data = unwrapResponse(response) || {}
    if (visitorController !== controller) return
    overview.cards = { ...(overview.cards || {}), uniqueVisitors: Number(data.uniqueVisitors) || 0, visitorStrategy: data.visitorStrategy || overview.cards?.visitorStrategy }
    if (Array.isArray(data.warnings) && data.warnings.length) overview.warnings = [...(overview.warnings || []), ...data.warnings]
    visitorLoading.value = false

    // Keep the fast Top-1000 estimate visible, then reconcile it with a full
    // paged aggregation without blocking the rest of the dashboard.
    const exactParams = { ...params, exact: true }
    const exactResponse = await logtailApi.getNginxDashboardVisitors(exactParams, {
      signal: controller.signal,
      skipErrorMessage: true
    })
    const exactData = unwrapResponse(exactResponse) || {}
    if (visitorController !== controller) return
    overview.cards = { ...(overview.cards || {}), uniqueVisitors: Number(exactData.uniqueVisitors) || 0, visitorStrategy: exactData.visitorStrategy || 'IP + User-Agent（完整统计）' }
    if (Array.isArray(exactData.warnings) && exactData.warnings.length) overview.warnings = [...(overview.warnings || []), ...exactData.warnings]
  } catch (error) {
    if (!isCanceled(error) && visitorController === controller) {
      overview.warnings = [...(overview.warnings || []), getErrorMessage(error)]
    }
  } finally {
    if (visitorController === controller) visitorLoading.value = false
  }
}

function applySelectedBinding(selected) {
  selectedBindingId.value = selected?.id || ''
  filters.domain = selected?.domain || ''
  filters.storeId = selected?.storeId || ''
}

function handleDomainChange(bindingId) {
  const selected = domains.value.find(item => String(item.id) === String(bindingId))
  applySelectedBinding(selected)
  loadOverview()
}

function selectWindow(value) {
  filters.window = value
  if (value !== 'custom') {
    filters.customRange = []
    loadOverview()
  }
}

function handleCustomRangeChange(value) {
  if (value?.length === 2) loadOverview()
}

function disableFutureDate(date) {
  return date.getTime() > Date.now()
}

function getErrorMessage(error) {
  const candidates = [error?.response?.data?.message, error?.message]
  const message = candidates.find(value => value && !/^(成功|success|ok)$/i.test(String(value).trim()))
  return message || '请检查 NGINX 大盘接口'
}

function toggleAutoRefresh(enabled) {
  clearInterval(refreshTimer)
  refreshTimer = null
  if (enabled) refreshTimer = window.setInterval(loadOverview, 30000)
}

function domainLabel(item) {
  return item.displayName ? `${item.displayName} · ${item.domain}` : item.domain
}

function storeLabel(item) {
  return item.name ? `${item.name}${item.code ? ` · ${item.code}` : ''}` : item.code || `日志库 ${item.id}`
}

function hasValue(value) {
  return value !== null && value !== undefined && value !== '' && Number.isFinite(Number(value))
}

function formatInteger(value) {
  if (!hasValue(value)) return '--'
  return new Intl.NumberFormat('zh-CN', { maximumFractionDigits: 0 }).format(Number(value))
}

function formatDecimal(value) {
  if (!hasValue(value)) return '--'
  return Number(value).toFixed(2)
}

function formatPercent(value) {
  if (!hasValue(value)) return '--'
  return `${Number(value).toFixed(2)}%`
}

function formatChange(value) {
  if (!hasValue(value)) return ''
  const number = Number(value)
  return `${number > 0 ? '+' : ''}${number.toFixed(2)}%`
}

function changeTone(value) {
  if (!hasValue(value)) return 'is-unavailable'
  return Number(value) > 0 ? 'is-up' : Number(value) < 0 ? 'is-down' : 'is-flat'
}

function formatLatency(value) {
  if (!hasValue(value)) return '--'
  const number = Number(value)
  if (number >= 1000) return `${(number / 1000).toFixed(number >= 10000 ? 1 : 2)} s`
  return `${number.toFixed(number >= 100 ? 0 : 1)} ms`
}

function formatBytes(value) {
  if (!hasValue(value)) return '--'
  const number = Number(value)
  if (number === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const index = Math.min(Math.floor(Math.log(Math.abs(number)) / Math.log(1024)), units.length - 1)
  const amount = number / (1024 ** index)
  return `${amount >= 100 ? amount.toFixed(0) : amount.toFixed(2)} ${units[index]}`
}

function formatDateTime(value) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return date.toLocaleString('zh-CN', { hour12: false }).replace(/\//g, '-')
}

function formatTimeOnly(value) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return date.toLocaleTimeString('zh-CN', { hour12: false })
}

function rankName(item, keys) {
  for (const key of keys) if (item?.[key]) return String(item[key])
  return '未知'
}

function rankValue(item) {
  return item?.requestTotal ?? item?.count ?? item?.value
}

function rankWidth(item, items) {
  const max = Math.max(...items.map(entry => Number(rankValue(entry)) || 0), 0)
  if (!max) return '0%'
  return `${Math.max(4, Math.min(100, (Number(rankValue(item)) || 0) / max * 100))}%`
}

function latencyLabel(item, index) {
  return item?.bucket || item?.range || item?.latency || item?.name || item?.label || `区间 ${index + 1}`
}

function latencyValue(item) {
  const value = item?.requestTotal ?? item?.request_total ?? item?.count ?? item?.requests ?? item?.value
  return formatInteger(value)
}

function methodTagType(method) {
  const value = String(method || '').toUpperCase()
  if (value === 'GET') return 'success'
  if (value === 'POST') return 'primary'
  if (value === 'DELETE') return 'danger'
  if (value === 'PUT' || value === 'PATCH') return 'warning'
  return 'info'
}

function statusTagType(status) {
  const value = Number(status)
  if (value >= 500) return 'danger'
  if (value >= 400) return 'warning'
  if (value >= 300) return 'primary'
  if (value >= 200) return 'success'
  return 'info'
}

function statusColor(status) {
  const value = Number(status)
  if (value >= 500) return '#ef6b73'
  if (value >= 400) return '#f3a84f'
  if (value >= 300) return '#6f86f4'
  if (value >= 200) return '#35bea0'
  return '#a5afbd'
}

function sourceStatusText(row) {
  const status = String(row?.status || row?.dataStatus || '').toLowerCase()
  if (['ready', 'success', 'ok', 'healthy', '1'].includes(status)) return '正常'
  if (['partial', 'warning', 'degraded'].includes(status)) return '部分可用'
  if (['failed', 'error', 'unavailable', 'offline', '2'].includes(status)) return '异常'
  return row?.status || '未知'
}

function sourceTagType(row) {
  const text = sourceStatusText(row)
  if (text === '正常') return 'success'
  if (text === '部分可用') return 'warning'
  if (text === '异常') return 'danger'
  return 'info'
}

function openErrorLog(row) {
  const query = {
    storeId: filters.storeId || overview.storeIds?.[0] || undefined,
    start: overview.window?.start || undefined,
    end: overview.window?.end || undefined,
    requestPath: row.requestPath || undefined,
    status: row.status || undefined,
    clientIp: row.clientIp || undefined
  }
  router.push({ path: '/logs/query', query: Object.fromEntries(Object.entries(query).filter(([, value]) => value !== undefined && value !== '')) })
}

function isCanceled(error) {
  return error?.code === 'ERR_CANCELED' || error?.name === 'CanceledError' || error?.message === 'canceled'
}

function isForbidden(error) {
  return Number(error?.businessCode) === 403 || Number(error?.response?.status) === 403 || Number(error?.response?.data?.code) === 403
}

async function renderCharts() {
  await nextTick()
  renderTrendChart()
  renderStatusChart()
  renderIPRegionChart()
  observeChartContainers()
}

function ensureChart(instance, element) {
  if (instance && !instance.isDisposed() && instance.getDom() === element) return instance
  if (instance && !instance.isDisposed()) instance.dispose()
  return element ? echarts.init(element, null, { renderer: 'canvas' }) : null
}

function renderTrendChart() {
  if (!overview.trend.length || !trendChartRef.value) {
    trendChart?.dispose()
    trendChart = null
    return
  }
  trendChart = ensureChart(trendChart, trendChartRef.value)
  const rows = overview.trend
  const seriesConfig = [
    ['请求总量', 'requestTotal', '#4d85f3'],
    ['4xx', 'http4xx', '#f3a84f'],
    ['5xx', 'http5xx', '#ef6b73']
  ]
  trendChart.setOption({
    animationDuration: 320,
    color: seriesConfig.map(item => item[2]),
    tooltip: { trigger: 'axis', backgroundColor: 'rgba(255,255,255,.96)', borderColor: '#e4e9f0', textStyle: { color: '#273142' } },
    legend: { top: 2, left: 0, itemWidth: 14, itemHeight: 6, textStyle: { color: '#7b8798', fontSize: 11 } },
    grid: { left: 48, right: 18, top: 42, bottom: 38 },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: rows.map(item => formatTrendTime(item.timestamp)),
      axisLine: { lineStyle: { color: '#dfe5ec' } },
      axisTick: { show: false },
      axisLabel: { color: '#8a96a6', hideOverlap: true }
    },
    yAxis: {
      type: 'value',
      minInterval: 1,
      axisLabel: { color: '#8a96a6' },
      splitLine: { lineStyle: { color: '#edf1f5', type: 'dashed' } }
    },
    series: seriesConfig.map(([name, key, color], index) => ({
      name,
      type: 'line',
      smooth: 0.25,
      showSymbol: false,
      symbolSize: 5,
      lineStyle: { width: index === 0 ? 2.5 : 1.8, color },
      areaStyle: index === 0 ? { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: 'rgba(77,133,243,.18)' }, { offset: 1, color: 'rgba(77,133,243,0)' }]) } : undefined,
      data: rows.map(item => item[key] ?? null)
    }))
  }, true)
}

function renderStatusChart() {
  if (!overview.statuses.length || !statusChartRef.value) {
    statusChart?.dispose()
    statusChart = null
    return
  }
  statusChart = ensureChart(statusChart, statusChartRef.value)
  const data = overview.statuses.map(item => ({
    name: String(item.status ?? '未知'),
    value: Number(item.requestTotal) || 0,
    itemStyle: { color: statusColor(item.status) }
  }))
  const total = data.reduce((sum, item) => sum + item.value, 0)
  statusChart.setOption({
    animationDuration: 320,
    tooltip: { trigger: 'item', formatter: '{b}<br/>{c} 次 ({d}%)', backgroundColor: 'rgba(255,255,255,.96)', borderColor: '#e4e9f0', textStyle: { color: '#273142' } },
    legend: { type: 'scroll', bottom: 0, left: 'center', itemWidth: 10, itemHeight: 10, textStyle: { color: '#7b8798', fontSize: 11 } },
    title: { text: formatInteger(total), subtext: '总请求', left: 'center', top: '38%', textStyle: { color: '#172033', fontSize: 27, fontWeight: 700 }, subtextStyle: { color: '#8d98a8', fontSize: 12 } },
    series: [{ type: 'pie', radius: ['50%', '70%'], center: ['50%', '46%'], minAngle: 2, avoidLabelOverlap: true, itemStyle: { borderColor: '#fff', borderWidth: 2 }, label: { color: '#566273', fontSize: 11, formatter: '{b}  {d}%' }, labelLine: { length: 10, length2: 6 }, data }]
  }, true)
}

function renderIPRegionChart() {
  if (!overview.ipRegions.length || !ipRegionChartRef.value) {
    ipRegionChart?.dispose()
    ipRegionChart = null
    return
  }
  echarts.registerMap('china', chinaGeoJSON)
  ipRegionChart = ensureChart(ipRegionChart, ipRegionChartRef.value)
  const rows = overview.ipRegions.map(item => ({
    name: normalizeProvinceName(item.province),
    value: Number(item.requestTotal) || 0,
    city: item.city || '',
    itemStyle: {
      areaColor: ipRegionAreaColor(normalizeProvinceName(item.province), Number(item.requestTotal) || 0)
    }
  }))
  ipRegionChart.setOption({
    animationDuration: 320,
    tooltip: {
      trigger: 'item',
      formatter: params => `${params.name}<br/>请求数：${formatInteger(params.value || 0)}${params.data?.city ? `<br/>城市：${params.data.city}` : ''}`,
      backgroundColor: 'rgba(255,255,255,.96)',
      borderColor: '#e4e9f0',
      textStyle: { color: '#273142' }
    },
    graphic: buildIPRegionTrafficLegend(),
    series: [{
      type: 'map',
      map: 'china',
      roam: false,
      selectedMode: false,
      layoutCenter: ['48%', '60%'],
      layoutSize: '135%',
      aspectScale: 0.65,
      label: { show: false },
      itemStyle: { areaColor: '#f1f4f8', borderColor: '#d7dee8', borderWidth: 0.7 },
      emphasis: {
        label: { show: false },
        itemStyle: { borderColor: '#42658a', borderWidth: 1.2, shadowBlur: 8, shadowColor: 'rgba(37, 64, 92, .22)' }
      },
      data: rows
    }]
  }, true)
}

function ipRegionTrafficLevel(value) {
  const count = Number(value) || 0
  return ipRegionTrafficLevels.find(level => count >= level.min && count <= level.max) || ipRegionTrafficLevels[0]
}

function ipRegionAreaColor(province, value) {
  const level = ipRegionTrafficLevel(value)
  const hue = ipRegionHueByName.get(province) ?? 210
  return `hsl(${hue}, ${level.saturation}%, ${level.lightness}%)`
}

function buildIPRegionTrafficLegend() {
  const children = [{
    type: 'text',
    style: { x: 0, y: 0, text: '颜色深浅：请求数', fill: '#667386', font: '11px sans-serif' }
  }]
  let offset = 0
  ipRegionTrafficLevels.forEach(level => {
    children.push({
      type: 'rect',
      shape: { x: offset, y: 18, width: 12, height: 10, r: 2 },
      style: { fill: `hsl(210, ${level.saturation}%, ${level.lightness}%)` }
    })
    children.push({
      type: 'text',
      style: { x: offset + 17, y: 18, text: level.label, fill: '#7b8798', font: '10px sans-serif' }
    })
    offset += level.label.length > 7 ? 88 : 72
  })
  return [{ type: 'group', left: 10, bottom: 5, silent: true, children }]
}

function normalizeProvinceName(value) {
  const name = String(value || '').trim()
  const aliases = {
    内蒙古: '内蒙古自治区',
    广西: '广西壮族自治区',
    西藏: '西藏自治区',
    宁夏: '宁夏回族自治区',
    新疆: '新疆维吾尔自治区',
    香港: '香港特别行政区',
    澳门: '澳门特别行政区',
    台湾: '台湾省'
  }
  return aliases[name] || name
}

function formatTrendTime(value) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  const windowMs = overview.window?.start && overview.window?.end ? new Date(overview.window.end) - new Date(overview.window.start) : 0
  return windowMs > 24 * 60 * 60 * 1000
    ? `${date.getMonth() + 1}-${String(date.getDate()).padStart(2, '0')} ${String(date.getHours()).padStart(2, '0')}:00`
    : date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })
}

function observeChartContainers() {
  resizeObserver?.disconnect()
  resizeObserver = new ResizeObserver(() => {
    trendChart?.resize()
    statusChart?.resize()
    ipRegionChart?.resize()
  })
  if (trendChartRef.value) resizeObserver.observe(trendChartRef.value)
  if (statusChartRef.value) resizeObserver.observe(statusChartRef.value)
  if (ipRegionChartRef.value) resizeObserver.observe(ipRegionChartRef.value)
}

async function openBindingDrawer() {
  bindingDrawerVisible.value = true
  try {
    const data = unwrapResponse(await logtailApi.getNginxDashboardOptions()) || {}
    managementOptions.stores = Array.isArray(data.stores) ? data.stores : []
  } catch (error) {
    ElMessage.error(error.message || '加载绑定选项失败')
  }
}

async function searchBindingStores(keyword) {
  const sequence = ++bindingStoreSearchSequence
  bindingStoreLoading.value = true
  try {
    const data = unwrapResponse(await logtailApi.getNginxDashboardOptions({ keyword: String(keyword || '').trim() })) || {}
    if (sequence === bindingStoreSearchSequence) {
      managementOptions.stores = Array.isArray(data.stores) ? data.stores : []
    }
  } catch (error) {
    if (sequence === bindingStoreSearchSequence) ElMessage.error(error.message || '搜索日志库失败')
  } finally {
    if (sequence === bindingStoreSearchSequence) bindingStoreLoading.value = false
  }
}

async function loadBindings() {
  bindingLoading.value = true
  try {
    const data = unwrapResponse(await logtailApi.getNginxDashboardBindings({ page: bindingPagination.page, pageSize: bindingPagination.pageSize })) || {}
    bindings.value = Array.isArray(data.list) ? data.list : []
    bindingPagination.total = Number(data.total) || 0
  } catch (error) {
    ElMessage.error(error.message || '加载域名绑定失败')
  } finally {
    bindingLoading.value = false
  }
}

function openBindingDialog(row) {
  Object.assign(bindingForm, createBindingForm(row))
  bindingDialogVisible.value = true
  nextTick(() => bindingFormRef.value?.clearValidate())
}

function normalizeBindingDomain() {
  bindingForm.domain = String(bindingForm.domain || '').trim().toLowerCase()
}

async function submitBinding() {
  const valid = await bindingFormRef.value?.validate().catch(() => false)
  if (!valid) return
  normalizeBindingDomain()
  bindingSubmitting.value = true
  try {
    const payload = {
      storeId: bindingForm.storeId,
      domain: bindingForm.domain
    }
    if (bindingForm.id) unwrapResponse(await logtailApi.updateNginxDashboardBinding(bindingForm.id, payload))
    else unwrapResponse(await logtailApi.createNginxDashboardBinding(payload))
    ElMessage.success(bindingForm.id ? '域名绑定已更新' : '域名绑定已创建')
    bindingDialogVisible.value = false
    const [, optionsLoaded] = await Promise.all([loadBindings(), loadOptions()])
    if (optionsLoaded) await loadOverview()
  } catch (error) {
    ElMessage.error(error.message || '保存域名绑定失败')
  } finally {
    bindingSubmitting.value = false
  }
}

async function removeBinding(row) {
  try {
    await ElMessageBox.confirm(`确定删除域名“${row.domain}”的日志库绑定吗？`, '删除确认', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
    unwrapResponse(await logtailApi.deleteNginxDashboardBinding(row.id))
    ElMessage.success('域名绑定已删除')
    const [, optionsLoaded] = await Promise.all([loadBindings(), loadOptions()])
    if (optionsLoaded) await loadOverview()
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    ElMessage.error(error.message || '删除域名绑定失败')
  }
}

watch(() => overview.generatedAt, renderCharts)

onMounted(async () => {
  const optionsLoaded = await loadOptions()
  if (optionsLoaded) await loadOverview()
})

onBeforeUnmount(() => {
  overviewController?.abort()
  visitorController?.abort()
  clearInterval(refreshTimer)
  resizeObserver?.disconnect()
  trendChart?.dispose()
  statusChart?.dispose()
  ipRegionChart?.dispose()
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
  --surface: rgba(255, 255, 255, 0.88);
  --border: var(--log-border);
  --text: var(--log-text-primary);
  --text-secondary: var(--log-text-secondary);
  --muted: var(--log-text-muted);
  --accent: var(--log-brand);
  height: calc(100vh - var(--log-page-height-offset));
  min-height: calc(100vh - var(--log-page-height-offset));
  max-height: calc(100vh - var(--log-page-height-offset));
  box-sizing: border-box;
  overflow: hidden;
  padding: 6px;
  color: var(--log-text-secondary);
  background: transparent;
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.log-page.is-embedded {
  height: auto;
  min-height: 0;
  max-height: none;
  padding: 0;
  overflow: visible;
}

.log-page.is-embedded .page-card,
.log-page.is-embedded-card {
  height: auto;
  min-height: 0;
  max-height: none;
  padding: 0;
  overflow: visible;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.log-page.is-embedded .page-card::before {
  display: none;
}

.log-page.is-embedded .page-header.dashboard-header {
  margin: 0 0 8px;
  border-radius: 12px;
}

.log-page.is-embedded .report-content {
  flex: none;
  overflow: visible;
  padding-bottom: 0;
}

.page-card {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: calc(100vh - var(--log-page-height-offset) - 12px);
  max-height: calc(100vh - var(--log-page-height-offset) - 12px);
  box-sizing: border-box;
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

.page-header.dashboard-header {
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

.report-content {
  min-height: 0;
  flex: 1;
  padding: 0 2px 4px;
  overflow-x: hidden;
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: rgba(148, 163, 184, 0.65) transparent;
}

.report-content::-webkit-scrollbar { width: 6px; }
.report-content::-webkit-scrollbar-thumb { border-radius: 999px; background: rgba(148, 163, 184, 0.55); }

.header-actions,
.metric-label,
.panel-header,
.rank-copy,
.binding-toolbar,
.option-row {
  display: flex;
  align-items: center;
}

.page-header h2 { margin: 0; color: var(--log-text-primary); font-size: 22px; line-height: 1.15; }
.scope-text { color: var(--muted); font-size: 12px; white-space: nowrap; }
.header-actions { justify-content: flex-end; gap: 9px; flex-wrap: wrap; }
.updated-at { color: var(--muted); font-size: 12px; white-space: nowrap; }
.health-state { padding: 3px 8px; border-radius: 5px; font-size: 11px; font-weight: 600; }
.health-state.is-ready { color: #16876f; background: #e6f8f3; }
.health-state.is-partial { color: #ad6a13; background: #fff5df; }

.filter-panel,
.dashboard-panel,
.metric-card,
.dashboard-skeleton,
.page-state {
  border: 0;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.82);
  box-shadow: 0 8px 24px rgba(17, 24, 39, 0.05);
}

.filter-panel { position: sticky; top: 0; z-index: 6; margin-bottom: 10px; padding: 9px 10px 8px; background: rgba(255, 255, 255, 0.95); }
.filter-grid { display: grid; grid-template-columns: minmax(240px, 360px) minmax(0, 1fr); gap: 8px 12px; align-items: center; }
.filter-grid :deep(.el-form-item) { min-width: 0; margin: 0; }
.filter-grid :deep(.el-form-item__label) { height: auto; padding: 0 7px 0 0; color: var(--log-text-secondary); font-size: 12px; font-weight: 600; line-height: 30px; }
.filter-grid :deep(.el-select) { width: 100%; }
.time-filter { display: flex; align-items: center; min-width: 0; gap: 8px; }
.filter-label { flex: 0 0 auto; color: var(--log-text-secondary); font-size: 12px; font-weight: 600; }
.time-filter-content { display: flex; align-items: center; min-width: 0; gap: 8px; flex-wrap: nowrap; }
.time-presets { display: flex; flex-wrap: nowrap; gap: 5px; }
.time-preset { height: 30px; margin: 0; padding: 0 10px; border-radius: 8px !important; font-size: 12px; white-space: nowrap; }
.time-preset.is-active { color: #fff; }
.custom-range-picker { flex: 0 1 280px; min-width: 230px; }
.option-row { width: 100%; justify-content: space-between; gap: 12px; }
.option-row small { overflow: hidden; color: var(--muted); text-overflow: ellipsis; white-space: nowrap; }

.status-alert { margin-bottom: 10px; overflow: hidden; border-radius: 10px; }
.status-alert details { font-size: 12px; }
.status-alert summary { cursor: pointer; }
.status-alert p { margin: 5px 0 0; }
.page-state { min-height: 420px; display: grid; place-items: center; }
.dashboard-skeleton { min-height: 560px; padding: 24px; }
.dashboard-content { transition: opacity 0.18s ease; }
.dashboard-content.is-refreshing { opacity: 0.72; pointer-events: none; }

.metric-grid { display: grid; grid-template-columns: repeat(9, minmax(0, 1fr)); gap: 9px; margin-bottom: 10px; }
.metric-card { position: relative; min-width: 0; min-height: 94px; padding: 9px 12px; overflow: hidden; border-radius: 13px; }
.metric-card::before { position: absolute; top: 0; right: 0; left: 0; height: 3px; background: var(--metric-color); content: ""; opacity: 0.78; }
.metric-label { justify-content: space-between; gap: 8px; color: var(--log-text-muted); font-size: 11px; }
.metric-label .el-icon { width: 23px; height: 23px; flex: 0 0 auto; border-radius: 7px; color: var(--metric-color); background: var(--metric-bg); font-size: 13px; }
.metric-card strong { display: block; margin-top: 4px; overflow: hidden; color: var(--metric-color); font-size: 20px; line-height: 1.15; text-overflow: ellipsis; white-space: nowrap; }
.metric-card small { display: block; margin-top: 4px; overflow: hidden; color: var(--log-text-muted); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.metric-comparison { display: flex; align-items: center; gap: 7px; margin-top: 4px; overflow: hidden; font-size: 10px; line-height: 1.2; white-space: nowrap; }
.metric-comparison span { overflow: hidden; text-overflow: ellipsis; }
.metric-comparison .is-up { color: #d97706; }
.metric-comparison .is-down { color: #2f9e78; }
.metric-comparison .is-flat,
.metric-comparison .is-unavailable { color: var(--log-text-muted); }
.tone-blue { --metric-color: #4d85f3; --metric-bg: #eaf2ff; }
.tone-cyan { --metric-color: #24aebe; --metric-bg: #e7f8fa; }
.tone-teal { --metric-color: #159a9c; --metric-bg: #e4f8f7; }
.tone-indigo { --metric-color: #7480e9; --metric-bg: #eff0ff; }
.tone-green { --metric-color: #35ae89; --metric-bg: #e8f8f2; }
.tone-amber { --metric-color: #e8a13f; --metric-bg: #fff5e5; }
.tone-red { --metric-color: #e96972; --metric-bg: #ffedef; }
.tone-violet { --metric-color: #946fd4; --metric-bg: #f3edfc; }

.primary-grid { display: grid; grid-template-columns: minmax(0, 1.65fr) minmax(300px, 0.75fr); gap: 9px; margin-bottom: 10px; }
.analytics-grid { display: grid; grid-template-columns: minmax(0, 1.65fr) minmax(300px, 0.75fr); align-items: start; gap: 9px; margin-bottom: 10px; }
.analytics-column { display: grid; align-content: start; gap: 9px; min-width: 0; }
.api-panel { min-height: 0; }
.rank-column { display: grid; grid-template-rows: 1fr 1fr; gap: 9px; }
.rank-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 9px; margin-bottom: 10px; }
.dashboard-panel { min-width: 0; padding: 12px 13px 10px; }
.panel-header { min-height: 38px; justify-content: space-between; gap: 14px; margin-bottom: 7px; }
.panel-header h3 { margin: 0; color: var(--text); font-size: 14px; line-height: 1.4; }
.panel-header p { margin: 3px 0 0; color: var(--muted); font-size: 10px; }
.panel-unit,
.panel-count { color: var(--muted); font-size: 10px; white-space: nowrap; }
.chart-large { height: 240px; }
.chart-geo { height: 300px; }
.chart { width: 100%; min-width: 0; }
.dashboard-panel :deep(.el-empty) { height: 220px; padding: 0; }
.rank-panel { min-height: 0; }
.rank-panel :deep(.el-empty) { height: 92px; }
.rank-panel-tall { min-height: 280px; }
.rank-panel-tall :deep(.el-empty) { height: 190px; }

.geo-panel,
.geo-top-panel { min-height: 0; }
.geo-panel { padding-bottom: 6px; }
.geo-rank-list { display: grid; gap: 12px; padding: 12px 4px 0; }
.geo-rank-item { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 8px; color: var(--text-secondary); font-size: 12px; }
.geo-rank-item > span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.geo-rank-item i { display: inline-grid; width: 20px; height: 20px; margin-right: 8px; place-items: center; border-radius: 6px; color: #4c8bf5; background: rgba(76, 139, 245, 0.12); font-size: 10px; font-style: normal; }
.geo-rank-item strong { color: var(--text); font-size: 12px; }
.geo-rank-track { grid-column: 1 / -1; height: 7px; margin-top: -3px; overflow: hidden; border-radius: 4px; background: #edf1f5; }
.geo-rank-track span { display: block; height: 100%; border-radius: inherit; background: #3d87ef; }

.rank-list { display: grid; gap: 9px; padding-top: 2px; }
.rank-item { min-width: 0; }
.rank-copy { justify-content: space-between; gap: 12px; color: var(--text-secondary); font-size: 11px; }
.rank-copy > span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.rank-copy i { display: inline-grid; width: 18px; height: 18px; margin-right: 7px; place-items: center; border-radius: 6px; color: #4c8bf5; background: rgba(76, 139, 245, 0.12); font-size: 9px; font-style: normal; }
.rank-copy strong { flex: 0 0 auto; color: var(--text); font-size: 11px; }
.rank-track { height: 5px; margin: 5px 0 0 25px; overflow: hidden; border-radius: 3px; background: #edf1f5; }
.rank-track span { display: block; height: 100%; border-radius: inherit; background: linear-gradient(90deg, #6aa1fb, #7d72d8); }
.latency-list { display: grid; gap: 0; }
.latency-row { display: grid; grid-template-columns: 100px 64px minmax(80px, 1fr); align-items: center; min-height: 32px; gap: 8px; border-bottom: 1px solid #f0f3f6; font-size: 11px; }
.latency-row:last-child { border-bottom: 0; }
.latency-row span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.latency-row strong { color: var(--text); text-align: right; }
.latency-track { height: 5px; overflow: hidden; border-radius: 3px; background: #edf1f5; }
.latency-track span { display: block; height: 100%; border-radius: inherit; background: #6aa1fb; }

.table-panel { margin-bottom: 10px; padding-bottom: 10px; }
.data-table { --el-table-header-bg-color: #f8fafc; --el-table-row-hover-bg-color: #f1f7ff; --el-table-border-color: #edf1f5; --el-table-header-text-color: #687588; --el-table-text-color: #465266; width: 100%; font-size: 11px; }
.data-table :deep(.el-table__inner-wrapper::before) { display: none; }
.data-table :deep(th.el-table__cell) { padding: 7px 0; font-weight: 600; }
.data-table :deep(td.el-table__cell) { padding: 7px 0; }
.clickable-table :deep(.el-table__row) { cursor: pointer; }
.danger-value { color: #df4f5b; font-weight: 700; }

.binding-toolbar { justify-content: space-between; gap: 18px; margin-bottom: 14px; }
.binding-toolbar > div { display: grid; gap: 3px; }
.binding-toolbar strong { color: var(--text); font-size: 14px; }
.binding-toolbar span { color: var(--muted); font-size: 11px; }
.binding-pagination { justify-content: flex-end; margin-top: 14px; }

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
.log-page :deep(.el-date-editor.el-input__wrapper.is-active) { box-shadow: inset 0 0 0 1px rgba(120, 156, 221, 0.96), 0 0 0 4px rgba(76, 139, 245, 0.08) !important; }
.log-page :deep(.el-button) { border-radius: 9px !important; font-weight: 600; transform: none !important; }
.log-page :deep(.el-button--primary:not(.is-plain):not(.is-link)) { border-color: transparent; background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%); box-shadow: 0 2px 8px rgba(76, 139, 245, 0.28); }
@media (max-width: 1500px) {
  .metric-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); }
}

@media (max-width: 1100px) {
  .page-card { height: 100%; min-height: 0; max-height: 100%; }
  .primary-grid,
  .analytics-grid { grid-template-columns: 1fr; }
  .rank-column { grid-template-columns: repeat(2, minmax(0, 1fr)); grid-template-rows: none; }
  .rank-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .time-filter { align-items: center; }
}

@media (max-width: 760px) {
  .dashboard-header { align-items: flex-start; flex-direction: column; }
  .header-actions { width: 100%; justify-content: flex-start; }
  .updated-at { width: 100%; }
  .metric-grid,
  .rank-grid,
  .rank-column { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .time-filter { flex-direction: column; }
  .time-filter-content { width: 100%; align-items: center; }
  .time-presets { flex-wrap: nowrap; }
  .metric-card { min-height: 94px; padding: 9px 12px; }
  .metric-card strong { font-size: 20px; }
  .dashboard-panel { padding: 12px; }
}

@media (max-width: 480px) {
  .metric-grid,
  .rank-grid,
  .rank-column { grid-template-columns: 1fr; }
  .chart-large { height: 260px; }
}

@media (prefers-reduced-motion: reduce) {
  .dashboard-content { transition: none; }
}
</style>
