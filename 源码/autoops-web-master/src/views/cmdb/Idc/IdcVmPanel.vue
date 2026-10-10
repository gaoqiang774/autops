<template>
  <div class="idc-vm-panel">
    <div class="toolbar">
      <div class="toolbar-main">
        <el-form :inline="true" :model="queryParams" class="toolbar-form" @submit.prevent>
          <el-form-item label="关键字">
            <el-input
              v-model="queryParams.keyword"
              size="small"
              clearable
              placeholder="物理机 IP / 虚拟机 IP / 名称 / 登录用户"
              style="width: 260px"
              @keyup.enter="handleQuery"
            />
          </el-form-item>
        </el-form>

        <div class="toolbar-tags">
          <span>当前筛选</span>
          <el-tag type="primary" effect="light">虚拟机视图</el-tag>
          <el-tag v-if="queryParams.physicalIp" type="success" closable @close="clearPhysicalFilter">
            物理机：{{ queryParams.physicalIp }}
          </el-tag>
          <el-tag v-if="queryParams.keyword" type="info">关键字：{{ queryParams.keyword }}</el-tag>
        </div>

        <div class="toolbar-actions">
          <el-button size="small" type="primary" @click="handleQuery">
            <el-icon><Search /></el-icon>查询
          </el-button>
          <el-button size="small" type="warning" @click="resetQuery">
            <el-icon><RefreshRight /></el-icon>重置
          </el-button>
          <el-button size="small" type="success" @click="openVmDialog('create')">
            <el-icon><Plus /></el-icon>新增虚拟机
          </el-button>
          <el-button size="small" type="primary" plain @click="openImportDialog">
            <el-icon><Upload /></el-icon>导入虚拟机
          </el-button>
        </div>
      </div>
      <div class="toolbar-divider"></div>
      <div class="view-tabs">
        <button
          type="button"
          class="view-tab"
          :class="{ active: activeView === 'room' }"
          @click="$emit('switch-view', 'room')"
        >
          机房
        </button>
        <button
          type="button"
          class="view-tab"
          :class="{ active: activeView === 'cabinet' }"
          @click="$emit('switch-view', 'cabinet')"
        >
          机柜
        </button>
        <button
          type="button"
          class="view-tab"
          :class="{ active: activeView === 'physical' }"
          @click="$emit('switch-view', 'physical')"
        >
          物理机
        </button>
        <button
          type="button"
          class="view-tab"
          :class="{ active: activeView === 'vm' }"
          @click="$emit('switch-view', 'vm')"
        >
          虚拟机
        </button>
        <button
          type="button"
          class="view-tab"
          :class="{ active: activeView === 'hardware' }"
          @click="$emit('switch-view', 'hardware')"
        >
          硬件监控
        </button>
      </div>
    </div>

    <div class="table-card">
      <div class="table-head">
        <div>
          <h3>虚拟机资产</h3>
          <p>IDC Virtual Machines</p>
        </div>
        <el-button text type="primary" @click="loadVmList">刷新列表</el-button>
      </div>

      <el-table v-loading="loadingTable" :data="vmList" border stripe row-key="id" class="vm-table">
        <el-table-column type="index" label="#" width="56" />
        <el-table-column prop="vmName" label="虚拟机名称" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">
            <div class="copy-hover-cell">
              <img
                src="@/assets/image/虚拟机.svg"
                alt="虚拟机"
                class="vm-type-icon"
              >
              <el-button
                link
                type="primary"
                class="link-button"
                :disabled="!row.vmName"
                @click="openHostDetail(row)"
              >
                {{ row.vmName || '-' }}
              </el-button>
              <el-icon
                v-if="row.vmName"
                class="copy-trigger"
                @click.stop="copyText(row.vmName, '虚拟机名称')"
              >
                <DocumentCopy />
              </el-icon>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="vmIp" label="虚拟机 IP" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">
            <div class="copy-hover-cell">
              <el-button
                link
                type="primary"
                class="link-button"
                :disabled="!row.vmIp"
                @click="openServiceDialog(row)"
              >
                {{ row.vmIp || '-' }}
              </el-button>
              <el-icon
                v-if="row.vmIp"
                class="copy-trigger"
                @click.stop="copyText(row.vmIp, '虚拟机IP')"
              >
                <DocumentCopy />
              </el-icon>
            </div>
          </template>
        </el-table-column>

        <el-table-column prop="physicalIp" label="物理机 IP" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">
            <div class="copy-hover-cell">
              <span>{{ row.physicalIp || '-' }}</span>
              <el-icon
                v-if="row.physicalIp"
                class="copy-trigger"
                @click.stop="copyText(row.physicalIp, '物理机IP')"
              >
                <DocumentCopy />
              </el-icon>
            </div>
          </template>
        </el-table-column>

        <el-table-column prop="username" label="登录用户" min-width="120" show-overflow-tooltip />
        <el-table-column prop="remark" label="备注" min-width="220" show-overflow-tooltip />
        <el-table-column label="更新时间" min-width="170">
          <template #default="{ row }">{{ formatDateTime(row.updateTime || row.createTime) }}</template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" width="140">
          <template #default="{ row }">
            <el-button link type="primary" @click="openVmDialog('edit', row)">编辑</el-button>
            <el-button link type="danger" @click="removeVm(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <div class="pagination">
        <el-pagination
          :current-page="queryParams.page"
          :page-size="queryParams.pageSize"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          :total="total"
          @size-change="handleSizeChange"
          @current-change="handleCurrentChange"
        />
      </div>
    </div>

    <el-dialog
      v-model="vmDialogVisible"
      :title="vmDialogMode === 'create' ? '新增虚拟机关联' : '编辑虚拟机关联'"
      width="720px"
      @closed="resetVmDialog"
    >
      <el-form ref="vmFormRef" :model="vmForm" :rules="vmRules" label-width="110px">
        <el-row :gutter="18">
          <el-col :span="12">
            <el-form-item label="物理机 IP" prop="physicalIp">
              <el-input v-model="vmForm.physicalIp" placeholder="请输入物理机 IP" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="虚拟机 IP" prop="vmIp">
              <el-input v-model="vmForm.vmIp" placeholder="请输入虚拟机 IP" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="18">
          <el-col :span="12">
            <el-form-item label="登录用户">
              <el-input v-model="vmForm.username" placeholder="请输入登录用户名" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="虚拟机名称">
              <el-input v-model="vmForm.vmName" placeholder="请输入虚拟机名称" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="备注">
          <el-input v-model="vmForm.remark" type="textarea" :rows="4" placeholder="请输入备注" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="vmDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitVm">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="importDialogVisible" title="导入虚拟机资产" width="520px" @closed="resetImportDialog">
      <div class="import-box">
        <div class="import-tip">请先下载模板，填写物理机 IP 与虚拟机信息后再导入。</div>
        <el-button type="primary" plain :loading="templateLoading" @click="downloadTemplate">
          <el-icon><Download /></el-icon>下载导入模板
        </el-button>
        <el-upload drag :auto-upload="false" :show-file-list="false" accept=".xlsx,.xls" :on-change="handleImportFileChange">
          <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
          <div class="el-upload__text">拖拽文件到此处，或点击选择 Excel 文件</div>
        </el-upload>
        <div v-if="importForm.file" class="file-line">
          <el-icon><Document /></el-icon>
          <span>{{ importForm.file.name }}</span>
          <el-button link type="danger" @click="handleImportFileRemove">移除</el-button>
        </div>
        <el-progress v-if="importUploading" :percentage="importProgress" :status="importProgress === 100 ? 'success' : ''" />
      </div>
      <template #footer>
        <el-button @click="importDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="importUploading" :disabled="!importForm.file" @click="submitImport">开始导入</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="serviceDialogVisible"
      :title="serviceDialogTitle"
      width="760px"
      @closed="resetServiceDialog"
    >
      <div v-loading="serviceDialogLoading" class="service-dialog-body">
        <div class="service-summary">
          <div class="service-summary-item">
            <span class="service-summary-label">虚拟机 IP</span>
            <strong class="service-summary-value">{{ serviceDialog.ip || '-' }}</strong>
          </div>
          <div class="service-summary-item">
            <span class="service-summary-label">主机 ID</span>
            <strong class="service-summary-value">{{ serviceDialog.hostId || '-' }}</strong>
          </div>
          <div class="service-summary-item">
            <span class="service-summary-label">服务数量</span>
            <strong class="service-summary-value">{{ serviceDialog.services.length }}</strong>
          </div>
          <div class="service-summary-item">
            <span class="service-summary-label">更新时间</span>
            <strong class="service-summary-value">{{ formatUnixDateTime(serviceDialog.updateTime) }}</strong>
          </div>
        </div>

        <el-empty
          v-if="!serviceDialogLoading && !serviceDialog.services.length"
          description="当前主机未获取到服务信息"
        />

        <el-table v-else :data="serviceDialog.services" border stripe class="service-table">
          <el-table-column type="index" label="#" width="56" />
          <el-table-column prop="name" label="服务名称" min-width="220" show-overflow-tooltip />
          <el-table-column label="CPU" width="120" align="center">
            <template #default="{ row }">
              <el-tag :type="getServiceMetricTagType(row.cpu, 5, 20)" effect="light">
                {{ formatPercent(row.cpu) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="内存" width="120" align="center">
            <template #default="{ row }">
              <el-tag :type="getServiceMetricTagType(row.memory, 10, 30)" effect="light">
                {{ formatPercent(row.memory) }}
              </el-tag>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-dialog>
  </div>
</template>

<script>
const makeVmForm = (filter = {}) => ({
  id: null,
  physicalIp: filter.physicalIp || '',
  vmIp: '',
  username: '',
  vmName: '',
  remark: ''
})

const makeQueryParams = (filter = {}) => ({
  page: 1,
  pageSize: 10,
  keyword: '',
  physicalAssetId: filter.physicalAssetId || '',
  physicalIp: filter.physicalIp || ''
})

const makeServiceDialog = () => ({
  ip: '',
  hostId: null,
  hostName: '',
  services: [],
  updateTime: ''
})

export default {
  name: 'IdcVmPanel',
  emits: ['switch-view'],
  props: {
    initialFilter: {
      type: Object,
      default: () => ({})
    },
    activeView: {
      type: String,
      default: 'vm'
    }
  },
  data() {
    const baseFilter = this.normalizeFilter(this.initialFilter)
    return {
      baseFilter,
      loadingTable: false,
      submitting: false,
      templateLoading: false,
      importUploading: false,
      importProgress: 0,
      queryParams: makeQueryParams(baseFilter),
      vmList: [],
      total: 0,
      vmDialogVisible: false,
      vmDialogMode: 'create',
      vmForm: makeVmForm(baseFilter),
      vmRules: {
        physicalIp: [{ required: true, message: '请输入物理机 IP', trigger: 'blur' }],
        vmIp: [{ required: true, message: '请输入虚拟机 IP', trigger: 'blur' }]
      },
      importDialogVisible: false,
      importForm: { file: null },
      serviceDialogVisible: false,
      serviceDialogLoading: false,
      serviceDialog: makeServiceDialog()
    }
  },
  computed: {
    serviceDialogTitle() {
      return this.serviceDialog.hostName ? `${this.serviceDialog.hostName} 服务详情` : '服务详情'
    }
  },
  created() {
    this.loadVmList()
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
      for (const key of ['list', 'items', 'rows', 'records', 'result']) {
        if (Array.isArray(payload[key])) return payload[key]
      }
      return []
    },
    page(payload) {
      const list = this.list(payload)
      return {
        list,
        total: this.num(this.pick(payload, ['total', 'count', 'totalCount', 'total_count'], list.length), list.length)
      }
    },
    compact(payload) {
      return Object.keys(payload).reduce((acc, key) => {
        const value = payload[key]
        if (value === '' || value === undefined || value === null) return acc
        acc[key] = typeof value === 'string' ? value.trim() : value
        return acc
      }, {})
    },
    normalizeFilter(filter = {}) {
      return {
        physicalAssetId: this.num(this.pick(filter, ['physicalAssetId']), ''),
        physicalIp: String(this.pick(filter, ['physicalIp'], ''))
      }
    },
    normalizeVm(item) {
      return {
        ...item,
        id: this.num(this.pick(item, ['id']), 0),
        physicalAssetId: this.num(this.pick(item, ['physicalAssetId', 'physical_asset_id']), ''),
        physicalIp: String(this.pick(item, ['physicalIp', 'physical_ip'], '')),
        vmIp: String(this.pick(item, ['vmIp', 'vm_ip'], '')),
        username: String(this.pick(item, ['username'], '')),
        vmName: String(this.pick(item, ['vmName', 'vm_name'], '')),
        remark: String(this.pick(item, ['remark'], '')),
        createTime: this.pick(item, ['createTime', 'create_time'], ''),
        updateTime: this.pick(item, ['updateTime', 'update_time'], '')
      }
    },
    normalizeHost(item) {
      return {
        id: this.num(this.pick(item, ['id', 'hostId']), null),
        hostName: String(this.pick(item, ['hostName', 'host_name', 'name'], '')),
        ip: String(this.pick(item, ['ip', 'privateIp', 'private_ip', 'sshIp', 'ssh_ip'], ''))
      }
    },
    normalizeService(item) {
      return {
        name: String(this.pick(item, ['name', 'serviceName', 'service_name'], '')),
        cpu: this.num(this.pick(item, ['cpu', 'cpuPercent', 'cpu_percent'], 0), 0),
        memory: this.num(this.pick(item, ['memory', 'memoryPercent', 'memory_percent', 'memPercent', 'mem_percent'], 0), 0)
      }
    },
    extractHostList(payload) {
      const list = this.list(payload)
      if (list.length) return list
      if (payload && typeof payload === 'object' && this.pick(payload, ['id', 'hostId'], null) !== null) {
        return [payload]
      }
      return []
    },
    formatDateTime(value) {
      return value ? String(value).replace('T', ' ').slice(0, 19) : '-'
    },
    formatUnixDateTime(value) {
      const timestamp = Number(value)
      if (!Number.isFinite(timestamp) || timestamp <= 0) return '-'
      const date = new Date(timestamp > 1000000000000 ? timestamp : timestamp * 1000)
      if (Number.isNaN(date.getTime())) return '-'
      const pad = item => String(item).padStart(2, '0')
      return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
    },
    formatPercent(value) {
      const num = Number(value)
      if (!Number.isFinite(num)) return '0.0%'
      return `${num.toFixed(1)}%`
    },
    getServiceMetricTagType(value, warning = 10, danger = 30) {
      const num = Number(value)
      if (!Number.isFinite(num) || num < warning) return 'success'
      if (num < danger) return 'warning'
      return 'danger'
    },
    async copyText(value, label = '内容') {
      const text = String(value || '').trim()
      if (!text) {
        this.$message.warning(`${label}为空，无法复制`)
        return
      }

      try {
        if (navigator.clipboard && window.isSecureContext) {
          await navigator.clipboard.writeText(text)
        } else {
          const textArea = document.createElement('textarea')
          textArea.value = text
          textArea.setAttribute('readonly', 'readonly')
          textArea.style.position = 'fixed'
          textArea.style.left = '-9999px'
          document.body.appendChild(textArea)
          textArea.select()
          document.execCommand('copy')
          document.body.removeChild(textArea)
        }
        this.$message.success(`${label}已复制`)
      } catch (error) {
        console.error('copyText failed:', error)
        this.$message.error('复制失败，请手动复制')
      }
    },
    async queryHostByIp(ip) {
      const { data: hostRes } = await this.$api.GetCmdbHostsByIP(ip, {
        page: 1,
        pageSize: 10
      })
      if (hostRes.code !== 200) throw new Error(hostRes.message || '通过 IP 查询主机失败')

      const hosts = this.extractHostList(hostRes.data).map(item => this.normalizeHost(item))
      return hosts.find(item => item.ip === ip) || hosts[0] || null
    },
    async loadVmList() {
      this.loadingTable = true
      try {
        const params = {
          page: this.queryParams.page,
          pageSize: this.queryParams.pageSize,
          sortBy: 'id',
          sortOrder: 'desc'
        }
        if (this.queryParams.keyword) params.keyword = this.queryParams.keyword
        if (this.queryParams.physicalAssetId) params.physicalAssetId = this.queryParams.physicalAssetId
        if (this.queryParams.physicalIp) params.physicalIp = this.queryParams.physicalIp

        const { data: res } = await this.$api.getCmdbIdcVmList(params)
        if (res.code !== 200) throw new Error(res.message || '获取虚拟机列表失败')

        const page = this.page(res.data)
        this.vmList = page.list.map(item => this.normalizeVm(item))
        this.total = page.total
      } catch (error) {
        console.error('loadVmList failed:', error)
        this.vmList = []
        this.total = 0
        this.$message.error(error.message || '获取虚拟机列表失败')
      } finally {
        this.loadingTable = false
      }
    },
    handleQuery() {
      this.queryParams.page = 1
      this.loadVmList()
    },
    resetQuery() {
      this.queryParams = makeQueryParams(this.baseFilter)
      this.loadVmList()
    },
    clearPhysicalFilter() {
      this.baseFilter = { physicalAssetId: '', physicalIp: '' }
      this.queryParams.page = 1
      this.queryParams.physicalAssetId = ''
      this.queryParams.physicalIp = ''
      this.loadVmList()
    },
    handleSizeChange(size) {
      this.queryParams.page = 1
      this.queryParams.pageSize = size
      this.loadVmList()
    },
    handleCurrentChange(page) {
      this.queryParams.page = page
      this.loadVmList()
    },
    openVmDialog(mode, row) {
      this.vmDialogMode = mode
      this.vmDialogVisible = true
      if (mode === 'create') {
        this.vmForm = makeVmForm(this.baseFilter)
        this.$nextTick(() => this.$refs.vmFormRef?.clearValidate())
        return
      }
      this.loadVmDetail(row.id)
    },
    resetVmDialog() {
      this.vmForm = makeVmForm(this.baseFilter)
      this.$refs.vmFormRef?.clearValidate()
    },
    async loadVmDetail(id) {
      try {
        this.submitting = true
        const { data: res } = await this.$api.getCmdbIdcVmInfo(id)
        if (res.code !== 200) throw new Error(res.message || '获取虚拟机详情失败')

        const detail = this.normalizeVm(res.data || {})
        this.vmForm = {
          id: detail.id,
          physicalIp: detail.physicalIp || '',
          vmIp: detail.vmIp || '',
          username: detail.username || '',
          vmName: detail.vmName || '',
          remark: detail.remark || ''
        }
        this.$nextTick(() => this.$refs.vmFormRef?.clearValidate())
      } catch (error) {
        console.error('loadVmDetail failed:', error)
        this.$message.error(error.message || '获取虚拟机详情失败')
        this.vmDialogVisible = false
      } finally {
        this.submitting = false
      }
    },
    async submitVm() {
      try {
        await this.$refs.vmFormRef.validate()
        this.submitting = true

        const payload = this.compact({
          physicalIp: this.vmForm.physicalIp,
          vmIp: this.vmForm.vmIp,
          username: this.vmForm.username,
          vmName: this.vmForm.vmName,
          remark: this.vmForm.remark
        })

        const response = this.vmDialogMode === 'create'
          ? await this.$api.createCmdbIdcVm(payload)
          : await this.$api.updateCmdbIdcVm({ id: this.vmForm.id, ...payload })

        if (response.data.code !== 200) throw new Error(response.data.message || '保存虚拟机关联失败')

        this.$message.success(this.vmDialogMode === 'create' ? '虚拟机关联创建成功' : '虚拟机关联更新成功')
        this.vmDialogVisible = false
        await this.loadVmList()
      } catch (error) {
        if (error !== false) {
          console.error('submitVm failed:', error)
          this.$message.error(error.message || '保存虚拟机关联失败')
        }
      } finally {
        this.submitting = false
      }
    },
    async removeVm(row) {
      try {
        await this.$confirm(`确认删除虚拟机关联“${row.vmIp || row.vmName || row.id}”吗？`, '删除虚拟机', {
          type: 'warning'
        })

        const { data: res } = await this.$api.deleteCmdbIdcVm(row.id)
        if (res.code !== 200) throw new Error(res.message || '删除虚拟机关联失败')

        this.$message.success('虚拟机关联删除成功')
        if (this.vmList.length === 1 && this.queryParams.page > 1) this.queryParams.page -= 1
        await this.loadVmList()
      } catch (error) {
        if (error !== 'cancel' && error !== 'close') {
          console.error('removeVm failed:', error)
          this.$message.error(error.message || '删除虚拟机关联失败')
        }
      }
    },
    openImportDialog() {
      this.importDialogVisible = true
      this.importProgress = 0
    },
    resetImportDialog() {
      this.importForm.file = null
      this.importProgress = 0
      this.importUploading = false
    },
    resetServiceDialog() {
      this.serviceDialog = makeServiceDialog()
      this.serviceDialogLoading = false
    },
    async openHostDetail(row) {
      const ip = String(row?.vmIp || '').trim()
      if (!ip) {
        this.$message.warning('当前虚拟机 IP 为空')
        return
      }

      try {
        const matchedHost = await this.queryHostByIp(ip)
        if (!matchedHost || !matchedHost.id) {
          this.$message.warning(`未在 host 表中找到 IP 为 ${ip} 的主机`)
          return
        }
        const path = Number(matchedHost?.hostType ?? matchedHost?.host_type) === 2
          ? `/cmdb/host/windows-detail/${matchedHost.id}`
          : `/cmdb/host/detail/${matchedHost.id}`
        this.$router.push(path)
      } catch (error) {
        console.error('openHostDetail failed:', error)
        this.$message.error(error.message || '跳转主机详情失败')
      }
    },
    async openServiceDialog(row) {
      const ip = String(row?.vmIp || '').trim()
      if (!ip) {
        this.$message.warning('当前虚拟机 IP 为空')
        return
      }

      this.serviceDialogVisible = true
      this.serviceDialogLoading = true
      this.serviceDialog = {
        ip,
        hostId: null,
        hostName: row?.vmName || '',
        services: [],
        updateTime: ''
      }

      try {
        const matchedHost = await this.queryHostByIp(ip)
        if (!matchedHost || !matchedHost.id) {
          this.$message.warning(`未在 host 表中找到 IP 为 ${ip} 的主机`)
          return
        }

        const { data: detailRes } = await this.$api.getCompleteHostMetrics(matchedHost.id)
        if (detailRes.code !== 200) throw new Error(detailRes.message || '获取主机服务信息失败')

        const detail = detailRes.data || {}
        const services = Array.isArray(detail.services) ? detail.services.map(item => this.normalizeService(item)) : []

        this.serviceDialog = {
          ip,
          hostId: matchedHost.id,
          hostName: String(detail.hostName || matchedHost.hostName || row?.vmName || ''),
          services,
          updateTime: detail.updateTime || ''
        }
      } catch (error) {
        console.error('openServiceDialog failed:', error)
        this.$message.error(error.message || '获取虚拟机服务列表失败')
      } finally {
        this.serviceDialogLoading = false
      }
    },
    handleImportFileChange(file) {
      const raw = file.raw || file
      if (!/\.(xlsx|xls)$/i.test(raw.name || '')) {
        this.$message.warning('请上传 Excel 文件')
        return
      }
      this.importForm.file = raw
    },
    handleImportFileRemove() {
      this.importForm.file = null
      this.importProgress = 0
    },
    async submitImport() {
      if (!this.importForm.file) {
        this.$message.warning('请先选择 Excel 文件')
        return
      }

      this.importUploading = true
      this.importProgress = 0
      try {
        const formData = new FormData()
        formData.append('file', this.importForm.file)
        const { data: res } = await this.$api.importCmdbIdcVmsFromExcel(formData, {
          onUploadProgress: event => {
            if (event.total) this.importProgress = Math.round((event.loaded * 100) / event.total)
          }
        })
        if (res.code !== 200) throw new Error(res.message || '导入虚拟机资产失败')

        this.importProgress = 100
        this.$message.success('虚拟机资产导入成功')
        this.importDialogVisible = false
        await this.loadVmList()
      } catch (error) {
        console.error('submitImport failed:', error)
        this.$message.error(error.message || '导入虚拟机资产失败')
      } finally {
        this.importUploading = false
      }
    },
    async downloadTemplate() {
      this.templateLoading = true
      try {
        const response = await this.$api.downloadCmdbIdcVmTemplate()
        const url = window.URL.createObjectURL(new Blob([response.data]))
        const link = document.createElement('a')
        link.href = url
        link.download = 'idc_vm_template.xlsx'
        document.body.appendChild(link)
        link.click()
        document.body.removeChild(link)
        window.URL.revokeObjectURL(url)
        this.$message.success('模板下载成功')
      } catch (error) {
        console.error('downloadTemplate failed:', error)
        this.$message.error(error.message || '模板下载失败')
      } finally {
        this.templateLoading = false
      }
    }
  }
}
</script>

<style scoped>
.idc-vm-panel {
  min-width: 0;
}

.toolbar,
.table-card {
  background: #fff;
  border-radius: 18px;
  box-shadow: 0 16px 36px rgba(15, 23, 42, 0.08);
}

.toolbar {
  margin-bottom: 20px;
  padding: 14px 16px;
}

.toolbar-main {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: nowrap;
  overflow-x: auto;
}

.toolbar-form {
  flex: 0 0 auto;
}

.toolbar-tags {
  display: flex;
  flex: 1 1 auto;
  align-items: center;
  gap: 8px;
  min-width: 0;
  white-space: nowrap;
}

.toolbar-actions {
  display: flex;
  gap: 6px;
  flex-wrap: nowrap;
  flex: 0 0 auto;
}

.toolbar-divider {
  height: 1px;
  margin: 14px -16px 0;
  background: #e5e7eb;
}

.view-tabs {
  display: flex;
  align-items: center;
  gap: 24px;
  padding-top: 14px;
}

.view-tab {
  padding: 0 0 10px;
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

.table-card {
  padding: 20px;
}

.table-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.table-head h3 {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  color: #1f2937;
}

.table-head p {
  margin: 6px 0 0;
  font-size: 12px;
  color: #8a94a6;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
}

.import-box {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.import-tip {
  font-size: 12px;
  color: #8a94a6;
  line-height: 1.6;
}

.file-line {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #4b5563;
}

.link-button {
  height: auto;
  padding: 0;
  font-weight: 600;
}

.copy-hover-cell {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
}

.vm-type-icon {
  width: 16px;
  height: 16px;
  flex: 0 0 16px;
  object-fit: contain;
}

.copy-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  color: #4d79d8;
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.18s ease, color 0.18s ease;
}

.copy-hover-cell:hover .copy-trigger {
  opacity: 1;
}

.copy-trigger:hover {
  color: #4c8bf5;
}

.service-dialog-body {
  min-height: 220px;
}

.service-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.service-summary-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 14px;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  background: #f8fafc;
}

.service-summary-label {
  font-size: 12px;
  color: #6b7280;
}

.service-summary-value {
  font-size: 14px;
  font-weight: 600;
  color: #111827;
  word-break: break-all;
}

.service-table {
  width: 100%;
}

:deep(.toolbar .el-form-item) {
  margin-bottom: 0;
  margin-right: 0;
}

:deep(.toolbar .el-form-item__label) {
  padding-right: 8px;
  font-size: 13px;
}

:deep(.toolbar .el-button) {
  padding: 7px 12px;
}

@media (max-width: 1200px) {
  .toolbar-main {
    flex-wrap: wrap;
  }

  .toolbar-tags {
    flex-basis: 100%;
    white-space: normal;
  }

  .toolbar-actions {
    flex-wrap: wrap;
  }

  .view-tabs {
    gap: 16px;
    overflow-x: auto;
  }

  .service-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .service-summary {
    grid-template-columns: 1fr;
  }
}
</style>
