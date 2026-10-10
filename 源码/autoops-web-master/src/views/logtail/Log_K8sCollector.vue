<template>
  <div class="k8s-collector-page">
    <div v-if="isClusterView" class="collector-tab-content">
      <div class="collector-toolbar">
        <el-form :model="clusterQuery" inline class="toolbar-form" @submit.prevent>
          <el-form-item label="关键字">
            <el-input v-model.trim="clusterQuery.keyword" placeholder="集群 / Collector / DaemonSet" clearable @keyup.enter="handleClusterSearch" />
          </el-form-item>
          <el-form-item label="集群">
            <el-select v-model="clusterQuery.clusterId" placeholder="全部" filterable clearable @change="handleClusterSearch">
              <el-option v-for="cluster in clusters" :key="cluster.id" :label="clusterNameOf(cluster)" :value="cluster.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="clusterQuery.status" placeholder="全部" clearable @change="handleClusterSearch">
              <el-option label="运行中" value="running" />
              <el-option label="安装中" value="pending" />
              <el-option label="异常" value="failed" />
              <el-option label="未安装" value="not_installed" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <div class="toolbar-actions">
              <el-button type="primary" size="small" :icon="Search" :loading="loading.clusterList" @click="handleClusterSearch">搜索</el-button>
              <el-button type="warning" size="small" :icon="Refresh" @click="resetClusterSearch">重置</el-button>
            </div>
          </el-form-item>
        </el-form>
      </div>

      <div class="table-shell">
        <el-table v-loading="loading.clusterList" :data="clusterCollectors" class="data-table" height="100%" :empty-text="clusterEmptyText">
          <el-table-column label="集群" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">
              <div class="name-cell">
                <strong>{{ k8sClusterCollectorClusterText(row) }}</strong>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="Collector" min-width="190" show-overflow-tooltip>
            <template #default="{ row }">
              <div class="name-cell">
                <strong>{{ k8sClusterCollectorName(row) }}</strong>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="运行状态" width="110">
            <template #default="{ row }">
              <el-tag :type="k8sCollectorStatusType(row)" size="small" effect="plain">
                {{ k8sCollectorStatusText(row) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="副本" min-width="180">
            <template #default="{ row }">
              <span class="mono-text">{{ k8sCollectorReplicaText(row) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="Pod 状态" min-width="120" show-overflow-tooltip>
            <template #default="{ row }">{{ k8sCollectorPodSummary(row) }}</template>
          </el-table-column>
          <el-table-column label="操作" fixed="right" width="250">
            <template #default="{ row }">
              <div class="operation-links">
                <el-button
                  v-if="showClusterCollectorAction(row, 'install')"
                  link
                  type="success"
                  :disabled="isK8sCollectorBusy(row)"
                  @click="submitK8sClusterCollectorOperation(row, 'install')"
                >
                  安装
                </el-button>
                <el-button
                  v-if="showClusterCollectorAction(row, 'update')"
                  link
                  type="primary"
                  :disabled="isK8sCollectorBusy(row)"
                  @click="submitK8sClusterCollectorOperation(row, 'update')"
                >
                  更新
                </el-button>
                <el-button
                  v-if="showClusterCollectorAction(row, 'reinstall')"
                  link
                  type="warning"
                  :disabled="isK8sCollectorBusy(row)"
                  @click="submitK8sClusterCollectorOperation(row, 'reinstall')"
                >
                  重装
                </el-button>
                <el-button
                  v-if="showClusterCollectorAction(row, 'uninstall')"
                  link
                  type="danger"
                  :disabled="isK8sCollectorBusy(row)"
                  @click="submitK8sClusterCollectorOperation(row, 'uninstall')"
                >
                  卸载
                </el-button>
                <el-button link type="primary" @click="openK8sClusterCollectorDrawer(row)">详情</el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="pagination-section">
        <el-pagination
          :current-page="clusterPagination.page"
          :page-size="clusterPagination.pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="clusterPagination.total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleClusterSizeChange"
          @current-change="handleClusterPageChange"
        />
      </div>
    </div>

    <div v-else class="collector-tab-content">
      <div class="collector-toolbar">
      <el-form :model="query" inline class="toolbar-form" @submit.prevent>
        <el-form-item label="关键字">
          <el-input v-model.trim="query.keyword" placeholder="任务 / 工作负载 / 命名空间 / 容器" clearable @keyup.enter="handleSearch" />
        </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="query.status" placeholder="全部" clearable @change="handleSearch">
              <el-option label="采集中" value="running" />
              <el-option label="下发中/未下发" value="pending" />
              <el-option label="采集失败" value="failed" />
              <el-option label="停止采集" value="stopped" />
            </el-select>
          </el-form-item>
        <el-form-item>
          <div class="toolbar-actions">
            <el-button type="primary" size="small" :icon="Search" :loading="loading.list" @click="handleSearch">搜索</el-button>
            <el-button type="warning" size="small" :icon="Refresh" @click="resetSearch">重置</el-button>
          </div>
        </el-form-item>
      </el-form>
    </div>

    <div class="table-shell">
      <el-table v-loading="loading.list" :data="collectors" class="data-table" height="100%" :empty-text="emptyText">
        <el-table-column label="Collector" min-width="180" show-overflow-tooltip>
          <template #default="{ row }">
            <div class="name-cell">
              <strong>{{ k8sCollectorName(row) }}</strong>
              <span>Task {{ k8sCollectorTaskId(row) || '-' }} / targetIndex {{ k8sCollectorTargetIndex(row) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="集群" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">{{ k8sCollectorClusterText(row) }}</template>
        </el-table-column>
        <el-table-column label="工作负载" min-width="230" show-overflow-tooltip>
          <template #default="{ row }">{{ k8sCollectorTargetText(row) }}</template>
        </el-table-column>
        <el-table-column label="采集来源" width="112">
          <template #default="{ row }">{{ k8sLogSourceText(k8sCollectorLogSource(row)) }}</template>
        </el-table-column>
        <el-table-column label="采集状态" width="118">
          <template #default="{ row }">
            <el-tag :type="k8sCollectorStatusType(row)" size="small" effect="plain">
              {{ k8sCollectorStatusText(row) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="副本" min-width="150">
          <template #default="{ row }">
            <span class="mono-text">{{ k8sCollectorReplicaText(row) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="Pod 状态" min-width="120" show-overflow-tooltip>
          <template #default="{ row }">{{ k8sCollectorPodSummary(row) }}</template>
        </el-table-column>
        <el-table-column label="操作" fixed="right" width="190">
          <template #default="{ row }">
            <div class="operation-links">
              <el-button
                v-if="showK8sCollectorAction(row, 'update')"
                link
                type="primary"
                :disabled="isK8sCollectorBusy(row)"
                @click="submitK8sCollectorOperation(row, 'update')"
              >
                更新
              </el-button>
<el-button link type="primary" @click="openK8sCollectorDrawer(row)">详情</el-button>
              <el-button
                v-if="showK8sCollectorAction(row, 'uninstall')"
                link
                type="danger"
                :disabled="isK8sCollectorBusy(row)"
                @click="submitK8sCollectorOperation(row, 'uninstall')"
              >
                卸载
              </el-button>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <div class="pagination-section">
      <el-pagination
        :current-page="pagination.page"
        :page-size="pagination.pageSize"
        :page-sizes="[10, 20, 50, 100]"
        :total="pagination.total"
        layout="total, sizes, prev, pager, next, jumper"
        @size-change="handleSizeChange"
        @current-change="handlePageChange"
      />
    </div>
    </div>

    <el-drawer v-model="clusterDrawer.visible" :title="clusterDrawerTitle" size="clamp(560px, 42vw, 860px)" class="log-drawer">
      <div v-loading="loading.clusterDetail" class="collector-detail">
        <div class="drawer-toolbar">
          <el-button size="small" :icon="Refresh" :loading="loading.clusterDetail" @click="loadK8sClusterCollectorStatus">刷新状态</el-button>
          <el-button
            v-if="showClusterCollectorAction(clusterCollectorDetail, 'install')"
            type="success"
            size="small"
            @click="submitK8sClusterCollectorOperation(clusterDrawer.current, 'install')"
          >
            安装
          </el-button>
          <el-button
            v-if="showClusterCollectorAction(clusterCollectorDetail, 'update')"
            type="primary"
            size="small"
            @click="submitK8sClusterCollectorOperation(clusterDrawer.current, 'update')"
          >
            更新
          </el-button>
          <el-button
            v-if="showClusterCollectorAction(clusterCollectorDetail, 'reinstall')"
            type="warning"
            size="small"
            @click="submitK8sClusterCollectorOperation(clusterDrawer.current, 'reinstall')"
          >
            重装
          </el-button>
          <el-button
            v-if="showClusterCollectorAction(clusterCollectorDetail, 'uninstall')"
            type="danger"
            size="small"
            @click="submitK8sClusterCollectorOperation(clusterDrawer.current, 'uninstall')"
          >
            卸载
          </el-button>
        </div>

        <el-descriptions :column="2" border>
          <el-descriptions-item label="集群">{{ k8sClusterCollectorClusterText(clusterCollectorDetail) }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="k8sCollectorStatusType(clusterCollectorDetail)" size="small" effect="plain">
              {{ k8sCollectorStatusText(clusterCollectorDetail) }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="Collector">{{ k8sClusterCollectorName(clusterCollectorDetail) }}</el-descriptions-item>
          <el-descriptions-item label="命名空间">{{ k8sClusterCollectorNamespace(clusterCollectorDetail) }}</el-descriptions-item>
          <el-descriptions-item label="DaemonSet">{{ k8sClusterCollectorDaemonSet(clusterCollectorDetail) }}</el-descriptions-item>
          <el-descriptions-item label="副本">{{ k8sCollectorReplicaText(clusterCollectorDetail) }}</el-descriptions-item>
          <el-descriptions-item label="进度" :span="2">
            <div class="detail-progress">
              <el-progress :percentage="k8sCollectorProgress(clusterCollectorDetail)" :status="k8sCollectorProgressStatus(clusterCollectorDetail)" />
              <span>{{ k8sCollectorProgressText(clusterCollectorDetail) }}</span>
            </div>
          </el-descriptions-item>
          <el-descriptions-item label="最近错误" :span="2">
            <span class="danger-text">{{ k8sCollectorLastError(clusterCollectorDetail) || '-' }}</span>
          </el-descriptions-item>
        </el-descriptions>

        <div class="detail-section-head">
          <h4>配置预览</h4>
        </div>
        <CodeEditor
          :model-value="clusterCollectorConfigPreview"
          language="yaml"
          theme="vs"
          height="220px"
          readonly
          placeholder="暂无 Vector YAML 配置预览"
        />

        <div class="detail-section-head">
          <h4>Pod 状态</h4>
        </div>
        <el-table :data="clusterCollectorPods" height="180" empty-text="暂无 Pod 状态">
          <el-table-column label="Pod" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">{{ k8sPodName(row) }}</template>
          </el-table-column>
          <el-table-column label="状态" width="110">
            <template #default="{ row }">{{ k8sPodStatus(row) }}</template>
          </el-table-column>
          <el-table-column label="重启次数" width="100">
            <template #default="{ row }">{{ k8sPodRestartCount(row) }}</template>
          </el-table-column>
          <el-table-column label="等待原因" min-width="140" show-overflow-tooltip>
            <template #default="{ row }">{{ k8sPodWaitingReason(row) }}</template>
          </el-table-column>
        </el-table>

        <div class="detail-section-head">
          <h4>Warning Events</h4>
        </div>
        <el-table :data="clusterCollectorEvents" height="180" empty-text="暂无 Warning Events">
          <el-table-column prop="type" label="类型" width="100" />
          <el-table-column prop="reason" label="原因" min-width="140" show-overflow-tooltip />
          <el-table-column label="消息" min-width="240" show-overflow-tooltip>
            <template #default="{ row }">{{ k8sEventMessage(row) }}</template>
          </el-table-column>
          <el-table-column label="时间" width="170">
            <template #default="{ row }">{{ formatTime(row.lastTimestamp || row.eventTime || row.createdAt || row.time) }}</template>
          </el-table-column>
        </el-table>

        <div class="detail-section-head">
          <h4>操作记录</h4>
          <el-button link type="primary" :loading="loading.clusterOperations" @click="loadK8sClusterCollectorOperations">刷新</el-button>
        </div>
        <el-table v-loading="loading.clusterOperations" :data="clusterOperations" height="220" empty-text="暂无操作记录">
          <el-table-column prop="opType" label="操作类型" min-width="130">
            <template #default="{ row }">{{ operationTypeText(row.opType || row.type || row.action) }}</template>
          </el-table-column>
          <el-table-column prop="opStatus" label="状态" width="110">
            <template #default="{ row }">
              <el-tag :type="operationStatusType(row.opStatus || row.status)" size="small" effect="plain">
                {{ operationStatusText(row.opStatus || row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="operator" label="操作人" width="110" />
          <el-table-column label="创建时间" width="170">
            <template #default="{ row }">{{ formatTime(row.createdAt || row.createTime || row.startedAt) }}</template>
          </el-table-column>
          <el-table-column label="错误信息" min-width="220" show-overflow-tooltip>
            <template #default="{ row }">
              <span class="danger-text">{{ row.errorMsg || row.errorMessage || '-' }}</span>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-drawer>

    <el-drawer v-model="drawer.visible" :title="drawerTitle" size="clamp(560px, 42vw, 860px)" class="log-drawer">
      <div v-loading="loading.detail" class="collector-detail">
        <div class="drawer-toolbar">
          <el-button size="small" :icon="Refresh" :loading="loading.detail" @click="loadK8sCollectorStatus">刷新状态</el-button>
          <el-button
            v-if="showK8sCollectorAction(collectorDetail, 'install')"
            type="success"
            size="small"
            @click="submitK8sCollectorOperation(drawer.current, 'install')"
          >
            安装
          </el-button>
          <el-button
            v-if="showK8sCollectorAction(collectorDetail, 'update')"
            type="primary"
            size="small"
            @click="submitK8sCollectorOperation(drawer.current, 'update')"
          >
            更新
          </el-button>
          <el-button
            v-if="showK8sCollectorAction(collectorDetail, 'reinstall')"
            type="warning"
            size="small"
            @click="submitK8sCollectorOperation(drawer.current, 'reinstall')"
          >
            重装
          </el-button>
          <el-button
            v-if="showK8sCollectorAction(collectorDetail, 'uninstall')"
            type="danger"
            size="small"
            @click="submitK8sCollectorOperation(drawer.current, 'uninstall')"
          >
            卸载任务
          </el-button>
        </div>

        <el-tabs v-model="drawer.activeTab" class="collector-detail-tabs">
          <el-tab-pane label="详情" name="detail">
            <el-descriptions :column="2" border>
              <el-descriptions-item label="Collector">{{ k8sCollectorName(collectorDetail) }}</el-descriptions-item>
              <el-descriptions-item label="采集状态">
                <el-tag :type="k8sCollectorStatusType(collectorDetail)" size="small" effect="plain">
                  {{ k8sCollectorStatusText(collectorDetail) }}
                </el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="集群">{{ k8sCollectorClusterText(collectorDetail) }}</el-descriptions-item>
              <el-descriptions-item label="命名空间">{{ k8sCollectorNamespace(collectorDetail) }}</el-descriptions-item>
              <el-descriptions-item label="工作负载" :span="2">{{ k8sCollectorTargetText(collectorDetail) }}</el-descriptions-item>
              <el-descriptions-item label="采集来源">{{ k8sLogSourceText(k8sCollectorLogSource(collectorDetail)) }}</el-descriptions-item>
              <el-descriptions-item label="副本">{{ k8sCollectorReplicaText(collectorDetail) }}</el-descriptions-item>
              <el-descriptions-item label="ConfigMap" :span="2">{{ collectorConfigMapText }}</el-descriptions-item>
              <el-descriptions-item label="进度" :span="2">
                <div class="detail-progress">
                  <el-progress :percentage="k8sCollectorProgress(collectorDetail)" :status="k8sCollectorProgressStatus(collectorDetail)" />
                  <span>{{ k8sCollectorProgressText(collectorDetail) }}</span>
                </div>
              </el-descriptions-item>
              <el-descriptions-item label="最近错误" :span="2">
                <span class="danger-text">{{ k8sCollectorLastError(collectorDetail) || '-' }}</span>
              </el-descriptions-item>
            </el-descriptions>
          </el-tab-pane>

          <el-tab-pane label="配置预览" name="config">
            <div class="drawer-toolbar">
              <span class="config-map-name">{{ collectorConfigMapText }}</span>
              <el-button size="small" :loading="loading.detail" @click="loadK8sCollectorStatus">刷新预览</el-button>
            </div>
            <CodeEditor
              :model-value="collectorConfigYaml"
              language="yaml"
              theme="vs"
              height="calc(100vh - 250px)"
              readonly
              placeholder="ConfigMap 不存在或暂无配置内容"
            />
          </el-tab-pane>

          <el-tab-pane label="采集任务" name="collect">
            <div class="detail-section-head">
              <h4>Pod 状态</h4>
            </div>
            <el-table :data="collectorPods" height="220" empty-text="暂无 Pod 状态">
              <el-table-column label="Pod" min-width="180" show-overflow-tooltip>
                <template #default="{ row }">{{ k8sPodName(row) }}</template>
              </el-table-column>
              <el-table-column label="状态" width="110">
                <template #default="{ row }">{{ k8sPodStatus(row) }}</template>
              </el-table-column>
              <el-table-column label="重启次数" width="100">
                <template #default="{ row }">{{ k8sPodRestartCount(row) }}</template>
              </el-table-column>
              <el-table-column label="等待原因" min-width="140" show-overflow-tooltip>
                <template #default="{ row }">{{ k8sPodWaitingReason(row) }}</template>
              </el-table-column>
            </el-table>

            <div class="detail-section-head">
              <h4>Warning Events</h4>
            </div>
            <el-table :data="collectorEvents" height="220" empty-text="暂无 Warning Events">
              <el-table-column prop="type" label="类型" width="100" />
              <el-table-column prop="reason" label="原因" min-width="140" show-overflow-tooltip />
              <el-table-column label="消息" min-width="240" show-overflow-tooltip>
                <template #default="{ row }">{{ k8sEventMessage(row) }}</template>
              </el-table-column>
              <el-table-column label="时间" width="170">
                <template #default="{ row }">{{ formatTime(row.lastTimestamp || row.eventTime || row.createdAt || row.time) }}</template>
              </el-table-column>
            </el-table>
          </el-tab-pane>

          <el-tab-pane label="操作记录" name="operations">
            <div class="drawer-toolbar">
              <el-button link type="primary" :loading="loading.operations" @click="loadK8sCollectorOperations">刷新</el-button>
            </div>
            <el-table v-loading="loading.operations" :data="operations" height="calc(100vh - 250px)" empty-text="暂无操作记录">
              <el-table-column prop="opType" label="操作类型" min-width="130">
                <template #default="{ row }">{{ operationTypeText(row.opType || row.type || row.action) }}</template>
              </el-table-column>
              <el-table-column prop="opStatus" label="状态" width="110">
                <template #default="{ row }">
                  <el-tag :type="operationStatusType(row.opStatus || row.status)" size="small" effect="plain">
                    {{ operationStatusText(row.opStatus || row.status) }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="operator" label="操作人" width="110" />
              <el-table-column label="创建时间" width="170">
                <template #default="{ row }">{{ formatTime(row.createdAt || row.createTime || row.startedAt) }}</template>
              </el-table-column>
              <el-table-column label="错误信息" min-width="220" show-overflow-tooltip>
                <template #default="{ row }">
                  <span class="danger-text">{{ row.errorMsg || row.errorMessage || '-' }}</span>
                </template>
              </el-table-column>
            </el-table>
          </el-tab-pane>
        </el-tabs>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Search } from '@element-plus/icons-vue'
import logtailApi from '@/api/logtail'
import k8sApi from '@/api/k8s'
import CodeEditor from '@/components/CodeEditor.vue'

/* global defineProps, defineExpose */
const props = defineProps({
  selectedNode: {
    type: Object,
    default: null
  },
  active: {
    type: Boolean,
    default: false
  },
  view: {
    type: String,
    default: 'workload'
  }
})

const clusters = ref([])
const clusterCollectors = ref([])
const collectors = ref([])
const clusterOperations = ref([])
const operations = ref([])
const clusterCollectorStatus = ref(null)
const collectorStatus = ref(null)
const clusterLoading = ref(false)

const loading = reactive({
  clusterList: false,
  clusterDetail: false,
  clusterOperations: false,
  list: false,
  detail: false,
  operations: false
})

const clusterQuery = reactive({
  keyword: '',
  clusterId: '',
  status: ''
})

const query = reactive({
  keyword: '',
  status: ''
})

const pagination = reactive({
  page: 1,
  pageSize: 10,
  total: 0
})

const clusterPagination = reactive({
  page: 1,
  pageSize: 10,
  total: 0
})

const clusterDrawer = reactive({
  visible: false,
  current: null
})

const drawer = reactive({
  visible: false,
  current: null,
  activeTab: 'detail'
})

let pollTimer = null
let pollDeadline = 0
let pollingInFlight = false

const VECTOR_IMAGE_REGISTRY = 'docker.1ms.run'
const VECTOR_IMAGE_REPOSITORY = 'timberio/vector'
const DEFAULT_VECTOR_IMAGE = `${VECTOR_IMAGE_REGISTRY}/${VECTOR_IMAGE_REPOSITORY}:latest-alpine`

const isClusterView = computed(() => props.view === 'cluster')

const k8sLogSourceOptions = [
  { label: 'K8S标准日志(stdout)', value: 'stdout' },
  { label: 'K8S容器日志(sidecar)', value: 'container_file' },
  { label: 'K8S节点日志(hostPath)', value: 'node_file' }
]

const k8sWorkloadTypeOptions = [
  { label: 'Deployment', value: 'deployment' },
  { label: 'StatefulSet', value: 'statefulset' },
  { label: 'DaemonSet', value: 'daemonset' },
  { label: 'Job', value: 'job' },
  { label: 'CronJob', value: 'cronjob' }
]

const extractData = response => response?.data?.data ?? response?.data ?? response

const extractList = response => {
  const data = extractData(response)
  if (Array.isArray(data)) return data
  const candidate = [
    data?.list,
    data?.items,
    data?.rows,
    data?.records,
    data?.values,
    data?.clusters,
    data?.namespaces,
    data?.clusterCollectors,
    data?.cluster_collectors,
    data?.collectors,
    data?.operations,
    data?.workloads,
    data?.pods
  ].find(Array.isArray)
  return candidate || []
}

const extractTotal = response => {
  const data = extractData(response)
  if (typeof data?.total === 'number') return data.total
  if (typeof data?.count === 'number') return data.count
  if (typeof data?.totalCount === 'number') return data.totalCount
  return extractList(response).length
}

function parseExecuteTargets(value) {
  if (Array.isArray(value)) return value
  const text = String(value || '').trim()
  if (!text) return []
  try {
    const parsed = JSON.parse(text)
    return Array.isArray(parsed) ? parsed : []
  } catch (error) {
    console.error('解析执行目标失败:', error)
    return []
  }
}

function firstK8sTarget(task = {}) {
  return parseExecuteTargets(task.executeTargetsJson || task.execute_targets_json)[0] || {}
}

function normalizeK8sLogSource(value) {
  const source = String(value || '').trim().toLowerCase()
  if (['node_file', 'host_path', 'hostpath', 'node_path', 'node_path_file'].includes(source)) return 'node_file'
  if (['container_file', 'containerfile', 'container_path'].includes(source)) return 'container_file'
  return 'stdout'
}

function normalizeK8sWorkloadType(value) {
  const type = String(value || '').trim().toLowerCase()
  if (['deployments', 'deployment'].includes(type)) return 'deployment'
  if (['statefulsets', 'statefulset'].includes(type)) return 'statefulset'
  if (['daemonsets', 'daemonset'].includes(type)) return 'daemonset'
  if (['jobs', 'job'].includes(type)) return 'job'
  if (['cronjobs', 'cronjob'].includes(type)) return 'cronjob'
  return type || 'deployment'
}

function k8sWorkloadTypeText(value) {
  const type = normalizeK8sWorkloadType(value)
  return k8sWorkloadTypeOptions.find(item => item.value === type)?.label || type || '-'
}

function k8sLogSourceText(value) {
  const source = normalizeK8sLogSource(value)
  return k8sLogSourceOptions.find(item => item.value === source)?.label || source
}

const scopeParams = computed(() => {
  const node = props.selectedNode
  const params = {}
  if (!node) return params
  if (node.project?.id) params.projectId = node.project.id
  if ((node.type === 'store' || node.type === 'task') && node.store?.id && !String(node.store.id).startsWith('orphan-')) {
    params.storeId = node.store.id
  }
  if (node.type === 'task' && node.task?.id) params.taskId = node.task.id
  return params
})

const scopeKey = computed(() => JSON.stringify(scopeParams.value))

const emptyText = computed(() => {
  if (props.selectedNode?.type === 'task') return '当前任务暂无 K8S Collector，请确认任务类型或安装状态'
  return '当前范围暂无 K8S Collector'
})

const clusterEmptyText = computed(() => {
  if (!clusters.value.length) return '暂无 K8S 集群，请先接入集群'
  return '暂无集群级 Vector 状态'
})

const clusterDrawerTitle = computed(() => (
  clusterDrawer.current ? `集群级 Vector - ${k8sClusterCollectorClusterText(clusterDrawer.current)}` : '集群级 Vector'
))

const drawerTitle = computed(() => (
  drawer.current ? `K8S Collector - ${k8sCollectorName(drawer.current)}` : 'K8S Collector'
))

const clusterCollectorDetail = computed(() => clusterCollectorStatus.value || clusterDrawer.current || {})

const clusterCollectorConfigPreview = computed(() => {
  const detail = clusterCollectorDetail.value
  const configPreview = detail?.configPreview
  const preview = (
    configPreview?.yaml ||
    configPreview?.vectorYaml ||
    configPreview?.manifestYaml ||
    configPreview ||
    detail?.vectorYaml ||
    detail?.vectorYAML ||
    detail?.manifestYaml ||
    detail?.yamlPreview ||
    detail?.yaml ||
    detail?.generatedYaml ||
    detail?.generated_yaml ||
    detail?.manifest
  )
  return stringifyConfigPreview(preview)
})

const collectorDetail = computed(() => collectorStatus.value || drawer.current || {})

const collectorConfigMapNamespace = computed(() => {
  const detail = collectorDetail.value
  return detail?.configMapNamespace || detail?.config_map_namespace || detail?.configMap?.namespace || '-'
})

const collectorConfigMapName = computed(() => {
  const detail = collectorDetail.value
  return detail?.configMapName || detail?.config_map_name || detail?.configMap?.name || '-'
})

const collectorConfigMapText = computed(() => `${collectorConfigMapNamespace.value}/${collectorConfigMapName.value}`)

const collectorConfigYaml = computed(() => {
  const detail = collectorDetail.value
  return stringifyConfigPreview(
    detail?.configYaml ||
    detail?.config_yaml ||
    detail?.configMap?.yaml ||
    detail?.configMap?.configYaml ||
    detail?.configMap?.data ||
    ''
  )
})

const clusterCollectorPods = computed(() => {
  const detail = clusterCollectorDetail.value
  return Array.isArray(detail?.pods) ? detail.pods : []
})

const collectorPods = computed(() => {
  const detail = collectorDetail.value
  return Array.isArray(detail?.pods) ? detail.pods : []
})

const clusterCollectorEvents = computed(() => {
  const detail = clusterCollectorDetail.value
  if (Array.isArray(detail?.warningEvents)) return detail.warningEvents
  return Array.isArray(detail?.events) ? detail.events : []
})

const collectorEvents = computed(() => {
  const detail = collectorDetail.value
  if (Array.isArray(detail?.warningEvents)) return detail.warningEvents
  return Array.isArray(detail?.events) ? detail.events : []
})

const clusterNameOf = cluster => cluster?.name || cluster?.clusterName || cluster?.displayName || `Cluster ${cluster?.id || ''}`

const sameId = (left, right) => (
  left !== undefined &&
  left !== null &&
  right !== undefined &&
  right !== null &&
  String(left) === String(right)
)

const textIncludes = (value, keyword) => String(value ?? '').toLowerCase().includes(String(keyword || '').toLowerCase())

const loadClusters = async (force = false) => {
  if (clusterLoading.value) return
  if (clusters.value.length && !force) return
  clusterLoading.value = true
  try {
    const response = await k8sApi.getClusterList({ page: 1, size: 100, pageSize: 100 })
    clusters.value = extractList(response).filter(cluster => cluster?.id !== undefined && cluster?.id !== null)
  } catch (error) {
    console.error('加载 K8S 集群失败:', error)
    clusters.value = []
  } finally {
    clusterLoading.value = false
  }
}

const buildClusterCollectorRows = rows => {
  const normalizedRows = []
  const seenClusterIds = new Set()

  extractList(rows).forEach(row => {
    const clusterId = k8sClusterCollectorId(row)
    if (clusterId !== undefined && clusterId !== null && clusterId !== '') seenClusterIds.add(String(clusterId))
    normalizedRows.push(row)
  })

  clusters.value.forEach(cluster => {
    if (seenClusterIds.has(String(cluster.id))) return
    normalizedRows.push({
      clusterId: cluster.id,
      clusterName: clusterNameOf(cluster),
      collectorName: 'aiops-vector',
      daemonSetName: 'aiops-vector',
      namespace: 'aiops-vector',
      status: 'not_installed',
      progress: 0
    })
  })

  return normalizedRows
}

const filterClusterCollectorRows = rows => {
  const keyword = String(clusterQuery.keyword || '').trim()
  return rows.filter(row => {
    if (clusterQuery.clusterId && !sameId(k8sClusterCollectorId(row), clusterQuery.clusterId)) return false
    if (clusterQuery.status && k8sCollectorStatusOf(row) !== clusterQuery.status) return false
    if (!keyword) return true
    return (
      textIncludes(k8sClusterCollectorClusterText(row), keyword) ||
      textIncludes(k8sClusterCollectorName(row), keyword) ||
      textIncludes(k8sClusterCollectorDaemonSet(row), keyword) ||
      textIncludes(k8sClusterCollectorNamespace(row), keyword)
    )
  })
}

const loadK8sClusterCollectors = async (options = {}) => {
  const { silent = false, force = false } = options
  if (!props.active && !force) return
  if (!silent) loading.clusterList = true
  try {
    await loadClusters()
    const response = await logtailApi.getK8sClusterCollectors({
      keyword: clusterQuery.keyword || undefined,
      clusterId: clusterQuery.clusterId || undefined,
      status: clusterQuery.status || undefined
    })
    const filteredRows = filterClusterCollectorRows(buildClusterCollectorRows(extractList(response)))
    clusterPagination.total = filteredRows.length
    const start = (clusterPagination.page - 1) * clusterPagination.pageSize
    clusterCollectors.value = filteredRows.slice(start, start + clusterPagination.pageSize)
    if (clusterCollectors.value.some(isK8sCollectorActive)) startPolling()
  } catch (error) {
    console.error('加载集群级 Vector 失败:', error)
    if (!silent) {
      clusterCollectors.value = []
      clusterPagination.total = 0
    }
  } finally {
    if (!silent) loading.clusterList = false
  }
}

const loadK8sCollectors = async (options = {}) => {
  const { silent = false, force = false } = options
  if (!props.active && !force) return
  if (!silent) loading.list = true
  try {
    const response = await logtailApi.getK8sCollectors({
      page: pagination.page,
      pageSize: pagination.pageSize,
      ...scopeParams.value,
      keyword: query.keyword || undefined,
      status: query.status === 'stopped' ? undefined : query.status || undefined,
      taskStatus: query.status === 'stopped' ? 2 : undefined
    })
    collectors.value = extractList(response)
    pagination.total = extractTotal(response)
    if (collectors.value.some(isK8sCollectorActive)) startPolling()
  } catch (error) {
    console.error('加载 K8S Collector 失败:', error)
    if (!silent) {
      collectors.value = []
      pagination.total = 0
    }
  } finally {
    if (!silent) loading.list = false
  }
}

const handleClusterSearch = () => {
  clusterPagination.page = 1
  loadK8sClusterCollectors()
}

const resetClusterSearch = () => {
  Object.assign(clusterQuery, {
    keyword: '',
    clusterId: '',
    status: ''
  })
  clusterPagination.page = 1
  loadK8sClusterCollectors()
}

const handleClusterSizeChange = size => {
  clusterPagination.pageSize = size
  clusterPagination.page = 1
  loadK8sClusterCollectors()
}

const handleClusterPageChange = page => {
  clusterPagination.page = page
  loadK8sClusterCollectors()
}

const handleSearch = () => {
  pagination.page = 1
  loadK8sCollectors()
}

const resetSearch = () => {
  Object.assign(query, {
    keyword: '',
    status: ''
  })
  pagination.page = 1
  loadK8sCollectors()
}

const handleSizeChange = size => {
  pagination.pageSize = size
  pagination.page = 1
  loadK8sCollectors()
}

const handlePageChange = page => {
  pagination.page = page
  loadK8sCollectors()
}

const getK8sCollectorOperationApi = action => ({
  install: logtailApi.installK8sCollector,
  update: logtailApi.updateK8sCollector,
  uninstall: logtailApi.uninstallK8sCollector,
  reinstall: logtailApi.reinstallK8sCollector
}[action])

const k8sCollectorActionText = action => ({
  install: '安装 Collector',
  update: '更新 Collector',
  uninstall: '卸载任务',
  reinstall: '重装 Collector'
}[action] || action)

const getK8sClusterCollectorOperationApi = action => ({
  install: logtailApi.installK8sClusterCollector,
  update: logtailApi.updateK8sClusterCollector,
  uninstall: logtailApi.uninstallK8sClusterCollector,
  reinstall: logtailApi.reinstallK8sClusterCollector
}[action])

const k8sClusterCollectorActionText = action => ({
  install: '安装集群级 Vector',
  update: '更新集群级 Vector',
  uninstall: '卸载集群级 Vector',
  reinstall: '重装集群级 Vector'
}[action] || action)

const confirmK8sClusterCollectorOperation = async (row, action, actionName) => {
  const clusterName = k8sClusterCollectorClusterText(row)
  if (['install', 'update'].includes(action)) {
    const currentImage = normalizeVectorImage(k8sClusterCollectorImage(row))
    const promptMessage = action === 'install'
      ? `确认对「${clusterName}」执行 ${actionName} 吗？可选填写 Vector 镜像。`
      : `确认对「${clusterName}」执行 ${actionName} 吗？不填写镜像时后端会优先沿用当前 DaemonSet 镜像。`
    const { value } = await ElMessageBox.prompt(promptMessage, actionName, {
      type: 'info',
      confirmButtonText: '确认',
      cancelButtonText: '取消',
      inputValue: currentImage || DEFAULT_VECTOR_IMAGE,
      inputPlaceholder: DEFAULT_VECTOR_IMAGE
    })
    return String(value || '').trim()
  }

  await ElMessageBox.confirm(`确认对「${clusterName}」执行 ${actionName} 吗？`, actionName, {
    type: ['uninstall', 'reinstall'].includes(action) ? 'warning' : 'info',
    confirmButtonText: '确认',
    cancelButtonText: '取消'
  })
  return ''
}

const submitK8sClusterCollectorOperation = async (row, action) => {
  const clusterId = k8sClusterCollectorId(row)
  const api = getK8sClusterCollectorOperationApi(action)
  if (!clusterId || !api) {
    ElMessage.warning('缺少集群信息，无法执行操作')
    return
  }
  const actionName = k8sClusterCollectorActionText(action)
  const vectorImage = await confirmK8sClusterCollectorOperation(row, action, actionName)
  const payload = {
    operator: 'admin',
    force: ['install', 'reinstall', 'uninstall'].includes(action)
  }
  if (vectorImage) payload.vectorImage = vectorImage
  await api(clusterId, payload)
  ElMessage.success(`${actionName} 已提交`)
  row.status = 'pending'
  row.progress = 0
  row.progressText = '等待后端执行'
  await loadK8sClusterCollectors({ silent: true, force: true })
  startPolling({ resetDeadline: true })
}

const submitK8sCollectorOperation = async (row, action) => {
  const taskId = k8sCollectorTaskId(row)
  const api = getK8sCollectorOperationApi(action)
  if (!taskId || !api) {
    ElMessage.warning('缺少 Collector 任务信息，无法执行操作')
    return
  }
  const actionName = k8sCollectorActionText(action)
  const confirmMessage = action === 'uninstall'
    ? `确认停止「${k8sCollectorName(row)}」的 ${k8sLogSourceText(k8sCollectorLogSource(row))} 采集吗？共享的 aiops-vector DaemonSet 不会被删除。`
    : `确认对「${k8sCollectorName(row)}」执行 ${actionName} 吗？`
  await ElMessageBox.confirm(confirmMessage, actionName, {
    type: ['uninstall', 'reinstall'].includes(action) ? 'warning' : 'info',
    confirmButtonText: '确认',
    cancelButtonText: '取消'
  })
  await api(taskId, {
    operator: 'admin',
    force: action === 'reinstall',
    targetIndex: k8sCollectorTargetIndex(row)
  })
  ElMessage.success(`${actionName} 已提交`)
  if (action === 'uninstall') {
    row.taskStatus = 2
    row.status = 'not_installed'
    row.progress = 0
    row.progressText = '任务已停止采集'
  } else {
    row.status = 'pending'
    row.progress = 0
    row.progressText = '等待后端执行'
  }
  await loadK8sCollectors({ silent: true, force: true })
  startPolling({ resetDeadline: true })
}

const loadK8sClusterCollectorStatus = async () => {
  const row = clusterDrawer.current
  const clusterId = k8sClusterCollectorId(row)
  if (!clusterId) return
  loading.clusterDetail = true
  try {
    const response = await logtailApi.getK8sClusterCollectorStatus(clusterId)
    clusterCollectorStatus.value = extractData(response) || row
  } catch (error) {
    console.error('加载集群级 Vector 状态失败:', error)
    clusterCollectorStatus.value = row
  } finally {
    loading.clusterDetail = false
  }
}

const loadK8sCollectorStatus = async () => {
  const row = drawer.current
  const taskId = k8sCollectorTaskId(row)
  if (!taskId) return
  loading.detail = true
  try {
    const response = await logtailApi.getK8sCollectorStatus(taskId, {
      targetIndex: k8sCollectorTargetIndex(row)
    })
    collectorStatus.value = extractData(response) || row
  } catch (error) {
    console.error('加载 K8S Collector 状态失败:', error)
    collectorStatus.value = row
  } finally {
    loading.detail = false
  }
}

const loadK8sClusterCollectorOperations = async () => {
  const row = clusterDrawer.current
  const clusterId = k8sClusterCollectorId(row)
  if (!clusterId) return
  loading.clusterOperations = true
  try {
    const response = await logtailApi.getK8sClusterCollectorOperations(clusterId, {
      page: 1,
      pageSize: 100
    })
    clusterOperations.value = extractList(response)
  } catch (error) {
    console.error('加载集群级 Vector 操作记录失败:', error)
    clusterOperations.value = []
  } finally {
    loading.clusterOperations = false
  }
}

const loadK8sCollectorOperations = async () => {
  const row = drawer.current
  const taskId = k8sCollectorTaskId(row)
  if (!taskId) return
  loading.operations = true
  try {
    const response = await logtailApi.getK8sCollectorOperations(taskId, {
      page: 1,
      pageSize: 100,
      targetIndex: k8sCollectorTargetIndex(row)
    })
    operations.value = extractList(response)
  } catch (error) {
    console.error('加载 K8S Collector 操作记录失败:', error)
    operations.value = []
  } finally {
    loading.operations = false
  }
}

const openK8sClusterCollectorDrawer = async row => {
  clusterDrawer.current = row
  clusterCollectorStatus.value = row
  clusterOperations.value = []
  clusterDrawer.visible = true
  await Promise.all([
    loadK8sClusterCollectorStatus(),
    loadK8sClusterCollectorOperations()
  ])
}

const openK8sCollectorDrawer = async row => {
  drawer.current = row
  drawer.activeTab = 'detail'
  collectorStatus.value = row
  operations.value = []
  drawer.visible = true
  await Promise.all([
    loadK8sCollectorStatus(),
    loadK8sCollectorOperations()
  ])
}

const refreshState = async () => {
  let hasActiveCollector = false
  if (props.active) {
    if (isClusterView.value) {
      await loadK8sClusterCollectors({ silent: true, force: true })
      hasActiveCollector = clusterCollectors.value.some(isK8sCollectorActive)
    } else {
      await loadK8sCollectors({ silent: true, force: true })
      hasActiveCollector = collectors.value.some(isK8sCollectorActive)
    }
  }
  if (clusterDrawer.visible && clusterDrawer.current) {
    await Promise.all([
      loadK8sClusterCollectorStatus(),
      loadK8sClusterCollectorOperations()
    ])
  }
  if (drawer.visible && drawer.current) {
    await Promise.all([
      loadK8sCollectorStatus(),
      loadK8sCollectorOperations()
    ])
  }
  return hasActiveCollector
}

const stopPolling = () => {
  if (pollTimer) clearInterval(pollTimer)
  pollTimer = null
  pollDeadline = 0
  pollingInFlight = false
}

const startPolling = (options = {}) => {
  const { resetDeadline = false } = options
  if (!pollTimer || resetDeadline || !pollDeadline) {
    pollDeadline = Date.now() + 60 * 1000
  }
  if (pollTimer) return

  pollTimer = setInterval(async () => {
    if (pollingInFlight) return
    pollingInFlight = true
    try {
      const hasActiveCollector = await refreshState()
      if (!hasActiveCollector || Date.now() > pollDeadline) {
        stopPolling()
      }
    } finally {
      pollingInFlight = false
    }
  }, 5000)
}

const refresh = async (options = {}) => {
  const { resetPolling = false } = options
  await loadClusters()
  if (isClusterView.value) await loadK8sClusterCollectors({ force: true })
  else await loadK8sCollectors({ force: true })
  if (resetPolling) startPolling({ resetDeadline: true })
}

function k8sClusterCollectorId(row) {
  return row?.clusterId || row?.k8sClusterId || row?.clusterID || row?.cluster?.id || row?.id
}

function k8sClusterCollectorClusterText(row) {
  const clusterId = k8sClusterCollectorId(row)
  const matchedCluster = clusters.value.find(cluster => sameId(cluster.id, clusterId))
  return row?.clusterName || row?.k8sClusterName || row?.cluster?.name || (matchedCluster ? clusterNameOf(matchedCluster) : (clusterId ? `集群 ${clusterId}` : '-'))
}

function k8sClusterCollectorName(row) {
  return row?.collectorName || row?.name || row?.vectorName || 'aiops-vector'
}

function k8sClusterCollectorNamespace(row) {
  return row?.namespace || row?.installNamespace || row?.vectorNamespace || 'aiops-vector'
}

function k8sClusterCollectorDaemonSet(row) {
  return row?.daemonSetName || row?.daemonsetName || row?.dsName || row?.workloadName || 'aiops-vector'
}

function k8sClusterCollectorImage(row) {
  return row?.vectorImage || row?.image || row?.currentImage || row?.daemonSetImage || row?.daemonsetImage || row?.containerImage || row?.containers?.[0]?.image || ''
}

function normalizeVectorImage(value) {
  const image = String(value || '').trim()
  if (!image) return ''
  if (image === 'vector' || image.startsWith('vector:') || image.startsWith('vector@')) {
    return `${VECTOR_IMAGE_REGISTRY}/${VECTOR_IMAGE_REPOSITORY}${image.slice('vector'.length)}`
  }
  if (image === VECTOR_IMAGE_REPOSITORY || image.startsWith(`${VECTOR_IMAGE_REPOSITORY}:`) || image.startsWith(`${VECTOR_IMAGE_REPOSITORY}@`)) {
    return `${VECTOR_IMAGE_REGISTRY}/${image}`
  }
  const firstPart = image.split('/')[0]
  if (firstPart.includes('.') || firstPart.includes(':') || firstPart === 'localhost') return image
  return `${VECTOR_IMAGE_REGISTRY}/${image}`
}

function stringifyConfigPreview(value) {
  if (!value) return ''
  if (typeof value === 'string') return value
  try {
    return JSON.stringify(value, null, 2)
  } catch (error) {
    console.error('格式化 Vector YAML 预览失败:', error)
    return String(value)
  }
}

function k8sCollectorTaskId(row) {
  return row?.taskId || row?.collectTaskId || row?.collectTaskID || row?.id
}

function k8sCollectorTargetIndex(row) {
  const index = row?.targetIndex ?? row?.target_index
  return index === undefined || index === null || index === '' ? 0 : Number(index)
}

function k8sCollectorName(row) {
  return row?.collectorName || row?.name || row?.taskName || row?.collectTaskName || `任务 ${k8sCollectorTaskId(row) || '-'}`
}

function isK8sTaskCollectorRow(row) {
  return Boolean(
    row?.taskId ||
    row?.collectTaskId ||
    row?.collectTaskID ||
    row?.taskStatus !== undefined ||
    row?.task_status !== undefined ||
    row?.configStatus !== undefined ||
    row?.config_status !== undefined ||
    row?.k8sLogSource ||
    row?.k8s_log_source ||
    row?.targetIndex !== undefined ||
    row?.target_index !== undefined
  )
}

function k8sCollectorTaskStatus(row) {
  const value = row?.taskStatus ?? row?.task_status ?? row?.task?.taskStatus ?? row?.task?.status
  const status = Number(value)
  return [1, 2].includes(status) ? status : undefined
}

function normalizeK8sCollectorRuntimeStatus(row) {
  const status = String(row?.status || row?.collectorStatus || row?.runtimeStatus || '').toLowerCase()
  if (!status) return ''
  if (['ready', 'healthy', 'installed'].includes(status)) return 'running'
  if (['notinstalled', 'not_install', 'none', 'uninstalled'].includes(status)) return 'not_installed'
  if (['installing', 'updating', 'reinstalling', 'uninstalling'].includes(status)) return 'pending'
  return status
}

function normalizeK8sCollectorConfigStatus(row) {
  const status = String(row?.configStatus || row?.config_status || '').toLowerCase()
  if (['success', 'synced', 'effective'].includes(status)) return 'applied'
  if (['error', 'exception'].includes(status)) return 'failed'
  return status
}

function k8sCollectorStatusOf(row) {
  if (isK8sTaskCollectorRow(row)) {
    if (k8sCollectorTaskStatus(row) === 2) return 'stopped'
    const runtimeStatus = normalizeK8sCollectorRuntimeStatus(row)
    const configStatus = normalizeK8sCollectorConfigStatus(row)
    if (runtimeStatus === 'running' && configStatus === 'applied') return 'collecting'
    if (runtimeStatus === 'failed' || configStatus === 'failed') return 'failed'
    return 'pending'
  }
  const status = normalizeK8sCollectorRuntimeStatus(row)
  if (!status) return 'not_installed'
  return status
}

function k8sCollectorStatusText(row) {
  const rawStatus = String(typeof row === 'string' ? row : (row?.status || row?.collectorStatus || row?.runtimeStatus || '')).toLowerCase()
  if (!isK8sTaskCollectorRow(row)) {
    if (rawStatus === 'installing') return '安装中'
    if (rawStatus === 'updating') return '更新中'
    if (rawStatus === 'reinstalling') return '重装中'
    if (rawStatus === 'uninstalling') return '卸载中'
  }
  const status = typeof row === 'string' ? row : k8sCollectorStatusOf(row)
  return {
    collecting: '采集中',
    stopped: '停止采集',
    running: '运行中',
    pending: '下发中/未下发',
    failed: isK8sTaskCollectorRow(row) ? '采集失败' : '异常',
    not_installed: '未安装',
    uninstalled: '未安装',
    unknown: '未知'
  }[status] || status || '-'
}

function k8sCollectorStatusType(row) {
  const status = typeof row === 'string' ? row : k8sCollectorStatusOf(row)
  return {
    collecting: 'success',
    stopped: 'info',
    running: 'success',
    pending: 'warning',
    failed: 'danger',
    not_installed: 'info',
    uninstalled: 'info'
  }[status] || 'info'
}

function k8sCollectorProgress(row) {
  if (k8sCollectorTaskStatus(row) === 2) return 0
  const progress = Number(row?.progress ?? row?.installProgress ?? row?.operationProgress ?? 0)
  if (progress < 0) return 100
  if (Number.isNaN(progress)) return 0
  return Math.min(100, Math.max(0, progress))
}

function k8sCollectorProgressStatus(row) {
  const status = k8sCollectorStatusOf(row)
  const progress = Number(row?.progress ?? 0)
  if (status === 'failed' || progress < 0) return 'exception'
  if (['running', 'collecting'].includes(status)) return 'success'
  return undefined
}

function k8sCollectorProgressText(row) {
  if (k8sCollectorTaskStatus(row) === 2) return '任务已停止采集'
  if (row?.progressText) return row.progressText
  if (row?.installProgressText) return row.installProgressText
  return k8sCollectorStatusText(row)
}

function isK8sCollectorBusy(row) {
  if (isK8sTaskCollectorRow(row)) {
    const rawStatus = String(row?.status || row?.collectorStatus || row?.runtimeStatus || '').toLowerCase()
    return ['installing', 'updating', 'reinstalling', 'uninstalling'].includes(rawStatus)
  }
  return k8sCollectorStatusOf(row) === 'pending'
}

function showClusterCollectorAction(row, action) {
  const status = k8sCollectorStatusOf(row)
  if (isK8sCollectorBusy(row)) return false
  if (['not_installed', 'uninstalled'].includes(status)) return action === 'install'
  if (status === 'running') return ['update', 'reinstall', 'uninstall'].includes(action)
  if (['failed', 'unknown'].includes(status)) return ['reinstall', 'uninstall'].includes(action)
  return ['reinstall', 'uninstall'].includes(action)
}

function showK8sCollectorAction(row, action) {
  const status = k8sCollectorStatusOf(row)
  if (isK8sCollectorBusy(row)) return false
  if (status === 'stopped') return action === 'install'
  if (['collecting', 'running', 'failed', 'pending'].includes(status)) return ['update', 'reinstall', 'uninstall'].includes(action)
  return ['install', 'update'].includes(action)
}

function k8sCollectorClusterText(row) {
  const target = firstK8sTarget(row)
  return row?.clusterName || row?.k8sClusterName || target.clusterName || (row?.clusterId || target.clusterId ? `集群 ${row?.clusterId || target.clusterId}` : '-')
}

function k8sCollectorNamespace(row) {
  return row?.namespace || row?.k8sNamespace || firstK8sTarget(row).namespace || '-'
}

function k8sCollectorWorkloadType(row) {
  return normalizeK8sWorkloadType(row?.workloadType || row?.k8sWorkloadType || firstK8sTarget(row).workloadType)
}

function k8sCollectorWorkloadName(row) {
  return row?.workloadName || row?.k8sWorkloadName || firstK8sTarget(row).workloadName || '-'
}

function k8sCollectorTargetText(row) {
  const source = k8sCollectorLogSource(row)
  if (source === 'node_file') return 'K8S节点日志(hostPath)'
  return `${k8sCollectorNamespace(row)} / ${k8sWorkloadTypeText(k8sCollectorWorkloadType(row))} ${k8sCollectorWorkloadName(row)}`
}

function k8sCollectorLogSource(row) {
  return normalizeK8sLogSource(row?.k8sLogSource || row?.k8s_log_source || row?.logSource)
}

function k8sCollectorReplicaText(row) {
  const desired = row?.desired ?? row?.desiredReplicas ?? row?.replicas ?? '-'
  const ready = row?.ready ?? row?.readyReplicas ?? '-'
  const updated = row?.updated ?? row?.updatedReplicas ?? '-'
  const available = row?.available ?? row?.availableReplicas ?? '-'
  return `${desired}/${ready}/${updated}/${available}`
}

function k8sCollectorPodSummary(row) {
  const pods = Array.isArray(row?.pods) ? row.pods : []
  if (!pods.length) return row?.podStatus || row?.podSummary || '-'
  const running = pods.filter(pod => ['running', 'ready'].includes(String(pod.status || pod.phase || '').toLowerCase())).length
  return `${running}/${pods.length} Running`
}

function k8sCollectorLastError(row) {
  return row?.lastError || row?.errorMsg || row?.errorMessage || row?.recentError || ''
}

function isK8sCollectorActive(row) {
  const status = k8sCollectorStatusOf(row)
  const progress = Number(row?.progress ?? row?.installProgress ?? 0)
  return status === 'pending' || (progress > 0 && progress < 100)
}

function k8sPodName(row) {
  return row?.name || row?.podName || row?.metadata?.name || '-'
}

function k8sPodStatus(row) {
  return row?.status || row?.phase || row?.readyStatus || '-'
}

function k8sPodRestartCount(row) {
  if (row?.restartCount !== undefined) return row.restartCount
  if (row?.restarts !== undefined) return row.restarts
  const containers = row?.containerStatuses || row?.containers || []
  return Array.isArray(containers) ? containers.reduce((total, item) => total + Number(item?.restartCount || item?.restarts || 0), 0) : 0
}

function k8sPodWaitingReason(row) {
  return row?.waitingReason || row?.reason || row?.state?.waiting?.reason || '-'
}

function k8sEventMessage(row) {
  return row?.message || row?.note || row?.reason || '-'
}

const operationTypeText = type => ({
  install: '安装 Collector',
  update: '更新 Collector',
  uninstall: '卸载任务',
  reinstall: '重装 Collector',
  install_k8s_collector: '安装 Collector',
  update_k8s_collector: '更新 Collector',
  uninstall_k8s_collector: '卸载任务',
  reinstall_k8s_collector: '重装 Collector',
  install_cluster_collector: '安装集群级 Vector',
  update_cluster_collector: '更新集群级 Vector',
  uninstall_cluster_collector: '卸载集群级 Vector',
  reinstall_cluster_collector: '重装集群级 Vector',
  install_k8s_cluster_collector: '安装集群级 Vector',
  update_k8s_cluster_collector: '更新集群级 Vector',
  uninstall_k8s_cluster_collector: '卸载集群级 Vector',
  reinstall_k8s_cluster_collector: '重装集群级 Vector'
}[type] || type || '-')

const operationStatusText = status => ({
  pending: '等待执行',
  running: '执行中',
  success: '成功',
  failed: '失败',
  canceled: '已取消'
}[status] || status || '-')

const operationStatusType = status => ({
  pending: 'warning',
  running: 'primary',
  success: 'success',
  failed: 'danger',
  canceled: 'info'
}[status] || 'info')

const formatTime = value => {
  if (!value || value === '0001-01-01 00:00:00') return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false }).replace(/\//g, '-')
}

watch(
  () => [props.active, props.view, scopeKey.value],
  async ([active]) => {
    if (!active) {
      stopPolling()
      return
    }
    clusterPagination.page = 1
    pagination.page = 1
    await refresh()
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  stopPolling()
})

defineExpose({
  refresh
})
</script>

<style scoped>
.k8s-collector-page {
  min-height: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  gap: 12px;
  --collector-border: var(--log-border, rgba(214, 223, 235, 0.96));
  --collector-text-primary: var(--log-text-primary, #172133);
  --collector-text-secondary: var(--log-text-secondary, #5f6b7c);
  --collector-text-muted: var(--log-text-muted, #8d97a8);
}

.collector-tab-content {
  min-height: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.collector-toolbar {
  flex: 0 0 auto;
  padding: 10px 12px 11px;
  border: 1px solid var(--collector-border);
  border-radius: 18px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.95), rgba(248, 251, 255, 0.92));
}

.toolbar-form {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.toolbar-form :deep(.el-form-item) {
  margin-right: 0;
  margin-bottom: 0;
  flex: 0 0 auto;
}

.toolbar-form :deep(.el-form-item:first-child) {
  flex: 0 1 300px;
}

.toolbar-form :deep(.el-form-item:first-child .el-input) {
  width: 260px;
}

.toolbar-form :deep(.el-select) {
  width: 132px;
}

.toolbar-form :deep(.el-form-item:nth-child(3) .el-input) {
  width: 142px;
}

.toolbar-actions,
.operation-links {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.table-shell {
  min-height: 0;
  flex: 1;
  overflow: hidden;
  border: 1px solid var(--collector-border);
  border-radius: 22px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.95), rgba(248, 251, 255, 0.92));
  box-shadow: 0 12px 34px rgba(17, 24, 39, 0.05);
}

.data-table {
  height: 100%;
  --el-table-border-color: rgba(223, 231, 242, 0.92);
  --el-table-header-bg-color: rgba(248, 250, 255, 0.94);
  --el-table-row-hover-bg-color: rgba(76, 139, 245, 0.05);
  --el-table-text-color: var(--collector-text-secondary);
  --el-table-header-text-color: var(--collector-text-muted);
}

.name-cell strong,
.name-cell span {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.name-cell strong {
  color: var(--collector-text-primary);
  font-size: 14px;
}

.name-cell span {
  margin-top: 4px;
  color: var(--collector-text-muted);
  font-size: 12px;
}

.mono-text {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
}

.danger-text {
  color: #df5c68;
}

.operation-progress {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.operation-progress-main {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.operation-progress-main :deep(.el-progress) {
  width: 68px;
  flex: 0 0 68px;
}

.operation-progress-caption {
  min-width: 0;
  color: #3faf55;
  font-size: 12px;
  line-height: 1.1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.operation-progress-icon {
  width: 18px;
  height: 14px;
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  border: 1px solid currentColor;
  background: #fff;
  font-size: 9px;
  font-weight: 800;
  line-height: 1;
}

.operation-progress :deep(.el-progress-bar__outer) {
  background-color: rgba(214, 223, 235, 0.72);
}

.operation-progress-running .operation-progress-icon,
.operation-progress-pending .operation-progress-icon {
  color: #52b83f;
}

.operation-progress-running :deep(.el-progress-bar__inner),
.operation-progress-pending :deep(.el-progress-bar__inner) {
  background-color: #52b83f;
}

.operation-progress-failed .operation-progress-caption,
.operation-progress-failed .operation-progress-icon {
  color: #df5c68;
}

.operation-progress-failed :deep(.el-progress-bar__inner) {
  background-color: #df5c68;
}

.pagination-section {
  display: flex;
  justify-content: flex-end;
  flex: 0 0 auto;
}

.collector-detail {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.collector-detail-tabs {
  min-height: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
}

.collector-detail-tabs :deep(.el-tabs__header) {
  flex: 0 0 auto;
  margin: 0 0 12px;
}

.collector-detail-tabs :deep(.el-tabs__item) {
  height: 34px;
  font-weight: 600;
}

.collector-detail-tabs :deep(.el-tabs__content) {
  min-height: 0;
  flex: 1;
  display: block !important;
}

.collector-detail-tabs :deep(.el-tab-pane) {
  min-height: 0;
}

.drawer-toolbar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  margin-bottom: 12px;
}

.detail-section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 14px 0 8px;
}

.detail-section-head h4 {
  margin: 0;
  color: var(--collector-text-primary);
  font-size: 14px;
}

.config-map-name {
  min-width: 0;
  color: var(--collector-text-muted);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-progress {
  display: grid;
  grid-template-columns: minmax(160px, 1fr) auto;
  align-items: center;
  gap: 12px;
}

.detail-progress span {
  color: var(--collector-text-secondary);
  font-size: 12px;
  white-space: nowrap;
}

.log-drawer :deep(.el-drawer__body) {
  padding: 16px 18px 20px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
}

.log-drawer :deep(.el-drawer__header) {
  margin-bottom: 0;
  padding: 16px 18px;
  border-bottom: 1px solid var(--collector-border);
}

.k8s-collector-page :deep(.el-input__wrapper),
.k8s-collector-page :deep(.el-select__wrapper) {
  border-radius: 14px;
  background: rgba(252, 253, 255, 0.98);
  box-shadow: inset 0 0 0 1px rgba(214, 223, 235, 0.96) !important;
}

.k8s-collector-page :deep(.el-input__wrapper.is-focus),
.k8s-collector-page :deep(.el-select__wrapper.is-focused) {
  box-shadow: inset 0 0 0 1px rgba(120, 156, 221, 0.96), 0 0 0 4px rgba(76, 139, 245, 0.08) !important;
}
</style>
