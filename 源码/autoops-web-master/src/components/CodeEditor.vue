<template>
  <div class="monaco-editor-wrapper" :style="{ height: height }">
    <div class="monaco-editor-container" ref="editorContainer"></div>
    <div
      v-if="placeholder && placeholderVisible"
      class="monaco-editor-placeholder"
      @mousedown.prevent="focus"
    >
      {{ placeholder }}
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import * as monaco from 'monaco-editor'

const props = defineProps({
  modelValue: {
    type: String,
    default: ''
  },
  language: {
    type: String,
    default: 'sql'
  },
  readonly: {
    type: Boolean,
    default: false
  },
  completionItems: {
    type: Array,
    default: () => []
  },
  height: {
    type: String,
    default: '400px'
  },
  theme: {
    type: String,
    default: 'vs-dark'
  },
  wordWrap: {
    type: String,
    default: 'off'
  },
  searchText: {
    type: String,
    default: ''
  },
  searchResults: {
    type: Array,
    default: () => []
  },
  currentSearchIndex: {
    type: Number,
    default: -1
  },
  placeholder: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['update:modelValue', 'execute'])

const editorContainer = ref(null)
let editor = null
let completionProvider = null
let searchDecorations = []
const placeholderVisible = ref(!props.modelValue)

const defineLogTheme = (name, palette) => {
  monaco.editor.defineTheme(name, {
    base: palette.base,
    inherit: true,
    rules: [
      { token: '', foreground: palette.foreground.replace('#', '') }
    ],
    colors: {
      'editor.background': palette.background,
      'editor.foreground': palette.foreground,
      'editorLineNumber.foreground': palette.lineNumber,
      'editorLineNumber.activeForeground': palette.accent,
      'editorCursor.foreground': palette.accent,
      'editor.selectionBackground': palette.selection,
      'editor.inactiveSelectionBackground': palette.inactiveSelection,
      'editor.lineHighlightBackground': palette.lineHighlight,
      'editorWhitespace.foreground': palette.whitespace
    }
  })
}

const ensureThemes = () => {
  monaco.editor.defineTheme('terminal-yellow', {
    base: 'vs-dark',
    inherit: true,
    rules: [
      { token: '', foreground: 'C9A227' }
    ],
    colors: {
      'editor.background': '#000000',
      'editor.foreground': '#C9A227',
      'editorLineNumber.foreground': '#6F5A16',
      'editorCursor.foreground': '#C9A227',
      'editor.selectionBackground': '#3D320F',
      'editor.inactiveSelectionBackground': '#241D08'
    }
  })

  defineLogTheme('aiops-log-light', {
    base: 'vs',
    background: '#f8fafc',
    foreground: '#334155',
    lineNumber: '#94a3b8',
    accent: '#2563eb',
    selection: '#dbeafe',
    inactiveSelection: '#eaf2ff',
    lineHighlight: '#f1f5f9',
    whitespace: '#d7e0ea'
  })

  defineLogTheme('aiops-log-dark', {
    base: 'vs-dark',
    background: '#111827',
    foreground: '#d1d5db',
    lineNumber: '#64748b',
    accent: '#60a5fa',
    selection: '#1e3a5f',
    inactiveSelection: '#1f2937',
    lineHighlight: '#172033',
    whitespace: '#374151'
  })

  defineLogTheme('aiops-log-agnoster', {
    base: 'vs-dark',
    background: '#1f2430',
    foreground: '#e6e6e6',
    lineNumber: '#65737e',
    accent: '#5ccfe6',
    selection: '#34455a',
    inactiveSelection: '#2a3242',
    lineHighlight: '#292f3d',
    whitespace: '#3f4a5a'
  })

  defineLogTheme('aiops-log-robbyrussell', {
    base: 'vs-dark',
    background: '#272822',
    foreground: '#f8f8f2',
    lineNumber: '#75715e',
    accent: '#a6e22e',
    selection: '#49483e',
    inactiveSelection: '#3e3d32',
    lineHighlight: '#30312b',
    whitespace: '#464741'
  })

  defineLogTheme('aiops-log-solarized', {
    base: 'vs-dark',
    background: '#002b36',
    foreground: '#93a1a1',
    lineNumber: '#586e75',
    accent: '#2aa198',
    selection: '#174a55',
    inactiveSelection: '#073642',
    lineHighlight: '#073642',
    whitespace: '#34515a'
  })

  monaco.editor.defineTheme('aiops-sql-light', {
    base: 'vs',
    inherit: true,
    rules: [
      { token: 'keyword', foreground: '1f6fff', fontStyle: 'bold' },
      { token: 'operator.sql', foreground: '5f6b7c' },
      { token: 'number', foreground: '8a5cf6' },
      { token: 'string', foreground: '0f9f6e' },
      { token: 'delimiter.identifier.sql', foreground: 'b7791f' },
      { token: 'identifier.quote.sql', foreground: 'b7791f' },
      { token: 'comment', foreground: '8d97a8', fontStyle: 'italic' }
    ],
    colors: {
      'editor.background': '#fffdf4',
      'editor.foreground': '#172133',
      'editorLineNumber.foreground': '#8d97a8',
      'editorLineNumber.activeForeground': '#1f6fff',
      'editorCursor.foreground': '#1f6fff',
      'editor.selectionBackground': '#dbeafe',
      'editor.inactiveSelectionBackground': '#edf2fd',
      'editor.lineHighlightBackground': '#fff7df',
      'editorLineNumber.border': '#dfe7f2'
    }
  })
}

const updateSearchDecorations = () => {
  if (!editor) {
    return
  }

  const hasSearch = props.searchText && props.searchResults.length > 0
  if (!hasSearch) {
    searchDecorations = editor.deltaDecorations(searchDecorations, [])
    return
  }

  const decorations = props.searchResults.map((result, index) => {
    const startLineNumber = Number(result.lineIndex || 0) + 1
    const startColumn = Number(result.startIndex || 0) + 1
    const endColumn = Number(result.endIndex || 0) + 1
    const isCurrent = index === props.currentSearchIndex

    return {
      range: new monaco.Range(startLineNumber, startColumn, startLineNumber, endColumn),
      options: {
        inlineClassName: isCurrent ? 'editor-search-current-match' : 'editor-search-match'
      }
    }
  })

  searchDecorations = editor.deltaDecorations(searchDecorations, decorations)
}

const revealCurrentSearchResult = () => {
  if (!editor || props.currentSearchIndex < 0 || props.currentSearchIndex >= props.searchResults.length) {
    return
  }

  const currentResult = props.searchResults[props.currentSearchIndex]
  const lineNumber = Number(currentResult.lineIndex || 0) + 1
  const startColumn = Number(currentResult.startIndex || 0) + 1
  const endColumn = Number(currentResult.endIndex || 0) + 1

  editor.setSelection(new monaco.Range(lineNumber, startColumn, lineNumber, endColumn))
  editor.revealLineInCenter(lineNumber)
}

// --- Auto-completion ---
const setupAutoCompletion = (items) => {
  // If a provider already exists, dispose of it first
  if (completionProvider) {
    completionProvider.dispose()
  }

  if (!items || items.length === 0) {
    return
  }

  completionProvider = monaco.languages.registerCompletionItemProvider(props.language, {
    provideCompletionItems: () => {
      const suggestions = []
      const addSuggestion = item => {
        if (!item) return
        if (typeof item === 'string') {
          suggestions.push({
            label: item,
            kind: monaco.languages.CompletionItemKind.Keyword,
            insertText: item
          })
          return
        }

        const label = item.label || item.name || item.value || item.keyword || item.tableName || item.columnName || item.database
        if (!label) return
        const rawKind = String(item.kind || item.type || '').toLowerCase()
        const kindMap = {
          keyword: monaco.languages.CompletionItemKind.Keyword,
          database: monaco.languages.CompletionItemKind.Module,
          schema: monaco.languages.CompletionItemKind.Module,
          table: monaco.languages.CompletionItemKind.Folder,
          view: monaco.languages.CompletionItemKind.Folder,
          column: monaco.languages.CompletionItemKind.Field,
          field: monaco.languages.CompletionItemKind.Field,
          function: monaco.languages.CompletionItemKind.Function
        }
        suggestions.push({
          label,
          kind: kindMap[rawKind] || monaco.languages.CompletionItemKind.Text,
          insertText: item.insertText || item.value || label,
          detail: item.detail || item.description || item.type || ''
        })
      }

      // Add tables and columns to suggestions
      items.forEach(table => {
        if (typeof table === 'string' || table.label || table.name || table.value || table.keyword) {
          addSuggestion(table)
          return
        }

        // Suggest table name
        addSuggestion({ label: table.tableName, kind: table.type || 'table', detail: 'Table' })

        // Suggest columns for that table
        ;(table.columns || []).forEach(column => {
          addSuggestion({
            label: column.columnName || column.name,
            kind: 'column',
            detail: `Column in ${table.tableName} (${column.columnType || column.type || ''})`
          })
        })
      })
      
      if (props.language === 'sql') {
        const keywords = ['SELECT', 'FROM', 'WHERE', 'INSERT', 'INTO', 'VALUES', 'UPDATE', 'SET', 'DELETE', 'GROUP BY', 'ORDER BY', 'AND', 'OR', 'LIMIT', 'OFFSET', 'COUNT', 'AVG', 'SUM', 'MIN', 'MAX', 'JOIN', 'LEFT JOIN', 'RIGHT JOIN', 'INNER JOIN', 'ON']
        keywords.forEach(keyword => {
          suggestions.push({
            label: keyword,
            kind: monaco.languages.CompletionItemKind.Keyword,
            insertText: keyword
          })
        })
      }

      return {
        suggestions: suggestions
      }
    }
  })
}

watch(() => props.completionItems, (newItems) => {
  if (editor) {
    setupAutoCompletion(newItems)
  }
}, { deep: true, immediate: true })

// --- Editor Lifecycle ---
onMounted(() => {
  if (editorContainer.value) {
    ensureThemes()

    editor = monaco.editor.create(editorContainer.value, {
      value: props.modelValue,
      language: props.language,
      readOnly: props.readonly,
      theme: props.theme,
      automaticLayout: true,
      fontSize: 13,
      lineHeight: 22,
      minimap: {
        enabled: false // Disable minimap
      },
      scrollBeyondLastLine: false,
      wordWrap: props.wordWrap,
    })

    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter, () => {
      emit('execute')
    })

    // Sync content back to parent
    editor.onDidChangeModelContent(() => {
      const currentValue = editor.getValue()
      placeholderVisible.value = !currentValue
      if (currentValue !== props.modelValue) {
        emit('update:modelValue', currentValue)
      }
    })

    // Setup initial completion items if they exist
    if (props.completionItems.length > 0) {
      setupAutoCompletion(props.completionItems)
    }

    updateSearchDecorations()
    revealCurrentSearchResult()
  }
})

onUnmounted(() => {
  if (editor) {
    editor.dispose()
  }
  if (completionProvider) {
    completionProvider.dispose()
  }
})

// Watch for external changes to modelValue
watch(() => props.modelValue, (newValue) => {
  if (editor && editor.getValue() !== newValue) {
    editor.setValue(newValue)
  }
  placeholderVisible.value = !newValue
})

watch(() => props.language, (newVal) => {
  if (!editor) return
  const model = editor.getModel()
  if (model) monaco.editor.setModelLanguage(model, newVal)
  setupAutoCompletion(props.completionItems)
})

// Watch for external changes to readonly state
watch(() => props.readonly, (newVal) => {
  if (editor) {
    editor.updateOptions({ readOnly: newVal })
  }
})

watch(() => props.theme, (newVal) => {
  if (editor) {
    monaco.editor.setTheme(newVal)
  }
})

watch(() => props.wordWrap, (newVal) => {
  if (editor) {
    editor.updateOptions({ wordWrap: newVal })
  }
})

watch(
  () => [props.searchText, props.searchResults, props.currentSearchIndex],
  () => {
    updateSearchDecorations()
    revealCurrentSearchResult()
  },
  { deep: true }
)

// Expose focus method
const focus = () => {
  if (editor) {
    editor.focus()
  }
}

const scrollToBottom = () => {
  if (!editor) {
    return
  }

  const model = editor.getModel()
  const lineCount = model?.getLineCount() || 1
  editor.revealLine(lineCount)

  // 再直接设置一次滚动位置，避免某些情况下 revealLine 未贴底
  const scrollHeight = editor.getScrollHeight()
  editor.setScrollTop(scrollHeight)
}

const scrollToLine = (lineNumber) => {
  if (!editor) {
    return
  }

  const model = editor.getModel()
  const maxLine = model?.getLineCount() || 1
  const targetLine = Math.min(Math.max(1, lineNumber), maxLine)
  editor.revealLineInCenter(targetLine)
}

defineExpose({ focus, scrollToBottom, scrollToLine })

</script>

<style scoped>
.monaco-editor-wrapper {
  position: relative;
  width: 100%;
}

.monaco-editor-container {
  width: 100%;
  height: 100%;
  min-height: 200px; /* Ensure a minimum height */
  border: 1px solid #ccc;
  border-radius: 4px;
  overflow: hidden;
}

.monaco-editor-placeholder {
  position: absolute;
  top: 13px;
  left: 62px;
  right: 20px;
  z-index: 2;
  color: #8d97a8;
  font-size: 13px;
  line-height: 22px;
  pointer-events: auto;
  white-space: pre-wrap;
}
</style>

<style>
.monaco-editor .editor-search-match {
  background: rgba(135, 206, 250, 0.42) !important;
  border-bottom: 1px solid rgba(135, 206, 250, 0.9) !important;
  color: #0b0b0b !important;
}

.monaco-editor .editor-search-current-match {
  background: rgba(135, 206, 250, 0.72) !important;
  border-bottom: 1px solid #87cefa !important;
  color: #000000 !important;
}
</style>
