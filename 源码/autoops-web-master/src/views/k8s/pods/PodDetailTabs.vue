<script setup>
/* global defineProps, defineEmits, defineExpose */
import { computed, nextTick, ref, watch } from 'vue'
import { Search, Refresh, Document, ArrowUp, ArrowDown } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import CodeEditor from '@/components/CodeEditor.vue'
import k8sApi from '@/api/k8s'
import PodYamlDialog from './PodYamlDialog.vue'
import { getYamlContentFromApiResponse } from '../utils/yaml'

const props = defineProps({
  modelValue: {
    type: String,
    default: 'containers'
  },
  clusterId: {
    type: [String, Number],
    required: true
  },
  namespace: {
    type: String,
    default: ''
  },
  loading: {
    type: Boolean,
    default: false
  },
  allRelatedPods: {
    type: Array,
    default: () => []
  },
  historyVersions: {
    type: Array,
    default: () => []
  },
  events: {
    type: Array,
    default: () => []
  },
  relatedResources: {
    type: Object,
    default: () => ({
      network: { services: [], ingresses: [] },
      storage: { pvcs: [], pvs: [], storageClasses: [] },
      config: { configMaps: [], secrets: [] },
      warnings: []
    })
  },
  relatedResourcesLoading: {
    type: Object,
    default: () => ({
      network: false,
      storage: false,
      config: false
    })
  },
  selectedContainerForLogs: {
    type: String,
    default: ''
  },
  selectedLogContainer: {
    type: String,
    default: ''
  },
  followLogs: {
    type: Boolean,
    default: false
  },
  autoScroll: {
    type: Boolean,
    default: true
  },
  showPreviousLogs: {
    type: Boolean,
    default: false
  },
  logsSearchText: {
    type: String,
    default: ''
  },
  logsSearchResults: {
    type: Array,
    default: () => []
  },
  logsSearchCurrentIndex: {
    type: Number,
    default: -1
  },
  logsLoading: {
    type: Boolean,
    default: false
  },
  currentLogs: {
    type: String,
    default: ''
  },
  logsUpdateKey: {
    type: [String, Number],
    default: 0
  },
  maxLogLines: {
    type: Number,
    default: 500
  },
  lastLogRefreshTime: {
    type: [String, Number, Date, Object],
    default: null
  },
  wsStatus: {
    type: String,
    default: 'disconnected'
  },
  getPodStatusTag: {
    type: Function,
    required: true
  },
  getPodStatusText: {
    type: Function,
    required: true
  },
  getStatusType: {
    type: Function,
    required: true
  },
  formatRunningTime: {
    type: Function,
    required: true
  },
  formatTime: {
    type: Function,
    required: true
  }
})

const emit = defineEmits([
  'update:modelValue',
  'update:selectedContainerForLogs',
  'update:selectedLogContainer',
  'update:followLogs',
  'update:autoScroll',
  'update:showPreviousLogs',
  'update:logsSearchText',
  'update:currentLogs',
  'tab-change',
  'show-monitoring',
  'view-yaml',
  'view-logs',
  'terminal',
  'file-manager',
  'ai-analyze',
  'delete',
  'rollback',
  'refresh-history',
  'refresh-events',
  'show-pod-detail-events',
  'container-change',
  'log-container-change',
  'search-logs',
  'clear-logs-search',
  'previous-logs-search-result',
  'next-logs-search-result',
  'refresh-logs',
  'download-logs',
  'refresh-related-resources'
])

const logsEditorRef = ref(null)
const tabsSectionRef = ref(null)
const resourceRenderKey = ref(0)
const activeNetworkResourceTab = ref('service')
const activeStorageResourceTab = ref('pvc')
const activeConfigResourceTab = ref('configmap')
const serviceTableRef = ref(null)
const ingressTableRef = ref(null)
const pvcTableRef = ref(null)
const pvTableRef = ref(null)
const storageClassTableRef = ref(null)
const configMapTableRef = ref(null)
const secretTableRef = ref(null)
const resourceYamlDialogVisible = ref(false)
const currentResourceYaml = ref('')
const currentResourceYamlInfo = ref({
  name: '',
  type: ''
})

const LOG_THEME_STORAGE_KEY = 'k8s-pod-log-theme'
const logThemeOptions = [
  {
    label: '清爽浅色',
    value: 'aiops-log-light',
    background: '#f8fafc',
    header: '#f1f5f9',
    border: '#dfe7f2',
    foreground: '#334155',
    accent: '#2563eb',
    muted: '#64748b'
  },
  {
    label: '经典深色',
    value: 'aiops-log-dark',
    background: '#111827',
    header: '#172033',
    border: '#334155',
    foreground: '#d1d5db',
    accent: '#60a5fa',
    muted: '#94a3b8'
  },
  {
    label: 'Agnoster',
    value: 'aiops-log-agnoster',
    background: '#1f2430',
    header: '#292f3d',
    border: '#3f4a5a',
    foreground: '#e6e6e6',
    accent: '#5ccfe6',
    muted: '#a7adba'
  },
  {
    label: 'Robbyrussell',
    value: 'aiops-log-robbyrussell',
    background: '#272822',
    header: '#30312b',
    border: '#49483e',
    foreground: '#f8f8f2',
    accent: '#a6e22e',
    muted: '#b8b8ad'
  },
  {
    label: 'Solarized Dark',
    value: 'aiops-log-solarized',
    background: '#002b36',
    header: '#073642',
    border: '#34515a',
    foreground: '#93a1a1',
    accent: '#2aa198',
    muted: '#839496'
  }
]

const getInitialLogTheme = () => {
  if (typeof window === 'undefined') return logThemeOptions[0].value

  const storedTheme = window.localStorage.getItem(LOG_THEME_STORAGE_KEY)
  return logThemeOptions.some(option => option.value === storedTheme)
    ? storedTheme
    : logThemeOptions[0].value
}

const selectedLogTheme = ref(getInitialLogTheme())
const selectedLogThemeOption = computed(() => (
  logThemeOptions.find(option => option.value === selectedLogTheme.value) || logThemeOptions[0]
))
const logThemeStyle = computed(() => {
  const theme = selectedLogThemeOption.value
  return {
    '--log-background': theme.background,
    '--log-header-background': theme.header,
    '--log-border': theme.border,
    '--log-foreground': theme.foreground,
    '--log-accent': theme.accent,
    '--log-muted': theme.muted
  }
})

watch(selectedLogTheme, value => {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(LOG_THEME_STORAGE_KEY, value)
  }
})

const networkTabs = [
  { label: 'Service网络', name: 'service' },
  { label: 'Ingress入口', name: 'ingress' }
]

const storageTabs = [
  { label: 'PVC', name: 'pvc' },
  { label: 'PV', name: 'pv' },
  { label: 'StorageClass', name: 'storageclass' }
]

const configTabs = [
  { label: 'ConfigMap', name: 'configmap' },
  { label: 'Secret', name: 'secret' }
]

const networkServices = computed(() => props.relatedResources?.network?.services || [])
const networkIngresses = computed(() => props.relatedResources?.network?.ingresses || [])
const storagePvcs = computed(() => props.relatedResources?.storage?.pvcs || [])
const storagePvs = computed(() => props.relatedResources?.storage?.pvs || [])
const storageClasses = computed(() => props.relatedResources?.storage?.storageClasses || [])
const configMaps = computed(() => props.relatedResources?.config?.configMaps || [])
const secrets = computed(() => props.relatedResources?.config?.secrets || [])
const relatedWarnings = computed(() => props.relatedResources?.warnings || [])

const formatMap = value => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return value || '-'
  return Object.entries(value)
    .map(([key, item]) => `${key}=${item}`)
    .join(', ') || '-'
}

const formatList = value => {
  if (Array.isArray(value)) {
    return value
      .filter(Boolean)
      .map(item => {
        if (!item || typeof item !== 'object') return item
        return formatMap(item)
      })
      .join(', ') || '-'
  }
  if (value && typeof value === 'object') return formatMap(value)
  return value || '-'
}

const getPodResourceValue = (value, key) => {
  if (!value || typeof value !== 'object') return '-'
  return value[key] || '-'
}

const formatKeyCount = value => {
  if (typeof value === 'number') return value
  if (Array.isArray(value)) return value.length
  if (value && typeof value === 'object') return Object.keys(value).length
  return value || 0
}

const getResourceName = row => row?.name || row?.metadata?.name || '-'
const getResourceNamespace = row => row?.namespace || row?.metadata?.namespace || '-'
const getResourceAge = row => row?.age || row?.createdAt || row?.creationTimestamp || row?.metadata?.creationTimestamp || '-'
const getServicePorts = row => {
  const ports = row?.ports || row?.spec?.ports || []
  if (!Array.isArray(ports) || ports.length === 0) return '-'
  return ports.map(port => {
    const portName = port.name ? `${port.name}: ` : ''
    const target = port.targetPort ? ` -> ${port.targetPort}` : ''
    const nodePort = port.nodePort ? ` / NodePort ${port.nodePort}` : ''
    const protocol = port.protocol || 'TCP'
    return `${portName}${port.port || '-'}${target}${nodePort} (${protocol})`
  }).join(', ')
}
const getServiceEndpoints = row => {
  const endpoints = row?.endpoints || []
  if (!Array.isArray(endpoints) || endpoints.length === 0) return '-'
  return endpoints.map(endpoint => {
    const ready = endpoint.ready === false ? 'NotReady' : 'Ready'
    const nodeName = endpoint.nodeName ? `@${endpoint.nodeName}` : ''
    const ports = Array.isArray(endpoint.ports) && endpoint.ports.length
      ? `:${endpoint.ports.map(port => port.port || port.name).filter(Boolean).join('/')}`
      : ''
    return `${endpoint.ip || '-'}${ports}${nodeName} (${ready})`
  }).join(', ')
}
const getIngressHosts = row => formatList(row?.hosts || row?.rules?.map(rule => rule.host) || row?.spec?.rules?.map(rule => rule.host))
const getIngressServices = row => formatList(row?.services || row?.backendServices || row?.backends)
const getIngressTlsSecrets = row => formatList(row?.tlsSecrets || row?.tlsSecretNames || row?.spec?.tls?.map(item => item.secretName))
const getPvcCapacity = row => row?.capacity || row?.storage || row?.resources?.requests?.storage || row?.spec?.resources?.requests?.storage || '-'
const getPvcVolume = row => row?.volumeName || row?.spec?.volumeName || '-'
const getPvCapacity = row => row?.capacity || row?.spec?.capacity?.storage || '-'
const getPvClaim = row => row?.claim || row?.claimRef || (row?.spec?.claimRef ? `${row.spec.claimRef.namespace || '-'}/${row.spec.claimRef.name || '-'}` : '-')
const getStorageClassProvisioner = row => row?.provisioner || row?.metadata?.provisioner || '-'
const getConfigKeys = row => formatKeyCount(row?.keys || row?.data || row?.binaryData)
const getSecretType = row => row?.type || row?.metadata?.type || '-'

const resourceYamlConfigs = {
  Service: {
    label: 'Service',
    kind: 'Service',
    apiVersion: 'v1',
    namespaced: true,
    fetchYaml: k8sApi.getServiceYaml
  },
  Ingress: {
    label: 'Ingress',
    kind: 'Ingress',
    apiVersion: 'networking.k8s.io/v1',
    namespaced: true,
    fetchYaml: k8sApi.getIngressYaml
  },
  PVC: {
    label: 'PVC',
    kind: 'PersistentVolumeClaim',
    apiVersion: 'v1',
    namespaced: true,
    fetchYaml: k8sApi.getPVCYaml
  },
  PV: {
    label: 'PV',
    kind: 'PersistentVolume',
    apiVersion: 'v1',
    namespaced: false,
    fetchYaml: k8sApi.getPVYaml
  },
  StorageClass: {
    label: 'StorageClass',
    kind: 'StorageClass',
    apiVersion: 'storage.k8s.io/v1',
    namespaced: false,
    fetchYaml: k8sApi.getStorageClassYaml
  },
  ConfigMap: {
    label: 'ConfigMap',
    kind: 'ConfigMap',
    apiVersion: 'v1',
    namespaced: true,
    fetchYaml: k8sApi.getConfigMapYaml
  },
  Secret: {
    label: 'Secret',
    kind: 'Secret',
    apiVersion: 'v1',
    namespaced: true,
    fetchYaml: k8sApi.getSecretYaml
  }
}

const getYamlNamespace = row => {
  const resourceNamespace = getResourceNamespace(row)
  return resourceNamespace && resourceNamespace !== '-' ? resourceNamespace : props.namespace
}

const buildFallbackYaml = (config, name, namespace) => {
  const namespaceLine = config.namespaced ? `\n  namespace: ${namespace}` : ''
  return `# ${config.label} ${name} YAML\napiVersion: ${config.apiVersion}\nkind: ${config.kind}\nmetadata:\n  name: ${name}${namespaceLine}`
}

const openResourceYaml = async (resourceType, row) => {
  const config = resourceYamlConfigs[resourceType]
  const name = getResourceName(row)
  const namespace = getYamlNamespace(row)

  if (!config || !props.clusterId || !name || name === '-') {
    ElMessage.warning('缺少资源信息，无法查看 YAML')
    return
  }

  if (config.namespaced && !namespace) {
    ElMessage.warning('缺少命名空间，无法查看 YAML')
    return
  }

  try {
    const response = config.namespaced
      ? await config.fetchYaml(props.clusterId, namespace, name)
      : await config.fetchYaml(props.clusterId, name)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      currentResourceYaml.value = getYamlContentFromApiResponse(
        responseData,
        buildFallbackYaml(config, name, namespace)
      )
      currentResourceYamlInfo.value = {
        name,
        type: config.label
      }
      resourceYamlDialogVisible.value = true
    } else {
      ElMessage.error(responseData.message || `获取 ${config.label} YAML 失败`)
    }
  } catch (error) {
    console.error(`获取 ${config.label} YAML 失败:`, error)
    ElMessage.error(`获取 ${config.label} YAML 失败`)
  }
}

const refreshResourceTableLayout = () => {
  nextTick(() => {
    requestAnimationFrame(() => {
      [
        serviceTableRef,
        ingressTableRef,
        pvcTableRef,
        pvTableRef,
        storageClassTableRef,
        configMapTableRef,
        secretTableRef
      ].forEach(tableRef => {
        tableRef.value?.doLayout?.()
      })
    })
  })
}

watch(
  () => [
    props.relatedResourcesLoading.network,
    props.relatedResourcesLoading.storage,
    props.relatedResourcesLoading.config,
    networkServices.value.length,
    networkIngresses.value.length,
    storagePvcs.value.length,
    storagePvs.value.length,
    storageClasses.value.length,
    configMaps.value.length,
    secrets.value.length
  ],
  () => {
    nextTick(() => {
      resourceRenderKey.value += 1
      refreshResourceTableLayout()
    })
  },
  { flush: 'post' }
)

watch(
  () => props.modelValue,
  value => {
    if (['network', 'storage', 'config'].includes(value)) {
      refreshResourceTableLayout()
    }
  },
  { flush: 'post' }
)

const activeTabModel = computed({
  get: () => props.modelValue,
  set: value => emit('update:modelValue', value)
})

const selectedContainerModel = computed({
  get: () => props.selectedContainerForLogs,
  set: value => emit('update:selectedContainerForLogs', value)
})

const selectedLogContainerModel = computed({
  get: () => props.selectedLogContainer,
  set: value => emit('update:selectedLogContainer', value)
})

const logContainerOptions = computed(() => {
  const selectedPod = props.allRelatedPods.find(pod => pod.name === props.selectedContainerForLogs)
  const containers = selectedPod?.containers?.length
    ? selectedPod.containers
    : (selectedPod?.rawData?.spec?.containers || [])

  return [...new Set(containers
    .map(container => (typeof container === 'string' ? container : container?.name))
    .filter(Boolean))]
})

const getHistoryRowImages = row => {
  const items = []

  const addItem = item => {
    if (!item) return

    if (typeof item === 'string') {
      const image = item.trim()
      if (image) items.push({ name: '', image })
      return
    }

    if (Array.isArray(item)) {
      item.forEach(addItem)
      return
    }

    if (typeof item !== 'object') return

    const image = String(item.image || item.imageName || item.containerImage || item.currentImage || '').trim()
    if (!image) return

    items.push({
      name: String(item.name || item.containerName || item.container?.name || '').trim(),
      image
    })
  }

  if (Array.isArray(row?.containers) && row.containers.length) {
    addItem(row.containers)
  } else if (Array.isArray(row?.images) && row.images.length) {
    addItem(row.images)
  } else if (row?.image) {
    String(row.image)
      .split(',')
      .map(image => image.trim())
      .filter(Boolean)
      .forEach(image => addItem(image))
  }

  const seen = new Set()
  return items.filter((item, index) => {
    const key = `${item.name || index}:${item.image}`
    if (seen.has(key)) return false
    seen.add(key)
    return true
  })
}

const formatHistoryImageLabel = item => item?.image

const followLogsModel = computed({
  get: () => props.followLogs,
  set: value => emit('update:followLogs', value)
})

const autoScrollModel = computed({
  get: () => props.autoScroll,
  set: value => emit('update:autoScroll', value)
})

const showPreviousLogsModel = computed({
  get: () => props.showPreviousLogs,
  set: value => emit('update:showPreviousLogs', value)
})

const logsSearchTextModel = computed({
  get: () => props.logsSearchText,
  set: value => emit('update:logsSearchText', value)
})

const currentLogsModel = computed({
  get: () => props.currentLogs,
  set: value => emit('update:currentLogs', value)
})

const handleTabChange = tabName => {
  emit('tab-change', tabName)
}

const handleContainerChange = podName => {
  emit('container-change', podName)
}

const handleLogContainerChange = containerName => {
  emit('log-container-change', containerName)
}

const systemPodNamespaces = new Set([
  'kube-system',
  'kube-public',
  'kube-node-lease',
  'calico-system',
  'tigera-operator'
])
const systemPodPrefixes = [
  'kube-',
  'calico-',
  'coredns',
  'metrics-server',
  'node-local-dns',
  'kubernetes-dashboard'
]
const isProtectedPod = row => {
  const namespace = row?.namespace || props.namespace
  const podName = String(row?.name || '')
  return systemPodNamespaces.has(namespace) || systemPodPrefixes.some(prefix => podName.startsWith(prefix))
}

defineExpose({
  scrollToBottom: (...args) => logsEditorRef.value?.scrollToBottom?.(...args),
  scrollToLine: (...args) => logsEditorRef.value?.scrollToLine?.(...args),
  scrollIntoView: options => tabsSectionRef.value?.scrollIntoView(options)
})
</script>

<template>
  <div ref="tabsSectionRef" class="tabs-section">
    <el-tabs v-model="activeTabModel" class="detail-tabs" @tab-change="handleTabChange">
      <el-tab-pane label="容器组" name="containers">
        <el-table
          class="detail-table"
          :data="allRelatedPods"
          v-loading="loading"
          stripe
          style="width: 100%"
          size="small"
        >
          <el-table-column prop="name" label="Pod名称" min-width="200">
            <template #default="{ row }">
              <div class="pod-name-container">
                <img src="@/assets/image/k8s.svg" alt="k8s" class="k8s-icon" width="16" height="16" />
                <span class="pod-name">{{ row.name }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="status" label="状态" min-width="100">
            <template #default="{ row }">
              <el-tag :type="getPodStatusTag(row.status)" size="small">
                {{ getPodStatusText(row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="restartCount" label="重启次数" min-width="100">
            <template #default="{ row }">
              <el-tag size="small" :type="row.restartCount > 0 ? 'warning' : 'success'">
                {{ row.restartCount }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="资源限制" min-width="140">
            <template #default="{ row }">
              <div class="pod-resource-cell">
                <span>CPU: {{ getPodResourceValue(row.resourceLimits, 'cpu') }}</span>
                <span>内存: {{ getPodResourceValue(row.resourceLimits, 'memory') }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="资源实际使用" min-width="160">
            <template #default="{ row }">
              <div class="pod-resource-cell">
                <span>CPU: {{ getPodResourceValue(row.currentUsage, 'cpu') }}</span>
                <span>内存: {{ getPodResourceValue(row.currentUsage, 'memory') }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="nodeName" label="节点" min-width="150" />
          <el-table-column prop="podIP" label="Pod IP" min-width="120" />
          <el-table-column prop="age" label="运行时间" min-width="100">
            <template #default="{ row }">
              <el-tag size="small" type="info">
                {{ formatRunningTime(row.runningTime || row.age) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" min-width="160" align="left" header-align="left">
            <template #default="{ row }">
              <div class="operation-buttons-container">
                <el-button link size="small" class="operation-text logs"  @click="emit('view-logs', row)">日志</el-button>
                <el-button link size="small" class="operation-text terminal"  @click="emit('terminal', row)">终端</el-button>
                <el-dropdown trigger="click">
                  <el-button link size="small" class="operation-text more">
                    更多<el-icon class="operation-more-icon"><ArrowDown /></el-icon>
                  </el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item @click="emit('view-yaml', row)">YAML</el-dropdown-item>
                      <el-dropdown-item
                        :disabled="isProtectedPod(row)"
                        @click="emit('file-manager', row)"
                      >文件</el-dropdown-item>
                      <el-dropdown-item @click="emit('ai-analyze', row)">AI诊断</el-dropdown-item>
                      <el-dropdown-item
                        :disabled="isProtectedPod(row)"
                        divided
                        @click="emit('delete', row)"
                      >删除</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <el-tab-pane label="历史版本" name="history">
        <el-table
          class="detail-table"
          :data="historyVersions"
          stripe
          style="width: 100%"
          size="small"
          v-loading="loading"
        >
          <el-table-column prop="revision" label="版本" min-width="80" />
          <el-table-column prop="description" label="描述" min-width="250" />
          <el-table-column prop="status" label="状态" min-width="100">
            <template #default="{ row }">
              <el-tag :type="getStatusType(row.status)" effect="dark" size="small">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="镜像" min-width="260">
            <template #default="{ row }">
              <div class="image-cell">
                <template v-if="getHistoryRowImages(row).length">
                  <el-tooltip
                    v-for="(item, index) in getHistoryRowImages(row)"
                    :key="`${item.name || index}-${item.image}`"
                    :content="formatHistoryImageLabel(item)"
                    placement="top"
                  >
                    <el-tag size="small" class="history-image-tag">
                      {{ formatHistoryImageLabel(item) }}
                    </el-tag>
                  </el-tooltip>
                </template>
                <span v-else>-</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="replicas" label="副本数" min-width="100" />
          <el-table-column prop="creationTime" label="创建时间" min-width="180">
            <template #default="{ row }">
              {{ formatTime(row.creationTime) }}
            </template>
          </el-table-column>
          <el-table-column prop="changeDescription" label="变更描述" min-width="200">
            <template #default="{ row }">
              <span v-if="row.changeDescription">{{ row.changeDescription }}</span>
              <span v-else>-</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" min-width="120">
            <template #default="{ row }">
              <div class="operation-buttons-container">
                <el-button link size="small" class="operation-text rollback"  @click="emit('rollback', row)">回滚</el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
        <div v-if="!loading && historyVersions.length === 0" class="empty-panel">
          <el-empty description="暂无历史版本数据">
            <div class="empty-actions">
              <el-button type="primary" @click="emit('refresh-history')">重新获取历史版本</el-button>
            </div>
          </el-empty>
        </div>
      </el-tab-pane>

      <el-tab-pane label="事件" name="events">
        <el-table
          class="detail-table"
          :data="events"
          stripe
          style="width: 100%"
          size="small"
          v-loading="loading"
        >
          <el-table-column prop="type" label="事件类型" min-width="100">
            <template #default="{ row }">
              <el-tag :type="row.type === 'Warning' ? 'warning' : 'success'" effect="dark" size="small">
                {{ row.type }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="来源" min-width="150">
            <template #default="{ row }">
              <span>{{ row.source || '未知' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="次数" min-width="80">
            <template #default="{ row }">
              <el-tag size="small" type="info">{{ row.count || 1 }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="reason" label="原因" min-width="150" />
          <el-table-column prop="message" label="消息" min-width="300" />
          <el-table-column label="最后时间" min-width="180">
            <template #default="{ row }">
              {{ formatTime(row.lastTime || row.firstTime) }}
            </template>
          </el-table-column>
        </el-table>
        <div v-if="!loading && events.length === 0" class="empty-panel">
          <el-empty description="暂无事件数据">
            <div class="empty-actions">
              <el-button type="primary" @click="emit('refresh-events')">重新获取事件</el-button>
              <el-button type="success" @click="emit('show-pod-detail-events')">显示Pod详情事件</el-button>
            </div>
          </el-empty>
        </div>
      </el-tab-pane>

      <el-tab-pane label="日志" name="logs">
        <div class="logs-tab-content">
          <div class="logs-controls">
            <el-row :gutter="8" class="logs-controls-row">
              <el-col :span="5">
                <el-select v-model="selectedContainerModel" placeholder="选择Pod" style="width: 100%;" @change="handleContainerChange" size="small">
                  <el-option
                    v-for="pod in allRelatedPods"
                    :key="pod.name"
                    :label="pod.name"
                    :value="pod.name"
                  />
                </el-select>
              </el-col>
              <el-col :span="4">
                <el-select
                  v-model="selectedLogContainerModel"
                  placeholder="选择容器"
                  style="width: 100%;"
                  :disabled="!selectedContainerForLogs || logContainerOptions.length === 0"
                  @change="handleLogContainerChange"
                  size="small"
                >
                  <el-option
                    v-for="containerName in logContainerOptions"
                    :key="containerName"
                    :label="containerName"
                    :value="containerName"
                  />
                </el-select>
              </el-col>
              <el-col :span="5">
                <div class="logs-inline-controls">
                  <span class="logs-control-label">实时</span>
                  <el-switch v-model="followLogsModel" size="small" />
                  <el-checkbox v-model="autoScrollModel" :disabled="!followLogs" size="small">滚动</el-checkbox>
                </div>
              </el-col>
              <el-col :span="5">
                <el-checkbox v-model="showPreviousLogsModel" :disabled="followLogs" size="small">上个容器</el-checkbox>
              </el-col>
            </el-row>
            <div class="logs-actions">
              <el-select
                v-model="selectedLogTheme"
                class="log-theme-select"
                size="small"
                title="切换日志主题"
              >
                <el-option
                  v-for="theme in logThemeOptions"
                  :key="theme.value"
                  :label="theme.label"
                  :value="theme.value"
                >
                  <span class="log-theme-option">
                    <span
                      class="log-theme-swatch"
                      :style="{ backgroundColor: theme.background, borderColor: theme.accent }"
                    ></span>
                    <span>{{ theme.label }}</span>
                  </span>
                </el-option>
              </el-select>
              <el-input
                v-model="logsSearchTextModel"
                placeholder="搜索..."
                :prefix-icon="Search"
                clearable
                @keyup.enter="emit('search-logs')"
                @clear="emit('clear-logs-search')"
                size="small"
                style="width: 150px;"
              />
              <el-button-group size="small">
                <el-button type="warning" :icon="Search" @click="emit('search-logs')" />
                <el-button type="success" :icon="ArrowUp" @click="emit('previous-logs-search-result')" :disabled="logsSearchResults.length === 0" />
                <el-button type="primary" :icon="ArrowDown" @click="emit('next-logs-search-result')" :disabled="logsSearchResults.length === 0" />
              </el-button-group>
              <el-text v-if="logsSearchResults.length > 0" type="info" size="small">
                {{ logsSearchCurrentIndex + 1 }}/{{ logsSearchResults.length }}
              </el-text>
              <el-button size="small" type="primary" :icon="Refresh" @click="emit('refresh-logs')" :loading="logsLoading">刷新</el-button>
              <el-button size="small" type="success" :icon="Document" @click="emit('download-logs')">下载</el-button>
            </div>
          </div>

          <div class="logs-display" :style="logThemeStyle" v-loading="logsLoading">
            <div v-if="!selectedContainerForLogs || !selectedLogContainer" class="logs-placeholder">
              <el-empty :description="selectedContainerForLogs ? '请选择容器查看日志' : '请选择Pod查看日志'">
                <div class="empty-actions">
                  <el-text type="info">选择上方 Pod 和容器后开始查看日志</el-text>
                </div>
              </el-empty>
            </div>
            <div v-else-if="currentLogs" class="logs-content-display">
              <div class="logs-header">
                <span class="container-name">{{ selectedContainerForLogs }} / {{ selectedLogContainer }}</span>
                <span class="logs-info">
                  当前 {{ currentLogs.split('\n').length }} 行
                  <template v-if="followLogs && maxLogLines < 999999">
                    (最大保留 {{ maxLogLines }} 行)
                  </template>
                  • {{ formatTime(lastLogRefreshTime) }}
                </span>
                <span v-if="followLogs" class="ws-status">
                  <el-tag v-if="wsStatus === 'connected'" type="success" size="small" effect="dark">
                    <i class="el-icon-connection"></i> 实时连接中
                  </el-tag>
                  <el-tag v-else-if="wsStatus === 'connecting'" type="warning" size="small" effect="dark">
                    <i class="el-icon-loading"></i> 连接中...
                  </el-tag>
                  <el-tag v-else-if="wsStatus === 'error'" type="danger" size="small" effect="dark">
                    <i class="el-icon-warning"></i> 连接失败
                  </el-tag>
                  <el-tag v-else type="info" size="small" effect="dark">
                    <i class="el-icon-connection"></i> 未连接
                  </el-tag>
                </span>
              </div>

              <div class="logs-editor-container">
                <CodeEditor
                  :key="`log-${logsUpdateKey}`"
                  ref="logsEditorRef"
                  v-model="currentLogsModel"
                  :language="null"
                  :theme="selectedLogTheme"
                  wordWrap="on"
                  height="600px"
                  fontSize="12px"
                  :readonly="true"
                  :searchText="logsSearchText"
                  :searchResults="logsSearchResults"
                  :currentSearchIndex="logsSearchCurrentIndex"
                />
              </div>
            </div>
            <div v-else class="logs-empty">
              <el-empty description="该Pod暂无日志数据">
                <div class="empty-actions">
                  <el-button type="primary" @click="emit('refresh-logs')">重新获取</el-button>
                </div>
              </el-empty>
            </div>
          </div>
        </div>
      </el-tab-pane>

      <el-tab-pane label="网络" name="network">
        <div class="resource-section">
          <el-alert
            v-if="relatedWarnings.length"
            class="resource-warning"
            type="warning"
            :closable="false"
            show-icon
          >
            <template #title>{{ relatedWarnings.join('；') }}</template>
          </el-alert>
          <el-tabs v-model="activeNetworkResourceTab" class="resource-tabs" @tab-change="refreshResourceTableLayout">
            <el-tab-pane :label="networkTabs[0].label" :name="networkTabs[0].name">
              <div class="resource-toolbar">
                <el-button :icon="Refresh" size="small" :loading="relatedResourcesLoading.network" @click="emit('refresh-related-resources', 'network')" />
              </div>
              <el-table
                :key="`service-${resourceRenderKey}`"
                ref="serviceTableRef"
                class="resource-table"
                :data="networkServices"
                v-loading="relatedResourcesLoading.network"
                stripe
                size="small"
                style="width: 100%"
              >
                <el-table-column label="名称" min-width="180">
                  <template #default="{ row }">{{ getResourceName(row) }}</template>
                </el-table-column>
                <el-table-column label="命名空间" min-width="140">
                  <template #default="{ row }">{{ getResourceNamespace(row) }}</template>
                </el-table-column>
                <el-table-column label="类型" min-width="120">
                  <template #default="{ row }">{{ row.type || row.spec?.type || '-' }}</template>
                </el-table-column>
                <el-table-column label="Cluster IP" min-width="140">
                  <template #default="{ row }">{{ row.clusterIP || row.spec?.clusterIP || '-' }}</template>
                </el-table-column>
                <el-table-column label="端口" min-width="220">
                  <template #default="{ row }">{{ getServicePorts(row) }}</template>
                </el-table-column>
                <el-table-column label="选择器" min-width="260">
                  <template #default="{ row }">{{ formatList(row.selector || row.spec?.selector) }}</template>
                </el-table-column>
                <el-table-column label="Endpoints" min-width="260">
                  <template #default="{ row }">{{ getServiceEndpoints(row) }}</template>
                </el-table-column>
                <el-table-column label="匹配原因" min-width="120">
                  <template #default="{ row }">{{ row.matchReason || '-' }}</template>
                </el-table-column>
                <el-table-column label="操作" min-width="120" fixed="right" align="left" header-align="left">
                  <template #default="{ row }">
                    <div class="operation-buttons-container">
                      <el-button link size="small" class="operation-text yaml" @click="openResourceYaml('Service', row)">查看YAML</el-button>
                    </div>
                  </template>
                </el-table-column>
              </el-table>
            </el-tab-pane>
            <el-tab-pane :label="networkTabs[1].label" :name="networkTabs[1].name">
              <div class="resource-toolbar">
                <el-button :icon="Refresh" size="small" :loading="relatedResourcesLoading.network" @click="emit('refresh-related-resources', 'network')" />
              </div>
              <el-table
                :key="`ingress-${resourceRenderKey}`"
                ref="ingressTableRef"
                class="resource-table"
                :data="networkIngresses"
                v-loading="relatedResourcesLoading.network"
                stripe
                size="small"
                style="width: 100%"
              >
                <el-table-column label="名称" min-width="180">
                  <template #default="{ row }">{{ getResourceName(row) }}</template>
                </el-table-column>
                <el-table-column label="命名空间" min-width="140">
                  <template #default="{ row }">{{ getResourceNamespace(row) }}</template>
                </el-table-column>
                <el-table-column label="Host" min-width="220">
                  <template #default="{ row }">{{ getIngressHosts(row) }}</template>
                </el-table-column>
                <el-table-column label="Service" min-width="200">
                  <template #default="{ row }">{{ getIngressServices(row) }}</template>
                </el-table-column>
                <el-table-column label="TLS Secret" min-width="200">
                  <template #default="{ row }">{{ getIngressTlsSecrets(row) }}</template>
                </el-table-column>
                <el-table-column label="操作" min-width="120" fixed="right" align="left" header-align="left">
                  <template #default="{ row }">
                    <div class="operation-buttons-container">
                      <el-button link size="small" class="operation-text yaml" @click="openResourceYaml('Ingress', row)">查看YAML</el-button>
                    </div>
                  </template>
                </el-table-column>
              </el-table>
            </el-tab-pane>
          </el-tabs>
        </div>
      </el-tab-pane>

      <el-tab-pane label="存储" name="storage">
        <div class="resource-section">
          <el-tabs v-model="activeStorageResourceTab" class="resource-tabs" @tab-change="refreshResourceTableLayout">
            <el-tab-pane :label="storageTabs[0].label" :name="storageTabs[0].name">
              <div class="resource-toolbar">
                <el-button :icon="Refresh" size="small" :loading="relatedResourcesLoading.storage" @click="emit('refresh-related-resources', 'storage')" />
              </div>
              <el-table
                :key="`pvc-${resourceRenderKey}`"
                ref="pvcTableRef"
                class="resource-table"
                :data="storagePvcs"
                v-loading="relatedResourcesLoading.storage"
                stripe
                size="small"
                style="width: 100%"
              >
                <el-table-column label="名称" min-width="180">
                  <template #default="{ row }">{{ getResourceName(row) }}</template>
                </el-table-column>
                <el-table-column label="状态" min-width="120">
                  <template #default="{ row }">{{ row.status || row.phase || row.status?.phase || '-' }}</template>
                </el-table-column>
                <el-table-column label="容量" min-width="120">
                  <template #default="{ row }">{{ getPvcCapacity(row) }}</template>
                </el-table-column>
                <el-table-column label="访问模式" min-width="160">
                  <template #default="{ row }">{{ formatList(row.accessModes || row.spec?.accessModes) }}</template>
                </el-table-column>
                <el-table-column label="PV" min-width="180">
                  <template #default="{ row }">{{ getPvcVolume(row) }}</template>
                </el-table-column>
                <el-table-column label="StorageClass" min-width="180">
                  <template #default="{ row }">{{ row.storageClass || row.storageClassName || row.spec?.storageClassName || '-' }}</template>
                </el-table-column>
                <el-table-column label="操作" min-width="120" fixed="right" align="left" header-align="left">
                  <template #default="{ row }">
                    <div class="operation-buttons-container">
                      <el-button link size="small" class="operation-text yaml" @click="openResourceYaml('PVC', row)">查看YAML</el-button>
                    </div>
                  </template>
                </el-table-column>
              </el-table>
            </el-tab-pane>
            <el-tab-pane :label="storageTabs[1].label" :name="storageTabs[1].name">
              <div class="resource-toolbar">
                <el-button :icon="Refresh" size="small" :loading="relatedResourcesLoading.storage" @click="emit('refresh-related-resources', 'storage')" />
              </div>
              <el-table :key="`pv-${resourceRenderKey}`" ref="pvTableRef" class="resource-table" :data="storagePvs" v-loading="relatedResourcesLoading.storage" stripe size="small" style="width: 100%">
                <el-table-column label="名称" min-width="180">
                  <template #default="{ row }">{{ getResourceName(row) }}</template>
                </el-table-column>
                <el-table-column label="状态" min-width="120">
                  <template #default="{ row }">{{ row.status || row.phase || row.status?.phase || '-' }}</template>
                </el-table-column>
                <el-table-column label="容量" min-width="120">
                  <template #default="{ row }">{{ getPvCapacity(row) }}</template>
                </el-table-column>
                <el-table-column label="访问模式" min-width="160">
                  <template #default="{ row }">{{ formatList(row.accessModes || row.spec?.accessModes) }}</template>
                </el-table-column>
                <el-table-column label="Claim" min-width="220">
                  <template #default="{ row }">{{ getPvClaim(row) }}</template>
                </el-table-column>
                <el-table-column label="StorageClass" min-width="180">
                  <template #default="{ row }">{{ row.storageClass || row.storageClassName || row.spec?.storageClassName || '-' }}</template>
                </el-table-column>
                <el-table-column label="操作" min-width="120" fixed="right" align="left" header-align="left">
                  <template #default="{ row }">
                    <div class="operation-buttons-container">
                      <el-button link size="small" class="operation-text yaml" @click="openResourceYaml('PV', row)">查看YAML</el-button>
                    </div>
                  </template>
                </el-table-column>
              </el-table>
            </el-tab-pane>
            <el-tab-pane :label="storageTabs[2].label" :name="storageTabs[2].name">
              <div class="resource-toolbar">
                <el-button :icon="Refresh" size="small" :loading="relatedResourcesLoading.storage" @click="emit('refresh-related-resources', 'storage')" />
              </div>
              <el-table :key="`storageclass-${resourceRenderKey}`" ref="storageClassTableRef" class="resource-table" :data="storageClasses" v-loading="relatedResourcesLoading.storage" stripe size="small" style="width: 100%">
                <el-table-column label="名称" min-width="180">
                  <template #default="{ row }">{{ getResourceName(row) }}</template>
                </el-table-column>
                <el-table-column label="Provisioner" min-width="220">
                  <template #default="{ row }">{{ getStorageClassProvisioner(row) }}</template>
                </el-table-column>
                <el-table-column label="回收策略" min-width="140">
                  <template #default="{ row }">{{ row.reclaimPolicy || row.reclaim_policy || '-' }}</template>
                </el-table-column>
                <el-table-column label="绑定模式" min-width="180">
                  <template #default="{ row }">{{ row.volumeBindingMode || row.volume_binding_mode || '-' }}</template>
                </el-table-column>
                <el-table-column label="操作" min-width="120" fixed="right" align="left" header-align="left">
                  <template #default="{ row }">
                    <div class="operation-buttons-container">
                      <el-button link size="small" class="operation-text yaml" @click="openResourceYaml('StorageClass', row)">查看YAML</el-button>
                    </div>
                  </template>
                </el-table-column>
              </el-table>
            </el-tab-pane>
          </el-tabs>
        </div>
      </el-tab-pane>

      <el-tab-pane label="配置" name="config">
        <div class="resource-section">
          <el-tabs v-model="activeConfigResourceTab" class="resource-tabs" @tab-change="refreshResourceTableLayout">
            <el-tab-pane :label="configTabs[0].label" :name="configTabs[0].name">
              <div class="resource-toolbar">
                <el-button :icon="Refresh" size="small" :loading="relatedResourcesLoading.config" @click="emit('refresh-related-resources', 'config')" />
              </div>
              <el-table
                :key="`configmap-${resourceRenderKey}`"
                ref="configMapTableRef"
                class="resource-table"
                :data="configMaps"
                v-loading="relatedResourcesLoading.config"
                stripe
                size="small"
                style="width: 100%"
              >
                <el-table-column label="名称" min-width="180">
                  <template #default="{ row }">{{ getResourceName(row) }}</template>
                </el-table-column>
                <el-table-column label="命名空间" min-width="140">
                  <template #default="{ row }">{{ getResourceNamespace(row) }}</template>
                </el-table-column>
                <el-table-column label="键数量" min-width="120">
                  <template #default="{ row }">{{ getConfigKeys(row) }}</template>
                </el-table-column>
                <el-table-column label="创建时间" min-width="180">
                  <template #default="{ row }">{{ getResourceAge(row) }}</template>
                </el-table-column>
                <el-table-column label="操作" min-width="120" fixed="right" align="left" header-align="left">
                  <template #default="{ row }">
                    <div class="operation-buttons-container">
                      <el-button link size="small" class="operation-text yaml" @click="openResourceYaml('ConfigMap', row)">查看YAML</el-button>
                    </div>
                  </template>
                </el-table-column>
              </el-table>
            </el-tab-pane>
            <el-tab-pane :label="configTabs[1].label" :name="configTabs[1].name">
              <div class="resource-toolbar">
                <el-button :icon="Refresh" size="small" :loading="relatedResourcesLoading.config" @click="emit('refresh-related-resources', 'config')" />
              </div>
              <el-table :key="`secret-${resourceRenderKey}`" ref="secretTableRef" class="resource-table" :data="secrets" v-loading="relatedResourcesLoading.config" stripe size="small" style="width: 100%">
                <el-table-column label="名称" min-width="180">
                  <template #default="{ row }">{{ getResourceName(row) }}</template>
                </el-table-column>
                <el-table-column label="命名空间" min-width="140">
                  <template #default="{ row }">{{ getResourceNamespace(row) }}</template>
                </el-table-column>
                <el-table-column label="类型" min-width="180">
                  <template #default="{ row }">{{ getSecretType(row) }}</template>
                </el-table-column>
                <el-table-column label="键数量" min-width="120">
                  <template #default="{ row }">{{ getConfigKeys(row) }}</template>
                </el-table-column>
                <el-table-column label="创建时间" min-width="180">
                  <template #default="{ row }">{{ getResourceAge(row) }}</template>
                </el-table-column>
                <el-table-column label="操作" min-width="120" fixed="right" align="left" header-align="left">
                  <template #default="{ row }">
                    <div class="operation-buttons-container">
                      <el-button link size="small" class="operation-text yaml" @click="openResourceYaml('Secret', row)">查看YAML</el-button>
                    </div>
                  </template>
                </el-table-column>
              </el-table>
            </el-tab-pane>
          </el-tabs>
        </div>
      </el-tab-pane>
    </el-tabs>

    <PodYamlDialog
      v-model:visible="resourceYamlDialogVisible"
      :yaml-content="currentResourceYaml"
      :resource-name="currentResourceYamlInfo.name"
      :resource-type="currentResourceYamlInfo.type"
      :editable="false"
      @close="resourceYamlDialogVisible = false"
    />
  </div>
</template>

<style scoped>
.tabs-section {
  margin-top: 12px;
  min-width: 0;
  max-width: 100%;
  overflow-x: hidden;
  scroll-margin-top: 12px;
}

.detail-tabs,
.detail-tabs :deep(.el-tabs__content),
.detail-tabs :deep(.el-tab-pane) {
  min-width: 0;
  max-width: 100%;
  overflow-x: hidden;
}

.detail-tabs :deep(.el-tabs__header) {
  margin: 0 0 16px;
  padding: 0 18px;
  border: 0;
  border-radius: 0;
  border-top: 1px solid rgba(223, 231, 242, 0.88);
  border-bottom: 1px solid rgba(223, 231, 242, 0.88);
  background: rgba(247, 250, 255, 0.56);
  box-shadow: none;
}

.detail-tabs :deep(.el-tabs__nav-wrap) {
  padding: 0;
  overflow: visible;
}

.detail-tabs :deep(.el-tabs__nav-wrap::after) {
  display: block;
  height: 1px;
  background-color: rgba(223, 231, 242, 0.95);
}

.detail-tabs :deep(.el-tabs__active-bar) {
  height: 2px;
  border-radius: 999px;
  background-color: var(--aiops-brand);
}

.detail-tabs :deep(.el-tabs__nav-scroll) {
  overflow: visible;
}

.detail-tabs :deep(.el-tabs__nav) {
  display: inline-flex;
  gap: 22px;
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
}

.detail-tabs :deep(.el-tabs__item) {
  height: 48px;
  padding: 0 !important;
  border: 0 !important;
  border-radius: 0;
  color: var(--aiops-text-primary);
  font-weight: 600;
  line-height: 48px;
  transition: color 0.18s ease;
}

.detail-tabs :deep(.el-tabs__item:hover),
.detail-tabs :deep(.el-tabs__item.is-active) {
  color: var(--aiops-brand);
}

.detail-tabs :deep(.el-tabs__item.is-active) {
  background: transparent;
  box-shadow: none;
}

.detail-tabs :deep(.el-tabs__content) {
  height: auto !important;
  min-height: 0 !important;
  margin-top: 8px;
  padding: 0;
  overflow-x: hidden !important;
  overflow-y: visible !important;
  border: 0;
  border-radius: 0;
  background: transparent;
}

.detail-tabs :deep(.el-tab-pane) {
  min-height: 0 !important;
  overflow-x: hidden !important;
  overflow-y: visible !important;
}

.detail-table {
  flex: none;
  width: 100%;
  max-width: 100%;
  border: 1px solid rgba(223, 231, 242, 0.88);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.92);
  overflow: hidden;
  box-shadow: none;
}

.detail-table :deep(.el-table__inner-wrapper),
.detail-table :deep(.el-table__header-wrapper),
.detail-table :deep(.el-table__body-wrapper),
.detail-table :deep(.el-scrollbar),
.detail-table :deep(.el-scrollbar__view) {
  max-width: 100%;
  overflow-x: hidden !important;
}

.detail-table :deep(.el-table__body-wrapper) {
  max-height: clamp(220px, calc(100vh - 390px), 430px);
  overflow-x: hidden !important;
  overflow-y: auto;
}

.detail-table :deep(.el-scrollbar__wrap) {
  max-height: clamp(220px, calc(100vh - 390px), 430px);
  margin-bottom: 0 !important;
  padding-bottom: 0 !important;
  overflow-x: hidden !important;
  overflow-y: auto !important;
}

.detail-table :deep(.el-scrollbar__bar.is-horizontal) {
  display: none !important;
  height: 0 !important;
  opacity: 0 !important;
  pointer-events: none;
}

.detail-table :deep(.el-scrollbar__bar.is-horizontal .el-scrollbar__thumb) {
  display: none !important;
}

.detail-table :deep(.el-scrollbar__wrap::-webkit-scrollbar),
.detail-tabs :deep(.el-tabs__content::-webkit-scrollbar),
.detail-tabs :deep(.el-tab-pane::-webkit-scrollbar),
.detail-table :deep(.el-table__inner-wrapper::-webkit-scrollbar),
.detail-table :deep(.el-table__body-wrapper::-webkit-scrollbar) {
  width: 10px;
  height: 0 !important;
}

.detail-tabs :deep(.el-tabs__content::-webkit-scrollbar:horizontal),
.detail-tabs :deep(.el-tab-pane::-webkit-scrollbar:horizontal),
.detail-table :deep(.el-table__inner-wrapper::-webkit-scrollbar:horizontal),
.detail-table :deep(.el-table__body-wrapper::-webkit-scrollbar:horizontal),
.detail-table :deep(.el-scrollbar__wrap::-webkit-scrollbar:horizontal) {
  display: none;
  height: 0 !important;
}

.detail-table :deep(.el-table__body-wrapper::-webkit-scrollbar-track) {
  background: rgba(237, 242, 250, 0.72);
  border-radius: 999px;
}

.detail-table :deep(.el-table__body-wrapper::-webkit-scrollbar-thumb) {
  background: rgba(165, 177, 195, 0.86);
  border: 2px solid rgba(237, 242, 250, 0.86);
  border-radius: 999px;
}

.detail-table :deep(.el-table__body-wrapper::-webkit-scrollbar-thumb:hover) {
  background: rgba(134, 148, 170, 0.92);
}

.pod-name-container {
  transition: color 0.2s ease;
}

.pod-name-container:hover {
  color: var(--aiops-brand);
  transform: none;
}

.pod-name {
  color: var(--aiops-brand);
  font-size: 13px;
  font-weight: 700;
}

.image-cell {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  max-width: 100%;
  line-height: 1.4;
}

.history-image-tag {
  max-width: 100%;
  font-family: "Courier New", Monaco, monospace;
  cursor: default;
}

.history-image-tag :deep(.el-tag__content) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pod-resource-cell {
  display: inline-flex;
  flex-direction: column;
  gap: 2px;
  color: #334155;
  font-size: 12px;
  line-height: 1.35;
}

.operation-buttons-container {
  display: flex;
  flex-direction: row;
  gap: 8px;
  flex-wrap: nowrap;
  justify-content: flex-start;
  align-items: center;
}

.operation-buttons-container :deep(.el-button) {
  height: auto;
  min-height: 0;
  margin-left: 0;
  padding: 0;
  border: 0;
  font-size: 12px;
  line-height: 1.2;
  background: transparent;
}

.operation-buttons-container :deep(.el-button:hover:not(.is-disabled)) {
  background: transparent;
  opacity: 0.78;
}

.operation-buttons-container :deep(.operation-text.logs),
.operation-buttons-container :deep(.operation-text.ai) {
  color: var(--aiops-brand);
}

.operation-buttons-container :deep(.operation-text.yaml),
.operation-buttons-container :deep(.operation-text.terminal),
.operation-buttons-container :deep(.operation-text.rollback) {
  color: var(--aiops-warning);
}

.operation-buttons-container :deep(.operation-text.more) {
  color: var(--aiops-text-secondary);
}

.operation-more-icon {
  margin-left: 2px;
  font-size: 11px;
}

.operation-buttons-container :deep(.operation-text.delete) {
  color: var(--aiops-danger);
}

.logs-tab-content {
  padding: 0;
}

.logs-controls {
  padding: 16px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 16px;
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.82), rgba(247, 250, 255, 0.76));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.72);
}

.logs-controls-row {
  margin-bottom: 12px;
}

.logs-inline-controls,
.logs-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.logs-actions {
  justify-content: flex-end;
}

.log-theme-select {
  width: 138px;
  flex: 0 0 138px;
}

.log-theme-option {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.log-theme-swatch {
  width: 14px;
  height: 14px;
  border: 2px solid;
  border-radius: 4px;
  box-sizing: border-box;
}

.logs-control-label {
  font-size: 12px;
}

.logs-display {
  height: 680px;
  margin-top: 12px;
  border: 1px solid var(--log-border, #dfe7f2);
  border-radius: 18px;
  background-color: var(--log-background, #f8fafc);
  overflow: hidden;
  transition: border-color 0.2s ease, background-color 0.2s ease;
}

.logs-placeholder,
.logs-empty {
  height: 100%;
  padding: 40px;
  border-radius: 8px;
  color: var(--log-foreground, var(--aiops-text-secondary));
  text-align: center;
  background-color: var(--log-background, #f8fafc);
}

.logs-content-display {
  height: 100%;
}

.logs-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid var(--log-border, #dfe7f2);
  border-radius: 8px 8px 0 0;
  background-color: var(--log-header-background, #f1f5f9);
  transition: border-color 0.2s ease, background-color 0.2s ease;
}

.container-name {
  color: var(--log-accent, #2563eb);
  font-size: 14px;
  font-weight: 600;
}

.logs-info {
  color: var(--log-muted, #64748b);
  font-size: 12px;
}

.logs-editor-container {
  background-color: var(--log-background, #f8fafc);
}

.ws-status {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
}

.empty-panel {
  padding: 20px;
  text-align: center;
}

.empty-actions {
  margin-top: 16px;
}

.resource-section {
  min-width: 0;
  max-width: 100%;
  overflow-x: hidden;
}

.resource-warning {
  margin-bottom: 12px;
  border-radius: 8px;
}

.resource-tabs :deep(.el-tabs__header) {
  margin: 0 0 16px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.95);
}

.resource-tabs :deep(.el-tabs__nav-wrap::after) {
  display: none;
}

.resource-tabs :deep(.el-tabs__active-bar) {
  height: 2px;
  border-radius: 999px;
  background-color: var(--aiops-brand);
}

.resource-tabs :deep(.el-tabs__nav) {
  gap: 22px;
}

.resource-tabs :deep(.el-tabs__item) {
  height: 38px;
  padding: 0 !important;
  color: var(--aiops-text-primary);
  font-weight: 500;
  line-height: 38px;
}

.resource-tabs :deep(.el-tabs__item:hover),
.resource-tabs :deep(.el-tabs__item.is-active) {
  color: var(--aiops-brand);
}

.resource-tabs :deep(.el-tabs__content) {
  overflow: visible;
}

.resource-toolbar {
  display: flex;
  justify-content: flex-start;
  margin: 0 0 12px;
}

.resource-table {
  width: 100%;
  border: 1px solid rgba(223, 231, 242, 0.88);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.92);
  overflow: hidden;
}

.resource-table :deep(.el-table__header th),
.resource-table :deep(.el-table__fixed-right-patch) {
  background: rgba(248, 250, 255, 0.98) !important;
}

.detail-table :deep(.el-table__header th),
.detail-table :deep(.el-table__header th .cell),
.detail-table :deep(.el-table__body td),
.detail-table :deep(.el-table__body td .cell),
.resource-table :deep(.el-table__header th),
.resource-table :deep(.el-table__header th .cell),
.resource-table :deep(.el-table__body td),
.resource-table :deep(.el-table__body td .cell) {
  text-align: left !important;
}
</style>
