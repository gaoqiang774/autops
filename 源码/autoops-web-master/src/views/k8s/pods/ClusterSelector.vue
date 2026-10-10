<template>
  <el-select
    :model-value="modelValue"
    placeholder="请选择集群"
    :size="size"
    :style="{ width, marginRight }"
    :class="['cluster-selector', theme ? `theme-${theme}` : '']"
    :disabled="disabled"
    @change="handleChange"
    :loading="!clusterList.length"
  >
    <template #prefix v-if="theme === 'cluster'">
      <el-icon class="selector-prefix-icon cluster-prefix-icon"><Connection /></el-icon>
    </template>
    <el-option
      v-for="cluster in clusterList"
      :key="cluster.id"
      :label="cluster.name"
      :value="cluster.id"
      :disabled="cluster.status !== 2"
    >
      <div class="cluster-option">
        <span class="cluster-name">{{ cluster.name }}</span>
        <el-tag
          :type="getClusterStatusTag(cluster.status)"
          size="small"
          class="cluster-status-tag"
        >
          {{ getClusterStatusText(cluster.status) }}
        </el-tag>
      </div>
    </el-option>
  </el-select>
</template>

<script setup>
/* global defineProps, defineEmits */
import { ref, onBeforeUnmount, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Connection } from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'
import {
  getStoredClusterId,
  saveStoredClusterId,
  resolvePreferredClusterId,
  subscribeStoredClusterId
} from '@/utils/k8sClusterSelection'

const props = defineProps({
  modelValue: {
    type: [String, Number],
    default: ''
  },
  width: {
    type: String,
    default: '200px'
  },
  size: {
    type: String,
    default: 'small'
  },
  marginRight: {
    type: String,
    default: '12px'
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

const clusterList = ref([])
const selectorSource = 'cluster-selector'
let unsubscribeClusterSelection = null

const resolveClusterIdFromOptions = (clusterId) => {
  const matchedCluster = clusterList.value.find(cluster => String(cluster.id) === String(clusterId || ''))
  return matchedCluster ? matchedCluster.id : clusterId
}

// 获取集群列表
const fetchClusterList = async () => {
  try {
    const response = await k8sApi.getClusterList()
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      const clusters = responseData.data?.list || responseData.data || []
      clusterList.value = clusters.map(cluster => ({
        id: cluster.id,
        name: cluster.name,
        status: cluster.status
      }))

      if (clusterList.value.length > 0) {
        const selectedId = resolvePreferredClusterId(
          clusterList.value,
          props.modelValue || getStoredClusterId()
        )

        if (selectedId) {
          saveStoredClusterId(selectedId, selectorSource)
        }

        if (props.modelValue !== selectedId) {
          emit('update:modelValue', selectedId)
          emit('change', selectedId)
        }
      }

      console.log('集群列表加载成功:', clusterList.value)
    } else {
      ElMessage.error(responseData.message || '获取集群列表失败')
    }
  } catch (error) {
    console.error('获取集群列表失败:', error)
    ElMessage.warning('无法获取集群列表，请检查后端服务')
  }
}

const handleChange = (value) => {
  saveStoredClusterId(value, selectorSource)
  emit('update:modelValue', value)
  emit('change', value)
}

// 集群状态相关方法
const getClusterStatusTag = (status) => {
  const tagMap = {
    1: 'info',
    2: 'success',
    3: 'warning',
    0: 'danger'
  }
  return tagMap[status] || 'info'
}

const getClusterStatusText = (status) => {
  const textMap = {
    0: '离线',
    1: '连接中',
    2: '在线',
    3: '异常'
  }
  return textMap[status] || '未知'
}

onMounted(() => {
  unsubscribeClusterSelection = subscribeStoredClusterId((clusterId, detail = {}) => {
    if (detail.source === selectorSource || String(props.modelValue || '') === String(clusterId || '')) {
      return
    }

    const resolvedClusterId = resolveClusterIdFromOptions(clusterId)
    emit('update:modelValue', resolvedClusterId)
    emit('change', resolvedClusterId)
  })

  fetchClusterList()
})

onBeforeUnmount(() => {
  if (unsubscribeClusterSelection) {
    unsubscribeClusterSelection()
  }
})
</script>

<style scoped>
/* 集群选择样式 */
.cluster-option {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}
.cluster-name {
  font-weight: 500;
  color: #4c8bf5;
}
.cluster-status-tag {
  margin-left: 8px;
}

.selector-prefix-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  margin-right: 6px;
  font-size: 15px;
}

.cluster-prefix-icon {
  color: #4c8bf5;
}

.cluster-selector.theme-cluster :deep(.el-input__wrapper) {
  min-height: 32px;
  background: rgba(252, 253, 255, 0.98);
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 13px;
  box-shadow: none;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;
}

.cluster-selector.theme-cluster :deep(.el-input__wrapper:hover) {
  border-color: #c0c4cc;
}

.cluster-selector.theme-cluster :deep(.el-input__wrapper.is-focus) {
  border-color: rgba(120, 156, 221, 0.9);
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
  background: #ffffff;
}

.cluster-selector.theme-cluster :deep(.el-select__selected-item),
.cluster-selector.theme-cluster :deep(.el-input__inner) {
  color: #172133;
  font-weight: 400;
}

.cluster-selector.theme-cluster :deep(.el-select__caret) {
  color: #8d97a8;
}
</style>
