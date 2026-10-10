<template>
  <main class="billing-workspace details-page">
    <div class="billing-workbench">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <el-icon><Document /></el-icon>
      </span>
      <div class="ops-page-header__content">
        <h1>账单明细</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>{{ detail.providerName || providerName(operator) }} / {{ selectedMonth }} 费用趋势、产品分类与资源账单</p>
      </div>
      <div class="ops-page-header__actions">
        <el-select v-model="operator" aria-label="云厂商" @change="changeProvider">
          <el-option v-for="provider in BILLING_PROVIDER_OPTIONS" :key="provider.keyType" :label="provider.label" :value="provider.code" />
        </el-select>
        <el-select v-model="cloudAccountId" clearable placeholder="全部账号" aria-label="云账号" @change="changeAccount">
          <el-option v-for="account in allAccounts" :key="account.id" :label="account.accountName || account.accountId" :value="account.id" />
        </el-select>
        <el-date-picker v-model="selectedMonth" type="month" format="YYYY-MM" value-format="YYYY-MM" :clearable="false" aria-label="账单月份" @change="changeMonth" />
      </div>
    </header>

    <el-alert v-if="errorMessage" class="page-alert" type="error" :title="errorMessage" show-icon :closable="false" />

    <template v-if="detailLoading">
      <section class="metric-grid loading-band"><div v-for="item in 5" :key="item" class="metric-card"><el-skeleton :rows="2" animated /></div></section>
      <section class="analytics-grid"><div class="ops-card chart-panel loading-panel"><el-skeleton :rows="6" animated /></div><div class="ops-card chart-panel loading-panel"><el-skeleton :rows="6" animated /></div></section>
    </template>

    <template v-else>
      <section class="metric-grid" aria-label="费用指标">
        <article class="metric-card metric-card--primary">
          <span>当前余额</span>
          <strong>{{ formatCurrency(detail.balance, detail.currency) }}</strong>
          <small>{{ displayAccounts.length }} 个账号</small>
        </article>
        <article class="metric-card metric-card--cost">
          <span>当月费用</span>
          <strong>{{ formatCurrency(detail.comparison.currentCost, detail.currency) }}</strong>
          <small>{{ detail.comparison.currentMonth }}</small>
        </article>
        <article class="metric-card metric-card--previous">
          <span>上月费用</span>
          <strong>{{ formatCurrency(detail.comparison.previousCost, detail.currency) }}</strong>
          <small>{{ detail.comparison.previousMonth }}</small>
        </article>
        <article class="metric-card metric-card--comparison">
          <span>费用环比</span>
          <strong :class="rateClass(detail.comparison.momRate)">{{ formatRate(detail.comparison.momRate) }}</strong>
          <small>与上月比较</small>
        </article>
        <article class="metric-card metric-card--comparison">
          <span>费用同比</span>
          <strong :class="rateClass(detail.comparison.yoyRate)">{{ formatRate(detail.comparison.yoyRate) }}</strong>
          <small>与去年同期比较</small>
        </article>
      </section>

      <section class="analytics-grid">
        <article class="ops-card chart-panel trend-panel">
          <header class="ops-card__header">
            <div><h2>近 12 个月费用趋势</h2><p>{{ detail.currency }} / 月度汇总</p></div>
            <div class="heading-value"><span>累计费用</span><b>{{ formatCurrency(trendTotal, detail.currency) }}</b></div>
          </header>
          <div v-if="detail.trend.length" ref="trendChartRef" class="chart trend-chart" aria-label="近12个月费用趋势图"></div>
          <el-empty v-else description="暂无趋势数据" :image-size="70" />
        </article>

        <article class="ops-card chart-panel product-panel">
          <header class="ops-card__header">
            <div><h2>产品费用分类</h2><p>{{ selectedMonth }} / 按产品类型</p></div>
            <div class="heading-value"><span>产品类型</span><b>{{ detail.products.length }}</b></div>
          </header>
          <div v-if="detail.products.length" ref="productChartRef" class="chart product-chart" aria-label="产品费用分类图"></div>
          <el-empty v-else description="暂无产品分类数据" :image-size="70" />
        </article>
      </section>

      <section class="ops-card product-table-panel">
        <header class="ops-card__header">
          <div><h2>产品费用明细</h2><p>当月费用与环比</p></div>
        </header>
        <el-table :data="detail.products" max-height="260" class="product-table" empty-text="暂无产品费用数据">
          <el-table-column type="index" label="#" width="52" />
          <el-table-column prop="productName" label="产品类型" min-width="180" show-overflow-tooltip />
          <el-table-column label="当月费用" min-width="140" align="right"><template #default="{ row }"><b>{{ formatCurrency(row.amount, detail.currency) }}</b></template></el-table-column>
          <el-table-column label="上月费用" min-width="140" align="right"><template #default="{ row }">{{ formatCurrency(row.previousAmount, detail.currency) }}</template></el-table-column>
          <el-table-column label="费用环比" width="110" align="right"><template #default="{ row }"><span :class="rateClass(row.momRate)">{{ formatRate(row.momRate) }}</span></template></el-table-column>
          <el-table-column label="费用占比" width="100" align="right"><template #default="{ row }">{{ Number(row.percent || 0).toFixed(1) }}%</template></el-table-column>
        </el-table>
      </section>

      <section class="ops-card resource-panel">
        <header class="ops-card__header resource-heading">
          <div><h2>资源账单</h2><p>{{ detailTotal }} 条明细</p></div>
          <el-button type="success" size="small" :loading="exporting" @click="exportDetails"><el-icon><Download /></el-icon>导出</el-button>
        </header>
        <div class="resource-toolbar">
          <el-input v-model="detailQuery.keyword" clearable placeholder="资源 ID、资源名称、订单 ID" @keyup.enter="searchDetails"><template #prefix><el-icon><Search /></el-icon></template></el-input>
          <el-select v-model="detailQuery.productName" clearable filterable placeholder="全部产品" @change="searchDetails">
            <el-option v-for="product in detail.products" :key="product.productName" :label="product.productName" :value="product.productName" />
          </el-select>
          <el-select v-model="detailQuery.chargeType" clearable placeholder="全部付费模式" @change="searchDetails"><el-option label="按量计费" value="按量计费" /><el-option label="包年包月" value="包年包月" /></el-select>
          <el-input v-model="detailQuery.region" clearable placeholder="地域" @keyup.enter="searchDetails" />
          <el-button type="primary" size="small" @click="searchDetails"><el-icon><Search /></el-icon>查询</el-button>
          <el-button type="warning" size="small" @click="resetDetails">重置</el-button>
        </div>

        <el-table v-loading="detailsLoading" :data="detailRows" class="detail-table" row-key="id">
          <el-table-column label="资源" min-width="200" fixed="left">
            <template #default="{ row }"><div class="resource-cell"><b>{{ row.resourceName || row.resourceId || '-' }}</b><small>{{ row.resourceId || '-' }}</small></div></template>
          </el-table-column>
          <el-table-column prop="productName" label="产品类型" min-width="150" show-overflow-tooltip />
          <el-table-column prop="billingItemName" label="计费项" min-width="150" show-overflow-tooltip />
          <el-table-column label="用量" min-width="115" align="right"><template #default="{ row }">{{ row.usageQuantity || '-' }} {{ row.usageUnit || '' }}</template></el-table-column>
          <el-table-column prop="chargeType" label="付费模式" width="105" />
          <el-table-column prop="region" label="地域" min-width="110" show-overflow-tooltip />
          <el-table-column label="实际费用" min-width="125" align="right"><template #default="{ row }"><b class="money-text">{{ formatCurrency(row.payableAmount, row.currency || detail.currency) }}</b></template></el-table-column>
          <el-table-column label="扣费时间" min-width="165"><template #default="{ row }">{{ formatBillTime(row.deductedAt) }}</template></el-table-column>
          <el-table-column label="操作" width="76" fixed="right" align="center"><template #default="{ row }"><el-button link type="primary" @click="openDetail(row.id)">详情</el-button></template></el-table-column>
        </el-table>
        <div class="pagination-wrap">
          <el-pagination v-model:current-page="detailQuery.page" v-model:page-size="detailQuery.pageSize" :page-sizes="[10, 20, 50, 100]" :total="detailTotal" layout="total, sizes, prev, pager, next" @size-change="loadDetails" @current-change="loadDetails" />
        </div>
      </section>
    </template>

    <el-dialog v-model="detailDialogVisible" title="资源账单详情" width="820px" class="ops-billing-dialog" modal-class="billing-dialog-overlay" destroy-on-close>
      <el-skeleton v-if="detailRecordLoading" :rows="8" animated />
      <el-descriptions v-else :column="2" border>
        <el-descriptions-item label="云厂商">{{ providerName(detailRecord.operator) }}</el-descriptions-item>
        <el-descriptions-item label="账单月份">{{ detailRecord.month || '-' }}</el-descriptions-item>
        <el-descriptions-item label="产品类型">{{ detailRecord.productName || '-' }}</el-descriptions-item>
        <el-descriptions-item label="子产品">{{ detailRecord.subProductName || '-' }}</el-descriptions-item>
        <el-descriptions-item label="资源名称">{{ detailRecord.resourceName || '-' }}</el-descriptions-item>
        <el-descriptions-item label="资源 ID">{{ detailRecord.resourceId || '-' }}</el-descriptions-item>
        <el-descriptions-item label="计费项">{{ detailRecord.billingItemName || '-' }}</el-descriptions-item>
        <el-descriptions-item label="计费项编码">{{ detailRecord.billingItemCode || '-' }}</el-descriptions-item>
        <el-descriptions-item label="用量">{{ detailRecord.usageQuantity || '-' }} {{ detailRecord.usageUnit || '' }}</el-descriptions-item>
        <el-descriptions-item label="付费模式">{{ detailRecord.chargeType || '-' }}</el-descriptions-item>
        <el-descriptions-item label="原价">{{ formatCurrency(detailRecord.originalAmount, detailRecord.currency) }}</el-descriptions-item>
        <el-descriptions-item label="实际费用"><b class="money-text">{{ formatCurrency(detailRecord.payableAmount, detailRecord.currency) }}</b></el-descriptions-item>
        <el-descriptions-item label="优惠金额">{{ formatCurrency(detailRecord.discountAmount, detailRecord.currency) }}</el-descriptions-item>
        <el-descriptions-item label="地域 / 可用区">{{ detailRecord.region || '-' }} / {{ detailRecord.zone || '-' }}</el-descriptions-item>
        <el-descriptions-item label="订单 ID">{{ detailRecord.orderId || '-' }}</el-descriptions-item>
        <el-descriptions-item label="交易 ID">{{ detailRecord.transactionId || '-' }}</el-descriptions-item>
        <el-descriptions-item label="开始时间">{{ formatBillTime(detailRecord.billingStartAt) }}</el-descriptions-item>
        <el-descriptions-item label="结束时间">{{ formatBillTime(detailRecord.billingEndAt) }}</el-descriptions-item>
      </el-descriptions>
      <template #footer><div class="dialog-footer"><el-button @click="detailDialogVisible = false">关闭</el-button></div></template>
    </el-dialog>
    </div>
  </main>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Document, Download, Search } from '@element-plus/icons-vue'
import * as echarts from 'echarts'
import dayjs from 'dayjs'
import billApi from '@/api/bill/bill.js'
import accountApi from '@/api/bill/datasource.js'
import { BILLING_PROVIDER_OPTIONS, formatCurrency, formatRate, providerName, rateClass, requestError, responseData, responsePage } from './billing-format.js'

const route = useRoute()
const router = useRouter()
const normalizeProviderCode = value => {
  const providerCode = String(Array.isArray(value) ? value[0] : value || '').trim().toLowerCase()
  const providerByKeyType = BILLING_PROVIDER_OPTIONS.find(item => String(item.keyType) === providerCode)
  if (providerByKeyType) return providerByKeyType.code

  const providerByLabel = BILLING_PROVIDER_OPTIONS.find(item => item.label.toLowerCase() === providerCode)
  if (providerByLabel) return providerByLabel.code

  const aliases = { tencent: 'tencentcloud', huaweicloud: 'huawei', volc: 'volcengine' }
  const normalizedCode = aliases[providerCode] || providerCode
  return BILLING_PROVIDER_OPTIONS.some(item => item.code === normalizedCode) ? normalizedCode : ''
}

const operator = ref(normalizeProviderCode(route.query.operator))
const cloudAccountId = ref(route.query.cloudAccountId ? Number(route.query.cloudAccountId) : '')
const selectedMonth = ref(route.query.month || dayjs().format('YYYY-MM'))
const detailLoading = ref(true)
const detailsLoading = ref(false)
const exporting = ref(false)
const errorMessage = ref('')
const allAccounts = ref([])
const detail = reactive({ providerCode: '', providerName: '', currency: 'CNY', balance: 0, comparison: { currentMonth: '', currentCost: 0, previousMonth: '', previousCost: 0, lastYearMonth: '', lastYearCost: 0, momRate: null, yoyRate: null }, trend: [], products: [], accounts: [] })
const detailRows = ref([])
const detailTotal = ref(0)
const detailQuery = reactive({ keyword: '', productName: '', chargeType: '', region: '', page: 1, pageSize: 20 })
const detailDialogVisible = ref(false)
const detailRecordLoading = ref(false)
const detailRecord = ref({})
const trendChartRef = ref()
const productChartRef = ref()
let trendChart
let productChart
let resizeObserver

const displayAccounts = computed(() => cloudAccountId.value ? detail.accounts : allAccounts.value)
const trendTotal = computed(() => detail.trend.reduce((sum, item) => sum + (Number(item.amount) || 0), 0))
const compactParams = source => Object.fromEntries(Object.entries(source).filter(([, value]) => value !== '' && value !== null && value !== undefined))

const getAccountProvider = account => normalizeProviderCode(
  account?.operator || account?.providerCode || account?.provider || account?.providerName || account?.keyType
)

const resolveInitialProvider = async () => {
  if (operator.value) return

  let configuredProvider = ''
  try {
    const accountPage = responsePage(await accountApi.getAccounts({ page: 1, pageSize: 500 }))
    const matchedAccount = cloudAccountId.value
      ? accountPage.list.find(account => Number(account.id) === cloudAccountId.value)
      : null
    const configuredAccount = matchedAccount || accountPage.list.find(account => Number(account.enabled) !== 0) || accountPage.list[0]
    configuredProvider = getAccountProvider(configuredAccount)

    if (configuredProvider) {
      operator.value = configuredProvider
      return
    }
  } catch (_) { /* 账单总览仍可作为厂商数据来源 */ }

  try {
    const overview = responseData(await billApi.getOverview({ month: selectedMonth.value })) || {}
    const billedProvider = (Array.isArray(overview.providers) ? overview.providers : [])
      .map(provider => normalizeProviderCode(provider.providerCode || provider.operator || provider.code || provider.providerName))
      .find(Boolean)
    operator.value = billedProvider || configuredProvider || BILLING_PROVIDER_OPTIONS[0].code
  } catch (_) {
    operator.value = configuredProvider || BILLING_PROVIDER_OPTIONS[0].code
  }
}

const updateRoute = () => router.replace({ path: route.path, query: compactParams({ operator: operator.value, month: selectedMonth.value, cloudAccountId: cloudAccountId.value }) })

const loadProvider = async () => {
  detailLoading.value = true
  errorMessage.value = ''
  try {
    const params = compactParams({ month: selectedMonth.value, cloudAccountId: cloudAccountId.value })
    const data = responseData(await billApi.getProviderDetail(operator.value, params)) || {}
    Object.assign(detail, data)
    detail.comparison = data.comparison || {}
    detail.trend = Array.isArray(data.trend) ? data.trend : []
    detail.products = Array.isArray(data.products) ? data.products : []
    detail.accounts = Array.isArray(data.accounts) ? data.accounts : []
    if (!cloudAccountId.value) allAccounts.value = detail.accounts
    detailLoading.value = false
    await nextTick()
    renderCharts()
  } catch (error) {
    errorMessage.value = requestError(error, '获取云厂商账单失败')
    detail.trend = []
    detail.products = []
    detail.accounts = []
    disposeCharts()
  } finally { detailLoading.value = false }
}

const loadProviderAccounts = async () => {
  try {
    const data = responseData(await billApi.getProviderDetail(operator.value, { month: selectedMonth.value })) || {}
    allAccounts.value = Array.isArray(data.accounts) ? data.accounts : []
  } catch (_) { allAccounts.value = [] }
}

const detailParams = (withPaging = true) => compactParams({
  month: selectedMonth.value,
  operator: operator.value,
  cloudAccountId: cloudAccountId.value,
  keyword: detailQuery.keyword,
  productName: detailQuery.productName,
  chargeType: detailQuery.chargeType,
  region: detailQuery.region,
  page: withPaging ? detailQuery.page : undefined,
  pageSize: withPaging ? detailQuery.pageSize : undefined
})

const loadDetails = async () => {
  detailsLoading.value = true
  try {
    const page = responsePage(await billApi.getDetails(detailParams()))
    detailRows.value = page.list
    detailTotal.value = page.total
  } catch (error) {
    detailRows.value = []
    detailTotal.value = 0
    ElMessage.error(requestError(error, '获取资源账单失败'))
  } finally { detailsLoading.value = false }
}

const reloadPage = async () => {
  updateRoute()
  await Promise.all([loadProvider(), loadDetails()])
}

const changeProvider = async () => { cloudAccountId.value = ''; allAccounts.value = []; detailQuery.page = 1; await reloadPage() }
const changeAccount = async () => { detailQuery.page = 1; updateRoute(); if (cloudAccountId.value && !allAccounts.value.length) await loadProviderAccounts(); await Promise.all([loadProvider(), loadDetails()]) }
const changeMonth = async () => { detailQuery.page = 1; await reloadPage() }
const searchDetails = () => { detailQuery.page = 1; loadDetails() }
const resetDetails = () => { Object.assign(detailQuery, { keyword: '', productName: '', chargeType: '', region: '', page: 1 }); loadDetails() }

const formatBillTime = value => value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-'

const openDetail = async id => {
  detailDialogVisible.value = true
  detailRecordLoading.value = true
  try { detailRecord.value = responseData(await billApi.getDetail(id)) || {} }
  catch (error) { ElMessage.error(requestError(error, '获取账单详情失败')) }
  finally { detailRecordLoading.value = false }
}

const exportDetails = async () => {
  exporting.value = true
  try {
    const response = await billApi.exportDetails(detailParams(false))
    const blob = new Blob([response.data], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `云账单明细_${operator.value}_${selectedMonth.value}.xlsx`
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
    ElMessage.success('账单明细已导出')
  } catch (error) { ElMessage.error(requestError(error, '导出账单明细失败')) }
  finally { exporting.value = false }
}

const renderCharts = () => {
  disposeCharts()
  const accent = '#2563eb'
  if (trendChartRef.value && detail.trend.length) {
    trendChart = echarts.init(trendChartRef.value)
    trendChart.setOption({
      animationDuration: 180,
      grid: { left: 12, right: 16, top: 18, bottom: 10, containLabel: true },
      tooltip: { trigger: 'axis', valueFormatter: value => formatCurrency(value, detail.currency) },
      xAxis: { type: 'category', boundaryGap: false, data: detail.trend.map(item => item.month), axisLine: { lineStyle: { color: '#dbe7f4' } }, axisLabel: { color: '#64748b', fontSize: 11 } },
      yAxis: { type: 'value', splitLine: { lineStyle: { color: '#e7eef7' } }, axisLabel: { color: '#64748b', fontSize: 11 } },
      series: [{ type: 'line', smooth: true, symbolSize: 5, data: detail.trend.map(item => item.amount), lineStyle: { width: 2, color: accent }, itemStyle: { color: accent }, areaStyle: { color: 'rgba(37,99,235,.08)' } }]
    })
  }
  if (productChartRef.value && detail.products.length) {
    productChart = echarts.init(productChartRef.value)
    const rows = detail.products.slice(0, 8).reverse()
    productChart.setOption({
      animationDuration: 180,
      grid: { left: 12, right: 22, top: 14, bottom: 8, containLabel: true },
      tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, valueFormatter: value => formatCurrency(value, detail.currency) },
      xAxis: { type: 'value', splitLine: { lineStyle: { color: '#e7eef7' } }, axisLabel: { color: '#64748b', fontSize: 11 } },
      yAxis: { type: 'category', data: rows.map(item => item.productName), axisLine: { show: false }, axisTick: { show: false }, axisLabel: { color: '#53657d', fontSize: 11, width: 100, overflow: 'truncate' } },
      series: [{ type: 'bar', data: rows.map(item => item.amount), barMaxWidth: 14, itemStyle: { color: accent, borderRadius: [0, 3, 3, 0] } }]
    })
  }
  observeCharts()
}

const observeCharts = () => {
  resizeObserver?.disconnect()
  resizeObserver = new ResizeObserver(() => { trendChart?.resize(); productChart?.resize() })
  if (trendChartRef.value) resizeObserver.observe(trendChartRef.value)
  if (productChartRef.value) resizeObserver.observe(productChartRef.value)
}
const disposeCharts = () => { resizeObserver?.disconnect(); trendChart?.dispose(); productChart?.dispose(); trendChart = undefined; productChart = undefined }

onMounted(async () => {
  await resolveInitialProvider()
  if (cloudAccountId.value) await loadProviderAccounts()
  await Promise.all([loadProvider(), loadDetails()])
})
onBeforeUnmount(disposeCharts)
</script>

<style scoped>
.billing-workspace {
  --ops-text-heading: #0f172a;
  --ops-text-primary: #172033;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-color-primary: #2563eb;
  --ops-color-success: #10b981;
  --ops-color-warning: #f59e0b;
  --ops-bg-surface: #fff;
  --ops-bg-surface-soft: #fbfdff;
  --ops-bg-subtle: #f3f7fc;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  box-sizing: border-box;
  height: calc(100vh - 124px);
  min-height: 0;
  overflow: auto;
  padding: 7px;
  color: var(--ops-text-primary);
  background: transparent;
  font-family: "PingFang SC", "Microsoft YaHei", sans-serif;
  letter-spacing: 0;
}

:global(.home-container .el-main:has(.details-page)) {
  padding-top: 10px;
  padding-right: 10px;
  padding-left: 10px;
}

.billing-workbench { display: grid; gap: 8px; min-width: 0; }

.ops-page-header {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  padding: 8px 12px;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.97);
  box-shadow: 0 10px 28px rgba(80, 124, 180, 0.1);
}

.ops-page-header__icon {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  place-items: center;
  color: var(--ops-color-primary);
  font-size: 29px;
}

.ops-page-header__content,
.ops-page-header__actions,
.resource-toolbar {
  display: flex;
  align-items: center;
}

.ops-page-header__content { gap: 10px; min-width: 0; }
.ops-page-header h1 { margin: 0; color: var(--ops-text-heading); font-size: 20px; font-weight: 700; line-height: 1.2; white-space: nowrap; }
.ops-page-header__divider { width: 1px; height: 22px; flex: 0 0 1px; background: var(--ops-border-soft); }
.ops-page-header p { margin: 0; overflow: hidden; color: var(--ops-text-muted); font-size: 12px; line-height: 1.5; text-overflow: ellipsis; white-space: nowrap; }
.ops-page-header__actions { gap: 8px; margin-left: auto; }
.ops-page-header__actions :deep(.el-select) { width: 138px; }
.ops-page-header__actions :deep(.el-date-editor) { width: 132px; }
.page-alert { margin: 0; border-radius: 8px; }

.metric-grid {
  display: grid;
  grid-template-columns: 1.25fr repeat(4, minmax(0, 1fr));
  gap: 8px;
  min-width: 0;
}

.metric-card {
  position: relative;
  min-width: 0;
  min-height: 88px;
  overflow: hidden;
  padding: 10px 11px 9px;
  border: 1px solid var(--ops-border);
  border-radius: 10px;
  background: linear-gradient(180deg, #fff, #fbfdff);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
}

.metric-card::before { position: absolute; top: 0; right: 0; left: 0; height: 3px; background: #60a5fa; content: ""; }
.metric-card--cost::before { background: #0ea5e9; }
.metric-card--previous::before { background: #94a3b8; }
.metric-card--comparison::before { background: #6366f1; }
.metric-card span,
.metric-card small { display: block; }
.metric-card span { color: var(--ops-text-secondary); font-size: 11px; font-weight: 700; }
.metric-card strong { display: block; margin-top: 9px; overflow-wrap: anywhere; color: var(--ops-text-heading); font-family: Consolas, "SFMono-Regular", monospace; font-size: 20px; font-weight: 800; line-height: 1.05; font-variant-numeric: tabular-nums; }
.metric-card--primary strong { color: var(--ops-color-primary); font-size: 23px; }
.metric-card small { margin-top: 7px; color: #7b8aa0; font-size: 11px; }
.loading-band .metric-card { padding-top: 15px; }

.ops-card {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.97);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
}

.ops-card__header { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 12px; border-bottom: 1px solid var(--ops-border-soft); }
.ops-card__header h2 { margin: 0; color: var(--ops-text-heading); font-size: 15px; font-weight: 700; line-height: 1.25; }
.ops-card__header p { margin: 3px 0 0; color: var(--ops-text-muted); font-size: 11px; line-height: 1.4; }

.analytics-grid { display: grid; grid-template-columns: minmax(0, 1.45fr) minmax(340px, 0.8fr); gap: 8px; min-width: 0; }
.chart-panel { position: relative; min-width: 0; overflow: hidden; background: var(--ops-bg-surface-soft); }
.chart-panel::before { position: absolute; z-index: 1; top: 0; right: 0; left: 0; height: 3px; background: var(--ops-color-primary); content: ""; }
.product-panel::before { background: #0ea5e9; }
.loading-panel { min-height: 258px; padding: 14px 12px; }
.heading-value { text-align: right; }
.heading-value span { display: block; margin-bottom: 3px; color: var(--ops-text-muted); font-size: 10.5px; }
.heading-value b { color: var(--ops-text-heading); font-family: Consolas, "SFMono-Regular", monospace; font-size: 13px; font-variant-numeric: tabular-nums; }
.chart { width: 100%; height: 210px; }
.chart-panel :deep(.el-empty) { height: 210px; padding: 0; }
.product-table-panel :deep(.el-table) { border: 0; }
.resource-heading { border-bottom: 0; }

.resource-toolbar { gap: 8px; padding: 7px 10px; border-top: 1px solid var(--ops-border-soft); border-bottom: 1px solid var(--ops-border-soft); background: var(--ops-bg-subtle); }
.resource-toolbar :deep(.el-input) { width: 220px; }
.resource-toolbar :deep(.el-select) { width: 148px; }
.resource-toolbar :deep(.el-input:nth-of-type(2)) { width: 132px; }
.resource-toolbar :deep(.el-button) { min-height: 30px; padding: 0 10px; font-size: 12px; }
.detail-table { width: 100%; }
.resource-cell b,
.resource-cell small { display: block; max-width: 185px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.resource-cell b { color: var(--ops-text-primary); font-size: 12px; }
.resource-cell small { margin-top: 3px; color: var(--ops-text-muted); font-size: 10.5px; }
.money-text { color: var(--ops-color-primary); font-family: Consolas, "SFMono-Regular", monospace; font-variant-numeric: tabular-nums; }
.pagination-wrap { display: flex; justify-content: flex-end; padding: 9px 10px; border-top: 1px solid var(--ops-border-soft); }
.dialog-footer { display: flex; justify-content: flex-end; }
.is-increase { color: #dc2626 !important; }
.is-decrease { color: #059669 !important; }
.is-neutral { color: var(--ops-text-muted) !important; }

.details-page :deep(.el-button) { border-radius: 8px; font-weight: 600; transition: color 160ms ease, background-color 160ms ease, border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease; }
.details-page :deep(.el-button:hover) { transform: translateY(-1px); }
.details-page :deep(.el-button:active) { transform: translateY(0); }
.details-page :deep(.el-table) { --el-table-border-color: #eef3f9; color: var(--ops-text-primary); font-size: 12px; }
.details-page :deep(.el-table th.el-table__cell) { height: 36px; padding: 0; color: var(--ops-text-secondary); background: #f3f7fc; font-size: 12px; font-weight: 700; }
.details-page :deep(.el-table td.el-table__cell) { padding: 6px 0; }
.details-page :deep(.el-table__row:hover > td.el-table__cell) { background: #f8fafc; }
.details-page :deep(.el-input__wrapper),
.details-page :deep(.el-select__wrapper) { min-height: 34px; border-radius: 8px; background: #fbfdff; box-shadow: 0 0 0 1px #d9e5f2 inset; }
.details-page :deep(.el-input__wrapper:hover),
.details-page :deep(.el-select__wrapper:hover) { box-shadow: 0 0 0 1px #b9d4ff inset; }

:global(.billing-dialog-overlay .el-dialog) {
  overflow: hidden;
  max-width: calc(100vw - 24px);
  border: 1px solid #d7e6f6;
  border-radius: 15px !important;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.ops-billing-dialog .el-dialog__header) { margin-right: 0; padding: 16px 18px 12px; border-bottom: 1px solid #e3edf8; }
:global(.ops-billing-dialog .el-dialog__title) { color: #0f172a; font-size: 17px; font-weight: 700; }
:global(.ops-billing-dialog .el-dialog__body) { padding: 16px 18px; }
:global(.ops-billing-dialog .el-dialog__footer) { padding: 12px 18px 16px; border-top: 1px solid #e3edf8; }
:global(.ops-billing-dialog .el-descriptions__label) { width: 112px; color: #475569; font-size: 12px; font-weight: 700; }
:global(.ops-billing-dialog .el-descriptions__content) { color: #172033; font-size: 12px; }

@media (max-width: 1200px) {
  .metric-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .analytics-grid { grid-template-columns: 1fr; }
  .resource-toolbar { flex-wrap: wrap; }
}

@media (max-width: 960px) {
  .ops-page-header { align-items: flex-start; flex-wrap: wrap; }
  .ops-page-header__actions { width: 100%; margin-left: 54px; flex-wrap: wrap; }
  .metric-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (max-width: 620px) {
  .billing-workspace { padding: 6px; }
  .ops-page-header__icon { width: 36px; height: 36px; flex-basis: 36px; font-size: 26px; }
  .ops-page-header__content { align-items: flex-start; flex: 1; flex-direction: column; gap: 2px; }
  .ops-page-header__divider { display: none; }
  .ops-page-header p { width: 100%; white-space: normal; }
  .ops-page-header__actions { margin-left: 0; }
  .ops-page-header__actions :deep(.el-select),
  .ops-page-header__actions :deep(.el-date-editor) { width: calc(50% - 4px); }
  .ops-page-header__actions :deep(.el-date-editor) { flex: 1; }
  .metric-grid { grid-template-columns: 1fr; }
  .chart { height: 190px; }
  .chart-panel :deep(.el-empty) { height: 190px; }
  .resource-toolbar :deep(.el-input),
  .resource-toolbar :deep(.el-select) { width: calc(50% - 4px); }
  .resource-toolbar :deep(.el-button) { flex: 1; }
  .pagination-wrap { justify-content: flex-start; overflow-x: auto; }
  :global(.ops-billing-dialog .el-dialog__body) { overflow-x: auto; }
}

@media (prefers-reduced-motion: reduce) {
  .details-page *,
  .details-page *::before,
  .details-page *::after {
    transition-duration: 0.01ms !important;
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
  }
}
</style>
