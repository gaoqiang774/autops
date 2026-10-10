export const escapeHtml = value => String(value || '')
  .replace(/&/g, '&amp;')
  .replace(/</g, '&lt;')
  .replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;')
  .replace(/'/g, '&#39;')

export const HIGHLIGHT_BLOCK_ICONS = ['💡', '📌', '🏅', '🌅', '🧭', '🛠️', '🔔', '✨']

export const getRandomHighlightBlockIcon = () => HIGHLIGHT_BLOCK_ICONS[
  Math.floor(Math.random() * HIGHLIGHT_BLOCK_ICONS.length)
]

const getStableHighlightBlockIcon = value => {
  const hash = Array.from(String(value || '')).reduce((result, char) => ((result * 31) + char.codePointAt(0)) | 0, 0)
  return HIGHLIGHT_BLOCK_ICONS[Math.abs(hash) % HIGHLIGHT_BLOCK_ICONS.length]
}

export const readPersistedTableWidths = table => {
  const columnCount = table?.rows?.[0]?.cells?.length || 0
  const widths = String(table?.dataset?.columnWidths || '')
    .split(',')
    .map(value => Number.parseInt(value, 10))
    .filter(value => Number.isFinite(value) && value >= 80)
  return widths.length === columnCount ? widths : null
}

const escapeMarkdownCell = value => String(value || '').replace(/\|/g, '\\|')

const nodeText = node => String(node?.textContent || '').replace(/\u00a0/g, ' ').trim()
const normalizeRichTextColor = value => /^#[0-9a-f]{6}$/i.test(String(value || ''))
  ? String(value).toLowerCase()
  : ''

const inlineNodeToMarkdown = node => {
  if (!node) return ''
  if (node.nodeType === Node.TEXT_NODE) return String(node.textContent || '')
    .replace(/\u00a0/g, ' ')
    .replace(/\u200b/g, '')
  if (node.nodeType !== Node.ELEMENT_NODE) return ''

  const tag = node.tagName.toLowerCase()
  if (tag === 'br') return '\n'
  if (tag === 'strong' || tag === 'b') return `**${childrenInlineToMarkdown(node)}**`
  if (tag === 'em' || tag === 'i') return `*${childrenInlineToMarkdown(node)}*`
  if (tag === 'del' || tag === 's' || tag === 'strike') return `~~${childrenInlineToMarkdown(node)}~~`
  if (tag === 'code') return `\`${nodeText(node) || '代码内容'}\``
  if (tag === 'a') return `[${childrenInlineToMarkdown(node) || '显示文本'}](${node.getAttribute('href') || 'URL链接'})`
  if (tag === 'img') return `![${node.getAttribute('alt') || '图片描述'}](${node.getAttribute('src') || '图片URL'})`
  if (tag === 'span' && node.classList.contains('text-color-mark')) {
    const textColor = normalizeRichTextColor(node.dataset.textColor)
    const backgroundColor = normalizeRichTextColor(node.dataset.backgroundColor)
    let content = childrenInlineToMarkdown(node)
    if (!content) return ''
    if (textColor) content = `{{fg:${textColor}}}${content}{{/fg}}`
    if (backgroundColor) content = `{{bg:${backgroundColor}}}${content}{{/bg}}`
    return content
  }
  return childrenInlineToMarkdown(node)
}

const childrenInlineToMarkdown = node => Array.from(node?.childNodes || []).map(inlineNodeToMarkdown).join('')

const codeNodeToText = node => {
  if (!node) return ''
  if (node.nodeType === Node.TEXT_NODE) return node.textContent || ''
  if (node.nodeType !== Node.ELEMENT_NODE) return ''
  if (node.tagName.toLowerCase() === 'br') return '\n'

  const children = Array.from(node.childNodes || [])
  return children.reduce((text, child, index) => {
    const tag = child.nodeType === Node.ELEMENT_NODE ? child.tagName.toLowerCase() : ''
    const isLineBlock = tag === 'div' || tag === 'p'
    let nextText = text
    if (isLineBlock && nextText && !nextText.endsWith('\n')) nextText += '\n'
    nextText += codeNodeToText(child)
    if (isLineBlock && index < children.length - 1 && !nextText.endsWith('\n')) nextText += '\n'
    return nextText
  }, '')
}

const codeBlockToText = node => codeNodeToText(node)
  .replace(/\u00a0/g, ' ')
  .replace(/\r\n?/g, '\n')
  .replace(/\n$/, '')

const highlightBlockToMarkdown = node => Array.from(node?.childNodes || []).reduce((markdown, child, index, children) => {
  const tag = child.nodeType === Node.ELEMENT_NODE ? child.tagName.toLowerCase() : ''
  const isLineBlock = tag === 'div' || tag === 'p' || tag === 'ol' || tag === 'ul'
  let nextMarkdown = markdown
  if (isLineBlock && nextMarkdown && !nextMarkdown.endsWith('\n')) nextMarkdown += '\n'
  if (tag === 'ol' || tag === 'ul') {
    const ordered = tag === 'ol'
    nextMarkdown += Array.from(child.children)
      .filter(item => item.tagName?.toLowerCase() === 'li')
      .map((item, itemIndex) => {
        const checkbox = item.querySelector?.('input[type="checkbox"]')
        if (checkbox) return `- [${checkbox.checked ? 'x' : ' '}] ${nodeText(item)}`
        return `${ordered ? `${itemIndex + 1}.` : '-'} ${childrenInlineToMarkdown(item).trim()}`
      })
      .join('\n')
  } else {
    nextMarkdown += isLineBlock ? childrenInlineToMarkdown(child) : inlineNodeToMarkdown(child)
  }
  if (isLineBlock && index < children.length - 1 && !nextMarkdown.endsWith('\n')) nextMarkdown += '\n'
  return nextMarkdown
}, '').replace(/\r\n?/g, '\n').replace(/\n$/, '')

const blockNodeToMarkdown = node => {
  if (!node) return ''
  if (node.nodeType === Node.TEXT_NODE) return node.textContent?.trim() || ''
  if (node.nodeType !== Node.ELEMENT_NODE) return ''

  const tag = node.tagName.toLowerCase()
  if (tag === 'h1') return `# ${childrenInlineToMarkdown(node).trim()}`
  if (tag === 'h2') return `## ${childrenInlineToMarkdown(node).trim()}`
  if (tag === 'h3') return `### ${childrenInlineToMarkdown(node).trim()}`
  if (tag === 'h4') return `#### ${childrenInlineToMarkdown(node).trim()}`
  if (tag === 'blockquote') return childrenInlineToMarkdown(node).split(/\n/).map(line => `> ${line}`).join('\n')
  if (tag === 'hr') return '---'
  if (tag === 'pre') {
    const code = node.querySelector('code')
    const lang = code?.dataset?.lang || ''
    return `\`\`\`${lang}\n${codeBlockToText(code || node)}\n\`\`\``
  }
  if (tag === 'div' && node.classList.contains('highlight-block')) {
    const icon = node.dataset.highlightIcon
    return `:::highlight${icon ? ` icon=${icon}` : ''}\n${highlightBlockToMarkdown(node)}\n:::`
  }
  if (tag === 'ul' || tag === 'ol') {
    const ordered = tag === 'ol'
    return Array.from(node.children)
      .filter(item => item.tagName?.toLowerCase() === 'li')
      .map((item, itemIndex) => {
        const checkbox = item.querySelector?.('input[type="checkbox"]')
        if (checkbox) return `- [${checkbox.checked ? 'x' : ' '}] ${nodeText(item)}`
        return `${ordered ? `${itemIndex + 1}.` : '-'} ${childrenInlineToMarkdown(item).trim()}`
      })
      .join('\n')
  }
  if (tag === 'table') {
    const rows = Array.from(node.querySelectorAll('tr'))
    if (!rows.length) return ''
    const headerCells = Array.from(rows[0].children).map(cell => escapeMarkdownCell(childrenInlineToMarkdown(cell).trim() || '列'))
    const separator = headerCells.map(() => ':---')
    const bodyRows = rows.slice(1).map(row => {
      const cells = Array.from(row.children).map(cell => escapeMarkdownCell(childrenInlineToMarkdown(cell).trim()))
      return `| ${cells.join(' | ')} |`
    })
    const widths = readPersistedTableWidths(node)
    const widthMetadata = widths ? `<!-- table-widths:${widths.join(',')} -->` : ''
    return [widthMetadata, `| ${headerCells.join(' | ')} |`, `| ${separator.join(' | ')} |`, ...bodyRows]
      .filter(Boolean)
      .join('\n')
  }
  if (tag === 'div' || tag === 'p') return childrenInlineToMarkdown(node).trim()
  return childrenInlineToMarkdown(node).trim()
}

export const htmlToMarkdown = root => Array.from(root?.childNodes || [])
  .map(node => blockNodeToMarkdown(node))
  .filter(Boolean)
  .join('\n\n')

const renderInline = value => {
  const styleTokens = []
  const styleStack = []
  const tokenized = String(value || '').replace(
    /\{\{(fg|bg):(#[0-9a-f]{6})\}\}|\{\{\/(fg|bg)\}\}/gi,
    (marker, openType, color, closeType) => {
      if (openType) {
        const normalizedColor = normalizeRichTextColor(color)
        if (!normalizedColor) return marker
        styleStack.push(openType.toLowerCase())
        const colorProperty = openType.toLowerCase() === 'fg' ? 'color' : 'background-color'
        const dataAttribute = openType.toLowerCase() === 'fg' ? 'data-text-color' : 'data-background-color'
        styleTokens.push(`<span class="text-color-mark" ${dataAttribute}="${normalizedColor}" style="${colorProperty}:${normalizedColor}">`)
        return `THINKTANKSTYLE${styleTokens.length - 1}TOKEN`
      }
      if (styleStack[styleStack.length - 1] !== closeType.toLowerCase()) return marker
      styleStack.pop()
      styleTokens.push('</span>')
      return `THINKTANKSTYLE${styleTokens.length - 1}TOKEN`
    }
  )

  return escapeHtml(tokenized)
    .replace(/!\[([^\]]*)\]\(([^)]+)\)/g, '<img src="$2" alt="$1" />')
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener noreferrer">$1</a>')
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/~~([^~]+)~~/g, '<del>$1</del>')
    .replace(/(^|[^*])\*([^*]+)\*/g, '$1<em>$2</em>')
    .replace(/THINKTANKSTYLE(\d+)TOKEN/g, (_, index) => styleTokens[Number(index)] || '')
}

const isTableSeparator = value => /^\s*\|?\s*:?-{3,}:?\s*(\|\s*:?-{3,}:?\s*)+\|?\s*$/.test(value)

const parseTableRow = value => {
  const normalized = value.trim().replace(/^\|/, '').replace(/\|$/, '')
  return normalized.split('|').map(cell => cell.trim())
}

const renderTable = (rows, columnWidths = []) => {
  if (rows.length < 2 || !isTableSeparator(rows[1])) return null
  const headers = parseTableRow(rows[0])
  const widths = columnWidths
    .map(value => Number.parseInt(value, 10))
    .filter(value => Number.isFinite(value) && value >= 80)
  const hasColumnWidths = widths.length === headers.length
  const aligns = parseTableRow(rows[1]).map(cell => {
    const left = cell.startsWith(':')
    const right = cell.endsWith(':')
    if (left && right) return 'center'
    if (right) return 'right'
    return 'left'
  })
  const bodyRows = rows.slice(2).map(parseTableRow)
  const th = headers.map((cell, index) => `<th style="text-align:${aligns[index] || 'left'}">${renderInline(cell)}</th>`).join('')
  const trs = bodyRows.map(row => {
    const tds = headers.map((_, index) => `<td style="text-align:${aligns[index] || 'left'}">${renderInline(row[index] || '')}</td>`).join('')
    return `<tr>${tds}</tr>`
  }).join('')
  const tableWidth = widths.reduce((sum, width) => sum + width, 0)
  const widthAttributes = hasColumnWidths
    ? ` data-column-widths="${widths.join(',')}" style="width:${tableWidth}px;min-width:${tableWidth}px;max-width:none;table-layout:fixed"`
    : ''
  const colgroup = hasColumnWidths
    ? `<colgroup>${widths.map(width => `<col style="width:${width}px">`).join('')}</colgroup>`
    : ''
  return `<table${widthAttributes}>${colgroup}<thead><tr>${th}</tr></thead><tbody>${trs}</tbody></table>`
}

export const renderMarkdown = (source, options = {}) => {
  const lines = String(source || '').split(/\r?\n/)
  const html = []
  const interactiveTasks = Boolean(options.interactiveTasks)
  let inCode = false
  let inHighlight = false
  let highlightLineCount = 0
  let highlightListType = ''
  let listType = ''
  let tableRows = []
  let pendingTableWidths = []

  const closeList = () => {
    if (listType) {
      html.push(`</${listType === 'task' ? 'ul' : listType}>`)
      listType = ''
    }
  }

  const closeHighlightList = () => {
    if (highlightListType) {
      html.push(`</${highlightListType === 'task' ? 'ul' : highlightListType}>`)
      highlightListType = ''
    }
  }

  const flushTable = () => {
    if (!tableRows.length) return
    const table = renderTable(tableRows, pendingTableWidths)
    if (table) {
      closeList()
      html.push(table)
    } else {
      tableRows.forEach(row => html.push(`<p>${renderInline(row.trim())}</p>`))
    }
    tableRows = []
    pendingTableWidths = []
  }

  lines.forEach((line, sourceIndex) => {
    const trimmed = line.trim()

    if (inHighlight) {
      if (trimmed === ':::') {
        closeHighlightList()
        if (!highlightLineCount) html.push('<br>')
        html.push('</div>')
        inHighlight = false
        highlightLineCount = 0
        return
      }

      const highlightTask = trimmed.match(/^[-*]\s+\[( |x|X)\]\s+(.+)$/)
      if (highlightTask) {
        if (highlightListType !== 'task') {
          closeHighlightList()
          highlightListType = 'task'
          html.push('<ul class="task-list">')
        }
        const checked = highlightTask[1].toLowerCase() === 'x' ? ' checked' : ''
        const disabled = interactiveTasks ? '' : ' disabled'
        const completedClass = checked ? ' is-task-complete' : ''
        const taskLabel = checked ? '已完成任务' : '未完成任务'
        html.push(`<li class="task-list-item${completedClass}"><input type="checkbox" contenteditable="false" aria-label="${taskLabel}"${disabled}${checked} />${renderInline(highlightTask[2])}</li>`)
        highlightLineCount += 1
        return
      }

      const highlightOrdered = trimmed.match(/^\d+\.\s+(.+)$/)
      if (highlightOrdered) {
        if (highlightListType !== 'ol') {
          closeHighlightList()
          highlightListType = 'ol'
          html.push('<ol>')
        }
        html.push(`<li>${renderInline(highlightOrdered[1])}</li>`)
        highlightLineCount += 1
        return
      }

      const highlightUnordered = trimmed.match(/^[-*]\s+(.+)$/)
      if (highlightUnordered) {
        if (highlightListType !== 'ul') {
          closeHighlightList()
          highlightListType = 'ul'
          html.push('<ul>')
        }
        html.push(`<li>${renderInline(highlightUnordered[1])}</li>`)
        highlightLineCount += 1
        return
      }

      closeHighlightList()
      if (highlightLineCount) html.push('<br>')
      html.push(line ? renderInline(line) : '<br>')
      highlightLineCount += 1
      return
    }

    const fence = line.match(/^```(\w*)/)
    if (fence) {
      flushTable()
      if (inCode) {
        html.push('</code></pre>')
        inCode = false
      } else {
        closeList()
        html.push(`<pre><code data-lang="${escapeHtml(fence[1] || '')}">`)
        inCode = true
      }
      return
    }

    if (inCode) {
      html.push(`${escapeHtml(line)}\n`)
      return
    }

    const highlightStart = trimmed.match(/^:::highlight(?:\s+icon=(\S+))?$/)
    if (highlightStart) {
      flushTable()
      closeList()
      const highlightIcon = highlightStart[1] || getStableHighlightBlockIcon(`${sourceIndex}:${line}`)
      html.push(`<div class="highlight-block" data-highlight-icon="${escapeHtml(highlightIcon)}">`)
      inHighlight = true
      highlightLineCount = 0
      highlightListType = ''
      return
    }

    if (!trimmed) {
      flushTable()
      closeList()
      return
    }

    const tableWidthMetadata = trimmed.match(/^<!--\s*table-widths:([\d,\s]+)\s*-->$/)
    if (tableWidthMetadata) {
      flushTable()
      pendingTableWidths = tableWidthMetadata[1].split(',').map(value => Number.parseInt(value.trim(), 10))
      return
    }

    if (/^\|.+\|$/.test(trimmed)) {
      tableRows.push(trimmed)
      return
    }

    flushTable()

    if (/^(-{3,}|\*{3,})$/.test(trimmed)) {
      closeList()
      html.push('<hr />')
      return
    }

    const heading = trimmed.match(/^(#{1,4})\s+(.+)$/)
    if (heading) {
      closeList()
      const level = heading[1].length
      html.push(`<h${level} data-heading-id="knowledge-heading-${sourceIndex}">${renderInline(heading[2])}</h${level}>`)
      return
    }

    const task = trimmed.match(/^[-*]\s+\[( |x|X)\]\s+(.+)$/)
    if (task) {
      if (listType !== 'task') {
        closeList()
        listType = 'task'
        html.push('<ul class="task-list">')
      }
      const checked = task[1].toLowerCase() === 'x' ? ' checked' : ''
      const disabled = interactiveTasks ? '' : ' disabled'
      const completedClass = checked ? ' is-task-complete' : ''
      const taskLabel = checked ? '已完成任务' : '未完成任务'
      html.push(`<li class="task-list-item${completedClass}"><input type="checkbox" contenteditable="false" aria-label="${taskLabel}"${disabled}${checked} />${renderInline(task[2])}</li>`)
      return
    }

    const ordered = trimmed.match(/^\d+\.\s+(.+)$/)
    if (ordered) {
      if (listType !== 'ol') {
        closeList()
        listType = 'ol'
        html.push('<ol>')
      }
      html.push(`<li>${renderInline(ordered[1])}</li>`)
      return
    }

    const unordered = trimmed.match(/^[-*]\s+(.+)$/)
    if (unordered) {
      if (listType !== 'ul') {
        closeList()
        listType = 'ul'
        html.push('<ul>')
      }
      html.push(`<li>${renderInline(unordered[1])}</li>`)
      return
    }

    const quote = trimmed.match(/^>\s+(.+)$/)
    if (quote) {
      closeList()
      html.push(`<blockquote>${renderInline(quote[1])}</blockquote>`)
      return
    }

    closeList()
    html.push(`<p>${renderInline(trimmed)}</p>`)
  })

  flushTable()
  closeList()
  if (inCode) html.push('</code></pre>')
  if (inHighlight) {
    closeHighlightList()
    html.push('</div>')
  }
  return html.join('')
}
