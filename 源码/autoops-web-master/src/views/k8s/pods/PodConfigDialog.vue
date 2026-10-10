<template>
  <el-dialog
    :model-value="visible"
    :title="`更新Pod配置 - ${workload.name || ''}`"
    width="600px"
    class="pod-config-dialog"
    @update:model-value="handleClose"
  >
    <el-form :model="configForm" label-width="120px">
      <el-form-item label="资源限制">
        <div class="resource-config">
          <el-row :gutter="20">
            <el-col :span="12">
              <div class="resource-section">
                <h5>CPU</h5>
                <el-form-item label="Request:" size="small">
                  <el-input
                    v-model="configForm.cpuRequest"
                    placeholder="如: 100m"
                    style="width: 100%"
                  />
                </el-form-item>
                <el-form-item label="Limit:" size="small">
                  <el-input
                    v-model="configForm.cpuLimit"
                    placeholder="如: 500m"
                    style="width: 100%"
                  />
                </el-form-item>
              </div>
            </el-col>
            <el-col :span="12">
              <div class="resource-section">
                <h5>内存</h5>
                <el-form-item label="Request:" size="small">
                  <el-input
                    v-model="configForm.memoryRequest"
                    placeholder="如: 128Mi"
                    style="width: 100%"
                  />
                </el-form-item>
                <el-form-item label="Limit:" size="small">
                  <el-input
                    v-model="configForm.memoryLimit"
                    placeholder="如: 512Mi"
                    style="width: 100%"
                  />
                </el-form-item>
              </div>
            </el-col>
          </el-row>
        </div>
      </el-form-item>
      <el-form-item label="环境变量">
        <div class="env-vars-section">
          <div v-for="(envVar, index) in configForm.envVars" :key="index" class="env-var-item">
            <el-row :gutter="10">
              <el-col :span="10">
                <el-input
                  v-model="envVar.name"
                  placeholder="变量名"
                  size="small"
                />
              </el-col>
              <el-col :span="10">
                <el-input
                  v-model="envVar.value"
                  placeholder="变量值"
                  size="small"
                />
              </el-col>
              <el-col :span="4">
                <el-button
                  type="danger"
                  size="small"
                  @click="removeEnvVar(index)"
                >
                  删除
                </el-button>
              </el-col>
            </el-row>
          </div>
          <el-button type="text" size="small" @click="addEnvVar">+ 添加环境变量</el-button>
        </div>
      </el-form-item>
    </el-form>
    <template #footer>
      <span class="dialog-footer">
        <el-button @click="handleClose">取消</el-button>
        <el-button type="primary" @click="handleSubmit" :loading="submitting">更新配置</el-button>
      </span>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, watch, defineProps, defineEmits } from 'vue'

const props = defineProps({
  visible: {
    type: Boolean,
    default: false
  },
  workload: {
    type: Object,
    default: () => ({})
  }
})

const emit = defineEmits(['update:visible', 'close', 'submit'])

const submitting = ref(false)

// 配置表单数据
const configForm = reactive({
  cpuRequest: '',
  cpuLimit: '',
  memoryRequest: '',
  memoryLimit: '',
  envVars: []
})

// 监听 workload 变化，初始化表单数据
watch(() => props.workload, (newWorkload) => {
  if (newWorkload && Object.keys(newWorkload).length > 0) {
    // 初始化资源配置
    configForm.cpuRequest = newWorkload.resources?.requests?.cpu || ''
    configForm.cpuLimit = newWorkload.resources?.limits?.cpu || ''
    configForm.memoryRequest = newWorkload.resources?.requests?.memory || ''
    configForm.memoryLimit = newWorkload.resources?.limits?.memory || ''

    // 初始化环境变量
    configForm.envVars = newWorkload.envVars || []
  }
}, { immediate: true, deep: true })

const handleClose = () => {
  emit('update:visible', false)
  emit('close')
}

const handleSubmit = async () => {
  submitting.value = true
  try {
    // 构建更新数据，按照正确的API格式
    const updateData = {
      template: {
        labels: {
          app: props.workload.labels?.app || props.workload.name
        },
        containers: [{
          name: props.workload.containerName || props.workload.name || 'nginx',
          image: props.workload.image || 'nginx:latest', // 必需字段
          resources: {
            requests: {
              cpu: configForm.cpuRequest || '100m',
              memory: configForm.memoryRequest || '128Mi'
            },
            limits: {
              cpu: configForm.cpuLimit || '500m',
              memory: configForm.memoryLimit || '512Mi'
            }
          },
          env: configForm.envVars.filter(env => env.name && env.value).map(env => ({
            name: env.name,
            value: env.value
          }))
        }]
      }
    }

    // 如果有端口配置，添加到容器中
    if (props.workload.ports && props.workload.ports.length > 0) {
      updateData.template.containers[0].ports = props.workload.ports
    }

    emit('submit', updateData)
  } finally {
    submitting.value = false
  }
}

// 添加环境变量
const addEnvVar = () => {
  configForm.envVars.push({ name: '', value: '' })
}

// 删除环境变量
const removeEnvVar = (index) => {
  configForm.envVars.splice(index, 1)
}
</script>

<style scoped>
.resource-config {
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 18px;
  padding: 16px;
  background: rgba(255, 255, 255, 0.82);
}

.resource-section h5 {
  margin: 0 0 12px 0;
  color: #172133;
  font-weight: 700;
}

.env-vars-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.env-var-item {
  padding: 8px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.82);
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.pod-config-dialog :deep(.el-dialog) {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 24px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
  box-shadow: 0 36px 110px rgba(15, 23, 42, 0.11);
  backdrop-filter: blur(18px);
}

.pod-config-dialog :deep(.el-dialog__header) {
  padding: 16px 20px 14px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.84);
}

.pod-config-dialog :deep(.el-dialog__title) {
  color: #172133;
  font-size: 17px;
  font-weight: 700;
}

.pod-config-dialog :deep(.el-dialog__body) {
  padding: 18px 20px;
  background:
    radial-gradient(circle at top left, rgba(76, 139, 245, 0.04), transparent 22%),
    linear-gradient(180deg, #f8fafc 0%, #f5f8fc 100%);
}

.pod-config-dialog :deep(.el-dialog__footer) {
  padding: 10px 20px 16px;
  background: rgba(247, 250, 255, 0.72);
  border-top: 1px solid rgba(223, 231, 242, 0.76);
}

.pod-config-dialog :deep(.el-form) {
  padding: 14px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.82);
}

.pod-config-dialog :deep(.el-form-item__label) {
  color: #5f6b7c;
  font-weight: 600;
}

.pod-config-dialog :deep(.el-input__wrapper) {
  min-height: 32px;
  background: rgba(252, 253, 255, 0.98);
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 13px;
  box-shadow: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;
}

.pod-config-dialog :deep(.el-input__wrapper):hover {
  border-color: #c0c4cc;
}

.pod-config-dialog :deep(.el-input__wrapper.is-focus) {
  border-color: rgba(120, 156, 221, 0.9);
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
  background: #ffffff;
}

.pod-config-dialog :deep(.el-button) {
  border-radius: 13px;
  font-weight: 600;
}

.pod-config-dialog :deep(.el-button--primary) {
  border: none;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
  box-shadow: 0 10px 18px rgba(76, 139, 245, 0.16);
}

.pod-config-dialog :deep(.el-button--text) {
  color: #4c8bf5;
  font-weight: 700;
}

@media (max-width: 768px) {
  .pod-config-dialog :deep(.el-dialog) {
    width: calc(100vw - 24px) !important;
  }

  .pod-config-dialog :deep(.el-col) {
    max-width: 100%;
    flex: 0 0 100%;
  }

  .dialog-footer {
    flex-direction: column-reverse;
  }

  .dialog-footer .el-button {
    width: 100%;
    margin-left: 0;
  }
}
</style>
