<template>
  <div class="business-topology-page">
    <section class="page-card">
      <header class="page-header">
        <div>
          <h2>{{ businessTitle }}</h2>
          <p class="page-description">{{ businessSummary }}</p>
        </div>
        <div class="header-actions">
          <el-button class="action-button action-button--back" @click="goBack">
            <el-icon><ArrowLeft /></el-icon>
            返回
          </el-button>
          <template v-if="isEditing">
            <el-button class="action-button action-button--secondary" :disabled="!canUndo" @click="undoDraft">
              <el-icon><RefreshLeft /></el-icon>
              撤销
            </el-button>
            <el-button class="action-button action-button--secondary" :disabled="!canRedo" @click="redoDraft">
              <el-icon><RefreshRight /></el-icon>
              重做
            </el-button>
            <el-button class="action-button action-button--warning" @click="cancelEditMode">
              <el-icon><Close /></el-icon>
              取消
            </el-button>
            <el-button class="action-button action-button--confirm" :loading="savingTopology" @click="saveTopologyDraft">
              <el-icon><Check /></el-icon>
              保存拓扑
            </el-button>
          </template>
          <el-button v-else class="action-button action-button--success" @click="enterEditMode">
            <el-icon><Edit /></el-icon>
            编辑拓扑
          </el-button>
          <el-button v-if="!isEditing" class="action-button action-button--warning" :loading="refreshing" @click="handleRefreshHealth">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </header>

      <section class="status-strip">
        <article v-for="item in statusCards" :key="item.key" class="status-card">
          <span>{{ item.label }}</span>
          <strong :style="{ color: item.color }">{{ item.value }}</strong>
        </article>
      </section>

      <main class="topology-layout">
        <section class="graph-panel">
          <div class="graph-toolbar">
            <div class="legend-row">
              <span v-for="item in legend" :key="item.key">
                <i :style="{ backgroundColor: item.color }"></i>
                {{ item.label }}
              </span>
            </div>
            <div v-if="isEditing" class="toolbar-actions">
              <span class="edit-change-pill">{{ draftChanges.length }} 项草稿</span>
              <el-button class="toolbar-edit-button action-button--success" size="small" @click="openDeploymentWizard('k8s')">
                <el-icon><Plus /></el-icon>
                添加部署
              </el-button>
            </div>
            <div v-else class="toolbar-actions">
              <el-select v-model="topologyQuery.direction" size="small" @change="loadTopology">
                <el-option label="下游" value="downstream" />
                <el-option label="双向" value="both" />
                <el-option label="上游" value="upstream" />
              </el-select>
              <el-select v-model="topologyQuery.depth" size="small" @change="loadTopology">
                <el-option v-for="depth in [2, 3, 4, 5, 6, 7, 8]" :key="depth" :label="`${depth} 层`" :value="depth" />
              </el-select>
              <el-button class="toolbar-edit-button action-button--success" size="small" @click="enterEditMode">
                绑定资源
              </el-button>
            </div>
          </div>

          <div
            class="graph-shell"
            v-loading="topologyLoading"
            @click="closeContextMenu"
            @contextmenu.prevent
          >
            <div v-if="!topologyLoading && !topologyData.nodes.length" class="empty-state">
              <el-empty description="暂无业务拓扑数据">
                <el-button class="action-button action-button--success" @click="enterEditMode">编辑拓扑并绑定资源</el-button>
              </el-empty>
            </div>
            <div ref="graphContainer" class="graph-canvas"></div>

            <div
              v-if="contextMenu.visible"
              class="topology-context-menu"
              :style="{ left: `${contextMenu.x}px`, top: `${contextMenu.y}px` }"
              @click.stop
            >
              <button
                v-for="item in contextMenuItems"
                :key="item.key"
                type="button"
                :class="{ danger: item.danger }"
                @click="handleContextMenuAction(item.key)"
              >
                {{ item.label }}
              </button>
            </div>
          </div>
        </section>

        <aside class="side-panel">
          <section v-if="isEditing" class="panel-section draft-section">
            <h3>草稿变更</h3>
            <div v-if="draftChanges.length" class="draft-list">
              <div v-for="item in draftChanges.slice().reverse().slice(0, 6)" :key="item.id">
                <strong>{{ draftChangeText(item) }}</strong>
                <span>{{ draftChangeSubText(item) }}</span>
              </div>
            </div>
            <div v-else class="empty-risk">当前暂无未保存变更</div>
          </section>

          <section class="panel-section">
            <h3>当前节点详情</h3>
            <div v-if="selectedNode" class="node-detail">
              <div class="node-title" :style="{ '--node-color': resolveHealthColor(selectedNode.healthStatus, selectedNode.healthColor) }">
                <span></span>
                <div>
                  <strong>{{ selectedNode.displayName || selectedNode.label || selectedNode.name }}</strong>
                  <p>{{ resourceTypeText(selectedNode.resourceType) }}</p>
                </div>
                <em>{{ formatScore(selectedNode.healthScore) }}</em>
              </div>

              <div class="detail-list">
                <div>
                  <span>状态</span>
                  <strong>{{ healthStatusText(selectedNode.healthStatus) }}</strong>
                </div>
                <div>
                  <span>来源</span>
                  <strong>{{ selectedNode.sourceType || '-' }}</strong>
                </div>
                <div>
                  <span>资源标识</span>
                  <strong>{{ selectedNode.sourceKey || selectedNode.sourceId || '-' }}</strong>
                </div>
              </div>

              <div v-if="!isObservabilityNode(selectedNode)" class="observability-overview" aria-label="可观测状态摘要">
                <div>
                  <span>日志</span>
                  <strong>{{ selectedObservability.logCount }}</strong>
                </div>
                <div :style="{ '--observation-color': resolveHealthColor(selectedObservability.monitorStatus) }">
                  <span>监控</span>
                  <strong>{{ selectedObservability.monitorText }}</strong>
                </div>
                <div :class="{ alerting: selectedObservability.alertCount > 0 }">
                  <span>告警</span>
                  <strong>{{ selectedObservability.alertCount }}</strong>
                </div>
              </div>

              <div v-if="!isObservabilityNode(selectedNode)" class="observation-details">
                <section class="observation-block">
                  <header>
                    <h4>关联日志</h4>
                    <el-button
                      v-if="selectedBoundLogs.length && !isEditing"
                      link
                      type="primary"
                      @click="toggleSelectedLogs"
                    >
                      {{ selectedLogsExpanded ? '收起日志' : '展开日志' }}
                    </el-button>
                  </header>
                  <ul v-if="selectedBoundLogs.length" class="bound-log-list">
                    <li v-for="item in selectedBoundLogs" :key="item.id || item.sourceKey || item.name">
                      <img :src="logIcon" alt="" />
                      <div>
                        <strong>{{ item.displayName || item.label || item.name }}</strong>
                        <span>{{ resourceTypeText(item.resourceType || item.type) }}</span>
                      </div>
                    </li>
                  </ul>
                  <p v-else class="observation-empty">未绑定日志资源</p>
                </section>

                <section class="observation-block">
                  <header><h4>监控</h4></header>
                  <div class="monitor-metric-grid">
                    <div v-for="metric in selectedMonitorMetrics" :key="metric.key">
                      <span>{{ metric.label }}</span>
                      <strong>{{ metric.value }}</strong>
                    </div>
                  </div>
                  <p class="monitor-match">
                    <span>自动匹配</span>
                    <code>{{ selectedMonitorMatchText }}</code>
                  </p>
                </section>

                <section class="observation-block">
                  <header><h4>告警</h4></header>
                  <div class="alert-summary-row">
                    <div>
                      <span>当前活跃</span>
                      <strong :class="{ alerting: selectedObservability.alertCount > 0 }">
                        {{ selectedObservability.alertCount }} 条
                      </strong>
                    </div>
                    <div>
                      <span>来源</span>
                      <strong>{{ selectedObservability.alertSource }}</strong>
                    </div>
                  </div>
                </section>
              </div>

              <p class="node-message">{{ selectedNode.problemSummary || selectedNode.message || '暂无问题摘要' }}</p>

              <div v-if="nodeEvidenceText" class="evidence-box">
                <span>问题证据</span>
                <pre>{{ nodeEvidenceText }}</pre>
              </div>

              <div class="dimension-stack">
                <div
                  v-for="item in normalizeNodeDimensions(selectedNode)"
                  :key="item.dimension"
                  :style="{ '--dimension-color': resolveHealthColor(item.status, item.color) }"
                >
                  <span>{{ item.label || dimensionText(item.dimension) }}</span>
                  <strong>{{ formatDimensionScore(item) }}</strong>
                </div>
              </div>

              <el-button
                v-if="canOpenNodeDetail(selectedNode)"
                class="node-detail-button action-button action-button--secondary"
                @click="openNodeDetail(selectedNode)"
              >
                <el-icon><View /></el-icon>
                查看详情
              </el-button>

              <div class="diagnose-actions">
                <el-button
                  v-for="action in diagnoseActions(selectedNode)"
                  :key="action.label"
                  link
                  type="primary"
                  @click="goDiagnose(action)"
                >
                  {{ action.label }}
                </el-button>
              </div>
            </div>
            <div v-else class="empty-risk">点击拓扑节点查看详情</div>
          </section>

          <section class="panel-section">
            <h3>资源统计</h3>
            <div class="asset-grid">
              <article v-for="item in assetCards" :key="item.key">
                <span>{{ item.label }}</span>
                <strong>{{ item.value }}</strong>
              </article>
            </div>
          </section>

          <section class="panel-section">
            <h3>风险节点</h3>
            <div class="risk-list">
              <button
                v-for="item in riskNodes"
                :key="item.id"
                type="button"
                class="risk-item"
                :style="{ '--risk-color': resolveHealthColor(item.healthStatus, item.healthColor) }"
                @click="focusOrOpenNode(item.id)"
              >
                <strong>{{ item.displayName || item.name }}</strong>
                <span>{{ resourceTypeText(item.resourceType) }} · {{ healthStatusText(item.healthStatus) }}</span>
                <p>{{ item.message || item.problemSummary || '暂无异常描述' }}</p>
              </button>
              <div v-if="!riskNodes.length" class="empty-risk">暂无黄色或红色风险节点</div>
            </div>
          </section>
        </aside>
      </main>
    </section>

    <el-dialog v-model="deploymentDialog.visible" :title="deploymentDialogTitle" width="620px" class="resource-edit-dialog" destroy-on-close>
      <div v-if="deploymentDialog.type === 'k8s'" class="resource-dialog-body">
        <el-select
          v-model="deploymentDialog.k8s.clusterId"
          filterable
          placeholder="选择 K8S 集群"
          :loading="deploymentDialog.k8s.loadingClusters"
          @change="handleK8sClusterChange"
        >
          <el-option
            v-for="cluster in deploymentDialog.k8s.clusters"
            :key="cluster.id"
            :label="cluster.displayName || cluster.name"
            :value="cluster.id"
          />
        </el-select>
        <el-select
          v-model="deploymentDialog.k8s.namespaceName"
          filterable
          placeholder="选择命名空间"
          :disabled="!deploymentDialog.k8s.clusterId"
          :loading="deploymentDialog.k8s.loadingNamespaces"
          @change="handleK8sNamespaceChange"
        >
          <el-option
            v-for="namespace in deploymentDialog.k8s.namespaces"
            :key="namespace.name"
            :label="namespace.name"
            :value="namespace.name"
          />
        </el-select>
        <el-select
          v-model="deploymentDialog.k8s.targetKeys"
          multiple
          collapse-tags
          collapse-tags-tooltip
          filterable
          remote
          placeholder="选择工作负载，可多选"
          :disabled="!deploymentDialog.k8s.namespaceName"
          :loading="deploymentDialog.k8s.loadingTargets"
          :remote-method="remoteSearchK8sTargets"
        >
          <el-option
            v-if="deploymentDialog.k8s.namespaceName && !deploymentDialog.k8s.loadingTargets && !deploymentDialog.k8s.targets.length"
            disabled
            label="当前命名空间暂无工作负载"
            value="__empty__"
          />
          <el-option
            v-for="target in deploymentDialog.k8s.targets"
            :key="target.key"
            :label="`${target.name} · ${target.workloadType || target.kind}`"
            :value="target.key"
          >
            <div class="k8s-target-option">
              <strong>{{ target.name }}</strong>
              <span>{{ target.workloadType || target.kind }} · {{ target.status || 'Unknown' }}</span>
            </div>
          </el-option>
        </el-select>
        <p class="k8s-selector-tip">默认选择稳定的工作负载，Pod 健康状态后续在节点详情中展示。</p>
      </div>

      <div v-else-if="deploymentDialog.type === 'host'" class="resource-dialog-body">
        <el-select
          v-model="deploymentDialog.host.hostKey"
          filterable
          remote
          placeholder="输入主机名称 / IP 搜索"
          :loading="deploymentDialog.host.loadingHosts"
          :remote-method="remoteSearchHosts"
          @change="handleHostSelectionChange"
        >
          <el-option
            v-if="!deploymentDialog.host.loadingHosts && !deploymentDialog.host.options.length"
            disabled
            label="输入主机名或 IP 搜索"
            value="__empty__"
          />
          <el-option
            v-for="host in deploymentDialog.host.options"
            :key="host.key"
            :label="host.label"
            :value="host.key"
          >
            <div class="host-target-option">
              <strong>{{ host.displayName }}</strong>
              <span>{{ host.ip || '-' }} · {{ host.os || '未知系统' }}</span>
            </div>
          </el-option>
        </el-select>
        <el-select
          v-model="deploymentDialog.host.serviceNames"
          multiple
          collapse-tags
          collapse-tags-tooltip
          clearable
          filterable
          placeholder="选择服务，可多选；留空表示监控该主机全部服务"
          :disabled="!deploymentDialog.host.hostKey"
          :loading="deploymentDialog.host.loadingServices"
        >
          <el-option
            v-if="!deploymentDialog.host.loadingServices && deploymentDialog.host.hostKey && !deploymentDialog.host.services.length"
            disabled
            label="该主机暂未返回服务指标，保存后按主机服务覆盖状态判断"
            value="__empty__"
          />
          <el-option
            v-for="service in deploymentDialog.host.services"
            :key="service.name"
            :label="`${service.name} · CPU ${formatPercent(service.cpu)} / 内存 ${formatPercent(service.memory)}`"
            :value="service.name"
          >
            <div class="service-target-option">
              <strong>{{ service.name }}</strong>
              <span>CPU {{ formatPercent(service.cpu) }} · 内存 {{ formatPercent(service.memory) }}</span>
            </div>
          </el-option>
        </el-select>
        <p class="k8s-selector-tip">不选择服务时，将监控这台主机上已采集到的全部服务 CPU / 内存。</p>
      </div>

      <div v-else-if="deploymentDialog.type === 'child'" class="resource-dialog-body">
        <div class="sibling-hint">
          <span>从当前节点延伸</span>
          <strong>{{ deploymentDialog.parentNode?.displayName || deploymentDialog.parentNode?.name || '-' }}</strong>
          <em>新增 {{ resourceTypeText(deploymentDialog.child.resourceType) }}</em>
        </div>
        <el-select
          v-if="deploymentDialog.child.resourceType === 'k8s_namespace'"
          v-model="deploymentDialog.child.namespaceName"
          filterable
          placeholder="选择命名空间"
          :loading="deploymentDialog.child.loadingNamespaces"
        >
          <el-option
            v-if="!deploymentDialog.child.loadingNamespaces && !deploymentDialog.child.namespaces.length"
            disabled
            label="当前集群暂无命名空间"
            value="__empty__"
          />
          <el-option
            v-for="namespace in deploymentDialog.child.namespaces"
            :key="namespace.name"
            :label="namespace.name"
            :value="namespace.name"
          />
        </el-select>
        <el-select
          v-else-if="deploymentDialog.child.resourceType === 'k8s_workload'"
          v-model="deploymentDialog.child.targetKeys"
          multiple
          collapse-tags
          collapse-tags-tooltip
          filterable
          remote
          placeholder="选择工作负载，可多选"
          :loading="deploymentDialog.child.loadingTargets"
          :remote-method="remoteSearchChildK8sTargets"
        >
          <el-option
            v-if="!deploymentDialog.child.loadingTargets && !deploymentDialog.child.targets.length"
            disabled
            label="输入至少 2 个字符搜索工作负载"
            value="__empty__"
          />
          <el-option
            v-for="target in deploymentDialog.child.targets"
            :key="target.key"
            :label="`${target.name} · ${target.workloadType || target.kind}`"
            :value="target.key"
          >
            <div class="k8s-target-option">
              <strong>{{ target.name }}</strong>
              <span>{{ target.workloadType || target.kind }} · {{ target.status || 'Unknown' }}</span>
            </div>
          </el-option>
        </el-select>
        <el-select
          v-else-if="deploymentDialog.child.resourceType === 'business_service'"
          v-model="deploymentDialog.child.serviceNames"
          multiple
          collapse-tags
          collapse-tags-tooltip
          filterable
          placeholder="选择服务，可多选"
          :loading="deploymentDialog.child.loadingServices"
        >
          <el-option
            v-if="!deploymentDialog.child.loadingServices && !deploymentDialog.child.services.length"
            disabled
            label="该主机暂未返回服务指标"
            value="__empty__"
          />
          <el-option
            v-for="service in deploymentDialog.child.services"
            :key="service.name"
            :label="`${service.name} · CPU ${formatPercent(service.cpu)} / 内存 ${formatPercent(service.memory)}`"
            :value="service.name"
          >
            <div class="service-target-option">
              <strong>{{ service.name }}</strong>
              <span>CPU {{ formatPercent(service.cpu) }} · 内存 {{ formatPercent(service.memory) }}</span>
            </div>
          </el-option>
        </el-select>
      </div>

      <div v-else-if="deploymentDialog.type === 'sibling'" class="resource-dialog-body">
        <div class="sibling-hint">
          <span>从上级节点延伸</span>
          <strong>{{ deploymentDialog.parentNode?.displayName || deploymentDialog.parentNode?.name || '项目根节点' }}</strong>
          <em>新增 {{ resourceTypeText(deploymentDialog.sibling.resourceType) }} 同级节点</em>
        </div>
        <el-select
          v-if="deploymentDialog.sibling.useK8sSearch"
          v-model="deploymentDialog.sibling.targetKeys"
          multiple
          collapse-tags
          collapse-tags-tooltip
          filterable
          remote
          placeholder="搜索同命名空间工作负载，可多选"
          :loading="deploymentDialog.sibling.loadingTargets"
          :remote-method="remoteSearchSiblingK8sTargets"
        >
          <el-option
            v-if="!deploymentDialog.sibling.loadingTargets && !deploymentDialog.sibling.targets.length"
            disabled
            label="输入至少 2 个字符搜索工作负载"
            value="__empty__"
          />
          <el-option
            v-for="target in deploymentDialog.sibling.targets"
            :key="target.key"
            :label="`${target.name} · ${target.workloadType || target.kind}`"
            :value="target.key"
          >
            <div class="k8s-target-option">
              <strong>{{ target.name }}</strong>
              <span>{{ target.workloadType || target.kind }} · {{ target.status || 'Unknown' }}</span>
            </div>
          </el-option>
        </el-select>
        <el-select
          v-else-if="isHostResourceType(deploymentDialog.sibling.resourceType)"
          v-model="deploymentDialog.sibling.hostKeys"
          multiple
          collapse-tags
          collapse-tags-tooltip
          filterable
          remote
          placeholder="输入主机名称 / IP 搜索，可多选"
          :loading="deploymentDialog.sibling.loadingHosts"
          :remote-method="remoteSearchSiblingHosts"
        >
          <el-option
            v-if="!deploymentDialog.sibling.loadingHosts && !deploymentDialog.sibling.hosts.length"
            disabled
            label="输入至少 2 个字符搜索主机"
            value="__empty__"
          />
          <el-option
            v-for="host in deploymentDialog.sibling.hosts"
            :key="host.key"
            :label="host.label"
            :value="host.key"
          >
            <div class="host-target-option">
              <strong>{{ host.displayName }}</strong>
              <span>{{ host.ip || '无 IP' }}</span>
            </div>
          </el-option>
        </el-select>
        <el-select
          v-else-if="isServiceResourceType(deploymentDialog.sibling.resourceType)"
          v-model="deploymentDialog.sibling.serviceNames"
          multiple
          collapse-tags
          collapse-tags-tooltip
          filterable
          placeholder="选择同主机服务，可多选"
          :loading="deploymentDialog.sibling.loadingServices"
        >
          <el-option
            v-if="!deploymentDialog.sibling.loadingServices && !deploymentDialog.sibling.services.length"
            disabled
            label="该主机暂未返回服务指标"
            value="__empty__"
          />
          <el-option
            v-for="service in deploymentDialog.sibling.services"
            :key="service.name"
            :label="`${service.name} · CPU ${formatPercent(service.cpu)} / 内存 ${formatPercent(service.memory)}`"
            :value="service.name"
          >
            <div class="service-target-option">
              <strong>{{ service.name }}</strong>
              <span>CPU {{ formatPercent(service.cpu) }} · 内存 {{ formatPercent(service.memory) }}</span>
            </div>
          </el-option>
        </el-select>
        <el-select
          v-else-if="deploymentDialog.sibling.resourceType === 'k8s_cluster'"
          v-model="deploymentDialog.sibling.clusterIds"
          multiple
          collapse-tags
          collapse-tags-tooltip
          filterable
          placeholder="选择其他 K8S 集群，可多选"
          :loading="deploymentDialog.sibling.loadingClusters"
        >
          <el-option
            v-if="!deploymentDialog.sibling.loadingClusters && !deploymentDialog.sibling.clusters.length"
            disabled
            label="暂无其他 K8S 集群"
            value="__empty__"
          />
          <el-option
            v-for="cluster in deploymentDialog.sibling.clusters"
            :key="cluster.id"
            :label="cluster.displayName || cluster.name"
            :value="cluster.id"
          />
        </el-select>
        <el-select
          v-else-if="deploymentDialog.sibling.resourceType === 'k8s_namespace'"
          v-model="deploymentDialog.sibling.namespaceNames"
          multiple
          collapse-tags
          collapse-tags-tooltip
          filterable
          placeholder="选择同集群命名空间，可多选"
          :loading="deploymentDialog.sibling.loadingNamespaces"
        >
          <el-option
            v-if="!deploymentDialog.sibling.loadingNamespaces && !deploymentDialog.sibling.namespaces.length"
            disabled
            label="暂无其他命名空间"
            value="__empty__"
          />
          <el-option
            v-for="namespace in deploymentDialog.sibling.namespaces"
            :key="namespace.name"
            :label="namespace.name"
            :value="namespace.name"
          />
        </el-select>
        <el-input
          v-else
          v-model="deploymentDialog.sibling.name"
          clearable
          :placeholder="`${resourceTypeText(deploymentDialog.sibling.resourceType)}名称`"
        />
        <p v-if="deploymentDialog.sibling.useK8sSearch" class="k8s-selector-tip">
          在 {{ deploymentDialog.sibling.clusterName || '-' }} / {{ deploymentDialog.sibling.namespaceName || '-' }} 内搜索工作负载。
        </p>
      </div>

      <div v-else-if="['nginx', 'gateway'].includes(deploymentDialog.type)" class="resource-dialog-body">
        <div class="sibling-hint">
          <span>新增主链路节点</span>
          <strong>{{ deploymentDialog.parentNode?.displayName || deploymentDialog.parentNode?.name || '业务节点' }}</strong>
          <em>{{ resourceTypeText(deploymentDialog.entry.resourceType) }}</em>
        </div>
        <el-input
          v-model="deploymentDialog.entry.name"
          clearable
          :placeholder="`${resourceTypeText(deploymentDialog.entry.resourceType)}名称`"
        />
        <el-radio-group v-model="deploymentDialog.entry.logResourceType" @change="handleEntryLogTypeChange">
          <el-radio-button label="log_store">日志库</el-radio-button>
          <el-radio-button label="log_project">日志项目</el-radio-button>
        </el-radio-group>
        <el-select
          v-if="deploymentDialog.entry.logResourceType === 'log_project'"
          v-model="deploymentDialog.entry.projectSourceId"
          filterable
          placeholder="选择节点关联的日志项目"
          :loading="deploymentDialog.log.loadingProjects"
        >
          <el-option
            v-for="project in deploymentDialog.log.projects"
            :key="project.id"
            :label="project.name"
            :value="project.id"
          />
        </el-select>
        <template v-else>
          <el-select
            v-model="deploymentDialog.entry.projectId"
            filterable
            placeholder="先选择日志项目"
            :loading="deploymentDialog.log.loadingProjects"
            @change="handleEntryProjectChange"
          >
            <el-option
              v-for="project in deploymentDialog.log.projects"
              :key="project.id"
              :label="project.name"
              :value="project.id"
            />
          </el-select>
          <el-select
            v-model="deploymentDialog.entry.storeId"
            filterable
            placeholder="选择节点关联的日志库"
            :disabled="!deploymentDialog.entry.projectId"
            :loading="deploymentDialog.log.loadingStores"
          >
            <el-option
              v-for="store in deploymentDialog.log.stores"
              :key="store.id"
              :label="store.name"
              :value="store.id"
            />
          </el-select>
        </template>
        <p class="k8s-selector-tip">节点进入主拓扑链路；日志身份用于健康分析和日志查询。</p>
      </div>

      <div v-else-if="deploymentDialog.type === 'log'" class="resource-dialog-body">
        <div class="sibling-hint">
          <span>日志显式绑定</span>
          <strong>{{ deploymentDialog.parentNode?.displayName || deploymentDialog.parentNode?.name || '-' }}</strong>
          <em>关系类型：产生日志</em>
        </div>
        <el-radio-group v-model="deploymentDialog.log.resourceType" @change="handleLogResourceTypeChange">
          <el-radio-button label="log_store">日志库</el-radio-button>
          <el-radio-button label="log_project">日志项目</el-radio-button>
        </el-radio-group>
        <el-select
          v-if="deploymentDialog.log.resourceType === 'log_project'"
          v-model="deploymentDialog.log.projectIds"
          multiple
          collapse-tags
          collapse-tags-tooltip
          filterable
          placeholder="选择日志项目，可多选"
          :loading="deploymentDialog.log.loadingProjects"
        >
          <el-option
            v-for="project in deploymentDialog.log.projects"
            :key="project.id"
            :label="project.name"
            :value="project.id"
          >
            <div class="log-target-option">
              <img :src="logIcon" alt="" />
              <strong>{{ project.name }}</strong>
              <span>{{ project.code || '日志项目' }}</span>
            </div>
          </el-option>
        </el-select>
        <template v-else>
          <el-select
            v-model="deploymentDialog.log.projectId"
            filterable
            placeholder="先选择日志项目"
            :loading="deploymentDialog.log.loadingProjects"
            @change="handleLogProjectChange"
          >
            <el-option
              v-for="project in deploymentDialog.log.projects"
              :key="project.id"
              :label="project.name"
              :value="project.id"
            />
          </el-select>
          <el-select
            v-model="deploymentDialog.log.storeIds"
            multiple
            collapse-tags
            collapse-tags-tooltip
            filterable
            placeholder="选择日志库，可多选"
            :disabled="!deploymentDialog.log.projectId"
            :loading="deploymentDialog.log.loadingStores"
          >
            <el-option
              v-for="store in deploymentDialog.log.stores"
              :key="store.id"
              :label="store.name"
              :value="store.id"
            >
              <div class="log-target-option">
                <img :src="logIcon" alt="" />
                <strong>{{ store.name }}</strong>
                <span>{{ store.storeType || store.code || '日志库' }}</span>
              </div>
            </el-option>
          </el-select>
        </template>
        <p class="k8s-selector-tip">日志关系需要人工确认；监控和告警仍根据资源身份自动匹配。</p>
      </div>

      <div v-else class="resource-dialog-body">
        <el-radio-group v-model="deploymentDialog.dependency.resourceType">
          <el-radio-button label="database">数据库</el-radio-button>
          <el-radio-button label="middleware">中间件</el-radio-button>
        </el-radio-group>
        <div v-if="deploymentDialog.dependency.resourceType === 'database'" class="dependency-filter-row">
          <el-select
            v-model="deploymentDialog.dependency.dbType"
            clearable
            placeholder="数据库类型"
            @change="refreshDatabaseAssetSearch"
          >
            <el-option v-for="item in databaseTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
          </el-select>
        </div>
        <el-select
          v-if="deploymentDialog.dependency.resourceType === 'database'"
          v-model="deploymentDialog.dependency.dbAssetKey"
          filterable
          remote
          placeholder="输入数据库名称 / IP / Endpoint 搜索"
          :loading="deploymentDialog.dependency.loadingDbAssets"
          :remote-method="remoteSearchDatabaseAssets"
        >
          <el-option
            v-if="!deploymentDialog.dependency.loadingDbAssets && !deploymentDialog.dependency.dbAssets.length"
            disabled
            label="输入至少 2 个字符搜索数据库资产"
            value="__empty__"
          />
          <el-option
            v-for="asset in deploymentDialog.dependency.dbAssets"
            :key="asset.key"
            :label="asset.label"
            :value="asset.key"
          >
            <div class="db-target-option">
              <strong>{{ asset.displayName }}</strong>
              <span>{{ asset.typeName || '数据库' }} · {{ asset.endpoint || asset.hostIp || '-' }}</span>
            </div>
          </el-option>
        </el-select>
        <el-radio-group v-else v-model="deploymentDialog.dependency.mode" class="resource-mode-group">
          <el-radio-button label="create">新建资源</el-radio-button>
          <el-radio-button label="existing">选择已有资源</el-radio-button>
        </el-radio-group>
        <el-input
          v-if="deploymentDialog.dependency.resourceType !== 'database' && deploymentDialog.dependency.mode === 'create'"
          v-model="deploymentDialog.dependency.name"
          clearable
          placeholder="依赖组件名称"
        />
        <div v-else-if="deploymentDialog.dependency.resourceType !== 'database'" class="existing-resource-picker">
          <el-input
            v-model="deploymentDialog.dependency.keyword"
            clearable
            placeholder="搜索已有数据库 / 中间件"
            @keyup.enter="searchDependencyResources"
          >
            <template #append>
              <el-button class="action-button action-button--confirm" :loading="deploymentDialog.dependency.searching" @click="searchDependencyResources">搜索</el-button>
            </template>
          </el-input>
          <div class="existing-resource-list" v-loading="deploymentDialog.dependency.searching">
            <button
              v-for="item in deploymentDialog.dependency.options"
              :key="item.id"
              type="button"
              :class="{ active: String(deploymentDialog.dependency.selectedResource?.id) === String(item.id) }"
              @click="deploymentDialog.dependency.selectedResource = item"
            >
              <strong>{{ item.displayName || item.name || item.label }}</strong>
              <span>{{ resourceTypeText(item.resourceType || item.type) }}</span>
            </button>
            <div v-if="!deploymentDialog.dependency.options.length" class="empty-risk">输入关键词搜索依赖组件</div>
          </div>
        </div>
        <el-select v-model="deploymentDialog.dependency.targetId" placeholder="挂载目标，默认项目">
          <el-option label="项目根节点" :value="rootNodeId" />
          <el-option
            v-for="item in serviceTargetOptions"
            :key="item.id"
            :label="item.displayName || item.name || item.label"
            :value="item.id"
          />
        </el-select>
      </div>

      <template #footer>
        <el-button class="action-button action-button--warning" @click="deploymentDialog.visible = false">取消</el-button>
        <el-button class="action-button action-button--confirm" @click="confirmDeploymentWizard">加入草稿</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, Check, Close, Edit, Plus, Refresh, RefreshLeft, RefreshRight, View } from '@element-plus/icons-vue'
import G6 from '@antv/g6'
import { bindBusinessResources, deleteBusinessBindingSubtree, getBusinessOverview, getBusinessTopology, refreshBusinessHealth } from '@/api/cmdbBusinessModel'
import { deleteRelation, searchResources, updateResource } from '@/api/cmdbModel/resource'
import cmdbApi from '@/api/cmdb'
import { dbAPI } from '@/api/database'
import k8sApi from '@/api/k8s'
import logtailApi from '@/api/logtail'
import appIcon from '@/assets/image/应用.svg'
import cloudHostIcon from '@/assets/image/云主机服务器.svg'
import databaseIcon from '@/assets/image/数据库.svg'
import gatewayIcon from '@/assets/image/gateway.svg'
import k8sIcon from '@/assets/image/k8s.svg'
import linuxIcon from '@/assets/image/linux.svg'
import logIcon from '@/assets/image/日志.svg'
import namespaceIcon from '@/assets/image/命名空间.svg'
import middlewareIcon from '@/assets/image/监控.svg'
import nginxIcon from '@/assets/image/nginx.svg'
import mongoIcon from '@/assets/image/mongodb.svg'
import mysqlIcon from '@/assets/image/mysqldb.svg'
import physicalServerIcon from '@/assets/image/物理机.svg'
import redisIcon from '@/assets/image/redis.svg'
import virtualIcon from '@/assets/image/虚拟机.svg'
import workloadIcon from '@/assets/image/工作负载.svg'

const route = useRoute()
const router = useRouter()
const graphContainer = ref(null)

const topologyLoading = ref(false)
const refreshing = ref(false)
const savingTopology = ref(false)
const isEditing = ref(false)
const selectedNode = ref(null)
const summary = ref({})
const dimensions = ref([])
const assetStats = ref({})
const riskNodes = ref([])
const topologyData = ref({ nodes: [], edges: [] })
const completeTopologyData = ref({ nodes: [], edges: [] })
const originalTopologyData = ref({ nodes: [], edges: [] })
const expandedLogResourceIds = ref([])
const draftChanges = ref([])
const redoStack = ref([])
const dragState = reactive({
  active: false,
  item: null,
  nodeId: '',
  startX: 0,
  startY: 0,
  nodeX: 0,
  nodeY: 0
})

const contextMenu = reactive({
  visible: false,
  x: 0,
  y: 0,
  node: null,
  edge: null,
  canvasPoint: { x: 0, y: 0 }
})

const deploymentDialog = reactive({
  visible: false,
  type: 'k8s',
  parentNode: null,
  k8s: {
    clusterId: '',
    namespaceName: '',
    targetKey: '',
    targetKeys: [],
    clusters: [],
    namespaces: [],
    targets: [],
    targetKeyword: '',
    loadingClusters: false,
    loadingNamespaces: false,
    loadingTargets: false
  },
  host: {
    hostKey: '',
    hostKeyword: '',
    options: [],
    loadingHosts: false,
    serviceName: '',
    serviceNames: [],
    services: [],
    loadingServices: false
  },
  child: {
    resourceType: '',
    namespaceName: '',
    namespaces: [],
    loadingNamespaces: false,
    targetKeys: [],
    targets: [],
    targetKeyword: '',
    loadingTargets: false,
    serviceNames: [],
    services: [],
    loadingServices: false,
    clusterId: '',
    clusterName: '',
    hostId: '',
    hostName: '',
    hostIp: ''
  },
  sibling: {
    resourceType: '',
    name: '',
    currentSourceId: '',
    currentName: '',
    useK8sSearch: false,
    clusterId: '',
    clusterName: '',
    namespaceName: '',
    clusterIds: [],
    clusters: [],
    loadingClusters: false,
    namespaceNames: [],
    namespaces: [],
    loadingNamespaces: false,
    targetKey: '',
    targetKeys: [],
    targets: [],
    targetKeyword: '',
    loadingTargets: false,
    hostId: '',
    hostName: '',
    hostIp: '',
    hostKeys: [],
    hosts: [],
    hostKeyword: '',
    loadingHosts: false,
    serviceNames: [],
    services: [],
    loadingServices: false
  },
  log: {
    resourceType: 'log_store',
    projectId: '',
    projectIds: [],
    storeIds: [],
    projects: [],
    stores: [],
    loadingProjects: false,
    loadingStores: false
  },
  entry: {
    resourceType: 'nginx',
    name: '',
    logResourceType: 'log_store',
    projectId: '',
    projectSourceId: '',
    storeId: ''
  },
  dependency: {
    resourceType: 'database',
    mode: 'create',
    name: '',
    keyword: '',
    searching: false,
    options: [],
    selectedResource: null,
    targetId: '',
    dbAssetKey: '',
    dbAssetKeyword: '',
    dbAssets: [],
    loadingDbAssets: false,
    dbType: ''
  }
})

const topologyQuery = reactive({
  depth: 6,
  direction: 'downstream'
})

const healthStatusMap = {
  healthy: { label: '正常', color: '#16a34a' },
  warning: { label: '轻微问题', color: '#f59e0b' },
  critical: { label: '严重异常', color: '#dc2626' },
  unknown: { label: '未接入数据', color: '#94a3b8' }
}

const resourceTypeMap = {
  business_system: '业务系统',
  business_service: '业务服务',
  application: '应用',
  physical_server: '物理机',
  host: '主机',
  vm: '虚拟机',
  virtual: '虚拟机',
  cloud_host: '云主机',
  k8s_cluster: 'K8S 集群',
  k8s_namespace: '命名空间',
  k8s_workload: '工作负载',
  pod: 'Pod',
  database: '数据库',
  middleware: '中间件',
  log_project: '日志项目',
  log_store: '日志库',
  redis: 'Redis',
  kafka: 'Kafka',
  nginx: 'Nginx',
  gateway: 'Gateway'
}

const relationTypeMap = {
  contains: '包含',
  depends_on: '依赖',
  uses: '使用',
  deployed_on: '部署于',
  runs_on: '运行于',
  emits_log_to: '产生日志',
  connects_to: '连接',
  connected_to: '连接',
  related_to: '关联'
}

const dependencyResourceTypes = new Set(['database', 'middleware', 'redis', 'kafka'])
const dependencyRelationTypes = new Set(['depends_on', 'uses'])
const observabilityResourceTypes = new Set(['log_project', 'log_store'])

const dimensionMap = {
  overall: '综合健康',
  infra: '基础资源',
  database: '数据库',
  middleware: '中间件',
  container: '容器',
  business: '业务监控',
  log: '日志'
}

const legend = [
  { key: 'healthy', label: '正常', color: '#16a34a' },
  { key: 'warning', label: '轻微问题', color: '#f59e0b' },
  { key: 'critical', label: '严重异常', color: '#dc2626' },
  { key: 'unknown', label: '未接入', color: '#94a3b8' }
]

const databaseTypeOptions = [
  { value: 1, label: 'MySQL' },
  { value: 2, label: 'PostgreSQL' },
  { value: 4, label: 'Redis' },
  { value: 5, label: 'MongoDB' },
  { value: 7, label: 'Elasticsearch' }
]

let graph = null
let graphNodeRegistered = false
let k8sTargetSearchTimer = null
let siblingK8sTargetSearchTimer = null
let childK8sTargetSearchTimer = null
let hostSearchTimer = null
let siblingHostSearchTimer = null
let databaseAssetSearchTimer = null

const businessId = computed(() => route.params.id)
const businessTitle = computed(() => summary.value.displayName || summary.value.name || `业务模型 ${businessId.value}`)
const businessSummary = computed(() => {
  const text = summary.value.problemSummary || ''
  if (!text || /active|来源于资源状态/i.test(text)) {
    return '按业务系统、业务服务、应用资源、基础设施和依赖组件分层展示健康链路。'
  }
  return text
})

const statusCards = computed(() => [
  { key: 'score', label: '综合健康', value: formatScore(summary.value.healthScore), color: resolveHealthColor(summary.value.healthStatus, summary.value.healthColor) },
  { key: 'healthy', label: '正常节点', value: topologyStatusStats.value.healthy || 0, color: '#16a34a' },
  { key: 'risk', label: '风险节点', value: topologyRiskCount.value, color: topologyRiskCount.value ? '#f59e0b' : '#16a34a' },
  { key: 'k8sDeploy', label: 'K8S 部署', value: productAssetStats.value.k8sDeploy, color: '#2563eb' },
  { key: 'dependency', label: '依赖组件', value: productAssetStats.value.dependency, color: '#7c3aed' }
])

const assetCards = computed(() => {
  const stats = topologyResourceStats.value
  return [
    { key: 'k8s_cluster', label: 'K8S 集群', value: sumStats(stats, ['k8s_cluster']) },
    { key: 'k8s_namespace', label: '命名空间', value: sumStats(stats, ['k8s_namespace']) },
    { key: 'k8s_workload', label: '工作负载', value: sumStats(stats, ['k8s_workload', 'pod', 'container']) },
    { key: 'host', label: '主机部署', value: sumStats(stats, ['host', 'physical_server', 'vm', 'cloud_host', 'virtual']) },
    { key: 'service', label: '业务服务', value: sumStats(stats, ['business_service', 'application']) },
    { key: 'dependency', label: '依赖组件', value: sumStats(stats, ['database', 'middleware', 'redis', 'kafka']) }
  ].filter(item => item.value > 0 || ['k8s_cluster', 'k8s_namespace', 'k8s_workload', 'host', 'dependency'].includes(item.key))
})

const nodeEvidenceText = computed(() => formatEvidence(selectedNode.value?.evidence || selectedNode.value?.properties?.evidence))
const selectedObservability = computed(() => {
  const node = selectedNode.value
  return buildObservabilitySummary(node, relatedLogNodesFor(node?.id).length)
})
const selectedBoundLogs = computed(() => boundLogItemsFor(selectedNode.value))
const selectedMonitorMetrics = computed(() => monitorMetricRowsFor(selectedNode.value))
const selectedMonitorMatchText = computed(() => monitorMatchTextFor(selectedNode.value))
const selectedLogsExpanded = computed(() => expandedLogResourceIds.value.includes(String(selectedNode.value?.id || '')))
const canUndo = computed(() => draftChanges.value.length > 0)
const canRedo = computed(() => redoStack.value.length > 0)
const rootNodeId = computed(() => topologyData.value.nodes.find(node => node.resourceType === 'business_system')?.id || String(businessId.value))
const isHostResourceType = type => ['host', 'physical_server', 'vm', 'virtual', 'cloud_host'].includes(type)
const isServiceResourceType = type => type === 'business_service'
const getNextChildResourceType = node => {
  const type = node?.resourceType
  if (isHostResourceType(type)) return 'business_service'
  if (type === 'k8s_cluster') return 'k8s_namespace'
  if (type === 'k8s_namespace') return 'k8s_workload'
  return ''
}
const deploymentDialogTitle = computed(() => {
  const titleMap = {
    k8s: '部署到 K8S',
    host: '部署到主机',
    nginx: '添加 Nginx',
    gateway: '添加 Gateway',
    log: '绑定日志资源',
    dependency: '添加依赖组件',
    child: '添加下一级',
    sibling: '添加同级节点'
  }
  return titleMap[deploymentDialog.type] || '添加部署'
})
const serviceTargetOptions = computed(() => {
  return topologyData.value.nodes.filter(node => ['business_service', 'application', 'k8s_workload'].includes(node.resourceType))
})
const contextMenuItems = computed(() => {
  if (contextMenu.edge) {
    return [
      { key: 'delete-edge', label: '删除拓扑连接', danger: true }
    ]
  }

  if (!contextMenu.node) {
    return [
      { key: 'deploy-k8s', label: '添加 K8S 部署' },
      { key: 'deploy-host', label: '添加主机部署' }
    ]
  }

  const node = contextMenu.node
  if (!isEditing.value) {
    return canOpenNodeDetail(node) ? [{ key: 'view-detail', label: '查看详情' }] : []
  }

  const resourceType = node.resourceType
  const isBusinessRoot = resourceType === 'business_system'
  const isNginx = resourceType === 'nginx'
  const isGateway = resourceType === 'gateway'
  const canAddDependency = isServiceResourceType(resourceType) || ['k8s_workload', 'pod'].includes(resourceType)
  const canBindLog = ['business_service', 'application', 'k8s_workload', 'pod', 'database', 'middleware', 'redis', 'kafka', 'nginx', 'gateway'].includes(resourceType)
  const childResourceType = getNextChildResourceType(node)

  return [
    isBusinessRoot ? { key: 'add-nginx', label: '添加 Nginx' } : null,
    isBusinessRoot || isNginx ? { key: 'add-gateway', label: '添加 Gateway' } : null,
    isBusinessRoot ? { key: 'deploy-k8s', label: '添加 K8S 部署' } : null,
    isBusinessRoot ? { key: 'deploy-host', label: '添加主机部署' } : null,
    isGateway ? { key: 'deploy-k8s', label: '添加 K8S 部署' } : null,
    isGateway ? { key: 'deploy-host', label: '添加主机部署' } : null,
    canAddDependency ? { key: 'add-dependency', label: '添加依赖组件' } : null,
    canBindLog ? { key: 'bind-log', label: '绑定日志' } : null,
    childResourceType ? { key: 'add-child', label: `添加下一级 ${resourceTypeText(childResourceType)}` } : null,
    !isBusinessRoot && !isNginx && !isGateway ? { key: 'add-sibling', label: '添加同级' } : null,
    !isBusinessRoot ? { key: 'delete-node', label: '删除节点', danger: true } : null
  ].filter(Boolean)
})

const k8sDetailResourceTypes = new Set(['k8s_workload', 'pod'])
const hostDetailResourceTypes = new Set(['host', 'physical_server', 'vm', 'virtual', 'cloud_host'])
const databaseDetailResourceTypes = new Set(['database'])
const directOpenResourceTypes = new Set([
  ...databaseDetailResourceTypes,
  ...k8sDetailResourceTypes,
  ...hostDetailResourceTypes,
  'k8s_cluster',
  'k8s_namespace',
  'log_project',
  'log_store'
])

const topologyStatusStats = computed(() => {
  const stats = {
    healthy: 0,
    warning: 0,
    critical: 0,
    unknown: 0
  }
  topologyData.value.nodes.forEach(node => {
    const status = node.healthStatus || 'unknown'
    stats[status] = (stats[status] || 0) + 1
  })
  return stats
})

const topologyRiskCount = computed(() => Number(topologyStatusStats.value.warning || 0) + Number(topologyStatusStats.value.critical || 0))

const topologyResourceStats = computed(() => {
  if (!topologyData.value.nodes.length) {
    return assetStats.value || {}
  }

  return topologyData.value.nodes.reduce((stats, node) => {
    const type = node.resourceType || 'unknown'
    stats[type] = (stats[type] || 0) + 1
    return stats
  }, {})
})

const productAssetStats = computed(() => {
  const stats = topologyResourceStats.value
  return {
    k8sDeploy: sumStats(stats, ['k8s_cluster', 'k8s_namespace', 'k8s_workload', 'pod', 'container']),
    hostDeploy: sumStats(stats, ['host', 'physical_server', 'vm', 'cloud_host', 'virtual']),
    dependency: sumStats(stats, ['database', 'middleware', 'redis', 'kafka']),
    service: sumStats(stats, ['business_service', 'application'])
  }
})

const normalizeResponse = response => {
  const axiosResponse = response?.data ? response : { data: response }
  return axiosResponse.data || null
}

const isSuccessResponse = response => response?.code === 0 || response?.code === 200 || response?.success

const pickList = (data, keys = []) => {
  if (Array.isArray(data)) return data
  for (const key of keys) {
    if (Array.isArray(data?.[key])) return data[key]
  }
  if (Array.isArray(data?.list)) return data.list
  return []
}

const resolveHealthColor = (status, customColor) => customColor || healthStatusMap[status]?.color || '#94a3b8'
const resolveHealthSoftColor = status => {
  const colorMap = {
    healthy: 'rgba(63, 185, 127, 0.13)',
    warning: 'rgba(243, 178, 75, 0.16)',
    critical: 'rgba(239, 107, 115, 0.15)',
    unknown: 'rgba(148, 163, 184, 0.14)'
  }
  return colorMap[status] || colorMap.unknown
}
const healthStatusText = status => healthStatusMap[status]?.label || '未知'
const resourceTypeText = type => resourceTypeMap[type] || type || '资源'
const relationTypeText = type => relationTypeMap[type] || type || ''
const dimensionText = key => dimensionMap[key] || key || '未知维度'
const sumStats = (stats, keys) => keys.reduce((sum, key) => sum + Number(stats?.[key] || 0), 0)
const formatPercent = value => {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return '-'
  return `${numeric.toFixed(numeric >= 10 ? 1 : 2).replace(/\.?0+$/, '')}%`
}

const formatScore = value => {
  if (value === null || value === undefined || value === '') return '--'
  return Number(value)
}

const formatDimensionScore = item => item?.status === 'unknown' ? '未接入' : formatScore(item?.score)

const normalizeK8sCluster = cluster => ({
  id: cluster.id,
  name: cluster.name || cluster.clusterName || '',
  displayName: cluster.displayName || cluster.aliasName || cluster.name || cluster.clusterName || '',
  status: cluster.status,
  rawData: cluster
})

const normalizeK8sNamespace = namespace => ({
  name: namespace.name || namespace.namespace || namespace.namespaceName || '',
  status: namespace.status,
  rawData: namespace
})

const normalizeK8sWorkload = workload => {
  const workloadType = workload.workloadType || workload.type || workload.kind || 'Deployment'
  const name = workload.name || workload.workloadName || ''
  return {
    key: `workload:${workloadType}:${name}`,
    kind: 'workload',
    name,
    workloadType,
    status: workload.status || '',
    healthScore: workload.healthScore,
    rawData: workload
  }
}

const normalizeCmdbHost = host => {
  const id = host.id || host.hostId
  const hostName = host.hostName || host.hostname || host.name || ''
  const ip = host.privateIp || host.sshIp || host.publicIp || host.ip || ''
  return {
    key: `host:${id}`,
    id,
    name: ip || hostName || `host-${id}`,
    displayName: hostName || ip || `主机 ${id}`,
    label: `${hostName || ip || `主机 ${id}`} · ${ip || '无 IP'}`,
    hostName,
    ip,
    privateIp: host.privateIp || '',
    publicIp: host.publicIp || '',
    sshIp: host.sshIp || '',
    hostType: host.hostType,
    vendor: host.vendor,
    os: host.os || '',
    status: host.status,
    instanceId: host.instanceId || '',
    rawData: host
  }
}

const normalizeHostService = service => ({
  name: service.name || service.serviceName || '',
  cpu: Number(service.cpu ?? service.cpuPercent ?? 0),
  memory: Number(service.memory ?? service.memPercent ?? service.memoryPercent ?? 0)
})

const normalizeDatabaseAsset = asset => {
  const id = asset.id || asset.assetId
  const typeName = asset.typeName || databaseTypeOptions.find(item => Number(item.value) === Number(asset.type))?.label || ''
  const endpoint = asset.endpoint || [asset.hostIp, asset.port].filter(Boolean).join(':')
  const displayName = asset.instanceName || asset.name || endpoint || `数据库资产 ${id}`
  return {
    key: `db_asset:${id}`,
    id,
    name: asset.name || asset.instanceName || endpoint || `db-${id}`,
    displayName,
    label: `${displayName} · ${endpoint || '-'}`,
    instanceName: asset.instanceName || '',
    type: asset.type,
    typeName,
    hostIp: asset.hostIp || '',
    port: asset.port,
    endpoint,
    environment: asset.environment || '',
    status: asset.status,
    authStatus: asset.authStatus,
    authStatusName: asset.authStatusName || '',
    businessSystem: asset.businessSystem || '',
    owner: asset.owner || '',
    rawData: asset
  }
}

const buildK8sSelection = () => {
  const k8s = deploymentDialog.k8s
  const cluster = k8s.clusters.find(item => String(item.id) === String(k8s.clusterId))
  const namespace = k8s.namespaces.find(item => item.name === k8s.namespaceName)
  const selectedKeys = new Set((k8s.targetKeys || []).map(String))
  const targets = k8s.targets.filter(item => selectedKeys.has(String(item.key)))
  if (!cluster || !namespace || !targets.length) return null
  return { cluster, namespace, targets }
}

const resolveK8sContext = (node, parentNode) => {
  const nodeProperties = normalizeProperties(node?.properties || node?.Properties)
  const parentProperties = normalizeProperties(parentNode?.properties || parentNode?.Properties)
  const clusterId = nodeProperties.clusterId || parentProperties.clusterId || (node?.resourceType === 'k8s_cluster' ? node?.sourceId : undefined)
  const namespaceName = nodeProperties.namespace || parentProperties.namespace
  const clusterName = nodeProperties.clusterName || parentProperties.clusterName || parentNode?.displayName || parentNode?.name
  return {
    clusterId,
    clusterName,
    namespaceName
  }
}

const normalizeWorkloadTypeForApi = type => {
  const value = String(type || '').trim().toLowerCase()
  const typeMap = {
    deployments: 'deployment',
    deployment: 'deployment',
    statefulsets: 'statefulset',
    statefulset: 'statefulset',
    daemonsets: 'daemonset',
    daemonset: 'daemonset',
    jobs: 'job',
    job: 'job',
    cronjobs: 'cronjob',
    cronjob: 'cronjob',
    pods: 'pod',
    pod: 'pod'
  }
  return typeMap[value] || value || 'deployment'
}

const parseK8sSourceKey = sourceKey => {
  const text = String(sourceKey || '')
  const match = text.match(/^k8s:cluster:([^:]+):namespace:([^:]+):workload:([^:]+):(.+)$/)
  if (!match) return {}
  return {
    clusterId: match[1],
    namespaceName: decodeURIComponent(match[2]),
    workloadType: decodeURIComponent(match[3]),
    workloadName: decodeURIComponent(match[4])
  }
}

const resolveK8sWorkloadContext = node => {
  const properties = normalizeProperties(node?.properties || node?.Properties)
  const sourceContext = parseK8sSourceKey(node?.sourceKey || node?.SourceKey)
  const clusterId = properties.clusterId || sourceContext.clusterId || node?.sourceId
  const namespaceName = properties.namespace || properties.namespaceName || sourceContext.namespaceName
  const workloadType = properties.workloadType || properties.kind || sourceContext.workloadType || node?.workloadType || 'deployment'
  const workloadName = properties.workloadName || properties.podName || sourceContext.workloadName || node?.name || node?.displayName

  return {
    clusterId,
    namespaceName,
    workloadType: normalizeWorkloadTypeForApi(workloadType),
    workloadName
  }
}

const canOpenNodeDetail = node => {
  if (isEditing.value || !node) return false
  return directOpenResourceTypes.has(node.resourceType) || ['db_asset', 'log_store', 'log_project'].includes(node.sourceType)
}

const resolveHostContext = node => {
  const properties = normalizeProperties(node?.properties || node?.Properties)
  const sourceId = node?.sourceId || node?.SourceId
  const sourceKey = String(node?.sourceKey || node?.SourceKey || '')
  const match = sourceKey.match(/cmdb_host:(\d+)/)
  const hostId = properties.hostId || properties.assetId || sourceId || match?.[1]
  const keyword = properties.hostName ||
    properties.privateIp ||
    properties.sshIp ||
    properties.publicIp ||
    node?.name ||
    node?.displayName ||
    node?.label ||
    node?.subtitle

  return {
    hostId,
    keyword
  }
}

const resolveDatabaseContext = node => {
  const properties = normalizeProperties(node?.properties || node?.Properties)
  const sourceId = node?.sourceId || node?.SourceId
  const sourceKey = String(node?.sourceKey || node?.SourceKey || '')
  const match = sourceKey.match(/(?:db_asset|middleware_asset):(\d+)/)
  const assetId = properties.assetId || properties.databaseId || sourceId || match?.[1]

  return {
    assetId: Number.isFinite(Number(assetId)) ? String(assetId) : ''
  }
}

const openDatabaseMonitor = async (node, extraQuery = {}) => {
  const context = resolveDatabaseContext(node)
  if (!context.assetId) {
    ElMessage.warning('当前数据库节点缺少数据库资产 ID')
    return false
  }
  await router.push({
    path: '/db/monitoring',
    query: { assetId: context.assetId, ...extraQuery }
  })
  return true
}

const openHostDetail = async node => {
  const context = resolveHostContext(node)
  if (context.hostId && !Number.isNaN(Number(context.hostId))) {
    await router.push(`/cmdb/host/detail/${context.hostId}`)
    return true
  }

  const keyword = String(context.keyword || '').trim()
  if (!keyword) {
    ElMessage.warning('当前主机节点缺少主机 ID、名称或 IP')
    return false
  }

  try {
    const res = normalizeResponse(await cmdbApi.GetCmdbHostsByKeyword(keyword, {
      page: 1,
      pageSize: 10
    }))

    if (!isSuccessResponse(res)) {
      throw new Error(res?.message || '搜索主机失败')
    }

    const hosts = pickList(res.data)
    const matchedHost = hosts.find(host => {
      const candidates = [
        host.id,
        host.hostName,
        host.name,
        host.privateIp,
        host.publicIp,
        host.sshIp
      ].filter(Boolean).map(value => String(value))
      return candidates.some(value => value === keyword || value.includes(keyword) || keyword.includes(value))
    }) || hosts[0]

    if (!matchedHost?.id) {
      ElMessage.warning(`未找到主机：${keyword}`)
      return false
    }

    await router.push(`/cmdb/host/detail/${matchedHost.id}`)
    return true
  } catch (error) {
    console.error('跳转主机详情失败:', error)
    ElMessage.error(error.message || '跳转主机详情失败')
    return false
  }
}

const openNodeDetail = async node => {
  if (['database'].includes(node?.resourceType) || node?.sourceType === 'db_asset') {
    return openDatabaseMonitor(node)
  }
  if (k8sDetailResourceTypes.has(node?.resourceType)) {
    return openK8sWorkloadDetail(node)
  }
  if (node?.resourceType === 'k8s_cluster') {
    const context = resolveK8sContext(node)
    await router.push({ path: '/k8s/workload', query: { clusterId: context.clusterId || undefined } })
    return true
  }
  if (node?.resourceType === 'k8s_namespace') {
    const context = resolveK8sContext(node)
    await router.push({
      path: '/k8s/workload',
      query: {
        clusterId: context.clusterId || undefined,
        namespace: context.namespaceName || undefined
      }
    })
    return true
  }
  if (hostDetailResourceTypes.has(node?.resourceType)) {
    return openHostDetail(node)
  }
  if (node?.resourceType === 'log_store' || node?.sourceType === 'log_store') {
    const properties = normalizeProperties(node?.properties)
    await router.push({
      path: '/logs/query',
      query: {
        projectId: properties.projectId || undefined,
        storeId: properties.storeId || node?.sourceId || undefined
      }
    })
    return true
  }
  if (node?.resourceType === 'log_project' || node?.sourceType === 'log_project') {
    await router.push('/logs/manage')
    return true
  }
  return false
}

const openK8sWorkloadDetail = async node => {
  const context = resolveK8sWorkloadContext(node)
  if (!context.clusterId || !context.namespaceName || !context.workloadName) {
    ElMessage.warning('当前节点缺少 K8S 集群、命名空间或工作负载信息')
    return false
  }

  try {
    if (context.workloadType === 'pod') {
      await router.push({
        path: `/k8s/pod/${context.clusterId}/${context.namespaceName}/${context.workloadName}`
      })
      return true
    }

    const res = normalizeResponse(await k8sApi.getWorkloadPods(
      context.clusterId,
      context.namespaceName,
      context.workloadType,
      context.workloadName
    ))

    if (!isSuccessResponse(res)) {
      throw new Error(res?.message || '获取工作负载 Pod 失败')
    }

    const pods = pickList(res.data)
    const firstPod = pods.find(item => item?.name) || pods[0]
    const podName = firstPod?.name || firstPod?.metadata?.name
    if (!podName) {
      ElMessage.warning('该工作负载下暂无可跳转的 Pod')
      return false
    }

    await router.push({
      path: `/k8s/pod/${context.clusterId}/${context.namespaceName}/${podName}`,
      query: {
        workloadType: context.workloadType,
        workloadName: context.workloadName
      }
    })
    return true
  } catch (error) {
    console.error('跳转 K8S 工作负载详情失败:', error)
    ElMessage.error(error.message || '跳转 K8S 工作负载详情失败')
    return false
  }
}

const loadOverview = async () => {
  try {
    const res = normalizeResponse(await getBusinessOverview(businessId.value))
    if (res?.code !== 0) {
      throw new Error(res?.message || '获取业务概览失败')
    }

    const data = res.data || {}
    summary.value = data.summary || {}
    dimensions.value = Array.isArray(data.dimensions) ? data.dimensions : []
    assetStats.value = data.assetStats || {}
    riskNodes.value = Array.isArray(data.riskNodes) ? data.riskNodes : []
  } catch (error) {
    console.error('获取业务概览失败:', error)
    ElMessage.error(error.message || '获取业务概览失败')
  }
}

const loadTopology = async () => {
  if (isEditing.value) return
  topologyLoading.value = true
  try {
    const res = normalizeResponse(await getBusinessTopology(businessId.value, {
      depth: topologyQuery.depth,
      direction: topologyQuery.direction
    }))

    if (res?.code !== 0) {
      throw new Error(res?.message || '获取业务拓扑失败')
    }

    completeTopologyData.value = transformTopology(res.data || {})
    expandedLogResourceIds.value = []
    topologyData.value = projectVisibleTopology(completeTopologyData.value)
    originalTopologyData.value = cloneTopology(topologyData.value)
    selectedNode.value = topologyData.value.nodes.find(item => item.id === String(route.query.nodeId)) || topologyData.value.nodes[0] || null
    await renderGraph()
  } catch (error) {
    console.error('获取业务拓扑失败:', error)
    ElMessage.error(error.message || '获取业务拓扑失败')
  } finally {
    topologyLoading.value = false
  }
}

const cloneTopology = data => ({
  nodes: (data.nodes || []).map(node => ({
    ...node,
    properties: { ...(node.properties || {}) },
    dimensions: (node.dimensions || []).map(item => ({ ...item, evidence: { ...(item.evidence || {}) } }))
  })),
  edges: (data.edges || []).map(edge => ({ ...edge })),
  hasSavedLayout: Boolean(data.hasSavedLayout)
})

const isObservabilityNode = node => node?.nodeRole === 'observability' || observabilityResourceTypes.has(node?.resourceType)

const projectVisibleTopology = (data, expandedIds = expandedLogResourceIds.value) => {
  const nodes = data?.nodes || []
  const edges = data?.edges || []
  const nodeMap = new Map(nodes.map(node => [String(node.id), node]))
  const visibleIds = new Set(nodes.filter(node => !isObservabilityNode(node)).map(node => String(node.id)))
  const frontier = [...new Set((expandedIds || []).map(String))]

  while (frontier.length) {
    const sourceId = frontier.shift()
    edges.forEach(edge => {
      if (String(edge.source) !== sourceId) return
      const targetId = String(edge.target)
      const target = nodeMap.get(targetId)
      if (!target || !isObservabilityNode(target) || visibleIds.has(targetId)) return
      if (edge.relationType !== 'emits_log_to' && !isObservabilityNode(nodeMap.get(sourceId))) return
      visibleIds.add(targetId)
      frontier.push(targetId)
    })
  }

  return {
    nodes: nodes.filter(node => visibleIds.has(String(node.id))),
    edges: edges.filter(edge => visibleIds.has(String(edge.source)) && visibleIds.has(String(edge.target))),
    hasSavedLayout: Boolean(data?.hasSavedLayout)
  }
}

const normalizeLogProject = project => ({
  id: project.id,
  name: project.name || project.displayName || `日志项目 ${project.id}`,
  code: project.code || '',
  status: project.status,
  description: project.description || '',
  rawData: project
})

const normalizeLogStore = store => ({
  id: store.id,
  projectId: store.projectId,
  name: store.name || store.displayName || `日志库 ${store.id}`,
  code: store.code || '',
  storeType: store.storeType || store.type || '',
  status: store.status,
  description: store.description || '',
  rawData: store
})

const normalizeProperties = properties => {
  if (!properties) return {}
  if (typeof properties === 'string') {
    try {
      return JSON.parse(properties)
    } catch (error) {
      return {}
    }
  }
  return properties
}

const evidenceObjects = node => {
  const objects = []
  const seen = new WeakSet()
  const nestedArrayKeys = [
    'matchedServices',
    'signals',
    'alerts',
    'activeAlerts',
    'firingAlerts',
    'alertEvents'
  ]

  const appendObject = value => {
    const normalized = normalizeProperties(value)
    if (!normalized || typeof normalized !== 'object' || Array.isArray(normalized) || seen.has(normalized)) return null
    seen.add(normalized)
    objects.push(normalized)
    return normalized
  }

  const appendNestedArrays = container => {
    if (!container) return
    nestedArrayKeys.forEach(key => {
      if (Array.isArray(container[key])) container[key].forEach(appendObject)
    })
  }

  const appendContainer = value => {
    const container = appendObject(value)
    if (!container) return null

    appendNestedArrays(container)
    if (Array.isArray(container.checks)) {
      container.checks.forEach(value => {
        const check = appendObject(value)
        if (!check) return
        appendNestedArrays(check)
        const checkEvidence = appendObject(check.evidence)
        appendNestedArrays(checkEvidence)
      })
    }
    return container
  }

  const dimensions = Array.isArray(node?.dimensions) ? node.dimensions : []
  dimensions.forEach(item => appendContainer(item?.evidence))
  appendContainer(node?.evidence)
  const properties = appendContainer(node?.properties)
  appendContainer(properties?.evidence)
  return objects
}

const boundLogEvidenceItems = (node, evidenceItems = evidenceObjects(node)) => {
  const items = new Map()
  let fallbackIndex = 0
  evidenceItems.forEach(evidence => {
    if (!Array.isArray(evidence.boundLogs)) return
    evidence.boundLogs.forEach(item => {
      const key = String(item?.id || item?.sourceKey || item?.name || `bound-log-${fallbackIndex++}`)
      if (!items.has(key)) items.set(key, item)
    })
  })
  return [...items.values()]
}

const firstEvidenceValue = (node, keys, evidenceItems = evidenceObjects(node)) => {
  for (const evidence of evidenceItems) {
    for (const key of keys) {
      const value = evidence?.[key]
      if (value !== undefined && value !== null && value !== '') return value
    }
  }
  return undefined
}

const numericEvidenceValue = (node, keys) => {
  const value = firstEvidenceValue(node, keys)
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : undefined
}

const monitorDimensionForNode = node => {
  const preferredMap = {
    host: 'infra',
    physical_server: 'infra',
    vm: 'infra',
    virtual: 'infra',
    cloud_host: 'infra',
    business_service: 'business',
    application: 'business',
    nginx: 'business',
    gateway: 'business',
    k8s_cluster: 'container',
    k8s_namespace: 'container',
    k8s_workload: 'container',
    pod: 'container',
    database: 'middleware',
    middleware: 'middleware'
  }
  const dimensions = node?.dimensions || []
  const preferred = preferredMap[node?.resourceType]
  return dimensions.find(item => item.dimension === preferred) ||
    dimensions.find(item => !['overall', 'log'].includes(item.dimension)) ||
    null
}

const observabilityStatusText = status => {
  const labels = {
    healthy: '正常',
    warning: '注意',
    critical: '异常',
    unknown: '未接入'
  }
  return labels[status] || '未接入'
}

const alertCountFromNode = (node, evidenceItems = evidenceObjects(node)) => {
  let count = 0
  evidenceItems.forEach(evidence => {
    const values = [
      evidence.activeAlertCount,
      evidence.alertCount,
      evidence.firingCount,
      evidence.firingEvents
    ]
    values.forEach(value => {
      const numeric = Number(value)
      if (Number.isFinite(numeric)) count = Math.max(count, numeric)
    })
    ;['activeAlerts', 'firingAlerts', 'alertEvents', 'alerts'].forEach(key => {
      if (Array.isArray(evidence[key])) count = Math.max(count, evidence[key].length)
    })
  })
  return count
}

const buildObservabilitySummary = (node, relatedLogCount = 0) => {
  const evidence = evidenceObjects(node)
  const boundLogs = boundLogEvidenceItems(node, evidence)
  const selfLogCount = ['log_store', 'log_project'].includes(node?.sourceType) && !isObservabilityNode(node) ? 1 : 0
  const explicitLogCount = evidence.reduce((count, item) => {
    const value = Number(item.boundLogCount)
    return Number.isFinite(value) ? Math.max(count, value) : count
  }, 0)
  const logCount = Math.max(
    explicitLogCount,
    boundLogs.length,
    selfLogCount,
    relatedLogCount
  )
  const monitorDimension = monitorDimensionForNode(node)
  const monitorStatus = monitorDimension?.status || 'unknown'
  const alertCount = alertCountFromNode(node, evidence)
  const logFiringEvents = evidence.reduce((count, item) => {
    const value = Number(item.firingEvents)
    return Number.isFinite(value) ? Math.max(count, value) : count
  }, 0)
  const explicitAlertSource = firstEvidenceValue(node, ['alertSource', 'alertProvider', 'alertManager', 'alertmanagerSource'], evidence)

  return {
    logCount,
    monitorStatus,
    monitorText: observabilityStatusText(monitorStatus),
    alertCount,
    alertSource: explicitAlertSource || (logFiringEvents > 0 ? '日志告警' : (alertCount > 0 ? 'Alertmanager' : '-'))
  }
}

const relatedLogNodesFor = nodeId => {
  const data = completeTopologyData.value
  const nodeMap = new Map((data.nodes || []).map(node => [String(node.id), node]))
  const result = []
  const seen = new Set()
  const frontier = [String(nodeId || '')]

  while (frontier.length) {
    const sourceId = frontier.shift()
    ;(data.edges || []).forEach(edge => {
      if (String(edge.source) !== sourceId) return
      const targetId = String(edge.target)
      const target = nodeMap.get(targetId)
      if (!target || !isObservabilityNode(target) || seen.has(targetId)) return
      if (edge.relationType !== 'emits_log_to' && !isObservabilityNode(nodeMap.get(sourceId))) return
      seen.add(targetId)
      result.push(target)
      frontier.push(targetId)
    })
  }
  return result
}

const boundLogItemsFor = node => {
  const items = new Map()
  relatedLogNodesFor(node?.id).forEach(item => items.set(String(item.id), item))
  if (['log_store', 'log_project'].includes(node?.sourceType) && !isObservabilityNode(node)) {
    const properties = normalizeProperties(node?.properties)
    const name = node.sourceType === 'log_store'
      ? properties.storeName
      : properties.projectName
    const item = {
      id: node.sourceId,
      sourceKey: node.sourceKey,
      name: name || node.displayName || node.name,
      resourceType: node.sourceType,
      sourceType: node.sourceType,
      sourceId: node.sourceId
    }
    items.set(String(item.sourceKey || item.id || item.name), item)
  }
  boundLogEvidenceItems(node).forEach(item => {
    const key = String(item.id || item.sourceKey || item.name)
    if (!items.has(key)) items.set(key, item)
  })
  return [...items.values()]
}

const formatObservationPercent = value => {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return '--'
  return `${numeric.toFixed(numeric >= 10 ? 1 : 2).replace(/\.?0+$/, '')}%`
}

const formatObservationRate = value => {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) return '--'
  return `${numeric.toFixed(numeric >= 100 ? 0 : 2).replace(/\.?0+$/, '')}/s`
}

const monitorMetricRowsFor = node => [
  { key: 'cpu', label: 'CPU', value: formatObservationPercent(numericEvidenceValue(node, ['cpuUsage', 'cpu', 'cpuPercent'])) },
  { key: 'memory', label: '内存', value: formatObservationPercent(numericEvidenceValue(node, ['memoryUsage', 'memory', 'memoryPercent', 'memPercent'])) },
  { key: 'request', label: '请求量', value: formatObservationRate(numericEvidenceValue(node, ['requestRate', 'requestsPerSecond', 'rps', 'qps', 'requestCount'])) },
  { key: 'error', label: '错误率', value: formatObservationPercent(numericEvidenceValue(node, ['errorRate', 'errorRatio', 'failureRate'])) }
]

const formatLabelSelector = labels => {
  if (!labels) return ''
  if (typeof labels === 'string') return labels
  if (typeof labels !== 'object') return String(labels)
  const preferredKeys = ['instance', 'namespace', 'workload', 'workloadName', 'pod', 'job']
  const entries = Object.entries(labels)
  entries.sort(([left], [right]) => {
    const leftIndex = preferredKeys.indexOf(left)
    const rightIndex = preferredKeys.indexOf(right)
    return (leftIndex < 0 ? 99 : leftIndex) - (rightIndex < 0 ? 99 : rightIndex)
  })
  return entries.slice(0, 4).map(([key, value]) => `${key}=${value}`).join(', ')
}

const monitorMatchTextFor = node => {
  const selector = firstEvidenceValue(node, [
    'runtimeSelector',
    'prometheusLabelSelector',
    'matchedLabels',
    'targetLabels',
    'labelSelector',
    'selector'
  ])
  const formatted = formatLabelSelector(selector)
  if (formatted) return formatted
  const properties = normalizeProperties(node?.properties)
  const ip = properties.ip || properties.privateIp || properties.sshIp || properties.publicIp || node?.ip
  if (ip) return `instance=${ip}`
  const k8sLabels = {
    namespace: properties.namespace || properties.namespaceName,
    workload: properties.workloadName || properties.podName
  }
  const k8sMatch = formatLabelSelector(Object.fromEntries(Object.entries(k8sLabels).filter(([, value]) => value)))
  return k8sMatch || '暂未匹配'
}

const hasValue = value => value !== undefined && value !== null && value !== ''

const inferSourceType = (resourceType, properties = {}) => {
  if (['host', 'physical_server', 'vm', 'virtual', 'cloud_host'].includes(resourceType)) return 'cmdb_host'
  if (resourceType === 'database') return 'db_asset'
  if (resourceType === 'k8s_cluster') return 'k8s_cluster'
  if (resourceType === 'k8s_namespace') return 'k8s_namespace'
  if (['k8s_workload', 'pod'].includes(resourceType)) return 'k8s_workload'
  if (resourceType === 'middleware' && hasValue(properties.assetId)) return 'middleware_asset'
  return ''
}

const inferSourceId = (resourceType, properties = {}) => {
  if (resourceType === 'database') return properties.assetId || properties.databaseId
  if (['host', 'physical_server', 'vm', 'virtual', 'cloud_host'].includes(resourceType)) {
    return properties.hostId || properties.assetId || properties.instanceId
  }
  if (resourceType === 'k8s_cluster') return properties.clusterId
  if (['k8s_workload', 'pod'].includes(resourceType)) return properties.workloadId || properties.podId
  if (resourceType === 'middleware') return properties.assetId || properties.middlewareId
  return undefined
}

const inferSourceKey = (resource, sourceType, sourceId, properties = {}) => {
  const resourceType = resource?.resourceType || resource?.type || ''
  const clusterId = properties.clusterId || sourceId
  const namespaceName = properties.namespace || properties.namespaceName
  const workloadType = properties.workloadType || 'Workload'
  const workloadName = properties.workloadName || properties.podName || resource?.name || resource?.displayName

  if (resourceType === 'k8s_cluster' && hasValue(clusterId)) return `k8s:cluster:${clusterId}`
  if (resourceType === 'k8s_namespace' && hasValue(properties.clusterId) && namespaceName) {
    return `k8s:cluster:${properties.clusterId}:namespace:${namespaceName}`
  }
  if (['k8s_workload', 'pod'].includes(resourceType) && hasValue(properties.clusterId) && namespaceName && workloadName) {
    return `k8s:cluster:${properties.clusterId}:namespace:${namespaceName}:workload:${workloadType}:${workloadName}`
  }
  if (sourceType === 'cmdb_host' && hasValue(sourceId)) return `cmdb_host:${sourceId}`
  if (sourceType === 'db_asset' && hasValue(sourceId)) return `db_asset:${sourceId}`
  if (sourceType === 'middleware_asset' && hasValue(sourceId)) return `middleware_asset:${sourceId}`
  return ''
}

const normalizeSourceIdentity = resource => {
  const properties = normalizeProperties(resource?.properties || resource?.Properties)
  const resourceType = resource?.resourceType || resource?.type || ''
  const sourceType = resource?.sourceType || resource?.SourceType || inferSourceType(resourceType, properties)
  const sourceId = hasValue(resource?.sourceId)
    ? resource.sourceId
    : (hasValue(resource?.SourceId) ? resource.SourceId : inferSourceId(resourceType, properties))
  const sourceKey = resource?.sourceKey || resource?.SourceKey || inferSourceKey(resource, sourceType, sourceId, properties)

  return {
    sourceType,
    sourceId,
    sourceKey,
    sourceName: resource?.sourceName || resource?.SourceName || ''
  }
}

const resolveLayoutPosition = node => {
  const properties = normalizeProperties(node.properties || node.Properties)
  const layout = normalizeProperties(properties.layout)
  const x = Number(node.x ?? node.X ?? layout.x)
  const y = Number(node.y ?? node.Y ?? layout.y)

  if (!Number.isFinite(x) || !Number.isFinite(y)) return {}
  return { x, y }
}

const normalizeResourcePayload = resource => {
  const properties = normalizeProperties(resource.properties || resource.Properties)
  const identity = normalizeSourceIdentity({ ...resource, properties })
  const normalized = {
    id: String(resource.id),
    name: resource.name || resource.label || resource.displayName || '',
    displayName: resource.displayName || resource.label || resource.name || '',
    label: resource.label || resource.displayName || resource.name || '',
    resourceType: resource.resourceType || resource.type || '',
    nodeRole: resource.nodeRole || (observabilityResourceTypes.has(resource.resourceType || resource.type) ? 'observability' : 'primary'),
    type: 'business-status-node',
    healthScore: resource.healthScore ?? '--',
    healthStatus: resource.healthStatus || 'unknown',
    healthColor: resolveHealthColor(resource.healthStatus, resource.healthColor),
    healthSoftColor: resolveHealthSoftColor(resource.healthStatus),
    icon: resolveNodeIcon({ ...resource, properties, ...identity }),
    subtitle: nodeSubtitle({ ...resource, properties }),
    scoreText: formatScore(resource.healthScore),
    layerRank: getLayerRank({ ...resource, properties }),
    size: [146, 138],
    ...resolveLayoutPosition({ ...resource, properties }),
    ...identity,
    properties,
    isDraft: Boolean(resource.isDraft),
    existingResourceId: resource.existingResourceId || null
  }
  normalized.observability = buildObservabilitySummary(normalized)
  return normalized
}

const createDraftNode = ({ resourceType, name, position, resource }) => {
  const tempId = resource?.id ? String(resource.id) : `tmp-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
  return normalizeResourcePayload({
    id: tempId,
    name,
    displayName: resource?.displayName || resource?.label || name,
    label: resource?.label || resource?.displayName || name,
    resourceType,
    healthStatus: resource?.healthStatus || 'unknown',
    healthScore: resource?.healthScore ?? '--',
    healthColor: resource?.healthColor,
    properties: {
      ...(resource?.properties || {}),
      layout: position ? { x: position.x, y: position.y } : undefined
    },
    x: position?.x,
    y: position?.y,
    sourceType: resource?.sourceType,
    sourceName: resource?.sourceName,
    sourceId: resource?.sourceId,
    sourceKey: resource?.sourceKey,
    isDraft: !resource?.id,
    existingResourceId: resource?.id ? Number(resource.id) : null
  })
}

const createDraftEdge = (source, target, options = {}) => ({
  id: options.id || `draft-edge-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
  type: 'line',
  source: String(source),
  target: String(target),
  label: options.label || relationTypeText(options.relationType),
  relationType: options.relationType || undefined,
  observability: options.relationType === 'emits_log_to',
  isDraft: Boolean(options.isDraft),
  style: options.relationType === 'emits_log_to'
    ? { stroke: '#64748b', lineDash: [3, 4], lineWidth: 1.2 }
    : (options.dependency ? { stroke: '#8aa4c7', lineDash: [6, 4] } : undefined),
  controlPoints: undefined
})

const transformTopology = rawData => {
  const nodes = (rawData.nodes || []).map(node => {
    const properties = normalizeProperties(node.properties || node.Properties)
    const identity = normalizeSourceIdentity({ ...node, properties })
    return {
      ...node,
      properties,
      ...identity,
      id: String(node.id),
      type: 'business-status-node',
      nodeRole: node.nodeRole || (observabilityResourceTypes.has(node.resourceType) ? 'observability' : 'primary'),
      size: [146, 138],
      layerRank: getLayerRank({ ...node, properties }),
      healthColor: resolveHealthColor(node.healthStatus, node.healthColor),
      healthSoftColor: resolveHealthSoftColor(node.healthStatus),
      icon: resolveNodeIcon({ ...node, properties, ...identity }),
      subtitle: nodeSubtitle({ ...node, properties }),
      scoreText: formatScore(node.healthScore),
      ...resolveLayoutPosition({ ...node, properties })
    }
  }).sort((left, right) => {
    if (left.layerRank !== right.layerRank) return left.layerRank - right.layerRank
    return String(left.id).localeCompare(String(right.id))
  })

  const nodeIds = new Set(nodes.map(item => item.id))
  const nodeMap = new Map(nodes.map(item => [item.id, item]))
  const edges = (rawData.edges || [])
    .map(edge => {
      const originalSource = String(edge.source ?? edge.sourceId ?? edge.srcId)
      const originalTarget = String(edge.target ?? edge.targetId ?? edge.dstId)
      const relationId = edge.relationId ?? edge.id
      const rawEdgeId = String(edge.id ?? `${originalSource}-${originalTarget}`)
      const graphEdgeId = rawEdgeId.startsWith('edge-') || rawEdgeId.startsWith('draft-edge-')
        ? rawEdgeId
        : `edge-${rawEdgeId}`

      return {
        ...edge,
        id: graphEdgeId,
        relationId,
        type: 'line',
        source: originalSource,
        target: originalTarget,
        label: relationTypeText(edge.relationType) || edge.label || '',
        observability: edge.relationType === 'emits_log_to',
        style: edge.relationType === 'emits_log_to'
          ? { stroke: '#64748b', lineDash: [3, 4], lineWidth: 1.2 }
          : (isDependencyEdge(edge, nodeMap) ? { stroke: '#8aa4c7', lineDash: [6, 4] } : undefined),
        controlPoints: undefined
      }
    })
    .filter(edge => nodeIds.has(edge.source) && nodeIds.has(edge.target))

  const directLogCounts = new Map()
  edges.forEach(edge => {
    if (edge.relationType !== 'emits_log_to' || !isObservabilityNode(nodeMap.get(String(edge.target)))) return
    const sourceId = String(edge.source)
    directLogCounts.set(sourceId, (directLogCounts.get(sourceId) || 0) + 1)
  })
  nodes.forEach(node => {
    node.observability = buildObservabilitySummary(node, directLogCounts.get(String(node.id)) || 0)
  })

  const hasSavedLayout = nodes.some(node => Number.isFinite(Number(node.x)) && Number.isFinite(Number(node.y)))

  return { nodes, edges, hasSavedLayout }
}

const resolveNodeIcon = node => {
  const type = node?.resourceType
  const source = `${node?.sourceType || ''} ${node?.sourceName || ''} ${node?.name || ''} ${node?.displayName || ''}`.toLowerCase()

  if (source.includes('mysql')) return mysqlIcon
  if (source.includes('mongo')) return mongoIcon
  if (source.includes('redis')) return redisIcon
  if (node?.dimension === 'log') return logIcon

  const iconMap = {
    business_system: appIcon,
    business_service: appIcon,
    application: appIcon,
    k8s_cluster: k8sIcon,
    k8s_namespace: namespaceIcon,
    k8s_workload: workloadIcon,
    pod: workloadIcon,
    container: workloadIcon,
    host: linuxIcon,
    physical_server: physicalServerIcon,
    vm: virtualIcon,
    virtual: virtualIcon,
    cloud_host: cloudHostIcon,
    database: databaseIcon,
    middleware: middlewareIcon,
    nginx: nginxIcon,
    gateway: gatewayIcon,
    log_project: logIcon,
    log_store: logIcon,
    redis: redisIcon,
    log: logIcon
  }

  return iconMap[type] || appIcon
}

const nodeSubtitle = node => {
  const properties = normalizeProperties(node?.properties || node?.Properties)
  return properties.endpoint || properties.privateIp || properties.namespace || properties.workloadType || node?.ip || resourceTypeText(node?.resourceType)
}

const getLayerRank = resource => {
  const resourceType = typeof resource === 'string' ? resource : resource?.resourceType
  const properties = normalizeProperties(typeof resource === 'string' ? {} : resource?.properties)
  const isHostService = resourceType === 'business_service' && (properties.deployMode === 'host' || hasValue(properties.hostId))
  const rankMap = {
    business_system: 0,
    business_service: isHostService ? 4 : 1,
    application: 1,
    nginx: 1,
    gateway: 2,
    k8s_cluster: 3,
    host: 3,
    physical_server: 3,
    vm: 3,
    virtual: 3,
    cloud_host: 3,
    k8s_namespace: 4,
    k8s_workload: 5,
    pod: 5,
    database: 6,
    middleware: 6,
    redis: 6,
    kafka: 6,
    log_project: 7,
    log_store: 7
  }
  return rankMap[resourceType] ?? 6
}

const isDependencyEdge = (edge, nodeMap) => {
  const source = nodeMap.get(String(edge.source))
  const target = nodeMap.get(String(edge.target))
  const chainPairs = new Set([
    'business_system:business_service',
    'business_system:k8s_cluster',
    'business_system:host',
    'business_system:physical_server',
    'business_system:vm',
    'business_system:virtual',
    'business_system:cloud_host',
    'business_system:nginx',
    'business_system:gateway',
    'nginx:gateway',
    'gateway:k8s_cluster',
    'gateway:host',
    'gateway:physical_server',
    'gateway:vm',
    'gateway:virtual',
    'gateway:cloud_host',
    'business_service:k8s_cluster',
    'business_service:host',
    'business_service:physical_server',
    'business_service:vm',
    'business_service:virtual',
    'business_service:cloud_host',
    'k8s_cluster:k8s_namespace',
    'k8s_namespace:k8s_workload',
    'k8s_namespace:pod',
    'host:business_service',
    'physical_server:business_service',
    'vm:business_service',
    'virtual:business_service',
    'cloud_host:business_service'
  ])
  const pairKey = `${source?.resourceType}:${target?.resourceType}`
  if (chainPairs.has(pairKey)) return false
  if (dependencyResourceTypes.has(target?.resourceType)) return true
  return dependencyRelationTypes.has(edge.relationType)
}

const registerNode = () => {
  if (graphNodeRegistered) return

  G6.registerNode('business-status-node', {
    draw(cfg, group) {
      const color = cfg.healthColor || '#94a3b8'
      const softColor = cfg.healthSoftColor || 'rgba(148, 163, 184, 0.14)'
      const label = cfg.displayName || cfg.label || cfg.name || '未命名节点'
      const subtitle = cfg.subtitle || resourceTypeText(cfg.resourceType)
      const scoreText = cfg.scoreText === '--' ? '' : `${cfg.scoreText}`
      const observability = cfg.observability || buildObservabilitySummary(cfg)

      group.addShape('circle', {
        attrs: {
          x: 0,
          y: -12,
          r: 33,
          fill: '#ffffff',
          stroke: color,
          lineWidth: cfg.healthStatus === 'critical' ? 3.2 : 2.4,
          shadowColor: 'rgba(15, 23, 42, 0.10)',
          shadowBlur: 18,
          shadowOffsetY: 10,
          cursor: 'pointer'
        },
        name: 'node-ring'
      })

      group.addShape('circle', {
        attrs: {
          x: 0,
          y: -12,
          r: 25,
          fill: softColor,
          stroke: 'rgba(255, 255, 255, 0.94)',
          lineWidth: 1
        },
        name: 'node-icon-bg'
      })

      group.addShape('image', {
        attrs: {
          x: -17,
          y: -29,
          width: 34,
          height: 34,
          img: cfg.icon,
          cursor: 'pointer'
        },
        name: 'node-icon'
      })

      group.addShape('circle', {
        attrs: {
          x: 25,
          y: -35,
          r: 5.5,
          fill: color,
          stroke: '#ffffff',
          lineWidth: 2
        },
        name: 'health-dot'
      })

      group.addShape('text', {
        attrs: {
          x: 0,
          y: 31,
          text: label.length > 16 ? `${label.slice(0, 16)}...` : label,
          fill: '#172133',
          fontSize: 12,
          fontWeight: 700,
          textAlign: 'center',
          textBaseline: 'middle'
        },
        name: 'title'
      })

      group.addShape('text', {
        attrs: {
          x: 0,
          y: 47,
          text: `${subtitle}`.length > 18 ? `${subtitle}`.slice(0, 18) : `${subtitle}`,
          fill: '#728197',
          fontSize: 9,
          textAlign: 'center',
          textBaseline: 'middle'
        },
        name: 'subtitle'
      })

      if (cfg.nodeRole !== 'observability') {
        const monitorColor = resolveHealthColor(observability.monitorStatus)
        ;[
          { x: -48, text: `日志 ${observability.logCount}`, fill: observability.logCount > 0 ? '#475569' : '#94a3b8' },
          { x: 0, text: `监控${observability.monitorText}`, fill: monitorColor },
          { x: 48, text: `告警 ${observability.alertCount}`, fill: observability.alertCount > 0 ? '#dc2626' : '#94a3b8' }
        ].forEach(item => {
          group.addShape('text', {
            attrs: {
              x: item.x,
              y: 65,
              text: item.text,
              fill: item.fill,
              fontSize: 8.5,
              fontWeight: 600,
              textAlign: 'center',
              textBaseline: 'middle'
            },
            name: 'observability-summary'
          })
        })
      }

      if (scoreText) {
        group.addShape('rect', {
          attrs: {
            x: -18,
            y: -57,
            width: 36,
            height: 18,
            radius: 9,
            fill: '#ffffff',
            stroke: 'rgba(223, 231, 242, 0.96)',
            lineWidth: 1
          },
          name: 'score-pill'
        })

        group.addShape('text', {
          attrs: {
            x: 0,
            y: -48,
            text: scoreText,
            fill: color,
            fontSize: 10,
            fontWeight: 700,
            textAlign: 'center',
            textBaseline: 'middle'
          },
          name: 'score'
        })
      }

      return group.get('children')[0]
    },
    setState(name, value, item) {
      const group = item.getContainer()
      const shape = group.find(element => element.get('name') === 'node-ring')
      if (shape && name === 'selected') {
        shape.attr({
          lineWidth: value ? 4 : 2.4,
          shadowColor: value ? 'rgba(76, 139, 245, 0.26)' : 'rgba(15, 23, 42, 0.10)',
          shadowBlur: value ? 30 : 18
        })
      }
    }
  })

  graphNodeRegistered = true
}

const initGraph = () => {
  if (!graphContainer.value) return

  registerNode()
  if (graph) graph.destroy()

  const tooltip = new G6.Tooltip({
    offsetX: 12,
    offsetY: 12,
    itemTypes: ['node'],
    getContent: event => {
      const model = event.item?.getModel?.() || {}
      return `
        <div style="padding: 12px 14px; max-width: 280px; color: #172133;">
          <strong>${model.displayName || model.label || model.name || '未命名节点'}</strong>
          <div style="margin-top: 8px; color: #5f6b7c; font-size: 12px; line-height: 1.7;">
            <div>资源类型：${resourceTypeText(model.resourceType)}</div>
            <div>健康状态：${healthStatusText(model.healthStatus)}</div>
            <div>健康分：${formatScore(model.healthScore)}</div>
            <div>日志：${model.observability?.logCount || 0} · 监控：${model.observability?.monitorText || '未接入'} · 告警：${model.observability?.alertCount || 0}</div>
            <div>${model.problemSummary || model.message || '暂无异常摘要'}</div>
          </div>
        </div>
      `
    }
  })

  graph = new G6.Graph({
    container: graphContainer.value,
    width: graphContainer.value.clientWidth || 900,
    height: graphContainer.value.clientHeight || 620,
    fitView: true,
    fitViewPadding: 30,
    modes: {
      default: ['drag-canvas', 'zoom-canvas']
    },
    layout: isEditing.value || topologyData.value.hasSavedLayout ? null : {
      type: 'dagre',
      rankdir: 'LR',
      nodesep: 56,
      ranksep: 128,
      sortBy: 'layerRank',
      ranker: 'network-simplex',
      controlPoints: false
    },
    defaultNode: {
      type: 'business-status-node'
    },
    defaultEdge: {
      type: 'line',
      style: {
        stroke: '#9fb3cf',
        lineWidth: 1.4,
        lineAppendWidth: 10,
        endArrow: {
          path: G6.Arrow.triangle(7, 9, 4),
          fill: '#9fb3cf'
        }
      },
      labelCfg: {
        autoRotate: true,
        style: {
          fill: 'transparent',
          fontSize: 10,
          background: {
            fill: 'transparent',
            padding: [3, 6, 3, 6],
            radius: 4
          }
        }
      }
    },
    plugins: [tooltip]
  })

  graph.on('node:click', event => {
    const model = event.item?.getModel?.()
    if (model) {
      selectedNode.value = model
      focusNode(model.id, { moveCanvas: false })
    }
  })

  graph.on('node:mousedown', event => {
    const model = event.item?.getModel?.()
    if (!model) return
    closeContextMenu()
    dragState.active = true
    dragState.item = event.item
    dragState.nodeId = String(model.id)
    dragState.startX = event.x
    dragState.startY = event.y
    dragState.nodeX = Number(model.x || 0)
    dragState.nodeY = Number(model.y || 0)
    selectedNode.value = model
  })

  graph.on('mousemove', event => {
    if (!dragState.active || !dragState.item) return
    const x = dragState.nodeX + event.x - dragState.startX
    const y = dragState.nodeY + event.y - dragState.startY
    graph.updateItem(dragState.item, { x, y })
    graph.refreshPositions()
  })

  graph.on('mouseup', () => {
    finishNodeDrag()
  })

  graph.on('canvas:mouseup', () => {
    finishNodeDrag()
  })

  graph.on('node:contextmenu', event => {
    event.preventDefault?.()
    const model = event.item?.getModel?.()
    if (model) {
      selectedNode.value = model
      focusNode(model.id, { moveCanvas: false })
      if (isEditing.value || canOpenNodeDetail(model)) {
        showContextMenu(event, { node: model })
      }
    }
  })

  graph.on('edge:contextmenu', event => {
    if (!isEditing.value) return
    event.preventDefault?.()
    const model = event.item?.getModel?.()
    if (model) showContextMenu(event, { edge: model })
  })

  graph.on('canvas:contextmenu', event => {
    if (!isEditing.value) return
    event.preventDefault?.()
    showContextMenu(event, { canvasPoint: { x: event.x, y: event.y } })
  })

  graph.on('edge:mouseenter', event => {
    const edge = event.item
    const model = edge?.getModel?.()
    if (!edge || !model?.label) return
    graph.updateItem(edge, {
      labelCfg: {
        autoRotate: true,
        style: {
          fill: '#7b8698',
          fontSize: 10,
          background: {
            fill: '#ffffff',
            padding: [3, 6, 3, 6],
            radius: 4
          }
        }
      }
    })
  })

  graph.on('edge:mouseleave', event => {
    const edge = event.item
    if (!edge) return
    graph.updateItem(edge, {
      labelCfg: {
        autoRotate: true,
        style: {
          fill: 'transparent',
          fontSize: 10,
          background: {
            fill: 'transparent',
            padding: [3, 6, 3, 6],
            radius: 4
          }
        }
      }
    })
  })

}

const finishNodeDrag = () => {
  if (!dragState.active || !dragState.item) return
  const model = dragState.item.getModel?.()
  if (model) {
    const node = topologyData.value.nodes.find(item => item.id === String(model.id))
    if (node) {
      node.x = model.x
      node.y = model.y
      node.properties = {
        ...(node.properties || {}),
        layout: { x: model.x, y: model.y }
      }
    }
    if (isEditing.value) {
      recordMoveNode(model.id, model.x, model.y)
    }
  }
  dragState.active = false
  dragState.item = null
  dragState.nodeId = ''
}

const renderGraph = async () => {
  await nextTick()
  if (!graphContainer.value) return
  if (!graph) initGraph()
  if (!graph) return

  graph.changeSize(graphContainer.value.clientWidth || 900, graphContainer.value.clientHeight || 620)
  graph.data(topologyData.value)
  graph.render()
  if (!isEditing.value) {
    graph.fitView(30)
  }
}

const toggleSelectedLogs = async () => {
  const nodeId = String(selectedNode.value?.id || '')
  if (!nodeId || !selectedBoundLogs.value.length) return

  expandedLogResourceIds.value = selectedLogsExpanded.value
    ? expandedLogResourceIds.value.filter(id => id !== nodeId)
    : [...expandedLogResourceIds.value, nodeId]
  topologyData.value = projectVisibleTopology(completeTopologyData.value)
  selectedNode.value = topologyData.value.nodes.find(node => String(node.id) === nodeId) || topologyData.value.nodes[0] || null
  await renderGraph()
  focusNode(nodeId, { moveCanvas: false })
}

const syncGraphPositionsToTopology = () => {
  if (!graph) return
  const positionMap = new Map(graph.getNodes().map(node => {
    const model = node.getModel()
    return [String(model.id), { x: model.x, y: model.y }]
  }))
  topologyData.value.nodes = topologyData.value.nodes.map(node => {
    const position = positionMap.get(String(node.id))
    if (!position) return node
    return {
      ...node,
      x: position.x,
      y: position.y,
      properties: {
        ...(node.properties || {}),
        layout: { x: position.x, y: position.y }
      }
    }
  })
  topologyData.value.hasSavedLayout = true
}

const focusNode = (id, options = {}) => {
  if (!graph) return
  const { moveCanvas = true } = options
  graph.getNodes().forEach(node => {
    const matched = node.getID() === String(id)
    graph.setItemState(node, 'selected', matched)
    if (matched) {
      selectedNode.value = node.getModel()
      if (moveCanvas) {
        graph.focusItem(node, true, { duration: 500 })
      }
    }
  })
}

const focusOrOpenNode = id => {
  focusNode(id)
}

const destroyGraph = () => {
  if (graph) {
    graph.destroy()
    graph = null
  }
}

const rerenderWithMode = async () => {
  destroyGraph()
  await renderGraph()
}

const enterEditMode = async () => {
  syncGraphPositionsToTopology()
  originalTopologyData.value = cloneTopology(projectVisibleTopology(completeTopologyData.value, []))
  expandedLogResourceIds.value = [...new Set(
    completeTopologyData.value.edges
      .filter(edge => edge.relationType === 'emits_log_to')
      .map(edge => String(edge.source))
  )]
  topologyData.value = projectVisibleTopology(completeTopologyData.value)
  isEditing.value = true
  draftChanges.value = []
  redoStack.value = []
  closeContextMenu()
  await rerenderWithMode()
}

const cancelEditMode = async () => {
  topologyData.value = cloneTopology(originalTopologyData.value)
  expandedLogResourceIds.value = []
  selectedNode.value = topologyData.value.nodes[0] || null
  isEditing.value = false
  draftChanges.value = []
  redoStack.value = []
  closeContextMenu()
  await rerenderWithMode()
}

const pushDraftChange = change => {
  draftChanges.value.push({
    id: `draft-${Date.now()}-${Math.random().toString(36).slice(2, 7)}`,
    ...change
  })
  redoStack.value = []
}

const applyDraftChange = change => {
  if (change.type === 'createResource') {
    const nodes = (change.nodes || [change.node]).filter(Boolean)
    const edges = (change.edges || (change.edge ? [change.edge] : [])).filter(Boolean)
    nodes.forEach(node => {
      if (!topologyData.value.nodes.some(item => String(item.id) === String(node.id))) {
        topologyData.value.nodes.push(node)
      }
    })
    edges.forEach(edge => {
      if (!topologyData.value.edges.some(item => String(item.id) === String(edge.id))) {
        topologyData.value.edges.push(edge)
      }
    })
  }

  if (change.type === 'deleteRelation') {
    topologyData.value.edges = topologyData.value.edges.filter(edge => edge.id !== change.edge.id)
  }

  if (change.type === 'deleteNode') {
    const nodeIds = new Set((change.nodes || [change.node]).filter(Boolean).map(node => String(node.id)))
    topologyData.value.nodes = topologyData.value.nodes.filter(node => !nodeIds.has(String(node.id)))
    topologyData.value.edges = topologyData.value.edges.filter(edge => !nodeIds.has(String(edge.source)) && !nodeIds.has(String(edge.target)))
  }

  if (change.type === 'moveNode') {
    const node = topologyData.value.nodes.find(item => item.id === String(change.nodeId))
    if (node) {
      node.x = change.to.x
      node.y = change.to.y
      node.properties = {
        ...(node.properties || {}),
        layout: { x: change.to.x, y: change.to.y }
      }
    }
  }
}

const revertDraftChange = change => {
  if (change.type === 'createResource') {
    const nodeIds = new Set((change.nodes || [change.node]).filter(Boolean).map(node => String(node.id)))
    const edgeIds = new Set((change.edges || (change.edge ? [change.edge] : [])).filter(Boolean).map(edge => String(edge.id)))
    topologyData.value.nodes = topologyData.value.nodes.filter(node => !nodeIds.has(String(node.id)))
    topologyData.value.edges = topologyData.value.edges.filter(edge => !edgeIds.has(String(edge.id)))
  }

  if (change.type === 'deleteRelation') {
    topologyData.value.edges.push(change.edge)
  }

  if (change.type === 'deleteNode') {
    const restoredNodes = (change.nodes || [change.node]).filter(Boolean)
    const restoredEdges = change.edges || []
    restoredNodes.forEach(node => {
      if (!topologyData.value.nodes.some(item => String(item.id) === String(node.id))) topologyData.value.nodes.push(node)
    })
    restoredEdges.forEach(edge => {
      if (!topologyData.value.edges.some(item => String(item.id) === String(edge.id))) topologyData.value.edges.push(edge)
    })
  }

  if (change.type === 'moveNode') {
    const node = topologyData.value.nodes.find(item => item.id === String(change.nodeId))
    if (node) {
      node.x = change.from.x
      node.y = change.from.y
      node.properties = {
        ...(node.properties || {}),
        layout: { x: change.from.x, y: change.from.y }
      }
    }
  }
}

const undoDraft = async () => {
  const change = draftChanges.value.pop()
  if (!change) return
  revertDraftChange(change)
  redoStack.value.push(change)
  await renderGraph()
}

const redoDraft = async () => {
  const change = redoStack.value.pop()
  if (!change) return
  applyDraftChange(change)
  draftChanges.value.push(change)
  await renderGraph()
}

const showContextMenu = (event, payload = {}) => {
  const containerRect = graphContainer.value?.getBoundingClientRect?.()
  const clientX = event.clientX ?? event.originalEvent?.clientX ?? 0
  const clientY = event.clientY ?? event.originalEvent?.clientY ?? 0

  contextMenu.visible = true
  contextMenu.x = containerRect ? clientX - containerRect.left : event.x || 0
  contextMenu.y = containerRect ? clientY - containerRect.top : event.y || 0
  contextMenu.node = payload.node || null
  contextMenu.edge = payload.edge || null
  contextMenu.canvasPoint = payload.canvasPoint || { x: event.x || 0, y: event.y || 0 }
}

const closeContextMenu = () => {
  contextMenu.visible = false
  contextMenu.node = null
  contextMenu.edge = null
}

const handleContextMenuAction = async action => {
  const node = contextMenu.node
  const edge = contextMenu.edge
  closeContextMenu()

  if (action === 'deploy-k8s') openDeploymentWizard('k8s', node)
  if (action === 'deploy-host') openDeploymentWizard('host', node)
  if (action === 'add-nginx') openDeploymentWizard('nginx', node)
  if (action === 'add-gateway') openDeploymentWizard('gateway', node)
  if (action === 'add-dependency') openDeploymentWizard('dependency', node)
  if (action === 'bind-log') openDeploymentWizard('log', node)
  if (action === 'add-child') openChildWizard(node)
  if (action === 'add-sibling') openSiblingWizard(node)
  if (action === 'view-detail' && node) {
    selectedNode.value = node
    if (canOpenNodeDetail(node)) await openNodeDetail(node)
  }
  if (action === 'delete-node') deleteDraftNode(node)
  if (action === 'delete-edge') deleteDraftEdge(edge)
}

const findParentNodeId = nodeId => {
  const id = String(nodeId || '')
  const incomingEdge = topologyData.value.edges.find(item => String(item.target) === id)
  if (incomingEdge?.source) return incomingEdge.source

  const currentNode = topologyData.value.nodes.find(item => String(item.id) === id)
  const currentRank = getLayerRank(currentNode?.resourceType)
  const neighborIds = topologyData.value.edges
    .filter(edge => String(edge.source) === id || String(edge.target) === id)
    .map(edge => String(edge.source) === id ? edge.target : edge.source)
  const parentPriority = {
    business_system: 0,
    nginx: 1,
    gateway: 2,
    host: 1,
    physical_server: 1,
    vm: 1,
    virtual: 1,
    k8s_namespace: 1,
    k8s_cluster: 2,
    application: 3,
    business_service: 4
  }
  const candidates = neighborIds
    .map(neighborId => topologyData.value.nodes.find(node => String(node.id) === String(neighborId)))
    .filter(Boolean)
    .filter(node => getLayerRank(node.resourceType) <= currentRank)
    .sort((left, right) => {
      const leftPriority = parentPriority[left.resourceType] ?? 9
      const rightPriority = parentPriority[right.resourceType] ?? 9
      if (leftPriority !== rightPriority) return leftPriority - rightPriority
      return getLayerRank(left.resourceType) - getLayerRank(right.resourceType)
    })

  return candidates[0]?.id || null
}

const DEFAULT_CHILD_OFFSET_X = 180
const DEFAULT_CHILD_OFFSET_Y = 80
const DEFAULT_SIBLING_GAP_Y = 150
const NODE_COLLISION_WIDTH = 146
const NODE_COLLISION_HEIGHT = 118

const toFiniteNumber = value => {
  const number = Number(value)
  return Number.isFinite(number) ? number : null
}

const getGraphNodePosition = nodeId => {
  if (!graph || !nodeId) return null
  const graphNode = graph.findById?.(String(nodeId))
  const model = graphNode?.getModel?.()
  const x = toFiniteNumber(model?.x)
  const y = toFiniteNumber(model?.y)
  if (x === null || y === null) return null
  return { x, y }
}

const getCanvasPosition = node => {
  const x = toFiniteNumber(node?.x ?? node?.properties?.layout?.x)
  const y = toFiniteNumber(node?.y ?? node?.properties?.layout?.y)
  if (x === null || y === null) return getGraphNodePosition(node?.id)
  return { x, y }
}

const collectOccupiedPositions = (reservedNodes = []) => {
  const positions = []
  topologyData.value.nodes.forEach(node => {
    const position = getCanvasPosition(node)
    if (position) positions.push(position)
  })
  graph?.getNodes?.().forEach(item => {
    const model = item?.getModel?.()
    const x = toFiniteNumber(model?.x)
    const y = toFiniteNumber(model?.y)
    if (x !== null && y !== null) positions.push({ x, y })
  })
  reservedNodes.forEach(node => {
    const position = getCanvasPosition(node)
    if (position) positions.push(position)
  })
  return positions
}

const hasNodeOverlap = (position, occupiedPositions = collectOccupiedPositions()) => occupiedPositions.some(nodePosition => (
  Math.abs(nodePosition.x - position.x) < NODE_COLLISION_WIDTH
  && Math.abs(nodePosition.y - position.y) < NODE_COLLISION_HEIGHT
))

const getDefaultPosition = (parentNode, index = 1, reservedNodes = []) => {
  const parentPosition = getCanvasPosition(parentNode)
  const baseX = parentPosition?.x ?? Number(graphContainer.value?.clientWidth / 2 || 400)
  const baseY = parentPosition?.y ?? Number(graphContainer.value?.clientHeight / 2 || 260)
  const parentId = String(parentNode?.id || '')
  const occupiedPositions = collectOccupiedPositions(reservedNodes)
  const siblingPositions = topologyData.value.edges
    .filter(edge => String(edge.source) === parentId)
    .map(edge => topologyData.value.nodes.find(node => String(node.id) === String(edge.target)))
    .filter(Boolean)
    .map(node => getCanvasPosition(node))
    .filter(Boolean)
    .sort((left, right) => left.y - right.y)

  const position = siblingPositions.length
    ? {
        x: baseX + DEFAULT_CHILD_OFFSET_X,
        y: siblingPositions[siblingPositions.length - 1].y + DEFAULT_SIBLING_GAP_Y
      }
    : {
        x: baseX + DEFAULT_CHILD_OFFSET_X,
        y: baseY + DEFAULT_CHILD_OFFSET_Y + Math.max(index - 1, 0) * DEFAULT_SIBLING_GAP_Y
      }

  while (hasNodeOverlap(position, occupiedPositions)) {
    position.y += DEFAULT_SIBLING_GAP_Y
  }

  return position
}

const resetDeploymentDialog = () => {
  deploymentDialog.k8s.clusterId = ''
  deploymentDialog.k8s.namespaceName = ''
  deploymentDialog.k8s.targetKey = ''
  deploymentDialog.k8s.targetKeys = []
  deploymentDialog.k8s.namespaces = []
  deploymentDialog.k8s.targets = []
  deploymentDialog.k8s.targetKeyword = ''
  deploymentDialog.k8s.loadingNamespaces = false
  deploymentDialog.k8s.loadingTargets = false
  deploymentDialog.host.hostKey = ''
  deploymentDialog.host.hostKeyword = ''
  deploymentDialog.host.options = []
  deploymentDialog.host.loadingHosts = false
  deploymentDialog.host.serviceName = ''
  deploymentDialog.host.serviceNames = []
  deploymentDialog.host.services = []
  deploymentDialog.host.loadingServices = false
  deploymentDialog.child.resourceType = ''
  deploymentDialog.child.namespaceName = ''
  deploymentDialog.child.namespaces = []
  deploymentDialog.child.loadingNamespaces = false
  deploymentDialog.child.targetKeys = []
  deploymentDialog.child.targets = []
  deploymentDialog.child.targetKeyword = ''
  deploymentDialog.child.loadingTargets = false
  deploymentDialog.child.serviceNames = []
  deploymentDialog.child.services = []
  deploymentDialog.child.loadingServices = false
  deploymentDialog.child.clusterId = ''
  deploymentDialog.child.clusterName = ''
  deploymentDialog.child.hostId = ''
  deploymentDialog.child.hostName = ''
  deploymentDialog.child.hostIp = ''
  deploymentDialog.sibling.resourceType = ''
  deploymentDialog.sibling.name = ''
  deploymentDialog.sibling.currentSourceId = ''
  deploymentDialog.sibling.currentName = ''
  deploymentDialog.sibling.useK8sSearch = false
  deploymentDialog.sibling.clusterId = ''
  deploymentDialog.sibling.clusterName = ''
  deploymentDialog.sibling.namespaceName = ''
  deploymentDialog.sibling.clusterIds = []
  deploymentDialog.sibling.clusters = []
  deploymentDialog.sibling.loadingClusters = false
  deploymentDialog.sibling.namespaceNames = []
  deploymentDialog.sibling.namespaces = []
  deploymentDialog.sibling.loadingNamespaces = false
  deploymentDialog.sibling.targetKey = ''
  deploymentDialog.sibling.targetKeys = []
  deploymentDialog.sibling.targets = []
  deploymentDialog.sibling.targetKeyword = ''
  deploymentDialog.sibling.loadingTargets = false
  deploymentDialog.sibling.hostId = ''
  deploymentDialog.sibling.hostName = ''
  deploymentDialog.sibling.hostIp = ''
  deploymentDialog.sibling.hostKeys = []
  deploymentDialog.sibling.hosts = []
  deploymentDialog.sibling.hostKeyword = ''
  deploymentDialog.sibling.loadingHosts = false
  deploymentDialog.sibling.serviceNames = []
  deploymentDialog.sibling.services = []
  deploymentDialog.sibling.loadingServices = false
  deploymentDialog.log.resourceType = 'log_store'
  deploymentDialog.log.projectId = ''
  deploymentDialog.log.projectIds = []
  deploymentDialog.log.storeIds = []
  deploymentDialog.log.projects = []
  deploymentDialog.log.stores = []
  deploymentDialog.log.loadingProjects = false
  deploymentDialog.log.loadingStores = false
  deploymentDialog.entry.resourceType = 'nginx'
  deploymentDialog.entry.name = ''
  deploymentDialog.entry.logResourceType = 'log_store'
  deploymentDialog.entry.projectId = ''
  deploymentDialog.entry.projectSourceId = ''
  deploymentDialog.entry.storeId = ''
  deploymentDialog.dependency.resourceType = 'database'
  deploymentDialog.dependency.mode = 'create'
  deploymentDialog.dependency.name = ''
  deploymentDialog.dependency.keyword = ''
  deploymentDialog.dependency.searching = false
  deploymentDialog.dependency.options = []
  deploymentDialog.dependency.selectedResource = null
  deploymentDialog.dependency.targetId = rootNodeId.value
  deploymentDialog.dependency.dbAssetKey = ''
  deploymentDialog.dependency.dbAssetKeyword = ''
  deploymentDialog.dependency.dbAssets = []
  deploymentDialog.dependency.loadingDbAssets = false
  deploymentDialog.dependency.dbType = ''
}

const openDeploymentWizard = (type = 'k8s', node = null) => {
  resetDeploymentDialog()
  deploymentDialog.visible = true
  deploymentDialog.type = type
  deploymentDialog.parentNode = node
  if (type === 'k8s') {
    loadK8sClusters()
  }
  if (type === 'log') {
    loadLogProjects()
  }
  if (['nginx', 'gateway'].includes(type)) {
    deploymentDialog.entry.resourceType = type
    loadLogProjects()
  }
  if (type === 'dependency' && node?.id) {
    deploymentDialog.dependency.targetId = node.id
  }
}

const loadLogProjects = async () => {
  deploymentDialog.log.loadingProjects = true
  deploymentDialog.log.projects = []
  try {
    const res = normalizeResponse(await logtailApi.getProjects({ page: 1, pageSize: 200, status: 1 }))
    if (!isSuccessResponse(res)) throw new Error(res?.message || '获取日志项目失败')
    deploymentDialog.log.projects = pickList(res.data, ['projects', 'records', 'rows'])
      .map(normalizeLogProject)
      .filter(item => item.id && item.name)
  } catch (error) {
    console.error('获取日志项目失败:', error)
    deploymentDialog.log.projects = []
    ElMessage.warning(error.message || '获取日志项目失败')
  } finally {
    deploymentDialog.log.loadingProjects = false
  }
}

const loadLogStores = async projectId => {
  deploymentDialog.log.stores = []
  if (!projectId) return
  deploymentDialog.log.loadingStores = true
  try {
    const res = normalizeResponse(await logtailApi.getStores({ page: 1, pageSize: 200, projectId, status: 1 }))
    if (!isSuccessResponse(res)) throw new Error(res?.message || '获取日志库失败')
    deploymentDialog.log.stores = pickList(res.data, ['stores', 'records', 'rows'])
      .map(normalizeLogStore)
      .filter(item => item.id && item.name)
  } catch (error) {
    console.error('获取日志库失败:', error)
    deploymentDialog.log.stores = []
    ElMessage.warning(error.message || '获取日志库失败')
  } finally {
    deploymentDialog.log.loadingStores = false
  }
}

const handleLogResourceTypeChange = () => {
  deploymentDialog.log.projectId = ''
  deploymentDialog.log.projectIds = []
  deploymentDialog.log.storeIds = []
  deploymentDialog.log.stores = []
}

const handleLogProjectChange = projectId => {
  deploymentDialog.log.storeIds = []
  loadLogStores(projectId)
}

const handleEntryLogTypeChange = () => {
  deploymentDialog.entry.projectId = ''
  deploymentDialog.entry.projectSourceId = ''
  deploymentDialog.entry.storeId = ''
  deploymentDialog.log.stores = []
}

const handleEntryProjectChange = projectId => {
  deploymentDialog.entry.storeId = ''
  loadLogStores(projectId)
}

const loadK8sClusters = async () => {
  if (deploymentDialog.k8s.clusters.length) return
  deploymentDialog.k8s.loadingClusters = true
  try {
    const res = normalizeResponse(await k8sApi.getClusterList({ page: 1, size: 200 }))
    if (!isSuccessResponse(res)) throw new Error(res?.message || '获取 K8S 集群失败')
    deploymentDialog.k8s.clusters = pickList(res.data, ['clusters']).map(normalizeK8sCluster).filter(item => item.id !== undefined && item.id !== null && item.name)
  } catch (error) {
    console.error('获取 K8S 集群失败:', error)
    deploymentDialog.k8s.clusters = []
    ElMessage.warning(error.message || '获取 K8S 集群失败')
  } finally {
    deploymentDialog.k8s.loadingClusters = false
  }
}

const loadK8sNamespaces = async clusterId => {
  if (!clusterId) return
  deploymentDialog.k8s.loadingNamespaces = true
  deploymentDialog.k8s.namespaces = []
  deploymentDialog.k8s.targets = []
  try {
    const res = normalizeResponse(await k8sApi.getNamespaces(clusterId))
    if (!isSuccessResponse(res)) throw new Error(res?.message || '获取命名空间失败')
    deploymentDialog.k8s.namespaces = pickList(res.data, ['namespaces']).map(normalizeK8sNamespace).filter(item => item.name)
  } catch (error) {
    console.error('获取命名空间失败:', error)
    deploymentDialog.k8s.namespaces = []
    ElMessage.warning(error.message || '获取命名空间失败')
  } finally {
    deploymentDialog.k8s.loadingNamespaces = false
  }
}

const loadK8sTargets = async (clusterId, namespaceName, keyword = '') => {
  if (!clusterId || !namespaceName) return
  deploymentDialog.k8s.loadingTargets = true
  deploymentDialog.k8s.targets = []
  try {
    deploymentDialog.k8s.targets = await fetchK8sWorkloadTargets(clusterId, namespaceName, keyword)
  } catch (error) {
    console.error('获取工作负载失败:', error)
    deploymentDialog.k8s.targets = []
    ElMessage.warning(error.message || '获取工作负载失败')
  } finally {
    deploymentDialog.k8s.loadingTargets = false
  }
}

const fetchK8sWorkloadTargets = async (clusterId, namespaceName, keyword = '') => {
  const params = {
    page: 1,
    pageSize: 20,
    type: 'all'
  }
  const trimmedKeyword = keyword.trim()
  const res = normalizeResponse(trimmedKeyword
    ? await k8sApi.searchWorkloadList(clusterId, namespaceName, { ...params, keyword: trimmedKeyword })
    : await k8sApi.getWorkloadList(clusterId, namespaceName, params)
  )
  if (!isSuccessResponse(res)) throw new Error(res?.message || '获取工作负载失败')
  return pickList(res.data, ['workloads']).map(normalizeK8sWorkload).filter(item => item.name)
}

const remoteSearchK8sTargets = query => {
  deploymentDialog.k8s.targetKeyword = query || ''
  if (k8sTargetSearchTimer) {
    clearTimeout(k8sTargetSearchTimer)
  }
  k8sTargetSearchTimer = setTimeout(() => {
    const keyword = deploymentDialog.k8s.targetKeyword.trim()
    if (keyword && keyword.length < 2) {
      deploymentDialog.k8s.targets = []
      return
    }
    loadK8sTargets(deploymentDialog.k8s.clusterId, deploymentDialog.k8s.namespaceName, keyword)
  }, 300)
}

const remoteSearchSiblingK8sTargets = query => {
  deploymentDialog.sibling.targetKeyword = query || ''
  if (siblingK8sTargetSearchTimer) {
    clearTimeout(siblingK8sTargetSearchTimer)
  }
  siblingK8sTargetSearchTimer = setTimeout(async () => {
    const keyword = deploymentDialog.sibling.targetKeyword.trim()
    if (keyword && keyword.length < 2) {
      deploymentDialog.sibling.targets = []
      return
    }
    if (!deploymentDialog.sibling.clusterId || !deploymentDialog.sibling.namespaceName) return
    deploymentDialog.sibling.loadingTargets = true
    try {
      const targets = await fetchK8sWorkloadTargets(
        deploymentDialog.sibling.clusterId,
        deploymentDialog.sibling.namespaceName,
        keyword
      )
      deploymentDialog.sibling.targets = targets.filter(item => item.name !== deploymentDialog.sibling.currentName)
    } catch (error) {
      console.error('搜索同级工作负载失败:', error)
      deploymentDialog.sibling.targets = []
      ElMessage.warning(error.message || '搜索同级工作负载失败')
    } finally {
      deploymentDialog.sibling.loadingTargets = false
    }
  }, 300)
}

const loadChildK8sNamespaces = async clusterId => {
  if (!clusterId) return
  deploymentDialog.child.loadingNamespaces = true
  deploymentDialog.child.namespaces = []
  try {
    const res = normalizeResponse(await k8sApi.getNamespaces(clusterId))
    if (!isSuccessResponse(res)) throw new Error(res?.message || '获取命名空间失败')
    deploymentDialog.child.namespaces = pickList(res.data, ['namespaces']).map(normalizeK8sNamespace).filter(item => item.name)
  } catch (error) {
    console.error('获取下级命名空间失败:', error)
    deploymentDialog.child.namespaces = []
    ElMessage.warning(error.message || '获取命名空间失败')
  } finally {
    deploymentDialog.child.loadingNamespaces = false
  }
}

const loadChildK8sTargets = async (clusterId, namespaceName, keyword = '') => {
  if (!clusterId || !namespaceName) return
  deploymentDialog.child.loadingTargets = true
  deploymentDialog.child.targets = []
  try {
    deploymentDialog.child.targets = await fetchK8sWorkloadTargets(clusterId, namespaceName, keyword)
  } catch (error) {
    console.error('获取下级工作负载失败:', error)
    deploymentDialog.child.targets = []
    ElMessage.warning(error.message || '获取工作负载失败')
  } finally {
    deploymentDialog.child.loadingTargets = false
  }
}

const remoteSearchChildK8sTargets = query => {
  deploymentDialog.child.targetKeyword = query || ''
  if (childK8sTargetSearchTimer) {
    clearTimeout(childK8sTargetSearchTimer)
  }
  childK8sTargetSearchTimer = setTimeout(() => {
    const keyword = deploymentDialog.child.targetKeyword.trim()
    if (keyword && keyword.length < 2) {
      deploymentDialog.child.targets = []
      return
    }
    loadChildK8sTargets(deploymentDialog.child.clusterId, deploymentDialog.child.namespaceName, keyword)
  }, 300)
}

const loadChildHostServices = async hostId => {
  if (!hostId) return
  deploymentDialog.child.loadingServices = true
  deploymentDialog.child.services = []
  try {
    const res = normalizeResponse(await cmdbApi.getCompleteHostMetrics(hostId))
    if (!isSuccessResponse(res)) throw new Error(res?.message || '获取主机服务失败')
    deploymentDialog.child.services = pickList(res.data, ['services'])
      .map(normalizeHostService)
      .filter(item => item.name)
  } catch (error) {
    console.error('获取下级主机服务失败:', error)
    deploymentDialog.child.services = []
    ElMessage.warning(error.message || '获取主机服务失败')
  } finally {
    deploymentDialog.child.loadingServices = false
  }
}

const remoteSearchHosts = query => {
  deploymentDialog.host.hostKeyword = query || ''
  if (hostSearchTimer) {
    clearTimeout(hostSearchTimer)
  }
  hostSearchTimer = setTimeout(async () => {
    const keyword = deploymentDialog.host.hostKeyword.trim()
    if (keyword && keyword.length < 2) {
      deploymentDialog.host.options = []
      return
    }
    if (!keyword) return
    deploymentDialog.host.loadingHosts = true
    try {
      const res = normalizeResponse(await cmdbApi.GetCmdbHostsByKeyword(keyword, { page: 1, pageSize: 20 }))
      if (!isSuccessResponse(res)) throw new Error(res?.message || '搜索主机失败')
      deploymentDialog.host.options = pickList(res.data, ['hosts', 'rows', 'records']).map(normalizeCmdbHost).filter(item => item.id)
    } catch (error) {
      console.error('搜索主机失败:', error)
      deploymentDialog.host.options = []
      ElMessage.warning(error.message || '搜索主机失败')
    } finally {
      deploymentDialog.host.loadingHosts = false
    }
  }, 300)
}

const remoteSearchSiblingHosts = query => {
  deploymentDialog.sibling.hostKeyword = query || ''
  if (siblingHostSearchTimer) {
    clearTimeout(siblingHostSearchTimer)
  }
  siblingHostSearchTimer = setTimeout(async () => {
    const keyword = deploymentDialog.sibling.hostKeyword.trim()
    if (!keyword || keyword.length < 2) {
      deploymentDialog.sibling.hosts = []
      return
    }
    deploymentDialog.sibling.loadingHosts = true
    try {
      const res = normalizeResponse(await cmdbApi.GetCmdbHostsByKeyword(keyword, { page: 1, pageSize: 20 }))
      if (!isSuccessResponse(res)) throw new Error(res?.message || '搜索同级主机失败')
      deploymentDialog.sibling.hosts = pickList(res.data, ['hosts', 'rows', 'records'])
        .map(normalizeCmdbHost)
        .filter(item => item.id && String(item.id) !== String(deploymentDialog.sibling.currentSourceId))
    } catch (error) {
      console.error('搜索同级主机失败:', error)
      deploymentDialog.sibling.hosts = []
      ElMessage.warning(error.message || '搜索同级主机失败')
    } finally {
      deploymentDialog.sibling.loadingHosts = false
    }
  }, 300)
}

const loadSiblingHostServices = async hostId => {
  if (!hostId) return
  deploymentDialog.sibling.loadingServices = true
  deploymentDialog.sibling.services = []
  try {
    const res = normalizeResponse(await cmdbApi.getCompleteHostMetrics(hostId))
    if (!isSuccessResponse(res)) throw new Error(res?.message || '获取同级服务失败')
    deploymentDialog.sibling.services = pickList(res.data, ['services'])
      .map(normalizeHostService)
      .filter(item => item.name && item.name !== deploymentDialog.sibling.currentName)
  } catch (error) {
    console.error('获取同级服务失败:', error)
    deploymentDialog.sibling.services = []
    ElMessage.warning(error.message || '获取同级服务失败')
  } finally {
    deploymentDialog.sibling.loadingServices = false
  }
}

const loadSiblingK8sClusters = async currentClusterId => {
  deploymentDialog.sibling.loadingClusters = true
  deploymentDialog.sibling.clusters = []
  try {
    const res = normalizeResponse(await k8sApi.getClusterList({ page: 1, size: 200 }))
    if (!isSuccessResponse(res)) throw new Error(res?.message || '获取 K8S 集群失败')
    deploymentDialog.sibling.clusters = pickList(res.data, ['clusters'])
      .map(normalizeK8sCluster)
      .filter(item => item.id !== undefined && item.id !== null && item.name && String(item.id) !== String(currentClusterId))
  } catch (error) {
    console.error('获取同级 K8S 集群失败:', error)
    deploymentDialog.sibling.clusters = []
    ElMessage.warning(error.message || '获取 K8S 集群失败')
  } finally {
    deploymentDialog.sibling.loadingClusters = false
  }
}

const loadSiblingK8sNamespaces = async (clusterId, currentNamespaceName) => {
  if (!clusterId) return
  deploymentDialog.sibling.loadingNamespaces = true
  deploymentDialog.sibling.namespaces = []
  try {
    const res = normalizeResponse(await k8sApi.getNamespaces(clusterId))
    if (!isSuccessResponse(res)) throw new Error(res?.message || '获取命名空间失败')
    deploymentDialog.sibling.namespaces = pickList(res.data, ['namespaces'])
      .map(normalizeK8sNamespace)
      .filter(item => item.name && item.name !== currentNamespaceName)
  } catch (error) {
    console.error('获取同级命名空间失败:', error)
    deploymentDialog.sibling.namespaces = []
    ElMessage.warning(error.message || '获取命名空间失败')
  } finally {
    deploymentDialog.sibling.loadingNamespaces = false
  }
}

const handleHostSelectionChange = async hostKey => {
  deploymentDialog.host.serviceName = ''
  deploymentDialog.host.serviceNames = []
  deploymentDialog.host.services = []
  const selectedHost = deploymentDialog.host.options.find(item => item.key === hostKey)
  if (!selectedHost?.id) return
  await loadHostServices(selectedHost.id)
}

const loadHostServices = async hostId => {
  deploymentDialog.host.loadingServices = true
  try {
    const res = normalizeResponse(await cmdbApi.getCompleteHostMetrics(hostId))
    if (!isSuccessResponse(res)) throw new Error(res?.message || '获取主机服务失败')
    deploymentDialog.host.services = pickList(res.data, ['services'])
      .map(normalizeHostService)
      .filter(item => item.name)
  } catch (error) {
    console.error('获取主机服务失败:', error)
    deploymentDialog.host.services = []
    ElMessage.warning(error.message || '获取主机服务失败，可先保存为监控全部服务')
  } finally {
    deploymentDialog.host.loadingServices = false
  }
}

const remoteSearchDatabaseAssets = query => {
  deploymentDialog.dependency.dbAssetKeyword = query || ''
  if (databaseAssetSearchTimer) {
    clearTimeout(databaseAssetSearchTimer)
  }
  databaseAssetSearchTimer = setTimeout(async () => {
    const keyword = deploymentDialog.dependency.dbAssetKeyword.trim()
    if (keyword && keyword.length < 2) {
      deploymentDialog.dependency.dbAssets = []
      return
    }
    if (!keyword) return
    deploymentDialog.dependency.loadingDbAssets = true
    try {
      const res = normalizeResponse(await dbAPI.getCrudAssets({
        keyword,
        page: 1,
        pageSize: 20,
        type: deploymentDialog.dependency.dbType || undefined
      }))
      if (!isSuccessResponse(res)) throw new Error(res?.message || '搜索数据库资产失败')
      deploymentDialog.dependency.dbAssets = pickList(res.data, ['assets', 'records', 'rows']).map(normalizeDatabaseAsset).filter(item => item.id)
    } catch (error) {
      console.error('搜索数据库资产失败:', error)
      deploymentDialog.dependency.dbAssets = []
      ElMessage.warning(error.message || '搜索数据库资产失败')
    } finally {
      deploymentDialog.dependency.loadingDbAssets = false
    }
  }, 300)
}

const refreshDatabaseAssetSearch = () => {
  deploymentDialog.dependency.dbAssetKey = ''
  const keyword = deploymentDialog.dependency.dbAssetKeyword
  if (keyword?.trim()) {
    remoteSearchDatabaseAssets(keyword)
  }
}

const handleK8sClusterChange = async clusterId => {
  deploymentDialog.k8s.namespaceName = ''
  deploymentDialog.k8s.targetKey = ''
  deploymentDialog.k8s.targetKeys = []
  deploymentDialog.k8s.targetKeyword = ''
  await loadK8sNamespaces(clusterId)
}

const handleK8sNamespaceChange = async namespaceName => {
  deploymentDialog.k8s.targetKey = ''
  deploymentDialog.k8s.targetKeys = []
  deploymentDialog.k8s.targetKeyword = ''
  await loadK8sTargets(deploymentDialog.k8s.clusterId, namespaceName)
}

const openChildWizard = async node => {
  const childResourceType = getNextChildResourceType(node)
  if (!childResourceType) {
    ElMessage.warning('当前节点暂不支持添加下一级')
    return
  }

  if (childResourceType === 'k8s_namespace') {
    const k8sContext = resolveK8sContext(node)
    if (!k8sContext.clusterId) {
      ElMessage.warning('当前 K8S 集群节点缺少集群 ID，无法加载命名空间')
      return
    }
    openDeploymentWizard('child', node)
    deploymentDialog.child.resourceType = childResourceType
    deploymentDialog.child.clusterId = k8sContext.clusterId
    deploymentDialog.child.clusterName = k8sContext.clusterName || node?.displayName || node?.name || ''
    await loadChildK8sNamespaces(k8sContext.clusterId)
    return
  }

  if (childResourceType === 'k8s_workload') {
    const parentNode = topologyData.value.nodes.find(item => String(item.id) === String(findParentNodeId(node?.id)))
    const k8sContext = resolveK8sContext(node, parentNode)
    if (!k8sContext.clusterId || !k8sContext.namespaceName) {
      ElMessage.warning('当前命名空间节点缺少集群或命名空间信息，无法加载工作负载')
      return
    }
    openDeploymentWizard('child', node)
    deploymentDialog.child.resourceType = childResourceType
    deploymentDialog.child.clusterId = k8sContext.clusterId
    deploymentDialog.child.clusterName = k8sContext.clusterName || ''
    deploymentDialog.child.namespaceName = k8sContext.namespaceName
    await loadChildK8sTargets(k8sContext.clusterId, k8sContext.namespaceName)
    return
  }

  if (childResourceType === 'business_service') {
    const hostContext = resolveHostContext(node)
    if (!hostContext.hostId) {
      ElMessage.warning('当前主机节点缺少主机 ID，无法加载服务')
      return
    }
    const properties = normalizeProperties(node?.properties || node?.Properties)
    openDeploymentWizard('child', node)
    deploymentDialog.child.resourceType = childResourceType
    deploymentDialog.child.hostId = hostContext.hostId
    deploymentDialog.child.hostName = properties.hostName || node?.displayName || node?.name || ''
    deploymentDialog.child.hostIp = properties.ip || properties.privateIp || properties.sshIp || properties.publicIp || node?.subtitle || ''
    await loadChildHostServices(hostContext.hostId)
  }
}

const openSiblingWizard = async node => {
  const parentId = findParentNodeId(node?.id)
  if (!parentId) {
    ElMessage.warning('当前节点没有上级，无法添加同级')
    return
  }
  const parentNode = topologyData.value.nodes.find(item => String(item.id) === String(parentId))
  openDeploymentWizard('sibling', parentNode)
  deploymentDialog.sibling.resourceType = node?.resourceType || 'business_service'
  deploymentDialog.sibling.name = ''

  if (isHostResourceType(node?.resourceType)) {
    const hostContext = resolveHostContext(node)
    deploymentDialog.sibling.currentSourceId = hostContext.hostId || ''
    return
  }

  if (isServiceResourceType(node?.resourceType)) {
    const hostContext = resolveHostContext(node)
    if (!hostContext.hostId) {
      deploymentDialog.visible = false
      ElMessage.warning('当前服务节点缺少主机 ID，无法加载同级服务')
      return
    }
    const properties = normalizeProperties(node?.properties || node?.Properties)
    deploymentDialog.sibling.currentName = properties.serviceName || node?.name || node?.displayName || ''
    deploymentDialog.sibling.hostId = hostContext.hostId
    deploymentDialog.sibling.hostName = properties.hostName || parentNode?.displayName || parentNode?.name || ''
    deploymentDialog.sibling.hostIp = properties.ip || properties.privateIp || properties.sshIp || properties.publicIp || parentNode?.subtitle || ''
    await loadSiblingHostServices(hostContext.hostId)
    return
  }

  if (node?.resourceType === 'k8s_cluster') {
    const k8sContext = resolveK8sContext(node, parentNode)
    deploymentDialog.sibling.currentSourceId = k8sContext.clusterId || ''
    await loadSiblingK8sClusters(k8sContext.clusterId)
    return
  }

  if (node?.resourceType === 'k8s_namespace') {
    const k8sContext = resolveK8sContext(node, parentNode)
    if (!k8sContext.clusterId) {
      deploymentDialog.visible = false
      ElMessage.warning('当前命名空间节点缺少集群 ID，无法加载同级命名空间')
      return
    }
    deploymentDialog.sibling.clusterId = k8sContext.clusterId
    deploymentDialog.sibling.clusterName = k8sContext.clusterName || parentNode?.displayName || parentNode?.name || ''
    deploymentDialog.sibling.currentName = k8sContext.namespaceName || node?.displayName || node?.name || ''
    await loadSiblingK8sNamespaces(k8sContext.clusterId, deploymentDialog.sibling.currentName)
    return
  }

  if (['k8s_workload', 'pod'].includes(node?.resourceType)) {
    const k8sContext = resolveK8sContext(node, parentNode)
    if (!k8sContext.clusterId || !k8sContext.namespaceName) {
      deploymentDialog.visible = false
      ElMessage.warning('当前节点缺少 K8S 集群或命名空间信息，无法搜索同级工作负载')
      return
    }
    const properties = normalizeProperties(node?.properties || node?.Properties)
    deploymentDialog.sibling.useK8sSearch = true
    deploymentDialog.sibling.clusterId = k8sContext.clusterId
    deploymentDialog.sibling.clusterName = k8sContext.clusterName || ''
    deploymentDialog.sibling.namespaceName = k8sContext.namespaceName
    deploymentDialog.sibling.currentName = properties.workloadName || properties.podName || node?.name || node?.displayName || ''
    deploymentDialog.sibling.loadingTargets = true
    try {
      const targets = await fetchK8sWorkloadTargets(k8sContext.clusterId, k8sContext.namespaceName)
      deploymentDialog.sibling.targets = targets.filter(item => item.name !== deploymentDialog.sibling.currentName)
    } catch (error) {
      console.error('加载同级工作负载失败:', error)
      deploymentDialog.sibling.targets = []
    } finally {
      deploymentDialog.sibling.loadingTargets = false
    }
  }
}

const appendDeploymentChange = async ({ action, nodes, edges }) => {
  const change = {
    type: 'createResource',
    action,
    nodes,
    edges,
    node: nodes[nodes.length - 1],
    edge: edges[edges.length - 1] || null,
    parentId: edges[0]?.source || rootNodeId.value,
    resourceType: nodes[nodes.length - 1]?.resourceType,
    name: nodes[nodes.length - 1]?.name
  }
  applyDraftChange(change)
  pushDraftChange(change)
  selectedNode.value = change.node
  deploymentDialog.visible = false
  await renderGraph()
}

const confirmDeploymentWizard = async () => {
  if (['nginx', 'gateway'].includes(deploymentDialog.type)) {
    const parentNode = deploymentDialog.parentNode || topologyData.value.nodes.find(node => node.id === rootNodeId.value)
    const entry = deploymentDialog.entry
    const name = entry.name.trim()
    if (!parentNode?.id) {
      ElMessage.warning('请选择主链路挂载目标')
      return
    }
    if (!name) {
      ElMessage.warning(`请输入 ${resourceTypeText(entry.resourceType)} 名称`)
      return
    }

    const isLogProject = entry.logResourceType === 'log_project'
    const logResource = isLogProject
      ? deploymentDialog.log.projects.find(item => String(item.id) === String(entry.projectSourceId))
      : deploymentDialog.log.stores.find(item => String(item.id) === String(entry.storeId))
    if (!logResource) {
      ElMessage.warning(isLogProject ? '请选择日志项目' : '请选择日志库')
      return
    }

    const project = isLogProject
      ? logResource
      : deploymentDialog.log.projects.find(item => String(item.id) === String(entry.projectId))
    const sourceType = entry.logResourceType
    const entryNode = createDraftNode({
      resourceType: entry.resourceType,
      name,
      position: getDefaultPosition(parentNode, 1),
      resource: {
        displayName: name,
        sourceType,
        sourceId: logResource.id,
        sourceKey: `${sourceType}:${logResource.id}`,
        properties: {
          projectId: project?.id,
          projectName: project?.name || '',
          projectCode: project?.code || '',
          storeId: isLogProject ? undefined : logResource.id,
          storeName: isLogProject ? undefined : logResource.name,
          storeCode: isLogProject ? undefined : logResource.code,
          storeType: isLogProject ? undefined : logResource.storeType,
          logResourceType: sourceType,
          status: logResource.status,
          description: logResource.description
        }
      }
    })
    entryNode.observability = { ...entryNode.observability, logCount: 1 }
    await appendDeploymentChange({
      action: entry.resourceType === 'nginx' ? 'add_nginx' : 'add_gateway',
      nodes: [entryNode],
      edges: [createDraftEdge(parentNode.id, entryNode.id, {
        isDraft: true,
        relationType: 'contains'
      })]
    })
    return
  }

  if (deploymentDialog.type === 'log') {
    const targetNode = deploymentDialog.parentNode
    if (!targetNode?.id) {
      ElMessage.warning('请选择日志挂载目标')
      return
    }
    const log = deploymentDialog.log
    let selectedResources = []
    if (log.resourceType === 'log_project') {
      const selectedIds = new Set((log.projectIds || []).map(String))
      selectedResources = log.projects
        .filter(project => selectedIds.has(String(project.id)))
        .map(project => ({
          resourceType: 'log_project',
          name: project.name,
          displayName: project.name,
          sourceType: 'log_project',
          sourceId: project.id,
          sourceKey: `log_project:${project.id}`,
          properties: {
            projectId: project.id,
            projectName: project.name,
            projectCode: project.code,
            status: project.status,
            description: project.description
          }
        }))
    } else {
      const selectedIds = new Set((log.storeIds || []).map(String))
      const project = log.projects.find(item => String(item.id) === String(log.projectId))
      selectedResources = log.stores
        .filter(store => selectedIds.has(String(store.id)))
        .map(store => ({
          resourceType: 'log_store',
          name: store.name,
          displayName: store.name,
          sourceType: 'log_store',
          sourceId: store.id,
          sourceKey: `log_store:${store.id}`,
          properties: {
            projectId: store.projectId || project?.id,
            projectName: project?.name || '',
            storeId: store.id,
            storeName: store.name,
            storeCode: store.code,
            storeType: store.storeType,
            status: store.status,
            description: store.description
          }
        }))
    }

    if (!selectedResources.length) {
      ElMessage.warning(log.resourceType === 'log_project' ? '请选择日志项目' : '请选择日志库')
      return
    }

    const reservedNodes = []
    const logNodes = selectedResources.map((resource, index) => {
      const logNode = createDraftNode({
        resourceType: resource.resourceType,
        name: resource.name,
        position: getDefaultPosition(targetNode, index + 1, reservedNodes),
        resource
      })
      reservedNodes.push(logNode)
      return logNode
    })
    await appendDeploymentChange({
      action: 'log_binding',
      nodes: logNodes,
      edges: logNodes.map(logNode => createDraftEdge(targetNode.id, logNode.id, {
        isDraft: true,
        relationType: 'emits_log_to'
      }))
    })
    return
  }

  if (deploymentDialog.type === 'k8s') {
    const selection = buildK8sSelection()
    if (!selection) {
      ElMessage.warning('请选择 K8S 集群、命名空间和工作负载')
      return
    }
    const { cluster, namespace, targets } = selection
    const parentNode = deploymentDialog.parentNode || topologyData.value.nodes.find(node => node.id === rootNodeId.value)
    const parentId = parentNode?.id || rootNodeId.value
    const clusterName = cluster.name
    const namespaceName = namespace.name
    const clusterNode = createDraftNode({
      resourceType: 'k8s_cluster',
      name: clusterName,
      position: getDefaultPosition(parentNode, 1),
      resource: {
        displayName: cluster.displayName || clusterName,
        sourceType: 'k8s_cluster',
        sourceId: cluster.id,
        sourceKey: `k8s:cluster:${cluster.id}`,
        healthStatus: cluster.status === 2 ? 'healthy' : undefined,
        properties: {
          clusterId: cluster.id,
          clusterName
        }
      }
    })
    const namespaceNode = createDraftNode({
      resourceType: 'k8s_namespace',
      name: `${clusterName}/${namespaceName}`,
      position: getDefaultPosition(clusterNode, 1, [clusterNode]),
      resource: {
        displayName: namespaceName,
        sourceType: 'k8s_namespace',
        sourceKey: `k8s:cluster:${cluster.id}:namespace:${namespaceName}`,
        properties: {
          clusterId: cluster.id,
          clusterName,
          namespace: namespaceName
        }
      }
    })
    const reservedNodes = [clusterNode, namespaceNode]
    const workloadNodes = targets.map((target, index) => {
      const workloadNode = createDraftNode({
        resourceType: 'k8s_workload',
        name: target.name,
        position: getDefaultPosition(namespaceNode, index + 1, reservedNodes),
        resource: {
          displayName: target.name,
          sourceType: 'k8s_workload',
          sourceKey: `k8s:cluster:${cluster.id}:namespace:${namespaceName}:workload:${target.workloadType}:${target.name}`,
          healthScore: target.healthScore,
          properties: {
            clusterId: cluster.id,
            clusterName,
            namespace: namespaceName,
            workloadType: target.workloadType,
            workloadName: target.name,
            status: target.status
          }
        }
      })
      reservedNodes.push(workloadNode)
      return workloadNode
    })
    await appendDeploymentChange({
      action: 'deploy_k8s',
      nodes: [clusterNode, namespaceNode, ...workloadNodes],
      edges: [
        createDraftEdge(parentId, clusterNode.id, { isDraft: true, relationType: 'deployed_on' }),
        createDraftEdge(clusterNode.id, namespaceNode.id, { isDraft: true, relationType: 'contains' }),
        ...workloadNodes.map(workloadNode => createDraftEdge(namespaceNode.id, workloadNode.id, { isDraft: true, relationType: 'contains' }))
      ]
    })
    return
  }

  if (deploymentDialog.type === 'host') {
    const serviceNames = deploymentDialog.host.serviceNames || []
    const selectedHost = deploymentDialog.host.options.find(item => item.key === deploymentDialog.host.hostKey)
    if (!selectedHost) {
      ElMessage.warning('请选择主机')
      return
    }
    const parentNode = deploymentDialog.parentNode || topologyData.value.nodes.find(node => node.id === rootNodeId.value)
    const parentId = parentNode?.id || rootNodeId.value
    const hostNode = createDraftNode({
      resourceType: 'host',
      name: selectedHost.name,
      position: getDefaultPosition(parentNode, 1),
      resource: {
        displayName: selectedHost.displayName,
        sourceType: 'cmdb_host',
        sourceId: selectedHost.id,
        sourceKey: `cmdb_host:${selectedHost.id}`,
        properties: {
          hostId: selectedHost.id,
          hostName: selectedHost.hostName,
          privateIp: selectedHost.privateIp,
          publicIp: selectedHost.publicIp,
          sshIp: selectedHost.sshIp,
          hostType: selectedHost.hostType,
          vendor: selectedHost.vendor,
          os: selectedHost.os,
          status: selectedHost.status,
          instanceId: selectedHost.instanceId
        }
      }
    })
    const selectedServices = serviceNames.length
      ? serviceNames.map(name => deploymentDialog.host.services.find(item => item.name === name) || { name }).filter(item => item.name)
      : [null]
    const reservedNodes = [hostNode]
    const serviceNodes = selectedServices.map((service, index) => {
      const monitorAllServices = !service
      const serviceName = service?.name || ''
      const serviceDisplayName = monitorAllServices ? `${selectedHost.displayName} 全部服务` : serviceName.trim()
      const serviceNode = createDraftNode({
        resourceType: 'business_service',
        name: serviceDisplayName,
        position: getDefaultPosition(hostNode, index + 1, reservedNodes),
        resource: {
          displayName: serviceDisplayName,
          sourceType: 'host_service',
          sourceKey: monitorAllServices ? `host_service:${selectedHost.id}:all` : `host_service:${selectedHost.id}:${serviceName.trim()}`,
          properties: {
            deployMode: 'host',
            hostId: selectedHost.id,
            hostName: selectedHost.hostName,
            ip: selectedHost.ip,
            serviceName: monitorAllServices ? '' : serviceName.trim(),
            monitorAllServices,
            serviceCpu: service?.cpu,
            serviceMemory: service?.memory
          }
        }
      })
      reservedNodes.push(serviceNode)
      return serviceNode
    })
    await appendDeploymentChange({
      action: 'deploy_host',
      nodes: [hostNode, ...serviceNodes],
      edges: [
        createDraftEdge(parentId, hostNode.id, { isDraft: true, relationType: 'deployed_on' }),
        ...serviceNodes.map(serviceNode => createDraftEdge(hostNode.id, serviceNode.id, { isDraft: true, relationType: 'runs_on' }))
      ]
    })
    return
  }

  if (deploymentDialog.type === 'child') {
    const parentNode = deploymentDialog.parentNode || topologyData.value.nodes.find(node => node.id === rootNodeId.value)
    const parentId = parentNode?.id || rootNodeId.value
    const child = deploymentDialog.child

    if (child.resourceType === 'k8s_namespace') {
      const namespace = child.namespaces.find(item => item.name === child.namespaceName)
      if (!namespace) {
        ElMessage.warning('请选择命名空间')
        return
      }
      const namespaceNode = createDraftNode({
        resourceType: 'k8s_namespace',
        name: `${child.clusterName || parentNode?.name || ''}/${namespace.name}`,
        position: getDefaultPosition(parentNode, 1),
        resource: {
          displayName: namespace.name,
          sourceType: 'k8s_namespace',
          sourceKey: `k8s:cluster:${child.clusterId}:namespace:${namespace.name}`,
          properties: {
            clusterId: child.clusterId,
            clusterName: child.clusterName,
            namespace: namespace.name,
            status: namespace.status
          }
        }
      })
      await appendDeploymentChange({
        action: 'child',
        nodes: [namespaceNode],
        edges: [
          createDraftEdge(parentId, namespaceNode.id, { isDraft: true, relationType: 'contains' })
        ]
      })
      return
    }

    if (child.resourceType === 'k8s_workload') {
      const selectedKeys = new Set((child.targetKeys || []).map(String))
      const targets = child.targets.filter(item => selectedKeys.has(String(item.key)))
      if (!targets.length) {
        ElMessage.warning('请选择工作负载')
        return
      }
      const reservedNodes = []
      const workloadNodes = targets.map((target, index) => {
        const workloadNode = createDraftNode({
          resourceType: 'k8s_workload',
          name: target.name,
          position: getDefaultPosition(parentNode, index + 1, reservedNodes),
          resource: {
            displayName: target.name,
            sourceType: 'k8s_workload',
            sourceKey: `k8s:cluster:${child.clusterId}:namespace:${child.namespaceName}:workload:${target.workloadType}:${target.name}`,
            healthScore: target.healthScore,
            properties: {
              clusterId: child.clusterId,
              clusterName: child.clusterName,
              namespace: child.namespaceName,
              workloadType: target.workloadType,
              workloadName: target.name,
              status: target.status
            }
          }
        })
        reservedNodes.push(workloadNode)
        return workloadNode
      })
      await appendDeploymentChange({
        action: 'child',
        nodes: workloadNodes,
        edges: workloadNodes.map(workloadNode => createDraftEdge(parentId, workloadNode.id, { isDraft: true, relationType: 'contains' }))
      })
      return
    }

    if (child.resourceType === 'business_service') {
      const selectedNames = child.serviceNames || []
      const services = selectedNames.map(name => child.services.find(item => item.name === name) || { name }).filter(item => item.name)
      if (!services.length) {
        ElMessage.warning('请选择服务')
        return
      }
      const reservedNodes = []
      const serviceNodes = services.map((service, index) => {
        const serviceName = service.name.trim()
        const serviceNode = createDraftNode({
          resourceType: 'business_service',
          name: serviceName,
          position: getDefaultPosition(parentNode, index + 1, reservedNodes),
          resource: {
            displayName: serviceName,
            sourceType: 'host_service',
            sourceKey: `host_service:${child.hostId}:${serviceName}`,
            properties: {
              deployMode: 'host',
              hostId: child.hostId,
              hostName: child.hostName,
              ip: child.hostIp,
              serviceName,
              monitorAllServices: false,
              serviceCpu: service.cpu,
              serviceMemory: service.memory
            }
          }
        })
        reservedNodes.push(serviceNode)
        return serviceNode
      })
      await appendDeploymentChange({
        action: 'child',
        nodes: serviceNodes,
        edges: serviceNodes.map(serviceNode => createDraftEdge(parentId, serviceNode.id, { isDraft: true, relationType: 'runs_on' }))
      })
      return
    }
  }

  if (deploymentDialog.type === 'sibling') {
    const parentNode = deploymentDialog.parentNode || topologyData.value.nodes.find(node => node.id === rootNodeId.value)
    const parentId = parentNode?.id || rootNodeId.value
    const sibling = deploymentDialog.sibling
    const { resourceType, name } = sibling
    const reservedNodes = []
    let siblingNodes = []
    let relationType
    const createPositionedNode = (index, options) => {
      const siblingNode = createDraftNode({
        ...options,
        position: getDefaultPosition(parentNode, index + 1, reservedNodes)
      })
      reservedNodes.push(siblingNode)
      return siblingNode
    }

    if (sibling.useK8sSearch) {
      const selectedKeys = new Set((sibling.targetKeys || []).map(String))
      const targets = sibling.targets.filter(item => selectedKeys.has(String(item.key)))
      if (!targets.length) {
        ElMessage.warning('请选择同级工作负载')
        return
      }
      relationType = 'contains'
      siblingNodes = targets.map((target, index) => createPositionedNode(index, {
        resourceType: 'k8s_workload',
        name: target.name,
        resource: {
          displayName: target.name,
          sourceType: 'k8s_workload',
          sourceKey: `k8s:cluster:${sibling.clusterId}:namespace:${sibling.namespaceName}:workload:${target.workloadType}:${target.name}`,
          healthScore: target.healthScore,
          properties: {
            clusterId: sibling.clusterId,
            clusterName: sibling.clusterName,
            namespace: sibling.namespaceName,
            workloadType: target.workloadType,
            workloadName: target.name,
            status: target.status
          }
        }
      }))
    } else if (isHostResourceType(resourceType)) {
      const selectedKeys = new Set((sibling.hostKeys || []).map(String))
      const hosts = sibling.hosts.filter(item => selectedKeys.has(String(item.key)))
      if (!hosts.length) {
        ElMessage.warning('请选择同级主机')
        return
      }
      relationType = 'deployed_on'
      siblingNodes = hosts.map((host, index) => createPositionedNode(index, {
        resourceType,
        name: host.name,
        resource: {
          displayName: host.displayName,
          sourceType: 'cmdb_host',
          sourceId: host.id,
          sourceKey: `cmdb_host:${host.id}`,
          properties: {
            hostId: host.id,
            hostName: host.hostName,
            privateIp: host.privateIp,
            publicIp: host.publicIp,
            sshIp: host.sshIp,
            hostType: host.hostType,
            vendor: host.vendor,
            os: host.os,
            status: host.status,
            instanceId: host.instanceId
          }
        }
      }))
    } else if (isServiceResourceType(resourceType)) {
      const services = (sibling.serviceNames || [])
        .map(serviceName => sibling.services.find(item => item.name === serviceName) || { name: serviceName })
        .filter(item => item.name)
      if (!services.length) {
        ElMessage.warning('请选择同级服务')
        return
      }
      relationType = 'runs_on'
      siblingNodes = services.map((service, index) => createPositionedNode(index, {
        resourceType: 'business_service',
        name: service.name,
        resource: {
          displayName: service.name,
          sourceType: 'host_service',
          sourceKey: `host_service:${sibling.hostId}:${service.name}`,
          properties: {
            deployMode: 'host',
            hostId: sibling.hostId,
            hostName: sibling.hostName,
            ip: sibling.hostIp,
            serviceName: service.name,
            monitorAllServices: false,
            serviceCpu: service.cpu,
            serviceMemory: service.memory
          }
        }
      }))
    } else if (resourceType === 'k8s_cluster') {
      const selectedIds = new Set((sibling.clusterIds || []).map(String))
      const clusters = sibling.clusters.filter(item => selectedIds.has(String(item.id)))
      if (!clusters.length) {
        ElMessage.warning('请选择同级 K8S 集群')
        return
      }
      relationType = 'deployed_on'
      siblingNodes = clusters.map((cluster, index) => createPositionedNode(index, {
        resourceType: 'k8s_cluster',
        name: cluster.name,
        resource: {
          displayName: cluster.displayName || cluster.name,
          sourceType: 'k8s_cluster',
          sourceId: cluster.id,
          sourceKey: `k8s:cluster:${cluster.id}`,
          properties: {
            clusterId: cluster.id,
            clusterName: cluster.name,
            status: cluster.status
          }
        }
      }))
    } else if (resourceType === 'k8s_namespace') {
      const selectedNames = new Set(sibling.namespaceNames || [])
      const namespaces = sibling.namespaces.filter(item => selectedNames.has(item.name))
      if (!namespaces.length) {
        ElMessage.warning('请选择同级命名空间')
        return
      }
      relationType = 'contains'
      siblingNodes = namespaces.map((namespace, index) => createPositionedNode(index, {
        resourceType: 'k8s_namespace',
        name: `${sibling.clusterName}/${namespace.name}`,
        resource: {
          displayName: namespace.name,
          sourceType: 'k8s_namespace',
          sourceKey: `k8s:cluster:${sibling.clusterId}:namespace:${namespace.name}`,
          properties: {
            clusterId: sibling.clusterId,
            clusterName: sibling.clusterName,
            namespace: namespace.name,
            status: namespace.status
          }
        }
      }))
    } else {
      if (!name.trim()) {
        ElMessage.warning('请输入同级节点名称')
        return
      }
      siblingNodes = [createPositionedNode(0, {
        resourceType,
        name: name.trim(),
        resource: null
      })]
    }

    await appendDeploymentChange({
      action: 'sibling',
      nodes: siblingNodes,
      edges: siblingNodes.map(siblingNode => createDraftEdge(parentId, siblingNode.id, {
        isDraft: true,
        relationType
      }))
    })
    return
  }

  const dependency = deploymentDialog.dependency
  const targetId = dependency.targetId || rootNodeId.value
  const targetNode = topologyData.value.nodes.find(node => String(node.id) === String(targetId))
  const selectedDbAsset = dependency.dbAssets.find(item => item.key === dependency.dbAssetKey)
  const selected = dependency.mode === 'existing' ? dependency.selectedResource : null
  const name = dependency.resourceType === 'database'
    ? selectedDbAsset?.displayName
    : (selected ? selected.displayName || selected.name || selected.label : dependency.name.trim())
  if (!name) {
    ElMessage.warning(dependency.resourceType === 'database'
      ? '请选择数据库资产'
      : (dependency.mode === 'existing' ? '请选择已有依赖资源' : '请输入依赖组件名称')
    )
    return
  }
  const dependencyNode = createDraftNode({
    resourceType: dependency.resourceType,
    name,
    position: getDefaultPosition(targetNode, 1),
    resource: dependency.resourceType === 'database' ? {
      displayName: selectedDbAsset.displayName,
      sourceType: 'db_asset',
      sourceId: selectedDbAsset.id,
      sourceKey: `db_asset:${selectedDbAsset.id}`,
      properties: {
        assetId: selectedDbAsset.id,
        instanceName: selectedDbAsset.instanceName || selectedDbAsset.displayName,
        type: selectedDbAsset.type,
        typeName: selectedDbAsset.typeName,
        hostIp: selectedDbAsset.hostIp,
        port: selectedDbAsset.port,
        endpoint: selectedDbAsset.endpoint,
        environment: selectedDbAsset.environment,
        status: selectedDbAsset.status,
        authStatus: selectedDbAsset.authStatus,
        authStatusName: selectedDbAsset.authStatusName,
        businessSystem: selectedDbAsset.businessSystem,
        owner: selectedDbAsset.owner
      }
    } : (selected ? { ...selected, resourceType: selected.resourceType || selected.type || dependency.resourceType } : null)
  })
  await appendDeploymentChange({
    action: 'dependency',
    nodes: [dependencyNode],
    edges: [
      createDraftEdge(targetId, dependencyNode.id, {
        isDraft: true,
        relationType: dependency.resourceType === 'database' ? 'depends_on' : 'uses',
        dependency: true
      })
    ]
  })
}

const searchDependencyResources = async () => {
  const dependency = deploymentDialog.dependency
  if (!dependency.keyword.trim()) {
    dependency.options = []
    return
  }

  dependency.searching = true
  try {
    const res = normalizeResponse(await searchResources({
      name: dependency.keyword.trim(),
      keyword: dependency.keyword.trim(),
      resourceType: dependency.resourceType,
      type: dependency.resourceType,
      page: 1,
      pageSize: 20
    }))
    const data = res?.data || {}
    dependency.options = Array.isArray(data.list) ? data.list : (Array.isArray(data) ? data : [])
  } catch (error) {
    console.error('搜索依赖资源失败:', error)
    dependency.options = []
  } finally {
    dependency.searching = false
  }
}

const deleteDraftEdge = async edge => {
  if (!edge) return
  const change = { type: 'deleteRelation', edge }
  applyDraftChange(change)
  pushDraftChange(change)
  await renderGraph()
}

const collectDraftNodeSubtree = rootNode => {
  const rootId = String(rootNode?.id || '')
  const nodeById = new Map(topologyData.value.nodes.map(node => [String(node.id), node]))
  const childrenByParent = new Map()
  const addChild = (parentId, childId) => {
    const parentKey = String(parentId || '')
    const childKey = String(childId || '')
    if (!parentKey || !childKey || parentKey === childKey || !nodeById.has(childKey)) return
    if (!childrenByParent.has(parentKey)) childrenByParent.set(parentKey, new Set())
    childrenByParent.get(parentKey).add(childKey)
  }

  topologyData.value.nodes.forEach(node => {
    if (node.parentId !== null && node.parentId !== undefined && node.parentId !== '') addChild(node.parentId, node.id)
  })
  topologyData.value.edges.forEach(edge => {
    const child = nodeById.get(String(edge.target))
    const hasExplicitParent = child?.parentId !== null && child?.parentId !== undefined && child?.parentId !== ''
    if (!hasExplicitParent) addChild(edge.source, edge.target)
  })

  const subtreeIds = new Set()
  const queue = rootId ? [rootId] : []
  while (queue.length) {
    const currentId = queue.shift()
    if (subtreeIds.has(currentId) || !nodeById.has(currentId)) continue
    subtreeIds.add(currentId)
    ;(childrenByParent.get(currentId) || []).forEach(childId => queue.push(childId))
  }
  return {
    nodes: [...subtreeIds].map(id => nodeById.get(id)).filter(Boolean),
    edges: topologyData.value.edges.filter(edge => subtreeIds.has(String(edge.source)) || subtreeIds.has(String(edge.target)))
  }
}

const deleteDraftNode = async node => {
  if (!node) return
  if (node.resourceType === 'business_system') {
    ElMessage.warning('业务系统根节点不能删除')
    return
  }
  const subtree = collectDraftNodeSubtree(node)
  const descendantCount = Math.max(0, subtree.nodes.length - 1)
  const nodeName = node.displayName || node.name || node.label || '当前节点'
  const message = descendantCount > 0
    ? `删除「${nodeName}」将同时删除其下方 ${descendantCount} 个节点及相关连接，是否继续？`
    : `确定从当前业务拓扑中删除「${nodeName}」吗？`
  try {
    await ElMessageBox.confirm(message, '级联删除节点', {
      type: 'warning',
      confirmButtonText: descendantCount > 0 ? `删除 ${subtree.nodes.length} 个节点` : '删除节点',
      cancelButtonText: '取消',
      distinguishCancelAndClose: true
    })
  } catch (error) {
    return
  }
  const change = { type: 'deleteNode', node, nodes: subtree.nodes, edges: subtree.edges }
  applyDraftChange(change)
  pushDraftChange(change)
  selectedNode.value = topologyData.value.nodes[0] || null
  await renderGraph()
}

const recordMoveNode = (nodeId, x, y) => {
  const originalNode = originalTopologyData.value.nodes.find(item => item.id === String(nodeId))
  const lastMove = [...draftChanges.value].reverse().find(item => item.type === 'moveNode' && String(item.nodeId) === String(nodeId))
  const from = lastMove?.to || {
    x: originalNode?.x || 0,
    y: originalNode?.y || 0
  }

  const change = {
    type: 'moveNode',
    nodeId: String(nodeId),
    from,
    to: { x, y }
  }
  pushDraftChange(change)
}

const resolveSavedResourceId = (id, createdMap) => {
  if (!id) return null
  if (createdMap.has(String(id))) return createdMap.get(String(id))
  return String(id).startsWith('tmp-') ? null : Number(id)
}

const extractSavedResourceId = (responseData, fallbackId) => {
  const saved = Array.isArray(responseData)
    ? responseData[0]
    : (responseData?.resource || responseData?.resources?.[0] || responseData)
  return saved?.id || saved?.resourceId || fallbackId
}

const saveDraftNodeBinding = async (node, parentId, createdIdMap, relationType) => {
  if (!node) return null
  const resolvedParentId = resolveSavedResourceId(parentId, createdIdMap) || Number(businessId.value)
  const existingResourceId = node.existingResourceId || null
  const res = normalizeResponse(await bindBusinessResources(businessId.value, {
    parentId: resolvedParentId,
    resources: [{
      resourceId: existingResourceId || undefined,
      name: existingResourceId ? undefined : (node.name || node.displayName || node.label),
      displayName: existingResourceId ? undefined : (node.displayName || node.name || node.label),
      resourceType: node.resourceType,
      sourceType: node.sourceType,
      sourceId: node.sourceId,
      sourceKey: node.sourceKey,
      relationType,
      properties: node.properties || {}
    }]
  }))

  if (res?.code !== 0) throw new Error(res?.message || '保存拓扑节点失败')
  const savedId = extractSavedResourceId(res.data, existingResourceId || node.id)
  if (!savedId || Number.isNaN(Number(savedId))) throw new Error('保存拓扑节点失败：后端未返回资源 ID')
  createdIdMap.set(String(node.id), Number(savedId))
  return Number(savedId)
}

const saveTopologyDraft = async () => {
  if (!draftChanges.value.length) {
    ElMessage.warning('当前没有需要保存的变更')
    return
  }

  syncGraphPositionsToTopology()
  savingTopology.value = true
  const createdIdMap = new Map()

  try {
    const deleteNodeChanges = draftChanges.value.filter(change => change.type === 'deleteNode')
    const deletedNodeIds = new Set(deleteNodeChanges.flatMap(change => (change.nodes || [change.node]).filter(Boolean).map(node => String(node.id))))
    const persistedNodeIds = new Set(completeTopologyData.value.nodes.map(node => String(node.id)))
    for (const change of deleteNodeChanges) {
      if (!persistedNodeIds.has(String(change.node.id))) continue
      const res = normalizeResponse(await deleteBusinessBindingSubtree(businessId.value, change.node.id))
      if (res?.code !== 0) throw new Error(res?.message || '级联删除节点失败')
    }

    const deleteRelationChanges = draftChanges.value.filter(change =>
      change.type === 'deleteRelation' &&
      !change.edge.isDraft &&
      !deletedNodeIds.has(String(change.edge.source)) &&
      !deletedNodeIds.has(String(change.edge.target))
    )
    for (const change of deleteRelationChanges) {
      const relationId = change.edge.relationId ?? change.edge.id
      const res = normalizeResponse(await deleteRelation(relationId))
      if (res?.code !== 0) throw new Error(res?.message || '删除拓扑连接失败')
    }

    const createChanges = draftChanges.value.filter(change => change.type === 'createResource')
    for (const change of createChanges) {
      const nodes = (change.nodes || [change.node]).filter(node => node && !deletedNodeIds.has(String(node.id)))
      const activeNodeIds = new Set([...topologyData.value.nodes.map(node => String(node.id)), ...nodes.map(node => String(node.id))])
      const edges = (change.edges || (change.edge ? [change.edge] : [])).filter(edge =>
        edge && activeNodeIds.has(String(edge.source)) && activeNodeIds.has(String(edge.target))
      )
      for (const node of nodes) {
        const parentEdge = edges.find(item => String(item.target) === String(node.id))
        const parentId = parentEdge?.source || rootNodeId.value
        await saveDraftNodeBinding(node, parentId, createdIdMap, parentEdge?.relationType)
      }
    }

    const moveChanges = draftChanges.value.filter(change => change.type === 'moveNode' && !String(change.nodeId).startsWith('tmp-') && !deletedNodeIds.has(String(change.nodeId)))
    const movedNodeIds = new Set(moveChanges.map(change => String(change.nodeId)))

    for (const nodeId of movedNodeIds) {
      const node = topologyData.value.nodes.find(item => String(resolveSavedResourceId(item.id, createdIdMap) || item.id) === String(nodeId))
      if (!node?.properties?.layout) continue
      const res = normalizeResponse(await updateResource(nodeId, {
        properties: {
          ...(node.properties || {}),
          layout: node.properties.layout
        }
      }))
      if (res?.code !== 0) throw new Error(res?.message || '保存节点坐标失败')
    }

    ElMessage.success('拓扑草稿已保存')
    isEditing.value = false
    draftChanges.value = []
    redoStack.value = []
    closeContextMenu()
    await Promise.all([loadOverview(), loadTopology()])
    await rerenderWithMode()
  } catch (error) {
    console.error('保存拓扑草稿失败:', error)
    ElMessage.error(error.message || '保存拓扑草稿失败')
  } finally {
    savingTopology.value = false
  }
}

const draftChangeText = change => {
  if (change.type === 'createResource') {
    const actionTextMap = {
      deploy_k8s: '新增 K8S 部署',
      deploy_host: '新增主机部署',
      add_nginx: '新增 Nginx',
      add_gateway: '新增 Gateway',
      dependency: '新增依赖组件',
      log_binding: '绑定日志资源',
      child: '新增下级节点',
      sibling: '新增同级节点'
    }
    return actionTextMap[change.action] || '新增拓扑节点'
  }

  const typeMap = {
    deleteRelation: '删除拓扑连接',
    deleteNode: '删除节点',
    moveNode: '移动节点'
  }
  return typeMap[change.type] || '拓扑变更'
}

const draftChangeSubText = change => {
  if (change.type === 'createResource') {
    const nodes = (change.nodes || [change.node]).filter(Boolean)
    return nodes.map(node => `${resourceTypeText(node.resourceType)} · ${node.displayName || node.name || node.label}`).join(' -> ')
  }
  if (change.type === 'deleteNode') {
    const count = (change.nodes || [change.node]).filter(Boolean).length
    return `${change.node.displayName || change.node.name} · 共 ${count} 个节点`
  }
  if (change.type === 'deleteRelation') return `${change.edge.source} -> ${change.edge.target}`
  if (change.type === 'moveNode') return change.nodeId
  return ''
}

const normalizeNodeDimensions = node => {
  if (Array.isArray(node?.dimensions) && node.dimensions.length) {
    return node.dimensions
  }
  return dimensions.value.filter(item => item.dimension === node?.dimension || item.status !== 'healthy').slice(0, 4)
}

const formatEvidence = evidence => {
  if (!evidence) return ''
  if (typeof evidence === 'string') return evidence
  try {
    return JSON.stringify(evidence, null, 2)
  } catch (error) {
    return ''
  }
}

const diagnoseActions = node => {
  const actions = []
  const type = node?.resourceType
  const sourceType = node?.sourceType

  if (['database'].includes(type) || sourceType === 'db') {
    actions.push({ label: '查看数据库监控', kind: 'db-monitor', node })
    actions.push({ label: '慢 SQL 分析', path: '/db/slowlog', query: { assetId: resolveDatabaseContext(node).assetId || undefined } })
    actions.push({ label: '连接数/锁等待', kind: 'db-monitor', node, query: { view: 'locks' } })
  } else if (['k8s_workload', 'pod'].includes(type)) {
    actions.push({ label: '查看容器详情', kind: 'k8s-workload-detail', node })
    actions.push({ label: '查看 K8S 工作负载', path: '/k8s/workload', query: buildK8sWorkloadListQuery(node) })
    actions.push({ label: '查看 K8S 监控', path: '/k8s/monitoring' })
  } else if (type === 'k8s_cluster' || sourceType === 'k8s') {
    actions.push({ label: '查看 K8S 工作负载', path: '/k8s/workload' })
    actions.push({ label: '查看 K8S 监控', path: '/k8s/monitoring' })
  } else if (['host', 'physical_server', 'vm', 'virtual', 'cloud_host'].includes(type)) {
    actions.push({ label: '查看主机详情', kind: 'host-detail', node })
    actions.push({ label: '查看主机监控', path: '/cmdb/ecs', query: { view: 'monitor', sourceId: node?.sourceId } })
    actions.push({ label: '查看进程', path: '/cmdb/ecs', query: { view: 'process', sourceId: node?.sourceId } })
    actions.push({ label: '查看磁盘/IO', path: '/cmdb/ecs', query: { view: 'disk', sourceId: node?.sourceId } })
    actions.push({ label: '查看日志分析', path: '/ai/logs_analysis' })
  } else if (node?.dimension === 'log' || sourceType === 'aiops') {
    actions.push({ label: '查看日志分析', path: '/ai/logs_analysis' })
  }

  return actions.length ? actions : [{ label: '返回业务模型', path: '/cmdb/service_model' }]
}

const buildK8sWorkloadListQuery = node => {
  const context = resolveK8sWorkloadContext(node)
  return {
    clusterId: context.clusterId,
    namespace: context.namespaceName,
    workloadType: context.workloadType,
    keyword: context.workloadName
  }
}

const goDiagnose = async action => {
  if (action.kind === 'db-monitor') {
    await openDatabaseMonitor(action.node, action.query)
    return
  }
  if (action.kind === 'k8s-workload-detail') {
    await openK8sWorkloadDetail(action.node)
    return
  }
  if (action.kind === 'host-detail') {
    await openHostDetail(action.node)
    return
  }
  router.push(action.query ? { path: action.path, query: action.query } : action.path)
}

const handleResize = () => {
  if (!graph || !graphContainer.value) return
  graph.changeSize(graphContainer.value.clientWidth || 900, graphContainer.value.clientHeight || 620)
  graph.fitView(30)
}

const handleWindowMouseUp = () => {
  finishNodeDrag()
}

const handleRefreshHealth = async () => {
  refreshing.value = true
  try {
    const res = normalizeResponse(await refreshBusinessHealth(businessId.value))
    if (res?.code !== 0) {
      throw new Error(res?.message || '刷新健康失败')
    }

    ElMessage.success('健康状态已刷新')
    await Promise.all([loadOverview(), loadTopology()])
  } catch (error) {
    console.error('刷新健康失败:', error)
    ElMessage.error(error.message || '刷新健康失败')
  } finally {
    refreshing.value = false
  }
}

const goBack = () => {
  router.push(`/cmdb/service_model/${businessId.value}`)
}

onMounted(async () => {
  await Promise.all([loadOverview(), loadTopology()])
  window.addEventListener('resize', handleResize)
  window.addEventListener('mouseup', handleWindowMouseUp)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  window.removeEventListener('mouseup', handleWindowMouseUp)
  if (k8sTargetSearchTimer) {
    clearTimeout(k8sTargetSearchTimer)
  }
  if (siblingK8sTargetSearchTimer) {
    clearTimeout(siblingK8sTargetSearchTimer)
  }
  if (childK8sTargetSearchTimer) {
    clearTimeout(childK8sTargetSearchTimer)
  }
  if (hostSearchTimer) {
    clearTimeout(hostSearchTimer)
  }
  if (siblingHostSearchTimer) {
    clearTimeout(siblingHostSearchTimer)
  }
  if (databaseAssetSearchTimer) {
    clearTimeout(databaseAssetSearchTimer)
  }
  if (graph) {
    graph.destroy()
    graph = null
  }
})
</script>

<style scoped>
.business-topology-page {
  --page-offset: 96px;
  --bottom-safe-space: 32px;
  --surface: rgba(255, 253, 248, 0.94);
  --surface-cool: rgba(247, 250, 255, 0.9);
  --text-primary: #172133;
  --text-secondary: #5f6b7c;
  --text-muted: #8d97a8;
  --border: rgba(207, 217, 230, 0.92);
  --topbar-gradient: linear-gradient(90deg, #fef7ef 0%, #f8f8f6 46%, #edf2fd 100%);

  height: 100%;
  padding: 4px 4px var(--bottom-safe-space);
  background: transparent;
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
  box-sizing: border-box;
  overflow: hidden;
}

:global(.home-container .el-main:has(.business-topology-page)) {
  overflow-y: hidden;
  padding: 0;
  padding-bottom: 0;
}

.page-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  height: 100%;
  min-height: 0;
  box-sizing: border-box;
  padding: 28px 8px 10px;
  border: 1px solid var(--border);
  border-radius: 24px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.96), rgba(247, 250, 255, 0.92));
  box-shadow: 0 18px 48px rgba(17, 24, 39, 0.08);
  overflow: hidden;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  margin: -28px -8px 0;
  padding: 10px 16px 12px;
  border-bottom: 1px solid var(--border);
  background: var(--topbar-gradient);
}

.page-kicker {
  margin: 0 0 10px;
  color: #6d7b8f;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.page-header h2 {
  margin: 0;
  color: var(--text-primary);
  font-size: 26px;
  font-weight: 700;
}

.page-description {
  margin: 6px 0 0;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.6;
}

.header-actions,
.toolbar-actions,
.legend-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.business-topology-page :deep(.el-button),
.business-topology-page :deep(.el-select__wrapper) {
  border-radius: 14px;
}

.action-button {
  min-height: 32px;
  border-radius: 12px;
  font-size: 13px;
  font-weight: 600;
  transition: border-color 0.18s ease, background 0.18s ease, box-shadow 0.18s ease, opacity 0.18s ease;
}

.action-button--confirm,
.action-button--back {
  border-color: rgba(76, 139, 245, 0.78);
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
  color: #ffffff;
  box-shadow: 0 10px 18px rgba(76, 139, 245, 0.14);
}

.action-button--success {
  border-color: rgba(63, 185, 127, 0.72);
  background: linear-gradient(180deg, #51c78d 0%, #3fb97f 100%);
  color: #ffffff;
  box-shadow: 0 10px 18px rgba(63, 185, 127, 0.14);
}

.action-button--warning {
  border-color: rgba(243, 178, 75, 0.76);
  background: linear-gradient(180deg, #f6be63 0%, #f3b24b 100%);
  color: #ffffff;
  box-shadow: 0 10px 18px rgba(243, 178, 75, 0.14);
}

.action-button--danger {
  border-color: rgba(239, 107, 115, 0.76);
  background: linear-gradient(180deg, #f47f86 0%, #ef6b73 100%);
  color: #ffffff;
  box-shadow: 0 10px 18px rgba(239, 107, 115, 0.14);
}

.action-button--secondary {
  border-color: rgba(207, 217, 230, 0.92);
  background: rgba(255, 255, 255, 0.86);
  color: var(--text-secondary);
  box-shadow: none;
}

.action-button--confirm:hover,
.action-button--confirm:focus,
.action-button--back:hover,
.action-button--back:focus {
  border-color: rgba(76, 139, 245, 0.9);
  background: linear-gradient(180deg, #78aafa 0%, #6099f7 100%);
  color: #ffffff;
  box-shadow: 0 12px 22px rgba(76, 139, 245, 0.18);
}

.action-button--success:hover,
.action-button--success:focus {
  border-color: rgba(63, 185, 127, 0.86);
  background: linear-gradient(180deg, #5ed097 0%, #46bf87 100%);
  color: #ffffff;
  box-shadow: 0 12px 22px rgba(63, 185, 127, 0.18);
}

.action-button--warning:hover,
.action-button--warning:focus {
  border-color: rgba(243, 178, 75, 0.9);
  background: linear-gradient(180deg, #f8c875 0%, #f4b95a 100%);
  color: #ffffff;
  box-shadow: 0 12px 22px rgba(243, 178, 75, 0.18);
}

.action-button--danger:hover,
.action-button--danger:focus {
  border-color: rgba(239, 107, 115, 0.9);
  background: linear-gradient(180deg, #f58b91 0%, #f0757c 100%);
  color: #ffffff;
  box-shadow: 0 12px 22px rgba(239, 107, 115, 0.18);
}

.action-button--secondary:hover,
.action-button--secondary:focus {
  border-color: rgba(76, 139, 245, 0.42);
  background: rgba(255, 255, 255, 0.96);
  color: var(--text-primary);
}

.action-button.is-disabled,
.action-button.is-loading,
.action-button:disabled {
  opacity: 0.58;
  box-shadow: none;
}

.toolbar-edit-button {
  min-height: 30px;
  padding: 0 14px;
  border-radius: 10px;
  font-size: 12px;
}

.business-topology-page :deep(.el-input-group__append .action-button) {
  min-height: 30px;
  border-radius: 0 12px 12px 0;
  box-shadow: none;
}

.toolbar-actions {
  flex-wrap: nowrap;
  justify-content: flex-end;
}

.toolbar-actions :deep(.el-select) {
  width: 128px;
  flex: 0 0 128px;
}

.toolbar-actions .toolbar-edit-button {
  flex: 0 0 auto;
  white-space: nowrap;
}

.status-strip {
  display: grid;
  grid-template-columns: repeat(5, minmax(120px, 1fr));
  gap: 10px;
}

.status-card,
.graph-panel,
.side-panel,
.panel-section {
  border: 1px solid var(--border);
  border-radius: 20px;
  background: linear-gradient(135deg, var(--surface), var(--surface-cool));
  box-shadow: 0 12px 34px rgba(17, 24, 39, 0.05);
}

.status-card {
  padding: 10px 14px;
}

.status-card span {
  color: var(--text-muted);
  font-size: 12px;
}

.status-card strong {
  display: block;
  margin-top: 4px;
  font-size: 22px;
  font-weight: 700;
}

.topology-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 360px;
  gap: 12px;
  flex: 1;
  min-height: 0;
}

.graph-panel,
.side-panel {
  min-height: 0;
  padding: 12px;
}

.graph-panel {
  display: flex;
  flex-direction: column;
}

.graph-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 8px;
}

.legend-row span {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: var(--text-secondary);
  font-size: 12px;
}

.legend-row i {
  width: 9px;
  height: 9px;
  border-radius: 50%;
}

.graph-shell {
  position: relative;
  flex: 1;
  min-height: 0;
  border: 1px solid rgba(207, 217, 230, 0.86);
  border-radius: 18px;
  background:
    radial-gradient(circle at 24% 18%, rgba(76, 139, 245, 0.055), transparent 30%),
    radial-gradient(circle at 76% 70%, rgba(63, 185, 127, 0.035), transparent 28%),
    linear-gradient(rgba(126, 152, 196, 0.035) 1px, transparent 1px),
    linear-gradient(90deg, rgba(126, 152, 196, 0.035) 1px, transparent 1px),
    rgba(252, 254, 255, 0.82);
  background-size: auto, auto, 28px 28px, 28px 28px, auto;
  overflow: hidden;
}

.topology-context-menu {
  position: absolute;
  z-index: 12;
  min-width: 150px;
  padding: 6px;
  border: 1px solid rgba(207, 217, 230, 0.96);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 16px 36px rgba(15, 23, 42, 0.16);
}

.topology-context-menu button {
  display: block;
  width: 100%;
  padding: 8px 10px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--text-secondary);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}

.topology-context-menu button:hover {
  background: rgba(76, 139, 245, 0.1);
  color: var(--text-primary);
}

.topology-context-menu button.danger {
  color: #dc2626;
}

.graph-shell::before {
  content: '';
  position: absolute;
  inset: 18px;
  border: 1px dashed rgba(174, 190, 216, 0.32);
  border-radius: 22px;
  pointer-events: none;
}

.graph-canvas {
  width: 100%;
  height: 100%;
  min-height: 0;
}

.side-panel {
  display: flex;
  flex-direction: column;
  gap: 14px;
  overflow: auto;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.side-panel::-webkit-scrollbar {
  display: none;
}

.panel-section {
  padding: 14px;
}

.draft-section {
  border-left: 4px solid #4c8bf5;
}

.draft-list {
  display: grid;
  gap: 8px;
}

.draft-list div {
  padding: 9px 10px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.72);
}

.draft-list strong {
  display: block;
  color: var(--text-primary);
  font-size: 12px;
}

.draft-list span {
  display: block;
  margin-top: 4px;
  color: var(--text-muted);
  font-size: 12px;
}

.panel-section h3 {
  margin: 0 0 12px;
  color: var(--text-primary);
  font-size: 17px;
  font-weight: 700;
}

.node-detail {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.node-title {
  display: grid;
  grid-template-columns: 10px minmax(0, 1fr) 48px;
  gap: 10px;
  align-items: start;
  padding: 12px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.8);
}

.node-title > span {
  width: 9px;
  height: 9px;
  margin-top: 5px;
  border-radius: 50%;
  background: var(--node-color);
}

.node-title strong {
  display: block;
  color: var(--text-primary);
  font-size: 14px;
}

.node-title p {
  margin: 5px 0 0;
  color: var(--text-muted);
  font-size: 12px;
}

.node-title em {
  color: var(--node-color);
  font-style: normal;
  font-size: 22px;
  font-weight: 700;
  text-align: right;
}

.detail-list {
  display: grid;
  gap: 8px;
}

.detail-list div {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  padding: 9px 10px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.68);
}

.detail-list span,
.dimension-stack span {
  color: var(--text-muted);
  font-size: 12px;
}

.detail-list strong,
.dimension-stack strong {
  color: var(--text-primary);
  font-size: 12px;
  text-align: right;
}

.observability-overview {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  padding: 9px 0;
  border-top: 1px solid rgba(223, 231, 242, 0.86);
  border-bottom: 1px solid rgba(223, 231, 242, 0.86);
}

.observability-overview > div {
  min-width: 0;
  padding: 0 8px;
  text-align: center;
}

.observability-overview > div + div {
  border-left: 1px solid rgba(223, 231, 242, 0.86);
}

.observability-overview span,
.observation-block span {
  display: block;
  color: var(--text-muted);
  font-size: 11px;
}

.observability-overview strong {
  display: block;
  margin-top: 4px;
  color: var(--observation-color, var(--text-primary));
  font-size: 13px;
}

.observability-overview .alerting strong,
.alert-summary-row strong.alerting {
  color: #dc2626;
}

.observation-details {
  border-top: 1px solid rgba(223, 231, 242, 0.86);
  border-bottom: 1px solid rgba(223, 231, 242, 0.86);
}

.observation-block {
  padding: 12px 0;
}

.observation-block + .observation-block {
  border-top: 1px solid rgba(230, 235, 243, 0.9);
}

.observation-block header {
  display: flex;
  min-height: 24px;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 8px;
}

.observation-block h4 {
  margin: 0;
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 700;
}

.observation-block header :deep(.el-button) {
  height: 24px;
  padding: 0;
  font-size: 12px;
}

.bound-log-list {
  display: grid;
  gap: 6px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.bound-log-list li {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 9px;
  padding: 7px 8px;
  border-radius: 6px;
  background: rgba(241, 245, 249, 0.72);
}

.bound-log-list img {
  width: 24px;
  height: 24px;
  flex: 0 0 24px;
}

.bound-log-list div {
  min-width: 0;
}

.bound-log-list strong {
  display: block;
  overflow: hidden;
  color: var(--text-primary);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.bound-log-list span {
  margin-top: 2px;
}

.observation-empty {
  margin: 0;
  color: var(--text-muted);
  font-size: 12px;
}

.monitor-metric-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px;
}

.monitor-metric-grid > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 7px 8px;
  border-radius: 6px;
  background: rgba(241, 245, 249, 0.72);
}

.monitor-metric-grid strong {
  color: var(--text-primary);
  font-size: 12px;
}

.monitor-match {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 8px;
  align-items: start;
  margin: 8px 0 0;
}

.monitor-match code {
  overflow-wrap: anywhere;
  color: #315f9b;
  font-family: "Consolas", "Menlo", monospace;
  font-size: 11px;
  line-height: 1.5;
  text-align: right;
}

.alert-summary-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 6px;
}

.alert-summary-row > div {
  min-width: 0;
  padding: 7px 8px;
  border-radius: 6px;
  background: rgba(241, 245, 249, 0.72);
}

.alert-summary-row strong {
  display: block;
  margin-top: 3px;
  overflow-wrap: anywhere;
  color: var(--text-primary);
  font-size: 12px;
}

.node-message {
  margin: 0;
  padding: 10px 12px;
  border: 1px dashed rgba(207, 217, 230, 0.92);
  border-radius: 14px;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.7;
  background: rgba(247, 250, 255, 0.64);
}

.evidence-box {
  padding: 10px 12px;
  border: 1px solid rgba(255, 192, 120, 0.56);
  border-radius: 14px;
  background: linear-gradient(180deg, rgba(255, 248, 238, 0.9), rgba(255, 250, 244, 0.9));
}

.evidence-box span {
  display: block;
  margin-bottom: 6px;
  color: #b67e4e;
  font-size: 12px;
  font-weight: 700;
}

.evidence-box pre {
  max-height: 120px;
  margin: 0;
  overflow: auto;
  scrollbar-width: none;
  -ms-overflow-style: none;
  color: #8a6749;
  font-family: "Consolas", "Menlo", monospace;
  font-size: 11px;
  line-height: 1.6;
  white-space: pre-wrap;
}

.evidence-box pre::-webkit-scrollbar {
  display: none;
}

.dimension-stack {
  display: grid;
  gap: 8px;
}

.dimension-stack div {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  padding: 8px 10px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  border-left: 4px solid var(--dimension-color);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.72);
}

.diagnose-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}

.node-detail-button {
  width: 100%;
  min-height: 34px;
}

.asset-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.asset-grid article {
  padding: 12px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.78);
}

.asset-grid span,
.risk-item span {
  color: var(--text-muted);
  font-size: 12px;
}

.asset-grid strong {
  display: block;
  margin-top: 6px;
  color: var(--text-primary);
  font-size: 24px;
}

.risk-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.risk-item {
  padding: 12px;
  border: 1px solid rgba(223, 231, 242, 0.86);
  border-left: 5px solid var(--risk-color);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.78);
  text-align: left;
  cursor: pointer;
}

.risk-item strong {
  display: block;
  color: var(--text-primary);
  font-size: 14px;
}

.risk-item span {
  display: block;
  margin-top: 5px;
}

.risk-item p {
  margin: 8px 0 0;
  color: var(--text-secondary);
  font-size: 12px;
  line-height: 1.6;
}

.empty-state {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.72);
  z-index: 1;
}

.empty-risk {
  padding: 26px 12px;
  border: 1px dashed rgba(207, 217, 230, 0.86);
  border-radius: 16px;
  color: var(--text-muted);
  text-align: center;
  font-size: 13px;
}

.resource-dialog-body {
  display: grid;
  gap: 14px;
}

.sibling-hint {
  display: grid;
  gap: 5px;
  padding: 12px 14px;
  border: 1px solid rgba(207, 217, 230, 0.88);
  border-radius: 14px;
  background: rgba(248, 251, 255, 0.82);
}

.sibling-hint span,
.sibling-hint em {
  color: var(--text-muted);
  font-size: 12px;
  font-style: normal;
}

.sibling-hint strong {
  color: var(--text-primary);
  font-size: 14px;
}

.k8s-selector-tip {
  margin: -2px 0 0;
  color: var(--text-muted);
  font-size: 12px;
  line-height: 1.6;
}

.k8s-target-option,
.host-target-option,
.service-target-option,
.db-target-option,
.log-target-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
}

.k8s-target-option strong,
.host-target-option strong,
.service-target-option strong,
.db-target-option strong,
.log-target-option strong {
  color: var(--text-primary);
  font-size: 13px;
}

.k8s-target-option span,
.host-target-option span,
.service-target-option span,
.db-target-option span,
.log-target-option span {
  color: var(--text-muted);
  font-size: 12px;
}

.log-target-option {
  justify-content: flex-start;
}

.log-target-option img {
  width: 22px;
  height: 22px;
  flex: 0 0 22px;
}

.log-target-option strong {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dependency-filter-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 10px;
}

.resource-type-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.resource-type-grid button,
.existing-resource-list button,
.combo-template-list button {
  border: 1px solid rgba(223, 231, 242, 0.9);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.8);
  color: var(--text-secondary);
  text-align: left;
  cursor: pointer;
}

.resource-type-grid button {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px;
  font-size: 12px;
}

.resource-type-grid button.active,
.existing-resource-list button.active {
  border-color: rgba(76, 139, 245, 0.72);
  background: rgba(76, 139, 245, 0.1);
  color: var(--text-primary);
}

.resource-type-grid img {
  width: 24px;
  height: 24px;
}

.resource-mode-group {
  width: fit-content;
}

.existing-resource-picker,
.existing-resource-list,
.combo-template-list {
  display: grid;
  gap: 10px;
}

.existing-resource-list {
  max-height: 240px;
  overflow: auto;
}

.existing-resource-list button,
.combo-template-list button {
  padding: 11px 12px;
}

.existing-resource-list strong,
.combo-template-list strong {
  display: block;
  color: var(--text-primary);
  font-size: 13px;
}

.existing-resource-list span,
.combo-template-list span {
  display: block;
  margin-top: 5px;
  color: var(--text-muted);
  font-size: 12px;
}

@media (max-width: 1200px) {
  .page-card {
    height: auto;
    min-height: 100%;
  }

  .topology-layout {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 760px) {
  .page-header,
  .graph-toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .status-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .asset-grid {
    grid-template-columns: 1fr;
  }
}
</style>
