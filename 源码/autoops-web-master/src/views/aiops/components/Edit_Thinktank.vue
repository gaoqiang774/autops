<template>
  <div class="thinktank-editor">
    <el-form
      ref="formRef"
      :model="formModel"
      :rules="formRules"
      label-position="left"
      class="document-form"
      @submit.prevent
    >
      <div class="editor-grid">
        <div class="markdown-editor-shell">
          <div class="markdown-toolbar" aria-label="Markdown 语法工具">
            <div class="markdown-tool-group document-info-tool-group">
              <el-popover
                v-model:visible="metadataVisibleModel"
                trigger="click"
                placement="bottom-start"
                :width="680"
                popper-class="document-meta-popover"
              >
                <template #reference>
                  <button
                    type="button"
                    class="markdown-tool document-info-trigger"
                    :class="{ active: metadataVisibleModel }"
                    title="编辑标题、类型、分组和标签"
                    @mousedown.prevent="rememberEditorSelection"
                  >
                    <el-icon><Setting /></el-icon>
                    文档信息
                  </button>
                </template>
                <div class="document-meta-panel">
                  <div class="document-meta-panel-head">
                    <strong>文档信息</strong>
                    <span>标题、类型、知识分组和标签</span>
                  </div>
                  <div class="form-grid document-meta-form">
                    <el-form-item label="标题" prop="title">
                      <el-input v-model.trim="titleModel" clearable placeholder="请输入文档标题" />
                    </el-form-item>
                    <el-form-item label="文档类型" prop="docType">
                      <el-select v-model="docTypeModel" placeholder="请选择" style="width: 100%">
                        <el-option v-for="item in docTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
                      </el-select>
                    </el-form-item>
                    <el-form-item label="知识分组" prop="groupId">
                      <el-tree-select
                        v-model="groupIdModel"
                        :data="groupSelectData"
                        check-strictly
                        default-expand-all
                        clearable
                        placeholder="选择知识分组"
                        style="width: 100%"
                      />
                    </el-form-item>
                    <el-form-item label="标签">
                      <el-select
                        v-model="tagsModel"
                        multiple
                        collapse-tags
                        collapse-tags-tooltip
                        :max-collapse-tags="1"
                        filterable
                        allow-create
                        default-first-option
                        placeholder="输入后回车添加"
                        style="width: 100%"
                      />
                    </el-form-item>
                  </div>
                </div>
              </el-popover>
            </div>

            <div v-for="group in markdownToolGroups" :key="group.label" class="markdown-tool-group">
              <template v-for="tool in group.tools" :key="tool.key">
                <el-popover
                  v-if="tool.key === 'table'"
                  v-model:visible="tablePickerVisible"
                  trigger="click"
                  placement="bottom-start"
                  :width="254"
                  popper-class="markdown-table-popover"
                  @hide="resetTablePickerHover"
                >
                  <template #reference>
                    <button
                      type="button"
                      class="markdown-tool"
                      :title="tool.title"
                      @mousedown.prevent="rememberEditorSelection"
                    >
                      {{ tool.text }}
                    </button>
                  </template>
                  <div class="table-picker" @mousedown.prevent>
                    <div class="table-picker-head">
                      <span>插入 Markdown 表格</span>
                      <strong>{{ tablePickerHover.cols }} x {{ tablePickerHover.rows }}</strong>
                    </div>
                    <div class="table-picker-grid" @mouseleave="resetTablePickerHover">
                      <template v-for="row in tablePickerMaxRows" :key="row">
                        <button
                          v-for="col in tablePickerMaxCols"
                          :key="`${row}-${col}`"
                          type="button"
                          class="table-picker-cell"
                          :class="{ active: row <= tablePickerHover.rows && col <= tablePickerHover.cols }"
                          @mouseenter="setTablePickerHover(row, col)"
                          @click="insertMarkdownTable(row, col)"
                        />
                      </template>
                    </div>
                  </div>
                </el-popover>
                <el-popover
                  v-else-if="tool.key === 'color'"
                  v-model:visible="colorPickerVisible"
                  trigger="click"
                  placement="bottom-start"
                  :width="272"
                  popper-class="markdown-color-popover"
                >
                  <template #reference>
                    <button
                      type="button"
                      class="markdown-tool rich-color-trigger"
                      :class="{ active: colorPickerVisible }"
                      :title="tool.title"
                      aria-label="字体和背景颜色"
                      @mousedown.prevent="rememberEditorSelection"
                    >
                      <span
                        class="rich-color-trigger-letter"
                        :style="{ color: activeTextColor || undefined, backgroundColor: activeBackgroundColor || undefined }"
                      >A</span>
                      <el-icon><ArrowDown /></el-icon>
                    </button>
                  </template>
                  <div class="rich-color-picker" @mousedown.prevent>
                    <section class="rich-color-section">
                      <strong>字体颜色</strong>
                      <div class="rich-color-grid" role="group" aria-label="字体颜色">
                        <button
                          v-for="color in textColorPalette"
                          :key="`text-${color.value}`"
                          type="button"
                          class="rich-color-swatch text-swatch"
                          :class="{ active: activeTextColor === color.value }"
                          :title="color.label"
                          :aria-label="`字体颜色：${color.label}`"
                          :aria-pressed="activeTextColor === color.value"
                          @click="applySelectionColor('text', color.value)"
                        >
                          <span :style="{ color: color.value }">A</span>
                        </button>
                      </div>
                    </section>
                    <section class="rich-color-section">
                      <strong>背景颜色</strong>
                      <div class="rich-color-grid background-grid" role="group" aria-label="背景颜色">
                        <button
                          v-for="color in backgroundColorPalette"
                          :key="`background-${color.value}`"
                          type="button"
                          class="rich-color-swatch background-swatch"
                          :class="{ active: activeBackgroundColor === color.value }"
                          :title="color.label"
                          :aria-label="`背景颜色：${color.label}`"
                          :aria-pressed="activeBackgroundColor === color.value"
                          @click="applySelectionColor('background', color.value)"
                        >
                          <span :style="{ backgroundColor: color.value }"></span>
                        </button>
                      </div>
                    </section>
                    <button type="button" class="rich-color-reset" @click="resetSelectionColors">
                      恢复默认
                    </button>
                  </div>
                </el-popover>
                <button
                  v-else
                  type="button"
                  class="markdown-tool"
                  :title="tool.title"
                  @mousedown.prevent="rememberEditorSelection"
                  @click="insertMarkdownSnippet(tool.key)"
                >
                  {{ tool.text }}
                </button>
              </template>
            </div>
          </div>

          <el-form-item prop="contentMarkdown" class="editor-form-item">
            <div
              ref="richEditorRef"
              class="rich-editor markdown-preview"
              contenteditable="true"
              data-placeholder="输入正文，工具栏会实时渲染标题、列表、表格和代码块"
              @input="handleRichEditorInput"
              @change="handleRichEditorChange"
              @focus="handleRichEditorFocus"
              @blur="handleRichEditorBlur"
              @click="handleRichEditorClick"
              @keydown="handleRichEditorKeydown"
              @keyup="handleRichEditorKeyup"
              @mouseup="handleRichEditorMouseup"
              @pointerdown="handleRichEditorPointerDown"
              @pointermove="handleRichEditorPointerMove"
              @mouseleave="handleRichEditorPointerLeave"
              @scroll.passive="handleRichEditorScroll"
              @paste="handleRichEditorPaste"
            ></div>
            <div
              ref="tableEdgeControlsRef"
              class="table-edge-controls"
              :class="{ visible: tableEdgeControlsVisible, resizing: tableColumnResizing }"
              aria-hidden="false"
            >
              <button
                v-for="handle in tableColumnHandles"
                :key="`column-${handle.index}`"
                type="button"
                class="table-boundary-add table-column-add"
                :style="{ left: `${handle.left}px`, top: `${handle.top}px` }"
                :aria-label="`在第 ${handle.index + 1} 列位置插入一列`"
                title="插入一列"
                @mousedown.prevent.stop
                @click.stop="insertTableColumnAt(handle.index)"
              >
                <span class="table-boundary-dot"></span>
                <el-icon><Plus /></el-icon>
              </button>
              <button
                v-for="handle in tableRowHandles"
                :key="`row-${handle.index}`"
                type="button"
                class="table-boundary-add table-row-add"
                :style="{ left: `${handle.left}px`, top: `${handle.top}px` }"
                :aria-label="`在第 ${handle.index} 行下方插入一行`"
                title="向下插入一行"
                @mousedown.prevent.stop
                @click.stop="insertTableRowAt(handle.index)"
              >
                <span class="table-boundary-dot"></span>
                <el-icon><Plus /></el-icon>
              </button>
              <button
                v-for="handle in tableColumnSelectionHandles"
                :key="`select-column-${handle.index}`"
                type="button"
                class="table-selection-trigger table-column-selection-trigger"
                :style="{ left: `${handle.left}px`, top: `${handle.top}px` }"
                :aria-label="`选择第 ${handle.index + 1} 列`"
                :title="`选择第 ${handle.index + 1} 列`"
                @mousedown.prevent.stop
                @click.stop="selectTableColumn(handle.index)"
              >
                <span></span>
              </button>
              <button
                v-for="handle in tableRowSelectionHandles"
                :key="`select-row-${handle.index}`"
                type="button"
                class="table-selection-trigger table-row-selection-trigger"
                :style="{ left: `${handle.left}px`, top: `${handle.top}px` }"
                :aria-label="`选择第 ${handle.index} 行`"
                :title="`选择第 ${handle.index} 行`"
                @mousedown.prevent.stop
                @click.stop="selectTableRow(handle.tableRowIndex)"
              >
                <span></span>
              </button>
              <div
                v-for="handle in tableColumnResizeHandles"
                :key="`resize-${handle.index}`"
                class="table-column-resize-handle"
                :class="{ active: tableResizeColumnIndex === handle.index }"
                :style="{
                  left: `${handle.left}px`,
                  top: `${handle.resizeTop}px`,
                  height: `${handle.height}px`
                }"
                role="separator"
                tabindex="0"
                aria-orientation="vertical"
                :aria-label="`拖动调整第 ${handle.index} 列宽度`"
                title="拖动调整列宽"
                @pointerdown.prevent.stop="startTableColumnResize($event, handle.index)"
                @keydown.left.prevent="resizeTableColumnByKeyboard(handle.index, 'ArrowLeft')"
                @keydown.right.prevent="resizeTableColumnByKeyboard(handle.index, 'ArrowRight')"
              ></div>
              <button
                v-if="tableDeleteControl"
                type="button"
                class="table-selection-delete"
                :class="`is-${tableDeleteControl.type}`"
                :style="{ left: `${tableDeleteControl.left}px`, top: `${tableDeleteControl.top}px` }"
                :disabled="tableDeleteControl.disabled"
                :aria-label="tableDeleteControl.label"
                :title="tableDeleteControl.label"
                @mousedown.prevent.stop
                @click.stop="deleteSelectedTablePart"
              >
                <el-icon><Delete /></el-icon>
              </button>
            </div>
            <div ref="blockInsertControlsRef" class="block-insert-controls">
              <button
                v-if="blockInsertControl"
                type="button"
                class="block-insert-trigger"
                :class="{ active: blockStyleMenuVisible }"
                :style="{ left: `${blockInsertControl.left}px`, top: `${blockInsertControl.top}px` }"
                aria-label="插入内容块"
                :aria-expanded="blockStyleMenuVisible"
                title="插入内容块"
                @mousedown.prevent.stop="rememberEditorSelection"
                @click.stop="toggleBlockStyleMenu"
              >
                <el-icon><Plus /></el-icon>
              </button>
              <div
                v-if="blockInsertControl && blockStyleMenuVisible"
                class="block-style-menu"
                :style="{ left: `${blockInsertControl.menuLeft}px`, top: `${blockInsertControl.menuTop}px` }"
                role="menu"
                aria-label="内容样式"
                @mousedown.prevent.stop
              >
                <div v-for="group in blockStyleMenuGroups" :key="group.label" class="block-style-menu-group">
                  <div class="block-style-menu-label">{{ group.label }}</div>
                  <div class="block-style-menu-items">
                    <button
                      v-for="tool in group.tools"
                      :key="tool.key"
                      type="button"
                      class="block-style-menu-item"
                      role="menuitem"
                      :title="tool.title"
                      @click.stop="applyBlockStyle(tool.key)"
                    >
                      <span class="block-style-menu-key">{{ tool.text }}</span>
                      <span>{{ tool.menuLabel }}</span>
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </el-form-item>
        </div>
      </div>
    </el-form>
  </div>
</template>

<script setup>
/* global defineProps, defineEmits, defineExpose */
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ArrowDown, Delete, Plus, Setting } from '@element-plus/icons-vue'
import {
  escapeHtml,
  getRandomHighlightBlockIcon,
  htmlToMarkdown,
  readPersistedTableWidths,
  renderMarkdown
} from './thinktankMarkdown'

const props = defineProps({
  title: { type: String, default: '' },
  docType: { type: String, default: '' },
  groupId: { type: [String, Number], default: '' },
  tags: { type: Array, default: () => [] },
  contentMarkdown: { type: String, default: '' },
  metadataVisible: { type: Boolean, default: false },
  docTypeOptions: { type: Array, default: () => [] },
  groupSelectData: { type: Array, default: () => [] }
})

const emit = defineEmits([
  'update:title',
  'update:docType',
  'update:groupId',
  'update:tags',
  'update:contentMarkdown',
  'update:metadataVisible'
])

const createModel = (propName, eventName) => computed({
  get: () => props[propName],
  set: value => emit(eventName, value)
})

const titleModel = createModel('title', 'update:title')
const docTypeModel = createModel('docType', 'update:docType')
const groupIdModel = createModel('groupId', 'update:groupId')
const tagsModel = createModel('tags', 'update:tags')
const metadataVisibleModel = createModel('metadataVisible', 'update:metadataVisible')
const formModel = computed(() => ({
  title: props.title,
  docType: props.docType,
  groupId: props.groupId,
  contentMarkdown: props.contentMarkdown
}))

const formRules = {
  title: [{ required: true, message: '请输入标题', trigger: 'blur' }],
  docType: [{ required: true, message: '请选择文档类型', trigger: 'change' }],
  groupId: [{ required: true, message: '请选择知识分组', trigger: 'change' }],
  contentMarkdown: [{ required: true, message: '请输入正文', trigger: 'blur' }]
}

const markdownToolGroups = [
  {
    label: '标题',
    tools: [
      { key: 'h1', text: 'H1', title: '# 一级标题' },
      { key: 'h2', text: 'H2', title: '## 二级标题' },
      { key: 'h3', text: 'H3', title: '### 三级标题' }
    ]
  },
  {
    label: '文本',
    tools: [
      { key: 'bold', text: 'B', title: '**粗体文本**' },
      { key: 'italic', text: 'I', title: '*斜体文本*' },
      { key: 'strike', text: 'S', title: '~~删除文本~~' },
      { key: 'inlineCode', text: '<>', title: '`代码内容`' },
      { key: 'quote', text: '“”', title: '> 引用的内容' },
      { key: 'color', text: 'A', title: '字体和背景颜色' }
    ]
  },
  {
    label: '列表',
    tools: [
      { key: 'ul', text: 'UL', title: '- 无序列表' },
      { key: 'ol', text: 'OL', title: '1. 有序列表' },
      { key: 'task', text: 'TODO', title: '- [ ] 任务列表' }
    ]
  },
  {
    label: '插入',
    tools: [
      { key: 'link', text: '链接', title: '[显示文本](URL链接)' },
      { key: 'image', text: '图片', title: '![图片描述](图片URL)' },
      { key: 'codeBlock', text: '代码块', title: '```语言' },
      { key: 'hr', text: '分割线', title: '---' },
      { key: 'table', text: '表格', title: 'Markdown 表格' },
      { key: 'highlightBlock', text: '高亮块', title: '插入高亮块' }
    ]
  }
]

const markdownSnippets = {
  h1: '# 一级标题',
  h2: '## 二级标题',
  h3: '### 三级标题',
  bold: '**粗体文本**',
  italic: '*斜体文本*',
  strike: '~~删除文本~~',
  inlineCode: '`代码内容`',
  quote: '> 引用的内容',
  ul: '- 列表内容',
  ol: '1. 列表内容',
  task: '- [ ] 待办\n- [x] 已完成',
  link: '[显示文本](URL链接)',
  image: '![图片描述](图片URL)',
  codeBlock: '```语言\n代码内容\n```',
  hr: '---',
  table: '| 左对齐 | 居中对齐 |\n| :--- | :---: |\n| 内容A | 内容B |\n| 内容C | 内容D |'
}

const blockStyleMenuLabels = {
  h1: '一级标题',
  h2: '二级标题',
  h3: '三级标题',
  bold: '粗体',
  italic: '斜体',
  strike: '删除线',
  inlineCode: '行内代码',
  quote: '引用',
  color: '字体/背景色',
  ul: '无序列表',
  ol: '有序列表',
  task: '任务列表',
  link: '链接',
  image: '图片',
  codeBlock: '代码块',
  hr: '分割线',
  table: '表格',
  highlightBlock: '高亮块'
}

const blockStyleMenuGroups = markdownToolGroups.map(group => ({
  ...group,
  tools: group.tools.map(tool => ({ ...tool, menuLabel: blockStyleMenuLabels[tool.key] || tool.text }))
}))

const textColorPalette = [
  { label: '深色', value: '#172033' },
  { label: '灰色', value: '#53657d' },
  { label: '红色', value: '#b91c1c' },
  { label: '橙色', value: '#c2410c' },
  { label: '黄色', value: '#a16207' },
  { label: '绿色', value: '#15803d' },
  { label: '蓝色', value: '#1d4ed8' },
  { label: '紫色', value: '#6d28d9' }
]

const backgroundColorPalette = [
  { label: '浅灰', value: '#f1f5f9' },
  { label: '浅红', value: '#fee2e2' },
  { label: '浅橙', value: '#ffedd5' },
  { label: '浅黄', value: '#fef3c7' },
  { label: '浅绿', value: '#dcfce7' },
  { label: '浅蓝', value: '#dbeafe' },
  { label: '浅紫', value: '#ede9fe' },
  { label: '灰蓝', value: '#e2e8f0' },
  { label: '柔红', value: '#fecaca' },
  { label: '柔橙', value: '#fed7aa' },
  { label: '柔黄', value: '#fef08a' },
  { label: '柔绿', value: '#bbf7d0' },
  { label: '柔蓝', value: '#bfdbfe' },
  { label: '柔紫', value: '#ddd6fe' }
]

const formRef = ref(null)
const richEditorRef = ref(null)
const richEditorSelection = ref(null)
const activeTableCell = ref(null)
const tableEdgeControlsRef = ref(null)
const blockInsertControlsRef = ref(null)
const blockInsertControl = ref(null)
const blockStyleMenuVisible = ref(false)
const colorPickerVisible = ref(false)
const activeTextColor = ref('')
const activeBackgroundColor = ref('')
const tableColumnHandles = ref([])
const tableRowHandles = ref([])
const tableColumnSelectionHandles = ref([])
const tableRowSelectionHandles = ref([])
const tableDeleteControl = ref(null)
const tableEdgeControlsVisible = ref(false)
const tableColumnResizing = ref(false)
const tableResizeColumnIndex = ref(-1)
const isSyncingRichEditor = ref(false)
const tablePickerVisible = ref(false)
const tablePickerMaxRows = 8
const tablePickerMaxCols = 8
const tablePickerHover = reactive({ rows: 3, cols: 3 })
const tableColumnResizeHandles = computed(() => tableColumnHandles.value.filter(handle => handle.index > 0))
const activeTableSelectionType = ref('')

let activeTableElement = null
let tableResizeState = null
let tableControlFrame = 0
let blockControlFrame = 0
let lastEmittedMarkdown = ''

const rememberRichEditorSelection = () => {
  const editor = richEditorRef.value
  const selection = window.getSelection?.()
  if (!editor || !selection || !selection.rangeCount) return
  const range = selection.getRangeAt(0)
  if (editor.contains(range.commonAncestorContainer)) richEditorSelection.value = range.cloneRange()
}

const restoreRichEditorSelection = () => {
  const editor = richEditorRef.value
  const selection = window.getSelection?.()
  if (!editor || !selection) return
  editor.focus()
  selection.removeAllRanges()
  if (richEditorSelection.value && editor.contains(richEditorSelection.value.commonAncestorContainer)) {
    selection.addRange(richEditorSelection.value)
    return
  }
  const range = document.createRange()
  range.selectNodeContents(editor)
  range.collapse(false)
  selection.addRange(range)
  richEditorSelection.value = range.cloneRange()
}

const getEditorBlockFromRange = range => {
  const editor = richEditorRef.value
  if (!editor || !range) return null
  let node = range.startContainer
  if (node === editor) {
    const childIndex = Math.min(Math.max(range.startOffset - 1, 0), Math.max(editor.childNodes.length - 1, 0))
    node = editor.childNodes[childIndex] || editor.firstChild
  }
  const element = node?.nodeType === Node.ELEMENT_NODE ? node : node?.parentElement
  const block = element?.closest?.('p, h1, h2, h3, h4, blockquote, li, pre, .highlight-block') || null
  return block && editor.contains(block) ? block : null
}

const hideBlockInsertControls = (closeMenu = true) => {
  blockInsertControl.value = null
  if (closeMenu) blockStyleMenuVisible.value = false
}

const refreshBlockInsertControl = () => {
  const editor = richEditorRef.value
  const controls = blockInsertControlsRef.value
  const selection = window.getSelection?.()
  const range = selection?.rangeCount ? selection.getRangeAt(0) : null
  const editorOwnsFocus = editor?.matches(':focus') || controls?.contains(document.activeElement)
  if (!editor || !controls || !editorOwnsFocus || !range?.collapsed || !editor.contains(range.commonAncestorContainer)) {
    hideBlockInsertControls()
    return
  }

  const block = getEditorBlockFromRange(range)
  if (!block || block.closest('table')) {
    hideBlockInsertControls()
    return
  }

  const editorRect = editor.getBoundingClientRect()
  const controlsRect = controls.getBoundingClientRect()
  const blockRect = block.getBoundingClientRect()
  const isVisible = blockRect.bottom >= editorRect.top && blockRect.top <= editorRect.bottom
  if (!isVisible) {
    hideBlockInsertControls()
    return
  }

  const left = Math.max(16, blockRect.left - controlsRect.left - 28)
  const top = Math.min(
    Math.max(blockRect.top - controlsRect.top + Math.min(blockRect.height / 2, 16), 16),
    Math.max(16, controlsRect.height - 16)
  )
  const menuWidth = 288
  const menuHeight = 410
  const menuLeft = Math.min(Math.max(left + 20, 8), Math.max(8, controlsRect.width - menuWidth - 8))
  const menuTop = top + 20 + menuHeight <= controlsRect.height
    ? top + 20
    : Math.max(8, top - menuHeight - 20)

  blockInsertControl.value = { left, top, menuLeft, menuTop }
}

const scheduleBlockInsertControlRefresh = () => {
  if (blockControlFrame) cancelAnimationFrame(blockControlFrame)
  blockControlFrame = requestAnimationFrame(() => {
    blockControlFrame = 0
    refreshBlockInsertControl()
  })
}

const toggleBlockStyleMenu = () => {
  blockStyleMenuVisible.value = !blockStyleMenuVisible.value
  if (blockStyleMenuVisible.value) scheduleBlockInsertControlRefresh()
}

const clearActiveTableCell = () => {
  const table = activeTableCell.value?.closest?.('table')
  table?.querySelectorAll?.('.is-active-table-cell, .is-selected-table-row, .is-selected-table-column')
    .forEach(cell => cell.classList.remove('is-active-table-cell', 'is-selected-table-row', 'is-selected-table-column'))
  activeTableCell.value = null
  activeTableSelectionType.value = ''
  tableDeleteControl.value = null
}

const setActiveTableCell = (cell, selectionType = 'cell') => {
  clearActiveTableCell()
  if (!cell || !richEditorRef.value?.contains(cell)) return
  const table = cell.closest('table')
  activeTableCell.value = cell
  if (selectionType === 'column') {
    activeTableSelectionType.value = 'column'
    Array.from(table?.rows || []).forEach(row => row.cells[cell.cellIndex]?.classList.add('is-selected-table-column'))
  } else if (selectionType === 'row') {
    activeTableSelectionType.value = 'row'
    Array.from(cell.parentElement?.cells || []).forEach(rowCell => rowCell.classList.add('is-selected-table-row'))
  } else {
    activeTableSelectionType.value = 'cell'
  }
  cell.classList.add('is-active-table-cell')
}

const hideTableEdgeControls = (clearTable = true) => {
  tableColumnHandles.value = []
  tableRowHandles.value = []
  tableColumnSelectionHandles.value = []
  tableRowSelectionHandles.value = []
  tableDeleteControl.value = null
  tableEdgeControlsVisible.value = false
  tableResizeColumnIndex.value = -1
  if (clearTable) activeTableElement = null
}

const refreshTableEdgeControls = () => {
  const table = activeTableElement
  const editor = richEditorRef.value
  const controls = tableEdgeControlsRef.value
  if (!table?.isConnected || !editor?.contains(table) || !controls) {
    hideTableEdgeControls()
    return
  }

  const editorRect = editor.getBoundingClientRect()
  const controlsRect = controls.getBoundingClientRect()
  const tableRect = table.getBoundingClientRect()
  const tableVisible = tableRect.bottom >= editorRect.top && tableRect.top <= editorRect.bottom &&
    tableRect.right >= editorRect.left && tableRect.left <= editorRect.right
  if (!tableVisible) {
    hideTableEdgeControls(false)
    return
  }

  const firstRowCells = Array.from(table.rows[0]?.cells || [])
  const tableTopVisible = tableRect.top >= editorRect.top && tableRect.top <= editorRect.bottom
  const visibleResizeTop = Math.max(tableRect.top, editorRect.top)
  const visibleResizeBottom = Math.min(tableRect.bottom, editorRect.bottom)
  const sharedHandleStyle = {
    top: tableRect.top - controlsRect.top,
    resizeTop: visibleResizeTop - controlsRect.top,
    height: Math.max(0, visibleResizeBottom - visibleResizeTop)
  }

  tableColumnHandles.value = tableTopVisible && firstRowCells.length
    ? [
        {
          index: 0,
          left: firstRowCells[0].getBoundingClientRect().left - controlsRect.left,
          ...sharedHandleStyle
        },
        ...firstRowCells.map((cell, index) => ({
          index: index + 1,
          left: cell.getBoundingClientRect().right - controlsRect.left,
          ...sharedHandleStyle
        }))
      ]
    : []

  tableColumnSelectionHandles.value = tableTopVisible
    ? firstRowCells.map((cell, index) => {
        const cellRect = cell.getBoundingClientRect()
        return {
          index,
          left: cellRect.left + cellRect.width / 2 - controlsRect.left,
          top: tableRect.top - controlsRect.top
        }
      })
    : []

  tableRowHandles.value = Array.from(table.rows)
    .map((row, index) => ({
      index: index + 1,
      left: tableRect.left - controlsRect.left,
      top: row.getBoundingClientRect().bottom - controlsRect.top
    }))
    .filter(handle => handle.top >= editorRect.top - controlsRect.top && handle.top <= editorRect.bottom - controlsRect.top)

  tableRowSelectionHandles.value = Array.from(table.rows)
    .filter(row => row.parentElement?.tagName !== 'THEAD')
    .map(row => {
      const rowRect = row.getBoundingClientRect()
      return {
        index: row.rowIndex,
        tableRowIndex: row.rowIndex,
        left: tableRect.left - controlsRect.left,
        top: rowRect.top + rowRect.height / 2 - controlsRect.top
      }
    })
    .filter(handle => handle.top >= editorRect.top - controlsRect.top && handle.top <= editorRect.bottom - controlsRect.top)

  const selectedCell = activeTableCell.value
  if (selectedCell?.isConnected && table.contains(selectedCell)) {
    if (activeTableSelectionType.value === 'column') {
      const cellRect = selectedCell.getBoundingClientRect()
      tableDeleteControl.value = {
        type: 'column',
        left: cellRect.left + cellRect.width / 2 - controlsRect.left,
        top: tableRect.top - controlsRect.top,
        disabled: firstRowCells.length <= 1,
        label: firstRowCells.length <= 1 ? '表格至少保留一列' : `删除第 ${selectedCell.cellIndex + 1} 列`
      }
    } else {
      const rowRect = selectedCell.parentElement.getBoundingClientRect()
      tableDeleteControl.value = {
        type: 'row',
        left: tableRect.left - controlsRect.left,
        top: rowRect.top + rowRect.height / 2 - controlsRect.top,
        disabled: false,
        label: `删除第 ${selectedCell.parentElement.rowIndex} 行`
      }
    }
  } else {
    tableDeleteControl.value = null
  }
  tableEdgeControlsVisible.value = Boolean(
    tableColumnHandles.value.length || tableRowHandles.value.length ||
    tableColumnSelectionHandles.value.length || tableRowSelectionHandles.value.length || tableDeleteControl.value
  )
}

const scheduleTableEdgeControlsRefresh = () => {
  if (tableControlFrame) cancelAnimationFrame(tableControlFrame)
  tableControlFrame = requestAnimationFrame(() => {
    tableControlFrame = 0
    refreshTableEdgeControls()
  })
}

const activateTableEdgeControls = table => {
  if (!table || !richEditorRef.value?.contains(table)) return
  activeTableElement = table
  scheduleTableEdgeControlsRefresh()
}

const selectTableColumn = columnIndex => {
  const table = activeTableElement
  const cell = table?.rows[0]?.cells[columnIndex]
  if (!cell) return
  setActiveTableCell(cell, 'column')
  scheduleTableEdgeControlsRefresh()
}

const selectTableRow = rowIndex => {
  const table = activeTableElement
  const cell = table?.rows[rowIndex]?.cells[0]
  if (!cell || cell.parentElement?.parentElement?.tagName === 'THEAD') return
  setActiveTableCell(cell, 'row')
  scheduleTableEdgeControlsRefresh()
}

const handleRichEditorClick = event => {
  blockStyleMenuVisible.value = false
  rememberRichEditorSelection()
  const cell = event.target?.closest?.('td, th') || null
  setActiveTableCell(cell, cell?.tagName?.toLowerCase() === 'th' ? 'column' : 'cell')
  if (cell) activateTableEdgeControls(cell.closest('table'))
  else hideTableEdgeControls()
  scheduleBlockInsertControlRefresh()
}

const handleRichEditorPointerDown = event => {
  const editor = richEditorRef.value
  if (!editor || event.button !== 0 || event.target !== editor) return

  const editorRect = editor.getBoundingClientRect()
  const verticalScrollbarWidth = Math.max(0, editor.offsetWidth - editor.clientWidth)
  const horizontalScrollbarHeight = Math.max(0, editor.offsetHeight - editor.clientHeight)
  if (
    event.clientX >= editorRect.right - verticalScrollbarWidth ||
    event.clientY >= editorRect.bottom - horizontalScrollbarHeight
  ) return

  const rootBlocks = Array.from(editor.children)
  let insertBefore = null
  let isInsideBlockRow = false
  for (const block of rootBlocks) {
    const blockRect = block.getBoundingClientRect()
    if (event.clientY < blockRect.top) {
      insertBefore = block
      break
    }
    if (event.clientY <= blockRect.bottom) {
      isInsideBlockRow = true
      break
    }
  }
  if (isInsideBlockRow) return

  event.preventDefault()
  blockStyleMenuVisible.value = false
  clearActiveTableCell()
  hideTableEdgeControls()

  const paragraph = document.createElement('p')
  paragraph.appendChild(document.createElement('br'))
  editor.insertBefore(paragraph, insertBefore)
  editor.focus({ preventScroll: true })

  const range = document.createRange()
  range.selectNodeContents(paragraph)
  range.collapse(true)
  const selection = window.getSelection?.()
  selection?.removeAllRanges()
  selection?.addRange(range)
  richEditorSelection.value = range.cloneRange()
  syncMarkdownFromRichEditor()
  scheduleBlockInsertControlRefresh()
}

const handleRichEditorPointerMove = event => {
  if (tableColumnResizing.value) return
  const table = event.target?.closest?.('table')
  if (table && richEditorRef.value?.contains(table)) {
    activateTableEdgeControls(table)
    return
  }
  hideTableEdgeControls()
}

const handleRichEditorPointerLeave = event => {
  if (tableColumnResizing.value || tableEdgeControlsRef.value?.contains(event.relatedTarget)) return
  hideTableEdgeControls()
}

const handleRichEditorScroll = () => {
  scheduleTableEdgeControlsRefresh()
  scheduleBlockInsertControlRefresh()
}

const readRenderedTableWidths = table => Array.from(table?.rows?.[0]?.cells || [])
  .map(cell => Math.max(80, Math.round(cell.getBoundingClientRect().width)))

const applyTableColumnWidths = (table, widths) => {
  if (!table || !widths.length) return
  let colgroup = Array.from(table.children).find(child => child.tagName === 'COLGROUP')
  if (!colgroup) {
    colgroup = document.createElement('colgroup')
    table.insertBefore(colgroup, table.firstChild)
  }
  colgroup.replaceChildren(...widths.map(width => {
    const col = document.createElement('col')
    col.style.width = `${width}px`
    return col
  }))
  const totalWidth = widths.reduce((sum, width) => sum + width, 0)
  table.dataset.columnWidths = widths.join(',')
  table.style.width = `${totalWidth}px`
  table.style.minWidth = `${totalWidth}px`
  table.style.maxWidth = 'none'
  table.style.tableLayout = 'fixed'
}

const syncMarkdownFromRichEditor = () => {
  if (isSyncingRichEditor.value) return props.contentMarkdown
  const markdown = htmlToMarkdown(richEditorRef.value)
  lastEmittedMarkdown = markdown
  emit('update:contentMarkdown', markdown)
  return markdown
}

const commitTableMutation = (nextCell, table = activeTableElement, selectionType = 'cell') => {
  setActiveTableCell(nextCell, selectionType)
  syncMarkdownFromRichEditor()
  activateTableEdgeControls(table)
  nextTick(scheduleTableEdgeControlsRefresh)
}

const insertTableRowAt = insertIndex => {
  const table = activeTableElement
  if (!table?.isConnected || !richEditorRef.value?.contains(table)) return
  const rows = Array.from(table.rows)
  const columnCount = Math.max(...rows.map(row => row.cells.length), 1)
  const normalizedIndex = Math.max(1, Math.min(insertIndex, rows.length))
  const body = table.tBodies[0] || table.createTBody()
  const referenceRow = rows[normalizedIndex]
  const newRow = document.createElement('tr')
  Array.from({ length: columnCount }).forEach(() => {
    const cell = document.createElement('td')
    cell.innerHTML = '<br>'
    newRow.appendChild(cell)
  })
  if (referenceRow?.parentElement === body) body.insertBefore(newRow, referenceRow)
  else body.appendChild(newRow)
  commitTableMutation(newRow.cells[0], table)
}

const insertTableColumnAt = insertIndex => {
  const table = activeTableElement
  if (!table?.isConnected || !richEditorRef.value?.contains(table)) return
  const rows = Array.from(table.rows)
  const columnCount = rows[0]?.cells.length || 0
  const normalizedIndex = Math.max(0, Math.min(insertIndex, columnCount))
  const persistedWidths = readPersistedTableWidths(table)
  let nextActiveCell = null

  rows.forEach(row => {
    const isHeader = row.parentElement?.tagName === 'THEAD'
    const newCell = document.createElement(isHeader ? 'th' : 'td')
    newCell.innerHTML = isHeader ? '新列' : '<br>'
    row.insertBefore(newCell, row.cells[normalizedIndex] || null)
    if (!nextActiveCell && !isHeader) nextActiveCell = newCell
  })

  if (persistedWidths) {
    const adjacentWidth = persistedWidths[normalizedIndex - 1] || persistedWidths[normalizedIndex] || 140
    persistedWidths.splice(normalizedIndex, 0, Math.max(110, adjacentWidth))
    applyTableColumnWidths(table, persistedWidths)
  }
  commitTableMutation(nextActiveCell || table.rows[0]?.cells[normalizedIndex], table)
}

const deleteSelectedTablePart = () => {
  const table = activeTableElement
  const selectedCell = activeTableCell.value
  if (!table?.isConnected || !selectedCell?.isConnected || !table.contains(selectedCell)) return

  if (activeTableSelectionType.value === 'column') {
    const columnCount = table.rows[0]?.cells.length || 0
    if (columnCount <= 1) return
    const columnIndex = selectedCell.cellIndex
    const persistedWidths = readPersistedTableWidths(table)
    Array.from(table.rows).forEach(row => {
      if (row.cells[columnIndex]) row.deleteCell(columnIndex)
    })
    if (persistedWidths) {
      persistedWidths.splice(columnIndex, 1)
      applyTableColumnWidths(table, persistedWidths)
    }
    const nextColumnIndex = Math.min(columnIndex, columnCount - 2)
    commitTableMutation(table.rows[0]?.cells[nextColumnIndex] || null, table, 'column')
    return
  }

  const row = selectedCell.parentElement
  if (!row || row.parentElement?.tagName === 'THEAD') return
  const rowIndex = row.rowIndex
  const cellIndex = selectedCell.cellIndex
  row.remove()
  const nextRow = table.rows[Math.min(rowIndex, table.rows.length - 1)] || table.rows[0]
  const nextCellIndex = Math.min(cellIndex, Math.max((nextRow?.cells.length || 1) - 1, 0))
  const nextSelectionType = nextRow?.parentElement?.tagName === 'THEAD' ? 'cell' : 'row'
  commitTableMutation(nextRow?.cells[nextCellIndex] || null, table, nextSelectionType)
}

const handleTableColumnResize = event => {
  if (!tableResizeState) return
  const { table, columnIndex, startX, startWidth, widths } = tableResizeState
  const nextWidth = Math.max(80, Math.min(720, Math.round(startWidth + event.clientX - startX)))
  const nextWidths = [...widths]
  nextWidths[columnIndex] = nextWidth
  applyTableColumnWidths(table, nextWidths)
  scheduleTableEdgeControlsRefresh()
}

const finishTableColumnResize = () => {
  if (!tableResizeState) return
  window.removeEventListener('pointermove', handleTableColumnResize)
  window.removeEventListener('pointerup', finishTableColumnResize)
  window.removeEventListener('pointercancel', finishTableColumnResize)
  document.body.classList.remove('knowledge-table-resizing')
  tableResizeState = null
  tableColumnResizing.value = false
  tableResizeColumnIndex.value = -1
  syncMarkdownFromRichEditor()
  scheduleTableEdgeControlsRefresh()
}

const startTableColumnResize = (event, boundaryIndex) => {
  const table = activeTableElement
  const columnIndex = boundaryIndex - 1
  if (!table?.isConnected || columnIndex < 0) return
  const widths = readPersistedTableWidths(table) || readRenderedTableWidths(table)
  if (!widths[columnIndex]) return
  applyTableColumnWidths(table, widths)
  tableResizeState = { table, columnIndex, startX: event.clientX, startWidth: widths[columnIndex], widths }
  tableColumnResizing.value = true
  tableResizeColumnIndex.value = boundaryIndex
  document.body.classList.add('knowledge-table-resizing')
  window.addEventListener('pointermove', handleTableColumnResize)
  window.addEventListener('pointerup', finishTableColumnResize)
  window.addEventListener('pointercancel', finishTableColumnResize)
}

const resizeTableColumnByKeyboard = (boundaryIndex, key) => {
  const table = activeTableElement
  const columnIndex = boundaryIndex - 1
  if (!table?.isConnected || columnIndex < 0) return
  const widths = readPersistedTableWidths(table) || readRenderedTableWidths(table)
  const delta = key === 'ArrowLeft' ? -16 : 16
  widths[columnIndex] = Math.max(80, Math.min(720, widths[columnIndex] + delta))
  applyTableColumnWidths(table, widths)
  syncMarkdownFromRichEditor()
  scheduleTableEdgeControlsRefresh()
}

const syncRichEditorFromMarkdown = async () => {
  await nextTick()
  if (!richEditorRef.value) return
  clearActiveTableCell()
  hideTableEdgeControls()
  hideBlockInsertControls()
  colorPickerVisible.value = false
  const html = renderMarkdown(props.contentMarkdown || '', { interactiveTasks: true })
  if (richEditorRef.value.innerHTML === html) return
  isSyncingRichEditor.value = true
  richEditorRef.value.innerHTML = html
  isSyncingRichEditor.value = false
}

const normalizeRichEditorRoot = () => {
  const editor = richEditorRef.value
  if (!editor || editor.innerHTML.trim()) return
  editor.innerHTML = '<p><br></p>'
}

const execRichCommand = command => {
  restoreRichEditorSelection()
  document.execCommand(command, false, null)
  normalizeRichEditorRoot()
  syncMarkdownFromRichEditor()
  rememberRichEditorSelection()
}

const setRichBlock = tag => {
  restoreRichEditorSelection()
  document.execCommand('formatBlock', false, tag)
  normalizeRichEditorRoot()
  syncMarkdownFromRichEditor()
  rememberRichEditorSelection()
}

const insertRichHtml = html => {
  restoreRichEditorSelection()
  document.execCommand('insertHTML', false, html)
  normalizeRichEditorRoot()
  syncMarkdownFromRichEditor()
  rememberRichEditorSelection()
}

const insertRichLink = () => {
  restoreRichEditorSelection()
  const selection = window.getSelection?.()
  const selected = selection?.toString()?.trim() || ''
  if (selected && /^https?:\/\//i.test(selected)) {
    insertRichHtml(`<a href="${escapeHtml(selected)}">${escapeHtml('显示文本')}</a>`)
    return
  }
  insertRichHtml(`<a href="${escapeHtml('URL链接')}">${escapeHtml(selected || '显示文本')}</a>`)
}

const insertRichImage = () => {
  restoreRichEditorSelection()
  const selected = window.getSelection?.().toString()?.trim() || '图片URL'
  insertRichHtml(`<img src="${escapeHtml(selected)}" alt="${escapeHtml('图片描述')}" />`)
}

const insertRichCodeBlock = () => {
  restoreRichEditorSelection()
  const selected = window.getSelection?.().toString() || '代码内容'
  insertRichHtml(`<pre><code data-lang="语言">${escapeHtml(selected)}</code></pre><p><br></p>`)
}

const insertHighlightBlock = () => {
  restoreRichEditorSelection()
  const editor = richEditorRef.value
  const selection = window.getSelection?.()
  const range = selection?.rangeCount ? selection.getRangeAt(0) : null
  if (!editor || !selection || !range || !editor.contains(range.commonAncestorContainer)) return

  const currentBlock = getEditorBlockFromRange(range)
  if (range.collapsed && currentBlock?.classList.contains('highlight-block')) return
  const highlightIcon = getRandomHighlightBlockIcon()

  if (range.collapsed && currentBlock?.parentElement === editor && currentBlock.tagName !== 'PRE') {
    const marker = document.createElement('span')
    marker.dataset.editorCaret = 'true'
    range.insertNode(marker)
    const highlightBlock = document.createElement('div')
    highlightBlock.className = 'highlight-block'
    highlightBlock.dataset.highlightIcon = highlightIcon
    while (currentBlock.firstChild) highlightBlock.appendChild(currentBlock.firstChild)
    if (!highlightBlock.textContent?.trim() && !highlightBlock.querySelector('br')) highlightBlock.appendChild(document.createElement('br'))
    currentBlock.replaceWith(highlightBlock)

    const nextRange = document.createRange()
    nextRange.setStartBefore(marker)
    nextRange.collapse(true)
    marker.remove()
    selection.removeAllRanges()
    selection.addRange(nextRange)
    richEditorSelection.value = nextRange.cloneRange()
    syncMarkdownFromRichEditor()
    scheduleBlockInsertControlRefresh()
    return
  }

  const selected = selection.toString()
  const content = escapeHtml(selected).replace(/\r?\n/g, '<br>') || '<br>'
  insertRichHtml(`<div class="highlight-block" data-highlight-icon="${escapeHtml(highlightIcon)}">${content}</div><p><br></p>`)
}

const getSelectedTextSegments = range => {
  const editor = richEditorRef.value
  if (!editor || !range || range.collapsed) return []
  const walker = document.createTreeWalker(editor, window.NodeFilter.SHOW_TEXT)
  const segments = []
  let node = walker.nextNode()
  while (node) {
    const parent = node.parentElement
    const excluded = parent?.closest?.('pre, [contenteditable="false"]')
    let intersects = false
    if (!excluded && node.textContent) {
      try {
        intersects = range.intersectsNode(node)
      } catch {
        intersects = false
      }
    }
    if (intersects) {
      const start = node === range.startContainer ? range.startOffset : 0
      const end = node === range.endContainer ? range.endOffset : node.textContent.length
      if (start < end) segments.push({ node, start, end })
    }
    node = walker.nextNode()
  }
  return segments
}

const restoreWrappedSelection = wrappers => {
  if (!wrappers.length) return
  const range = document.createRange()
  range.setStartBefore(wrappers[0])
  range.setEndAfter(wrappers[wrappers.length - 1])
  const selection = window.getSelection?.()
  selection?.removeAllRanges()
  selection?.addRange(range)
  richEditorSelection.value = range.cloneRange()
}

const applySelectionColor = (type, color) => {
  restoreRichEditorSelection()
  const selection = window.getSelection?.()
  const range = selection?.rangeCount ? selection.getRangeAt(0) : null
  const segments = getSelectedTextSegments(range)
  if (!segments.length) {
    if (!range?.collapsed) return
    const rangeElement = range.startContainer.nodeType === Node.ELEMENT_NODE
      ? range.startContainer
      : range.startContainer.parentElement
    if (rangeElement?.closest?.('pre, [contenteditable="false"]')) return

    const mark = document.createElement('span')
    const placeholder = document.createTextNode('\u200b')
    mark.className = 'text-color-mark'
    if (type === 'text') {
      mark.dataset.textColor = color
      mark.style.color = color
      activeTextColor.value = color
    } else {
      mark.dataset.backgroundColor = color
      mark.style.backgroundColor = color
      activeBackgroundColor.value = color
    }
    mark.appendChild(placeholder)
    range.insertNode(mark)
    range.setStart(placeholder, placeholder.textContent.length)
    range.collapse(true)
    selection.removeAllRanges()
    selection.addRange(range)
    richEditorSelection.value = range.cloneRange()
    syncMarkdownFromRichEditor()
    scheduleBlockInsertControlRefresh()
    return
  }

  const wrappers = []
  segments.slice().reverse().forEach(segment => {
    const segmentRange = document.createRange()
    segmentRange.setStart(segment.node, segment.start)
    segmentRange.setEnd(segment.node, segment.end)
    const mark = document.createElement('span')
    mark.className = 'text-color-mark'
    if (type === 'text') {
      mark.dataset.textColor = color
      mark.style.color = color
    } else {
      mark.dataset.backgroundColor = color
      mark.style.backgroundColor = color
    }
    segmentRange.surroundContents(mark)
    wrappers.unshift(mark)
  })

  if (type === 'text') activeTextColor.value = color
  else activeBackgroundColor.value = color
  restoreWrappedSelection(wrappers)
  syncMarkdownFromRichEditor()
  scheduleBlockInsertControlRefresh()
}

const resetSelectionColors = () => {
  restoreRichEditorSelection()
  const editor = richEditorRef.value
  const selection = window.getSelection?.()
  const range = selection?.rangeCount ? selection.getRangeAt(0) : null
  if (!editor || !range) return

  if (range.collapsed) {
    const rangeElement = range.startContainer.nodeType === Node.ELEMENT_NODE
      ? range.startContainer
      : range.startContainer.parentElement
    const marks = []
    let mark = rangeElement?.closest?.('.text-color-mark') || null
    while (mark && editor.contains(mark)) {
      marks.push(mark)
      mark = mark.parentElement?.closest?.('.text-color-mark') || null
    }
    if (!marks.length) return

    const caretMarker = document.createElement('span')
    range.insertNode(caretMarker)
    marks.forEach(colorMark => colorMark.replaceWith(...Array.from(colorMark.childNodes)))
    const nextRange = document.createRange()
    nextRange.setStartBefore(caretMarker)
    nextRange.collapse(true)
    caretMarker.remove()
    selection.removeAllRanges()
    selection.addRange(nextRange)
    richEditorSelection.value = nextRange.cloneRange()
    activeTextColor.value = ''
    activeBackgroundColor.value = ''
    syncMarkdownFromRichEditor()
    scheduleBlockInsertControlRefresh()
    return
  }

  const marks = Array.from(editor.querySelectorAll('.text-color-mark')).filter(mark => {
    try {
      return range.intersectsNode(mark)
    } catch {
      return false
    }
  })
  marks.sort((first, second) => first.contains(second) ? 1 : (second.contains(first) ? -1 : 0))
  marks.forEach(mark => mark.replaceWith(...Array.from(mark.childNodes)))
  activeTextColor.value = ''
  activeBackgroundColor.value = ''
  syncMarkdownFromRichEditor()
  rememberRichEditorSelection()
  scheduleBlockInsertControlRefresh()
}

const findDirectTaskCheckbox = item => Array.from(item?.children || [])
  .find(child => child.tagName?.toLowerCase() === 'input' && child.type === 'checkbox')

const insertTaskList = () => {
  restoreRichEditorSelection()
  const editor = richEditorRef.value
  const selection = window.getSelection?.()
  const range = selection?.rangeCount ? selection.getRangeAt(0) : null
  if (!editor || !selection || !range || !editor.contains(range.commonAncestorContainer)) return

  const selectedItems = range.collapsed
    ? []
    : Array.from(editor.querySelectorAll('li')).filter(item => {
        try {
          return range.intersectsNode(item)
        } catch {
          return false
        }
      })

  if (selectedItems.length) {
    const parentLists = new Set()
    selectedItems.forEach(item => {
      if (!findDirectTaskCheckbox(item)) {
        const checkbox = document.createElement('input')
        checkbox.type = 'checkbox'
        checkbox.contentEditable = 'false'
        checkbox.setAttribute('aria-label', '未完成任务')
        item.insertBefore(checkbox, item.firstChild)
      }
      item.classList.add('task-list-item')
      if (item.parentElement) parentLists.add(item.parentElement)
    })
    parentLists.forEach(list => {
      const items = Array.from(list.children).filter(child => child.tagName?.toLowerCase() === 'li')
      if (items.length && items.every(findDirectTaskCheckbox)) list.classList.add('task-list')
    })
    syncMarkdownFromRichEditor()
    rememberRichEditorSelection()
    return
  }

  const selectedLines = selection.toString()
    .split(/\r?\n+/)
    .map(line => line.trim())
    .filter(Boolean)
  const taskLines = selectedLines.length ? selectedLines : ['待办']
  const taskHtml = taskLines
    .map(line => `<li class="task-list-item"><input type="checkbox" contenteditable="false" aria-label="未完成任务">${escapeHtml(line)}</li>`)
    .join('')
  insertRichHtml(`<ul class="task-list">${taskHtml}</ul>`)
}

const resetTablePickerHover = () => {
  tablePickerHover.rows = 3
  tablePickerHover.cols = 3
}

const setTablePickerHover = (rows, cols) => {
  tablePickerHover.rows = rows
  tablePickerHover.cols = cols
}

const createMarkdownTable = (rows, cols) => {
  const headers = Array.from({ length: cols }, (_, index) => `列${index + 1}`)
  const separator = Array.from({ length: cols }, () => ':---').join(' | ')
  const body = Array.from({ length: Math.max(rows - 1, 0) }, (_, rowIndex) => {
    const cells = Array.from({ length: cols }, (_, colIndex) => `内容${rowIndex + 1}-${colIndex + 1}`)
    return `| ${cells.join(' | ')} |`
  })
  return [`| ${headers.join(' | ')} |`, `| ${separator} |`, ...body].join('\n')
}

const insertMarkdownTable = (rows, cols) => {
  rememberRichEditorSelection()
  tablePickerVisible.value = false
  resetTablePickerHover()
  insertRichHtml(renderMarkdown(createMarkdownTable(rows, cols)))
}

const insertMarkdownSnippet = key => {
  if (key === 'bold') return execRichCommand('bold')
  if (key === 'italic') return execRichCommand('italic')
  if (key === 'strike') return execRichCommand('strikeThrough')
  if (key === 'inlineCode') return insertRichHtml(`<code>${escapeHtml(window.getSelection?.().toString() || '代码内容')}</code>`)
  if (key === 'link') return insertRichLink()
  if (key === 'image') return insertRichImage()
  if (key === 'codeBlock') return insertRichCodeBlock()
  if (key === 'h1') return setRichBlock('h1')
  if (key === 'h2') return setRichBlock('h2')
  if (key === 'h3') return setRichBlock('h3')
  if (key === 'quote') return setRichBlock('blockquote')
  if (key === 'ul') return execRichCommand('insertUnorderedList')
  if (key === 'ol') return execRichCommand('insertOrderedList')
  if (key === 'task') return insertTaskList()
  if (key === 'hr') return insertRichHtml('<hr />')
  if (key === 'highlightBlock') return insertHighlightBlock()
  return insertRichHtml(renderMarkdown(markdownSnippets[key] || ''))
}

const applyBlockStyle = key => {
  blockStyleMenuVisible.value = false
  if (key === 'color') colorPickerVisible.value = true
  else if (key === 'table') insertMarkdownTable(3, 3)
  else insertMarkdownSnippet(key)
  nextTick(scheduleBlockInsertControlRefresh)
}

const rememberEditorSelection = () => rememberRichEditorSelection()

const handleRichEditorInput = () => {
  normalizeRichEditorRoot()
  syncMarkdownFromRichEditor()
  rememberRichEditorSelection()
  scheduleBlockInsertControlRefresh()
}

const handleRichEditorChange = event => {
  const checkbox = event.target?.closest?.('input[type="checkbox"]')
  if (!checkbox || !richEditorRef.value?.contains(checkbox)) return
  checkbox.closest('li')?.classList.toggle('is-task-complete', checkbox.checked)
  checkbox.setAttribute('aria-label', checkbox.checked ? '已完成任务' : '未完成任务')
  syncMarkdownFromRichEditor()
}

const editableNodeToPlainText = node => {
  if (!node) return ''
  if (node.nodeType === Node.TEXT_NODE) return node.textContent || ''
  if (node.nodeType === Node.ELEMENT_NODE && node.tagName.toLowerCase() === 'br') return '\n'

  const children = Array.from(node.childNodes || [])
  return children.reduce((text, child, index) => {
    const tag = child.nodeType === Node.ELEMENT_NODE ? child.tagName.toLowerCase() : ''
    const isLineBlock = tag === 'div' || tag === 'p'
    let nextText = text
    if (isLineBlock && nextText && !nextText.endsWith('\n')) nextText += '\n'
    nextText += editableNodeToPlainText(child)
    if (isLineBlock && index < children.length - 1 && !nextText.endsWith('\n')) nextText += '\n'
    return nextText
  }, '')
}

const convertSimpleBlockToOrderedList = (block, range) => {
  const textNode = block?.childNodes?.length === 1 ? block.firstChild : null
  const isSupportedBlock = block?.classList?.contains('highlight-block') || block?.tagName === 'P'
  if (!isSupportedBlock || textNode?.nodeType !== Node.TEXT_NODE || range.startContainer !== textNode) return false

  const marker = String(textNode.textContent || '').match(/^\s*1\.\s+/)
  if (!marker || range.startOffset < marker[0].length) return false
  const firstItemText = textNode.textContent.slice(marker[0].length, range.startOffset)
  if (!firstItemText.trim()) return false

  const secondItemText = textNode.textContent.slice(range.startOffset)
  const list = document.createElement('ol')
  const firstItem = document.createElement('li')
  const secondItem = document.createElement('li')
  firstItem.textContent = firstItemText
  if (secondItemText) secondItem.textContent = secondItemText
  else secondItem.appendChild(document.createElement('br'))
  list.append(firstItem, secondItem)

  if (block.classList.contains('highlight-block')) block.replaceChildren(list)
  else block.replaceWith(list)

  const nextRange = document.createRange()
  nextRange.selectNodeContents(secondItem)
  nextRange.collapse(true)
  const selection = window.getSelection?.()
  selection?.removeAllRanges()
  selection?.addRange(nextRange)
  richEditorSelection.value = nextRange.cloneRange()
  return true
}

const handleRichEditorKeydown = event => {
  if (event.key !== 'Enter' || event.shiftKey || event.isComposing) return
  const editor = richEditorRef.value
  const selection = window.getSelection?.()
  const range = selection?.rangeCount ? selection.getRangeAt(0) : null
  if (!editor || !range?.collapsed || !editor.contains(range.commonAncestorContainer)) return

  const rangeElement = range.startContainer.nodeType === Node.ELEMENT_NODE
    ? range.startContainer
    : range.startContainer.parentElement
  if (rangeElement?.closest?.('li, pre, table')) return

  const currentBlock = getEditorBlockFromRange(range)
  if (!currentBlock) return
  const prefixRange = document.createRange()
  prefixRange.selectNodeContents(currentBlock)
  try {
    prefixRange.setEnd(range.startContainer, range.startOffset)
  } catch {
    return
  }
  const currentLine = editableNodeToPlainText(prefixRange.cloneContents()).split('\n').pop() || ''
  const orderedLine = currentLine.match(/^\s*(\d+)\.\s+(\S.*)$/)
  if (!orderedLine) return

  event.preventDefault()
  if (Number.parseInt(orderedLine[1], 10) === 1 && convertSimpleBlockToOrderedList(currentBlock, range)) {
    syncMarkdownFromRichEditor()
    rememberRichEditorSelection()
    scheduleBlockInsertControlRefresh()
    return
  }
  const nextNumber = Number.parseInt(orderedLine[1], 10) + 1
  document.execCommand('insertHTML', false, `<br>${nextNumber}.&nbsp;`)
  normalizeRichEditorRoot()
  syncMarkdownFromRichEditor()
  rememberRichEditorSelection()
  scheduleBlockInsertControlRefresh()
}

const handleRichEditorKeyup = event => {
  if (event.key === 'Escape') blockStyleMenuVisible.value = false
  if (event.key === 'Enter') normalizeRichEditorRoot()
  handleRichEditorInput()
}

const handleRichEditorFocus = () => {
  rememberRichEditorSelection()
  scheduleBlockInsertControlRefresh()
}

const handleRichEditorBlur = () => {
  requestAnimationFrame(() => {
    const controlsOwnFocus = blockInsertControlsRef.value?.contains(document.activeElement)
    if (!richEditorRef.value?.matches(':focus') && !controlsOwnFocus) hideBlockInsertControls()
  })
}

const handleRichEditorMouseup = () => {
  rememberRichEditorSelection()
  scheduleBlockInsertControlRefresh()
}

const handleRichEditorPaste = event => {
  event.preventDefault()
  const text = event.clipboardData?.getData('text/plain') || ''
  insertRichHtml(escapeHtml(text).replace(/\r?\n/g, '<br>'))
}

const validate = () => formRef.value?.validate()
const scrollToTop = () => {
  if (richEditorRef.value) richEditorRef.value.scrollTop = 0
}

watch(() => props.contentMarkdown, value => {
  if (value === lastEmittedMarkdown) {
    lastEmittedMarkdown = ''
    return
  }
  syncRichEditorFromMarkdown()
})

onMounted(() => {
  window.addEventListener('resize', scheduleTableEdgeControlsRefresh)
  window.addEventListener('resize', scheduleBlockInsertControlRefresh)
  syncRichEditorFromMarkdown()
})

onBeforeUnmount(() => {
  if (tableControlFrame) cancelAnimationFrame(tableControlFrame)
  if (blockControlFrame) cancelAnimationFrame(blockControlFrame)
  window.removeEventListener('resize', scheduleTableEdgeControlsRefresh)
  window.removeEventListener('resize', scheduleBlockInsertControlRefresh)
  window.removeEventListener('pointermove', handleTableColumnResize)
  window.removeEventListener('pointerup', finishTableColumnResize)
  window.removeEventListener('pointercancel', finishTableColumnResize)
  document.body.classList.remove('knowledge-table-resizing')
})

defineExpose({
  validate,
  syncFromMarkdown: syncRichEditorFromMarkdown,
  syncToMarkdown: syncMarkdownFromRichEditor,
  scrollToTop,
  hideTableEdgeControls,
  hideBlockInsertControls
})
</script>

<style scoped>
.thinktank-editor,
.document-form,
.editor-grid,
.markdown-editor-shell {
  height: 100%;
  min-height: 0;
}

.thinktank-editor,
.document-form,
.markdown-editor-shell {
  overflow: hidden;
}

.document-form {
  display: block;
}

.document-form :deep(.el-form-item) {
  align-items: center;
  margin-bottom: 0;
}

.document-form :deep(.el-form-item__label) {
  width: 68px;
  min-width: 68px;
  height: 28px;
  justify-content: flex-start;
  padding: 0 8px 0 0;
  color: var(--think-text-secondary);
  font-size: 12px;
  line-height: 28px;
}

.document-form :deep(.el-form-item__content) {
  min-width: 0;
}

.document-form :deep(.el-input__wrapper),
.document-form :deep(.el-select__wrapper) {
  min-height: 28px;
  border-radius: 6px;
}

.document-form :deep(.el-select__tags) {
  max-width: calc(100% - 28px);
}

.editor-grid {
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.markdown-editor-shell {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--think-border-cool);
  border-radius: 8px;
  background: #fff;
}

.editor-form-item {
  height: auto;
  max-height: none;
  min-height: 0;
  flex: 1 1 0;
  align-items: stretch !important;
  margin-bottom: 0;
  overflow: hidden;
}

.editor-form-item :deep(.el-form-item__label) {
  display: none;
}

.editor-form-item :deep(.el-form-item__content) {
  position: relative;
  height: 100%;
  max-height: 100%;
  min-height: 0;
  display: flex;
  flex: 1 1 0;
  margin-left: 0 !important;
  overflow: hidden;
}

.markdown-toolbar {
  display: flex;
  align-items: center;
  flex: 0 0 auto;
  flex-wrap: nowrap;
  gap: 4px;
  min-height: 36px;
  overflow-x: auto;
  overflow-y: hidden;
  padding: 5px 6px;
  border-bottom: 1px solid var(--think-border-soft);
  background: #f8fbff;
}

.markdown-tool-group {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding-right: 6px;
  border-right: 1px solid rgba(223, 231, 242, 0.78);
  white-space: nowrap;
}

.markdown-tool-group:last-child {
  border-right: 0;
}

.markdown-tool {
  height: 24px;
  padding: 0 7px;
  border: 1px solid rgba(214, 223, 235, 0.92);
  border-radius: 6px;
  background: #fff;
  color: var(--think-text-secondary);
  cursor: pointer;
  font-size: 12px;
  font-weight: 650;
  transition: border-color 0.2s ease, background-color 0.2s ease, color 0.2s ease;
}

.document-info-trigger {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  border-color: #b9d4f5;
  color: var(--think-brand-active);
}

.document-info-trigger.active {
  border-color: var(--think-brand);
  background: var(--think-brand-soft);
}

.markdown-tool:hover {
  border-color: rgba(120, 156, 221, 0.72);
  background: rgba(76, 139, 245, 0.08);
  color: var(--think-brand-active);
}

.rich-color-trigger {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 0 5px;
}

.rich-color-trigger.active {
  border-color: rgba(64, 158, 255, 0.72);
  background: #ecf5ff;
  color: #3370ff;
}

.rich-color-trigger-letter {
  width: 17px;
  height: 18px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-bottom: 2px solid currentColor;
  border-radius: 2px;
  font-size: 14px;
  line-height: 18px;
}

.rich-color-trigger :deep(.el-icon) {
  font-size: 10px;
}

:global(.markdown-table-popover) {
  padding: 10px !important;
  border: 1px solid rgba(214, 223, 235, 0.96) !important;
  border-radius: 8px !important;
  box-shadow: 0 18px 44px rgba(15, 23, 42, 0.12) !important;
}

:global(.markdown-color-popover) {
  padding: 10px !important;
  border: 1px solid rgba(203, 213, 225, 0.94) !important;
  border-radius: 8px !important;
  box-shadow: 0 18px 44px rgba(15, 23, 42, 0.14) !important;
}

.rich-color-picker {
  display: grid;
  gap: 10px;
}

.rich-color-section {
  display: grid;
  gap: 6px;
}

.rich-color-section strong {
  color: #475569;
  font-size: 12px;
  font-weight: 650;
}

.rich-color-grid {
  display: grid;
  grid-template-columns: repeat(8, 26px);
  gap: 5px;
}

.rich-color-grid.background-grid {
  grid-template-columns: repeat(7, 26px);
}

.rich-color-swatch {
  width: 26px;
  height: 26px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 1px solid #dbe3ed;
  border-radius: 4px;
  outline: none;
  background: #fff;
  cursor: pointer;
  transition: border-color 0.16s ease, box-shadow 0.16s ease, background-color 0.16s ease;
}

.rich-color-swatch:hover,
.rich-color-swatch:focus-visible,
.rich-color-swatch.active {
  border-color: #3370ff;
  box-shadow: 0 0 0 2px rgba(51, 112, 255, 0.16);
}

.rich-color-swatch.text-swatch span {
  font-size: 16px;
  font-weight: 700;
  line-height: 1;
}

.rich-color-swatch.background-swatch span {
  width: 18px;
  height: 18px;
  border-radius: 3px;
}

.rich-color-reset {
  width: 100%;
  height: 30px;
  padding: 0 10px;
  border: 1px solid #dbe3ed;
  border-radius: 5px;
  outline: none;
  background: #fff;
  color: #475569;
  cursor: pointer;
  font-size: 12px;
  transition: border-color 0.16s ease, background-color 0.16s ease, color 0.16s ease;
}

.rich-color-reset:hover,
.rich-color-reset:focus-visible {
  border-color: #93b4e8;
  background: #f1f6ff;
  color: #1d4ed8;
}

:global(.document-meta-popover) {
  --think-border-cool: #dbe7f4;
  --think-text-primary: #172033;
  --think-text-secondary: #53657d;
  --think-text-muted: #64748b;
  max-width: calc(100vw - 32px);
  padding: 14px !important;
  border: 1px solid var(--think-border-cool) !important;
  border-radius: 8px !important;
  box-shadow: 0 16px 44px rgba(15, 23, 42, 0.14) !important;
}

.document-meta-panel {
  display: grid;
  gap: 12px;
}

.document-meta-panel-head {
  display: flex;
  align-items: baseline;
  gap: 10px;
}

.document-meta-panel-head strong {
  color: var(--think-text-primary);
  font-size: 14px;
  font-weight: 750;
}

.document-meta-panel-head span {
  color: var(--think-text-muted);
  font-size: 11px;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 14px;
}

.document-meta-form {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px 14px;
}

.document-meta-form :deep(.el-form-item) {
  align-items: stretch;
  flex-direction: column;
}

.document-meta-form :deep(.el-form-item__label) {
  width: auto;
  height: 24px;
  justify-content: flex-start;
  padding: 0;
  line-height: 24px;
}

.document-meta-form :deep(.el-input__wrapper),
.document-meta-form :deep(.el-select__wrapper) {
  min-height: 34px;
}

.table-picker {
  display: grid;
  gap: 10px;
}

.table-picker-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: var(--think-text-secondary);
  font-size: 12px;
}

.table-picker-head strong {
  color: var(--think-text-primary);
  font-size: 12px;
  font-weight: 750;
}

.table-picker-grid {
  display: grid;
  grid-template-columns: repeat(8, 20px);
  gap: 5px;
}

.table-picker-cell {
  width: 20px;
  height: 20px;
  padding: 0;
  border: 1px solid rgba(226, 232, 240, 0.92);
  border-radius: 3px;
  background: rgba(241, 245, 249, 0.9);
  cursor: pointer;
  transition: background-color 0.12s ease, border-color 0.12s ease;
}

.table-picker-cell.active {
  border-color: rgba(137, 169, 231, 0.85);
  background: rgba(133, 166, 239, 0.82);
}

.table-edge-controls {
  position: absolute;
  inset: 0;
  z-index: 8;
  overflow: visible;
  pointer-events: none;
  opacity: 0;
  transition: opacity 0.16s ease;
}

.table-edge-controls.visible {
  opacity: 1;
}

.table-boundary-add {
  position: absolute;
  z-index: 6;
  width: 24px;
  height: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 0;
  border-radius: 50%;
  outline: none;
  background: transparent;
  color: #fff;
  cursor: pointer;
  pointer-events: auto;
  transform: translate(-50%, -50%);
  transition: background-color 0.16s ease, box-shadow 0.16s ease;
}

.table-boundary-dot {
  width: 6px;
  height: 6px;
  border-radius: 2px;
  background: #aeb9c7;
  box-shadow: 0 0 0 1px rgba(255, 255, 255, 0.94);
  transition: opacity 0.12s ease;
}

.table-boundary-add :deep(.el-icon) {
  position: absolute;
  opacity: 0;
  font-size: 14px;
  font-weight: 700;
  transition: opacity 0.12s ease;
}

.table-boundary-add:hover,
.table-boundary-add:focus-visible {
  background: #3370ff;
  box-shadow: 0 2px 8px rgba(51, 112, 255, 0.3);
}

.table-boundary-add:hover .table-boundary-dot,
.table-boundary-add:focus-visible .table-boundary-dot {
  opacity: 0;
}

.table-boundary-add:hover :deep(.el-icon),
.table-boundary-add:focus-visible :deep(.el-icon) {
  opacity: 1;
}

.table-selection-trigger {
  position: absolute;
  z-index: 4;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 0;
  border-radius: 4px;
  outline: none;
  background: transparent;
  cursor: pointer;
  pointer-events: auto;
  transform: translate(-50%, -50%);
  transition: background-color 0.16s ease, box-shadow 0.16s ease;
}

.table-column-selection-trigger {
  width: 40px;
  height: 18px;
}

.table-row-selection-trigger {
  width: 18px;
  height: 38px;
}

.table-selection-trigger span {
  border-radius: 2px;
  background: #aeb9c7;
  transition: background-color 0.16s ease, box-shadow 0.16s ease;
}

.table-column-selection-trigger span {
  width: 30px;
  height: 5px;
}

.table-row-selection-trigger span {
  width: 5px;
  height: 28px;
}

.table-selection-trigger:hover,
.table-selection-trigger:focus-visible {
  background: rgba(51, 112, 255, 0.1);
  box-shadow: 0 0 0 2px rgba(51, 112, 255, 0.16);
}

.table-selection-trigger:hover span,
.table-selection-trigger:focus-visible span {
  background: #3370ff;
  box-shadow: 0 2px 6px rgba(51, 112, 255, 0.28);
}

.table-column-resize-handle {
  position: absolute;
  z-index: 2;
  width: 10px;
  min-height: 18px;
  outline: none;
  cursor: col-resize;
  pointer-events: auto;
  transform: translateX(-50%);
  touch-action: none;
}

.table-column-resize-handle::after {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 50%;
  width: 2px;
  content: '';
  background: #3370ff;
  opacity: 0;
  transform: translateX(-50%);
  transition: opacity 0.14s ease;
}

.table-column-resize-handle:hover::after,
.table-column-resize-handle:focus-visible::after,
.table-column-resize-handle.active::after {
  opacity: 1;
}

.table-edge-controls.resizing .table-boundary-add {
  pointer-events: none;
  opacity: 0;
}

.table-selection-delete {
  position: absolute;
  z-index: 7;
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 1px solid rgba(239, 68, 68, 0.3);
  border-radius: 6px;
  outline: none;
  background: #fff;
  box-shadow: 0 4px 14px rgba(15, 23, 42, 0.14);
  color: #ef4444;
  cursor: pointer;
  pointer-events: auto;
  transform: translate(-50%, -50%);
  transition: border-color 0.16s ease, background-color 0.16s ease, box-shadow 0.16s ease;
}

.table-selection-delete:hover,
.table-selection-delete:focus-visible {
  border-color: rgba(239, 68, 68, 0.62);
  background: #fff1f2;
  box-shadow: 0 0 0 3px rgba(239, 68, 68, 0.14), 0 4px 14px rgba(15, 23, 42, 0.14);
}

.table-selection-delete:disabled {
  border-color: rgba(148, 163, 184, 0.3);
  background: #f8fafc;
  box-shadow: none;
  color: #a8b3c2;
  cursor: not-allowed;
}

.table-edge-controls.resizing .table-selection-delete {
  pointer-events: none;
  opacity: 0;
}

.table-edge-controls.resizing .table-selection-trigger {
  pointer-events: none;
  opacity: 0;
}

.block-insert-controls {
  position: absolute;
  inset: 0;
  z-index: 9;
  overflow: visible;
  pointer-events: none;
}

.block-insert-trigger {
  position: absolute;
  width: 26px;
  height: 26px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 1px solid rgba(194, 205, 219, 0.92);
  border-radius: 6px;
  outline: none;
  background: #fff;
  box-shadow: 0 2px 8px rgba(15, 23, 42, 0.08);
  color: #64748b;
  cursor: pointer;
  pointer-events: auto;
  transform: translate(-50%, -50%);
  transition: border-color 0.16s ease, background-color 0.16s ease, color 0.16s ease, box-shadow 0.16s ease;
}

.block-insert-trigger:hover,
.block-insert-trigger:focus-visible,
.block-insert-trigger.active {
  border-color: rgba(64, 158, 255, 0.72);
  background: #ecf5ff;
  box-shadow: 0 0 0 3px rgba(64, 158, 255, 0.12);
  color: #3370ff;
}

.block-insert-trigger :deep(.el-icon) {
  font-size: 15px;
  font-weight: 700;
}

.block-style-menu {
  position: absolute;
  width: 288px;
  max-height: 410px;
  overflow: auto;
  padding: 8px;
  border: 1px solid rgba(203, 213, 225, 0.92);
  border-radius: 8px;
  background: #fff;
  box-shadow: 0 18px 44px rgba(15, 23, 42, 0.16);
  pointer-events: auto;
  scrollbar-color: #bdc8d6 transparent;
  scrollbar-width: thin;
}

.block-style-menu-group + .block-style-menu-group {
  margin-top: 6px;
  padding-top: 6px;
  border-top: 1px solid rgba(226, 232, 240, 0.88);
}

.block-style-menu-label {
  padding: 2px 5px 5px;
  color: #8a98aa;
  font-size: 11px;
  line-height: 18px;
}

.block-style-menu-items {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 3px;
}

.block-style-menu-item {
  min-width: 0;
  height: 32px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 7px;
  border: 0;
  border-radius: 5px;
  outline: none;
  background: transparent;
  color: #334155;
  cursor: pointer;
  font-size: 12px;
  text-align: left;
}

.block-style-menu-item:hover,
.block-style-menu-item:focus-visible {
  background: #f1f5f9;
  color: #1d4ed8;
}

.block-style-menu-key {
  width: 30px;
  min-width: 30px;
  overflow: hidden;
  color: #64748b;
  font-size: 11px;
  font-weight: 750;
  text-align: center;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.block-style-menu-item:hover .block-style-menu-key,
.block-style-menu-item:focus-visible .block-style-menu-key {
  color: #2563eb;
}

:global(body.knowledge-table-resizing),
:global(body.knowledge-table-resizing *) {
  cursor: col-resize !important;
  user-select: none !important;
}

.markdown-preview {
  color: var(--think-text-secondary);
  font-size: 13px;
  line-height: 1.65;
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
  border: 1px solid rgba(223, 231, 242, 0.86);
  border-radius: 6px;
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
  cursor: pointer;
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
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-collapse: separate;
  border-spacing: 0;
  border-radius: 6px;
  table-layout: auto;
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
  border: 1px solid rgba(35, 48, 70, 0.12);
  border-radius: 6px;
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

.rich-editor {
  position: absolute;
  inset: 0;
  box-sizing: border-box;
  width: 100%;
  height: auto;
  min-height: 0;
  max-height: none;
  flex: none;
  overflow-x: auto;
  overflow-y: scroll;
  padding: 26px clamp(20px, 4vw, 64px) 80px;
  outline: none;
  background: #fff;
  font-size: 15px;
  line-height: 1.75;
  scrollbar-color: #aebdd0 #f4f7fb;
  scrollbar-gutter: stable;
  scrollbar-width: thin;
}

.rich-editor::-webkit-scrollbar {
  width: 10px;
  height: 10px;
}

.rich-editor::-webkit-scrollbar-track {
  background: #f4f7fb;
}

.rich-editor::-webkit-scrollbar-thumb {
  border: 2px solid #f4f7fb;
  border-radius: 8px;
  background: #aebdd0;
}

.rich-editor::-webkit-scrollbar-thumb:hover {
  background: #8fa2ba;
}

.rich-editor:empty::before {
  content: attr(data-placeholder);
  color: var(--think-text-muted);
}

.rich-editor:focus {
  box-shadow: inset 0 0 0 2px rgba(76, 139, 245, 0.14);
}

.rich-editor :deep(p:first-child),
.rich-editor :deep(h1:first-child),
.rich-editor :deep(h2:first-child),
.rich-editor :deep(h3:first-child),
.rich-editor :deep(h4:first-child) {
  margin-top: 0;
}

.rich-editor :deep(td.is-active-table-cell),
.rich-editor :deep(th.is-active-table-cell) {
  position: relative;
  box-shadow: inset 0 0 0 2px #409eff;
}

.rich-editor :deep(td.is-selected-table-column),
.rich-editor :deep(th.is-selected-table-column) {
  background: rgba(64, 158, 255, 0.14);
}

.rich-editor :deep(td.is-selected-table-row),
.rich-editor :deep(th.is-selected-table-row) {
  background: rgba(99, 102, 241, 0.13);
}

.rich-editor :deep(img) {
  cursor: default;
}

@media (prefers-reduced-motion: reduce) {
  .table-edge-controls,
  .table-boundary-add,
  .table-boundary-dot,
  .table-boundary-add :deep(.el-icon),
  .table-selection-trigger,
  .table-selection-delete,
  .block-insert-trigger,
  .table-column-resize-handle::after {
    transition: none;
  }
}

@media (max-width: 820px) {
  .document-meta-form,
  .form-grid {
    grid-template-columns: 1fr;
  }

  .document-form :deep(.el-form-item) {
    align-items: stretch;
    flex-direction: column;
  }

  .document-form :deep(.el-form-item__label) {
    width: auto;
    min-width: 0;
    height: auto;
    padding: 0 0 6px;
    line-height: 1.3;
  }
}
</style>
