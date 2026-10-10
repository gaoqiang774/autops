<template>
  <el-dialog
    v-model="dialogVisible"
    title="编辑 K8s 集群"
    width="min(760px, calc(100vw - 32px))"
    top="2vh"
    class="edit-dialog aiops-cluster-dialog"
    modal-class="k8s-cluster-form-dialog-overlay"
    :append-to-body="true"
    @closed="handleDialogClosed"
  >
    <div class="cluster-dialog">
      <div class="hero-panel">
        <div class="hero-copy">
          <div class="hero-title">{{ form.clusterName || 'K8s 集群' }}</div>
          <div class="hero-subtitle">Cluster profile maintenance</div>
          <div class="hero-description">
            维护集群名称、凭证与描述信息。版本信息由集群同步结果生成，不在此处手动修改。
          </div>
        </div>
        <div class="hero-tags">
          <el-tag type="info" effect="plain">{{ form.version || '未知版本' }}</el-tag>
          <el-tag type="primary" effect="plain">配置维护</el-tag>
        </div>
      </div>

      <div class="panel-stack">
        <section class="surface-panel">
          <div class="surface-panel-head">
            <div>
              <div class="panel-eyebrow">CLUSTER PROFILE</div>
              <div class="panel-title">基础信息与凭证</div>
            </div>
          </div>

          <el-form :model="form" label-width="112px" ref="formRef" class="cluster-form">
            <el-form-item 
              label="集群名称" 
              required
              prop="clusterName"
              :rules="[
                { required: true, message: '请输入集群名称', trigger: 'blur' },
                { min: 2, max: 50, message: '集群名称长度在 2 到 50 个字符', trigger: 'blur' }
              ]"
            >
              <el-input
                v-model="form.clusterName"
                placeholder="请输入集群名称"
                clearable
                :disabled="loading"
              />
            </el-form-item>
            
            <el-form-item label="集群版本">
              <el-input
                v-model="form.version"
                readonly
                style="width: 100%"
                placeholder="集群版本信息"
                :disabled="loading"
              >
                <template #prefix>
                  <el-icon><Setting /></el-icon>
                </template>
              </el-input>
              <div class="version-tip">
                <el-text size="small" type="info">集群版本为只读信息，无法修改</el-text>
              </div>
            </el-form-item>

            <el-form-item label="告警数据源" prop="monitorDatasourceId">
              <el-select
                v-model="form.monitorDatasourceId"
                placeholder="请选择告警中心数据源"
                clearable
                filterable
                class="full-select"
                :loading="datasourceLoading"
                :disabled="loading"
                @change="handleMonitorDatasourceChange"
              >
                <el-option
                  v-for="item in monitorDatasourceOptions"
                  :key="monitorDatasourceId(item)"
                  :label="monitorDatasourceLabel(item)"
                  :value="monitorDatasourceId(item)"
                />
              </el-select>
            </el-form-item>
            
            <el-form-item label="集群凭证" class="credential-form-item">
              <CredentialEditor
                v-model="form.credential"
                height="160px"
                placeholder="请输入或上传集群凭证配置"
                :disabled="loading"
              />
            </el-form-item>
            
            <el-form-item label="描述信息" class="description-form-item">
              <el-input
                v-model="form.description"
                type="textarea"
                :rows="2"
                placeholder="请输入集群描述"
                :disabled="loading"
                maxlength="200"
                show-word-limit
              />
            </el-form-item>
          </el-form>
        </section>
      </div>
    </div>
    
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="handleCancel" :disabled="loading">取消</el-button>
        <el-button 
          type="primary" 
          @click="handleSubmit"
          :loading="loading"
        >
          更新
        </el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Setting } from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'
import CredentialEditor from '../components/CredentialEditor.vue'
import {
  fetchMonitorDatasourceOptions,
  monitorDatasourceId,
  monitorDatasourceLabel,
  monitorDatasourceName,
  monitorDatasourceType
} from '../monitorDatasource'

const props = defineProps({
  visible: {
    type: Boolean,
    default: false
  },
  cluster: {
    type: Object,
    default: () => ({
      id: '',
      clusterName: '',
      version: '',
      credential: '',
      description: '',
      monitorDatasourceId: '',
      monitorDatasourceType: '',
      monitorDatasourceName: ''
    })
  }
})

const emit = defineEmits(['update:visible', 'success', 'closed'])

const formRef = ref(null)
const loading = ref(false)
const datasourceLoading = ref(false)
const monitorDatasourceOptions = ref([])

// 双向绑定 visible
const dialogVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value)
})

// 表单数据
const form = reactive({
  id: '',
  clusterName: '',
  version: '',
  credential: '',
  description: '',
  monitorDatasourceId: '',
  monitorDatasourceType: '',
  monitorDatasourceName: ''
})

const loadMonitorDatasourceOptions = async () => {
  if (datasourceLoading.value) return

  datasourceLoading.value = true
  try {
    monitorDatasourceOptions.value = await fetchMonitorDatasourceOptions()
    ensureCurrentDatasourceOption()
  } catch (error) {
    console.error('加载告警数据源失败:', error)
    ElMessage.error('加载告警数据源失败')
  } finally {
    datasourceLoading.value = false
  }
}

const ensureCurrentDatasourceOption = () => {
  if (!form.monitorDatasourceId || !form.monitorDatasourceName) return

  const exists = monitorDatasourceOptions.value.some(item => (
    String(monitorDatasourceId(item)) === String(form.monitorDatasourceId)
  ))

  if (!exists) {
    monitorDatasourceOptions.value = [
      {
        id: form.monitorDatasourceId,
        name: form.monitorDatasourceName,
        datasourceType: form.monitorDatasourceType,
        enabled: 1
      },
      ...monitorDatasourceOptions.value
    ]
  }
}

const handleMonitorDatasourceChange = value => {
  const selected = monitorDatasourceOptions.value.find(item => String(monitorDatasourceId(item)) === String(value))
  form.monitorDatasourceType = selected ? monitorDatasourceType(selected) : ''
  form.monitorDatasourceName = selected ? monitorDatasourceName(selected) : ''
}

// 重置表单
const resetForm = () => {
  form.id = ''
  form.clusterName = ''
  form.version = ''
  form.credential = ''
  form.description = ''
  form.monitorDatasourceId = ''
  form.monitorDatasourceType = ''
  form.monitorDatasourceName = ''
  // 清除验证状态
  if (formRef.value) {
    formRef.value.clearValidate()
  }
}

// 填充表单数据
const fillForm = (cluster) => {
  console.log('🔄 填充编辑表单数据:', {
    clusterId: cluster.id,
    clusterName: cluster.clusterName || cluster.name,
    hasCredential: !!(cluster.credential || cluster.kubeconfig),
    credentialLength: (cluster.credential || cluster.kubeconfig || '').length
  })

  form.id = cluster.id || ''
  form.clusterName = cluster.clusterName || cluster.name || ''
  form.version = cluster.version || ''
  // 尝试多种可能的凭据字段名
  form.credential = cluster.credential || cluster.kubeconfig || ''
  form.description = cluster.description || cluster.remark || ''
  form.monitorDatasourceId = cluster.monitorDatasourceId ?? ''
  form.monitorDatasourceType = cluster.monitorDatasourceType || ''
  form.monitorDatasourceName = cluster.monitorDatasourceName || ''
  ensureCurrentDatasourceOption()
}

// 提交更新
const handleSubmit = async () => {
  try {
    // 表单验证
    if (!formRef.value) {
      ElMessage.error('表单验证失败')
      return
    }
    
    const isValid = await formRef.value.validate().catch(() => false)
    if (!isValid) {
      return
    }
    
    if (!form.id) {
      ElMessage.error('集群ID不能为空')
      return
    }
    
    loading.value = true
    handleMonitorDatasourceChange(form.monitorDatasourceId)

    // 根据后端API响应结构，使用正确的字段名
    const updateData = {
      name: form.clusterName,
      credential: form.credential,  // 后端使用credential字段
      description: form.description,
      monitorDatasourceId: form.monitorDatasourceId ? Number(form.monitorDatasourceId) : 0,
      monitorDatasourceType: form.monitorDatasourceType || ''
    }

    console.log('📤 发送更新请求:', {
      clusterId: form.id,
      url: `/api/v1/k8s/cluster/${form.id}`,
      data: {
        ...updateData,
        credential: updateData.credential ? `${updateData.credential.substring(0, 100)}...` : ''
      }
    })

    const response = await k8sApi.updateCluster(form.id, updateData)

    const responseData = response.data || response
    console.log('📥 更新集群API响应:', responseData)

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success('集群信息更新成功')
      dialogVisible.value = false
      resetForm()
      emit('success', responseData)
    } else {
      ElMessage.error(responseData.message || '更新集群信息失败')
    }
  } catch (error) {
    console.error('更新集群信息失败:', error)
    
    // 根据错误类型给出更友好的提示
    if (error.response?.status === 400) {
      ElMessage.error('请求参数错误，请检查集群信息')
    } else if (error.response?.status === 404) {
      ElMessage.error('集群不存在或已被删除')
    } else if (error.response?.status === 409) {
      ElMessage.error('集群名称已存在，请使用其他名称')
    } else if (error.code === 'ERR_NETWORK') {
      ElMessage.error('网络连接失败，请检查网络状态')
    } else {
      ElMessage.error('更新集群信息失败，请重试')
    }
  } finally {
    loading.value = false
  }
}

// 取消操作
const handleCancel = () => {
  dialogVisible.value = false
}

// 对话框关闭处理
const handleDialogClosed = () => {
  resetForm()
  emit('closed')
}

// 监听集群数据变化，填充表单
watch(() => props.cluster, (newCluster) => {
  if (newCluster && props.visible) {
    fillForm(newCluster)
  }
}, { deep: true, immediate: true })

// 监听对话框打开，填充表单数据
watch(() => props.visible, (newVisible) => {
  if (newVisible && props.cluster) {
    fillForm(props.cluster)
    loadMonitorDatasourceOptions()
  } else if (!newVisible) {
    resetForm()
  }
})
</script>

<style scoped>
.cluster-dialog {
  --aiops-surface-cool: rgba(247, 250, 255, 0.82);
  --aiops-surface-strong: rgba(255, 255, 255, 0.92);
  --aiops-surface-warm-strong: rgba(255, 248, 239, 0.96);
  --aiops-text-primary: #172133;
  --aiops-text-secondary: #5f6b7c;
  --aiops-text-muted: #8d97a8;
  --aiops-border-cool: #dfe7f2;
  --aiops-border-mid: rgba(207, 217, 230, 0.92);
  --aiops-brand: #4c8bf5;
  --aiops-brand-soft: rgba(76, 139, 245, 0.12);
  --aiops-shadow-panel: 0 36px 110px rgba(15, 23, 42, 0.11);
  display: flex;
  flex-direction: column;
  gap: 8px;
  color: var(--aiops-text-primary);
}

:deep(.edit-dialog.el-dialog) {
  display: flex;
  flex-direction: column;
  max-height: calc(100vh - 24px);
  margin-bottom: 0;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 24px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
  box-shadow: var(--aiops-shadow-panel);
}

:deep(.edit-dialog .el-dialog__header) {
  flex-shrink: 0;
  padding: 12px 18px 0;
}

:deep(.edit-dialog .el-dialog__title) {
  color: var(--aiops-text-primary);
  font-size: 18px;
  font-weight: 700;
  letter-spacing: 0.2px;
}

:deep(.edit-dialog .el-dialog__headerbtn) {
  top: 18px;
  right: 18px;
  width: 32px;
  height: 32px;
  border-radius: 12px;
  transition: background-color 0.2s ease;
}

:deep(.edit-dialog .el-dialog__headerbtn:hover) {
  background: rgba(255, 255, 255, 0.72);
}

.edit-dialog :deep(.el-dialog__headerbtn .el-dialog__close) {
  color: var(--aiops-text-secondary);
}

:deep(.edit-dialog .el-dialog__body) {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 8px 18px 10px;
  background:
    radial-gradient(circle at top left, rgba(76, 139, 245, 0.05), transparent 20%),
    linear-gradient(180deg, #f8fafc 0%, #f5f8fc 100%);
}

:deep(.edit-dialog .el-dialog__footer) {
  flex-shrink: 0;
  padding: 8px 18px 12px;
  background: linear-gradient(180deg, rgba(247, 250, 255, 0.18), rgba(247, 250, 255, 0.72));
  border-top: 1px solid rgba(223, 231, 242, 0.76);
}

.hero-panel {
  position: relative;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  padding: 12px 16px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 18px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.72),
    0 18px 40px rgba(17, 24, 39, 0.05);
}

.hero-copy {
  min-width: 0;
}

.hero-title {
  margin-bottom: 4px;
  color: var(--aiops-text-primary);
  font-size: 17px;
  font-weight: 700;
}

.hero-subtitle {
  color: var(--aiops-text-muted);
  font-size: 13px;
}

.hero-description {
  margin-top: 6px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.45;
}

.hero-tags {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
  flex-shrink: 0;
}

.panel-stack {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.surface-panel {
  position: relative;
  padding: 12px 14px 0;
  border: 1px solid var(--aiops-border-cool);
  border-radius: 18px;
  background: linear-gradient(135deg, var(--aiops-surface-warm-strong), var(--aiops-surface-cool));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.78),
    0 18px 42px rgba(17, 24, 39, 0.05);
}

.surface-panel::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 34px;
  border-radius: 18px 18px 0 0;
  background: linear-gradient(90deg, rgba(255, 249, 240, 0.95), rgba(248, 248, 246, 0.86), rgba(237, 242, 253, 0.92));
  pointer-events: none;
}

.surface-panel > * {
  position: relative;
  z-index: 1;
}

.surface-panel-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 10px;
}

.panel-eyebrow {
  margin-bottom: 4px;
  color: var(--aiops-text-muted);
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.9px;
}

.panel-title {
  color: var(--aiops-text-primary);
  font-size: 16px;
  font-weight: 700;
}

.cluster-form :deep(.el-form-item) {
  margin-bottom: 12px;
}

.cluster-form :deep(.el-form-item__label) {
  color: var(--aiops-text-secondary);
  font-weight: 600;
  line-height: 36px;
}

.cluster-form :deep(.el-input__wrapper) {
  min-height: 36px;
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 14px;
  background: rgba(252, 253, 255, 0.98);
  box-shadow: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;
}

.full-select {
  width: 100%;
}

.cluster-form :deep(.el-input__wrapper:hover:not(.is-disabled)) {
  border-color: rgba(171, 189, 220, 0.95);
}

.cluster-form :deep(.el-input__wrapper.is-focus) {
  border-color: rgba(120, 156, 221, 0.9);
  background: #ffffff;
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
}

.cluster-form :deep(.el-textarea__inner) {
  min-height: 58px !important;
  border: 1px solid rgba(188, 203, 226, 0.95);
  border-radius: 14px;
  background: #ffffff;
  box-shadow: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.cluster-form :deep(.el-textarea__inner):hover:not(:disabled) {
  border-color: rgba(171, 189, 220, 0.95);
}

.cluster-form :deep(.el-textarea__inner):focus {
  border-color: rgba(120, 156, 221, 0.9);
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
}

.version-tip {
  margin-top: 4px;
  padding-left: 4px;
}

.version-tip .el-text {
  color: var(--aiops-text-muted);
}

.cluster-form :deep(.el-input.is-disabled .el-input__wrapper),
.cluster-form :deep(.el-input[readonly] .el-input__wrapper) {
  border-color: rgba(223, 231, 242, 0.9);
  background-color: rgba(244, 247, 252, 0.8);
  cursor: not-allowed;
}

.cluster-form :deep(.el-input.is-disabled .el-input__inner),
.cluster-form :deep(.el-input[readonly] .el-input__inner) {
  color: var(--aiops-text-muted);
  cursor: not-allowed;
}

.credential-form-item {
  margin-bottom: 12px !important;
}

.credential-form-item :deep(.el-form-item__label) {
  line-height: 160px;
}

.credential-form-item :deep(.credential-editor-container) {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.credential-form-item :deep(.credential-editor) {
  margin-bottom: 0;
  border-radius: 14px;
  background: #1f2933;
}

.credential-form-item :deep(.editor),
.credential-form-item :deep(.monaco-editor-container) {
  border-radius: 14px;
}

.credential-form-item :deep(.credential-actions) {
  padding-top: 0;
  gap: 8px;
}

.credential-form-item :deep(.credential-actions .el-button) {
  height: 30px;
  border-radius: 11px;
}

.description-form-item {
  margin-bottom: 12px !important;
}

.description-form-item :deep(.el-form-item__label) {
  line-height: 58px;
}

.description-form-item :deep(.el-textarea__inner) {
  min-height: 58px !important;
  border: 1px solid rgba(157, 178, 211, 0.96) !important;
  background: #ffffff !important;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.92),
    0 0 0 1px rgba(223, 231, 242, 0.46) !important;
}

.description-form-item :deep(.el-textarea__inner:hover:not(:disabled)) {
  border-color: rgba(123, 151, 196, 0.98) !important;
}

.description-form-item :deep(.el-textarea__inner:focus) {
  border-color: rgba(76, 139, 245, 0.86) !important;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.92),
    0 0 0 4px rgba(76, 139, 245, 0.08) !important;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.dialog-footer :deep(.el-button) {
  min-width: 104px;
  height: 32px;
  border-radius: 12px;
  font-weight: 600;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.dialog-footer :deep(.el-button:hover:not(.is-disabled)) {
  transform: translateY(-1px);
  box-shadow: 0 10px 18px rgba(17, 24, 39, 0.08);
}

.dialog-footer :deep(.el-button--primary) {
  border: none;
  background: linear-gradient(180deg, #6299f6 0%, #4c8bf5 100%);
  box-shadow: 0 8px 14px rgba(76, 139, 245, 0.16);
}

.dialog-footer :deep(.el-button--default) {
  border-color: rgba(214, 223, 235, 0.96);
  background: rgba(255, 255, 255, 0.92);
  color: var(--aiops-text-secondary);
}

@media (max-width: 768px) {
  :deep(.edit-dialog.el-dialog) {
    width: 95% !important;
    margin: 12px auto !important;
  }

  :deep(.edit-dialog .el-dialog__header),
  :deep(.edit-dialog .el-dialog__footer) {
    padding-left: 16px;
    padding-right: 16px;
  }

  :deep(.edit-dialog .el-dialog__body) {
    padding: 10px 16px 14px;
  }

  .hero-panel,
  .surface-panel-head {
    flex-direction: column;
  }

  .cluster-form :deep(.el-form-item__label) {
    float: none;
    display: block;
    width: auto !important;
    text-align: left;
  }

  .cluster-form :deep(.el-form-item__content) {
    margin-left: 0 !important;
  }
  
  .dialog-footer {
    flex-direction: column;
  }
  
  .dialog-footer :deep(.el-button) {
    width: 100%;
    margin: 0;
  }
}

/* DESIGN.md: standard form dialog shell */
:global(.k8s-cluster-form-dialog-overlay .el-dialog) {
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: 15px !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.k8s-cluster-form-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
}

:global(.k8s-cluster-form-dialog-overlay .el-dialog__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.k8s-cluster-form-dialog-overlay .el-dialog__body) {
  padding: 14px 18px;
  background: #fbfdff;
}

:global(.k8s-cluster-form-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #fff;
}

.cluster-dialog {
  gap: 10px;
}

.hero-panel,
.surface-panel {
  border-color: #dce8f5;
  border-radius: 10px;
  background: #f8fbff;
  box-shadow: none;
}

.hero-panel::before,
.surface-panel::before {
  display: none;
}

.hero-panel {
  padding: 10px 12px;
}

.hero-title,
.panel-title {
  font-size: 15px;
}

.hero-subtitle,
.panel-eyebrow {
  font-size: 10px;
}

.hero-description {
  margin-top: 4px;
  font-size: 12px;
  line-height: 1.5;
}

.surface-panel {
  padding: 10px 12px 0;
}

.cluster-form :deep(.el-form-item) {
  margin-bottom: 10px;
}

.cluster-form :deep(.el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

.cluster-form :deep(.el-input__wrapper),
.cluster-form :deep(.el-select__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.cluster-form :deep(.el-textarea__inner) {
  border-radius: 8px;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.dialog-footer {
  gap: 10px;
}

.dialog-footer :deep(.el-button) {
  min-width: 92px;
  height: 34px;
  border-radius: 8px;
}
</style>
