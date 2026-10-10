<template>
  <div class="assistant-page">
    <div class="assistant-shell">
      <section class="assistant-card">
        <header class="page-header">
          <div class="header-title">
            <span class="header-assistant-icon" aria-hidden="true">
              <img :src="assistantAvatar" alt="" />
            </span>
            <h2>AIOps 智能助手</h2>
            <el-select
              v-model="chatForm.providerId"
              clearable
              filterable
              placeholder="默认模型"
              :loading="providerLoading"
              class="provider-select"
            >
              <el-option
                v-for="provider in providerOptions"
                :key="getProviderId(provider)"
                :label="getProviderLabel(provider)"
                :value="getProviderId(provider)"
              />
            </el-select>
            <p>你好，我可以帮你结合平台上下文查询资源、分析根因并生成执行建议。</p>
          </div>
          <div class="header-actions">
            <span class="status-chip">
              {{ streamConnected ? '正在监听执行轨迹' : '当前为只读分析模式' }}
            </span>
          </div>
        </header>

        <div class="workspace">
          <aside class="session-panel">
            <div class="panel-title">
              <h3>会话历史</h3>
              <div class="session-actions">
                <el-button
                  circle
                  :icon="Delete"
                  class="session-action-btn danger"
                  :loading="historyClearing"
                  :disabled="!currentSession"
                  title="清理当前会话历史"
                  @click="clearChatHistory(false)"
                />
                <el-button :icon="Plus" class="session-new-button" @click="createSession()">新建</el-button>
              </div>
            </div>
            <div class="session-list">
              <button
                v-for="session in sessions"
                :key="session.id"
                type="button"
                class="session-item"
                :class="{ active: session.id === activeSessionId }"
                @click="switchSession(session.id)"
              >
                <span>{{ session.title || '新的会话' }}</span>
              </button>
            </div>

            <section class="template-panel">
              <details class="prompt-tip">
                <summary>提问小技巧</summary>
                <p>查询平台数据时补充“当前”“最近”或具体资源名称，结果会更准确。</p>
                <div class="prompt-tip-list">
                  <button
                    v-for="question in promptQuestions"
                    :key="question"
                    type="button"
                    class="template-item"
                    @click="applyPrompt(question)"
                  >
                    <span>{{ question }}</span>
                  </button>
                </div>
              </details>
            </section>
          </aside>

          <main class="chat-panel">
            <div class="conversation-toolbar">
              <div class="conversation-context">
                <strong :title="currentSession?.title || '新的会话'">{{ currentSession?.title || '新的会话' }}</strong>
                <el-popover placement="bottom-start" width="420" trigger="click" popper-class="assistant-context-popover">
                  <template #reference>
                    <el-button
                      class="environment-button"
                      :class="{ 'is-bound': contextForm.pageResourceId || contextForm.host || contextForm.alert }"
                      :icon="Setting"
                    >
                      {{ contextForm.pageResourceId || contextForm.host || contextForm.alert ? '已绑定环境' : '未指定环境' }}
                    </el-button>
                  </template>
                  <div class="context-editor">
                    <el-row :gutter="10">
                      <el-col :span="12">
                        <el-form-item label="页面类型">
                          <el-input v-model.trim="contextForm.pageType" placeholder="global_chat" />
                        </el-form-item>
                      </el-col>
                      <el-col :span="12">
                        <el-form-item label="主对象 ID">
                          <el-input v-model.trim="contextForm.pageResourceId" placeholder="hostId / alertId" />
                        </el-form-item>
                      </el-col>
                    </el-row>
                    <el-row :gutter="10">
                      <el-col :span="12">
                        <el-form-item label="主机">
                          <el-input v-model.trim="contextForm.host" placeholder="IP / hostname" />
                        </el-form-item>
                      </el-col>
                      <el-col :span="12">
                        <el-form-item label="告警">
                          <el-input v-model.trim="contextForm.alert" placeholder="alertId / fingerprint" />
                        </el-form-item>
                      </el-col>
                    </el-row>
                    <el-form-item label="扩展 JSON">
                      <el-input
                        v-model="contextForm.extraJson"
                        type="textarea"
                        :rows="4"
                        resize="none"
                        placeholder='{"groupName":"AIOPS开发环境"}'
                      />
                    </el-form-item>
                  </div>
                </el-popover>
                <span class="mode-label">分析范围</span>
                <el-select v-model="chatForm.mode" class="mode-select">
                  <el-option label="全局" value="global" />
                  <el-option label="页面" value="page" />
                  <el-option label="Web" value="web" />
                </el-select>
              </div>
              <span class="readonly-note">{{ streamConnected ? '正在监听执行轨迹' : '只读模式下不会执行变更操作' }}</span>
            </div>

            <div ref="messageViewportRef" class="message-viewport" v-loading="sendLoading">
              <template v-if="messages.length">
                <article
                  v-for="message in messages"
                  :key="message.id"
                  class="message-row"
                  :class="[
                    `is-${message.role}`,
                    { 'requires-confirmation': isMcpActionPending(message.result?.pendingAction) }
                  ]"
                >
                  <div v-if="message.role === 'assistant'" class="message-origin">
                    <span>智能助手</span>
                    <span>{{ message.createdAt }}</span>
                  </div>
                  <div class="message-bubble">
                    <section
                      v-if="message.role === 'assistant' && hasAnalysisTaskFlow(message.result)"
                      class="thinking-panel diagnosis-thinking-panel"
                      :class="`status-${getThinkingStatus(message)}`"
                    >
                      <button
                        type="button"
                        class="thinking-summary"
                        :aria-expanded="isThinkingExpanded(message.id)"
                        :aria-controls="`thinking-detail-${message.id}`"
                        @click="toggleThinking(message.id)"
                      >
                        <span class="thinking-summary-main">
                          <el-icon class="thinking-status-icon" :class="{ 'is-loading': getThinkingStatus(message) === 'running' }">
                            <Loading v-if="getThinkingStatus(message) === 'running'" />
                            <WarningFilled v-else-if="getThinkingStatus(message) === 'failed'" />
                            <CircleCheck v-else />
                          </el-icon>
                          <strong>诊断过程</strong>
                          <small>{{ getThinkingSummary(message) }}</small>
                        </span>
                        <span class="thinking-summary-actions">
                          <em>{{ getProgressStatusText(getThinkingStatus(message)) }}</em>
                          <el-icon class="thinking-toggle-icon" :class="{ expanded: isThinkingExpanded(message.id) }"><ArrowDown /></el-icon>
                        </span>
                      </button>
                      <div v-show="isThinkingExpanded(message.id)" :id="`thinking-detail-${message.id}`" class="thinking-detail diagnosis-thinking-detail">
                        <div class="diagnosis-flow-head">
                          <div>
                            <span>诊断任务流</span>
                            <strong>{{ getAnalysisTaskMessage(message.result) }}</strong>
                          </div>
                          <em>{{ getAnalysisTaskPercent(message.result) }}%</em>
                        </div>
                        <el-progress
                          :percentage="getAnalysisTaskPercent(message.result)"
                          :status="getElementProgressStatus(getThinkingStatus(message))"
                          :stroke-width="6"
                          :show-text="false"
                        />
                        <div class="diagnosis-flow-steps">
                          <div
                            v-for="(step, index) in getAnalysisTaskFlow(message.result)"
                            :key="step.key"
                            class="diagnosis-flow-step"
                            :class="`status-${step.status}`"
                          >
                            <span class="diagnosis-flow-index">{{ index + 1 }}</span>
                            <strong>{{ step.title }}</strong>
                            <em>{{ getProgressStatusText(step.status) }}</em>
                          </div>
                        </div>
                      </div>
                    </section>
                    <section
                      v-else-if="message.role === 'assistant' && shouldShowGeneralThinking(message)"
                      class="thinking-panel general-thinking-panel"
                      :class="`status-${getThinkingStatus(message)}`"
                    >
                      <button
                        type="button"
                        class="thinking-summary"
                        :aria-expanded="isThinkingExpanded(message.id)"
                        :aria-controls="`thinking-detail-${message.id}`"
                        @click="toggleThinking(message.id)"
                      >
                        <span class="thinking-summary-main">
                          <el-icon class="thinking-status-icon" :class="{ 'is-loading': getThinkingStatus(message) === 'running' }">
                            <Loading v-if="getThinkingStatus(message) === 'running'" />
                            <WarningFilled v-else-if="getThinkingStatus(message) === 'failed'" />
                            <CircleCheck v-else />
                          </el-icon>
                          <strong>思考过程</strong>
                          <small>{{ getThinkingSummary(message) }}</small>
                        </span>
                        <span class="thinking-summary-actions">
                          <em>{{ getProgressStatusText(getThinkingStatus(message)) }}</em>
                          <el-icon class="thinking-toggle-icon" :class="{ expanded: isThinkingExpanded(message.id) }"><ArrowDown /></el-icon>
                        </span>
                      </button>
                      <div v-show="isThinkingExpanded(message.id)" :id="`thinking-detail-${message.id}`" class="thinking-detail general-thinking-detail">
                        <div v-if="getThinkingTotalSteps(message)" class="general-thinking-progress">
                          <div>
                            <span>执行进度</span>
                            <strong>{{ getThinkingCompletedSteps(message) }} / {{ getThinkingTotalSteps(message) }}</strong>
                          </div>
                          <el-progress
                            :percentage="getThinkingPercent(message)"
                            :status="getElementProgressStatus(getThinkingStatus(message))"
                            :stroke-width="5"
                            :show-text="false"
                          />
                        </div>
                        <div
                          v-for="step in getThinkingSteps(message)"
                          :key="step.key"
                          class="general-thinking-step"
                          :class="`status-${step.status}`"
                        >
                          <span class="general-thinking-step-icon" aria-hidden="true">
                            <el-icon :class="{ 'is-loading': step.status === 'running' }">
                              <Loading v-if="step.status === 'running'" />
                              <CircleCheck v-else-if="step.status === 'succeeded'" />
                              <WarningFilled v-else-if="step.status === 'failed'" />
                              <Clock v-else />
                            </el-icon>
                          </span>
                          <div>
                            <div class="general-thinking-step-title">
                              <strong>{{ step.title }}</strong>
                              <span><time v-if="step.meta">{{ step.meta }}</time><em>{{ getProgressStatusText(step.status) }}</em></span>
                            </div>
                            <p v-if="step.message">{{ step.message }}</p>
                            <code v-if="step.toolName" class="thinking-tool" :title="step.toolName">{{ step.toolName }}</code>
                            <span v-if="step.error" class="thinking-error">{{ step.error }}</span>
                          </div>
                        </div>
                        <p v-if="!getThinkingSteps(message).length" class="general-thinking-empty">正在等待执行轨迹</p>
                      </div>
                    </section>
                    <section
                      v-if="message.role === 'assistant' && message.result?.workloadStatus"
                      class="workload-status-panel"
                      :class="message.result.workloadStatus.healthy ? 'is-healthy' : 'is-warning'"
                    >
                      <header class="workload-status-header">
                        <div class="workload-status-heading">
                          <span class="workload-status-icon" aria-hidden="true">
                            <el-icon>
                              <CircleCheck v-if="message.result.workloadStatus.healthy" />
                              <WarningFilled v-else />
                            </el-icon>
                          </span>
                          <div>
                            <h3>{{ message.result.workloadStatus.workloadName }} 工作负载状态</h3>
                            <p>
                              {{ message.result.workloadStatus.clusterName || '未知集群' }}
                              <span>/</span>
                              {{ message.result.workloadStatus.namespace || 'default' }}
                              <span>/</span>
                              {{ message.result.workloadStatus.workloadType || 'Workload' }}
                            </p>
                          </div>
                        </div>
                        <span class="workload-health-badge">
                          {{ getWorkloadHealthLabel(message.result.workloadStatus) }}
                        </span>
                      </header>

                      <div class="workload-metric-grid">
                        <div v-for="metric in getWorkloadMetrics(message.result.workloadStatus)" :key="metric.label" class="workload-metric">
                          <span>{{ metric.label }}</span>
                          <strong>{{ metric.value }}</strong>
                        </div>
                      </div>

                      <div v-if="message.result.workloadStatus.images.length" class="workload-image-row">
                        <span>运行镜像</span>
                        <code :title="message.result.workloadStatus.images.join('\n')">
                          {{ formatWorkloadImages(message.result.workloadStatus.images) }}
                        </code>
                      </div>

                      <div v-if="message.result.workloadStatus.pods.length" class="workload-pod-table-wrap">
                        <table class="workload-pod-table">
                          <thead>
                            <tr>
                              <th>Pod</th>
                              <th>节点</th>
                              <th>Pod IP</th>
                              <th>运行时长</th>
                              <th>重启</th>
                              <th>状态</th>
                            </tr>
                          </thead>
                          <tbody>
                            <tr v-for="pod in message.result.workloadStatus.pods" :key="pod.name">
                              <td><code :title="pod.name">{{ pod.name }}</code></td>
                              <td>{{ pod.nodeName || '-' }}</td>
                              <td>{{ pod.podIP || '-' }}</td>
                              <td>{{ pod.runningTime || '-' }}</td>
                              <td :class="{ 'has-restarts': pod.restartCount > 0 }">{{ pod.restartCount }}</td>
                              <td>
                                <span class="pod-status" :class="isHealthyPod(pod) ? 'is-running' : 'is-abnormal'">
                                  <i></i>{{ pod.status || pod.phase || 'Unknown' }}
                                </span>
                              </td>
                            </tr>
                          </tbody>
                        </table>
                      </div>
                      <div v-else class="workload-empty-pods">当前未返回关联 Pod</div>

                      <footer class="workload-status-footer">
                        <el-icon aria-hidden="true">
                          <CircleCheck v-if="message.result.workloadStatus.healthy" />
                          <WarningFilled v-else />
                        </el-icon>
                        <span>{{ getWorkloadConclusion(message.result.workloadStatus) }}</span>
                      </footer>
                    </section>
                    <div
                      v-else-if="message.role === 'assistant'"
                      class="answer-content"
                      v-html="renderAnswer(message.content)"
                      @click="handleAnswerContentClick"
                    />
                    <p v-else class="user-content">{{ message.content }}</p>

                    <div v-if="message.loading" class="typing-indicator">
                      <i></i>
                      <i></i>
                      <i></i>
                    </div>

                    <div v-if="message.error" class="error-strip">
                      {{ message.error }}
                    </div>

                    <section
                      v-if="message.result?.pendingAction?.requiresSelection === true"
                      class="mcp-selection-panel"
                    >
                      <header>
                        <strong>请选择目标工作负载</strong>
                        <span>{{ getMcpActionSelectionTitle(message.result.pendingAction) }}</span>
                      </header>
                      <div v-if="getMcpActionCandidates(message.result.pendingAction).length" class="mcp-selection-table-wrap">
                        <table class="mcp-selection-table">
                          <thead>
                            <tr><th></th><th>集群</th><th>Namespace</th><th>类型</th><th>名称</th></tr>
                          </thead>
                          <tbody>
                            <tr
                              v-for="(candidate, index) in getMcpActionCandidates(message.result.pendingAction)"
                              :key="getMcpCandidateKey(candidate, index)"
                              :class="{ 'is-selected': getMcpSelectionKey(message.id) === getMcpCandidateKey(candidate, index) }"
                              @click="selectMcpCandidate(message.id, candidate, index)"
                            >
                              <td>
                                <input
                                  type="radio"
                                  :name="`mcp-candidate-${message.id}`"
                                  :checked="getMcpSelectionKey(message.id) === getMcpCandidateKey(candidate, index)"
                                  @click.stop="selectMcpCandidate(message.id, candidate, index)"
                                >
                              </td>
                              <td>{{ getMcpCandidateCluster(candidate) || '-' }}</td>
                              <td>{{ candidate.namespace || '-' }}</td>
                              <td>{{ formatMcpCandidateType(candidate) }}</td>
                              <td><code>{{ getMcpCandidateName(candidate) || '-' }}</code></td>
                            </tr>
                          </tbody>
                        </table>
                      </div>
                      <p v-else class="mcp-selection-empty">未找到匹配的工作负载</p>
                      <footer>
                        <el-button
                          type="primary"
                          size="small"
                          :disabled="!getMcpSelectedCandidate(message.id) || sendLoading"
                          @click="confirmMcpSelection(message, getMcpSelectedCandidate(message.id))"
                        >
                          使用此目标继续
                        </el-button>
                      </footer>
                    </section>

                    <section
                      v-if="shouldShowMcpConfirmation(message.result?.pendingAction)"
                      class="mcp-confirmation-panel"
                      :class="`status-${getMcpActionStatus(message.result.pendingAction)}`"
                    >
                      <header class="mcp-confirmation-header">
                        <span class="mcp-confirmation-icon" aria-hidden="true">
                          <el-icon>
                            <CircleCheck v-if="isMcpActionCompleted(message.result.pendingAction)" />
                            <Close v-else-if="isMcpActionCancelled(message.result.pendingAction)" />
                            <Loading v-else-if="isMcpActionSubmitting(message.result.pendingAction)" class="is-loading" />
                            <WarningFilled v-else />
                          </el-icon>
                        </span>
                        <div>
                          <strong>{{ getMcpActionStatusLabel(message.result.pendingAction) }}</strong>
                          <span>{{ getMcpActionTitle(message.result.pendingAction) }}</span>
                        </div>
                      </header>

                      <dl class="mcp-confirmation-summary">
                        <div v-if="getMcpActionTargetLabel(message.result.pendingAction)">
                          <dt>操作目标</dt>
                          <dd><code>{{ getMcpActionTargetLabel(message.result.pendingAction) }}</code></dd>
                        </div>
                        <div v-if="getMcpActionEnvironmentLabel(message.result.pendingAction)">
                          <dt>执行环境</dt>
                          <dd>{{ getMcpActionEnvironmentLabel(message.result.pendingAction) }}</dd>
                        </div>
                        <div v-if="getMcpActionChangeText(message.result.pendingAction)">
                          <dt>变更内容</dt>
                          <dd>{{ getMcpActionChangeText(message.result.pendingAction) }}</dd>
                        </div>
                        <div>
                          <dt>执行结果</dt>
                          <dd>{{ getMcpActionResultText(message.result.pendingAction) }}</dd>
                        </div>
                        <div v-if="getMcpActionExpiryLabel(message.result.pendingAction)">
                          <dt>确认有效期</dt>
                          <dd>{{ getMcpActionExpiryLabel(message.result.pendingAction) }}</dd>
                        </div>
                      </dl>

                      <details v-if="hasMcpActionDetails(message.result.pendingAction)" class="mcp-confirmation-details">
                        <summary>查看执行详情</summary>
                        <dl>
                          <div v-if="message.result.pendingAction.toolName"><dt>工具</dt><dd><code>{{ message.result.pendingAction.toolName }}</code></dd></div>
                          <div v-if="message.result.pendingAction.command"><dt>命令</dt><dd><code>{{ message.result.pendingAction.command }}</code></dd></div>
                          <div v-if="message.result.pendingAction.message"><dt>状态信息</dt><dd>{{ message.result.pendingAction.message }}</dd></div>
                        </dl>
                      </details>

                      <p v-if="message.result.pendingAction.confirmationError" class="mcp-confirmation-error">
                        {{ message.result.pendingAction.confirmationError }}
                      </p>

                      <footer class="mcp-confirmation-actions">
                        <span v-if="!canConfirmMcpAction(message.result.pendingAction)">当前账号无权执行此操作</span>
                        <el-button
                          v-if="isMcpActionExpired(message.result.pendingAction) || isMcpActionInvalid(message.result.pendingAction)"
                          size="small"
                          :disabled="sendLoading"
                          @click="regenerateMcpPreview(message)"
                        >
                          重新生成预览
                        </el-button>
                        <template v-else-if="isMcpActionPending(message.result.pendingAction)">
                          <el-button
                            size="small"
                            :loading="isMcpActionCancelling(message.result.pendingAction)"
                            :disabled="isMcpActionSubmitting(message.result.pendingAction)"
                            @click="cancelMcpAction(message.id, message.result.pendingAction)"
                          >
                            取消
                          </el-button>
                          <el-button
                            :type="isDestructiveMcpAction(message.result.pendingAction) ? 'danger' : 'primary'"
                            size="small"
                            :loading="isMcpActionConfirming(message.result.pendingAction)"
                            :disabled="!canConfirmMcpAction(message.result.pendingAction) || !message.result.pendingAction.confirmationId || isMcpActionSubmitting(message.result.pendingAction)"
                            @click="confirmMcpAction(message.id, message.result.pendingAction)"
                          >
                            确认并执行
                          </el-button>
                        </template>
                        <el-tag
                          v-else
                          size="small"
                          :type="isMcpActionCompleted(message.result.pendingAction) ? 'success' : (isMcpActionFailed(message.result.pendingAction) ? 'danger' : 'info')"
                          effect="plain"
                        >
                          {{ getMcpActionStatusLabel(message.result.pendingAction) }}
                        </el-tag>
                      </footer>
                    </section>

                    <div v-if="message.result" class="message-meta">
                      <span>{{ getIntentLabel(message.result.intent) }}</span>
                      <span v-if="message.result.runId">Run #{{ message.result.runId }}</span>
                      <span>{{ message.result.pendingAction && isMcpActionPending(message.result.pendingAction) ? '待确认' : (message.result.readonly === false ? '变更' : '只读') }}</span>
                      <button type="button" @click="selectAssistantMessage(message.id)">查看结构</button>
                    </div>

                    <div v-if="getClarifySuggestions(message.result).length" class="suggestion-row">
                      <button
                        v-for="suggestion in getClarifySuggestions(message.result)"
                        :key="suggestion"
                        type="button"
                        @click="sendWithText(suggestion)"
                      >
                        {{ suggestion }}
                      </button>
                    </div>
                  </div>
                </article>
              </template>
              <el-empty v-else description="新会话已就绪" :image-size="94" />
            </div>

            <section class="composer">
              <el-form :model="chatForm" @submit.prevent>
                <el-input
                  v-model="chatForm.question"
                  type="textarea"
                  :autosize="{ minRows: 2, maxRows: 6 }"
                  maxlength="1000"
                  resize="none"
                  show-word-limit
                  placeholder="输入你的问题，Enter 发送，Shift + Enter 换行"
                  @keydown.enter.exact.prevent="sendMessage"
                  @keydown.ctrl.enter.prevent="sendMessage"
                />
                <div class="composer-footer">
                  <span class="composer-tip">Enter 发送，Shift + Enter 换行</span>
                  <div class="composer-actions">
                    <el-button :icon="Delete" :disabled="sendLoading || !chatForm.question" @click="clearComposer">清空</el-button>
                    <el-button
                      type="primary"
                      :icon="Promotion"
                      :loading="sendLoading"
                      :disabled="sendLoading || !chatForm.question"
                      @click="sendMessage"
                    >
                      发送
                    </el-button>
                  </div>
                </div>
              </el-form>
            </section>
          </main>

          <button
            v-show="insightDrawerVisible"
            type="button"
            class="insight-backdrop"
            aria-label="关闭诊断详情"
            @click="insightDrawerVisible = false"
          ></button>
          <aside v-show="insightDrawerVisible" class="insight-panel">
            <div class="panel-title">
              <div>
                <h3>诊断摘要</h3>
                <p>{{ selectedResult ? getIntentLabel(selectedResult.intent) : '等待结果' }}</p>
              </div>
              <div class="insight-actions">
                <el-button
                  v-if="selectedResult"
                  :icon="CopyDocument"
                  circle
                  title="复制结果"
                  @click="copySelectedResult"
                />
                <el-button :icon="Close" circle title="关闭详情" @click="insightDrawerVisible = false" />
              </div>
            </div>

            <template v-if="selectedResult">
              <DiagnosisReport
                v-if="reportSummary"
                :report="reportSummary"
                :show-facts="false"
                :max-summary-length="110"
              />

              <div v-if="contextMismatch" class="context-warning">
                <strong>上下文可能不一致</strong>
                <span>当前问题中的 IP 是 {{ contextMismatch.questionIp }}，但回答中出现 {{ contextMismatch.answerIp }}。建议重新按当前 IP 查询。</span>
              </div>

              <el-collapse
                v-if="hasDiagnosisCard"
                v-model="expandedDiagnosisSections"
                class="detail-diagnosis-collapse"
              >
                <el-collapse-item name="detail">
                  <template #title>
                    <span class="collapse-title">
                      详细诊断
                      <small>展开</small>
                    </span>
                  </template>
                  <section class="diagnosis-card" :class="`tone-${diagnosisSeverity}`">
                    <div class="diagnosis-head">
                      <el-icon><CircleCheck /></el-icon>
                      <div>
                        <h4>{{ reportSummary?.summary || diagnosisCard.summary || selectedResult.answer || '诊断完成' }}</h4>
                        <span>{{ diagnosisCard.handoffNote || '本轮未执行任何变更操作。' }}</span>
                      </div>
                    </div>
                    <div v-if="normalizeList(diagnosisCard.impactedResources).length" class="tag-row">
                      <el-tag
                        v-for="item in normalizeList(diagnosisCard.impactedResources)"
                        :key="formatDisplay(item)"
                        effect="light"
                      >
                        {{ formatDisplay(item) }}
                      </el-tag>
                    </div>
                    <div v-if="normalizeList(diagnosisCard.hypotheses).length" class="section-list">
                      <strong>关键依据</strong>
                      <div
                        v-for="item in normalizeList(diagnosisCard.hypotheses)"
                        :key="formatDisplay(item)"
                        class="list-item"
                      >
                        <span>{{ item.cause || formatDisplay(item) }}</span>
                        <small v-if="item.confidence">confidence: {{ item.confidence }}</small>
                      </div>
                    </div>
                    <div v-if="normalizeList(diagnosisCard.nextChecks).length" class="section-list">
                      <strong>下一步</strong>
                      <span
                        v-for="item in normalizeList(diagnosisCard.nextChecks)"
                        :key="formatDisplay(item)"
                        class="dot-line"
                      >
                        {{ formatDisplay(item) }}
                      </span>
                    </div>
                  </section>
                </el-collapse-item>
              </el-collapse>

              <el-collapse
                v-if="missingEvidenceItems.length"
                v-model="expandedDiagnosisSections"
                class="missing-collapse"
              >
                <el-collapse-item name="missing">
                  <template #title>
                    <span class="collapse-title">
                      缺失数据
                      <small>{{ missingEvidenceItems.length }}</small>
                    </span>
                  </template>
                  <span
                    v-for="item in missingEvidenceItems"
                    :key="item"
                    class="dot-line"
                  >
                    {{ item }}
                  </span>
                </el-collapse-item>
              </el-collapse>

              <div v-if="reflectionSummary" class="reflection-card">
                <strong>诊断可信度</strong>
                <span>{{ reflectionSummary.summary }}</span>
                <small>confidence: {{ reflectionSummary.confidenceLabel }}</small>
              </div>

              <el-empty v-if="!reportSummary && !hasDiagnosisCard" description="暂无诊断摘要" :image-size="74" />

              <el-collapse v-model="expandedInsightSections" class="execution-collapse">
                <el-collapse-item name="execution">
                  <template #title>
                    <span class="collapse-title">
                      执行详情
                      <small>{{ executionSummaryText }}</small>
                    </span>
                  </template>

                  <div class="execution-summary">
                    <span>意图：{{ selectedResult.intent || 'unknown' }}</span>
                    <span>工具：{{ usedTools.length }} 个</span>
                    <span>证据：{{ evidenceItems.length }} 条</span>
                    <span>置信度：{{ reportSummary?.confidenceLabel || '-' }}</span>
                  </div>

                  <section v-if="selectedProgress" class="progress-detail-card">
                    <div class="progress-detail-head">
                      <div>
                        <strong>执行进度</strong>
                        <span>当前：{{ getCurrentProgressStepName(selectedProgress) }}</span>
                      </div>
                      <el-tag
                        size="small"
                        effect="light"
                        :style="getProgressStatusStyle(selectedProgress.status)"
                      >
                        {{ getProgressStatusText(selectedProgress.status) }} · {{ selectedProgress.percent }}%
                      </el-tag>
                    </div>
                    <el-progress
                      :percentage="selectedProgress.percent"
                      :status="getElementProgressStatus(selectedProgress.status)"
                      :show-text="false"
                      class="progress-detail-bar"
                    />
                    <div v-if="selectedProgress.steps.length" class="progress-timeline">
                      <div
                        v-for="(step, index) in selectedProgress.steps"
                        :key="getProgressStepKey(step, index)"
                        class="progress-timeline-item"
                        :class="`status-${step.status}`"
                      >
                        <i></i>
                        <div>
                          <div class="progress-timeline-title">
                            <strong>{{ step.name }}</strong>
                            <span :style="getProgressStatusStyle(step.status)">
                              {{ getProgressStatusText(step.status) }}
                            </span>
                          </div>
                          <p v-if="step.purpose">{{ step.purpose }}</p>
                          <div class="progress-timeline-meta">
                            <span v-if="step.toolName">工具：{{ step.toolName }}</span>
                            <span v-if="step.percent || step.status === 'pending'">进度：{{ step.percent }}%</span>
                            <span v-if="step.durationMs">耗时：{{ formatDurationMs(step.durationMs) }}</span>
                            <span v-if="step.startedAt">开始：{{ formatProgressTime(step.startedAt) }}</span>
                            <span v-if="step.finishedAt">结束：{{ formatProgressTime(step.finishedAt) }}</span>
                          </div>
                          <el-tooltip
                            v-if="step.error"
                            placement="top-start"
                            :content="step.error"
                          >
                            <span class="progress-error">失败原因：{{ step.error }}</span>
                          </el-tooltip>
                        </div>
                      </div>
                    </div>
                  </section>

                  <div class="run-actions">
                    <el-button
                      :disabled="!selectedResult.runId || streamConnected"
                      :loading="streamConnecting"
                      :icon="Connection"
                      @click="subscribeSelectedRun"
                    >
                      监听轨迹
                    </el-button>
                    <el-button
                      :disabled="!selectedResult.runId"
                      :icon="Refresh"
                      @click="refreshSelectedRun"
                    >
                      回看 Run
                    </el-button>
                  </div>

                  <div class="run-grid">
                    <div>
                      <span>Run ID</span>
                      <strong>{{ selectedResult.runId || '-' }}</strong>
                    </div>
                    <div>
                      <span>调查计划</span>
                      <strong>{{ planSummary.success }} / {{ planSummary.total }}</strong>
                    </div>
                  </div>

                  <div v-if="assistantPlanItems.length" class="plan-card">
                    <div class="plan-card-head">
                      <strong>Planner 调查计划</strong>
                      <span>{{ planSummary.success }} / {{ planSummary.total }} 成功</span>
                    </div>
                    <div
                      v-for="(item, index) in assistantPlanItems"
                      :key="`${getStepName(item)}-${index}`"
                      class="plan-step"
                      :class="`status-${getStepStatus(item)}`"
                    >
                      <i>{{ index + 1 }}</i>
                      <div>
                        <strong>{{ getStepName(item) }}</strong>
                        <span>{{ getStepPurpose(item) }}</span>
                      </div>
                    </div>
                  </div>

                  <div v-if="planAndTimeline.length" class="timeline-list">
                    <div
                      v-for="(item, index) in planAndTimeline"
                      :key="`${getStepName(item)}-${index}`"
                      class="timeline-item"
                      :class="`status-${getStepStatus(item)}`"
                    >
                      <i></i>
                      <div>
                        <strong>{{ getStepName(item) }}</strong>
                        <span>{{ getStepPurpose(item) }}</span>
                        <small>{{ getStepStatusLabel(item) }}</small>
                      </div>
                    </div>
                  </div>
                  <el-empty v-else description="暂无执行轨迹" :image-size="74" />

                  <el-collapse v-if="evidenceGroups.length" v-model="expandedEvidence" class="evidence-collapse">
                    <el-collapse-item
                      v-for="group in evidenceGroups"
                      :key="group.name"
                      :name="group.name"
                    >
                      <template #title>
                        <span class="collapse-title">
                          {{ group.title }}
                          <small>{{ group.items.length }}</small>
                        </span>
                      </template>
                      <pre
                        v-for="(item, index) in group.items"
                        :key="`${group.name}-${index}`"
                        class="evidence-item"
                      >{{ formatEvidence(item) }}</pre>
                    </el-collapse-item>
                  </el-collapse>

                  <div v-if="entityRows.length" class="kv-grid">
                    <div v-for="item in entityRows" :key="item.key">
                      <span>{{ item.key }}</span>
                      <strong>{{ item.value }}</strong>
                    </div>
                  </div>
                </el-collapse-item>
              </el-collapse>

              <div v-if="reportSummary || hasDiagnosisCard" class="feedback-card">
                <strong>这个结论有帮助吗？</strong>
                <div class="feedback-actions">
                  <el-button size="small" :loading="feedbackLoading" @click="submitFeedback('useful')">有用</el-button>
                  <el-button size="small" :loading="feedbackLoading" @click="submitFeedback('inaccurate')">不准确</el-button>
                  <el-button size="small" :loading="feedbackLoading" @click="submitFeedback('insufficient_evidence')">证据不足</el-button>
                  <el-button size="small" :loading="feedbackLoading" @click="submitFeedback('tool_error')">工具错误</el-button>
                </div>
              </div>
            </template>
            <el-empty v-else description="发送问题后展示诊断摘要" :image-size="96" />
          </aside>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup>
import { computed, defineComponent, h, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  ArrowDown,
  CircleCheck,
  Clock,
  Close,
  Connection,
  CopyDocument,
  Delete,
  Loading,
  Plus,
  Promotion,
  Refresh,
  Setting,
  WarningFilled
} from '@element-plus/icons-vue'
import aiopsApi from '@/api/aiops'
import storage from '@/utils/storage'
import { withAppBasePath } from '@/utils/url'
import assistantAvatar from '@/assets/image/AI小助手.svg'

const DiagnosisReport = defineComponent({
  name: 'DiagnosisReport',
  props: {
    report: {
      type: Object,
      required: true
    },
    compact: {
      type: Boolean,
      default: false
    },
    showFacts: {
      type: Boolean,
      default: true
    },
    maxSummaryLength: {
      type: Number,
      default: 0
    }
  },
  setup(props) {
    const visibleMetrics = computed(() => props.compact
      ? props.report.metrics.slice(0, 5)
      : props.report.metrics)
    const summaryText = computed(() => {
      const text = props.report.summary || ''
      if (!props.maxSummaryLength || text.length <= props.maxSummaryLength) return text
      return `${text.slice(0, props.maxSummaryLength)}...`
    })
    return () => h('section', {
      class: ['diagnosis-report', { compact: props.compact }]
    }, [
      h('div', { class: 'report-summary' }, [
        h('div', { class: ['risk-badge', `tone-${props.report.severity}`] }, props.report.severityLabel),
        h('div', [
          h('h4', props.report.title),
          h('p', summaryText.value)
        ])
      ]),
      props.showFacts
        ? h('div', { class: 'report-facts' }, [
          props.report.primaryIssue
            ? h('div', [h('span', '主要异常'), h('strong', props.report.primaryIssue)])
            : null,
          props.report.judgement
            ? h('div', [h('span', '判断'), h('strong', props.report.judgement)])
            : null,
          h('div', [h('span', '置信度'), h('strong', props.report.confidenceLabel)])
        ].filter(Boolean))
        : null,
      visibleMetrics.value.length
        ? h('div', { class: 'report-metrics' }, visibleMetrics.value.map(metric =>
          h('div', { key: metric.name, class: ['metric-row', `tone-${metric.tone}`] }, [
            h('span', metric.name),
            h('strong', metric.value),
            h('small', metric.status)
          ])
        ))
        : null,
      props.report.missingEvidence.length
        ? h('details', { class: 'missing-evidence' }, [
          h('summary', `证据完整度：${props.report.completenessLabel}，缺失 ${props.report.missingEvidence.length} 项`),
          ...props.report.missingEvidence.map(item => h('span', { key: item }, item))
        ])
        : null,
      props.report.nextSteps.length && !props.compact
        ? h('div', { class: 'report-next' }, [
          h('strong', '下一步建议'),
          ...props.report.nextSteps.map(item => h('span', { key: item }, item))
        ])
        : null
    ])
  }
})

const successCodes = new Set([0, 200])
const sessionsStorageKey = 'aiops-general-assistant-sessions-v1'
const activeSessionStorageKey = 'aiops-general-assistant-active-v1'
const runningRunStatuses = new Set(['queued', 'pending', 'running', 'created', 'accepted', 'processing', 'started'])
const terminalRunStatuses = new Set(['success', 'succeeded', 'completed', 'done', 'failed', 'error', 'finished', 'cancelled', 'canceled'])
const progressStreamEventTypes = new Set(['run', 'step', 'intent', 'llm', 'open', 'progress', 'analysis_progress'])
const progressAnswerTexts = ['正在查询', '正在创建', '正在连接', '正在识别', '正在调用', '工具结果', '平台上下文', '处理中', '执行中']
const streamLogPrefix = '[AIOps Stream]'
const assistantProgressStatusMap = {
  pending: { text: '等待中', color: '#94a3b8' },
  running: { text: '执行中', color: '#3b82f6' },
  succeeded: { text: '已完成', color: '#16a34a' },
  success: { text: '已完成', color: '#16a34a' },
  completed: { text: '已完成', color: '#16a34a' },
  done: { text: '已完成', color: '#16a34a' },
  failed: { text: '失败', color: '#dc2626' },
  error: { text: '失败', color: '#dc2626' },
  skipped: { text: '已跳过', color: '#94a3b8' }
}
const analysisStepDefinitions = [
  { key: 'task_created', title: '任务创建' },
  { key: 'start', title: '开始分析' },
  { key: 'status_check', title: '状态检查' },
  { key: 'log_scan', title: '日志扫描' },
  { key: 'signal_scan', title: '事件与资源分析' },
  { key: 'ai_diagnosis', title: 'AI 诊断' }
]
const analysisStepKeys = new Set(analysisStepDefinitions.map(item => item.key))

const promptQuestions = [
  '帮我看下当前平台整体风险',
  '当前有哪些异常需要关注',
  '现在系统健康吗',
  '今天平台运维情况怎么样',
  '帮我分析一下当前平台有没有明显风险',
  '现在有哪些活跃告警',
  '当前 firing 告警有哪些',
  '帮我看下最近需要优先处理的告警',
  '有没有严重告警需要关注',
  '现在数据库资产状态怎么样',
  '帮我看下数据库整体情况',
  '当前有哪些数据库实例',
  '数据库资产里有没有异常状态的实例',
  '当前 K8s 集群健康情况怎么样',
  '帮我看下 K8s 集群整体状态',
  '有没有节点未 Ready 的集群',
  '现在集群有没有风险',
  '有没有 nginx 502 的处理方案',
  '知识库里有没有 MySQL 慢查询的排查文档',
  '有没有 Redis 内存高的 Runbook',
  '查一下 Pod CrashLoopBackOff 的处理方案',
  '帮我看一下 172.16.7.138 这台机器最近半小时有没有压力',
  '172.16.3.238 这个机器上面部署了什么服务',
  '帮我看下这台主机 CPU、内存、磁盘有没有异常',
  '帮我写一个 Linux 只读巡检脚本',
  '帮我写一个安装 nginx 的脚本',
  '如何排查 nginx 502',
  'Redis 内存高一般怎么排查'
]

const intentLabelMap = {
  platform_overview: '平台整体',
  host_pressure: '主机压力',
  alert_overview: '活跃告警',
  database_overview: '数据库概览',
  k8s_overview: 'K8s 概览',
  resource_governance: '资源治理',
  linux_health_check: 'Linux 巡检',
  clarify: '需要追问',
  unknown: '待识别'
}

const chatForm = reactive({
  question: '',
  providerId: '',
  mode: 'global'
})

const contextForm = reactive({
  pageType: 'global_chat',
  pageResourceId: '',
  host: '',
  alert: '',
  extraJson: ''
})

const sessions = ref([])
const activeSessionId = ref('')
const providerOptions = ref([])
const providerLoading = ref(false)
const sendLoading = ref(false)
const streamConnecting = ref(false)
const streamConnected = ref(false)
const feedbackLoading = ref(false)
const historyClearing = ref(false)
const expandedEvidence = ref(['tools', 'evidence', 'stream'])
const expandedDiagnosisSections = ref([])
const expandedInsightSections = ref([])
const expandedThinkingMessageIds = ref(new Set())
const selectedAssistantMessageId = ref('')
const insightDrawerVisible = ref(false)
const messageViewportRef = ref(null)
const mcpConfirmationSubmittingId = ref('')
const mcpConfirmationSubmittingMode = ref('')
const mcpConfirmationNow = ref(Date.now())
const mcpSelectionKeys = reactive({})
const mcpSelectionCandidates = reactive({})
let activeEventSource = null
let mcpConfirmationClockTimer = null

const currentSession = computed(() => sessions.value.find(item => item.id === activeSessionId.value) || null)
const messages = computed(() => currentSession.value?.messages || [])
const assistantMessages = computed(() => messages.value.filter(item => item.role === 'assistant' && item.result))
const selectedAssistantMessage = computed(() => {
  const selected = assistantMessages.value.find(item => item.id === selectedAssistantMessageId.value)
  return selected || assistantMessages.value[assistantMessages.value.length - 1] || null
})
const selectedResult = computed(() => selectedAssistantMessage.value?.result || null)
const selectedProgress = computed(() => getAssistantProgress(selectedResult.value))
const diagnosisCard = computed(() => selectedResult.value?.diagnosisCard || selectedResult.value?.diagnosis_card || {})
const hasDiagnosisCard = computed(() => Object.keys(diagnosisCard.value || {}).length > 0)
const diagnosisSeverity = computed(() => String(diagnosisCard.value?.severity || 'info').toLowerCase())
const usedTools = computed(() => normalizeList(selectedResult.value?.usedTools || selectedResult.value?.tools))
const evidenceItems = computed(() => normalizeList(selectedResult.value?.evidence || selectedResult.value?.evidences))
const assistantPlanItems = computed(() => normalizeList(
  selectedResult.value?.assistantPlan
  || selectedResult.value?.assistant_plan
  || selectedResult.value?.plan
  || selectedResult.value?.planner
))
const reflectionData = computed(() =>
  selectedResult.value?.reflection
  || selectedResult.value?.reflector
  || selectedResult.value?.selfReflection
  || selectedResult.value?.self_reflection
  || null)
const planAndTimeline = computed(() => [
  ...assistantPlanItems.value,
  ...normalizeList(selectedResult.value?.toolPlan || selectedResult.value?.tool_plan),
  ...normalizeList(selectedResult.value?.timeline || selectedResult.value?.steps || selectedResult.value?.run?.steps),
  ...normalizeList(selectedAssistantMessage.value?.streamEvents)
])
const planSummary = computed(() => getPlanSummary(assistantPlanItems.value.length ? assistantPlanItems.value : planAndTimeline.value))
const reportSummary = computed(() => buildDiagnosisReport(selectedResult.value))
const missingEvidenceItems = computed(() => reportSummary.value?.missingEvidence || [])
const reflectionSummary = computed(() => normalizeReflection(reflectionData.value, reportSummary.value))
const contextMismatch = computed(() => getContextMismatch(selectedResult.value))
const executionSummaryText = computed(() => {
  const intent = selectedResult.value?.intent || 'unknown'
  const confidence = reportSummary.value?.confidenceLabel || '-'
  return `${getIntentLabel(intent)} / ${confidence}`
})

const entityRows = computed(() => objectToRows(selectedResult.value?.entities))
const evidenceGroups = computed(() => {
  const groups = []
  if (usedTools.value.length) groups.push({ name: 'tools', title: '工具调用', items: usedTools.value })
  if (evidenceItems.value.length) groups.push({ name: 'evidence', title: '证据', items: evidenceItems.value })
  if (normalizeList(selectedAssistantMessage.value?.streamEvents).length) {
    groups.push({ name: 'stream', title: 'SSE 事件', items: selectedAssistantMessage.value.streamEvents })
  }
  return groups
})

const getBody = response => response?.data || response || {}

const assertSuccess = (response, fallbackMessage = '请求失败') => {
  const body = getBody(response)
  if (body.code !== undefined && !successCodes.has(Number(body.code))) {
    throw new Error(body.message || body.msg || fallbackMessage)
  }
  return body
}

const getPayload = (response, fallbackMessage = '请求失败') => {
  const body = assertSuccess(response, fallbackMessage)
  return body.data !== undefined ? body.data : body
}

const streamDebug = (stage, detail = {}) => {
  const payload = detail instanceof Error
    ? { message: detail.message, stack: detail.stack }
    : detail
  console.log(`${streamLogPrefix} ${stage}`, payload)
}

const streamWarn = (stage, detail = {}) => {
  const payload = detail instanceof Error
    ? { message: detail.message, stack: detail.stack }
    : detail
  console.warn(`${streamLogPrefix} ${stage}`, payload)
}

const normalizeList = value => {
  if (Array.isArray(value)) return value.filter(item => item !== undefined && item !== null && item !== '')
  if (Array.isArray(value?.list)) return value.list
  if (Array.isArray(value?.items)) return value.items
  if (Array.isArray(value?.records)) return value.records
  if (value === undefined || value === null || value === '') return []
  return [value]
}

const normalizeWorkloadStatus = value => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const numberValue = field => {
    const number = Number(value[field] ?? 0)
    return Number.isFinite(number) ? number : 0
  }
  const pods = normalizeList(value.pods).map(pod => ({
    name: String(pod?.name || ''),
    status: String(pod?.status || pod?.phase || ''),
    phase: String(pod?.phase || pod?.status || ''),
    restartCount: Number(pod?.restartCount ?? pod?.restart_count ?? 0) || 0,
    nodeName: String(pod?.nodeName || pod?.node_name || ''),
    podIP: String(pod?.podIP || pod?.podIp || pod?.pod_ip || ''),
    runningTime: String(pod?.runningTime || pod?.running_time || '')
  }))
  const desiredReplicas = numberValue('desiredReplicas')
  const currentReplicas = numberValue('currentReplicas')
  const readyReplicas = numberValue('readyReplicas')
  const availableReplicas = numberValue('availableReplicas')
  const status = String(value.status || '')
  const derivedHealthy = desiredReplicas === 0
    ? currentReplicas === 0 && readyReplicas === 0 && availableReplicas === 0
    : status.toLowerCase() === 'running' &&
      currentReplicas === desiredReplicas &&
      readyReplicas === desiredReplicas &&
      availableReplicas === desiredReplicas

  return {
    clusterId: value.clusterId || value.cluster_id || '',
    clusterName: String(value.clusterName || value.cluster_name || ''),
    namespace: String(value.namespace || ''),
    workloadType: String(value.workloadType || value.workload_type || ''),
    workloadName: String(value.workloadName || value.workload_name || ''),
    status,
    healthy: typeof value.healthy === 'boolean' ? value.healthy : derivedHealthy,
    desiredReplicas,
    currentReplicas,
    readyReplicas,
    availableReplicas,
    images: normalizeList(value.images).map(image => String(image)),
    pods,
    warningEvents: Number(value.warningEvents ?? value.warning_events ?? 0) || 0
  }
}

const getWorkloadMetrics = status => [
  { label: '期望副本', value: status?.desiredReplicas ?? 0 },
  { label: '当前副本', value: status?.currentReplicas ?? 0 },
  { label: 'Ready', value: status?.readyReplicas ?? 0 },
  { label: '可用副本', value: status?.availableReplicas ?? 0 }
]

const getWorkloadHealthLabel = status => {
  if (status?.desiredReplicas === 0 && status?.healthy) return '已缩容至 0'
  return status?.healthy ? '运行正常' : '未完全就绪'
}

const isHealthyPod = pod => String(pod?.status || pod?.phase || '').toLowerCase() === 'running'

const formatWorkloadImages = images => normalizeList(images).join(' / ')

const getWorkloadConclusion = status => {
  const pods = normalizeList(status?.pods)
  const restartCount = pods.reduce((total, pod) => total + (Number(pod?.restartCount) || 0), 0)
  const warningText = status?.warningEvents
    ? `当前有 ${status.warningEvents} 条 Warning 事件。`
    : '当前无 Warning 事件。'
  if (status?.desiredReplicas === 0 && status?.healthy) return `工作负载已缩容至 0，${warningText}`
  if (status?.healthy) {
    return `${status.readyReplicas}/${status.desiredReplicas} 个副本已就绪，${pods.length} 个 Pod 处于 Running，累计重启 ${restartCount} 次，${warningText}`
  }
  return `${status?.readyReplicas || 0}/${status?.desiredReplicas || 0} 个副本已就绪，请优先检查异常 Pod 和事件，${warningText}`
}

const isFilled = value => value !== undefined && value !== null && value !== ''

const getFirstFilledValue = (...values) => values.find(isFilled)

const getMcpText = value => {
  if (value === undefined || value === null || typeof value === 'object') return ''
  const text = String(value).trim()
  return ['undefined', 'null', '[object Object]'].includes(text) ? '' : text
}

const getMcpPendingActionSource = data => {
  if (!data || typeof data !== 'object' || Array.isArray(data)) return null
  const nestedCandidates = [
    data.pendingAction,
    data.pending_action,
    data.result?.pendingAction,
    data.result?.pending_action,
    data.output?.pendingAction,
    data.output?.pending_action,
    data.payload?.pendingAction,
    data.payload?.pending_action,
    data.data?.pendingAction,
    data.data?.pending_action
  ]
  const nested = nestedCandidates.find(candidate => candidate && typeof candidate === 'object' && !Array.isArray(candidate))
  if (nested) return nested

  const toolName = String(data.toolName || data.tool_name || data.tool || '').toLowerCase()
  const looksLikeAction = data.requiresConfirmation === true
    || data.requires_confirmation === true
    || data.requiresSelection === true
    || data.requires_selection === true
    || Boolean(data.confirmationId || data.confirmation_id || data.confirmation?.id)
    || toolName.startsWith('k8s_request_')
    || toolName.startsWith('ecs_request_')
  return looksLikeAction ? data : null
}

const normalizeMcpPendingAction = data => {
  const source = getMcpPendingActionSource(data)
  if (!source) return null

  const toolName = String(getFirstFilledValue(
    source.toolName,
    source.tool_name,
    source.tool,
    source.action,
    data.toolName,
    data.tool_name,
    data.tool,
    ''
  ) || '')
  const confirmationId = getFirstFilledValue(
    source.confirmationId,
    source.confirmation_id,
    source.confirmation?.id,
    source.confirmation?.confirmationId,
    source.confirmation?.confirmation_id,
    data.confirmationId,
    data.confirmation_id,
    ''
  )
  const target = source.target && typeof source.target === 'object' ? source.target : {}
  const changeDetail = source.change && typeof source.change === 'object' ? source.change : {}
  const inferredK8s = toolName.toLowerCase().startsWith('k8s_')
    || isFilled(source.clusterId)
    || isFilled(source.cluster_id)
    || (isFilled(source.namespace) && (isFilled(source.workloadName) || isFilled(target.name)))

  return {
    ...source,
    requiresConfirmation: source.requiresConfirmation === true
      || source.requires_confirmation === true
      || source.confirmation?.required === true
      || Boolean(confirmationId),
    requiresSelection: source.requiresSelection === true || source.requires_selection === true,
    confirmationId: String(confirmationId || ''),
    confirmationStatus: String(getFirstFilledValue(
      source.confirmationStatus,
      source.confirmation_status,
      source.confirmation?.status,
      'pending'
    ) || 'pending').toLowerCase(),
    status: String(getFirstFilledValue(source.status, source.phase, source.confirmation?.state, '') || '').toLowerCase(),
    actionType: String(getFirstFilledValue(source.actionType, source.action_type, inferredK8s ? 'k8s' : '', '') || ''),
    action: String(getFirstFilledValue(source.action, source.tool, source.toolName, source.tool_name, '') || ''),
    toolName,
    resourceType: String(getFirstFilledValue(
      source.resourceType,
      source.resource_type,
      source.workloadType,
      source.workload_type,
      source.kind,
      target.resourceType,
      target.resource_type,
      target.kind,
      ''
    ) || ''),
    resourceName: String(getFirstFilledValue(
      source.resourceName,
      source.resource_name,
      source.workloadName,
      source.workload_name,
      target.resourceName,
      target.resource_name,
      target.name,
      ''
    ) || ''),
    clusterId: getFirstFilledValue(source.clusterId, source.cluster_id, target.clusterId, target.cluster_id, ''),
    clusterName: String(getFirstFilledValue(
      source.clusterName,
      source.cluster_name,
      source.cluster,
      target.clusterName,
      target.cluster_name,
      target.cluster,
      ''
    ) || ''),
    namespace: String(getFirstFilledValue(source.namespace, target.namespace, '') || ''),
    summary: getMcpText(getFirstFilledValue(source.summary, changeDetail.summary, source.description, '')),
    change: getMcpText(getFirstFilledValue(changeDetail.summary, source.change, source.changeSummary, source.change_summary, source.description, '')),
    changeDetail,
    replicas: getFirstFilledValue(source.replicas, changeDetail.toReplicas, changeDetail.to_replicas),
    expectedReplicas: getFirstFilledValue(source.expectedReplicas, source.expected_replicas, changeDetail.fromReplicas, changeDetail.from_replicas),
    command: String(getFirstFilledValue(source.command, '') || ''),
    message: getMcpText(getFirstFilledValue(source.message, source.statusMessage, source.status_message, '')),
    expiresAt: getFirstFilledValue(
      source.expiresAt,
      source.expires_at,
      source.expireAt,
      source.expire_at,
      source.confirmation?.expiresAt,
      source.confirmation?.expires_at,
      ''
    ),
    candidates: normalizeList(source.candidates || source.candidateList || source.candidate_list),
    files: normalizeList(source.files || source.fileList || source.file_list),
    hostName: String(getFirstFilledValue(source.hostName, source.host_name, source.host?.name, '') || ''),
    hostIp: String(getFirstFilledValue(source.hostIp, source.host_ip, source.host?.ip, source.ip, '') || ''),
    confirmed: source.confirmed === true,
    executed: source.executed === true
  }
}

const getMcpActionCandidates = action => normalizeList(action?.candidates || action?.candidateList || action?.candidate_list)
const getMcpCandidateCluster = candidate => candidate?.clusterName || candidate?.cluster_name || candidate?.cluster || candidate?.clusterId || candidate?.cluster_id
const getMcpCandidateType = candidate => candidate?.workloadType || candidate?.workload_type || candidate?.resourceType || candidate?.resource_type || candidate?.kind
const getMcpCandidateName = candidate => candidate?.workloadName || candidate?.workload_name || candidate?.resourceName || candidate?.resource_name || candidate?.name
const formatMcpCandidateType = candidate => {
  const value = String(getMcpCandidateType(candidate) || '').trim()
  return value ? value.charAt(0).toUpperCase() + value.slice(1) : '-'
}
const getMcpCandidateKey = (candidate, index) => [
  candidate?.clusterId || candidate?.cluster_id || '',
  candidate?.namespace || '',
  getMcpCandidateType(candidate) || '',
  getMcpCandidateName(candidate) || index
].join('|')
const getMcpSelectionKey = messageId => mcpSelectionKeys[messageId] || ''
const getMcpSelectedCandidate = messageId => mcpSelectionCandidates[messageId] || null
const getMcpActionSelectionTitle = action => action?.workloadName || action?.workload_name || action?.resourceName || action?.summary || '发现多个匹配目标'
const selectMcpCandidate = (messageId, candidate, index) => {
  mcpSelectionKeys[messageId] = getMcpCandidateKey(candidate, index)
  mcpSelectionCandidates[messageId] = candidate
}

const confirmMcpSelection = (message, candidate) => {
  if (!candidate || sendLoading.value) return
  const workloadName = getMcpCandidateName(candidate) || ''
  const namespace = candidate.namespace || ''
  const clusterName = candidate.clusterName || candidate.cluster_name || candidate.cluster || ''
  sendWithText(`继续处理选中的 K8s 工作负载 ${workloadName}`, {
    clusterId: candidate.clusterId ?? candidate.cluster_id,
    clusterName,
    namespace,
    workloadType: getMcpCandidateType(candidate),
    workloadName
  })
}

const isK8sMcpAction = action => String(action?.actionType || '').toLowerCase() === 'k8s'
  || String(action?.toolName || action?.action || action?.tool || '').toLowerCase().startsWith('k8s_')

const isSystemAdmin = () => storage.getItem('sysAdmin')?.isSystemAdmin === true
const canConfirmMcpAction = action => isK8sMcpAction(action) || isSystemAdmin()

const getMcpActionExpiryTime = action => {
  const value = action?.expiresAt || action?.expires_at || action?.expireAt || action?.expire_at
  if (!isFilled(value)) return null
  if (typeof value === 'number' || /^\d+$/.test(String(value))) {
    const numericValue = Number(value)
    const milliseconds = numericValue < 1000000000000 ? numericValue * 1000 : numericValue
    return Number.isFinite(milliseconds) ? milliseconds : null
  }
  const parsed = Date.parse(String(value))
  return Number.isFinite(parsed) ? parsed : null
}

const isMcpActionCompleted = action => action?.executed === true
  || ['confirmed', 'completed', 'success', 'succeeded', 'executed', 'triggered'].includes(String(action?.confirmationStatus || action?.status || '').toLowerCase())
const isMcpActionCancelled = action => ['cancelled', 'canceled'].includes(String(action?.confirmationStatus || action?.status || '').toLowerCase())
const isMcpActionFailed = action => ['failed', 'error'].includes(String(action?.confirmationStatus || action?.status || '').toLowerCase())
const isMcpActionExpired = action => {
  if (String(action?.confirmationStatus || action?.status || '').toLowerCase() === 'expired') return true
  const expiresAt = getMcpActionExpiryTime(action)
  return expiresAt !== null && expiresAt <= mcpConfirmationNow.value
}
const isMcpActionInvalid = action => ['invalid', 'used', 'forbidden'].includes(String(action?.confirmationStatus || '').toLowerCase())
const getMcpActionStatus = action => {
  if (isMcpActionCompleted(action)) return 'completed'
  if (isMcpActionCancelled(action)) return 'cancelled'
  if (isMcpActionExpired(action)) return 'expired'
  if (isMcpActionInvalid(action)) return 'invalid'
  if (isMcpActionFailed(action)) return 'failed'
  return 'pending'
}
const isMcpActionPending = action => action?.requiresConfirmation === true && getMcpActionStatus(action) === 'pending'
const shouldShowMcpConfirmation = action => Boolean(action)
  && action.requiresSelection !== true
  && (action.requiresConfirmation === true || ['completed', 'cancelled', 'expired', 'invalid', 'failed'].includes(getMcpActionStatus(action)))

const getMcpActionStatusLabel = action => {
  if (isMcpActionSubmitting(action)) return mcpConfirmationSubmittingMode.value === 'cancel' ? '正在取消操作' : '正在确认并执行'
  return ({
    completed: '操作已执行',
    cancelled: '操作已取消',
    expired: '确认已过期',
    invalid: '确认凭证已失效',
    failed: '操作执行失败',
    pending: '操作待确认'
  })[getMcpActionStatus(action)]
}

const getMcpActionTitle = action => action?.summary || action?.change || action?.description || action?.toolName || '执行变更操作'
const getMcpActionTargetLabel = action => {
  if (isK8sMcpAction(action)) {
    const type = action?.resourceType || 'Workload'
    return action?.resourceName ? `${type} / ${action.resourceName}` : type
  }
  const hostName = String(action?.hostName || '').trim()
  const hostIp = String(action?.hostIp || '').trim()
  return hostName && hostIp ? `${hostName} (${hostIp})` : (hostName || hostIp)
}
const getMcpActionEnvironmentLabel = action => isK8sMcpAction(action)
  ? [action?.clusterName || action?.clusterId, action?.namespace].filter(Boolean).join(' / ')
  : ''
const getMcpActionChangeText = action => {
  const detail = action?.changeDetail || {}
  const fromReplicas = getFirstFilledValue(detail.fromReplicas, detail.from_replicas, action?.expectedReplicas)
  const toReplicas = getFirstFilledValue(detail.toReplicas, detail.to_replicas, action?.replicas)
  if (isFilled(toReplicas)) {
    return isFilled(fromReplicas) ? `副本数从 ${fromReplicas} 调整为 ${toReplicas}` : `副本数调整为 ${toReplicas}`
  }
  return action?.change || action?.summary || ''
}
const getMcpActionResultText = action => {
  if (isMcpActionCompleted(action)) return action?.message || '变更已执行'
  if (isMcpActionCancelled(action)) return action?.message || '未执行，资源保持不变'
  if (isMcpActionExpired(action)) return '未执行，请重新生成操作预览'
  if (isMcpActionFailed(action) || isMcpActionInvalid(action)) return action?.message || '未执行'
  return '等待用户确认，当前尚未执行'
}
const getMcpActionExpiryLabel = action => {
  const expiresAt = getMcpActionExpiryTime(action)
  if (expiresAt === null) return ''
  if (expiresAt <= mcpConfirmationNow.value) return '已过期'
  return `${new Date(expiresAt).toLocaleString('zh-CN', { hour12: false })} 前有效`
}
const hasMcpActionDetails = action => Boolean(action?.toolName || action?.command || action?.message)
const isDestructiveMcpAction = action => /cleanup|delete|stop|rollback/.test(String(action?.toolName || action?.action || '').toLowerCase())

const isMcpActionSubmitting = action => Boolean(action?.confirmationId)
  && mcpConfirmationSubmittingId.value === action.confirmationId
const isMcpActionConfirming = action => isMcpActionSubmitting(action) && mcpConfirmationSubmittingMode.value === 'confirm'
const isMcpActionCancelling = action => isMcpActionSubmitting(action) && mcpConfirmationSubmittingMode.value === 'cancel'

const updateMcpPendingAction = (messageId, action, patch) => {
  const message = messages.value.find(item => item.id === messageId)
  if (!message?.result) return
  const pendingAction = { ...action, ...patch }
  updateMessage(messageId, {
    result: {
      ...message.result,
      requiresConfirmation: pendingAction.requiresConfirmation === true,
      pendingAction
    }
  })
}

const getMcpConfirmationError = error => error?.response?.data?.message
  || error?.response?.data?.msg
  || error?.message
  || '操作请求失败'

const isMcpConfirmationExpiredError = error => /过期|expired|失效/.test(getMcpConfirmationError(error))
const isMcpConfirmationInvalidError = error => /invalid|already been used|已使用|无效/.test(getMcpConfirmationError(error))

const cancelMcpAction = async (messageId, action) => {
  if (!action?.confirmationId || mcpConfirmationSubmittingId.value) return
  if (isMcpActionExpired(action)) {
    updateMcpPendingAction(messageId, action, {
      requiresConfirmation: false,
      confirmationStatus: 'expired',
      confirmationError: '确认凭证已过期，请重新生成预览'
    })
    return
  }

  mcpConfirmationSubmittingId.value = action.confirmationId
  mcpConfirmationSubmittingMode.value = 'cancel'
  updateMcpPendingAction(messageId, action, { confirmationError: '' })
  try {
    const data = getPayload(await aiopsApi.cancelMCPAction(action.confirmationId), '取消操作失败')
    updateMcpPendingAction(messageId, action, {
      ...data,
      requiresConfirmation: false,
      confirmationStatus: 'cancelled',
      confirmationError: '',
      message: data?.message || '操作已取消，确认凭证已失效'
    })
    ElMessage.info('操作已取消')
  } catch (error) {
    const expired = isMcpConfirmationExpiredError(error)
    updateMcpPendingAction(messageId, action, {
      requiresConfirmation: !expired,
      confirmationStatus: expired ? 'expired' : 'pending',
      confirmationError: getMcpConfirmationError(error)
    })
  } finally {
    mcpConfirmationSubmittingId.value = ''
    mcpConfirmationSubmittingMode.value = ''
  }
}

const confirmMcpAction = async (messageId, action) => {
  if (!action?.confirmationId || mcpConfirmationSubmittingId.value) return
  if (!canConfirmMcpAction(action)) {
    updateMcpPendingAction(messageId, action, { confirmationError: '当前账号无权执行此操作' })
    return
  }
  if (isMcpActionExpired(action)) {
    updateMcpPendingAction(messageId, action, {
      requiresConfirmation: false,
      confirmationStatus: 'expired',
      confirmationError: '确认凭证已过期，请重新生成预览'
    })
    return
  }

  mcpConfirmationSubmittingId.value = action.confirmationId
  mcpConfirmationSubmittingMode.value = 'confirm'
  updateMcpPendingAction(messageId, action, { confirmationError: '' })
  try {
    const response = isK8sMcpAction(action)
      ? await aiopsApi.confirmMCPK8sAction(action.confirmationId)
      : await aiopsApi.confirmMCPCleanupLogs(action.confirmationId)
    const data = getPayload(response, '变更执行失败')
    const executionFailed = ['failed', 'error'].includes(String(data?.status || '').toLowerCase())
    updateMcpPendingAction(messageId, action, {
      ...data,
      requiresConfirmation: false,
      confirmationStatus: executionFailed ? 'failed' : 'completed',
      confirmationError: '',
      executed: executionFailed ? false : data?.executed !== false,
      executionResult: data,
      message: data?.message || (executionFailed ? '操作执行失败' : '变更已执行')
    })
    if (executionFailed) ElMessage.error(data?.message || '操作执行失败')
    else ElMessage.success(data?.message || '操作已执行')
  } catch (error) {
    const expired = isMcpConfirmationExpiredError(error)
    const invalid = expired || isMcpConfirmationInvalidError(error)
    updateMcpPendingAction(messageId, action, {
      requiresConfirmation: !invalid,
      confirmationStatus: expired ? 'expired' : (invalid ? 'invalid' : 'pending'),
      confirmationError: getMcpConfirmationError(error)
    })
  } finally {
    mcpConfirmationSubmittingId.value = ''
    mcpConfirmationSubmittingMode.value = ''
  }
}

const regenerateMcpPreview = message => {
  const question = String(message?.result?.requestPayload?.question || '').trim()
  if (!question) {
    ElMessage.warning('缺少原始请求，无法重新生成预览')
    return
  }
  sendWithText(question)
}

const extractIpList = value => {
  const matches = String(value || '').match(/\b(?:25[0-5]|2[0-4]\d|1?\d?\d)(?:\.(?:25[0-5]|2[0-4]\d|1?\d?\d)){3}\b/g)
  return Array.from(new Set(matches || []))
}

const toMaybeNumber = value => {
  if (!isFilled(value)) return undefined
  const number = Number(value)
  return Number.isFinite(number) && String(value).trim() !== '' ? number : value
}

const compactObject = object => Object.entries(object).reduce((payload, [key, value]) => {
  if (Array.isArray(value)) {
    if (value.length) payload[key] = value
    return payload
  }
  if (value && typeof value === 'object') {
    if (Object.keys(value).length) payload[key] = value
    return payload
  }
  if (isFilled(value)) payload[key] = value
  return payload
}, {})

const createSessionId = () => `web-chat-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`

const formatTime = (date = new Date()) => {
  const pad = value => String(value).padStart(2, '0')
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

const createSession = (question = '') => {
  const session = {
    id: createSessionId(),
    title: question ? question.slice(0, 24) : '新的会话',
    lastIntentLabel: '待识别',
    updatedAt: formatTime(),
    messages: []
  }
  sessions.value.unshift(session)
  activeSessionId.value = session.id
  selectedAssistantMessageId.value = ''
  insightDrawerVisible.value = false
  if (question) chatForm.question = question
  persistSessions()
}

const switchSession = id => {
  activeSessionId.value = id
  insightDrawerVisible.value = false
  const lastAssistant = [...messages.value].reverse().find(item => item.role === 'assistant' && item.result)
  selectedAssistantMessageId.value = lastAssistant?.id || ''
  nextTick(scrollMessagesToBottom)
}

const ensureSession = () => {
  if (!currentSession.value) createSession()
}

const addMessage = message => {
  ensureSession()
  currentSession.value.messages.push(message)
  currentSession.value.updatedAt = formatTime()
  persistSessions()
  nextTick(scrollMessagesToBottom)
}

const updateMessage = (id, patch) => {
  const message = messages.value.find(item => item.id === id)
  if (!message) return
  Object.assign(message, patch)
  if (currentSession.value) currentSession.value.updatedAt = formatTime()
  persistSessions()
  nextTick(scrollMessagesToBottom)
}

const applyPrompt = question => {
  chatForm.question = question
}

const clearComposer = () => {
  chatForm.question = ''
}

const formatHistoryClearResult = data => {
  const parts = [
    `会话 ${data.deletedSessions || 0}`,
    `消息 ${data.deletedMessages || 0}`,
    `Run ${data.deletedRuns || 0}`,
    `证据 ${data.deletedEvidence || 0}`
  ]
  return parts.join('，')
}

const clearLocalChatHistory = sessionId => {
  if (sessionId) {
    sessions.value = sessions.value.filter(item => item.id !== sessionId)
  } else {
    sessions.value = []
  }
  selectedAssistantMessageId.value = ''
  insightDrawerVisible.value = false
  if (!sessions.value.length) {
    createSession()
  } else {
    activeSessionId.value = sessions.value[0].id
    persistSessions()
  }
  nextTick(scrollMessagesToBottom)
}

const clearChatHistory = async (all = false) => {
  if (historyClearing.value) return
  const sessionId = all ? '' : activeSessionId.value
  const message = all
    ? '确认清理全部通用助手聊天历史？此操作会删除后端历史记录。'
    : '确认清理当前会话历史？此操作会删除后端历史记录。'

  try {
    await ElMessageBox.confirm(message, '清理历史', {
      type: 'warning',
      confirmButtonText: '清理',
      cancelButtonText: '取消',
      distinguishCancelAndClose: true
    })
  } catch {
    return
  }

  historyClearing.value = true
  try {
    const payload = sessionId ? { sessionId } : {}
    const data = getPayload(await aiopsApi.clearAssistantChatHistory(payload), '清理历史失败')
    clearLocalChatHistory(sessionId)
    ElMessage.success(`历史已清理：${formatHistoryClearResult(data)}`)
  } catch (error) {
    ElMessage.error(error.message || '清理历史失败')
  } finally {
    historyClearing.value = false
  }
}

const buildContext = (question = '') => {
  let extra = {}
  if (contextForm.extraJson.trim()) {
    try {
      extra = JSON.parse(contextForm.extraJson)
    } catch (error) {
      ElMessage.warning('扩展 JSON 格式不正确')
      return null
    }
  }

  const hostValue = contextForm.host.trim()
  const alertValue = contextForm.alert.trim()
  const explicitIps = extractIpList(question)
  const explicitIp = explicitIps[0] || ''
  const context = compactObject({
    pageType: contextForm.pageType || 'global_chat',
    pageResourceId: toMaybeNumber(contextForm.pageResourceId),
    channel: 'web',
    ...extra,
    ip: explicitIp || (/^\d{1,3}(?:\.\d{1,3}){3}$/.test(hostValue) ? hostValue : ''),
    explicitIp,
    explicitIps,
    currentQuestionIp: explicitIp,
    targetIp: explicitIp,
    contextPriority: explicitIp ? 'current_question_ip_over_page_context_and_memory' : '',
    hostName: explicitIp ? '' : (hostValue && !/^\d{1,3}(?:\.\d{1,3}){3}$/.test(hostValue) ? hostValue : ''),
    alertId: /^\d+$/.test(alertValue) ? Number(alertValue) : '',
    fingerprint: alertValue && !/^\d+$/.test(alertValue) ? alertValue : ''
  })

  return Object.keys(context).length ? context : { pageType: 'global_chat', channel: 'web' }
}

const buildPayload = (question, contextOverride = null) => {
  const context = buildContext(question)
  if (!context) return null
  const payload = {
    sessionId: activeSessionId.value,
    question,
    mode: chatForm.mode || 'global',
    async: true,
    stream: true,
    responseMode: 'stream',
    context: contextOverride ? compactObject({ ...context, ...contextOverride }) : context
  }
  if (isFilled(chatForm.providerId)) payload.providerId = toMaybeNumber(chatForm.providerId)
  return payload
}

const sendMessage = async (contextOverride = null) => {
  const question = chatForm.question.trim()
  if (!question) {
    ElMessage.warning('请输入问题')
    return
  }

  ensureSession()
  const payload = buildPayload(question, contextOverride)
  if (!payload) return

  closeRunStream()
  streamDebug('send:start', {
    sessionId: payload.sessionId,
    mode: payload.mode,
    responseMode: payload.responseMode,
    async: payload.async,
    stream: payload.stream,
    providerId: payload.providerId || '',
    questionLength: question.length
  })
  chatForm.question = ''
  sendLoading.value = true

  addMessage({
    id: `user-${Date.now()}`,
    role: 'user',
    content: question,
    createdAt: formatTime()
  })

  const assistantId = `assistant-${Date.now()}`
  expandedThinkingMessageIds.value = new Set([...expandedThinkingMessageIds.value, assistantId])
  addMessage({
    id: assistantId,
    role: 'assistant',
    content: '',
    createdAt: formatTime(),
    loading: true,
    streamEvents: []
  })

  try {
    await sendAssistantRequest(payload, assistantId, question)
    streamDebug('send:handled', { assistantId })
  } catch (error) {
    streamWarn('send:error', error)
    console.error('assistant chat failed:', error)
    updateMessage(assistantId, {
      content: '这次请求没有完成。',
      loading: false,
      error: error.message || 'AI 助手请求失败'
    })
    ElMessage.error(error.message || 'AI 助手请求失败')
  } finally {
    sendLoading.value = false
  }
}

const sendWithText = (text, contextOverride = null) => {
  chatForm.question = text
  sendMessage(contextOverride)
}

const sendAssistantRequest = async (payload, assistantId, question) => {
  if (typeof ReadableStream !== 'undefined' && aiopsApi.fetchAssistantChatStream) {
    streamDebug('chat:fetch-stream:start', { assistantId, url: '/api/v1/aiops/assistant/chat' })
    const handled = await sendAssistantStreamingRequest(payload, assistantId, question)
    streamDebug('chat:fetch-stream:handled', { assistantId, handled })
    if (handled) return
  }

  streamDebug('chat:axios:start', { assistantId })
  const response = await aiopsApi.chatAssistant(payload)
  const data = normalizeAssistantResult(getPayload(response, 'AI 助手请求失败'), payload)
  streamDebug('chat:axios:result', {
    assistantId,
    runId: data.runId,
    status: data.status || data.runStatus || data.run?.status || '',
    answerLength: String(data.answer || '').length,
    shouldAutoSubscribe: shouldAutoSubscribeRun(data)
  })
  applyAssistantInitialResult(data, assistantId, question)
}

const sendAssistantStreamingRequest = async (payload, assistantId, question) => {
  const startedAt = Date.now()
  const headerTimer = window.setTimeout(() => {
    streamWarn('chat:fetch-stream:waiting-headers', {
      assistantId,
      elapsedMs: Date.now() - startedAt,
      hint: '如果这里一直停住，通常是后端 /assistant/chat 没有先返回响应头，前端无法开始读流。'
    })
  }, 10000)
  const response = await aiopsApi.fetchAssistantChatStream(payload)
  window.clearTimeout(headerTimer)
  streamDebug('chat:fetch-stream:headers', {
    assistantId,
    elapsedMs: Date.now() - startedAt,
    ok: response.ok,
    status: response.status,
    contentType: response.headers.get('content-type') || '',
    transferEncoding: response.headers.get('transfer-encoding') || '',
    cacheControl: response.headers.get('cache-control') || '',
    xAccelBuffering: response.headers.get('x-accel-buffering') || ''
  })
  if (!response.ok) {
    throw new Error(await getFetchErrorMessage(response))
  }

  const contentType = String(response.headers.get('content-type') || '').toLowerCase()
  updateMessage(assistantId, {
    content: '响应已建立，等待首个数据块...',
    loading: true
  })
  if (!response.body) {
    streamWarn('chat:fetch-stream:no-body', { assistantId, contentType })
    const body = await readFetchBody(response)
    const data = normalizeAssistantResult(getPayload({ data: body }, 'AI 助手请求失败'), payload)
    streamDebug('chat:fetch-stream:no-body-result', {
      assistantId,
      runId: data.runId,
      status: data.status || data.runStatus || data.run?.status || '',
      answerLength: String(data.answer || '').length
    })
    applyAssistantInitialResult(data, assistantId, question)
    return true
  }
  if (isBufferedJsonResponse(contentType)) {
    streamWarn('chat:fetch-stream:buffered-json', { assistantId, contentType })
    updateMessage(assistantId, {
      content: '后端返回普通 JSON，正在等待接口执行完成...',
      loading: true
    })
    const bufferedJsonTimer = window.setTimeout(() => {
      streamWarn('chat:fetch-stream:waiting-json-body', {
        assistantId,
        contentType,
        hint: '后端返回 application/json，前端必须等待完整响应体结束，无法逐字打印。若这里等待很久，说明后端仍在同步执行。'
      })
    }, 10000)
    const body = await readFetchBody(response)
    window.clearTimeout(bufferedJsonTimer)
    const data = normalizeAssistantResult(getPayload({ data: body }, 'AI 助手请求失败'), payload)
    streamDebug('chat:fetch-stream:buffered-json-result', {
      assistantId,
      runId: data.runId,
      status: data.status || data.runStatus || data.run?.status || '',
      answerLength: String(data.answer || '').length,
      shouldAutoSubscribe: shouldAutoSubscribeRun(data)
    })
    applyAssistantInitialResult(data, assistantId, question)
    return true
  }

  sendLoading.value = false
  updateMessage(assistantId, {
    content: '',
    loading: true
  })
  await consumeAssistantResponseStream(response.body, contentType, payload, assistantId, question)
  streamDebug('chat:fetch-stream:done', { assistantId })
  continueRunStreamIfNeeded(assistantId)
  return true
}

const applyAssistantInitialResult = (data, assistantId, question) => {
  selectedAssistantMessageId.value = assistantId
  updateSessionSummary(question, data)
  streamDebug('assistant:initial-result', {
    assistantId,
    runId: data.runId,
    status: data.status || data.runStatus || data.run?.status || '',
    answerLength: String(data.answer || '').length,
    timelineCount: data.timeline?.length || 0,
    usedToolsCount: data.usedTools?.length || 0,
    shouldAutoSubscribe: shouldAutoSubscribeRun(data)
  })

  if (shouldAutoSubscribeRun(data)) {
    updateMessage(assistantId, {
      content: '',
      loading: true,
      result: data
    })
    subscribeRunStream(data.runId, assistantId, { auto: true })
    return
  }

  updateMessage(assistantId, {
    content: getAssistantAnswerText(data),
    loading: false,
    result: data
  })
}

const isBufferedJsonResponse = contentType =>
  contentType.includes('application/json')
  && !contentType.includes('stream')
  && !contentType.includes('ndjson')
  && !contentType.includes('jsonl')

const readFetchBody = async response => {
  const text = await response.text()
  streamDebug('chat:read-body', {
    status: response.status,
    contentType: response.headers?.get?.('content-type') || '',
    chars: text.length,
    preview: text.slice(0, 240)
  })
  if (!text) return {}
  try {
    return JSON.parse(text)
  } catch {
    return { answer: text }
  }
}

const getFetchErrorMessage = async response => {
  try {
    const body = await readFetchBody(response)
    return body?.message || body?.msg || body?.error || `AI 助手请求失败(${response.status})`
  } catch {
    return `AI 助手请求失败(${response.status})`
  }
}

const consumeAssistantResponseStream = async (body, contentType, payload, assistantId, question) => {
  const reader = body.getReader()
  const decoder = new TextDecoder('utf-8')
  const state = createAssistantStreamParseState(contentType)
  let chunkCount = 0
  let eventCount = 0
  let firstChunkReceived = false
  let firstChunkTimer = window.setTimeout(() => {
    if (firstChunkReceived) return
    streamWarn('chat:body-stream:waiting-first-chunk', {
      assistantId,
      contentType,
      hint: '响应头已返回，但 10 秒没有收到任何 chunk。常见原因是后端/网关缓冲，或后端没有 flush。'
    })
  }, 10000)
  streamDebug('chat:body-stream:start', { assistantId, contentType, mode: state.mode || 'auto' })

  try {
    let reading = true
    while (reading) {
      const { value, done } = await reader.read()
      if (done) {
        streamDebug('chat:body-stream:reader-done', { assistantId, chunkCount, eventCount })
        reading = false
        break
      }

      const chunk = decoder.decode(value, { stream: true })
      if (!firstChunkReceived) {
        firstChunkReceived = true
        window.clearTimeout(firstChunkTimer)
      }
      chunkCount += 1
      streamDebug('chat:body-stream:chunk', {
        assistantId,
        chunkCount,
        bytes: value?.byteLength || 0,
        chars: chunk.length,
        mode: state.mode || 'auto',
        preview: chunk.slice(0, 240)
      })
      const events = extractAssistantStreamEvents(chunk, state)
      eventCount += events.length
      streamDebug('chat:body-stream:events', {
        assistantId,
        chunkCount,
        events: events.map(item => ({
          type: item.type,
          keys: item.data && typeof item.data === 'object' ? Object.keys(item.data).slice(0, 12) : [],
          preview: typeof item.data === 'string' ? item.data.slice(0, 120) : ''
        }))
      })
      for (const event of events) {
        await dispatchAssistantStreamPacket(event.type, event.data, payload, assistantId, question)
      }
    }

    const tail = decoder.decode()
    const events = extractAssistantStreamEvents(tail, state, true)
    eventCount += events.length
    streamDebug('chat:body-stream:flush', {
      assistantId,
      tailChars: tail.length,
      events: events.map(item => item.type),
      totalChunks: chunkCount,
      totalEvents: eventCount,
      finalMode: state.mode
    })
    for (const event of events) {
      await dispatchAssistantStreamPacket(event.type, event.data, payload, assistantId, question)
    }
  } finally {
    window.clearTimeout(firstChunkTimer)
    streamDebug('chat:body-stream:release', { assistantId, chunkCount, eventCount, mode: state.mode })
    reader.releaseLock?.()
  }
}

const createAssistantStreamParseState = contentType => ({
  mode: detectAssistantStreamMode(contentType),
  contentType,
  sseBuffer: '',
  sseEvent: 'message',
  sseDataLines: [],
  jsonBuffer: ''
})

const detectAssistantStreamMode = (contentType, chunk = '') => {
  const type = String(contentType || '').toLowerCase()
  if (type.includes('event-stream')) return 'sse'
  if (type.includes('ndjson') || type.includes('jsonl')) return 'ndjson'

  const text = String(chunk || '').trimStart()
  if (text.startsWith('event:') || text.startsWith('data:') || text.includes('\ndata:')) return 'sse'
  if (text.startsWith('{') || text.startsWith('[')) return 'ndjson'
  return ''
}

const extractAssistantStreamEvents = (chunk, state, flush = false) => {
  if (!state.mode && chunk) {
    state.mode = detectAssistantStreamMode(state.contentType, chunk) || 'raw'
    streamDebug('chat:body-stream:mode-detected', {
      contentType: state.contentType,
      mode: state.mode,
      preview: String(chunk).slice(0, 160)
    })
  }
  if (state.mode === 'sse') return extractSseEvents(chunk, state, flush)
  if (state.mode === 'ndjson') return extractLineJsonEvents(chunk, state, flush)
  if (!chunk) return []
  return [{ type: 'delta', data: { delta: chunk } }]
}

const extractSseEvents = (chunk, state, flush = false) => {
  const events = []
  state.sseBuffer += chunk || ''
  const lines = state.sseBuffer.split('\n')
  state.sseBuffer = flush ? '' : (lines.pop() || '')
  const readyLines = flush ? lines : lines.slice(0)
  if (flush && state.sseBuffer) readyLines.push(state.sseBuffer)

  readyLines.forEach(rawLine => {
    const line = rawLine.replace(/\r$/, '')
    if (!line) {
      flushSseEvent(state, events)
      return
    }
    if (line.startsWith(':')) return
    if (line.startsWith('event:')) {
      state.sseEvent = line.slice(6).trim() || 'message'
      return
    }
    if (line.startsWith('data:')) {
      state.sseDataLines.push(line.slice(5).replace(/^ /, ''))
      return
    }
    if (line.startsWith('id:') || line.startsWith('retry:')) return
    state.sseDataLines.push(line)
  })

  if (flush) flushSseEvent(state, events)
  return events
}

const flushSseEvent = (state, events) => {
  if (!state.sseDataLines.length) {
    state.sseEvent = 'message'
    return
  }
  events.push({
    type: state.sseEvent || 'message',
    data: parseStreamPayload(state.sseDataLines.join('\n'))
  })
  state.sseEvent = 'message'
  state.sseDataLines = []
}

const extractLineJsonEvents = (chunk, state, flush = false) => {
  const events = []
  state.jsonBuffer += chunk || ''
  let parsed
  do {
    parsed = shiftJsonPayload(state.jsonBuffer)
    if (parsed) {
      streamDebug('chat:body-stream:json-object', {
        type: parsed.value?.type || parsed.value?.event || '',
        status: parsed.value?.status || '',
        runId: parsed.value?.runId || parsed.value?.run_id || '',
        restChars: parsed.rest.length
      })
      events.push({ type: 'message', data: parsed.value })
      state.jsonBuffer = parsed.rest
    }
  } while (parsed)

  if (flush) {
    const rest = state.jsonBuffer.trim()
    if (rest) {
      streamWarn('chat:body-stream:json-tail-unparsed', {
        chars: rest.length,
        preview: rest.slice(0, 240)
      })
      events.push({ type: 'message', data: parseStreamPayload(rest) })
    }
    state.jsonBuffer = ''
  }

  return events
}

const shiftJsonPayload = value => {
  const source = String(value || '').trimStart()
  if (!source) return null

  const startChar = source[0]
  if (startChar !== '{' && startChar !== '[') {
    const newlineIndex = source.search(/\r?\n/)
    if (newlineIndex === -1) return null
    const line = source.slice(0, newlineIndex).trim()
    const rest = source.slice(newlineIndex).trimStart()
    return line ? { value: parseStreamPayload(line), rest } : { value: null, rest }
  }

  const closeChar = startChar === '{' ? '}' : ']'
  let depth = 0
  let inString = false
  let escaped = false

  for (let index = 0; index < source.length; index += 1) {
    const char = source[index]
    if (inString) {
      if (escaped) {
        escaped = false
      } else if (char === '\\') {
        escaped = true
      } else if (char === '"') {
        inString = false
      }
      continue
    }

    if (char === '"') {
      inString = true
      continue
    }
    if (char === startChar) depth += 1
    if (char === closeChar) depth -= 1

    if (depth === 0) {
      const jsonText = source.slice(0, index + 1)
      const rest = source.slice(index + 1).trimStart()
      return {
        value: parseStreamPayload(jsonText),
        rest
      }
    }
  }

  return null
}

const parseStreamPayload = value => {
  const text = String(value ?? '')
  if (!text || text === '[DONE]' || text.toLowerCase() === 'done') return { status: 'done' }
  try {
    return JSON.parse(text)
  } catch {
    return { message: text }
  }
}

const dispatchAssistantStreamPacket = async (rawType, rawData, payload, assistantId, question) => {
  const unwrapped = unwrapApiStreamPacket(rawData)
  const { type, data } = normalizeStreamEvent(rawType, unwrapped)
  const eventData = data && typeof data === 'object' && !Array.isArray(data) ? data : { value: data }
  const normalizedEvent = { type, ...eventData, receivedAt: formatTime() }
  const message = messages.value.find(item => item.id === assistantId)
  streamDebug('chat:packet', {
    assistantId,
    rawType,
    type,
    keys: Object.keys(eventData).slice(0, 16),
    runId: getRunId(eventData),
    status: eventData.status || eventData.runStatus || eventData.run_status || '',
    textLength: String(getStreamTextSegment(eventData, type) || '').length,
    isTerminal: isTerminalStreamEvent(type, eventData)
  })

  mergeAssistantStreamResult(assistantId, eventData, payload, question, type)

  if (isStreamTextEvent(type, eventData)) {
    appendStreamEvent(assistantId, normalizedEvent)
    appendStreamText(assistantId, getStreamTextSegment(eventData, type))
    return
  }

  appendStreamEvent(assistantId, normalizedEvent)

  if (type === 'result') {
    finalizeAssistantResultEvent(assistantId, payload, question, eventData)
    return
  }

  if (isTerminalStreamEvent(type, eventData)) {
    if (type === 'done') {
      finalizeAssistantDone(assistantId, payload, question, eventData)
      return
    }
    const runId = getRunId(eventData) || getRunId(message?.result)
    if (runId) {
      await finishRunFromStream(runId, assistantId, eventData)
      return
    }
    finalizeAssistantStreamText(assistantId, payload, question, eventData)
    return
  }

  const finalCandidate = normalizeAssistantResult({
    ...message?.result,
    ...unwrapStreamResult(eventData)
  }, message?.result?.requestPayload || payload)
  if (isAssistantFinalPayload(eventData) && !shouldAutoSubscribeRun(finalCandidate)) {
    if (progressStreamEventTypes.has(String(type).toLowerCase())) return
    finalizeAssistantStreamText(assistantId, payload, question, eventData)
    return
  }
}

const unwrapApiStreamPacket = data => {
  if (!data || typeof data !== 'object' || Array.isArray(data)) return data
  if (data.code !== undefined) {
    if (!successCodes.has(Number(data.code))) {
      throw new Error(data.message || data.msg || data.error || 'AI 助手请求失败')
    }
    return data.data !== undefined ? data.data : data
  }
  return data
}

const mergeAssistantStreamResult = (messageId, eventData, payload, question, type = '') => {
  const message = messages.value.find(item => item.id === messageId)
  const patch = getAssistantStreamResultPatch(eventData, type, message?.result)
  if (!Object.keys(patch).length) return
  streamDebug('chat:merge-result', {
    messageId,
    type,
    patch,
    eventKeys: Object.keys(eventData || {}).slice(0, 16)
  })

  if (!message) return

  const merged = normalizeAssistantResult({
    ...message.result,
    ...patch
  }, message.result?.requestPayload || payload)

  selectedAssistantMessageId.value = messageId
  updateSessionSummary(question, merged)
  const eventType = String(type || '').toLowerCase()
  const finalAnswer = getAssistantAnswerText(merged)
  const hasFinalAnswer = Boolean(
    !progressStreamEventTypes.has(eventType)
    && finalAnswer
    && !isProgressAnswerText(finalAnswer)
  )
  updateMessage(messageId, {
    result: merged,
    content: hasFinalAnswer
      ? finalAnswer
      : (message.streamAnswer || ''),
    loading: !terminalRunStatuses.has(String(merged.status || merged.runStatus || merged.run?.status || '').toLowerCase())
  })
}

const getAssistantStreamResultPatch = (data, type = '', currentResult = {}) => {
  if (!data || typeof data !== 'object' || Array.isArray(data)) return {}
  const patch = { ...unwrapStreamResult(data) }
  const runId = getRunId(data) || (String(type).toLowerCase() === 'run' ? data.id : '')
  if (runId) patch.runId = runId
  const status = data.status || data.runStatus || data.run_status || data.run?.status
  if (status) patch.status = status
  if (data.intent || data.taskIntent) patch.intent = data.intent || data.taskIntent
  if (data.type) patch.streamType = data.type
  if (data.readonly !== undefined) patch.readonly = data.readonly
  const progress = getRawAssistantProgress(data)
  if (progress) patch.progress = progress
  const thinkingProcess = normalizeThinkingProcess(
    data.thinkingProcess
    || data.thinking_process
    || data.result?.thinkingProcess
    || data.result?.thinking_process
  )
  if (thinkingProcess) patch.thinkingProcess = thinkingProcess
  const analysisTask = normalizeAnalysisTask(
    data.analysisTask
    || data.analysis_task
    || data.result?.analysisTask
    || data.result?.analysis_task
  )
  if (analysisTask) patch.analysisTask = analysisTask
  const eventType = String(type || data.type || data.event || '').toLowerCase()
  if (eventType === 'analysis_progress') {
    const detail = data.event || data.result?.event
    if (detail) patch.analysisProgress = mergeAnalysisStepIntoProgress(currentResult?.analysisProgress, detail)
  }
  return compactObject(patch)
}

const isAssistantAnswerPlaceholder = value => /^(?:assistant\s+(?:result|response)|result|response|stream\s+completed)$/i.test(String(value || '').trim())

const getDisplayableAssistantAnswer = (...values) => values.find(value => {
  if (typeof value !== 'string' && typeof value !== 'number') return false
  const text = String(value).trim()
  return Boolean(text) && !isProgressAnswerText(text) && !isAssistantAnswerPlaceholder(text)
}) || ''

const getAnswerFields = data => [
  data?.answer,
  data?.content,
  data?.text,
  data?.finalAnswer,
  data?.final_answer,
  data?.response,
  data?.summary
]

const getFinalAnswerFromPayload = data => {
  if (!data || typeof data !== 'object') return ''

  // SSE event messages (for example "assistant result") describe the event only.
  // The actual answer may be nested in the final result payload.
  const nestedContainers = [data.result, data.output, data.payload, data.data, data.run]
  return getDisplayableAssistantAnswer(
    ...getAnswerFields(data),
    ...nestedContainers.flatMap(getAnswerFields),
    data.diagnosisCard?.summary,
    data.diagnosis_card?.summary
  )
}

const isAssistantFinalPayload = data => {
  if (!data || typeof data !== 'object' || Array.isArray(data)) return false
  if (getFinalAnswerFromPayload(data)) return true
  if (data.diagnosisCard || data.diagnosis_card || data.clarifyQuestions || data.clarify_questions) return true
  if (data.result || data.output || data.payload) return true
  return false
}

const getRunId = data => data?.runId || data?.run_id || data?.agentRunId || data?.agent_run_id || data?.run?.id || ''

const finalizeAssistantStreamText = (messageId, payload, question, eventData = {}) => {
  const message = messages.value.find(item => item.id === messageId)
  if (!message) return

  const finalPayload = unwrapStreamResult(eventData)
  const result = normalizeAssistantResult({
    ...message.result,
    ...finalPayload
  }, message.result?.requestPayload || payload)
  const finalAnswer = getAssistantAnswerText(result)
  const streamText = message.streamAnswer || ''
  const content = streamText && finalAnswer && !streamText.includes(finalAnswer)
    ? `${streamText}\n\n${finalAnswer}`
    : (finalAnswer || streamText || getStreamTextSegment(eventData) || message.content)

  updateMessage(messageId, {
    content,
    loading: false,
    result
  })
  updateSessionSummary(question, result)
}

const finalizeAssistantResultEvent = (messageId, payload, question, eventData = {}) => {
  const message = messages.value.find(item => item.id === messageId)
  if (!message) return

  const finalPayload = unwrapStreamResult(eventData)
  const explicitAnswer = getFinalAnswerFromPayload(eventData) || getFinalAnswerFromPayload(finalPayload)
  const result = normalizeAssistantResult({
    ...message.result,
    ...finalPayload,
    answer: explicitAnswer || finalPayload.answer || message.result?.answer || ''
  }, message.result?.requestPayload || payload)
  const finalAnswer = explicitAnswer || getAssistantAnswerText(result)

  streamDebug('chat:result-finalize', {
    messageId,
    eventKeys: Object.keys(eventData || {}).slice(0, 16),
    finalPayloadKeys: Object.keys(finalPayload || {}).slice(0, 16),
    answerLength: String(finalAnswer || '').length,
    previousContentLength: String(message.content || '').length
  })

  updateMessage(messageId, {
    content: finalAnswer || message.content || message.streamAnswer || '后端已返回结果，请查看结构化结果。',
    loading: false,
    result
  })
  updateSessionSummary(question, result)
}

const finalizeAssistantDone = (messageId, payload, question, eventData = {}) => {
  const message = messages.value.find(item => item.id === messageId)
  if (!message) return

  const result = normalizeAssistantResult({
    ...message.result,
    ...getAssistantStreamResultPatch(eventData, 'done'),
    status: eventData.status || 'done'
  }, message.result?.requestPayload || payload)
  const finalAnswer = getAssistantAnswerText(result)

  updateMessage(messageId, {
    content: finalAnswer || message.content || message.streamAnswer || '后端已返回结果，请查看结构化结果。',
    loading: false,
    result
  })
  updateSessionSummary(question, result)
}

const continueRunStreamIfNeeded = messageId => {
  const message = messages.value.find(item => item.id === messageId)
  if (!message?.loading) {
    streamDebug('sse:continue:skip-not-loading', { messageId })
    return
  }

  const status = String(message.result?.status || message.result?.runStatus || message.result?.run?.status || '').toLowerCase()
  if (terminalRunStatuses.has(status)) {
    streamDebug('sse:continue:skip-terminal', { messageId, status })
    updateMessage(messageId, { loading: false })
    return
  }

  const runId = getRunId(message.result)
  if (runId) {
    streamDebug('sse:continue:subscribe', { messageId, runId, status })
    subscribeRunStream(runId, messageId, { auto: true, keepExistingText: true })
    return
  }

  if (message.streamAnswer || message.content) {
    streamWarn('sse:continue:no-run-id', {
      messageId,
      status,
      contentLength: String(message.streamAnswer || message.content || '').length
    })
    finalizeAssistantStreamText(messageId, message.result?.requestPayload || {}, message.result?.requestPayload?.question || currentSession.value?.title || '')
  }
}

const normalizeAssistantResult = (payload, requestPayload) => {
  const data = typeof payload === 'object' && payload !== null && !Array.isArray(payload)
    ? { ...payload }
    : { answer: String(payload ?? '') }

  data.intent = data.intent || data.taskIntent || 'unknown'
  data.answer = getFinalAnswerFromPayload(data) || getFirstClarifyQuestion(data) || ''
  if ((!data.intent || data.intent === 'unknown') && data.answer) {
    const parsed = parseAnswerDiagnosis(data.answer)
    if (parsed.intentText) data.intent = parsed.intentText
  }
  data.runId = data.runId || data.run_id || data.agentRunId || data.run?.id || ''
  data.entities = data.entities || data.entity || {}
  data.memory = data.memory || {}
  data.toolPlan = normalizeList(data.toolPlan || data.tool_plan)
  data.timeline = normalizeList(data.timeline || data.steps || data.run?.steps)
  data.usedTools = normalizeList(data.usedTools || data.used_tools || data.tools)
  data.evidence = normalizeList(data.evidence || data.evidences)
  data.clarifyQuestions = normalizeList(data.clarifyQuestions || data.clarify_questions)
  data.diagnosisCard = data.diagnosisCard || data.diagnosis_card || data.card || null
  data.assistantPlan = normalizeList(data.assistantPlan || data.assistant_plan || data.plan || data.planner)
  data.thinkingProcess = normalizeThinkingProcess(
    data.thinkingProcess
    || data.thinking_process
    || data.result?.thinkingProcess
    || data.result?.thinking_process
  )
  data.progress = normalizeAssistantProgress(getRawAssistantProgress(data))
  data.analysisProgress = normalizeAssistantProgress(data.analysisProgress || data.analysis_progress)
  data.analysisTask = normalizeAnalysisTask(
    data.analysisTask
    || data.analysis_task
    || data.result?.analysisTask
    || data.result?.analysis_task
  )
  if (!data.progress && data.analysisTask) {
    data.progress = normalizeAssistantProgress({
      percent: data.analysisTask.progress,
      status: data.analysisTask.status,
      currentStep: data.analysisTask.currentStep,
      steps: []
    })
  }
  data.reflection = data.reflection || data.reflector || data.selfReflection || data.self_reflection || null
  data.missingEvidence = normalizeList(data.missingEvidence || data.missing_evidence || data.evidenceGap || data.evidence_gap)
  data.evidenceCompleteness = data.evidenceCompleteness || data.evidence_completeness || data.reflection?.evidenceCompleteness || data.reflection?.evidence_completeness || ''
  data.confidence = data.confidence || data.confidenceLevel || data.confidence_level || data.reflection?.confidence || data.diagnosisCard?.confidence || ''
  data.workloadStatus = normalizeWorkloadStatus(data.workloadStatus || data.workload_status)
  const pendingAction = normalizeMcpPendingAction(data)
  if (pendingAction) {
    data.pendingAction = pendingAction
    data.requiresConfirmation = pendingAction.requiresConfirmation === true
  } else {
    data.requiresConfirmation = data.requiresConfirmation === true || data.requires_confirmation === true
  }
  data.requestPayload = requestPayload
  return data
}

const getAssistantAnswerText = data =>
  getFinalAnswerFromPayload(data)
  || getDisplayableAssistantAnswer(data?.answer)
  || getFirstClarifyQuestion(data)
  || data?.diagnosisCard?.summary
  || data?.error
  || getDisplayableAssistantAnswer(data?.message)
  || '后端已返回结果，请查看结构化结果。'

const shouldAutoSubscribeRun = data => {
  if (!data?.runId) return false
  if (getFirstClarifyQuestion(data) || data.diagnosisCard) return false

  const status = String(data.status || data.runStatus || data.run?.status || '').toLowerCase()
  if (runningRunStatuses.has(status)) return true
  if (terminalRunStatuses.has(status)) return false

  if (data.answer) return isProgressAnswerText(data.answer)

  return !data.timeline.length && !data.usedTools.length && !data.evidence.length
}

const isProgressAnswerText = value => {
  const text = String(value || '').trim()
  return Boolean(text) && progressAnswerTexts.some(keyword => text.includes(keyword))
}

const getFirstClarifyQuestion = result => {
  const question = normalizeList(result?.clarifyQuestions || result?.clarify_questions)[0]
  if (!question) return ''
  return typeof question === 'string' ? question : question.question || ''
}

const getClarifySuggestions = result => normalizeList(result?.clarifyQuestions || result?.clarify_questions)
  .flatMap(item => normalizeList(item?.suggestions))
  .map(formatDisplay)
  .filter(Boolean)
  .slice(0, 6)

const getRawAssistantProgress = data => {
  if (!data || typeof data !== 'object' || Array.isArray(data)) return null
  return data.progress
    || data.assistantProgress
    || data.assistant_progress
    || data.result?.progress
    || data.output?.progress
    || data.payload?.progress
    || data.data?.progress
    || null
}

const normalizeProgressStatus = (status, fallback = 'pending') => {
  const value = String(status || fallback || 'pending').toLowerCase()
  if (['success', 'completed', 'complete', 'done'].includes(value)) return 'succeeded'
  if (['error', 'exception'].includes(value)) return 'failed'
  if (['skip', 'skipped'].includes(value)) return 'skipped'
  if (['processing', 'in_progress', 'progressing', 'started'].includes(value)) return 'running'
  if (['queued', 'waiting'].includes(value)) return 'pending'
  if (['running', 'pending', 'succeeded', 'failed'].includes(value)) return value
  return fallback
}

const normalizeAnalysisTask = task => {
  if (!task || typeof task !== 'object' || Array.isArray(task)) return null
  return {
    taskId: task.taskId || task.task_id || '',
    progress: toPercent(task.progress),
    status: normalizeProgressStatus(task.status, 'running'),
    currentStep: task.currentStep || task.current_step || '',
    message: task.message || '',
    resultUrl: task.resultUrl || task.result_url || '',
    streamUrl: task.streamUrl || task.stream_url || ''
  }
}

const normalizeThinkingProcess = process => {
  if (!process || typeof process !== 'object' || Array.isArray(process)) return null
  const items = normalizeList(process.items).map((item, index) => ({
    key: item?.key || item?.id || `thinking-step-${index + 1}`,
    name: item?.name || item?.title || `步骤 ${index + 1}`,
    description: item?.description || item?.message || '',
    status: normalizeProgressStatus(item?.status, 'pending'),
    time: item?.time || '',
    startedAt: item?.startedAt || item?.started_at || '',
    finishedAt: item?.finishedAt || item?.finished_at || '',
    durationMs: Number(item?.durationMs ?? item?.duration_ms) || 0,
    toolName: item?.toolName || item?.tool_name || '',
    error: item?.error || ''
  }))
  return {
    status: normalizeProgressStatus(process.status, 'running'),
    startedAt: process.startedAt || process.started_at || '',
    finishedAt: process.finishedAt || process.finished_at || '',
    durationMs: Number(process.durationMs ?? process.duration_ms) || 0,
    toolCallCount: Number(process.toolCallCount ?? process.tool_call_count) || 0,
    completedSteps: Number(process.completedSteps ?? process.completed_steps) || 0,
    totalSteps: Number(process.totalSteps ?? process.total_steps) || items.length,
    items
  }
}

const toPercent = value => {
  const number = Number(value)
  if (!Number.isFinite(number)) return 0
  return Math.max(0, Math.min(100, Math.round(number)))
}

const normalizeAssistantProgress = progress => {
  if (!progress || typeof progress !== 'object' || Array.isArray(progress)) return null
  const steps = normalizeList(progress.steps || progress.items || progress.timeline).map((item, index) => {
    const source = item && typeof item === 'object' && !Array.isArray(item) ? item : { name: formatDisplay(item) }
    const status = normalizeProgressStatus(source.status, 'pending')
    return {
      step: source.step || source.index || index + 1,
      key: source.key || source.id || source.name || source.toolName || source.tool_name || `step-${index + 1}`,
      name: source.name || source.stepName || source.step_name || source.title || source.toolName || source.tool_name || `步骤 ${index + 1}`,
      toolName: source.toolName || source.tool_name || '',
      purpose: source.purpose || source.description || source.summary || source.message || '',
      status,
      percent: toPercent(source.percent ?? source.progress ?? (status === 'succeeded' ? 100 : 0)),
      startedAt: source.startedAt || source.started_at || source.startTime || source.start_time || '',
      finishedAt: source.finishedAt || source.finished_at || source.endTime || source.end_time || '',
      durationMs: Number(source.durationMs || source.duration_ms || source.elapsedMs || source.elapsed_ms || 0) || 0,
      error: source.error || source.reason || source.messageError || source.message_error || ''
    }
  })
  const status = normalizeProgressStatus(progress.status, steps.some(item => item.status === 'failed') ? 'failed' : 'running')
  const percent = toPercent(progress.percent ?? progress.progress ?? (steps.length ? Math.round(steps.reduce((sum, item) => sum + item.percent, 0) / steps.length) : 0))
  const runningIndex = steps.findIndex(item => item.status === 'running')
  const currentStep = progress.currentStep || progress.current_step || progress.step || (runningIndex >= 0 ? steps[runningIndex].step : 0)
  return { percent, currentStep, status, steps }
}

const mergeAnalysisStepIntoProgress = (currentProgress, detail) => {
  const base = normalizeAssistantProgress(currentProgress || { steps: [] }) || { percent: 0, currentStep: '', status: 'running', steps: [] }
  const key = detail.step || detail.key || detail.id || `analysis-step-${base.steps.length + 1}`
  const status = normalizeProgressStatus(detail.stepStatus || detail.status, 'running')
  const nextStep = {
    key,
    step: key,
    name: detail.name || detail.stepName || getAnalysisStepLabel(key),
    purpose: detail.message || detail.purpose || '',
    status,
    percent: toPercent(detail.progress ?? (status === 'succeeded' ? 100 : 0)),
    durationMs: Number(detail.durationMs || detail.duration_ms || 0) || 0,
    error: detail.error || detail.reason || ''
  }
  const index = base.steps.findIndex(item => item.key === key || item.step === key)
  const steps = [...base.steps]
  if (index >= 0) steps[index] = { ...steps[index], ...nextStep }
  else steps.push(nextStep)
  return normalizeAssistantProgress({
    ...base,
    steps,
    currentStep: key,
    status: status === 'failed' ? 'failed' : base.status
  })
}

const getAssistantProgress = result => result?.progress || normalizeAssistantProgress(getRawAssistantProgress(result))

const getAnalysisStepLabel = step => analysisStepDefinitions.find(item => item.key === String(step || ''))?.title || String(step || '') || '分析中'

const hasAnalysisTaskFlow = result => {
  if (result?.analysisTask) return true
  return normalizeList(result?.analysisProgress?.steps).some(step => analysisStepKeys.has(step.key || step.step))
}

const getAnalysisTaskPercent = result => {
  const taskPercent = Number(result?.analysisTask?.progress)
  if (Number.isFinite(taskPercent)) return toPercent(taskPercent)
  return getAssistantProgress(result)?.percent || 0
}

const getAnalysisTaskMessage = result => result?.analysisTask?.message
  || getAnalysisStepLabel(result?.analysisTask?.currentStep)
  || '正在等待诊断进度'

const getAnalysisTaskFlow = result => {
  const task = result?.analysisTask || {}
  const progress = result?.analysisProgress || getAssistantProgress(result) || { steps: [] }
  const receivedSteps = normalizeList(progress.steps)
  const currentKey = task.currentStep || progress.currentStep || ''
  const currentIndex = analysisStepDefinitions.findIndex(item => item.key === currentKey)
  const taskStatus = normalizeProgressStatus(task.status || progress.status, 'running')

  return analysisStepDefinitions.map((definition, index) => {
    const received = receivedSteps.find(step => step.key === definition.key || step.step === definition.key)
    let status = normalizeProgressStatus(received?.status, 'pending')
    if (!received) {
      if (taskStatus === 'succeeded') status = 'succeeded'
      else if (index < currentIndex) status = 'succeeded'
      else if (index === currentIndex) status = taskStatus === 'failed' ? 'failed' : 'running'
      else status = 'pending'
    }
    return { ...definition, status }
  })
}

const getThinkingProcess = message => message?.result?.thinkingProcess || null

const getThinkingStatus = message => {
  const rawStatus = message?.result?.analysisTask?.status
    || getThinkingProcess(message)?.status
    || getAssistantProgress(message?.result)?.status
  const fallback = message?.error ? 'failed' : (message?.loading ? 'running' : 'succeeded')
  const status = normalizeProgressStatus(rawStatus, fallback)
  return status === 'pending' && message?.loading ? 'running' : status
}

const formatThinkingTime = value => {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  const pad = number => String(number).padStart(2, '0')
  return `${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

const getThinkingSteps = message => {
  const processItems = normalizeList(getThinkingProcess(message)?.items)
  if (processItems.length) {
    return processItems.map(item => ({
      key: item.key,
      title: item.name,
      message: item.description,
      status: item.status,
      toolName: item.toolName,
      error: item.error,
      meta: item.time || [
        [formatThinkingTime(item.startedAt), formatThinkingTime(item.finishedAt)].filter(Boolean).join(' - '),
        formatDurationMs(item.durationMs)
      ].filter(Boolean).join(' / ')
    }))
  }
  return normalizeList(getAssistantProgress(message?.result)?.steps).map((step, index) => ({
    key: getProgressStepKey(step, index),
    title: step.name,
    message: step.purpose,
    status: step.status,
    toolName: step.toolName,
    error: step.error,
    meta: formatDurationMs(step.durationMs)
  }))
}

const getThinkingTotalSteps = message => getThinkingProcess(message)?.totalSteps || getThinkingSteps(message).length

const getThinkingCompletedSteps = message => {
  const process = getThinkingProcess(message)
  if (process) return process.completedSteps
  return getThinkingSteps(message).filter(step => ['succeeded', 'skipped'].includes(step.status)).length
}

const getThinkingPercent = message => {
  const total = getThinkingTotalSteps(message)
  if (total) return toPercent(getThinkingCompletedSteps(message) * 100 / total)
  return getAssistantProgress(message?.result)?.percent || 0
}

const getThinkingSummary = message => {
  const process = getThinkingProcess(message)
  const progress = getAssistantProgress(message?.result)
  const duration = Number(process?.durationMs || 0)
    || normalizeList(progress?.steps).reduce((total, step) => total + (Number(step.durationMs) || 0), 0)
  const durationText = duration > 0
    ? `已用时 ${duration < 1000 ? `${Math.round(duration)} 毫秒` : `${Number((duration / 1000).toFixed(duration < 10000 ? 1 : 0))} 秒`}`
    : (message?.loading ? '正在执行' : '执行完成')
  const toolCount = Number(process?.toolCallCount || 0)
    || new Set(getThinkingSteps(message).map(step => step.toolName).filter(Boolean)).size
  return toolCount ? `${durationText} / 调用 ${toolCount} 个工具` : durationText
}

const shouldShowGeneralThinking = message => Boolean(
  message
  && !hasAnalysisTaskFlow(message.result)
  && (getThinkingProcess(message) || getAssistantProgress(message.result) || message.loading)
)

const isThinkingExpanded = messageId => expandedThinkingMessageIds.value.has(messageId)

const toggleThinking = messageId => {
  const next = new Set(expandedThinkingMessageIds.value)
  if (next.has(messageId)) next.delete(messageId)
  else next.add(messageId)
  expandedThinkingMessageIds.value = next
}

const getProgressStatusText = status => assistantProgressStatusMap[normalizeProgressStatus(status)]?.text || assistantProgressStatusMap.pending.text

const getProgressStatusStyle = status => {
  const color = assistantProgressStatusMap[normalizeProgressStatus(status)]?.color || assistantProgressStatusMap.pending.color
  return {
    color,
    borderColor: `${color}33`,
    background: `${color}12`
  }
}

const getElementProgressStatus = status => {
  const normalized = normalizeProgressStatus(status)
  if (normalized === 'failed') return 'exception'
  if (normalized === 'succeeded') return 'success'
  return undefined
}

const getProgressStepKey = (step, index) => step?.key || `${step?.step || index + 1}-${step?.name || 'step'}`

const getCurrentProgressStepName = progress => {
  const current = progress?.steps?.find(item => item.step === progress.currentStep)
    || progress?.steps?.find(item => item.status === 'running')
    || progress?.steps?.[progress.steps.length - 1]
  return current?.name || '-'
}

const formatDurationMs = value => {
  const number = Number(value)
  if (!Number.isFinite(number) || number <= 0) return ''
  if (number < 1000) return `${Math.round(number)}ms`
  return `${(number / 1000).toFixed(number < 10000 ? 1 : 0)}s`
}

const formatProgressTime = value => {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return formatTime(date)
}

const confidenceLabelMap = {
  high: '高',
  medium: '中',
  low: '低'
}

const severityLabelMap = {
  critical: '严重',
  danger: '严重',
  error: '严重',
  warning: '警告',
  warn: '警告',
  info: '提示',
  ok: '正常',
  normal: '正常',
  success: '正常'
}

const normalizeConfidence = value => {
  const text = String(value || '').toLowerCase()
  if (text.includes('high') || text.includes('高')) return 'high'
  if (text.includes('low') || text.includes('低')) return 'low'
  if (text.includes('medium') || text.includes('中')) return 'medium'
  return ''
}

const getConfidenceLabel = value => {
  const confidence = normalizeConfidence(value)
  return confidenceLabelMap[confidence] || formatDisplay(value) || '未知'
}

const normalizeCompleteness = value => {
  if (value === undefined || value === null || value === '') return null
  const number = Number(String(value).replace('%', ''))
  if (!Number.isFinite(number)) return null
  return number <= 1 ? Math.round(number * 100) : Math.round(number)
}

const getCompletenessLabel = value => {
  const normalized = normalizeCompleteness(value)
  return normalized === null ? '未知' : `${normalized}%`
}

const inferSeverity = result => {
  const card = result?.diagnosisCard || result?.diagnosis_card || {}
  const severity = String(result?.severity || result?.level || card?.severity || '').toLowerCase()
  if (severity) return severity
  const text = `${result?.answer || ''} ${card?.summary || ''}`
  if (/严重|critical|danger|error/i.test(text)) return 'critical'
  if (/风险|警告|偏高|异常|warning|warn/i.test(text)) return 'warning'
  if (/正常|健康|无明显/i.test(text)) return 'success'
  return 'info'
}

const extractFirstMatch = (text, patterns) => {
  for (const pattern of patterns) {
    const match = String(text || '').match(pattern)
    if (match?.[1]) return match[1].trim()
  }
  return ''
}

const stripMarkdownMarks = value => String(value || '')
  .replace(/^#{1,6}\s*/gm, '')
  .replace(/\*\*([^*]+)\*\*/g, '$1')
  .replace(/`([^`]+)`/g, '$1')
  .replace(/\s+/g, ' ')
  .trim()

const splitAnswerSections = value => {
  const lines = String(value || '').replace(/\r\n/g, '\n').split('\n')
  const sections = {}
  let current = 'intro'
  lines.forEach(rawLine => {
    const line = rawLine.trim()
    if (!line) return
    const heading = line.match(/^#{1,6}\s*(.+)$/)
    if (heading) {
      current = heading[1].trim()
      sections[current] = sections[current] || []
      return
    }
    sections[current] = sections[current] || []
    sections[current].push(line)
  })
  return sections
}

const getSectionText = (sections, names) => {
  const entry = Object.entries(sections).find(([key]) => names.some(name => key.includes(name)))
  return entry ? stripMarkdownMarks(entry[1].join('\n')) : ''
}

const normalizeInlineLabelText = value => stripMarkdownMarks(String(value || '')
  .replace(/^(主要异常|判断|结论|下一步建议|置信度|风险等级)\s*[-:：]?\s*/i, ''))

const parseAnswerDiagnosis = answer => {
  const text = String(answer || '').trim()
  if (!text) return {}
  const sections = splitAnswerSections(text)
  const introLines = sections.intro || []
  const severityLine = introLines.find(line => /^(严重|警告|提示|正常|critical|warning|info|ok)$/i.test(line.trim()))
  const intentLine = introLines.find(line => /^[a-z][a-z0-9_]+$/i.test(line.trim()))
  const conclusionText = getSectionText(sections, ['结论', '总结'])
  const nextText = getSectionText(sections, ['下一步', '建议'])
  const evidenceText = getSectionText(sections, ['关键证据', '证据'])
  const issue = extractFirstMatch(text, [
    /主要异常\s*[-:：]\s*([^\n]+)/,
    /主要风险\s*[-:：]\s*([^\n]+)/,
    /(?:存在|有)([^。\n]*(?:风险|异常)[^。\n]*)/
  ])
  const judgement = extractFirstMatch(text, [
    /判断\s*[-:：]\s*([^\n]+)/,
    /判断[^。\n]*[:：]\s*([^。\n]+)/,
    /(需要按[^。\n]+逐层收敛[^。\n]*。?)/
  ])
  const confidence = extractFirstMatch(text, [
    /置信度\s*[:：]?\s*(高|中|低|high|medium|low)/i
  ])
  const nextSteps = nextText
    ? nextText.split(/[。；;]/).map(item => normalizeInlineLabelText(item)).filter(Boolean).slice(0, 4)
    : []

  return {
    severityText: severityLine || extractFirstMatch(text, [/风险等级\s*[:：]?\s*(严重|警告|提示|正常|critical|warning|info|ok)/i]),
    intentText: intentLine || '',
    conclusionText,
    evidenceText,
    primaryIssue: normalizeInlineLabelText(issue),
    judgement: normalizeInlineLabelText(judgement),
    confidence,
    nextSteps
  }
}

const getMetricTone = (name, status) => {
  const text = `${name} ${status}`
  if (/严重|异常|风险|偏高|警告|高|critical|danger|error|warn/i.test(text)) return 'warning'
  if (/正常|ok|success/i.test(text)) return 'success'
  return 'info'
}

const normalizeMetricRows = result => {
  const metrics = normalizeList(
    result?.keyMetrics
    || result?.key_metrics
    || result?.metrics
    || result?.diagnosisCard?.metrics
    || result?.diagnosis_card?.metrics
  ).map(item => {
    if (typeof item === 'string') {
      const [name, value, status] = item.split(/[|：:]/).map(part => part.trim())
      return compactObject({ name, value, status, tone: getMetricTone(name, status) })
    }
    const name = item?.name || item?.metric || item?.label || item?.key || ''
    const value = item?.value ?? item?.current ?? item?.displayValue ?? item?.display_value ?? ''
    const status = item?.status || item?.state || item?.description || item?.note || ''
    return compactObject({ name, value: formatDisplay(value), status: formatDisplay(status), tone: getMetricTone(name, status) })
  }).filter(item => item.name)

  const answer = result?.answer || ''
  const inferred = [
    ['CPU 使用率', extractFirstMatch(answer, [/CPU\s*使用率\s*\|\s*([^|\n]+)/i, /CPU[^0-9]*(\d+(?:\.\d+)?%)/i])],
    ['内存使用率', extractFirstMatch(answer, [/内存使用率\s*\|\s*([^|\n]+)/, /内存[^0-9]*(\d+(?:\.\d+)?%)/])],
    ['磁盘使用率', extractFirstMatch(answer, [/磁盘使用率\s*\|\s*([^|\n]+)/, /磁盘[^0-9]*(\d+(?:\.\d+)?%)/])],
    ['5分钟负载', extractFirstMatch(answer, [/5分钟负载\s*\|\s*([^|\n]+)/, /5\s*分钟负载[^0-9]*(\d+(?:\.\d+)?)/])],
    ['CPU Top 进程', extractFirstMatch(answer, [/CPU\s*Top\s*进程\s*\|\s*([^|\n]+)/i])],
    ['内存 Top 进程', extractFirstMatch(answer, [/内存\s*Top\s*进程\s*\|\s*([^|\n]+)/i])]
  ].filter(([, value]) => value)
    .map(([name, value]) => ({
      name,
      value,
      status: /内存/.test(name) && /8[5-9]|9\d/.test(value) ? '偏高，存在风险' : '已获取',
      tone: getMetricTone(name, /内存/.test(name) && /8[5-9]|9\d/.test(value) ? '风险' : '正常')
    }))

  const names = new Set(metrics.map(item => item.name))
  inferred.forEach(item => {
    if (!names.has(item.name)) metrics.push(item)
  })
  return metrics
}

const getMissingEvidence = result => normalizeList(
  result?.missingEvidence
  || result?.missing_evidence
  || result?.evidenceGap
  || result?.evidence_gap
  || result?.reflection?.missingEvidence
  || result?.reflection?.missing_evidence
  || result?.diagnosisCard?.missingEvidence
  || result?.diagnosis_card?.missing_evidence
).map(formatDisplay).filter(Boolean)

const buildDiagnosisReport = result => {
  if (!result || typeof result !== 'object') return null
  const card = result.diagnosisCard || result.diagnosis_card || {}
  const reflection = result.reflection || result.reflector || result.selfReflection || result.self_reflection || null
  const hypotheses = normalizeList(card.hypotheses)
  const firstHypothesis = hypotheses[0] || {}
  const rawAnswer = result.answer || ''
  const cardSummaryText = typeof card.summary === 'string' ? card.summary : ''
  const answer = rawAnswer || cardSummaryText
  const cardSummaryLooksLikeReport = /((^|\n)\s*(#{1,6}\s*)?结论|platform_overview|主要异常|下一步建议)/.test(cardSummaryText)
  const parsedAnswer = parseAnswerDiagnosis(answer)
  const parsedIntent = parsedAnswer.intentText || result.intent
  const severity = parsedAnswer.severityText
    ? inferSeverity({ ...result, severity: parsedAnswer.severityText })
    : inferSeverity(result)
  const confidence = normalizeConfidence(
    result.confidence
    || result.confidenceLevel
    || result.confidence_level
    || card.confidence
    || firstHypothesis.confidence
    || reflection?.confidence
    || parsedAnswer.confidence
  )
  const metrics = normalizeMetricRows(result)
  const primaryIssue = card.primaryIssue
    || card.primary_issue
    || result.primaryIssue
    || result.primary_issue
    || parsedAnswer.primaryIssue
    || extractFirstMatch(answer, [/主要异常[:：]\s*([^\n]+)/, /(内存使用率\s*\d+(?:\.\d+)?%)/])
    || metrics.find(item => item.tone === 'warning')?.name
    || ''
  const judgement = card.judgement
    || card.judgment
    || result.judgement
    || result.judgment
    || firstHypothesis.cause
    || parsedAnswer.judgement
    || parsedAnswer.evidenceText
    || extractFirstMatch(answer, [/判断[:：]\s*([^\n]+)/, /(Redis[^。\n]+(?:较高|风险|阈值)[^。\n]*)/i])
  const missingEvidence = getMissingEvidence(result)
  const completeness = result.evidenceCompleteness
    || result.evidence_completeness
    || reflection?.evidenceCompleteness
    || reflection?.evidence_completeness
    || (missingEvidence.length ? Math.max(35, 100 - missingEvidence.length * 8) : '')
  const summary = parsedAnswer.conclusionText
    || (cardSummaryLooksLikeReport ? '' : card.summary)
    || extractFirstMatch(answer, [/结论\s*\n?([^。\n]+。?)/, /(主机\s*[^\n。]+(?:风险|异常|正常)[^。\n]*。?)/])
    || answer.split('\n').map(stripMarkdownMarks).find(line => line && !/^[a-z][a-z0-9_]+$/i.test(line) && !/^(严重|警告|提示|正常)$/i.test(line))
    || '诊断完成'

  if (!card.summary && !answer && !metrics.length && !hypotheses.length) return null

  return {
    title: getIntentLabel(parsedIntent) || '诊断报告',
    summary,
    severity,
    severityLabel: severityLabelMap[severity] || severityLabelMap.info,
    primaryIssue,
    judgement,
    confidence,
    confidenceLabel: getConfidenceLabel(confidence),
    completeness,
    completenessLabel: getCompletenessLabel(completeness),
    metrics,
    missingEvidence,
    nextSteps: [
      ...normalizeList(card.nextChecks || card.next_checks || result.nextSteps || result.next_steps).map(formatDisplay),
      ...parsedAnswer.nextSteps
    ].filter(Boolean)
  }
}

const getPlanSummary = items => {
  const list = normalizeList(items)
  const success = list.filter(item => ['success', 'succeeded', 'completed', 'done'].includes(getStepStatus(item))).length
  return { total: list.length, success }
}

const normalizeReflection = (reflection, report) => {
  if (!reflection && !report) return null
  const data = reflection && typeof reflection === 'object' && !Array.isArray(reflection)
    ? reflection
    : { summary: formatDisplay(reflection) }
  const summary = data.summary || data.message || data.conclusion || (report?.missingEvidence?.length ? '仍有部分关键证据缺失，建议结合下一步补采后确认。' : '')
  if (!summary) return null
  return {
    summary,
    confidence: normalizeConfidence(data.confidence || report?.confidence),
    confidenceLabel: getConfidenceLabel(data.confidence || report?.confidence)
  }
}

const getContextMismatch = result => {
  if (!result) return null
  const question = result.requestPayload?.question || ''
  const questionIp = extractIpList(question)[0] || result.requestPayload?.context?.explicitIp || ''
  if (!questionIp) return null
  const answerText = [
    result.answer,
    result.diagnosisCard?.summary,
    result.diagnosis_card?.summary,
    reportSummary.value?.summary,
    reportSummary.value?.primaryIssue,
    reportSummary.value?.judgement
  ].filter(Boolean).join('\n')
  const answerIp = extractIpList(answerText).find(ip => ip !== questionIp)
  return answerIp ? { questionIp, answerIp } : null
}

const updateSessionSummary = (question, result) => {
  if (!currentSession.value) return
  currentSession.value.title = question.slice(0, 32)
  currentSession.value.lastIntentLabel = getIntentLabel(result.intent)
  currentSession.value.updatedAt = formatTime()
  persistSessions()
}

const selectAssistantMessage = id => {
  selectedAssistantMessageId.value = id
  insightDrawerVisible.value = true
}

const subscribeSelectedRun = () => {
  if (!selectedResult.value?.runId || !selectedAssistantMessage.value) return
  subscribeRunStream(selectedResult.value.runId, selectedAssistantMessage.value.id)
}

const subscribeRunStream = (runId, messageId, options = {}) => {
  closeRunStream()
  streamConnecting.value = true
  try {
    streamDebug('sse:subscribe:start', { runId, messageId, options })
    if (options.keepExistingText) {
      ensureStreamOutput(messageId)
    } else {
      resetStreamOutput(messageId)
    }
    const source = aiopsApi.createAgentRunStream(runId)
    activeEventSource = source
    const trackedSource = source
    let firstSseEventReceived = false
    const firstSseEventTimer = window.setTimeout(() => {
      if (activeEventSource !== trackedSource || firstSseEventReceived) return
      streamWarn('sse:waiting-first-event', {
        runId,
        messageId,
        readyState: source.readyState,
        hint: 'SSE 已创建但 10 秒没有收到 run/step/result 等事件。若 readyState=1，通常是后端没有推送或代理缓冲。'
      })
    }, 10000)
    streamDebug('sse:subscribe:created', {
      runId,
      messageId,
      readyState: source.readyState,
      url: source.url || withAppBasePath(`/api/v1/aiops/agent/runs/${runId}/stream`)
    })

    const handleEvent = async (rawType, event) => {
      if (activeEventSource !== trackedSource) return
      if (!firstSseEventReceived) {
        firstSseEventReceived = true
        window.clearTimeout(firstSseEventTimer)
      }
      const rawData = parseEventData(event)
      const { type, data } = normalizeStreamEvent(rawType, rawData)
      const eventData = data && typeof data === 'object' && !Array.isArray(data) ? data : { value: data }
      const normalizedEvent = { type, ...eventData, receivedAt: formatTime() }
      streamDebug('sse:event', {
        runId,
        messageId,
        rawType,
        type,
        readyState: source.readyState,
        keys: Object.keys(eventData).slice(0, 16),
        status: eventData.status || eventData.runStatus || eventData.run_status || '',
        textLength: String(getStreamTextSegment(eventData, type) || '').length,
        isTerminal: isTerminalStreamEvent(type, eventData),
        rawPreview: typeof event?.data === 'string' ? event.data.slice(0, 240) : ''
      })

      mergeAssistantStreamResult(messageId, eventData, selectedResult.value?.requestPayload || {}, currentSession.value?.title || 'Run 轨迹', type)

      if (isStreamTextEvent(type, eventData)) {
        appendStreamEvent(messageId, normalizedEvent)
        appendStreamText(messageId, getStreamTextSegment(eventData, type))
        return
      }

      appendStreamEvent(messageId, normalizedEvent)

      if (isTerminalStreamEvent(type, eventData)) {
        window.clearTimeout(firstSseEventTimer)
        await finishRunFromStream(runId, messageId, eventData)
        return
      }
    }

    source.addEventListener('open', () => {
      if (activeEventSource !== trackedSource) return
      streamDebug('sse:open', { runId, messageId, readyState: source.readyState })
      streamConnecting.value = false
      streamConnected.value = true
      appendStreamEvent(messageId, { type: 'open', status: 'connected', receivedAt: formatTime() })
    })
    source.addEventListener('run', event => handleEvent('run', event))
    source.addEventListener('step', event => handleEvent('step', event))
    source.addEventListener('progress', event => handleEvent('progress', event))
    source.addEventListener('analysis_progress', event => handleEvent('analysis_progress', event))
    source.addEventListener('intent', event => handleEvent('intent', event))
    source.addEventListener('llm', event => handleEvent('llm', event))
    source.addEventListener('message', event => handleEvent('message', event))
    source.addEventListener('delta', event => handleEvent('delta', event))
    source.addEventListener('token', event => handleEvent('token', event))
    source.addEventListener('answer', event => handleEvent('answer', event))
    source.addEventListener('answer_delta', event => handleEvent('answer_delta', event))
    source.addEventListener('content_delta', event => handleEvent('content_delta', event))
    source.addEventListener('llm_delta', event => handleEvent('llm_delta', event))
    source.addEventListener('result', event => handleEvent('result', event))
    source.addEventListener('complete', event => handleEvent('complete', event))
    source.addEventListener('done', event => handleEvent('done', event))
    source.onerror = () => {
      if (activeEventSource !== trackedSource) return
      window.clearTimeout(firstSseEventTimer)
      streamWarn('sse:error', { runId, messageId, readyState: source.readyState })
      streamConnecting.value = false
      streamConnected.value = false
      appendStreamEvent(messageId, { type: 'error', status: 'closed', receivedAt: formatTime() })
      markStreamClosed(messageId)
      closeRunStream()
    }
  } catch (error) {
    streamWarn('sse:subscribe:error', error)
    streamConnecting.value = false
    streamConnected.value = false
    ElMessage.error(error.message || 'SSE 连接失败')
  }
}

const normalizeStreamEvent = (rawType, rawData) => {
  if (rawData && typeof rawData === 'object') {
    const embeddedType = rawData.type || rawData.event || rawData.eventType
    const embeddedData = rawData.data || rawData.payload
    if (embeddedType && embeddedData && typeof embeddedData === 'object' && !Array.isArray(embeddedData)) {
      return {
        type: String(embeddedType).toLowerCase(),
        data: {
          ...embeddedData,
          rawEventType: rawType
        }
      }
    }
    return {
      type: String(embeddedType || rawType).toLowerCase(),
      data: rawData
    }
  }

  return {
    type: String(rawType).toLowerCase(),
    data: { message: String(rawData ?? '') }
  }
}

const isTerminalStreamEvent = (type, data) => {
  const eventType = String(type || '').toLowerCase()
  if (['result', 'complete', 'completed', 'done', 'finish', 'finished'].includes(eventType)) return true

  if (!['run', 'message'].includes(eventType)) return false

  const status = String(data?.status || data?.runStatus || '').toLowerCase()
  if (status === 'running') return false
  return ['success', 'succeeded', 'completed', 'done', 'failed', 'error', 'finished'].includes(status)
}

const isStreamTextEvent = (type, data) => Boolean(getStreamTextSegment(data, type))
  && ['message', 'delta', 'token', 'answer', 'answer_delta', 'content_delta', 'llm_delta'].includes(type)

const getStreamTextSegment = (data, type = '') => {
  if (!data || typeof data !== 'object') return ''
  const eventType = String(type || data.type || data.event || data.eventType || '').toLowerCase()
  return data.delta
    || data.token
    || data.answerDelta
    || data.answer_delta
    || data.contentDelta
    || data.content_delta
    || (eventType === 'answer' ? data.answer : '')
    || data.text
    || data.content
    || data.message
    || ''
}

const resetStreamOutput = (messageId, text = '') => {
  const message = messages.value.find(item => item.id === messageId)
  if (!message) return
  message.streamAnswer = text
  message.content = text
  message.loading = true
  persistSessions()
  nextTick(scrollMessagesToBottom)
}

const ensureStreamOutput = (messageId, fallbackText = '') => {
  const message = messages.value.find(item => item.id === messageId)
  if (!message) return
  const text = message.streamAnswer || message.content || fallbackText
  message.streamAnswer = text
  message.content = text
  message.loading = true
  persistSessions()
  nextTick(scrollMessagesToBottom)
}

const appendStreamText = (messageId, segment) => {
  if (!segment) return
  const message = messages.value.find(item => item.id === messageId)
  if (!message) return
  message.streamAnswer = `${message.streamAnswer || ''}${segment}`
  message.content = message.streamAnswer
  message.loading = true
  persistSessions()
  nextTick(scrollMessagesToBottom)
}

const appendStreamEvent = (messageId, event) => {
  const message = messages.value.find(item => item.id === messageId)
  if (!message) return
  message.streamEvents = [...normalizeList(message.streamEvents), event].slice(-30)
  persistSessions()
}

const finishRunFromStream = async (runId, messageId, data) => {
  const message = messages.value.find(item => item.id === messageId)
  if (!message) return

  if (String(data?.type || '').toLowerCase() === 'result' || getFinalAnswerFromPayload(data)) {
    finalizeAssistantResultEvent(
      messageId,
      message.result?.requestPayload || {},
      message.result?.requestPayload?.question || currentSession.value?.title || 'Run 结果',
      data
    )
    closeRunStream()
    return
  }

  const finalPayload = unwrapStreamResult(data)
  streamDebug('stream:finish', {
    runId,
    messageId,
    dataType: data?.type || '',
    keys: Object.keys(data || {}).slice(0, 16),
    finalPayloadKeys: Object.keys(finalPayload || {}).slice(0, 16),
    answerLength: String(finalPayload?.answer || finalPayload?.result?.answer || '').length
  })

  if (Object.keys(finalPayload).length) {
    const merged = normalizeAssistantResult({
      ...message.result,
      ...finalPayload
    }, message.result?.requestPayload)
    const finalAnswer = getAssistantAnswerText(merged)
    const content = message.streamAnswer && finalAnswer && !message.streamAnswer.includes(finalAnswer)
      ? `${message.streamAnswer}\n\n${finalAnswer}`
      : (finalAnswer || message.streamAnswer)
    updateMessage(messageId, {
      content,
      loading: false,
      result: merged
    })
    updateSessionSummary(message.result?.requestPayload?.question || currentSession.value?.title || 'Run 结果', merged)
  } else {
    await loadRunIntoMessage(runId, messageId, { silent: true, keepStreamText: true })
  }

  closeRunStream()
}

const unwrapStreamResult = data => {
  if (!data || typeof data !== 'object') return {}
  const candidate = data.result || data.output || data.data || data.payload
  if (candidate && typeof candidate === 'object' && !Array.isArray(candidate)) return candidate
  if (
    data.answer
    || data.diagnosisCard
    || data.diagnosis_card
    || data.assistantPlan
    || data.assistant_plan
    || data.progress
    || data.thinkingProcess
    || data.thinking_process
    || data.analysisTask
    || data.analysis_task
    || data.reflection
    || data.intent
    || data.entities
  ) return data
  return {}
}

const markStreamClosed = messageId => {
  const message = messages.value.find(item => item.id === messageId)
  if (!message?.loading) return
  const status = String(message.result?.status || message.result?.runStatus || message.result?.run?.status || '').toLowerCase()
  if (terminalRunStatuses.has(status)) {
    updateMessage(messageId, { loading: false })
    return
  }
  updateMessage(messageId, {
    loading: false,
    content: message.streamAnswer || message.content || '实时连接已关闭，可点击回看 Run 获取最新结果。'
  })
}

const parseEventData = event => {
  try {
    return JSON.parse(event.data)
  } catch {
    return { message: event.data }
  }
}

const closeRunStream = () => {
  if (activeEventSource) {
    streamDebug('sse:close', { readyState: activeEventSource.readyState, url: activeEventSource.url || '' })
    activeEventSource.close()
    activeEventSource = null
  }
  streamConnecting.value = false
  streamConnected.value = false
}

const refreshSelectedRun = async () => {
  if (!selectedResult.value?.runId || !selectedAssistantMessage.value) return
  await loadRunIntoMessage(selectedResult.value.runId, selectedAssistantMessage.value.id)
}

const loadRunIntoMessage = async (runId, messageId, options = {}) => {
  try {
    const response = await aiopsApi.getAgentRun(runId)
    const data = getPayload(response, '获取 Run 详情失败')
    const message = messages.value.find(item => item.id === messageId)
    const currentResult = message?.result || selectedResult.value || {}
    const merged = normalizeAssistantResult({
      ...currentResult,
      run: data,
      ...unwrapStreamResult(data),
      steps: data?.steps || currentResult.timeline,
      evidence: data?.evidence || data?.evidences || currentResult.evidence
    }, currentResult.requestPayload)
    const answerText = getAssistantAnswerText(merged)
    const content = options.keepStreamText && message?.streamAnswer
      ? `${message.streamAnswer}\n\n${answerText}`
      : answerText
    updateMessage(messageId, {
      result: merged,
      content,
      loading: false
    })
    if (!options.silent) ElMessage.success('Run 详情已更新')
  } catch (error) {
    console.error('refresh run failed:', error)
    if (!options.silent) {
      ElMessage.error(error.message || '获取 Run 详情失败')
    } else {
      updateMessage(messageId, {
        loading: false,
        error: error.message || '获取 Run 详情失败'
      })
    }
  }
}

const loadProviders = async () => {
  providerLoading.value = true
  try {
    const response = await aiopsApi.getEnabledLLMProviders({ page: 1, pageSize: 100 })
    providerOptions.value = normalizeList(getPayload(response, '获取 Provider 失败'))
    syncDefaultProvider()
  } catch (error) {
    console.warn('load providers failed:', error)
    providerOptions.value = []
  } finally {
    providerLoading.value = false
  }
}

const getProviderId = provider => provider?.id ?? provider?.providerId ?? provider?.value ?? ''

const getProviderLabel = provider => {
  const name = provider?.modelName || provider?.name || provider?.providerName || `Provider ${getProviderId(provider)}`
  const model = provider?.modelCode || provider?.model || ''
  return model ? `${name} (${model})` : name
}

const isDefaultProvider = provider => {
  const value = provider?.isDefault
    ?? provider?.is_default
    ?? provider?.defaultProvider
    ?? provider?.default_provider
    ?? provider?.defaultFlag
    ?? provider?.default_flag
    ?? provider?.default
  return value === true || value === 1 || value === '1' || String(value).toLowerCase() === 'true'
}

const syncDefaultProvider = () => {
  if (isFilled(chatForm.providerId)) return
  const provider = providerOptions.value.find(isDefaultProvider) || providerOptions.value[0]
  if (provider) chatForm.providerId = getProviderId(provider)
}

const objectToRows = object => {
  if (!object || typeof object !== 'object' || Array.isArray(object)) return []
  return Object.entries(object).map(([key, value]) => ({
    key,
    value: formatDisplay(value)
  }))
}

const getIntentLabel = intent => intentLabelMap[intent] || intent || intentLabelMap.unknown

const getStepName = item => item?.toolName || item?.tool_name || item?.name || item?.stepName || item?.type || `step ${item?.step || ''}`
const getStepPurpose = item => item?.purpose || item?.message || item?.description || item?.summary || formatDisplay(item)
const getStepStatus = item => String(item?.status || (item?.success === false ? 'failed' : item?.success === true ? 'succeeded' : 'pending')).toLowerCase()
const getStepStatusLabel = item => {
  const status = getStepStatus(item)
  const duration = item?.durationMs || item?.duration || item?.elapsedMs || ''
  return duration ? `${status} · ${duration}ms` : status
}

const formatDisplay = value => {
  if (value === undefined || value === null || value === '') return ''
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  try {
    return JSON.stringify(value)
  } catch {
    return String(value)
  }
}

const formatEvidence = value => {
  if (typeof value === 'string') return value
  try {
    return JSON.stringify(value, null, 2)
  } catch {
    return String(value)
  }
}

const escapeHtml = value =>
  String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')

const renderInlineMarkdown = text =>
  escapeHtml(text)
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')

const getCodeBlockLanguage = value => {
  const language = String(value || '').trim()
  return language || 'text'
}

const renderCodeBlock = (codeLines, language = '') => {
  const lang = getCodeBlockLanguage(language)
  const code = escapeHtml(codeLines.join('\n').replace(/\n+$/g, ''))
  return [
    '<div class="script-block">',
    '<div class="script-block-head">',
    `<span>${escapeHtml(lang)}</span>`,
    '<button type="button" class="script-copy-btn" data-copy-code>复制</button>',
    '</div>',
    `<pre><code class="language-${escapeHtml(lang)}">${code}</code></pre>`,
    '</div>'
  ].join('')
}

const shellCommandPattern = /^\s*(#!\/|(?:sudo\s+)?(?:apt|apt-get|awk|cat|cd|chmod|chown|cp|curl|date|df|dnf|docker|du|echo|egrep|export|find|free|grep|head|hostname|ip|journalctl|kubectl|ls|lsof|mkdir|mv|netstat|ps|rm|rpm|sed|set|sleep|sort|ss|systemctl|tail|tar|top|touch|tr|uname|uptime|vmstat|wc|wget|yum)\b|\w+=)/
const shellControlPattern = /^\s*(?:if|then|else|elif|fi|for|while|do|done|case|esac|function)\b/

const isShellScriptLine = value => {
  const text = String(value || '')
  const trimmed = text.trim()
  if (!trimmed) return false
  if (/^#/.test(trimmed)) return true
  if (shellCommandPattern.test(text) || shellControlPattern.test(text)) return true
  return /(?:\|\||&&|[|;])\s*$/.test(text)
}

const isExecutableShellLine = value => {
  const text = String(value || '')
  return shellCommandPattern.test(text) || shellControlPattern.test(text)
}

const isLikelyShellScriptStart = (lines, index) => {
  const current = lines[index]
  const nextLines = lines.slice(index, index + 4)
  if (/^\s*#!\//.test(current)) return true
  if (/^\s*~{3,}\s*(?:bash|sh|shell|zsh)?\s*$/i.test(current)) return true
  if (!isShellScriptLine(current)) return false
  const scriptLineCount = nextLines.filter(isShellScriptLine).length
  const executableLineCount = nextLines.filter(isExecutableShellLine).length
  return scriptLineCount >= 2 && executableLineCount >= 1
}

const collectShellScriptLines = (lines, startIndex) => {
  const codeLines = []
  let index = startIndex
  for (; index < lines.length; index += 1) {
    const line = lines[index]
    const trimmed = line.trim()
    if (!trimmed) {
      if (codeLines.length) break
      continue
    }
    if (!isShellScriptLine(line) && codeLines.length) break
    codeLines.push(line)
  }
  return { codeLines, nextIndex: index - 1 }
}

const isMarkdownTableRow = value => {
  const text = String(value || '').trim()
  return text.startsWith('|') && text.endsWith('|') && text.split('|').length >= 4
}

const splitMarkdownTableRow = value =>
  String(value || '')
    .trim()
    .replace(/^\|/, '')
    .replace(/\|$/, '')
    .split('|')
    .map(item => item.trim())

const isMarkdownTableSeparator = value => {
  if (!isMarkdownTableRow(value)) return false
  const cells = splitMarkdownTableRow(value)
  return cells.length > 1 && cells.every(cell => /^:?-{2,}:?$/.test(cell))
}

const renderMarkdownTable = rows => {
  const header = splitMarkdownTableRow(rows[0])
  const bodyRows = rows.slice(2).map(splitMarkdownTableRow)
  const columnCount = header.length
  const renderCells = (cells, tag) => Array.from({ length: columnCount }, (_, index) => {
    const content = cells[index] || ''
    return `<${tag}>${renderInlineMarkdown(content)}</${tag}>`
  }).join('')

  return [
    '<div class="markdown-table-wrap">',
    '<table class="markdown-table">',
    '<thead>',
    `<tr>${renderCells(header, 'th')}</tr>`,
    '</thead>',
    '<tbody>',
    ...bodyRows.map(cells => `<tr>${renderCells(cells, 'td')}</tr>`),
    '</tbody>',
    '</table>',
    '</div>'
  ].join('')
}

const parseFieldTableRow = value => {
  const text = String(value || '').trim()
  const rowMatch = text.match(/^(?:[-*]\s*)?(?:(\d+)[.)]\s*)?(.+)$/)
  if (!rowMatch || !text.includes('|')) return null

  const segments = rowMatch[2]
    .split('|')
    .map(item => item.trim())
    .filter(Boolean)
  if (segments.length < 3) return null

  const row = { name: segments[0] }
  segments.slice(1).forEach(segment => {
    const match = segment.match(/^([^:：]+)\s*[:：]\s*(.*)$/)
    if (match) {
      row[match[1].trim()] = match[2].trim()
    }
  })

  const fieldCount = Object.keys(row).length - 1
  return fieldCount >= 2 ? row : null
}

const collectFieldTableRows = (lines, startIndex) => {
  const rows = []
  let index = startIndex
  for (; index < lines.length; index += 1) {
    const row = parseFieldTableRow(lines[index])
    if (!row) break
    rows.push(row)
  }
  return { rows, nextIndex: index - 1 }
}

const renderFieldTable = rows => {
  const preferredHeaders = ['名称', '类型', '环境', '状态', '地址', '负责人']
  const dynamicHeaders = []
  rows.forEach(row => {
    Object.keys(row).forEach(key => {
      const label = key === 'name' ? '名称' : key
      if (label !== '名称' && !preferredHeaders.includes(label) && !dynamicHeaders.includes(label)) {
        dynamicHeaders.push(label)
      }
    })
  })
  const headers = preferredHeaders
    .filter(header => header === '名称' || rows.some(row => row[header]))
    .concat(dynamicHeaders)
  const renderCell = (row, header) => renderInlineMarkdown(header === '名称' ? row.name : row[header] || '-')

  return [
    '<div class="markdown-table-wrap field-table-wrap">',
    '<table class="markdown-table field-table">',
    '<thead>',
    `<tr>${headers.map(header => `<th>${escapeHtml(header)}</th>`).join('')}</tr>`,
    '</thead>',
    '<tbody>',
    ...rows.map(row => `<tr>${headers.map(header => `<td>${renderCell(row, header)}</td>`).join('')}</tr>`),
    '</tbody>',
    '</table>',
    '</div>'
  ].join('')
}

const renderAnswer = value => {
  const lines = String(value || '').replace(/\r\n/g, '\n').split('\n')
  const html = []
  let listType = ''
  let inCodeBlock = false
  let codeLanguage = ''
  let codeLines = []

  const closeList = () => {
    if (!listType) return
    html.push(listType === 'ol' ? '</ol>' : '</ul>')
    listType = ''
  }

  const openList = type => {
    if (listType === type) return
    closeList()
    html.push(type === 'ol' ? '<ol>' : '<ul>')
    listType = type
  }

  for (let index = 0; index < lines.length; index += 1) {
    const line = lines[index]
    const fenceMatch = line.match(/^\s*(?:```|~~~|~{3,})\s*~?([\w-]*)\s*$/)
    if (fenceMatch) {
      if (inCodeBlock) {
        html.push(renderCodeBlock(codeLines, codeLanguage))
        codeLines = []
        codeLanguage = ''
        inCodeBlock = false
        continue
      }
      closeList()
      inCodeBlock = true
      codeLanguage = fenceMatch[1] || ''
      codeLines = []
      continue
    }

    if (inCodeBlock) {
      codeLines.push(line)
      continue
    }

    const trimmed = line.trim()
    if (!trimmed) {
      closeList()
      continue
    }

    if (isLikelyShellScriptStart(lines, index)) {
      closeList()
      if (/^\s*~{3,}\s*/.test(line)) {
        inCodeBlock = true
        codeLanguage = line.replace(/^\s*~{3,}\s*/, '').trim() || 'bash'
        codeLines = []
        continue
      }
      const collected = collectShellScriptLines(lines, index)
      if (collected.codeLines.length) {
        html.push(renderCodeBlock(collected.codeLines, 'bash'))
        index = collected.nextIndex
        continue
      }
    }

    if (isMarkdownTableRow(line) && isMarkdownTableSeparator(lines[index + 1])) {
      closeList()
      const tableRows = [line, lines[index + 1]]
      index += 2
      while (index < lines.length && isMarkdownTableRow(lines[index])) {
        tableRows.push(lines[index])
        index += 1
      }
      index -= 1
      html.push(renderMarkdownTable(tableRows))
      continue
    }

    const fieldRow = parseFieldTableRow(line)
    if (fieldRow) {
      const collected = collectFieldTableRows(lines, index)
      if (collected.rows.length >= 2) {
        closeList()
        html.push(renderFieldTable(collected.rows))
        index = collected.nextIndex
        continue
      }
    }

    if (/^---+$/.test(trimmed)) {
      closeList()
      html.push('<hr>')
      continue
    }

    const headingMatch = trimmed.match(/^(#{1,4})\s+(.+)$/)
    if (headingMatch) {
      closeList()
      const level = Math.min(headingMatch[1].length + 2, 5)
      html.push(`<h${level}>${renderInlineMarkdown(headingMatch[2])}</h${level}>`)
      continue
    }

    const numberedMatch = trimmed.match(/^(\d+)[.)]\s+(.+)$/)
    if (numberedMatch) {
      openList('ol')
      html.push(`<li>${renderInlineMarkdown(numberedMatch[2])}</li>`)
      continue
    }

    const bulletMatch = trimmed.match(/^[-*]\s+(.+)$/)
    if (bulletMatch) {
      openList('ul')
      html.push(`<li>${renderInlineMarkdown(bulletMatch[1])}</li>`)
      continue
    }

    closeList()
    html.push(`<p>${renderInlineMarkdown(trimmed)}</p>`)
  }

  if (inCodeBlock) {
    html.push(renderCodeBlock(codeLines, codeLanguage))
  }
  closeList()
  return html.join('')
}

const copyTextToClipboard = async text => {
  const value = String(text || '')
  if (!value) return false

  if (navigator.clipboard?.writeText && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(value)
      return true
    } catch (error) {
      console.warn('navigator clipboard failed, fallback to textarea copy:', error)
    }
  }

  const textarea = document.createElement('textarea')
  textarea.value = value
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.top = '-9999px'
  textarea.style.left = '-9999px'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)

  const selection = document.getSelection()
  const selectedRange = selection && selection.rangeCount > 0 ? selection.getRangeAt(0) : null

  textarea.focus()
  textarea.select()
  textarea.setSelectionRange(0, textarea.value.length)

  let copied = false
  try {
    copied = document.execCommand('copy')
  } catch (error) {
    console.warn('textarea copy failed:', error)
  } finally {
    document.body.removeChild(textarea)
    if (selectedRange && selection) {
      selection.removeAllRanges()
      selection.addRange(selectedRange)
    }
  }

  return copied
}

const handleAnswerContentClick = async event => {
  const button = event.target?.closest?.('[data-copy-code]')
  if (!button) return

  const block = button.closest('.script-block')
  const code = block?.querySelector('pre code')?.textContent || ''
  if (!code) return

  const copied = await copyTextToClipboard(code)
  if (copied) {
    ElMessage.success('脚本已复制')
  } else {
    ElMessage.error('复制失败，请手动选中脚本复制')
  }
}

const scrollMessagesToBottom = () => {
  const viewport = messageViewportRef.value
  if (!viewport) return
  viewport.scrollTop = viewport.scrollHeight
}

const persistSessions = () => {
  try {
    window.localStorage.setItem(sessionsStorageKey, JSON.stringify(sessions.value.slice(0, 12)))
    window.localStorage.setItem(activeSessionStorageKey, activeSessionId.value)
  } catch (error) {
    console.warn('persist assistant sessions failed:', error)
  }
}

const loadLocalSessions = () => {
  try {
    const saved = JSON.parse(window.localStorage.getItem(sessionsStorageKey) || '[]')
    if (Array.isArray(saved) && saved.length) {
      sessions.value = saved.map(session => ({
        ...session,
        messages: normalizeList(session?.messages).map(message => message?.result
          ? {
              ...message,
              result: normalizeAssistantResult(message.result, message.result.requestPayload)
            }
          : message)
      }))
      activeSessionId.value = window.localStorage.getItem(activeSessionStorageKey) || saved[0].id
    }
  } catch (error) {
    console.warn('load assistant sessions failed:', error)
  }
  if (!sessions.value.length) createSession()
}

const copySelectedResult = async () => {
  if (!selectedResult.value) return
  const copied = await copyTextToClipboard(JSON.stringify(selectedResult.value, null, 2))
  if (copied) {
    ElMessage.success('结果已复制')
  } else {
    ElMessage.error('复制失败，请手动复制结果')
  }
}

const submitFeedback = async type => {
  if (!selectedResult.value) return
  feedbackLoading.value = true
  try {
    const ratingMap = {
      useful: 'helpful',
      inaccurate: 'wrong',
      insufficient_evidence: 'insufficient_evidence',
      tool_error: 'tool_error'
    }
    await aiopsApi.submitAgentFeedback({
      runId: selectedResult.value.runId,
      sessionId: activeSessionId.value,
      rating: ratingMap[type] || type,
      intent: selectedResult.value.intent,
      feedbackType: type,
      type,
      helpful: type === 'useful',
      messageId: selectedAssistantMessage.value?.id || '',
      context: {
        question: selectedResult.value.requestPayload?.question || currentSession.value?.title || '',
        diagnosisCard: selectedResult.value.diagnosisCard || selectedResult.value.diagnosis_card || null,
        reflection: selectedResult.value.reflection || null,
        reportSummary: reportSummary.value,
        selectedReason: type,
        contextMismatch: contextMismatch.value,
        requestContext: selectedResult.value.requestPayload?.context || {}
      },
      question: selectedResult.value.requestPayload?.question || currentSession.value?.title || '',
      answerPreview: String(selectedResult.value.answer || '').slice(0, 500)
    })
    ElMessage.success('反馈已提交')
  } catch (error) {
    console.error('submit feedback failed:', error)
    ElMessage.error(error.message || '反馈提交失败')
  } finally {
    feedbackLoading.value = false
  }
}

watch(activeSessionId, persistSessions)

onMounted(() => {
  loadLocalSessions()
  loadProviders()
  mcpConfirmationClockTimer = window.setInterval(() => {
    mcpConfirmationNow.value = Date.now()
  }, 1000)
  nextTick(scrollMessagesToBottom)
})

onBeforeUnmount(() => {
  closeRunStream()
  if (mcpConfirmationClockTimer) window.clearInterval(mcpConfirmationClockTimer)
})
</script>

<style scoped>
.assistant-page {
  --aiops-surface-cool: rgba(247, 250, 255, 0.84);
  --aiops-surface-warm: rgba(255, 253, 248, 0.94);
  --aiops-text-primary: #172133;
  --aiops-text-secondary: #5f6b7c;
  --aiops-text-muted: #8d97a8;
  --aiops-border-cool: #dfe7f2;
  --aiops-border-mid: rgba(207, 217, 230, 0.92);
  --aiops-brand: #4c8bf5;
  --aiops-brand-active: #4d79d8;
  --aiops-brand-soft: rgba(76, 139, 245, 0.12);
  --aiops-success: #3fb97f;
  --aiops-warning: #f3b24b;
  --aiops-danger: #ef6b73;
  padding: 5px;
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.assistant-shell {
  width: 100%;
  margin: 0 auto;
}

.assistant-card {
  height: calc(100vh - 96px);
  min-height: 620px;
  max-height: calc(100vh - 96px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.94);
  border-radius: 28px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.96), rgba(247, 250, 255, 0.92));
  box-shadow: 0 18px 48px rgba(17, 24, 39, 0.08);
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
  padding: 18px 20px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.9);
  background: linear-gradient(90deg, #fff9f0 0%, #f8f8f6 52%, #edf2fd 100%);
}

.header-title p,
.header-title h2,
.header-title span {
  margin: 0;
}

.header-title p {
  color: var(--aiops-brand-active);
  font-size: 12px;
  font-weight: 700;
}

.header-title h2 {
  margin-top: 4px;
  color: var(--aiops-text-primary);
  font-size: 28px;
  font-weight: 750;
  line-height: 1.2;
}

.header-title span {
  display: block;
  margin-top: 6px;
  color: var(--aiops-text-muted);
  font-size: 13px;
}

.header-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  flex-wrap: wrap;
}

.provider-select {
  width: 230px;
}

.status-chip {
  height: 32px;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 0 12px;
  border-radius: 999px;
  border: 1px solid rgba(76, 139, 245, 0.16);
  background: rgba(76, 139, 245, 0.08);
  color: var(--aiops-brand-active);
  font-size: 12px;
  font-weight: 700;
}

.status-chip i {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: var(--aiops-success);
}

.assistant-page :deep(.header-action-btn) {
  height: 30px;
  min-width: 82px;
  border-radius: 9px;
  border: 0;
  color: #fff;
  font-weight: 750;
  box-shadow: 0 8px 18px rgba(17, 24, 39, 0.08);
  transition: transform 0.18s ease, box-shadow 0.18s ease, filter 0.18s ease;
}

.assistant-page :deep(.header-action-btn.success) {
  background: linear-gradient(180deg, #46c98b 0%, var(--aiops-success) 100%);
}

.assistant-page :deep(.header-action-btn.warning) {
  background: linear-gradient(180deg, #f6c15f 0%, var(--aiops-warning) 100%);
}

.assistant-page :deep(.header-action-btn.success:hover),
.assistant-page :deep(.header-action-btn.success:focus),
.assistant-page :deep(.header-action-btn.warning:hover),
.assistant-page :deep(.header-action-btn.warning:focus) {
  color: #fff;
  filter: brightness(1.03);
  box-shadow: 0 10px 22px rgba(63, 185, 127, 0.24);
}

.assistant-page :deep(.header-action-btn.warning:hover),
.assistant-page :deep(.header-action-btn.warning:focus) {
  box-shadow: 0 10px 22px rgba(243, 178, 75, 0.24);
}

.assistant-page :deep(.header-action-btn.danger) {
  background: linear-gradient(180deg, #f47b82 0%, var(--aiops-danger) 100%);
}

.assistant-page :deep(.header-action-btn.danger:hover),
.assistant-page :deep(.header-action-btn.danger:focus) {
  color: #fff;
  filter: brightness(1.03);
  box-shadow: 0 10px 22px rgba(239, 107, 115, 0.24);
}

.assistant-page :deep(.header-action-btn:not(.is-disabled):hover) {
  transform: translateY(-1px);
}

.assistant-page :deep(.header-action-btn.is-disabled) {
  opacity: 0.58;
  color: #fff;
  box-shadow: none;
}

.workspace {
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
  display: grid;
  grid-template-columns: 280px minmax(420px, 1fr) 380px;
  gap: 8px;
  padding: 8px;
}

.session-panel,
.chat-panel,
.insight-panel {
  min-height: 0;
  border: 1px solid rgba(223, 231, 242, 0.9);
  background: rgba(255, 255, 255, 0.62);
  backdrop-filter: blur(14px);
}

.session-panel {
  display: flex;
  flex-direction: column;
  border-radius: 22px;
  padding: 12px;
}

.chat-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  overflow: hidden;
  border-radius: 24px;
  background: rgba(247, 250, 255, 0.66);
}

.insight-panel {
  overflow: auto;
  border-radius: 22px;
  padding: 12px;
}

.panel-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 12px;
}

.panel-title.compact {
  margin: 18px 0 10px;
}

.panel-title h3,
.panel-title p {
  margin: 0;
}

.panel-title h3 {
  color: var(--aiops-text-primary);
  font-size: 15px;
  font-weight: 750;
}

.panel-title p {
  margin-top: 3px;
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.session-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.assistant-page :deep(.session-action-btn) {
  width: 32px;
  height: 32px;
  border: 0;
  border-radius: 9px;
  color: #fff;
  box-shadow: 0 6px 14px rgba(17, 24, 39, 0.08);
  transition: transform 0.18s ease, box-shadow 0.18s ease, border-color 0.18s ease, background 0.18s ease;
}

.assistant-page :deep(.session-action-btn.success) {
  background: linear-gradient(180deg, #46c98b 0%, var(--aiops-success) 100%);
}

.assistant-page :deep(.session-action-btn.success:hover),
.assistant-page :deep(.session-action-btn.success:focus) {
  color: #fff;
  filter: brightness(1.03);
  box-shadow: 0 8px 18px rgba(63, 185, 127, 0.22);
}

.assistant-page :deep(.session-action-btn.danger) {
  background: linear-gradient(180deg, #f47b82 0%, var(--aiops-danger) 100%);
}

.assistant-page :deep(.session-action-btn.danger:hover),
.assistant-page :deep(.session-action-btn.danger:focus) {
  color: #fff;
  filter: brightness(1.03);
  box-shadow: 0 8px 18px rgba(239, 107, 115, 0.22);
}

.assistant-page :deep(.session-action-btn:not(.is-disabled):hover) {
  transform: translateY(-1px);
}

.assistant-page :deep(.session-action-btn.is-disabled) {
  opacity: 0.58;
  color: #fff;
  box-shadow: none;
}

.session-list {
  display: grid;
  gap: 8px;
  overflow: auto;
  max-height: 38vh;
  padding-right: 2px;
}

.session-item,
.template-item {
  width: 100%;
  border: 1px solid rgba(223, 231, 242, 0.76);
  background: rgba(255, 255, 255, 0.58);
  cursor: pointer;
  text-align: left;
  transition: border-color 0.2s ease, background 0.2s ease, transform 0.2s ease;
}

.session-item {
  display: grid;
  gap: 5px;
  padding: 10px 11px;
  border-radius: 16px;
}

.session-item:hover,
.template-item:hover {
  transform: translateY(-1px);
  border-color: rgba(76, 139, 245, 0.28);
  background: rgba(255, 255, 255, 0.82);
}

.session-item.active {
  border-color: rgba(76, 139, 245, 0.34);
  background: rgba(76, 139, 245, 0.08);
}

.session-item span,
.session-item strong,
.session-item small {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-item span {
  color: var(--aiops-text-primary);
  font-size: 13px;
  font-weight: 700;
}

.session-item strong {
  color: var(--aiops-brand-active);
  font-size: 12px;
}

.session-item small {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.template-panel {
  border-top: 1px solid rgba(223, 231, 242, 0.86);
  min-height: 0;
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  overflow: hidden;
}

.prompt-intro {
  margin: 0 0 8px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.45;
}

.prompt-collapse {
  min-height: 0;
  overflow: auto;
  padding-right: 2px;
}

.prompt-collapse :deep(.el-collapse-item) {
  margin-bottom: 6px;
  border: 1px solid rgba(223, 231, 242, 0.76);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.52);
}

.prompt-collapse :deep(.el-collapse-item__header) {
  height: 34px;
  padding: 0 9px;
  border: 0;
  background: transparent;
  line-height: 34px;
}

.prompt-collapse :deep(.el-collapse-item__wrap) {
  border: 0;
  background: transparent;
}

.prompt-collapse :deep(.el-collapse-item__content) {
  padding: 0 7px 7px;
}

.prompt-group-title {
  width: 100%;
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.prompt-group-title strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--aiops-text-primary);
  font-size: 13px;
  font-weight: 750;
}

.prompt-group-title small {
  flex: 0 0 auto;
  color: var(--aiops-brand-active);
  font-size: 11px;
  font-weight: 700;
}

.prompt-question-list {
  display: grid;
  gap: 6px;
}

.template-item {
  display: block;
  padding: 8px 9px;
  border-radius: 9px;
}

.template-item span {
  display: block;
  color: var(--aiops-text-primary);
  font-size: 13px;
  font-weight: 650;
  line-height: 1.45;
}

.prompt-tip {
  margin-top: 8px;
  padding: 8px 9px;
  border: 1px solid rgba(76, 139, 245, 0.15);
  border-radius: 10px;
  background: rgba(76, 139, 245, 0.07);
}

.prompt-tip summary {
  color: var(--aiops-brand-active);
  font-size: 12px;
  font-weight: 750;
  cursor: pointer;
}

.prompt-tip p {
  margin: 6px 0 0;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.45;
}

.message-viewport {
  flex: 1 1 auto;
  min-height: 0;
  max-height: 100%;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 18px;
}

.message-row {
  display: flex;
  align-items: flex-start;
  margin-bottom: 14px;
}

.message-row.is-user {
  justify-content: flex-end;
}

.message-row.is-user .message-bubble {
  max-width: min(720px, 72%);
  background: rgba(76, 139, 245, 0.09);
  border-color: rgba(76, 139, 245, 0.2);
}

.message-avatar {
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 28px;
  border-radius: 10px;
  background: rgba(255, 248, 238, 0.94);
  color: var(--aiops-brand-active);
  font-size: 13px;
  font-weight: 800;
}

.message-avatar img {
  width: 20px;
  height: 20px;
  display: block;
  object-fit: contain;
}

.message-bubble {
  min-width: 0;
  width: 100%;
  max-width: 100%;
  overflow-y: visible;
  overflow-x: hidden;
  padding: 13px 14px;
  border: 1px solid rgba(223, 231, 242, 0.88);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.76);
}

.message-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 8px;
}

.message-title {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.message-head strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--aiops-text-primary);
  font-size: 13px;
}

.message-head span {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.user-content {
  margin: 0;
  color: var(--aiops-text-primary);
  font-size: 14px;
  line-height: 1.7;
  white-space: pre-wrap;
}

.answer-content {
  color: var(--aiops-text-secondary);
  font-size: 14px;
  line-height: 1.78;
}

.answer-content :deep(p) {
  margin: 6px 0 0;
}

.answer-content :deep(p:first-child) {
  margin-top: 0;
}

.answer-content :deep(h3),
.answer-content :deep(h4),
.answer-content :deep(h5) {
  margin: 14px 0 6px;
  color: var(--aiops-text-primary);
  font-size: 15px;
  font-weight: 750;
}

.answer-content :deep(ol),
.answer-content :deep(ul) {
  margin: 8px 0 0;
  padding-left: 20px;
}

.answer-content :deep(li) {
  margin: 5px 0;
}

.answer-content :deep(code) {
  padding: 2px 6px;
  border-radius: 8px;
  background: rgba(76, 139, 245, 0.08);
  color: var(--aiops-brand-active);
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
}

.answer-content :deep(hr) {
  height: 1px;
  margin: 12px 0;
  border: 0;
  background: rgba(223, 231, 242, 0.95);
}

.answer-content :deep(.markdown-table-wrap) {
  max-width: 100%;
  overflow-x: auto;
  margin: 10px 0 12px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.78);
}

.answer-content :deep(.markdown-table) {
  width: 100%;
  min-width: 620px;
  border-collapse: collapse;
  color: var(--aiops-text-secondary);
  font-size: 13px;
}

.answer-content :deep(.markdown-table th),
.answer-content :deep(.markdown-table td) {
  padding: 8px 10px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.78);
  border-right: 1px solid rgba(223, 231, 242, 0.68);
  text-align: left;
  vertical-align: top;
  line-height: 1.55;
  white-space: normal;
}

.answer-content :deep(.markdown-table th:last-child),
.answer-content :deep(.markdown-table td:last-child) {
  border-right: 0;
}

.answer-content :deep(.markdown-table tr:last-child td) {
  border-bottom: 0;
}

.answer-content :deep(.markdown-table th) {
  background: rgba(247, 250, 255, 0.96);
  color: var(--aiops-text-primary);
  font-weight: 750;
}

.answer-content :deep(.field-table) {
  min-width: 760px;
}

.answer-content :deep(.field-table th:first-child),
.answer-content :deep(.field-table td:first-child) {
  min-width: 150px;
  color: var(--aiops-text-primary);
  font-weight: 700;
}

.answer-content :deep(.field-table td) {
  white-space: nowrap;
}

.answer-content :deep(.script-block) {
  margin: 12px 0;
  overflow: hidden;
  border: 1px solid rgba(47, 59, 78, 0.14);
  border-radius: 12px;
  background: #101827;
}

.answer-content :deep(.script-block-head) {
  min-height: 34px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 0 10px 0 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.05);
}

.answer-content :deep(.script-block-head span) {
  color: #c7d2e5;
  font-size: 12px;
  font-weight: 700;
}

.answer-content :deep(.script-copy-btn) {
  height: 24px;
  padding: 0 9px;
  border: 1px solid rgba(199, 210, 229, 0.28);
  border-radius: 7px;
  background: rgba(255, 255, 255, 0.08);
  color: #e5edf8;
  cursor: pointer;
  font-size: 12px;
  line-height: 22px;
}

.answer-content :deep(.script-copy-btn:hover) {
  border-color: rgba(199, 210, 229, 0.48);
  background: rgba(255, 255, 255, 0.14);
}

.answer-content :deep(.script-block pre) {
  max-height: 360px;
  overflow: auto;
  margin: 0;
  padding: 14px;
}

.answer-content :deep(.script-block pre code) {
  display: block;
  padding: 0;
  border-radius: 0;
  background: transparent;
  color: #edf4ff;
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
  line-height: 1.7;
  white-space: pre;
}

.assistant-progress {
  display: grid;
  gap: 8px;
  margin: 8px 0 10px;
  padding: 10px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  border-radius: 12px;
  background: rgba(247, 250, 255, 0.76);
}

.assistant-progress-head,
.progress-detail-head,
.progress-timeline-title {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.assistant-progress-head span,
.progress-detail-head span {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.assistant-progress-head strong,
.progress-detail-head strong,
.progress-timeline-title strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--aiops-text-primary);
  font-size: 13px;
  font-weight: 760;
}

.assistant-progress :deep(.el-progress-bar__outer),
.progress-detail-bar :deep(.el-progress-bar__outer) {
  height: 6px !important;
  background: rgba(223, 231, 242, 0.9);
}

.assistant-progress-steps {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 8px;
}

.assistant-progress-step {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.35;
}

.assistant-progress-step i,
.progress-timeline-item > i {
  flex: 0 0 auto;
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: #94a3b8;
}

.assistant-progress-step span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.assistant-progress-step.status-running i,
.progress-timeline-item.status-running > i {
  background: #3b82f6;
  box-shadow: 0 0 0 4px rgba(59, 130, 246, 0.12);
  animation: progressPulse 1.25s ease-in-out infinite;
}

.assistant-progress-step.status-succeeded i,
.progress-timeline-item.status-succeeded > i {
  background: #16a34a;
}

.assistant-progress-step.status-failed i,
.progress-timeline-item.status-failed > i {
  background: #dc2626;
}

.assistant-progress-step.status-skipped {
  color: var(--aiops-text-muted);
}

.assistant-progress-step.status-skipped i,
.progress-timeline-item.status-skipped > i {
  border: 1px dashed #94a3b8;
  background: transparent;
}

.progress-detail-card {
  display: grid;
  gap: 10px;
  margin: 10px 0;
  padding: 11px;
  border-radius: 14px;
  border: 1px solid rgba(223, 231, 242, 0.82);
  background: rgba(255, 255, 255, 0.68);
}

.progress-detail-head > div {
  min-width: 0;
  display: grid;
  gap: 2px;
}

.progress-timeline {
  display: grid;
  gap: 8px;
}

.progress-timeline-item {
  display: grid;
  grid-template-columns: 10px minmax(0, 1fr);
  gap: 10px;
  padding: 9px;
  border-radius: 12px;
  border: 1px solid rgba(223, 231, 242, 0.76);
  background: rgba(247, 250, 255, 0.62);
}

.progress-timeline-item.status-skipped {
  border-style: dashed;
  background: rgba(248, 250, 252, 0.58);
}

.progress-timeline-item > i {
  margin-top: 6px;
}

.progress-timeline-title span {
  flex: 0 0 auto;
  padding: 2px 7px;
  border: 1px solid transparent;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 750;
}

.progress-timeline-item p {
  margin: 4px 0 0;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.progress-timeline-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
  margin-top: 6px;
}

.progress-timeline-meta span {
  padding: 2px 6px;
  border-radius: 999px;
  background: rgba(76, 139, 245, 0.07);
  color: var(--aiops-text-muted);
  font-size: 11px;
  line-height: 1.4;
}

.progress-error {
  display: inline-block;
  max-width: 100%;
  margin-top: 6px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #dc2626;
  font-size: 12px;
}

@keyframes progressPulse {
  0%,
  100% {
    transform: scale(1);
    opacity: 0.9;
  }

  50% {
    transform: scale(1.22);
    opacity: 1;
  }
}

:deep(.diagnosis-report) {
  display: grid;
  gap: 10px;
  margin-top: 12px;
  padding: 12px;
  border: 1px solid rgba(223, 231, 242, 0.88);
  border-radius: 12px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.92), rgba(247, 250, 255, 0.78));
}

:deep(.diagnosis-report.compact) {
  margin-top: 10px;
  padding: 11px;
  background: rgba(247, 250, 255, 0.72);
}

:deep(.report-summary) {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 10px;
  align-items: flex-start;
}

:deep(.risk-badge) {
  min-width: 44px;
  padding: 5px 8px;
  border-radius: 999px;
  background: rgba(76, 139, 245, 0.1);
  color: var(--aiops-brand-active);
  font-size: 12px;
  font-weight: 750;
  text-align: center;
}

:deep(.risk-badge.tone-warning),
:deep(.risk-badge.tone-warn) {
  background: rgba(243, 178, 75, 0.16);
  color: #b7791f;
}

:deep(.risk-badge.tone-critical),
:deep(.risk-badge.tone-danger),
:deep(.risk-badge.tone-error) {
  background: rgba(239, 107, 115, 0.14);
  color: #c7565f;
}

:deep(.risk-badge.tone-success),
:deep(.risk-badge.tone-ok),
:deep(.risk-badge.tone-normal) {
  background: rgba(63, 185, 127, 0.12);
  color: #2c8f62;
}

:deep(.report-summary h4),
:deep(.report-summary p) {
  margin: 0;
}

:deep(.report-summary h4) {
  color: var(--aiops-text-primary);
  font-size: 15px;
  font-weight: 760;
}

:deep(.report-summary p) {
  margin-top: 4px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.65;
}

:deep(.report-facts) {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

:deep(.report-facts div) {
  min-width: 0;
  padding: 8px;
  border-radius: 10px;
  border: 1px solid rgba(223, 231, 242, 0.75);
  background: rgba(255, 255, 255, 0.66);
}

:deep(.report-facts span),
:deep(.report-facts strong) {
  display: block;
  min-width: 0;
}

:deep(.report-facts span) {
  color: var(--aiops-text-muted);
  font-size: 11px;
}

:deep(.report-facts strong) {
  margin-top: 4px;
  color: var(--aiops-text-primary);
  font-size: 12px;
  line-height: 1.45;
}

:deep(.report-metrics) {
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.82);
  border-radius: 10px;
}

:deep(.metric-row) {
  display: grid;
  grid-template-columns: minmax(86px, 0.8fr) minmax(80px, 1fr) minmax(120px, 1.2fr);
  gap: 8px;
  padding: 8px 10px;
  border-top: 1px solid rgba(223, 231, 242, 0.72);
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

:deep(.metric-row:first-child) {
  border-top: 0;
}

:deep(.metric-row strong) {
  color: var(--aiops-text-primary);
  font-size: 12px;
}

:deep(.metric-row small) {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

:deep(.metric-row.tone-warning small),
:deep(.metric-row.tone-warning strong) {
  color: #b7791f;
}

:deep(.metric-row.tone-success small) {
  color: #2c8f62;
}

:deep(.missing-evidence) {
  padding: 8px 10px;
  border-radius: 10px;
  background: rgba(76, 139, 245, 0.06);
}

:deep(.missing-evidence summary) {
  color: var(--aiops-brand-active);
  cursor: pointer;
  font-size: 12px;
  font-weight: 750;
}

:deep(.missing-evidence span) {
  display: block;
  margin-top: 6px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.45;
}

:deep(.report-next) {
  display: grid;
  gap: 6px;
}

:deep(.report-next strong) {
  color: var(--aiops-text-primary);
  font-size: 13px;
}

:deep(.report-next span) {
  position: relative;
  padding-left: 13px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.45;
}

:deep(.report-next span::before) {
  content: '';
  position: absolute;
  top: 7px;
  left: 0;
  width: 5px;
  height: 5px;
  border-radius: 999px;
  background: var(--aiops-brand);
}

.typing-indicator {
  display: inline-flex;
  gap: 4px;
  margin-top: 10px;
}

.typing-indicator i {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: rgba(76, 139, 245, 0.48);
}

.error-strip {
  margin-top: 10px;
  padding: 8px 10px;
  border-radius: 12px;
  border: 1px solid rgba(239, 107, 115, 0.28);
  background: rgba(255, 243, 244, 0.86);
  color: #c7565f;
  font-size: 13px;
}

.message-meta,
.suggestion-row,
.tag-row,
.run-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 7px;
}

.message-meta {
  margin-top: 11px;
}

.message-meta span,
.message-meta button,
.suggestion-row button {
  min-height: 26px;
  padding: 0 9px;
  border-radius: 999px;
  border: 1px solid rgba(76, 139, 245, 0.16);
  background: rgba(76, 139, 245, 0.08);
  color: var(--aiops-brand-active);
  font-size: 12px;
  font-weight: 700;
}

.message-meta button,
.suggestion-row button {
  cursor: pointer;
}

.suggestion-row {
  margin-top: 10px;
}

.composer {
  padding: 12px;
  border-top: 1px solid rgba(223, 231, 242, 0.86);
  background: rgba(255, 255, 255, 0.74);
}

.composer-footer {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  margin-top: 10px;
}

.context-controls,
.composer-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.mode-select {
  width: 104px;
}

.context-editor {
  display: grid;
  gap: 2px;
}

.run-grid,
.kv-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.run-grid div,
.kv-grid div {
  min-width: 0;
  padding: 10px;
  border-radius: 14px;
  border: 1px solid rgba(223, 231, 242, 0.82);
  background: rgba(255, 255, 255, 0.66);
}

.run-grid span,
.run-grid strong,
.kv-grid span,
.kv-grid strong {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.run-grid span,
.kv-grid span {
  color: var(--aiops-text-muted);
  font-size: 11px;
}

.run-grid strong,
.kv-grid strong {
  margin-top: 5px;
  color: var(--aiops-text-primary);
  font-size: 14px;
  font-weight: 750;
}

.run-actions {
  margin: 10px 0;
}

.execution-collapse {
  margin-top: 12px;
}

.execution-summary {
  display: grid;
  gap: 6px;
  margin-bottom: 8px;
}

.execution-summary span {
  padding: 7px 9px;
  border-radius: 9px;
  background: rgba(247, 250, 255, 0.78);
  color: var(--aiops-text-secondary);
  font-size: 12px;
  font-weight: 650;
}

.diagnosis-card {
  margin-top: 10px;
  padding: 13px;
  border-radius: 18px;
  border: 1px solid rgba(223, 231, 242, 0.88);
  background: rgba(255, 255, 255, 0.68);
}

.diagnosis-card.tone-warning,
.diagnosis-card.tone-warn {
  border-color: rgba(243, 178, 75, 0.32);
  background: rgba(255, 248, 238, 0.9);
}

.diagnosis-card.tone-critical,
.diagnosis-card.tone-danger,
.diagnosis-card.tone-error {
  border-color: rgba(239, 107, 115, 0.28);
  background: rgba(255, 243, 244, 0.9);
}

.diagnosis-head {
  display: grid;
  grid-template-columns: 32px minmax(0, 1fr);
  gap: 10px;
}

.diagnosis-head .el-icon {
  width: 32px;
  height: 32px;
  border-radius: 12px;
  background: rgba(63, 185, 127, 0.12);
  color: var(--aiops-success);
}

.diagnosis-head h4,
.diagnosis-head span {
  margin: 0;
}

.diagnosis-head h4 {
  color: var(--aiops-text-primary);
  font-size: 15px;
  font-weight: 750;
  line-height: 1.55;
}

.diagnosis-head span {
  display: block;
  margin-top: 4px;
  color: var(--aiops-text-muted);
  font-size: 12px;
  line-height: 1.55;
}

.section-list {
  display: grid;
  gap: 8px;
  margin-top: 12px;
}

.section-list > strong {
  color: var(--aiops-text-primary);
  font-size: 13px;
}

.list-item {
  display: grid;
  gap: 3px;
  padding: 9px 10px;
  border-radius: 13px;
  border: 1px solid rgba(223, 231, 242, 0.7);
  background: rgba(255, 255, 255, 0.58);
}

.list-item span {
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.55;
}

.list-item small {
  color: var(--aiops-text-muted);
  font-size: 11px;
}

.dot-line {
  position: relative;
  padding-left: 15px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.dot-line::before {
  content: '';
  position: absolute;
  top: 8px;
  left: 0;
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: var(--aiops-brand);
}

.missing-collapse {
  margin-top: 10px;
}

.detail-diagnosis-collapse {
  margin-top: 10px;
}

.context-warning {
  display: grid;
  gap: 4px;
  margin-top: 10px;
  padding: 9px 10px;
  border: 1px solid rgba(239, 107, 115, 0.26);
  border-radius: 12px;
  background: rgba(255, 243, 244, 0.86);
}

.context-warning strong {
  color: #c7565f;
  font-size: 13px;
}

.context-warning span {
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}

.reflection-card,
.feedback-card,
.plan-card {
  display: grid;
  gap: 9px;
  margin-top: 10px;
  padding: 11px;
  border-radius: 14px;
  border: 1px solid rgba(223, 231, 242, 0.78);
  background: rgba(255, 255, 255, 0.66);
}

.feedback-card {
  margin-top: 12px;
  background: rgba(247, 250, 255, 0.74);
}

.reflection-card strong,
.feedback-card strong,
.plan-card strong {
  color: var(--aiops-text-primary);
  font-size: 13px;
}

.reflection-card span {
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.55;
}

.reflection-card small {
  color: var(--aiops-text-muted);
  font-size: 11px;
}

.feedback-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
}

.plan-card {
  margin: 0 0 12px;
}

.plan-card-head {
  display: flex;
  justify-content: space-between;
  gap: 8px;
}

.plan-card-head span {
  color: var(--aiops-brand-active);
  font-size: 12px;
  font-weight: 700;
}

.plan-step {
  display: grid;
  grid-template-columns: 24px minmax(0, 1fr);
  gap: 9px;
  padding: 8px;
  border-radius: 10px;
  background: rgba(247, 250, 255, 0.72);
}

.plan-step i {
  width: 24px;
  height: 24px;
  border-radius: 999px;
  background: rgba(76, 139, 245, 0.1);
  color: var(--aiops-brand-active);
  font-size: 12px;
  font-style: normal;
  font-weight: 750;
  line-height: 24px;
  text-align: center;
}

.plan-step.status-succeeded i,
.plan-step.status-success i {
  background: rgba(63, 185, 127, 0.12);
  color: #2c8f62;
}

.plan-step.status-failed i,
.plan-step.status-error i {
  background: rgba(239, 107, 115, 0.14);
  color: #c7565f;
}

.plan-step span {
  display: block;
  margin-top: 3px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.45;
}

.timeline-list {
  display: grid;
  gap: 8px;
}

.timeline-item {
  display: grid;
  grid-template-columns: 10px minmax(0, 1fr);
  gap: 10px;
  padding: 10px;
  border-radius: 14px;
  border: 1px solid rgba(223, 231, 242, 0.76);
  background: rgba(255, 255, 255, 0.62);
}

.timeline-item > i {
  width: 9px;
  height: 9px;
  margin-top: 6px;
  border-radius: 999px;
  background: var(--aiops-brand);
}

.timeline-item.status-failed > i,
.timeline-item.status-error > i {
  background: var(--aiops-danger);
}

.timeline-item.status-succeeded > i,
.timeline-item.status-success > i {
  background: var(--aiops-success);
}

.timeline-item strong,
.timeline-item span,
.timeline-item small {
  display: block;
  min-width: 0;
}

.timeline-item strong {
  color: var(--aiops-text-primary);
  font-size: 13px;
}

.timeline-item span {
  margin-top: 4px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 1.55;
}

.timeline-item small {
  margin-top: 3px;
  color: var(--aiops-text-muted);
  font-size: 11px;
}

.collapse-title {
  width: 100%;
  display: inline-flex;
  align-items: center;
  justify-content: space-between;
  color: var(--aiops-text-primary);
  font-size: 13px;
  font-weight: 750;
}

.collapse-title small {
  min-width: 22px;
  padding: 0 7px;
  border-radius: 999px;
  background: rgba(76, 139, 245, 0.08);
  color: var(--aiops-brand-active);
  line-height: 18px;
  text-align: center;
}

.evidence-item {
  max-height: 220px;
  overflow: auto;
  margin: 0 0 8px;
  padding: 10px;
  border-radius: 12px;
  border: 1px solid rgba(223, 231, 242, 0.74);
  background: rgba(247, 250, 255, 0.68);
  color: var(--aiops-text-secondary);
  font-family: Consolas, "Courier New", monospace;
  font-size: 12px;
  line-height: 1.55;
  white-space: pre-wrap;
}

.assistant-page :deep(.el-button) {
  border-radius: 14px;
  font-weight: 650;
  box-shadow: none !important;
}

.assistant-page :deep(.el-button--primary:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: transparent;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
}

.assistant-page :deep(.el-input__wrapper),
.assistant-page :deep(.el-select__wrapper),
.assistant-page :deep(.el-textarea__inner) {
  border-radius: 16px;
  background: rgba(252, 253, 255, 0.98);
  border: 1px solid transparent;
  box-shadow: inset 0 0 0 1px rgba(214, 223, 235, 0.96) !important;
}

.assistant-page :deep(.el-input__wrapper:hover),
.assistant-page :deep(.el-select__wrapper:hover),
.assistant-page :deep(.el-textarea__inner:hover) {
  box-shadow: inset 0 0 0 1px rgba(171, 189, 220, 0.96) !important;
}

.assistant-page :deep(.el-input__wrapper.is-focus),
.assistant-page :deep(.el-select__wrapper.is-focused),
.assistant-page :deep(.el-textarea__inner:focus) {
  background: #ffffff;
  box-shadow: inset 0 0 0 1px rgba(120, 156, 221, 0.96), 0 0 0 4px rgba(76, 139, 245, 0.08) !important;
}

.assistant-page :deep(.el-tag) {
  border-radius: 999px;
  border-color: transparent;
  font-weight: 650;
}

.assistant-page :deep(.el-tabs__item) {
  color: var(--aiops-text-muted);
  font-weight: 650;
}

.assistant-page :deep(.el-tabs__item.is-active) {
  color: var(--aiops-brand-active);
}

.assistant-page :deep(.el-collapse) {
  --el-collapse-header-bg-color: transparent;
  --el-collapse-content-bg-color: transparent;
  --el-collapse-border-color: transparent;
  border: 0;
}

.assistant-page :deep(.el-collapse-item) {
  margin-bottom: 8px;
  overflow: hidden;
  border: 1px solid rgba(223, 231, 242, 0.78);
  border-radius: 15px;
  background: rgba(255, 255, 255, 0.58);
}

.assistant-page :deep(.el-collapse-item__header) {
  height: 40px;
  padding: 0 12px;
  border-bottom-color: rgba(223, 231, 242, 0.68);
}

.assistant-page :deep(.el-collapse-item__content) {
  padding: 10px 12px 12px;
}

.assistant-card {
  height: calc(100vh - 82px);
  min-height: 560px;
  max-height: calc(100vh - 82px);
  border-radius: 16px;
  box-shadow: 0 10px 28px rgba(17, 24, 39, 0.06);
}

.page-header {
  gap: 12px;
  padding: 10px 14px;
}

.header-title h2 {
  font-size: 20px;
}

.header-title span {
  margin-top: 3px;
  font-size: 12px;
}

.header-actions {
  gap: 6px;
}

.provider-select {
  width: 190px;
}

.status-chip {
  height: 28px;
  padding: 0 10px;
}

.workspace {
  grid-template-columns: 260px minmax(420px, 1fr) 320px;
  gap: 6px;
  padding: 6px;
}

.session-panel,
.insight-panel {
  border-radius: 14px;
  padding: 10px;
}

.chat-panel {
  border-radius: 14px;
  background: rgba(248, 251, 255, 0.72);
}

.panel-title {
  margin-bottom: 8px;
}

.panel-title.compact {
  margin: 12px 0 8px;
}

.session-list {
  gap: 6px;
}

.session-item {
  gap: 3px;
  padding: 8px 9px;
  border-radius: 10px;
}

.template-item {
  margin-top: 6px;
  padding: 8px 9px;
  border-radius: 10px;
}

.message-viewport {
  padding: 10px 12px;
}

.message-row,
.message-row.is-user {
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr);
  gap: 8px;
  align-items: flex-start;
  justify-content: initial;
  margin-bottom: 10px;
}

.message-row.is-user .message-bubble,
.message-bubble {
  width: 100%;
  max-width: 100%;
  padding: 9px 11px;
  border-radius: 10px;
}

.message-row.is-user .message-bubble {
  background: rgba(232, 243, 255, 0.9);
  border-color: rgba(76, 139, 245, 0.22);
}

.message-avatar {
  width: 28px;
  height: 28px;
  flex: 0 0 28px;
  overflow: hidden;
  border: 1px solid rgba(207, 217, 230, 0.86);
  border-radius: 10px;
  background: rgba(248, 251, 255, 0.96);
}

.message-avatar img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.message-head {
  margin-bottom: 5px;
}

.message-title {
  gap: 0;
}

.message-head strong {
  font-size: 12px;
}

.message-head span {
  font-size: 12px;
}

.user-content,
.answer-content {
  font-size: 13px;
  line-height: 1.58;
}

.answer-content :deep(p) {
  margin-top: 4px;
}

.answer-content :deep(h3),
.answer-content :deep(h4),
.answer-content :deep(h5) {
  margin: 8px 0 4px;
  font-size: 13px;
}

.answer-content :deep(ol),
.answer-content :deep(ul) {
  margin-top: 4px;
  padding-left: 18px;
}

.answer-content :deep(li) {
  margin: 2px 0;
}

.message-meta {
  margin-top: 7px;
  gap: 5px;
}

.message-meta span,
.message-meta button,
.suggestion-row button {
  min-height: 22px;
  padding: 0 7px;
  font-size: 11px;
}

.suggestion-row {
  margin-top: 7px;
}

.composer {
  padding: 8px 10px 10px;
  background: rgba(255, 255, 255, 0.82);
}

.composer-footer {
  align-items: center;
  margin-top: 7px;
}

.assistant-page :deep(.el-button) {
  border-radius: 8px;
}

.assistant-page :deep(.el-input__wrapper),
.assistant-page :deep(.el-select__wrapper),
.assistant-page :deep(.el-textarea__inner) {
  border-radius: 7px;
}

.composer :deep(.el-textarea__inner) {
  min-height: 84px !important;
}

@media (max-width: 1500px) {
  :deep(.report-facts) {
    grid-template-columns: 1fr;
  }

  :deep(.metric-row) {
    grid-template-columns: minmax(78px, 0.8fr) minmax(80px, 1fr);
  }

  :deep(.metric-row small) {
    grid-column: 1 / -1;
  }
}

@media (max-width: 1360px) {
  .workspace {
    grid-template-columns: 240px minmax(380px, 1fr);
  }

  .insight-panel {
    grid-column: 1 / -1;
    max-height: 420px;
  }
}

@media (max-width: 980px) {
  .page-header {
    align-items: stretch;
    flex-direction: column;
  }

  .header-actions {
    justify-content: flex-start;
  }

  .workspace {
    grid-template-columns: 1fr;
  }

  .session-panel,
  .insight-panel {
    max-height: none;
  }

  .message-row.is-user .message-bubble,
  .message-bubble {
    max-width: 100%;
  }
}

@media (max-width: 720px) {
  .assistant-page {
    padding: 4px;
  }

  .assistant-card {
    border-radius: 22px;
    height: calc(100vh - 20px);
    min-height: 0;
    max-height: calc(100vh - 20px);
  }

  .page-header {
    padding: 14px;
  }

  .header-title h2 {
    font-size: 24px;
  }

  .provider-select,
  .header-actions :deep(.el-button),
  .composer-actions :deep(.el-button) {
    width: 100%;
  }

  .workspace {
    padding: 6px;
  }

  .message-row,
  .message-row.is-user {
    display: grid;
  }

  .message-row.is-user .message-bubble {
    max-width: 100%;
  }

  .composer-footer {
    flex-direction: column;
    align-items: stretch;
  }

  .context-controls,
  .composer-actions {
    align-items: stretch;
    flex-direction: column;
  }

  .mode-select,
  .run-grid,
  .kv-grid {
    width: 100%;
    grid-template-columns: 1fr;
  }
}

/* DESIGN.md: compact AIOps workbench */
.assistant-page {
  --assistant-blue-500: #409eff;
  --assistant-blue-600: #2563eb;
  --assistant-green-500: #67c23a;
  --assistant-yellow-400: #f3c66b;
  --assistant-red-500: #ef4444;
  --assistant-text-heading: #0f172a;
  --assistant-text-primary: #172033;
  --assistant-text-secondary: #53657d;
  --assistant-text-muted: #64748b;
  --assistant-bg-surface: #fff;
  --assistant-bg-soft: #fbfdff;
  --assistant-bg-subtle: #f3f7fc;
  --assistant-border: #dbe7f4;
  --assistant-border-soft: #e7eef7;
  --assistant-border-hover: #b9d4ff;
  --assistant-card-radius: 12px;
  --assistant-inner-radius: 8px;
  --assistant-control-height: 30px;

  box-sizing: border-box;
  height: calc(100vh - 124px);
  min-height: 0;
  padding: 7px;
  overflow: hidden;
  color: var(--assistant-text-primary);
  background: transparent;
}

:global(.home-container .el-main:has(.assistant-page)) {
  padding: 10px;
  overflow: hidden;
}

.assistant-shell {
  width: 100%;
  height: 100%;
  min-height: 0;
}

.assistant-card {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  min-height: 0;
  max-height: none;
  overflow: hidden;
  border: 1px solid var(--assistant-border);
  border-radius: var(--assistant-card-radius);
  background: var(--assistant-bg-surface);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
}

.page-header {
  display: flex;
  align-items: center;
  flex: 0 0 auto;
  justify-content: space-between;
  gap: 12px;
  min-height: 58px;
  padding: 8px 12px;
  border-bottom: 1px solid var(--assistant-border-soft);
  background: linear-gradient(90deg, #fff6e7 0%, #f7fbff 40%, #ecf7ff 100%);
}

.header-title {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  gap: 9px;
  min-width: 0;
}

.header-assistant-icon {
  display: grid;
  flex: 0 0 42px;
  width: 42px;
  height: 42px;
  place-items: center;
  border: 0;
  background: transparent;
}

.header-assistant-icon img {
  width: 30px;
  height: 30px;
  object-fit: contain;
}

.header-title-copy {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.header-heading-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.header-title h2 {
  flex: 0 0 auto;
  margin: 0;
  color: var(--assistant-text-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: 0;
  white-space: nowrap;
}

.header-title-divider {
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  background: var(--assistant-border);
}

.header-title-copy > p {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  color: var(--assistant-text-muted);
  font-size: 12px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.model-badge {
  display: inline-flex;
  align-items: center;
  flex: 0 0 auto;
  height: 20px;
  padding: 0 7px;
  border: 1px solid #c9eee9;
  border-radius: 999px;
  color: #16897d;
  background: #ecfbf8;
  font-size: 10px;
  font-weight: 600;
}

.header-actions {
  display: flex;
  align-items: center;
  flex: 0 0 auto;
  justify-content: flex-end;
  gap: 6px;
  flex-wrap: nowrap;
}

.provider-select {
  width: 190px;
}

.status-chip {
  display: inline-flex;
  align-items: center;
  height: 26px;
  gap: 6px;
  padding: 0 8px;
  border: 1px solid #cfe0ff;
  border-radius: 999px;
  color: var(--assistant-blue-600);
  background: #eef6ff;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
}

.status-chip i {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: var(--assistant-green-500);
}

.assistant-page :deep(.header-action-btn) {
  height: var(--assistant-control-height);
  min-width: 0;
  padding: 5px 9px;
  border-radius: var(--assistant-inner-radius);
  color: #fff;
  font-size: 11px;
  font-weight: 600;
  box-shadow: none;
  transform: none;
}

.assistant-page :deep(.header-action-btn.warning) {
  border-color: var(--assistant-yellow-400);
  background: var(--assistant-yellow-400);
}

.assistant-page :deep(.header-action-btn.danger) {
  border-color: var(--assistant-red-500);
  background: var(--assistant-red-500);
}

.assistant-page :deep(.header-action-btn:not(.is-disabled):hover),
.assistant-page :deep(.header-action-btn:not(.is-disabled):focus-visible) {
  color: #fff;
  box-shadow: 0 4px 10px rgba(64, 158, 255, 0.1);
  filter: brightness(0.96);
  transform: none;
}

.workspace {
  display: grid;
  grid-template-columns: 250px minmax(500px, 1fr) 310px;
  flex: 1 1 auto;
  gap: 0;
  min-height: 0;
  padding: 0;
  overflow: hidden;
}

.session-panel,
.chat-panel,
.insight-panel {
  min-width: 0;
  min-height: 0;
  border: 0;
  border-radius: 0;
  background: var(--assistant-bg-surface);
  backdrop-filter: none;
}

.session-panel {
  display: flex;
  flex-direction: column;
  padding: 10px;
  overflow: hidden;
  border-right: 1px solid var(--assistant-border-soft);
  background: var(--assistant-bg-soft);
}

.chat-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
  background: #fff;
}

.insight-panel {
  padding: 10px;
  overflow: auto;
  border-left: 1px solid var(--assistant-border-soft);
  background: var(--assistant-bg-soft);
}

.panel-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.panel-title.compact {
  margin: 10px 0 6px;
}

.panel-title h3 {
  margin: 0;
  color: var(--assistant-text-heading);
  font-size: 13px;
  font-weight: 700;
}

.panel-title p {
  margin: 2px 0 0;
  color: var(--assistant-text-muted);
  font-size: 10.5px;
}

.session-actions {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.assistant-page :deep(.session-action-btn) {
  width: 28px;
  height: 28px;
  border: 0;
  border-radius: 7px;
  color: #fff;
  box-shadow: none;
  transform: none;
}

.assistant-page :deep(.session-action-btn.success) {
  background: var(--assistant-green-500);
}

.assistant-page :deep(.session-action-btn.danger) {
  background: var(--assistant-red-500);
}

.assistant-page :deep(.session-action-btn:not(.is-disabled):hover) {
  color: #fff;
  filter: brightness(0.96);
  box-shadow: none;
  transform: none;
}

.session-list {
  display: grid;
  flex: 0 1 auto;
  gap: 2px;
  max-height: 42%;
  padding: 0 2px 0 0;
  overflow: auto;
}

.session-item {
  display: grid;
  gap: 2px;
  width: 100%;
  padding: 7px 8px;
  border: 0;
  border-left: 3px solid transparent;
  border-radius: 6px;
  color: var(--assistant-text-primary);
  background: transparent;
  text-align: left;
  cursor: pointer;
  transition: border-color 160ms ease, background-color 160ms ease;
}

.session-item:hover {
  border-color: #cfe0ff;
  background: #f3f7fc;
  transform: none;
}

.session-item.active {
  border-color: var(--assistant-blue-500);
  background: #eef6ff;
}

.session-item span,
.session-item strong,
.session-item small {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-item span {
  color: var(--assistant-text-heading);
  font-size: 12px;
  font-weight: 600;
}

.session-item strong {
  color: var(--assistant-blue-600);
  font-size: 10.5px;
  font-weight: 600;
}

.session-item small {
  color: var(--assistant-text-muted);
  font-size: 10px;
}

.template-panel {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  margin-top: 8px;
  overflow: hidden;
  border-top: 1px solid var(--assistant-border-soft);
}

.prompt-intro {
  margin: 0 0 6px;
  color: var(--assistant-text-secondary);
  font-size: 10.5px;
  line-height: 1.4;
}

.prompt-collapse {
  min-height: 0;
  padding-right: 2px;
  overflow: auto;
}

.prompt-collapse :deep(.el-collapse-item),
.assistant-page :deep(.el-collapse-item) {
  margin-bottom: 5px;
  overflow: hidden;
  border: 1px solid var(--assistant-border-soft);
  border-radius: var(--assistant-inner-radius);
  background: #fff;
}

.prompt-collapse :deep(.el-collapse-item__header) {
  height: 32px;
  padding: 0 8px;
  border: 0;
  background: transparent;
  line-height: 32px;
}

.prompt-collapse :deep(.el-collapse-item__content) {
  padding: 0 6px 6px;
}

.prompt-group-title strong {
  color: var(--assistant-text-primary);
  font-size: 11px;
  font-weight: 700;
}

.prompt-group-title small {
  color: var(--assistant-blue-600);
  font-size: 9.5px;
}

.prompt-question-list {
  display: grid;
  gap: 4px;
}

.template-item {
  width: 100%;
  margin: 0;
  padding: 6px 7px;
  border: 1px solid var(--assistant-border-soft);
  border-radius: 6px;
  background: var(--assistant-bg-soft);
  text-align: left;
  cursor: pointer;
  transition: border-color 160ms ease, background-color 160ms ease;
}

.template-item:hover {
  border-color: var(--assistant-border-hover);
  background: #eef6ff;
  transform: none;
}

.template-item span {
  color: var(--assistant-text-primary);
  font-size: 11px;
  font-weight: 500;
  line-height: 1.4;
}

.prompt-tip {
  margin-top: 6px;
  padding: 6px 7px;
  border: 1px solid #cfe0ff;
  border-radius: 6px;
  background: #eef6ff;
}

.prompt-tip summary {
  color: var(--assistant-blue-600);
  font-size: 10.5px;
  font-weight: 600;
}

.prompt-tip p {
  margin: 4px 0 0;
  color: var(--assistant-text-secondary);
  font-size: 10px;
  line-height: 1.4;
}

.message-viewport {
  flex: 1 1 auto;
  min-height: 0;
  max-height: none;
  padding: 12px 14px;
  overflow-x: hidden;
  overflow-y: auto;
  background: #fff;
}

.message-row,
.message-row.is-user {
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr);
  align-items: flex-start;
  gap: 8px;
  margin-bottom: 10px;
}

.message-avatar {
  display: grid;
  width: 28px;
  height: 28px;
  flex: 0 0 28px;
  overflow: hidden;
  place-items: center;
  border: 1px solid var(--assistant-border-soft);
  border-radius: 7px;
  background: var(--assistant-bg-soft);
}

.message-avatar img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.message-bubble,
.message-row.is-user .message-bubble {
  width: 100%;
  max-width: 100%;
  min-width: 0;
  padding: 8px 10px;
  overflow-x: hidden;
  border: 1px solid var(--assistant-border-soft);
  border-radius: var(--assistant-inner-radius);
  background: #fff;
}

.message-row.is-assistant .message-bubble {
  padding: 2px 6px 10px;
  border-color: transparent;
  background: transparent;
}

.message-row.is-user .message-bubble {
  border-color: #cfe0ff;
  background: #eef6ff;
}

.message-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 5px;
}

.message-head strong {
  color: var(--assistant-text-heading);
  font-size: 11px;
  font-weight: 700;
}

.message-head span {
  color: var(--assistant-text-muted);
  font-size: 10px;
}

.user-content,
.answer-content {
  color: var(--assistant-text-secondary);
  font-size: 12.5px;
  line-height: 1.65;
}

.answer-content :deep(h3),
.answer-content :deep(h4),
.answer-content :deep(h5) {
  margin: 8px 0 4px;
  color: var(--assistant-text-heading);
  font-size: 13px;
  font-weight: 700;
}

.answer-content :deep(p),
.answer-content :deep(ol),
.answer-content :deep(ul) {
  margin-top: 4px;
}

.answer-content :deep(li) {
  margin: 2px 0;
}

.answer-content :deep(code) {
  border-radius: 5px;
  color: var(--assistant-blue-600);
  background: #eef6ff;
}

.answer-content :deep(.markdown-table-wrap),
.answer-content :deep(.script-block) {
  border-radius: var(--assistant-inner-radius);
}

.assistant-progress,
.progress-detail-card,
.progress-timeline-item,
:deep(.diagnosis-report),
.diagnosis-card,
.list-item,
.context-warning,
.reflection-card,
.feedback-card,
.plan-card,
.plan-step,
.timeline-item,
.evidence-item,
.run-grid div,
.kv-grid div {
  border-radius: var(--assistant-inner-radius);
  box-shadow: none;
}

.message-meta,
.suggestion-row {
  gap: 5px;
  margin-top: 7px;
}

.message-meta span,
.message-meta button,
.suggestion-row button {
  min-height: 22px;
  padding: 0 7px;
  border-radius: 999px;
  font-size: 10px;
}

.composer {
  flex: 0 0 auto;
  padding: 8px 10px 9px;
  border-top: 1px solid var(--assistant-border-soft);
  background: #fff;
}

.composer :deep(.el-textarea__inner) {
  min-height: 76px !important;
  padding: 8px 10px;
  border-radius: var(--assistant-inner-radius);
  font-size: 12px;
  line-height: 1.5;
}

.composer-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-top: 6px;
}

.context-controls,
.composer-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: nowrap;
}

.mode-select {
  width: 96px;
}

.assistant-page :deep(.el-button) {
  border-radius: var(--assistant-inner-radius);
  font-weight: 600;
  box-shadow: none;
  transform: none;
}

.assistant-page :deep(.composer .el-button),
.assistant-page :deep(.context-controls .el-button) {
  min-height: var(--assistant-control-height);
  font-size: 11px;
}

.assistant-page :deep(.el-button--primary:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: var(--assistant-blue-500);
  color: #fff;
  background: var(--assistant-blue-500);
}

.assistant-page :deep(.el-button:not(.is-disabled):hover) {
  transform: none;
}

.assistant-page :deep(.el-input__wrapper),
.assistant-page :deep(.el-select__wrapper),
.assistant-page :deep(.el-textarea__inner) {
  border: 0;
  border-radius: var(--assistant-inner-radius);
  background: var(--assistant-bg-soft);
  box-shadow: 0 0 0 1px #d9e5f2 inset !important;
  transition: background-color 160ms ease, box-shadow 160ms ease;
}

.assistant-page :deep(.el-input__wrapper:hover),
.assistant-page :deep(.el-select__wrapper:hover),
.assistant-page :deep(.el-textarea__inner:hover) {
  box-shadow: 0 0 0 1px var(--assistant-border-hover) inset !important;
}

.assistant-page :deep(.el-input__wrapper.is-focus),
.assistant-page :deep(.el-select__wrapper.is-focused),
.assistant-page :deep(.el-textarea__inner:focus) {
  background: #fff;
  box-shadow: 0 0 0 1px var(--assistant-blue-500) inset !important;
}

.assistant-page :deep(.el-collapse-item__header) {
  min-height: 34px;
  height: auto;
  padding: 0 9px;
  border-bottom-color: var(--assistant-border-soft);
  font-size: 11px;
  line-height: 1.4;
}

.assistant-page :deep(.el-collapse-item__content) {
  padding: 8px 9px 9px;
}

.insight-panel .panel-title {
  position: sticky;
  top: -10px;
  z-index: 2;
  margin: -10px -10px 8px;
  padding: 9px 10px 8px;
  border-bottom: 1px solid var(--assistant-border-soft);
  background: rgba(251, 253, 255, 0.96);
}

@media (max-width: 1500px) {
  .workspace {
    grid-template-columns: 230px minmax(440px, 1fr) 290px;
  }

  .provider-select {
    width: 168px;
  }
}

@media (max-width: 1360px) {
  .workspace {
    grid-template-columns: 220px minmax(420px, 1fr) 280px;
  }

  .insight-panel {
    grid-column: auto;
    max-height: none;
  }

  .header-title-copy > p,
  .header-title-divider,
  .status-chip {
    display: none;
  }
}

@media (max-width: 1080px) {
  .assistant-page {
    height: auto;
    overflow: visible;
  }

  .assistant-card {
    height: auto;
    min-height: calc(100vh - 124px);
  }

  .page-header {
    align-items: stretch;
    flex-direction: column;
  }

  .header-actions {
    justify-content: flex-start;
  }

  .workspace {
    grid-template-columns: 220px minmax(0, 1fr);
    overflow: visible;
  }

  .chat-panel {
    min-height: 680px;
  }

  .insight-panel {
    grid-column: 1 / -1;
    max-height: 480px;
    border-top: 1px solid var(--assistant-border-soft);
    border-left: 0;
  }
}

@media (max-width: 720px) {
  .assistant-page {
    padding: 6px;
  }

  .assistant-card {
    min-height: 0;
    border-radius: var(--assistant-card-radius);
  }

  .page-header {
    padding: 8px 10px;
  }

  .header-title-copy {
    align-items: flex-start;
    flex-direction: column;
    gap: 2px;
  }

  .header-title-copy > p {
    display: block;
    white-space: normal;
  }

  .header-title h2 {
    font-size: 17px;
  }

  .header-actions,
  .provider-select,
  .header-actions :deep(.el-button) {
    width: 100%;
  }

  .header-actions {
    align-items: stretch;
    flex-direction: column;
  }

  .workspace {
    grid-template-columns: 1fr;
  }

  .session-panel {
    max-height: 420px;
    border-right: 0;
    border-bottom: 1px solid var(--assistant-border-soft);
  }

  .chat-panel {
    min-height: 620px;
  }

  .message-row,
  .message-row.is-user {
    grid-template-columns: 24px minmax(0, 1fr);
    gap: 6px;
  }

  .message-avatar {
    width: 24px;
    height: 24px;
  }

  .composer-footer,
  .context-controls,
  .composer-actions {
    align-items: stretch;
    flex-direction: column;
  }

  .context-controls,
  .composer-actions,
  .mode-select,
  .composer-actions :deep(.el-button) {
    width: 100%;
  }
}

/* Reference conversation layout: flat history rail + continuous chat canvas */
.assistant-page {
  height: calc(100vh - 124px);
  padding: 7px;
  overflow: hidden;
  background: transparent;
}

.assistant-shell,
.assistant-card {
  width: 100%;
  height: 100%;
  min-height: 0;
}

.assistant-card {
  position: relative;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid #dbe7f4;
  border-radius: 16px;
  background: #fff;
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.06);
}

.page-header {
  display: flex;
  align-items: center;
  flex-direction: row;
  flex: 0 0 54px;
  justify-content: space-between;
  min-height: 54px;
  padding: 0 14px;
  border-bottom: 1px solid #dbe7f4;
  background: linear-gradient(90deg, #fff9ef 0%, #f8fbff 42%, #eef8ff 100%);
}

.header-title {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  gap: 9px;
  min-width: 0;
}

.header-assistant-icon {
  display: grid;
  flex: 0 0 34px;
  width: 34px;
  height: 34px;
  place-items: center;
  border: 0;
  background: transparent;
}

.header-assistant-icon img {
  width: 28px;
  height: 28px;
  object-fit: contain;
}

.header-title h2 {
  flex: 0 0 auto;
  margin: 0;
  color: #172033;
  font-size: 15px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: 0;
  white-space: nowrap;
}

.header-title > p {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  color: #64748b;
  font-size: 11px;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.header-actions {
  display: none;
}

.provider-select {
  flex: 0 0 150px;
  width: 150px;
}

.assistant-page :deep(.provider-select .el-select__wrapper) {
  min-height: 24px;
  padding: 0 8px;
  border: 1px solid #cdeee9;
  border-radius: 12px;
  background: #effbf9;
  box-shadow: none !important;
}

.assistant-page :deep(.provider-select .el-select__selected-item),
.assistant-page :deep(.provider-select .el-select__placeholder) {
  color: #16897d;
  font-size: 10px;
  font-weight: 600;
}

.workspace {
  position: relative;
  display: grid;
  grid-template-columns: 280px minmax(0, 1fr);
  flex: 1 1 auto;
  gap: 0;
  min-height: 0;
  padding: 0;
  overflow: hidden;
}

.session-panel {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  padding: 0;
  overflow: hidden;
  border: 0;
  border-right: 1px solid #dbe7f4;
  border-radius: 0;
  background: #f7f9fc;
}

.session-panel > .panel-title {
  display: flex;
  align-items: center;
  flex: 0 0 48px;
  justify-content: space-between;
  gap: 8px;
  min-height: 48px;
  margin: 0;
  padding: 0 12px 0 14px;
  border-bottom: 1px solid #dbe7f4;
}

.session-panel > .panel-title h3 {
  color: #172033;
  font-size: 13px;
  font-weight: 700;
}

.session-actions {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.assistant-page :deep(.session-action-btn) {
  width: 26px;
  height: 26px;
  border: 1px solid #d6e2ef;
  border-radius: 7px;
  color: #7b8aa0;
  background: #fff;
  box-shadow: none;
}

.assistant-page :deep(.session-action-btn.danger),
.assistant-page :deep(.session-action-btn.danger:hover) {
  border-color: #d6e2ef;
  color: #7b8aa0;
  background: #fff;
}

.assistant-page :deep(.session-new-button) {
  height: 28px;
  padding: 0 10px;
  border: 1px solid #c7dbf2;
  border-radius: 7px;
  color: #2563eb;
  background: #fff;
  font-size: 11px;
  font-weight: 600;
  box-shadow: none;
}

.assistant-page :deep(.session-new-button:hover) {
  border-color: #8fbcf2;
  color: #2563eb;
  background: #f4f9ff;
}

.session-list {
  display: flex;
  flex: 0 1 auto;
  flex-direction: column;
  gap: 2px;
  min-height: 54px;
  max-height: 190px;
  padding: 8px 9px 12px;
  overflow-x: hidden;
  overflow-y: auto;
  scrollbar-color: #c9d6e5 transparent;
  scrollbar-width: thin;
}

.session-list::-webkit-scrollbar,
.message-viewport::-webkit-scrollbar,
.insight-panel::-webkit-scrollbar {
  width: 5px;
}

.session-list::-webkit-scrollbar-thumb,
.message-viewport::-webkit-scrollbar-thumb,
.insight-panel::-webkit-scrollbar-thumb {
  border-radius: 5px;
  background: #c9d6e5;
}

.session-item {
  display: flex;
  align-items: center;
  flex: 0 0 36px;
  width: 100%;
  height: 36px;
  min-height: 36px;
  padding: 0 11px;
  overflow: hidden;
  border: 0;
  border-left: 0;
  border-radius: 10px;
  color: #25364d;
  background: transparent;
  text-align: left;
  cursor: pointer;
  transition: color 160ms ease, background-color 160ms ease, box-shadow 160ms ease;
}

.session-item:hover {
  border-color: transparent;
  color: #172033;
  background: #edf3f9;
  transform: none;
}

.session-item.active {
  border-color: transparent;
  color: #172033;
  background: #fff;
  box-shadow: 0 3px 10px rgba(80, 124, 180, 0.08);
}

.session-item span {
  min-width: 0;
  overflow: hidden;
  color: inherit;
  font-size: 12px;
  font-weight: 600;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.session-item strong,
.session-item small {
  display: none;
}

.template-panel {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
  margin: 0;
  padding: 8px 9px 9px;
  overflow: hidden;
  border-top: 1px solid #dbe7f4;
  background: #fbfdff;
}

.prompt-panel-heading {
  display: flex;
  align-items: center;
  flex: 0 0 auto;
  justify-content: space-between;
  margin-bottom: 4px;
  padding: 0 2px;
}

.prompt-panel-heading h3 {
  margin: 0;
  color: #172033;
  font-size: 13px;
  font-weight: 700;
}

.prompt-panel-heading p {
  margin: 2px 0 0;
  color: #7b8aa0;
  font-size: 10px;
  line-height: 1.35;
}

.prompt-intro {
  flex: 0 0 auto;
  margin: 0 2px 7px;
  color: #64748b;
  font-size: 10.5px;
  line-height: 1.45;
}

.prompt-collapse {
  flex: 1 1 auto;
  min-height: 0;
  padding-right: 3px;
  overflow-x: hidden;
  overflow-y: auto;
  border: 0;
  scrollbar-color: #c9d6e5 transparent;
  scrollbar-width: thin;
}

.prompt-collapse::-webkit-scrollbar {
  width: 5px;
}

.prompt-collapse::-webkit-scrollbar-thumb {
  border-radius: 5px;
  background: #c9d6e5;
}

.assistant-page :deep(.prompt-collapse .el-collapse-item) {
  margin: 0 0 6px;
  overflow: hidden;
  border: 1px solid #dce8f5;
  border-radius: 10px;
  background: #fff;
}

.assistant-page :deep(.prompt-collapse .el-collapse-item__header) {
  height: 38px;
  min-height: 38px;
  padding: 0 10px;
  border: 0;
  color: #25364d;
  background: #fff;
  line-height: 38px;
}

.assistant-page :deep(.prompt-collapse .el-collapse-item__header:hover) {
  color: #2563eb;
  background: #f7faff;
}

.assistant-page :deep(.prompt-collapse .el-collapse-item__wrap) {
  border: 0;
  background: #fff;
}

.assistant-page :deep(.prompt-collapse .el-collapse-item__content) {
  padding: 0 8px 8px;
}

.prompt-group-title {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
  padding-right: 6px;
}

.prompt-group-title strong {
  overflow: hidden;
  color: inherit;
  font-size: 11.5px;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.prompt-group-title small {
  flex: 0 0 auto;
  color: #2563eb;
  font-size: 9.5px;
  font-weight: 600;
}

.prompt-question-list {
  display: grid;
  gap: 5px;
}

.template-item {
  width: 100%;
  min-height: 36px;
  margin: 0;
  padding: 7px 9px;
  border: 1px solid #dce8f5;
  border-radius: 8px;
  color: #25364d;
  background: #fff;
  text-align: left;
  cursor: pointer;
  transition: border-color 160ms ease, color 160ms ease, background-color 160ms ease;
}

.template-item:hover,
.template-item:focus-visible {
  border-color: #9fc5f1;
  color: #2563eb;
  background: #f5f9ff;
  outline: none;
  transform: none;
}

.template-item span {
  display: block;
  color: inherit;
  font-size: 11px;
  font-weight: 600;
  line-height: 1.45;
}

.prompt-tip {
  flex: 0 0 auto;
  width: 100%;
  max-height: 100%;
  margin-top: auto;
  padding: 6px 8px;
  overflow: hidden;
  border: 1px solid #cfe0ff;
  border-radius: 8px;
  background: #eef5ff;
}

.prompt-tip[open] {
  display: flex;
  height: 100%;
  flex-direction: column;
  margin-top: 0;
}

.prompt-tip summary {
  flex: 0 0 auto;
  color: #2563eb;
  font-size: 10.5px;
  font-weight: 600;
  cursor: pointer;
}

.prompt-tip p {
  flex: 0 0 auto;
  margin: 5px 0 0;
  color: #53657d;
  font-size: 10px;
  line-height: 1.45;
}

.prompt-tip-list {
  display: grid;
  flex: 1 1 auto;
  gap: 5px;
  min-height: 0;
  margin-top: 7px;
  padding-right: 3px;
  overflow-x: hidden;
  overflow-y: auto;
  scrollbar-color: #c9d6e5 transparent;
  scrollbar-width: thin;
}

.prompt-tip-list::-webkit-scrollbar {
  width: 5px;
}

.prompt-tip-list::-webkit-scrollbar-thumb {
  border-radius: 5px;
  background: #c9d6e5;
}

.chat-panel {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  border: 0;
  border-radius: 0;
  background: #fff;
}

.conversation-toolbar {
  display: flex;
  align-items: center;
  flex: 0 0 44px;
  justify-content: space-between;
  gap: 12px;
  min-height: 44px;
  padding: 0 12px;
  border-bottom: 1px solid #dbe7f4;
  background: #fff;
}

.conversation-context {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  gap: 8px;
  min-width: 0;
}

.conversation-context > strong {
  max-width: min(440px, 44vw);
  overflow: hidden;
  color: #25364d;
  font-size: 12px;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.assistant-page :deep(.environment-button) {
  flex: 0 0 auto;
  height: 24px;
  padding: 0 8px;
  border: 1px solid #ffd4a8;
  border-radius: 7px;
  color: #d97706;
  background: #fff7ed;
  font-size: 10px;
  font-weight: 600;
}

.assistant-page :deep(.environment-button.is-bound) {
  border-color: #bfe7dd;
  color: #16897d;
  background: #effbf9;
}

.mode-label {
  flex: 0 0 auto;
  color: #64748b;
  font-size: 10px;
  white-space: nowrap;
}

.mode-select {
  flex: 0 0 76px;
  width: 76px;
}

.assistant-page :deep(.conversation-toolbar .mode-select .el-select__wrapper) {
  min-height: 24px;
  padding: 0 7px;
  border-radius: 7px;
  background: #f7f9fc;
  box-shadow: 0 0 0 1px #d8e4f0 inset !important;
}

.readonly-note {
  flex: 0 0 auto;
  color: #7b8aa0;
  font-size: 10.5px;
  white-space: nowrap;
}

.message-viewport {
  flex: 1 1 auto;
  min-height: 0;
  max-height: none;
  padding: 10px 12px 14px;
  overflow-x: hidden;
  overflow-y: auto;
  background: #f8fbff;
  scrollbar-color: #c9d6e5 transparent;
  scrollbar-width: thin;
}

.message-row,
.message-row.is-user {
  display: block;
  width: 100%;
  margin: 0 0 10px;
}

.message-avatar {
  display: none;
}

.message-bubble,
.message-row.is-assistant .message-bubble,
.message-row.is-user .message-bubble {
  width: 100%;
  max-width: 100%;
  min-width: 0;
  padding: 10px 12px;
  overflow-x: hidden;
  border: 1px solid #dce8f5;
  border-radius: 12px;
  background: #fff;
  box-shadow: 0 4px 12px rgba(80, 124, 180, 0.05);
}

.message-row.is-user .message-bubble {
  width: fit-content;
  max-width: min(72%, 820px);
  margin-left: auto;
  border-color: #cfe2f8;
  border-radius: 12px 12px 4px 12px;
  background: #eef6ff;
  box-shadow: none;
}

.message-row.requires-confirmation .message-bubble {
  border-color: #fdba74;
  background: #fffdfa;
}

.message-head {
  display: none;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 22px;
  margin: 0 0 5px;
}

.message-head strong {
  color: #172033;
  font-size: 11px;
  font-weight: 700;
}

.message-head > span:last-child {
  color: #94a3b8;
  font-size: 9.5px;
}

.user-content,
.answer-content {
  color: #35465e;
  font-size: 12px;
  line-height: 1.65;
}

.answer-content :deep(pre) {
  max-height: 320px;
  overflow: auto;
  border-radius: 8px;
}

.assistant-progress,
.progress-detail-card,
.diagnosis-card,
:deep(.diagnosis-report),
.plan-card,
.feedback-card,
.context-warning,
.reflection-card {
  border-radius: 8px;
  box-shadow: none;
}

.message-meta {
  display: flex;
  align-items: center;
  gap: 5px;
  margin-top: 8px;
  padding-top: 7px;
  border-top: 1px dashed #e2ebf5;
}

.message-meta span,
.message-meta button,
.suggestion-row button {
  min-height: 22px;
  padding: 0 7px;
  border-radius: 6px;
  font-size: 10px;
}

.composer {
  flex: 0 0 auto;
  padding: 8px 12px 7px;
  border-top: 1px solid #dbe7f4;
  background: #fff;
}

.composer :deep(.el-textarea__inner) {
  min-height: 52px !important;
  max-height: 116px;
  padding: 8px 10px 18px;
  border: 1px solid #409eff;
  border-radius: 4px;
  color: #25364d;
  background: #fff;
  font-size: 11px;
  line-height: 1.5;
  box-shadow: none !important;
}

.composer :deep(.el-textarea__inner:hover),
.composer :deep(.el-textarea__inner:focus) {
  border-color: #409eff;
  background: #fff;
  box-shadow: 0 0 0 1px rgba(64, 158, 255, 0.08) !important;
}

.composer :deep(.el-input__count) {
  right: 8px;
  bottom: 4px;
  color: #94a3b8;
  background: transparent;
  font-size: 9px;
}

.composer-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 28px;
  margin-top: 3px;
}

.composer-tip {
  color: #7b8aa0;
  font-size: 9.5px;
}

.composer-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.assistant-page :deep(.composer-actions .el-button) {
  height: 28px;
  min-height: 28px;
  padding: 0 11px;
  border-radius: 7px;
  font-size: 11px;
}

.assistant-page :deep(.composer-actions .el-button:first-child) {
  border-color: #d7e3ef;
  color: #53657d;
  background: #f8fafc;
}

.assistant-page :deep(.composer-actions .el-button--primary) {
  border-color: #2563eb;
  color: #fff;
  background: #2563eb;
}

.assistant-page :deep(.composer-actions .el-button--primary.is-disabled) {
  border-color: #8fc5ff;
  color: #fff;
  background: #8fc5ff;
  opacity: 1;
}

.insight-backdrop {
  position: absolute;
  inset: 0;
  z-index: 10;
  width: 100%;
  height: 100%;
  padding: 0;
  border: 0;
  background: rgba(30, 48, 72, 0.12);
  cursor: default;
}

.insight-panel {
  position: absolute;
  inset: 0 0 0 auto;
  z-index: 11;
  display: block;
  width: min(440px, calc(100% - 24px));
  min-width: 0;
  min-height: 0;
  padding: 0 12px 14px;
  overflow-x: hidden;
  overflow-y: auto;
  border: 0;
  border-left: 1px solid #dbe7f4;
  border-radius: 0;
  background: #fbfdff;
  box-shadow: -14px 0 30px rgba(55, 91, 135, 0.12);
  scrollbar-color: #c9d6e5 transparent;
  scrollbar-width: thin;
}

.insight-panel > .panel-title,
.insight-panel .panel-title {
  position: sticky;
  top: 0;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 48px;
  margin: 0 -12px 10px;
  padding: 7px 12px;
  border-bottom: 1px solid #dbe7f4;
  background: rgba(251, 253, 255, 0.98);
}

.insight-actions {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.assistant-page :deep(.insight-actions .el-button) {
  width: 28px;
  height: 28px;
  border-radius: 7px;
}

@media (max-width: 1200px) {
  .workspace {
    grid-template-columns: 240px minmax(0, 1fr);
  }

  .header-title > p,
  .readonly-note {
    display: none;
  }

  .conversation-context > strong {
    max-width: 42vw;
  }
}

@media (max-width: 760px) {
  .assistant-page {
    height: auto;
    min-height: calc(100vh - 124px);
    padding: 5px;
    overflow: visible;
  }

  .assistant-card {
    min-height: calc(100vh - 134px);
    border-radius: 12px;
  }

  .page-header {
    padding: 0 10px;
  }

  .provider-select {
    flex-basis: 120px;
    width: 120px;
  }

  .workspace {
    grid-template-columns: 1fr;
    overflow: visible;
  }

  .session-panel {
    max-height: 440px;
    border-right: 0;
    border-bottom: 1px solid #dbe7f4;
  }

  .session-list {
    max-height: 120px;
  }

  .template-panel {
    min-height: 250px;
  }

  .chat-panel {
    min-height: 620px;
  }

  .conversation-toolbar {
    align-items: flex-start;
    height: auto;
    min-height: 44px;
    padding: 7px 10px;
  }

  .conversation-context {
    flex-wrap: wrap;
  }

  .conversation-context > strong {
    width: 100%;
    max-width: none;
  }

  .message-row.is-user .message-bubble {
    max-width: 92%;
  }

  .composer-footer {
    align-items: stretch;
    flex-direction: row;
  }

  .composer-actions,
  .composer-actions :deep(.el-button) {
    width: auto;
  }
}

/* Conversation hierarchy: source line, primary answer card, secondary execution detail */
.message-viewport {
  padding: 16px 20px 18px;
  background: #f7faff;
}

.message-row,
.message-row.is-user {
  margin-bottom: 16px;
}

.message-origin {
  display: flex;
  align-items: center;
  gap: 7px;
  margin: 0 0 6px 12px;
  color: #8a9ab0;
  font-size: 10px;
  line-height: 1.3;
}

.message-origin span:first-child {
  color: #64748b;
  font-weight: 600;
}

.message-row.is-assistant .message-bubble {
  width: min(1140px, calc(100% - 24px));
  max-width: 100%;
  margin-left: 12px;
  padding: 13px 15px 12px;
  border-color: #dde8f4;
  border-radius: 12px;
  background: #fff;
  box-shadow: 0 5px 16px rgba(80, 124, 180, 0.05);
}

.message-row.is-user .message-bubble {
  max-width: min(76%, 760px);
  margin-right: 8px;
  padding: 9px 12px;
  border-color: #cfe2f8;
  border-radius: 12px 12px 4px 12px;
  background: #edf6ff;
  color: #35465e;
}

.message-row.requires-confirmation .message-bubble {
  border-color: #fdc98e;
  background: #fffdfa;
}

.assistant-progress {
  margin: 0 0 12px;
  padding: 8px 10px;
  border: 1px solid #e2eaf3;
  border-radius: 9px;
  background: #f8fafc;
}

.assistant-progress-head {
  min-height: 18px;
  margin-bottom: 5px;
}

.assistant-progress-head span {
  color: #53657d;
  font-size: 11px;
  font-weight: 700;
}

.assistant-progress-head strong {
  color: #2563eb;
  font-size: 11px;
}

.assistant-progress :deep(.el-progress-bar__outer) {
  height: 5px !important;
  background: #e8eef6;
}

.assistant-progress-steps {
  gap: 8px;
  margin-top: 7px;
}

.assistant-progress-step {
  min-width: 0;
  padding: 0;
  border: 0;
  background: transparent;
}

.assistant-progress-step span {
  color: #64748b;
  font-size: 10px;
}

.answer-content {
  color: #35465e;
  font-size: 12.5px;
  line-height: 1.72;
}

.answer-content :deep(p:first-child) {
  margin-top: 0;
}

.answer-content :deep(h3),
.answer-content :deep(h4),
.answer-content :deep(h5) {
  margin-top: 12px;
  color: #25364d;
  font-size: 13px;
}

.answer-content :deep(code) {
  padding: 1px 4px;
  border-radius: 4px;
  color: #2563eb;
  background: #edf5ff;
}

.message-meta {
  gap: 6px;
  margin-top: 10px;
  padding-top: 8px;
  border-top-style: solid;
  border-top-color: #e9eff6;
}

.message-meta span,
.message-meta button,
.suggestion-row button {
  min-height: 22px;
  border-radius: 6px;
  font-size: 10px;
  font-weight: 600;
}

.message-meta button {
  color: #2563eb;
  background: #f3f8ff;
}

.suggestion-row {
  gap: 6px;
  margin-top: 9px;
}

.user-content {
  margin: 0;
  color: #35465e;
  font-size: 12px;
  line-height: 1.55;
}

.thinking-panel {
  margin: 0 0 10px;
  overflow: hidden;
  border: 1px solid #dce7f3;
  border-radius: 8px;
  background: #f9fbfe;
}

.thinking-panel.status-running {
  border-color: #b9d3f3;
  box-shadow: inset 3px 0 0 #409eff;
}

.thinking-panel.status-succeeded {
  border-color: #c7e4d2;
  box-shadow: inset 3px 0 0 #2ea868;
}

.thinking-panel.status-failed {
  border-color: #efc8c5;
  box-shadow: inset 3px 0 0 #d94c45;
}

.thinking-summary {
  width: 100%;
  min-height: 43px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 11px;
  border: 0;
  color: #25364d;
  background: transparent;
  cursor: pointer;
  text-align: left;
}

.thinking-summary:hover {
  background: #f1f6fc;
}

.thinking-summary:active {
  transform: translateY(1px);
}

.thinking-summary:focus-visible {
  outline: 2px solid #74adf2;
  outline-offset: -2px;
}

.thinking-summary-main,
.thinking-summary-actions {
  min-width: 0;
  display: flex;
  align-items: center;
}

.thinking-summary-main {
  flex: 1 1 auto;
  gap: 7px;
}

.thinking-status-icon {
  width: 22px;
  height: 22px;
  flex: 0 0 22px;
  color: #2f7ed8;
  font-size: 17px;
}

.status-succeeded .thinking-status-icon {
  color: #238a57;
}

.status-failed .thinking-status-icon {
  color: #c43f38;
}

.thinking-summary-main strong {
  flex: 0 0 auto;
  color: #25364d;
  font-size: 12px;
  font-weight: 750;
}

.thinking-summary-main small {
  min-width: 0;
  overflow: hidden;
  color: #718096;
  font-size: 10.5px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.thinking-summary-actions {
  flex: 0 0 auto;
  gap: 8px;
}

.thinking-summary-actions em {
  color: #64748b;
  font-size: 10.5px;
  font-style: normal;
  font-weight: 600;
}

.thinking-toggle-icon {
  color: #6f8095;
  font-size: 14px;
  transition: transform 160ms ease;
}

.thinking-toggle-icon.expanded {
  transform: rotate(180deg);
}

.thinking-detail {
  padding: 10px 12px 12px;
  border-top: 1px solid #e2eaf3;
  background: #fff;
}

.general-thinking-progress {
  margin-bottom: 5px;
  padding-bottom: 10px;
  border-bottom: 1px solid #e8eef5;
}

.general-thinking-progress > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 6px;
  color: #69798c;
  font-size: 10.5px;
}

.general-thinking-progress strong {
  color: #34465c;
  font-size: 10.5px;
  font-variant-numeric: tabular-nums;
}

.thinking-detail :deep(.el-progress-bar__outer) {
  background: #e7edf4;
}

.general-thinking-step {
  position: relative;
  display: grid;
  grid-template-columns: 22px minmax(0, 1fr);
  gap: 9px;
  padding: 7px 0 8px;
}

.general-thinking-step:not(:last-of-type)::after {
  position: absolute;
  top: 28px;
  bottom: -3px;
  left: 10px;
  width: 1px;
  background: #dce5ef;
  content: '';
}

.general-thinking-step-icon {
  position: relative;
  z-index: 1;
  width: 22px;
  height: 22px;
  display: inline-grid;
  place-items: center;
  border: 1px solid #d5dfeb;
  border-radius: 50%;
  color: #7d8da0;
  background: #f7f9fc;
  font-size: 13px;
}

.general-thinking-step.status-running .general-thinking-step-icon {
  border-color: #91bcec;
  color: #3478df;
  background: #edf6ff;
}

.general-thinking-step.status-succeeded .general-thinking-step-icon {
  border-color: #9ed3b7;
  color: #238a57;
  background: #effaf4;
}

.general-thinking-step.status-failed .general-thinking-step-icon {
  border-color: #edb4b4;
  color: #c43f38;
  background: #fff2f2;
}

.general-thinking-step > div {
  min-width: 0;
}

.general-thinking-step-title {
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.general-thinking-step-title strong {
  min-width: 0;
  overflow: hidden;
  color: #2f4054;
  font-size: 11px;
  font-weight: 750;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.general-thinking-step-title > span {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 8px;
}

.general-thinking-step-title time,
.general-thinking-step-title em {
  color: #8795a6;
  font-size: 9.5px;
  font-style: normal;
  font-variant-numeric: tabular-nums;
}

.general-thinking-step.status-running .general-thinking-step-title strong,
.general-thinking-step.status-running .general-thinking-step-title em {
  color: #2868c7;
}

.general-thinking-step.status-succeeded .general-thinking-step-title em {
  color: #238a57;
}

.general-thinking-step.status-failed .general-thinking-step-title strong,
.general-thinking-step.status-failed .general-thinking-step-title em {
  color: #b42318;
}

.general-thinking-step p {
  margin: 2px 0 0;
  color: #69798c;
  font-size: 10.5px;
  line-height: 1.55;
}

.thinking-tool,
.thinking-error {
  display: inline-block;
  max-width: 100%;
  margin-top: 4px;
  font-size: 10px;
}

.thinking-tool {
  overflow: hidden;
  padding: 1px 5px;
  border: 1px solid #d5e4f7;
  border-radius: 4px;
  color: #285fa7;
  background: #f2f7fd;
  font-family: Consolas, "Courier New", monospace;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.thinking-error {
  color: #b42318;
}

.general-thinking-empty {
  margin: 0;
  padding: 8px 0 2px;
  color: #7c8999;
  font-size: 10.5px;
}

.diagnosis-flow-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 9px;
}

.diagnosis-flow-head > div {
  min-width: 0;
}

.diagnosis-flow-head span,
.diagnosis-flow-head strong {
  display: block;
}

.diagnosis-flow-head span {
  color: #7b8796;
  font-size: 9.5px;
  font-weight: 700;
}

.diagnosis-flow-head strong {
  overflow: hidden;
  margin-top: 2px;
  color: #303b4a;
  font-size: 11px;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.diagnosis-flow-head > em {
  flex: 0 0 auto;
  color: #53657d;
  font-size: 11px;
  font-style: normal;
  font-variant-numeric: tabular-nums;
}

.diagnosis-flow-steps {
  display: flex;
  align-items: flex-start;
  margin-top: 12px;
  padding: 0 1px 2px;
  overflow-x: auto;
  overflow-y: hidden;
}

.diagnosis-flow-step {
  position: relative;
  min-width: 112px;
  flex: 1 1 0;
  padding: 0 7px;
  text-align: center;
}

.diagnosis-flow-step::before {
  position: absolute;
  top: 12px;
  right: 0;
  left: 0;
  height: 2px;
  background: #dfe7f0;
  content: '';
}

.diagnosis-flow-step:first-child::before {
  left: 50%;
}

.diagnosis-flow-step:last-child::before {
  right: 50%;
}

.diagnosis-flow-step.status-running::before,
.diagnosis-flow-step.status-succeeded::before {
  background: #9bcfb3;
}

.diagnosis-flow-step.status-failed::before {
  background: #e8a5a1;
}

.diagnosis-flow-index {
  position: relative;
  z-index: 1;
  width: 26px;
  height: 26px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  color: #7b8796;
  background: #edf1f5;
  box-shadow: 0 0 0 4px #fff;
  font-size: 10.5px;
  font-weight: 800;
}

.diagnosis-flow-step > strong,
.diagnosis-flow-step > em {
  display: block;
}

.diagnosis-flow-step > strong {
  margin-top: 7px;
  color: #3a4655;
  font-size: 10.5px;
  line-height: 1.35;
}

.diagnosis-flow-step > em {
  width: fit-content;
  margin: 5px auto 0;
  padding: 2px 6px;
  border-radius: 4px;
  color: #7b8796;
  background: #edf1f5;
  font-size: 9px;
  font-style: normal;
  font-weight: 700;
}

.diagnosis-flow-step.status-running .diagnosis-flow-index,
.diagnosis-flow-step.status-running > em {
  color: #2868c7;
  background: #e8f1ff;
}

.diagnosis-flow-step.status-succeeded .diagnosis-flow-index,
.diagnosis-flow-step.status-succeeded > em {
  color: #238a57;
  background: #e5f5ec;
}

.diagnosis-flow-step.status-failed .diagnosis-flow-index,
.diagnosis-flow-step.status-failed > em {
  color: #c43f38;
  background: #ffebe9;
}

@media (prefers-reduced-motion: reduce) {
  .thinking-status-icon,
  .general-thinking-step-icon .el-icon,
  .thinking-toggle-icon,
  .mcp-confirmation-icon .is-loading {
    animation: none !important;
    transition: none !important;
  }
}

.workload-status-panel {
  overflow: hidden;
  border: 1px solid #dce8f5;
  border-radius: 8px;
  background: #fff;
}

.workload-status-header {
  min-height: 64px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 11px 13px;
  border-bottom: 1px solid #e7eef7;
  background: #fbfdff;
}

.workload-status-heading {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 10px;
}

.workload-status-icon {
  width: 34px;
  height: 34px;
  display: inline-flex;
  flex: 0 0 34px;
  align-items: center;
  justify-content: center;
  border: 1px solid #b7e4c7;
  border-radius: 8px;
  color: #16803d;
  background: #edf9f1;
  font-size: 19px;
}

.workload-status-panel.is-warning .workload-status-icon {
  border-color: #f4d49a;
  color: #a16207;
  background: #fff8e8;
}

.workload-status-heading > div {
  min-width: 0;
}

.workload-status-heading h3 {
  overflow: hidden;
  margin: 0;
  color: #172033;
  font-size: 14px;
  font-weight: 700;
  letter-spacing: 0;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.workload-status-heading p {
  display: flex;
  align-items: center;
  gap: 5px;
  overflow: hidden;
  margin: 3px 0 0;
  color: #718096;
  font-size: 10.5px;
  line-height: 1.4;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.workload-status-heading p span {
  color: #bdc8d6;
}

.workload-health-badge {
  min-height: 24px;
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  padding: 0 9px;
  border: 1px solid #b7e4c7;
  border-radius: 6px;
  color: #147a38;
  background: #edf9f1;
  font-size: 10.5px;
  font-weight: 700;
}

.is-warning .workload-health-badge {
  border-color: #f4d49a;
  color: #9a5d06;
  background: #fff8e8;
}

.workload-metric-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  border-bottom: 1px solid #e7eef7;
}

.workload-metric {
  min-height: 66px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 3px;
  border-right: 1px solid #e7eef7;
  background: #fff;
}

.workload-metric:last-child {
  border-right: 0;
}

.workload-metric span {
  color: #718096;
  font-size: 10.5px;
}

.workload-metric strong {
  color: #172033;
  font-size: 19px;
  font-weight: 700;
  line-height: 1.15;
}

.workload-status-panel.is-healthy .workload-metric:nth-child(n + 3) strong {
  color: #16803d;
}

.workload-status-panel.is-warning .workload-metric strong {
  color: #a16207;
}

.workload-image-row {
  min-width: 0;
  display: grid;
  grid-template-columns: 68px minmax(0, 1fr);
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid #e7eef7;
  color: #718096;
  font-size: 10.5px;
}

.workload-image-row code {
  display: block;
  overflow: hidden;
  padding: 3px 6px;
  border: 1px solid #e0e9f4;
  border-radius: 4px;
  color: #334155;
  background: #f7faff;
  font-family: Consolas, "Courier New", monospace;
  font-size: 10.5px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.workload-pod-table-wrap {
  max-width: 100%;
  overflow-x: auto;
}

.workload-pod-table {
  width: 100%;
  min-width: 760px;
  border-collapse: collapse;
  color: #53657d;
  font-size: 10.5px;
}

.workload-pod-table th,
.workload-pod-table td {
  height: 35px;
  padding: 6px 10px;
  border-bottom: 1px solid #edf2f7;
  text-align: left;
  white-space: nowrap;
}

.workload-pod-table th {
  color: #64748b;
  background: #f7faff;
  font-weight: 700;
}

.workload-pod-table tbody tr:last-child td {
  border-bottom: 0;
}

.workload-pod-table td:first-child {
  width: 34%;
}

.workload-pod-table td code {
  display: block;
  max-width: 310px;
  overflow: hidden;
  color: #25364d;
  font-family: Consolas, "Courier New", monospace;
  font-size: 10.5px;
  text-overflow: ellipsis;
}

.workload-pod-table td.has-restarts {
  color: #b45309;
  font-weight: 700;
}

.pod-status {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-weight: 700;
}

.pod-status i {
  width: 6px;
  height: 6px;
  flex: 0 0 6px;
  border-radius: 50%;
  background: #94a3b8;
}

.pod-status.is-running {
  color: #16803d;
}

.pod-status.is-running i {
  background: #22a447;
}

.pod-status.is-abnormal {
  color: #b45309;
}

.pod-status.is-abnormal i {
  background: #e4a11b;
}

.workload-empty-pods {
  padding: 15px 12px;
  border-bottom: 1px solid #e7eef7;
  color: #718096;
  text-align: center;
  font-size: 11px;
}

.workload-status-footer {
  min-height: 38px;
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 8px 12px;
  border-top: 1px solid #e7eef7;
  color: #356346;
  background: #f5fbf7;
  font-size: 11px;
  line-height: 1.5;
}

.workload-status-footer .el-icon {
  flex: 0 0 auto;
  color: #16803d;
  font-size: 14px;
}

.is-warning .workload-status-footer {
  color: #82520c;
  background: #fffaf0;
}

.is-warning .workload-status-footer .el-icon {
  color: #b7791f;
}

@media (max-width: 960px) {
  .message-viewport {
    padding: 12px;
  }

  .message-row.is-assistant .message-bubble {
    width: 100%;
    margin-left: 0;
  }

  .message-origin {
    margin-left: 0;
  }

  .workload-metric-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .workload-metric:nth-child(2) {
    border-right: 0;
  }

  .workload-metric:nth-child(-n + 2) {
    border-bottom: 1px solid #e7eef7;
  }
}

.mcp-selection-panel,
.mcp-confirmation-panel {
  margin-top: 12px;
  padding: 11px 12px;
  border: 1px solid #e7c780;
  border-radius: 8px;
  background: #fffaf0;
}

.mcp-selection-panel > header strong,
.mcp-selection-panel > header span {
  display: block;
}

.mcp-selection-panel > header strong {
  color: #6f4c0b;
  font-size: 12px;
}

.mcp-selection-panel > header span {
  margin-top: 2px;
  color: #806d4d;
  font-size: 11px;
}

.mcp-selection-table-wrap {
  margin-top: 9px;
  overflow-x: auto;
  border: 1px solid #ead8b4;
  border-radius: 6px;
  background: #fff;
}

.mcp-selection-table {
  width: 100%;
  min-width: 560px;
  border-collapse: collapse;
  color: #445269;
  font-size: 11px;
}

.mcp-selection-table th,
.mcp-selection-table td {
  padding: 7px 9px;
  border-bottom: 1px solid #edf0f3;
  text-align: left;
  white-space: nowrap;
}

.mcp-selection-table th {
  color: #6d7787;
  background: #f7f8fa;
  font-weight: 600;
}

.mcp-selection-table tbody tr {
  cursor: pointer;
}

.mcp-selection-table tbody tr:last-child td {
  border-bottom: 0;
}

.mcp-selection-table tbody tr:hover,
.mcp-selection-table tbody tr.is-selected {
  background: #eef5ff;
}

.mcp-selection-table input[type="radio"] {
  accent-color: #2563eb;
}

.mcp-selection-table code {
  color: #31445f;
  font-family: Consolas, "Courier New", monospace;
}

.mcp-selection-empty {
  margin: 9px 0 0;
  color: #7b8798;
  font-size: 11px;
}

.mcp-selection-panel > footer,
.mcp-confirmation-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 7px;
  margin-top: 10px;
}

.mcp-confirmation-header {
  display: flex;
  align-items: center;
  gap: 9px;
}

.mcp-confirmation-header > div {
  min-width: 0;
}

.mcp-confirmation-header strong,
.mcp-confirmation-header span {
  display: block;
}

.mcp-confirmation-header strong {
  color: #754b08;
  font-size: 12px;
  line-height: 18px;
}

.mcp-confirmation-header div > span {
  overflow: hidden;
  color: #665b49;
  font-size: 11px;
  line-height: 17px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mcp-confirmation-icon {
  display: inline-grid;
  width: 28px;
  height: 28px;
  flex: 0 0 28px;
  place-items: center;
  border: 1px solid #e6c06e;
  border-radius: 6px;
  color: #9a6507;
  background: #fff2d2;
}

.mcp-confirmation-icon :deep(.el-icon) {
  font-size: 15px;
}

.mcp-confirmation-icon .is-loading {
  animation: rotating 1.2s linear infinite;
}

.mcp-confirmation-summary {
  margin: 10px 0 0;
  border-top: 1px solid rgba(205, 174, 110, 0.44);
}

.mcp-confirmation-summary > div,
.mcp-confirmation-details dl > div {
  display: grid;
  grid-template-columns: 76px minmax(0, 1fr);
  gap: 10px;
  align-items: start;
  padding: 6px 0;
  border-bottom: 1px solid rgba(205, 174, 110, 0.34);
}

.mcp-confirmation-summary dt,
.mcp-confirmation-details dt {
  color: #826d49;
  font-size: 10.5px;
  font-weight: 600;
}

.mcp-confirmation-summary dd,
.mcp-confirmation-details dd {
  min-width: 0;
  margin: 0;
  color: #3d4c61;
  font-size: 11px;
  line-height: 1.55;
  overflow-wrap: anywhere;
}

.mcp-confirmation-summary code,
.mcp-confirmation-details code {
  color: #2f435d;
  font-family: Consolas, "Courier New", monospace;
}

.mcp-confirmation-details {
  margin-top: 8px;
}

.mcp-confirmation-details summary {
  width: fit-content;
  color: #406c9d;
  font-size: 11px;
  cursor: pointer;
  user-select: none;
}

.mcp-confirmation-details dl {
  margin: 7px 0 0;
  padding: 0 9px;
  border: 1px solid #dce5ef;
  border-radius: 6px;
  background: #f7f9fb;
}

.mcp-confirmation-details dl > div:last-child {
  border-bottom: 0;
}

.mcp-confirmation-error {
  margin: 9px 0 0;
  padding: 7px 9px;
  border-left: 3px solid #d4584c;
  background: #fff1ef;
  color: #a33e34;
  font-size: 11px;
  line-height: 1.55;
}

.mcp-confirmation-actions > span {
  margin-right: auto;
  color: #a33e34;
  font-size: 11px;
}

.assistant-page :deep(.mcp-selection-panel .el-button),
.assistant-page :deep(.mcp-confirmation-actions .el-button) {
  min-width: 76px;
  height: 28px;
  border-radius: 6px;
  font-size: 11px;
  white-space: nowrap;
}

.assistant-page :deep(.mcp-selection-panel .el-button:active),
.assistant-page :deep(.mcp-confirmation-actions .el-button:active) {
  transform: translateY(1px);
}

.mcp-confirmation-panel.status-completed {
  border-color: #a9d8c1;
  background: #f2faf6;
}

.status-completed .mcp-confirmation-icon {
  border-color: #9fd1b8;
  color: #177653;
  background: #e3f5eb;
}

.status-completed .mcp-confirmation-header strong {
  color: #176c4e;
}

.mcp-confirmation-panel.status-cancelled,
.mcp-confirmation-panel.status-expired {
  border-color: #d5dde7;
  background: #f7f9fb;
}

.status-cancelled .mcp-confirmation-icon,
.status-expired .mcp-confirmation-icon {
  border-color: #cbd5e1;
  color: #64748b;
  background: #eef2f6;
}

.status-cancelled .mcp-confirmation-header strong,
.status-expired .mcp-confirmation-header strong {
  color: #536175;
}

.mcp-confirmation-panel.status-failed,
.mcp-confirmation-panel.status-invalid {
  border-color: #e3b2ac;
  background: #fff7f6;
}

.status-failed .mcp-confirmation-icon,
.status-invalid .mcp-confirmation-icon {
  border-color: #dfa9a2;
  color: #b54338;
  background: #ffebe8;
}

.status-failed .mcp-confirmation-header strong,
.status-invalid .mcp-confirmation-header strong {
  color: #9f392f;
}

@media (max-width: 560px) {
  .thinking-summary {
    align-items: flex-start;
    padding: 8px 9px;
  }

  .thinking-summary-main {
    flex-wrap: wrap;
  }

  .thinking-summary-main small {
    width: calc(100% - 29px);
    margin-left: 29px;
    white-space: normal;
  }

  .thinking-summary-actions em {
    display: none;
  }

  .thinking-detail {
    padding: 9px;
  }

  .general-thinking-step-title {
    align-items: flex-start;
    flex-direction: column;
    gap: 3px;
  }

  .diagnosis-flow-head {
    align-items: flex-start;
  }

  .workload-status-header {
    align-items: flex-start;
    padding: 10px;
  }

  .workload-status-heading h3 {
    white-space: normal;
  }

  .workload-status-heading p {
    flex-wrap: wrap;
    white-space: normal;
  }

  .workload-health-badge {
    min-height: 22px;
    padding: 0 6px;
  }

  .workload-image-row {
    grid-template-columns: 1fr;
    gap: 4px;
  }

  .mcp-selection-panel,
  .mcp-confirmation-panel {
    padding: 10px;
  }

  .mcp-confirmation-header {
    align-items: flex-start;
  }

  .mcp-confirmation-header div > span {
    white-space: normal;
  }

  .mcp-confirmation-summary > div,
  .mcp-confirmation-details dl > div {
    grid-template-columns: 64px minmax(0, 1fr);
    gap: 7px;
  }

  .mcp-confirmation-actions {
    align-items: stretch;
    flex-wrap: wrap;
  }

  .mcp-confirmation-actions > span {
    width: 100%;
    margin-right: 0;
  }
}
</style>
