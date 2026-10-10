<template>
  <div class="table-section">
    <el-table v-loading="loading" :data="hostListWithMonitor" stripe style="width: 100%" height="100%" class="host-table"
      @selection-change="handleSelectionChange" ref="hostTableRef">
      <!-- 复选框列 -->
      <el-table-column type="selection" width="55" />
      <el-table-column label="ID" prop="id" v-if="false" />
      <el-table-column label="主机名称" width="200">
        <template v-slot="scope">
          <div class="host-name-cell-wrapper" @mouseenter="showCopyIcon($event)" @mouseleave="hideCopyIcon">
            <el-tooltip :content="scope.row.hostName" placement="top" :disabled="!isTextOverflow(scope.row.hostName)"
              :popper-options="{ modifiers: [{ name: 'offset', options: { offset: [0, 8] } }] }">
              <div class="host-name-cell">
                <img :src="getHostSystemIcon(scope.row)" :alt="getHostSystemAlt(scope.row)"
                  style="height: 20px; object-fit: contain; flex-shrink: 0;" />
                <el-link type="primary" @click="$emit('show-detail', scope.row)">{{ scope.row.hostName }}</el-link>
              </div>
            </el-tooltip>
            <el-icon class="copy-icon" @click="copyToClipboard(scope.row.hostName, '主机名称', $event)">
              <DocumentCopy />
            </el-icon>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="IP地址" width="130" show-overflow-tooltip>
        <template v-slot="scope">
          <div class="ip-cell">
            <div v-if="scope.row.publicIp" class="ip-row public-ip">
              <img src="@/assets/image/公.svg" alt="公网" class="ip-icon" />
              <span>{{ scope.row.publicIp || '无公网IP' }}</span>
              <el-icon class="copy-icon ip-copy-icon" @click="copyToClipboard(scope.row.publicIp, '公网IP', $event)">
                <DocumentCopy />
              </el-icon>
            </div>
            <div v-if="scope.row.privateIp" class="ip-row private-ip">
              <img src="@/assets/image/内.svg" alt="内网" class="ip-icon" />
              <span>{{ scope.row.privateIp || '无内网IP' }}</span>
              <el-icon class="copy-icon ip-copy-icon" @click="copyToClipboard(scope.row.privateIp, '内网IP', $event)">
                <DocumentCopy />
              </el-icon>
            </div>
          </div>
        </template>
      </el-table-column>
      <el-table-column width="110" align="center">
        <template #header>
          <div class="metric-header-cell">
            <span>CPU</span>
            <span class="metric-sort-icons">
              <span class="metric-sort-icon" :class="`is-${getMetricArrowDirection('cpu')}`"
                @click.stop="toggleMetricSort('cpu')">
                <template v-if="getMetricArrowDirection('cpu') === 'down'">&#8595;</template>
                <template v-else>&#8593;</template>
              </span>
            </span>
          </div>
        </template>
        <template v-slot="scope">
          <span :style="{ color: getUsageTextColor(scope.row.cpuUsage), fontWeight: '600' }">
            {{ scope.row.cpuUsage || 0 }}%
          </span>
        </template>
      </el-table-column>
      <el-table-column label="内存" width="100" align="center">
        <template #header>
          <div class="metric-header-cell">
            <span>内存</span>
            <span class="metric-sort-icons">
              <span class="metric-sort-icon" :class="`is-${getMetricArrowDirection('memory')}`"
                @click.stop="toggleMetricSort('memory')">
                <template v-if="getMetricArrowDirection('memory') === 'down'">&#8595;</template>
                <template v-else>&#8593;</template>
              </span>
            </span>
          </div>
        </template>
        <template v-slot="scope">
          <span :style="{ color: getUsageTextColor(scope.row.memoryUsage), fontWeight: '600' }">
            {{ scope.row.memoryUsage || 0 }}%
          </span>
        </template>
      </el-table-column>
      <el-table-column label="磁盘/" width="100" align="center">
        <template #header>
          <div class="metric-header-cell">
            <span>磁盘/</span>
            <span class="metric-sort-icons">
              <span class="metric-sort-icon" :class="`is-${getMetricArrowDirection('disk')}`"
                @click.stop="toggleMetricSort('disk')">
                <template v-if="getMetricArrowDirection('disk') === 'down'">&#8595;</template>
                <template v-else>&#8593;</template>
              </span>
            </span>
          </div>
        </template>
        <template v-slot="scope">
          <span :style="{ color: getUsageTextColor(scope.row.diskUsage), fontWeight: '600' }">
            {{ scope.row.diskUsage || 0 }}%
          </span>
        </template>
      </el-table-column>
      <el-table-column label="标签" width="80" prop="tag" align="center">
        <template v-slot="scope">
          <div class="tag-cell-icon">
            <button v-if="scope.row.tag" type="button" class="tag-icon-button" @click="showTagDialog(scope.row)">
              <el-icon>
                <PriceTag />
              </el-icon>
              <span class="tag-count">{{ getTagCount(scope.row.tag) }}</span>
            </button>
            <span v-else style="color: #909399;">-</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="进程" width="60" align="center">
        <template v-slot="scope">
          <el-tooltip class="item" effect="light" :content="getProcessMonitorTooltip(scope.row)" placement="top">
            <img :src="getProcessMonitorIcon(scope.row)" alt="进程" class="process-monitor-icon"
              @click="handleProcessMonitorAction(scope.row)" />
          </el-tooltip>
        </template>
      </el-table-column>
      <el-table-column label="端口" width="60" align="center">
        <template v-slot="scope">
          <el-tooltip class="item" effect="light" content="查看TCP端口监控" placement="top">
            <img src="@/assets/image/端口.svg" alt="端口" style="width: 24px; height: 24px; cursor: pointer;"
              @click="showTcpPortMonitor(scope.row)" />
          </el-tooltip>
        </template>
      </el-table-column>
      <el-table-column label="配置" width="100" show-overflow-tooltip>
        <template v-slot="scope">
          <div class="config-cell">
            <img src="@/assets/image/配置.svg" alt="配置"
              style="width: 16px; height: 16px; margin-right: 6px; flex-shrink: 0;" />
            <span>{{ scope.row.cpu }}核{{ scope.row.memory }}G</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template v-slot="scope">
          <div class="status-cell">
            <img :src="scope.row.isAlive ? require('@/assets/image/主机在线.svg') : require('@/assets/image/主机离线.svg')"
              :alt="scope.row.isAlive ? '在线' : '离线'"
              style="width: 16px; height: 16px; margin-right: 6px; flex-shrink: 0;" />
            <el-tag :type="scope.row.isAlive ? 'success' : 'danger'" size="small">
              {{ scope.row.isAlive ? '在线' : '离线' }}
            </el-tag>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="类型" width="80" prop="vendor" show-overflow-tooltip>
        <template v-slot="scope">
          <div class="vendor-cell">
            <template v-if="scope.row.vendor == 1">
              <el-icon :size="18" color="#409EFF">
                <OfficeBuilding />
              </el-icon>
              <span>自建</span>
            </template>
            <template v-else-if="scope.row.vendor == 2">
              <img src="@/assets/image/aliyun.png" style="width: 18px; height: 18px" />
              <span>阿里</span>
            </template>
            <template v-else-if="scope.row.vendor == 3">
              <img src="@/assets/image/tengxun.png" style="width: 18px; height: 18px" />
              <span>腾讯</span>
            </template>
            <template v-else-if="scope.row.vendor == 4">
              <img src="@/assets/image/baidu.svg" style="width: 18px; height: 18px" />
              <span>百度</span>
            </template>
            <template v-else-if="scope.row.vendor == 5">
              <img src="@/assets/image/华为.svg" style="width: 18px; height: 18px" />
              <span>华为</span>
            </template>
            <template v-else-if="scope.row.vendor == 6">
              <img src="@/assets/image/aws.svg" style="width: 18px; height: 18px" />
              <span>AWS</span>
            </template>
            <template v-else-if="scope.row.vendor == 7">
              <span class="tianyi-vendor-icon">天</span>
              <span>天翼</span>
            </template>
            <template v-else>
              {{ scope.row.vendor }}
            </template>
          </div>
        </template>
      </el-table-column>



      <el-table-column label="操作" width="190" fixed="right" align="center" header-align="center">
        <template v-slot="scope">
          <div class="table-operation">
            <el-button link class="operation-text-btn operation-text-btn--monitor" v-authority="['cmdb:ecs:details']"
              @click="handleHostDetail(scope.row)">
              <el-icon><TrendCharts /></el-icon>
              <span>监控</span>
            </el-button>
            <el-button link class="operation-text-btn operation-text-btn--terminal" v-authority="['cmdb:ecs:terminal']"
              @click="handleMonitorAction(scope.row)">
              <img src="@/assets/image/终端.svg" alt="" class="operation-local-icon" />
              <span>终端</span>
            </el-button>
            <el-dropdown trigger="click" placement="bottom-end" @command="command => handleMoreCommand(command, scope.row)">
              <el-button link class="operation-text-btn operation-text-btn--more">
                <span>更多</span>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item class="operation-more-item operation-more-item--edit" command="edit" v-authority="['cmdb:ecs:edit']">
                    <el-icon><Edit /></el-icon>
                    编辑资产
                  </el-dropdown-item>
                  <el-dropdown-item class="operation-more-item operation-more-item--file" command="file" v-authority="['cmdb:ecs:download']">
                    <el-icon><FolderOpened /></el-icon>
                    文件管理
                  </el-dropdown-item>
                  <el-dropdown-item class="operation-more-item operation-more-item--delete" command="delete" divided v-authority="['cmdb:ecs:delete']">
                    <el-icon><Delete /></el-icon>
                    删除资产
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </template>
      </el-table-column>
    </el-table>

    <monitor-dialog v-if="showMonitorDialog" :visible="showMonitorDialog" :host-id="currentHostId"
      style="z-index: 2001" @change="showMonitorDialog = $event" />

    <process-monitor-dialog v-if="showProcessDialog" :visible="showProcessDialog" :host-id="currentProcessHostId"
      style="z-index: 2002" @change="showProcessDialog = $event" />

    <tcp-port-monitor-dialog v-if="showTcpPortDialog" :visible="showTcpPortDialog" :host-id="currentTcpPortHostId"
      style="z-index: 2003" @change="showTcpPortDialog = $event" />

    <windows-exporter-install-dialog v-if="showWindowsExporterDialog" :visible="showWindowsExporterDialog"
      :host="currentWindowsExporterHost" style="z-index: 2004" @change="showWindowsExporterDialog = $event" />

    <!-- 标签详情对话框 -->
    <el-dialog v-model="tagDialogVisible" :title="`${currentHost.hostName} 的标签`" width="600px"
      class="tag-detail-dialog">
      <div class="tag-detail-content">
        <div class="tags-list" v-if="parsedTags.length > 0">
          <el-tag v-for="(tagItem, index) in parsedTags" :key="index" type="primary" size="large"
            class="tag-item-display" @click="copyTagToClipboard(`${tagItem.key}=${tagItem.value}`)">
            <el-icon class="tag-icon">
              <DocumentCopy />
            </el-icon>
            {{ tagItem.key }}={{ tagItem.value }}
          </el-tag>
        </div>
        <div v-else class="no-tags">
          <el-empty description="该主机没有标签" :image-size="60" />
        </div>
      </div>
    </el-dialog>

    <!-- 批量修改分组对话框 -->
    <el-dialog v-model="batchUpdateGroupDialogVisible" title="批量修改业务分组" width="500px" @close="handleBatchDialogClose">
      <div class="batch-update-group-content">
        <el-alert :title="`将为 ${multipleSelection.length} 台主机修改业务分组`" type="info" :closable="false" show-icon
          style="margin-bottom: 20px" />

        <el-form label-width="100px">
          <el-form-item label="选择分组" required>
            <el-select v-model="selectedGroupId" placeholder="请选择二级分组（子分组）" style="width: 100%" clearable filterable>
              <el-option-group v-for="parentGroup in groupedOptions" :key="parentGroup.id" :label="parentGroup.name">
                <el-option v-for="childGroup in parentGroup.children" :key="childGroup.id" :label="childGroup.name"
                  :value="childGroup.id">
                  <span style="float: left">{{ childGroup.name }}</span>
                  <span style="float: right; color: #8492a6; font-size: 13px">{{ parentGroup.name }}</span>
                </el-option>
              </el-option-group>
            </el-select>
          </el-form-item>
        </el-form>

        <div class="selected-hosts-preview">
          <div class="preview-title">待修改的主机:</div>
          <div class="hosts-list">
            <el-tag v-for="host in multipleSelection" :key="host.id" type="info" size="small" style="margin: 4px">
              {{ host.hostName }}
            </el-tag>
          </div>
        </div>
      </div>

      <template #footer>
        <el-button @click="batchUpdateGroupDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="confirmBatchUpdateGroup" :loading="batchUpdating">
          确认修改
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import MonitorDialog from './MonitorDialog.vue'
import ProcessMonitorDialog from './ProcessMonitorDialog.vue'
import TcpPortMonitorDialog from './TcpPortMonitorDialog.vue'
import WindowsExporterInstallDialog from './WindowsExporterInstallDialog.vue'
import { DocumentCopy, PriceTag } from '@element-plus/icons-vue'

export default {
  name: 'CmdbHostTable',
  components: {
    MonitorDialog,
    ProcessMonitorDialog,
    TcpPortMonitorDialog,
    WindowsExporterInstallDialog,
    DocumentCopy,
    PriceTag
  },
  props: {
    hostList: {
      type: Array,
      required: true
    },
    loading: {
      type: Boolean,
      default: false
    }
  },
  data() {
    return {
      monitorData: {},
      refreshInterval: null,
      refreshRate: 10000, // 监控数据和存活状态查询频率，单位毫秒(当前10秒)
      showMonitorDialog: false,
      currentHostId: '',
      showProcessDialog: false,
      currentProcessHostId: '',
      showTcpPortDialog: false,
      currentTcpPortHostId: '',
      showWindowsExporterDialog: false,
      currentWindowsExporterHost: {},
      isFirstOpen: true,
      tagDialogVisible: false,
      currentHost: {},
      parsedTags: [],
      // 批量操作相关
      multipleSelection: [],
      batchUpdateGroupDialogVisible: false,
      selectedGroupId: null,
      groupedOptions: [],
      batchUpdating: false,
      metricSortField: '',
      metricSortOrder: '',
      rankingLimit: 10,
      rankingHosts: [],
      metricArrowDirection: {
        cpu: 'down',
        memory: 'down',
        disk: 'down'
      },
      hostDetailCache: {}
    }
  },
  watch: {
    hostList: {
      immediate: true,
      deep: true,
      handler(newVal) {
        this.syncHostDetailCache(newVal)
        if (this.isRankingEnabled) {
          this.resetMetricSort()
        }
        if (newVal && newVal.length > 0) {
          this.stopRefresh() // 先停止之前的刷新
          this.fetchMonitorData() // 立即加载数据
          this.startRefresh() // 重新启动定时刷新
        }
      }
    }
  },
  computed: {
    isRankingEnabled() {
      return !!(this.metricSortField && this.metricSortOrder)
    },
    displayHostList() {
      return this.isRankingEnabled ? this.rankingHosts : this.hostList
    },
    hostListWithMonitor() {
      const result = this.displayHostList.map(host => {
        const monitor = this.monitorData[host.id] || {}
        const merged = {
          ...host,
          cpuUsage: this.normalizeMetricValue(
            monitor.cpuUsage ?? this.extractMetricValue(host, 'cpu')
          ),
          memoryUsage: this.normalizeMetricValue(
            monitor.memoryUsage ?? this.extractMetricValue(host, 'memory')
          ),
          diskUsage: this.normalizeMetricValue(
            monitor.diskUsage ?? this.extractMetricValue(host, 'disk')
          ),
          isAlive: monitor.onlineStatus === 0 || host.isAlive
        }
        return merged
      })

      return result
    },
    // 是否有选中的主机
    hasSelection() {
      return this.multipleSelection.length > 0
    },
    // 选中的主机数量
    selectionCount() {
      return this.multipleSelection.length
    }
  },
  methods: {
    // 判断文本是否溢出（简单判断：字符串长度超过25个字符认为可能溢出）
    isTextOverflow(text) {
      if (!text) return false
      return text.length > 20
    },
    getHostTypeValue(host) {
      return host?.hostType
        ?? host?.host_type
        ?? host?.deviceType
        ?? host?.device_type
        ?? host?.osType
        ?? host?.os_type
        ?? host?.systemType
        ?? host?.system_type
        ?? host?.platform
        ?? host?.os
        ?? host?.type
    },
    getHostSystemIcon(host) {
      return this.isWindowsHost(host)
        ? require('@/assets/image/Windows.svg')
        : require('@/assets/image/linux.svg')
    },
    getHostSystemAlt(host) {
      return this.isWindowsHost(host) ? 'windows' : 'linux'
    },
    isWindowsHost(host) {
      const hostType = this.getHostTypeValue(host)
      const numericHostType = Number(hostType)
      if (Number.isFinite(numericHostType)) {
        return numericHostType === 2
      }

      const normalizedHostType = String(hostType || '').trim().toLowerCase()
      return ['windows', 'window', 'win', 'win32', 'win64'].includes(normalizedHostType)
    },
    getProcessMonitorTooltip(host) {
      return this.isWindowsHost(host) ? '安装 Windows Exporter' : '查看进程监控'
    },
    getProcessMonitorIcon(host) {
      return this.isWindowsHost(host)
        ? require('@/assets/image/process-windows.svg')
        : require('@/assets/image/进程.svg')
    },
    handleProcessMonitorAction(host) {
      if (this.isWindowsHost(host)) {
        this.openWindowsExporterInstall(host)
        return
      }

      this.showProcessMonitor(host)
    },
    openWindowsExporterInstall(host) {
      const normalizedHost = this.normalizeHostShape(host)
      const cachedHost = this.normalizeHostShape(this.hostDetailCache[normalizedHost.id] || {})
      this.currentWindowsExporterHost = this.normalizeHostShape(this.mergeHostData(cachedHost, normalizedHost))
      this.showWindowsExporterDialog = true
    },
    handleFileManage(host) {
      this.$emit('show-download', host)
    },
    handleMoreCommand(command, host) {
      switch (command) {
        case 'edit':
          this.$emit('edit-host', host.id)
          break
        case 'file':
          this.handleFileManage(host)
          break
        case 'delete':
          this.$emit('delete-host', host)
          break
        default:
          break
      }
    },
    handleMonitorAction(host) {
      if (this.isWindowsHost(host)) {
        const url = `/rdp?rdpid=${encodeURIComponent(host.id)}`
        window.open(url, '_blank', 'noopener')
        return
      }

      this.openSshTerminal(host)
    },
    getMonitorActionTooltip(host) {
      return this.isWindowsHost(host) ? 'RDP桌面' : 'SSH终端'
    },
    getMonitorActionIcon(host) {
      return this.isWindowsHost(host) ? 'Connection' : 'Monitor'
    },
    openSshTerminal(host) {
      const normalizedHost = this.normalizeHostShape(host)
      const targetIp = normalizedHost.sshIp || normalizedHost.privateIp || normalizedHost.publicIp || ''

      if (!normalizedHost.id) {
        this.$message.error('主机信息不完整，无法打开 SSH 终端')
        return
      }

      this.$router.push({
        path: '/cmdb/ssh',
        query: {
          autoOpen: '1',
          hostId: String(normalizedHost.id),
          ip: targetIp
        }
      })
    },
    getStatusText(status) {
      const statusMap = {
        1: '认证成功',
        2: '未认证',
        3: '认证失败'
      }
      return statusMap[status] || '未知状态'
    },
    getStatusTagType(status) {
      const typeMap = {
        1: 'success',
        2: 'warning',
        3: 'danger'
      }
      return typeMap[status] || 'info'
    },
    getUsageColor(usage) {
      if (!usage) return '#909399'
      if (usage > 80) return '#F56C6C'
      if (usage > 60) return '#E6A23C'
      return '#67C23A'
    },
    getUsageTextColor(usage) {
      if (!usage) return '#909399'
      if (usage >= 85) return '#F56C6C'  // 红色：85-100%
      if (usage >= 50) return '#E6A23C'  // 黄色：50-85%
      return '#67C23A'  // 绿色：0-50%
    },
    getMetricArrowDirection(metric) {
      return this.metricArrowDirection?.[metric] || 'down'
    },
    resetMetricSort() {
      this.metricSortField = ''
      this.metricSortOrder = ''
      this.rankingHosts = []
      this.$refs.hostTableRef?.clearSelection()
      this.multipleSelection = []
      this.$emit('selection-change', [])
      this.metricArrowDirection = {
        cpu: 'down',
        memory: 'down',
        disk: 'down'
      }
    },
    syncHostDetailCache(hosts) {
      if (!Array.isArray(hosts) || hosts.length === 0) return

      const nextCache = { ...this.hostDetailCache }
      hosts.forEach(host => {
        const normalizedHost = this.normalizeHostShape(host)
        if (normalizedHost?.id) {
          nextCache[normalizedHost.id] = this.mergeHostData(
            nextCache[normalizedHost.id] || {},
            normalizedHost
          )
        }
      })
      this.hostDetailCache = nextCache
    },
    mergeHostData(base, patch) {
      const merged = { ...(base || {}) }
      Object.entries(patch || {}).forEach(([key, value]) => {
        if (value !== undefined && value !== null && value !== '') {
          merged[key] = value
        }
      })
      return merged
    },
    normalizeHostShape(host) {
      if (!host || typeof host !== 'object') return {}

      const normalized = { ...host }
      normalized.id = normalized.id ?? normalized.hostId ?? normalized.host_id
      normalized.hostName = normalized.hostName ?? normalized.host_name
      normalized.hostType = this.getHostTypeValue(normalized)
      normalized.publicIp = normalized.publicIp
        ?? normalized.public_ip
        ?? normalized.eip
        ?? normalized.internetIp
        ?? normalized.internet_ip
        ?? normalized.externalIp
        ?? normalized.external_ip
      normalized.privateIp = normalized.privateIp
        ?? normalized.private_ip
        ?? normalized.ip
        ?? normalized.localIp
        ?? normalized.local_ip
        ?? normalized.intranetIp
        ?? normalized.intranet_ip
      normalized.groupId = normalized.groupId ?? normalized.group_id
      normalized.instanceId = normalized.instanceId ?? normalized.instance_id
      normalized.sshIp = normalized.sshIp ?? normalized.ssh_ip
      normalized.sshName = normalized.sshName ?? normalized.ssh_name
      normalized.sshPort = normalized.sshPort ?? normalized.ssh_port
      normalized.sshKeyId = normalized.sshKeyId ?? normalized.ssh_key_id
      normalized.createTime = normalized.createTime ?? normalized.create_time
      normalized.updateTime = normalized.updateTime ?? normalized.update_time
      if (!normalized.privateIp && normalized.sshIp) {
        normalized.privateIp = normalized.sshIp
      }
      return normalized
    },
    buildHostDataMap() {
      const hostMap = {}
      this.hostList.forEach(host => {
        const normalized = this.normalizeHostShape(host)
        if (normalized.id) {
          hostMap[normalized.id] = normalized
        }
      })
      return hostMap
    },
    async toggleMetricSort(metric) {
      if (!metric) return

      const currentDirection = this.getMetricArrowDirection(metric)
      this.metricSortField = metric
      this.metricSortOrder = currentDirection === 'down' ? 'max' : 'min'
      this.metricArrowDirection = {
        ...this.metricArrowDirection,
        [metric]: currentDirection === 'down' ? 'up' : 'down'
      }

      this.rankingHosts = []
      this.$refs.hostTableRef?.clearSelection()
      this.multipleSelection = []
      this.$emit('selection-change', [])
      await this.fetchMonitorData()
    },
    normalizeMetricValue(value) {
      if (value === null || value === undefined || value === '') return 0
      const normalized = typeof value === 'string' ? value.replace('%', '').trim() : value
      const numericValue = Number(normalized)
      if (!Number.isFinite(numericValue)) return 0
      return Number(numericValue.toFixed(2))
    },
    extractMetricValue(host, metric) {
      if (!host || typeof host !== 'object') return 0

      const metricFields = {
        cpu: ['cpuUsage', 'cpu_usage', 'cpuPercent', 'cpu_percent', 'cpuRate', 'value', 'usage'],
        memory: ['memoryUsage', 'memory_usage', 'memoryPercent', 'memory_percent', 'memoryRate', 'memUsage', 'mem_usage', 'value', 'usage'],
        disk: ['diskUsage', 'disk_usage', 'diskPercent', 'disk_percent', 'diskRate', 'rootDiskUsage', 'root_disk_usage', 'rootDiskPercent', 'root_disk_percent', 'value', 'usage']
      }

      const fields = metricFields[metric] || []
      for (const field of fields) {
        if (host[field] !== undefined && host[field] !== null) {
          return host[field]
        }
      }

      return 0
    },
    getRankingList(data) {
      const rankingKeyMap = {
        cpu: { max: 'maxCpu', min: 'minCpu' },
        memory: { max: 'maxMemory', min: 'minMemory' },
        disk: { max: 'maxDisk', min: 'minDisk' }
      }

      const key = rankingKeyMap[this.metricSortField]?.[this.metricSortOrder]
      if (!key) return []

      const rankingData = data?.rankings || data
      const list = rankingData?.[key]
      return Array.isArray(list) ? list : []
    },
    normalizeRankingHosts(list) {
      if (!Array.isArray(list)) return []

      const usageFieldMap = {
        cpu: 'cpuUsage',
        memory: 'memoryUsage',
        disk: 'diskUsage'
      }
      const usageField = usageFieldMap[this.metricSortField]
      const hostDataMap = this.buildHostDataMap()

      return list
        .map(item => {
          const nestedHost = item?.host && typeof item.host === 'object' ? item.host : {}
          const rankingHost = this.normalizeHostShape({
            ...nestedHost,
            ...item
          })
          const hostId = rankingHost.id
          if (!hostId) return null

          const baseHost = this.normalizeHostShape(
            hostDataMap[hostId] || this.hostDetailCache[hostId] || {}
          )
          const mergedHost = this.mergeHostData(baseHost, rankingHost)

          return {
            ...mergedHost,
            id: hostId,
            [usageField]: this.normalizeMetricValue(this.extractMetricValue(rankingHost, this.metricSortField))
          }
        })
        .filter(Boolean)
    },
    async enrichRankingHosts(hosts) {
      if (!Array.isArray(hosts) || hosts.length === 0) return []

      const hostDataMap = this.buildHostDataMap()
      const idsToFetch = hosts
        .map(host => host?.id)
        .filter(id => !!id && !hostDataMap[id] && !this.hostDetailCache[id])

      const uniqueIds = [...new Set(idsToFetch)]
      if (uniqueIds.length > 0) {
        const detailList = await Promise.all(
          uniqueIds.map(async id => {
            try {
              const { data: res } = await this.$api.getCmdbHostById(id)
              if (res?.code === 200 && res?.data) {
                return this.normalizeHostShape(res.data)
              }
            } catch (error) {
              console.error(`鑾峰彇涓绘満璇︽儏澶辫触(hostId=${id}):`, error)
            }
            return null
          })
        )

        detailList.filter(Boolean).forEach(detail => {
          this.hostDetailCache = {
            ...this.hostDetailCache,
            [detail.id]: this.mergeHostData(this.hostDetailCache[detail.id] || {}, detail)
          }
        })
      }

      return hosts.map(host => {
        const baseHost = this.normalizeHostShape(
          hostDataMap[host.id] || this.hostDetailCache[host.id] || {}
        )
        return this.normalizeHostShape(this.mergeHostData(baseHost, host))
      })
    },
    async fetchCurrentRankings() {
      if (!this.isRankingEnabled) return []

      const rankingRes = await this.$api.getHostsCurrentRankings({ limit: this.rankingLimit })
      if (rankingRes?.data?.code !== 200) return this.rankingHosts

      const rankingList = this.getRankingList(rankingRes.data.data)
      const normalizedRankingHosts = this.normalizeRankingHosts(rankingList)
      this.rankingHosts = await this.enrichRankingHosts(normalizedRankingHosts)
      return this.rankingHosts
    },
    async fetchMonitorData() {
      let sourceHosts = this.displayHostList

      if (this.isRankingEnabled) {
        try {
          sourceHosts = await this.fetchCurrentRankings()
        } catch (error) {
          console.error('鑾峰彇涓绘満鎺掕鏁版嵁澶辫触:', error)
        }
      }

      if (!sourceHosts || sourceHosts.length === 0) return

      const validHosts = sourceHosts.filter(host => host?.id)
      if (validHosts.length === 0) return

      const ids = validHosts.map(host => host.id).join(',')

      // 请求合并后的监控数据接口
      const monitorRes = await this.$api.getHostsMonitorData(ids)

      // 立即更新数据，不等待任何延迟
      if (monitorRes.data.code === 200) {
        this.monitorData = {
          ...this.monitorData,
          ...monitorRes.data.data
        }
      }
    },
    startRefresh() {
      // 先停止之前的定时器，避免重复创建
      this.stopRefresh()
      // 立即执行第一次数据加载
      this.fetchMonitorData()
      // 设置定时刷新，但确保第一次加载不等待
      this.refreshInterval = setInterval(() => {
        this.fetchMonitorData()
      }, this.refreshRate)
      console.log('监控数据轮询已启动')
    },
    stopRefresh() {
      if (this.refreshInterval) {
        clearInterval(this.refreshInterval)
        this.refreshInterval = null
        console.log('监控数据轮询已停止')
      }
    },
    showMonitor(host) {
      this.currentHostId = host.id
      this.showMonitorDialog = true
      this.isFirstOpen = true
    },
    showProcessMonitor(host) {
      this.currentProcessHostId = host.id
      this.showProcessDialog = true
    },
    showTcpPortMonitor(host) {
      this.currentTcpPortHostId = host.id
      this.showTcpPortDialog = true
    },

    // 显示复制图标
    showCopyIcon(event) {
      const icons = event.currentTarget.querySelectorAll('.copy-icon')
      icons.forEach(icon => {
        icon.style.display = 'inline-block'
      })
    },

    // 隐藏复制图标
    hideCopyIcon(event) {
      const icons = event.currentTarget.querySelectorAll('.copy-icon')
      icons.forEach(icon => {
        icon.style.display = 'none'
      })
    },

    // 复制到剪贴板
    async copyToClipboard(text, type, event) {
      try {
        await navigator.clipboard.writeText(text)
        this.$message.success(`${type} 已复制: ${text}`)

        // 添加复制成功的视觉反馈
        if (event && event.target) {
          const icon = event.target.closest('.copy-icon')
          if (icon) {
            icon.classList.add('copied')
            setTimeout(() => {
              icon.classList.remove('copied')
            }, 1000)
          }
        }
      } catch (error) {
        // 降级方案
        const textArea = document.createElement('textarea')
        textArea.value = text
        document.body.appendChild(textArea)
        textArea.focus()
        textArea.select()
        try {
          document.execCommand('copy')
          this.$message.success(`${type} 已复制: ${text}`)

          // 添加复制成功的视觉反馈
          if (event && event.target) {
            const icon = event.target.closest('.copy-icon')
            if (icon) {
              icon.classList.add('copied')
              setTimeout(() => {
                icon.classList.remove('copied')
              }, 1000)
            }
          }
        } catch (fallbackError) {
          this.$message.error('复制失败，请手动复制')
        }
        document.body.removeChild(textArea)
      }
    },
    // 解析标签字符串，格式：key=value,key=value
    parseTagString(tagStr) {
      if (!tagStr || typeof tagStr !== 'string') return []

      const tags = []
      const pairs = tagStr.split(',')

      pairs.forEach(pair => {
        const trimmed = pair.trim()
        if (trimmed) {
          const [key, value] = trimmed.split('=')
          if (key && value) {
            tags.push({ key: key.trim(), value: value.trim() })
          }
        }
      })

      return tags
    },
    // 获取标签数量
    getTagCount(tagStr) {
      return this.parseTagString(tagStr).length
    },
    // 显示标签详情对话框
    showTagDialog(row) {
      this.currentHost = row
      this.parsedTags = this.parseTagString(row.tag)
      this.tagDialogVisible = true
    },
    // 复制标签到剪贴板
    async copyTagToClipboard(text) {
      try {
        await navigator.clipboard.writeText(text)
        this.$message.success('标签已复制到剪贴板')
      } catch (error) {
        console.error('复制失败:', error)
        const textArea = document.createElement('textarea')
        textArea.value = text
        document.body.appendChild(textArea)
        textArea.select()
        try {
          document.execCommand('copy')
          this.$message.success('标签已复制到剪贴板')
        } catch (fallbackError) {
          this.$message.error('复制失败，请手动复制')
        }
        document.body.removeChild(textArea)
      }
    },

    // 批量操作相关方法
    // 处理表格选择变化
    handleSelectionChange(selection) {
      console.log('表格选择变化:', selection.length, '台主机')
      this.multipleSelection = selection

      // 根据选择状态控制轮询
      if (selection.length > 0) {
        // 有选中主机时，停止轮询刷新
        console.log('检测到主机选中，暂停监控数据轮询')
        this.stopRefresh()
      } else {
        // 没有选中主机时，恢复轮询刷新
        console.log('主机已取消选中，恢复监控数据轮询')
        this.startRefresh()
      }

      // 通知父组件选择状态变化
      this.$emit('selection-change', selection)
    },

    // 清除选择
    clearSelection() {
      this.$refs.hostTableRef.clearSelection()
      this.multipleSelection = []
      // 清除选择后恢复轮询
      console.log('清除主机选择，恢复监控数据轮询')
      this.startRefresh()
    },

    // 显示批量修改分组对话框
    async showBatchUpdateGroupDialog() {
      if (this.multipleSelection.length === 0) {
        this.$message.warning('请先选择要修改的主机')
        return
      }

      console.log('打开对话框前，选中主机数:', this.multipleSelection.length)

      // 加载分组数据
      await this.loadGroupOptions()

      this.batchUpdateGroupDialogVisible = true

      console.log('对话框已打开，选中主机数:', this.multipleSelection.length)
    },

    // 加载分组选项
    async loadGroupOptions() {
      try {
        const res = await this.$api.getAllCmdbGroups()
        if (res.data.code === 200) {
          // 转换为分组选择器需要的格式（只显示二级分组）
          this.groupedOptions = this.buildGroupedOptions(res.data.data)
        }
      } catch (error) {
        console.error('加载分组失败:', error)
        this.$message.error('加载分组数据失败')
      }
    },

    // 构建分组选项（一级分组作为分组标题，二级分组作为选项）
    buildGroupedOptions(groups) {
      if (!groups || !Array.isArray(groups)) return []

      const result = []

      // 遍历所有分组（后端已返回树形结构）
      groups.forEach(group => {
        // 只处理一级分组（parentId === 0）
        if (group.parentId === 0) {
          // 检查是否有子分组
          if (group.children && Array.isArray(group.children) && group.children.length > 0) {
            result.push({
              id: group.id,
              name: group.name,
              children: group.children.map(child => ({
                id: child.id,
                name: child.name,
                parentId: child.parentId
              }))
            })
          }
        }
      })

      console.log('构建的分组选项:', result)
      return result
    },

    // 确认批量修改分组
    async confirmBatchUpdateGroup() {
      if (!this.selectedGroupId) {
        this.$message.warning('请选择目标分组')
        return
      }

      const hostIds = this.multipleSelection.map(host => host.id)

      try {
        this.batchUpdating = true

        const res = await this.$api.batchUpdateHostGroup(hostIds, this.selectedGroupId)

        if (res.data.code === 200) {
          this.$message.success(res.data.data.message || '批量修改分组成功')

          // 关闭对话框
          this.batchUpdateGroupDialogVisible = false

          // 清除选择并恢复轮询
          this.clearSelection()

          // 通知父组件刷新数据
          this.$emit('refresh-list')
        } else {
          this.$message.error(res.data.msg || '批量修改分组失败')
        }
      } catch (error) {
        console.error('批量修改分组失败:', error)
        this.$message.error(error.response?.data?.msg || '批量修改分组失败')
      } finally {
        this.batchUpdating = false
      }
    },

    // 关闭批量修改对话框时的处理
    handleBatchDialogClose() {
      this.selectedGroupId = null
      // 如果用户关闭对话框但没有完成操作,根据当前选择状态决定是否恢复轮询
      // 由于 handleSelectionChange 已经控制了轮询,这里不需要额外处理
    },

    // 跳转到主机详情页面
    handleHostDetail(row) {
      const path = this.isWindowsHost(row)
        ? `/cmdb/host/windows-detail/${row.id}`
        : `/cmdb/host/detail/${row.id}`
      this.$router.push(path)
    }
  },
  mounted() {
    // 如果hostList已有数据，立即获取监控数据
    if (this.hostList && this.hostList.length > 0) {
      this.fetchMonitorData()
    }
    // 启动定时刷新
    this.startRefresh()
  },
  beforeUnmount() {
    this.stopRefresh()
  },
  beforeRouteEnter(to, from, next) {
    next(vm => {
      vm.stopRefresh()
      vm.fetchMonitorData()
      vm.startRefresh()
    })
  },
  beforeRouteUpdate(to, from, next) {
    this.stopRefresh()
    this.fetchMonitorData()
    this.startRefresh()
    next()
  },
  activated() {
    this.stopRefresh()
    this.fetchMonitorData()
    this.startRefresh()
  }
}
</script>

<style scoped>
/* 🎨 现代化科技感表格样式 */

.table-section {
  display: flex;
  height: 100%;
  min-height: 0;
  margin-bottom: 0;
  width: 100%;
}

/* 📊 表格样式 */
.host-table {
  flex: 1;
  min-height: 0;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
}

.host-table :deep(.el-table__header) {
  background: linear-gradient(135deg, #667eea, #764ba2);
}

.host-table :deep(.el-table__header th) {
  background: transparent !important;
  color: #2c3e50 !important;
  font-weight: 700 !important;
  border-bottom: none;
  text-shadow: 0 1px 2px rgba(255, 255, 255, 0.8);
}

.host-table :deep(.el-table__header th .cell) {
  color: #2c3e50 !important;
  font-weight: 700 !important;
  text-shadow: 0 1px 2px rgba(255, 255, 255, 0.8);
}

.metric-header-cell {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
}

.metric-sort-icons {
  display: inline-flex;
  align-items: center;
  line-height: 1;
}

.metric-sort-icon {
  font-size: 14px;
  font-weight: 700;
  cursor: pointer;
  transition: opacity 0.2s ease;
}

.metric-sort-icon:hover {
  opacity: 0.85;
}

.metric-sort-icon.is-down {
  color: #e6a23c;
}

.metric-sort-icon.is-up {
  color: #67c23a;
}

.host-table :deep(.el-table__row) {
  transition: background-color 0.2s ease;
}

.host-table :deep(.el-table__row:hover) {
  background-color: rgba(103, 126, 234, 0.05) !important;
  transform: none;
  box-shadow: none;
}

/* 🎯 操作按钮样式 - 现代化设计 */
.table-operation {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 12px;
  padding: 4px 0;
  white-space: nowrap;
}

.operation-text-btn {
  min-height: 24px;
  padding: 0;
  font-size: 13px;
  font-weight: 500;
}

.operation-text-btn :deep(.el-icon) {
  margin-right: 3px;
  font-size: 14px;
}

.operation-local-icon {
  flex: 0 0 auto;
  width: 14px;
  height: 14px;
  margin-right: 3px;
  object-fit: contain;
}

.operation-text-btn + .operation-text-btn {
  margin-left: 0;
}

.operation-text-btn--monitor {
  color: #409eff !important;
}

.operation-text-btn--monitor:hover,
.operation-text-btn--monitor:focus {
  color: #1d7fe8 !important;
}

.operation-text-btn--terminal {
  color: #1f3f8b !important;
}

.operation-text-btn--terminal:hover,
.operation-text-btn--terminal:focus {
  color: #17306d !important;
}

.operation-text-btn--more {
  color: #7c3aed !important;
}

.operation-text-btn--more:hover,
.operation-text-btn--more:focus {
  color: #5b21b6 !important;
}

.operation-more-item--edit {
  color: #409eff !important;
}

.operation-more-item--file {
  color: #e6a23c !important;
}

.operation-more-item--delete {
  color: #f56c6c !important;
}

.operation-more-item :deep(.el-icon) {
  margin-right: 6px;
}

/* 🏷️ 标签样式优化 */
.el-tag {
  font-weight: 500;
  border-radius: 8px;
  border: none;
}

/* 📊 进度条样式 */
.el-progress {
  margin: 2px 0;
}

/* 🔗 链接样式 */
.el-link {
  font-weight: 400;
  font-size: 13px;
  transition: all 0.3s ease;
}

.el-link:hover {
  text-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

/* 🔧 表格单元格样式 - 防止换行 */
.host-name-cell-wrapper {
  display: flex;
  align-items: center;
  gap: 4px;
  position: relative;
  width: 100%;
}

.host-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
  white-space: nowrap;
  overflow: hidden;
  flex: 1;
  min-width: 0;
  font-size: 13px;
}

/* Tooltip 样式调整 - 减少偏移距离 */
.host-table :deep(.el-table__cell .cell) {
  padding-left: 10px;
  padding-right: 10px;
}

.ip-cell {
  white-space: nowrap;
  overflow: hidden;
}

.ip-row {
  display: flex;
  align-items: center;
  gap: 4px;
  white-space: nowrap;
  font-size: 12px;
  line-height: 1.2;
  position: relative;
}

.ip-row.public-ip {
  color: #409EFF;
  margin-bottom: 2px;
}

.ip-row.private-ip {
  color: #67C23A;
}

.ip-row span {
  flex: 1;
  min-width: 0;
  padding-right: 18px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.ip-row .ip-copy-icon {
  position: absolute;
  top: 50%;
  right: 0;
  display: inline-flex !important;
  margin: 0;
  opacity: 0;
  transform: translateY(-50%);
  transition: opacity 0.18s ease, background-color 0.18s ease;
}

.ip-row.public-ip .ip-copy-icon {
  color: #409eff;
}

.ip-row.private-ip .ip-copy-icon {
  color: #67c23a;
}

.ip-row:hover .ip-copy-icon,
.ip-row .ip-copy-icon:hover,
.ip-row .ip-copy-icon.copied {
  transform: translateY(-50%);
}

.vendor-cell {
  display: flex;
  align-items: center;
  gap: 5px;
  white-space: nowrap;
  overflow: hidden;
}

.vendor-cell span {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tianyi-vendor-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 4px;
  background: #e60012;
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  flex-shrink: 0;
}

.tag-cell {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 2px;
  white-space: normal !important;
  line-height: 1.3;
  padding: 4px 0;
}

.tag-item {
  margin-right: 3px;
  margin-bottom: 2px;
  font-size: 9px;
  white-space: nowrap;
  padding: 2px 6px !important;
  height: auto;
  line-height: 1.3;
}

.config-cell {
  display: flex;
  align-items: center;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.process-monitor-icon {
  width: 24px;
  height: 24px;
  cursor: pointer;
  display: block;
  margin: 0 auto;
  transition: transform 0.2s ease, opacity 0.2s ease;
}

.process-monitor-icon:hover {
  transform: translateY(-1px) scale(1.05);
  opacity: 0.92;
}

.status-cell {
  display: flex;
  align-items: center;
  white-space: nowrap;
  overflow: hidden;
}

/* 确保所有表格单元格内容不换行 */
.host-table :deep(.el-table__cell) {
  white-space: nowrap;
  overflow: hidden;
}

.host-table :deep(.cell) {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* 标签列徽标需要允许溢出显示 */
.host-table :deep(.el-table__cell .tag-cell),
.host-table :deep(.el-table__cell .tag-cell-icon) {
  white-space: normal !important;
  overflow: visible !important;
}

.host-table :deep(.cell:has(.tag-cell)),
.host-table :deep(.cell:has(.tag-cell-icon)) {
  white-space: normal !important;
  overflow: visible !important;
}

/* 📋 复制图标样式 */
.copy-icon {
  opacity: 0;
  transition: all 0.3s ease;
  font-size: 14px !important;
  padding: 2px;
  border-radius: 4px;
  cursor: pointer;
  color: #409EFF;
  flex-shrink: 0;
}

.copy-icon:hover {
  background-color: rgba(64, 158, 255, 0.1);
  transform: scale(1.1);
}

/* 鼠标悬停时显示复制图标 */
.host-name-cell-wrapper:hover .copy-icon,
.ip-row:hover .copy-icon {
  opacity: 1;
  display: inline-flex !important;
}

/* 复制成功动画效果 */
.copy-icon.copied {
  color: #67C23A !important;
  transform: scale(1.2);
}

/* IP图标样式 */
.ip-icon {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  object-fit: contain;
}

/* 🏷️ 标签图标按钮样式 */
.tag-cell-icon {
  display: flex;
  justify-content: center;
  align-items: center;
}

.tag-icon-button {
  position: relative;
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 0;
  border-radius: 8px;
  background: #2f8df7;
  color: #fff;
  cursor: pointer;
  overflow: visible;
  box-shadow: 0 2px 6px rgba(47, 141, 247, 0.22);
  transition: background-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
}

.tag-icon-button:hover {
  background: #1f7be3;
  box-shadow: 0 4px 10px rgba(47, 141, 247, 0.3);
  transform: translateY(-1px);
}

.tag-icon-button .el-icon {
  font-size: 14px;
}

.tag-count {
  position: absolute;
  top: -5px;
  right: -7px;
  background: #f56c6c;
  color: white;
  font-size: 10px;
  font-weight: 700;
  padding: 0 5px;
  border: 2px solid #fff;
  border-radius: 999px;
  line-height: 16px;
  min-width: 18px;
  height: 18px;
  text-align: center;
  box-sizing: border-box;
  box-shadow: 0 2px 5px rgba(245, 108, 108, 0.35);
}

/* 标签详情对话框样式 */
.tag-detail-dialog {
  border-radius: 12px;
}

.tag-detail-content {
  padding: 10px;
}

.tags-list {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.tag-item-display {
  cursor: pointer;
  transition: all 0.3s ease;
  font-size: 14px;
  padding: 8px 12px !important;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.tag-item-display:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(64, 158, 255, 0.3);
}

.tag-item-display .tag-icon {
  font-size: 14px;
}

.no-tags {
  text-align: center;
  padding: 20px;
  color: #909399;
}

/* 批量修改分组对话框样式 */
.batch-update-group-content {
  padding: 10px 0;
}

.selected-hosts-preview {
  margin-top: 20px;
  padding: 15px;
  background: #f5f7fa;
  border-radius: 8px;
}

.preview-title {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 10px;
}

.hosts-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  max-height: 200px;
  overflow-y: auto;
}

.hosts-list::-webkit-scrollbar {
  width: 6px;
}

.hosts-list::-webkit-scrollbar-thumb {
  background: #dcdfe6;
  border-radius: 3px;
}

.hosts-list::-webkit-scrollbar-thumb:hover {
  background: #c0c4cc;
}
</style>
