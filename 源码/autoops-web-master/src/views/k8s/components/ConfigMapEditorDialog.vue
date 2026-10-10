<template>
  <el-dialog
    :model-value="visible"
    :title="dialogTitle"
    width="980px"
    class="configmap-editor-dialog"
    modal-class="k8s-main-content-dialog-modal"
    append-to-body
    :close-on-click-modal="false"
    @update:model-value="handleClose"
  >
    <div class="configmap-editor">
      <div class="editor-toolbar">
        <div class="resource-meta">
          <el-tag type="info" size="small">ConfigMap</el-tag>
          <el-tag type="warning" size="small">{{ displayResourceName }}</el-tag>
          <el-tag size="small" type="success">{{ entryCountText }}</el-tag>
        </div>
        <div class="editor-actions">
          <el-radio-group v-model="editorMode" size="small" @change="handleModeChange">
            <el-radio-button label="visual">标签页编辑</el-radio-button>
            <el-radio-button label="yaml">YAML</el-radio-button>
          </el-radio-group>
          <el-tooltip content="复制 YAML" placement="top">
            <el-button
              :icon="DocumentCopy"
              size="small"
              circle
              :disabled="!yamlText.trim()"
              @click="copyYaml"
            />
          </el-tooltip>
        </div>
      </div>

      <el-alert
        v-if="parseError"
        :title="parseError"
        type="warning"
        show-icon
        :closable="false"
        class="parse-alert"
      />

      <div v-if="editorMode === 'visual'" class="visual-editor">
        <div class="visual-header">
          <div class="identity-fields">
            <el-input
              v-model="metadataForm.name"
              size="small"
              placeholder="ConfigMap 名称"
              :disabled="!editable"
            >
              <template #prepend>名称</template>
            </el-input>
            <el-input
              v-model="metadataForm.namespace"
              size="small"
              placeholder="命名空间"
              :disabled="!editable"
            >
              <template #prepend>命名空间</template>
            </el-input>
          </div>
          <el-button
            v-if="editable"
            type="primary"
            size="small"
            :icon="Plus"
            @click="addEntry"
          >
            添加配置项
          </el-button>
        </div>

        <el-empty
          v-if="!configEntries.length"
          description="暂无配置项"
          class="config-empty"
        >
          <el-button v-if="editable" type="primary" size="small" :icon="Plus" @click="addEntry">
            添加配置项
          </el-button>
        </el-empty>

        <el-tabs
          v-else
          v-model="activeEntryId"
          type="border-card"
          class="config-entry-tabs"
        >
          <el-tab-pane
            v-for="entry in configEntries"
            :key="entry.id"
            :name="entry.id"
          >
            <template #label>
              <span class="entry-tab-label" :title="entry.key || '未命名配置项'">
                {{ entry.key || '未命名配置项' }}
              </span>
            </template>

            <div class="entry-editor">
              <div class="entry-fields">
                <el-input
                  v-model="entry.key"
                  size="small"
                  placeholder="配置项 Key，例如 app.yaml"
                  :disabled="!editable"
                >
                  <template #prepend>Key</template>
                </el-input>
                <el-select
                  v-model="entry.section"
                  size="small"
                  :disabled="!editable"
                  class="entry-section-select"
                >
                  <el-option label="data" value="data" />
                  <el-option label="binaryData" value="binaryData" />
                </el-select>
                <el-button
                  v-if="editable"
                  type="danger"
                  size="small"
                  plain
                  @click="removeEntry(entry.id)"
                >
                  删除
                </el-button>
              </div>
              <CodeEditor
                v-model="entry.value"
                language="yaml"
                height="360px"
                theme="vs-dark"
                word-wrap="on"
                :readonly="!editable"
              />
            </div>
          </el-tab-pane>
        </el-tabs>
      </div>

      <div v-else class="yaml-editor">
        <CodeEditor
          v-model="yamlText"
          language="yaml"
          height="440px"
          theme="vs-dark"
          word-wrap="on"
          :readonly="!editable"
        />
      </div>

      <div class="dialog-footer">
        <el-button @click="handleClose">{{ editable ? '取消' : '关闭' }}</el-button>
        <el-button
          v-if="editable"
          type="primary"
          :loading="saving"
          @click="saveConfigMap"
        >
          保存
        </el-button>
      </div>
    </div>
  </el-dialog>
</template>

<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { DocumentCopy, Plus } from '@element-plus/icons-vue'
import yaml from 'js-yaml'
import CodeEditor from '@/components/CodeEditor.vue'

const props = defineProps({
  visible: { type: Boolean, default: false },
  yamlContent: { type: String, default: '' },
  resourceName: { type: String, default: '' },
  resourceType: { type: String, default: 'ConfigMap' },
  editable: { type: Boolean, default: true }
})

const emit = defineEmits(['update:visible', 'close', 'save'])

const yamlText = ref('')
const editorMode = ref('visual')
const parseError = ref('')
const saving = ref(false)
const activeEntryId = ref('')
const configEntries = ref([])
const baseDocument = ref({})
const metadataForm = reactive({
  name: '',
  namespace: ''
})

let entrySeed = 0

const dialogTitle = computed(() => `${props.editable ? '编辑 ConfigMap' : '查看 ConfigMap'} - ${displayResourceName.value}`)
const displayResourceName = computed(() => metadataForm.name || props.resourceName || 'new-configmap')
const entryCountText = computed(() => `共 ${configEntries.value.length} 项`)

const createEntryId = () => `config-entry-${Date.now()}-${entrySeed++}`

const normalizeObject = value => (value && typeof value === 'object' && !Array.isArray(value) ? value : {})

const parseYamlDocument = content => {
  const parsed = yaml.load(content || '') || {}
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('YAML 顶层必须是对象')
  }
  return parsed
}

const loadFromYaml = content => {
  yamlText.value = String(content || '')
  parseError.value = ''

  try {
    const parsed = parseYamlDocument(yamlText.value)
    baseDocument.value = parsed
    metadataForm.name = parsed?.metadata?.name || props.resourceName || ''
    metadataForm.namespace = parsed?.metadata?.namespace || ''

    const entries = []
    ;['data', 'binaryData'].forEach(section => {
      const sectionData = normalizeObject(parsed[section])
      Object.keys(sectionData).forEach(key => {
        entries.push({
          id: createEntryId(),
          section,
          key,
          value: sectionData[key] === null || sectionData[key] === undefined ? '' : String(sectionData[key])
        })
      })
    })

    configEntries.value = entries
    activeEntryId.value = entries[0]?.id || ''
    editorMode.value = 'visual'
  } catch (error) {
    parseError.value = `YAML 解析失败，已切换到原始 YAML 编辑：${error.message}`
    baseDocument.value = {}
    metadataForm.name = props.resourceName || ''
    metadataForm.namespace = ''
    configEntries.value = []
    activeEntryId.value = ''
    editorMode.value = 'yaml'
  }
}

const validateEntries = () => {
  const seenKeys = new Set()

  for (const entry of configEntries.value) {
    const key = String(entry.key || '').trim()
    if (!key) {
      return '配置项 Key 不能为空'
    }
    if (!/^[A-Za-z0-9._-]+$/.test(key)) {
      return `配置项 "${key}" 只能包含字母、数字、点、下划线和中划线`
    }
    if (seenKeys.has(key)) {
      return `配置项 "${key}" 重复`
    }
    seenKeys.add(key)
  }

  if (!String(metadataForm.name || '').trim()) {
    return 'ConfigMap 名称不能为空'
  }

  return ''
}

const buildYamlFromVisual = () => {
  const validationMessage = validateEntries()
  if (validationMessage) {
    throw new Error(validationMessage)
  }

  const document = JSON.parse(JSON.stringify(baseDocument.value || {}))
  document.apiVersion = document.apiVersion || 'v1'
  document.kind = 'ConfigMap'
  document.metadata = normalizeObject(document.metadata)
  document.metadata.name = String(metadataForm.name || '').trim()

  const namespace = String(metadataForm.namespace || '').trim()
  if (namespace) {
    document.metadata.namespace = namespace
  } else if (document.metadata.namespace) {
    delete document.metadata.namespace
  }

  delete document.data
  delete document.binaryData

  const nextData = {}
  const nextBinaryData = {}
  configEntries.value.forEach(entry => {
    const target = entry.section === 'binaryData' ? nextBinaryData : nextData
    target[String(entry.key || '').trim()] = entry.value === null || entry.value === undefined ? '' : String(entry.value)
  })

  if (Object.keys(nextData).length) {
    document.data = nextData
  }
  if (Object.keys(nextBinaryData).length) {
    document.binaryData = nextBinaryData
  }

  return yaml.dump(document, {
    lineWidth: -1,
    noRefs: true,
    quotingType: '"',
    forceQuotes: false
  })
}

const handleModeChange = mode => {
  if (mode === 'yaml') {
    try {
      yamlText.value = buildYamlFromVisual()
      parseError.value = ''
    } catch (error) {
      editorMode.value = 'visual'
      ElMessage.warning(error.message)
    }
    return
  }

  loadFromYaml(yamlText.value)
}

const addEntry = () => {
  const entry = {
    id: createEntryId(),
    section: 'data',
    key: `config-${configEntries.value.length + 1}.yaml`,
    value: ''
  }
  configEntries.value.push(entry)
  activeEntryId.value = entry.id
}

const removeEntry = id => {
  const index = configEntries.value.findIndex(entry => entry.id === id)
  if (index < 0) return

  configEntries.value.splice(index, 1)
  if (activeEntryId.value === id) {
    activeEntryId.value = configEntries.value[Math.max(0, index - 1)]?.id || ''
  }
}

const copyYaml = async () => {
  try {
    const content = editorMode.value === 'visual' ? buildYamlFromVisual() : yamlText.value
    if (!content.trim()) {
      ElMessage.warning('没有内容可以复制')
      return
    }
    await navigator.clipboard.writeText(content)
    ElMessage.success('YAML 已复制到剪贴板')
  } catch (error) {
    ElMessage.warning(error.message || '复制失败')
  }
}

const saveConfigMap = async () => {
  saving.value = true

  try {
    const nextYaml = editorMode.value === 'visual' ? buildYamlFromVisual() : yamlText.value
    parseYamlDocument(nextYaml)

    emit('save', {
      yamlContent: nextYaml,
      resourceName: displayResourceName.value,
      resourceType: props.resourceType
    })
  } catch (error) {
    ElMessage.warning(error.message || 'ConfigMap 内容不合法')
  } finally {
    saving.value = false
  }
}

const handleClose = () => {
  emit('update:visible', false)
  emit('close')
}

watch(
  () => props.visible,
  visible => {
    if (visible) {
      loadFromYaml(props.yamlContent)
    }
  },
  { immediate: true }
)

watch(
  () => props.yamlContent,
  content => {
    if (props.visible) {
      loadFromYaml(content)
    }
  }
)
</script>

<style scoped>
.configmap-editor {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.editor-toolbar,
.visual-header,
.entry-fields,
.dialog-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.editor-toolbar {
  padding: 14px 16px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.82);
}

.resource-meta,
.editor-actions,
.identity-fields {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.identity-fields {
  flex: 1;
}

.identity-fields .el-input {
  max-width: 320px;
}

.parse-alert {
  margin: 0;
}

.visual-editor,
.yaml-editor {
  min-height: 440px;
}

.visual-header {
  margin-bottom: 12px;
}

.config-empty {
  height: 380px;
  border: 1px dashed rgba(190, 203, 222, 0.95);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.68);
}

.config-entry-tabs {
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 18px;
  overflow: hidden;
  background: rgba(255, 255, 255, 0.88);
}

.config-entry-tabs :deep(.el-tabs__header) {
  background: rgba(248, 250, 255, 0.98);
}

.config-entry-tabs :deep(.el-tabs__content) {
  padding: 14px;
}

.entry-tab-label {
  display: inline-block;
  max-width: 180px;
  overflow: hidden;
  text-overflow: ellipsis;
  vertical-align: bottom;
  white-space: nowrap;
}

.entry-editor {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.entry-fields {
  justify-content: flex-start;
}

.entry-fields .el-input {
  max-width: 420px;
}

.entry-section-select {
  width: 140px;
}

.dialog-footer {
  justify-content: flex-end;
  padding-top: 14px;
  border-top: 1px solid rgba(223, 231, 242, 0.76);
}

.configmap-editor-dialog :deep(.el-dialog) {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 24px;
  background: linear-gradient(180deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
  box-shadow: 0 36px 110px rgba(15, 23, 42, 0.11);
  max-height: 84vh;
  display: flex;
  flex-direction: column;
}

.configmap-editor-dialog :deep(.el-dialog__header) {
  padding: 16px 20px 14px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.84);
}

.configmap-editor-dialog :deep(.el-dialog__title) {
  color: #172133;
  font-size: 17px;
  font-weight: 700;
}

.configmap-editor-dialog :deep(.el-dialog__body) {
  padding: 18px 20px;
  overflow-y: auto;
  flex: 1;
  max-height: calc(84vh - 112px);
  background:
    radial-gradient(circle at top left, rgba(76, 139, 245, 0.04), transparent 22%),
    linear-gradient(180deg, #f8fafc 0%, #f5f8fc 100%);
}

.configmap-editor-dialog :deep(.el-tag) {
  border-radius: 999px;
  font-weight: 500;
}

.configmap-editor-dialog :deep(.el-button) {
  border-radius: 13px;
  font-weight: 600;
}

@media (max-width: 768px) {
  .editor-toolbar,
  .visual-header,
  .entry-fields,
  .resource-meta,
  .editor-actions,
  .identity-fields {
    align-items: stretch;
    flex-direction: column;
  }

  .identity-fields .el-input,
  .entry-fields .el-input,
  .entry-section-select {
    width: 100%;
    max-width: none;
  }
}
</style>
