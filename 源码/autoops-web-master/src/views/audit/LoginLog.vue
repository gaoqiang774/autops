<template>
  <div class="loginlog-management">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/账号.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>登录日志管理</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>集中查询用户登录状态、来源地址、终端环境与访问时间。</p>
      </div>
      <div class="audit-header-summary" aria-label="登录日志统计">
        <span><em>记录</em><strong>{{ total }}</strong></span>
        <i aria-hidden="true"></i>
        <span><em>已选</em><strong>{{ ids.length }}</strong></span>
      </div>
    </header>

    <el-card shadow="never" class="loginlog-card">
      <!-- 搜索表单 -->
      <div class="search-section">
        <el-form :model="queryParams" :inline="true" class="search-form">
          <el-form-item prop="username" label="用户名称">
            <el-input v-model="queryParams.username" placeholder="请输入用户名称" clearable size="small"
                      style="width: 200px" @keyup.enter="handleQuery" />
          </el-form-item>
          <el-form-item prop="loginStatus" label="登录状态">
            <el-select size="small" placeholder="请选择状态" v-model="queryParams.loginStatus" style="width: 150px" clearable>
              <el-option v-for="item in loginStatusList" :key="item.value" :label="item.label" :value="item.value">
              </el-option>
            </el-select>
          </el-form-item>
          <el-form-item prop="beginTime" label="开始时间">
            <el-date-picker v-model="queryParams.beginTime" size="small" type="date"
                            style="width: 190px" value-format="yyyy-MM-dd" clearable placeholder="请选择开始时间"
                            @keyup.enter="handleQuery" />
          </el-form-item>
          <el-form-item prop="endTime" label="结束时间">
            <el-date-picker v-model="queryParams.endTime" size="small" type="date"
                            style="width: 190px" value-format="yyyy-MM-dd" clearable placeholder="请选择结束时间"
                            @keyup.enter="handleQuery" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" size="small" class="audit-toolbar-button" @click="handleQuery">
              <el-icon><Search /></el-icon>
              <span>查询</span>
            </el-button>
            <el-button type="warning" size="small" class="audit-toolbar-button" @click="resetQuery">
              <el-icon><RefreshRight /></el-icon>
              <span>重置</span>
            </el-button>
            <el-button type="danger" size="small" class="audit-toolbar-button" :disabled="multiple"
                       @click="batchHandleDelete" v-authority="['monitor:loginLog:delete']">
              <el-icon><Delete /></el-icon>
              <span>批量删除</span>
            </el-button>
            <el-button type="danger" size="small" class="audit-toolbar-button" @click="handleClean"
                       v-authority="['monitor:loginLog:clean']">
              <el-icon><Delete /></el-icon>
              <span>清空日志</span>
            </el-button>
          </el-form-item>
        </el-form>
      </div>

      <!-- 列表区域 -->
      <div class="table-section">
        <el-table v-loading="Loading" :data="sysLoginInfoList" stripe style="width: 100%"
                  class="loginlog-table" @selection-change="handleSelectionChange">
          <el-table-column type="selection" />
          <el-table-column label="ID" prop="id" v-if="false" />
          <el-table-column label="用户账号" prop="username" min-width="120" />
          <el-table-column label="登录IP" prop="ipAddress" min-width="140" />
          <el-table-column label="登录地点" prop="loginLocation" min-width="150" show-overflow-tooltip />
          <el-table-column label="浏览器" prop="browser" min-width="120" show-overflow-tooltip />
          <el-table-column label="操作系统" prop="os" min-width="120" show-overflow-tooltip />
          <el-table-column label="登录状态" prop="loginStatus" min-width="100">
            <template #default="scope">
              <el-tag v-if="scope.row.loginStatus === 1" type="success" size="small" effect="dark">成功</el-tag>
              <el-tag v-else-if="scope.row.loginStatus === 2" type="danger" size="small" effect="dark">失败</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="提示消息" prop="message" min-width="150" show-overflow-tooltip />
          <el-table-column label="访问时间" prop="loginTime" min-width="180" />
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="scope">
              <div class="operation-buttons">
                <el-tooltip content="删除" placement="top">
                  <el-button
                    type="danger"
                    :icon="Delete"
                    size="small"
                    @click="handleDelete(scope.row.id)"
                    v-authority="['monitor:loginLog:delete']"
                  >删除</el-button>
                </el-tooltip>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <!-- 分页区域 -->
      <div class="pagination-section">
        <el-pagination @size-change="handleSizeChange" @current-change="handleCurrentChange"
                       :current-page="queryParams.pageNum" :page-sizes="[10, 50, 100, 500, 1000]" :page-size="queryParams.pageSize"
                       layout="total, sizes, prev, pager, next, jumper" :total="total" />
      </div>
    </el-card>
  </div>
</template>

<script>
import { Delete, RefreshRight, Search } from '@element-plus/icons-vue'

export default {
  components: { Delete, RefreshRight, Search },
  data() {
    return {
      queryParams: {},
      loginStatusList: [{
        value: '1',
        label: '成功'
      }, {
        value: '2',
        label: '失败'
      }],
      sysLoginInfoList: [],
      Loading: true,
      ids: [],
      single: true,
      multiple: true,
      total: 0,
    }
  },
  methods: {
    // 多选框选中数据
    handleSelectionChange(selection) {
      this.ids = selection.map(item => item.id);
      this.single = selection.length != 1;
      this.multiple = !selection.length;
    },
    // 查询列表
    async getSysLoginInfoList() {
      this.Loading = true
      const { data: res } = await this.$api.querySysLoginInfoList(this.queryParams)
      if (res.code !== 200) {
        this.$message.error(res.message)
      } else {
        this.sysLoginInfoList = res.data.list
        this.total = res.data.total
        this.Loading = false
      }
    },
    // 搜索按钮操作
    handleQuery() {
      this.getSysLoginInfoList();
    },
    // 重置按钮操作
    resetQuery() {
      this.queryParams = {}
      this.getSysLoginInfoList();
      this.$message.success("重置成功")
    },
    // pageSize
    handleSizeChange(newSize) {
      this.queryParams.pageSize = newSize
      this.getSysLoginInfoList()
    },
    // pageNum
    handleCurrentChange(newPage) {
      this.queryParams.pageNum = newPage
      this.getSysLoginInfoList()
    },
    // 清空
    async handleClean() {
      const confirmResult = await this.$confirm('是否清空登录日志？', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
        customClass: 'audit-log-confirm',
        confirmButtonClass: 'el-button--danger'
      }).catch(err => err)
      if (confirmResult !== 'confirm') {
        return this.$message.info('已取消')
      }
      const { data: res } = await this.$api.cleanSysLoginInfo()
      if (res.code !== 200) {
        this.$message.error(res.message)
      } else {
        this.$message.success('清空成功')
        await this.getSysLoginInfoList()
      }
    },
    // 删除
    async handleDelete(id) {
      const confirmResult = await this.$confirm('是否确认删除登录日志编号为"' + id + '"的数据项？', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
        customClass: 'audit-log-confirm',
        confirmButtonClass: 'el-button--danger'
      }).catch(err => err)
      if (confirmResult !== 'confirm') {
        return this.$message.info('已取消删除')
      }
      const { data: res } = await this.$api.deleteSysLoginInfo(id)
      if (res.code !== 200) {
        this.$message.error(res.message)
      } else {
        this.$message.success('删除成功')
        await this.getSysLoginInfoList()
      }
    },
    // 批量删除
    async batchHandleDelete() {
      const loginInfoIds = this.ids;
      const confirmResult = await this.$confirm('是否确认删除登录日志编号为"' + loginInfoIds + '"的数据项？', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning',
        customClass: 'audit-log-confirm',
        confirmButtonClass: 'el-button--danger'
      }).catch(err => err)
      if (confirmResult !== 'confirm') {
        return this.$message.info('已取消删除')
      }
      const { data: res } = await this.$api.batchDeleteSysLoginInfo(loginInfoIds)
      if (res.code !== 200) {
        this.$message.error(res.message)
      } else {
        this.$message.success('删除成功')
        await this.getSysLoginInfoList()
      }
    },
  },
  created() {
    this.getSysLoginInfoList()
  }
}
</script>

<style scoped>
.loginlog-management {
  padding: 5px;
  min-height: 100vh;
  background: #eef1f6;
}

.loginlog-card {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-radius: 16px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.title {
  font-size: 20px;
  font-weight: 600;
  color: #2c3e50;
  background: linear-gradient(45deg, #667eea, #764ba2);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
}

.search-section {
  margin-bottom: 24px;
  padding: 20px;
  background: rgba(103, 126, 234, 0.05);
  border-radius: 12px;
  border: 1px solid rgba(103, 126, 234, 0.1);
}

.search-form .el-form-item {
  margin-bottom: 0;
  margin-right: 16px;
}

.search-form .el-form-item__label {
  color: #606266;
  font-weight: 500;
}

.action-section {
  margin-bottom: 24px;
  padding: 12px 0;
}

.action-section .el-button {
  margin-right: 12px;
}

.table-section {
  margin-bottom: 24px;
}

.loginlog-table {
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
}

.loginlog-table :deep(.el-table__header) {
  background: linear-gradient(135deg, #667eea, #764ba2);
}

.loginlog-table :deep(.el-table__header th) {
  background: transparent !important;
  color: #2c3e50 !important;
  font-weight: 700 !important;
  border-bottom: none;
  text-shadow: 0 1px 2px rgba(255, 255, 255, 0.8);
}

.loginlog-table :deep(.el-table__header th .cell) {
  color: #2c3e50 !important;
  font-weight: 700 !important;
  text-shadow: 0 1px 2px rgba(255, 255, 255, 0.8);
}

.loginlog-table :deep(.el-table__row) {
  transition: all 0.3s ease;
}

.loginlog-table :deep(.el-table__row:hover) {
  background-color: rgba(103, 126, 234, 0.05) !important;
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.operation-buttons {
  display: flex;
  gap: 8px;
  justify-content: center;
}

.operation-buttons .el-button {
  transition: all 0.3s ease;
}

.operation-buttons .el-button:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

.pagination-section {
  display: flex;
  justify-content: center;
  padding: 20px 0;
}

.el-tag {
  font-weight: 500;
  border-radius: 8px;
  border: none;
}

.el-button {
  border-radius: 8px;
  font-weight: 500;
  transition: all 0.3s ease;
}

.el-button:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.el-input :deep(.el-input__wrapper),
.el-select :deep(.el-input__wrapper) {
  background: rgba(255, 255, 255, 0.8);
  border: 1px solid rgba(103, 126, 234, 0.2);
  border-radius: 8px;
  box-shadow: none;
  transition: all 0.3s ease;
}

.el-input :deep(.el-input__inner),
.el-select :deep(.el-input__inner) {
  background: transparent;
  border: none;
  color: #2c3e50;
}

.el-date-picker :deep(.el-input__inner) {
  border-radius: 8px;
  border: 1px solid rgba(103, 126, 234, 0.2);
  transition: all 0.3s ease;
}

.el-input :deep(.el-input__wrapper):hover,
.el-select :deep(.el-input__wrapper):hover {
  border-color: #c0c4cc;
}

.el-input :deep(.el-input__wrapper.is-focus),
.el-select :deep(.el-input__wrapper.is-focus),
.el-date-picker :deep(.el-input__inner):focus {
  border-color: #667eea;
  box-shadow: 0 0 0 2px rgba(103, 126, 234, 0.2);
  background: rgba(255, 255, 255, 1);
}

.el-loading-mask {
  background-color: rgba(103, 126, 234, 0.1);
  backdrop-filter: blur(4px);
}

.el-pagination {
  background: transparent;
}

.el-pagination :deep(.btn-next),
.el-pagination :deep(.btn-prev),
.el-pagination :deep(.el-pager li) {
  border-radius: 6px;
  margin: 0 2px;
  transition: all 0.3s ease;
}

.el-pagination :deep(.btn-next):hover,
.el-pagination :deep(.btn-prev):hover,
.el-pagination :deep(.el-pager li):hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
}

/* DESIGN.md compact audit page overrides */
.loginlog-management {
  --ops-shell-gutter: 10px;
  --ops-page-padding: 7px;
  --ops-module-gap: 8px;
  --ops-text-heading: #0f172a;
  --ops-text-primary: #172033;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-bg-surface: #ffffff;
  --ops-bg-subtle: #f3f7fc;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  box-sizing: border-box;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: var(--ops-module-gap);
  height: calc(100vh - 124px);
  min-height: 0;
  padding: var(--ops-page-padding);
  overflow: hidden;
  background: transparent;
  color: var(--ops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

:global(.home-container .el-main:has(.loginlog-management)) {
  padding-top: var(--ops-shell-gutter, 10px);
  padding-right: var(--ops-shell-gutter, 10px);
  padding-left: var(--ops-shell-gutter, 10px);
  overflow: hidden;
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

.audit-header-summary {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 10px;
}

.audit-header-summary span {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  white-space: nowrap;
}

.audit-header-summary em {
  color: var(--ops-text-muted);
  font-size: 12px;
  font-style: normal;
}

.audit-header-summary strong {
  color: var(--ops-text-heading);
  font-family: Consolas, Monaco, monospace;
  font-size: 16px;
  line-height: 1;
}

.audit-header-summary > i {
  width: 1px;
  height: 18px;
  background: var(--ops-border-soft);
}

.loginlog-card {
  position: relative;
  min-width: 0;
  min-height: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden !important;
  border: 1px solid var(--ops-border);
  border-radius: 12px !important;
  background: var(--ops-bg-surface);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
  backdrop-filter: none;
}

.loginlog-card::before {
  display: none;
}

.loginlog-card :deep(.el-card__header) {
  position: relative;
  z-index: 1;
  flex-shrink: 0;
  padding: 14px 18px 12px;
  border-bottom: none;
  background: transparent;
}

.loginlog-card :deep(.el-card__body) {
  box-sizing: border-box;
  min-height: 0;
  height: 100%;
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 8px 10px 10px;
  overflow: hidden;
}

.card-header {
  display: block;
}

.card-header::before {
  content: 'Audit Center';
  display: block;
  margin-bottom: 3px;
  color: #4c8bf5;
  font-size: 12px;
  font-weight: 700;
}

.title {
  display: block;
  background: none;
  color: #172133;
  -webkit-text-fill-color: currentColor;
  font-size: 20px;
  line-height: 1.25;
  font-weight: 700;
}

.search-section {
  flex-shrink: 0;
  margin-bottom: 8px;
  padding: 8px 10px 2px;
  border: 1px solid var(--ops-border-soft);
  border-radius: 10px;
  background: var(--ops-bg-subtle);
}

.search-form {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0 10px;
}

.search-form :deep(.el-form-item) {
  margin-right: 0;
  margin-bottom: 6px;
}

.search-form :deep(.el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

.search-form :deep(.el-input__wrapper),
.search-form :deep(.el-select__wrapper),
.search-form :deep(.el-date-editor.el-input__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 7px;
  background: #ffffff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.search-form :deep(.el-input__wrapper:hover),
.search-form :deep(.el-select__wrapper:hover),
.search-form :deep(.el-date-editor.el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.search-form :deep(.el-input__wrapper.is-focus),
.search-form :deep(.el-select__wrapper.is-focused),
.search-form :deep(.el-date-editor.el-input__wrapper.is-active) {
  box-shadow: 0 0 0 1px #409eff inset;
}

.search-form :deep(.audit-toolbar-button.el-button) {
  height: 30px;
  min-height: 30px;
  padding: 0 10px;
  border-radius: 8px;
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  transform: none;
  box-shadow: none;
}

.search-form :deep(.el-button--primary) {
  border-color: #409eff;
  background: #409eff;
}

.search-form :deep(.el-button--warning) {
  border-color: #e6a23c;
  background: #e6a23c;
}

.search-form :deep(.el-button--danger) {
  border-color: #f56c6c;
  background: #f56c6c;
}

.search-form :deep(.audit-toolbar-button .el-icon) {
  color: inherit;
  font-size: 14px;
}

.action-section {
  flex-shrink: 0;
  display: flex;
  gap: 8px;
  margin: 0 0 8px;
  padding: 0 2px;
}

.action-section :deep(.el-button) {
  height: 30px;
  min-height: 30px;
  margin: 0;
  padding: 0 14px;
  border-radius: 8px;
  border-color: transparent;
  background: linear-gradient(180deg, #f47f86 0%, #ef6b73 100%);
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  transform: none;
  box-shadow: none;
}

.table-section {
  min-height: 0;
  flex: 1;
  margin: 0;
  overflow: hidden;
  border: 1px solid var(--ops-border-soft);
  border-radius: 8px;
  background: #ffffff;
}

.loginlog-table {
  height: 100%;
  border-radius: 8px;
  box-shadow: none;
  font-size: 12px;
}

.loginlog-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.loginlog-table :deep(.el-table__header),
.loginlog-table :deep(.el-table__header th) {
  background: var(--ops-bg-subtle) !important;
}

.loginlog-table :deep(.el-table__header th) {
  height: 38px;
  padding: 6px 0 !important;
  color: var(--ops-text-secondary) !important;
  font-size: 12px;
  font-weight: 700 !important;
  text-shadow: none;
}

.loginlog-table :deep(.el-table__header th .cell) {
  color: var(--ops-text-secondary) !important;
  text-shadow: none;
}

.loginlog-table :deep(.el-table__row td) {
  height: 40px;
  padding: 5px 0 !important;
  border-bottom: 1px solid #eef3f9;
}

.loginlog-table :deep(.el-table__row:hover),
.loginlog-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: #f8fafc !important;
  transform: none;
  box-shadow: none;
}

.operation-buttons .el-button {
  height: auto;
  min-height: auto;
  padding: 0;
  border: none;
  background: transparent !important;
  color: #ef6b73;
  font-size: 12px;
  font-weight: 600;
  transform: none;
  box-shadow: none;
}

.operation-buttons .el-button:hover,
.operation-buttons .el-button:active {
  transform: none;
  box-shadow: none;
}

.operation-buttons .el-button .el-icon {
  display: none;
}

.pagination-section {
  flex-shrink: 0;
  justify-content: flex-end;
  padding: 6px 2px 0;
}

.loginlog-management :deep(.el-button:hover),
.loginlog-management :deep(.el-button:active),
.loginlog-management :deep(.el-pagination button:hover),
.loginlog-management :deep(.el-pagination .el-pager li:hover) {
  transform: none;
}

:global(.audit-log-confirm.el-message-box) {
  --el-messagebox-border-radius: 15px;
  max-width: calc(100vw - 32px);
  padding: 0;
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-messagebox-border-radius) !important;
  background: #ffffff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.audit-log-confirm .el-message-box__header) {
  padding: 16px 18px 10px;
  border-bottom: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.audit-log-confirm .el-message-box__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.audit-log-confirm .el-message-box__content) {
  padding: 14px 18px;
  color: #53657d;
}

:global(.audit-log-confirm .el-message-box__btns) {
  gap: 10px;
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
}

:global(.audit-log-confirm .el-message-box__btns .el-button) {
  min-height: 34px;
  border-radius: 8px;
}

:global(.audit-log-confirm .el-message-box__btns .el-button--danger) {
  border-color: #f56c6c;
  background: #f56c6c;
  color: #ffffff;
}

:global(.audit-log-confirm .el-button:hover),
:global(.audit-log-confirm .el-button:active) {
  transform: none;
}

@media (max-width: 1200px) {
  .ops-page-header {
    flex-wrap: wrap;
  }

  .audit-header-summary {
    width: 100%;
    padding-left: 54px;
  }
}

@media (max-width: 768px) {
  :global(.home-container .el-main:has(.loginlog-management)) {
    padding-right: 6px;
    padding-left: 6px;
    overflow-y: auto;
  }

  .loginlog-management {
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

  .audit-header-summary {
    padding-left: 50px;
  }

  .loginlog-card :deep(.el-card__body) {
    overflow: visible;
  }

  .table-section {
    min-height: 460px;
  }
}
</style>
