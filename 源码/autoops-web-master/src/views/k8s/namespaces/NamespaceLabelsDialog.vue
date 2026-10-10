<template>
  <el-dialog
    :model-value="visible"
    :title="`修改标签 - ${namespaceName}`"
    width="600px"
    class="ns-labels-dialog"
    modal-class="k8s-namespace-dialog-overlay"
    :close-on-click-modal="false"
    @update:model-value="handleClose"
    @open="initLabels"
  >
    <div class="labels-content">
      <div class="labels-tip">
        <el-alert type="warning" :closable="false" show-icon>
          <template #title>标签将全量替换，提交后旧标签全部覆盖</template>
        </el-alert>
      </div>

      <div class="labels-list">
        <div v-for="(item, index) in labelItems" :key="index" class="label-row">
          <el-input v-model="item.key" placeholder="key" size="small" class="label-key" />
          <span class="label-sep">=</span>
          <el-input v-model="item.value" placeholder="value" size="small" class="label-value" />
          <el-button :icon="Delete" type="danger" size="small" circle @click="removeLabel(index)" />
        </div>

        <el-button :icon="Plus" type="success" size="small" @click="addLabel">添加标签</el-button>
      </div>

      <div class="action-buttons">
        <el-button @click="handleClose">取消</el-button>
        <el-button type="primary" @click="saveLabels" :loading="saving">保存</el-button>
      </div>
    </div>
  </el-dialog>
</template>

<script setup>
import { ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Delete, Plus } from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'

const props = defineProps({
  visible: { type: Boolean, default: false },
  clusterId: { type: [String, Number], required: true },
  namespaceName: { type: String, default: '' },
  currentLabels: { type: Object, default: () => ({}) }
})
const emit = defineEmits(['update:visible', 'saved'])

const labelItems = ref([])
const saving = ref(false)

const systemPrefixes = ['kubernetes.io/', 'beta.kubernetes.io/', 'app.kubernetes.io/']
const isSystemLabel = (key) => systemPrefixes.some(p => key.startsWith(p))

const initLabels = () => {
  // 过滤系统标签，只展示用户标签
  labelItems.value = Object.entries(props.currentLabels || {})
    .filter(([key]) => !isSystemLabel(key))
    .map(([key, value]) => ({ key, value }))
}

watch(() => props.visible, (val) => { if (val) initLabels() })

const addLabel = () => { labelItems.value.push({ key: '', value: '' }) }
const removeLabel = (index) => { labelItems.value.splice(index, 1) }

const saveLabels = async () => {
  // 验证 key 不为空
  for (const item of labelItems.value) {
    if (!item.key.trim()) { ElMessage.warning('标签 key 不能为空'); return }
  }
  // 构建标签对象（用户标签 + 保留系统标签）
  const userLabels = Object.fromEntries(labelItems.value.map(i => [i.key.trim(), i.value.trim()]))
  const systemLabels = Object.fromEntries(
    Object.entries(props.currentLabels || {}).filter(([key]) => isSystemLabel(key))
  )
  const labels = { ...systemLabels, ...userLabels }

  saving.value = true
  try {
    const res = await k8sApi.updateNamespaceLabels(props.clusterId, props.namespaceName, labels)
    const data = res.data || res
    if (data.code === 200 || data.success) {
      ElMessage.success('标签更新成功')
      emit('saved')
      handleClose()
    } else {
      ElMessage.error(data.message || '更新失败')
    }
  } catch {
    ElMessage.error('更新标签失败')
  } finally {
    saving.value = false
  }
}

const handleClose = () => emit('update:visible', false)
</script>

<style scoped>
.ns-labels-dialog :deep(.el-dialog) {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 24px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
  box-shadow: 0 36px 110px rgba(15, 23, 42, 0.11);
  backdrop-filter: blur(18px);
}

.ns-labels-dialog :deep(.el-dialog__header) {
  padding: 16px 20px 14px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.84);
}

.ns-labels-dialog :deep(.el-dialog__title) {
  color: #172133;
  font-size: 17px;
  font-weight: 700;
}

.ns-labels-dialog :deep(.el-dialog__body) {
  padding: 18px 20px;
  background:
    radial-gradient(circle at top left, rgba(76, 139, 245, 0.04), transparent 22%),
    linear-gradient(180deg, #f8fafc 0%, #f5f8fc 100%);
}

.labels-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.labels-tip {
  margin-bottom: 0;
}

.labels-tip :deep(.el-alert) {
  border: 1px solid rgba(243, 178, 75, 0.28);
  border-radius: 14px;
  background: rgba(255, 248, 235, 0.86);
}

.labels-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.82);
}

.label-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1.4fr) 28px;
  align-items: center;
  gap: 8px;
}

.label-key,
.label-value {
  min-width: 0;
}

.label-sep {
  color: #8d97a8;
  font-weight: 700;
  flex-shrink: 0;
}

.action-buttons {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding-top: 14px;
  border-top: 1px solid rgba(223, 231, 242, 0.76);
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

.el-button--danger {
  border: 1px solid rgba(239, 107, 115, 0.24);
  background: rgba(255, 244, 245, 0.92);
  color: #ef6b73;
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
  .ns-labels-dialog :deep(.el-dialog) {
    width: calc(100vw - 24px) !important;
  }

  .label-row {
    grid-template-columns: 1fr 24px;
  }

  .label-row .label-key,
  .label-row .label-value {
    grid-column: 1 / 2;
  }

  .label-sep {
    display: none;
  }

  .label-row .el-button {
    grid-column: 2 / 3;
    grid-row: 1 / 3;
  }

  .action-buttons {
    flex-direction: column-reverse;
  }

  .action-buttons .el-button {
    width: 100%;
    margin-left: 0;
  }
}

.labels-content {
  gap: 12px;
}

.labels-tip :deep(.el-alert) {
  border-radius: 8px;
}

.labels-list {
  gap: 8px;
  padding: 10px;
  border-color: #e3ebf5;
  border-radius: 10px;
  background: #fbfdff;
}

.label-row {
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1.4fr) 30px;
}

.action-buttons {
  gap: 10px;
  padding-top: 12px;
  border-top-color: #e3edf8;
}

.ns-labels-dialog :deep(.el-input__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.ns-labels-dialog :deep(.el-button) {
  min-height: 34px;
  border-radius: 8px;
  transform: none;
}

.ns-labels-dialog :deep(.label-row .el-button) {
  width: 30px;
  height: 30px;
  min-height: 30px;
  padding: 0;
}
</style>
