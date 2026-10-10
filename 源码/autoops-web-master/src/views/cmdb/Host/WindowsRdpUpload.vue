<template>
  <el-dialog
    v-model="visible"
    title="Windows 文件上传"
    width="560px"
    @closed="handleClosed"
  >
    <el-alert type="info" :closable="false" show-icon class="upload-intro">
      <template #title>文件会自动上传到目标 Windows 主机当前登录用户的 Downloads 目录</template>
      <div class="upload-intro-text">
        前端只负责提交文件和轮询进度，WinRM 连通性、上传模式和目标位置由后端自动处理。
      </div>
    </el-alert>

    <el-form label-width="96px" class="upload-form">
      <el-form-item label="目标主机">
        <el-input :model-value="currentHost.hostName || ''" disabled />
      </el-form-item>

      <el-form-item label="选择文件" required>
        <el-upload
          :auto-upload="false"
          :show-file-list="false"
          :disabled="isUploading"
          :on-change="handleFileChange"
        >
          <el-button type="primary" :disabled="isUploading">选择文件</el-button>
        </el-upload>

        <div v-if="selectedFile" class="selected-file">
          已选择：{{ selectedFile.name }}
          <el-button
            type="danger"
            text
            size="small"
            :disabled="isUploading"
            @click="clearSelectedFile"
          >
            移除
          </el-button>
        </div>
        <div v-else class="selected-file selected-file--muted">
          请选择一个要上传到目标 Windows 主机的本地文件
        </div>
      </el-form-item>

      <el-form-item label="目标文件名">
        <el-input
          v-model="targetFileName"
          clearable
          placeholder="不填则使用原文件名"
          :disabled="isUploading"
        />
      </el-form-item>
    </el-form>

    <div v-if="taskId || isUploading || uploadProgress > 0" class="progress-panel">
      <div class="progress-header">
        <span class="progress-text">{{ uploadStatusText }}</span>
        <el-tag size="small" :type="taskStatusTagType">{{ taskStatusText }}</el-tag>
      </div>
      <el-progress :percentage="uploadProgress" :status="progressStatus" />
      <div v-if="uploadedBytes > 0 || totalBytes > 0" class="progress-meta">
        {{ formatBytes(uploadedBytes) }} / {{ formatBytes(totalBytes) }}
      </div>
    </div>

    <el-alert
      v-if="taskErrorText"
      type="error"
      :closable="false"
      show-icon
      class="status-alert"
      :title="taskErrorText"
    />

    <el-alert
      v-if="uploadResult"
      type="success"
      :closable="false"
      show-icon
      class="status-alert"
      title="上传成功"
    >
      <div class="result-block">
        <div class="result-line">
          <span class="result-label">状态</span>
          <span>{{ uploadResult.status }}</span>
        </div>
        <div class="result-line">
          <span class="result-label">位置</span>
          <span class="path-text">{{ uploadResult.remotePath || '-' }}</span>
        </div>
      </div>
    </el-alert>

    <template #footer>
      <el-button @click="visible = false" :disabled="isUploading">关闭</el-button>
      <el-button
        type="primary"
        :loading="isUploading"
        :disabled="!selectedFile"
        @click="handleUpload"
      >
        上传到 Windows
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import API from '@/api/cmdb'

const emit = defineEmits(['upload-success'])

const visible = ref(false)
const currentHost = ref({})
const selectedFile = ref(null)
const targetFileName = ref('')
const isUploading = ref(false)
const uploadProgress = ref(0)
const uploadResult = ref(null)
const uploadStatusText = ref('')
const taskId = ref('')
const taskStatus = ref('')
const taskMode = ref('')
const uploadedBytes = ref(0)
const totalBytes = ref(0)
const taskErrorText = ref('')
let uploadPollingTimer = null

const progressStatus = computed(() => {
  if (taskStatus.value === 'failed') return 'exception'
  if (taskStatus.value === 'completed' && uploadProgress.value >= 100) return 'success'
  return ''
})

const taskStatusText = computed(() => {
  const map = {
    pending: '等待中',
    uploading: '上传中',
    completed: '已完成',
    failed: '失败'
  }
  return map[taskStatus.value] || '未开始'
})

const taskStatusTagType = computed(() => {
  const map = {
    pending: 'warning',
    uploading: 'primary',
    completed: 'success',
    failed: 'danger'
  }
  return map[taskStatus.value] || 'info'
})

const getHostId = host => host?.id ?? host?.hostId ?? host?.host_id

const resetTaskState = () => {
  taskId.value = ''
  taskStatus.value = ''
  taskMode.value = ''
  uploadedBytes.value = 0
  totalBytes.value = 0
  taskErrorText.value = ''
  uploadStatusText.value = ''
  uploadProgress.value = 0
  uploadResult.value = null
}

const resetState = () => {
  selectedFile.value = null
  targetFileName.value = ''
  isUploading.value = false
  clearUploadPolling()
  resetTaskState()
}

const clearSelectedFile = () => {
  selectedFile.value = null
}

const handleFileChange = file => {
  selectedFile.value = file?.raw || null
}

const clearUploadPolling = () => {
  if (uploadPollingTimer) {
    clearInterval(uploadPollingTimer)
    uploadPollingTimer = null
  }
}

const formatBytes = value => {
  const bytes = Number(value || 0)
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'

  const units = ['B', 'KB', 'MB', 'GB']
  let size = bytes
  let unitIndex = 0

  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex += 1
  }

  return `${size.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`
}

const normalizeTask = payload => {
  if (!payload || typeof payload !== 'object') return null

  const task = payload.task && typeof payload.task === 'object'
    ? payload.task
    : payload

  if (!task || typeof task !== 'object') return null

  return {
    taskId: task.taskId || '',
    status: String(task.status || '').trim().toLowerCase(),
    progress: Math.max(0, Math.min(100, Math.round(Number(task.progress || 0)))),
    uploadedBytes: Number(task.uploadedBytes || 0),
    totalBytes: Number(task.totalBytes || 0),
    mode: task.mode || '',
    remotePath: task.remotePath || '',
    message: task.message || '',
    error: task.error || ''
  }
}

const applyTaskState = task => {
  if (!task) return

  taskId.value = task.taskId || taskId.value
  taskStatus.value = task.status
  taskMode.value = task.mode || ''
  uploadedBytes.value = task.uploadedBytes
  totalBytes.value = task.totalBytes
  uploadProgress.value = task.progress
  uploadStatusText.value = task.message || (
    task.status === 'completed'
      ? '上传完成'
      : task.status === 'failed'
        ? '上传失败'
        : task.status === 'uploading'
          ? '正在上传文件到目标主机...'
          : '任务已创建，等待上传'
  )
  taskErrorText.value = task.error || ''
}

const pollUploadTaskStatus = async () => {
  if (!taskId.value) return

  try {
    const { data: res } = await API.getWindowsUploadTaskStatus(taskId.value)
    if (Number(res?.code) !== 200) {
      throw new Error(res?.message || '查询上传进度失败')
    }

    const payload = res?.data && typeof res.data === 'object' ? res.data : {}
    const task = normalizeTask(payload)
    if (!task) {
      throw new Error('未获取到有效的上传任务信息')
    }

    applyTaskState(task)

    if (task.status === 'completed') {
      clearUploadPolling()
      isUploading.value = false
      uploadProgress.value = 100
      uploadResult.value = {
        status: '上传成功',
        remotePath: task.remotePath || ''
      }
      ElMessage.success('文件上传成功')
      emit('upload-success', uploadResult.value)
      return
    }

    if (task.status === 'failed') {
      clearUploadPolling()
      isUploading.value = false
      taskErrorText.value = task.error || task.message || '文件上传失败'
      ElMessage.error(taskErrorText.value)
    }
  } catch (error) {
    clearUploadPolling()
    isUploading.value = false
    taskErrorText.value = error?.message || '查询上传进度失败'
    ElMessage.error(taskErrorText.value)
  }
}

const startUploadPolling = nextTaskId => {
  if (!nextTaskId) return

  clearUploadPolling()
  taskId.value = nextTaskId
  pollUploadTaskStatus()
  uploadPollingTimer = setInterval(() => {
    pollUploadTaskStatus()
  }, 2000)
}

const handleUpload = async () => {
  const hostId = getHostId(currentHost.value)

  if (!hostId) {
    ElMessage.error('缺少主机 ID，无法上传文件')
    return
  }

  if (!selectedFile.value) {
    ElMessage.warning('请选择要上传的文件')
    return
  }

  const formData = new FormData()
  formData.append('file', selectedFile.value)

  const trimmedFileName = targetFileName.value.trim()
  if (trimmedFileName) {
    formData.append('fileName', trimmedFileName)
  }

  try {
    isUploading.value = true
    clearUploadPolling()
    resetTaskState()
    uploadStatusText.value = '文件已提交，等待后台创建上传任务...'

    const { data: res } = await API.uploadFileToWindowsHost(hostId, formData)
    if (Number(res?.code) !== 200) {
      throw new Error(res?.message || '文件上传失败')
    }

    const payload = res?.data && typeof res.data === 'object' ? res.data : {}
    const task = normalizeTask(payload.task)
    if (!task?.taskId) {
      throw new Error('后端未返回有效的上传任务 ID')
    }

    applyTaskState(task)
    uploadStatusText.value = payload.message || task.message || '文件已接收，后台上传任务已启动'
    startUploadPolling(task.taskId)
  } catch (error) {
    clearUploadPolling()
    isUploading.value = false
    resetTaskState()
    ElMessage.error(error?.message || '文件上传失败')
  }
}

const show = host => {
  currentHost.value = host || {}
  resetState()
  visible.value = true
}

const handleClosed = () => {
  currentHost.value = {}
  resetState()
}

defineExpose({
  show
})
</script>

<style scoped>
.upload-intro {
  margin-bottom: 16px;
}

.upload-intro-text {
  line-height: 1.6;
}

.upload-form {
  margin-top: 8px;
}

.selected-file {
  margin-top: 10px;
  color: #303133;
  line-height: 1.6;
}

.selected-file--muted {
  color: #909399;
}

.progress-panel {
  margin-top: 12px;
}

.progress-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.progress-text {
  color: #606266;
  font-size: 13px;
  line-height: 1.5;
}

.progress-meta {
  margin-top: 8px;
  color: #909399;
  font-size: 12px;
  text-align: right;
}

.status-alert {
  margin-top: 16px;
}

.result-block {
  display: flex;
  flex-direction: column;
  gap: 8px;
  line-height: 1.6;
}

.result-line {
  display: flex;
  gap: 8px;
  align-items: flex-start;
}

.result-label {
  min-width: 60px;
  color: #909399;
}

.path-text {
  word-break: break-all;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  color: #303133;
}
</style>
