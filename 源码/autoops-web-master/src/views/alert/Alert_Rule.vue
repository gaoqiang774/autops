<template>
  <div class="alert-center-page alert-rule-page">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/警告.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>告警管理</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>统一维护告警规则、规则分组和静默规则，管理完整告警触发配置。</p>
      </div>
      <div class="rule-header-summary" aria-label="告警规则统计">
        <span><em>规则</em><strong>{{ totals.rules }}</strong></span>
        <i aria-hidden="true"></i>
        <span><em>分组</em><strong>{{ totals.groups }}</strong></span>
        <i aria-hidden="true"></i>
        <span><em>静默</em><strong>{{ totals.silences }}</strong></span>
      </div>
    </header>

    <el-card shadow="never" class="alert-center-card">
      <el-tabs v-model="activeTab" class="alert-tabs" @tab-change="handleTabChange">
        <el-tab-pane label="告警规则" name="rules" />
        <el-tab-pane label="静默规则" name="silences" />
      </el-tabs>

      <div v-if="activeTab === 'rules'" class="rule-workbench">
        <aside class="rule-group-panel">
          <div class="rule-group-head">
            <div>
              <strong>规则分组</strong>
              <span>按规则分组查看告警配置</span>
            </div>
            <button type="button" class="rule-icon-button" title="新增分组" @click="openGroupDialog()">
              <el-icon><Plus /></el-icon>
            </button>
          </div>

          <button
            type="button"
            class="rule-group-all"
            :class="{ active: selectedGroupId === '' }"
            @click="selectGroup('')"
          >
            <img :src="closedFolderIcon" alt="" class="rule-group-folder-icon" />
            <span class="rule-group-name">全部规则</span>
            <small class="rule-group-count">{{ totals.rules }}</small>
          </button>

          <div class="rule-group-list" v-loading="groupLoading">
            <div
              v-for="group in groupRows"
              :key="group.id"
              class="rule-group-item"
              :class="{ active: selectedGroupId === group.id }"
              @click="selectGroup(group.id)"
              @contextmenu.prevent.stop="openGroupDialog(group)"
            >
              <img :src="closedFolderIcon" alt="" class="rule-group-folder-icon" />
              <span class="rule-group-name">{{ group.name }}</span>
              <small class="rule-group-count">{{ groupRuleCountText(group) }}</small>
              <span class="rule-group-tools" @click.stop>
                <button type="button" title="修改分组" @click="openGroupDialog(group)">
                  <el-icon><EditPen /></el-icon>
                </button>
                <button type="button" class="danger" title="删除分组" @click="deleteGroup(group)">
                  <el-icon><Delete /></el-icon>
                </button>
              </span>
            </div>
            <el-empty v-if="!groupLoading && !groupRows.length" description="暂无规则分组" :image-size="64" />
          </div>
        </aside>

        <main class="rule-list-panel">
          <section class="alert-search-section">
            <el-form :model="query" :inline="true" class="alert-search-form" @submit.prevent>
              <el-form-item label="关键字">
                <el-input v-model="query.keyword" class="alert-filter-input" clearable placeholder="名称 / 查询语句" @keyup.enter="handleSearch" />
              </el-form-item>
              <el-form-item label="级别">
                <el-select v-model="query.severity" class="alert-filter-small" clearable placeholder="全部">
                  <el-option v-for="item in severityOptions" :key="item.value" :label="item.label" :value="item.value" />
                </el-select>
              </el-form-item>
              <el-form-item label="启用">
                <el-select v-model="query.enabled" class="alert-filter-small" clearable placeholder="全部">
                  <el-option v-for="item in enabledOptions" :key="item.value" :label="item.label" :value="item.value" />
                </el-select>
              </el-form-item>
              <el-form-item label="数据源" class="rule-datasource-item">
                <el-select v-model="query.datasourceId" class="rule-datasource-filter" clearable placeholder="全部">
                  <el-option
                    v-for="item in datasourceOptionRows"
                    :key="datasourceOptionId(item)"
                    :label="datasourceOptionLabel(item)"
                    :value="datasourceOptionId(item)"
                  />
                </el-select>
              </el-form-item>
              <el-form-item>
                <el-button type="primary" size="small" class="alert-search-button rule-toolbar-button" @click="handleSearch">
                  <el-icon><Search /></el-icon>
                  <span>查询</span>
                </el-button>
                <el-button type="warning" size="small" class="alert-reset-button rule-toolbar-button" @click="handleReset">
                  <el-icon><RefreshRight /></el-icon>
                  <span>重置</span>
                </el-button>
                <el-button type="success" size="small" class="alert-create-button rule-toolbar-button" @click="openCreateDialog">
                  <el-icon><Plus /></el-icon>
                  <span>新增</span>
                </el-button>
                <el-dropdown class="rule-more-dropdown" trigger="click" @command="handleMoreCommand">
                  <el-button type="warning" size="small" class="rule-more-button rule-toolbar-button">
                    更多
                    <el-icon><ArrowDown /></el-icon>
                  </el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="batchDatasource" :disabled="!selectedRuleRows.length">批量修改数据源</el-dropdown-item>
                      <el-dropdown-item command="batchNotifyPolicy" :disabled="!selectedRuleRows.length">批量修改告警策略</el-dropdown-item>
                      <el-dropdown-item command="yamlImport">导入 YAML</el-dropdown-item>
                      <el-dropdown-item command="templateImport">从模板创建规则</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </el-form-item>
            </el-form>
          </section>

          <div class="alert-section-header">
            <span>{{ selectedGroupName }} / 共 {{ totals.rules }} 条规则</span>
          </div>

          <div class="alert-table-shell">
            <el-table class="alert-table" :data="rows" v-loading="loading" empty-text="暂无告警规则" @selection-change="handleRuleSelectionChange">
              <el-table-column type="selection" width="46" />
              <el-table-column label="规则名称" min-width="210">
                <template #default="{ row }">
                  <button type="button" class="rule-name-link" @click="openEditDialog(row)">{{ row.name || '-' }}</button>
                </template>
              </el-table-column>
              <el-table-column label="告警等级" width="120">
                <template #default="{ row }">
                  <div class="alert-tag-list rule-severity-tags">
                    <el-tag v-for="item in ruleSeverityList(row)" :key="item" :type="severityTagType(item)" effect="light">{{ severityCodeLabel(item) }}</el-tag>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="数据源" width="120">
                <template #default="{ row }">
                  <el-tag effect="light">{{ datasourceName(row) }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="告警策略" min-width="180" show-overflow-tooltip>
                <template #default="{ row }">
                  <span class="rule-notify-policy-text">{{ notifyRuleNames(row) }}</span>
                </template>
              </el-table-column>
              <el-table-column label="描述" min-width="220">
                <template #default="{ row }">
                  <el-tooltip
                    :content="ruleDescription(row)"
                    placement="top"
                    :show-after="300"
                    popper-class="rule-description-tooltip"
                  >
                    <span class="rule-description-text">{{ ruleDescription(row) }}</span>
                  </el-tooltip>
                </template>
              </el-table-column>
              <el-table-column label="更新时间" width="160">
                <template #default="{ row }">{{ formatDateTime(row.updatedAt || row.updated_at || row.createdAt || row.created_at) }}</template>
              </el-table-column>
              <el-table-column label="操作人" width="100">
                <template #default="{ row }">
                  <el-tag effect="plain">{{ row.operator || row.updatedBy || row.updated_by || row.createdBy || row.created_by || '-' }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column label="状态" width="100">
                <template #default="{ row }">
                  <el-switch
                    :model-value="Number(row.enabled) === 1"
                    :loading="statusLoadingId === row.id"
                    active-color="#3fb97f"
                    inactive-color="#d2d9e5"
                    @change="value => toggleRuleStatus(row, value)"
                  />
                </template>
              </el-table-column>
              <el-table-column label="操作" width="150" fixed="right">
                <template #default="{ row }">
                  <div class="alert-operation-links">
                    <button type="button" class="alert-operation-link alert-operation-success" @click="openCloneDialog(row)">克隆</button>
                    <button type="button" class="alert-operation-link" @click="openEditDialog(row)">修改</button>
                    <button type="button" class="alert-operation-link alert-operation-danger" @click="handleDelete(row)">删除</button>
                  </div>
                </template>
              </el-table-column>
            </el-table>
          </div>

          <div class="alert-pagination">
            <el-pagination
              v-model:current-page="query.page"
              v-model:page-size="query.pageSize"
              :page-sizes="[10, 20, 50, 100]"
              :total="totals.rules"
              layout="total, sizes, prev, pager, next, jumper"
              @size-change="loadList"
              @current-change="loadList"
            />
          </div>
        </main>
      </div>

      <template v-else>
        <section class="alert-search-section">
          <el-form :model="query" :inline="true" class="alert-search-form" @submit.prevent>
            <el-form-item label="关键字">
              <el-input v-model="query.keyword" class="alert-filter-input" clearable placeholder="名称 / 原因" @keyup.enter="handleSearch" />
            </el-form-item>
            <el-form-item label="启用">
              <el-select v-model="query.enabled" class="alert-filter-small" clearable placeholder="全部">
                <el-option v-for="item in enabledOptions" :key="item.value" :label="item.label" :value="item.value" />
              </el-select>
            </el-form-item>
            <el-form-item>
              <el-button type="primary" size="small" class="alert-search-button rule-toolbar-button" @click="handleSearch">
                <el-icon><Search /></el-icon>
                <span>查询</span>
              </el-button>
              <el-button type="warning" size="small" class="alert-reset-button rule-toolbar-button" @click="handleReset">
                <el-icon><RefreshRight /></el-icon>
                <span>重置</span>
              </el-button>
              <el-button type="success" size="small" class="alert-create-button rule-toolbar-button" @click="openCreateDialog">
                <el-icon><Plus /></el-icon>
                <span>新增静默</span>
              </el-button>
            </el-form-item>
          </el-form>
        </section>

        <div class="alert-section-header">
          <span>按标签匹配告警，并在指定时间窗口内停止触发通知</span>
        </div>

        <div class="alert-table-shell">
          <el-table class="alert-table" :data="rows" v-loading="loading" empty-text="暂无静默规则">
            <el-table-column label="静默名称" min-width="190">
              <template #default="{ row }">
                <div class="alert-main-text">{{ row.name || '-' }}</div>
                <div class="alert-muted-text">{{ row.reason || '-' }}</div>
              </template>
            </el-table-column>
            <el-table-column label="匹配条件" min-width="260">
              <template #default="{ row }">{{ summarizeMatchers(row.matchers) }}</template>
            </el-table-column>
            <el-table-column label="时间窗口" min-width="260">
              <template #default="{ row }">{{ formatDateTime(row.startTime) }} - {{ formatDateTime(row.endTime) }}</template>
            </el-table-column>
            <el-table-column label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="silenceStatusTagType(row)" effect="light">{{ silenceStatusText(row) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="创建人" width="110">
              <template #default="{ row }">{{ row.createdBy || row.created_by || '-' }}</template>
            </el-table-column>
            <el-table-column label="操作" width="150" fixed="right">
              <template #default="{ row }">
                <div class="alert-operation-links">
                  <button type="button" class="alert-operation-link alert-operation-success" @click="openCloneDialog(row)">克隆</button>
                  <button type="button" class="alert-operation-link" @click="openEditDialog(row)">修改</button>
                  <button v-if="silenceStatus(row) === 'active' || silenceStatus(row) === 'pending'" type="button" class="alert-operation-link alert-operation-danger" @click="handleDelete(row)">取消</button>
                </div>
              </template>
            </el-table-column>
          </el-table>
        </div>

        <div class="alert-pagination">
          <el-pagination
            v-model:current-page="query.page"
            v-model:page-size="query.pageSize"
            :page-sizes="[10, 20, 50, 100]"
            :total="totals.silences"
            layout="total, sizes, prev, pager, next, jumper"
            @size-change="loadList"
            @current-change="loadList"
          />
        </div>
      </template>
    </el-card>

    <el-dialog v-model="batchDatasourceVisible" title="批量修改数据源" width="520px" class="alert-dialog" modal-class="alert-rule-dialog-overlay" destroy-on-close>
      <el-form :model="batchDatasourceForm" label-width="96px">
        <el-form-item label="已选规则">
          <span class="alert-muted-text">{{ selectedRuleRows.length }} 条</span>
        </el-form-item>
        <el-form-item label="目标数据源" required>
          <el-select v-model="batchDatasourceForm.datasourceId" class="full-select" placeholder="请选择目标数据源">
            <el-option
              v-for="item in datasourceOptionRows"
              :key="datasourceOptionId(item)"
              :label="datasourceOptionLabel(item)"
              :value="datasourceOptionId(item)"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="batchDatasourceVisible = false">取消</el-button>
        <el-button type="primary" :loading="batchDatasourceSaving" @click="submitBatchDatasource">确认修改</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="batchNotifyPolicyVisible" title="批量修改告警策略" width="560px" class="alert-dialog" modal-class="alert-rule-dialog-overlay" destroy-on-close>
      <el-form :model="batchNotifyPolicyForm" label-width="96px">
        <el-form-item label="已选规则">
          <span class="alert-muted-text">{{ selectedRuleRows.length }} 条</span>
        </el-form-item>
        <el-form-item label="修改方式">
          <el-radio-group v-model="batchNotifyPolicyForm.mode">
            <el-radio value="replace">替换策略</el-radio>
            <el-radio value="clear">清除策略</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="batchNotifyPolicyForm.mode === 'replace'" label="通知策略" required>
          <el-select
            v-model="batchNotifyPolicyForm.notifyRuleIds"
            class="full-select"
            multiple
            clearable
            collapse-tags
            collapse-tags-tooltip
            placeholder="请选择通知策略"
          >
            <el-option
              v-for="item in notifyRuleOptions"
              :key="notifyRuleId(item)"
              :label="notifyRuleLabel(item)"
              :value="notifyRuleId(item)"
            />
          </el-select>
        </el-form-item>
        <el-form-item v-else label="配置结果">
          <span class="alert-muted-text">所选规则将不再绑定任何通知策略</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="batchNotifyPolicyVisible = false">取消</el-button>
        <el-button type="primary" :loading="batchNotifyPolicySaving" @click="submitBatchNotifyPolicy">确认修改</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="980px" class="alert-dialog rule-edit-dialog" modal-class="alert-rule-dialog-overlay" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="92px">
        <div v-if="activeTab === 'rules'" class="rule-edit-sections">
          <section class="rule-edit-section">
            <div class="rule-edit-section-head">
              <h3>基础配置</h3>
              <span>定义规则归属和列表描述</span>
            </div>
            <div class="rule-basic-grid">
              <el-form-item label="规则名称" prop="name">
                <el-input v-model="form.name" placeholder="请输入规则名称" />
              </el-form-item>
              <el-form-item label="规则分组">
                <el-select v-model="form.groupId" class="full-select" clearable placeholder="请选择规则分组">
                  <el-option v-for="item in groupRows" :key="item.id" :label="item.name" :value="item.id" />
                </el-select>
              </el-form-item>
              <el-form-item label="数据源" prop="datasourceId">
                <el-select v-model="form.datasourceId" class="full-select" placeholder="请选择数据源" @change="handleRuleDatasourceChange">
                  <el-option
                    v-for="item in datasourceOptionRows"
                    :key="datasourceOptionId(item)"
                    :label="datasourceOptionLabel(item)"
                    :value="datasourceOptionId(item)"
                  />
                </el-select>
              </el-form-item>
              <el-form-item label="规则描述" class="alert-form-full">
                <el-input v-model="form.note" placeholder="请输入规则描述" />
              </el-form-item>
            </div>
          </section>

          <section class="rule-edit-section">
            <div class="rule-edit-section-head">
              <h3>规则配置</h3>
              <span>配置查询语句和触发条件</span>
            </div>
            <div class="rule-config-panel">
              <el-form-item prop="ruleQueries" class="rule-query-item">
                <div class="rule-query-field">
                  <div class="rule-query-title">告警条件</div>
                  <div class="rule-query-list">
                    <div v-for="(item, index) in form.ruleQueries" :key="index" class="rule-query-row">
                      <div class="rule-query-main">
                        <el-select
                          v-model="item.severity"
                          placeholder="告警级别"
                          class="rule-query-severity"
                          :class="`severity-${severityCodeLabel(item.severity).toLowerCase()}`"
                        >
                          <template #label="{ label, value }">
                            <span class="rule-severity-selected" :class="`severity-${severityCodeLabel(value).toLowerCase()}`">
                              <span>{{ severityCodeLabel(value) }}</span>
                              <em>{{ label.replace(severityCodeLabel(value), '').trim() }}</em>
                            </span>
                          </template>
                          <el-option v-for="option in ruleSeverityOptions" :key="option.value" :label="option.label" :value="option.value">
                            <span class="rule-severity-option" :class="`severity-${severityCodeLabel(option.value).toLowerCase()}`">
                              {{ severityCodeLabel(option.value) }}
                            </span>
                            <span>{{ option.label.replace(severityCodeLabel(option.value), '').trim() }}</span>
                          </el-option>
                        </el-select>
                        <el-input
                          v-model="item.prom_ql"
                          class="rule-query-textarea"
                          placeholder="查询语句 PromQL，例如 system_memory_usage_percent > 80"
                        />
                        <div class="rule-query-actions">
                          <el-button class="rule-query-action rule-query-preview" title="预览" :loading="previewingQuery === item" @click="previewRuleQuery(item)">
                            <el-icon><Search /></el-icon>
                          </el-button>
                          <el-button class="rule-query-action rule-query-plus" title="新增条件" @click="addRuleQuery">
                            <el-icon><Plus /></el-icon>
                          </el-button>
                          <el-button
                            v-if="form.ruleQueries.length > 1"
                            class="rule-query-action rule-query-remove"
                            title="删除条件"
                            @click="removeRuleQuery(index)"
                          >
                            <el-icon><Delete /></el-icon>
                          </el-button>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </el-form-item>

              <div class="rule-condition-grid">
                <el-form-item label="持续次数">
                  <el-input-number v-model="form.forDurationCount" :min="0" :precision="0" controls-position="right" class="full-input-number" />
                </el-form-item>
                <el-form-item label="执行频率">
                  <el-input-number v-model="form.evalIntervalSeconds" :min="1" controls-position="right" class="full-input-number" />
                </el-form-item>
                <el-form-item label="恢复等待">
                  <el-input-number v-model="form.recoverDurationSeconds" :min="0" controls-position="right" class="full-input-number" />
                </el-form-item>
              </div>
            </div>
          </section>

          <section class="rule-edit-section">
            <div class="rule-edit-section-head">
              <h3>生效配置</h3>
              <span>控制规则启停和生效时间窗口</span>
            </div>
            <div class="rule-effective-grid">
              <div class="rule-effective-row rule-effective-time-row">
                <el-form-item label="每周生效" class="rule-weekday-item">
                  <el-checkbox-group v-model="form.effectiveWeekdays" class="rule-weekdays">
                    <el-checkbox-button v-for="item in weekdayOptions" :key="item.value" :label="item.value">{{ item.label }}</el-checkbox-button>
                  </el-checkbox-group>
                </el-form-item>
                <el-form-item label="开始时间" class="rule-time-item">
                  <el-time-select
                    v-model="form.effectiveStartTime"
                    start="00:00"
                    step="00:30"
                    end="23:30"
                    placeholder="开始"
                    class="full-date"
                  />
                </el-form-item>
                <el-form-item label="结束时间" class="rule-time-item">
                  <el-time-select
                    v-model="form.effectiveEndTime"
                    start="00:30"
                    step="00:30"
                    end="23:59"
                    placeholder="结束"
                    class="full-date"
                  />
                </el-form-item>
              </div>
              <div class="rule-effective-row rule-effective-switch-row">
                <el-form-item label="立即启用">
                  <el-switch v-model="form.enabled" :active-value="1" :inactive-value="0" active-text="启用" inactive-text="禁用" />
                </el-form-item>
                <el-form-item label="业务组内">
                  <el-switch v-model="form.enableInBg" :active-value="1" :inactive-value="0" active-text="仅本业务组" inactive-text="不限" />
                </el-form-item>
              </div>
            </div>
          </section>

          <section class="rule-edit-section">
            <div class="rule-edit-section-head">
              <h3>通知配置</h3>
              <span>选择通知策略和重复发送方式</span>
            </div>
            <div class="rule-notify-grid">
              <el-form-item label="通知策略" prop="notifyRuleIds" class="notify-policy-item">
                <el-select v-model="form.notifyRuleIds" class="full-select" multiple clearable collapse-tags collapse-tags-tooltip placeholder="请选择通知策略">
                  <el-option
                    v-for="item in notifyRuleOptions"
                    :key="notifyRuleId(item)"
                    :label="notifyRuleLabel(item)"
                    :value="notifyRuleId(item)"
                  />
                </el-select>
              </el-form-item>
            </div>
          </section>

        </div>

        <div v-else class="alert-form-grid">
          <el-form-item label="静默名称" prop="name">
            <el-input v-model="form.name" placeholder="请输入静默规则名称" />
          </el-form-item>
          <el-form-item label="开始时间" prop="startTime">
            <el-date-picker v-model="form.startTime" type="datetime" value-format="YYYY-MM-DD HH:mm:ss" class="full-date" placeholder="选择开始时间" />
          </el-form-item>
          <el-form-item label="结束时间" prop="endTime">
            <el-date-picker v-model="form.endTime" type="datetime" value-format="YYYY-MM-DD HH:mm:ss" class="full-date" placeholder="选择结束时间" />
          </el-form-item>
          <el-form-item label="启用状态">
            <el-switch v-model="form.enabled" :active-value="1" :inactive-value="0" active-text="启用" inactive-text="禁用" />
          </el-form-item>
          <el-form-item label="原因" prop="reason" class="alert-form-full">
            <el-input v-model="form.reason" type="textarea" :rows="3" placeholder="请输入静默原因" />
          </el-form-item>
          <el-form-item label="匹配条件" prop="matchers" class="alert-form-full">
            <div class="silence-matcher-list">
              <div v-for="(matcher, index) in form.matchers" :key="index" class="silence-matcher-row">
                <el-input v-model="matcher.label" placeholder="标签名，例如 service" />
                <el-select v-model="matcher.op" class="silence-matcher-op">
                  <el-option v-for="item in silenceMatcherOperatorOptions" :key="item.value" :label="item.label" :value="item.value" />
                </el-select>
                <el-input v-model="matcher.value" placeholder="匹配值" />
                <el-button class="silence-matcher-remove" :icon="Delete" circle title="删除匹配条件" @click="removeSilenceMatcher(index)" />
              </div>
              <el-button class="silence-matcher-add" :icon="Plus" @click="addSilenceMatcher">添加匹配条件</el-button>
            </div>
          </el-form-item>
        </div>
      </el-form>

      <template #footer>
        <div class="alert-dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="saving" @click="submitForm">确认</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="groupDialogVisible" :title="groupEditingId ? '修改规则分组' : '新增规则分组'" width="560px" class="alert-dialog" modal-class="alert-rule-dialog-overlay" destroy-on-close>
      <el-form ref="groupFormRef" :model="groupForm" :rules="groupFormRules" label-width="96px">
        <el-form-item label="分组名称" prop="name">
          <el-input v-model="groupForm.name" placeholder="请输入分组名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="groupForm.description" type="textarea" :rows="3" placeholder="请输入分组描述" />
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="alert-dialog-footer">
          <el-button @click="groupDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="groupSaving" @click="submitGroupForm">确认</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="importDialogVisible" title="从模板创建规则" width="920px" class="alert-dialog import-rule-dialog" modal-class="alert-rule-dialog-overlay" destroy-on-close>
      <section class="import-toolbar">
        <el-input v-model="builtinQuery.keyword" class="import-keyword-input" clearable placeholder="搜索模板名称" @keyup.enter="loadBuiltinRules" />
        <el-cascader
          v-model="builtinGroupPath"
          :options="builtinGroupOptions"
          :props="builtinGroupProps"
          class="import-group-cascader"
          clearable
          filterable
          placeholder="告警一级分组"
          @change="handleBuiltinGroupChange"
        />
        <el-select v-model="importForm.groupId" class="import-target-select" clearable placeholder="目标分组">
          <el-option v-for="item in groupRows" :key="item.id" :label="item.name" :value="item.id" />
        </el-select>
        <el-select v-model="importForm.datasourceId" class="import-target-select" clearable placeholder="数据源">
          <el-option
            v-for="item in datasourceOptionRows"
            :key="datasourceOptionId(item)"
            :label="datasourceOptionLabel(item)"
            :value="datasourceOptionId(item)"
          />
        </el-select>
        <el-button type="primary" size="small" class="alert-search-button" @click="loadBuiltinRules">
          <el-icon><Search /></el-icon>
          <span>查询</span>
        </el-button>
      </section>

      <div class="import-table-shell">
        <el-table
          ref="builtinTableRef"
          class="alert-table"
          :data="builtinRows"
          v-loading="builtinLoading"
          empty-text="暂无告警模板"
          @selection-change="selectedBuiltinRows = $event"
        >
          <el-table-column type="selection" width="44" />
          <el-table-column label="模板名称" min-width="210">
            <template #default="{ row }">
              <div class="alert-main-text">{{ row.name || '-' }}</div>
              <div class="alert-muted-text">{{ row.description || '-' }}</div>
            </template>
          </el-table-column>
          <el-table-column label="分类" width="100" prop="category" />
          <el-table-column label="级别" width="92">
            <template #default="{ row }">
              <el-tag :type="severityTagType(row.defaultSeverity)" effect="light">{{ severityLabel(row.defaultSeverity) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="查询模板" min-width="260">
            <template #default="{ row }">
              <span class="alert-code">{{ row.queryTemplate || '-' }}</span>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <template #footer>
        <div class="alert-dialog-footer">
          <span class="import-selected-count">已选择 {{ selectedBuiltinRows.length }} 条</span>
          <el-button @click="importDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="importing" @click="importBuiltinRules">创建规则</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="yamlImportVisible" title="导入 Prometheus Rule YAML" width="760px" class="alert-dialog yaml-import-dialog" modal-class="alert-rule-dialog-overlay" destroy-on-close>
      <el-form ref="yamlImportFormRef" :model="yamlImportForm" :rules="yamlImportRules" label-width="104px">
        <div class="alert-form-grid">
          <el-form-item label="规则分组" prop="groupId">
            <el-select v-model="yamlImportForm.groupId" class="full-select" placeholder="请选择规则分组">
              <el-option v-for="item in groupRows" :key="item.id" :label="item.name" :value="item.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="数据源" prop="datasourceId">
            <el-select v-model="yamlImportForm.datasourceId" class="full-select" placeholder="请选择数据源">
              <el-option
                v-for="item in datasourceOptionRows"
                :key="datasourceOptionId(item)"
                :label="datasourceOptionLabel(item)"
                :value="datasourceOptionId(item)"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="默认启用">
            <el-switch v-model="yamlImportForm.defaultEnabled" :active-value="1" :inactive-value="0" active-text="启用" inactive-text="禁用" />
          </el-form-item>
          <el-form-item label="YAML 内容" prop="content" class="alert-form-full">
            <el-input
              v-model="yamlImportForm.content"
              type="textarea"
              :rows="14"
              class="yaml-import-textarea"
              :placeholder="yamlImportExample"
            />
          </el-form-item>
        </div>
      </el-form>
      <template #footer>
        <div class="alert-dialog-footer">
          <el-button @click="yamlImportVisible = false">取消</el-button>
          <el-button type="primary" :loading="yamlImporting" @click="submitYamlImport">确认导入</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="previewDialogVisible" title="查询预览" width="820px" class="alert-dialog rule-preview-dialog" modal-class="alert-rule-dialog-overlay" destroy-on-close>
      <div class="rule-preview-meta">
        <span>类型：{{ previewResult.resultType || '-' }}</span>
        <span>总数：{{ previewResult.total ?? 0 }}</span>
        <span>截断：{{ previewResult.limited ? '是' : '否' }}</span>
        <span>执行时间：{{ formatDateTime(previewResult.executedAt) }}</span>
      </div>
      <div v-if="previewWarnings.length" class="rule-preview-warnings">
        {{ previewWarnings.join('；') }}
      </div>
      <div v-if="previewRows.length" class="rule-preview-list">
        <div v-for="(row, index) in previewRows" :key="index" class="rule-preview-row">
          <div class="rule-preview-series">
            <span class="rule-preview-metric">{{ row.metricName }}</span>
            <span class="rule-preview-brace">{</span>
            <span v-for="label in row.labels" :key="label.key" class="rule-preview-label">
              <b>{{ label.key }}</b>=&quot;{{ label.value }}&quot;
            </span>
            <span class="rule-preview-brace">}</span>
          </div>
          <span class="rule-preview-value">{{ row.value }}</span>
        </div>
      </div>
      <el-empty v-else :description="Number(previewResult.total) === 0 ? '当前查询无数据' : '暂无预览结果'" :image-size="72" />
      <template #footer>
        <div class="alert-dialog-footer">
          <el-button type="primary" @click="previewDialogVisible = false">关闭</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ArrowDown, Delete, EditPen, Plus, RefreshRight, Search } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import alertApi from '@/api/alertmanager'
import {
  compactObject,
  enabledOptions,
  formatDateTime,
  isSuccess,
  normalizeSeverity,
  normalizeListResult,
  severityLabel,
  severityOptions,
  severityTagType,
  unwrapData
} from './alertCommon'
import './alert-pages.css'

const closedFolderIcon = new URL('@/assets/image/关闭文件夹.svg', import.meta.url).href
const DEFAULT_EVAL_INTERVAL_SECONDS = 60
const DEFAULT_FOR_DURATION_COUNT = 3
const DEFAULT_FOR_DURATION_SECONDS = DEFAULT_EVAL_INTERVAL_SECONDS * DEFAULT_FOR_DURATION_COUNT

const silenceMatcherOperatorOptions = [
  { label: '=', value: '=' },
  { label: '!=', value: '!=' },
  { label: '=~', value: '=~' },
  { label: '!~', value: '!~' }
]

const route = useRoute()
const activeTab = ref(route.query.tab === 'silences' ? 'silences' : 'rules')
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const previewDialogVisible = ref(false)
const editingId = ref(null)
const statusLoadingId = ref(null)
const previewingQuery = ref(null)
const rows = ref([])
const formRef = ref()
const totals = reactive({ rules: 0, groups: 0, silences: 0 })
const selectedRuleRows = ref([])
const batchDatasourceVisible = ref(false)
const batchDatasourceSaving = ref(false)
const batchDatasourceForm = reactive({ datasourceId: '' })
const batchNotifyPolicyVisible = ref(false)
const batchNotifyPolicySaving = ref(false)
const batchNotifyPolicyForm = reactive({ mode: 'replace', notifyRuleIds: [] })

const groupLoading = ref(false)
const groupRows = ref([])
const datasourceOptionRows = ref([])
const notifyRuleOptions = ref([])
const selectedGroupId = ref('')
const groupDialogVisible = ref(false)
const groupSaving = ref(false)
const groupEditingId = ref(null)
const groupFormRef = ref()
const groupForm = reactive({})

const importDialogVisible = ref(false)
const builtinLoading = ref(false)
const importing = ref(false)
const builtinRows = ref([])
const builtinGroupTree = ref([])
const builtinGroupPath = ref([])
const selectedBuiltinRows = ref([])
const builtinQuery = reactive({ page: 1, pageSize: 100, keyword: '', component: '', collector: '', category: '', enabled: '1' })
const importForm = reactive({ groupId: '', datasourceId: '' })

const yamlImportVisible = ref(false)
const yamlImporting = ref(false)
const yamlImportFormRef = ref()
const yamlImportForm = reactive({
  groupId: '',
  datasourceId: '',
  defaultEnabled: 1,
  content: ''
})

const yamlImportExample = `# 示例:
rules:
- alert: Exporter Componen is Down
  expr: up == 0
  for: 2m
  labels:
    severity: serious
  annotations:
    summary: 节点 Exporter Componen is Down
    description: 节点 Exporter Componen is Down`

const query = reactive({
  page: 1,
  pageSize: 10,
  keyword: '',
  severity: '',
  datasourceId: '',
  enabled: ''
})

const form = reactive({})
const previewResult = reactive({})

const formRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  startTime: [{ required: true, message: '请选择开始时间', trigger: 'change' }],
  endTime: [{ required: true, message: '请选择结束时间', trigger: 'change' }],
  reason: [{ required: true, message: '请输入静默原因', trigger: 'blur' }],
  matchers: [{
    validator: (_rule, value, callback) => {
      const items = Array.isArray(value) ? value : []
      if (!items.length || items.some(item => !String(item?.label || '').trim())) {
        callback(new Error('请至少配置一个完整的标签匹配条件'))
        return
      }
      callback()
    },
    trigger: 'change'
  }],
  datasourceId: [{ required: true, message: '请选择数据源', trigger: 'change' }],
  notifyRuleIds: [{
    validator: (_rule, value, callback) => {
      if (!Array.isArray(value) || !value.length) {
        callback(new Error('请选择通知策略'))
        return
      }
      callback()
    },
    trigger: 'change'
  }],
  ruleQueries: [{
    validator: (rule, value, callback) => {
      const items = Array.isArray(value) ? value : []
      if (!items.length || items.some(item => !String(item?.prom_ql || '').trim() || !item?.severity)) {
        callback(new Error('请完善告警条件'))
        return
      }
      callback()
    },
    trigger: 'blur'
  }]
}

const weekdayOptions = [
  { label: '一', value: 1 },
  { label: '二', value: 2 },
  { label: '三', value: 3 },
  { label: '四', value: 4 },
  { label: '五', value: 5 },
  { label: '六', value: 6 },
  { label: '日', value: 0 }
]

const ruleSeverityOptions = [
  { label: 'P0 严重', value: 'critical' },
  { label: 'P1 警告', value: 'warning' },
  { label: 'P2 提示', value: 'info' }
]

const groupFormRules = {
  name: [{ required: true, message: '请输入分组名称', trigger: 'blur' }]
}

const yamlImportRules = {
  groupId: [{ required: true, message: '请选择规则分组', trigger: 'change' }],
  datasourceId: [{ required: true, message: '请选择数据源', trigger: 'change' }],
  content: [{ required: true, message: '请粘贴 Prometheus Rule YAML', trigger: 'blur' }]
}

const dialogTitle = computed(() => `${editingId.value ? '修改' : '新增'}${activeTab.value === 'rules' ? '告警规则' : '静默规则'}`)
const selectedGroup = computed(() => groupRows.value.find(item => item.id === selectedGroupId.value))
const selectedGroupName = computed(() => selectedGroup.value?.name || '全部规则')
const previewWarnings = computed(() => Array.isArray(previewResult.warnings) ? previewResult.warnings : [])
const previewRows = computed(() => normalizePreviewRows(previewResult))
const builtinGroupProps = {
  emitPath: true,
  checkStrictly: true
}
const builtinGroupOptions = computed(() => builtinGroupTree.value.map(component => ({
  label: component.label,
  value: component.component,
  children: (component.collectors || []).map(collector => ({
    label: collector.label,
    value: collector.collector,
    children: (collector.categories || []).map(category => ({
      label: category.label,
      value: category.category
    }))
  }))
})))
const apiMap = {
  rules: {
    list: alertApi.getRules,
    create: alertApi.createRule,
    update: alertApi.updateRule,
    delete: alertApi.deleteRule
  },
  silences: {
    list: alertApi.getSilenceRules,
    create: alertApi.createSilenceRule,
    update: alertApi.updateSilenceRule,
    delete: alertApi.deleteSilenceRule
  }
}

const loadGroups = async () => {
  groupLoading.value = true
  try {
    const response = await alertApi.getRuleGroups({ page: 1, pageSize: 500 })
    const result = normalizeListResult(response)
    groupRows.value = result.list
    totals.groups = result.total
  } finally {
    groupLoading.value = false
  }
}

const loadDatasourceOptions = async () => {
  const response = await alertApi.getDatasourceOptions()
  datasourceOptionRows.value = normalizeListResult(response).list
}

const loadNotifyRuleOptions = async () => {
  const response = await alertApi.getNotifyRules({ page: 1, pageSize: 500, enabled: 1 })
  notifyRuleOptions.value = normalizeListResult(response).list
}

const loadBuiltinGroupTree = async () => {
  const response = await alertApi.getBuiltinRuleGroupTree()
  builtinGroupTree.value = normalizeBuiltinGroupTree(response)
}

const loadList = async () => {
  loading.value = true
  try {
    const params = activeTab.value === 'rules'
      ? compactObject({
        page: query.page,
        pageSize: query.pageSize,
        keyword: query.keyword,
        severity: query.severity,
        datasourceId: query.datasourceId,
        enabled: query.enabled,
        groupId: selectedGroupId.value
      })
      : compactObject({
        page: query.page,
        pageSize: query.pageSize,
        keyword: query.keyword,
        enabled: query.enabled
      })
    const response = await apiMap[activeTab.value].list(params)
    const result = normalizeListResult(response)
    rows.value = result.list
    totals[activeTab.value] = result.total
    if (activeTab.value === 'rules') {
      selectedRuleRows.value = []
    }
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  query.page = 1
  loadList()
}

const handleReset = () => {
  Object.assign(query, { page: 1, pageSize: 10, keyword: '', severity: '', datasourceId: '', enabled: '' })
  loadList()
}

const handleTabChange = () => {
  query.page = 1
  rows.value = []
  loadList()
}

const selectGroup = groupId => {
  selectedGroupId.value = groupId
  query.page = 1
  loadList()
}

const handleRuleSelectionChange = selection => {
  selectedRuleRows.value = Array.isArray(selection) ? selection : []
}

const handleMoreCommand = command => {
  const commandHandlers = {
    batchDatasource: openBatchDatasourceDialog,
    batchNotifyPolicy: openBatchNotifyPolicyDialog,
    yamlImport: openYamlImportDialog,
    templateImport: openImportDialog
  }
  commandHandlers[command]?.()
}

const openBatchDatasourceDialog = () => {
  if (!selectedRuleRows.value.length) {
    ElMessage.warning('请先选择告警规则')
    return
  }
  batchDatasourceForm.datasourceId = ''
  batchDatasourceVisible.value = true
}

const submitBatchDatasource = async () => {
  const datasourceId = Number(batchDatasourceForm.datasourceId)
  if (!datasourceId) {
    ElMessage.warning('请选择目标数据源')
    return
  }
  const ruleIds = selectedRuleRows.value.map(row => Number(row?.id)).filter(Boolean)
  if (!ruleIds.length) {
    ElMessage.warning('请先选择告警规则')
    return
  }
  const target = datasourceOptionRows.value.find(item => Number(datasourceOptionId(item)) === datasourceId)
  const targetName = datasourceOptionLabel(target) || datasourceId
  try {
    await ElMessageBox.confirm(`确认将 ${ruleIds.length} 条告警规则的数据源修改为「${targetName}」？`, '批量修改确认', {
      type: 'warning',
      confirmButtonText: '确认修改',
      cancelButtonText: '取消'
    })
  } catch {
    return
  }
  batchDatasourceSaving.value = true
  try {
    const response = await alertApi.batchUpdateRuleDatasource({ ruleIds, datasourceId })
    if (isSuccess(response)) {
      ElMessage.success('批量修改成功')
      batchDatasourceVisible.value = false
      selectedRuleRows.value = []
      await loadList()
    }
  } finally {
    batchDatasourceSaving.value = false
  }
}

const openBatchNotifyPolicyDialog = () => {
  if (!selectedRuleRows.value.length) {
    ElMessage.warning('请先选择告警规则')
    return
  }
  Object.assign(batchNotifyPolicyForm, { mode: 'replace', notifyRuleIds: [] })
  batchNotifyPolicyVisible.value = true
}

const submitBatchNotifyPolicy = async () => {
  const ruleIds = selectedRuleRows.value.map(row => Number(row?.id)).filter(Boolean)
  if (!ruleIds.length) {
    ElMessage.warning('请先选择告警规则')
    return
  }
  const notifyRuleIds = batchNotifyPolicyForm.mode === 'clear'
    ? []
    : [...new Set((batchNotifyPolicyForm.notifyRuleIds || []).map(Number).filter(Boolean))]
  if (batchNotifyPolicyForm.mode === 'replace' && !notifyRuleIds.length) {
    ElMessage.warning('请选择通知策略')
    return
  }
  const policyNames = notifyRuleIds
    .map(id => notifyRuleOptions.value.find(item => Number(notifyRuleId(item)) === id))
    .map(notifyRuleLabel)
    .filter(Boolean)
  const targetText = notifyRuleIds.length ? `「${policyNames.join('、')}」` : '不绑定任何通知策略'
  try {
    await ElMessageBox.confirm(`确认将 ${ruleIds.length} 条告警规则的策略配置修改为${targetText}？`, '批量修改确认', {
      type: 'warning',
      confirmButtonText: '确认修改',
      cancelButtonText: '取消'
    })
  } catch {
    return
  }
  batchNotifyPolicySaving.value = true
  try {
    const response = await alertApi.batchUpdateRuleNotifyRules({ ruleIds, notifyRuleIds })
    if (isSuccess(response)) {
      ElMessage.success('批量修改告警策略成功')
      batchNotifyPolicyVisible.value = false
      selectedRuleRows.value = []
      await loadList()
    }
  } finally {
    batchNotifyPolicySaving.value = false
  }
}

const resetForm = row => {
  Object.keys(form).forEach(key => delete form[key])
  if (activeTab.value === 'rules') {
    const firstQuery = row ? ruleQuerySummary(row) : ''
    const ruleConfig = safeJson(row?.ruleConfig || row?.rule_config, {})
    const ruleQueries = normalizeRuleQueries(ruleConfig?.queries, row, firstQuery)
    const rawNotifyRuleIds = row?.notifyRuleIds ?? row?.notify_rule_ids
    const notifyRuleIds = Array.isArray(rawNotifyRuleIds) ? rawNotifyRuleIds : safeJson(rawNotifyRuleIds, [])
    const datasourceId = row?.datasourceId ?? datasourceOptionId(datasourceOptionRows.value[0]) ?? ''
    const datasource = datasourceOptionRows.value.find(item => String(datasourceOptionId(item)) === String(datasourceId))
    const effectiveTime = safeJson(row?.effectiveTime || row?.effective_time, {})
    const effectivePeriod = Array.isArray(effectiveTime?.periods) ? effectiveTime.periods[0] : {}
    const defaults = {
      name: '',
      groupId: selectedGroupId.value || '',
      note: '',
      datasourceId,
      datasourceType: datasourceOptionType(datasource) || 'builtin',
      query: '',
      ruleQueries: createDefaultRuleQueries(),
      evalIntervalSeconds: DEFAULT_EVAL_INTERVAL_SECONDS,
      forDurationSeconds: DEFAULT_FOR_DURATION_SECONDS,
      forDurationCount: DEFAULT_FOR_DURATION_COUNT,
      recoverDurationSeconds: 0,
      severity: 'warning',
      enabled: 1,
      timeZone: 'Asia/Shanghai',
      effectiveWeekdays: [1, 2, 3, 4, 5, 6, 0],
      effectiveStartTime: '00:00',
      effectiveEndTime: '23:59',
      enableInBg: 0,
      businessId: '',
      noDataPolicy: 'ignore',
      errorPolicy: 'ignore',
      notifyRuleIds: [],
    }
    const overrides = {
      query: row?.query || firstQuery || '',
      ruleQueries,
      severity: normalizeSeverity(ruleQueries[0]?.severity || row?.severity) || 'warning',
      forDurationSeconds: ruleQueries[0]?.forDurationSeconds ?? row?.forDurationSeconds ?? row?.for_duration_seconds ?? DEFAULT_FOR_DURATION_SECONDS,
      forDurationCount: row?.forDurationCount ?? durationCountFromSeconds(ruleQueries[0]?.forDurationSeconds ?? row?.forDurationSeconds ?? row?.for_duration_seconds ?? DEFAULT_FOR_DURATION_SECONDS, row?.evalIntervalSeconds ?? row?.eval_interval_seconds ?? DEFAULT_EVAL_INTERVAL_SECONDS),
      note: row?.note || row?.description || '',
      recoverDurationSeconds: row?.recoverDurationSeconds ?? row?.recover_duration_seconds ?? 0,
      timeZone: row?.timeZone || row?.time_zone || 'Asia/Shanghai',
      effectiveWeekdays: normalizeEffectiveWeekdays(effectiveTime, effectivePeriod),
      effectiveStartTime: effectivePeriod?.startTime || effectivePeriod?.start_time || '00:00',
      effectiveEndTime: effectivePeriod?.endTime || effectivePeriod?.end_time || '23:59',
      enableInBg: row?.enableInBg ?? row?.enable_in_bg ?? 0,
      notifyRuleIds
    }
    Object.assign(form, defaults, row, overrides)
    handleRuleDatasourceChange(form.datasourceId)
  } else {
    const defaults = {
      name: '',
      repeatType: 'once',
      reason: '',
      startTime: '',
      endTime: '',
      enabled: 1,
      matchers: [{ label: '', op: '=', value: '' }],
      originEventId: '',
    }
    const overrides = {
      matchers: normalizeSilenceMatchers(row?.matchers),
      originEventId: safeJson(row?.matchers, {})?.eventId || ''
    }
    Object.assign(form, defaults, row, overrides)
  }
}

const openCreateDialog = () => {
  editingId.value = null
  resetForm()
  dialogVisible.value = true
}

const openEditDialog = row => {
  editingId.value = row.id
  resetForm(row)
  dialogVisible.value = true
}

const openCloneDialog = row => {
  editingId.value = null
  resetForm({
    ...row,
    id: undefined,
    name: `${row?.name || '未命名'} Copy`
  })
  dialogVisible.value = true
}

const handleRuleDatasourceChange = value => {
  const datasource = datasourceOptionRows.value.find(item => String(datasourceOptionId(item)) === String(value))
  if (datasource) form.datasourceType = datasourceOptionType(datasource)
}

const createDefaultRuleQueries = () => ([
  { severity: 'critical', prom_ql: '', forDurationSeconds: DEFAULT_FOR_DURATION_SECONDS },
  { severity: 'warning', prom_ql: '', forDurationSeconds: DEFAULT_FOR_DURATION_SECONDS },
  { severity: 'info', prom_ql: '', forDurationSeconds: DEFAULT_FOR_DURATION_SECONDS }
])

const normalizeRuleQueries = (queries, row, fallbackQuery) => {
  const items = Array.isArray(queries) ? queries : []
  const normalized = items
    .map(item => ({
      severity: normalizeSeverity(item?.severity || item?.level) || 'warning',
      prom_ql: item?.prom_ql || item?.promQl || item?.expr || '',
      forDurationSeconds: item?.forDurationSeconds ?? item?.for_duration_seconds ?? row?.forDurationSeconds ?? row?.for_duration_seconds ?? DEFAULT_FOR_DURATION_SECONDS
    }))
    .filter(item => item.prom_ql || item.severity)
  if (normalized.length) return normalized
  return [{
    severity: normalizeSeverity(row?.severity) || 'warning',
    prom_ql: row?.query || fallbackQuery || '',
    forDurationSeconds: row?.forDurationSeconds ?? row?.for_duration_seconds ?? DEFAULT_FOR_DURATION_SECONDS
  }]
}

const addRuleQuery = () => {
  form.ruleQueries.push({ severity: 'warning', prom_ql: '', forDurationSeconds: formForDurationSeconds() || DEFAULT_FOR_DURATION_SECONDS })
}

const removeRuleQuery = index => {
  form.ruleQueries.splice(index, 1)
}

const formForDurationSeconds = () => {
  const interval = Math.max(1, Number(form.evalIntervalSeconds) || DEFAULT_EVAL_INTERVAL_SECONDS)
  const count = Math.max(0, Number(form.forDurationCount) || 0)
  return interval * count
}

const durationCountFromSeconds = (seconds, interval) => {
  const normalizedSeconds = Math.max(0, Number(seconds) || 0)
  if (!normalizedSeconds) return 0
  const normalizedInterval = Math.max(1, Number(interval) || DEFAULT_EVAL_INTERVAL_SECONDS)
  return Math.ceil(normalizedSeconds / normalizedInterval)
}

const severityPriority = value => {
  const priorityMap = { critical: 0, warning: 1, info: 2 }
  return priorityMap[normalizeSeverity(value)] ?? 99
}

const pickPrimaryQuery = queries => {
  const items = Array.isArray(queries) ? queries : []
  return [...items].sort((left, right) => severityPriority(left?.severity) - severityPriority(right?.severity))[0]
}

const normalizeSubmitWeekdays = value => {
  const days = Array.isArray(value) ? value : []
  return days.map(day => Number(day) === 0 ? 7 : Number(day)).filter(day => day >= 1 && day <= 7)
}

const normalizeEffectiveWeekdays = (effectiveTime, effectivePeriod) => {
  const days = effectiveTime?.week || effectiveTime?.weekdays || effectivePeriod?.weekdays || effectivePeriod?.week
  const normalized = Array.isArray(days) ? days.map(day => Number(day) === 7 ? 0 : Number(day)).filter(day => day >= 0 && day <= 6) : []
  return normalized.length ? normalized : [1, 2, 3, 4, 5, 6, 0]
}

const previewRuleQuery = async item => {
  if (!String(item?.prom_ql || '').trim()) {
    ElMessage.warning('请先填写查询语句')
    return
  }
  if (!form.datasourceId) {
    ElMessage.warning('请先选择数据源')
    return
  }
  previewingQuery.value = item
  try {
    const response = await alertApi.previewRuleQuery(compactObject({
      datasourceId: form.datasourceId,
      datasourceType: form.datasourceType,
      query: String(item.prom_ql).trim(),
      limit: 100
    }))
    Object.keys(previewResult).forEach(key => delete previewResult[key])
    Object.assign(previewResult, unwrapData(response))
    previewDialogVisible.value = true
  } finally {
    previewingQuery.value = null
  }
}

const buildDatasourceQueries = () => {
  return [{
    matchType: 'include',
    datasourceIds: form.datasourceId ? [form.datasourceId] : []
  }]
}

const buildPayload = () => {
  if (activeTab.value === 'rules') {
    const notifyRuleIds = Array.isArray(form.notifyRuleIds) ? form.notifyRuleIds : []
    const ruleConfig = buildRuleConfig()
    const firstQuery = pickPrimaryQuery(ruleConfig.queries) || {}
    return compactObject({
      name: form.name,
      note: form.note,
      groupId: form.groupId,
      datasourceId: form.datasourceId,
      datasourceType: form.datasourceType,
      datasourceQueries: buildDatasourceQueries(),
      ruleConfig,
      query: firstQuery.prom_ql,
      evalIntervalSeconds: form.evalIntervalSeconds,
      forDurationSeconds: formForDurationSeconds(),
      forDurationCount: form.forDurationCount,
      recoverDurationSeconds: form.recoverDurationSeconds,
      severity: normalizeSeverity(firstQuery.severity),
      enabled: form.enabled,
      timeZone: form.timeZone,
      effectiveTime: {
        week: normalizeSubmitWeekdays(form.effectiveWeekdays),
        startTime: form.effectiveStartTime,
        endTime: form.effectiveEndTime
      },
      enableInBg: form.enableInBg,
      notifyRuleIds,
      labels: {},
      businessId: form.businessId,
      noDataPolicy: form.noDataPolicy,
      errorPolicy: form.errorPolicy
    })
  }
  const matchers = {
    items: (Array.isArray(form.matchers) ? form.matchers : []).map(item => ({
      label: String(item.label || '').trim(),
      op: item.op || '=',
      value: String(item.value ?? '')
    }))
  }
  if (form.originEventId) matchers.eventId = form.originEventId
  return {
    name: form.name,
    matchers,
    startTime: form.startTime,
    endTime: form.endTime,
    repeatType: 'once',
    reason: form.reason,
    enabled: form.enabled
  }
}

const buildRuleConfig = () => ({
  inhibit: false,
  queries: (Array.isArray(form.ruleQueries) ? form.ruleQueries : [])
    .filter(item => String(item?.prom_ql || '').trim())
    .map(item => ({
      level: severityCodeLabel(item.severity),
      prom_ql: item.prom_ql,
      severity: normalizeSeverity(item.severity),
      forDurationSeconds: formForDurationSeconds()
    }))
})

const notifyRuleId = item => item?.id ?? item?.value ?? ''

const notifyRuleLabel = item => item?.name || item?.label || notifyRuleId(item)

const notifyRuleNames = row => {
  const rawIDs = row?.notifyRuleIds ?? row?.notify_rule_ids
  const ids = Array.isArray(rawIDs) ? rawIDs : safeJson(rawIDs, [])
  if (!Array.isArray(ids) || !ids.length) return '未配置'
  return ids
    .map(value => Number(value))
    .filter(Number.isFinite)
    .map(id => notifyRuleOptions.value.find(item => Number(notifyRuleId(item)) === id)?.name || `策略 #${id}`)
    .join('、') || '未配置'
}

const submitForm = async () => {
  await formRef.value?.validate()
  saving.value = true
  try {
    const payload = buildPayload()
    const response = editingId.value
      ? await apiMap[activeTab.value].update(editingId.value, payload)
      : await apiMap[activeTab.value].create(payload)
    if (isSuccess(response)) {
      ElMessage.success(editingId.value ? '修改成功' : '创建成功')
      dialogVisible.value = false
      loadList()
    }
  } catch (error) {
    if (error?.message === 'JSON 格式不正确') ElMessage.error(error.message)
  } finally {
    saving.value = false
  }
}

const handleDelete = async row => {
  const isSilence = activeTab.value === 'silences'
  await ElMessageBox.confirm(`${isSilence ? '确认取消静默' : '确认删除'}「${row.name || row.id}」吗？`, isSilence ? '取消静默' : '删除确认', {
    type: 'warning',
    confirmButtonText: isSilence ? '确认取消' : '确认删除',
    cancelButtonText: '取消'
  })
  const response = await apiMap[activeTab.value].delete(row.id)
  if (isSuccess(response)) {
    ElMessage.success(isSilence ? '已取消静默' : '删除成功')
    loadList()
  }
}

const toggleRuleStatus = async (row, value) => {
  statusLoadingId.value = row.id
  try {
    const response = await alertApi.updateRuleStatus(row.id, value ? 1 : 0)
    if (isSuccess(response)) {
      row.enabled = value ? 1 : 0
      ElMessage.success('状态已更新')
    }
  } finally {
    statusLoadingId.value = null
  }
}

const openGroupDialog = row => {
  Object.keys(groupForm).forEach(key => delete groupForm[key])
  groupEditingId.value = row?.id || null
  Object.assign(groupForm, {
    parentId: 0,
    name: '',
    description: '',
    ...(row || {})
  })
  groupDialogVisible.value = true
}

const submitGroupForm = async () => {
  await groupFormRef.value?.validate()
  groupSaving.value = true
  try {
    const payload = {
      parentId: groupForm.parentId || 0,
      name: groupForm.name,
      description: groupForm.description
    }
    const response = groupEditingId.value
      ? await alertApi.updateRuleGroup(groupEditingId.value, payload)
      : await alertApi.createRuleGroup(payload)
    if (isSuccess(response)) {
      ElMessage.success(groupEditingId.value ? '分组已修改' : '分组已创建')
      groupDialogVisible.value = false
      await loadGroups()
    }
  } finally {
    groupSaving.value = false
  }
}

const deleteGroup = async group => {
  const target = group || selectedGroup.value
  if (!target) {
    ElMessage.warning('请先选择一个规则分组')
    return
  }
  await ElMessageBox.confirm(`确认删除分组「${target.name}」吗？`, '删除确认', {
    type: 'warning',
    confirmButtonText: '确认删除',
    cancelButtonText: '取消'
  })
  const response = await alertApi.deleteRuleGroup(target.id)
  if (isSuccess(response)) {
    ElMessage.success('分组已删除')
    if (selectedGroupId.value === target.id) selectedGroupId.value = ''
    await loadGroups()
    loadList()
  }
}

const openImportDialog = async () => {
  Object.assign(importForm, {
    groupId: selectedGroupId.value || '',
    datasourceId: ''
  })
  selectedBuiltinRows.value = []
  importDialogVisible.value = true
  await Promise.all([loadBuiltinGroupTree(), loadBuiltinRules()])
}

const loadBuiltinRules = async () => {
  builtinLoading.value = true
  try {
    const response = await alertApi.getBuiltinRules(compactObject(builtinQuery))
    builtinRows.value = normalizeListResult(response).list
  } finally {
    builtinLoading.value = false
  }
}

const handleBuiltinGroupChange = value => {
  const [component, collector, category] = value || []
  Object.assign(builtinQuery, {
    component: component || '',
    collector: collector || '',
    category: category || ''
  })
  loadBuiltinRules()
}

const importBuiltinRules = async () => {
  if (!importForm.groupId) {
    ElMessage.warning('请选择规则分组')
    return
  }
  if (!importForm.datasourceId) {
    ElMessage.warning('请选择数据源')
    return
  }
  if (!selectedBuiltinRows.value.length) {
    ElMessage.warning('请选择要创建为规则的模板')
    return
  }
  importing.value = true
  try {
    for (const template of selectedBuiltinRows.value) {
      const payload = buildRulePayloadFromTemplate(template)
      await alertApi.createRule(payload)
    }
    ElMessage.success(`已创建 ${selectedBuiltinRows.value.length} 条告警规则`)
    importDialogVisible.value = false
    loadList()
  } finally {
    importing.value = false
  }
}

const openYamlImportDialog = () => {
  const firstDatasource = datasourceOptionRows.value[0]
  Object.assign(yamlImportForm, {
    groupId: selectedGroupId.value || '',
    datasourceId: datasourceOptionId(firstDatasource) || '',
    defaultEnabled: 1,
    content: ''
  })
  yamlImportVisible.value = true
}

const submitYamlImport = async () => {
  await yamlImportFormRef.value?.validate()
  yamlImporting.value = true
  try {
    const datasource = datasourceOptionRows.value.find(item => String(datasourceOptionId(item)) === String(yamlImportForm.datasourceId))
    const response = await alertApi.importPrometheusRules(compactObject({
      groupId: yamlImportForm.groupId,
      datasourceType: datasourceOptionType(datasource),
      datasourceId: yamlImportForm.datasourceId,
      defaultEnabled: yamlImportForm.defaultEnabled,
      content: yamlImportForm.content
    }))
    if (isSuccess(response)) {
      const result = unwrapData(response)
      const imported = result?.imported ?? 0
      const skipped = result?.skipped ?? 0
      ElMessage.success(`导入完成：新增 ${imported} 条，跳过 ${skipped} 条`)
      yamlImportVisible.value = false
      loadList()
    }
  } finally {
    yamlImporting.value = false
  }
}

const buildRulePayloadFromTemplate = template => {
  const variables = safeJson(template.variables, {})
  const threshold = variables.threshold?.default ?? template.defaultConditionValue ?? 0
  const queryText = String(template.queryTemplate || '').replace(/\{\{\s*threshold\s*\}\}/g, threshold)
  const datasource = datasourceOptionRows.value.find(item => String(datasourceOptionId(item)) === String(importForm.datasourceId))
  return compactObject({
    name: template.name,
    groupId: importForm.groupId,
    datasourceId: importForm.datasourceId,
    datasourceType: datasourceOptionType(datasource),
    query: queryText,
    conditionOperator: template.defaultConditionOperator || '>',
    conditionValue: template.defaultConditionValue ?? threshold,
    evalIntervalSeconds: template.defaultEvalIntervalSeconds || DEFAULT_EVAL_INTERVAL_SECONDS,
    forDurationSeconds: template.defaultForDurationSeconds || DEFAULT_FOR_DURATION_SECONDS,
    forDurationCount: durationCountFromSeconds(template.defaultForDurationSeconds || DEFAULT_FOR_DURATION_SECONDS, template.defaultEvalIntervalSeconds || DEFAULT_EVAL_INTERVAL_SECONDS),
    severity: normalizeSeverity(template.defaultSeverity) || 'warning',
    enabled: 1,
    labels: safeJson(template.labels, {}),
    annotations: safeJson(template.annotations, {}),
    noDataPolicy: 'ignore',
    errorPolicy: 'ignore'
  })
}

const ruleQuerySummary = row => {
  const ruleConfig = safeJson(row?.ruleConfig, {})
  const queries = Array.isArray(ruleConfig?.queries) ? ruleConfig.queries : []
  const firstQuery = queries[0] || {}
  return firstQuery.prom_ql || firstQuery.promQl || firstQuery.expr || row?.query || '-'
}

const ruleSeverityList = row => {
  const ruleConfig = safeJson(row?.ruleConfig, {})
  const queries = Array.isArray(ruleConfig?.queries) ? ruleConfig.queries : []
  const values = queries.map(item => normalizeSeverity(item?.severity || item?.level)).filter(Boolean)
  if (row?.severity) values.unshift(normalizeSeverity(row.severity))
  return [...new Set(values)].slice(0, 3).length ? [...new Set(values)].slice(0, 3) : ['warning']
}

const severityCodeLabel = value => {
  const labelMap = {
    critical: 'P0',
    warning: 'P1',
    info: 'P2'
  }
  return labelMap[normalizeSeverity(value)] || value || '-'
}

const datasourceName = row => {
  const target = datasourceOptionRows.value.find(item => String(datasourceOptionId(item)) === String(row?.datasourceId))
  return target?.name || target?.label || row?.datasourceName || row?.datasource_name || row?.datasourceId || '-'
}

const ruleDescription = row => {
  const annotations = safeJson(row?.annotations, {})
  return row?.description || row?.note || annotations.description || annotations.summary || '没有留下任何描述~'
}

const normalizePreviewRows = payload => {
  const result = payload?.result
  if (!Array.isArray(result)) {
    if (Array.isArray(result?.result)) return normalizePreviewRows({ result: result.result })
    if (payload?.resultType === 'scalar' && Array.isArray(result)) return [formatPreviewSample({ metric: {}, value: result })]
    return []
  }
  if (payload?.resultType === 'scalar') return [formatPreviewSample({ metric: {}, value: result })]
  return result.map(formatPreviewSample).filter(Boolean)
}

const formatPreviewSample = sample => {
  const metric = sample?.metric && typeof sample.metric === 'object' ? sample.metric : {}
  const metricName = metric.__name__ || 'value'
  const labels = Object.entries(metric)
    .filter(([key]) => key !== '__name__')
    .map(([key, value]) => ({ key, value }))
  return {
    metricName,
    labels,
    value: previewSampleValue(sample)
  }
}

const previewSampleValue = sample => {
  const value = sample?.value || sample?.values || sample
  if (Array.isArray(value)) {
    const last = Array.isArray(value[0]) ? value[value.length - 1] : value
    return Array.isArray(last) ? String(last[1] ?? last[0] ?? '-') : String(last ?? '-')
  }
  return value === undefined || value === null ? '-' : String(value)
}

const datasourceOptionId = item => item?.id ?? item?.value ?? item?.datasourceId ?? ''

const datasourceOptionType = item => String(item?.type || item?.datasourceType || item?.datasource_type || '').trim().toLowerCase()

const datasourceOptionLabel = item => {
  return item?.name || item?.label || datasourceOptionId(item)
}

const normalizeBuiltinGroupTree = response => {
  const payload = unwrapData(response)
  const list = Array.isArray(payload)
    ? payload
    : (payload?.tree || payload?.list || payload?.items || payload?.children || payload?.components || [])
  if (!Array.isArray(list)) return []
  return list.map(normalizeBuiltinComponentNode).filter(item => item.component)
}

const normalizeBuiltinComponentNode = item => {
  const component = nodeValue(item, ['component', 'value', 'code', 'name'])
  return {
    ...item,
    label: nodeLabel(item, component),
    component,
    collectors: nodeChildren(item, ['collectors', 'children']).map(collector => normalizeBuiltinCollectorNode(collector, component))
  }
}

const normalizeBuiltinCollectorNode = (item, component) => {
  const collector = nodeValue(item, ['collector', 'value', 'code', 'name'])
  return {
    ...item,
    label: nodeLabel(item, collector),
    component,
    collector,
    categories: nodeChildren(item, ['categories', 'children']).map(category => normalizeBuiltinCategoryNode(category, component, collector))
  }
}

const normalizeBuiltinCategoryNode = (item, component, collector) => {
  const category = nodeValue(item, ['category', 'value', 'code', 'name'])
  return {
    ...item,
    label: nodeLabel(item, category),
    component,
    collector,
    category
  }
}

const nodeChildren = (item, keys) => {
  for (const key of keys) {
    if (Array.isArray(item?.[key])) return item[key]
  }
  return []
}

const nodeValue = (item, keys) => {
  for (const key of keys) {
    if (item?.[key] !== undefined && item?.[key] !== null && item?.[key] !== '') return String(item[key])
  }
  return ''
}

const nodeLabel = (item, fallback) => item?.label || item?.name || fallback || '-'

const safeJson = (value, fallback) => {
  if (!value) return fallback
  if (typeof value === 'object') return value
  try {
    return JSON.parse(value)
  } catch (error) {
    return fallback
  }
}

const summarizeMatchers = matchers => {
  const items = parseMatcherItems(matchers)
  if (items.length) return items.map(item => `${item.label || item.name || '-'} ${item.op || item.operator || '='} ${item.value || '-'}`).join('；')
  const parsed = safeJson(matchers, {})
  const direct = Object.entries(parsed || {}).filter(([key]) => key !== 'items' && key !== 'eventId')
  return direct.length ? direct.map(([key, value]) => `${key} = ${value}`).join('；') : '-'
}

const parseMatcherItems = value => {
  try {
    const parsed = typeof value === 'string' ? JSON.parse(value) : value
    return Array.isArray(parsed?.items) ? parsed.items : []
  } catch (error) {
    return []
  }
}

const normalizeSilenceMatchers = value => {
  const items = parseMatcherItems(value)
  if (items.length) {
    return items.map(item => ({
      label: item.label || item.name || '',
      op: item.op || item.operator || '=',
      value: String(item.value ?? '')
    }))
  }
  const parsed = safeJson(value, {})
  const direct = Object.entries(parsed || {}).filter(([key]) => key !== 'items' && key !== 'eventId')
  if (direct.length) return direct.map(([label, matcherValue]) => ({ label, op: '=', value: String(matcherValue ?? '') }))
  return [{ label: '', op: '=', value: '' }]
}

const addSilenceMatcher = () => {
  if (!Array.isArray(form.matchers)) form.matchers = []
  form.matchers.push({ label: '', op: '=', value: '' })
}

const removeSilenceMatcher = index => {
  form.matchers.splice(index, 1)
  if (!form.matchers.length) addSilenceMatcher()
}

const silenceStatus = row => {
  if (Number(row?.enabled) !== 1) return 'canceled'
  const now = Date.now()
  const start = parseSilenceTime(row?.startTime || row?.start_time)
  const end = parseSilenceTime(row?.endTime || row?.end_time)
  if (start && now < start) return 'pending'
  if (end && now > end) return 'expired'
  return 'active'
}

const parseSilenceTime = value => {
  if (!value) return 0
  const timestamp = new Date(String(value).replace(/-/g, '/')).getTime()
  return Number.isFinite(timestamp) ? timestamp : 0
}

const silenceStatusText = row => ({
  active: '生效中',
  pending: '待生效',
  expired: '已过期',
  canceled: '已取消'
})[silenceStatus(row)]

const silenceStatusTagType = row => ({
  active: 'success',
  pending: 'warning',
  expired: 'info',
  canceled: 'info'
})[silenceStatus(row)]

const groupRuleCount = group => {
  const value = group?.ruleCount ?? group?.rule_count ?? group?.rulesCount ?? group?.rules_count ?? group?.count ?? group?.total ?? 0
  const count = Number(value)
  return Number.isFinite(count) ? count : 0
}

const groupRuleCountText = group => groupRuleCount(group)

onMounted(async () => {
  await Promise.all([loadGroups(), loadDatasourceOptions(), loadNotifyRuleOptions()])
  loadList()
})
</script>

<style scoped>
:global(.home-container .el-main:has(.alert-rule-page)) {
  --ops-shell-gutter: 10px;
  --ops-page-padding: 7px;
  --ops-module-gap: 8px;
  padding-top: var(--ops-shell-gutter);
  padding-right: var(--ops-shell-gutter);
  padding-left: var(--ops-shell-gutter);
  overflow-y: hidden;
}

.alert-rule-page {
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
  gap: var(--ops-module-gap, 8px);
  height: calc(100vh - 124px);
  max-height: none;
  padding: var(--ops-page-padding, 7px);
  overflow: hidden;
  border-radius: 0;
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

.rule-header-summary {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 10px;
  color: var(--ops-text-secondary);
}

.rule-header-summary span {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  white-space: nowrap;
}

.rule-header-summary em {
  color: var(--ops-text-muted);
  font-size: 12px;
  font-style: normal;
}

.rule-header-summary strong {
  color: var(--ops-text-heading);
  font-family: Consolas, Monaco, monospace;
  font-size: 16px;
  line-height: 1;
}

.rule-header-summary > i {
  width: 1px;
  height: 18px;
  background: var(--ops-border-soft);
}

.alert-rule-page :deep(.alert-center-card.el-card) {
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

.alert-rule-page :deep(.alert-center-card::before) {
  display: none;
}

.alert-rule-page :deep(.alert-center-card .el-card__body) {
  box-sizing: border-box;
  display: flex;
  min-height: 0;
  height: 100%;
  flex: 1;
  flex-direction: column;
  padding: 8px 10px 10px;
  overflow: hidden;
  border-radius: 0;
}

.alert-rule-page :deep(.alert-tabs .el-tabs__header) {
  margin: 0 0 6px;
}

.alert-rule-page :deep(.alert-tabs .el-tabs__item) {
  height: 34px;
}

.rule-workbench {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: 206px minmax(0, 1fr);
  gap: 8px;
}

.rule-group-panel,
.rule-list-panel {
  min-height: 0;
}

.rule-group-panel {
  display: flex;
  flex-direction: column;
  padding: 10px;
  border-radius: 10px;
  border: 1px solid var(--ops-border-soft);
  background: var(--ops-bg-surface-soft);
}

.rule-group-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 8px;
}

.rule-group-head strong {
  display: block;
  color: var(--aiops-brand-active);
  font-size: 13px;
}

.rule-group-head span {
  display: block;
  margin-top: 2px;
  color: var(--aiops-text-muted);
  font-size: 11px;
}

.rule-icon-button {
  width: 28px;
  height: 28px;
  display: inline-grid;
  place-items: center;
  border: 1px solid #67c23a;
  border-radius: 8px;
  background: #67c23a;
  color: #ffffff;
  font-size: 14px;
  font-weight: 700;
  cursor: pointer;
}

.rule-icon-button:hover {
  border-color: #529b2e;
  background: #529b2e;
}

.rule-group-all,
.rule-group-item {
  width: 100%;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--aiops-text-secondary);
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  align-items: center;
  gap: 6px;
  text-align: left;
  cursor: pointer;
}

.rule-group-all {
  display: flex;
  padding: 6px 8px;
  margin-bottom: 4px;
}

.rule-group-all.active,
.rule-group-item.active {
  border-color: #4c8bf5;
  background: #4c8bf5;
  color: #fff;
  box-shadow: 0 6px 14px rgba(76, 139, 245, 0.2);
}

.rule-group-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
}

.rule-group-item {
  position: relative;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  min-height: 28px;
  margin-bottom: 2px;
  padding: 4px 8px;
}

.rule-group-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rule-group-folder-icon {
  width: 15px;
  height: 15px;
  flex: 0 0 15px;
  object-fit: contain;
}

.rule-group-count {
  min-width: 18px;
  flex: 0 0 auto;
  color: var(--aiops-text-muted);
  font-size: 11px;
  line-height: 16px;
  text-align: right;
}

.rule-group-item.active .rule-group-count {
  color: rgba(255, 255, 255, 0.82);
}

.rule-group-tools {
  display: none;
  position: absolute;
  top: 50%;
  right: 4px;
  transform: translateY(-50%);
  align-items: center;
  gap: 2px;
  padding-left: 6px;
  background: linear-gradient(90deg, rgba(255, 255, 255, 0), #fff 24%);
}

.rule-group-item.active .rule-group-tools {
  background: linear-gradient(90deg, rgba(76, 139, 245, 0), #4c8bf5 24%);
}

.rule-group-item:hover .rule-group-count {
  display: none;
}

.rule-group-item:hover .rule-group-tools {
  display: flex;
}

.rule-group-tools button {
  width: 22px;
  height: 22px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 6px;
  background: transparent;
  padding: 0;
  color: var(--aiops-text-muted);
  cursor: pointer;
}

.rule-group-item.active .rule-group-tools button {
  color: rgba(255, 255, 255, 0.86);
}

.rule-group-tools button:hover {
  background: rgba(76, 139, 245, 0.08);
  color: var(--aiops-brand-active);
}

.rule-group-item.active .rule-group-tools button:hover {
  background: rgba(255, 255, 255, 0.18);
  color: #fff;
}

.rule-group-tools button.danger {
  color: var(--aiops-danger);
}

.rule-group-item.active .rule-group-tools button.danger {
  color: #fff;
}

.rule-group-tools button.danger:hover {
  background: rgba(245, 63, 63, 0.08);
  color: var(--aiops-danger);
}

.rule-list-panel {
  display: flex;
  flex-direction: column;
}

.alert-rule-page .alert-search-section {
  margin-bottom: 8px;
  padding: 8px 10px 2px;
  border: 1px solid var(--ops-border-soft);
  border-radius: 10px;
  background: var(--ops-bg-subtle);
}

.alert-rule-page :deep(.alert-search-form .el-form-item) {
  margin-right: 10px;
  margin-bottom: 6px;
}

.alert-rule-page :deep(.alert-search-form .el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

.alert-rule-page :deep(.alert-search-form .el-input__wrapper),
.alert-rule-page :deep(.alert-search-form .el-select__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 7px;
  background: #ffffff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.alert-rule-page :deep(.alert-search-form .el-input__wrapper:hover),
.alert-rule-page :deep(.alert-search-form .el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.alert-rule-page :deep(.alert-search-form .el-input__wrapper.is-focus),
.alert-rule-page :deep(.alert-search-form .el-select__wrapper.is-focused) {
  box-shadow: 0 0 0 1px #409eff inset;
}

.alert-rule-page :deep(.rule-toolbar-button.el-button) {
  height: 30px;
  min-height: 30px;
  padding: 0 10px;
  border-radius: 8px;
  color: #ffffff;
  font-size: 12px;
  box-shadow: none;
}

.alert-rule-page :deep(.alert-search-button.el-button) {
  border-color: #409eff;
  background: #409eff;
}

.alert-rule-page :deep(.alert-reset-button.el-button),
.alert-rule-page :deep(.rule-more-button.el-button) {
  border-color: #e6a23c;
  background: #e6a23c;
}

.alert-rule-page :deep(.alert-create-button.el-button) {
  border-color: #67c23a;
  background: #67c23a;
}

.alert-rule-page :deep(.rule-toolbar-button .el-icon) {
  font-size: 14px;
}

.alert-rule-page :deep(.el-button:hover),
.alert-rule-page :deep(.el-button:active) {
  transform: none;
}

.alert-rule-page .alert-section-header {
  padding: 0 2px 6px;
  color: var(--ops-text-secondary);
  font-size: 12px;
}

.alert-rule-page .alert-table-shell,
.alert-rule-page :deep(.alert-table-shell .el-table),
.alert-rule-page :deep(.alert-table-shell .el-table__inner-wrapper) {
  border-radius: 8px;
}

.alert-rule-page .alert-table-shell {
  border-color: var(--ops-border-soft);
  background: #ffffff;
}

.alert-rule-page :deep(.alert-table th.el-table__cell) {
  height: 40px;
  padding: 6px 0;
  background: #f3f7fc;
  color: #334155;
  font-size: 12px;
  font-weight: 700;
}

.alert-rule-page :deep(.alert-table td.el-table__cell) {
  height: 50px;
  padding: 6px 0;
}

.alert-rule-page :deep(.alert-table .el-table__row:hover > td.el-table__cell) {
  background: #f7faff;
}

.alert-rule-page .alert-pagination {
  padding: 8px 2px 0;
}
.rule-edit-dialog :deep(.el-dialog) {
  border-radius: 15px;
  overflow: hidden;
  border: 1px solid #d7e6f6;
  background: #ffffff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

.rule-edit-dialog :deep(.el-dialog__header) {
  padding: 16px 22px 14px;
  margin: 0;
  background: #fbfdff;
  border-bottom: 1px solid #e3edf8;
}

.rule-edit-dialog :deep(.el-dialog__title) {
  color: #172133;
  font-size: 16px;
  font-weight: 700;
}

.rule-edit-dialog :deep(.el-dialog__body) {
  max-height: calc(100vh - 148px);
  padding: 12px 14px 8px;
  overflow: auto;
  background: #ffffff;
}

.rule-edit-dialog :deep(.el-dialog__footer) {
  padding: 10px 16px 14px;
  border-top: 1px solid #e3edf8;
  background: #ffffff;
}

.rule-edit-dialog :deep(.el-form-item) {
  margin-bottom: 8px;
}

.rule-edit-dialog :deep(.el-form-item__label) {
  height: 30px;
  line-height: 30px;
  color: #5f6b7c;
  font-size: 12px;
  font-weight: 600;
}

.rule-edit-dialog :deep(.el-form-item__content) {
  min-width: 0;
  line-height: 30px;
}

.rule-edit-dialog :deep(.el-input__wrapper),
.rule-edit-dialog :deep(.el-select__wrapper),
.rule-edit-dialog :deep(.el-input-number .el-input__wrapper),
.rule-edit-dialog :deep(.el-textarea__inner) {
  min-height: 30px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.94);
  box-shadow: none;
  border: 1px solid rgba(214, 223, 235, 0.9);
}

.rule-edit-dialog :deep(.el-input__wrapper:hover),
.rule-edit-dialog :deep(.el-select__wrapper:hover),
.rule-edit-dialog :deep(.el-textarea__inner:hover) {
  border-color: rgba(171, 189, 220, 0.95);
}

.rule-edit-dialog :deep(.el-input__wrapper.is-focus),
.rule-edit-dialog :deep(.el-select__wrapper.is-focused),
.rule-edit-dialog :deep(.el-textarea__inner:focus) {
  border-color: rgba(76, 139, 245, 0.72);
  box-shadow: 0 0 0 3px rgba(76, 139, 245, 0.07);
}

.rule-edit-dialog :deep(.el-input__inner),
.rule-edit-dialog :deep(.el-select__placeholder),
.rule-edit-dialog :deep(.el-select__selected-item),
.rule-edit-dialog :deep(.el-textarea__inner) {
  font-size: 12px;
}

.rule-edit-dialog :deep(.el-input-number) {
  width: 100%;
}

.rule-edit-dialog :deep(.el-input-number__decrease),
.rule-edit-dialog :deep(.el-input-number__increase) {
  width: 26px;
}

.rule-edit-dialog :deep(.el-button) {
  min-height: 30px;
  padding: 6px 11px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
}

.rule-edit-dialog :deep(.el-switch) {
  display: inline-flex;
  align-items: center;
  min-width: 118px;
  height: 30px;
  white-space: nowrap;
}

.rule-edit-dialog :deep(.el-switch__label) {
  display: inline-flex;
  align-items: center;
  height: 20px;
  color: #6b7788;
  white-space: nowrap;
  word-break: keep-all;
  writing-mode: horizontal-tb;
}

.rule-edit-dialog :deep(.el-switch__label span) {
  display: inline-block;
  white-space: nowrap;
  word-break: keep-all;
  writing-mode: horizontal-tb;
}

.rule-edit-sections {
  display: grid;
  gap: 7px;
}

.rule-edit-section {
  padding: 9px 10px 1px;
  border-radius: 10px;
  border: 1px solid #dbe7f4;
  background: #fbfdff;
}

.rule-edit-section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-height: 20px;
  margin-bottom: 7px;
}

.rule-edit-section-head h3 {
  position: relative;
  margin: 0;
  padding-left: 9px;
  color: #172133;
  font-size: 13px;
  line-height: 18px;
  font-weight: 700;
}

.rule-edit-section-head h3::before {
  content: '';
  position: absolute;
  left: 0;
  top: 4px;
  width: 3px;
  height: 11px;
  border-radius: 999px;
  background: #4c8bf5;
}

.rule-edit-section-head span {
  color: #8d97a8;
  font-size: 11px;
}

.rule-basic-grid,
.rule-condition-grid,
.rule-effective-grid,
.rule-notify-grid {
  display: grid;
  column-gap: 10px;
  row-gap: 0;
  min-width: 0;
}

.rule-basic-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.rule-basic-grid .alert-form-full {
  grid-column: 1 / -1;
}

.rule-config-panel {
  padding: 8px 9px 0;
  border-radius: 10px;
  background: rgba(247, 250, 255, 0.68);
  border: 1px solid rgba(229, 235, 244, 0.88);
}

.rule-query-item {
  margin-bottom: 7px !important;
}

.rule-query-item :deep(.el-form-item__content) {
  margin-left: 0 !important;
  min-width: 0;
}

.rule-query-field {
  width: 100%;
  display: grid;
  gap: 6px;
}

.rule-query-title {
  color: #5f6b7c;
  font-size: 12px;
  font-weight: 600;
  line-height: 18px;
}

.rule-query-list {
  width: 100%;
  display: grid;
  gap: 5px;
}

.rule-query-row {
  display: grid;
  padding: 5px 6px;
  border-radius: 8px;
  border: 1px solid rgba(223, 231, 242, 0.82);
  background: rgba(255, 255, 255, 0.9);
}

.rule-query-main {
  display: grid;
  grid-template-columns: 116px minmax(300px, 1fr) 102px;
  align-items: center;
  column-gap: 7px;
  min-width: 0;
}

.rule-query-severity {
  width: 100%;
}

.rule-query-severity.severity-p0 :deep(.el-select__wrapper) {
  border-color: transparent;
  background: rgba(255, 241, 243, 0.72);
}

.rule-query-severity.severity-p1 :deep(.el-select__wrapper) {
  border-color: transparent;
  background: rgba(255, 247, 232, 0.72);
}

.rule-query-severity.severity-p2 :deep(.el-select__wrapper) {
  border-color: transparent;
  background: rgba(239, 245, 255, 0.72);
}

.rule-query-severity :deep(.el-select__placeholder),
.rule-query-severity :deep(.el-select__selected-item) {
  width: 100%;
}

.rule-severity-selected {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-width: 0;
}

.rule-severity-selected span {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 34px;
  height: 20px;
  border-radius: 999px;
  color: #fff;
  font-size: 12px;
  font-weight: 700;
}

.rule-severity-selected em {
  overflow: hidden;
  color: #5f6b7c;
  font-style: normal;
  font-size: 12px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rule-severity-selected.severity-p0 span {
  background: #ef5d6e;
}

.rule-severity-selected.severity-p1 span {
  background: #f2ad3d;
}

.rule-severity-selected.severity-p2 span {
  background: #4c8bf5;
}

.rule-severity-option {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 30px;
  height: 20px;
  margin-right: 8px;
  border-radius: 999px;
  color: #fff;
  font-size: 12px;
  font-weight: 700;
}

.rule-severity-option.severity-p0 {
  background: #ef5d6e;
}

.rule-severity-option.severity-p1 {
  background: #f2ad3d;
}

.rule-severity-option.severity-p2 {
  background: #4c8bf5;
}

.rule-query-actions {
  display: grid;
  grid-template-columns: repeat(3, 30px);
  gap: 6px;
  align-items: center;
}

.rule-query-action,
.rule-query-plus,
.rule-query-remove {
  width: 30px;
  height: 30px;
  min-height: 30px !important;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0 !important;
  border-radius: 8px;
  cursor: pointer;
}

.rule-query-actions :deep(.el-button + .el-button) {
  margin-left: 0;
}

.rule-query-actions :deep(.el-button) {
  margin: 0;
}

.rule-query-preview {
  color: #fff !important;
  background: #f2ad3d;
  border-color: #f2ad3d;
  box-shadow: 0 5px 12px rgba(242, 173, 61, 0.2);
}

.rule-query-preview:hover {
  color: #fff !important;
  background: #dc9630;
  border-color: #dc9630;
}

.rule-preview-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 10px;
}

.rule-preview-meta span {
  padding: 4px 9px;
  border-radius: 999px;
  background: rgba(76, 139, 245, 0.08);
  color: #5f6b7c;
  font-size: 12px;
  font-weight: 600;
}

.rule-preview-warnings {
  margin-bottom: 10px;
  padding: 8px 10px;
  border-radius: 10px;
  background: rgba(242, 173, 61, 0.12);
  color: #a46d1f;
  font-size: 12px;
  line-height: 1.5;
}

.rule-preview-list {
  max-height: 420px;
  overflow: auto;
  border-radius: 10px;
  border: 1px solid rgba(223, 231, 242, 0.9);
  background: #fff;
}

.rule-preview-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) max-content;
  align-items: center;
  gap: 14px;
  min-height: 36px;
  padding: 7px 10px;
  border-bottom: 1px solid rgba(223, 231, 242, 0.72);
}

.rule-preview-row:last-child {
  border-bottom: none;
}

.rule-preview-series {
  min-width: 0;
  overflow: hidden;
  color: #334155;
  font-family: Consolas, Monaco, monospace;
  font-size: 12px;
  line-height: 20px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rule-preview-metric {
  color: #069669;
}

.rule-preview-brace {
  margin: 0 3px;
  color: #64748b;
}

.rule-preview-label {
  display: inline-block;
  margin-right: 4px;
  color: #475569;
}

.rule-preview-label b {
  padding: 1px 4px;
  border-radius: 4px;
  background: #eef0f3;
  color: #1f2937;
}

.rule-preview-value {
  color: #334155;
  font-family: Consolas, Monaco, monospace;
  font-size: 12px;
  text-align: right;
  white-space: nowrap;
}

.rule-query-plus {
  border: 1px solid #35bf7f;
  background: #35bf7f;
  color: #fff;
  box-shadow: 0 5px 12px rgba(53, 191, 127, 0.18);
}

.rule-query-plus:hover {
  border-color: #2ca46d;
  background: #2ca46d;
}

.rule-query-remove {
  border: 1px solid #ef6b73;
  background: #ef6b73;
  color: #fff;
  box-shadow: 0 5px 12px rgba(239, 107, 115, 0.18);
}

.rule-query-remove:hover {
  border-color: #dc5b63;
  background: #dc5b63;
  color: #fff;
}

.rule-query-action :deep(.el-icon),
.rule-query-plus :deep(.el-icon),
.rule-query-remove .el-icon {
  margin: 0;
  font-size: 14px;
}

.rule-query-textarea :deep(.el-input__inner) {
  font-family: Consolas, Monaco, monospace;
  font-size: 12px;
}

.rule-condition-grid {
  grid-template-columns: repeat(3, minmax(0, 1fr));
  padding-top: 7px;
  border-top: 1px solid rgba(223, 231, 242, 0.72);
}

.rule-condition-grid :deep(.el-form-item) {
  margin-bottom: 7px;
}

.rule-effective-grid {
  grid-template-columns: 1fr;
  gap: 7px;
  min-width: 0;
}

.rule-effective-grid :deep(.el-form-item) {
  margin-bottom: 0;
}

.rule-effective-row {
  display: grid;
  align-items: center;
  column-gap: 14px;
  width: 100%;
  min-width: 0;
}

.rule-effective-time-row {
  grid-template-columns: minmax(360px, 1.45fr) minmax(210px, 1fr) minmax(210px, 1fr);
}

.rule-effective-switch-row {
  grid-template-columns: minmax(360px, 1fr) minmax(360px, 1fr);
}

.rule-effective-grid :deep(.el-date-editor.el-input),
.rule-effective-grid :deep(.el-select),
.rule-effective-grid :deep(.el-select__wrapper) {
  width: 100%;
  min-width: 0;
}

.rule-effective-grid :deep(.el-form-item__label) {
  width: auto !important;
  flex: 0 0 auto;
  padding-right: 10px;
  white-space: nowrap;
}

.rule-effective-grid :deep(.el-form-item__content) {
  margin-left: 0 !important;
  min-width: 0;
  white-space: nowrap;
}

.rule-time-item :deep(.el-form-item__label) {
  padding-right: 10px;
}

.rule-effective-switch-row :deep(.el-switch) {
  min-width: 150px;
}

.rule-notify-grid {
  grid-template-columns: minmax(250px, 1fr);
  align-items: center;
  column-gap: 14px;
  width: 100%;
}

.rule-notify-grid :deep(.el-form-item) {
  min-width: 0;
}

.rule-notify-grid :deep(.el-form-item__label) {
  width: auto !important;
  flex: 0 0 auto;
  padding-right: 8px;
  font-size: 12px;
  white-space: nowrap;
}

.rule-notify-grid :deep(.el-form-item__content) {
  margin-left: 0 !important;
  min-width: max-content;
  flex: 1 1 auto;
  white-space: nowrap;
}

.notify-policy-item :deep(.el-form-item__content) {
  min-width: 160px;
}

.rule-weekdays {
  display: flex;
  flex-wrap: nowrap;
  gap: 4px;
  min-width: 0;
}

.rule-weekdays :deep(.el-checkbox-button__inner) {
  min-width: 31px;
  height: 30px;
  padding: 7px 8px;
  border-radius: 8px !important;
  border-left: 1px solid var(--el-border-color) !important;
  background: #eef2f7;
  border-color: #d7deea;
  color: #6b7788;
  font-size: 12px;
  line-height: 14px;
}

.rule-weekdays :deep(.el-checkbox-button.is-checked .el-checkbox-button__inner) {
  background: #409eff;
  border-color: #409eff;
  color: #fff;
  box-shadow: none;
}

.rule-match-tabs :deep(.el-radio-button__inner) {
  padding: 6px 10px;
  font-size: 12px;
}

.rule-list-panel .alert-search-section {
  padding: 8px 10px 2px;
}

.rule-list-panel .alert-search-form {
  flex-wrap: wrap;
  gap: 0 8px;
}

.rule-list-panel .alert-search-form .el-form-item {
  margin-right: 0;
}

.rule-list-panel .alert-filter-input {
  width: 170px;
  flex-basis: 170px;
}

.rule-list-panel .rule-datasource-filter {
  width: 220px;
  min-width: 220px;
  flex: 0 0 220px;
}

.rule-list-panel .rule-datasource-item {
  width: 278px;
  flex: 0 0 276px;
  min-width: 278px;
}

.rule-list-panel .rule-datasource-item :deep(.el-form-item__content) {
  width: 220px;
  flex: 0 0 220px;
  min-width: 220px;
}

.rule-list-panel .rule-datasource-filter :deep(.el-select__wrapper) {
  width: 100%;
}

.rule-list-panel .alert-filter-small {
  width: 120px;
  min-width: 120px;
  flex-basis: 120px;
}

.rule-list-panel .alert-search-form .el-button {
  height: 30px;
  min-height: 30px;
  padding: 0 10px;
  border-radius: 8px;
  font-size: 12px;
}

.rule-more-dropdown {
  display: inline-flex;
  margin-left: 0;
  vertical-align: middle;
}

.rule-more-button :deep(.el-icon) {
  margin-left: 4px;
}

.rule-name-link {
  appearance: none;
  border: none;
  background: transparent;
  padding: 0;
  color: #2f7df6 !important;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.rule-name-link:hover {
  color: #1f63d6 !important;
}

.rule-severity-tags {
  flex-wrap: nowrap;
  gap: 4px;
  white-space: nowrap;
}

.rule-severity-tags :deep(.el-tag) {
  padding: 0 7px;
}

.rule-description-text {
  display: block;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  cursor: default;
}

:global(.rule-description-tooltip) {
  max-width: 520px;
  white-space: normal;
  word-break: break-word;
  line-height: 1.6;
}

.full-select,
.full-date {
  width: 100%;
}

.full-input-number {
  width: 100%;
}

.silence-matcher-list {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.silence-matcher-row {
  display: grid;
  grid-template-columns: minmax(150px, 1fr) 84px minmax(180px, 1.3fr) 32px;
  align-items: center;
  gap: 8px;
}

.silence-matcher-op {
  width: 84px;
}

.silence-matcher-remove {
  width: 32px;
  height: 32px;
}

.silence-matcher-add {
  align-self: flex-start;
}

.import-toolbar {
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.import-toolbar {
  margin-bottom: 10px;
}

.import-keyword-input {
  width: 190px;
  flex: 0 0 190px;
}

.import-group-cascader {
  width: 216px;
  flex: 0 0 216px;
}

.import-target-select {
  width: 190px;
  flex: 0 0 190px;
}

.import-toolbar .alert-search-button {
  flex: 0 0 auto;
  min-width: 54px;
  padding: 7px 11px;
}

.import-table-shell {
  height: 390px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: 10px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  background: rgba(255, 255, 255, 0.82);
}

.import-selected-count {
  margin-right: auto;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  font-weight: 600;
}

:global(.alert-rule-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 32px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: #ffffff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.alert-rule-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.alert-rule-dialog-overlay .el-dialog__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.alert-rule-dialog-overlay .el-dialog__body) {
  box-sizing: border-box;
  max-height: calc(100vh - 160px);
  padding: 16px 18px;
  overflow: auto;
  color: #53657d;
}

:global(.alert-rule-dialog-overlay .rule-edit-dialog .el-dialog__body) {
  max-height: calc(100vh - 148px);
  padding: 12px 14px 8px;
}

:global(.alert-rule-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #ffffff;
}

:global(.alert-rule-dialog-overlay .rule-edit-dialog .el-dialog__footer) {
  padding: 10px 16px 14px;
}

:global(.alert-rule-dialog-overlay .el-dialog__footer .el-button) {
  min-height: 34px;
  border-radius: 8px;
}

:global(.alert-rule-dialog-overlay .el-button:hover),
:global(.alert-rule-dialog-overlay .el-button:active) {
  transform: none;
}

:global(.alert-rule-dialog-overlay .el-dialog:not(.rule-edit-dialog) .el-input__wrapper),
:global(.alert-rule-dialog-overlay .el-dialog:not(.rule-edit-dialog) .el-select__wrapper),
:global(.alert-rule-dialog-overlay .el-dialog:not(.rule-edit-dialog) .el-cascader .el-input__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.alert-rule-dialog-overlay .el-dialog:not(.rule-edit-dialog) .el-textarea__inner) {
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.alert-rule-dialog-overlay .el-dialog:not(.rule-edit-dialog) .el-input__wrapper:hover),
:global(.alert-rule-dialog-overlay .el-dialog:not(.rule-edit-dialog) .el-select__wrapper:hover),
:global(.alert-rule-dialog-overlay .el-dialog:not(.rule-edit-dialog) .el-textarea__inner:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

:global(.alert-rule-dialog-overlay .el-dialog:not(.rule-edit-dialog) .el-input__wrapper.is-focus),
:global(.alert-rule-dialog-overlay .el-dialog:not(.rule-edit-dialog) .el-select__wrapper.is-focused),
:global(.alert-rule-dialog-overlay .el-dialog:not(.rule-edit-dialog) .el-textarea__inner:focus) {
  box-shadow: 0 0 0 1px #409eff inset;
}

:global(.alert-rule-dialog-overlay .import-toolbar .alert-search-button) {
  height: 30px;
  min-height: 30px;
  padding: 0 10px;
  border-color: #409eff;
  border-radius: 8px;
  background: #409eff;
  color: #ffffff;
  font-size: 12px;
  box-shadow: none;
}

@media (max-width: 1200px) {
  .ops-page-header {
    flex-wrap: wrap;
  }

  .rule-header-summary {
    width: 100%;
    padding-left: 54px;
  }
}

@media (max-width: 1024px) {
  .rule-workbench {
    grid-template-columns: 1fr;
  }

  .rule-group-panel {
    max-height: 260px;
  }

  .silence-matcher-row {
    grid-template-columns: 1fr 76px 32px;
  }

  .silence-matcher-row > :nth-child(3) {
    grid-column: 1 / 3;
    grid-row: 2;
  }

  .silence-matcher-remove {
    grid-column: 3;
    grid-row: 1;
  }
}

@media (max-width: 768px) {
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
  }

  .rule-header-summary {
    padding-left: 0;
  }

  .alert-rule-page :deep(.alert-search-form .el-form-item) {
    width: 100%;
    margin-right: 0;
  }

  .alert-rule-page :deep(.alert-search-form .el-form-item:last-child .el-form-item__content) {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
}
</style>
