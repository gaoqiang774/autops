<template>
  <div class="alert-center-page alert-template-page" @click="hideGroupMenu">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/moban.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>告警模板</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>按组件、采集器和分类维护告警模板库，统一配置查询条件与告警内容。</p>
      </div>
      <div class="template-header-summary" aria-label="告警模板统计">
        <span><em>模板</em><strong>{{ total }}</strong></span>
        <i aria-hidden="true"></i>
        <span><em>分类</em><strong>{{ categoryCount }}</strong></span>
      </div>
    </header>

    <el-card shadow="never" class="alert-center-card">
      <section class="alert-search-section">
        <el-form :model="query" :inline="true" class="alert-search-form" @submit.prevent>
          <el-form-item label="关键字">
            <el-input v-model="query.keyword" class="alert-filter-input" clearable placeholder="模板名称 / 标签" @keyup.enter="handleSearch" />
          </el-form-item>
          <el-form-item label="数据源">
            <el-select v-model="query.datasourceType" class="alert-filter-small" clearable placeholder="全部">
              <el-option v-for="item in templateDatasourceOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="来源">
            <el-select v-model="query.source" class="alert-filter-small" clearable placeholder="全部">
              <el-option label="内置" value="builtin" />
              <el-option label="自定义" value="custom" />
              <el-option label="Nightingale" value="n9e" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" size="small" class="alert-search-button template-toolbar-button" @click="handleSearch">
              <el-icon><Search /></el-icon>
              <span>查询</span>
            </el-button>
            <el-button type="warning" size="small" class="alert-reset-button template-toolbar-button" @click="handleReset">
              <el-icon><RefreshRight /></el-icon>
              <span>重置</span>
            </el-button>
            <el-button type="success" size="small" class="alert-create-button template-toolbar-button" @click="openCreateDialog">
              <el-icon><Plus /></el-icon>
              <span>新增模板</span>
            </el-button>
            <el-button type="warning" size="small" class="template-import-button template-toolbar-button" @click="openPrometheusImportDialog">
              <el-icon><Upload /></el-icon>
              <span>导入 Prometheus 模板</span>
            </el-button>
          </el-form-item>
        </el-form>
      </section>

      <div class="alert-split-layout template-market-layout">
        <aside class="alert-side-panel template-side-panel">
          <p class="alert-side-title">组件模板库</p>

          <div class="template-tree">
            <button
              type="button"
              class="template-tree-node root-node"
              :class="{ active: !query.component && !query.collector && !query.category }"
              @click="selectAllTemplates"
              @contextmenu.prevent.stop="openGroupMenu($event, 'root')"
            >
              <span class="node-spacer"></span>
              <img :src="openFolderIcon" alt="" class="node-svg-icon node-folder" />
              <span class="node-label">全部模板</span>
              <span class="node-count">({{ allTemplateTotal }})</span>
            </button>

            <div v-for="component in componentTree" :key="component.component" class="template-domain-group">
              <button
                type="button"
                class="template-tree-node component-node"
                :class="{ active: query.component === component.component && !query.collector && !query.category }"
                @click="handleComponentClick(component)"
                @contextmenu.prevent.stop="openGroupMenu($event, 'component', { component })"
              >
                <span class="node-toggle">
                  <el-icon :class="{ expanded: expandedComponents[component.component] }"><ArrowRight /></el-icon>
                </span>
                <img
                  :src="expandedComponents[component.component] ? openFolderIcon : closedFolderIcon"
                  alt=""
                  class="node-svg-icon node-folder"
                />
                <span class="node-label">{{ component.label }}</span>
                <span class="node-count">({{ componentTemplateCount(component) }})</span>
              </button>

              <div v-show="expandedComponents[component.component]" class="template-collectors">
                <div v-for="collector in component.collectors" :key="collector.collector" class="template-collector-group">
                  <button
                    type="button"
                    class="template-tree-node collector-node"
                    :class="{ active: query.component === component.component && query.collector === collector.collector && !query.category }"
                    @click="handleCollectorClick(component, collector)"
                    @contextmenu.prevent.stop="openGroupMenu($event, 'collector', { component, collector })"
                  >
                    <span class="node-toggle">
                      <el-icon :class="{ expanded: expandedCollectors[collectorKey(component, collector)] }"><ArrowRight /></el-icon>
                    </span>
                    <img
                      :src="expandedCollectors[collectorKey(component, collector)] ? openFolderIcon : closedFolderIcon"
                      alt=""
                      class="node-svg-icon node-folder"
                    />
                    <span class="node-label">{{ collector.label }}</span>
                    <span class="node-count">({{ collectorTemplateCount(component, collector) }})</span>
                  </button>

                  <div v-show="expandedCollectors[collectorKey(component, collector)]" class="template-children">
                    <button
                      v-for="category in collector.categories"
                      :key="`${collector.collector}-${category.category}`"
                      type="button"
                      class="template-tree-node category-node"
                      :class="{ active: query.component === component.component && query.collector === collector.collector && query.category === category.category }"
                      @click="handleCategoryClick(component, collector, category)"
                      @contextmenu.prevent.stop="openGroupMenu($event, 'category', { component, collector, category })"
                    >
                      <span class="node-svg-icon node-script" :style="scriptIconStyle"></span>
                      <span class="node-label">{{ category.label }}</span>
                      <span class="node-code">{{ categoryTemplateCount(component, collector, category) }}</span>
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <Teleport to="body">
            <div
              v-if="groupMenu.visible"
              class="template-tree-menu"
              :style="{ left: `${groupMenu.x}px`, top: `${groupMenu.y}px` }"
              @click.stop
            >
              <button v-if="groupMenu.canCreate" type="button" @click="openCreateGroupDialog(groupMenu.nextLevel)">
                <el-icon><Plus /></el-icon>
                <span>{{ groupMenu.level === 'root' ? '创建根分组' : `新增${groupLevelLabel(groupMenu.nextLevel)}` }}</span>
              </button>
              <button v-if="groupMenu.canEdit" type="button" @click="openEditGroupDialog">
                <el-icon><EditPen /></el-icon>
                <span>重命名</span>
              </button>
              <button v-if="groupMenu.canDelete" type="button" class="danger" @click="deleteGroup">
                <el-icon><Delete /></el-icon>
                <span>删除分组</span>
              </button>
            </div>
          </Teleport>
        </aside>

        <section class="alert-main-panel">
          <div class="alert-table-shell">
            <el-table class="alert-table" :data="rows" v-loading="loading" empty-text="暂无内置模板">
              <el-table-column label="模板名称" min-width="210">
                <template #default="{ row }">
                  <div class="alert-main-text">{{ row.name || '-' }}</div>
                  <div class="alert-muted-text query-summary">{{ querySummary(row) }}</div>
                </template>
              </el-table-column>
              <el-table-column label="组件" width="120">
                <template #default="{ row }">{{ componentLabel(row) }}</template>
              </el-table-column>
              <el-table-column label="采集器" width="150">
                <template #default="{ row }">
                  <span class="alert-code">{{ collectorLabel(row) }}</span>
                </template>
              </el-table-column>
              <el-table-column label="对象域" width="100">
                <template #default="{ row }">
                  <el-tag effect="light">{{ resourceTypeLabel(row.resourceType) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="分类" prop="category" width="100" />
              <el-table-column label="数据源" width="135">
                <template #default="{ row }">{{ row.datasourceType || '-' }}</template>
              </el-table-column>
              <el-table-column label="来源" width="100">
                <template #default="{ row }">
                  <el-tag :type="isBuiltinTemplate(row) ? 'primary' : 'success'" effect="light">{{ sourceLabel(row) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="操作" width="120" fixed="right">
                <template #default="{ row }">
                  <div class="alert-operation-links template-operation-links">
                    <button type="button" class="alert-operation-link alert-operation-warning" @click="openDetailDrawer(row)">查看</button>
                    <button type="button" class="alert-operation-link" @click="openEditDialog(row)">编辑</button>
                  </div>
                </template>
              </el-table-column>
            </el-table>
          </div>

          <div class="alert-pagination">
            <el-pagination
              v-model:current-page="query.page"
              v-model:page-size="query.pageSize"
              :page-sizes="[10, 20, 50, 100]"
              :total="total"
              layout="total, sizes, prev, pager, next, jumper"
              @size-change="loadList"
              @current-change="loadList"
            />
          </div>
        </section>
      </div>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="1060px" class="alert-dialog template-edit-dialog" modal-class="alert-template-dialog-overlay" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="82px" class="template-watch-form">
        <section class="template-editor-header">
          <el-form-item label="模板名称" prop="name" class="template-title-field">
            <el-input v-model="form.name" placeholder="请输入模板名称" />
          </el-form-item>
          <el-form-item label="模板分组" prop="groupPath" class="template-group-field">
            <el-cascader
              v-model="form.groupPath"
              :options="groupCascaderOptions"
              :props="groupCascaderProps"
              class="full-select"
              filterable
              placeholder="Linux / node_exporter / memory"
              @change="handleGroupPathChange"
            />
          </el-form-item>
          <el-form-item label="模板说明" class="template-description-field">
            <el-input v-model="form.description" placeholder="用于列表中说明这个模板的用途" />
          </el-form-item>
        </section>

        <section class="template-editor-core">
          <div class="template-datasource-tabs">
            <button
              v-for="item in templateDatasourceOptions"
              :key="item.value"
              type="button"
              class="template-datasource-card"
              :class="{ active: form.datasourceType === item.value }"
              @click="form.datasourceType = item.value"
            >
              <span>{{ item.label }}</span>
              <small>{{ item.value }}</small>
            </button>
          </div>

          <div class="template-core-grid">
            <section class="template-editor-panel template-query-panel">
              <div class="template-panel-head">
                <h3>查询语句</h3>
              </div>
              <el-form-item prop="queryTemplate" class="template-query-field">
                <el-input v-model="form.queryTemplate" type="textarea" :rows="4" placeholder='up{job=~"node.*|node_exporter"} == 0' />
              </el-form-item>
            </section>

            <section class="template-editor-panel template-condition-panel">
              <div class="template-panel-head">
                <h3>告警条件</h3>
                <div class="template-inline-control">
                  <span>执行频率</span>
                  <el-input-number v-model="form.defaultEvalIntervalSeconds" :min="1" controls-position="right" />
                  <span>秒</span>
                </div>
              </div>
              <div class="template-condition-list">
                <div v-for="item in form.conditionRows" :key="item.priority" class="template-condition-row">
                  <span class="template-priority-badge" :class="`priority-${item.priority.toLowerCase()}`">{{ item.priority }}</span>
                  <el-select v-model="item.operator" class="condition-operator-select">
                    <el-option v-for="option in conditionOperatorOptions" :key="option.value" :label="option.label" :value="option.value" />
                  </el-select>
                  <el-input-number v-model="item.value" controls-position="right" class="condition-value-input" />
                  <span class="condition-duration-label">持续</span>
                  <el-input-number v-model="item.duration" :min="0" controls-position="right" class="condition-duration-input" />
                  <span class="condition-duration-label">秒</span>
                </div>
              </div>
            </section>

            <section class="template-editor-panel template-copy-panel">
              <div class="template-panel-head">
                <h3>告警详情</h3>
              </div>
              <div class="template-copy-grid">
                <el-form-item label="告警标题" class="alert-form-full">
                  <el-input v-model="form.alertSummary" placeholder="例如：主机 Exporter 离线" />
                </el-form-item>
                <el-form-item label="告警描述" class="alert-form-full">
                  <el-input
                    v-model="form.alertDescription"
                    type="textarea"
                    :rows="2"
                    placeholder="例如：实例 {{ $labels.instance }} Exporter 无法抓取。"
                  />
                </el-form-item>
              </div>
            </section>
          </div>
        </section>
      </el-form>
      <template #footer>
        <div class="alert-dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="saving" @click="submitForm">确认</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="groupDialogVisible" :title="groupDialogTitle" width="520px" class="alert-dialog template-group-dialog" modal-class="alert-template-dialog-overlay" destroy-on-close>
      <el-form ref="groupFormRef" :model="groupForm" :rules="groupRules" label-width="92px">
        <div class="alert-form-grid template-group-form">
          <el-form-item label="分组层级">
            <el-input :model-value="groupLevelLabel(groupForm.level)" disabled />
          </el-form-item>
          <el-form-item label="父级">
            <el-input :model-value="groupForm.parentName || '根节点'" disabled />
          </el-form-item>
          <el-form-item label="分组名称" prop="name">
            <el-input v-model="groupForm.name" placeholder="例如 Linux / node_exporter / cpu" />
          </el-form-item>
          <el-form-item label="排序">
            <el-input-number v-model="groupForm.sort" :min="0" controls-position="right" class="full-input-number" />
          </el-form-item>
          <el-form-item label="描述" class="alert-form-full">
            <el-input v-model="groupForm.description" type="textarea" :rows="2" placeholder="请输入分组说明" />
          </el-form-item>
        </div>
      </el-form>
      <template #footer>
        <div class="alert-dialog-footer">
          <el-button @click="groupDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="groupSaving" @click="submitGroupForm">确认</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="prometheusImportVisible" title="导入 Prometheus 模板" width="980px" class="alert-dialog prometheus-import-dialog" modal-class="alert-template-dialog-overlay" destroy-on-close>
      <section class="prometheus-import-toolbar">
        <el-select
          v-model="prometheusImportQuery.type"
          class="prometheus-import-type"
          filterable
          placeholder="导入类型"
          @change="handlePrometheusImportTypeChange"
        >
          <el-option
            v-for="item in prometheusImportTypes"
            :key="prometheusImportTypeValue(item)"
            :label="prometheusImportTypeLabel(item)"
            :value="prometheusImportTypeValue(item)"
          />
        </el-select>
        <el-input-number v-model="prometheusImportQuery.limit" :min="1" :max="500" controls-position="right" class="prometheus-import-limit" />
        <el-button type="primary" size="small" class="alert-search-button" :loading="prometheusPreviewLoading" @click="previewPrometheusTemplates">
          <el-icon><Search /></el-icon>
          <span>查询</span>
        </el-button>
      </section>

      <div class="prometheus-import-table-shell">
        <el-table
          class="alert-table"
          :data="prometheusPreviewRows"
          v-loading="prometheusPreviewLoading"
          empty-text="暂无可导入模板"
          @selection-change="selectedPrometheusRows = $event"
        >
          <el-table-column type="selection" width="44" :selectable="row => !row.exists" />
          <el-table-column label="模板名称" min-width="230">
            <template #default="{ row }">
              <div class="alert-main-text">{{ row.name || '-' }}</div>
              <div class="alert-muted-text query-summary">{{ row.description || row.queryTemplate || '-' }}</div>
            </template>
          </el-table-column>
          <el-table-column label="分组" min-width="210">
            <template #default="{ row }">
              <span class="alert-code">{{ prometheusTemplateGroupText(row) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="级别" width="92">
            <template #default="{ row }">
              <el-tag effect="light">{{ severityPriorityText(row.defaultSeverity) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="90">
            <template #default="{ row }">
              <el-tag :type="row.exists ? 'info' : 'success'" effect="light">{{ row.exists ? '已存在' : '可导入' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="查询模板" min-width="260">
            <template #default="{ row }">
              <span class="alert-code">{{ row.queryTemplate || '-' }}</span>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <template #footer>
        <div class="alert-dialog-footer">
          <span class="import-selected-count">已选择 {{ selectedPrometheusRows.length }} 条</span>
          <span class="import-total-count">共 {{ prometheusPreviewTotal }} 条</span>
          <el-button @click="prometheusImportVisible = false">取消</el-button>
          <el-button type="primary" :loading="prometheusImporting" @click="importPrometheusTemplates">确认导入</el-button>
        </div>
      </template>
    </el-dialog>

    <el-drawer v-model="detailVisible" title="模板详情" size="560px" class="alert-detail-drawer" modal-class="alert-template-detail-overlay">
      <div v-if="currentRow" class="alert-detail-stack">
        <section class="alert-detail-panel">
          <h4>{{ currentRow.name }}</h4>
          <div class="alert-detail-row"><span>对象域</span><span>{{ resourceTypeLabel(currentRow.resourceType) }}</span></div>
          <div class="alert-detail-row"><span>组件</span><span>{{ componentLabel(currentRow) }}</span></div>
          <div class="alert-detail-row"><span>采集器</span><span>{{ collectorLabel(currentRow) }}</span></div>
          <div class="alert-detail-row"><span>分类</span><span>{{ currentRow.category || '-' }}</span></div>
        </section>
        <section class="alert-detail-panel">
          <h4>查询条件</h4>
          <pre class="alert-json-preview">{{ currentRow.queryTemplate || '-' }}</pre>
          <div class="alert-detail-row"><span>触发条件</span><span>{{ currentRow.defaultConditionOperator || '>' }} {{ currentRow.defaultConditionValue ?? '-' }}</span></div>
          <div class="alert-detail-row"><span>执行频率</span><span>{{ currentRow.defaultEvalIntervalSeconds ?? '-' }}s</span></div>
          <div class="alert-detail-row"><span>持续时间</span><span>{{ currentRow.defaultForDurationSeconds ?? '-' }}s</span></div>
        </section>
        <section class="alert-detail-panel">
          <h4>告警详情</h4>
          <div class="alert-detail-row"><span>告警标题</span><span>{{ annotationText(currentRow, 'summary') }}</span></div>
          <div class="alert-detail-row"><span>告警描述</span><span>{{ annotationText(currentRow, 'description') }}</span></div>
        </section>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ArrowRight, Delete, EditPen, Plus, RefreshRight, Search, Upload } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import alertApi from '@/api/alertmanager'
import {
  compactObject,
  isSuccess,
  normalizeListResult,
  unwrapData
} from './alertCommon'
import './alert-pages.css'

const openFolderIcon = new URL('@/assets/image/打开文件夹.svg', import.meta.url).href
const closedFolderIcon = new URL('@/assets/image/关闭文件夹.svg', import.meta.url).href
const scriptIcon = new URL('@/assets/image/脚本.svg', import.meta.url).href
const scriptIconStyle = computed(() => ({ '--script-icon-url': `url(${scriptIcon})` }))

const fallbackComponentTree = [
  {
    label: 'Linux',
    component: 'Linux',
    resourceType: 'host',
    collectors: [
      {
        label: 'node_exporter',
        collector: 'node_exporter',
        datasourceType: 'prometheus',
        categories: [
          { label: 'CPU', category: 'cpu' },
          { label: '内存', category: 'memory' },
          { label: '磁盘', category: 'disk' },
          { label: '可用性', category: 'availability' },
          { label: '负载', category: 'load' },
          { label: '网络', category: 'network' }
        ]
      }
    ]
  },
  {
    label: 'Kubernetes',
    component: 'Kubernetes',
    resourceType: 'k8s',
    collectors: [
      {
        label: 'kube-state-metrics',
        collector: 'kube-state-metrics',
        datasourceType: 'prometheus',
        categories: [
          { label: 'Node', category: 'node' },
          { label: 'Pod', category: 'pod' },
          { label: 'Deployment', category: 'deployment' },
          { label: 'PVC', category: 'pvc' },
          { label: 'Event', category: 'event' }
        ]
      }
    ]
  },
  {
    label: 'MySQL',
    component: 'MySQL',
    resourceType: 'mysql',
    collectors: [
      {
        label: 'mysqld_exporter',
        collector: 'mysqld_exporter',
        datasourceType: 'prometheus',
        categories: [
          { label: '连接数', category: 'connection' },
          { label: '文件数', category: 'file' },
          { label: '主从复制', category: 'replication' },
          { label: '慢查询', category: 'slow_query' },
          { label: '锁等待', category: 'lock_wait' }
        ]
      }
    ]
  },
  {
    label: 'VictoriaMetrics',
    component: 'VictoriaMetrics',
    resourceType: 'victoriametrics',
    collectors: [
      {
        label: 'victoriametrics',
        collector: 'victoriametrics',
        datasourceType: 'victoriametrics',
        categories: [
          { label: '写入', category: 'ingest' },
          { label: '查询', category: 'query' },
          { label: '存储', category: 'storage' }
        ]
      }
    ]
  },
  {
    label: 'Prometheus',
    component: 'Prometheus',
    resourceType: 'prometheus',
    collectors: [
      {
        label: 'prometheus',
        collector: 'prometheus',
        datasourceType: 'prometheus',
        categories: [
          { label: '采集', category: 'scrape' },
          { label: '规则', category: 'rule' },
          { label: '存储', category: 'storage' }
        ]
      }
    ]
  }
]

const resourceTypeOptions = [
  { label: '主机', value: 'host' },
  { label: 'K8S', value: 'k8s' },
  { label: 'MySQL', value: 'mysql' },
  { label: 'Redis', value: 'redis' },
  { label: 'VictoriaMetrics', value: 'victoriametrics' },
  { label: 'Prometheus', value: 'prometheus' }
]

const conditionOperatorOptions = [
  { label: '>', value: '>' },
  { label: '>=', value: '>=' },
  { label: '<', value: '<' },
  { label: '<=', value: '<=' },
  { label: '==', value: '==' },
  { label: '!=', value: '!=' }
]

const conditionPriorityMeta = [
  { priority: 'P0', severity: 'critical', operator: '>', value: 80, duration: 300 },
  { priority: 'P1', severity: 'warning', operator: '>', value: 75, duration: 300 },
  { priority: 'P2', severity: 'info', operator: '>', value: 70, duration: 300 }
]

const fallbackTemplateDatasourceTypes = [
  { label: '平台内置告警源', value: 'builtin', sort: 0 },
  { label: 'Prometheus', value: 'prometheus', sort: 10 },
  { label: 'VictoriaMetrics', value: 'victoriametrics', sort: 20 }
]
const prioritySeverityMap = conditionPriorityMeta.reduce((result, item) => {
  result[item.priority] = item.severity
  return result
}, {})
const severityPriorityMap = conditionPriorityMeta.reduce((result, item) => {
  result[item.severity] = item.priority
  return result
}, {})

const loading = ref(false)
const saving = ref(false)
const groupSaving = ref(false)
const prometheusPreviewLoading = ref(false)
const prometheusImporting = ref(false)
const dialogVisible = ref(false)
const detailVisible = ref(false)
const groupDialogVisible = ref(false)
const prometheusImportVisible = ref(false)
const editingId = ref(null)
const rows = ref([])
const allRows = ref([])
const prometheusImportTypes = ref([])
const prometheusPreviewRows = ref([])
const prometheusPreviewTotal = ref(0)
const selectedPrometheusRows = ref([])
const componentTree = ref([])
const datasourceTypeRows = ref([])
const total = ref(0)
const allTemplateTotal = ref(0)
const formRef = ref()
const groupFormRef = ref()
const currentRow = ref(null)
const dialogTitle = ref('新增模板')
const groupDialogTitle = ref('新增分组')
const originalTemplate = ref({})
const query = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  component: '',
  collector: '',
  resourceType: '',
  category: '',
  datasourceType: '',
  source: ''
})
const form = reactive({})
const groupForm = reactive({})
const prometheusImportQuery = reactive({
  type: '',
  keyword: '',
  component: '',
  collector: '',
  category: '',
  datasourceType: 'prometheus',
  limit: 200
})
const groupMenu = reactive({
  visible: false,
  x: 0,
  y: 0,
  level: 'root',
  nextLevel: 'component',
  groupId: 0,
  groupName: '',
  groupValue: '',
  groupDescription: '',
  groupSort: 100,
  groupEnabled: 1,
  parentId: 0,
  parentName: '',
  currentParentId: 0,
  currentParentName: '',
  component: '',
  collector: '',
  resourceType: '',
  canCreate: true,
  canEdit: false,
  canDelete: false
})
const expandedComponents = reactive({})
const expandedCollectors = reactive({})
const categoryCount = computed(() => componentTree.value.reduce((total, item) => {
  return total + item.collectors.reduce((sum, collector) => sum + collector.categories.length, 0)
}, 0))
const groupCascaderProps = {
  emitPath: true,
  checkStrictly: true
}
const groupCascaderOptions = computed(() => componentTree.value.map(component => ({
  label: component.label,
  value: component.component,
  children: (component.collectors || []).map(collector => ({
    label: collector.label,
    value: collector.collector,
    children: (collector.categories || []).map(category => ({
      label: category.label,
      value: category.category
    }))
  }))
})))
const templateDatasourceOptions = computed(() => {
  const source = datasourceTypeRows.value.length ? datasourceTypeRows.value : fallbackTemplateDatasourceTypes
  return source
    .map(normalizeDatasourceType)
    .filter(item => item.value && templateDatasourceTypeSet.has(item.value))
    .sort((left, right) => Number(left.sort ?? 100) - Number(right.sort ?? 100))
})

const templateDatasourceTypeSet = new Set(fallbackTemplateDatasourceTypes.map(item => item.value))

const formRules = {
  name: [{ required: true, message: '请输入模板名称', trigger: 'blur' }],
  groupPath: [{ required: true, message: '请选择模板分组', trigger: 'change' }],
  queryTemplate: [{ required: true, message: '请输入查询模板', trigger: 'blur' }]
}

const groupRules = {
  name: [{ required: true, message: '请输入分组名称', trigger: 'blur' }]
}

const collectorKey = (component, collector) => `${component.component}:${collector.collector}`

const loadList = async () => {
  loading.value = true
  try {
    const response = await alertApi.getBuiltinRules(compactObject(query))
    const result = normalizeListResult(response)
    rows.value = result.list
    total.value = result.total
  } finally {
    loading.value = false
  }
}

const loadTreeCounts = async () => {
  const [ruleResponse, treeResponse] = await Promise.all([
    alertApi.getBuiltinRules({ page: 1, pageSize: 1000 }),
    alertApi.getBuiltinRuleGroupTree()
  ])
  const result = normalizeListResult(ruleResponse)
  allRows.value = result.list
  componentTree.value = normalizeGroupTree(treeResponse)
  if (!componentTree.value.length) componentTree.value = fallbackComponentTree
  const treeTotal = componentTree.value.reduce((sum, component) => sum + componentTemplateCount(component), 0)
  allTemplateTotal.value = treeTotal || result.total
}

const loadDatasourceTypes = async () => {
  try {
    const response = await alertApi.getDatasourceTypes()
    datasourceTypeRows.value = normalizeDatasourceTypes(response)
  } catch (error) {
    datasourceTypeRows.value = []
  }
}

const loadPrometheusImportTypes = async () => {
  const response = await alertApi.getPrometheusBuiltinRuleImportTypes()
  prometheusImportTypes.value = normalizeListResult(response).list
}

const normalizeDatasourceTypes = response => {
  const payload = unwrapData(response)
  if (Array.isArray(payload)) return payload
  if (Array.isArray(payload.types)) return payload.types
  if (Array.isArray(payload.list)) return payload.list
  if (Array.isArray(payload.options)) return payload.options
  return normalizeListResult(response).list
}

const normalizeDatasourceType = item => {
  if (typeof item === 'string') {
    return {
      label: datasourceTypeLabel(item),
      value: item
    }
  }
  const value = String(item?.value || item?.type || item?.datasourceType || item?.code || '').trim().toLowerCase()
  return {
    ...item,
    label: item?.label || item?.name || datasourceTypeLabel(value),
    value,
    sort: item?.sort
  }
}

const datasourceTypeLabel = value => {
  const labelMap = {
    builtin: '平台内置告警源',
    prometheus: 'Prometheus',
    victoriametrics: 'VictoriaMetrics'
  }
  return labelMap[value] || value || '-'
}

const normalizeGroupTree = response => {
  const payload = unwrapData(response)
  const list = Array.isArray(payload)
    ? payload
    : (payload.tree || payload.list || payload.items || payload.children || payload.components || [])
  if (!Array.isArray(list)) return []
  return list.map(normalizeComponentNode).filter(item => item.component)
}

const normalizeComponentNode = item => {
  const component = nodeValue(item, ['component', 'value', 'code', 'name'])
  const collectors = nodeChildren(item, ['collectors', 'children']).map(collector => normalizeCollectorNode(collector, component))
  return {
    ...item,
    id: item?.id,
    label: nodeLabel(item, component),
    component,
    resourceType: item?.resourceType || item?.resource_type || '',
    collectors
  }
}

const normalizeCollectorNode = (item, component) => {
  const collector = nodeValue(item, ['collector', 'value', 'code', 'name'])
  const categories = nodeChildren(item, ['categories', 'children']).map(category => normalizeCategoryNode(category, component, collector))
  return {
    ...item,
    id: item?.id,
    label: nodeLabel(item, collector),
    component,
    collector,
    datasourceType: item?.datasourceType || item?.datasource_type || 'prometheus',
    categories
  }
}

const normalizeCategoryNode = (item, component, collector) => {
  const category = nodeValue(item, ['category', 'value', 'code', 'name'])
  return {
    ...item,
    id: item?.id,
    label: nodeLabel(item, category),
    component,
    collector,
    category
  }
}

const nodeChildren = (item, keys) => {
  for (const key of keys) {
    if (Array.isArray(item?.[key])) return item[key]
  }
  return []
}

const nodeValue = (item, keys) => {
  for (const key of keys) {
    if (item?.[key] !== undefined && item?.[key] !== null && item?.[key] !== '') return String(item[key])
  }
  return ''
}

const nodeLabel = (item, fallback) => item?.label || item?.name || fallback || '-'

const handleSearch = () => {
  query.page = 1
  loadList()
}

const handleReset = () => {
  Object.assign(query, {
    page: 1,
    pageSize: 10,
    keyword: '',
    component: '',
    collector: '',
    resourceType: '',
    category: '',
    datasourceType: '',
    source: ''
  })
  Object.keys(expandedComponents).forEach(key => { expandedComponents[key] = false })
  Object.keys(expandedCollectors).forEach(key => { expandedCollectors[key] = false })
  loadList()
}

const selectAllTemplates = () => {
  Object.assign(query, { component: '', collector: '', resourceType: '', category: '' })
  handleSearch()
}

const handleComponentClick = component => {
  const nextExpanded = !expandedComponents[component.component]
  Object.keys(expandedComponents).forEach(key => { expandedComponents[key] = false })
  Object.keys(expandedCollectors).forEach(key => { expandedCollectors[key] = false })
  expandedComponents[component.component] = nextExpanded
  Object.assign(query, {
    component: component.component,
    collector: '',
    resourceType: component.resourceType,
    category: ''
  })
  handleSearch()
}

const handleCollectorClick = (component, collector) => {
  const key = collectorKey(component, collector)
  const nextExpanded = !expandedCollectors[key]
  Object.keys(expandedComponents).forEach(item => { expandedComponents[item] = false })
  Object.keys(expandedCollectors).forEach(item => { expandedCollectors[item] = false })
  expandedComponents[component.component] = true
  expandedCollectors[key] = nextExpanded
  Object.assign(query, {
    component: component.component,
    collector: collector.collector,
    resourceType: component.resourceType,
    category: ''
  })
  handleSearch()
}

const handleCategoryClick = (component, collector, category) => {
  Object.assign(query, {
    component: component.component,
    collector: collector.collector,
    resourceType: component.resourceType,
    category: category.category
  })
  handleSearch()
}

const handleGroupPathChange = value => {
  const [componentValue, collectorValue, categoryValue] = value || []
  form.component = componentValue || ''
  form.collector = collectorValue || ''
  form.category = categoryValue || ''
  const component = findComponentConfig(form.component)
  if (component?.resourceType) form.resourceType = component.resourceType
  const collector = findCollectorConfig(form.component, form.collector)
  if (collector?.datasourceType && !form.datasourceType) form.datasourceType = collector.datasourceType
}

const findComponentConfig = component => {
  return componentTree.value.find(item => normalizeKey(item.component) === normalizeKey(component))
}

const findCollectorConfig = (component, collector) => {
  const targetComponent = findComponentConfig(component)
  return targetComponent?.collectors?.find(item => normalizeKey(item.collector) === normalizeKey(collector))
}

const buildGroupPath = row => {
  const component = row?.component || inferComponent(row)
  const collector = row?.collector || row?.collectorName || inferCollector(row)
  const category = row?.category || ''
  return [component, collector, category].filter(Boolean)
}

const selectedGroupPath = () => {
  return [query.component, query.collector, query.category].filter(Boolean)
}

const defaultGroupPath = row => {
  const path = buildGroupPath(row)
  if (path.length) return path
  const selectedPath = selectedGroupPath()
  return selectedPath.length ? selectedPath : ['Linux', 'node_exporter', 'cpu']
}

const parseConditionRows = row => {
  const ruleConfig = safeJson(row?.ruleConfig, {})
  const queries = Array.isArray(ruleConfig.queries) ? ruleConfig.queries : []
  const rows = conditionPriorityMeta.map(item => ({ ...item }))
  queries.forEach(query => {
    const severity = query?.severity || query?.level
    const priority = query?.priority || severityPriorityMap[severity]
    const target = rows.find(item => item.priority === priority || item.severity === severity)
    if (!target) return
    const condition = parsePromQlCondition(query.prom_ql || query.promQl || query.expr || '')
    target.operator = query.operator || condition.operator || target.operator
    target.value = Number(query.value ?? query.threshold ?? condition.value ?? target.value)
    target.duration = Number(query.forDurationSeconds ?? query.for_duration_seconds ?? query.duration ?? row?.defaultForDurationSeconds ?? target.duration)
  })
  if (!queries.length) {
    const defaultRow = rows.find(item => item.severity === row?.defaultSeverity) || rows[1]
    defaultRow.operator = row?.defaultConditionOperator || defaultRow.operator
    defaultRow.value = Number(row?.defaultConditionValue ?? defaultRow.value)
    defaultRow.duration = Number(row?.defaultForDurationSeconds ?? defaultRow.duration)
  }
  return rows
}

const parsePromQlCondition = value => {
  const match = String(value || '').trim().match(/(>=|<=|==|!=|>|<)\s*(-?\d+(?:\.\d+)?)\s*$/)
  if (!match) return {}
  return {
    operator: match[1],
    value: Number(match[2])
  }
}

const stripTrailingCondition = value => String(value || '').trim().replace(/\s+(>=|<=|==|!=|>|<)\s*-?\d+(?:\.\d+)?\s*$/, '')

const buildRuleConfig = original => {
  const baseRuleConfig = safeJson(original?.ruleConfig, {})
  const baseQuery = stripTrailingCondition(form.queryTemplate) || form.queryTemplate || ''
  const queries = (form.conditionRows || []).map(item => ({
    prom_ql: baseQuery ? `${baseQuery} ${item.operator || '>'} ${item.value ?? 0}` : '',
    severity: prioritySeverityMap[item.priority] || item.severity || 'warning',
    forDurationSeconds: Number(item.duration ?? 0)
  })).filter(item => item.prom_ql)
  return {
    ...baseRuleConfig,
    queries
  }
}

const defaultConditionRow = () => {
  return (form.conditionRows || []).find(item => item.priority === 'P1') ||
    conditionPriorityMeta[1]
}

const resetForm = row => {
  Object.keys(form).forEach(key => delete form[key])
  originalTemplate.value = row ? { ...row } : {}
  const annotations = safeJson(row?.annotations, {})
  const firstRuleQuery = firstRuleConfigQuery(row)
  const groupPath = defaultGroupPath(row)
  Object.assign(form, {
    component: '',
    collector: '',
    resourceType: '',
    category: '',
    name: '',
    description: '',
    datasourceType: 'prometheus',
    queryTemplate: '',
    defaultConditionOperator: '>',
    defaultConditionValue: 85,
    defaultEvalIntervalSeconds: 60,
    defaultForDurationSeconds: 300,
    defaultSeverity: 'warning',
    enabled: 1,
    alertSummary: '',
    alertDescription: '',
    conditionRows: parseConditionRows(row),
    ...row,
    component: row?.component || inferComponent(row) || query.component || '',
    collector: row?.collector || row?.collectorName || inferCollector(row) || query.collector || '',
    category: row?.category || query.category || '',
    groupPath,
    queryTemplate: row?.queryTemplate || stripTrailingCondition(firstRuleQuery) || '',
    conditionRows: parseConditionRows(row),
    alertSummary: annotations.summary || row?.alertSummary || row?.summary || '',
    alertDescription: annotations.description || row?.alertDescription || ''
  })
  handleGroupPathChange(groupPath)
}

const openCreateDialog = () => {
  editingId.value = null
  dialogTitle.value = '新增模板'
  resetForm()
  dialogVisible.value = true
}

const openEditDialog = row => {
  editingId.value = row.id
  dialogTitle.value = '编辑模板'
  resetForm(row)
  dialogVisible.value = true
}

const openDetailDrawer = row => {
  currentRow.value = row
  detailVisible.value = true
}

const openPrometheusImportDialog = async () => {
  if (!prometheusImportTypes.value.length) await loadPrometheusImportTypes()
  const firstType = prometheusImportTypes.value[0]
  Object.assign(prometheusImportQuery, {
    type: prometheusImportTypeValue(firstType),
    keyword: prometheusImportTypeValue(firstType),
    component: firstType?.component || '',
    collector: firstType?.collector || '',
    category: '',
    datasourceType: 'prometheus',
    limit: 200
  })
  prometheusPreviewRows.value = []
  prometheusPreviewTotal.value = 0
  selectedPrometheusRows.value = []
  prometheusImportVisible.value = true
}

const handlePrometheusImportTypeChange = value => {
  const target = prometheusImportTypes.value.find(item => prometheusImportTypeValue(item) === value)
  Object.assign(prometheusImportQuery, {
    type: value || '',
    keyword: value || '',
    component: target?.component || '',
    collector: target?.collector || '',
    category: ''
  })
}

const previewPrometheusTemplates = async () => {
  prometheusPreviewLoading.value = true
  try {
    const response = await alertApi.previewImportPrometheusBuiltinRules(compactObject({
      keyword: prometheusImportQuery.keyword,
      component: prometheusImportQuery.component,
      collector: prometheusImportQuery.collector,
      category: prometheusImportQuery.category,
      datasourceType: prometheusImportQuery.datasourceType,
      limit: prometheusImportQuery.limit
    }))
    const result = normalizePrometheusPreviewResult(response)
    prometheusPreviewRows.value = result.list
    prometheusPreviewTotal.value = result.total
    selectedPrometheusRows.value = []
  } finally {
    prometheusPreviewLoading.value = false
  }
}

const importPrometheusTemplates = async () => {
  const rules = selectedPrometheusRows.value.filter(row => !row.exists)
  if (!rules.length) {
    ElMessage.warning('请选择要导入的 Prometheus 模板')
    return
  }
  prometheusImporting.value = true
  try {
    const response = await alertApi.importPrometheusBuiltinRules({ rules })
    if (isSuccess(response)) {
      const result = unwrapData(response)
      const imported = result?.imported ?? rules.length
      const skipped = result?.skipped ?? 0
      ElMessage.success(`导入完成：新增 ${imported} 条，跳过 ${skipped} 条`)
      prometheusImportVisible.value = false
      await loadTreeCounts()
      loadList()
    }
  } finally {
    prometheusImporting.value = false
  }
}

const buildPayload = () => {
  const original = originalTemplate.value || {}
  const conditionRow = defaultConditionRow()
  const annotations = {
    ...safeJson(original.annotations, {}),
    summary: form.alertSummary,
    description: form.alertDescription
  }
  const ruleConfig = buildRuleConfig(original)
  return compactObject({
    ...original,
    component: form.component,
    collector: form.collector,
    resourceType: form.resourceType,
    category: form.category,
    name: form.name,
    description: form.description,
    datasourceType: form.datasourceType,
    queryTemplate: form.queryTemplate,
    defaultConditionOperator: conditionRow?.operator || form.defaultConditionOperator,
    defaultConditionValue: conditionRow?.value ?? form.defaultConditionValue,
    defaultEvalIntervalSeconds: form.defaultEvalIntervalSeconds,
    defaultForDurationSeconds: conditionRow?.duration ?? form.defaultForDurationSeconds,
    defaultSeverity: conditionRow?.severity || form.defaultSeverity,
    enabled: 1,
    annotations,
    ruleConfig
  })
}

const submitForm = async () => {
  await formRef.value?.validate()
  saving.value = true
  try {
    const response = editingId.value
      ? await alertApi.updateBuiltinRule(editingId.value, buildPayload())
      : await alertApi.createBuiltinRule(buildPayload())
    if (isSuccess(response)) {
      ElMessage.success(editingId.value ? '修改成功' : '创建成功')
      dialogVisible.value = false
      loadList()
      loadTreeCounts()
    }
  } catch (error) {
    if (error?.message === 'JSON 格式不正确') ElMessage.error(error.message)
  } finally {
    saving.value = false
  }
}

const hideGroupMenu = () => {
  groupMenu.visible = false
}

const openGroupMenu = (event, level, context = {}) => {
  const component = context.component || {}
  const collector = context.collector || {}
  const category = context.category || {}
  const nodeMap = {
    root: {},
    component,
    collector,
    category
  }
  const currentNode = nodeMap[level] || {}
  const nextLevel = level === 'root' ? 'component' : level === 'component' ? 'collector' : 'category'
  const parentId = level === 'root' ? 0 : level === 'component' ? component.id : collector.id
  const parentName = level === 'root' ? '' : level === 'component' ? component.label : collector.label
  const currentParentId = level === 'collector' ? component.id : level === 'category' ? collector.id : 0
  const currentParentName = level === 'collector' ? component.label : level === 'category' ? collector.label : ''
  const groupId = level === 'root' ? 0 : currentNode.id
  const groupValue = level === 'component'
    ? component.component
    : level === 'collector'
      ? collector.collector
      : level === 'category'
        ? category.category
        : ''
  const canCreate = level !== 'category' && (level === 'root' || Boolean(parentId))
  const canManage = level !== 'root' && Boolean(groupId)
  if (!canCreate && !canManage) {
    ElMessage.warning('分组树尚未加载到分组 ID，请刷新后重试')
    return
  }
  Object.assign(groupMenu, {
    visible: true,
    x: event.clientX + 6,
    y: event.clientY + 6,
    level,
    nextLevel,
    groupId: groupId || 0,
    groupName: currentNode.label || currentNode.name || groupValue || '',
    groupValue,
    groupDescription: currentNode.description || '',
    groupSort: Number(currentNode.sort ?? 100),
    groupEnabled: currentNode.enabled ?? 1,
    parentId: parentId || 0,
    parentName: parentName || '',
    currentParentId: currentParentId || 0,
    currentParentName: currentParentName || '',
    component: component.component || category.component || '',
    collector: collector.collector || category.collector || '',
    resourceType: component.resourceType || '',
    canCreate,
    canEdit: canManage,
    canDelete: canManage
  })
}

const openCreateGroupDialog = level => {
  if (level !== 'component' && !groupMenu.parentId) {
    ElMessage.warning('分组树尚未加载到父级 ID，请刷新后重试')
    hideGroupMenu()
    return
  }
  Object.keys(groupForm).forEach(key => delete groupForm[key])
  Object.assign(groupForm, {
    parentId: groupMenu.parentId || 0,
    parentName: groupMenu.parentName || '',
    level,
    name: '',
    value: '',
    resourceType: groupMenu.resourceType || '',
    description: '',
    sort: 100,
    enabled: 1,
    component: groupMenu.component || '',
    collector: groupMenu.collector || ''
  })
  groupDialogTitle.value = `新增${groupLevelLabel(level)}`
  hideGroupMenu()
  groupDialogVisible.value = true
}

const openEditGroupDialog = () => {
  if (!groupMenu.groupId) {
    ElMessage.warning('分组树尚未加载到分组 ID，请刷新后重试')
    hideGroupMenu()
    return
  }
  Object.keys(groupForm).forEach(key => delete groupForm[key])
  Object.assign(groupForm, {
    id: groupMenu.groupId,
    parentId: groupMenu.currentParentId || 0,
    parentName: groupMenu.currentParentName || '根节点',
    level: groupMenu.level,
    name: groupMenu.groupName,
    value: groupMenu.groupValue,
    resourceType: groupMenu.resourceType || '',
    description: groupMenu.groupDescription || '',
    sort: groupMenu.groupSort ?? 100,
    enabled: groupMenu.groupEnabled ?? 1,
    component: groupMenu.component || '',
    collector: groupMenu.collector || ''
  })
  groupDialogTitle.value = `编辑${groupLevelLabel(groupMenu.level)}`
  hideGroupMenu()
  groupDialogVisible.value = true
}

const deleteGroup = async () => {
  const target = {
    id: groupMenu.groupId,
    name: groupMenu.groupName,
    level: groupMenu.level,
    component: groupMenu.component,
    collector: groupMenu.collector,
    value: groupMenu.groupValue
  }
  hideGroupMenu()
  if (!target.id) {
    ElMessage.warning('分组树尚未加载到分组 ID，请刷新后重试')
    return
  }
  try {
    await ElMessageBox.confirm(`确认删除分组「${target.name || target.value}」？`, '删除分组', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消'
    })
    const response = await alertApi.deleteBuiltinRuleGroup(target.id)
    if (isSuccess(response)) {
      ElMessage.success('分组删除成功')
      resetDeletedGroupQuery(target)
      await loadTreeCounts()
      loadList()
    }
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') throw error
  }
}

const resetDeletedGroupQuery = target => {
  const shouldReset = target.level === 'component'
    ? query.component === target.value
    : target.level === 'collector'
      ? query.component === target.component && query.collector === target.value
      : query.component === target.component && query.collector === target.collector && query.category === target.value
  if (!shouldReset) return
  Object.assign(query, {
    page: 1,
    component: '',
    collector: '',
    resourceType: '',
    category: ''
  })
}

const submitGroupForm = async () => {
  await groupFormRef.value?.validate()
  groupSaving.value = true
  try {
    const groupValue = groupForm.name
    const payload = compactObject({
      parentId: groupForm.parentId,
      level: groupForm.level,
      name: groupForm.name,
      value: groupValue,
      description: groupForm.description,
      sort: groupForm.sort,
      enabled: groupForm.enabled
    })
    const response = groupForm.id
      ? await alertApi.updateBuiltinRuleGroup(groupForm.id, payload)
      : await alertApi.createBuiltinRuleGroup(payload)
    if (isSuccess(response)) {
      ElMessage.success(groupForm.id ? '分组修改成功' : '分组创建成功')
      groupDialogVisible.value = false
      await loadTreeCounts()
      loadList()
    }
  } finally {
    groupSaving.value = false
  }
}

const groupLevelLabel = level => ({
  component: '一级分组',
  collector: '二级分组',
  category: '三级分组'
}[level] || '分组')

const safeJson = (value, fallback) => {
  if (!value) return fallback
  if (typeof value === 'object') return value
  try {
    return JSON.parse(value)
  } catch (error) {
    return fallback
  }
}

const firstRuleConfigQuery = row => {
  const ruleConfig = safeJson(row?.ruleConfig, {})
  const queries = Array.isArray(ruleConfig.queries) ? ruleConfig.queries : []
  const first = queries[0] || {}
  return first.prom_ql || first.promQl || first.expr || ''
}

const annotationText = (row, key) => {
  const annotations = safeJson(row?.annotations, {})
  return annotations[key] || '-'
}

const isBuiltinTemplate = row => {
  const source = String(row?.source || row?.templateSource || '').toLowerCase()
  return source === 'builtin' || source === 'n9e' || source === 'integration' || source === 'platform' || Number(row?.builtin) === 1 || row?.isBuiltin === true
}

const sourceLabel = row => {
  const source = String(row?.source || '').toLowerCase()
  if (source === 'n9e') return 'N9E'
  if (source === 'platform') return '平台'
  if (source === 'custom') return '自定义'
  if (isBuiltinTemplate(row)) return '内置'
  return row?.source || '自定义'
}

const componentLabel = row => {
  const component = row?.component || inferComponent(row)
  const target = componentTree.value.find(item => normalizeKey(item.component) === normalizeKey(component))
  return target?.label || component || '-'
}

const collectorLabel = row => row?.collector || row?.collectorName || inferCollector(row) || '-'

const inferComponent = row => {
  if (row?.resourceType === 'host') return 'Linux'
  if (row?.resourceType === 'k8s') return 'Kubernetes'
  if (row?.resourceType === 'mysql') return 'MySQL'
  if (row?.resourceType === 'victoriametrics') return 'VictoriaMetrics'
  if (row?.resourceType === 'prometheus') return 'Prometheus'
  return row?.resourceType || ''
}

const inferCollector = row => {
  if (row?.resourceType === 'host') return 'node_exporter'
  if (row?.resourceType === 'k8s') return 'kube-state-metrics'
  if (row?.resourceType === 'mysql') return 'mysqld_exporter'
  return row?.collector || ''
}

const normalizeKey = value => String(value || '').trim().toLowerCase()
const rowComponentValue = row => normalizeKey(row?.component || inferComponent(row))
const rowCollectorValue = row => normalizeKey(row?.collector || row?.collectorName || inferCollector(row))

const templateCountValue = item => {
  const value = item?.templateCount ?? item?.template_count ?? item?.count ?? item?.total
  const count = Number(value)
  return Number.isFinite(count) ? count : null
}

const componentTemplateCount = component => {
  const count = templateCountValue(component)
  if (count !== null) return count
  return allRows.value.filter(row => rowComponentValue(row) === normalizeKey(component.component)).length
}

const collectorTemplateCount = (component, collector) => {
  const count = templateCountValue(collector)
  if (count !== null) return count
  return allRows.value.filter(row => {
    return rowComponentValue(row) === normalizeKey(component.component) && rowCollectorValue(row) === normalizeKey(collector.collector)
  }).length
}

const categoryTemplateCount = (component, collector, category) => {
  const count = templateCountValue(category)
  if (count !== null) return count
  return allRows.value.filter(row => {
    return rowComponentValue(row) === normalizeKey(component.component) &&
      rowCollectorValue(row) === normalizeKey(collector.collector) &&
      normalizeKey(row.category) === normalizeKey(category.category)
  }).length
}

const querySummary = row => {
  const text = String(row?.queryTemplate || '')
  if (!text) return row?.description || '-'
  return text.length > 92 ? `${text.slice(0, 92)}...` : text
}

const normalizePrometheusPreviewResult = response => {
  const result = normalizeListResult(response)
  if (result.list.length || result.total) return result

  const responseBody = response?.data && typeof response.data === 'object' ? response.data : response
  const payload = responseBody?.data && typeof responseBody.data === 'object' ? responseBody.data : responseBody
  if (Array.isArray(payload)) return { list: payload, total: payload.length }

  const list = payload?.list || payload?.rows || payload?.records || payload?.items || []
  return {
    list: Array.isArray(list) ? list : [],
    total: Number(payload?.total ?? payload?.count ?? (Array.isArray(list) ? list.length : 0))
  }
}

const prometheusTemplateGroupText = row => {
  return [row?.component, row?.collector, row?.category].filter(Boolean).join(' / ') || '-'
}

const prometheusImportTypeValue = item => item?.value || item?.type || item?.code || ''

const prometheusImportTypeLabel = item => {
  const value = prometheusImportTypeValue(item)
  const path = [item?.component, item?.collector].filter(Boolean).join(' / ')
  return path ? `${value} - ${path}` : (item?.label || item?.name || value || '-')
}

const severityPriorityText = value => {
  const severity = String(value || '').trim().toLowerCase()
  return severityPriorityMap[severity] || value || '-'
}

const resourceTypeLabel = value => ({
  host: '主机',
  k8s: 'K8S',
  mysql: 'MySQL',
  redis: 'Redis',
  victoriametrics: 'VictoriaMetrics',
  prometheus: 'Prometheus'
}[value] || value || '-')

onMounted(async () => {
  await Promise.all([loadList(), loadTreeCounts(), loadDatasourceTypes()])
})
</script>

<style scoped>
:global(.home-container .el-main:has(.alert-template-page)) {
  --ops-shell-gutter: 10px;
  --ops-page-padding: 7px;
  --ops-module-gap: 8px;
  padding-top: var(--ops-shell-gutter);
  padding-right: var(--ops-shell-gutter);
  padding-left: var(--ops-shell-gutter);
  overflow-y: hidden;
}

.alert-template-page {
  --ops-text-heading: #0f172a;
  --ops-text-primary: #172033;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-bg-surface: #ffffff;
  --ops-bg-surface-soft: #fbfdff;
  --ops-bg-subtle: #f3f7fc;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  box-sizing: border-box;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: var(--ops-module-gap, 8px);
  height: calc(100vh - 124px);
  max-height: none;
  padding: var(--ops-page-padding, 7px);
  overflow: hidden;
  border-radius: 0;
  background: transparent;
  color: var(--ops-text-primary);
}

.ops-page-header {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 12px;
  min-width: 0;
  padding: 8px 12px;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 10px 28px rgba(80, 124, 180, 0.1);
}

.ops-page-header__icon {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  place-items: center;
  border: 0;
  background: transparent;
}

.ops-page-header__icon img {
  width: 30px;
  height: 30px;
  object-fit: contain;
}

.ops-page-header__content {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.ops-page-header h1 {
  flex: 0 0 auto;
  margin: 0;
  color: var(--ops-text-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: 0;
  white-space: nowrap;
}

.ops-page-header__divider {
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  background: var(--ops-border-soft);
}

.ops-page-header__content p {
  margin: 0;
  overflow: hidden;
  color: var(--ops-text-muted);
  font-size: 12px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.template-header-summary {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 10px;
  color: var(--ops-text-secondary);
}

.template-header-summary span {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  white-space: nowrap;
}

.template-header-summary em {
  color: var(--ops-text-muted);
  font-size: 12px;
  font-style: normal;
}

.template-header-summary strong {
  color: var(--ops-text-heading);
  font-family: Consolas, Monaco, monospace;
  font-size: 16px;
  line-height: 1;
}

.template-header-summary > i {
  width: 1px;
  height: 18px;
  background: var(--ops-border-soft);
}

.alert-template-page :deep(.alert-center-card.el-card) {
  position: relative;
  display: flex;
  min-width: 0;
  min-height: 0;
  height: 100%;
  flex-direction: column;
  overflow: hidden !important;
  border: 1px solid var(--ops-border);
  border-radius: 12px !important;
  background: var(--ops-bg-surface);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
  backdrop-filter: none;
}

.alert-template-page :deep(.alert-center-card::before) {
  display: none;
}

.alert-template-page :deep(.alert-center-card .el-card__body) {
  box-sizing: border-box;
  display: flex;
  min-height: 0;
  height: 100%;
  flex: 1;
  flex-direction: column;
  padding: 8px 10px 10px;
  overflow: hidden;
  border-radius: 0;
}

.alert-template-page .alert-search-section {
  margin-bottom: 8px;
  padding: 8px 10px 2px;
  border: 1px solid var(--ops-border-soft);
  border-radius: 10px;
  background: var(--ops-bg-subtle);
}

.alert-template-page :deep(.alert-search-form .el-form-item) {
  margin-right: 10px;
  margin-bottom: 6px;
}

.alert-template-page :deep(.alert-search-form .el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

.alert-template-page :deep(.alert-search-form .el-input__wrapper),
.alert-template-page :deep(.alert-search-form .el-select__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 7px;
  background: #ffffff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.alert-template-page :deep(.alert-search-form .el-input__wrapper:hover),
.alert-template-page :deep(.alert-search-form .el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.alert-template-page :deep(.alert-search-form .el-input__wrapper.is-focus),
.alert-template-page :deep(.alert-search-form .el-select__wrapper.is-focused) {
  box-shadow: 0 0 0 1px #409eff inset;
}

.alert-template-page :deep(.template-toolbar-button.el-button) {
  height: 30px;
  min-height: 30px;
  padding: 0 10px;
  border-radius: 8px;
  color: #ffffff;
  font-size: 12px;
  box-shadow: none;
}

.alert-template-page :deep(.alert-search-button.el-button) {
  border-color: #409eff;
  background: #409eff;
}

.alert-template-page :deep(.alert-reset-button.el-button),
.alert-template-page :deep(.template-import-button.el-button) {
  border-color: #e6a23c;
  background: #e6a23c;
}

.alert-template-page :deep(.alert-create-button.el-button) {
  border-color: #67c23a;
  background: #67c23a;
}

.alert-template-page :deep(.template-toolbar-button .el-icon) {
  font-size: 14px;
}

.alert-template-page :deep(.el-button:hover),
.alert-template-page :deep(.el-button:active) {
  transform: none;
}

.template-market-layout {
  grid-template-columns: 228px minmax(0, 1fr);
  gap: 8px;
}

.template-side-panel {
  padding: 10px;
  border-radius: 10px;
  border-color: var(--ops-border-soft);
  background: var(--ops-bg-surface-soft);
}

.alert-template-page .alert-table-shell,
.alert-template-page :deep(.alert-table-shell .el-table),
.alert-template-page :deep(.alert-table-shell .el-table__inner-wrapper) {
  border-radius: 8px;
}

.alert-template-page .alert-table-shell {
  border-color: var(--ops-border-soft);
  background: #ffffff;
}

.alert-template-page :deep(.alert-table th.el-table__cell) {
  height: 40px;
  padding: 6px 0;
  background: #f3f7fc;
  color: #334155;
  font-size: 12px;
  font-weight: 700;
}

.alert-template-page :deep(.alert-table td.el-table__cell) {
  height: 50px;
  padding: 6px 0;
}

.alert-template-page :deep(.alert-table .el-table__row:hover > td.el-table__cell) {
  background: #f7faff;
}

.alert-template-page .alert-pagination {
  padding: 8px 2px 0;
}

.full-select,
.full-input-number {
  width: 100%;
}

.template-tree {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.template-domain-group,
.template-collector-group {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.template-tree-node {
  width: 100%;
  min-height: 28px;
  display: flex;
  align-items: center;
  gap: 6px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--aiops-text-secondary);
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  text-align: left;
  cursor: pointer;
  transition: background-color 0.18s ease, color 0.18s ease;
}

.template-tree-node:hover {
  background: rgba(76, 139, 245, 0.06);
}

.template-tree-node.active {
  border-color: #4c8bf5;
  background: #4c8bf5;
  color: #fff;
  box-shadow: 0 6px 14px rgba(76, 139, 245, 0.2);
}

.root-node,
.component-node {
  padding: 4px 8px;
}

.collector-node {
  min-height: 27px;
  padding: 3px 8px 3px 14px;
}

.category-node {
  min-height: 26px;
  padding: 3px 8px 3px 26px;
  color: #647084;
  font-size: 12px;
}

.template-collectors,
.template-children {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.template-collectors {
  margin: 0 0 2px;
}

.template-children {
  margin-bottom: 2px;
}

.node-toggle,
.node-spacer {
  width: 8px;
  height: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.node-toggle {
  color: #7d8aa1;
}

.node-toggle .el-icon {
  font-size: 10px;
  transition: transform 0.18s ease;
}

.node-toggle .el-icon.expanded {
  transform: rotate(90deg);
}

.node-svg-icon {
  width: 15px;
  height: 15px;
  display: inline-block;
  object-fit: contain;
  flex-shrink: 0;
}

.node-script {
  width: 14px;
  height: 14px;
  background: #35bf7f;
  -webkit-mask: var(--script-icon-url) center / contain no-repeat;
  mask: var(--script-icon-url) center / contain no-repeat;
}

.node-label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-count {
  color: #7b9af0;
  font-size: 11px;
  font-weight: 600;
  flex-shrink: 0;
}

.template-tree-node.active .node-count,
.template-tree-node.active .node-code {
  color: rgba(255, 255, 255, 0.9);
}

.node-code {
  color: #7f8aa0;
  font-size: 11px;
  flex-shrink: 0;
}

.template-tree-menu {
  position: fixed;
  z-index: 9999;
  min-width: 126px;
  padding: 6px;
  border-radius: 10px;
  border: 1px solid rgba(223, 231, 242, 0.94);
  background: #fff;
  box-shadow: 0 14px 34px rgba(15, 23, 42, 0.14);
}

.template-tree-menu button {
  width: 100%;
  height: 30px;
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 0 9px;
  border: none;
  border-radius: 7px;
  background: transparent;
  color: var(--aiops-text-secondary, #667085);
  font: inherit;
  font-size: 12px;
  font-weight: 500;
  text-align: left;
  cursor: pointer;
}

.template-tree-menu button .el-icon {
  font-size: 14px;
}

.template-tree-menu button:hover {
  background: rgba(76, 139, 245, 0.08);
  color: var(--aiops-brand-active, #4d79d8);
}

.template-tree-menu button.danger {
  color: var(--aiops-danger, #ef4444);
}

.template-tree-menu button.danger:hover {
  background: rgba(245, 63, 63, 0.08);
  color: var(--aiops-danger, #ef4444);
}

.query-summary {
  max-width: 360px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.template-operation-links {
  gap: 10px;
}

.prometheus-import-toolbar {
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  min-width: 0;
}

.prometheus-import-type {
  width: 320px;
  flex: 0 0 320px;
}

.prometheus-import-limit {
  width: 112px;
  flex: 0 0 112px;
}

.prometheus-import-table-shell {
  height: 430px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: 10px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  background: rgba(255, 255, 255, 0.82);
}

.import-selected-count,
.import-total-count {
  color: var(--aiops-text-secondary);
  font-size: 13px;
  font-weight: 600;
}

.import-total-count {
  margin-right: auto;
}

.template-form-section {
  margin-bottom: 10px;
  padding: 10px 14px 2px;
  border-radius: 10px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  background: rgba(255, 255, 255, 0.48);
}

.template-form-section h3 {
  margin: 0 0 8px;
  color: var(--aiops-text-primary);
  font-size: 14px;
  line-height: 20px;
}

.template-edit-dialog :deep(.el-dialog__body) {
  max-height: calc(100vh - 138px);
  padding: 4px 16px 8px;
  overflow: hidden;
}

.template-edit-dialog :deep(.el-dialog__header) {
  padding: 14px 16px 8px;
  border-bottom: none;
  background: transparent;
}

.template-edit-dialog :deep(.el-dialog__footer) {
  padding: 6px 16px 10px;
}

.template-edit-dialog :deep(.el-dialog__title) {
  font-size: 16px;
  line-height: 22px;
}

.template-watch-form {
  display: grid;
  gap: 6px;
}

.template-watch-form :deep(.el-form-item) {
  margin-bottom: 0;
}

.template-watch-form :deep(.el-form-item__label) {
  height: 28px;
  line-height: 28px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
}

.template-watch-form :deep(.el-input__wrapper),
.template-watch-form :deep(.el-select__wrapper),
.template-watch-form :deep(.el-cascader .el-input__wrapper),
.template-watch-form :deep(.el-input-number .el-input__wrapper) {
  min-height: 30px;
  border-radius: 8px;
}

.template-watch-form :deep(.el-textarea__inner) {
  border-radius: 8px;
  font-family: Monaco, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.48;
  resize: none;
}

.template-editor-header {
  display: grid;
  grid-template-columns: minmax(220px, 1.05fr) minmax(280px, 1.3fr) 132px;
  gap: 5px 10px;
  align-items: start;
  padding: 7px 10px;
  border-radius: 10px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  background: rgba(255, 255, 255, 0.58);
}

.template-description-field {
  grid-column: 1 / -1;
}

.template-editor-core {
  display: grid;
  gap: 6px;
}

.template-datasource-tabs {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 6px;
}

.template-datasource-card {
  min-height: 36px;
  padding: 4px 10px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.58);
  color: var(--aiops-text-secondary);
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.18s ease, background-color 0.18s ease, color 0.18s ease;
}

.template-datasource-card span,
.template-datasource-card small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.template-datasource-card span {
  color: var(--aiops-text-primary);
  font-size: 12px;
  font-weight: 700;
}

.template-datasource-card small {
  margin-top: 2px;
  color: var(--aiops-text-muted);
  font-size: 11px;
}

.template-datasource-card:hover,
.template-datasource-card.active {
  border-color: rgba(76, 139, 245, 0.36);
  background: rgba(76, 139, 245, 0.08);
}

.template-datasource-card.active span,
.template-datasource-card.active small {
  color: var(--aiops-brand-active);
}

.template-core-grid {
  display: grid;
  gap: 6px;
}

.template-editor-panel {
  min-width: 0;
  padding: 7px 10px;
  border-radius: 10px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  background: rgba(255, 255, 255, 0.58);
}

.template-query-panel {
  padding-bottom: 8px;
}

.template-panel-head {
  min-height: 22px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 4px;
}

.template-panel-head h3 {
  margin: 0;
  color: var(--aiops-text-primary);
  font-size: 14px;
  line-height: 20px;
}

.template-inline-control {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--aiops-text-muted);
  font-size: 12px;
  white-space: nowrap;
}

.template-inline-control :deep(.el-input-number) {
  width: 92px;
}

.template-query-field :deep(.el-textarea__inner) {
  height: 64px !important;
  min-height: 64px !important;
  padding: 9px 10px;
  overflow-y: hidden;
  background: rgba(247, 250, 255, 0.82);
}

.template-condition-list {
  display: grid;
  gap: 4px;
}

.template-condition-row {
  min-width: 0;
  display: grid;
  grid-template-columns: 42px 72px minmax(120px, 1fr) 34px 88px 20px;
  gap: 6px;
  align-items: center;
  padding: 3px 6px;
  border-radius: 10px;
  border: 1px solid rgba(223, 231, 242, 0.72);
  background: rgba(247, 250, 255, 0.6);
}

.template-priority-badge {
  height: 22px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  color: #ffffff;
  font-size: 12px;
  font-weight: 800;
}

.priority-p0 {
  background: #ef6b73;
}

.priority-p1 {
  background: #f3b24b;
}

.priority-p2 {
  background: #4c8bf5;
}

.condition-operator-select {
  width: 72px;
}

.condition-value-input,
.condition-duration-input {
  width: 100%;
}

.condition-duration-label {
  color: var(--aiops-text-muted);
  font-size: 12px;
  white-space: nowrap;
}

.template-copy-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.2fr);
  gap: 6px 10px;
}

.template-copy-panel :deep(.el-textarea__inner) {
  height: 44px !important;
  min-height: 44px !important;
  padding: 7px 10px;
  overflow-y: hidden;
}

.template-copy-grid :deep(.el-form-item__label) {
  width: 70px !important;
}

.template-copy-grid :deep(.el-form-item__content) {
  margin-left: 70px !important;
}

.template-group-dialog :deep(.el-dialog__body) {
  padding: 14px 18px 4px;
}

.template-group-dialog :deep(.el-dialog__header) {
  padding: 15px 18px 10px;
}

.template-group-dialog :deep(.el-dialog__footer) {
  padding: 8px 18px 16px;
}

.template-compact-form :deep(.el-form-item) {
  margin-bottom: 10px;
}

.template-compact-form :deep(.el-form-item__label) {
  height: 32px;
  line-height: 32px;
  font-size: 13px;
}

.template-compact-form :deep(.el-input__wrapper),
.template-compact-form :deep(.el-select__wrapper),
.template-compact-form :deep(.el-input-number .el-input__wrapper) {
  min-height: 32px;
  border-radius: 8px;
}

.template-compact-form :deep(.el-textarea__inner) {
  min-height: 72px !important;
  border-radius: 8px;
}

.template-compact-form .alert-form-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0 18px;
}

.template-compact-form .alert-form-full {
  grid-column: 1 / -1;
}

.template-group-form {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.template-group-form :deep(.el-form-item) {
  margin-bottom: 10px;
}

.template-group-form :deep(.el-form-item__label) {
  height: 32px;
  line-height: 32px;
  font-size: 13px;
}

.template-group-form :deep(.el-input__wrapper),
.template-group-form :deep(.el-select__wrapper),
.template-group-form :deep(.el-input-number .el-input__wrapper),
.template-group-form :deep(.el-textarea__inner) {
  border-radius: 8px;
}

:global(.alert-template-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 32px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: #ffffff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.alert-template-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.alert-template-dialog-overlay .el-dialog__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.alert-template-dialog-overlay .el-dialog__body) {
  box-sizing: border-box;
  max-height: calc(100vh - 160px);
  padding: 16px 18px;
  overflow: auto;
  color: #53657d;
}

:global(.alert-template-dialog-overlay .template-edit-dialog .el-dialog__body) {
  max-height: calc(100vh - 138px);
  padding: 4px 16px 8px;
  overflow: hidden;
}

:global(.alert-template-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #ffffff;
}

:global(.alert-template-dialog-overlay .template-edit-dialog .el-dialog__footer) {
  padding: 8px 16px 12px;
}

:global(.alert-template-dialog-overlay .el-dialog__footer .el-button) {
  min-height: 34px;
  border-radius: 8px;
}

:global(.alert-template-dialog-overlay .el-button:hover),
:global(.alert-template-dialog-overlay .el-button:active) {
  transform: none;
}

:global(.alert-template-dialog-overlay .el-dialog:not(.template-edit-dialog) .el-input__wrapper),
:global(.alert-template-dialog-overlay .el-dialog:not(.template-edit-dialog) .el-select__wrapper),
:global(.alert-template-dialog-overlay .el-dialog:not(.template-edit-dialog) .el-cascader .el-input__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.alert-template-dialog-overlay .el-dialog:not(.template-edit-dialog) .el-textarea__inner) {
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.alert-template-dialog-overlay .prometheus-import-toolbar .alert-search-button) {
  height: 30px;
  min-height: 30px;
  padding: 0 10px;
  border-color: #409eff;
  border-radius: 8px;
  background: #409eff;
  color: #ffffff;
  font-size: 12px;
  box-shadow: none;
}

:global(.alert-template-detail-overlay .el-drawer) {
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: 15px 0 0 15px !important;
  background: #ffffff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.alert-template-detail-overlay .el-drawer__header) {
  min-height: 54px;
  margin: 0;
  padding: 14px 16px 12px;
  border-bottom: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.alert-template-detail-overlay .el-drawer__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.alert-template-detail-overlay .el-drawer__body) {
  padding: 14px 16px;
  background: #ffffff;
}

:global(.alert-template-detail-overlay .alert-detail-stack) {
  gap: 8px;
}

:global(.alert-template-detail-overlay .alert-detail-panel) {
  padding: 12px;
  border-radius: 10px;
  background: #fbfdff;
}

:global(.alert-template-detail-overlay .alert-json-preview) {
  border-radius: 8px;
}

@media (max-width: 1200px) {
  .ops-page-header {
    flex-wrap: wrap;
  }

  .template-header-summary {
    width: 100%;
    padding-left: 54px;
  }
}

@media (max-width: 1024px) {
  .template-side-panel {
    max-height: 260px;
  }
}

@media (max-width: 768px) {
  .ops-page-header {
    gap: 8px;
  }

  .ops-page-header__content {
    flex-wrap: wrap;
    gap: 2px 8px;
  }

  .ops-page-header__divider {
    display: none;
  }

  .ops-page-header__content p {
    width: 100%;
  }

  .template-header-summary {
    padding-left: 0;
  }

  .alert-template-page :deep(.alert-search-form .el-form-item) {
    width: 100%;
    margin-right: 0;
  }

  .alert-template-page :deep(.alert-search-form .el-form-item:last-child .el-form-item__content) {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
}
</style>
