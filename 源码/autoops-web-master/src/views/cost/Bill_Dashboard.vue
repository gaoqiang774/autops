<template>
  <main class="billing-workspace overview-page">
    <div class="billing-workbench">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <el-icon><DataAnalysis /></el-icon>
      </span>
      <div class="ops-page-header__content">
        <h1>账单总览</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>{{ overview.month }} 云资源费用、账户余额与变化趋势</p>
      </div>
      <div class="ops-page-header__actions">
        <el-date-picker
          v-model="selectedMonth"
          type="month"
          format="YYYY-MM"
          value-format="YYYY-MM"
          :clearable="false"
          aria-label="账单月份"
          @change="loadOverview"
        />
        <el-tooltip content="刷新全部余额" placement="bottom">
          <el-button class="ops-icon-button ops-refresh-button" type="warning" :loading="refreshing" aria-label="刷新全部余额" @click="refreshAllBalances">
            <el-icon><Wallet /></el-icon>
          </el-button>
        </el-tooltip>
        <el-tooltip content="同步全部账单" placement="bottom">
          <el-button class="ops-icon-button" type="primary" :loading="syncing" aria-label="同步全部账单" @click="syncAll">
            <el-icon><Refresh /></el-icon>
          </el-button>
        </el-tooltip>
      </div>
    </header>

    <el-alert
      v-if="errorMessage"
      class="page-alert"
      type="error"
      :title="errorMessage"
      show-icon
      :closable="false"
    />

    <template v-if="loading">
      <section class="metric-grid loading-band">
        <div v-for="item in 5" :key="item" class="metric-card"><el-skeleton :rows="2" animated /></div>
      </section>
      <section class="ops-card loading-panel"><el-skeleton :rows="9" animated /></section>
    </template>

    <template v-else>
      <section class="metric-grid" aria-label="费用与余额汇总">
        <article class="metric-card metric-card--primary">
          <span>本月费用</span>
          <strong>{{ formatCurrency(overview.totalCost, overview.currency) }}</strong>
          <small>{{ overview.accountCount }} 个云账号</small>
        </article>
        <article class="metric-card metric-card--balance">
          <span>当前余额</span>
          <template v-if="overview.balanceTotals.length">
            <strong v-for="item in overview.balanceTotals" :key="item.currency">
              {{ formatCurrency(item.balance, item.currency) }}
            </strong>
          </template>
          <strong v-else>{{ formatCurrency(0, overview.currency) }}</strong>
          <small>按币种独立汇总</small>
        </article>
        <article class="metric-card metric-card--comparison">
          <span>费用环比</span>
          <strong :class="rateClass(overview.momRate)">{{ formatRate(overview.momRate) }}</strong>
          <small>{{ overview.previousMonth }} {{ formatCurrency(overview.previousTotalCost, overview.currency) }}</small>
        </article>
        <article class="metric-card metric-card--comparison">
          <span>费用同比</span>
          <strong :class="rateClass(overview.yoyRate)">{{ formatRate(overview.yoyRate) }}</strong>
          <small>{{ overview.lastYearMonth }} {{ formatCurrency(overview.lastYearTotalCost, overview.currency) }}</small>
        </article>
        <article class="metric-card metric-card--health">
          <span>余额健康</span>
          <strong :class="{ 'has-risk': balanceRiskCount > 0 }">{{ balanceRiskCount }}</strong>
          <small>{{ overview.lowBalanceCount }} 个低余额，{{ overview.balanceErrorCount }} 个查询失败</small>
        </article>
      </section>

      <section class="ops-card provider-panel">
        <header class="ops-card__header">
          <div>
            <h2>云厂商费用</h2>
            <p>{{ overview.providers.length }} 个厂商，点击厂商查看账单明细</p>
          </div>
          <el-button type="primary" @click="goCenter">
            <el-icon><Setting /></el-icon>
            账号管理
          </el-button>
        </header>

        <div v-if="overview.providers.length" class="provider-list">
          <div class="provider-list__head" aria-hidden="true">
            <span>云厂商</span>
            <span>账号范围</span>
            <span>当前余额</span>
            <span>本月费用</span>
            <span>环比 / 同比</span>
            <span>同步状态</span>
            <span></span>
          </div>
          <button
            v-for="provider in overview.providers"
            :key="provider.providerCode"
            type="button"
            class="provider-row"
            @click="openProvider(provider.providerCode)"
          >
            <span class="provider-name-cell">
              <span class="provider-mark" :class="provider.providerCode">
                <img
                  :src="providerIcon(provider.providerCode)"
                  :alt="`${provider.providerName || providerName(provider.providerCode)}图标`"
                />
              </span>
              <span class="provider-identity">
                <b>{{ provider.providerName || providerName(provider.providerCode) }}</b>
                <small>{{ provider.accountCount }} 个账号</small>
              </span>
            </span>
            <span class="provider-accounts" :title="provider.accountNames.join('、')">
              {{ provider.accountNames.join('、') || '未命名账号' }}
            </span>
            <span class="provider-value">
              <b>{{ formatProviderBalances(provider) }}</b>
            </span>
            <span class="provider-value">
              <b>{{ formatCurrency(provider.currentMonthCost, provider.currency) }}</b>
            </span>
            <span class="provider-value">
              <b>
                <i :class="rateClass(provider.momRate)">{{ formatRate(provider.momRate) }}</i>
                <i :class="rateClass(provider.yoyRate)">{{ formatRate(provider.yoyRate) }}</i>
              </b>
            </span>
            <span class="provider-state">
              <el-tag :type="syncStatusType(provider.syncStatus)" effect="light" size="small">
                {{ syncStatusLabel(provider.syncStatus) }}
              </el-tag>
              <small v-if="provider.lowBalanceCount || provider.balanceErrorCount" class="risk-text">
                {{ provider.lowBalanceCount + provider.balanceErrorCount }} 个余额异常
              </small>
              <small v-else>{{ provider.lastSyncTime || '尚未同步' }}</small>
            </span>
            <el-icon class="row-arrow"><ArrowRight /></el-icon>
          </button>
        </div>

        <el-empty v-else description="暂无云账号">
          <el-button type="primary" @click="goCenter">添加云账号</el-button>
        </el-empty>
      </section>
    </template>
    </div>
  </main>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowRight, DataAnalysis, Refresh, Setting, Wallet } from '@element-plus/icons-vue'
import dayjs from 'dayjs'
import billApi from '@/api/bill/bill.js'
import accountApi from '@/api/bill/datasource.js'
import {
  formatCurrency,
  formatRate,
  providerIcon,
  providerName,
  rateClass,
  requestError,
  responseData,
  syncStatusLabel,
  syncStatusType
} from './billing-format.js'

const router = useRouter()
const selectedMonth = ref(dayjs().format('YYYY-MM'))
const loading = ref(true)
const syncing = ref(false)
const refreshing = ref(false)
const errorMessage = ref('')
const overview = reactive({
  month: selectedMonth.value,
  previousMonth: '',
  lastYearMonth: '',
  totalCost: 0,
  previousTotalCost: 0,
  lastYearTotalCost: 0,
  momRate: null,
  yoyRate: null,
  currency: 'CNY',
  balanceTotals: [],
  accountCount: 0,
  lowBalanceCount: 0,
  balanceErrorCount: 0,
  providers: []
})

const balanceRiskCount = computed(() => overview.lowBalanceCount + overview.balanceErrorCount)

const loadOverview = async () => {
  loading.value = true
  errorMessage.value = ''
  try {
    const data = responseData(await billApi.getOverview({ month: selectedMonth.value })) || {}
    Object.assign(overview, data)
    overview.balanceTotals = Array.isArray(data.balanceTotals) ? data.balanceTotals : []
    overview.providers = Array.isArray(data.providers) ? data.providers : []
  } catch (error) {
    errorMessage.value = requestError(error, '获取账单总览失败')
    overview.providers = []
  } finally {
    loading.value = false
  }
}

const runAll = async (type) => {
  const isBalance = type === 'balance'
  const flag = isBalance ? refreshing : syncing
  flag.value = true
  try {
    const response = isBalance
      ? await accountApi.refreshAllBalances()
      : await accountApi.syncAll()
    const accepted = responseData(response)?.accepted || 0
    ElMessage.success(`已创建 ${accepted} 个${isBalance ? '余额刷新' : '账单同步'}任务`)
    window.setTimeout(loadOverview, 1200)
  } catch (error) {
    ElMessage.error(requestError(error, isBalance ? '刷新余额失败' : '同步账单失败'))
  } finally {
    flag.value = false
  }
}

const syncAll = () => runAll('bill')
const refreshAllBalances = () => runAll('balance')
const goCenter = () => router.push('/cost/cloud_bill')
const openProvider = operator => router.push({ path: '/cost/cloud_view', query: { operator, month: selectedMonth.value } })

const formatProviderBalances = provider => {
  const totals = Array.isArray(provider.balanceTotals) ? provider.balanceTotals : []
  if (!totals.length) return formatCurrency(provider.availableBalance, provider.currency)
  return totals.map(item => formatCurrency(item.balance, item.currency)).join(' / ')
}

onMounted(loadOverview)
</script>

<style scoped>
.billing-workspace {
  --ops-text-heading: #0f172a;
  --ops-text-primary: #172033;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-color-primary: #2563eb;
  --ops-color-success: #10b981;
  --ops-color-danger: #ef4444;
  --ops-bg-surface: #fff;
  --ops-bg-surface-soft: #fbfdff;
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

:global(.home-container .el-main:has(.overview-page)) {
  padding-top: 10px;
  padding-right: 10px;
  padding-left: 10px;
}

.billing-workbench {
  display: grid;
  gap: 8px;
  min-width: 0;
}

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
.ops-page-header__actions {
  display: flex;
  align-items: center;
}

.ops-page-header__content {
  gap: 10px;
  min-width: 0;
}

.ops-page-header h1 {
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

.ops-page-header p {
  margin: 0;
  overflow: hidden;
  color: var(--ops-text-muted);
  font-size: 12px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ops-page-header__actions {
  gap: 8px;
  margin-left: auto;
}

.ops-page-header__actions :deep(.el-date-editor) {
  width: 132px;
}

.ops-icon-button {
  width: 34px;
  height: 34px;
  padding: 0;
}

.ops-refresh-button {
  border-color: #f3c66b;
  background: #f3c66b;
}

.page-alert {
  margin: 0;
  border-radius: 8px;
}

.metric-grid {
  display: grid;
  grid-template-columns: 1.25fr 1.25fr repeat(3, minmax(0, 1fr));
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

.metric-card::before {
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 3px;
  background: #60a5fa;
  content: "";
}

.metric-card--balance::before { background: #0ea5e9; }
.metric-card--comparison::before { background: #6366f1; }
.metric-card--health::before { background: #10b981; }

.metric-card > span,
.metric-card > small,
.provider-state small {
  display: block;
}

.metric-card > span {
  color: var(--ops-text-secondary);
  font-size: 11px;
  font-weight: 700;
}

.metric-card strong {
  display: block;
  margin-top: 9px;
  overflow-wrap: anywhere;
  color: var(--ops-text-heading);
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 20px;
  font-weight: 800;
  line-height: 1.05;
  font-variant-numeric: tabular-nums;
}

.metric-card--primary strong { color: var(--ops-color-primary); font-size: 24px; }
.metric-card--balance strong { margin-top: 6px; font-size: 17px; }
.metric-card > small { margin-top: 7px; color: #7b8aa0; font-size: 11px; }
.metric-card--health .has-risk,
.risk-text { color: #dc2626 !important; }
.loading-band .metric-card { padding-top: 15px; }

.ops-card {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.97);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
}

.loading-panel { min-height: 270px; padding: 12px; }

.ops-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--ops-border-soft);
}

.ops-card__header h2 {
  margin: 0;
  color: var(--ops-text-heading);
  font-size: 15px;
  font-weight: 700;
  line-height: 1.25;
}

.ops-card__header p {
  margin: 3px 0 0;
  color: var(--ops-text-muted);
  font-size: 11px;
  line-height: 1.4;
}

.provider-list { width: 100%; overflow-x: auto; }

.provider-list__head,
.provider-row {
  display: grid;
  grid-template-columns: minmax(150px, 1.05fr) minmax(140px, 1.15fr) minmax(145px, 1fr) minmax(135px, 0.95fr) minmax(145px, 0.95fr) minmax(135px, 0.9fr) 18px;
  gap: 12px;
  align-items: center;
  min-width: 940px;
}

.provider-list__head {
  padding: 7px 12px;
  color: var(--ops-text-secondary);
  background: #f3f7fc;
  font-size: 12px;
  font-weight: 700;
}

.provider-list__head span:nth-child(n+3):nth-child(-n+5) { text-align: right; }

.provider-row {
  width: 100%;
  min-height: 54px;
  padding: 7px 12px;
  border: 0;
  border-bottom: 1px solid #eef3f9;
  color: inherit;
  text-align: left;
  background: transparent;
  cursor: pointer;
  transition: background-color 160ms ease, box-shadow 160ms ease;
}

.provider-row:last-child { border-bottom: 0; }
.provider-row:hover { background: #f8fafc; }
.provider-row:active { background: #f3f7fc; }
.provider-row:focus-visible { outline: 2px solid #409eff; outline-offset: -2px; }

.provider-name-cell,
.provider-identity,
.provider-value,
.provider-state {
  min-width: 0;
}

.provider-name-cell { display: flex; align-items: center; gap: 9px; }

.provider-mark {
  display: grid;
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  place-items: center;
  overflow: hidden;
  border: 1px solid #dbe7f4;
  border-radius: 8px;
  background: #f8fafc;
}

.provider-mark img { width: 22px; height: 22px; object-fit: contain; }
.provider-mark.aliyun { border-color: #fed7aa; background: #fff7ed; }
.provider-mark.tencentcloud { border-color: #bae6fd; background: #f0f9ff; }
.provider-mark.huawei { border-color: #fecdd3; background: #fff1f2; }
.provider-mark.volcengine { border-color: #bfdbfe; background: #eff6ff; }

.provider-identity b,
.provider-value b {
  display: block;
  overflow: hidden;
  color: var(--ops-text-primary);
  font-size: 12px;
  font-style: normal;
  font-variant-numeric: tabular-nums;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.provider-value { text-align: right; }
.provider-identity small,
.provider-accounts { color: var(--ops-text-muted); font-size: 11px; }
.provider-identity small { display: block; margin-top: 3px; }
.provider-accounts { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.provider-value i { margin-left: 8px; font-size: 11px; font-style: normal; }
.provider-state small { margin-top: 4px; overflow: hidden; color: var(--ops-text-muted); font-size: 10.5px; text-overflow: ellipsis; white-space: nowrap; }
.row-arrow { color: #7b8aa0; }
.is-increase { color: #dc2626 !important; }
.is-decrease { color: #059669 !important; }
.is-neutral { color: var(--ops-text-muted) !important; }

.overview-page :deep(.el-button) {
  min-height: 34px;
  border-radius: 8px;
  font-weight: 600;
  transition: color 160ms ease, background-color 160ms ease, border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease;
}

.overview-page :deep(.el-button:hover) { transform: translateY(-1px); }
.overview-page :deep(.el-button:active) { transform: translateY(0); }
.overview-page :deep(.el-input__wrapper) { min-height: 34px; border-radius: 8px; background: #fbfdff; box-shadow: 0 0 0 1px #d9e5f2 inset; }

@media (max-width: 1200px) {
  .metric-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); }
}

@media (max-width: 960px) {
  .ops-page-header { align-items: flex-start; flex-wrap: wrap; }
  .ops-page-header__actions { width: 100%; margin-left: 54px; }
  .metric-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (max-width: 620px) {
  .billing-workspace { padding: 6px; }
  .ops-page-header__icon { width: 36px; height: 36px; flex-basis: 36px; font-size: 26px; }
  .ops-page-header__content { align-items: flex-start; flex: 1; flex-direction: column; gap: 2px; }
  .ops-page-header__divider { display: none; }
  .ops-page-header p { width: 100%; white-space: normal; }
  .ops-page-header__actions { margin-left: 0; }
  .ops-page-header__actions :deep(.el-date-editor) { flex: 1; min-width: 0; }
  .metric-grid { grid-template-columns: 1fr; }
  .ops-card__header { align-items: flex-start; }
}

@media (prefers-reduced-motion: reduce) {
  .overview-page *,
  .overview-page *::before,
  .overview-page *::after {
    transition-duration: 0.01ms !important;
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
  }
}
</style>
