<template>
  <div class="mcp-page">
    <section class="page-card">
      <header class="page-header">
        <div class="page-title-group">
          <span class="page-mark" aria-hidden="true">
            <el-icon><Connection /></el-icon>
          </span>
          <div class="page-title-copy">
            <div class="page-title-line">
              <h1>MCP 能力管理</h1>
              <span class="page-status">{{ summary.enabledServers }} 个服务运行中</span>
            </div>
            <p class="page-description">统一管理 MCP 服务、Tool 可用性与执行策略</p>
          </div>
        </div>
        <div class="header-actions">
          <el-button
            class="refresh-action"
            :icon="Refresh"
            :loading="loading"
            @click="loadServers"
          >
            刷新数据
          </el-button>
        </div>
      </header>

      <section class="summary-strip" aria-label="MCP 状态汇总">
        <div class="summary-heading">
          <strong>运行概览</strong>
          <span>实时统计当前接入能力</span>
        </div>
        <div class="summary-item">
          <span>MCP 服务</span>
          <strong>{{ summary.totalServers }}</strong>
        </div>
        <div class="summary-item summary-item--success">
          <span>已启用服务</span>
          <strong>{{ summary.enabledServers }}</strong>
        </div>
        <div class="summary-item summary-item--primary">
          <span>已启用 Tool</span>
          <strong>{{ summary.enabledTools }}</strong>
        </div>
        <div class="summary-item summary-item--muted">
          <span>待实现 Tool</span>
          <strong>{{ summary.unsupportedTools }}</strong>
        </div>
      </section>

      <el-alert
        v-if="loadError"
        class="load-error"
        type="error"
        :title="loadError"
        show-icon
        :closable="false"
      >
        <template #default>
          <el-button link type="primary" @click="loadServers">重新加载</el-button>
        </template>
      </el-alert>

      <div class="workspace">
        <nav class="domain-nav" aria-label="MCP 服务列表">
          <div class="domain-nav-head">
            <div>
              <strong>服务列表</strong>
              <span>{{ servers.length }} 个接入域</span>
            </div>
            <span class="nav-total">{{ summary.enabledServers }}/{{ summary.totalServers }}</span>
          </div>
          <div class="domain-nav-list">
            <button
              v-for="server in servers"
              :key="server.code"
              type="button"
              class="domain-nav-item"
              :class="{ active: server.code === selectedCode }"
              @click="selectedCode = server.code"
            >
              <span class="domain-icon">
                <el-icon><component :is="domainIcon(server.icon)" /></el-icon>
              </span>
              <span class="domain-copy">
                <strong>{{ server.displayName }}</strong>
                <small>{{ server.enabledTools }}/{{ server.totalTools }} Tool</small>
              </span>
              <span class="service-state" :class="{ enabled: server.enabled }">
                {{ server.enabled ? '运行' : '停用' }}
              </span>
            </button>
          </div>
        </nav>

        <main class="tool-workspace" v-loading="loading">
          <template v-if="currentServer">
            <div class="server-bar">
              <div class="server-identity">
                <span class="server-icon">
                  <el-icon><component :is="domainIcon(currentServer.icon)" /></el-icon>
                </span>
                <div>
                  <div class="server-title-line">
                    <h2>{{ currentServer.displayName }}</h2>
                    <el-tag size="small" effect="plain">v{{ currentServer.version }}</el-tag>
                  </div>
                  <p>{{ currentServer.description }}</p>
                </div>
              </div>
              <div class="master-switch">
                <span>{{ currentServer.enabled ? '服务已启用' : '服务已关闭' }}</span>
                <el-switch
                  v-model="currentServer.enabled"
                  :loading="serverUpdating"
                  aria-label="切换 MCP 服务状态"
                  @change="handleServerToggle"
                />
              </div>
            </div>

            <div class="tool-toolbar">
              <el-input
                v-model="filters.keyword"
                :prefix-icon="Search"
                clearable
                aria-label="搜索 Tool 名称或功能"
                placeholder="搜索 Tool 名称或功能"
              />
              <el-select v-model="filters.availability" aria-label="可用状态筛选">
                <el-option label="全部状态" value="all" />
                <el-option label="当前可用" value="supported" />
                <el-option label="待实现" value="unsupported" />
              </el-select>
              <el-select v-model="filters.risk" aria-label="风险等级筛选">
                <el-option label="全部风险" value="all" />
                <el-option v-for="risk in riskLevels" :key="risk" :label="risk" :value="risk" />
              </el-select>
              <span class="filter-result">{{ filteredTools.length }} 项</span>
            </div>

            <div class="table-shell">
              <el-table :data="filteredTools" row-key="name" class="tool-table" height="100%">
                <el-table-column prop="displayName" label="功能" width="160" fixed="left" show-overflow-tooltip>
                  <template #default="{ row }">
                    <strong class="tool-display-name">{{ row.displayName }}</strong>
                  </template>
                </el-table-column>
                <el-table-column prop="name" label="Tool 名称" width="210" show-overflow-tooltip>
                  <template #default="{ row }">
                    <code class="tool-code">{{ row.name }}</code>
                  </template>
                </el-table-column>
                <el-table-column prop="description" label="功能描述" min-width="280" show-overflow-tooltip>
                  <template #default="{ row }">
                    <span class="tool-description">{{ row.description }}</span>
                  </template>
                </el-table-column>
                <el-table-column label="分类" width="110">
                  <template #default="{ row }">
                    <span class="category-label">{{ row.category }}</span>
                  </template>
                </el-table-column>
                <el-table-column label="类型" width="104" align="center">
                  <template #default="{ row }">
                    <el-tag :type="row.readonly ? 'info' : 'warning'" size="small" effect="plain">
                      {{ row.readonly ? '只读' : '变更' }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="风险" width="82" align="center">
                  <template #default="{ row }">
                    <span class="risk-badge" :class="`risk-${row.effectiveRisk.toLowerCase()}`">{{ row.effectiveRisk }}</span>
                  </template>
                </el-table-column>
                <el-table-column label="二次确认" width="104" align="center">
                  <template #default="{ row }">
                    <span v-if="row.requiredConfirmation" class="confirm-required">
                      <el-icon><Lock /></el-icon>
                      需要
                    </span>
                    <span v-else class="muted-value">无需</span>
                  </template>
                </el-table-column>
                <el-table-column label="实现状态" width="112" align="center">
                  <template #default="{ row }">
                    <span class="support-state" :class="{ supported: row.supported }">
                      <el-icon><component :is="row.supported ? CircleCheck : Clock" /></el-icon>
                      {{ row.supported ? '可用' : '待实现' }}
                    </span>
                  </template>
                </el-table-column>
                <el-table-column label="启用" width="88" align="center">
                  <template #default="{ row }">
                    <el-tooltip :disabled="row.supported" content="当前版本尚未接入执行处理器" placement="top">
                      <span>
                        <el-switch
                          v-model="row.enabled"
                          :disabled="!row.supported || row.effectiveRisk === 'R4' || !currentServer.enabled"
                          :loading="toolUpdating === row.name"
                          :aria-label="`${row.displayName}启用状态`"
                          @change="value => handleToolToggle(row, value)"
                        />
                      </span>
                    </el-tooltip>
                  </template>
                </el-table-column>
                <el-table-column label="策略" width="76" align="center" fixed="right">
                  <template #default="{ row }">
                    <el-button link type="primary" @click="openPolicy(row)">配置</el-button>
                  </template>
                </el-table-column>
                <template #empty>
                  <el-empty description="没有符合条件的 Tool" :image-size="72" />
                </template>
              </el-table>
            </div>
          </template>
          <el-empty v-else-if="!loading" description="暂无 MCP 配置" />
        </main>
      </div>
    </section>

    <el-drawer v-model="policyVisible" title="Tool 策略" size="480px" class="mcp-policy-drawer" destroy-on-close>
      <template #header>
        <div class="drawer-title">
          <span class="drawer-title-icon"><el-icon><Setting /></el-icon></span>
          <div>
            <strong>Tool 策略</strong>
            <span>配置风险、确认范围与执行限制</span>
          </div>
        </div>
      </template>
      <template v-if="policyTarget">
        <div class="policy-heading">
          <strong>{{ policyTarget.displayName }}</strong>
          <code>{{ policyTarget.name }}</code>
        </div>
        <el-form label-position="top" class="policy-form">
          <div class="policy-grid">
            <el-form-item label="风险等级">
              <el-select v-model="policyForm.riskLevel">
                <el-option
                  v-for="risk in allowedRiskLevels"
                  :key="risk"
                  :label="risk"
                  :value="risk"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="二次确认">
              <el-switch
                v-model="policyForm.requiredConfirmation"
                :disabled="riskRequiresConfirmation"
                active-text="需要"
                inactive-text="无需"
              />
            </el-form-item>
          </div>
          <el-form-item label="允许环境">
            <el-checkbox-group v-model="policyForm.allowedEnvironments">
              <el-checkbox label="dev">开发</el-checkbox>
              <el-checkbox label="test">测试</el-checkbox>
              <el-checkbox label="prod">生产</el-checkbox>
            </el-checkbox-group>
          </el-form-item>
          <div class="policy-grid">
            <el-form-item label="超时（秒）">
              <el-input-number v-model="policyForm.timeoutSeconds" :min="1" :max="600" controls-position="right" />
            </el-form-item>
            <el-form-item label="每分钟限流">
              <el-input-number v-model="policyForm.rateLimitPerMinute" :min="1" :max="10000" controls-position="right" />
            </el-form-item>
            <el-form-item label="最大返回行数">
              <el-input-number v-model="policyForm.maxResultRows" :min="1" :max="100000" controls-position="right" />
            </el-form-item>
            <el-form-item label="最大返回字节">
              <el-input-number v-model="policyForm.maxResultBytes" :min="1024" :max="52428800" :step="1024" controls-position="right" />
            </el-form-item>
          </div>
        </el-form>
      </template>
      <template #footer>
        <el-button class="cancel-action" @click="policyVisible = false">取消</el-button>
        <el-button class="save-action" type="primary" :loading="policySaving" @click="savePolicy">保存</el-button>
      </template>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Bell,
  CircleCheck,
  Clock,
  Coin,
  Collection,
  Connection,
  Document,
  Grid,
  Lock,
  Monitor,
  Refresh,
  Search,
  Setting
} from '@element-plus/icons-vue'
import aiopsApi from '@/api/aiops'

const loading = ref(false)
const loadError = ref('')
const servers = ref([])
const selectedCode = ref('ecs')
const serverUpdating = ref(false)
const toolUpdating = ref('')
const policyVisible = ref(false)
const policySaving = ref(false)
const policyTarget = ref(null)
const riskLevels = ['R0', 'R1', 'R2', 'R3', 'R4']

const filters = reactive({ keyword: '', availability: 'all', risk: 'all' })
const policyForm = reactive({
  riskLevel: 'R0',
  requiredConfirmation: false,
  allowedEnvironments: [],
  timeoutSeconds: 30,
  rateLimitPerMinute: 60,
  maxResultRows: 1000,
  maxResultBytes: 1048576
})

const iconMap = { Monitor, Grid, Coin, Document, Bell, Collection, Connection }
const domainIcon = name => iconMap[name] || Connection

const currentServer = computed(() => servers.value.find(server => server.code === selectedCode.value) || servers.value[0])

const filteredTools = computed(() => {
  const keyword = filters.keyword.trim().toLowerCase()
  return (currentServer.value?.tools || []).filter(tool => {
    const matchesKeyword = !keyword || [tool.name, tool.displayName, tool.description]
      .some(value => String(value || '').toLowerCase().includes(keyword))
    const matchesAvailability = filters.availability === 'all'
      || (filters.availability === 'supported' ? tool.supported : !tool.supported)
    const matchesRisk = filters.risk === 'all' || tool.effectiveRisk === filters.risk
    return matchesKeyword && matchesAvailability && matchesRisk
  })
})

const summary = computed(() => {
  const allTools = servers.value.flatMap(server => server.tools || [])
  return {
    totalServers: servers.value.length,
    enabledServers: servers.value.filter(server => server.enabled).length,
    enabledTools: servers.value.reduce((total, server) => total + Number(server.enabledTools || 0), 0),
    unsupportedTools: allTools.filter(tool => !tool.supported).length
  }
})

const allowedRiskLevels = computed(() => {
  const floorIndex = riskLevels.indexOf(policyTarget.value?.riskFloor || 'R0')
  return riskLevels.slice(Math.max(floorIndex, 0))
})

const riskRequiresConfirmation = computed(() => ['R2', 'R3', 'R4'].includes(policyForm.riskLevel))

const responseBody = response => response?.data || response || {}
const getPayload = response => {
  const body = responseBody(response)
  if (body.code !== undefined && ![0, 200].includes(Number(body.code))) {
    throw new Error(body.message || '请求失败')
  }
  return body.data !== undefined ? body.data : body
}

const loadServers = async () => {
  loading.value = true
  loadError.value = ''
  try {
    const payload = getPayload(await aiopsApi.listMCPServers())
    servers.value = Array.isArray(payload) ? payload : (payload.servers || [])
    if (!servers.value.some(server => server.code === selectedCode.value)) {
      selectedCode.value = servers.value[0]?.code || ''
    }
  } catch (error) {
    loadError.value = error.message || 'MCP 配置加载失败'
  } finally {
    loading.value = false
  }
}

const handleServerToggle = async enabled => {
  const server = currentServer.value
  if (!server) return

  try {
    await ElMessageBox.confirm(
      enabled ? `确认启用「${server.displayName}」？` : `确认关闭「${server.displayName}」及其全部 Tool？`,
      enabled ? '启用 MCP' : '关闭 MCP',
      { type: enabled ? 'info' : 'warning', confirmButtonText: '确认', cancelButtonText: '取消' }
    )
    serverUpdating.value = true
    getPayload(await aiopsApi.updateMCPServerStatus(server.code, enabled))
    ElMessage.success(enabled ? 'MCP 已启用' : 'MCP 已关闭')
    await loadServers()
  } catch (error) {
    server.enabled = !enabled
    if (error !== 'cancel' && error !== 'close') ElMessage.error(error.message || '更新 MCP 状态失败')
  } finally {
    serverUpdating.value = false
  }
}

const handleToolToggle = async (tool, enabled) => {
  const serverCode = tool.domain
  try {
    await ElMessageBox.confirm(
      enabled ? `确认启用 Tool「${tool.displayName}」？` : `确认关闭 Tool「${tool.displayName}」？`,
      enabled ? '启用 Tool' : '关闭 Tool',
      { type: enabled ? 'info' : 'warning', confirmButtonText: '确认', cancelButtonText: '取消' }
    )
    toolUpdating.value = tool.name
    getPayload(await aiopsApi.updateMCPToolConfig(serverCode, tool.name, { enabled }))
    ElMessage.success(enabled ? 'Tool 已启用' : 'Tool 已关闭')
    await loadServers()
  } catch (error) {
    tool.enabled = !enabled
    if (error !== 'cancel' && error !== 'close') ElMessage.error(error.message || '更新 Tool 状态失败')
  } finally {
    toolUpdating.value = ''
  }
}

const openPolicy = tool => {
  policyTarget.value = tool
  Object.assign(policyForm, {
    riskLevel: tool.effectiveRisk,
    requiredConfirmation: tool.requiredConfirmation,
    allowedEnvironments: [...(tool.allowedEnvironments || [])],
    timeoutSeconds: tool.timeoutSeconds,
    rateLimitPerMinute: tool.rateLimitPerMinute,
    maxResultRows: tool.maxResultRows,
    maxResultBytes: tool.maxResultBytes
  })
  policyVisible.value = true
}

const savePolicy = async () => {
  if (!policyTarget.value) return
  if (!policyForm.allowedEnvironments.length) {
    ElMessage.warning('至少选择一个允许环境')
    return
  }
  if (riskRequiresConfirmation.value) policyForm.requiredConfirmation = true
  policySaving.value = true
  try {
    getPayload(await aiopsApi.updateMCPToolConfig(policyTarget.value.domain, policyTarget.value.name, { ...policyForm }))
    ElMessage.success('Tool 策略已保存')
    policyVisible.value = false
    await loadServers()
  } catch (error) {
    ElMessage.error(error.message || '保存 Tool 策略失败')
  } finally {
    policySaving.value = false
  }
}

onMounted(loadServers)
</script>

<style scoped>
.mcp-page {
  --mcp-page-height-offset: 124px;
  --mcp-brand: #3370ff;
  --mcp-brand-active: #245bdb;
  --mcp-brand-soft: #e8f0ff;
  --mcp-warm: #f8c544;
  --mcp-warm-soft: #fff8df;
  --mcp-success: #34a853;
  --mcp-warning: #d97706;
  --mcp-danger: #dc2626;
  --mcp-text-primary: #1f2329;
  --mcp-text-secondary: #646a73;
  --mcp-text-muted: #8f959e;
  --mcp-border: #dee0e3;
  --mcp-border-soft: #e8eaed;
  height: calc(100vh - var(--mcp-page-height-offset));
  min-height: 0;
  padding: 6px;
  box-sizing: border-box;
  overflow: hidden;
  color: var(--mcp-text-primary);
  background: #edf3f8;
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
  letter-spacing: 0;
}

.page-card {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
  min-width: 0;
  overflow: hidden;
  border: 1px solid #d9e1ea;
  border-radius: 12px;
  background: #fffdf9;
  box-shadow: 0 6px 20px rgba(45, 61, 84, 0.06);
}

.page-header,
.server-bar,
.tool-toolbar,
.server-identity,
.server-title-line,
.master-switch,
.policy-heading {
  display: flex;
  align-items: center;
}

.page-header {
  justify-content: space-between;
  gap: 12px;
  min-height: 58px;
  padding: 8px 16px;
  box-sizing: border-box;
  border-bottom: 1px solid #eadfca;
  background: #fffaf2;
}

.page-title-group {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
}

.page-mark {
  display: inline-grid;
  width: 32px;
  height: 32px;
  flex: 0 0 32px;
  place-items: center;
  border-radius: 6px;
  background: var(--mcp-brand);
  color: #fff;
  font-size: 18px;
}

.page-title-copy {
  min-width: 0;
}

.page-title-line {
  display: flex;
  align-items: center;
  gap: 9px;
}

.page-status {
  display: inline-flex;
  align-items: center;
  height: 20px;
  padding: 0 7px;
  border: 1px solid #eed27a;
  border-radius: 4px;
  background: var(--mcp-warm-soft);
  color: #9a6900;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
}

h1,
h2,
p {
  margin: 0;
}

h1 {
  color: var(--mcp-text-primary);
  font-size: 17px;
  font-weight: 700;
  line-height: 22px;
  white-space: nowrap;
}

.page-description {
  color: var(--mcp-text-secondary);
  font-size: 11px;
  line-height: 16px;
}

.header-actions {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 8px;
}

.refresh-action {
  min-width: 92px;
  height: 32px;
  border: 1px solid #e1b62f;
  border-radius: 6px;
  background: var(--mcp-warm);
  color: #5c4300;
  font-size: 12px;
  font-weight: 600;
  box-shadow: none;
}

.refresh-action:hover,
.refresh-action:focus {
  border-color: #d0a51e;
  background: #f5bd2e;
  color: #4a3600;
}

.refresh-action.is-disabled {
  color: #5c4300;
  opacity: 0.58;
}

.summary-strip {
  display: grid;
  grid-template-columns: 180px repeat(4, minmax(0, 1fr));
  min-height: 46px;
  box-sizing: border-box;
  border-bottom: 1px solid #eadfca;
  background: #fff8eb;
}

.summary-heading {
  display: flex;
  min-width: 0;
  flex-direction: column;
  justify-content: center;
  padding: 6px 16px;
  border-right: 1px solid #eadfca;
}

.summary-heading strong {
  color: var(--mcp-text-primary);
  font-size: 12px;
  line-height: 18px;
}

.summary-heading span {
  overflow: hidden;
  color: var(--mcp-text-muted);
  font-size: 10px;
  line-height: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.summary-item {
  position: relative;
  min-width: 0;
  display: flex;
  align-items: baseline;
  justify-content: center;
  gap: 8px;
  padding: 5px 12px;
}

.summary-item:not(:last-child)::after {
  position: absolute;
  top: 20%;
  right: 0;
  width: 1px;
  height: 60%;
  background: var(--mcp-border-soft);
  content: '';
}

.summary-item span {
  color: var(--mcp-text-secondary);
  font-size: 11px;
  white-space: nowrap;
}

.summary-item strong {
  color: var(--mcp-text-primary);
  font-family: Consolas, "SFMono-Regular", monospace;
  font-size: 18px;
  font-variant-numeric: tabular-nums;
  line-height: 20px;
}

.summary-item--success strong { color: #2d9560; }
.summary-item--primary strong { color: var(--mcp-brand-active); }
.summary-item--muted strong { color: #a07b3c; }

.load-error {
  margin: 0 8px 8px;
  border-radius: 8px;
}

.workspace {
  display: grid;
  flex: 1;
  grid-template-columns: 220px minmax(0, 1fr);
  min-width: 0;
  min-height: 0;
  margin: 8px;
  overflow: hidden;
  border: 1px solid #d5e2ef;
  border-radius: 9px;
  background: #eaf3fb;
}

.domain-nav {
  min-width: 0;
  min-height: 0;
  overflow-y: auto;
  padding: 8px;
  border-right: 1px solid #d5e2ef;
  background: #f7f1e8;
}

.domain-nav-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 3px 4px 9px;
}

.domain-nav-head > div {
  min-width: 0;
}

.domain-nav-head strong,
.domain-nav-head span {
  display: block;
}

.domain-nav-head strong {
  color: var(--mcp-text-primary);
  font-size: 13px;
  font-weight: 700;
}

.domain-nav-head span {
  margin-top: 2px;
  color: #76849a;
  font-size: 11px;
}

.domain-nav-head .nav-total {
  flex: 0 0 auto;
  min-width: 40px;
  margin-top: 0;
  padding: 4px 7px;
  border: 1px solid #c9d8fb;
  border-radius: 4px;
  background: #fff;
  color: var(--mcp-brand-active);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  font-weight: 700;
  text-align: center;
}

.domain-nav-list {
  display: grid;
  gap: 4px;
}

.domain-nav-item {
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr) 34px;
  gap: 8px;
  align-items: center;
  width: 100%;
  min-height: 46px;
  margin: 0;
  padding: 5px 7px;
  border: 1px solid transparent;
  border-radius: 6px;
  color: var(--mcp-text-secondary);
  text-align: left;
  background: transparent;
  cursor: pointer;
  transition: border-color 0.2s ease, background-color 0.2s ease, color 0.2s ease;
}

.domain-nav-item:hover {
  border-color: #d6e2f2;
  background: #fffdf9;
}

.domain-nav-item.active {
  border-color: #b9cff0;
  color: var(--mcp-brand-active);
  background: #edf5ff;
  box-shadow: none;
}

.domain-icon,
.server-icon {
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  border-radius: 6px;
  color: #5e6d82;
  background: #fff;
  border: 1px solid var(--mcp-border);
}

.domain-icon {
  width: 28px;
  height: 28px;
}

.active .domain-icon {
  color: var(--mcp-brand-active);
  background: #fff;
  border-color: #b8ccfa;
}

.domain-copy {
  min-width: 0;
}

.domain-copy strong,
.domain-copy small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.domain-copy strong { color: inherit; font-size: 13px; font-weight: 600; }
.domain-copy small { margin-top: 3px; color: #77859a; font-size: 11px; }

.service-state {
  color: var(--mcp-text-muted);
  font-size: 10px;
  font-weight: 600;
  text-align: right;
}

.service-state.enabled { color: #238a43; }

.tool-workspace {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  background: #fffdfb;
}

.server-bar {
  justify-content: space-between;
  min-height: 58px;
  padding: 7px 14px;
  box-sizing: border-box;
  border-bottom: 1px solid #dbe6f0;
  background: #fdf9f3;
}

.server-identity {
  min-width: 0;
  gap: 9px;
}

.server-icon {
  width: 32px;
  height: 32px;
  color: var(--mcp-brand-active);
  background: var(--mcp-brand-soft);
  border-color: #c9d8fb;
}

.server-title-line { gap: 7px; }
.server-title-line h2 { color: var(--mcp-text-primary); font-size: 15px; line-height: 20px; }
.server-identity p { margin-top: 1px; color: var(--mcp-text-secondary); font-size: 11px; line-height: 16px; }

.master-switch {
  flex: 0 0 auto;
  gap: 10px;
  margin-left: 20px;
  color: var(--mcp-text-secondary);
  font-size: 12px;
  font-weight: 600;
}

.tool-toolbar {
  gap: 8px;
  min-height: 42px;
  padding: 5px 10px;
  box-sizing: border-box;
  border-bottom: 1px solid #dbe6f0;
  background: #f0f6fb;
}

.tool-toolbar .el-input { width: min(340px, 40%); }
.tool-toolbar .el-select { width: 128px; }
.filter-result {
  margin-left: auto;
  color: var(--mcp-text-muted);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.table-shell {
  flex: 1;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  background: #fff;
}

.tool-table {
  width: 100%;
}

.tool-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.tool-table :deep(th.el-table__cell) {
  height: 34px;
  padding: 4px 0;
  background: #f5f6f7;
  color: #646a73;
  font-size: 11px;
  font-weight: 600;
}

.tool-table :deep(td.el-table__cell) {
  height: 44px;
  padding: 3px 0;
  color: #373c43;
  border-bottom-color: var(--mcp-border-soft);
}

.tool-table :deep(.cell) {
  padding: 0 10px;
  line-height: 20px;
}

.tool-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: #f5f8ff;
}

.tool-display-name,
.tool-code,
.tool-description {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tool-display-name {
  color: var(--mcp-text-primary);
  font-size: 13px;
  font-weight: 600;
}

.tool-code {
  color: #40618c;
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
}

.tool-description {
  color: var(--mcp-text-muted);
  font-size: 12px;
}

.category-label { color: var(--mcp-text-secondary); font-size: 12px; }

.risk-badge {
  display: inline-grid;
  place-items: center;
  width: 34px;
  height: 22px;
  border: 1px solid;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 700;
}

.risk-r0 { color: #238a43; border-color: #b7ebc6; background: #f0fbf3; }
.risk-r1 { color: #245bdb; border-color: #c9d8fb; background: #f0f5ff; }
.risk-r2 { color: #ad6800; border-color: #f5d59a; background: #fff8e8; }
.risk-r3 { color: #c24b32; border-color: #f2b8aa; background: #fff3f0; }
.risk-r4 { color: #a11b1b; border-color: #f0b8b8; background: #fff1f1; }

.confirm-required {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: #ad6800;
  font-size: 12px;
}
.muted-value { color: var(--mcp-text-muted); font-size: 12px; }

.support-state {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: #8a6978;
  font-size: 12px;
}

.support-state.supported { color: #238a43; }

.policy-heading {
  gap: 10px;
  padding: 11px 12px;
  border: 1px solid var(--mcp-border);
  border-radius: 6px;
  background: #f8f9fa;
}

.policy-heading code {
  overflow: hidden;
  color: var(--mcp-text-secondary);
  font-family: Consolas, "Courier New", monospace;
  text-overflow: ellipsis;
}

.policy-form { margin-top: 18px; }
.policy-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
.policy-grid .el-input-number,
.policy-grid .el-select { width: 100%; }

.drawer-title {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 11px;
}

.drawer-title-icon {
  width: 32px;
  height: 32px;
  display: inline-flex;
  flex: 0 0 32px;
  align-items: center;
  justify-content: center;
  border: 1px solid #c9d8fb;
  border-radius: 6px;
  background: var(--mcp-brand-soft);
  color: var(--mcp-brand);
}

.drawer-title strong,
.drawer-title span {
  display: block;
}

.drawer-title strong {
  color: var(--mcp-text-primary);
  font-size: 16px;
  font-weight: 700;
}

.drawer-title div > span {
  margin-top: 3px;
  color: var(--mcp-text-secondary);
  font-size: 12px;
}

.cancel-action,
.save-action {
  min-width: 88px;
  height: 32px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
}

.cancel-action {
  border: 1px solid var(--mcp-border);
  background: #fff;
  color: var(--mcp-text-primary);
}

.save-action {
  border: 1px solid var(--mcp-brand);
  background: var(--mcp-brand);
  color: #fff;
}

.cancel-action:hover,
.cancel-action:focus {
  border-color: #b9bdc4;
  background: #f5f6f7;
  color: var(--mcp-text-primary);
}

.save-action:hover,
.save-action:focus {
  border-color: var(--mcp-brand-active);
  background: var(--mcp-brand-active);
  color: #fff;
}

.save-action.is-disabled {
  color: #fff;
  opacity: 0.58;
}

.mcp-page :deep(.el-button) {
  font-weight: 600;
}

.mcp-page :deep(.el-input__wrapper),
.mcp-page :deep(.el-select__wrapper),
.mcp-page :deep(.el-input-number) {
  border-radius: 6px;
  background: #fff;
  box-shadow: inset 0 0 0 1px var(--mcp-border) !important;
}

.mcp-page :deep(.el-input__wrapper:hover),
.mcp-page :deep(.el-select__wrapper:hover),
.mcp-page :deep(.el-input-number:hover) {
  box-shadow: inset 0 0 0 1px #b9bdc4 !important;
}

.mcp-page :deep(.el-input__wrapper.is-focus),
.mcp-page :deep(.el-select__wrapper.is-focused),
.mcp-page :deep(.el-input-number.is-controls-right:focus-within) {
  box-shadow: inset 0 0 0 1px var(--mcp-brand), 0 0 0 2px rgba(51, 112, 255, 0.1) !important;
}

.mcp-page :deep(.el-tag) {
  border-radius: 4px;
  font-weight: 600;
}

.mcp-page :deep(.el-switch.is-checked .el-switch__core) {
  border-color: var(--mcp-success);
  background: var(--mcp-success);
}

.domain-nav-item:focus-visible,
.mcp-page :deep(.el-button:focus-visible) {
  outline: 2px solid rgba(51, 112, 255, 0.42);
  outline-offset: 2px;
}

:global(.mcp-policy-drawer) {
  --mcp-brand: #3370ff;
  --mcp-text-primary: #1f2329;
  --mcp-text-secondary: #646a73;
  overflow: hidden;
  border-left: 1px solid #dee0e3;
  border-radius: 8px 0 0 8px;
  background: #fff;
  box-shadow: -12px 0 32px rgba(31, 35, 41, 0.12);
}

:global(.mcp-policy-drawer .el-drawer__header) {
  min-height: 60px;
  margin-bottom: 0;
  padding: 14px 18px;
  border-bottom: 1px solid #e8eaed;
  background: #f8f9fa;
}

:global(.mcp-policy-drawer .el-drawer__body) {
  padding: 18px;
}

:global(.mcp-policy-drawer .el-drawer__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e8eaed;
  background: #fff;
}

:global(.mcp-policy-drawer .el-input__wrapper),
:global(.mcp-policy-drawer .el-select__wrapper),
:global(.mcp-policy-drawer .el-input-number) {
  border-radius: 6px;
  background: #fff;
  box-shadow: inset 0 0 0 1px #dee0e3 !important;
}

:global(.mcp-policy-drawer .el-input__wrapper:hover),
:global(.mcp-policy-drawer .el-select__wrapper:hover),
:global(.mcp-policy-drawer .el-input-number:hover) {
  box-shadow: inset 0 0 0 1px #b9bdc4 !important;
}

:global(.mcp-policy-drawer .el-input__wrapper.is-focus),
:global(.mcp-policy-drawer .el-select__wrapper.is-focused),
:global(.mcp-policy-drawer .el-input-number:focus-within) {
  box-shadow: inset 0 0 0 1px #3370ff, 0 0 0 2px rgba(51, 112, 255, 0.1) !important;
}

@media (max-width: 900px) {
  .mcp-page { padding: 4px; }
  .summary-strip { grid-template-columns: 150px repeat(4, minmax(100px, 1fr)); }
  .workspace {
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: auto minmax(0, 1fr);
  }
  .domain-nav {
    overflow-y: hidden;
    border-right: 0;
    border-bottom: 1px solid var(--mcp-border);
  }
  .domain-nav-list {
    display: flex;
    gap: 6px;
    overflow-x: auto;
    padding-bottom: 2px;
  }
  .domain-nav-item { flex: 0 0 210px; margin: 0; }
  .tool-toolbar { flex-wrap: wrap; }
  .tool-toolbar .el-input { width: 100%; }
}

@media (max-width: 560px) {
  .page-header {
    align-items: stretch;
    flex-direction: column;
    gap: 8px;
    padding: 9px 12px;
  }
  .page-title-group {
    align-items: center;
    flex-direction: row;
    gap: 9px;
  }
  .page-status { display: none; }
  .header-actions { justify-content: flex-end; }
  .refresh-action { width: 100%; }
  h1 { font-size: 16px; }
  .summary-heading { display: none; }
  .summary-strip { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .summary-item:not(:last-child)::after { display: none; }
  .summary-item:nth-child(2)::after,
  .summary-item:nth-child(4)::after { display: block; }
  .summary-item:nth-child(2),
  .summary-item:nth-child(3) { border-bottom: 1px solid var(--mcp-border-soft); }
  .summary-item { gap: 6px; padding: 7px 9px; }
  .summary-item span { font-size: 11px; }
  .summary-item strong { font-size: 16px; }
  .workspace { margin: 6px; }
  .server-bar { align-items: flex-start; gap: 14px; }
  .server-identity p { display: none; }
  .master-switch span { display: none; }
  .tool-toolbar .el-select { flex: 1 1 120px; width: auto; }
  .filter-result { width: 100%; margin-left: 0; }
  .policy-grid { grid-template-columns: 1fr; gap: 0; }
  :global(.mcp-policy-drawer) {
    width: 94vw !important;
    border-radius: 8px 0 0 8px;
  }
  :global(.mcp-policy-drawer .el-drawer__footer) {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }
  .cancel-action,
  .save-action {
    width: 100%;
    min-width: 0;
  }
}
</style>
