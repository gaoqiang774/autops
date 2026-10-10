<template>
  <div class="k8s-log-source-panel">
    <div class="k8s-tools">
      <el-select
        v-model="selection.clusterId"
        filterable
        clearable
        placeholder="选择集群"
        :loading="clustersLoading"
        @change="handleClusterChange"
      >
        <el-option
          v-for="cluster in clusterOptions"
          :key="cluster.id"
          :label="getClusterName(cluster)"
          :value="cluster.id"
        >
          <div class="cluster-option">
            <span>{{ getClusterName(cluster) }}</span>
            <small>{{ getClusterStatusText(cluster) }}</small>
          </div>
        </el-option>
      </el-select>
      <el-button :icon="Refresh" :loading="clustersLoading" @click="loadClusters(true)" />
    </div>

    <div class="k8s-cascade">
      <el-form label-position="top" @submit.prevent>
        <el-form-item label="命名空间">
          <el-select
            v-model="selection.namespace"
            filterable
            clearable
            placeholder="选择命名空间"
            :disabled="!selection.clusterId"
            :loading="namespacesLoading"
            @change="handleNamespaceChange"
          >
            <el-option
              v-for="namespace in namespaceOptions"
              :key="namespace.name"
              :label="namespace.name"
              :value="namespace.name"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="Pod">
          <el-select
            v-model="selection.pod"
            filterable
            clearable
            placeholder="选择 Pod"
            :disabled="!selection.namespace"
            :loading="podsLoading"
            @change="handlePodChange"
          >
            <el-option
              v-for="pod in podOptions"
              :key="pod.name"
              :label="pod.name"
              :value="pod.name"
            >
              <div class="pod-option">
                <span>{{ pod.name }}</span>
                <small>{{ pod.status || pod.phase || '-' }}</small>
              </div>
            </el-option>
          </el-select>
        </el-form-item>

        <el-form-item label="容器">
          <el-select
            v-model="selection.container"
            filterable
            clearable
            placeholder="默认业务容器"
            :disabled="!selection.pod"
            :loading="containersLoading"
            @change="emitSelection"
          >
            <el-option label="默认业务容器" value="" />
            <el-option
              v-for="container in containerOptions"
              :key="container.name"
              :label="container.name"
              :value="container.name"
            />
          </el-select>
        </el-form-item>

        <div class="k8s-options">
          <el-checkbox v-model="selection.previous" @change="emitSelection">上一实例日志</el-checkbox>
          <el-checkbox v-model="selection.timestamps" @change="emitSelection">包含时间戳</el-checkbox>
        </div>

        <el-button
          type="primary"
          :icon="Search"
          :disabled="!canUseSource"
          @click="emit('query')"
        >
          查询 K8s 日志
        </el-button>
      </el-form>
    </div>

    <el-scrollbar class="k8s-source-scroll">
      <div class="source-summary" :class="{ ready: canUseSource }">
        <el-icon><Connection /></el-icon>
        <strong>{{ sourceTitle }}</strong>
        <span>{{ sourceDescription }}</span>
      </div>

      <div class="selection-list">
        <button
          type="button"
          :class="{ active: !!selection.clusterId }"
          @click="focusClusterSelect"
        >
          <span>集群</span>
          <strong>{{ selectedClusterName || '未选择' }}</strong>
        </button>
        <button
          type="button"
          :class="{ active: !!selection.namespace }"
          @click="selection.clusterId && loadNamespaces(true)"
        >
          <span>命名空间</span>
          <strong>{{ selection.namespace || '未选择' }}</strong>
        </button>
        <button
          type="button"
          :class="{ active: !!selection.pod }"
          @click="selection.namespace && loadPods(true)"
        >
          <span>Pod</span>
          <strong>{{ selection.pod || '未选择' }}</strong>
        </button>
        <button
          type="button"
          :class="{ active: !!selection.container }"
          @click="selection.pod && loadContainers(true)"
        >
          <span>容器</span>
          <strong>{{ selection.container || '默认业务容器' }}</strong>
        </button>
      </div>

      <el-empty
        v-if="!clusterOptions.length && !clustersLoading"
        description="暂无可用 K8s 集群"
        :image-size="72"
      />
    </el-scrollbar>
  </div>
</template>

<script setup>
/* global defineProps, defineEmits */
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { Connection, Refresh, Search } from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'

const props = defineProps({
  modelValue: {
    type: Object,
    default: () => ({})
  }
})

const emit = defineEmits(['update:modelValue', 'change', 'query'])

const successCodes = new Set([0, 200])

const clustersLoading = ref(false)
const namespacesLoading = ref(false)
const podsLoading = ref(false)
const containersLoading = ref(false)
const clusterOptions = ref([])
const namespaceOptions = ref([])
const podOptions = ref([])
const containerOptions = ref([])
const syncingFromParent = ref(false)

const selection = reactive({
  clusterId: '',
  namespace: '',
  pod: '',
  container: '',
  previous: false,
  timestamps: true
})

const getBody = response => response?.data || response || {}

const assertSuccess = (response, fallbackMessage) => {
  const body = getBody(response)
  if (body.code !== undefined && !successCodes.has(Number(body.code)) && !body.success) {
    throw new Error(body.message || body.msg || fallbackMessage)
  }
  return body
}

const getPayload = (response, fallbackMessage = '请求失败') => {
  const body = assertSuccess(response, fallbackMessage)
  return body.data !== undefined ? body.data : body
}

const toList = payload => {
  if (Array.isArray(payload)) return payload
  if (Array.isArray(payload?.list)) return payload.list
  if (Array.isArray(payload?.items)) return payload.items
  if (Array.isArray(payload?.records)) return payload.records
  if (Array.isArray(payload?.namespaces)) return payload.namespaces
  if (Array.isArray(payload?.pods)) return payload.pods
  if (Array.isArray(payload?.containers)) return payload.containers
  if (Array.isArray(payload?.data)) return payload.data
  return []
}

const getClusterName = cluster => cluster?.name || cluster?.clusterName || `Cluster ${cluster?.id || ''}`

const getClusterStatusText = cluster => {
  const status = cluster?.status
  if (status === 2 || status === 'online' || status === 'ready') return '在线'
  if (status === 1 || status === 'offline') return '离线'
  return status === undefined || status === null || status === '' ? '未知' : String(status)
}

const normalizeNamespace = namespace => {
  if (typeof namespace === 'string') return { name: namespace }
  return {
    ...namespace,
    name: namespace?.name || namespace?.namespace || namespace?.metadata?.name || ''
  }
}

const normalizePod = pod => {
  if (typeof pod === 'string') return { name: pod }
  return {
    ...pod,
    name: pod?.name || pod?.podName || pod?.metadata?.name || ''
  }
}

const normalizeContainer = container => {
  if (typeof container === 'string') return { name: container }
  return {
    ...container,
    name: container?.name || container?.containerName || ''
  }
}

const selectedCluster = computed(() =>
  clusterOptions.value.find(cluster => String(cluster.id) === String(selection.clusterId)) || null
)

const selectedClusterName = computed(() => selectedCluster.value ? getClusterName(selectedCluster.value) : '')

const canUseSource = computed(() =>
  !!selection.clusterId && !!selection.namespace && !!selection.pod
)

const sourceTitle = computed(() => canUseSource.value ? 'K8s 日志源已就绪' : '选择 K8s 日志源')

const sourceDescription = computed(() => {
  if (!selection.clusterId) return '先选择集群，再依次选择命名空间、Pod 和容器。'
  if (!selection.namespace) return '请选择命名空间。'
  if (!selection.pod) return '请选择需要分析的 Pod。'
  const container = selection.container || '默认业务容器'
  return `${selectedClusterName.value} / ${selection.namespace} / ${selection.pod} / ${container}`
})

const emitSelection = () => {
  if (syncingFromParent.value) return
  const payload = {
    clusterId: selection.clusterId,
    namespace: selection.namespace,
    pod: selection.pod,
    container: selection.container || '',
    previous: !!selection.previous,
    timestamps: selection.timestamps !== false,
    clusterName: selectedClusterName.value
  }
  emit('update:modelValue', payload)
  emit('change', payload)
}

const applyModelValue = value => {
  syncingFromParent.value = true
  selection.clusterId = value?.clusterId || ''
  selection.namespace = value?.namespace || ''
  selection.pod = value?.pod || ''
  selection.container = value?.container || ''
  selection.previous = !!value?.previous
  selection.timestamps = value?.timestamps !== false
  syncingFromParent.value = false
}

const loadClusters = async (force = false) => {
  if (clustersLoading.value) return
  if (clusterOptions.value.length && !force) return

  clustersLoading.value = true
  try {
    const response = await k8sApi.getClusterList({ page: 1, size: 100, pageSize: 100 })
    const clusters = toList(getPayload(response, '获取 K8s 集群失败'))
    clusterOptions.value = clusters.filter(cluster => cluster?.id !== undefined && cluster?.id !== null)

    if (!selection.clusterId && clusterOptions.value.length) {
      const onlineCluster = clusterOptions.value.find(cluster => cluster.status === 2)
      selection.clusterId = (onlineCluster || clusterOptions.value[0]).id
      await handleClusterChange(selection.clusterId)
    } else {
      emitSelection()
    }
  } catch (error) {
    console.error('load k8s clusters failed:', error)
    ElMessage.error(error.message || '获取 K8s 集群失败')
  } finally {
    clustersLoading.value = false
  }
}

const loadNamespaces = async (force = false) => {
  if (!selection.clusterId) {
    namespaceOptions.value = []
    return
  }
  if (namespaceOptions.value.length && !force) return

  namespacesLoading.value = true
  try {
    const response = await k8sApi.getNamespaces(selection.clusterId)
    namespaceOptions.value = toList(getPayload(response, '获取命名空间失败'))
      .map(normalizeNamespace)
      .filter(namespace => namespace.name)

    if (selection.namespace && !namespaceOptions.value.some(namespace => namespace.name === selection.namespace)) {
      selection.namespace = ''
    }
    if (!selection.namespace && namespaceOptions.value.length) {
      const defaultNamespace = namespaceOptions.value.find(namespace => namespace.name === 'default')
      selection.namespace = (defaultNamespace || namespaceOptions.value[0]).name
      await handleNamespaceChange(selection.namespace)
    } else {
      emitSelection()
    }
  } catch (error) {
    console.error('load k8s namespaces failed:', error)
    ElMessage.error(error.message || '获取命名空间失败')
  } finally {
    namespacesLoading.value = false
  }
}

const loadPods = async (force = false) => {
  if (!selection.clusterId || !selection.namespace) {
    podOptions.value = []
    return
  }
  if (podOptions.value.length && !force) return

  podsLoading.value = true
  try {
    const response = await k8sApi.getPodList(selection.clusterId, selection.namespace, { page: 1, pageSize: 200 })
    podOptions.value = toList(getPayload(response, '获取 Pod 列表失败'))
      .map(normalizePod)
      .filter(pod => pod.name)

    if (selection.pod && !podOptions.value.some(pod => pod.name === selection.pod)) {
      selection.pod = ''
      selection.container = ''
    }
    emitSelection()
  } catch (error) {
    console.error('load k8s pods failed:', error)
    ElMessage.error(error.message || '获取 Pod 列表失败')
  } finally {
    podsLoading.value = false
  }
}

const loadContainers = async (force = false) => {
  if (!selection.clusterId || !selection.namespace || !selection.pod) {
    containerOptions.value = []
    return
  }
  if (containerOptions.value.length && !force) return

  containersLoading.value = true
  try {
    const response = await k8sApi.getPodContainers(selection.clusterId, selection.namespace, selection.pod)
    containerOptions.value = toList(getPayload(response, '获取容器列表失败'))
      .map(normalizeContainer)
      .filter(container => container.name)

    if (selection.container && !containerOptions.value.some(container => container.name === selection.container)) {
      selection.container = ''
    }
    emitSelection()
  } catch (error) {
    console.error('load k8s containers failed:', error)
    ElMessage.warning(error.message || '获取容器列表失败，将使用默认业务容器')
    emitSelection()
  } finally {
    containersLoading.value = false
  }
}

const handleClusterChange = async () => {
  selection.namespace = ''
  selection.pod = ''
  selection.container = ''
  namespaceOptions.value = []
  podOptions.value = []
  containerOptions.value = []
  emitSelection()
  await loadNamespaces(true)
}

const handleNamespaceChange = async () => {
  selection.pod = ''
  selection.container = ''
  podOptions.value = []
  containerOptions.value = []
  emitSelection()
  await loadPods(true)
}

const handlePodChange = async () => {
  selection.container = ''
  containerOptions.value = []
  emitSelection()
  await loadContainers(true)
}

const focusClusterSelect = () => {
  loadClusters(true)
}

watch(
  () => props.modelValue,
  value => {
    applyModelValue(value || {})
  },
  { deep: true, immediate: true }
)

watch(
  () => selection.clusterId,
  clusterId => {
    if (!clusterId || syncingFromParent.value) return
    if (!namespaceOptions.value.length) loadNamespaces()
  }
)

watch(
  () => selection.namespace,
  namespace => {
    if (!namespace || syncingFromParent.value) return
    if (!podOptions.value.length) loadPods()
  }
)

watch(
  () => selection.pod,
  pod => {
    if (!pod || syncingFromParent.value) return
    if (!containerOptions.value.length) loadContainers()
  }
)

onMounted(() => {
  loadClusters()
})
</script>

<style scoped>
.k8s-log-source-panel {
  display: grid;
  grid-template-rows: auto auto minmax(0, 1fr);
  gap: 10px;
  min-height: 0;
  height: 100%;
}

.k8s-tools {
  display: flex;
  gap: 8px;
}

.k8s-tools .el-select {
  flex: 1;
}

.cluster-option,
.pod-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.cluster-option small,
.pod-option small {
  color: var(--log-muted, #6b778a);
  font-size: 11px;
}

.k8s-cascade {
  padding: 10px;
  border: 1px solid var(--log-border-soft, rgba(223, 231, 242, 0.72));
  border-radius: 8px;
  background: var(--log-panel-soft, #f8fafc);
}

.k8s-cascade :deep(.el-form-item) {
  margin-bottom: 8px;
}

.k8s-cascade .el-button {
  width: 100%;
}

.k8s-options {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 12px;
  margin-bottom: 10px;
}

.k8s-source-scroll {
  min-height: 0;
  border: 1px solid var(--log-border-soft, rgba(223, 231, 242, 0.72));
  border-radius: 8px;
  background: var(--log-panel-soft, #f8fafc);
}

.source-summary {
  display: grid;
  justify-items: center;
  gap: 8px;
  margin: 8px;
  padding: 18px 12px;
  border: 1px dashed var(--log-border, #dfe7f2);
  border-radius: 8px;
  background: #fff;
  color: var(--log-muted, #6b778a);
  text-align: center;
}

.source-summary.ready {
  border-style: solid;
  border-color: rgba(76, 139, 245, 0.36);
  background: #edf4ff;
  color: #1f4e94;
}

.source-summary .el-icon {
  color: var(--log-brand, #4c8bf5);
  font-size: 28px;
}

.source-summary strong,
.source-summary span {
  display: block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
}

.source-summary span {
  font-size: 12px;
  line-height: 1.6;
}

.selection-list {
  display: grid;
  gap: 8px;
  padding: 0 8px 8px;
}

.selection-list button {
  display: grid;
  grid-template-columns: 70px minmax(0, 1fr);
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 9px 10px;
  border: 1px solid var(--log-border-soft, rgba(223, 231, 242, 0.72));
  border-radius: 8px;
  background: #fff;
  color: inherit;
  cursor: pointer;
  text-align: left;
}

.selection-list button:hover,
.selection-list button.active {
  border-color: rgba(76, 139, 245, 0.5);
  background: #edf4ff;
}

.selection-list span {
  color: var(--log-muted, #6b778a);
  font-size: 12px;
}

.selection-list strong {
  overflow: hidden;
  color: var(--log-text, #172133);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
