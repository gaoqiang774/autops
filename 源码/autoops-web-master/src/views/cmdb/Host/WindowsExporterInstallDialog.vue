<template>
  <el-dialog
    v-model="dialogVisible"
    title="Windows Exporter 管理"
    width="min(880px, calc(100vw - 32px))"
    top="2vh"
    class="windows-exporter-dialog"
    :append-to-body="true"
    @close="handleClose"
  >
    <div class="exporter-dialog">
      <div class="hero-panel">
        <div class="hero-copy">
          <div class="hero-head">
            <div>
              <div class="hero-title">{{ host.hostName || 'Windows 主机' }}</div>
              <div class="hero-subtitle">{{ targetDisplay }}</div>
            </div>
          </div>
          <div class="hero-description">
            通过后端 WinRM 脚本安装、卸载和查询 Windows Exporter。安装前会先执行远程环境检测，只有检测通过才允许提交安装任务。
          </div>
        </div>
        <div class="hero-actions">
          <el-button type="primary" icon="Document" :loading="installLogLoading" @click="handleShowInstallLog">
            安装日志
          </el-button>
          <el-button type="success" icon="Refresh" :loading="isRefreshing" @click="handleRefreshAll">
            检测环境
          </el-button>
          <el-button type="warning" icon="Download" :loading="downloadingScript" @click="handleDownloadInitScript">
            下载WinRM脚本
          </el-button>
        </div>
      </div>

      <div class="panel-stack">
        <section class="surface-panel host-panel">
          <div class="surface-panel-head">
            <div>
              <div class="panel-eyebrow">HOST SNAPSHOT</div>
              <div class="panel-title">主机与状态</div>
            </div>
            <div class="section-status-cluster">
              <el-tag :type="statusMeta.type" effect="plain">{{ statusMeta.label }}</el-tag>
              <el-tag :type="precheckStatusTag.type" effect="plain">{{ precheckStatusTag.text }}</el-tag>
            </div>
          </div>

          <div class="table-shell">
            <table class="data-table host-data-table">
              <tbody>
                <tr>
                  <th>主机名称</th>
                  <td>{{ host.hostName || '-' }}</td>
                  <th>主机 ID</th>
                  <td class="is-mono">{{ host.id || '-' }}</td>
                </tr>
                <tr>
                  <th>连接地址</th>
                  <td class="is-mono">{{ targetDisplay }}</td>
                  <th>认证凭据</th>
                  <td class="is-soft">{{ credentialDisplay }}</td>
                </tr>
                <tr>
                  <th>Exporter 状态</th>
                  <td>
                    <div class="table-badges">
                      <el-tag :type="statusMeta.type" effect="plain">{{ statusMeta.label }}</el-tag>
                      <span class="table-inline-text">{{ exporterStatus.version ? `v${exporterStatus.version}` : '等待安装' }}</span>
                    </div>
                  </td>
                  <th>环境检测</th>
                  <td>
                    <div class="table-badges">
                      <el-tag :type="precheckStatusTag.type" effect="plain">{{ precheckStatusTag.text }}</el-tag>
                      <span class="table-inline-text">
                        {{ precheck.ready ? '可继续安装' : precheck.checked ? '存在阻塞项' : '等待检测' }}
                      </span>
                    </div>
                  </td>
                </tr>
                <tr>
                  <th>运行状态</th>
                  <td :class="{ 'is-soft': !exporterStatus.running }">
                    {{ exporterStatus.running ? '服务运行中' : '未运行' }}
                  </td>
                  <th>安装版本</th>
                  <td>{{ exporterStatus.version || '-' }}</td>
                </tr>
                <tr>
                  <th>监听端口</th>
                  <td class="is-mono">{{ exporterStatus.port || '-' }}</td>
                  <th>最后检测时间</th>
                  <td>{{ precheck.checkedAt ? formatTime(precheck.checkedAt) : '-' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section v-loading="precheck.loading" class="surface-panel check-panel">
          <div class="surface-panel-head">
            <div>
              <div class="panel-eyebrow">REMOTE READINESS</div>
              <div class="panel-title">环境检测</div>
            </div>
            <div class="surface-panel-tag" :class="{
              'is-success': precheck.checked && precheck.ready,
              'is-danger': precheck.checked && !precheck.ready,
              'is-neutral': !precheck.checked
            }">
              {{ precheckStatusTag.text }}
            </div>
          </div>

          <div class="table-shell">
            <table v-if="displayPrecheckItems.length > 0" class="data-table precheck-data-table">
              <thead>
                <tr>
                  <th>检查项</th>
                  <th>结果</th>
                  <th>说明</th>
                  <th>返回值</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="item in displayPrecheckItems" :key="item.key">
                  <td class="check-name-cell">
                    <span class="check-state-dot" :class="`is-${item.state}`" />
                    <span>{{ item.label }}</span>
                  </td>
                  <td>
                    <el-tag size="small" :type="item.type" effect="plain">{{ item.statusText }}</el-tag>
                  </td>
                  <td>{{ item.message || '-' }}</td>
                  <td class="is-mono">{{ item.value || '-' }}</td>
                </tr>
              </tbody>
            </table>
            <el-empty v-else description="暂无检测结果" :image-size="72" />
          </div>

          <div v-if="precheck.checkedAt" class="precheck-meta">
            最后检测时间：{{ formatTime(precheck.checkedAt) }}
          </div>
        </section>
      </div>

      <el-alert
        v-if="pendingAction"
        :title="pendingAction === 'install' ? '安装任务已提交，正在自动轮询状态。' : '卸载任务已提交，正在自动轮询状态。'"
        type="info"
        :closable="false"
        show-icon
        class="workflow-alert"
      />

    </div>

    <el-dialog
      v-model="installLogDialogVisible"
      title="Windows Exporter 安装日志"
      width="min(980px, calc(100vw - 40px))"
      top="4vh"
      class="install-log-dialog"
      :append-to-body="true"
    >
      <div class="install-log-body" v-loading="installLogLoading">
        <div class="install-log-summary">
          <div class="install-log-summary-left">
            <strong>{{ host.hostName || '当前主机' }}</strong>
            <span>{{ installLogData.logPath || '未返回日志路径' }}</span>
          </div>
          <div class="install-log-summary-right">
            <el-tag :type="installLogData.running ? 'success' : installLogData.installed ? 'warning' : 'info'" effect="plain">
              {{ installLogData.running ? '服务运行中' : installLogData.installed ? '已安装未运行' : '未安装' }}
            </el-tag>
            <el-tag type="info" effect="plain">
              版本 {{ installLogData.version || '-' }}
            </el-tag>
          </div>
        </div>

        <el-tabs v-model="installLogActiveTab" class="install-log-tabs">
          <el-tab-pane label="安装日志" name="installLog">
            <pre class="log-block">{{ installLogData.installLog || '暂无安装日志' }}</pre>
          </el-tab-pane>
          <el-tab-pane label="服务状态" name="serviceStatus">
            <pre class="log-block">{{ installLogData.serviceStatus || '暂无服务状态信息' }}</pre>
          </el-tab-pane>
          <el-tab-pane label="服务配置" name="serviceConfig">
            <pre class="log-block">{{ installLogData.serviceConfig || '暂无服务配置信息' }}</pre>
          </el-tab-pane>
          <el-tab-pane label="Application 事件" name="applicationEvents">
            <pre class="log-block">{{ installLogData.applicationEvents || '暂无 Application 事件日志' }}</pre>
          </el-tab-pane>
          <el-tab-pane label="System 事件" name="systemEvents">
            <pre class="log-block">{{ installLogData.systemEvents || '暂无 System 事件日志' }}</pre>
          </el-tab-pane>
        </el-tabs>
      </div>

      <template #footer>
        <div class="install-log-footer">
          <el-button @click="installLogDialogVisible = false">关闭</el-button>
        </div>
      </template>
    </el-dialog>

    <template #footer>
      <div class="dialog-footer">
        <el-button
          v-if="showUninstallButton"
          type="danger"
          icon="Delete"
          :loading="uninstallSubmitting"
          :disabled="!canUninstall"
          class="footer-action-button is-danger"
          @click="handleUninstall"
        >
          卸载 Exporter
        </el-button>
        <el-button
          v-if="showInstallButton"
          type="warning"
          icon="Download"
          :loading="installSubmitting"
          :disabled="!canInstall"
          class="footer-action-button is-warning"
          @click="handleInstall"
        >
          安装 Exporter
        </el-button>
        <el-button @click="handleClose">关闭</el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script>
export default {
  name: 'WindowsExporterInstallDialog',
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    host: {
      type: Object,
      default: () => ({})
    }
  },
  emits: ['change'],
  data() {
    return {
      exporterStatus: this.getDefaultExporterStatus(),
      precheck: this.getDefaultPrecheckResult(),
      installSubmitting: false,
      uninstallSubmitting: false,
      downloadingScript: false,
      installLogLoading: false,
      installLogDialogVisible: false,
      installLogActiveTab: 'installLog',
      installLogData: this.getDefaultInstallLogData(),
      pollingTimer: null,
      pollingInterval: 4000,
      pendingAction: ''
    }
  },
  computed: {
    dialogVisible: {
      get() {
        return this.visible
      },
      set(value) {
        this.$emit('change', value)
      }
    },
    isWindowsHost() {
      return Number(this.host?.hostType ?? this.host?.host_type) === 2
    },
    isRefreshing() {
      return this.exporterStatus.loading || this.precheck.loading
    },
    targetDisplay() {
      const username = this.host?.sshName ? `${this.host.sshName}@` : ''
      const ip = this.host?.privateIp || this.host?.sshIp || this.host?.publicIp || '-'
      const port = this.host?.sshPort ? `:${this.host.sshPort}` : ''
      return `${username}${ip}${port}`
    },
    credentialDisplay() {
      return this.host?.sshKeyId ? `已配置凭据(ID: ${this.host.sshKeyId})` : '请确认主机已配置认证凭据'
    },
    statusMeta() {
      return this.getStatusMeta(this.exporterStatus.status)
    },
    showInstallButton() {
      return [0, 4].includes(this.exporterStatus.status)
    },
    showUninstallButton() {
      return [1, 3].includes(this.exporterStatus.status)
    },
    canInstall() {
      return this.isWindowsHost
        && !!this.host?.id
        && this.precheck.checked
        && this.precheck.ready
        && !this.precheck.loading
        && !this.installSubmitting
        && !this.uninstallSubmitting
    },
    canUninstall() {
      return !!this.host?.id && !this.installSubmitting && !this.uninstallSubmitting
    },
    precheckStatusTag() {
      if (this.precheck.loading) {
        return { text: '检测中', type: 'info' }
      }
      if (!this.precheck.checked) {
        return { text: '未检测', type: 'info' }
      }
      if (this.precheck.ready) {
        return { text: '可安装', type: 'success' }
      }
      return { text: '不满足安装条件', type: 'danger' }
    },
    precheckSummaryText() {
      if (this.precheck.loading) {
        return '正在执行远程环境检测，请稍候。'
      }
      if (this.precheck.errorMessage) {
        return this.precheck.errorMessage
      }
      if (!this.precheck.checked) {
        return '尚未执行环境检测。'
      }
      if (this.precheck.ready) {
        return this.precheck.message || '环境检测通过，可以执行安装。'
      }
      return this.precheck.message || '环境检测未通过，请先处理失败项。'
    },
    precheckOverviewTitle() {
      if (this.precheck.loading) {
        return '正在检测远程环境'
      }
      if (this.precheck.errorMessage) {
        return '检测执行失败'
      }
      if (!this.precheck.checked) {
        return '等待执行环境检测'
      }
      return this.precheck.ready ? '环境通过，可以继续安装' : '存在阻塞项，暂不可安装'
    },
    normalizedPrecheckItems() {
      return this.precheck.items.map((item, index) => this.normalizePrecheckItem(item, index))
    },
    displayPrecheckItems() {
      const stateOrder = {
        fail: 0,
        unknown: 1,
        pass: 2
      }
      return [...this.normalizedPrecheckItems].sort((a, b) => {
        const left = stateOrder[a.state] ?? 9
        const right = stateOrder[b.state] ?? 9
        return left - right
      })
    },
    precheckCounts() {
      return this.normalizedPrecheckItems.reduce((acc, item) => {
        if (item.state === 'pass') {
          acc.pass += 1
        } else if (item.state === 'fail') {
          acc.fail += 1
        } else {
          acc.unknown += 1
        }
        return acc
      }, {
        pass: 0,
        fail: 0,
        unknown: 0
      })
    }
  },
  watch: {
    visible: {
      immediate: true,
      handler(value) {
        if (value) {
          this.initializeDialog()
          return
        }
        this.stopPolling()
        this.pendingAction = ''
      }
    },
    'host.id': {
      handler(newVal, oldVal) {
        if (!newVal || newVal === oldVal) return
        this.stopPolling()
        this.pendingAction = ''
        this.exporterStatus = this.getDefaultExporterStatus()
        this.precheck = this.getDefaultPrecheckResult()
        if (this.visible) {
          this.initializeDialog()
        }
      }
    }
  },
  methods: {
    getDefaultExporterStatus() {
      return {
        loading: false,
        status: 0,
        statusText: 'not_installed',
        statusColor: 'info',
        version: '',
        port: '',
        installed: false,
        running: false
      }
    },
    getDefaultPrecheckResult() {
      return {
        loading: false,
        checked: false,
        ready: false,
        items: [],
        message: '',
        errorMessage: '',
        checkedAt: ''
      }
    },
    getDefaultInstallLogData() {
      return {
        installed: false,
        running: false,
        version: '',
        logPath: '',
        installLog: '',
        serviceStatus: '',
        serviceConfig: '',
        applicationEvents: '',
        systemEvents: ''
      }
    },
    getStatusMeta(status) {
      const statusMap = {
        0: {
          label: '未安装',
          type: 'info',
          alertType: 'info',
          description: '当前主机尚未安装 Windows Exporter，可以在环境检测通过后发起安装任务。'
        },
        1: {
          label: '运行中',
          type: 'success',
          alertType: 'success',
          description: 'Windows Exporter 已安装并处于运行状态。'
        },
        2: {
          label: '安装中',
          type: 'primary',
          alertType: 'info',
          description: '安装任务执行中，请等待轮询返回最终状态。'
        },
        3: {
          label: '已安装未运行',
          type: 'warning',
          alertType: 'warning',
          description: 'Windows Exporter 已安装，但服务当前未运行，请检查目标机状态。'
        },
        4: {
          label: '安装失败',
          type: 'danger',
          alertType: 'error',
          description: '安装失败，请优先检查 WinRM、认证凭据和安装包可达性。'
        },
        5: {
          label: '启动中',
          type: 'primary',
          alertType: 'info',
          description: 'Windows Exporter 正在启动中，请稍后刷新或等待自动轮询。'
        }
      }
      return statusMap[status] || {
        label: '未知状态',
        type: 'info',
        alertType: 'info',
        description: '当前状态无法识别，请稍后刷新重试。'
      }
    },
    normalizeExporterStatus(payload = {}) {
      return {
        loading: false,
        status: payload.status ?? 0,
        statusText: payload.statusText || '',
        statusColor: payload.statusColor || '',
        version: payload.version || '',
        port: payload.port ?? '',
        installed: Boolean(payload.installed),
        running: Boolean(payload.running)
      }
    },
    normalizePrecheckResult(payload = {}) {
      const rawItems = Array.isArray(payload.items)
        ? payload.items
        : Array.isArray(payload.checkItems)
          ? payload.checkItems
          : Array.isArray(payload.results)
            ? payload.results
            : []

      return {
        loading: false,
        checked: true,
        ready: Boolean(payload.ready),
        items: rawItems,
        message: payload.message || payload.summary || payload.msg || '',
        errorMessage: '',
        checkedAt: Date.now()
      }
    },
    resolvePrecheckItemState(item) {
      const booleanKeys = ['passed', 'pass', 'success', 'ok', 'ready', 'result']
      for (const key of booleanKeys) {
        if (typeof item?.[key] === 'boolean') {
          return item[key] ? 'pass' : 'fail'
        }
      }

      const status = item?.status ?? item?.state ?? item?.level
      if (typeof status === 'number') {
        return status > 0 ? 'pass' : 'fail'
      }

      if (typeof status === 'string') {
        const normalized = status.toLowerCase()
        if (['pass', 'passed', 'success', 'ok', 'ready', 'reachable'].some(keyword => normalized.includes(keyword))) {
          return 'pass'
        }
        if (['fail', 'failed', 'error', 'warning', 'not_ready', 'unreachable'].some(keyword => normalized.includes(keyword))) {
          return 'fail'
        }
      }

      return 'unknown'
    },
    normalizePrecheckItem(item, index) {
      const state = this.resolvePrecheckItemState(item)
      const itemName = item?.label || item?.name || item?.title || item?.key || item?.item || `检查项 ${index + 1}`
      const itemLabelMap = {
        tcp_connect: 'WinRM 端口连通性',
        winrm_command: 'WinRM 命令执行'
      }
      const label = itemLabelMap[itemName] || itemName
      const rawMessage = item?.message
        || item?.detail
        || item?.reason
        || item?.error
        || item?.desc
        || item?.description
        || item?.statusText
        || item?.resultText
        || ''
      const message = typeof rawMessage === 'string'
        ? rawMessage
        : rawMessage
          ? JSON.stringify(rawMessage)
          : ''
      const rawValue = item?.value ?? item?.result ?? item?.output ?? ''
      const value = typeof rawValue === 'string'
        ? rawValue
        : rawValue
          ? JSON.stringify(rawValue)
          : ''

      return {
        key: item?.key || item?.name || item?.label || `precheck-${index}`,
        label,
        state,
        type: state === 'pass' ? 'success' : state === 'fail' ? 'danger' : 'info',
        statusText: item?.statusText || (state === 'pass' ? '通过' : state === 'fail' ? '失败' : '未知'),
        message,
        value
      }
    },
    formatTime(value) {
      if (!value) return '-'
      const date = new Date(value)
      if (Number.isNaN(date.getTime())) return '-'
      return date.toLocaleString('zh-CN', { hour12: false })
    },
    initializeDialog() {
      if (this.host?.id) {
        this.fetchExporterStatus({ silent: true })
      }
      if (this.hasPrecheckPayload()) {
        this.fetchPrecheck({ silent: true })
      }
    },
    buildPrecheckPayload() {
      const payload = {}

      if (this.host?.id) {
        payload.deviceId = this.host.id
      }

      const directHost = this.host?.winrmHost
        || this.host?.winrm_host
        || this.host?.privateIp
        || this.host?.sshIp
        || this.host?.publicIp
      const username = this.host?.sshName || this.host?.username
      const password = this.host?.password
        || this.host?.sshPassword
        || this.host?.ssh_password
        || this.host?.credentialPassword
      const domain = this.host?.domain || this.host?.remoteDomain || this.host?.remote_domain || ''
      const winrmPort = Number(this.host?.winrmPort ?? this.host?.winrm_port)

      if (directHost && username && password) {
        payload.host = directHost
        payload.username = username
        payload.password = password
        if (domain) {
          payload.domain = domain
        }
        if (Number.isInteger(winrmPort) && winrmPort > 0) {
          payload.winrmPort = winrmPort
        }
      }

      return payload
    },
    hasPrecheckPayload() {
      const payload = this.buildPrecheckPayload()
      return !!(payload.deviceId || (payload.host && payload.username && payload.password))
    },
    async fetchExporterStatus({ silent = false } = {}) {
      if (!this.host?.id) return false

      try {
        this.exporterStatus = {
          ...this.exporterStatus,
          loading: true
        }
        const { data: res } = await this.$api.getWindowsExporterStatus(this.host.id)
        if (res?.code !== 200) {
          throw new Error(res?.message || '查询状态失败')
        }

        const nextStatus = this.normalizeExporterStatus(res.data)
        this.exporterStatus = nextStatus
        this.handlePollingState(nextStatus.status)
        return true
      } catch (error) {
        this.exporterStatus = {
          ...this.exporterStatus,
          loading: false
        }
        if (!silent) {
          this.$message.error(`查询状态失败: ${error?.message || '未知错误'}`)
        }
        return false
      }
    },
    async fetchPrecheck({ silent = false } = {}) {
      const payload = this.buildPrecheckPayload()
      if (!payload.deviceId && !(payload.host && payload.username && payload.password)) return false

      try {
        this.precheck = {
          ...this.precheck,
          loading: true,
          errorMessage: ''
        }
        const { data: res } = await this.$api.precheckWindowsExporter(payload)
        if (res?.code !== 200) {
          throw new Error(res?.message || '环境检测失败')
        }

        this.precheck = this.normalizePrecheckResult(res.data || {})
        return this.precheck.ready
      } catch (error) {
        this.precheck = {
          ...this.getDefaultPrecheckResult(),
          checked: true,
          errorMessage: error?.message || '环境检测失败'
        }
        if (!silent) {
          this.$message.error(`环境检测失败: ${error?.message || '未知错误'}`)
        }
        return false
      }
    },
    handlePollingState(status) {
      if (this.pendingAction === 'install' && [1, 3, 4].includes(status)) {
        this.stopPolling()
        this.fetchPrecheck({ silent: true })
        if (status === 1) {
          this.$message.success('Windows Exporter 安装完成')
        } else if (status === 3) {
          this.$message.warning('Windows Exporter 已安装，但服务未运行')
        } else {
          this.$message.error('Windows Exporter 安装失败')
        }
        this.pendingAction = ''
        return
      }

      if (this.pendingAction === 'uninstall' && status === 0) {
        this.stopPolling()
        this.fetchPrecheck({ silent: true })
        this.$message.success('Windows Exporter 已卸载')
        this.pendingAction = ''
        return
      }

      if ([2, 5].includes(status) || this.pendingAction) {
        this.startPolling()
        return
      }

      this.stopPolling()
    },
    startPolling() {
      if (this.pollingTimer) return
      this.pollingTimer = window.setInterval(() => {
        this.fetchExporterStatus({ silent: true })
      }, this.pollingInterval)
    },
    stopPolling() {
      if (!this.pollingTimer) return
      window.clearInterval(this.pollingTimer)
      this.pollingTimer = null
    },
    async handleInstall() {
      if (!this.host?.id) return
      if (!this.canInstall) {
        this.$message.warning('环境检测未通过，暂时不能安装 Windows Exporter')
        return
      }

      try {
        await this.$confirm(
          `确认在主机 "${this.host.hostName || this.host.id}" 上安装 Windows Exporter？`,
          '确认安装',
          {
            confirmButtonText: '确定',
            cancelButtonText: '取消',
            type: 'warning'
          }
        )
      } catch (_) {
        return
      }

      try {
        this.installSubmitting = true
        const { data: res } = await this.$api.installWindowsExporter({
          deviceId: this.host.id
        })
        if (res?.code !== 200) {
          throw new Error(res?.message || '安装任务提交失败')
        }

        this.pendingAction = 'install'
        this.exporterStatus = {
          ...this.exporterStatus,
          loading: false,
          status: 2
        }
        this.$message.success(res?.data?.message || '安装任务已提交')
        this.startPolling()
      } catch (error) {
        this.$message.error(`安装失败: ${error?.message || '未知错误'}`)
      } finally {
        this.installSubmitting = false
      }
    },
    async handleUninstall() {
      if (!this.host?.id) return

      try {
        await this.$confirm(
          `确认从主机 "${this.host.hostName || this.host.id}" 卸载 Windows Exporter？`,
          '确认卸载',
          {
            confirmButtonText: '确定',
            cancelButtonText: '取消',
            type: 'warning'
          }
        )
      } catch (_) {
        return
      }

      try {
        this.uninstallSubmitting = true
        const { data: res } = await this.$api.uninstallWindowsExporter(this.host.id)
        if (res?.code !== 200) {
          throw new Error(res?.message || '卸载任务提交失败')
        }

        this.pendingAction = 'uninstall'
        this.$message.success(res?.data?.message || '卸载任务已提交')
        this.startPolling()
      } catch (error) {
        this.$message.error(`卸载失败: ${error?.message || '未知错误'}`)
      } finally {
        this.uninstallSubmitting = false
      }
    },
    async handleRefreshAll() {
      await Promise.allSettled([
        this.fetchExporterStatus(),
        this.fetchPrecheck()
      ])
    },
    async handleDownloadInitScript() {
      try {
        this.downloadingScript = true
        const res = await this.$api.downloadWindowsExporterInitScript()
        const blob = new Blob([res.data], { type: 'application/octet-stream' })
        const disposition = res.headers?.['content-disposition'] || ''
        const filenameMatch = disposition.match(/filename\*=UTF-8''([^;]+)|filename=\"?([^\";]+)\"?/)
        const filename = decodeURIComponent(filenameMatch?.[1] || filenameMatch?.[2] || 'init_winrm.ps1')
        const url = window.URL.createObjectURL(blob)
        const link = document.createElement('a')
        link.href = url
        link.download = filename
        document.body.appendChild(link)
        link.click()
        document.body.removeChild(link)
        window.URL.revokeObjectURL(url)
        this.$message.success('WinRM 初始化脚本已开始下载')
      } catch (error) {
        this.$message.error(`脚本下载失败: ${error?.message || '未知错误'}`)
      } finally {
        this.downloadingScript = false
      }
    },
    async handleShowInstallLog() {
      if (!this.host?.id) {
        this.$message.warning('缺少主机 ID，无法获取安装日志')
        return
      }

      try {
        this.installLogLoading = true
        this.installLogDialogVisible = true
        this.installLogActiveTab = 'installLog'
        const { data: res } = await this.$api.getWindowsExporterInstallLog(this.host.id)
        if (res?.code !== 200) {
          throw new Error(res?.message || '获取安装日志失败')
        }
        this.installLogData = {
          ...this.getDefaultInstallLogData(),
          ...(res.data || {})
        }
      } catch (error) {
        this.installLogDialogVisible = false
        this.$message.error(`获取安装日志失败: ${error?.message || '未知错误'}`)
      } finally {
        this.installLogLoading = false
      }
    },
    handleClose() {
      this.stopPolling()
      this.pendingAction = ''
      this.$emit('change', false)
    }
  },
  beforeUnmount() {
    this.stopPolling()
  }
}
</script>

<style scoped>
.exporter-dialog {
  --aiops-surface-cool: rgba(247, 250, 255, 0.82);
  --aiops-surface-strong: rgba(255, 255, 255, 0.92);
  --aiops-surface-warm: rgba(255, 253, 248, 0.92);
  --aiops-surface-warm-strong: rgba(255, 248, 239, 0.96);
  --aiops-text-primary: #172133;
  --aiops-text-secondary: #5f6b7c;
  --aiops-text-muted: #8d97a8;
  --aiops-border-soft: rgba(22, 34, 56, 0.06);
  --aiops-border-cool: #dfe7f2;
  --aiops-border-mid: rgba(207, 217, 230, 0.92);
  --aiops-brand: #4c8bf5;
  --aiops-brand-soft: rgba(76, 139, 245, 0.12);
  --aiops-success: #3fb97f;
  --aiops-warning: #f3b24b;
  --aiops-danger: #ef6b73;
  --aiops-shadow-soft: 0 20px 60px rgba(17, 24, 39, 0.07);
  --aiops-shadow-panel: 0 36px 110px rgba(15, 23, 42, 0.11);
  display: flex;
  flex-direction: column;
  gap: 10px;
  color: var(--aiops-text-primary);
}

:deep(.windows-exporter-dialog.el-dialog) {
  border-radius: 30px;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
  box-shadow: var(--aiops-shadow-panel);
}

:deep(.windows-exporter-dialog .el-dialog__header) {
  padding: 14px 24px 0;
}

:deep(.windows-exporter-dialog .el-dialog__title) {
  font-size: 18px;
  font-weight: 700;
  color: var(--aiops-text-primary);
  letter-spacing: 0.2px;
}

:deep(.windows-exporter-dialog .el-dialog__body) {
  padding: 10px 24px 14px;
  background:
    radial-gradient(circle at top left, rgba(76, 139, 245, 0.05), transparent 20%),
    linear-gradient(180deg, #f8fafc 0%, #f5f8fc 100%);
}

:deep(.windows-exporter-dialog .el-dialog__footer) {
  padding: 0 24px 16px;
}

.hero-panel {
  position: relative;
  display: flex;
  justify-content: space-between;
  align-items: stretch;
  gap: 16px;
  padding: 18px 18px 16px;
  border-radius: 22px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border: 1px solid rgba(223, 231, 242, 0.92);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.72),
    0 18px 40px rgba(17, 24, 39, 0.05);
}

.hero-copy {
  flex: 1;
  padding-right: 200px;
}

.hero-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 8px;
}

.hero-title {
  font-size: 18px;
  font-weight: 700;
  color: var(--aiops-text-primary);
  margin-bottom: 4px;
}

.hero-subtitle {
  font-size: 13px;
  color: var(--aiops-text-muted);
}

.hero-description {
  font-size: 13px;
  line-height: 1.5;
  color: var(--aiops-text-secondary);
}

.hero-actions {
  position: absolute;
  top: 14px;
  right: 16px;
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: nowrap;
}

.hero-actions :deep(.el-button) {
  min-width: 92px;
  height: 30px;
  padding: 0 10px;
  border-radius: 11px;
  font-size: 12px;
  font-weight: 600;
  border-color: transparent;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.hero-actions :deep(.el-button:hover) {
  transform: translateY(-1px);
}

.hero-actions :deep(.el-button--primary) {
  color: #fff;
  background: linear-gradient(180deg, #6da0f7 0%, #4c8bf5 100%);
  box-shadow: 0 6px 12px rgba(76, 139, 245, 0.15);
}

.hero-actions :deep(.el-button--primary:hover) {
  box-shadow: 0 8px 14px rgba(76, 139, 245, 0.18);
}

.hero-actions :deep(.el-button--success) {
  color: #fff;
  background: linear-gradient(180deg, #50bf86 0%, #3fb97f 100%);
  box-shadow: 0 6px 12px rgba(63, 185, 127, 0.15);
}

.hero-actions :deep(.el-button--success:hover) {
  box-shadow: 0 8px 14px rgba(63, 185, 127, 0.18);
}

.hero-actions :deep(.el-button--warning) {
  color: #fff;
  background: linear-gradient(180deg, #f5bd57 0%, #f3b24b 100%);
  box-shadow: 0 6px 12px rgba(243, 178, 75, 0.15);
}

.hero-actions :deep(.el-button--warning:hover) {
  box-shadow: 0 8px 14px rgba(243, 178, 75, 0.18);
}

.panel-stack {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.surface-panel {
  position: relative;
  padding: 16px;
  border-radius: 22px;
  background: linear-gradient(135deg, var(--aiops-surface-warm-strong), var(--aiops-surface-cool));
  border: 1px solid var(--aiops-border-cool);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.78),
    0 18px 42px rgba(17, 24, 39, 0.05);
}

.surface-panel::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 42px;
  border-radius: 22px 22px 0 0;
  background: linear-gradient(90deg, rgba(255, 249, 240, 0.95), rgba(248, 248, 246, 0.86), rgba(237, 242, 253, 0.92));
  opacity: 0.92;
  pointer-events: none;
}

.surface-panel > * {
  position: relative;
  z-index: 1;
}

.surface-panel-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 12px;
}

.section-status-cluster {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.section-status-cluster :deep(.el-tag),
.table-badges :deep(.el-tag) {
  height: 22px;
  padding: 0 8px;
  border-radius: 999px;
  font-weight: 600;
  font-size: 11px;
}

.panel-eyebrow {
  margin-bottom: 4px;
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.9px;
  color: var(--aiops-text-muted);
}

.panel-title {
  font-size: 17px;
  font-weight: 700;
  color: var(--aiops-text-primary);
}

.surface-panel-tag {
  display: inline-flex;
  align-items: center;
  padding: 6px 10px;
  border-radius: 999px;
  border: 1px solid rgba(207, 217, 230, 0.92);
  background: rgba(255, 255, 255, 0.74);
  font-size: 11px;
  font-weight: 600;
  color: var(--aiops-text-secondary);
}

.surface-panel-tag.is-success {
  color: var(--aiops-success);
  border-color: rgba(63, 185, 127, 0.2);
  background: rgba(63, 185, 127, 0.08);
}

.surface-panel-tag.is-danger {
  color: var(--aiops-danger);
  border-color: rgba(239, 107, 115, 0.22);
  background: rgba(239, 107, 115, 0.08);
}

.table-shell {
  overflow: hidden;
  border-radius: 16px;
  border: 1px solid rgba(223, 231, 242, 0.94);
  background: rgba(255, 255, 255, 0.76);
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
}

.data-table th,
.data-table td {
  padding: 8px 12px;
  font-size: 12px;
  line-height: 1.45;
  border-bottom: 1px solid rgba(223, 231, 242, 0.82);
  vertical-align: middle;
  word-break: break-word;
}

.data-table tr:last-child th,
.data-table tr:last-child td {
  border-bottom: none;
}

.data-table thead th {
  background: linear-gradient(180deg, rgba(248, 250, 253, 0.94), rgba(243, 247, 252, 0.92));
  font-size: 11px;
  font-weight: 600;
  color: var(--aiops-text-muted);
  text-align: left;
}

.host-data-table th,
.precheck-data-table th:first-child {
  font-weight: 600;
  color: var(--aiops-text-secondary);
  background: rgba(247, 250, 255, 0.72);
}

.host-data-table th {
  width: 84px;
}

.host-data-table td {
  color: var(--aiops-text-primary);
}

.precheck-meta {
  margin-top: 8px;
  font-size: 11px;
  color: var(--aiops-text-muted);
  text-align: right;
}

.table-badges {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.table-inline-text {
  font-size: 11px;
  color: var(--aiops-text-secondary);
}

.check-name-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.check-state-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  flex-shrink: 0;
  background: var(--aiops-text-muted);
}

.check-state-dot.is-pass {
  background: var(--aiops-success);
}

.check-state-dot.is-fail {
  background: var(--aiops-danger);
}

.data-table td.is-mono,
.data-table .is-mono {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 11px;
}

.data-table td.is-soft {
  color: var(--aiops-text-secondary);
}

.workflow-alert {
  border-radius: 18px;
}

:deep(.workflow-alert.el-alert) {
  padding: 13px 16px;
  border-radius: 18px;
  border: 1px solid rgba(223, 231, 242, 0.92);
}

:deep(.workflow-alert.el-alert--success) {
  background: linear-gradient(180deg, rgba(244, 252, 247, 0.94), rgba(255, 255, 255, 0.9));
}

:deep(.workflow-alert.el-alert--warning) {
  background: linear-gradient(180deg, rgba(255, 249, 241, 0.96), rgba(255, 255, 255, 0.9));
}

:deep(.workflow-alert.el-alert--error) {
  background: linear-gradient(180deg, rgba(255, 248, 248, 0.96), rgba(255, 255, 255, 0.92));
}

:deep(.workflow-alert.el-alert--info) {
  background: linear-gradient(180deg, rgba(246, 249, 255, 0.94), rgba(255, 255, 255, 0.9));
}

:deep(.workflow-alert .el-alert__title) {
  font-size: 13px;
  line-height: 1.65;
  font-weight: 600;
  color: var(--aiops-text-secondary);
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.dialog-footer :deep(.el-button) {
  min-width: 84px;
  height: 32px;
  border-radius: 10px;
  font-size: 12px;
  font-weight: 600;
}

.footer-action-button {
  min-width: 104px !important;
}

.footer-action-button.is-warning :deep(.el-icon),
.footer-action-button.is-danger :deep(.el-icon) {
  margin-right: 2px;
}

.dialog-footer :deep(.el-button--warning.footer-action-button) {
  border-color: transparent;
  color: #fff;
  background: linear-gradient(180deg, #f3b24b 0%, #eda940 100%);
  box-shadow: 0 10px 18px rgba(243, 178, 75, 0.18);
}

.dialog-footer :deep(.el-button--danger.footer-action-button) {
  color: #fff;
  border-color: transparent;
  background: linear-gradient(180deg, #ef6b73 0%, #e65b65 100%);
  box-shadow: 0 10px 18px rgba(239, 107, 115, 0.2);
}

.dialog-footer :deep(.el-button--danger.footer-action-button:hover),
.dialog-footer :deep(.el-button--danger.footer-action-button:focus) {
  color: #fff;
  border-color: transparent;
  background: linear-gradient(180deg, #f2787f 0%, #eb656f 100%);
}

.dialog-footer :deep(.el-button--danger.footer-action-button.is-disabled),
.dialog-footer :deep(.el-button--danger.footer-action-button.is-disabled:hover) {
  color: rgba(255, 255, 255, 0.88);
  border-color: transparent;
  background: linear-gradient(180deg, rgba(239, 107, 115, 0.68) 0%, rgba(230, 91, 101, 0.68) 100%);
  box-shadow: none;
}

:deep(.install-log-dialog.el-dialog) {
  border-radius: 24px;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
}

:deep(.install-log-dialog .el-dialog__body) {
  padding: 12px 18px 14px;
  background: linear-gradient(180deg, #f8fafc 0%, #f5f8fc 100%);
}

.install-log-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.install-log-summary {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border-radius: 16px;
  border: 1px solid rgba(223, 231, 242, 0.94);
  background: rgba(255, 255, 255, 0.84);
}

.install-log-summary-left {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.install-log-summary-left strong {
  font-size: 14px;
  color: var(--aiops-text-primary);
}

.install-log-summary-left span {
  font-size: 12px;
  color: var(--aiops-text-muted);
  word-break: break-all;
}

.install-log-summary-right {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.install-log-tabs :deep(.el-tabs__header) {
  margin-bottom: 10px;
}

.install-log-tabs :deep(.el-tabs__item) {
  height: 32px;
  font-size: 12px;
}

.log-block {
  margin: 0;
  min-height: 340px;
  max-height: 52vh;
  overflow: auto;
  padding: 14px;
  border-radius: 16px;
  border: 1px solid rgba(223, 231, 242, 0.94);
  background: #f8fafc;
  color: #243247;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
}

.install-log-footer {
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 768px) {
  :deep(.windows-exporter-dialog .el-dialog__body) {
    padding: 12px 16px 16px;
  }

  :deep(.windows-exporter-dialog .el-dialog__header),
  :deep(.windows-exporter-dialog .el-dialog__footer) {
    padding-left: 16px;
    padding-right: 16px;
  }

  .hero-panel,
  .hero-head,
  .surface-panel-head,
  .dialog-footer,
  .install-log-summary {
    flex-direction: column;
  }

  .hero-actions {
    position: static;
    min-width: 100%;
    flex-direction: column;
    align-items: stretch;
  }

  .hero-copy {
    padding-right: 0;
  }

  .footer-action-button {
    width: 100%;
  }

  .install-log-summary-right {
    width: 100%;
    justify-content: flex-start;
  }

  .data-table,
  .data-table tbody,
  .data-table tr,
  .data-table th,
  .data-table td,
  .table-badges {
    display: block;
    width: 100%;
  }

  .data-table thead {
    display: none;
  }

  .data-table tr {
    padding: 6px 0;
  }

  .data-table th,
  .data-table td {
    padding: 8px 12px;
    border-bottom: none;
  }

  .host-data-table tr,
  .precheck-data-table tr {
    border-bottom: 1px solid rgba(223, 231, 242, 0.82);
  }

  .host-data-table tr:last-child,
  .precheck-data-table tr:last-child {
    border-bottom: none;
  }
}
</style>
