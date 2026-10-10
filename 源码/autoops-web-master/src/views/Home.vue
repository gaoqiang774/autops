<template>
    <el-container class="home-container" :style="homeLayoutStyle">
      <el-header height="64px" class="platform-header">
        <div class="header-brand" :style="{ width: isCollapse ? '64px' : '200px' }">
          <img src="../assets/image/DevOps平台.svg" >
          <h2 v-show="!isCollapse">AutoOps天枢</h2>
        </div>
        <div class="page-header">
          <div class="header-left">

            <el-popover
              v-model:visible="navDialogVisible"
              placement="bottom-start"
              :width="700"
              trigger="click"
              :show-arrow="false"
              popper-class="platform-nav-popper"
            >
              <template #reference>
                <el-button type="text" :class="['nav-trigger-btn', { 'is-open': navDialogVisible }]">
                  <span class="nav-trigger-pill">
                  <span class="nav-trigger-text">平台导航</span>
                  <el-icon size="12" class="nav-trigger-arrow"><component :is="'ArrowDown'" /></el-icon>
                  </span>
                </el-button>
              </template>

              <div class="nav-module-panel">
                <div v-if="navModules.length" class="nav-module-body">
                  <div class="nav-module-grid">
                    <button
                      v-for="(item, index) in navModules"
                      :key="item.key"
                      type="button"
                      class="nav-module-card visitor-navigation"
                      :class="{ active: currentNavModule === item.key }"
                      @click="handleModuleSwitch(item.key)"
                    >
                      <span
                        class="nav-module-icon"
                        :style="{ background: getNavThemeStyle(index)['--nav-icon'] }"
                      >
                        <el-icon size="28" style="color:#ffffff"><component :is="item.icon" /></el-icon>
                      </span>
                      <span class="nav-module-text">
                        <span class="nav-module-title">{{ item.label }}</span>
                        <span class="nav-module-desc">{{ item.description }}</span>
                      </span>
                    </button>
                  </div>
                </div>
                <div v-else class="nav-module-empty">No platform modules</div></div>
            </el-popover>
          </div>
          <div class="header-right">
            <HeadImage />
          </div>
        </div>
      </el-header>
      <el-container class="shell-body">
      <el-aside ref="sideNav" :width="isCollapse ? '64px' : '200px'">
        <div class="aside-geometry" aria-hidden="true">
          <span class="aside-zone zone-1"></span>
          <span class="aside-zone zone-2"></span>
          <span class="aside-zone zone-3"></span>
          <span class="aside-zone zone-4"></span>
        </div>

        <div v-if="showRootMenuLabel" class="root-menu-title">
          <el-icon><component :is="getMenuIcon(currentRootMenu)" /></el-icon>
          <span v-show="!isCollapse">{{ currentRootMenu.menuName }}</span>
        </div>
        <el-menu background-color="transparent"  router :default-active="currentMenuActivePath"
               :collapse="isCollapse" :collapse-transition="false" :unique-opened="true" :class="['modern-menu', { 'with-root-menu': showRootMenuLabel }]">
        <!--无子集菜单-->
        <el-menu-item :index="getMenuPath(item)" v-for="item in noChildren" :key="item.menuName" @click="saveNavState(getMenuPath(item))">
          <el-icon><component :is="getMenuIcon(item)" /></el-icon>
          <template v-slot:title>
            <span>{{ item.menuName }}</span>
          </template>
        </el-menu-item>
        <!--有子集菜单-->
        <el-sub-menu :index="item.id + ''" v-for="item in hasChildren" :key="item.id">
          <template #title>
            <el-icon><component :is="getMenuIcon(item)" /></el-icon>
            <span>{{ item.menuName }}</span>
          </template>
          <el-menu-item :index="getMenuPath(subItem)" v-for="subItem in item.menuSvoList" :key="subItem.id"
                        @click="saveNavState(getMenuPath(subItem))">
            <el-icon><component :is="getMenuIcon(subItem)" /></el-icon>
            <template v-slot:title>
              <span>{{ subItem.menuName }}</span>
            </template>
          </el-menu-item>
        </el-sub-menu>

        
      </el-menu>
        <div class="aside-fold-btn">
          <el-button type="text" @click="toggleCollapse" class="collapse-btn">
            <el-icon size="24"><component :is="collapseBtnClass" /></el-icon>
          </el-button>
        </div>
    </el-aside>

    <!-- 主体内容 -->
    <el-container>
      <el-header v-if="false" height="50px" class="page-header">
        <div class="header-left">
          <div class="fold-btn">
            <el-button type="text" @click="toggleCollapse" class="collapse-btn">
              <el-icon size="24"><component :is="collapseBtnClass" /></el-icon>
            </el-button>
          </div>
          <el-popover
            v-model:visible="navDialogVisible"
            placement="bottom-start"
            :width="700"
            trigger="click"
            :show-arrow="false"
            popper-class="platform-nav-popper"
          >
            <template #reference>
              <el-button type="text" :class="['nav-trigger-btn', { 'is-open': navDialogVisible }]">
                <span class="nav-trigger-pill">
                  <span class="nav-trigger-text">平台导航</span>
                  <el-icon size="12" class="nav-trigger-arrow"><component :is="'ArrowDown'" /></el-icon>
                </span>
              </el-button>
            </template>

            <div class="nav-module-panel">
              <div v-if="navModules.length" class="nav-module-body">
                <div class="nav-module-grid">
                  <button
                    v-for="(item, index) in navModules"
                    :key="item.key"
                    type="button"
                    class="nav-module-card visitor-navigation"
                    :class="{ active: currentNavModule === item.key }"
                    @click="handleModuleSwitch(item.key)"
                  >
                    <span
                      class="nav-module-icon"
                      :style="{ background: getNavThemeStyle(index)['--nav-icon'] }"
                    >
                      <el-icon size="28" style="color:#ffffff"><component :is="item.icon" /></el-icon>
                    </span>
                    <span class="nav-module-text">
                      <span class="nav-module-title">{{ item.label }}</span>
                      <span class="nav-module-desc">{{ item.description }}</span>
                    </span>
                  </button>
                </div>
              </div>
              <div v-else class="nav-module-empty">暂无可切换的平台导航</div>
            </div>
          </el-popover>
        </div>
        <div class="header-right">
          <HeadImage />
        </div>
      </el-header>
      <el-main
        :class="{
          'k8s-module-main': isK8sModule,
          'k8s-transparent-main': isK8sTransparentBackgroundPage,
          'k8s-terminal-main': isK8sTerminalPage
        }"
      >
        <router-view />
      </el-main>
    </el-container>

  </el-container>

  </el-container>
</template>

<script>
import storage from "@/utils/storage";
import HeadImage from "@/components/HeadImage.vue";

const normalizePath = url => {
  if (!url) return ''
  return url.startsWith('/') ? url : `/${url}`
}

const MENU_ACTIVE_PATH_ALIASES = {
  '/logs/manage': '/logs/project',
  '/k8s/list': '/k8s/cluster'
}

const getMenuActivePath = path => {
  const normalizedPath = normalizePath(path)
  return MENU_ACTIVE_PATH_ALIASES[normalizedPath] || normalizedPath
}

const getMenuKey = item => String(item?.id ?? item?.url ?? item?.menuName ?? '')

const getVisibleMenus = item => {
  if (Array.isArray(item?.menuSvoList) && item.menuSvoList.length) {
    return item.menuSvoList
  }
  return item ? [item] : []
}

const getFirstLeafPath = item => {
  if (!item) return '/dashboard'
  if (Array.isArray(item.menuSvoList) && item.menuSvoList.length) {
    return getFirstLeafPath(item.menuSvoList[0])
  }
  return normalizePath(item.url) || '/dashboard'
}

const containsPath = (item, path) => {
  const normalizedPath = normalizePath(path)
  const activePath = getMenuActivePath(normalizedPath)
  const itemPath = normalizePath(item?.url)
  if (itemPath && (
    itemPath === normalizedPath ||
    itemPath === activePath ||
    normalizedPath.startsWith(`${itemPath}/`)
  )) {
    return true
  }
  return (item?.menuSvoList || []).some(child => containsPath(child, normalizedPath))
}

const findRootMenuByPath = (menuList, path) => {
  return (menuList || []).find(item => containsPath(item, path)) || null
}

const SSH_TERMINAL_PATHS = new Set(['/cmdb/ssh'])

const isDashboardMenu = item => {
  const menuName = String(item?.menuName || '').trim()
  return normalizePath(item?.url) === '/dashboard' || menuName === '仪表盘'
}

const K8S_MENU_ICON_BY_PATH = {
  '/k8s/cluster': 'Platform',
  '/k8s/list': 'Platform',
  '/k8s/node': 'Monitor',
  '/k8s/namespace': 'Collection',
  '/k8s/workload': 'Operation',
  '/k8s/events': 'Bell',
  '/k8s/network': 'Connection',
  '/k8s/config': 'Setting',
  '/k8s/storage': 'FolderOpened',
  '/k8s/monitoring': 'DataAnalysis',
  '/k8s/arthas': 'FirstAidKit',
  '/k8s/inspection': 'Finished'
}

const K8S_MENU_ICON_BY_NAME = {
  容器管理: 'Box',
  集群管理: 'Platform',
  节点管理: 'Monitor',
  命名空间: 'Collection',
  工作负载: 'Operation',
  事件管理: 'Bell',
  网络管理: 'Connection',
  配置管理: 'Setting',
  存储管理: 'FolderOpened',
  监控仪表板: 'DataAnalysis',
  应用诊断: 'FirstAidKit',
  集群巡检: 'Finished'
}

const getMenuIcon = item => {
  const pathIcon = K8S_MENU_ICON_BY_PATH[normalizePath(item?.url)]
  if (pathIcon) return pathIcon

  const nameIcon = K8S_MENU_ICON_BY_NAME[String(item?.menuName || '').trim()]
  if (nameIcon) return nameIcon

  return item?.icon || 'Grid'
}

const NAV_THEME_PRESETS = [
  {
    accent: '#18b368',
    soft: 'rgba(24, 179, 104, 0.14)',
    icon: 'linear-gradient(135deg, #29cf7f 0%, #129b56 100%)',
    border: 'rgba(24, 179, 104, 0.30)'
  },
  {
    accent: '#0ea5e9',
    soft: 'rgba(14, 165, 233, 0.14)',
    icon: 'linear-gradient(135deg, #38c7ff 0%, #0b86c9 100%)',
    border: 'rgba(14, 165, 233, 0.30)'
  },
  {
    accent: '#4f7cff',
    soft: 'rgba(79, 124, 255, 0.14)',
    icon: 'linear-gradient(135deg, #69a3ff 0%, #3c6fff 100%)',
    border: 'rgba(79, 124, 255, 0.30)'
  },
  {
    accent: '#14b8a6',
    soft: 'rgba(20, 184, 166, 0.14)',
    icon: 'linear-gradient(135deg, #34d8c4 0%, #109787 100%)',
    border: 'rgba(20, 184, 166, 0.30)'
  },
  {
    accent: '#84cc16',
    soft: 'rgba(132, 204, 22, 0.14)',
    icon: 'linear-gradient(135deg, #9cde39 0%, #6ca90c 100%)',
    border: 'rgba(132, 204, 22, 0.30)'
  },
  {
    accent: '#f59e0b',
    soft: 'rgba(245, 158, 11, 0.14)',
    icon: 'linear-gradient(135deg, #ffbe41 0%, #dd8d00 100%)',
    border: 'rgba(245, 158, 11, 0.30)'
  }
]

export default {
  name: "Home",
  components: { HeadImage },
  data() {
    const storedMenuList = storage.getItem('leftMenuList')
    const allMenuList = Array.isArray(storedMenuList) ? storedMenuList : []
    return {
      allMenuList,
      leftMenuList: [],
      activePath: storage.getItem('activePath') || '',
      collapseBtnClass: "Fold",
      isCollapse: false,
      autoCollapsedForRoute: false,
      measuredAsideWidth: 200,
      asideResizeObserver: null,
      navDialogVisible: false,
      currentNavModule: storage.getItem('navModule') || '',
      currentRootMenu: null,
      navModules: []
    }
  },
  computed: {
    noChildren() {
      return (this.leftMenuList || []).filter(item => !item.menuSvoList)
    },
    hasChildren() {
      return (this.leftMenuList || []).filter(item => item.menuSvoList)
    },
    showRootMenuLabel() {
      return !!(this.currentRootMenu && Array.isArray(this.currentRootMenu.menuSvoList) && this.currentRootMenu.menuSvoList.length)
    },
    currentMenuActivePath() {
      return getMenuActivePath(this.activePath || this.$route.path)
    },
    isK8sModule() {
      return String(this.$route.path || '').startsWith('/k8s')
    },
    isK8sTransparentBackgroundPage() {
      return [
        '/k8s/cluster',
        '/k8s/list',
        '/k8s/namespace',
        '/k8s/node',
        '/k8s/workload',
        '/k8s/network',
        '/k8s/storage',
        '/k8s/config',
        '/k8s/events',
        '/k8s/arthas',
        '/k8s/inspection',
        '/k8s/bigdata'
      ].includes(String(this.$route.path || ''))
    },
    isK8sTerminalPage() {
      return String(this.$route.path || '').startsWith('/k8s/terminal/')
    },
    homeLayoutStyle() {
      return {
        '--home-aside-width': `${this.measuredAsideWidth || (this.isCollapse ? 64 : 200)}px`
      }
    }
  },
  watch: {
    '$route.path': {
      immediate: true,
      handler(path) {
        this.syncMenuByRoute(path)
      }
    },
    isCollapse: {
      immediate: true,
      handler() {
        this.$nextTick(() => {
          this.syncAsideWidthVariable()
        })
      }
    }
  },
  methods: {
    getSideNavElement() {
      const sideNav = this.$refs.sideNav
      return sideNav?.$el || sideNav || null
    },
    syncAsideWidthVariable() {
      if (typeof document === 'undefined') return
      const sideNavEl = this.getSideNavElement()
      const measuredWidth = sideNavEl ? Math.round(sideNavEl.getBoundingClientRect().width) : 0
      const nextWidth = measuredWidth || (this.isCollapse ? 64 : 200)
      this.measuredAsideWidth = nextWidth
      document.documentElement.style.setProperty('--home-aside-width', `${nextWidth}px`)
    },
    observeAsideWidth() {
      if (typeof ResizeObserver === 'undefined') {
        this.syncAsideWidthVariable()
        return
      }

      const sideNavEl = this.getSideNavElement()
      if (!sideNavEl) return

      this.asideResizeObserver?.disconnect()
      this.asideResizeObserver = new ResizeObserver(() => {
        this.syncAsideWidthVariable()
      })
      this.asideResizeObserver.observe(sideNavEl)
      this.syncAsideWidthVariable()
    },
    initMenuData() {
      const storedMenuList = storage.getItem('leftMenuList')
      this.allMenuList = Array.isArray(storedMenuList) ? storedMenuList : []
      this.navModules = this.allMenuList.filter(item => !isDashboardMenu(item)).map(item => ({
        key: getMenuKey(item),
        label: item.menuName,
        icon: getMenuIcon(item),
        description: Array.isArray(item.menuSvoList) && item.menuSvoList.length
          ? `${item.menuSvoList.length} 个菜单入口`
          : '点击进入该平台'
      }))
      this.syncMenuByRoute(this.$route.path)
    },
    saveNavState(activePath) {
      const menuActivePath = getMenuActivePath(activePath)
      storage.setItem('activePath', menuActivePath)
      this.activePath = menuActivePath
    },
    toggleCollapse() {
      this.isCollapse = !this.isCollapse
      this.collapseBtnClass = this.isCollapse ? "Fold" : "Expand"
      this.autoCollapsedForRoute = false
    },
    setCollapseState(collapsed, options = {}) {
      const { auto = false } = options
      this.isCollapse = collapsed
      this.collapseBtnClass = this.isCollapse ? "Fold" : "Expand"
      this.autoCollapsedForRoute = auto && collapsed
    },
    syncCollapseByRoute(path) {
      const normalizedPath = normalizePath(path)
      const shouldAutoCollapse = SSH_TERMINAL_PATHS.has(normalizedPath)

      if (shouldAutoCollapse && !this.isCollapse) {
        this.setCollapseState(true, { auto: true })
      }
    },
    removeFocusOutline() {
      // Keep the method for compatibility with older shell styling, but do not
      // blur focused menu items. Keyboard focus is handled by CSS focus-visible.
    },
    setNavModule(moduleKey, shouldNavigate = false) {
      const module = this.allMenuList.find(item => getMenuKey(item) === moduleKey) || this.allMenuList[0]
      if (!module) {
        this.leftMenuList = []
        this.currentNavModule = ''
        this.currentRootMenu = null
        return
      }

      this.currentNavModule = getMenuKey(module)
      this.currentRootMenu = module
      this.leftMenuList = getVisibleMenus(module)
      storage.setItem('navModule', this.currentNavModule)

      if (shouldNavigate) {
        const targetPath = getFirstLeafPath(module)
        if (targetPath && this.$route.path !== targetPath) {
          this.saveNavState(targetPath)
          this.$router.push(targetPath)
        }
      }
    },
    handleModuleSwitch(moduleKey) {
      this.navDialogVisible = false
      this.setNavModule(moduleKey, true)
    },
    getMenuPath(item) {
      return normalizePath(item?.url)
    },
    getMenuIcon,
    getNavThemeStyle(index) {
      const theme = NAV_THEME_PRESETS[index % NAV_THEME_PRESETS.length]
      return {
        '--nav-accent': theme.accent,
        '--nav-soft': theme.soft,
        '--nav-icon': theme.icon,
        '--nav-border': theme.border
      }
    },
    syncMenuByRoute(path) {
      this.syncCollapseByRoute(path)

      if (!this.allMenuList.length) {
        this.leftMenuList = []
        this.navModules = []
        return
      }

      const matchedRoot = findRootMenuByPath(this.allMenuList, path)
      const nextModuleKey = matchedRoot ? getMenuKey(matchedRoot) : this.currentNavModule || getMenuKey(this.allMenuList[0])
      if (this.currentNavModule !== nextModuleKey || !this.leftMenuList.length) {
        this.setNavModule(nextModuleKey)
      }
      const menuActivePath = getMenuActivePath(path)
      this.activePath = menuActivePath
      storage.setItem('activePath', menuActivePath)
    }
  },

  
  mounted() {
    this.initMenuData()
    this.removeFocusOutline()
    this.$nextTick(() => {
      this.observeAsideWidth()
    })
  },
  beforeUnmount() {
    this.asideResizeObserver?.disconnect()
    this.asideResizeObserver = null
    if (typeof document !== 'undefined') {
      document.documentElement.style.removeProperty('--home-aside-width')
    }
  }
}
</script>

<style lang="less" scoped>
.home-container {
  height: 100%;
  min-height: 100%;
  --home-bg-page:
    radial-gradient(circle at 82% 12%, rgba(79, 111, 220, 0.04), transparent 28%),
    linear-gradient(135deg, #fdfefe 0%, #f7f9fc 48%, #eef3f8 100%);
  --home-bg-page-soft:
    radial-gradient(circle at 88% 18%, rgba(79, 111, 220, 0.025), transparent 28%),
    linear-gradient(135deg, rgba(253, 254, 255, 0.98) 0%, rgba(248, 250, 253, 0.96) 46%, rgba(241, 245, 249, 0.94) 100%);
  --home-side-zone-1: #ecf2f9;
  --home-side-zone-2: #e4ecf5;
  --home-side-zone-3: #dde6f0;
  --home-side-zone-4: #d6e0eb;
  --home-side-center-x: 50%;
  --home-side-center-y: 62%;
  --home-side-top-left-x: 14%;
  --home-side-top-right-x: 86%;
  --home-side-bottom-left-x: 6%;
  --home-side-bottom-right-x: 88%;
  --home-side-divider: rgba(148, 163, 184, 0.18);
  --home-side-text-primary: #17324d;
  --home-side-text-secondary: #5a6b7f;
  --home-side-text-muted: #8695a8;
  --home-side-active-text: #1f4aa8;
  --home-side-section-text: #17324d;
  --home-side-hover-bg: linear-gradient(90deg, rgba(79, 111, 220, 0.08) 0%, rgba(79, 111, 220, 0.04) 100%);
  --home-side-section-bg: linear-gradient(90deg, rgba(241, 245, 250, 0.98) 0%, rgba(249, 252, 255, 0.98) 100%);
  --home-side-submenu-bg: linear-gradient(180deg, rgba(244, 248, 252, 0.98) 0%, rgba(239, 244, 250, 0.98) 100%);
  --home-side-active-bg: linear-gradient(135deg, #2f82e9 0%, #22a8c8 100%);
  --home-side-active-bg-inverse: rgba(255, 255, 255, 0.86);
  --home-side-active-shadow: 0 8px 18px rgba(47, 130, 233, 0.2);
  --home-side-active-gap-left: 5px;
  --home-side-active-radius: 999px;
  --home-side-active-notch-size: 18px;
  --home-side-panel-bg: #f4f7fa;
  --home-side-active-cutout-bg: #f4f7fa;
  --home-side-active-outer-bg: #f4f7fa;
  --home-topbar:
    linear-gradient(90deg,
      rgba(229, 241, 255, 0.98) 0%,
      rgba(235, 244, 255, 0.98) 18%,
      rgba(245, 249, 255, 0.98) 44%,
      rgba(255, 250, 242, 0.98) 78%,
      rgba(255, 244, 229, 0.98) 100%);
  --home-surface: rgba(255, 255, 255, 0.94);
  --home-surface-soft: rgba(248, 250, 252, 0.9);
  --home-text-primary: #172133;
  --home-text-secondary: #5f6b7c;
  --home-text-muted: #8d97a8;
  --home-border: rgba(226, 232, 240, 0.95);
  --home-brand: #4f6fdc;
  --home-brand-active: #3f60c8;
  --home-brand-soft: #edf3ff;
  --home-hover-bg: rgba(237, 243, 255, 0.7);
  --home-active-bg: linear-gradient(90deg, rgba(255, 255, 255, 0.96) 0%, rgba(248, 251, 255, 0.98) 100%);
  --home-panel-warm: linear-gradient(135deg, rgba(255, 255, 255, 0.96), rgba(248, 250, 252, 0.94));
  --home-focus-ring: 0 0 0 3px rgba(79, 111, 220, 0.2);
  --home-shell-shadow: inset 1px 0 0 rgba(226, 232, 240, 0.9);
  background: var(--home-bg-page);

  .el-aside {
    background: linear-gradient(160deg, #edf6fb 0%, #f7f8fb 48%, #fff1e9 100%);
    position: relative;
    overflow: visible;
    z-index: 20;
    isolation: isolate;
    backdrop-filter: none;
    border-right: 1px solid rgba(142, 166, 184, 0.28);
    box-shadow:
      inset -1px 0 0 rgba(255, 255, 255, 0.82),
      8px 0 22px rgba(35, 68, 94, 0.06);

    .aside-geometry {
      position: absolute;
      inset: 0;
      z-index: 0;
      pointer-events: none;
      display: none;

      .aside-zone {
        position: absolute;
        inset: 0;
      }

      .zone-1 {
        background: var(--home-side-zone-1);
        clip-path: polygon(
          100% 0,
          100% 100%,
          var(--home-side-bottom-right-x) 100%,
          var(--home-side-center-x) var(--home-side-center-y),
          var(--home-side-top-right-x) 0
        );
      }

      .zone-2 {
        background: var(--home-side-zone-2);
        clip-path: polygon(
          var(--home-side-top-left-x) 0,
          var(--home-side-top-right-x) 0,
          var(--home-side-center-x) var(--home-side-center-y)
        );
      }

      .zone-3 {
        background: var(--home-side-zone-3);
        clip-path: polygon(
          var(--home-side-center-x) var(--home-side-center-y),
          var(--home-side-bottom-right-x) 100%,
          var(--home-side-bottom-left-x) 100%
        );
      }

      .zone-4 {
        background: var(--home-side-zone-4);
        clip-path: polygon(
          0 0,
          var(--home-side-top-left-x) 0,
          var(--home-side-center-x) var(--home-side-center-y),
          var(--home-side-bottom-left-x) 100%,
          0 100%
        );
      }
    }

    &::before {
      content: '';
      display: none;
    }

    &::after {
      content: '';
      display: none;
    }

    .logo,
    .root-menu-title,
    .modern-menu {
      position: relative;
      z-index: 1;
    }

    .logo {
      margin-top: 8px;
      display: flex;
      align-items: center;
      font-size: 14px;
      height: 50px;
  
      font-weight: 500;
      padding: 8px 12px;
      white-space: nowrap;
      overflow: hidden;

      .siderbar-logo {
        width: 52px;
        height: 40px;
        margin-right: 0.5px;
        flex-shrink: 0;
        border-radius: 0;
      }

      h2 {
        margin: 0;
        font-weight: 500;
        font-family: 'SF Pro Display', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', 'Helvetica Neue', 'Segoe UI', Arial, sans-serif;
        letter-spacing: 0.8px;
        font-size: 20px;
        color: var(--home-text-primary);
        flex-shrink: 0;
        min-width: 0;
      }
    }

    .root-menu-title {
      display: flex;
      align-items: center;
      gap: 8px;
      margin: 8px 12px 6px;
      padding: 6px 8px;
      border-radius: 0;
      background: transparent;
      box-shadow: none;
      color: #6f7f93 !important;
      cursor: not-allowed;
      user-select: none;
      pointer-events: none;
      position: relative;
      z-index: 2;
      font-size: 13px;
      font-weight: 600;

      &::before {
        content: '';
        width: 3px;
        height: 14px;
        border-radius: 999px;
        background: rgba(79, 111, 220, 0.28);
        flex-shrink: 0;
      }
    }

    .modern-menu {
      border-right: none;
      background: transparent !important;
      
      * {
        outline: none !important;
        
        &:focus {
          outline: none !important;
          box-shadow: none !important;
        }
        
        &:focus-visible {
          outline: none !important;
          box-shadow: none !important;
        }
      }
    }

    .aside-fold-btn {
      position: absolute;
      bottom: 16px;
      left: 50%;
      transform: translateX(-50%);
      z-index: 10;
      width: 100%;
      display: flex;
      justify-content: center;

      .collapse-btn {
        padding: 8px;
        border-radius: 8px;
        transition: color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease;
        color: var(--home-side-text-muted);
        min-width: 36px;
        min-height: 36px;

        &:hover {
          color: var(--home-side-text-primary);
          background: rgba(255, 255, 255, 0.12);
          box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.08);
        }
      }
    }
  }

  .modern-menu {
    .el-menu-item {
      transition: background-color 0.15s ease, color 0.15s ease, box-shadow 0.15s ease !important;
      border-radius: 10px;
      margin: 2px 8px !important;
      min-height: 38px;
      height: 38px;
      line-height: 38px;
      padding: 0 10px 0 12px !important;
      box-sizing: border-box;
      color: var(--home-side-text-secondary) !important;
      font-size: 15px;
      font-weight: 500;
      outline: none !important;
      border: none !important;
      
      &:hover {
        color: var(--home-side-text-primary) !important;
        background: var(--home-side-hover-bg) !important;
        box-shadow: inset 0 0 0 1px rgba(79, 111, 220, 0.08) !important;
      }
      
      &:focus {
        outline: none !important;
        border: none !important;
        box-shadow: none !important;
      }
    }
    
    .el-sub-menu {
      .el-sub-menu__title {
        transition: background-color 0.15s ease, color 0.15s ease, box-shadow 0.15s ease !important;
        border-radius: 10px;
        margin: 2px 8px !important;
        min-height: 38px;
        height: 38px;
        line-height: 38px;
        padding: 0 10px 0 12px !important;
        box-sizing: border-box;
        color: var(--home-side-text-secondary) !important;
        font-size: 15px;
        font-weight: 500;
        outline: none !important;
        border: none !important;
        
        &:hover {
          color: var(--home-side-text-primary) !important;
          background: var(--home-side-hover-bg) !important;
          box-shadow: inset 0 0 0 1px rgba(79, 111, 220, 0.08) !important;
        }
        
        &:focus {
          outline: none !important;
          border: none !important;
          box-shadow: none !important;
        }
      }
      
      .el-menu-item {
        margin: 1px 8px !important;
        border-radius: 9px;
        min-height: 34px;
        height: 34px;
        line-height: 34px;
        padding: 0 10px 0 42px !important;
        box-sizing: border-box;
        font-size: 13px;
        font-weight: 400;
        color: #65758b !important;
        outline: none !important;
        border: none !important;

        &:hover {
          color: #17324d !important;
          background: rgba(79, 111, 220, 0.06) !important;
        }
        
        &:focus {
          outline: none !important;
          border: none !important;
          box-shadow: none !important;
        }
        
      }
    }

    > .el-sub-menu.is-active,
    > .el-sub-menu.is-opened {
      background-color: transparent !important;
      
      .el-sub-menu__title {
        background: var(--home-side-section-bg) !important;
        color: var(--home-side-section-text) !important;
        font-weight: 600;
        border-radius: 12px;
        box-shadow: inset 0 0 0 1px rgba(79, 111, 220, 0.06) !important;

        &:hover {
          background: var(--home-side-section-bg) !important;
        }
      }
    }

    &.with-root-menu {
      > .el-menu-item {
        padding-left: 32px !important;
      }

      > .el-sub-menu > :deep(.el-sub-menu__title) {
        padding-left: 32px !important;
      }
    }
  }

  :deep(.modern-menu) {
    --el-menu-item-height: 38px;
    --el-menu-sub-item-height: 34px;
    overflow: visible !important;
  }

  :deep(.modern-menu > .el-menu-item),
  :deep(.modern-menu > .el-sub-menu > .el-sub-menu__title),
  :deep(.modern-menu .el-sub-menu .el-menu-item) {
    display: flex;
    align-items: center;
    padding-top: 0 !important;
    padding-bottom: 0 !important;
    border: none !important;
    border-radius: 10px !important;
    box-sizing: border-box !important;
    box-shadow: none !important;
    overflow: visible !important;
  }

  :deep(.modern-menu .el-sub-menu .el-menu-item) {
    border-radius: 9px !important;
  }

  :deep(.modern-menu > .el-menu-item),
  :deep(.modern-menu > .el-sub-menu > .el-sub-menu__title) {
    min-height: 38px !important;
    height: 38px !important;
    line-height: 38px !important;
  }

  :deep(.modern-menu .el-sub-menu .el-menu-item) {
    min-height: 34px !important;
    height: 34px !important;
    line-height: 34px !important;
    font-size: 13px !important;
    font-weight: 400 !important;
    color: #65758b !important;
    font-family: 'Microsoft YaHei UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', 'Helvetica Neue', Arial, sans-serif !important;
    letter-spacing: 0.4px !important;
  }

  :deep(.modern-menu .el-sub-menu .el-menu-item .el-menu-title-content) {
    font-size: 13px !important;
    font-weight: 400 !important;
    color: inherit !important;
    font-family: 'Microsoft YaHei UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', 'Helvetica Neue', Arial, sans-serif !important;
    letter-spacing: 0.4px !important;
  }

  :deep(.modern-menu > .el-menu-item.root-route-item),
  :deep(.modern-menu > .el-menu-item.root-route-item .el-menu-title-content),
  :deep(.modern-menu > .el-sub-menu > .el-sub-menu__title),
  :deep(.modern-menu > .el-sub-menu > .el-sub-menu__title > span:last-child) {
    font-family: 'Microsoft YaHei UI', 'Microsoft YaHei', 'PingFang SC', 'Hiragino Sans GB', 'Helvetica Neue', Arial, sans-serif !important;
    font-weight: 600 !important;
    font-synthesis: none !important;
  }

  :deep(.modern-menu .el-sub-menu .el-menu-item.sub-route-item),
  :deep(.modern-menu .el-sub-menu .el-menu-item.sub-route-item .el-menu-title-content) {
    font-family: 'DengXian', 'Microsoft YaHei UI', 'Microsoft YaHei', 'PingFang SC', 'Hiragino Sans GB', 'Helvetica Neue', Arial, sans-serif !important;
    font-weight: 400 !important;
    font-synthesis: none !important;
  }

  :deep(.modern-menu .el-sub-menu .el-menu-item.sub-route-item.is-active),
  :deep(.modern-menu .el-sub-menu .el-menu-item.sub-route-item.is-active .el-menu-title-content) {
    font-family: 'DengXian', 'Microsoft YaHei UI', 'Microsoft YaHei', 'PingFang SC', 'Hiragino Sans GB', 'Helvetica Neue', Arial, sans-serif !important;
    font-weight: 400 !important;
  }

  :deep(.modern-menu > .el-sub-menu.is-active > .el-sub-menu__title),
  :deep(.modern-menu > .el-sub-menu.is-opened > .el-sub-menu__title) {
    background: var(--home-side-section-bg) !important;
    color: var(--home-side-section-text) !important;
    border-radius: 0 16px 16px 0 !important;
    box-shadow: none !important;
  }

  :deep(.modern-menu > .el-sub-menu > .el-menu) {
    background: var(--home-side-submenu-bg) !important;
  }

  :deep(.modern-menu .el-menu-item .el-icon),
  :deep(.modern-menu .el-sub-menu__title .el-icon) {
    margin-right: 8px !important;
  }

  :deep(.modern-menu .el-menu-item span),
  :deep(.modern-menu .el-sub-menu__title span) {
    line-height: 1 !important;
  }

  :deep(.modern-menu > .el-menu-item .el-icon),
  :deep(.modern-menu > .el-sub-menu > .el-sub-menu__title .el-icon) {
    font-size: 17px !important;
  }

  :deep(.modern-menu .el-sub-menu .el-menu-item .el-icon) {
    font-size: 15px !important;
  }

  .platform-header {
    display: flex;
    align-items: stretch;
    padding: 0;
    background: var(--home-topbar) !important;
    border-bottom: 1px solid rgba(219, 228, 241, 0.95);
    box-shadow:
      inset 0 1px 0 rgba(255, 255, 255, 0.85),
      0 10px 24px rgba(15, 23, 42, 0.035);
    z-index: 100;

    &::after {
      content: '';
      position: absolute;
      inset: auto 0 0 0;
      height: 1px;
      background: linear-gradient(90deg, rgba(79, 111, 220, 0.22) 0%, rgba(255, 173, 92, 0.18) 70%, rgba(255, 255, 255, 0) 100%);
      pointer-events: none;
    }

    .header-brand {
      display: flex;
      align-items: center;
      padding: 0 12px;
      background: transparent;
      white-space: nowrap;
      overflow: hidden;
      flex-shrink: 0;
      transition: width 0.28s;
      gap: 6px;
      border-right: none;
      position: relative;

      img {
        width: 36px;
        height: 36px;
        flex-shrink: 0;
        object-fit: contain;
        border-radius: 10px;
      }

      .siderbar-logo {
        width: 36px;
        height: 36px;
        flex-shrink: 0;
        border-radius: 10px;
        box-shadow: 0 4px 12px rgba(79, 111, 220, 0.12);
      }

      h2 {
        margin: 0;
        font-size: 18px;
        font-weight: 700;
        white-space: nowrap;
        letter-spacing: 0.8px;
        background: linear-gradient(135deg, #3f60c8 0%, #4f6fdc 38%, #7a92e7 58%, #d9a15a 100%);
        -webkit-background-clip: text;
        background-clip: text;
        -webkit-text-fill-color: transparent;
        color: transparent;
        text-shadow: 0 1px 0 rgba(255, 255, 255, 0.24);
      }
    }

    .page-header {
      flex: 1;
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 0 20px;
      background: transparent;

      .header-left,
      .header-right {
        display: flex;
        align-items: center;
      }

      .header-right {
        margin-left: auto;
      }

      .fold-btn {
        display: flex;
        align-items: center;

        .collapse-btn {
          padding: 8px;
          border-radius: 8px;
          transition: color 0.2s ease, background-color 0.2s ease, box-shadow 0.2s ease;
          color: var(--home-text-secondary);

          &:hover {
            color: var(--home-brand-active);
            background: var(--home-hover-bg);
          }
        }
      }

      .nav-trigger-btn {
        display: inline-flex;
        align-items: center;
        padding: 0;
        background: transparent;
        border: none;
        color: inherit;
        outline: none;
        box-shadow: none;

        &:hover,
        &:focus,
        &:focus-visible,
        &:active {
          background: transparent;
          color: inherit;
          border-color: transparent;
          outline: none;
          box-shadow: none;
        }

      .nav-trigger-pill {
          display: inline-flex;
          align-items: center;
          gap: 6px;
          height: 30px;
          padding: 0 12px;
          white-space: nowrap;
          border-radius: 999px;
          background: linear-gradient(135deg, #fff1c2 0%, #ffc85a 48%, #f59e0b 100%);
          color: #8a4b00;
          border: 1px solid rgba(245, 158, 11, 0.34);
          box-shadow: 0 5px 14px rgba(245, 158, 11, 0.16), inset 0 1px 0 rgba(255, 255, 255, 0.45);
          transition: background 0.2s ease, border-color 0.2s ease, color 0.2s ease, box-shadow 0.2s ease;
        }

        &:hover .nav-trigger-pill,
        &.is-open .nav-trigger-pill {
          color: #783f00;
          border-color: rgba(245, 158, 11, 0.48);
          box-shadow: 0 7px 18px rgba(245, 158, 11, 0.22), inset 0 1px 0 rgba(255, 255, 255, 0.55);
        }
      }

      .nav-trigger-text {
        font-size: 13px;
        font-weight: 600;
        line-height: 1;
        white-space: nowrap;
      }

      .nav-trigger-arrow {
        color: currentColor;
        transition: transform 0.2s ease;
      }

      .nav-trigger-btn.is-open .nav-trigger-arrow {
        transform: rotate(180deg);
      }
    }
  }

  .el-header:not(.platform-header) {
    background: transparent;
    backdrop-filter: none;
    border-bottom: none;
    box-shadow: none;
    align-items: center;
    justify-content: space-between;
    display: flex;
    padding: 0 20px;

    .header-left,
    .header-right {
      display: flex;
      align-items: center;
    }

    .header-right {
      margin-left: auto;
    }

      .fold-btn {
        display: flex;
        align-items: center;
        padding-top: 2px;
        font-size: 23px;
        cursor: pointer;

      .collapse-btn {
        padding: 8px;
        border-radius: 8px;
        transition: color 0.2s ease, background-color 0.2s ease, transform 0.2s ease, box-shadow 0.2s ease;
        color: var(--home-text-secondary);

          &:hover {
            background: rgba(255, 255, 255, 0.5);
            transform: translateY(-1px);
            color: var(--home-brand-active);
          }
        }
      }

    .nav-trigger-btn {
      display: inline-flex;
      align-items: center;
      padding: 0;
      background: transparent;
      border: none;
      color: inherit;
      outline: none;
      box-shadow: none;

      &:hover,
      &:focus,
      &:focus-visible,
      &:active {
        background: transparent;
        color: inherit;
        border-color: transparent;
        outline: none;
        box-shadow: none;
      }

        .nav-trigger-pill {
          display: inline-flex;
          align-items: center;
          gap: 6px;
          height: 30px;
          padding: 0 12px;
          white-space: nowrap;
          border-radius: 999px;
          background: linear-gradient(135deg, #fff1c2 0%, #ffc85a 48%, #f59e0b 100%);
          color: #8a4b00;
          border: 1px solid rgba(245, 158, 11, 0.34);
          box-shadow: 0 5px 14px rgba(245, 158, 11, 0.16), inset 0 1px 0 rgba(255, 255, 255, 0.45);
          transition: background 0.2s ease, border-color 0.2s ease, color 0.2s ease, box-shadow 0.2s ease;
        }

        &:hover .nav-trigger-pill,
        &.is-open .nav-trigger-pill {
          color: #783f00;
          border-color: rgba(245, 158, 11, 0.48);
          box-shadow: 0 7px 18px rgba(245, 158, 11, 0.22), inset 0 1px 0 rgba(255, 255, 255, 0.55);
        }
      }

    .nav-trigger-text {
      font-size: 13px;
      font-weight: 600;
      line-height: 1;
      white-space: nowrap;
    }

    .nav-trigger-arrow {
      color: currentColor;
      transition: transform 0.2s ease;
    }

    .nav-trigger-btn.is-open .nav-trigger-arrow {
      transform: rotate(180deg);
    }
  }

  .el-main {
    position: relative;
    background:
      radial-gradient(circle at 18% 12%, rgba(255, 232, 200, 0.58), transparent 34%),
      radial-gradient(circle at 88% 16%, rgba(111, 203, 255, 0.42), transparent 38%),
      linear-gradient(115deg, #fff7eb 0%, #f7fbff 42%, #dff3ff 100%);
    box-shadow: var(--home-shell-shadow);
    padding-bottom: 40px;
    overflow-y: auto;

    .footer-version {
      position: fixed;
      bottom: 0;
      left: 50%;
      transform: translateX(-50%);
      padding: 6px 12px;
      font-size: 11px;

      color: var(--home-text-muted);
      background: rgba(255, 255, 255, 0.58);
      border-top: 1px solid var(--home-border);
      border-left: 1px solid var(--home-border);
      border-right: 1px solid var(--home-border);
      border-top-left-radius: 6px;
      border-top-right-radius: 6px;
      backdrop-filter: blur(8px);
      z-index: 999;
      opacity: 0.8;
    }
  }

}

@media (max-width: 768px) {
  .home-container {
    .el-header {
      padding: 0 14px;
    }

    & > .platform-header .header-brand {
      width: 170px !important;
    }

    & > .platform-header > .page-header {
      min-width: 0;
    }

    & > .platform-header > .page-header .header-right {
      display: none;
    }

    .shell-body > .el-aside {
      display: none;
      width: 0 !important;
    }

    .shell-body > .el-container {
      width: 100%;
      min-width: 0;
    }

    .el-main {
      padding: 0;
    }
  }
}

@keyframes titleFlow {
  0% { background-position: 0% 50%; }
  100% { background-position: 200% 50%; }
}

@keyframes titleFloat {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-1px); }
}

@keyframes underlinePulse {
  0%, 100% { opacity: 0.6; transform: scaleX(0.96); }
  50% { opacity: 1; transform: scaleX(1); }
}

:deep(.el-menu) {
  * {
    outline: none !important;
    border: none !important;
    box-shadow: none !important;
  }
  
  .el-menu-item,
  .el-sub-menu__title {
    outline: none !important;
    border: none !important;
    
    &:focus,
    &:active,
    &:focus-within,
    &:focus-visible {
      outline: none !important;
      border: none !important;
      background-color: transparent !important;
      box-shadow: none !important;
    }
    
  }
  
  .el-menu-item {
    &,
    &:hover,
    &:focus,
    &:active,
    &.is-active {
      outline: none !important;
      border: none !important;
      
    }
  }
  
  .el-sub-menu__title {
    &,
    &:hover,
    &:focus,
    &:active {
      outline: none !important;
      border: none !important;
      
    }
  }
}

.el-aside {
  * {
    outline: 0 !important;
    outline: none !important;
    outline-width: 0 !important;
    outline-style: none !important;
    outline-color: transparent !important;
    border: 0 !important;
    border: none !important;
    border-width: 0 !important;
    border-style: none !important;
    border-color: transparent !important;
    box-shadow: none !important;
    
    &:focus,
    &:active,
    &:hover,
    &:focus-visible,
    &:focus-within,
    &:target {
      outline: 0 !important;
      outline: none !important;
      outline-width: 0 !important;
      outline-style: none !important;
      outline-color: transparent !important;
      border: 0 !important;
      border: none !important;
      border-width: 0 !important;
      border-style: none !important;
      border-color: transparent !important;
      box-shadow: none !important;
    }
  }
}

.shell-body {
  flex: 1;
  min-height: 0;

  > .el-container {
    position: relative;
    z-index: 1;
  }
}
.platform-header {
  height: 64px !important; 
  flex-shrink: 0;
  border-bottom: 1px solid var(--home-border) !important;
}
:deep(.el-menu) {
  outline: none !important;
  border: none !important;

  * {
    outline: none !important;
    border: none !important;
    outline-width: 0 !important;
    border-width: 0 !important;
  }

  .el-menu-item,
  .el-sub-menu,
  .el-sub-menu__title {
    outline: none !important;
    border: none !important;
    outline-width: 0 !important;
    border-width: 0 !important;

    &,
    &:before,
    &:after,
    &:focus,
    &:active,
    &:hover,
    &:focus-visible,
    &:focus-within,
    &.is-opened {
      outline: none !important;
      border: none !important;
      outline-width: 0 !important;
      border-width: 0 !important;
      outline-style: none !important;
      border-style: none !important;
      outline-color: transparent !important;
      border-color: transparent !important;
      box-shadow: none !important;
    }
  }
}

:deep(.modern-menu > .el-menu-item.is-active),
:deep(.modern-menu > .el-menu-item.is-active:hover),
:deep(.modern-menu > .el-menu-item.is-active:focus),
:deep(.modern-menu > .el-menu-item.is-active:focus-visible),
:deep(.modern-menu > .el-menu-item.is-active:active) {
  position: relative !important;
  z-index: 3 !important;
  overflow: visible !important;
  width: calc(100% - 16px) !important;
  margin: 0 8px !important;
  border-radius: 12px !important;
  background: var(--home-side-active-bg) !important;
  background-color: var(--home-side-active-bg) !important;
  color: #ffffff !important;
  box-shadow: var(--home-side-active-shadow) !important;
}

:deep(.modern-menu .el-sub-menu .el-menu-item.is-active),
:deep(.modern-menu .el-sub-menu .el-menu-item.is-active:hover),
:deep(.modern-menu .el-sub-menu .el-menu-item.is-active:focus),
:deep(.modern-menu .el-sub-menu .el-menu-item.is-active:focus-visible),
:deep(.modern-menu .el-sub-menu .el-menu-item.is-active:active) {
  position: relative !important;
  z-index: 3 !important;
  overflow: visible !important;
  width: calc(100% - 16px) !important;
  margin: 0 8px 0 8px !important;
  padding-left: 40px !important;
  border-radius: 12px !important;
  background: rgba(47, 130, 233, 0.12) !important;
  background-color: rgba(47, 130, 233, 0.12) !important;
  color: var(--home-side-active-text) !important;
  box-shadow: inset 0 0 0 1px rgba(47, 130, 233, 0.12) !important;
  font-weight: 400 !important;
}

:deep(.modern-menu .el-sub-menu .el-menu-item.is-active .el-menu-title-content) {
  font-weight: 400 !important;
  color: inherit !important;
}

:deep(.modern-menu .el-sub-menu .el-menu-item.is-active::before),
:deep(.modern-menu .el-sub-menu .el-menu-item.is-active::after) {
  display: none !important;
}

:deep(.modern-menu > .el-menu-item.is-active::before),
:deep(.modern-menu > .el-menu-item.is-active::after),
:deep(.modern-menu .el-sub-menu .el-menu-item.is-active::before),
:deep(.modern-menu .el-sub-menu .el-menu-item.is-active::after) {
  display: none !important;
}

:deep(.modern-menu > .el-menu-item.is-active::before),
:deep(.modern-menu .el-sub-menu .el-menu-item.is-active::before) {
  top: calc(var(--home-side-active-notch-size) * -1);
  background: radial-gradient(circle at 0 0, var(--home-side-panel-bg) 0 calc(var(--home-side-active-notch-size) - 1px), var(--home-side-active-bg) var(--home-side-active-notch-size)) !important;
}

:deep(.modern-menu > .el-menu-item.is-active::after),
:deep(.modern-menu .el-sub-menu .el-menu-item.is-active::after) {
  bottom: calc(var(--home-side-active-notch-size) * -1);
  background: radial-gradient(circle at 0 100%, var(--home-side-panel-bg) 0 calc(var(--home-side-active-notch-size) - 1px), var(--home-side-active-bg) var(--home-side-active-notch-size)) !important;
}

:deep(.modern-menu.is-collapse > .el-menu-item.is-active::before),
:deep(.modern-menu.is-collapse > .el-menu-item.is-active::after) {
  display: none;
}

:deep(.modern-menu .el-sub-menu .el-menu-item.is-active) {
  z-index: 4 !important;
}

:deep(.modern-menu > .el-menu-item:focus-visible),
:deep(.modern-menu > .el-sub-menu > .el-sub-menu__title:focus-visible),
:deep(.modern-menu .el-sub-menu .el-menu-item:focus-visible) {
  box-shadow: var(--home-focus-ring) !important;
}

:deep(.modern-menu > .el-menu-item.is-active:focus-visible),
:deep(.modern-menu .el-sub-menu .el-menu-item.is-active:focus-visible) {
  box-shadow: inset 0 0 0 1px rgba(79, 111, 220, 0.22), var(--home-focus-ring) !important;
}

</style>

<style lang="less">
.nav-trigger-btn.el-button,
.nav-trigger-btn.el-button:hover,
.nav-trigger-btn.el-button:focus,
.nav-trigger-btn.el-button:focus-visible,
.nav-trigger-btn.el-button:active {
  background: transparent !important;
  border-color: transparent !important;
  outline: none !important;
  box-shadow: none !important;
}

.platform-nav-popper.el-popover {
.el-menu--popup-bottom-start,
.el-menu--popup {
  border: 1px solid rgba(226, 232, 240, 0.96) !important;
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.06) !important;
  backdrop-filter: blur(14px) saturate(1.02) !important;
  border-radius: 16px !important;
  background: rgba(255, 255, 255, 0.98) !important;

  .el-menu-item {
    background: transparent !important;
    transition: none !important;
    margin: 0 !important;
    box-sizing: border-box !important;
    border-radius: 0 !important;
    border: none !important;
    min-height: 32px !important;
    height: 32px !important;
    line-height: 32px !important;
    padding: 0 16px !important;

    &:hover {
      background: rgba(237, 243, 255, 0.82) !important;
    }

    &.is-active {
      color: #3f60c8 !important;
      background: rgba(237, 243, 255, 0.96) !important;
      box-shadow: none !important;
      border: none !important;
    }
  }
}
}

.nav-module-panel {
  width: 100%;
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.98), rgba(247, 250, 253, 0.98));
  border-radius: 16px;
  border: 1px solid rgba(226, 232, 240, 0.84);
  box-shadow: 0 12px 28px rgba(15, 23, 42, 0.05);

  .nav-module-body {
    padding: 12px 14px 14px;
  }

  .nav-module-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 4px;
  }

  .nav-module-card {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 10px 12px;
    border: 1px solid transparent;
    border-radius: 9px;
    background: transparent;
    cursor: pointer;
    transition: background-color 0.15s ease, border-color 0.15s ease, box-shadow 0.15s ease, transform 0.15s ease;
    outline: none;

    &:hover {
      background: rgba(237, 243, 255, 0.8);
      border-color: rgba(79, 111, 220, 0.1);
    }

    &:focus-visible {
      box-shadow: 0 0 0 3px rgba(79, 111, 220, 0.18);
    }

    &.active {
      background: rgba(237, 243, 255, 0.96);
      border-color: rgba(79, 111, 220, 0.16);
      box-shadow: inset 0 0 0 1px rgba(79, 111, 220, 0.08);

      .nav-module-title {
        color: #3f60c8;
      }
    }
  }

  .nav-module-icon {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 40px;
    height: 40px;
    border-radius: 10px;
    flex-shrink: 0;
    border: none !important;
    box-shadow: none !important;
    outline: none !important;

    .el-icon {
      color: #ffffff !important;
    }
  }

  .nav-module-text {
    display: flex;
    flex-direction: column;
    gap: 3px;
    min-width: 0;
    text-align: left;
  }

  .nav-module-title {
    font-size: 13px;
    font-weight: 700;
    color: #172133;
    line-height: 1.4;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .nav-module-desc {
    font-size: 12px;
    color: #8d97a8;
    line-height: 1.3;
    white-space: normal;
    word-break: break-all;
  }

  .nav-module-empty {
    padding: 48px 24px;
    text-align: center;
    font-size: 14px;
    color: #8d97a8;
  }
}

.k8s-module-main {
  .el-table__body-wrapper .cell,
  .el-table__fixed-body-wrapper .cell {
    text-align: left !important;
  }

  .el-table__body-wrapper .cell > div,
  .el-table__body-wrapper .cell > span,
  .el-table__fixed-body-wrapper .cell > div,
  .el-table__fixed-body-wrapper .cell > span {
    text-align: left !important;
  }

  .el-table__body-wrapper .cell > div,
  .el-table__fixed-body-wrapper .cell > div {
    justify-content: flex-start !important;
  }
}

.k8s-terminal-main {
  display: flex;
  min-height: 0;
  padding: 0 !important;
  overflow: hidden !important;
}
</style>
