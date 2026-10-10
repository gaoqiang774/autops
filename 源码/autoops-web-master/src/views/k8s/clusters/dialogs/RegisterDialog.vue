<template>
  <el-dialog
    v-model="dialogVisible"
    title="注册 K8s 集群"
    width="min(760px, calc(100vw - 32px))"
    top="4vh"
    class="register-dialog aiops-cluster-dialog"
    modal-class="k8s-cluster-form-dialog-overlay"
    :append-to-body="true"
    @closed="handleDialogClosed"
  >
    <div class="cluster-dialog">
      <div class="hero-panel">
        <div class="hero-copy">
          <div class="hero-title">接入 Kubernetes 集群</div>
          <div class="hero-subtitle">External kubeconfig registration</div>
          <div class="hero-description">
            通过 kubeconfig 接入外部集群。注册完成后系统会同步基础资源、节点与运行状态。
          </div>
        </div>
        <div class="hero-tags">
          <el-tag type="primary" effect="plain">导入集群</el-tag>
          <el-tag type="info" effect="plain">自动同步</el-tag>
        </div>
      </div>

      <div class="panel-stack">
        <section class="surface-panel">
          <div class="surface-panel-head">
            <div>
              <div class="panel-eyebrow">CLUSTER PROFILE</div>
              <div class="panel-title">基础信息与凭证</div>
            </div>
            <div class="surface-panel-tag">必填</div>
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

            <el-form-item
              label="集群凭证"
              required
              prop="credential"
              :rules="[
                { required: true, message: '请输入集群凭证', trigger: 'blur' }
              ]"
            >
              <CredentialEditor
                v-model="form.credential"
                height="220px"
                placeholder="请输入或上传集群凭证配置"
                :disabled="loading"
              />
            </el-form-item>

            <el-form-item label="描述信息">
              <el-input
                v-model="form.description"
                type="textarea"
                :rows="3"
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
          注册
        </el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import k8sApi from '@/api/k8s'
import CredentialEditor from '../components/CredentialEditor.vue'
import {
  fetchMonitorDatasourceOptions,
  monitorDatasourceId,
  monitorDatasourceLabel,
  monitorDatasourceType
} from '../monitorDatasource'

const props = defineProps({
  visible: {
    type: Boolean,
    default: false
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
  clusterName: '',
  credential: '',
  description: '',
  monitorDatasourceId: '',
  monitorDatasourceType: ''
})

const loadMonitorDatasourceOptions = async () => {
  if (datasourceLoading.value) return

  datasourceLoading.value = true
  try {
    monitorDatasourceOptions.value = await fetchMonitorDatasourceOptions()
  } catch (error) {
    console.error('加载告警数据源失败:', error)
    ElMessage.error('加载告警数据源失败')
  } finally {
    datasourceLoading.value = false
  }
}

const handleMonitorDatasourceChange = value => {
  const selected = monitorDatasourceOptions.value.find(item => String(monitorDatasourceId(item)) === String(value))
  form.monitorDatasourceType = selected ? monitorDatasourceType(selected) : ''
}

// 重置表单
const resetForm = () => {
  form.clusterName = ''
  form.credential = ''
  form.description = ''
  form.monitorDatasourceId = ''
  form.monitorDatasourceType = ''
  // 清除验证状态
  if (formRef.value) {
    formRef.value.clearValidate()
  }
}

// 提交注册
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
    
    loading.value = true

    handleMonitorDatasourceChange(form.monitorDatasourceId)
    
    const response = await k8sApi.registerExternalCluster({
      name: form.clusterName,
      description: form.description,
      clusterType: 2,  // 导入集群
      kubeconfig: form.credential,
      monitorDatasourceId: form.monitorDatasourceId ? Number(form.monitorDatasourceId) : 0,
      monitorDatasourceType: form.monitorDatasourceType || ''
    })
    
    const responseData = response.data || response
    console.log('注册集群API响应:', responseData)

    if (responseData.code === 200 || responseData.success) {
      ElMessage.success('集群注册成功，正在同步集群信息...')
      dialogVisible.value = false
      resetForm()
      emit('success', responseData)
    } else {
      ElMessage.error(responseData.message || '注册集群失败')
    }
  } catch (error) {
    console.error('注册集群失败:', error)
    
    // 根据错误类型给出更友好的提示
    if (error.response?.status === 400) {
      ElMessage.error('请求参数错误，请检查集群信息')
    } else if (error.response?.status === 409) {
      ElMessage.error('集群名称已存在，请使用其他名称')
    } else if (error.code === 'ERR_NETWORK') {
      ElMessage.error('网络连接失败，请检查网络状态')
    } else {
      ElMessage.error('注册集群失败，请重试')
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

// 监听对话框打开，重置表单
watch(() => props.visible, (newVisible) => {
  if (newVisible) {
    resetForm()
    loadMonitorDatasourceOptions()
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
  gap: 10px;
  color: var(--aiops-text-primary);
}

:deep(.register-dialog.el-dialog) {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 30px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
  box-shadow: var(--aiops-shadow-panel);
}

:deep(.register-dialog .el-dialog__header) {
  padding: 14px 24px 0;
}

:deep(.register-dialog .el-dialog__title) {
  color: var(--aiops-text-primary);
  font-size: 18px;
  font-weight: 700;
  letter-spacing: 0.2px;
}

:deep(.register-dialog .el-dialog__headerbtn) {
  top: 18px;
  right: 18px;
  width: 32px;
  height: 32px;
  border-radius: 12px;
  transition: background-color 0.2s ease;
}

:deep(.register-dialog .el-dialog__headerbtn:hover) {
  background: rgba(255, 255, 255, 0.72);
}

.register-dialog :deep(.el-dialog__headerbtn .el-dialog__close) {
  color: var(--aiops-text-secondary);
}

:deep(.register-dialog .el-dialog__body) {
  padding: 10px 24px 14px;
  background:
    radial-gradient(circle at top left, rgba(76, 139, 245, 0.05), transparent 20%),
    linear-gradient(180deg, #f8fafc 0%, #f5f8fc 100%);
}

:deep(.register-dialog .el-dialog__footer) {
  padding: 0 24px 16px;
}

.hero-panel {
  position: relative;
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  padding: 18px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 22px;
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
  font-size: 18px;
  font-weight: 700;
}

.hero-subtitle {
  color: var(--aiops-text-muted);
  font-size: 13px;
}

.hero-description {
  margin-top: 8px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.6;
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
  gap: 10px;
}

.surface-panel {
  position: relative;
  padding: 16px 16px 0;
  border: 1px solid var(--aiops-border-cool);
  border-radius: 22px;
  background: linear-gradient(135deg, var(--aiops-surface-warm-strong), var(--aiops-surface-cool));
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.78),
    0 18px 42px rgba(17, 24, 39, 0.05);
}

.surface-panel::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 42px;
  border-radius: 22px 22px 0 0;
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
  margin-bottom: 14px;
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
  font-size: 17px;
  font-weight: 700;
}

.surface-panel-tag {
  display: inline-flex;
  align-items: center;
  padding: 6px 10px;
  border: 1px solid rgba(207, 217, 230, 0.92);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.74);
  color: var(--aiops-text-secondary);
  font-size: 11px;
  font-weight: 600;
}

.cluster-form :deep(.el-form-item) {
  margin-bottom: 20px;
}

.cluster-form :deep(.el-form-item__label) {
  color: var(--aiops-text-secondary);
  font-weight: 600;
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

.cluster-form :deep(.el-input__wrapper:hover) {
  border-color: rgba(171, 189, 220, 0.95);
}

.cluster-form :deep(.el-input__wrapper.is-focus) {
  border-color: rgba(120, 156, 221, 0.9);
  background: #ffffff;
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
}

.cluster-form :deep(.el-textarea__inner) {
  border-color: rgba(214, 223, 235, 0.95);
  border-radius: 14px;
  background: rgba(252, 253, 255, 0.98);
  box-shadow: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.cluster-form :deep(.el-textarea__inner):hover {
  border-color: rgba(171, 189, 220, 0.95);
}

.cluster-form :deep(.el-textarea__inner):focus {
  border-color: rgba(120, 156, 221, 0.9);
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
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
  border-radius: 13px;
  font-weight: 600;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.dialog-footer :deep(.el-button:hover:not(.is-disabled)) {
  transform: translateY(-1px);
  box-shadow: 0 10px 18px rgba(17, 24, 39, 0.08);
}

.dialog-footer :deep(.el-button--primary) {
  border: none;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
  box-shadow: 0 10px 18px rgba(76, 139, 245, 0.18);
}

.dialog-footer :deep(.el-button--default) {
  border-color: rgba(214, 223, 235, 0.96);
  background: rgba(255, 255, 255, 0.92);
  color: var(--aiops-text-secondary);
}

@media (max-width: 768px) {
  :deep(.register-dialog.el-dialog) {
    width: 95% !important;
    margin: 5vh auto !important;
  }

  :deep(.register-dialog .el-dialog__header),
  :deep(.register-dialog .el-dialog__footer) {
    padding-left: 16px;
    padding-right: 16px;
  }

  :deep(.register-dialog .el-dialog__body) {
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
