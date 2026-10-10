<template>
  <div class="key-management">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/key-clean.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>云厂商密钥管理</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>统一维护云厂商密钥、主机同步与定时采集配置。</p>
      </div>
      <div class="header-summary" aria-label="资源统计">
        <span>密钥 <strong>{{ total }}</strong></span>
        <span class="header-summary__divider" aria-hidden="true"></span>
        <span>同步 <strong>{{ scheduleTotal }}</strong></span>
      </div>
    </header>

    <el-card shadow="never" class="key-card">
      <!-- 搜索表单 -->
      <div class="search-section">
        <el-form :model="queryParams" :inline="true" class="search-form" ref="queryForm">
          <el-form-item v-if="activeTab === 'keys'" label="云厂商类型" prop="keyType">
            <el-select placeholder="请选择云厂商类型" clearable v-model="queryParams.keyType" class="filter-select">
              <el-option v-for="item in keyTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button v-if="activeTab === 'keys'" type="primary" icon="Search" class="search-button" @click="handleQuery">查询</el-button>
            <el-button v-if="activeTab === 'keys'" type="warning" icon="Refresh" class="reset-button" @click="resetQuery">重置</el-button>
            <el-button
              v-if="activeTab === 'keys'"
              type="success"
              icon="Plus"
              class="create-button"
              v-authority="['config:keymanage:create']"
              @click="showAddDialog"
            >
              创建密钥
            </el-button>
            <el-button
              v-else
              type="success"
              icon="Plus"
              class="create-button"
              @click="showSyncScheduleDialog"
            >
              创建定时同步
            </el-button>
          </el-form-item>
        </el-form>
      </div>
    
      <!-- 标签页导航 -->
      <el-tabs v-model="activeTab" class="config-tabs" @tab-change="handleTabChange">
        <el-tab-pane label="密钥管理" name="keys" />
        <el-tab-pane label="定时同步配置" name="schedule" />
      </el-tabs>

      <div class="content-header">
        <h2>{{ activeTab === 'keys' ? '密钥列表' : '定时同步列表' }}</h2>
        <span class="resource-count">
          {{ activeTab === 'keys' ? `共 ${total} 个密钥配置` : `共 ${scheduleTotal} 个同步任务` }}
        </span>
      </div>
    
      <!-- 密钥列表区域 -->
      <div class="table-section" v-show="activeTab === 'keys'">
        <el-table
          :data="keyList"
          v-loading="loading"
          element-loading-text="加载中..."
          class="resource-table"
          empty-text="暂无密钥配置"
          show-overflow-tooltip
        >
          <el-table-column label="云厂商类型" prop="keyType" width="180" min-width="150">
            <template #default="{ row }">
              <div class="resource-name">
                <img :src="getKeyTypeIcon(row.keyType)" class="cloud-icon"/>
                <el-tag :type="getKeyTypeTag(row.keyType).type">
                  {{ getKeyTypeTag(row.keyType).label }}
                </el-tag>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="密钥名称" prop="keyName" min-width="150">
            <template #default="{ row }">
              <span class="key-name-text">{{ row.keyName || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="密钥ID" prop="keyId" min-width="180">
            <template #default="{ row }">
              <div class="key-info">
                <el-icon class="key-icon"><Key /></el-icon>
                <span class="key-value">{{ maskKey(row.keyId) }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="密钥Secret" prop="keySecret" min-width="180">
            <template #default="{ row }">
              <div class="key-info">
                <el-icon class="secret-icon"><Lock /></el-icon>
                <span class="key-value">{{ maskKey(row.keySecret) }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="创建时间" prop="createdAt" width="180">
            <template #default="{ row }">
              <span class="time-text">{{ row.createdAt }}</span>
            </template>
          </el-table-column>
          <el-table-column label="备注" prop="remark" min-width="150">
            <template #default="{ row }">
              <span class="remark-text">{{ row.remark || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="210" fixed="right" align="right" header-align="right">
            <template #default="{ row }">
              <div class="operation-buttons">
                <button type="button" class="operation-link operation-warning" v-authority="['config:keymanage:decrypt']" @click="handleDecryptKey(row)">查看密钥</button>
                <button type="button" class="operation-link operation-primary" v-authority="['config:keymanage:edit']" @click="showEditDialog(row)">修改</button>
                <el-dropdown trigger="click">
                  <button type="button" class="operation-link operation-more">更多</button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item v-authority="['config:keymanage:rsync']" @click="handleSyncHosts(row)">同步主机</el-dropdown-item>
                      <el-dropdown-item v-authority="['config:keymanage:delete']" divided @click="handleDelete(row)">删除</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 定时同步配置列表区域 -->
      <div class="table-section" v-show="activeTab === 'schedule'">
        <el-table
          :data="scheduleList"
          v-loading="scheduleLoading"
          element-loading-text="加载中..."
          class="resource-table"
          empty-text="暂无定时同步配置"
          show-overflow-tooltip
        >
          <el-table-column label="配置名称" prop="name" min-width="160">
            <template #default="{ row }">
              <div class="resource-name">
                <el-icon class="schedule-icon"><Setting /></el-icon>
                <span class="schedule-name">{{ row.name }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="Cron表达式" prop="cronExpr" min-width="150">
            <template #default="{ row }">
              <div class="cron-info">
                <el-icon class="cron-icon"><Timer /></el-icon>
                <code class="cron-text">{{ row.cronExpr }}</code>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="云账号" prop="keyTypes" min-width="200">
            <template #default="{ row }">
              <div class="cloud-accounts">
                <el-tooltip
                  v-for="keyId in parseKeyTypes(row.keyTypes)"
                  :key="keyId"
                  :content="getKeyTagByKeyId(keyId).label"
                  placement="top"
                >
                  <img :src="getKeyTagByKeyId(keyId).icon" class="cloud-account-icon"/>
                </el-tooltip>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="状态" prop="status" width="100">
            <template #default="{ row }">
              <div class="status-control">
                <el-switch
                  v-model="row.status"
                  :active-value="1"
                  :inactive-value="0"
                  active-color="#3fb97f"
                  inactive-color="#d2d9e5"
                  @change="handleToggleStatus(row)"
                />
                <span class="status-text">{{ row.status === 1 ? '启用' : '禁用' }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="下次同步时间" prop="nextRunTime" width="180">
            <template #default="{ row }">
              <div class="next-run-time">
                <el-icon class="time-icon"><Timer /></el-icon>
                <span class="time-text">{{ row.nextRunTime || '-' }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="备注" prop="remark" min-width="120">
            <template #default="{ row }">
              <span class="remark-text">{{ row.remark || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="创建时间" prop="createdAt" width="180">
            <template #default="{ row }">
              <span class="time-text">{{ row.createdAt }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="210" fixed="right" align="right" header-align="right">
            <template #default="{ row }">
              <div class="operation-buttons">
                <button type="button" class="operation-link operation-success" @click="handleTriggerSync(row)">立即同步</button>
                <button type="button" class="operation-link operation-warning" @click="showSyncLog(row)">同步日志</button>
                <el-dropdown trigger="click">
                  <button type="button" class="operation-link operation-more">更多</button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item @click="showEditSyncScheduleDialog(row)">编辑</el-dropdown-item>
                      <el-dropdown-item divided @click="handleDeleteSchedule(row)">删除</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 分页区域 -->
      <div class="pagination-section" v-show="activeTab === 'keys'">
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

      <!-- 定时同步分页区域 -->
      <div class="pagination-section" v-show="activeTab === 'schedule'">
        <el-pagination
          @size-change="handleScheduleSizeChange"
          @current-change="handleScheduleCurrentChange"
          :current-page="scheduleQueryParams.pageNum"
          :page-sizes="[10, 20, 50, 100]"
          :page-size="scheduleQueryParams.pageSize"
          layout="total, sizes, prev, pager, next, jumper"
          :total="scheduleTotal"
        />
      </div>
    
      <!--新增/编辑对话框-->
      <el-dialog
        :title="dialogTitle"
        v-model="dialogVisible"
        width="640px"
        class="cloud-edit-dialog"
        modal-class="cloud-account-dialog-overlay"
      >
        <el-form :model="formData" :rules="formRules" ref="formRef" label-width="104px" class="cloud-form">
          <el-form-item label="云厂商类型" prop="keyType">
            <el-select v-model="formData.keyType" placeholder="请选择云厂商类型" class="full-select">
              <el-option v-for="item in keyTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="密钥名称" prop="keyName">
            <el-input v-model="formData.keyName" placeholder="请输入密钥名称"></el-input>
          </el-form-item>
          <el-form-item label="密钥ID" prop="keyId">
            <el-input v-model="formData.keyId" placeholder="请输入密钥ID" show-password></el-input>
          </el-form-item>
          <el-form-item label="密钥Secret" prop="keySecret">
            <el-input v-model="formData.keySecret" placeholder="请输入密钥Secret" show-password></el-input>
          </el-form-item>
          <el-form-item label="备注信息" prop="remark">
            <el-input v-model="formData.remark" type="textarea" :rows="3" placeholder="请输入备注信息"></el-input>
          </el-form-item>
        </el-form>
        <template #footer>
          <span class="dialog-footer">
            <el-button @click="dialogVisible = false">取消</el-button>
            <el-button type="primary" :loading="submitting" @click="submitForm">
              {{ formData.id ? '保存修改' : '创建密钥' }}
            </el-button>
          </span>
        </template>
      </el-dialog>

      <!--查看密钥对话框-->
      <el-dialog
        title="查看密钥"
        v-model="decryptDialogVisible"
        width="680px"
        class="cloud-secret-dialog"
        modal-class="cloud-account-dialog-overlay"
      >
        <div v-loading="decryptLoading" class="decrypt-container">
          <el-alert
            title="安全提示"
            type="warning"
            :closable="false"
            class="security-alert"
          >
            请妥善保管密钥信息，不要泄露给他人
          </el-alert>
          <div v-if="decryptedKey" class="decrypt-info">
            <el-form label-width="140px">
              <el-form-item label="云厂商类型">
                <el-tag :type="getKeyTypeTag(decryptedKey.keyType).type">
                  {{ getKeyTypeTag(decryptedKey.keyType).label }}
                </el-tag>
              </el-form-item>
              <el-form-item label="密钥名称">
                <span>{{ decryptedKey.keyName || '-' }}</span>
              </el-form-item>
              <el-form-item label="AccessKey ID">
                <div class="key-display">
                  <el-input
                    v-model="decryptedKey.keyId"
                    readonly
                  >
                    <template #append>
                      <el-button @click="copyToClipboard(decryptedKey.keyId, 'AccessKey ID')" icon="DocumentCopy">
                        复制
                      </el-button>
                    </template>
                  </el-input>
                </div>
              </el-form-item>
              <el-form-item label="AccessKey Secret">
                <div class="key-display">
                  <el-input
                    v-model="decryptedKey.keySecret"
                    readonly
                  >
                    <template #append>
                      <el-button @click="copyToClipboard(decryptedKey.keySecret, 'AccessKey Secret')" icon="DocumentCopy">
                        复制
                      </el-button>
                    </template>
                  </el-input>
                </div>
              </el-form-item>
            </el-form>
          </div>
        </div>
        <template #footer>
          <el-button type="primary" @click="decryptDialogVisible = false">关闭</el-button>
        </template>
      </el-dialog>

      <!--同步日志对话框-->
      <el-dialog
        title="同步日志"
        v-model="syncLogDialogVisible"
        width="760px"
        class="cloud-log-dialog"
        modal-class="cloud-account-dialog-overlay"
      >
        <div v-loading="syncLogLoading" class="sync-log-container">
          <div v-if="currentSyncLog" class="log-info">
            <div class="log-header">
              <div class="log-header-item">
                <span class="log-label">配置名称:</span>
                <span class="log-value">{{ currentSyncLog.name }}</span>
              </div>
              <div class="log-header-item">
                <span class="log-label">上次执行时间:</span>
                <span class="log-value">{{ currentSyncLog.lastRunTime || '-' }}</span>
              </div>
              <div class="log-header-item">
                <span class="log-label">下次执行时间:</span>
                <span class="log-value">{{ currentSyncLog.nextRunTime || '-' }}</span>
              </div>
            </div>
            <div class="log-content">
              <div class="log-title">同步日志:</div>
              <pre class="log-text">{{ currentSyncLog.syncLog || '暂无日志' }}</pre>
            </div>
          </div>
          <el-empty v-else description="暂无日志数据" />
        </div>
        <template #footer>
          <el-button @click="syncLogDialogVisible = false">关闭</el-button>
        </template>
      </el-dialog>

      <!--定时同步配置对话框-->
      <el-dialog
        :title="scheduleDialogTitle"
        v-model="scheduleDialogVisible"
        width="680px"
        class="cloud-schedule-dialog"
        modal-class="cloud-account-dialog-overlay"
      >
        <el-form
          :model="scheduleFormData"
          :rules="scheduleFormRules"
          ref="scheduleFormRef"
          label-width="104px"
          class="cloud-form"
        >
          <el-form-item label="配置名称" prop="name">
            <el-input v-model="scheduleFormData.name" placeholder="请输入配置名称"></el-input>
          </el-form-item>
          <el-form-item label="Cron表达式" prop="cronExpr">
            <el-input
              v-model="scheduleFormData.cronExpr"
              placeholder="请输入Cron表达式"
              @input="handleCronExprChange"
            >
              <template #append>
                <el-popover placement="bottom" width="300" trigger="click">
                  <template #reference>
                    <el-button>模板</el-button>
                  </template>
                  <div class="cron-templates">
                    <div
                      v-for="template in cronTemplates"
                      :key="template.value"
                      class="cron-template-item"
                      @click="selectCronTemplate(template.value)"
                    >
                      <strong>{{ template.label }}</strong>: {{ template.value }}
                    </div>
                  </div>
                </el-popover>
              </template>
            </el-input>

            <!-- Cron表达式预览 - 参考TaskJob.vue的简洁设计 -->
            <div v-if="scheduleFormData.cronExpr && nextExecutionTimes.length > 0" class="cron-preview cron-preview-success">
              <span>
                下次执行时间: {{ nextExecutionTimes[0] }}
              </span>
            </div>
            <div v-else-if="scheduleFormData.cronExpr && cronPreviewError" class="cron-preview cron-preview-error">
              <span>
                {{ cronPreviewError }}
              </span>
            </div>
          </el-form-item>
          <el-form-item label="云账号选择" prop="keyTypes">
            <el-select
              v-model="scheduleFormData.keyTypes"
              placeholder="请选择云账号"
              multiple
              collapse-tags
              collapse-tags-tooltip
              class="full-select"
            >
              <el-option
                v-for="key in keyList"
                :key="key.id"
                :label="`${getKeyTypeTag(key.keyType).label} - ${key.keyName || key.keyId}`"
                :value="key.id"
              >
                <div class="cloud-option">
                  <img :src="getKeyTypeIcon(key.keyType)" class="cloud-option-icon"/>
                  <el-tag :type="getKeyTypeTag(key.keyType).type" size="small">
                    {{ getKeyTypeTag(key.keyType).label }}
                  </el-tag>
                  <span>{{ key.keyName || maskKey(key.keyId) }}</span>
                </div>
              </el-option>
            </el-select>
          </el-form-item>
          <el-form-item label="启用状态" prop="status">
            <el-switch
              v-model="scheduleFormData.status"
              :active-value="1"
              :inactive-value="0"
              active-text="启用"
              inactive-text="禁用"
            />
          </el-form-item>
          <el-form-item label="备注信息" prop="remark">
            <el-input
              v-model="scheduleFormData.remark"
              type="textarea"
              :rows="3"
              placeholder="请输入备注信息"
            ></el-input>
          </el-form-item>
        </el-form>
        <template #footer>
          <span class="dialog-footer">
            <el-button @click="scheduleDialogVisible = false">取消</el-button>
            <el-button type="primary" :loading="scheduleSubmitting" @click="submitScheduleForm">
              {{ scheduleFormData.id ? '保存修改' : '创建配置' }}
            </el-button>
          </span>
        </template>
      </el-dialog>
    </el-card>
  </div>
</template>

<script>
import { Setting, Key, Lock, Timer } from '@element-plus/icons-vue'
import API from '@/api/config'

export default {
  components: {
    Setting,
    Key,
    Lock,
    Timer
  },
  data() {
    return {
      activeTab: 'keys',
      queryParams: {
        keyType: '',
        pageNum: 1,
        pageSize: 10
      },
      loading: false,
      keyList: [],
      total: 0,
      dialogVisible: false,
      dialogTitle: '',
      submitting: false,
      scheduleQueryParams: {
        pageNum: 1,
        pageSize: 10
      },
      scheduleLoading: false,
      scheduleList: [],
      scheduleTotal: 0,
      scheduleDialogVisible: false,
      scheduleDialogTitle: '',
      scheduleSubmitting: false,
      syncLogDialogVisible: false,
      syncLogLoading: false,
      currentSyncLog: null,
      decryptDialogVisible: false,
      decryptLoading: false,
      decryptedKey: null,
      keyTypeOptions: [
        { value: 1, label: '阿里云' },
        { value: 2, label: '腾讯云' },
        { value: 3, label: '百度云' },
        { value: 4, label: '华为云' },
        { value: 5, label: 'AWS云' },
        { value: 6, label: '火山引擎' },
        { value: 7, label: '飞书应用' }
      ],
      cronTemplates: [
        { label: '每天凌晨2点', value: '0 0 2 * * ?' },
        { label: '每6小时', value: '0 0 */6 * * ?' },
        { label: '每天上午9点', value: '0 0 9 * * ?' },
        { label: '工作日下午6点', value: '0 0 18 * * MON-FRI' },
        { label: '每周日凌晨3点', value: '0 0 3 * * SUN' }
      ],
      formData: {
        id: '',
        keyType: '',
        keyName: '',
        keyId: '',
        keySecret: '',
        remark: ''
      },
      formRules: {
        keyType: [{ required: true, message: '请选择云厂商类型', trigger: 'change' }],
        keyId: [{ required: true, message: '请输入密钥ID', trigger: 'blur' }],
        keySecret: [{ required: true, message: '请输入密钥Secret', trigger: 'blur' }]
      },
      scheduleFormData: {
        id: '',
        name: '',
        cronExpr: '',
        keyTypes: [],
        status: 1,
        remark: ''
      },
      scheduleFormRules: {
        name: [{ required: true, message: '请输入配置名称', trigger: 'blur' }],
        cronExpr: [{ required: true, message: '请输入Cron表达式', trigger: 'blur' }],
        keyTypes: [{ required: true, message: '请选择云账号', trigger: 'change' }]
      },
      cronPreviewLoading: false,
      cronPreviewError: '',
      nextExecutionTimes: [],
      cronPreviewTimer: null
    }
  },
  methods: {
    // 获取密钥列表
    async getList() {
      this.loading = true
      try {
        const { data: res } = await API.getKeyManageList({
          page: this.queryParams.pageNum,
          pageSize: this.queryParams.pageSize,
          keyType: this.queryParams.keyType || undefined
        })
        if (res.code === 200) {
          this.keyList = res.data?.list || []
          this.total = res.data?.total || 0
        } else {
          this.$message.error(res.message || '获取密钥列表失败')
        }
      } catch (error) {
        console.error('获取密钥列表失败:', error)
        this.$message.error('获取密钥列表失败')
      } finally {
        this.loading = false
      }
    },
    
    // 搜索
    async handleQuery() {
      this.queryParams.pageNum = 1
      this.getList()
    },
    
    // 重置搜索
    resetQuery() {
      this.queryParams = {
        keyType: '',
        pageNum: 1,
        pageSize: 10
      }
      this.getList()
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
      this.dialogTitle = '创建密钥'
      this.$nextTick(() => {
        this.formData = {
          id: '',
          keyType: '',
          keyName: '',
          keyId: '',
          keySecret: '',
          remark: ''
        }
        this.dialogVisible = true
      })
    },
    
    // 显示编辑对话框
    showEditDialog(row) {
      this.dialogTitle = '修改密钥'
      this.$nextTick(() => {
        this.formData = {
          id: row.id,
          keyType: row.keyType,
          keyName: row.keyName || '',
          keyId: row.keyId,
          keySecret: row.keySecret,
          remark: row.remark || ''
        }
        this.dialogVisible = true
      })
    },
    
    // 提交表单
    async submitForm() {
      if (this.submitting) return

      try {
        await this.$refs.formRef.validate()
        this.submitting = true
        
        const formData = { ...this.formData }
        
        let res
        if (formData.id) {
          // 更新
          res = await API.updateKeyManage(formData)
        } else {
          // 新增
          res = await API.createKeyManage(formData)
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
    
    // 删除密钥
    async handleDelete(row) {
      try {
        await this.$confirm(`确定删除该密钥配置?`, '提示', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        })
        
        const { data: res } = await API.deleteKeyManage(row.id)
        if (res.code === 200) {
          this.$message.success('删除成功')
          this.getList()
        } else {
          this.$message.error(res.message || '删除失败')
        }
      } catch (error) {
        console.error('删除失败:', error)
      }
    },

    // 查看解密后的密钥
    async handleDecryptKey(row) {
      this.decryptDialogVisible = true
      this.decryptLoading = true
      this.decryptedKey = null

      try {
        const { data: res } = await API.decryptKeys(row.id)
        if (res.code === 200 && res.data) {
          this.decryptedKey = {
            keyType: row.keyType,
            keyName: row.keyName,
            keyId: res.data.accessKeyId || res.data.keyId,
            keySecret: res.data.accessKeySecret || res.data.keySecret
          }
        } else {
          this.$message.error(res.message || '解密失败')
          this.decryptDialogVisible = false
        }
      } catch (error) {
        console.error('解密失败:', error)
        this.$message.error('解密失败')
        this.decryptDialogVisible = false
      } finally {
        this.decryptLoading = false
      }
    },

    // 复制到剪贴板
    copyToClipboard(text, label) {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(() => {
          this.$message.success(`${label} 已复制到剪贴板`)
        }).catch(err => {
          console.error('复制失败:', err)
          this.fallbackCopy(text, label)
        })
      } else {
        this.fallbackCopy(text, label)
      }
    },

    // 降级复制方法
    fallbackCopy(text, label) {
      const textArea = document.createElement('textarea')
      textArea.value = text
      textArea.style.position = 'fixed'
      textArea.style.left = '-999999px'
      document.body.appendChild(textArea)
      textArea.select()
      try {
        document.execCommand('copy')
        this.$message.success(`${label} 已复制到剪贴板`)
      } catch (err) {
        console.error('复制失败:', err)
        this.$message.error('复制失败，请手动复制')
      }
      document.body.removeChild(textArea)
    },

    // 同步主机
    async handleSyncHosts(row) {
      try {
        await this.$confirm(`确定同步${this.getKeyTypeTag(row.keyType).label}主机?`, '提示', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'info'
        })

        const syncData = {
          keyId: row.id,
          keyType: row.keyType
        }

        const { data: res } = await API.syncCloudHosts(syncData)

        if (res.code === 200) {
          this.$message.success('同步成功')
        } else {
          this.$message.error(res.message || '同步失败')
        }
      } catch (error) {
        console.error('同步失败:', error)
        this.$message.error('同步失败')
      }
    },
    
    // 获取云厂商标签样式
    getKeyTypeTag(keyType) {
      const typeMap = {
        1: { label: '阿里云', type: 'primary' },
        2: { label: '腾讯云', type: 'success' },
        3: { label: '百度云', type: 'info' },
        4: { label: '华为云', type: 'warning' },
        5: { label: 'AWS云', type: 'danger' },
        6: { label: '火山引擎', type: 'danger' },
        7: { label: '飞书应用', type: 'primary' }
      }
      return typeMap[keyType] || { label: '未知', type: 'info' }
    },
    
    // 获取云厂商图标
    getKeyTypeIcon(keyType) {
      const iconMap = {
        1: require('@/assets/image/aliyun.png'),      // 阿里云
        2: require('@/assets/image/tengxun.png'),     // 腾讯云
        3: require('@/assets/image/baidu.svg'),       // 百度云
        4: require('@/assets/image/huaweiyun.svg'),   // 华为云
        5: require('@/assets/image/aws.svg'),         // AWS云
        6: require('@/assets/image/火山引擎.svg'),      // 火山引擎
        7: require('@/assets/image/飞书.svg')          // 飞书应用
      }
      return iconMap[keyType] || require('@/assets/image/版本.svg')
    },
    
    // 密钥掩码显示
    maskKey(key) {
      if (!key) return ''
      if (key.length <= 8) return key
      return key.substring(0, 4) + '****' + key.substring(key.length - 4)
    },

    // 标签页切换
    handleTabChange(tabName) {
      if (tabName === 'schedule') {
        this.getScheduleList()
      }
    },

    // 获取定时同步配置列表
    async getScheduleList() {
      this.scheduleLoading = true
      try {
        const { data: res } = await API.getSyncScheduleList({
          page: this.scheduleQueryParams.pageNum,
          pageSize: this.scheduleQueryParams.pageSize
        })
        if (res.code === 200) {
          this.scheduleList = res.data?.list || []
          this.scheduleTotal = res.data?.total || 0
        } else {
          this.$message.error(res.message || '获取定时同步配置列表失败')
        }
      } catch (error) {
        console.error('获取定时同步配置列表失败:', error)
        this.$message.error('获取定时同步配置列表失败')
      } finally {
        this.scheduleLoading = false
      }
    },

    // 定时同步分页处理
    handleScheduleSizeChange(val) {
      this.scheduleQueryParams.pageSize = val
      this.scheduleQueryParams.pageNum = 1
      this.getScheduleList()
    },

    handleScheduleCurrentChange(val) {
      this.scheduleQueryParams.pageNum = val
      this.getScheduleList()
    },

    // 显示新增定时同步对话框
    showSyncScheduleDialog() {
      this.scheduleDialogTitle = '创建定时同步配置'
      // 确保有密钥列表
      if (this.keyList.length === 0) {
        this.getList()
      }
      this.$nextTick(() => {
        this.scheduleFormData = {
          id: '',
          name: '',
          cronExpr: '',
          keyTypes: [],
          status: 1,
          remark: ''
        }
        // 重置预览状态
        this.cronPreviewError = ''
        this.nextExecutionTimes = []
        this.cronPreviewLoading = false
        this.scheduleDialogVisible = true
      })
    },

    // 显示编辑定时同步对话框
    showEditSyncScheduleDialog(row) {
      this.scheduleDialogTitle = '编辑定时同步配置'
      // 确保有密钥列表
      if (this.keyList.length === 0) {
        this.getList()
      }
      this.$nextTick(() => {
        this.scheduleFormData = {
          id: row.id,
          name: row.name,
          cronExpr: row.cronExpr,
          keyTypes: this.parseKeyTypes(row.keyTypes),
          status: row.status,
          remark: row.remark || ''
        }
        // 重置预览状态
        this.cronPreviewError = ''
        this.nextExecutionTimes = []
        this.cronPreviewLoading = false
        this.scheduleDialogVisible = true

        // 如果有现有的Cron表达式，立即验证
        if (row.cronExpr) {
          this.handleCronExprChange(row.cronExpr)
        }
      })
    },

    // 选择Cron模板
    selectCronTemplate(cronExpr) {
      this.scheduleFormData.cronExpr = cronExpr
      this.handleCronExprChange(cronExpr)
    },

    // 处理Cron表达式变化
    handleCronExprChange(cronExpr) {
      // 清除之前的定时器
      if (this.cronPreviewTimer) {
        clearTimeout(this.cronPreviewTimer)
      }

      // 重置状态
      this.cronPreviewError = ''
      this.nextExecutionTimes = []

      // 如果表达式为空，不进行验证
      if (!cronExpr || cronExpr.trim() === '') {
        return
      }

      // 防抖处理，500ms 后才执行验证
      this.cronPreviewTimer = setTimeout(() => {
        this.validateCronExpression(cronExpr.trim())
      }, 500)
    },

    // 验证Cron表达式并获取执行时间
    async validateCronExpression(cronExpr) {
      this.cronPreviewLoading = true
      this.cronPreviewError = ''

      try {
        const response = await API.getNextExecutionTime(cronExpr)
        const responseData = response?.data || {}

        if (responseData.code === 200) {
          // 参考 TaskJob.vue 的处理方式
          if (responseData.data?.next_execution_time) {
            // 单个时间的处理
            this.nextExecutionTimes = [this.formatTime(responseData.data.next_execution_time)]
          } else if (Array.isArray(responseData.data)) {
            // 时间数组的处理
            this.nextExecutionTimes = responseData.data.map(time => this.formatTime(time))
          } else {
            this.nextExecutionTimes = []
          }

          if (this.nextExecutionTimes.length === 0) {
            this.cronPreviewError = 'Cron表达式有效，但无法计算执行时间'
          }
        } else {
          this.cronPreviewError = responseData.message || 'Cron表达式格式错误'
        }
      } catch (error) {
        console.error('验证Cron表达式失败:', error)
        this.cronPreviewError = 'Cron表达式格式错误，请检查语法'
      } finally {
        this.cronPreviewLoading = false
      }
    },

    // 格式化时间
    formatTime(timeStr) {
      if (!timeStr) return ''
      const date = new Date(timeStr)
      return date.toLocaleString('zh-CN', {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false
      }).replace(/\//g, '-')
    },

    // 提交定时同步表单
    async submitScheduleForm() {
      if (this.scheduleSubmitting) return

      try {
        await this.$refs.scheduleFormRef.validate()
        this.scheduleSubmitting = true

        const formData = {
          ...this.scheduleFormData,
          keyTypes: JSON.stringify(this.scheduleFormData.keyTypes)
        }

        let res
        if (formData.id) {
          res = await API.updateSyncSchedule(formData)
        } else {
          res = await API.createSyncSchedule(formData)
        }

        if (res.data.code === 200) {
          this.$message.success(formData.id ? '更新成功' : '创建成功')
          this.scheduleDialogVisible = false
          await this.getScheduleList()
        } else {
          this.$message.error(res.data.message || (formData.id ? '更新失败' : '创建失败'))
        }
      } catch (error) {
        console.error('操作失败:', error)
        this.$message.error('操作失败: ' + error.message)
      } finally {
        this.scheduleSubmitting = false
      }
    },

    // 切换定时同步状态
    async handleToggleStatus(row) {
      try {
        const { data: res } = await API.toggleSyncScheduleStatus({
          id: row.id,
          status: row.status
        })

        if (res.code === 200) {
          this.$message.success(`${row.status === 1 ? '启用' : '禁用'}成功`)
        } else {
          row.status = row.status === 1 ? 0 : 1
          this.$message.error(res.message || '状态切换失败')
        }
      } catch (error) {
        row.status = row.status === 1 ? 0 : 1
        console.error('状态切换失败:', error)
        this.$message.error('状态切换失败')
      }
    },

    // 删除定时同步配置
    async handleDeleteSchedule(row) {
      try {
        await this.$confirm(`确定删除定时同步配置"${row.name}"?`, '提示', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning'
        })

        const { data: res } = await API.deleteSyncSchedule(row.id)
        if (res.code === 200) {
          this.$message.success('删除成功')
          this.getScheduleList()
        } else {
          this.$message.error(res.message || '删除失败')
        }
      } catch (error) {
        console.error('删除失败:', error)
      }
    },

    // 立即触发同步任务
    async handleTriggerSync(row) {
      try {
        await this.$confirm(`确定立即执行同步任务"${row.name}"?`, '提示', {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'info'
        })

        const { data: res } = await API.triggerSyncSchedule(row.id)
        if (res.code === 200) {
          this.$message.success(res.message || '同步任务已触发，正在后台执行')
        } else {
          this.$message.error(res.message || '触发同步任务失败')
        }
      } catch (error) {
        if (error !== 'cancel') {
          console.error('触发同步任务失败:', error)
          this.$message.error('触发同步任务失败')
        }
      }
    },

    // 解析keyTypes字符串(实际存储的是keyId数组)
    parseKeyTypes(keyTypesStr) {
      if (!keyTypesStr) return []
      try {
        return JSON.parse(keyTypesStr)
      } catch (error) {
        console.error('解析keyTypes失败:', error)
        return []
      }
    },

    // 根据keyId获取密钥信息标签
    getKeyTagByKeyId(keyId) {
      const key = this.keyList.find(k => k.id === keyId)
      if (!key) {
        return { label: '未知账号', type: 'info', icon: require('@/assets/image/版本.svg') }
      }
      return {
        label: key.keyName || this.maskKey(key.keyId),
        type: this.getKeyTypeTag(key.keyType).type,
        icon: this.getKeyTypeIcon(key.keyType)
      }
    },

    // 显示同步日志
    async showSyncLog(row) {
      this.syncLogDialogVisible = true
      this.syncLogLoading = true
      this.currentSyncLog = null

      try {
        const { data: res } = await API.getSyncScheduleLog(row.id)
        if (res.code === 200 && res.data) {
          this.currentSyncLog = {
            id: res.data.id,
            name: res.data.name,
            syncLog: res.data.syncLog,
            lastRunTime: res.data.lastRunTime ? this.formatTime(res.data.lastRunTime) : '-',
            nextRunTime: res.data.nextRunTime ? this.formatTime(res.data.nextRunTime) : '-'
          }
        } else {
          this.$message.warning(res.message || '暂无日志数据')
        }
      } catch (error) {
        console.error('获取同步日志失败:', error)
        this.$message.error('获取同步日志失败')
      } finally {
        this.syncLogLoading = false
      }
    }
  },
  created() {
    this.getList()
  },
  beforeUnmount() {
    // 清理定时器
    if (this.cronPreviewTimer) {
      clearTimeout(this.cronPreviewTimer)
    }
  }
}
</script>

<style scoped>
.key-management {
  --aiops-bg-page: #f4f7fb;
  --aiops-bg-page-soft: #eef3f9;
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
  --aiops-brand-active: #4d79d8;
  --aiops-brand-soft: rgba(76, 139, 245, 0.12);
  --aiops-brand-fade: rgba(76, 139, 245, 0.06);
  --aiops-success: #3fb97f;
  --aiops-warning: #f3b24b;
  --aiops-danger: #ef6b73;
  --aiops-shadow-soft: 0 20px 60px rgba(17, 24, 39, 0.07);
  --aiops-shadow-panel: 0 36px 110px rgba(15, 23, 42, 0.11);
  --cloud-page-height-offset: 136px;

  position: relative;
  box-sizing: border-box;
  height: calc(100vh - var(--cloud-page-height-offset));
  max-height: calc(100vh - var(--cloud-page-height-offset));
  min-height: 0;
  padding: 6px;
  overflow: hidden;
  background: transparent;
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

:global(.home-container .el-main:has(.key-management)) {
  --ops-shell-gutter: 10px;
  --ops-page-padding: 7px;
  --ops-module-gap: 8px;
  padding-top: var(--ops-shell-gutter);
  padding-right: var(--ops-shell-gutter);
  padding-left: var(--ops-shell-gutter);
  overflow-y: hidden;
}

.key-card {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
  max-height: 100%;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 22px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.96), rgba(247, 250, 255, 0.92));
  box-shadow: var(--aiops-shadow-soft);
  backdrop-filter: blur(18px);
  overflow: hidden;
}

.key-card::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 40px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  pointer-events: none;
}

.key-card :deep(.el-card__header) {
  position: relative;
  z-index: 1;
  padding: 16px 20px 12px;
  border-bottom: none;
  background: transparent;
}

.key-card :deep(.el-card__body) {
  position: relative;
  z-index: 1;
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
  padding: 0 10px 10px;
  overflow: hidden;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 14px;
}

.title-block {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.title-kicker {
  font-size: 12px;
  font-weight: 700;
  color: var(--aiops-brand);
}

.title {
  margin: 0;
  font-size: 20px;
  line-height: 1.25;
  font-weight: 700;
  color: var(--aiops-text-primary);
}

.title-subtitle {
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.45;
}

.header-metrics {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.metric-chip {
  display: inline-flex;
  align-items: center;
  min-height: 26px;
  padding: 0 10px;
  border-radius: 999px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  background: rgba(255, 255, 255, 0.8);
  color: var(--aiops-text-secondary);
  font-size: 12px;
  font-weight: 600;
}

.search-section {
  flex-shrink: 0;
  margin-bottom: 8px;
  padding: 12px 14px 2px;
  border-radius: 16px;
  border: 1px solid var(--aiops-border-soft);
  background: rgba(255, 255, 255, 0.58);
}

.search-form {
  margin: 0;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0 12px;
}

.search-form :deep(.el-form-item) {
  margin-bottom: 10px;
  margin-right: 12px;
}

.search-form :deep(.el-form-item__label) {
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

.search-form :deep(.el-form-item__content) {
  gap: 10px;
}

.filter-select {
  width: 180px;
}

.full-select {
  width: 100%;
}

.el-button {
  min-height: 32px;
  border-radius: 14px;
  font-weight: 600;
  transition: transform 0.2s ease, border-color 0.2s ease, background-color 0.2s ease, color 0.2s ease, box-shadow 0.2s ease;
}

.el-button:hover {
  transform: translateY(-1px);
}

.search-button,
.el-button--primary {
  border-color: transparent;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
  box-shadow: 0 10px 18px rgba(76, 139, 245, 0.14);
}

.create-button,
.el-button--success {
  border-color: transparent;
  background: linear-gradient(180deg, #51c78d 0%, #3fb97f 100%);
  box-shadow: 0 10px 18px rgba(63, 185, 127, 0.14);
}

.reset-button,
.dialog-footer :deep(.el-button--warning) {
  border-color: transparent;
  background: linear-gradient(180deg, #f6be63 0%, #f3b24b 100%);
  color: #ffffff;
  box-shadow: 0 10px 18px rgba(243, 178, 75, 0.14);
}

.secondary-action,
.el-button--default {
  border-color: rgba(215, 224, 236, 0.95);
  background: rgba(255, 255, 255, 0.9);
  color: var(--aiops-text-secondary);
}

.secondary-action:hover,
.el-button--default:hover {
  border-color: rgba(120, 156, 221, 0.8);
  color: var(--aiops-brand-active);
  background: rgba(255, 255, 255, 0.98);
}

.el-input :deep(.el-input__wrapper),
.el-select :deep(.el-input__wrapper),
.el-textarea :deep(.el-textarea__inner) {
  min-height: 32px;
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 14px;
  background: rgba(252, 253, 255, 0.98);
  box-shadow: none;
  color: var(--aiops-text-primary);
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;
}

.el-input :deep(.el-input__wrapper):hover,
.el-select :deep(.el-input__wrapper):hover,
.el-textarea :deep(.el-textarea__inner):hover {
  border-color: rgba(171, 189, 220, 0.95);
}

.el-input :deep(.el-input__wrapper.is-focus),
.el-select :deep(.el-input__wrapper.is-focus),
.el-textarea :deep(.el-textarea__inner:focus) {
  border-color: rgba(120, 156, 221, 0.9);
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
  background: #ffffff;
}

.el-input :deep(.el-input__inner),
.el-select :deep(.el-input__inner) {
  background: transparent;
  border: none;
  color: var(--aiops-text-primary);
}

.el-tag {
  border-radius: 999px;
  border: 1px solid transparent;
  font-weight: 600;
  height: 22px;
  padding: 0 9px;
  font-size: 12px;
  line-height: 20px;
}

.config-tabs {
  flex-shrink: 0;
  margin-top: 0;
}

.config-tabs :deep(.el-tabs__header) {
  margin: 0 0 6px;
}

.config-tabs :deep(.el-tabs__item) {
  height: 34px;
  padding: 0 14px;
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

.config-tabs :deep(.el-tabs__item.is-active) {
  color: var(--aiops-brand);
}

.config-tabs :deep(.el-tabs__active-bar) {
  height: 2px;
  border-radius: 999px;
  background: var(--aiops-brand);
}

.config-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: rgba(223, 231, 242, 0.86);
}

.tab-content {
  padding: 0;
}

.content-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 6px;
  padding: 0 2px 3px;
}

.resource-count {
  color: var(--aiops-text-secondary);
  font-size: 13px;
  font-weight: 600;
}

.table-section {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: 16px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  background: rgba(255, 255, 255, 0.82);
}

.resource-table {
  width: 100%;
  min-height: 0;
  flex: 1;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  background: transparent;
}

.resource-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.resource-table :deep(.el-table__body-wrapper),
.resource-table :deep(.el-table__header-wrapper),
.resource-table :deep(.el-table__fixed-right),
.resource-table :deep(.el-table__fixed-right-patch),
.resource-table :deep(.el-table__cell) {
  background: transparent;
}

.resource-table :deep(th.el-table__cell) {
  height: 38px;
  padding: 6px 0;
  border-bottom: 1px solid rgba(223, 231, 242, 0.86);
  background: rgba(248, 250, 255, 0.96);
  color: var(--aiops-text-muted);
  font-size: 12px;
  font-weight: 700;
}

.resource-table :deep(td.el-table__cell) {
  height: 42px;
  padding: 6px 0;
  border-bottom: 1px solid rgba(15, 23, 42, 0.04);
}

.resource-table :deep(.cell) {
  line-height: 20px;
}

.resource-table :deep(.el-table__row) {
  background: rgba(255, 255, 255, 0.54);
}

.resource-table :deep(.el-table__row:hover) {
  background-color: rgba(76, 139, 245, 0.04);
}

.resource-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: rgba(76, 139, 245, 0.045);
}

.resource-name {
  display: flex;
  align-items: center;
  gap: 7px;
}

.cloud-icon {
  width: 18px;
  height: 18px;
  object-fit: contain;
}

.key-info {
  display: flex;
  align-items: center;
  gap: 7px;
}

.key-icon {
  color: var(--aiops-brand);
  font-size: 14px;
}

.secret-icon {
  color: var(--aiops-warning);
  font-size: 14px;
}

.key-value {
  color: var(--aiops-text-secondary);
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 13px;
}

.key-name-text {
  color: var(--aiops-text-primary);
  font-weight: 600;
}

.time-text {
  color: var(--aiops-text-muted);
  font-size: 13px;
}

.remark-text {
  color: var(--aiops-text-secondary);
  font-size: 13px;
}

.schedule-icon {
  color: var(--aiops-brand);
  font-size: 14px;
}

.schedule-name {
  color: var(--aiops-brand-active);
  cursor: pointer;
  font-weight: 600;
  transition: color 0.2s ease;
}

.cron-info {
  display: flex;
  align-items: center;
  gap: 6px;
}

.cron-icon {
  color: var(--aiops-success);
  font-size: 14px;
}

.cron-text {
  padding: 2px 8px;
  border-radius: 10px;
  border: 1px solid rgba(223, 231, 242, 0.88);
  background: rgba(247, 250, 255, 0.9);
  color: var(--aiops-warning);
  font-size: 12px;
}

.cloud-types,
.cloud-accounts {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}

.cloud-account-icon {
  width: 22px;
  height: 22px;
  padding: 3px;
  border-radius: 10px;
  border: 1px solid rgba(223, 231, 242, 0.82);
  background: rgba(255, 255, 255, 0.88);
  cursor: pointer;
  object-fit: contain;
  transition: border-color 0.2s ease, background-color 0.2s ease;
}

.cloud-account-icon:hover {
  border-color: rgba(120, 156, 221, 0.65);
  background: #ffffff;
}

.cloud-tag {
  font-size: 11px;
  max-width: 80px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.next-run-time {
  display: flex;
  align-items: center;
  gap: 6px;
}

.time-icon {
  color: var(--aiops-success);
  font-size: 14px;
}

.operation-buttons {
  display: flex;
  flex-wrap: nowrap;
  gap: 14px;
  justify-content: flex-start;
  align-items: center;
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

.operation-primary {
  color: var(--aiops-brand-active);
}

.operation-success {
  color: #2d9c72;
}

.operation-warning {
  color: #b77b24;
}

.operation-danger {
  color: #d85c66;
}

.pagination-section {
  flex-shrink: 0;
  display: flex;
  justify-content: flex-end;
  padding: 8px 2px 0;
}

.el-pagination {
  background: transparent;
}

.el-pagination :deep(.btn-next),
.el-pagination :deep(.btn-prev),
.el-pagination :deep(.el-pager li) {
  border-radius: 10px;
  margin: 0 2px;
  color: var(--aiops-text-secondary);
  transition: background-color 0.2s ease, color 0.2s ease;
}

.el-pagination :deep(.btn-next):hover,
.el-pagination :deep(.btn-prev):hover,
.el-pagination :deep(.el-pager li):hover {
  color: var(--aiops-brand);
  background: var(--aiops-brand-fade);
  box-shadow: none;
}

.el-pagination :deep(.el-pager li.is-active) {
  color: var(--aiops-brand);
  background: var(--aiops-brand-soft);
}

.cron-templates {
  max-height: 200px;
  overflow-y: auto;
}

.cron-template-item {
  padding: 10px 12px;
  margin-bottom: 8px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 12px;
  background: rgba(247, 250, 255, 0.9);
  color: var(--aiops-text-secondary);
  cursor: pointer;
  transition: border-color 0.2s ease, background-color 0.2s ease;
}

.cron-template-item:hover {
  border-color: rgba(120, 156, 221, 0.65);
  background: #ffffff;
  box-shadow: none;
}

.cron-template-item:last-child {
  margin-bottom: 0;
}

.cron-template-item strong {
  color: var(--aiops-brand);
  display: block;
  margin-bottom: 2px;
}

.el-checkbox-group .el-checkbox {
  margin-right: 20px;
  margin-bottom: 10px;
}

.el-checkbox-group .el-checkbox:deep(.el-checkbox__label) {
  font-weight: 500;
}

.el-loading-mask {
  background-color: rgba(247, 250, 255, 0.72);
  backdrop-filter: blur(4px);
}

.decrypt-container {
  min-height: 200px;
}

.security-alert {
  margin-bottom: 16px;
}

.decrypt-info {
  padding: 16px 0;
}

.key-display {
  width: 100%;
}

.key-display .el-input {
  font-family: 'Monaco', 'Menlo', 'Courier New', monospace;
}

.key-display .el-input :deep(.el-input__inner) {
  font-size: 13px;
  font-family: 'Monaco', 'Menlo', 'Courier New', monospace;
}

.key-display :deep(.el-input-group__append) {
  padding: 0;
}

.key-display :deep(.el-input-group__append .el-button) {
  margin: 0;
  border-left: 1px solid rgba(223, 231, 242, 0.92);
}

.sync-log-container {
  min-height: 200px;
}

.log-info {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.log-header {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  border-radius: 16px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  background: rgba(247, 250, 255, 0.9);
}

.log-header-item {
  display: flex;
  align-items: center;
  gap: 8px;
}

.log-label {
  font-weight: 600;
  color: var(--aiops-text-secondary);
  min-width: 100px;
}

.log-value {
  color: var(--aiops-brand-active);
  font-weight: 600;
}

.log-content {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.log-title {
  font-weight: 600;
  color: var(--aiops-text-secondary);
  font-size: 14px;
}

.log-text {
  padding: 16px;
  border-radius: 16px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  background: rgba(247, 250, 255, 0.92);
  font-family: 'Monaco', 'Menlo', 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.6;
  color: var(--aiops-text-primary);
  max-height: 400px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-wrap: break-word;
  margin: 0;
}

.log-text::-webkit-scrollbar {
  width: 8px;
}

.log-text::-webkit-scrollbar-track {
  background: rgba(238, 243, 249, 0.8);
  border-radius: 4px;
}

.log-text::-webkit-scrollbar-thumb {
  background: rgba(141, 151, 168, 0.62);
  border-radius: 4px;
}

.log-text::-webkit-scrollbar-thumb:hover {
  background: rgba(95, 107, 124, 0.72);
}

.cloud-option {
  display: flex;
  align-items: center;
  gap: 8px;
}

.cloud-option-icon {
  width: 16px;
  height: 16px;
  object-fit: contain;
}

.cron-preview {
  margin-top: 8px;
  padding: 7px 10px;
  border-radius: 12px;
  font-size: 13px;
  font-weight: 600;
}

.cron-preview-success {
  border: 1px solid rgba(63, 185, 127, 0.18);
  background: rgba(63, 185, 127, 0.08);
  color: var(--aiops-success);
}

.cron-preview-error {
  border: 1px solid rgba(239, 107, 115, 0.18);
  background: rgba(239, 107, 115, 0.08);
  color: var(--aiops-danger);
}

:deep(.el-dialog) {
  border-radius: 28px;
  border: 1px solid rgba(223, 231, 242, 0.96);
  background: linear-gradient(
    135deg,
    rgba(255, 253, 248, 0.98) 0%,
    rgba(255, 249, 241, 0.95) 42%,
    rgba(247, 250, 255, 0.94) 100%
  );
  box-shadow: var(--aiops-shadow-panel);
  overflow: hidden;
}

:deep(.el-dialog__header) {
  margin-right: 0;
  padding: 20px 24px 14px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.82);
  background: linear-gradient(90deg, #fef7ef 0%, #f8f8f6 52%, #edf2fd 100%);
}

:deep(.el-dialog__title) {
  color: var(--aiops-text-primary);
  font-size: 18px;
  font-weight: 700;
}

:deep(.el-dialog__body) {
  padding: 20px 24px 8px;
  color: var(--aiops-text-secondary);
}

:deep(.el-dialog__footer),
.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 14px 24px 22px;
}

:deep(.el-dialog__footer .dialog-footer) {
  padding: 0;
}

:deep(.el-dialog .el-form-item__label) {
  color: var(--aiops-text-secondary);
  font-weight: 600;
}

:deep(.el-alert) {
  border-radius: 16px;
  border: 1px solid rgba(243, 178, 75, 0.18);
  background: rgba(243, 178, 75, 0.09);
}

@media (max-width: 768px) {
  .key-management {
    padding: 4px;
    overflow: hidden;
  }

  .key-card {
    max-height: 100%;
    border-radius: 18px;
  }

  .key-card :deep(.el-card__header) {
    padding: 14px 16px 10px;
  }

  .key-card :deep(.el-card__body) {
    padding: 0 8px 8px;
  }

  .card-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }

  .header-metrics {
    justify-content: flex-start;
  }

  .content-header {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }

  .resource-table :deep(.el-table) {
    font-size: 12px;
  }

  .operation-buttons {
    gap: 10px;
  }

  .search-section {
    padding: 12px;
  }

  .search-form {
    flex-direction: column;
    align-items: stretch;
  }

  .search-form :deep(.el-form-item) {
    margin-bottom: 8px;
  }

  .cloud-types {
    gap: 2px;
  }

  .cloud-tag {
    max-width: 60px;
  }

  .log-header {
    padding: 12px;
  }

  .log-label {
    min-width: 80px;
    font-size: 13px;
  }

  .log-text {
    font-size: 12px;
    padding: 12px;
  }

  :deep(.el-dialog) {
    width: calc(100vw - 24px) !important;
    margin-top: 6vh !important;
  }

  .table-section {
    overflow-x: auto;
  }

  .resource-table {
    min-width: 980px;
  }

  .dialog-footer {
    align-items: stretch;
    flex-direction: column;
  }

  .dialog-footer .el-button {
    width: 100%;
    margin-left: 0;
  }
}

/* DESIGN.md compact workbench overrides */
.key-management {
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

.header-summary {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 10px;
  margin-left: auto;
  color: var(--ops-text-muted);
  font-size: 12px;
}

.header-summary strong {
  margin-left: 3px;
  color: #2563eb;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 14px;
}

.header-summary__divider {
  width: 1px;
  height: 18px;
  background: var(--ops-border-soft);
}

.key-card {
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

.key-card::before {
  display: none;
}

.key-card :deep(.el-card__body) {
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

.filter-select {
  width: 220px;
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
.create-button {
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

.reset-button {
  border-color: #e6a23c;
  background: #e6a23c;
}

.create-button {
  min-width: 104px;
  border-color: #67c23a;
  background: #67c23a;
}

.search-button:hover,
.search-button:focus,
.reset-button:hover,
.reset-button:focus,
.create-button:hover,
.create-button:focus {
  color: #fff;
  transform: none;
  box-shadow: none;
}

.config-tabs {
  flex-shrink: 0;
}

.config-tabs :deep(.el-tabs__header) {
  margin: 0;
}

.config-tabs :deep(.el-tabs__item) {
  height: 32px;
  padding: 0 12px;
  color: var(--ops-text-secondary);
  font-size: 12px;
  font-weight: 600;
}

.config-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: var(--ops-border-soft);
}

.content-header {
  display: flex;
  min-height: 38px;
  flex-shrink: 0;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin: 0 0 6px;
  padding: 3px 2px;
  background: transparent;
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

.table-section {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  border: 1px solid #e3ebf5;
  border-radius: 10px;
  background: #fff;
}

.resource-table {
  min-height: 0;
  border: 0;
  border-radius: 9px;
  background: #fff;
  color: var(--ops-text-secondary);
  font-size: 12px;
}

.resource-table :deep(th.el-table__cell),
.resource-table :deep(.el-table__fixed-right-patch) {
  height: 38px;
  padding: 6px 0;
  border-bottom: 1px solid #e3ebf5;
  background: #f3f7fc !important;
}

.resource-table :deep(th.el-table__cell .cell) {
  color: #475569 !important;
  font-size: 12px;
  font-weight: 700;
}

.resource-table :deep(td.el-table__cell) {
  height: 40px;
  padding: 6px 0;
  border-bottom-color: #eef3f9;
  background: #fff;
}

.resource-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

.resource-table :deep(.el-table__fixed-right),
.resource-table :deep(.el-table-fixed-column--right) {
  background: #fff;
}

.resource-name,
.key-info,
.cron-info,
.next-run-time,
.status-control {
  gap: 7px;
  min-width: 0;
}

.key-name-text,
.key-value,
.time-text,
.remark-text,
.schedule-name {
  overflow: hidden;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.key-value,
.time-text,
.cron-text {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 11px;
}

.resource-table :deep(.el-tag) {
  height: 22px;
  padding: 0 7px;
  border-radius: 6px;
  font-size: 11px;
  line-height: 20px;
}

.cron-text {
  padding: 2px 6px;
  border-radius: 6px;
}

.status-control {
  display: flex;
  align-items: center;
}

.status-text {
  color: var(--ops-text-muted);
  font-size: 11px;
}

.operation-buttons {
  gap: 14px;
  justify-content: flex-end;
}

.operation-buttons :deep(.el-dropdown) {
  display: inline-flex;
}

.operation-link {
  transform: none;
  transition: color 160ms ease, opacity 160ms ease;
}

.operation-more {
  color: #b7791f;
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

.key-management .el-button:hover,
.key-management .el-button:focus,
.cloud-account-icon:hover {
  transform: none;
}

:global(.cloud-account-dialog-overlay .el-dialog),
:global(.el-dialog.cloud-edit-dialog),
:global(.el-dialog.cloud-secret-dialog),
:global(.el-dialog.cloud-log-dialog),
:global(.el-dialog.cloud-schedule-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 24px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: 15px !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.cloud-account-dialog-overlay .el-dialog__header),
:global(.cloud-edit-dialog .el-dialog__header),
:global(.cloud-secret-dialog .el-dialog__header),
:global(.cloud-log-dialog .el-dialog__header),
:global(.cloud-schedule-dialog .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: linear-gradient(90deg, #fff6e7 0%, #f7fbff 36%, #ecf7ff 100%);
}

:global(.cloud-account-dialog-overlay .el-dialog__title),
:global(.cloud-edit-dialog .el-dialog__title),
:global(.cloud-secret-dialog .el-dialog__title),
:global(.cloud-log-dialog .el-dialog__title),
:global(.cloud-schedule-dialog .el-dialog__title) {
  color: #172033;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.cloud-account-dialog-overlay .el-dialog__headerbtn),
:global(.cloud-edit-dialog .el-dialog__headerbtn),
:global(.cloud-secret-dialog .el-dialog__headerbtn),
:global(.cloud-log-dialog .el-dialog__headerbtn),
:global(.cloud-schedule-dialog .el-dialog__headerbtn) {
  top: 12px;
  right: 14px;
  width: 32px;
  height: 32px;
}

:global(.cloud-account-dialog-overlay .el-dialog__body),
:global(.cloud-edit-dialog .el-dialog__body),
:global(.cloud-secret-dialog .el-dialog__body),
:global(.cloud-log-dialog .el-dialog__body),
:global(.cloud-schedule-dialog .el-dialog__body) {
  max-height: calc(100vh - 190px);
  overflow: auto;
  padding: 16px 18px;
  color: #53657d;
  background: #fff;
}

:global(.cloud-account-dialog-overlay .el-dialog__footer),
:global(.cloud-edit-dialog .el-dialog__footer),
:global(.cloud-secret-dialog .el-dialog__footer),
:global(.cloud-log-dialog .el-dialog__footer),
:global(.cloud-schedule-dialog .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.cloud-account-dialog-overlay .el-input__wrapper),
:global(.cloud-account-dialog-overlay .el-select__wrapper),
:global(.cloud-edit-dialog .el-input__wrapper),
:global(.cloud-edit-dialog .el-select__wrapper),
:global(.cloud-secret-dialog .el-input__wrapper),
:global(.cloud-log-dialog .el-input__wrapper),
:global(.cloud-schedule-dialog .el-input__wrapper),
:global(.cloud-schedule-dialog .el-select__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.cloud-account-dialog-overlay .el-input__wrapper:hover),
:global(.cloud-account-dialog-overlay .el-select__wrapper:hover),
:global(.cloud-edit-dialog .el-input__wrapper:hover),
:global(.cloud-edit-dialog .el-select__wrapper:hover),
:global(.cloud-schedule-dialog .el-input__wrapper:hover),
:global(.cloud-schedule-dialog .el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

:global(.cloud-account-dialog-overlay .el-input__wrapper.is-focus),
:global(.cloud-account-dialog-overlay .el-select__wrapper.is-focused),
:global(.cloud-edit-dialog .el-input__wrapper.is-focus),
:global(.cloud-edit-dialog .el-select__wrapper.is-focused),
:global(.cloud-schedule-dialog .el-input__wrapper.is-focus),
:global(.cloud-schedule-dialog .el-select__wrapper.is-focused) {
  box-shadow: 0 0 0 1px #409eff inset;
}

:global(.cloud-account-dialog-overlay .el-textarea__inner),
:global(.cloud-edit-dialog .el-textarea__inner),
:global(.cloud-schedule-dialog .el-textarea__inner) {
  min-height: 72px !important;
  border: 1px solid #d9e5f2;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: none;
}

:global(.cloud-account-dialog-overlay .el-form-item),
:global(.cloud-edit-dialog .el-form-item),
:global(.cloud-secret-dialog .el-form-item),
:global(.cloud-schedule-dialog .el-form-item) {
  margin-bottom: 14px;
}

:global(.cloud-account-dialog-overlay .el-form-item__label),
:global(.cloud-edit-dialog .el-form-item__label),
:global(.cloud-secret-dialog .el-form-item__label),
:global(.cloud-schedule-dialog .el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

:global(.cloud-account-dialog-overlay .el-button),
:global(.cloud-edit-dialog .el-button),
:global(.cloud-secret-dialog .el-button),
:global(.cloud-log-dialog .el-button),
:global(.cloud-schedule-dialog .el-button) {
  min-height: 34px;
  border-radius: 8px;
  font-weight: 600;
  transform: none !important;
}

:global(.cloud-account-dialog-overlay .el-button--primary),
:global(.cloud-edit-dialog .el-button--primary),
:global(.cloud-secret-dialog .el-button--primary),
:global(.cloud-log-dialog .el-button--primary),
:global(.cloud-schedule-dialog .el-button--primary) {
  border-color: #409eff;
  background: linear-gradient(135deg, #5aa8ff, #2563eb);
  color: #fff;
}

.full-select,
.key-display {
  width: 100%;
}

.security-alert {
  margin-bottom: 12px;
  border-radius: 8px;
}

.decrypt-container,
.sync-log-container {
  min-height: 160px;
}

.decrypt-info {
  padding: 8px 0 0;
}

.log-info {
  gap: 10px;
}

.log-header {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px 14px;
  padding: 10px 12px;
  border-color: #dce8f5;
  border-radius: 10px;
  background: #f8fbff;
}

.log-header-item {
  min-width: 0;
  align-items: flex-start;
  flex-direction: column;
  gap: 3px;
}

.log-label {
  min-width: 0;
  color: var(--ops-text-muted);
  font-size: 11px;
}

.log-value {
  overflow: hidden;
  max-width: 100%;
  color: var(--ops-text-primary);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.log-title {
  font-size: 13px;
}

.log-text {
  max-height: 360px;
  padding: 10px 12px;
  border-color: #dce8f5;
  border-radius: 8px;
  background: #f8fbff;
  font-size: 12px;
  line-height: 1.65;
}

.cron-preview {
  margin-top: 6px;
  padding: 6px 8px;
  border-radius: 8px;
  font-size: 12px;
}

.cron-template-item {
  margin-bottom: 6px;
  padding: 8px 10px;
  border-radius: 8px;
}

@media (max-width: 1080px) {
  .table-section {
    overflow-x: auto;
    overflow-y: hidden;
  }

  .resource-table {
    min-width: 1040px;
  }
}

@media (max-width: 768px) {
  .key-management {
    height: calc(100vh - 110px);
    padding: 6px;
    overflow: auto;
  }

  .ops-page-header {
    flex-wrap: wrap;
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

  .header-summary {
    width: 100%;
    margin-left: 54px;
  }

  .key-card {
    border-radius: 12px;
  }

  .key-card :deep(.el-card__body) {
    padding: 8px;
  }

  .search-form {
    display: grid;
    grid-template-columns: 1fr;
  }

  .search-form :deep(.el-form-item),
  .filter-select {
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

  .content-header {
    align-items: center;
    flex-direction: row;
    gap: 8px;
  }

  .log-header {
    grid-template-columns: 1fr;
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
  .key-management *,
  .key-management *::before,
  .key-management *::after {
    scroll-behavior: auto !important;
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}
</style>
