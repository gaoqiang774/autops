<template>
  <div class="database-sql-page">
    <section class="sql-workbench">
      <header class="sql-hero">
        <div class="hero-copy">
          <span class="ops-page-header__icon" aria-hidden="true">
            <img :src="databaseSvg" alt="">
          </span>
          <div class="ops-page-header__content">
            <h1>数据库 SQL 工作台</h1>
            <span class="ops-page-header__divider" aria-hidden="true"></span>
            <p>集中管理数据库连接、对象查询、结果集与审计记录。</p>
          </div>
        </div>
        <div class="hero-actions">
          <el-button
            class="success-action"
            type="success"
            :icon="Connection"
            :loading="pingLoading"
            :disabled="!selectedAsset"
            @click="handlePingAsset"
          >
            连接测试
          </el-button>
          <el-button
            class="warning-action ops-refresh-action"
            type="warning"
            :icon="Refresh"
            :loading="assetLoading"
            @click="reloadWorkspace"
          >
            刷新
          </el-button>
        </div>
      </header>

      <section class="console-layout">
        <aside class="datasource-pane">
          <div class="datasource-head">
            <div>
              <strong>数据源</strong>
              <span>{{ selectedAsset ? getAssetTitle(selectedAsset) : '请选择连接' }}</span>
            </div>
            <div class="datasource-head-actions">
              <el-tag size="small" effect="light" round>{{ assets.length }} 个</el-tag>
              <el-tooltip content="刷新" placement="top">
                <el-button
                  class="datasource-refresh-button"
                  :icon="Refresh"
                  :loading="assetLoading"
                  aria-label="刷新数据源"
                  @click="reloadWorkspace"
                />
              </el-tooltip>
            </div>
          </div>

          <div class="tree-search">
            <el-input
              v-model="assetKeyword"
              clearable
              placeholder="搜索数据源 / 表"
              :prefix-icon="Search"
              @keyup.enter="loadAssets"
            />
          </div>

          <el-tree
            ref="treeRef"
            v-loading="assetLoading || treeLoading"
            node-key="key"
            lazy
            :data="datasourceTree"
            :load="loadTreeNode"
            :props="treeProps"
            :expand-on-click-node="true"
            class="datasource-tree"
            @node-click="handleTreeNodeClick"
          >
            <template #default="{ data }">
              <span
                class="tree-node"
                :class="[`node-${data.type}`, {
                  'active-path': isActiveTreePath(data),
                  'active-current': isActiveTreeCurrent(data)
                }]"
              >
                <img
                  v-if="getTreeIcon(data)"
                  class="tree-svg-icon"
                  :src="getTreeIcon(data)"
                  alt=""
                />
                <el-icon v-else><component :is="getNodeIcon(data.type)" /></el-icon>
                <span class="tree-label">{{ getTreeNodeName(data) }}</span>
                <span v-if="getTreeNodeMeta(data)" class="tree-meta" :class="`tree-meta-${data.type}`">
                  {{ getTreeNodeMeta(data) }}
                </span>
                <span v-if="data.primaryKey" class="tree-mini-tag">PK</span>
              </span>
            </template>
          </el-tree>
        </aside>

        <main class="sql-console">
          <div class="window-tabs">
            <div
              v-for="tab in workspaceTabs"
              :key="tab.key"
              class="window-tab"
              :class="{ active: tab.key === activeWindowKey }"
            >
              <button type="button" class="window-tab-main" @click="setActiveWindow(tab.key)">
                <el-icon class="tab-type-icon">
                  <component :is="tab.type === 'sql' ? Document : Grid" />
                </el-icon>
                <span class="window-tab-title">{{ tab.title }}</span>
                <span v-if="tab.dirty" class="dirty-dot" title="存在未保存修改"></span>
              </button>
              <button
                v-if="workspaceTabs.length > 1"
                type="button"
                class="tab-close"
                :aria-label="`关闭 ${tab.title}`"
                @click="closeWorkspaceTab(tab.key)"
              >
                <el-icon><Close /></el-icon>
              </button>
            </div>
            <el-tooltip content="新建 SQL 窗口" placement="top">
              <button type="button" class="add-tab-button" aria-label="新建 SQL 窗口" @click="createSqlWindow">
                <el-icon><Plus /></el-icon>
              </button>
            </el-tooltip>
          </div>

          <div class="connection-strip">
            <div class="connection-main">
              <span class="status-dot" :class="{ online: selectedAsset }"></span>
              <strong>{{ selectedAsset ? getEngineLabel(getAssetEngine(selectedAsset)) : '未选择' }}</strong>
              <span v-if="selectedAsset" class="mono">{{ getAssetTitle(selectedAsset) }}</span>
              <span class="separator"></span>
              <el-icon><Coin /></el-icon>
              <span>{{ activeDatabase || '请选择库' }}</span>
              <span v-if="activeTable" class="separator"></span>
              <span v-if="activeTable" class="mono">{{ activeTable }}</span>
            </div>
            <div class="query-actions">
              <el-button
                v-if="!isRedisMode"
                link
                class="ai-tool-link"
                :icon="MagicStick"
                @click="openAiAssistant"
              >
                AI 助手
              </el-button>
              <el-button v-if="!isRedisMode" link class="tool-link" @click="formatSql">格式化</el-button>
              <el-button link class="tool-link" @click="openSnippetDialog">片段</el-button>
              <el-button link class="tool-link" @click="saveSnippet">保存</el-button>
              <el-tooltip content="最多导出 10000 行数据" placement="top">
                <el-button v-if="!isRedisMode" link class="tool-link" :loading="exportingData" @click="downloadSql">下载</el-button>
              </el-tooltip>
              <el-button
                class="execute-button"
                type="primary"
                :icon="VideoPlay"
                :loading="sqlExecuting"
                @click="executeCurrentSql(false)"
              >
                {{ isRedisMode ? '执行命令' : '执行' }}
              </el-button>
            </div>
          </div>

          <section v-if="activeWindow?.type === 'sql'" class="sql-window">
            <div class="editor-shell monaco-shell">
              <CodeEditor
                v-model="activeWindow.sql"
                :language="editorLanguage"
                theme="aiops-sql-light"
                height="127px"
                :completion-items="sqlCompletionItems"
                :placeholder="editorPlaceholder"
                word-wrap="off"
                @update:model-value="markActiveDirty"
                @execute="executeCurrentSql"
              />
            </div>
            <ResultDock
              :active="activeWindow.resultActive"
              :columns="activeWindow.resultColumns"
              :rows="activeWindow.resultRows"
              :logs="activeWindow.logs"
              :history="sqlHistory"
              @change="activeWindow.resultActive = $event"
              @reuse="reuseHistory"
            />
          </section>

          <section v-else-if="activeWindow?.type === 'object'" class="object-window">
            <div class="object-query-row">
              <div class="object-editor-shell monaco-shell">
                <CodeEditor
                  v-model="activeWindow.sql"
                :language="editorLanguage"
                theme="aiops-sql-light"
                height="118px"
                :completion-items="sqlCompletionItems"
                :placeholder="editorPlaceholder"
                word-wrap="off"
                @update:model-value="markActiveDirty"
                @execute="executeCurrentSql"
                />
              </div>
            </div>

            <div v-if="!isRedisObjectWindow" class="object-data-head">
              <div class="object-actions">
                <el-tooltip content="仅支持 SQL 查询" placement="top">
                  <el-button
                    class="icon-action success-icon-action"
                    :icon="Plus"
                    disabled
                    aria-label="新增数据行"
                  />
                </el-tooltip>
                <el-tooltip content="重置条件并刷新" placement="top">
                  <el-button
                    class="icon-action warning-icon-action"
                    :icon="Refresh"
                    :loading="dataLoading"
                    aria-label="重置条件并刷新"
                    @click="resetFilter"
                  />
                </el-tooltip>
              </div>
            </div>

            <el-tabs v-model="activeWindow.activeView" class="object-tabs" @tab-change="handleObjectTabChange">
              <el-tab-pane :label="isRedisObjectWindow ? 'Key 内容' : '数据'" name="data">
                <div class="filter-bar">
                  <el-form v-if="!isRedisObjectWindow" :inline="true" :model="filterForm" class="filter-form">
                    <el-form-item label="字段">
                      <el-select v-model="filterForm.column" clearable filterable placeholder="选择字段">
                        <el-option v-for="column in dataState.columns" :key="column.name" :label="column.name" :value="column.name" />
                      </el-select>
                    </el-form-item>
                    <el-form-item label="条件">
                      <el-select v-model="filterForm.operator" placeholder="条件">
                        <el-option v-for="operator in filterOperators" :key="operator" :label="operator" :value="operator" />
                      </el-select>
                    </el-form-item>
                    <el-form-item label="值">
                      <el-input
                        v-model="filterForm.value"
                        clearable
                        placeholder="输入筛选值"
                        @keyup.enter="applyFilter"
                      />
                    </el-form-item>
                    <el-form-item>
                      <el-button
                        class="primary-action compact"
                        type="primary"
                        :icon="Search"
                        :loading="dataLoading"
                        @click="applyFilter"
                      >
                        查询
                      </el-button>
                      <el-tooltip content="最多导出 10000 行数据" placement="top">
                        <el-button
                          class="export-query-button compact"
                          type="warning"
                          :icon="Download"
                          :loading="exportingData"
                          @click="exportCurrentObjectData('query')"
                        >
                          下载
                        </el-button>
                      </el-tooltip>
                    </el-form-item>
                  </el-form>
                  <div v-else class="redis-tip">
                    请选择 key 或输入只读 Redis 命令，例如 GET key、HGETALL key、SCAN 0 MATCH prefix:* COUNT 100。
                  </div>
                </div>

                <DataGrid
                  :loading="dataLoading"
                  :columns="visibleDataColumns"
                  :rows="dataState.rows"
                  :editable="canMutateRows"
                  @edit="openRowDialog('edit', $event)"
                  @delete="deleteRow"
                />
                <div class="pagination-row">
                  <span>{{ isRedisObjectWindow ? 'Redis Key 内容按后端返回字段动态渲染' : (dataState.editable ? '当前对象支持行级操作' : '当前对象只读，已隐藏危险操作') }}</span>
                  <el-pagination
                    v-model:current-page="queryState.page"
                    v-model:page-size="queryState.pageSize"
                    :page-sizes="[20, 50, 100, 200]"
                    layout="total, sizes, prev, pager, next"
                    :total="dataState.total"
                    @size-change="handleDataPageChange"
                    @current-change="handleDataPageChange"
                  />
                </div>
              </el-tab-pane>

              <el-tab-pane v-if="!isRedisObjectWindow" label="字段" name="columns">
                <div class="table-shell">
                  <el-table v-loading="metaLoading.columns" :data="columnMeta" class="data-table" height="100%">
                    <el-table-column prop="name" label="字段名" min-width="180" />
                    <el-table-column prop="type" label="类型" min-width="140" />
                    <el-table-column label="约束" min-width="180">
                      <template #default="{ row }">
                        <div class="tag-line">
                          <el-tag v-if="row.primaryKey" size="small" type="success" effect="light" round>主键</el-tag>
                          <el-tag v-if="row.autoIncrement" size="small" type="warning" effect="light" round>自增</el-tag>
                          <el-tag :type="row.nullable ? 'info' : 'danger'" size="small" effect="light" round>
                            {{ row.nullable ? '可空' : '非空' }}
                          </el-tag>
                        </div>
                      </template>
                    </el-table-column>
                    <el-table-column prop="defaultValue" label="默认值" min-width="130" show-overflow-tooltip />
                    <el-table-column prop="comment" label="备注" min-width="180" show-overflow-tooltip />
                  </el-table>
                </div>
              </el-tab-pane>

              <el-tab-pane v-if="!isRedisObjectWindow" label="索引" name="indexes">
                <div class="table-shell">
                  <el-table v-loading="metaLoading.indexes" :data="indexMeta" class="data-table" height="100%">
                    <el-table-column prop="name" label="索引名" min-width="180" show-overflow-tooltip />
                    <el-table-column prop="columns" label="字段" min-width="220" show-overflow-tooltip>
                      <template #default="{ row }">{{ formatIndexColumns(row) }}</template>
                    </el-table-column>
                    <el-table-column label="类型" min-width="140">
                      <template #default="{ row }">
                        <el-tag :type="row.unique ? 'success' : 'info'" size="small" effect="light" round>
                          {{ row.unique ? '唯一索引' : row.type || '普通索引' }}
                        </el-tag>
                      </template>
                    </el-table-column>
                    <el-table-column prop="comment" label="备注" min-width="180" show-overflow-tooltip />
                  </el-table>
                </div>
              </el-tab-pane>

              <el-tab-pane v-if="!isRedisObjectWindow" label="DDL" name="ddl">
                <div v-loading="metaLoading.ddl" class="code-panel">
                  <pre>{{ ddlText || '暂无 DDL 信息' }}</pre>
                </div>
              </el-tab-pane>

              <el-tab-pane v-if="!isRedisObjectWindow" label="审计" name="audit">
                <div class="table-shell">
                  <el-table v-loading="auditLoading" :data="auditState.rows" class="data-table" height="100%">
                    <el-table-column prop="createdAt" label="时间" min-width="170" />
                    <el-table-column prop="operator" label="操作人" min-width="110" />
                    <el-table-column prop="action" label="动作" min-width="100">
                      <template #default="{ row }">
                        <el-tag size="small" effect="light" round>{{ row.action }}</el-tag>
                      </template>
                    </el-table-column>
                    <el-table-column prop="success" label="结果" min-width="90">
                      <template #default="{ row }">
                        <el-tag :type="row.success === false ? 'danger' : 'success'" size="small" effect="light" round>
                          {{ row.success === false ? '失败' : '成功' }}
                        </el-tag>
                      </template>
                    </el-table-column>
                    <el-table-column prop="affectedRows" label="影响行" min-width="90" />
                    <el-table-column prop="costMs" label="耗时(ms)" min-width="100" />
                    <el-table-column prop="errorMessage" label="错误信息" min-width="180" show-overflow-tooltip />
                    <el-table-column label="操作" width="90" fixed="right" align="center">
                      <template #default="{ row }">
                        <el-button link type="primary" @click="openAuditDetail(row)">详情</el-button>
                      </template>
                    </el-table-column>
                  </el-table>
                </div>
              </el-tab-pane>
            </el-tabs>
          </section>

          <section v-else class="empty-workspace">
            <div class="empty-mark">
              <el-icon><DataBoard /></el-icon>
            </div>
            <h2>打开 SQL 查询窗口</h2>
            <p>点击上方 + 新建 SQL 窗口，或从左侧选择表自动生成对象窗口和 SELECT 查询。</p>
          </section>
        </main>
      </section>
    </section>

    <el-dialog
      v-model="rowDialog.visible"
      :title="rowDialog.mode === 'create' ? '新增数据行' : '修改数据行'"
      width="760px"
      class="workbench-dialog"
      modal-class="database-workbench-dialog-overlay"
      destroy-on-close
    >
      <div class="dialog-hero">
        <div>
          <span>Row Editor</span>
          <h3>{{ currentObject?.objectName || '-' }}</h3>
          <p>{{ rowDialog.mode === 'create' ? '填写需要写入的字段值，自增主键已自动隐藏。' : '主键字段不可修改，仅提交变更字段。' }}</p>
        </div>
        <el-tag effect="light" round>{{ rowDialog.mode === 'create' ? 'Create' : 'Update' }}</el-tag>
      </div>

      <el-form label-position="top" class="row-form">
        <el-row :gutter="14">
          <el-col v-for="column in editableColumns" :key="column.name" :span="12">
            <el-form-item :label="column.name" :required="!column.nullable && rowDialog.mode === 'create' && !column.autoIncrement">
              <el-input
                v-model="rowDialog.values[column.name]"
                clearable
                :disabled="rowDialog.mode === 'edit' && column.primaryKey"
                :placeholder="column.type || '请输入字段值'"
              />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>

      <template #footer>
        <span class="dialog-footer">
          <el-button class="cancel-button" @click="rowDialog.visible = false">取消</el-button>
          <el-button class="confirm-button" type="primary" :loading="rowSubmitting" @click="submitRow">确认</el-button>
        </span>
      </template>
    </el-dialog>

    <el-dialog
      v-model="snippetDialogVisible"
      title="SQL 片段"
      width="680px"
      class="workbench-dialog"
      modal-class="database-workbench-dialog-overlay"
      destroy-on-close
    >
      <div class="snippet-list">
        <button
          v-for="snippet in sqlSnippets"
          :key="snippet.id"
          type="button"
          class="snippet-item"
          @click="applySnippet(snippet)"
        >
          <strong>{{ snippet.title }}</strong>
          <span>{{ snippet.sql }}</span>
        </button>
        <el-empty v-if="sqlSnippets.length === 0" description="暂无 SQL 片段" />
      </div>
    </el-dialog>

    <el-dialog
      v-model="auditDialog.visible"
      title="审计详情"
      width="720px"
      class="workbench-dialog"
      modal-class="database-workbench-dialog-overlay"
      destroy-on-close
    >
      <div class="audit-detail" v-loading="auditDialog.loading">
        <pre>{{ auditDialog.content }}</pre>
      </div>
    </el-dialog>

    <el-dialog
      v-model="aiDialog.visible"
      title="AI SQL 助手"
      width="860px"
      class="workbench-dialog ai-sql-dialog"
      modal-class="database-workbench-dialog-overlay"
      destroy-on-close
    >
      <div class="ai-assistant-panel">
        <div class="ai-mode-tabs">
          <button
            v-for="mode in aiModes"
            :key="mode.value"
            type="button"
            :class="{ active: aiDialog.mode === mode.value }"
            @click="setAiMode(mode.value)"
          >
            {{ mode.label }}
          </button>
        </div>

        <div class="ai-context-strip">
          <span class="status-dot online"></span>
          <strong>{{ aiContext.engine }}</strong>
          <span>{{ aiContext.asset }}</span>
          <span class="separator"></span>
          <el-icon><Coin /></el-icon>
          <span>{{ aiContext.database || '未识别库' }}</span>
          <template v-if="aiContext.objectName">
            <span class="separator"></span>
            <span class="mono">{{ aiContext.objectName }}</span>
            <el-tag size="small" effect="light" round>{{ getNodeTypeLabel(aiContext.objectType) }}</el-tag>
          </template>
        </div>

        <el-form label-position="top" class="ai-form">
          <el-form-item :label="aiDialog.mode === 'text_to_sql' ? '自然语言需求' : '补充说明'">
            <el-input
              v-model="aiDialog.input"
              type="textarea"
              :rows="4"
              resize="none"
              :placeholder="aiDialog.mode === 'text_to_sql' ? '例如：查询最近 20 条发布记录，按创建时间倒序' : '例如：帮我优化这条 SQL，说明风险点'"
            />
          </el-form-item>
          <el-form-item v-if="aiDialog.mode !== 'text_to_sql'" label="当前 SQL">
            <div class="ai-current-sql">
              <pre>{{ activeWindow?.sql || '当前编辑器暂无 SQL' }}</pre>
            </div>
          </el-form-item>
        </el-form>

        <section v-if="aiDialog.result" class="ai-result">
          <div class="ai-result-head">
            <div>
              <strong>{{ aiDialog.result.providerName || 'AI' }}</strong>
              <span>{{ aiDialog.result.modelCode || '-' }} / {{ aiDialog.result.confidence || 'unknown' }}</span>
            </div>
            <el-button
              v-if="aiDialog.result.sql"
              class="primary-action compact"
              @click="insertAiSql"
            >
              插入到编辑器
            </el-button>
          </div>
          <div v-if="aiDialog.result.sql" class="ai-sql-block">
            <pre>{{ aiDialog.result.sql }}</pre>
          </div>
          <p v-if="aiDialog.result.explanation" class="ai-explanation">{{ aiDialog.result.explanation }}</p>
          <div v-if="aiDialog.result.warnings?.length" class="ai-advice-group">
            <strong>风险提示</strong>
            <ul>
              <li v-for="item in aiDialog.result.warnings" :key="item">{{ item }}</li>
            </ul>
          </div>
          <div v-if="aiDialog.result.indexes?.length" class="ai-advice-group">
            <strong>索引建议</strong>
            <ul>
              <li v-for="item in aiDialog.result.indexes" :key="item">{{ item }}</li>
            </ul>
          </div>
        </section>
      </div>
      <template #footer>
        <span class="dialog-footer">
          <el-button class="cancel-button" @click="aiDialog.visible = false">关闭</el-button>
          <el-button class="confirm-button" type="primary" :loading="aiDialog.loading" @click="submitAiAssistant">提交</el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, defineComponent, h, onMounted, reactive, ref } from 'vue'
import { ElButton, ElMessage, ElMessageBox, ElTable, ElTableColumn } from 'element-plus'
import {
  Close,
  Coin,
  Connection,
  DataBoard,
  Document,
  Download,
  FolderOpened,
  Grid,
  MagicStick,
  Plus,
  Refresh,
  Search,
  Tickets,
  VideoPlay
} from '@element-plus/icons-vue'
import dbApi from '@/api/database'
import CodeEditor from '@/components/CodeEditor.vue'
import databaseSvg from '@/assets/image/数据库.svg'
import tableSvg from '@/assets/image/数据表.svg'
import fieldSvg from '@/assets/image/字段.svg'
import mysqlSvg from '@/assets/image/mysqldb.svg'
import postgresqlSvg from '@/assets/image/PostgreSQL.svg'
import redisSvg from '@/assets/image/redis.svg'
import mongoSvg from '@/assets/image/mongodb.svg'
import elasticsearchSvg from '@/assets/image/Elasticsearch.svg'

const MYSQL_ENGINE = 1
const POSTGRESQL_ENGINE = 2
const REDIS_ENGINE = 4
const SUPPORTED_WORKBENCH_ENGINES = [MYSQL_ENGINE, POSTGRESQL_ENGINE, REDIS_ENGINE]
const MAX_WORKSPACE_TABS = 5
const REDIS_READONLY_COMMANDS = ['GET', 'MGET', 'TYPE', 'TTL', 'HGETALL', 'LRANGE', 'SMEMBERS', 'ZRANGE', 'SCAN', 'INFO', 'DBSIZE', 'MEMORY']
const REDIS_DANGEROUS_COMMANDS = ['SET', 'DEL', 'UNLINK', 'FLUSHDB', 'FLUSHALL', 'EXPIRE', 'EXPIREAT', 'PEXPIRE', 'PERSIST', 'HSET', 'HMSET', 'HDEL', 'LPUSH', 'RPUSH', 'LPOP', 'RPOP', 'SADD', 'SREM', 'ZADD', 'ZREM', 'XADD', 'XDEL', 'INCR', 'DECR', 'RENAME', 'MSET']
const REDIS_COMMAND_PLACEHOLDER = 'GET key\nHGETALL key\nSCAN 0 MATCH prefix:* COUNT 100'

const DataGrid = defineComponent({
  name: 'DataGrid',
  props: {
    loading: Boolean,
    columns: { type: Array, default: () => [] },
    rows: { type: Array, default: () => [] },
    editable: Boolean
  },
  emits: ['edit', 'delete'],
  setup(props, { emit }) {
    const formatCellValue = value => {
      if (value === null) return 'NULL'
      if (value === undefined) return ''
      if (typeof value === 'object') return JSON.stringify(value)
      return String(value)
    }

    return () => h('div', { class: 'table-shell data-grid-shell' }, [
      h(ElTable, {
        data: props.rows,
        class: 'data-table',
        height: '100%',
        'show-overflow-tooltip': true,
        'element-loading-text': '加载中',
        loading: props.loading
      }, () => [
        h(ElTableColumn, { type: 'index', label: '#', width: 54, fixed: true }),
        ...props.columns.map(column => h(ElTableColumn, {
          key: column.name,
          prop: column.name,
          label: column.name,
          minWidth: 132,
          showOverflowTooltip: true
        }, {
          header: () => h('span', { class: 'column-header' }, [
            column.name,
            column.primaryKey ? h('em', 'PK') : null
          ]),
          default: ({ row }) => h('span', { class: 'cell-value' }, formatCellValue(row[column.name]))
        })),
        h(ElTableColumn, { label: '操作', width: 96, fixed: 'right', align: 'center' }, {
          default: ({ row }) => h('div', { class: 'row-actions' }, [
            h(ElButton, { link: true, type: 'primary', disabled: !props.editable, onClick: () => emit('edit', row) }, () => '修改'),
            h(ElButton, { link: true, type: 'danger', disabled: !props.editable, onClick: () => emit('delete', row) }, () => '删除')
          ])
        })
      ])
    ])
  }
})

const ResultDock = defineComponent({
  name: 'ResultDock',
  props: {
    active: { type: String, default: 'result' },
    columns: { type: Array, default: () => [] },
    rows: { type: Array, default: () => [] },
    logs: { type: Array, default: () => [] },
    history: { type: Array, default: () => [] }
  },
  emits: ['change', 'reuse'],
  setup(props, { emit }) {
    const formatCellValue = value => {
      if (value === null) return 'NULL'
      if (value === undefined) return ''
      if (typeof value === 'object') return JSON.stringify(value)
      return String(value)
    }

    const renderTabs = () => h('div', { class: 'result-tabs' }, [
      h('button', { class: { active: props.active === 'result' }, onClick: () => emit('change', 'result') }, `结果集 1`),
      h('button', { class: { active: props.active === 'log' }, onClick: () => emit('change', 'log') }, '执行日志'),
      h('button', { class: { active: props.active === 'history' }, onClick: () => emit('change', 'history') }, '历史')
    ])

    const renderResult = () => h('div', { class: 'table-shell result-table-shell' }, [
      h(ElTable, { data: props.rows, class: 'data-table', height: '100%' }, () => [
        h(ElTableColumn, { type: 'index', label: '序号', width: 72 }),
        ...props.columns.map(column => h(ElTableColumn, {
          key: column.name || column,
          prop: column.name || column,
          label: column.name || column,
          minWidth: 150,
          showOverflowTooltip: true
        }, {
          default: ({ row }) => h('span', { class: 'cell-value' }, formatCellValue(row[column.name || column]))
        }))
      ])
    ])

    const renderLog = () => h('div', { class: 'log-panel' }, props.logs.length
      ? props.logs.map(item => h('p', { key: item.time + item.message }, [
        h('span', item.time),
        item.message
      ]))
      : [h('p', [h('span', '--:--:--'), '暂无执行日志'])])

    const renderHistory = () => h('div', { class: 'history-panel' }, props.history.length
      ? props.history.map(item => h('button', {
        key: item.id,
        type: 'button',
        class: 'history-item',
        onClick: () => emit('reuse', item)
      }, [
        h('strong', item.title),
        h('span', item.sql)
      ]))
      : [h('div', { class: 'history-empty' }, '暂无 SQL 执行历史')])

    return () => h('div', { class: 'result-dock' }, [
      renderTabs(),
      props.active === 'result' ? renderResult() : null,
      props.active === 'log' ? renderLog() : null,
      props.active === 'history' ? renderHistory() : null
    ])
  }
})

const assets = ref([])
const assetKeyword = ref('')
const selectedAsset = ref(null)
const currentObject = ref(null)
const datasourceChildrenCache = reactive({})
const unsupportedMessage = ref('')
const treeRef = ref(null)

const assetLoading = ref(false)
const treeLoading = ref(false)
const dataLoading = ref(false)
const pingLoading = ref(false)
const rowSubmitting = ref(false)
const auditLoading = ref(false)
const sqlExecuting = ref(false)
const exportingData = ref(false)

const sqlWindowIndex = ref(1)
const activeWindowKey = ref('sql-1')
const workspaceTabs = ref([
  {
    key: 'sql-1',
    type: 'sql',
    title: 'SQL窗口 1',
    database: '',
    objectName: '',
    sql: 'SELECT * FROM `demo`.`demo_products` LIMIT 20',
    resultColumns: [],
    resultRows: [],
    resultActive: 'result',
    logs: [],
    dirty: false
  }
])

const filterOperators = ['=', '!=', '>', '>=', '<', '<=', 'like', 'in', 'is_null', 'is_not_null']
const filterForm = reactive({ column: '', operator: '=', value: '' })
const queryState = reactive({ page: 1, pageSize: 20, filters: [], sorts: [] })
const dataState = reactive({ columns: [], rows: [], primaryKeys: [], total: 0, editable: false, source: 'object' })
const columnMeta = ref([])
const indexMeta = ref([])
const ddlText = ref('')
const metaLoaded = reactive({ columns: false, indexes: false, ddl: false })
const metaLoading = reactive({ columns: false, indexes: false, ddl: false })
const auditState = reactive({ rows: [], total: 0, page: 1, pageSize: 20 })
const sqlHistory = ref([])
const sqlCompletionItems = ref([])
const sqlSnippets = ref([])
const snippetDialogVisible = ref(false)

const rowDialog = reactive({ visible: false, mode: 'create', values: {}, original: null })
const auditDialog = reactive({ visible: false, loading: false, content: '' })
const aiModes = [
  { value: 'text_to_sql', label: '生成 SQL' },
  { value: 'optimize', label: '优化 SQL' },
  { value: 'explain', label: '解释 SQL' }
]
const aiDialog = reactive({
  visible: false,
  loading: false,
  mode: 'text_to_sql',
  input: '',
  result: null
})

const treeProps = {
  label: 'label',
  children: 'children',
  isLeaf: data => data.leaf === true || ['column', 'index', 'redis_key'].includes(data.type)
}

const datasourceTree = computed(() => {
  const keyword = assetKeyword.value.trim().toLowerCase()
  return assets.value
    .filter(asset => {
      if (!keyword) return true
      return [asset.name, asset.assetName, asset.host, asset.address, asset.ip, asset.description]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(keyword)
    })
    .map(asset => ({
      key: `asset:${getAssetId(asset)}`,
      label: getAssetTitle(asset),
      type: 'asset',
      assetId: getAssetId(asset),
      engine: getAssetEngine(asset),
      raw: asset,
      leaf: false
    }))
})

const activeWindow = computed(() => workspaceTabs.value.find(tab => tab.key === activeWindowKey.value))
const activeDatabase = computed(() => activeWindow.value?.database || currentObject.value?.database || '')
const activeTable = computed(() => activeWindow.value?.objectName || currentObject.value?.objectName || '')
const isRedisAsset = computed(() => getAssetEngine(selectedAsset.value) === REDIS_ENGINE)
const isRedisMode = computed(() => isRedisAsset.value || getWindowEngine(activeWindow.value) === REDIS_ENGINE)
const isRedisObjectWindow = computed(() => activeWindow.value?.type === 'object' && activeWindow.value?.object?.objectType === 'redis_key')
const editorLanguage = computed(() => isRedisMode.value ? 'plaintext' : 'sql')
const editorPlaceholder = computed(() => isRedisMode.value ? REDIS_COMMAND_PLACEHOLDER : '')
const aiContext = computed(() => {
  const context = getActiveSqlContext()
  const asset = selectedAsset.value
  return {
    engine: asset ? getEngineLabel(getAssetEngine(asset)) : 'unknown',
    asset: asset ? getAssetTitle(asset) : '未选择数据源',
    database: context.database,
    objectName: context.objectName,
    objectType: currentObject.value?.objectType || activeWindow.value?.object?.objectType || 'table'
  }
})
const visibleDataColumns = computed(() => dataState.columns.slice(0, 80))
const canMutateRows = computed(() => dataState.editable && dataState.primaryKeys.length > 0)
const editableColumns = computed(() => dataState.columns.filter(column => rowDialog.mode === 'create' ? !column.autoIncrement : true))

const unwrap = response => {
  const body = response?.data || response || {}
  if (body.code && body.code !== 200 && body.success !== true) throw new Error(body.message || '请求失败')
  return body.data !== undefined ? body.data : body
}

const normalizeList = payload => {
  if (Array.isArray(payload)) return payload
  if (Array.isArray(payload?.list)) return payload.list
  if (Array.isArray(payload?.rows)) return payload.rows
  if (Array.isArray(payload?.data)) return payload.data
  return []
}

const normalizeSqlColumns = payload => {
  const columns = Array.isArray(payload?.columns) ? payload.columns : []
  if (columns.length) {
    return columns.map(column => {
      if (typeof column === 'string') return { name: column, type: '' }
      return {
        ...column,
        name: column.name || column.columnName || column.label || column.field || '-',
        type: column.type || column.columnType || column.dataType || ''
      }
    })
  }
  const firstRow = normalizeList(payload)[0]
  return firstRow && typeof firstRow === 'object'
    ? Object.keys(firstRow).map(name => ({ name, type: '' }))
    : []
}

const normalizeSqlCompletion = payload => {
  if (!payload) return []
  const items = []
  const pushKeyword = keyword => items.push({ label: keyword, kind: 'keyword' })
  const pushDatabase = database => items.push({ label: database.name || database.database || database, kind: 'database' })
  const pushTable = table => {
    const label = table.name || table.tableName || table.objectName || table.label || table
    const columns = Array.isArray(table.columns) ? table.columns.map(column => ({
      columnName: column.name || column.columnName || column.label || column,
      columnType: column.type || column.columnType || column.dataType || ''
    })) : []
    items.push({ tableName: label, type: table.type || 'table', columns })
  }
  const pushColumn = column => items.push({
    label: column.name || column.columnName || column.label || column,
    kind: 'column',
    detail: column.type || column.columnType || column.dataType || ''
  })

  normalizeList(payload).forEach(item => items.push(item))
  ;(payload.keywords || []).forEach(pushKeyword)
  ;(payload.databases || []).forEach(pushDatabase)
  ;(payload.tables || payload.objects || []).forEach(pushTable)
  ;(payload.columns || payload.fields || []).forEach(pushColumn)
  return items
}

const getRedisCompletionItems = () => [
  ...REDIS_READONLY_COMMANDS.map(command => ({ label: command, kind: 'keyword' })),
  { label: 'MATCH', kind: 'keyword' },
  { label: 'COUNT', kind: 'keyword' }
]

const getAssetId = asset => asset?.id || asset?.assetId || asset?.dbId
const getAssetEngine = asset => Number(asset?.type || asset?.engine || asset?.dbType || 0)
const getAssetTitle = asset => asset?.name || asset?.assetName || asset?.host || `数据源 #${getAssetId(asset)}`
const getWindowEngine = win => Number(win?.engine || win?.object?.engine || getAssetEngine(selectedAsset.value) || 0)
const getAssetEnvironmentText = asset => {
  const env = asset?.environmentName || asset?.environment || asset?.envName || asset?.env || asset?.stage
  if (env === undefined || env === null || env === '') return ''
  const value = String(env).toLowerCase()
  return ({
    1: 'dev',
    2: 'test',
    3: 'prod',
    development: 'dev',
    develop: 'dev',
    testing: 'test',
    test: 'test',
    tese: 'test',
    production: 'prod',
    product: 'prod',
    prod: 'prod'
  }[value] || String(env))
}
const isAssetReady = asset => {
  const status = asset?.authStatus || asset?.status || asset?.state
  return status === undefined || status === 1 || status === 2 || status === 'success' || status === 'online' || status === true
}

const getEngineLabel = engine => ({
  1: 'mysql',
  2: 'postgresql',
  3: 'oracle',
  4: 'redis',
  5: 'mongodb',
  6: 'sqlserver',
  7: 'elasticsearch'
}[Number(engine)] || 'unknown')
const isPostgreSQLObject = object => Number(object?.engine || getAssetEngine(selectedAsset.value)) === POSTGRESQL_ENGINE
const isRedisObject = object => Number(object?.engine || getAssetEngine(selectedAsset.value)) === REDIS_ENGINE || object?.objectType === 'redis_key'

const getNodeTypeLabel = type => ({
  asset: '数据源',
  database: '数据库',
  schema: 'Schema',
  table: '表',
  view: '视图',
  column: '字段',
  index: '索引',
  collection: '集合',
  redis_db: 'Redis DB',
  redis_key: 'Redis Key',
  es_index: 'ES Index'
}[type] || type || '-')

const getNodeIcon = type => ({
  asset: Coin,
  database: FolderOpened,
  schema: FolderOpened,
  table: Grid,
  view: Tickets,
  column: Document,
  index: Tickets,
  collection: Grid,
  redis_db: FolderOpened,
  redis_key: Document,
  es_index: Grid
}[type] || Document)

const getLocalNodeIcon = type => ({
  database: databaseSvg,
  schema: databaseSvg,
  table: tableSvg,
  view: tableSvg,
  column: fieldSvg,
  redis_db: databaseSvg,
  redis_key: tableSvg
}[type] || '')

const getAssetTypeIcon = node => ({
  1: mysqlSvg,
  2: postgresqlSvg,
  3: databaseSvg,
  4: redisSvg,
  5: mongoSvg,
  6: databaseSvg,
  7: elasticsearchSvg
}[Number(node?.engine || node?.raw?.type || node?.raw?.engine || node?.raw?.dbType || 0)] || databaseSvg)

const getTreeIcon = node => node?.type === 'asset' ? getAssetTypeIcon(node) : getLocalNodeIcon(node?.type)

const getTreeNodeName = node => node?.label || node?.name || node?.objectName || '-'

const getTreeNodeMeta = node => {
  if (node.type === 'asset') return getAssetEnvironmentText(node.raw)
  if (node.type === 'redis_key') {
    return [node.extra?.dataType, node.extra?.ttlText].filter(Boolean).join(' · ')
  }
  if (node.extra?.sizeText) return node.extra.sizeText
  if (node.sizeText) return node.sizeText
  if (node.type === 'column') {
    const columnType = node.columnType || node.dataType || node.typeName || node.extra?.type || node.extra?.dataType
    return columnType || ''
  }
  if (node.type === 'table' && Number.isFinite(Number(node.rowCount))) return `${node.rowCount} rows`
  if (node.type === 'view') return 'view'
  return ''
}

const isActiveTreePath = node => {
  if (!currentObject.value || !node) return false
  const activeAssetId = currentObject.value.assetId || getAssetId(selectedAsset.value)
  if (node.type === 'asset') return Number(node.assetId) === Number(activeAssetId)
  const nodeDatabase = node.database || node.label || node.name
  const nodeSchema = node.schema || node.label || node.name || ''
  if (['database', 'redis_db'].includes(node.type)) return Number(node.assetId) === Number(activeAssetId) && nodeDatabase === currentObject.value.database
  if (node.type === 'schema') {
    return Number(node.assetId) === Number(activeAssetId) &&
      nodeDatabase === currentObject.value.database &&
      nodeSchema === currentObject.value.schema
  }
  return isActiveTreeCurrent(node)
}

const isActiveTreeCurrent = node => {
  if (!currentObject.value || !node) return false
  return ['table', 'view', 'collection', 'redis_key', 'es_index'].includes(node.type) &&
    Number(node.assetId) === Number(currentObject.value.assetId) &&
    node.database === currentObject.value.database &&
    (node.schema || '') === (currentObject.value.schema || '') &&
    (node.objectName || node.label) === currentObject.value.objectName
}

const loadAssets = async () => {
  assetLoading.value = true
  try {
    const previousEngine = getAssetEngine(selectedAsset.value)
    const previousAssetId = getAssetId(selectedAsset.value)
    const payload = unwrap(await dbApi.getCrudAssets({ page: 1, pageSize: 100, keyword: assetKeyword.value || undefined }))
    assets.value = normalizeList(payload)
    if (!selectedAsset.value && assets.value.length) {
      selectedAsset.value = assets.value.find(isAssetReady) || assets.value[0]
    } else if (selectedAsset.value) {
      selectedAsset.value = assets.value.find(asset => Number(getAssetId(asset)) === Number(previousAssetId)) || selectedAsset.value
    }
    const nextEngine = getAssetEngine(selectedAsset.value)
    if (previousAssetId !== getAssetId(selectedAsset.value) || previousEngine !== nextEngine) handleAssetModeChange(previousEngine)
  } catch (error) {
    showFriendlyError(error, '获取数据库资产失败')
  } finally {
    assetLoading.value = false
  }
}

const fetchTreeNodes = async params => {
  const payload = unwrap(await dbApi.getCrudTree(params))
  return normalizeList(payload).map(node => {
    const label = node.label || node.name || node.objectName || node.database || node.schema || '-'
    const database = node.database || (['database', 'redis_db'].includes(node.type) ? label : params.database)
    const schema = node.schema || (node.type === 'schema' ? label : params.schema)
    const objectName = node.objectName || (['table', 'view', 'collection', 'redis_key', 'es_index'].includes(node.type) ? label : '')
    return {
      ...node,
      database,
      schema,
      objectName,
      key: node.key || `${node.type}:${node.assetId || params.assetId}:${database || ''}:${schema || ''}:${objectName || label || ''}`,
      label,
      leaf: node.leaf === true || ['column', 'index', 'redis_key'].includes(node.type),
      primaryKey: node.primaryKey || node.extra?.primaryKey
    }
  })
}

const loadTreeNode = async (node, resolve) => {
  if (node.level === 0) {
    resolve(datasourceTree.value)
    return
  }

  const data = node.data
  if (!data || data.leaf) {
    resolve([])
    return
  }

  if (data.type === 'asset') {
    const previousEngine = getAssetEngine(selectedAsset.value)
    selectedAsset.value = data.raw || assets.value.find(asset => getAssetId(asset) === data.assetId)
    handleAssetModeChange(previousEngine)
    if (!SUPPORTED_WORKBENCH_ENGINES.includes(Number(data.engine))) {
      unsupportedMessage.value = '该数据库类型正在接入中，当前暂不支持对象浏览和数据操作。'
      resolve([])
      return
    }
  }

  const cacheKey = data.key
  if (datasourceChildrenCache[cacheKey]) {
    resolve(datasourceChildrenCache[cacheKey])
    return
  }

  treeLoading.value = true
  try {
    const nodes = await fetchTreeNodes({
      assetId: data.assetId || getAssetId(selectedAsset.value),
      level: data.type === 'asset' ? 'root' : data.type,
      database: data.database,
      schema: data.schema,
      objectName: data.objectName,
      objectType: data.type
    })
    datasourceChildrenCache[cacheKey] = nodes
    resolve(nodes)
  } catch (error) {
    showFriendlyError(error, '加载对象树失败')
    resolve([])
  } finally {
    treeLoading.value = false
  }
}

const handleTreeNodeClick = async node => {
  unsupportedMessage.value = ''
  if (node.type === 'asset') {
    const previousEngine = getAssetEngine(selectedAsset.value)
    selectedAsset.value = node.raw || assets.value.find(asset => getAssetId(asset) === node.assetId)
    handleAssetModeChange(previousEngine)
    return
  }

  if (node.database) {
    const win = activeWindow.value
    if (win) win.database = node.database
    loadSqlCompletion()
  }

  if (node.type === 'redis_db') {
    currentObject.value = {
      assetId: node.assetId || getAssetId(selectedAsset.value),
      engine: REDIS_ENGINE,
      database: node.database,
      schema: '',
      objectName: '',
      objectType: 'redis_db'
    }
    resetObjectMeta()
    resetDataState()
    ElMessage.info('请选择 key 或输入只读 Redis 命令')
    return
  }

  if (['table', 'view', 'collection', 'redis_key', 'es_index'].includes(node.type)) {
    await openObjectWindow(node)
  }
}

const openObjectWindow = async node => {
  currentObject.value = {
    assetId: node.assetId || getAssetId(selectedAsset.value),
    engine: node.engine || getAssetEngine(selectedAsset.value),
    database: node.database,
    schema: node.schema || '',
    objectName: node.objectName || node.label,
    objectType: node.type,
    extra: node.extra || {}
  }
  resetObjectMeta()

  const key = `object:${currentObject.value.assetId}:${currentObject.value.database}:${currentObject.value.schema}:${currentObject.value.objectName}`
  let tab = workspaceTabs.value.find(item => item.key === key)
  const defaultCommand = buildDefaultCommand(currentObject.value)

  if (!tab) {
    tab = {
      key,
      type: 'object',
      title: currentObject.value.objectName,
      object: { ...currentObject.value },
      engine: currentObject.value.engine,
      database: currentObject.value.database,
      objectName: currentObject.value.objectName,
      activeView: 'data',
      sql: defaultCommand,
      resultColumns: [],
      resultRows: [],
      resultActive: 'result',
      logs: [],
      dirty: false
    }
    workspaceTabs.value.push(tab)
  }

  setActiveWindow(key)
  enforceWorkspaceTabLimit()
  resetFilter(false)
  await refreshCurrentObject()
}

const resetObjectMeta = () => {
  columnMeta.value = []
  indexMeta.value = []
  ddlText.value = ''
  metaLoaded.columns = false
  metaLoaded.indexes = false
  metaLoaded.ddl = false
}

const resetDataState = () => {
  dataState.columns = []
  dataState.rows = []
  dataState.primaryKeys = []
  dataState.total = 0
  dataState.editable = false
  dataState.source = 'object'
}

const handleAssetModeChange = (previousEngine = 0) => {
  const engine = getAssetEngine(selectedAsset.value)
  if (!engine) return
  currentObject.value = null
  resetObjectMeta()
  resetDataState()
  if (engine !== REDIS_ENGINE) {
    if (previousEngine === REDIS_ENGINE || activeWindow.value?.engine === REDIS_ENGINE) {
      workspaceTabs.value = [{
        key: 'sql-1',
        type: 'sql',
        engine,
        title: 'SQL窗口 1',
        database: '',
        objectName: '',
        sql: '',
        resultColumns: [],
        resultRows: [],
        resultActive: 'result',
        logs: [],
        dirty: false
      }]
      activeWindowKey.value = 'sql-1'
      sqlWindowIndex.value = 1
    }
    return
  }

  workspaceTabs.value = [{
    key: 'redis-command',
    type: 'sql',
    engine: REDIS_ENGINE,
    title: 'Redis命令',
    database: '',
    objectName: '',
    sql: '',
    resultColumns: [],
    resultRows: [],
    resultActive: 'result',
    logs: [],
    dirty: false
  }]
  activeWindowKey.value = 'redis-command'
  sqlCompletionItems.value = getRedisCompletionItems()
  sqlHistory.value = []
}

const setActiveWindow = key => {
  activeWindowKey.value = key
  const tab = activeWindow.value
  if (tab?.type === 'object') currentObject.value = { ...tab.object }
  loadSqlCompletion()
  loadSqlHistory()
}

const closeWorkspaceTab = key => {
  const index = workspaceTabs.value.findIndex(tab => tab.key === key)
  if (index === -1) return
  workspaceTabs.value.splice(index, 1)
  if (activeWindowKey.value === key) {
    activeWindowKey.value = workspaceTabs.value[Math.max(0, index - 1)]?.key || ''
    setActiveWindow(activeWindowKey.value)
  }
}

const enforceWorkspaceTabLimit = () => {
  while (workspaceTabs.value.length > MAX_WORKSPACE_TABS) {
    const removeIndex = workspaceTabs.value.findIndex(tab => tab.key !== activeWindowKey.value)
    if (removeIndex === -1) return
    workspaceTabs.value.splice(removeIndex, 1)
  }
}

const createSqlWindow = () => {
  sqlWindowIndex.value += 1
  const key = `sql-${sqlWindowIndex.value}`
  const engine = getAssetEngine(selectedAsset.value)
  workspaceTabs.value.push({
    key,
    type: 'sql',
    engine,
    title: engine === REDIS_ENGINE ? `Redis命令 ${sqlWindowIndex.value}` : `SQL窗口 ${sqlWindowIndex.value}`,
    database: currentObject.value?.database || '',
    objectName: currentObject.value?.objectName || '',
    sql: engine === REDIS_ENGINE ? '' : (currentObject.value ? buildDefaultSelect(currentObject.value) : ''),
    resultColumns: [],
    resultRows: [],
    resultActive: 'result',
    logs: [],
    dirty: false
  })
  activeWindowKey.value = key
  enforceWorkspaceTabLimit()
  loadSqlCompletion()
  loadSqlHistory()
}

const buildDefaultCommand = object => {
  if (isRedisObject(object)) return ''
  return buildDefaultSelect(object)
}

const buildDefaultSelect = object => {
  if (!object) return ''
  if (isRedisObject(object)) return ''
  const quote = value => isPostgreSQLObject(object)
    ? `"${String(value).replace(/"/g, '""')}"`
    : `\`${String(value).replace(/`/g, '``')}\``
  const tableRef = object.schema
    ? `${quote(object.schema)}.${quote(object.objectName)}`
    : `${quote(object.database)}.${quote(object.objectName)}`
  return `SELECT * FROM ${tableRef} LIMIT 20`
}

const buildObjectParams = () => ({
  assetId: currentObject.value.assetId,
  database: currentObject.value.database,
  schema: currentObject.value.schema,
  objectName: currentObject.value.objectName,
  objectType: currentObject.value.objectType
})

const refreshCurrentObject = async () => {
  if (!currentObject.value) return
  const view = activeWindow.value?.activeView || 'data'
  if (view === 'data') {
    await queryData()
    if (isRedisObject(currentObject.value)) return
    if (!metaLoaded.columns) await loadColumns()
    return
  }
  if (view === 'columns') {
    await loadColumns()
    return
  }
  if (view === 'indexes') {
    await loadIndexes()
    return
  }
  if (view === 'ddl') {
    await loadDDL()
    return
  }
  if (view === 'audit') {
    await loadAuditList()
  }
}

const queryData = async () => {
  if (!currentObject.value) return
  dataLoading.value = true
  try {
    const payload = unwrap(await dbApi.queryCrudData({
      ...buildObjectParams(),
      page: queryState.page,
      pageSize: Math.min(queryState.pageSize, 200),
      columns: [],
      filters: queryState.filters,
      sorts: queryState.sorts
    }))
    dataState.columns = Array.isArray(payload.columns) ? payload.columns : dataState.columns
    dataState.rows = Array.isArray(payload.rows) ? payload.rows : normalizeList(payload)
    dataState.primaryKeys = Array.isArray(payload.primaryKeys) ? payload.primaryKeys : dataState.columns.filter(c => c.primaryKey).map(c => c.name)
    dataState.total = Number(payload.total || dataState.rows.length || 0)
    dataState.editable = !isRedisObject(currentObject.value) && !isPostgreSQLObject(currentObject.value) && payload.editable !== false
    dataState.source = 'object'
  } catch (error) {
    showFriendlyError(error, isRedisObject(currentObject.value) ? '查询 Redis Key 失败' : '查询表数据失败')
  } finally {
    dataLoading.value = false
  }
}

const loadColumns = async () => {
  if (!currentObject.value) return
  if (metaLoading.columns) return
  metaLoading.columns = true
  try {
    const payload = unwrap(await dbApi.getCrudColumns(buildObjectParams()))
    columnMeta.value = normalizeList(payload)
    if (!dataState.columns.length && columnMeta.value.length) dataState.columns = columnMeta.value
    metaLoaded.columns = true
  } catch (error) {
    showFriendlyError(error, '获取字段信息失败')
  } finally {
    metaLoading.columns = false
  }
}

const loadIndexes = async () => {
  if (!currentObject.value) return
  if (metaLoading.indexes) return
  metaLoading.indexes = true
  try {
    const payload = unwrap(await dbApi.getCrudIndexes(buildObjectParams()))
    indexMeta.value = normalizeList(payload)
    metaLoaded.indexes = true
  } catch (error) {
    showFriendlyError(error, '获取索引信息失败')
  } finally {
    metaLoading.indexes = false
  }
}

const loadDDL = async () => {
  if (!currentObject.value) return
  if (metaLoading.ddl) return
  metaLoading.ddl = true
  try {
    const payload = unwrap(await dbApi.getCrudDDL(buildObjectParams()))
    ddlText.value = typeof payload === 'string' ? payload : payload.ddl || payload.sql || ''
    metaLoaded.ddl = true
  } catch (error) {
    showFriendlyError(error, '获取 DDL 失败')
  } finally {
    metaLoading.ddl = false
  }
}

const loadAuditList = async () => {
  if (!selectedAsset.value) return
  if (auditLoading.value) return
  auditLoading.value = true
  try {
    const payload = unwrap(await dbApi.getCrudAuditList({
      page: auditState.page,
      pageSize: auditState.pageSize,
      assetId: getAssetId(selectedAsset.value),
      database: currentObject.value?.database,
      objectName: currentObject.value?.objectName
    }))
    auditState.rows = normalizeList(payload)
    auditState.total = Number(payload.total || auditState.rows.length || 0)
  } catch (error) {
    showFriendlyError(error, '获取审计列表失败')
  } finally {
    auditLoading.value = false
  }
}

const getActiveSqlContext = () => {
  const win = activeWindow.value
  const target = win?.type === 'object' ? win.object : currentObject.value
  return {
    assetId: target?.assetId || getAssetId(selectedAsset.value),
    database: win?.database || target?.database || currentObject.value?.database || '',
    schema: target?.schema || currentObject.value?.schema || '',
    objectName: win?.objectName || target?.objectName || currentObject.value?.objectName || ''
  }
}

const loadSqlCompletion = async () => {
  const context = getActiveSqlContext()
  if (!context.assetId) return
  if (isRedisMode.value) {
    sqlCompletionItems.value = getRedisCompletionItems()
    return
  }
  try {
    const payload = unwrap(await dbApi.getWorkbenchSqlCompletion({
      assetId: context.assetId,
      database: context.database || undefined,
      schema: context.schema || undefined,
      objectName: context.objectName || undefined
    }))
    sqlCompletionItems.value = normalizeSqlCompletion(payload)
  } catch (error) {
    pushLog(activeWindow.value, `补全元数据加载失败：${error?.message || '请求失败'}`)
  }
}

const loadSqlHistory = async () => {
  const context = getActiveSqlContext()
  if (!context.assetId) return
  if (isRedisMode.value) {
    sqlHistory.value = []
    return
  }
  try {
    const payload = unwrap(await dbApi.getWorkbenchSqlHistory({
      assetId: context.assetId,
      page: 1,
      pageSize: 20
    }))
    sqlHistory.value = normalizeList(payload).map((item, index) => ({
      id: item.id || item.auditId || `${Date.now()}-${index}`,
      title: item.title || item.database || item.objectName || item.operator || 'SQL执行',
      sql: item.sql || item.statement || item.content || item.detail || '',
      time: item.createdAt || item.createTime || item.time || ''
    })).filter(item => item.sql)
  } catch (error) {
    pushLog(activeWindow.value, `执行历史加载失败：${error?.message || '请求失败'}`)
  }
}

const handleObjectTabChange = name => {
  if (isRedisObjectWindow.value) return
  if (name === 'columns' && !metaLoaded.columns) loadColumns()
  if (name === 'indexes' && !metaLoaded.indexes) loadIndexes()
  if (name === 'ddl' && !metaLoaded.ddl) loadDDL()
  if (name === 'audit') loadAuditList()
}

const getRedisCommandName = commandText => {
  const firstLine = String(commandText || '').trim().split(/\r?\n/).find(line => line.trim() && !line.trim().startsWith('#')) || ''
  return firstLine.trim().split(/\s+/)[0]?.toUpperCase() || ''
}

const validateRedisCommand = commandText => {
  const command = getRedisCommandName(commandText)
  if (!command) return { valid: false, message: '请输入 Redis 命令' }
  if (REDIS_DANGEROUS_COMMANDS.includes(command)) {
    return { valid: false, message: `已拦截危险 Redis 命令：${command}` }
  }
  if (!REDIS_READONLY_COMMANDS.includes(command)) {
    return { valid: false, message: `仅允许只读 Redis 命令：${REDIS_READONLY_COMMANDS.join('、')}` }
  }
  if (command === 'MEMORY' && !/^memory\s+usage\b/i.test(commandText.trim())) {
    return { valid: false, message: 'MEMORY 仅允许 MEMORY USAGE key' }
  }
  return { valid: true, command }
}

const executeCurrentSql = async (confirmed = false) => {
  confirmed = confirmed === true
  const win = activeWindow.value
  if (!win) return

  const sql = (win.sql || '').trim()
  if (!sql) {
    ElMessage.warning(isRedisMode.value ? '请输入 Redis 命令' : '请输入 SQL')
    return
  }

  const redisMode = isRedisMode.value
  if (redisMode) {
    const validation = validateRedisCommand(sql)
    if (!validation.valid) {
      ElMessage.warning(validation.message)
      pushLog(win, validation.message)
      return
    }
  } else if (!/^(select|with)\b/i.test(sql)) {
    ElMessage.warning('SQL 工作台仅开放 SELECT / WITH 只读查询，变更类 SQL 请走左侧 SQL 审核菜单')
    pushLog(win, '已拦截非 SELECT SQL，请走 SQL 审核流程')
    return
  }

  const context = getActiveSqlContext()
  if (!context.assetId || !context.database) {
    ElMessage.warning(redisMode ? '请先选择 Redis DB' : '请先选择数据源和数据库')
    return
  }

  sqlExecuting.value = true
  try {
    const payload = unwrap(await dbApi.executeWorkbenchSql({
      assetId: context.assetId,
      database: context.database,
      schema: redisMode ? undefined : (context.schema || undefined),
      sql,
      page: queryState.page,
      pageSize: Math.min(queryState.pageSize, 200),
      confirmed
    }))
    if (payload.estimate?.message) {
      pushLog(win, payload.estimate.message)
    }
    if (payload.requireConfirm) {
      const estimate = payload.estimate || {}
      try {
        await ElMessageBox.confirm(
          estimate.message || `本条 SQL 执行预计耗时 ${estimate.estimatedSeconds || '-'} 秒，是否要执行？`,
          'SQL 执行确认',
          {
            confirmButtonText: '继续执行',
            cancelButtonText: '取消',
            type: estimate.riskLevel === 'high' ? 'warning' : 'info',
            customClass: 'workbench-confirm-dialog'
          }
        )
        pushLog(win, `用户确认执行：预计耗时 ${estimate.estimatedSeconds || '-'} 秒`)
        ElMessage.info('已确认，继续执行 SQL')
        await executeCurrentSql(true)
      } catch (confirmError) {
        if (confirmError === 'cancel' || confirmError === 'close') {
          pushLog(win, '用户取消执行预估耗时较长的 SQL')
          ElMessage.info('已取消执行')
          return
        }
        throw confirmError
      }
      return
    }
    const columns = normalizeSqlColumns(payload)
    const rows = Array.isArray(payload.rows) ? payload.rows : normalizeList(payload)
    const total = Number(payload.total || rows.length || 0)
    win.resultColumns = columns
    win.resultRows = rows
    win.resultActive = 'result'
    win.dirty = false
    if (win.type === 'object') {
      dataState.columns = columns
      dataState.rows = rows
      dataState.primaryKeys = columns.filter(c => c.primaryKey).map(c => c.name)
      dataState.total = total
      dataState.editable = false
      dataState.source = 'sql'
    }
    const successMessage = `${redisMode ? 'Redis 命令执行成功' : '执行成功'}，返回 ${rows.length} 行${payload.durationMs !== undefined ? `，耗时 ${payload.durationMs} ms` : ''}`
    pushLog(win, successMessage)
    ElMessage.success(successMessage)
    await loadSqlHistory()
  } catch (error) {
    showFriendlyError(error, redisMode ? 'Redis 命令执行失败' : 'SQL 执行失败')
    pushLog(win, `执行失败：${error?.message || '请求失败'}`)
  } finally {
    sqlExecuting.value = false
  }
}

const pushLog = (win, message) => {
  if (!win) return
  win.logs.unshift({
    time: new Date().toLocaleTimeString(),
    message
  })
}

const markActiveDirty = () => {
  if (activeWindow.value) activeWindow.value.dirty = true
}

const formatSql = () => {
  const win = activeWindow.value
  if (!win?.sql) return
  win.sql = win.sql
    .replace(/\s+/g, ' ')
    .replace(/\bfrom\b/ig, '\nFROM')
    .replace(/\bwhere\b/ig, '\nWHERE')
    .replace(/\border by\b/ig, '\nORDER BY')
    .replace(/\blimit\b/ig, '\nLIMIT')
  markActiveDirty()
}

const saveSnippet = () => {
  const win = activeWindow.value
  if (!win?.sql?.trim()) {
    ElMessage.warning(isRedisMode.value ? '没有可保存的 Redis 命令' : '没有可保存的 SQL')
    return
  }
  sqlSnippets.value.unshift({
    id: Date.now(),
    title: win.title,
    sql: win.sql.trim()
  })
  ElMessage.success(isRedisMode.value ? 'Redis 命令片段已保存' : 'SQL 片段已保存')
}

const openSnippetDialog = () => {
  snippetDialogVisible.value = true
}

const applySnippet = snippet => {
  if (!activeWindow.value) return
  activeWindow.value.sql = snippet.sql
  activeWindow.value.dirty = true
  snippetDialogVisible.value = false
}

const downloadSql = () => {
  const win = activeWindow.value
  if (!win?.sql) return
  if (isRedisMode.value) return
  if (win.type === 'object' && currentObject.value) {
    exportCurrentObjectData()
    return
  }
  const blob = new Blob([win.sql], { type: 'text/plain;charset=utf-8' })
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = `${win.title || 'query'}.sql`
  link.click()
  URL.revokeObjectURL(link.href)
}

const getDownloadFilename = (response, fallback) => {
  const disposition = response?.headers?.['content-disposition'] || response?.headers?.['Content-Disposition'] || ''
  const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i)
  if (utf8Match?.[1]) return decodeURIComponent(utf8Match[1])
  const match = disposition.match(/filename="?([^";]+)"?/i)
  if (match?.[1]) return decodeURIComponent(match[1])
  return fallback
}

const downloadBlob = (blob, filename) => {
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = filename
  link.click()
  URL.revokeObjectURL(link.href)
}

const exportCurrentObjectData = async (scope = 'all') => {
  if (!currentObject.value || exportingData.value) return
  exportingData.value = true
  try {
    const response = await dbApi.exportCrudData({
      ...buildObjectParams(),
      page: scope === 'query' ? queryState.page : 1,
      pageSize: scope === 'query' ? Math.min(queryState.pageSize, 200) : 10000,
      filters: scope === 'query' ? queryState.filters : [],
      sorts: queryState.sorts
    })
    const filename = getDownloadFilename(response, `${currentObject.value.objectName || 'data'}.csv`)
    downloadBlob(response.data, filename)
    ElMessage.success(`${scope === 'query' ? '当前查询结果导出已开始' : '全部数据导出已开始'}，最多导出 10000 行数据`)
    loadAuditList()
  } catch (error) {
    showFriendlyError(error, 'CSV 导出失败')
  } finally {
    exportingData.value = false
  }
}

const reuseHistory = item => {
  if (!activeWindow.value) createSqlWindow()
  activeWindow.value.sql = item.sql
  activeWindow.value.dirty = true
  activeWindow.value.resultActive = 'result'
}

const openAiAssistant = () => {
  if (isRedisMode.value) {
    ElMessage.info('Redis 命令模式暂不使用 AI SQL 助手')
    return
  }
  if (!activeWindow.value) createSqlWindow()
  aiDialog.visible = true
  aiDialog.mode = activeWindow.value?.sql?.trim() ? 'optimize' : 'text_to_sql'
  aiDialog.input = aiDialog.mode === 'text_to_sql' ? '' : '帮我优化这条 SQL'
  aiDialog.result = null
}

const setAiMode = mode => {
  aiDialog.mode = mode
  aiDialog.result = null
  if (mode === 'text_to_sql') aiDialog.input = ''
  if (mode === 'optimize') aiDialog.input = '帮我优化这条 SQL'
  if (mode === 'explain') aiDialog.input = '解释这条 SQL 的作用和风险'
}

const submitAiAssistant = async () => {
  const win = activeWindow.value
  if (!win) return
  const context = getActiveSqlContext()
  const sql = (win.sql || '').trim()
  if (aiDialog.mode === 'text_to_sql' && !aiDialog.input.trim()) {
    ElMessage.warning('请输入自然语言需求')
    return
  }
  if (aiDialog.mode !== 'text_to_sql' && !sql) {
    ElMessage.warning('当前编辑器没有可分析的 SQL')
    return
  }
  if (!context.assetId || !context.database) {
    ElMessage.warning('请先选择数据源和数据库')
    return
  }

  aiDialog.loading = true
  try {
    const payload = unwrap(await dbApi.askSqlAssistant({
      assetId: context.assetId,
      database: context.database,
      schema: currentObject.value?.schema || '',
      objectName: context.objectName || '',
      objectType: currentObject.value?.objectType || '',
      mode: aiDialog.mode,
      input: aiDialog.input,
      sql: aiDialog.mode === 'text_to_sql' ? undefined : sql,
      providerId: 0
    }))
    aiDialog.result = payload
    ElMessage.success('AI SQL 建议已生成')
  } catch (error) {
    showFriendlyError(error, 'AI SQL 助手请求失败')
  } finally {
    aiDialog.loading = false
  }
}

const insertAiSql = () => {
  if (!aiDialog.result?.sql || !activeWindow.value) return
  activeWindow.value.sql = aiDialog.result.sql
  activeWindow.value.dirty = true
  aiDialog.visible = false
  ElMessage.success('已插入到 SQL 编辑器，请确认后执行')
}

const applyFilter = () => {
  if (isRedisObjectWindow.value) {
    queryData()
    return
  }
  if (!filterForm.column) {
    ElMessage.warning('请选择筛选字段')
    return
  }
  if (!['is_null', 'is_not_null'].includes(filterForm.operator) && filterForm.value === '') {
    ElMessage.warning('请输入筛选值')
    return
  }
  queryState.filters = [{ column: filterForm.column, operator: filterForm.operator, value: filterForm.value }]
  queryState.page = 1
  queryData()
}

const handleDataPageChange = () => {
  if (dataState.source === 'sql') {
    executeCurrentSql(false)
    return
  }
  queryData()
}

const resetFilter = (reload = true) => {
  filterForm.column = ''
  filterForm.operator = '='
  filterForm.value = ''
  queryState.filters = []
  queryState.page = 1
  dataState.source = 'object'
  if (reload) queryData()
}

const openRowDialog = (mode, row = null) => {
  if (!currentObject.value) return
  if (isRedisObject(currentObject.value)) {
    ElMessage.warning('Redis Key 内容只读，已隐藏新增和修改操作')
    return
  }
  if (mode === 'edit' && !canMutateRows.value) {
    ElMessage.warning(dataState.primaryKeys.length ? '当前对象不可编辑' : '当前表无主键，暂不支持修改')
    return
  }
  rowDialog.mode = mode
  rowDialog.original = row
  rowDialog.values = {}
  dataState.columns.forEach(column => {
    if (mode === 'create') {
      if (!column.autoIncrement) rowDialog.values[column.name] = ''
    } else {
      rowDialog.values[column.name] = row?.[column.name] ?? ''
    }
  })
  rowDialog.visible = true
}

const getPrimaryKeyValues = row => {
  const keys = {}
  dataState.primaryKeys.forEach(key => { keys[key] = row?.[key] })
  return keys
}

const submitRow = async () => {
  if (!currentObject.value) return
  if (isRedisObject(currentObject.value)) return
  rowSubmitting.value = true
  try {
    const values = {}
    Object.entries(rowDialog.values).forEach(([key, value]) => {
      const column = dataState.columns.find(item => item.name === key)
      if (rowDialog.mode === 'edit' && column?.primaryKey) return
      values[key] = value
    })

    if (rowDialog.mode === 'create') {
      await dbApi.createCrudRow({ ...buildObjectParams(), values })
      ElMessage.success('新增成功')
    } else {
      await dbApi.updateCrudRow({ ...buildObjectParams(), keys: getPrimaryKeyValues(rowDialog.original), values })
      ElMessage.success('修改成功')
    }
    rowDialog.visible = false
    queryData()
    loadAuditList()
  } catch (error) {
    showFriendlyError(error, rowDialog.mode === 'create' ? '新增失败' : '修改失败')
  } finally {
    rowSubmitting.value = false
  }
}

const deleteRow = async row => {
  if (isRedisObject(currentObject.value)) {
    ElMessage.warning('Redis Key 内容只读，已隐藏删除操作')
    return
  }
  if (!canMutateRows.value) {
    ElMessage.warning(dataState.primaryKeys.length ? '当前对象不可编辑' : '当前表无主键，暂不支持删除')
    return
  }
  try {
    await ElMessageBox.confirm('确定删除当前数据行吗？该操作不可撤销。', '删除确认', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning',
      confirmButtonClass: 'el-button--danger'
    })
    await dbApi.deleteCrudRow({ ...buildObjectParams(), keys: getPrimaryKeyValues(row) })
    ElMessage.success('删除成功')
    queryData()
    loadAuditList()
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    showFriendlyError(error, '删除失败')
  }
}

const handlePingAsset = async () => {
  if (!selectedAsset.value) return
  pingLoading.value = true
  try {
    await dbApi.pingCrudAsset(getAssetId(selectedAsset.value))
    ElMessage.success('连接测试通过')
    loadAuditList()
  } catch (error) {
    showFriendlyError(error, '连接测试失败')
  } finally {
    pingLoading.value = false
  }
}

const openAuditDetail = async row => {
  auditDialog.visible = true
  auditDialog.loading = true
  auditDialog.content = ''
  try {
    const payload = unwrap(await dbApi.getCrudAuditInfo({ id: row.id }))
    auditDialog.content = JSON.stringify(payload, null, 2)
  } catch (error) {
    auditDialog.content = JSON.stringify(row, null, 2)
    showFriendlyError(error, '获取审计详情失败')
  } finally {
    auditDialog.loading = false
  }
}

const reloadWorkspace = async () => {
  Object.keys(datasourceChildrenCache).forEach(key => delete datasourceChildrenCache[key])
  await loadAssets()
  if (currentObject.value) await refreshCurrentObject()
  loadSqlCompletion()
  loadSqlHistory()
}

const showFriendlyError = (error, fallback = '请求失败') => {
  const message = String(error?.message || error?.response?.data?.message || fallback)
  if (message.includes('not implemented yet') || message.includes('暂未实现')) {
    unsupportedMessage.value = '该数据库类型正在接入中，当前暂不支持对象浏览和数据操作。'
    ElMessage.warning(unsupportedMessage.value)
    return
  }
  ElMessage.error(message)
}

const formatIndexColumns = row => Array.isArray(row.columns) ? row.columns.join(', ') : row.columns || row.columnName || row.column || '-'

onMounted(() => {
  loadAssets().then(() => {
    loadSqlCompletion()
    loadSqlHistory()
  })
})
</script>

<style scoped>
.database-sql-page {
  --ops-blue-500: #409eff;
  --ops-blue-600: #2563eb;
  --ops-green-500: #10b981;
  --ops-orange-500: #f59e0b;
  --ops-red-500: #ef4444;
  --ops-button-success: #67c23a;
  --ops-button-success-hover: #58b832;
  --ops-button-success-active: #4ca52a;
  --ops-button-warning: #e6a23c;
  --ops-button-warning-hover: #d8922f;
  --ops-button-warning-active: #c98226;
  --ops-text-heading: #0f172a;
  --ops-text-primary: #172033;
  --ops-text-secondary: #53657d;
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
  --aiops-bg-page: #f6f8fb;
  --aiops-surface-strong: var(--ops-bg-surface);
  --aiops-surface-soft: var(--ops-bg-surface-soft);
  --aiops-text-primary: var(--ops-text-primary);
  --aiops-text-secondary: var(--ops-text-secondary);
  --aiops-text-muted: var(--ops-text-weak);
  --aiops-border-cool: var(--ops-border-soft);
  --aiops-border-mid: var(--ops-border);
  --aiops-brand: var(--ops-blue-600);
  --aiops-brand-soft: #eef6ff;
  --aiops-success: var(--ops-green-500);
  --aiops-warning: var(--ops-orange-500);
  --aiops-danger: var(--ops-red-500);
  box-sizing: border-box;
  height: calc(100vh - 124px);
  min-height: 0;
  overflow: auto;
  padding: 7px;
  background: transparent;
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

:global(.home-container .el-main:has(.database-sql-page)) {
  padding: 10px;
  overflow: hidden;
}

.sql-workbench {
  display: flex;
  box-sizing: border-box;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: var(--ops-bg-surface);
  box-shadow: var(--ops-shadow-card);
}

.sql-workbench::before {
  display: none;
}

.sql-hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-shrink: 0;
  padding: 8px 12px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-surface);
}

.hero-copy {
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
  background: transparent;
}

.ops-page-header__icon img {
  width: 30px;
  height: 30px;
  object-fit: contain;
}

.ops-page-header__content {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
}

.ops-page-header__divider {
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  background: var(--ops-border-soft);
}

.ops-page-header__content h1 {
  margin: 0;
  color: var(--ops-text-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  white-space: nowrap;
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

.hero-actions,
.object-actions,
.query-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.primary-action,
.success-action,
.warning-action,
.secondary-action,
.execute-button,
.confirm-button,
.cancel-button {
  height: 30px;
  min-height: 30px;
  padding: 0 10px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  transition:
    color 160ms ease,
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease,
    transform 160ms ease;
}

.primary-action,
.execute-button,
.confirm-button {
  border-color: var(--ops-blue-600);
  background: var(--ops-blue-600);
  color: #fff;
  box-shadow: 0 5px 12px rgba(37, 99, 235, 0.14);
}

.success-action {
  border-color: var(--ops-button-success);
  background: var(--ops-button-success);
  color: #fff;
}

.warning-action {
  border-color: var(--ops-button-warning);
  background: var(--ops-button-warning);
  color: #fff;
}

.success-action:not(.is-disabled):hover {
  border-color: var(--ops-button-success-hover);
  background: var(--ops-button-success-hover);
}

.success-action:not(.is-disabled):active {
  border-color: var(--ops-button-success-active);
  background: var(--ops-button-success-active);
}

.warning-action:not(.is-disabled):hover {
  border-color: var(--ops-button-warning-hover);
  background: var(--ops-button-warning-hover);
}

.warning-action:not(.is-disabled):active {
  border-color: var(--ops-button-warning-active);
  background: var(--ops-button-warning-active);
}

.secondary-action,
.cancel-button {
  border: 1px solid var(--ops-border-inner);
  background: var(--ops-bg-surface);
  color: var(--aiops-text-secondary);
}

.primary-action:hover,
.execute-button:hover,
.confirm-button:hover,
.success-action:hover,
.warning-action:hover,
.secondary-action:hover,
.cancel-button:hover {
  transform: translateY(-1px);
}

.primary-action:active,
.execute-button:active,
.confirm-button:active,
.success-action:active,
.warning-action:active,
.secondary-action:active,
.cancel-button:active {
  transform: translateY(0);
}

.primary-action:focus-visible,
.execute-button:focus-visible,
.confirm-button:focus-visible,
.success-action:focus-visible,
.warning-action:focus-visible,
.secondary-action:focus-visible,
.cancel-button:focus-visible {
  outline: 2px solid rgba(64, 158, 255, 0.35);
  outline-offset: 2px;
}

.primary-action.is-disabled,
.execute-button.is-disabled,
.confirm-button.is-disabled,
.success-action.is-disabled,
.warning-action.is-disabled,
.secondary-action.is-disabled,
.cancel-button.is-disabled {
  opacity: 0.5;
  transform: none;
}

.console-layout {
  display: grid;
  grid-template-columns: 276px minmax(0, 1fr);
  flex: 1;
  min-width: 0;
  min-height: 0;
}

.datasource-pane {
  display: flex;
  min-width: 0;
  min-height: 0;
  flex-direction: column;
  border-right: 1px solid var(--aiops-border-cool);
  background: var(--ops-bg-surface-soft);
}

.datasource-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 9px 10px 8px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-subtle);
}

.datasource-head strong {
  display: block;
  color: var(--aiops-text-primary);
  font-size: 13px;
}

.datasource-head span:not(.el-tag__content) {
  display: block;
  margin-top: 3px;
  color: var(--ops-text-muted);
  font-size: 11px;
}

.datasource-head-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.datasource-refresh-button {
  width: 28px;
  height: 28px;
  min-height: 28px;
  padding: 0;
  border-color: var(--ops-button-warning);
  border-radius: 8px;
  background: var(--ops-button-warning);
  color: #fff;
  box-shadow: none;
}

.datasource-refresh-button:hover,
.datasource-refresh-button:focus-visible {
  border-color: var(--ops-button-warning-hover);
  background: var(--ops-button-warning-hover);
  color: #fff;
}

.datasource-refresh-button:active {
  border-color: var(--ops-button-warning-active);
  background: var(--ops-button-warning-active);
}

.tree-search {
  padding: 8px 10px;
}

.tree-search :deep(.el-input__wrapper),
.filter-form :deep(.el-input__wrapper),
.filter-form :deep(.el-select__wrapper),
.row-form :deep(.el-input__wrapper) {
  min-height: 32px;
  border: 0;
  border-radius: 8px;
  background: var(--ops-bg-surface-soft);
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.tree-search :deep(.el-input__wrapper:hover),
.filter-form :deep(.el-input__wrapper:hover),
.filter-form :deep(.el-select__wrapper:hover),
.row-form :deep(.el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px var(--ops-border-hover) inset;
}

.tree-search :deep(.el-input__wrapper.is-focus),
.filter-form :deep(.el-input__wrapper.is-focus),
.filter-form :deep(.el-select__wrapper.is-focused),
.row-form :deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 1px var(--ops-blue-500) inset;
}

.datasource-tree {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 6px 7px 10px;
  background: transparent;
}

.datasource-tree :deep(.el-tree-node__content) {
  height: 30px;
  border-radius: 7px;
}

.datasource-tree :deep(.el-tree-node__content:hover) {
  background: #f0f6ff;
}

.datasource-tree :deep(.el-tree-node__content:has(.tree-node.active-path)) {
  background: #eef6ff;
}

.datasource-tree :deep(.el-tree-node__content:has(.tree-node.active-current)) {
  background: #e4f0ff;
}

.tree-node {
  display: flex;
  align-items: center;
  gap: 7px;
  width: 100%;
  min-width: 0;
  color: var(--aiops-text-secondary);
  font-size: 13px;
}

.tree-node .el-icon {
  color: var(--aiops-brand);
}

.tree-svg-icon {
  width: 13px;
  height: 13px;
  flex-shrink: 0;
  object-fit: contain;
}

.tree-node.active-path {
  color: #315176;
  font-weight: 700;
}

.tree-node.active-current {
  color: var(--aiops-brand);
  font-weight: 800;
}

.tree-label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-column .tree-label {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
}

.tree-meta {
  flex-shrink: 0;
  margin-left: auto;
  color: var(--aiops-text-muted);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 10px;
  font-weight: 600;
}

.tree-meta-asset {
  margin-left: 4px;
  padding: 1px 5px;
  border-radius: 999px;
  background: rgba(31, 111, 255, 0.08);
  color: var(--aiops-brand);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
}

.tree-mini-tag {
  padding: 0 5px;
  border-radius: 999px;
  background: rgba(63, 185, 127, 0.12);
  color: var(--aiops-success);
  font-size: 10px;
  font-weight: 700;
}

.sql-console {
  display: flex;
  min-width: 0;
  min-height: 0;
  flex-direction: column;
  background: var(--ops-bg-surface);
}

.window-tabs {
  display: flex;
  align-items: stretch;
  flex-shrink: 0;
  min-height: 38px;
  overflow-x: auto;
  overflow-y: hidden;
  border-bottom: 1px solid var(--aiops-border-cool);
  background: var(--ops-bg-subtle);
}

.window-tab {
  display: flex;
  min-width: 0;
  height: 38px;
  align-items: center;
  border-right: 1px solid var(--ops-border-soft);
  background: transparent;
}

.window-tab-main {
  display: inline-flex;
  min-width: 0;
  height: 100%;
  align-items: center;
  gap: 7px;
  padding: 0 8px 0 11px;
  border: none;
  background: transparent;
  color: var(--aiops-text-secondary);
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.window-tab-title {
  max-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.window-tab.active {
  background: var(--ops-bg-surface);
  color: var(--aiops-brand);
  box-shadow: inset 0 2px 0 var(--aiops-brand);
}

.window-tab.active .window-tab-main {
  color: var(--aiops-brand);
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: var(--aiops-text-muted);
}

.status-dot.online {
  background: var(--aiops-success);
}

.tab-type-icon {
  flex: 0 0 auto;
  color: var(--ops-blue-600);
  font-size: 14px;
}

.dirty-dot {
  width: 5px;
  height: 5px;
  border-radius: 999px;
  background: var(--aiops-warning);
}

.tab-close {
  display: grid;
  width: 26px;
  height: 26px;
  flex: 0 0 26px;
  place-items: center;
  margin-right: 4px;
  padding: 0;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--aiops-text-muted);
  font-size: 13px;
  cursor: pointer;
}

.tab-close:hover {
  color: #dc2626;
  background: #fff0f0;
}

.window-tab-main:focus-visible,
.tab-close:focus-visible,
.add-tab-button:focus-visible {
  outline: 2px solid rgba(64, 158, 255, 0.35);
  outline-offset: -2px;
}

.add-tab-button {
  display: grid;
  width: 38px;
  height: 38px;
  flex: 0 0 38px;
  place-items: center;
  padding: 0;
  border: 0;
  border-right: 1px solid var(--ops-border-soft);
  background: transparent;
  color: var(--ops-blue-600);
  font-size: 16px;
  cursor: pointer;
}

.add-tab-button:hover {
  background: #eef6ff;
}

.connection-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-shrink: 0;
  min-height: 38px;
  padding: 4px 12px;
  border-bottom: 1px solid var(--aiops-border-cool);
  background: var(--ops-bg-surface);
}

.connection-main {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.connection-main strong {
  color: var(--aiops-text-primary);
  font-weight: 700;
}

.separator {
  width: 1px;
  height: 14px;
  background: var(--aiops-border-cool);
}

.mono {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
}

.tool-link {
  height: auto;
  padding: 0;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  font-weight: 700;
}

.ai-tool-link {
  height: 26px;
  padding: 0 8px;
  border-radius: 8px;
  background: #eef6ff;
  color: var(--aiops-brand);
  font-size: 12px;
  font-weight: 800;
}

.tool-link:hover {
  color: var(--aiops-brand);
}

.tool-link:focus-visible,
.ai-tool-link:focus-visible {
  outline: 2px solid rgba(64, 158, 255, 0.35);
  outline-offset: 2px;
}

.sql-window,
.object-window {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
}

.editor-shell {
  flex: 0 0 127px;
  min-height: 127px;
  border-bottom: 1px solid var(--aiops-border-cool);
  background: var(--ops-bg-surface-soft);
}

.monaco-shell :deep(.monaco-editor-container) {
  width: 100%;
  height: 100%;
  min-height: 0;
  border: none;
  border-radius: 0;
}

.monaco-shell :deep(.monaco-editor),
.monaco-shell :deep(.monaco-editor-background),
.monaco-shell :deep(.monaco-editor .margin) {
  background: var(--ops-bg-surface-soft) !important;
}

.monaco-shell :deep(.monaco-editor .view-line) {
  font-weight: 700;
}

.result-dock {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
  background: var(--ops-bg-surface);
}

.sql-window :deep(.result-dock),
.object-window :deep(.result-dock) {
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
  background: var(--ops-bg-surface);
}

.object-window :deep(.data-grid-shell) {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  border: 1px solid #e3ebf5;
  border-radius: 8px;
  background: var(--ops-bg-surface);
}

.result-dock :deep(.result-tabs) {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  height: 36px;
  border-bottom: 1px solid var(--aiops-border-cool);
  background: var(--ops-bg-subtle);
}

.result-dock :deep(.result-tabs button) {
  height: 36px;
  padding: 0 12px;
  border: none;
  border-right: 1px solid var(--ops-border-soft);
  background: transparent;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.result-dock :deep(.result-tabs button.active) {
  color: var(--aiops-brand);
  background: var(--ops-bg-surface);
  box-shadow: inset 0 2px 0 var(--aiops-brand);
}

.result-dock :deep(.result-table-shell) {
  margin: 0;
  border: none;
  border-radius: 0;
}

.result-dock :deep(.log-panel) {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 14px 16px;
  box-sizing: border-box;
  border-radius: 0;
  background: rgba(17, 24, 39, 0.92);
}

.result-dock :deep(.log-panel p) {
  margin: 0 0 8px;
  color: #d8e3f8;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
}

.result-dock :deep(.log-panel span) {
  margin-right: 12px;
  color: #8fb4ff;
}

.result-dock :deep(.history-panel) {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 12px;
  background: var(--ops-bg-surface-soft);
}

.result-dock :deep(.history-item) {
  display: block;
  width: 100%;
  margin-bottom: 8px;
  padding: 10px 12px;
  border: 1px solid var(--ops-border-soft);
  border-radius: 8px;
  background: var(--ops-bg-surface);
  text-align: left;
  cursor: pointer;
}

.result-dock :deep(.history-item strong),
.result-dock :deep(.history-item span) {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-dock :deep(.history-item strong) {
  color: var(--aiops-text-primary);
  font-size: 13px;
}

.result-dock :deep(.history-item span) {
  margin-top: 4px;
  color: var(--aiops-text-muted);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
}

.result-dock :deep(.history-empty) {
  color: var(--aiops-text-muted);
  font-size: 13px;
}

.result-tabs {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  height: 36px;
  border-bottom: 1px solid var(--aiops-border-cool);
  background: var(--ops-bg-subtle);
}

.result-tabs button {
  height: 36px;
  padding: 0 12px;
  border: none;
  border-right: 1px solid var(--ops-border-soft);
  background: transparent;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.result-tabs button.active {
  color: var(--aiops-brand);
  background: var(--ops-bg-surface);
  box-shadow: inset 0 2px 0 var(--aiops-brand);
}

.table-shell {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  border: 1px solid #e3ebf5;
  border-radius: 8px;
  background: var(--ops-bg-surface);
}

.result-table-shell {
  margin: 0;
  border: none;
  border-radius: 0;
}

.data-grid-shell {
  margin-top: 0;
}

.data-grid-shell :deep(.data-table),
.result-dock :deep(.data-table) {
  width: 100%;
  background: transparent;
}

.data-grid-shell :deep(.data-table .el-table__inner-wrapper::before),
.result-dock :deep(.data-table .el-table__inner-wrapper::before) {
  display: none;
}

.data-grid-shell :deep(.data-table .el-table__header th),
.data-grid-shell :deep(.data-table .el-table__fixed-right-patch),
.result-dock :deep(.data-table .el-table__header th),
.result-dock :deep(.data-table .el-table__fixed-right-patch) {
  background: var(--ops-bg-subtle) !important;
  border-bottom: 1px solid var(--ops-border-soft);
}

.data-grid-shell :deep(.data-table .el-table__header th .cell),
.result-dock :deep(.data-table .el-table__header th .cell) {
  color: var(--ops-text-heading);
  font-size: 12px;
  font-weight: 700;
}

.data-grid-shell :deep(.data-table .el-table__header th),
.data-grid-shell :deep(.data-table .el-table__body td),
.result-dock :deep(.data-table .el-table__header th),
.result-dock :deep(.data-table .el-table__body td) {
  height: 36px;
  padding: 4px 0;
}

.data-grid-shell :deep(.data-table .el-table__body td),
.result-dock :deep(.data-table .el-table__body td),
.data-table :deep(.el-table__body td) {
  color: #44546a;
  font-size: 12px;
}

.data-grid-shell :deep(.data-table .cell),
.result-dock :deep(.data-table .cell) {
  line-height: 20px;
  padding: 0 10px;
  font-size: 11px;
}

.data-grid-shell :deep(.data-table .el-table__body tr:hover > td.el-table__cell),
.result-dock :deep(.data-table .el-table__body tr:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

.data-table {
  width: 100%;
  background: transparent;
}

.data-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.data-table :deep(.el-table__header th),
.data-table :deep(.el-table__fixed-right-patch) {
  background: var(--ops-bg-subtle) !important;
  border-bottom: 1px solid var(--ops-border-soft);
}

.data-table :deep(.el-table__header th .cell) {
  color: var(--ops-text-heading);
  font-size: 12px;
  font-weight: 700;
}

.data-table :deep(.el-table__body tr:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

.column-header {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.column-header em {
  padding: 1px 6px;
  border-radius: 999px;
  background: rgba(63, 185, 127, 0.12);
  color: var(--aiops-success);
  font-style: normal;
  font-size: 10px;
}

.cell-value {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 11px;
  line-height: 20px;
}

.row-actions {
  display: inline-flex;
  gap: 8px;
}

.data-grid-shell :deep(.row-actions) {
  display: inline-flex;
  gap: 8px;
}

.data-grid-shell :deep(.row-actions .el-button) {
  height: auto;
  min-height: 0;
  padding: 0;
  border: none;
  background: transparent;
  font-size: 10px;
  font-weight: 700;
}

.row-actions :deep(.el-button) {
  height: auto;
  min-height: 0;
  padding: 0;
  border: none;
  background: transparent;
  font-size: 12px;
  font-weight: 700;
}

.object-window {
  position: relative;
  padding: 8px 10px 10px;
  background: var(--ops-bg-surface-soft);
}

.object-query-row {
  flex-shrink: 0;
  margin-bottom: 8px;
  border: 1px solid var(--ops-border-inner);
  border-radius: 8px;
  overflow: hidden;
  background: var(--ops-bg-surface-soft);
}

.object-editor-shell {
  height: 118px;
  min-height: 118px;
  background: var(--ops-bg-surface-soft);
}

.object-data-head {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  position: absolute;
  top: 136px;
  right: 12px;
  z-index: 2;
  height: 30px;
  padding: 0;
}

.object-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-shrink: 0;
  margin-bottom: 10px;
  padding: 13px 16px;
  border: 1px solid var(--ops-border-inner);
  border-radius: 10px;
  background: var(--ops-bg-subtle);
}

.object-title span {
  color: var(--aiops-brand);
  font-size: 11px;
  font-weight: 700;
}

.object-title h2 {
  display: inline-block;
  margin: 0 8px 0 0;
  color: var(--aiops-text-primary);
  font-size: 14px;
}

.object-title p {
  display: inline-block;
  margin: 0;
  color: var(--aiops-text-muted);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 11px;
}

.icon-action {
  width: 28px;
  height: 28px;
  min-height: 28px;
  padding: 0;
  border: 1px solid var(--ops-border-inner);
  border-radius: 8px;
  background: var(--ops-bg-surface);
  color: var(--aiops-text-secondary);
  box-shadow: none;
}

.success-icon-action {
  border-color: var(--ops-button-success);
  background: var(--ops-button-success);
  color: #fff;
}

.warning-icon-action {
  border-color: var(--ops-button-warning);
  background: var(--ops-button-warning);
  color: #fff;
}

.success-icon-action:not(.is-disabled):hover,
.success-icon-action:not(.is-disabled):focus-visible {
  border-color: var(--ops-button-success-hover);
  background: var(--ops-button-success-hover);
  color: #fff;
}

.warning-icon-action:not(.is-disabled):hover,
.warning-icon-action:not(.is-disabled):focus-visible {
  border-color: var(--ops-button-warning-hover);
  background: var(--ops-button-warning-hover);
  color: #fff;
}

.success-icon-action:not(.is-disabled):active {
  border-color: var(--ops-button-success-active);
  background: var(--ops-button-success-active);
}

.warning-icon-action:not(.is-disabled):active {
  border-color: var(--ops-button-warning-active);
  background: var(--ops-button-warning-active);
}

.filter-bar {
  flex-shrink: 0;
  margin-bottom: 6px;
  padding: 5px 8px;
  border: 1px solid var(--ops-border-inner);
  border-radius: 8px;
  background: var(--ops-bg-surface);
}

.filter-form {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  row-gap: 6px;
}

.filter-form :deep(.el-form-item) {
  margin-right: 10px;
  margin-bottom: 0;
}

.filter-form :deep(.el-form-item__label) {
  height: 28px;
  line-height: 28px;
  padding-right: 6px;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

.filter-form :deep(.el-select) {
  width: 136px;
}

.filter-form :deep(.el-input) {
  width: 160px;
}

.filter-form :deep(.el-input__wrapper),
.filter-form :deep(.el-select__wrapper) {
  min-height: 28px;
  border-radius: 7px;
}

.filter-form :deep(.el-input__inner) {
  height: 26px;
  font-size: 12px;
}

.filter-form .compact {
  height: 28px;
  min-height: 28px;
  padding: 0 13px;
  border-radius: 8px;
  font-size: 12px;
}

.export-query-button {
  padding: 0 13px;
  border-color: var(--ops-button-warning);
  background: var(--ops-button-warning);
  color: #fff;
  font-weight: 600;
}

.export-query-button:not(.is-disabled):hover,
.export-query-button:not(.is-disabled):focus-visible {
  border-color: var(--ops-button-warning-hover);
  background: var(--ops-button-warning-hover);
  color: #fff;
}

.export-query-button:not(.is-disabled):active {
  border-color: var(--ops-button-warning-active);
  background: var(--ops-button-warning-active);
}

.object-tabs {
  position: relative;
  display: flex;
  flex: 1;
  min-height: 0;
  flex-direction: column;
}

.object-tabs :deep(.el-tabs__header) {
  flex-shrink: 0;
  margin: 0 0 6px;
  padding-right: 78px;
}

.object-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: var(--ops-border-soft);
}

.object-tabs :deep(.el-tabs__item) {
  height: 34px;
  padding: 0 16px;
  font-size: 12px;
  font-weight: 600;
  line-height: 34px;
}

.object-tabs :deep(.el-tabs__item.is-active),
.object-tabs :deep(.el-tabs__item:hover) {
  color: var(--ops-blue-600);
}

.object-tabs :deep(.el-tabs__active-bar) {
  height: 2px;
  background: var(--ops-blue-600);
}

.object-tabs :deep(.el-tabs__content) {
  flex: 1;
  min-height: 0;
}

.object-tabs :deep(.el-tab-pane) {
  display: flex;
  min-height: 0;
  height: 100%;
  flex-direction: column;
}

.pagination-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-shrink: 0;
  padding-top: 6px;
  color: var(--ops-text-muted);
  font-size: 11px;
}

.pagination-row :deep(.el-pagination) {
  --el-pagination-button-height: 28px;
  --el-pagination-button-width: 28px;
  font-size: 12px;
}

.pagination-row :deep(.btn-prev),
.pagination-row :deep(.btn-next),
.pagination-row :deep(.el-pager li) {
  border-radius: 7px;
}

.tag-line {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.code-panel,
.audit-detail,
.log-panel {
  flex: 1;
  min-height: 0;
  overflow: auto;
  border-radius: 8px;
  background: rgba(17, 24, 39, 0.92);
}

.code-panel pre,
.audit-detail pre {
  margin: 0;
  padding: 16px;
  color: #d8e3f8;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
  line-height: 1.7;
  white-space: pre-wrap;
}

.log-panel {
  padding: 14px 16px;
  box-sizing: border-box;
}

.log-panel p {
  margin: 0 0 8px;
  color: #d8e3f8;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
}

.log-panel span {
  margin-right: 12px;
  color: #8fb4ff;
}

.history-panel {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 12px;
  background: var(--ops-bg-surface-soft);
}

.history-item,
.snippet-item {
  display: block;
  width: 100%;
  margin-bottom: 8px;
  padding: 10px 12px;
  border: 1px solid var(--ops-border-soft);
  border-radius: 8px;
  background: var(--ops-bg-surface);
  text-align: left;
  cursor: pointer;
  transition:
    border-color 160ms ease,
    background-color 160ms ease,
    box-shadow 160ms ease,
    transform 160ms ease;
}

.history-item:hover,
.snippet-item:hover {
  border-color: var(--ops-border-hover);
  background: #f8fbff;
  box-shadow: 0 5px 14px rgba(64, 158, 255, 0.08);
  transform: translateY(-1px);
}

.history-item:active,
.snippet-item:active {
  transform: translateY(0);
}

.history-item:focus-visible,
.snippet-item:focus-visible {
  outline: 2px solid rgba(64, 158, 255, 0.35);
  outline-offset: 2px;
}

.history-item strong,
.snippet-item strong,
.history-item span,
.snippet-item span {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.history-item strong,
.snippet-item strong {
  color: var(--aiops-text-primary);
  font-size: 13px;
}

.history-item span,
.snippet-item span {
  margin-top: 4px;
  color: var(--aiops-text-muted);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
}

.history-empty {
  color: var(--aiops-text-muted);
  font-size: 13px;
}

.empty-workspace {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 1;
  flex-direction: column;
  padding: 32px;
  text-align: center;
}

.empty-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 58px;
  height: 58px;
  border-radius: 10px;
  background: var(--aiops-brand-soft);
  color: var(--aiops-brand);
  font-size: 26px;
}

.empty-workspace h2 {
  margin: 16px 0 8px;
  color: var(--aiops-text-primary);
  font-size: 16px;
}

.empty-workspace p {
  max-width: 420px;
  margin: 0;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.7;
}

.dialog-hero {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
  padding: 16px;
  border: 1px solid var(--ops-border-inner);
  border-radius: 10px;
  background: var(--ops-bg-subtle);
}

.dialog-hero span {
  color: var(--aiops-brand);
  font-size: 12px;
  font-weight: 700;
}

.dialog-hero h3 {
  margin: 5px 0;
  color: var(--aiops-text-primary);
}

.dialog-hero p {
  margin: 0;
  color: var(--aiops-text-secondary);
  font-size: 13px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.snippet-list {
  max-height: 420px;
  overflow: auto;
}

.ai-assistant-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.ai-mode-tabs {
  display: inline-flex;
  width: fit-content;
  padding: 3px;
  border: 1px solid var(--ops-border-inner);
  border-radius: 8px;
  background: var(--ops-bg-subtle);
}

.ai-mode-tabs button {
  height: 30px;
  padding: 0 14px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
}

.ai-mode-tabs button.active {
  background: var(--ops-bg-surface);
  color: var(--aiops-brand);
  box-shadow: 0 3px 8px rgba(64, 158, 255, 0.1);
}

.ai-mode-tabs button:focus-visible {
  outline: 2px solid rgba(64, 158, 255, 0.35);
  outline-offset: 1px;
}

.ai-context-strip {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 34px;
  padding: 6px 10px;
  border: 1px solid var(--ops-border-inner);
  border-radius: 8px;
  background: var(--ops-bg-subtle);
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.ai-context-strip strong {
  color: var(--aiops-text-primary);
  font-weight: 800;
}

.ai-context-strip .separator {
  height: 14px;
}

.ai-form :deep(.el-form-item) {
  margin-bottom: 10px;
}

.ai-form :deep(.el-textarea__inner) {
  border: 0;
  border-radius: 8px;
  background: var(--ops-bg-surface-soft);
  box-shadow: 0 0 0 1px #d9e5f2 inset;
  font-size: 13px;
}

.ai-form :deep(.el-textarea__inner:hover) {
  box-shadow: 0 0 0 1px var(--ops-border-hover) inset;
}

.ai-form :deep(.el-textarea__inner:focus) {
  box-shadow: 0 0 0 1px var(--ops-blue-500) inset;
}

.ai-current-sql,
.ai-sql-block {
  overflow: auto;
  border: 1px solid var(--ops-border-inner);
  border-radius: 8px;
  background: var(--ops-bg-surface-soft);
}

.ai-current-sql pre,
.ai-sql-block pre {
  margin: 0;
  padding: 10px 12px;
  color: #172133;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
  line-height: 1.7;
  white-space: pre-wrap;
}

.ai-result {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--ops-border-inner);
  border-radius: 10px;
  background: var(--ops-bg-surface-soft);
}

.ai-result-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.ai-result-head strong {
  color: var(--aiops-text-primary);
  font-size: 13px;
}

.ai-result-head span,
.ai-explanation,
.ai-advice-group li {
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.ai-explanation {
  margin: 0;
  line-height: 1.7;
}

.ai-advice-group strong {
  color: var(--aiops-text-primary);
  font-size: 12px;
}

.ai-advice-group ul {
  margin: 6px 0 0;
  padding-left: 18px;
}

:global(.database-workbench-dialog-overlay) {
  --ops-blue-500: #409eff;
  --ops-blue-600: #2563eb;
  --ops-text-heading: #0f172a;
  --ops-text-primary: #172033;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-bg-surface: #ffffff;
  --ops-bg-surface-soft: #fbfdff;
  --ops-bg-subtle: #f3f7fc;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  --ops-border-inner: #dce8f5;
  --ops-border-hover: #b9d4ff;
  --aiops-text-primary: var(--ops-text-primary);
  --aiops-text-secondary: var(--ops-text-secondary);
  --aiops-text-muted: #7b8aa0;
  --aiops-brand: var(--ops-blue-600);
}

:global(.database-workbench-dialog-overlay .el-dialog) {
  max-width: calc(100vw - 24px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: 15px !important;
  background: var(--ops-bg-surface);
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.database-workbench-dialog-overlay .el-dialog.ai-sql-dialog) {
  border-top: 3px solid var(--ops-blue-500);
  border-radius: 20px !important;
}

:global(.database-workbench-dialog-overlay .el-dialog__header) {
  margin-right: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid var(--ops-border-soft);
}

:global(.database-workbench-dialog-overlay .el-dialog__title) {
  color: var(--ops-text-heading);
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.database-workbench-dialog-overlay .el-dialog__headerbtn) {
  top: 8px;
  right: 10px;
  width: 34px;
  height: 34px;
  border-radius: 8px;
}

:global(.database-workbench-dialog-overlay .el-dialog__headerbtn:hover) {
  background: var(--ops-bg-subtle);
}

:global(.database-workbench-dialog-overlay .el-dialog__body) {
  max-height: min(68vh, 720px);
  overflow: auto;
  padding: 16px 18px;
  color: var(--ops-text-primary);
}

:global(.database-workbench-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid var(--ops-border-soft);
}

:global(.database-workbench-dialog-overlay .el-tag) {
  height: 22px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
}

@media (max-width: 960px) {
  .sql-workbench {
    height: auto;
    min-height: 100%;
    overflow: visible;
  }

  .sql-hero,
  .connection-strip,
  .object-toolbar,
  .pagination-row {
    flex-direction: column;
    align-items: stretch;
  }

  .sql-hero {
    padding: 8px 10px;
  }

  .ops-page-header__content {
    align-items: flex-start;
    flex-direction: column;
    gap: 2px;
  }

  .ops-page-header__divider {
    display: none;
  }

  .console-layout {
    grid-template-columns: 1fr;
  }

  .datasource-pane {
    min-height: 300px;
    max-height: 380px;
    border-right: none;
    border-bottom: 1px solid var(--aiops-border-cool);
  }

  .sql-console {
    min-height: 680px;
  }

  .query-actions,
  .hero-actions,
  .object-actions {
    width: 100%;
    flex-wrap: wrap;
  }

  .connection-main {
    width: 100%;
    flex-wrap: wrap;
  }

  .object-data-head {
    position: static;
    height: 30px;
    margin-bottom: 4px;
  }

  .object-tabs :deep(.el-tabs__header) {
    padding-right: 0;
  }

  .pagination-row :deep(.el-pagination) {
    overflow-x: auto;
  }
}

@media (max-width: 620px) {
  .database-sql-page {
    padding: 6px;
  }

  .hero-copy {
    align-items: flex-start;
  }

  .ops-page-header__icon {
    width: 36px;
    height: 36px;
    flex-basis: 36px;
  }

  .ops-page-header__icon img {
    width: 27px;
    height: 27px;
  }

  .hero-actions .el-button {
    flex: 1 1 0;
  }

  .filter-form {
    display: grid;
    grid-template-columns: 1fr;
  }

  .filter-form :deep(.el-form-item),
  .filter-form :deep(.el-form-item__content),
  .filter-form :deep(.el-select),
  .filter-form :deep(.el-input) {
    width: 100%;
  }

  .filter-form :deep(.el-form-item) {
    display: grid;
    grid-template-columns: 58px minmax(0, 1fr);
    margin-right: 0;
  }

  .filter-form :deep(.el-form-item:last-child .el-form-item__content) {
    grid-column: 1 / -1;
    display: flex;
  }

  .filter-form :deep(.el-form-item:last-child .el-button) {
    flex: 1 1 0;
  }

  .row-form :deep(.el-col-12) {
    max-width: 100%;
    flex: 0 0 100%;
  }

  .ai-context-strip {
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .ai-result-head {
    align-items: flex-start;
    flex-direction: column;
  }

  :global(.database-workbench-dialog-overlay .el-dialog) {
    margin-top: 12px;
  }

  :global(.database-workbench-dialog-overlay .el-dialog__body) {
    max-height: calc(100vh - 150px);
    padding: 14px;
  }
}
</style>
