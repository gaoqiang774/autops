<template>
  <div class="cmdb-host-management">
    <el-card shadow="hover" class="host-card">
    <!-- 左右布局容器 -->
    <div class="host-management-container">
      <!-- 左侧分组树 -->
      <CmdbGroup
        ref="cmdbGroup"
        :group-list="groupList"
        :expanded-keys="expandedKeys"
        @group-search="handleGroupSearch"
        @group-click="handleGroupClick"
        @node-expand="handleNodeExpand"
        @node-collapse="handleNodeCollapse"
        @collapse-all="handleCollapseAll"
        @expand-all="handleExpandAll"
        @refresh-groups="handleRefreshGroups"
        @create-group="handleCreateGroup"
        @update-group="handleUpdateGroup"
        @delete-group="handleDeleteGroup"
      />
      <!-- 右侧主机管理区域 -->
      <div class="host-table-section">
        <!-- 搜索表单 -->
        <div class="search-section">
          <el-form :inline="true" :model="queryParams" class="demo-form-inline">
            <el-form-item label="主机名称/IP地址" prop="keyword">
              <el-input
                  size="small"
                  placeholder="请输入主机名称或IP地址"
                  clearable
                  v-model="queryParams.keyword"
                  @keyup.enter="handleQuery"
                  style="width: 220px;"
              />
            </el-form-item>
            <el-form-item label="主机状态" prop="status">
              <el-select size="small" placeholder="请选择状态" v-model="queryParams.status" style="width: 120px;">
                <el-option v-for="item in statusList" :key="item.value" :label="item.label" :value="item.value"></el-option>
              </el-select>
            </el-form-item>

            <!-- 主机统计信息 -->
            <el-form-item v-if="hostStatistics" style="margin-left: 20px;">
              <div class="inline-statistics">
                <div class="stat-item">
                  <el-icon :size="20" color="#409EFF"><Monitor /></el-icon>
                  <span class="stat-text">总计: {{ hostStatistics.total || 0 }}</span>
                </div>
                <div
                  v-for="detail in hostStatistics.details"
                  :key="detail.vendor"
                  class="stat-item stat-item--clickable"
                  :class="{ 'is-active': isStatisticActive(detail) }"
                  role="button"
                  tabindex="0"
                  :title="`查看${getStatisticName(detail)}主机`"
                  @click="handleVendorStatisticClick(detail)"
                  @keyup.enter="handleVendorStatisticClick(detail)"
                >
                  <el-icon v-if="Number(detail.vendor) === 1" :size="20" color="#409EFF"><OfficeBuilding /></el-icon>
                  <img v-else-if="Number(detail.vendor) === 2" src="@/assets/image/aliyun.png" class="stat-icon"/>
                  <img v-else-if="Number(detail.vendor) === 3" src="@/assets/image/tengxun.png" class="stat-icon"/>
                  <img v-else-if="Number(detail.vendor) === 4" src="@/assets/image/baidu.svg" class="stat-icon"/>
                  <img v-else-if="Number(detail.vendor) === 5" src="@/assets/image/华为.svg" class="stat-icon"/>
                  <img v-else-if="Number(detail.vendor) === 6" src="@/assets/image/Windows.svg" class="stat-icon"/>
                  <img v-else-if="Number(detail.vendor) === 7" src="@/assets/image/tianyiyun.svg" class="stat-icon"/>
                  <span class="stat-text">{{ getStatisticCount(detail) }}</span>
                </div>
              </div>
            </el-form-item>
            <!-- 操作按钮 -->
            <div class="action-section">
            <el-row :gutter="10" class="mb8" style="text-align: left">
              <el-col :span="24" class="action-toolbar">
                <!-- 搜索按钮 - 蓝色 -->
                <el-button type="primary" size="small" @click="handleQuery" style="margin-right: 10px">
                  <el-icon><Search /></el-icon>
                  <span style="margin-left: 4px">搜索</span>
                </el-button>
                
                <!-- 重置按钮 - 黄色 -->
                <el-button type="warning" size="small" @click="resetQuery" style="margin-right: 10px">
                  <el-icon><Refresh /></el-icon>
                  <span style="margin-left: 4px">重置</span>
                </el-button>

                <!--新建主机 - 绿色-->
                <el-dropdown
                  ref="createDropdown"
                  @command="handleCreateCommand"
                  @visible-change="handleDropdownVisibleChange"
                  :hide-on-click="true"
                  trigger="click"
                  placement="bottom-start">
                  <el-button
                    type="success"
                    size="small"
                    style="margin-right: 10px"
                    v-authority="['cmdb:ecs:add']"
                    @click.stop="handleCreateClick">
                    <el-icon><Plus /></el-icon>
                    <span style="margin-left: 4px">新建</span>
                    <el-icon class="el-icon--right"><ArrowDown /></el-icon>
                  </el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="importHost"><el-icon color="#409EFC" :size="20"><Edit /></el-icon>新增主机</el-dropdown-item>
                      <el-dropdown-item command="excelImport"><el-icon color="#409EFC" :size="20"><Folder /></el-icon>Excel创建主机</el-dropdown-item>
                      <el-dropdown-item command="cloudHost"><el-icon color="#409EFC" :size="21"><MostlyCloudy /></el-icon>新增云主机</el-dropdown-item>
                      <el-dropdown-item command="jumpserverImport"><el-icon color="#67C23A" :size="20"><Connection /></el-icon>JumpServer导入</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>

                <!--终端按钮 - 深色-->
                <el-button
                  size="small"
                  v-authority="['cmdb:ecs:terminal']"
                  @click="handleHostSSH"
                  class="terminal-dark-btn"
                  style="margin-left: 10px"
                >
                  <el-icon><Monitor /></el-icon>
                  <span style="margin-left: 4px">终端</span>
                </el-button>

                <!--资产授权按钮 - 深蓝色-->
                <el-button
                  size="small"
                  type="primary"
                  @click="handleHostAuthorization"
                  style="margin-left: 10px; background: #003d99; border-color: #003d99;"
                >
                  <el-icon><Key /></el-icon>
                  <span style="margin-left: 4px">资产授权</span>
                </el-button>

                <!-- 批量操作按钮 -->
                <el-dropdown
                  @command="handleBatchCommand"
                  trigger="click"
                  placement="bottom-start"
                  style="margin-left: 10px">
                  <el-button
                    size="small"
                    :disabled="!hasSelection"
                    :type="hasSelection ? 'primary' : 'info'"
                    v-authority="['cmdb:ecs:batch']">
                    <el-icon><Operation /></el-icon>
                    <span style="margin-left: 4px">批量操作</span>
                    <span v-if="selectionCount > 0" style="margin-left: 4px">({{ selectionCount }})</span>
                    <el-icon class="el-icon--right"><ArrowDown /></el-icon>
                  </el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="updateGroup">
                        <el-icon color="#409EFC" :size="18"><FolderOpened /></el-icon>
                        修改分组
                      </el-dropdown-item>
                      <el-dropdown-item command="updateVendor">
                        <el-icon color="#67C23A" :size="18"><Edit /></el-icon>
                        修改类型
                      </el-dropdown-item>
                      <el-dropdown-item command="bindTag">
                        <el-icon color="#E6A23C" :size="18"><PriceTag /></el-icon>
                        绑定标签
                      </el-dropdown-item>
                      <el-dropdown-item command="batchDelete" divided>
                        <el-icon color="#F56C6C" :size="18"><Delete /></el-icon>
                        批量删除
                      </el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>

                <!-- 导出主机 CSV -->
                <el-tooltip content="导出 CSV" placement="top">
                  <el-button
                    type="warning"
                    size="small"
                    class="host-export-btn"
                    :loading="exportLoading"
                    @click="handleExportHosts"
                  >
                    <el-icon v-if="!exportLoading"><Download /></el-icon>
                  </el-button>
                </el-tooltip>
              </el-col>
            </el-row>
            </div>
          </el-form>
        </div>

        <div class="table-card">
          <!-- 主机表格 -->
          <CmdbHostTable
            ref="hostTable"
            :key="$route.fullPath"
            :host-list="hostList"
            :loading="loading"
            @show-detail="showHostDetail"
            @edit-host="showEditHostDialog"
            @show-download="showDownloadDialog"
            @delete-host="handleHostDelete"
            @selection-change="handleHostSelectionChange"
            @refresh-list="getHostList"
          />

          <!-- 分页 -->
          <div class="pagination-section">
            <el-pagination
                @size-change="handleSizeChange"
                @current-change="handleCurrentChange"
                :current-page="queryParams.pageNum"
                :page-sizes="[10, 50, 100]"
                :page-size="queryParams.pageSize"
                layout="total, sizes, prev, pager, next, jumper"
                :total="total"
            ></el-pagination>
          </div>
        </div>
      </div>
    </div>

    <!-- 新增主机对话框 -->
    <CreateHost
      :visible="addDialogVisible"
      :group-list="groupList"
      :auth-list="authList"
      @close="addDialogVisible = false"
      @submit="addHost"
      @refresh-auth-list="getAuthList"
    />

    <!-- 编辑主机对话框 -->
    <EditHost
      :visible="editDialogVisible"
      :host-info="hostInfo"
      :group-list="groupList"
      :auth-list="authList"
      @close="editDialogVisible = false"
      @submit="editHost"
    />

    <!-- 导入云主机对话框 -->
    <CreateCloud v-model="cloudDialogVisible" @success="handleCloudImportSuccess" />

    <!-- Excel导入对话框 -->
    <CreateExcel v-model="ExcelDialogVisible" @success="handleExcelImportSuccess" />

    <!-- JumpServer导入对话框 -->
    <CreateJumpServer v-model="jumpServerDialogVisible" @success="handleJumpServerImportSuccess" />

    <!-- SSH终端对话框 -->
    <HostSSH 
      v-if="sshDialogVisible"
      :visible="sshDialogVisible"
      :host-id="currentHostId"
      @update:visible="val => {
        sshDialogVisible = val
      }"
    />

    <!-- 主机详情抽屉 -->
    <el-drawer
      v-model="detailDrawer"
      class="host-detail-drawer"
      title="主机详情"
      direction="rtl"
      size="min(920px, 94vw)"
      append-to-body
      :before-close="handleDetailClose"
      @opened="handleDetailOpened"
      @closed="handleDetailClosed">
      <div class="host-detail-panel">
        <section class="detail-surface-panel">
          <div class="surface-panel-head">
            <div>
              <div class="panel-eyebrow">HOST SNAPSHOT</div>
              <h3>基本信息</h3>
            </div>
          </div>
          <div class="detail-table-shell">
            <table class="detail-data-table">
              <tbody>
                <tr>
                  <th>主机名称</th>
                  <td>{{ hostDetail.hostName || '-' }}</td>
                  <th>主机分组</th>
                  <td>{{ getGroupName(hostDetail.groupId) }}</td>
                </tr>
                <tr>
                  <th>连接地址</th>
                  <td class="is-mono">{{ getConnectionAddress(hostDetail) }}</td>
                  <th>认证类型</th>
                  <td>
                    <el-tag :type="getAuthTypeTagType(hostDetail.sshKeyId)" effect="plain" size="small">
                      {{ getAuthTypeName(hostDetail.sshKeyId) }}
                    </el-tag>
                  </td>
                </tr>
                <tr>
                  <th>认证状态</th>
                  <td>
                    <el-tag :type="getStatusTagType(hostDetail.status)" effect="plain" size="small">
                      {{ getStatusText(hostDetail.status) }}
                    </el-tag>
                  </td>
                  <th>备注</th>
                  <td>{{ hostDetail.remark || '-' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="host-detail-hero">
          <div class="host-detail-title">
            <div>
              <div class="panel-eyebrow">LIVE METRICS</div>
              <div class="host-detail-name">运行概览</div>
            </div>
            <div class="host-detail-badges">
              <el-tag :type="getStatusTagType(hostDetail.status)" effect="plain" size="small">
                {{ getStatusText(hostDetail.status) }}
              </el-tag>
              <el-tag type="info" effect="plain" size="small">
                {{ getVendorName(hostDetail.vendor) }}
              </el-tag>
            </div>
          </div>

          <div class="metric-card-grid">
            <article
              v-for="metric in hostMetricCards"
              :key="metric.key"
              class="metric-card"
              :class="`is-${metric.tone}`"
            >
              <div class="metric-ring">
                <el-progress
                  type="circle"
                  :percentage="metric.percent"
                  :width="66"
                  :stroke-width="6"
                  :color="metric.color"
                  :show-text="false"
                />
                <strong>{{ metric.value }}</strong>
              </div>
              <div class="metric-card-copy">
                <div class="metric-card-label">
                  <i :style="{ backgroundColor: metric.color }"></i>
                  {{ metric.label }}
                </div>
                <span>{{ metric.helper }}</span>
                <small v-if="metric.detail">{{ metric.detail }}</small>
              </div>
            </article>
          </div>
        </section>

        <section class="detail-surface-panel host-trend-panel">
          <div class="surface-panel-head">
            <div>
              <div class="panel-eyebrow">PERFORMANCE TREND</div>
              <h3>运行趋势</h3>
            </div>
            <div class="trend-panel-actions">
              <span>最近 30 分钟</span>
              <el-tooltip content="刷新监控数据" placement="top">
                <el-button
                  circle
                  size="small"
                  :loading="hostMetricsLoading"
                  :disabled="!hostDetail.id"
                  aria-label="刷新监控数据"
                  @click="fetchHostMonitorData(hostDetail.id)"
                >
                  <el-icon v-if="!hostMetricsLoading"><Refresh /></el-icon>
                </el-button>
              </el-tooltip>
            </div>
          </div>

          <div v-if="hostMetricsLoading" class="trend-loading-state">
            <el-skeleton :rows="4" animated />
          </div>
          <div v-else-if="hostMetricsError" class="trend-feedback-state is-error">
            <el-icon><WarningFilled /></el-icon>
            <div>
              <strong>监控数据加载失败</strong>
              <span>{{ hostMetricsError }}</span>
            </div>
            <el-button size="small" @click="fetchHostMonitorData(hostDetail.id)">重试</el-button>
          </div>
          <el-empty
            v-else-if="!hasHostTrendData"
            class="trend-empty-state"
            :image-size="54"
            description="最近 30 分钟暂无监控数据"
          />
          <div v-else ref="hostTrendGrid" class="host-trend-grid">
            <article class="trend-chart-card">
              <div class="trend-chart-head">
                <div>
                  <span>CPU 总使用率</span>
                  <strong>{{ formatMetricPercent(latestHostMetrics.cpu) }}</strong>
                </div>
                <i class="trend-legend-dot is-cpu"></i>
              </div>
              <div ref="cpuTrendChart" class="trend-chart"></div>
            </article>
            <article class="trend-chart-card">
              <div class="trend-chart-head">
                <div>
                  <span>系统负载</span>
                  <strong>{{ formatMetricLoad(latestHostMetrics.load) }}</strong>
                </div>
                <div class="load-legend">
                  <span><i class="is-load1"></i>1m</span>
                  <span><i class="is-load5"></i>5m</span>
                  <span><i class="is-load15"></i>15m</span>
                </div>
              </div>
              <div ref="loadTrendChart" class="trend-chart"></div>
            </article>
            <article class="trend-chart-card">
              <div class="trend-chart-head">
                <div>
                  <span>内存使用率</span>
                  <strong>{{ formatMetricPercent(latestHostMetrics.memory) }}</strong>
                </div>
                <i class="trend-legend-dot is-memory"></i>
              </div>
              <div ref="memoryTrendChart" class="trend-chart"></div>
            </article>
          </div>
        </section>

        <section class="detail-surface-panel">
          <div class="surface-panel-head">
            <div>
              <div class="panel-eyebrow">RESOURCE PROFILE</div>
              <h3>扩展信息</h3>
            </div>
            <el-button
              type="primary"
              size="small"
              icon="Refresh"
              :loading="syncLoading"
              v-authority="['cmdb:ecs:rsync']"
              @click="handleHostSync"
            >
              {{ syncLoading ? '同步中' : '同步' }}
            </el-button>
          </div>
          <div class="detail-table-shell">
            <table class="detail-data-table">
              <tbody>
                <tr>
                  <th>实例ID</th>
                  <td class="is-mono">{{ hostDetail.instanceId || '-' }}</td>
                  <th>实例名称</th>
                  <td>{{ hostDetail.name || '-' }}</td>
                </tr>
                <tr>
                  <th>操作系统</th>
                  <td>{{ hostDetail.os || '-' }}</td>
                  <th>配置</th>
                  <td>{{ formatHostSpec(hostDetail) }}</td>
                </tr>
                <tr>
                  <th>内网IP</th>
                  <td class="is-mono">{{ hostDetail.privateIp || '-' }}</td>
                  <th>公网IP</th>
                  <td class="is-mono">{{ hostDetail.publicIp || '-' }}</td>
                </tr>
                <tr>
                  <th>实例计费</th>
                  <td>{{ hostDetail.billingType || '-' }}</td>
                  <th>网络计费</th>
                  <td>{{ hostDetail.networkBillingType || '按流量计费' }}</td>
                </tr>
                <tr>
                  <th>创建时间</th>
                  <td>{{ hostDetail.createTime || '-' }}</td>
                  <th>到期时间</th>
                  <td>{{ hostDetail.expireTime || '-' }}</td>
                </tr>
                <tr>
                  <th>更新时间</th>
                  <td colspan="3">{{ hostDetail.updateTime || '-' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </el-drawer>

    <!-- 文件管理组件 -->
    <FileManager
      ref="fileManager"
      @upload-success="handleFileOperationSuccess"
      @download-success="handleFileOperationSuccess"
    />
  </el-card>
  </div>
</template>

<script>
import configApi from '@/api/config'
import cmdbApi from '@/api/cmdb'
import * as echarts from 'echarts'
import CreateCloud from './Host/CreateCloud.vue'
import HostSSH from './Host/SSH.vue'
import CreateExcel from './Host/CreateExcel.vue'
import CreateJumpServer from './Host/CreateJumpServer.vue'
import CmdbGroup from './Host/CmdbGroup.vue'
import CmdbHostTable from './Host/CmdbHostTable.vue'
import CreateHost from './Host/CreateHost.vue'
import EditHost from './Host/EditHost.vue'
import FileManager from './Host/FileManager.vue'

export default {
  components: {
    CreateCloud,
    HostSSH,
    CreateExcel,
    CreateJumpServer,
    CmdbGroup,
    CmdbHostTable,
    CreateHost,
    EditHost,
    FileManager
  },
  data() {
    return {
      ExcelDialogVisible: false,
      jumpServerDialogVisible: false,
      expandedKeys: [], // 用于跟踪展开的节点key
      // 批量操作相关
      selectedHosts: [],
      statusList: [
        { value: 2, label: '未认证' },
        { value: 1, label: '认证成功' },
        { value: 3, label: '认证失败' }
      ],
      loading: false,
      exportLoading: false,
      queryParams: {
        pageNum: 1,
        pageSize: 10,
        keyword: '',
        status: '',
        vendor: '',
        hostType: '',
        groupId: ''
      },
      hostList: [],
      total: 0,
      hostStatistics: null, // 主机统计数据
      windowsHostCount: 0,
      addDialogVisible: false,
      editDialogVisible: false,
      cloudDialogVisible: false,
      groupList: [],
      defaultProps: {
        children: 'children',
        label: 'name'
      },
      currentGroupId: null,
      authList: [],
      addForm: {
        hostName: '',
        ip: '',
        port: 22,
        username: '',
        authId: '',
        groupId: '',
        remark: ''
      },
      addFormRules: {
        hostName: [{ required: true, message: '请输入主机名称', trigger: 'blur' }],
        ip: [{ required: true, message: '请输入IP地址', trigger: 'blur' }],
        port: [{ required: true, message: '请输入端口号', trigger: 'blur' }],
        username: [{ required: true, message: '请输入连接用户名', trigger: 'blur' }],
        authId: [{ required: true, message: '请选择认证凭据', trigger: 'change' }],
        groupId: [{ required: true, message: '请选择所属分组', trigger: 'change' }]
      },
      hostInfo: {},
      editFormRules: {
        hostName: [{ required: true, message: '请输入主机名称', trigger: 'blur' }],
        ip: [{ required: true, message: '请输入IP地址', trigger: 'blur' }],
        port: [{ required: true, message: '请输入端口号', trigger: 'blur' }],
        username: [{ required: true, message: '请输入连接用户名', trigger: 'blur' }],
        authId: [{ required: true, message: '请选择认证凭据', trigger: 'change' }],
        groupId: [{ required: true, message: '请选择所属分组', trigger: 'change' }]
      },
    // SSH终端对话框相关
    sshDialogVisible: false,
    currentHostId: null,

    // 主机详情相关
    detailDrawer: false,
    syncLoading: false, // 同步加载状态
    hostMetricsLoading: false,
    hostMetricsError: '',
    hostMetricsData: {},
    hostCurrentMetrics: {},
    hostDetail: {
        hostName: '',
        groupId: '',
        privateIp: '',
        publicIp: '',
        sshIp: '',
        sshName: '',
        sshKeyId: '',
        sshKeyIds: [],
        sshAuths: [],
        sshPort: 22,
        remark: '',
        vendor: '',
        region: '',
        instanceId: '',
        os: '',
        status: 0,
        cpu: '',
        memory: '',
        disk: '',
        billingType: '',
        createTime: '',
        expireTime: '',
        updateTime: '',
        name: '',
        cpuUsage: 0,
        memoryUsage: 0,
        diskUsage: 0
      }
    }
  },
  computed: {
    // 是否有选中的主机
    hasSelection() {
      return this.selectedHosts.length > 0
    },
    // 选中的主机数量
    selectionCount() {
      return this.selectedHosts.length
    },
    hostMetricCards() {
      const createMetric = (key, label, rawValue, baseColor, helper = '当前使用率') => {
        const percent = this.normalizeMetricPercent(rawValue)
        const tone = this.getMetricTone(percent)
        return {
          key,
          label,
          value: this.hostMetricsLoading ? '--' : this.formatMetricPercent(percent),
          percent,
          color: tone === 'normal' ? baseColor : this.getMetricProgressColor(percent),
          tone,
          helper,
          detail: ''
        }
      }

      const loadValue = this.latestHostMetrics.load
      const loadPercent = this.getLoadPercent(loadValue)

      return [
        createMetric('cpu', 'CPU 总使用率', this.latestHostMetrics.cpu, '#4c8bf5'),
        createMetric('memory', '内存', this.hostDetail.memoryUsage, '#22a49a'),
        {
          key: 'load',
          label: '系统负载',
          value: this.hostMetricsLoading ? '--' : this.formatMetricPercent(loadPercent),
          percent: loadPercent,
          color: this.getMetricTone(loadPercent) === 'normal'
            ? '#7c6ee6'
            : this.getMetricProgressColor(loadPercent),
          tone: this.getMetricTone(loadPercent),
          helper: '按 CPU 核数折算',
          detail: `5 分钟负载 ${this.hostMetricsLoading ? '--' : this.formatMetricLoad(loadValue)}`
        },
        createMetric('disk', '磁盘', this.hostDetail.diskUsage, '#55a57a')
      ]
    },
    latestHostMetrics() {
      return {
        cpu: this.getCurrentCpuValue(),
        memory: this.getCurrentMetricValue('memoryUsage', 'memory'),
        disk: this.getCurrentMetricValue('diskUsage', 'disk'),
        load: this.getCurrentMetricValue('load5', 'load5min'),
        load1min: this.getLatestMetricValue('load1min'),
        load5min: this.getLatestMetricValue('load5min'),
        load15min: this.getLatestMetricValue('load15min')
      }
    },
    hasHostTrendData() {
      return ['cpu', 'memory', 'load1min', 'load5min', 'load15min']
        .some(key => this.getMetricSeries(key).length > 0)
    }
  },
  created() {
    this._hostChartInstances = Object.create(null)
    this._hostChartResizeObserver = null
    this._hostMetricsRequestId = 0
    this.getAllGroups()
    this.getAuthList()
    // 默认加载所有主机
    this.getHostList()
    // 加载主机统计信息
    this.fetchHostStatistics()
  },

  beforeUnmount() {
    this._hostMetricsRequestId += 1
    this.disposeHostCharts()
  },

    beforeRouteEnter(to, from, next) {
      next(vm => {
        // 立即获取主机列表
        vm.getHostList().then(() => {
          // 主机列表加载完成后立即触发监控数据加载
          vm.$refs.hostTable?.fetchMonitorData()
        })
      })
    },

    beforeRouteUpdate(to, from, next) {
      // 立即获取主机列表
      this.getHostList().then(() => {
        // 主机列表加载完成后立即触发监控数据加载
        this.$refs.hostTable?.fetchMonitorData()
        next()
      })
    },
  methods: {
    getGroupTimestamp(group) {
      const value = group?.updateTime
        || group?.update_time
        || group?.modifyTime
        || group?.modify_time
        || group?.updatedAt
        || group?.updated_at
        || group?.createTime
        || group?.create_time
        || group?.createdAt
        || group?.created_at

      if (!value) return 0

      const timestamp = typeof value === 'number' ? value : Date.parse(value)
      return Number.isFinite(timestamp) ? timestamp : 0
    },
    isDefaultRootGroup(group) {
      return group?.isDefault === true ||
        Number(group?.id) === 1 ||
        String(group?.name || '').trim() === '默认业务组'
    },
    sortGroupTree(groups, isRootLevel = true) {
      if (!Array.isArray(groups)) {
        return []
      }

      return [...groups]
        .map(group => ({
          ...group,
          children: this.sortGroupTree(group.children, false)
        }))
        .sort((a, b) => {
          if (isRootLevel) {
            const defaultDiff = Number(this.isDefaultRootGroup(b)) - Number(this.isDefaultRootGroup(a))
            if (defaultDiff !== 0) return defaultDiff
          }

          const timeDiff = this.getGroupTimestamp(b) - this.getGroupTimestamp(a)
          if (timeDiff !== 0) return timeDiff

          const idDiff = Number(b?.id || 0) - Number(a?.id || 0)
          if (idDiff !== 0) return idDiff

          return String(a?.name || '').localeCompare(String(b?.name || ''), 'zh-Hans-CN')
        })
    },
    // 获取所有分组
    async getAllGroups() {
      const { data: res } = await this.$api.getAllCmdbGroups()
      if (res.code === 200) {
        this.groupList = this.sortGroupTree(res.data)
        // 设置默认分组为业务组
        const businessGroup = this.groupList.find(group => group.name === '业务组')
        if (businessGroup) {
          this.addForm.groupId = businessGroup.id
        }
      }
    },

    // 获取主机统计信息
    async fetchHostStatistics() {
      try {
        const [statisticsResponse, windowsResponse] = await Promise.all([
          this.$api.getHostCloudStatistics(),
          this.$api.GetCmdbHostsByHostType(2, { page: 1, pageSize: 1 })
        ])

        const statisticsResult = statisticsResponse?.data
        if (statisticsResult?.code === 200) {
          this.hostStatistics = statisticsResult.data
        }

        const windowsResult = windowsResponse?.data
        if (windowsResult?.code === 200) {
          this.windowsHostCount = Number(windowsResult.data?.total) || 0
        }
      } catch (error) {
        console.error('获取主机统计信息失败:', error)
      }
    },

    // 处理分组搜索
    async handleGroupSearch(searchText) {
      this.groupSearchText = searchText
      if (!this.groupSearchText) {
        this.expandedKeys = []
        return
      }
      
      try {
        const { data: res } = await this.$api.getCmdbGroupByName(this.groupSearchText)
        
        if (res.code === 200 && res.data) {
          
          // 获取CmdbGroup组件的树引用
          const cmdbGroupRef = this.$refs.cmdbGroup
          const tree = cmdbGroupRef ? cmdbGroupRef.$refs.groupTree : null
          if (!tree) {
            console.error('树组件引用不存在')
            return
          }



          // 找到匹配的分组并展开其父级
          const findAndExpandParent = (groups, targetId, path = []) => {
            for (const group of groups) {

              if (group.id === targetId) {

                return [...path, group.id]
              }
              if (group.children && group.children.length > 0) {
                const foundPath = findAndExpandParent(group.children, targetId, [...path, group.id])
                if (foundPath) {
                  return foundPath
                }
              }
            }
            return null
          }
          
          // 获取展开路径
          const expandPath = findAndExpandParent(this.groupList, res.data.id)
          
          if (expandPath) {
            // 设置展开的keys
            this.expandedKeys = expandPath.slice(0, -1)
            
            // 强制更新树组件
            this.$nextTick(() => {
              tree.setCurrentKey(res.data.id)
              
              // 确保树组件已更新
              setTimeout(() => {

              }, 500)
            })
          } else {
            console.warn('未找到匹配分组的路径')
            this.$message.warning('未找到匹配的分组路径')
          }
        } else {
          console.warn('未找到匹配分组')
        }
      } catch (error) {
        console.error('搜索分组失败:', error)
        this.$message.error('搜索分组失败: ' + (error?.message || '未知错误'))
      }
    },

    // 获取认证凭据列表
    async getAuthList() {
      try {
        const response = await configApi.getEcsAuthList({
          page: 1,
          pageSize: 100  // 获取认证凭据，用于下拉选择
        })
        
        if (response && response.data) {
          const res = response.data
          
          if (res.code === 200) {
            this.authList = Array.isArray(res.data?.list) ? res.data.list : []
            if (this.authList.length === 0) {
              console.warn('认证凭据列表为空')
            }
          } else {
            console.error('获取认证凭据失败:', res.message || '未知错误')
            this.$message.error(`获取认证凭据失败: ${res.message || '未知错误'}`)
          }
        } else {
          console.error('无效的响应格式:', response)
          this.$message.error('获取认证凭据失败: 无效的响应格式')
        }
      } catch (error) {
        console.error('获取认证凭据异常:', error)
        this.$message.error(`获取认证凭据异常: ${error?.message || '未知错误'}`)
        // 临时添加模拟数据用于测试
        this.authList = [
          { id: 1, name: '默认凭据', username: 'root' },
          { id: 2, name: '测试凭据', username: 'test' }
        ]
        console.warn('使用模拟凭据数据:', this.authList)
      }
    },
    
    // 获取主机列表
    async getHostList() {
      this.loading = true
      try {
        let response
        const { keyword: rawKeyword, status, vendor, hostType, pageNum, pageSize } = this.queryParams
        const keyword = typeof rawKeyword === 'string' ? rawKeyword.trim() : ''

        // 构建分页参数
        const baseParams = {
          page: pageNum,
          pageSize: pageSize,
          _t: Date.now() // 添加时间戳防止缓存
        }

        // 优先级：分组查询 > 条件查询 > 全部查询
        // 如果当前有选中的分组，使用分组查询
        if (this.currentGroupId) {
          response = await this.$api.getCmdbHostsByGroupId(this.currentGroupId, baseParams)
        }
        // 根据查询条件选择API调用
        else if (keyword) {
          response = await this.$api.GetCmdbHostsByKeyword(keyword, {
            ...baseParams,
            ...(status ? { status } : {})
          })
        } else if (vendor) {
          response = await this.$api.GetCmdbHostsByVendor(vendor, {
            ...baseParams,
            ...(status ? { status } : {})
          })
        } else if (hostType) {
          response = await this.$api.GetCmdbHostsByHostType(hostType, {
            ...baseParams,
            ...(status ? { status } : {})
          })
        } else if (status) {
          response = await this.$api.GetCmdbHostsByStatus(status, baseParams)
        } else {
          response = await this.$api.getCmdbHostList(baseParams)
        }


        // 处理axios响应结构
        const axiosResponse = response?.data ? response : { data: response }

        // 严格检查响应格式
        if (!axiosResponse || typeof axiosResponse !== 'object') {
          throw new Error('API返回无效响应格式')
        }

        // 检查响应数据
        const res = axiosResponse.data
        if (!res || typeof res !== 'object') {
          throw new Error('无效的响应数据结构')
        }

        // 检查响应码
        if (res.code === undefined || res.code !== 200) {
          throw new Error(res.message || '获取主机列表失败')
        }

        // 确保data存在，即使为空数组
        if (res.data === undefined) {
          throw new Error('响应缺少data字段')
        }

        // 处理响应数据 - 适配不同API返回格式
        if (Array.isArray(res.data)) {
          // 直接返回数组的情况（如GetCmdbHostsByIP）
          this.hostList = res.data
          this.total = res.data.length
        } else if (res.data?.list) {
          // 返回分页格式的情况（如getCmdbHostList）
          this.hostList = res.data.list
          this.total = res.data.total
          if (res.data.page) {
            this.queryParams.pageNum = res.data.page
          }
          if (res.data.pageSize) {
            this.queryParams.pageSize = res.data.pageSize
          }
        } else {
          // 其他情况
          this.hostList = []
          this.total = 0
        }

        // 主机列表加载完成后立即触发监控数据加载
        this.$nextTick(() => {
          this.$refs.hostTable?.fetchMonitorData()
        })

      } catch (error) {
        console.error('获取主机列表异常:', {
          error: error?.message || '未知错误',
          stack: error?.stack || '无堆栈信息',
          queryParams: this.queryParams,
          currentGroupId: this.currentGroupId
        })
        this.$message.error(`获取主机列表失败: ${error?.message || '未知错误'}`)
        this.hostList = []
        this.total = 0
      } finally {
        this.loading = false
      }
    },
    
    // 处理分组选择变化
    handleGroupChange(value) {
      if (value && value.length > 0) {
        // 取最后一级作为选中分组ID
        this.addForm.groupId = value[value.length - 1]
        this.hostInfo.groupId = value[value.length - 1]
      } else {
        // 如果没有选择分组，设置默认分组
        const defaultGroup = this.groupList.find(item => item.isDefault)
        if (defaultGroup) {
          this.addForm.groupId = defaultGroup.id
          this.hostInfo.groupId = defaultGroup.id
        }
      }
    },

    // 根据分组获取主机
    async getHostsByGroup(groupId) {
      this.loading = true
      this.queryParams.groupId = groupId
      try {
        const { data: res } = await this.$api.getCmdbHostsByGroupId(groupId, {
          page: this.queryParams.pageNum,
          pageSize: this.queryParams.pageSize
        })
        if (res.code === 200) {
          // 处理响应数据 - 适配分页格式
          if (Array.isArray(res.data)) {
            // 直接返回数组的情况（旧格式）
            this.hostList = res.data
            this.total = res.data.length
          } else if (res.data?.list) {
            // 返回分页格式的情况（新格式）
            this.hostList = res.data.list
            this.total = res.data.total
            if (res.data.page) {
              this.queryParams.pageNum = res.data.page
            }
            if (res.data.pageSize) {
              this.queryParams.pageSize = res.data.pageSize
            }
          } else {
            // 其他情况
            this.hostList = []
            this.total = 0
          }

          // 主机列表加载完成后立即触发监控数据加载
          this.$nextTick(() => {
            this.$refs.hostTable?.fetchMonitorData()
          })
        }
      } catch (error) {
        console.error('获取主机列表失败:', error)
        this.hostList = []
        this.total = 0
      } finally {
        this.loading = false
      }
    },
    
    // 点击分组节点
    handleGroupClick(node, element) {
      let groupId
      if (element && element.data && element.data.id) {
        groupId = element.data.id
      } else if (element && element.id) {
        groupId = element.id
      } else if (node && node.key) {
        groupId = node.key
      }

      if (!groupId) {
        this.$message.warning("无法获取分组ID")
        return
      }

      this.currentGroupId = groupId
      this.queryParams.vendor = ''
      this.queryParams.hostType = ''
      // 重置到第一页
      this.queryParams.pageNum = 1
      this.getHostsByGroup(groupId)
    },

    handleNodeExpand(data, node) {
      if (!this.expandedKeys.includes(node.key)) {
        this.expandedKeys.push(node.key)
      }
    },

    handleNodeCollapse(data, node) {
      this.expandedKeys = this.expandedKeys.filter(key => key !== node.key)
    },

    // 折叠所有节点
    handleCollapseAll() {
      this.expandedKeys = []
    },

    // 展开所有节点
    handleExpandAll() {
      const allKeys = []
      const collectKeys = (nodes) => {
        nodes.forEach(node => {
          allKeys.push(node.id)
          if (node.children && node.children.length > 0) {
            collectKeys(node.children)
          }
        })
      }
      collectKeys(this.groupList)
      this.expandedKeys = allKeys
    },

    // 刷新分组列表
    async handleRefreshGroups() {
      try {
        await this.getAllGroups()
        this.$message.success('刷新成功')
      } catch (error) {
        console.error('刷新分组失败:', error)
        this.$message.error('刷新分组失败')
      }
    },
    
    // 搜索按钮操作
    handleQuery() {
      this.queryParams.pageNum = 1
      this.queryParams.vendor = ''
      this.queryParams.hostType = ''
      // 搜索时清除分组选择，使用搜索条件
      this.currentGroupId = null
      this.getHostList()
    },

    getVendorName(vendor) {
      const vendorMap = {
        1: '自建',
        2: '阿里云',
        3: '腾讯云',
        4: '百度云',
        5: '华为云',
        6: 'AWS',
        7: '天翼云'
      }
      return vendorMap[Number(vendor)] || '未知类型'
    },

    getStatisticName(detail) {
      return Number(detail?.vendor) === 6 ? 'Windows' : this.getVendorName(detail?.vendor)
    },

    getStatisticCount(detail) {
      return Number(detail?.vendor) === 6 ? this.windowsHostCount : detail?.count
    },

    isStatisticActive(detail) {
      const vendor = Number(detail?.vendor)
      if (vendor === 6) {
        return Number(this.queryParams.hostType) === 2
      }
      return Number(this.queryParams.vendor) === vendor
    },

    handleVendorStatisticClick(detail) {
      const vendor = Number(detail?.vendor)
      if (!vendor) return
      this.queryParams.pageNum = 1
      this.queryParams.keyword = ''
      this.queryParams.vendor = vendor === 6 ? '' : vendor
      this.queryParams.hostType = vendor === 6 ? 2 : ''
      this.currentGroupId = null
      this.getHostList()
    },
    
    // 重置按钮操作
    resetQuery() {
      this.queryParams = {
        pageNum: 1,
        pageSize: 10,
        keyword: '',
        status: '',
        vendor: '',
        hostType: '',
        groupId: ''
      }
      this.currentGroupId = null
      this.getHostList()
    },
    
    // pageSize变化
    handleSizeChange(newSize) {
      this.queryParams.pageSize = newSize
      this.getHostList()
    },
    
    // pageNum变化
    handleCurrentChange(newPage) {
      this.queryParams.pageNum = newPage
      this.getHostList()
    },
    buildExportParams() {
      const { keyword: rawKeyword, status } = this.queryParams
      const keyword = typeof rawKeyword === 'string' ? rawKeyword.trim() : ''
      const params = {
        sortBy: 'id',
        sortOrder: 'desc'
      }

      if (keyword) {
        params.keyword = keyword
        params.name = keyword
        params.ip = keyword
      }

      if (status !== '' && status !== null && status !== undefined) {
        params.status = Array.isArray(status) ? status.join(',') : status
      }

      if (this.currentGroupId) {
        params.groupId = this.currentGroupId
      } else if (this.queryParams.groupId) {
        params.groupId = this.queryParams.groupId
      }

      return params
    },
    getDownloadFilename(response, fallback) {
      const disposition = response?.headers?.['content-disposition'] || response?.headers?.['Content-Disposition'] || ''
      const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i)
      if (utf8Match?.[1]) {
        return decodeURIComponent(utf8Match[1])
      }

      const match = disposition.match(/filename="?([^";]+)"?/i)
      if (match?.[1]) {
        return decodeURIComponent(match[1])
      }

      return fallback
    },
    downloadBlob(blob, filename) {
      const url = window.URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = filename
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      window.URL.revokeObjectURL(url)
    },
    async handleExportHosts() {
      this.exportLoading = true
      try {
        const response = await this.$api.exportCmdbHosts(this.buildExportParams())
        const filename = this.getDownloadFilename(response, `cmdb_hosts_${Date.now()}.csv`)
        this.downloadBlob(response.data, filename)
        this.$message.success('主机 CSV 导出成功')
      } catch (error) {
        console.error('导出主机 CSV 失败:', error)
        this.$message.error('导出主机 CSV 失败')
      } finally {
        this.exportLoading = false
      }
    },

    // 新增主机
    async addHost(requestData) {
      try {
        const { data: res } = await this.$api.createCmdbHost(requestData)

        if (res.code === 200) {
          this.$message.success('新增主机成功，正在同步主机信息...')
          this.addDialogVisible = false

          // 立即刷新一次列表显示新创建的主机
          await this.getHostList()

          // 等待3秒让后端同步主机状态信息，然后再次刷新
          setTimeout(async () => {
            await this.getHostList()
            this.$message.success('主机信息同步完成')
          }, 3000)
        } else if (res.code === 426) {
          const authIdText = requestData.sshKeyId ? `(凭据ID: ${requestData.sshKeyId})` : ''
          this.$message.error(`认证凭据不存在${authIdText}，请检查后重试`)
          // 刷新凭据列表
          await this.getAuthList()
        } else {
          this.$message.error(res.message || '新增主机失败')
        }
      } catch (error) {
        console.error('新增主机失败:', error)
        this.$message.error('新增主机失败: ' + error.message)
      }
    },
    
    // 展示编辑主机对话框
    async showEditHostDialog(id) {
      const { data: res } = await this.$api.getCmdbHostById(id)
      if (res.code === 200) {
        const hostType = Number(res.data.hostType) === 2 ? 2 : 1
        this.hostInfo = {
          id: res.data.id,
          hostName: res.data.hostName,
          groupId: res.data.groupId,
          remark: res.data.remark,
          hostType,
          sshIp: res.data.sshIp,
          privateIp: res.data.privateIp,
          sshPort: res.data.sshPort,
          sshName: res.data.sshName,
          sshKeyId: res.data.sshKeyId,
          sshKeyIds: res.data.sshKeyIds || [],
          sshAuths: res.data.sshAuths || [],
          os: res.data.os,
          cpu: res.data.cpu,
          memory: res.data.memory,
          disk: res.data.disk,
          vendor: res.data.vendor,
          tag: res.data.tag
        }
        this.editDialogVisible = true
      }
    },
    
    // 监听编辑主机对话框关闭
    editDialogClosed() {
      this.$refs.editFormRef.resetFields()
    },
    
    // 编辑主机信息
    async editHost(requestData) {
      try {
        if (requestData.sshPort !== undefined && requestData.sshPort !== null && requestData.sshPort !== '') {
          const port = Number(requestData.sshPort)
          if (isNaN(port) || port < 1 || port > 65535) {
            this.$message.error('端口号必须为1-65535之间的数字')
            return false
          }
        }

        const { data: res } = await this.$api.updateCmdbHost(requestData)
        if (res.code === 200) {
          this.$message.success('修改主机成功')
          this.editDialogVisible = false
          this.getHostList()
          return true
        } else if (res.code === 426) {
          const authIdText = requestData.sshKeyId ? `(凭据ID: ${requestData.sshKeyId})` : ''
          this.$message.error(`认证凭据不存在${authIdText}，请检查后重试`)
          // 刷新凭据列表
          await this.getAuthList()
          return false
        } else {
          this.$message.error(res.message || '修改主机失败')
          return false
        }
      } catch (error) {
        console.error('修改主机失败:', error)
        this.$message.error('修改主机失败: ' + error.message)
        return false
      }
    },
    
    // 获取状态文本

    // 根据分组ID获取分组名称
    getGroupName(groupId) {
      if (!groupId) return '未分组'
      const findGroup = (groups, id) => {
        for (const group of groups) {
          if (group.id === id) return group.name
          if (group.children && group.children.length > 0) {
            const found = findGroup(group.children, id)
            if (found) return found
          }
        }
        return null
      }
      return findGroup(this.groupList, groupId) || '未知分组'
    },

    findHostAuthById(authId) {
      if (!authId) return null

      const authIdStr = String(authId)
      const detailAuths = Array.isArray(this.hostDetail?.sshAuths) ? this.hostDetail.sshAuths : []
      const allAuths = [...detailAuths, ...this.authList]

      return allAuths.find(item => {
        const itemId = item?.id ?? item?.authId
        const itemAuthId = item?.authId ?? item?.id
        return String(itemId) === authIdStr || String(itemAuthId) === authIdStr
      }) || null
    },

    // 根据认证凭据ID获取认证类型名称
    getAuthTypeName(authId) {
      if (!authId) return '未设置'
      const auth = this.findHostAuthById(authId)
      if (!auth) return '未知类型'

      switch (Number(auth.type)) {
        case 1:
          return '密码认证'
        case 2:
          return '密钥认证'
        case 3:
          return '公钥认证'
        case 4:
          return '跳板机认证'
        default:
          return '未知类型'
      }
    },

    // 获取认证类型标签颜色
    getAuthTypeTagType(authId) {
      if (!authId) return 'info'
      const auth = this.findHostAuthById(authId)
      if (!auth) return 'warning'

      switch (Number(auth.type)) {
        case 1:
          return 'primary'  // 密码认证 - 蓝色
        case 2:
          return 'success'  // 密钥认证 - 绿色
        case 3:
          return 'warning'  // 公钥认证 - 橙色
        case 4:
          return 'danger'   // 堡垒机认证 - 红色
        default:
          return 'info'
      }
    },

    // 获取认证状态文本
    getStatusText(status) {
      const statusMap = {
        1: '认证成功',
        2: '未认证',
        3: '认证失败'
      }
      return statusMap[status] || '未知状态'
    },

    // 获取认证状态标签类型
    getStatusTagType(status) {
      const typeMap = {
        1: 'success',
        2: 'warning',
        3: 'danger'
      }
      return typeMap[status] || 'info'
    },

    // 显示主机详情
    async showHostDetail(row) {
      try {
        const { data: res } = await this.$api.getCmdbHostById(row.id)

        if (res.code === 200) {
          this.hostDetail = res.data
          this.hostMetricsData = {}
          this.hostCurrentMetrics = {}
          this.hostMetricsError = ''
          this.detailDrawer = true

          this.fetchHostMonitorData(row.id)

        } else {
          console.error('获取主机详情失败:', res.message)
          this.$message.error(res.message || '获取主机详情失败')
        }
      } catch (error) {
        console.error('获取主机详情失败:', error)
        this.$message.error('获取主机详情失败: ' + error.message)
      }
    },

    // 获取主机监控数据
    async fetchHostMonitorData(hostId) {
      if (!hostId) return

      const requestId = ++this._hostMetricsRequestId
      this.hostMetricsLoading = true
      this.hostMetricsError = ''

      try {
        const [historyResult, currentResult] = await Promise.allSettled([
          cmdbApi.getHostMonitorData({ id: hostId, duration: '30m' }),
          cmdbApi.getHostsMonitorData(hostId)
        ])
        if (requestId !== this._hostMetricsRequestId) return

        const historyResponse = historyResult.status === 'fulfilled'
          ? historyResult.value?.data
          : null
        if (historyResponse?.code === 200) {
          this.hostMetricsData = historyResponse.data && typeof historyResponse.data === 'object'
            ? historyResponse.data
            : {}
        } else {
          this.hostMetricsData = {}
          this.hostMetricsError = historyResult.status === 'rejected'
            ? (historyResult.reason?.response?.data?.message || historyResult.reason?.message || '请稍后重试')
            : (historyResponse?.message || '历史监控接口返回异常')
        }

        const currentResponse = currentResult.status === 'fulfilled'
          ? currentResult.value?.data
          : null
        const currentMetrics = currentResponse?.code === 200
          ? (currentResponse.data?.[hostId] || currentResponse.data?.[String(hostId)])
          : null

        this.hostCurrentMetrics = currentMetrics && typeof currentMetrics === 'object'
          ? currentMetrics
          : {}

        if (!currentMetrics && currentResult.status === 'rejected') {
          console.warn('获取主机实时监控摘要失败，将使用历史数据兜底:', currentResult.reason)
        }

        this.hostDetail = {
          ...this.hostDetail,
          cpuUsage: this.getCurrentCpuValue(),
          memoryUsage: this.getCurrentMetricValue('memoryUsage', 'memory'),
          diskUsage: this.getCurrentMetricValue('diskUsage', 'disk')
        }
      } catch (error) {
        if (requestId !== this._hostMetricsRequestId) return
        console.error('获取主机监控数据失败:', error)
        this.hostMetricsData = {}
        this.hostCurrentMetrics = {}
        this.hostMetricsError = error?.response?.data?.message || error?.message || '请稍后重试'
      } finally {
        if (requestId === this._hostMetricsRequestId) {
          this.hostMetricsLoading = false
          await this.$nextTick()
          this.renderHostTrendCharts()
        }
      }
    },

    // 关闭详情抽屉
    handleDetailClose(done) {
      if (typeof done === 'function') {
        done()
      } else {
        this.detailDrawer = false
      }
    },

    handleDetailOpened() {
      this.setupHostChartResizeObserver()
      this.renderHostTrendCharts()
    },

    handleDetailClosed() {
      this._hostMetricsRequestId += 1
      this.hostMetricsLoading = false
      this.disposeHostCharts()
    },

    getRawMetricSeries(key) {
      const points = this.hostMetricsData?.[key]
      if (!Array.isArray(points)) return []

      return points
        .map(point => {
          const rawTimestamp = Array.isArray(point) ? point[0] : point?.timestamp
          const rawValue = Array.isArray(point) ? point[1] : point?.value
          const numericTimestamp = Number(rawTimestamp)
          const timestamp = Number.isFinite(numericTimestamp)
            ? (numericTimestamp < 1000000000000 ? numericTimestamp * 1000 : numericTimestamp)
            : Date.parse(rawTimestamp)
          const value = Number(rawValue)

          if (!Number.isFinite(timestamp) || !Number.isFinite(value)) return null
          return [timestamp, Number(value.toFixed(3))]
        })
        .filter(Boolean)
        .sort((a, b) => a[0] - b[0])
    },

    getCpuTotalSeries() {
      const userSeries = this.getRawMetricSeries('cpuUser')
      const systemSeries = this.getRawMetricSeries('cpuSystem')

      if (!userSeries.length && !systemSeries.length) {
        return this.getRawMetricSeries('cpu')
      }

      const pointsByTimestamp = new Map()
      for (const [timestamp, value] of [...userSeries, ...systemSeries]) {
        pointsByTimestamp.set(timestamp, (pointsByTimestamp.get(timestamp) || 0) + value)
      }

      return Array.from(pointsByTimestamp, ([timestamp, value]) => [
        timestamp,
        Number(value.toFixed(3))
      ]).sort((a, b) => a[0] - b[0])
    },

    getMetricSeries(key) {
      return key === 'cpu' ? this.getCpuTotalSeries() : this.getRawMetricSeries(key)
    },

    getLatestMetricValue(key) {
      const series = this.getMetricSeries(key)
      return series.length ? series[series.length - 1][1] : 0
    },

    getCurrentMetricValue(currentKey, historyKey) {
      const rawValue = this.hostCurrentMetrics?.[currentKey]
      const currentValue = rawValue === null || rawValue === undefined || rawValue === ''
        ? Number.NaN
        : Number(rawValue)
      return Number.isFinite(currentValue) ? currentValue : this.getLatestMetricValue(historyKey)
    },

    getCurrentCpuValue() {
      const cpuUser = Number(this.hostCurrentMetrics?.cpuUser)
      const cpuSystem = Number(this.hostCurrentMetrics?.cpuSystem)
      const hasCpuUser = Number.isFinite(cpuUser)
      const hasCpuSystem = Number.isFinite(cpuSystem)

      if (hasCpuUser || hasCpuSystem) {
        return (hasCpuUser ? cpuUser : 0) + (hasCpuSystem ? cpuSystem : 0)
      }

      const currentCpu = this.hostCurrentMetrics?.cpuUsage ?? this.hostCurrentMetrics?.cpu
      const numericCurrentCpu = Number(currentCpu)
      return Number.isFinite(numericCurrentCpu)
        ? numericCurrentCpu
        : this.getLatestMetricValue('cpu')
    },

    getLoadPercent(value) {
      const coreCount = Math.max(1, Number(this.hostDetail.cpu) || 1)
      return this.normalizeMetricPercent((Number(value) / coreCount) * 100)
    },

    formatMetricLoad(value) {
      const numericValue = Number(value)
      return Number.isFinite(numericValue) ? numericValue.toFixed(2) : '--'
    },

    setupHostChartResizeObserver() {
      this._hostChartResizeObserver?.disconnect()
      const container = this.$refs.hostTrendGrid
      if (!container || typeof ResizeObserver === 'undefined') return

      this._hostChartResizeObserver = new ResizeObserver(() => {
        Object.values(this._hostChartInstances || {}).forEach(chart => chart?.resize())
      })
      this._hostChartResizeObserver.observe(container)
    },

    renderHostTrendCharts() {
      if (!this.detailDrawer || this.hostMetricsLoading || !this.hasHostTrendData) return

      this.$nextTick(() => {
        const chartConfigs = [
          {
            ref: 'cpuTrendChart',
            key: 'cpu',
            color: '#3f7de8',
            name: 'CPU 总使用率',
            percent: true
          },
          {
            ref: 'loadTrendChart',
            key: 'load',
            series: [
              { key: 'load1min', name: '1 分钟', color: '#3f7de8' },
              { key: 'load5min', name: '5 分钟', color: '#e3a43b' },
              { key: 'load15min', name: '15 分钟', color: '#55a57a' }
            ]
          },
          {
            ref: 'memoryTrendChart',
            key: 'memory',
            color: '#22a49a',
            name: '内存',
            percent: true
          }
        ]

        chartConfigs.forEach(config => this.renderHostTrendChart(config))
        this.setupHostChartResizeObserver()
      })
    },

    renderHostTrendChart(config) {
      const chartElement = this.$refs[config.ref]
      if (!chartElement) return

      let chart = this._hostChartInstances[config.key]
      if (!chart) {
        chart = echarts.init(chartElement, null, { renderer: 'canvas' })
        this._hostChartInstances[config.key] = chart
      }

      const sourceSeries = config.series || [{ key: config.key, name: config.name, color: config.color }]
      const chartSeries = sourceSeries.map((item, index) => ({
        name: item.name,
        type: 'line',
        data: this.getMetricSeries(item.key),
        showSymbol: false,
        smooth: 0.28,
        connectNulls: false,
        lineStyle: { width: index === 0 ? 2 : 1.5, color: item.color },
        itemStyle: { color: item.color },
        areaStyle: sourceSeries.length === 1
          ? {
              color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                { offset: 0, color: `${item.color}30` },
                { offset: 1, color: `${item.color}05` }
              ])
            }
          : undefined,
        emphasis: { focus: 'series' }
      }))

      chart.setOption({
        animationDuration: 350,
        grid: { left: 8, right: 10, top: 12, bottom: 2, containLabel: true },
        tooltip: {
          trigger: 'axis',
          backgroundColor: 'rgba(255, 255, 255, 0.98)',
          borderColor: '#d9e2ef',
          borderWidth: 1,
          textStyle: { color: '#263449', fontSize: 11 },
          extraCssText: 'box-shadow: 0 8px 22px rgba(29, 45, 72, 0.12); border-radius: 6px;',
          axisPointer: { type: 'line', lineStyle: { color: '#9eb3d0', type: 'dashed' } },
          valueFormatter: value => `${Number(value).toFixed(2)}${config.percent ? '%' : ''}`
        },
        xAxis: {
          type: 'time',
          boundaryGap: false,
          axisLine: { lineStyle: { color: '#dce5f0' } },
          axisTick: { show: false },
          splitLine: { show: false },
          axisLabel: {
            color: '#8a96a8',
            fontSize: 10,
            hideOverlap: true,
            formatter: value => {
              const date = new Date(value)
              return `${String(date.getHours()).padStart(2, '0')}:${String(date.getMinutes()).padStart(2, '0')}`
            }
          }
        },
        yAxis: {
          type: 'value',
          min: 0,
          max: config.percent ? 100 : undefined,
          splitNumber: 3,
          axisLine: { show: false },
          axisTick: { show: false },
          axisLabel: {
            color: '#8a96a8',
            fontSize: 10,
            formatter: value => `${value}${config.percent ? '%' : ''}`
          },
          splitLine: { lineStyle: { color: '#edf1f6', type: 'dashed' } }
        },
        series: chartSeries
      }, true)
      chart.resize()
    },

    disposeHostCharts() {
      this._hostChartResizeObserver?.disconnect()
      this._hostChartResizeObserver = null
      Object.values(this._hostChartInstances || {}).forEach(chart => chart?.dispose())
      this._hostChartInstances = Object.create(null)
    },

    // 处理云主机导入成功
    handleCloudImportSuccess() {
      this.cloudDialogVisible = false
      this.getHostList()
      this.fetchHostStatistics()
    },

    // 处理Excel导入成功
    handleExcelImportSuccess() {
      this.getHostList()
      this.fetchHostStatistics()
    },

    // 处理JumpServer导入成功
    handleJumpServerImportSuccess() {
      this.$message.success('JumpServer同步任务已在后台执行,请稍后刷新列表查看结果')
      // 延迟5秒后自动刷新列表
      setTimeout(() => {
        this.getHostList()
        this.fetchHostStatistics()
      }, 5000)
    },

    // 连接SSH终端
    handleHostSSH() {
      if (!this.hostList.length) {
        this.$message.warning('请先选择主机')
        return
      }
      const selectedHost = this.hostList[0] // 默认选择第一个主机
      this.$router.push({
        path: '/cmdb/ssh',
        query: {
          hostId: selectedHost.id
        }
      })
    },

    // 跳转到资产授权页面
    handleHostAuthorization() {
      this.$router.push({
        path: '/system/post'
      })
    },

    // 跳转到主机详情页面
    handleHostDetail() {
      console.log('点击主机详情按钮')
      console.log('selectedHosts:', this.selectedHosts)
      console.log('hostList:', this.hostList)

      // 如果没有选中主机，但有主机列表，使用第一台主机
      if (!this.selectedHosts || this.selectedHosts.length === 0) {
        if (this.hostList && this.hostList.length > 0) {
          const host = this.hostList[0]
          const path = Number(host?.hostType ?? host?.host_type) === 2
            ? `/cmdb/host/windows-detail/${host.id}`
            : `/cmdb/host/detail/${host.id}`
          this.$router.push(path)
          return
        } else {
          this.$message.warning('请先选择一台主机')
          return
        }
      }

      if (this.selectedHosts.length > 1) {
        this.$message.warning('请只选择一台主机')
        return
      }

      const host = this.selectedHosts[0]
      const path = Number(host?.hostType ?? host?.host_type) === 2
        ? `/cmdb/host/windows-detail/${host.id}`
        : `/cmdb/host/detail/${host.id}`
      this.$router.push(path)
    },

    // 显示下载对话框
    showDownloadDialog(row) {
      this.$refs.fileManager.showDownloadDialog(row)
    },

    // 文件操作成功回调
    handleFileOperationSuccess() {
      // 可以在这里添加需要的刷新逻辑
      console.log('文件操作成功')
    },

    // 处理下拉框显示状态变化
    handleDropdownVisibleChange(visible) {
      console.log('下拉框显示状态变化:', visible)
      if (visible) {
        // 当下拉框即将显示时检查权限
        const hasPermission = this.checkPermission(['cmdb:ecs:add'])
        console.log('权限检查结果:', hasPermission)

        if (!hasPermission) {
          this.$message.warning('您没有新建主机的权限')
          // 阻止下拉框显示
          this.$nextTick(() => {
            this.$refs.createDropdown?.hide()
          })
          return false
        }
      }
    },

    // 处理新建按钮点击（现在只是一个占位符，真正的逻辑在visible-change中）
    handleCreateClick() {
      console.log('点击了新建按钮')
      // 权限检查会在 handleDropdownVisibleChange 中进行
    },

    // 处理下拉框选项点击
    handleCreateCommand(command) {
      console.log('选择了下拉框选项:', command)

      switch (command) {
        case 'importHost':
          this.addDialogVisible = true
          break
        case 'excelImport':
          this.ExcelDialogVisible = true
          break
        case 'jumpserverImport':
          this.jumpServerDialogVisible = true
          break
        case 'cloudHost':
          this.cloudDialogVisible = true
          break
      }
    },

    // 检查权限方法
    checkPermission(permissions) {
      console.log('checkPermission被调用，权限列表:', permissions)

      // 临时返回true用于测试
      console.log('权限检查通过')
      return true

      // TODO: 实现真正的权限检查逻辑
      // 假设您有全局的权限检查方法
      // if (this.$checkPermission) {
      //   return this.$checkPermission(permissions)
      // }

      // 或者检查store中的权限
      // if (this.$store && this.$store.getters.permissions) {
      //   const userPermissions = this.$store.getters.permissions
      //   return permissions.some(permission => userPermissions.includes(permission))
      // }
    },

    // 删除主机
    async handleHostDelete(row) {
      const confirmResult = await this.$confirm('是否确认删除主机"' + row.hostName + '"?', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).catch(err => err)

      if (confirmResult !== 'confirm') {
        return this.$message.info('已取消删除')
      }

      const { data: res } = await this.$api.deleteCmdbHost(row.id)
      if (res.code === 200) {
        this.$message.success('删除成功')
        this.getHostList()
        this.fetchHostStatistics()
      }
    },

    // 处理表格选择变化
    handleHostSelectionChange(selection) {
      this.selectedHosts = selection
    },

    // 处理批量操作命令
    handleBatchCommand(command) {
      const hostTable = this.$refs.hostTable
      if (!this.selectedHosts || this.selectedHosts.length === 0) {
        this.$message.warning('请先选择要操作的主机')
        return
      }

      switch (command) {
        case 'updateGroup':
          // 调用子组件的批量修改分组方法
          hostTable.showBatchUpdateGroupDialog()
          break
        case 'updateVendor':
          this.$message.info('修改类型功能开发中...')
          // TODO: 实现批量修改类型功能
          break
        case 'bindTag':
          this.$message.info('绑定标签功能开发中...')
          // TODO: 实现批量绑定标签功能
          break
        case 'batchDelete':
          this.handleBatchDelete()
          break
        default:
          break
      }
    },

    // 批量删除主机
    async handleBatchDelete() {
      if (!this.selectedHosts || this.selectedHosts.length === 0) {
        this.$message.warning('请先选择要删除的主机')
        return
      }

      const hostNames = this.selectedHosts.map(h => h.hostName).join('、')
      const confirmResult = await this.$confirm(
        `是否确认删除以下 ${this.selectedHosts.length} 台主机？\n\n${hostNames}`,
        '批量删除确认',
        {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning',
          dangerouslyUseHTMLString: false
        }
      ).catch(err => err)

      if (confirmResult !== 'confirm') {
        return this.$message.info('已取消删除')
      }

      try {
        // 提取所有主机ID
        const hostIds = this.selectedHosts.map(h => h.id)

        // 调用批量删除接口
        const { data: res } = await this.$api.batchDeleteCmdbHost(hostIds)

        if (res.code === 200) {
          const deletedCount = res.data?.deletedCount || hostIds.length
          this.$message.success(res.data?.message || `成功删除 ${deletedCount} 台主机`)

          // 清除选择
          const hostTable = this.$refs.hostTable
          if (hostTable) {
            hostTable.clearSelection()
          }

          // 刷新列表
          this.getHostList()
          this.fetchHostStatistics()
        } else {
          this.$message.error(res.message || '批量删除失败')
        }
      } catch (error) {
        console.error('批量删除异常:', error)
        this.$message.error('批量删除操作失败')
      }
    },

    // 同步主机配置信息
    async handleHostSync() {
      try {
        // 检查是否有当前主机详情
        if (!this.hostDetail || !this.hostDetail.id) {
          this.$message.warning('请先选择要同步的主机')
          return
        }
        
        this.syncLoading = true
        const { data: res } = await this.$api.syncHostConfig(this.hostDetail.id)
        
        if (res.code === 200) {
          this.$message.success(res.data?.message || '开始同步主机信息，请稍后查看结果')
          // 同步成功后刷新主机列表，延迟3秒后刷新以获取最新数据
          setTimeout(() => {
            this.getHostList()
            // 如果详情面板打开，也刷新详情信息
            if (this.detailDrawer && this.hostDetail.id) {
              this.showHostDetail({ id: this.hostDetail.id })
            }
          }, 3000)
        } else {
          this.$message.error(res.message || '同步失败')
        }
      } catch (error) {
        console.error('同步主机配置失败:', error)
        this.$message.error('同步失败: ' + (error.message || '未知错误'))
      } finally {
        this.syncLoading = false
      }
    },

    // 分组管理 - 创建分组
    async handleCreateGroup(groupData) {
      try {
        console.log('创建分组数据:', groupData)
        const { data: res } = await cmdbApi.createCmdbGroup(groupData)
        
        if (res.code === 200) {
          this.$message.success('创建分组成功')
          // 刷新分组列表
          await this.getAllGroups()
        } else {
          this.$message.error(res.message || '创建分组失败')
        }
      } catch (error) {
        console.error('创建分组失败:', error)
        this.$message.error('创建分组失败: ' + (error.response?.data?.message || error.message))
      }
    },

    // 分组管理 - 更新分组
    async handleUpdateGroup(groupData) {
      try {
        console.log('更新分组数据:', groupData)
        const { data: res } = await cmdbApi.updateCmdbGroup(groupData)
        
        if (res.code === 200) {
          this.$message.success('更新分组成功')
          // 刷新分组列表
          await this.getAllGroups()
        } else {
          this.$message.error(res.message || '更新分组失败')
        }
      } catch (error) {
        console.error('更新分组失败:', error)
        this.$message.error('更新分组失败: ' + (error.response?.data?.message || error.message))
      }
    },

    // 分组管理 - 删除分组
    async handleDeleteGroup(groupId) {
      try {
        console.log('删除分组ID:', groupId)
        const { data: res } = await cmdbApi.deleteCmdbGroup(groupId)

        if (res.code === 200) {
          this.$message.success('删除分组成功')
          // 刷新分组列表
          await this.getAllGroups()
          // 如果删除的是当前选中的分组，重置选择
          if (this.currentGroupId === groupId) {
            this.currentGroupId = null
            this.getHostList()
          }
        } else {
          this.$message.error(res.message || '删除分组失败')
        }
      } catch (error) {
        console.error('删除分组失败:', error)
        this.$message.error('删除分组失败: ' + (error.response?.data?.message || error.message))
      }
    },

    normalizeMetricPercent(value) {
      const num = Number(value)
      if (!Number.isFinite(num)) return 0
      return Math.min(100, Math.max(0, Number(num.toFixed(2))))
    },

    formatMetricPercent(value) {
      return `${this.normalizeMetricPercent(value).toFixed(2)}%`
    },

    getMetricTone(value) {
      const num = this.normalizeMetricPercent(value)
      if (num >= 80) return 'danger'
      if (num >= 60) return 'warning'
      return 'normal'
    },

    getMetricProgressColor(value) {
      const tone = this.getMetricTone(value)
      if (tone === 'danger') return '#ef6b73'
      if (tone === 'warning') return '#f3b24b'
      return '#4c8bf5'
    },

    getConnectionAddress(host) {
      const user = host?.sshName || '-'
      const ip = host?.sshIp || host?.privateIp || host?.publicIp || '-'
      const port = host?.sshPort || '-'
      return `${user}@${ip}:${port}`
    },

    formatHostSpec(host) {
      const parts = []
      if (host?.cpu !== '' && host?.cpu !== null && host?.cpu !== undefined) {
        parts.push(`${host.cpu}核`)
      }
      if (host?.memory !== '' && host?.memory !== null && host?.memory !== undefined) {
        parts.push(`${host.memory}G`)
      }
      if (host?.disk !== '' && host?.disk !== null && host?.disk !== undefined) {
        parts.push(`${host.disk}GB`)
      }
      return parts.length ? parts.join(' / ') : '-'
    }

  }
}
</script>

<style scoped>
/* 🎨 现代化科技感设计风格 - 仿照cmdbDB.vue */

.cmdb-host-management {
  padding: 0;
  box-sizing: border-box;
  height: calc(100vh - 124px);
  min-height: 0;
  background: transparent;
  overflow: hidden;
}

.host-card {
  background: transparent !important;
  backdrop-filter: none !important;
  border-radius: 0 !important;
  box-shadow: none !important;
  border: none !important;
  height: 100% !important;
  min-height: 0 !important;
  padding: 0 !important;
}

:deep(.host-card) {
  background: transparent !important;
  border: none !important;
  box-shadow: none !important;
  height: 100% !important;
  min-height: 0 !important;
  padding: 0 !important;
}

:deep(.el-card) {
  background: transparent !important;
  border: none !important;
  box-shadow: none !important;
  height: 100% !important;
  min-height: 0 !important;
  padding: 0 !important;
}

:deep(.el-card__body) {
  box-sizing: border-box;
  height: 100% !important;
  min-height: 0 !important;
  padding: 0 !important;
}

.host-management-container {
  display: flex;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.group-tree-section {
  width: 250px;
  margin-right: 10px;
  flex: 0 0 250px;
  min-height: 0;
}

.host-table-section {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  min-width: 0; /* 允许flex容器压缩 */
}

/* 🔍 搜索区域样式 */
.search-section {
  flex: 0 0 auto;
  margin-bottom: 10px;
  padding: 10px 20px;
  background: linear-gradient(90deg, #fef7ef 0%, #f8f8f6 52%, #edf2fd 100%);
  border-radius: 16px;
  border: 1px solid rgba(223, 231, 242, 0.94);
  box-shadow: 0 10px 28px rgba(17, 24, 39, 0.06);
}

.search-section .demo-form-inline {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px 16px;
  overflow: hidden;
}

/* 📊 内联统计信息样式 */
.inline-statistics {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: nowrap;
}

.stat-item {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.stat-item--clickable {
  cursor: pointer;
  padding: 3px 6px;
  border-radius: 6px;
  transition: all 0.2s ease;
}

.stat-item--clickable:hover,
.stat-item--clickable:focus,
.stat-item--clickable.is-active {
  background: rgba(64, 158, 255, 0.12);
  color: #409eff;
  outline: none;
}

.stat-icon {
  width: 20px;
  height: 20px;
  object-fit: contain;
  flex-shrink: 0;
}

.stat-text {
  font-weight: 600;
  font-size: 14px;
  color: #2c3e50;
}

/* 🎯 操作按钮区域 */
.action-section {
  flex: 0 0 100%;
  display: flex;
  width: 100%;
  justify-content: flex-start;
  margin-top: 8px;
  margin-bottom: 0;
  padding-left: 0;
}

.action-section .mb8 {
  width: 100%;
  margin: 0;
}

.action-toolbar {
  display: flex !important;
  align-items: center;
  width: 100%;
  min-width: 0;
}

.action-toolbar :deep(.el-dropdown) {
  display: inline-flex;
  align-items: center;
}

.action-toolbar :deep(.el-button) {
  height: 32px;
  min-height: 32px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
}

.action-toolbar :deep(.el-button .el-icon) {
  line-height: 1;
}

.table-card {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 10px;
  border-radius: 16px;
  border: 1px solid rgba(223, 231, 242, 0.94);
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 10px 28px rgba(17, 24, 39, 0.06);
}

.table-card :deep(.table-section) {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  margin-bottom: 0;
  overflow: hidden;
}

.table-card :deep(.host-table) {
  flex: 1 1 auto;
  min-height: 0;
  height: 100%;
  border-radius: 12px;
  box-shadow: none;
}

.table-card :deep(.host-table .el-table__body-wrapper),
.table-card :deep(.host-table .el-scrollbar__wrap) {
  overflow-x: hidden !important;
}

.pagination-section {
  display: flex;
  justify-content: flex-end;
  flex: 0 0 auto;
  margin-top: auto;
  padding-top: 10px;
  border-top: 1px solid rgba(223, 231, 242, 0.82);
}

.font-weight-bold {
  font-weight: bold;
}

.table-operation {
  display: flex;
  justify-content: space-around;
}

/* 完全移除表单分割线 */
.el-dialog .el-form-item {
  border-bottom: none !important;
  margin-bottom: 12px;
  padding-bottom: 0;
}

/* 移除行和列之间的分割线 */
.el-row {
  border-bottom: none !important;
}

.el-col {
  border-right: none !important;
  padding-right: 0 !important;
  margin-right: 0 !important;
}

/* 移除最后一个列的右边距 */
.el-col:last-child {
  padding-right: 0 !important;
  margin-right: 0 !important;
}

/* 🎨 按钮样式优化 */
.el-button {
  border-radius: 8px;
  font-weight: 500;
  transition: all 0.3s ease;
}

.el-button:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

/* 📝 输入框和选择器样式 */
.el-input :deep(.el-input__wrapper),
.el-select :deep(.el-input__wrapper) {
  background: rgba(255, 255, 255, 0.8);
  border: 1px solid rgba(103, 126, 234, 0.2);
  border-radius: 8px;
  box-shadow: none;
  transition: all 0.3s ease;
}

.el-input :deep(.el-input__wrapper):hover,
.el-select :deep(.el-input__wrapper):hover {
  border-color: #c0c4cc;
}

.el-input :deep(.el-input__wrapper.is-focus),
.el-select :deep(.el-input__wrapper.is-focus) {
  border-color: #667eea;
  box-shadow: 0 0 0 2px rgba(103, 126, 234, 0.2);
  background: rgba(255, 255, 255, 1);
}

.el-input :deep(.el-input__inner),
.el-select :deep(.el-input__inner) {
  background: transparent;
  border: none;
  color: #2c3e50;
}

/* 🔧 表单项样式 */
.search-section .el-form-item {
  margin-bottom: 0;
  margin-right: 0;
  flex: 0 0 auto;
}

.search-section .el-form-item__label {
  color: #606266;
  font-weight: 500;
}

/* 🖥️ 终端按钮深色样式 */
.terminal-dark-btn {
  background: #2c3e50 !important;
  border-color: #2c3e50 !important;
  color: white !important;
}

.terminal-dark-btn:hover {
  background: #34495e !important;
  border-color: #34495e !important;
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(44, 62, 80, 0.4);
}

.host-export-btn {
  width: 32px;
  min-width: 32px;
  height: 32px;
  padding: 0 !important;
  margin-left: auto;
  border-color: #e6a23c !important;
  background: #e6a23c !important;
  color: #fff !important;
  box-shadow: 0 2px 6px rgba(230, 162, 60, 0.24);
}

.host-export-btn:hover,
.host-export-btn:focus {
  border-color: #d8922f !important;
  background: #d8922f !important;
  color: #fff !important;
}

.host-export-btn :deep(.el-icon) {
  margin: 0;
}

:global(.host-detail-drawer .el-drawer__header) {
  box-sizing: border-box;
  height: 44px;
  min-height: 44px;
  flex: 0 0 44px;
  margin: 0 !important;
  padding: 0 12px !important;
  border-bottom: 1px solid rgba(223, 231, 242, 0.9);
}

:global(.host-detail-drawer .el-drawer__title) {
  color: #172133;
  font-size: 15px;
  font-weight: 700;
  line-height: 1;
}

:global(.host-detail-drawer .el-drawer__close-btn) {
  width: 30px;
  height: 30px;
  margin: 0;
  border-radius: 6px;
  color: #56657a;
}

:global(.host-detail-drawer .el-drawer__close-btn:hover) {
  color: #245fbd;
  background: #edf4ff;
}

:global(.host-detail-drawer .el-drawer__body) {
  padding: 8px 12px 12px !important;
  background:
    radial-gradient(circle at top left, rgba(76, 139, 245, 0.05), transparent 24%),
    linear-gradient(180deg, #f8fafc 0%, #f5f8fc 100%);
}

.host-detail-panel {
  display: flex;
  flex-direction: column;
  gap: 8px;
  color: #172133;
}

.host-detail-hero,
.detail-surface-panel {
  position: relative;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 8px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.94));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.78),
    0 12px 26px rgba(17, 24, 39, 0.045);
}

.host-detail-hero::before,
.detail-surface-panel::before {
  content: '';
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 34px;
  background: linear-gradient(90deg, rgba(255, 249, 240, 0.94), rgba(248, 248, 246, 0.84), rgba(237, 242, 253, 0.9));
  pointer-events: none;
}

.host-detail-hero > *,
.detail-surface-panel > * {
  position: relative;
  z-index: 1;
}

.host-detail-hero {
  padding: 12px 14px;
}

.host-detail-title {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 9px;
}

.host-detail-name {
  color: #172133;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.2;
}

.host-detail-target {
  margin-top: 4px;
  color: #7a8799;
  font-size: 12px;
  font-family: Consolas, "Courier New", monospace;
}

.host-detail-badges {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.host-detail-badges :deep(.el-tag),
.detail-data-table :deep(.el-tag) {
  height: 21px;
  padding: 0 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
}

.metric-card-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0;
  overflow: hidden;
  border: 1px solid #dfe7f2;
  border-radius: 7px;
  background: rgba(255, 255, 255, 0.82);
}

.metric-card {
  display: flex;
  align-items: center;
  gap: 11px;
  min-width: 0;
  min-height: 92px;
  padding: 10px 11px;
  border-right: 1px solid #e6edf6;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.78), rgba(248, 251, 255, 0.78));
}

.metric-card:last-child {
  border-right: none;
}

.metric-ring {
  position: relative;
  display: flex;
  flex: 0 0 66px;
  align-items: center;
  justify-content: center;
  width: 66px;
  height: 66px;
}

.metric-ring :deep(.el-progress-circle__track) {
  stroke: #e9eef5;
}

.metric-ring strong {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #25344b;
  font-size: 12px;
  font-weight: 700;
  line-height: 1;
  white-space: nowrap;
}

.metric-card-copy,
.metric-card-label {
  display: flex;
}

.metric-card-copy {
  min-width: 0;
  flex-direction: column;
  gap: 5px;
}

.metric-card-label {
  align-items: center;
  gap: 6px;
  color: #344258;
  font-size: 12px;
  font-weight: 700;
  white-space: nowrap;
}

.metric-card-label i {
  width: 7px;
  height: 7px;
  flex: 0 0 7px;
  border-radius: 50%;
}

.metric-card-copy > span {
  color: #8a96a8;
  font-size: 10px;
  line-height: 1.3;
}

.metric-card-copy small {
  color: #657287;
  font-size: 10px;
  line-height: 1.3;
  white-space: nowrap;
}

.metric-card.is-warning .metric-ring strong {
  color: #d68c28;
}

.metric-card.is-danger .metric-ring strong {
  color: #e6505a;
}

.detail-surface-panel {
  padding: 10px 14px 12px;
}

.surface-panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.panel-eyebrow {
  margin-bottom: 2px;
  color: #9aa4b2;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.surface-panel-head h3 {
  margin: 0;
  color: #172133;
  font-size: 15px;
  line-height: 1.2;
}

.surface-panel-head :deep(.el-button) {
  height: 28px;
  border-radius: 6px;
  padding: 0 12px;
}

.trend-panel-actions {
  display: flex;
  align-items: center;
  gap: 9px;
  color: #7a8799;
  font-size: 11px;
}

.trend-panel-actions :deep(.el-button) {
  width: 28px;
  min-width: 28px;
  padding: 0;
}

.host-trend-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.trend-chart-card {
  min-width: 0;
  padding: 10px 11px 7px;
  border: 1px solid #dfe7f2;
  border-radius: 7px;
  background: rgba(255, 255, 255, 0.84);
}

.trend-chart-card:last-child {
  grid-column: 1 / -1;
}

.trend-chart-head,
.trend-chart-head > div,
.load-legend,
.load-legend span {
  display: flex;
  align-items: center;
}

.trend-chart-head {
  justify-content: space-between;
  gap: 10px;
}

.trend-chart-head > div:first-child {
  gap: 8px;
  min-width: 0;
}

.trend-chart-head span {
  color: #657287;
  font-size: 11px;
}

.trend-chart-head strong {
  color: #24344c;
  font-size: 14px;
  line-height: 1;
}

.trend-legend-dot,
.load-legend i {
  width: 7px;
  height: 7px;
  flex: 0 0 7px;
  border-radius: 50%;
}

.trend-legend-dot.is-cpu,
.load-legend .is-load1 {
  background: #3f7de8;
}

.trend-legend-dot.is-memory {
  background: #22a49a;
}

.load-legend {
  gap: 8px;
}

.load-legend span {
  gap: 4px;
  font-size: 10px;
}

.load-legend .is-load5 {
  background: #e3a43b;
}

.load-legend .is-load15 {
  background: #55a57a;
}

.trend-chart {
  width: 100%;
  height: 142px;
}

.trend-loading-state {
  min-height: 230px;
  padding: 15px 12px 0;
  border: 1px solid #e2e9f2;
  border-radius: 7px;
  background: rgba(255, 255, 255, 0.76);
}

.trend-feedback-state {
  display: flex;
  align-items: center;
  min-height: 76px;
  gap: 10px;
  padding: 12px 14px;
  border: 1px solid #f0d4d7;
  border-radius: 7px;
  background: #fff8f8;
  color: #d9535f;
}

.trend-feedback-state > div {
  display: flex;
  flex: 1;
  min-width: 0;
  flex-direction: column;
  gap: 3px;
}

.trend-feedback-state strong {
  color: #9f3742;
  font-size: 12px;
}

.trend-feedback-state span {
  overflow: hidden;
  color: #a95b63;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.trend-empty-state {
  min-height: 150px;
  padding: 8px 0 0;
  border: 1px dashed #dbe4ef;
  border-radius: 7px;
  background: rgba(255, 255, 255, 0.64);
}

@media (max-width: 720px) {
  .metric-card-grid,
  .host-trend-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .metric-card:nth-child(2) {
    border-right: none;
  }

  .metric-card:nth-child(-n + 2) {
    border-bottom: 1px solid #e6edf6;
  }

  .metric-card {
    min-height: 88px;
  }

  .trend-chart-card:last-child {
    grid-column: auto;
  }

  .trend-chart {
    height: 132px;
  }
}

.detail-table-shell {
  overflow: hidden;
  border-radius: 12px;
  border: 1px solid #dfe7f2;
  background: rgba(255, 255, 255, 0.8);
}

.detail-data-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
  font-size: 12px;
}

.detail-data-table th,
.detail-data-table td {
  height: 30px;
  padding: 6px 10px;
  border-bottom: 1px solid #e6edf6;
  color: #39465a;
  line-height: 1.3;
  word-break: break-all;
}

.detail-data-table tr:last-child th,
.detail-data-table tr:last-child td {
  border-bottom: none;
}

.detail-data-table th {
  width: 18%;
  color: #657287;
  font-weight: 700;
  background: rgba(243, 247, 253, 0.95);
}

.detail-data-table td {
  width: 32%;
  background: rgba(255, 255, 255, 0.72);
}

.detail-data-table .is-mono {
  font-family: Consolas, "Courier New", monospace;
  font-size: 11px;
}

/* 📁 文件浏览器样式 */
.breadcrumb-nav {
  margin-bottom: 16px;
  padding: 12px;
  background-color: #f5f7fa;
  border-radius: 8px;
}

.breadcrumb-nav :deep(.el-breadcrumb__item) {
  cursor: pointer;
  transition: all 0.3s;
}

.breadcrumb-nav :deep(.el-breadcrumb__item:hover) {
  color: #409eff;
}

.current-path {
  margin-bottom: 16px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.current-path .el-input {
  flex: 1;
}

.current-path .path-action-btn {
  flex-shrink: 0;
}

.current-path :deep(.el-input-group__prepend) {
  background-color: #ecf5ff;
  border-color: #409eff;
  color: #409eff;
  padding: 0 15px;
}

.current-path :deep(.el-input__wrapper) {
  border-left-color: #409eff;
}

.current-path .el-text {
  color: #606266;
  font-weight: 500;
}
</style>
