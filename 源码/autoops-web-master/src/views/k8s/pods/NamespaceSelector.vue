<template>
  <el-select
    :model-value="modelValue"
    placeholder="请选择命名空间"
    :size="size"
    :style="{ width }"
    :class="['namespace-selector', theme ? `theme-${theme}` : '']"
    @change="handleChange"
    @focus="fetchNamespaceList"
    :disabled="disabled || !clusterId"
    :loading="namespaceLoading"
    :filterable="filterable"
    loading-text="命名空间加载中，请稍候..."
    no-data-text="暂无命名空间数据"
    no-match-text="未匹配到命名空间"
  >
    <template #prefix v-if="theme === 'namespace' || (!namespaceLoading && clusterId && namespaceList.length === 0)">
      <div class="selector-prefix">
        <el-icon v-if="theme === 'namespace'" class="selector-prefix-icon namespace-prefix-icon">
          <FolderOpened />
        </el-icon>
        <el-button
          v-if="!namespaceLoading && clusterId && namespaceList.length === 0"
          type="text"
          size="small"
          @click="fetchNamespaceList(true)"
          class="namespace-refresh-btn"
        >
          刷新
        </el-button>
      </div>
    </template>
    <el-option
      v-for="ns in renderedNamespaceList"
      :key="ns.value"
      :label="ns.name"
      :value="ns.value"
    >
      <div class="namespace-option">
        <span :class="['namespace-name', { 'namespace-name-all': ns.isAllOption }]">{{ ns.name }}</span>
        <el-tag
          :type="ns.isAllOption ? 'info' : (ns.status === 'Active' ? 'success' : 'warning')"
          size="small"
          class="namespace-status-tag"
        >
          {{ ns.isAllOption ? '全部' : ns.status }}
        </el-tag>
      </div>
    </el-option>
  </el-select>
</template>

<script setup>
/* global defineProps, defineEmits */
import { ref, watch, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { FolderOpened } from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'
import {
  DEFAULT_K8S_NAMESPACE,
  getStoredNamespace,
  resolvePreferredNamespace,
  saveStoredNamespace
} from '@/utils/k8sNamespaceSelection'

const props = defineProps({
  modelValue: {
    type: String,
    default: 'default'
  },
  clusterId: {
    type: String,
    required: true
  },
  filterable: {
    type: Boolean,
    default: false
  },
  includeAllOption: {
    type: Boolean,
    default: false
  },
  allOptionLabel: {
    type: String,
    default: '全部命名空间'
  },
  allOptionValue: {
    type: String,
    default: '__all__'
  },
  width: {
    type: String,
    default: '200px'
  },
  size: {
    type: String,
    default: 'small'
  },
  theme: {
    type: String,
    default: ''
  },
  disabled: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['update:modelValue', 'change'])

const namespaceList = ref([])
const namespaceLoading = ref(false)
let namespaceRequestPromise = null
let latestNamespaceRequestId = 0

const renderedNamespaceList = computed(() => {
  const options = namespaceList.value.map(ns => ({
    ...ns,
    value: ns.name
  }))

  if (!props.includeAllOption) {
    return options
  }

  return [
    {
      name: props.allOptionLabel,
      value: props.allOptionValue,
      status: 'All',
      isAllOption: true
    },
    ...options
  ]
})

const emitNamespaceSelection = (namespace) => {
  emit('update:modelValue', namespace)
  emit('change', namespace)
}

const getFallbackNamespace = (clusterId) => {
  const storedNamespace = getStoredNamespace(clusterId)
  if (storedNamespace) return storedNamespace
  return props.includeAllOption ? props.allOptionValue : DEFAULT_K8S_NAMESPACE
}

const resolveNamespaceForCurrentCluster = () => {
  const storedNamespace = getStoredNamespace(props.clusterId)
  return resolvePreferredNamespace(
    namespaceList.value,
    storedNamespace || props.modelValue,
    {
      includeAllOption: props.includeAllOption,
      allOptionValue: props.allOptionValue
    }
  )
}

const applyResolvedNamespace = () => {
  if (!props.clusterId || namespaceList.value.length === 0) return

  const selectedNamespace = resolveNamespaceForCurrentCluster()
  if (!selectedNamespace) return

  if (selectedNamespace !== props.modelValue) {
    emitNamespaceSelection(selectedNamespace)
  }

  saveStoredNamespace(props.clusterId, selectedNamespace)
}

// 获取命名空间列表
const fetchNamespaceList = async (force = false) => {
  if (!props.clusterId) {
    namespaceList.value = []
    namespaceRequestPromise = null
    latestNamespaceRequestId++
    return
  }

  // 如果正在加载且不是强制刷新，等待当前请求完成
  if (namespaceLoading.value && !force && namespaceRequestPromise) {
    try {
      await namespaceRequestPromise
    } catch (error) {
      // 忽略错误，让用户手动重试
    }
    return
  }

  // 如果已经有数据且不是强制刷新，则跳过
  if (namespaceList.value.length > 0 && !force) {
    applyResolvedNamespace()
    return
  }

  const requestClusterId = props.clusterId
  const requestId = ++latestNamespaceRequestId

  try {
    namespaceLoading.value = true

    // 创建请求Promise并缓存
    namespaceRequestPromise = k8sApi.getNamespaces(requestClusterId)
    const response = await namespaceRequestPromise
    const responseData = response.data || response

    if (requestId !== latestNamespaceRequestId || requestClusterId !== props.clusterId) {
      return
    }

    console.log('命名空间列表API响应:', responseData)

    if (responseData.code === 200 || responseData.success) {
      // 根据API文档，数据结构是 { data: { namespaces: [...] } }
      const namespaces = responseData.data?.namespaces || responseData.data || []
      // 确保数据是数组
      const namespaceArray = Array.isArray(namespaces) ? namespaces : []

      namespaceList.value = namespaceArray.map(ns => ({
        name: ns.name || ns,
        status: ns.status || 'Active',
        resourceCount: ns.resourceCount || {},
        createdAt: ns.createdAt
      }))

      applyResolvedNamespace()

      console.log('命名空间列表加载成功:', namespaceList.value)
    } else {
      console.error('获取命名空间列表失败:', responseData.message)
      namespaceList.value = []
    }
  } catch (error) {
    console.error('获取命名空间列表失败:', error)
    namespaceList.value = []

    // 只在严重错误时显示消息
    if (error.response?.status === 401) {
      ElMessage.error('认证失败，请重新登录')
    } else if (error.response?.status === 403) {
      ElMessage.error('权限不足，请联系管理员')
    } else if (error.code === 'ECONNABORTED') {
      ElMessage.warning('请求超时，命名空间加载时间较长，请稍后重试')
    } else if (!force) {
      // 静默失败，用户可以手动重试
      console.warn('命名空间列表加载失败，错误已记录')
    }
  } finally {
    if (requestId === latestNamespaceRequestId) {
      namespaceLoading.value = false
      namespaceRequestPromise = null
    }
  }
}

const handleChange = (value) => {
  emitNamespaceSelection(value)
  // 保存用户选择到 localStorage
  if (props.clusterId && value) {
    saveStoredNamespace(props.clusterId, value)
  }
}

// 监听集群变化，重新获取命名空间列表
watch(() => props.clusterId, (newClusterId) => {
  if (newClusterId) {
    namespaceList.value = []
    // 尝试从 localStorage 恢复该集群的 namespace 选择
    const preferredNamespace = getFallbackNamespace(newClusterId)
    emitNamespaceSelection(preferredNamespace)
    fetchNamespaceList(true)
  } else {
    namespaceList.value = []
    latestNamespaceRequestId++
    emitNamespaceSelection('')
  }
}, { immediate: true })
</script>

<style scoped>
/* 命名空间选择样式 */
.namespace-option {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}
.namespace-name {
  font-weight: 500;
  color: #4c8bf5;
}

.namespace-name-all {
  color: #8d97a8;
}

.namespace-status-tag {
  margin-left: 8px;
}

.selector-prefix {
  display: inline-flex;
  align-items: center;
}

.selector-prefix-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin-right: 6px;
  font-size: 15px;
}

.namespace-prefix-icon {
  color: #f3b24b;
}

.namespace-refresh-btn {
  color: #4c8bf5;
  padding: 0;
  margin-right: 4px;
}

.namespace-selector.theme-namespace :deep(.el-input__wrapper) {
  min-height: 32px;
  background: rgba(252, 253, 255, 0.98);
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 13px;
  box-shadow: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;
}

.namespace-selector.theme-namespace :deep(.el-input__wrapper:hover) {
  border-color: #c0c4cc;
}

.namespace-selector.theme-namespace :deep(.el-input__wrapper.is-focus) {
  border-color: rgba(120, 156, 221, 0.9);
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
  background: #ffffff;
}

.namespace-selector.theme-namespace :deep(.el-select__selected-item),
.namespace-selector.theme-namespace :deep(.el-input__inner) {
  color: #172133;
  font-weight: 400;
}

.namespace-selector.theme-namespace :deep(.el-select__caret) {
  color: #8d97a8;
}
</style>
