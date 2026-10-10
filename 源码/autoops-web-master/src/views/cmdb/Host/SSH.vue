<template>
  <div class="ssh-container">
    <div class="ssh-layout">
      <!-- 左侧主机树区域 -->
      <div class="tree-section" :style="{ width: treeWidth + 'px' }">
        <el-card shadow="never" class="group-card">
          <template #header>
            <div class="card-header">
              <span class="header-title">主机列表</span>
            </div>
          </template>

          <!-- 搜索框和刷新按钮 -->
          <div class="search-container">
            <el-input
              v-model="groupSearchText"
              placeholder="输入主机名/IP..."
              clearable
              size="small"
              class="search-input"
              @input="handleGroupSearch"
            >
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>
            <el-button
              size="small"
              :icon="refreshButtonIcon"
              :class="['refresh-btn', refreshButtonClass]"
              @click="handleRefresh"
              title="刷新主机列表"
            />
          </div>

          <!-- 主机树 -->
          <el-tree
            ref="groupTree"
            :data="displayGroupList"
            :props="defaultProps"
            node-key="id"
            :expanded-keys="expandedKeys"
            :default-expanded-keys="expandedKeys"
            :highlight-current="true"
            @node-click="handleGroupClick"
            @node-expand="handleNodeExpand"
            @node-collapse="handleNodeCollapse"
            class="group-tree"
          >
            <template #default="{ node, data }">
              <span :class="{ 'business-group': !data.parentId, 'host-node': data.isHost }">
                <template v-if="data.isHost">
                  <!-- 主机在线图标 -->
                  <span class="tree-icon host-icon">
                    <img :src="LinuxHostIcon" alt="Linux" />
                  </span>
                </template>
                <template v-else-if="data.parentId">
                  <!-- 子分组：根据展开状态显示打开/关闭文件夹 -->
                  <span class="tree-icon folder-icon">
                    <img v-if="node.expanded" :src="FolderOpenIcon" alt="打开文件夹" />
                    <img v-else :src="FolderCloseIcon" alt="关闭文件夹" />
                  </span>
                </template>
                <template v-else>
                  <!-- 业务组：根据展开状态显示打开/关闭文件夹 -->
                  <span class="tree-icon folder-icon">
                    <img v-if="node.expanded" :src="FolderOpenIcon" alt="打开文件夹" />
                    <img v-else :src="FolderCloseIcon" alt="关闭文件夹" />
                  </span>
                </template>
                <span class="node-label">{{ node.label }}</span>
                <span v-if="!data.isHost && data.hostCount !== undefined" class="host-count">({{ data.hostCount }})</span>
              </span>
            </template>
          </el-tree>
          <el-empty
            v-if="displayGroupList.length === 0"
            description="无数据"
            :image-size="72"
            class="tree-empty"
          />
        </el-card>
      </div>

      <!-- 拖拽分隔条 -->
      <div
        class="resizer"
        @mousedown="startResize"
        @dblclick="resetTreeWidth"
        title="拖动调整宽度，双击恢复默认"
      >
        <div class="resizer-line"></div>
      </div>

      <!-- 右侧终端标签页区域 -->
      <div class="terminal-section">
        <!-- 空状态 - 当没有打开任何终端时显示 -->
        <div v-if="terminalTabs.length === 0" class="welcome-placeholder">
          <div class="welcome-icon">
            <svg viewBox="0 0 200 150" xmlns="http://www.w3.org/2000/svg">
              <!-- 显示器 -->
              <rect x="20" y="20" width="160" height="90" fill="none" stroke="#409eff" stroke-width="3" rx="5"/>
              <!-- 屏幕内容 -->
              <rect x="30" y="30" width="140" height="70" fill="#2c3e50"/>
              <!-- 终端提示符 -->
              <text x="40" y="55" font-family="monospace" font-size="12" fill="#67c23a">$</text>
              <line x1="50" y1="48" x2="70" y2="48" stroke="#67c23a" stroke-width="2"/>
              <!-- 底座 -->
              <line x1="80" y1="110" x2="120" y2="110" stroke="#409eff" stroke-width="3"/>
              <line x1="100" y1="110" x2="100" y2="125" stroke="#409eff" stroke-width="3"/>
              <rect x="70" y="125" width="60" height="5" fill="#409eff" rx="2"/>
            </svg>
          </div>
          <div class="welcome-text">
            <h1>AutoOps</h1>
            <p>双击左侧主机节点开启终端会话</p>
          </div>
        </div>

        <!-- 终端标签页 -->
        <div v-else class="terminal-tabs-shell">
          <el-dropdown
            trigger="click"
            class="terminal-close-actions"
            @command="handleCloseCommand"
          >
            <el-button size="small" :icon="MoreFilled" circle title="关闭窗口" />
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="all" :icon="Close">关闭所有窗口</el-dropdown-item>
                <el-dropdown-item command="left" :icon="DArrowLeft" :disabled="!hasLeftTabs">关闭左侧窗口</el-dropdown-item>
                <el-dropdown-item command="right" :icon="DArrowRight" :disabled="!hasRightTabs">关闭右侧窗口</el-dropdown-item>
                <el-dropdown-item command="others" :icon="CircleClose" :disabled="!hasOtherTabs">关闭其它窗口</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>

          <el-tabs
            v-model="activeTabName"
            type="card"
            closable
            @tab-remove="handleTabRemove"
            @tab-click="handleTabClick"
            class="terminal-tabs"
          >
            <el-tab-pane
              v-for="tab in terminalTabs"
              :key="tab.name"
              :label="tab.title"
              :name="tab.name"
            >
              <!-- 每个标签页对应一个终端实例 -->
              <terminal
                v-if="tab.visible"
                :ref="'terminal_' + tab.name"
                :current-host="tab.host"
                :tab-name="tab.name"
                @terminal-closed="handleTerminalClosed"
              />
            </el-tab-pane>
          </el-tabs>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, nextTick, onUnmounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Search,
  Refresh,
  Check,
  MoreFilled,
  Close,
  DArrowLeft,
  DArrowRight,
  CircleClose
} from '@element-plus/icons-vue'
import { useRoute } from 'vue-router'
import Terminal from './Terminal.vue'
import cmdbApi from '@/api/cmdb'
import LinuxHostIcon from '@/assets/image/linux.svg'
import FolderOpenIcon from '@/assets/image/打开文件夹.svg'
import FolderCloseIcon from '@/assets/image/关闭文件夹.svg'

// 数据状态
const route = useRoute()

const groupSearchText = ref('')
const expandedKeys = ref([])
const groupList = ref([])
const displayGroupList = ref([])
const groupTree = ref(null)
const lastAutoOpenSignature = ref('')
const openingHostIds = new Set()
const refreshStatus = ref('idle')

const defaultProps = {
  children: 'children',
  label: 'name'
}

const ROOT_GROUP_NAMES = new Set(['业务组'])
const DEFAULT_GROUP_NAMES = new Set(['默认业务组', '默认分组'])

const isRootGroupNode = group => ROOT_GROUP_NAMES.has(String(group?.name || '').trim())

const isDefaultGroupNode = group =>
  group?.isDefault === true ||
  group?.isDefault === 1 ||
  group?.isDefault === '1' ||
  DEFAULT_GROUP_NAMES.has(String(group?.name || '').trim())

const getHostType = host => host?.hostType ?? host?.host_type ?? host?.deviceType ?? host?.device_type

const isWindowsHost = host => {
  const hostType = getHostType(host)
  const numericHostType = Number(hostType)

  if (Number.isFinite(numericHostType)) {
    return numericHostType === 2
  }

  return String(hostType || '').trim().toLowerCase() === 'windows'
}

const createHostNode = host => ({
  id: `host_${host.id}`,
  name: host.hostName,
  isHost: true,
  hostData: host
})

const countVisibleHosts = children =>
  children.reduce((count, item) => {
    if (item?.isHost) {
      return count + 1
    }

    if (Array.isArray(item?.children) && item.children.length > 0) {
      return count + countVisibleHosts(item.children)
    }

    return count
  }, 0)

const buildVisibleGroupTree = (groups, parentId = 0) =>
  (Array.isArray(groups) ? groups : []).reduce((result, group) => {
    if (!group) {
      return result
    }

    if (isDefaultGroupNode(group)) {
      return result
    }

    const nextParentId = isRootGroupNode(group) ? parentId : group.id
    const nestedGroups = buildVisibleGroupTree(group.children, nextParentId)

    if (isRootGroupNode(group)) {
      result.push(...nestedGroups)
      return result
    }

    const visibleHosts = (Array.isArray(group.hosts) ? group.hosts : [])
      .filter(host => !isWindowsHost(host))
      .map(createHostNode)

    const children = [...nestedGroups, ...visibleHosts]

    result.push({
      ...group,
      parentId,
      children,
      hostCount: countVisibleHosts(children)
    })

    return result
  }, [])

// 终端标签页管理
const terminalTabs = ref([])
const normalizeSearchValue = value => String(value || '').trim().toLowerCase()

const getHostSearchValues = host =>
  [
    host?.hostName,
    host?.sshIp,
    host?.privateIp,
    host?.publicIp,
    host?.ip
  ]
    .map(normalizeSearchValue)
    .filter(Boolean)

const findMatchingNodes = (nodes, searchText) => {
  const keyword = normalizeSearchValue(searchText)
  if (!keyword) return []

  const results = []
  const traverse = (nodeList, parentKeys = []) => {
    nodeList.forEach(node => {
      const currentKeys = [...parentKeys, node.id]
      const nameMatch = normalizeSearchValue(node.name).includes(keyword)
      const hostMatch = node.isHost && getHostSearchValues(node.hostData).some(value => value.includes(keyword))

      if (nameMatch || hostMatch) {
        results.push({ node, keys: currentKeys })
      }

      if (Array.isArray(node.children) && node.children.length > 0) {
        traverse(node.children, currentKeys)
      }
    })
  }

  traverse(nodes)
  return results
}

const filterTreeByKeyword = (nodes, searchText) => {
  const keyword = normalizeSearchValue(searchText)

  if (!keyword) {
    return nodes
  }

  return nodes.reduce((result, node) => {
    const nameMatch = normalizeSearchValue(node?.name).includes(keyword)

    if (node?.isHost) {
      const hostMatch = getHostSearchValues(node.hostData).some(value => value.includes(keyword))
      if (nameMatch || hostMatch) {
        result.push(node)
      }
      return result
    }

    const children = Array.isArray(node?.children)
      ? filterTreeByKeyword(node.children, keyword)
      : []

    if (nameMatch || children.length > 0) {
      result.push({
        ...node,
        children,
        hostCount: countVisibleHosts(children)
      })
    }

    return result
  }, [])
}

const syncDisplayGroupList = () => {
  const keyword = normalizeSearchValue(groupSearchText.value)
  displayGroupList.value = keyword
    ? filterTreeByKeyword(groupList.value, keyword)
    : groupList.value
}

const getDefaultExpandedKeys = (nodes = []) =>
  (Array.isArray(nodes) ? nodes : [])
    .filter(node => !node?.isHost)
    .map(node => node.id)

const applyTreeMatches = matches => {
  if (!Array.isArray(matches) || matches.length === 0) return

  const allKeys = new Set()
  matches.forEach(match => {
    match.keys.slice(0, -1).forEach(key => allKeys.add(key))
  })
  expandedKeys.value = Array.from(allKeys)

  nextTick(() => {
    if (groupTree.value) {
      groupTree.value.setCurrentKey(matches[0].node.id)
    }
  })
}

const findHostNodeByRouteTarget = ({ hostId, ip }) => {
  const targetHostId = String(hostId || '').trim()
  const targetIp = normalizeSearchValue(ip)
  let matchedNode = null

  const traverse = (nodeList, parentKeys = []) => {
    for (const node of nodeList) {
      const currentKeys = [...parentKeys, node.id]

      if (node?.isHost) {
        const nodeHostId = String(node.hostData?.id || '').trim()
        const hostMatchesId = !!targetHostId && nodeHostId === targetHostId
        const hostMatchesIp = !!targetIp && getHostSearchValues(node.hostData).includes(targetIp)

        if (hostMatchesId || hostMatchesIp) {
          matchedNode = { node, keys: currentKeys }
          return
        }
      }

      if (Array.isArray(node?.children) && node.children.length > 0) {
        traverse(node.children, currentKeys)
        if (matchedNode) return
      }
    }
  }

  traverse(groupList.value)
  return matchedNode
}

const handleRouteAutoOpen = async () => {
  const autoOpen = ['1', 'true'].includes(String(route.query.autoOpen || '').trim().toLowerCase())
  const hostId = String(route.query.hostId || '').trim()
  const ip = String(route.query.ip || '').trim()
  const signature = `${autoOpen ? '1' : '0'}|${hostId}|${ip}`

  if (!autoOpen || (!hostId && !ip) || groupList.value.length === 0) {
    return
  }

  if (lastAutoOpenSignature.value === signature) {
    return
  }

  if (ip) {
    groupSearchText.value = ip
    const searchMatches = findMatchingNodes(groupList.value, ip)
    if (searchMatches.length > 0) {
      applyTreeMatches(searchMatches)
    }
  }

  const targetNode = findHostNodeByRouteTarget({ hostId, ip })
  if (!targetNode) {
    lastAutoOpenSignature.value = signature
    ElMessage.warning('未找到目标 SSH 主机')
    return
  }

  applyTreeMatches([targetNode])
  lastAutoOpenSignature.value = signature
  await nextTick()
  await openTerminal(targetNode.node.hostData)
}

const activeTabName = ref('')
let tabIndex = 0
let refreshStatusResetTimer = null

const activeTabIndex = computed(() =>
  terminalTabs.value.findIndex(tab => tab.name === activeTabName.value)
)
const hasLeftTabs = computed(() => activeTabIndex.value > 0)
const hasRightTabs = computed(() =>
  activeTabIndex.value >= 0 && activeTabIndex.value < terminalTabs.value.length - 1
)
const hasOtherTabs = computed(() => terminalTabs.value.length > 1 && activeTabIndex.value >= 0)
const refreshButtonIcon = computed(() => {
  if (refreshStatus.value === 'success') return Check
  if (refreshStatus.value === 'error') return Close
  return Refresh
})
const refreshButtonClass = computed(() => ({
  'refresh-btn-success': refreshStatus.value === 'success',
  'refresh-btn-error': refreshStatus.value === 'error'
}))

const clearRefreshStatusResetTimer = () => {
  if (refreshStatusResetTimer) {
    clearTimeout(refreshStatusResetTimer)
    refreshStatusResetTimer = null
  }
}

const resetRefreshStatusLater = () => {
  clearRefreshStatusResetTimer()
  refreshStatusResetTimer = setTimeout(() => {
    refreshStatus.value = 'idle'
    refreshStatusResetTimer = null
  }, 3000)
}

// 拖拽调整宽度相关
const treeWidth = ref(280) // 默认宽度280px
const minTreeWidth = 200 // 最小宽度
const maxTreeWidth = 600 // 最大宽度
const isResizing = ref(false)

// 加载主机分组列表
const loadGroupList = async () => {
  try {
    const response = await cmdbApi.getGroupListWithHosts()
    if (response.data.code === 200) {
      groupList.value = buildVisibleGroupTree(response.data.data)
      syncDisplayGroupList()

      expandedKeys.value = getDefaultExpandedKeys(displayGroupList.value)

      await handleRouteAutoOpen()
      return true
    }
    return false
  } catch (error) {
    console.error('加载分组列表失败:', error)
    ElMessage.error('加载分组列表失败')
    return false
  }
}

// 处理主机节点点击（双击打开终端）
const handleGroupClick = (data, node) => {
  if (data.isHost && node.parent.parent) {
    // 双击检测逻辑
    const now = Date.now()
    const lastClickTime = node._lastClickTime || 0

    if (now - lastClickTime < 300) {
      // 双击 - 打开终端
      openTerminal(data.hostData)
    }

    node._lastClickTime = now
  }
}

// 打开新的终端标签页
const normalizeHostAuths = data => {
  const auths = Array.isArray(data?.auths) ? data.auths : []
  return auths.map(auth => ({
    ...auth,
    authId: auth.authId || auth.id
  })).filter(auth => auth.authId)
}

const escapeHtml = value => String(value ?? '')
  .replace(/&/g, '&amp;')
  .replace(/</g, '&lt;')
  .replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;')
  .replace(/'/g, '&#39;')

const selectLoginAuth = async (host) => {
  const response = await cmdbApi.getHostSshAuths(host.id)
  const data = response?.data?.data || response?.data || {}
  const auths = normalizeHostAuths(data)

  if (!data.required || auths.length <= 1) {
    return auths[0]?.authId || host.sshKeyId || ''
  }

  const authCards = auths.map((auth, index) => {
    const checked = index === 0 ? 'checked' : ''
    const defaultBadge = auth.isDefault ? '<span class="ssh-auth-default">默认</span>' : ''
    const authName = escapeHtml(auth.name || `认证账号 ${auth.authId}`)
    const username = escapeHtml(auth.username || '-')
    const authId = escapeHtml(auth.authId)

    return `
      <label class="ssh-auth-option">
        <input type="radio" name="ssh-auth-id" value="${authId}" ${checked} />
        <span class="ssh-auth-radio"></span>
        <span class="ssh-auth-content">
          <span class="ssh-auth-row">
            <strong>${authName}</strong>
            ${defaultBadge}
            <span class="ssh-auth-user">登录用户：${username}</span>
          </span>
        </span>
      </label>
    `
  }).join('')

  let selectedAuthId = auths[0]?.authId || ''

  await ElMessageBox.confirm(
    `<div class="ssh-auth-dialog">
      <div class="ssh-auth-host">请选择用于登录 <strong>${escapeHtml(host.hostName || host.id)}</strong> 的认证账号</div>
      <div class="ssh-auth-list">${authCards}</div>
    </div>`,
    '选择登录账号',
    {
      dangerouslyUseHTMLString: true,
      confirmButtonText: '登录',
      cancelButtonText: '取消',
      customClass: 'ssh-auth-message-box',
      beforeClose: (action, instance, done) => {
        if (action === 'confirm') {
          selectedAuthId = document.querySelector('input[name="ssh-auth-id"]:checked')?.value || selectedAuthId
        }
        done()
      }
    }
  )

  return selectedAuthId
}

const openTerminal = async (host) => {
  const hostId = String(host?.id || '').trim()
  if (!hostId || openingHostIds.has(hostId)) {
    return
  }

  openingHostIds.add(hostId)
  try {
    // 检查是否已经打开该主机的终端
    const sameHostTabCount = terminalTabs.value.filter(tab => tab.host.id === host.id).length

    let authId = ''
    try {
      authId = await selectLoginAuth(host)
    } catch (error) {
      if (error !== 'cancel') {
        console.error('选择SSH登录账号失败:', error)
        ElMessage.error('获取SSH登录账号失败')
      }
      return
    }

    // 创建新的标签页
    const newTabName = `tab_${++tabIndex}`
    const newTab = {
      name: newTabName,
      title: sameHostTabCount > 0 ? `${host.hostName} (${sameHostTabCount + 1})` : host.hostName,
      host: {
        id: host.id,
        hostName: host.hostName,
        sshName: host.sshName,
        sshIp: host.sshIp,
        sshPort: host.sshPort,
        sshKeyId: host.sshKeyId,
        authId
      },
      visible: true
    }

    terminalTabs.value.push(newTab)
    activeTabName.value = newTabName
  } finally {
    openingHostIds.delete(hostId)
  }
}

// 关闭标签页
const handleTabRemove = (targetName) => {
  const tabs = terminalTabs.value
  const targetIndex = tabs.findIndex(tab => tab.name === targetName)
  if (targetIndex < 0) {
    return
  }

  closeTerminalTabsByIndex(index => index === targetIndex, targetIndex)
}

const syncActiveTabAfterClose = (nextTabs, fallbackIndex, preferredName = activeTabName.value) => {
  terminalTabs.value = nextTabs

  if (nextTabs.length === 0) {
    activeTabName.value = ''
    return
  }

  if (preferredName && nextTabs.some(tab => tab.name === preferredName)) {
    activeTabName.value = preferredName
    return
  }

  const safeIndex = Math.min(Math.max(fallbackIndex, 0), nextTabs.length - 1)
  activeTabName.value = nextTabs[safeIndex].name
}

const closeTerminalTabsByIndex = (shouldClose, fallbackIndex = 0, preferredName = activeTabName.value) => {
  const tabs = terminalTabs.value
  const nextTabs = tabs.filter((tab, index) => !shouldClose(index, tab))

  if (nextTabs.length === tabs.length) {
    return
  }

  syncActiveTabAfterClose(nextTabs, fallbackIndex, preferredName)
}

const handleCloseCommand = (command) => {
  const tabs = terminalTabs.value
  const index = activeTabIndex.value

  if (tabs.length === 0 || (command !== 'all' && index < 0)) {
    return
  }

  if (command === 'all') {
    syncActiveTabAfterClose([], 0, '')
    return
  }

  if (command === 'left') {
    closeTerminalTabsByIndex(paneIndex => paneIndex < index, 0, activeTabName.value)
    return
  }

  if (command === 'right') {
    closeTerminalTabsByIndex(paneIndex => paneIndex > index, index, activeTabName.value)
    return
  }

  if (command === 'others') {
    closeTerminalTabsByIndex(paneIndex => paneIndex !== index, 0, activeTabName.value)
  }
}

// 标签页点击切换
const handleTabClick = () => {
  // 可以在这里添加标签切换的额外逻辑
}

// 终端关闭回调
const handleTerminalClosed = (tabName) => {
  handleTabRemove(tabName)
}

// 刷新主机列表
const handleRefresh = async () => {
  clearRefreshStatusResetTimer()
  refreshStatus.value = 'idle'
  groupSearchText.value = ''
  expandedKeys.value = []
  refreshStatus.value = await loadGroupList() ? 'success' : 'error'
  resetRefreshStatusLater()
}

// 搜索分组
const handleGroupSearch = () => {
  syncDisplayGroupList()

  if (!groupSearchText.value) {
    expandedKeys.value = getDefaultExpandedKeys(displayGroupList.value)
    nextTick(() => {
      groupTree.value?.setCurrentKey(null)
    })
    return
  }

  const matches = findMatchingNodes(displayGroupList.value, groupSearchText.value)
  if (matches.length > 0) {
    applyTreeMatches(matches)
  } else {
    expandedKeys.value = []
    nextTick(() => {
      groupTree.value?.setCurrentKey(null)
    })
  }
}

// 节点展开/折叠
const handleNodeExpand = (data, node) => {
  if (!expandedKeys.value.includes(node.key)) {
    expandedKeys.value.push(node.key)
  }
}

const handleNodeCollapse = (data, node) => {
  expandedKeys.value = expandedKeys.value.filter(key => key !== node.key)
}

// 开始拖拽调整宽度
const startResize = (e) => {
  e.preventDefault()
  isResizing.value = true
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'

  // 获取容器的左边界位置
  const container = e.target.closest('.ssh-layout')
  if (!container) return

  const containerRect = container.getBoundingClientRect()

  const handleMouseMove = (moveEvent) => {
    if (!isResizing.value) return

    // 计算鼠标相对于容器左边界的距离，这就是新的宽度
    let newWidth = moveEvent.clientX - containerRect.left

    // 限制在最小和最大宽度之间
    if (newWidth < minTreeWidth) {
      newWidth = minTreeWidth
    } else if (newWidth > maxTreeWidth) {
      newWidth = maxTreeWidth
    }

    treeWidth.value = newWidth
  }

  const handleMouseUp = () => {
    isResizing.value = false
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
    document.removeEventListener('mousemove', handleMouseMove)
    document.removeEventListener('mouseup', handleMouseUp)

    // 保存宽度到本地存储
    localStorage.setItem('ssh-tree-width', treeWidth.value.toString())
  }

  document.addEventListener('mousemove', handleMouseMove)
  document.addEventListener('mouseup', handleMouseUp)
}

// 重置树宽度
const resetTreeWidth = () => {
  treeWidth.value = 280
  localStorage.setItem('ssh-tree-width', '280')
  ElMessage.success('已恢复默认宽度')
}

// 从本地存储加载宽度
const loadTreeWidth = () => {
  const savedWidth = localStorage.getItem('ssh-tree-width')
  if (savedWidth) {
    const width = parseInt(savedWidth, 10)
    if (width >= minTreeWidth && width <= maxTreeWidth) {
      treeWidth.value = width
    }
  }
}

// 组件挂载
watch(
  expandedKeys,
  newKeys => {
    nextTick(() => {
      const tree = groupTree.value
      if (!tree) {
        return
      }

      const expandedKeySet = new Set((Array.isArray(newKeys) ? newKeys : []).map(key => String(key)))
      const nodesMap = tree.store?.nodesMap || {}
      Object.values(nodesMap).forEach(node => {
        const shouldExpand = expandedKeySet.has(String(node?.key))
        if (shouldExpand && !node?.expanded) {
          node.expand()
        } else if (!shouldExpand && node?.expanded) {
          node.collapse()
        }
      })
    })
  },
  {
    deep: true
  }
)

watch(
  () => [route.query.autoOpen, route.query.hostId, route.query.ip],
  () => {
    handleRouteAutoOpen()
  }
)

onMounted(() => {
  loadTreeWidth()
  loadGroupList()
})

// 组件卸载时清理
onUnmounted(() => {
  clearRefreshStatusResetTimer()
  document.body.style.cursor = ''
  document.body.style.userSelect = ''
})
</script>

<style scoped>
.ssh-container {
  height: 100%;
  min-height: 0;
  padding: 0;
  overflow: hidden;
  background-color: #f5f7fa;
}

.ssh-layout {
  display: flex;
  height: 100%;
  min-height: 0;
  gap: 0;
}

/* ==================== 左侧主机树区域 ==================== */
.tree-section {
  flex-shrink: 0;
  min-height: 0;
  transition: width 0.1s ease-out;
}

/* ==================== 拖拽分隔条 ==================== */
.resizer {
  width: 8px;
  background: transparent;
  cursor: col-resize;
  position: relative;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.2s ease;
}

.resizer:hover {
  background-color: rgba(64, 158, 255, 0.1);
}

.resizer:hover .resizer-line {
  background-color: #409eff;
  width: 3px;
}

.resizer-line {
  width: 1px;
  height: 40px;
  background-color: #dcdfe6;
  border-radius: 2px;
  transition: all 0.2s ease;
}

.resizer:active .resizer-line {
  background-color: #409eff;
  width: 3px;
}

.group-card {
  height: 100%;
  border-radius: 0;
  border: 1px solid #e4e7ed;
  border-top: none;
  border-left: none;
  border-bottom: none;
  overflow: hidden;
  margin: 0;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.header-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.search-container {
  display: flex;
  gap: 8px;
  margin-bottom: 15px;
}

.search-input {
  flex: 1;
}

:deep(.search-input .el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px #003d99 inset;
}

:deep(.search-input .el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 1px #003d99 inset !important;
}

:deep(.search-input .el-icon) {
  color: #003d99;
}

.refresh-btn {
  flex-shrink: 0;
}

.refresh-btn-success {
  color: #67c23a;
  border-color: #b3e19d;
  background-color: #f0f9eb;
}

.refresh-btn-success:hover,
.refresh-btn-success:focus {
  color: #67c23a;
  border-color: #95d475;
  background-color: #e1f3d8;
}

.refresh-btn-error {
  color: #f56c6c;
  border-color: #fab6b6;
  background-color: #fef0f0;
}

.refresh-btn-error:hover,
.refresh-btn-error:focus {
  color: #f56c6c;
  border-color: #f89898;
  background-color: #fde2e2;
}

.group-tree {
  max-height: calc(100% - 100px);
  overflow-y: auto;
}

.tree-empty {
  padding-top: 32px;
}

.tree-icon {
  margin-right: 6px;
  font-size: 14px;
  display: inline-flex;
  align-items: center;
  vertical-align: middle;
}

.tree-icon svg,
.tree-icon img {
  width: 16px;
  height: 16px;
}

/* 文件夹图标 */
.folder-icon img {
  width: 14px;
  height: 14px;
}

.business-group {
  font-weight: 600;
  color: #303133;
}

.host-node {
  color: #303133;
  font-weight: 500;
}

.node-label {
  flex: 1;
}

.host-count {
  margin-left: 4px;
  color: #909399;
  font-size: 12px;
  font-weight: 400;
}

/* ==================== 右侧终端区域 ==================== */
.terminal-section {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: #1e1e1e;
  border-radius: 0;
  border: none;
  overflow: hidden;
}

.terminal-tabs {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.terminal-tabs-shell {
  position: relative;
  height: 100%;
  min-height: 0;
}

.terminal-close-actions {
  position: absolute;
  top: 6px;
  right: 10px;
  z-index: 5;
}

.terminal-close-actions :deep(.el-button) {
  width: 28px;
  height: 28px;
}

:deep(.el-tabs__header) {
  margin: 0;
  padding-right: 52px;
  border-bottom: 1px solid #e4e7ed;
  background: #f5f7fa;
  box-sizing: border-box;
}

:deep(.el-tabs__content) {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  padding: 0;
}

:deep(.el-tab-pane) {
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

:deep(.el-tabs__item) {
  height: 40px;
  line-height: 40px;
  padding: 0 20px;
  color: #606266;
  font-size: 14px;
}

:deep(.el-tabs__item.is-active) {
  background: #ffffff;
  color: #409eff;
  font-weight: 600;
}

:deep(.el-tabs__item:hover) {
  color: #409eff;
}

/* ==================== 欢迎占位页面 ==================== */
.welcome-placeholder {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: #1e1e1e;
  gap: 30px;
}

.welcome-icon {
  width: 400px;
  height: 300px;
  opacity: 0.8;
  transition: all 0.3s ease;
}

.welcome-icon:hover {
  opacity: 1;
  transform: scale(1.05);
}

.welcome-icon svg {
  width: 100%;
  height: 100%;
  filter: drop-shadow(0 4px 8px rgba(0, 0, 0, 0.1));
}

.welcome-text {
  text-align: center;
}

.welcome-text h1 {
  font-size: 48px;
  font-weight: 700;
  color: #ffffff;
  margin: 0 0 12px 0;
  letter-spacing: 2px;
  font-family: 'Arial', 'Microsoft YaHei', sans-serif;
}

.welcome-text p {
  font-size: 16px;
  color: #b0b0b0;
  margin: 0;
}

/* ==================== 滚动条样式 ==================== */
.group-tree::-webkit-scrollbar {
  width: 6px;
}

.group-tree::-webkit-scrollbar-track {
  background: #f1f1f1;
  border-radius: 3px;
}

.group-tree::-webkit-scrollbar-thumb {
  background: #c1c1c1;
  border-radius: 3px;
}

.group-tree::-webkit-scrollbar-thumb:hover {
  background: #a8a8a8;
}

/* ==================== 树节点样式优化 ==================== */
:deep(.el-tree-node__content) {
  height: 32px;
  border-radius: 4px;
  margin-bottom: 2px;
}

:deep(.el-tree-node__content:hover) {
  background-color: #f5f7fa;
}

.group-tree :deep(.el-tree-node.is-current > .el-tree-node__content),
.group-tree :deep(.el-tree-node.is-current > .el-tree-node__content:hover) {
  background: linear-gradient(180deg, #0b4fc2 0%, #003d99 100%) !important;
  color: #ffffff !important;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.12);
}

.group-tree :deep(.el-tree-node.is-current > .el-tree-node__content .business-group),
.group-tree :deep(.el-tree-node.is-current > .el-tree-node__content .host-node),
.group-tree :deep(.el-tree-node.is-current > .el-tree-node__content .node-label) {
  color: #ffffff !important;
  text-shadow: 0 1px 0 rgba(0, 0, 0, 0.12);
}

.group-tree :deep(.el-tree-node.is-current > .el-tree-node__content .host-count) {
  color: rgba(255, 255, 255, 0.88) !important;
}

/* ==================== el-card 样式覆盖 ==================== */
:deep(.el-card__header) {
  padding: 12px 16px;
  border-bottom: 1px solid #e4e7ed;
}

:deep(.el-card__body) {
  padding: 16px;
  height: calc(100% - 49px);
  overflow: hidden;
}

:global(.home-container .el-main:has(.ssh-container)) {
  overflow-y: hidden;
  padding: 0;
  padding-bottom: 0;
}
</style>

<style>
.ssh-auth-message-box {
  width: 520px;
  max-width: calc(100vw - 32px);
  border-radius: 14px;
}

.ssh-auth-message-box .el-message-box__header {
  padding: 20px 22px 10px;
}

.ssh-auth-message-box .el-message-box__title {
  font-size: 18px;
  font-weight: 700;
  color: #1f2937;
}

.ssh-auth-message-box .el-message-box__content {
  padding: 8px 22px 16px;
}

.ssh-auth-message-box .el-message-box__btns {
  padding: 0 22px 20px;
}

.ssh-auth-dialog {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.ssh-auth-host {
  padding: 10px 12px;
  border-radius: 10px;
  background: #f8fafc;
  color: #64748b;
  font-size: 13px;
}

.ssh-auth-host strong {
  color: #1e40af;
}

.ssh-auth-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 320px;
  overflow-y: auto;
}

.ssh-auth-option {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  background: #ffffff;
  cursor: pointer;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;
}

.ssh-auth-option:hover {
  border-color: #93c5fd;
  background: #f8fbff;
  box-shadow: 0 8px 24px rgba(37, 99, 235, 0.08);
}

.ssh-auth-option input {
  display: none;
}

.ssh-auth-radio {
  width: 16px;
  height: 16px;
  border: 2px solid #cbd5e1;
  border-radius: 50%;
  flex: 0 0 auto;
  position: relative;
}

.ssh-auth-option input:checked + .ssh-auth-radio {
  border-color: #2563eb;
}

.ssh-auth-option input:checked + .ssh-auth-radio::after {
  content: '';
  position: absolute;
  width: 6px;
  height: 6px;
  top: 3px;
  left: 3px;
  border-radius: 50%;
  background: #2563eb;
}

.ssh-auth-option:has(input:checked) {
  border-color: #2563eb;
  background: #eff6ff;
}

.ssh-auth-content {
  min-width: 0;
  flex: 1;
}

.ssh-auth-row {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  color: #64748b;
  font-size: 13px;
}

.ssh-auth-row strong {
  color: #0f172a;
  font-size: 15px;
  min-width: 76px;
  max-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ssh-auth-user {
  min-width: 130px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ssh-auth-default {
  display: inline-flex;
  align-items: center;
  height: 20px;
  padding: 0 8px;
  border-radius: 999px;
  background: #dcfce7;
  color: #15803d;
  font-size: 12px;
  font-weight: 600;
}

</style>
