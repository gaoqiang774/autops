<template>
  <div class="license-page">
    <header class="license-page-header">
      <span class="license-page-header__icon" aria-hidden="true">
        <el-icon><Key /></el-icon>
      </span>
      <div class="license-page-header__content">
        <h1>系统授权</h1>
        <span class="license-page-header__divider" aria-hidden="true"></span>
        <p>查看授权状态、设备信息并完成许可证激活。</p>
      </div>
      <div class="license-page-header__actions">
        <el-tag :type="statusTagType" effect="light" size="small" class="license-status-tag">
          {{ statusLabel }}
        </el-tag>
        <el-tooltip content="刷新授权状态" placement="top">
          <el-button
            type="warning"
            size="small"
            class="license-refresh-button"
            :loading="loading"
            aria-label="刷新授权状态"
            @click="loadStatus"
          >
            <el-icon><Refresh /></el-icon>
          </el-button>
        </el-tooltip>
      </div>
    </header>

    <main v-loading="loading" class="license-workbench">
      <div v-if="loadError" class="license-error" role="alert">
        <el-icon><WarningFilled /></el-icon>
        <span>{{ loadError }}</span>
        <el-button type="primary" link size="small" @click="loadStatus">重新加载</el-button>
      </div>

      <section class="license-status" :class="`is-${status.state || 'unlicensed'}`">
        <div class="license-status__main">
          <span class="license-status__mark" aria-hidden="true">
            <el-icon v-if="status.licensed"><CircleCheckFilled /></el-icon>
            <el-icon v-else><WarningFilled /></el-icon>
          </span>
          <div class="license-status__copy">
            <span>当前状态</span>
            <strong>{{ statusLabel }}</strong>
            <small>{{ status.message || '系统尚未授权' }}</small>
          </div>
        </div>

        <div class="license-status__metrics">
          <div class="license-status__metric">
            <span>授权期限</span>
            <strong>{{ periodText }}</strong>
            <small v-if="status.licensed && !status.unlimited">剩余 {{ status.remainingDays || 0 }} 天</small>
            <small v-else>{{ status.unlimited ? '长期有效' : '等待激活' }}</small>
          </div>
          <div class="license-status__metric">
            <span>授权范围</span>
            <strong>{{ moduleCountText }}</strong>
            <small :title="moduleScopeText">{{ moduleScopeText }}</small>
          </div>
        </div>
      </section>

      <div class="license-main-grid" :class="{ 'has-activation': isAdministrator }">
        <section class="license-card license-detail" aria-labelledby="license-detail-title">
          <header class="license-card__header">
            <div class="license-card__title">
              <el-icon><Monitor /></el-icon>
              <h2 id="license-detail-title">设备与授权信息</h2>
            </div>
          </header>

          <dl class="license-detail__grid">
            <div class="license-detail__item machine-code-item">
              <dt>机器码</dt>
              <dd>
                <code>{{ status.machineCode || '-' }}</code>
                <el-tooltip content="复制机器码" placement="top">
                  <el-button
                    class="license-copy-button"
                    :disabled="!status.machineCode"
                    aria-label="复制机器码"
                    @click="copyMachineCode"
                  >
                    <el-icon><CopyDocument /></el-icon>
                  </el-button>
                </el-tooltip>
              </dd>
            </div>
            <div class="license-detail__item">
              <dt>绑定方式</dt>
              <dd>{{ bindingTypeText }}</dd>
            </div>
            <div class="license-detail__item">
              <dt>授权编号</dt>
              <dd :title="status.licenseId || '-'">{{ status.licenseId || '-' }}</dd>
            </div>
            <div class="license-detail__item">
              <dt>授权客户</dt>
              <dd :title="status.customer || '-'">{{ status.customer || '-' }}</dd>
            </div>
            <div class="license-detail__item">
              <dt>签发时间</dt>
              <dd>{{ formatTime(status.issuedAt) }}</dd>
            </div>
            <div class="license-detail__item">
              <dt>到期时间</dt>
              <dd>{{ status.unlimited ? '永久授权' : formatTime(status.expiresAt) }}</dd>
            </div>
            <div class="license-detail__item project-intro-item">
              <dt>项目介绍</dt>
              <dd>
                <a :href="projectIntroUrl" target="_blank" rel="noopener noreferrer" class="license-project-link">
                  <el-icon><Document /></el-icon>
                  <span>查看项目介绍</span>
                </a>
              </dd>
            </div>
            <div class="license-detail__item module-scope-item">
              <dt>可用模块</dt>
              <dd :title="moduleScopeText">{{ moduleScopeText }}</dd>
            </div>
          </dl>
        </section>

        <section v-if="isAdministrator" class="license-card license-activation" aria-labelledby="activation-title">
          <header class="license-card__header">
            <div class="license-card__title">
              <el-icon><Unlock /></el-icon>
              <h2 id="activation-title">激活授权</h2>
            </div>
          </header>

          <el-form class="license-activation__form" label-position="top" @submit.prevent>
            <el-form-item label="授权码">
              <el-input
                v-model="licenseKey"
                type="textarea"
                :rows="4"
                resize="vertical"
                maxlength="4096"
                show-word-limit
                placeholder="粘贴完整授权码"
              />
            </el-form-item>
            <p class="license-activation__hint">激活成功后会刷新当前账号的菜单和权限。</p>
            <el-button
              class="license-activation__button"
              :disabled="!licenseKey.trim()"
              type="primary"
              :loading="activating"
              @click="activate"
            >
              <el-icon><Key /></el-icon>
              激活授权
            </el-button>
          </el-form>
        </section>
      </div>

      <section class="license-card license-contact" aria-labelledby="license-contact-title">
        <header class="license-card__header">
          <div class="license-card__title">
            <el-icon><ChatDotRound /></el-icon>
            <h2 id="license-contact-title">联系作者</h2>
          </div>
        </header>

        <div class="license-contact__body">
          <div class="license-contact__links">
            <div class="license-contact__item">
              <span class="license-contact__item-icon" aria-hidden="true">
                <el-icon><ChatDotRound /></el-icon>
              </span>
              <div>
                <strong>交流与反馈</strong>
                <p>扫码添加微信，进群交流、提需求或反馈意见。</p>
              </div>
            </div>

            <div class="license-contact__item">
              <span class="license-contact__item-icon" aria-hidden="true">
                <el-icon><Link /></el-icon>
              </span>
              <div>
                <strong>开源项目</strong>
                <a href="https://github.com/zhang1024fan/deviops" target="_blank" rel="noopener noreferrer">
                  zhang1024fan/deviops
                </a>
              </div>
            </div>

            <div class="license-contact__item">
              <span class="license-contact__item-icon" aria-hidden="true">
                <el-icon><Key /></el-icon>
              </span>
              <div>
                <strong>激活项目</strong>
                <a href="mailto:zfwh1024@163.com">zfwh1024@163.com</a>
              </div>
            </div>

            <div class="license-contact__item">
              <span class="license-contact__item-icon" aria-hidden="true">
                <el-icon><Monitor /></el-icon>
              </span>
              <div>
                <strong>在线体验</strong>
                <div class="license-contact__experience">
                  <a href="http://autoops.com.cn/" target="_blank" rel="noopener noreferrer">autoops.com.cn</a>
                  <span><code>test</code> / <code>123456</code></span>
                </div>
              </div>
            </div>
          </div>

          <div class="license-contact__qr-grid">
            <figure class="license-contact__qr-item">
              <a :href="weixinImage" target="_blank" aria-label="查看微信二维码大图">
                <img :src="weixinImage" width="132" height="132" loading="lazy" alt="作者微信二维码">
              </a>
              <figcaption>
                <strong>微信二维码</strong>
                <span>添加微信并加入交流群</span>
              </figcaption>
            </figure>
            <figure class="license-contact__qr-item">
              <a :href="weixinPayImage" target="_blank" aria-label="查看微信打赏二维码大图">
                <img :src="weixinPayImage" width="132" height="132" loading="lazy" alt="微信打赏二维码">
              </a>
              <figcaption>
                <strong>微信打赏</strong>
                <span>感谢支持项目持续维护</span>
              </figcaption>
            </figure>
          </div>
        </div>
      </section>
    </main>
  </div>
</template>

<script>
import dayjs from 'dayjs'
import storage from '@/utils/storage'
import weixinPayImage from '@/assets/image/weixin-pay.png'
import weixinImage from '@/assets/image/weixin.png'
import {
  ChatDotRound,
  CircleCheckFilled,
  CopyDocument,
  Document,
  Key,
  Link,
  Monitor,
  Refresh,
  Unlock,
  WarningFilled
} from '@element-plus/icons-vue'

const projectIntroUrl = `${process.env.BASE_URL || '/'}autoops.html`

export default {
  name: 'SystemLicense',
  components: {
    ChatDotRound,
    CircleCheckFilled,
    CopyDocument,
    Document,
    Key,
    Link,
    Monitor,
    Refresh,
    Unlock,
    WarningFilled
  },
  data() {
    return {
      loading: false,
      activating: false,
      loadError: '',
      licenseKey: '',
      projectIntroUrl,
      weixinImage,
      weixinPayImage,
      status: {
        licensed: false,
        state: 'unlicensed',
        allowedModules: []
      }
    }
  },
  computed: {
    isAdministrator() {
      const admin = storage.getItem('sysAdmin') || {}
      const username = String(admin.username || '').trim().toLowerCase()
      return username === 'admin' || username === 'root'
    },
    statusLabel() {
      const labels = {
        valid: '已授权',
        unlicensed: '未授权',
        expired: '已过期',
        machine_mismatch: '机器不匹配',
        not_yet_valid: '尚未生效',
        public_key_unconfigured: '授权未配置',
        installation_identity_error: '实例初始化失败',
        invalid: '授权无效'
      }
      return labels[this.status.state] || '未授权'
    },
    statusTagType() {
      if (this.status.licensed) return 'success'
      if (['expired', 'invalid', 'machine_mismatch'].includes(this.status.state)) return 'danger'
      return 'warning'
    },
    periodText() {
      if (!this.status.licensed) return '-'
      if (this.status.unlimited) return '永久'
      return this.formatTime(this.status.expiresAt, 'YYYY-MM-DD')
    },
    moduleScopeText() {
      const modules = Array.isArray(this.status.allowedModules) ? this.status.allowedModules : []
      return modules.length ? modules.join('、') : '-'
    },
    moduleCountText() {
      const modules = Array.isArray(this.status.allowedModules) ? this.status.allowedModules : []
      return modules.length ? `${modules.length} 个模块` : '-'
    },
    bindingTypeText() {
      return this.status.bindingType === 'legacy-hardware-v1' ? '旧版硬件绑定' : '安装实例绑定'
    }
  },
  mounted() {
    this.loadStatus()
  },
  methods: {
    async loadStatus() {
      this.loading = true
      this.loadError = ''
      try {
        const { data: res } = await this.$api.getLicenseStatus()
        if (res.code === 200) {
          this.status = res.data || this.status
        } else {
          this.loadError = res.message || '获取授权状态失败'
        }
      } catch (error) {
        console.error('获取授权状态失败:', error)
        this.loadError = '获取授权状态失败，请检查服务连接后重试'
      } finally {
        this.loading = false
      }
    },
    async activate() {
      const key = this.licenseKey.trim()
      if (!key) return
      this.activating = true
      try {
        const { data: res } = await this.$api.activateLicense(key)
        if (res.code !== 200) {
          this.$message.error(res.message || '激活授权失败')
          return
        }
        this.status = res.data
        this.licenseKey = ''
        await this.refreshSessionAccess()
        this.$message.success('授权激活成功')
        setTimeout(() => window.location.reload(), 500)
      } catch (error) {
        console.error('激活授权失败:', error)
      } finally {
        this.activating = false
      }
    },
    async refreshSessionAccess() {
      const { data: res } = await this.$api.getLicenseSessionAccess()
      if (res.code !== 200) return
      storage.setItem('leftMenuList', res.data.leftMenuList || [])
      storage.setItem('permissionList', res.data.permissionList || [])
      storage.setItem('license', res.data.license || {})
      storage.clearItem('navModule')
    },
    async copyMachineCode() {
      if (!this.status.machineCode) return
      try {
        await this.writeClipboardText(this.status.machineCode)
        this.$message.success('机器码已复制')
      } catch (error) {
        console.error('复制机器码失败:', error)
        this.$message.error('复制机器码失败')
      }
    },
    async writeClipboardText(text) {
      if (window.isSecureContext && navigator.clipboard?.writeText) {
        try {
          await navigator.clipboard.writeText(text)
          return
        } catch (error) {
          console.warn('Clipboard API 不可用，尝试兼容复制:', error)
        }
      }

      const textarea = document.createElement('textarea')
      textarea.value = text
      textarea.setAttribute('readonly', '')
      textarea.style.position = 'fixed'
      textarea.style.top = '0'
      textarea.style.left = '-9999px'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)

      try {
        textarea.focus()
        textarea.select()
        textarea.setSelectionRange(0, textarea.value.length)
        if (!document.execCommand('copy')) throw new Error('浏览器拒绝复制操作')
      } finally {
        document.body.removeChild(textarea)
      }
    },
    formatTime(value, format = 'YYYY-MM-DD HH:mm:ss') {
      if (!value) return '-'
      const parsed = dayjs(value)
      return parsed.isValid() ? parsed.format(format) : '-'
    }
  }
}
</script>

<style lang="less" scoped>
.license-page {
  box-sizing: border-box;
  height: calc(100vh - 124px);
  min-height: 0;
  padding: var(--ops-page-padding, 7px);
  overflow: hidden;
  color: #172033;
  background: transparent;
}

:global(.home-container .el-main:has(.license-page)) {
  padding-top: 10px;
  padding-right: 10px;
  padding-left: 10px;
}

.license-page-header {
  display: flex;
  align-items: center;
  min-height: 52px;
  padding: 5px 10px;
  border: 1px solid #d7e3f1;
  border-radius: 12px;
  background: #fff;
  box-shadow: 0 4px 16px rgba(29, 72, 122, 0.05);
}

.license-page-header__content,
.license-page-header__actions,
.license-status__main,
.license-status__copy,
.license-card__title,
.machine-code-item dd {
  display: flex;
  align-items: center;
}

.license-page-header__content {
  flex: 1;
  gap: 10px;
  min-width: 0;
}

.license-page-header__icon {
  display: grid;
  flex: 0 0 36px;
  width: 36px;
  height: 36px;
  place-items: center;
  color: #2f7df4;
  font-size: 25px;
}

.license-page-header__content h1 {
  flex: 0 0 auto;
  margin: 0;
  font-size: 20px;
  line-height: 26px;
  letter-spacing: 0;
}

.license-page-header__divider {
  flex: 0 0 1px;
  width: 1px;
  height: 22px;
  background: #dce5f0;
}

.license-page-header__content p {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  color: #7a8699;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.license-page-header__actions {
  flex: 0 0 auto;
  gap: 8px;
  margin-left: 12px;
}

.license-page-header__actions :deep(.el-tag) {
  height: 24px;
  padding: 0 9px;
  border-radius: 12px;
}

.license-refresh-button {
  width: 30px;
  height: 30px;
  padding: 0;
  border-radius: 8px;
  transition: transform 0.16s ease, box-shadow 0.16s ease;
}

.license-refresh-button:hover {
  transform: translateY(-1px);
  box-shadow: 0 5px 12px rgba(45, 103, 170, 0.14);
}

.license-refresh-button:active {
  transform: translateY(0);
}

.license-refresh-button {
  color: #fff;
  border-color: #f0a800;
  background: #f0a800;
}

.license-refresh-button:hover,
.license-refresh-button:focus {
  color: #fff;
  border-color: #dc9800;
  background: #dc9800;
}

.license-workbench {
  display: grid;
  min-height: 330px;
  margin-top: 8px;
  gap: 8px;
}

.license-error {
  display: grid;
  grid-template-columns: 18px minmax(0, 1fr) auto;
  align-items: center;
  min-height: 38px;
  padding: 6px 10px;
  border: 1px solid #f6d68d;
  border-radius: 8px;
  color: #8b5b00;
  background: #fff9eb;
  font-size: 12px;
}

.license-error .el-icon {
  color: #e6a23c;
  font-size: 16px;
}

.license-error span {
  padding: 0 7px;
}

.license-status {
  display: grid;
  grid-template-columns: minmax(260px, 1fr) auto;
  align-items: center;
  min-height: 70px;
  overflow: hidden;
  border: 1px solid #f0c66a;
  border-left: 3px solid #e6a23c;
  border-radius: 12px;
  background: #fffaf0;
  box-shadow: 0 4px 16px rgba(85, 66, 22, 0.04);
}

.license-status.is-valid {
  border-color: #9ad8bb;
  border-left-color: #18a66a;
  background: #f2fbf7;
}

.license-status.is-expired,
.license-status.is-invalid,
.license-status.is-machine_mismatch {
  border-color: #f1b0b7;
  border-left-color: #e54d5d;
  background: #fff7f8;
}

.license-status__main {
  gap: 10px;
  min-width: 0;
  padding: 7px 10px;
}

.license-status__mark {
  display: grid;
  flex: 0 0 34px;
  width: 34px;
  height: 34px;
  place-items: center;
  border-radius: 10px;
  color: #d38a19;
  background: #fff;
  box-shadow: 0 3px 12px rgba(90, 70, 20, 0.09);
  font-size: 20px;
}

.is-valid .license-status__mark {
  color: #18a66a;
}

.is-expired .license-status__mark,
.is-invalid .license-status__mark,
.is-machine_mismatch .license-status__mark {
  color: #e54d5d;
}

.license-status__copy {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  min-width: 0;
  gap: 1px;
}

.license-status__copy span,
.license-status__metric span,
.license-detail__item dt {
  color: #8792a5;
  font-size: 11px;
}

.license-status__copy strong {
  max-width: 100%;
  overflow: hidden;
  font-size: 18px;
  line-height: 25px;
  letter-spacing: 0;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.license-status__copy small {
  max-width: 100%;
  overflow: hidden;
  color: #667287;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.license-status__metrics {
  display: grid;
  grid-template-columns: repeat(2, minmax(150px, 190px));
  align-self: stretch;
  border-left: 1px solid rgba(106, 120, 141, 0.18);
}

.license-status__metric {
  display: flex;
  justify-content: center;
  flex-direction: column;
  min-width: 0;
  padding: 6px 12px;
}

.license-status__metric + .license-status__metric {
  border-left: 1px solid rgba(106, 120, 141, 0.18);
}

.license-status__metric strong {
  overflow: hidden;
  color: #263248;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 17px;
  line-height: 24px;
  letter-spacing: 0;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.license-status__metric small {
  overflow: hidden;
  color: #667287;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.license-main-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 8px;
}

.license-main-grid.has-activation {
  grid-template-columns: minmax(0, 1fr) minmax(300px, 360px);
  align-items: start;
}

.license-card {
  min-width: 0;
  overflow: hidden;
  border: 1px solid #dce4ef;
  border-radius: 12px;
  background: #fff;
  box-shadow: 0 4px 16px rgba(29, 72, 122, 0.04);
}

.license-card__header {
  display: flex;
  align-items: center;
  min-height: 36px;
  padding: 6px 10px;
  border-bottom: 1px solid #e7edf5;
}

.license-card__title {
  gap: 7px;
  color: #2f7df4;
}

.license-card__title .el-icon {
  font-size: 16px;
}

.license-card__title h2 {
  margin: 0;
  color: #263248;
  font-size: 15px;
  line-height: 22px;
  letter-spacing: 0;
}

.license-detail__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1px;
  margin: 0;
  background: #e9eef5;
}

.license-detail__item {
  min-width: 0;
  padding: 6px 10px;
  background: #fff;
}

.license-detail__item dt {
  margin-bottom: 2px;
  font-weight: 600;
}

.license-detail__item dd {
  min-height: 18px;
  margin: 0;
  overflow: hidden;
  color: #263248;
  font-size: 12px;
  line-height: 18px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.machine-code-item,
.module-scope-item {
  grid-column: 1 / -1;
}

.machine-code-item dd {
  justify-content: flex-start;
  gap: 8px;
  white-space: normal;
}

.machine-code-item code {
  display: inline-block;
  min-width: 0;
  padding: 2px 4px;
  border-radius: 4px;
  color: #1d5fc4;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  overflow-wrap: anywhere;
  transition: background-color 0.16s ease;
}

.machine-code-item dd:hover code,
.machine-code-item dd:focus-within code {
  background: #eef6ff;
}

.license-copy-button {
  flex: 0 0 28px;
  width: 28px;
  height: 28px;
  padding: 0;
  color: #2f7df4;
  border-radius: 7px;
  opacity: 0;
  visibility: hidden;
  transform: translateX(-4px);
  transition: opacity 0.16s ease, transform 0.16s ease, visibility 0.16s ease;
}

.machine-code-item dd:hover .license-copy-button,
.machine-code-item dd:focus-within .license-copy-button {
  opacity: 1;
  visibility: visible;
  transform: translateX(0);
}

.module-scope-item dd {
  white-space: normal;
}

.project-intro-item dd {
  overflow: visible;
}

.license-project-link {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: #1769d2;
  font-size: 12px;
  line-height: 20px;
  text-decoration: none;
}

.license-project-link:hover {
  text-decoration: underline;
}

.license-project-link:focus-visible {
  border-radius: 3px;
  outline: 2px solid #2f7df4;
  outline-offset: 2px;
}

.license-activation__form {
  padding: 7px 10px 9px;
}

.license-activation__form :deep(.el-form-item) {
  margin-bottom: 5px;
}

.license-activation__form :deep(.el-form-item__label) {
  padding-bottom: 4px;
  color: #4d5b70;
  font-size: 12px;
  font-weight: 600;
  line-height: 18px;
}

.license-activation__form :deep(.el-textarea__inner) {
  min-height: 92px !important;
  padding: 7px 9px;
  border: 1px solid #ccd9e8;
  border-radius: 8px;
  background: #fbfcfe;
  box-shadow: none;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  line-height: 20px;
  resize: vertical;
}

.license-activation__form :deep(.el-textarea__inner:hover) {
  border-color: #9db9db;
}

.license-activation__form :deep(.el-textarea__inner:focus) {
  border-color: #409eff;
  box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.12);
}

.license-activation__form :deep(.el-input__count) {
  right: 8px;
  bottom: 4px;
  color: #8b97a8;
  background: transparent;
  font-size: 10px;
}

.license-activation__hint {
  margin: 0 0 6px;
  color: #7b8798;
  font-size: 11px;
  line-height: 17px;
}

.license-activation__button {
  width: 100%;
  height: 32px;
  border-radius: 8px;
}

.license-contact__body {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 300px;
  min-height: 138px;
}

.license-contact__links {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1px;
  min-width: 0;
  background: #e9eef5;
}

.license-contact__item {
  display: flex;
  align-items: center;
  min-width: 0;
  min-height: 68px;
  padding: 6px 10px;
  gap: 10px;
  background: #fff;
}

.license-contact__item-icon {
  display: grid;
  flex: 0 0 30px;
  width: 30px;
  height: 30px;
  place-items: center;
  border-radius: 8px;
  color: #2f7df4;
  background: #eef6ff;
  font-size: 15px;
}

.license-contact__item > div {
  min-width: 0;
}

.license-contact__item strong,
.license-contact__item p,
.license-contact__item a {
  letter-spacing: 0;
}

.license-contact__item strong {
  display: block;
  margin-bottom: 3px;
  color: #4d5b70;
  font-size: 12px;
  line-height: 18px;
}

.license-contact__item p {
  margin: 0;
  color: #667287;
  font-size: 12px;
  line-height: 19px;
}

.license-contact__item a {
  color: #1769d2;
  font-size: 12px;
  line-height: 19px;
  overflow-wrap: anywhere;
  text-decoration: none;
}

.license-contact__item a:hover {
  text-decoration: underline;
}

.license-contact__experience {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 5px 10px;
}

.license-contact__experience span {
  color: #667287;
  font-size: 11px;
}

.license-contact__experience code {
  padding: 1px 4px;
  border-radius: 4px;
  color: #43536a;
  background: #f1f4f8;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.license-contact__qr-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  border-left: 1px solid #e9eef5;
}

.license-contact__qr-item {
  display: flex;
  align-items: center;
  flex-direction: column;
  justify-content: center;
  min-width: 0;
  margin: 0;
  padding: 6px;
  text-align: center;
}

.license-contact__qr-item + .license-contact__qr-item {
  border-left: 1px solid #e9eef5;
}

.license-contact__qr-item > a {
  display: inline-flex;
  padding: 4px;
  border: 1px solid #dce6f2;
  border-radius: 8px;
  background: #fff;
  transition: border-color 0.16s ease, box-shadow 0.16s ease;
}

.license-contact__qr-item > a:hover,
.license-contact__qr-item > a:focus-visible {
  border-color: #7fb5f5;
  outline: 0;
  box-shadow: 0 0 0 2px rgba(64, 158, 255, 0.12);
}

.license-contact__qr-item img {
  display: block;
  width: 92px;
  height: 92px;
  object-fit: contain;
}

.license-contact__qr-item figcaption {
  display: flex;
  flex-direction: column;
  margin-top: 6px;
  gap: 1px;
}

.license-contact__qr-item figcaption strong {
  color: #263248;
  font-size: 12px;
  line-height: 18px;
}

.license-contact__qr-item figcaption span {
  color: #7b8798;
  font-size: 10px;
  line-height: 16px;
}

.license-contact__item a:focus-visible {
  border-radius: 3px;
  outline: 2px solid #2f7df4;
  outline-offset: 2px;
}

.license-refresh-button:focus-visible,
.license-copy-button:focus-visible,
.license-activation__button:focus-visible {
  outline: 2px solid #2f7df4;
  outline-offset: 2px;
}

@media (max-width: 960px) {
  .license-page {
    overflow: auto;
  }

  .license-main-grid.has-activation {
    grid-template-columns: minmax(0, 1fr);
  }

  .license-contact__body {
    grid-template-columns: minmax(0, 1fr);
  }

  .license-contact__qr-grid {
    border-top: 1px solid #e9eef5;
    border-left: 0;
  }
}

@media (max-width: 760px) {
  .license-page-header {
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .license-page-header__content {
    align-items: flex-start;
    flex-direction: column;
    gap: 1px;
  }

  .license-page-header__divider {
    display: none;
  }

  .license-page-header__actions {
    justify-content: flex-end;
    width: 100%;
    margin: 0;
  }

  .license-status {
    grid-template-columns: minmax(0, 1fr);
  }

  .license-status__metrics {
    border-top: 1px solid rgba(106, 120, 141, 0.18);
    border-left: 0;
  }
}

@media (max-width: 620px) {
  .license-page {
    padding: 6px;
  }

  .license-error {
    grid-template-columns: 18px minmax(0, 1fr);
  }

  .license-error .el-button {
    grid-column: 2;
    justify-self: start;
  }

  .license-status__metrics,
  .license-detail__grid,
  .license-contact__links {
    grid-template-columns: minmax(0, 1fr);
  }

  .license-status__metric + .license-status__metric {
    border-top: 1px solid rgba(106, 120, 141, 0.18);
    border-left: 0;
  }

  .machine-code-item,
  .module-scope-item {
    grid-column: auto;
  }
}

@media (prefers-reduced-motion: reduce) {
  .license-refresh-button,
  .machine-code-item code,
  .license-copy-button,
  .license-contact__qr-item > a {
    transition: none;
  }
}

@media (max-width: 420px) {
  .license-contact__qr-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .license-contact__qr-item + .license-contact__qr-item {
    border-top: 1px solid #e9eef5;
    border-left: 0;
  }
}

@media (hover: none) {
  .license-copy-button {
    opacity: 1;
    visibility: visible;
    transform: none;
  }
}
</style>
