<template>
  <div class="user-info">
    <el-tooltip content="大屏" placement="bottom">
      <button type="button" class="screen-entry" aria-label="打开大屏" @click.stop="goDashboard">
        <img :src="screenIcon" alt="" class="screen-entry-icon" />
      </button>
    </el-tooltip>

    <el-dropdown trigger="click" @command="handleCommand">
      <span class="dropdown-trigger">
        <img
          :src="sysAdmin.icon || require('./../assets/image/touxiang.svg')"
          alt="头像"
          class="user-avatar"
          @error="useDefaultAvatar"
        />
        <span class="user-username-trigger">
          <span class="user-username">{{ sysAdmin.username || '管理员' }}</span>
          <el-icon class="dropdown-arrow"><ArrowDown /></el-icon>
        </span>
      </span>

      <template #dropdown>
        <el-dropdown-menu>
          <el-dropdown-item command="profile">个人信息</el-dropdown-item>
          <el-dropdown-item command="project-intro">项目介绍</el-dropdown-item>
          <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
        </el-dropdown-menu>
      </template>
    </el-dropdown>
  </div>
</template>

<script>
import storage from '@/utils/storage'
import screenIcon from '@/assets/image/数据大屏.svg'
import { ArrowDown } from '@element-plus/icons-vue'

const projectIntroUrl = `${process.env.BASE_URL || '/'}autoops.html`

export default {
  name: 'HeadImage',
  components: {
    ArrowDown
  },
  data() {
    return {
      screenIcon
    }
  },
  computed: {
    sysAdmin() {
      return this.$store.state.sysAdmin || storage.getItem('sysAdmin') || {}
    }
  },
  methods: {
    useDefaultAvatar(e) {
      e.target.src = require('./../assets/image/touxiang.svg')
    },
    goDashboard() {
      if (this.$route.path !== '/dashboard') {
        this.$router.push('/dashboard')
      }
    },
    handleCommand(command) {
      if (command === 'logout') {
        this.logout()
      } else if (command === 'profile') {
        this.$router.push('/system/personal')
      } else if (command === 'project-intro') {
        window.open(projectIntroUrl, '_blank', 'noopener,noreferrer')
      }
    },
    logout() {
      storage.clearItem('token')
      storage.clearItem('sysAdmin')
      storage.clearItem('leftMenuList')
      storage.clearItem('permissionList')
      storage.clearItem('activePath')
      this.$store.commit('saveSysAdmin', '')
      this.$store.commit('saveToken', '')
      this.$store.commit('saveLeftMenuList', '')
      this.$store.commit('savePermissionList', '')
      this.$router.replace('/login')
    }
  }
}
</script>

<style scoped>
.user-info {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  white-space: nowrap;
}

.screen-entry {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  padding: 0;
  border: 1px solid rgba(148, 163, 184, 0.28);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.38);
  color: #64748b;
  cursor: pointer;
  transition: color 0.16s ease, border-color 0.16s ease, background 0.16s ease, box-shadow 0.16s ease;
}

.screen-entry:hover,
.screen-entry:focus {
  border-color: rgba(64, 158, 255, 0.48);
  background: rgba(236, 245, 255, 0.92);
  color: #409eff;
  box-shadow: 0 8px 18px rgba(64, 158, 255, 0.14);
  outline: none;
}

.screen-entry-icon {
  width: 20px;
  height: 20px;
  display: block;
}

.dropdown-trigger {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
}

.user-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  object-fit: cover;
  border: 1px solid #dcdfe6;
}

.user-username-trigger {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: #303133;
}

.user-username {
  max-width: 120px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 14px;
}

.dropdown-arrow {
  color: #606266;
}

@media (max-width: 768px) {
  .user-username {
    display: none;
  }
}
</style>
