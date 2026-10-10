<template>
  <div class="log-collect-page">
    <section class="page-card">
      <header class="page-header">
        <div class="page-header-copy">
          <span class="ops-page-header__icon" aria-hidden="true">
            <img :src="logSvg" alt="">
          </span>
          <div class="ops-page-header__content">
            <h1>日志采集</h1>
            <span class="ops-page-header__divider" aria-hidden="true"></span>
            <p class="page-description">集中配置采集任务，并管理 Agent、Vector 与配置下发状态。</p>
          </div>
        </div>
        <div class="summary-group">
          <div class="summary-chip">
            <span>采集任务</span>
            <strong>{{ taskPagination.total }}</strong>
          </div>
          <div class="summary-chip">
            <span>采集节点</span>
            <strong>{{ taskAgentStats.total }}</strong>
          </div>
          <div class="summary-chip">
            <span>命中 Agent</span>
            <strong>{{ taskAgentStats.total }}</strong>
          </div>
          <div class="summary-chip">
            <span>已安装 Vector</span>
            <strong>{{ taskAgentStats.installed }}</strong>
          </div>
          <div class="summary-chip success">
            <span>运行中</span>
            <strong>{{ taskAgentStats.running }}</strong>
          </div>
          <div class="summary-chip success">
            <span>配置已生效</span>
            <strong>{{ taskAgentStats.configApplied }}</strong>
          </div>
          <div class="summary-chip warning">
            <span>配置未生效</span>
            <strong>{{ taskAgentStats.configPending }}</strong>
          </div>
          <div class="summary-chip info">
            <span>任务禁用中</span>
            <strong>{{ taskAgentStats.configDisabled }}</strong>
          </div>
          <div class="summary-chip danger">
            <span>异常中</span>
            <strong>{{ taskAgentStats.error }}</strong>
          </div>
        </div>
      </header>

      <section class="collect-workbench">
        <aside class="task-side-panel">
          <div class="side-head">
            <div>
              <h3>采集配置</h3>
              <span>{{ taskPagination.total }} 个任务</span>
            </div>
            <div class="side-head-actions">
              <el-button type="success" size="small" :icon="Plus" aria-label="新增采集任务" @click="openTaskDialog()" />
              <el-button type="warning" size="small" :icon="Refresh" aria-label="刷新采集配置" :loading="taskLoading" @click="loadConfigTree()" />
            </div>
          </div>

          <div class="tree-search">
            <el-input v-model.trim="treeKeyword" placeholder="搜索项目 / 日志库 / 任务名 / 路径" clearable>
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>
          </div>

          <div v-loading="taskLoading" class="config-tree">
            <template v-for="projectNode in filteredConfigTree" :key="projectNode.key">
              <button type="button" class="tree-node project-node" :class="{ active: isActiveNode(projectNode.key) }" @click="toggleProjectNode(projectNode)">
                <span class="node-main">
                  <el-icon class="fold-icon" :class="{ expanded: isProjectExpanded(projectNode.key) }"><ArrowRight /></el-icon>
                  <img
                    class="node-icon project-icon"
                    :src="isProjectExpanded(projectNode.key) ? openFolderIcon : closedFolderIcon"
                    alt=""
                    aria-hidden="true"
                  >
                  <span class="node-title">{{ projectNode.project.name || '-' }}</span>
                </span>
                <span class="node-count">{{ projectNode.tasks.length }}</span>
              </button>

              <div v-show="isProjectExpanded(projectNode.key)" class="tree-children store-children">
                <template v-for="storeNode in projectNode.stores" :key="storeNode.key">
                  <div class="store-node-row">
                    <button type="button" class="tree-node store-node" :class="{ active: isActiveNode(storeNode.key) }" @click="toggleStoreNode(storeNode)">
                      <span class="node-main">
                        <el-icon class="fold-icon store-fold-icon" :class="{ expanded: isStoreExpanded(storeNode.key) }"><ArrowRight /></el-icon>
                        <img class="node-icon store-icon" :src="logStoreIcon" alt="" aria-hidden="true">
                        <span class="node-title">{{ storeNode.store.name || storeNode.store.code || '-' }}</span>
                      </span>
                      <span class="node-count">{{ storeNode.tasks.length }}</span>
                    </button>
                    <el-button
                      v-if="canCreateTaskInStore(storeNode)"
                      class="store-node-add"
                      type="success"
                      size="small"
                      :icon="Plus"
                      aria-label="新增采集任务"
                      title="新增采集任务"
                      @click.stop="openStoreTaskDialog(storeNode)"
                    />
                  </div>

                  <div v-show="isStoreExpanded(storeNode.key)" class="tree-children task-children">
                    <button
                      v-for="task in storeNode.tasks"
                      :key="configTaskIdOf(task)"
                      type="button"
                      class="tree-node task-node"
                      :class="{ active: isActiveTask(task) }"
                      @click="selectTask(task, projectNode, storeNode)"
                    >
                      <span class="task-node-main">
                        <span class="task-node-line">
                          <img class="node-dot" :src="collectorIcon" alt="" aria-hidden="true">
                          <span class="node-title">{{ task.name || '-' }}</span>
                          <span class="task-node-actions">
                            <el-button
                              type="primary"
                              size="small"
                              :icon="EditPen"
                              aria-label="编辑任务"
                              title="编辑任务"
                              @click.stop="openTaskDialog(task)"
                            />
                            <el-button
                              type="danger"
                              size="small"
                              :icon="Delete"
                              aria-label="删除任务"
                              title="删除任务"
                              @click.stop="removeTask(task)"
                            />
                          </span>
                        </span>
                      </span>
                    </button>
                  </div>
                </template>
              </div>
            </template>
            <el-empty v-if="!filteredConfigTree.length && !taskLoading" description="暂无采集配置" :image-size="90" />
          </div>
        </aside>

        <main class="task-agent-panel">
          <div v-if="selectedNode" class="task-control-card">
            <div class="task-detail-card">
              <div class="task-detail-main">
                <h3>{{ currentScopeTitle }}</h3>
                <div class="scope-meta-grid">
                  <span v-for="item in scopeMetaItems" :key="item.label">
                    <b>{{ item.label }}：</b>
                    <em :class="{ 'mono-text': item.mono }">{{ item.value }}</em>
                  </span>
                </div>
              </div>
              <div v-if="selectedNode" class="task-actions">
                <el-button
                  v-if="isProjectScopeSelected || isK8sTask(currentTask)"
                  class="aggregator-config-btn"
                  type="primary"
                  plain
                  size="small"
                  :icon="View"
                  @click="openAggregatorConfigDialog"
                >
                  查看 Aggregator 配置
                </el-button>
                <el-tag v-else-if="!currentTask" type="primary" effect="plain">{{ selectedScopeTypeText }}</el-tag>
                <el-tag v-else :type="statusTagType(currentTask.status)" effect="plain">{{ statusText(currentTask.status) }}</el-tag>
                <el-button class="vector-manage-btn" size="small" @click="openPackageDialog">Vector 管理</el-button>
                <el-button class="agent-deploy-btn" size="small" @click="goCmdbAgent">部署Agent</el-button>
              </div>
            </div>

            <div class="control-divider"></div>

            <div v-if="!isK8sCollectorContentTab" class="agent-toolbar-card">
              <el-form :model="agentQuery" inline class="toolbar-form" @submit.prevent>
                <el-form-item label="主机/IP">
                  <el-input v-model.trim="agentQuery.keyword" placeholder="主机名或 IP" clearable @keyup.enter="handleAgentSearch" />
                </el-form-item>
                <el-form-item label="Agent 状态">
                  <el-select v-model="agentQuery.onlineStatus" placeholder="全部" clearable @change="loadSelectedNodeAgents">
                    <el-option label="在线" :value="1" />
                    <el-option label="离线" :value="2" />
                  </el-select>
                </el-form-item>
                <el-form-item label="Vector 状态">
                  <el-select v-model="agentQuery.vectorStatus" placeholder="全部" clearable @change="loadSelectedNodeAgents">
                    <el-option label="未安装" value="not_installed" />
                    <el-option label="已安装" value="installed" />
                    <el-option label="运行中" value="running" />
                    <el-option label="异常中" value="error" />
                  </el-select>
                </el-form-item>
                <el-form-item label="配置状态">
                  <el-select v-model="agentQuery.configStatus" placeholder="全部" clearable @change="loadSelectedNodeAgents">
                    <el-option label="待生效" value="pending" />
                    <el-option label="已生效" value="applied" />
                    <el-option label="任务禁用中" value="disabled" />
                    <el-option label="失败" value="failed" />
                  </el-select>
                </el-form-item>
                <el-form-item class="package-form-item">
                  <div class="package-status" :class="{ warning: packageInfo && !packageReady }">
                    <span class="package-status-main">Vector 包：<strong>{{ packageStatusText }}</strong></span>
                    <span v-if="packageInfo?.version" class="package-version">版本 {{ packageInfo.version }}</span>
                  </div>
                </el-form-item>
                <el-form-item>
                  <div class="toolbar-actions">
                    <el-button type="primary" size="small" :icon="Search" :loading="agentLoading" @click="handleAgentSearch">搜索</el-button>
                    <el-button type="warning" size="small" :icon="Refresh" @click="resetAgentSearch">重置</el-button>
                  </div>
                </el-form-item>
              </el-form>
            </div>
          </div>

          <el-empty v-else description="请选择或新增采集配置" :image-size="110" />

          <div v-if="selectedNode" class="collect-tabs">
            <el-tabs v-model="agentContentTab" @tab-change="handleAgentTabChange">
              <el-tab-pane :label="taskAgentTabLabel" name="task" />
              <el-tab-pane label="主机Vector列表" name="overview" />
              <el-tab-pane label="K8S Collector" name="k8s" />
              <el-tab-pane label="集群级 Vector" name="k8sCluster" />
            </el-tabs>

            <div class="content-panel">
              <div v-if="!isK8sCollectorContentTab" class="table-shell">
                <el-table v-loading="agentLoading" :data="agents" class="data-table" height="100%" :empty-text="agentEmptyText">
                  <el-table-column prop="hostName" label="主机名称" min-width="170" show-overflow-tooltip>
                    <template #default="{ row }">
                      <div class="name-cell">
                        <strong>{{ agentHostName(row) }}</strong>
                      </div>
                    </template>
                  </el-table-column>
                  <el-table-column label="IP" min-width="130" show-overflow-tooltip>
                    <template #default="{ row }">
                      <span class="mono-text">{{ agentIp(row) }}</span>
                    </template>
                  </el-table-column>
                  <el-table-column prop="agentOnlineStatus" label="Agent 状态" min-width="130">
                    <template #default="{ row }">
                      <div v-if="shouldShowCmdbAgentProgress(row)" class="agent-status-progress" :class="cmdbAgentProgressClass(row)">
                        <div class="agent-status-progress-main">
                          <el-progress
                            :percentage="cmdbAgentProgress(row)"
                            :status="cmdbAgentProgressStatus(row)"
                            :stroke-width="8"
                            :show-text="false"
                          />
                          <span class="agent-status-progress-icon">{{ cmdbAgentProgressIcon(row) }}</span>
                        </div>
                        <div class="agent-status-progress-caption">
                          {{ cmdbAgentProgressText(row) }}
                        </div>
                      </div>
                      <el-tag v-else :type="agentOnline(row) ? 'success' : 'danger'" size="small" effect="plain">
                        {{ agentOnline(row) ? '在线' : '离线' }}
                      </el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column prop="vectorStatus" label="Vector 状态" width="118">
                    <template #default="{ row }">
                      <el-tag :type="vectorStatusType(row)" size="small" effect="plain">
                        {{ vectorStatusText(row) }}
                      </el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column label="安装进度" min-width="135">
                    <template #default="{ row }">
                      <div v-if="shouldShowOperationProgress(row)" class="operation-progress" :class="operationProgressClass(row)">
                        <div class="operation-progress-main">
                          <el-progress
                            :percentage="operationProgress(row)"
                            :status="operationProgressStatus(row)"
                            :stroke-width="8"
                            :show-text="false"
                          />
                          <span class="operation-progress-icon">{{ operationProgressIcon(row) }}</span>
                        </div>
                        <div class="operation-progress-caption">
                          {{ operationProgressText(row) }}
                        </div>
                      </div>
                      <span v-else :class="operationIdleClass(row)">{{ operationIdleText(row) }}</span>
                    </template>
                  </el-table-column>
                  <el-table-column prop="configStatus" label="配置状态" width="180">
                    <template #default="{ row }">
                      <el-tag :type="configStatusType(row)" class="config-status-tag" :class="configStatusClass(row)" size="small" effect="plain">
                        {{ configStatusText(row) }}
                      </el-tag>
                    </template>
                  </el-table-column>
                  <el-table-column prop="lastVectorHeartbeat" label="最近上报" min-width="150" show-overflow-tooltip>
                    <template #default="{ row }">{{ vectorHeartbeatText(row) }}</template>
                  </el-table-column>
                  <el-table-column prop="lastError" label="最近错误" min-width="120" show-overflow-tooltip>
                    <template #default="{ row }">
                      <button
                        v-if="canOpenErrorDialog(row)"
                        type="button"
                        class="error-icon-button"
                        title="查看错误详情"
                        @click="openErrorDialog(row)"
                      >
                        !
                      </button>
                      <span v-else>-</span>
                    </template>
                  </el-table-column>
                  <el-table-column label="操作" fixed="right" width="220">
                    <template #default="{ row }">
                      <div class="operation-links">
                        <el-button
                          link
                          :type="vectorPrimaryActionType(row)"
                          :disabled="!canSubmitVectorPrimaryAction(row)"
                          @click="submitAgentOperation(row, vectorPrimaryActionOp(row))"
                        >
                          {{ vectorPrimaryActionText(row) }}
                        </el-button>
                        <el-button link type="primary" @click="openAgentDrawer(row, 'basic')">详情</el-button>
                        <el-button link type="primary" :disabled="!canOperateVector(row)" @click="openAgentDrawer(row, 'config')">配置</el-button>
                        <el-dropdown trigger="click" @command="command => handleAgentMoreCommand(row, command)">
                          <el-button link type="primary">更多</el-button>
                          <template #dropdown>
                            <el-dropdown-menu>
                              <el-dropdown-item command="restart" :disabled="!canOperateVector(row)">
                                <span class="dropdown-action-item">
                                  <img :src="restartIcon" alt="" />
                                  <span>重启 Vector</span>
                                </span>
                              </el-dropdown-item>
                              <el-dropdown-item
                                v-if="currentTask && !isK8sTask(currentTask)"
                                command="stopCollect"
                                :disabled="!canStopCollectTask(currentTask) || taskActionLoading"
                                divided
                              >
                                <span class="dropdown-action-item danger">
                                  <el-icon><VideoPause /></el-icon>
                                  <span>停止采集</span>
                                </span>
                              </el-dropdown-item>
                              <el-dropdown-item command="uninstall" :disabled="!canUninstallVector(row)" divided>
                                <span class="dropdown-action-item danger">
                                  <img :src="uninstallIcon" alt="" />
                                  <span>卸载 Vector</span>
                                </span>
                              </el-dropdown-item>
                              <el-dropdown-item command="redeployAgent" :disabled="!canRedeployCmdbAgent(row)">
                                <span class="dropdown-action-item">
                                  <img :src="refreshIcon" alt="" />
                                  <span>重装 Agent</span>
                                </span>
                              </el-dropdown-item>
                            </el-dropdown-menu>
                          </template>
                        </el-dropdown>
                      </div>
                    </template>
                  </el-table-column>
                </el-table>
              </div>
              <LogK8sCollector
                v-else
                ref="k8sCollectorPageRef"
                :selected-node="selectedNode"
                :active="isK8sCollectorContentTab"
                :view="k8sCollectorView"
              />

              <div v-if="!isK8sCollectorContentTab" class="pagination-section">
                <el-pagination
                  :current-page="agentPagination.page"
                  :page-size="agentPagination.pageSize"
                  :page-sizes="[10, 20, 50, 100]"
                  :total="agentPagination.total"
                  layout="total, sizes, prev, pager, next, jumper"
                  @size-change="handleAgentSizeChange"
                  @current-change="handleAgentPageChange"
                />
              </div>
            </div>
          </div>
        </main>
      </section>
    </section>

    <el-dialog
      v-model="taskDialog.visible"
      :title="taskDialog.form.id ? '编辑采集任务' : '新增采集任务'"
      width="820px"
      destroy-on-close
      class="log-dialog"
      modal-class="log-agent-dialog-overlay"
    >
      <el-form ref="taskFormRef" :model="taskDialog.form" :rules="taskRules" label-width="118px">
        <div class="form-grid">
          <el-form-item label="项目" prop="projectId">
            <el-select v-model="taskDialog.form.projectId" placeholder="选择项目" filterable @change="handleDialogProjectChange">
              <el-option v-for="item in projects" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="目标日志库" prop="storeId">
            <el-select v-model="taskDialog.form.storeId" placeholder="选择日志库" filterable @change="handleDialogStoreChange">
              <el-option v-for="item in dialogStores" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="任务名称" prop="name">
            <el-input v-model.trim="taskDialog.form.name" placeholder="Nginx 日志采集" />
          </el-form-item>
          <el-form-item label="采集场景" prop="collectType">
            <el-radio-group v-model="taskDialog.form.collectType" @change="handleCollectTypeChange">
              <el-radio-button label="host">主机日志</el-radio-button>
              <el-radio-button label="kubernetes">K8S 日志</el-radio-button>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="日志类型" prop="logType">
            <el-select v-model="taskDialog.form.logType" placeholder="选择日志类型" filterable @change="handleLogTypeChange">
              <el-option
                v-for="item in taskLogTypeOptions"
                :key="item.value"
                :label="item.label"
                :value="item.value"
              />
            </el-select>
          </el-form-item>
          <el-form-item v-if="isTaskDialogHost" :label="targetAgentLabel" prop="targetAgentIds" required>
            <el-select
              v-model="taskDialog.form.targetAgentIds"
              multiple
              filterable
              collapse-tags
              collapse-tags-tooltip
              :max-collapse-tags="2"
              :loading="dialogAgentLoading"
              :placeholder="targetAgentPlaceholder"
              :no-data-text="targetAgentEmptyText"
            >
              <el-option
                v-for="agent in dialogAgents"
                :key="agentIdOf(agent)"
                :label="dialogAgentLabel(agent)"
                :value="agentSelectValue(agent)"
                :disabled="!isSelectableDialogAgent(agent)"
              >
                <div class="agent-option">
                  <strong>{{ agentHostName(agent) }}</strong>
                  <span>{{ agentIp(agent) }}</span>
                </div>
              </el-option>
            </el-select>
          </el-form-item>
          <template v-if="isTaskDialogK8s">
            <el-form-item label="K8S 集群" prop="k8sClusterId">
              <el-select
                v-model="taskDialog.form.k8sClusterId"
                placeholder="选择集群"
                filterable
                :loading="k8sLoading.clusters"
                @visible-change="visible => visible && loadK8sFormClusters(true)"
                @change="handleK8sClusterChange"
              >
                <el-option
                  v-for="cluster in k8sClusters"
                  :key="cluster.id"
                  :label="clusterNameOf(cluster)"
                  :value="cluster.id"
                  :disabled="!isSelectableK8sCluster(cluster)"
                >
                  <div class="k8s-cluster-option">
                    <strong>{{ clusterNameOf(cluster) }}</strong>
                    <span :class="{ 'is-ready': isSelectableK8sCluster(cluster) }">{{ k8sClusterOptionTip(cluster) }}</span>
                  </div>
                </el-option>
              </el-select>
            </el-form-item>
            <template v-if="showK8sWorkloadFields">
              <el-form-item label="命名空间" prop="k8sNamespace">
                <el-select
                  v-model="taskDialog.form.k8sNamespace"
                  placeholder="选择命名空间"
                  filterable
                  :disabled="!taskDialog.form.k8sClusterId"
                  :loading="k8sLoading.namespaces"
                  @visible-change="visible => visible && loadK8sFormNamespaces()"
                  @change="handleK8sNamespaceChange"
                >
                  <el-option v-for="namespace in k8sNamespaces" :key="namespace.name" :label="namespace.name" :value="namespace.name" />
                </el-select>
              </el-form-item>
              <el-form-item label="工作负载类型" prop="k8sWorkloadType">
                <el-select
                  v-model="taskDialog.form.k8sWorkloadType"
                  placeholder="选择类型"
                  :disabled="!taskDialog.form.k8sNamespace"
                  @change="handleK8sWorkloadTypeChange"
                >
                  <el-option v-for="item in k8sWorkloadTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
                </el-select>
              </el-form-item>
              <el-form-item class="k8s-workload-form-item" label="工作负载" prop="k8sWorkloadName">
                <el-select
                  v-model="taskDialog.form.k8sWorkloadName"
                  placeholder="选择工作负载"
                  filterable
                  remote
                  clearable
                  :disabled="!taskDialog.form.k8sNamespace"
                  :loading="k8sLoading.workloads"
                  :remote-method="searchK8sFormWorkloads"
                  @visible-change="handleK8sWorkloadVisibleChange"
                  @change="handleK8sWorkloadChange"
                >
                  <el-option
                    v-for="workload in k8sWorkloads"
                    :key="`${workload.type}-${workload.name}`"
                    :label="workload.name"
                    :value="workload.name"
                  />
                </el-select>
              </el-form-item>
            </template>
            <el-form-item label="采集来源" prop="k8sLogSource">
              <el-radio-group v-model="taskDialog.form.k8sLogSource" class="k8s-log-source-group" @change="handleK8sLogSourceChange">
                <el-radio v-for="item in k8sLogSourceOptions" :key="item.value" :label="item.value">
                  {{ item.label }}
                </el-radio>
              </el-radio-group>
            </el-form-item>
          </template>
          <el-form-item v-if="showPathPatternEditor" label="多行日志">
            <div class="multiline-config">
              <el-switch
                v-model="taskDialog.form.multilineEnabledSnapshot"
                inline-prompt
                active-text="合并"
                inactive-text="不合并"
                :active-value="true"
                :inactive-value="false"
                @change="handleMultilineEnabledChange"
              />
            </div>
          </el-form-item>
          <el-form-item label="状态">
            <el-radio-group
              v-model="taskDialog.form.status"
              class="task-status-toggle"
              :class="Number(taskDialog.form.status) === 2 ? 'is-status-disabled' : 'is-status-enabled'"
            >
              <el-radio-button class="status-radio-enable" :label="1">启用</el-radio-button>
              <el-radio-button class="status-radio-disable" :label="2">禁用</el-radio-button>
            </el-radio-group>
          </el-form-item>
        </div>

        <div v-if="showAdvancedParseConfig" class="parse-config-section">
          <div class="parse-config-body">
            <el-form-item v-if="showRegexPatternField" label="解析正则" prop="regexPattern">
              <el-input
                v-model="taskDialog.form.regexPattern"
                type="textarea"
                :rows="2"
                :placeholder="regexPatternPlaceholder"
              />
            </el-form-item>
            <el-form-item v-if="showSampleLogField" prop="sampleLog" class="sample-log-form-item">
              <div class="sample-log-ai-field">
                <div class="sample-log-ai-head">
                  <span class="sample-log-ai-label">样例日志</span>
                  <el-button type="primary" size="small" :loading="aiParse.loading" @click="generateAiParseTemplate">AI解析</el-button>
                </div>
                <el-input
                  v-model="taskDialog.form.sampleLog"
                  type="textarea"
                  :rows="5"
                  :placeholder="sampleLogPlaceholder"
                  @input="handleSampleLogInput"
                />
                <div v-if="aiParse.error" class="ai-parse-error">{{ aiParse.error }}</div>
                <div v-if="aiParse.result" class="ai-parse-preview">
                  <div class="ai-parse-preview-head">
                    <strong>AI解析结果</strong>
                    <el-tag v-if="taskDialog.form.parseTemplateId && sameId(taskDialog.form.parseTemplateId, aiParseTemplateId)" type="success" size="small" effect="plain">已使用</el-tag>
                  </div>
                  <div class="ai-parse-summary">
                    <span><b>解析模式：</b>{{ aiParseModeText }}</span>
                    <span><b>模板ID：</b>{{ aiParseTemplateId || '-' }}</span>
                  </div>
                  <div v-if="aiParseFieldsText" class="ai-parse-line">
                    <b>识别字段：</b>
                    <span>{{ aiParseFieldsText }}</span>
                  </div>
                  <div v-if="aiParseMappingsText" class="ai-parse-line">
                    <b>字段映射：</b>
                    <span>{{ aiParseMappingsText }}</span>
                  </div>
                  <div v-if="aiParseWarningsText" class="ai-parse-warning">
                    {{ aiParseWarningsText }}
                  </div>
                  <div class="ai-parse-actions">
                    <el-button type="success" size="small" :disabled="!aiParseTemplateId" @click="useAiParseTemplate">使用该规则</el-button>
                  </div>
                </div>
              </div>
            </el-form-item>
          </div>
        </div>

        <el-form-item v-if="showPathPatternEditor" :label="pathPatternLabel" prop="pathPatterns">
          <div class="path-pattern-editor">
            <div
              v-for="(path, index) in taskDialog.form.pathPatterns"
              :key="`task-path-${index}`"
              :data-path="path"
              class="path-pattern-row"
            >
              <el-input
                v-model.trim="taskDialog.form.pathPatterns[index]"
                :placeholder="pathPatternPlaceholder"
                clearable
              />
              <el-button type="success" :icon="Plus" aria-label="新增路径" @click="addPathPatternRow(taskDialog.form)" />
              <el-button
                type="danger"
                :icon="Delete"
                aria-label="删除路径"
                :disabled="taskDialog.form.pathPatterns.length <= 1"
                @click="removePathPatternRow(taskDialog.form, index)"
              />
            </div>
            <div class="path-editor-hint">{{ pathPatternHint }}</div>
          </div>
        </el-form-item>
        <div v-else-if="isTaskDialogK8s" class="k8s-stdout-hint">
          {{ k8sNoPathHint }}
        </div>
        <el-form-item label="备注">
          <el-input v-model="taskDialog.form.remark" type="textarea" :rows="2" placeholder="任务说明" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="taskDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="taskDialog.submitting" @click="submitTask">
          {{ taskDialog.form.id ? '保存并下发' : '保存' }}
        </el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="pathEditDialog.visible"
      title="编辑采集路径"
      width="560px"
      destroy-on-close
      class="log-dialog"
      modal-class="log-agent-dialog-overlay"
    >
      <el-form ref="pathEditFormRef" :model="pathEditDialog.form" :rules="pathEditRules" label-width="96px">
        <el-form-item label="任务名称">
          <el-input v-model="pathEditDialog.form.name" disabled />
        </el-form-item>
        <el-form-item label="日志路径" prop="pathPatterns">
          <div class="path-pattern-editor">
            <div
              v-for="(path, index) in pathEditDialog.form.pathPatterns"
              :key="`edit-path-${index}`"
              :data-path="path"
              class="path-pattern-row"
            >
              <el-input
                v-model.trim="pathEditDialog.form.pathPatterns[index]"
                placeholder="/var/log/messages"
                clearable
              />
              <el-button type="success" :icon="Plus" aria-label="新增路径" @click="addPathPatternRow(pathEditDialog.form)" />
              <el-button
                type="danger"
                :icon="Delete"
                aria-label="删除路径"
                :disabled="pathEditDialog.form.pathPatterns.length <= 1"
                @click="removePathPatternRow(pathEditDialog.form, index)"
              />
            </div>
            <div class="path-editor-hint">保存时会自动生成多路径 include，并重新下发配置。</div>
          </div>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="pathEditDialog.form.remark" type="textarea" :rows="3" placeholder="可选" />
        </el-form-item>
      </el-form>
      <div class="path-pattern-tips">
        <div class="path-tips-title">路径填写示例</div>
        <div class="path-tip-list">
          <div class="path-tip-item">
            <span>精确文件</span>
            <code>/var/log/messages</code>
          </div>
          <div class="path-tip-item">
            <span>单层通配</span>
            <code>/var/log/*.log</code>
          </div>
          <div class="path-tip-item">
            <span>递归通配</span>
            <code>/var/log/**/*.log</code>
          </div>
        </div>
      </div>
      <template #footer>
        <el-button @click="pathEditDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="pathEditDialog.submitting" @click="submitPathEdit">
          保存
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="packageDialog.visible" title="Vector 管理" width="640px" class="log-dialog" modal-class="log-agent-dialog-overlay">
      <el-alert
        v-if="packageInfo && !packageReady"
        title="未找到可用 Vector 包，请上传后再安装 Vector。"
        type="warning"
        show-icon
        :closable="false"
        class="package-alert"
      />
      <el-descriptions :column="1" border>
        <el-descriptions-item label="状态">{{ packageStatusText }}</el-descriptions-item>
        <el-descriptions-item label="版本">{{ packageInfo?.version || '-' }}</el-descriptions-item>
        <el-descriptions-item label="文件名">{{ packageInfo?.fileName || '-' }}</el-descriptions-item>
        <el-descriptions-item label="大小">{{ formatBytes(packageInfo?.sizeBytes) }}</el-descriptions-item>
        <el-descriptions-item label="SHA256">
          <span class="mono-text">{{ packageInfo?.sha256 || '-' }}</span>
        </el-descriptions-item>
        <el-descriptions-item label="官网下载">
          <a class="official-download-link" href="https://vector.dev/download/" target="_blank" rel="noopener noreferrer">
            官网 Vector 下载
          </a>
        </el-descriptions-item>
        <el-descriptions-item label="更新时间">{{ formatTime(packageInfo?.modTime) }}</el-descriptions-item>
      </el-descriptions>
      <div v-if="packageUploadTask.taskId || packageUploading" class="package-upload-progress">
        <div class="package-upload-progress-head">
          <strong>{{ packageUploadMessage }}</strong>
          <span>{{ packageUploadBytesText }}</span>
        </div>
        <el-progress
          :percentage="packageUploadProgress"
          :status="packageUploadProgressStatus"
          :stroke-width="10"
        />
      </div>
      <input ref="packageFileInputRef" type="file" class="hidden-file-input" @change="handlePackageFileChange" />
      <template #footer>
        <el-button @click="packageDialog.visible = false">关闭</el-button>
        <el-button type="primary" :loading="packageLoading" @click="loadVectorPackage">检查</el-button>
        <el-button type="success" :loading="packageUploading" @click="chooseVectorPackageFile">上传</el-button>
        <el-button type="danger" :loading="packageDeleting" :disabled="!packageReady" @click="removeVectorPackage">删除</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="aggregatorDialog.visible" title="Aggregator 配置" width="860px" class="log-dialog" modal-class="log-agent-dialog-overlay">
      <section v-loading="aggregatorConfigLoading" class="aggregator-config-panel aggregator-config-panel--dialog">
        <div class="aggregator-config-head">
          <div>
            <h4>Aggregator 配置</h4>
            <span>{{ aggregatorConfigSummary }}</span>
          </div>
          <el-button link type="primary" :loading="aggregatorConfigLoading" @click="loadAggregatorConfigPreview">刷新</el-button>
        </div>
        <el-alert
          v-if="aggregatorConfigError"
          :title="aggregatorConfigError"
          type="warning"
          show-icon
          :closable="false"
          class="aggregator-config-alert"
        />
        <div class="aggregator-action-row">
          <el-button
            type="primary"
            size="small"
            :icon="DocumentCopy"
            :disabled="!hasAggregatorVectorYaml"
            @click="copyAggregatorVectorYaml"
          >
            复制 vector.yaml
          </el-button>
          <el-button
            type="success"
            size="small"
            :icon="DocumentCopy"
            :disabled="!hasAggregatorDockerCommand"
            @click="copyAggregatorDockerCommand"
          >
            复制 Docker 安装命令
          </el-button>
          <el-button
            type="warning"
            size="small"
            :icon="Download"
            :disabled="!hasAggregatorKubernetesYaml"
            @click="downloadAggregatorK8sYaml"
          >
            下载 K8s YAML
          </el-button>
        </div>
        <div class="aggregator-meta-grid">
          <span>
            <b>镜像</b>
            <em>{{ aggregatorConfigSummary }}</em>
          </span>
          <span>
            <b>容器</b>
            <em>{{ aggregatorDockerName }}</em>
          </span>
          <span>
            <b>配置路径</b>
            <em>{{ aggregatorDockerConfigPath }}</em>
          </span>
          <span>
            <b>数据目录</b>
            <em>{{ aggregatorDockerDataDir }}</em>
          </span>
          <span>
            <b>命名空间</b>
            <em>{{ aggregatorKubernetesNamespace }}</em>
          </span>
          <span>
            <b>Deployment</b>
            <em>{{ aggregatorKubernetesName }}</em>
          </span>
          <span>
            <b>ConfigMap</b>
            <em>{{ aggregatorKubernetesConfigMapName }}</em>
          </span>
          <span>
            <b>应用命令</b>
            <em>{{ aggregatorKubernetesApplyCommand }}</em>
          </span>
        </div>
        <div class="aggregator-config-editor">
          <CodeEditor
            :model-value="aggregatorVectorYaml"
            language="yaml"
            readonly
            height="320px"
            theme="aiops-sql-light"
            word-wrap="on"
            placeholder="暂无 Aggregator vector.yaml 配置"
          />
        </div>
      </section>
      <template #footer>
        <el-button @click="aggregatorDialog.visible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="errorDialog.visible" title="错误详情" width="760px" class="log-dialog" modal-class="log-agent-dialog-overlay">
      <el-descriptions v-if="errorDialog.current" :column="2" border class="error-meta">
        <el-descriptions-item label="主机">{{ agentHostName(errorDialog.current) }}</el-descriptions-item>
        <el-descriptions-item label="IP">{{ agentIp(errorDialog.current) }}</el-descriptions-item>
        <el-descriptions-item label="Agent ID">{{ agentIdOf(errorDialog.current) || '-' }}</el-descriptions-item>
        <el-descriptions-item label="错误时间">{{ formatTime(errorDialog.current.operationErrorAt || errorDialog.current.lastErrorAt) }}</el-descriptions-item>
      </el-descriptions>
      <pre class="error-detail-content">{{ errorDialog.message }}</pre>
      <template #footer>
        <el-button @click="errorDialog.visible = false">关闭</el-button>
      </template>
    </el-dialog>

    <el-drawer
      v-model="agentDrawer.visible"
      :title="agentDrawerTitle"
      size="clamp(420px, 29vw, 620px)"
      class="log-drawer"
      modal-class="log-agent-drawer-overlay"
    >
      <el-tabs v-model="agentDrawer.activeTab" @tab-change="handleAgentDrawerTabChange">
        <el-tab-pane label="详情" name="basic">
          <el-descriptions v-if="agentDrawer.current" :column="2" border>
            <el-descriptions-item label="主机名称">{{ agentHostName(agentDrawer.current) }}</el-descriptions-item>
            <el-descriptions-item label="IP 地址">{{ agentIp(agentDrawer.current) }}</el-descriptions-item>
            <el-descriptions-item label="Host ID">{{ agentDrawer.current.hostId || '-' }}</el-descriptions-item>
            <el-descriptions-item label="Agent ID">{{ agentIdOf(agentDrawer.current) || '-' }}</el-descriptions-item>
            <el-descriptions-item label="Agent 版本">{{ agentDrawer.current.agentVersion || agentDrawer.current.version || '-' }}</el-descriptions-item>
            <el-descriptions-item label="Agent 状态">{{ agentOnline(agentDrawer.current) ? '在线' : '离线' }}</el-descriptions-item>
            <el-descriptions-item label="Agent 心跳">{{ formatTime(agentDrawer.current.agentLastHeartbeat || agentDrawer.current.lastHeartbeat) }}</el-descriptions-item>
            <el-descriptions-item label="Vector 版本">{{ agentDrawer.current.vectorVersion || '-' }}</el-descriptions-item>
            <el-descriptions-item label="Vector 状态">{{ vectorStatusText(agentDrawer.current) }}</el-descriptions-item>
            <el-descriptions-item label="Vector PID">{{ agentDrawer.current.vectorPid || '-' }}</el-descriptions-item>
            <el-descriptions-item label="安装路径" :span="2">{{ agentDrawer.current.installPath || '-' }}</el-descriptions-item>
            <el-descriptions-item label="配置路径" :span="2">{{ agentDrawer.current.configPath || '-' }}</el-descriptions-item>
            <el-descriptions-item label="错误" :span="2">
              <span class="danger-text">{{ agentRecentError(agentDrawer.current) || '-' }}</span>
            </el-descriptions-item>
          </el-descriptions>
          <div class="detail-operations-head">
            <h4>操作记录</h4>
            <el-button link type="primary" :loading="drawerLoading.operations" @click="loadAgentOperations">刷新</el-button>
          </div>
          <el-table v-loading="drawerLoading.operations" :data="agentOperations" height="220" empty-text="暂无操作记录">
            <el-table-column prop="opType" label="操作类型" min-width="130">
              <template #default="{ row }">{{ operationTypeText(row.opType) }}</template>
            </el-table-column>
            <el-table-column prop="opStatus" label="状态" width="110">
              <template #default="{ row }">
                <el-tag :type="operationStatusType(row.opStatus)" size="small" effect="plain">{{ operationStatusText(row.opStatus) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="createdAt" label="创建时间" min-width="160">
              <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
            </el-table-column>
            <el-table-column prop="errorMsg" label="错误信息" min-width="220" show-overflow-tooltip>
              <template #default="{ row }">
                <span class="danger-text">{{ row.errorMsg || '-' }}</span>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
        <el-tab-pane label="配置预览" name="config">
          <div class="drawer-toolbar">
            <el-button
              type="success"
              size="small"
              :disabled="!editableConfigTask"
              @click="openPathEditDialog"
            >
              编辑采集路径
            </el-button>
            <el-button type="primary" size="small" :disabled="!agentDrawer.current" @click="submitAgentOperation(agentDrawer.current, 'reload_vector')">重载配置</el-button>
            <el-button size="small" :loading="drawerLoading.config" @click="loadConfigPreview">刷新预览</el-button>
          </div>
          <div v-loading="drawerLoading.config" class="config-preview-editor">
            <CodeEditor
              :model-value="configPreviewContent"
              language="yaml"
              readonly
              height="calc(100vh - 250px)"
              theme="aiops-sql-light"
              word-wrap="on"
              placeholder="暂无配置预览"
            />
          </div>
        </el-tab-pane>
        <el-tab-pane label="采集任务" name="tasks">
          <el-table v-loading="drawerLoading.tasks" :data="agentTasks" height="420" empty-text="暂无采集任务">
            <el-table-column prop="name" label="任务名称" min-width="160" show-overflow-tooltip />
            <el-table-column prop="storeName" label="日志库" min-width="140" show-overflow-tooltip>
              <template #default="{ row }">{{ row.storeName || row.storeCode || '-' }}</template>
            </el-table-column>
            <el-table-column prop="pathPattern" label="日志路径" min-width="220" show-overflow-tooltip>
              <template #default="{ row }">{{ taskPathText(row) }}</template>
            </el-table-column>
            <el-table-column prop="logType" label="日志类型" min-width="190" show-overflow-tooltip>
              <template #default="{ row }">{{ logTypeText(taskLogType(row)) }}</template>
            </el-table-column>
            <el-table-column prop="parseModeSnapshot" label="解析" width="100">
              <template #default="{ row }">{{ row.parseModeSnapshot || 'raw' }}</template>
            </el-table-column>
            <el-table-column prop="status" label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="statusTagType(row.status)" size="small" effect="plain">{{ statusText(row.status) }}</el-tag>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
        <el-tab-pane label="操作记录" name="operations">
          <el-table v-loading="drawerLoading.operations" :data="agentOperations" height="420" empty-text="暂无操作记录">
            <el-table-column prop="opType" label="操作类型" min-width="130">
              <template #default="{ row }">{{ operationTypeText(row.opType) }}</template>
            </el-table-column>
            <el-table-column prop="opStatus" label="状态" width="110">
              <template #default="{ row }">
                <el-tag :type="operationStatusType(row.opStatus)" size="small" effect="plain">{{ operationStatusText(row.opStatus) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="operator" label="操作人" width="100" />
            <el-table-column prop="createdAt" label="创建时间" min-width="160">
              <template #default="{ row }">{{ formatTime(row.createdAt) }}</template>
            </el-table-column>
            <el-table-column prop="errorMsg" label="错误信息" min-width="220" show-overflow-tooltip>
              <template #default="{ row }">
                <span class="danger-text">{{ row.errorMsg || '-' }}</span>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </el-drawer>

  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowRight, Delete, DocumentCopy, Download, EditPen, Plus, Refresh, Search, VideoPause, View } from '@element-plus/icons-vue'
import logtailApi from '@/api/logtail'
import k8sApi from '@/api/k8s'
import cmdbAPI from '@/api/cmdb'
import CodeEditor from '@/components/CodeEditor.vue'
import logSvg from '@/assets/image/日志.svg'
import openFolderIcon from '@/assets/image/打开文件夹.svg'
import closedFolderIcon from '@/assets/image/关闭文件夹.svg'
import logStoreIcon from '@/assets/image/日志.svg'
import collectorIcon from '@/assets/image/icon-collect.svg'
import LogK8sCollector from './Log_K8sCollector.vue'

const router = useRouter()
const refreshIcon = new URL('@/assets/image/刷新.svg', import.meta.url).href
const restartIcon = new URL('@/assets/image/重启.svg', import.meta.url).href
const uninstallIcon = new URL('@/assets/image/卸载.svg', import.meta.url).href
const projects = ref([])
const dialogStores = ref([])
const dialogAgents = ref([])
const dialogAgentMode = ref('available')
const k8sClusters = ref([])
const k8sNamespaces = ref([])
const k8sWorkloads = ref([])
const k8sWorkloadSearchKeyword = ref('')
const tasks = ref([])
const agents = ref([])
const configTree = ref([])
const treeKeyword = ref('')
const expandedProjectKey = ref('')
const expandedStoreKeys = ref([])
const selectedNode = ref(null)
const agentContentTab = ref('task')
const scopeAgentRows = ref([])
const statAgentRows = ref([])
const selectedTaskId = ref(null)
const packageInfo = ref(null)
const aggregatorConfigPreview = ref(null)
const aggregatorConfigError = ref('')
const configPreview = ref(null)
const agentTasks = ref([])
const agentOperations = ref([])
const taskLoading = ref(false)
const agentLoading = ref(false)
const packageLoading = ref(false)
const taskActionLoading = ref(false)
const aggregatorConfigLoading = ref(false)
const packageUploading = ref(false)
const packageDeleting = ref(false)
const dialogAgentLoading = ref(false)
const taskFormRef = ref()
const pathEditFormRef = ref()
const packageFileInputRef = ref()
const k8sCollectorPageRef = ref()
const k8sCollectorTabNames = new Set(['k8s', 'k8sCluster'])
const cmdbAgentCache = new Map()
const operationErrorHydratedIds = new Set()
const operationErrorCache = new Map()
const operationPollTargets = new Map()
let operationPollTimer = null
let operationPollDeadline = 0
let operationPollingInFlight = false
let packageUploadPollTimer = null
let cmdbAgentPollTimer = null
let cmdbAgentPollDeadline = 0
let cmdbAgentPollingInFlight = false
let cmdbAgentPollCounter = 0
let k8sWorkloadRequestSeq = 0
const MAX_CMDB_AGENT_POLL_COUNT = 5
const K8S_WORKLOAD_PAGE_SIZE = 10
const K8S_WORKLOAD_MAX_PAGES = 50

const taskQuery = reactive({
  page: 1,
  pageSize: 10,
  projectId: '',
  storeId: '',
  keyword: '',
  collectType: '',
  status: ''
})

const agentQuery = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  onlineStatus: '',
  vectorStatus: '',
  configStatus: ''
})

const taskPagination = reactive({ page: 1, pageSize: 10, total: 0 })
const agentPagination = reactive({ page: 1, pageSize: 10, total: 0 })

const k8sLoading = reactive({
  clusters: false,
  namespaces: false,
  workloads: false
})

const logTypeOptions = [
  { label: '文本日志', value: 'txt' },
  { label: 'JSON结构化日志', value: 'json' },
  { label: 'Nginx日志', value: 'nginx' },
  { label: '数据库慢日志', value: 'db_slow_log' }
]
const defaultLogType = 'txt'
const logTypeValueSet = new Set(logTypeOptions.map(item => item.value))
const sampleLogTypeSet = new Set(['json', 'nginx', 'regex'])
const legacyNginxLogTypes = new Set([
  'nginx_json', 'nginx+json', 'json_nginx', 'json+nginx', 'nginx-json', 'json-nginx',
  'nginx_json_loose', 'nginx+json-loose', 'nginx-json-loose', 'nginx+loose-json',
  'nginx_loose_json', 'json_loose_nginx', 'json+nginx-loose'
])
const namedCaptureGroupRegExp = /\(\?P<[A-Za-z_][A-Za-z0-9_]*>/
const legacyDefaultTextMultilinePattern = '^(\\d{4}[-/]\\d{2}[-/]\\d{2}[ T]\\d{2}:\\d{2}:\\d{2}([.,]\\d+)?|\\[\\d{4}-\\d{2}-\\d{2}[ T]\\d{2}:\\d{2}:\\d{2})'
const previousDefaultTextMultilinePattern = '^(\\d{4}[-/]\\d{2}[-/]\\d{2}[ T]\\d{2}:\\d{2}:\\d{2}([.,]\\d+)?|\\[\\d{4}[-/]\\d{2}[-/]\\d{2}[ T]\\d{2}:\\d{2}:\\d{2}([.,]\\d+)?\\]?)'
const defaultTextMultilinePattern = '^(?:\\[?(?:\\d{4}[-/.]\\d{2}[-/.]\\d{2}|\\d{2}[-/]\\d{2})[ T]\\d{2}:\\d{2}:\\d{2}(?:[.,]\\d+)?(?:Z|[+-]\\d{2}:?\\d{2})?\\]?|[A-Z][a-z]{2}[ ]+\\d{1,2}[ T]\\d{2}:\\d{2}:\\d{2}|\\[\\d{1,2}/[A-Z][a-z]{2}/\\d{4}:\\d{2}:\\d{2}:\\d{2}(?: [+-]\\d{4})?\\]|\\[[A-Z][a-z]{2} [A-Z][a-z]{2}[ ]+\\d{1,2} \\d{2}:\\d{2}:\\d{2}(?:\\.\\d+)? \\d{4}\\]|<\\d{1,3}>\\d?[ ]+\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2})'
const defaultSlowLogMultilinePattern = '^# Time: '

const k8sLogSourceOptions = [
  { label: 'K8S标准日志(stdout)', value: 'stdout' },
  { label: 'K8S容器日志(sidecar)', value: 'container_file' },
  { label: 'K8S节点日志(hostPath)', value: 'node_file' }
]

const k8sWorkloadTypeOptions = [
  { label: 'Deployment', value: 'deployment', apiValue: 'deployments' },
  { label: 'StatefulSet', value: 'statefulset', apiValue: 'statefulsets' },
  { label: 'DaemonSet', value: 'daemonset', apiValue: 'daemonsets' },
  { label: 'Job', value: 'job', apiValue: 'jobs' },
  { label: 'CronJob', value: 'cronjob', apiValue: 'cronjobs' }
]

function normalizeCollectType(value) {
  const type = String(value || 'host').trim().toLowerCase()
  return ['k8s', 'kubernetes'].includes(type) ? 'kubernetes' : 'host'
}

function isK8sCollectType(value) {
  return normalizeCollectType(value) === 'kubernetes'
}

function resolveK8sLogSource(value) {
  const raw = Array.isArray(value) ? value[0] : value
  const source = String(raw || '').trim().toLowerCase()
  if (!source) return ''
  if (['stdout', 'stdouterr', 'stdout_stderr', 'stdout/stderr'].includes(source)) return 'stdout'
  if (['node_file', 'host_path', 'hostpath', 'host_path_file', 'node_path', 'node_path_file'].includes(source)) return 'node_file'
  if (['container_file', 'containerfile', 'container_path'].includes(source)) return 'container_file'
  if (source === 'both' || source.includes('container_file')) return 'container_file'
  return ''
}

function normalizeK8sLogSource(value) {
  return resolveK8sLogSource(value) || 'stdout'
}

function k8sLogSourceSelections(value) {
  return [normalizeK8sLogSource(value)]
}

function k8sLogSourceRequiresPath(value) {
  return ['node_file', 'container_file'].includes(normalizeK8sLogSource(value))
}

function k8sLogSourceNeedsWorkload(value) {
  return ['stdout', 'container_file'].includes(normalizeK8sLogSource(value))
}

function normalizeK8sWorkloadType(value) {
  const type = String(value || '').trim().toLowerCase()
  if (['deployments', 'deployment'].includes(type)) return 'deployment'
  if (['statefulsets', 'statefulset'].includes(type)) return 'statefulset'
  if (['daemonsets', 'daemonset'].includes(type)) return 'daemonset'
  if (['jobs', 'job'].includes(type)) return 'job'
  if (['cronjobs', 'cronjob'].includes(type)) return 'cronjob'
  return type || 'deployment'
}

function k8sWorkloadTypeForApi(value) {
  const type = normalizeK8sWorkloadType(value)
  return k8sWorkloadTypeOptions.find(item => item.value === type)?.apiValue || 'deployments'
}

function k8sWorkloadTypeText(value) {
  const type = normalizeK8sWorkloadType(value)
  return k8sWorkloadTypeOptions.find(item => item.value === type)?.label || type || '-'
}

function k8sLogSourceText(value) {
  const source = normalizeK8sLogSource(value)
  return k8sLogSourceOptions.find(item => item.value === source)?.label || source
}

function normalizeLogType(value) {
  const raw = String(value || '').trim().toLowerCase()
  if (legacyNginxLogTypes.has(raw)) return 'nginx'
  if (raw === 'regex') return 'regex'
  const logType = raw
  return logTypeValueSet.has(logType) ? logType : defaultLogType
}

function shouldShowSampleLog(logType) {
  return sampleLogTypeSet.has(normalizeLogType(logType))
}

function hasNamedCaptureGroup(pattern) {
  return namedCaptureGroupRegExp.test(String(pattern || ''))
}

function logTypeText(value) {
  if (normalizeLogType(value) === 'regex') return '文本日志（历史正则解析）'
  return logTypeOptions.find(item => item.value === normalizeLogType(value))?.label || logTypeOptions[0].label
}

const taskLogType = task => task?.logType || task?.log_type || task?.logTypeSnapshot || task?.parseModeSnapshot || ''
const taskSampleLog = task => task?.sampleLog || task?.sample_log || ''
const taskRegexPattern = task => task?.regexPattern || task?.regex_pattern || ''

function defaultMultilinePattern(logType) {
  const normalized = normalizeLogType(logType)
  if (normalized === 'txt') return defaultTextMultilinePattern
  if (normalized === 'db_slow_log') return defaultSlowLogMultilinePattern
  return ''
}

function normalizeDefaultTextPattern(pattern) {
  const normalized = String(pattern || '').trim()
  return [legacyDefaultTextMultilinePattern, previousDefaultTextMultilinePattern].includes(normalized) ? defaultTextMultilinePattern : normalized
}

function taskMultilineEnabled(row, logType) {
  const configured = row?.multilineEnabledSnapshot ?? row?.multiline_enabled_snapshot
  if (configured !== undefined && configured !== null) return Boolean(configured)
  if (row?.multilinePatternSnapshot || row?.multiline_pattern_snapshot) return true
  return Boolean(defaultMultilinePattern(logType))
}

const taskDialog = reactive({
  visible: false,
  submitting: false,
  form: createTaskForm()
})
const taskLogTypeOptions = computed(() => {
  if (taskDialog.form.id && normalizeLogType(taskDialog.form.logType) === 'regex') {
    return [...logTypeOptions, { label: '文本日志（历史正则解析）', value: 'regex' }]
  }
  return logTypeOptions
})

const aiParse = reactive({
  loading: false,
  result: null,
  error: ''
})

const isTaskDialogK8s = computed(() => isK8sCollectType(taskDialog.form.collectType))
const isTaskDialogHost = computed(() => !isTaskDialogK8s.value)
const currentK8sLogSource = computed(() => normalizeK8sLogSource(taskDialog.form.k8sLogSource || taskDialog.form.k8sLogSources))
const isTaskDialogK8sNodeFile = computed(() => isTaskDialogK8s.value && currentK8sLogSource.value === 'node_file')
const showK8sWorkloadFields = computed(() => isTaskDialogK8s.value && k8sLogSourceNeedsWorkload(currentK8sLogSource.value))
const showPathPatternEditor = computed(() => (
  isTaskDialogHost.value ||
  (isTaskDialogK8s.value && k8sLogSourceRequiresPath(currentK8sLogSource.value))
))
const pathPatternLabel = computed(() => {
  if (!isTaskDialogK8s.value) return '日志路径'
  return isTaskDialogK8sNodeFile.value ? '宿主机路径' : '容器内路径'
})
const pathPatternPlaceholder = computed(() => {
  if (!isTaskDialogK8s.value) return '/var/log/messages'
  return isTaskDialogK8sNodeFile.value ? '/data/logs/tomcatlogs/**/*.log' : '/app/logs/*.log'
})
const pathPatternHint = computed(() => {
  if (!isTaskDialogK8s.value) return '支持精确路径、单层通配、递归通配；每行一个采集路径。'
  if (isTaskDialogK8sNodeFile.value) return 'K8S节点日志(hostPath)会通过集群级 Vector DaemonSet 挂载宿主机 /data/logs 采集，不修改业务工作负载。'
  return 'K8S容器日志(sidecar)会通过 Sidecar + 共享卷采集指定路径，安装或配置变更可能触发业务 Pod 滚动更新。'
})
const k8sNoPathHint = computed(() => 'K8S标准日志(stdout)会通过集群级 Vector DaemonSet 采集匹配容器的 stdout/stderr，不修改业务工作负载。')
const isK8sCollectorContentTab = computed(() => k8sCollectorTabNames.has(agentContentTab.value))
const k8sCollectorView = computed(() => agentContentTab.value === 'k8sCluster' ? 'cluster' : 'workload')
const targetAgentLabel = computed(() => taskDialog.form.id ? '执行Agent' : '可用Vector')
const targetAgentPlaceholder = computed(() => taskDialog.form.id ? '选择执行 Agent' : '选择可用 Vector 主机')
const targetAgentEmptyText = computed(() => taskDialog.form.id ? '暂无可选 Agent' : '暂无可用 Vector 主机')
const showSampleLogField = computed(() => shouldShowSampleLog(taskDialog.form.logType))
const showRegexPatternField = computed(() => normalizeLogType(taskDialog.form.logType) === 'regex')
const showAdvancedParseConfig = computed(() => showSampleLogField.value || showRegexPatternField.value)
const regexPatternPlaceholder = '^(?P<log_time>\\S+\\s+\\S+)\\s+(?P<level>\\S+)\\s+(?P<service>\\S+).*$'
const sampleLogPlaceholder = computed(() => {
  const logType = normalizeLogType(taskDialog.form.logType)
  if (logType === 'json') return '{"time":"2026-07-06T20:30:00+08:00","level":"INFO","message":"user login","traceId":"abc123"}'
  if (logType === 'nginx') return '127.0.0.1 - - [06/Jul/2026:20:30:00 +0800] "GET /api HTTP/1.1" 200 123 "-" "curl/8.0"'
  if (logType === 'nginx_json_loose') return '{"@timestamp":"2026-07-16 14:37:09","clientip":"192.168.214.76","request_method":"GET","request":"/zero-ai-api/v1/commands","status":200,"request_body":"{\\"user\\":\\"admin\\"}"}'
  if (logType === 'regex') return '2026-07-06 20:30:15 INFO order-service trace=abc123 user=1001 cost=88ms msg=pay success'
  return ''
})

const aiParseTemplate = computed(() => aiParse.result?.template || {})
const aiParseTemplateId = computed(() => aiParseTemplate.value?.id || aiParse.result?.templateId || '')
const aiParseModeText = computed(() => (
  aiParseTemplate.value?.parseMode ||
  aiParseTemplate.value?.parseModeSnapshot ||
  aiParseTemplate.value?.parse_mode ||
  aiParse.result?.parseMode ||
  normalizeLogType(taskDialog.form.logType) ||
  '-'
))
const aiParseFieldsText = computed(() => formatAiPreviewList(normalizeAiFields(
  aiParse.result?.fields ||
  aiParseTemplate.value?.fields ||
  aiParseTemplate.value?.fieldsJson
)))
const aiParseMappingsText = computed(() => formatAiPreviewList(normalizeAiMappings(
  aiParse.result?.fieldMappings ||
  aiParse.result?.field_mappings ||
  aiParseTemplate.value?.fieldMappings ||
  aiParseTemplate.value?.fieldsMappingJson ||
  aiParseTemplate.value?.fieldMappingsJson
)))
const aiParseWarningsText = computed(() => formatAiPreviewList(normalizeAiFields(aiParse.result?.warnings), 3))

const pathEditDialog = reactive({
  visible: false,
  submitting: false,
  task: null,
  form: {
    taskId: '',
    name: '',
    pathPattern: '',
    pathPatterns: [''],
    remark: ''
  }
})

const currentTask = computed(() => selectedNode.value?.type === 'task' ? selectedNode.value.task : null)

const packageDialog = reactive({
  visible: false
})

const packageUploadTask = reactive({
  taskId: '',
  status: '',
  progress: 0,
  uploadedBytes: 0,
  totalBytes: 0,
  message: '',
  error: '',
  package: null
})

const aggregatorDialog = reactive({
  visible: false
})

const errorDialog = reactive({
  visible: false,
  current: null,
  message: ''
})

const agentDrawer = reactive({
  visible: false,
  activeTab: 'basic',
  current: null
})

const drawerLoading = reactive({
  config: false,
  tasks: false,
  operations: false
})

function validatePathPatterns(rule, value, callback) {
  const collectType = normalizeCollectType(taskDialog.form.collectType)
  const k8sLogSource = normalizeK8sLogSource(taskDialog.form.k8sLogSource || taskDialog.form.k8sLogSources)
  if (collectType === 'kubernetes' && !k8sLogSourceRequiresPath(k8sLogSource)) {
    callback()
    return
  }
  if (splitPathPatterns(value).length) {
    callback()
    return
  }
  if (collectType === 'kubernetes') {
    callback(new Error(k8sLogSource === 'node_file' ? '请至少填写一个宿主机日志路径' : '请至少填写一个容器内日志路径'))
    return
  }
  callback(new Error('请至少填写一个日志路径'))
}

function validateK8sLogSource(rule, value, callback) {
  if (!isK8sCollectType(taskDialog.form.collectType)) {
    callback()
    return
  }
  if (resolveK8sLogSource(value || taskDialog.form.k8sLogSources)) {
    callback()
    return
  }
  callback(new Error('请选择采集来源'))
}

function validateTargetAgentIds(rule, value, callback) {
  if (isK8sCollectType(taskDialog.form.collectType)) {
    callback()
    return
  }
  if (Array.isArray(value) && value.length) {
    callback()
    return
  }
  callback(new Error('请选择健康 Vector 主机'))
}

function validateK8sCluster(rule, value, callback) {
  if (!isK8sCollectType(taskDialog.form.collectType)) {
    callback()
    return
  }
  if (!value) {
    callback(new Error('请选择 K8S 集群'))
    return
  }
  const selectedCluster = k8sClusters.value.find(cluster => sameId(cluster.id, value))
  if (!selectedCluster || isSelectableK8sCluster(selectedCluster)) {
    callback()
    return
  }
  callback(new Error('请先安装并等待集群级 Vector 运行中'))
}

function validateK8sNamespace(rule, value, callback) {
  if (!isK8sCollectType(taskDialog.form.collectType) || isTaskDialogK8sNodeFile.value || value) {
    callback()
    return
  }
  callback(new Error('请选择命名空间'))
}

function validateK8sWorkloadName(rule, value, callback) {
  if (!isK8sCollectType(taskDialog.form.collectType) || isTaskDialogK8sNodeFile.value || value) {
    callback()
    return
  }
  callback(new Error('请选择工作负载'))
}

function validateRegexPattern(rule, value, callback) {
  const logType = normalizeLogType(taskDialog.form.logType)
  const pattern = String(value || '').trim()
  if (logType === 'regex' && !pattern) {
    callback(new Error('请输入解析正则'))
    return
  }
  if (pattern && !hasNamedCaptureGroup(pattern)) {
    callback(new Error('正则必须包含命名捕获组，例如 (?P<status>\\d+)'))
    return
  }
  callback()
}

const taskRules = {
  projectId: [{ required: true, message: '请选择项目', trigger: 'change' }],
  storeId: [{ required: true, message: '请选择日志库', trigger: 'change' }],
  name: [{ required: true, message: '请输入任务名称', trigger: 'blur' }],
  collectType: [{ required: true, message: '请选择采集场景', trigger: 'change' }],
  logType: [{ required: true, message: '请选择日志类型', trigger: 'change' }],
  regexPattern: [{ validator: validateRegexPattern, trigger: ['blur', 'change'] }],
  targetAgentIds: [{ validator: validateTargetAgentIds, trigger: 'change' }],
  pathPatterns: [{ validator: validatePathPatterns, trigger: ['blur', 'change'] }],
  k8sLogSource: [{ validator: validateK8sLogSource, trigger: 'change' }],
  k8sClusterId: [{ validator: validateK8sCluster, trigger: 'change' }],
  k8sNamespace: [{ validator: validateK8sNamespace, trigger: 'change' }],
  k8sWorkloadName: [{ validator: validateK8sWorkloadName, trigger: 'change' }]
}

const pathEditRules = {
  pathPatterns: [{ validator: validatePathPatterns, trigger: ['blur', 'change'] }]
}

const packageReady = computed(() => Boolean(packageInfo.value?.exists) && !packageInfo.value?.error)

const packageStatusText = computed(() => {
  if (packageLoading.value) return '加载中'
  if (!packageInfo.value) return '未加载'
  return packageReady.value ? '可用' : packageInfo.value.error || '未找到'
})

const packageUploadProgress = computed(() => {
  const progress = Number(packageUploadTask.progress || 0)
  if (!Number.isFinite(progress)) return 0
  return Math.min(100, Math.max(0, Number(progress.toFixed(2))))
})

const packageUploadMessage = computed(() => (
  packageUploadTask.error ||
  packageUploadTask.message ||
  (packageUploading.value ? '正在上传 Vector 包' : '等待上传')
))

const packageUploadBytesText = computed(() => {
  const uploaded = formatBytes(packageUploadTask.uploadedBytes)
  const total = formatBytes(packageUploadTask.totalBytes)
  if (uploaded === '-' && total === '-') return ''
  return `${uploaded} / ${total}`
})

const packageUploadProgressStatus = computed(() => {
  const status = String(packageUploadTask.status || '').toLowerCase()
  if (['failed', 'error'].includes(status) || packageUploadTask.error) return 'exception'
  if (['completed', 'succeeded', 'success', 'done'].includes(status) || packageUploadProgress.value >= 100) return 'success'
  return undefined
})

const aggregatorVectorYaml = computed(() => String(aggregatorConfigPreview.value?.content || ''))

const aggregatorDockerCommand = computed(() => String(aggregatorConfigPreview.value?.docker?.command || ''))

const aggregatorKubernetesYaml = computed(() => String(aggregatorConfigPreview.value?.kubernetes?.yaml || ''))

const aggregatorKubernetesFileName = computed(() => String(aggregatorConfigPreview.value?.kubernetes?.fileName || 'aiops-vector-aggregator.yaml').trim())

const hasAggregatorVectorYaml = computed(() => Boolean(aggregatorVectorYaml.value.trim()))

const hasAggregatorDockerCommand = computed(() => Boolean(aggregatorDockerCommand.value.trim()))

const hasAggregatorKubernetesYaml = computed(() => Boolean(aggregatorKubernetesYaml.value.trim()))

const aggregatorConfigSummary = computed(() => {
  if (aggregatorConfigLoading.value) return '加载中'
  if (!aggregatorConfigPreview.value) return '未加载'
  return aggregatorConfigPreview.value?.image || '-'
})

const aggregatorDockerName = computed(() => aggregatorConfigPreview.value?.docker?.containerName || '-')

const aggregatorDockerConfigPath = computed(() => aggregatorConfigPreview.value?.docker?.configPath || '-')

const aggregatorDockerDataDir = computed(() => aggregatorConfigPreview.value?.docker?.dataDir || '-')

const aggregatorKubernetesNamespace = computed(() => aggregatorConfigPreview.value?.kubernetes?.namespace || '-')

const aggregatorKubernetesName = computed(() => aggregatorConfigPreview.value?.kubernetes?.deploymentName || '-')

const aggregatorKubernetesConfigMapName = computed(() => aggregatorConfigPreview.value?.kubernetes?.configMapName || '-')

const aggregatorKubernetesApplyCommand = computed(() => aggregatorConfigPreview.value?.kubernetes?.applyCommand || '-')

const configPreviewContent = computed(() => configPreview.value?.content || '')

const configPreviewTasks = computed(() => (
  Array.isArray(configPreview.value?.tasks) ? configPreview.value.tasks : []
))

const editableConfigTask = computed(() => {
  const previewTasks = configPreviewTasks.value
  if (!previewTasks.length) return null
  const currentTaskId = configTaskIdOf(currentTask.value)
  return previewTasks.find(task => sameId(configTaskIdOf(task), currentTaskId)) || previewTasks[0]
})

const filteredConfigTree = computed(() => {
  const keyword = treeKeyword.value.trim().toLowerCase()
  if (!keyword) return configTree.value

  return configTree.value.reduce((projectNodes, projectNode) => {
    const projectMatched = textIncludes(projectNode.project.name, keyword) || textIncludes(projectNode.project.code, keyword)
    const stores = projectNode.stores.reduce((storeNodes, storeNode) => {
      const storeMatched = textIncludes(storeNode.store.name, keyword) || textIncludes(storeNode.store.code, keyword)
      const matchedTasks = storeNode.tasks.filter(task => (
        projectMatched ||
        storeMatched ||
        textIncludes(task.name, keyword) ||
        textIncludes(task.code, keyword) ||
        textIncludes(normalizeCollectType(task.collectType), keyword) ||
        textIncludes(k8sTaskTargetText(task), keyword) ||
        textIncludes(taskPathPatterns(task).join('\n'), keyword)
      ))
      if (projectMatched || storeMatched || matchedTasks.length) {
        storeNodes.push({
          ...storeNode,
          tasks: projectMatched || storeMatched ? storeNode.tasks : matchedTasks
        })
      }
      return storeNodes
    }, [])

    if (projectMatched || stores.length) {
      projectNodes.push({
        ...projectNode,
        stores
      })
    }
    return projectNodes
  }, [])
})

const currentScopeTitle = computed(() => {
  const node = selectedNode.value
  if (!node) return '采集配置'
  if (node.type === 'project') return node.project?.name || '-'
  if (node.type === 'store') return node.store?.name || node.store?.code || '-'
  return node.task?.name || '-'
})

const scopeMetaItems = computed(() => {
  const node = selectedNode.value
  if (!node) return []
  if (node.type === 'task') {
    if (isK8sTask(node.task)) {
      return [
        { label: '类型', value: 'K8S 日志' },
        { label: '目标', value: k8sTaskTargetText(node.task) },
        { label: '采集来源', value: k8sLogSourceText(node.task?.k8sLogSource || node.task?.k8s_log_source) },
        { label: '日志类型', value: logTypeText(taskLogType(node.task)) },
        { label: '项目', value: projectNameOf(node.task) },
        { label: '日志库', value: storeNameOf(node.task) }
      ]
    }
    return [
      { label: '路径', value: taskPathText(node.task), mono: true },
      { label: '日志类型', value: logTypeText(taskLogType(node.task)) },
      { label: '项目', value: projectNameOf(node.task) },
      { label: '日志库', value: storeNameOf(node.task) },
      { label: '执行范围', value: executeModeText(node.task?.executeMode) }
    ]
  }
  if (node.type === 'store') {
    return [
      { label: '项目', value: node.project?.name || '-' },
      { label: '日志库', value: node.store?.name || node.store?.code || '-' },
      { label: '采集任务', value: `${node.tasks?.length || 0} 个` },
      { label: '执行范围', value: '日志库汇总' }
    ]
  }
  return [
    { label: '项目', value: node.project?.name || '-' },
    { label: '日志库', value: `${node.stores?.length || 0} 个` },
    { label: '采集任务', value: `${node.tasks?.length || 0} 个` },
    { label: '执行范围', value: '项目汇总' }
  ]
})

const selectedScopeTypeText = computed(() => {
  if (selectedNode.value?.type === 'project') return '项目范围'
  if (selectedNode.value?.type === 'store') return '日志库范围'
  return '采集任务'
})

const isProjectScopeSelected = computed(() => selectedNode.value?.type === 'project')

const agentEmptyText = computed(() => {
  if (agentContentTab.value === 'overview') return '暂无 Vector Agent'
  if (selectedNode.value?.type === 'task') return '当前任务未命中采集节点，请检查执行模式或执行目标配置'
  return '当前范围暂无命中采集节点，请检查采集任务配置'
})

const taskAgentTabLabel = computed(() => (
  selectedNode.value?.type === 'task' ? '主机命中Vector' : '范围命中节点'
))

const operationAgents = computed(() => statAgentRows.value.length ? statAgentRows.value : agents.value)

const taskAgentStats = computed(() => {
  const rows = operationAgents.value
  const installed = rows.filter(isVectorInstalled).length
  const running = rows.filter(item => normalizeVectorStatus(item.vectorStatus) === 'running').length
  const configApplied = rows.filter(item => normalizeConfigStatus(item.configStatus) === 'applied').length
  const configPending = rows.filter(item => normalizeConfigStatus(item.configStatus) === 'pending').length
  const configDisabled = rows.filter(item => normalizeConfigStatus(item.configStatus) === 'disabled').length
  const error = rows.filter(item => normalizeVectorStatus(item.vectorStatus) === 'error' || normalizeConfigStatus(item.configStatus) === 'failed').length
  return {
    total: agentPagination.total || rows.length,
    installed,
    running,
    configApplied,
    configPending,
    configDisabled,
    error
  }
})

const agentDrawerTitle = computed(() => (
  agentDrawer.current ? `Vector Agent - ${agentHostName(agentDrawer.current)}` : 'Vector Agent'
))

function executeTargetsJsonOf(row = {}) {
  return row.executeTargetsJson || row.execute_targets_json || ''
}

function parseTargetAgentIds(value) {
  return parseExecuteTargets(value).map(item => Number(item.agentId || item.id)).filter(Boolean)
}

function parseExecuteTargets(value) {
  if (Array.isArray(value)) return value
  const text = String(value || '').trim()
  if (!text) return []
  try {
    const parsed = JSON.parse(text)
    return Array.isArray(parsed) ? parsed : []
  } catch (error) {
    console.error('解析执行目标失败:', error)
    return []
  }
}

function parseK8sTargets(task = {}) {
  const targets = parseExecuteTargets(executeTargetsJsonOf(task))
  if (targets.length) return targets
  if (task.clusterId || task.k8sClusterId || task.namespace || task.k8sNamespace || task.workloadName || task.k8sWorkloadName) {
    return [{
      clusterId: task.clusterId || task.k8sClusterId,
      clusterName: task.clusterName || task.k8sClusterName,
      namespace: task.namespace || task.k8sNamespace,
      workloadType: task.workloadType || task.k8sWorkloadType,
      workloadName: task.workloadName || task.k8sWorkloadName
    }]
  }
  return []
}

function createAgentOptionFromTarget(target = {}) {
  const agentId = Number(target.agentId || target.id || 0)
  const hostId = Number(target.hostId || target.hostID || target.cmdbHostId || 0)
  if (!agentId) return null
  return {
    agentId,
    hostId,
    hostName: target.hostName || target.hostname || target.name || `Agent ${agentId}`,
    privateIp: target.privateIp || target.sshIp || target.publicIp || target.ip || (hostId ? `Host ${hostId}` : '-'),
    vectorStatus: target.vectorStatus || 'running',
    configStatus: target.configStatus || 'applied',
    agentOnlineStatus: Number(target.agentOnlineStatus ?? target.onlineStatus ?? 1)
  }
}

function ensureDialogAgentsContainTargets(task = {}) {
  const targets = parseExecuteTargets(executeTargetsJsonOf(task))
    .map(createAgentOptionFromTarget)
    .filter(Boolean)
  if (!targets.length) return

  const agentMap = new Map(dialogAgents.value.map(agent => [String(agentIdOf(agent)), agent]))
  targets.forEach(target => {
    const key = String(agentIdOf(target))
    const existing = agentMap.get(key)
    if (existing) {
      if (!agentHostIdOf(existing) && target.hostId) existing.hostId = target.hostId
      return
    }
    agentMap.set(key, target)
  })
  dialogAgents.value = Array.from(agentMap.values())
}

function firstK8sTarget(task = {}) {
  return parseK8sTargets(task)[0] || {}
}

function dialogStoreNameById(storeId, fallbackRow = {}) {
  const normalizedStoreId = String(storeId || '').trim()
  if (!normalizedStoreId) return ''
  const store = (
    dialogStores.value.find(item => String(item?.id || '') === normalizedStoreId) ||
    (String(selectedNode.value?.store?.id || '') === normalizedStoreId ? selectedNode.value?.store : null)
  )
  return fallbackRow?.storeName || fallbackRow?.store?.name || store?.name || ''
}

function createTaskForm(row = {}) {
  const pathPatterns = taskPathPatterns(row)
  const logType = normalizeLogType(taskLogType(row))
  const collectType = normalizeCollectType(row.collectType)
  const k8sTarget = firstK8sTarget(row)
  const executeTargetsJson = executeTargetsJsonOf(row)
  const rawK8sLogSource = row.k8sLogSource || row.k8s_log_source || (collectType === 'kubernetes' && pathPatterns.length ? 'node_file' : 'stdout')
  const k8sLogSource = normalizeK8sLogSource(rawK8sLogSource)
  const k8sLogSources = k8sLogSourceSelections(k8sLogSource)
  const storeId = row.storeId || taskQuery.storeId || ''
  const defaultTaskName = dialogStoreNameById(storeId, row)
  const multilineEnabledSnapshot = row.id ? taskMultilineEnabled(row, logType) : true
  return {
    id: row.id,
    projectId: row.projectId || taskQuery.projectId || '',
    storeId,
    name: row.name || defaultTaskName || '',
    collectType,
    logType,
    sampleLog: shouldShowSampleLog(logType) ? taskSampleLog(row) : '',
    regexPattern: logType === 'regex' ? taskRegexPattern(row) : '',
    multilineEnabledSnapshot,
    multilinePatternSnapshot: normalizeDefaultTextPattern(row.multilinePatternSnapshot || row.multiline_pattern_snapshot) || (multilineEnabledSnapshot ? defaultMultilinePattern(logType) : ''),
    parseTemplateId: row.parseTemplateId || row.parse_template_id || '',
    parseModeSnapshot: row.parseModeSnapshot || row.parse_mode_snapshot || logType,
    executeMode: collectType === 'kubernetes' ? (k8sLogSource === 'node_file' ? 'selected_clusters' : 'selected_workloads') : 'selected_agents',
    executeTargetsJson,
    targetAgentIds: parseTargetAgentIds(executeTargetsJson),
    k8sClusterId: k8sTarget.clusterId || row.k8sClusterId || row.clusterId || '',
    k8sClusterName: k8sTarget.clusterName || row.k8sClusterName || row.clusterName || '',
    k8sNamespace: k8sTarget.namespace || row.k8sNamespace || row.namespace || '',
    k8sWorkloadType: normalizeK8sWorkloadType(k8sTarget.workloadType || row.k8sWorkloadType || row.workloadType),
    k8sWorkloadName: k8sTarget.workloadName || row.k8sWorkloadName || row.workloadName || '',
    k8sLogSource,
    k8sLogSources,
    pathPattern: joinPathPatterns(pathPatterns),
    pathPatterns: pathPatterns.length ? pathPatterns : [''],
    status: Number(row.status || 1),
    remark: row.remark || '',
    createdBy: row.createdBy || 'admin',
    updatedBy: 'admin'
  }
}

const extractData = response => response?.data?.data ?? response?.data ?? response

const responsePayload = response => (
  response?.data && typeof response.data === 'object' ? response.data : response
)

const isBusinessSuccess = response => {
  const payload = responsePayload(response)
  return payload?.code === undefined || Number(payload.code) === 200
}

const responseMessage = response => {
  const payload = responsePayload(response)
  return payload?.message || ''
}

const parseJsonIfPossible = value => {
  if (typeof value !== 'string') return value
  const text = value.trim()
  if (!text) return ''
  try {
    return JSON.parse(text)
  } catch {
    return value
  }
}

const normalizeAiFields = value => {
  const parsed = parseJsonIfPossible(value)
  if (Array.isArray(parsed)) {
    return parsed
      .map(item => {
        if (typeof item === 'string') return item
        return item?.name || item?.field || item?.key || item?.source || ''
      })
      .filter(Boolean)
  }
  if (parsed && typeof parsed === 'object') return Object.keys(parsed)
  if (typeof parsed === 'string') return parsed.split(/[,，\n]/).map(item => item.trim()).filter(Boolean)
  return []
}

const normalizeAiMappings = value => {
  const parsed = parseJsonIfPossible(value)
  if (Array.isArray(parsed)) {
    return parsed
      .map(item => {
        if (typeof item === 'string') return item
        const source = item?.source || item?.from || item?.field || item?.name || item?.src || ''
        const target = item?.target || item?.to || item?.mapped || item?.mapping || item?.dst || ''
        return source && target ? `${source} -> ${target}` : source || target
      })
      .filter(Boolean)
  }
  if (parsed && typeof parsed === 'object') {
    return Object.entries(parsed)
      .map(([source, target]) => `${source} -> ${typeof target === 'string' ? target : JSON.stringify(target)}`)
      .filter(Boolean)
  }
  if (typeof parsed === 'string') return parsed.split(/[,，\n]/).map(item => item.trim()).filter(Boolean)
  return []
}

const formatAiPreviewList = (items, limit = 10) => {
  const values = Array.from(new Set((items || []).map(item => String(item || '').trim()).filter(Boolean)))
  if (!values.length) return ''
  const head = values.slice(0, limit).join(', ')
  return values.length > limit ? `${head} ...` : head
}

const extractList = response => {
  const data = extractData(response)
  if (Array.isArray(data)) return data
  const candidate = [
    data?.list,
    data?.items,
    data?.agents,
    data?.rows,
    data?.records,
    data?.values,
    data?.namespaces,
    data?.workloads,
    data?.pods,
    data?.containers,
    data?.clusterCollectors,
    data?.cluster_collectors,
    data?.collectors
  ].find(Array.isArray)
  if (candidate) return candidate
  const groupedWorkloads = data?.workloads
  if (groupedWorkloads && typeof groupedWorkloads === 'object') {
    return Object.values(groupedWorkloads).flatMap(value => Array.isArray(value) ? value : [])
  }
  return []
}

const extractTotal = response => {
  const data = extractData(response)
  if (typeof data?.total === 'number') return data.total
  if (typeof data?.count === 'number') return data.count
  if (typeof data?.agentCount === 'number') return data.agentCount
  if (typeof data?.matchedAgentCount === 'number') return data.matchedAgentCount
  if (typeof data?.totalAgents === 'number') return data.totalAgents
  return extractList(response).length
}

function splitPathPatterns(value) {
  if (Array.isArray(value)) {
    const seen = new Set()
    return value
      .flatMap(item => splitPathPatterns(item))
      .filter(Boolean)
      .filter(item => {
        if (seen.has(item)) return false
        seen.add(item)
        return true
      })
  }
  const text = String(value || '').trim()
  if (!text) return []
  const seen = new Set()
  return text
    .split(/[\n,;]+/)
    .map(item => item.trim())
    .filter(Boolean)
    .filter(item => {
      if (seen.has(item)) return false
      seen.add(item)
      return true
    })
}

function parsePathPatternsJson(value) {
  if (!value) return []
  if (Array.isArray(value)) return splitPathPatterns(value)
  try {
    return splitPathPatterns(JSON.parse(value))
  } catch (error) {
    return []
  }
}

function taskPathPatterns(task) {
  const structured = splitPathPatterns(task?.pathPatterns)
  if (structured.length) return structured
  const jsonPatterns = parsePathPatternsJson(task?.pathPatternsJson)
  if (jsonPatterns.length) return jsonPatterns
  return splitPathPatterns(task?.pathPattern)
}

function joinPathPatterns(patterns) {
  return splitPathPatterns(patterns).join('\n')
}

function ensurePathPatternRows(form) {
  if (!Array.isArray(form.pathPatterns)) form.pathPatterns = []
  if (!form.pathPatterns.length) form.pathPatterns.push('')
}

function addPathPatternRow(form) {
  ensurePathPatternRows(form)
  form.pathPatterns.push('')
}

function removePathPatternRow(form, index) {
  ensurePathPatternRows(form)
  if (form.pathPatterns.length <= 1) return
  form.pathPatterns.splice(index, 1)
  ensurePathPatternRows(form)
}

const findCmdbAgentByIp = (items, ip) => (
  items.find(item => [item?.sshIp, item?.privateIp, item?.publicIp, item?.ip].some(value => String(value || '') === String(ip))) || items[0] || null
)

const fetchCmdbAgentByIp = async (ip, options = {}) => {
  if (!ip || ip === '-') return null
  if (!options.force && cmdbAgentCache.has(ip)) return cmdbAgentCache.get(ip)

  const response = await cmdbAPI.queryAgent({ page: 1, pageSize: 10, ip })
  const cmdbAgent = findCmdbAgentByIp(extractList(response), ip)
  cmdbAgentCache.set(ip, cmdbAgent)
  return cmdbAgent
}

const hydrateCmdbAgents = async (rows, options = {}) => {
  const ipList = Array.from(new Set(rows.map(agentIp).filter(ip => ip && ip !== '-')))
  if (!ipList.length) return

  const results = await Promise.allSettled(ipList.map(ip => fetchCmdbAgentByIp(ip, options)))
  results.forEach((result, index) => {
    if (result.status === 'rejected') {
      console.error(`加载 CMDB Agent 状态失败(${ipList[index]}):`, result.reason)
    }
  })

  rows.forEach(row => {
    row.cmdbAgent = cmdbAgentCache.get(agentIp(row)) || null
  })
}

const textIncludes = (value, keyword) => String(value || '').toLowerCase().includes(keyword)
const sameId = (left, right) => String(left ?? '') === String(right ?? '')
const configTaskIdOf = task => task?.id || task?.taskId || task?.collectTaskId
const taskNodeKey = task => {
  const taskId = configTaskIdOf(task)
  return taskId ? `task-${taskId}` : ''
}
const isK8sTask = task => isK8sCollectType(task?.collectType)
const canStopCollectTask = task => Boolean(configTaskIdOf(task)) && !isK8sTask(task) && Number(task?.status || 1) !== 2
const k8sTaskTargetText = task => {
  const target = firstK8sTarget(task)
  const cluster = target.clusterName || task?.k8sClusterName || task?.clusterName || (target.clusterId ? `集群 ${target.clusterId}` : '未选集群')
  const source = normalizeK8sLogSource(task?.k8sLogSource || task?.k8s_log_source)
  if (source === 'node_file') return `${cluster} / K8S节点日志(hostPath)`
  const namespace = target.namespace || task?.k8sNamespace || task?.namespace || '-'
  const workloadType = k8sWorkloadTypeText(target.workloadType || task?.k8sWorkloadType || task?.workloadType)
  const workloadName = target.workloadName || task?.k8sWorkloadName || task?.workloadName || '-'
  return `${cluster} / ${namespace} / ${workloadType} ${workloadName}`
}
const taskPathText = task => {
  if (isK8sTask(task)) {
    const source = normalizeK8sLogSource(task?.k8sLogSource || task?.k8s_log_source)
    if (source === 'stdout') return 'K8S标准日志(stdout)'
    if (source === 'node_file') return taskPathPatterns(task).length ? taskPathPatterns(task).join('、') : 'K8S节点日志(hostPath)'
  }
  const patterns = taskPathPatterns(task)
  if (!patterns.length) return '-'
  return patterns.length === 1 ? patterns[0] : `${patterns[0]} 等 ${patterns.length} 条`
}

const enrichTask = (task, project, store) => ({
  ...task,
  projectName: task.projectName || project?.name || task.projectCode || '',
  storeName: task.storeName || store?.name || task.storeCode || '',
  storeCode: task.storeCode || store?.code || task.storeCode || ''
})

const buildProjectNode = (project, stores, projectTasks) => {
  const storeNodes = stores.map(store => {
    const storeTasks = projectTasks
      .filter(task => sameId(task.storeId, store.id))
      .map(task => enrichTask(task, project, store))
    return {
      key: `store-${store.id}`,
      type: 'store',
      project,
      store,
      tasks: storeTasks
    }
  })

  const assignedTaskIds = new Set(storeNodes.flatMap(storeNode => storeNode.tasks.map(task => String(configTaskIdOf(task)))))
  const orphanTasks = projectTasks
    .filter(task => !assignedTaskIds.has(String(configTaskIdOf(task))))
    .map(task => enrichTask(task, project, null))

  if (orphanTasks.length) {
    storeNodes.push({
      key: `store-orphan-${project.id}`,
      type: 'store',
      project,
      store: { id: `orphan-${project.id}`, name: '未归属日志库', code: '-' },
      tasks: orphanTasks
    })
  }

  const projectAllTasks = storeNodes.flatMap(storeNode => storeNode.tasks)
  return {
    key: `project-${project.id}`,
    type: 'project',
    project,
    stores: storeNodes,
    tasks: projectAllTasks
  }
}

const getNodeTasks = node => {
  if (!node) return []
  if (node.type === 'task') return node.task ? [node.task] : []
  return node.tasks || []
}

const isK8sNode = node => node?.type === 'task' && isK8sTask(node.task)

const findNodeByKey = (nodes, key) => {
  if (!key) return null
  for (const projectNode of nodes) {
    if (projectNode.key === key) return projectNode
    for (const storeNode of projectNode.stores) {
      if (storeNode.key === key) return storeNode
      const task = storeNode.tasks.find(item => taskNodeKey(item) === key)
      if (task) {
        return {
          key: taskNodeKey(task),
          type: 'task',
          project: projectNode.project,
          store: storeNode.store,
          task
        }
      }
    }
  }
  return null
}

const firstTaskProjectNode = nodes => nodes.find(projectNode => projectNode.tasks?.length) || nodes[0] || null

const setSelectedNode = node => {
  selectedNode.value = node
  selectedTaskId.value = node?.type === 'task' ? configTaskIdOf(node.task) : null
  taskQuery.projectId = node?.project?.id || ''
  taskQuery.storeId = node?.type === 'store' || node?.type === 'task' ? node.store?.id || '' : ''
}

const projectKeyOfNode = node => {
  if (!node) return ''
  if (node.type === 'project') return node.key
  return node.project?.id ? `project-${node.project.id}` : ''
}

const storeKeyOfNode = node => {
  if (!node) return ''
  if (node.type === 'store') return node.key
  if (node.type === 'task' && node.store?.id) return `store-${node.store.id}`
  return ''
}

const syncExpandedProject = node => {
  const projectKey = projectKeyOfNode(node)
  if (projectKey) expandedProjectKey.value = projectKey
  const storeKey = storeKeyOfNode(node)
  if (storeKey) setStoreExpanded(storeKey, true)
}

const clearConfigSelection = () => {
  selectedNode.value = null
  selectedTaskId.value = null
  agents.value = []
  scopeAgentRows.value = []
  statAgentRows.value = []
  agentPagination.total = 0
}

const isActiveNode = key => selectedNode.value?.key === key
const isActiveTask = task => selectedNode.value?.type === 'task' && sameId(configTaskIdOf(selectedNode.value.task), configTaskIdOf(task))
const isProjectExpanded = key => Boolean(treeKeyword.value.trim()) || expandedProjectKey.value === key
const isStoreExpanded = key => Boolean(treeKeyword.value.trim()) || expandedStoreKeys.value.includes(key)

const setStoreExpanded = (key, expanded) => {
  if (!key) return
  const currentKeys = expandedStoreKeys.value
  const exists = currentKeys.includes(key)
  if (expanded && !exists) {
    expandedStoreKeys.value = [...currentKeys, key]
    return
  }
  if (!expanded && exists) {
    expandedStoreKeys.value = currentKeys.filter(item => item !== key)
  }
}

const loadStoresByProject = async (projectId, targetRef) => {
  if (!projectId) {
    targetRef.value = []
    return
  }
  try {
    const response = await logtailApi.getStores({ page: 1, pageSize: 100, projectId })
    targetRef.value = extractList(response)
  } catch (error) {
    console.error('加载日志库失败:', error)
    targetRef.value = []
  }
}

const loadDialogAgents = async (taskId = null, mode = 'available') => {
  dialogAgentMode.value = mode
  dialogAgentLoading.value = true
  try {
    const response = mode === 'task' && taskId
      ? await logtailApi.getCollectTaskAgents(taskId, { page: 1, pageSize: 1000 })
      : await logtailApi.getVectorAgents({ page: 1, pageSize: 1000, onlineStatus: 1 })
    dialogAgents.value = extractList(response).filter(isSelectableDialogAgent)
  } catch (error) {
    console.error(mode === 'task' ? '加载执行 Agent 失败:' : '加载可用 Vector 主机失败:', error)
    dialogAgents.value = []
  } finally {
    dialogAgentLoading.value = false
  }
}

const clusterNameOf = cluster => cluster?.name || cluster?.clusterName || cluster?.displayName || `Cluster ${cluster?.id || ''}`

const k8sClusterCollectorId = row => row?.clusterId || row?.k8sClusterId || row?.clusterID || row?.cluster?.id || row?.id

const normalizeK8sClusterCollectorStatus = value => {
  const status = String(value || '').trim().toLowerCase()
  if (!status) return 'not_installed'
  if (['ready', 'healthy', 'installed'].includes(status)) return 'running'
  if (['installing', 'updating', 'reinstalling', 'uninstalling'].includes(status)) return 'pending'
  if (['notinstalled', 'not_install', 'none', 'uninstalled'].includes(status)) return 'not_installed'
  return status
}

const k8sClusterCollectorReplicasReady = row => {
  const desired = Number(row?.desired ?? row?.desiredReplicas ?? row?.replicas ?? 0)
  const ready = Number(row?.ready ?? row?.readyReplicas ?? 0)
  const available = Number(row?.available ?? row?.availableReplicas ?? 0)
  return desired > 0 && ready >= desired && available > 0
}

const k8sClusterCollectorStatusOf = row => {
  const currentStatus = row?.vectorStatus || row?.status || row?.collectorStatus
  if (currentStatus) return normalizeK8sClusterCollectorStatus(currentStatus)
  if (k8sClusterCollectorReplicasReady(row)) return 'running'
  return normalizeK8sClusterCollectorStatus(
    row?.runtimeStatus ||
    row?.lastOperation?.runtimeStatus ||
    row?.lastOperation?.status ||
    row?.lastOperation?.opStatus
  )
}

const k8sClusterVectorStatusOf = cluster => normalizeK8sClusterCollectorStatus(
  cluster?.clusterCollectorStatus ||
  cluster?.collectorStatus ||
  cluster?.vectorStatus ||
  cluster?.runtimeStatus
)

const isSelectableK8sCluster = cluster => k8sClusterVectorStatusOf(cluster) === 'running'

const k8sClusterOptionTip = cluster => {
  const status = k8sClusterVectorStatusOf(cluster)
  if (status === 'running') return '集群级 Vector：运行中'
  if (status === 'pending') return '集群级 Vector 安装/更新中'
  if (status === 'failed') return '集群级 Vector 异常，请先处理'
  return '请先安装集群级 Vector'
}

const normalizeK8sNamespace = namespace => {
  if (typeof namespace === 'string') return { name: namespace }
  return {
    ...namespace,
    name: namespace?.name || namespace?.namespace || namespace?.metadata?.name || ''
  }
}

const normalizeK8sWorkload = workload => {
  if (typeof workload === 'string') {
    return {
      name: workload,
      type: normalizeK8sWorkloadType(taskDialog.form.k8sWorkloadType)
    }
  }
  return {
    ...workload,
    name: workload?.name || workload?.workloadName || workload?.metadata?.name || '',
    type: normalizeK8sWorkloadType(workload?.type || workload?.kind || workload?.workloadType)
  }
}

const getK8sWorkloadTotalPages = (response, pageSize, rowCount, currentPage) => {
  const data = extractData(response)
  const explicitTotalPages = Number(data?.totalPages ?? data?.total_pages ?? data?.pages)
  if (Number.isFinite(explicitTotalPages) && explicitTotalPages > 0) return explicitTotalPages

  const total = Number(data?.total ?? data?.count ?? data?.totalCount ?? data?.total_count)
  if (Number.isFinite(total) && total > 0) return Math.ceil(total / pageSize)

  return rowCount >= pageSize ? currentPage + 1 : currentPage
}

const normalizeK8sWorkloadOptions = (rows, expectedType) => {
  const seen = new Set()
  return rows
    .map(normalizeK8sWorkload)
    .filter(workload => workload.name && normalizeK8sWorkloadType(workload.type) === expectedType)
    .filter(workload => {
      const key = `${normalizeK8sWorkloadType(workload.type)}:${workload.name}`
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
}

const fetchK8sWorkloadPage = async (page, keyword, expectedType) => {
  const params = {
    page,
    pageSize: K8S_WORKLOAD_PAGE_SIZE,
    type: k8sWorkloadTypeForApi(expectedType)
  }

  if (keyword) {
    return k8sApi.searchWorkloadList(taskDialog.form.k8sClusterId, taskDialog.form.k8sNamespace, {
      ...params,
      keyword
    })
  }

  return k8sApi.getWorkloadList(taskDialog.form.k8sClusterId, taskDialog.form.k8sNamespace, params)
}

const loadK8sFormClusters = async (force = false) => {
  if (k8sLoading.clusters) return
  if (k8sClusters.value.length && !force) return
  k8sLoading.clusters = true
  try {
    const [clusterResponse, collectorResponse] = await Promise.all([
      k8sApi.getClusterList({ page: 1, size: 100, pageSize: 100 }),
      logtailApi.getK8sClusterCollectors({ page: 1, pageSize: 200 }).catch(error => {
        console.warn('加载集群级 Vector 状态失败:', error)
        return null
      })
    ])
    const collectorRows = extractList(collectorResponse)
    const collectorStatusMap = new Map(collectorRows.map(row => [
      String(k8sClusterCollectorId(row)),
      k8sClusterCollectorStatusOf(row)
    ]))
    k8sClusters.value = extractList(clusterResponse)
      .filter(cluster => cluster?.id !== undefined && cluster?.id !== null)
      .map(cluster => ({
        ...cluster,
        clusterCollectorStatus: collectorStatusMap.get(String(cluster.id)) || 'not_installed'
      }))
    if (taskDialog.form.k8sClusterId) {
      const selectedCluster = k8sClusters.value.find(cluster => sameId(cluster.id, taskDialog.form.k8sClusterId))
      if (selectedCluster) taskDialog.form.k8sClusterName = clusterNameOf(selectedCluster)
      if (selectedCluster && !isSelectableK8sCluster(selectedCluster)) {
        taskDialog.form.k8sClusterId = ''
        taskDialog.form.k8sClusterName = ''
        taskDialog.form.k8sNamespace = ''
        taskDialog.form.k8sWorkloadName = ''
        ElMessage.warning('请选择已安装并运行中的集群级 Vector')
      }
    }
  } catch (error) {
    console.error('加载 K8S 集群失败:', error)
    k8sClusters.value = []
  } finally {
    k8sLoading.clusters = false
  }
}

const loadK8sFormNamespaces = async (force = false) => {
  if (!taskDialog.form.k8sClusterId) {
    k8sNamespaces.value = []
    return
  }
  if (k8sLoading.namespaces) return
  if (k8sNamespaces.value.length && !force) return
  k8sLoading.namespaces = true
  try {
    const response = await k8sApi.getNamespaces(taskDialog.form.k8sClusterId)
    k8sNamespaces.value = extractList(response).map(normalizeK8sNamespace).filter(namespace => namespace.name)
    if (taskDialog.form.k8sNamespace && !k8sNamespaces.value.some(namespace => namespace.name === taskDialog.form.k8sNamespace)) {
      taskDialog.form.k8sNamespace = ''
    }
  } catch (error) {
    console.error('加载 K8S 命名空间失败:', error)
    k8sNamespaces.value = []
  } finally {
    k8sLoading.namespaces = false
  }
}

const loadK8sFormWorkloads = async (force = false, keyword = '') => {
  const normalizedKeyword = String(keyword || '').trim()
  k8sWorkloadSearchKeyword.value = normalizedKeyword

  if (!taskDialog.form.k8sClusterId || !taskDialog.form.k8sNamespace) {
    k8sWorkloads.value = []
    return
  }

  if (k8sWorkloads.value.length && !force && !normalizedKeyword) return

  const requestSeq = ++k8sWorkloadRequestSeq
  k8sLoading.workloads = true
  try {
    const expectedType = normalizeK8sWorkloadType(taskDialog.form.k8sWorkloadType)
    const workloadRows = []
    let page = 1
    let totalPages = 1

    do {
      const response = await fetchK8sWorkloadPage(page, normalizedKeyword, expectedType)
      if (requestSeq !== k8sWorkloadRequestSeq) return

      const rows = extractList(response)
      workloadRows.push(...rows)
      totalPages = getK8sWorkloadTotalPages(response, K8S_WORKLOAD_PAGE_SIZE, rows.length, page)
      if (rows.length < K8S_WORKLOAD_PAGE_SIZE && page >= totalPages) break
      page += 1
    } while (page <= Math.min(totalPages, K8S_WORKLOAD_MAX_PAGES))

    k8sWorkloads.value = normalizeK8sWorkloadOptions(workloadRows, expectedType)
    if (!normalizedKeyword && taskDialog.form.k8sWorkloadName && !k8sWorkloads.value.some(workload => workload.name === taskDialog.form.k8sWorkloadName)) {
      taskDialog.form.k8sWorkloadName = ''
    }
  } catch (error) {
    console.error('加载 K8S 工作负载失败:', error)
    if (requestSeq === k8sWorkloadRequestSeq) k8sWorkloads.value = []
  } finally {
    if (requestSeq === k8sWorkloadRequestSeq) k8sLoading.workloads = false
  }
}

const loadK8sTaskOptions = async () => {
  await loadK8sFormClusters(true)
  if (taskDialog.form.k8sClusterId) await loadK8sFormNamespaces(true)
  if (taskDialog.form.k8sWorkloadName) {
    k8sWorkloads.value = normalizeK8sWorkloadOptions([{
      name: taskDialog.form.k8sWorkloadName,
      type: taskDialog.form.k8sWorkloadType,
      namespace: taskDialog.form.k8sNamespace
    }], normalizeK8sWorkloadType(taskDialog.form.k8sWorkloadType))
  }
}

const handleK8sWorkloadVisibleChange = visible => {
  if (visible) loadK8sFormWorkloads(false, k8sWorkloadSearchKeyword.value)
}

const searchK8sFormWorkloads = keyword => {
  loadK8sFormWorkloads(true, keyword)
}

const handleK8sClusterChange = async clusterId => {
  const selectedCluster = k8sClusters.value.find(cluster => sameId(cluster.id, clusterId))
  if (selectedCluster && !isSelectableK8sCluster(selectedCluster)) {
    ElMessage.warning('请先安装并等待集群级 Vector 运行中')
    taskDialog.form.k8sClusterId = ''
    taskDialog.form.k8sClusterName = ''
    return
  }
  taskDialog.form.k8sClusterName = selectedCluster ? clusterNameOf(selectedCluster) : ''
  taskDialog.form.k8sNamespace = ''
  taskDialog.form.k8sWorkloadName = ''
  k8sWorkloadSearchKeyword.value = ''
  k8sNamespaces.value = []
  k8sWorkloads.value = []
  await loadK8sFormNamespaces(true)
}

const handleK8sNamespaceChange = () => {
  taskDialog.form.k8sWorkloadName = ''
  k8sWorkloadSearchKeyword.value = ''
  k8sWorkloads.value = []
}

const handleK8sWorkloadTypeChange = () => {
  taskDialog.form.k8sWorkloadName = ''
  k8sWorkloadSearchKeyword.value = ''
  k8sWorkloads.value = []
}

const handleK8sWorkloadChange = workloadName => {
  const selectedWorkload = k8sWorkloads.value.find(workload => workload.name === workloadName)
  if (selectedWorkload?.type) taskDialog.form.k8sWorkloadType = selectedWorkload.type
}

const handleK8sLogSourceChange = () => {
  taskDialog.form.k8sLogSource = normalizeK8sLogSource(taskDialog.form.k8sLogSource || taskDialog.form.k8sLogSources)
  taskDialog.form.k8sLogSources = k8sLogSourceSelections(taskDialog.form.k8sLogSource)
  taskFormRef.value?.clearValidate(['k8sLogSource'])
  taskDialog.form.executeMode = taskDialog.form.k8sLogSource === 'node_file' ? 'selected_clusters' : 'selected_workloads'
  if (taskDialog.form.k8sLogSource === 'node_file') {
    taskDialog.form.k8sNamespace = ''
    taskDialog.form.k8sWorkloadName = ''
    k8sWorkloads.value = []
  }
  if (!k8sLogSourceRequiresPath(taskDialog.form.k8sLogSource)) {
    taskFormRef.value?.clearValidate(['pathPatterns'])
  }
}

const loadConfigTree = async (preferredKey = '') => {
  taskLoading.value = true
  try {
    const projectResponse = await logtailApi.getProjects({ page: 1, pageSize: 200 })
    projects.value = extractList(projectResponse)

    const projectNodes = await Promise.all(projects.value.map(async project => {
      const [storeResponse, taskResponse] = await Promise.all([
        logtailApi.getStores({ page: 1, pageSize: 500, projectId: project.id }),
        logtailApi.getCollectTasks({ page: 1, pageSize: 1000, projectId: project.id })
      ])
      return buildProjectNode(project, extractList(storeResponse), extractList(taskResponse))
    }))

    configTree.value = projectNodes
    const validStoreKeys = new Set(projectNodes.flatMap(projectNode => projectNode.stores.map(storeNode => storeNode.key)))
    expandedStoreKeys.value = expandedStoreKeys.value.filter(key => validStoreKeys.has(key))
    tasks.value = projectNodes.flatMap(projectNode => projectNode.tasks)
    taskPagination.total = tasks.value.length

    if (!tasks.value.length && !projectNodes.length) {
      expandedProjectKey.value = ''
      expandedStoreKeys.value = []
      clearConfigSelection()
      return
    }

    const preferredNode = findNodeByKey(projectNodes, preferredKey)
    const nextNode = preferredNode || firstTaskProjectNode(projectNodes)
    if (!nextNode) {
      expandedProjectKey.value = ''
      expandedStoreKeys.value = []
      clearConfigSelection()
      return
    }

    if (!preferredNode && nextNode.type === 'project') {
      expandedStoreKeys.value = []
    }
    agentContentTab.value = isK8sNode(nextNode) ? 'k8s' : 'task'
    syncExpandedProject(nextNode)
    setSelectedNode(nextNode)
    agentPagination.page = 1
    if (isK8sCollectorContentTab.value) await loadK8sFormClusters()
    await loadSelectedNodeAgents()
  } catch (error) {
    console.error('加载采集配置失败:', error)
    configTree.value = []
    tasks.value = []
    taskPagination.total = 0
    expandedProjectKey.value = ''
    expandedStoreKeys.value = []
    clearConfigSelection()
  } finally {
    taskLoading.value = false
  }
}

const loadVectorPackage = async () => {
  packageLoading.value = true
  try {
    const response = await logtailApi.getVectorPackage()
    packageInfo.value = extractData(response) || null
  } catch (error) {
    console.error('加载 Vector 包失败:', error)
    packageInfo.value = { exists: false, error: '获取 Vector 包失败' }
  } finally {
    packageLoading.value = false
  }
}

const chooseVectorPackageFile = () => {
  packageFileInputRef.value?.click()
}

const resetPackageUploadTask = () => {
  Object.assign(packageUploadTask, {
    taskId: '',
    status: '',
    progress: 0,
    uploadedBytes: 0,
    totalBytes: 0,
    message: '',
    error: '',
    package: null
  })
}

const stopPackageUploadPolling = () => {
  if (packageUploadPollTimer) {
    clearTimeout(packageUploadPollTimer)
    packageUploadPollTimer = null
  }
}

const applyPackageUploadTask = task => {
  if (!task) return
  packageUploadTask.taskId = task.taskId || packageUploadTask.taskId
  packageUploadTask.status = task.status || packageUploadTask.status
  packageUploadTask.progress = Number(task.progress ?? packageUploadTask.progress ?? 0)
  packageUploadTask.uploadedBytes = Number(task.uploadedBytes ?? packageUploadTask.uploadedBytes ?? 0)
  packageUploadTask.totalBytes = Number(task.totalBytes ?? packageUploadTask.totalBytes ?? 0)
  packageUploadTask.message = task.message || packageUploadTask.message
  packageUploadTask.error = task.error || packageUploadTask.error
  packageUploadTask.package = task.package || packageUploadTask.package
}

const packageUploadTaskOf = response => extractData(response)?.task || responsePayload(response)?.task || null
const isPackageUploadCompleted = task => ['completed', 'succeeded', 'success', 'done'].includes(String(task?.status || '').toLowerCase())
const isPackageUploadFailed = task => ['failed', 'error'].includes(String(task?.status || '').toLowerCase()) || Boolean(task?.error)

const waitVectorPackageUploadTask = taskId => new Promise((resolve, reject) => {
  const poll = async () => {
    try {
      const response = await logtailApi.getVectorPackageUploadTask(taskId)
      const task = packageUploadTaskOf(response)
      applyPackageUploadTask(task)

      if (isPackageUploadFailed(task)) {
        reject(new Error(task?.error || task?.message || 'Vector 包上传失败'))
        return
      }

      if (isPackageUploadCompleted(task)) {
        resolve(task)
        return
      }

      packageUploadPollTimer = setTimeout(poll, 1000)
    } catch (error) {
      reject(error)
    }
  }

  poll()
})

const handlePackageFileChange = async event => {
  const file = event.target.files?.[0]
  event.target.value = ''
  if (!file) return

  const formData = new FormData()
  formData.append('file', file)
  formData.append('replace', 'true')
  stopPackageUploadPolling()
  resetPackageUploadTask()
  packageUploading.value = true
  try {
    const response = await logtailApi.uploadVectorPackage(formData, { replace: true })
    const task = packageUploadTaskOf(response)
    applyPackageUploadTask(task)

    if (task?.taskId) {
      const completedTask = await waitVectorPackageUploadTask(task.taskId)
      if (completedTask?.package) packageInfo.value = completedTask.package
    }

    ElMessage.success('Vector 包上传成功')
    await loadVectorPackage()
  } catch (error) {
    packageUploadTask.error = error?.message || 'Vector 包上传失败'
    console.error('上传 Vector 包失败:', error)
    ElMessage.error(packageUploadTask.error)
  } finally {
    stopPackageUploadPolling()
    packageUploading.value = false
  }
}

const removeVectorPackage = async () => {
  await ElMessageBox.confirm('确认删除当前 Vector 包吗？删除后将无法安装 Vector。', '删除 Vector 包', {
    type: 'warning',
    confirmButtonText: '删除',
    cancelButtonText: '取消'
  })
  packageDeleting.value = true
  try {
    await logtailApi.deleteVectorPackage()
    ElMessage.success('Vector 包已删除')
    await loadVectorPackage()
  } finally {
    packageDeleting.value = false
  }
}

const loadTaskAgents = async (options = {}) => {
  const { silent = false, hydrateCmdb = true } = options
  const taskId = configTaskIdOf(currentTask.value)
  if (!taskId) {
    agents.value = []
    statAgentRows.value = []
    agentPagination.total = 0
    return
  }
  if (!silent) agentLoading.value = true
  try {
    const response = await logtailApi.getCollectTaskAgents(taskId, {
      page: agentPagination.page,
      pageSize: agentPagination.pageSize,
      keyword: agentQuery.keyword || undefined,
      onlineStatus: agentQuery.onlineStatus || undefined,
      vectorStatus: agentQuery.vectorStatus || undefined,
      configStatus: agentQuery.configStatus || undefined
    })
    const nextAgents = extractList(response)
    agents.value = silent ? mergeExistingAgentRows(agents.value, nextAgents) : nextAgents
    if (hydrateCmdb) await hydrateCmdbAgents(agents.value, { force: true })
    agents.value.forEach(attachCachedOperationError)
    statAgentRows.value = agents.value
    agentPagination.total = extractTotal(response)
    syncOperationPollingState(agents.value)
    syncCmdbAgentPollingState(agents.value)
  } catch (error) {
    console.error('加载 Agent 失败:', error)
    if (!silent) {
      agents.value = []
      statAgentRows.value = []
      agentPagination.total = 0
    }
  } finally {
    if (!silent) agentLoading.value = false
  }
}

const loadVectorOverviewAgents = async (options = {}) => {
  const { silent = false, hydrateCmdb = true } = options
  if (!silent) agentLoading.value = true
  try {
    const response = await logtailApi.getVectorAgents({
      page: agentPagination.page,
      pageSize: agentPagination.pageSize,
      keyword: agentQuery.keyword || undefined,
      onlineStatus: agentQuery.onlineStatus || undefined,
      vectorStatus: agentQuery.vectorStatus || undefined,
      configStatus: agentQuery.configStatus || undefined
    })
    const nextAgents = extractList(response)
    agents.value = silent ? mergeExistingAgentRows(agents.value, nextAgents) : nextAgents
    scopeAgentRows.value = []
    statAgentRows.value = agents.value
    agentPagination.total = extractTotal(response)
    if (hydrateCmdb) await hydrateCmdbAgents(agents.value, { force: true })
    agents.value.forEach(attachCachedOperationError)
    syncOperationPollingState(agents.value)
    syncCmdbAgentPollingState(agents.value)
  } catch (error) {
    console.error('加载 Vector Agent 总览失败:', error)
    if (!silent) {
      agents.value = []
      scopeAgentRows.value = []
      statAgentRows.value = []
      agentPagination.total = 0
    }
  } finally {
    if (!silent) agentLoading.value = false
  }
}

const activeOperationStatuses = ['pending', 'running']
const operationStatusOf = row => String(row?.currentOperationStatus || '').toLowerCase()
const isCurrentOperationRunning = row => hasCurrentOperation(row)
const agentRecentError = row => isCurrentOperationRunning(row) ? '' : (row?.lastError || row?.errorMsg || row?.operationErrorMsg || '')

const canOpenErrorDialog = row => (
  Boolean(agentRecentError(row)) ||
  normalizeVectorStatus(row?.vectorStatus) === 'error' ||
  normalizeConfigStatus(row?.configStatus) === 'failed'
)

const fetchLatestOperationError = async row => {
  const agentId = agentIdOf(row)
  if (!agentId || operationErrorHydratedIds.has(String(agentId))) return
  operationErrorHydratedIds.add(String(agentId))
  const response = await logtailApi.getAgentOperations(agentId, { page: 1, pageSize: 5 })
  const failedOperation = extractList(response).find(item => String(item.opStatus || '').toLowerCase() === 'failed' && item.errorMsg)
  if (!failedOperation) return
  row.operationErrorMsg = failedOperation.errorMsg
  row.operationErrorAt = failedOperation.finishedAt || failedOperation.updatedAt || failedOperation.createdAt || ''
  operationErrorCache.set(String(agentId), {
    message: row.operationErrorMsg,
    time: row.operationErrorAt
  })
}

const openErrorDialog = async row => {
  errorDialog.current = row
  errorDialog.message = agentRecentError(row) || '正在加载错误详情...'
  errorDialog.visible = true
  if (!agentRecentError(row)) {
    try {
      await fetchLatestOperationError(row)
    } catch (error) {
      console.error('加载 Agent 错误详情失败:', error)
    }
    errorDialog.message = agentRecentError(row) || '暂无错误详情'
  }
}

const attachCachedOperationError = row => {
  const cachedError = operationErrorCache.get(String(agentIdOf(row)))
  if (!cachedError || row.operationErrorMsg) return
  row.operationErrorMsg = cachedError.message
  row.operationErrorAt = cachedError.time
}

const agentMatchesCurrentFilter = row => {
  const keyword = agentQuery.keyword.trim().toLowerCase()
  const keywordMatched = !keyword || [agentHostName(row), agentIp(row)].some(value => textIncludes(value, keyword))
  const onlineMatched = !agentQuery.onlineStatus || Number(agentQuery.onlineStatus) === Number(agentOnline(row) ? 1 : 2)
  const vectorMatched = !agentQuery.vectorStatus || normalizeVectorStatus(row.vectorStatus) === agentQuery.vectorStatus
  const configMatched = !agentQuery.configStatus || normalizeConfigStatus(row.configStatus) === agentQuery.configStatus
  return keywordMatched && onlineMatched && vectorMatched && configMatched
}

const mergeAgentRows = rows => {
  const merged = new Map()
  rows.forEach(row => {
    const key = agentIdOf(row) || row.hostId || `${agentHostName(row)}-${agentIp(row)}`
    if (!key) return
    const existing = merged.get(String(key))
    if (!existing) {
      merged.set(String(key), { ...row })
      return
    }
    merged.set(String(key), {
      ...existing,
      ...row,
      collectTaskCount: Math.max(vectorTaskCount(existing), vectorTaskCount(row))
    })
  })
  return Array.from(merged.values())
}

const agentRowKey = row => String(agentIdOf(row) || row?.hostId || `${agentHostName(row)}-${agentIp(row)}`)

const mergeExistingAgentRows = (currentRows, nextRows) => {
  const currentMap = new Map(currentRows.map(row => [agentRowKey(row), row]))
  return nextRows.map(row => {
    const existing = currentMap.get(agentRowKey(row))
    if (!existing) return row
    const cmdbAgent = row.cmdbAgent || existing.cmdbAgent || null
    Object.assign(existing, row)
    if (cmdbAgent) existing.cmdbAgent = cmdbAgent
    attachCachedOperationError(existing)
    return existing
  })
}

const operationProgressFields = [
  'vectorVersion',
  'vectorStatus',
  'vectorPid',
  'desiredConfigVersion',
  'appliedConfigVersion',
  'desiredConfigHash',
  'appliedConfigHash',
  'configStatus',
  'lastVectorHeartbeat',
  'lastReloadAt',
  'lastError',
  'lastErrorAt',
  'collectTaskCount',
  'currentOperationId',
  'currentOperationType',
  'currentOperationStatus',
  'currentOperationProgress',
  'currentOperationProgressText'
]

const patchOperationProgressState = (target, source) => {
  if (!target || !source) return
  operationProgressFields.forEach(field => {
    if (Object.prototype.hasOwnProperty.call(source, field)) {
      target[field] = source[field]
    }
  })
  if (source.lastError) {
    target.operationErrorMsg = source.lastError
    target.operationErrorAt = source.lastErrorAt || ''
    operationErrorCache.set(String(agentIdOf(target)), {
      message: target.operationErrorMsg,
      time: target.operationErrorAt
    })
  }
  attachCachedOperationError(target)
}

const normalizeOperationProgress = operation => {
  if (!operation) return null
  const opStatus = String(operation.opStatus || operation.status || '').toLowerCase()
  return {
    currentOperationId: operation.id || operation.operationId || 1,
    currentOperationType: operation.opType || operation.operationType || '',
    currentOperationStatus: opStatus,
    currentOperationProgress: Number(operation.progress ?? operation.currentOperationProgress ?? 0),
    currentOperationProgressText: operation.progressText || operation.currentOperationProgressText || '',
    lastError: opStatus === 'failed' ? operation.errorMsg || '' : '',
    lastErrorAt: opStatus === 'failed' ? operation.finishedAt || operation.updatedAt || operation.createdAt || '' : ''
  }
}

const patchOperationRecord = (row, operation) => {
  const progressState = normalizeOperationProgress(operation)
  if (!progressState) return false
  patchOperationProgressState(row, progressState)
  if (!activeOperationStatuses.includes(progressState.currentOperationStatus)) {
    operationPollTargets.delete(String(agentIdOf(row)))
  }
  return activeOperationStatuses.includes(progressState.currentOperationStatus)
}

const patchOperationProgressRows = sourceRows => {
  const sourceMap = new Map(sourceRows.map(row => [String(agentIdOf(row)), row]))
  const patchedObjects = new Set()

  ;[agents.value, statAgentRows.value, scopeAgentRows.value].forEach(rowList => {
    rowList.forEach(row => {
      const agentId = String(agentIdOf(row))
      const source = sourceMap.get(agentId)
      if (!source || patchedObjects.has(row)) return
      patchOperationProgressState(row, source)
      patchedObjects.add(row)
    })
  })

  return operationAgents.value.some(hasCurrentOperation)
}

const findVisibleAgentRow = agentId => (
  agents.value.find(row => sameId(agentIdOf(row), agentId)) ||
  statAgentRows.value.find(row => sameId(agentIdOf(row), agentId)) ||
  scopeAgentRows.value.find(row => sameId(agentIdOf(row), agentId)) ||
  null
)

const fetchOperationProgressRows = async () => {
  const agentIds = new Set(agents.value.map(row => String(agentIdOf(row))).filter(Boolean))
  if (!agentIds.size) return []

  const params = { page: 1, pageSize: Math.max(1000, agentIds.size) }
  const taskId = configTaskIdOf(currentTask.value)
  const response = agentContentTab.value === 'overview'
    ? await logtailApi.getVectorAgents(params)
    : taskId
    ? await logtailApi.getCollectTaskAgents(taskId, params)
    : await logtailApi.getLogtailAgents(params)

  return extractList(response).filter(row => agentIds.has(String(agentIdOf(row))))
}

const fetchLatestAgentOperation = async (agentId, opType) => {
  const response = await logtailApi.getAgentOperations(agentId, {
    page: 1,
    pageSize: 1,
    opType
  })
  return extractList(response)[0] || null
}

const refreshTrackedOperationProgress = async () => {
  if (!operationPollTargets.size) return false

  const results = await Promise.allSettled(Array.from(operationPollTargets.entries()).map(async ([agentId, opType]) => {
    const row = findVisibleAgentRow(agentId)
    if (!row) {
      operationPollTargets.delete(agentId)
      return false
    }
    const operation = await fetchLatestAgentOperation(agentId, opType)
    if (!operation) return hasCurrentOperation(row)
    return patchOperationRecord(row, operation)
  }))

  results.forEach(result => {
    if (result.status === 'rejected') console.error('刷新单机操作进度失败:', result.reason)
  })
  return operationAgents.value.some(hasCurrentOperation) || operationPollTargets.size > 0
}

const markOperationPending = (row, opType, operationId = 1) => {
  if (!row) return
  const agentId = String(agentIdOf(row))
  row.currentOperationId = operationId || 1
  row.currentOperationType = opType
  row.currentOperationStatus = 'pending'
  row.currentOperationProgress = 0
  row.currentOperationProgressText = '等待 Agent 执行'
  operationPollTargets.set(agentId, opType)
  operationErrorHydratedIds.delete(agentId)
  operationErrorCache.delete(agentId)
  row.operationErrorMsg = ''
  row.operationErrorAt = ''
}

const applyScopeAgentPagination = async (options = {}) => {
  const { silent = false, hydrateCmdb = true } = options
  const filtered = scopeAgentRows.value.filter(agentMatchesCurrentFilter)
  statAgentRows.value = filtered
  agentPagination.total = filtered.length
  const start = (agentPagination.page - 1) * agentPagination.pageSize
  const pageRows = filtered.slice(start, start + agentPagination.pageSize)
  agents.value = silent ? mergeExistingAgentRows(agents.value, pageRows) : pageRows
  if (hydrateCmdb) await hydrateCmdbAgents(agents.value, { force: true })
  agents.value.forEach(attachCachedOperationError)
  syncOperationPollingState(operationAgents.value)
  syncCmdbAgentPollingState(agents.value)
}

const loadScopeAgents = async (options = {}) => {
  const { silent = false } = options
  const scopeTasks = getNodeTasks(selectedNode.value)
  if (!scopeTasks.length) {
    scopeAgentRows.value = []
    statAgentRows.value = []
    agents.value = []
    agentPagination.total = 0
    return
  }

  if (!silent) agentLoading.value = true
  try {
    const responses = await Promise.all(scopeTasks.map(task => (
      logtailApi.getCollectTaskAgents(configTaskIdOf(task), { page: 1, pageSize: 1000 })
    )))
    scopeAgentRows.value = mergeAgentRows(responses.flatMap(response => extractList(response)))
    await applyScopeAgentPagination(options)
  } catch (error) {
    console.error('加载范围 Agent 失败:', error)
    if (!silent) {
      scopeAgentRows.value = []
      statAgentRows.value = []
      agents.value = []
      agentPagination.total = 0
    }
  } finally {
    if (!silent) agentLoading.value = false
  }
}

const loadSelectedNodeAgents = async (options = {}) => {
  if (isK8sCollectorContentTab.value) {
    return
  }
  if (agentContentTab.value === 'overview') {
    await loadVectorOverviewAgents(options)
    return
  }
  if (selectedNode.value?.type === 'task') {
    scopeAgentRows.value = []
    await loadTaskAgents(options)
    return
  }
  await loadScopeAgents(options)
}

const refreshK8sCollectorPage = async (options = {}) => {
  if (!isK8sCollectorContentTab.value) return
  await nextTick()
  await k8sCollectorPageRef.value?.refresh?.(options)
}

const handleDialogProjectChange = async () => {
  taskDialog.form.storeId = ''
  await loadStoresByProject(taskDialog.form.projectId, dialogStores)
}

const syncTaskNameWithStore = options => {
  if (taskDialog.form.id) return
  const { force = false } = options || {}
  const storeName = dialogStoreNameById(taskDialog.form.storeId)
  if (!storeName) return
  const currentName = String(taskDialog.form.name || '').trim()
  const knownStoreNames = new Set([
    ...dialogStores.value.map(item => String(item?.name || '').trim()),
    String(selectedNode.value?.store?.name || '').trim()
  ].filter(Boolean))
  if (force || !currentName || knownStoreNames.has(currentName)) {
    taskDialog.form.name = storeName
  }
}

const handleDialogStoreChange = () => {
  syncTaskNameWithStore()
}

const handleAgentSearch = () => {
  agentPagination.page = 1
  loadSelectedNodeAgents()
}

const resetAgentSearch = () => {
  Object.assign(agentQuery, {
    keyword: '',
    onlineStatus: '',
    vectorStatus: '',
    configStatus: ''
  })
  agentPagination.page = 1
  loadSelectedNodeAgents()
}

const handleAgentSizeChange = size => {
  agentPagination.pageSize = size
  agentPagination.page = 1
  loadSelectedNodeAgents()
}

const handleAgentPageChange = async page => {
  agentPagination.page = page
  if (agentContentTab.value === 'overview') {
    await loadVectorOverviewAgents()
    return
  }
  if (selectedNode.value?.type === 'task') {
    await loadTaskAgents()
    return
  }
  await applyScopeAgentPagination()
}

const handleAgentTabChange = async () => {
  agentPagination.page = 1
  if (isK8sCollectorContentTab.value) {
    await refreshK8sCollectorPage()
    return
  }
  await loadSelectedNodeAgents()
}

const selectConfigNode = async (node, options = {}) => {
  if (!node) return
  const sameNode = selectedNode.value?.key === node.key
  const nextContentTab = isK8sNode(node) ? 'k8s' : 'task'
  const shouldResetToTaskTab = agentContentTab.value !== nextContentTab
  if (sameNode && agents.value.length && !shouldResetToTaskTab) return
  if (!options.preserveExpansion) syncExpandedProject(node)
  agentContentTab.value = nextContentTab
  setSelectedNode(node)
  agentPagination.page = 1
  await loadSelectedNodeAgents()
}

const toggleProjectNode = async projectNode => {
  if (!projectNode) return
  const nextExpandedKey = expandedProjectKey.value === projectNode.key ? '' : projectNode.key
  expandedProjectKey.value = nextExpandedKey
  await selectConfigNode(projectNode, { preserveExpansion: true })
}

const toggleStoreNode = storeNode => {
  if (!storeNode) return
  setStoreExpanded(storeNode.key, !expandedStoreKeys.value.includes(storeNode.key))
}

const selectTask = async (task, projectNode, storeNode) => {
  const taskId = configTaskIdOf(task)
  if (!taskId) return
  if (storeNode?.key) setStoreExpanded(storeNode.key, true)
  const node = {
    key: `task-${taskId}`,
    type: 'task',
    project: projectNode?.project || selectedNode.value?.project,
    store: storeNode?.store || selectedNode.value?.store,
    task
  }
  await selectConfigNode(node)
}

const openTaskDialog = async row => {
  const isEdit = Boolean(row?.id)
  let dialogTask = row || {}
  resetAiParseResult()
  if (isEdit) {
    try {
      const response = await logtailApi.getCollectTask(row.id)
      dialogTask = {
        ...row,
        ...(extractData(response) || {}),
        id: row.id
      }
    } catch (error) {
      console.error('加载采集任务详情失败:', error)
    }
  }

  Object.assign(taskDialog.form, createTaskForm(dialogTask))
  if (isK8sCollectType(taskDialog.form.collectType)) {
    dialogAgents.value = []
    await Promise.all([
      loadStoresByProject(taskDialog.form.projectId, dialogStores),
      loadK8sTaskOptions()
    ])
  } else {
    await Promise.all([
      loadStoresByProject(taskDialog.form.projectId, dialogStores),
      loadDialogAgents(isEdit ? dialogTask.id : null, isEdit ? 'task' : 'available')
    ])
    ensureDialogAgentsContainTargets(dialogTask)
    if (dialogTask?.id && dialogTask.executeMode === 'all_agents' && !taskDialog.form.targetAgentIds.length) {
      taskDialog.form.targetAgentIds = dialogAgents.value.map(agentSelectValue).filter(Boolean)
    } else {
      taskDialog.form.targetAgentIds = taskDialog.form.targetAgentIds.filter(id => (
        dialogAgents.value.some(agent => sameId(agentIdOf(agent), id))
      ))
    }
  }
  if (!isEdit) {
    taskDialog.form.k8sLogSource = 'stdout'
    taskDialog.form.k8sLogSources = k8sLogSourceSelections('stdout')
    syncTaskNameWithStore({ force: true })
  }
  taskDialog.visible = true
}

const openStoreTaskDialog = storeNode => {
  const projectId = storeNode?.project?.id
  const storeId = storeNode?.store?.id
  if (!projectId || !storeId || String(storeId).startsWith('orphan-')) {
    ElMessage.warning('当前日志库不能创建采集任务')
    return
  }
  return openTaskDialog({
    projectId,
    storeId,
    storeName: storeNode.store?.name || storeNode.store?.code || '',
    store: storeNode.store
  })
}

const canCreateTaskInStore = storeNode => {
  const projectId = String(storeNode?.project?.id || '')
  const storeId = String(storeNode?.store?.id || '')
  return Boolean(projectId && storeId) && !storeId.startsWith('orphan-')
}

const findLocalTaskById = taskId => (
  tasks.value.find(task => sameId(configTaskIdOf(task), taskId)) ||
  (sameId(configTaskIdOf(currentTask.value), taskId) ? currentTask.value : null)
)

const buildPathEditTaskBase = async task => {
  const taskId = configTaskIdOf(task)
  if (!taskId) return null
  let base = {
    ...(findLocalTaskById(taskId) || {}),
    ...task,
    id: taskId
  }

  if (!base.projectId || !base.storeId || !base.name) {
    const response = await logtailApi.getCollectTask(taskId)
    base = {
      ...base,
      ...(extractData(response) || {}),
      id: taskId
    }
  }

  return base
}

const openPathEditDialog = () => {
  const task = editableConfigTask.value
  const taskId = configTaskIdOf(task)
  if (!task || !taskId) {
    ElMessage.warning('当前配置预览暂无可编辑的采集任务')
    return
  }

  pathEditDialog.task = task
  const pathPatterns = taskPathPatterns(task)
  Object.assign(pathEditDialog.form, {
    taskId,
    name: task.name || task.taskName || '-',
    pathPattern: joinPathPatterns(pathPatterns),
    pathPatterns: pathPatterns.length ? pathPatterns : [''],
    remark: task.remark || ''
  })
  pathEditDialog.visible = true
}

const handleCollectTypeChange = type => {
  taskDialog.form.collectType = normalizeCollectType(type)
  if (isK8sCollectType(type)) {
    taskDialog.form.targetAgentIds = []
    taskDialog.form.k8sLogSource = normalizeK8sLogSource(taskDialog.form.k8sLogSource || taskDialog.form.k8sLogSources)
    taskDialog.form.k8sLogSources = k8sLogSourceSelections(taskDialog.form.k8sLogSource)
    taskDialog.form.executeMode = taskDialog.form.k8sLogSource === 'node_file' ? 'selected_clusters' : 'selected_workloads'
    taskFormRef.value?.clearValidate(['targetAgentIds', 'pathPatterns', 'k8sLogSource'])
    loadK8sTaskOptions()
    return
  }
  taskDialog.form.executeMode = 'selected_agents'
  taskFormRef.value?.clearValidate(['k8sClusterId', 'k8sNamespace', 'k8sWorkloadName'])
  loadDialogAgents(taskDialog.form.id || null, taskDialog.form.id ? 'task' : 'available')
}

const handleLogTypeChange = () => {
  const logType = normalizeLogType(taskDialog.form.logType)
  if (!shouldShowSampleLog(logType)) taskDialog.form.sampleLog = ''
  if (logType !== 'regex') taskDialog.form.regexPattern = ''
  taskDialog.form.parseTemplateId = ''
  taskDialog.form.parseModeSnapshot = logType
  taskDialog.form.multilineEnabledSnapshot = true
  taskDialog.form.multilinePatternSnapshot = defaultMultilinePattern(logType)
  resetAiParseResult()
  taskFormRef.value?.clearValidate(['regexPattern', 'sampleLog', 'multilinePatternSnapshot'])
}

const handleMultilineEnabledChange = enabled => {
  if (enabled && !String(taskDialog.form.multilinePatternSnapshot || '').trim()) {
    taskDialog.form.multilinePatternSnapshot = defaultMultilinePattern(taskDialog.form.logType) || defaultTextMultilinePattern
  }
  taskFormRef.value?.clearValidate('multilinePatternSnapshot')
}

const handleSampleLogInput = () => {
  if (!aiParse.result && !taskDialog.form.parseTemplateId) return
  taskDialog.form.parseTemplateId = ''
  taskDialog.form.parseModeSnapshot = normalizeLogType(taskDialog.form.logType)
  resetAiParseResult()
}

const resetAiParseResult = () => {
  aiParse.loading = false
  aiParse.result = null
  aiParse.error = ''
}

const buildAiParseTemplateName = () => {
  const logType = normalizeLogType(taskDialog.form.logType)
  const base = String(taskDialog.form.name || dialogStoreNameById(taskDialog.form.storeId) || 'log_parse')
    .trim()
    .replace(/[^\w\u4e00-\u9fa5-]+/g, '_')
    .replace(/^_+|_+$/g, '')
  return `${base || 'log_parse'}_${logType}_ai_parse`
}

const generateAiParseTemplate = async () => {
  const projectId = Number(taskDialog.form.projectId || 0)
  const logType = normalizeLogType(taskDialog.form.logType)
  const sampleLog = String(taskDialog.form.sampleLog || '').trim()
  aiParse.error = ''
  if (!projectId) {
    ElMessage.warning('请先选择项目')
    return
  }
  if (!logType) {
    ElMessage.warning('请先选择日志类型')
    return
  }
  if (!sampleLog) {
    ElMessage.warning('请先粘贴样例日志')
    return
  }

  aiParse.loading = true
  try {
    const response = await logtailApi.generateParseTemplateByAI({
      providerId: 0,
      projectId,
      name: buildAiParseTemplateName(),
      logType,
      sampleLog,
      save: true
    })
    if (!isBusinessSuccess(response)) {
      const message = responseMessage(response) || 'AI解析失败'
      aiParse.error = message
      ElMessage.error(message)
      return
    }
    aiParse.result = extractData(response) || {}
    if (!aiParseTemplateId.value) {
      aiParse.error = 'AI解析已返回，但未获得模板 ID'
      ElMessage.warning(aiParse.error)
      return
    }
    ElMessage.success('AI解析模板已生成')
  } catch (error) {
    console.error('AI解析模板生成失败:', error)
    const message = error.response?.data?.message || error.message || 'AI解析失败'
    aiParse.error = message
    ElMessage.error(message)
  } finally {
    aiParse.loading = false
  }
}

const useAiParseTemplate = () => {
  if (!aiParseTemplateId.value) {
    ElMessage.warning('当前 AI 解析结果没有模板 ID')
    return
  }
  taskDialog.form.parseTemplateId = aiParseTemplateId.value
  taskDialog.form.parseModeSnapshot = normalizeLogType(taskDialog.form.logType)
  const templateMultilineEnabled = aiParseTemplate.value?.multilineEnabled ?? aiParseTemplate.value?.multiline_enabled
  const templateMultilinePattern = aiParseTemplate.value?.multilinePattern || aiParseTemplate.value?.multiline_pattern
  if (templateMultilineEnabled !== undefined && templateMultilineEnabled !== null) {
    taskDialog.form.multilineEnabledSnapshot = Boolean(templateMultilineEnabled)
    taskDialog.form.multilinePatternSnapshot = templateMultilineEnabled ? (templateMultilinePattern || defaultMultilinePattern(taskDialog.form.logType)) : ''
  }
  ElMessage.success('已使用 AI 解析规则')
}

const selectedTargetAgents = form => (
  form.targetAgentIds
    .map(id => dialogAgents.value.find(agent => sameId(agentIdOf(agent), id)))
    .filter(Boolean)
)

const buildExecuteTargetsJson = form => {
  const targets = selectedTargetAgents(form).map(agent => ({
    agentId: Number(agentIdOf(agent)),
    hostId: Number(agentHostIdOf(agent))
  }))
  if (!targets.length) return ''
  return JSON.stringify(targets)
}

const buildK8sExecuteTargetsJson = form => {
  const k8sLogSource = normalizeK8sLogSource(form.k8sLogSource || form.k8sLogSources)
  const target = {
    clusterId: Number(form.k8sClusterId),
    clusterName: form.k8sClusterName || '',
    namespace: form.k8sNamespace,
    workloadType: normalizeK8sWorkloadType(form.k8sWorkloadType),
    workloadName: form.k8sWorkloadName
  }
  if (!target.clusterId) return ''
  if (k8sLogSource === 'node_file') {
    return JSON.stringify([{
      clusterId: target.clusterId,
      clusterName: target.clusterName
    }])
  }
  if (!target.namespace || !target.workloadName) return ''
  return JSON.stringify([target])
}

const normalizeParsePayload = form => {
  const logType = normalizeLogType(form.logType)
  const sampleLog = shouldShowSampleLog(logType) ? String(form.sampleLog || '').trim() : ''
  const regexPattern = logType === 'regex' ? String(form.regexPattern || '').trim() : ''
  const parseTemplateId = Number(form.parseTemplateId || 0)
  const multilineEnabledSnapshot = Boolean(form.multilineEnabledSnapshot)
  const multilinePatternSnapshot = multilineEnabledSnapshot ? String(form.multilinePatternSnapshot || '').trim() : ''
  if (logType === 'regex' && !regexPattern) {
    ElMessage.warning('请输入解析正则')
    return null
  }
  if (regexPattern && !hasNamedCaptureGroup(regexPattern)) {
    ElMessage.warning('解析正则必须包含命名捕获组，例如 (?P<status>\\d+)')
    return null
  }
  if (multilineEnabledSnapshot && !multilinePatternSnapshot) {
    ElMessage.warning('请输入日志起始正则')
    return null
  }
  return {
    logType,
    parseModeSnapshot: form.parseModeSnapshot || logType,
    parseTemplateId,
    sampleLog,
    regexPattern,
    multilineEnabledSnapshot,
    multilinePatternSnapshot
  }
}

const normalizeHostTaskPayload = form => {
  const parsePayload = normalizeParsePayload(form)
  if (!parsePayload) return null

  const executeMode = 'selected_agents'
  const executeTargetsJson = buildExecuteTargetsJson(form)
  if (!executeTargetsJson) {
    ElMessage.warning('请选择至少一个健康 Vector 主机')
    return null
  }
  const pathPatterns = splitPathPatterns(form.pathPatterns)
  if (!pathPatterns.length) {
    ElMessage.warning('请至少添加一个日志路径')
    return null
  }

  return {
    id: form.id,
    projectId: Number(form.projectId),
    storeId: Number(form.storeId),
    name: form.name,
    collectType: 'host',
    ...parsePayload,
    executeMode,
    executeTargetsJson,
    pathPattern: joinPathPatterns(pathPatterns),
    pathPatterns,
    status: Number(form.status || 1),
    remark: form.remark || '',
    createdBy: form.createdBy || 'admin',
    updatedBy: 'admin'
  }
}

const normalizeK8sTaskPayload = form => {
  const parsePayload = normalizeParsePayload(form)
  if (!parsePayload) return null
  const k8sLogSource = normalizeK8sLogSource(form.k8sLogSource || form.k8sLogSources)
  const executeTargetsJson = buildK8sExecuteTargetsJson(form)
  if (!executeTargetsJson) {
    ElMessage.warning(k8sLogSource === 'node_file' ? '请选择 K8S 集群' : '请选择 K8S 集群、命名空间和工作负载')
    return null
  }
  if (!k8sLogSource) {
    ElMessage.warning('请选择采集来源')
    return null
  }
  const pathPatterns = k8sLogSourceRequiresPath(k8sLogSource) ? splitPathPatterns(form.pathPatterns) : []
  if (k8sLogSourceRequiresPath(k8sLogSource) && !pathPatterns.length) {
    ElMessage.warning(k8sLogSource === 'node_file' ? '请至少添加一个宿主机日志路径' : '请至少添加一个容器内日志路径')
    return null
  }
  const pathPattern = joinPathPatterns(pathPatterns)

  return {
    id: form.id,
    projectId: Number(form.projectId),
    storeId: Number(form.storeId),
    name: form.name,
    collectType: 'kubernetes',
    ...parsePayload,
    executeMode: k8sLogSource === 'node_file' ? 'selected_clusters' : 'selected_workloads',
    executeTargetsJson,
    k8sLogSource,
    pathPattern,
    pathPatterns,
    pathPatternsJson: JSON.stringify(pathPatterns),
    status: Number(form.status || 1),
    remark: form.remark || '',
    createdBy: form.createdBy || 'admin',
    updatedBy: 'admin'
  }
}

const normalizeTaskPayload = form => (
  isK8sCollectType(form.collectType)
    ? normalizeK8sTaskPayload(form)
    : normalizeHostTaskPayload(form)
)

const taskApplyMessage = (response, fallback = '') => {
  const data = extractData(response)
  return data?.message || data?.autoApplyMessage || responseMessage(response) || fallback
}

const showTaskApplyMessage = (response, fallback = '') => {
  const data = extractData(response)
  const message = taskApplyMessage(response, fallback)
  if (String(data?.status || '').toLowerCase() === 'skipped') {
    ElMessage.info(message)
    return
  }
  ElMessage.success(message)
}

const submitTask = async () => {
  await taskFormRef.value?.validate()
  const payload = normalizeTaskPayload(taskDialog.form)
  if (!payload) return
  taskDialog.submitting = true
  try {
    let result = null
    let taskId = payload.id
    let shouldPollOperations = false
    const isK8sPayload = isK8sCollectType(payload.collectType)
    if (payload.id) {
      const response = await logtailApi.updateCollectTask(payload.id, payload)
      result = extractData(response)
      if (taskId && !isK8sPayload) {
        const applyResponse = await logtailApi.applyCollectTask(taskId, { operator: 'admin' })
        shouldPollOperations = String(extractData(applyResponse)?.status || '').toLowerCase() !== 'skipped'
        showTaskApplyMessage(applyResponse, result?.autoApplyMessage || '采集任务已保存并下发')
      } else {
        showTaskApplyMessage(response, 'K8S 采集任务已更新，后端将同步 Collector')
      }
    } else {
      const response = await logtailApi.createCollectTask(payload)
      result = extractData(response)
      taskId = result?.id
      if (taskId) selectedTaskId.value = taskId
      shouldPollOperations = !isK8sPayload && String(result?.status || '').toLowerCase() !== 'skipped' && Number(payload.status) !== 2
      showTaskApplyMessage(response, isK8sPayload ? 'K8S 采集任务已创建，后端将自动下发 Collector' : '采集任务已创建，后端已自动下发')
    }
    taskDialog.visible = false
    if (payload.id) selectedTaskId.value = payload.id
    await loadConfigTree(`task-${taskId || selectedTaskId.value}`)
    if (isK8sPayload) await refreshK8sCollectorPage({ resetPolling: true })
    else if (shouldPollOperations) startOperationPolling({ resetDeadline: true })
  } finally {
    taskDialog.submitting = false
  }
}

const syncTaskPathLocally = (taskId, pathPattern, pathPatterns, remark) => {
  const syncTask = task => {
    if (!task || !sameId(configTaskIdOf(task), taskId)) return
    task.pathPattern = pathPattern
    task.pathPatterns = pathPatterns
    task.pathPatternsJson = JSON.stringify(pathPatterns)
    task.remark = remark
  }

  tasks.value.forEach(syncTask)
  configPreviewTasks.value.forEach(syncTask)
  configTree.value.forEach(projectNode => {
    projectNode.tasks?.forEach(syncTask)
    projectNode.stores?.forEach(storeNode => storeNode.tasks?.forEach(syncTask))
  })
  if (selectedNode.value?.type === 'task') syncTask(selectedNode.value.task)
}

const submitPathEdit = async () => {
  await pathEditFormRef.value?.validate()
  const taskId = pathEditDialog.form.taskId
  const baseTask = await buildPathEditTaskBase(pathEditDialog.task)
  if (!taskId || !baseTask) {
    ElMessage.warning('缺少采集任务信息，无法保存')
    return
  }

  pathEditDialog.submitting = true
  try {
    const pathPatterns = splitPathPatterns(pathEditDialog.form.pathPatterns)
    if (!pathPatterns.length) {
      ElMessage.warning('请至少添加一个日志路径')
      return
    }
    const pathPattern = joinPathPatterns(pathPatterns)
    const remark = pathEditDialog.form.remark || ''
    const payload = {
      ...baseTask,
      id: taskId,
      pathPattern,
      pathPatterns,
      remark,
      updatedBy: 'admin'
    }
    await logtailApi.updateCollectTask(taskId, payload)
    syncTaskPathLocally(taskId, pathPattern, pathPatterns, remark)
    pathEditDialog.visible = false
    ElMessage.success('保存成功，已自动重新下发配置')
    await loadConfigPreview()
    if (agentDrawer.activeTab === 'tasks') await loadAgentTasks()
    await loadSelectedNodeAgents({ silent: true, hydrateCmdb: false })
    startOperationPolling({ resetDeadline: true })
  } finally {
    pathEditDialog.submitting = false
  }
}

const removeTask = async row => {
  const taskId = configTaskIdOf(row)
  if (!taskId) {
    ElMessage.warning('缺少采集任务 ID，无法删除')
    return
  }
  const k8s = isK8sTask(row)
  const prerequisite = k8s ? '卸载 K8S Collector 并删除 Kubernetes 资源' : '停止采集，等待 Agent 配置生效并删除任务配置文件'
  await ElMessageBox.confirm(`删除任务「${row.name || taskId}」将执行以下流程：\n1. ${prerequisite}\n2. 删除数据库任务\n是否继续？`, '删除采集任务', {
    type: 'warning',
    confirmButtonText: k8s ? '卸载并删除' : '停止并删除',
    cancelButtonText: '取消'
  })
  taskActionLoading.value = true
  try {
    const actionResponse = k8s
      ? await logtailApi.uninstallKubernetesCollector(taskId, { operator: 'admin' })
      : await logtailApi.stopCollectTask(taskId, { operator: 'admin' })
    if (!isBusinessSuccess(actionResponse)) {
      ElMessage.error(responseMessage(actionResponse) || (k8s ? '卸载 K8S Collector 失败' : '停止采集失败'))
      return
    }
    const response = await logtailApi.deleteCollectTask(taskId)
    if (!isBusinessSuccess(response)) {
      ElMessage.error(responseMessage(response) || '采集任务删除失败，请确认前置操作已完成后重试')
      return
    }
    ElMessage.success(responseMessage(response) || '采集任务已删除')
    await loadConfigTree()
  } finally {
    taskActionLoading.value = false
  }
}

const stopCollectTask = async row => {
  const taskId = configTaskIdOf(row)
  if (!taskId) {
    ElMessage.warning('缺少采集任务 ID，无法停止采集')
    return
  }
  if (!canStopCollectTask(row)) {
    ElMessage.warning('当前采集任务已禁用')
    return
  }

  try {
    await ElMessageBox.confirm(`确认停止采集任务「${row.name}」吗？`, '停止采集', {
      type: 'warning',
      confirmButtonText: '停止采集',
      cancelButtonText: '取消'
    })
    taskActionLoading.value = true
    const response = await logtailApi.stopCollectTask(taskId, { operator: 'admin' })
    if (!isBusinessSuccess(response)) {
      ElMessage.error(responseMessage(response) || '停止采集失败')
      return
    }
    ElMessage.success(responseMessage(response) || '采集任务已停止')
    try {
      await loadConfigTree(`task-${taskId}`)
    } catch (refreshError) {
      console.error('刷新采集配置失败:', refreshError)
      ElMessage.warning('采集任务已停止，但页面刷新失败')
    }
  } catch (error) {
    if (error !== 'cancel') {
      console.error('停止采集任务失败:', error)
      ElMessage.error(`停止采集失败: ${error.response?.data?.message || error.message || '未知错误'}`)
    }
  } finally {
    taskActionLoading.value = false
  }
}

const executeModeText = mode => ({
  all_agents: '全部 Agent',
  selected_agents: '指定 Agent',
  selected_workloads: '指定工作负载',
  k8s_daemonset: 'K8s DaemonSet'
}[mode] || mode || '-')

const projectNameOf = task => task?.projectName || projects.value.find(item => item.id === task?.projectId)?.name || '-'
const storeNameOf = task => task?.storeName || task?.storeCode || task?.storeId || '-'
const statusText = status => Number(status) === 2 ? '禁用' : '启用'
const statusTagType = status => Number(status) === 2 ? 'info' : 'success'
const cmdbAgentOf = row => row?.cmdbAgent || null
const cmdbAgentStatus = row => Number(cmdbAgentOf(row)?.status ?? 0)
const cmdbAgentOnlineStatus = row => Number(cmdbAgentOf(row)?.onlineStatus ?? 0)
const cmdbAgentInstallProgress = row => Number(cmdbAgentOf(row)?.installProgress ?? 0)
const agentOnline = row => {
  const cmdbAgent = cmdbAgentOf(row)
  if (cmdbAgent) return Number(cmdbAgent.onlineStatus ?? 0) === 1
  return Number(row?.agentOnlineStatus ?? row?.onlineStatus ?? row?.agentStatus ?? row?.status) === 1
}
const agentHostName = row => row?.hostName || row?.hostname || row?.name || '-'
const agentIp = row => row?.privateIp || row?.sshIp || row?.publicIp || row?.ip || '-'
const agentIdOf = row => row?.agentId || row?.id
const agentSelectValue = row => Number(agentIdOf(row)) || agentIdOf(row)
const agentHostIdOf = row => row?.hostId || row?.hostID || row?.cmdbHostId || row?.assetId || cmdbAgentOf(row)?.hostId || 0
const isAvailableVectorAgent = row => {
  const vectorStatus = normalizeVectorStatus(row?.vectorStatus)
  return (
    agentOnline(row) &&
    Boolean(agentIdOf(row)) &&
    ['installed', 'running'].includes(vectorStatus)
  )
}
const isSelectableDialogAgent = row => (
  dialogAgentMode.value === 'task'
    ? agentOnline(row) && Boolean(agentIdOf(row))
    : isAvailableVectorAgent(row)
)
const dialogAgentLabel = row => `${agentHostName(row)} / ${agentIp(row)}`

const normalizeVectorStatus = status => {
  const value = String(status || '').toLowerCase()
  if (!value) return 'not_installed'
  return value === 'stopped' ? 'error' : value
}
const normalizeConfigStatus = status => String(status || 'pending').trim().toLowerCase()

const vectorTaskCount = row => Number(row?.collectTaskCount ?? row?.taskCount ?? 0)

const vectorStatusOf = value => (
  value && typeof value === 'object' ? normalizeVectorStatus(value.vectorStatus) : normalizeVectorStatus(value)
)

const vectorStatusText = value => {
  const status = vectorStatusOf(value)
  if (status === 'not_installed') return '未安装'
  if (status === 'installed') return '已安装'
  if (status === 'running') return '运行中'
  if (status === 'error') return '异常中'
  return status || '-'
}

const vectorStatusType = value => {
  const status = vectorStatusOf(value)
  if (status === 'running') return 'success'
  if (status === 'installed') return 'warning'
  if (status === 'error') return 'danger'
  return 'info'
}

const configStatusOf = value => (
  value && typeof value === 'object' ? normalizeConfigStatus(value.configStatus) : normalizeConfigStatus(value)
)

const configStatusText = value => {
  if (value && typeof value === 'object' && value.configStatusText) return value.configStatusText
  const status = configStatusOf(value)
  return ({
  pending: '待生效',
  unconfigured: '未配置',
  none: '未配置',
  applied: '已生效',
  disabled: '任务禁用中',
  failed: '失败'
  }[status] || status || '-')
}

const configStatusType = value => ({
  pending: 'warning',
  unconfigured: 'info',
  none: 'info',
  applied: 'success',
  disabled: 'danger',
  failed: 'danger'
}[configStatusOf(value)] || 'info')

const configStatusClass = value => ({
  'is-running-config-not-applied': String(configStatusText(value)).trim() === '运行中，新配置未生效'
})

const operationTypeText = type => ({
  install_vector: '安装 Vector',
  upgrade_vector: '升级 Vector',
  validate_config: '校验配置',
  reload_vector: '重载配置',
  restart_vector: '重启 Vector',
  stop_vector: '停止 Vector',
  uninstall_vector: '卸载 Vector',
  install: '安装 Collector',
  update: '更新 Collector',
  uninstall: '卸载 Collector',
  reinstall: '重装 Collector',
  install_k8s_collector: '安装 Collector',
  update_k8s_collector: '更新 Collector',
  uninstall_k8s_collector: '卸载 Collector',
  reinstall_k8s_collector: '重装 Collector'
}[type] || type || '-')

const operationStatusText = status => ({
  pending: '等待执行',
  running: '执行中',
  success: '成功',
  failed: '失败',
  canceled: '已取消'
}[status] || status || '-')

const operationStatusType = status => ({
  pending: 'warning',
  running: 'primary',
  success: 'success',
  failed: 'danger',
  canceled: 'info'
}[status] || 'info')

const isCmdbAgentDeploying = row => cmdbAgentStatus(row) === 1
const isCmdbAgentDeployFailed = row => cmdbAgentStatus(row) === 2 || cmdbAgentInstallProgress(row) === -1
const isCmdbAgentRunningState = row => (
  cmdbAgentStatus(row) === 3 &&
  cmdbAgentOnlineStatus(row) === 1 &&
  cmdbAgentInstallProgress(row) === 100
)
const shouldShowCmdbAgentProgress = row => Boolean(cmdbAgentOf(row))

const cmdbAgentProgress = row => {
  if (isCmdbAgentDeployFailed(row)) return 100
  if (isCmdbAgentRunningState(row)) return 100
  const progress = cmdbAgentInstallProgress(row)
  if (Number.isNaN(progress)) return 0
  return Math.min(100, Math.max(0, progress))
}

const cmdbAgentProgressStatus = row => {
  if (isCmdbAgentDeployFailed(row) || cmdbAgentOnlineStatus(row) === 2) return 'exception'
  if (isCmdbAgentRunningState(row) || isCmdbAgentDeploying(row)) return 'success'
  return undefined
}

const cmdbAgentProgressClass = row => {
  if (isCmdbAgentDeployFailed(row) || cmdbAgentOnlineStatus(row) === 2) return 'agent-status-progress-failed'
  if (isCmdbAgentRunningState(row)) return 'agent-status-progress-success'
  if (isCmdbAgentDeploying(row)) return 'agent-status-progress-running'
  return 'agent-status-progress-muted'
}

const cmdbAgentProgressIcon = row => {
  if (isCmdbAgentDeployFailed(row) || cmdbAgentOnlineStatus(row) === 2) return '×'
  if (isCmdbAgentRunningState(row)) return '✓'
  return '●'
}

const cmdbAgentProgressText = row => {
  const cmdbAgent = cmdbAgentOf(row)
  if (!cmdbAgent) return agentOnline(row) ? '在线' : '离线'
  if (isCmdbAgentDeployFailed(row)) return cmdbAgent.installProgressText || '部署失败'
  if (isCmdbAgentRunningState(row)) return cmdbAgent.installProgressText || '启动成功'
  if (isCmdbAgentDeploying(row)) return cmdbAgent.installProgressText || '部署中...'
  if (cmdbAgentOnlineStatus(row) === 1) return cmdbAgent.statusText || '在线'
  if (cmdbAgentOnlineStatus(row) === 2) return cmdbAgent.statusText || '离线'
  return cmdbAgent.installProgressText || cmdbAgent.statusText || '未知状态'
}

const isVectorInstalled = row => normalizeVectorStatus(row?.vectorStatus) !== 'not_installed'
const hasCurrentOperation = row => Number(row?.currentOperationId || 0) > 0 && activeOperationStatuses.includes(operationStatusOf(row))
const shouldShowOperationProgress = row => (
  hasCurrentOperation(row) ||
  ['success', 'failed'].includes(operationStatusOf(row)) ||
  ['installed', 'running', 'error'].includes(normalizeVectorStatus(row?.vectorStatus))
)
const canSubmitInstallVector = row => agentOnline(row) && !hasCurrentOperation(row)
const canRedeployCmdbAgent = row => Boolean(agentHostIdOf(row)) && !isCmdbAgentDeploying(row)
const isVectorNotInstalled = row => normalizeVectorStatus(row?.vectorStatus) === 'not_installed'
const vectorPrimaryActionText = row => isVectorNotInstalled(row) ? '安装' : '重载'
const vectorPrimaryActionType = row => isVectorNotInstalled(row) ? 'success' : 'warning'
const vectorPrimaryActionOp = row => isVectorNotInstalled(row) ? 'install_vector' : 'reload_vector'
const canOperateVector = row => agentOnline(row) && !hasCurrentOperation(row) && isVectorInstalled(row)
const canReloadConfig = row => canOperateVector(row)
const canSubmitVectorPrimaryAction = row => isVectorNotInstalled(row) ? canSubmitInstallVector(row) : canReloadConfig(row)
const canUninstallVector = row => canOperateVector(row)

const operationProgress = row => {
  const progress = Number(row?.currentOperationProgress || 0)
  const safeProgress = Number.isNaN(progress) ? 0 : Math.min(100, Math.max(0, progress))
  const vectorStatus = normalizeVectorStatus(row?.vectorStatus)
  const status = operationStatusOf(row)
  if (status === 'success') return 100
  if (status === 'failed') return safeProgress || 100
  if (!hasCurrentOperation(row) && ['installed', 'running', 'error'].includes(vectorStatus)) return 100
  return safeProgress
}

const operationProgressStatus = row => {
  const status = operationStatusOf(row)
  const vectorStatus = normalizeVectorStatus(row?.vectorStatus)
  if (status === 'success') return 'success'
  if (status === 'failed') return 'exception'
  if (!hasCurrentOperation(row) && ['installed', 'running'].includes(vectorStatus)) return 'success'
  if (!hasCurrentOperation(row) && vectorStatus === 'error') return 'exception'
  return undefined
}

const operationProgressState = row => {
  const status = operationStatusOf(row)
  if (['pending', 'running', 'success', 'failed'].includes(status)) return status
  const vectorStatus = normalizeVectorStatus(row?.vectorStatus)
  if (['installed', 'running'].includes(vectorStatus)) return 'success'
  if (vectorStatus === 'error') return 'failed'
  return 'running'
}

const operationProgressClass = row => `operation-progress-${operationProgressState(row)}`
const operationProgressIcon = row => {
  const status = operationStatusOf(row)
  const vectorStatus = normalizeVectorStatus(row?.vectorStatus)
  if (status === 'success') return '✓'
  if (status === 'failed') return '×'
  if (!hasCurrentOperation(row) && ['installed', 'running'].includes(vectorStatus)) return '✓'
  if (!hasCurrentOperation(row) && vectorStatus === 'error') return '×'
  return '●'
}

const operationProgressText = row => {
  const status = operationStatusOf(row)
  const progressText = row?.currentOperationProgressText || ''
  if (status === 'failed') return progressText ? `失败：${progressText}` : '执行失败'
  if (status === 'success') return progressText || '执行完成'
  if (!hasCurrentOperation(row)) return operationIdleText(row)
  if (progressText) return progressText
  return operationTypeText(row?.currentOperationType) || '执行中'
}

const operationIdleText = row => {
  const status = normalizeVectorStatus(row?.vectorStatus)
  if (status === 'installed') return '已安装'
  if (status === 'running') return '已安装'
  if (status === 'not_installed') return '未安装'
  if (status === 'error') return '异常待处理'
  return '-'
}

const operationIdleClass = row => {
  const status = normalizeVectorStatus(row?.vectorStatus)
  if (['installed', 'running'].includes(status)) return 'success-text'
  if (status === 'error') return 'danger-text'
  return 'muted-text'
}

const vectorHeartbeatText = row => {
  if (normalizeVectorStatus(row?.vectorStatus) === 'not_installed' && row?.lastVectorHeartbeat) return 'Agent已接入'
  return formatTime(row?.lastVectorHeartbeat)
}

const formatBytes = value => {
  const size = Number(value || 0)
  if (!size) return '-'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  if (size < 1024 * 1024 * 1024) return `${(size / 1024 / 1024).toFixed(1)} MB`
  return `${(size / 1024 / 1024 / 1024).toFixed(1)} GB`
}

const openPackageDialog = async () => {
  packageDialog.visible = true
  if (!packageInfo.value) await loadVectorPackage()
}

const goCmdbAgent = () => {
  router.push('/cmdb/agent')
}

const openAggregatorConfigDialog = async () => {
  aggregatorDialog.visible = true
  await loadAggregatorConfigPreview()
}

const loadAggregatorConfigPreview = async () => {
  aggregatorConfigLoading.value = true
  aggregatorConfigError.value = ''
  try {
    const response = await logtailApi.getAggregatorConfigPreview()
    aggregatorConfigPreview.value = extractData(response) || null
  } catch (error) {
    console.error('加载 Aggregator 配置预览失败:', error)
    aggregatorConfigPreview.value = null
    aggregatorConfigError.value = 'Aggregator 配置预览加载失败'
  } finally {
    aggregatorConfigLoading.value = false
  }
}

const fallbackCopyText = text => {
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.left = '-9999px'
  textarea.style.top = '-9999px'
  document.body.appendChild(textarea)
  textarea.select()
  try {
    return document.execCommand('copy')
  } finally {
    document.body.removeChild(textarea)
  }
}

const copyTextToClipboard = async (text, successMessage) => {
  const content = String(text || '')
  if (!content.trim()) {
    ElMessage.warning('暂无可复制内容')
    return
  }
  try {
    if (navigator.clipboard?.writeText && window.isSecureContext) {
      await navigator.clipboard.writeText(content)
    } else if (!fallbackCopyText(content)) {
      throw new Error('fallback copy failed')
    }
    ElMessage.success(successMessage)
  } catch (error) {
    console.error('复制失败:', error)
    ElMessage.error('复制失败，请手动复制')
  }
}

const copyAggregatorVectorYaml = () => {
  copyTextToClipboard(aggregatorVectorYaml.value, 'vector.yaml 已复制')
}

const copyAggregatorDockerCommand = () => {
  copyTextToClipboard(aggregatorDockerCommand.value, 'Docker 安装命令已复制')
}

const downloadTextFile = (content, filename, type = 'text/plain;charset=utf-8') => {
  const blob = new Blob([content], { type })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}

const downloadAggregatorK8sYaml = () => {
  const content = aggregatorKubernetesYaml.value
  if (!content) {
    ElMessage.warning('暂无可下载内容')
    return
  }
  downloadTextFile(content, aggregatorKubernetesFileName.value || 'aiops-vector-aggregator.yaml', 'application/x-yaml;charset=utf-8')
  ElMessage.success('K8s YAML 已下载')
}

const openAgentDrawer = async (row, tab = 'basic') => {
  agentDrawer.current = row
  agentDrawer.activeTab = tab
  agentDrawer.visible = true
  await loadAgentDrawerTab(tab)
}

const handleAgentMoreCommand = (row, command) => {
  if (command === 'redeployAgent') redeployCmdbAgent(row)
  if (command === 'restart') submitAgentOperation(row, 'restart_vector')
  if (command === 'stopCollect') stopCollectTask(currentTask.value)
  if (command === 'uninstall') submitAgentOperation(row, 'uninstall_vector')
}

const handleAgentDrawerTabChange = async tab => {
  await loadAgentDrawerTab(tab)
}

const loadAgentDrawerTab = async tab => {
  if (tab === 'basic') await loadAgentOperations()
  if (tab === 'config') await loadConfigPreview()
  if (tab === 'tasks') await loadAgentTasks()
  if (tab === 'operations') await loadAgentOperations()
}

const loadConfigPreview = async () => {
  const agentId = agentIdOf(agentDrawer.current)
  if (!agentId) return
  drawerLoading.config = true
  try {
    const response = await logtailApi.getAgentConfigPreview(agentId)
    configPreview.value = extractData(response) || null
  } catch (error) {
    console.error('加载 Vector 配置预览失败:', error)
    configPreview.value = { content: '' }
  } finally {
    drawerLoading.config = false
  }
}

const loadAgentTasks = async () => {
  const agentId = agentIdOf(agentDrawer.current)
  if (!agentId) return
  drawerLoading.tasks = true
  try {
    const response = await logtailApi.getAgentCollectTasks(agentId, { page: 1, pageSize: 100 })
    agentTasks.value = extractList(response)
  } catch (error) {
    console.error('加载 Agent 采集任务失败:', error)
    agentTasks.value = []
  } finally {
    drawerLoading.tasks = false
  }
}

const loadAgentOperations = async () => {
  const agentId = agentIdOf(agentDrawer.current)
  if (!agentId) return
  drawerLoading.operations = true
  try {
    const response = await logtailApi.getAgentOperations(agentId, { page: 1, pageSize: 100 })
    agentOperations.value = extractList(response)
    const failedOperation = agentOperations.value.find(item => String(item.opStatus || '').toLowerCase() === 'failed' && item.errorMsg)
    if (failedOperation && agentDrawer.current && !agentRecentError(agentDrawer.current)) {
      agentDrawer.current.operationErrorMsg = failedOperation.errorMsg
      agentDrawer.current.operationErrorAt = failedOperation.finishedAt || failedOperation.updatedAt || failedOperation.createdAt || ''
      operationErrorCache.set(String(agentId), {
        message: agentDrawer.current.operationErrorMsg,
        time: agentDrawer.current.operationErrorAt
      })
      operationErrorHydratedIds.add(String(agentId))
    }
  } catch (error) {
    console.error('加载 Agent 操作记录失败:', error)
    agentOperations.value = []
  } finally {
    drawerLoading.operations = false
  }
}

const stopOperationPolling = () => {
  if (operationPollTimer) clearInterval(operationPollTimer)
  operationPollTimer = null
  operationPollDeadline = 0
  operationPollingInFlight = false
  operationPollTargets.clear()
}

const stopCmdbAgentPolling = () => {
  if (!cmdbAgentPollTimer) return
  clearInterval(cmdbAgentPollTimer)
  cmdbAgentPollTimer = null
  cmdbAgentPollDeadline = 0
  cmdbAgentPollingInFlight = false
  cmdbAgentPollCounter = 0
}

function syncOperationPollingState(rows) {
  if (rows.some(hasCurrentOperation)) {
    startOperationPolling()
    return
  }
  stopOperationPolling()
}

function syncCmdbAgentPollingState(rows) {
  if (rows.some(isCmdbAgentDeploying)) {
    startCmdbAgentPolling()
    return
  }
  stopCmdbAgentPolling()
}

const refreshCmdbAgentState = async () => {
  await hydrateCmdbAgents(agents.value, { force: true })
  if (agentDrawer.visible && agentDrawer.current) {
    const currentKey = String(agentHostIdOf(agentDrawer.current) || agentIdOf(agentDrawer.current) || '')
    agentDrawer.current = agents.value.find(item => String(agentHostIdOf(item) || agentIdOf(item) || '') === currentKey) || agentDrawer.current
  }
  return agents.value.some(isCmdbAgentDeploying)
}

const startCmdbAgentPolling = (options = {}) => {
  const { resetDeadline = false } = options
  if (!cmdbAgentPollTimer || resetDeadline || !cmdbAgentPollDeadline) {
    cmdbAgentPollDeadline = Date.now() + 60 * 1000
  }
  if (cmdbAgentPollTimer) return

  cmdbAgentPollTimer = setInterval(async () => {
    if (cmdbAgentPollingInFlight) return
    cmdbAgentPollingInFlight = true
    try {
      const hasDeployingAgent = await refreshCmdbAgentState()
      if (hasDeployingAgent && Date.now() <= cmdbAgentPollDeadline) {
        cmdbAgentPollCounter = 0
        return
      }
      cmdbAgentPollCounter += 1
      if (!hasDeployingAgent || cmdbAgentPollCounter >= MAX_CMDB_AGENT_POLL_COUNT || Date.now() > cmdbAgentPollDeadline) {
        stopCmdbAgentPolling()
      }
    } finally {
      cmdbAgentPollingInFlight = false
    }
  }, 3000)
}

const refreshOperationState = async () => {
  if (!operationAgents.value.some(hasCurrentOperation) && !operationPollTargets.size) return false
  const rows = await fetchOperationProgressRows()
  patchOperationProgressRows(rows)
  const hasTrackedOperation = await refreshTrackedOperationProgress()
  if (agentDrawer.visible && agentDrawer.activeTab === 'operations') {
    agentDrawer.current = agents.value.find(item => agentIdOf(item) === agentIdOf(agentDrawer.current)) || agentDrawer.current
    await loadAgentOperations()
  }
  return operationAgents.value.some(hasCurrentOperation) || hasTrackedOperation
}

const startOperationPolling = (options = {}) => {
  const { resetDeadline = false } = options
  if (!operationPollTimer || resetDeadline || !operationPollDeadline) {
    operationPollDeadline = Date.now() + 60 * 1000
  }
  if (operationPollTimer) return

  operationPollTimer = setInterval(async () => {
    if (operationPollingInFlight) return
    operationPollingInFlight = true
    try {
      const hasActiveOperation = await refreshOperationState()
      if (!hasActiveOperation || Date.now() > operationPollDeadline) {
        stopOperationPolling()
      }
    } finally {
      operationPollingInFlight = false
    }
  }, 5000)
}

const redeployCmdbAgent = async row => {
  try {
    const hostId = agentHostIdOf(row)
    if (!hostId) {
      ElMessage.warning('缺少 Host ID，无法重装 Agent')
      return
    }
    await ElMessageBox.confirm(`确定要重新部署 Agent 到主机「${agentHostName(row)}」吗？`, '重装 Agent', {
      type: 'info',
      confirmButtonText: '重装',
      cancelButtonText: '取消'
    })
    await cmdbAPI.redeployAgent([hostId])
    ElMessage.success('Agent 重装任务已提交，请等待部署完成')
    await refreshCmdbAgentState()
    startCmdbAgentPolling({ resetDeadline: true })
  } catch (error) {
    if (error !== 'cancel') {
      console.error('重装 Agent 失败:', error)
      ElMessage.error(`重装 Agent 失败: ${error.response?.data?.message || error.message || '未知错误'}`)
    }
  }
}

const submitAgentOperation = async (row, opType) => {
  const agentId = agentIdOf(row)
  if (!agentId) {
    ElMessage.warning('缺少 Agent ID，无法下发操作')
    return
  }
  if (!agentOnline(row)) {
    ElMessage.warning('自研 Agent 离线，无法执行 Vector 操作')
    return
  }
  if (opType === 'install_vector') {
    if (!packageInfo.value) await loadVectorPackage()
    if (!packageReady.value) {
      ElMessage.warning('默认 Vector 包不可用，请先上传或检查后端配置')
      return
    }
  }

  const actionName = operationTypeText(opType)
  const dangerOps = ['stop_vector', 'uninstall_vector']
  await ElMessageBox.confirm(`确认对「${agentHostName(row)}」执行 ${actionName} 吗？`, actionName, {
    type: dangerOps.includes(opType) ? 'warning' : 'info',
    confirmButtonText: '确认',
    cancelButtonText: '取消'
  })
  const response = await logtailApi.createAgentOperation(agentId, {
    opType,
    operator: 'admin'
  })
  markOperationPending(row, opType, extractData(response)?.id)
  ElMessage.success(`${actionName}任务已提交，等待 Agent 执行`)
  await refreshOperationState()
  startOperationPolling({ resetDeadline: true })
}

const formatTime = value => {
  if (!value || value === '0001-01-01 00:00:00') return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN', { hour12: false }).replace(/\//g, '-')
}

onMounted(async () => {
  await loadVectorPackage()
  await loadConfigTree()
})

onBeforeUnmount(() => {
  stopPackageUploadPolling()
  stopOperationPolling()
  stopCmdbAgentPolling()
})
</script>

<style scoped>
.log-collect-page {
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
  --log-surface-warm: var(--ops-bg-surface);
  --log-surface-cool: var(--ops-bg-surface-soft);
  --log-text-primary: var(--ops-text-primary);
  --log-text-secondary: var(--ops-text-secondary);
  --log-text-muted: var(--ops-text-weak);
  --log-border: var(--ops-border-inner);
  height: calc(100vh - 124px);
  min-height: 0;
  overflow: auto;
  padding: 7px;
  background: transparent;
  color: var(--ops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

:global(.home-container .el-main:has(.log-collect-page)) {
  padding: 10px;
  overflow: hidden;
}

.page-card {
  display: flex;
  box-sizing: border-box;
  flex-direction: column;
  height: 100%;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: var(--ops-bg-surface);
  box-shadow: var(--ops-shadow-card);
}

.page-card::before {
  display: none;
}

.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-shrink: 0;
  padding: 8px 12px;
  border-bottom: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-surface);
}

.page-header-copy {
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

.page-description {
  margin: 0;
  overflow: hidden;
  color: var(--ops-text-muted);
  font-size: 12px;
  line-height: 1.5;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.summary-group {
  display: flex;
  align-items: center;
  gap: 0;
  flex-wrap: wrap;
  justify-content: flex-end;
  max-width: 920px;
  padding: 3px 4px;
  border: 1px solid var(--ops-border-soft);
  border-radius: 8px;
  background: var(--ops-bg-subtle);
}

.summary-chip {
  position: relative;
  min-width: 68px;
  padding: 2px 8px;
  border: 0;
  border-radius: 0;
  background: transparent;
}

.summary-chip + .summary-chip::before {
  content: "";
  position: absolute;
  left: 0;
  top: 5px;
  bottom: 5px;
  width: 1px;
  background: var(--ops-border);
}

.summary-chip span {
  color: var(--log-text-muted);
  font-size: 11px;
  white-space: nowrap;
}

.summary-chip strong {
  display: block;
  margin-top: 1px;
  color: var(--log-text-primary);
  font-size: 15px;
  line-height: 1.05;
}

.summary-chip.success strong {
  color: #16a34a;
}

.summary-chip.warning strong {
  color: #d97706;
}

.summary-chip.info strong {
  color: var(--ops-text-muted);
}

.summary-chip.danger strong {
  color: var(--ops-red-500);
}

.collect-workbench {
  display: grid;
  grid-template-columns: 276px minmax(0, 1fr);
  flex: 1;
  gap: 8px;
  min-width: 0;
  min-height: 0;
  padding: 8px 10px 10px;
}

.task-side-panel,
.task-agent-panel,
.task-control-card {
  border: 1px solid var(--ops-border-inner);
  background: var(--ops-bg-surface);
  box-shadow: none;
}

.task-side-panel {
  display: flex;
  min-width: 0;
  min-height: 0;
  flex-direction: column;
  padding: 8px;
  overflow: hidden;
  border-radius: 10px;
  background: var(--ops-bg-surface-soft);
}

.side-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.side-head-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex: 0 0 auto;
}

.side-head-actions :deep(.el-button) {
  width: 28px;
  height: 28px;
  min-height: 28px;
  margin-left: 0;
  padding: 0;
  border-radius: 8px;
  font-size: 14px;
}

.side-head h3 {
  margin: 0;
  color: var(--log-text-primary);
  font-size: 14px;
}

.side-head span {
  display: block;
  margin-top: 3px;
  color: var(--log-text-muted);
  font-size: 11px;
}

.tree-search {
  padding-bottom: 8px;
  border-bottom: 1px solid var(--ops-border-soft);
}

.tree-search :deep(.el-input__wrapper) {
  min-height: 32px;
}

.config-tree {
  min-height: 0;
  flex: 1;
  overflow: auto;
  padding: 8px 2px 4px 0;
}

.tree-node {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  min-height: 28px;
  margin-bottom: 3px;
  padding: 5px 7px;
  border: 1px solid transparent;
  border-radius: 7px;
  background: transparent;
  color: var(--log-text-secondary);
  text-align: left;
  cursor: pointer;
  transition: background 0.18s ease, border-color 0.18s ease, box-shadow 0.18s ease;
}

.tree-node:hover {
  border-color: var(--ops-border-hover);
  background: #f0f6ff;
}

.tree-node.active {
  border-color: var(--ops-border-hover);
  background: #eaf3ff;
  box-shadow: inset 2px 0 0 var(--ops-blue-500);
}

.node-main,
.task-node-line {
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.node-title {
  min-width: 0;
  color: var(--log-text-primary);
  font-size: 12px;
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-icon,
.node-dot,
.fold-icon {
  flex: 0 0 auto;
  display: inline-block;
}

.fold-icon {
  width: 14px;
  height: 14px;
  color: var(--ops-text-muted);
  font-size: 13px;
  transition: color 160ms ease, transform 160ms ease;
}

.fold-icon.expanded {
  color: var(--ops-blue-500);
  transform: rotate(90deg);
}

.node-icon {
  width: 14px;
  height: 14px;
  color: var(--ops-orange-500);
  font-size: 14px;
}

.store-icon {
  color: var(--ops-blue-500);
}

.node-dot {
  width: 17px;
  height: 17px;
  color: var(--ops-blue-600);
  font-size: 17px;
  object-fit: contain;
}

.node-count {
  flex: 0 0 auto;
  min-width: 20px;
  padding: 1px 6px;
  border-radius: 999px;
  color: #6681ab;
  background: rgba(239, 246, 255, 0.92);
  font-size: 11px;
  font-weight: 700;
  text-align: center;
}

.tree-children {
  position: relative;
  margin-left: 10px;
  padding-left: 8px;
  border-left: 1px solid rgba(214, 223, 235, 0.8);
}

.store-node-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 24px;
  align-items: center;
  gap: 3px;
  width: 100%;
}

.store-node-row .store-node {
  width: 100%;
}

.store-node-add {
  width: 22px;
  height: 22px;
  min-height: 22px;
  padding: 0;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.16s ease, transform 0.16s ease;
}

.store-node-row:hover .store-node-add,
.store-node-row:focus-within .store-node-add {
  opacity: 1;
  pointer-events: auto;
}

.store-node-row:hover .store-node-add {
  transform: translateX(-1px);
}

.task-children {
  margin-left: 8px;
}

.project-node .node-title {
  font-size: 13px;
  font-weight: 600;
}

.store-node .node-title {
  color: #2f4668;
}

.task-node {
  align-items: flex-start;
  padding: 5px 7px;
}

.task-node-line {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 6px;
}

.task-node-line .node-title {
  flex: 1 1 auto;
}

.task-node-main {
  min-width: 0;
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.task-node-actions {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin-left: auto;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.16s ease;
}

.task-node:hover .task-node-actions,
.task-node:focus-within .task-node-actions {
  opacity: 1;
  pointer-events: auto;
}

.task-node-actions :deep(.el-button) {
  width: 20px;
  height: 20px;
  min-height: 20px;
  padding: 0;
}

.task-agent-panel {
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0;
  overflow: hidden;
  border: 0;
  border-radius: 0;
  background: transparent;
}

.task-control-card {
  flex: 0 0 auto;
  overflow: hidden;
  border-radius: 10px;
}

.task-detail-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px 9px;
}

.control-divider {
  height: 1px;
  margin: 0 12px;
  background: rgba(214, 223, 235, 0.86);
}

.task-detail-main {
  min-width: 0;
}

.task-detail-main h3 {
  margin: 0;
  color: var(--log-text-primary);
  font-size: 15px;
  line-height: 1.2;
}

.scope-meta-grid {
  display: flex;
  align-items: center;
  gap: 5px 14px;
  flex-wrap: wrap;
  max-width: 760px;
  margin-top: 6px;
  color: var(--log-text-muted);
  font-size: 12px;
  line-height: 1.35;
}

.scope-meta-grid span {
  min-width: 0;
  max-width: 360px;
  display: inline-flex;
  align-items: center;
  gap: 3px;
}

.scope-meta-grid b {
  flex: 0 0 auto;
  color: #6b7890;
  font-weight: 700;
}

.scope-meta-grid em {
  min-width: 0;
  font-style: normal;
  color: #4d5f79;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.task-actions {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 7px;
  flex-wrap: wrap;
}

.task-actions :deep(.vector-manage-btn) {
  --el-button-bg-color: #e6a23c;
  --el-button-border-color: #e6a23c;
  --el-button-text-color: #fff;
  --el-button-hover-bg-color: #d8922f;
  --el-button-hover-border-color: #d8922f;
  --el-button-hover-text-color: #fff;
  --el-button-active-bg-color: #b45309;
  --el-button-active-border-color: #b45309;
  --el-button-active-text-color: #fff;
  color: #fff;
  font-weight: 700;
}

.task-actions :deep(.agent-deploy-btn) {
  --el-button-bg-color: #67c23a;
  --el-button-border-color: #67c23a;
  --el-button-text-color: #fff;
  --el-button-hover-bg-color: #58b832;
  --el-button-hover-border-color: #58b832;
  --el-button-hover-text-color: #fff;
  --el-button-active-bg-color: #15803d;
  --el-button-active-border-color: #15803d;
  --el-button-active-text-color: #fff;
  color: #fff;
  font-weight: 700;
}

.task-actions :deep(.aggregator-config-btn) {
  font-weight: 700;
}

.collect-tabs {
  min-height: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
}

.collect-tabs :deep(.el-tabs__header) {
  flex: 0 0 auto;
  margin: 0 0 8px;
}

.collect-tabs :deep(.el-tabs__nav-wrap::after) {
  height: 1px;
  background: rgba(214, 223, 235, 0.86);
}

.collect-tabs :deep(.el-tabs__item) {
  height: 30px;
  color: var(--log-text-secondary);
  font-size: 13px;
}

.collect-tabs :deep(.el-tabs__content) {
  display: none;
}

.content-panel {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.table-shell {
  border: 1px solid var(--ops-border-inner);
  border-radius: 8px;
  background: var(--ops-bg-surface);
  box-shadow: none;
}

.agent-toolbar-card {
  padding: 10px 12px 11px;
}

.toolbar-form {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.toolbar-form :deep(.el-form-item) {
  margin-right: 0;
  margin-bottom: 0;
}

.agent-toolbar-card .toolbar-form :deep(.el-form-item) {
  flex: 0 0 auto;
}

.agent-toolbar-card .toolbar-form :deep(.el-form-item:first-child) {
  flex: 0 1 280px;
}

.agent-toolbar-card .toolbar-form :deep(.el-form-item:first-child .el-input) {
  width: 220px;
}

.agent-toolbar-card .toolbar-form :deep(.el-select) {
  width: 84px;
}

.k8s-collector-toolbar .toolbar-form :deep(.el-form-item:first-child) {
  flex: 0 1 300px;
}

.k8s-collector-toolbar .toolbar-form :deep(.el-form-item:first-child .el-input) {
  width: 260px;
}

.k8s-collector-toolbar .toolbar-form :deep(.el-select) {
  width: 132px;
}

.k8s-collector-toolbar .toolbar-form :deep(.el-form-item:nth-child(3) .el-input) {
  width: 142px;
}

.toolbar-actions,
.operation-links {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.package-status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  padding: 0 8px;
  border-left: 1px solid rgba(214, 223, 235, 0.86);
  color: var(--log-text-secondary);
  font-size: 12px;
  flex-wrap: nowrap;
  white-space: nowrap;
}

.package-status-main,
.package-version {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  white-space: nowrap;
  word-break: keep-all;
}

.package-status strong {
  display: inline-block;
  margin-left: 3px;
  color: #2f9f6d;
  white-space: nowrap;
  word-break: keep-all;
}

.package-status.warning strong {
  color: #d8912f;
}

.table-shell {
  min-height: 0;
  flex: 1;
  overflow: hidden;
}

.data-table {
  height: 100%;
  --el-table-border-color: var(--ops-border-soft);
  --el-table-header-bg-color: var(--ops-bg-subtle);
  --el-table-row-hover-bg-color: #f8fafc;
  --el-table-text-color: var(--log-text-secondary);
  --el-table-header-text-color: var(--ops-text-heading);
}

.data-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.data-table :deep(.el-table__header th),
.data-table :deep(.el-table__body td) {
  height: 36px;
  padding: 4px 0;
}

.data-table :deep(.el-table__header th) {
  border-bottom: 1px solid var(--ops-border-soft);
  font-size: 12px;
  font-weight: 700;
}

.data-table :deep(.el-table__body td) {
  color: #44546a;
  font-size: 12px;
}

.data-table :deep(.cell) {
  line-height: 20px;
}

.name-cell strong,
.name-cell span {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.name-cell strong {
  color: var(--log-text-primary);
  font-size: 14px;
}

.name-cell span {
  margin-top: 4px;
  color: var(--log-text-muted);
  font-size: 12px;
}

.mono-text {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
}

.agent-status-progress {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.agent-status-progress-main {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.agent-status-progress-main :deep(.el-progress) {
  width: 68px;
  flex: 0 0 68px;
}

.agent-status-progress-caption {
  min-width: 0;
  color: #3faf55;
  font-size: 12px;
  line-height: 1.1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-status-progress-icon {
  width: 14px;
  height: 14px;
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid currentColor;
  border-radius: 50%;
  background: #fff;
  font-size: 10px;
  font-weight: 800;
  line-height: 1;
}

.agent-status-progress :deep(.el-progress-bar__outer) {
  background-color: rgba(214, 223, 235, 0.72);
}

.agent-status-progress-running .agent-status-progress-icon,
.agent-status-progress-success .agent-status-progress-icon {
  color: #52b83f;
}

.agent-status-progress-running :deep(.el-progress-bar__inner),
.agent-status-progress-success :deep(.el-progress-bar__inner) {
  background-color: #52b83f;
}

.agent-status-progress-failed .agent-status-progress-caption {
  color: #df5c68;
}

.agent-status-progress-failed .agent-status-progress-icon {
  color: #df5c68;
}

.agent-status-progress-failed :deep(.el-progress-bar__inner) {
  background-color: #df5c68;
}

.agent-status-progress-muted .agent-status-progress-caption,
.agent-status-progress-muted .agent-status-progress-icon {
  color: var(--log-text-muted);
}

.operation-progress {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.operation-progress-main {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.operation-progress-main :deep(.el-progress) {
  width: 68px;
  flex: 0 0 68px;
}

.operation-progress-caption {
  min-width: 0;
  color: #3faf55;
  font-size: 12px;
  line-height: 1.1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.operation-progress-icon {
  width: 14px;
  height: 14px;
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  border: 1px solid currentColor;
  background: #fff;
  font-size: 10px;
  font-weight: 800;
  line-height: 1;
}

.operation-progress :deep(.el-progress-bar__outer) {
  background-color: rgba(214, 223, 235, 0.72);
}

.operation-progress-running .operation-progress-icon,
.operation-progress-pending .operation-progress-icon {
  color: #52b83f;
}

.operation-progress-running :deep(.el-progress-bar__inner),
.operation-progress-pending :deep(.el-progress-bar__inner) {
  background-color: #52b83f;
}

.operation-progress-success .operation-progress-caption {
  color: #2f9f6d;
}

.operation-progress-success .operation-progress-icon {
  color: #2f9f6d;
}

.operation-progress-success :deep(.el-progress-bar__inner) {
  background-color: #52b83f;
}

.operation-progress-failed .operation-progress-caption {
  color: #df5c68;
}

.operation-progress-failed .operation-progress-icon {
  color: #df5c68;
}

.operation-progress-failed :deep(.el-progress-bar__inner) {
  background-color: #df5c68;
}

.success-text {
  color: #2f9f6d;
}

.warning-text {
  color: #d8912f;
}

.danger-text {
  color: #df5c68;
}

.muted-text {
  color: var(--log-text-muted);
}

.error-icon-button {
  width: 20px;
  height: 20px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 1px solid rgba(223, 92, 104, 0.36);
  border-radius: 50%;
  color: #df5c68;
  background: rgba(255, 241, 242, 0.92);
  font-size: 12px;
  font-weight: 900;
  line-height: 1;
  cursor: pointer;
  transition: background 0.16s ease, border-color 0.16s ease, transform 0.16s ease;
}

.error-icon-button:hover {
  border-color: rgba(223, 92, 104, 0.72);
  background: rgba(255, 229, 231, 0.98);
  transform: translateY(-1px);
}

.error-meta {
  margin-bottom: 12px;
}

.config-status-tag.is-running-config-not-applied {
  border-color: rgba(217, 119, 6, 0.42) !important;
  background: rgba(255, 251, 235, 0.98) !important;
  color: #d97706 !important;
}

.error-detail-content {
  max-height: 460px;
  margin: 0;
  padding: 14px;
  overflow: auto;
  border: 1px solid rgba(250, 204, 204, 0.92);
  border-radius: 10px;
  color: #9f2d3a;
  background: rgba(255, 247, 247, 0.96);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 12px;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
}

.pagination-section {
  display: flex;
  justify-content: flex-end;
  flex: 0 0 auto;
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 12px;
}

.parse-config-section {
  margin: 2px 0 14px 0;
  padding: 0;
  border: 0;
  background: transparent;
}

.parse-config-body :deep(.el-form-item) {
  margin-bottom: 10px;
}

.parse-config-body :deep(.el-textarea__inner) {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 12px;
}

.sample-log-ai-field {
  width: 100%;
}

.sample-log-form-item :deep(.el-form-item__content) {
  display: block;
  width: auto;
  margin-left: 66px !important;
  line-height: normal;
}

.sample-log-ai-head {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 6px;
  margin-bottom: 6px;
}

.sample-log-ai-label {
  color: var(--log-text-secondary);
  font-size: 13px;
  font-weight: 500;
}

.sample-log-ai-head :deep(.el-button) {
  height: 24px;
  padding: 0 10px;
  border-radius: 12px;
  font-size: 12px;
}

.sample-log-ai-field :deep(.el-textarea__inner) {
  min-height: 128px !important;
  resize: vertical;
}

@media (max-width: 960px) {
  .sample-log-form-item :deep(.el-form-item__content) {
    margin-left: 0 !important;
  }
}

.ai-parse-error {
  margin-top: 6px;
  color: #df5c68;
  font-size: 12px;
  line-height: 1.5;
}

.ai-parse-preview {
  margin-top: 10px;
  padding: 10px 12px;
  border: 1px solid rgba(147, 197, 253, 0.72);
  border-radius: 12px;
  background: rgba(239, 246, 255, 0.76);
  color: var(--log-text-secondary);
  font-size: 12px;
  line-height: 1.65;
}

.ai-parse-preview-head,
.ai-parse-summary,
.ai-parse-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.ai-parse-preview-head {
  justify-content: space-between;
  margin-bottom: 4px;
}

.ai-parse-preview-head strong,
.ai-parse-line b,
.ai-parse-summary b {
  color: var(--log-text-primary);
}

.ai-parse-summary,
.ai-parse-line {
  flex-wrap: wrap;
  word-break: break-word;
}

.ai-parse-warning {
  margin-top: 4px;
  color: #d97706;
}

.ai-parse-actions {
  justify-content: flex-end;
  margin-top: 8px;
}

.task-status-toggle.is-status-enabled {
  --el-color-primary: #22c55e;
  --el-radio-button-checked-bg-color: #22c55e;
  --el-radio-button-checked-border-color: #22c55e;
  --el-radio-button-checked-text-color: #fff;
}

.task-status-toggle.is-status-disabled {
  --el-color-primary: #df5c68;
  --el-radio-button-checked-bg-color: #df5c68;
  --el-radio-button-checked-border-color: #df5c68;
  --el-radio-button-checked-text-color: #fff;
}

.task-status-toggle.is-status-enabled :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) {
  border-color: #22c55e !important;
  background-color: #22c55e !important;
  box-shadow: -1px 0 0 0 #22c55e !important;
  color: #fff !important;
}

.task-status-toggle.is-status-disabled :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) {
  border-color: #df5c68 !important;
  background-color: #df5c68 !important;
  box-shadow: -1px 0 0 0 #df5c68 !important;
  color: #fff !important;
}

.agent-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.agent-option strong {
  min-width: 0;
  color: var(--log-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.agent-option span {
  flex: 0 0 auto;
  color: var(--log-text-muted);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 12px;
}

.k8s-cluster-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.k8s-cluster-option strong {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.k8s-cluster-option span {
  flex: 0 0 auto;
  color: #df8b1f;
  font-size: 12px;
}

.k8s-cluster-option span.is-ready {
  color: #22a45d;
}

.path-pattern-editor {
  width: 100%;
  display: grid;
  gap: 8px;
}

.multiline-config {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 10px;
}

.multiline-pattern-editor {
  width: 100%;
  display: grid;
  gap: 6px;
}

.multiline-pattern-editor :deep(.el-textarea__inner) {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 12px;
  resize: vertical;
}

.path-pattern-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 34px 34px;
  align-items: center;
  gap: 8px;
}

.path-pattern-row :deep(.el-input__wrapper) {
  min-height: 36px;
  border-radius: 8px;
}

.path-pattern-row :deep(.el-input__inner) {
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 12px;
}

.path-pattern-row .el-button {
  width: 34px;
  height: 34px;
  min-height: 34px;
}

.path-editor-hint {
  margin-top: 0;
  color: var(--log-text-muted);
  font-size: 12px;
  line-height: 1.45;
}

.k8s-log-source-group {
  display: flex;
  flex-wrap: wrap;
  gap: 10px 16px;
}

.k8s-stdout-hint {
  margin: -2px 0 14px 118px;
  color: var(--log-text-muted);
  font-size: 12px;
  line-height: 1.45;
}

.path-pattern-tips {
  margin: -2px 0 2px 96px;
  padding: 10px 12px;
  border: 1px solid #dce7f5;
  border-radius: 10px;
  background: var(--ops-bg-surface-soft);
}

.path-tips-title {
  margin-bottom: 7px;
  color: var(--log-text-secondary);
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0;
}

.path-tip-list {
  display: grid;
  gap: 6px;
}

.path-tip-item {
  display: grid;
  grid-template-columns: 68px minmax(0, 1fr);
  align-items: center;
  gap: 8px;
  font-size: 12px;
}

.path-tip-item span {
  color: #6f7f95;
  font-weight: 700;
}

.path-tip-item code {
  min-width: 0;
  padding: 4px 8px;
  overflow: hidden;
  border: 1px solid rgba(92, 148, 230, 0.22);
  border-radius: 9px;
  color: #244a78;
  background: rgba(239, 246, 255, 0.86);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.drawer-toolbar {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-bottom: 12px;
}

.detail-operations-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 14px 0 8px;
}

.detail-operations-head h4 {
  margin: 0;
  color: var(--log-text-primary);
  font-size: 14px;
}

.config-preview-editor {
  overflow: hidden;
  border: 1px solid var(--log-border);
  border-radius: 10px;
  background: var(--ops-bg-surface-soft);
}

.config-preview-editor :deep(.monaco-editor-container) {
  min-height: 360px;
  border: 0;
  border-radius: 10px;
}

.config-preview-editor :deep(.monaco-editor-wrapper) {
  border-radius: 10px;
}

.k8s-collector-detail {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.k8s-detail-progress {
  display: grid;
  grid-template-columns: minmax(160px, 1fr) auto;
  align-items: center;
  gap: 12px;
}

.k8s-detail-progress span {
  color: var(--log-text-secondary);
  font-size: 12px;
  white-space: nowrap;
}

.package-alert {
  margin-bottom: 12px;
}

.package-upload-progress {
  margin-top: 12px;
  padding: 12px;
  border: 1px solid rgba(214, 223, 235, 0.96);
  border-radius: 12px;
  background: rgba(248, 251, 255, 0.82);
}

.package-upload-progress-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
  color: var(--log-text-secondary);
  font-size: 12px;
}

.package-upload-progress-head strong {
  min-width: 0;
  color: var(--log-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.package-upload-progress-head span {
  flex: 0 0 auto;
  color: var(--log-text-muted);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
}

.aggregator-config-panel {
  margin-top: 12px;
  padding: 12px;
  border: 1px solid rgba(214, 223, 235, 0.96);
  border-radius: 10px;
  background: var(--ops-bg-surface-soft);
}

.aggregator-config-panel--dialog {
  margin-top: 0;
}

.aggregator-config-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.aggregator-config-head h4 {
  margin: 0;
  color: var(--log-text-primary);
  font-size: 14px;
  line-height: 1.25;
}

.aggregator-config-head span {
  display: block;
  max-width: 480px;
  margin-top: 4px;
  color: var(--log-text-muted);
  font-family: "SFMono-Regular", Consolas, "Liberation Mono", monospace;
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.aggregator-config-alert {
  margin-bottom: 10px;
}

.aggregator-action-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.aggregator-meta-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-top: 10px;
}

.aggregator-meta-grid span {
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 9px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.82);
  color: var(--log-text-secondary);
  font-size: 12px;
}

.aggregator-meta-grid b {
  flex: 0 0 auto;
  color: #6b7890;
}

.aggregator-meta-grid em {
  min-width: 0;
  font-style: normal;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.aggregator-config-editor {
  margin-top: 12px;
  overflow: hidden;
  border: 1px solid var(--log-border);
  border-radius: 10px;
}

.aggregator-config-editor :deep(.monaco-editor-container),
.aggregator-config-editor :deep(.monaco-editor-wrapper) {
  border-radius: 10px;
}

.official-download-link {
  color: #3478f6;
  font-weight: 700;
  text-decoration: none;
}

.official-download-link:hover {
  text-decoration: underline;
}

.hidden-file-input {
  display: none;
}

:global(.log-agent-dialog-overlay),
:global(.log-drawer.el-drawer) {
  --ops-blue-500: #409eff;
  --ops-blue-600: #2563eb;
  --ops-green-500: #10b981;
  --ops-orange-500: #f59e0b;
  --ops-red-500: #ef4444;
  --ops-button-success: #67c23a;
  --ops-button-warning: #e6a23c;
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
  --log-text-primary: var(--ops-text-primary);
  --log-text-secondary: var(--ops-text-secondary);
  --log-text-muted: var(--ops-text-weak);
  --log-border: var(--ops-border-inner);
}

:global(.log-agent-dialog-overlay .el-dialog) {
  width: min(var(--el-dialog-width), calc(100vw - 24px));
  max-height: calc(100vh - 24px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: 15px !important;
  background: var(--ops-bg-surface);
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.log-agent-dialog-overlay .el-dialog__header) {
  margin-right: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
}

:global(.log-agent-dialog-overlay .el-dialog__title) {
  color: var(--ops-text-heading);
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.log-agent-dialog-overlay .el-dialog__headerbtn) {
  top: 8px;
  right: 10px;
  width: 34px;
  height: 34px;
  border-radius: 8px;
}

:global(.log-agent-dialog-overlay .el-dialog__headerbtn:hover) {
  background: var(--ops-bg-subtle);
}

:global(.log-agent-dialog-overlay .el-dialog__body) {
  max-height: calc(100vh - 154px);
  overflow: auto;
  padding: 16px 18px;
  color: var(--ops-text-primary);
}

:global(.log-agent-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
}

:global(.log-agent-dialog-overlay .el-form-item) {
  margin-bottom: 12px;
}

:global(.log-agent-dialog-overlay .el-form-item__label) {
  color: var(--ops-text-secondary);
  font-size: 12px;
  font-weight: 700;
}

:global(.log-agent-dialog-overlay .k8s-workload-form-item.el-form-item) {
  display: grid !important;
  grid-template-columns: 118px minmax(0, 1fr);
  align-items: start;
}

:global(.log-agent-dialog-overlay .k8s-workload-form-item > .el-form-item__label) {
  width: auto !important;
  grid-column: 1;
}

:global(.log-agent-dialog-overlay .k8s-workload-form-item > .el-form-item__content) {
  width: auto !important;
  min-width: 0;
  margin-left: 0 !important;
  grid-column: 2;
}

:global(.log-agent-dialog-overlay .el-input__wrapper),
:global(.log-agent-dialog-overlay .el-select__wrapper),
:global(.log-agent-dialog-overlay .el-textarea__inner),
:global(.log-agent-dialog-overlay .el-input-number .el-input__wrapper) {
  border-radius: 8px;
  background: var(--ops-bg-surface-soft);
  box-shadow: inset 0 0 0 1px var(--ops-border-inner) !important;
}

:global(.log-agent-dialog-overlay .el-button) {
  border-radius: 8px;
  font-weight: 600;
}

:global(.log-drawer.el-drawer) {
  max-width: calc(100vw - 12px);
  border-left: 1px solid #d7e6f6;
  background: var(--ops-bg-surface);
  box-shadow: -18px 0 52px rgba(49, 92, 142, 0.16);
}

:global(.log-drawer .el-drawer__header) {
  margin-bottom: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  color: var(--ops-text-heading);
  font-size: 17px;
  font-weight: 700;
}

:global(.log-drawer .el-drawer__body) {
  overflow: auto;
  padding: 12px 14px 16px;
  background: var(--ops-bg-surface);
}

:global(.log-drawer .el-drawer__close-btn) {
  width: 34px;
  height: 34px;
  border-radius: 8px;
}

.log-collect-page :deep(.el-input__wrapper),
.log-collect-page :deep(.el-select__wrapper),
.log-collect-page :deep(.el-textarea__inner),
.log-collect-page :deep(.el-input-number .el-input__wrapper) {
  border-radius: 8px;
  background: var(--ops-bg-surface-soft);
  box-shadow: inset 0 0 0 1px var(--ops-border-inner) !important;
}

.log-collect-page :deep(.el-input__wrapper.is-focus),
.log-collect-page :deep(.el-select__wrapper.is-focused) {
  box-shadow: inset 0 0 0 1px var(--ops-blue-500) !important;
}

.log-collect-page :deep(.el-button) {
  border-radius: 8px !important;
  font-weight: 600;
  transition:
    color 160ms ease,
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease,
    transform 160ms ease;
}

.log-collect-page :deep(.el-button--primary:not(.is-link)) {
  border-color: var(--ops-blue-500);
  color: #fff;
  background: var(--ops-blue-500);
}

.log-collect-page :deep(.el-button--success:not(.is-link)) {
  border-color: var(--ops-button-success);
  color: #fff;
  background: var(--ops-button-success);
}

.log-collect-page :deep(.el-button--warning:not(.is-link)) {
  border-color: var(--ops-button-warning);
  color: #fff;
  background: var(--ops-button-warning);
}

.log-collect-page :deep(.el-button--primary:not(.is-link):not(.is-disabled):hover) {
  border-color: #337ecc;
  background: #337ecc;
  transform: translateY(-1px);
}

.log-collect-page :deep(.el-button--success:not(.is-link):not(.is-disabled):hover) {
  border-color: var(--ops-button-success-hover);
  background: var(--ops-button-success-hover);
  transform: translateY(-1px);
}

.log-collect-page :deep(.el-button--warning:not(.is-link):not(.is-disabled):hover) {
  border-color: var(--ops-button-warning-hover);
  background: var(--ops-button-warning-hover);
  transform: translateY(-1px);
}

.log-collect-page :deep(.el-button:active) {
  transform: translateY(0);
}

.operation-links :deep(.el-button) {
  margin-left: 0;
  padding: 0;
}

.dropdown-action-item {
  min-width: 104px;
  display: inline-flex;
  align-items: center;
  gap: 7px;
}

.dropdown-action-item img {
  width: 14px;
  height: 14px;
  flex: 0 0 auto;
}

.dropdown-action-item :deep(.el-icon) {
  width: 14px;
  height: 14px;
  flex: 0 0 auto;
}

.dropdown-action-item.danger {
  color: #df5c68;
}

@media (max-width: 960px) {
  .page-header {
    align-items: stretch;
    flex-direction: column;
  }

  .summary-group {
    width: 100%;
    max-width: none;
    justify-content: flex-start;
  }

  .collect-workbench {
    grid-template-columns: 1fr;
    overflow: visible;
  }

  .page-card {
    height: auto;
    min-height: 100%;
    overflow: visible;
  }

  .task-side-panel {
    max-height: 360px;
  }

  .task-detail-card {
    align-items: stretch;
    flex-direction: column;
  }

  .form-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .ops-page-header__content {
    align-items: flex-start;
    flex-direction: column;
    gap: 2px;
  }

  .ops-page-header__divider {
    display: none;
  }

  .task-actions,
  .toolbar-form {
    align-items: stretch;
    flex-direction: column;
  }

  .agent-toolbar-card .toolbar-form :deep(.el-form-item),
  .agent-toolbar-card .toolbar-form :deep(.el-input),
  .agent-toolbar-card .toolbar-form :deep(.el-select) {
    width: 100%;
  }

  .package-form-item {
    width: 100%;
  }

  .package-status {
    width: 100%;
    border-left: 0;
  }
}

@media (max-width: 620px) {
  .log-collect-page {
    padding: 6px;
  }

  .page-header {
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

  .summary-chip {
    flex: 1 1 88px;
  }

  .summary-chip + .summary-chip::before {
    display: none;
  }

  .collect-workbench {
    padding-right: 8px;
    padding-left: 8px;
  }

  .path-pattern-row {
    grid-template-columns: minmax(0, 1fr) 30px 30px;
    gap: 6px;
  }

  .path-pattern-row .el-button {
    width: 30px;
    height: 30px;
    min-height: 30px;
  }

  :global(.log-agent-dialog-overlay .el-dialog) {
    margin-top: 12px;
  }

  :global(.log-agent-dialog-overlay .el-dialog__body) {
    max-height: calc(100vh - 146px);
    padding: 14px;
  }
}
</style>
