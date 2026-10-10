<template>
  <div class="accountauth-management">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/账号.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>通用账号认证管理</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>统一维护数据库、中间件与观测平台账号认证信息。</p>
      </div>
    </header>

    <el-card shadow="never" class="accountauth-card">
      <!-- 搜索表单 -->
      <div class="search-section">
        <el-form :model="queryParams" :inline="true" class="search-form" ref="queryForm">
          <el-form-item label="账号别名" prop="alias">
            <el-input
              v-model="queryParams.alias"
              placeholder="请输入账号别名"
              clearable
              @keyup.enter="handleQuery"
            />
          </el-form-item>
          <el-form-item label="账号类型" prop="type" class="account-type-filter">
            <el-select v-model="queryParams.type" placeholder="请选择账号类型" clearable>
              <el-option
                v-for="item in accountTypeOptions"
                :key="item.value"
                :label="item.label"
                :value="item.value"
              />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" icon="Search" class="search-button" @click="handleQuery">查询</el-button>
            <el-button type="warning" icon="Refresh" class="reset-button" @click="resetQuery">重置</el-button>
            <el-button
              type="success"
              v-authority="['config:common:add']"
              icon="Plus"
              class="add-button"
              @click="showAddDialog"
            >
              新增账号
            </el-button>
            <el-button type="warning" icon="Upload" class="import-button" @click="showImportDialog">
              批量导入
            </el-button>
          </el-form-item>
        </el-form>
      </div>

      <!-- 列表区域 -->
      <div class="table-section">
        <div class="content-header">
          <h2>账号列表</h2>
          <span class="resource-count">共 {{ total }} 个账号</span>
        </div>
        <el-table stripe v-loading="loading" :data="accountList" class="accountauth-table">
      <el-table-column label="账号别名" prop="alias">
        <template v-slot="scope">
          <div class="cell-main">
            <img src="@/assets/image/账号.svg" class="cell-icon"/>
            <span>{{ scope.row.alias }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="账号地址" prop="host"  width="320" >
        <template v-slot="scope">
          <div class="cell-main">
            <img src="@/assets/image/url.svg" class="cell-icon"/>
            <span class="mono-text">{{ scope.row.host }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="用户名" prop="name">
        <template #default>
          <div class="cell-main">
            <img src="@/assets/image/ren.svg" class="cell-icon"/>
            <span class="masked-text">* * * * * *</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="账号类型" width="120">
        <template v-slot="scope">
          <el-tag :type="getAccountTypeTag(scope.row.type)">
            {{ getAccountTypeLabel(scope.row.type) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="创建时间" prop="createdAt" />
      <el-table-column label="备注" prop="remark" />
      <el-table-column label="操作" class-name="small-padding fixed-width" width="260" align="right" header-align="right">
        <template v-slot="scope">
          <div class="operation-buttons">
            <button type="button" v-authority="['config:common:edit']" class="operation-link operation-primary" @click="showEditDialog(scope.row)">修改</button>
            <button type="button" v-authority="['config:common:decrypt']" class="operation-link operation-warning" @click="handleDecrypt(scope.row)">解密</button>
            <button type="button" v-authority="['config:common:delete']" class="operation-link operation-danger" @click="handleDelete(scope.row)">删除</button>
          </div>
        </template>
      </el-table-column>
        </el-table>
      </div>

      <!-- 分页区域 -->
      <div class="pagination-section">
        <el-pagination
          @size-change="handleSizeChange"
          @current-change="handleCurrentChange"
          :current-page="queryParams.pageNum"
          :page-sizes="[10, 20, 50, 100]"
          :page-size="queryParams.pageSize"
          layout="total, sizes, prev, pager, next, jumper"
          :total="total"
        />
      </div>
    
    <!--新增/编辑对话框-->
    <el-dialog
      :title="dialogTitle"
      v-model="dialogVisible"
      width="640px"
      :modal="false"
      class="accountauth-edit-dialog"
      modal-class="accountauth-dialog-overlay"
    >
      <el-form :model="formData" :rules="formRules" ref="formRef" label-width="100px" class="account-form">
        <el-form-item label="账号别名" prop="alias">
          <el-input v-model="formData.alias" placeholder="请输入账号别名"></el-input>
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="账号地址" prop="host">
              <el-input v-model="formData.host" placeholder="192.168.1.1:3306 (无需协议)"></el-input>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="账号类型" prop="type">
              <el-select v-model="formData.type" placeholder="请选择账号类型">
                <el-option
                  v-for="item in accountTypeOptions"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value"
                />
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="用户名" prop="name">
              <el-input v-model="formData.name" placeholder="请输入用户名"></el-input>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="密码" prop="password">
              <el-input v-model="formData.password" show-password placeholder="请输入密码"></el-input>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="备注" prop="remark">
          <el-input v-model="formData.remark" type="textarea" :rows="3" placeholder="请输入备注信息"></el-input>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitForm">
            {{ formData.id ? '保存修改' : '创建账号' }}
          </el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog
      v-model="importDialogVisible"
      title="批量导入账号认证"
      width="680px"
      :modal="false"
      class="accountauth-import-dialog"
      modal-class="accountauth-dialog-overlay"
      @closed="resetImportDialog"
    >
      <div class="import-panel">
        <el-alert
          title="导入列顺序固定为：账号别名、账号地址、用户名、密码。账号类型不填写时按通用类型导入。"
          type="info"
          show-icon
          :closable="false"
        />

        <el-radio-group v-model="importMode" class="import-mode-group">
          <el-radio-button label="file">上传 Excel</el-radio-button>
          <el-radio-button label="text">粘贴文本</el-radio-button>
        </el-radio-group>

        <div v-if="importMode === 'file'" class="import-mode-panel">
          <el-upload
            ref="importUploadRef"
            drag
            :auto-upload="false"
            :limit="1"
            accept=".xlsx"
            :file-list="importFileList"
            :on-change="handleImportFileChange"
            :on-remove="handleImportFileRemove"
            :on-exceed="handleImportFileExceed"
          >
            <div class="upload-drag-content">
              <p>拖拽文件到此处，或点击上传</p>
              <span>仅支持 xlsx 文件，字段名 file</span>
            </div>
          </el-upload>
        </div>

        <div v-else class="import-mode-panel">
          <el-input
            v-model="importText"
            type="textarea"
            :rows="8"
            placeholder="账号别名 账号地址 用户名 密码&#10;demo 192.168.1.10:3306 root password"
          />
        </div>

        <el-form label-position="top" class="import-form">
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="账号类型">
                <el-select v-model="importType" placeholder="请选择账号类型" style="width: 100%">
                  <el-option
                    v-for="item in accountTypeOptions"
                    :key="item.value"
                    :label="item.label"
                    :value="item.value"
                  />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="批量备注（可选）">
                <el-input
                  v-model="importRemark"
                  maxlength="200"
                  show-word-limit
                  placeholder="可为本次导入统一追加备注"
                />
              </el-form-item>
            </el-col>
          </el-row>
        </el-form>

        <div v-if="importResult" class="import-result" aria-live="polite">
          <div class="import-result-head">
            <h4>导入结果</h4>
            <span>总数 {{ importResult.total }} / 成功 {{ importResult.success }} / 失败 {{ importResult.fail }}</span>
          </div>
          <el-alert
            :type="importResult.fail > 0 ? 'warning' : 'success'"
            :title="importResult.fail > 0 ? '存在失败记录，请检查失败行并修正后重试' : '导入成功，未发现失败记录'"
            show-icon
            :closable="false"
          />
          <div v-if="Array.isArray(importResult.failedRows) && importResult.failedRows.length" class="failed-rows">
            <div
              v-for="(row, index) in importResult.failedRows"
              :key="index"
              class="failed-row-item"
            >
              <span>第 {{ row.row || row.rowNum || row.rowNumber || row.line || '-' }} 行</span>
              <span>{{ row.reason || row.message || row.error || row.msg || '未知错误' }}</span>
            </div>
          </div>
        </div>
      </div>

      <template #footer>
        <div class="dialog-footer">
          <el-button @click="importDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="importing" @click="submitImport">
            {{ importing ? '导入中...' : '开始导入' }}
          </el-button>
        </div>
      </template>
    </el-dialog>
    </el-card>
  </div>
</template>

<script>
import API from '@/api/config'

export default {
  data() {
    return {
      queryParams: {
        alias: '',
        type: undefined,
        pageNum: 1,
        pageSize: 10
      },
      accountTypeOptions: [
        { value: 1, label: 'prometheus', tag: 'success' },
        { value: 2, label: 'grafana', tag: 'warning' },
        { value: 3, label: 'MySQL', tag: 'danger' },
        { value: 4, label: 'Redis', tag: 'info' },
        { value: 5, label: '通用类型', tag: '' },
        { value: 6, label: 'PostgreSQL', tag: '' },
        { value: 7, label: 'Oracle', tag: 'warning' },
        { value: 8, label: 'MongoDB', tag: 'success' },
        { value: 9, label: 'SQL Server', tag: 'info' },
        { value: 10, label: 'Elasticsearch', tag: 'warning' }
      ],
      loading: false,
      accountList: [],
      total: 0,
      dialogVisible: false,
      dialogTitle: '',
      importDialogVisible: false,
      importMode: 'file',
      importing: false,
      importType: 5,
      importRemark: '',
      importText: '',
      importFile: null,
      importFileList: [],
      importResult: null,
      formData: {
        id: '',
        alias: '',
        host: '',
        name: '',
        password: '',
        type: undefined,
        remark: ''
      },
      formRules: {
        alias: [{ required: true, message: '请输入账号别名', trigger: 'blur' }],
        host: [{ required: true, message: '请输入账号地址(格式: IP或域名:端口)', trigger: 'blur' }],
        name: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
        password: [{ required: true, message: '请输入密码', trigger: 'blur' }],
        type: [{ required: true, message: '请选择账号类型', trigger: 'change' }]
      }
    }
  },
  methods: {
    getAccountTypeLabel(type) {
      return this.accountTypeOptions.find(item => item.value === Number(type))?.label || `unknown(${type})`
    },

    getAccountTypeTag(type) {
      const option = this.accountTypeOptions.find(item => item.value === Number(type))
      return option ? option.tag : 'info'
    },

    // 获取账号列表
    normalizeAccountResult(data) {
      const payload = data?.data !== undefined ? data.data : data
      if (Array.isArray(payload)) {
        return { list: payload, total: payload.length }
      }
      if (Array.isArray(payload?.list)) {
        return { list: payload.list, total: Number(payload.total) || payload.list.length }
      }
      if (Array.isArray(payload?.rows)) {
        return { list: payload.rows, total: Number(payload.total) || payload.rows.length }
      }
      if (payload && typeof payload === 'object' && payload.id !== undefined) {
        return { list: [payload], total: 1 }
      }
      return { list: [], total: 0 }
    },

    filterAccountRows(rows) {
      const alias = (this.queryParams.alias || '').trim().toLowerCase()
      const type = Number(this.queryParams.type)
      return rows.filter(item => {
        const aliasMatched = !alias || String(item.alias || '').toLowerCase().includes(alias)
        const typeMatched = !type || Number(item.type) === type
        return aliasMatched && typeMatched
      })
    },

    pageAccountRows(rows) {
      const start = (this.queryParams.pageNum - 1) * this.queryParams.pageSize
      return rows.slice(start, start + this.queryParams.pageSize)
    },

    async getList() {
      this.loading = true
      try {
        const alias = (this.queryParams.alias || '').trim()
        const type = this.queryParams.type
        const useClientFilter = Boolean(alias || type)
        let response

        if (type) {
          response = await API.getAccountAuthByType(type)
        } else {
          response = await API.listAccountAuth({
            page: alias ? 1 : this.queryParams.pageNum,
            pageSize: alias ? 1000 : this.queryParams.pageSize
          })
        }

        const { data: res } = response
        if (res.code === 200) {
          const { list, total } = this.normalizeAccountResult(res)
          const rows = useClientFilter ? this.filterAccountRows(list) : list
          this.accountList = useClientFilter ? this.pageAccountRows(rows) : rows
          this.total = useClientFilter ? rows.length : total
        } else {
          this.$message.error(res.message || '获取账号列表失败')
        }
      } catch (error) {
        console.error('获取账号列表失败:', error)
        this.$message.error('获取账号列表失败')
      } finally {
        this.loading = false
      }
    },
    
    // 搜索
    async handleQuery() {
      this.queryParams.pageNum = 1  // 重置到第一页
      this.getList()
    },
    
    // 重置搜索
    resetQuery() {
      this.queryParams = {
        alias: '',
        type: undefined,
        pageNum: 1,
        pageSize: 10
      }
      this.getList()
    },

    showImportDialog() {
      this.importDialogVisible = true
      this.importResult = null
    },

    handleImportFileChange(file, fileList) {
      const latest = fileList.slice(-1)
      this.importFileList = latest
      this.importFile = latest.length ? (latest[0].raw || latest[0]) : null
      this.importResult = null
    },

    handleImportFileRemove() {
      this.importFile = null
      this.importFileList = []
    },

    handleImportFileExceed(files) {
      const file = files && files[0]
      if (!file) return
      this.importFile = file
      this.importFileList = [{
        name: file.name,
        size: file.size,
        status: 'ready',
        raw: file
      }]
      this.importResult = null
    },

    resetImportDialog() {
      this.importMode = 'file'
      this.importing = false
      this.importType = 5
      this.importRemark = ''
      this.importText = ''
      this.importFile = null
      this.importFileList = []
      this.importResult = null
      this.$refs.importUploadRef?.clearFiles?.()
    },

    normalizeImportResult(result) {
      return {
        total: Number(result?.total || 0),
        success: Number(result?.success || 0),
        fail: Number(result?.fail || 0),
        failedRows: Array.isArray(result?.failedRows) ? result.failedRows : []
      }
    },

    async submitImport() {
      let payload
      let config = {}
      const type = Number(this.importType) > 0 ? Number(this.importType) : 5

      if (this.importMode === 'file') {
        if (!this.importFile) {
          this.$message.warning('请先选择导入文件')
          return
        }

        const fileName = this.importFile.name || ''
        if (!/\.xlsx$/i.test(fileName)) {
          this.$message.warning('仅支持 .xlsx 文件')
          return
        }

        payload = new FormData()
        payload.append('file', this.importFile)
        payload.append('type', String(type))
        const remark = this.importRemark.trim()
        if (remark) {
          payload.append('remark', remark)
        }
        config = {
          headers: {
            'Content-Type': 'multipart/form-data'
          }
        }
      } else {
        const tableData = this.importText.trim()
        if (!tableData) {
          this.$message.warning('请输入要导入的账号数据')
          return
        }

        payload = {
          tableData,
          type
        }
      }

      this.importing = true
      try {
        const { data: res } = await API.importAccountAuth(payload, config)
        if (res.code === 200) {
          this.importResult = this.normalizeImportResult(res.data || {})

          if (this.importResult.fail > 0) {
            this.$message.warning(`导入完成：成功 ${this.importResult.success} 条，失败 ${this.importResult.fail} 条`)
          } else {
            this.$message.success(`导入成功：共 ${this.importResult.success} 条`)
          }

          this.importFile = null
          this.importFileList = []
          this.importText = ''
          this.$refs.importUploadRef?.clearFiles?.()
          await this.getList()
        } else {
          this.$message.error(res.message || '导入失败')
        }
      } catch (error) {
        console.error('导入失败:', error)
        this.$message.error(error.response?.data?.message || error.message || '导入失败')
      } finally {
        this.importing = false
      }
    },
    
    // 分页处理
    handleSizeChange(val) {
      this.queryParams.pageSize = val
      this.queryParams.pageNum = 1
      this.getList()
    },
    
    handleCurrentChange(val) {
      this.queryParams.pageNum = val
      this.getList()
    },
    
    // 显示新增对话框
    showAddDialog() {
      this.dialogTitle = '创建账号'
      this.$nextTick(() => {
        this.formData = {
          id: '',
          alias: '',
          host: '',
          name: '',
          password: '',
          type: undefined,
          remark: ''
        }
        this.dialogVisible = true
      })
    },
    
    // 显示编辑对话框
    showEditDialog(row) {
      this.dialogTitle = '修改账号'
      this.$nextTick(() => {
        this.formData = {
          id: row.id,
          alias: row.alias,
          host: row.host,
          name: row.name,
          password: '',
          type: row.type,
          remark: row.remark,
          createdAt: row.createdAt,  // 保留创建时间
          updatedAt: row.updatedAt   // 保留更新时间
        }
        this.dialogVisible = true
      })
    },
    
    // 提交表单
    async submitForm() {
      try {
        await this.$refs.formRef.validate()

        const formData = {
          ...this.formData,
          type: Number(this.formData.type),
          id: this.formData.id ? Number(this.formData.id) : undefined,
          createdAt: this.formData.createdAt, // 保留创建时间
          updatedAt: this.formData.updatedAt  // 保留更新时间
        }

        let res
        if (formData.id) {
          // 更新 - 仅发送可修改字段
          const updateData = {
            id: formData.id,
            alias: formData.alias,
            host: formData.host,
            name: formData.name,
            password: formData.password,
            type: formData.type,
            remark: formData.remark
          }
          res = await API.updateAccountAuth({
            id: updateData.id,
            alias: updateData.alias,
            host: updateData.host,
            name: updateData.name,
            password: updateData.password,
            type: updateData.type,
            remark: updateData.remark
          })
        } else {
          // 新增 - 确保不发送ID字段
          const createData = {
            alias: formData.alias,
            host: formData.host,
            name: formData.name,
            password: formData.password,
            type: formData.type,
            remark: formData.remark
          }
          res = await API.createAccountAuth(createData)
        }

        if (res.data.code === 200) {
          this.$message.success(formData.id ? '修改成功' : '创建成功')
          this.dialogVisible = false
          await this.getList()
        } else {
          this.$message.error(res.data.message || (formData.id ? '修改失败' : '创建失败'))
        }
      } catch (error) {
        console.error('操作失败:', error)
        this.$message.error('操作失败: ' + error.message)
      }
    },
    
    // 删除账号
    // 测试账号连接
    async handleTest(row) {
      const loading = this.$loading({
        lock: true,
        text: `正在测试 ${row.alias} 连接...`,
        background: 'rgba(0, 0, 0, 0.7)'
      })

      try {
        const res = await API.testAccountAuth(row.id)
        loading.close()

        if (res.data.code === 200 && res.data.data) {
          const { success, message } = res.data.data

          if (success) {
            this.$notify({
              title: '连接成功',
              message: `账号 "${row.alias}" ${message || '连接测试通过'}`,
              type: 'success',
              duration: 3000
            })
          } else {
            this.$notify({
              title: '连接失败',
              message: `账号 "${row.alias}" ${message || '连接测试失败'}`,
              type: 'error',
              duration: 5000
            })
          }
        } else {
          this.$message.error(res.data.message || '测试失败')
        }
      } catch (error) {
        loading.close()
        console.error('测试连接失败:', error)
        this.$notify({
          title: '测试失败',
          message: error.response?.data?.message || error.message || '网络请求失败',
          type: 'error',
          duration: 5000
        })
      }
    },

    // 解密密码
    async handleDecrypt(row) {
      try {
        const res = await API.decryptPassword({}, {
          params: { id: row.id }
        })
        if (res.data.code === 200) {
          this.$alert(`
            <div>
              <p>账号: ${row.alias}</p>
              <p>密码: <span style="color: #1890ff; font-weight: bold; font-size: 18px;">${res.data.data.password}</span></p>
            </div>
          `, '解密结果', {
            confirmButtonText: '确定',
            customClass: 'decrypt-result-alert',
            dangerouslyUseHTMLString: true
          })
        } else {
          this.$message.error(res.data.message || '解密失败')
        }
      } catch (error) {
        console.error('解密失败:', error)
        this.$message.error('解密失败: ' + (error.response?.data?.message || error.message))
      }
    },

    // 删除账号
    async handleDelete(row) {
      try {
        await this.$confirm(`确定删除账号"${row.alias}"?`, '提示', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        })
        
        const res = await API.deleteAccountAuth(row.id)
        if (res.data.code === 200) {
          this.$message.success('删除成功')
          this.getList()
        } else {
          this.$message.error(res.data.message || '删除失败')
        }
      } catch (error) {
        console.error('删除失败:', error)
      }
    }
  },
  created() {
    this.getList()
  }
}
</script>

<style scoped>
.accountauth-management {
  --account-page-height-offset: 136px;
  --aiops-bg-page: #f4f7fb;
  --aiops-surface-strong: rgba(255, 255, 255, 0.92);
  --aiops-surface-warm: rgba(255, 253, 248, 0.94);
  --aiops-text-primary: #172133;
  --aiops-text-secondary: #5f6b7c;
  --aiops-text-muted: #8d97a8;
  --aiops-border-soft: rgba(22, 34, 56, 0.06);
  --aiops-border-cool: #dfe7f2;
  --aiops-border-mid: rgba(207, 217, 230, 0.92);
  --aiops-brand: #4c8bf5;
  --aiops-brand-active: #4d79d8;
  --aiops-brand-soft: rgba(76, 139, 245, 0.12);
  --aiops-success: #3fb97f;
  --aiops-warning: #f3b24b;
  --aiops-danger: #ef6b73;
  --aiops-shadow-soft: 0 20px 60px rgba(17, 24, 39, 0.07);
  box-sizing: border-box;
  height: calc(100vh - var(--account-page-height-offset));
  max-height: calc(100vh - var(--account-page-height-offset));
  min-height: 0;
  padding: 6px;
  overflow: hidden;
  background: transparent;
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.accountauth-card {
  position: relative;
  height: 100%;
  max-height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: 22px;
  border: 1px solid rgba(223, 231, 242, 0.96);
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.96), rgba(247, 250, 255, 0.92));
  box-shadow: var(--aiops-shadow-soft);
  backdrop-filter: blur(18px);
}

.accountauth-card::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 40px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  pointer-events: none;
}

.accountauth-card :deep(.el-card__header) {
  position: relative;
  z-index: 1;
  padding: 16px 20px 12px;
  border-bottom: none;
}

.accountauth-card :deep(.el-card__body) {
  position: relative;
  z-index: 1;
  min-height: 0;
  display: flex;
  flex: 1;
  flex-direction: column;
  padding: 0 10px 10px;
  overflow: hidden;
}

.card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.header-main {
  min-width: 0;
}

.eyebrow {
  margin: 0 0 4px;
  color: var(--aiops-brand);
  font-size: 12px;
  font-weight: 700;
}

.title {
  margin: 0;
  color: var(--aiops-text-primary);
  font-size: 20px;
  line-height: 1.25;
  font-weight: 700;
}

.subtitle {
  margin: 6px 0 0;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.5;
}

.search-section {
  margin-bottom: 8px;
  padding: 12px 14px 2px;
  border-radius: 16px;
  border: 1px solid var(--aiops-border-soft);
  background: rgba(255, 255, 255, 0.58);
}

.search-form {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0 12px;
}

.search-form :deep(.el-form-item) {
  margin-bottom: 10px;
  margin-right: 16px;
}

.search-form :deep(.el-form-item__label) {
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

.search-form :deep(.el-input),
.search-form :deep(.el-select) {
  width: 220px;
}

.action-section {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 8px;
  padding: 0 2px;
}

.action-section .el-button {
  margin-left: 0;
}

.table-section {
  min-height: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  margin-bottom: 0;
  overflow: hidden;
  border-radius: 16px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  background: rgba(255, 255, 255, 0.82);
}

.accountauth-table {
  min-height: 0;
  flex: 1;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  background: transparent;
  border-radius: 16px;
  overflow: hidden;
}

.accountauth-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.accountauth-table :deep(.el-table__body-wrapper) {
  min-height: 0;
}

.accountauth-table :deep(.el-table__header th) {
  height: 38px;
  background: rgba(248, 250, 255, 0.96) !important;
  color: var(--aiops-text-muted) !important;
  font-size: 12px;
  font-weight: 700 !important;
  border-bottom: 1px solid rgba(223, 231, 242, 0.9);
}

.accountauth-table :deep(.el-table__header th .cell) {
  color: var(--aiops-text-muted) !important;
  font-weight: 700 !important;
}

.accountauth-table :deep(.el-table__body tr) {
  transition: background-color 0.2s ease;
}

.accountauth-table :deep(.el-table__body tr > td) {
  height: 42px;
  border-bottom: 1px solid rgba(15, 23, 42, 0.04);
  background: rgba(255, 255, 255, 0.6);
}

.accountauth-table :deep(.el-table__cell) {
  padding: 6px 0;
}

.accountauth-table :deep(.cell) {
  line-height: 20px;
}

.accountauth-table :deep(.el-table__row--striped > td.el-table__cell) {
  background: rgba(247, 250, 255, 0.56);
}

.accountauth-table :deep(.el-table__row:hover) {
  background-color: rgba(76, 139, 245, 0.045) !important;
}

.accountauth-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: rgba(76, 139, 245, 0.045) !important;
}

.cell-main {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  max-width: 100%;
}

.cell-icon {
  width: 14px;
  height: 14px;
  opacity: 0.72;
  flex-shrink: 0;
}

.cell-main span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mono-text {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  color: #486076;
}

.masked-text {
  color: var(--aiops-text-muted);
  letter-spacing: 0;
}

.operation-buttons {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 14px;
  white-space: nowrap;
}

.operation-link {
  appearance: none;
  border: none;
  background: transparent;
  padding: 0;
  color: var(--aiops-brand-active);
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  line-height: 1.4;
  cursor: pointer;
  transition: color 0.2s ease, opacity 0.2s ease;
}

.operation-link:hover,
.operation-link:focus-visible {
  opacity: 0.78;
  outline: none;
}

.operation-success {
  color: #2d9c72;
}

.operation-primary {
  color: var(--aiops-brand-active);
}

.operation-warning {
  color: #b77b24;
}

.operation-danger {
  color: #d85c66;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.accountauth-edit-dialog :deep(.el-dialog),
.accountauth-import-dialog :deep(.el-dialog) {
  border-radius: 28px;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.95));
  box-shadow: 0 36px 110px rgba(15, 23, 42, 0.11);
}

.accountauth-edit-dialog :deep(.el-dialog__header),
.accountauth-import-dialog :deep(.el-dialog__header) {
  margin: 0;
  padding: 20px 24px 16px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.9);
}

.accountauth-edit-dialog :deep(.el-dialog__title),
.accountauth-import-dialog :deep(.el-dialog__title) {
  color: var(--aiops-text-primary);
  font-size: 17px;
  font-weight: 700;
}

.accountauth-edit-dialog :deep(.el-dialog__body),
.accountauth-import-dialog :deep(.el-dialog__body) {
  padding: 22px 24px 8px;
}

.accountauth-edit-dialog :deep(.el-dialog__footer),
.accountauth-import-dialog :deep(.el-dialog__footer) {
  padding: 14px 24px 22px;
}

.import-panel {
  display: grid;
  gap: 14px;
}

.import-mode-group {
  justify-self: start;
}

.import-mode-panel {
  min-height: 120px;
}

.upload-drag-content {
  color: var(--aiops-text-secondary);
}

.upload-drag-content p {
  margin: 0 0 4px;
  font-size: 14px;
  font-weight: 600;
}

.upload-drag-content span {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.import-form {
  margin-top: 2px;
}

.import-result {
  display: grid;
  gap: 10px;
  padding: 12px 14px;
  border-radius: 16px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  background: rgba(255, 255, 255, 0.74);
}

.import-result-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
}

.import-result-head h4 {
  margin: 0;
  color: var(--aiops-text-primary);
  font-size: 14px;
  font-weight: 700;
}

.import-result-head span {
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.failed-rows {
  display: grid;
  gap: 6px;
  max-height: 180px;
  overflow: auto;
  padding-right: 2px;
}

.failed-row-item {
  display: grid;
  grid-template-columns: 96px minmax(0, 1fr);
  gap: 8px;
  padding: 8px 10px;
  border-radius: 12px;
  background: rgba(253, 246, 236, 0.9);
  border: 1px solid rgba(230, 162, 60, 0.24);
  color: #9a6f42;
  font-size: 12px;
  line-height: 1.6;
}

.failed-row-item span:last-child {
  word-break: break-all;
}

.el-tag {
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  height: 22px;
  padding: 0 9px;
  line-height: 20px;
}

.el-button {
  min-height: 32px;
  border-radius: 14px;
  font-weight: 600;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease, opacity 0.2s ease;
}

.el-button:hover {
  transform: translateY(-1px);
}

.search-button,
.import-button {
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
  border-color: transparent;
  box-shadow: 0 10px 18px rgba(76, 139, 245, 0.14);
}

.add-button {
  background: linear-gradient(180deg, #51c78d 0%, #3fb97f 100%);
  border-color: transparent;
  box-shadow: 0 10px 18px rgba(63, 185, 127, 0.14);
}

.reset-button,
.dialog-footer :deep(.el-button--warning) {
  background: linear-gradient(180deg, #f6be63 0%, #f3b24b 100%);
  border-color: transparent;
  color: #ffffff;
  box-shadow: 0 10px 18px rgba(243, 178, 75, 0.14);
}

.el-input :deep(.el-input__wrapper),
.el-select :deep(.el-input__wrapper),
.el-textarea :deep(.el-textarea__inner) {
  background: rgba(252, 253, 255, 0.98);
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 14px;
  box-shadow: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;
}

.el-input :deep(.el-input__inner),
.el-select :deep(.el-input__inner) {
  background: transparent;
  border: none;
  color: var(--aiops-text-primary);
}

.el-input :deep(.el-input__wrapper):hover,
.el-select :deep(.el-input__wrapper):hover,
.el-textarea :deep(.el-textarea__inner):hover {
  border-color: rgba(171, 189, 220, 0.95);
}

.el-input :deep(.el-input__wrapper.is-focus),
.el-select :deep(.el-input__wrapper.is-focus),
.el-textarea :deep(.el-textarea__inner):focus {
  border-color: rgba(120, 156, 221, 0.9);
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
  background: #ffffff;
}

.accountauth-table :deep(.el-loading-mask) {
  background-color: rgba(247, 250, 255, 0.72);
  backdrop-filter: blur(4px);
}

.pagination-section {
  flex-shrink: 0;
  display: flex;
  justify-content: flex-end;
  padding: 8px 2px 0;
}

:global(.home-container .el-main:has(.accountauth-management)) {
  --ops-shell-gutter: 10px;
  --ops-page-padding: 7px;
  --ops-module-gap: 8px;
  padding-top: var(--ops-shell-gutter);
  padding-right: var(--ops-shell-gutter);
  padding-left: var(--ops-shell-gutter);
  overflow-y: hidden;
}

.el-pagination {
  background: transparent;
  color: var(--aiops-text-secondary);
}

.el-pagination :deep(.btn-next),
.el-pagination :deep(.btn-prev),
.el-pagination :deep(.el-pager li) {
  border-radius: 10px;
  margin: 0 2px;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.el-pagination :deep(.btn-next):hover,
.el-pagination :deep(.btn-prev):hover,
.el-pagination :deep(.el-pager li):hover {
  color: var(--aiops-brand);
  background: rgba(76, 139, 245, 0.08);
}

@media (max-width: 1080px) {
  .accountauth-card {
    max-height: 100%;
  }

  .accountauth-card :deep(.el-card__body) {
    overflow: hidden;
  }

  .table-section {
    overflow-x: auto;
    overflow-y: hidden;
  }

  .accountauth-table {
    min-width: 980px;
  }
}

@media (max-width: 720px) {
  .accountauth-management {
    padding: 4px;
  }

  .accountauth-card {
    border-radius: 18px;
  }

  .accountauth-card :deep(.el-card__header) {
    padding: 14px 16px 10px;
  }

  .accountauth-card :deep(.el-card__body) {
    padding: 0 8px 8px;
  }

  .title {
    font-size: 18px;
  }

  .search-form {
    display: grid;
    grid-template-columns: 1fr;
  }

  .search-form :deep(.el-form-item),
  .search-form :deep(.el-input),
  .search-form :deep(.el-select) {
    width: 100%;
    margin-right: 0;
  }

  .action-section,
  .dialog-footer {
    align-items: stretch;
    flex-direction: column;
  }

  .action-section .el-button,
  .dialog-footer .el-button {
    width: 100%;
    margin-left: 0;
  }

  .pagination-section {
    justify-content: flex-start;
    overflow-x: auto;
  }
}

/* DESIGN.md compact workbench overrides */
.accountauth-management {
  --ops-text-heading: #0f172a;
  --ops-text-primary: #172033;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: var(--ops-module-gap, 8px);
  height: calc(100vh - 124px);
  max-height: none;
  min-height: 0;
  padding: var(--ops-page-padding, 7px);
  overflow: hidden;
  background: transparent;
  color: var(--ops-text-primary);
}

.ops-page-header {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 12px;
  min-width: 0;
  padding: 8px 12px;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 10px 28px rgba(80, 124, 180, 0.1);
}

.ops-page-header__icon {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  place-items: center;
  border: 0;
  background: transparent;
}

.ops-page-header__icon img {
  width: 30px;
  height: 30px;
  object-fit: contain;
}

.ops-page-header__content {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.ops-page-header h1 {
  margin: 0;
  color: var(--ops-text-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: 0;
  white-space: nowrap;
}

.ops-page-header__divider {
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  background: var(--ops-border-soft);
}

.ops-page-header__content p {
  margin: 0;
  overflow: hidden;
  color: var(--ops-text-muted);
  font-size: 12px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.accountauth-card {
  height: auto;
  max-height: none;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
  backdrop-filter: none;
}

.accountauth-card::before {
  display: none;
}

.accountauth-card :deep(.el-card__body) {
  min-height: 0;
  padding: 10px;
  overflow: hidden;
}

.search-section {
  flex-shrink: 0;
  margin-bottom: 8px;
  padding: 8px 10px 0;
  border: 1px solid var(--ops-border-soft);
  border-radius: 10px;
  background: #f8fbff;
}

.search-form {
  gap: 0;
  row-gap: 8px;
}

.search-form :deep(.el-form-item) {
  margin-right: 12px;
  margin-bottom: 8px;
}

.search-form :deep(.el-form-item:last-child) {
  margin-right: 0;
}

.search-form :deep(.el-form-item__label) {
  height: 34px;
  padding-right: 8px;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
  line-height: 34px;
}

.search-form :deep(.el-input) {
  width: 220px;
}

.search-form :deep(.el-select),
.account-type-filter {
  width: 200px;
}

.search-form :deep(.el-input__wrapper),
.search-form :deep(.el-select__wrapper) {
  min-height: 34px;
  padding: 0 11px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.search-form :deep(.el-input__wrapper:hover),
.search-form :deep(.el-select__wrapper:hover) {
  border: 0;
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.search-form :deep(.el-input__wrapper.is-focus),
.search-form :deep(.el-select__wrapper.is-focused) {
  border: 0;
  background: #fff;
  box-shadow: 0 0 0 1px #409eff inset;
}

.search-button,
.reset-button,
.add-button,
.import-button {
  height: 34px;
  min-width: 76px;
  border-radius: 8px;
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  box-shadow: none;
  transition: background-color 160ms ease, border-color 160ms ease, box-shadow 160ms ease;
}

.search-button {
  border-color: #409eff;
  background: #409eff;
}

.reset-button,
.import-button {
  border-color: #e6a23c;
  background: #e6a23c;
}

.add-button {
  border-color: #67c23a;
  background: #67c23a;
}

.search-button:hover,
.search-button:focus,
.reset-button:hover,
.reset-button:focus,
.add-button:hover,
.add-button:focus,
.import-button:hover,
.import-button:focus {
  color: #fff;
  transform: none;
  box-shadow: none;
}

.table-section {
  flex: 1;
  min-height: 0;
  margin: 0;
  overflow: hidden;
  border: 1px solid #e3ebf5;
  border-radius: 10px;
  background: #fff;
}

.content-header {
  display: flex;
  min-height: 42px;
  flex-shrink: 0;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 5px 10px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: #fbfdff;
}

.content-header h2 {
  margin: 0;
  color: var(--ops-text-heading);
  font-size: 14px;
  font-weight: 700;
  line-height: 1.3;
}

.resource-count {
  color: var(--ops-text-muted);
  font-size: 12px;
  font-weight: 500;
}

.accountauth-table {
  min-height: 0;
  border: 0;
  border-radius: 0;
  background: #fff;
  color: var(--ops-text-secondary);
  font-size: 12px;
}

.accountauth-table :deep(.el-table__header th) {
  height: 38px;
  padding: 6px 0;
  border-bottom: 1px solid #e3ebf5;
  background: #f3f7fc !important;
}

.accountauth-table :deep(.el-table__header th .cell) {
  color: #475569 !important;
  font-size: 12px;
}

.accountauth-table :deep(.el-table__body tr > td) {
  height: 40px;
  border-bottom-color: #eef3f9;
  background: #fff;
}

.accountauth-table :deep(.el-table__row--striped > td.el-table__cell) {
  background: #fbfdff;
}

.accountauth-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

.accountauth-table :deep(.el-table__cell) {
  padding: 6px 0;
}

.cell-main {
  gap: 7px;
}

.cell-icon {
  width: 16px;
  height: 16px;
  opacity: 0.82;
}

.mono-text {
  color: #475f75;
  font-size: 11px;
}

.accountauth-table :deep(.el-tag) {
  height: 22px;
  padding: 0 7px;
  border-radius: 6px;
  font-size: 11px;
  line-height: 20px;
}

.operation-buttons {
  gap: 14px;
  justify-content: flex-end;
}

.operation-link {
  transform: none;
  transition: color 160ms ease, opacity 160ms ease;
}

.operation-link:focus-visible {
  border-radius: 3px;
  outline: 2px solid rgba(64, 158, 255, 0.5);
  outline-offset: 3px;
}

.pagination-section {
  padding: 8px 2px 0;
}

.el-pagination {
  --el-pagination-button-width: 28px;
  --el-pagination-button-height: 28px;
  font-size: 12px;
}

.el-pagination :deep(.btn-next),
.el-pagination :deep(.btn-prev),
.el-pagination :deep(.el-pager li) {
  border-radius: 7px;
}

.accountauth-management .el-button:hover,
.accountauth-management .el-button:focus {
  transform: none;
}

:global(.accountauth-dialog-overlay .el-dialog),
:global(.el-dialog.accountauth-edit-dialog),
:global(.el-dialog.accountauth-import-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 24px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: 15px !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.accountauth-dialog-overlay .el-dialog__header),
:global(.accountauth-edit-dialog .el-dialog__header),
:global(.accountauth-import-dialog .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: linear-gradient(90deg, #fff6e7 0%, #f7fbff 36%, #ecf7ff 100%);
}

:global(.accountauth-dialog-overlay .el-dialog__title),
:global(.accountauth-edit-dialog .el-dialog__title),
:global(.accountauth-import-dialog .el-dialog__title) {
  color: #172033;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.accountauth-dialog-overlay .el-dialog__headerbtn),
:global(.accountauth-edit-dialog .el-dialog__headerbtn),
:global(.accountauth-import-dialog .el-dialog__headerbtn) {
  top: 12px;
  right: 14px;
  width: 32px;
  height: 32px;
}

:global(.accountauth-dialog-overlay .el-dialog__body),
:global(.accountauth-edit-dialog .el-dialog__body),
:global(.accountauth-import-dialog .el-dialog__body) {
  max-height: calc(100vh - 190px);
  overflow: auto;
  padding: 14px 18px;
  background: #fff;
}

:global(.accountauth-dialog-overlay .el-dialog__footer),
:global(.accountauth-edit-dialog .el-dialog__footer),
:global(.accountauth-import-dialog .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.accountauth-dialog-overlay .el-input__wrapper),
:global(.accountauth-dialog-overlay .el-select__wrapper),
:global(.accountauth-edit-dialog .el-input__wrapper),
:global(.accountauth-edit-dialog .el-select__wrapper),
:global(.accountauth-import-dialog .el-input__wrapper),
:global(.accountauth-import-dialog .el-select__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.accountauth-dialog-overlay .el-textarea__inner),
:global(.accountauth-edit-dialog .el-textarea__inner),
:global(.accountauth-import-dialog .el-textarea__inner) {
  border: 1px solid #d9e5f2;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: none;
}

:global(.accountauth-dialog-overlay .el-form-item),
:global(.accountauth-edit-dialog .el-form-item),
:global(.accountauth-import-dialog .el-form-item) {
  margin-bottom: 16px;
}

:global(.accountauth-dialog-overlay .el-form-item__label),
:global(.accountauth-edit-dialog .el-form-item__label),
:global(.accountauth-import-dialog .el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

:global(.accountauth-dialog-overlay .el-button),
:global(.accountauth-edit-dialog .el-button),
:global(.accountauth-import-dialog .el-button) {
  min-height: 34px;
  border-radius: 8px;
  font-weight: 600;
  transform: none !important;
}

:global(.accountauth-dialog-overlay .el-button--primary),
:global(.accountauth-edit-dialog .el-button--primary),
:global(.accountauth-import-dialog .el-button--primary) {
  border-color: #409eff;
  background: linear-gradient(135deg, #5aa8ff, #2563eb);
  color: #fff;
}

.import-panel {
  gap: 10px;
}

.import-mode-panel {
  min-height: 104px;
}

.import-panel :deep(.el-upload-dragger) {
  padding: 18px 12px;
  border-color: #cfe0f3;
  border-radius: 10px;
  background: #f8fbff;
}

.import-result {
  gap: 8px;
  padding: 10px 12px;
  border-color: #dce8f5;
  border-radius: 10px;
  background: #f8fbff;
}

.failed-row-item {
  padding: 7px 9px;
  border-radius: 8px;
}

@media (max-width: 1080px) {
  .accountauth-card :deep(.el-card__body) {
    overflow: hidden;
  }

  .table-section {
    overflow-x: auto;
    overflow-y: hidden;
  }

  .accountauth-table {
    min-width: 980px;
  }
}

@media (max-width: 768px) {
  .accountauth-management {
    height: calc(100vh - 110px);
    padding: 6px;
    overflow: auto;
  }

  .ops-page-header {
    align-items: flex-start;
    padding: 8px 10px;
  }

  .ops-page-header__content {
    flex-direction: column;
    align-items: stretch;
    gap: 2px;
  }

  .ops-page-header__divider,
  .ops-page-header__content p {
    display: none;
  }

  .accountauth-card {
    border-radius: 12px;
  }

  .accountauth-card :deep(.el-card__body) {
    padding: 8px;
  }

  .search-form {
    display: grid;
    grid-template-columns: 1fr;
  }

  .search-form :deep(.el-form-item),
  .search-form :deep(.el-input),
  .search-form :deep(.el-select),
  .account-type-filter {
    width: 100%;
    margin-right: 0;
  }

  .search-form :deep(.el-form-item:last-child .el-form-item__content) {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
    width: 100%;
  }

  .search-form :deep(.el-form-item:last-child .el-button) {
    width: 100%;
    margin-left: 0;
  }

  .account-form :deep(.el-col-12) {
    max-width: 100%;
    flex: 0 0 100%;
  }

  .dialog-footer {
    align-items: center;
    flex-direction: row;
  }

  .dialog-footer .el-button {
    width: auto;
    margin-left: 0;
  }

  .pagination-section {
    justify-content: flex-start;
    overflow-x: auto;
  }
}

@media (prefers-reduced-motion: reduce) {
  .accountauth-management *,
  .accountauth-management *::before,
  .accountauth-management *::after {
    scroll-behavior: auto !important;
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}
</style>
