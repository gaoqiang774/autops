<template>
  <div class="system-nav-page">
    <header class="nav-topbar">
      <div class="brand-area">
        <span class="brand-mark">
          <el-icon><Link /></el-icon>
        </span>
        <div class="brand-copy">
          <strong>应用导航</strong>
          <span class="brand-divider" aria-hidden="true"></span>
          <span class="brand-description">集中访问常用运维平台，并按分类、环境和可见范围统一管理。</span>
        </div>
      </div>

      <el-input
        v-model="portalKeyword"
        clearable
        class="top-search"
        placeholder="搜索导航名称、地址或标签"
        @keyup.enter="loadVisibleNav"
        @clear="loadVisibleNav"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>

      <div class="top-actions">
        <el-tooltip content="刷新导航" placement="top">
          <el-button class="icon-action refresh-action" aria-label="刷新导航" :loading="portalLoading" @click="refreshPortal">
            <el-icon><Refresh /></el-icon>
          </el-button>
        </el-tooltip>
        <el-button class="aux-action add-action" @click="openItemDialog()">
          <el-icon><Plus /></el-icon>
          新增导航
        </el-button>
        <el-button class="aux-action manage-action" @click="openManageDrawer">
          <el-icon><Setting /></el-icon>
          导航管理
        </el-button>
      </div>
    </header>

    <main class="nav-workbench">
      <aside class="category-sidebar">
        <button
          type="button"
          class="category-nav-item"
          :class="{ active: activeCategoryKey === 'all' }"
          @click="selectPortalCategory('all')"
        >
          <el-icon><Star /></el-icon>
          <span>全部导航</span>
          <em>{{ visibleItemTotal }}</em>
        </button>
        <button
          v-for="category in sidebarCategories"
          :key="categoryKey(category)"
          type="button"
          class="category-nav-item"
          :class="{ active: activeCategoryKey === categoryKey(category) }"
          @click="selectPortalCategory(category)"
        >
          <span
            class="side-icon"
            :class="{ 'is-image': isImageIcon(category.icon) }"
            :style="iconThemeStyle(category)"
          >
            <img v-if="isImageIcon(category.icon)" :src="category.icon" alt="" />
            <el-icon v-else><component :is="resolveIcon(category.icon)" /></el-icon>
          </span>
          <span>{{ category.name || '未分类' }}</span>
          <em>{{ categoryItemCount(category) }}</em>
        </button>
      </aside>

      <section ref="portalScrollRef" v-loading="portalLoading" class="portal-main">
        <template v-if="portalGroups.length">
          <section
            v-for="group in portalGroups"
            :id="`nav-category-${categoryKey(group)}`"
            :key="categoryKey(group)"
            class="portal-category-section"
          >
            <h2>{{ group.category?.name || '未分类' }}</h2>
            <div v-if="group.items?.length" class="portal-card-grid">
              <article
                v-for="item in group.items"
                :key="item.id"
                class="portal-nav-card"
                role="link"
                tabindex="0"
                @click="openNav(item)"
                @keydown.enter.prevent="openNav(item)"
                @keydown.space.prevent="openNav(item)"
              >
                <span
                  class="card-icon"
                  :class="{ 'is-image': isImageIcon(item.icon) }"
                  :style="iconThemeStyle(item)"
                >
                  <img v-if="isImageIcon(item.icon)" :src="item.icon" alt="" />
                  <el-icon v-else><component :is="resolveIcon(item.icon)" /></el-icon>
                </span>
                <div class="card-main">
                  <div class="card-title-row">
                    <strong>{{ item.name }}</strong>
                  </div>
                  <p class="card-sub-row">
                    <span>{{ cardSubText(item) }}</span>
                    <em>{{ scopeText(item.scope) }}</em>
                  </p>
                  <div class="card-meta">
                    <span>{{ getUrlHost(item.url) }}</span>
                    <span :class="environmentClass(item.environment)">{{ environmentText(item.environment) }}</span>
                    <span>{{ scopeText(item.scope) }}</span>
                  </div>
                </div>
              </article>
            </div>
            <div v-else class="category-empty-line">
              当前分类暂无导航链接
            </div>
          </section>
        </template>

        <el-empty
          v-else-if="!portalLoading"
          class="portal-empty"
          description="暂无可见导航"
        />
      </section>
    </main>

    <el-drawer
      v-model="manageDrawerVisible"
      title="导航管理"
      size="82%"
      class="manage-drawer"
      modal-class="nav-manage-drawer-overlay"
      @opened="loadManageData"
    >
      <el-tabs v-model="manageTab" class="manage-tabs">
        <el-tab-pane label="导航分类" name="category">
          <section class="table-toolbar">
            <el-form :model="categoryQuery" :inline="true" class="query-form">
              <el-form-item label="分类名称">
                <el-input
                  v-model="categoryQuery.name"
                  clearable
                  placeholder="请输入分类名称"
                  @keyup.enter="loadCategories"
                />
              </el-form-item>
              <el-form-item label="状态">
                <el-select v-model="categoryQuery.status" clearable placeholder="全部状态" style="width: 140px">
                  <el-option label="启用" value="1" />
                  <el-option label="禁用" value="2" />
                </el-select>
              </el-form-item>
              <el-form-item>
                <el-button type="primary" class="primary-action" @click="loadCategories"><el-icon><Search /></el-icon>查询</el-button>
                <el-button type="warning" class="warning-action" @click="resetCategoryQuery"><el-icon><RefreshRight /></el-icon>重置</el-button>
                <el-button type="success" class="success-action" @click="openCategoryDialog()"><el-icon><Plus /></el-icon>新增分类</el-button>
              </el-form-item>
            </el-form>
          </section>

          <div class="table-shell">
            <el-table v-loading="categoryLoading" :data="categoryList" class="data-table">
              <el-table-column label="分类名称" min-width="180">
                <template #default="{ row }">
                  <div class="name-cell">
                    <span class="mini-icon">
                      <img v-if="isImageIcon(row.icon)" :src="row.icon" alt="" />
                      <el-icon v-else><component :is="resolveIcon(row.icon)" /></el-icon>
                    </span>
                    <div>
                      <strong>{{ row.name }}</strong>
                      <span>{{ row.description || '暂无描述' }}</span>
                    </div>
                  </div>
                </template>
              </el-table-column>
              <el-table-column prop="code" label="编码" min-width="160" />
              <el-table-column prop="sort" label="排序" width="90" />
              <el-table-column label="状态" width="120">
                <template #default="{ row }">
                  <el-switch
                    v-model="row.status"
                    :active-value="1"
                    :inactive-value="2"
                    active-text="启用"
                    inactive-text="禁用"
                    @change="changeCategoryStatus(row)"
                  />
                </template>
              </el-table-column>
              <el-table-column prop="createTime" label="创建时间" min-width="170" />
              <el-table-column label="操作" width="150" fixed="right">
                <template #default="{ row }">
                  <div class="text-actions">
                    <button type="button" @click="openCategoryDialog(row)">修改</button>
                    <button type="button" class="danger" @click="deleteCategory(row)">删除</button>
                  </div>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </el-tab-pane>

        <el-tab-pane label="导航链接" name="item">
          <section class="table-toolbar">
            <el-form :model="itemQuery" :inline="true" class="query-form">
              <el-form-item label="分类">
                <el-select v-model="itemQuery.categoryId" clearable filterable placeholder="全部分类" style="width: 160px">
                  <el-option v-for="item in categoryOptions" :key="item.id" :label="item.name" :value="item.id" />
                  <template #empty>
                    <div class="select-empty-action">
                      <span>暂无启用分类</span>
                    </div>
                  </template>
                </el-select>
              </el-form-item>
              <el-form-item label="名称">
                <el-input v-model="itemQuery.name" clearable placeholder="导航名称" @keyup.enter="loadItems" />
              </el-form-item>
              <el-form-item label="范围">
                <el-select v-model="itemQuery.scope" clearable placeholder="全部范围" style="width: 140px">
                  <el-option v-for="item in scopeOptions" :key="item.value" :label="item.label" :value="item.value" />
                </el-select>
              </el-form-item>
              <el-form-item label="状态">
                <el-select v-model="itemQuery.status" clearable placeholder="全部状态" style="width: 130px">
                  <el-option label="启用" value="1" />
                  <el-option label="禁用" value="2" />
                </el-select>
              </el-form-item>
              <el-form-item label="所属用户">
                <el-select v-model="itemQuery.ownerId" clearable filterable placeholder="全部用户" style="width: 150px">
                  <el-option v-for="user in userOptions" :key="user.id" :label="user.nickname || user.username" :value="user.id" />
                </el-select>
              </el-form-item>
              <el-form-item>
                <el-button type="primary" class="primary-action" @click="loadItems"><el-icon><Search /></el-icon>查询</el-button>
                <el-button type="warning" class="warning-action" @click="resetItemQuery"><el-icon><RefreshRight /></el-icon>重置</el-button>
                <el-button type="success" class="success-action" @click="openItemDialog()"><el-icon><Plus /></el-icon>新增导航</el-button>
              </el-form-item>
            </el-form>
          </section>

          <div class="table-shell">
            <el-table v-loading="itemLoading" :data="itemList" class="data-table">
              <el-table-column label="名称" min-width="210">
                <template #default="{ row }">
                  <div class="name-cell">
                    <span class="mini-icon">
                      <img v-if="isImageIcon(row.icon)" :src="row.icon" alt="" />
                      <el-icon v-else><component :is="resolveIcon(row.icon)" /></el-icon>
                    </span>
                    <div>
                      <strong>{{ row.name }}</strong>
                      <span>{{ row.description || getUrlHost(row.url) }}</span>
                    </div>
                  </div>
                </template>
              </el-table-column>
              <el-table-column prop="categoryName" label="分类" min-width="130" />
              <el-table-column label="地址" width="280">
                <template #default="{ row }">
                  <el-tooltip :content="row.url" placement="top" :show-after="300">
                    <span class="mono-text url-text">{{ row.url }}</span>
                  </el-tooltip>
                </template>
              </el-table-column>
              <el-table-column label="环境" width="90">
                <template #default="{ row }">
                  <span class="table-tag" :class="environmentClass(row.environment)">
                    {{ environmentText(row.environment) }}
                  </span>
                </template>
              </el-table-column>
              <el-table-column label="范围" width="90">
                <template #default="{ row }">
                  <span class="table-tag">{{ scopeText(row.scope) }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="ownerName" label="所属用户" min-width="100" />
              <el-table-column label="打开" width="90">
                <template #default="{ row }">{{ openModeText(row.openMode) }}</template>
              </el-table-column>
              <el-table-column prop="sort" label="排序" width="80" />
              <el-table-column label="状态" width="120">
                <template #default="{ row }">
                  <el-switch
                    v-model="row.status"
                    :active-value="1"
                    :inactive-value="2"
                    active-text="启用"
                    inactive-text="禁用"
                    @change="changeItemStatus(row)"
                  />
                </template>
              </el-table-column>
              <el-table-column label="操作" width="150" fixed="right">
                <template #default="{ row }">
                  <div class="text-actions">
                    <button type="button" @click="openItemDialog(row)">修改</button>
                    <button type="button" class="danger" @click="deleteItem(row)">删除</button>
                  </div>
                </template>
              </el-table-column>
            </el-table>
            <div class="pagination-row">
              <el-pagination
                v-model:current-page="itemQuery.pageNum"
                v-model:page-size="itemQuery.pageSize"
                :page-sizes="[10, 20, 50, 100]"
                layout="total, sizes, prev, pager, next, jumper"
                :total="itemTotal"
                @size-change="loadItems"
                @current-change="loadItems"
              />
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </el-drawer>

    <el-dialog
      v-model="categoryDialogVisible"
      :title="categoryDialogTitle"
      width="560px"
      class="nav-dialog"
      modal-class="nav-form-dialog-overlay"
      @closed="resetCategoryForm"
    >
      <el-form ref="categoryFormRef" :model="categoryForm" :rules="categoryRules" label-position="top" class="dialog-form">
        <div class="form-panel">
          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="分类名称" prop="name">
                <el-input v-model="categoryForm.name" placeholder="例如：公共服务" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="分类编码" prop="code">
                <el-input v-model="categoryForm.code" placeholder="public_service" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="图标" prop="icon">
                <el-select v-model="categoryForm.icon" filterable clearable placeholder="选择图标">
                  <el-option v-for="item in iconOptions" :key="item" :label="item" :value="item">
                    <span class="icon-option">
                      <el-icon><component :is="item" /></el-icon>
                      {{ item }}
                    </span>
                  </el-option>
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="排序" prop="sort">
                <el-input-number v-model="categoryForm.sort" :min="0" controls-position="right" />
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <el-form-item label="描述">
                <el-input v-model="categoryForm.description" type="textarea" :rows="3" maxlength="200" show-word-limit />
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <el-form-item label="状态" prop="status">
                <el-radio-group v-model="categoryForm.status">
                  <el-radio :label="1">启用</el-radio>
                  <el-radio :label="2">禁用</el-radio>
                </el-radio-group>
              </el-form-item>
            </el-col>
          </el-row>
        </div>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button class="dialog-cancel-action" @click="categoryDialogVisible = false">取消</el-button>
          <el-button type="primary" class="primary-action" :loading="categorySubmitting" @click="submitCategory">确定</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog
      v-model="itemDialogVisible"
      :title="itemDialogTitle"
      width="760px"
      class="nav-dialog"
      modal-class="nav-form-dialog-overlay"
      @closed="resetItemForm"
    >
      <el-form ref="itemFormRef" :model="itemForm" :rules="itemRules" label-position="top" class="dialog-form">
        <div class="form-panel">
          <div class="form-section-title">基础信息</div>
          <el-row :gutter="12">
            <el-col :span="12">
              <el-form-item label="分类" prop="categoryId">
                <el-select v-model="itemForm.categoryId" filterable placeholder="请选择分类">
                  <el-option v-for="item in categoryOptions" :key="item.id" :label="item.name" :value="item.id" />
                  <template #empty>
                    <div class="select-empty-action">
                      <span>暂无启用分类</span>
                    </div>
                  </template>
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="名称" prop="name">
                <el-input v-model="itemForm.name" placeholder="例如：GitLab" />
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <el-form-item label="地址" prop="url">
                <el-input v-model="itemForm.url" placeholder="https://gitlab.example.com" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="图标">
                <div class="icon-field">
                  <el-radio-group v-model="iconMode" size="small" class="icon-mode-group">
                    <el-radio-button label="builtin">内置图标</el-radio-button>
                    <el-radio-button label="upload">上传图标</el-radio-button>
                  </el-radio-group>
                  <el-select
                    v-if="iconMode === 'builtin'"
                    v-model="itemForm.icon"
                    filterable
                    clearable
                    placeholder="选择图标"
                  >
                    <el-option v-for="item in iconOptions" :key="item" :label="item" :value="item">
                      <span class="icon-option">
                        <el-icon><component :is="resolveIcon(item)" /></el-icon>
                        {{ item }}
                      </span>
                    </el-option>
                  </el-select>
                  <div v-else class="upload-icon-box">
                    <el-upload
                      :action="uploadIconUrl"
                      :headers="uploadHeaders"
                      :show-file-list="false"
                      accept="image/*"
                      class="nav-icon-uploader"
                      :on-success="handleIconUploadSuccess"
                      :before-upload="beforeIconUpload"
                    >
                      <span class="upload-preview">
                        <img v-if="isImageIcon(itemForm.icon)" :src="itemForm.icon" alt="" />
                        <el-icon v-else><Upload /></el-icon>
                      </span>
                    </el-upload>
                    <el-input v-model="itemForm.icon" placeholder="上传后自动填入图标 URL" />
                  </div>
                </div>
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="环境">
                <el-select v-model="itemForm.environment" placeholder="环境">
                  <el-option v-for="item in environmentOptions" :key="item.value" :label="item.label" :value="item.value" />
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="标签">
                <el-input v-model="itemForm.tags" placeholder="逗号分隔" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="排序">
                <el-input-number v-model="itemForm.sort" :min="0" controls-position="right" />
              </el-form-item>
            </el-col>
            <el-col :span="24">
              <el-form-item label="描述">
                <el-input v-model="itemForm.description" type="textarea" :rows="2" maxlength="200" show-word-limit />
              </el-form-item>
            </el-col>
          </el-row>

          <div class="form-section-title">可见范围</div>
          <el-row :gutter="12">
            <el-col :span="24">
              <el-form-item label="范围" prop="scope">
                <el-radio-group v-model="itemForm.scope">
                  <el-radio-button label="public">公共</el-radio-button>
                  <el-radio-button label="private">个人</el-radio-button>
                  <el-radio-button label="acl">授权</el-radio-button>
                </el-radio-group>
              </el-form-item>
            </el-col>
            <template v-if="itemForm.scope === 'acl'">
              <el-col :span="8">
                <el-form-item label="授权用户">
                  <el-select v-model="itemForm.visibleUserIds" multiple filterable collapse-tags placeholder="选择用户">
                    <el-option v-for="user in userOptions" :key="user.id" :label="user.nickname || user.username" :value="user.id" />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="授权角色">
                  <el-select v-model="itemForm.visibleRoleIds" multiple filterable collapse-tags placeholder="选择角色">
                    <el-option v-for="role in roleOptions" :key="role.id" :label="role.roleName || role.name || role.label" :value="role.id" />
                  </el-select>
                </el-form-item>
              </el-col>
              <el-col :span="8">
                <el-form-item label="授权部门">
                  <el-select v-model="itemForm.visibleDeptIds" multiple filterable collapse-tags placeholder="选择部门">
                    <el-option v-for="dept in deptOptions" :key="dept.id" :label="dept.deptName || dept.name || dept.label" :value="dept.id" />
                  </el-select>
                </el-form-item>
              </el-col>
            </template>
          </el-row>

        </div>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button class="dialog-cancel-action" @click="itemDialogVisible = false">取消</el-button>
          <el-button type="primary" class="primary-action" :loading="itemSubmitting" @click="submitItem">确定</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import storage from '@/utils/storage'

const getTokenValue = () => {
  const token = storage.getItem("token") || ''
  return typeof token === 'object' ? (token.access_token || token.token || '') : token
}

export default {
  name: 'SystemNav',
  data() {
    return {
      portalKeyword: '',
      portalLoading: false,
      visibleGroups: [],
      portalCountGroups: [],
      activeCategoryKey: 'all',
      manageDrawerVisible: false,
      manageTab: 'item',
      categoryLoading: false,
      categoryList: [],
      enabledCategoryList: [],
      categoryQuery: {
        name: '',
        status: ''
      },
      itemLoading: false,
      itemList: [],
      itemTotal: 0,
      itemQuery: {
        pageNum: 1,
        pageSize: 10,
        categoryId: undefined,
        name: '',
        scope: '',
        status: '',
        ownerId: undefined
      },
      userOptions: [],
      roleOptions: [],
      deptOptions: [],
      categoryDialogVisible: false,
      categorySubmitting: false,
      categoryForm: this.createCategoryForm(),
      itemDialogVisible: false,
      itemSubmitting: false,
      itemForm: this.createItemForm(),
      iconMode: 'builtin',
      uploadIconUrl: '/api/v1/upload',
      uploadHeaders: {
        Authorization: "Bearer " + getTokenValue()
      },
      categoryRules: {
        name: [{ required: true, message: '请输入分类名称', trigger: 'blur' }],
        code: [{ required: true, message: '请输入分类编码', trigger: 'blur' }],
        status: [{ required: true, message: '请选择状态', trigger: 'change' }]
      },
      itemRules: {
        categoryId: [{ required: true, message: '请选择分类', trigger: 'change' }],
        name: [{ required: true, message: '请输入导航名称', trigger: 'blur' }],
        url: [{ required: true, message: '请输入导航地址', trigger: 'blur' }],
        scope: [{ required: true, message: '请选择可见范围', trigger: 'change' }]
      },
      iconOptions: [
        'Grid',
        'Menu',
        'Link',
        'Connection',
        'Monitor',
        'Platform',
        'Tools',
        'Setting',
        'FolderOpened',
        'DataBoard',
        'TrendCharts',
        'Document',
        'Tickets',
        'Lock',
        'Key',
        'User',
        'Management',
        'Cpu',
        'Coin',
        'Bell',
        'Promotion',
        'Share',
        'Star',
        'House'
      ],
      scopeOptions: [
        { value: 'public', label: '公共' },
        { value: 'private', label: '个人' },
        { value: 'acl', label: '授权' }
      ],
      environmentOptions: [
        { value: 'none', label: '无环境' },
        { value: 'dev', label: '开发' },
        { value: 'test', label: '测试' },
        { value: 'pre', label: '预发' },
        { value: 'prod', label: '生产' }
      ]
    }
  },
  computed: {
    visibleItemTotal() {
      return this.countSourceGroups.reduce((total, group) => total + (group.items?.length || 0), 0)
    },
    countSourceGroups() {
      return this.portalCountGroups.length ? this.portalCountGroups : this.visibleGroups
    },
    portalGroups() {
      const groupMap = new Map()

      this.visibleGroups.forEach(group => {
        const key = this.categoryKey(group)
        groupMap.set(key, {
          category: group.category || { id: key, name: '未分类', icon: 'FolderOpened' },
          items: Array.isArray(group.items) ? group.items : []
        })
      })

      if (!this.portalKeyword?.trim() && this.activeCategoryKey === 'all') {
        this.categoryOptions.forEach(category => {
          const key = String(category.id || category.code)
          if (!groupMap.has(key)) {
            groupMap.set(key, {
              category,
              items: []
            })
          }
        })
      }

      if (!this.portalKeyword?.trim() && this.activeCategoryKey !== 'all' && !groupMap.has(this.activeCategoryKey)) {
        const activeCategory = this.categoryOptions.find(category => this.categoryKey(category) === this.activeCategoryKey)
        if (activeCategory) {
          groupMap.set(this.activeCategoryKey, {
            category: activeCategory,
            items: []
          })
        }
      }

      return Array.from(groupMap.values())
    },
    categoryOptions() {
      return this.mergeCategories([
        ...this.enabledCategoryList,
        ...this.portalCountGroups.map(group => group.category).filter(Boolean),
        ...this.visibleGroups.map(group => group.category).filter(Boolean)
      ])
    },
    sidebarCategories() {
      return this.categoryOptions
    },
    activeCategoryId() {
      if (this.activeCategoryKey === 'all') return undefined
      const category = this.categoryOptions.find(item => this.categoryKey(item) === this.activeCategoryKey)
      return category?.id
    },
    categoryDialogTitle() {
      return this.categoryForm.id ? '修改导航分类' : '新增导航分类'
    },
    itemDialogTitle() {
      return this.itemForm.id ? '修改导航链接' : '新增导航链接'
    }
  },
  watch: {
    'itemForm.scope'(scope) {
      if (scope !== 'acl') {
        this.itemForm.visibleUserIds = []
        this.itemForm.visibleRoleIds = []
        this.itemForm.visibleDeptIds = []
      }
    }
  },
  created() {
    this.loadVisibleNav()
    this.loadEnabledCategories()
  },
  methods: {
    createCategoryForm() {
      return {
        id: undefined,
        name: '',
        code: '',
        icon: 'Grid',
        description: '',
        sort: 10,
        status: 1
      }
    },
    createItemForm() {
      return {
        id: undefined,
        categoryId: undefined,
        name: '',
        url: '',
        icon: 'Link',
        description: '',
        environment: 'none',
        tags: '',
        scope: 'public',
        visibleUserIds: [],
        visibleRoleIds: [],
        visibleDeptIds: [],
        ssoEnabled: false,
        ssoMode: 'none',
        openMode: 'blank',
        sort: 10,
        status: 1
      }
    },
    categoryKey(target) {
      const category = target?.category || target
      return String(category?.id || category?.code || 'uncategorized')
    },
    mergeCategories(categories) {
      const map = new Map()
      categories
        .filter(item => item && (item.id || item.code))
        .forEach(item => {
          const key = String(item.id || item.code)
          if (!map.has(key)) {
            map.set(key, item)
          }
        })

      return Array.from(map.values()).sort((a, b) => {
        const sortA = Number(a.sort) || 0
        const sortB = Number(b.sort) || 0
        if (sortA !== sortB) return sortA - sortB
        return Number(a.id) - Number(b.id)
      })
    },
    categoryItemCount(category) {
      const key = this.categoryKey(category)
      const group = this.countSourceGroups.find(item => this.categoryKey(item) === key)
      return group?.items?.length || 0
    },
    async selectPortalCategory(category) {
      this.activeCategoryKey = category === 'all' ? 'all' : this.categoryKey(category)
      await this.loadVisibleNav()
      this.$refs.portalScrollRef?.scrollTo?.({ top: 0, behavior: 'smooth' })
    },
    async refreshPortal() {
      await this.loadVisibleNav()
      this.$message.success('导航已刷新')
    },
    async loadVisibleNav() {
      this.portalLoading = true
      try {
        const params = {}
        const keyword = this.portalKeyword?.trim()
        if (keyword) params.keyword = keyword
        if (this.activeCategoryId) params.categoryId = this.activeCategoryId

        const countParams = keyword ? { keyword } : {}
        const requests = [this.$api.getNavVisible(params)]
        if (this.activeCategoryId) {
          requests.push(this.$api.getNavVisible(countParams))
        }

        const [visibleResponse, countResponse] = await Promise.all(requests)
        const res = visibleResponse.data
        if (res.code === 200) {
          this.visibleGroups = Array.isArray(res.data) ? res.data : []
          if (!this.activeCategoryId) {
            this.portalCountGroups = this.visibleGroups
          } else if (countResponse?.data?.code === 200) {
            this.portalCountGroups = Array.isArray(countResponse.data.data) ? countResponse.data.data : []
          }
          if (this.activeCategoryKey !== 'all' && !this.categoryOptions.some(category => this.categoryKey(category) === this.activeCategoryKey)) {
            this.activeCategoryKey = 'all'
          }
        } else {
          this.$message.error(res.message || '获取可见导航失败')
        }
      } catch (error) {
        console.error('获取可见导航失败:', error)
      } finally {
        this.portalLoading = false
      }
    },
    openManageDrawer() {
      this.manageDrawerVisible = true
    },
    async ensureFormOptionsLoaded() {
      await Promise.all([
        this.enabledCategoryList.length ? Promise.resolve() : this.loadEnabledCategories(),
        this.userOptions.length || this.roleOptions.length || this.deptOptions.length ? Promise.resolve() : this.loadAclOptions()
      ])
    },
    async loadManageData() {
      await Promise.all([
        this.loadCategories(),
        this.loadItems()
      ])
    },
    async loadCategories() {
      this.categoryLoading = true
      try {
        const params = {
          name: this.categoryQuery.name?.trim(),
          status: this.categoryQuery.status
        }
        const { data: res } = await this.$api.getNavCategoryList(params)
        if (res.code === 200) {
          this.categoryList = Array.isArray(res.data) ? res.data : []
          if (!this.categoryQuery.name && (!this.categoryQuery.status || Number(this.categoryQuery.status) === 1)) {
            this.enabledCategoryList = this.categoryList.filter(item => Number(item.status) === 1)
          }
        } else {
          this.$message.error(res.message || '获取导航分类失败')
        }
      } catch (error) {
        console.error('获取导航分类失败:', error)
      } finally {
        this.categoryLoading = false
      }
    },
    async loadEnabledCategories() {
      try {
        const { data: res } = await this.$api.getNavCategoryList({ status: '1' })
        if (res.code === 200) {
          this.enabledCategoryList = Array.isArray(res.data) ? res.data : []
        } else {
          this.enabledCategoryList = []
          this.$message.error(res.message || '获取启用分类失败')
        }
      } catch (error) {
        this.enabledCategoryList = []
        console.error('获取启用分类失败:', error)
      }
    },
    async loadItems() {
      this.itemLoading = true
      try {
        const params = {
          ...this.itemQuery,
          name: this.itemQuery.name?.trim()
        }
        const { data: res } = await this.$api.getNavItemList(params)
        if (res.code === 200) {
          this.itemList = Array.isArray(res.data?.list) ? res.data.list : []
          this.itemTotal = Number(res.data?.total) || 0
          this.itemQuery.pageNum = Number(res.data?.pageNum) || this.itemQuery.pageNum
          this.itemQuery.pageSize = Number(res.data?.pageSize) || this.itemQuery.pageSize
        } else {
          this.$message.error(res.message || '获取导航链接失败')
        }
      } catch (error) {
        console.error('获取导航链接失败:', error)
      } finally {
        this.itemLoading = false
      }
    },
    async loadAclOptions() {
      try {
        const [userRes, roleRes, deptRes] = await Promise.all([
          this.$api.queryAdminList({ pageNum: 1, pageSize: 1000 }),
          this.$api.querySysRoleVoList(),
          this.$api.queryDeptList({})
        ])
        if (userRes.data.code === 200) {
          this.userOptions = Array.isArray(userRes.data.data?.list) ? userRes.data.data.list : []
        }
        if (roleRes.data.code === 200) {
          this.roleOptions = Array.isArray(roleRes.data.data) ? roleRes.data.data : []
        }
        if (deptRes.data.code === 200) {
          this.deptOptions = this.flattenDeptList(Array.isArray(deptRes.data.data) ? deptRes.data.data : [])
        }
      } catch (error) {
        console.error('加载授权选项失败:', error)
      }
    },
    flattenDeptList(list) {
      const result = []
      const walk = (nodes) => {
        nodes.forEach(node => {
          result.push(node)
          if (Array.isArray(node.children)) {
            walk(node.children)
          }
        })
      }
      walk(this.$handleTree?.handleTree ? this.$handleTree.handleTree(list, 'id') : list)
      return result
    },
    resetCategoryQuery() {
      this.categoryQuery = {
        name: '',
        status: ''
      }
      this.loadCategories()
    },
    resetItemQuery() {
      this.itemQuery = {
        pageNum: 1,
        pageSize: 10,
        categoryId: undefined,
        name: '',
        scope: '',
        status: '',
        ownerId: undefined
      }
      this.loadItems()
    },
    openCategoryDialog(row) {
      this.categoryForm = row ? { ...row } : this.createCategoryForm()
      this.categoryDialogVisible = true
    },
    resetCategoryForm() {
      this.categoryForm = this.createCategoryForm()
      this.$refs.categoryFormRef?.clearValidate?.()
    },
    async submitCategory() {
      const valid = await this.$refs.categoryFormRef.validate().catch(() => false)
      if (!valid) return

      this.categorySubmitting = true
      try {
        const request = this.categoryForm.id ? this.$api.updateNavCategory : this.$api.addNavCategory
        const { data: res } = await request(this.categoryForm)
        if (res.code === 200) {
          this.$message.success(this.categoryForm.id ? '修改分类成功' : '新增分类成功')
          this.categoryDialogVisible = false
          await this.loadCategories()
          await this.loadVisibleNav()
        } else {
          this.$message.error(res.message || '保存分类失败')
        }
      } catch (error) {
        console.error('保存分类失败:', error)
      } finally {
        this.categorySubmitting = false
      }
    },
    async deleteCategory(row) {
      const confirmResult = await this.$confirm(`确认删除分类 "${row.name}"？`, '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).catch(error => error)
      if (confirmResult !== 'confirm') return

      const { data: res } = await this.$api.deleteNavCategory(row.id)
      if (res.code === 200) {
        this.$message.success('删除分类成功')
        await this.loadCategories()
        await this.loadVisibleNav()
      } else {
        this.$message.error(res.message || '删除分类失败')
      }
    },
    async changeCategoryStatus(row) {
      const { data: res } = await this.$api.updateNavCategory({ ...row })
      if (res.code === 200) {
        this.$message.success('状态已更新')
        await this.loadVisibleNav()
      } else {
        this.$message.error(res.message || '状态更新失败')
        await this.loadCategories()
      }
    },
    async openItemDialog(row) {
      await this.ensureFormOptionsLoaded()
      this.iconMode = row?.icon && this.isImageIcon(row.icon) ? 'upload' : 'builtin'
      this.itemForm = row
        ? {
            ...this.createItemForm(),
            ...row,
            visibleUserIds: Array.isArray(row.visibleUserIds) ? row.visibleUserIds : [],
            visibleRoleIds: Array.isArray(row.visibleRoleIds) ? row.visibleRoleIds : [],
            visibleDeptIds: Array.isArray(row.visibleDeptIds) ? row.visibleDeptIds : [],
            ssoEnabled: false,
            ssoMode: 'none'
          }
        : this.createItemForm()
      this.itemDialogVisible = true
    },
    resetItemForm() {
      this.itemForm = this.createItemForm()
      this.iconMode = 'builtin'
      this.$refs.itemFormRef?.clearValidate?.()
    },
    buildItemPayload() {
      const payload = { ...this.itemForm }
      if (payload.scope !== 'acl') {
        payload.visibleUserIds = []
        payload.visibleRoleIds = []
        payload.visibleDeptIds = []
      }
      payload.ssoEnabled = false
      payload.ssoMode = 'none'
      payload.url = this.normalizeUrl(payload.url)
      payload.openMode = payload.openMode || 'blank'
      payload.sort = Number(payload.sort) || 10
      payload.status = Number(payload.status) || 1
      return payload
    },
    handleIconUploadSuccess(res) {
      if (res?.code === 200 && res?.data) {
        this.itemForm.icon = res.data
        this.$message.success('图标上传成功')
      } else {
        this.$message.error(res?.message || '图标上传失败')
      }
    },
    beforeIconUpload(file) {
      if (!file.type?.startsWith('image/')) {
        this.$message.warning('请上传图片文件')
        return false
      }
      if (file.size > 2 * 1024 * 1024) {
        this.$message.warning('图标大小不能超过 2MB')
        return false
      }
      return true
    },
    async submitItem() {
      const valid = await this.$refs.itemFormRef.validate().catch(() => false)
      if (!valid) return

      if (
        this.itemForm.scope === 'acl' &&
        !this.itemForm.visibleUserIds.length &&
        !this.itemForm.visibleRoleIds.length &&
        !this.itemForm.visibleDeptIds.length
      ) {
        this.$message.warning('授权可见至少选择用户、角色或部门中的一种')
        return
      }

      this.itemSubmitting = true
      try {
        const payload = this.buildItemPayload()
        const request = payload.id ? this.$api.updateNavItem : this.$api.addNavItem
        const { data: res } = await request(payload)
        if (res.code === 200) {
          this.$message.success(payload.id ? '修改导航成功' : '新增导航成功')
          this.itemDialogVisible = false
          await this.loadItems()
          await this.loadVisibleNav()
        } else {
          this.$message.error(res.message || '保存导航失败')
        }
      } catch (error) {
        console.error('保存导航失败:', error)
      } finally {
        this.itemSubmitting = false
      }
    },
    async deleteItem(row) {
      const confirmResult = await this.$confirm(`确认删除导航 "${row.name}"？`, '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).catch(error => error)
      if (confirmResult !== 'confirm') return

      const { data: res } = await this.$api.deleteNavItem(row.id)
      if (res.code === 200) {
        this.$message.success('删除导航成功')
        await this.loadItems()
        await this.loadVisibleNav()
      } else {
        this.$message.error(res.message || '删除导航失败')
      }
    },
    async changeItemStatus(row) {
      const { data: res } = await this.$api.updateNavItem({ ...row })
      if (res.code === 200) {
        this.$message.success('状态已更新')
        await this.loadVisibleNav()
      } else {
        this.$message.error(res.message || '状态更新失败')
        await this.loadItems()
      }
    },
    openNav(item) {
      if (!item?.url) return
      const targetUrl = this.normalizeUrl(item.url)
      if (item.openMode === 'self') {
        window.location.href = targetUrl
        return
      }
      window.open(targetUrl, '_blank', 'noopener,noreferrer')
    },
    normalizeUrl(url) {
      const value = String(url || '').trim()
      if (!value) return ''
      if (/^(https?:)?\/\//i.test(value) || /^(mailto|tel):/i.test(value)) {
        return value
      }
      return `https://${value}`
    },
    isImageIcon(icon) {
      return /^https?:\/\//.test(icon || '') || /^data:image\//.test(icon || '')
    },
    iconThemeStyle(item = {}) {
      if (this.isImageIcon(item.icon)) return {}

      const source = `${item.name || ''} ${item.code || ''} ${item.url || ''} ${item.icon || ''}`.toLowerCase()
      const namedThemes = [
        {
          test: /阿里|aliyun|alibaba|orange/,
          bg: 'linear-gradient(135deg, #ff9f3f 0%, #ff7a1a 100%)'
        },
        {
          test: /腾讯|tencent|qcloud|cloud/,
          bg: 'linear-gradient(135deg, #38bdf8 0%, #2f80ed 100%)'
        },
        {
          test: /百度|baidu/,
          bg: 'linear-gradient(135deg, #5b8cff 0%, #336dff 100%)'
        },
        {
          test: /gitlab|git/,
          bg: 'linear-gradient(135deg, #ff8a4c 0%, #f0642f 100%)'
        },
        {
          test: /jenkins/,
          bg: 'linear-gradient(135deg, #94a3b8 0%, #64748b 100%)'
        },
        {
          test: /db|数据库|mysql|redis|mongo|sql/,
          bg: 'linear-gradient(135deg, #34d399 0%, #20b976 100%)'
        }
      ]
      const matched = namedThemes.find(theme => theme.test.test(source))
      const theme = matched || this.pickIconPalette(source)

      return {
        '--icon-bg': theme.bg,
        '--icon-color': '#ffffff'
      }
    },
    pickIconPalette(source) {
      const palettes = [
        {
          bg: 'linear-gradient(135deg, #38bdf8 0%, #2f80ed 100%)'
        },
        {
          bg: 'linear-gradient(135deg, #34d399 0%, #20b976 100%)'
        },
        {
          bg: 'linear-gradient(135deg, #fbbf24 0%, #f59e0b 100%)'
        },
        {
          bg: 'linear-gradient(135deg, #7c8cff 0%, #4f6df5 100%)'
        },
        {
          bg: 'linear-gradient(135deg, #fb7185 0%, #e84f68 100%)'
        },
        {
          bg: 'linear-gradient(135deg, #94a3b8 0%, #64748b 100%)'
        }
      ]
      const seed = String(source || 'nav')
      const hash = seed.split('').reduce((total, char) => total + char.charCodeAt(0), 0)
      return palettes[hash % palettes.length]
    },
    resolveIcon(icon) {
      const aliasMap = {
        GitBranch: 'Connection',
        Braces: 'Document',
        Hammer: 'Tools',
        Server: 'Monitor'
      }
      return aliasMap[icon] || icon || 'Link'
    },
    getUrlHost(url) {
      if (!url) return '-'
      try {
        return new URL(this.normalizeUrl(url)).host
      } catch (error) {
        return url.replace(/^https?:\/\//, '').split('/')[0]
      }
    },
    cardSubText(item) {
      return item?.description || this.getUrlHost(item?.url)
    },
    scopeText(scope) {
      const map = {
        public: '公共',
        private: '个人',
        acl: '授权'
      }
      return map[scope] || '公共'
    },
    environmentText(environment) {
      const map = {
        dev: '开发',
        test: '测试',
        pre: '预发',
        prod: '生产',
        none: '无环境'
      }
      return map[environment] || '无环境'
    },
    environmentClass(environment) {
      return {
        'env-dev': environment === 'dev',
        'env-test': environment === 'test',
        'env-pre': environment === 'pre',
        'env-prod': environment === 'prod'
      }
    },
    openModeText(openMode) {
      return openMode === 'self' ? '当前页' : '新窗口'
    }
  }
}
</script>

<style scoped>
@property --card-angle {
  syntax: "<angle>";
  inherits: false;
  initial-value: 0deg;
}

.system-nav-page {
  --system-nav-height-offset: 92px;
  --page-bg: rgba(255, 255, 255, 0.66);
  --sidebar-bg: rgba(245, 249, 255, 0.8);
  --sidebar-bg-deep: rgba(237, 245, 255, 0.82);
  --surface: rgba(255, 255, 255, 0.92);
  --surface-soft: rgba(247, 250, 255, 0.86);
  --surface-warm: rgba(255, 253, 248, 0.94);
  --text-primary: #172133;
  --text-secondary: #5f6b7c;
  --text-muted: #8d97a8;
  --border-soft: rgba(22, 34, 56, 0.06);
  --border-mid: rgba(223, 231, 242, 0.95);
  --brand: #4c8bf5;
  --brand-soft: rgba(76, 139, 245, 0.12);
  --brand-fade: rgba(76, 139, 245, 0.06);
  --success: #3fb97f;
  --warning: #f3b24b;
  --danger: #ef6b73;
  height: calc(100vh - var(--system-nav-height-offset));
  max-height: calc(100vh - var(--system-nav-height-offset));
  min-height: 520px;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.72);
  border-radius: 22px;
  background:
    radial-gradient(circle at 16% 20%, rgba(255, 255, 255, 0.62), transparent 34%),
    linear-gradient(135deg, rgba(255, 255, 255, 0.76), rgba(249, 252, 255, 0.64));
  box-shadow:
    0 18px 48px rgba(17, 24, 39, 0.05),
    inset 0 1px 0 rgba(255, 255, 255, 0.88);
  color: var(--text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

:global(.home-container .el-main:has(.system-nav-page)) {
  overflow: hidden;
  padding-bottom: 0;
}

.nav-topbar {
  height: 42px;
  flex-shrink: 0;
  display: grid;
  grid-template-columns: 200px minmax(260px, 520px) 1fr;
  align-items: center;
  gap: 14px;
  padding: 0 12px;
  border-bottom: 1px solid rgba(222, 232, 245, 0.7);
  background: linear-gradient(90deg, rgba(255, 255, 255, 0.86) 0%, rgba(250, 253, 255, 0.76) 100%);
}

.brand-area {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.brand-mark {
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  color: var(--brand);
  background: rgba(255, 255, 255, 0.86);
  border: 1px solid rgba(205, 219, 244, 0.86);
}

.brand-area strong {
  display: block;
  font-size: 14px;
  line-height: 1.2;
}

.brand-area span:not(.brand-mark) {
  display: block;
  margin-top: 2px;
  color: var(--text-muted);
  font-size: 11px;
}

.top-search {
  width: 100%;
}

.top-search :deep(.el-input__wrapper) {
  min-height: 30px;
  border-radius: 13px;
}

.top-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 8px;
}

.nav-workbench {
  min-height: 0;
  flex: 1;
  display: grid;
  grid-template-columns: 200px minmax(0, 1fr);
}

.category-sidebar {
  min-height: 0;
  padding: 8px 7px;
  overflow-y: auto;
  border-right: 1px solid rgba(207, 217, 230, 0.78);
  background:
    radial-gradient(circle at 20% 8%, rgba(255, 255, 255, 0.7), transparent 34%),
    linear-gradient(180deg, var(--sidebar-bg) 0%, var(--sidebar-bg-deep) 100%);
  box-shadow:
    inset -1px 0 0 rgba(255, 255, 255, 0.68),
    10px 0 28px rgba(70, 120, 180, 0.035);
}

.category-nav-item {
  width: 100%;
  height: 34px;
  display: grid;
  grid-template-columns: 22px minmax(0, 1fr) auto;
  align-items: center;
  gap: 7px;
  margin-bottom: 4px;
  padding: 6px 8px;
  border: 1px solid transparent;
  border-radius: 12px;
  background: transparent;
  color: var(--text-secondary);
  text-align: left;
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: border-color 0.18s ease, background 0.18s ease, color 0.18s ease;
}

.category-nav-item:hover,
.category-nav-item.active {
  background: rgba(255, 255, 255, 0.92);
  border-color: rgba(197, 216, 244, 0.76);
  color: var(--brand);
}

.category-nav-item span:not(.side-icon) {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.category-nav-item em {
  min-width: 20px;
  padding: 1px 6px;
  border-radius: 999px;
  background: rgba(76, 139, 245, 0.09);
  color: #5d7fc7;
  font-style: normal;
  font-size: 10px;
  text-align: center;
}

.side-icon,
.card-icon,
.mini-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: var(--icon-color, var(--brand));
  background: var(--icon-bg, var(--brand-soft));
  border: none;
  box-shadow: none;
}

.side-icon {
  width: 22px;
  height: 22px;
  border-radius: 8px;
}

.side-icon img,
.card-icon img,
.mini-icon img {
  width: 68%;
  height: 68%;
  object-fit: contain;
}

.side-icon.is-image,
.card-icon.is-image {
  background: transparent;
  border: none;
  box-shadow: none;
}

.side-icon.is-image img {
  width: 78%;
  height: 78%;
}

.card-icon.is-image img {
  width: 84%;
  height: 84%;
}

.portal-main {
  position: relative;
  min-height: 0;
  flex: 1;
  overflow: auto;
  padding: 10px 12px 12px;
  background:
    radial-gradient(circle at 88% 18%, rgba(255, 255, 255, 0.82) 0 1px, transparent 2px 100%),
    radial-gradient(circle at 93% 30%, rgba(255, 255, 255, 0.68) 0 2px, transparent 3px 100%),
    radial-gradient(circle at 78% 16%, rgba(207, 231, 255, 0.22), transparent 22%),
    radial-gradient(circle at 22% 72%, rgba(255, 232, 207, 0.18), transparent 28%),
    linear-gradient(110deg, rgba(255, 255, 255, 0.7) 0%, rgba(252, 254, 255, 0.76) 48%, rgba(244, 251, 255, 0.72) 100%);
  scrollbar-gutter: stable;
}

.portal-main::before {
  content: "";
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    linear-gradient(rgba(89, 129, 178, 0.08) 1px, transparent 1px),
    linear-gradient(90deg, rgba(89, 129, 178, 0.08) 1px, transparent 1px),
    linear-gradient(rgba(89, 129, 178, 0.035) 1px, transparent 1px),
    linear-gradient(90deg, rgba(89, 129, 178, 0.035) 1px, transparent 1px),
    radial-gradient(circle at 75% 8%, rgba(255, 255, 255, 0.64) 0 1px, transparent 2px 100%),
    radial-gradient(circle at 82% 23%, rgba(255, 255, 255, 0.58) 0 1px, transparent 2px 100%),
    radial-gradient(circle at 92% 8%, rgba(255, 255, 255, 0.72) 0 2px, transparent 3px 100%),
    radial-gradient(circle at 68% 40%, rgba(255, 255, 255, 0.48) 0 1px, transparent 2px 100%),
    conic-gradient(from 0deg at 77% 18%, transparent 0deg, rgba(255, 255, 255, 0.34) 8deg, transparent 16deg, transparent 58deg, rgba(255, 255, 255, 0.28) 66deg, transparent 74deg, transparent 130deg, rgba(255, 255, 255, 0.26) 138deg, transparent 146deg, transparent 220deg, rgba(255, 255, 255, 0.3) 228deg, transparent 236deg, transparent 360deg);
  background-size: 40px 40px, 40px 40px, 160px 160px, 160px 160px, auto, auto, auto, auto, auto;
  animation: portalGridDrift 26s linear infinite;
  opacity: 0.58;
}

.portal-main::after {
  content: "";
  position: absolute;
  inset: 0;
  pointer-events: none;
  background-image:
    linear-gradient(45deg, transparent 47%, rgba(255, 255, 255, 0.62) 48%, rgba(255, 255, 255, 0.62) 52%, transparent 53%),
    linear-gradient(-45deg, transparent 47%, rgba(255, 255, 255, 0.62) 48%, rgba(255, 255, 255, 0.62) 52%, transparent 53%),
    linear-gradient(0deg, transparent 47%, rgba(255, 255, 255, 0.5) 48%, rgba(255, 255, 255, 0.5) 52%, transparent 53%),
    linear-gradient(90deg, transparent 47%, rgba(255, 255, 255, 0.5) 48%, rgba(255, 255, 255, 0.5) 52%, transparent 53%);
  background-size: 18px 18px, 18px 18px, 18px 18px, 18px 18px;
  background-position: 95% 18%, 95% 18%, 95% 18%, 95% 18%;
  background-repeat: no-repeat;
  opacity: 0.34;
}

.portal-motion-layer {
  position: absolute;
  inset: 0;
  z-index: 0;
  overflow: hidden;
  pointer-events: none;
}

.portal-motion-layer::before {
  content: "";
  position: absolute;
  inset: -20%;
  background:
    radial-gradient(circle at 18% 20%, rgba(255, 255, 255, 0.42) 0 1px, transparent 2px),
    radial-gradient(circle at 62% 18%, rgba(255, 255, 255, 0.5) 0 1px, transparent 2px),
    radial-gradient(circle at 84% 42%, rgba(255, 255, 255, 0.44) 0 1px, transparent 2px),
    radial-gradient(circle at 42% 70%, rgba(255, 255, 255, 0.36) 0 1px, transparent 2px);
  animation: portalStarsFloat 18s ease-in-out infinite alternate;
}

.meteor {
  position: absolute;
  top: -72px;
  left: 60%;
  width: 2px;
  height: 150px;
  border-radius: 999px;
  background: linear-gradient(180deg, rgba(103, 170, 255, 0), rgba(103, 170, 255, 0.78), rgba(255, 255, 255, 0.92));
  filter: drop-shadow(0 0 8px rgba(103, 170, 255, 0.45));
  transform: rotate(48deg);
  opacity: 0;
  animation: meteorFall 7.8s linear infinite;
}

.meteor::after {
  content: "";
  position: absolute;
  left: -2px;
  bottom: 0;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 0 14px rgba(99, 166, 255, 0.78);
}

.meteor-one {
  left: 76%;
  animation-delay: 0.8s;
}

.meteor-two {
  left: 48%;
  height: 120px;
  animation-delay: 3.4s;
  animation-duration: 9.2s;
  opacity: 0;
}

.meteor-three {
  left: 90%;
  height: 100px;
  animation-delay: 5.8s;
  animation-duration: 10.5s;
  background: linear-gradient(180deg, rgba(255, 189, 126, 0), rgba(255, 189, 126, 0.5), rgba(255, 255, 255, 0.86));
}

.meteor-four {
  left: 28%;
  height: 92px;
  animation-delay: 1.9s;
  animation-duration: 8.6s;
  --meteor-x: -300px;
  --meteor-y: 420px;
  background: linear-gradient(180deg, rgba(132, 204, 255, 0), rgba(132, 204, 255, 0.42), rgba(255, 255, 255, 0.78));
  filter: drop-shadow(0 0 7px rgba(132, 204, 255, 0.38));
}

.meteor-five {
  left: 64%;
  height: 138px;
  animation-delay: 4.7s;
  animation-duration: 7.2s;
  --meteor-x: -520px;
  --meteor-y: 560px;
  background: linear-gradient(180deg, rgba(176, 160, 255, 0), rgba(176, 160, 255, 0.5), rgba(255, 255, 255, 0.88));
  filter: drop-shadow(0 0 9px rgba(176, 160, 255, 0.34));
}

.meteor-six {
  left: 38%;
  height: 74px;
  animation-delay: 6.6s;
  animation-duration: 11.2s;
  opacity: 0;
  --meteor-x: -240px;
  --meteor-y: 360px;
  background: linear-gradient(180deg, rgba(255, 213, 168, 0), rgba(255, 213, 168, 0.36), rgba(255, 255, 255, 0.76));
}

.meteor-seven {
  left: 82%;
  height: 116px;
  animation-delay: 8.1s;
  animation-duration: 9.8s;
  --meteor-x: -460px;
  --meteor-y: 500px;
  background: linear-gradient(180deg, rgba(103, 170, 255, 0), rgba(103, 170, 255, 0.52), rgba(255, 255, 255, 0.86));
}

.portal-bg-visual {
  position: absolute;
  top: 36px;
  right: 56px;
  width: 250px;
  height: 190px;
  pointer-events: none;
  z-index: 0;
  opacity: 0.24;
}

.cloud-cube,
.cloud-arrow,
.spark {
  position: absolute;
  display: block;
}

.cloud-cube {
  border-radius: 28px;
  background:
    linear-gradient(145deg, rgba(255, 255, 255, 0.86), rgba(190, 197, 255, 0.34) 50%, rgba(121, 203, 255, 0.2));
  border: 1px solid rgba(255, 255, 255, 0.78);
  box-shadow:
    20px 20px 42px rgba(74, 133, 205, 0.14),
    inset 10px 12px 20px rgba(255, 255, 255, 0.72),
    inset -12px -14px 22px rgba(131, 116, 240, 0.14);
  transform: rotate(36deg) skew(-8deg, -8deg);
}

.cube-main {
  right: 42px;
  top: 52px;
  width: 128px;
  height: 92px;
}

.cube-top {
  right: 122px;
  top: 24px;
  width: 78px;
  height: 64px;
}

.cube-side {
  right: 18px;
  top: 20px;
  width: 76px;
  height: 70px;
}

.cloud-arrow {
  right: 88px;
  top: 30px;
  width: 72px;
  height: 118px;
  border-radius: 22px;
  background: linear-gradient(155deg, rgba(255, 255, 255, 0.92), rgba(203, 206, 255, 0.48));
  box-shadow:
    inset 10px 12px 18px rgba(255, 255, 255, 0.78),
    inset -10px -14px 18px rgba(126, 116, 235, 0.16);
  clip-path: polygon(30% 0, 70% 0, 70% 58%, 100% 58%, 50% 100%, 0 58%, 30% 58%);
  transform: rotate(-38deg);
}

.spark {
  width: 72px;
  height: 72px;
  border-radius: 50%;
  background:
    linear-gradient(0deg, transparent 46%, rgba(255, 255, 255, 0.62) 48%, rgba(255, 255, 255, 0.62) 52%, transparent 54%),
    linear-gradient(60deg, transparent 46%, rgba(255, 255, 255, 0.5) 48%, rgba(255, 255, 255, 0.5) 52%, transparent 54%),
    linear-gradient(120deg, transparent 46%, rgba(255, 255, 255, 0.5) 48%, rgba(255, 255, 255, 0.5) 52%, transparent 54%);
}

.spark-a {
  right: 172px;
  top: 4px;
  transform: scale(1.2);
}

.spark-b {
  right: 8px;
  top: 72px;
  transform: scale(0.7) rotate(20deg);
  opacity: 0.72;
}

.spark-c {
  right: 204px;
  top: 112px;
  transform: scale(0.52) rotate(-18deg);
  opacity: 0.58;
}

.category-sidebar,
.portal-main {
  scrollbar-width: thin;
  scrollbar-color: rgba(142, 157, 180, 0.42) transparent;
}

.category-sidebar::-webkit-scrollbar,
.portal-main::-webkit-scrollbar {
  width: 6px;
  height: 6px;
}

.category-sidebar::-webkit-scrollbar-thumb,
.portal-main::-webkit-scrollbar-thumb {
  border-radius: 999px;
  background: rgba(142, 157, 180, 0.36);
}

.category-sidebar::-webkit-scrollbar-track,
.portal-main::-webkit-scrollbar-track {
  background: transparent;
}

.portal-category-section {
  position: relative;
  z-index: 1;
  scroll-margin-top: 10px;
  margin-bottom: 12px;
}

.portal-category-section h2 {
  margin: 0 0 7px;
  color: #37465a;
  font-size: 13px;
  font-weight: 700;
}

.portal-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(172px, 196px));
  gap: 8px;
}

.portal-nav-card {
  position: relative;
  z-index: 1;
  overflow: hidden;
  height: 52px;
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr);
  align-items: center;
  gap: 8px;
  padding: 7px 9px;
  border-radius: 14px;
  border: 1px solid rgba(214, 223, 235, 0.92);
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.96), rgba(250, 253, 255, 0.92));
  box-shadow: 0 8px 20px rgba(15, 23, 42, 0.035);
  cursor: pointer;
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease, background 0.2s ease;
}

.portal-nav-card:hover {
  transform: translateY(-3px);
  border-color: rgba(89, 147, 246, 0.72);
  background: rgba(255, 255, 255, 0.96);
  box-shadow:
    0 14px 30px rgba(57, 104, 167, 0.12),
    0 0 0 1px rgba(255, 255, 255, 0.72) inset;
}

.portal-nav-card::before {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: inherit;
  padding: 1px;
  background: conic-gradient(from var(--card-angle, 0deg), transparent 0deg, transparent 56deg, rgba(90, 147, 246, 0.96) 86deg, rgba(52, 211, 153, 0.82) 120deg, transparent 154deg, transparent 360deg);
  opacity: 0;
  pointer-events: none;
  -webkit-mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0);
  -webkit-mask-composite: xor;
  mask-composite: exclude;
}

.portal-nav-card::after {
  content: "";
  position: absolute;
  inset: -55% -40%;
  background: linear-gradient(115deg, transparent 35%, rgba(255, 255, 255, 0.58) 50%, transparent 65%);
  transform: translateX(-42%) rotate(8deg);
  opacity: 0;
  pointer-events: none;
}

.portal-nav-card:hover::before {
  opacity: 1;
  animation: cardBorderSpin 1.6s linear infinite;
}

.portal-nav-card:hover::after {
  opacity: 1;
  animation: cardLightSweep 0.9s ease forwards;
}

.portal-nav-card:hover .card-icon {
  transform: scale(1.06);
  box-shadow: 0 8px 18px rgba(57, 104, 167, 0.14);
}

.card-icon {
  width: 34px;
  height: 34px;
  border-radius: 12px;
  font-size: 19px;
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

.card-icon :deep(.el-icon),
.side-icon :deep(.el-icon) {
  color: currentColor;
}

.card-main {
  min-width: 0;
}

.card-title-row {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.card-title-row strong {
  min-width: 0;
  overflow: hidden;
  color: var(--text-primary);
  font-size: 12px;
  line-height: 1.3;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-sub-row {
  margin: 1px 0 0;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.card-sub-row span {
  min-width: 0;
  flex: 0 1 auto;
  overflow: hidden;
  color: #2f80ed;
  font-size: 10px;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-sub-row em {
  flex-shrink: 0;
  color: var(--text-muted);
  font-style: normal;
  font-size: 10px;
  line-height: 1.35;
}

.card-meta {
  display: none;
}

.mini-badge,
.table-tag {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 2px 6px;
  border-radius: 999px;
  color: #5d7fc7;
  background: rgba(76, 139, 245, 0.09);
  font-size: 10px;
  font-weight: 700;
  white-space: nowrap;
}

.portal-empty {
  margin-top: 56px;
}

.category-empty-line {
  display: inline-flex;
  align-items: center;
  min-height: 28px;
  padding: 0 10px;
  border-radius: 999px;
  color: var(--text-muted);
  background: rgba(255, 255, 255, 0.58);
  border: 1px solid rgba(223, 231, 242, 0.7);
  font-size: 12px;
}

.table-toolbar {
  padding: 12px;
  margin-bottom: 10px;
  border-radius: 16px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  background: rgba(255, 255, 255, 0.72);
}

.query-form {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.query-form :deep(.el-form-item) {
  margin: 0;
}

.table-shell {
  min-height: 0;
  display: flex;
  flex-direction: column;
  border-radius: 16px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  background: rgba(255, 255, 255, 0.82);
  overflow: hidden;
}

.data-table {
  flex: 1;
}

.data-table :deep(.el-table__header th) {
  background: rgba(248, 250, 255, 0.96) !important;
  color: var(--text-secondary);
  font-weight: 700;
}

.data-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: rgba(76, 139, 245, 0.04);
}

.name-cell {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.name-cell strong {
  display: block;
  overflow: hidden;
  color: var(--text-primary);
  font-size: 14px;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.name-cell span:not(.mini-icon) {
  display: block;
  overflow: hidden;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mini-icon {
  width: 32px;
  height: 32px;
  border-radius: 12px;
}

.mono-text {
  display: inline-block;
  max-width: 100%;
  color: var(--text-secondary);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 12px;
}

.url-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  vertical-align: middle;
}

.text-actions {
  display: flex;
  align-items: center;
  gap: 14px;
}

.text-actions button {
  padding: 0;
  border: none;
  background: transparent;
  color: var(--brand);
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.text-actions .danger {
  color: var(--danger);
}

.pagination-row {
  display: flex;
  justify-content: flex-end;
  padding: 12px 14px;
  border-top: 1px solid rgba(223, 231, 242, 0.8);
}

.icon-action,
.aux-action,
.primary-action,
.success-action,
.warning-action {
  min-height: 30px;
  border-radius: 12px;
  font-weight: 600;
  border: none;
}

.icon-action {
  width: 32px;
  padding: 0;
  color: var(--text-secondary);
  background: #fff;
  border: 1px solid rgba(214, 223, 235, 0.95);
}

.aux-action {
  color: var(--text-secondary);
  background: rgba(255, 255, 255, 0.86);
  border: 1px solid rgba(214, 223, 235, 0.95);
  box-shadow: none;
  padding: 0 12px;
}

.aux-action:hover,
.icon-action:hover {
  color: var(--brand);
  border-color: rgba(147, 174, 224, 0.78);
  background: #fff;
}

.refresh-action,
.refresh-action:hover,
.refresh-action:focus {
  color: #fff;
  border-color: transparent;
  background: linear-gradient(180deg, #f7be60 0%, #f3b24b 100%);
  box-shadow: 0 8px 16px rgba(243, 178, 75, 0.18);
}

.add-action,
.add-action:hover,
.add-action:focus {
  color: #fff;
  border-color: transparent;
  background: linear-gradient(180deg, #55c88f 0%, #3fb97f 100%);
  box-shadow: 0 8px 16px rgba(63, 185, 127, 0.18);
}

.manage-action,
.manage-action:hover,
.manage-action:focus {
  color: #fff;
  border-color: transparent;
  background: linear-gradient(180deg, #69a0fa 0%, #5a93f6 100%);
  box-shadow: 0 8px 16px rgba(76, 139, 245, 0.18);
}

.refresh-action.is-disabled,
.refresh-action.is-loading,
.add-action.is-disabled,
.manage-action.is-disabled {
  opacity: 0.72;
}

.primary-action {
  background: linear-gradient(180deg, #69a0fa 0%, #5a93f6 100%);
  color: #fff;
}

.success-action {
  background: linear-gradient(180deg, #55c88f 0%, #3fb97f 100%);
  color: #fff;
}

.warning-action {
  background: linear-gradient(180deg, #f7be60 0%, #f3b24b 100%);
  color: #fff;
}

:deep(.el-input__wrapper),
:deep(.el-select__wrapper),
:deep(.el-textarea__inner) {
  border-radius: 12px;
  background: rgba(252, 253, 255, 0.98);
  border: 1px solid rgba(214, 223, 235, 0.95);
  box-shadow: none;
}

:deep(.el-input__wrapper:hover),
:deep(.el-select__wrapper:hover),
:deep(.el-textarea__inner:hover) {
  border-color: rgba(171, 189, 220, 0.95);
}

:deep(.el-input__wrapper.is-focus),
:deep(.el-select__wrapper.is-focused),
:deep(.el-textarea__inner:focus) {
  border-color: rgba(120, 156, 221, 0.9);
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
}

.manage-drawer :deep(.el-drawer) {
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
}

.manage-drawer :deep(.el-drawer__header) {
  margin-bottom: 0;
  padding: 16px 20px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.94);
}

.manage-drawer :deep(.el-drawer__body) {
  min-height: 0;
  padding: 14px;
  overflow: auto;
}

.nav-dialog :deep(.el-dialog) {
  border-radius: 18px;
  overflow: hidden;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.96));
  border: 1px solid rgba(223, 231, 242, 0.96);
  box-shadow: 0 24px 80px rgba(15, 23, 42, 0.12);
}

.nav-dialog :deep(.el-dialog__header) {
  margin: 0;
  padding: 12px 16px 10px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.94);
}

.nav-dialog :deep(.el-dialog__title) {
  color: var(--text-primary);
  font-size: 16px;
  font-weight: 700;
}

.nav-dialog :deep(.el-dialog__body) {
  padding: 10px 14px;
}

.nav-dialog :deep(.el-dialog__footer) {
  padding: 10px 14px 12px;
  border-top: 1px solid rgba(223, 231, 242, 0.82);
}

.dialog-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  flex-wrap: wrap;
}

.dialog-footer .el-button {
  min-height: 32px;
  padding: 0 16px;
  border-radius: 9px;
}

.form-panel {
  padding: 12px;
  border-radius: 16px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  background: rgba(255, 255, 255, 0.72);
}

.form-section-title {
  margin: 0 0 8px;
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 700;
}

.form-section-title:not(:first-child) {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px solid rgba(223, 231, 242, 0.82);
}

.dialog-form :deep(.el-form-item) {
  margin-bottom: 10px;
}

.dialog-form :deep(.el-form-item__label) {
  margin-bottom: 4px;
  padding-bottom: 0;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.25;
  font-weight: 600;
}

.dialog-form :deep(.el-form-item__content) {
  line-height: 1.2;
}

.dialog-form :deep(.el-select),
.dialog-form :deep(.el-input-number) {
  width: 100%;
}

.dialog-form :deep(.el-input__wrapper),
.dialog-form :deep(.el-select__wrapper),
.dialog-form :deep(.el-input-number .el-input__wrapper) {
  min-height: 32px;
  border-radius: 10px;
}

.dialog-form :deep(.el-input__inner) {
  height: 30px;
  line-height: 30px;
  font-size: 12px;
}

.dialog-form :deep(.el-textarea__inner) {
  min-height: 56px !important;
  border-radius: 10px;
  font-size: 12px;
}

.dialog-form :deep(.el-radio-button__inner) {
  height: 30px;
  padding: 7px 14px;
  line-height: 14px;
}

.dialog-form :deep(.el-input-number .el-input-number__decrease),
.dialog-form :deep(.el-input-number .el-input-number__increase) {
  height: 15px;
  line-height: 15px;
}

.icon-field {
  display: grid;
  gap: 5px;
}

.icon-mode-group {
  height: 26px;
}

.icon-mode-group :deep(.el-radio-button__inner) {
  height: 26px;
  padding: 5px 11px;
  line-height: 14px;
}

.upload-icon-box {
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr);
  align-items: center;
  gap: 8px;
}

.upload-preview {
  width: 32px;
  height: 32px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  background: rgba(247, 250, 255, 0.92);
  border: 1px dashed rgba(171, 189, 220, 0.95);
  color: var(--brand);
}

.upload-preview img {
  width: 22px;
  height: 22px;
  object-fit: contain;
}

.icon-option {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.select-empty-action {
  display: grid;
  justify-items: center;
  gap: 6px;
  padding: 10px 12px;
  color: var(--text-muted);
  font-size: 13px;
}

.env-dev {
  color: #32936f;
  background: rgba(63, 185, 127, 0.12);
}

.env-test {
  color: #5d7fc7;
  background: rgba(76, 139, 245, 0.1);
}

.env-pre {
  color: #b98531;
  background: rgba(243, 178, 75, 0.14);
}

.env-prod {
  color: #cf5f69;
  background: rgba(239, 107, 115, 0.12);
}

.table-tag.enabled {
  color: #32936f;
  background: rgba(63, 185, 127, 0.12);
}

@media (max-width: 1100px) {
  .portal-bg-visual {
    right: 18px;
    transform: scale(0.78);
    transform-origin: top right;
  }

  .nav-topbar {
    grid-template-columns: 180px minmax(220px, 1fr);
  }

  .top-actions {
    grid-column: 1 / -1;
    justify-content: flex-start;
  }

  .nav-topbar {
    height: auto;
    padding: 8px 12px;
  }
}

@media (max-width: 760px) {
  .nav-topbar,
  .nav-workbench {
    grid-template-columns: 1fr;
  }

  .category-sidebar {
    display: flex;
    gap: 8px;
    overflow-x: auto;
    border-right: none;
    border-bottom: 1px solid #d7e2f0;
  }

  .category-nav-item {
    width: auto;
    min-width: 140px;
  }

  .portal-main {
    height: auto;
    min-height: calc(100vh - 190px);
  }

  .portal-bg-visual {
    top: 82px;
    right: -72px;
    opacity: 0.28;
    transform: scale(0.58);
  }

  .portal-card-grid {
    grid-template-columns: 1fr;
  }

  .top-actions,
  .top-actions .el-button,
  .query-form,
  .query-form :deep(.el-form-item),
  .query-form :deep(.el-input),
  .query-form :deep(.el-select),
  .dialog-footer .el-button {
    width: 100%;
  }

  .manage-drawer :deep(.el-drawer) {
    width: 100% !important;
  }

  .nav-dialog :deep(.el-dialog) {
    width: calc(100vw - 20px) !important;
  }
}

@keyframes portalGridDrift {
  0% {
    background-position: 0 0, 0 0, 0 0, 0 0, 0 0, 0 0, 0 0, 0 0, 0 0;
  }
  100% {
    background-position: 40px 40px, 40px 40px, 80px 80px, 80px 80px, 0 0, 0 0, 0 0, 0 0, 0 0;
  }
}

@keyframes portalStarsFloat {
  0% {
    transform: translate3d(0, 0, 0);
    opacity: 0.58;
  }
  100% {
    transform: translate3d(-18px, 14px, 0);
    opacity: 0.86;
  }
}

@keyframes meteorFall {
  0% {
    transform: translate3d(0, -80px, 0) rotate(48deg);
    opacity: 0;
  }
  8% {
    opacity: 0.76;
  }
  34% {
    opacity: 0.76;
  }
  48% {
    transform: translate3d(var(--meteor-x, -420px), var(--meteor-y, 520px), 0) rotate(48deg);
    opacity: 0;
  }
  100% {
    transform: translate3d(var(--meteor-x, -420px), var(--meteor-y, 520px), 0) rotate(48deg);
    opacity: 0;
  }
}

@keyframes cardBorderSpin {
  to {
    --card-angle: 360deg;
  }
}

@keyframes cardLightSweep {
  0% {
    transform: translateX(-42%) rotate(8deg);
    opacity: 0;
  }
  35% {
    opacity: 0.88;
  }
  100% {
    transform: translateX(42%) rotate(8deg);
    opacity: 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .portal-main::before,
  .portal-motion-layer::before,
  .meteor,
  .portal-nav-card:hover::before,
  .portal-nav-card:hover::after {
    animation: none !important;
  }

  .portal-nav-card,
  .card-icon {
    transition: none !important;
  }
}

/* DESIGN.md compact navigation workbench */
:global(.home-container .el-main:has(.system-nav-page)) {
  padding-top: var(--ops-shell-gutter, 10px);
  padding-right: var(--ops-shell-gutter, 10px);
  padding-left: var(--ops-shell-gutter, 10px);
  overflow: hidden;
}

.system-nav-page {
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
  gap: 8px;
  height: calc(100vh - 124px);
  max-height: none;
  min-height: 0;
  padding: 7px;
  overflow: hidden;
  border: 0;
  border-radius: 0;
  background: transparent;
  box-shadow: none;
  color: var(--ops-text-primary);
}

.nav-topbar {
  display: grid;
  grid-template-columns: minmax(350px, 1fr) minmax(260px, 480px) auto;
  align-items: center;
  gap: 12px;
  height: auto;
  min-width: 0;
  padding: 8px 12px;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 10px 28px rgba(80, 124, 180, 0.1);
}

.brand-area {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.brand-mark {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  place-items: center;
  border: 0;
  border-radius: 0;
  background: transparent;
  color: #2563eb;
}

.brand-mark :deep(.el-icon) {
  font-size: 30px;
}

.brand-copy {
  display: flex;
  flex: 1;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.brand-area .brand-copy strong {
  flex: 0 0 auto;
  color: var(--ops-text-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: 0;
  white-space: nowrap;
}

.brand-area .brand-divider {
  display: block;
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  margin: 0;
  background: var(--ops-border-soft);
}

.brand-area .brand-description {
  display: block;
  min-width: 0;
  margin: 0;
  overflow: hidden;
  color: var(--ops-text-muted);
  font-size: 12px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.top-search :deep(.el-input__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: var(--ops-bg-surface-soft);
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.top-search :deep(.el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.top-search :deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 1px #409eff inset;
}

.top-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  white-space: nowrap;
}

.top-actions .icon-action,
.top-actions .aux-action {
  height: 30px;
  min-height: 30px;
  padding: 0 10px;
  border-radius: 8px;
  color: #ffffff;
  font-size: 12px;
  box-shadow: none;
}

.top-actions .icon-action {
  width: 30px;
  padding: 0;
}

.top-actions .refresh-action,
.top-actions .refresh-action:hover,
.top-actions .refresh-action:focus {
  border-color: #f3c66b;
  background: #f3c66b;
  color: #ffffff;
  box-shadow: none;
}

.top-actions .add-action,
.top-actions .add-action:hover,
.top-actions .add-action:focus {
  border-color: #67c23a;
  background: #67c23a;
  color: #ffffff;
  box-shadow: none;
}

.top-actions .manage-action,
.top-actions .manage-action:hover,
.top-actions .manage-action:focus {
  border-color: #409eff;
  background: #409eff;
  color: #ffffff;
  box-shadow: none;
}

.top-actions .el-button:hover,
.top-actions .el-button:active {
  transform: none;
}

.nav-workbench {
  min-width: 0;
  min-height: 0;
  display: grid;
  grid-template-columns: 190px minmax(0, 1fr);
  overflow: hidden;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: var(--ops-bg-surface);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
}

.category-sidebar {
  min-width: 0;
  min-height: 0;
  padding: 8px;
  overflow-y: auto;
  border-right: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-subtle);
  box-shadow: none;
}

.category-nav-item {
  height: 36px;
  margin-bottom: 4px;
  padding: 6px 8px;
  border: 1px solid transparent;
  border-radius: 8px;
  color: var(--ops-text-secondary);
  font-size: 12px;
  transition: color 160ms ease, border-color 160ms ease, background-color 160ms ease;
}

.category-nav-item:hover {
  border-color: #d9e5f2;
  background: #ffffff;
  color: #2563eb;
}

.category-nav-item.active {
  border-color: #b9d4ff;
  background: #eef6ff;
  color: #2563eb;
}

.category-nav-item:focus-visible,
.portal-nav-card:focus-visible {
  outline: 2px solid rgba(64, 158, 255, 0.45);
  outline-offset: 1px;
}

.side-icon {
  width: 22px;
  height: 22px;
  border-radius: 7px;
}

.portal-main {
  position: relative;
  min-width: 0;
  min-height: 0;
  padding: 10px;
  overflow: auto;
  background: var(--ops-bg-surface-soft);
  scrollbar-gutter: stable;
}

.portal-main::before,
.portal-main::after,
.portal-motion-layer,
.portal-bg-visual {
  display: none;
}

.portal-category-section {
  position: relative;
  z-index: 1;
  margin-bottom: 12px;
  scroll-margin-top: 10px;
}

.portal-category-section:last-child {
  margin-bottom: 0;
}

.portal-category-section h2 {
  margin: 0 0 7px;
  color: var(--ops-text-primary);
  font-size: 13px;
  font-weight: 700;
  line-height: 1.35;
}

.portal-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
  gap: 8px;
}

.portal-nav-card {
  position: relative;
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr);
  align-items: center;
  gap: 8px;
  height: 58px;
  padding: 8px 10px;
  overflow: hidden;
  border: 1px solid var(--ops-border-soft);
  border-radius: 10px;
  background: #ffffff;
  box-shadow: none;
  transition: border-color 160ms ease, box-shadow 160ms ease, background-color 160ms ease;
}

.portal-nav-card:hover {
  transform: none;
  border-color: #b9d4ff;
  background: #ffffff;
  box-shadow: 0 8px 18px rgba(64, 158, 255, 0.08);
}

.portal-nav-card::before,
.portal-nav-card::after {
  display: none;
}

.portal-nav-card:hover .card-icon {
  transform: none;
  box-shadow: none;
}

.card-icon {
  width: 34px;
  height: 34px;
  border-radius: 10px;
  font-size: 18px;
  transition: none;
}

.card-title-row strong {
  color: var(--ops-text-primary);
  font-size: 12px;
  font-weight: 700;
}

.card-sub-row span {
  color: #2563eb;
}

.card-sub-row em {
  color: var(--ops-text-muted);
}

.category-empty-line {
  min-height: 30px;
  padding: 5px 9px;
  border: 1px dashed #d9e5f2;
  border-radius: 8px;
  background: #ffffff;
  color: var(--ops-text-muted);
}

.portal-empty {
  margin-top: 40px;
}

:global(.nav-manage-drawer-overlay .el-drawer) {
  background: #ffffff;
  box-shadow: -12px 0 36px rgba(49, 92, 142, 0.12);
}

:global(.nav-manage-drawer-overlay .el-drawer__header) {
  margin: 0;
  padding: 16px 20px 12px;
  border-bottom: 1px solid #e3edf8;
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.nav-manage-drawer-overlay .el-drawer__body) {
  min-height: 0;
  padding: 12px 14px 14px;
  overflow: auto;
}

:global(.nav-manage-drawer-overlay .table-toolbar) {
  margin-bottom: 8px;
  padding: 8px 10px 2px;
  border: 1px solid #e7eef7;
  border-radius: 10px;
  background: #f3f7fc;
}

:global(.nav-manage-drawer-overlay .query-form) {
  gap: 0 10px;
}

:global(.nav-manage-drawer-overlay .query-form .el-form-item) {
  margin-right: 0;
  margin-bottom: 6px;
}

:global(.nav-manage-drawer-overlay .query-form .el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

:global(.nav-manage-drawer-overlay .el-input__wrapper),
:global(.nav-manage-drawer-overlay .el-select__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: #ffffff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.nav-manage-drawer-overlay .primary-action),
:global(.nav-manage-drawer-overlay .success-action),
:global(.nav-manage-drawer-overlay .warning-action) {
  height: 30px;
  min-height: 30px;
  padding: 0 10px;
  border-radius: 8px;
  color: #ffffff;
  font-size: 12px;
  box-shadow: none;
}

:global(.nav-manage-drawer-overlay .primary-action) {
  border-color: #409eff;
  background: #409eff;
}

:global(.nav-manage-drawer-overlay .success-action) {
  border-color: #67c23a;
  background: #67c23a;
}

:global(.nav-manage-drawer-overlay .warning-action) {
  border-color: #e6a23c;
  background: #e6a23c;
}

:global(.nav-manage-drawer-overlay .table-shell) {
  overflow: hidden;
  border: 1px solid #e7eef7;
  border-radius: 8px;
  background: #ffffff;
}

:global(.nav-manage-drawer-overlay .data-table .el-table__header th) {
  height: 38px;
  background: #f3f7fc !important;
  color: #53657d;
  font-size: 12px;
  font-weight: 700;
}

:global(.nav-manage-drawer-overlay .data-table .el-table__row td) {
  height: 40px;
  padding: 5px 0;
  font-size: 12px;
}

:global(.nav-manage-drawer-overlay .data-table .el-table__row:hover > td.el-table__cell) {
  background: #f8fafc;
}

:global(.nav-manage-drawer-overlay .pagination-row) {
  padding: 8px 12px;
  border-top: 1px solid #e7eef7;
}

:global(.nav-manage-drawer-overlay .el-button:hover),
:global(.nav-manage-drawer-overlay .el-button:active) {
  transform: none;
}

:global(.nav-form-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 32px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: #ffffff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.nav-form-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.nav-form-dialog-overlay .el-dialog__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.nav-form-dialog-overlay .el-dialog__body) {
  box-sizing: border-box;
  max-height: calc(100vh - 170px);
  padding: 16px 18px;
  overflow: auto;
}

:global(.nav-form-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #ffffff;
}

:global(.nav-form-dialog-overlay .form-panel) {
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
}

:global(.nav-form-dialog-overlay .form-section-title) {
  margin: 0 0 8px;
  color: #172033;
  font-size: 13px;
  font-weight: 700;
}

:global(.nav-form-dialog-overlay .form-section-title:not(:first-child)) {
  margin-top: 2px;
  padding-top: 10px;
  border-top: 1px solid #e7eef7;
}

:global(.nav-form-dialog-overlay .el-form-item) {
  margin-bottom: 12px;
}

:global(.nav-form-dialog-overlay .el-form-item__label) {
  margin-bottom: 6px;
  padding: 0;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.4;
}

:global(.nav-form-dialog-overlay .el-input__wrapper),
:global(.nav-form-dialog-overlay .el-select__wrapper),
:global(.nav-form-dialog-overlay .el-textarea__inner),
:global(.nav-form-dialog-overlay .el-input-number .el-input__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.nav-form-dialog-overlay .el-select),
:global(.nav-form-dialog-overlay .el-input-number) {
  width: 100%;
}

:global(.nav-form-dialog-overlay .el-textarea__inner) {
  min-height: 72px !important;
}

:global(.nav-form-dialog-overlay .dialog-footer) {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

:global(.nav-form-dialog-overlay .dialog-footer .el-button) {
  min-height: 34px;
  padding: 0 14px;
  border-radius: 8px;
  box-shadow: none;
}

:global(.nav-form-dialog-overlay .dialog-cancel-action) {
  border-color: #dbe7f4;
  background: #ffffff;
  color: #53657d;
}

:global(.nav-form-dialog-overlay .primary-action) {
  border-color: #409eff;
  background: #409eff;
  color: #ffffff;
}

:global(.nav-form-dialog-overlay .el-button:hover),
:global(.nav-form-dialog-overlay .el-button:active) {
  transform: none;
}

@media (max-width: 1100px) {
  .nav-topbar {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .top-search {
    grid-column: 1 / -1;
    grid-row: 2;
  }

  .top-actions {
    grid-column: 2;
    grid-row: 1;
  }
}

@media (max-width: 760px) {
  :global(.home-container .el-main:has(.system-nav-page)) {
    padding-right: 6px;
    padding-left: 6px;
    overflow-y: auto;
  }

  .system-nav-page {
    height: auto;
    min-height: calc(100vh - 124px);
    padding: 6px;
    overflow: visible;
  }

  .nav-topbar {
    grid-template-columns: 1fr;
    gap: 8px;
  }

  .brand-copy {
    flex-wrap: wrap;
    gap: 2px 8px;
  }

  .brand-area .brand-divider {
    display: none;
  }

  .brand-area .brand-description {
    width: 100%;
    white-space: normal;
  }

  .top-search,
  .top-actions {
    grid-column: 1;
    grid-row: auto;
    width: 100%;
  }

  .top-actions .aux-action {
    flex: 1;
  }

  .nav-workbench {
    grid-template-columns: 1fr;
    overflow: visible;
  }

  .category-sidebar {
    display: flex;
    gap: 6px;
    overflow-x: auto;
    border-right: 0;
    border-bottom: 1px solid var(--ops-border-soft);
  }

  .category-nav-item {
    width: auto;
    min-width: 140px;
    margin: 0;
  }

  .portal-main {
    min-height: 460px;
  }

  .portal-card-grid {
    grid-template-columns: 1fr;
  }

  :global(.nav-manage-drawer-overlay .el-drawer) {
    width: 100% !important;
  }

  :global(.nav-form-dialog-overlay .el-col-6),
  :global(.nav-form-dialog-overlay .el-col-8),
  :global(.nav-form-dialog-overlay .el-col-12) {
    max-width: 100%;
    flex: 0 0 100%;
  }
}
</style>
