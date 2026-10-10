<template>
  <div class="cmdb-host-management">
    <div class="page-shell">
      <div class="page-card">
        <div class="page-header">
          <div>
            <p class="page-kicker">Config Center</p>
            <h2>主机资产管理</h2>
            <p class="page-description">
              统一管理主机接入、认证状态与实时监控，保持分组与资产信息的可扫描性。
            </p>
          </div>
          <div class="summary-chip-group">
            <div class="summary-chip">
              <span>总主机数</span>
              <strong>{{ totalHostCount }}</strong>
            </div>
            <div class="summary-chip">
              <span>当前筛选</span>
              <strong>{{ total }}</strong>
            </div>
          </div>
        </div>

        <div class="content-grid">
          <aside class="group-panel">
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
          </aside>

          <section class="main-panel">
            <div class="toolbar-card">
              <el-form
                :inline="true"
                :model="queryParams"
                class="toolbar-form"
                @submit.prevent
              >
                <el-form-item label="主机名称/IP地址" prop="keyword">
                  <el-input
                    v-model="queryParams.keyword"
                    size="small"
                    placeholder="请输入主机名称或IP地址"
                    clearable
                    @keyup.enter="handleQuery"
                  />
                </el-form-item>

                <el-form-item label="主机状态" prop="status">
                  <el-select
                    v-model="queryParams.status"
                    size="small"
                    placeholder="请选择状态"
                    clearable
                  >
                    <el-option
                      v-for="item in statusList"
                      :key="item.value"
                      :label="item.label"
                      :value="item.value"
                    />
                  </el-select>
                </el-form-item>

                <el-form-item class="toolbar-form-actions-item">
                  <div class="toolbar-actions">
                    <el-button type="primary" @click="handleQuery">
                      <el-icon><Search /></el-icon>
                      <span>搜索</span>
                    </el-button>

                    <el-button @click="resetQuery">
                      <el-icon><Refresh /></el-icon>
                      <span>重置</span>
                    </el-button>

                    <el-dropdown
                      ref="createDropdown"
                      trigger="click"
                      placement="bottom-start"
                      :hide-on-click="true"
                      @command="handleCreateCommand"
                      @visible-change="handleDropdownVisibleChange"
                    >
                      <el-button
                        type="primary"
                        v-authority="['cmdb:ecs:add']"
                        @click.stop="handleCreateClick"
                      >
                        <el-icon><Plus /></el-icon>
                        <span>新建</span>
                        <el-icon class="el-icon--right"><ArrowDown /></el-icon>
                      </el-button>
                      <template #dropdown>
                        <el-dropdown-menu>
                          <el-dropdown-item command="importHost">
                            <el-icon><Edit /></el-icon>
                            新增主机
                          </el-dropdown-item>
                          <el-dropdown-item command="excelImport">
                            <el-icon><Folder /></el-icon>
                            Excel创建主机
                          </el-dropdown-item>
                          <el-dropdown-item command="cloudHost">
                            <el-icon><MostlyCloudy /></el-icon>
                            新增云主机
                          </el-dropdown-item>
                          <el-dropdown-item command="jumpserverImport">
                            <el-icon><Connection /></el-icon>
                            JumpServer导入
                          </el-dropdown-item>
                        </el-dropdown-menu>
                      </template>
                    </el-dropdown>

                    <el-button
                      class="secondary-action"
                      v-authority="['cmdb:ecs:terminal']"
                      @click="handleHostSSH"
                    >
                      <el-icon><Monitor /></el-icon>
                      <span>终端</span>
                    </el-button>

                    <el-button class="secondary-action" @click="handleHostAuthorization">
                      <el-icon><Key /></el-icon>
                      <span>资产授权</span>
                    </el-button>

                    <el-dropdown @command="handleBatchCommand" trigger="click" placement="bottom-start">
                      <el-button
                        :disabled="!hasSelection"
                        :type="hasSelection ? 'primary' : 'info'"
                        v-authority="['cmdb:ecs:batch']"
                      >
                        <el-icon><Operation /></el-icon>
                        <span>批量操作</span>
                        <span v-if="selectionCount > 0">({{ selectionCount }})</span>
                        <el-icon class="el-icon--right"><ArrowDown /></el-icon>
                      </el-button>
                      <template #dropdown>
                        <el-dropdown-menu>
                          <el-dropdown-item command="updateGroup">
                            <el-icon><FolderOpened /></el-icon>
                            修改分组
                          </el-dropdown-item>
                          <el-dropdown-item command="updateVendor">
                            <el-icon><Edit /></el-icon>
                            修改类型
                          </el-dropdown-item>
                          <el-dropdown-item command="bindTag">
                            <el-icon><PriceTag /></el-icon>
                            绑定标签
                          </el-dropdown-item>
                          <el-dropdown-item command="batchDelete" divided>
                            <el-icon><Delete /></el-icon>
                            批量删除
                          </el-dropdown-item>
                        </el-dropdown-menu>
                      </template>
                    </el-dropdown>
                  </div>
                </el-form-item>
              </el-form>

              <div v-if="vendorStatistics.length" class="inline-statistics">
                <div class="stat-item">
                  <el-icon class="stat-leading-icon"><Monitor /></el-icon>
                  <span class="stat-text">总计 {{ totalHostCount }}</span>
                </div>
                <div
                  v-for="detail in vendorStatistics"
                  :key="detail.vendor"
                  class="stat-item stat-item--clickable"
                  :class="{ 'is-active': isStatisticActive(detail) }"
                  role="button"
                  tabindex="0"
                  :title="`查看${getStatisticName(detail)}主机`"
                  @click="handleVendorStatisticClick(detail)"
                  @keyup.enter="handleVendorStatisticClick(detail)"
                >
                  <el-icon v-if="Number(detail.vendor) === 1" :size="16"><OfficeBuilding /></el-icon>
                  <img v-else-if="Number(detail.vendor) === 2" src="@/assets/image/aliyun.png" class="stat-icon" alt="阿里云" />
                  <img v-else-if="Number(detail.vendor) === 3" src="@/assets/image/tengxun.png" class="stat-icon" alt="腾讯云" />
                  <img v-else-if="Number(detail.vendor) === 4" src="@/assets/image/baidu.svg" class="stat-icon" alt="百度云" />
                  <img v-else-if="Number(detail.vendor) === 5" src="@/assets/image/华为.svg" class="stat-icon" alt="华为云" />
                  <img v-else-if="Number(detail.vendor) === 6" src="@/assets/image/Windows.svg" class="stat-icon" alt="Windows" />
                  <img v-else-if="Number(detail.vendor) === 7" src="@/assets/image/tianyiyun.svg" class="stat-icon" alt="天翼云" />
                  <span class="stat-text">{{ getStatisticCount(detail) }}</span>
                </div>
              </div>
            </div>

            <div class="table-card">
              <div class="table-head">
                <div>
                  <h3>主机列表</h3>
                  <p>Host Assets & Runtime Metrics</p>
                </div>
                <span class="selection-pill">
                  已选 {{ selectionCount }} 台
                </span>
              </div>

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

              <div class="pagination-section">
                <el-pagination
                  :current-page="queryParams.pageNum"
                  :page-sizes="[10, 50, 100]"
                  :page-size="queryParams.pageSize"
                  :total="total"
                  layout="total, sizes, prev, pager, next, jumper"
                  @size-change="handleSizeChange"
                  @current-change="handleCurrentChange"
                />
              </div>
            </div>
          </section>
        </div>
      </div>
    </div>

    <CreateHost
      :visible="addDialogVisible"
      :group-list="groupList"
      :default-group-id="currentGroupId"
      :auth-list="authList"
      @close="addDialogVisible = false"
      @submit="addHost"
      @refresh-auth-list="getAuthList"
    />

    <EditHost
      :visible="editDialogVisible"
      :host-info="hostInfo"
      :group-list="groupList"
      :auth-list="authList"
      @close="editDialogVisible = false"
      @submit="editHost"
    />

    <CreateCloud v-model="cloudDialogVisible" @success="handleCloudImportSuccess" />
    <CreateExcel v-model="ExcelDialogVisible" @success="handleExcelImportSuccess" />
    <CreateJumpServer v-model="jumpServerDialogVisible" @success="handleJumpServerImportSuccess" />

    <HostSSH
      v-if="sshDialogVisible"
      :visible="sshDialogVisible"
      :host-id="currentHostId"
      @update:visible="val => {
        sshDialogVisible = val
      }"
    />

    <el-drawer
      v-model="detailDrawer"
      class="host-detail-drawer"
      title="主机详情"
      direction="rtl"
      size="40%"
      :before-close="handleDetailClose"
    >
      <div class="host-detail-content">
        <section class="dashboard-section">
          <div class="gauge-container">
            <div ref="cpuGauge" class="gauge-item"></div>
            <div ref="memoryGauge" class="gauge-item"></div>
            <div ref="diskGauge" class="gauge-item"></div>
          </div>
        </section>

        <section class="detail-section">
          <div class="detail-section-head">
            <h3>基本信息</h3>
          </div>
          <el-descriptions :column="1" border class="detail-descriptions">
            <el-descriptions-item label="主机名称">{{ hostDetail.hostName }}</el-descriptions-item>
            <el-descriptions-item label="主机分组">{{ getGroupName(hostDetail.groupId) }}</el-descriptions-item>
            <el-descriptions-item label="连接地址">
              {{ hostDetail.sshName }}@{{ hostDetail.sshIp }}:{{ hostDetail.sshPort }}
            </el-descriptions-item>
            <el-descriptions-item label="认证类型">
              <el-tag :type="getAuthTypeTagType(hostDetail.sshKeyId)" size="small">
                {{ getAuthTypeName(hostDetail.sshKeyId) }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="认证状态">
              <el-tag :type="getStatusTagType(hostDetail.status)">
                {{ getStatusText(hostDetail.status) }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="描述信息">{{ hostDetail.remark }}</el-descriptions-item>
          </el-descriptions>
        </section>

        <section class="detail-section">
          <div class="detail-section-head detail-section-head--between">
            <h3>扩展信息</h3>
            <el-button
              type="primary"
              class="sync-button"
              size="small"
              icon="Refresh"
              :loading="syncLoading"
              v-authority="['cmdb:ecs:rsync']"
              @click="handleHostSync"
            >
              {{ syncLoading ? '同步中...' : '同步' }}
            </el-button>
          </div>
          <el-descriptions :column="1" border class="detail-descriptions">
            <el-descriptions-item label="实例ID">{{ hostDetail.instanceId }}</el-descriptions-item>
            <el-descriptions-item label="实例名称">{{ hostDetail.name }}</el-descriptions-item>
            <el-descriptions-item label="操作系统">{{ hostDetail.os }}</el-descriptions-item>
            <el-descriptions-item label="CPU">{{ hostDetail.cpu }}核</el-descriptions-item>
            <el-descriptions-item label="内存">{{ hostDetail.memory }}G</el-descriptions-item>
            <el-descriptions-item label="磁盘">{{ hostDetail.disk }}GB</el-descriptions-item>
            <el-descriptions-item label="内网IP">{{ hostDetail.privateIp }}</el-descriptions-item>
            <el-descriptions-item label="公网IP">{{ hostDetail.publicIp || '无' }}</el-descriptions-item>
            <el-descriptions-item label="实例计费方式">{{ hostDetail.billingType }}</el-descriptions-item>
            <el-descriptions-item label="网络计费方式">{{ hostDetail.networkBillingType || '按流量计费' }}</el-descriptions-item>
            <el-descriptions-item label="创建时间">{{ hostDetail.createTime }}</el-descriptions-item>
            <el-descriptions-item label="到期时间">{{ hostDetail.expireTime || '无' }}</el-descriptions-item>
            <el-descriptions-item label="更新时间">{{ hostDetail.updateTime }}</el-descriptions-item>
          </el-descriptions>
        </section>
      </div>
    </el-drawer>

    <FileManager
      ref="fileManager"
      @upload-success="handleFileOperationSuccess"
      @download-success="handleFileOperationSuccess"
    />
  </div>
</template>

<script>
import * as echarts from 'echarts'
import configApi from '@/api/config'
import cmdbApi from '@/api/cmdb'
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
      groupSearchText: '',
      // 批量操作相关
      selectedHosts: [],
      statusList: [
        { value: 2, label: '未认证' },
        { value: 1, label: '认证成功' },
        { value: 3, label: '认证失败' }
      ],
      loading: false,
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
      },
    // ECharts 实例
    cpuChart: null,
    memoryChart: null,
    diskChart: null
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
    totalHostCount() {
      const statsCount = Number(this.hostStatistics && this.hostStatistics.total)
      if (Number.isFinite(statsCount)) {
        return statsCount
      }
      return this.total
    },
    vendorStatistics() {
      if (!this.hostStatistics || !Array.isArray(this.hostStatistics.details)) {
        return []
      }
      return this.hostStatistics.details
    }
  },
  created() {
    this.getAllGroups()
    this.getAuthList()
    // 默认加载所有主机
    this.getHostList()
    // 加载主机统计信息
    this.fetchHostStatistics()
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
      return Number(detail?.vendor) === 6 ? this.windowsHostCount : (detail?.count || 0)
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
          this.$message.error(`认证凭据不存在(凭据ID: ${requestData.sshKeyId})，请检查后重试`)
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
        this.hostInfo = {
          id: res.data.id,
          hostName: res.data.hostName,
          groupId: res.data.groupId,
          remark: res.data.remark,
          ip: res.data.sshIp,
          port: res.data.sshPort,
          username: res.data.sshName,
          authId: res.data.sshKeyId,
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
        // 验证凭据是否存在
        const authExists = this.authList.some(auth => auth.id === requestData.sshKeyId)
        if (!authExists) {
          this.$message.error('选择的认证凭据不存在，请刷新凭据列表后重试')
          return false
        }

        // 确保端口为数字
        const port = Number(requestData.sshPort)
        if (isNaN(port) || port < 1 || port > 65535) {
          this.$message.error('端口号必须为1-65535之间的数字')
          return false
        }
        const { data: res } = await this.$api.updateCmdbHost(requestData)
        if (res.code === 200) {
          this.$message.success('修改主机成功')
          this.editDialogVisible = false
          this.getHostList()
          return true
        } else if (res.code === 426) {
          this.$message.error(`认证凭据不存在(凭据ID: ${requestData.sshKeyId})，请检查后重试`)
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
          this.detailDrawer = true

          // 获取监控数据
          await this.fetchHostMonitorData(row.id)

          // 等待 DOM 更新后初始化图表
          this.$nextTick(() => {
            this.initGaugeCharts()
          })
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
      try {
        const { data: res } = await this.$api.getHostsMonitorData(hostId)

        if (res.code === 200 && res.data) {
          const monitorData = res.data[hostId]
          if (monitorData) {
            // 更新监控数据，保留两位小数
            this.hostDetail.cpuUsage = parseFloat(monitorData.cpuUsage?.toFixed(2) || 0)
            this.hostDetail.memoryUsage = parseFloat(monitorData.memoryUsage?.toFixed(2) || 0)
            this.hostDetail.diskUsage = parseFloat(monitorData.diskUsage?.toFixed(2) || 0)
          }
        }
      } catch (error) {
        console.error('获取主机监控数据失败:', error)
        // 不显示错误提示，使用默认值
        this.hostDetail.cpuUsage = 0
        this.hostDetail.memoryUsage = 0
        this.hostDetail.diskUsage = 0
      }
    },

    // 关闭详情抽屉
    handleDetailClose() {
      this.detailDrawer = false
      // 销毁图表实例
      this.destroyGaugeCharts()
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

    // 初始化仪表盘图表
    initGaugeCharts() {
      this.$nextTick(() => {
        // 初始化 CPU 仪表盘
        if (this.$refs.cpuGauge) {
          this.cpuChart = echarts.init(this.$refs.cpuGauge)
          this.cpuChart.setOption(this.getGaugeOption(this.hostDetail.cpuUsage || 0, 'CPU'))
        }

        // 初始化内存仪表盘
        if (this.$refs.memoryGauge) {
          this.memoryChart = echarts.init(this.$refs.memoryGauge)
          this.memoryChart.setOption(this.getGaugeOption(this.hostDetail.memoryUsage || 0, '内存'))
        }

        // 初始化磁盘仪表盘
        if (this.$refs.diskGauge) {
          this.diskChart = echarts.init(this.$refs.diskGauge)
          this.diskChart.setOption(this.getGaugeOption(this.hostDetail.diskUsage || 0, '磁盘'))
        }
      })
    },

    // 获取仪表盘配置
    getGaugeOption(value, name) {
      return {
        series: [
          {
            type: 'gauge',
            startAngle: 225,
            endAngle: -45,
            min: 0,
            max: 100,
            radius: '85%',
            center: ['50%', '60%'],
            splitNumber: 10,
            axisLine: {
              lineStyle: {
                width: 10,
                color: [
                  [0.3, '#67e0e3'],
                  [0.7, '#37a2da'],
                  [1, '#fd666d']
                ]
              }
            },
            pointer: {
              length: '60%',
              width: 4,
              itemStyle: {
                color: '#4169E1'
              }
            },
            axisTick: {
              distance: -10,
              length: 4,
              lineStyle: {
                color: '#333',
                width: 1
              }
            },
            splitLine: {
              distance: -10,
              length: 8,
              lineStyle: {
                color: '#333',
                width: 2
              }
            },
            axisLabel: {
              color: '#666',
              distance: 20,
              fontSize: 12,
              formatter: function(value) {
                return value
              }
            },
            detail: {
              valueAnimation: true,
              formatter: '{value}%',
              color: '#ff0000',
              fontSize: 20,
              fontWeight: 'bold',
              offsetCenter: [0, '70%']
            },
            title: {
              offsetCenter: [0, '50%'],
              fontSize: 14,
              color: '#333',
              fontWeight: 'bold'
            },
            data: [
              {
                value: value,
                name: name
              }
            ]
          }
        ]
      }
    },

    // 销毁图表实例
    destroyGaugeCharts() {
      if (this.cpuChart) {
        this.cpuChart.dispose()
        this.cpuChart = null
      }
      if (this.memoryChart) {
        this.memoryChart.dispose()
        this.memoryChart = null
      }
      if (this.diskChart) {
        this.diskChart.dispose()
        this.diskChart = null
      }
    }

  }
}
</script>

<style scoped>
.cmdb-host-management {
  --host-bg-page: #f4f7fb;
  --host-text-primary: #172133;
  --host-text-secondary: #5f6b7c;
  --host-text-muted: #8d97a8;
  --host-border: rgba(22, 34, 56, 0.08);
  --host-border-strong: rgba(214, 223, 235, 0.96);
  --host-brand: #4c8bf5;
  --host-brand-active: #4d79d8;
  --host-topbar-gradient: linear-gradient(90deg, #fef7ef 0%, #f8f8f6 52%, #edf2fd 100%);
  padding: 18px;
  min-width: 0;
  background: transparent;
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.page-shell {
  width: 100%;
  margin: 0 auto;
  min-width: 0;
}

.page-card {
  position: relative;
  padding: 50px 24px 24px;
  border-radius: 28px;
  border: 1px solid var(--host-border-strong);
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.96), rgba(247, 250, 255, 0.92));
  box-shadow: 0 18px 48px rgba(17, 24, 39, 0.08);
  overflow: hidden;
  isolation: isolate;
  min-width: 0;
}

.page-card::before {
  content: "";
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 40px;
  z-index: 0;
  pointer-events: none;
  background: var(--host-topbar-gradient);
}

.page-header {
  position: relative;
  z-index: 1;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 18px;
  margin: -50px -24px 18px;
  padding: 12px 24px 16px;
  border-bottom: 1px solid var(--host-border-strong);
  background: var(--host-topbar-gradient);
}

.page-kicker {
  margin: 0 0 10px;
  color: #6d7b8f;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.page-header h2 {
  margin: 0;
  color: var(--host-text-primary);
  font-size: 32px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: -0.02em;
}

.page-description {
  margin: 10px 0 0;
  max-width: 740px;
  color: var(--host-text-secondary);
  font-size: 14px;
  line-height: 1.7;
}

.summary-chip-group {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.summary-chip {
  min-width: 118px;
  padding: 12px 14px 10px;
  border-radius: 16px;
  border: 1px solid var(--host-border-strong);
  background: rgba(255, 255, 255, 0.88);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.66);
}

.summary-chip span {
  display: block;
  color: var(--host-text-muted);
  font-size: 11px;
}

.summary-chip strong {
  display: block;
  margin-top: 4px;
  color: var(--host-text-primary);
  font-size: 24px;
  font-weight: 700;
}

.content-grid {
  display: grid;
  grid-template-columns: minmax(260px, 280px) minmax(0, 1fr);
  gap: 16px;
  min-width: 0;
}

.group-panel,
.main-panel {
  min-width: 0;
}

.main-panel {
  display: grid;
  gap: 14px;
}

.toolbar-card,
.table-card {
  border-radius: 24px;
  border: 1px solid var(--host-border-strong);
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.95), rgba(248, 251, 255, 0.92));
  box-shadow: 0 12px 34px rgba(17, 24, 39, 0.05);
  min-width: 0;
}

.toolbar-card {
  padding: 16px 18px;
}

.toolbar-form {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px 12px;
}

.toolbar-form :deep(.el-form-item) {
  margin-right: 0;
  margin-bottom: 0;
}

.toolbar-form :deep(.el-form-item__label) {
  color: var(--host-text-secondary);
  font-weight: 500;
}

.toolbar-form :deep(.el-input__wrapper),
.toolbar-form :deep(.el-select__wrapper),
.toolbar-form :deep(.el-select .el-input__wrapper) {
  border: 1px solid rgba(22, 34, 56, 0.1);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.9);
  box-shadow: none;
  transition: border-color 0.22s ease, box-shadow 0.22s ease;
}

.toolbar-form :deep(.el-input__wrapper:hover),
.toolbar-form :deep(.el-select .el-input__wrapper:hover) {
  border-color: rgba(76, 139, 245, 0.35);
}

.toolbar-form :deep(.is-focus.el-input__wrapper),
.toolbar-form :deep(.el-select .is-focus.el-input__wrapper) {
  border-color: rgba(76, 139, 245, 0.72);
  box-shadow: 0 0 0 3px rgba(76, 139, 245, 0.14);
}

.toolbar-form-actions-item {
  margin-left: auto;
}

.toolbar-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.toolbar-actions :deep(.el-button) {
  height: 34px;
  border-radius: 14px;
  padding: 0 14px;
  border: 1px solid rgba(22, 34, 56, 0.12);
  color: var(--host-text-secondary);
  background: rgba(255, 255, 255, 0.84);
  transition: transform 0.22s ease, border-color 0.22s ease, box-shadow 0.22s ease, color 0.22s ease;
}

.toolbar-actions :deep(.el-button + .el-button) {
  margin-left: 0;
}

.toolbar-actions :deep(.el-button:hover) {
  transform: translateY(-1px);
  border-color: rgba(76, 139, 245, 0.32);
  box-shadow: 0 10px 20px rgba(17, 24, 39, 0.08);
}

.toolbar-actions :deep(.el-button--primary) {
  border-color: rgba(76, 139, 245, 0.78);
  background: linear-gradient(180deg, #4f8ef6 0%, #4a82e6 100%);
  color: #fff;
}

.toolbar-actions :deep(.el-button--primary:hover) {
  border-color: rgba(76, 139, 245, 0.92);
}

.toolbar-actions :deep(.el-button--info) {
  border-color: rgba(22, 34, 56, 0.1);
  background: rgba(255, 255, 255, 0.75);
  color: var(--host-text-muted);
}

.toolbar-actions .secondary-action {
  font-weight: 500;
}

.inline-statistics {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid rgba(223, 231, 242, 0.86);
}

.stat-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  border-radius: 999px;
  border: 1px solid rgba(223, 231, 242, 0.96);
  background: rgba(255, 255, 255, 0.74);
  color: var(--host-text-secondary);
}

.stat-item--clickable {
  cursor: pointer;
  transition: all 0.2s ease;
}

.stat-item--clickable:hover,
.stat-item--clickable:focus,
.stat-item--clickable.is-active {
  border-color: rgba(65, 112, 245, 0.35);
  background: rgba(65, 112, 245, 0.08);
  color: var(--host-brand);
  box-shadow: 0 6px 16px rgba(65, 112, 245, 0.12);
  outline: none;
}

.stat-leading-icon {
  color: var(--host-brand);
}

.stat-icon {
  width: 16px;
  height: 16px;
  object-fit: contain;
}

.stat-text {
  font-size: 12px;
  font-weight: 600;
}

.table-card {
  padding: 18px;
}

.table-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 10px;
  padding-bottom: 14px;
  margin-bottom: 14px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.86);
}

.table-head h3 {
  margin: 0;
  color: var(--host-text-primary);
  font-size: 18px;
  font-weight: 700;
}

.table-head p {
  margin: 6px 0 0;
  color: var(--host-text-muted);
  font-size: 12px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.selection-pill {
  display: inline-flex;
  align-items: center;
  height: 30px;
  padding: 0 12px;
  border-radius: 999px;
  border: 1px solid rgba(76, 139, 245, 0.26);
  background: rgba(76, 139, 245, 0.1);
  color: var(--host-brand-active);
  font-size: 12px;
  font-weight: 600;
}

.pagination-section {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid rgba(223, 231, 242, 0.82);
}

.table-card :deep(.table-section) {
  margin-bottom: 0;
}

.table-card :deep(.host-table) {
  border: 1px solid rgba(223, 231, 242, 0.95);
  border-radius: 16px;
}

.table-card :deep(.host-table .el-table__header th) {
  background: rgba(242, 246, 252, 0.76);
  color: var(--host-text-secondary);
  font-weight: 600;
}

.table-card :deep(.host-table .el-table__row > td) {
  color: var(--host-text-secondary);
  border-bottom-color: rgba(223, 231, 242, 0.66);
}

.table-card :deep(.host-table .el-table__row:hover > td) {
  background: rgba(76, 139, 245, 0.06) !important;
}

.table-card :deep(.operation-btn-group) {
  gap: 6px;
  display: inline-flex;
  box-shadow: none;
}

.table-card :deep(.operation-btn) {
  width: 30px;
  height: 30px;
  min-height: 30px;
  padding: 0;
  border-radius: 10px;
  border: 1px solid rgba(223, 231, 242, 0.96);
  background: rgba(255, 255, 255, 0.9);
  box-shadow: none;
  color: #627085;
}

.table-card :deep(.operation-btn::before) {
  display: none;
}

.table-card :deep(.operation-btn:hover) {
  transform: translateY(-1px);
  border-color: rgba(76, 139, 245, 0.4);
  box-shadow: 0 8px 16px rgba(17, 24, 39, 0.08);
}

.table-card :deep(.operation-btn-detail),
.table-card :deep(.operation-btn-edit),
.table-card :deep(.operation-btn-file),
.table-card :deep(.operation-btn-monitor) {
  color: #5f6b7c;
}

.table-card :deep(.operation-btn-delete) {
  color: #d75d67;
  border-color: rgba(239, 107, 115, 0.38);
}

.table-card :deep(.el-table__fixed-right .el-table__fixed-body-wrapper) {
  overflow: visible;
}

.group-panel :deep(.group-tree-section) {
  width: 100%;
  margin-right: 0;
}

.group-panel :deep(.group-card) {
  height: 100%;
  border-radius: 22px;
  border: 1px solid rgba(223, 231, 242, 0.94);
  background: linear-gradient(135deg, rgba(245, 250, 255, 0.96), rgba(255, 253, 248, 0.9));
  box-shadow: 0 12px 34px rgba(17, 24, 39, 0.05);
}

.group-panel :deep(.group-card::before),
.group-panel :deep(.pulse-ring),
.group-panel :deep(.search-glow),
.group-panel :deep(.hover-effect) {
  display: none;
}

.group-panel :deep(.card-header) {
  padding: 16px 16px 12px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.86);
}

.group-panel :deep(.title-icon) {
  width: 30px;
  height: 30px;
}

.group-panel :deep(.main-icon) {
  font-size: 16px;
  color: var(--host-brand);
  background: rgba(76, 139, 245, 0.12);
  border-radius: 10px;
  padding: 6px;
}

.group-panel :deep(.title-content .title) {
  font-size: 16px;
  font-weight: 700;
  color: var(--host-text-primary);
  background: none;
  -webkit-text-fill-color: initial;
}

.group-panel :deep(.subtitle) {
  color: var(--host-text-muted);
  letter-spacing: 0.08em;
}

.group-panel :deep(.stats-indicator) {
  background: rgba(76, 139, 245, 0.1);
  border: 1px solid rgba(76, 139, 245, 0.22);
}

.group-panel :deep(.stats-text) {
  color: var(--host-brand-active);
}

.group-panel :deep(.search-container) {
  padding: 12px 16px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.82);
}

.group-panel :deep(.tech-input .el-input__wrapper) {
  border: 1px solid rgba(223, 231, 242, 0.95);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: none;
}

.group-panel :deep(.tech-input .el-input__wrapper:hover) {
  border-color: rgba(76, 139, 245, 0.35);
  background: rgba(255, 255, 255, 0.96);
}

.group-panel :deep(.tech-input .el-input__wrapper.is-focus) {
  border-color: rgba(76, 139, 245, 0.74);
  box-shadow: 0 0 0 3px rgba(76, 139, 245, 0.14);
  background: #fff;
}

.group-panel :deep(.tree-container) {
  padding: 8px 10px 10px;
}

.group-panel :deep(.tech-tree .el-tree-node__content) {
  height: 36px;
  border-radius: 12px;
}

.group-panel :deep(.tree-node) {
  border-radius: 12px;
}

.group-panel :deep(.tree-node:hover) {
  background: rgba(76, 139, 245, 0.08);
}

.group-panel :deep(.tech-tree .el-tree-node.is-current > .el-tree-node__content .tree-node) {
  background: rgba(76, 139, 245, 0.14);
}

.group-panel :deep(.node-label) {
  color: var(--host-text-secondary);
}

.group-panel :deep(.tech-tree .el-tree-node.is-current > .el-tree-node__content .node-label) {
  color: var(--host-brand-active);
}

.group-panel :deep(.context-menu) {
  border-radius: 14px;
  border: 1px solid rgba(223, 231, 242, 0.96);
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 12px 30px rgba(17, 24, 39, 0.12);
}

.group-panel :deep(.context-menu-item) {
  color: var(--host-text-secondary);
}

.group-panel :deep(.context-menu-item:hover) {
  background: rgba(76, 139, 245, 0.1);
  color: var(--host-brand-active);
}

:deep(.el-dialog .el-form-item) {
  margin-bottom: 12px;
}

:deep(.host-detail-drawer .el-drawer__header) {
  margin-bottom: 0;
  padding: 14px 18px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.94);
  background: var(--host-topbar-gradient);
}

:deep(.host-detail-drawer .el-drawer__title) {
  color: var(--host-text-primary);
  font-size: 18px;
  font-weight: 700;
}

:deep(.host-detail-drawer .el-drawer__body) {
  padding: 18px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.95), rgba(247, 250, 255, 0.9));
}

.host-detail-content {
  display: grid;
  gap: 16px;
}

.dashboard-section,
.detail-section {
  border-radius: 18px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  background: rgba(255, 255, 255, 0.78);
}

.dashboard-section {
  padding: 10px 14px;
}

.gauge-container {
  display: flex;
  gap: 12px;
  align-items: center;
}

.gauge-item {
  flex: 1;
  min-width: 0;
  height: 168px;
}

.detail-section {
  padding: 14px;
}

.detail-section-head {
  margin-bottom: 10px;
}

.detail-section-head h3 {
  margin: 0;
  color: var(--host-text-primary);
  font-size: 16px;
  font-weight: 700;
}

.detail-section-head--between {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.sync-button {
  border-radius: 12px;
}

.detail-descriptions :deep(.el-descriptions__label) {
  color: var(--host-text-secondary);
}

.detail-descriptions :deep(.el-descriptions__content) {
  color: var(--host-text-primary);
}

@media (max-width: 1320px) {
  .content-grid {
    grid-template-columns: 1fr;
  }

  .group-panel {
    order: 2;
  }

  .main-panel {
    order: 1;
  }
}

@media (max-width: 920px) {
  .cmdb-host-management {
    padding: 12px;
  }

  .page-card {
    border-radius: 22px;
    padding: 50px 16px 16px;
  }

  .page-header {
    margin: -50px -16px 14px;
    padding: 10px 16px 14px;
    flex-direction: column;
    align-items: stretch;
  }

  .page-header h2 {
    font-size: 26px;
  }

  .summary-chip-group {
    justify-content: flex-start;
  }

  .toolbar-form-actions-item {
    width: 100%;
    margin-left: 0;
  }

  .toolbar-actions {
    width: 100%;
  }

  .gauge-container {
    flex-direction: column;
  }

  .gauge-item {
    width: 100%;
    height: 160px;
  }
}
</style>
