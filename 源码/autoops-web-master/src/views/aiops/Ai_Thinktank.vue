<template>
  <div class="thinktank-page">
    <section class="thinktank-card">
      <header class="page-header">
        <div class="header-brand">
          <span class="header-brand-icon" aria-hidden="true">
            <img src="@/assets/image/AI小助手.svg" alt="">
          </span>
          <div class="header-title">
            <h2>运维知识库</h2>
          </div>
        </div>
        <div class="header-commands">
          <el-button size="small" type="warning" :icon="Upload" @click="openImportDialog">导入文档</el-button>
          <el-button size="small" type="success" :icon="Plus" @click="openCreate">新建文档</el-button>
        </div>
      </header>

      <div class="workspace" :class="{ 'outline-collapsed': outlineCollapsed }">
        <aside class="tree-panel">
          <div class="knowledge-toolbar">
            <el-input
              v-model.trim="listQuery.keyword"
              clearable
              placeholder="搜索知识库"
              class="global-search"
              @keyup.enter="handleKnowledgeSearch"
              @clear="handleQuery"
            >
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>
            <el-button
              circle
              type="primary"
              :icon="Search"
              :disabled="!listQuery.keyword"
              :loading="searching"
              title="搜索"
              aria-label="搜索"
              @click="handleKnowledgeSearch"
            />
          </div>

          <div class="panel-head">
            <div>
              <h3>目录</h3>
            </div>
            <div class="tree-head-actions">
              <el-button circle :icon="Plus" class="tree-add-btn" title="新建根分组" aria-label="新建根分组" @click="openGroupDialog('create')" />
              <el-button
                circle
                :icon="Refresh"
                :loading="loadingTree || loadingList"
                class="tree-refresh-btn"
                title="刷新知识树"
                aria-label="刷新知识树"
                @click="refreshKnowledgeTree"
              />
            </div>
          </div>

          <el-scrollbar v-loading="loadingTree || loadingList" class="tree-scroll">
            <el-tree
              ref="treeRef"
              :data="treeData"
              node-key="id"
              :props="treeProps"
              :highlight-current="true"
              :expand-on-click-node="false"
              default-expand-all
              class="knowledge-tree"
              @node-click="handleTreeNodeClick"
              @node-contextmenu="handleTreeNodeContextMenu"
            >
              <template #default="{ data, node }">
                <div class="tree-node" :class="`type-${data.type}`">
                  <el-icon v-if="data.type === 'document'" class="tree-node-icon tree-document-icon" aria-hidden="true">
                    <Document />
                  </el-icon>
                  <img
                    v-else
                    class="tree-node-icon"
                    :src="data.type === 'root' ? documentTreeIcon : (node.expanded ? openedFolderIcon : closedFolderIcon)"
                    alt=""
                    aria-hidden="true"
                  />
                  <span>{{ data.label }}</span>
                  <small v-if="data.type !== 'document'">{{ data.count || 0 }}</small>
                  <el-dropdown
                    v-if="data.type === 'group'"
                    trigger="click"
                    @command="command => handleGroupCommand(command, data)"
                    @click.stop
                  >
                    <button type="button" class="tree-node-more" title="分组操作" aria-label="分组操作" @click.stop>
                      <el-icon><MoreFilled /></el-icon>
                    </button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item command="create">新建子分组</el-dropdown-item>
                        <el-dropdown-item command="rename">重命名</el-dropdown-item>
                        <el-dropdown-item command="move">移动分组</el-dropdown-item>
                        <el-dropdown-item divided command="delete">删除空分组</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </div>
              </template>
            </el-tree>
          </el-scrollbar>
        </aside>

        <nav class="document-panel" :class="{ collapsed: outlineCollapsed }" aria-label="文章目录">
          <div class="outline-panel-head">
            <strong>文章段落</strong>
            <button
              type="button"
              class="outline-toggle"
              :title="outlineCollapsed ? '展开文章段落' : '收起文章段落'"
              :aria-label="outlineCollapsed ? '展开文章段落' : '收起文章段落'"
              :aria-expanded="!outlineCollapsed"
              @click="outlineCollapsed = !outlineCollapsed"
            >
              <el-icon>
                <DArrowRight v-if="outlineCollapsed" />
                <DArrowLeft v-else />
              </el-icon>
            </button>
          </div>

          <el-scrollbar class="outline-scroll">
            <div v-if="documentOutline.length" class="outline-list">
              <button
                v-for="heading in documentOutline"
                :key="heading.id"
                type="button"
                class="outline-item"
                :class="[`level-${heading.level}`, { active: activeOutlineId === heading.id }]"
                :title="heading.text"
                @click="scrollToHeading(heading.id)"
              >
                {{ heading.text }}
              </button>
            </div>
            <div v-else class="outline-empty">
              <el-icon aria-hidden="true"><Document /></el-icon>
              <span>{{ hasDetail ? '暂无目录' : '未选择文章' }}</span>
            </div>
          </el-scrollbar>
        </nav>

        <section v-loading="loadingDetail" class="detail-panel">
          <template v-if="hasDetail">
            <div class="detail-head">
              <div class="detail-head-main">
                <div class="detail-title">
                  <div class="document-mark">
                    <el-icon><Document /></el-icon>
                  </div>
                  <div>
                    <h3>{{ formData.title || currentDocument?.title || '新建知识文档' }}</h3>
                    <p>{{ detailSubtitle }}</p>
                  </div>
                </div>

                <div class="detail-tab-nav" role="tablist" aria-label="文档视图">
                  <button
                    type="button"
                    role="tab"
                    :aria-selected="activeDetailTab === 'preview'"
                    :class="{ active: activeDetailTab === 'preview' }"
                    @click="switchDetailTab('preview')"
                  >
                    预览
                  </button>
                  <button
                    type="button"
                    role="tab"
                    :aria-selected="activeDetailTab === 'edit'"
                    :class="{ active: activeDetailTab === 'edit' }"
                    @click="openEditTab"
                  >
                    编辑
                  </button>
                  <button
                    type="button"
                    role="tab"
                    :aria-selected="activeDetailTab === 'ai'"
                    :class="{ active: activeDetailTab === 'ai' }"
                    :disabled="!formData.id"
                    @click="switchDetailTab('ai')"
                  >
                    AI 总结
                  </button>
                  <button
                    type="button"
                    role="tab"
                    :aria-selected="activeDetailTab === 'search'"
                    :class="{ active: activeDetailTab === 'search' }"
                    :disabled="!formData.id"
                    @click="switchDetailTab('search')"
                  >
                    检索详情
                  </button>
                </div>
              </div>

              <div class="detail-actions">
                <template v-if="activeDetailTab === 'edit'">
                  <el-button @click="cancelEdit">取消</el-button>
                  <el-button type="primary" :icon="Check" :loading="saving" @click="saveDocument">
                    {{ formData.id ? '保存' : '创建' }}
                  </el-button>
                </template>
                <template v-else>
                  <el-button :icon="CopyDocument" :disabled="!formData.contentMarkdown" @click="copyMarkdown">复制</el-button>
                  <el-button :icon="EditPen" @click="openEditTab">编辑</el-button>
                  <el-dropdown v-if="formData.id" trigger="click" @command="handleDocumentAction">
                    <el-button :icon="MoreFilled">更多</el-button>
                    <template #dropdown>
                      <el-dropdown-menu>
                        <el-dropdown-item command="summarize" :icon="MagicStick" :disabled="summarizing">生成 AI 总结</el-dropdown-item>
                        <el-dropdown-item command="refresh" :icon="Refresh" :disabled="refreshingSearch">更新检索数据</el-dropdown-item>
                        <el-dropdown-item divided command="delete" :icon="Delete">删除文档</el-dropdown-item>
                      </el-dropdown-menu>
                    </template>
                  </el-dropdown>
                </template>
              </div>
            </div>

            <el-tabs ref="detailTabsRef" v-model="activeDetailTab" class="detail-tabs">
              <el-tab-pane label="预览" name="preview" class="preview-tab-pane">
                <div class="preview-layout">
                  <el-collapse v-if="aiExtractionCount" class="ai-collapse">
                    <el-collapse-item name="ai">
                      <template #title>
                        <span class="collapse-title">AI 提取 <small>{{ aiExtractionCount }} 项</small></span>
                      </template>
                      <div class="ai-blocks">
                        <div v-for="block in aiExtractionBlocks" :key="block.key" class="ai-block">
                          <span>{{ block.label }}</span>
                          <ul>
                            <li v-for="item in block.items" :key="item">{{ item }}</li>
                          </ul>
                        </div>
                      </div>
                    </el-collapse-item>
                  </el-collapse>

                  <section class="markdown-section">
                    <div ref="previewContentRef" class="markdown-preview" v-html="renderedMarkdown"></div>
                  </section>
                </div>
              </el-tab-pane>

              <el-tab-pane label="编辑" name="edit" class="edit-tab-pane">
                <EditThinktank
                  ref="editThinktankRef"
                  v-model:title="formData.title"
                  v-model:doc-type="formData.docType"
                  v-model:group-id="formData.groupId"
                  v-model:tags="formData.tags"
                  v-model:content-markdown="formData.contentMarkdown"
                  v-model:metadata-visible="editMetadataVisible"
                  :doc-type-options="docTypeOptions"
                  :group-select-data="groupSelectData"
                />
              </el-tab-pane>

              <el-tab-pane label="AI 总结" name="ai">
                <div class="summary-workbench">
                  <div class="summary-action-panel">
                    <div>
                      <strong>结构化总结</strong>
                      <span>调用默认或指定 Provider，回写摘要、现象、根因、处理步骤和关键词。</span>
                    </div>
                    <el-select
                      v-model="summaryProviderId"
                      clearable
                      filterable
                      placeholder="默认 Provider"
                      :loading="providerLoading"
                      class="provider-select"
                    >
                      <el-option
                        v-for="provider in providerOptions"
                        :key="getProviderId(provider)"
                        :label="getProviderLabel(provider)"
                        :value="getProviderId(provider)"
                      />
                    </el-select>
                    <el-button
                      type="primary"
                      :icon="MagicStick"
                      :disabled="!formData.id"
                      :loading="summarizing"
                      @click="summarizeDocument"
                    >
                      开始总结
                    </el-button>
                  </div>

                  <div class="summary-result-grid compact-summary-grid">
                    <section class="summary-result-block summary-compact-main">
                      <span>摘要</span>
                      <p>{{ formData.summary || '暂无摘要' }}</p>
                    </section>
                    <section
                      v-for="block in summaryResultBlocks"
                      :key="block.key"
                      class="summary-result-block"
                      :class="`summary-block-${block.key}`"
                    >
                      <span>{{ block.label }}</span>
                      <div v-if="block.items.length" class="line-list" :class="{ 'keyword-list': block.key === 'keywords' }">
                        <p v-for="item in block.items" :key="item">{{ item }}</p>
                      </div>
                      <p v-else>暂无内容</p>
                    </section>
                  </div>
                </div>
              </el-tab-pane>

              <el-tab-pane label="检索详情" name="search">
                <div class="search-workbench">
                  <div class="search-status-head">
                    <div>
                      <strong>MySQL 关键词检索</strong>
                      <span>保存文档时同步更新分块和关键词；只有检索可用且已发布的文档参与 AI 助手召回。</span>
                    </div>
                    <el-tag :type="getSearchTagType(formData.searchStatus)" effect="plain">
                      {{ getSearchStatusLabel(formData.searchStatus) }}
                    </el-tag>
                  </div>
                  <dl class="search-facts">
                    <div><dt>分块数量</dt><dd>{{ formData.chunkCount || 0 }}</dd></div>
                    <div><dt>检索版本</dt><dd>v{{ formData.searchVersion || 1 }}</dd></div>
                    <div><dt>摘要状态</dt><dd>{{ getSummaryStatusLabel(formData.summaryStatus) }}</dd></div>
                    <div><dt>检索方式</dt><dd>MySQL 加权关键词</dd></div>
                  </dl>
                  <el-alert
                    v-if="formData.searchError || formData.summaryError"
                    :title="formData.searchError || formData.summaryError"
                    type="error"
                    show-icon
                    :closable="false"
                  />
                  <div class="search-actions">
                    <el-button type="primary" :icon="Refresh" :loading="refreshingSearch" @click="refreshSearchData">
                      更新分块与关键词
                    </el-button>
                  </div>
                </div>
              </el-tab-pane>
            </el-tabs>
          </template>

          <div v-else class="knowledge-home document-detail-empty">
            <div class="detail-empty-icon" aria-hidden="true">
              <el-icon><Document /></el-icon>
            </div>
            <strong>未选择文章</strong>
          </div>
        </section>
      </div>
    </section>

    <el-dialog
      v-model="groupDialogVisible"
      :title="groupDialogTitle"
      width="480px"
      class="knowledge-dialog"
      @closed="resetGroupDialog"
    >
      <el-form ref="groupFormRef" :model="groupForm" :rules="groupFormRules" label-position="top">
        <el-form-item label="分组名称" prop="name">
          <el-input v-model.trim="groupForm.name" maxlength="128" show-word-limit placeholder="例如数据库运维" />
        </el-form-item>
        <el-form-item label="上级分组">
          <el-tree-select
            v-model="groupForm.parentId"
            :data="movableGroupSelectData"
            check-strictly
            default-expand-all
            clearable
            placeholder="留空表示根分组"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model.trim="groupForm.description" type="textarea" :rows="3" maxlength="500" show-word-limit placeholder="说明该空间收录的知识范围" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="savingGroup" @click="submitGroup">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="importDialogVisible"
      title="导入 Markdown 文档"
      width="760px"
      class="knowledge-dialog"
      @closed="resetImportDialog"
    >
      <div class="import-dialog-body">
        <el-segmented
          v-model="importMode"
          :options="importModeOptions"
          class="import-mode-segment"
          @change="handleImportModeChange"
        />

        <el-upload
          v-if="importMode === 'file'"
          drag
          :auto-upload="false"
          :limit="1"
          accept=".md,.markdown,.txt"
          :file-list="importFileList"
          :on-change="handleImportFileChange"
          :on-remove="handleImportFileRemove"
          :on-exceed="handleImportFileExceed"
        >
          <el-icon class="el-icon--upload"><Upload /></el-icon>
          <div class="el-upload__text">拖拽文件到此处，或点击选择</div>
          <template #tip>
            <div class="el-upload__tip">支持 .md / .markdown / .txt</div>
          </template>
        </el-upload>

        <div v-else class="paste-import-panel">
          <el-input
            v-model="importForm.contentMarkdown"
            type="textarea"
            :rows="10"
            resize="vertical"
            placeholder="粘贴 Markdown 内容。标题可不填，后端会取正文第一行作为标题。"
          />
        </div>

        <el-form :model="importForm" label-position="left" class="import-form import-compact-form">
          <div class="form-grid">
            <el-form-item label="标题">
              <el-input
                v-model.trim="importForm.title"
                clearable
                :placeholder="importMode === 'file' ? '选择文件后自动填写' : '请输入文档标题'"
              />
            </el-form-item>
            <el-form-item label="文档类型">
              <el-select v-model="importForm.docType" style="width: 100%">
                <el-option v-for="item in docTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
            <el-form-item label="知识分组">
              <el-tree-select
                v-model="importForm.groupId"
                :data="groupSelectData"
                check-strictly
                default-expand-all
                clearable
                placeholder="选择导入位置"
                style="width: 100%"
              />
            </el-form-item>
            <el-form-item label="标签">
              <el-select
                v-model="importForm.tags"
                multiple
                collapse-tags
                collapse-tags-tooltip
                :max-collapse-tags="1"
                filterable
                allow-create
                default-first-option
                placeholder="输入后回车添加"
                style="width: 100%"
              >
              </el-select>
            </el-form-item>
          </div>
        </el-form>
      </div>

      <template #footer>
        <div class="dialog-footer">
          <el-button type="warning" @click="importDialogVisible = false">取消</el-button>
          <el-button type="success" :loading="importing" @click="submitImport">开始导入</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Check,
  CopyDocument,
  DArrowLeft,
  DArrowRight,
  Delete,
  Document,
  EditPen,
  MagicStick,
  MoreFilled,
  Plus,
  Refresh,
  Search,
  Upload
} from '@element-plus/icons-vue'
import aiopsApi from '@/api/aiops'
import closedFolderIcon from '@/assets/image/关闭文件夹.svg'
import documentTreeIcon from '@/assets/image/数据表.svg'
import openedFolderIcon from '@/assets/image/打开文件夹.svg'
import EditThinktank from './components/Edit_Thinktank.vue'
import { renderMarkdown } from './components/thinktankMarkdown'

const successCodes = new Set([0, 200])

const docTypeOptions = [
  { label: '故障复盘', value: 'incident' },
  { label: 'Runbook', value: 'runbook' },
  { label: 'FAQ', value: 'faq' },
  { label: '笔记', value: 'note' }
]

const statusOptions = [
  { label: '草稿', value: 'draft' },
  { label: '已发布', value: 'published' },
  { label: '已禁用', value: 'disabled' }
]

const importModeOptions = [
  { label: '上传文件', value: 'file' },
  { label: '粘贴内容', value: 'paste' }
]

const treeProps = {
  label: 'label',
  children: 'children'
}

const createDefaultMarkdown = title => `# ${title || '故障标题'}

## 1. 事件经过

## 2. 影响范围

## 3. 根本原因

## 4. 解决方案

### 4.1 临时处理

### 4.2 长期治理

## 5. 相关责任人

## 6. 经验教训

## 7. 后续改进项
`

const createFormData = () => ({
  id: '',
  title: '',
  docType: 'incident',
  status: 'draft',
  severity: '',
  groupId: '',
  groupPath: '',
  groupName: '',
  category: '',
  serviceName: '',
  env: 'prod',
  tags: [],
  keywords: [],
  alertName: '',
  logKeywords: [],
  owners: [],
  contentMarkdown: createDefaultMarkdown(),
  summary: '',
  summaryStatus: 'pending',
  summaryError: '',
  searchStatus: 'ready',
  searchError: '',
  searchVersion: 1,
  chunkCount: 0,
  symptoms: [],
  rootCauses: [],
  resolutionSteps: [],
  verification: [],
  rollbackPlan: '',
  updatedAt: ''
})

const createImportForm = () => ({
  title: '',
  docType: 'incident',
  status: 'published',
  groupId: '',
  tags: [],
  contentMarkdown: ''
})

const treeRef = ref(null)
const editThinktankRef = ref(null)
const detailTabsRef = ref(null)
const groupFormRef = ref(null)
const loadingList = ref(false)
const loadingTree = ref(false)
const loadingDetail = ref(false)
const saving = ref(false)
const searching = ref(false)
const summarizing = ref(false)
const importing = ref(false)
const refreshingSearch = ref(false)
const savingGroup = ref(false)
const providerLoading = ref(false)
const queryMode = ref('list')
const selectedDocId = ref('')
const detailRequestId = ref(0)
const selectedTreeId = ref('all')
const selectedTreeFilters = reactive({
  groupId: '',
  groupPath: ''
})
const currentDocument = ref(null)
const documentList = ref([])
const knowledgeGroups = ref([])
const total = ref(0)
const activeDetailTab = ref('preview')
const outlineCollapsed = ref(false)
const editMetadataVisible = ref(false)
const importDialogVisible = ref(false)
const importFileList = ref([])
const importMode = ref('file')
const groupDialogVisible = ref(false)
const groupDialogMode = ref('create')
const editingGroupId = ref('')
const groupForm = reactive({ parentId: '', name: '', description: '', sortOrder: 0 })
const providerOptions = ref([])
const summaryProviderId = ref('')
const previewContentRef = ref(null)
const activeOutlineId = ref('')

const listQuery = reactive({
  page: 1,
  pageSize: 100,
  keyword: '',
  groupId: '',
  recursive: true,
  groupName: '',
  category: '',
  docType: '',
  status: '',
  serviceName: '',
  env: ''
})

const formData = reactive(createFormData())
const importForm = reactive(createImportForm())

const hasDetail = computed(() => Boolean(currentDocument.value || selectedDocId.value || activeDetailTab.value === 'edit'))

const detailSubtitle = computed(() => {
  const parts = [getDocTypeLabel(formData.docType), getStatusLabel(formData.status)]
  if (formData.groupPath) parts.push(formData.groupPath)
  return parts.filter(Boolean).join(' / ')
})
const renderedMarkdown = computed(() => renderMarkdown(formData.contentMarkdown || ''))
const documentOutline = computed(() => {
  if (!hasDetail.value) return []
  const headings = String(formData.contentMarkdown || '')
    .split(/\r?\n/)
    .map((line, index) => {
      const matched = line.trim().match(/^(#{1,4})\s+(.+)$/)
      if (!matched) return null
      return {
        id: `knowledge-heading-${index}`,
        sourceLevel: matched[1].length,
        rawText: matched[2].trim()
      }
    })
    .filter(Boolean)

  if (headings[0]?.sourceLevel === 1) headings.shift()
  if (!headings.length) return []

  const baseLevel = Math.min(...headings.map(item => item.sourceLevel))
  const counters = []

  return headings.map(heading => {
    const requestedDepth = Math.max(0, heading.sourceLevel - baseLevel)
    const depth = Math.min(requestedDepth, counters.length)
    counters[depth] = (counters[depth] || 0) + 1
    counters.length = depth + 1
    const number = counters.join('.')
    const plainText = heading.rawText
      .replace(/\{\{(?:fg|bg):#[0-9a-f]{6}\}\}|\{\{\/(?:fg|bg)\}\}/gi, '')
      .replace(/[*_~`]/g, '')
      .trim() || heading.rawText
    const hasExplicitNumber = /^(?:\d+(?:\.\d+)*|[一二三四五六七八九十百零]+)(?:[.、:：)）]\s*|\s+)/.test(plainText)
    return {
      id: heading.id,
      level: depth + 1,
      text: hasExplicitNumber ? plainText : `${number}. ${plainText}`
    }
  })
})
const aiExtractionBlocks = computed(() => [
  { key: 'symptoms', label: '故障现象', items: formData.symptoms },
  { key: 'rootCauses', label: '根因分析', items: formData.rootCauses },
  { key: 'resolutionSteps', label: '处理步骤', items: formData.resolutionSteps },
  { key: 'verification', label: '验证方式', items: formData.verification }
].filter(block => block.items.length))
const aiExtractionCount = computed(() => aiExtractionBlocks.value.reduce((sum, block) => sum + block.items.length, 0))
const summaryResultBlocks = computed(() => [
  { key: 'symptoms', label: '故障现象', items: formData.symptoms },
  { key: 'rootCauses', label: '根因分析', items: formData.rootCauses },
  { key: 'resolutionSteps', label: '处理步骤', items: formData.resolutionSteps },
  { key: 'verification', label: '验证方式', items: formData.verification },
  { key: 'keywords', label: '关键词', items: formData.keywords }
])
const treeData = computed(() => {
  const groupedDocumentCount = knowledgeGroups.value.reduce((sum, group) => sum + Number(group.docCount || 0), 0)
  const documentsByGroup = new Map()
  const rootDocuments = []
  const mapDocument = document => ({
    id: `document:${document.id}`,
    documentId: document.id,
    label: document.title || '未命名文章',
    type: 'document',
    document,
    children: []
  })

  if (selectedTreeFilters.groupId || queryMode.value === 'search') {
    documentList.value.forEach(document => {
      const groupId = Number(document.groupId || selectedTreeFilters.groupId) || 0
      if (!groupId) {
        rootDocuments.push(mapDocument(document))
        return
      }
      if (!documentsByGroup.has(groupId)) documentsByGroup.set(groupId, [])
      documentsByGroup.get(groupId).push(mapDocument(document))
    })
  }

  const root = {
    id: 'all',
    label: queryMode.value === 'search' ? '搜索结果' : '全部文档',
    type: 'root',
    count: Math.max(groupedDocumentCount, total.value),
    filters: {},
    children: []
  }
  const mapGroup = group => {
    const childGroups = (group.children || []).map(mapGroup)
    const childDocuments = documentsByGroup.get(Number(group.id)) || []
    return {
      id: `group:${group.id}`,
      groupId: group.id,
      parentId: group.parentId || 0,
      label: group.name,
      path: group.path || group.name,
      description: group.description || '',
      sortOrder: group.sortOrder || 0,
      type: 'group',
      count: Number(group.docCount || 0),
      children: [...childGroups, ...childDocuments]
    }
  }
  root.children = [...rootDocuments, ...knowledgeGroups.value.map(mapGroup)]

  return [root]
})

const mapGroupSelectChildren = groups => (groups || [])
  .filter(group => group.type === 'group')
  .map(group => ({
    ...group,
    value: group.groupId,
    label: group.label,
    children: mapGroupSelectChildren(group.children)
  }))

const filterMovableGroups = (groups, excludedId) => (groups || [])
  .filter(group => Number(group.groupId) !== Number(excludedId))
  .map(group => ({ ...group, children: filterMovableGroups(group.children, excludedId) }))

const groupSelectData = computed(() => treeData.value[0].children
  .filter(group => group.type === 'group')
  .map(group => ({
    ...group,
    value: group.groupId,
    label: group.label,
    children: mapGroupSelectChildren(group.children)
  })))
const movableGroupSelectData = computed(() => filterMovableGroups(groupSelectData.value, Number(editingGroupId.value) || 0))
const groupDialogTitle = computed(() => ({ create: '新建知识分组', rename: '编辑知识分组', move: '移动知识分组' }[groupDialogMode.value] || '知识分组'))
const groupFormRules = { name: [{ required: true, message: '请输入分组名称', trigger: 'blur' }] }

const getBody = response => response?.data || response || {}

const assertSuccess = (response, fallbackMessage = '请求失败') => {
  const body = getBody(response)
  if (body.code !== undefined && !successCodes.has(Number(body.code))) {
    throw new Error(body.message || body.msg || fallbackMessage)
  }
  return body
}

const getPayload = (response, fallbackMessage = '请求失败') => {
  const body = assertSuccess(response, fallbackMessage)
  return body.data !== undefined ? body.data : body
}

const toArray = value => {
  if (Array.isArray(value)) return value.map(item => String(item || '').trim()).filter(Boolean)
  if (value === undefined || value === null || value === '') return []
  if (typeof value === 'object') return Object.values(value).map(item => String(item || '').trim()).filter(Boolean)
  const text = String(value).trim()
  if (!text) return []
  if (text.startsWith('[') || text.startsWith('{')) {
    try {
      return toArray(JSON.parse(text))
    } catch {
      return [text]
    }
  }
  return text.split(/[,，\n]/).map(item => item.trim()).filter(Boolean)
}

const normalizeListPayload = payload => {
  const listSource = Array.isArray(payload)
    ? payload
    : payload?.list || payload?.records || payload?.items || payload?.rows || payload?.data || []
  const list = Array.isArray(listSource) ? listSource : []
  const count = payload?.total ?? payload?.count ?? payload?.totalCount ?? payload?.pagination?.total ?? list.length
  return { list, count: Number(count) || list.length }
}

const normalizeDocument = (row = {}) => {
  const id = row.id ?? row.documentId ?? row.document_id ?? ''
  const docType = row.docType || row.doc_type || row.type || 'incident'
  const status = row.status || 'draft'
  return {
    ...row,
    id,
    clientKey: id || `${row.title || 'draft'}-${row.updatedAt || Date.now()}`,
    title: row.title || row.name || '',
    docType,
    status,
    severity: row.severity || '',
    groupId: Number(row.groupId ?? row.group_id ?? 0) || '',
    groupPath: row.groupPath || row.group_path || '',
    groupName: row.groupName || row.group_name || '',
    category: row.category || '',
    serviceName: row.serviceName || row.service_name || '',
    env: row.env || row.environment || '',
    tags: toArray(row.tags || row.tagsJson || row.tags_json),
    keywords: toArray(row.keywords || row.keywordsJson || row.keywords_json),
    alertName: row.alertName || row.alert_name || toArray(row.alertNames).join(','),
    logKeywords: toArray(row.logKeywords || row.logKeywordsJson || row.log_keywords_json),
    owners: toArray(row.owners || row.ownersJson || row.owners_json),
    contentMarkdown: row.contentMarkdown || row.content_markdown || row.markdown || row.content || '',
    summary: row.summary || '',
    summaryStatus: row.summaryStatus || row.summary_status || (row.summary ? 'completed' : 'pending'),
    summaryError: row.summaryError || row.summary_error || '',
    searchStatus: row.searchStatus || row.search_status || 'ready',
    searchError: row.searchError || row.search_error || '',
    searchVersion: Number(row.searchVersion ?? row.search_version ?? 1) || 1,
    chunkCount: Number(row.chunkCount ?? row.chunk_count ?? 0) || 0,
    symptoms: toArray(row.symptoms || row.symptomsJson || row.symptoms_json),
    rootCauses: toArray(row.rootCauses || row.rootCausesJson || row.root_causes_json),
    resolutionSteps: toArray(row.resolutionSteps || row.resolutionStepsJson || row.resolution_steps_json),
    verification: toArray(row.verification || row.verificationJson || row.verification_json),
    rollbackPlan: row.rollbackPlan || row.rollback_plan || '',
    matchedFields: toArray(row.matchedFields || row.matched_fields),
    score: row.score,
    snippet: row.snippet || '',
    updatedAt: row.updatedAt || row.updated_at || row.updateTime || row.update_time || row.createdAt || ''
  }
}

const normalizeSummaryPayload = payload => {
  const data = payload?.summaryResult || payload?.result || payload || {}
  return {
    summary: data.summary || '',
    symptoms: toArray(data.symptoms),
    rootCauses: toArray(data.rootCauses || data.root_causes),
    resolutionSteps: toArray(data.resolutionSteps || data.resolution_steps),
    verification: toArray(data.verification),
    keywords: toArray(data.keywords),
    usedLlm: data.usedLlm ?? data.used_llm
  }
}

const buildListParams = (extra = {}) => {
  const params = {
    page: listQuery.page,
    pageSize: listQuery.pageSize,
    keyword: listQuery.keyword,
    groupId: listQuery.groupId,
    recursive: listQuery.recursive,
    groupName: listQuery.groupName,
    category: listQuery.category,
    docType: listQuery.docType,
    status: listQuery.status,
    serviceName: listQuery.serviceName,
    env: listQuery.env,
    ...extra
  }
  Object.keys(params).forEach(key => {
    if (params[key] === '' || params[key] === undefined || params[key] === null) delete params[key]
  })
  return params
}

const buildDocumentPayload = () => ({
  title: formData.title,
  docType: formData.docType,
  status: formData.status,
  groupId: formData.groupId,
  groupName: formData.groupName,
  category: formData.category,
  severity: formData.severity,
  serviceName: formData.serviceName,
  env: formData.env,
  tags: formData.tags,
  keywords: formData.keywords,
  alertNames: toArray(formData.alertName),
  logKeywords: formData.logKeywords,
  owners: formData.owners,
  contentMarkdown: formData.contentMarkdown,
  content: formData.contentMarkdown,
  summary: formData.summary,
  symptoms: formData.symptoms,
  rootCauses: formData.rootCauses,
  resolutionSteps: formData.resolutionSteps,
  verification: formData.verification,
  rollbackPlan: formData.rollbackPlan
})

const buildCreateDocumentPayload = () => {
  const payload = {
    title: formData.title,
    docType: formData.docType,
    groupId: formData.groupId,
    contentMarkdown: formData.contentMarkdown
  }
  if (formData.status && formData.status !== 'draft') payload.status = formData.status
  if (formData.env && formData.env !== 'prod') payload.env = formData.env
  if (formData.serviceName?.trim()) payload.serviceName = formData.serviceName.trim()
  if (formData.tags.length) payload.tags = formData.tags
  return payload
}

const fillForm = document => {
  const normalized = normalizeDocument(document)
  Object.assign(formData, createFormData(), {
    id: normalized.id,
    title: normalized.title,
    docType: normalized.docType,
    status: normalized.status,
    severity: normalized.severity,
    groupId: normalized.groupId,
    groupPath: normalized.groupPath,
    groupName: normalized.groupName,
    category: normalized.category,
    serviceName: normalized.serviceName,
    env: normalized.env || 'prod',
    tags: normalized.tags,
    keywords: normalized.keywords,
    alertName: normalized.alertName,
    logKeywords: normalized.logKeywords,
    owners: normalized.owners,
    contentMarkdown: normalized.contentMarkdown || createDefaultMarkdown(normalized.title),
    summary: normalized.summary,
    summaryStatus: normalized.summaryStatus,
    summaryError: normalized.summaryError,
    searchStatus: normalized.searchStatus,
    searchError: normalized.searchError,
    searchVersion: normalized.searchVersion,
    chunkCount: normalized.chunkCount,
    symptoms: normalized.symptoms,
    rootCauses: normalized.rootCauses,
    resolutionSteps: normalized.resolutionSteps,
    verification: normalized.verification,
    rollbackPlan: normalized.rollbackPlan,
    updatedAt: normalized.updatedAt
  })
  editThinktankRef.value?.syncFromMarkdown()
}

const clearDetail = () => {
  detailRequestId.value += 1
  loadingDetail.value = false
  editMetadataVisible.value = false
  activeOutlineId.value = ''
  selectedDocId.value = ''
  currentDocument.value = null
  Object.assign(formData, createFormData())
  activeDetailTab.value = 'preview'
}

const loadTreeDocuments = async () => {
  loadingTree.value = true
  try {
    const response = await aiopsApi.listKnowledgeGroups()
    const payload = getPayload(response, '获取知识分组失败')
    knowledgeGroups.value = Array.isArray(payload) ? payload : []
    await nextTick()
    treeRef.value?.setCurrentKey(selectedTreeId.value)
  } catch (error) {
    console.error('load knowledge groups failed:', error)
    ElMessage.error(error.message || '获取知识分组失败')
  } finally {
    loadingTree.value = false
  }
}

const loadDocuments = async (autoSelect = false) => {
  queryMode.value = 'list'
  loadingList.value = true
  try {
    const response = await aiopsApi.listKnowledgeDocuments(buildListParams())
    const { list, count } = normalizeListPayload(getPayload(response, '获取知识文档失败'))
    documentList.value = list.map(normalizeDocument)
    total.value = count

    if (autoSelect && documentList.value.length && !selectedDocId.value) {
      await openDocument(documentList.value[0])
    } else if (!autoSelect && selectedDocId.value) {
      const currentStillVisible = documentList.value.some(item => String(item.id) === String(selectedDocId.value))
      if (!currentStillVisible) {
        clearDetail()
        selectedTreeId.value = selectedTreeFilters.groupId ? `group:${selectedTreeFilters.groupId}` : 'all'
      }
    } else if (!documentList.value.length) {
      clearDetail()
    }
    await nextTick()
    treeRef.value?.setCurrentKey(selectedTreeId.value)
  } catch (error) {
    console.error('load knowledge documents failed:', error)
    ElMessage.error(error.message || '获取知识文档失败')
  } finally {
    loadingList.value = false
  }
}

const loadProviders = async () => {
  providerLoading.value = true
  try {
    const response = await aiopsApi.getEnabledLLMProviders()
    const payload = getPayload(response, '获取 Provider 失败')
    providerOptions.value = Array.isArray(payload) ? payload : payload?.list || payload?.records || payload?.items || []
  } catch (error) {
    console.warn('load providers failed:', error)
  } finally {
    providerLoading.value = false
  }
}

const openDocument = async doc => {
  editMetadataVisible.value = false
  if (!doc?.id) {
    detailRequestId.value += 1
    loadingDetail.value = false
    currentDocument.value = normalizeDocument(doc)
    fillForm(currentDocument.value)
    activeDetailTab.value = 'preview'
    return
  }

  selectedTreeId.value = `document:${doc.id}`
  selectedDocId.value = doc.id
  activeOutlineId.value = ''
  currentDocument.value = normalizeDocument(doc)
  fillForm(currentDocument.value)
  activeDetailTab.value = activeDetailTab.value === 'edit' ? 'preview' : activeDetailTab.value
  const requestId = detailRequestId.value + 1
  detailRequestId.value = requestId
  loadingDetail.value = true

  try {
    const response = await aiopsApi.getKnowledgeDocument(doc.id)
    if (requestId !== detailRequestId.value || String(selectedDocId.value) !== String(doc.id)) return
    currentDocument.value = normalizeDocument(getPayload(response, '获取文档详情失败'))
    fillForm(currentDocument.value)
  } catch (error) {
    if (requestId !== detailRequestId.value) return
    console.error('load knowledge document detail failed:', error)
    ElMessage.error(error.message || '获取文档详情失败')
  } finally {
    if (requestId === detailRequestId.value) loadingDetail.value = false
  }
  await nextTick()
  treeRef.value?.setCurrentKey(selectedTreeId.value)
}

const scrollToHeading = async id => {
  activeOutlineId.value = id
  if (activeDetailTab.value !== 'preview') activeDetailTab.value = 'preview'
  await nextTick()
  const heading = previewContentRef.value?.querySelector(`[data-heading-id="${id}"]`)
  if (!heading) return
  const reduceMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)')?.matches
  heading.scrollIntoView({ behavior: reduceMotion ? 'auto' : 'smooth', block: 'start' })
}

const switchDetailTab = tab => {
  if ((tab === 'ai' || tab === 'search') && !formData.id) return
  editMetadataVisible.value = false
  activeDetailTab.value = tab
}

const openEditTab = () => {
  editMetadataVisible.value = false
  outlineCollapsed.value = true
  activeDetailTab.value = 'edit'
}

const handleDocumentAction = command => {
  if (command === 'summarize') summarizeDocument()
  if (command === 'refresh') refreshSearchData()
  if (command === 'delete') deleteDocument()
}

const openCreate = () => {
  selectedDocId.value = ''
  currentDocument.value = null
  Object.assign(formData, createFormData(), {
    groupId: selectedTreeFilters.groupId || '',
    groupPath: selectedTreeFilters.groupPath || ''
  })
  outlineCollapsed.value = true
  activeDetailTab.value = 'edit'
  editThinktankRef.value?.syncFromMarkdown()
  nextTick(() => {
    editMetadataVisible.value = true
  })
}

const handleTreeNodeClick = async data => {
  if (data.type === 'document') {
    await openDocument(data.document)
    return
  }

  selectedTreeId.value = data.id
  selectedTreeFilters.groupId = data.groupId || ''
  selectedTreeFilters.groupPath = data.path || ''
  listQuery.groupId = selectedTreeFilters.groupId
  listQuery.page = 1
  clearDetail()
  await loadDocuments()
  await nextTick()
  treeRef.value?.getNode(data.id)?.expand()
  treeRef.value?.setCurrentKey(data.id)
}

const findKnowledgeGroup = id => {
  const visit = groups => {
    for (const group of groups || []) {
      if (Number(group.id) === Number(id)) return group
      const child = visit(group.children)
      if (child) return child
    }
    return null
  }
  return visit(knowledgeGroups.value)
}

const resetGroupDialog = () => {
  Object.assign(groupForm, { parentId: '', name: '', description: '', sortOrder: 0 })
  editingGroupId.value = ''
  groupDialogMode.value = 'create'
  savingGroup.value = false
  groupFormRef.value?.clearValidate?.()
}

const openGroupDialog = (mode = 'create', data = null) => {
  const group = data?.groupId ? findKnowledgeGroup(data.groupId) : null
  groupDialogMode.value = mode
  editingGroupId.value = group?.id || ''
  if (mode === 'create') {
    Object.assign(groupForm, {
      parentId: data?.groupId || '',
      name: '',
      description: '',
      sortOrder: 0
    })
  } else {
    Object.assign(groupForm, {
      parentId: group?.parentId || '',
      name: group?.name || data?.label || '',
      description: group?.description || '',
      sortOrder: group?.sortOrder || 0
    })
  }
  groupDialogVisible.value = true
}

const handleTreeNodeContextMenu = (event, data) => {
  event?.preventDefault?.()
  if (data?.type === 'group') openGroupDialog('create', data)
}

const deleteKnowledgeGroup = async data => {
  try {
    await ElMessageBox.confirm(`确定删除空分组「${data.label}」吗？`, '删除知识分组', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      confirmButtonClass: 'el-button--danger'
    })
    await aiopsApi.deleteKnowledgeGroup(data.groupId)
    if (Number(selectedTreeFilters.groupId) === Number(data.groupId)) {
      selectedTreeId.value = 'all'
      selectedTreeFilters.groupId = ''
      selectedTreeFilters.groupPath = ''
      listQuery.groupId = ''
    }
    ElMessage.success('分组已删除')
    await refreshKnowledgeTree()
  } catch (error) {
    if (error !== 'cancel' && error?.message) ElMessage.error(error.message)
  }
}

const handleGroupCommand = (command, data) => {
  if (command === 'delete') {
    deleteKnowledgeGroup(data)
    return
  }
  openGroupDialog(command, data)
}

const submitGroup = async () => {
  try {
    await groupFormRef.value?.validate()
    savingGroup.value = true
    const payload = {
      parentId: Number(groupForm.parentId) || 0,
      name: groupForm.name,
      description: groupForm.description,
      sortOrder: Number(groupForm.sortOrder) || 0
    }
    if (groupDialogMode.value === 'create') {
      await aiopsApi.createKnowledgeGroup(payload)
    } else {
      await aiopsApi.updateKnowledgeGroup(editingGroupId.value, payload)
    }
    ElMessage.success(groupDialogMode.value === 'create' ? '分组已创建' : '分组已更新')
    groupDialogVisible.value = false
    await loadTreeDocuments()
  } catch (error) {
    if (error?.message) ElMessage.error(error.message)
  } finally {
    savingGroup.value = false
  }
}

const handleQuery = () => {
  listQuery.page = 1
  selectedTreeId.value = 'all'
  selectedTreeFilters.groupId = ''
  selectedTreeFilters.groupPath = ''
  listQuery.groupId = ''
  listQuery.groupName = ''
  listQuery.category = ''
  clearDetail()
  loadDocuments()
}

const refreshKnowledgeTree = async () => {
  await Promise.all([loadTreeDocuments(), loadDocuments()])
}

const handleKnowledgeSearch = async () => {
  if (!listQuery.keyword) {
    ElMessage.warning('请输入检索关键词')
    return
  }

  searching.value = true
  selectedTreeId.value = 'all'
  selectedTreeFilters.groupId = ''
  selectedTreeFilters.groupPath = ''
  listQuery.groupId = ''
  listQuery.groupName = ''
  listQuery.category = ''
  clearDetail()
  try {
    const payload = buildListParams({
      query: listQuery.keyword,
      limit: listQuery.pageSize
    })
    delete payload.page
    delete payload.pageSize
    delete payload.keyword
    const response = await aiopsApi.searchKnowledge(payload)
    const data = getPayload(response, '知识检索失败')
    const list = Array.isArray(data) ? data : normalizeListPayload(data).list
    documentList.value = list.map(normalizeDocument)
    total.value = documentList.value.length
    queryMode.value = 'search'
  } catch (error) {
    console.error('knowledge search failed:', error)
    ElMessage.error(error.message || '知识检索失败')
  } finally {
    searching.value = false
  }
}

const saveDocument = async () => {
  try {
    const syncedMarkdown = editThinktankRef.value?.syncToMarkdown()
    if (typeof syncedMarkdown === 'string') {
      formData.contentMarkdown = syncedMarkdown
    }
    if (!String(formData.contentMarkdown || '').trim()) {
      ElMessage.warning('请输入文档正文')
      return
    }
    if (!formData.title?.trim() || !formData.docType || !formData.groupId) {
      editMetadataVisible.value = true
      await nextTick()
    }
    await editThinktankRef.value?.validate()
    saving.value = true
    const payload = formData.id ? buildDocumentPayload() : buildCreateDocumentPayload()
    const response = formData.id
      ? await aiopsApi.updateKnowledgeDocument(formData.id, payload)
      : await aiopsApi.createKnowledgeDocument(payload)
    const saved = normalizeDocument(getPayload(response, formData.id ? '保存失败' : '创建失败'))
    ElMessage.success(formData.id ? '保存成功' : '创建成功')
    if (saved.id) {
      selectedDocId.value = saved.id
      currentDocument.value = saved
      fillForm(saved)
    }
    editMetadataVisible.value = false
    activeDetailTab.value = 'preview'
    await Promise.all([loadDocuments(), loadTreeDocuments()])
    if (selectedDocId.value) await openDocument({ id: selectedDocId.value })
  } catch (error) {
    if (error?.message) ElMessage.error(error.message)
  } finally {
    saving.value = false
  }
}

const cancelEdit = () => {
  editMetadataVisible.value = false
  if (currentDocument.value) {
    fillForm(currentDocument.value)
    activeDetailTab.value = 'preview'
    return
  }
  clearDetail()
  selectedTreeId.value = selectedTreeFilters.groupId ? `group:${selectedTreeFilters.groupId}` : 'all'
  nextTick(() => treeRef.value?.setCurrentKey(selectedTreeId.value))
}

const deleteDocument = async () => {
  if (!formData.id) return
  try {
    await ElMessageBox.confirm(`确定删除「${formData.title || formData.id}」吗？`, '删除知识文档', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      confirmButtonClass: 'el-button--danger'
    })
    await aiopsApi.deleteKnowledgeDocument(formData.id)
    ElMessage.success('删除成功')
    clearDetail()
    selectedTreeId.value = selectedTreeFilters.groupId ? `group:${selectedTreeFilters.groupId}` : 'all'
    await Promise.all([loadDocuments(), loadTreeDocuments()])
  } catch (error) {
    if (error !== 'cancel' && error?.message) ElMessage.error(error.message)
  }
}

const summarizeDocument = async () => {
  if (!formData.id) {
    ElMessage.warning('请先保存文档')
    return
  }
  summarizing.value = true
  try {
    const data = summaryProviderId.value ? { providerId: summaryProviderId.value } : {}
    const response = await aiopsApi.summarizeKnowledgeDocument(formData.id, data)
    const summary = normalizeSummaryPayload(getPayload(response, 'AI 总结失败'))
    Object.assign(formData, {
      summary: summary.summary || formData.summary,
      symptoms: summary.symptoms.length ? summary.symptoms : formData.symptoms,
      rootCauses: summary.rootCauses.length ? summary.rootCauses : formData.rootCauses,
      resolutionSteps: summary.resolutionSteps.length ? summary.resolutionSteps : formData.resolutionSteps,
      verification: summary.verification.length ? summary.verification : formData.verification,
      keywords: summary.keywords.length ? Array.from(new Set([...formData.keywords, ...summary.keywords])) : formData.keywords
    })
    ElMessage.success(summary.usedLlm === false ? '已完成结构化提取' : 'AI 总结已完成')
    activeDetailTab.value = 'ai'
    await openDocument({ id: formData.id })
  } catch (error) {
    console.error('summarize knowledge document failed:', error)
    ElMessage.error(error.message || 'AI 总结失败')
  } finally {
    summarizing.value = false
  }
}

const openImportDialog = () => {
  importDialogVisible.value = true
  importMode.value = 'file'
  Object.assign(importForm, createImportForm(), {
    groupId: selectedTreeFilters.groupId || ''
  })
}

const refreshSearchData = async () => {
  if (!formData.id) return
  refreshingSearch.value = true
  try {
    const response = await aiopsApi.refreshKnowledgeSearchData(formData.id)
    const document = normalizeDocument(getPayload(response, '更新检索数据失败'))
    currentDocument.value = document
    fillForm(document)
    activeDetailTab.value = 'search'
    ElMessage.success('检索分块与关键词已更新')
    await loadTreeDocuments()
  } catch (error) {
    ElMessage.error(error.message || '更新检索数据失败')
  } finally {
    refreshingSearch.value = false
  }
}

const resetImportDialog = () => {
  importFileList.value = []
  importMode.value = 'file'
  Object.assign(importForm, createImportForm())
  importing.value = false
}

const handleImportModeChange = mode => {
  if (mode === 'file') {
    importForm.contentMarkdown = ''
  } else {
    importFileList.value = []
  }
}

const handleImportFileChange = (file, files) => {
  const name = file.name || file.raw?.name || ''
  const matched = /\.(md|markdown|txt)$/i.test(name)
  if (!matched) {
    ElMessage.warning('仅支持 .md / .markdown / .txt 文件')
    importFileList.value = []
    importForm.title = ''
    return
  }
  importFileList.value = files.slice(-1)
  importForm.title = name.replace(/\.(md|markdown|txt)$/i, '').trim()
}

const handleImportFileRemove = () => {
  importFileList.value = []
  importForm.title = ''
}

const handleImportFileExceed = files => {
  const file = files[files.length - 1]
  if (!file) return
  const name = file.name || ''
  if (!/\.(md|markdown|txt)$/i.test(name)) {
    ElMessage.warning('仅支持 .md / .markdown / .txt 文件')
    return
  }
  importFileList.value = [{ name, raw: file }]
  importForm.title = name.replace(/\.(md|markdown|txt)$/i, '').trim()
}

const submitImport = async () => {
  const file = importFileList.value[0]?.raw
  const contentMarkdown = String(importForm.contentMarkdown || '').trim()
  if (importMode.value === 'file' && !file) {
    ElMessage.warning('请选择要导入的文档')
    return
  }
  if (importMode.value === 'paste' && !contentMarkdown) {
    ElMessage.warning('请粘贴 Markdown 内容')
    return
  }
  importing.value = true
  try {
    const form = new FormData()
    if (importMode.value === 'file') {
      form.append('file', file)
    } else {
      form.append('contentMarkdown', contentMarkdown)
    }
    Object.entries(importForm).forEach(([key, value]) => {
      if (key === 'contentMarkdown') return
      if (Array.isArray(value)) {
        if (value.length) form.append(key, JSON.stringify(value))
        return
      }
      if (value !== undefined && value !== null && value !== '') form.append(key, value)
    })
    const response = await aiopsApi.importKnowledgeDocument(form)
    const imported = normalizeDocument(getPayload(response, '导入失败'))
    ElMessage.success('导入成功')
    importDialogVisible.value = false
    await Promise.all([loadDocuments(), loadTreeDocuments()])
    if (imported.id) openDocument(imported)
  } catch (error) {
    console.error('import knowledge document failed:', error)
    ElMessage.error(error.message || '导入失败')
  } finally {
    importing.value = false
  }
}

const copyMarkdown = async () => {
  if (!formData.contentMarkdown) return
  try {
    await navigator.clipboard.writeText(formData.contentMarkdown)
    ElMessage.success('Markdown 已复制')
  } catch {
    ElMessage.warning('当前浏览器不支持自动复制')
  }
}

const getProviderId = provider => provider?.id ?? provider?.providerId ?? provider?.value

const getProviderLabel = provider => {
  const name = provider?.modelName || provider?.name || provider?.label || provider?.modelCode || provider?.value
  const code = provider?.modelCode || provider?.model || provider?.type || ''
  return code && code !== name ? `${name} / ${code}` : name || '默认 Provider'
}

const getDocTypeLabel = value => docTypeOptions.find(item => item.value === value)?.label || value || '-'
const getStatusLabel = value => statusOptions.find(item => item.value === value)?.label || value || '-'
const getSearchStatusLabel = value => ({ ready: '检索可用', failed: '检索数据异常' }[value] || '检索可用')
const getSearchTagType = value => ({ ready: 'success', failed: 'danger' }[value] || 'success')
const getSummaryStatusLabel = value => ({ pending: '等待总结', processing: '总结处理中', completed: '总结完成', failed: '总结失败' }[value] || '等待总结')

watch(activeDetailTab, async tab => {
  if (tab !== 'edit') {
    editThinktankRef.value?.hideTableEdgeControls()
    return
  }
  await editThinktankRef.value?.syncFromMarkdown()
  await nextTick()
  const editPane = detailTabsRef.value?.$el?.querySelector('.edit-tab-pane')
  if (editPane) editPane.scrollTop = 0
  editThinktankRef.value?.scrollToTop()
})

onMounted(async () => {
  await Promise.all([loadTreeDocuments(), loadProviders()])
  await loadDocuments()
  editThinktankRef.value?.syncFromMarkdown()
})
</script>

<style scoped>
:global(.home-container .el-main:has(.thinktank-page)) {
  padding-top: 10px;
  padding-right: 10px;
  padding-left: 10px;
  overflow-y: hidden;
}

.thinktank-page {
  --think-page-height-offset: 89px;
  --think-bg-page: transparent;
  --think-surface-cool: #fbfdff;
  --think-surface-warm: #ffffff;
  --think-text-primary: #172033;
  --think-text-secondary: #53657d;
  --think-text-muted: #64748b;
  --think-border-cool: #dbe7f4;
  --think-border-mid: #dce8f5;
  --think-brand: #409eff;
  --think-brand-active: #2563eb;
  --think-brand-soft: #eef6ff;
  --think-success: #10b981;
  --think-warning: #f59e0b;
  --think-danger: #ef4444;
  --think-topbar-gradient: linear-gradient(90deg, #fff6e7 0%, #f7fbff 36%, #ecf7ff 100%);
  --think-panel-gradient: #ffffff;
  --think-sidebar-gradient: linear-gradient(180deg, #f5f9ff 0%, #eef5ff 100%);
  --think-border-soft: #e7eef7;
  --think-shadow-soft: 0 8px 24px rgba(80, 124, 180, 0.07);
  box-sizing: border-box;
  height: calc(100vh - var(--think-page-height-offset));
  max-height: calc(100vh - var(--think-page-height-offset));
  min-height: 0;
  padding: 7px;
  overflow: hidden;
  background: var(--think-bg-page);
  color: var(--think-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.thinktank-card {
  position: relative;
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--think-border-cool);
  border-radius: 12px;
  background: var(--think-panel-gradient);
  box-shadow: var(--think-shadow-soft);
}

.thinktank-card::before {
  position: absolute;
  inset: 0 0 auto;
  z-index: 2;
  height: 3px;
  background: linear-gradient(90deg, #f5a524 0%, #62a8ff 48%, #55d6be 100%);
  content: '';
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 44px;
  padding: 4px 12px;
  border-bottom: 1px solid var(--think-border-soft);
  background: var(--think-topbar-gradient);
}

.header-brand {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
}

.header-brand-icon {
  display: grid;
  width: 32px;
  height: 32px;
  flex: 0 0 32px;
  place-items: center;
}

.header-brand-icon img {
  width: 24px;
  height: 24px;
  object-fit: contain;
}

.header-title h2,
.header-title span {
  margin: 0;
}

.header-title {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
}

.header-title h2 {
  flex: 0 0 auto;
  color: var(--think-text-primary);
  font-size: 17px;
  font-weight: 700;
  line-height: 1.2;
}

.knowledge-toolbar {
  display: flex;
  align-items: center;
  gap: 5px;
  min-height: 42px;
  padding: 5px 6px;
  border-bottom: 1px solid var(--think-border-soft);
  background: #f8fbff;
}

.header-commands {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.header-commands {
  flex: 0 0 auto;
}

.global-search {
  min-width: 0;
  width: auto;
  flex: 1 1 auto;
}

.workspace {
  flex: 1 1 auto;
  min-height: 0;
  display: grid;
  grid-template-columns: 244px 244px minmax(520px, 1fr);
  gap: 0;
  padding: 0;
  overflow: hidden;
}

.workspace.outline-collapsed {
  grid-template-columns: 244px 42px minmax(520px, 1fr);
}

.tree-panel,
.document-panel,
.detail-panel {
  min-width: 0;
  min-height: 0;
  border: 0;
  backdrop-filter: none;
}

.tree-panel {
  display: grid;
  grid-template-rows: auto auto minmax(0, 1fr);
  border-radius: 0;
  border-right: 1px solid var(--think-border-mid);
  background: var(--think-sidebar-gradient);
}

.document-panel {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  border-right: 1px solid var(--think-border-mid);
  background: #fff;
}

.detail-panel {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: 0;
  background: #fbfdff;
}

.outline-panel-head {
  min-height: 40px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--think-border-mid);
  background: #fbfdff;
}

.outline-toggle {
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 28px;
  padding: 0;
  border: 1px solid #f59e0b;
  border-radius: 6px;
  background: #fbbf24;
  color: #713f12;
  cursor: pointer;
  box-shadow: 0 1px 3px rgba(180, 83, 9, 0.18);
  transition: border-color 0.16s ease, background-color 0.16s ease, color 0.16s ease, box-shadow 0.16s ease;
}

.outline-toggle:hover,
.outline-toggle:focus-visible {
  outline: none;
  border-color: #d97706;
  background: #f59e0b;
  color: #451a03;
  box-shadow: 0 0 0 3px rgba(245, 158, 11, 0.2);
}

.document-panel.collapsed .outline-panel-head {
  justify-content: center;
  padding: 6px;
}

.document-panel.collapsed .outline-panel-head strong,
.document-panel.collapsed .outline-scroll {
  display: none;
}

.outline-panel-head strong {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.outline-panel-head strong {
  color: var(--think-text-primary);
  font-size: 12px;
  font-weight: 750;
}

.outline-scroll {
  min-height: 0;
  padding: 6px;
}

.outline-list {
  display: grid;
  gap: 1px;
}

.outline-item {
  position: relative;
  width: 100%;
  min-height: 30px;
  overflow: hidden;
  padding: 5px 8px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--think-text-secondary);
  cursor: pointer;
  font-size: 12px;
  line-height: 1.5;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: background-color 0.16s ease, color 0.16s ease;
}

.outline-item:hover {
  background: #f1f6fc;
  color: var(--think-brand-active);
}

.outline-item:focus-visible {
  outline: 2px solid rgba(37, 99, 235, 0.38);
  outline-offset: -2px;
}

.outline-item.active {
  background: #e8f2ff;
  color: var(--think-brand-active);
  font-weight: 700;
}

.outline-item.level-2 {
  padding-left: 22px;
}

.outline-item.level-3 {
  padding-left: 36px;
}

.outline-item.level-4 {
  padding-left: 50px;
}

.outline-empty {
  min-height: 160px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 7px;
  color: var(--think-text-muted);
  font-size: 11px;
}

.outline-empty .el-icon {
  color: #9ab4d3;
  font-size: 24px;
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 40px;
  padding: 6px 10px;
  border-bottom: 1px solid var(--think-border-mid);
  background: #f8fbff;
}

.panel-head > div:first-child {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.panel-head h3,
.panel-head p {
  margin: 0;
}

.panel-head h3 {
  color: var(--think-text-primary);
  font-size: 12px;
  font-weight: 750;
  line-height: 1.2;
  white-space: nowrap;
}

.panel-head p {
  margin-top: 0;
  overflow: hidden;
  color: var(--think-text-muted);
  font-size: 10px;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tree-head-actions {
  display: flex;
  align-items: center;
  gap: 4px;
  flex: 0 0 auto;
}

.tree-head-actions :deep(.el-button) {
  width: 24px;
  height: 24px;
  margin-left: 0;
  padding: 0;
  border-color: #c9dbef;
  border-radius: 8px;
  background: #fff;
  color: var(--think-brand-active);
}

.tree-head-actions :deep(.el-button:hover),
.tree-head-actions :deep(.el-button:focus-visible) {
  border-color: #87b7f6;
  background: var(--think-brand-soft);
}

.tree-refresh-btn {
  width: 24px;
  height: 24px;
  flex: 0 0 24px;
}

.tree-scroll {
  min-height: 0;
  padding: 4px 6px 6px;
}

.knowledge-tree {
  background: transparent;
}

.knowledge-tree :deep(.el-tree-node__content) {
  min-height: 28px;
  margin: 1px 0;
  border: 1px solid transparent;
  border-radius: 8px;
  transition: background-color 0.18s ease, border-color 0.18s ease, box-shadow 0.18s ease;
}

.knowledge-tree :deep(.el-tree-node__content:hover) {
  background: rgba(255, 255, 255, 0.72);
}

.knowledge-tree :deep(.el-tree-node.is-current > .el-tree-node__content) {
  border-color: var(--think-brand);
  background: var(--think-brand);
  box-shadow: none;
}

.knowledge-tree :deep(.el-tree-node.is-current > .el-tree-node__content .tree-node span),
.knowledge-tree :deep(.el-tree-node.is-current > .el-tree-node__content .tree-node small),
.knowledge-tree :deep(.el-tree-node.is-current > .el-tree-node__content .tree-node-more),
.knowledge-tree :deep(.el-tree-node.is-current > .el-tree-node__content .tree-document-icon) {
  color: #fff;
}

.knowledge-tree :deep(.el-tree-node.is-current > .el-tree-node__content .el-tree-node__expand-icon) {
  filter: brightness(0) invert(1);
}

.knowledge-tree :deep(.el-tree-node__expand-icon) {
  width: 14px;
  height: 14px;
  padding: 0;
  margin: 0 4px 0 2px;
  flex: 0 0 14px;
  background: url('../../assets/image/折叠.svg') center / 12px 12px no-repeat;
  color: transparent;
  transform: rotate(-90deg);
}

.knowledge-tree :deep(.el-tree-node__expand-icon.expanded) {
  transform: rotate(0deg);
}

.knowledge-tree :deep(.el-tree-node__expand-icon.is-leaf) {
  background: none;
}

.tree-node {
  display: flex;
  align-items: center;
  gap: 7px;
  width: 100%;
  min-width: 0;
}

.tree-node-icon {
  width: 16px;
  height: 16px;
  object-fit: contain;
  flex-shrink: 0;
}

.tree-document-icon {
  color: var(--think-brand-active);
  font-size: 16px;
}

.tree-node span {
  min-width: 0;
  overflow: hidden;
  color: var(--think-text-primary);
  font-size: 13px;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tree-node small {
  margin-left: auto;
  color: var(--think-text-muted);
  font-size: 11px;
}

.tree-node-more {
  width: 24px;
  height: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 24px;
  margin: -2px 0 -2px 2px;
  padding: 0;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: var(--think-text-muted);
  cursor: pointer;
  transition: background-color 0.2s ease, color 0.2s ease;
}

.tree-node-more:hover,
.tree-node-more:focus-visible {
  outline: none;
  background: rgba(76, 139, 245, 0.1);
  color: var(--think-brand-active);
}

.tree-node.type-document {
  padding-left: 2px;
}

.tree-node.type-document span {
  font-weight: 600;
}

.detail-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 48px;
  padding: 4px 10px;
  border-bottom: 1px solid var(--think-border-soft);
  background: #fff;
}

.detail-head-main {
  display: flex;
  align-items: center;
  gap: 14px;
  flex: 1 1 auto;
  min-width: 0;
}

.detail-title {
  display: flex;
  align-items: center;
  gap: 7px;
  flex: 0 1 280px;
  min-width: 0;
}

.detail-title > div:last-child {
  display: block;
  flex: 1 1 auto;
  min-width: 0;
}

.document-mark {
  width: 26px;
  height: 26px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 26px;
  border: 1px solid #cfe2fb;
  border-radius: 8px;
  background: var(--think-brand-soft);
  color: var(--think-brand-active);
}

.detail-title h3,
.detail-title p {
  margin: 0;
}

.detail-title h3 {
  overflow: hidden;
  color: var(--think-text-primary);
  font-size: 14px;
  font-weight: 750;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-title p {
  margin-top: 2px;
  overflow: hidden;
  color: var(--think-text-muted);
  font-size: 11px;
  line-height: 1.2;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-tab-nav {
  height: 40px;
  display: flex;
  align-items: stretch;
  gap: 2px;
  flex: 0 0 auto;
}

.detail-tab-nav button {
  position: relative;
  min-width: 48px;
  padding: 0 9px;
  border: 0;
  background: transparent;
  color: var(--think-text-muted);
  cursor: pointer;
  font-size: 12px;
  font-weight: 650;
  white-space: nowrap;
  transition: color 0.16s ease, background-color 0.16s ease;
}

.detail-tab-nav button:hover:not(:disabled),
.detail-tab-nav button:focus-visible {
  outline: none;
  background: #f5f8fc;
  color: var(--think-text-primary);
}

.detail-tab-nav button.active {
  color: var(--think-brand-active);
}

.detail-tab-nav button.active::after {
  position: absolute;
  right: 9px;
  bottom: 0;
  left: 9px;
  height: 2px;
  border-radius: 2px;
  background: var(--think-brand);
  content: '';
}

.detail-tab-nav button:disabled {
  cursor: not-allowed;
  opacity: 0.42;
}

.detail-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex: 0 0 auto;
  flex-wrap: nowrap;
  gap: 4px;
}

.detail-actions :deep(.el-button + .el-button) {
  margin-left: 0;
}

.knowledge-home {
  position: relative;
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  padding: 10px;
  background: #f8fbff;
}

.document-detail-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 8px;
  color: var(--think-text-muted);
}

.detail-empty-icon {
  width: 48px;
  height: 48px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #d5e4f5;
  border-radius: 8px;
  background: #fff;
  color: #7ea7d8;
  font-size: 23px;
}

.document-detail-empty > strong {
  color: var(--think-text-secondary);
  font-size: 13px;
  font-weight: 650;
}

.home-overview,
.home-panel {
  border: 1px solid var(--think-border-cool);
  border-radius: 8px;
  background: #fff;
}

.home-overview {
  overflow: hidden;
}

.home-overview-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 38px;
  padding: 5px 10px;
  background: var(--think-topbar-gradient);
}

.home-overview-head span,
.home-overview-head h3 {
  display: block;
  margin: 0;
}

.home-overview-head span {
  color: var(--think-text-muted);
  font-size: 10px;
}

.home-overview-head h3 {
  margin-top: 1px;
  color: var(--think-text-primary);
  font-size: 13px;
  line-height: 1.3;
}

.home-metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0;
  margin: 0;
}

.home-metric {
  min-width: 0;
  padding: 9px 10px;
  border-right: 1px solid var(--think-border-soft);
}

.home-metric:last-child {
  border-right: 0;
}

.home-metric span,
.home-metric small,
.home-panel-head span {
  display: block;
  color: var(--think-text-muted);
  font-size: 11px;
}

.home-metric strong {
  display: block;
  margin: 2px 0 1px;
  color: var(--think-text-primary);
  font-size: 20px;
  font-family: Consolas, "SFMono-Regular", monospace;
  line-height: 1.1;
}

.home-content-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.45fr) minmax(260px, 0.55fr);
  gap: 8px;
  margin-top: 8px;
}

.home-panel {
  padding: 9px;
}

.home-panel-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 7px;
}

.home-panel-head strong {
  color: var(--think-text-primary);
  font-size: 14px;
  font-weight: 780;
}

.recent-doc-list {
  display: grid;
  gap: 4px;
}

.recent-doc-list button {
  width: 100%;
  min-height: 44px;
  padding: 7px 9px;
  border: 1px solid var(--think-border-soft);
  border-radius: 8px;
  background: #fbfdff;
  text-align: left;
  cursor: pointer;
}

.recent-doc-list button:hover {
  border-color: #b9d4ff;
  background: var(--think-brand-soft);
}

.recent-doc-list strong,
.recent-doc-list span {
  display: block;
}

.recent-doc-list strong {
  overflow: hidden;
  color: var(--think-text-primary);
  font-size: 13px;
  font-weight: 750;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.recent-doc-list span {
  margin-top: 3px;
  color: var(--think-text-muted);
  font-size: 11px;
}

.home-empty-note {
  min-height: 74px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px dashed rgba(76, 139, 245, 0.34);
  border-radius: 8px;
  color: var(--think-text-muted);
  font-size: 12px;
  background: #fbfdff;
}

.scope-list {
  display: grid;
  border: 1px solid var(--think-border-soft);
  border-radius: 8px;
  background: #fbfdff;
}

.scope-list > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 32px;
  padding: 0 9px;
  border-bottom: 1px solid var(--think-border-soft);
}

.scope-list > div:last-child {
  border-bottom: 0;
}

.scope-list span,
.scope-list strong {
  overflow: hidden;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.scope-list span {
  color: var(--think-text-muted);
}

.scope-list strong {
  color: var(--think-text-primary);
  font-weight: 650;
}

.search-workbench {
  max-width: 920px;
  display: grid;
  gap: 12px;
  padding: 12px;
  border: 1px solid var(--think-border-cool);
  border-radius: 8px;
  background: #fff;
}

.search-status-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--think-border-soft);
}

.search-status-head strong,
.search-status-head span {
  display: block;
}

.search-status-head strong {
  color: var(--think-text-primary);
  font-size: 14px;
  font-weight: 750;
}

.search-status-head span {
  max-width: 680px;
  margin-top: 3px;
  color: var(--think-text-muted);
  font-size: 12px;
  line-height: 1.6;
}

.search-status-head :deep(.el-tag) {
  flex: 0 0 auto;
}

.search-facts {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin: 0;
  overflow: hidden;
  border: 1px solid var(--think-border-soft);
  border-radius: 8px;
  background: #f8fbff;
}

.search-facts > div {
  min-width: 0;
  padding: 10px;
  border-right: 1px solid var(--think-border-soft);
}

.search-facts > div:last-child {
  border-right: 0;
}

.search-facts dt,
.search-facts dd {
  margin: 0;
}

.search-facts dt {
  color: var(--think-text-muted);
  font-size: 11px;
}

.search-facts dd {
  margin-top: 4px;
  overflow: hidden;
  color: var(--think-text-primary);
  font-size: 14px;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.search-actions {
  display: flex;
  justify-content: flex-end;
}

.detail-tabs {
  min-height: 0;
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  padding: 8px 10px 10px;
  overflow: hidden;
}

.detail-tabs :deep(.el-tabs__header) {
  display: none;
}

.detail-tabs :deep(.el-tabs__content) {
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}

.detail-tabs :deep(.el-tab-pane) {
  height: 100%;
  min-height: 0;
  overflow: auto;
  padding-right: 4px;
}

.detail-tabs :deep(.preview-tab-pane) {
  overflow: hidden;
  padding-right: 0;
}

.detail-tabs :deep(.edit-tab-pane) {
  overflow: hidden;
  padding-right: 0;
}

.detail-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background-color: var(--think-border-soft);
}

.preview-layout,
.summary-workbench {
  display: grid;
  gap: 8px;
}

.preview-layout {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.preview-layout .ai-collapse {
  flex: 0 0 auto;
}

.markdown-section,
.summary-action-panel,
.summary-result-block {
  border: 1px solid var(--think-border-cool);
  border-radius: 8px;
  background: #fff;
  padding: 9px;
}

.markdown-section {
  min-height: 0;
  flex: 1 1 auto;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 20px clamp(18px, 3vw, 44px);
  border: 0;
  border-radius: 0;
}

.ai-blocks {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px;
}

.ai-collapse {
  --el-collapse-border-color: transparent;
  border: 0;
}

.ai-collapse :deep(.el-collapse-item) {
  overflow: hidden;
  border: 1px solid var(--think-border-cool);
  border-radius: 8px;
  background: #fff;
}

.ai-collapse :deep(.el-collapse-item__header) {
  height: 40px;
  padding: 0 12px;
  border-bottom-color: rgba(223, 231, 242, 0.7);
  background: transparent;
}

.ai-collapse :deep(.el-collapse-item__content) {
  padding: 8px;
}

.collapse-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--think-text-primary);
  font-size: 13px;
  font-weight: 750;
}

.collapse-title small {
  color: var(--think-text-muted);
  font-size: 11px;
  font-weight: 600;
}

.ai-block {
  padding: 8px 9px;
  border-radius: 8px;
  border: 1px solid var(--think-border-soft);
  background: #f8fbff;
}

.ai-block span {
  display: block;
  color: var(--think-text-primary);
  font-size: 13px;
  font-weight: 750;
}

.ai-block ul {
  display: grid;
  gap: 6px;
  margin: 8px 0 0;
  padding-left: 18px;
  color: var(--think-text-secondary);
  font-size: 12px;
  line-height: 1.6;
}

.markdown-preview {
  color: var(--think-text-secondary);
  font-size: 13px;
  line-height: 1.65;
}

.preview-layout .markdown-preview {
  min-height: 0;
  flex: 1 1 auto;
  overflow: auto;
  padding-right: 4px;
}

.markdown-preview.compact {
  max-height: 560px;
  overflow: auto;
  padding-right: 4px;
}

.markdown-preview :deep(h1),
.markdown-preview :deep(h2),
.markdown-preview :deep(h3),
.markdown-preview :deep(h4) {
  margin: 16px 0 8px;
  color: var(--think-text-primary);
  font-weight: 750;
  line-height: 1.35;
}

.markdown-preview :deep(h1) {
  margin-top: 0;
  font-size: 24px;
}

.markdown-preview :deep(h2) {
  font-size: 18px;
}

.markdown-preview :deep(h3) {
  font-size: 16px;
}

.markdown-preview :deep(h4) {
  font-size: 14px;
}

.markdown-preview :deep(p) {
  margin: 7px 0;
}

.markdown-preview :deep(a) {
  color: var(--think-brand-active);
  text-decoration: none;
}

.markdown-preview :deep(a:hover) {
  text-decoration: underline;
}

.markdown-preview :deep(img) {
  display: block;
  max-width: 100%;
  margin: 10px 0;
  border-radius: 6px;
  border: 1px solid rgba(223, 231, 242, 0.86);
}

.markdown-preview :deep(del) {
  color: var(--think-text-muted);
}

.markdown-preview :deep(hr) {
  height: 1px;
  margin: 14px 0;
  border: 0;
  background: rgba(223, 231, 242, 0.92);
}

.markdown-preview :deep(ul),
.markdown-preview :deep(ol) {
  margin: 8px 0;
  padding-left: 22px;
}

.markdown-preview :deep(li) {
  margin: 5px 0;
}

.markdown-preview :deep(.task-list) {
  padding-left: 0;
  list-style: none;
}

.markdown-preview :deep(.task-list-item) {
  display: flex;
  align-items: center;
  gap: 8px;
}

.markdown-preview :deep(.task-list-item input) {
  width: 14px;
  height: 14px;
  cursor: not-allowed;
}

.markdown-preview :deep(.task-list-item.is-task-complete) {
  opacity: 0.62;
  text-decoration: line-through;
}

.markdown-preview :deep(table) {
  width: auto;
  min-width: min(480px, 100%);
  max-width: 100%;
  overflow: hidden;
  margin: 18px 0 26px;
  border-collapse: separate;
  border-spacing: 0;
  table-layout: auto;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 6px;
  font-size: 12px;
}

.markdown-preview :deep(th),
.markdown-preview :deep(td) {
  min-width: 80px;
  max-width: 280px;
  padding: 9px 12px;
  border-right: 1px solid rgba(223, 231, 242, 0.74);
  border-bottom: 1px solid rgba(223, 231, 242, 0.74);
  line-height: 1.55;
  overflow-wrap: anywhere;
  vertical-align: top;
}

.markdown-preview :deep(th) {
  color: var(--think-text-primary);
  background: rgba(247, 250, 255, 0.9);
  font-weight: 750;
}

.markdown-preview :deep(th:last-child),
.markdown-preview :deep(td:last-child) {
  border-right: 0;
}

.markdown-preview :deep(tbody tr:last-child td) {
  border-bottom: 0;
}

.markdown-preview :deep(code) {
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(76, 139, 245, 0.08);
  color: var(--think-brand-active);
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
}

.markdown-preview :deep(pre) {
  box-sizing: border-box;
  width: fit-content;
  max-width: 100%;
  overflow: auto;
  margin: 10px 0;
  padding: 12px;
  border-radius: 6px;
  border: 1px solid rgba(35, 48, 70, 0.12);
  background: #101827;
}

.markdown-preview :deep(pre code) {
  display: block;
  padding: 0;
  border-radius: 0;
  background: transparent;
  color: #edf4ff;
  white-space: pre;
}

.markdown-preview :deep(blockquote) {
  margin: 10px 0;
  padding: 8px 12px;
  border-left: 3px solid rgba(76, 139, 245, 0.38);
  border-radius: 6px;
  background: rgba(76, 139, 245, 0.06);
}

.markdown-preview :deep(.text-color-mark[data-background-color]) {
  padding: 1px 2px;
  border-radius: 2px;
  box-decoration-break: clone;
  -webkit-box-decoration-break: clone;
}

.markdown-preview :deep(.highlight-block) {
  position: relative;
  box-sizing: border-box;
  width: min(860px, 100%);
  min-height: 48px;
  margin: 12px 0;
  padding: 12px 16px 12px 50px;
  border: 1px solid rgba(245, 158, 11, 0.46);
  border-radius: 6px;
  background: #fff7ed;
  box-shadow: inset 3px 0 0 #f59e0b;
  color: #475569;
  line-height: 1.7;
  overflow-wrap: anywhere;
}

.markdown-preview :deep(.highlight-block)::before {
  position: absolute;
  top: 12px;
  left: 14px;
  width: 24px;
  height: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  content: attr(data-highlight-icon);
  font-size: 18px;
  line-height: 24px;
  text-align: center;
}

.markdown-preview :deep(.highlight-block li::marker) {
  color: #2563eb;
  font-weight: 700;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 14px;
}

.span-2 {
  grid-column: span 2;
}

.span-3 {
  grid-column: 1 / -1;
}

.readonly-ai-field :deep(.el-select__wrapper) {
  background: rgba(241, 245, 249, 0.78);
  border-color: rgba(214, 223, 235, 0.82);
  cursor: not-allowed;
}

.readonly-ai-field :deep(.el-select__placeholder),
.readonly-ai-field :deep(.el-select__selected-item) {
  color: var(--think-text-muted);
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.summary-action-panel {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 220px auto;
  gap: 10px;
  align-items: center;
  padding: 10px 12px;
}

.summary-action-panel strong,
.summary-action-panel span,
.summary-result-block span,
.summary-result-block p {
  display: block;
}

.summary-action-panel strong,
.summary-result-block span {
  color: var(--think-text-primary);
  font-size: 13px;
  font-weight: 750;
}

.summary-action-panel span {
  margin-top: 2px;
  color: var(--think-text-muted);
  font-size: 11px;
  line-height: 1.4;
}

.provider-select {
  width: 100%;
}

.summary-result-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.compact-summary-grid .summary-result-block {
  min-height: 0;
  padding: 8px 10px;
  border-radius: 6px;
}

.summary-result-block p {
  margin: 6px 0 0;
  color: var(--think-text-secondary);
  font-size: 12px;
  line-height: 1.55;
}

.summary-compact-main {
  grid-column: span 1;
}

.line-list {
  display: grid;
  gap: 2px;
  margin-top: 6px;
}

.line-list p {
  position: relative;
  margin: 0;
  padding: 2px 0 2px 12px;
  border-radius: 0;
  background: transparent;
  color: var(--think-text-secondary);
  font-size: 12px;
  line-height: 1.45;
  white-space: normal;
}

.line-list p::before {
  content: '';
  position: absolute;
  top: 10px;
  left: 2px;
  width: 4px;
  height: 4px;
  border-radius: 999px;
  background: rgba(76, 139, 245, 0.42);
}

.summary-block-resolutionSteps .line-list {
  counter-reset: resolution-step;
}

.summary-block-resolutionSteps .line-list p {
  counter-increment: resolution-step;
  padding-left: 22px;
}

.summary-block-resolutionSteps .line-list p::before {
  content: counter(resolution-step);
  top: 2px;
  left: 0;
  width: 16px;
  height: 16px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: rgba(76, 139, 245, 0.1);
  color: var(--think-brand-active);
  font-size: 10px;
  font-weight: 750;
}

.keyword-list {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}

.keyword-list p {
  padding: 2px 8px;
  border-radius: 999px;
  background: rgba(15, 23, 42, 0.045);
  color: var(--think-text-muted);
  line-height: 1.4;
}

.keyword-list p::before {
  display: none;
}

.import-dialog-body {
  display: grid;
  gap: 16px;
}

.import-mode-segment {
  justify-self: start;
}

.paste-import-panel {
  overflow: hidden;
  border: 1px solid var(--think-border-cool);
  border-radius: 8px;
  background: #fff;
}

.paste-import-panel :deep(.el-textarea__inner) {
  min-height: 240px !important;
  border: 0;
  border-radius: 0;
  font-family: Consolas, "Courier New", monospace;
  font-size: 13px;
  line-height: 1.65;
}

.import-form {
  padding: 8px;
  border-radius: 8px;
  border: 1px solid var(--think-border-cool);
  background: #fff;
}

.import-compact-form .form-grid {
  grid-template-columns: repeat(2, minmax(260px, 1fr));
  gap: 8px 14px;
}

.import-compact-form :deep(.el-form-item) {
  align-items: center;
  margin-bottom: 0;
}

.import-compact-form :deep(.el-form-item__label) {
  width: 72px;
  min-width: 72px;
  height: 34px;
  justify-content: flex-start;
  padding: 0 8px 0 0;
  color: var(--think-text-secondary);
  font-size: 12px;
  line-height: 34px;
}

.import-compact-form :deep(.el-form-item__content) {
  min-width: 0;
}

.import-compact-form :deep(.el-input__wrapper),
.import-compact-form :deep(.el-select__wrapper) {
  min-height: 34px;
  border-radius: 8px;
}

.thinktank-page :deep(.el-button) {
  min-height: 26px;
  padding: 0 9px;
  border-radius: 6px;
  font-weight: 650;
  border-color: #d3e0ee;
  background: #fff;
  box-shadow: none;
  transition: border-color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease;
}

.thinktank-page :deep(.el-button:hover) {
  border-color: #a9c8ed;
  background: #f8fbff;
}

.thinktank-page :deep(.el-button + .el-button) {
  margin-left: 0;
}

.thinktank-page :deep(.el-button:active:not(.is-disabled)) {
  transform: translateY(1px);
}

.thinktank-page :deep(.el-button--primary:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: #409eff;
  background: #409eff;
  color: #fff;
}

.thinktank-page :deep(.el-button--success:not(.is-link):not(.is-text):not(.is-plain)) {
  color: #fff;
  border-color: #10b981;
  background: #10b981;
}

.thinktank-page :deep(.el-button--warning:not(.is-link):not(.is-text):not(.is-plain)) {
  color: #fff;
  border-color: #f59e0b;
  background: #f59e0b;
}

.thinktank-page :deep(.el-button--danger:not(.is-link):not(.is-text):not(.is-plain)) {
  color: #fff;
  border-color: #ef4444;
  background: #ef4444;
}

.thinktank-page :deep(.el-button.is-disabled),
.thinktank-page :deep(.el-button.is-loading) {
  opacity: 0.58;
}

.thinktank-page :deep(.el-button--primary.is-plain) {
  color: var(--think-brand-active);
  border-color: rgba(171, 189, 220, 0.96);
  background: rgba(255, 255, 255, 0.92);
}

.thinktank-page :deep(.el-input__wrapper),
.thinktank-page :deep(.el-select__wrapper),
.thinktank-page :deep(.el-textarea__inner) {
  border-radius: 8px;
  background: #fff;
  border: 1px solid #d3e0ee;
  box-shadow: none !important;
}

.thinktank-page :deep(.el-input__wrapper:hover),
.thinktank-page :deep(.el-select__wrapper:hover),
.thinktank-page :deep(.el-textarea__inner:hover) {
  border-color: #a9c8ed;
  box-shadow: none !important;
}

.thinktank-page :deep(.el-input__wrapper.is-focus),
.thinktank-page :deep(.el-select__wrapper.is-focused),
.thinktank-page :deep(.el-textarea__inner:focus) {
  background: #ffffff;
  border-color: #409eff;
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07) !important;
}

.thinktank-page :deep(.el-form-item__label) {
  color: var(--think-text-primary);
  font-size: 13px;
  font-weight: 650;
}

.thinktank-page :deep(.el-tag) {
  border-radius: 999px;
  border-color: transparent;
  font-weight: 650;
}

.thinktank-page :deep(.el-tabs__header) {
  margin-bottom: 6px;
}

.thinktank-page :deep(.el-tabs__item) {
  height: 36px;
  color: var(--think-text-muted);
  font-size: 12px;
  font-weight: 650;
}

.thinktank-page :deep(.el-tabs__item.is-active) {
  color: var(--think-brand-active);
}

.thinktank-page :deep(.el-input__wrapper),
.thinktank-page :deep(.el-select__wrapper) {
  min-height: 28px;
}

.thinktank-page .knowledge-toolbar :deep(.el-input__wrapper),
.thinktank-page .knowledge-toolbar :deep(.el-button),
.thinktank-page .header-commands :deep(.el-button),
.thinktank-page .detail-actions :deep(.el-button) {
  min-height: 32px;
}

.tree-head-actions :deep(.el-button) {
  min-height: 24px;
}

.tree-head-actions :deep(.tree-add-btn) {
  border-color: #10b981;
  background: #10b981;
  color: #fff;
}

.tree-head-actions :deep(.tree-add-btn:hover),
.tree-head-actions :deep(.tree-add-btn:focus-visible) {
  border-color: #059669;
  background: #059669;
  color: #fff;
}

.tree-head-actions :deep(.tree-refresh-btn) {
  border-color: #409eff;
  background: #409eff;
  color: #fff;
}

.tree-head-actions :deep(.tree-refresh-btn:hover),
.tree-head-actions :deep(.tree-refresh-btn:focus-visible) {
  border-color: #2563eb;
  background: #2563eb;
  color: #fff;
}

:global(.knowledge-dialog.el-dialog) {
  border-radius: 15px;
  overflow: hidden;
  border: 1px solid #dbe7f4;
  background: #fff;
  box-shadow: 0 22px 70px rgba(15, 23, 42, 0.14);
}

:global(.knowledge-dialog .el-dialog__header) {
  margin-right: 0;
  padding: 12px 14px 10px;
  background: linear-gradient(90deg, #fff6e7 0%, #f7fbff 36%, #ecf7ff 100%);
  border-bottom: 1px solid #e7eef7;
}

:global(.knowledge-dialog .el-dialog__title) {
  color: #172033;
  font-size: 16px;
  font-weight: 750;
}

.thinktank-page :deep(.el-upload-dragger) {
  border-radius: 8px;
  border-color: rgba(171, 189, 220, 0.86);
  background: rgba(247, 250, 255, 0.74);
}

.thinktank-page :deep(.el-upload-dragger:hover) {
  border-color: rgba(76, 139, 245, 0.48);
}

@media (max-width: 1440px) {
  .workspace {
    grid-template-columns: 220px 224px minmax(420px, 1fr);
  }

  .workspace.outline-collapsed {
    grid-template-columns: 220px 42px minmax(420px, 1fr);
  }

  .span-3 {
    grid-column: 1 / -1;
  }

  .summary-action-panel {
    grid-template-columns: 1fr;
  }

  .summary-result-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .home-content-grid {
    grid-template-columns: minmax(0, 1.35fr) minmax(240px, 0.65fr);
  }
}

@media (max-width: 1180px) {
  .workspace {
    grid-template-columns: 196px 210px minmax(0, 1fr);
  }

  .workspace.outline-collapsed {
    grid-template-columns: 196px 42px minmax(0, 1fr);
  }

  .detail-panel {
    min-height: 0;
  }

  .detail-title {
    flex-basis: 180px;
  }

  .detail-title p {
    display: none;
  }

  .detail-head-main {
    gap: 6px;
  }

  .detail-tab-nav button {
    min-width: auto;
    padding: 0 6px;
  }
}

@media (max-width: 820px) {
  :global(.home-container .el-main:has(.thinktank-page)) {
    overflow-y: auto;
  }

  .thinktank-page {
    height: auto;
    max-height: none;
    min-height: calc(100vh - var(--think-page-height-offset));
    padding: 6px;
    overflow: visible;
  }

  .thinktank-card {
    height: auto;
    min-height: 0;
  }

  .page-header {
    align-items: stretch;
    flex-direction: column;
  }

  .header-brand {
    align-items: flex-start;
  }

  .header-title {
    align-items: flex-start;
    flex-direction: column;
    gap: 1px;
  }

  .header-commands {
    width: 100%;
  }

  .header-commands :deep(.el-button) {
    flex: 1 1 calc(50% - 3px);
  }

  .detail-head {
    align-items: stretch;
    flex-direction: column;
  }

  .detail-head-main {
    align-items: stretch;
    flex-direction: column;
    gap: 4px;
  }

  .detail-title {
    flex-basis: auto;
  }

  .detail-tab-nav {
    height: 36px;
    overflow-x: auto;
  }

  .detail-actions {
    justify-content: flex-start;
  }

  .detail-actions :deep(.el-button) {
    width: 100%;
  }

  .workspace,
  .workspace.outline-collapsed,
  .ai-blocks,
  .summary-result-grid,
  .home-content-grid,
  .form-grid {
    grid-template-columns: 1fr;
  }

  .home-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .search-status-head {
    align-items: stretch;
    flex-direction: column;
  }

  .search-status-head :deep(.el-tag) {
    align-self: flex-start;
  }

  .search-facts {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .search-facts > div:nth-child(2) {
    border-right: 0;
  }

  .search-facts > div:nth-child(-n + 2) {
    border-bottom: 1px solid var(--think-border-soft);
  }

  .tree-head-actions :deep(.el-button),
  .tree-node-more {
    width: 40px;
    height: 40px;
    flex-basis: 40px;
  }

  .knowledge-home {
    padding: 8px;
  }

  .home-overview-head {
    align-items: stretch;
    flex-direction: column;
  }

  .home-metric:nth-child(2) {
    border-right: 0;
  }

  .home-metric:nth-child(odd) {
    border-right: 1px solid var(--think-border-soft);
  }

  .home-metric:nth-child(-n + 2) {
    border-bottom: 1px solid var(--think-border-soft);
  }

  .span-2 {
    grid-column: span 1;
  }

  .span-3 {
    grid-column: span 1;
  }

  .import-compact-form .form-grid {
    grid-template-columns: 1fr;
  }

  .import-compact-form :deep(.el-form-item) {
    align-items: stretch;
    flex-direction: column;
  }

  .import-compact-form :deep(.el-form-item__label) {
    width: auto;
    min-width: 0;
    height: auto;
    padding: 0 0 6px;
    line-height: 1.3;
  }

  .tree-panel,
  .document-panel,
  .detail-panel {
    min-height: 460px;
  }

  .document-panel.collapsed {
    min-height: 42px;
  }

  .tree-panel,
  .document-panel {
    border-right: 0;
    border-bottom: 1px solid var(--think-border-mid);
  }

  .search-actions {
    justify-content: flex-start;
  }
}
</style>
