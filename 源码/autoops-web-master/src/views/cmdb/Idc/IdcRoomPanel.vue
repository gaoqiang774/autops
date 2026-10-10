<template>
  <div class="idc-room-panel">
    <div class="toolbar">
      <div class="toolbar-main">
        <div class="toolbar-tags">
          <span>当前机房</span>
          <el-tag v-if="selectedRoomName" type="primary" effect="light">{{ selectedRoomName }}</el-tag>
          <el-tag v-if="layoutInfo.cabinetCount" type="success" effect="light">{{ layoutInfo.cabinetCount }} 个机柜</el-tag>
          <el-tag v-if="layoutInfo.assetCount" type="info" effect="light">{{ layoutInfo.assetCount }} 台资产</el-tag>
          <span v-if="!hasRoomSelection" class="hint-text">请先从左侧选择机房</span>
        </div>

        <div class="toolbar-actions">
          <el-button-group>
            <el-button size="small" :icon="ZoomOut" :disabled="!hasRoomSelection" @click="zoomOut" />
            <el-button size="small" :disabled="!hasRoomSelection" @click="resetView">{{ Math.round(scale * 100) }}%</el-button>
            <el-button size="small" :icon="ZoomIn" :disabled="!hasRoomSelection" @click="zoomIn" />
          </el-button-group>
          <el-button size="small" type="primary" :loading="loadingLayout" :disabled="!hasRoomSelection" @click="reloadLayout">
            <el-icon><RefreshRight /></el-icon>刷新视图
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

    <div class="room-board" v-loading="loadingLayout">
      <el-empty v-if="!hasRoomSelection" description="点击左侧机房后查看 2.5D 全局视角" />

      <template v-else>
        <section
          ref="viewport"
          class="room-viewport"
          @wheel.prevent="handleWheel"
          @pointerdown="handleViewportPointerDown"
          @pointermove="handleViewportPointerMove"
          @pointerup="handleViewportPointerUp"
          @pointercancel="handleViewportPointerUp"
          @dblclick.self="resetView"
        >
          <div class="scene-header">
            <div>
              <h3>{{ layoutInfo.roomName || selectedRoomName }}</h3>
              <p>IDC Room Isometric View</p>
            </div>
            <div class="scene-badges">
              <span class="status-pill normal">{{ normalCount }} 正常</span>
              <span class="status-pill critical">{{ criticalCount }} 严重</span>
            </div>
          </div>

          <div
            class="room-canvas"
            :style="canvasTransformStyle"
            @pointerdown.stop="handleCanvasPointerDown"
            @pointermove.stop="handleViewportPointerMove"
            @pointerup.stop="handleViewportPointerUp"
            @pointercancel.stop="handleViewportPointerUp"
          >
            <svg class="iso-floor" viewBox="0 0 1200 760" preserveAspectRatio="xMidYMid slice" aria-hidden="true">
              <defs>
                <linearGradient id="room-back-wall" x1="0" x2="1" y1="0" y2="1">
                  <stop offset="0%" stop-color="#eef2f7" />
                  <stop offset="58%" stop-color="#b9c2cf" />
                  <stop offset="100%" stop-color="#7f8a99" />
                </linearGradient>
                <linearGradient id="room-left-wall" x1="0" x2="1" y1="0" y2="1">
                  <stop offset="0%" stop-color="#cfd6df" />
                  <stop offset="100%" stop-color="#8b96a6" />
                </linearGradient>
                <linearGradient id="room-floor-base" x1="0" x2="1" y1="0" y2="1">
                  <stop offset="0%" stop-color="#1b2230" />
                  <stop offset="48%" stop-color="#121a29" />
                  <stop offset="100%" stop-color="#080d18" />
                </linearGradient>
                <radialGradient id="floor-vignette" cx="54%" cy="46%" r="68%">
                  <stop offset="0%" stop-color="rgba(255,255,255,0.05)" />
                  <stop offset="58%" stop-color="rgba(255,255,255,0)" />
                  <stop offset="100%" stop-color="rgba(0,0,0,0.5)" />
                </radialGradient>
                <pattern id="wall-panel" width="90" height="58" patternUnits="userSpaceOnUse">
                  <path d="M0 58 H90 M90 0 V58" fill="none" stroke="rgba(31, 41, 55, 0.16)" stroke-width="1" />
                </pattern>
                <filter id="floor-soft-shadow" x="-20%" y="-20%" width="140%" height="150%">
                  <feGaussianBlur stdDeviation="10" />
                </filter>
              </defs>
              <rect width="1200" height="760" fill="#050816" />
              <polygon :points="roomBackWallPoints()" fill="url(#room-back-wall)" />
              <polygon :points="roomLeftWallPoints()" fill="url(#room-left-wall)" />
              <polygon :points="roomFloorShadowPoints()" fill="rgba(0, 0, 0, 0.36)" filter="url(#floor-soft-shadow)" />
              <polygon :points="roomFloorPoints()" fill="url(#room-floor-base)" />
              <g class="room-floor-grid">
                <polyline
                  v-for="(line, index) in roomFloorGridLines()"
                  :key="`floor-grid-${index}`"
                  :points="line"
                />
              </g>
              <polygon :points="roomFloorPoints()" fill="url(#floor-vignette)" />
              <polygon :points="roomBackWallPoints()" fill="url(#wall-panel)" opacity="0.42" />
              <polygon :points="roomLeftWallPoints()" fill="url(#wall-panel)" opacity="0.36" />
              <polyline :points="roomWallOutlinePoints()" fill="none" stroke="rgba(226, 232, 240, 0.56)" stroke-width="1.4" />
              <polyline :points="roomFloorOutlinePoints()" fill="none" stroke="rgba(120, 147, 183, 0.22)" stroke-width="1.2" />
              <path :d="roomBackEdgePath()" stroke="rgba(219, 234, 254, 0.18)" stroke-width="3" />
            </svg>

            <svg class="rack-bank-layer" viewBox="0 0 1200 760" preserveAspectRatio="xMidYMid slice">
              <defs>
                <linearGradient id="rack-top-filled" x1="0" x2="1" y1="0" y2="1">
                  <stop offset="0%" stop-color="#778391" />
                  <stop offset="100%" stop-color="#3f4856" />
                </linearGradient>
                <linearGradient id="rack-face-filled" x1="0" x2="0" y1="0" y2="1">
                  <stop offset="0%" stop-color="#3d4652" />
                  <stop offset="100%" stop-color="#151b25" />
                </linearGradient>
                <linearGradient id="rack-face-empty" x1="0" x2="0" y1="0" y2="1">
                  <stop offset="0%" stop-color="#3f4650" />
                  <stop offset="100%" stop-color="#20252d" />
                </linearGradient>
                <linearGradient id="rack-top-empty" x1="0" x2="1" y1="0" y2="1">
                  <stop offset="0%" stop-color="#697383" />
                  <stop offset="100%" stop-color="#363d49" />
                </linearGradient>
                <filter id="alarm-glow" x="-80%" y="-80%" width="260%" height="260%">
                  <feGaussianBlur stdDeviation="3" result="blur" />
                  <feMerge>
                    <feMergeNode in="blur" />
                    <feMergeNode in="SourceGraphic" />
                  </feMerge>
                </filter>
              </defs>

              <g
                v-for="row in renderRows"
                :key="`row-bank-${row.rowNo}-${row.index}`"
                class="rack-bank"
              >
                <polygon class="rack-bank-shadow" :points="rowBankShadowPoints(row)" />

                <polygon
                  v-for="cabinet in row.cabinets"
                  :key="`top-${getCabinetKey(cabinet)}`"
                  class="rack-top-cell"
                  :class="{ filled: hasCabinetData(cabinet), empty: !hasCabinetData(cabinet), placeholder: cabinet.placeholder }"
                  :points="cabinetTopPoints(row, cabinet.visualIndex)"
                />

                <polygon class="rack-end-face" :points="rowBankEndPoints(row)" />

                <g
                  v-for="cabinet in row.cabinets"
                  :key="getCabinetKey(cabinet)"
                  class="rack-cabinet-cell"
                  :class="[
                    cabinetStatusClass(cabinet),
                    {
                      selected: selectedCabinetKey === getCabinetKey(cabinet),
                      filled: hasCabinetData(cabinet),
                      empty: !hasCabinetData(cabinet),
                      placeholder: cabinet.placeholder
                    }
                  ]"
                  :aria-label="`${cabinet.name} - ${formatRate(cabinet.usageRate)}`"
                  role="button"
                  @pointerdown.stop
                  @click.stop="handleCabinetCellClick(cabinet)"
                  @dblclick.stop="handleCabinetCellDblClick(cabinet)"
                >
                  <polygon class="rack-cabinet-face" :points="cabinetFacePoints(row, cabinet.visualIndex)" />
                  <polygon class="rack-door-frame" :points="cabinetDoorFramePoints(row, cabinet.visualIndex)" />
                  <polygon
                    v-if="hasCabinetData(cabinet)"
                    class="rack-header-strip"
                    :class="resolveStatus(cabinet)"
                    :points="cabinetHeaderStripPoints(row, cabinet.visualIndex)"
                  />
                  <polyline
                    v-for="rail in [0.14, 0.86]"
                    :key="`rail-${rail}`"
                    class="rack-rail-line"
                    :points="cabinetRailLinePoints(row, cabinet.visualIndex, rail)"
                  />
                  <polyline
                    v-for="line in 10"
                    :key="`line-${line}`"
                    class="rack-slot-line"
                    :points="cabinetSlotLinePoints(row, cabinet.visualIndex, line)"
                  />
                  <template v-if="hasCabinetData(cabinet)">
                    <polygon
                      v-for="block in 4"
                      :key="`device-${block}`"
                      class="rack-device-block"
                      :class="resolveStatus(cabinet)"
                      :points="cabinetDeviceBlockPoints(row, cabinet.visualIndex, block)"
                    />
                  </template>
                  <circle
                    v-if="hasCabinetData(cabinet)"
                    class="rack-led"
                    :class="resolveStatus(cabinet)"
                    :cx="cabinetLedPoint(row, cabinet.visualIndex).x"
                    :cy="cabinetLedPoint(row, cabinet.visualIndex).y"
                    r="3.8"
                  />
                  <text
                    class="rack-cabinet-no"
                    :class="{ empty: !hasCabinetData(cabinet) }"
                    :x="cabinetTextPoint(row, cabinet.visualIndex).x"
                    :y="cabinetTextPoint(row, cabinet.visualIndex).y"
                    :transform="cabinetTextTransform(row, cabinet.visualIndex)"
                  >{{ cabinetDisplayNo(cabinet) }}</text>
                  <circle
                    v-if="hasCabinetData(cabinet)"
                    class="rack-top-vent"
                    :cx="cabinetTopVentPoint(row, cabinet.visualIndex).x"
                    :cy="cabinetTopVentPoint(row, cabinet.visualIndex).y"
                    r="4.8"
                  />
                  <g
                    v-if="resolveStatus(cabinet) === 'critical'"
                    class="rack-alarm-badge"
                    :class="resolveStatus(cabinet)"
                    filter="url(#alarm-glow)"
                  >
                    <title>{{ cabinetAlarmTitle(cabinet) }}</title>
                    <polygon :points="cabinetAlarmTrianglePoints(row, cabinet.visualIndex)" />
                    <text
                      :x="cabinetAlarmPoint(row, cabinet.visualIndex).x"
                      :y="cabinetAlarmPoint(row, cabinet.visualIndex).y + 4"
                    >!</text>
                  </g>
                </g>

                <polyline class="rack-bank-outline" :points="rowBankOutlinePoints(row)" />
                <text
                  class="rack-row-label"
                  :x="rowLabelPoint(row).x"
                  :y="rowLabelPoint(row).y"
                  :transform="rowLabelTransform(row)"
                >{{ row.rowLabel || `${row.rowNo || row.index + 1}列` }}</text>
              </g>
            </svg>
          </div>
        </section>

        <aside class="room-side">
          <section class="info-card">
            <div class="card-title">
              <h3>机房信息</h3>
              <span>{{ layoutInfo.layoutMode || '-' }}</span>
            </div>
            <div class="metric-list">
              <div><span>机房名称</span><strong>{{ layoutInfo.roomName || selectedRoomName || '-' }}</strong></div>
              <div><span>机柜数量</span><strong>{{ layoutInfo.cabinetCount || cabinets.length }}</strong></div>
              <div><span>物理机数量</span><strong>{{ layoutInfo.assetCount || 0 }}</strong></div>
              <div><span>未匹配机柜</span><strong>{{ unmatchedCabinets.length }}</strong></div>
            </div>
          </section>

          <section class="info-card">
            <div class="card-title">
              <h3>容量分析</h3>
              <span>{{ highUsageCount }} 高容量</span>
            </div>
            <div class="capacity-ring" :style="{ '--rate': `${averageUsageRate}%` }">
              <div>
                <strong>{{ averageUsageRate.toFixed(1) }}%</strong>
                <span>平均使用率</span>
              </div>
            </div>
            <div class="legend-list">
              <span><i class="legend-dot normal"></i>正常 &lt; 70%</span>
              <span><i class="legend-dot warning"></i>警告 70%-90%</span>
              <span><i class="legend-dot critical"></i>严重 >= 90%</span>
            </div>
          </section>

          <section class="info-card cabinet-detail-card">
            <div class="card-title">
              <h3>选中机柜</h3>
              <el-button v-if="selectedCabinet" text type="primary" :loading="loadingCabinet" @click="loadCabinetDetail">刷新</el-button>
            </div>
            <el-empty v-if="!selectedCabinet" description="点击机柜查看详情" :image-size="72" />
            <template v-else>
              <div class="metric-list compact">
                <div><span>机柜名称</span><strong>{{ selectedCabinet.name }}</strong></div>
                <div><span>U 位</span><strong>{{ selectedCabinet.usedU || 0 }} / {{ selectedCabinet.totalU || 42 }}</strong></div>
                <div><span>使用率</span><strong>{{ formatRate(selectedCabinet.usageRate) }}</strong></div>
                <div><span>资产数量</span><strong>{{ selectedCabinet.assetCount || 0 }}</strong></div>
              </div>
              <div v-loading="loadingCabinet" class="cabinet-assets">
                <el-empty v-if="!cabinetAssets.length" description="暂无 U 位明细" :image-size="64" />
                <div v-for="asset in cabinetAssets" :key="asset.id || asset.ip" class="asset-row">
                  <span>{{ asset.ip || asset.displayName || '-' }}</span>
                  <em>{{ asset.startU || asset.cabinetPosition || '-' }}U / {{ asset.cabinetUSize || 1 }}U</em>
                </div>
              </div>
            </template>
          </section>

          <section v-if="unmatchedCabinets.length" class="info-card unmatched-card">
            <div class="card-title">
              <h3>未匹配机柜</h3>
              <span>{{ unmatchedCabinets.length }}</span>
            </div>
            <p>以下机柜未参与主布局，请检查命名是否符合“01列05柜”。</p>
            <div class="unmatched-list">
              <span v-for="cabinet in unmatchedCabinets" :key="cabinet.cabinetId || cabinet.name">{{ cabinet.name }}</span>
            </div>
          </section>
        </aside>
      </template>
    </div>
  </div>
</template>

<script>
import { RefreshRight, ZoomIn, ZoomOut } from '@element-plus/icons-vue'

const EMPTY_LAYOUT = {
  roomId: '',
  roomName: '',
  cabinetCount: 0,
  assetCount: 0,
  layoutMode: '',
  rows: [],
  unmatchedCabinets: []
}

export default {
  name: 'IdcRoomPanel',
  components: { RefreshRight },
  props: {
    activeView: {
      type: String,
      default: 'room'
    },
    selectedRoom: {
      type: Object,
      default: null
    }
  },
  emits: ['switch-view', 'select-cabinet'],
  data() {
    return {
      ZoomIn,
      ZoomOut,
      loadingLayout: false,
      loadingCabinet: false,
      layoutInfo: { ...EMPTY_LAYOUT },
      selectedCabinet: null,
      cabinetLayout: null,
      scale: 0.78,
      offset: { x: -26, y: 18 },
      dragState: {
        active: false,
        x: 0,
        y: 0,
        startX: 0,
        startY: 0
      }
    }
  },
  computed: {
    hasRoomSelection() {
      return !!this.roomId
    },
    roomId() {
      return this.num(this.pick(this.selectedRoom, ['id', 'roomId', 'room_id']), '')
    },
    selectedRoomName() {
      return String(this.pick(this.selectedRoom, ['name', 'roomName', 'room_name'], ''))
    },
    cabinets() {
      return this.visualRows.flatMap(row => row.cabinets).filter(item => !item.placeholder)
    },
    visualRows() {
      const rawRows = (this.layoutInfo.rows || []).map((row, rowIndex) => {
        const rowNo = this.num(this.pick(row, ['rowNo', 'row_no']), rowIndex + 1)
        const rowLabel = String(this.pick(row, ['rowLabel', 'row_label'], `${String(rowNo).padStart(2, '0')}列`))
        return {
          ...row,
          sourceIndex: rowIndex,
          rowNo,
          rowLabel,
          cabinets: this.list(row.cabinets)
            .map((item, cabinetIndex) => this.normalizeCabinet(item, row, rowIndex, cabinetIndex))
            .sort((first, second) => first.cabinetNo - second.cabinetNo)
        }
      })

      const slotNumbers = this.buildCabinetSlotNumbers(rawRows)
      const rowNumbers = this.buildRoomRowNumbers(rawRows)
      const rowByNo = new Map(rawRows.map(row => [row.rowNo, row]))

      return rowNumbers.map((rowNo, rowIndex) => {
        const row = rowByNo.get(rowNo) || {
          rowNo,
          rowLabel: `${String(rowNo).padStart(2, '0')}列`,
          cabinets: []
        }
        const cabinetByNo = new Map(row.cabinets.map(cabinet => [cabinet.cabinetNo, cabinet]))
        const cabinets = slotNumbers.map((slotNo, slotIndex) => {
          const visualIndex = slotNumbers.length - 1 - slotIndex
          const cabinet = cabinetByNo.get(slotNo)
          if (cabinet) {
            return {
              ...cabinet,
              x: visualIndex,
              y: rowIndex,
              slotNo,
              visualIndex,
              visualRowIndex: rowIndex,
              placeholder: false
            }
          }
          return this.createPlaceholderCabinet(row, rowIndex, slotNo, slotIndex, visualIndex)
        })

        return {
          ...row,
          index: rowIndex,
          rowNo,
          rowLabel: row.rowLabel || `${String(rowNo).padStart(2, '0')}列`,
          slotNumbers,
          cabinets
        }
      })
    },
    renderRows() {
      return [...this.visualRows].reverse()
    },
    unmatchedCabinets() {
      return this.list(this.layoutInfo.unmatchedCabinets).map(item => this.normalizeCabinet(item))
    },
    selectedCabinetKey() {
      return this.getCabinetKey(this.selectedCabinet)
    },
    normalCount() {
      return this.cabinets.filter(item => this.hasCabinetData(item) && this.resolveStatus(item) === 'normal').length
    },
    warningCount() {
      return 0
    },
    criticalCount() {
      return this.cabinets.filter(item => this.resolveStatus(item) === 'critical').length
    },
    highUsageCount() {
      return this.cabinets.filter(item => Number(item.usageRate || 0) >= 70).length
    },
    averageUsageRate() {
      if (!this.cabinets.length) return 0
      const total = this.cabinets.reduce((sum, item) => sum + Number(item.usageRate || 0), 0)
      return total / this.cabinets.length
    },
    cabinetAssets() {
      const data = this.cabinetLayout || {}
      const list = this.list(this.pick(data, ['assets', 'servers', 'assetList', 'serverList', 'items', 'list'], []))
      return list.map(item => ({
        ...item,
        id: this.num(this.pick(item, ['id', 'assetId', 'asset_id']), ''),
        ip: String(this.pick(item, ['ip'], '')),
        displayName: String(this.pick(item, ['displayName', 'assetName', 'name'], '')),
        startU: this.num(this.pick(item, ['startU', 'cabinetPosition', 'cabinet_position', 'position']), ''),
        cabinetUSize: this.num(this.pick(item, ['cabinetUSize', 'cabinet_u_size', 'uSize', 'u_size']), 1)
      }))
    },
    canvasTransformStyle() {
      return {
        transform: `translate(${this.offset.x}px, ${this.offset.y}px) scale(${this.scale})`
      }
    }
  },
  watch: {
    activeView(value) {
      if (value === 'room') this.tryLoadLayout()
    },
    roomId() {
      if (this.activeView === 'room') this.tryLoadLayout(true)
    }
  },
  mounted() {
    if (this.activeView === 'room') this.tryLoadLayout()
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
      for (const key of ['list', 'items', 'rows', 'records', 'result', 'assets', 'servers']) {
        if (Array.isArray(payload[key])) return payload[key]
      }
      return []
    },
    normalizeCabinet(item, row = {}, rowIndex = 0, cabinetIndex = 0) {
      const rowNo = this.num(this.pick(item, ['rowNo', 'row_no']), this.num(row.rowNo, rowIndex + 1))
      const cabinetNo = this.num(this.pick(item, ['cabinetNo', 'cabinet_no']), cabinetIndex + 1)
      const usageRate = this.num(this.pick(item, ['usageRate', 'usage_rate']), 0)
      const hardwareSummary = this.pick(item, ['hardwareSummary', 'hardware_summary'], {})
      const hardwareStatus = String(this.pick(item, ['hardwareStatus', 'hardware_status'], this.pick(item, ['status'], 'normal'))).toLowerCase()
      const criticalAlarmCount = this.num(this.pick(hardwareSummary, ['criticalAssetCount', 'critical_asset_count']), 0)
      const alarmCount = criticalAlarmCount
      return {
        ...item,
        cabinetId: this.num(this.pick(item, ['cabinetId', 'cabinet_id', 'id']), 0),
        name: String(this.pick(item, ['name', 'cabinetName', 'cabinet_name'], '未命名机柜')),
        rowNo,
        rowLabel: String(this.pick(row, ['rowLabel', 'row_label'], '')),
        cabinetNo,
        x: this.num(this.pick(item, ['x', 'gridX', 'grid_x']), cabinetIndex),
        y: rowIndex,
        visualIndex: cabinetIndex,
        visualRowIndex: rowIndex,
        totalU: this.num(this.pick(item, ['totalU', 'total_u']), 42),
        usedU: this.num(this.pick(item, ['usedU', 'used_u']), 0),
        freeU: this.num(this.pick(item, ['freeU', 'free_u']), 0),
        usageRate,
        idleRate: this.num(this.pick(item, ['idleRate', 'idle_rate']), Math.max(0, 100 - usageRate)),
        assetCount: this.num(this.pick(item, ['assetCount', 'asset_count']), 0),
        alarmCount,
        warningAlarmCount: 0,
        criticalAlarmCount,
        hardwareStatus: hardwareStatus || 'normal',
        hardwareSummary,
        status: String(this.pick(item, ['status'], hardwareStatus)).toLowerCase() || 'normal',
        parseStatus: String(this.pick(item, ['parseStatus', 'parse_status'], '')).toLowerCase(),
        placeholder: false
      }
    },
    normalizeLayout(data = {}) {
      return {
        ...EMPTY_LAYOUT,
        ...data,
        roomId: this.num(this.pick(data, ['roomId', 'room_id', 'id']), this.roomId),
        roomName: String(this.pick(data, ['roomName', 'room_name', 'name'], this.selectedRoomName)),
        cabinetCount: this.num(this.pick(data, ['cabinetCount', 'cabinet_count']), 0),
        assetCount: this.num(this.pick(data, ['assetCount', 'asset_count']), 0),
        layoutMode: String(this.pick(data, ['layoutMode', 'layout_mode'], 'name_order')),
        rows: this.list(this.pick(data, ['rows', 'rowList', 'row_list'], [])),
        unmatchedCabinets: this.list(this.pick(data, ['unmatchedCabinets', 'unmatched_cabinets'], []))
      }
    },
    resolveStatus(cabinet) {
      if (cabinet?.placeholder || !this.hasCabinetData(cabinet)) return 'empty'
      const hardwareStatus = String(cabinet?.hardwareStatus || cabinet?.status || '').toLowerCase()
      if (hardwareStatus === 'critical' || Number(cabinet?.criticalAlarmCount || 0) > 0) return 'critical'
      return 'normal'
    },
    cabinetAlarmTitle(cabinet) {
      const parts = []
      const criticalAlarmCount = Number(cabinet?.criticalAlarmCount || 0)

      if (criticalAlarmCount > 0) parts.push(`严重硬件异常 ${criticalAlarmCount} 台`)

      return `${cabinet?.name || '机柜'}：${parts.length ? parts.join('，') : '存在严重物理机硬件异常'}`
    },
    cabinetStatusClass(cabinet) {
      return `cabinet-${this.resolveStatus(cabinet)}`
    },
    getCabinetKey(cabinet) {
      if (!cabinet) return ''
      return cabinet.cabinetId ? `id-${cabinet.cabinetId}` : `name-${cabinet.name}`
    },
    formatRate(value) {
      const number = Number(value || 0)
      return `${number.toFixed(1)}%`
    },
    hasCabinetData(cabinet) {
      return !!cabinet && !cabinet.placeholder
    },
    buildCabinetSlotNumbers(rows) {
      const numbers = rows.flatMap(row => row.cabinets.map(cabinet => Number(cabinet.cabinetNo || 0))).filter(number => number > 0)
      if (!numbers.length) return Array.from({ length: 11 }, (_, index) => index + 1)
      const min = Math.min(...numbers)
      const max = Math.max(...numbers)
      const count = Math.max(11, max - min + 1)
      return Array.from({ length: count }, (_, index) => min + index)
    },
    buildRoomRowNumbers(rows) {
      const numbers = rows.map(row => Number(row.rowNo || 0)).filter(number => number > 0)
      const max = Math.max(4, ...numbers)
      return Array.from({ length: max }, (_, index) => index + 1)
    },
    createPlaceholderCabinet(row, rowIndex, slotNo, slotIndex, visualIndex = slotIndex) {
      const rowNo = Number(row.rowNo || rowIndex + 1)
      return {
        cabinetId: 0,
        name: `${String(rowNo).padStart(2, '0')}列${String(slotNo).padStart(2, '0')}柜`,
        rowNo,
        rowLabel: row.rowLabel || `${String(rowNo).padStart(2, '0')}列`,
        cabinetNo: slotNo,
        slotNo,
        x: visualIndex,
        y: rowIndex,
        visualIndex,
        visualRowIndex: rowIndex,
        totalU: 42,
        usedU: 0,
        freeU: 42,
        usageRate: 0,
        idleRate: 100,
        assetCount: 0,
        status: 'empty',
        parseStatus: 'placeholder',
        placeholder: true
      }
    },
    getSceneMetrics() {
      return {
        originX: 210,
        originY: 280,
        slotVector: { x: 60, y: 42 },
        rowVector: { x: 93, y: -55 },
        heightVector: { x: 0, y: -104 },
        depthVector: { x: 43, y: -24 }
      }
    },
    toIsoPoint(x, y) {
      const metrics = this.getSceneMetrics()
      const slotIndex = Number(x || 0)
      const rowIndex = Number(y || 0)
      return {
        left: metrics.originX + slotIndex * metrics.slotVector.x + rowIndex * metrics.rowVector.x,
        top: metrics.originY + slotIndex * metrics.slotVector.y + rowIndex * metrics.rowVector.y
      }
    },
    addPoint(point, vector, scale = 1) {
      return {
        x: point.x + vector.x * scale,
        y: point.y + vector.y * scale
      }
    },
    pointString(points) {
      return points.map(point => `${point.x.toFixed(1)},${point.y.toFixed(1)}`).join(' ')
    },
    rowBankGeometry(row) {
      const metrics = this.getSceneMetrics()
      const slotCount = Math.max(1, row.cabinets.length)
      const base = this.toSvgPoint(this.toIsoPoint(0, row.index))
      const end = this.addPoint(base, metrics.slotVector, slotCount)
      const topLeft = this.addPoint(base, metrics.heightVector)
      const topRight = this.addPoint(end, metrics.heightVector)
      const backBottomLeft = this.addPoint(base, metrics.depthVector)
      const backBottomRight = this.addPoint(end, metrics.depthVector)
      const backTopLeft = this.addPoint(topLeft, metrics.depthVector)
      const backTopRight = this.addPoint(topRight, metrics.depthVector)
      return {
        base,
        end,
        topLeft,
        topRight,
        backBottomLeft,
        backBottomRight,
        backTopLeft,
        backTopRight,
        slotCount,
        slotVector: metrics.slotVector,
        heightVector: metrics.heightVector,
        depthVector: metrics.depthVector
      }
    },
    roomGeometry() {
      const metrics = this.getSceneMetrics()
      const slotCount = Math.max(11, this.visualRows[0]?.cabinets?.length || 0)
      const rowCount = Math.max(4, this.visualRows.length || 0)
      const floorSlotCount = slotCount + 5.6
      const floorRowCount = rowCount + 4.65
      const wallVector = { x: 0, y: -220 }
      const frontLeft = {
        x: metrics.originX - metrics.slotVector.x * 2.45 - metrics.rowVector.x * 1.55,
        y: metrics.originY - metrics.slotVector.y * 2.45 - metrics.rowVector.y * 1.55 + 178
      }
      const frontRight = this.addPoint(frontLeft, metrics.slotVector, floorSlotCount)
      const backLeft = this.addPoint(frontLeft, metrics.rowVector, floorRowCount)
      const backRight = this.addPoint(frontRight, metrics.rowVector, floorRowCount)
      const backTopLeft = this.addPoint(backLeft, wallVector)
      const backTopRight = this.addPoint(backRight, wallVector)
      const rightTopFront = this.addPoint(frontRight, wallVector)
      const rightTopBack = this.addPoint(backRight, wallVector)
      return {
        frontLeft,
        frontRight,
        backLeft,
        backRight,
        backTopLeft,
        backTopRight,
        rightTopFront,
        rightTopBack,
        wallVector,
        floorSlotCount,
        floorRowCount
      }
    },
    roomFloorPoints() {
      const geometry = this.roomGeometry()
      return this.pointString([geometry.frontLeft, geometry.frontRight, geometry.backRight, geometry.backLeft])
    },
    roomFloorShadowPoints() {
      const geometry = this.roomGeometry()
      return this.pointString([
        { x: geometry.frontLeft.x - 16, y: geometry.frontLeft.y + 22 },
        { x: geometry.frontRight.x + 24, y: geometry.frontRight.y + 18 },
        { x: geometry.backRight.x + 36, y: geometry.backRight.y + 36 },
        { x: geometry.backLeft.x - 18, y: geometry.backLeft.y + 34 }
      ])
    },
    roomFloorGridLines() {
      const geometry = this.roomGeometry()
      const metrics = this.getSceneMetrics()
      const lines = []

      for (let index = 0; index <= Math.ceil(geometry.floorSlotCount); index += 1) {
        const start = this.addPoint(geometry.frontLeft, metrics.slotVector, index)
        const end = this.addPoint(start, metrics.rowVector, geometry.floorRowCount)
        lines.push(this.pointString([start, end]))
      }

      for (let index = 0; index <= Math.ceil(geometry.floorRowCount); index += 1) {
        const start = this.addPoint(geometry.frontLeft, metrics.rowVector, index)
        const end = this.addPoint(start, metrics.slotVector, geometry.floorSlotCount)
        lines.push(this.pointString([start, end]))
      }

      return lines
    },
    roomBackWallPoints() {
      const geometry = this.roomGeometry()
      return this.pointString([geometry.backLeft, geometry.backRight, geometry.backTopRight, geometry.backTopLeft])
    },
    roomLeftWallPoints() {
      const geometry = this.roomGeometry()
      return this.pointString([geometry.frontRight, geometry.backRight, geometry.rightTopBack, geometry.rightTopFront])
    },
    roomWallOutlinePoints() {
      const geometry = this.roomGeometry()
      return this.pointString([
        geometry.backLeft,
        geometry.backRight,
        geometry.backTopRight,
        geometry.backTopLeft,
        geometry.backLeft,
        geometry.backRight,
        geometry.frontRight,
        geometry.rightTopFront,
        geometry.rightTopBack
      ])
    },
    roomFloorOutlinePoints() {
      const geometry = this.roomGeometry()
      return this.pointString([geometry.frontLeft, geometry.frontRight, geometry.backRight, geometry.backLeft, geometry.frontLeft])
    },
    roomBackEdgePath() {
      const geometry = this.roomGeometry()
      return `M${geometry.backLeft.x.toFixed(1)} ${geometry.backLeft.y.toFixed(1)} L${geometry.backRight.x.toFixed(1)} ${geometry.backRight.y.toFixed(1)}`
    },
    toSvgPoint(point) {
      return {
        x: point.left,
        y: point.top
      }
    },
    cabinetFacePoints(row, slotIndex) {
      const geometry = this.rowBankGeometry(row)
      const leftBottom = this.addPoint(geometry.base, geometry.slotVector, slotIndex)
      const rightBottom = this.addPoint(geometry.base, geometry.slotVector, slotIndex + 1)
      const rightTop = this.addPoint(rightBottom, geometry.heightVector)
      const leftTop = this.addPoint(leftBottom, geometry.heightVector)
      return this.pointString([leftTop, rightTop, rightBottom, leftBottom])
    },
    cabinetFacePoint(row, slotIndex, slotRatio, heightRatio) {
      const geometry = this.rowBankGeometry(row)
      const bottom = this.addPoint(geometry.base, geometry.slotVector, slotIndex + slotRatio)
      return this.addPoint(bottom, geometry.heightVector, heightRatio)
    },
    cabinetInsetFacePoints(row, slotIndex, leftRatio = 0.12, rightRatio = 0.88, bottomRatio = 0.1, topRatio = 0.92) {
      return this.pointString([
        this.cabinetFacePoint(row, slotIndex, leftRatio, topRatio),
        this.cabinetFacePoint(row, slotIndex, rightRatio, topRatio),
        this.cabinetFacePoint(row, slotIndex, rightRatio, bottomRatio),
        this.cabinetFacePoint(row, slotIndex, leftRatio, bottomRatio)
      ])
    },
    cabinetDoorFramePoints(row, slotIndex) {
      return this.cabinetInsetFacePoints(row, slotIndex, 0.1, 0.9, 0.08, 0.92)
    },
    cabinetHeaderStripPoints(row, slotIndex) {
      return this.cabinetInsetFacePoints(row, slotIndex, 0.16, 0.84, 0.78, 0.9)
    },
    cabinetRailLinePoints(row, slotIndex, slotRatio) {
      return this.pointString([
        this.cabinetFacePoint(row, slotIndex, slotRatio, 0.12),
        this.cabinetFacePoint(row, slotIndex, slotRatio, 0.88)
      ])
    },
    cabinetTopPoints(row, slotIndex) {
      const geometry = this.rowBankGeometry(row)
      const leftTop = this.addPoint(this.addPoint(geometry.base, geometry.slotVector, slotIndex), geometry.heightVector)
      const rightTop = this.addPoint(this.addPoint(geometry.base, geometry.slotVector, slotIndex + 1), geometry.heightVector)
      const rightBack = this.addPoint(rightTop, geometry.depthVector)
      const leftBack = this.addPoint(leftTop, geometry.depthVector)
      return this.pointString([leftBack, rightBack, rightTop, leftTop])
    },
    rowBankEndPoints(row) {
      const geometry = this.rowBankGeometry(row)
      return this.pointString([geometry.topRight, geometry.backTopRight, geometry.backBottomRight, geometry.end])
    },
    rowBankOutlinePoints(row) {
      const geometry = this.rowBankGeometry(row)
      return this.pointString([
        geometry.base,
        geometry.end,
        geometry.backBottomRight,
        geometry.backTopRight,
        geometry.backTopLeft,
        geometry.topLeft,
        geometry.base
      ])
    },
    rowBankShadowPoints(row) {
      const geometry = this.rowBankGeometry(row)
      return this.pointString([
        { x: geometry.base.x - 24, y: geometry.base.y + 26 },
        { x: geometry.end.x + 22, y: geometry.end.y + 14 },
        { x: geometry.backBottomRight.x + 44, y: geometry.backBottomRight.y + 42 },
        { x: geometry.backBottomLeft.x - 4, y: geometry.backBottomLeft.y + 56 }
      ])
    },
    cabinetSlotLinePoints(row, slotIndex, lineIndex) {
      const t = 0.14 + lineIndex * 0.065
      return this.pointString([
        this.cabinetFacePoint(row, slotIndex, 0.18, t),
        this.cabinetFacePoint(row, slotIndex, 0.82, t)
      ])
    },
    cabinetDeviceBlockPoints(row, slotIndex, blockIndex) {
      const top = 0.2 + blockIndex * 0.12
      return this.cabinetInsetFacePoints(row, slotIndex, 0.2, 0.8, top, top + 0.055)
    },
    cabinetLedPoint(row, slotIndex) {
      return this.cabinetFacePoint(row, slotIndex, 0.5, 0.17)
    },
    cabinetTextPoint(row, slotIndex) {
      return this.cabinetFacePoint(row, slotIndex, 0.5, 0.69)
    },
    cabinetTextTransform(row, slotIndex) {
      const point = this.cabinetTextPoint(row, slotIndex)
      const vector = this.getSceneMetrics().slotVector
      const angle = Math.atan2(vector.y, vector.x) * 180 / Math.PI
      return `rotate(${angle.toFixed(2)} ${point.x.toFixed(1)} ${point.y.toFixed(1)})`
    },
    cabinetDisplayNo(cabinet) {
      return String(cabinet?.cabinetNo || cabinet?.slotNo || '').padStart(2, '0')
    },
    cabinetTopVentPoint(row, slotIndex) {
      const geometry = this.rowBankGeometry(row)
      const topCenter = this.addPoint(this.addPoint(geometry.base, geometry.slotVector, slotIndex + 0.5), geometry.heightVector)
      return this.addPoint(topCenter, geometry.depthVector, 0.48)
    },
    cabinetAlarmPoint(row, slotIndex) {
      const geometry = this.rowBankGeometry(row)
      const topCenter = this.addPoint(this.addPoint(geometry.base, geometry.slotVector, slotIndex + 0.5), geometry.heightVector, 1.04)
      return this.addPoint(topCenter, geometry.depthVector, 0.56)
    },
    cabinetAlarmTrianglePoints(row, slotIndex) {
      const point = this.cabinetAlarmPoint(row, slotIndex)
      return this.pointString([
        { x: point.x, y: point.y - 15 },
        { x: point.x + 14, y: point.y + 12 },
        { x: point.x - 14, y: point.y + 12 }
      ])
    },
    rowLabelPoint(row) {
      const geometry = this.rowBankGeometry(row)
      return {
        x: (geometry.end.x + geometry.backBottomRight.x + geometry.topRight.x + geometry.backTopRight.x) / 4,
        y: (geometry.end.y + geometry.backBottomRight.y + geometry.topRight.y + geometry.backTopRight.y) / 4
      }
    },
    rowLabelTransform(row) {
      const point = this.rowLabelPoint(row)
      const vector = this.getSceneMetrics().depthVector
      const angle = Math.atan2(vector.y, vector.x) * 180 / Math.PI
      return `rotate(${angle.toFixed(2)} ${point.x.toFixed(1)} ${point.y.toFixed(1)})`
    },
    handleCabinetCellClick(cabinet) {
      if (cabinet?.placeholder) return
      this.selectCabinet(cabinet)
    },
    handleCabinetCellDblClick(cabinet) {
      if (cabinet?.placeholder) return
      this.focusCabinet(cabinet)
    },
    resetView() {
      this.scale = 0.78
      this.offset = { x: -26, y: 18 }
    },
    zoomIn() {
      this.scale = Math.min(1.55, Number((this.scale + 0.1).toFixed(2)))
    },
    zoomOut() {
      this.scale = Math.max(0.48, Number((this.scale - 0.1).toFixed(2)))
    },
    handleWheel(event) {
      const direction = event.deltaY > 0 ? -0.06 : 0.06
      this.scale = Math.min(1.7, Math.max(0.48, Number((this.scale + direction).toFixed(2))))
    },
    handleViewportPointerDown(event) {
      if (event.button !== 0) return
      this.startDrag(event)
    },
    handleCanvasPointerDown(event) {
      if (event.button !== 0) return
      this.startDrag(event)
    },
    startDrag(event) {
      this.dragState = {
        active: true,
        x: event.clientX,
        y: event.clientY,
        startX: this.offset.x,
        startY: this.offset.y
      }
      event.currentTarget?.setPointerCapture?.(event.pointerId)
    },
    handleViewportPointerMove(event) {
      if (!this.dragState.active) return
      this.offset = {
        x: this.dragState.startX + event.clientX - this.dragState.x,
        y: this.dragState.startY + event.clientY - this.dragState.y
      }
    },
    handleViewportPointerUp(event) {
      this.dragState.active = false
      event.currentTarget?.releasePointerCapture?.(event.pointerId)
    },
    async tryLoadLayout(force = false) {
      if (!this.hasRoomSelection) {
        this.layoutInfo = { ...EMPTY_LAYOUT }
        this.selectedCabinet = null
        this.cabinetLayout = null
        return
      }
      if (!force && this.layoutInfo.roomId === this.roomId && this.cabinets.length) return
      await this.loadLayout()
    },
    reloadLayout() {
      this.tryLoadLayout(true)
    },
    async loadLayout() {
      this.loadingLayout = true
      try {
        const { data: res } = await this.$api.getCmdbIdcRoomLayout({ roomId: this.roomId })
        if (res.code !== 200) throw new Error(res.message || '获取机房布局失败')
        this.layoutInfo = this.normalizeLayout(res.data || {})
        this.selectedCabinet = null
        this.cabinetLayout = null
        this.resetView()
      } catch (error) {
        console.error('load room layout failed:', error)
        this.layoutInfo = { ...EMPTY_LAYOUT }
        this.$message.error(error.message || '获取机房布局失败')
      } finally {
        this.loadingLayout = false
      }
    },
    buildSelectedCabinet(cabinet) {
      const cabinetId = this.num(this.pick(cabinet, ['cabinetId', 'cabinet_id', 'id']), '')
      const cabinetName = String(this.pick(cabinet, ['name', 'cabinetName', 'cabinet_name'], ''))
      return {
        ...cabinet,
        id: cabinetId,
        cabinetId,
        name: cabinetName,
        cabinetName,
        roomId: this.roomId,
        roomName: this.layoutInfo.roomName || this.selectedRoomName,
        type: 'cabinet',
        nodeKey: cabinetId ? `cabinet-${cabinetId}` : `cabinet-${this.roomId}-${cabinetName}`
      }
    },
    selectCabinet(cabinet) {
      this.selectedCabinet = cabinet
      this.$emit('select-cabinet', this.buildSelectedCabinet(cabinet))
    },
    focusCabinet(cabinet) {
      const point = this.toIsoPoint(cabinet.x, cabinet.y)
      this.offset = {
        x: 560 - point.left,
        y: 360 - point.top
      }
      this.scale = Math.max(this.scale, 1.08)
    },
    async loadCabinetDetail() {
      if (!this.selectedCabinet?.cabinetId) {
        this.cabinetLayout = null
        return
      }

      this.loadingCabinet = true
      try {
        const { data: res } = await this.$api.getCmdbIdcCabinetLayout({ cabinetId: this.selectedCabinet.cabinetId })
        if (res.code !== 200) throw new Error(res.message || '获取机柜布局失败')
        this.cabinetLayout = res.data || {}
      } catch (error) {
        console.error('load cabinet detail failed:', error)
        this.cabinetLayout = null
        this.$message.error(error.message || '获取机柜布局失败')
      } finally {
        this.loadingCabinet = false
      }
    }
  }
}
</script>

<style scoped>
.idc-room-panel {
  min-width: 0;
}

.toolbar {
  border-radius: 24px;
  border: 1px solid #dfe7f2;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.94), rgba(247, 250, 255, 0.92));
  box-shadow: 0 20px 60px rgba(17, 24, 39, 0.07);
  margin-bottom: 18px;
  overflow: hidden;
}

.toolbar-main {
  display: flex;
  justify-content: space-between;
  gap: 18px;
  padding: 18px 20px 14px;
}

.toolbar-tags,
.toolbar-actions,
.view-tabs,
.scene-badges {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.toolbar-tags > span:first-child {
  color: #172133;
  font-size: 13px;
  font-weight: 700;
}

.hint-text {
  color: #8d97a8;
  font-size: 13px;
}

.toolbar-divider {
  height: 1px;
  background: #dfe7f2;
}

.view-tabs {
  padding: 0 20px;
}

.view-tab {
  position: relative;
  height: 50px;
  padding: 0;
  border: 0;
  background: transparent;
  color: #5f6b7c;
  cursor: pointer;
  font-size: 14px;
  font-weight: 700;
}

.view-tab + .view-tab {
  margin-left: 22px;
}

.view-tab.active {
  color: #4c8bf5;
}

.view-tab.active::after {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  height: 2px;
  background: #4c8bf5;
  content: "";
}

.room-board {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 300px;
  gap: 18px;
  min-height: calc(100vh - 315px);
  border-radius: 24px;
  border: 1px solid #dfe7f2;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.94), rgba(239, 247, 255, 0.92));
  box-shadow: 0 20px 60px rgba(17, 24, 39, 0.07);
  padding: 16px;
}

.room-viewport {
  position: relative;
  min-height: 640px;
  overflow: hidden;
  border-radius: 18px;
  background:
    radial-gradient(circle at 72% 20%, rgba(30, 41, 59, 0.34), transparent 32%),
    linear-gradient(145deg, #050816 0%, #070b18 52%, #030712 100%);
  cursor: grab;
  touch-action: none;
}

.room-viewport:active {
  cursor: grabbing;
}

.scene-header {
  position: absolute;
  top: 18px;
  right: 18px;
  left: 18px;
  z-index: 50;
  display: flex;
  justify-content: space-between;
  gap: 16px;
  pointer-events: none;
}

.scene-header h3 {
  margin: 0;
  color: #f8fbff;
  font-size: 20px;
}

.scene-header p {
  margin: 4px 0 0;
  color: rgba(190, 207, 232, 0.76);
  font-size: 12px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.status-pill {
  padding: 6px 10px;
  border-radius: 999px;
  color: #dbeafe;
  background: rgba(24, 33, 50, 0.72);
  border: 1px solid rgba(104, 132, 179, 0.35);
  font-size: 12px;
}

.status-pill.normal {
  color: #8df5d4;
}

.status-pill.warning {
  color: #ffd38b;
}

.status-pill.critical {
  color: #ff9aa5;
}

.room-canvas {
  position: absolute;
  inset: 0;
  width: 1380px;
  height: 900px;
  overflow: visible;
  transform-origin: center center;
  transition: transform 0.12s ease;
}

.iso-floor {
  position: absolute;
  inset: 0;
  width: 1200px;
  height: 760px;
  overflow: visible;
  filter: drop-shadow(0 22px 34px rgba(0, 0, 0, 0.24));
}

.room-floor-grid polyline {
  fill: none;
  stroke: rgba(109, 125, 149, 0.26);
  stroke-width: 1.2;
  vector-effect: non-scaling-stroke;
}

.room-floor-grid polyline:nth-child(4n) {
  stroke: rgba(139, 157, 184, 0.34);
}

.rack-bank-layer {
  position: absolute;
  inset: 0;
  width: 1200px;
  height: 760px;
  overflow: visible;
}

.rack-bank {
  filter: drop-shadow(18px 28px 18px rgba(0, 0, 0, 0.3));
}

.rack-bank-shadow {
  fill: rgba(0, 0, 0, 0.28);
  stroke: none;
  filter: blur(3px);
}

.rack-top-cell {
  fill: url("#rack-top-filled");
  stroke: rgba(14, 22, 35, 0.62);
  stroke-width: 1;
}

.rack-top-cell.empty,
.rack-top-cell.placeholder {
  fill: url("#rack-top-empty");
}

.rack-end-face {
  fill: #1b2534;
  stroke: rgba(116, 138, 168, 0.62);
  stroke-width: 1.2;
}

.rack-cabinet-cell {
  cursor: pointer;
}

.rack-cabinet-cell.placeholder {
  cursor: default;
}

.rack-cabinet-face {
  fill: url("#rack-face-filled");
  stroke: rgba(11, 18, 31, 0.86);
  stroke-width: 1.1;
  transition: fill 0.16s ease, filter 0.16s ease, stroke 0.16s ease;
}

.rack-cabinet-cell.empty .rack-cabinet-face,
.rack-cabinet-cell.placeholder .rack-cabinet-face {
  fill: url("#rack-face-empty");
  filter: grayscale(0.5) brightness(0.82);
}

.rack-cabinet-cell:hover .rack-cabinet-face,
.rack-cabinet-cell.selected .rack-cabinet-face {
  filter: brightness(1.2);
  stroke: #38bdf8;
  stroke-width: 2.4;
}

.rack-cabinet-cell.cabinet-warning .rack-cabinet-face {
  stroke: #f59e0b;
  stroke-width: 2;
}

.rack-cabinet-cell.cabinet-critical .rack-cabinet-face {
  stroke: #ef4444;
  stroke-width: 2.2;
}

.rack-door-frame {
  fill: rgba(8, 13, 22, 0.22);
  stroke: rgba(202, 213, 226, 0.22);
  stroke-width: 1.1;
  pointer-events: none;
}

.rack-header-strip {
  fill: #3b82f6;
  stroke: rgba(219, 234, 254, 0.38);
  stroke-width: 0.8;
  pointer-events: none;
}

.rack-header-strip.warning {
  fill: #f59e0b;
}

.rack-header-strip.critical {
  fill: #ef4444;
}

.rack-rail-line {
  fill: none;
  stroke: rgba(226, 235, 247, 0.2);
  stroke-width: 1.1;
  pointer-events: none;
}

.rack-slot-line {
  fill: none;
  stroke: rgba(226, 235, 247, 0.14);
  stroke-width: 0.85;
  pointer-events: none;
}

.rack-device-block {
  fill: rgba(83, 102, 124, 0.78);
  stroke: rgba(168, 185, 207, 0.28);
  stroke-width: 0.7;
  pointer-events: none;
}

.rack-device-block.normal {
  fill: rgba(74, 92, 112, 0.84);
}

.rack-device-block.warning {
  fill: rgba(108, 84, 43, 0.86);
}

.rack-device-block.critical {
  fill: rgba(108, 48, 50, 0.9);
}

.rack-led {
  fill: #38bdf8;
  stroke: rgba(219, 234, 254, 0.9);
  stroke-width: 1;
  filter: drop-shadow(0 0 5px rgba(56, 189, 248, 0.88));
  pointer-events: none;
}

.rack-led.warning {
  fill: #f59e0b;
  filter: drop-shadow(0 0 5px rgba(245, 158, 11, 0.86));
}

.rack-led.critical {
  fill: #ef4444;
  filter: drop-shadow(0 0 6px rgba(239, 68, 68, 0.9));
}

.rack-top-vent {
  fill: #22c55e;
  stroke: rgba(220, 252, 231, 0.92);
  stroke-width: 1;
  filter: drop-shadow(0 0 5px rgba(34, 197, 94, 0.72));
  pointer-events: none;
}

.rack-alarm-badge {
  pointer-events: none;
}

.rack-alarm-badge polygon {
  fill: #f59e0b;
  stroke: rgba(255, 237, 213, 0.9);
  stroke-width: 1.4;
}

.rack-alarm-badge.critical polygon {
  fill: #ef4444;
  stroke: rgba(254, 226, 226, 0.95);
}

.rack-alarm-badge text {
  fill: #fff7ed;
  font-size: 18px;
  font-weight: 900;
  text-anchor: middle;
  dominant-baseline: middle;
}

.rack-cabinet-no {
  fill: #f8fbff;
  font-size: 11px;
  font-weight: 800;
  text-anchor: middle;
  dominant-baseline: middle;
  pointer-events: none;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.72);
}

.rack-cabinet-no.empty {
  fill: #aeb8c8;
}

.rack-bank-outline {
  fill: none;
  stroke: rgba(219, 234, 254, 0.46);
  stroke-width: 1.4;
  pointer-events: none;
}

.rack-row-label {
  fill: #38bdf8;
  font-size: 12px;
  font-weight: 900;
  text-anchor: middle;
  dominant-baseline: middle;
  paint-order: stroke;
  stroke: rgba(3, 7, 18, 0.96);
  stroke-width: 3px;
  pointer-events: none;
}

.room-side {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.info-card {
  border-radius: 18px;
  border: 1px solid rgba(43, 54, 74, 0.12);
  background: rgba(255, 255, 255, 0.78);
  padding: 16px;
  box-shadow: 0 18px 46px rgba(17, 24, 39, 0.08);
}

.card-title {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  margin-bottom: 12px;
}

.card-title h3 {
  margin: 0;
  color: #172133;
  font-size: 16px;
}

.card-title span {
  color: #8d97a8;
  font-size: 12px;
}

.metric-list {
  display: grid;
  gap: 10px;
}

.metric-list div {
  display: flex;
  justify-content: space-between;
  gap: 14px;
  padding-bottom: 10px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.8);
}

.metric-list div:last-child {
  border-bottom: 0;
  padding-bottom: 0;
}

.metric-list span {
  color: #5f6b7c;
  font-size: 13px;
}

.metric-list strong {
  min-width: 0;
  overflow: hidden;
  color: #172133;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
}

.metric-list.compact {
  margin-bottom: 12px;
}

.capacity-ring {
  width: 156px;
  height: 156px;
  margin: 8px auto 14px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: conic-gradient(#4c8bf5 var(--rate), rgba(223, 231, 242, 0.95) 0);
}

.capacity-ring > div {
  width: 104px;
  height: 104px;
  display: grid;
  place-items: center;
  align-content: center;
  border-radius: 50%;
  background: #fff;
}

.capacity-ring strong {
  color: #172133;
  font-size: 22px;
}

.capacity-ring span,
.legend-list span {
  color: #5f6b7c;
  font-size: 12px;
}

.legend-list {
  display: grid;
  gap: 8px;
}

.legend-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  margin-right: 8px;
  border-radius: 50%;
  background: #3fb97f;
}

.legend-dot.warning {
  background: #f3b24b;
}

.legend-dot.critical {
  background: #ef6b73;
}

.cabinet-assets {
  max-height: 190px;
  overflow: auto;
}

.asset-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 10px;
  border-radius: 10px;
  background: rgba(76, 139, 245, 0.06);
  color: #172133;
  font-size: 12px;
}

.asset-row + .asset-row {
  margin-top: 6px;
}

.asset-row span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.asset-row em {
  flex-shrink: 0;
  color: #8d97a8;
  font-style: normal;
}

.unmatched-card p {
  margin: 0 0 10px;
  color: #5f6b7c;
  font-size: 12px;
  line-height: 1.6;
}

.unmatched-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.unmatched-list span {
  padding: 4px 8px;
  border-radius: 999px;
  color: #b45309;
  background: rgba(243, 178, 75, 0.14);
  font-size: 12px;
}

@media (max-width: 1280px) {
  .room-board {
    grid-template-columns: 1fr;
  }

  .room-side {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    display: grid;
  }
}
</style>
