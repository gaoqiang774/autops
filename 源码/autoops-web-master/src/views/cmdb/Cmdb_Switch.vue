<template>
  <div class="cmdb-snmp-management">
    <header class="switch-hero-panel">
      <div class="switch-hero-copy">
        <span class="switch-hero-icon" aria-hidden="true">
          <img :src="switchDeviceIcon" alt="" />
        </span>
        <div class="switch-hero-content">
          <h1>交换机资产管理</h1>
          <span class="switch-hero-divider" aria-hidden="true"></span>
          <p>统一维护交换机资产、SNMP采集入口与运行状态。</p>
        </div>
      </div>
      <div class="switch-hero-metrics" aria-label="交换机资产摘要">
        <div>
          <span>交换机</span>
          <strong>{{ total }}</strong>
        </div>
        <div>
          <span>当前页</span>
          <strong>{{ deviceList.length }}</strong>
        </div>
        <div>
          <span>厂商</span>
          <strong>{{ vendorCount }}</strong>
        </div>
      </div>
    </header>

    <el-card shadow="never" class="snmp-card">
      <div class="switch-toolbar">
        <div class="switch-toolbar-main">
          <el-form :inline="true" :model="queryParams" class="search-form" @submit.prevent>
            <el-form-item label="关键词">
              <el-input
                v-model="queryParams.keyword"
                placeholder="设备名称 / IP / 厂商 / 型号"
                clearable
                size="small"
                class="keyword-input"
                @keyup.enter="handleQuery"
              />
            </el-form-item>
            <el-form-item class="query-actions">
              <el-button type="primary" size="small" @click="handleQuery">
                <el-icon><Search /></el-icon>
                查询
              </el-button>
              <el-button type="warning" size="small" @click="resetQuery">
                <el-icon><Refresh /></el-icon>
                重置
              </el-button>
            </el-form-item>
          </el-form>

          <div class="toolbar-actions">
            <el-button
              type="success"
              size="small"
              v-authority="['cmdb:snmp:add']"
              @click="addDeviceDialogVisible = true">
              <el-icon><Plus /></el-icon>
              新增交换机
            </el-button>
            <el-button
              type="warning"
              size="small"
              class="switch-refresh-button"
              @click="handleRefresh">
              <el-icon><Refresh /></el-icon>
              刷新列表
            </el-button>
          </div>

          <div class="toolbar-context">
            <span>当前结果</span>
            <el-tag type="primary" effect="light">{{ total }} 台设备</el-tag>
          </div>
        </div>
      </div>

      <div class="table-section table-shell">
        <el-table
          :data="deviceList"
          v-loading="loading"
          stripe
          style="width: 100%"
          class="device-table">
          <el-table-column label="设备名称" prop="deviceName" min-width="180">
            <template #default="{ row }">
              <div class="device-name-cell">
                <img :src="switchDeviceIcon" alt="交换机" class="device-name-icon" />
                <span class="device-name">{{ row.deviceName }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="IP地址" prop="ipAddress" width="140" />
          <el-table-column label="厂商" prop="deviceVendor" width="120">
            <template #default="{ row }">
              <div class="vendor-cell">
                <img
                  v-if="getVendorIcon(row.deviceVendor)"
                  :src="getVendorIcon(row.deviceVendor)"
                  :alt="row.deviceVendor"
                  class="vendor-icon"
                />
                <span>{{ row.deviceVendor || '-' }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="型号" prop="deviceModel" min-width="160" show-overflow-tooltip>
            <template #default="{ row }">
              <span>{{ row.deviceModel || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="模板" min-width="160" show-overflow-tooltip>
            <template #default="{ row }">
              <span>{{ row.monitorTemplate?.name || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="位置" prop="location" min-width="140" show-overflow-tooltip>
            <template #default="{ row }">
              <span>{{ row.location || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="负责人" prop="owner" min-width="100" show-overflow-tooltip>
            <template #default="{ row }">
              <span>{{ row.owner || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="180" fixed="right">
            <template #default="{ row }">
              <div class="operation-links">
                <el-button link type="primary" @click="handleOpenMonitor(row)" v-authority="['cmdb:snmp:collect']">监控</el-button>
                <el-button link class="operation-link-warning" @click="showEditDeviceDialog(row.id)" v-authority="['cmdb:snmp:update']">编辑</el-button>
                <el-button link type="danger" @click="handleDeviceDelete(row)" v-authority="['cmdb:snmp:delete']">删除</el-button>
              </div>
            </template>
          </el-table-column>
          <template #empty>
            <el-empty description="暂无交换机资产" :image-size="64" />
          </template>
        </el-table>
      </div>

      <div class="pagination-section">
        <el-pagination
          v-model:current-page="queryParams.page"
          v-model:page-size="queryParams.pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSizeChange"
          @current-change="handleCurrentChange"
        />
      </div>
    </el-card>

    <el-dialog
      title="新增交换机"
      v-model="addDeviceDialogVisible"
      width="760px"
      class="switch-form-dialog"
      modal-class="switch-form-dialog-overlay"
      destroy-on-close
      @close="addDeviceDialogClosed">
      <el-form :model="addDeviceForm" :rules="deviceFormRules" ref="addDeviceFormRef" label-position="top" class="switch-device-form">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="设备名称" prop="deviceName">
              <el-input v-model="addDeviceForm.deviceName" placeholder="例如: core-sw-01" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="IP地址" prop="ipAddress">
              <el-input v-model="addDeviceForm.ipAddress" placeholder="例如: 10.10.10.1" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="SNMP URL" prop="snmpUrl">
          <el-input
            v-model="addDeviceForm.snmpUrl"
            placeholder="例如: http://172.16.1.50:9000/api/snmp/devices/10.10.10.1/monitor"
          />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="厂商" prop="deviceVendor">
              <el-select
                v-model="addDeviceForm.deviceVendor"
                placeholder="请选择厂商"
                style="width: 100%">
                <template #prefix>
                  <img
                    v-if="getVendorIcon(addDeviceForm.deviceVendor)"
                    :src="getVendorIcon(addDeviceForm.deviceVendor)"
                    :alt="addDeviceForm.deviceVendor"
                    class="vendor-select-icon"
                  />
                </template>
                <el-option
                  v-for="option in deviceVendorOptions"
                  :key="option.value"
                  :label="option.label"
                  :value="option.value">
                  <div class="vendor-option">
                    <img :src="option.icon" :alt="option.label" class="vendor-option-icon" />
                    <span>{{ option.label }}</span>
                  </div>
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="型号" prop="deviceModel">
              <el-input v-model="addDeviceForm.deviceModel" placeholder="例如: S6730-H24X6C" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-divider content-position="left">业务扩展</el-divider>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="位置" prop="location">
              <el-input v-model="addDeviceForm.location" placeholder="例如: 机房A-01" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="负责人" prop="owner">
              <el-input v-model="addDeviceForm.owner" placeholder="例如: 张三" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="备注" prop="remark">
          <el-input v-model="addDeviceForm.remark" type="textarea" :rows="2" placeholder="例如: 核心交换机" />
        </el-form-item>

        <el-divider content-position="left">SSH 扩展</el-divider>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="SSH用户" prop="sshName" label-width="70px">
              <el-input v-model="addDeviceForm.sshName" placeholder="例如: admin" size="small" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="密码" prop="sshPasswd" label-width="50px">
              <el-input v-model="addDeviceForm.sshPasswd" type="password" show-password placeholder="密码" size="small" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="SSH端口" prop="sshPort" label-width="70px">
              <el-input-number v-model="addDeviceForm.sshPort" :min="1" :max="65535" placeholder="22" size="small" style="width: 100%" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="addDeviceDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="addSubmitting" @click="addDevice">保存</el-button>
        </span>
      </template>
    </el-dialog>

    <el-dialog
      title="编辑交换机"
      v-model="editDeviceDialogVisible"
      width="760px"
      class="switch-form-dialog"
      modal-class="switch-form-dialog-overlay"
      destroy-on-close>
      <el-form :model="deviceInfo" :rules="deviceFormRules" ref="editDeviceFormRef" label-position="top" class="switch-device-form">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="设备名称" prop="deviceName">
              <el-input v-model="deviceInfo.deviceName" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="IP地址" prop="ipAddress">
              <el-input v-model="deviceInfo.ipAddress" placeholder="例如: 10.10.10.1" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="SNMP URL" prop="snmpUrl">
          <el-input
            v-model="deviceInfo.snmpUrl"
            placeholder="例如: http://172.16.1.50:9000/api/snmp/devices/10.10.10.1/monitor"
          />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="厂商" prop="deviceVendor">
              <el-select
                v-model="deviceInfo.deviceVendor"
                placeholder="请选择厂商"
                style="width: 100%">
                <template #prefix>
                  <img
                    v-if="getVendorIcon(deviceInfo.deviceVendor)"
                    :src="getVendorIcon(deviceInfo.deviceVendor)"
                    :alt="deviceInfo.deviceVendor"
                    class="vendor-select-icon"
                  />
                </template>
                <el-option
                  v-for="option in deviceVendorOptions"
                  :key="option.value"
                  :label="option.label"
                  :value="option.value">
                  <div class="vendor-option">
                    <img :src="option.icon" :alt="option.label" class="vendor-option-icon" />
                    <span>{{ option.label }}</span>
                  </div>
                </el-option>
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="型号" prop="deviceModel">
              <el-input v-model="deviceInfo.deviceModel" placeholder="例如: S6730-H24X6C" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-divider content-position="left">业务扩展</el-divider>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="位置" prop="location">
              <el-input v-model="deviceInfo.location" placeholder="例如: 机房A-01" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="负责人" prop="owner">
              <el-input v-model="deviceInfo.owner" placeholder="例如: 张三" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="备注" prop="remark">
          <el-input v-model="deviceInfo.remark" type="textarea" :rows="2" placeholder="例如: 核心交换机" />
        </el-form-item>

        <el-divider content-position="left">SSH 扩展</el-divider>
        <el-row :gutter="16">
          <el-col :span="8">
            <el-form-item label="SSH用户" prop="sshName" label-width="70px">
              <el-input v-model="deviceInfo.sshName" placeholder="例如: admin" size="small" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="密码" prop="sshPasswd" label-width="50px">
              <el-input v-model="deviceInfo.sshPasswd" type="password" show-password placeholder="留空则不修改" size="small" />
            </el-form-item>
          </el-col>
          <el-col :span="8">
            <el-form-item label="SSH端口" prop="sshPort" label-width="70px">
              <el-input-number v-model="deviceInfo.sshPort" :min="1" :max="65535" placeholder="22" size="small" style="width: 100%" />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="editDeviceDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="editSubmitting" @click="editDevice">保存</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import API from '@/api/cmdb'
import {
  Search,
  Refresh,
  Plus
} from '@element-plus/icons-vue'
import HuaweiVendorIcon from '@/assets/image/华为.svg'
import CiscoVendorIcon from '@/assets/image/cisco.svg'
import H3CVendorIcon from '@/assets/image/H3C.svg'
import SwitchDeviceIcon from '@/assets/image/交换机.svg'

const createQueryParams = () => ({
  page: 1,
  pageSize: 10,
  keyword: '',
  status: ''
})

const createDeviceForm = () => ({
  deviceName: '',
  ipAddress: '',
  snmpUrl: '',
  deviceVendor: '',
  deviceModel: '',
  location: '',
  owner: '',
  remark: '',
  sshName: '',
  sshPasswd: '',
  sshPort: null
})

const deviceVendorOptions = [
  { label: 'H3C', value: 'H3C', icon: H3CVendorIcon },
  { label: 'Huawei', value: 'Huawei', icon: HuaweiVendorIcon },
  { label: 'Cisco', value: 'Cisco', icon: CiscoVendorIcon }
]

const deviceFormRules = {
  deviceName: [{ required: true, message: '请输入设备名称', trigger: 'blur' }],
  ipAddress: [
    { required: true, message: '请输入IP地址', trigger: 'blur' },
    {
      pattern: /^(\d{1,3}\.){3}\d{1,3}$/,
      message: 'IP地址格式不正确',
      trigger: 'blur'
    }
  ],
  snmpUrl: [{ required: true, message: '请输入SNMP URL', trigger: 'blur' }],
  deviceVendor: [{ required: true, message: '请选择厂商', trigger: 'change' }],
  deviceModel: [{ required: true, message: '请输入型号', trigger: 'blur' }]
}

function mapDevice(device = {}) {
  return {
    id: device.id,
    deviceName: device.deviceName || '',
    ipAddress: device.ipAddress || '',
    snmpUrl: device.snmpUrl || '',
    deviceVendor: device.deviceVendor || '',
    deviceModel: device.deviceModel || '',
    location: device.location || '',
    owner: device.owner || '',
    remark: device.remark || '',
    sshName: device.sshName || '',
    sshPasswd: device.sshPasswd || '',
    sshPort: device.sshPort ?? null,
    monitorTemplate: device.monitorTemplate || null,
    status: device.status || '',
    metricsAvailable: device.metricsAvailable,
    updateTime: device.updateTime || ''
  }
}

function buildCreatePayload(form) {
  const payload = {
    deviceName: form.deviceName.trim(),
    deviceType: 'switch',
    ipAddress: form.ipAddress.trim(),
    snmpUrl: form.snmpUrl.trim(),
    deviceVendor: form.deviceVendor.trim(),
    deviceModel: form.deviceModel.trim()
  }

  if (form.location) payload.location = form.location
  if (form.owner) payload.owner = form.owner
  if (form.remark) payload.remark = form.remark
  if (form.sshName) payload.sshName = form.sshName.trim()
  if (form.sshPasswd) payload.sshPasswd = form.sshPasswd
  if (form.sshPort) payload.sshPort = form.sshPort

  return payload
}

function buildUpdatePayload(form) {
  const payload = {
    deviceName: form.deviceName.trim(),
    deviceType: 'switch',
    ipAddress: form.ipAddress.trim(),
    snmpUrl: form.snmpUrl.trim(),
    deviceVendor: form.deviceVendor.trim(),
    deviceModel: form.deviceModel.trim(),
    location: form.location,
    owner: form.owner,
    remark: form.remark,
    sshName: form.sshName ? form.sshName.trim() : ''
  }

  if (form.sshPort) payload.sshPort = form.sshPort
  if (form.sshPasswd) payload.sshPasswd = form.sshPasswd

  return payload
}

export default {
  components: {
    Search,
    Refresh,
    Plus
  },
  data() {
    return {
      queryParams: createQueryParams(),
      loading: true,
      deviceList: [],
      total: 0,
      switchDeviceIcon: SwitchDeviceIcon,
      deviceVendorOptions,
      addDeviceDialogVisible: false,
      addSubmitting: false,
      addDeviceForm: createDeviceForm(),
      editDeviceDialogVisible: false,
      editSubmitting: false,
      deviceInfo: createDeviceForm(),
      deviceFormRules
    }
  },
  computed: {
    vendorCount() {
      return new Set(this.deviceList.map(device => device.deviceVendor).filter(Boolean)).size
    }
  },
  methods: {
    getVendorIcon(vendor) {
      return this.deviceVendorOptions.find(option => option.value === vendor)?.icon || ''
    },

    buildQueryParams() {
      const params = {
        page: this.queryParams.page,
        pageSize: this.queryParams.pageSize,
        deviceType: 'switch'
      }

      if (this.queryParams.keyword.trim()) {
        params.keyword = this.queryParams.keyword.trim()
      }

      if (this.queryParams.status) {
        params.status = this.queryParams.status
      }

      return params
    },

    async getList() {
      this.loading = true
      try {
        const { data: res } = await API.getSnmpDevices({
          params: this.buildQueryParams()
        })

        if (res.code !== 200) {
          throw new Error(res.message || '获取交换机列表失败')
        }

        this.deviceList = (res.data.items || []).map(mapDevice)
        this.total = res.data.total || 0

        if (this.deviceList.length > 0) {
          await this.loadBatchResourceUsage()
        }
      } catch (error) {
        console.error('获取交换机列表失败:', error)
        this.deviceList = []
        this.total = 0
        this.$message.error(error?.response?.data?.message || error?.message || '获取交换机列表失败')
      } finally {
        this.loading = false
      }
    },

    async loadBatchResourceUsage() {
      try {
        const deviceIds = this.deviceList.map(device => device.id).join(',')
        const { data: res } = await API.getSnmpDevicesBatchResource(deviceIds)

        if (res.code !== 200 || !Array.isArray(res.data?.items)) {
          return
        }

        const resourceMap = {}
        res.data.items.forEach(item => {
          resourceMap[item.deviceId] = {
            status: item.status || '',
            deviceVendor: item.deviceVendor || '',
            deviceModel: item.deviceModel || '',
            monitorTemplate: item.monitorTemplate || null,
            metricsAvailable: item.metricsAvailable,
            updateTime: item.updateTime || ''
          }
        })

        this.deviceList = this.deviceList.map(device => {
          const resourceItem = resourceMap[device.id]
          if (!resourceItem) return device

          return {
            ...device,
            ...resourceItem,
            deviceVendor: resourceItem.deviceVendor || device.deviceVendor,
            deviceModel: resourceItem.deviceModel || device.deviceModel,
            monitorTemplate: resourceItem.monitorTemplate || device.monitorTemplate
          }
        })
      } catch (error) {
        console.error('获取交换机资源数据失败:', error)
      }
    },

    handleQuery() {
      this.queryParams.page = 1
      this.getList()
    },

    resetQuery() {
      this.queryParams = createQueryParams()
      this.getList()
      this.$message.success('重置成功')
    },

    handleRefresh() {
      this.getList()
      this.$message.success('刷新成功')
    },

    handleSizeChange(pageSize) {
      this.queryParams.pageSize = pageSize
      this.queryParams.page = 1
      this.getList()
    },

    handleCurrentChange(page) {
      this.queryParams.page = page
      this.getList()
    },

    addDeviceDialogClosed() {
      this.$refs.addDeviceFormRef?.resetFields()
      this.addDeviceForm = createDeviceForm()
    },

    async addDevice() {
      try {
        await this.$refs.addDeviceFormRef.validate()
        this.addSubmitting = true

        const { data: res } = await API.createSnmpDevice(buildCreatePayload(this.addDeviceForm))

        if (res.code !== 200) {
          throw new Error(res.message || '新增交换机失败')
        }

        this.$message.success('新增交换机成功')
        this.addDeviceDialogVisible = false
        await this.getList()
      } catch (error) {
        console.error('新增交换机失败:', error)
        if (error !== false && typeof error === 'object') {
          this.$message.error(error?.response?.data?.message || error?.message || '新增交换机失败')
        }
      } finally {
        this.addSubmitting = false
      }
    },

    async showEditDeviceDialog(id) {
      try {
        const { data: res } = await API.getSnmpDeviceById(id)

        if (res.code !== 200) {
          throw new Error(res.message || '获取交换机信息失败')
        }

        this.deviceInfo = {
          ...createDeviceForm(),
          ...mapDevice(res.data),
          sshPasswd: ''
        }

        this.editDeviceDialogVisible = true
      } catch (error) {
        console.error('获取交换机信息失败:', error)
        this.$message.error(error?.response?.data?.message || error?.message || '获取交换机信息失败')
      }
    },

    async editDevice() {
      try {
        await this.$refs.editDeviceFormRef.validate()
        this.editSubmitting = true

        const { data: res } = await API.updateSnmpDevice(this.deviceInfo.id, buildUpdatePayload(this.deviceInfo))

        if (res.code !== 200) {
          throw new Error(res.message || '更新交换机失败')
        }

        this.$message.success('更新交换机成功')
        this.editDeviceDialogVisible = false
        await this.getList()
      } catch (error) {
        console.error('更新交换机失败:', error)
        if (error !== false && typeof error === 'object') {
          this.$message.error(error?.response?.data?.message || error?.message || '更新交换机失败')
        }
      } finally {
        this.editSubmitting = false
      }
    },

    async handleDeviceDelete(row) {
      try {
        const confirmResult = await this.$confirm(
          `是否确认删除交换机 "${row.deviceName}"？删除后将移除该设备的SNMP指标配置。`,
          '提示',
          {
            confirmButtonText: '确定',
            cancelButtonText: '取消',
            type: 'warning'
          }
        ).catch(err => err)

        if (confirmResult !== 'confirm') {
          return this.$message.info('已取消删除')
        }

        const { data: res } = await API.deleteSnmpDevice(row.id)

        if (res.code !== 200) {
          throw new Error(res.message || '删除交换机失败')
        }

        this.$message.success('删除交换机成功')
        await this.getList()
      } catch (error) {
        console.error('删除交换机失败:', error)
        this.$message.error(error?.response?.data?.message || error?.message || '删除交换机失败')
      }
    },

    handleOpenMonitor(row) {
      this.$router.push({
        path: `/cmdb/switch/${row.id}`,
        query: {
          ip: row.ipAddress || '',
          name: row.deviceName || ''
        }
      })
    }
  },
  created() {
    this.getList()
  }
}
</script>

<style scoped>
.cmdb-snmp-management {
  --snmp-page-height-offset: 124px;
  --snmp-bg-page: #f4f7fb;
  --snmp-surface: rgba(247, 250, 255, 0.84);
  --snmp-surface-strong: rgba(255, 255, 255, 0.94);
  --snmp-surface-warm: rgba(255, 253, 248, 0.94);
  --snmp-surface-warm-strong: rgba(255, 248, 239, 0.96);
  --snmp-text-primary: #172133;
  --snmp-text-secondary: #5f6b7c;
  --snmp-text-muted: #8d97a8;
  --snmp-border-soft: rgba(15, 23, 42, 0.06);
  --snmp-border-cool: #dfe7f2;
  --snmp-border-mid: rgba(207, 217, 230, 0.92);
  --snmp-brand: #4c8bf5;
  --snmp-brand-active: #4d79d8;
  --snmp-brand-soft: rgba(76, 139, 245, 0.12);
  --snmp-brand-fade: rgba(76, 139, 245, 0.06);
  --snmp-success: #3fb97f;
  --snmp-warning: #f3b24b;
  --snmp-danger: #ef6b73;
  --snmp-shadow-soft: 0 20px 60px rgba(17, 24, 39, 0.07);
  box-sizing: border-box;
  height: calc(100vh - var(--snmp-page-height-offset));
  min-height: 0;
  overflow: hidden;
  padding: 6px;
  background: transparent;
  color: var(--snmp-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.snmp-card {
  position: relative;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: linear-gradient(180deg, var(--snmp-surface-warm), rgba(247, 250, 255, 0.9));
  backdrop-filter: none;
  border-radius: 24px;
  box-shadow: var(--snmp-shadow-soft);
  border: 1px solid rgba(223, 231, 242, 0.96);
}

.snmp-card::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 92px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  pointer-events: none;
  z-index: 0;
}

.snmp-card :deep(.el-card__header) {
  position: relative;
  z-index: 1;
  padding: 16px 20px 14px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.86);
  background: transparent;
}

.snmp-card :deep(.el-card__body) {
  position: relative;
  z-index: 1;
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 14px 16px 16px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 18px;
}

.title-block {
  min-width: 0;
}

.title-kicker {
  display: block;
  margin-bottom: 5px;
  color: var(--snmp-brand);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.title {
  display: block;
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  color: var(--snmp-text-primary);
  background: none;
  -webkit-text-fill-color: currentColor;
}

.title-block p {
  margin: 6px 0 0;
  color: var(--snmp-text-secondary);
  font-size: 13px;
  line-height: 1.5;
}

.header-status {
  flex: 0 0 auto;
  margin-top: 2px;
  border-color: rgba(223, 231, 242, 0.92);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.72);
  color: var(--snmp-text-muted);
}

.search-section {
  flex: 0 0 auto;
  margin-bottom: 12px;
  padding: 14px 16px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.82), rgba(247, 250, 255, 0.78));
  border-radius: 20px;
  border: 1px solid rgba(223, 231, 242, 0.92);
}

.search-form {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px 14px;
}

.search-form .el-form-item {
  margin-bottom: 0;
  margin-right: 0;
}

.search-form .el-form-item__label {
  color: var(--snmp-text-secondary);
  font-weight: 600;
}

.action-section {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
  padding-left: 0;
}

.mode-alert {
  margin-bottom: 20px;
}

.table-section {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.table-shell {
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.72);
}

.device-table {
  flex: 1 1 auto;
  min-height: 0;
  height: 100%;
  border-radius: 18px;
  overflow: hidden;
  box-shadow: none;
  background: transparent;
}

.device-table :deep(.el-table__body-wrapper),
.device-table :deep(.el-scrollbar__wrap) {
  overflow-x: hidden !important;
}

.device-table :deep(.el-table__header) {
  background: transparent;
}

.device-table :deep(.el-table__header th) {
  background: rgba(248, 250, 255, 0.9) !important;
  color: var(--snmp-text-secondary) !important;
  font-weight: 600 !important;
  border-bottom: 1px solid rgba(223, 231, 242, 0.86);
  text-shadow: none;
}

.device-table :deep(.el-table__header th .cell) {
  color: var(--snmp-text-secondary) !important;
  font-weight: 600 !important;
  text-shadow: none;
}

.device-table :deep(.el-table__body),
.device-table :deep(.el-table__body tr),
.device-table :deep(.el-table__body td) {
  background: transparent;
}

.device-table :deep(.el-table__cell) {
  border-bottom-color: rgba(223, 231, 242, 0.66);
}

.device-table :deep(.el-table__row) {
  transition: background-color 0.2s ease;
}

.device-table :deep(.el-table__row:hover) {
  background-color: var(--snmp-brand-fade) !important;
  transform: none;
  box-shadow: none;
}

.device-name-cell {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.device-name-icon {
  width: 18px;
  height: 18px;
  object-fit: contain;
  flex-shrink: 0;
}

.device-name {
  font-weight: 600;
  color: var(--snmp-text-primary);
  font-size: 14px;
}

.vendor-cell {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.vendor-icon,
.vendor-option-icon,
.vendor-select-icon {
  width: 18px;
  height: 18px;
  object-fit: contain;
  flex-shrink: 0;
}

.vendor-option {
  display: flex;
  align-items: center;
  gap: 8px;
}

.empty-text {
  color: var(--snmp-text-muted);
}

.operation-links {
  display: flex;
  align-items: center;
  gap: 14px;
  justify-content: center;
}

.operation-links :deep(.el-button) {
  height: auto;
  padding: 0;
  border-radius: 0;
  font-weight: 600;
  background: transparent !important;
  box-shadow: none !important;
  transform: none !important;
}

.operation-link-warning {
  color: #c37a20;
}

.operation-link-warning:hover,
.operation-link-warning:focus {
  color: var(--snmp-warning);
}

.pagination-section {
  flex: 0 0 auto;
  margin-top: auto;
  padding-top: 12px;
  display: flex;
  justify-content: flex-end;
}

.el-tag {
  font-weight: 500;
  border-radius: 999px;
  border: none;
}

.el-button {
  height: 32px;
  border-radius: 14px;
  font-weight: 600;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, color 0.2s ease, background-color 0.2s ease;
}

.el-button:hover {
  transform: none;
  box-shadow: none;
}

.el-button--primary {
  border-color: transparent;
  background: #4c8bf5;
}

.el-button--success {
  border-color: transparent;
  background: #3fb97f;
}

.el-button--warning {
  border-color: transparent;
  background: #f3b24b;
}

.el-input :deep(.el-input__wrapper),
.el-select :deep(.el-input__wrapper) {
  background: rgba(255, 255, 255, 0.88);
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 14px;
  box-shadow: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.el-input :deep(.el-input__wrapper):hover,
.el-select :deep(.el-input__wrapper):hover {
  border-color: rgba(76, 139, 245, 0.35);
}

.el-input :deep(.el-input__wrapper.is-focus),
.el-select :deep(.el-input__wrapper.is-focus) {
  border-color: rgba(76, 139, 245, 0.72);
  box-shadow: 0 0 0 3px rgba(76, 139, 245, 0.12);
  background: #fff;
}

.el-input :deep(.el-input__inner),
.el-select :deep(.el-input__inner) {
  background: transparent;
  border: none;
  color: var(--snmp-text-secondary);
}

:deep(.el-loading-mask) {
  background-color: rgba(247, 250, 255, 0.72);
  backdrop-filter: blur(4px);
}

.el-dialog {
  border-radius: 12px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

:deep(.el-switch.is-checked .el-switch__core) {
  background-color: #67c23a;
}

@media (max-width: 768px) {
  .cmdb-snmp-management {
    padding: 15px;
  }

  .title {
    font-size: 18px;
  }

  .search-section {
    padding: 16px;
  }

  .search-form .el-form-item {
    margin-bottom: 8px;
  }
}

/* DESIGN.md: primitive -> semantic -> component tokens */
.cmdb-snmp-management {
  --switch-blue-500: #409eff;
  --switch-blue-600: #2563eb;
  --switch-green-500: #67c23a;
  --switch-yellow-400: #f3c66b;
  --switch-yellow-500: #e6a23c;
  --switch-red-500: #ef4444;
  --switch-slate-950: #0f172a;
  --switch-slate-800: #172033;
  --switch-slate-600: #53657d;
  --switch-slate-500: #64748b;
  --switch-slate-400: #7b8aa0;

  --switch-color-primary: var(--switch-blue-500);
  --switch-color-success: var(--switch-green-500);
  --switch-color-warning: var(--switch-yellow-500);
  --switch-color-danger: var(--switch-red-500);
  --switch-text-heading: var(--switch-slate-950);
  --switch-text-primary: var(--switch-slate-800);
  --switch-text-secondary: var(--switch-slate-600);
  --switch-text-muted: var(--switch-slate-500);
  --switch-bg-surface: #fff;
  --switch-bg-surface-soft: #fbfdff;
  --switch-bg-subtle: #f3f7fc;
  --switch-border: #dbe7f4;
  --switch-border-soft: #e7eef7;
  --switch-border-hover: #b9d4ff;

  --switch-card-radius: 12px;
  --switch-control-radius: 8px;
  --switch-dialog-radius: 15px;
  --switch-control-height: 30px;
  --switch-card-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
  --switch-hero-shadow: 0 10px 28px rgba(80, 124, 180, 0.1);

  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  align-content: start;
  gap: 8px;
  height: calc(100vh - 124px);
  min-height: 0;
  padding: 7px;
  overflow: hidden;
  color: var(--switch-text-primary);
  background: transparent;
}

:global(.home-container .el-main:has(.cmdb-snmp-management)) {
  padding: 10px;
}

.switch-hero-panel {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
  padding: 8px 12px;
  border: 1px solid var(--switch-border);
  border-radius: var(--switch-card-radius);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 251, 255, 0.95));
  box-shadow: var(--switch-hero-shadow);
}

.switch-hero-copy {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  gap: 8px;
  min-width: 0;
}

.switch-hero-icon {
  display: grid;
  flex: 0 0 42px;
  width: 42px;
  height: 42px;
  place-items: center;
  border: 0;
  border-radius: 0;
  background: transparent;
}

.switch-hero-icon img {
  width: 30px;
  height: 30px;
  object-fit: contain;
}

.switch-hero-content {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  gap: 10px;
  min-width: 0;
}

.switch-hero-copy h1 {
  flex: 0 0 auto;
  margin: 0;
  color: var(--switch-text-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: 0;
  white-space: nowrap;
}

.switch-hero-divider {
  flex: 0 0 1px;
  width: 1px;
  height: 22px;
  background: var(--switch-border);
}

.switch-hero-content p {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  color: var(--switch-text-muted);
  font-size: 12px;
  font-weight: 400;
  line-height: 1.5;
  letter-spacing: 0;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.switch-hero-metrics {
  display: flex;
  align-items: stretch;
  flex: 0 0 auto;
}

.switch-hero-metrics > div {
  display: grid;
  grid-template-columns: auto auto;
  align-items: baseline;
  gap: 6px;
  min-width: 84px;
  padding: 3px 12px;
  border-left: 1px solid var(--switch-border-soft);
}

.switch-hero-metrics span {
  color: var(--switch-slate-400);
  font-size: 11px;
  font-weight: 600;
}

.switch-hero-metrics strong {
  color: var(--switch-text-heading);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 18px;
  line-height: 1;
}

.snmp-card,
:deep(.snmp-card) {
  min-width: 0;
  min-height: 0;
  height: 100%;
  border: 1px solid var(--switch-border) !important;
  border-radius: var(--switch-card-radius) !important;
  background: rgba(255, 255, 255, 0.97) !important;
  box-shadow: var(--switch-card-shadow) !important;
}

.snmp-card::before {
  display: none;
}

.snmp-card :deep(.el-card__body) {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  padding: 0;
}

.switch-toolbar {
  flex: 0 0 auto;
  padding: 8px 10px;
  border-bottom: 1px solid var(--switch-border-soft);
  background: var(--switch-bg-surface);
}

.switch-toolbar-main {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.search-form {
  display: flex;
  align-items: center;
  flex: 0 0 auto;
  flex-wrap: nowrap;
  gap: 8px;
}

.keyword-input {
  width: 280px;
}

.search-form .el-form-item,
.query-actions {
  margin: 0;
}

.search-form :deep(.el-form-item__label) {
  height: var(--switch-control-height);
  padding-right: 7px;
  color: var(--switch-text-secondary);
  font-size: 12px;
  font-weight: 700;
  line-height: var(--switch-control-height);
}

.query-actions :deep(.el-form-item__content),
.toolbar-actions {
  display: flex;
  align-items: center;
  flex-wrap: nowrap;
  gap: 6px;
}

.toolbar-context {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  gap: 6px;
  min-width: 0;
  color: var(--switch-text-muted);
  font-size: 11px;
  white-space: nowrap;
}

.toolbar-actions {
  flex: 0 0 auto;
}

.table-section {
  flex: 1 1 auto;
  min-height: 0;
  margin: 8px 10px 0;
  border: 1px solid #e3ebf5;
  border-radius: var(--switch-control-radius);
  background: var(--switch-bg-surface);
}

.device-table {
  width: 100%;
  min-height: 0;
  height: 100%;
  border-radius: var(--switch-control-radius);
  background: var(--switch-bg-surface);
}

.cmdb-snmp-management :deep(.device-table.el-table) {
  --el-table-border-color: #e3ebf5;
  --el-table-header-bg-color: var(--switch-bg-subtle);
  --el-table-tr-bg-color: #fff;
  --el-table-row-hover-bg-color: #f8fafc;
  --el-table-current-row-bg-color: #eef6ff;
  --el-table-bg-color: #fff;
  --el-table-text-color: var(--switch-text-secondary);
  --el-table-header-text-color: #475569;
  overflow: hidden;
  font-size: 12px;
}

.cmdb-snmp-management :deep(.device-table .el-table__inner-wrapper::before) {
  display: none;
}

.cmdb-snmp-management :deep(.device-table th.el-table__cell) {
  height: 34px;
  padding: 5px 0;
  color: #475569;
  background: var(--switch-bg-subtle) !important;
  font-size: 12px;
  font-weight: 700 !important;
}

.cmdb-snmp-management :deep(.device-table td.el-table__cell) {
  padding: 5px 0;
  font-size: 12px;
}

.cmdb-snmp-management :deep(.device-table td.el-table__cell),
.cmdb-snmp-management :deep(.device-table th.el-table__cell.is-leaf) {
  border-bottom-color: #eef3f9;
}

.cmdb-snmp-management :deep(.device-table .el-table__row),
.cmdb-snmp-management :deep(.device-table .el-table__row:hover) {
  box-shadow: none;
  transform: none;
}

.cmdb-snmp-management :deep(.device-table .el-table__row:hover > td.el-table__cell),
.cmdb-snmp-management :deep(.device-table .el-table__fixed-right .el-table__row:hover > .el-table__cell) {
  background: #f8fafc !important;
}

.device-name-cell,
.vendor-cell {
  gap: 6px;
}

.device-name-icon,
.vendor-icon,
.vendor-option-icon,
.vendor-select-icon {
  width: 16px;
  height: 16px;
}

.device-name {
  overflow: hidden;
  color: var(--switch-text-primary);
  font-size: 12px;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.operation-links {
  gap: 12px;
}

.operation-links :deep(.el-button) {
  min-height: 24px;
  font-size: 12px;
}

.pagination-section {
  display: flex;
  flex: 0 0 auto;
  justify-content: flex-end;
  margin: 0;
  padding: 8px 10px 10px;
}

.cmdb-snmp-management :deep(.pagination-section .btn-prev),
.cmdb-snmp-management :deep(.pagination-section .btn-next),
.cmdb-snmp-management :deep(.pagination-section .el-pager li) {
  border-radius: 7px;
  color: var(--switch-text-secondary);
  background: #f8fafc;
}

.cmdb-snmp-management :deep(.pagination-section .el-pager li.is-active) {
  color: #fff;
  background: var(--switch-blue-600);
}

.cmdb-snmp-management :deep(.toolbar-context .el-tag) {
  height: 22px;
  border-color: #cfe0ff;
  border-radius: 999px;
  color: var(--switch-blue-600);
  background: #eef6ff;
  font-size: 11px;
}

.cmdb-snmp-management :deep(.el-input__wrapper),
.cmdb-snmp-management :deep(.el-select__wrapper),
.cmdb-snmp-management :deep(.el-textarea__inner),
.cmdb-snmp-management :deep(.el-input-number) {
  border-radius: var(--switch-control-radius);
  background: var(--switch-bg-surface-soft);
  box-shadow: 0 0 0 1px #d9e5f2 inset !important;
  transition: background-color 160ms ease, box-shadow 160ms ease;
}

.cmdb-snmp-management :deep(.el-input__wrapper:hover),
.cmdb-snmp-management :deep(.el-select__wrapper:hover),
.cmdb-snmp-management :deep(.el-textarea__inner:hover) {
  box-shadow: 0 0 0 1px var(--switch-border-hover) inset !important;
}

.cmdb-snmp-management :deep(.el-input__wrapper.is-focus),
.cmdb-snmp-management :deep(.el-select__wrapper.is-focused),
.cmdb-snmp-management :deep(.el-textarea__inner:focus) {
  box-shadow: 0 0 0 1px var(--switch-color-primary) inset !important;
}

.keyword-input :deep(.el-input__wrapper) {
  min-height: var(--switch-control-height);
}

.cmdb-snmp-management :deep(.el-button) {
  border-radius: var(--switch-control-radius);
  font-weight: 600;
  transition: color 160ms ease, background-color 160ms ease, border-color 160ms ease, box-shadow 160ms ease;
}

.cmdb-snmp-management :deep(.switch-toolbar .el-button) {
  height: var(--switch-control-height);
  min-height: var(--switch-control-height);
  padding: 5px 10px;
  font-size: 12px;
}

.cmdb-snmp-management :deep(.el-button:not(.is-link):not(.is-text):hover) {
  box-shadow: 0 5px 12px rgba(64, 158, 255, 0.12);
  transform: none;
}

.cmdb-snmp-management :deep(.el-button--primary:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: var(--switch-color-primary);
  color: #fff;
  background: var(--switch-color-primary);
}

.cmdb-snmp-management :deep(.el-button--success:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: var(--switch-color-success);
  color: #fff;
  background: var(--switch-color-success);
}

.cmdb-snmp-management :deep(.el-button--warning:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: var(--switch-color-warning);
  color: #fff;
  background: var(--switch-color-warning);
}

.cmdb-snmp-management :deep(.el-button.switch-refresh-button) {
  border-color: var(--switch-yellow-400);
  color: #fff;
  background: var(--switch-yellow-400);
}

.cmdb-snmp-management :deep(.el-button.switch-refresh-button:hover),
.cmdb-snmp-management :deep(.el-button.switch-refresh-button:focus-visible) {
  border-color: #dcae50;
  background: #dcae50;
}

.cmdb-snmp-management :deep(.el-button--primary:not(.is-link) .el-icon),
.cmdb-snmp-management :deep(.el-button--success:not(.is-link) .el-icon),
.cmdb-snmp-management :deep(.el-button--warning:not(.is-link) .el-icon) {
  color: #fff;
}

.switch-device-form :deep(.el-form-item) {
  margin-bottom: 14px;
}

.switch-device-form :deep(.el-form-item__label) {
  height: auto;
  margin-bottom: 6px;
  padding: 0;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.4;
}

.switch-device-form :deep(.el-input__wrapper),
.switch-device-form :deep(.el-select__wrapper),
.switch-device-form :deep(.el-input-number) {
  min-height: 34px;
  border-radius: var(--switch-control-radius);
  background: var(--switch-bg-surface-soft);
  box-shadow: 0 0 0 1px #d9e5f2 inset !important;
  transition: background-color 160ms ease, box-shadow 160ms ease;
}

.switch-device-form :deep(.el-textarea__inner) {
  min-height: 72px !important;
  border-radius: var(--switch-control-radius);
  background: var(--switch-bg-surface-soft);
  box-shadow: 0 0 0 1px #d9e5f2 inset !important;
  font-size: 12px;
  line-height: 1.6;
}

.switch-device-form :deep(.el-input__wrapper:hover),
.switch-device-form :deep(.el-select__wrapper:hover),
.switch-device-form :deep(.el-textarea__inner:hover) {
  box-shadow: 0 0 0 1px var(--switch-border-hover) inset !important;
}

.switch-device-form :deep(.el-input__wrapper.is-focus),
.switch-device-form :deep(.el-select__wrapper.is-focused),
.switch-device-form :deep(.el-textarea__inner:focus) {
  box-shadow: 0 0 0 1px var(--switch-color-primary) inset !important;
}

.switch-device-form :deep(.el-divider) {
  margin: 10px 0 16px;
  border-top-color: var(--switch-border-soft);
}

.switch-device-form :deep(.el-divider__text) {
  padding: 0 10px;
  color: var(--switch-text-secondary);
  background: var(--switch-bg-surface);
  font-size: 12px;
  font-weight: 700;
}

:global(.switch-form-dialog-overlay) {
  --switch-text-heading: #0f172a;
  --switch-text-secondary: #53657d;
  --switch-bg-surface: #fff;
  --switch-bg-surface-soft: #fbfdff;
  --switch-border-soft: #e7eef7;
  --switch-border-hover: #b9d4ff;
  --switch-color-primary: #409eff;
  --switch-control-radius: 8px;
}

:global(.switch-form-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  width: min(760px, calc(100vw - 24px)) !important;
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.switch-form-dialog .el-dialog__header) {
  position: relative;
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: linear-gradient(90deg, #fff6e7 0%, #f7fbff 36%, #ecf7ff 100%);
}

:global(.switch-form-dialog .el-dialog__header::before) {
  content: '';
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 3px;
  background: linear-gradient(90deg, #f5a524 0%, #62a8ff 48%, #55d6be 100%);
}

:global(.switch-form-dialog .el-dialog__title) {
  color: var(--switch-text-heading);
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.switch-form-dialog .el-dialog__headerbtn) {
  top: 10px;
  right: 12px;
}

:global(.switch-form-dialog .el-dialog__body) {
  max-height: calc(100vh - 190px);
  padding: 16px 18px 4px;
  overflow-y: auto;
}

:global(.switch-form-dialog .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #fff;
}

:global(.switch-form-dialog .dialog-footer .el-button) {
  min-width: 78px;
  min-height: 32px;
  border-radius: 8px;
  font-weight: 600;
}

:global(.switch-form-dialog .dialog-footer .el-button--primary) {
  border-color: #409eff;
  color: #fff;
  background: linear-gradient(135deg, #5aa8ff, #2563eb);
}

@media (max-width: 1100px) {
  .switch-toolbar-main {
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .toolbar-context {
    order: 3;
    flex-basis: 100%;
  }
}

@media (max-width: 768px) {
  .cmdb-snmp-management {
    grid-template-rows: auto auto;
    height: auto;
    padding: 6px;
    overflow: visible;
  }

  .switch-hero-panel {
    align-items: stretch;
    flex-direction: column;
  }

  .switch-hero-content {
    align-items: flex-start;
    flex-direction: column;
    gap: 3px;
  }

  .switch-hero-divider {
    display: none;
  }

  .switch-hero-content p {
    width: 100%;
    white-space: normal;
  }

  .switch-hero-metrics {
    border-top: 1px solid var(--switch-border-soft);
  }

  .switch-hero-metrics > div {
    flex: 1;
    justify-content: center;
    min-width: 0;
    padding: 7px 4px 2px;
    border-left: 0;
  }

  .search-form,
  .toolbar-actions,
  .keyword-input {
    width: 100%;
  }

  .search-form {
    align-items: stretch;
    flex-wrap: wrap;
  }

  .search-form .el-form-item:first-child {
    width: 100%;
  }

  .search-form .el-form-item:first-child :deep(.el-form-item__content) {
    flex: 1;
  }

  .toolbar-actions {
    justify-content: flex-start;
  }

  .table-section {
    min-height: 460px;
  }

  .switch-device-form :deep(.el-col) {
    max-width: 100%;
    flex: 0 0 100%;
  }

  :global(.switch-form-dialog .el-dialog__body) {
    padding-right: 14px;
    padding-left: 14px;
  }
}
</style>
