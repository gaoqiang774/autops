<template>
  <div class="personal-management">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/账号.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>个人中心</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>维护当前账号的基本资料、头像与登录密码。</p>
      </div>
    </header>

    <div class="personal-workspace">
      <aside class="personal-card profile-card">
        <header class="section-header">
          <div>
            <h2>个人信息</h2>
            <p>当前登录账号概览</p>
          </div>
        </header>

        <div class="profile-summary">
          <el-avatar :src="adminDetail.icon" :size="96" :key="avatarKey" class="profile-avatar">
            <el-icon><UserFilled /></el-icon>
          </el-avatar>
          <strong>{{ adminDetail.nickname || adminDetail.username || '未设置昵称' }}</strong>
          <span>{{ adminDetail.username || '未设置账号' }}</span>
        </div>

        <dl class="profile-list">
          <div>
            <dt>用户账号</dt>
            <dd :title="adminDetail.username">{{ adminDetail.username || '-' }}</dd>
          </div>
          <div>
            <dt>用户昵称</dt>
            <dd :title="adminDetail.nickname">{{ adminDetail.nickname || '-' }}</dd>
          </div>
          <div>
            <dt>用户邮箱</dt>
            <dd :title="adminDetail.email">{{ adminDetail.email || '-' }}</dd>
          </div>
          <div>
            <dt>用户备注</dt>
            <dd :title="adminDetail.note">{{ adminDetail.note || '-' }}</dd>
          </div>
          <div>
            <dt>创建时间</dt>
            <dd :title="adminDetail.createTime">{{ adminDetail.createTime || '-' }}</dd>
          </div>
        </dl>
      </aside>

      <section class="personal-card detail-card">
        <header class="section-header">
          <div>
            <h2>账户设置</h2>
            <p>更新资料后将同步到当前账号</p>
          </div>
        </header>

        <div class="tabs-section">
          <el-tabs v-model="activeName" class="personal-tabs">
            <el-tab-pane label="基本资料" name="first">
              <el-form
                ref="editFormRefForm"
                :model="adminDetail"
                :rules="editFormRules"
                label-position="top"
                class="edit-form form-grid"
              >
                <el-form-item label="用户头像" prop="icon" class="avatar-form-item form-field-full">
                  <el-upload
                    :headers="headers"
                    :action="uploadIconUrl"
                    :show-file-list="false"
                    :on-success="handleAvatarSuccess"
                    class="avatar-uploader"
                    aria-label="上传用户头像"
                  >
                    <el-avatar :src="icon || adminDetail.icon" :size="84" :key="avatarKey" title="点击更换头像">
                      <el-icon><UserFilled /></el-icon>
                    </el-avatar>
                    <span class="avatar-edit-mark" aria-hidden="true"><el-icon><Camera /></el-icon></span>
                  </el-upload>
                </el-form-item>
                <el-form-item label="用户账号" prop="username">
                  <el-input v-model="adminDetail.username" />
                </el-form-item>
                <el-form-item label="用户昵称" prop="nickname">
                  <el-input v-model="adminDetail.nickname" />
                </el-form-item>
                <el-form-item label="用户邮箱" prop="email">
                  <el-input v-model="adminDetail.email" maxlength="50" />
                </el-form-item>
                <el-form-item label="用户备注" prop="note" class="form-field-full">
                  <el-input v-model="adminDetail.note" />
                </el-form-item>
                <el-form-item class="form-actions form-field-full">
                  <el-button type="primary" :loading="profileSubmitting" @click="submitFirst">
                    <el-icon><Check /></el-icon>保存
                  </el-button>
                  <el-button type="warning" @click="closeFirst">
                    <el-icon><Close /></el-icon>关闭
                  </el-button>
                </el-form-item>
              </el-form>
            </el-tab-pane>

            <el-tab-pane label="修改密码" name="second">
              <el-form
                ref="updateFormRefForm"
                :model="updateForm"
                :rules="updateFormRules"
                label-position="top"
                class="edit-form password-form form-grid"
              >
                <el-form-item label="旧密码" prop="password">
                  <el-input v-model="updateForm.password" placeholder="请输入旧密码" type="password" autocomplete="current-password" show-password />
                </el-form-item>
                <el-form-item label="新密码" prop="newPassword">
                  <el-input v-model="updateForm.newPassword" placeholder="请输入新密码" type="password" autocomplete="new-password" show-password />
                </el-form-item>
                <el-form-item label="确认密码" prop="resetPassword">
                  <el-input v-model="updateForm.resetPassword" placeholder="请再次输入新密码" type="password" autocomplete="new-password" show-password />
                </el-form-item>
                <el-form-item class="form-actions form-field-full">
                  <el-button type="primary" :loading="passwordSubmitting" @click="submitSecond">
                    <el-icon><Check /></el-icon>保存
                  </el-button>
                  <el-button type="warning" @click="closeSecond">
                    <el-icon><Close /></el-icon>关闭
                  </el-button>
                </el-form-item>
              </el-form>
            </el-tab-pane>
          </el-tabs>
        </div>
      </section>
    </div>
  </div>
</template>

<script>
import storage from '@/utils/storage'
import { Camera, Check, Close, UserFilled } from '@element-plus/icons-vue'

export default {
  name: 'SystemPersonal',
  components: {
    Camera,
    Check,
    Close,
    UserFilled
  },
  data() {
    return {
      adminDetail: storage.getItem("sysAdmin") || {},
      activeName: 'first',
      uploadIconUrl: '/api/v1/upload', // 直接使用字符串而不是调用方法
      headers: {
        Authorization: "Bearer " + storage.getItem("token"),
      },
      icon: '',
      avatarKey: Date.now(),
      profileSubmitting: false,
      passwordSubmitting: false,
      editFormRules: {
        username: [{ required: true, message: '请输入用户账号', trigger: 'blur' }],
        email: [{ required: true, message: '请输入用户邮箱', trigger: 'blur' }],
        nickname: [{ required: true, message: '请输入用户昵称', trigger: 'blur' }],
        note: [{ required: true, message: '请输入用户备注', trigger: 'blur' }],
      },
      updateForm: {
        password: '',
        newPassword: '',
        resetPassword: ''
      },
      updateFormRules: {
        password: [{ required: true, message: '请输入旧密码', trigger: 'blur' }],
        newPassword: [{ required: true, message: '请输入新密码', trigger: 'blur' }],
        resetPassword: [{ required: true, message: '请输入重复密码', trigger: 'blur' }],
      }
    }
  },
  mounted() {
    this.loadAdminDetail()
  },
  methods: {
    async loadAdminDetail() {
      const cachedAdmin = storage.getItem('sysAdmin') || {}
      const adminId = cachedAdmin.id || this.adminDetail.id
      if (!adminId) {
        this.$message.error('无法获取当前用户信息')
        return
      }

      try {
        const { data: res } = await this.$api.adminInfo(adminId)
        if (res.code !== 200) {
          this.$message.error(res.message)
          return
        }

        this.adminDetail = { ...cachedAdmin, ...res.data }
        this.icon = ''
        this.avatarKey = Date.now()
        this.$store.commit('saveSysAdmin', this.adminDetail)
      } catch (error) {
        console.error('获取个人信息失败:', error)
      }
    },
    // 成功调用图片信息
    handleAvatarSuccess(res) {
      this.icon = res.data;
      this.adminDetail.icon = res.data;
      this.avatarKey = Date.now();
    },
    // 修改个人信息
    submitFirst() {
      this.$refs.editFormRefForm.validate(async valid => {
        if (!valid) return
        this.profileSubmitting = true
        try {
          const { data: res } = await this.$api.adminUpdatePersonal({
            icon: this.icon === '' ? this.adminDetail.icon : this.icon,
            username: this.adminDetail.username,
            email: this.adminDetail.email,
            nickname: this.adminDetail.nickname,
            note: this.adminDetail.note,
          })
          if (res.code !== 200) {
            this.$message.error(res.message)
          } else {
            this.$storage.clearItem("sysAdmin")
            this.$store.commit('saveSysAdmin', res.data)
            this.adminDetail = res.data
            this.icon = ''
            this.avatarKey = Date.now()
            this.$message.success('修改用户成功')
          }
        } finally {
          this.profileSubmitting = false
        }
      })
    },
    closeFirst() {
      this.$router.push('/home')
    },
    // 修改个人密码
    submitSecond() {
      this.$refs.updateFormRefForm.validate(async valid => {
        if (!valid) return
        this.passwordSubmitting = true
        try {
          const { data: res } = await this.$api.adminUpdatePersonalPassword({
            password: this.updateForm.password,
            newPassword: this.updateForm.newPassword,
            resetPassword: this.updateForm.resetPassword
          })
          if (res.code !== 200) {
            this.$message.error(res.message)
          } else {
            this.$storage.clearAll()
            this.$router.push("/login")
            this.$message.success('修改密码成功')
          }
        } finally {
          this.passwordSubmitting = false
        }
      })
    },
    // 关闭页面跳转到首页
    closeSecond() {
      this.$router.push('/home')
    }
  }
}
</script>

<style scoped>
.personal-management {
  padding: 20px;
  min-height: 100vh;
  background: #eef1f6;
}

.profile-card,
.detail-card {
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
   background-clip: text;
  -webkit-text-fill-color: transparent;
}

.profile-content {
  padding: 20px 0;
}

.avatar-section {
  text-align: center;
  margin-bottom: 24px;
}

.profile-avatar {
  border: 4px solid rgba(103, 126, 234, 0.2);
  box-shadow: 0 4px 16px rgba(103, 126, 234, 0.3);
  transition: all 0.3s ease;
}

.profile-avatar:hover {
  transform: none;
  box-shadow: none;
}

.profile-form .el-form-item {
  margin-bottom: 16px;
}

.profile-form .el-form-item__label {
  color: #606266;
  font-weight: 500;
}

.profile-value {
  color: #2c3e50;
  font-weight: 500;
  padding: 4px 8px;
  background: rgba(103, 126, 234, 0.1);
  border-radius: 6px;
  display: inline-block;
  min-width: 120px;
}

.tabs-section {
  padding: 20px 0;
}

.personal-tabs {
  background: transparent;
}

.personal-tabs :deep(.el-tabs__header) {
  margin-bottom: 24px;
}

.personal-tabs :deep(.el-tabs__nav-wrap::after) {
  background: linear-gradient(90deg, #667eea, #764ba2);
  height: 2px;
}

.personal-tabs :deep(.el-tabs__item) {
  color: #606266;
  font-weight: 500;
  font-size: 16px;
  transition: all 0.3s ease;
}

.personal-tabs :deep(.el-tabs__item:hover) {
  color: #667eea;
}

.personal-tabs :deep(.el-tabs__item.is-active) {
  color: #667eea;
  font-weight: 600;
}

.personal-tabs :deep(.el-tabs__active-bar) {
  background: linear-gradient(90deg, #667eea, #764ba2);
}

.edit-form {
  padding: 20px;
  background: rgba(103, 126, 234, 0.02);
  border-radius: 12px;
}

.edit-form .el-form-item {
  margin-bottom: 24px;
}

.edit-form .el-form-item__label {
  color: #606266;
  font-weight: 500;
}

.form-actions {
  text-align: center;
  margin-top: 32px;
  padding-top: 24px;
  border-top: 1px solid rgba(103, 126, 234, 0.1);
}

.form-actions .el-button {
  margin: 0 12px;
}

.avatar-uploader {
  border: 2px dashed rgba(103, 126, 234, 0.3);
  border-radius: 12px;
  cursor: pointer;
  width: 80px;
  height: 80px;
  position: relative;
  overflow: hidden;
  transition: all 0.3s ease;
  background: rgba(103, 126, 234, 0.05);
}

.avatar-uploader:hover {
  border-color: #667eea;
  background: rgba(103, 126, 234, 0.1);
  transform: none;
  box-shadow: none;
}

.avatar-uploader .avatar {
  width: 80px;
  height: 80px;
  display: block;
  border-radius: 8px;
}

.el-button {
  border-radius: 8px;
  font-weight: 500;
  transition: all 0.3s ease;
  padding: 8px 24px;
}

.el-button:hover {
  transform: none;
  box-shadow: none;
}

.el-input :deep(.el-input__inner) {
  border-radius: 8px;
  border: 1px solid rgba(103, 126, 234, 0.2);
  transition: all 0.3s ease;
  background: rgba(255, 255, 255, 0.8);
  color: #2c3e50;
}

.el-input :deep(.el-input__inner):focus {
  border-color: #667eea;
  box-shadow: 0 0 0 2px rgba(103, 126, 234, 0.2);
  background: rgba(255, 255, 255, 1);
}

.el-input :deep(.el-input__inner::placeholder) {
  color: rgba(44, 62, 80, 0.6);
}

.el-card :deep(.el-card__body) {
  padding: 24px;
}

@media (max-width: 768px) {
  .personal-management {
    padding: 10px;
  }
  
  .profile-avatar {
    width: 100px !important;
    height: 100px !important;
  }
  
  .edit-form {
    padding: 16px;
  }
  
  .edit-form .el-form-item__label {
    width: 80px !important;
  }
}

/* DESIGN.md compact system page overrides */
.personal-management {
  --sys-page-offset: 136px;
  --sys-text-primary: #172133;
  --sys-text-secondary: #5f6b7c;
  --sys-text-muted: #8d97a8;
  --sys-border: rgba(223, 231, 242, 0.92);
  --sys-brand: #4c8bf5;
  --sys-warning: #f3b24b;
  box-sizing: border-box;
  height: calc(100vh - var(--sys-page-offset));
  min-height: 0;
  padding: 6px;
  overflow: hidden;
  background: transparent;
  color: var(--sys-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

:global(.home-container .el-main:has(.personal-management)) {
  overflow-y: hidden;
}

.personal-management > .el-row {
  height: 100%;
}

.personal-management > .el-row > .el-col {
  height: 100%;
  margin-bottom: 0 !important;
}

.profile-card,
.detail-card {
  position: relative;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: 22px;
  border: 1px solid var(--sys-border);
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.96), rgba(247, 250, 255, 0.92));
  box-shadow: 0 20px 60px rgba(17, 24, 39, 0.07);
  backdrop-filter: blur(18px);
}

.profile-card::before,
.detail-card::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 40px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  pointer-events: none;
}

.profile-card :deep(.el-card__header),
.detail-card :deep(.el-card__header) {
  position: relative;
  z-index: 1;
  padding: 16px 20px 12px;
  border-bottom: none;
  background: transparent;
}

.profile-card :deep(.el-card__body),
.detail-card :deep(.el-card__body) {
  position: relative;
  z-index: 1;
  min-height: 0;
  flex: 1;
  padding: 10px 18px 18px;
  overflow: auto;
}

.card-header {
  display: block;
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

.profile-content,
.tabs-section {
  padding: 0;
}

.avatar-section {
  margin-bottom: 16px;
}

.profile-avatar {
  border: 1px solid rgba(214, 223, 235, 0.95);
  box-shadow: none;
}

.profile-value {
  min-width: 0;
  padding: 0;
  background: transparent;
  color: var(--sys-text-secondary);
  font-weight: 500;
}

.profile-form .el-form-item,
.edit-form .el-form-item {
  margin-bottom: 12px;
}

.edit-form {
  padding: 14px;
  border-radius: 16px;
  border: 1px solid var(--sys-border);
  background: rgba(255, 255, 255, 0.58);
}

.personal-tabs :deep(.el-tabs__header) {
  margin-bottom: 12px;
}

.personal-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: rgba(223, 231, 242, 0.86);
}

.personal-tabs :deep(.el-tabs__item) {
  height: 34px;
  color: var(--sys-text-secondary);
  font-size: 14px;
  font-weight: 600;
}

.personal-tabs :deep(.el-tabs__item.is-active),
.personal-tabs :deep(.el-tabs__item:hover) {
  color: var(--sys-brand);
}

.personal-tabs :deep(.el-tabs__active-bar) {
  background: var(--sys-brand);
}

.form-actions {
  margin-top: 18px;
  padding-top: 14px;
  border-top: 1px solid var(--sys-border);
}

.el-button {
  min-height: 32px;
  border-radius: 14px;
  font-weight: 600;
  padding: 8px 18px;
  box-shadow: none;
}

.el-button--primary {
  border-color: transparent;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
}

.form-actions .el-button:not(.el-button--primary) {
  border-color: transparent;
  background: linear-gradient(180deg, #f6be63 0%, #f3b24b 100%);
  color: #fff;
}

.el-input :deep(.el-input__wrapper),
.el-input :deep(.el-input__inner) {
  min-height: 32px;
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 14px;
  background: rgba(252, 253, 255, 0.98);
  box-shadow: none;
}

/* DESIGN.md personal center alignment */
:global(.home-container .el-main:has(.personal-management)) {
  padding-top: var(--ops-shell-gutter, 10px);
  padding-right: var(--ops-shell-gutter, 10px);
  padding-left: var(--ops-shell-gutter, 10px);
  overflow: hidden;
}

.personal-management {
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
  min-height: 0;
  padding: 7px;
  overflow: hidden;
  background: transparent;
  color: var(--ops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
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

.personal-workspace {
  display: grid;
  grid-template-columns: minmax(260px, 300px) minmax(0, 1fr);
  gap: 8px;
  min-width: 0;
  min-height: 0;
}

.personal-card {
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: var(--ops-bg-surface);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
}

.profile-card,
.detail-card {
  display: flex;
  height: auto;
  flex-direction: column;
  border-radius: 12px;
  background: var(--ops-bg-surface);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
  backdrop-filter: none;
}

.profile-card::before,
.detail-card::before {
  display: none;
}

.section-header {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-surface);
}

.section-header h2 {
  margin: 0;
  color: var(--ops-text-heading);
  font-size: 15px;
  font-weight: 700;
  line-height: 1.25;
  letter-spacing: 0;
}

.section-header p {
  margin: 2px 0 0;
  color: var(--ops-text-muted);
  font-size: 11px;
  line-height: 1.4;
}

.profile-card {
  overflow-y: auto;
}

.profile-summary {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 14px 12px 12px;
  border-bottom: 1px solid var(--ops-border-soft);
}

.profile-avatar {
  border: 1px solid #dbe7f4;
  background: #f3f7fc;
  box-shadow: none;
  color: #64748b;
  font-size: 32px;
  transition: none;
}

.profile-avatar:hover {
  transform: none;
  box-shadow: none;
}

.profile-summary strong {
  max-width: 100%;
  margin-top: 8px;
  overflow: hidden;
  color: var(--ops-text-heading);
  font-size: 15px;
  font-weight: 700;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.profile-summary > span {
  max-width: 100%;
  margin-top: 2px;
  overflow: hidden;
  color: var(--ops-text-muted);
  font-size: 12px;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.profile-list {
  margin: 0;
  padding: 4px 12px 10px;
}

.profile-list > div {
  display: grid;
  grid-template-columns: 76px minmax(0, 1fr);
  align-items: center;
  gap: 10px;
  min-height: 39px;
  border-bottom: 1px solid #eef3f9;
}

.profile-list > div:last-child {
  border-bottom: 0;
}

.profile-list dt {
  color: var(--ops-text-muted);
  font-size: 12px;
  font-weight: 600;
}

.profile-list dd {
  margin: 0;
  overflow: hidden;
  color: var(--ops-text-primary);
  font-size: 12px;
  font-weight: 600;
  line-height: 1.4;
  text-align: right;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.detail-card {
  overflow: hidden;
}

.tabs-section {
  min-height: 0;
  flex: 1;
  padding: 0 12px 12px;
  overflow: auto;
}

.personal-tabs {
  min-height: 0;
  background: transparent;
}

.personal-tabs :deep(.el-tabs__header) {
  margin: 0 0 10px;
}

.personal-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: var(--ops-border-soft);
}

.personal-tabs :deep(.el-tabs__item) {
  height: 38px;
  color: var(--ops-text-secondary);
  font-size: 13px;
  font-weight: 600;
  transition: color 160ms ease;
}

.personal-tabs :deep(.el-tabs__item:hover),
.personal-tabs :deep(.el-tabs__item.is-active) {
  color: #2563eb;
}

.personal-tabs :deep(.el-tabs__active-bar) {
  height: 2px;
  background: #409eff;
}

.edit-form {
  padding: 0;
  border: 0;
  border-radius: 0;
  background: transparent;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  column-gap: 14px;
  align-content: start;
}

.form-field-full {
  grid-column: 1 / -1;
}

.edit-form :deep(.el-form-item) {
  min-width: 0;
  margin-bottom: 12px;
}

.edit-form :deep(.el-form-item__label) {
  height: auto;
  margin-bottom: 6px;
  padding: 0;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.4;
}

.edit-form :deep(.el-input__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: var(--ops-bg-surface-soft);
  box-shadow: 0 0 0 1px #d9e5f2 inset;
  transition: box-shadow 160ms ease, background-color 160ms ease;
}

.edit-form :deep(.el-input__wrapper:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.edit-form :deep(.el-input__wrapper.is-focus) {
  box-shadow: 0 0 0 1px #409eff inset;
}

.edit-form :deep(.el-input__inner) {
  min-height: 32px;
  border: 0;
  border-radius: 0;
  background: transparent;
  color: var(--ops-text-primary);
  font-size: 12px;
}

.edit-form :deep(.el-input__inner::placeholder) {
  color: #94a3b8;
}

.avatar-form-item :deep(.el-form-item__content) {
  line-height: 1;
}

.avatar-uploader {
  position: relative;
  width: 86px;
  height: 86px;
  overflow: visible;
  border: 1px dashed #b9d4ff;
  border-radius: 10px;
  background: #f6faff;
  transition: border-color 160ms ease, background-color 160ms ease;
}

.avatar-uploader :deep(.el-upload) {
  position: relative;
  display: grid;
  width: 100%;
  height: 100%;
  place-items: center;
  border-radius: inherit;
}

.avatar-uploader:hover {
  transform: none;
  border-color: #409eff;
  background: #eef6ff;
  box-shadow: none;
}

.avatar-uploader :deep(.el-avatar) {
  border-radius: 8px;
  background: #f3f7fc;
  color: #64748b;
  font-size: 28px;
}

.avatar-edit-mark {
  position: absolute;
  right: -5px;
  bottom: -5px;
  display: grid;
  width: 26px;
  height: 26px;
  place-items: center;
  border: 2px solid #ffffff;
  border-radius: 8px;
  background: #409eff;
  color: #ffffff;
  font-size: 14px;
}

.form-actions {
  margin: 0;
  padding-top: 10px;
  border-top: 1px solid var(--ops-border-soft);
}

.form-actions :deep(.el-form-item__content) {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.form-actions :deep(.el-button) {
  height: 32px;
  min-height: 32px;
  margin: 0;
  padding: 0 14px;
  border-radius: 8px;
  color: #ffffff;
  font-size: 12px;
  font-weight: 600;
  box-shadow: none;
  transition: background-color 160ms ease, border-color 160ms ease;
}

.form-actions :deep(.el-button--primary),
.form-actions :deep(.el-button--primary:hover),
.form-actions :deep(.el-button--primary:focus) {
  border-color: #409eff;
  background: #409eff;
}

.form-actions :deep(.el-button--warning),
.form-actions :deep(.el-button--warning:hover),
.form-actions :deep(.el-button--warning:focus) {
  border-color: #e6a23c;
  background: #e6a23c;
}

.form-actions :deep(.el-button:hover),
.form-actions :deep(.el-button:active) {
  transform: none;
  box-shadow: none;
}

.form-actions :deep(.el-button .el-icon) {
  color: inherit;
  font-size: 14px;
}

@media (max-width: 1000px) {
  .personal-workspace {
    grid-template-columns: 240px minmax(0, 1fr);
  }
}

@media (max-width: 768px) {
  :global(.home-container .el-main:has(.personal-management)) {
    padding-right: 6px;
    padding-left: 6px;
    overflow-y: auto;
  }

  .personal-management {
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

  .personal-workspace,
  .form-grid {
    grid-template-columns: 1fr;
  }

  .personal-card {
    overflow: visible;
  }

  .form-field-full {
    grid-column: auto;
  }

  .tabs-section {
    overflow: visible;
  }
}

@media (prefers-reduced-motion: reduce) {
  .personal-management *,
  .personal-management *::before,
  .personal-management *::after {
    transition-duration: 0.01ms !important;
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
  }
}
</style>
