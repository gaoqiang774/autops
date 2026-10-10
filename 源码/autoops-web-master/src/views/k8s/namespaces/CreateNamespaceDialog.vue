<template>
  <!-- 创建命名空间对话框 -->
  <el-dialog
    :model-value="visible"
    title="创建命名空间"
    width="500px"
    class="create-namespace-dialog"
    modal-class="k8s-namespace-dialog-overlay"
    @update:model-value="handleClose"
    @close="handleClose"
  >
    <el-form :model="createForm" label-width="100px" ref="formRef" class="namespace-dialog-form">
      <el-form-item 
        label="名称" 
        required
        :rules="[
          { required: true, message: '请输入命名空间名称', trigger: 'blur' },
          { pattern: /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/, message: '命名空间名称只能包含小写字母、数字和连字符，且必须以字母数字字符开头和结尾', trigger: 'blur' }
        ]"
      >
        <el-input
          v-model="createForm.name"
          placeholder="请输入命名空间名称"
        />
      </el-form-item>
      
      <el-form-item label="描述">
        <el-input
          v-model="createForm.description"
          placeholder="请输入描述信息"
          type="textarea"
          :rows="2"
        />
      </el-form-item>

      <el-form-item label="标签">
        <el-input
          v-model="createForm.customLabels"
          placeholder="格式：key=value，每行一个"
          type="textarea"
          :rows="3"
        />
        <div class="form-hint">
        </div>
      </el-form-item>
    </el-form>
    
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="handleCancel">取消</el-button>
        <el-button type="primary" @click="handleSubmit" :loading="submitting">创建</el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, watch } from 'vue'
import { ElMessage } from 'element-plus'

// Props
const props = defineProps({
  visible: {
    type: Boolean,
    default: false
  },
  selectedClusterId: {
    type: [String, Number],
    required: true
  }
})

// Emits
const emit = defineEmits(['update:visible', 'success'])

// Form ref
const formRef = ref(null)

// 提交状态
const submitting = ref(false)

// 创建命名空间表单
const createForm = reactive({
  name: '',
  description: '',
  customLabels: ''
})

// 监听 visible 变化，重置表单
watch(() => props.visible, (newVal) => {
  if (newVal) {
    resetForm()
  }
})

// 重置表单
const resetForm = () => {
  createForm.name = ''
  createForm.description = ''
  createForm.customLabels = ''
  if (formRef.value) {
    formRef.value.clearValidate()
  }
}

// 验证标签键值格式
const validateLabel = (key, value) => {
  // 标签键验证：最多253字符，格式为[前缀/]名称
  const keyRegex = /^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\/)?[a-zA-Z0-9]([-_.a-zA-Z0-9]*[a-zA-Z0-9])?$/
  if (key.length > 253 || !keyRegex.test(key)) {
    return `标签键 "${key}" 格式不正确。最多253字符，格式：[前缀/]名称`
  }
  
  // 标签值验证：最多63字符，只能包含字母数字、连字符、下划线、点号
  const valueRegex = /^[a-zA-Z0-9]([-_.a-zA-Z0-9]*[a-zA-Z0-9])?$/
  if (value && (value.length > 63 || !valueRegex.test(value))) {
    return `标签值 "${value}" 格式不正确。最多63字符，只能包含字母数字、连字符、下划线、点号`
  }
  
  return null
}

// 解析键值对字符串
const parseKeyValuePairs = (str) => {
  const result = {}
  if (!str.trim()) return result
  
  const pairs = str.split('\n').filter(line => line.trim())
  for (const pair of pairs) {
    const [key, ...valueParts] = pair.split('=')
    if (key && key.trim()) {
      result[key.trim()] = valueParts.join('=').trim() || ''
    }
  }
  return result
}

// 处理取消
const handleCancel = () => {
  emit('update:visible', false)
}

// 处理关闭
const handleClose = () => {
  emit('update:visible', false)
}

// 处理提交
const handleSubmit = async () => {
  try {
    // 表单验证
    if (!formRef.value) return
    const valid = await formRef.value.validate()
    if (!valid) return

    if (!createForm.name) {
      ElMessage.warning('请输入命名空间名称')
      return
    }

    submitting.value = true
    
    const data = {
      name: createForm.name,
      labels: {
        'app.kubernetes.io/managed-by': 'k8s-platform'
      },
      annotations: {}
    }
    
    // 解析自定义标签
    if (createForm.customLabels) {
      const customLabels = parseKeyValuePairs(createForm.customLabels)
      for (const [key, value] of Object.entries(customLabels)) {
        const error = validateLabel(key, value)
        if (error) {
          ElMessage.warning(error)
          return
        }
        data.labels[key] = value
      }
    }
    
    // 描述作为注释
    if (createForm.description) {
      data.annotations['description'] = createForm.description
    }
    
    // 添加创建者注释
    data.annotations['kubernetes.io/created-by'] = 'k8s-platform'
    data.annotations['created-at'] = new Date().toISOString()
    
    // 发射成功事件，由父组件处理API调用
    emit('success', data)
    
  } catch (error) {
    console.error('表单验证失败:', error)
  } finally {
    submitting.value = false
  }
}

// 暴露方法供父组件调用
defineExpose({
  resetForm,
  setSubmitting: (loading) => {
    submitting.value = loading
  }
})
</script>

<style scoped>
.create-namespace-dialog :deep(.el-dialog) {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 24px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
  box-shadow: 0 36px 110px rgba(15, 23, 42, 0.11);
  backdrop-filter: blur(18px);
}

.create-namespace-dialog :deep(.el-dialog__header) {
  padding: 16px 20px 14px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.84);
}

.create-namespace-dialog :deep(.el-dialog__title) {
  color: #172133;
  font-size: 17px;
  font-weight: 700;
}

.create-namespace-dialog :deep(.el-dialog__body) {
  padding: 18px 20px;
  background:
    radial-gradient(circle at top left, rgba(76, 139, 245, 0.04), transparent 22%),
    linear-gradient(180deg, #f8fafc 0%, #f5f8fc 100%);
}

.create-namespace-dialog :deep(.el-dialog__footer) {
  padding: 10px 20px 16px;
  background: rgba(247, 250, 255, 0.72);
  border-top: 1px solid rgba(223, 231, 242, 0.76);
}

.create-namespace-dialog :deep(.el-form) {
  padding: 14px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.82);
}

.create-namespace-dialog :deep(.el-form-item__label) {
  color: #5f6b7c;
  font-weight: 600;
}

.dialog-footer {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
}

.form-hint {
  display: none;
}

.create-namespace-dialog :deep(.el-divider__text) {
  color: #172133;
  font-weight: 700;
}

.create-namespace-dialog :deep(.el-divider) {
  margin: 18px 0;
  border-top-color: rgba(223, 231, 242, 0.92);
}

.create-namespace-dialog :deep(.el-textarea__inner) {
  min-height: 74px;
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 13px;
  background: rgba(252, 253, 255, 0.98);
  box-shadow: none;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 13px;
  line-height: 1.5;
  transition: border-color 0.2s ease, box-shadow 0.2s ease;
}

.create-namespace-dialog :deep(.el-textarea__inner):focus {
  border-color: rgba(120, 156, 221, 0.9);
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
}

.el-button {
  border-radius: 13px;
  font-weight: 600;
  transition: border-color 0.2s ease, opacity 0.2s ease;
}

.el-button:hover {
  transform: none;
  box-shadow: none;
  opacity: 0.9;
}

.el-button--primary {
  border: none;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
  box-shadow: 0 10px 18px rgba(76, 139, 245, 0.16);
}

.el-button--default {
  border: 1px solid rgba(214, 223, 235, 0.96);
  background: rgba(255, 255, 255, 0.92);
  color: #5f6b7c;
}

.el-input :deep(.el-input__wrapper) {
  min-height: 32px;
  background: rgba(252, 253, 255, 0.98);
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 13px;
  box-shadow: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;
}

.el-input :deep(.el-input__wrapper):hover {
  border-color: #c0c4cc;
}

.el-input :deep(.el-input__wrapper.is-focus) {
  border-color: rgba(120, 156, 221, 0.9);
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
  background: #ffffff;
}

@media (max-width: 768px) {
  .create-namespace-dialog :deep(.el-dialog) {
    width: calc(100vw - 24px) !important;
  }

  .dialog-footer {
    flex-direction: column-reverse;
  }

  .dialog-footer .el-button {
    width: 100%;
    margin-left: 0;
  }
}

.create-namespace-dialog :deep(.namespace-dialog-form) {
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
}

.create-namespace-dialog :deep(.el-form-item) {
  margin-bottom: 14px;
}

.create-namespace-dialog :deep(.el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

.create-namespace-dialog :deep(.el-input__wrapper),
.create-namespace-dialog :deep(.el-textarea__inner) {
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.create-namespace-dialog :deep(.el-input__wrapper) {
  min-height: 34px;
}

.create-namespace-dialog :deep(.el-button) {
  min-height: 34px;
  border-radius: 8px;
  transform: none;
}
</style>
