<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import {
  Bell,
  CircleCheck,
  Clock,
  Refresh,
  Search,
  Warning
} from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'
import ClusterSelector from './pods/ClusterSelector.vue'
import NamespaceSelector from './pods/NamespaceSelector.vue'

const ALL_NAMESPACES = '__all__'
const EVENT_NAMESPACE_STORAGE_KEY = 'k8s-events-namespace'

const loading = ref(false)
const selectedClusterId = ref('')
const events = ref([])
const currentPage = ref(1)
const pageSize = ref(10)

const queryParams = reactive({
  namespace: ALL_NAMESPACES,
  type: '',
  kind: '',
  name: '',
  reason: '',
  limit: 200
})

const typeOptions = [
  { label: '全部类型', value: '' },
  { label: 'Normal', value: 'Normal' },
  { label: 'Warning', value: 'Warning' }
]

const kindOptions = [
  { label: '全部资源', value: '' },
  { label: 'Pod', value: 'Pod' },
  { label: 'Deployment', value: 'Deployment' },
  { label: 'StatefulSet', value: 'StatefulSet' },
  { label: 'DaemonSet', value: 'DaemonSet' },
  { label: 'Service', value: 'Service' },
  { label: 'Node', value: 'Node' },
  { label: 'Ingress', value: 'Ingress' },
  { label: 'PersistentVolumeClaim', value: 'PersistentVolumeClaim' }
]

const limitOptions = [
  { label: '最近 100 条', value: 100 },
  { label: '最近 200 条', value: 200 },
  { label: '最近 500 条', value: 500 }
]

const filteredEvents = computed(() => {
  const name = queryParams.name.trim().toLowerCase()
  const reason = queryParams.reason.trim().toLowerCase()

  return events.value.filter(item => {
    const matchType = !queryParams.type || item.type === queryParams.type
    const matchKind = !queryParams.kind || String(item.involvedObject || '').startsWith(`${queryParams.kind}/`)
    const matchName = !name || [
      item.message,
      item.involvedObject,
      item.source
    ].some(value => String(value || '').toLowerCase().includes(name))
    const matchReason = !reason || String(item.reason || '').toLowerCase().includes(reason)

    return matchType && matchKind && matchName && matchReason
  })
})

const pagedEvents = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredEvents.value.slice(start, start + pageSize.value)
})

const eventStats = computed(() => {
  const total = events.value.length
  const warning = events.value.filter(item => item.type === 'Warning').length
  const normal = events.value.filter(item => item.type === 'Normal').length
  const latest = events.value[0]?.lastTime || ''

  return [
    {
      label: '最近事件',
      value: total,
      icon: Bell,
      className: 'total'
    },
    {
      label: 'Warning',
      value: warning,
      icon: Warning,
      className: 'warning'
    },
    {
      label: 'Normal',
      value: normal,
      icon: CircleCheck,
      className: 'normal'
    },
    {
      label: '最新时间',
      value: latest ? formatDateTime(latest) : '-',
      icon: Clock,
      className: 'latest'
    }
  ]
})

const reasonGroups = computed(() => {
  const groupMap = new Map()

  events.value.forEach(item => {
    const reason = item.reason || 'Unknown'
    const current = groupMap.get(reason) || {
      reason,
      total: 0,
      warning: 0,
      normal: 0,
      latestTime: ''
    }

    current.total += 1
    if (item.type === 'Warning') current.warning += 1
    if (item.type === 'Normal') current.normal += 1
    if (!current.latestTime || compareTime(item.lastTime, current.latestTime) > 0) {
      current.latestTime = item.lastTime
    }

    groupMap.set(reason, current)
  })

  return Array.from(groupMap.values())
    .sort((a, b) => b.warning - a.warning || b.total - a.total || compareTime(b.latestTime, a.latestTime))
    .slice(0, 8)
})

const fetchEvents = async () => {
  if (!selectedClusterId.value) {
    events.value = []
    return
  }

  try {
    loading.value = true
    const params = {
      limit: queryParams.limit
    }

    if (queryParams.kind) {
      params.kind = queryParams.kind
    }

    const response = queryParams.namespace === ALL_NAMESPACES
      ? await k8sApi.getClusterEvents(selectedClusterId.value, params)
      : await k8sApi.getNamespaceEvents(selectedClusterId.value, queryParams.namespace, params)

    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      const list = responseData.data?.events || []
      events.value = normalizeEvents(Array.isArray(list) ? list : [])
      currentPage.value = 1
    } else {
      events.value = []
      ElMessage.error(responseData.message || '获取事件列表失败')
    }
  } catch (error) {
    console.error('获取 K8s 事件失败:', error)
    events.value = []
    ElMessage.error(error.response?.data?.message || '获取事件列表失败，请检查集群连接')
  } finally {
    loading.value = false
  }
}

const normalizeEvents = list => (
  list.map((item, index) => {
    const message = item.message || ''
    const involvedObject = item.involvedObject || parseInvolvedObject(message)

    return {
      ...item,
      id: `${item.lastTime || index}-${item.reason || 'event'}-${index}`,
      involvedObject,
      lastTime: item.lastTime || item.firstTime || '',
      firstTime: item.firstTime || item.lastTime || ''
    }
  }).sort((a, b) => compareTime(b.lastTime, a.lastTime))
)

const parseInvolvedObject = message => {
  const matched = String(message || '').match(/^([^:\s]+\/[^:\s]+):/)
  return matched ? matched[1] : ''
}

const compareTime = (left, right) => {
  const leftTime = left ? new Date(left).getTime() : 0
  const rightTime = right ? new Date(right).getTime() : 0
  return leftTime - rightTime
}

const handleClusterChange = clusterId => {
  selectedClusterId.value = clusterId
  queryParams.namespace = ALL_NAMESPACES
  fetchEvents()
}

const handleNamespaceChange = namespace => {
  queryParams.namespace = namespace
  if (typeof window !== 'undefined') {
    window.sessionStorage.setItem(EVENT_NAMESPACE_STORAGE_KEY, namespace || ALL_NAMESPACES)
  }
  queryParams.name = ''
  currentPage.value = 1
  fetchEvents()
}

const handleSearch = () => {
  currentPage.value = 1
  fetchEvents()
}

const handleReset = () => {
  queryParams.type = ''
  queryParams.kind = ''
  queryParams.name = ''
  queryParams.reason = ''
  queryParams.limit = 200
  currentPage.value = 1
  fetchEvents()
}

const handlePageSizeChange = size => {
  pageSize.value = size
  currentPage.value = 1
}

const getTypeTag = type => {
  if (type === 'Warning') return 'warning'
  if (type === 'Normal') return 'success'
  return 'info'
}

const formatDateTime = value => {
  if (!value) return '-'

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value

  const pad = number => String(number).padStart(2, '0')
  return [
    date.getFullYear(),
    pad(date.getMonth() + 1),
    pad(date.getDate())
  ].join('-') + ' ' + [
    pad(date.getHours()),
    pad(date.getMinutes()),
    pad(date.getSeconds())
  ].join(':')
}

const formatRelativeTime = value => {
  if (!value) return '-'

  const timestamp = new Date(value).getTime()
  if (Number.isNaN(timestamp)) return '-'

  const diff = Math.max(0, Math.floor((Date.now() - timestamp) / 1000))
  if (diff < 60) return `${diff}s 前`
  if (diff < 3600) return `${Math.floor(diff / 60)}m 前`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h 前`
  return `${Math.floor(diff / 86400)}d 前`
}

watch(
  () => [queryParams.type, queryParams.reason],
  () => {
    currentPage.value = 1
  }
)

watch(selectedClusterId, () => {
  if (typeof window !== 'undefined' && !window.sessionStorage.getItem(EVENT_NAMESPACE_STORAGE_KEY)) {
    queryParams.namespace = ALL_NAMESPACES
  }
})

onMounted(() => {
  if (typeof window !== 'undefined') {
    const storedNamespace = window.sessionStorage.getItem(EVENT_NAMESPACE_STORAGE_KEY)
    if (storedNamespace) {
      queryParams.namespace = storedNamespace
    }
  }

  if (selectedClusterId.value) {
    fetchEvents()
  }
})
</script>

<template>
  <div class="k8s-events-management">
    <section class="events-card">
      <header class="page-hero">
        <div class="hero-title-group">
          <span class="ops-page-header__icon" aria-hidden="true">
            <el-icon><Bell /></el-icon>
          </span>
          <div class="ops-page-header__content">
            <h1>K8s 事件管理</h1>
            <span class="ops-page-header__divider" aria-hidden="true"></span>
            <p class="page-description">
              获取集群最近 Events，按类型与原因聚合，快速定位 Warning 与资源变更。
            </p>
          </div>
        </div>
        <div class="header-actions">
          <ClusterSelector
            v-model="selectedClusterId"
            width="220px"
            size="small"
            margin-right="0"
            theme="cluster"
            @change="handleClusterChange"
          />
          <NamespaceSelector
            v-model="queryParams.namespace"
            :cluster-id="selectedClusterId"
            include-all-option
            all-option-label="全部命名空间"
            :all-option-value="ALL_NAMESPACES"
            filterable
            width="240px"
            size="small"
            theme="namespace"
            @change="handleNamespaceChange"
          />
        </div>
      </header>

      <main class="events-workspace">
        <section class="stats-grid" aria-label="事件统计">
          <article
            v-for="item in eventStats"
            :key="item.label"
            :class="['stat-panel', `is-${item.className}`]"
          >
            <span class="stat-icon">
              <el-icon><component :is="item.icon" /></el-icon>
            </span>
            <span class="stat-content">
              <span class="stat-label">{{ item.label }}</span>
              <span class="stat-value" :title="String(item.value)">{{ item.value }}</span>
            </span>
          </article>
        </section>

        <section class="search-section">
          <div class="section-heading filter-heading">
            <div>
              <h2>事件筛选</h2>
              <span>数据范围：最近 {{ queryParams.limit }} 条事件</span>
            </div>
          </div>
          <el-form :inline="true" :model="queryParams" class="search-form" @submit.prevent>
            <el-form-item label="事件类型">
              <el-select
                v-model="queryParams.type"
                size="small"
                style="width: 150px"
                @change="handleSearch"
              >
                <el-option
                  v-for="option in typeOptions"
                  :key="option.value"
                  :label="option.label"
                  :value="option.value"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="资源类型">
              <el-select
                v-model="queryParams.kind"
                size="small"
                style="width: 190px"
                @change="handleSearch"
              >
                <el-option
                  v-for="option in kindOptions"
                  :key="option.value"
                  :label="option.label"
                  :value="option.value"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="对象/消息">
              <el-input
                v-model="queryParams.name"
                clearable
                size="small"
                style="width: 240px"
                placeholder="资源名或消息关键字"
                @keyup.enter="handleSearch"
                @clear="handleSearch"
              >
                <template #prefix>
                  <el-icon><Search /></el-icon>
                </template>
              </el-input>
            </el-form-item>
            <el-form-item label="原因">
              <el-input
                v-model="queryParams.reason"
                clearable
                size="small"
                style="width: 170px"
                placeholder="Reason"
                @keyup.enter="handleSearch"
                @clear="handleSearch"
              />
            </el-form-item>
            <el-form-item label="范围">
              <el-select
                v-model="queryParams.limit"
                size="small"
                style="width: 150px"
                @change="handleSearch"
              >
                <el-option
                  v-for="option in limitOptions"
                  :key="option.value"
                  :label="option.label"
                  :value="option.value"
                />
              </el-select>
            </el-form-item>
            <el-form-item class="search-actions">
              <el-button
                class="query-action"
                type="primary"
                :icon="Search"
                size="small"
                :loading="loading"
                @click="handleSearch"
              >
                查询
              </el-button>
              <el-button
                class="reset-action"
                type="warning"
                :icon="Refresh"
                size="small"
                :disabled="loading"
                @click="handleReset"
              >
                重置
              </el-button>
            </el-form-item>
          </el-form>
        </section>

        <div class="content-grid">
          <aside class="reason-section">
            <div class="section-heading">
              <div>
                <h2>事件原因分类</h2>
                <span>按告警数和事件总数排序，展示前 8 项</span>
              </div>
            </div>
            <div v-if="reasonGroups.length" class="reason-list">
              <button
                v-for="item in reasonGroups"
                :key="item.reason"
                type="button"
                class="reason-item"
                :class="{ 'is-active': queryParams.reason === item.reason }"
                :aria-pressed="queryParams.reason === item.reason"
                @click="queryParams.reason = queryParams.reason === item.reason ? '' : item.reason"
              >
                <span class="reason-main">
                  <span class="reason-name">{{ item.reason }}</span>
                  <span class="reason-time">{{ formatRelativeTime(item.latestTime) }}</span>
                </span>
                <span class="reason-counts">
                  <el-tag v-if="item.warning" type="warning" size="small">{{ item.warning }}</el-tag>
                  <el-tag type="info" size="small">{{ item.total }}</el-tag>
                </span>
              </button>
            </div>
            <el-empty v-else description="暂无分类数据" :image-size="72" />
          </aside>

          <section class="table-section">
            <div class="section-heading table-heading">
              <div>
                <h2>事件明细</h2>
                <span>展示当前集群与命名空间范围内匹配的事件</span>
              </div>
              <span class="result-count">{{ filteredEvents.length }} 条结果</span>
            </div>
            <div class="table-shell">
              <el-table
                :data="pagedEvents"
                v-loading="loading"
                empty-text="暂无事件数据"
                class="events-table"
                height="100%"
                style="width: 100%"
              >
                <el-table-column label="类型" width="92">
                  <template #default="{ row }">
                    <el-tag :type="getTypeTag(row.type)" size="small">{{ row.type || '-' }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column prop="reason" label="Reason" min-width="140" show-overflow-tooltip />
                <el-table-column label="对象" min-width="185">
                  <template #default="{ row }">
                    <span class="object-text">{{ row.involvedObject || '-' }}</span>
                  </template>
                </el-table-column>
                <el-table-column prop="message" label="消息" min-width="340" show-overflow-tooltip />
                <el-table-column prop="source" label="来源" min-width="130" show-overflow-tooltip />
                <el-table-column label="次数" width="72" align="center">
                  <template #default="{ row }">
                    <span class="count-text">{{ row.count || 0 }}</span>
                  </template>
                </el-table-column>
                <el-table-column label="最后发生" min-width="165">
                  <template #default="{ row }">
                    <div class="time-cell">
                      <span>{{ formatDateTime(row.lastTime) }}</span>
                      <small>{{ formatRelativeTime(row.lastTime) }}</small>
                    </div>
                  </template>
                </el-table-column>
              </el-table>
            </div>

            <div v-if="filteredEvents.length > 0" class="pagination-wrapper">
              <el-pagination
                v-model:current-page="currentPage"
                v-model:page-size="pageSize"
                :page-sizes="[10, 20, 50]"
                :total="filteredEvents.length"
                layout="total, sizes, prev, pager, next, jumper"
                background
                @size-change="handlePageSizeChange"
              />
            </div>
          </section>
        </div>
      </main>
    </section>
  </div>
</template>

<style scoped>
.k8s-events-management {
  --ops-blue-500: #409eff;
  --ops-blue-600: #2563eb;
  --ops-green-500: #10b981;
  --ops-orange-500: #f59e0b;
  --ops-teal-500: #14b8a6;
  --ops-text-heading: #0f172a;
  --ops-text-primary: #172033;
  --ops-text-muted: #64748b;
  --ops-text-weak: #7b8aa0;
  --ops-bg-surface: #ffffff;
  --ops-bg-surface-soft: #fbfdff;
  --ops-bg-subtle: #f3f7fc;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  --ops-border-inner: #dce8f5;
  --ops-border-hover: #b9d4ff;
  --ops-shadow-card: 0 8px 24px rgba(80, 124, 180, 0.07);
  box-sizing: border-box;
  height: calc(100vh - 124px);
  min-height: 0;
  overflow: auto;
  padding: 7px;
  color: var(--ops-text-primary);
  background: transparent;
}

.events-card {
  min-height: 100%;
  overflow: hidden;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: var(--ops-bg-surface);
  box-shadow: var(--ops-shadow-card);
}

.page-hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-surface);
}

.hero-title-group {
  display: flex;
  align-items: center;
  min-width: 0;
}

.ops-page-header__icon {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  place-items: center;
  color: var(--ops-blue-600);
  background: transparent;
  font-size: 29px;
}

.ops-page-header__content {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
}

.ops-page-header__content h1 {
  margin: 0;
  color: var(--ops-text-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  white-space: nowrap;
}

.ops-page-header__divider {
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  background: var(--ops-border-soft);
}

.page-description {
  margin: 0;
  overflow: hidden;
  color: var(--ops-text-muted);
  font-size: 12px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.header-actions {
  display: flex;
  flex: 0 0 auto;
  flex-wrap: nowrap;
  gap: 8px;
  align-items: center;
  justify-content: flex-end;
}

.header-actions :deep(.cluster-selector .el-input__wrapper),
.header-actions :deep(.cluster-selector .el-select__wrapper),
.header-actions :deep(.namespace-selector .el-input__wrapper),
.header-actions :deep(.namespace-selector .el-select__wrapper) {
  min-height: 32px;
  border: 0;
  border-radius: 8px !important;
  background: var(--ops-bg-surface-soft);
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.header-actions :deep(.cluster-selector .el-input__wrapper:hover),
.header-actions :deep(.cluster-selector .el-select__wrapper:hover),
.header-actions :deep(.namespace-selector .el-input__wrapper:hover),
.header-actions :deep(.namespace-selector .el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px var(--ops-border-hover) inset;
}

.header-actions :deep(.cluster-selector .el-input__wrapper.is-focus),
.header-actions :deep(.cluster-selector .el-select__wrapper.is-focused),
.header-actions :deep(.namespace-selector .el-input__wrapper.is-focus),
.header-actions :deep(.namespace-selector .el-select__wrapper.is-focused) {
  box-shadow: 0 0 0 1px var(--ops-blue-500) inset;
}

.events-workspace {
  display: grid;
  gap: 8px;
  min-width: 0;
  padding: 8px 10px 10px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
  min-width: 0;
}

.stat-panel {
  display: flex;
  min-width: 0;
  min-height: 78px;
  align-items: center;
  gap: 10px;
  padding: 8px 11px;
  overflow: hidden;
  border: 1px solid var(--ops-border-inner);
  border-top: 3px solid var(--ops-blue-500);
  border-radius: 10px;
  background: linear-gradient(145deg, #ffffff, var(--ops-bg-surface-soft));
}

.stat-panel.is-warning {
  border-top-color: var(--ops-orange-500);
}

.stat-panel.is-normal {
  border-top-color: var(--ops-green-500);
}

.stat-panel.is-latest {
  border-top-color: var(--ops-teal-500);
}

.stat-icon {
  display: grid;
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  place-items: center;
  color: var(--ops-blue-600);
  border-radius: 10px;
  background: #eef6ff;
  font-size: 17px;
}

.stat-panel.is-warning .stat-icon {
  color: #d97706;
  background: #fff4e5;
}

.stat-panel.is-normal .stat-icon {
  color: #059669;
  background: #eaf8ef;
}

.stat-panel.is-latest .stat-icon {
  color: #0f8f83;
  background: #eafaf7;
}

.stat-content {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 4px;
}

.stat-label {
  overflow: hidden;
  color: var(--ops-text-muted);
  font-size: 11px;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stat-value {
  overflow: hidden;
  color: var(--ops-text-heading);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 19px;
  font-weight: 800;
  line-height: 1.1;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.stat-panel.is-latest .stat-value {
  font-size: 14px;
}

.search-section {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--ops-border-inner);
  border-radius: 10px;
  background: var(--ops-bg-surface-soft);
}

.section-heading {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 8px 10px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-subtle);
}

.section-heading > div {
  min-width: 0;
}

.section-heading h2 {
  margin: 0;
  color: var(--ops-text-heading);
  font-size: 15px;
  font-weight: 700;
  line-height: 1.25;
}

.section-heading span {
  display: block;
  margin-top: 2px;
  overflow: hidden;
  color: var(--ops-text-muted);
  font-size: 11px;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.filter-heading {
  padding-bottom: 7px;
}

.search-form {
  display: flex;
  min-width: 0;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 8px 12px;
  padding: 9px 10px 10px;
}

.search-form .el-form-item {
  margin-bottom: 0;
  margin-right: 0;
}

.search-form :deep(.el-form-item__label) {
  height: 32px;
  padding-right: 6px;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
  line-height: 32px;
}

.search-form :deep(.el-input__wrapper),
.search-form :deep(.el-select__wrapper) {
  min-height: 32px;
  border-radius: 8px;
  background: #ffffff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
  transition:
    box-shadow 160ms ease,
    background-color 160ms ease;
}

.search-form :deep(.el-input__wrapper:hover),
.search-form :deep(.el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px var(--ops-border-hover) inset;
}

.search-form :deep(.el-input__wrapper.is-focus),
.search-form :deep(.el-select__wrapper.is-focused) {
  box-shadow: 0 0 0 1px var(--ops-blue-500) inset;
}

.search-form :deep(.el-input__inner),
.search-form :deep(.el-select__selected-item) {
  color: var(--ops-text-primary);
  font-size: 12px;
}

.search-actions :deep(.el-form-item__content) {
  gap: 8px;
  flex-wrap: nowrap;
}

.search-actions :deep(.el-button) {
  height: 30px;
  margin-left: 0;
  padding: 0 10px;
  border-radius: 8px;
  color: #ffffff;
  font-size: 12px;
  font-weight: 600;
  transition:
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease,
    transform 160ms ease;
}

.search-actions :deep(.el-button:hover) {
  transform: translateY(-1px);
}

.search-actions :deep(.el-button:active) {
  transform: translateY(0);
}

.search-actions :deep(.el-button:focus-visible) {
  outline: 2px solid rgba(64, 158, 255, 0.35);
  outline-offset: 2px;
}

.search-actions :deep(.query-action) {
  border-color: var(--ops-blue-500);
  background: var(--ops-blue-500);
}

.search-actions :deep(.query-action:hover) {
  border-color: var(--ops-blue-600);
  background: var(--ops-blue-600);
}

.search-actions :deep(.reset-action) {
  border-color: #e6a23c;
  background: #e6a23c;
}

.search-actions :deep(.reset-action:hover) {
  border-color: #d98c20;
  background: #d98c20;
}

.search-actions :deep(.el-button.is-disabled),
.search-actions :deep(.el-button.is-disabled:hover) {
  opacity: 0.5;
  transform: none;
}

.content-grid {
  display: grid;
  grid-template-columns: 250px minmax(0, 1fr);
  gap: 8px;
  min-width: 0;
  align-items: start;
}

.reason-section {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--ops-border-inner);
  border-radius: 10px;
  background: var(--ops-bg-surface-soft);
}

.reason-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px;
}

.reason-item {
  display: flex;
  width: 100%;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 7px 8px;
  color: inherit;
  font: inherit;
  text-align: left;
  border: 1px solid var(--ops-border-soft);
  border-radius: 8px;
  background: #ffffff;
  cursor: pointer;
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    box-shadow 160ms ease,
    transform 160ms ease;
}

.reason-item:hover {
  border-color: var(--ops-border-hover);
  background: #f8fbff;
  box-shadow: 0 5px 14px rgba(64, 158, 255, 0.08);
  transform: translateY(-1px);
}

.reason-item:active {
  transform: translateY(0);
}

.reason-item:focus-visible {
  outline: 2px solid rgba(64, 158, 255, 0.35);
  outline-offset: 2px;
}

.reason-item.is-active {
  border-color: var(--ops-blue-500);
  background: #eef6ff;
  box-shadow: 0 0 0 1px rgba(64, 158, 255, 0.08);
}

.reason-main {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 3px;
}

.reason-name {
  overflow: hidden;
  color: var(--ops-text-primary);
  font-size: 12px;
  font-weight: 700;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.reason-item.is-active .reason-name {
  color: var(--ops-blue-600);
}

.reason-time {
  color: var(--ops-text-weak);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 11px;
}

.reason-counts {
  display: inline-flex;
  flex-shrink: 0;
  gap: 4px;
}

.reason-counts :deep(.el-tag) {
  min-width: 28px;
  height: 20px;
  justify-content: center;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 700;
}

.table-section {
  display: flex;
  min-width: 0;
  overflow: hidden;
  flex-direction: column;
  border: 1px solid var(--ops-border-inner);
  border-radius: 10px;
  background: var(--ops-bg-surface);
}

.table-heading {
  flex: 0 0 auto;
}

.table-heading .result-count {
  flex: 0 0 auto;
  margin: 0;
  padding: 3px 8px;
  color: var(--ops-blue-600);
  border: 1px solid #cfe2ff;
  border-radius: 999px;
  background: #eef6ff;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 11px;
  font-weight: 700;
}

.table-shell {
  height: 408px;
  min-width: 0;
  min-height: 320px;
  margin: 8px 8px 0;
  overflow: hidden;
  border: 1px solid #e3ebf5;
  border-radius: 8px;
  background: #ffffff;
}

.events-table {
  --el-table-border-color: #eef3f9;
  --el-table-header-bg-color: var(--ops-bg-subtle);
  --el-table-row-hover-bg-color: #f8fafc;
  color: var(--ops-text-primary);
  font-size: 12px;
}

.events-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.events-table :deep(.el-table__header th) {
  height: 36px;
  color: var(--ops-text-heading) !important;
  font-size: 12px;
  font-weight: 700 !important;
  background: var(--ops-bg-subtle) !important;
}

.events-table :deep(.el-table__cell) {
  padding: 6px 0;
  border-bottom-color: #eef3f9;
}

.events-table :deep(.el-table__body tr:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

.events-table :deep(.el-tag) {
  height: 22px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
}

.object-text {
  color: var(--ops-blue-600);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 11px;
  font-weight: 600;
}

.count-text {
  color: var(--ops-text-heading);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-weight: 800;
}

.time-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
  color: var(--ops-text-primary);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 11px;
  line-height: 1.35;
}

.time-cell small {
  color: var(--ops-text-weak);
  font-size: 10px;
}

.pagination-wrapper {
  display: flex;
  min-height: 44px;
  align-items: center;
  justify-content: flex-end;
  padding: 7px 8px 8px;
}

.pagination-wrapper :deep(.el-pagination) {
  --el-pagination-button-bg-color: var(--ops-bg-subtle);
  --el-pagination-hover-color: var(--ops-blue-600);
  font-size: 12px;
}

.pagination-wrapper :deep(.btn-prev),
.pagination-wrapper :deep(.btn-next),
.pagination-wrapper :deep(.el-pager li) {
  border-radius: 7px;
}

@media (max-width: 1180px) {
  .stats-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .content-grid {
    grid-template-columns: 1fr;
  }

  .reason-list {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 960px) {
  .page-hero {
    align-items: flex-start;
  }

  .ops-page-header__content {
    align-items: flex-start;
    flex-direction: column;
    gap: 2px;
  }

  .ops-page-header__divider {
    display: none;
  }

  .header-actions {
    flex-wrap: wrap;
  }
}

@media (max-width: 760px) {
  .k8s-events-management {
    height: calc(100vh - 104px);
    padding: 6px;
  }

  .page-hero {
    flex-direction: column;
    padding: 8px 10px;
  }

  .header-actions {
    width: 100%;
    justify-content: flex-start;
  }

  .header-actions :deep(.cluster-selector),
  .header-actions :deep(.namespace-selector) {
    width: 100% !important;
  }

  .events-workspace {
    padding: 8px;
  }

  .stats-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .search-form {
    display: grid;
    grid-template-columns: 1fr;
  }

  .search-form .el-form-item,
  .search-form :deep(.el-form-item__content),
  .search-form :deep(.el-select),
  .search-form :deep(.el-input) {
    width: 100% !important;
  }

  .search-form :deep(.el-form-item) {
    display: grid;
    grid-template-columns: 76px minmax(0, 1fr);
  }

  .search-actions :deep(.el-form-item__content) {
    grid-column: 1 / -1;
  }

  .search-actions :deep(.el-button) {
    flex: 1 1 0;
  }

  .reason-list {
    grid-template-columns: 1fr;
  }

  .table-heading {
    align-items: flex-start;
  }

  .table-shell {
    height: 340px;
    overflow-x: auto;
  }

  .pagination-wrapper {
    overflow-x: auto;
    justify-content: flex-start;
  }
}

@media (max-width: 520px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }

  .ops-page-header__icon {
    width: 36px;
    height: 36px;
    flex-basis: 36px;
    font-size: 26px;
  }

  .section-heading span {
    white-space: normal;
  }

  .table-heading .result-count {
    white-space: nowrap;
  }
}
</style>
