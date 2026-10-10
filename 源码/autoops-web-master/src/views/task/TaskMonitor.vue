<template>
  <section class="task-view" aria-labelledby="monitor-title">
    <div class="task-summary-grid" v-loading="loading">
      <article class="task-metric-card is-primary">
        <div class="task-metric-card__label"><el-icon><List /></el-icon>等待队列</div>
        <div class="task-metric-card__value">{{ metric('queue_length') }}</div>
        <div class="task-metric-card__hint">当前等待执行的任务</div>
      </article>
      <article class="task-metric-card is-success">
        <div class="task-metric-card__label"><el-icon><CircleCheck /></el-icon>累计成功</div>
        <div class="task-metric-card__value">{{ metric('processed_total') }}</div>
        <div class="task-metric-card__hint">已处理完成的任务</div>
      </article>
      <article class="task-metric-card is-danger">
        <div class="task-metric-card__label"><el-icon><CircleClose /></el-icon>累计失败</div>
        <div class="task-metric-card__value">{{ metric('failed_total') }}</div>
        <div class="task-metric-card__hint">可从失败队列重试</div>
      </article>
      <article class="task-metric-card">
        <div class="task-metric-card__label"><el-icon><Cpu /></el-icon>活跃 Worker</div>
        <div class="task-metric-card__value">{{ metric('active_workers') }}</div>
        <div class="task-metric-card__hint">正在消费队列的节点</div>
      </article>
      <article class="task-metric-card" :class="scheduler.running ? 'is-success' : 'is-warning'">
        <div class="task-metric-card__label"><el-icon><Timer /></el-icon>调度器</div>
        <div class="task-metric-card__value">{{ scheduler.running ? '运行中' : '未运行' }}</div>
        <div class="task-metric-card__hint">{{ Number(scheduler.active_entries || 0) }} 个活跃调度项</div>
      </article>
      <article class="task-metric-card" :class="redisHealthy ? 'is-success' : 'is-warning'">
        <div class="task-metric-card__label"><el-icon><Coin /></el-icon>Redis</div>
        <div class="task-metric-card__value">{{ redisHealthy ? '已连接' : '不可用' }}</div>
        <div class="task-metric-card__hint">不可用时普通任务回退本地执行</div>
      </article>
    </div>

    <div class="task-panel">
      <header class="task-panel__header">
        <div class="task-panel__title">
          <h2 id="monitor-title">运行监控</h2>
          <p>队列、调度器和基础设施运行状态</p>
        </div>
        <div class="task-toolbar">
          <el-input-number v-model="retryLimit" :min="1" :max="100" controls-position="right" aria-label="失败任务重试数量" />
          <el-button type="warning" :loading="retrying" :disabled="failedQueueLength === 0" @click="retryFailed">
            <el-icon><RefreshRight /></el-icon>重试失败任务
          </el-button>
          <el-button type="danger" :loading="clearing" :disabled="failedQueueLength === 0" @click="clearFailed">
            <el-icon><Delete /></el-icon>清空失败队列
          </el-button>
          <el-button class="ops-refresh-button" type="warning" :loading="loading" @click="loadMonitor">
            <el-icon><Refresh /></el-icon>刷新
          </el-button>
        </div>
      </header>
      <div class="task-panel__body">
        <div v-if="partialErrors.length" class="task-inline-message is-warning" role="status">
          <el-icon><Warning /></el-icon>
          <span>{{ partialErrors.join('；') }}</span>
        </div>

        <h3 class="task-section-title">队列详情</h3>
        <div class="task-table-wrap">
          <el-table :data="queueRows" table-layout="fixed">
            <el-table-column label="优先级" min-width="150">
              <template #default="{ row }">
                <el-tag :type="queueType(row.name)" effect="light">{{ queueLabel(row.name) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="key" label="Redis Key" min-width="260" show-overflow-tooltip />
            <el-table-column prop="length" label="任务数量" width="140" align="right" />
            <template #empty><el-empty description="暂无队列数据" /></template>
          </el-table>
        </div>

        <h3 class="task-section-title">调度器信息</h3>
        <dl class="task-detail-list">
          <div class="task-detail-item"><dt>运行状态</dt><dd>{{ scheduler.running ? '运行中' : '未运行' }}</dd></div>
          <div class="task-detail-item"><dt>已注册任务</dt><dd>{{ Number(scheduler.total_tasks || 0) }}</dd></div>
          <div class="task-detail-item"><dt>活跃调度项</dt><dd>{{ Number(scheduler.active_entries || 0) }}</dd></div>
          <div class="task-detail-item"><dt>最近处理时间</dt><dd>{{ formatDate(metrics.last_process_time) }}</dd></div>
          <div class="task-detail-item"><dt>注册任务 ID</dt><dd>{{ registeredTaskIds }}</dd></div>
          <div class="task-detail-item"><dt>系统状态</dt><dd>{{ systemStatusText }}</dd></div>
        </dl>
      </div>
    </div>
  </section>
</template>

<script setup>
import { computed, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import taskApi from '@/api/task'
import { errorMessage, formatDate } from './taskCommon'

const loading = ref(false)
const retrying = ref(false)
const clearing = ref(false)
const retryLimit = ref(10)
const partialErrors = ref([])
const metrics = reactive({})
const scheduler = reactive({ running: false })
const system = reactive({})
const queueDetails = reactive({})

const queueRows = computed(() => ['high', 'normal', 'low', 'retry', 'failed'].map(name => ({
  name,
  key: queueDetails[name]?.key || '--',
  length: Number(queueDetails[name]?.length || 0)
})))

const failedQueueLength = computed(() => Number(queueDetails.failed?.length || 0))
const redisHealthy = computed(() => {
  const value = system.redis?.connected ?? system.redis_connected ?? system.redis?.healthy ?? system.redis_status
  return value === true || ['ok', 'healthy', 'connected', 'running'].includes(String(value || '').toLowerCase())
})
const registeredTaskIds = computed(() => Array.isArray(scheduler.registered_task_ids) && scheduler.registered_task_ids.length
  ? scheduler.registered_task_ids.join(', ')
  : '--')
const systemStatusText = computed(() => system.status_name || system.status || (redisHealthy.value ? '服务可用' : '部分能力不可用'))

const metric = key => Number(metrics[key] || 0).toLocaleString('zh-CN')
const queueLabel = name => ({ high: '高优先级', normal: '普通', low: '低优先级', retry: '重试', failed: '失败' }[name] || name)
const queueType = name => ({ high: 'danger', normal: 'primary', low: 'info', retry: 'warning', failed: 'danger' }[name] || 'info')

async function loadMonitor() {
  loading.value = true
  partialErrors.value = []
  const requests = [
    ['队列指标', taskApi.getQueueMetrics(), metrics],
    ['调度器统计', taskApi.getSchedulerStats(), scheduler],
    ['系统状态', taskApi.getSystemStatus(), system],
    ['队列详情', taskApi.getQueueDetails(), queueDetails]
  ]
  const results = await Promise.allSettled(requests.map(item => item[1]))
  results.forEach((result, index) => {
    const [label, , target] = requests[index]
    if (result.status === 'fulfilled') {
      Object.keys(target).forEach(key => delete target[key])
      Object.assign(target, result.value || {})
    } else {
      partialErrors.value.push(`${label}加载失败`)
    }
  })
  loading.value = false
}

async function retryFailed() {
  retrying.value = true
  try {
    const result = await taskApi.retryFailedQueue(retryLimit.value)
    ElMessage.success(`失败任务已提交重试${result?.count !== undefined ? `：${result.count} 个` : ''}`)
    await loadMonitor()
  } catch (error) {
    ElMessage.error(errorMessage(error, '失败任务重试失败'))
  } finally {
    retrying.value = false
  }
}

async function clearFailed() {
  try {
    await ElMessageBox.confirm(`将清空 ${failedQueueLength.value} 个失败任务，确定继续吗？`, '清空失败队列', {
      type: 'warning', confirmButtonText: '清空', cancelButtonText: '取消'
    })
  } catch (_) {
    return
  }
  clearing.value = true
  try {
    await taskApi.clearFailedQueue()
    ElMessage.success('失败队列已清空')
    await loadMonitor()
  } catch (error) {
    ElMessage.error(errorMessage(error, '失败队列清空失败'))
  } finally {
    clearing.value = false
  }
}

loadMonitor()
</script>
