<template>
  <div class="db-review-page">
    <section class="review-shell">
      <header class="review-hero">
        <div class="review-hero-copy">
          <span class="ops-page-header__icon" aria-hidden="true">
            <img :src="databaseSvg" alt="">
          </span>
          <div class="ops-page-header__content">
            <h1>数据库 SQL 审核</h1>
            <span class="ops-page-header__divider" aria-hidden="true"></span>
            <p>承接 DML、DDL 与查询工单，集中完成检测、审批、执行和回滚。</p>
          </div>
        </div>
      </header>

      <section class="filter-panel">
        <el-form :inline="true" :model="query" class="filter-form" @submit.prevent>
          <el-form-item label="关键词">
            <el-input v-model="query.keyword" clearable placeholder="工单号 / 标题 / SQL" @keyup.enter="handleSearch" />
          </el-form-item>
          <el-form-item label="类型">
            <el-select v-model="query.ticketType" clearable placeholder="全部">
              <el-option label="全部工单" value="" />
              <el-option label="DML 工单" value="dml" />
              <el-option label="DDL 工单" value="ddl" />
              <el-option label="查询工单" value="query" />
            </el-select>
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="query.status" clearable placeholder="全部状态">
              <el-option v-for="item in statusOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="资产">
            <el-select v-model="query.assetId" clearable filterable placeholder="全部资产">
              <el-option
                v-for="asset in assets"
                :key="getAssetId(asset)"
                :label="getAssetTitle(asset)"
                :value="getAssetId(asset)"
              >
                <span class="asset-option">
                  <img :src="getAssetIcon(asset)" class="asset-option-icon" alt="" />
                  <span>{{ getAssetTitle(asset) }}</span>
                </span>
              </el-option>
            </el-select>
          </el-form-item>
          <el-form-item label="数据库">
            <el-input v-model="query.database" clearable placeholder="database" @keyup.enter="handleSearch" />
          </el-form-item>
          <el-form-item>
            <el-button class="primary-action compact" type="primary" :icon="Search" @click="handleSearch">查询</el-button>
            <el-button class="warning-action compact" type="warning" :icon="RefreshRight" @click="resetSearch">重置</el-button>
            <el-button class="success-action compact" type="success" :icon="Plus" @click="openCreateDialog">创建工单</el-button>
            <el-button class="warning-action compact" type="warning" :icon="Tools" @click="openToolboxDialog">工具箱</el-button>
          </el-form-item>
        </el-form>
      </section>

      <section class="table-panel">
        <div class="status-tabs">
          <button v-for="item in typeTabs" :key="item.value" type="button" :class="{ active: query.ticketType === item.value }" @click="switchType(item.value)">
            {{ item.label }}
          </button>
        </div>
        <div class="table-shell">
          <el-table v-loading="loading" :data="tickets" class="review-table" height="100%">
            <el-table-column prop="ticketNo" label="工单号" min-width="140" show-overflow-tooltip>
              <template #default="{ row }">
                <button class="ticket-no-link" type="button" @click="openDetail(row)">
                  {{ row.ticketNo || '-' }}
                </button>
              </template>
            </el-table-column>
            <el-table-column label="数据库类型" width="128" show-overflow-tooltip>
              <template #default="{ row }">
                <span :class="['db-type-cell', `db-type-${getTicketDbEngine(row)}`]">
                  <span class="db-type-icon-box">
                    <img :src="getTicketDbIcon(row)" class="db-type-icon" alt="" />
                  </span>
                  <strong class="db-type-name">{{ getTicketDbType(row) }}</strong>
                </span>
              </template>
            </el-table-column>
            <el-table-column prop="title" label="标题" min-width="190" show-overflow-tooltip />
            <el-table-column prop="ticketType" label="类型" width="78">
              <template #default="{ row }">
                <el-tag size="small" effect="light" round>{{ ticketTypeText(row.ticketType || row.type) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="status" label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="statusTag(row.status)" size="small" effect="light" round>{{ statusText(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="riskLevel" label="风险" width="86">
              <template #default="{ row }">
                <span :class="['risk-dot', `risk-${row.riskLevel || 'low'}`]"></span>{{ riskText(row.riskLevel) }}
              </template>
            </el-table-column>
            <el-table-column prop="assetName" label="目标实例" min-width="140" show-overflow-tooltip />
            <el-table-column label="数据库" width="120" show-overflow-tooltip>
              <template #default="{ row }">{{ getTicketDatabase(row) }}</template>
            </el-table-column>
            <el-table-column label="提交时间" min-width="150" show-overflow-tooltip>
              <template #default="{ row }">{{ getSubmittedTime(row) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="240" fixed="right" align="center">
              <template #default="{ row }">
                <div class="operation-links">
                  <el-button link type="primary" @click="openDetail(row)">详情</el-button>
                  <el-button v-if="canApproveTicket(row)" link type="success" @click="ticketAction(row, 'approve')">通过</el-button>
                  <el-button v-if="canApproveTicket(row)" link type="danger" @click="ticketAction(row, 'reject')">驳回</el-button>
                  <el-button v-if="canExecuteTicket(row)" link type="success" @click="ticketAction(row, 'execute')">执行</el-button>
                  <el-button v-if="canCancelTicket(row)" link type="warning" @click="ticketAction(row, 'cancel')">撤销</el-button>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </div>
        <div class="pagination-row">
          <span>共 {{ total }} 条</span>
          <el-pagination
            v-model:current-page="query.page"
            v-model:page-size="query.pageSize"
            :page-sizes="[10, 50, 100]"
            layout="sizes, prev, pager, next"
            :total="total"
            @size-change="loadTickets"
            @current-change="loadTickets"
          />
        </div>
      </section>
    </section>

    <el-dialog
      v-model="createDialog.visible"
      title="提交 SQL 审核工单"
      width="1080px"
      class="review-dialog"
      modal-class="database-review-dialog-overlay"
      destroy-on-close
    >
      <div class="create-layout">
        <el-form label-position="top" :model="ticketForm" class="ticket-form">
          <div class="form-grid">
            <el-form-item class="form-span-1" label="工单类型">
              <el-radio-group v-model="ticketForm.ticketType">
                <el-radio-button label="dml">DML</el-radio-button>
                <el-radio-button label="ddl">DDL</el-radio-button>
                <el-radio-button label="query">查询</el-radio-button>
              </el-radio-group>
            </el-form-item>
            <el-form-item class="form-span-2" label="工单标题">
              <el-input v-model="ticketForm.title" placeholder="例如：修改主机状态" />
            </el-form-item>
            <div class="sql-type-hint">
              <span :class="{ active: ticketForm.ticketType === 'dml' }"><strong>DML</strong> {{ isSelectedPostgreSQL ? 'INSERT / UPDATE / DELETE / ON CONFLICT' : 'INSERT / UPDATE / DELETE / REPLACE' }}</span>
              <span :class="{ active: ticketForm.ticketType === 'ddl' }"><strong>DDL</strong> CREATE / ALTER / DROP / TRUNCATE</span>
              <span :class="{ active: ticketForm.ticketType === 'query' }"><strong>查询</strong> SELECT / WITH</span>
            </div>
            <el-form-item label="数据库资产">
              <el-select v-model="ticketForm.assetId" filterable placeholder="选择资产" @change="handleTicketAssetChange">
                <el-option
                  v-for="asset in assets"
                  :key="getAssetId(asset)"
                  :label="getAssetTitle(asset)"
                  :value="getAssetId(asset)"
                >
                  <span class="asset-option">
                    <img :src="getAssetIcon(asset)" class="asset-option-icon" alt="" />
                    <span>{{ getAssetTitle(asset) }}</span>
                  </span>
                </el-option>
              </el-select>
            </el-form-item>
            <el-form-item label="数据库">
              <el-select
                ref="databaseSelectRef"
                v-model="ticketForm.database"
                filterable
                clearable
                :loading="databaseLoading"
                :disabled="!ticketForm.assetId"
                placeholder="选择数据库"
                @visible-change="visible => visible && loadTicketDatabases(ticketForm.assetId)"
              >
                <el-option v-for="item in databaseOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
            <el-form-item v-if="showSchemaField" label="Schema">
              <el-input v-model="ticketForm.schema" :placeholder="schemaPlaceholder" />
            </el-form-item>
            <div v-else class="form-spacer"></div>
            <el-form-item label="审批方式">
              <el-radio-group v-model="ticketForm.approvalMode">
                <el-radio-button value="workflow">审批流</el-radio-button>
                <el-radio-button value="legacy">指定审批人</el-radio-button>
              </el-radio-group>
            </el-form-item>
            <el-form-item v-if="ticketForm.approvalMode === 'workflow'" label="审批流程">
              <el-select v-model="ticketForm.workflowId" filterable placeholder="选择 SQL 审批流">
                <el-option v-for="flow in applicableSqlWorkflows" :key="flow.id" :label="flow.name" :value="flow.id" />
              </el-select>
            </el-form-item>
            <el-form-item v-else label="审批人">
              <el-select v-model="ticketForm.approverId" filterable placeholder="选择审批人">
                <el-option v-for="user in users" :key="user.id" :label="userLabel(user)" :value="user.id" />
              </el-select>
            </el-form-item>
            <el-form-item label="执行人">
              <el-select v-model="ticketForm.executorId" filterable placeholder="选择执行人">
                <el-option v-for="user in users" :key="user.id" :label="userLabel(user)" :value="user.id" />
              </el-select>
            </el-form-item>
            <el-form-item v-if="ticketForm.ticketType === 'dml'" label="闪回保障">
              <div class="backup-switch-field">
                <el-switch v-model="ticketForm.backupRequired" />
                <span v-if="ticketForm.backupRequired">{{ dmlProtectionText }}</span>
              </div>
            </el-form-item>
            <el-form-item v-else-if="ticketForm.ticketType === 'ddl' && isSelectedMySQL" label=" ">
              <div class="ddl-switch-row">
                <label>
                  <span>gh-ost无锁变更</span>
                  <el-switch v-model="ticketForm.useGhOst" />
                </label>
              </div>
            </el-form-item>
            <el-form-item v-else-if="ticketForm.ticketType === 'ddl' && isSelectedPostgreSQL" label=" ">
              <el-alert :title="ddlCapabilityText" type="warning" show-icon :closable="false" />
            </el-form-item>
            <el-form-item v-if="ticketForm.ticketType === 'query'" label="有效期">
              <el-select v-model="ticketForm.validDuration" placeholder="选择有效期">
                <el-option label="1 小时" value="1h" />
                <el-option label="4 小时" value="4h" />
                <el-option label="1 天" value="24h" />
              </el-select>
            </el-form-item>
            <el-form-item v-if="ticketForm.ticketType === 'query'" label="最大返回行数">
              <el-input-number v-model="ticketForm.maxRows" :min="1" :max="10000" :step="100" controls-position="right" />
            </el-form-item>
            <el-form-item v-if="ticketForm.ticketType === 'query'" label="允许导出">
              <el-switch v-model="ticketForm.allowExport" />
            </el-form-item>
          </div>
          <el-form-item label="SQL 内容">
            <CodeEditor
              v-model="ticketForm.sql"
              class="ticket-sql-editor"
              language="sql"
              theme="aiops-sql-light"
              :height="`${sqlEditorHeight}px`"
              word-wrap="off"
              :completion-items="sqlCompletionItems"
            />
            <div class="sql-resize-handle" @mousedown="startSqlResize" @dblclick="resetSqlEditorHeight">
              <span></span>
            </div>
            <div v-if="isSelectedPostgreSQL && ticketForm.ticketType === 'dml'" class="sql-engine-tip">
              PostgreSQL UPDATE / DELETE 建议使用主键或唯一键 WHERE 精确限定，批量处理可使用 CTE / 主键分批。
            </div>
          </el-form-item>
          <el-form-item v-if="ticketForm.ticketType !== 'query'" class="rollback-form-item">
            <template #label>
              <span class="compact-label">{{ rollbackLabel }}</span>
            </template>
            <CodeEditor
              v-model="ticketForm.rollbackSql"
              class="rollback-sql-editor"
              language="sql"
              theme="aiops-sql-light"
              height="88px"
              word-wrap="off"
              :completion-items="sqlCompletionItems"
            />
          </el-form-item>
          <el-form-item label="备注">
            <el-input v-model="ticketForm.remark" type="textarea" :rows="1" resize="none" :placeholder="remarkPlaceholder" />
          </el-form-item>
        </el-form>

        <section class="check-panel">
          <div class="check-head">
            <strong>自动检测</strong>
            <el-button class="primary-action compact" :loading="checking" @click="runReviewCheck">检测</el-button>
          </div>
          <div v-if="checkResult" class="check-summary">
            <div>
              <el-tag :type="canSubmitTicket ? 'success' : 'danger'" effect="light" round>
                {{ canSubmitTicket ? '检测通过' : '检测未通过' }}
              </el-tag>
              <span>风险：{{ riskText(checkResult.riskLevel) }}</span>
            </div>
            <strong>{{ checkSubmitText }}</strong>
          </div>
          <div v-if="checkResult" class="check-metrics">
            <article v-for="item in checkMetricCards" :key="item.label">
              <span>{{ item.label }}</span>
              <strong>{{ item.value }}</strong>
            </article>
          </div>
          <div v-if="checkResult" class="check-rule-overview">
            <span class="level-error">错误 {{ groupedCheckResults.error.length }}</span>
            <span class="level-warn">警告 {{ groupedCheckResults.warn.length }}</span>
            <span class="level-info">提示 {{ groupedCheckResults.info.length }}</span>
          </div>
          <div class="check-list">
            <article v-for="item in checkResults" :key="item.ruleCode || item.message" :class="['check-item', `level-${item.level || 'info'}`]">
              <strong>{{ item.ruleName || item.ruleCode || item.level || '规则' }}</strong>
              <span>{{ item.message || item.description || '-' }}</span>
            </article>
            <el-empty v-if="!checkResults.length" description="暂无检测结果" />
          </div>
        </section>
      </div>
      <template #footer>
        <span class="dialog-footer">
          <el-button class="cancel-button" @click="createDialog.visible = false">取消</el-button>
          <el-button class="confirm-button" type="primary" :loading="submitting" :disabled="!canSubmitTicket" @click="submitTicket">提交工单</el-button>
        </span>
      </template>
    </el-dialog>

    <el-drawer v-model="detailDrawer.visible" size="900px" title="SQL 审核工单详情" class="review-drawer" destroy-on-close>
      <div v-loading="detailDrawer.loading" class="detail-body">
        <template v-if="detailTicket">
          <el-tabs v-model="detailTab" class="detail-tabs" @tab-change="handleDetailTabChange">
            <el-tab-pane label="基本信息" name="base">
              <section class="detail-overview-card">
                <div class="detail-overview-head">
                  <div>
                    <p>{{ detailTicket.ticketNo || '-' }}</p>
                    <h2>{{ detailTicket.title || '-' }}</h2>
                    <span>{{ getTicketDatabase(detailTicket) }} / {{ detailTicket.assetName || detailTicket.assetId || '-' }}</span>
                    <em class="current-action-tip">{{ currentActionTip }}</em>
                  </div>
                  <div class="detail-actions">
                    <el-tag :type="statusTag(detailTicket.status)" effect="light" round>{{ statusText(detailTicket.status) }}</el-tag>
                    <el-button v-if="canApproveDetail" class="success-action compact" @click="ticketAction(detailTicket, 'approve')">审批通过</el-button>
                    <el-button v-if="canApproveDetail" class="danger-action compact" @click="ticketAction(detailTicket, 'reject')">驳回</el-button>
                    <el-button v-if="canExecuteDetail" class="success-action compact" @click="ticketAction(detailTicket, 'execute')">执行</el-button>
                    <el-button v-if="canCancelDetail" class="warning-action compact" @click="ticketAction(detailTicket, 'cancel')">撤销</el-button>
                  </div>
                </div>
                <div class="workflow-rail">
                  <div class="workflow-track">
                    <div :class="['workflow-fill', `progress-${workflowProgress}`]"></div>
                  </div>
                  <article v-for="(step, index) in workflowSteps" :key="step.key" :class="['workflow-step', step.state]">
                    <i>{{ step.done ? '✓' : index + 1 }}</i>
                    <div>
                      <span>{{ step.label }}</span>
                      <strong>{{ step.name || '-' }}</strong>
                      <em>{{ step.time || '-' }}</em>
                    </div>
                  </article>
                </div>
                <div class="info-table">
                  <div v-for="item in detailItems" :key="item.label" class="info-cell">
                    <span>{{ item.label }}</span>
                    <strong>{{ item.value || '-' }}</strong>
                  </div>
                </div>
                <div class="overview-sql-block">
                  <div class="overview-sql-head">
                    <strong>SQL 内容</strong>
                    <div class="tag-row">
                      <el-tag size="small" effect="light" round>{{ ticketTypeText(detailTicket.ticketType || detailTicket.type) }}</el-tag>
                      <el-tag v-if="detailTicket.useGhOst" size="small" type="warning" effect="light" round>gh-ost 在线 DDL</el-tag>
                    </div>
                  </div>
                  <pre>{{ getTicketSql(detailTicket) }}</pre>
                </div>
              </section>
            </el-tab-pane>
            <el-tab-pane label="SQL 与审核结果" name="sql">
              <section class="review-result-panel">
                <div class="review-summary-grid">
                  <article v-for="item in detailReviewSummary" :key="item.label">
                    <span>{{ item.label }}</span>
                    <strong>{{ item.value }}</strong>
                  </article>
                </div>
                <div class="detail-sql">
                  <div class="detail-sql-head">
                    <strong>最终执行 SQL</strong>
                    <div class="tag-row">
                      <el-tag size="small" effect="light" round>{{ ticketTypeText(detailTicket.ticketType || detailTicket.type) }}</el-tag>
                      <el-tag v-if="detailTicket.useGhOst" size="small" type="warning" effect="light" round>gh-ost 在线 DDL</el-tag>
                    </div>
                  </div>
                  <pre>{{ getTicketSql(detailTicket) }}</pre>
                </div>
                <div v-if="detailTicket.useGhOst" class="detail-sql ghost-script-block">
                  <div class="detail-sql-head">
                    <strong>gh-ost 执行脚本</strong>
                    <el-button class="primary-action compact" :disabled="!detailTicket.ghOstScript" @click="copyGhOstScript">复制脚本</el-button>
                  </div>
                  <el-alert type="warning" show-icon :closable="false">
                    <template #title>
                      执行节点需要提前配置 gh-ost 二进制工具：/opt/db/gh-ost。未配置时请手动下载
                      <a
                        href="https://github.com/github/gh-ost/releases/download/v1.1.9/gh-ost-binary-linux-amd64-20260501022029.tar.gz"
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        github/gh-ost
                      </a>
                      后放到该路径。
                    </template>
                  </el-alert>
                  <pre>{{ detailTicket.ghOstScript || '暂无 gh-ost 脚本' }}</pre>
                </div>
                <div class="check-list detail-check-list">
                  <article v-for="item in detailReviewResults" :key="item.ruleCode || item.message" :class="['check-item', `level-${item.level || 'info'}`]">
                    <strong>{{ item.ruleName || item.ruleCode || item.level || '规则' }}</strong>
                    <span>{{ item.message || item.description || '-' }}</span>
                  </article>
                  <el-empty v-if="!detailReviewResults.length" description="暂无审核命中" />
                </div>
              </section>
            </el-tab-pane>
            <el-tab-pane label="审批记录" name="approval">
              <section v-if="approvalWorkflowDetail" class="generic-workflow-panel">
                <div class="generic-workflow-head">
                  <div><strong>{{ approvalWorkflowDetail.instance?.definitionName }}</strong><span>当前节点：{{ approvalWorkflowDetail.instance?.currentNodeName || '流程已结束' }}</span></div>
                  <el-tag :type="approvalWorkflowDetail.instance?.status === 'approved' ? 'success' : approvalWorkflowDetail.instance?.status === 'rejected' ? 'danger' : 'warning'">{{ genericWorkflowStatusText(approvalWorkflowDetail.instance?.status) }}</el-tag>
                </div>
                <el-timeline>
                  <el-timeline-item v-for="task in approvalWorkflowDetail.tasks || []" :key="task.id" :timestamp="displayTime(task.actedAt || task.createdAt)" :type="task.status === 'approved' ? 'success' : task.status === 'rejected' ? 'danger' : 'primary'">
                    <strong>{{ task.nodeOrder }}. {{ task.nodeName }}</strong>
                    <span class="generic-workflow-assignee">{{ task.assigneeName }} / {{ genericTaskStatusText(task.status) }}</span>
                    <p v-if="task.comment">{{ task.comment }}</p>
                  </el-timeline-item>
                </el-timeline>
              </section>
              <el-table :data="detailData.approvalRecords" class="review-table compact-table">
                <el-table-column label="节点" width="96">
                  <template #default="{ row }">{{ approvalActionText(row.action) }}</template>
                </el-table-column>
                <el-table-column prop="operatorName" label="操作人" min-width="120" />
                <el-table-column label="结果" width="110">
                  <template #default="{ row }">
                    <el-tag :type="approvalActionTag(row.action)" size="small" effect="light" round>{{ approvalActionText(row.action) }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="意见" min-width="220" show-overflow-tooltip>
                  <template #default="{ row }">{{ row.comment || row.remark || '无补充意见' }}</template>
                </el-table-column>
                <el-table-column label="时间" min-width="150">
                  <template #default="{ row }">{{ displayTime(row.createdAt || row.created_at) }}</template>
                </el-table-column>
              </el-table>
            </el-tab-pane>
            <el-tab-pane label="执行日志" name="logs">
              <el-table v-loading="logsLoading" :data="executionLogs" class="review-table compact-table">
                <el-table-column label="SQL 语句" min-width="260" show-overflow-tooltip>
                  <template #default="{ row }">{{ getExecutionSql(row) }}</template>
                </el-table-column>
                <el-table-column label="状态" width="96">
                  <template #default="{ row }">
                    <el-tag :type="executionStatusTag(row.status)" size="small" effect="light" round>{{ executionStatusText(row.status) }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="影响行" width="90">
                  <template #default="{ row }">{{ row.rowsAffected ?? row.affectedRows ?? '-' }}</template>
                </el-table-column>
                <el-table-column label="耗时(ms)" width="100">
                  <template #default="{ row }">{{ row.durationMs ?? row.costMs ?? '-' }}</template>
                </el-table-column>
                <el-table-column label="错误" min-width="220" show-overflow-tooltip>
                  <template #default="{ row }">{{ row.errorMessage || '-' }}</template>
                </el-table-column>
                <el-table-column prop="executorName" label="执行人" width="100" />
                <el-table-column label="开始时间" min-width="150">
                  <template #default="{ row }">{{ displayTime(row.startedAt || row.createdAt) }}</template>
                </el-table-column>
                <el-table-column label="结束时间" min-width="150">
                  <template #default="{ row }">{{ displayTime(row.finishedAt || row.endedAt) }}</template>
                </el-table-column>
                <el-table-column label="执行日志" width="120" fixed="right">
                  <template #default="{ row }">
                    <el-button link type="primary" :disabled="!getExecutionLog(row)" @click="openExecLogDialog(row)">查看日志</el-button>
                  </template>
                </el-table-column>
              </el-table>
            </el-tab-pane>
            <el-tab-pane v-if="isQueryTicket(detailTicket)" label="查询结果" name="result">
              <section v-loading="queryResult.loading" class="query-result-panel">
                <div class="query-result-head">
                  <div class="query-result-title">
                    <strong>查询结果</strong>
                    <span>{{ queryResult.total || queryResult.rows.length }} 行</span>
                    <span>{{ queryResult.columns.length }} 列</span>
                    <span v-if="queryResult.durationMs">{{ queryResult.durationMs }}ms</span>
                  </div>
                  <el-alert
                    class="query-result-tip"
                    title="页面仅展示前 100 行，下载 CSV 最多返回 1000 行"
                    type="warning"
                    show-icon
                    :closable="false"
                  />
                  <el-button class="warning-action compact" :disabled="!queryResult.rows.length" @click="downloadQueryResult">下载 CSV</el-button>
                </div>
                <div class="query-result-table">
                  <el-table :data="queryResult.rows" class="review-table compact-table" height="100%" empty-text="暂无查询结果">
                    <el-table-column type="index" label="#" width="56" fixed />
                    <el-table-column
                      v-for="column in queryResult.columns"
                      :key="getResultColumnName(column)"
                      :prop="getResultColumnName(column)"
                      :label="getResultColumnName(column)"
                      min-width="140"
                      show-overflow-tooltip
                    >
                      <template #default="{ row }">{{ formatResultCell(row[getResultColumnName(column)]) }}</template>
                    </el-table-column>
                  </el-table>
                </div>
                <div v-if="queryResult.sql" class="query-result-sql">
                  <strong>执行 SQL</strong>
                  <pre>{{ queryResult.sql }}</pre>
                </div>
              </section>
            </el-tab-pane>
            <el-tab-pane v-if="!isQueryTicket(detailTicket)" label="回滚 SQL" name="rollback">
              <div v-loading="rollbackLoading" class="rollback-panel">
                <el-alert v-if="rollbackInfo && rollbackInfo.available === false" :title="rollbackInfo.message || '当前工单不可自动回滚，需要人工处理。'" type="warning" show-icon :closable="false" />
                <div class="rollback-meta" v-if="rollbackInfo">
                  <span>类型：{{ rollbackInfo.rollbackType || '-' }}</span>
                  <span>原表：{{ rollbackInfo.originalDatabase || '-' }}.{{ rollbackInfo.originalTable || '-' }}</span>
                  <span v-if="rollbackInfo.backupTable">备份表：{{ rollbackInfo.backupDatabase || '-' }}.{{ rollbackInfo.backupTable || '-' }}</span>
                  <span v-if="rollbackInfo.backupRows !== undefined">备份行数：{{ rollbackInfo.backupRows ?? '-' }}</span>
                  <span>起始位点：{{ formatBinlogPoint(rollbackInfo, 'start') }}</span>
                  <span>结束位点：{{ formatBinlogPoint(rollbackInfo, 'end') }}</span>
                  <span>时间：{{ rollbackInfo.createdAt || '-' }}</span>
                </div>
                <el-alert
                  v-if="isDetailMySQL && !rollbackInfo?.rollbackSql && hasBinlogPoint"
                  title="已记录 binlog 位点，可调用本机 /opt/db/my2sql 生成闪回 SQL。"
                  type="info"
                  show-icon
                  :closable="false"
                />
                <el-alert
                  v-if="isDetailPostgreSQL && !rollbackInfo?.rollbackSql"
                  title="PostgreSQL 自动回滚依赖 WalMiner/WAL 解析能力，当前以前置检测和人工确认回滚 SQL 为主。"
                  type="warning"
                  show-icon
                  :closable="false"
                />
                <el-select
                  v-if="rollbackRecords.length > 1"
                  v-model="activeRollbackRecordId"
                  class="rollback-record-select"
                  placeholder="选择回滚记录"
                >
                  <el-option
                    v-for="record in rollbackRecords"
                    :key="record.id"
                    :label="`${record.originalTable || '记录'} / ${record.rollbackType || '-'} / ${record.createdAt || '-'}`"
                    :value="record.id"
                  />
                </el-select>
                <div class="detail-sql">
                  <pre>{{ rollbackInfo?.rollbackSql || '暂无回滚 SQL' }}</pre>
                </div>
                <div class="rollback-actions">
                  <el-button class="primary-action compact" :disabled="!rollbackInfo?.rollbackSql" @click="copyRollbackSql">复制回滚 SQL</el-button>
                  <el-button class="warning-action compact" :disabled="!rollbackInfo?.rollbackSql" @click="openRollbackTicketDialog">转回滚工单</el-button>
                  <el-button v-if="isDetailMySQL" class="success-action compact" :loading="flashbackLoading" :disabled="!canGenerateFlashback" @click="generateFlashbackSql">生成闪回 SQL</el-button>
                  <el-button class="danger-action compact" :loading="backupCleanupLoading" :disabled="!hasBackupTables" @click="cleanupBackupTables">删除备份表</el-button>
                </div>
              </div>
            </el-tab-pane>
          </el-tabs>
        </template>
      </div>
    </el-drawer>

    <el-dialog
      v-model="rollbackTicketDialog.visible"
      title="转回滚工单"
      width="520px"
      class="review-dialog"
      modal-class="database-review-dialog-overlay"
      destroy-on-close
    >
      <el-form label-position="top" :model="rollbackTicketForm" class="ticket-form">
        <el-form-item label="工单标题">
          <el-input v-model="rollbackTicketForm.title" placeholder="例如：故障回滚-users数据恢复" />
        </el-form-item>
        <el-form-item label="审批人">
          <el-select v-model="rollbackTicketForm.approverId" filterable placeholder="选择审批人">
            <el-option v-for="user in users" :key="user.id" :label="userLabel(user)" :value="user.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="执行人">
          <el-select v-model="rollbackTicketForm.executorId" filterable placeholder="选择执行人">
            <el-option v-for="user in users" :key="user.id" :label="userLabel(user)" :value="user.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="rollbackTicketForm.remark" type="textarea" :rows="2" resize="none" placeholder="回滚原因 / 影响范围" />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button class="cancel-button" @click="rollbackTicketDialog.visible = false">取消</el-button>
          <el-button class="confirm-button" type="primary" :loading="rollbackTicketDialog.submitting" @click="submitRollbackTicket">提交工单</el-button>
        </span>
      </template>
    </el-dialog>

    <el-dialog
      v-model="execLogDialog.visible"
      title="执行日志"
      width="760px"
      class="review-dialog"
      modal-class="database-review-dialog-overlay"
      destroy-on-close
    >
      <div class="detail-sql exec-log-dialog">
        <pre>{{ execLogDialog.content || '暂无日志' }}</pre>
      </div>
    </el-dialog>

    <el-dialog
      v-model="toolboxDialog.visible"
      title="数据库工具箱"
      width="760px"
      class="review-dialog toolbox-dialog"
      modal-class="database-review-dialog-overlay"
      destroy-on-close
    >
      <div class="toolbox-target">
        <span>当前类型：{{ toolboxDatabaseName }}</span>
        <el-segmented v-model="toolboxScope" :options="toolboxScopeOptions" size="small" @change="loadToolboxTools" />
        <el-button
          class="warning-action compact"
          type="warning"
          :icon="Refresh"
          :loading="toolboxDialog.loading"
          @click="loadToolboxTools"
        >
          {{ toolboxRefreshText }}
        </el-button>
      </div>
      <div v-loading="toolboxDialog.loading" class="toolbox-grid">
        <article v-for="tool in toolboxTools" :key="tool.key" class="toolbox-card">
          <div class="toolbox-card-head">
            <div>
              <strong>{{ tool.name }}</strong>
              <span>{{ tool.usageScenario || tool.description }}</span>
              <div class="toolbox-tags">
                <el-tag size="small" effect="light" round>{{ tool.databaseTypeName || '-' }}</el-tag>
                <el-tag size="small" effect="light" round>{{ capabilityText(tool.capability) }}</el-tag>
              </div>
            </div>
            <el-tag :type="toolStatusTag(tool)" effect="light" round>{{ toolStatusText(tool) }}</el-tag>
          </div>
          <div class="toolbox-actions">
            <el-button class="primary-action compact" :loading="tool.loading" @click="checkToolStatus(tool)">
              {{ tool.result?.checkButtonText || '重新检测' }}
            </el-button>
            <el-button class="secondary-action compact" :icon="DocumentCopy" @click="copyToolInstallGuide(tool)">复制安装命令</el-button>
          </div>
          <div v-if="tool.result" class="toolbox-result">
            <span>安装路径：{{ getToolInstallPath(tool) }}</span>
            <span v-if="tool.url || tool.downloadUrl">
              下载地址：
              <a :href="tool.url || tool.downloadUrl" target="_blank" rel="noreferrer">GitHub Releases</a>
            </span>
            <span>检测结果：{{ getToolCheckMessage(tool.result) }}</span>
          </div>
          <el-collapse class="toolbox-install-collapse">
            <el-collapse-item title="安装命令" :name="tool.key">
              <pre class="toolbox-install-command">{{ getToolInstallCommand(tool) }}</pre>
            </el-collapse-item>
          </el-collapse>
        </article>
        <el-empty v-if="!toolboxTools.length && !toolboxDialog.loading" description="暂无工具状态" />
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { DocumentCopy, Plus, Refresh, RefreshRight, Search, Tools } from '@element-plus/icons-vue'
import dbApi from '@/api/database'
import cmdbApi from '@/api/cmdb'
import systemApi from '@/api/system'
import ticketApi from '@/api/ticket'
import storage from '@/utils/storage'
import CodeEditor from '@/components/CodeEditor.vue'
import databaseSvg from '@/assets/image/数据库.svg'
import mysqlSvg from '@/assets/image/mysqldb.svg'
import postgresqlSvg from '@/assets/image/PostgreSQL.svg'
import redisSvg from '@/assets/image/redis.svg'
import mongoSvg from '@/assets/image/mongodb.svg'
import elasticsearchSvg from '@/assets/image/Elasticsearch.svg'

const loading = ref(false)
const checking = ref(false)
const submitting = ref(false)
const databaseLoading = ref(false)
const logsLoading = ref(false)
const rollbackLoading = ref(false)
const backupCleanupLoading = ref(false)
const flashbackLoading = ref(false)
const sqlEditorHeight = ref(124)
const tickets = ref([])
const total = ref(0)
const assets = ref([])
const databaseOptions = ref([])
const sqlCompletionItems = ref([])
const users = ref([])
const sqlWorkflows = ref([])
const approvalWorkflowDetail = ref(null)
const checkResult = ref(null)
const executionLogs = ref([])
const rollbackInfo = ref(null)
const rollbackRecords = ref([])
const activeRollbackRecordId = ref('')
const databaseSelectRef = ref(null)
const detailTab = ref('base')
const currentUser = ref({})
const queryResult = reactive({
  loading: false,
  columns: [],
  rows: [],
  total: 0,
  durationMs: 0,
  sql: ''
})

const query = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  ticketType: '',
  status: '',
  assetId: '',
  database: ''
})

const createDialog = reactive({ visible: false })
const rollbackTicketDialog = reactive({ visible: false, submitting: false })
const execLogDialog = reactive({ visible: false, content: '' })
const toolboxDialog = reactive({ visible: false, loading: false })
const toolboxScope = ref('current')
const toolboxScopeOptions = [
  { label: '当前数据库', value: 'current' },
  { label: '全部工具', value: 'all' }
]
const rollbackTicketForm = reactive({
  rollbackRecordId: '',
  title: '',
  approverId: '',
  executorId: '',
  remark: ''
})
const toolboxTools = ref([])
const detailDrawer = reactive({ visible: false, loading: false })
const detailData = reactive({
  ticket: null,
  statements: [],
  reviewResults: [],
  approvalRecords: [],
  executionRecords: [],
  rollbackRecords: []
})

const ticketForm = reactive({
  ticketType: 'dml',
  title: '',
  assetId: '',
  database: '',
  schema: '',
  sql: '',
  backupRequired: true,
  rollbackSql: '',
  useGhOst: false,
  validDuration: '4h',
  maxRows: 1000,
  allowExport: false,
  approvalMode: 'workflow',
  workflowId: '',
  approverId: '',
  executorId: '',
  remark: ''
})

const typeTabs = [
  { value: '', label: '全部工单' },
  { value: 'dml', label: 'DML 工单' },
  { value: 'ddl', label: 'DDL 工单' },
  { value: 'query', label: '查询工单' }
]

const statusOptions = [
  { value: 'pending_review', label: '待审批' },
  { value: 'approved', label: '已审批' },
  { value: 'rejected', label: '已驳回' },
  { value: 'running', label: '执行中' },
  { value: 'succeeded', label: '执行成功' },
  { value: 'failed', label: '执行失败' },
  { value: 'canceled', label: '已撤销' }
]

const detailTicket = computed(() => detailData.ticket || {})
const checkResults = computed(() => normalizeReviewResults(checkResult.value))
const detailReviewResults = computed(() => normalizeList(detailData.reviewResults))
const canSubmitTicket = computed(() => Boolean(checkResult.value && (checkResult.value.allowSubmit ?? checkResult.value.reviewPassed)))
const activeRollbackRecord = computed(() => rollbackRecords.value.find(record => String(record.id) === String(activeRollbackRecordId.value)) || rollbackRecords.value[0] || null)
const hasBackupTables = computed(() => rollbackRecords.value.some(record => record.backupTable && String(record.backupTable).startsWith('_aiops_backup_')))
const hasBinlogPoint = computed(() => Boolean(
  rollbackInfo.value?.binlogStartFile &&
  rollbackInfo.value?.binlogStartPos !== undefined &&
  rollbackInfo.value?.binlogEndFile &&
  rollbackInfo.value?.binlogEndPos !== undefined
))
const isDetailMySQL = computed(() => getAssetEngine(detailTicket.value) === 'mysql')
const isDetailPostgreSQL = computed(() => getAssetEngine(detailTicket.value) === 'postgresql')
const canGenerateFlashback = computed(() => Boolean(isDetailMySQL.value && getRollbackRecordId() && hasBinlogPoint.value && !rollbackInfo.value?.rollbackSql))
const currentUserId = computed(() => currentUser.value?.id || currentUser.value?.userId || currentUser.value?.adminId)
const currentUsername = computed(() => currentUser.value?.username || currentUser.value?.name || currentUser.value?.nickname)
const selectedTicketAsset = computed(() => assets.value.find(asset => Number(getAssetId(asset)) === Number(ticketForm.assetId)) || null)
const applicableSqlWorkflows = computed(() => {
  const environment = String(selectedTicketAsset.value?.environment || selectedTicketAsset.value?.Environment || '').toLowerCase()
  return sqlWorkflows.value.filter(item => (item.businessTypes || []).includes('sql_review') && (!(item.environments || []).length || !environment || item.environments.includes(environment)))
})
const selectedAssetEngine = computed(() => getAssetEngine(selectedTicketAsset.value))
const isSelectedMySQL = computed(() => selectedAssetEngine.value === 'mysql')
const isSelectedPostgreSQL = computed(() => selectedAssetEngine.value === 'postgresql')
const showSchemaField = computed(() => ['postgresql', 'oracle', 'sqlserver'].includes(selectedAssetEngine.value))
const schemaPlaceholder = computed(() => ({
  postgresql: 'public',
  oracle: '当前用户',
  sqlserver: 'dbo'
}[selectedAssetEngine.value] || '可选'))
const dmlProtectionText = computed(() => (
  isSelectedPostgreSQL.value
    ? '开启后保留 WAL 解析回滚依赖提示；自动回滚依赖 WalMiner 能力，当前以审核提示和工具检测为主'
    : '开启后执行前后记录 binlog 位点，可通过 my2sql 生成闪回 SQL'
))
const ddlCapabilityText = computed(() => (
  isSelectedPostgreSQL.value
    ? 'PostgreSQL 大表 DDL 建议使用 pgroll，当前仅检测工具状态，执行能力以后端支持为准'
    : ''
))
const rollbackLabel = computed(() => (
  ticketForm.ticketType === 'ddl'
    ? '回滚 SQL（建议填写）'
    : '回滚 SQL（可选）'
))
const remarkPlaceholder = computed(() => (
  ticketForm.ticketType === 'query'
    ? '申请原因 / 使用场景 / 数据范围'
    : '变更原因 / 窗口 / 影响范围'
))
const groupedCheckResults = computed(() => {
  const groups = { error: [], warn: [], info: [] }
  checkResults.value.forEach(item => {
    const level = String(item.level || item.severity || item.type || 'info').toLowerCase()
    if (['error', 'danger', 'critical', 'block'].includes(level)) groups.error.push(item)
    else if (['warn', 'warning', 'medium', 'high'].includes(level)) groups.warn.push(item)
    else groups.info.push(item)
  })
  return groups
})
const checkStatements = computed(() => normalizeList(checkResult.value?.statements || checkResult.value?.sqlStatements))
const checkSubmitText = computed(() => {
  const allow = checkResult.value?.allowSubmit ?? checkResult.value?.reviewPassed
  return allow ? '允许提交' : '禁止提交'
})
const checkMetricCards = computed(() => {
  const result = checkResult.value || {}
  const statementType = result.statementType || result.sqlType || result.type || checkStatements.value[0]?.type || ticketForm.ticketType.toUpperCase()
  const rows = pickFirst(result, ['estimatedAffectedRows', 'affectedRows', 'estimatedRows', 'scanRows', 'estimatedScanRows'], '-')
  const rollbackGenerated = result.rollbackGenerated ?? Boolean(result.rollbackSql || result.rollbackSQL)
  const rollbackText = ticketForm.ticketType === 'query' ? '不需要' : rollbackGenerated ? '已生成' : '未生成'
  return [
    { label: '语句数', value: result.statementCount || result.sqlCount || checkStatements.value.length || 1 },
    { label: '语句类型', value: String(statementType || '-').toUpperCase() },
    { label: ticketForm.ticketType === 'query' ? '预计扫描行' : '影响行预估', value: rows },
    { label: '提交限制', value: checkSubmitText.value },
    { label: '回滚 SQL', value: rollbackText }
  ]
})
const detailReviewSummary = computed(() => {
  const row = detailTicket.value
  const items = [
    { label: '审核状态', value: row.reviewPassed === false ? '未通过' : '已通过' },
    { label: '风险等级', value: riskText(row.riskLevel) },
    { label: '语句类型', value: ticketTypeText(row.ticketType || row.type) },
    { label: '规则命中', value: `${detailReviewResults.value.length} 条` },
    { label: isQueryTicket(row) ? '返回行数' : '影响行数', value: row.affectedRows ?? row.estimatedRows ?? queryResult.total ?? '-' },
    { label: '备份/回滚', value: isQueryTicket(row) ? '不需要' : row.rollbackRequired || row.backupRequired ? '已要求' : '未要求' }
  ]
  if (row.useGhOst) items.splice(3, 0, { label: '在线 DDL', value: 'gh-ost' })
  return items
})
const canApproveWorkflowDetail = computed(() => (approvalWorkflowDetail.value?.tasks || []).some(task => task.status === 'pending' && isCurrentUser(task.assigneeId, task.assigneeName)))
const canApproveDetail = computed(() => canApproveTicket(detailTicket.value) || canApproveWorkflowDetail.value)
const canCancelDetail = computed(() => canCancelTicket(detailTicket.value))
const canExecuteDetail = computed(() => canExecuteTicket(detailTicket.value))
const currentActionTip = computed(() => {
  const row = detailTicket.value
  if (row.status === 'pending_review') return `当前等待 ${approvalWorkflowDetail.value?.instance?.currentNodeName || row.approverName || '审批人'} 审批`
  if (row.status === 'approved') return `当前等待 ${row.executorName || '执行人'} 执行`
  if (row.status === 'running') return `当前由 ${row.executorName || '执行人'} 执行中`
  if (row.status === 'rejected') return `已由 ${row.approverName || '审批人'} 驳回`
  if (row.status === 'succeeded') return '工单已执行成功'
  if (row.status === 'failed') return '工单执行失败'
  if (row.status === 'canceled') return '工单已撤销'
  return ''
})
const workflowSteps = computed(() => {
  const row = detailTicket.value
  const status = row.status
  const approved = ['approved', 'running', 'succeeded', 'failed'].includes(status)
  const executed = ['succeeded', 'failed'].includes(status)
  const rejected = status === 'rejected'
  return [
    {
      key: 'submit',
      label: '提交',
      name: row.submitterName || row.creatorName || row.createdBy,
      time: getSubmittedTime(row),
      state: 'done',
      done: true
    },
    {
      key: 'approve',
      label: '审批',
      name: row.approverName,
      time: getReviewedTime(row),
      state: rejected ? 'failed' : approved ? 'done' : 'pending',
      done: approved
    },
    {
      key: 'execute',
      label: '执行',
      name: row.executorName,
      time: getExecutedTime(row),
      state: executed ? (status === 'failed' ? 'failed' : 'done') : status === 'running' ? 'active' : 'pending',
      done: executed && status !== 'failed'
    }
  ]
})
const workflowProgress = computed(() => {
  const states = workflowSteps.value.map(step => step.state)
  if (states[2] === 'done' || states[2] === 'failed') return 'full'
  if (states[1] === 'done' || states[1] === 'failed') return 'two'
  return 'one'
})
const detailItems = computed(() => {
  const row = detailTicket.value
  const items = [
    { label: '工单号', value: row.ticketNo },
    { label: '类型', value: ticketTypeText(row.ticketType || row.type) },
    { label: '状态', value: statusText(row.status) },
    { label: '风险', value: riskText(row.riskLevel) },
    { label: '资产', value: row.assetName || row.assetInstance || row.assetId },
    { label: '数据库', value: getTicketDatabase(row) },
    { label: 'Schema', value: getTicketSchema(row) },
    { label: '环境', value: row.environment },
    { label: '发起人', value: row.submitterName },
    { label: '审批人', value: row.approverName },
    { label: '执行人', value: row.executorName },
    { label: '提交时间', value: getSubmittedTime(row) },
    { label: '审批时间', value: getReviewedTime(row) },
    { label: '执行时间', value: getExecutedTime(row) },
    { label: '备注', value: row.remark }
  ]
  return items.filter(item => item.label !== 'Schema' || shouldShowDetailSchema(row))
})

const unwrap = response => {
  const body = response?.data || response || {}
  if (body.code && body.code !== 200 && body.success !== true) throw new Error(body.message || '请求失败')
  return body.data !== undefined ? body.data : body
}

const normalizeList = payload => {
  if (Array.isArray(payload)) return payload
  if (Array.isArray(payload?.list)) return payload.list
  if (Array.isArray(payload?.rows)) return payload.rows
  if (Array.isArray(payload?.tools)) return payload.tools
  if (Array.isArray(payload?.data)) return payload.data
  return []
}

const normalizeSqlCompletion = payload => {
  if (!payload) return []
  const items = []
  const pushKeyword = keyword => items.push({ label: keyword, kind: 'keyword' })
  const pushDatabase = database => items.push({ label: database.name || database.database || database, kind: 'database' })
  const pushTable = table => {
    const label = table.name || table.tableName || table.objectName || table.label || table
    const columns = Array.isArray(table.columns) ? table.columns.map(column => ({
      columnName: column.name || column.columnName || column.label || column,
      columnType: column.type || column.columnType || column.dataType || ''
    })) : []
    items.push({ tableName: label, type: table.type || 'table', columns })
  }
  const pushColumn = column => items.push({
    label: column.name || column.columnName || column.label || column,
    kind: 'column',
    detail: column.type || column.columnType || column.dataType || ''
  })

  normalizeList(payload).forEach(item => items.push(item))
  ;(payload.keywords || []).forEach(pushKeyword)
  ;(payload.databases || []).forEach(pushDatabase)
  ;(payload.tables || payload.objects || []).forEach(pushTable)
  ;(payload.columns || payload.fields || []).forEach(pushColumn)
  return items
}

const parseMaybeJson = (value, fallback = []) => {
  if (Array.isArray(value)) return value
  if (!value) return fallback
  if (typeof value === 'string') {
    try {
      return JSON.parse(value || '[]')
    } catch (error) {
      return fallback
    }
  }
  return fallback
}

const getResultColumnName = column => {
  if (typeof column === 'string') return column
  return column?.name || column?.columnName || column?.label || column?.field || ''
}

const formatResultCell = value => {
  if (value === null) return 'NULL'
  if (value === undefined) return ''
  if (typeof value === 'object') return JSON.stringify(value)
  return String(value)
}

const hasQueryResultRecord = record => {
  const columns = record?.resultColumns || record?.result_columns
  const rows = record?.resultRows || record?.result_rows
  if (!columns && !rows) return false
  const status = String(record?.status || '').toLowerCase()
  return !status || status === 'succeeded'
}

const pickQueryResultRecord = records => {
  const list = normalizeList(records)
  for (let index = list.length - 1; index >= 0; index--) {
    if (hasQueryResultRecord(list[index])) return list[index]
  }
  return null
}

const applyQueryResultRecord = record => {
  const columns = parseMaybeJson(record?.resultColumns || record?.result_columns)
  const rows = parseMaybeJson(record?.resultRows || record?.result_rows)
  applyQueryResultPayload({
    columns,
    rows,
    total: rows.length,
    durationMs: record?.durationMs || record?.costMs,
    sql: record?.sql || record?.sqlText || record?.sql_text
  })
}

const applyQueryResultPayload = payload => {
  let columns = normalizeList(payload?.columns)
  const rows = normalizeList(payload?.rows)
  if (!columns.length && rows[0] && typeof rows[0] === 'object') {
    columns = Object.keys(rows[0]).map(name => ({ name }))
  }
  queryResult.columns = columns.map(column => {
    if (typeof column === 'string') return { name: column }
    return column
  }).filter(column => getResultColumnName(column))
  queryResult.rows = rows
  queryResult.total = Number(payload?.total ?? rows.length ?? 0)
  queryResult.durationMs = Number(payload?.durationMs || 0)
  queryResult.sql = payload?.sql || ''
}

const getCurrentUser = () => {
  const store = storage.getStorage?.() || {}
  return storage.getItem('userInfo') ||
    storage.getItem('user') ||
    storage.getItem('sysAdmin') ||
    storage.getItem('admin') ||
    storage.getItem('adminInfo') ||
    store.userInfo ||
    store.user ||
    store.sysAdmin ||
    store.admin ||
    {}
}

const normalizeReviewResults = result => {
  const list = [
    ...normalizeList(result?.reviewResults),
    ...normalizeList(result?.rules),
    ...normalizeList(result?.errors).map(item => ({ ...item, level: item.level || 'error' })),
    ...normalizeList(result?.warnings).map(item => ({ ...item, level: item.level || 'warn' })),
    ...normalizeList(result?.infos).map(item => ({ ...item, level: item.level || 'info' }))
  ]
  return list.map(item => ({
    ...item,
    level: normalizeReviewLevel(item.level || item.severity || item.type)
  }))
}

const normalizeReviewLevel = level => {
  const value = String(level || 'info').toLowerCase()
  if (['error', 'danger', 'critical', 'block', 'failed'].includes(value)) return 'error'
  if (['warn', 'warning', 'medium', 'high'].includes(value)) return 'warn'
  return 'info'
}

const pickFirst = (source, keys, fallback = '') => {
  for (const key of keys) {
    const value = source?.[key]
    if (value !== undefined && value !== null && value !== '') return value
  }
  return fallback
}

const isCurrentUser = (id, name) => {
  if (id && currentUserId.value && Number(id) === Number(currentUserId.value)) return true
  if (name && currentUsername.value && String(name) === String(currentUsername.value)) return true
  return false
}

const canApproveTicket = row => row?.status === 'pending_review' && isCurrentUser(row.approverId, row.approverName)
const canCancelTicket = row => ['pending_review', 'approved'].includes(row?.status) && isCurrentUser(row.submitterId, row.submitterName)
const canExecuteTicket = row => row?.status === 'approved' && isCurrentUser(row.executorId, row.executorName)

const dbTypeMap = {
  1: 'MySQL',
  2: 'PostgreSQL',
  3: 'Oracle',
  4: 'Redis',
  5: 'MongoDB',
  6: 'SQL Server',
  7: 'Elasticsearch'
}

const dbTypeIconMap = {
  1: mysqlSvg,
  2: postgresqlSvg,
  3: databaseSvg,
  4: redisSvg,
  5: mongoSvg,
  6: databaseSvg,
  7: elasticsearchSvg
}

const getAssetId = asset => asset?.id || asset?.assetId || asset?.dbId
const getAssetTypeNumber = asset => Number(asset?.type || asset?.engine || asset?.dbType || asset?.databaseType || 0)
const getAssetEngine = asset => {
  const raw = asset?.type || asset?.engine || asset?.dbType || asset?.databaseType || asset?.driver || asset?.kind
  const value = String(raw || '').toLowerCase()
  return ({
    1: 'mysql',
    2: 'postgresql',
    3: 'oracle',
    4: 'redis',
    5: 'mongodb',
    6: 'sqlserver',
    7: 'elasticsearch',
    mysql: 'mysql',
    mariadb: 'mysql',
    pg: 'postgresql',
    postgresql: 'postgresql',
    postgres: 'postgresql',
    oracle: 'oracle',
    redis: 'redis',
    mongodb: 'mongodb',
    mongo: 'mongodb',
    sqlserver: 'sqlserver',
    mssql: 'sqlserver',
    elasticsearch: 'elasticsearch',
    es: 'elasticsearch'
  }[value] || value)
}
const getAssetTitle = asset => asset?.name || asset?.assetName || asset?.host || `数据源 #${getAssetId(asset)}`
const getAssetIcon = asset => dbTypeIconMap[getAssetTypeNumber(asset)] || databaseSvg
const getDatabaseName = node => node?.database || node?.name || node?.label || node?.objectName || ''
const isEmptyTime = value => !value || String(value).startsWith('0001/01/01') || String(value).startsWith('0001-01-01')
const displayTime = value => isEmptyTime(value) ? '-' : value
const getTicketDbType = row => row?.assetTypeName || dbTypeMap[row?.engine] || getAssetEngine(row) || '-'
const getTicketDbEngine = row => getAssetEngine(row) || 'default'
const getTicketDbIcon = row => dbTypeIconMap[Number(row?.engine || row?.type || row?.dbType || 0)] || databaseSvg
const getTicketDatabase = row => row?.databaseName || row?.database || row?.dbName || '-'
const getTicketSchema = row => row?.schemaName || row?.schema || '-'
const getTicketSql = row => row?.sqlContent || row?.sql || row?.finalSql || '-'
const getExecutionSql = row => row?.sqlText || row?.sql || row?.statement || '-'
const getExecutionLog = row => row?.execLog || row?.executionLog || row?.log || ''
const isQueryTicket = row => String(row?.ticketType || row?.type || '').toLowerCase() === 'query'
const shouldShowDetailSchema = row => {
  const engine = getAssetEngine(row)
  return ['postgresql', 'oracle', 'sqlserver'].includes(engine) && getTicketSchema(row) !== '-'
}
const getSubmittedTime = row => displayTime(row?.submittedAt || row?.createdAt)
const getReviewedTime = row => displayTime(row?.reviewedAt || row?.approvedAt)
const getExecutedTime = row => displayTime(row?.executedAt || row?.finishedAt)
const getRollbackRecordId = () => activeRollbackRecord.value?.rollbackRecordId || activeRollbackRecord.value?.recordId || activeRollbackRecord.value?.id || rollbackInfo.value?.id || ''
const formatBinlogPoint = (record, type) => {
  const file = record?.[`binlog${type === 'start' ? 'Start' : 'End'}File`]
  const pos = record?.[`binlog${type === 'start' ? 'Start' : 'End'}Pos`]
  return file && pos !== undefined && pos !== null && pos !== '' ? `${file}:${pos}` : '-'
}
const getDefaultSchema = asset => {
  const engine = getAssetEngine(asset)
  if (engine === 'postgresql') return 'public'
  if (engine === 'sqlserver') return 'dbo'
  if (engine === 'oracle') return asset?.username || asset?.user || asset?.account || ''
  return ''
}
const userLabel = user => user?.nickname ? `${user.nickname} (${user.username || user.id})` : user?.username || `用户 ${user?.id}`

const statusText = status => ({
  pending_review: '待审批',
  approved: '已审批',
  rejected: '已驳回',
  running: '执行中',
  succeeded: '执行成功',
  failed: '执行失败',
  canceled: '已撤销'
}[status] || status || '-')

const statusTag = status => ({
  pending_review: 'warning',
  approved: 'primary',
  rejected: 'danger',
  running: 'warning',
  succeeded: 'success',
  failed: 'danger',
  canceled: 'info'
}[status] || 'info')

const executionStatusText = status => ({
  pending: '待执行',
  running: '执行中',
  succeeded: '成功',
  success: '成功',
  failed: '失败'
}[status] || status || '-')

const executionStatusTag = status => ({
  pending: 'info',
  running: 'warning',
  succeeded: 'success',
  success: 'success',
  failed: 'danger'
}[status] || 'info')

const approvalActionText = action => ({
  approve: '审批通过',
  approved: '审批通过',
  reject: '审批驳回',
  rejected: '审批驳回',
  cancel: '撤销',
  canceled: '撤销',
  execute: '执行',
  executed: '执行'
}[action] || action || '-')

const approvalActionTag = action => ({
  approve: 'success',
  approved: 'success',
  reject: 'danger',
  rejected: 'danger',
  cancel: 'warning',
  canceled: 'warning',
  execute: 'primary',
  executed: 'primary'
}[action] || 'info')

const genericWorkflowStatusText = status => ({ running: '审批中', approved: '已通过', rejected: '已驳回', canceled: '已撤销' }[status] || status || '-')
const genericTaskStatusText = status => ({ pending: '待审批', approved: '已通过', rejected: '已驳回', canceled: '已取消' }[status] || status || '-')

const ticketTypeText = type => ({
  dml: 'DML',
  ddl: 'DDL',
  query: '查询',
  DML: 'DML',
  DDL: 'DDL',
  QUERY: '查询'
}[type] || type || '-')

const riskText = risk => ({
  low: '低',
  medium: '中',
  high: '高',
  critical: '严重'
}[risk] || risk || '-')

const dedupeAssets = list => {
  const seen = new Set()
  return list.filter(asset => {
    const id = getAssetId(asset)
    if (!id || seen.has(String(id))) return false
    seen.add(String(id))
    return true
  })
}

const loadAssets = async () => {
  try {
    const baseParams = {
      page: 1,
      pageSize: 100,
      sortBy: 'updatedAt',
      sortOrder: 'desc'
    }
    const [mysqlPayload, postgresqlPayload] = await Promise.all([
      cmdbApi.getDatabaseAssetList({ ...baseParams, type: 1 }),
      cmdbApi.getDatabaseAssetList({ ...baseParams, type: 2 })
    ])
    assets.value = dedupeAssets([
      ...normalizeList(unwrap(mysqlPayload)),
      ...normalizeList(unwrap(postgresqlPayload))
    ])
  } catch (error) {
    ElMessage.error(error.message || '获取数据库资产失败')
  }
}

const loadUsers = async () => {
  try {
    const payload = unwrap(await systemApi.queryAdminList({ pageNum: 1, pageSize: 100, status: 1 }))
    users.value = normalizeList(payload)
  } catch (error) {
    ElMessage.error(error.message || '获取用户列表失败')
  }
}

const loadSqlWorkflows = async () => {
  try {
    const payload = unwrap(await ticketApi.workflows({ page: 1, pageSize: 100, enabled: 1 }))
    sqlWorkflows.value = normalizeList(payload)
  } catch (error) {
    ElMessage.error(error.message || '获取 SQL 审批流失败')
  }
}

const loadTicketDatabases = async assetId => {
  if (!assetId || databaseLoading.value) return
  databaseLoading.value = true
  try {
    const payload = unwrap(await dbApi.getCrudTree({ assetId, level: 'root' }))
    databaseOptions.value = normalizeList(payload)
      .map(node => {
        const name = getDatabaseName(node)
        return name ? { label: name, value: name } : null
      })
      .filter(Boolean)
  } catch (error) {
    databaseOptions.value = []
    ElMessage.error(error.message || '获取数据库列表失败')
  } finally {
    databaseLoading.value = false
  }
}

const loadTicketSqlCompletion = async () => {
  if (!ticketForm.assetId) {
    sqlCompletionItems.value = []
    return
  }
  try {
    const payload = unwrap(await dbApi.getWorkbenchSqlCompletion({
      assetId: ticketForm.assetId,
      database: ticketForm.database || undefined
    }))
    sqlCompletionItems.value = normalizeSqlCompletion(payload)
  } catch (error) {
    sqlCompletionItems.value = []
  }
}

const handleTicketAssetChange = async assetId => {
  const asset = assets.value.find(item => Number(getAssetId(item)) === Number(assetId))
  ticketForm.database = ''
  ticketForm.schema = getDefaultSchema(asset)
  databaseOptions.value = []
  sqlCompletionItems.value = []
  checkResult.value = null
  await loadTicketDatabases(assetId)
  loadTicketSqlCompletion()
  databaseSelectRef.value?.focus?.()
  databaseSelectRef.value?.toggleMenu?.()
}

const resetSqlEditorHeight = () => {
  sqlEditorHeight.value = 124
}

const startSqlResize = event => {
  event.preventDefault()
  const startY = event.clientY
  const startHeight = sqlEditorHeight.value
  const handleMouseMove = moveEvent => {
    const nextHeight = startHeight + moveEvent.clientY - startY
    sqlEditorHeight.value = Math.min(320, Math.max(124, nextHeight))
  }
  const handleMouseUp = () => {
    window.removeEventListener('mousemove', handleMouseMove)
    window.removeEventListener('mouseup', handleMouseUp)
  }
  window.addEventListener('mousemove', handleMouseMove)
  window.addEventListener('mouseup', handleMouseUp)
}

const buildQueryParams = () => ({
  page: query.page,
  pageSize: query.pageSize,
  keyword: query.keyword || undefined,
  ticketType: query.ticketType || undefined,
  status: query.status || undefined,
  assetId: query.assetId || undefined,
  database: query.database || undefined
})

const loadTickets = async () => {
  loading.value = true
  try {
    const params = buildQueryParams()
    const api = query.ticketType === 'dml'
      ? dbApi.getDmlReviewTickets
      : query.ticketType === 'ddl'
        ? dbApi.getDdlReviewTickets
        : query.ticketType === 'query'
          ? (dbApi.getQueryReviewTickets || dbApi.getReviewTickets)
          : dbApi.getReviewTickets
    const payload = unwrap(await api(params))
    tickets.value = normalizeList(payload)
    total.value = Number(payload.total || payload.count || tickets.value.length || 0)
  } catch (error) {
    ElMessage.error(error.message || '获取工单列表失败')
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  query.page = 1
  loadTickets()
}

const resetSearch = () => {
  Object.assign(query, { page: 1, pageSize: 10, keyword: '', ticketType: '', status: '', assetId: '', database: '' })
  loadTickets()
}

const openToolboxDialog = () => {
  toolboxDialog.visible = true
  toolboxScope.value = 'current'
  loadToolboxTools()
}

const selectedToolboxAsset = computed(() => assets.value.find(asset => Number(getAssetId(asset)) === Number(query.assetId)) || null)
const currentToolboxDbType = computed(() => {
  if (selectedToolboxAsset.value) return Number(selectedToolboxAsset.value.type || selectedToolboxAsset.value.engine || selectedToolboxAsset.value.dbType || 0)
  const firstTypedTicket = tickets.value.find(item => item.engine || item.databaseType || item.assetType || item.assetTypeName)
  if (firstTypedTicket?.engine || firstTypedTicket?.databaseType || firstTypedTicket?.assetType) {
    return Number(firstTypedTicket.engine || firstTypedTicket.databaseType || firstTypedTicket.assetType)
  }
  return 1
})
const toolboxDatabaseName = computed(() => {
  if (toolboxScope.value === 'all') return '全部'
  if (selectedToolboxAsset.value) return getTicketDbType(selectedToolboxAsset.value)
  if (toolboxTools.value[0]?.databaseTypeName) return toolboxTools.value[0].databaseTypeName
  if (currentToolboxDbType.value) return dbTypeMap[currentToolboxDbType.value] || '全部'
  return '全部'
})
const toolboxRefreshText = computed(() => (
  toolboxScope.value === 'all'
    ? '重新检测全部工具'
    : `重新检测${toolboxDatabaseName.value}工具`
))
const filterToolboxTools = tools => {
  if (toolboxScope.value === 'all') return tools
  const dbType = currentToolboxDbType.value
  if (!dbType) return tools
  return tools.filter(tool => Number(tool.databaseType) === Number(dbType))
}
const buildToolboxParams = tool => {
  if (toolboxScope.value === 'all') return {}
  const params = {}
  if (query.assetId) params.assetId = query.assetId
  else if (tool?.databaseType) params.dbType = tool.databaseType
  else if (currentToolboxDbType.value) params.dbType = currentToolboxDbType.value
  return params
}
const normalizeToolboxTool = tool => ({
  ...tool,
  key: tool.key || tool.toolName || tool.name || tool.installPath || `${tool.databaseType || 'db'}-${tool.capability || 'tool'}`,
  name: tool.toolName || tool.name || '-',
  result: tool,
  loading: false
})
const isToolInstalled = result => Boolean(result?.installed ?? result?.available ?? result?.ok ?? result?.success ?? result?.exists)
const getToolStatus = result => {
  if (!result) return 'unchecked'
  const rawStatus = String(result.status || result.state || '').toLowerCase()
  if (rawStatus === 'installed') return 'installed'
  if (['not_installed', 'missing', 'not_found'].includes(rawStatus)) return 'missing'
  if (rawStatus === 'abnormal') return 'error'
  if (['error', 'exception', 'failed', 'fail'].includes(rawStatus) || result.exception || result.checkError) return 'error'
  if (isToolInstalled(result)) return 'installed'
  return 'missing'
}
const toolStatusText = tool => {
  if (tool.loading) return '检查中'
  if (!tool.result) return '未检查'
  if (tool.result.statusText) return tool.result.statusText
  return ({
    installed: '已安装',
    missing: '未安装',
    error: '异常'
  }[getToolStatus(tool.result)] || '未检查')
}
const toolStatusTag = tool => {
  if (tool.loading) return 'warning'
  if (!tool.result) return 'info'
  return ({
    installed: 'success',
    missing: 'warning',
    error: 'danger'
  }[getToolStatus(tool.result)] || 'info')
}
const getToolCheckMessage = result => (
  result?.lastCheckMessage ||
  result?.checkMessage ||
  result?.message ||
  result?.error ||
  result?.stderr ||
  '-'
)
const getToolInstallPath = tool => tool?.installPath || tool?.result?.installPath || tool?.result?.path || tool?.result?.binaryPath || tool?.result?.binPath || tool?.result?.toolPath || '-'
const getToolInstallCommand = tool => {
  const text = tool?.installCommand || tool?.installGuide || ''
  if (!text) return '暂无安装命令'
  const skipPatterns = [
    /^工具需由/,
    /^安装路径/,
    /^下载地址/,
    /^检测命令/,
    /^检测结果/,
    /^使用场景/,
    /^1[.、]\s*从/,
    /^2[.、]\s*解压/,
    /^3[.、]\s*执行/,
    /^4[.、]\s*确认/
  ]
  const lines = String(text)
    .split('\n')
    .map(line => line.trim())
    .filter(line => line && !skipPatterns.some(pattern => pattern.test(line)))
  return lines.length ? lines.join('\n') : text
}
const capabilityText = capability => ({
  flashback_rollback: '回滚工具',
  online_ddl: '在线DDL工具'
}[capability] || capability || '-')

const loadToolboxTools = async tool => {
  toolboxDialog.loading = true
  try {
    const payload = unwrap(await dbApi.getDbToolsStatus(buildToolboxParams(tool)))
    toolboxTools.value = filterToolboxTools(normalizeList(payload).map(normalizeToolboxTool))
  } catch (error) {
    ElMessage.error(error.message || '获取工具状态失败')
  } finally {
    toolboxDialog.loading = false
  }
}

const checkToolStatus = async tool => {
  if (!tool) return
  tool.loading = true
  try {
    await loadToolboxTools(tool)
    ElMessage.success(`${tool.name} 检查完成`)
  } catch (error) {
    tool.result = { status: 'error', lastCheckMessage: error.message || '检查失败' }
    ElMessage.error(error.message || `${tool.name} 检查失败`)
  } finally {
    tool.loading = false
  }
}

const copyToolInstallGuide = async tool => {
  if (!tool) return
  const command = getToolInstallCommand(tool)
  if (!command) {
    ElMessage.warning('暂无安装命令')
    return
  }
  if (await copyText(command)) {
    ElMessage.success('已复制安装命令')
  } else {
    ElMessage.error('复制失败，请手动复制')
  }
}

const switchType = type => {
  query.ticketType = type
  handleSearch()
}

watch(() => ticketForm.ticketType, type => {
  checkResult.value = null
  if (type === 'query') {
    ticketForm.rollbackSql = ''
    ticketForm.backupRequired = false
    ticketForm.useGhOst = false
    return
  }
  ticketForm.backupRequired = type === 'dml'
  if (type === 'ddl') ticketForm.backupRequired = false
  if (type !== 'ddl') ticketForm.useGhOst = false
})

watch(selectedAssetEngine, () => {
  if (!isSelectedMySQL.value) ticketForm.useGhOst = false
  if (!showSchemaField.value) {
    ticketForm.schema = ''
    return
  }
  if (!ticketForm.schema) ticketForm.schema = getDefaultSchema(selectedTicketAsset.value)
})

watch(() => [ticketForm.database, ticketForm.schema, ticketForm.sql], () => {
  checkResult.value = null
})

watch(() => [ticketForm.assetId, ticketForm.database], () => {
  loadTicketSqlCompletion()
})

watch(activeRollbackRecord, record => {
  rollbackInfo.value = record || null
})

const openCreateDialog = () => {
  Object.assign(ticketForm, {
    ticketType: 'dml',
    title: '',
    assetId: '',
    database: '',
    schema: '',
    sql: '',
    backupRequired: true,
    rollbackSql: '',
    useGhOst: false,
    validDuration: '4h',
    maxRows: 1000,
    allowExport: false,
    approvalMode: sqlWorkflows.value.length ? 'workflow' : 'legacy',
    workflowId: '',
    approverId: '',
    executorId: '',
    remark: ''
  })
  checkResult.value = null
  databaseOptions.value = []
  sqlCompletionItems.value = []
  createDialog.visible = true
}

const validateTicketForm = () => {
  const approvalMissing = ticketForm.approvalMode === 'workflow' ? !ticketForm.workflowId : !ticketForm.approverId
  if (!ticketForm.title || !ticketForm.assetId || !ticketForm.database || !ticketForm.sql || approvalMissing || !ticketForm.executorId) {
    ElMessage.warning('请完整填写标题、资产、数据库、SQL、审批配置和执行人')
    return false
  }
  if (ticketForm.ticketType === 'query' && !/^\s*(select|with)\b/i.test(ticketForm.sql || '')) {
    ElMessage.warning('查询工单仅支持 SELECT / WITH 只读 SQL')
    return false
  }
  return true
}

const reviewPayload = () => ({
  assetId: ticketForm.assetId,
  database: ticketForm.database,
  schema: ticketForm.schema,
  sql: ticketForm.sql,
  ticketType: ticketForm.ticketType,
  backupRequired: ticketForm.backupRequired,
  useGhOst: ticketForm.ticketType === 'ddl' && isSelectedMySQL.value && ticketForm.useGhOst,
  validDuration: ticketForm.validDuration,
  maxRows: ticketForm.maxRows,
  allowExport: ticketForm.allowExport
})

const runReviewCheck = async () => {
  if (!validateTicketForm()) return
  checking.value = true
  try {
    const api = ticketForm.ticketType === 'ddl'
      ? dbApi.checkDdlReview
      : ticketForm.ticketType === 'query'
        ? dbApi.checkQueryReview
        : dbApi.checkDmlReview
    checkResult.value = unwrap(await api(reviewPayload()))
    ElMessage.success(canSubmitTicket.value ? '检测通过' : '检测完成，请处理风险项')
  } catch (error) {
    ElMessage.error(error.message || 'SQL 检测失败')
  } finally {
    checking.value = false
  }
}

const submitTicket = async () => {
  if (!validateTicketForm()) return
  if (!canSubmitTicket.value) {
    ElMessage.warning('请先通过自动检测')
    return
  }
  submitting.value = true
  try {
    const api = ticketForm.ticketType === 'ddl'
      ? dbApi.createDdlReviewTicket
      : ticketForm.ticketType === 'query'
        ? dbApi.createQueryReviewTicket
        : dbApi.createDmlReviewTicket
    const payload = unwrap(await api({
      title: ticketForm.title,
      ...reviewPayload(),
      rollbackSql: ticketForm.rollbackSql,
      workflowId: ticketForm.approvalMode === 'workflow' ? Number(ticketForm.workflowId) : 0,
      approverId: ticketForm.approvalMode === 'legacy' ? ticketForm.approverId : 0,
      executorId: ticketForm.executorId,
      remark: ticketForm.remark
    }))
    createDialog.visible = false
    ElMessage.success('工单已提交')
    loadTickets()
    const id = payload.id || payload.ticketId
    if (id) openDetail({ id })
  } catch (error) {
    ElMessage.error(error.message || '提交工单失败')
  } finally {
    submitting.value = false
  }
}

const openDetail = async row => {
  detailDrawer.visible = true
  detailDrawer.loading = true
  detailTab.value = 'base'
  executionLogs.value = []
  rollbackInfo.value = null
  rollbackRecords.value = []
  activeRollbackRecordId.value = ''
  queryResult.columns = []
  queryResult.rows = []
  queryResult.total = 0
  queryResult.durationMs = 0
  queryResult.sql = ''
  approvalWorkflowDetail.value = null
  try {
    const payload = unwrap(await dbApi.getReviewTicketDetail(row.id || row.ticketId))
    detailData.ticket = payload.ticket || payload
    if (isQueryTicket(detailData.ticket) && detailTab.value === 'rollback') detailTab.value = 'base'
    detailData.statements = normalizeList(payload.statements)
    detailData.reviewResults = normalizeList(payload.reviewResults)
    detailData.approvalRecords = normalizeList(payload.approvalRecords)
    detailData.executionRecords = normalizeList(payload.executionRecords)
    detailData.rollbackRecords = normalizeList(payload.rollbackRecords)
    if (detailData.ticket.workflowInstanceId) {
      approvalWorkflowDetail.value = unwrap(await ticketApi.workflowInstance(detailData.ticket.workflowInstanceId))
    }
    const resultRecord = pickQueryResultRecord(detailData.executionRecords)
    if (resultRecord) applyQueryResultRecord(resultRecord)
  } catch (error) {
    ElMessage.error(error.message || '获取工单详情失败')
  } finally {
    detailDrawer.loading = false
  }
}

const ticketAction = async (row, action) => {
  const id = row.id || row.ticketId
  const actionText = { approve: '审批通过', reject: '驳回', cancel: '撤销', execute: '执行' }[action]
  try {
    const { value } = await ElMessageBox.prompt(`请输入${actionText}说明`, actionText, {
      inputType: 'textarea',
      inputPlaceholder: '可选',
      confirmButtonText: action === 'execute' ? '确认执行' : '确认',
      cancelButtonText: '取消'
    })
    const data = action === 'execute' ? { comment: value || '', confirmed: true } : { comment: value || '' }
    const api = {
      approve: dbApi.approveReviewTicket,
      reject: dbApi.rejectReviewTicket,
      cancel: dbApi.cancelReviewTicket,
      execute: dbApi.executeReviewTicket
    }[action]
    unwrap(await api(id, data))
    ElMessage.success(`${actionText}成功`)
    loadTickets()
    if (detailDrawer.visible) {
      await openDetail({ id })
      if (action === 'execute' && isQueryTicket(detailTicket.value)) {
        detailTab.value = 'result'
        loadQueryResult(id)
      }
    }
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    ElMessage.error(error.message || `${actionText}失败`)
  }
}

const handleDetailTabChange = name => {
  const id = detailTicket.value.id || detailTicket.value.ticketId
  if (name === 'logs') loadLogs(id)
  if (name === 'result' && isQueryTicket(detailTicket.value)) loadQueryResult(id)
  if (name === 'rollback' && !isQueryTicket(detailTicket.value)) loadRollback(id)
}

const loadLogs = async id => {
  if (!id) return
  logsLoading.value = true
  try {
    const payload = unwrap(await dbApi.getReviewTicketLogs(id))
    executionLogs.value = normalizeList(payload)
    const resultRecord = pickQueryResultRecord(executionLogs.value)
    if (resultRecord) applyQueryResultRecord(resultRecord)
  } catch (error) {
    ElMessage.error(error.message || '获取执行日志失败')
  } finally {
    logsLoading.value = false
  }
}

const loadQueryResult = async id => {
  if (!id || queryResult.loading) return
  if (queryResult.rows.length || queryResult.columns.length) return
  queryResult.loading = true
  try {
    const resultPayload = unwrap(await dbApi.getReviewTicketResult(id))
    applyQueryResultPayload(resultPayload)
    queryResult.loading = false
    return
  } catch (error) {
    // Older backends may not have the structured result endpoint; fall back to execution logs.
  }
  try {
    if (!executionLogs.value.length) {
      const payload = unwrap(await dbApi.getReviewTicketLogs(id))
      executionLogs.value = normalizeList(payload)
    }
    const resultRecord = pickQueryResultRecord(executionLogs.value) || pickQueryResultRecord(detailData.executionRecords)
    if (resultRecord) applyQueryResultRecord(resultRecord)
  } catch (error) {
    ElMessage.error(error.message || '获取查询结果失败')
  } finally {
    queryResult.loading = false
  }
}

const loadRollback = async id => {
  if (!id) return
  rollbackLoading.value = true
  try {
    const payload = unwrap(await dbApi.getReviewTicketRollback(id))
    const records = normalizeList(payload)
    rollbackRecords.value = records
    const firstAvailable = records.find(record => record.available !== false && record.rollbackSql) || records[0] || null
    rollbackInfo.value = firstAvailable
    activeRollbackRecordId.value = firstAvailable?.id || ''
  } catch (error) {
    rollbackRecords.value = []
    rollbackInfo.value = null
    ElMessage.error(error.message || '获取回滚 SQL 失败')
  } finally {
    rollbackLoading.value = false
  }
}

const getBlobFilename = (response, fallback) => {
  const disposition = response?.headers?.['content-disposition'] || response?.headers?.['Content-Disposition'] || ''
  const match = disposition.match(/filename\*?=(?:UTF-8'')?["']?([^"';]+)["']?/i)
  return match ? decodeURIComponent(match[1]) : fallback
}

const downloadBlob = (blob, filename) => {
  const link = document.createElement('a')
  link.href = URL.createObjectURL(blob)
  link.download = filename
  link.click()
  URL.revokeObjectURL(link.href)
}

const downloadQueryResult = async () => {
  const id = detailTicket.value.id || detailTicket.value.ticketId
  if (!id) return
  try {
    const response = await dbApi.exportReviewTicketResult(id)
    downloadBlob(response.data, getBlobFilename(response, `${detailTicket.value.ticketNo || 'query-result'}.csv`))
  } catch (error) {
    ElMessage.error(error.message || '下载查询结果失败')
  }
}

const copyText = async text => {
  if (!text) return false
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
      return true
    }
  } catch (error) {
    // Fall back to textarea copy for non-HTTPS/intranet pages.
  }
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.left = '-9999px'
  document.body.appendChild(textarea)
  textarea.select()
  const copied = document.execCommand('copy')
  document.body.removeChild(textarea)
  return copied
}

const openRollbackTicketDialog = () => {
  Object.assign(rollbackTicketForm, {
    rollbackRecordId: getRollbackRecordId(),
    title: `故障回滚-${detailTicket.value.title || detailTicket.value.ticketNo || ''}`,
    approverId: detailTicket.value.approverId || '',
    executorId: detailTicket.value.executorId || '',
    remark: ''
  })
  rollbackTicketDialog.visible = true
}

const submitRollbackTicket = async () => {
  const id = detailTicket.value.id || detailTicket.value.ticketId
  if (!id) return
  if (!rollbackTicketForm.title || !rollbackTicketForm.approverId || !rollbackTicketForm.executorId) {
    ElMessage.warning('请填写标题、审批人和执行人')
    return
  }
  rollbackTicketDialog.submitting = true
  try {
    const payload = unwrap(await dbApi.createRollbackReviewTicket(id, {
      rollbackRecordId: rollbackTicketForm.rollbackRecordId || undefined,
      title: rollbackTicketForm.title,
      approverId: rollbackTicketForm.approverId,
      executorId: rollbackTicketForm.executorId,
      remark: rollbackTicketForm.remark
    }))
    rollbackTicketDialog.visible = false
    ElMessage.success('回滚工单已提交')
    loadTickets()
    const newId = payload.id || payload.ticketId
    if (newId) openDetail({ id: newId })
  } catch (error) {
    ElMessage.error(error.message || '提交回滚工单失败')
  } finally {
    rollbackTicketDialog.submitting = false
  }
}

const cleanupBackupTables = async () => {
  const id = detailTicket.value.id || detailTicket.value.ticketId
  if (!id) return
  try {
    await ElMessageBox.confirm(
      '只会删除当前工单登记的 _aiops_backup_ 备份表，删除后无法再通过这些备份表生成回滚 SQL。确认删除？',
      '删除备份表',
      {
        type: 'warning',
        confirmButtonText: '确认删除',
        cancelButtonText: '取消'
      }
    )
    backupCleanupLoading.value = true
    const payload = unwrap(await dbApi.cleanupReviewTicketBackup(id, { confirmed: true }))
    const count = payload.deletedCount ?? normalizeList(payload.deletedTables).length
    ElMessage.success(`已删除 ${count} 张备份表`)
    await loadRollback(id)
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    ElMessage.error(error.message || '删除备份表失败')
  } finally {
    backupCleanupLoading.value = false
  }
}

const copyRollbackSql = async () => {
  const text = activeRollbackRecord.value?.rollbackSql || rollbackInfo.value?.rollbackSql || ''
  if (!text) return
  if (await copyText(text)) {
    ElMessage.success('已复制回滚 SQL')
  } else {
    ElMessage.error('复制失败，请手动复制')
  }
}

const generateFlashbackSql = async () => {
  const id = detailTicket.value.id || detailTicket.value.ticketId
  const rollbackRecordId = getRollbackRecordId()
  if (!id || !rollbackRecordId) return
  try {
    await ElMessageBox.confirm(
      '后端将调用本机 /opt/db/my2sql 基于 binlog 位点生成反向 SQL，并写回当前回滚记录。确认生成？',
      '生成闪回 SQL',
      {
        type: 'warning',
        confirmButtonText: '确认生成',
        cancelButtonText: '取消'
      }
    )
    flashbackLoading.value = true
    await dbApi.generateReviewTicketFlashback(id, { rollbackRecordId, confirmed: true })
    ElMessage.success('闪回 SQL 已生成')
    await loadRollback(id)
  } catch (error) {
    if (error === 'cancel' || error === 'close') return
    ElMessage.error(error.message || '生成闪回 SQL 失败')
  } finally {
    flashbackLoading.value = false
  }
}

const copyGhOstScript = async () => {
  const text = detailTicket.value.ghOstScript || ''
  if (!text) return
  if (await copyText(text)) {
    ElMessage.success('已复制 gh-ost 脚本')
  } else {
    ElMessage.error('复制失败，请手动复制')
  }
}

const openExecLogDialog = row => {
  execLogDialog.content = getExecutionLog(row)
  execLogDialog.visible = true
}

onMounted(() => {
  currentUser.value = getCurrentUser()
  loadAssets()
  loadUsers()
  loadSqlWorkflows()
  loadTickets()
})
</script>

<style scoped>
.db-review-page {
  box-sizing: border-box;
  --ops-blue-500: #409eff;
  --ops-blue-600: #2563eb;
  --ops-green-500: #10b981;
  --ops-orange-500: #f59e0b;
  --ops-red-500: #ef4444;
  --ops-button-success: #67c23a;
  --ops-button-success-hover: #58b832;
  --ops-button-warning: #e6a23c;
  --ops-button-warning-hover: #d8922f;
  --ops-text-heading: #0f172a;
  --ops-text-primary: #172033;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-text-weak: #7b8aa0;
  --ops-bg-surface: #ffffff;
  --ops-bg-surface-soft: #fbfdff;
  --ops-bg-subtle: #f3f7fc;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  --ops-border-inner: #dce8f5;
  --ops-border-hover: #b9d4ff;
  --ops-shadow-card: 0 8px 24px rgba(80, 124, 180, 0.07);
  --aiops-bg-page: #f6f8fb;
  --aiops-text-primary: var(--ops-text-primary);
  --aiops-text-secondary: var(--ops-text-secondary);
  --aiops-text-muted: var(--ops-text-weak);
  --aiops-border-cool: var(--ops-border-soft);
  --aiops-border-mid: var(--ops-border);
  --aiops-brand: var(--ops-blue-600);
  --aiops-success: var(--ops-green-500);
  --aiops-warning: var(--ops-orange-500);
  --aiops-danger: var(--ops-red-500);
  height: calc(100vh - 124px);
  min-height: 0;
  overflow: auto;
  padding: 7px;
  background: transparent;
  color: var(--aiops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

:global(.home-container .el-main:has(.db-review-page)) {
  padding: 10px;
  overflow: hidden;
}

.review-shell {
  display: flex;
  box-sizing: border-box;
  flex-direction: column;
  height: 100%;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--aiops-border-mid);
  border-radius: 12px;
  background: var(--ops-bg-surface);
  box-shadow: var(--ops-shadow-card);
}

.review-hero {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-shrink: 0;
  padding: 8px 12px;
  border-bottom: 1px solid var(--aiops-border-cool);
  background: var(--ops-bg-surface);
}

.review-hero-copy {
  display: flex;
  min-width: 0;
  align-items: center;
}

.ops-page-header__icon {
  display: grid;
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  place-items: center;
}

.ops-page-header__icon img {
  width: 30px;
  height: 30px;
  object-fit: contain;
}

.ops-page-header__content {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
}

.ops-page-header__divider {
  width: 1px;
  height: 22px;
  flex: 0 0 1px;
  background: var(--ops-border-soft);
}

.ops-page-header__content h1 {
  margin: 0;
  color: var(--ops-text-heading);
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
  white-space: nowrap;
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

.pagination-row,
.check-summary,
.rollback-meta {
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.dialog-footer,
.detail-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.primary-action,
.secondary-action,
.warning-action,
.success-action,
.danger-action,
.confirm-button,
.cancel-button {
  height: 30px;
  min-height: 30px;
  padding: 0 10px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  transition:
    color 160ms ease,
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease,
    transform 160ms ease;
}

.primary-action {
  border-color: var(--ops-blue-500);
  background: var(--ops-blue-500);
  color: #fff;
}

.confirm-button {
  border-color: var(--ops-blue-600);
  background: var(--ops-blue-600);
  color: #fff;
}

.success-action {
  border-color: var(--ops-button-success);
  background: var(--ops-button-success);
  color: #fff;
}

.warning-action {
  border-color: var(--ops-button-warning);
  background: var(--ops-button-warning);
  color: #fff;
}

.danger-action {
  border-color: var(--ops-red-500);
  background: var(--ops-red-500);
  color: #fff;
}

.secondary-action,
.cancel-button {
  border: 1px solid var(--ops-border-inner);
  background: var(--ops-bg-surface);
  color: var(--ops-text-secondary);
}

.primary-action:not(.is-disabled):hover,
.confirm-button:not(.is-disabled):hover {
  border-color: #1d4ed8;
  background: #1d4ed8;
}

.success-action:not(.is-disabled):hover {
  border-color: var(--ops-button-success-hover);
  background: var(--ops-button-success-hover);
}

.warning-action:not(.is-disabled):hover {
  border-color: var(--ops-button-warning-hover);
  background: var(--ops-button-warning-hover);
}

.danger-action:not(.is-disabled):hover {
  border-color: #dc2626;
  background: #dc2626;
}

.secondary-action:not(.is-disabled):hover,
.cancel-button:not(.is-disabled):hover {
  border-color: var(--ops-border-hover);
  background: var(--ops-bg-subtle);
  color: var(--ops-blue-600);
}

.primary-action:not(.is-disabled):hover,
.secondary-action:not(.is-disabled):hover,
.warning-action:not(.is-disabled):hover,
.success-action:not(.is-disabled):hover,
.danger-action:not(.is-disabled):hover,
.confirm-button:not(.is-disabled):hover,
.cancel-button:not(.is-disabled):hover {
  transform: translateY(-1px);
}

.primary-action:active,
.secondary-action:active,
.warning-action:active,
.success-action:active,
.danger-action:active,
.confirm-button:active,
.cancel-button:active {
  transform: translateY(0);
}

.primary-action:focus-visible,
.secondary-action:focus-visible,
.warning-action:focus-visible,
.success-action:focus-visible,
.danger-action:focus-visible,
.confirm-button:focus-visible,
.cancel-button:focus-visible {
  outline: 2px solid rgba(64, 158, 255, 0.35);
  outline-offset: 2px;
}

.primary-action.is-disabled,
.secondary-action.is-disabled,
.warning-action.is-disabled,
.success-action.is-disabled,
.danger-action.is-disabled,
.confirm-button.is-disabled,
.cancel-button.is-disabled {
  opacity: 0.5;
  transform: none;
}

.confirm-button.is-disabled,
.confirm-button.is-disabled:hover,
.confirm-button.is-disabled:focus {
  border-color: var(--ops-blue-600);
  background: var(--ops-blue-600);
  color: #fff;
}

.compact {
  height: 28px;
  min-height: 28px;
  padding: 0 10px;
  border-radius: 8px;
  font-size: 12px;
}

.filter-panel {
  flex-shrink: 0;
  padding: 8px 10px;
  border-bottom: 1px solid var(--aiops-border-cool);
  background: var(--ops-bg-surface-soft);
}

.filter-form {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  row-gap: 8px;
}

.filter-form :deep(.el-form-item) {
  margin-right: 8px;
  margin-bottom: 0;
}

.filter-form :deep(.el-form-item__label) {
  padding-right: 6px;
  color: var(--ops-text-secondary);
  font-size: 12px;
  font-weight: 600;
}

.filter-form :deep(.el-input),
.filter-form :deep(.el-select) {
  width: 168px;
}

.filter-form :deep(.el-input__wrapper),
.filter-form :deep(.el-select__wrapper),
.ticket-form :deep(.el-input__wrapper),
.ticket-form :deep(.el-select__wrapper),
.ticket-form :deep(.el-textarea__inner) {
  min-height: 30px;
  border: 0;
  border-radius: 8px;
  background: var(--ops-bg-surface-soft);
  box-shadow: 0 0 0 1px var(--ops-border-inner) inset;
}

.filter-form :deep(.el-input__wrapper:hover),
.filter-form :deep(.el-select__wrapper:hover),
.ticket-form :deep(.el-input__wrapper:hover),
.ticket-form :deep(.el-select__wrapper:hover),
.ticket-form :deep(.el-textarea__inner:hover) {
  box-shadow: 0 0 0 1px var(--ops-border-hover) inset;
}

.filter-form :deep(.el-input__wrapper.is-focus),
.filter-form :deep(.el-select__wrapper.is-focused),
.ticket-form :deep(.el-input__wrapper.is-focus),
.ticket-form :deep(.el-select__wrapper.is-focused) {
  box-shadow: 0 0 0 1px var(--ops-blue-500) inset;
}

.table-panel {
  display: flex;
  flex: 1;
  min-width: 0;
  min-height: 0;
  flex-direction: column;
  padding: 8px 10px 10px;
}

.status-tabs {
  display: inline-flex;
  width: fit-content;
  margin-bottom: 8px;
  padding: 2px;
  border: 1px solid var(--ops-border-soft);
  border-radius: 8px;
  background: var(--ops-bg-subtle);
}

.status-tabs button {
  height: 28px;
  padding: 0 12px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  font-weight: 700;
  cursor: pointer;
}

.status-tabs button.active {
  background: var(--ops-bg-surface);
  color: var(--aiops-brand);
  box-shadow: 0 2px 7px rgba(64, 158, 255, 0.1);
}

.status-tabs button:hover {
  color: var(--ops-blue-600);
}

.status-tabs button:focus-visible {
  outline: 2px solid rgba(64, 158, 255, 0.35);
  outline-offset: -2px;
}

.table-shell {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--ops-border-inner);
  border-radius: 8px;
  background: var(--ops-bg-surface);
}

.review-table {
  width: 100%;
  background: transparent;
}

.review-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.review-table :deep(.el-table__header th) {
  height: 36px;
  padding: 4px 0;
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-subtle) !important;
}

.review-table :deep(.cell) {
  font-size: 12px;
  line-height: 20px;
}

.review-table :deep(.el-table__body td) {
  height: 36px;
  padding: 4px 0;
  color: #44546a;
  font-size: 12px;
}

.review-table :deep(.el-table__body tr:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

.ticket-no-link {
  max-width: 100%;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--aiops-brand);
  font: inherit;
  font-weight: 700;
  cursor: pointer;
}

.ticket-no-link:hover {
  text-decoration: underline;
}

.db-type-cell {
  display: inline-flex;
  max-width: 100%;
  align-items: center;
  gap: 7px;
  color: #324155;
  font-weight: 700;
  vertical-align: middle;
}

.db-type-icon-box {
  display: inline-flex;
  width: 24px;
  height: 24px;
  flex: 0 0 24px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--ops-border-inner);
  border-radius: 7px;
  background: var(--ops-bg-subtle);
}

.db-type-icon {
  width: 18px;
  height: 18px;
  flex: 0 0 18px;
  object-fit: contain;
}

.db-type-name {
  font-size: 12px;
  font-weight: 800;
  line-height: 24px;
}

.asset-option {
  display: inline-flex;
  max-width: 100%;
  align-items: center;
  gap: 8px;
  color: #324155;
  font-weight: 700;
}

.asset-option-icon {
  width: 18px;
  height: 18px;
  flex: 0 0 18px;
  object-fit: contain;
}

.db-type-mysql {
  color: #a15d00;
}

.db-type-mysql .db-type-icon-box {
  border-color: rgba(245, 158, 11, 0.42);
  background: #fff3cf;
}

.db-type-postgresql {
  color: #1f5fbf;
}

.db-type-postgresql .db-type-icon-box {
  border-color: rgba(37, 99, 235, 0.36);
  background: #e8f0ff;
}

.db-type-redis {
  color: #c92f2f;
}

.db-type-redis .db-type-icon-box {
  border-color: rgba(220, 38, 38, 0.32);
  background: #ffe9e9;
}

.db-type-mongodb {
  color: #2f8c57;
}

.db-type-mongodb .db-type-icon-box {
  border-color: rgba(47, 140, 87, 0.25);
  background: #edf9f0;
}

.db-type-elasticsearch {
  color: #9b6b16;
}

.db-type-elasticsearch .db-type-icon-box {
  border-color: rgba(230, 169, 54, 0.3);
  background: #fff7e6;
}

.db-type-oracle,
.db-type-sqlserver {
  color: #b42318;
}

.db-type-oracle .db-type-icon-box,
.db-type-sqlserver .db-type-icon-box {
  border-color: rgba(180, 35, 24, 0.28);
  background: #fff0ed;
}

.operation-links {
  display: inline-flex;
  gap: 8px;
}

.operation-links :deep(.el-button) {
  height: auto;
  padding: 0;
  font-size: 12px;
  font-weight: 700;
}

.pagination-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;
  padding-top: 8px;
}

.pagination-row :deep(.el-pager li),
.pagination-row :deep(.btn-prev),
.pagination-row :deep(.btn-next),
.pagination-row :deep(.el-select__wrapper) {
  border-radius: 7px;
}

.risk-dot {
  display: inline-block;
  width: 7px;
  height: 7px;
  margin-right: 6px;
  border-radius: 999px;
  background: var(--aiops-success);
}

.risk-medium {
  background: var(--aiops-warning);
}

.risk-high,
.risk-critical {
  background: var(--aiops-danger);
}

.create-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 330px;
  gap: 14px;
  max-height: calc(100vh - 170px);
  overflow: hidden;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0 14px;
}

.form-span-1 {
  grid-column: span 1;
}

.form-span-2 {
  grid-column: span 2;
}

.form-spacer {
  min-height: 1px;
}

.ticket-form {
  min-height: 0;
  overflow: auto;
  padding-right: 4px;
}

.ticket-form :deep(.el-form-item) {
  margin-bottom: 12px;
}

.ticket-form :deep(.el-form-item__label) {
  margin-bottom: 6px;
  color: var(--ops-text-secondary);
  font-size: 12px;
  font-weight: 700;
  line-height: 1.4;
}

.compact-label {
  font-size: 11px;
  line-height: 13px;
}

.rollback-form-item {
  margin-bottom: 12px !important;
}

.ticket-sql-editor {
  min-height: 124px !important;
}

.sql-resize-handle {
  display: flex;
  height: 10px;
  align-items: center;
  justify-content: center;
  margin-top: -1px;
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-top: none;
  border-radius: 0 0 8px 8px;
  background: rgba(248, 250, 255, 0.92);
  cursor: ns-resize;
}

.sql-resize-handle span {
  width: 42px;
  height: 3px;
  border-radius: 999px;
  background: #c8d3e4;
}

.sql-resize-handle:hover span {
  background: var(--aiops-brand);
}

.sql-engine-tip {
  margin-top: 6px;
  color: var(--aiops-text-muted);
  font-size: 12px;
  line-height: 1.5;
}

.rollback-sql-editor {
  min-height: 88px !important;
}

.sql-type-hint {
  grid-column: 1 / -1;
  display: grid;
  grid-template-columns: 1.25fr 1.15fr 0.7fr;
  align-items: center;
  gap: 6px;
  min-height: 28px;
  margin: -2px 0 6px;
  padding: 3px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 9px;
  background: rgba(248, 250, 255, 0.82);
}

.sql-type-hint span {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
  margin: 0;
  padding: 1px 6px;
  border: 1px solid transparent;
  border-radius: 7px;
  background: #fff;
  color: var(--aiops-text-secondary);
  font-size: 10px;
  line-height: 17px;
  white-space: nowrap;
  transition:
    color 160ms ease,
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease;
}

.sql-type-hint strong {
  padding: 1px 6px;
  border-radius: 999px;
  background: rgba(31, 111, 255, 0.08);
  color: var(--aiops-text-primary);
  font-size: 10px;
}

.sql-type-hint span.active {
  border-color: rgba(243, 178, 75, 0.68);
  background: #fff7df;
  color: #9a640f;
  box-shadow: 0 4px 12px rgba(243, 178, 75, 0.16);
}

.sql-type-hint span.active strong {
  background: rgba(243, 178, 75, 0.2);
  color: #8a5a10;
}

.ticket-form :deep(.el-input-number) {
  width: 100%;
}

.backup-switch-field {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 28px;
}

.backup-switch-field span {
  color: #9a640f;
  font-size: 11px;
  line-height: 16px;
}

.ddl-switch-row {
  display: flex;
  align-items: center;
  gap: 14px;
  min-height: 28px;
  justify-content: flex-start;
  margin-top: -6px;
}

.ddl-switch-row label {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 4px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  white-space: nowrap;
}

.ddl-switch-row :deep(.el-switch) {
  margin-top: -1px;
}

.check-panel,
.ai-card {
  border: 1px solid var(--ops-border-inner);
  border-radius: 10px;
  background: var(--ops-bg-surface-soft);
}

.check-panel {
  display: flex;
  min-height: 0;
  flex-direction: column;
  padding: 8px;
}

.check-head,
.check-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.check-summary {
  margin-top: 8px;
  padding: 6px 8px;
  border: 1px solid var(--ops-border-soft);
  border-radius: 8px;
  background: var(--ops-bg-surface);
}

.check-summary > div {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.check-summary > strong {
  color: var(--aiops-text-primary);
  font-size: 12px;
}

.check-metrics {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 5px;
  margin-top: 6px;
}

.check-metrics article {
  min-width: 0;
  padding: 5px 7px;
  border: 1px solid var(--ops-border-soft);
  border-radius: 8px;
  background: var(--ops-bg-surface);
}

.check-metrics span,
.check-metrics strong {
  display: block;
}

.check-metrics span {
  color: var(--aiops-text-muted);
  font-size: 10px;
}

.check-metrics strong {
  overflow: hidden;
  margin-top: 2px;
  color: var(--aiops-text-primary);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.check-rule-overview {
  display: flex;
  gap: 5px;
  margin-top: 6px;
}

.check-rule-overview span {
  flex: 1;
  height: 22px;
  border-radius: 8px;
  background: #fff;
  font-size: 11px;
  font-weight: 700;
  line-height: 22px;
  text-align: center;
}

.check-list {
  display: flex;
  flex-direction: column;
  gap: 5px;
  margin-top: 6px;
  overflow: auto;
}

.check-item {
  padding: 6px 8px;
  border: 1px solid var(--ops-border-soft);
  border-left: 3px solid #8d97a8;
  border-radius: 8px;
  background: var(--ops-bg-surface);
}

.check-item strong,
.check-item span {
  display: block;
  font-size: 12px;
}

.check-item strong {
  color: var(--aiops-text-primary);
}

.check-item span {
  margin-top: 3px;
  color: var(--aiops-text-secondary);
}

.level-error {
  border-left-color: var(--aiops-danger);
  color: var(--aiops-danger);
}

.level-warn {
  border-left-color: var(--aiops-warning);
  color: #b7791f;
}

.level-info {
  border-left-color: var(--aiops-brand);
  color: var(--aiops-brand);
}

.detail-body {
  min-height: 100%;
}

.detail-overview-card {
  overflow: hidden;
  border: 1px solid var(--ops-border-inner);
  border-radius: 12px;
  background: var(--ops-bg-surface);
}

.detail-overview-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-surface-soft);
}

.detail-overview-head p,
.detail-overview-head h2 {
  margin: 0;
}

.detail-overview-head p {
  color: var(--aiops-brand);
  font-size: 12px;
  font-weight: 800;
}

.detail-overview-head h2 {
  margin-top: 3px;
  font-size: 17px;
}

.detail-overview-head span {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.current-action-tip {
  display: inline-flex;
  margin-top: 5px;
  padding: 3px 8px;
  border-radius: 999px;
  background: rgba(31, 111, 255, 0.08);
  color: var(--aiops-brand);
  font-size: 12px;
  font-style: normal;
  font-weight: 700;
}

.workflow-rail {
  position: relative;
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  align-items: start;
  gap: 18px;
  padding: 10px 14px;
  background: var(--ops-bg-subtle);
}

.workflow-track {
  position: absolute;
  top: 21px;
  right: calc(16.666% + 26px);
  left: calc(16.666% + 26px);
  height: 6px;
  overflow: hidden;
  border-radius: 999px;
  background: #dfe7f2;
}

.workflow-fill {
  width: 0;
  height: 100%;
  border-radius: inherit;
  background: var(--aiops-success);
  transition: width 0.2s ease;
}

.workflow-fill.progress-one {
  width: 0;
}

.workflow-fill.progress-two {
  width: 50%;
}

.workflow-fill.progress-full {
  width: 100%;
}

.workflow-step {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 5px;
  min-width: 0;
  text-align: center;
}

.workflow-step i {
  display: inline-flex;
  width: 30px;
  height: 30px;
  flex: 0 0 30px;
  align-items: center;
  justify-content: center;
  border: 3px solid #f8faff;
  border-radius: 999px;
  background: #e8eef8;
  color: var(--aiops-text-muted);
  font-style: normal;
  font-size: 12px;
  font-weight: 800;
  box-shadow: 0 0 0 2px #dfe7f2;
}

.workflow-step.done i {
  background: rgba(63, 185, 127, 0.16);
  color: var(--aiops-success);
  box-shadow: 0 0 0 2px rgba(63, 185, 127, 0.45);
}

.workflow-step.active i {
  background: rgba(31, 111, 255, 0.14);
  color: var(--aiops-brand);
  box-shadow: 0 0 0 2px rgba(31, 111, 255, 0.42);
}

.workflow-step.failed i {
  background: rgba(239, 107, 115, 0.16);
  color: var(--aiops-danger);
  box-shadow: 0 0 0 2px rgba(239, 107, 115, 0.42);
}

.workflow-step div {
  min-width: 0;
}

.workflow-step span,
.workflow-step strong,
.workflow-step em {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.workflow-step span {
  color: var(--aiops-text-muted);
  font-size: 11px;
}

.workflow-step strong {
  margin-top: 1px;
  color: var(--aiops-text-primary);
  font-size: 12px;
}

.workflow-step em {
  margin-top: 1px;
  color: var(--aiops-text-muted);
  font-size: 11px;
  font-style: normal;
}

.info-table {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  overflow: hidden;
  border-top: 1px solid rgba(223, 231, 242, 0.95);
  background: var(--ops-bg-surface);
}

.info-cell {
  min-width: 0;
  min-height: 48px;
  padding: 7px 12px;
  border-right: 1px solid rgba(223, 231, 242, 0.9);
  border-bottom: 1px solid rgba(223, 231, 242, 0.9);
  background: var(--ops-bg-surface);
}

.info-cell:nth-child(3n) {
  border-right: none;
}

.info-cell:nth-last-child(-n + 3) {
  border-bottom: none;
}

.info-cell span,
.info-cell strong {
  display: block;
}

.info-cell span {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.info-cell strong {
  overflow: hidden;
  margin-top: 4px;
  color: var(--aiops-text-primary);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.overview-sql-block {
  border-top: 1px solid rgba(223, 231, 242, 0.95);
  background: var(--ops-bg-surface-soft);
}

.overview-sql-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 7px 12px 0;
}

.overview-sql-head strong {
  color: var(--aiops-text-primary);
  font-size: 12px;
}

.tag-row {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.overview-sql-block pre {
  overflow: auto;
  max-height: 132px;
  margin: 0;
  padding: 8px 12px 10px;
  color: #172133;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
}

.review-result-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.review-summary-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid var(--ops-border-inner);
  border-radius: 10px;
  background: var(--ops-bg-surface);
}

.review-summary-grid article {
  min-width: 0;
  padding: 9px 12px;
  border-right: 1px solid rgba(223, 231, 242, 0.86);
  border-bottom: 1px solid rgba(223, 231, 242, 0.86);
}

.review-summary-grid article:nth-child(3n) {
  border-right: none;
}

.review-summary-grid article:nth-last-child(-n + 3) {
  border-bottom: none;
}

.review-summary-grid span,
.review-summary-grid strong {
  display: block;
}

.review-summary-grid span {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.review-summary-grid strong {
  margin-top: 4px;
  color: var(--aiops-text-primary);
  font-size: 13px;
}

.detail-sql {
  overflow: auto;
  margin-bottom: 10px;
  border: 1px solid var(--ops-border-inner);
  border-radius: 10px;
  background: var(--ops-bg-surface-soft);
}

.detail-sql-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px 0;
}

.detail-sql-head strong {
  color: var(--aiops-text-primary);
  font-size: 12px;
}

.detail-sql pre {
  margin: 0;
  padding: 12px;
  color: #172133;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
  line-height: 1.7;
  white-space: pre-wrap;
}

.ghost-script-block {
  background: #fffaf0;
}

.ghost-script-block :deep(.el-alert) {
  margin: 8px 12px 0;
}

.ghost-script-block :deep(.el-alert__title) {
  line-height: 18px;
}

.ghost-script-block :deep(.el-alert__title a) {
  color: var(--aiops-brand);
  font-weight: 800;
  text-decoration: none;
}

.exec-log-dialog pre {
  max-height: 520px;
  white-space: pre-wrap;
}

.compact-table {
  border: 1px solid var(--ops-border-inner);
  border-radius: 8px;
}

.query-result-panel {
  display: flex;
  min-height: 360px;
  flex-direction: column;
  gap: 8px;
}

.query-result-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.query-result-tip {
  width: auto;
  flex: 1;
  min-width: 260px;
  padding: 4px 8px;
  border-radius: 9px;
}

.query-result-tip :deep(.el-alert__title) {
  font-size: 12px;
  line-height: 18px;
}

.query-result-tip :deep(.el-alert__content) {
  padding: 0 4px;
}

.query-result-title {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 8px;
  min-width: 0;
  white-space: nowrap;
}

.query-result-title strong {
  color: var(--aiops-text-primary);
  font-size: 13px;
}

.query-result-title span {
  display: inline-flex;
  padding: 2px 7px;
  border-radius: 999px;
  background: rgba(31, 111, 255, 0.08);
  color: var(--aiops-text-muted);
  font-size: 12px;
  line-height: 18px;
}

.query-result-table {
  flex: 1;
  min-height: 320px;
}

.query-result-sql {
  border: 1px solid var(--ops-border-inner);
  border-radius: 8px;
  background: var(--ops-bg-surface-soft);
}

.query-result-sql strong {
  display: block;
  padding: 7px 10px 0;
  color: var(--aiops-text-primary);
  font-size: 12px;
}

.query-result-sql pre {
  overflow: auto;
  max-height: 90px;
  margin: 0;
  padding: 6px 10px 8px;
  color: #172133;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
  line-height: 1.55;
  white-space: pre-wrap;
}

.rollback-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.rollback-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.rollback-record-select {
  width: 360px;
  max-width: 100%;
}

.rollback-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
}

.toolbox-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.toolbox-target {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 12px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.toolbox-card {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--ops-border-inner);
  border-radius: 10px;
  background: var(--ops-bg-surface-soft);
}

.toolbox-card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.toolbox-card-head div {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 4px;
}

.toolbox-card-head strong {
  color: var(--aiops-text-primary);
  font-size: 15px;
}

.toolbox-card-head span,
.toolbox-result {
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.toolbox-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.toolbox-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.toolbox-result {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px 10px;
  border-radius: 10px;
  border: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-surface);
}

.toolbox-result a {
  color: var(--aiops-brand);
  font-weight: 700;
  text-decoration: none;
}

.toolbox-result a:hover {
  text-decoration: underline;
}

.toolbox-install-collapse {
  overflow: hidden;
  border: 1px solid var(--ops-border-soft);
  border-radius: 10px;
  background: var(--ops-bg-surface);
}

.toolbox-install-collapse :deep(.el-collapse-item__header) {
  height: 32px;
  padding: 0 10px;
  border-bottom: none;
  background: transparent;
  color: var(--aiops-text-primary);
  font-size: 12px;
  font-weight: 700;
}

.toolbox-install-collapse :deep(.el-collapse-item__wrap) {
  border-bottom: none;
  background: transparent;
}

.toolbox-install-command {
  overflow: auto;
  max-height: 140px;
  margin: 0;
  padding: 6px 10px 10px;
  color: #172133;
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, monospace;
  font-size: 12px;
  line-height: 1.55;
  white-space: pre-wrap;
}

:global(.database-review-dialog-overlay),
:global(.review-drawer.el-drawer) {
  --ops-blue-500: #409eff;
  --ops-blue-600: #2563eb;
  --ops-green-500: #10b981;
  --ops-orange-500: #f59e0b;
  --ops-red-500: #ef4444;
  --ops-button-success: #67c23a;
  --ops-button-success-hover: #58b832;
  --ops-button-warning: #e6a23c;
  --ops-button-warning-hover: #d8922f;
  --ops-text-heading: #0f172a;
  --ops-text-primary: #172033;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-text-weak: #7b8aa0;
  --ops-bg-surface: #ffffff;
  --ops-bg-surface-soft: #fbfdff;
  --ops-bg-subtle: #f3f7fc;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  --ops-border-inner: #dce8f5;
  --ops-border-hover: #b9d4ff;
  --aiops-text-primary: var(--ops-text-primary);
  --aiops-text-secondary: var(--ops-text-secondary);
  --aiops-text-muted: var(--ops-text-weak);
  --aiops-border-cool: var(--ops-border-soft);
  --aiops-border-mid: var(--ops-border);
  --aiops-brand: var(--ops-blue-600);
  --aiops-success: var(--ops-green-500);
  --aiops-warning: var(--ops-orange-500);
  --aiops-danger: var(--ops-red-500);
}

:global(.database-review-dialog-overlay .el-dialog) {
  width: min(var(--el-dialog-width), calc(100vw - 24px));
  max-height: calc(100vh - 24px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: 15px !important;
  background: var(--ops-bg-surface);
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.database-review-dialog-overlay .el-dialog__header) {
  margin-right: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
}

:global(.database-review-dialog-overlay .el-dialog__title) {
  color: var(--ops-text-heading);
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.database-review-dialog-overlay .el-dialog__headerbtn) {
  top: 8px;
  right: 10px;
  width: 34px;
  height: 34px;
  border-radius: 8px;
}

:global(.database-review-dialog-overlay .el-dialog__headerbtn:hover) {
  background: var(--ops-bg-subtle);
}

:global(.database-review-dialog-overlay .el-dialog__body) {
  max-height: calc(100vh - 154px);
  overflow: auto;
  padding: 16px 18px;
  color: var(--ops-text-primary);
}

:global(.database-review-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
}

:global(.database-review-dialog-overlay .monaco-editor-container) {
  min-height: 0;
}

:global(.database-review-dialog-overlay .el-tag) {
  height: 22px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
}

:global(.review-drawer.el-drawer) {
  max-width: calc(100vw - 12px);
  border-left: 1px solid #d7e6f6;
  background: #fff;
  box-shadow: -18px 0 52px rgba(49, 92, 142, 0.16);
}

:global(.review-drawer .el-drawer__header) {
  margin-bottom: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  color: var(--ops-text-heading);
  font-size: 17px;
  font-weight: 700;
}

:global(.review-drawer .el-drawer__body) {
  overflow: auto;
  padding: 12px 14px 16px;
}

:global(.review-drawer .el-drawer__close-btn) {
  width: 34px;
  height: 34px;
  border-radius: 8px;
}

:global(.review-drawer .el-drawer__close-btn:hover) {
  background: #f3f7fc;
}

.generic-workflow-panel {
  margin-bottom: 14px;
  padding: 12px 14px 0;
  border: 1px solid var(--ops-border, #dfe6ef);
  border-radius: 6px;
  background: #fafcff;
}

.generic-workflow-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.generic-workflow-head div,
.generic-workflow-assignee {
  display: flex;
  align-items: center;
  gap: 10px;
}

.generic-workflow-head span,
.generic-workflow-assignee {
  color: var(--ops-text-secondary);
  font-size: 12px;
}

@media (max-width: 1100px) {
  .create-layout {
    grid-template-columns: 1fr;
    overflow: auto;
  }

  .toolbox-grid {
    grid-template-columns: 1fr;
  }

  .info-table {
    grid-template-columns: 1fr;
  }

  .workflow-rail {
    grid-template-columns: 1fr;
  }

  .workflow-track {
    top: 25px;
    bottom: 25px;
    right: auto;
    left: 27px;
    width: 2px;
    height: auto;
  }

  .workflow-fill.progress-two {
    width: 100%;
    height: 50%;
  }

  .workflow-fill.progress-full {
    width: 100%;
    height: 100%;
  }

  .workflow-step {
    flex-direction: row;
    text-align: left;
  }

  .info-cell,
  .info-cell:nth-child(3n),
  .info-cell:nth-last-child(-n + 3) {
    border-right: none;
    border-bottom: 1px solid rgba(223, 231, 242, 0.9);
  }

  .info-cell:last-child {
    border-bottom: none;
  }

  .review-summary-grid {
    grid-template-columns: 1fr;
  }

  .review-summary-grid article,
  .review-summary-grid article:nth-child(3n),
  .review-summary-grid article:nth-last-child(-n + 3) {
    border-right: none;
    border-bottom: 1px solid rgba(223, 231, 242, 0.86);
  }

  .review-summary-grid article:last-child {
    border-bottom: none;
  }

  .form-grid {
    grid-template-columns: 1fr;
  }

  .form-span-1,
  .form-span-2 {
    grid-column: span 1;
  }
}

@media (max-width: 768px) {
  .review-shell {
    height: auto;
    min-height: 100%;
    overflow: visible;
  }

  .table-panel {
    min-height: 520px;
  }

  .ops-page-header__content {
    align-items: flex-start;
    flex-direction: column;
    gap: 2px;
  }

  .ops-page-header__divider {
    display: none;
  }

  .filter-form :deep(.el-form-item:last-child) {
    width: 100%;
  }

  .pagination-row,
  .detail-overview-head,
  .query-result-head {
    align-items: stretch;
    flex-direction: column;
  }

  .query-result-tip {
    width: 100%;
    min-width: 0;
  }
}

@media (max-width: 620px) {
  .db-review-page {
    padding: 6px;
  }

  .review-hero {
    padding: 8px 10px;
  }

  .ops-page-header__icon {
    width: 36px;
    height: 36px;
    flex-basis: 36px;
  }

  .ops-page-header__icon img {
    width: 27px;
    height: 27px;
  }

  .filter-form,
  .filter-form :deep(.el-form-item),
  .filter-form :deep(.el-form-item__content),
  .filter-form :deep(.el-input),
  .filter-form :deep(.el-select) {
    width: 100%;
  }

  .filter-form :deep(.el-form-item) {
    display: grid;
    grid-template-columns: 58px minmax(0, 1fr);
    margin-right: 0;
  }

  .filter-form :deep(.el-form-item:last-child .el-form-item__content) {
    grid-column: 1 / -1;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 6px;
  }

  .filter-form :deep(.el-form-item:last-child .el-button) {
    width: 100%;
    margin-left: 0;
  }

  .status-tabs {
    width: 100%;
    overflow-x: auto;
  }

  .status-tabs button {
    flex: 1 0 auto;
  }

  .toolbox-actions {
    display: grid;
    grid-template-columns: 1fr;
  }

  :global(.database-review-dialog-overlay .el-dialog) {
    margin-top: 12px;
  }

  :global(.database-review-dialog-overlay .el-dialog__body) {
    max-height: calc(100vh - 146px);
    padding: 14px;
  }
}
</style>
