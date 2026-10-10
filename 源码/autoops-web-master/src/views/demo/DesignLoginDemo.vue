<template>
  <div class="design-login-page">
    <div class="ambient ambient-left"></div>
    <div class="ambient ambient-right"></div>
    <div class="grid-mask"></div>

    <div class="workspace-shell" aria-hidden="true">
      <aside class="shell-sidebar">
        <div class="shell-brand">
          <div class="shell-brand-mark">
            <el-icon><Connection /></el-icon>
          </div>
          <div>
            <strong>SxDevOps</strong>
            <span>AI AGENT</span>
          </div>
        </div>

        <nav class="shell-nav">
          <div
            v-for="section in navSections"
            :key="section.title"
            class="nav-section"
          >
            <p class="nav-title">{{ section.title }}</p>
            <button
              v-for="item in section.items"
              :key="item"
              class="nav-item"
              type="button"
            >
              <span class="nav-bullet"></span>
              {{ item }}
            </button>
          </div>
        </nav>

        <div class="assistant-chip">
          <div class="assistant-avatar">
            <el-icon><ChatDotRound /></el-icon>
          </div>
          <div>
            <strong>AIOps</strong>
            <span>智能助手在线</span>
          </div>
        </div>
      </aside>

      <main class="shell-main">
        <header class="shell-header">
          <div>
            <p class="shell-breadcrumb">AIOps / 智能助手</p>
            <h2>执行确认与异常分析</h2>
          </div>
          <div class="shell-actions">
            <span class="signal-dot"></span>
            <div class="avatar-pill">A</div>
          </div>
        </header>

        <section class="shell-summary">
          <article v-for="card in summaryCards" :key="card.label" class="summary-card">
            <span class="summary-value">{{ card.value }}</span>
            <span class="summary-label">{{ card.label }}</span>
          </article>
        </section>

        <section class="shell-panel">
          <div class="shell-panel-header">
            <div class="shell-panel-title">
              <div class="mini-badge">
                <el-icon><DataBoard /></el-icon>
              </div>
              <div>
                <strong>智能体策略</strong>
                <span>统一管理策略、MCP 与审计动作</span>
              </div>
            </div>
            <button type="button" class="ghost-action">保存策略</button>
          </div>

          <div class="shell-table">
            <div class="shell-table-head">
              <span>名称</span>
              <span>类型</span>
              <span>启用工具</span>
              <span>状态</span>
            </div>
            <div v-for="row in previewRows" :key="row.name" class="shell-table-row">
              <span>{{ row.name }}</span>
              <span class="row-tag">{{ row.type }}</span>
              <span>{{ row.tool }}</span>
              <span class="row-status">{{ row.status }}</span>
            </div>
          </div>
        </section>
      </main>
    </div>

    <section class="auth-stage">
      <div class="auth-dialog">
        <aside class="intro-panel">
          <div class="brand-pill">
            <div class="brand-pill-icon">
              <el-icon><Monitor /></el-icon>
            </div>
            <span>AIOps Workspace</span>
          </div>

          <p class="intro-eyebrow">Private Access</p>
          <h1>欢迎回到你的运维控制台</h1>
          <p class="intro-copy">
            以飞书式清爽层次和 Linear 式克制细节重构登录体验，
            让资产、告警、执行与 AI 分析在同一入口下保持统一。
          </p>

          <div class="feature-list">
            <article class="feature-item">
              <div class="feature-icon">
                <el-icon><FolderOpened /></el-icon>
              </div>
              <div>
                <strong>统一资源视角</strong>
                <span>资产分组、可观测性与任务执行保持同一信息层级。</span>
              </div>
            </article>

            <article class="feature-item">
              <div class="feature-icon">
                <el-icon><TrendCharts /></el-icon>
              </div>
              <div>
                <strong>更轻的科技感</strong>
                <span>保留控制台属性，但把装饰退到背景，突出清晰与秩序。</span>
              </div>
            </article>

            <article class="feature-item">
              <div class="feature-icon">
                <el-icon><Lock /></el-icon>
              </div>
              <div>
                <strong>安全接入链路</strong>
                <span>账号接入、权限校验与执行确认在统一会话里完成闭环。</span>
              </div>
            </article>
          </div>

          <div class="trust-row">
            <span class="trust-chip">SSO Ready</span>
            <span class="trust-chip">Audit Trace</span>
            <span class="trust-chip">AI Copilot</span>
          </div>
        </aside>

        <div class="form-panel">
          <div class="form-panel-head">
            <div>
              <p class="form-kicker">Sign in</p>
              <h2>平台登录</h2>
            </div>
            <span class="form-status">安全接入已启用</span>
          </div>

          <el-form
            ref="formRef"
            :model="form"
            :rules="rules"
            label-position="top"
            class="login-form"
          >
            <el-form-item label="账号" prop="username">
              <el-input
                v-model="form.username"
                size="large"
                clearable
                placeholder="请输入账号"
              >
                <template #prefix>
                  <el-icon><User /></el-icon>
                </template>
              </el-input>
            </el-form-item>

            <el-form-item label="密码" prop="password">
              <el-input
                v-model="form.password"
                size="large"
                show-password
                type="password"
                placeholder="请输入密码"
              >
                <template #prefix>
                  <el-icon><Lock /></el-icon>
                </template>
              </el-input>
            </el-form-item>

            <div class="form-meta">
              <el-checkbox v-model="form.remember">保持登录状态</el-checkbox>
              <button type="button" class="text-link">忘记密码</button>
            </div>

            <div class="action-group">
              <el-button
                type="primary"
                size="large"
                class="primary-button"
                :loading="submitting"
                @click="submit"
              >
                进入控制台
              </el-button>
              <el-button
                size="large"
                class="secondary-button"
                @click="reset"
              >
                重置
              </el-button>
            </div>
          </el-form>

          <div class="form-footer">
            <div class="footer-note">
              <span class="note-dot"></span>
              <span>当前页面是图片风格重构后的 demo，不影响现有登录流程。</span>
            </div>
            <div class="footer-actions">
              <button type="button" class="demo-trigger-button" @click="dialogVisible = true">
                查看弹框 Demo
              </button>
              <div class="footer-tags">
                <span>Feishu-inspired</span>
                <span>Linear-inspired</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <transition name="demo-fade">
      <div
        v-if="dialogVisible"
        class="assistant-demo-overlay"
        @click.self="dialogVisible = false"
      >
        <div class="assistant-demo-modal">
          <div class="assistant-demo-header">
            <div class="assistant-demo-header-main">
              <div class="assistant-demo-logo">
                <el-icon><ChatDotRound /></el-icon>
              </div>
              <div class="assistant-demo-title-wrap">
                <div class="assistant-demo-title-row">
                  <h3>AIOps 智能助手</h3>
                  <span class="assistant-demo-badge">gpt-5.2-low</span>
                  <span class="assistant-demo-badge">执行需确认</span>
                </div>
                <p>你好，我可以帮你查询资源、告警和生成运维任务。</p>
              </div>
            </div>
            <div class="assistant-demo-header-actions">
              <button type="button">新会话</button>
              <button type="button" @click="dialogVisible = false">收起</button>
            </div>
          </div>

          <div class="assistant-demo-body">
            <aside class="assistant-demo-sidebar">
              <div class="assistant-demo-sidebar-head">
                <span>会话历史</span>
                <span class="assistant-demo-count">6</span>
              </div>
              <button
                v-for="item in dialogHistory"
                :key="item.title"
                type="button"
                class="assistant-demo-history-item"
                :class="{ active: item.active }"
              >
                {{ item.title }}
              </button>
            </aside>

            <section class="assistant-demo-content">
              <div class="assistant-demo-topic">
                <div>
                  <h4>为 10.10.1.11 生成 redis 巡检任务</h4>
                  <p>活跃于 04/12 20:45</p>
                </div>
                <div class="assistant-demo-topic-meta">
                  <span>只分析</span>
                  <span class="toggle-chip"></span>
                </div>
              </div>

              <div class="assistant-demo-message">
                <ul>
                  <li>巡检内容：版本 / 基础信息 / 关键指标（连接数 / 内存 / 碎片率 / ops）等。</li>
                  <li>默认检查地址：<strong>127.0.0.1:6379</strong>，在目标机本地执行 `redis-cli`。</li>
                </ul>
                <p>
                  回复 “确认执行” 即可；如果你的 Redis 不是 6379 或需要密码 / ACL，也请把端口与认证方式告诉我。
                </p>
                <div class="assistant-demo-message-tags">
                  <span>任务中心</span>
                </div>
              </div>

              <div class="assistant-demo-execution-card">
                <div class="assistant-demo-execution-head">
                  <div>
                    <h5>Ansible Playbook 执行</h5>
                    <p>状态：已执行</p>
                  </div>
                  <span class="assistant-demo-risk">高</span>
                </div>

                <div class="assistant-demo-execution-grid">
                  <article v-for="item in executionItems" :key="item.label" class="assistant-demo-execution-item">
                    <span>{{ item.label }}</span>
                    <strong>{{ item.value }}</strong>
                  </article>
                </div>

                <div class="assistant-demo-execution-foot">
                  已创建任务 #78
                  <button type="button">查看任务中心</button>
                </div>
              </div>

              <div class="assistant-demo-input-panel">
                <textarea
                  v-model="dialogInput"
                  class="assistant-demo-textarea"
                  placeholder="输入你的问题，Enter 发送，Shift + Enter 换行，Esc 收起"
                ></textarea>
                <div class="assistant-demo-input-meta">
                  <span>Enter 发送，Shift + Enter 换行</span>
                  <div class="assistant-demo-input-actions">
                    <button type="button" class="ghost" @click="dialogInput = ''">清空</button>
                    <button type="button" class="send">发送</button>
                  </div>
                </div>
              </div>
            </section>
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  ChatDotRound,
  Connection,
  DataBoard,
  FolderOpened,
  Lock,
  Monitor,
  TrendCharts,
  User
} from '@element-plus/icons-vue'

const formRef = ref()
const submitting = ref(false)
const dialogVisible = ref(false)
const dialogInput = ref('')

const navSections = [
  {
    title: '工作台',
    items: ['仪表盘', 'CMDB', '主机中心']
  },
  {
    title: '工单系统',
    items: ['应用发布', 'SQL 审计', '审批流']
  },
  {
    title: 'AI Ops',
    items: ['智能助手', '智能体配置']
  }
]

const summaryCards = [
  { value: '12', label: '活跃智能体' },
  { value: '8', label: '接入 MCP' },
  { value: '24', label: '执行策略' }
]

const previewRows = [
  { name: 'CMDB MCP', type: '平台内置', tool: 'query_cmdb_items', status: '已启用' },
  { name: 'Grafana MCP', type: 'HTTP', tool: 'search_dashboards', status: '已启用' },
  { name: 'SkyWalking MCP', type: 'STDIO', tool: 'query_traces', status: '已启用' }
]

const dialogHistory = [
  { title: '为 10.10.1.11 生成 redis 巡检任务', active: true },
  { title: '当前未确认的严重告警有哪些？', active: false },
  { title: 'app-prod-k8s 集群有没有异常？', active: false },
  { title: '数据平台生产环境月成本多少', active: false },
  { title: '分析生产 order-center 最近异常', active: false },
  { title: '最近电商线生产有哪些工单', active: false }
]

const executionItems = [
  { label: '目标主机', value: '1台' },
  { label: '执行方式', value: 'ansible' },
  { label: '执行策略', value: 'continue' },
  { label: '超时', value: '30s' }
]

const form = reactive({
  username: 'admin',
  password: '',
  remember: true
})

const rules = {
  username: [{ required: true, message: '请输入账号', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

const submit = () => {
  formRef.value?.validate(async valid => {
    if (!valid) return

    submitting.value = true
    await new Promise(resolve => setTimeout(resolve, 700))
    submitting.value = false
    ElMessage.success('新的极简高级风登录 demo 已可用，可以继续按这个方向扩展正式页面。')
  })
}

const reset = () => {
  formRef.value?.resetFields()
}
</script>

<style scoped>
.design-login-page {
  --bg-page: #f4f7fb;
  --bg-soft: #eef3f9;
  --surface: rgba(247, 250, 255, 0.82);
  --surface-strong: rgba(255, 255, 255, 0.92);
  --surface-muted: rgba(244, 247, 252, 0.78);
  --surface-warm: rgba(255, 253, 248, 0.92);
  --surface-warm-strong: rgba(255, 248, 239, 0.95);
  --border-soft: rgba(22, 34, 56, 0.06);
  --border-mid: rgba(207, 217, 230, 0.92);
  --text-primary: #172133;
  --text-secondary: #5f6b7c;
  --text-muted: #8d97a8;
  --nav-bg: #e5effd;
  --nav-bg-deep: #eff4fe;
  --nav-surface: rgba(236, 244, 253, 0.96);
  --nav-text: #5f6b7c;
  --nav-muted: #9aa4b2;
  --nav-active-text: #4d79d8;
  --nav-active-bg: rgba(231, 239, 252, 0.92);
  --brand: #4c8bf5;
  --topbar-gradient: linear-gradient(90deg, #fef7ef 0%, #f8f8f6 52%, #edf2fd 100%);
  --brand-soft: rgba(76, 139, 245, 0.12);
  --brand-fade: rgba(76, 139, 245, 0.06);
  --success: #3fb97f;
  --shadow-soft: 0 20px 60px rgba(17, 24, 39, 0.07);
  --shadow-panel: 0 36px 110px rgba(15, 23, 42, 0.11);
  min-height: 100vh;
  position: relative;
  overflow: hidden;
  background:
    radial-gradient(circle at 15% 18%, rgba(76, 139, 245, 0.08), transparent 26%),
    radial-gradient(circle at 82% 20%, rgba(128, 160, 220, 0.07), transparent 24%),
    linear-gradient(180deg, #f7f9fc 0%, #f4f7fb 100%);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.ambient {
  position: absolute;
  border-radius: 999px;
  filter: blur(70px);
  pointer-events: none;
}

.ambient-left {
  left: -80px;
  top: 120px;
  width: 260px;
  height: 260px;
  background: rgba(95, 138, 214, 0.1);
}

.ambient-right {
  right: -60px;
  bottom: 100px;
  width: 320px;
  height: 320px;
  background: rgba(187, 201, 228, 0.16);
}

.grid-mask {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(126, 152, 196, 0.025) 1px, transparent 1px),
    linear-gradient(90deg, rgba(126, 152, 196, 0.025) 1px, transparent 1px);
  background-size: 40px 40px;
  opacity: 0.4;
  pointer-events: none;
}

.workspace-shell {
  position: absolute;
  inset: 22px;
  display: grid;
  grid-template-columns: 220px 1fr;
  border-radius: 28px;
  background: rgba(246, 249, 253, 0.58);
  border: 1px solid rgba(255, 255, 255, 0.62);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.75);
  backdrop-filter: blur(18px);
  overflow: hidden;
}

.workspace-shell::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 72px;
  background: var(--topbar-gradient);
  border-bottom: 1px solid rgba(223, 231, 242, 0.94);
  z-index: 0;
}

.workspace-shell::after {
  content: '';
  position: absolute;
  inset: 0;
  background: rgba(247, 249, 253, 0.42);
  backdrop-filter: blur(10px);
}

.shell-sidebar,
.shell-main {
  position: relative;
  z-index: 1;
}

.shell-sidebar {
  padding: 22px 18px;
  border-right: 1px solid rgba(207, 217, 230, 0.72);
  background: linear-gradient(
    180deg,
    transparent 0,
    transparent 72px,
    var(--nav-bg) 72px,
    var(--nav-bg-deep) 100%
  );
  box-shadow:
    inset -1px 0 0 rgba(255, 255, 255, 0.55),
    inset 0 1px 0 rgba(255, 255, 255, 0.4);
}

.shell-brand {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 24px;
}

.shell-brand-mark {
  width: 38px;
  height: 38px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(245, 248, 252, 0.96);
  color: var(--brand);
  border: 1px solid rgba(208, 218, 232, 0.96);
}

.shell-brand strong,
.assistant-chip strong {
  display: block;
  font-size: 18px;
  color: var(--text-primary);
}

.shell-brand span,
.assistant-chip span {
  display: block;
  margin-top: 3px;
  font-size: 11px;
  letter-spacing: 1.1px;
  color: var(--nav-muted);
}

.shell-nav {
  display: grid;
  gap: 18px;
}

.nav-title {
  margin: 0 0 8px;
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.7px;
  color: var(--nav-muted);
}

.nav-item {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  margin-bottom: 4px;
  border: none;
  border-radius: 12px;
  background: transparent;
  text-align: left;
  color: var(--nav-text);
  font: inherit;
  font-size: 14px;
  font-weight: 500;
}

.nav-item:last-child {
  background: var(--nav-active-bg);
  color: var(--nav-active-text);
  box-shadow: inset 0 0 0 1px rgba(207, 220, 245, 0.7);
}

.nav-bullet {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: currentColor;
  opacity: 0.75;
}

.assistant-chip {
  position: absolute;
  left: 18px;
  right: 18px;
  bottom: 20px;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border-radius: 16px;
  background: var(--nav-surface);
  border: 1px solid rgba(208, 218, 232, 0.92);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.45);
}

.assistant-avatar {
  width: 34px;
  height: 34px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--brand);
  background: var(--brand-soft);
}

.shell-main {
  padding: 26px 28px;
  background: linear-gradient(
    180deg,
    transparent 0,
    transparent 72px,
    rgba(248, 250, 253, 0.18) 72px,
    rgba(248, 250, 253, 0.06) 100%
  );
}

.shell-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 18px;
  padding: 8px 2px 12px;
  border-radius: 0;
  background: transparent;
  border: none;
  box-shadow: none;
}

.shell-breadcrumb {
  margin: 0 0 4px;
  font-size: 13px;
  color: var(--brand);
  font-weight: 600;
}

.shell-header h2 {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
  color: var(--text-primary);
}

.shell-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.signal-dot {
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: #ff6b6b;
  box-shadow: 0 0 0 8px rgba(255, 107, 107, 0.08);
}

.avatar-pill {
  width: 36px;
  height: 36px;
  border-radius: 999px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(76, 139, 245, 0.12);
  color: var(--brand);
  font-weight: 700;
}

.shell-summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 16px;
}

.summary-card {
  padding: 18px 20px;
  border-radius: 18px;
  background: linear-gradient(180deg, rgba(255, 253, 249, 0.82), rgba(247, 250, 255, 0.8));
  border: 1px solid rgba(223, 231, 242, 0.9);
}

.summary-value {
  display: block;
  margin-bottom: 6px;
  font-size: 28px;
  font-weight: 700;
  color: var(--text-primary);
}

.summary-label {
  font-size: 13px;
  color: var(--text-secondary);
}

.shell-panel {
  padding: 18px;
  border-radius: 24px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.84), rgba(247, 250, 255, 0.82));
  border: 1px solid rgba(223, 231, 242, 0.92);
}

.shell-panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 18px;
}

.shell-panel-title {
  display: flex;
  align-items: center;
  gap: 12px;
}

.mini-badge {
  width: 38px;
  height: 38px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--brand);
  background: rgba(76, 139, 245, 0.12);
}

.shell-panel-title strong {
  display: block;
  font-size: 18px;
  color: var(--text-primary);
}

.shell-panel-title span {
  display: block;
  margin-top: 4px;
  font-size: 13px;
  color: var(--text-secondary);
}

.ghost-action {
  padding: 10px 16px;
  border-radius: 14px;
  border: 1px solid rgba(215, 224, 236, 0.92);
  background: rgba(255, 255, 255, 0.92);
  color: #5f6b7c;
  font: inherit;
  font-weight: 600;
}

.shell-table {
  border-radius: 18px;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.9);
  background: rgba(255, 255, 255, 0.88);
}

.shell-table-head,
.shell-table-row {
  display: grid;
  grid-template-columns: 1.3fr 0.9fr 1.4fr 0.7fr;
  gap: 12px;
  align-items: center;
  padding: 14px 18px;
}

.shell-table-head {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  background: rgba(248, 250, 255, 0.96);
}

.shell-table-row {
  font-size: 14px;
  color: var(--text-secondary);
  border-top: 1px solid rgba(15, 23, 42, 0.04);
}

.row-tag,
.row-status {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: fit-content;
  min-width: 64px;
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
}

.row-tag {
  color: var(--brand);
  background: rgba(76, 139, 245, 0.08);
}

.row-status {
  color: var(--success);
  background: rgba(63, 185, 127, 0.12);
}

.auth-stage {
  position: relative;
  z-index: 2;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
  box-sizing: border-box;
}

.auth-dialog {
  width: min(1100px, 100%);
  border-radius: 32px;
  border: 1px solid rgba(223, 231, 242, 0.96);
  background: linear-gradient(
    135deg,
    rgba(255, 253, 248, 0.97) 0%,
    rgba(255, 249, 241, 0.95) 42%,
    rgba(247, 250, 255, 0.93) 100%
  );
  box-shadow: var(--shadow-panel);
  backdrop-filter: blur(24px);
  overflow: hidden;
}

.auth-dialog {
  display: grid;
  grid-template-columns: minmax(320px, 0.95fr) minmax(340px, 0.82fr);
}

.intro-panel {
  padding: 40px 38px 34px;
  background: linear-gradient(
    180deg,
    rgba(255, 252, 246, 0.98) 0%,
    rgba(255, 248, 239, 0.92) 100%
  );
  border-right: 1px solid rgba(223, 231, 242, 0.9);
}

.brand-pill {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px 8px 8px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.94);
  border: 1px solid rgba(223, 231, 242, 0.92);
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 600;
}

.brand-pill-icon {
  width: 28px;
  height: 28px;
  border-radius: 999px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--brand-soft);
  color: var(--brand);
}

.intro-eyebrow {
  margin: 28px 0 10px;
  font-size: 12px;
  letter-spacing: 1.5px;
  font-weight: 700;
  color: var(--brand);
  text-transform: uppercase;
}

.intro-panel h1 {
  margin: 0;
  max-width: 460px;
  font-size: 40px;
  line-height: 1.14;
  letter-spacing: -0.03em;
  color: var(--text-primary);
}

.intro-copy {
  margin: 18px 0 28px;
  max-width: 500px;
  font-size: 15px;
  line-height: 1.9;
  color: var(--text-secondary);
}

.feature-list {
  display: grid;
  gap: 14px;
}

.feature-item {
  display: grid;
  grid-template-columns: 42px 1fr;
  gap: 14px;
  align-items: start;
  padding: 14px 16px;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.74);
  border: 1px solid rgba(228, 234, 242, 0.92);
}

.feature-icon {
  width: 42px;
  height: 42px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(76, 139, 245, 0.08);
  color: var(--brand);
}

.feature-item strong {
  display: block;
  margin-bottom: 6px;
  font-size: 15px;
  color: var(--text-primary);
}

.feature-item span {
  display: block;
  font-size: 13px;
  line-height: 1.75;
  color: var(--text-secondary);
}

.trust-row {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 24px;
}

.trust-chip {
  padding: 8px 12px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.88);
  border: 1px solid rgba(228, 234, 242, 0.92);
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 600;
}

.form-panel {
  padding: 36px 34px 30px;
  background: linear-gradient(
    180deg,
    rgba(251, 253, 255, 0.94) 0%,
    rgba(247, 250, 255, 0.9) 100%
  );
}

.form-panel-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 14px;
  margin-bottom: 28px;
}

.form-kicker {
  margin: 0 0 8px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 1.2px;
  color: var(--text-muted);
  text-transform: uppercase;
}

.form-panel-head h2 {
  margin: 0;
  font-size: 30px;
  line-height: 1.1;
  color: var(--text-primary);
}

.form-status {
  padding: 8px 12px;
  border-radius: 999px;
  background: rgba(235, 241, 250, 0.96);
  color: #5f6b7c;
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
}

.login-form :deep(.el-form-item) {
  margin-bottom: 20px;
}

.login-form :deep(.el-form-item__label) {
  padding-bottom: 8px;
  color: var(--text-primary);
  font-weight: 600;
}

.login-form :deep(.el-input__wrapper) {
  min-height: 50px;
  border-radius: 16px;
  background: rgba(252, 253, 255, 0.98);
  box-shadow: none;
  border: 1px solid rgba(214, 223, 235, 0.95);
  transition: border-color 0.2s ease, box-shadow 0.2s ease, background-color 0.2s ease;
}

.login-form :deep(.el-input__wrapper:hover) {
  border-color: rgba(171, 189, 220, 0.95);
}

.login-form :deep(.el-input__wrapper.is-focus) {
  background: #ffffff;
  border-color: rgba(120, 156, 221, 0.9);
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.07);
}

.login-form :deep(.el-input__prefix-inner) {
  color: #93a1b5;
}

.form-meta {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 14px;
  margin: 6px 0 22px;
}

.form-meta :deep(.el-checkbox__label) {
  color: var(--text-secondary);
}

.text-link {
  border: none;
  padding: 0;
  background: transparent;
  color: var(--brand);
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.action-group {
  display: grid;
  grid-template-columns: 1fr 120px;
  gap: 12px;
}

.primary-button,
.secondary-button {
  height: 48px;
  border-radius: 16px;
  font-size: 14px;
  font-weight: 600;
}

.primary-button {
  border: none;
  background: linear-gradient(180deg, #69a0fa 0%, #5a93f6 100%);
  box-shadow: 0 16px 26px rgba(76, 139, 245, 0.18);
}

.primary-button:hover {
  transform: translateY(-1px);
  box-shadow: 0 18px 28px rgba(76, 139, 245, 0.22);
}

.secondary-button {
  background: #ffffff;
  border: 1px solid rgba(214, 223, 235, 0.95);
  color: var(--text-secondary);
}

.secondary-button:hover {
  border-color: rgba(171, 189, 220, 0.95);
  color: var(--brand);
}

.form-footer {
  margin-top: 24px;
  padding-top: 18px;
  border-top: 1px solid rgba(223, 231, 242, 0.9);
}

.footer-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 14px;
  margin-top: 14px;
}

.demo-trigger-button {
  padding: 10px 14px;
  border: 1px solid rgba(214, 223, 235, 0.96);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.92);
  color: var(--text-secondary);
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.footer-note {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12px;
  line-height: 1.7;
  color: var(--text-secondary);
}

.note-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: var(--success);
  box-shadow: 0 0 0 6px rgba(63, 185, 127, 0.1);
  flex-shrink: 0;
}

.footer-tags {
  display: flex;
  gap: 8px;
  margin-top: 14px;
}

.footer-tags span {
  padding: 6px 10px;
  border-radius: 999px;
  background: rgba(237, 242, 251, 0.96);
  color: #6981b4;
  font-size: 11px;
  font-weight: 600;
}

.assistant-demo-overlay {
  position: fixed;
  inset: 0;
  z-index: 30;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: rgba(165, 178, 198, 0.16);
  backdrop-filter: blur(10px);
}

.assistant-demo-modal {
  width: min(960px, 100%);
  max-height: min(680px, calc(100vh - 48px));
  display: flex;
  flex-direction: column;
  border-radius: 30px;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.96);
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 42px 120px rgba(15, 23, 42, 0.16);
}

.assistant-demo-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 18px;
  padding: 18px 22px 14px;
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
  border-bottom: 1px solid rgba(223, 231, 242, 0.94);
}

.assistant-demo-header-main {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  min-width: 0;
}

.assistant-demo-logo {
  width: 42px;
  height: 42px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--brand);
  background: rgba(255, 255, 255, 0.9);
  border: 1px solid rgba(214, 223, 235, 0.95);
  flex-shrink: 0;
}

.assistant-demo-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.assistant-demo-title-row h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--text-primary);
}

.assistant-demo-badge {
  padding: 6px 10px;
  border-radius: 999px;
  background: rgba(234, 242, 252, 0.94);
  color: #5d7fc7;
  font-size: 12px;
  font-weight: 600;
}

.assistant-demo-title-wrap p {
  margin: 6px 0 0;
  font-size: 13px;
  color: var(--text-secondary);
}

.assistant-demo-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.assistant-demo-header-actions button {
  border: none;
  background: transparent;
  color: var(--text-secondary);
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.assistant-demo-body {
  min-height: 0;
  display: grid;
  grid-template-columns: 220px 1fr;
  flex: 1;
}

.assistant-demo-sidebar {
  padding: 14px;
  background: linear-gradient(180deg, #f5f8fd 0%, #edf2fb 100%);
  border-right: 1px solid rgba(223, 231, 242, 0.94);
  overflow: auto;
}

.assistant-demo-sidebar-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 600;
}

.assistant-demo-count {
  width: 24px;
  height: 24px;
  border-radius: 999px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.9);
  border: 1px solid rgba(214, 223, 235, 0.95);
  font-size: 12px;
  color: var(--text-muted);
}

.assistant-demo-history-item {
  width: 100%;
  margin-bottom: 8px;
  padding: 11px 12px;
  border: 1px solid transparent;
  border-radius: 14px;
  background: transparent;
  color: var(--text-secondary);
  text-align: left;
  font: inherit;
  font-size: 13px;
  font-weight: 500;
  line-height: 1.5;
  cursor: pointer;
}

.assistant-demo-history-item.active {
  background: rgba(255, 255, 255, 0.88);
  border-color: rgba(214, 223, 235, 0.95);
  color: var(--text-primary);
  box-shadow: 0 10px 20px rgba(15, 23, 42, 0.04);
}

.assistant-demo-content {
  padding: 14px 16px 16px;
  overflow: auto;
  background: rgba(255, 255, 255, 0.96);
}

.assistant-demo-topic {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 14px;
  margin-bottom: 14px;
}

.assistant-demo-topic h4 {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--text-primary);
}

.assistant-demo-topic p {
  margin: 4px 0 0;
  font-size: 13px;
  color: var(--text-muted);
}

.assistant-demo-topic-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 600;
}

.toggle-chip {
  width: 34px;
  height: 20px;
  border-radius: 999px;
  background: rgba(221, 227, 236, 0.96);
  position: relative;
}

.toggle-chip::after {
  content: '';
  position: absolute;
  left: 3px;
  top: 3px;
  width: 14px;
  height: 14px;
  border-radius: 999px;
  background: #ffffff;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.14);
}

.assistant-demo-message {
  padding: 16px 18px;
  border-radius: 20px;
  background: rgba(255, 255, 255, 0.94);
  border: 1px solid rgba(228, 234, 242, 0.92);
  color: var(--text-secondary);
  font-size: 14px;
  line-height: 1.8;
}

.assistant-demo-message ul {
  margin: 0;
  padding-left: 18px;
}

.assistant-demo-message p {
  margin: 14px 0 0;
}

.assistant-demo-message strong {
  color: #4f75d9;
}

.assistant-demo-message-tags {
  display: flex;
  gap: 8px;
  margin-top: 14px;
}

.assistant-demo-message-tags span {
  padding: 6px 10px;
  border-radius: 999px;
  background: rgba(237, 246, 242, 0.96);
  color: #4aa07d;
  font-size: 12px;
  font-weight: 600;
}

.assistant-demo-execution-card {
  margin-top: 16px;
  padding: 16px;
  border-radius: 18px;
  background: linear-gradient(180deg, rgba(255, 248, 238, 0.92), rgba(255, 250, 244, 0.94));
  border: 1px solid rgba(255, 192, 120, 0.7);
}

.assistant-demo-execution-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
  margin-bottom: 14px;
}

.assistant-demo-execution-head h5 {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: #b65f1c;
}

.assistant-demo-execution-head p {
  margin: 6px 0 0;
  font-size: 13px;
  color: #b67e4e;
}

.assistant-demo-risk {
  width: 26px;
  height: 26px;
  border-radius: 999px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 213, 219, 0.96);
  color: #d06778;
  font-size: 12px;
  font-weight: 700;
}

.assistant-demo-execution-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.assistant-demo-execution-item {
  padding: 12px 14px;
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.76);
  border: 1px solid rgba(255, 240, 224, 0.98);
}

.assistant-demo-execution-item span {
  display: block;
  margin-bottom: 6px;
  font-size: 12px;
  color: #b67e4e;
}

.assistant-demo-execution-item strong {
  display: block;
  font-size: 14px;
  color: #af5616;
}

.assistant-demo-execution-foot {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 14px;
  color: #9b7f60;
  font-size: 13px;
}

.assistant-demo-execution-foot button {
  border: none;
  background: transparent;
  color: #7b6d59;
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.assistant-demo-input-panel {
  margin-top: 16px;
}

.assistant-demo-textarea {
  width: 100%;
  min-height: 92px;
  padding: 14px 16px;
  border-radius: 14px;
  border: 1px solid rgba(112, 163, 247, 0.9);
  background: #ffffff;
  color: var(--text-primary);
  font: inherit;
  font-size: 14px;
  line-height: 1.7;
  resize: none;
  box-sizing: border-box;
  outline: none;
}

.assistant-demo-input-panel:focus-within .assistant-demo-textarea {
  box-shadow: 0 0 0 4px rgba(76, 139, 245, 0.08);
}

.assistant-demo-input-meta {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 14px;
  margin-top: 12px;
  color: var(--text-muted);
  font-size: 12px;
}

.assistant-demo-input-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.assistant-demo-input-actions button {
  height: 34px;
  padding: 0 14px;
  border-radius: 12px;
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.assistant-demo-input-actions .ghost {
  border: none;
  background: transparent;
  color: var(--text-secondary);
}

.assistant-demo-input-actions .send {
  border: none;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
  color: #ffffff;
  box-shadow: 0 10px 18px rgba(76, 139, 245, 0.18);
}

.demo-fade-enter-active,
.demo-fade-leave-active {
  transition: opacity 0.2s ease;
}

.demo-fade-enter-from,
.demo-fade-leave-to {
  opacity: 0;
}

@media (max-width: 1180px) {
  .workspace-shell {
    inset: 16px;
    grid-template-columns: 190px 1fr;
  }

  .auth-stage {
    padding: 24px;
  }

  .auth-dialog {
    grid-template-columns: 1fr;
  }

  .assistant-demo-body {
    grid-template-columns: 1fr;
  }

  .assistant-demo-sidebar {
    border-right: none;
    border-bottom: 1px solid rgba(223, 231, 242, 0.94);
  }

  .intro-panel {
    border-right: none;
    border-bottom: 1px solid rgba(15, 23, 42, 0.05);
  }
}

@media (max-width: 820px) {
  .workspace-shell {
    display: none;
  }

  .auth-stage {
    padding: 16px;
  }

  .auth-dialog {
    border-radius: 24px;
  }

  .assistant-demo-overlay {
    padding: 12px;
  }

  .assistant-demo-modal {
    max-height: calc(100vh - 24px);
    border-radius: 22px;
  }

  .assistant-demo-header {
    flex-direction: column;
    align-items: stretch;
  }

  .assistant-demo-header-actions,
  .footer-actions,
  .assistant-demo-input-meta,
  .assistant-demo-topic {
    flex-direction: column;
    align-items: flex-start;
  }

  .assistant-demo-execution-grid,
  .shell-summary,
  .action-group {
    grid-template-columns: 1fr;
  }

  .intro-panel,
  .form-panel {
    padding: 24px 20px;
  }

  .intro-panel h1 {
    font-size: 32px;
  }

  .form-panel-head {
    flex-direction: column;
  }

  .action-group {
    grid-template-columns: 1fr;
  }
}
</style>
