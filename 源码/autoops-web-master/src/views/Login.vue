<template>
  <div class="login_container">
    <video
      ref="loginVideo"
      class="login_video"
      :src="earthVideo"
      autoplay
      muted
      loop
      playsinline
      preload="auto"
      aria-hidden="true"
    ></video>
    <div class="login_overlay"></div>

    <div class="grid-overlay"></div>
    <div class="login_scanline"></div>

    <div class="login_box">
      <el-form class="login_form" ref="loginFormRef" :rules="rules" :model="loginForm" @submit.prevent="loginBtn">
        <div class="title-container">
          <div class="title-with-logo">
            <div class="logo-mark" aria-hidden="true">
              <span class="logo-mark__cube"></span>
              <span class="logo-mark__core"></span>
            </div>
            <div>
              <div class="title">AutoOps天枢运维平台</div>
              <div class="subtitle">编排混沌 . 智驭运维</div>
            </div>
          </div>
        </div>
        
        <el-form-item prop="username" class="form-item">
          <label class="field-label">账号</label>
          <div class="input-wrapper">
            <div class="input-icon">
              <el-icon><User /></el-icon>
            </div>
            <el-input placeholder="请输入账号" v-model="loginForm.username" class="tech-input" clearable />
          </div>
        </el-form-item>
        
        <el-form-item prop="password" class="form-item">
          <label class="field-label">密码</label>
          <div class="input-wrapper">
            <div class="input-icon">
              <el-icon><Lock /></el-icon>
            </div>
            <el-input placeholder="请输入密码" v-model="loginForm.password" type="password" show-password class="tech-input" clearable />
          </div>
        </el-form-item>
        
        <el-form-item class="button-group">
          <div class="tech-buttons">
            <button class="tech-btn primary-btn" type="submit">
              <span class="btn-text">登录系统</span>
              <el-icon class="btn-icon"><Right /></el-icon>
            </button>
            <button class="tech-btn secondary-btn" @click="resetLoginForm" type="button">
              <span class="btn-text">重置表单</span>
              <el-icon class="btn-icon"><RefreshRight /></el-icon>
            </button>
          </div>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script>
import earthVideo from '../assets/image/旋转地球.mp4'
import { Lock, RefreshRight, Right, User } from '@element-plus/icons-vue'

export default {
  name: "LoginPage",
  components: {
    Lock,
    RefreshRight,
    Right,
    User
  },
  data() {
    return {
      earthVideo,
      rules: {
        username: [
          { required: true, message: "请输入账号", trigger: "blur" }
        ],
        password: [
          { required: true, message: "请输入密码", trigger: "blur" }
        ]
      },
      loginForm: {
        username: '',
        password: ''
      }
    }
  },
  mounted() {
    this.$nextTick(() => {
      const video = this.$refs.loginVideo
      if (!video) {
        return
      }

      const tryPlay = () => {
        const playPromise = video.play && video.play()
        if (playPromise && typeof playPromise.catch === 'function') {
          playPromise.catch(() => {})
        }
      }

      if (video.readyState >= 2) {
        tryPlay()
      } else {
        video.addEventListener('canplay', tryPlay, { once: true })
      }
    })
  },
  methods: {

    // 登录按钮
    loginBtn() {
      this.$refs.loginFormRef.validate(async valid => {
        if (valid) {
          const { data: res } = await this.$api.login(this.loginForm)

          console.log("【Login.vue】登录接口返回完整数据：", res)

          if (res.code !== 200) {
            this.$message.error(res.message)
          } else {
            this.$message.success("登录成功")
            console.log("【Login.vue】res.data 内容：", res.data)

            this.$store.commit('saveSysAdmin', res.data.sysAdmin)
            this.$store.commit('saveToken', res.data.token)
            this.$store.commit('saveLeftMenuList', res.data.leftMenuList)
            this.$store.commit('savePermissionList', res.data.permissionList)
			this.$storage.setItem('license', res.data.license || {})

			await this.$router.push(res.data.license?.licensed ? "/home" : "/system/license")
          }
        } else {
          return false
        }
      })
    },

    // 重置按钮
    resetLoginForm() {
      this.$refs.loginFormRef.resetFields()
    }
  }
}
</script>

<style lang="less" scoped>
@accent: #16c8f3;
@accent-strong: #29d9ff;
@panel: rgba(5, 12, 24, 0.76);
@line: rgba(144, 221, 255, 0.2);

.login_container {
  position: fixed;
  inset: 0;
  z-index: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100dvh;
  width: 100%;
  padding: 24px;
  overflow: hidden;
  box-sizing: border-box;
}

.login_video {
  position: absolute;
  inset: 0;
  z-index: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center;
  pointer-events: none;
  filter: brightness(0.78) contrast(1.15) saturate(1.02);
  transform: translateZ(0) scale(1.01);
}

.login_overlay {
  position: absolute;
  inset: 0;
  z-index: 1;
  background:
    linear-gradient(90deg, rgba(2, 7, 18, 0.18) 0%, rgba(2, 7, 18, 0.3) 48%, rgba(2, 7, 18, 0.72) 100%),
    linear-gradient(180deg, rgba(2, 7, 18, 0.12) 0%, rgba(2, 7, 18, 0.58) 100%);
}

.grid-overlay {
  position: absolute;
  inset: 0;
  z-index: 2;
  pointer-events: none;
  opacity: 0.55;
  background-image:
    linear-gradient(rgba(22, 200, 243, 0.045) 1px, transparent 1px),
    linear-gradient(90deg, rgba(22, 200, 243, 0.045) 1px, transparent 1px);
  background-size: 72px 72px;
  mask-image: linear-gradient(90deg, transparent 0%, #000 30%, #000 100%);
  animation: gridDrift 18s linear infinite;
}

.login_scanline {
  position: absolute;
  inset: 0;
  z-index: 3;
  pointer-events: none;
  opacity: 0.24;
  background: linear-gradient(180deg, transparent, rgba(41, 217, 255, 0.13), transparent);
  transform: translateY(-100%);
  animation: scanline 5.8s cubic-bezier(0.16, 1, 0.3, 1) infinite;
}

.login_box {
  position: relative;
  z-index: 10;
  width: min(420px, 100%);
  border: 1px solid @line;
  border-radius: 16px;
  overflow: hidden;
  background:
    linear-gradient(145deg, rgba(255, 255, 255, 0.08), rgba(255, 255, 255, 0.02)),
    @panel;
  backdrop-filter: blur(18px) saturate(135%);
  -webkit-backdrop-filter: blur(18px) saturate(135%);
  box-shadow:
    0 28px 80px rgba(0, 0, 0, 0.48),
    0 0 0 1px rgba(255, 255, 255, 0.04) inset,
    0 1px 0 rgba(255, 255, 255, 0.14) inset;
  animation: panelEnter 0.72s cubic-bezier(0.16, 1, 0.3, 1) both;
}

.login_box::before {
  content: "";
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    linear-gradient(90deg, @accent 0, transparent 26%) top left / 100% 1px no-repeat,
    radial-gradient(circle at 82% 8%, rgba(41, 217, 255, 0.2), transparent 32%);
}

.login_form {
  position: relative;
  display: grid;
  gap: 16px;
  width: 100%;
  padding: 30px 32px 28px;
  box-sizing: border-box;

  .form-item {
    display: block;
    margin-bottom: 0;

    &:deep(.el-form-item__content) {
      display: block;
      line-height: normal;
    }

    &:deep(.el-form-item__error) {
      padding-top: 6px;
      color: #ff8a8a;
      font-size: 12px;
      line-height: 1.2;
    }
  }
}

.title-container {
  padding-bottom: 4px;
}

.title-with-logo {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 11px;
  width: 100%;
  text-align: left;
}

.logo-mark {
  display: grid;
  place-items: center;
  position: relative;
  flex: 0 0 38px;
  width: 38px;
  height: 38px;
  border: 1px solid rgba(41, 217, 255, 0.48);
  border-radius: 11px;
  background:
    radial-gradient(circle at 50% 38%, rgba(41, 217, 255, 0.28), transparent 48%),
    linear-gradient(145deg, rgba(41, 217, 255, 0.2), rgba(41, 217, 255, 0.04)),
    rgba(5, 21, 34, 0.72);
  box-shadow:
    0 12px 26px rgba(22, 200, 243, 0.22),
    0 0 20px rgba(22, 200, 243, 0.2),
    inset 0 1px 0 rgba(255, 255, 255, 0.14);

  &::before,
  &::after {
    content: "";
    position: absolute;
    pointer-events: none;
  }

  &::before {
    inset: 6px;
    border: 1px solid rgba(166, 239, 255, 0.24);
    border-radius: 8px;
  }

  &::after {
    width: 16px;
    height: 1px;
    background: linear-gradient(90deg, transparent, rgba(151, 235, 255, 0.86), transparent);
    transform: translateY(10px);
  }
}

.logo-mark__cube {
  position: relative;
  width: 18px;
  height: 16px;
  transform: translateY(-1px);
  border: 2px solid @accent-strong;
  border-top-color: rgba(154, 239, 255, 0.95);
  border-radius: 4px;
  clip-path: polygon(50% 0, 100% 27%, 100% 78%, 50% 100%, 0 78%, 0 27%);
  box-shadow:
    0 0 12px rgba(41, 217, 255, 0.5),
    inset 0 0 10px rgba(22, 200, 243, 0.12);
}

.logo-mark__cube::before,
.logo-mark__cube::after {
  content: "";
  position: absolute;
  left: 50%;
  top: 4px;
  width: 2px;
  height: 8px;
  background: rgba(41, 217, 255, 0.9);
  transform-origin: top center;
}

.logo-mark__cube::before {
  transform: rotate(58deg);
}

.logo-mark__cube::after {
  transform: rotate(-58deg);
}

.logo-mark__core {
  position: absolute;
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: #c7f8ff;
  box-shadow: 0 0 12px rgba(41, 217, 255, 0.9);
}

.title {
  color: @accent-strong;
  font-size: 22px;
  font-weight: 800;
  line-height: 1.18;
  letter-spacing: 0;
  text-shadow: 0 0 18px rgba(22, 200, 243, 0.22);
}

.subtitle {
  margin-top: 7px;
  color: rgba(215, 241, 255, 0.82);
  font-family: "Microsoft YaHei UI", "PingFang SC", "Microsoft YaHei", sans-serif;
  font-size: 13px;
  font-weight: 600;
  line-height: 1.4;
  letter-spacing: 0;
  text-align: center;
  text-shadow: 0 0 12px rgba(22, 200, 243, 0.18);
}

.field-label {
  display: block;
  margin-bottom: 8px;
  color: rgba(226, 245, 255, 0.86);
  font-size: 13px;
  font-weight: 600;
  line-height: 1.2;
}

.input-wrapper {
  position: relative;
  display: flex;
  align-items: center;
  width: 100%;
  min-height: 44px;
  padding: 0 12px;
  border: 1px solid rgba(183, 231, 255, 0.22);
  border-radius: 10px;
  background: rgba(3, 11, 22, 0.58);
  box-sizing: border-box;
  transition: border-color 0.22s ease, background-color 0.22s ease, box-shadow 0.22s ease, transform 0.22s ease;

  &:hover {
    border-color: rgba(41, 217, 255, 0.46);
    background: rgba(5, 17, 32, 0.68);
  }

  &:focus-within {
    border-color: rgba(41, 217, 255, 0.82);
    background: rgba(4, 18, 34, 0.78);
    box-shadow: 0 0 0 3px rgba(22, 200, 243, 0.16);
  }
}

.input-icon {
  display: grid;
  place-items: center;
  flex: 0 0 18px;
  margin-right: 10px;
  color: rgba(155, 221, 247, 0.84);

  .el-icon {
    font-size: 17px;
  }
}

.tech-input {
  flex: 1;

  &:deep(.el-input__wrapper) {
    min-height: 42px;
    padding: 0;
    border: 0;
    background: transparent !important;
    box-shadow: none !important;
  }

  &:deep(.el-input__inner) {
    color: #f5fbff !important;
    font-size: 14px;
    font-weight: 500;
    background: transparent !important;

    &::placeholder {
      color: rgba(183, 219, 238, 0.5) !important;
    }
  }

  &:deep(.el-input__suffix-inner) {
    color: rgba(183, 219, 238, 0.65);
  }
}

.button-group {
  margin-top: 4px;
  margin-bottom: 0;
}

.tech-buttons {
  display: grid;
  grid-template-columns: 1.25fr 1fr;
  gap: 12px;
  width: 100%;
}

.tech-btn {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  width: 100%;
  height: 44px;
  border-radius: 10px;
  border: 1px solid transparent;
  cursor: pointer;
  font-size: 14px;
  font-weight: 700;
  line-height: 1;
  white-space: nowrap;
  transition: transform 0.2s ease, border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;

  .btn-icon {
    font-size: 15px;
    transition: transform 0.2s ease;
  }

  &:hover {
    transform: translateY(-1px);
  }

  &:active {
    transform: translateY(1px) scale(0.99);
  }

  &:focus-visible {
    outline: 3px solid rgba(41, 217, 255, 0.28);
    outline-offset: 2px;
  }

  &.primary-btn {
    color: #02111d;
    background: linear-gradient(135deg, #65e9ff 0%, @accent 48%, #0aa1e6 100%);
    box-shadow: 0 12px 28px rgba(9, 164, 229, 0.28);

    &:hover {
      box-shadow: 0 16px 34px rgba(9, 164, 229, 0.36);

      .btn-icon {
        transform: translateX(2px);
      }
    }
  }

  &.secondary-btn {
    color: rgba(238, 249, 255, 0.92);
    border-color: rgba(183, 231, 255, 0.22);
    background: rgba(255, 255, 255, 0.07);

    &:hover {
      border-color: rgba(41, 217, 255, 0.42);
      background: rgba(255, 255, 255, 0.11);
    }
  }
}

@keyframes panelEnter {
  from {
    opacity: 0;
    transform: translateY(18px) scale(0.98);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

@keyframes gridDrift {
  from {
    background-position: 0 0, 0 0;
  }
  to {
    background-position: 72px 72px, 72px 72px;
  }
}

@keyframes scanline {
  0%, 68% {
    opacity: 0;
    transform: translateY(-100%);
  }
  72% {
    opacity: 0.2;
  }
  100% {
    opacity: 0;
    transform: translateY(100%);
  }
}

@media (max-width: 768px) {
  .login_container {
    align-items: flex-end;
    padding: 18px;
  }

  .login_overlay {
    background:
      linear-gradient(180deg, rgba(2, 7, 18, 0.18) 0%, rgba(2, 7, 18, 0.78) 100%),
      rgba(2, 7, 18, 0.2);
  }

  .grid-overlay {
    opacity: 0.32;
    background-size: 56px 56px;
  }

  .login_box {
    width: 100%;
    max-width: 420px;
    border-radius: 14px;
  }

  .login_form {
    padding: 24px 20px 22px;
    gap: 14px;
  }

  .title {
    font-size: 20px;
  }

  .title-with-logo {
    gap: 10px;
  }

  .logo-mark {
    flex-basis: 36px;
    width: 36px;
    height: 36px;
  }

  .tech-buttons {
    grid-template-columns: 1fr;
  }
}

@media (prefers-reduced-motion: reduce) {
  .grid-overlay,
  .login_scanline,
  .login_box {
    animation: none;
  }

  .tech-btn,
  .tech-btn .btn-icon,
  .input-wrapper {
    transition: none;
  }
}

@supports not ((backdrop-filter: blur(1px)) or (-webkit-backdrop-filter: blur(1px))) {
  .login_box {
    background: rgba(5, 12, 24, 0.94);
  }
}
</style>
