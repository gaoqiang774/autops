<template>
  <div class="host-detail-container" v-loading="loading">
    <div class="page-header">
      <div class="header-content">
        <el-button :icon="ArrowLeft" class="back-btn" @click="goBack">返回</el-button>
        <h2 class="page-title">{{ hostInfo.hostName || hostInfo.name || '主机详情' }}</h2>
        <el-tag :type="getStatusTagType(hostInfo.status)" size="large">
          {{ getStatusText(hostInfo.status) }}
        </el-tag>
      </div>
    </div>

    <div class="tab-container">
      <el-tabs v-model="activeTab" class="detail-tabs">
        <el-tab-pane label="主机详情" name="details" />
        <el-tab-pane label="监控图表" name="monitor" />
        <el-tab-pane label="告警记录" name="alert" />
      </el-tabs>
    </div>

    <div v-show="activeTab === 'details'" class="details-content">
      <div class="left-section">
        <el-card shadow="hover" class="info-card">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon"><Monitor /></el-icon>
              <span class="card-title">主机信息</span>
            </div>
          </template>

          <div class="info-content">
            <div class="info-row">
              <span class="label">实例名称</span>
              <span class="value">{{ hostInfo.name || '-' }}</span>
            </div>
            <div class="info-row">
              <span class="label">主机名称</span>
              <span class="value">{{ hostInfo.hostName || '-' }}</span>
            </div>
            <div class="info-row">
              <span class="label">业务组</span>
              <span class="value">{{ hostInfo.groupName || '-' }}</span>
            </div>
            <div class="info-row">
              <span class="label">内网 IP</span>
              <span class="value">{{ hostInfo.privateIp || '-' }}</span>
            </div>
            <div class="info-row">
              <span class="label">公网 IP</span>
              <span class="value">{{ hostInfo.publicIp || '-' }}</span>
            </div>
            <div class="info-row">
              <span class="label">操作系统</span>
              <span class="value">{{ hostInfo.os || '-' }}</span>
            </div>
            <div class="info-row">
              <span class="label">配置</span>
              <span class="value">{{ formatSpec(hostInfo) }}</span>
            </div>
            <div class="info-row">
              <span class="label">运行时长</span>
              <span class="value">{{ monitorData.uptimeStr || '-' }}</span>
            </div>
            <div class="info-row">
              <span class="label">进程总数</span>
              <span class="value">{{ monitorData.processCount ?? 0 }}</span>
            </div>
            <div class="info-row">
              <span class="label">打开文件数</span>
              <span class="value">{{ monitorData.openFiles ?? 0 }}</span>
            </div>
          </div>
        </el-card>
      </div>

      <div class="right-section">
        <el-card shadow="hover" class="metric-card cpu-card">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon"><Cpu /></el-icon>
              <span class="card-title">CPU 详情</span>
              <el-button
                size="small"
                :icon="Refresh"
                :loading="refreshing"
                circle
                @click="refreshSummaryData"
              />
            </div>
          </template>

          <div class="metric-content">
            <div class="metric-main">
              <div class="metric-value-large">{{ formatPercent(monitorData.cpu?.total) }}</div>
              <div class="metric-label">总 CPU 使用率</div>
            </div>
            <el-divider />
            <div class="metric-details">
              <div class="detail-item">
                <span class="detail-label">用户态</span>
                <span class="detail-value">{{ formatPercent(monitorData.cpu?.user) }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">内核态</span>
                <span class="detail-value">{{ formatPercent(monitorData.cpu?.system) }}</span>
              </div>
            </div>
          </div>
        </el-card>

        <el-card shadow="hover" class="metric-card memory-card">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon"><Memo /></el-icon>
              <span class="card-title">内存详情</span>
            </div>
          </template>

          <div class="metric-content">
            <div class="metric-main">
              <div class="metric-value-large">{{ formatPercent(monitorData.memory?.usageRate) }}</div>
              <div class="metric-label">内存使用率</div>
            </div>
            <el-divider />
            <div class="metric-details">
              <div class="detail-item">
                <span class="detail-label">可用内存</span>
                <span class="detail-value">{{ monitorData.memory?.freeStr || '-' }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">已用内存</span>
                <span class="detail-value">{{ monitorData.memory?.usedStr || '-' }}</span>
              </div>
            </div>
          </div>
        </el-card>

        <el-card shadow="hover" class="metric-card disk-card">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon"><HardDrive /></el-icon>
              <span class="card-title">磁盘详情</span>
            </div>
          </template>

          <div class="metric-content">
            <div class="metric-main">
              <div class="metric-value-large">{{ formatPercent(monitorData.disk?.usageRate) }}</div>
              <div class="metric-label">磁盘使用率</div>
            </div>
            <el-divider />
            <div class="metric-details">
              <div class="detail-item">
                <span class="detail-label">读取速度</span>
                <span class="detail-value">{{ formatRate(diskReadRate) }}</span>
              </div>
              <div class="detail-item">
                <span class="detail-label">写入速度</span>
                <span class="detail-value">{{ formatRate(diskWriteRate) }}</span>
              </div>
            </div>
          </div>
        </el-card>

        <el-card shadow="hover" class="metric-card connection-card">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon"><Connection /></el-icon>
              <span class="card-title">连接数</span>
            </div>
          </template>

          <div class="metric-content">
            <div class="connection-grid">
              <div class="connection-item">
                <div class="connection-value">{{ tcpConnections }}</div>
                <div class="connection-label">TCP 连接</div>
              </div>
              <div class="connection-item">
                <div class="connection-value">{{ udpConnections }}</div>
                <div class="connection-label">UDP 连接</div>
              </div>
              <div class="connection-item">
                <div class="connection-value">{{ totalConnections }}</div>
                <div class="connection-label">总连接数</div>
              </div>
            </div>
          </div>
        </el-card>

        <el-card shadow="hover" class="metric-card load-card">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon"><TrendCharts /></el-icon>
              <span class="card-title">系统负载</span>
            </div>
          </template>

          <div class="metric-content">
            <div class="load-grid">
              <div class="load-item">
                <div class="load-value">{{ formatNumber(load1, 2) }}</div>
                <div class="load-label">1 分钟</div>
              </div>
              <div class="load-item">
                <div class="load-value">{{ formatNumber(load5, 2) }}</div>
                <div class="load-label">5 分钟</div>
              </div>
              <div class="load-item">
                <div class="load-value">{{ formatNumber(load15, 2) }}</div>
                <div class="load-label">15 分钟</div>
              </div>
            </div>
          </div>
        </el-card>

        <el-card shadow="hover" class="metric-card network-card">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon"><Link /></el-icon>
              <span class="card-title">网络流量</span>
            </div>
          </template>

          <div class="metric-content">
            <div class="network-grid">
              <div class="network-item">
                <el-icon class="network-icon download"><Download /></el-icon>
                <div class="network-info">
                  <div class="network-value">{{ formatRate(networkReceiveRate) }}</div>
                  <div class="network-label">接收流量</div>
                </div>
              </div>
              <div class="network-item">
                <el-icon class="network-icon upload"><Upload /></el-icon>
                <div class="network-info">
                  <div class="network-value">{{ formatRate(networkSendRate) }}</div>
                  <div class="network-label">发送流量</div>
                </div>
              </div>
            </div>
          </div>
        </el-card>
      </div>

      <div class="bottom-section">
        <el-card shadow="hover" class="services-card">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon"><List /></el-icon>
              <span class="card-title">系统服务详情 (Top 7)</span>
            </div>
          </template>

          <el-table :data="topServices" stripe style="width: 100%">
            <el-table-column prop="name" label="服务名称" min-width="180" show-overflow-tooltip />
            <el-table-column label="CPU" width="120" align="center">
              <template #default="{ row }">
                <el-tag :type="getCpuTagType(row.cpu)" size="small">
                  {{ formatPercent(row.cpu) }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="内存" width="120" align="center">
              <template #default="{ row }">
                <el-tag :type="getMemoryTagType(row.memory)" size="small">
                  {{ formatPercent(row.memory) }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <el-card shadow="hover" class="process-card cpu-top-card">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon"><Cpu /></el-icon>
              <span class="card-title">CPU Top5 进程</span>
            </div>
          </template>

          <el-table :data="topProcesses.topCPU" stripe style="width: 100%">
            <el-table-column prop="pid" label="PID" width="100" align="center">
              <template #default="{ row }">
                <el-tag type="warning" size="small">{{ row.pid }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="name" label="进程名称" min-width="180" show-overflow-tooltip />
            <el-table-column label="CPU" width="120" align="center">
              <template #default="{ row }">
                <el-tag :type="getCpuTagType(row.cpuPercent)" size="small">
                  {{ formatPercent(row.cpuPercent) }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <el-card shadow="hover" class="process-card memory-top-card">
          <template #header>
            <div class="card-header">
              <el-icon class="card-icon"><Memo /></el-icon>
              <span class="card-title">内存 Top5 进程</span>
            </div>
          </template>

          <el-table :data="topProcesses.topMemory" stripe style="width: 100%">
            <el-table-column prop="pid" label="PID" width="100" align="center">
              <template #default="{ row }">
                <el-tag type="warning" size="small">{{ row.pid }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="name" label="进程名称" min-width="180" show-overflow-tooltip />
            <el-table-column label="内存" width="120" align="center">
              <template #default="{ row }">
                <el-tag :type="getMemoryTagType(row.memPercent)" size="small">
                  {{ formatPercent(row.memPercent) }}
                </el-tag>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </div>
    </div>

    <div v-show="activeTab === 'monitor'" class="monitor-content">
      <HostMonitorCharts :hostId="hostId" :visible="activeTab === 'monitor'" />
    </div>

    <div v-show="activeTab === 'alert'" class="alert-content">
      <el-card shadow="hover" class="alert-card">
        <template #header>
          <div class="card-header">
            <el-icon class="card-icon"><Warning /></el-icon>
            <span class="card-title">告警记录</span>
          </div>
        </template>

        <div class="developing-wrap">
          <el-empty description="开发中，告警记录 API 暂未接入" />
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import {
  ArrowLeft,
  Connection,
  Cpu,
  Download,
  HardDrive,
  Link,
  List,
  Memo,
  Monitor,
  Refresh,
  TrendCharts,
  Upload,
  Warning
} from '@element-plus/icons-vue'
import api from '@/api'
import HostMonitorCharts from './HostMonitorCharts.vue'

const route = useRoute()
const router = useRouter()

const hostId = computed(() => route.params.id)
const activeTab = ref('details')
const loading = ref(true)
const refreshing = ref(false)
const hostInfo = ref({})
const monitorData = ref({})
const topProcesses = ref({ topCPU: [], topMemory: [] })

let refreshTimer = null

const toNumber = value => {
  const num = Number(value)
  return Number.isFinite(num) ? num : 0
}

const topServices = computed(() => {
  const services = Array.isArray(monitorData.value.services) ? monitorData.value.services : []
  return services.slice(0, 7)
})

const diskReadRate = computed(() =>
  monitorData.value.disk?.diskThroughput?.readKbPerSec ?? monitorData.value.disk?.readKbPerSec ?? 0
)

const diskWriteRate = computed(() =>
  monitorData.value.disk?.diskThroughput?.writeKbPerSec ?? monitorData.value.disk?.writeKbPerSec ?? 0
)

const tcpConnections = computed(() =>
  monitorData.value.connections?.tcp ?? monitorData.value.tcpConnections ?? 0
)

const udpConnections = computed(() =>
  monitorData.value.connections?.udp ?? monitorData.value.udpConnections ?? 0
)

const totalConnections = computed(() =>
  monitorData.value.connections?.total ?? (toNumber(tcpConnections.value) + toNumber(udpConnections.value))
)

const load1 = computed(() => monitorData.value.load?.load1 ?? monitorData.value.load?.load1min ?? 0)
const load5 = computed(() => monitorData.value.load?.load5 ?? monitorData.value.load?.load5min ?? 0)
const load15 = computed(() => monitorData.value.load?.load15 ?? monitorData.value.load?.load15min ?? 0)

const networkReceiveRate = computed(() =>
  monitorData.value.network?.receiveKbPerSec ?? monitorData.value.network?.networkReceive ?? 0
)

const networkSendRate = computed(() =>
  monitorData.value.network?.sendKbPerSec ?? monitorData.value.network?.networkSend ?? 0
)

const fetchHostInfo = async () => {
  if (!hostId.value) return

  try {
    const { data: res } = await api.getCmdbHostById(hostId.value)
    if (res.code === 200) {
      hostInfo.value = res.data || {}
      return
    }
    ElMessage.error(res.message || '获取主机信息失败')
  } catch (error) {
    console.error('fetch host info failed:', error)
    ElMessage.error('获取主机信息失败')
  }
}

const fetchMonitorData = async () => {
  if (!hostId.value) return

  try {
    const { data: res } = await api.getHostCompleteMonitorData(hostId.value)
    if (res.code === 200) {
      monitorData.value = res.data || {}
    }
  } catch (error) {
    console.error('fetch monitor data failed:', error)
  }
}

const fetchTopProcesses = async () => {
  if (!hostId.value) return

  try {
    const { data: res } = await api.getHostTopProcesses(hostId.value)
    if (res.code === 200) {
      topProcesses.value = {
        topCPU: res.data?.topCPU || [],
        topMemory: res.data?.topMemory || []
      }
    }
  } catch (error) {
    console.error('fetch top processes failed:', error)
  }
}

const initData = async () => {
  loading.value = true
  await Promise.all([fetchHostInfo(), fetchMonitorData(), fetchTopProcesses()])
  loading.value = false
}

const refreshSummaryData = async () => {
  refreshing.value = true
  await Promise.all([fetchMonitorData(), fetchTopProcesses()])
  refreshing.value = false
}

const goBack = () => {
  router.back()
}

const formatSpec = host => {
  const cpu = host.cpu ? `${host.cpu} 核` : '-'
  const memory = host.memory ? `${host.memory} GB` : '-'
  const disk = host.disk ? `${host.disk} GB` : '-'
  return `${cpu} / ${memory} / ${disk}`
}

const formatPercent = value => {
  const num = Number(value)
  if (!Number.isFinite(num)) return '0.0%'
  return `${num.toFixed(1)}%`
}

const formatRate = value => {
  const num = Number(value)
  if (!Number.isFinite(num)) return '0.00 KB/s'
  return `${num.toFixed(2)} KB/s`
}

const formatNumber = (value, digits = 1) => {
  const num = Number(value)
  if (!Number.isFinite(num)) return '0.00'
  return num.toFixed(digits)
}

const getStatusText = status => {
  const map = {
    1: '正常',
    2: '未认证',
    3: '认证失败'
  }
  return map[status] || '未知'
}

const getStatusTagType = status => {
  const map = {
    1: 'success',
    2: 'warning',
    3: 'danger'
  }
  return map[status] || 'info'
}

const getCpuTagType = value => {
  const num = Number(value)
  if (!Number.isFinite(num) || num < 50) return 'success'
  if (num < 80) return 'warning'
  return 'danger'
}

const getMemoryTagType = value => {
  const num = Number(value)
  if (!Number.isFinite(num) || num < 60) return 'success'
  if (num < 80) return 'warning'
  return 'danger'
}

const startAutoRefresh = () => {
  stopAutoRefresh()
  refreshTimer = setInterval(() => {
    fetchMonitorData()
    fetchTopProcesses()
  }, 30000)
}

const stopAutoRefresh = () => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
}

onMounted(() => {
  if (!hostId.value) {
    ElMessage.error('缺少主机 ID')
    return
  }

  initData()
  startAutoRefresh()
})

watch(
  () => hostId.value,
  async newId => {
    if (!newId) return
    activeTab.value = 'details'
    await initData()
    startAutoRefresh()
  }
)

onBeforeUnmount(() => {
  stopAutoRefresh()
})
</script>

<style scoped>
.host-detail-container {
  min-height: 100vh;
  background: #f5f7fa;
}

.page-header {
  position: relative;
  z-index: 10;
  padding: 16px 24px;
  background: #ffffff;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
}

.header-content {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
}

.back-btn {
  position: absolute;
  left: 0;
  border: none;
  color: #ffffff;
  background: #67c23a;
}

.back-btn:hover {
  color: #ffffff;
  background: #5daf34;
}

.page-title {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
  color: #303133;
}

.tab-container {
  position: relative;
  z-index: 9;
  padding: 0 24px;
  margin-bottom: 16px;
  background: #ffffff;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.detail-tabs {
  max-width: 100%;
}

.details-content,
.monitor-content,
.alert-content {
  padding: 0 20px 20px;
}

.details-content {
  display: grid;
  grid-template-columns: 400px 1fr;
  grid-template-rows: auto auto;
  gap: 12px;
}

.left-section {
  grid-column: 1;
  grid-row: 1;
}

.right-section {
  display: grid;
  grid-column: 2;
  grid-row: 1;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  align-items: start;
}

.bottom-section {
  display: grid;
  grid-column: 1 / -1;
  grid-row: 2;
  grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
  gap: 12px;
}

.info-card,
.metric-card,
.services-card,
.process-card,
.alert-card {
  overflow: hidden;
  position: relative;
  border: 1px solid #e4e7ed;
  border-radius: 12px;
  background: #ffffff;
  transition: all 0.3s ease;
}

.info-card:hover,
.metric-card:hover,
.services-card:hover,
.process-card:hover,
.alert-card:hover {
  transform: translateY(-4px);
  box-shadow: 0 8px 28px rgba(102, 126, 234, 0.18);
}

.metric-card :deep(.el-card__header),
.services-card :deep(.el-card__header),
.process-card :deep(.el-card__header),
.alert-card :deep(.el-card__header),
.info-card :deep(.el-card__header) {
  padding: 12px 16px;
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.05) 0%, rgba(118, 75, 162, 0.05) 100%);
  border-bottom: 1px solid rgba(102, 126, 234, 0.1);
}

.metric-card :deep(.el-card__body),
.services-card :deep(.el-card__body),
.process-card :deep(.el-card__body),
.info-card :deep(.el-card__body),
.alert-card :deep(.el-card__body) {
  padding: 12px 16px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.card-icon {
  font-size: 16px;
  color: #667eea;
}

.card-title {
  flex: 1;
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}

.info-card {
  height: 100%;
}

.info-content {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.info-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  border-radius: 8px;
  background: #f8fafc;
}

.info-row .label {
  font-size: 13px;
  color: #909399;
  white-space: nowrap;
}

.info-row .value {
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  color: #303133;
  text-align: right;
  word-break: break-all;
}

.metric-content {
  padding: 2px 0;
}

.metric-main {
  padding: 8px 0;
  text-align: center;
}

.metric-value-large {
  margin-bottom: 4px;
  font-size: 32px;
  font-weight: 700;
  line-height: 1;
  color: #303133;
}

.metric-label {
  font-size: 13px;
  color: #909399;
}

.metric-details {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.detail-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border: 1px solid rgba(102, 126, 234, 0.1);
  border-radius: 8px;
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.05) 0%, rgba(118, 75, 162, 0.05) 100%);
}

.detail-label {
  font-size: 12px;
  color: #909399;
}

.detail-value {
  font-size: 13px;
  font-weight: 600;
  color: #303133;
}

.connection-grid,
.load-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
  padding: 8px 0;
}

.connection-item,
.load-item {
  padding: 10px 8px;
  text-align: center;
  border: 1px solid rgba(102, 126, 234, 0.1);
  border-radius: 8px;
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.05) 0%, rgba(118, 75, 162, 0.05) 100%);
}

.connection-value,
.load-value {
  margin-bottom: 4px;
  font-size: 22px;
  font-weight: 700;
  color: #303133;
}

.connection-label,
.load-label {
  font-size: 12px;
  color: #909399;
}

.network-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 8px;
  padding: 8px 0;
}

.network-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid rgba(102, 126, 234, 0.1);
  border-radius: 8px;
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.05) 0%, rgba(118, 75, 162, 0.05) 100%);
}

.network-icon {
  font-size: 24px;
}

.network-icon.download {
  color: #67c23a;
}

.network-icon.upload {
  color: #409eff;
}

.network-info {
  flex: 1;
  min-width: 0;
}

.network-value {
  margin-bottom: 2px;
  font-size: 18px;
  font-weight: 700;
  color: #303133;
}

.network-label {
  font-size: 12px;
  color: #909399;
}

.developing-wrap {
  padding: 24px 0;
}

@media (max-width: 1200px) {
  .details-content {
    grid-template-columns: 1fr;
    grid-template-rows: auto auto auto;
  }

  .left-section {
    grid-column: 1;
    grid-row: 1;
  }

  .right-section {
    grid-column: 1;
    grid-row: 2;
  }

  .bottom-section {
    grid-column: 1;
    grid-row: 3;
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 768px) {
  .page-header {
    padding: 12px 16px;
  }

  .header-content {
    justify-content: flex-start;
    flex-wrap: wrap;
    padding-top: 36px;
  }

  .back-btn {
    top: 0;
    left: 0;
  }

  .page-title {
    font-size: 18px;
  }

  .tab-container {
    padding: 0 16px;
  }

  .details-content,
  .monitor-content,
  .alert-content {
    padding: 0 16px 16px;
  }

  .right-section {
    grid-template-columns: 1fr;
  }

  .bottom-section,
  .network-grid,
  .load-grid,
  .connection-grid {
    grid-template-columns: 1fr;
  }

  .metric-value-large {
    font-size: 28px;
  }
}
</style>
