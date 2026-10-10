<template>
  <el-dialog title="编辑主机" v-model="dialogVisible" width="50%" @close="handleClose">
    <el-form :model="form" :rules="rules" ref="formRef" label-width="100px">
      <el-row>
        <el-col :span="12">
          <el-form-item label="主机名称" prop="hostName">
            <el-input v-model="form.hostName" placeholder="请输入主机名称" />
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="所属分组" prop="groupId">
            <el-select
              v-model="form.groupId"
              placeholder="请选择分组"
              style="width: 100%"
              clearable
              filterable
              @change="handleGroupChange"
            >
              <el-option
                v-for="group in selectableGroups"
                :key="group.id"
                :label="group.displayName"
                :value="group.id"
                :class="group.isBusinessGroup ? 'business-group-option' : 'sub-group-option'"
              >
                <div class="group-option-content">
                  <span :class="group.isBusinessGroup ? 'business-group-text' : 'sub-group-text'">
                    {{ group.displayName }}
                  </span>
                  <el-tag v-if="group.isBusinessGroup" type="success" size="small" class="business-tag">
                    默认
                  </el-tag>
                </div>
              </el-option>
            </el-select>
          </el-form-item>
        </el-col>
      </el-row>
      <el-row>
        <el-col :span="12">
          <el-form-item label="系统类型" prop="hostType">
            <el-select
              v-model="form.hostType"
              placeholder="请选择类型"
              style="width: 100%"
              @change="handleHostTypeChange"
            >
              <el-option :value="1" label="Linux" />
              <el-option :value="2" label="Windows" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="12">
          <el-form-item label="登录用户" prop="sshName">
            <el-input v-model="form.sshName" placeholder="请输入登录用户" />
          </el-form-item>
        </el-col>
      </el-row>

      <el-row>
        <el-col :span="24">
          <el-form-item label="认证凭据" prop="sshKeyId">
            <div class="auth-select-panel">
              <div class="auth-select-row">
                <el-select
                  v-model="form.sshKeyIds"
                  multiple
                  filterable
                  remote
                  clearable
                  :reserve-keyword="false"
                  :loading="authSearchLoading"
                  :remote-method="handleAuthRemoteSearch"
                  :no-data-text="authSearchKeyword ? '未找到匹配凭据' : '请输入凭据名称搜索'"
                  placeholder="输入凭据名称自动搜索"
                  class="auth-credential-select"
                  @visible-change="handleAuthSelectVisibleChange"
                  @change="handleAuthSelectionChange"
                >
                  <el-option
                    v-for="item in authDisplayOptions"
                    :key="item.id"
                    :label="item.name"
                    :value="item.id"
                  >
                    <div class="auth-option-content">
                      <span class="auth-option-name">{{ item.name }}</span>
                      <span class="auth-option-meta">{{ item.username || '-' }} / Port {{ item.port || '-' }}</span>
                    </div>
                  </el-option>
                </el-select>
              </div>

              <div v-if="selectedAuthList.length" class="auth-selected-summary">
                <span>已选 {{ selectedAuthList.length }} 个</span>
                <span v-if="selectedAuth">默认：{{ selectedAuth.name }}</span>
                <el-button link type="primary" @click="clearSelectedAuth">清空</el-button>
              </div>
            </div>
          </el-form-item>
        </el-col>
      </el-row>

      <template v-if="isLinuxHost">
        <el-row>
          <el-col :span="12">
            <el-form-item label="SSH IP" prop="sshIp">
              <el-input v-model="form.sshIp" placeholder="请输入SSH IP" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="SSH端口" prop="sshPort">
              <el-input-number
                v-model="form.sshPort"
                :min="1"
                :max="65535"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>
      </template>

      <template v-else>
        <el-row>
          <el-col :span="12">
            <el-form-item label="内网IP" prop="privateIp">
              <el-input v-model="form.privateIp" placeholder="请输入内网IP" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="远程端口" prop="sshPort">
              <el-input-number
                v-model="form.sshPort"
                :min="1"
                :max="65535"
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row>
          <el-col :span="12">
            <el-form-item label="操作系统" prop="os">
              <el-input v-model="form.os" placeholder="可选" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="CPU" prop="cpu">
              <el-input v-model="form.cpu" placeholder="可选" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row>
          <el-col :span="12">
            <el-form-item label="内存" prop="memory">
              <el-input v-model="form.memory" placeholder="可选" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="磁盘" prop="disk">
              <el-input v-model="form.disk" placeholder="可选" />
            </el-form-item>
          </el-col>
        </el-row>
      </template>
      <el-row>
        <el-col :span="24">
          <el-form-item label="标签" prop="tag">
            <el-input
              v-model="form.tag"
              placeholder="格式: key=value,key=value 例如: bline=s3,env=prod"
            />
            <div style="font-size: 12px; color: #909399; margin-top: 4px;">
              多个标签用逗号分隔，格式为 key=value
            </div>
          </el-form-item>
        </el-col>
      </el-row>
      <el-row>
        <el-col :span="24">
          <el-form-item label="备注" prop="remark">
            <el-input v-model="form.remark" type="textarea" placeholder="请输入备注信息" />
          </el-form-item>
        </el-col>
      </el-row>
    </el-form>
    <template #footer>
      <span class="dialog-footer">
        <el-button type="primary" @click="submitForm">确 定</el-button>
        <el-button @click="dialogVisible = false">取 消</el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script>
import API from '@/api/config'

export default {
  name: 'EditHost',
  props: {
    visible: {
      type: Boolean,
      default: false
    },
    hostInfo: {
      type: Object,
      required: true
    },
    groupList: {
      type: Array,
      required: true
    },
    authList: {
      type: Array,
      required: true
    }
  },
  data() {
    return {
      dialogVisible: this.visible,
      formReady: false,
      authSelectOptions: [],
      authSearchLoading: false,
      authSearchKeyword: '',
      authSearchSeq: 0,
      authOptionCache: {},
      form: {
        id: '',
        hostName: '',
        groupId: '',
        remark: '',
        hostType: 1,
        sshIp: '',
        privateIp: '',
        sshName: '',
        sshPort: 22,
        sshKeyId: '',
        sshKeyIds: [],
        os: '',
        cpu: '',
        memory: '',
        disk: '',
        tag: ''
      },
      rules: {
        hostName: [{ required: true, message: '请输入主机名称', trigger: 'blur' }],
        groupId: [{ required: true, message: '请选择所属分组', trigger: 'change' }],
        hostType: [{ required: true, message: '请选择系统类型', trigger: 'change' }],
        sshName: [{ required: true, message: '请输入登录用户', trigger: 'blur' }],
        sshIp: [{ validator: this.validateSshIp, trigger: 'blur' }],
        privateIp: [{ validator: this.validatePrivateIp, trigger: 'blur' }],
        sshKeyId: [{ validator: this.validateSshKeyId, trigger: 'change' }],
        sshPort: [{ validator: this.validateSshPort, trigger: 'blur' }]
      }
    }
  },
  computed: {
    isLinuxHost() {
      return Number(this.form.hostType) !== 2
    },
    isWindowsHost() {
      return Number(this.form.hostType) === 2
    },
    selectedAuth() {
      return this.findAuthById(this.form.sshKeyId)
    },
    selectedAuthList() {
      return this.form.sshKeyIds
        .map(authId => this.findAuthById(authId))
        .filter(Boolean)
    },
    authDisplayOptions() {
      return this.dedupeAuthList([
        ...this.authSelectOptions,
        ...this.selectedAuthList
      ])
    },
    // 生成可选择的分组列表：业务组 + 所有二级分组
    selectableGroups() {
      const groups = []
      
      // 遍历分组树，提取业务组和二级分组
      const extractGroups = (groupList) => {
        groupList.forEach(group => {
          // 如果是业务组，直接添加
          if (group.name === '业务组' || group.isDefault) {
            groups.push({
              id: group.id,
              name: group.name,
              displayName: group.name,
              isBusinessGroup: true
            })
          }
          
          // 如果有子分组，遍历子分组（二级）
          if (group.children && group.children.length > 0) {
            group.children.forEach(subGroup => {
              // 只有叶子节点（没有子节点）才可选
              if (!subGroup.children || subGroup.children.length === 0) {
                groups.push({
                  id: subGroup.id,
                  name: subGroup.name,
                  displayName: subGroup.name,
                  parentName: group.name,
                  isBusinessGroup: false
                })
              }
            })
          }
        })
      }
      
      extractGroups(this.groupList)
      return groups
    }
  },
  watch: {
    visible(newVal) {
      this.dialogVisible = newVal
      if (newVal) {
        this.$nextTick(() => {
          this.initForm()
        })
      }
    },
    hostInfo: {
      deep: true,
      immediate: true,
      handler(newVal) {
        if (newVal) {
          const hostType = Number(newVal.hostType) === 2 ? 2 : 1
          this.form = {
            id: newVal.id || '',
            hostName: newVal.hostName || '',
            groupId: newVal.groupId || (this.groupList.length > 0 ? this.groupList[0].id : ''),
            remark: newVal.remark || '',
            hostType,
            sshIp: newVal.sshIp || '',
            privateIp: newVal.privateIp || (hostType === 2 ? (newVal.sshIp || '') : ''),
            sshName: newVal.sshName || (hostType === 2 ? 'Administrator' : 'root'),
            sshPort: newVal.sshPort || (hostType === 2 ? 3389 : 22),
            sshKeyId: newVal.sshKeyId || (this.authList.length > 0 ? this.authList[0].id : ''),
            sshKeyIds: this.normalizeSshKeyIds(newVal),
            os: newVal.os || '',
            cpu: newVal.cpu || '',
            memory: newVal.memory || '',
            disk: newVal.disk || '',
            tag: newVal.tag || ''
          }
          this.updateAuthOptionCache(this.normalizeAuthList(newVal.sshAuths))
          this.authSearchKeyword = ''
          this.authSelectOptions = []
        }
      }
    },
    authList: {
      immediate: true,
      handler(newVal) {
        const normalizedList = this.normalizeAuthList(newVal)
        this.updateAuthOptionCache(normalizedList)
      }
    },
    'form.sshKeyId'(newVal) {
      if (newVal === '' || newVal === null || newVal === undefined) {
        return
      }
    },
    'form.sshKeyIds': {
      deep: true,
      handler(newVal) {
        if (!Array.isArray(newVal)) {
          this.form.sshKeyIds = []
          return
        }

        const dedupedIds = Array.from(new Set(newVal.filter(id => id !== '' && id !== null && id !== undefined)))
        if (dedupedIds.length !== newVal.length) {
          this.form.sshKeyIds = dedupedIds
          return
        }

        if (!dedupedIds.length) {
          this.form.sshKeyId = ''
          return
        }

        if (!dedupedIds.some(id => String(id) === String(this.form.sshKeyId))) {
          this.form.sshKeyId = dedupedIds[0]
        }
      }
    }
  },
  mounted() {
    if (this.visible) {
      this.initForm()
    }
  },
  methods: {
    resetAuthSearch() {
      this.authSearchKeyword = ''
      this.authSearchLoading = false
      this.authSearchSeq += 1
      this.authSelectOptions = []
    },
    handleAuthSelectionChange(selectedIds) {
      const normalizedIds = Array.isArray(selectedIds) ? selectedIds : []
      if (!normalizedIds.length) {
        this.form.sshKeyId = ''
      } else if (!normalizedIds.some(id => String(id) === String(this.form.sshKeyId))) {
        this.form.sshKeyId = normalizedIds[0]
      }
      this.$nextTick(() => {
        this.$refs.formRef?.clearValidate(['sshKeyId'])
      })
    },
    clearSelectedAuth() {
      this.form.sshKeyId = ''
      this.form.sshKeyIds = []
      this.resetAuthSearch()
      this.$nextTick(() => {
        this.$refs.formRef?.clearValidate(['sshKeyId'])
      })
    },
    findAuthById(authId) {
      if (authId === '' || authId === null || authId === undefined) {
        return null
      }

      const authIdStr = String(authId)
      return this.authSelectOptions.find(item => String(item.id) === authIdStr) ||
        this.authList.find(item => String(item.id) === authIdStr) ||
        this.authOptionCache[authIdStr] ||
        null
    },
    formatAuthOptionLabel(item) {
      if (!item) {
        return ''
      }

      const meta = [item.username, item.port ? `Port ${item.port}` : ''].filter(Boolean).join(' / ')
      return meta ? `${item.name} (${meta})` : item.name
    },
    normalizeAuthList(payload) {
      if (Array.isArray(payload)) {
        return payload.filter(Boolean)
      }

      if (payload && Array.isArray(payload.list)) {
        return payload.list.filter(Boolean)
      }

      if (payload && typeof payload === 'object') {
        if (payload.id !== undefined || payload.name) {
          return [payload]
        }
      }

      return []
    },
    dedupeAuthList(list) {
      const dedupedMap = new Map()
      this.normalizeAuthList(list).forEach(item => {
        if (item.id !== undefined && item.id !== null) {
          dedupedMap.set(String(item.id), item)
        }
      })
      return Array.from(dedupedMap.values())
    },
    updateAuthOptionCache(list) {
      this.dedupeAuthList(list).forEach(item => {
        this.authOptionCache[String(item.id)] = item
      })
    },
    handleAuthSelectVisibleChange(visible) {
      if (visible) {
        return
      }

      this.authSearchKeyword = ''
      this.authSearchLoading = false
      this.authSearchSeq += 1
    },
    async handleAuthRemoteSearch(query) {
      const keyword = (query || '').trim()
      this.authSearchKeyword = keyword
      const currentSeq = ++this.authSearchSeq

      if (!keyword) {
        this.authSearchLoading = false
        this.authSelectOptions = []
        return
      }

      this.authSearchLoading = true

      try {
        const exactResponse = await API.getEcsAuthByName(keyword)
        if (currentSeq !== this.authSearchSeq) {
          return
        }

        let searchList = []
        if (exactResponse?.data?.code === 200) {
          searchList = this.normalizeAuthList(exactResponse.data.data)
        }

        const normalizedResults = this.dedupeAuthList(searchList)
        this.updateAuthOptionCache(normalizedResults)
        this.authSelectOptions = normalizedResults
      } catch (error) {
        if (currentSeq !== this.authSearchSeq) {
          return
        }
        console.error('Search credential list failed:', error)
        this.authSelectOptions = []
      } finally {
        if (currentSeq === this.authSearchSeq) {
          this.authSearchLoading = false
        }
      }
    },
    normalizeSshKeyIds(host) {
      const ids = Array.isArray(host?.sshKeyIds) ? host.sshKeyIds : []
      const defaultId = host?.sshKeyId || (this.authList.length > 0 ? this.authList[0].id : '')
      const mergedIds = defaultId ? [defaultId, ...ids] : ids
      return Array.from(new Set(mergedIds.filter(id => id !== '' && id !== null && id !== undefined)))
    },
    validateSshIp(rule, value, callback) {
      if (this.isLinuxHost && !value) {
        return callback(new Error('请输入SSH IP'))
      }
      callback()
    },
    validatePrivateIp(rule, value, callback) {
      if (this.isWindowsHost && !value) {
        return callback(new Error('请输入内网IP'))
      }
      callback()
    },
    validateSshKeyId(rule, value, callback) {
      if (!value) {
        return callback(new Error('请选择认证凭据'))
      }
      callback()
    },
    validateSshPort(rule, value, callback) {
      if (value === '' || value === null || value === undefined) {
        return callback()
      }
      const port = Number(value)
      if (isNaN(port) || port < 1 || port > 65535) {
        return callback(new Error('端口号必须在1-65535之间'))
      }
      callback()
    },
    handleHostTypeChange(value) {
      const hostType = Number(value)
      if (hostType === 2 && !this.form.privateIp && this.form.sshIp) {
        this.form.privateIp = this.form.sshIp
      }
      if (hostType === 1 && !this.form.sshIp && this.form.privateIp) {
        this.form.sshIp = this.form.privateIp
      }
      if (hostType === 2 && (!this.form.sshPort || Number(this.form.sshPort) === 22)) {
        this.form.sshPort = 3389
      }
      if (hostType === 1 && (!this.form.sshPort || Number(this.form.sshPort) === 3389)) {
        this.form.sshPort = 22
      }
      if (hostType === 2 && (!this.form.sshName || this.form.sshName === 'root')) {
        this.form.sshName = 'Administrator'
      }
      if (hostType === 1 && (!this.form.sshName || this.form.sshName === 'Administrator')) {
        this.form.sshName = 'root'
      }
      this.$nextTick(() => {
        this.$refs.formRef?.clearValidate(['sshIp', 'privateIp', 'sshKeyId', 'sshPort'])
      })
    },
    async initForm() {
      await this.$nextTick()
      this.formReady = !!this.$refs.formRef
      if (!this.formReady) {
        console.error('表单初始化失败', {
          refs: this.$refs,
          formRef: this.$refs.formRef
        })
      }
    },
    handleClose() {
      if (this.formReady) {
        this.$refs.formRef.resetFields()
      }
      this.$emit('close')
    },
    handleGroupChange(value) {
      if (value) {
        this.form.groupId = value
      } else {
        // 清空时设置默认分组
        const defaultGroup = this.selectableGroups.find(group => group.isBusinessGroup)
        if (defaultGroup) {
          this.form.groupId = defaultGroup.id
        }
      }
    },
    async submitForm() {
      try {
        // 确保表单组件已挂载
        await this.$nextTick()
        
        // 直接检查表单引用
        if (!this.$refs.formRef || !this.$refs.formRef.validate) {
          console.error('表单引用无效:', this.$refs.formRef)
          return this.$message.error('表单组件未准备好，请稍后再试')
        }

        // 直接调用validate方法
        return new Promise((resolve) => {
          this.$refs.formRef.validate((valid) => {
            if (valid) {
              const isLinuxHost = Number(this.form.hostType) !== 2
              const authIds = Array.from(new Set([this.form.sshKeyId, ...this.form.sshKeyIds].filter(Boolean)))
              const authExists = authIds.length > 0 && authIds.every(authId => this.findAuthById(authId))
              if (!authExists) {
                this.$message.error('选择的认证凭据不存在，请刷新凭据列表后重试')
                return resolve(false)
              }

              // 转换数据格式为后端期望的结构
              const requestData = {
                id: this.form.id,
                groupId: this.form.groupId,
                hostName: this.form.hostName,
                hostType: Number(this.form.hostType) || 1,
                remark: this.form.remark,
                sshName: this.form.sshName,
                sshKeyId: this.form.sshKeyId,
                sshKeyIds: authIds,
                authIds,
                tag: this.form.tag || ''
              }

              const port = Number(this.form.sshPort)
              if (!isNaN(port) && port >= 1 && port <= 65535) {
                requestData.sshPort = port
              } else if (isLinuxHost) {
                requestData.sshPort = 22
              }

              if (isLinuxHost) {
                requestData.sshIp = this.form.sshIp
              } else {
                requestData.privateIp = this.form.privateIp
                if (this.form.os) requestData.os = this.form.os
                if (this.form.cpu) requestData.cpu = this.form.cpu
                if (this.form.memory) requestData.memory = this.form.memory
                if (this.form.disk) requestData.disk = this.form.disk
              }

              this.$emit('submit', requestData)
              this.dialogVisible = false
              resolve(true)
            } else {
              this.$message.error('请填写完整的表单信息')
              resolve(false)
            }
          })
        })
      } catch (error) {
        console.error('表单提交失败:', error)
        this.$message.error(`表单提交失败: ${error.message}`)
        return false
      }
    }
  }
}
</script>

<style scoped>
/* 分组选择器样式 */
.group-option-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.business-group-option {
  background-color: #f0f9ff !important;
}

.business-group-text {
  color: #059669 !important;
  font-weight: 600 !important;
}

.sub-group-text {
  color: #374151 !important;
}

.business-tag {
  margin-left: 8px !important;
}

.sub-group-option:hover {
  background-color: #f3f4f6 !important;
}

.business-group-option:hover {
  background-color: #e0f2fe !important;
}

.auth-select-panel {
  display: flex;
  flex-direction: column;
  gap: 6px;
  width: 100%;
}

.auth-select-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.auth-credential-select {
  flex: 1;
}

.auth-credential-select :deep(.el-select__wrapper) {
  min-height: 32px;
  padding-top: 2px;
  padding-bottom: 2px;
}

.auth-credential-select :deep(.el-tag) {
  margin: 1px 4px 1px 0;
  max-width: 180px;
}

.auth-credential-select :deep(.el-tag__content) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.auth-selected-summary {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 22px;
  color: #64748b;
  font-size: 12px;
  line-height: 20px;
}

.auth-selected-summary span:first-child {
  color: #2563eb;
  font-weight: 600;
}

.auth-selected-summary :deep(.el-button) {
  height: 20px;
  padding: 0;
  font-size: 12px;
}

.auth-option-content {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  width: 100%;
}

.auth-option-name {
  overflow: hidden;
  color: #0f172a;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.auth-option-meta {
  flex-shrink: 0;
  color: #64748b;
  font-size: 12px;
}

.auth-option-content:hover .auth-option-name {
  color: #0f172a;
}
</style>
