<template>
  <div class="ecs-key-page">
    <div class="page-shell">
      <div class="page-card">
        <div class="page-header">
          <div class="page-header-copy">
            <span class="page-header-icon" aria-hidden="true">
              <img src="@/assets/image/凭据.svg" alt="" />
            </span>
            <div class="page-header-content">
            <h2>ECS 凭据管理</h2>
              <span class="page-header-divider" aria-hidden="true"></span>
            <p class="page-description">统一管理密码、公钥与跳板机认证凭据，保持更清晰的接入边界。</p>
            </div>
          </div>
          <div class="page-header-metrics" aria-label="凭据摘要">
            <div>
              <span>凭据</span>
              <strong>{{ total }}</strong>
            </div>
            <div>
              <span>当前页</span>
              <strong>{{ authList.length }}</strong>
            </div>
            <div>
              <span>当前类型</span>
              <strong>{{ authTypeCount }}</strong>
            </div>
          </div>
        </div>

        <div class="content-stack">
          <div class="toolbar-card">
            <div class="toolbar-main">
              <el-form ref="queryForm" :model="queryParams" :inline="true" class="toolbar-form" @submit.prevent>
                <el-form-item label="凭据名称" prop="name">
                  <el-input
                    v-model="queryParams.name"
                    clearable
                    class="credential-keyword-input"
                    placeholder="请输入凭据名称"
                    @keyup.enter="handleQuery"
                  />
                </el-form-item>
                <el-form-item class="toolbar-form-actions-item">
                  <div class="toolbar-form-actions">
                    <el-button class="action-btn action-btn-primary" type="primary" @click="handleQuery"><el-icon><Search /></el-icon>查询</el-button>
                    <el-button class="action-btn action-btn-warning" type="warning" @click="resetQuery"><el-icon><Refresh /></el-icon>重置</el-button>
                    <el-button class="action-btn action-btn-primary" type="primary" :loading="templateDownloading" @click="handleDownloadTemplate"><el-icon v-if="!templateDownloading"><Download /></el-icon>下载模板</el-button>
                    <el-button class="action-btn action-btn-warning" type="warning" @click="showImportDialog"><el-icon><Upload /></el-icon>表格导入</el-button>
                    <el-button
                      v-authority="['config:ecs:create']"
                      class="action-btn action-btn-success"
                      type="success"
                      @click="showAddDialog"
                    >
                      <el-icon><Plus /></el-icon>
                      创建凭据
                    </el-button>
                  </div>
                </el-form-item>
              </el-form>
            </div>
          </div>

          <div class="table-card">
            <div class="table-head">
              <div>
                <h3>凭据列表</h3>
              </div>
            </div>

            <el-table
              v-loading="loading"
              :data="authList"
              stripe
              class="ecs-key-table"
            >
            <el-table-column label="凭据名称" prop="name" min-width="160">
              <template #default="scope">
                <div class="cell-with-icon">
                  <img src="@/assets/image/凭据.svg" class="cell-icon" alt="凭据" />
                  <span class="cell-text">{{ scope.row.name || '-' }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column v-if="showUsernameColumn" label="用户名" prop="username" min-width="120">
              <template #default="scope">
                <div class="cell-with-icon">
                  <img src="@/assets/image/ren.svg" class="cell-icon" alt="用户" />
                  <span class="cell-text">{{ scope.row.username || '-' }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column label="端口" prop="port" width="92">
              <template #default="scope">
                <div class="cell-with-icon cell-with-icon--compact">
                  <img src="@/assets/image/端口.svg" class="cell-icon" alt="端口" />
                  <span class="cell-text">{{ scope.row.port || '-' }}</span>
                </div>
              </template>
            </el-table-column>

            <el-table-column label="认证信息" min-width="132">
              <template #default="scope">
                <div class="cell-with-icon auth-info-cell">
                  <img :src="getAuthIcon(scope.row.type)" class="cell-icon" alt="认证" />
                  <el-tag :type="getAuthTagType(scope.row.type)" effect="light" class="auth-tag">
                    {{ getAuthTypeLabel(scope.row.type) }}
                  </el-tag>
                </div>
              </template>
            </el-table-column>

            <el-table-column label="创建时间" prop="createTime" min-width="150" show-overflow-tooltip />
            <el-table-column label="备注" prop="remark" min-width="150" show-overflow-tooltip>
              <template #default="scope">
                <span class="remark-text">{{ scope.row.remark || '-' }}</span>
              </template>
            </el-table-column>

            <el-table-column label="操作" width="240" align="center">
              <template #default="scope">
                <div class="operation-links">
                  <el-button
                    v-authority="['config:ecs:edit']"
                    link
                    type="primary"
                    @click="showEditDialog(scope.row)"
                  >
                    修改
                  </el-button>
                  <el-button
                    link
                    type="warning"
                    @click="handleDecryptCredential(scope.row)"
                  >
                    解密
                  </el-button>
                  <el-button
                    v-authority="['config:ecs:delete']"
                    link
                    type="danger"
                    @click="handleDelete(scope.row)"
                  >
                    删除
                  </el-button>
                </div>
              </template>
            </el-table-column>
              <template #empty>
                <el-empty description="暂无凭据" :image-size="64" />
              </template>
            </el-table>

            <div class="pagination-section">
              <el-pagination
                :current-page="queryParams.pageNum"
                :page-sizes="[10, 20, 50, 100]"
                :page-size="queryParams.pageSize"
                :total="total"
                layout="total, sizes, prev, pager, next, jumper"
                @size-change="handleSizeChange"
                @current-change="handleCurrentChange"
              />
            </div>
          </div>
        </div>

        <el-dialog
          v-model="dialogVisible"
          :title="dialogTitle"
          width="700px"
          class="credential-dialog"
          modal-class="credential-form-dialog-overlay"
          destroy-on-close
        >
          <el-form
            ref="formRef"
            :model="formData"
            :rules="formRules"
            label-position="top"
            size="small"
            class="credential-form"
          >
            <section class="dialog-section">
              <div class="section-head">
                <h4>基础信息</h4>
                <p>先定义凭据名称与接入方式，再补充具体认证内容。</p>
              </div>

              <el-row :gutter="12">
                <el-col :span="12">
                  <el-form-item label="凭据名称" prop="name">
                    <el-input v-model="formData.name" placeholder="请输入凭据名称" />
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="认证类型" prop="type">
                    <el-radio-group v-model="formData.type" class="auth-type-group">
                      <el-radio-button :label="1">密码认证</el-radio-button>
                      <el-radio-button :label="3">公钥认证</el-radio-button>
                      <el-radio-button :label="4">跳板机认证</el-radio-button>
                    </el-radio-group>
                  </el-form-item>
                </el-col>
              </el-row>
            </section>

            <section v-if="formData.type === 1" class="dialog-section">
              <div class="section-head">
                <h4>密码认证配置</h4>
                <p>适用于直接通过用户名、密码和端口建立连接。</p>
              </div>

              <el-row :gutter="12">
                <el-col :span="12">
                  <el-form-item label="用户名" prop="username">
                    <el-input v-model="formData.username" placeholder="请输入用户名" />
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="端口" prop="port">
                    <el-input v-model.number="formData.port" type="number" :min="1" :max="65535" placeholder="例如 22" />
                  </el-form-item>
                </el-col>
              </el-row>

              <el-form-item label="密码" prop="password">
                <el-input v-model="formData.password" type="password" show-password placeholder="请输入密码" />
              </el-form-item>
            </section>

            <section v-if="formData.type === 3" class="dialog-section">
              <div class="section-head">
                <h4>公钥认证配置</h4>
                <p>系统使用 DevOps 服务器私钥完成认证，这里只保留接入说明。</p>
              </div>

              <el-row :gutter="12">
                <el-col :span="12">
                  <el-form-item label="用户名" prop="username">
                    <el-input v-model="formData.username" placeholder="请输入用户名" />
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="端口" prop="port">
                    <el-input v-model.number="formData.port" type="number" :min="1" :max="65535" placeholder="例如 22" />
                  </el-form-item>
                </el-col>
              </el-row>

              <div class="auth-hint-panel">
                <div class="auth-hint-title">公钥认证配置说明</div>
                <ol class="auth-hint-list">
                  <li>复制 DevOps 服务器公钥：<span class="code-chip">cat ~/.ssh/id_rsa.pub</span></li>
                  <li>将公钥追加到目标主机：<span class="code-chip">echo "公钥内容" &gt;&gt; /root/.ssh/authorized_keys</span></li>
                </ol>
                <p class="form-note">公钥认证无需在系统中额外存储密码或私钥，平台会自动使用已有私钥完成认证。</p>
              </div>
            </section>

            <section v-if="formData.type === 4" class="dialog-section">
              <div class="section-head">
                <h4>跳板机认证配置</h4>
                <p>统一配置跳板机接入方式以及目标主机认证策略。</p>
              </div>

              <div class="subsection-title">跳板机配置</div>
              <el-row :gutter="12">
                <el-col :span="8">
                  <el-form-item label="用户名" prop="bastionUsername">
                    <el-input v-model="formData.bastionUsername" placeholder="例如：admin" />
                  </el-form-item>
                </el-col>
                <el-col :span="8">
                  <el-form-item label="跳板机 IP" prop="bastionHost">
                    <el-input v-model="formData.bastionHost" placeholder="例如：123.45.67.89" />
                  </el-form-item>
                </el-col>
                <el-col :span="8">
                  <el-form-item label="端口" prop="bastionPort">
                    <el-input-number v-model="formData.bastionPort" :min="1" :max="65535" style="width: 100%" />
                  </el-form-item>
                </el-col>
              </el-row>

              <el-form-item label="认证方式" prop="bastionAuthType">
                <el-select v-model="formData.bastionAuthType" style="width: 100%">
                  <el-option label="密码认证" :value="1" />
                  <el-option label="私钥认证" :value="2" />
                  <el-option label="免密认证" :value="3" />
                </el-select>
              </el-form-item>

              <el-form-item v-if="formData.bastionAuthType === 1" label="跳板机密码" prop="bastionPassword">
                <el-input v-model="formData.bastionPassword" type="password" show-password placeholder="请输入跳板机密码" />
              </el-form-item>

              <el-form-item v-if="formData.bastionAuthType === 2" label="跳板机私钥" prop="bastionPrivateKey">
                <el-input
                  v-model="formData.bastionPrivateKey"
                  type="textarea"
                  :rows="6"
                  placeholder="请粘贴私钥内容（PEM 格式）"
                />
              </el-form-item>

              <div class="subsection-title">目标主机认证</div>
              <el-form-item label="认证方式" prop="targetAuthType">
                <el-select v-model="formData.targetAuthType" style="width: 100%">
                  <el-option label="密码认证" :value="1" />
                  <el-option label="跳板机代理免密" :value="4" />
                </el-select>
              </el-form-item>

              <el-form-item v-if="formData.targetAuthType === 1" label="目标主机密码" prop="targetPassword">
                <el-input v-model="formData.targetPassword" type="password" show-password placeholder="请输入目标主机密码" />
              </el-form-item>
            </section>

            <section class="dialog-section dialog-section--compact">
              <el-form-item label="备注" prop="remark">
                <el-input v-model="formData.remark" type="textarea" :rows="2" placeholder="请输入备注信息" />
              </el-form-item>
            </section>
          </el-form>

          <template #footer>
            <div class="dialog-footer">
              <el-button @click="dialogVisible = false">取消</el-button>
              <el-button type="primary" :loading="submitting" @click="submitForm">保存</el-button>
            </div>
          </template>
        </el-dialog>

        <el-dialog
          v-model="importDialogVisible"
          title="批量导入 ECS 密码凭据"
          width="680px"
          class="credential-import-dialog"
          modal-class="credential-import-dialog-overlay"
          destroy-on-close
          @closed="resetImportDialog"
        >
          <div class="import-panel">
            <p class="import-tip">
              仅支持 `.xlsx`，列顺序固定为：
              <span class="code-chip">name</span>
              <span class="code-chip">端口</span>
              <span class="code-chip">用户名</span>
              <span class="code-chip">密码</span>
            </p>

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
                <span>仅支持 xlsx 文件</span>
              </div>
            </el-upload>

            <el-form label-position="top" class="import-form">
              <el-form-item label="批量备注（可选）">
                <el-input
                  v-model="importRemark"
                  type="textarea"
                  :rows="2"
                  maxlength="200"
                  show-word-limit
                  placeholder="可为本次导入统一追加备注"
                />
              </el-form-item>
            </el-form>

            <div v-if="importResult" class="import-result">
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
                  <span>第 {{ row.row || row.rowNum || '-' }} 行</span>
                  <span>{{ row.reason || row.message || '未知错误' }}</span>
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

        <el-dialog
          v-model="decryptDialogVisible"
          title="查看明文凭据"
          width="560px"
          class="credential-decrypt-dialog"
          modal-class="credential-decrypt-dialog-overlay"
          destroy-on-close
          @closed="resetDecryptDialog"
        >
          <div v-loading="decryptLoading" class="decrypt-dialog-body">
            <div class="decrypt-meta-card">
              <div
                v-for="item in decryptMetaItems"
                :key="item.label"
                class="decrypt-meta-row"
              >
                <span class="decrypt-meta-label">{{ item.label }}</span>
                <span class="decrypt-meta-value">{{ item.value }}</span>
              </div>
            </div>

            <div class="decrypt-result-section">
              <div class="decrypt-result-title">解密结果</div>

              <div v-if="decryptFieldItems.length" class="decrypt-result-list">
                <div
                  v-for="item in decryptFieldItems"
                  :key="item.key"
                  class="decrypt-field-row"
                  :class="{ 'decrypt-field-row--block': item.block }"
                >
                  <span class="decrypt-field-label">{{ item.label }}:</span>

                  <button
                    v-if="item.copyable"
                    type="button"
                    class="decrypt-copy-chip"
                    @click="copyDecryptValue(item.value, item.label)"
                  >
                    <span class="decrypt-copy-chip__value">{{ item.value }}</span>
                    <span class="decrypt-copy-chip__action">点击复制</span>
                  </button>

                  <pre v-else-if="item.block" class="decrypt-field-block">{{ item.value }}</pre>

                  <span v-else class="decrypt-field-value">{{ item.value }}</span>
                </div>
              </div>

              <div v-else-if="!decryptLoading" class="decrypt-empty-state">
                接口未返回可展示的明文字段
              </div>
            </div>
          </div>

          <template #footer>
            <div class="dialog-footer">
              <el-button type="primary" @click="decryptDialogVisible = false">关闭</el-button>
            </div>
          </template>
        </el-dialog>
      </div>
    </div>
  </div>
</template>

<script>
import API from '@/api/config'
import {
  Download,
  Plus,
  Refresh,
  Search,
  Upload
} from '@element-plus/icons-vue'

const getDefaultFormData = () => ({
  id: '',
  name: '',
  type: 1,
  username: 'root',
  password: '',
  publicKey: '',
  port: 22,
  remark: '',
  bastionHost: '',
  bastionPort: 22,
  bastionUsername: '',
  bastionAuthType: 1,
  bastionPassword: '',
  bastionPrivateKey: '',
  targetAuthType: 4,
  targetPassword: '',
  targetPrivateKey: ''
})

export default {
  components: {
    Download,
    Plus,
    Refresh,
    Search,
    Upload
  },
  data() {
    return {
      queryParams: {
        name: '',
        pageNum: 1,
        pageSize: 10
      },
      loading: false,
      authList: [],
      total: 0,
      templateDownloading: false,
      dialogVisible: false,
      dialogTitle: '',
      submitting: false,
      formData: getDefaultFormData(),
      decryptDialogVisible: false,
      decryptLoading: false,
      decryptMetaItems: [],
      decryptFieldItems: [],
      importDialogVisible: false,
      importing: false,
      importRemark: '',
      importFile: null,
      importFileList: [],
      importResult: null,
      formRules: {
        name: [{ required: true, message: '请输入凭据名称', trigger: 'blur' }],
        type: [{ required: true, message: '请选择认证类型', trigger: 'change' }],
        username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
        password: [{
          required: true,
          message: '请输入密码',
          trigger: 'blur',
          validator: (rule, value, callback) => {
            if (this.formData.type === 1 && !value) {
              callback(new Error('请输入密码'))
            } else {
              callback()
            }
          }
        }],
        publicKey: [{
          required: false,
          message: '请输入公钥',
          trigger: 'blur'
        }],
        port: [{
          required: true,
          message: '请输入端口号',
          trigger: 'blur',
          validator: (rule, value, callback) => {
            if (!value) {
              callback(new Error('请输入端口号'))
            } else if (isNaN(value) || value < 1 || value > 65535) {
              callback(new Error('请输入1-65535之间的有效端口号'))
            } else {
              callback()
            }
          }
        }]
      }
    }
  },
  computed: {
    authTypeCount() {
      return new Set(this.authList.map(item => Number(item.type)).filter(Boolean)).size
    },
    showUsernameColumn() {
      return this.authList.some(item => Number(item.type) === 1)
    }
  },
  methods: {
    getAuthTypeLabel(type) {
      const currentType = Number(type)
      if (currentType === 1) return '密码认证'
      if (currentType === 3) return '公钥认证'
      if (currentType === 4) return '跳板机认证'
      return '未知类型'
    },
    getAuthTagType(type) {
      const currentType = Number(type)
      if (currentType === 1) return 'success'
      if (currentType === 4) return 'danger'
      return 'info'
    },
    getAuthIcon(type) {
      return Number(type) === 1
        ? require('@/assets/image/密码.svg')
        : require('@/assets/image/密钥.svg')
    },
    getTemplateFileName(disposition) {
      if (!disposition || typeof disposition !== 'string') {
        return 'ecs_auth_template.xlsx'
      }

      const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i)
      if (utf8Match && utf8Match[1]) {
        try {
          return decodeURIComponent(utf8Match[1].replace(/["']/g, ''))
        } catch (error) {
          console.warn('解析UTF-8文件名失败:', error)
        }
      }

      const normalMatch = disposition.match(/filename=([^;]+)/i)
      if (normalMatch && normalMatch[1]) {
        return normalMatch[1].trim().replace(/["']/g, '')
      }

      return 'ecs_auth_template.xlsx'
    },
    async handleDownloadTemplate() {
      this.templateDownloading = true
      try {
        const response = await API.downloadEcsAuthTemplate()
        const blobData = response?.data
        if (!blobData) {
          throw new Error('模板内容为空')
        }

        const fileName = this.getTemplateFileName(
          response?.headers?.['content-disposition'] || response?.headers?.['Content-Disposition']
        )
        const blob = blobData instanceof Blob ? blobData : new Blob([blobData])
        const blobUrl = window.URL.createObjectURL(blob)
        const link = document.createElement('a')
        link.href = blobUrl
        link.download = fileName || 'ecs_auth_template.xlsx'
        document.body.appendChild(link)
        link.click()
        document.body.removeChild(link)
        window.URL.revokeObjectURL(blobUrl)
        this.$message.success('模板下载成功')
      } catch (error) {
        console.error('模板下载失败:', error)
        this.$message.error(error.message || '模板下载失败')
      } finally {
        this.templateDownloading = false
      }
    },
    showImportDialog() {
      this.importDialogVisible = true
      this.importResult = null
    },
    handleImportFileChange(file, fileList) {
      const latest = fileList.slice(-1)
      this.importFileList = latest
      this.importFile = latest.length ? (latest[0].raw || latest[0]) : null
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
    },
    resetImportDialog() {
      this.importing = false
      this.importRemark = ''
      this.importFile = null
      this.importFileList = []
      this.importResult = null
      this.$refs.importUploadRef?.clearFiles?.()
    },
    async submitImport() {
      if (!this.importFile) {
        this.$message.warning('请先选择导入文件')
        return
      }

      const fileName = this.importFile.name || ''
      if (!/\.xlsx$/i.test(fileName)) {
        this.$message.warning('仅支持 .xlsx 文件')
        return
      }

      const formData = new FormData()
      formData.append('file', this.importFile)
      const remark = this.importRemark.trim()
      if (remark) {
        formData.append('remark', remark)
      }

      this.importing = true
      try {
        const { data: res } = await API.importEcsAuthByExcel(formData)
        if (res.code === 200) {
          const result = res.data || {}
          this.importResult = {
            total: Number(result.total || 0),
            success: Number(result.success || 0),
            fail: Number(result.fail || 0),
            failedRows: Array.isArray(result.failedRows) ? result.failedRows : []
          }

          if (this.importResult.fail > 0) {
            this.$message.warning(`导入完成：成功 ${this.importResult.success} 条，失败 ${this.importResult.fail} 条`)
          } else {
            this.$message.success(`导入成功：共 ${this.importResult.success} 条`)
          }

          this.importFile = null
          this.importFileList = []
          this.$refs.importUploadRef?.clearFiles?.()
          await this.getList()
        } else {
          this.$message.error(res.message || '导入失败')
        }
      } catch (error) {
        console.error('导入失败:', error)
        this.$message.error(error.message || '导入失败')
      } finally {
        this.importing = false
      }
    },
    resetFormData() {
      this.formData = getDefaultFormData()
    },
    clearFormValidate() {
      this.$nextTick(() => this.$refs.formRef?.clearValidate())
    },
    normalizeAuthData(payload) {
      if (Array.isArray(payload)) {
        return { list: payload, total: payload.length }
      }

      if (payload && Array.isArray(payload.list)) {
        return { list: payload.list, total: Number(payload.total ?? payload.list.length) }
      }

      if (payload && typeof payload === 'object') {
        if (payload.id || payload.name) {
          return { list: [payload], total: 1 }
        }
      }

      return { list: [], total: 0 }
    },
    getDecryptFieldLabel(field) {
      const labelMap = {
        accessKeyId: 'AccessKey ID',
        accessKeySecret: 'AccessKey Secret',
        bastionAuthType: '堡垒机认证方式',
        bastionHost: '堡垒机地址',
        bastionPassword: '堡垒机密码',
        bastionPort: '堡垒机端口',
        bastionPrivateKey: '堡垒机私钥',
        bastionUsername: '堡垒机用户名',
        name: '凭据名称',
        password: '密码',
        port: '端口',
        privateKey: '私钥',
        publicKey: '公钥',
        remark: '备注',
        targetAuthType: '目标认证方式',
        targetPassword: '目标主机密码',
        targetPrivateKey: '目标主机私钥',
        type: '认证类型',
        username: '用户名'
      }

      return labelMap[field] || field
    },
    stringifyDecryptValue(value) {
      if (value === null || value === undefined) {
        return ''
      }

      if (typeof value === 'string') {
        return value
      }

      if (typeof value === 'number' || typeof value === 'boolean') {
        return String(value)
      }

      try {
        return JSON.stringify(value, null, 2)
      } catch (error) {
        console.warn('Failed to stringify decrypted field:', error)
        return String(value)
      }
    },
    buildDecryptMetaItems(row) {
      return [
        { label: '凭据名称', value: row.name },
        { label: '认证类型', value: this.getAuthTypeLabel(row.type) },
        { label: '用户名', value: row.username },
        { label: '端口', value: row.port },
        { label: '堡垒机地址', value: row.bastionHost },
        { label: '堡垒机用户名', value: row.bastionUsername }
      ].filter(item => item.value !== null && item.value !== undefined && item.value !== '')
    },
    isCopyableDecryptField(field, value) {
      return /password|secret/i.test(String(field || '')) && typeof value === 'string' && !value.includes('\n')
    },
    normalizeDecryptFields(payload) {
      const detail = payload && typeof payload === 'object' && !Array.isArray(payload)
        ? payload
        : { result: payload }
      return Object.entries(detail)
        .filter(([, value]) => value !== null && value !== undefined && value !== '')
        .map(([field, value]) => {
          const text = this.stringifyDecryptValue(value)
          const copyable = this.isCopyableDecryptField(field, text)
          return {
            key: field,
            label: this.getDecryptFieldLabel(field),
            value: text,
            copyable,
            block: !copyable && (text.includes('\n') || text.length > 80)
          }
        })
    },
    async copyDecryptValue(value, label) {
      try {
        if (navigator.clipboard?.writeText) {
          await navigator.clipboard.writeText(value)
        } else {
          const textarea = document.createElement('textarea')
          textarea.value = value
          textarea.style.position = 'fixed'
          textarea.style.left = '-9999px'
          document.body.appendChild(textarea)
          textarea.select()
          document.execCommand('copy')
          document.body.removeChild(textarea)
        }

        this.$message.success(`${label}已复制`)
      } catch (error) {
        console.error('复制失败:', error)
        this.$message.error('复制失败')
      }
    },
    resetDecryptDialog() {
      this.decryptLoading = false
      this.decryptMetaItems = []
      this.decryptFieldItems = []
    },
    async handleDecryptCredential(row) {
      this.decryptDialogVisible = true
      this.decryptLoading = true
      this.decryptMetaItems = this.buildDecryptMetaItems(row)
      this.decryptFieldItems = []

      try {
        const { data: res } = await API.decryptEcsAuth(row.id)
        if (res.code === 200) {
          this.decryptFieldItems = this.normalizeDecryptFields(res.data)
        } else {
          this.$message.error(res.message || '解密失败')
          this.decryptDialogVisible = false
        }
      } catch (error) {
        console.error('解密失败:', error)
        this.$message.error('解密失败: ' + (error.response?.data?.message || error.message))
        this.decryptDialogVisible = false
      } finally {
        this.decryptLoading = false
      }
    },
    async getList() {
      this.loading = true
      try {
        const keyword = typeof this.queryParams.name === 'string' ? this.queryParams.name.trim() : this.queryParams.name
        const params = {
          page: this.queryParams.pageNum,
          pageSize: this.queryParams.pageSize
        }

        if (keyword) {
          params.name = keyword
        }

        const { data: res } = await API.getEcsAuthList({
          ...params
        })
        if (res.code === 200) {
          this.authList = res.data?.list || []
          this.total = res.data?.total || 0
        } else {
          this.$message.error(res.message || '获取凭据列表失败')
        }
      } catch (error) {
        console.error('获取凭据列表失败:', error)
        this.$message.error('获取凭据列表失败')
      } finally {
        this.loading = false
      }
    },
    async handleQuery() {
      this.queryParams.pageNum = 1
      this.queryParams.name = (this.queryParams.name || '').trim()

      if (!this.queryParams.name) {
        await this.getList()
        return
      }

      this.loading = true
      try {
        const { data: res } = await API.getEcsAuthByName(this.queryParams.name)
        if (res.code === 200) {
          const normalized = this.normalizeAuthData(res.data)
          this.authList = normalized.list
          this.total = normalized.total
          return
        }
      } catch (error) {
        console.warn('按名称查询失败，回退列表接口过滤:', error)
      } finally {
        this.loading = false
      }

      await this.getList()
    },
    resetQuery() {
      this.queryParams = {
        name: '',
        pageNum: 1,
        pageSize: 10
      }
      this.getList()
    },
    handleSizeChange(val) {
      this.queryParams.pageSize = val
      this.queryParams.pageNum = 1
      this.getList()
    },
    handleCurrentChange(val) {
      this.queryParams.pageNum = val
      this.getList()
    },
    showAddDialog() {
      this.dialogTitle = '创建凭据'
      this.resetFormData()
      this.dialogVisible = true
      this.clearFormValidate()
    },
    showEditDialog(row) {
      this.dialogTitle = '修改凭据'
      this.formData = {
        ...getDefaultFormData(),
        id: row.id,
        name: row.name,
        type: Number(row.type),
        username: row.username || 'root',
        password: row.type === 1 ? row.password : '',
        publicKey: row.type === 2 ? row.publicKey : '',
        port: row.port || 22,
        remark: row.remark || '',
        bastionHost: row.bastionHost || '',
        bastionPort: row.bastionPort || 22,
        bastionUsername: row.bastionUsername || '',
        bastionAuthType: row.bastionAuthType || 1,
        bastionPassword: row.bastionPassword || '',
        bastionPrivateKey: row.bastionPrivateKey || '',
        targetAuthType: row.targetAuthType || 4,
        targetPassword: row.targetPassword || '',
        targetPrivateKey: row.targetPrivateKey || ''
      }
      this.dialogVisible = true
      this.clearFormValidate()
    },
    async submitForm() {
      try {
        await this.$refs.formRef.validate()
        this.submitting = true

        const typeValue = Number(this.formData.type)
        if (typeValue !== 1 && typeValue !== 3 && typeValue !== 4) {
          throw new Error('请选择有效的认证类型')
        }

        const formData = {
          ...this.formData,
          type: typeValue
        }

        if (typeValue !== 4) {
          delete formData.bastionHost
          delete formData.bastionPort
          delete formData.bastionUsername
          delete formData.bastionAuthType
          delete formData.bastionPassword
          delete formData.bastionPrivateKey
          delete formData.targetAuthType
          delete formData.targetPassword
          delete formData.targetPrivateKey
        } else {
          delete formData.username
          delete formData.password
          delete formData.publicKey
          delete formData.port

          if (formData.bastionAuthType !== 1) {
            delete formData.bastionPassword
          }
          if (formData.bastionAuthType !== 2) {
            delete formData.bastionPrivateKey
          }
          if (formData.targetAuthType !== 1) {
            delete formData.targetPassword
          }
          if (formData.targetAuthType !== 2) {
            delete formData.targetPrivateKey
          }
        }

        let res
        if (formData.id) {
          res = await API.updateEcsAuth(formData)
        } else {
          res = await API.createEcsAuth(formData)
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
      } finally {
        this.submitting = false
      }
    },
    async handleDelete(row) {
      try {
        await this.$confirm(`确定删除凭据"${row.name}"?`, '提示', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        })

        const { data: res } = await API.deleteEcsAuth(row.id)
        if (res.code === 200) {
          this.$message.success('删除成功')
          this.getList()
        } else {
          this.$message.error(res.message || '删除失败')
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
.ecs-key-page {
  --ecs-page-height-offset: 124px;
  --ecs-bg-page: #f4f7fb;
  --ecs-bg-page-soft: #eef3f9;
  --ecs-nav-surface: #e5effd;
  --ecs-nav-surface-deep: #eff4fe;
  --ecs-surface-cool: rgba(247, 250, 255, 0.84);
  --ecs-surface-strong: rgba(255, 255, 255, 0.92);
  --ecs-surface-warm: rgba(255, 253, 248, 0.94);
  --ecs-surface-warm-strong: rgba(255, 248, 239, 0.96);
  --ecs-text-primary: #172133;
  --ecs-text-secondary: #5f6b7c;
  --ecs-text-muted: #8d97a8;
  --ecs-border-soft: rgba(22, 34, 56, 0.06);
  --ecs-border-cool: #dfe7f2;
  --ecs-border-mid: rgba(207, 217, 230, 0.92);
  --ecs-brand: #4c8bf5;
  --ecs-brand-active: #4d79d8;
  --ecs-brand-soft: rgba(76, 139, 245, 0.12);
  --ecs-brand-fade: rgba(76, 139, 245, 0.06);
  --ecs-success: #3fb97f;
  --ecs-success-soft: rgba(63, 185, 127, 0.12);
  --ecs-warning: #f3b24b;
  --ecs-danger: #ef6b73;
  --ecs-danger-soft: rgba(239, 107, 115, 0.12);
  --ecs-shadow-soft: 0 20px 60px rgba(17, 24, 39, 0.07);
  --ecs-shadow-panel: 0 36px 110px rgba(15, 23, 42, 0.11);
  --ecs-topbar-gradient: linear-gradient(90deg, #fef7ef 0%, #f8f8f6 52%, #edf2fd 100%);
  box-sizing: border-box;
  height: calc(100vh - var(--ecs-page-height-offset));
  min-height: 0;
  overflow: hidden;
  padding: 0;
  position: relative;
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
  background: transparent;
}

.page-shell {
  position: relative;
  z-index: 1;
  width: 100%;
  height: 100%;
  min-height: 0;
  margin: 0 auto;
}

.page-card {
  position: relative;
  box-sizing: border-box;
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding: 40px 20px 16px;
  border-radius: 28px;
  border: 1px solid rgba(223, 231, 242, 0.94);
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.96), rgba(247, 250, 255, 0.92));
  box-shadow: 0 18px 48px rgba(17, 24, 39, 0.08);
  overflow: hidden;
  min-width: 0;
  isolation: isolate;
}

.page-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 32px;
  background: var(--ecs-topbar-gradient);
  pointer-events: none;
  z-index: 0;
}

.page-header {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  flex: 0 0 auto;
  margin: -40px -20px 10px;
  padding: 10px 20px 10px;
  background: var(--ecs-topbar-gradient);
  border-bottom: 1px solid rgba(223, 231, 242, 0.94);
}

.page-kicker {
  margin: 0 0 10px;
  color: #6d7b8f;
  font-size: 12px;
  letter-spacing: 0.12em;
  font-weight: 600;
  text-transform: uppercase;
}

.page-header h2 {
  margin: 0;
  color: var(--ecs-text-primary);
  font-size: 22px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: 0;
}

.page-description {
  margin: 5px 0 0;
  max-width: 760px;
  color: var(--ecs-text-secondary);
  font-size: 12px;
  line-height: 1.45;
}

.summary-chip {
  flex-shrink: 0;
  min-width: 104px;
  padding: 8px 12px;
  border-radius: 16px;
  border: 1px solid rgba(214, 223, 235, 0.96);
  background: rgba(255, 255, 255, 0.88);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.65);
}

.summary-chip span {
  display: block;
  color: var(--ecs-text-muted);
  font-size: 11px;
}

.summary-chip strong {
  display: block;
  margin-top: 2px;
  color: var(--ecs-text-primary);
  font-size: 20px;
  font-weight: 700;
}

.content-stack {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}

.toolbar-card,
.table-card {
  border-radius: 24px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  box-shadow: 0 12px 34px rgba(17, 24, 39, 0.05);
  min-width: 0;
}

.toolbar-card {
  flex: 0 0 auto;
  padding: 12px 16px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.95), rgba(248, 251, 255, 0.92));
}

.toolbar-main {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  flex-wrap: wrap;
  min-width: 0;
}

.toolbar-form {
  display: flex;
  align-items: center;
  gap: 10px 12px;
  flex-wrap: wrap;
  min-width: 0;
}

.toolbar-form-actions-item {
  margin-right: 0;
}

.toolbar-form-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  min-width: 0;
}

.toolbar-form-actions :deep(.el-button + .el-button) {
  margin-left: 0;
}

.toolbar-form-actions :deep(.action-btn) {
  height: 32px;
  padding: 0 14px;
  border-radius: 12px;
  font-size: 13px;
  font-weight: 600;
  box-shadow: none !important;
  transform: none !important;
}

.toolbar-form-actions :deep(.action-btn-primary) {
  color: #fff;
  border-color: transparent;
  background: #4c8bf5;
}

.toolbar-form-actions :deep(.action-btn-primary:hover),
.toolbar-form-actions :deep(.action-btn-primary:focus) {
  color: #fff;
  background: #4d79d8;
}

.toolbar-form-actions :deep(.action-btn-warning) {
  color: #fff;
  border-color: transparent;
  background: #f3b24b;
}

.toolbar-form-actions :deep(.action-btn-warning:hover),
.toolbar-form-actions :deep(.action-btn-warning:focus) {
  color: #fff;
  background: #df9c33;
}

.toolbar-form-actions :deep(.action-btn-success) {
  color: #fff;
  border-color: transparent;
  background: #3fb97f;
}

.toolbar-form-actions :deep(.action-btn-success:hover),
.toolbar-form-actions :deep(.action-btn-success:focus) {
  color: #fff;
  background: #319c6a;
}

.toolbar-form-actions :deep(.action-btn-secondary) {
  color: var(--ecs-brand-active);
  border-color: rgba(171, 189, 220, 0.96);
  background: rgba(255, 255, 255, 0.92);
}

.toolbar-form-actions :deep(.action-btn-secondary:hover),
.toolbar-form-actions :deep(.action-btn-secondary:focus) {
  color: var(--ecs-brand);
  border-color: rgba(120, 156, 221, 0.78);
  background: rgba(255, 255, 255, 0.98);
}

.table-card {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 14px 16px 16px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.95), rgba(248, 251, 255, 0.92));
}

.table-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  flex: 0 0 auto;
  margin-bottom: 10px;
  padding-bottom: 10px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.86);
}

.table-head h3 {
  margin: 0;
  color: var(--ecs-text-primary);
  font-size: 18px;
  font-weight: 700;
}

.table-head p {
  margin: 6px 0 0;
  color: var(--ecs-text-muted);
  font-size: 12px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.cell-with-icon {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.cell-with-icon--compact {
  gap: 6px;
}

.cell-icon {
  width: 16px;
  height: 16px;
  object-fit: contain;
  flex-shrink: 0;
}

.cell-text,
.remark-text {
  color: var(--ecs-text-secondary);
}

.auth-info-cell {
  gap: 10px;
}

.auth-tag {
  border-radius: 999px;
}

.operation-links {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  white-space: nowrap;
  flex-wrap: nowrap;
}

.ecs-key-page :deep(.operation-links .el-button) {
  margin: 0;
  padding: 0 2px;
  min-width: auto;
}

.ecs-key-page :deep(.operation-links .el-button > span) {
  overflow: visible;
  text-overflow: clip;
}

.pagination-section {
  display: flex;
  justify-content: flex-end;
  flex: 0 0 auto;
  margin-top: auto;
  padding-top: 12px;
  border-top: 1px solid rgba(223, 231, 242, 0.82);
}

.credential-form {
  color: var(--ecs-text-secondary);
  padding: 14px 14px 10px;
  border-radius: 18px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  background: rgba(255, 255, 255, 0.68);
}

.dialog-section {
  padding: 0;
  border: none;
  border-radius: 0;
  background: transparent;
}

.dialog-section + .dialog-section {
  margin-top: 14px;
}

.dialog-section--compact {
  padding-bottom: 0;
}

.section-head {
  margin-bottom: 10px;
}

.section-head h4,
.subsection-title {
  margin: 0;
  color: var(--ecs-text-primary);
  font-size: 15px;
  font-weight: 600;
}

.section-head p {
  margin: 5px 0 0;
  color: var(--ecs-text-muted);
  font-size: 12px;
  line-height: 1.5;
}

.subsection-title {
  margin-bottom: 10px;
}

.subsection-title + .el-row,
.subsection-title + .el-form-item {
  margin-top: 2px;
}

.auth-type-group {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  width: 100%;
  gap: 8px;
}

.auth-type-group :deep(.el-radio-button__inner) {
  width: 100%;
  min-height: 34px;
  padding: 8px 10px;
  border-radius: 12px !important;
  border-left: 1px solid var(--el-border-color) !important;
  font-size: 13px;
  font-weight: 600;
  box-shadow: none !important;
}

.auth-type-group :deep(.el-radio-button:first-child .el-radio-button__inner) {
  border-left: 1px solid var(--el-border-color) !important;
}

.credential-dialog :deep(.el-form-item) {
  margin-bottom: 14px;
}

.credential-dialog :deep(.el-form-item__label) {
  padding-bottom: 6px;
  font-size: 13px;
  font-weight: 600;
}

.credential-dialog :deep(.el-input__wrapper),
.credential-dialog :deep(.el-textarea__inner),
.credential-dialog :deep(.el-input-number),
.credential-dialog :deep(.el-select__wrapper) {
  border-radius: 12px;
}

.credential-dialog :deep(.el-input__wrapper),
.credential-dialog :deep(.el-select__wrapper),
.credential-dialog :deep(.el-input-number .el-input__wrapper) {
  min-height: 36px;
}

.credential-dialog :deep(.el-textarea__inner) {
  min-height: 84px;
}

.credential-dialog :deep(.el-dialog__header) {
  padding: 16px 20px 14px;
}

.credential-dialog :deep(.el-dialog__body) {
  padding: 16px 20px;
}

.credential-dialog :deep(.el-dialog__footer) {
  padding: 0 20px 16px;
}

.auth-hint-panel {
  padding: 12px 14px;
  border-radius: 14px;
  border: 1px solid rgba(255, 192, 120, 0.58);
  background: linear-gradient(180deg, rgba(255, 248, 238, 0.92), rgba(255, 252, 247, 0.94));
}

.auth-hint-title {
  color: #b65f1c;
  font-size: 14px;
  font-weight: 600;
}

.auth-hint-list {
  margin: 10px 0 0;
  padding-left: 20px;
  color: #9a6f42;
  font-size: 12px;
  line-height: 1.7;
}

.form-note {
  margin: 10px 0 0;
  color: #9d7b5f;
  font-size: 12px;
  line-height: 1.6;
}

.code-chip {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  margin-left: 4px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.82);
  border: 1px solid rgba(255, 226, 197, 0.92);
  color: #7f5a37;
  font-size: 12px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.decrypt-dialog-body {
  display: grid;
  gap: 18px;
}

.decrypt-meta-card {
  margin: -20px -22px 0;
  padding: 22px 22px 18px;
  background: linear-gradient(90deg, rgba(255, 247, 232, 0.98) 0%, rgba(238, 246, 255, 0.98) 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.92);
}

.decrypt-meta-row {
  display: grid;
  grid-template-columns: 110px minmax(0, 1fr);
  gap: 12px;
  align-items: start;
}

.decrypt-meta-row + .decrypt-meta-row {
  margin-top: 10px;
}

.decrypt-meta-label {
  color: var(--ecs-text-muted);
  font-size: 13px;
}

.decrypt-meta-value {
  color: var(--ecs-text-primary);
  font-size: 14px;
  font-weight: 600;
  word-break: break-all;
}

.decrypt-result-section {
  display: grid;
  gap: 14px;
}

.decrypt-result-title {
  color: var(--ecs-text-primary);
  font-size: 16px;
  font-weight: 700;
}

.decrypt-result-list {
  display: grid;
  gap: 12px;
}

.decrypt-field-row {
  display: grid;
  grid-template-columns: 110px minmax(0, 1fr);
  gap: 12px;
  align-items: center;
}

.decrypt-field-row--block {
  grid-template-columns: 1fr;
  align-items: stretch;
}

.decrypt-field-label {
  color: var(--ecs-text-primary);
  font-size: 14px;
  font-weight: 700;
}

.decrypt-field-row--block .decrypt-field-label {
  margin-bottom: -2px;
}

.decrypt-field-value {
  color: var(--ecs-text-secondary);
  word-break: break-all;
}

.decrypt-field-block {
  margin: 0;
  padding: 12px 14px;
  border-radius: 16px;
  border: 1px solid rgba(223, 231, 242, 0.96);
  background: rgba(247, 249, 253, 0.95);
  color: var(--ecs-text-primary);
  white-space: pre-wrap;
  word-break: break-all;
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
  line-height: 1.6;
}

.decrypt-copy-chip {
  position: relative;
  display: inline-flex;
  align-items: center;
  width: fit-content;
  max-width: 100%;
  padding: 11px 52px 11px 14px;
  border: 1px solid rgba(230, 184, 54, 0.96);
  border-radius: 16px;
  background: linear-gradient(135deg, #fff8bd 0%, #ffe27a 100%);
  color: #6e4b00;
  font-size: 15px;
  font-weight: 800;
  line-height: 1.4;
  cursor: pointer;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
  box-shadow: 0 10px 24px rgba(243, 178, 75, 0.18);
}

.decrypt-copy-chip:hover {
  transform: translateY(-1px);
  border-color: rgba(212, 158, 25, 0.98);
  box-shadow: 0 14px 28px rgba(243, 178, 75, 0.24);
}

.decrypt-copy-chip__value {
  display: block;
  text-align: left;
  word-break: break-all;
}

.decrypt-copy-chip__action {
  position: absolute;
  top: 50%;
  right: 10px;
  transform: translateY(-50%);
  padding: 4px 8px;
  border-radius: 999px;
  background: rgba(110, 75, 0, 0.12);
  color: #7b5600;
  font-size: 12px;
  font-weight: 700;
  opacity: 0;
  transition: opacity 0.2s ease;
  pointer-events: none;
}

.decrypt-copy-chip:hover .decrypt-copy-chip__action {
  opacity: 1;
}

.decrypt-empty-state {
  padding: 16px 18px;
  border-radius: 16px;
  background: rgba(247, 249, 253, 0.92);
  color: var(--ecs-text-muted);
  font-size: 13px;
  text-align: center;
}

.import-panel {
  display: grid;
  gap: 14px;
}

.import-tip {
  margin: 0;
  color: var(--ecs-text-secondary);
  font-size: 13px;
  line-height: 1.8;
}

.upload-drag-content {
  color: var(--ecs-text-secondary);
}

.upload-drag-content p {
  margin: 0 0 4px;
  font-size: 14px;
  font-weight: 600;
}

.upload-drag-content span {
  color: var(--ecs-text-muted);
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
  background: rgba(255, 255, 255, 0.75);
}

.import-result-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
}

.import-result-head h4 {
  margin: 0;
  color: var(--ecs-text-primary);
  font-size: 14px;
  font-weight: 700;
}

.import-result-head span {
  color: var(--ecs-text-secondary);
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
  border-radius: 10px;
  background: rgba(255, 248, 238, 0.85);
  border: 1px solid rgba(255, 192, 120, 0.38);
  color: #9a6f42;
  font-size: 12px;
  line-height: 1.6;
}

.failed-row-item span:last-child {
  word-break: break-all;
}

.ecs-key-page :deep(.el-form-item) {
  margin-right: 0;
}

.ecs-key-page :deep(.el-form-item__label) {
  color: var(--ecs-text-primary);
  font-size: 13px;
  font-weight: 600;
}

.ecs-key-page :deep(.toolbar-form .el-form-item) {
  margin-bottom: 0;
}

.ecs-key-page :deep(.el-input__wrapper),
.ecs-key-page :deep(.el-select__wrapper),
.ecs-key-page :deep(.el-textarea__inner),
.ecs-key-page :deep(.el-input-number .el-input__wrapper) {
  border-radius: 16px;
  background: rgba(252, 253, 255, 0.98);
  border: 1px solid transparent;
  box-shadow: inset 0 0 0 1px rgba(214, 223, 235, 0.96) !important;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;
}

.ecs-key-page :deep(.el-input__wrapper:hover),
.ecs-key-page :deep(.el-select__wrapper:hover),
.ecs-key-page :deep(.el-textarea__inner:hover),
.ecs-key-page :deep(.el-input-number .el-input__wrapper:hover) {
  box-shadow: inset 0 0 0 1px rgba(171, 189, 220, 0.96) !important;
}

.ecs-key-page :deep(.el-input__wrapper.is-focus),
.ecs-key-page :deep(.el-select__wrapper.is-focused),
.ecs-key-page :deep(.el-textarea__inner:focus),
.ecs-key-page :deep(.el-input-number .el-input__wrapper.is-focus) {
  background: #ffffff;
  box-shadow: inset 0 0 0 1px rgba(120, 156, 221, 0.96), 0 0 0 4px rgba(76, 139, 245, 0.08) !important;
}

.ecs-key-page :deep(.el-input__inner),
.ecs-key-page :deep(.el-textarea__inner),
.ecs-key-page :deep(.el-select__placeholder) {
  color: var(--ecs-text-secondary);
}

.ecs-key-page :deep(.el-input.is-disabled .el-input__wrapper),
.ecs-key-page :deep(.el-select.is-disabled .el-select__wrapper),
.ecs-key-page :deep(.el-textarea.is-disabled .el-textarea__inner),
.ecs-key-page :deep(.el-input-number.is-disabled .el-input__wrapper) {
  background: rgba(245, 248, 253, 0.92) !important;
  box-shadow: inset 0 0 0 1px rgba(223, 231, 242, 0.96) !important;
  color: var(--ecs-text-muted);
}

.ecs-key-page :deep(.el-button) {
  border-radius: 14px;
  font-weight: 600;
  transition: border-color 0.2s ease, background-color 0.2s ease, color 0.2s ease;
  box-shadow: none !important;
  transform: none !important;
}

.ecs-key-page :deep(.el-button:hover),
.ecs-key-page :deep(.el-button:focus) {
  box-shadow: none !important;
  transform: none !important;
}

.ecs-key-page :deep(.el-button--primary:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: transparent;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
}

.ecs-key-page :deep(.el-button--primary.is-plain) {
  color: var(--ecs-brand-active);
  border-color: rgba(171, 189, 220, 0.96);
  background: rgba(255, 255, 255, 0.92);
}

.ecs-key-page :deep(.el-button--primary.is-link),
.ecs-key-page :deep(.el-button--primary.is-text) {
  color: var(--ecs-brand-active);
  border-color: transparent;
  background: transparent;
}

.ecs-key-page :deep(.el-button--primary.is-link:hover),
.ecs-key-page :deep(.el-button--primary.is-text:hover) {
  color: var(--ecs-brand);
  background: rgba(76, 139, 245, 0.06);
}

.ecs-key-page :deep(.el-button.is-disabled),
.ecs-key-page :deep(.el-button.is-disabled:hover) {
  color: #9aa4b2;
  border-color: rgba(223, 231, 242, 0.96);
  background: rgba(245, 248, 253, 0.92);
}

.ecs-key-page :deep(.el-button--primary.is-disabled),
.ecs-key-page :deep(.el-button--primary.is-disabled:hover) {
  color: rgba(255, 255, 255, 0.86);
  border-color: transparent;
  background: linear-gradient(180deg, #9fc0f6 0%, #8eb1ef 100%);
}

.ecs-key-page :deep(.ecs-key-table.el-table) {
  flex: 1 1 auto;
  min-height: 0;
  height: 100%;
  --el-table-border-color: rgba(223, 231, 242, 0.92);
  --el-table-header-bg-color: rgba(248, 250, 255, 0.94);
  --el-table-tr-bg-color: rgba(255, 255, 255, 0.78);
  --el-table-row-hover-bg-color: rgba(76, 139, 245, 0.05);
  --el-table-current-row-bg-color: rgba(76, 139, 245, 0.07);
  --el-table-bg-color: transparent;
  --el-table-text-color: var(--ecs-text-secondary);
  --el-table-header-text-color: var(--ecs-text-muted);
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 18px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.7);
  width: 100%;
  max-width: 100%;
}

.ecs-key-page :deep(.ecs-key-table .el-table__body-wrapper),
.ecs-key-page :deep(.ecs-key-table .el-scrollbar__wrap) {
  overflow-x: hidden !important;
}

.ecs-key-page :deep(.ecs-key-table .el-table__inner-wrapper::before) {
  display: none;
}

.ecs-key-page :deep(.ecs-key-table th.el-table__cell) {
  font-weight: 600;
}

.ecs-key-page :deep(.ecs-key-table td.el-table__cell),
.ecs-key-page :deep(.ecs-key-table th.el-table__cell.is-leaf) {
  border-bottom-color: rgba(223, 231, 242, 0.7);
}

.ecs-key-page :deep(.ecs-key-table .el-table__row:hover > td.el-table__cell) {
  background: rgba(76, 139, 245, 0.05) !important;
}

.ecs-key-page :deep(.ecs-key-table .el-loading-mask) {
  background: rgba(247, 250, 255, 0.72);
}

.ecs-key-page :deep(.ecs-key-table .el-loading-spinner .path) {
  stroke: var(--ecs-brand);
}

.ecs-key-page :deep(.ecs-key-table .el-loading-spinner .el-loading-text) {
  color: var(--ecs-text-secondary);
}

.ecs-key-page :deep(.el-tag) {
  font-weight: 600;
  border-radius: 999px;
  border-color: transparent;
}

.ecs-key-page :deep(.el-pagination .btn-prev),
.ecs-key-page :deep(.el-pagination .btn-next),
.ecs-key-page :deep(.el-pagination .el-pager li) {
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.88);
  color: var(--ecs-text-secondary);
}

.ecs-key-page :deep(.el-pagination .el-pager li.is-active) {
  background: rgba(231, 239, 252, 0.94);
  color: var(--ecs-brand-active);
}

.ecs-key-page :deep(.el-dialog) {
  border-radius: 30px;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.95));
  box-shadow: var(--ecs-shadow-panel);
}

.ecs-key-page :deep(.el-dialog__header) {
  margin-right: 0;
  padding: 18px 22px 16px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.94);
}

.ecs-key-page :deep(.el-dialog__title) {
  color: var(--ecs-text-primary);
  font-size: 18px;
  font-weight: 700;
}

.ecs-key-page :deep(.el-dialog__body) {
  padding: 20px 22px;
  color: var(--ecs-text-secondary);
}

.ecs-key-page :deep(.el-dialog__footer) {
  padding: 0 22px 20px;
}

@media (max-width: 1200px) {
  .page-header {
    flex-direction: column;
    align-items: stretch;
    margin-bottom: 16px;
  }

  .summary-chip {
    width: fit-content;
    min-width: 112px;
  }
}

@media (max-width: 768px) {
  .ecs-key-page {
    padding: 12px;
  }

  .page-card {
    padding: 44px 18px 18px;
    border-radius: 22px;
  }

  .page-card::before {
    height: 30px;
  }

  .page-header h2 {
    font-size: 24px;
  }

  .page-header {
    margin: -44px -18px 16px;
    padding: 10px 18px 14px;
  }

  .toolbar-main {
    align-items: stretch;
  }

  .toolbar-form,
  .toolbar-form-actions {
    width: 100%;
  }

  .toolbar-form-actions :deep(.el-button) {
    width: 100%;
  }

  .pagination-section {
    justify-content: center;
  }

  .decrypt-meta-card {
    margin-left: -18px;
    margin-right: -18px;
    padding-left: 18px;
    padding-right: 18px;
  }

  .decrypt-meta-row,
  .decrypt-field-row {
    grid-template-columns: 1fr;
    gap: 6px;
  }

  .decrypt-copy-chip {
    width: 100%;
  }
}

/* DESIGN.md: primitive -> semantic -> component tokens */
.ecs-key-page {
  --credential-blue-500: #409eff;
  --credential-blue-600: #2563eb;
  --credential-green-500: #67c23a;
  --credential-yellow-500: #e6a23c;
  --credential-red-500: #ef4444;
  --credential-slate-950: #0f172a;
  --credential-slate-800: #172033;
  --credential-slate-600: #53657d;
  --credential-slate-500: #64748b;
  --credential-slate-400: #7b8aa0;

  --credential-color-primary: var(--credential-blue-500);
  --credential-color-success: var(--credential-green-500);
  --credential-color-warning: var(--credential-yellow-500);
  --credential-color-danger: var(--credential-red-500);
  --credential-text-heading: var(--credential-slate-950);
  --credential-text-primary: var(--credential-slate-800);
  --credential-text-secondary: var(--credential-slate-600);
  --credential-text-muted: var(--credential-slate-500);
  --credential-bg-surface: #fff;
  --credential-bg-surface-soft: #fbfdff;
  --credential-bg-subtle: #f3f7fc;
  --credential-border: #dbe7f4;
  --credential-border-soft: #e7eef7;
  --credential-border-hover: #b9d4ff;

  --credential-card-radius: 12px;
  --credential-control-radius: 8px;
  --credential-dialog-radius: 15px;
  --credential-control-height: 30px;
  --credential-card-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
  --credential-hero-shadow: 0 10px 28px rgba(80, 124, 180, 0.1);

  box-sizing: border-box;
  height: calc(100vh - 124px);
  min-height: 0;
  padding: 7px;
  overflow: hidden;
  color: var(--credential-text-primary);
  background: transparent;
}

:global(.home-container .el-main:has(.ecs-key-page)) {
  padding: 10px;
}

.page-shell {
  height: 100%;
  min-height: 0;
}

.page-card {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: 8px;
  height: 100%;
  min-height: 0;
  padding: 0;
  overflow: hidden;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.page-card::before {
  display: none;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
  margin: 0;
  padding: 8px 12px;
  border: 1px solid var(--credential-border);
  border-radius: var(--credential-card-radius);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 251, 255, 0.95));
  box-shadow: var(--credential-hero-shadow);
}

.page-header-copy {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  gap: 8px;
  min-width: 0;
}

.page-header-icon {
  display: grid;
  flex: 0 0 42px;
  width: 42px;
  height: 42px;
  place-items: center;
  border: 0;
  background: transparent;
}

.page-header-icon img {
  width: 30px;
  height: 30px;
  object-fit: contain;
}

.page-header-content {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  gap: 10px;
  min-width: 0;
}

.page-header h2 {
  flex: 0 0 auto;
  margin: 0;
  color: var(--credential-text-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: 0;
  white-space: nowrap;
}

.page-header-divider {
  flex: 0 0 1px;
  width: 1px;
  height: 22px;
  background: var(--credential-border-soft);
}

.page-description {
  min-width: 0;
  max-width: none;
  margin: 0;
  overflow: hidden;
  color: var(--credential-text-muted);
  font-size: 12px;
  font-weight: 400;
  line-height: 1.5;
  letter-spacing: 0;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.page-header-metrics {
  display: flex;
  align-items: stretch;
  flex: 0 0 auto;
}

.page-header-metrics > div {
  display: grid;
  grid-template-columns: auto auto;
  align-items: baseline;
  gap: 6px;
  min-width: 84px;
  padding: 3px 12px;
  border-left: 1px solid var(--credential-border-soft);
}

.page-header-metrics span {
  color: var(--credential-slate-400);
  font-size: 11px;
  font-weight: 600;
}

.page-header-metrics strong {
  color: var(--credential-text-heading);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 18px;
  line-height: 1;
}

.content-stack {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  min-height: 0;
}

.toolbar-card,
.table-card {
  min-width: 0;
  border: 1px solid var(--credential-border);
  border-radius: var(--credential-card-radius);
  background: rgba(255, 255, 255, 0.97);
  box-shadow: var(--credential-card-shadow);
}

.toolbar-card {
  flex: 0 0 auto;
  padding: 8px 10px;
}

.toolbar-main,
.toolbar-form {
  display: flex;
  align-items: center;
  flex-wrap: nowrap;
  gap: 8px;
  min-width: 0;
}

.toolbar-form {
  flex: 1 1 auto;
}

.toolbar-form-actions-item {
  flex: 1 1 auto;
  margin: 0;
}

.toolbar-form-actions {
  display: flex;
  align-items: center;
  flex-wrap: nowrap;
  gap: 6px;
}

.credential-keyword-input {
  width: 240px;
}

.ecs-key-page :deep(.toolbar-form .el-form-item) {
  margin: 0;
}

.ecs-key-page :deep(.toolbar-form .el-form-item__label) {
  height: var(--credential-control-height);
  padding-right: 7px;
  color: var(--credential-text-secondary);
  font-size: 12px;
  font-weight: 700;
  line-height: var(--credential-control-height);
}

.toolbar-form-actions :deep(.action-btn) {
  height: var(--credential-control-height);
  min-height: var(--credential-control-height);
  padding: 5px 10px;
  border-radius: var(--credential-control-radius);
  color: #fff;
  font-size: 12px;
  font-weight: 600;
}

.toolbar-form-actions :deep(.action-btn-primary) {
  border-color: var(--credential-color-primary);
  background: var(--credential-color-primary);
}

.toolbar-form-actions :deep(.action-btn-primary:hover),
.toolbar-form-actions :deep(.action-btn-primary:focus-visible) {
  border-color: #337ecc;
  background: #337ecc;
}

.toolbar-form-actions :deep(.action-btn-warning) {
  border-color: var(--credential-color-warning);
  background: var(--credential-color-warning);
}

.toolbar-form-actions :deep(.action-btn-warning:hover),
.toolbar-form-actions :deep(.action-btn-warning:focus-visible) {
  border-color: #b88230;
  background: #b88230;
}

.toolbar-form-actions :deep(.action-btn-success) {
  border-color: var(--credential-color-success);
  background: var(--credential-color-success);
}

.toolbar-form-actions :deep(.action-btn-success:hover),
.toolbar-form-actions :deep(.action-btn-success:focus-visible) {
  border-color: #529b2e;
  background: #529b2e;
}

.toolbar-form-actions :deep(.action-btn .el-icon) {
  color: #fff;
}

.table-card {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  padding: 0;
  overflow: hidden;
}

.table-head {
  align-items: center;
  margin: 0;
  padding: 8px 12px;
  border-bottom: 1px solid var(--credential-border-soft);
}

.table-head h3 {
  margin: 0;
  color: var(--credential-text-heading);
  font-size: 15px;
  font-weight: 700;
  line-height: 1.25;
}

.ecs-key-page :deep(.ecs-key-table.el-table) {
  flex: 1 1 auto;
  width: calc(100% - 20px);
  min-height: 0;
  height: 100%;
  margin: 8px 10px 0;
  overflow: hidden;
  border: 1px solid #e3ebf5;
  border-radius: var(--credential-control-radius);
  background: #fff;
  font-size: 12px;
  --el-table-border-color: #e3ebf5;
  --el-table-header-bg-color: var(--credential-bg-subtle);
  --el-table-tr-bg-color: #fff;
  --el-table-row-hover-bg-color: #f8fafc;
  --el-table-current-row-bg-color: #eef6ff;
  --el-table-bg-color: #fff;
  --el-table-text-color: var(--credential-text-secondary);
  --el-table-header-text-color: #475569;
}

.ecs-key-page :deep(.ecs-key-table th.el-table__cell) {
  height: 34px;
  padding: 5px 0;
  color: #475569;
  background: var(--credential-bg-subtle) !important;
  font-size: 12px;
  font-weight: 700;
}

.ecs-key-page :deep(.ecs-key-table td.el-table__cell) {
  padding: 5px 0;
  font-size: 12px;
}

.ecs-key-page :deep(.ecs-key-table td.el-table__cell),
.ecs-key-page :deep(.ecs-key-table th.el-table__cell.is-leaf) {
  border-bottom-color: #eef3f9;
}

.ecs-key-page :deep(.ecs-key-table .el-table__row),
.ecs-key-page :deep(.ecs-key-table .el-table__row:hover) {
  box-shadow: none;
  transform: none;
}

.ecs-key-page :deep(.ecs-key-table .el-table__row:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

.cell-with-icon,
.auth-info-cell {
  gap: 6px;
}

.auth-tag {
  height: 22px;
  border-radius: 999px;
  font-size: 11px;
}

.operation-links {
  gap: 10px;
}

.ecs-key-page :deep(.operation-links .el-button) {
  min-height: 24px;
  font-size: 12px;
}

.pagination-section {
  display: flex;
  flex: 0 0 auto;
  justify-content: flex-end;
  margin: 0;
  padding: 8px 10px 10px;
  border-top: 0;
}

.ecs-key-page :deep(.el-pagination .btn-prev),
.ecs-key-page :deep(.el-pagination .btn-next),
.ecs-key-page :deep(.el-pagination .el-pager li) {
  border-radius: 7px;
  color: var(--credential-text-secondary);
  background: #f8fafc;
}

.ecs-key-page :deep(.el-pagination .el-pager li.is-active) {
  color: #fff;
  background: var(--credential-blue-600);
}

.ecs-key-page :deep(.el-input__wrapper),
.ecs-key-page :deep(.el-select__wrapper),
.ecs-key-page :deep(.el-textarea__inner),
.ecs-key-page :deep(.el-input-number .el-input__wrapper) {
  border: 0;
  border-radius: var(--credential-control-radius);
  background: var(--credential-bg-surface-soft);
  box-shadow: 0 0 0 1px #d9e5f2 inset !important;
  transition: background-color 160ms ease, box-shadow 160ms ease;
}

.ecs-key-page :deep(.el-input__wrapper:hover),
.ecs-key-page :deep(.el-select__wrapper:hover),
.ecs-key-page :deep(.el-textarea__inner:hover) {
  box-shadow: 0 0 0 1px var(--credential-border-hover) inset !important;
}

.ecs-key-page :deep(.el-input__wrapper.is-focus),
.ecs-key-page :deep(.el-select__wrapper.is-focused),
.ecs-key-page :deep(.el-textarea__inner:focus) {
  background: #fff;
  box-shadow: 0 0 0 1px var(--credential-color-primary) inset !important;
}

.credential-keyword-input :deep(.el-input__wrapper) {
  min-height: var(--credential-control-height);
}

.ecs-key-page :deep(.el-button) {
  border-radius: var(--credential-control-radius);
  font-weight: 600;
  transition: color 160ms ease, background-color 160ms ease, border-color 160ms ease, box-shadow 160ms ease;
  transform: none;
}

.credential-form {
  padding: 0;
  border: 0;
  border-radius: 0;
  color: var(--credential-text-secondary);
  background: transparent;
}

.dialog-section + .dialog-section {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid var(--credential-border-soft);
}

.section-head {
  margin-bottom: 10px;
}

.section-head h4,
.subsection-title {
  color: var(--credential-text-heading);
  font-size: 13px;
  font-weight: 700;
}

.section-head p {
  margin-top: 3px;
  color: var(--credential-text-muted);
  font-size: 11px;
  line-height: 1.5;
}

.auth-type-group {
  gap: 6px;
}

.auth-type-group :deep(.el-radio-button__inner) {
  min-height: 34px;
  padding: 8px;
  border-radius: var(--credential-control-radius) !important;
  font-size: 12px;
}

.auth-hint-panel,
.import-result,
.decrypt-meta-card,
.decrypt-field-block,
.decrypt-empty-state {
  border-radius: 10px;
}

.decrypt-copy-chip {
  border-radius: var(--credential-control-radius);
  transition: border-color 160ms ease, box-shadow 160ms ease;
  transform: none;
}

.decrypt-copy-chip:hover {
  box-shadow: 0 8px 18px rgba(243, 178, 75, 0.2);
  transform: none;
}

.failed-row-item {
  border-radius: var(--credential-control-radius);
}

:global(.credential-form-dialog-overlay),
:global(.credential-import-dialog-overlay),
:global(.credential-decrypt-dialog-overlay) {
  --credential-text-heading: #0f172a;
  --credential-text-primary: #172033;
  --credential-text-secondary: #53657d;
  --credential-text-muted: #64748b;
  --credential-bg-surface: #fff;
  --credential-bg-surface-soft: #fbfdff;
  --credential-border-soft: #e7eef7;
  --credential-border-hover: #b9d4ff;
  --credential-color-primary: #409eff;
  --credential-control-radius: 8px;
  --ecs-text-primary: #172033;
  --ecs-text-secondary: #53657d;
  --ecs-text-muted: #64748b;
  --ecs-border-cool: #dbe7f4;
  --ecs-brand: #409eff;
  --ecs-brand-active: #2563eb;
  --ecs-warning: #e6a23c;
}

:global(.credential-form-dialog-overlay .el-dialog),
:global(.credential-import-dialog-overlay .el-dialog),
:global(.credential-decrypt-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 24px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.credential-dialog .el-dialog__header),
:global(.credential-import-dialog .el-dialog__header),
:global(.credential-decrypt-dialog .el-dialog__header) {
  position: relative;
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: linear-gradient(90deg, #fff6e7 0%, #f7fbff 36%, #ecf7ff 100%);
}

:global(.credential-dialog .el-dialog__header::before),
:global(.credential-import-dialog .el-dialog__header::before),
:global(.credential-decrypt-dialog .el-dialog__header::before) {
  content: '';
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 3px;
  background: linear-gradient(90deg, #f5a524 0%, #62a8ff 48%, #55d6be 100%);
}

:global(.credential-dialog .el-dialog__title),
:global(.credential-import-dialog .el-dialog__title),
:global(.credential-decrypt-dialog .el-dialog__title) {
  color: var(--credential-text-heading);
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.credential-dialog .el-dialog__headerbtn),
:global(.credential-import-dialog .el-dialog__headerbtn),
:global(.credential-decrypt-dialog .el-dialog__headerbtn) {
  top: 10px;
  right: 12px;
}

:global(.credential-dialog .el-dialog__body),
:global(.credential-import-dialog .el-dialog__body),
:global(.credential-decrypt-dialog .el-dialog__body) {
  max-height: calc(100vh - 190px);
  padding: 16px 18px;
  overflow-y: auto;
  color: var(--credential-text-secondary);
}

:global(.credential-dialog .el-dialog__footer),
:global(.credential-import-dialog .el-dialog__footer),
:global(.credential-decrypt-dialog .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #fff;
}

:global(.credential-dialog .el-form-item),
:global(.credential-import-dialog .el-form-item) {
  margin-bottom: 14px;
}

:global(.credential-dialog .el-form-item__label),
:global(.credential-import-dialog .el-form-item__label) {
  height: auto;
  margin-bottom: 6px;
  padding: 0;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.4;
}

:global(.credential-dialog .el-input__wrapper),
:global(.credential-dialog .el-select__wrapper),
:global(.credential-dialog .el-input-number),
:global(.credential-dialog .el-textarea__inner),
:global(.credential-import-dialog .el-input__wrapper),
:global(.credential-import-dialog .el-textarea__inner) {
  border-radius: var(--credential-control-radius);
  background: var(--credential-bg-surface-soft);
  box-shadow: 0 0 0 1px #d9e5f2 inset !important;
}

:global(.credential-dialog .el-input__wrapper),
:global(.credential-dialog .el-select__wrapper),
:global(.credential-dialog .el-input-number),
:global(.credential-import-dialog .el-input__wrapper) {
  min-height: 34px;
}

:global(.credential-dialog .el-textarea__inner),
:global(.credential-import-dialog .el-textarea__inner) {
  min-height: 72px !important;
}

:global(.credential-dialog .dialog-footer .el-button),
:global(.credential-import-dialog .dialog-footer .el-button),
:global(.credential-decrypt-dialog .dialog-footer .el-button) {
  min-width: 78px;
  min-height: 32px;
  border-radius: var(--credential-control-radius);
  font-weight: 600;
}

:global(.credential-dialog .dialog-footer .el-button--primary),
:global(.credential-import-dialog .dialog-footer .el-button--primary),
:global(.credential-decrypt-dialog .dialog-footer .el-button--primary) {
  border-color: #409eff;
  color: #fff;
  background: linear-gradient(135deg, #5aa8ff, #2563eb);
}

@media (max-width: 1180px) {
  .toolbar-form,
  .toolbar-form-actions {
    flex-wrap: wrap;
  }

  .toolbar-form-actions-item {
    flex-basis: 100%;
  }
}

@media (max-width: 768px) {
  .ecs-key-page {
    height: auto;
    padding: 6px;
    overflow: visible;
  }

  .page-card {
    grid-template-rows: auto auto;
    padding: 0;
    border-radius: 0;
  }

  .page-header {
    align-items: stretch;
    flex-direction: column;
    margin: 0;
    padding: 8px 12px;
  }

  .page-header-content {
    align-items: flex-start;
    flex-direction: column;
    gap: 3px;
  }

  .page-header-divider {
    display: none;
  }

  .page-description {
    width: 100%;
    white-space: normal;
  }

  .page-header-metrics {
    width: 100%;
    border-top: 1px solid var(--credential-border-soft);
  }

  .page-header-metrics > div {
    flex: 1;
    justify-content: center;
    min-width: 0;
    padding: 7px 4px 2px;
    border-left: 0;
  }

  .toolbar-form,
  .toolbar-form-actions,
  .credential-keyword-input {
    width: 100%;
  }

  .toolbar-form-actions :deep(.action-btn) {
    width: auto;
  }

  .table-card {
    min-height: 520px;
  }

  :global(.credential-dialog .el-col) {
    max-width: 100%;
    flex: 0 0 100%;
  }
}
</style>
