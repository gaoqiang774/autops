<template>
  <div class="agent-management">
    <header class="agent-hero-panel">
      <div class="agent-hero-copy">
        <span class="agent-hero-icon" aria-hidden="true">
          <img src="@/assets/image/探针.svg" alt="" />
        </span>
        <div class="agent-hero-content">
          <h1>Agent 管理</h1>
          <span class="agent-hero-divider" aria-hidden="true"></span>
          <p>统一管理 Agent 部署、运行状态与健康信息，保持主机侧接入边界清晰。</p>
        </div>
      </div>
      <div class="agent-hero-metrics" aria-label="Agent 运行摘要">
        <div>
          <span>Agent</span>
          <strong>{{ summaryTotal }}</strong>
        </div>
        <div>
          <span>运行中</span>
          <strong class="metric-success">{{ runningCount }}</strong>
        </div>
        <div>
          <span>异常</span>
          <strong class="metric-danger">{{ abnormalCount }}</strong>
        </div>
      </div>
    </header>

    <div class="agent-card">
      <div class="agent-toolbar">
        <div class="agent-toolbar-main">
          <el-form :model="queryParams" :inline="true" class="search-form" @submit.prevent>
            <el-form-item label="主机 / IP" class="search-keyword-item">
              <el-input
                v-model="queryParams.keyword"
                placeholder="主机名称或 IP 地址"
                size="small"
                clearable
                @keyup.enter="handleQuery"
              />
            </el-form-item>
            <el-form-item label="状态" class="search-status-item">
              <el-select
                v-model="queryParams.onlineStatus"
                placeholder="全部状态"
                size="small"
                @change="handleQuery"
              >
                <el-option label="全部" value="" />
                <el-option label="在线" :value="1" />
                <el-option label="离线" :value="2" />
              </el-select>
            </el-form-item>
            <el-form-item label="版本" class="search-version-item">
              <el-select
                v-model="queryParams.version"
                placeholder="全部版本"
                size="small"
                clearable
                filterable
                @change="handleQuery"
              >
                <el-option
                  v-for="item in agentVersionOptions"
                  :key="item.version"
                  :label="`${item.version} (${item.count})`"
                  :value="item.version"
                />
              </el-select>
            </el-form-item>
          </el-form>

          <div class="toolbar-actions">
            <el-button type="primary" icon="Search" size="small" @click="handleQuery">查询</el-button>
            <el-button type="warning" icon="Refresh" size="small" @click="resetQuery">重置</el-button>
            <el-button
              type="success"
              icon="Plus"
              size="small"
              v-authority="['ops:agent:create']"
              @click="handleDeployToHosts"
            >部署 Agent</el-button>
            <el-button
              type="primary"
              plain
              icon="Upload"
              size="small"
              v-authority="['ops:agent:create']"
              @click="handleOfflineDeployToHosts"
            >离线部署</el-button>
            <el-button
              type="info"
              plain
              icon="DocumentChecked"
              size="small"
              v-authority="['ops:agent:create']"
              @click="openOfflineAssetCheck"
            >离线包管理</el-button>
            <el-button
              type="warning"
              icon="Refresh"
              size="small"
              v-authority="['ops:agent:create']"
              :disabled="selectedAgents.length === 0"
              @click="batchRedeploy"
            >批量更新 ({{ selectedAgents.length }})</el-button>
            <el-button
              type="danger"
              icon="Delete"
              size="small"
              v-authority="['ops:agent:deleteall']"
              :disabled="selectedAgents.length === 0"
              @click="batchUninstall"
            >批量卸载 ({{ selectedAgents.length }})</el-button>
          </div>

        </div>
      </div>

      <div class="table-section">
        <el-table
          v-loading="loading"
          :data="agents"
          stripe
          style="width: 100%"
          class="agent-table"
          @selection-change="handleSelectionChange"
        >
      <el-table-column type="selection" width="55" />
      <el-table-column prop="hostName" label="主机名称" min-width="170">
        <template #default="{row}">
          <div class="cell-with-icon host-name-cell">
            <img src="@/assets/image/探针.svg" class="cell-icon" />
            <el-tooltip :content="row.hostName || '-'" placement="top">
              <span class="cell-text host-name-text">{{ row.hostName || '-' }}</span>
            </el-tooltip>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="sshIp" label="IP地址" min-width="145">
        <template #default="{row}">
          <div style="display: flex; align-items: center; gap: 8px;">
            <img src="@/assets/image/ip.svg" style="width: 16px; height: 16px"/>
            <span>{{ row.sshIp }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="version" label="版本" min-width="120">
        <template #default="{row}">
          <div style="display: flex; align-items: center; gap: 8px;">
            <img src="@/assets/image/版本.svg" style="width: 16px; height: 16px"/>
            <span>{{ row.version }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="onlineStatus" label="状态" min-width="120">
        <template #default="{row}">
          <span :class="['status-chip', getStatusClass(row.onlineStatus)]">
            <el-icon><component :is="getStatusIcon(row.onlineStatus)" /></el-icon>
            {{ getStatusText(row.onlineStatus) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="port" label="监听端口" min-width="110">
        <template #default="{row}">
          <div style="display: flex; align-items: center; gap: 8px;">
            <img src="@/assets/image/端口.svg" style="width: 16px; height: 16px"/>
            <span>{{ row.port }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="安装进度" min-width="135">
        <template #default="{row}">
          <div v-if="isAgentDeploying(row)">
            <el-progress
              :percentage="getAgentProgressPercentage(row)"
              status="success"
              :stroke-width="8"
            />
            <div style="margin-top: 4px; font-size: 12px; color: #67C23A;">
              {{ getAgentProgressText(row) }}
            </div>
          </div>
          <div v-else-if="isAgentDeployFailed(row)">
            <el-progress
              :percentage="100"
              status="exception"
              :stroke-width="8"
            />
            <div style="margin-top: 4px; font-size: 12px; color: #F56C6C;">
              {{ getAgentProgressText(row) }}
            </div>
          </div>
          <div v-else-if="isAgentRunningState(row)">
            <el-progress
              :percentage="100"
              status="success"
              :stroke-width="8"
            />
          </div>
          <span v-else>-</span>
        </template>
      </el-table-column>
      <el-table-column prop="lastHeartbeat" label="最后心跳" min-width="160">
        <template #default="{row}">
          <span v-if="row.lastHeartbeat && row.lastHeartbeat !== '0001-01-01 00:00:00'">
            {{ formatTime(row.lastHeartbeat) }}
          </span>
          <span v-else class="text-muted">未连接</span>
        </template>
      </el-table-column>
      <el-table-column prop="updateTime" label="更新时间" min-width="160">
        <template #default="{row}">
          {{ formatTime(row.updateTime) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" fixed="right" width="270">
        <template #default="{row}">
          <div class="operation-links">
          <el-button
            link
            size="small"
            type="success"
            icon="Refresh"
            @click="handleRedeploy(row)"
            :disabled="row.status === 1"
          >重新部署</el-button>

          <el-button
            link
            size="small"
            type="warning"
            icon="Refresh"
            @click="handleRestart(row)"
            :disabled="row.status === 1"
            v-if="row.status === 4"
          >重启</el-button>

          <el-tooltip content="卸载" placement="top">
            <el-button
              link
              size="small"
              type="warning"
              v-authority="['ops:agent:delete']"
              @click="handleUninstall(row)"
            >
              <img src="@/assets/image/卸载.svg" style="width: 16px; height: 16px"/>卸载
            </el-button>
          </el-tooltip>

          <el-button
            link
            size="small"
            type="danger"
            icon="Delete"
            v-authority="['ops:agent:delete']"
            @click="handleDelete(row)"
          >删除</el-button>

          <el-button
            link
            size="small"
            type="success"
            icon="View"
            v-authority="['ops:agent:get']"
            @click="viewAgentDetails(row)"
          >详情</el-button>
          </div>
        </template>
      </el-table-column>
      <template #empty>
        <el-empty description="暂无 Agent 数据" :image-size="64" />
      </template>
        </el-table>
      </div>

      <!-- 分页区域 -->
      <div class="pagination-section">
        <el-pagination
          @size-change="handleSizeChange"
          @current-change="handleCurrentChange"
          :current-page="queryParams.page"
          :page-sizes="[10, 50, 100]"
          :page-size="queryParams.pageSize"
          layout="total, sizes, prev, pager, next, jumper"
          :total="total"
        />
      </div>

    <!--Agent详情对话框-->
    <el-dialog
      title="Agent详情"
      v-model="detailVisible"
      width="760px"
      class="agent-detail-dialog"
      modal-class="agent-detail-dialog-overlay"
    >
      <el-descriptions
        :column="2"
        border
        v-if="currentAgent"
      >
        <el-descriptions-item label="主机名称">
          <div class="cell-with-icon">
            <img src="@/assets/image/探针.svg" class="cell-icon" />
            <span class="cell-text">{{ currentAgent.hostName || '-' }}</span>
          </div>
        </el-descriptions-item>
        <el-descriptions-item label="IP地址">{{ currentAgent.sshIp }}</el-descriptions-item>
        <el-descriptions-item label="版本">{{ currentAgent.version }}</el-descriptions-item>
        <el-descriptions-item label="状态">
          <span :class="['status-chip', getStatusClass(currentAgent.onlineStatus)]">
            <el-icon><component :is="getStatusIcon(currentAgent.onlineStatus)" /></el-icon>
            {{ getStatusText(currentAgent.onlineStatus) }}
          </span>
        </el-descriptions-item>
        <el-descriptions-item label="安装路径">{{ currentAgent.installPath || '未设置' }}</el-descriptions-item>
        <el-descriptions-item label="监听端口">{{ currentAgent.port }}</el-descriptions-item>
        <el-descriptions-item label="进程ID">{{ currentAgent.pid || '无' }}</el-descriptions-item>
        <el-descriptions-item label="最后心跳" span="2">
          <span v-if="currentAgent.lastHeartbeat && currentAgent.lastHeartbeat !== '0001-01-01 00:00:00'">
            {{ formatTime(currentAgent.lastHeartbeat) }}
          </span>
          <span v-else class="text-muted">未连接</span>
        </el-descriptions-item>
        <el-descriptions-item label="创建时间" span="2">
          {{ formatTime(currentAgent.createTime) }}
        </el-descriptions-item>
        <el-descriptions-item label="更新时间" span="2">
          {{ formatTime(currentAgent.updateTime) }}
        </el-descriptions-item>
        <el-descriptions-item label="错误信息" span="2" v-if="currentAgent.errorMsg">
          <el-alert
            :title="currentAgent.errorMsg"
            type="error"
            :closable="false"
            show-icon
          />
        </el-descriptions-item>
      </el-descriptions>
      <template #footer>
        <el-button @click="detailVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="offlineAssetCheckVisible"
      title="离线 Agent 包管理"
      width="620px"
      class="offline-asset-dialog"
      :close-on-click-modal="false"
      @open="refreshOfflineAssetCheck"
    >
      <el-alert
        :title="offlineAssetCheck.message || '正在检查离线 Agent 包'"
        :type="offlineAssetCheck.ready ? 'success' : 'warning'"
        :closable="false"
        show-icon
      />
      <div v-loading="offlineAssetCheckLoading" class="offline-asset-status-list">
        <div v-for="slot in offlineAssetSlots" :key="slot.key" class="offline-asset-status-row">
          <div class="offline-asset-status-title">
            <span>{{ slot.label }}</span>
            <el-tag :type="slot.data.uploaded ? 'success' : 'danger'" effect="light">
              {{ slot.data.uploaded ? '已上传' : '未上传' }}
            </el-tag>
          </div>
          <div v-if="slot.data.asset" class="offline-asset-status-file">
            {{ slot.data.asset.path }} · {{ formatOfflineAssetSize(slot.data.asset.size) }}
          </div>
          <div v-else class="offline-asset-status-file is-missing">
            请上传 {{ slot.key === 'x86_64' ? 'x86_64/amd64' : 'arm64/aarch64' }} Agent 二进制
          </div>
        </div>
      </div>
      <el-upload
        :http-request="uploadOfflineBinary"
        :show-file-list="false"
        class="offline-asset-upload"
      >
        <el-button type="primary" :loading="offlineBinaryUploading">上传离线 Agent 二进制</el-button>
      </el-upload>
      <div class="offline-asset-upload-hint">
        可连续上传两个包，服务端会按 ELF 文件架构识别 x86_64 与 arm64，上传成功后状态会自动刷新。
      </div>
      <template #footer>
        <el-button @click="offlineAssetCheckVisible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="offlineDeployVisible"
      title="离线部署 Agent"
      width="560px"
      class="offline-agent-dialog"
      :close-on-click-modal="false"
      @closed="resetOfflineDeploy"
    >
      <el-alert
        title="服务端会根据目标主机 CPU 架构自动选择二进制，并复用在线部署的传输、启动和状态流程。"
        type="info"
        :closable="false"
        show-icon
        class="offline-agent-tip"
      />
      <el-form label-position="top" class="offline-agent-form">
        <el-form-item label="上传离线 Agent 二进制">
          <el-upload
            :http-request="uploadOfflineBinary"
            :show-file-list="false"
            class="offline-agent-upload"
          >
            <el-button type="primary" plain :loading="offlineBinaryUploading">上传二进制</el-button>
          </el-upload>
          <div class="offline-agent-upload-hint">
            文件会保存到服务端 upload/agent/，可分别上传 x86_64 和 arm64 版本。
          </div>
        </el-form-item>
        <el-form-item label="x86_64 / amd64 Agent" required>
          <el-select
            v-model="offlineForm.binaryX86Path"
            placeholder="选择 upload/agent/ 下的 x86_64 二进制"
            filterable
            clearable
            class="offline-agent-select"
          >
            <el-option
              v-for="asset in offlineBinaryAssets"
              :key="`x86-${asset.path}`"
              :label="formatOfflineAssetLabel(asset)"
              :value="asset.path"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="arm64 / aarch64 Agent" required>
          <el-select
            v-model="offlineForm.binaryArm64Path"
            placeholder="选择 upload/agent/ 下的 arm64 二进制"
            filterable
            clearable
            class="offline-agent-select"
          >
            <el-option
              v-for="asset in offlineBinaryAssets"
              :key="`arm64-${asset.path}`"
              :label="formatOfflineAssetLabel(asset)"
              :value="asset.path"
            />
          </el-select>
        </el-form-item>
        <div class="offline-agent-generated-config">
          配置文件由服务端根据每台主机的 CMDB 分组、主机标识和全局监控配置自动生成，并随二进制下发。
        </div>
      </el-form>
      <div v-if="offlineHosts.length > 0" class="offline-agent-hosts">
        已选择 {{ offlineHosts.length }} 台主机：{{ offlineHosts.map(host => host.hostName || host.name).join('、') }}
      </div>
      <template #footer>
        <el-button @click="offlineDeployVisible = false">取消</el-button>
        <el-button
          type="primary"
          :loading="offlineDeployLoading"
          :disabled="!canSubmitOfflineDeploy"
          @click="submitOfflineDeploy"
        >开始离线部署</el-button>
      </template>
    </el-dialog>

    <!--选择主机部署Agent对话框-->
    <AgentHostSelectorDialog
      v-model="showDeployHostDialog"
      :title="deployMode === 'offline' ? '选择离线部署 Agent 的主机' : '选择部署 Agent 的主机'"
      @hosts-selected="handleHostsSelected"
    />
    </div>
  </div>
</template>

<script setup>
import { computed, ref, reactive, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { 
  CircleCheck,
  CircleClose
} from '@element-plus/icons-vue'
import cmdbAPI from '@/api/cmdb'
import AgentHostSelectorDialog from '@/components/AgentHostSelectorDialog.vue'

const agents = ref([])
const loading = ref(false)
const detailVisible = ref(false)
const currentAgent = ref(null)
const selectedAgents = ref([])
const total = ref(0)
const agentVersionOptions = ref([])
const agentSummary = reactive({
  total: 0,
  running: 0,
  abnormal: 0
})
const showDeployHostDialog = ref(false)
const deployMode = ref('online')
const offlineDeployVisible = ref(false)
const offlineDeployLoading = ref(false)
const offlineBinaryUploading = ref(false)
const offlineAssetCheckVisible = ref(false)
const offlineAssetCheckLoading = ref(false)
const offlineAssetCheck = reactive({
  ready: false,
  message: '',
  x86_64: { uploaded: false, asset: null },
  arm64: { uploaded: false, asset: null }
})
const offlineAssets = ref([])
const offlineHosts = ref([])
const offlineForm = reactive({
  binaryX86Path: '',
  binaryArm64Path: ''
})
let pollingTimer = null
let pollingCounter = 0
const MAX_POLLING_COUNT = 5 // 最多轮询5次(15秒)

const queryParams = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  onlineStatus: '',
  version: ''
})

const getAgentStatus = item => Number(item?.status ?? 0)
const getAgentOnlineStatus = item => Number(item?.onlineStatus ?? 0)
const getAgentInstallProgress = item => Number(item?.installProgress ?? 0)

const isAgentDeploying = item => getAgentStatus(item) === 1

const isAgentDeployFailed = item =>
  getAgentStatus(item) === 2 || getAgentInstallProgress(item) === -1

const isAgentRunningState = item =>
  getAgentStatus(item) === 3 &&
  getAgentOnlineStatus(item) === 1 &&
  getAgentInstallProgress(item) === 100

const isAgentRunning = item => getAgentStatus(item) === 3 && getAgentOnlineStatus(item) === 1

const isAgentAbnormal = item =>
  getAgentOnlineStatus(item) === 2 ||
  isAgentDeployFailed(item) ||
  getAgentStatus(item) === 4

const getAgentProgressPercentage = item => {
  if (isAgentDeployFailed(item)) return 100
  const progress = getAgentInstallProgress(item)
  return Math.min(Math.max(progress, 0), 100)
}

const getAgentProgressText = item => {
  if (isAgentDeployFailed(item)) {
    return item.installProgressText || '部署失败'
  }
  if (isAgentRunningState(item)) {
    return item.installProgressText || '启动成功'
  }
  return item.installProgressText || '部署中...'
}

const summaryTotal = computed(() => agentSummary.total)
const runningCount = computed(() => agentSummary.running)
const abnormalCount = computed(() => agentSummary.abnormal)

const isIpSearchToken = value => /^\d{1,3}(?:\.\d{1,3}){1,3}$/.test(String(value || '').trim())

const parseAgentSearchKeyword = keyword => {
  const normalizedKeyword = String(keyword || '').trim()
  if (!normalizedKeyword) {
    return {
      hostName: '',
      sshIp: ''
    }
  }

  const tokens = normalizedKeyword.split(/[\s,，]+/).filter(Boolean)
  const sshIp = tokens.find(isIpSearchToken) || ''
  const hostName = sshIp
    ? tokens.filter(token => token !== sshIp).join(' ').trim()
    : normalizedKeyword

  return {
    hostName,
    sshIp
  }
}

const extractList = payload => {
  if (Array.isArray(payload)) return payload
  return payload?.list || payload?.items || []
}

const extractTotal = payload => {
  if (Array.isArray(payload)) return payload.length
  if (typeof payload?.total === 'number') return payload.total
  if (typeof payload?.count === 'number') return payload.count
  return extractList(payload).length
}

const normalizeAgent = item => {
  const normalizedItem = {
    id: item.id,
    hostId: item.hostId,
    hostName: item.hostName,
    sshIp: item.sshIp,
    version: item.version,
    status: item.status,
    onlineStatus: item.onlineStatus,
    statusText: item.statusText,
    installPath: item.installPath,
    port: item.port,
    pid: item.pid,
    lastHeartbeat: item.lastHeartbeat,
    updateTime: item.updateTime,
    createTime: item.createTime,
    isHealthy: item.isHealthy,
    errorMsg: item.errorMsg,
    installProgress: item.installProgress,
    installProgressText: item.installProgressText
  }

  if (getAgentStatus(normalizedItem) === 3 && getAgentOnlineStatus(normalizedItem) === 1) {
    normalizedItem.installProgress = 100
    normalizedItem.installProgressText = normalizedItem.installProgressText || '启动成功'
    normalizedItem.errorMsg = ''
  }

  return normalizedItem
}

const normalizeVersionOption = item => ({
  version: String(item?.version || '').trim(),
  count: Number(item?.count || 0)
})

const sortVersionOptions = options =>
  [...options].sort((left, right) =>
    String(right.version || '').localeCompare(String(left.version || ''), 'zh-CN', {
      numeric: true,
      sensitivity: 'base'
    })
  )

const fetchAgentVersions = async () => {
  try {
    const response = await cmdbAPI.getAgentVersions()
    if (response.data?.code !== 200) {
      agentVersionOptions.value = []
      return []
    }

    const versionOptions = extractList(response.data.data)
      .map(normalizeVersionOption)
      .filter(item => item.version)

    agentVersionOptions.value = sortVersionOptions(versionOptions)
    return agentVersionOptions.value
  } catch (error) {
    console.error('获取Agent版本列表失败:', error)
    agentVersionOptions.value = []
    return []
  }
}

const getDeployVersion = async () => {
  const currentVersion = agentVersionOptions.value[0]?.version
  if (currentVersion) {
    return currentVersion
  }

  const versions = await fetchAgentVersions()
  const deployVersion = versions[0]?.version

  if (!deployVersion) {
    throw new Error('后端未返回可用的 Agent 版本')
  }

  return deployVersion
}

const offlineBinaryAssets = computed(() =>
  offlineAssets.value.filter(asset => asset.kind === 'binary')
)

const offlineAssetSlots = computed(() => [
  { key: 'x86_64', label: 'x86_64 / amd64', data: offlineAssetCheck.x86_64 },
  { key: 'arm64', label: 'arm64 / aarch64', data: offlineAssetCheck.arm64 }
])

const canSubmitOfflineDeploy = computed(() =>
  offlineHosts.value.length > 0 &&
  Boolean(offlineForm.binaryX86Path) &&
  Boolean(offlineForm.binaryArm64Path)
)

const formatOfflineAssetLabel = asset => {
  const size = Number(asset?.size || 0)
  const sizeLabel = size >= 1024 * 1024
    ? `${(size / (1024 * 1024)).toFixed(1)} MB`
    : `${Math.max(1, Math.round(size / 1024))} KB`
  const archLabel = asset?.arch ? ` · ${asset.arch}` : ''
  return `${asset?.path || asset?.name || '-'} (${sizeLabel}${archLabel})`
}

const formatOfflineAssetSize = size => {
  const value = Number(size || 0)
  if (value >= 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MB`
  return `${Math.max(1, Math.round(value / 1024))} KB`
}

const refreshOfflineAssetCheck = async () => {
  offlineAssetCheckLoading.value = true
  try {
    const response = await cmdbAPI.checkAgentOfflineAssets()
    if (response.data?.code !== 200) {
      throw new Error(response.data?.message || '检查离线 Agent 包失败')
    }
    const payload = response.data.data || {}
    offlineAssetCheck.ready = Boolean(payload.ready)
    offlineAssetCheck.message = payload.message || ''
    offlineAssetCheck.x86_64 = payload.x86_64 || { uploaded: false, asset: null }
    offlineAssetCheck.arm64 = payload.arm64 || { uploaded: false, asset: null }
  } catch (error) {
    offlineAssetCheck.ready = false
    offlineAssetCheck.message = error.message || '检查离线 Agent 包失败'
    ElMessage.error(offlineAssetCheck.message)
  } finally {
    offlineAssetCheckLoading.value = false
  }
}

const openOfflineAssetCheck = () => {
  offlineAssetCheckVisible.value = true
}

const loadOfflineAssets = async ({ openDialog = true } = {}) => {
  try {
    const response = await cmdbAPI.getAgentOfflineAssets()
    if (response.data?.code !== 200) {
      throw new Error(response.data?.message || '获取离线 Agent 制品失败')
    }

    offlineAssets.value = extractList(response.data.data)
      .filter(asset => asset && asset.path)

    const x86Asset = offlineBinaryAssets.value.find(asset =>
      asset.arch === 'x86_64' || /amd64|x86_64|x86/i.test(asset.path)
    )
    const arm64Asset = offlineBinaryAssets.value.find(asset =>
      asset.arch === 'arm64' || /arm64|aarch64/i.test(asset.path)
    )
    if (!offlineForm.binaryX86Path && x86Asset) {
      offlineForm.binaryX86Path = x86Asset.path
    }
    if (!offlineForm.binaryArm64Path && arm64Asset) {
      offlineForm.binaryArm64Path = arm64Asset.path
    }
    if (openDialog) {
      offlineDeployVisible.value = true
      if (offlineBinaryAssets.value.length === 0) {
        ElMessage.warning('upload/agent/ 下暂无 Agent 二进制文件，请先上传')
      }
    }
  } catch (error) {
    if (openDialog) {
      offlineHosts.value = []
    }
    ElMessage.error(error.message || '获取离线 Agent 制品失败')
  }
}

const uploadOfflineBinary = async ({ file, onSuccess, onError }) => {
  offlineBinaryUploading.value = true
  try {
    const response = await cmdbAPI.uploadAgentOfflineBinary(file)
    if (response.data?.code !== 200) {
      throw new Error(response.data?.message || '上传离线 Agent 二进制失败')
    }
    ElMessage.success(`已上传 ${file.name}`)
    await loadOfflineAssets({ openDialog: false })
    await refreshOfflineAssetCheck()
    onSuccess(response.data)
  } catch (error) {
    onError(error)
    ElMessage.error(error.response?.data?.message || error.message || '上传离线 Agent 二进制失败')
  } finally {
    offlineBinaryUploading.value = false
  }
}

const fetchAgentSummary = async () => {
  try {
    const response = await cmdbAPI.getAgentList({ page: 1, pageSize: 1000 })
    if (response.data?.code !== 200) {
      return
    }

    const agentList = extractList(response.data.data).map(normalizeAgent)
    agentSummary.total = extractTotal(response.data.data)
    agentSummary.running = agentList.filter(isAgentRunning).length
    agentSummary.abnormal = agentList.filter(isAgentAbnormal).length
  } catch (error) {
    console.error('获取Agent统计失败:', error)
  }
}

const fetchAgents = async () => {
  loading.value = true
  try {
    const { hostName, sshIp } = parseAgentSearchKeyword(queryParams.keyword)
    const version = queryParams.version.trim()
    const onlineStatus = queryParams.onlineStatus || undefined
    const hasKeyword = Boolean(hostName || sshIp)
    const shouldUseVersionQueryApi = Boolean(version) && !onlineStatus
    const shouldUseQueryApi = !version && !onlineStatus && Boolean(hostName) !== Boolean(sshIp)
    const response = shouldUseVersionQueryApi
      ? await cmdbAPI.queryAgentByVersion(version, {
          page: queryParams.page,
          pageSize: queryParams.pageSize,
          hostName: hasKeyword ? hostName || undefined : undefined,
          ip: sshIp || undefined
        })
      : shouldUseQueryApi
        ? await cmdbAPI.queryAgent({
            page: queryParams.page,
            pageSize: queryParams.pageSize,
            hostName: hostName || undefined,
            ip: sshIp || undefined
          })
        : await cmdbAPI.getAgentList({
            page: queryParams.page,
            pageSize: queryParams.pageSize,
            hostName: hostName || undefined,
            sshIp: sshIp || undefined,
            onlineStatus,
            version: version || undefined
          })
    
    if (response.data?.code === 200) {
      const payload = response.data.data || {}
      const normalizedAgents = extractList(payload).map(normalizeAgent)

      // 映射API响应数据到组件所需的格式
      agents.value = normalizedAgents
      total.value = extractTotal(payload)
      fetchAgentSummary()
    } else {
      throw new Error(response.data?.message || '获取Agent列表失败')
    }
  } catch (error) {
    console.error('获取Agent列表失败:', error)
    ElMessage.error(`获取Agent列表失败: ${error.message || '未知错误'}`)
    agents.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

const handleRedeploy = async (row) => {
  try {
    await ElMessageBox.confirm(`确定要重新部署Agent到主机 ${row.hostName} 吗？`, '确认重新部署', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'info'
    })
    
    await cmdbAPI.redeployAgent([row.hostId])
    ElMessage.success('Agent重新部署已启动，请等待部署完成')
    fetchAgents()
    
    startPolling()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('重新部署Agent失败:', error)
      ElMessage.error(`重新部署Agent失败: ${error.response?.data?.message || error.message || '未知错误'}`)
    }
  }
}

const handleDeployToHosts = () => {
  deployMode.value = 'online'
  showDeployHostDialog.value = true
}

const handleOfflineDeployToHosts = () => {
  deployMode.value = 'offline'
  showDeployHostDialog.value = true
}

const handleHostsSelected = async (selectedHosts) => {
  if (!selectedHosts || selectedHosts.length === 0) {
    ElMessage.warning('请选择要部署Agent的主机')
    return
  }

  if (deployMode.value === 'offline') {
    offlineHosts.value = selectedHosts
    await loadOfflineAssets()
    return
  }

  try {
    const hostNames = selectedHosts
      .map(host => host.hostName || host.name || `资产ID ${host.hostId || host.id}`)
      .join(', ')
    await ElMessageBox.confirm(`确定要在以下主机上部署Agent吗？\n${hostNames}`, '确认部署', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'info'
    })
    
    const hostIds = selectedHosts.map(host => host.hostId || host.id)
    const deployVersion = await getDeployVersion()
    await cmdbAPI.deployAgent(hostIds, deployVersion)
    
    ElMessage.success(`已开始在 ${selectedHosts.length} 台主机上部署Agent，请等待部署完成`)
    showDeployHostDialog.value = false
    fetchAgents()
    startPolling()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('部署Agent失败:', error)
      ElMessage.error(`部署Agent失败: ${error.response?.data?.message || error.message || '未知错误'}`)
    }
  }
}

const resetOfflineDeploy = () => {
  offlineHosts.value = []
  offlineAssets.value = []
  offlineForm.binaryX86Path = ''
  offlineForm.binaryArm64Path = ''
}

const submitOfflineDeploy = async () => {
  if (!canSubmitOfflineDeploy.value) {
    ElMessage.warning('请选择 x86_64 和 arm64 Agent 二进制文件')
    return
  }

  try {
    const hostNames = offlineHosts.value
      .map(host => host.hostName || host.name || `资产ID ${host.hostId || host.id}`)
      .join(', ')
    await refreshOfflineAssetCheck()
    if (!offlineAssetCheck.ready) {
      ElMessage.error(offlineAssetCheck.message || '请先上传 x86_64 和 arm64 离线 Agent 包')
      return
    }
    const hostIds = offlineHosts.value.map(host => host.hostId || host.id)
    const precheckResponse = await cmdbAPI.precheckAgentOfflineDeployment({
      hostIds,
      binaryX86Path: offlineForm.binaryX86Path,
      binaryArm64Path: offlineForm.binaryArm64Path
    })
    if (precheckResponse.data?.code !== 200) {
      throw new Error(precheckResponse.data?.message || '离线部署预检查失败')
    }
    const precheck = precheckResponse.data.data || {}
    if (!precheck.ready) {
      const failedHosts = (precheck.hosts || [])
        .filter(host => !host.ready)
        .map(host => `${host.hostName || host.hostId}: ${host.error || '未通过检查'}`)
        .join('\n')
      ElMessage.error(failedHosts || precheck.error || '离线部署预检查未通过')
      return
    }
    await ElMessageBox.confirm(`确定要在以下主机上执行离线部署吗？\n${hostNames}`, '确认离线部署', {
      confirmButtonText: '开始部署',
      cancelButtonText: '取消',
      type: 'info'
    })

    offlineDeployLoading.value = true
    const deployVersion = await getDeployVersion()
    await cmdbAPI.deployAgent(hostIds, deployVersion, {
      deployMode: 'offline',
      binaryX86Path: offlineForm.binaryX86Path,
      binaryArm64Path: offlineForm.binaryArm64Path
    })

    ElMessage.success(`已开始在 ${offlineHosts.value.length} 台主机上离线部署 Agent，请等待部署完成`)
    offlineDeployVisible.value = false
    await fetchAgents()
    startPolling()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('离线部署Agent失败:', error)
      ElMessage.error(`离线部署Agent失败: ${error.response?.data?.message || error.message || '未知错误'}`)
    }
  } finally {
    offlineDeployLoading.value = false
  }
}

const handleUninstall = async (row) => {
  try {
    await ElMessageBox.confirm(`确定要卸载主机 ${row.hostName} 的Agent吗？`, '确认卸载', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })

    await cmdbAPI.uninstallAgent([row.hostId])
    ElMessage.success('Agent卸载已启动，请等待卸载完成')

    // 立即刷新一次数据
    await fetchAgents()

    // 启动轮询以监控卸载进度
    startPolling()

    // 额外设置一个3秒后的刷新，确保状态更新
    setTimeout(() => {
      fetchAgents()
    }, 3000)

  } catch (error) {
    if (error !== 'cancel') {
      console.error('卸载Agent失败:', error)
      ElMessage.error(`卸载Agent失败: ${error.response?.data?.message || error.message || '未知错误'}`)
    }
  }
}

const handleDelete = async (row) => {
  try {
    await ElMessageBox.confirm(`确定要删除主机 ${row.hostName} 的Agent数据吗？此操作不可恢复！`, '确认删除', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })

    await cmdbAPI.deleteAgent(row.id)
    ElMessage.success('Agent数据删除成功')

    // 立即刷新数据
    await fetchAgents()

  } catch (error) {
    if (error !== 'cancel') {
      console.error('删除Agent失败:', error)
      ElMessage.error(`删除Agent失败: ${error.response?.data?.message || error.message || '未知错误'}`)
    }
  }
}

const handleRestart = async (row) => {
  try {
    await ElMessageBox.confirm(`确定要重启主机 ${row.hostName} 的Agent吗？`, '确认重启', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'info'
    })

    await cmdbAPI.restartAgent(row.hostId)
    ElMessage.success('Agent重启指令已发送')

    // 立即刷新数据
    await fetchAgents()

    // 启动轮询以监控重启后的状态变化
    startPolling()

  } catch (error) {
    if (error !== 'cancel') {
      console.error('重启Agent失败:', error)
      ElMessage.error(`重启Agent失败: ${error.response?.data?.message || error.message || '未知错误'}`)
    }
  }
}


const batchRedeploy = async () => {
  if (selectedAgents.value.length === 0) {
    ElMessage.warning('请选择要重新部署的Agent')
    return
  }

  try {
    const hostNames = selectedAgents.value.map(agent => agent.hostName).join(', ')
    await ElMessageBox.confirm(`确定要批量重新部署以下主机的Agent吗？\n${hostNames}`, '确认批量重新部署', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'info'
    })

    const hostIds = selectedAgents.value.map(agent => agent.hostId)
    await cmdbAPI.redeployAgent(hostIds)

    ElMessage.success(`已开始重新部署 ${selectedAgents.value.length} 台主机的Agent，请等待部署完成`)
    await fetchAgents()
    startPolling()
  } catch (error) {
    if (error !== 'cancel') {
      console.error('批量重新部署Agent失败:', error)
      ElMessage.error(`批量重新部署Agent失败: ${error.response?.data?.message || error.message || '未知错误'}`)
    }
  }
}

const batchUninstall = async () => {
  try {
    const hostNames = selectedAgents.value.map(agent => agent.hostName).join(', ')
    await ElMessageBox.confirm(`确定要批量卸载以下主机的Agent吗？\n${hostNames}`, '确认批量卸载', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })

    const hostIds = selectedAgents.value.map(agent => agent.hostId)
    await cmdbAPI.uninstallAgent(hostIds)

    ElMessage.success('批量卸载已启动，请等待卸载完成')

    // 立即刷新一次数据
    await fetchAgents()

    // 启动轮询以监控卸载进度
    startPolling()

    // 额外设置一个3秒后的刷新，确保状态更新
    setTimeout(() => {
      fetchAgents()
    }, 3000)

  } catch (error) {
    if (error !== 'cancel') {
      console.error('批量卸载失败:', error)
      ElMessage.error(`批量卸载失败: ${error.message || '未知错误'}`)
    }
  }
}

const viewAgentDetails = (row) => {
  currentAgent.value = row
  detailVisible.value = true
}

const handleSelectionChange = (selection) => {
  selectedAgents.value = selection
}

const handleQuery = () => {
  queryParams.page = 1
  fetchAgents()
}

const resetQuery = () => {
  queryParams.keyword = ''
  queryParams.onlineStatus = ''
  queryParams.version = ''
  queryParams.page = 1
  fetchAgents()
}

const handleSizeChange = (val) => {
  queryParams.pageSize = val
  queryParams.page = 1
  fetchAgents()
}

const handleCurrentChange = (val) => {
  queryParams.page = val
  fetchAgents()
}

const getStatusClass = (status) => {
  const statusClasses = {
    1: 'status-running',
    2: 'status-exception'
  }
  return statusClasses[status] || ''
}

const getStatusIcon = (status) => {
  const statusIcons = {
    1: CircleCheck,
    2: CircleClose
  }
  return statusIcons[status] || CircleClose
}

const getStatusText = (status) => {
  const statusTexts = {
    1: '在线',
    2: '离线'
  }
  return statusTexts[status] || '未知状态'
}

const formatTime = (timeStr) => {
  if (!timeStr || timeStr === '0001-01-01 00:00:00') return '无'
  const date = new Date(timeStr)
  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false
  }).replace(/\//g, '-')
}

const startPolling = () => {
  if (pollingTimer) {
    clearInterval(pollingTimer)
  }

  // 重置计数器
  pollingCounter = 0

  pollingTimer = setInterval(() => {
    const hasActiveOperations = agents.value.some(agent =>
      isAgentDeploying(agent)
    )

    if (hasActiveOperations) {
      // 有活跃操作时重置计数器并继续轮询
      pollingCounter = 0
      fetchAgents()
    } else {
      // 没有活跃操作时增加计数器
      pollingCounter++

      // 继续轮询几次以确保状态同步（比如卸载操作完成后的状态更新）
      if (pollingCounter <= MAX_POLLING_COUNT) {
        fetchAgents()
      } else {
        // 超过最大轮询次数后停止
        stopPolling()
      }
    }
  }, 3000)
}

const stopPolling = () => {
  if (pollingTimer) {
    clearInterval(pollingTimer)
    pollingTimer = null
  }
}

onMounted(() => {
  fetchAgentVersions()
  fetchAgents()
})

onUnmounted(() => {
  stopPolling()
})
</script>

<style scoped>
/* DESIGN.md: primitive -> semantic -> component tokens */
.agent-management {
  --agent-blue-500: #409eff;
  --agent-blue-600: #2563eb;
  --agent-green-500: #67c23a;
  --agent-yellow-400: #f3c66b;
  --agent-yellow-500: #e6a23c;
  --agent-red-500: #ef4444;
  --agent-slate-950: #0f172a;
  --agent-slate-800: #172033;
  --agent-slate-600: #53657d;
  --agent-slate-500: #64748b;
  --agent-slate-400: #7b8aa0;

  --agent-color-primary: var(--agent-blue-500);
  --agent-color-success: var(--agent-green-500);
  --agent-color-warning: var(--agent-yellow-500);
  --agent-color-danger: var(--agent-red-500);
  --agent-text-heading: var(--agent-slate-950);
  --agent-text-primary: var(--agent-slate-800);
  --agent-text-secondary: var(--agent-slate-600);
  --agent-text-muted: var(--agent-slate-500);
  --agent-bg-surface: #fff;
  --agent-bg-surface-soft: #fbfdff;
  --agent-bg-subtle: #f3f7fc;
  --agent-border: #dbe7f4;
  --agent-border-soft: #e7eef7;
  --agent-border-hover: #b9d4ff;

  --agent-card-radius: 12px;
  --agent-control-radius: 8px;
  --agent-dialog-radius: 15px;
  --agent-control-height: 30px;
  --agent-card-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
  --agent-hero-shadow: 0 10px 28px rgba(80, 124, 180, 0.1);

  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  align-content: start;
  gap: 8px;
  height: calc(100vh - 124px);
  min-height: 0;
  padding: 7px;
  overflow: hidden;
  color: var(--agent-text-primary);
  background: transparent;
}

:global(.home-container .el-main:has(.agent-management)) {
  padding: 10px;
}

.agent-hero-panel {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
  padding: 8px 12px;
  border: 1px solid var(--agent-border);
  border-radius: var(--agent-card-radius);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 251, 255, 0.95));
  box-shadow: var(--agent-hero-shadow);
}

.agent-hero-copy {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  gap: 8px;
  min-width: 0;
}

.agent-hero-icon {
  display: grid;
  flex: 0 0 42px;
  width: 42px;
  height: 42px;
  place-items: center;
  border: 0;
  background: transparent;
}

.agent-hero-icon img {
  width: 30px;
  height: 30px;
  object-fit: contain;
}

.agent-hero-content {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  gap: 10px;
  min-width: 0;
}

.agent-hero-content h1 {
  flex: 0 0 auto;
  margin: 0;
  color: var(--agent-text-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: 0;
  white-space: nowrap;
}

.agent-hero-divider {
  flex: 0 0 1px;
  width: 1px;
  height: 22px;
  background: var(--agent-border);
}

.agent-hero-content p {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  color: var(--agent-text-muted);
  font-size: 12px;
  line-height: 1.5;
  letter-spacing: 0;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-hero-metrics {
  display: flex;
  align-items: stretch;
  flex: 0 0 auto;
}

.agent-hero-metrics > div {
  display: grid;
  grid-template-columns: auto auto;
  align-items: baseline;
  gap: 6px;
  min-width: 82px;
  padding: 3px 12px;
  border-left: 1px solid var(--agent-border-soft);
}

.agent-hero-metrics span {
  color: var(--agent-slate-400);
  font-size: 11px;
  font-weight: 600;
}

.agent-hero-metrics strong {
  color: var(--agent-text-heading);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 18px;
  line-height: 1;
}

.agent-hero-metrics .metric-success {
  color: #2f9b61;
}

.agent-hero-metrics .metric-danger {
  color: #dc4c5a;
}

.agent-card {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  height: 100%;
  padding: 0;
  overflow: hidden;
  border: 1px solid var(--agent-border);
  border-radius: var(--agent-card-radius);
  background: rgba(255, 255, 255, 0.97);
  box-shadow: var(--agent-card-shadow);
}

.agent-card::before {
  display: none;
}

.agent-toolbar {
  flex: 0 0 auto;
  padding: 8px 10px;
  border-bottom: 1px solid var(--agent-border-soft);
  background: var(--agent-bg-surface);
}

.agent-toolbar-main {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  min-width: 0;
}

.search-form {
  display: flex;
  align-items: center;
  flex: 0 0 100%;
  flex-wrap: nowrap;
  gap: 8px;
  min-width: 0;
}

.search-form :deep(.el-form-item) {
  width: auto;
  margin: 0;
}

.search-form :deep(.el-form-item__label) {
  height: var(--agent-control-height);
  padding-right: 7px;
  color: var(--agent-text-secondary);
  font-size: 12px;
  font-weight: 700;
  line-height: var(--agent-control-height);
}

.search-keyword-item {
  flex: 0 0 272px;
  width: 272px;
}

.search-status-item {
  flex: 0 0 168px;
  width: 168px;
}

.search-version-item {
  flex: 0 0 168px;
  width: 168px;
}

.search-form :deep(.el-form-item__content) {
  min-width: 0;
}

.search-keyword-item :deep(.el-form-item__content),
.search-status-item :deep(.el-form-item__content),
.search-version-item :deep(.el-form-item__content) {
  flex: 1 1 auto;
}

.search-form :deep(.el-input),
.search-form :deep(.el-select) {
  width: 100%;
  min-width: 0;
}

.toolbar-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.search-form-actions :deep(.el-button + .el-button),
.agent-toolbar :deep(.el-button + .el-button) {
  margin-left: 0;
}

.toolbar-actions {
  flex: 0 1 auto;
  width: auto;
  min-width: 0;
}

.toolbar-actions :deep(.el-button) {
  flex: 0 0 auto;
  white-space: nowrap;
}

:global(.agent-management .toolbar-actions > *) {
  flex: 0 0 auto;
}

.table-section {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  margin: 8px 10px 0;
  padding: 0;
  overflow: hidden;
  border: 1px solid #e3ebf5;
  border-radius: var(--agent-control-radius);
  background: var(--agent-bg-surface);
  box-shadow: none;
}

.agent-table {
  flex: 1 1 auto;
  min-height: 0;
  height: 100%;
  border: 0;
  border-radius: var(--agent-control-radius);
  background: var(--agent-bg-surface);
}

.agent-management :deep(.agent-table.el-table) {
  --el-table-border-color: #e3ebf5;
  --el-table-header-bg-color: var(--agent-bg-subtle);
  --el-table-tr-bg-color: #fff;
  --el-table-row-hover-bg-color: #f8fafc;
  --el-table-current-row-bg-color: #eef6ff;
  --el-table-bg-color: #fff;
  --el-table-text-color: var(--agent-text-secondary);
  --el-table-header-text-color: #475569;
  overflow: hidden;
  border: 0;
  font-size: 12px;
}

.agent-management :deep(.agent-table .el-table__body-wrapper),
.agent-management :deep(.agent-table .el-scrollbar__wrap) {
  overflow-x: auto !important;
}

.agent-management :deep(.agent-table .el-table__inner-wrapper::before) {
  display: none;
}

.agent-management :deep(.agent-table th.el-table__cell) {
  height: 34px;
  padding: 5px 0;
  color: #475569;
  background: var(--agent-bg-subtle) !important;
  font-size: 12px;
  font-weight: 700 !important;
}

.agent-management :deep(.agent-table td.el-table__cell) {
  padding: 5px 0;
  font-size: 12px;
}

.agent-management :deep(.agent-table td.el-table__cell),
.agent-management :deep(.agent-table th.el-table__cell.is-leaf) {
  border-bottom-color: #eef3f9;
}

.agent-management :deep(.agent-table .el-table__row),
.agent-management :deep(.agent-table .el-table__row:hover) {
  box-shadow: none;
  transform: none;
}

.agent-management :deep(.agent-table .el-table__row:hover > td.el-table__cell),
.agent-management :deep(.agent-table .el-table__fixed-right .el-table__row:hover > .el-table__cell) {
  background: #f8fafc !important;
}

.cell-with-icon {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.cell-icon {
  flex: 0 0 auto;
  width: 16px;
  height: 16px;
}

.host-name-cell {
  display: flex;
  width: 100%;
  max-width: 100%;
  min-width: 0;
  overflow: hidden;
  flex-wrap: nowrap;
  white-space: nowrap;
}

.host-name-text {
  display: block;
  flex: 1 1 auto;
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
  color: var(--agent-text-primary);
  font-size: 12px;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.status-chip {
  gap: 4px;
  padding: 3px 7px;
  font-size: 11px;
}

.status-running {
  color: #2f8f62;
  background: #edf9f3;
}

.status-exception {
  color: #d84f5c;
  background: #fff0f1;
}

.text-muted {
  color: var(--agent-text-muted);
}

.operation-links {
  gap: 11px;
}

.agent-management :deep(.operation-links .el-button) {
  min-height: 24px;
  height: auto;
  padding: 0;
  border: 0 !important;
  border-radius: 0;
  font-size: 12px;
  background: transparent !important;
  box-shadow: none !important;
  transform: none !important;
}

.agent-management :deep(.operation-links .el-button > span) {
  font-size: 12px;
}

.agent-management :deep(.operation-links .el-button .el-icon),
.agent-management .operation-links img {
  display: none;
}

.agent-management :deep(.operation-links .el-button:hover),
.agent-management :deep(.operation-links .el-button:focus-visible) {
  background: transparent !important;
  box-shadow: none !important;
  transform: none !important;
}

.pagination-section {
  display: flex;
  flex: 0 0 auto;
  justify-content: flex-end;
  margin: 0;
  padding: 8px 10px 10px;
  border-top: 0;
}

.agent-management :deep(.el-pagination .btn-prev),
.agent-management :deep(.el-pagination .btn-next),
.agent-management :deep(.el-pagination .el-pager li) {
  border-radius: 7px;
  color: var(--agent-text-secondary);
  background: #f8fafc;
}

.agent-management :deep(.el-pagination .el-pager li.is-active) {
  color: #fff;
  background: var(--agent-blue-600);
}

.agent-management :deep(.el-input__wrapper),
.agent-management :deep(.el-select__wrapper),
.agent-management :deep(.el-textarea__inner),
.agent-management :deep(.el-input-number .el-input__wrapper) {
  min-height: var(--agent-control-height);
  border: 0;
  border-radius: var(--agent-control-radius);
  background: var(--agent-bg-surface-soft);
  box-shadow: 0 0 0 1px #d9e5f2 inset !important;
  transition: background-color 160ms ease, box-shadow 160ms ease;
}

.agent-management :deep(.el-input__wrapper:hover),
.agent-management :deep(.el-select__wrapper:hover),
.agent-management :deep(.el-textarea__inner:hover) {
  box-shadow: 0 0 0 1px var(--agent-border-hover) inset !important;
}

.agent-management :deep(.el-input__wrapper.is-focus),
.agent-management :deep(.el-select__wrapper.is-focused),
.agent-management :deep(.el-textarea__inner:focus) {
  background: #fff;
  box-shadow: 0 0 0 1px var(--agent-color-primary) inset !important;
}

.agent-management :deep(.el-button) {
  border-radius: var(--agent-control-radius);
  font-weight: 600;
  transition: color 160ms ease, background-color 160ms ease, border-color 160ms ease, box-shadow 160ms ease;
}

.agent-management :deep(.agent-toolbar .el-button) {
  height: var(--agent-control-height);
  min-height: var(--agent-control-height);
  padding: 5px 10px;
  font-size: 12px;
}

.agent-management :deep(.el-button:not(.is-link):not(.is-text):hover) {
  box-shadow: 0 5px 12px rgba(64, 158, 255, 0.12) !important;
  transform: none !important;
}

.agent-management :deep(.el-button--primary:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: var(--agent-color-primary);
  color: #fff;
  background: var(--agent-color-primary);
}

.agent-management :deep(.el-button--success:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: var(--agent-color-success);
  color: #fff;
  background: var(--agent-color-success);
}

.agent-management :deep(.el-button--warning:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: var(--agent-color-warning);
  color: #fff;
  background: var(--agent-color-warning);
}

.agent-management :deep(.el-button--danger:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: var(--agent-color-danger);
  color: #fff;
  background: var(--agent-color-danger);
}

:global(.agent-detail-dialog-overlay) {
  background: rgba(15, 23, 42, 0.22) !important;
}

:global(.agent-detail-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  width: min(760px, calc(100vw - 24px)) !important;
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.agent-detail-dialog .el-dialog__header) {
  position: relative;
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: linear-gradient(90deg, #fff6e7 0%, #f7fbff 36%, #ecf7ff 100%);
}

:global(.agent-detail-dialog .el-dialog__header::before) {
  content: '';
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 3px;
  background: linear-gradient(90deg, #f5a524 0%, #62a8ff 48%, #55d6be 100%);
}

:global(.agent-detail-dialog .el-dialog__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.agent-detail-dialog .el-dialog__headerbtn) {
  top: 10px;
  right: 12px;
}

:global(.agent-detail-dialog .el-dialog__body) {
  max-height: calc(100vh - 190px);
  padding: 16px 18px;
  overflow-y: auto;
}

:global(.agent-detail-dialog .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
}

:global(.agent-detail-dialog .el-descriptions__label) {
  width: 104px;
  color: #53657d !important;
  background: #f3f7fc !important;
  font-size: 12px;
  font-weight: 700;
}

:global(.agent-detail-dialog .el-descriptions__content) {
  color: #172033 !important;
  font-size: 12px;
}

:global(.agent-detail-dialog .el-dialog__footer .el-button) {
  min-width: 78px;
  min-height: 32px;
  border-radius: 8px;
}

.offline-agent-dialog :deep(.el-dialog__header) {
  border-bottom: 1px solid var(--agent-border-soft);
}

.offline-agent-dialog :deep(.el-dialog__body) {
  padding: 16px 20px 12px;
}

.offline-asset-dialog :deep(.el-dialog__body) {
  padding: 16px 20px 12px;
}

.offline-asset-status-list {
  margin: 16px 0;
}

.offline-asset-status-row {
  padding: 12px 0;
  border-bottom: 1px solid var(--agent-border-soft);
}

.offline-asset-status-row:last-child {
  border-bottom: 0;
}

.offline-asset-status-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--agent-text-primary);
  font-size: 13px;
  font-weight: 600;
}

.offline-asset-status-file {
  margin-top: 6px;
  color: var(--agent-text-secondary);
  font-size: 12px;
  line-height: 1.5;
  word-break: break-all;
}

.offline-asset-status-file.is-missing {
  color: var(--el-color-danger);
}

.offline-asset-upload {
  display: inline-flex;
}

.offline-asset-upload-hint {
  margin-top: 8px;
  color: var(--agent-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.offline-agent-tip {
  margin-bottom: 18px;
}

.offline-agent-form :deep(.el-form-item) {
  margin-bottom: 16px;
}

.offline-agent-form :deep(.el-form-item__label) {
  padding-bottom: 6px;
  color: var(--agent-text-primary);
  font-size: 13px;
  font-weight: 600;
}

.offline-agent-select {
  width: 100%;
}

.offline-agent-upload {
  display: inline-flex;
}

.offline-agent-upload-hint,
.offline-agent-generated-config {
  margin-top: 8px;
  color: var(--agent-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.offline-agent-generated-config {
  padding: 10px 12px;
  border: 1px solid var(--agent-border-soft);
  border-radius: 8px;
  background: var(--agent-bg-subtle);
}

.offline-agent-hosts {
  padding: 10px 12px;
  border: 1px solid var(--agent-border-soft);
  border-radius: 8px;
  color: var(--agent-text-secondary);
  background: var(--agent-bg-subtle);
  font-size: 12px;
  line-height: 1.5;
}

:global(.offline-agent-dialog .el-dialog__footer) {
  padding: 12px 20px 16px;
  border-top: 1px solid var(--agent-border-soft);
}

@media (max-width: 1480px) {
  .agent-toolbar-main {
    align-items: flex-start;
  }
}

@media (max-width: 920px) {
  .search-form {
    align-items: stretch;
    flex-wrap: wrap;
    min-width: 0;
  }
}

@media (max-width: 768px) {
  .agent-management {
    grid-template-rows: auto auto;
    height: auto;
    padding: 6px;
    overflow: visible;
  }

  .agent-hero-panel {
    align-items: stretch;
    flex-direction: column;
  }

  .agent-hero-content {
    align-items: flex-start;
    flex-direction: column;
    gap: 3px;
  }

  .agent-hero-divider {
    display: none;
  }

  .agent-hero-content p {
    width: 100%;
    white-space: normal;
  }

  .agent-hero-metrics {
    border-top: 1px solid var(--agent-border-soft);
  }

  .agent-hero-metrics > div {
    flex: 1;
    justify-content: center;
    min-width: 0;
    padding: 7px 4px 2px;
    border-left: 0;
  }

  .agent-card {
    height: auto;
    border-radius: var(--agent-card-radius);
  }

  .search-form,
  .toolbar-actions,
  .search-keyword-item,
  .search-status-item,
  .search-version-item {
    width: 100%;
  }

  .toolbar-actions {
    min-width: 0;
  }

  .search-form :deep(.el-form-item) {
    width: 100%;
  }

  .search-form :deep(.el-form-item__content) {
    flex: 1;
  }

  .toolbar-actions {
    align-items: stretch;
    flex-direction: column;
  }

  .toolbar-actions :deep(.el-button) {
    width: 100%;
  }

  .table-section {
    min-height: 460px;
  }

  .pagination-section {
    overflow-x: auto;
  }
}
</style>
