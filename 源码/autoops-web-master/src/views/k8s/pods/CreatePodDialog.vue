<template>
  <el-dialog
    :model-value="visible"
    :title="createMode === 'form' ? '新增 Deployment 参数填写' : '创建工作负载'"
    width="960px"
    class="create-pod-dialog"
    modal-class="k8s-workload-dialog-overlay"
    :close-on-click-modal="false"
    @update:model-value="handleClose"
  >
    <div class="create-pod-content">
      <div class="dialog-toolbar">
        <el-radio-group v-model="createMode" class="mode-switch">
          <el-radio-button label="yaml">YAML 创建</el-radio-button>
          <el-radio-button label="form">表格创建</el-radio-button>
        </el-radio-group>

        <div v-if="createMode === 'yaml'" class="yaml-actions">
          <el-tooltip content="YAML 创建校验" placement="top">
            <el-button
              :icon="Search"
              type="primary"
              size="small"
              circle
              :loading="loading"
              class="validate-btn"
              @click="previewPodCreation"
            />
          </el-tooltip>
          <el-tooltip content="复制 YAML" placement="top">
            <el-button
              :icon="DocumentCopy"
              type="warning"
              size="small"
              circle
              class="copy-btn"
              @click="copyToClipboard"
            />
          </el-tooltip>
        </div>
      </div>

      <template v-if="createMode === 'yaml'">
        <div class="target-info">
          <div class="target-item">
            <span class="target-label">目标集群</span>
            <el-tag type="info" size="small">{{ clusterName || '未选择' }}</el-tag>
          </div>
          <div class="target-item">
            <span class="target-label">命名空间</span>
            <el-tag type="warning" size="small">{{ namespace || '未选择' }}</el-tag>
          </div>
          <el-tag
            v-if="dryRunResult"
            :type="dryRunResult.success ? 'success' : 'danger'"
            size="small"
          >
            {{ dryRunResult.success ? '校验通过' : '校验失败' }}
          </el-tag>
        </div>

        <div class="yaml-editor-container">
          <CodeEditor
            ref="yamlEditor"
            v-model="yamlContent"
            language="yaml"
            height="460px"
            fontSize="14px"
            :readonly="false"
            placeholder="请输入工作负载的 YAML 配置..."
          />
        </div>

        <div v-if="dryRunResult" class="dryrun-result">
          <el-alert
            :title="dryRunResult.message"
            :type="dryRunResult.success ? 'success' : 'error'"
            :description="dryRunResult.details"
            show-icon
            :closable="false"
          />
        </div>
      </template>

      <el-form
        v-else
        ref="formRef"
        :model="deploymentForm"
        label-width="0"
        class="simple-deployment-form"
      >
        <div class="form-grid two-column">
          <div class="field-block">
            <div class="field-label required">命名空间</div>
            <div class="field-content">
              <el-select v-model="deploymentForm.namespace" disabled>
                <el-option
                  :label="deploymentForm.namespace || 'default'"
                  :value="deploymentForm.namespace || 'default'"
                />
              </el-select>
              <p class="field-help">使用搜索栏当前命名空间</p>
            </div>
          </div>

          <div class="field-block">
            <div class="field-label required">名称</div>
            <div class="field-content">
              <el-input v-model.trim="deploymentForm.name" placeholder="请输入 Deployment 名称" clearable />
              <p class="field-help">允许数字、字母和“-”组成，不能以“-”和数字开头，不能以“-”结尾</p>
            </div>
          </div>
        </div>

        <div class="field-block full-row">
          <div class="field-label required">标签设置</div>
          <div class="field-content">
            <div
              v-for="(label, labelIndex) in deploymentForm.labels"
              :key="label.id"
              class="pair-row"
            >
              <el-input v-model.trim="label.key" placeholder="Key" />
              <el-input v-model.trim="label.value" placeholder="Value" />
              <el-button class="remove-row-button" text @click="removeItem(deploymentForm.labels, labelIndex)">
                ×
              </el-button>
            </div>
            <el-button type="success" class="add-line-button" @click="addKeyValue(deploymentForm.labels)">
              + 新增
            </el-button>
            <p class="field-help">建议填写 version: v1，如不需要可以删除</p>
          </div>
        </div>

        <div class="field-block full-row">
          <div class="field-label required">副本数量</div>
          <div class="field-content">
            <el-input
              v-model.number="deploymentForm.replicas"
              type="number"
              min="0"
              placeholder="1"
            />
            <p class="field-help">Pod 副本的数量</p>
          </div>
        </div>

        <div class="field-block full-row">
          <div class="field-label">容器设置</div>
          <div class="field-content">
            <div
              v-for="(container, containerIndex) in deploymentForm.containers"
              :key="container.id"
              class="container-row"
            >
              <div class="inline-field">
                <span>容器名称</span>
                <el-input v-model.trim="container.name" placeholder="nginx" clearable />
                <p class="field-help">允许数字、字母和“-”组成，不能以“-”和数字开头，不能以“-”结尾</p>
              </div>
              <div class="inline-field">
                <span>镜像<span class="required-mark">*</span></span>
                <el-input v-model.trim="container.image" placeholder="nginx:1.25" clearable />
                <p class="field-help">镜像地址</p>
              </div>
              <div class="inline-field pull-policy-field">
                <span>镜像拉取策略<span class="required-mark">*</span></span>
                <el-select v-model="container.imagePullPolicy">
                  <el-option label="始终拉取" value="Always" />
                  <el-option label="本地不存在时拉取" value="IfNotPresent" />
                  <el-option label="从不拉取" value="Never" />
                </el-select>
              </div>
              <el-button
                v-if="deploymentForm.containers.length > 1"
                class="remove-row-button container-remove"
                text
                @click="removeItem(deploymentForm.containers, containerIndex)"
              >
                ×
              </el-button>
            </div>
            <el-button type="success" class="add-line-button" @click="addContainer">+ 添加容器</el-button>
          </div>
        </div>

        <div class="field-block full-row">
          <div class="field-label">注解</div>
          <div class="field-content">
            <div
              v-for="(annotation, annotationIndex) in deploymentForm.annotations"
              :key="annotation.id"
              class="pair-row"
            >
              <el-input v-model.trim="annotation.key" placeholder="Key" />
              <el-input v-model.trim="annotation.value" placeholder="Value" />
              <el-button class="remove-row-button" text @click="removeItem(deploymentForm.annotations, annotationIndex)">
                ×
              </el-button>
            </div>
            <el-button type="success" class="add-line-button" @click="addKeyValue(deploymentForm.annotations)">
              + 新增
            </el-button>
          </div>
        </div>
      </el-form>

      <div class="action-buttons">
        <el-button @click="handleClose">取消</el-button>
        <el-button
          type="primary"
          :loading="loading"
          @click="handleSubmit"
        >
          {{ createMode === 'yaml' ? '通过 YAML 创建' : '确认' }}
        </el-button>
      </div>
    </div>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, watch, nextTick, defineProps, defineEmits } from 'vue'
import { ElMessage } from 'element-plus'
import { Search, DocumentCopy } from '@element-plus/icons-vue'
import CodeEditor from '@/components/CodeEditor.vue'

const props = defineProps({
  visible: {
    type: Boolean,
    default: false
  },
  clusterId: {
    type: String,
    default: ''
  },
  clusterName: {
    type: String,
    default: ''
  },
  namespace: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['update:visible', 'close', 'preview', 'create'])

const yamlEditor = ref(null)
const formRef = ref(null)
const loading = ref(false)
const dryRunResult = ref(null)
const createMode = ref('yaml')
const yamlContent = ref('')

let rowId = 0
const nextRowId = () => {
  rowId += 1
  return rowId
}

const createKeyValue = (key = '', value = '') => ({
  id: nextRowId(),
  key,
  value
})

const createContainer = () => ({
  id: nextRowId(),
  name: '',
  image: '',
  imagePullPolicy: 'Always'
})

const createDefaultDeploymentForm = () => ({
  namespace: props.namespace || 'default',
  name: '',
  replicas: 1,
  labels: [createKeyValue('version', 'v1')],
  annotations: [createKeyValue('description', '')],
  containers: [createContainer()]
})

const deploymentForm = reactive(createDefaultDeploymentForm())

const defaultPodYaml = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: custom-deployment-\${timestamp}
  namespace: \${namespace}
  labels:
    app: custom-deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: custom-deployment
  template:
    metadata:
      labels:
        app: custom-deployment
    spec:
      containers:
      - name: main
        image: nginx:latest
        ports:
        - containerPort: 80
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi`

const normalizeText = (value) => String(value ?? '').trim()
const namePattern = /^[A-Za-z](?:[A-Za-z0-9-]*[A-Za-z0-9])?$/

const initializeYaml = () => {
  const timestamp = new Date().getTime()

  yamlContent.value = defaultPodYaml
    .replace('${timestamp}', timestamp)
    .replace('${namespace}', props.namespace || 'default')
}

const resetDeploymentForm = () => {
  Object.assign(deploymentForm, createDefaultDeploymentForm())
}

watch(() => props.visible, (newVal) => {
  if (newVal) {
    createMode.value = 'yaml'
    initializeYaml()
    resetDeploymentForm()
    dryRunResult.value = null

    nextTick(() => {
      if (yamlEditor.value && yamlEditor.value.focus) {
        yamlEditor.value.focus()
      }
    })
  }
})

watch(() => props.namespace, (nextNamespace) => {
  deploymentForm.namespace = nextNamespace || 'default'
})

watch(createMode, (mode) => {
  dryRunResult.value = null
  if (mode === 'yaml') {
    nextTick(() => {
      if (yamlEditor.value && yamlEditor.value.focus) {
        yamlEditor.value.focus()
      }
    })
  }
})

const handleClose = () => {
  emit('update:visible', false)
  emit('close')
}

const previewPodCreation = async () => {
  if (!yamlContent.value.trim()) {
    ElMessage.warning('YAML 内容不能为空')
    return
  }

  emit('preview', {
    mode: 'yaml',
    yamlContent: yamlContent.value,
    dryRun: true
  })
}

const addKeyValue = (list) => {
  list.push(createKeyValue())
}

const removeItem = (list, index) => {
  list.splice(index, 1)
}

const addContainer = () => {
  deploymentForm.containers.push(createContainer())
}

const toKeyValueObject = (items) => {
  return items.reduce((result, item) => {
    const key = normalizeText(item.key)
    if (!key) return result

    result[key] = normalizeText(item.value)
    return result
  }, {})
}

const cleanObject = (source) => {
  return Object.entries(source).reduce((result, [key, value]) => {
    if (Array.isArray(value)) {
      if (value.length) result[key] = value
      return result
    }

    if (value && typeof value === 'object') {
      const nextValue = cleanObject(value)
      if (Object.keys(nextValue).length) result[key] = nextValue
      return result
    }

    if (value !== undefined && value !== null && value !== '') {
      result[key] = value
    }

    return result
  }, {})
}

const validateName = (value, label) => {
  const name = normalizeText(value)
  if (!name) {
    ElMessage.warning(`请输入${label}`)
    return false
  }

  if (!namePattern.test(name)) {
    ElMessage.warning(`${label}格式不正确`)
    return false
  }

  return true
}

const validateDeploymentForm = () => {
  if (!validateName(deploymentForm.name, 'Deployment 名称')) {
    return false
  }

  const replicas = Number(deploymentForm.replicas)
  if (!Number.isInteger(replicas) || replicas < 0) {
    ElMessage.warning('副本数量必须是大于等于 0 的整数')
    return false
  }

  if (!deploymentForm.containers.length) {
    ElMessage.warning('至少需要添加一个容器')
    return false
  }

  for (const [containerIndex, container] of deploymentForm.containers.entries()) {
    if (normalizeText(container.name) && !namePattern.test(normalizeText(container.name))) {
      ElMessage.warning(`容器 ${containerIndex + 1} 名称格式不正确`)
      return false
    }

    if (!normalizeText(container.image)) {
      ElMessage.warning(`请输入容器 ${containerIndex + 1} 的镜像`)
      return false
    }
  }

  return true
}

const buildDeploymentPayload = () => {
  if (!validateDeploymentForm()) return null

  const labels = toKeyValueObject(deploymentForm.labels)
  const containers = deploymentForm.containers.map((container, index) => cleanObject({
    name: normalizeText(container.name) || `container-${index + 1}`,
    image: normalizeText(container.image),
    imagePullPolicy: container.imagePullPolicy || 'Always'
  }))

  return cleanObject({
    name: normalizeText(deploymentForm.name),
    namespace: deploymentForm.namespace || props.namespace || undefined,
    replicas: Number(deploymentForm.replicas),
    labels,
    annotations: toKeyValueObject(deploymentForm.annotations),
    template: {
      containers
    }
  })
}

const copyText = async (text, successMessage) => {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success(successMessage)
  } catch (error) {
    try {
      const textArea = document.createElement('textarea')
      textArea.value = text
      document.body.appendChild(textArea)
      textArea.select()
      document.execCommand('copy')
      document.body.removeChild(textArea)
      ElMessage.success(successMessage)
    } catch (fallbackError) {
      console.error('复制失败:', fallbackError)
      ElMessage.error('复制失败，请手动复制')
    }
  }
}

const copyToClipboard = async () => {
  if (!yamlContent.value || typeof yamlContent.value !== 'string' || yamlContent.value.trim() === '') {
    ElMessage.warning('没有内容可以复制')
    return
  }

  await copyText(yamlContent.value, 'YAML 已复制到剪贴板')
}

const handleSubmit = async () => {
  if (createMode.value === 'yaml') {
    if (!yamlContent.value.trim()) {
      ElMessage.warning('YAML 内容不能为空')
      return
    }

    loading.value = true

    try {
      emit('create', {
        mode: 'yaml',
        yamlContent: yamlContent.value,
        dryRun: false
      })
    } finally {
      loading.value = false
    }

    return
  }

  const payload = buildDeploymentPayload()
  if (!payload) return

  loading.value = true

  try {
    emit('create', {
      mode: 'form',
      resourceType: 'Deployment',
      payload
    })
  } finally {
    loading.value = false
  }
}

defineExpose({
  setDryRunResult: (result) => {
    dryRunResult.value = result
  },
  setLoading: (state) => {
    loading.value = state
  }
})
</script>

<style scoped>
.create-pod-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-height: calc(86vh - 82px);
  overflow: auto;
  padding-right: 4px;
}

.dialog-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.mode-switch :deep(.el-radio-button__inner) {
  height: 32px;
  padding: 0 18px;
  border-color: #dfe7f2;
  background: #ffffff;
  color: #5f6b7c;
  font-weight: 700;
  line-height: 30px;
  box-shadow: none;
}

.mode-switch :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) {
  border-color: #4c8bf5;
  background: #4c8bf5;
  color: #ffffff;
  box-shadow: none;
}

.yaml-actions,
.target-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.target-info {
  flex-wrap: wrap;
  padding: 12px 14px;
  border: 1px solid #dfe7f2;
  border-radius: 12px;
  background: #f8fafc;
}

.target-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.target-label {
  color: #5f6b7c;
  font-size: 13px;
  font-weight: 600;
}

.validate-btn {
  border: none;
  background: #4c8bf5;
  color: #ffffff;
  box-shadow: none;
}

.copy-btn {
  border: 1px solid #dfe7f2;
  background: #ffffff;
  color: #4c8bf5;
  box-shadow: none;
}

.yaml-editor-container {
  overflow: hidden;
  border: 1px solid #dfe7f2;
  border-radius: 12px;
  background: #ffffff;
}

.simple-deployment-form {
  display: flex;
  flex-direction: column;
  gap: 22px;
  padding: 2px 2px 0;
}

.form-grid.two-column {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  column-gap: 46px;
  row-gap: 18px;
}

.field-block {
  display: grid;
  grid-template-columns: 104px minmax(0, 1fr);
  gap: 8px;
  align-items: start;
}

.field-block.full-row {
  width: 100%;
}

.field-label {
  padding-top: 8px;
  color: #344054;
  font-size: 13px;
  font-weight: 600;
  text-align: right;
  white-space: nowrap;
}

.field-label.required::before,
.required-mark {
  color: #ef6b73;
  content: "*";
  margin-right: 2px;
}

.field-content {
  min-width: 0;
}

.field-help {
  margin: 6px 0 0;
  color: #8d97a8;
  font-size: 12px;
  line-height: 1.5;
}

.pair-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) 30px;
  gap: 10px;
  align-items: center;
}

.pair-row + .pair-row {
  margin-top: 8px;
}

.container-row {
  display: grid;
  grid-template-columns: minmax(180px, 230px) minmax(220px, 1fr) minmax(170px, 210px) 30px;
  gap: 10px;
  align-items: start;
}

.container-row + .container-row {
  margin-top: 12px;
}

.inline-field > span {
  display: block;
  margin-bottom: 6px;
  color: #344054;
  font-size: 13px;
  font-weight: 600;
}

.pull-policy-field {
  min-width: 0;
}

.remove-row-button {
  width: 28px;
  height: 32px;
  padding: 0;
  color: #a7b0bf;
  font-size: 18px;
  font-weight: 400;
}

.remove-row-button:hover {
  color: #ef6b73;
  background: transparent;
}

.container-remove {
  margin-top: 24px;
}

.add-line-button {
  height: 32px;
  margin-top: 8px;
  padding: 0 14px;
  border: 1px solid #dfe7f2;
  border-radius: 4px;
  background: #ffffff;
  color: #344054;
  font-weight: 500;
}

.add-line-button:hover {
  border-color: #4c8bf5;
  color: #4c8bf5;
  background: #ffffff;
}

.dryrun-result {
  margin-top: 0;
}

.action-buttons {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 2px;
  padding-top: 14px;
  border-top: 1px solid #edf1f7;
}

.create-pod-dialog :deep(.el-dialog) {
  overflow: hidden;
  max-width: calc(100vw - 32px);
  border: 1px solid #dfe7f2;
  border-radius: 6px;
  background: #ffffff;
  box-shadow: 0 28px 80px rgba(15, 23, 42, 0.18);
}

.create-pod-dialog :deep(.el-dialog__header) {
  padding: 20px 20px 14px;
  border-bottom: 1px solid #edf1f7;
  background: #ffffff;
}

.create-pod-dialog :deep(.el-dialog__title) {
  color: #172133;
  font-size: 15px;
  font-weight: 600;
}

.create-pod-dialog :deep(.el-dialog__body) {
  padding: 16px 20px 20px;
  background: #ffffff;
}

.create-pod-dialog :deep(.el-tag) {
  border-radius: 999px;
  font-weight: 500;
}

.create-pod-dialog :deep(.el-input__wrapper),
.create-pod-dialog :deep(.el-select .el-input__wrapper) {
  min-height: 32px;
  border: 1px solid #dfe4ec;
  border-radius: 4px;
  background: #ffffff;
  box-shadow: none;
}

.create-pod-dialog :deep(.el-input__wrapper:hover),
.create-pod-dialog :deep(.el-select .el-input__wrapper:hover) {
  border-color: #b8c4d6;
}

.create-pod-dialog :deep(.el-input__wrapper.is-focus),
.create-pod-dialog :deep(.el-select .el-input__wrapper.is-focus) {
  border-color: #2f6df6;
  box-shadow: none;
}

.create-pod-dialog :deep(.el-input.is-disabled .el-input__wrapper),
.create-pod-dialog :deep(.el-select .el-input.is-disabled .el-input__wrapper) {
  background: #f8fafc;
}

.create-pod-dialog :deep(.el-button) {
  border-radius: 4px;
  font-weight: 500;
}

.action-buttons :deep(.el-button--primary) {
  min-width: 72px;
  border-color: #2f6df6;
  background: #2f6df6;
}

@media (max-width: 900px) {
  .form-grid.two-column {
    grid-template-columns: 1fr;
    gap: 18px;
  }

  .container-row,
  .pair-row {
    grid-template-columns: 1fr;
  }

  .container-remove {
    margin-top: 0;
  }
}

@media (max-width: 768px) {
  .create-pod-dialog :deep(.el-dialog) {
    width: calc(100vw - 24px) !important;
  }

  .create-pod-content {
    max-height: calc(88vh - 82px);
  }

  .dialog-toolbar,
  .action-buttons {
    align-items: stretch;
    flex-direction: column;
  }

  .mode-switch,
  .mode-switch :deep(.el-radio-button),
  .mode-switch :deep(.el-radio-button__inner) {
    width: 100%;
  }

  .field-block {
    grid-template-columns: 1fr;
  }

  .field-label {
    padding-top: 0;
    text-align: left;
  }

  .action-buttons .el-button {
    width: 100%;
    margin-left: 0;
  }
}

.create-pod-content {
  gap: 12px;
  padding-right: 2px;
}

.dialog-toolbar {
  gap: 8px;
}

.mode-switch :deep(.el-radio-button__inner) {
  height: 34px;
  padding: 0 16px;
  border-color: #d9e5f2;
  line-height: 32px;
}

.target-info {
  gap: 8px;
  padding: 9px 10px;
  border-color: #dce8f5;
  border-radius: 10px;
  background: #f8fbff;
}

.target-label,
.field-label,
.inline-field > span {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

.yaml-editor-container {
  border-color: #dce8f5;
  border-radius: 10px;
}

.simple-deployment-form {
  gap: 14px;
}

.form-grid.two-column {
  column-gap: 24px;
  row-gap: 14px;
}

.field-help {
  margin-top: 4px;
  color: #64748b;
  font-size: 11px;
}

.pair-row,
.container-row {
  gap: 8px;
}

.add-line-button {
  height: 32px;
  margin-top: 6px;
  border-color: #67c23a;
  border-radius: 8px;
  background: #67c23a;
  color: #fff;
  font-weight: 600;
}

.add-line-button:hover,
.add-line-button:focus {
  border-color: #529b2e;
  background: #529b2e;
  color: #fff;
  transform: none;
}

.action-buttons {
  gap: 10px;
  padding-top: 12px;
  border-top-color: #e3edf8;
}

.create-pod-dialog :deep(.el-input__wrapper),
.create-pod-dialog :deep(.el-select__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.create-pod-dialog :deep(.el-input__wrapper:hover),
.create-pod-dialog :deep(.el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.create-pod-dialog :deep(.el-button) {
  border-radius: 8px;
  font-weight: 600;
  transform: none;
}

.create-pod-dialog :deep(.yaml-actions .el-button) {
  width: 32px;
  height: 32px;
  min-height: 32px;
  padding: 0;
}
</style>
