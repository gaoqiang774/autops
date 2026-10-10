<template>
  <main class="billing-workspace center-page">
    <div class="billing-workbench">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <el-icon><Wallet /></el-icon>
      </span>
      <div class="ops-page-header__content">
        <h1>账单中心</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>统一管理云账号、账单采集、账户余额与余额告警</p>
      </div>
      <div class="ops-page-header__actions">
        <el-button class="ops-refresh-button" type="warning" size="small" :loading="refreshingAll" @click="runAll('balance')">
          <el-icon><Wallet /></el-icon>
          刷新全部余额
        </el-button>
        <el-button type="primary" size="small" :loading="syncingAll" @click="runAll('bill')">
          <el-icon><Refresh /></el-icon>
          同步全部账单
        </el-button>
        <el-button type="success" size="small" @click="openAccountDialog()">
          <el-icon><Plus /></el-icon>
          添加云账号
        </el-button>
      </div>
    </header>

    <el-alert v-if="errorMessage" class="page-alert" type="error" :title="errorMessage" show-icon :closable="false" />

    <section class="metric-grid" aria-label="余额管理概览">
      <article class="metric-card metric-card--primary">
        <span>当前余额</span>
        <template v-if="balanceOverview.currencyTotals.length">
          <strong v-for="item in balanceOverview.currencyTotals" :key="item.currency">
            {{ formatCurrency(item.balance, item.currency) }}
          </strong>
        </template>
        <strong v-else>{{ formatCurrency(0) }}</strong>
        <small>最近刷新 {{ balanceOverview.lastBalanceRefresh || '暂无记录' }}</small>
      </article>
      <article class="metric-card metric-card--account">
        <span>接入账号</span>
        <strong>{{ balanceOverview.accountCount }}</strong>
        <small>{{ balanceOverview.monitoringCount }} 个已开启余额采集</small>
      </article>
      <article class="metric-card metric-card--alert">
        <span>余额告警</span>
        <strong>{{ balanceOverview.alertEnabledCount }}</strong>
        <small>已启用规则</small>
      </article>
      <article class="metric-card metric-card--risk">
        <span>余额异常</span>
        <strong>{{ balanceOverview.lowBalanceCount + balanceOverview.queryErrorCount }}</strong>
        <small>{{ balanceOverview.lowBalanceCount }} 个低余额，{{ balanceOverview.queryErrorCount }} 个查询失败</small>
      </article>
    </section>

    <section class="ops-card account-panel">
          <header class="ops-card__header">
            <div>
              <h2>云账号与采集配置</h2>
              <p>共 {{ accountTotal }} 个账号，可独立控制账单与余额采集</p>
            </div>
          </header>
          <div class="toolbar">
            <div class="filters">
              <el-select v-model="accountQuery.operator" clearable placeholder="全部云厂商" @change="resetAccountPage">
                <el-option v-for="provider in BILLING_PROVIDER_OPTIONS" :key="provider.keyType" :label="provider.label" :value="provider.code" />
              </el-select>
              <el-select v-model="accountQuery.enabled" clearable placeholder="全部状态" @change="resetAccountPage">
                <el-option label="已启用" :value="1" />
                <el-option label="已停用" :value="0" />
              </el-select>
              <el-select v-model="accountQuery.syncStatus" clearable placeholder="全部同步状态" @change="resetAccountPage">
                <el-option label="同步成功" value="success" />
                <el-option label="部分成功" value="partial_success" />
                <el-option label="同步中" value="syncing" />
                <el-option label="同步失败" value="failed" />
                <el-option label="待同步" value="pending" />
              </el-select>
            </div>
            <el-tooltip content="刷新列表" placement="bottom">
              <el-button class="ops-refresh-button toolbar-icon-button" type="warning" aria-label="刷新列表" @click="loadAccounts"><el-icon><Refresh /></el-icon></el-button>
            </el-tooltip>
          </div>

          <el-table v-loading="accountLoading" :data="accounts" class="account-table" row-key="id">
            <el-table-column label="云账号" min-width="210">
              <template #default="{ row }">
                <div class="account-cell">
                  <span class="provider-mark" :class="row.operator">
                    <img :src="providerIcon(row.operator)" :alt="`${row.providerName || providerName(row.operator)}图标`" />
                  </span>
                  <div>
                    <b>{{ row.accountName || row.accountId || `账号 #${row.id}` }}</b>
                    <small>{{ row.providerName }} / {{ row.keyName || `密钥 #${row.keyManageId}` }}</small>
                  </div>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="当前余额" min-width="150" align="right">
              <template #default="{ row }">
                <div class="money-cell">
                  <b>{{ formatCurrency(row.availableBalance, row.currency) }}</b>
                  <el-tag :type="balanceStatusType(row.balanceStatus)" effect="light" size="small">
                    {{ balanceStatusLabel(row.balanceStatus) }}
                  </el-tag>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="预计可用" width="105" align="right">
              <template #default="{ row }">
                {{ row.estimatedAvailableDays === null ? '-' : `${Number(row.estimatedAvailableDays).toFixed(1)} 天` }}
              </template>
            </el-table-column>
            <el-table-column label="本月费用" min-width="125" align="right">
              <template #default="{ row }"><b>{{ formatCurrency(row.currentMonthCost, row.currency) }}</b></template>
            </el-table-column>
            <el-table-column label="同步状态" min-width="145">
              <template #default="{ row }">
                <div class="state-cell">
                  <el-tag :type="syncStatusType(row.syncStatus)" effect="light" size="small">
                    {{ syncStatusLabel(row.syncStatus) }}
                  </el-tag>
                  <small>{{ row.lastSyncTime || '尚未同步' }}</small>
                  <el-tooltip v-if="row.lastSyncError" :content="row.lastSyncError" placement="top" :show-after="300">
                    <small class="sync-error">{{ row.lastSyncError }}</small>
                  </el-tooltip>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="采集开关" min-width="185">
              <template #default="{ row }">
                <div class="switch-cell">
                  <label>账单 <el-switch v-model="row.billSyncEnabled" :active-value="1" :inactive-value="0" @change="updateSwitch(row, 'billSyncEnabled')" /></label>
                  <label>余额 <el-switch v-model="row.balanceSyncEnabled" :active-value="1" :inactive-value="0" @change="updateSwitch(row, 'balanceSyncEnabled')" /></label>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="余额告警" width="110" align="center">
              <template #default="{ row }">
                <el-button link :type="row.balanceAlertEnabled ? 'danger' : 'primary'" @click="openAlertDialog(row)">
                  {{ row.balanceAlertEnabled ? formatAlertThreshold(row) : '未配置' }}
                </el-button>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="222" fixed="right" align="center">
              <template #default="{ row }">
                <div class="row-actions">
                  <el-tooltip content="同步账单" placement="top"><el-button circle aria-label="同步账单" :loading="rowActionKey === `bill-${row.id}`" @click="runAccountTask(row, 'bill')"><el-icon><Refresh /></el-icon></el-button></el-tooltip>
                  <el-tooltip content="刷新余额" placement="top"><el-button circle aria-label="刷新余额" :loading="rowActionKey === `balance-${row.id}`" @click="runAccountTask(row, 'balance')"><el-icon><Wallet /></el-icon></el-button></el-tooltip>
                  <el-tooltip content="余额告警" placement="top"><el-button circle aria-label="余额告警" @click="openAlertDialog(row)"><el-icon><Bell /></el-icon></el-button></el-tooltip>
                  <el-tooltip content="编辑账号" placement="top"><el-button circle aria-label="编辑账号" @click="openAccountDialog(row)"><el-icon><Edit /></el-icon></el-button></el-tooltip>
                  <el-tooltip content="删除账号" placement="top"><el-button circle type="danger" plain aria-label="删除账号" @click="removeAccount(row)"><el-icon><Delete /></el-icon></el-button></el-tooltip>
                </div>
              </template>
            </el-table-column>
          </el-table>

          <div class="pagination-wrap">
            <el-pagination
              v-model:current-page="accountQuery.page"
              v-model:page-size="accountQuery.pageSize"
              :page-sizes="[10, 20, 50]"
              :total="accountTotal"
              layout="total, sizes, prev, pager, next"
              @size-change="loadAccounts"
              @current-change="loadAccounts"
            />
          </div>
    </section>

    <el-dialog v-model="accountDialogVisible" :title="editingAccountId ? '编辑云账号' : '添加云账号'" width="680px" class="ops-billing-dialog" modal-class="billing-dialog-overlay" destroy-on-close>
      <el-form ref="accountFormRef" :model="accountForm" :rules="accountRules" label-position="top" class="ops-billing-form">
        <div class="form-grid">
          <el-form-item label="配置中心密钥" prop="keyManageId" class="form-span-2">
            <el-select v-model="accountForm.keyManageId" filterable placeholder="选择云厂商密钥">
              <el-option v-for="key in availableKeys" :key="key.id" :label="`${providerNameByKey(key.keyType)} / ${key.keyName}`" :value="key.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="账号名称" prop="accountName"><el-input v-model="accountForm.accountName" maxlength="128" /></el-form-item>
          <el-form-item label="云厂商账号 ID"><el-input v-model="accountForm.accountId" maxlength="128" /></el-form-item>
          <el-form-item label="账单同步周期">
            <el-select v-model="accountForm.syncCycle"><el-option label="每天" value="daily" /><el-option label="每周" value="weekly" /><el-option label="每月" value="monthly" /></el-select>
          </el-form-item>
          <el-form-item label="首次回溯月份"><el-input-number v-model="accountForm.lookbackMonths" :min="1" :max="12" controls-position="right" /></el-form-item>
          <el-form-item label="余额刷新间隔">
            <el-select v-model="accountForm.balanceSyncIntervalMinutes"><el-option label="15 分钟" :value="15" /><el-option label="30 分钟" :value="30" /><el-option label="1 小时" :value="60" /><el-option label="6 小时" :value="360" /><el-option label="24 小时" :value="1440" /></el-select>
          </el-form-item>
          <el-form-item label="账号状态"><el-switch v-model="accountForm.enabled" :active-value="1" :inactive-value="0" active-text="启用" inactive-text="停用" /></el-form-item>
          <el-form-item label="采集任务" class="form-span-2">
            <el-checkbox v-model="billSyncChecked">同步账单</el-checkbox>
            <el-checkbox v-model="balanceSyncChecked">刷新余额</el-checkbox>
          </el-form-item>
        </div>
      </el-form>
      <template #footer><div class="dialog-footer"><el-button @click="accountDialogVisible = false">取消</el-button><el-button type="primary" :loading="accountSaving" @click="saveAccount">保存</el-button></div></template>
    </el-dialog>

    <el-dialog v-model="alertDialogVisible" title="余额告警" width="680px" class="ops-billing-dialog" modal-class="billing-dialog-overlay" destroy-on-close>
      <div class="alert-account"><b>{{ alertAccount.accountName }}</b><span>{{ alertAccount.providerName }} / 当前余额 {{ formatCurrency(alertAccount.availableBalance, alertAccount.currency) }}</span></div>
      <el-form ref="alertFormRef" :model="alertForm" :rules="alertRules" label-position="top" class="ops-billing-form">
        <div class="form-grid">
          <el-form-item label="告警状态"><el-switch v-model="alertForm.enabled" :active-value="1" :inactive-value="0" active-text="启用" inactive-text="停用" /></el-form-item>
          <el-form-item label="告警级别"><el-select v-model="alertForm.severity"><el-option label="警告" value="warning" /><el-option label="严重" value="critical" /></el-select></el-form-item>
          <el-form-item label="余额低于" prop="thresholdAmount"><el-input-number v-model="alertForm.thresholdAmount" :min="0" :precision="2" controls-position="right" /></el-form-item>
          <el-form-item label="预计可用天数低于" prop="thresholdDays"><el-input-number v-model="alertForm.thresholdDays" :min="0" :max="365" controls-position="right" /></el-form-item>
          <el-form-item label="重复通知间隔"><el-select v-model="alertForm.repeatIntervalMinutes"><el-option label="30 分钟" :value="30" /><el-option label="2 小时" :value="120" /><el-option label="12 小时" :value="720" /><el-option label="24 小时" :value="1440" /><el-option label="7 天" :value="10080" /></el-select></el-form-item>
          <el-form-item label="异常处理"><el-checkbox v-model="queryErrorChecked">余额查询失败时告警</el-checkbox><el-checkbox v-model="recoveredChecked">恢复后发送通知</el-checkbox></el-form-item>
          <el-form-item label="通知策略" class="form-span-2">
            <el-select v-model="alertForm.notifyRuleIds" multiple filterable clearable collapse-tags collapse-tags-tooltip placeholder="选择告警中心通知策略">
              <el-option v-for="rule in notifyRules" :key="rule.id" :label="rule.name || `策略 #${rule.id}`" :value="rule.id" />
            </el-select>
          </el-form-item>
        </div>
      </el-form>
      <template #footer><div class="dialog-footer"><el-button @click="alertDialogVisible = false">取消</el-button><el-button type="primary" :loading="alertSaving" @click="saveAlert">保存规则</el-button></div></template>
    </el-dialog>
    </div>
  </main>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Bell, Delete, Edit, Plus, Refresh, Wallet } from '@element-plus/icons-vue'
import accountApi from '@/api/bill/datasource.js'
import alertApi from '@/api/alertmanager.js'
import {
  BILLING_KEY_TYPES,
  BILLING_PROVIDER_OPTIONS,
  balanceStatusLabel,
  balanceStatusType,
  formatCurrency,
  providerIcon,
  providerName,
  requestError,
  responseData,
  responsePage,
  syncStatusLabel,
  syncStatusType
} from './billing-format.js'

const errorMessage = ref('')
const accountLoading = ref(false)
const syncingAll = ref(false)
const refreshingAll = ref(false)
const rowActionKey = ref('')
const accounts = ref([])
const accountTotal = ref(0)
const keys = ref([])
const notifyRules = ref([])
const balanceOverview = reactive({ accountCount: 0, monitoringCount: 0, alertEnabledCount: 0, lowBalanceCount: 0, queryErrorCount: 0, currencyTotals: [], lastBalanceRefresh: '' })
const accountQuery = reactive({ operator: '', enabled: '', syncStatus: '', page: 1, pageSize: 10 })

const accountDialogVisible = ref(false)
const editingAccountId = ref(0)
const accountSaving = ref(false)
const accountFormRef = ref()
const accountForm = reactive({ keyManageId: '', accountName: '', accountId: '', enabled: 1, billSyncEnabled: 1, balanceSyncEnabled: 1, balanceSyncIntervalMinutes: 60, syncCycle: 'daily', lookbackMonths: 12 })
const accountRules = { keyManageId: [{ required: true, message: '请选择配置中心密钥', trigger: 'change' }], accountName: [{ required: true, message: '请输入账号名称', trigger: 'blur' }] }
const billSyncChecked = computed({ get: () => accountForm.billSyncEnabled === 1, set: value => { accountForm.billSyncEnabled = value ? 1 : 0 } })
const balanceSyncChecked = computed({ get: () => accountForm.balanceSyncEnabled === 1, set: value => { accountForm.balanceSyncEnabled = value ? 1 : 0 } })

const alertDialogVisible = ref(false)
const alertSaving = ref(false)
const alertFormRef = ref()
const alertAccount = ref({})
const alertForm = reactive({ enabled: 0, thresholdAmount: 0, thresholdDays: 0, alertOnQueryError: 1, severity: 'warning', repeatIntervalMinutes: 1440, notifyRecovered: 1, notifyRuleIds: [] })
const alertRules = { thresholdAmount: [{ type: 'number', min: 0, message: '阈值不能小于 0', trigger: 'change' }], thresholdDays: [{ type: 'number', min: 0, max: 365, message: '请输入 0 到 365 天', trigger: 'change' }] }
const queryErrorChecked = computed({ get: () => alertForm.alertOnQueryError === 1, set: value => { alertForm.alertOnQueryError = value ? 1 : 0 } })
const recoveredChecked = computed({ get: () => alertForm.notifyRecovered === 1, set: value => { alertForm.notifyRecovered = value ? 1 : 0 } })
const availableKeys = computed(() => keys.value.filter(key => BILLING_KEY_TYPES.includes(Number(key.keyType))))

const compactParams = source => Object.fromEntries(Object.entries(source).filter(([, value]) => value !== '' && value !== null && value !== undefined))

const loadAccounts = async () => {
  accountLoading.value = true
  try {
    const page = responsePage(await accountApi.getAccounts(compactParams(accountQuery)))
    accounts.value = page.list
    accountTotal.value = page.total
  } catch (error) {
    errorMessage.value = requestError(error, '获取云账号失败')
  } finally {
    accountLoading.value = false
  }
}

const loadBalanceOverview = async () => {
  try {
    const data = responseData(await accountApi.getBalanceOverview()) || {}
    Object.assign(balanceOverview, data)
    balanceOverview.currencyTotals = Array.isArray(data.currencyTotals) ? data.currencyTotals : []
  } catch (error) {
    errorMessage.value = requestError(error, '获取余额概览失败')
  }
}

const loadKeys = async () => {
  try { keys.value = responsePage(await accountApi.getKeyList({ page: 1, pageSize: 500 })).list } catch (_) { keys.value = [] }
}

const loadNotifyRules = async () => {
  try { notifyRules.value = responsePage(await alertApi.getNotifyRules({ page: 1, pageSize: 500, enabled: 1 })).list } catch (_) { notifyRules.value = [] }
}

const resetAccountPage = () => { accountQuery.page = 1; loadAccounts() }

const resetAccountForm = () => Object.assign(accountForm, { keyManageId: '', accountName: '', accountId: '', enabled: 1, billSyncEnabled: 1, balanceSyncEnabled: 1, balanceSyncIntervalMinutes: 60, syncCycle: 'daily', lookbackMonths: 12 })
const openAccountDialog = (row = null) => {
  editingAccountId.value = row?.id || 0
  resetAccountForm()
  if (row) Object.assign(accountForm, { keyManageId: row.keyManageId, accountName: row.accountName, accountId: row.accountId, enabled: row.enabled, billSyncEnabled: row.billSyncEnabled, balanceSyncEnabled: row.balanceSyncEnabled, balanceSyncIntervalMinutes: row.balanceSyncIntervalMinutes, syncCycle: row.syncCycle, lookbackMonths: row.lookbackMonths })
  accountDialogVisible.value = true
}

const saveAccount = async () => {
  if (!await accountFormRef.value?.validate().catch(() => false)) return
  accountSaving.value = true
  try {
    const payload = { ...accountForm }
    if (editingAccountId.value) await accountApi.updateAccount(editingAccountId.value, payload)
    else await accountApi.createAccount(payload)
    ElMessage.success(editingAccountId.value ? '云账号已更新' : '云账号已添加')
    accountDialogVisible.value = false
    await Promise.all([loadAccounts(), loadBalanceOverview(), loadKeys()])
  } catch (error) {
    ElMessage.error(requestError(error, '保存云账号失败'))
  } finally { accountSaving.value = false }
}

const updateSwitch = async (row, field) => {
  try {
    await accountApi.updateAccount(row.id, { [field]: row[field] })
    ElMessage.success('采集设置已更新')
    loadBalanceOverview()
  } catch (error) {
    row[field] = row[field] === 1 ? 0 : 1
    ElMessage.error(requestError(error, '更新采集设置失败'))
  }
}

const removeAccount = async row => {
  try {
    await ElMessageBox.confirm(`确认删除云账号“${row.accountName}”？`, '删除云账号', { type: 'warning', confirmButtonText: '删除', cancelButtonText: '取消' })
    await accountApi.deleteAccount(row.id)
    ElMessage.success('云账号已删除')
    await Promise.all([loadAccounts(), loadBalanceOverview(), loadKeys()])
  } catch (error) { if (error !== 'cancel' && error !== 'close') ElMessage.error(requestError(error, '删除云账号失败')) }
}

const runAccountTask = async (row, type) => {
  rowActionKey.value = `${type}-${row.id}`
  try {
		await (type === 'bill' ? accountApi.syncBills(row.id) : accountApi.refreshBalance(row.id))
		ElMessage.success(type === 'bill' ? '账单同步已受理' : '余额刷新已受理')
    window.setTimeout(() => Promise.all([loadAccounts(), loadBalanceOverview()]), 1200)
  } catch (error) { ElMessage.error(requestError(error, '创建任务失败')) } finally { rowActionKey.value = '' }
}

const runAll = async type => {
  const flag = type === 'bill' ? syncingAll : refreshingAll
  flag.value = true
  try {
    const response = type === 'bill' ? await accountApi.syncAll() : await accountApi.refreshAllBalances()
    ElMessage.success(`已创建 ${responseData(response)?.accepted || 0} 个任务`)
    window.setTimeout(() => Promise.all([loadAccounts(), loadBalanceOverview()]), 1200)
  } catch (error) { ElMessage.error(requestError(error, '批量任务创建失败')) } finally { flag.value = false }
}

const openAlertDialog = async row => {
  alertAccount.value = row
  alertDialogVisible.value = true
  try {
    const data = responseData(await accountApi.getBalanceAlert(row.id)) || {}
    Object.assign(alertForm, { enabled: 0, thresholdAmount: 0, thresholdDays: 0, alertOnQueryError: 1, severity: 'warning', repeatIntervalMinutes: 1440, notifyRecovered: 1, notifyRuleIds: [], ...data })
  } catch (error) { ElMessage.error(requestError(error, '获取余额告警规则失败')) }
}

const saveAlert = async () => {
  if (!await alertFormRef.value?.validate().catch(() => false)) return
  if (alertForm.enabled === 1 && !alertForm.thresholdAmount && !alertForm.thresholdDays && !alertForm.alertOnQueryError) {
    ElMessage.warning('开启告警时至少配置一种告警条件')
    return
  }
  alertSaving.value = true
  try {
    await accountApi.saveBalanceAlert(alertAccount.value.id, { ...alertForm })
    ElMessage.success('余额告警规则已保存')
    alertDialogVisible.value = false
    await Promise.all([loadAccounts(), loadBalanceOverview()])
  } catch (error) { ElMessage.error(requestError(error, '保存余额告警规则失败')) } finally { alertSaving.value = false }
}

const formatAlertThreshold = row => row.balanceAlertThreshold > 0 ? `< ${formatCurrency(row.balanceAlertThreshold, row.currency)}` : '已启用'
const providerNameByKey = type => BILLING_PROVIDER_OPTIONS.find(item => item.keyType === Number(type))?.label || '其他'

onMounted(async () => {
  errorMessage.value = ''
  await Promise.all([loadAccounts(), loadBalanceOverview(), loadKeys(), loadNotifyRules()])
})
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
  --ops-color-danger: #ef4444;
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

:global(.home-container .el-main:has(.center-page)) {
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
.toolbar,
.filters,
.account-cell,
.row-actions {
  display: flex;
  align-items: center;
}

.ops-page-header__content { gap: 10px; min-width: 0; }
.ops-page-header h1 { margin: 0; color: var(--ops-text-heading); font-size: 20px; font-weight: 700; line-height: 1.2; white-space: nowrap; }
.ops-page-header__divider { width: 1px; height: 22px; flex: 0 0 1px; background: var(--ops-border-soft); }
.ops-page-header p { margin: 0; overflow: hidden; color: var(--ops-text-muted); font-size: 12px; line-height: 1.5; text-overflow: ellipsis; white-space: nowrap; }
.ops-page-header__actions { gap: 8px; margin-left: auto; }

.ops-refresh-button { color: #fff; border-color: #f3c66b; background: #f3c66b; }
.page-alert { margin: 0; border-radius: 8px; }

.metric-grid {
  display: grid;
  grid-template-columns: 1.35fr repeat(3, minmax(0, 1fr));
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
.metric-card--account::before { background: #0ea5e9; }
.metric-card--alert::before { background: #f59e0b; }
.metric-card--risk::before { background: #ef4444; }
.metric-card span,
.metric-card small { display: block; }
.metric-card span { color: var(--ops-text-secondary); font-size: 11px; font-weight: 700; }
.metric-card strong { display: block; margin-top: 9px; overflow-wrap: anywhere; color: var(--ops-text-heading); font-family: Consolas, "SFMono-Regular", monospace; font-size: 20px; font-weight: 800; line-height: 1.05; font-variant-numeric: tabular-nums; }
.metric-card--primary strong { margin-top: 6px; color: var(--ops-color-primary); font-size: 20px; }
.metric-card small { margin-top: 7px; color: #7b8aa0; font-size: 11px; }
.metric-card--risk strong { color: #dc2626; }

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

.toolbar { justify-content: space-between; gap: 12px; padding: 7px 10px; border-bottom: 1px solid var(--ops-border-soft); background: var(--ops-bg-subtle); }
.filters { gap: 8px; flex-wrap: wrap; }
.filters :deep(.el-select) { width: 150px; }
.toolbar-icon-button { width: 30px; height: 30px; min-height: 30px !important; padding: 0; }
.account-table { width: 100%; }
.account-cell { min-width: 0; gap: 9px; }
.account-cell > div { min-width: 0; }
.account-cell b,
.account-cell small,
.state-cell small { display: block; }
.account-cell b { overflow: hidden; color: var(--ops-text-primary); font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.account-cell small,
.state-cell small { margin-top: 3px; color: var(--ops-text-muted); font-size: 10.5px; }

.provider-mark { display: grid; width: 34px; height: 34px; flex: 0 0 34px; place-items: center; overflow: hidden; border: 1px solid #dbe7f4; border-radius: 8px; background: #f8fafc; }
.provider-mark img { width: 22px; height: 22px; object-fit: contain; }
.provider-mark.aliyun { border-color: #fed7aa; background: #fff7ed; }
.provider-mark.tencentcloud { border-color: #bae6fd; background: #f0f9ff; }
.provider-mark.huawei { border-color: #fecdd3; background: #fff1f2; }
.provider-mark.volcengine { border-color: #bfdbfe; background: #eff6ff; }
.sync-error { max-width: 132px; overflow: hidden; color: #dc2626 !important; text-overflow: ellipsis; white-space: nowrap; cursor: help; }
.money-cell b { display: block; margin-bottom: 4px; color: var(--ops-text-heading); font-family: Consolas, "SFMono-Regular", monospace; font-size: 12px; font-variant-numeric: tabular-nums; }
.switch-cell { display: flex; gap: 12px; }
.switch-cell label { display: flex; align-items: center; gap: 5px; color: var(--ops-text-secondary); font-size: 11px; white-space: nowrap; }
.row-actions { justify-content: center; gap: 4px; }
.row-actions :deep(.el-button + .el-button) { margin-left: 0; }
.row-actions :deep(.el-button) { width: 28px; height: 28px; min-height: 28px; padding: 0; border-radius: 8px; }
.pagination-wrap { display: flex; justify-content: flex-end; padding: 9px 10px; border-top: 1px solid var(--ops-border-soft); }

.form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0 14px; }
.form-span-2 { grid-column: 1 / -1; }
.form-grid :deep(.el-select),
.form-grid :deep(.el-input-number) { width: 100%; }
.alert-account { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; padding: 10px 13px; border: 1px solid #cfe2ff; border-radius: 10px; background: #f6faff; }
.alert-account span { color: var(--ops-text-muted); font-size: 12px; }
.dialog-footer { display: flex; justify-content: flex-end; gap: 10px; }

.center-page :deep(.el-button) { border-radius: 8px; font-weight: 600; transition: color 160ms ease, background-color 160ms ease, border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease; }
.center-page :deep(.el-button:hover) { transform: translateY(-1px); }
.center-page :deep(.el-button:active) { transform: translateY(0); }
.center-page :deep(.el-table) { --el-table-border-color: #eef3f9; color: var(--ops-text-primary); font-size: 12px; }
.center-page :deep(.el-table th.el-table__cell) { height: 36px; padding: 0; color: var(--ops-text-secondary); background: #f3f7fc; font-size: 12px; font-weight: 700; }
.center-page :deep(.el-table td.el-table__cell) { padding: 6px 0; }
.center-page :deep(.el-table__row:hover > td.el-table__cell) { background: #f8fafc; }
.center-page :deep(.el-input__wrapper),
.center-page :deep(.el-select__wrapper) { min-height: 34px; border-radius: 8px; background: #fbfdff; box-shadow: 0 0 0 1px #d9e5f2 inset; }
.center-page :deep(.el-input__wrapper:hover),
.center-page :deep(.el-select__wrapper:hover) { box-shadow: 0 0 0 1px #b9d4ff inset; }

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
:global(.ops-billing-dialog .el-form-item) { margin-bottom: 13px; }
:global(.ops-billing-dialog .el-form-item__label) { margin-bottom: 6px; color: #334155; font-size: 12px; font-weight: 700; line-height: 1.4; }
:global(.ops-billing-dialog .el-input__wrapper),
:global(.ops-billing-dialog .el-select__wrapper) { min-height: 34px; border-radius: 8px; background: #fbfdff; box-shadow: 0 0 0 1px #d9e5f2 inset; }
:global(.ops-billing-dialog .el-input__wrapper:hover),
:global(.ops-billing-dialog .el-select__wrapper:hover) { box-shadow: 0 0 0 1px #b9d4ff inset; }

@media (max-width: 1100px) {
  .metric-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}

@media (max-width: 960px) {
  .ops-page-header { align-items: flex-start; flex-wrap: wrap; }
  .ops-page-header__actions { width: 100%; margin-left: 54px; flex-wrap: wrap; }
  .toolbar { align-items: flex-start; }
}

@media (max-width: 620px) {
  .billing-workspace { padding: 6px; }
  .ops-page-header__icon { width: 36px; height: 36px; flex-basis: 36px; font-size: 26px; }
  .ops-page-header__content { align-items: flex-start; flex: 1; flex-direction: column; gap: 2px; }
  .ops-page-header__divider { display: none; }
  .ops-page-header p { width: 100%; white-space: normal; }
  .ops-page-header__actions { margin-left: 0; }
  .ops-page-header__actions :deep(.el-button) { flex: 1 1 calc(50% - 4px); margin-left: 0; }
  .metric-grid { grid-template-columns: 1fr; }
  .toolbar { flex-direction: column; }
  .filters { width: 100%; }
  .filters :deep(.el-select) { width: calc(50% - 4px); }
  .form-grid { grid-template-columns: 1fr; }
  .form-span-2 { grid-column: auto; }
  .alert-account { align-items: flex-start; flex-direction: column; }
  .pagination-wrap { justify-content: flex-start; overflow-x: auto; }
}

@media (prefers-reduced-motion: reduce) {
  .center-page *,
  .center-page *::before,
  .center-page *::after {
    transition-duration: 0.01ms !important;
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
  }
}
</style>
