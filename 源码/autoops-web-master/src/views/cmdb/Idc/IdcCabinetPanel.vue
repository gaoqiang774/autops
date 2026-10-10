<template>
  <div class="idc-cabinet-panel">
    <div class="toolbar">
      <div class="toolbar-main">
        <div class="toolbar-tags">
          <span>当前机柜</span>
          <el-tag v-if="selectedCabinetName" type="primary" effect="light">{{ selectedCabinetName }}</el-tag>
          <el-tag v-if="selectedRoomName" type="success" effect="light">机房：{{ selectedRoomName }}</el-tag>
          <el-tag v-if="hasCabinetSelection" type="info" effect="light">{{ totalU }}U</el-tag>
          <span v-if="!hasCabinetSelection" class="hint-text">请先从左侧选择机柜</span>
        </div>
        <div class="toolbar-actions">
          <el-button size="small" type="primary" :loading="loadingLayout" :disabled="!hasCabinetSelection" @click="reloadLayout">
            <el-icon><RefreshRight /></el-icon>刷新布局
          </el-button>
        </div>
      </div>
      <div class="toolbar-divider"></div>
      <div class="view-tabs">
        <button type="button" class="view-tab" :class="{ active: activeView === 'room' }" @click="$emit('switch-view', 'room')">机房</button>
        <button type="button" class="view-tab" :class="{ active: activeView === 'cabinet' }" @click="$emit('switch-view', 'cabinet')">机柜</button>
        <button type="button" class="view-tab" :class="{ active: activeView === 'physical' }" @click="$emit('switch-view', 'physical')">物理机</button>
        <button type="button" class="view-tab" :class="{ active: activeView === 'vm' }" @click="$emit('switch-view', 'vm')">虚拟机</button>
        <button type="button" class="view-tab" :class="{ active: activeView === 'hardware' }" @click="$emit('switch-view', 'hardware')">硬件监控</button>
      </div>
    </div>

    <div class="panel-card">
      <el-empty v-if="!hasCabinetSelection" description="请在左侧机柜树中点击一个机柜后查看 42U 布局" />
      <template v-else>
        <div class="stats-grid">
          <div class="stat-card">
            <span>总U位</span>
            <strong>{{ stats.totalU }}</strong>
          </div>
          <div class="stat-card">
            <span>已使用U位</span>
            <strong>{{ stats.usedU }}</strong>
          </div>
          <div class="stat-card">
            <span>空闲U位</span>
            <strong>{{ stats.freeU }}</strong>
          </div>
          <div class="stat-card">
            <span>使用率</span>
            <strong>{{ formatRate(stats.usageRate) }}</strong>
          </div>
          <div class="stat-card">
            <span>空闲率</span>
            <strong>{{ formatRate(stats.idleRate) }}</strong>
          </div>
          <div class="stat-card">
            <span>满柜状态</span>
            <strong>{{ stats.isFull ? '是' : '否' }}</strong>
          </div>
        </div>

        <div v-if="cabinetAlertSummary.visible" class="cabinet-alert-summary" :class="cabinetAlertSummary.level">
          <div class="cabinet-alert-head">
            <strong>{{ cabinetAlertSummary.title }}</strong>
            <div class="cabinet-alert-tags">
              <el-tag v-if="cabinetAlertSummary.criticalCount" size="small" type="danger" effect="light">
                严重 {{ cabinetAlertSummary.criticalCount }} 台
              </el-tag>
              <el-tag v-if="cabinetAlertSummary.unknownCount" size="small" type="info" effect="light">
                未采集 {{ cabinetAlertSummary.unknownCount }} 台
              </el-tag>
            </div>
          </div>
          <p>{{ cabinetAlertSummary.description }}</p>
          <div v-if="cabinetAlertSummary.issues.length" class="cabinet-alert-issues">
            <div v-for="issue in cabinetAlertSummary.issues" :key="issue.assetId || issue.ip" class="cabinet-alert-issue">
              <button
                type="button"
                class="cabinet-alert-ip"
                :disabled="!canOpenHardwareMonitor(issue)"
                @click="openHardwareMonitor(issue)"
              >
                {{ issue.ip || issue.remoteManagementCard || '-' }}
              </button>
              <em>{{ formatHardwareMetrics(issue.metrics) }}</em>
            </div>
          </div>
        </div>

        <div v-loading="loadingLayout" class="panel-main">
          <section class="rack-section">
            <div class="section-head">
              <h3>机柜可视化</h3>
              <p>42U Vertical Scale</p>
            </div>
            <div class="rack-legend">
              <span class="legend-item"><i class="legend-dot placed"></i>已上架</span>
              <span class="legend-item"><i class="legend-dot critical"></i>严重</span>
              <span class="legend-item"><i class="legend-dot overlap"></i>重叠</span>
              <span class="legend-item"><i class="legend-dot out-of-range"></i>越界</span>
              <span class="legend-item"><i class="legend-dot unplaced"></i>未上架</span>
            </div>
            <div class="rack-wrapper">
              <div class="cabinet-iso">
                <div class="cabinet-top-face"></div>
                <div class="cabinet-side-face"></div>

                <div class="cabinet-shell">
                  <div class="cabinet-head">
                    <span class="cabinet-title">{{ selectedCabinetName || '机柜' }}</span>
                    <span class="cabinet-subtitle">{{ totalU }}U Cabinet</span>
                  </div>
                  <div class="cabinet-core">
                    <div class="rack-scale rack-scale-left" :style="{ height: `${rackHeight}px` }">
                      <span
                        v-for="u in scaleLabels"
                        :key="`scale-left-${u}`"
                        class="rack-scale-label"
                        :style="{ bottom: `${(u - 0.5) * unitHeight}px` }"
                      >
                        {{ u }}
                      </span>
                    </div>

                    <div class="cabinet-door" :style="{ height: `${rackHeight + 18}px` }">
                      <span class="cabinet-side left"></span>
                      <span class="cabinet-side right"></span>
                      <span class="cabinet-rail rail-left"></span>
                      <span class="cabinet-rail rail-right"></span>
                      <span class="cabinet-handle"></span>

                      <div class="rack-body" :style="{ height: `${rackHeight}px` }">
                        <span
                          v-for="u in totalU"
                          :key="`u-line-${u}`"
                          class="u-line"
                          :style="{ bottom: `${u * unitHeight}px` }"
                        ></span>

                        <el-tooltip
                          v-for="server in rackServers"
                          :key="`server-${server.id}`"
                          placement="top"
                          :show-after="80"
                          :disabled="!server.id || dragState.active"
                        >
                          <template #content>
                            <div>{{ server.displayName }}</div>
                            <div>IP：{{ server.ip || '-' }}</div>
                            <div>位置：{{ getVisualStartU(server) }}U / {{ server.cabinetUSize }}U</div>
                            <div>状态：{{ statusLabel(server.layoutStatus) }}</div>
                          </template>
                          <button
                            type="button"
                            class="server-block"
                            :class="[serverStatusClass(server), { selected: server.id === selectedServerId, 'is-dragging': isDraggingServer(server.id) }]"
                            :style="serverBlockStyle(server)"
                            @pointerdown.stop.prevent="handleServerPointerDown($event, server)"
                            @pointermove.stop.prevent="handleServerPointerMove($event)"
                            @pointerup.stop.prevent="handleServerPointerUp($event)"
                            @pointercancel.stop.prevent="handleServerPointerUp($event)"
                            @click="handleServerClick(server)"
                          >
                            <span class="server-name">{{ server.displayName }}</span>
                            <span class="server-meta">{{ getVisualStartU(server) }}U / {{ server.cabinetUSize }}U</span>
                          </button>
                        </el-tooltip>
                      </div>
                    </div>

                    <div class="rack-scale rack-scale-right" :style="{ height: `${rackHeight}px` }">
                      <span
                        v-for="u in scaleLabels"
                        :key="`scale-right-${u}`"
                        class="rack-scale-label"
                        :style="{ bottom: `${(u - 0.5) * unitHeight}px` }"
                      >
                        {{ u }}
                      </span>
                    </div>
                  </div>

                  <div class="cabinet-foot">
                    <span></span>
                    <span></span>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <aside class="detail-section">
            <div class="section-head">
              <h3>服务器详情</h3>
              <p>IDC Asset Details</p>
            </div>
            <div v-loading="loadingDetail" class="detail-card">
              <el-empty v-if="!selectedServerDetail" description="点击左侧服务器块查看详情" />
              <template v-else>
                <el-descriptions :column="2" border size="small">
                  <el-descriptions-item label="IP">{{ selectedServerDetail.ip || '-' }}</el-descriptions-item>
                  <el-descriptions-item label="品牌">{{ selectedServerDetail.brand || '-' }}</el-descriptions-item>
                  <el-descriptions-item label="型号">{{ selectedServerDetail.model || '-' }}</el-descriptions-item>
                  <el-descriptions-item label="CPU">{{ selectedServerDetail.cpu || '-' }}</el-descriptions-item>
                  <el-descriptions-item label="内存">{{ selectedServerDetail.memory || '-' }}</el-descriptions-item>
                  <el-descriptions-item label="磁盘">{{ selectedServerDetail.disk || '-' }}</el-descriptions-item>
                  <el-descriptions-item label="用途">{{ selectedServerDetail.purpose || '-' }}</el-descriptions-item>
                  <el-descriptions-item label="位置(U)">{{ selectedServerDetail.cabinetPosition || '-' }}</el-descriptions-item>
                  <el-descriptions-item label="U高度">{{ selectedServerDetail.cabinetUSize || '-' }}</el-descriptions-item>
                </el-descriptions>

                <div class="position-form">
                  <h4>手动调整位置</h4>
                  <div class="drag-save-config">
                    <span>拖拽结束自动保存</span>
                    <el-switch v-model="autoSaveAfterDrag" />
                  </div>
                  <el-form :inline="true" label-width="70px">
                    <el-form-item label="位置(U)">
                      <el-input-number v-model="positionForm.cabinetPosition" :min="1" :max="totalU" controls-position="right" />
                    </el-form-item>
                    <el-form-item label="U高度">
                      <el-input-number v-model="positionForm.cabinetUSize" :min="1" :max="totalU" controls-position="right" />
                    </el-form-item>
                    <el-form-item>
                      <el-button type="primary" size="small" :loading="savingPosition" @click="submitPosition">保存位置</el-button>
                    </el-form-item>
                  </el-form>
                </div>
              </template>
            </div>

            <div class="unplaced-card">
              <div class="sub-head">
                <h4>未上架服务器</h4>
                <span>{{ unplacedServers.length }}</span>
              </div>
              <el-empty v-if="!unplacedServers.length" description="暂无未上架服务器" />
              <el-table v-else :data="unplacedServers" stripe size="small" height="120">
                <el-table-column prop="ip" label="IP" min-width="140" show-overflow-tooltip />
                <el-table-column label="品牌/型号" min-width="160" show-overflow-tooltip>
                  <template #default="{ row }">{{ row.brand || '-' }} / {{ row.model || '-' }}</template>
                </el-table-column>
                <el-table-column prop="purpose" label="用途" min-width="120" show-overflow-tooltip />
                <el-table-column label="操作" width="80" fixed="right">
                  <template #default="{ row }">
                    <el-button link type="primary" @click="selectServer(row)">详情</el-button>
                  </template>
                </el-table-column>
              </el-table>
            </div>
          </aside>
        </div>
      </template>
    </div>
  </div>
</template>

<script>
const DEFAULT_TOTAL_U = 42
const UNIT_HEIGHT = 8

const makePositionForm = () => ({
  cabinetPosition: 1,
  cabinetUSize: 1
})

const makeEmptyLayout = () => ({
  stats: {
    totalU: DEFAULT_TOTAL_U,
    usedU: 0,
    freeU: DEFAULT_TOTAL_U,
    usageRate: 0,
    idleRate: 100,
    isFull: false
  },
  hardwareStatus: 'normal',
  hardwareSummary: {
    normalAssetCount: 0,
    warningAssetCount: 0,
    criticalAssetCount: 0,
    unknownAssetCount: 0,
    alertAssetCount: 0,
    issues: []
  },
  servers: []
})

const makeDragState = () => ({
  active: false,
  moved: false,
  pointerId: null,
  assetId: null,
  startY: 0,
  startU: 0,
  currentU: 0,
  cabinetUSize: 1
})

export default {
  name: 'IdcCabinetPanel',
  emits: ['switch-view', 'open-hardware-monitor'],
  props: {
    activeView: {
      type: String,
      default: 'cabinet'
    },
    selectedCabinet: {
      type: Object,
      default: null
    },
    selectedRoom: {
      type: Object,
      default: null
    }
  },
  data() {
    return {
      loadingLayout: false,
      loadingDetail: false,
      savingPosition: false,
      layoutInfo: makeEmptyLayout(),
      selectedServerId: null,
      selectedServerDetail: null,
      positionForm: makePositionForm(),
      lastLoadedKey: '',
      autoSaveAfterDrag: true,
      suppressClickOnce: false,
      dragState: makeDragState()
    }
  },
  computed: {
    cabinetSelectionKey() {
      if (!this.selectedCabinet) return ''
      const cabinetId = this.num(this.pick(this.selectedCabinet, ['id', 'cabinetId', 'cabinet_id']), '')
      if (cabinetId) return `id-${cabinetId}`
      const roomId = this.num(this.pick(this.selectedCabinet, ['roomId', 'room_id', 'id']), '')
      const cabinetName = String(this.pick(this.selectedCabinet, ['name', 'cabinetName', 'cabinet_name'], '')).trim()
      if (!roomId || !cabinetName) return ''
      return `name-${roomId}-${cabinetName}`
    },
    hasCabinetSelection() {
      return !!this.cabinetSelectionKey
    },
    selectedCabinetName() {
      return String(this.pick(this.selectedCabinet, ['name', 'cabinetName', 'cabinet_name'], ''))
    },
    selectedRoomName() {
      return String(this.pick(this.selectedRoom, ['name', 'roomName', 'room_name'], ''))
    },
    stats() {
      return this.layoutInfo.stats || makeEmptyLayout().stats
    },
    hardwareSummary() {
      const summary = this.pick(this.layoutInfo, ['hardwareSummary', 'hardware_summary'], null) ||
        this.pick(this.selectedCabinet, ['hardwareSummary', 'hardware_summary'], {}) ||
        {}

      return {
        normalAssetCount: this.num(this.pick(summary, ['normalAssetCount', 'normal_asset_count']), 0),
        warningAssetCount: this.num(this.pick(summary, ['warningAssetCount', 'warning_asset_count']), 0),
        criticalAssetCount: this.num(this.pick(summary, ['criticalAssetCount', 'critical_asset_count']), 0),
        unknownAssetCount: this.num(this.pick(summary, ['unknownAssetCount', 'unknown_asset_count']), 0),
        alertAssetCount: this.num(this.pick(summary, ['alertAssetCount', 'alert_asset_count']), 0),
        issues: this.list(this.pick(summary, ['issues', 'issueList', 'issue_list'], [])).map(item => this.normalizeHardwareIssue(item))
      }
    },
    cabinetHardwareStatus() {
      return String(
        this.pick(this.layoutInfo, ['hardwareStatus', 'hardware_status'],
          this.pick(this.selectedCabinet, ['hardwareStatus', 'hardware_status'], 'normal'))
      ).toLowerCase() || 'normal'
    },
    cabinetAlertSummary() {
      const summary = this.hardwareSummary
      const level = summary.criticalAssetCount > 0 ? 'critical' : 'normal'
      const hasAlert = level === 'critical'
      const hasUnknown = summary.unknownAssetCount > 0 && !hasAlert

      if (!hasAlert && !hasUnknown) {
        return {
          visible: false,
          level: 'normal',
          title: '',
          description: '',
          criticalCount: 0,
          warningCount: 0,
          unknownCount: 0,
          issues: []
        }
      }

      const title = hasAlert ? '物理机严重硬件异常' : '部分物理机未采集硬件状态'
      const description = hasAlert
        ? '该机柜内有物理机的严重硬件状态类指标异常，仅聚合 critical 级别的磁盘健康、散热系统、RAID/控制器、内存/系统稳定性等当前状态。'
        : '该机柜内部分物理机没有远程管理卡或暂未匹配到硬件监控指标。'

      return {
        visible: true,
        level: hasAlert ? level : 'unknown',
        title,
        description,
        criticalCount: summary.criticalAssetCount,
        warningCount: 0,
        unknownCount: summary.unknownAssetCount,
        issues: summary.issues
      }
    },
    totalU() {
      const value = this.num(this.stats.totalU, DEFAULT_TOTAL_U)
      return value > 0 ? value : DEFAULT_TOTAL_U
    },
    unitHeight() {
      return UNIT_HEIGHT
    },
    rackHeight() {
      return this.totalU * this.unitHeight
    },
    scaleLabels() {
      return Array.from({ length: this.totalU }, (_, index) => index + 1)
    },
    rackServers() {
      return this.layoutInfo.servers
        .filter(item => item.layoutStatus !== 'unplaced')
        .sort((a, b) => a.startU - b.startU || b.cabinetUSize - a.cabinetUSize || a.id - b.id)
    },
    unplacedServers() {
      return this.layoutInfo.servers
        .filter(item => item.layoutStatus === 'unplaced')
        .sort((a, b) => a.id - b.id)
    }
  },
  watch: {
    activeView: {
      immediate: true,
      handler(value) {
        if (value !== 'cabinet') return
        this.tryLoadLayout(false)
      }
    },
    cabinetSelectionKey() {
      this.lastLoadedKey = ''
      if (this.activeView !== 'cabinet') return
      this.tryLoadLayout(true)
    }
  },
  beforeUnmount() {
    this.clearDragState()
  },
  methods: {
    pick(obj, keys, fallback = '') {
      if (!obj || typeof obj !== 'object') return fallback
      for (const key of keys) {
        const value = obj[key]
        if (value !== undefined && value !== null && value !== '') return value
      }
      return fallback
    },
    num(value, fallback = null) {
      if (value === '' || value === undefined || value === null) return fallback
      const next = Number(value)
      return Number.isNaN(next) ? fallback : next
    },
    list(payload) {
      if (Array.isArray(payload)) return payload
      if (!payload || typeof payload !== 'object') return []
      for (const key of ['list', 'items', 'rows', 'records', 'result', 'assets', 'assetList', 'servers']) {
        if (Array.isArray(payload[key])) return payload[key]
      }
      return []
    },
    formatRate(value) {
      const rate = Number(value)
      if (!Number.isFinite(rate)) return '0.0%'
      return `${rate.toFixed(1)}%`
    },
    parseRate(value, fallback = 0) {
      if (typeof value === 'string' && value.endsWith('%')) {
        return this.num(value.slice(0, -1), fallback)
      }
      return this.num(value, fallback)
    },
    statusClass(status) {
      if (status === 'overlap') return 'status-overlap'
      if (status === 'out_of_range') return 'status-out-of-range'
      if (status === 'unplaced') return 'status-unplaced'
      return 'status-placed'
    },
    serverStatusClass(server) {
      if (this.isCriticalHardwareServer(server)) return 'status-critical'
      return this.statusClass(server?.layoutStatus)
    },
    isCriticalHardwareServer(server) {
      if (!server) return false
      const hardwareStatus = String(server.hardwareStatus || server.hardware_status || '').toLowerCase()
      if (hardwareStatus === 'critical') return true

      return this.cabinetAlertSummary.issues.some(issue => {
        const issueId = this.num(this.pick(issue, ['assetId', 'asset_id', 'id']), '')
        const serverId = this.num(this.pick(server, ['id', 'assetId', 'asset_id']), '')
        if (issueId && serverId && issueId === serverId) return true
        if (issue.ip && server.ip && String(issue.ip) === String(server.ip)) return true
        return issue.remoteManagementCard &&
          server.remoteManagementCard &&
          String(issue.remoteManagementCard) === String(server.remoteManagementCard)
      })
    },
    statusLabel(status) {
      if (status === 'overlap') return '重叠'
      if (status === 'out_of_range') return '越界'
      if (status === 'unplaced') return '未上架'
      return '已上架'
    },
    clamp(value, min, max) {
      return Math.min(Math.max(value, min), max)
    },
    isDraggingServer(assetId) {
      return this.dragState.active && this.dragState.assetId === assetId
    },
    findServerById(assetId) {
      return this.layoutInfo.servers.find(item => item.id === assetId) || null
    },
    getVisualStartU(server) {
      if (!server) return 0
      if (this.isDraggingServer(server.id)) return this.dragState.currentU
      return this.num(server.startU, 0)
    },
    handleServerClick(server) {
      if (this.suppressClickOnce) {
        this.suppressClickOnce = false
        return
      }
      this.selectServer(server)
    },
    syncSelectedPosition(assetId, startU, cabinetUSize) {
      if (!assetId) return
      if (this.selectedServerId === assetId) {
        this.positionForm.cabinetPosition = startU
        this.positionForm.cabinetUSize = cabinetUSize
      }
      if (this.selectedServerDetail?.id === assetId) {
        this.selectedServerDetail = {
          ...this.selectedServerDetail,
          cabinetPosition: startU,
          cabinetUSize
        }
      }
    },
    handleServerPointerDown(event, server) {
      if (!server?.id) return
      if (event.button !== undefined && event.button !== 0) return

      const startU = this.num(server.startU, 1)
      const cabinetUSize = Math.max(1, this.num(server.cabinetUSize, 1))
      const maxStartU = Math.max(1, this.totalU - cabinetUSize + 1)
      const safeStartU = this.clamp(startU, 1, maxStartU)

      this.selectedServerId = server.id
      this.selectedServerDetail = {
        ...this.normalizeAssetDetail(server),
        id: server.id,
        cabinetPosition: safeStartU,
        cabinetUSize
      }
      this.syncSelectedPosition(server.id, safeStartU, cabinetUSize)
      this.loadServerDetail(server.id, server)

      this.dragState = {
        active: true,
        moved: false,
        pointerId: event.pointerId,
        assetId: server.id,
        startY: event.clientY,
        startU: safeStartU,
        currentU: safeStartU,
        cabinetUSize
      }

      const target = event.currentTarget
      if (target?.setPointerCapture && event.pointerId !== undefined) {
        try {
          target.setPointerCapture(event.pointerId)
        } catch (error) {
          // ignore capture failures
        }
      }
    },
    handleServerPointerMove(event) {
      if (!this.dragState.active) return
      if (event.pointerId !== undefined && this.dragState.pointerId !== null && event.pointerId !== this.dragState.pointerId) return

      const deltaU = Math.round((this.dragState.startY - event.clientY) / this.unitHeight)
      const maxStartU = Math.max(1, this.totalU - this.dragState.cabinetUSize + 1)
      const nextU = this.clamp(this.dragState.startU + deltaU, 1, maxStartU)
      if (nextU === this.dragState.currentU) return

      this.dragState = {
        ...this.dragState,
        moved: true,
        currentU: nextU
      }
      this.syncSelectedPosition(this.dragState.assetId, nextU, this.dragState.cabinetUSize)
    },
    async handleServerPointerUp(event) {
      if (!this.dragState.active) return
      if (event.pointerId !== undefined && this.dragState.pointerId !== null && event.pointerId !== this.dragState.pointerId) return

      const target = event.currentTarget
      if (target?.releasePointerCapture && event.pointerId !== undefined) {
        try {
          target.releasePointerCapture(event.pointerId)
        } catch (error) {
          // ignore release failures
        }
      }

      const snapshot = { ...this.dragState }
      this.dragState = makeDragState()

      if (!snapshot.moved || !snapshot.assetId) return

      this.suppressClickOnce = true
      this.applyLocalPosition(snapshot.assetId, snapshot.currentU)
      this.syncSelectedPosition(snapshot.assetId, snapshot.currentU, snapshot.cabinetUSize)

      if (this.autoSaveAfterDrag) {
        await this.submitPosition({ silentSuccess: true })
      } else {
        this.$message.info('拖拽位置已写入，点击“保存位置”后提交到后端')
      }
    },
    clearDragState() {
      this.dragState = makeDragState()
      this.suppressClickOnce = false
    },
    applyLocalPosition(assetId, nextStartU) {
      const updated = this.layoutInfo.servers.map(item => {
        if (item.id !== assetId) return { ...item }
        return {
          ...item,
          startU: nextStartU,
          cabinetPosition: nextStartU,
          placementStatus: item.placementStatus === 'unplaced' ? 'placed' : item.placementStatus
        }
      })
      this.layoutInfo = {
        ...this.layoutInfo,
        servers: this.resolveLayoutStatus(updated, this.totalU)
      }
    },
    serverBlockStyle(server) {
      const startU = this.getVisualStartU(server)
      const bottom = Math.max(0, startU - 1) * this.unitHeight
      const height = Math.max(1, server.cabinetUSize * this.unitHeight - 2)
      return {
        bottom: `${bottom}px`,
        height: `${height}px`,
        zIndex: this.isDraggingServer(server.id) ? 40 : (server.layoutStatus === 'overlap' ? 20 : 10)
      }
    },
    buildLayoutQuery() {
      const cabinetId = this.num(this.pick(this.selectedCabinet, ['id', 'cabinetId', 'cabinet_id']), '')
      if (cabinetId) return { cabinetId }
      const roomId = this.num(this.pick(this.selectedCabinet, ['roomId', 'room_id']), '')
      const cabinetName = String(this.pick(this.selectedCabinet, ['name', 'cabinetName', 'cabinet_name'], '')).trim()
      if (!roomId || !cabinetName) return {}
      return { roomId, cabinetName }
    },
    normalizeLayoutServer(item) {
      const startU = this.num(this.pick(item, ['startU', 'cabinetPosition', 'cabinet_position', 'position']), 0)
      const cabinetUSize = Math.max(1, this.num(this.pick(item, ['cabinetUSize', 'cabinet_u_size', 'uSize', 'u_size']), 1))
      const placementStatus = String(this.pick(item, ['placementStatus', 'placement_status'], '')).toLowerCase()
      return {
        ...item,
        id: this.num(this.pick(item, ['id', 'assetId', 'asset_id']), 0),
        ip: String(this.pick(item, ['ip'], '')),
        brand: String(this.pick(item, ['brand'], '')),
        model: String(this.pick(item, ['model'], '')),
        cpu: String(this.pick(item, ['cpu'], '')),
        memory: String(this.pick(item, ['memory'], '')),
        disk: String(this.pick(item, ['disk'], '')),
        purpose: String(this.pick(item, ['purpose'], '')),
        remoteManagementCard: String(this.pick(item, ['remoteManagementCard', 'remote_management_card'], '')),
        hardwareStatus: String(this.pick(item, ['hardwareStatus', 'hardware_status'], 'unknown')).toLowerCase(),
        hardwareMetrics: this.list(this.pick(item, ['hardwareMetrics', 'hardware_metrics', 'metrics'], [])),
        cabinetPosition: startU || '',
        startU,
        cabinetUSize,
        placementStatus: placementStatus || (startU > 0 ? 'placed' : 'unplaced'),
        layoutStatus: placementStatus || (startU > 0 ? 'placed' : 'unplaced'),
        displayName: String(this.pick(item, ['ip', 'name', 'assetName', 'asset_name'], '未命名服务器'))
      }
    },
    normalizeHardwareIssue(item = {}) {
      const assetId = this.num(this.pick(item, ['assetId', 'asset_id', 'id']), '')
      const ip = String(this.pick(item, ['ip'], '')).trim()
      const matchedServer = this.layoutInfo.servers.find(server => {
        if (assetId && server.id === assetId) return true
        return ip && server.ip === ip
      }) || {}
      const remoteManagementCard = String(
        this.pick(item, ['remoteManagementCard', 'remote_management_card'],
          this.pick(matchedServer, ['remoteManagementCard', 'remote_management_card'], ''))
      ).trim()

      return {
        ...matchedServer,
        ...item,
        id: assetId || this.num(this.pick(matchedServer, ['id', 'assetId', 'asset_id']), ''),
        assetId: assetId || this.num(this.pick(matchedServer, ['id', 'assetId', 'asset_id']), ''),
        ip: ip || String(this.pick(matchedServer, ['ip'], '')).trim(),
        remoteManagementCard,
        brand: String(this.pick(item, ['brand'], this.pick(matchedServer, ['brand'], ''))),
        model: String(this.pick(item, ['model'], this.pick(matchedServer, ['model'], ''))),
        purpose: String(this.pick(item, ['purpose'], this.pick(matchedServer, ['purpose'], ''))),
        metrics: this.list(this.pick(item, ['metrics', 'hardwareMetrics', 'hardware_metrics'], this.pick(matchedServer, ['hardwareMetrics', 'hardware_metrics'], [])))
      }
    },
    canOpenHardwareMonitor(issue) {
      return !!(issue?.remoteManagementCard || issue?.assetId || issue?.id)
    },
    openHardwareMonitor(issue) {
      if (!this.canOpenHardwareMonitor(issue)) {
        this.$message.warning('当前物理机缺少远程管理卡或资产ID，无法查看硬件监控')
        return
      }
      this.$emit('open-hardware-monitor', {
        id: issue.assetId || issue.id || '',
        assetId: issue.assetId || issue.id || '',
        remoteManagementCard: issue.remoteManagementCard || '',
        ip: issue.ip || '',
        brand: issue.brand || '',
        model: issue.model || '',
        purpose: issue.purpose || ''
      })
    },
    resolveLayoutStatus(servers, totalU) {
      const next = servers.map(item => ({ ...item }))
      const occupiedMap = {}

      next.forEach(item => {
        const explicitStatus = String(item.layoutStatus || item.placementStatus || '').toLowerCase()
        if (explicitStatus === 'unplaced') {
          item.layoutStatus = 'unplaced'
          return
        }
        if (explicitStatus === 'overlap' || explicitStatus === 'out_of_range') {
          item.layoutStatus = explicitStatus
          return
        }

        const endU = item.startU + item.cabinetUSize - 1
        if (item.startU < 1 || endU > totalU) {
          item.layoutStatus = 'out_of_range'
          return
        }

        item.layoutStatus = 'placed'
        for (let u = item.startU; u <= endU; u += 1) {
          if (!occupiedMap[u]) occupiedMap[u] = []
          occupiedMap[u].push(item.id)
        }
      })

      const overlapIds = new Set()
      Object.keys(occupiedMap).forEach(key => {
        if (occupiedMap[key].length > 1) {
          occupiedMap[key].forEach(id => overlapIds.add(id))
        }
      })

      next.forEach(item => {
        if (item.layoutStatus === 'placed' && overlapIds.has(item.id)) {
          item.layoutStatus = 'overlap'
        }
      })

      return next
    },
    normalizeLayout(data = {}) {
      const totalU = Math.max(1, this.num(this.pick(data, ['totalU', 'total_u', 'cabinetTotalU', 'cabinet_total_u']), DEFAULT_TOTAL_U))
      const sourceList = this.list(this.pick(data, ['servers', 'serverList', 'assets', 'assetList', 'items', 'list'], []))
      const normalized = sourceList.map(item => this.normalizeLayoutServer(item))
      const servers = this.resolveLayoutStatus(normalized, totalU)
      const occupied = new Set()

      servers.forEach(item => {
        if (item.layoutStatus === 'unplaced') return
        const start = Math.max(1, item.startU)
        const end = Math.min(totalU, item.startU + item.cabinetUSize - 1)
        for (let u = start; u <= end; u += 1) occupied.add(u)
      })

      const usedU = this.num(this.pick(data, ['usedU', 'used_u']), occupied.size)
      const freeU = this.num(this.pick(data, ['freeU', 'free_u']), Math.max(0, totalU - usedU))
      const usageRate = this.parseRate(this.pick(data, ['usageRate', 'usage_rate']), totalU ? (usedU / totalU) * 100 : 0)
      const idleRate = this.parseRate(this.pick(data, ['idleRate', 'idle_rate']), totalU ? (freeU / totalU) * 100 : 0)
      const isFull = Boolean(this.pick(data, ['isFull', 'is_full'], freeU <= 0))
      const fallbackSummary = makeEmptyLayout().hardwareSummary

      return {
        stats: {
          totalU,
          usedU,
          freeU,
          usageRate,
          idleRate,
          isFull
        },
        hardwareStatus: String(this.pick(data, ['hardwareStatus', 'hardware_status'], 'normal')).toLowerCase(),
        hardwareSummary: this.pick(data, ['hardwareSummary', 'hardware_summary'], fallbackSummary) || fallbackSummary,
        servers
      }
    },
    normalizeAssetDetail(item = {}) {
      return {
        id: this.num(this.pick(item, ['id']), 0),
        ip: String(this.pick(item, ['ip'], '')),
        brand: String(this.pick(item, ['brand'], '')),
        model: String(this.pick(item, ['model'], '')),
        cpu: String(this.pick(item, ['cpu'], '')),
        memory: String(this.pick(item, ['memory'], '')),
        disk: String(this.pick(item, ['disk'], '')),
        purpose: String(this.pick(item, ['purpose'], '')),
        cabinetPosition: this.num(this.pick(item, ['cabinetPosition', 'cabinet_position']), ''),
        cabinetUSize: this.num(this.pick(item, ['cabinetUSize', 'cabinet_u_size']), '')
      }
    },
    formatHardwareMetrics(metrics = []) {
      const list = this.list(metrics)
      if (!list.length) return '硬件状态异常'
      return list.slice(0, 4).join(' / ')
    },
    resetPanel() {
      this.layoutInfo = makeEmptyLayout()
      this.selectedServerId = null
      this.selectedServerDetail = null
      this.positionForm = makePositionForm()
      this.clearDragState()
    },
    reloadLayout() {
      this.tryLoadLayout(true)
    },
    async tryLoadLayout(force = false) {
      if (!this.hasCabinetSelection) {
        this.resetPanel()
        return
      }
      if (!force && this.cabinetSelectionKey === this.lastLoadedKey) return
      await this.loadLayout()
      this.lastLoadedKey = this.cabinetSelectionKey
    },
    async loadLayout() {
      const params = this.buildLayoutQuery()
      if (!Object.keys(params).length) {
        this.resetPanel()
        return
      }

      this.clearDragState()
      this.loadingLayout = true
      try {
        const { data: res } = await this.$api.getCmdbIdcCabinetLayout(params)
        if (res.code !== 200) throw new Error(res.message || '获取机柜布局失败')
        this.layoutInfo = this.normalizeLayout(res.data || {})

        const selected = this.layoutInfo.servers.find(item => item.id === this.selectedServerId)
        if (selected) {
          await this.loadServerDetail(selected.id, selected)
        } else {
          this.selectedServerId = null
          this.selectedServerDetail = null
          this.positionForm = makePositionForm()
        }
      } catch (error) {
        console.error('loadLayout failed:', error)
        this.resetPanel()
        this.$message.error(error.message || '获取机柜布局失败')
      } finally {
        this.loadingLayout = false
      }
    },
    async selectServer(server) {
      if (!server || !server.id) return
      this.selectedServerId = server.id
      await this.loadServerDetail(server.id, server)
    },
    async loadServerDetail(assetId, fallback = null) {
      this.loadingDetail = true
      try {
        const { data: res } = await this.$api.getCmdbIdcAssetInfo(assetId)
        if (res.code !== 200) throw new Error(res.message || '获取服务器详情失败')
        const detail = this.normalizeAssetDetail(res.data || {})
        this.selectedServerDetail = detail
        const isDraggingCurrent = this.isDraggingServer(assetId)
        this.positionForm = {
          cabinetPosition: isDraggingCurrent
            ? this.dragState.currentU
            : this.num(detail.cabinetPosition || fallback?.cabinetPosition || fallback?.startU, 1),
          cabinetUSize: isDraggingCurrent
            ? this.dragState.cabinetUSize
            : Math.max(1, this.num(detail.cabinetUSize || fallback?.cabinetUSize, 1))
        }
      } catch (error) {
        console.error('loadServerDetail failed:', error)
        this.selectedServerDetail = fallback ? this.normalizeAssetDetail(fallback) : null
        this.$message.error(error.message || '获取服务器详情失败')
      } finally {
        this.loadingDetail = false
      }
    },
    async submitPosition(options = {}) {
      const { silentSuccess = false } = options
      if (!this.selectedServerDetail?.id) {
        this.$message.warning('请先选择服务器')
        return
      }

      const cabinetId = this.num(this.pick(this.selectedCabinet, ['id', 'cabinetId', 'cabinet_id']), '')
      if (!cabinetId) {
        this.$message.warning('当前机柜信息不完整，无法调整位置')
        return
      }

      const cabinetPosition = this.num(this.positionForm.cabinetPosition, 0)
      const cabinetUSize = this.num(this.positionForm.cabinetUSize, 0)
      if (cabinetPosition < 1 || cabinetUSize < 1) {
        this.$message.warning('位置和U高度必须大于0')
        return
      }

      this.savingPosition = true
      try {
        const payload = {
          id: this.selectedServerDetail.id,
          cabinetId,
          cabinetPosition,
          cabinetUSize
        }
        const { data: res } = await this.$api.updateCmdbIdcAssetPosition(payload)
        if (res.code !== 200) throw new Error(res.message || '更新服务器位置失败')
        if (!silentSuccess) this.$message.success('服务器位置更新成功')
        await this.loadLayout()
        await this.loadServerDetail(payload.id)
      } catch (error) {
        console.error('submitPosition failed:', error)
        this.$message.error(error.message || '更新服务器位置失败')
      } finally {
        this.savingPosition = false
      }
    }
  }
}
</script>

<style scoped>
.idc-cabinet-panel {
  min-width: 0;
  height: calc(100vh - 140px);
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.toolbar,
.panel-card {
  border-radius: 18px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.95), rgba(247, 250, 255, 0.92));
  box-shadow: 0 16px 36px rgba(15, 23, 42, 0.08);
}

.toolbar {
  margin-bottom: 8px;
  padding: 8px 12px 6px;
  flex: 0 0 auto;
}

.toolbar-main {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.toolbar-tags {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  color: #5f6b7c;
  font-size: 13px;
}

.hint-text {
  color: #8d97a8;
}

.toolbar-divider {
  height: 1px;
  margin: 6px -12px 0;
  background: #e5e7eb;
}

.view-tabs {
  display: flex;
  align-items: center;
  gap: 20px;
  padding-top: 6px;
}

.view-tab {
  padding: 0 0 5px;
  border: none;
  background: transparent;
  color: #6b7280;
  font-size: 15px;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  transition: color 0.2s ease, border-color 0.2s ease;
}

.view-tab:hover {
  color: #2563eb;
}

.view-tab.active {
  color: #2563eb;
  border-bottom-color: #3b82f6;
  font-weight: 600;
}

.panel-card {
  padding: 10px;
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 6px;
  margin-bottom: 8px;
  flex: 0 0 auto;
}

.stat-card {
  min-height: 36px;
  border-radius: 10px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  background: rgba(255, 255, 255, 0.78);
  padding: 6px 10px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.stat-card span {
  display: inline-flex;
  font-size: 11px;
  color: #7a8798;
  margin-bottom: 0;
  white-space: nowrap;
}

.stat-card strong {
  color: #172133;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.1;
  white-space: nowrap;
}

.cabinet-alert-summary {
  flex: 0 0 auto;
  margin: 0 0 8px;
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid rgba(245, 158, 11, 0.36);
  background: linear-gradient(135deg, rgba(255, 251, 235, 0.96), rgba(255, 247, 237, 0.9));
  color: #7c2d12;
}

.cabinet-alert-summary.critical {
  border-color: rgba(239, 68, 68, 0.36);
  background: linear-gradient(135deg, rgba(254, 242, 242, 0.96), rgba(255, 247, 237, 0.9));
  color: #7f1d1d;
}

.cabinet-alert-summary.unknown {
  border-color: rgba(148, 163, 184, 0.38);
  background: rgba(248, 250, 252, 0.92);
  color: #334155;
}

.cabinet-alert-head,
.cabinet-alert-tags,
.cabinet-alert-issue {
  display: flex;
  align-items: center;
  gap: 8px;
}

.cabinet-alert-head {
  justify-content: space-between;
}

.cabinet-alert-head strong {
  color: inherit;
  font-size: 13px;
}

.cabinet-alert-summary p {
  margin: 6px 0 0;
  color: inherit;
  font-size: 12px;
  line-height: 1.5;
  opacity: 0.88;
}

.cabinet-alert-issues {
  display: grid;
  gap: 6px;
  margin-top: 8px;
}

.cabinet-alert-issue {
  justify-content: space-between;
  padding: 6px 8px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.58);
  font-size: 12px;
}

.cabinet-alert-ip {
  min-width: 0;
  overflow: hidden;
  padding: 0;
  border: none;
  background: transparent;
  color: #172133;
  font: inherit;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: pointer;
}

.cabinet-alert-ip:hover:not(:disabled) {
  color: #2563eb;
  text-decoration: underline;
}

.cabinet-alert-ip:disabled {
  cursor: not-allowed;
  opacity: 0.7;
}

.cabinet-alert-issue em {
  flex-shrink: 0;
  max-width: 60%;
  overflow: hidden;
  color: inherit;
  font-style: normal;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.panel-main {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 380px;
  gap: 10px;
  align-items: stretch;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}

.section-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 6px;
}

.section-head h3 {
  margin: 0;
  font-size: 16px;
  color: #172133;
}

.section-head p {
  margin: 2px 0 0;
  font-size: 10px;
  color: #8d97a8;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}

.rack-section,
.detail-card,
.unplaced-card {
  border-radius: 14px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  background: rgba(255, 255, 255, 0.8);
}

.rack-section {
  box-sizing: border-box;
  padding: 10px 12px;
  border-color: rgba(186, 200, 220, 0.95);
  background: linear-gradient(145deg, rgba(239, 245, 253, 0.98), rgba(228, 237, 249, 0.94));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.75), 0 10px 26px rgba(20, 30, 48, 0.08);
  min-height: 0;
  overflow: hidden;
}

.rack-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 6px;
}

.legend-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: #5e6f86;
  font-size: 11px;
  font-weight: 500;
}

.legend-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  display: inline-block;
}

.legend-dot.placed {
  background: #2d86f4;
}

.legend-dot.critical {
  background: #ef4444;
}

.legend-dot.overlap {
  background: #e25562;
}

.legend-dot.out-of-range {
  background: #e98d2f;
}

.legend-dot.unplaced {
  background: #8c97a8;
}

.rack-wrapper {
  display: flex;
  justify-content: center;
  align-items: flex-start;
  position: relative;
  padding: 14px 64px 6px 64px;
  overflow: hidden;
  min-height: 0;
}

.cabinet-iso {
  position: relative;
  flex: 0 0 auto;
  width: 35%;
  min-width: 280px;
  margin: 0 auto;
  transform: translateX(-54px);
}

.cabinet-iso::after {
  content: '';
  position: absolute;
  left: 19px;
  right: -31px;
  bottom: -8px;
  height: 19px;
  border-radius: 999px;
  background: radial-gradient(ellipse at center, rgba(15, 23, 35, 0.28) 0%, rgba(15, 23, 35, 0.06) 68%, transparent 100%);
  filter: blur(2px);
  pointer-events: none;
  z-index: 0;
}

.cabinet-top-face {
  position: absolute;
  left: 16px;
  right: -35px;
  top: -20px;
  height: 25px;
  border: 1px solid #8798b0;
  border-bottom: none;
  border-radius: 10px 18px 0 0;
  background: linear-gradient(180deg, #7a8ba3 0%, #65758c 55%, #5a6a81 100%);
  transform: skewX(-64deg);
  transform-origin: left bottom;
  box-shadow: inset 0 1px 0 rgba(243, 247, 253, 0.38), inset 0 -1px 0 rgba(39, 52, 69, 0.34);
  pointer-events: none;
  z-index: 1;
}

.cabinet-top-face::after {
  content: '';
  position: absolute;
  inset: 5px 12px 0;
  border-top: 1px solid rgba(223, 234, 249, 0.35);
  border-radius: 8px 14px 0 0;
  opacity: 0.8;
}

.cabinet-side-face {
  position: absolute;
  top: 12px;
  right: -81px;
  bottom: 16px;
  width: 85px;
  border: 1px solid #6f8098;
  border-left: none;
  border-radius: 0 16px 16px 0;
  background: linear-gradient(180deg, #44546a 0%, #334156 44%, #2b374a 100%);
  transform: skewY(-30deg);
  transform-origin: left top;
  box-shadow: inset 0 1px 0 rgba(194, 210, 231, 0.24);
  pointer-events: none;
  z-index: 1;
}

.cabinet-side-face::before {
  content: '';
  position: absolute;
  top: 9px;
  bottom: 9px;
  left: 23px;
  right: 19px;
  border-radius: 6px;
  background: repeating-linear-gradient(180deg, rgba(166, 184, 209, 0.34) 0 2px, rgba(44, 58, 76, 0.1) 2px 9px);
  opacity: 0.85;
}

.cabinet-side-face::after {
  content: '';
  position: absolute;
  left: -12px;
  top: -7px;
  width: 16px;
  height: 13px;
  background: linear-gradient(180deg, #6f8199 0%, #596b84 100%);
  border-top: 1px solid rgba(131, 150, 174, 0.8);
  border-left: 1px solid rgba(131, 150, 174, 0.8);
  clip-path: polygon(0 100%, 100% 0, 100% 100%);
  pointer-events: none;
}

.cabinet-shell {
  position: relative;
  z-index: 2;
  padding: 7px;
  border-radius: 12px;
  border: 1px solid #8797ad;
  background: linear-gradient(180deg, #56647a 0%, #46556b 12%, #324055 50%, #2a3648 100%);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.34), inset 0 -1px 0 rgba(15, 24, 36, 0.42), 0 18px 32px rgba(20, 31, 48, 0.22);
  overflow: hidden;
}

.cabinet-shell::before {
  content: '';
  position: absolute;
  left: 7px;
  right: 7px;
  top: 4px;
  height: 28px;
  border-radius: 8px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.24), rgba(255, 255, 255, 0.02));
  pointer-events: none;
}

.cabinet-shell::after {
  content: '';
  position: absolute;
  inset: 5px;
  border-radius: 9px;
  border: 1px solid rgba(162, 179, 203, 0.38);
  pointer-events: none;
}

.cabinet-head {
  position: relative;
  z-index: 2;
  height: 23px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 5px;
  padding: 0 7px;
  border-radius: 7px;
  background: linear-gradient(180deg, #6f7f95 0%, #5b6b82 46%, #4f5f74 100%);
  border: 1px solid rgba(173, 189, 211, 0.62);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.34), 0 6px 12px rgba(17, 26, 40, 0.22);
  margin-bottom: 7px;
}

.cabinet-title {
  color: #ecf2ff;
  font-size: 11px;
  font-weight: 600;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cabinet-subtitle {
  color: #d2dceb;
  font-size: 9px;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  text-shadow: 0 1px 0 rgba(16, 26, 38, 0.34);
}

.cabinet-core {
  position: relative;
  z-index: 2;
  display: grid;
  grid-template-columns: 23px minmax(0, 1fr) 23px;
  gap: 5px;
  align-items: center;
}

.rack-scale {
  position: relative;
}

.rack-scale-label {
  position: absolute;
  transform: translateY(50%);
  font-size: 8px;
  color: #d9e4f3;
  font-weight: 600;
  line-height: 1;
  text-shadow: 0 1px 0 rgba(18, 26, 39, 0.4);
}

.rack-scale-left .rack-scale-label {
  right: 0;
}

.rack-scale-right .rack-scale-label {
  left: 0;
}

.cabinet-door {
  position: relative;
  border-radius: 9px;
  border: 1px solid #72829a;
  background: linear-gradient(180deg, #243044 0%, #1c2737 44%, #172130 100%);
  padding: 9px 16px;
  box-shadow: inset 0 0 0 1px rgba(180, 197, 219, 0.2), inset 0 16px 26px rgba(255, 255, 255, 0.04), inset 0 -12px 24px rgba(9, 14, 22, 0.36), 0 10px 18px rgba(8, 14, 22, 0.28);
}

.cabinet-door::before {
  content: '';
  position: absolute;
  inset: 4px;
  border-radius: 6px;
  border: 1px solid rgba(156, 171, 191, 0.24);
  pointer-events: none;
}

.cabinet-door::after {
  content: '';
  position: absolute;
  inset: 5px 16px;
  border-radius: 5px;
  background: linear-gradient(90deg, rgba(123, 146, 176, 0.12), rgba(255, 255, 255, 0.02), rgba(123, 146, 176, 0.12));
  pointer-events: none;
}

.cabinet-side {
  position: absolute;
  top: 7px;
  bottom: 7px;
  width: 5px;
  border-radius: 3px;
  background: linear-gradient(180deg, #8396b0 0%, #62758f 100%);
  box-shadow: inset 0 0 0 1px rgba(193, 209, 231, 0.38), inset -1px 0 0 rgba(24, 35, 49, 0.4);
  z-index: 2;
}

.cabinet-side.left {
  left: 4px;
}

.cabinet-side.right {
  right: 4px;
}

.cabinet-rail {
  position: absolute;
  top: 9px;
  bottom: 9px;
  width: 7px;
  border-radius: 4px;
  background: linear-gradient(180deg, #162130 0%, #0f1825 100%);
  border: 1px solid rgba(125, 146, 174, 0.55);
  z-index: 3;
  box-shadow: inset 0 1px 0 rgba(173, 193, 218, 0.25);
}

.cabinet-rail::after {
  content: '';
  position: absolute;
  top: 3px;
  bottom: 3px;
  left: 1px;
  right: 1px;
  border-radius: 3px;
  background: repeating-linear-gradient(180deg, rgba(146, 165, 189, 0.62) 0 2px, rgba(17, 25, 36, 0.2) 2px 9px);
}

.cabinet-rail.rail-left {
  left: 11px;
}

.cabinet-rail.rail-right {
  right: 11px;
}

.cabinet-handle {
  position: absolute;
  right: 5px;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 56px;
  border-radius: 3px;
  background: linear-gradient(180deg, #9eb0c9 0%, #72829a 100%);
  z-index: 3;
  box-shadow: inset 0 1px 0 rgba(232, 239, 249, 0.55), 0 0 0 1px rgba(34, 47, 63, 0.36);
}

.rack-body {
  position: relative;
  z-index: 4;
  border-radius: 6px;
  border: 1px solid rgba(140, 162, 193, 0.52);
  background: linear-gradient(180deg, #0f1928 0%, #0c1522 100%);
  box-shadow: inset 0 1px 0 rgba(177, 198, 228, 0.16), inset 0 -1px 0 rgba(3, 8, 15, 0.5);
  overflow: hidden;
}

.rack-body::before {
  content: '';
  position: absolute;
  inset: 0;
  background-image: radial-gradient(circle, rgba(178, 197, 224, 0.26) 1px, transparent 1px);
  background-size: 5px 5px;
  opacity: 0.2;
  pointer-events: none;
}

.rack-body::after {
  content: '';
  position: absolute;
  inset: 0;
  background-image: repeating-linear-gradient(180deg, rgba(111, 140, 177, 0.08) 0 1px, transparent 1px 8px);
  opacity: 0.6;
  pointer-events: none;
}

.u-line {
  position: absolute;
  left: 0;
  right: 0;
  height: 1px;
  background: rgba(129, 154, 191, 0.2);
}

.server-block {
  position: absolute;
  left: 6px;
  right: 6px;
  border: 1px solid rgba(208, 227, 255, 0.26);
  border-radius: 5px;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  gap: 1px;
  padding: 3px 7px 3px 11px;
  color: #f8fbff;
  text-align: left;
  cursor: grab;
  user-select: none;
  touch-action: none;
  overflow: hidden;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.24), inset 0 -1px 0 rgba(3, 18, 41, 0.34);
  transition: box-shadow 0.18s ease, transform 0.18s ease, filter 0.18s ease;
}

.server-block::before {
  content: '';
  position: absolute;
  left: 4px;
  top: 50%;
  transform: translateY(-50%);
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: #9af7bf;
  box-shadow: 0 0 8px rgba(154, 247, 191, 0.75);
}

.server-block::after {
  content: '';
  position: absolute;
  inset: 0;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.16), rgba(255, 255, 255, 0.02) 32%, transparent 100%);
  pointer-events: none;
}

.server-block:hover {
  transform: translateY(-1px);
  filter: brightness(1.03);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.28), 0 8px 14px rgba(7, 16, 30, 0.35);
}

.server-block.selected {
  border-color: rgba(221, 236, 255, 0.86);
  box-shadow: 0 0 0 2px rgba(228, 240, 255, 0.96), 0 0 0 4px rgba(90, 154, 255, 0.42), 0 10px 18px rgba(8, 16, 28, 0.34);
}

.server-block.is-dragging {
  cursor: grabbing;
  transform: scale(1.01);
  filter: brightness(1.06);
}

.server-name {
  width: 100%;
  font-size: 9px;
  font-weight: 600;
  line-height: 1.1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.server-meta {
  font-size: 8px;
  line-height: 1.1;
  opacity: 0.93;
  letter-spacing: 0.02em;
}

.status-placed {
  background: linear-gradient(90deg, #3388ee 0%, #2f79db 52%, #2c70cd 100%);
}

.status-critical {
  border-color: rgba(255, 205, 210, 0.72);
  background: linear-gradient(90deg, #ef4444 0%, #dc2626 52%, #b91c1c 100%);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.26), inset 0 -1px 0 rgba(69, 10, 10, 0.38), 0 0 10px rgba(239, 68, 68, 0.32);
}

.status-critical::before {
  background: #fecaca;
  box-shadow: 0 0 8px rgba(254, 202, 202, 0.92);
}

.status-overlap {
  background: linear-gradient(90deg, #e45f6d 0%, #d54f5d 100%);
}

.status-overlap::before {
  background: #ffd4da;
  box-shadow: 0 0 8px rgba(255, 212, 218, 0.82);
}

.status-out-of-range {
  background: linear-gradient(90deg, #e99a43 0%, #dc8628 100%);
}

.status-out-of-range::before {
  background: #ffe0ba;
  box-shadow: 0 0 8px rgba(255, 224, 186, 0.82);
}

.status-unplaced {
  background: linear-gradient(90deg, #8190a5 0%, #6b7a90 100%);
}

.status-unplaced::before {
  background: #d7dee8;
  box-shadow: 0 0 6px rgba(215, 222, 232, 0.7);
}

.cabinet-foot {
  display: flex;
  justify-content: space-between;
  padding: 0 13px;
  margin-top: 6px;
  position: relative;
  z-index: 2;
}

.cabinet-foot span {
  width: 21px;
  height: 3px;
  border-radius: 3px;
  background: linear-gradient(180deg, #8ea0ba 0%, #70839e 100%);
  box-shadow: inset 0 1px 0 rgba(234, 241, 250, 0.45), 0 2px 3px rgba(20, 31, 45, 0.3);
}

.detail-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  min-height: 0;
  align-self: stretch;
  overflow: hidden;
}

.detail-card {
  padding: 10px;
  flex: 0 0 auto;
}

.position-form {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px dashed #dbe3ef;
}

.drag-save-config {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
  color: #64748b;
  font-size: 11px;
}

.position-form h4 {
  margin: 0 0 6px;
  font-size: 13px;
  color: #1f2937;
}

.sub-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}

.sub-head h4 {
  margin: 0;
  font-size: 13px;
  color: #1f2937;
}

.sub-head span {
  color: #64748b;
  font-size: 12px;
}

.unplaced-card {
  padding: 10px;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}

:deep(.position-form .el-form-item) {
  margin-right: 6px;
  margin-bottom: 6px;
}

:deep(.detail-card .el-descriptions__body) {
  background: rgba(255, 255, 255, 0.72);
}

:deep(.detail-card .el-descriptions__cell) {
  padding: 5px 7px !important;
  font-size: 11px;
  line-height: 1.25;
}

:deep(.detail-card .el-descriptions__label) {
  width: 54px;
  min-width: 54px;
  color: #4b5563;
  font-weight: 700;
}

:deep(.detail-card .el-descriptions__content) {
  min-width: 0;
  word-break: break-all;
}

:deep(.position-form .el-input-number) {
  width: 104px;
}

:deep(.position-form .el-input-number .el-input__wrapper) {
  min-height: 28px;
  border-radius: 12px;
}

:deep(.position-form .el-button) {
  min-height: 28px;
  padding: 5px 10px;
}

:deep(.unplaced-card .el-table) {
  border-radius: 10px;
}

:deep(.unplaced-card .el-table .cell) {
  line-height: 18px;
  font-size: 11px;
}

@media (max-width: 1400px) {
  .stats-grid {
    grid-template-columns: repeat(6, minmax(0, 1fr));
  }

  .panel-main {
    grid-template-columns: minmax(0, 1fr) 360px;
  }
}

@media (max-width: 860px) {
  .stats-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .rack-wrapper {
    padding: 22px 36px 10px 36px;
  }

  .cabinet-iso {
    width: 100%;
    min-width: 0;
    transform: none;
  }

  .cabinet-top-face {
    left: 12px;
    right: -18px;
    top: -14px;
    height: 18px;
    transform: skewX(-52deg);
  }

  .cabinet-side-face {
    right: -42px;
    width: 50px;
    top: 8px;
    bottom: 12px;
    transform: skewY(-26deg);
  }

  .cabinet-core {
    grid-template-columns: 24px minmax(0, 1fr) 24px;
    gap: 6px;
  }

  .view-tabs {
    gap: 16px;
    overflow-x: auto;
    scrollbar-width: none;
  }

  .view-tabs::-webkit-scrollbar {
    display: none;
  }
}
</style>
