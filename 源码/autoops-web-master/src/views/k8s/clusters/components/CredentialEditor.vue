<template>
  <div class="credential-editor-container">
    <div class="credential-editor">
      <CodeEditor
        v-model="credentialContent"
        language="yaml"
        :height="height"
        class="editor"
      />
    </div>
    <div class="credential-actions">
      <input
        ref="fileInputRef"
        type="file"
        accept="*"
        style="display: none"
        @change="handleFileChange"
      />
      <el-button 
        size="small" 
        type="success" 
        @click="triggerFileUpload"
        :disabled="disabled"
        class="upload-btn"
      >
        <el-icon><Upload /></el-icon>
        上传配置文件
      </el-button>
      <el-button 
        size="small" 
        type="warning" 
        @click="clearCredential" 
        plain
        :disabled="disabled"
      >
        <el-icon><Delete /></el-icon>
        清空
      </el-button>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Upload, Delete } from '@element-plus/icons-vue'
import CodeEditor from '@/components/CodeEditor.vue'

const props = defineProps({
  modelValue: {
    type: String,
    default: ''
  },
  height: {
    type: String,
    default: '400px'
  },
  placeholder: {
    type: String,
    default: '请输入或上传集群凭证配置'
  },
  disabled: {
    type: Boolean,
    default: false
  },
  maxFileSize: {
    type: Number,
    default: 5 * 1024 * 1024 // 5MB
  },
  acceptedFileNames: {
    type: Array,
    default: () => []
  }
})

const emit = defineEmits(['update:modelValue'])

const fileInputRef = ref(null)

// 双向绑定
const credentialContent = computed({
  get: () => props.modelValue,
  set: (value) => emit('update:modelValue', value)
})

// 触发文件选择
const triggerFileUpload = () => {
  if (props.disabled) {
    ElMessage.warning('当前状态不允许上传文件')
    return
  }
  
  if (fileInputRef.value) {
    fileInputRef.value.click()
  } else {
    ElMessage.error('文件选择器初始化失败，请刷新页面重试')
  }
}

// 处理文件选择变化
const handleFileChange = (event) => {
  const file = event.target.files[0]
  if (!file) return

  console.log('选择的文件:', file.name, '文件大小:', file.size)
  
  // 仅在显式配置限制时校验文件名
  const fileName = file.name
  if (props.acceptedFileNames.length > 0) {
    const isValidFileName = props.acceptedFileNames.some(name =>
      fileName === name || fileName.toLowerCase().endsWith(`.${name}`)
    )

    if (!isValidFileName) {
      const acceptedNames = props.acceptedFileNames.join('", "')
      ElMessage.error(`请上传名为 "${acceptedNames}" 的文件`)
      event.target.value = ''
      return
    }
  }
  
  // 检查文件大小
  if (file.size > props.maxFileSize) {
    const sizeMB = (props.maxFileSize / (1024 * 1024)).toFixed(1)
    ElMessage.error(`文件大小不能超过 ${sizeMB}MB`)
    event.target.value = ''
    return
  }
  
  const reader = new FileReader()
  reader.onload = (e) => {
    try {
      const content = e.target.result
      if (content && content.trim()) {
        credentialContent.value = content.trim()
        ElMessage.success(`凭证文件 ${fileName} 上传成功`)
      } else {
        ElMessage.warning('文件内容为空')
      }
    } catch (error) {
      console.error('读取文件失败:', error)
      ElMessage.error('读取文件失败，请检查文件格式')
    }
    event.target.value = ''
  }
  
  reader.onerror = (error) => {
    console.error('文件读取错误:', error)
    ElMessage.error('文件读取失败')
    event.target.value = ''
  }
  
  reader.readAsText(file, 'UTF-8')
}

// 清空凭证内容
const clearCredential = () => {
  if (props.disabled) {
    ElMessage.warning('当前状态不允许清空')
    return
  }
  
  if (!credentialContent.value.trim()) {
    ElMessage.info('凭证内容已经为空')
    return
  }
  
  ElMessageBox.confirm(
    '确定要清空凭证内容吗？',
    '清空确认',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    }
  )
  .then(() => {
    credentialContent.value = ''
    ElMessage.success('凭证内容已清空')
  })
  .catch(() => {
    // 用户取消，不执行任何操作
  })
}
</script>

<style scoped>
.credential-editor-container {
  --aiops-text-secondary: #5f6b7c;
  --aiops-border-mid: rgba(207, 217, 230, 0.92);
  --aiops-brand: #4c8bf5;
  --aiops-success: #3fb97f;
  --aiops-warning: #f3b24b;
  width: 100%;
}

.credential-editor {
  margin-bottom: 12px;
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.74);
  overflow: hidden;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.credential-editor:focus-within {
  border-color: rgba(120, 156, 221, 0.9);
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
}

.editor {
  border-radius: 16px;
  overflow: hidden;
}

.editor :deep(.monaco-editor-container) {
  border: none;
  border-radius: 16px;
}

.credential-actions {
  display: flex;
  gap: 10px;
  justify-content: flex-start;
  align-items: center;
  flex-wrap: wrap;
}

.credential-actions :deep(.el-button) {
  height: 32px;
  margin-left: 0;
  padding: 0 14px;
  border-radius: 13px;
  font-size: 12px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease, background-color 0.2s ease;
}

.credential-actions :deep(.el-button:hover:not(.is-disabled)) {
  transform: translateY(-1px);
  box-shadow: 0 10px 18px rgba(17, 24, 39, 0.08);
}

.credential-actions :deep(.el-button .el-icon) {
  font-size: 14px;
}

.credential-actions :deep(.upload-btn) {
  border-color: rgba(63, 185, 127, 0.28) !important;
  background: rgba(63, 185, 127, 0.1) !important;
  color: #318966 !important;
}

.credential-actions :deep(.upload-btn:hover:not(.is-disabled)) {
  border-color: rgba(63, 185, 127, 0.4) !important;
  background: rgba(63, 185, 127, 0.14) !important;
}

.credential-actions :deep(.el-button--warning.is-plain) {
  border-color: rgba(243, 178, 75, 0.3) !important;
  background: rgba(243, 178, 75, 0.1) !important;
  color: #a66b18 !important;
}

.credential-actions :deep(.el-button--warning.is-plain:hover:not(.is-disabled)) {
  border-color: rgba(243, 178, 75, 0.42) !important;
  background: rgba(243, 178, 75, 0.14) !important;
  color: #965f13 !important;
}

.credential-actions :deep(.el-button.is-disabled) {
  opacity: 0.58;
  cursor: not-allowed;
}

@media (max-width: 768px) {
  .credential-actions {
    flex-direction: column;
    align-items: stretch;
  }

  .credential-actions :deep(.el-button) {
    width: 100%;
  }
}
</style>
