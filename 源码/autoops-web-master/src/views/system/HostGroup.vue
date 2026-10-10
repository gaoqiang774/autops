<template>
  <div class="group-management">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/人员分组.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>堡垒机用户组</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>统一维护用户组成员及主机、K8S 集群和日志项目授权范围。</p>
      </div>
      <div class="system-header-summary" aria-label="堡垒机用户组统计">
        <span><em>用户组</em><strong>{{ pagination.total }}</strong></span>
      </div>
    </header>

    <el-card shadow="never" class="group-card">

      <!-- 搜索表单 -->
      <div class="search-section">
        <el-form :inline="true" :model="queryForm" class="search-form">
          <el-form-item label="用户组名称">
            <el-input
              v-model="queryForm.name"
              placeholder="请输入用户组名称"
              size="small"
              clearable
              class="modern-input"
            />
          </el-form-item>
          <el-form-item label="状态" style="width: 200px">
            <el-select
              v-model="queryForm.status"
              placeholder="请选择"
              clearable
              size="small"
              class="modern-select"
              style="width: 100%"
            >
              <el-option label="全部" value="" />
              <el-option label="启用" :value="1" />
              <el-option label="禁用" :value="2" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button
              type="primary"
              size="small"
              @click="handleQuery"
              class="modern-btn primary-btn system-toolbar-button"
            >
              <el-icon><Search /></el-icon>
              查询
            </el-button>
            <el-button
              type="warning"
              size="small"
              @click="handleReset"
              class="modern-btn reset-btn system-toolbar-button"
            >
              <el-icon><RefreshRight /></el-icon>
              重置
            </el-button>
            <el-button
              type="success"
              size="small"
              @click="handleCreate"
              class="modern-btn success-btn system-toolbar-button"
            >
              <el-icon><Plus /></el-icon>
              新建用户组
            </el-button>
          </el-form-item>
        </el-form>
      </div>

      <!-- 表格 -->
      <div class="table-section">
        <el-table
          v-loading="loading"
          :data="groupList"
          stripe
          class="modern-table"
        >
          <el-table-column prop="id" label="ID" width="60" />
          <el-table-column prop="name" label="用户组名称" min-width="120" />
          <el-table-column prop="description" label="描述" min-width="150" show-overflow-tooltip />
          <el-table-column prop="memberCount" label="成员" width="70" align="center">
            <template #default="{ row }">
              <el-tag size="small">{{ row.memberCount || 0 }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="资产" min-width="180" align="center">
            <template #default="{ row }">
              <div class="asset-count-group">
                <el-tag type="success" size="small" class="asset-count-tag">
                  主机 {{ getAssetCountValue(row, 'host') }}
                </el-tag>
                <el-tag type="primary" size="small" class="asset-count-tag">
                  K8S {{ getAssetCountValue(row, 'k8s') }}
                </el-tag>
                <el-tag type="warning" size="small" class="asset-count-tag">
                  日志 {{ getAssetCountValue(row, 'log') }}
                </el-tag>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="status" label="状态" width="80" align="center">
            <template #default="{ row }">
              <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
                {{ row.status === 1 ? '启用' : '禁用' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="createTime" label="创建时间" width="160" />
          <el-table-column label="操作" width="280" fixed="right">
            <template #default="{ row }">
              <div class="operation-buttons">
                <el-button type="primary" size="small" @click="handleEdit(row)">
                  编辑
                </el-button>
                <el-button type="info" size="small" @click="handleMembers(row)">
                  成员
                </el-button>
                <el-button type="warning" size="small" @click="handleAuthorization(row)">
                  资产授权
                </el-button>
                <el-button type="danger" size="small" @click="handleDelete(row)">
                  删除
                </el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>

        <!-- 分页 -->
        <el-pagination
          :current-page="pagination.page"
          :page-size="pagination.pageSize"
          :total="pagination.total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="handleSizeChange"
          @current-change="handlePageChange"
          class="modern-pagination"
        />
      </div>
    </el-card>

    <!-- 创建/编辑对话框 -->
    <el-dialog
      :title="dialogTitle"
      v-model="dialogVisible"
      width="600px"
      class="modern-dialog"
      modal-class="host-group-dialog-overlay"
      @close="handleDialogClose"
    >
      <el-form
        ref="formRef"
        :model="formData"
        :rules="formRules"
        label-width="100px"
        class="dialog-form"
      >
        <el-form-item label="用户组名称" prop="name">
          <el-input
            v-model="formData.name"
            placeholder="请输入用户组名称"
            class="modern-input"
          />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="formData.description"
            type="textarea"
            :rows="3"
            placeholder="请输入描述"
            class="modern-input"
          />
        </el-form-item>
        <el-form-item label="成员" prop="memberIds">
          <el-select
            v-model="formData.memberIds"
            multiple
            filterable
            remote
            placeholder="搜索并选择用户"
            :remote-method="searchUsers"
            :loading="userLoading"
            style="width: 100%;"
            class="modern-select"
          >
            <el-option
              v-for="user in userOptions"
              :key="user.id"
              :label="`${user.nickname || user.username} (${user.username})`"
              :value="user.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="状态" prop="status">
          <el-radio-group v-model="formData.status" class="modern-radio-group">
            <el-radio :label="1">启用</el-radio>
            <el-radio :label="2">禁用</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="dialogVisible = false" class="modern-btn secondary-btn">取消</el-button>
          <el-button type="primary" :loading="submitLoading" @click="handleSubmit" class="modern-btn primary-btn">
            确定
          </el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 成员管理对话框 -->
    <el-dialog
      title="成员管理"
      v-model="memberDialogVisible"
      width="800px"
      class="modern-dialog"
      modal-class="host-group-dialog-overlay"
    >
      <div style="margin-bottom: 16px;">
        <el-button type="success" size="small" @click="handleAddMembers" class="modern-btn success-btn">
          <el-icon><Plus /></el-icon>
          添加成员
        </el-button>
        <el-button
          type="danger"
          size="small"
          :disabled="selectedMembers.length === 0"
          @click="handleRemoveMembers"
          class="modern-btn danger-btn"
        >
          <el-icon><Delete /></el-icon>
          移除成员 ({{ selectedMembers.length }})
        </el-button>
      </div>
      <el-table
        :data="memberList"
        border
        @selection-change="handleMemberSelectionChange"
        v-loading="memberLoading"
        class="modern-table"
      >
        <el-table-column type="selection" width="55" />
        <el-table-column prop="userId" label="用户ID" width="80" />
        <el-table-column prop="username" label="用户名" min-width="120" />
        <el-table-column prop="nickname" label="昵称" min-width="120" />
        <el-table-column prop="email" label="邮箱" min-width="180" />
        <el-table-column prop="phone" label="手机号" min-width="120" />
      </el-table>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="memberDialogVisible = false" class="modern-btn secondary-btn">关闭</el-button>
        </div>
      </template>
    </el-dialog>

    <!-- 添加成员对话框 -->
    <el-dialog
      title="添加成员"
      v-model="addMemberDialogVisible"
      width="600px"
      class="modern-dialog"
      modal-class="host-group-dialog-overlay"
    >
      <el-form label-width="100px" class="dialog-form">
        <el-form-item label="选择用户">
          <el-select
            v-model="selectedUserIds"
            multiple
            filterable
            remote
            placeholder="搜索并选择用户"
            :remote-method="searchUsers"
            :loading="userLoading"
            style="width: 100%;"
            class="modern-select"
          >
            <el-option
              v-for="user in userOptions"
              :key="user.id"
              :label="`${user.nickname || user.username} (${user.username})`"
              :value="user.id"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="dialog-footer">
          <el-button @click="addMemberDialogVisible = false" class="modern-btn secondary-btn">取消</el-button>
          <el-button type="primary" @click="handleConfirmAddMembers" class="modern-btn primary-btn">
            确定添加 ({{ selectedUserIds.length }})
          </el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import {
  Search,
  RefreshRight,
  Plus,
  Delete
} from '@element-plus/icons-vue'

export default {
  name: 'UserGroupManagement',
  components: {
    Search,
    RefreshRight,
    Plus,
    Delete
  },
  data() {
    return {
      loading: false,
      submitLoading: false,
      groupList: [],
      queryForm: {
        name: '',
        status: ''
      },
      pagination: {
        page: 1,
        pageSize: 10,
        total: 0
      },
      dialogVisible: false,
      dialogTitle: '',
      dialogMode: 'create',
      formData: {
        id: null,
        name: '',
        description: '',
        memberIds: [],
        status: 1
      },
      formRules: {
        name: [
          { required: true, message: '请输入用户组名称', trigger: 'blur' }
        ],
        status: [
          { required: true, message: '请选择状态', trigger: 'change' }
        ]
      },
      userLoading: false,
      userOptions: [],
      memberDialogVisible: false,
      memberLoading: false,
      memberList: [],
      currentGroupId: null,
      selectedMembers: [],
      addMemberDialogVisible: false,
      selectedUserIds: []
    }
  },
  mounted() {
    this.fetchList()
  },
  methods: {
    async fetchList() {
      this.loading = true
      try {
        const { data: res } = await this.$api.getUserGroupList({
          ...this.queryForm,
          page: this.pagination.page,
          pageSize: this.pagination.pageSize
        })
        if (res.code === 200) {
          this.groupList = res.data.list || []
          this.pagination.total = res.data.total || 0
        } else {
          this.$message.error(res.message || '获取用户组列表失败')
        }
      } catch (error) {
        this.$message.error('获取用户组列表失败')
      } finally {
        this.loading = false
      }
    },

    handleQuery() {
      this.pagination.page = 1
      this.fetchList()
    },

    handleReset() {
      this.queryForm = {
        name: '',
        status: ''
      }
      this.handleQuery()
    },

    handleSizeChange(size) {
      this.pagination.pageSize = size
      this.fetchList()
    },

    handlePageChange(page) {
      this.pagination.page = page
      this.fetchList()
    },

    getAssetCountValue(row, type) {
      const counts = row.assetCounts || row.asset_counts || {}
      const countMap = {
        host: [row.hostCount, row.host_count, counts.host, counts.hostCount],
        k8s: [row.k8sClusterCount, row.k8s_count, row.k8sCount, counts.k8s_cluster, counts.k8sCluster, counts.k8s],
        log: [row.logProjectCount, row.log_project_count, row.logCount, counts.log_project, counts.logProject, counts.log]
      }
      const value = (countMap[type] || []).find(item => item !== undefined && item !== null)
      return Number(value || 0)
    },

    handleCreate() {
      this.dialogMode = 'create'
      this.dialogTitle = '新建用户组'
      this.dialogVisible = true
      this.loadUserOptions()
    },

    handleEdit(row) {
      this.dialogMode = 'edit'
      this.dialogTitle = '编辑用户组'
      this.formData = {
        id: row.id,
        name: row.name,
        description: row.description,
        memberIds: row.memberIds || [],
        status: row.status
      }
      this.dialogVisible = true
      this.loadUserOptions()
    },

    async handleSubmit() {
      this.$refs.formRef.validate(async valid => {
        if (!valid) return

        this.submitLoading = true
        try {
          let res
          if (this.dialogMode === 'create') {
            res = await this.$api.createUserGroup(this.formData)
          } else {
            res = await this.$api.updateUserGroup(this.formData)
          }

          if (res.data.code === 200) {
            this.$message.success(this.dialogMode === 'create' ? '创建成功' : '更新成功')
            this.dialogVisible = false
            this.fetchList()
          } else {
            this.$message.error(res.data.message || '操作失败')
          }
        } catch (error) {
          this.$message.error(error.message || '操作失败')
        } finally {
          this.submitLoading = false
        }
      })
    },

    handleDialogClose() {
      this.$refs.formRef.resetFields()
      this.formData = {
        id: null,
        name: '',
        description: '',
        memberIds: [],
        status: 1
      }
      this.userOptions = []
    },

    async handleMembers(row) {
      this.currentGroupId = row.id
      this.memberDialogVisible = true
      await this.fetchGroupMembers(row.id)
    },

    async fetchGroupMembers(groupId) {
      this.memberLoading = true
      try {
        const { data: res } = await this.$api.getUserGroupDetail(groupId)
        if (res.code === 200) {
          this.memberList = res.data.members || []
        } else {
          this.$message.error(res.message || '获取成员列表失败')
        }
      } catch (error) {
        this.$message.error('获取成员列表失败')
      } finally {
        this.memberLoading = false
      }
    },

    handleMemberSelectionChange(selection) {
      this.selectedMembers = selection
    },

    handleAddMembers() {
      this.selectedUserIds = []
      this.addMemberDialogVisible = true
      this.loadUserOptions()
    },

    async handleConfirmAddMembers() {
      if (this.selectedUserIds.length === 0) {
        this.$message.warning('请选择要添加的用户')
        return
      }

      try {
        const { data: res } = await this.$api.addGroupMembers({
          groupId: this.currentGroupId,
          userIds: this.selectedUserIds
        })
        if (res.code === 200) {
          this.$message.success('添加成员成功')
          this.addMemberDialogVisible = false
          await this.fetchGroupMembers(this.currentGroupId)
          await this.fetchList()
        } else {
          this.$message.error(res.message || '添加成员失败')
        }
      } catch (error) {
        this.$message.error('添加成员失败')
      }
    },

    async handleRemoveMembers() {
      if (this.selectedMembers.length === 0) {
        this.$message.warning('请选择要移除的成员')
        return
      }

      try {
        await this.$confirm(
          `确定要移除选中的 ${this.selectedMembers.length} 个成员吗？`,
          '提示',
          {
            confirmButtonText: '确定',
            cancelButtonText: '取消',
            type: 'warning',
            customClass: 'host-access-confirm-dialog',
            confirmButtonClass: 'el-button--danger'
          }
        )

        const userIds = this.selectedMembers.map(m => m.userId)
        const { data: res } = await this.$api.removeGroupMembers({
          groupId: this.currentGroupId,
          userIds
        })

        if (res.code === 200) {
          this.$message.success('移除成员成功')
          await this.fetchGroupMembers(this.currentGroupId)
          await this.fetchList()
        } else {
          this.$message.error(res.message || '移除成员失败')
        }
      } catch (error) {
        if (error !== 'cancel') {
          this.$message.error('移除成员失败')
        }
      }
    },

    handleAuthorization(row) {
      this.$router.push({
        name: 'BastionAuthorization',
        query: {
          subjectType: 'group',
          subjectId: row.id
        }
      })
    },

    async handleDelete(row) {
      try {
        await this.$confirm(
          `确定要删除用户组"${row.name}"吗？删除后该用户组的所有授权关系也将被删除。`,
          '警告',
          {
            confirmButtonText: '确定',
            cancelButtonText: '取消',
            type: 'warning',
            customClass: 'host-access-confirm-dialog',
            confirmButtonClass: 'el-button--danger'
          }
        )

        const { data: res } = await this.$api.deleteUserGroup({ id: row.id })
        if (res.code === 200) {
          this.$message.success('删除成功')
          this.fetchList()
        } else {
          this.$message.error(res.message || '删除失败')
        }
      } catch (error) {
        if (error !== 'cancel') {
          this.$message.error('删除失败')
        }
      }
    },

    async loadUserOptions() {
      this.userLoading = true
      try {
        const { data: res } = await this.$api.getUserList({
          page: 1,
          pageSize: 100
        })
        if (res.code === 200) {
          this.userOptions = res.data.list || []
        }
      } catch (error) {
        console.error('加载用户列表失败:', error)
      } finally {
        this.userLoading = false
      }
    },

    async searchUsers(query) {
      if (!query) {
        await this.loadUserOptions()
        return
      }

      this.userLoading = true
      try {
        const { data: res } = await this.$api.getUserList({
          keyword: query,
          page: 1,
          pageSize: 20
        })
        if (res.code === 200) {
          this.userOptions = res.data.list || []
        }
      } catch (error) {
        console.error('搜索用户失败:', error)
      } finally {
        this.userLoading = false
      }
    }
  }
}
</script>

<style lang="less" scoped>
.group-management {
  padding: 5px;
  min-height: 100vh;
  background: #eef1f6;
}

.group-card {
  background: rgba(255, 255, 255, 0.95);
  backdrop-filter: blur(10px);
  border-radius: 16px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.2);
  transition: all 0.3s ease;

  :deep(.el-card__header) {
    border-bottom: 1px solid rgba(103, 126, 234, 0.1);
    padding: 20px 24px;
  }

  :deep(.el-card__body) {
    padding: 20px 24px;
  }
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.title {
  font-size: 18px;
  font-weight: 600;
  color: #2c3e50;
  background: linear-gradient(45deg, #667eea, #764ba2);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  margin: 0;
}

.search-section {
  margin-bottom: 16px;
  padding: 16px;
  background: rgba(103, 126, 234, 0.05);
  border-radius: 12px;
  border: 1px solid rgba(103, 126, 234, 0.1);

  .search-form {
    display: flex;
    flex-wrap: wrap;
    gap: 16px;
    width: 100%;

    :deep(.el-form-item) {
      margin-bottom: 0;
    }

    :deep(.el-form-item__label) {
      color: #606266;
      font-weight: 500;
    }
  }
}

.action-section {
  margin-bottom: 16px;
  padding: 12px 0;

  .el-button {
    margin-right: 12px;
  }
}

.table-section {
  margin-bottom: 20px;
  flex: 1;
}

.modern-btn {
  border-radius: 8px;
  padding: 8px 20px;
  font-weight: 500;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  border: none;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);

  &:hover:not(:disabled) {
    transform: translateY(-2px);
    box-shadow: 0 6px 20px rgba(0, 0, 0, 0.2);
  }
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

.danger-btn {
  background: linear-gradient(45deg, #FF4D4F, #FF7875);
  color: white;

  &:hover:not(:disabled) {
    background: linear-gradient(45deg, #FF2629, #FF5E61);
  }

  &:disabled {
    background: linear-gradient(45deg, #ccc 0%, #bbb 100%);
    cursor: not-allowed;
    transform: none;
    box-shadow: none;
  }
}

.secondary-btn {
  background: #fff;
  color: #606266;
  border: 1px solid #dcdfe6;

  &:hover {
    border-color: #667eea;
    background: rgba(103, 126, 234, 0.05);
  }
}

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

.modern-select {
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
    }

    .el-table__row {
      transition: all 0.3s ease;

      td {
        padding: 12px !important;
      }

      &:hover {
        background: rgba(103, 126, 234, 0.05) !important;
        transform: translateY(-2px);
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
      }
    }
  }
}

.operation-buttons {
  display: flex;
  justify-content: center;
  gap: 4px;
  flex-wrap: wrap;

  .el-button {
    padding: 4px 12px;
    font-size: 12px;
    margin: 2px;
  }
}

.modern-pagination {
  margin-top: 20px;
  display: flex;
  justify-content: center;

  :deep(.btn-next),
  :deep(.btn-prev),
  :deep(.el-pager li) {
    border-radius: 6px;
    margin: 0 2px;
    transition: all 0.3s ease;

    &:hover {
      transform: translateY(-2px);
      box-shadow: 0 4px 8px rgba(0, 0, 0, 0.1);
    }
  }
}

.modern-dialog {
  :deep(.el-dialog) {
    background: #ffffff;
    border-radius: 16px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.2);

    .el-dialog__header {
      background: linear-gradient(135deg, #667eea, #764ba2);
      border-radius: 16px 16px 0 0;
      padding: 20px 24px;

      .el-dialog__title {
        color: #ffffff;
        font-weight: 600;
        font-size: 16px;
      }

      .el-dialog__headerbtn {
        .el-dialog__close {
          color: #ffffff;
          font-size: 20px;

          &:hover {
            color: #f0f0f0;
          }
        }
      }
    }

    .el-dialog__body {
      padding: 24px;
    }

    .el-dialog__footer {
      border-top: 1px solid #f0f0f0;
      padding: 16px 24px;
    }
  }
}

.dialog-form {
  :deep(.el-form-item__label) {
    color: #606266;
    font-weight: 500;
  }

  :deep(.el-form-item__content) {
    .el-input__wrapper {
      background: #ffffff;
      border: 1px solid #dcdfe6;
      border-radius: 8px;
      box-shadow: none;

      &:hover {
        border-color: #c0c4cc;
      }

      &.is-focus {
        border-color: #409eff;
        box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.2);
      }
    }

    .el-input__inner,
    .el-textarea__inner {
      background: transparent;
      border: none;
      color: #606266;

      &::placeholder {
        color: #c0c4cc;
      }
    }
  }
}

.modern-radio-group {
  :deep(.el-radio) {
    margin-right: 30px;

    .el-radio__label {
      color: #333;
      font-weight: 500;
    }

    .el-radio__input.is-checked .el-radio__inner {
      background-color: #667eea;
      border-color: #667eea;
    }

    &:hover .el-radio__inner {
      border-color: #667eea;
    }
  }
}

.dialog-footer {
  text-align: center;
  padding-top: 12px;
  display: flex;
  align-items: center;
  justify-content: flex-end;

  .modern-btn {
    margin-left: 8px;
  }
}

.el-tag {
  font-weight: 500;
  border-radius: 8px;
  border: none;
}

@media (max-width: 768px) {
  .group-management {
    padding: 12px;
  }

  .group-card {
    :deep(.el-card__header) {
      padding: 16px;
    }

    :deep(.el-card__body) {
      padding: 16px;
    }
  }

  .modern-dialog :deep(.el-dialog) {
    width: 90% !important;
    margin: 5vh auto;
  }

  .search-section {
    padding: 12px;

    .search-form {
      gap: 8px;
    }
  }
}

/* DESIGN.md compact system page overrides */
.group-management {
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

:global(.home-container .el-main:has(.group-management)) {
  overflow-y: hidden;
}

.group-card {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: 22px;
  border: 1px solid var(--sys-border);
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.96), rgba(247, 250, 255, 0.92));
  box-shadow: 0 20px 60px rgba(17, 24, 39, 0.07);
}

.group-card::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 40px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  pointer-events: none;
}

.group-card :deep(.el-card__header) {
  position: relative;
  z-index: 1;
  padding: 16px 20px 12px;
  border-bottom: none;
  background: transparent;
}

.group-card :deep(.el-card__body) {
  position: relative;
  z-index: 1;
  min-height: 0;
  display: flex;
  flex: 1;
  flex-direction: column;
  padding: 0 10px 10px;
  overflow: hidden;
}

.card-header {
  margin: 0;
  padding: 0;
  border: none;
}

.card-header::before {
  content: 'System Center';
  display: block;
  margin-bottom: 3px;
  color: var(--sys-brand);
  font-size: 12px;
  font-weight: 700;
}

.title {
  display: block;
  color: var(--sys-text-primary);
  background: none;
  -webkit-text-fill-color: currentColor;
  font-size: 20px;
  line-height: 1.25;
  font-weight: 700;
}

.search-section {
  flex-shrink: 0;
  margin-bottom: 8px;
  padding: 12px 14px 2px;
  border-radius: 16px;
  border: 1px solid rgba(22, 34, 56, 0.06);
  background: rgba(255, 255, 255, 0.58);
}

.search-section .search-form {
  gap: 0 12px;
}

.search-section .search-form :deep(.el-form-item) {
  margin-bottom: 10px;
}

.action-section {
  margin: 0 0 8px;
  padding: 0 2px;
}

.table-section {
  min-height: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  margin: 0;
  overflow: hidden;
  border-radius: 16px;
  border: 1px solid var(--sys-border);
  background: rgba(255, 255, 255, 0.82);
}

.modern-table {
  flex: 1;
  min-height: 0;
  border-radius: 16px;
  box-shadow: none;
  color: var(--sys-text-secondary);
  font-size: 13px;
}

.modern-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.modern-table :deep(th.el-table__cell) {
  height: 38px;
  padding: 6px 0 !important;
  background: rgba(248, 250, 255, 0.96) !important;
  color: var(--sys-text-muted) !important;
  border-bottom: 1px solid var(--sys-border);
  font-size: 12px;
  font-weight: 700 !important;
  text-shadow: none;
}

.modern-table :deep(td.el-table__cell) {
  height: 42px;
  padding: 6px 0 !important;
  border-bottom: 1px solid rgba(15, 23, 42, 0.04);
  color: var(--sys-text-secondary);
}

.modern-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: rgba(76, 139, 245, 0.045) !important;
}

.asset-count-group {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  flex-wrap: nowrap;
}

.asset-count-tag {
  min-width: 48px;
  justify-content: center;
  border-radius: 7px;
  font-weight: 600;
}

.operation-buttons {
  flex-wrap: nowrap;
  justify-content: flex-start;
  gap: 14px;
  white-space: nowrap;
}

.operation-buttons .el-button {
  min-height: auto;
  margin: 0;
  padding: 0;
  border: none;
  background: transparent !important;
  box-shadow: none;
  color: var(--sys-brand);
  font-size: 12px;
  font-weight: 600;
}

.operation-buttons .el-button .el-icon {
  display: none;
}

.operation-buttons .el-button--danger {
  color: var(--sys-danger);
}

.operation-buttons .el-button--warning,
.operation-buttons .el-button--info {
  color: #b77b24;
}

.modern-pagination {
  flex-shrink: 0;
  justify-content: flex-end;
  margin: 0;
  padding: 8px 2px 0;
}

.modern-btn,
.el-button {
  min-height: 32px;
  border-radius: 14px;
  font-weight: 600;
  box-shadow: none;
}

.primary-btn,
.el-button--primary {
  border-color: transparent;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
}

.success-btn,
.el-button--success {
  border-color: transparent;
  background: linear-gradient(180deg, #51c78d 0%, #3fb97f 100%);
}

.reset-btn,
.el-button--warning {
  border-color: transparent;
  background: linear-gradient(180deg, #f6be63 0%, #f3b24b 100%);
  color: #fff;
}

.modern-input :deep(.el-input__wrapper),
.modern-select :deep(.el-input__wrapper),
.dialog-form :deep(.el-input__wrapper),
.dialog-form :deep(.el-textarea__inner) {
  min-height: 32px;
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 14px;
  background: rgba(252, 253, 255, 0.98);
  box-shadow: none;
}

.modern-dialog :deep(.el-dialog) {
  border-radius: 28px;
  border: 1px solid rgba(223, 231, 242, 0.96);
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.98), rgba(247, 250, 255, 0.94));
  box-shadow: 0 36px 110px rgba(15, 23, 42, 0.11);
  overflow: hidden;
}

.modern-dialog :deep(.el-dialog__header) {
  margin: 0;
  padding: 20px 24px 14px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.82);
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
}

.modern-dialog :deep(.el-dialog__title) {
  color: var(--sys-text-primary);
  font-size: 18px;
  font-weight: 700;
}

.dialog-footer {
  padding-top: 0;
}

/* DESIGN.md final bastion host group overrides */
:global(.home-container .el-main:has(.group-management)) {
  padding-top: 10px;
  padding-right: 10px;
  padding-left: 10px;
  overflow-y: hidden;
}

.group-management {
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

.system-header-summary span {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  white-space: nowrap;
}

.system-header-summary em {
  color: var(--ops-text-muted);
  font-size: 12px;
  font-style: normal;
}

.system-header-summary strong {
  color: var(--ops-text-heading);
  font-family: Consolas, Monaco, monospace;
  font-size: 16px;
  line-height: 1;
}

.group-card {
  position: relative;
  display: flex;
  min-width: 0;
  min-height: 0;
  height: 100%;
  flex-direction: column;
  overflow: hidden !important;
  border: 1px solid var(--ops-border);
  border-radius: 12px !important;
  background: var(--ops-bg-surface);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
  backdrop-filter: none;
}

.group-card::before {
  display: none;
}

.group-card :deep(.el-card__body) {
  box-sizing: border-box;
  display: flex;
  min-height: 0;
  height: 100%;
  flex: 1;
  flex-direction: column;
  padding: 8px 10px 10px;
  overflow: hidden;
}

.search-section {
  margin-bottom: 8px;
  padding: 8px 10px 2px;
  border: 1px solid var(--ops-border-soft);
  border-radius: 10px;
  background: var(--ops-bg-subtle);
}

.search-section .search-form {
  align-items: center;
  gap: 0 10px;
}

.search-section .search-form :deep(.el-form-item) {
  margin-right: 0;
  margin-bottom: 6px;
}

.search-section .search-form :deep(.el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

.search-section :deep(.modern-input .el-input__wrapper),
.search-section :deep(.modern-select .el-select__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 7px;
  background: #ffffff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.search-section :deep(.modern-input .el-input__wrapper:hover),
.search-section :deep(.modern-select .el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.search-section :deep(.system-toolbar-button.el-button) {
  height: 30px;
  min-height: 30px;
  padding: 0 10px;
  border-radius: 8px;
  color: #ffffff;
  font-size: 12px;
  box-shadow: none;
}

.search-section :deep(.primary-btn.system-toolbar-button) {
  border-color: #409eff;
  background: #409eff;
}

.search-section :deep(.reset-btn.system-toolbar-button) {
  border-color: #e6a23c;
  background: #e6a23c;
}

.search-section :deep(.success-btn.system-toolbar-button) {
  border-color: #67c23a;
  background: #67c23a;
}

.search-section :deep(.system-toolbar-button .el-icon) {
  color: inherit;
  font-size: 14px;
}

.table-section {
  min-width: 0;
  min-height: 0;
  flex: 1;
  margin: 0;
  overflow: hidden;
  border: 1px solid var(--ops-border-soft);
  border-radius: 8px;
  background: #ffffff;
}

.modern-table,
.modern-table :deep(.el-table__inner-wrapper) {
  border-radius: 8px;
}

.modern-table {
  font-size: 12px;
}

.modern-table :deep(th.el-table__cell) {
  height: 38px;
  background: var(--ops-bg-subtle) !important;
  color: var(--ops-text-secondary) !important;
  border-bottom-color: #eef3f9;
}

.modern-table :deep(td.el-table__cell) {
  height: 40px;
  padding: 5px 0 !important;
  border-bottom-color: #eef3f9;
}

.modern-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

.operation-buttons .el-button:hover,
.operation-buttons .el-button:active,
.group-management :deep(.el-button:hover),
.group-management :deep(.el-button:active),
.group-management :deep(.el-pagination button:hover),
.group-management :deep(.el-pagination .el-pager li:hover) {
  transform: none;
}

.modern-pagination {
  padding-top: 6px;
}

:global(.host-group-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 32px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: #ffffff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.host-group-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.host-group-dialog-overlay .el-dialog__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.host-group-dialog-overlay .el-dialog__body) {
  box-sizing: border-box;
  max-height: calc(100vh - 170px);
  padding: 16px 18px;
  overflow: auto;
  color: #53657d;
}

:global(.host-group-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #ffffff;
}

:global(.host-group-dialog-overlay .el-form-item__label) {
  color: #53657d;
  font-size: 13px;
  font-weight: 600;
}

:global(.host-group-dialog-overlay .el-input__wrapper),
:global(.host-group-dialog-overlay .el-select__wrapper),
:global(.host-group-dialog-overlay .el-textarea__inner) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.host-group-dialog-overlay .el-dialog__footer .dialog-footer) {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

:global(.host-group-dialog-overlay .el-dialog__footer .el-button) {
  min-height: 34px;
  border-radius: 8px;
  box-shadow: none;
}

:global(.host-group-dialog-overlay .el-dialog__footer .secondary-btn) {
  border-color: #dbe7f4;
  background: #ffffff;
  color: #53657d;
}

:global(.host-group-dialog-overlay .el-dialog__footer .primary-btn) {
  border-color: #409eff;
  background: #409eff;
  color: #ffffff;
}

:global(.host-group-dialog-overlay .el-button:hover),
:global(.host-group-dialog-overlay .el-button:active) {
  transform: none;
}

:global(.host-access-confirm-dialog.el-message-box) {
  --el-messagebox-border-radius: 15px;
  max-width: calc(100vw - 32px);
  padding: 0;
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-messagebox-border-radius) !important;
  background: #ffffff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.host-access-confirm-dialog .el-message-box__header) {
  padding: 16px 18px 10px;
  border-bottom: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.host-access-confirm-dialog .el-message-box__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.host-access-confirm-dialog .el-message-box__content) {
  padding: 14px 18px;
  color: #53657d;
}

:global(.host-access-confirm-dialog .el-message-box__btns) {
  gap: 10px;
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
}

:global(.host-access-confirm-dialog .el-message-box__btns .el-button) {
  min-height: 34px;
  border-radius: 8px;
}

:global(.host-access-confirm-dialog .el-message-box__btns .el-button--danger) {
  border-color: #f56c6c;
  background: #f56c6c;
  color: #ffffff;
}

:global(.host-access-confirm-dialog .el-button:hover),
:global(.host-access-confirm-dialog .el-button:active) {
  transform: none;
}

@media (max-width: 1200px) {
  .ops-page-header {
    flex-wrap: wrap;
  }

  .system-header-summary {
    width: 100%;
    padding-left: 54px;
  }
}

@media (max-width: 768px) {
  :global(.home-container .el-main:has(.group-management)) {
    padding-right: 6px;
    padding-left: 6px;
    overflow-y: auto;
  }

  .group-management {
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

  .system-header-summary {
    padding-left: 50px;
  }

  .group-card :deep(.el-card__body) {
    overflow: visible;
  }

  .table-section {
    min-height: 480px;
  }
}
</style>
