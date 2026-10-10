<template>
  <div class="modern-menu-container">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/数据表.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>菜单管理</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>统一维护系统菜单层级、组件路径、权限标识与启用状态。</p>
      </div>
    </header>

    <!-- 主卡片 -->
    <div class="glass-card main-card">
      <!-- 搜索区域 -->
      <div class="search-section">
        <el-form :inline="true" :model="queryParams" class="search-form">
          <el-form-item prop="menuName" label="菜单名称">
            <el-input 
              placeholder="请输入菜单名称" 
              clearable 
              size="small" 
              @keyup.enter="handleQuery"
              v-model="queryParams.menuName" 
              class="modern-input" />
          </el-form-item>
          <el-form-item prop="menuStatus" label="菜单状态">
            <el-select 
              size="small" 
              placeholder="菜单状态" 
              v-model="queryParams.menuStatus" 
              class="modern-select"
              style="width: 150px;">
              <el-option v-for="item in menuStatusList" :key="item.value" :label="item.label" :value="item.value">
              </el-option>
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" size="small" @click="handleQuery" class="modern-btn primary-btn system-toolbar-button">
              <el-icon><Search /></el-icon>
              查询
            </el-button>
            <el-button type="warning" size="small" @click="resetQuery" class="modern-btn reset-btn system-toolbar-button">
              <el-icon><RefreshRight /></el-icon>
              重置
            </el-button>
            <el-button
              type="success"
              size="small"
              @click="handleAddMenu"
              v-authority="['base:menu:add']"
              class="modern-btn success-btn system-toolbar-button">
              <el-icon><Plus /></el-icon>
              新增菜单
            </el-button>
            <el-button
              type="warning"
              size="small"
              @click="toggleExpandAll"
              class="modern-btn secondary-btn system-toolbar-button">
              <el-icon><Sort /></el-icon>
              展开/折叠
            </el-button>
          </el-form-item>
        </el-form>
      </div>

      <!-- 数据表格区域 -->
      <div class="table-section">
      <el-table 
        v-if="refreshTable"
        v-loading="loading" 
        :data="menuList" 
        row-key="id" 
        :default-expand-all="isExpandAll"
        :tree-props="{ children: 'children' }"
        class="modern-table"
        :header-cell-style="{ background: 'transparent', color: '#2c3e50', fontWeight: '600' }"
        :row-style="{ background: 'rgba(255, 255, 255, 0.05)' }"
        stripe>
        <el-table-column prop="menuName" label="菜单名称" min-width="150">
          <template v-slot="scope">
            <span class="menu-name">{{ scope.row.menuName }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="icon" label="图标" width="80">
          <template v-slot="scope">
            <div class="icon-wrapper">
              <el-icon class="menu-icon">
                <component :is="scope.row.icon" />
              </el-icon>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="sort" label="排序" width="80" />
        <el-table-column prop="value" label="权限标识" min-width="150" />
        <el-table-column prop="url" label="组件路径" min-width="200" />
        <el-table-column prop="menuType" label="菜单类型" width="100">
          <template v-slot="scope">
            <el-tag 
              v-if="scope.row.menuType === 1" 
              class="modern-tag modern-tag-directory">
              目录
            </el-tag>
            <el-tag 
              v-else-if="scope.row.menuType === 2" 
              class="modern-tag modern-tag-menu">
              菜单
            </el-tag>
            <el-tag 
              v-else-if="scope.row.menuType === 3" 
              class="modern-tag modern-tag-button">
              按钮
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="menuStatus" label="状态" width="100">
          <template v-slot="scope">
            <el-tag 
              v-if="scope.row.menuStatus === 2" 
              class="modern-tag modern-tag-active">
              启用
            </el-tag>
            <el-tag 
              v-else-if="scope.row.menuStatus === 1" 
              class="modern-tag modern-tag-inactive">
              禁用
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="创建时间" prop="createTime" min-width="160" />
        <el-table-column label="操作" width="280" fixed="right">
          <template v-slot="scope">
            <div class="operation-buttons">
              <el-tooltip content="修改" placement="top">
                <el-button
                  type="warning"
                  size="small"
                  @click="showEditMenuDialog(scope.row.id)"
                  v-authority="['base:menu:edit']"
                >
                  <el-icon><Edit /></el-icon>修改
                </el-button>
              </el-tooltip>
              <el-tooltip content="复制" placement="top">
                <el-button
                  type="primary"
                  size="small"
                  @click="handleCopyMenu(scope.row)"
                  v-authority="['base:menu:add']"
                >
                  <el-icon><DocumentCopy /></el-icon>复制
                </el-button>
              </el-tooltip>
              <el-tooltip content="删除" placement="top">
                <el-button
                  type="danger"
                  size="small"
                  @click="handleMenuDelete(scope.row)"
                  v-authority="['base:admin:delete']"
                >
                  <el-icon><Delete /></el-icon>删除
                </el-button>
              </el-tooltip>
            </div>
          </template>
        </el-table-column>
      </el-table>
      </div>
    </div>

    <!-- 新增菜单对话框 -->
    <el-dialog 
      title="新增菜单" 
      v-model="addMenuDialogVisible" 
      width="680px"
      @close="addMenuDialogClosed"
      class="modern-dialog"
      modal-class="menu-form-dialog-overlay">
      <div class="dialog-content">
        <el-form :model="menuForm" :rules="addMenuFormRules" ref="addMenuFormRefForm" label-width="100px" class="modern-form">
          <el-row :gutter="20">
            <el-col :span="24">
              <el-form-item label="菜单类型" prop="menuType">
                <el-radio-group v-model="menuForm.menuType" class="modern-radio-group">
                  <el-radio :label="1" class="modern-radio">目录</el-radio>
                  <el-radio :label="2" class="modern-radio">菜单</el-radio>
                  <el-radio :label="3" class="modern-radio">按钮</el-radio>
                </el-radio-group>
              </el-form-item>
            </el-col>
            <el-col :span="24" v-if="menuForm.menuType != 1">
              <el-form-item label="上级菜单" prop="parentId">
                <treeselect :options="treeList" placeholder="请选择上级菜单" v-model="menuForm.parentId" class="modern-treeselect" />
              </el-form-item>
            </el-col>
            <el-col :span="24" v-if="menuForm.menuType === 2">
              <el-form-item label="菜单图标" prop="icon">
                <el-select v-model="menuForm.icon" class="modern-select" placeholder="请选择图标">
                  <el-option v-for="item in iconList" :key="item.value" :label="item.label" :value="item.value">
                      <span style="display: flex; align-items: center;">
                        <el-icon style="font-size: 20px;">
                          <component :is="item.value" />
                        </el-icon>
                        <span style="margin-left: 8px;">{{ item.label }}</span>
                      </span>
                  </el-option>
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="菜单名称" prop="menuName">
                <el-input v-model="menuForm.menuName" placeholder="请输入菜单名称" class="modern-input" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="显示排序" prop="sort">
                <el-input-number v-model="menuForm.sort" controls-position="right" :min="0" class="modern-input-number" />
              </el-form-item>
            </el-col>
            <el-col :span="24" v-if="menuForm.menuType != 3">
              <el-form-item label="菜单路径" prop="url">
                <el-input v-model="menuForm.url" placeholder="请输入菜单路径" class="modern-input" />
              </el-form-item>
            </el-col>
            <el-col :span="24" v-if="menuForm.menuType != 1">
              <el-form-item label="权限标识" prop="value">
                <el-input v-model="menuForm.value" placeholder="请输入权限标识" maxlength="50" class="modern-input" />
              </el-form-item>
            </el-col>
            <el-col :span="24" v-if="menuForm.menuType != 3">
              <el-form-item label="显示状态" prop="menuStatus">
                <el-radio-group v-model="menuForm.menuStatus" class="modern-radio-group">
                  <el-radio :label="1" class="modern-radio">停用</el-radio>
                  <el-radio :label="2" class="modern-radio">启用</el-radio>
                </el-radio-group>
              </el-form-item>
            </el-col>
          </el-row>
        </el-form>
      </div>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="addMenuDialogVisible = false" class="modern-btn modern-btn-cancel">取消</el-button>
          <el-button type="primary" @click="addMenu" class="modern-btn modern-btn-primary">确定</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 修改菜单对话框 -->
    <el-dialog 
      title="修改菜单" 
      v-model="editMenuDialogVisible" 
      width="680px"
      @close="editMenuDialogClosed"
      class="modern-dialog"
      modal-class="menu-form-dialog-overlay">
      <div class="dialog-content">
        <el-form :model="menuInfo" :rules="editMenuFormRules" ref="editMenuFormRefForm" label-width="100px" class="modern-form">
          <el-row :gutter="20">
            <el-col :span="24">
              <el-form-item label="菜单类型" prop="menuType">
                <el-radio-group v-model="menuInfo.menuType" class="modern-radio-group">
                  <el-radio :label="1" class="modern-radio">目录</el-radio>
                  <el-radio :label="2" class="modern-radio">菜单</el-radio>
                  <el-radio :label="3" class="modern-radio">按钮</el-radio>
                </el-radio-group>
              </el-form-item>
            </el-col>
            <el-col :span="24" v-if="menuInfo.menuType != 1">
              <el-form-item label="上级菜单" prop="parentId">
                <treeselect :options="treeList" placeholder="请选择上级菜单" v-model="menuInfo.parentId" class="modern-treeselect" />
              </el-form-item>
            </el-col>
            <el-col :span="24" v-if="menuInfo.menuType != 3">
              <el-form-item label="菜单图标" prop="icon">
                <el-select v-model="menuInfo.icon" class="modern-select" placeholder="请选择图标">
                  <el-option v-for="item in iconList" :key="item.value" :label="item.label" :value="item.value">
                      <span style="display: flex; align-items: center;">
                        <el-icon style="font-size: 20px;">
                          <component :is="item.value" />
                        </el-icon>
                        <span style="margin-left: 8px;">{{ item.label }}</span>
                      </span>
                  </el-option>
                </el-select>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="菜单名称" prop="menuName">
                <el-input v-model="menuInfo.menuName" placeholder="请输入菜单名称" class="modern-input" />
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="显示排序" prop="sort">
                <el-input-number v-model="menuInfo.sort" controls-position="right" :min="0" class="modern-input-number" />
              </el-form-item>
            </el-col>
            <el-col :span="24" v-if="menuInfo.menuType != 3">
              <el-form-item label="菜单路径" prop="url">
                <el-input v-model="menuInfo.url" placeholder="请输入菜单路径" class="modern-input" />
              </el-form-item>
            </el-col>
            <el-col :span="24" v-if="menuInfo.menuType != 1">
              <el-form-item label="权限标识" prop="value">
                <el-input v-model="menuInfo.value" placeholder="请输入权限标识" maxlength="50" class="modern-input" />
              </el-form-item>
            </el-col>
            <el-col :span="24" v-if="menuInfo.menuType != 3">
              <el-form-item label="显示状态" prop="menuStatus">
                <el-radio-group v-model="menuInfo.menuStatus" class="modern-radio-group">
                  <el-radio :label="1" class="modern-radio">停用</el-radio>
                  <el-radio :label="2" class="modern-radio">启用</el-radio>
                </el-radio-group>
              </el-form-item>
            </el-col>
          </el-row>
        </el-form>
      </div>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="editMenuDialogVisible = false" class="modern-btn modern-btn-cancel">取消</el-button>
          <el-button type="primary" @click="editMenu" class="modern-btn modern-btn-primary">确定</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import Treeselect from 'vue3-treeselect'
import 'vue3-treeselect/dist/vue3-treeselect.css'
import {
  Search,
  RefreshRight,
  Plus,
  Sort,
  Edit,
  Delete,
  DocumentCopy
} from '@element-plus/icons-vue'

export default {
  name: 'SystemMenu',
  components: {
    Treeselect,
    Search,
    RefreshRight,
    Plus,
    Sort,
    Edit,
    Delete,
    DocumentCopy
  },
  data() {
    return {
      queryParams: {},
      menuStatusList: [{
        value: '2',
        label: '启用'
      }, {
        value: '1',
        label: '停用'
      }],
      loading: true,
      menuList: [],
      isExpandAll: false,
      refreshTable: true,
      iconList: [
        // 通用与导航
        { value: 'HomeFilled', label: 'HomeFilled' },
        { value: 'House', label: 'House' },
        { value: 'Menu', label: 'Menu' },
        { value: 'Grid', label: 'Grid' },
        { value: 'List', label: 'List' },

        // 操作与状态
        { value: 'Search', label: 'Search' },
        { value: 'Edit', label: 'Edit' },
        { value: 'EditPen', label: 'EditPen' },
        { value: 'Delete', label: 'Delete' },
        { value: 'DeleteFilled', label: 'DeleteFilled' },
        { value: 'Plus', label: 'Plus' },
        { value: 'Minus', label: 'Minus' },
        { value: 'Close', label: 'Close' },
        { value: 'CirclePlus', label: 'CirclePlus' },
        { value: 'CircleClose', label: 'CircleClose' },
        { value: 'Check', label: 'Check' },
        { value: 'CircleCheck', label: 'CircleCheck' },
        { value: 'SuccessFilled', label: 'SuccessFilled' },
        { value: 'WarningFilled', label: 'WarningFilled' },
        { value: 'InfoFilled', label: 'InfoFilled' },
        { value: 'QuestionFilled', label: 'QuestionFilled' },
        { value: 'More', label: 'More' },
        { value: 'Star', label: 'Star' },
        { value: 'StarFilled', label: 'StarFilled' },

        // 业务与数据
        { value: 'Platform', label: 'Platform' },
        { value: 'TrendCharts', label: 'TrendCharts' },
        { value: 'Document', label: 'Document' },
        { value: 'DocumentAdd', label: 'DocumentAdd' },
        { value: 'DocumentCopy', label: 'DocumentCopy' },
        { value: 'DocumentDelete', label: 'DocumentDelete' },
        { value: 'DocumentRemove', label: 'DocumentRemove' },
        { value: 'Files', label: 'Files' },
        { value: 'Folder', label: 'Folder' },
        { value: 'FolderOpened', label: 'FolderOpened' },
        { value: 'FolderAdd', label: 'FolderAdd' },
        { value: 'FolderRemove', label: 'FolderRemove' },
        { value: 'FolderDelete', label: 'FolderDelete' },
        { value: 'FolderChecked', label: 'FolderChecked' },
        { value: 'Tickets', label: 'Tickets' },

        // 设备与系统
        { value: 'Monitor', label: 'Monitor' },
        { value: 'CpuFill', label: 'CpuFill' },
        { value: 'MemoryStick', label: 'MemoryStick' },
        { value: 'Server', label: 'Server' },
        { value: 'Network', label: 'Network' },
        { value: 'Box', label: 'Box' },

        // 多媒体与查看
        { value: 'View', label: 'View' },
        { value: 'Picture', label: 'Picture' },
        { value: 'Camera', label: 'Camera' },
        { value: 'CameraFilled', label: 'CameraFilled' },
        { value: 'VideoPlay', label: 'VideoPlay' },
        { value: 'VideoPause', label: 'VideoPause' },

        // 账户与设置
        { value: 'User', label: 'User' },
        { value: 'Setting', label: 'Setting' },
        { value: 'Tools', label: 'Tools' },
        { value: 'Key', label: 'Key' },
        { value: 'Lock', label: 'Lock' },
        { value: 'Unlock', label: 'Unlock' },

        // 商务与电商
        { value: 'Eleme', label: 'Eleme' },
        { value: 'ElemeFilled', label: 'ElemeFilled' },
        { value: 'Goods', label: 'Goods' },
        { value: 'Shop', label: 'Shop' },
        { value: 'ShoppingCartFull', label: 'ShoppingCartFull' },
        { value: 'ShoppingCart', label: 'ShoppingCart' },
        { value: 'Wallet', label: 'Wallet' },
        { value: 'Coin', label: 'Coin' },
        { value: 'Discount', label: 'Discount' },
        { value: 'PriceTag', label: 'PriceTag' },
        { value: 'Present', label: 'Present' },
        { value: 'Promotion', label: 'Promotion' },

        // 通讯与社交
        { value: 'Phone', label: 'Phone' },
        { value: 'Message', label: 'Message' },
        { value: 'ChatDotRound', label: 'ChatDotRound' },
        { value: 'ChatLineRound', label: 'ChatLineRound' },
        { value: 'Notification', label: 'Notification' },
        { value: 'Bell', label: 'Bell' },
        { value: 'Link', label: 'Link' },
        { value: 'Connection', label: 'Connection' },
        { value: 'Share', label: 'Share' },

        // 时间与刷新
        { value: 'Calendar', label: 'Calendar' },
        { value: 'Clock', label: 'Clock' },
        { value: 'Timer', label: 'Timer' },
        { value: 'Refresh', label: 'Refresh' },
        { value: 'RefreshLeft', label: 'RefreshLeft' },
        { value: 'RefreshRight', label: 'RefreshRight' },

        // 方向与箭头
        { value: 'ArrowLeft', label: 'ArrowLeft' },
        { value: 'ArrowRight', label: 'ArrowRight' },
        { value: 'ArrowUp', label: 'ArrowUp' },
        { value: 'ArrowDown', label: 'ArrowDown' },
        { value: 'Top', label: 'Top' },
        { value: 'Bottom', label: 'Bottom' },
        { value: 'CaretTop', label: 'CaretTop' },
        { value: 'CaretBottom', label: 'CaretBottom' },
        { value: 'CaretLeft', label: 'CaretLeft' },
        { value: 'CaretRight', label: 'CaretRight' },
        { value: 'DArrowRight', label: 'DArrowRight' },

        // 上传下载与打印
        { value: 'Upload', label: 'Upload' },
        { value: 'UploadFilled', label: 'UploadFilled' },
        { value: 'Download', label: 'Download' },
        { value: 'Printer', label: 'Printer' },

        // 其它
        { value: 'Guide', label: 'Guide' },
        { value: 'Help', label: 'Help' },
        { value: 'Warning', label: 'Warning' },
        { value: 'MagicStick', label: 'MagicStick' }
      ],
      addMenuDialogVisible: false,
      menuForm: {
        menuType: 1,
        menuStatus: 2
      },
      addMenuFormRules: {
        menuType: [{ required: true, message: "菜单类型不能为空", trigger: "blur" }],
        menuName: [{ required: true, message: "菜单名称不能为空", trigger: "blur" }],
        sort: [{ required: true, message: "菜单顺序不能为空", trigger: "blur" }],
        value: [{ required: true, message: "权限标识不能为空", trigger: "blur" }]
      },
      treeList: [],
      editMenuDialogVisible: false,
      menuInfo: [],
      editMenuFormRules: {
        menuType: [{ required: true, message: "菜单类型不能为空", trigger: "blur" }],
        menuName: [{ required: true, message: "菜单名称不能为空", trigger: "blur" }],
        sort: [{ required: true, message: "菜单顺序不能为空", trigger: "blur" }],
        value: [{ required: true, message: "权限标识不能为空", trigger: "blur" }]
      },
    }
  },
  methods: {
    // 列表
    async getMenuList() {
      this.loading = true;
      const { data: res } = await this.$api.queryMenuList(this.queryParams)
      // console.log(res)
      if (res.code !== 200) {
        this.$message.error(res.message);
      } else {
        this.menuList = this.$handleTree.handleTree(res.data, "id");
        this.loading = false;
      }
    },
    // 搜索
    handleQuery() {
      this.getMenuList();
    },
    // 重置
    resetQuery() {
      this.queryParams = {}
      this.getMenuList();
      this.$message.success("重置成功")
    },
    // 展开/折叠
    toggleExpandAll() {
      this.refreshTable = false
      this.isExpandAll = !this.isExpandAll
      this.$nextTick(() => {
        this.refreshTable = true
      })
    },
    // 新增菜单关闭事件
    addMenuDialogClosed() {
      this.$refs.addMenuFormRefForm.resetFields()
    },
    // 打开新增菜单弹窗
    handleAddMenu() {
      this.menuForm = {
        menuType: 1,
        menuStatus: 2,
        sort: 0
      }
      this.addMenuDialogVisible = true
      this.$nextTick(() => {
        this.$refs.addMenuFormRefForm?.clearValidate()
      })
    },
    // 按sort字段对菜单数据排序
    sortMenusBySort(menuList) {
      if (!menuList || !Array.isArray(menuList)) {
        return [];
      }

      // 对当前层级按sort字段排序
      const sortedList = [...menuList].sort((a, b) => {
        const sortA = a.sort || 0;
        const sortB = b.sort || 0;
        return sortA - sortB;
      });

      return sortedList;
    },

    // 新增下拉列表
    async getMenuVoList() {
      try {
        // 同时获取菜单数据和带sort的菜单数据
        const [menuVoRes, menuWithSortRes] = await Promise.all([
          this.$api.querySysMenuVoList(),
          this.$api.queryMenuList({})
        ]);

        if (menuVoRes.data.code !== 200) {
          this.$message.error(menuVoRes.data.message);
          return;
        }

        const menuVoData = menuVoRes.data.data;
        const menuWithSort = menuWithSortRes.data.data;

        // 创建id到sort值的映射
        const sortMap = {};
        if (menuWithSort && Array.isArray(menuWithSort)) {
          menuWithSort.forEach(menu => {
            sortMap[menu.id] = menu.sort || 0;
          });
        }

        // 为菜单数据添加sort字段
        const menuWithSortField = menuVoData.map(menu => ({
          ...menu,
          sort: sortMap[menu.id] || 999
        }));

        // 按sort字段排序
        const sortedMenus = this.sortMenusBySort(menuWithSortField);

        // 构建树形结构
        this.treeList = this.$handleTree.handleTree(sortedMenus, "id");

        console.log('上级菜单排序结果:', this.treeList);
      } catch (error) {
        console.error('获取菜单列表失败:', error);
        this.$message.error('获取菜单列表失败');
      }
    },
    // 新增操作
    addMenu() {
      this.$refs.addMenuFormRefForm.validate(async valid => {
        if (!valid) return

        // 清理复制时可能产生的无效字段
        const submitData = { ...this.menuForm }
        delete submitData.children
        delete submitData.createTime
        delete submitData.updateTime

        console.log('提交的菜单数据:', submitData)

        try {
          const { data: res } = await this.$api.addMenu(submitData);
          if (res.code === 200) {
            this.$message.success("新增菜单成功")
            this.addMenuDialogVisible = false
            await this.getMenuList()
            await this.getMenuVoList()
          } else {
            this.$message.error(res.message || "新增菜单失败")
          }
        } catch (error) {
          console.error('新增菜单错误:', error)
          // 如果是400错误但实际可能创建成功，先关闭对话框再刷新列表
          if (error.response?.status === 400) {
            this.$message.warning("操作完成，正在刷新数据...")
            this.addMenuDialogVisible = false
            await this.getMenuList()
            await this.getMenuVoList()
          } else {
            this.$message.error("新增菜单失败: " + (error.message || "未知错误"))
          }
        }
      })
    },
    // 监听修改菜单关闭事件
    editMenuDialogClosed() {
      this.$refs.editMenuFormRefForm.resetFields()
    },
    // 打开菜单
    async showEditMenuDialog(id) {
      const { data: res } = await this.$api.menuInfo(id)
      if (res.code !== 200) {
        this.$message.error(res.message)
      } else {
        this.menuInfo = res.data
        this.editMenuDialogVisible = true
      }
    },
    // 修改菜单
    editMenu() {
      this.$refs.editMenuFormRefForm.validate(async valid => {
        if (!valid) return
        const { data: res } = await this.$api.menuUpdate(this.menuInfo)
        if (res.code !== 200) {
          this.$message.error(res.message)
        } else {
          this.editMenuDialogVisible = false
          await this.getMenuList()
          this.$message.success("修改菜单成功")
        }
      })
    },
    // 复制菜单
    handleCopyMenu(menuData) {
      // 复制数据并清除不需要的字段
      const copyData = {
        parentId: menuData.parentId,
        menuName: `${menuData.menuName}_副本`,
        icon: menuData.icon || '',
        value: menuData.value ? `${menuData.value}_copy` : '',
        menuType: menuData.menuType,
        url: menuData.url ? `${menuData.url}_copy` : '',
        sort: menuData.sort,
        menuStatus: menuData.menuStatus || 2
      }

      // 重置表单并填充复制的数据
      this.$nextTick(() => {
        if (this.$refs.addMenuFormRefForm) {
          this.$refs.addMenuFormRefForm.resetFields()
        }
      })

      this.menuForm = { ...copyData }
      this.addMenuDialogVisible = true

      // 调试输出
      console.log('复制的菜单数据:', copyData)
    },
    // 删除菜单
    async handleMenuDelete(row) {
      const confirmResult = await this.$confirm('是否确认删除菜单为"' + row.menuName + '"的数据项？', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      }).catch(err => err)
      if (confirmResult !== 'confirm') {
        return this.$message.info('已取消删除')
      }
      const { data: res } = await this.$api.menuDelete(row.id)
      if (res.code !== 200) {
        this.$message.error(res.message)
      } else {
        this.$message.success('删除成功')
        await this.getMenuList()
      }
    },
  },
  created() {
    this.getMenuList()
    this.getMenuVoList()
  }
}
</script>

<style lang="less" scoped>
.modern-menu-container {
  min-height: 100vh;
  background: #eef1f6;
  padding: 20px;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', 'Helvetica Neue', Helvetica, Arial, sans-serif;
  
  .main-card {
    background: rgba(255, 255, 255, 0.95) !important;
    backdrop-filter: blur(10px);
    border-radius: 16px;
    border: 1px solid rgba(255, 255, 255, 0.2);
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
    padding: 24px;
    transition: all 0.3s ease;
    box-sizing: border-box;
    width: 100%;
    overflow: hidden;
    
    &:hover {
      transform: translateY(-2px);
      box-shadow: 0 12px 40px rgba(0, 0, 0, 0.15);
    }
  }
  
  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;
    padding-bottom: 12px;
    border-bottom: 1px solid rgba(103, 126, 234, 0.1);
    
    .gradient-title {
      color: #2c3e50;
      background: linear-gradient(45deg, #667eea, #764ba2);
      background-clip: text;
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
      font-size: 20px;
      font-weight: 600;
      margin: 0;
    }
  }
  
  .search-section {
    margin-bottom: 24px;
    padding: 20px;
    background: rgba(103, 126, 234, 0.05);
    border-radius: 12px;
    border: 1px solid rgba(103, 126, 234, 0.1);
    overflow: hidden;
    margin-left: 0;
    margin-right: 0;
    
    .search-form {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 16px;
      width: 100%;
      box-sizing: border-box;
      
      :deep(.el-form-item) {
        margin-bottom: 0;
        flex-shrink: 0;
      }
      
      :deep(.el-form-item__label) {
        color: #606266;
        font-weight: 500;
      }

      :deep(.el-form-item:last-child) {
        display: flex;
        gap: 12px;
      }
    }
  }
  
  // 操作按钮区域样式
  .action-section {
    margin-bottom: 5px;
    margin-left: 0;
    margin-right: 0;
    padding-left: 20px;
    
    .action-buttons {
      display: flex;
      gap: 12px;
      align-items: center;
      
      .modern-btn {
        transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
        
        &:hover {
          transform: translateY(-2px);
          box-shadow: 0 6px 20px rgba(0, 0, 0, 0.15);
        }
      }
    }
  }
  
  .table-section {
    background: transparent;
    border-radius: 12px;
    padding: 20px;
    overflow: hidden;
    min-width: 0;
    margin-left: 0;
    margin-right: 0;
  }
}

// 页面标题
.page-header {
  text-align: center;
  margin-bottom: 30px;
  
  .page-title {
    font-size: 32px;
    font-weight: 700;
    margin: 0 0 10px 0;
    background: linear-gradient(135deg, #ffffff, #e0e6ff);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
    text-shadow: 0 4px 8px rgba(0, 0, 0, 0.3);
  }
  
  .page-description {
    font-size: 16px;
    color: rgba(255, 255, 255, 0.8);
    margin: 0;
  }
}

// 毛玻璃卡片效果
.glass-card {
  background: rgba(255, 255, 255, 0.1);
  backdrop-filter: blur(10px);
  border-radius: 20px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  margin-bottom: 20px;
  transition: all 0.3s ease;
  
  &:hover {
    transform: translateY(-2px);
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.15);
  }
}

// 搜索卡片
.search-card {
  padding: 25px;
  
  .search-form {
    :deep(.el-form-item__label) {
      color: rgba(255, 255, 255, 0.9);
      font-weight: 500;
    }
  }
}

// 操作按钮区域
.action-card {
  padding: 20px 25px;
  
  .action-buttons {
    display: flex;
    align-items: center;
    gap: 12px;
  }
}

// 表格卡片
.table-card {
  padding: 25px;
  overflow: hidden;
}

// 现代化按钮
.modern-btn {
  border-radius: 8px;
  padding: 8px 20px;
  font-weight: 500;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  border: none;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.modern-btn:hover {
  transform: translateY(-2px);
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.2);
}

.primary-btn {
  background: linear-gradient(45deg, #409EFF, #66B3FF);
  color: white;
}

.reset-btn {
  background: linear-gradient(45deg, #E6A23C, #EEBE77);
  color: white;
}

.success-btn {
  background: linear-gradient(45deg, #67C23A, #85CE61);
  color: white;
}

.secondary-btn {
  background: linear-gradient(45deg, #909399, #B1B3B8);
  color: white;
}

// 现代化输入框
.modern-input {
  :deep(.el-input__wrapper) {
    background: rgba(255, 255, 255, 0.8);
    border: 1px solid rgba(103, 126, 234, 0.2);
    border-radius: 8px;
    box-shadow: none;
    transition: all 0.3s ease;
  }

  :deep(.el-input__wrapper):hover {
    border-color: #c0c4cc;
  }

  :deep(.el-input__wrapper.is-focus) {
    border-color: #667eea;
    box-shadow: 0 0 0 2px rgba(103, 126, 234, 0.2);
    background: rgba(255, 255, 255, 1);
  }

  :deep(.el-input__inner) {
    background: transparent;
    border: none;
    color: #2c3e50;

    &::placeholder {
      color: rgba(44, 62, 80, 0.6);
    }
  }
}

// 现代化选择框
.modern-select {
  :deep(.el-select__wrapper) {
    background: rgba(255, 255, 255, 0.8);
    border: 1px solid rgba(103, 126, 234, 0.2);
    border-radius: 8px;
    box-shadow: none;
    transition: all 0.3s ease;
  }

  :deep(.el-select__wrapper):hover {
    border-color: #c0c4cc;
  }

  :deep(.el-select__wrapper.is-focus) {
    border-color: #667eea;
    box-shadow: 0 0 0 2px rgba(103, 126, 234, 0.2);
    background: rgba(255, 255, 255, 1);
  }

  :deep(.el-input__inner) {
    background: transparent;
    border: none;
    color: #2c3e50;
  }

  :deep(.el-input__inner::placeholder) {
    color: rgba(44, 62, 80, 0.6);
  }

  :deep(.el-input__suffix-inner) {
    color: #606266;
  }
}

// 数字输入框
.modern-input-number {
  :deep(.el-input-number) {
    .el-input__wrapper {
      background: rgba(255, 255, 255, 0.1);
      border: 1px solid rgba(255, 255, 255, 0.2);
      border-radius: 12px;
      backdrop-filter: blur(10px);
      
      &:hover, &.is-focus {
        background: rgba(255, 255, 255, 0.15);
        border-color: rgba(102, 126, 234, 0.6);
        box-shadow: 0 0 0 2px rgba(102, 126, 234, 0.2);
      }
      
      .el-input__inner {
        color: rgba(255, 255, 255, 0.9);
      }
    }
    
    .el-input-number__increase, .el-input-number__decrease {
      background: rgba(255, 255, 255, 0.1);
      border-color: rgba(255, 255, 255, 0.2);
      color: rgba(255, 255, 255, 0.7);
      
      &:hover {
        background: rgba(255, 255, 255, 0.2);
        color: white;
      }
    }
  }
}

// 现代化表格
.modern-table {
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
  
  :deep(.el-table) {
    background: transparent;
    
    &::before {
      display: none;
    }
    
    th {
      background: linear-gradient(135deg, #667eea, #764ba2) !important;
      border-bottom: none;
      color: #2c3e50 !important;
      font-weight: 700 !important;
      padding: 8px 12px !important;
      height: 40px;
      text-shadow: 0 1px 2px rgba(255, 255, 255, 0.8);
      
      .cell {
        color: #2c3e50 !important;
        font-weight: 700 !important;
        text-shadow: 0 1px 2px rgba(255, 255, 255, 0.8);
      }
    }
    
    td {
      background: rgba(255, 255, 255, 0.05) !important;
      border-bottom: 1px solid rgba(103, 126, 234, 0.1);
      color: #2c3e50;
      padding: 8px 12px !important;
      height: 40px;
    }
    
    .el-table__row {
      transition: all 0.3s ease;
      
      &:hover {
        background: rgba(103, 126, 234, 0.05) !important;
        transform: translateY(-2px);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
      }
    }

    .el-table__cell {
      padding: 8px 12px !important;
    }
  }
  
  .menu-name {
    font-weight: 500;
    color: #2c3e50;
  }
  
  .icon-wrapper {
    display: flex;
    justify-content: center;
    align-items: center;
    
    .menu-icon {
      font-size: 18px;
      color: rgba(102, 126, 234, 0.8);
      transition: all 0.3s ease;
      
      &:hover {
        color: rgba(102, 126, 234, 1);
        transform: scale(1.2);
      }
    }
  }
  
  .operation-buttons {
    display: flex;
    gap: 8px;
    justify-content: center;
    
    .el-button {
      transition: all 0.3s ease;
      
      &:hover {
        transform: translateY(-2px);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
      }
    }
  }
}

// 现代化标签
.modern-tag {
  border-radius: 12px;
  font-weight: 500;
  padding: 4px 12px;
  border: none;
  backdrop-filter: blur(10px);
  transition: all 0.3s ease;
  
  &.modern-tag-directory {
    background: linear-gradient(135deg, rgba(64, 158, 255, 0.3), rgba(103, 126, 234, 0.3));
    color: rgba(64, 158, 255, 0.9);
    border: 1px solid rgba(64, 158, 255, 0.4);
  }
  
  &.modern-tag-menu {
    background: linear-gradient(135deg, rgba(82, 196, 26, 0.3), rgba(115, 209, 61, 0.3));
    color: rgba(82, 196, 26, 0.9);
    border: 1px solid rgba(82, 196, 26, 0.4);
  }
  
  &.modern-tag-button {
    background: linear-gradient(135deg, rgba(245, 101, 101, 0.3), rgba(255, 135, 135, 0.3));
    color: rgba(245, 101, 101, 0.9);
    border: 1px solid rgba(245, 101, 101, 0.4);
  }
  
  &.modern-tag-active {
    background: linear-gradient(135deg, rgba(82, 196, 26, 0.3), rgba(115, 209, 61, 0.3));
    color: rgba(82, 196, 26, 0.9);
    border: 1px solid rgba(82, 196, 26, 0.4);
  }
  
  &.modern-tag-inactive {
    background: linear-gradient(135deg, rgba(245, 101, 101, 0.3), rgba(255, 135, 135, 0.3));
    color: rgba(245, 101, 101, 0.9);
    border: 1px solid rgba(245, 101, 101, 0.4);
  }
  
  &:hover {
    transform: scale(1.05);
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
  }
}

// 现代化对话框
:deep(.modern-dialog) {
  .el-dialog {
    background: #ffffff;
    border-radius: 8px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    
    .el-dialog__header {
      background: #ffffff;
      border-bottom: 1px solid #f0f0f0;
      padding: 16px 20px;
      
      .el-dialog__title {
        color: #303133;
        font-weight: 600;
        font-size: 16px;
      }
      
      .el-dialog__close {
        color: #909399;
        font-size: 16px;
        
        &:hover {
          color: #303133;
        }
      }
    }
    
    .el-dialog__body {
      padding: 24px;
    }
    
    .el-dialog__footer {
      border-top: 1px solid #f0f0f0;
      padding: 20px 24px;
    }
  }
}

// 对话框内容
.dialog-content {
  .modern-form {
    :deep(.el-form-item__label) {
      color: #606266;
      font-weight: 500;
    }
    
    :deep(.el-form-item__content) {
      .el-input__wrapper {
        background: #ffffff;
        border: 1px solid #dcdfe6;
        border-radius: 4px;
        
        &:hover, &.is-focus {
          border-color: #409eff;
          box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.2);
        }
        
        .el-input__inner {
          color: #606266;
          
          &::placeholder {
            color: #c0c4cc;
          }
        }
      }
      
      .el-select__wrapper {
        background: #ffffff;
        border: 1px solid #dcdfe6;
        border-radius: 4px;
        
        &:hover, &.is-focus {
          border-color: #409eff;
          box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.2);
        }
        
        .el-select__selected-item {
          color: #606266;
        }
        
        .el-select__placeholder {
          color: #c0c4cc;
        }
      }
      
      .el-input-number {
        .el-input__wrapper {
          background: #ffffff;
          border: 1px solid #dcdfe6;
          border-radius: 4px;
          
          &:hover, &.is-focus {
            border-color: #409eff;
            box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.2);
          }
          
          .el-input__inner {
            color: #606266;
          }
        }
        
        .el-input-number__increase, .el-input-number__decrease {
          background: #f5f7fa;
          border-color: #dcdfe6;
          color: #606266;
          
          &:hover {
            background: #409eff;
            color: #ffffff;
          }
        }
      }
    }
  }
}

// 对话框底部
.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

// 现代化单选按钮组
.modern-radio-group {
  :deep(.el-radio) {
    margin-right: 20px;
    
    &.modern-radio {
      .el-radio__input {
        .el-radio__inner {
          background: #ffffff;
          border: 1px solid #dcdfe6;
          
          &:hover {
            border-color: #409eff;
          }
        }
        
        &.is-checked {
          .el-radio__inner {
            background: #409eff;
            border-color: #409eff;
          }
        }
      }
      
      .el-radio__label {
        color: #606266;
        font-weight: 500;
      }
    }
  }
}

// Treeselect 组件样式
.modern-treeselect {
  :deep(.vue-treeselect__control) {
    background: #ffffff;
    border: 1px solid #dcdfe6;
    border-radius: 4px;
    
    &:hover, &.vue-treeselect--focused {
      border-color: #409eff;
      box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.2);
    }
    
    .vue-treeselect__single-value {
      color: #606266;
    }
    
    .vue-treeselect__placeholder {
      color: #c0c4cc;
    }
  }
  
  :deep(.vue-treeselect__menu) {
    background: #ffffff;
    border: 1px solid #dcdfe6;
    border-radius: 4px;
    
    .vue-treeselect__option {
      color: #606266;
      
      &:hover {
        background: #f5f7fa;
      }
      
      &.vue-treeselect__option--selected {
        background: #409eff;
        color: #ffffff;
      }
    }
  }
}

// 全局遮罩样式
:deep(.modern-modal) {
  background: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(5px);
}

// 响应式设计
@media (max-width: 768px) {
  .modern-menu-container {
    padding: 15px;
  }
  
  .page-header .page-title {
    font-size: 24px;
  }
  
  .glass-card {
    margin-bottom: 15px;
    padding: 20px;
  }
  
  .search-card, .action-card, .table-card {
    padding: 20px;
  }
  
  :deep(.modern-dialog .el-dialog) {
    width: 90% !important;
    margin: 5vh auto;
  }
}

// 加载动画优化
:deep(.el-loading-mask) {
  background: rgba(103, 126, 234, 0.1);
  backdrop-filter: blur(10px);
  
  .el-loading-spinner {
    .el-loading-text {
      color: rgba(255, 255, 255, 0.9);
    }
    
    .circular {
      stroke: rgba(102, 126, 234, 0.8);
    }
  }
}

// 滚动条样式
::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

::-webkit-scrollbar-track {
  background: rgba(255, 255, 255, 0.1);
  border-radius: 4px;
}

::-webkit-scrollbar-thumb {
  background: rgba(102, 126, 234, 0.6);
  border-radius: 4px;
  
  &:hover {
    background: rgba(102, 126, 234, 0.8);
  }
}

/* DESIGN.md compact system page overrides */
.modern-menu-container {
  --sys-page-offset: 136px;
  --sys-text-primary: #172133;
  --sys-text-secondary: #5f6b7c;
  --sys-text-muted: #8d97a8;
  --sys-border: rgba(223, 231, 242, 0.92);
  --sys-brand: #4c8bf5;
  --sys-success: #3fb97f;
  --sys-warning: #f3b24b;
  --sys-danger: #ef6b73;
  box-sizing: border-box;
  height: calc(100vh - var(--sys-page-offset));
  min-height: 0;
  padding: 6px;
  overflow: hidden;
  background: transparent;
  color: var(--sys-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

:global(.home-container .el-main:has(.modern-menu-container)) {
  overflow-y: hidden;
}

.modern-menu-container .main-card {
  position: relative;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 0 10px 10px;
  border-radius: 22px;
  border: 1px solid var(--sys-border);
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.96), rgba(247, 250, 255, 0.92)) !important;
  box-shadow: 0 20px 60px rgba(17, 24, 39, 0.07);
  backdrop-filter: blur(18px);
}

.modern-menu-container .main-card::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 40px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  pointer-events: none;
}

.modern-menu-container .main-card:hover,
.glass-card:hover {
  transform: none;
  box-shadow: 0 20px 60px rgba(17, 24, 39, 0.07);
}

.modern-menu-container .card-header {
  position: relative;
  z-index: 1;
  display: block;
  flex-shrink: 0;
  margin: 0;
  padding: 16px 10px 12px;
  border: none;
}

.modern-menu-container .card-header::before {
  content: 'System Center';
  display: block;
  margin-bottom: 3px;
  color: var(--sys-brand);
  font-size: 12px;
  font-weight: 700;
}

.modern-menu-container .card-header .gradient-title {
  margin: 0;
  color: var(--sys-text-primary);
  background: none;
  -webkit-text-fill-color: currentColor;
  font-size: 20px;
  line-height: 1.25;
  font-weight: 700;
}

.modern-menu-container .search-section {
  position: relative;
  z-index: 1;
  flex-shrink: 0;
  margin-bottom: 8px;
  padding: 12px 14px 2px;
  border-radius: 16px;
  border: 1px solid rgba(22, 34, 56, 0.06);
  background: rgba(255, 255, 255, 0.58);
}

.modern-menu-container .search-section .search-form {
  gap: 0 12px;
}

.modern-menu-container .search-section :deep(.el-form-item) {
  margin-bottom: 10px;
}

.modern-menu-container .action-section {
  flex-shrink: 0;
  margin: 0 0 8px;
  padding: 0 2px;
}

.modern-menu-container .table-section {
  min-height: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  margin: 0;
  padding: 0;
  overflow: hidden;
  border-radius: 16px;
  border: 1px solid var(--sys-border);
  background: rgba(255, 255, 255, 0.82);
}

.modern-table {
  flex: 1;
  min-height: 0;
  box-shadow: none;
  color: var(--sys-text-secondary);
  font-size: 13px;
}

.modern-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.modern-table :deep(.el-table__header th) {
  height: 38px;
  padding: 6px 0 !important;
  background: rgba(248, 250, 255, 0.96) !important;
  color: var(--sys-text-muted) !important;
  border-bottom: 1px solid var(--sys-border);
  font-size: 12px;
  font-weight: 700 !important;
  text-shadow: none;
}

.modern-table :deep(.el-table__row td) {
  height: 42px;
  padding: 6px 0 !important;
  border-bottom: 1px solid rgba(15, 23, 42, 0.04);
  color: var(--sys-text-secondary);
}

.modern-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: rgba(76, 139, 245, 0.045) !important;
}

.modern-table .operation-buttons {
  flex-wrap: nowrap;
  justify-content: flex-start;
  gap: 14px;
  white-space: nowrap;
}

.modern-table .operation-buttons .el-button {
  min-height: auto;
  padding: 0;
  border: none;
  background: transparent !important;
  box-shadow: none;
  color: var(--sys-brand);
  font-size: 12px;
  font-weight: 600;
}

.modern-table .operation-buttons .el-button .el-icon {
  display: none;
}

.modern-table .operation-buttons .el-button--danger {
  color: var(--sys-danger);
}

.modern-table .operation-buttons .el-button--warning {
  color: #b77b24;
}

.modern-btn,
.el-button {
  min-height: 32px;
  border-radius: 14px;
  font-weight: 600;
  box-shadow: none;
}

.primary-btn,
.modern-btn-primary,
.el-button--primary {
  border-color: transparent;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
  color: #fff;
}

.success-btn,
.el-button--success {
  border-color: transparent;
  background: linear-gradient(180deg, #51c78d 0%, #3fb97f 100%);
  color: #fff;
}

.reset-btn,
.modern-btn-cancel {
  border-color: transparent;
  background: linear-gradient(180deg, #f6be63 0%, #f3b24b 100%);
  color: #fff;
}

.modern-input :deep(.el-input__wrapper),
.modern-select :deep(.el-input__wrapper),
.dialog-content :deep(.el-input__wrapper) {
  min-height: 32px;
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 14px;
  background: rgba(252, 253, 255, 0.98);
  box-shadow: none;
}

:deep(.modern-dialog .el-dialog) {
  border-radius: 28px;
  border: 1px solid rgba(223, 231, 242, 0.96);
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.94));
  box-shadow: 0 36px 110px rgba(15, 23, 42, 0.11);
  overflow: hidden;
}

:deep(.modern-dialog .el-dialog__header) {
  margin: 0;
  padding: 20px 24px 14px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.82);
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
}

:deep(.modern-dialog .el-dialog__title) {
  color: var(--sys-text-primary);
  font-size: 18px;
  font-weight: 700;
}

/* DESIGN.md system page alignment */
:global(.home-container .el-main:has(.modern-menu-container)) {
  padding-top: var(--ops-shell-gutter, 10px);
  padding-right: var(--ops-shell-gutter, 10px);
  padding-left: var(--ops-shell-gutter, 10px);
  overflow: hidden;
}

.modern-menu-container {
  --ops-text-heading: #0f172a;
  --ops-text-primary: #172033;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-bg-surface: #ffffff;
  --ops-bg-subtle: #f3f7fc;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: 8px;
  height: calc(100vh - 124px);
  min-height: 0;
  padding: 7px;
  overflow: hidden;
  background: transparent;
  color: var(--ops-text-primary);
}

.ops-page-header {
  display: flex;
  align-items: center;
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

.modern-menu-container .main-card {
  position: relative;
  display: flex;
  min-width: 0;
  min-height: 0;
  height: 100%;
  flex-direction: column;
  padding: 8px 10px 10px;
  overflow: hidden;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: var(--ops-bg-surface) !important;
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
  backdrop-filter: none;
}

.modern-menu-container .main-card::before,
.modern-menu-container .card-header {
  display: none;
}

.modern-menu-container .main-card:hover,
.modern-menu-container .glass-card:hover {
  transform: none;
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
}

.modern-menu-container .search-section {
  position: relative;
  flex-shrink: 0;
  margin: 0 0 8px;
  padding: 8px 10px 2px;
  overflow: visible;
  border: 1px solid var(--ops-border-soft);
  border-radius: 10px;
  background: var(--ops-bg-subtle);
}

.modern-menu-container .search-section .search-form {
  align-items: center;
  gap: 0 10px;
}

.modern-menu-container .search-section :deep(.el-form-item) {
  margin-right: 0;
  margin-bottom: 6px;
}

.modern-menu-container .search-section :deep(.el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

.modern-menu-container :deep(.modern-input .el-input__wrapper),
.modern-menu-container :deep(.modern-select .el-select__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 7px;
  background: #ffffff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.modern-menu-container :deep(.modern-input .el-input__wrapper:hover),
.modern-menu-container :deep(.modern-select .el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.modern-menu-container :deep(.modern-input .el-input__wrapper.is-focus),
.modern-menu-container :deep(.modern-select .el-select__wrapper.is-focused) {
  box-shadow: 0 0 0 1px #409eff inset;
}

.modern-menu-container :deep(.system-toolbar-button.el-button) {
  height: 30px;
  min-height: 30px;
  padding: 0 10px;
  border-radius: 8px;
  color: #ffffff;
  font-size: 12px;
  box-shadow: none;
}

.modern-menu-container :deep(.primary-btn.system-toolbar-button) {
  border-color: #409eff;
  background: #409eff;
}

.modern-menu-container :deep(.reset-btn.system-toolbar-button),
.modern-menu-container :deep(.secondary-btn.system-toolbar-button) {
  border-color: #e6a23c;
  background: #e6a23c;
}

.modern-menu-container :deep(.success-btn.system-toolbar-button) {
  border-color: #67c23a;
  background: #67c23a;
}

.modern-menu-container :deep(.system-toolbar-button .el-icon) {
  color: inherit;
  font-size: 14px;
}

.modern-menu-container .table-section {
  min-width: 0;
  min-height: 0;
  flex: 1;
  margin: 0;
  padding: 0;
  overflow: hidden;
  border: 1px solid var(--ops-border-soft);
  border-radius: 8px;
  background: #ffffff;
}

.modern-menu-container :deep(.modern-table),
.modern-menu-container :deep(.modern-table .el-table__inner-wrapper) {
  border-radius: 8px;
}

.modern-menu-container :deep(.modern-table) {
  font-size: 12px;
  box-shadow: none;
}

.modern-menu-container :deep(.modern-table .el-table__header th) {
  height: 38px;
  padding: 6px 0 !important;
  background: var(--ops-bg-subtle) !important;
  color: var(--ops-text-secondary) !important;
  border-bottom: 1px solid #eef3f9;
  text-shadow: none;
}

.modern-menu-container :deep(.modern-table .el-table__row td) {
  height: 40px;
  padding: 5px 0 !important;
  border-bottom-color: #eef3f9;
}

.modern-menu-container :deep(.modern-table .el-table__row:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

.modern-menu-container :deep(.modern-table .el-table__row:hover),
.modern-menu-container :deep(.menu-icon:hover),
.modern-menu-container :deep(.modern-tag:hover),
.modern-menu-container :deep(.el-button:hover),
.modern-menu-container :deep(.el-button:active) {
  transform: none;
}

.modern-menu-container .modern-table .operation-buttons {
  flex-wrap: nowrap;
  justify-content: flex-start;
  gap: 14px;
  white-space: nowrap;
}

.modern-menu-container .modern-table .operation-buttons .el-button {
  min-height: auto;
  padding: 0;
  border: 0;
  background: transparent !important;
  box-shadow: none;
  color: #409eff;
  font-size: 12px;
  font-weight: 600;
}

.modern-menu-container .modern-table .operation-buttons .el-button .el-icon {
  display: none;
}

.modern-menu-container .modern-table .operation-buttons .el-button--danger {
  color: #dc2626;
}

.modern-menu-container .modern-table .operation-buttons .el-button--warning {
  color: #d97706;
}

:global(.menu-form-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 32px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: #ffffff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.menu-form-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.menu-form-dialog-overlay .el-dialog__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.menu-form-dialog-overlay .el-dialog__body) {
  box-sizing: border-box;
  max-height: calc(100vh - 170px);
  padding: 16px 18px;
  overflow: auto;
}

:global(.menu-form-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #ffffff;
}

:global(.menu-form-dialog-overlay .el-form-item) {
  margin-bottom: 14px;
}

:global(.menu-form-dialog-overlay .el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

:global(.menu-form-dialog-overlay .el-input__wrapper),
:global(.menu-form-dialog-overlay .el-select__wrapper),
:global(.menu-form-dialog-overlay .el-input-number .el-input__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.menu-form-dialog-overlay .el-select),
:global(.menu-form-dialog-overlay .el-input-number),
:global(.menu-form-dialog-overlay .vue-treeselect) {
  width: 100%;
}

:global(.menu-form-dialog-overlay .vue-treeselect__control) {
  min-height: 34px;
  border-color: #d9e5f2;
  border-radius: 8px;
  background: #fbfdff;
}

:global(.menu-form-dialog-overlay .dialog-footer) {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

:global(.menu-form-dialog-overlay .dialog-footer .el-button) {
  min-height: 34px;
  padding: 0 14px;
  border-radius: 8px;
  box-shadow: none;
}

:global(.menu-form-dialog-overlay .modern-btn-cancel) {
  border-color: #dbe7f4;
  background: #ffffff;
  color: #53657d;
}

:global(.menu-form-dialog-overlay .modern-btn-primary) {
  border-color: #409eff;
  background: #409eff;
  color: #ffffff;
}

:global(.menu-form-dialog-overlay .el-button:hover),
:global(.menu-form-dialog-overlay .el-button:active) {
  transform: none;
}

@media (max-width: 768px) {
  :global(.home-container .el-main:has(.modern-menu-container)) {
    padding-right: 6px;
    padding-left: 6px;
    overflow-y: auto;
  }

  .modern-menu-container {
    height: auto;
    min-height: calc(100vh - 124px);
    padding: 6px;
    overflow: visible;
  }

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
    white-space: normal;
  }

  .modern-menu-container .main-card,
  .modern-menu-container .table-section {
    overflow: visible;
  }

  .modern-menu-container .table-section {
    min-height: 460px;
  }
}
</style>
