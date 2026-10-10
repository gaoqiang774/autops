<template>
  <el-dialog
    v-model="dialogVisible"
    title="查看集群凭证"
    width="min(820px, calc(100vw - 32px))"
    top="4vh"
    class="credential-view-dialog aiops-credential-dialog"
    modal-class="k8s-credential-dialog-overlay"
    :append-to-body="true"
    @closed="handleDialogClosed"
  >
    <div class="credential-dialog">
      <div class="hero-panel">
        <div class="hero-copy">
          <div class="hero-title">{{ credential.clusterName || '未命名集群' }}</div>
          <div class="hero-subtitle">Kubeconfig credential snapshot</div>
          <div class="hero-description">
            当前凭证仅用于查看、复制与下载，请避免在非受控环境中传播 kubeconfig 内容。
          </div>
        </div>
        <div class="hero-actions">
          <el-button
            type="primary"
            size="small"
            @click="copyCredential"
            :icon="DocumentCopy"
            :loading="copyLoading"
          >
            复制凭证
          </el-button>
          <el-button
            type="warning"
            size="small"
            @click="downloadCredential"
            :icon="Download"
            :loading="downloadLoading"
          >
            下载文件
          </el-button>
        </div>
      </div>

      <div class="panel-stack">
        <section class="surface-panel">
          <div class="surface-panel-head">
            <div>
              <div class="panel-eyebrow">CREDENTIAL CONTENT</div>
              <div class="panel-title">凭证内容</div>
            </div>
            <div class="section-status-cluster">
              <el-tag size="small" type="info" effect="plain">
                字符数：{{ credential.content?.length || 0 }}
              </el-tag>
              <el-tag size="small" type="success" effect="plain">
                YAML/Config
              </el-tag>
              <el-tag size="small" type="warning" effect="plain" v-if="!credential.content?.trim()">
                内容为空
              </el-tag>
            </div>
          </div>

          <div class="code-container">
            <pre class="code-block yaml-code" v-html="highlightedCredential"></pre>
          </div>
        </section>
      </div>
    </div>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { DocumentCopy, Download } from '@element-plus/icons-vue'
import { highlight } from '@/utils/highlight'

const props = defineProps({
  visible: {
    type: Boolean,
    default: false
  },
  credential: {
    type: Object,
    default: () => ({
      clusterName: '',
      content: ''
    })
  }
})

const emit = defineEmits(['update:visible', 'closed'])

const copyLoading = ref(false)
const downloadLoading = ref(false)

// 双向绑定 visible
const dialogVisible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value)
})

// 计算高亮后的凭证内容
const highlightedCredential = computed(() => {
  if (!props.credential.content) return '<span style="color: #909399; font-style: italic;">暂无凭证内容</span>'
  return highlight(props.credential.content, 'yaml')
})

// 复制凭证内容
const copyCredential = async () => {
  if (!props.credential.content?.trim()) {
    ElMessage.warning('凭证内容为空，无法复制')
    return
  }
  
  try {
    copyLoading.value = true
    await navigator.clipboard.writeText(props.credential.content)
    ElMessage.success('凭证内容已复制到剪贴板')
  } catch (error) {
    console.error('复制失败:', error)
    // 降级处理：使用传统方式复制
    try {
      const textArea = document.createElement('textarea')
      textArea.value = props.credential.content
      textArea.style.position = 'fixed'
      textArea.style.opacity = '0'
      document.body.appendChild(textArea)
      textArea.select()
      document.execCommand('copy')
      document.body.removeChild(textArea)
      ElMessage.success('凭证内容已复制到剪贴板')
    } catch (fallbackError) {
      console.error('降级复制也失败:', fallbackError)
      ElMessage.error('复制失败，请手动复制')
    }
  } finally {
    copyLoading.value = false
  }
}

// 下载凭证文件
const downloadCredential = () => {
  if (!props.credential.content?.trim()) {
    ElMessage.warning('凭证内容为空，无法下载')
    return
  }
  
  if (!props.credential.clusterName?.trim()) {
    ElMessage.warning('集群名称为空，无法生成文件名')
    return
  }
  
  try {
    downloadLoading.value = true
    const blob = new Blob([props.credential.content], { type: 'text/plain;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `${props.credential.clusterName}-config`
    link.style.display = 'none'
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
    ElMessage.success('凭证文件下载成功')
  } catch (error) {
    console.error('下载失败:', error)
    ElMessage.error('下载失败，请重试')
  } finally {
    downloadLoading.value = false
  }
}

// 对话框关闭处理
const handleDialogClosed = () => {
  emit('closed')
}

// 监听凭证变化，验证数据完整性
watch(() => props.credential, (newCredential) => {
  if (props.visible && newCredential) {
    if (!newCredential.clusterName?.trim()) {
      console.warn('凭证对话框：集群名称为空')
    }
    if (!newCredential.content?.trim()) {
      console.warn('凭证对话框：凭证内容为空')
    }
  }
}, { deep: true, immediate: true })
</script>

<style scoped>
.credential-dialog {
  --aiops-surface-cool: rgba(247, 250, 255, 0.82);
  --aiops-surface-warm-strong: rgba(255, 248, 239, 0.96);
  --aiops-text-primary: #172133;
  --aiops-text-secondary: #5f6b7c;
  --aiops-text-muted: #8d97a8;
  --aiops-border-cool: #dfe7f2;
  --aiops-border-mid: rgba(207, 217, 230, 0.92);
  --aiops-brand: #4c8bf5;
  --aiops-shadow-panel: 0 36px 110px rgba(15, 23, 42, 0.11);
  display: flex;
  flex-direction: column;
  gap: 10px;
  color: var(--aiops-text-primary);
}

:deep(.credential-view-dialog.el-dialog) {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 30px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
  box-shadow: var(--aiops-shadow-panel);
}

:deep(.credential-view-dialog .el-dialog__header) {
  padding: 14px 24px 0;
}

:deep(.credential-view-dialog .el-dialog__title) {
  color: var(--aiops-text-primary);
  font-size: 18px;
  font-weight: 700;
  letter-spacing: 0.2px;
}

:deep(.credential-view-dialog .el-dialog__headerbtn) {
  top: 18px;
  right: 18px;
  width: 32px;
  height: 32px;
  border-radius: 12px;
  transition: background-color 0.2s ease;
}

:deep(.credential-view-dialog .el-dialog__headerbtn:hover) {
  background: rgba(255, 255, 255, 0.72);
}

.credential-view-dialog :deep(.el-dialog__headerbtn .el-dialog__close) {
  color: var(--aiops-text-secondary);
}

:deep(.credential-view-dialog .el-dialog__body) {
  padding: 10px 24px 16px;
  background:
    radial-gradient(circle at top left, rgba(76, 139, 245, 0.05), transparent 20%),
    linear-gradient(180deg, #f8fafc 0%, #f5f8fc 100%);
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
  padding-right: 12px;
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

.hero-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
  flex-shrink: 0;
}

.hero-actions :deep(.el-button) {
  min-width: 92px;
  height: 30px;
  padding: 0 10px;
  border-color: transparent;
  border-radius: 11px;
  font-size: 12px;
  font-weight: 600;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.hero-actions :deep(.el-button:hover:not(.is-disabled)) {
  transform: translateY(-1px);
}

.hero-actions :deep(.el-button--primary) {
  background: linear-gradient(180deg, #6da0f7 0%, #4c8bf5 100%);
  box-shadow: 0 6px 12px rgba(76, 139, 245, 0.15);
}

.hero-actions :deep(.el-button--warning) {
  color: #ffffff;
  background: linear-gradient(180deg, #f5bd57 0%, #f3b24b 100%);
  box-shadow: 0 6px 12px rgba(243, 178, 75, 0.15);
}

.panel-stack {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.surface-panel {
  position: relative;
  padding: 16px;
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
  margin-bottom: 12px;
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

.section-status-cluster {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
}

.code-container {
  position: relative;
  max-height: 400px;
  overflow: auto;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.78);
}

.code-block {
  margin: 0;
  padding: 20px;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 13px;
  line-height: 1.6;
  border-radius: 18px;
  overflow-x: auto;
  white-space: pre-wrap;
  word-wrap: break-word;
  min-height: 100px;
}

.yaml-code {
  background: #0d1117;
  color: #c9d1d9;
}

.code-container::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

.code-container::-webkit-scrollbar-track {
  background: rgba(241, 245, 249, 0.8);
  border-radius: 4px;
}

.code-container::-webkit-scrollbar-thumb {
  background: rgba(148, 163, 184, 0.55);
  border-radius: 4px;
}

.code-container::-webkit-scrollbar-thumb:hover {
  background: rgba(100, 116, 139, 0.62);
}

@media (max-width: 768px) {
  :deep(.credential-view-dialog.el-dialog) {
    width: 95% !important;
    margin: 5vh auto !important;
  }

  :deep(.credential-view-dialog .el-dialog__header) {
    padding-left: 16px;
    padding-right: 16px;
  }

  :deep(.credential-view-dialog .el-dialog__body) {
    padding: 10px 16px 14px;
  }
  
  .hero-panel,
  .surface-panel-head {
    flex-direction: column;
  }

  .hero-actions,
  .hero-actions :deep(.el-button) {
    width: 100%;
  }
  
  .code-container {
    max-height: 300px;
  }
  
  .code-block {
    padding: 15px;
    font-size: 12px;
  }
}

/* DESIGN.md: standard credential dialog shell */
:global(.k8s-credential-dialog-overlay .el-dialog) {
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: 15px !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.k8s-credential-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
}

:global(.k8s-credential-dialog-overlay .el-dialog__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.k8s-credential-dialog-overlay .el-dialog__body) {
  padding: 14px 18px 16px;
  background: #fbfdff;
}

.credential-dialog {
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
  padding: 10px 12px;
}

.code-container,
.code-block {
  border-radius: 8px;
}

.code-block {
  padding: 14px;
  font-size: 12px;
}

.hero-actions :deep(.el-button) {
  height: 32px;
  border-radius: 8px;
}
</style>
