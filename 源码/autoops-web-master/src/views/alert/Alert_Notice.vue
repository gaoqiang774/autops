<template>
  <div class="alert-center-page alert-notice-page">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/通知.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>告警通知</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>统一管理通知策略、发送媒介和消息模板。</p>
      </div>
      <div class="notice-header-summary" aria-label="告警通知配置统计">
        <span><em>策略</em><strong>{{ totals.rules }}</strong></span>
        <i aria-hidden="true"></i>
        <span><em>媒介</em><strong>{{ totals.channels }}</strong></span>
        <i aria-hidden="true"></i>
        <span><em>模板</em><strong>{{ totals.templates }}</strong></span>
      </div>
    </header>

    <el-card shadow="never" class="alert-center-card">
      <el-tabs v-model="activeTab" class="alert-tabs" @tab-change="handleTabChange">
        <el-tab-pane label="通知策略" name="rules" />
        <el-tab-pane label="媒介配置" name="channels" />
        <el-tab-pane label="通知模板" name="templates" />
      </el-tabs>

      <section class="alert-search-section">
        <el-form :model="query" :inline="true" class="alert-search-form" @submit.prevent>
          <el-form-item label="关键字">
            <el-input v-model="query.keyword" class="alert-filter-input" clearable placeholder="名称" @keyup.enter="handleSearch" />
          </el-form-item>
          <el-form-item v-if="activeTab !== 'rules'" label="类型">
            <el-select v-model="query.type" class="alert-filter-small" clearable placeholder="全部">
              <el-option v-for="item in currentTypeOptions" :key="item.value" :label="item.label" :value="item.value">
                <span class="notice-channel-option">
                  <img v-if="item.icon" :src="item.icon" :alt="item.label" class="notice-channel-icon" />
                  <span>{{ item.label }}</span>
                </span>
              </el-option>
            </el-select>
          </el-form-item>
          <el-form-item label="启用">
            <el-select v-model="query.enabled" class="alert-filter-small" clearable placeholder="全部">
              <el-option v-for="item in enabledOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" size="small" class="alert-search-button notice-toolbar-button" @click="handleSearch">
              <el-icon><Search /></el-icon>
              <span>查询</span>
            </el-button>
            <el-button type="warning" size="small" class="alert-reset-button notice-toolbar-button" @click="handleReset">
              <el-icon><RefreshRight /></el-icon>
              <span>重置</span>
            </el-button>
            <el-button type="success" size="small" class="alert-create-button" @click="openCreateDialog">
              <el-icon><Plus /></el-icon>
              <span>{{ createButtonLabel }}</span>
            </el-button>
          </el-form-item>
        </el-form>
      </section>

      <div class="alert-section-header">
        <span>{{ activeTabDescription }}</span>
      </div>

      <div class="alert-table-shell">
        <el-table class="alert-table" :data="rows" v-loading="loading" empty-text="暂无通知配置">
          <template v-if="activeTab === 'rules'">
            <el-table-column label="策略名称" min-width="180">
              <template #default="{ row }">
                <div class="alert-main-text">{{ row.name || '-' }}</div>
              </template>
            </el-table-column>
            <el-table-column label="匹配条件" min-width="280" show-overflow-tooltip>
              <template #default="{ row }">{{ summarizeMatchers(row.matchers) }}</template>
            </el-table-column>
            <el-table-column label="媒介" min-width="220" show-overflow-tooltip>
              <template #default="{ row }">
                <div class="notice-rule-channel-list">
                  <template v-for="item in ruleChannelSummaries(row.channels)" :key="item.key">
                    <span class="notice-rule-channel-item">
                      <img v-if="item.icon" :src="item.icon" :alt="item.typeLabel" class="notice-channel-icon small" />
                      <span>{{ item.name }}</span>
                    </span>
                  </template>
                  <span v-if="!ruleChannelSummaries(row.channels).length">-</span>
                </div>
              </template>
            </el-table-column>
            <el-table-column label="重复间隔" width="110">
              <template #default="{ row }">{{ formatDuration(row.repeatIntervalSeconds) }}</template>
            </el-table-column>
            <el-table-column label="恢复通知" width="100">
              <template #default="{ row }">
                <el-tag :type="Number(row.recoveryNotify) === 1 ? 'success' : 'info'" effect="light">
                  {{ Number(row.recoveryNotify) === 1 ? '启用' : '关闭' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="启用" width="90">
              <template #default="{ row }">
                <el-switch
                  :model-value="Number(row.enabled) === 1"
                  :loading="isStatusLoading(row)"
                  active-color="#3fb97f"
                  inactive-color="#d2d9e5"
                  :aria-label="`${row.name || '通知策略'}启用状态`"
                  @change="value => toggleNotifyRuleStatus(row, value)"
                />
              </template>
            </el-table-column>
            <el-table-column label="更新时间" width="170">
              <template #default="{ row }">{{ formatDateTime(row.updatedAt || row.createdAt) }}</template>
            </el-table-column>
          </template>

          <template v-else-if="activeTab === 'channels'">
            <el-table-column label="媒介名称" min-width="260">
              <template #default="{ row }">
                <div class="alert-main-text">{{ row.name || '-' }}</div>
              </template>
            </el-table-column>
            <el-table-column label="类型" width="190">
              <template #default="{ row }">
                <el-tag effect="light">
                  <span class="notice-channel-inline">
                    <img v-if="typeIcon(row.type)" :src="typeIcon(row.type)" :alt="typeLabel(row.type)" class="notice-channel-icon small" />
                    <span class="notice-channel-type-text">{{ typeLabel(row.type) }}</span>
                  </span>
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="备注" min-width="420" show-overflow-tooltip>
              <template #default="{ row }">
                <span class="notice-description-text">{{ row.description || '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column label="群聊运维" width="110">
              <template #default="{ row }">
                <span v-if="row.type !== 'feishu_card'">-</span>
                <el-tag v-else :type="channelAIOpsEnabled(row) ? 'success' : 'info'" effect="light">
                  {{ channelAIOpsEnabled(row) ? '启用' : '关闭' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="启用" width="100">
              <template #default="{ row }">
                <el-tag :type="Number(row.enabled) === 1 ? 'success' : 'info'" effect="light">{{ enabledLabel(row.enabled) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="创建时间" width="170">
              <template #default="{ row }">{{ formatDateTime(row.createdAt) }}</template>
            </el-table-column>
          </template>

          <template v-else>
            <el-table-column label="名称" min-width="260">
              <template #default="{ row }">
                <div class="alert-main-text">{{ row.name || '-' }}</div>
              </template>
            </el-table-column>
            <el-table-column label="通知渠道" width="120">
              <template #default="{ row }">
                <el-tag effect="light">
                  <span class="notice-channel-inline">
                    <img
                      v-if="typeIcon(row.channelType || row.channel_type)"
                      :src="typeIcon(row.channelType || row.channel_type)"
                      :alt="typeLabel(row.channelType || row.channel_type)"
                      class="notice-channel-icon small"
                    />
                    <span>{{ typeLabel(row.channelType || row.channel_type) }}</span>
                  </span>
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column label="模板类型" width="120">
              <template #default="{ row }">
                <el-tag effect="plain">{{ templateTypeLabel(row.templateType) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="描述" min-width="300">
              <template #default="{ row }">{{ row.description || '-' }}</template>
            </el-table-column>
            <el-table-column label="启用" width="100">
              <template #default="{ row }">
                <el-tag :type="Number(row.enabled) === 1 ? 'success' : 'info'" effect="light">{{ enabledLabel(row.enabled) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="更新时间" width="170">
              <template #default="{ row }">{{ formatDateTime(row.updatedAt || row.createdAt) }}</template>
            </el-table-column>
            <el-table-column label="操作人" width="120">
              <template #default="{ row }">
                <el-tag size="small" effect="plain">{{ row.operator || row.updatedBy || (row.builtin ? '系统内置' : '未知用户') }}</el-tag>
              </template>
            </el-table-column>
          </template>

          <el-table-column label="操作" width="260" fixed="right">
            <template #default="{ row }">
              <div class="alert-operation-links">
                <button type="button" class="alert-operation-link" @click="openEditDialog(row)">编辑</button>
                <button v-if="activeTab === 'rules'" type="button" class="alert-operation-link" @click="openCloneDialog(row)">复制</button>
                <button v-if="activeTab === 'channels'" type="button" class="alert-operation-link alert-operation-success" @click="openTestDialog(row)">测试</button>
                <button v-if="activeTab === 'templates'" type="button" class="alert-operation-link alert-operation-success" @click="openPreviewDialog(row)">预览</button>
                <button v-if="activeTab === 'rules'" type="button" class="alert-operation-link" @click="openNotifyRecords(row)">通知记录</button>
                <button v-if="!(activeTab === 'templates' && isBuiltinTemplate(row))" type="button" class="alert-operation-link alert-operation-danger" @click="handleDelete(row)">删除</button>
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
          :total="currentTotal"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="loadList"
          @current-change="loadList"
        />
      </div>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" :width="activeTab === 'rules' ? '1000px' : '760px'" class="alert-dialog notice-rule-dialog" modal-class="alert-notice-dialog-overlay" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" :label-width="activeTab === 'rules' ? '106px' : '132px'">
        <div v-if="activeTab === 'rules'" class="notice-rule-form">
          <section class="notice-rule-section">
            <h4>基础信息</h4>
            <div class="notice-basic-grid">
              <el-form-item label="策略名称" prop="name" class="notice-basic-name">
                <el-input v-model="form.name" placeholder="请输入策略名称" />
              </el-form-item>
              <el-form-item label="启用">
                <div class="notice-switch-inline">
                  <span>禁用</span>
                  <el-switch v-model="form.enabled" :active-value="1" :inactive-value="0" />
                  <span>启用</span>
                </div>
              </el-form-item>
            </div>
          </section>

          <section class="notice-rule-section">
            <h4>告警降噪</h4>
            <div class="notice-noise-rows">
              <div class="notice-noise-row notice-noise-repeat-row">
                <el-form-item label="重复通知">
                  <div class="notice-switch-inline">
                    <span>关闭</span>
                    <el-switch v-model="form.repeatNotify" :active-value="1" :inactive-value="0" />
                    <span>开启</span>
                  </div>
                </el-form-item>
                <el-form-item label="重复次数">
                  <div class="notice-repeat-number">
                    <el-input-number v-model="form.maxRepeatTimes" :min="0" :precision="0" controls-position="right" class="notice-repeat-count-input" :disabled="Number(form.repeatNotify) !== 1" />
                    <em>次，0 表示不限</em>
                  </div>
                </el-form-item>
                <el-form-item label="重复间隔">
                  <div class="notice-repeat-number">
                    <el-input-number v-model="form.repeatIntervalValue" :min="1" :precision="0" controls-position="right" class="notice-repeat-seconds-input" :disabled="Number(form.repeatNotify) !== 1" />
                    <em>秒</em>
                  </div>
                </el-form-item>
              </div>
              <div class="notice-noise-row notice-noise-aggregation-row">
                <el-form-item label="聚合通知">
                  <div class="notice-switch-inline">
                    <span>关闭</span>
                    <el-switch v-model="form.aggregationEnabled" :active-value="1" :inactive-value="0" />
                    <span>开启</span>
                  </div>
                </el-form-item>
                <el-form-item label="首次等待(秒)">
                  <el-input-number v-model="form.groupWaitSeconds" :min="0" controls-position="right" class="full-input-number" :disabled="Number(form.aggregationEnabled) !== 1" />
                </el-form-item>
                <el-form-item label="聚合间隔(分)">
                  <el-input-number v-model="form.groupIntervalMinutes" :min="1" controls-position="right" class="full-input-number" :disabled="Number(form.aggregationEnabled) !== 1" />
                </el-form-item>
              </div>
            </div>
          </section>

          <div class="notice-rule-config-list">
            <section v-for="(config, configIndex) in form.ruleConfigs" :key="config.id" class="notice-rule-section notice-rule-config-card">
              <div class="notice-rule-card-head">
                <h4>策略规则</h4>
                <div class="notice-section-actions">
                  <el-button size="small" @click="openMatchPreview">匹配预览</el-button>
                  <el-button v-if="form.ruleConfigs.length > 1" link type="danger" @click="removeRuleConfig(configIndex)">删除规则</el-button>
                </div>
              </div>
              <div class="notice-rule-block-title">触发条件</div>
              <div class="notice-match-simple-grid">
                <el-form-item label="告警等级">
                  <el-checkbox-group v-model="config.severities" class="notice-severity-checks">
                    <el-checkbox v-for="item in severityOptions" :key="item.value" :label="item.value">
                      <span class="notice-severity-badge" :class="`is-${String(item.value).toLowerCase()}`">{{ item.label }}</span>
                    </el-checkbox>
                  </el-checkbox-group>
                </el-form-item>
                <el-form-item label="适用范围">
                  <el-select v-model="config.scopeMode" class="full-select" placeholder="全部告警" @change="handleScopeModeChange(config, $event)">
                    <el-option v-for="item in scopeModeOptions" :key="item.value" :label="item.label" :value="item.value" />
                  </el-select>
                </el-form-item>
                <el-form-item v-if="config.scopeMode === 'group'" label="规则分组">
                  <el-select v-model="config.ruleGroupIds" class="full-select" multiple clearable filterable collapse-tags collapse-tags-tooltip placeholder="请选择规则分组">
                    <el-option v-for="item in ruleGroupOptions" :key="optionId(item, ['ruleGroupId'])" :label="optionLabel(item)" :value="optionId(item, ['ruleGroupId'])" />
                  </el-select>
                </el-form-item>
                <el-form-item v-if="config.scopeMode === 'rule'" label="告警规则">
                  <el-select v-model="config.ruleIds" class="full-select" multiple clearable filterable collapse-tags collapse-tags-tooltip placeholder="请选择告警规则">
                    <el-option v-for="item in ruleOptions" :key="optionId(item, ['ruleId'])" :label="optionLabel(item)" :value="optionId(item, ['ruleId'])" />
                  </el-select>
                </el-form-item>
              </div>

              <el-collapse v-model="config.advancedMatchActive" class="notice-advanced-match-collapse">
                <el-collapse-item :title="advancedMatchTitle(config)" name="advanced">
                  <div class="alert-form-grid">
                    <el-form-item label="数据源类型">
                      <el-select v-model="config.datasourceTypes" class="full-select" multiple clearable collapse-tags collapse-tags-tooltip placeholder="全部类型">
                        <el-option v-for="item in datasourceTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
                      </el-select>
                    </el-form-item>
                    <el-form-item label="数据源">
                      <el-select v-model="config.datasourceIds" class="full-select" multiple clearable filterable collapse-tags collapse-tags-tooltip placeholder="全部数据源">
                        <el-option v-for="item in datasourceOptions" :key="optionId(item, ['datasourceId'])" :label="optionLabel(item)" :value="optionId(item, ['datasourceId'])" />
                      </el-select>
                    </el-form-item>
                    <el-form-item label="业务系统">
                      <el-select v-model="config.businessIds" class="full-select" multiple clearable filterable allow-create default-first-option collapse-tags collapse-tags-tooltip placeholder="输入业务 ID 后回车" />
                    </el-form-item>
                  </div>

                  <div class="notice-rule-subsection">
                    <div class="notice-rule-subtitle">
                      <strong>标签匹配</strong>
                      <el-button size="small" @click="addMatcherRow(config, 'labels')">新增</el-button>
                    </div>
                    <div v-for="(item, index) in config.labels" :key="`label-${config.id}-${index}`" class="notice-matcher-row">
                      <el-input v-model="item.key" placeholder="key" />
                      <el-select v-model="item.op">
                        <el-option v-for="op in matcherOpOptions" :key="op.value" :label="op.label" :value="op.value" />
                      </el-select>
                      <el-input v-model="item.value" :disabled="matcherValueDisabled(item.op)" placeholder="value" />
                      <el-button type="danger" plain @click="removeArrayRow(config.labels, index)">删除</el-button>
                    </div>
                    <div v-if="!config.labels.length" class="notice-empty-line">未配置标签匹配</div>
                  </div>

                  <div class="notice-rule-subsection">
                    <div class="notice-rule-subtitle">
                      <strong>注解匹配</strong>
                      <el-button size="small" @click="addMatcherRow(config, 'annotations')">新增</el-button>
                    </div>
                    <div v-for="(item, index) in config.annotations" :key="`annotation-${config.id}-${index}`" class="notice-matcher-row">
                      <el-input v-model="item.key" placeholder="key" />
                      <el-select v-model="item.op">
                        <el-option v-for="op in matcherOpOptions" :key="op.value" :label="op.label" :value="op.value" />
                      </el-select>
                      <el-input v-model="item.value" :disabled="matcherValueDisabled(item.op)" placeholder="value" />
                      <el-button type="danger" plain @click="removeArrayRow(config.annotations, index)">删除</el-button>
                    </div>
                    <div v-if="!config.annotations.length" class="notice-empty-line">未配置注解匹配</div>
                  </div>

                  <div class="notice-rule-subsection">
                    <div class="notice-rule-subtitle">
                      <strong>生效时间</strong>
                      <el-button size="small" @click="addTimeRange(config)">新增</el-button>
                    </div>
                    <div v-for="(item, index) in config.timeRanges" :key="`time-${config.id}-${index}`" class="notice-time-row">
                      <el-select v-model="item.weekdays" multiple collapse-tags collapse-tags-tooltip placeholder="请选择星期">
                        <el-option v-for="weekday in weekdayOptions" :key="weekday.value" :label="weekday.label" :value="weekday.value" />
                      </el-select>
                      <el-time-picker v-model="item.start" format="HH:mm" value-format="HH:mm" placeholder="开始" />
                      <el-time-picker v-model="item.end" format="HH:mm" value-format="HH:mm" placeholder="结束" />
                      <el-button type="danger" plain @click="removeArrayRow(config.timeRanges, index)">删除</el-button>
                    </div>
                    <div v-if="!config.timeRanges.length" class="notice-empty-line">未限制生效时间</div>
                  </div>
                </el-collapse-item>
              </el-collapse>

              <div class="notice-rule-flow-tip">满足以上条件的告警，将通过以下通知动作发送。</div>
              <div class="notice-rule-subtitle">
                <h4>通知动作</h4>
              </div>
              <div class="notice-action-list">
                <div v-for="(row, index) in config.channelItems" :key="index" class="notice-action-card">
                  <div class="notice-action-card-head">
                    <strong>动作配置</strong>
                    <div class="notice-action-card-tools">
                      <span>启用</span>
                      <el-switch v-model="row.enabled" />
                    </div>
                  </div>
                  <div class="notice-action-card-grid">
                    <el-form-item label="通知媒介">
                      <el-select v-model="row.channelId" class="full-select" filterable placeholder="选择媒介" @change="handleRuleChannelChange(row)">
                        <el-option v-for="item in channelOptions" :key="channelId(item)" :label="channelLabel(item)" :value="channelId(item)" />
                      </el-select>
                    </el-form-item>
                    <el-form-item label="通知模板">
                      <el-select v-model="row.templateId" class="full-select" filterable clearable placeholder="选择模板">
                        <el-option v-for="item in templatesForChannel(row)" :key="templateId(item)" :label="item.name || templateId(item)" :value="normalizeId(templateId(item))" />
                      </el-select>
                    </el-form-item>
                  </div>
                </div>
              </div>
              <div class="notice-action-control-grid">
                <el-form-item label="恢复通知">
                  <div class="notice-switch-inline">
                    <span>关闭</span>
                    <el-switch v-model="form.recoveryNotify" :active-value="1" :inactive-value="0" />
                    <span>开启</span>
                  </div>
                </el-form-item>
              </div>
            </section>
            <el-button class="notice-add-rule-card" @click="addRuleConfig">+ 添加通知动作</el-button>
          </div>
        </div>

        <div v-else-if="activeTab === 'channels'" class="alert-form-grid">
          <el-form-item label="媒介名称" prop="name">
            <el-input v-model="form.name" placeholder="请输入媒介名称" />
          </el-form-item>
          <el-form-item label="媒介类型" prop="type">
            <el-select v-model="form.type" class="full-select" placeholder="请选择媒介类型" @change="handleChannelTypeChange">
              <template #prefix>
                <img v-if="typeIcon(form.type)" :src="typeIcon(form.type)" :alt="typeLabel(form.type)" class="notice-select-prefix-icon" />
              </template>
              <el-option v-for="item in channelTypeOptions" :key="item.value" :label="item.label" :value="item.value">
                <span class="notice-channel-option">
                  <img :src="item.icon" :alt="item.label" class="notice-channel-icon" />
                  <span>{{ item.label }}</span>
                </span>
              </el-option>
            </el-select>
          </el-form-item>
          <el-form-item label="启用状态">
            <el-switch v-model="form.enabled" :active-value="1" :inactive-value="0" active-text="启用" inactive-text="禁用" />
          </el-form-item>
          <el-form-item label="备注" class="alert-form-full">
            <el-input v-model="form.description" placeholder="请输入媒介备注，例如：生产告警专用" />
          </el-form-item>
          <template v-if="form.type === 'feishu_card'">
            <el-form-item label="App ID" prop="appId" class="alert-form-full">
              <el-input v-model="form.appId" clearable placeholder="请输入飞书应用 App ID，例如：cli_xxx" />
            </el-form-item>
            <el-form-item label="App Secret" prop="appSecret" class="alert-form-full">
              <el-input v-model="form.appSecret" clearable show-password placeholder="请输入飞书应用 App Secret" />
            </el-form-item>
            <el-form-item label="Verification Token" prop="verificationToken" class="alert-form-full">
              <el-input v-model="form.verificationToken" clearable show-password placeholder="请输入飞书应用 Verification Token" />
            </el-form-item>
            <el-form-item label="启用 AIOps 群聊">
              <el-switch v-model="form.aiopsEnabled" active-text="启用" inactive-text="关闭" />
            </el-form-item>
            <el-form-item v-if="form.aiopsEnabled" label="必须 @机器人">
              <el-switch v-model="form.aiopsRequireMention" active-text="开启" inactive-text="关闭" />
            </el-form-item>
          </template>
          <template v-else>
            <el-form-item label="Webhook" prop="webhook" class="alert-form-full">
              <el-input
                v-model="form.webhook"
                clearable
                placeholder="请输入机器人 webhook"
              />
            </el-form-item>
            <el-form-item v-if="form.type === 'feishu'" label="连接说明" class="alert-form-full">
              <div class="notice-inline-help">
                飞书机器人使用群机器人 webhook / secret 发送普通文本或 Markdown 消息。
              </div>
            </el-form-item>
          </template>
          <template v-if="form.type !== 'wecom' && form.type !== 'feishu_card'">
            <el-form-item label="加签 Secret" class="alert-form-full">
              <el-input v-model="form.secret" placeholder="可选，加签 secret" show-password />
            </el-form-item>
          </template>
          <el-form-item label="高级配置" class="alert-form-full">
            <el-switch v-model="form.advancedEnabled" active-text="展开" inactive-text="关闭" />
            <span class="notice-advanced-tip">仅在需要 @提醒、超时或重试策略时开启</span>
          </el-form-item>
          <template v-if="form.advancedEnabled">
            <template v-if="form.type === 'dingtalk'">
              <el-form-item label="@功能">
                <el-switch v-model="form.atEnabled" active-text="开启" inactive-text="关闭" />
              </el-form-item>
              <el-form-item label="@所有人">
                <el-switch v-model="form.isAtAll" :disabled="!form.atEnabled" active-text="是" inactive-text="否" />
              </el-form-item>
              <el-form-item label="@手机号" class="alert-form-full">
                <el-select
                  v-model="form.atMobiles"
                  class="full-select"
                  multiple
                  filterable
                  allow-create
                  default-first-option
                  clearable
                  placeholder="输入手机号后回车"
                  :disabled="!form.atEnabled"
                />
              </el-form-item>
              <el-form-item label="@用户ID" class="alert-form-full">
                <el-select
                  v-model="form.atUserIds"
                  class="full-select"
                  multiple
                  filterable
                  allow-create
                  default-first-option
                  clearable
                  placeholder="输入用户ID后回车"
                  :disabled="!form.atEnabled"
                />
              </el-form-item>
            </template>
            <el-form-item label="超时(秒)">
              <el-input-number v-model="form.timeoutSeconds" :min="1" controls-position="right" class="full-input-number" />
            </el-form-item>
            <el-form-item label="重试次数">
              <el-input-number v-model="form.retryTimes" :min="0" controls-position="right" class="full-input-number" />
            </el-form-item>
            <el-form-item label="重试间隔(秒)">
              <el-input-number v-model="form.retryIntervalSeconds" :min="0" controls-position="right" class="full-input-number" />
            </el-form-item>
          </template>
        </div>

        <div v-else class="alert-form-grid">
          <el-form-item label="模板名称" prop="name">
            <el-input v-model="form.name" placeholder="请输入模板名称" />
          </el-form-item>
          <el-form-item label="通知渠道" prop="channelType">
            <el-select v-model="form.channelType" class="full-select" placeholder="请选择通知渠道" @change="handleChannelTypeChange">
              <template #prefix>
                <img v-if="typeIcon(form.channelType)" :src="typeIcon(form.channelType)" :alt="typeLabel(form.channelType)" class="notice-select-prefix-icon" />
              </template>
              <el-option v-for="item in channelTypeOptions" :key="item.value" :label="item.label" :value="item.value">
                <span class="notice-channel-option">
                  <img :src="item.icon" :alt="item.label" class="notice-channel-icon" />
                  <span>{{ item.label }}</span>
                </span>
              </el-option>
            </el-select>
          </el-form-item>
          <el-form-item label="模板类型" prop="templateType">
            <el-select v-model="form.templateType" class="full-select" placeholder="请选择模板类型">
              <el-option v-for="item in templateTypeOptions" :key="item.value" :label="item.label" :value="item.value" />
            </el-select>
          </el-form-item>
          <el-form-item label="启用状态">
            <el-switch v-model="form.enabled" :active-value="1" :inactive-value="0" active-text="启用" inactive-text="禁用" />
          </el-form-item>
          <el-form-item label="描述" class="alert-form-full">
            <el-input v-model="form.description" placeholder="请输入模板描述" />
          </el-form-item>
          <template v-if="isFeishuCardTemplate">
            <el-form-item label="卡片模式" class="alert-form-full">
              <el-radio-group v-model="form.feishuCardMode">
                <el-radio-button label="structured">结构化卡片</el-radio-button>
                <el-radio-button label="json">完整 JSON</el-radio-button>
              </el-radio-group>
            </el-form-item>
            <template v-if="form.feishuCardMode !== 'json'">
              <el-form-item label="卡片标题" class="alert-form-full">
                <el-input v-model="form.cardTitle" placeholder="例如：{{ if .IsRecovered }}【已恢复】{{ else }}【报警中】{{ end }}{{ .RuleName }}" />
              </el-form-item>
              <el-form-item label="标题颜色">
                <el-select v-model="form.cardTitleColor" class="full-select" placeholder="请选择标题颜色">
                  <el-option v-for="item in cardTitleColorOptions" :key="item.value" :label="item.label" :value="item.value" />
                </el-select>
              </el-form-item>
              <el-form-item label="启用 Footer">
                <el-switch v-model="form.cardFooterEnabled" active-text="启用" inactive-text="关闭" />
              </el-form-item>
              <el-form-item label="事件内容" class="alert-form-full">
                <el-input
                  v-model="form.cardEvent"
                  class="notice-template-content"
                  type="textarea"
                  :rows="8"
                  placeholder="请输入卡片正文，支持 Go template 变量"
                />
              </el-form-item>
              <el-form-item v-if="form.cardFooterEnabled" label="Footer" class="alert-form-full">
                <el-input v-model="form.cardFooter" placeholder="例如：AIOps 告警中心" />
              </el-form-item>
            </template>
            <el-form-item v-else label="卡片 JSON" class="alert-form-full">
              <el-input
                v-model="form.templateContent"
                class="notice-template-content"
                type="textarea"
                :rows="12"
                placeholder="请输入完整 interactive card JSON，支持 Go template 变量"
              />
            </el-form-item>
          </template>
          <template v-else>
            <el-form-item label="标题模板" class="alert-form-full">
              <el-input v-model="form.templateTitle" placeholder="例如：{{ if .IsRecovered }}【已恢复】{{ else }}【报警中】{{ end }}{{ .RuleName }}" />
            </el-form-item>
            <el-form-item label="内容模板" class="alert-form-full">
              <el-input v-model="form.templateContent" class="notice-template-content" type="textarea" :rows="9" placeholder="请输入通知内容模板，支持 Go template 变量" />
            </el-form-item>
          </template>
          <el-form-item label="变量提示" class="alert-form-full">
            <div class="notice-template-vars">
              <el-tag v-for="item in (isFeishuCardTemplate ? feishuCardVariableHints : templateVariableHints)" :key="item" effect="plain">{{ item }}</el-tag>
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

    <el-dialog v-model="testDialogVisible" title="测试通知媒介" width="560px" class="alert-dialog" modal-class="alert-notice-dialog-overlay" destroy-on-close>
      <el-form :model="testForm" label-width="96px">
        <el-form-item label="媒介">
          <div class="alert-main-text">{{ testChannel?.name || '-' }}</div>
        </el-form-item>
        <el-form-item label="标题">
          <el-input v-model="testForm.title" placeholder="AIOps 测试通知" />
        </el-form-item>
        <el-form-item label="内容">
          <el-input v-model="testForm.content" type="textarea" :rows="4" placeholder="这是一条测试消息" />
        </el-form-item>
        <el-form-item v-if="testResult" label="结果">
          <el-tag :type="testSucceeded(testResult) ? 'success' : 'danger'" effect="light" class="notice-test-status">
            {{ testSucceeded(testResult) ? '成功' : '失败' }}
          </el-tag>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="alert-dialog-footer">
          <el-button @click="testDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="testing" @click="submitTest">发送测试</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="previewDialogVisible" title="预览通知模板" width="720px" class="alert-dialog" modal-class="alert-notice-dialog-overlay" destroy-on-close>
      <el-form :model="previewForm" label-width="104px">
        <el-form-item label="模板">
          <div class="alert-main-text">{{ previewTemplate?.name || '-' }}</div>
        </el-form-item>
        <el-form-item label="样例数据" class="alert-form-full">
          <el-input v-model="previewForm.dataText" class="notice-template-content" type="textarea" :rows="8" />
        </el-form-item>
        <el-form-item v-if="previewResult" label="渲染结果" class="alert-form-full">
          <pre class="notice-preview-result">{{ formatPreviewResult(previewResult) }}</pre>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="alert-dialog-footer">
          <el-button @click="previewDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="previewing" @click="submitPreview">预览</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="recordDialogVisible" :title="recordDialogTitle" width="1120px" class="alert-dialog notice-record-dialog" modal-class="alert-notice-dialog-overlay" destroy-on-close>
      <div v-loading="recordLoading">
        <div class="notice-record-stats">
          <div><span>总数</span><strong>{{ recordStats.total }}</strong></div>
          <div><span>成功</span><strong class="success">{{ recordStats.successCount }}</strong></div>
          <div><span>失败</span><strong class="failed">{{ recordStats.failedCount }}</strong></div>
        </div>
        <el-table class="notice-record-table" :data="recordTableRows" size="small" empty-text="暂无通知记录">
          <el-table-column label="通知时间" width="160">
            <template #default="{ row }">{{ formatDateTime(row.createdAt || row.sentAt) }}</template>
          </el-table-column>
          <el-table-column label="通知类型" width="128">
            <template #default="{ row }">{{ typeLabel(row.channelType || row.channel_type) }}</template>
          </el-table-column>
          <el-table-column label="通知媒介" min-width="150" show-overflow-tooltip>
            <template #default="{ row }">
              <span class="notice-record-text">{{ row.channelName || row.channel_name || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="通知对象" min-width="170" show-overflow-tooltip>
            <template #default="{ row }">
              <span class="notice-record-text">{{ row.receiver || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="88" align="center">
            <template #default="{ row }">
              <el-tag :type="notifyStatusTagType(row.status)" effect="light" size="small">{{ notifyStatusText(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="Request" width="84" align="center">
            <template #default="{ row }">
              <el-button class="notice-record-detail-button" link size="small" type="primary" @click="showPayload('Request', row.request)">查看详情</el-button>
            </template>
          </el-table-column>
          <el-table-column label="Response" width="84" align="center">
            <template #default="{ row }">
              <el-button class="notice-record-detail-button" link size="small" type="primary" @click="showPayload('Response', row.response)">查看详情</el-button>
            </template>
          </el-table-column>
        </el-table>
        <div class="notice-record-pagination">
          <el-pagination
            v-model:current-page="recordQuery.page"
            v-model:page-size="recordQuery.pageSize"
            :page-sizes="[10, 20, 50, 100]"
            :total="recordTotal"
            layout="total, sizes, prev, pager, next"
            @size-change="handleRecordSizeChange"
            @current-change="loadNotifyRecords"
          />
        </div>
      </div>
    </el-dialog>

    <el-dialog v-model="payloadDialogVisible" :title="payloadDialogTitle" width="720px" class="alert-dialog" modal-class="alert-notice-dialog-overlay" destroy-on-close>
      <pre class="notice-payload-content">{{ payloadDialogContent }}</pre>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, RefreshRight, Search } from '@element-plus/icons-vue'
import alertApi from '@/api/alertmanager'
import {
  compactObject,
  enabledLabel,
  enabledOptions,
  formatDateTime,
  isSuccess,
  normalizeListResult,
  sortNotifyRecordsByTimeDesc
} from './alertCommon'
import dingtalkIcon from '@/assets/image/钉钉01.svg'
import feishuIcon from '@/assets/image/飞书.svg'
import wecomIcon from '@/assets/image/企业微信.svg'
import './alert-pages.css'

const channelTypeOptions = [
  { label: '飞书机器人', value: 'feishu', icon: feishuIcon },
  { label: '飞书应用卡片', value: 'feishu_card', icon: feishuIcon },
  { label: '钉钉机器人', value: 'dingtalk', icon: dingtalkIcon },
  { label: '企业微信机器人', value: 'wecom', icon: wecomIcon }
]

const templateTypeOptions = [
  { label: 'Markdown', value: 'markdown' },
  { label: '文本', value: 'text' },
  { label: '卡片', value: 'card' },
  { label: 'JSON', value: 'json' }
]

const cardTitleColorOptions = [
  { label: '红色', value: 'red' },
  { label: '橙色', value: 'orange' },
  { label: '蓝色', value: 'blue' },
  { label: '绿色', value: 'green' },
  { label: '灰色', value: 'grey' }
]

const severityOptions = [
  { label: 'P0 严重', value: 'P0' },
  { label: 'P1 警告', value: 'P1' },
  { label: 'P2 提示', value: 'P2' }
]

const DEFAULT_REPEAT_INTERVAL_SECONDS = 120
const DEFAULT_MAX_REPEAT_TIMES = 3

const scopeModeOptions = [
  { label: '全部告警', value: 'all' },
  { label: '指定规则分组', value: 'group' },
  { label: '指定告警规则', value: 'rule' }
]

const matcherOpOptions = [
  { label: '等于', value: '=' },
  { label: '不等于', value: '!=' },
  { label: '正则匹配', value: '=~' },
  { label: '正则不匹配', value: '!~' },
  { label: '包含任一', value: 'in' },
  { label: '不包含', value: 'not_in' },
  { label: '存在', value: 'exists' },
  { label: '不存在', value: 'not_exists' }
]

const weekdayOptions = [
  { label: '周日', value: 0 },
  { label: '周一', value: 1 },
  { label: '周二', value: 2 },
  { label: '周三', value: 3 },
  { label: '周四', value: 4 },
  { label: '周五', value: 5 },
  { label: '周六', value: 6 }
]

const templateVariableHints = [
  '{{ .RuleName }}',
  '{{ .Severity }}',
  '{{ .Status }}',
  '{{ .ResourceName }}',
  '{{ .Instance }}',
  '{{ .Value }}',
  '{{ .StartsAt }}',
  '{{ .Duration }}',
  '{{ if .IsRecovered }}...{{ end }}'
]

const feishuCardVariableHints = [
  'Title',
  'TitleColor',
  'Event',
  'Footer',
  '{{ .RuleName }}',
  '{{ .Severity }}',
  '{{ .Status }}',
  '{{ .Instance }}'
]

const route = useRoute()
const router = useRouter()
const noticeTabNames = new Set(['rules', 'channels', 'templates'])
const requestedTab = typeof route.query.tab === 'string' ? route.query.tab : ''
const activeTab = ref(noticeTabNames.has(requestedTab) ? requestedTab : 'rules')
const loading = ref(false)
const statusLoadingIds = ref(new Set())
const saving = ref(false)
const testing = ref(false)
const previewing = ref(false)
const dialogVisible = ref(false)
const testDialogVisible = ref(false)
const previewDialogVisible = ref(false)
const recordDialogVisible = ref(false)
const payloadDialogVisible = ref(false)
const editingId = ref(null)
const rows = ref([])
const channelOptions = ref([])
const templateOptions = ref([])
const ruleTemplateOptions = ref([])
const datasourceOptions = ref([])
const datasourceTypeRows = ref([])
const ruleOptions = ref([])
const ruleGroupOptions = ref([])
const testChannel = ref(null)
const testResult = ref(null)
const previewTemplate = ref(null)
const previewResult = ref(null)
const recordRule = ref(null)
const recordRows = ref([])
const recordTableRows = computed(() => sortNotifyRecordsByTimeDesc(recordRows.value))
const recordTotal = ref(0)
const recordLoading = ref(false)
const payloadDialogTitle = ref('')
const payloadDialogContent = ref('')
const formRef = ref()
const totals = reactive({ rules: 0, channels: 0, templates: 0 })
const query = reactive({ page: 1, pageSize: 10, keyword: '', type: '', enabled: '' })
const form = reactive({})
const testForm = reactive({
  title: 'AIOps 测试通知',
  content: '这是一条测试消息'
})
const previewForm = reactive({
  dataText: ''
})
const recordQuery = reactive({ page: 1, pageSize: 10 })
const recordStats = reactive({ total: 0, successCount: 0, failedCount: 0 })

const requiredChannelWebhook = message => (_rule, value, callback) => {
  if (activeTab.value !== 'channels' || form.type === 'feishu_card' || value) return callback()
  callback(new Error(message))
}

const requiredFeishuCardField = message => (_rule, value, callback) => {
  if (activeTab.value !== 'channels' || form.type !== 'feishu_card' || value) return callback()
  callback(new Error(message))
}

const formRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择媒介类型', trigger: 'change' }],
  channelType: [{ required: true, message: '请选择通知渠道', trigger: 'change' }],
  templateType: [{ required: true, message: '请选择模板类型', trigger: 'change' }],
  webhook: [{ validator: requiredChannelWebhook('请输入机器人 webhook'), trigger: ['blur', 'change'] }],
  appId: [{ validator: requiredFeishuCardField('请输入飞书应用 App ID'), trigger: ['blur', 'change'] }],
  appSecret: [{ validator: requiredFeishuCardField('请输入飞书应用 App Secret'), trigger: ['blur', 'change'] }],
  verificationToken: [{ validator: requiredFeishuCardField('请输入飞书应用 Verification Token'), trigger: ['blur', 'change'] }]
}

const currentTotal = computed(() => totals[activeTab.value] || 0)
const currentTypeOptions = computed(() => channelTypeOptions)
const datasourceTypeOptions = computed(() => datasourceTypeRows.value)
const createButtonLabel = computed(() => activeTab.value === 'rules' ? '新增策略' : activeTab.value === 'channels' ? '新增媒介' : '新增模板')
const isFeishuCardTemplate = computed(() => activeTab.value === 'templates' && form.channelType === 'feishu_card')
const recordDialogTitle = computed(() => `通知记录 - ${recordRule.value?.name || '-'}`)
const activeTabDescription = computed(() => {
  if (activeTab.value === 'rules') return '定义什么告警发给谁，以及重复通知、恢复通知和聚合窗口'
  if (activeTab.value === 'channels') return '配置机器人连接信息；模板在通知策略中选择'
  return '维护通知内容模板，媒介只负责机器人连接，模板负责消息内容'
})
const advancedMatchCount = config => [
  config?.datasourceTypes,
  config?.datasourceIds,
  config?.businessIds,
  config?.labels,
  config?.annotations,
  config?.timeRanges
].reduce((count, item) => count + (arrayValue(item).length ? 1 : 0), 0)
const advancedMatchTitle = config => {
  const count = advancedMatchCount(config)
  return count ? `高级匹配（已配置 ${count} 项）` : '高级匹配'
}
const dialogTitle = computed(() => {
  const action = editingId.value ? '修改' : '新增'
  const target = activeTab.value === 'rules' ? '通知策略' : activeTab.value === 'channels' ? '通知媒介' : '通知模板'
  return `${action}${target}`
})
const channelNameMap = computed(() => new Map(channelOptions.value.map(item => [String(channelId(item)), channelLabel(item)])))
const channelTypeMap = computed(() => new Map(channelOptions.value.map(item => [String(channelId(item)), item?.type || item?.channelType || item?.channel_type || ''])))
const templateChannelTypeMap = computed(() => new Map(ruleTemplateOptions.value.map(item => [String(templateId(item)), item.channelType || item.channel_type || ''])))
const apiMap = {
  rules: {
    list: alertApi.getNotifyRules,
    create: alertApi.createNotifyRule,
    update: alertApi.updateNotifyRule,
    delete: alertApi.deleteNotifyRule
  },
  channels: {
    list: alertApi.getNotifyChannels,
    create: alertApi.createNotifyChannel,
    update: alertApi.updateNotifyChannel,
    delete: alertApi.deleteNotifyChannel
  },
  templates: {
    list: alertApi.getNotifyTemplates,
    create: alertApi.createNotifyTemplate,
    update: alertApi.updateNotifyTemplate,
    delete: alertApi.deleteNotifyTemplate
  }
}

const requestOk = response => isSuccess(response) || (response?.status >= 200 && response.status < 300)

const unwrapData = response => {
  let payload = response ?? {}
  for (let index = 0; index < 3; index += 1) {
    if (!payload || typeof payload !== 'object') return payload
    if ('code' in payload) return payload.data ?? {}
    if ('data' in payload) {
      payload = payload.data
      continue
    }
    break
  }
  return payload
}

const toInt = (value, fallback = 0) => {
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : fallback
}

const normalizeSwitchValue = (value, fallback = 1) => {
  if (value === 0 || value === '0' || value === false) return 0
  if (value === 1 || value === '1' || value === true) return 1
  return fallback
}

const normalizeId = value => {
  const numeric = Number(value)
  return value !== '' && value !== null && value !== undefined && Number.isFinite(numeric) ? numeric : value
}

const arrayValue = value => Array.isArray(value) ? value.filter(item => item !== '' && item !== null && item !== undefined) : []

const idArrayValue = value => arrayValue(value)
  .filter(item => !['all', '全部', '全部规则', '全部分组', '全部数据源'].includes(String(item).trim()))
  .map(normalizeId)

const parseSafe = value => {
  if (!value) return {}
  if (typeof value === 'object') return value
  try {
    return JSON.parse(value)
  } catch (error) {
    return {}
  }
}

const normalizeTemplate = item => ({
  ...item,
  channelType: item.channelType || item.channel_type || item.type || '',
  templateType: item.templateType || item.template_type || '',
  builtin: Boolean(item.builtin || item.isBuiltin || item.is_builtin)
})

const normalizeDatasourceTypeOptions = response => {
  const payload = unwrapData(response)
  const source = Array.isArray(payload)
    ? payload
    : (payload?.types || payload?.options || payload?.list || payload?.items || payload?.records || payload)

  if (Array.isArray(source)) {
    return source.map(item => {
      const value = typeof item === 'string'
        ? item
        : (item?.value || item?.type || item?.datasourceType || item?.datasource_type || item?.code || item?.name)
      const label = typeof item === 'string'
        ? item
        : (item?.label || item?.name || item?.description || value)
      return { label, value }
    }).filter(item => item.value)
  }

  if (source && typeof source === 'object') {
    return Object.entries(source).map(([value, item]) => {
      if (item && typeof item === 'object') {
        return {
          label: item.label || item.name || item.description || value,
          value: item.value || item.type || item.code || value
        }
      }
      return { label: String(item || value), value }
    }).filter(item => item.value)
  }

  return []
}

const loadTemplateOptions = async (params = {}) => {
  const response = await alertApi.getNotifyTemplates({ page: 1, pageSize: 1000, ...params })
  templateOptions.value = normalizeListResult(response).list.map(normalizeTemplate)
}

const templateCache = new Map()

const appendRuleTemplateOptions = list => {
  const map = new Map(ruleTemplateOptions.value.map(item => [String(templateId(item)), item]))
  list.forEach(item => map.set(String(templateId(item)), item))
  ruleTemplateOptions.value = [...map.values()]
}

const loadTemplatesByChannelType = async channelType => {
  if (!channelType) return []
  if (templateCache.has(channelType)) {
    const cachedTemplates = templateCache.get(channelType)
    appendRuleTemplateOptions(cachedTemplates)
    return cachedTemplates
  }
  const response = await alertApi.getNotifyTemplates({
    page: 1,
    pageSize: 1000,
    channelType,
    enabled: 1
  })
  let list = normalizeListResult(response).list.map(normalizeTemplate)
  if (!list.length) {
    const fallbackResponse = await alertApi.getNotifyTemplates({ page: 1, pageSize: 1000, enabled: 1 })
    list = normalizeListResult(fallbackResponse).list
      .map(normalizeTemplate)
      .filter(item => item.channelType === channelType)
  }
  templateCache.set(channelType, list)
  appendRuleTemplateOptions(list)
  return list
}

const resetRuleTemplateOptions = () => {
  ruleTemplateOptions.value = []
}

const loadChannelOptions = async () => {
  const response = await alertApi.getNotifyChannels({ page: 1, pageSize: 1000, enabled: 1 })
  channelOptions.value = normalizeListResult(response).list
}

const loadRuleFormOptions = async () => {
  const [datasourceResponse, datasourceTypeResponse, ruleResponse, groupResponse] = await Promise.all([
    alertApi.getDatasourceOptions(),
    alertApi.getDatasourceTypes(),
    alertApi.getRules({ page: 1, pageSize: 1000 }),
    alertApi.getRuleGroups({ page: 1, pageSize: 1000 })
  ])
  datasourceOptions.value = normalizeListResult(datasourceResponse).list
  datasourceTypeRows.value = normalizeDatasourceTypeOptions(datasourceTypeResponse)
  ruleOptions.value = normalizeListResult(ruleResponse).list
  ruleGroupOptions.value = normalizeListResult(groupResponse).list
}

const loadList = async () => {
  loading.value = true
  try {
    if (activeTab.value === 'templates') {
      const response = await alertApi.getNotifyTemplates(compactObject({
        page: query.page,
        pageSize: query.pageSize,
        keyword: query.keyword,
        channelType: query.type,
        enabled: query.enabled
      }))
      const result = normalizeListResult(response)
      rows.value = result.list.map(normalizeTemplate)
      totals.templates = result.total
      templateOptions.value = rows.value
      return
    }

    const response = await apiMap[activeTab.value].list(compactObject({
      page: query.page,
      pageSize: query.pageSize,
      keyword: query.keyword,
      type: activeTab.value === 'channels' ? query.type : undefined,
      enabled: query.enabled
    }))
    const result = normalizeListResult(response)
    rows.value = result.list
    totals[activeTab.value] = result.total
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  query.page = 1
  loadList()
}

const handleReset = () => {
  Object.assign(query, { page: 1, pageSize: 10, keyword: '', type: '', enabled: '' })
  loadList()
}

const handleTabChange = () => {
  router.replace({ query: { ...route.query, tab: activeTab.value } })
  query.page = 1
  rows.value = []
  loadList()
  if (activeTab.value === 'rules') {
    loadChannelOptions()
    resetRuleTemplateOptions()
    loadRuleFormOptions()
  }
  if (activeTab.value === 'channels') loadTemplateOptions()
}

const channelId = item => item?.id ?? item?.ID ?? item?.channelId ?? item?.value ?? ''

const optionId = (item, aliases = []) => {
  for (const key of ['id', 'ID', 'value', ...aliases]) {
    const value = item?.[key]
    if (value !== undefined && value !== null && value !== '') return value
  }
  return ''
}

const optionLabel = item => item?.name || item?.label || item?.ruleName || item?.ruleGroupName || item?.datasourceName || optionId(item)

const channelLabel = item => {
  const enabledText = Number(item?.enabled ?? 1) === 1 ? '' : '（已禁用）'
  return `${item?.name || item?.label || channelId(item)} / ${typeLabel(item?.type)}${enabledText}`
}

const channelNameById = id => channelNameMap.value.get(String(id)) || id || '-'

const channelOptionById = id => channelOptions.value.find(item => String(channelId(item)) === String(id))

const templateId = item => item?.id ?? item?.ID ?? item?.templateId ?? item?.template_id ?? ''

const isBuiltinTemplate = row => Boolean(row?.builtin || row?.isBuiltin || row?.is_builtin)

const hasChannelAdvancedConfig = config => Boolean(
  config.timeoutSeconds !== undefined ||
  config.timeout_seconds !== undefined ||
  config.retryTimes !== undefined ||
  config.retry_times !== undefined ||
  config.retryIntervalSeconds !== undefined ||
  config.retry_interval_seconds !== undefined ||
  config.at
)

const channelAIOpsEnabled = row => {
  const config = parseSafe(row?.config)
  return Boolean(config.aiopsEnabled ?? config.aiops_enabled)
}

const parseTemplateContent = value => {
  const content = parseSafe(value)
  if (typeof value === 'string' && !content.title && !content.content) {
    return { title: '', content: value }
  }
  const title = content.title || content.Title || ''
  if (!content.content && !content.Content && (content.TitleColor || content.Event || content.Footer)) {
    const rest = { ...content }
    delete rest.title
    delete rest.Title
    return {
      title,
      content: rest
    }
  }
  return {
    title,
    content: content.content || content.Content || content.body || content.Body || ''
  }
}

const parseFeishuCardFields = value => {
  const raw = parseSafe(value)
  const title = raw.title || raw.Title || ''
  const titleColor = raw.TitleColor || raw.titleColor || 'red'
  const event = raw.Event || raw.event || raw.content || raw.Content || ''
  const footer = raw.Footer || raw.footer || ''
  const isStructured = Boolean(raw.TitleColor || raw.Event || raw.Footer || raw.Title || raw.title)
  return {
    mode: isStructured ? 'structured' : 'json',
    title,
    titleColor,
    event: typeof event === 'string' ? event : stringifyTemplateContent(event),
    footer: typeof footer === 'string' ? footer : stringifyTemplateContent(footer),
    footerEnabled: Boolean(footer),
    raw: isStructured ? '' : stringifyTemplateContent(raw)
  }
}

const stringifyTemplateContent = value => {
  if (value === undefined || value === null) return ''
  return typeof value === 'string' ? value : JSON.stringify(value, null, 2)
}

const parseTemplateContentText = value => {
  const text = String(value ?? '')
  if (!text.trim()) return ''
  try {
    return JSON.parse(text)
  } catch (error) {
    return text
  }
}

const buildPreviewData = () => ({
  RuleName: '系统磁盘使用率超过82%',
  Severity: 'P0',
  Status: 'firing',
  ResourceName: 'ceshi-quanxian-3-153',
  Instance: '172.16.3.153',
  Value: '84.22%',
  StartsAt: '2026-06-25 09:30:00',
  Duration: '28分钟',
  IsRecovered: false
})

const extractSeverities = value => {
  const matchers = parseSafe(value)
  if (Array.isArray(matchers.severities)) return matchers.severities
  if (Array.isArray(matchers.severity)) return matchers.severity
  const items = Array.isArray(matchers.items) ? matchers.items : []
  return items
    .filter(item => ['severity', 'severities', 'level', 'priority'].includes(String(item.label || item.key || '').toLowerCase()))
    .flatMap(item => Array.isArray(item.value) ? item.value : [item.value])
    .filter(Boolean)
}

const extractChannelItems = value => {
  const parsed = parseSafe(value)
  const list = Array.isArray(parsed)
    ? parsed
    : (parsed.items || parsed.channels || parsed.ids || [])
  return arrayValue(list).map(item => {
    if (typeof item !== 'object') {
      return {
        channelId: normalizeId(item),
        templateId: '',
        receivers: [],
        enabled: true
      }
    }
    return {
      channelId: normalizeId(item.channelId ?? item.channel_id ?? item.notifyChannelId ?? item.notify_channel_id ?? item.id ?? item.ID ?? ''),
      templateId: normalizeId(item.templateId ?? item.template_id ?? item.notifyTemplateId ?? item.notify_template_id ?? item.template ?? ''),
      receivers: arrayValue(item.receivers),
      enabled: item.enabled !== false && Number(item.enabled ?? 1) !== 0
    }
  }).filter(item => item.channelId !== '')
}

let ruleConfigSeed = 0

const createChannelItem = () => ({
  channelId: '',
  templateId: '',
  receivers: [],
  enabled: true
})

const createRuleConfig = source => {
  const matchers = parseSafe(source?.matchers)
  const existingSeverities = extractSeverities(matchers)
  const channelItems = source ? extractChannelItems(source.channels) : [createChannelItem()]
  const config = {
    id: source?.id || `rule-config-${++ruleConfigSeed}`,
    severities: source ? (existingSeverities.length ? existingSeverities : ['P0', 'P1', 'P2']) : ['P0', 'P1', 'P2'],
    scopeMode: source ? inferScopeMode(matchers) : 'all',
    datasourceTypes: arrayValue(matchers.datasourceTypes || matchers.datasource_types),
    datasourceIds: arrayValue(matchers.datasourceIds || matchers.datasource_ids),
    ruleIds: arrayValue(matchers.ruleIds || matchers.rule_ids),
    ruleGroupIds: arrayValue(matchers.ruleGroupIds || matchers.rule_group_ids),
    businessIds: arrayValue(matchers.businessIds || matchers.business_ids),
    labels: normalizeMatcherRows(matchers.labels),
    annotations: normalizeMatcherRows(matchers.annotations),
    timeRanges: normalizeTimeRanges(matchers.timeRanges || matchers.time_ranges),
    channelItems: channelItems.length ? channelItems : [createChannelItem()],
    advancedMatchActive: []
  }
  if (advancedMatchCount(config)) config.advancedMatchActive = ['advanced']
  return config
}

const extractRuleConfigs = row => {
  const parsedRules = parseSafe(row?.rules || row?.ruleConfigs || row?.rule_configs)
  const rules = Array.isArray(parsedRules) ? parsedRules : []
  if (rules.length) {
    return rules.map(item => createRuleConfig({
      id: item.id,
      matchers: item.matchers,
      channels: item.channels
    }))
  }
  return [createRuleConfig(row)]
}

const normalizeMatcherRows = rows => arrayValue(rows).map(item => ({
  key: item?.key || item?.label || '',
  op: item?.op || '=',
  value: item?.value ?? ''
})).filter(item => item.key || item.value)

const normalizeWeekday = value => {
  const day = toInt(value, value)
  return day === 7 ? 0 : day
}

const normalizeTimeRanges = rows => arrayValue(rows).map(item => ({
  weekdays: arrayValue(item?.weekdays).map(normalizeWeekday),
  start: item?.start || '',
  end: item?.end || ''
})).filter(item => item.weekdays.length || item.start || item.end)

const inferScopeMode = matchers => {
  if (arrayValue(matchers.ruleIds || matchers.rule_ids).length) return 'rule'
  if (arrayValue(matchers.ruleGroupIds || matchers.rule_group_ids).length) return 'group'
  return 'all'
}

const normalizeSeconds = (seconds, fallbackSeconds) => Math.max(1, toInt(seconds, fallbackSeconds))
const secondsToMinutes = (seconds, fallbackMinutes) => Math.max(1, Math.round(toInt(seconds, fallbackMinutes * 60) / 60))

const buildRuleForm = row => {
  const repeatSeconds = row?.repeatIntervalSeconds ?? row?.repeat_interval_seconds ?? DEFAULT_REPEAT_INTERVAL_SECONDS
  const groupWaitSeconds = row?.groupWaitSeconds ?? row?.group_wait_seconds ?? row?.aggregationWindowSeconds ?? row?.aggregation_window_seconds ?? 30
  const groupIntervalSeconds = row?.groupIntervalSeconds ?? row?.group_interval_seconds ?? 300
  return {
    name: row?.name || '',
    ruleConfigs: row ? extractRuleConfigs(row) : [createRuleConfig()],
    repeatIntervalValue: normalizeSeconds(repeatSeconds, DEFAULT_REPEAT_INTERVAL_SECONDS),
    recoveryNotify: normalizeSwitchValue(row?.recoveryNotify ?? row?.recovery_notify, 1),
    repeatNotify: normalizeSwitchValue(row?.repeatNotify ?? row?.repeat_notify, row ? 0 : 1),
    maxRepeatTimes: toInt(row?.maxRepeatTimes ?? row?.max_repeat_times, row ? 0 : DEFAULT_MAX_REPEAT_TIMES),
    aggregationEnabled: normalizeSwitchValue(row?.aggregationEnabled ?? row?.aggregation_enabled, row ? 0 : 1),
    groupWaitSeconds: toInt(groupWaitSeconds, 30),
    groupIntervalMinutes: secondsToMinutes(groupIntervalSeconds, 5),
    priority: toInt(row?.priority, 0),
    enabled: normalizeSwitchValue(row?.enabled, 1)
  }
}

const resetForm = row => {
  Object.keys(form).forEach(key => delete form[key])
  if (activeTab.value === 'rules') {
    Object.assign(form, buildRuleForm(row))
  } else if (activeTab.value === 'channels') {
    const config = parseSafe(row?.config)
    const advancedEnabled = Boolean(row && hasChannelAdvancedConfig(config))
    Object.assign(form, {
      name: row?.name || '',
      type: row?.type || 'dingtalk',
      description: row?.description || '',
      webhook: config.webhook || '',
      appId: config.appId || config.app_id || '',
      appSecret: config.appSecret || config.app_secret || '',
      verificationToken: config.verificationToken || config.verification_token || '',
      aiopsEnabled: Boolean(config.aiopsEnabled ?? config.aiops_enabled),
      aiopsRequireMention: (config.aiopsRequireMention ?? config.aiops_require_mention) !== false,
      secret: config.secret || '',
      advancedEnabled,
      timeoutSeconds: config.timeoutSeconds ?? config.timeout_seconds ?? 10,
      retryTimes: config.retryTimes ?? config.retry_times ?? 1,
      retryIntervalSeconds: config.retryIntervalSeconds ?? config.retry_interval_seconds ?? 1,
      atEnabled: Boolean(config.at?.enabled),
      atMobiles: arrayValue(config.at?.mobiles),
      atUserIds: arrayValue(config.at?.userIds || config.at?.user_ids),
      isAtAll: Boolean(config.at?.isAtAll ?? config.at?.is_at_all),
      enabled: row?.enabled ?? 1
    })
  } else {
    const content = parseTemplateContent(row?.content)
    const cardFields = parseFeishuCardFields(row?.content)
    Object.assign(form, {
      name: row?.name || '',
      channelType: row?.channelType || row?.channel_type || 'feishu',
      templateType: row?.templateType || row?.template_type || 'markdown',
      description: row?.description || '',
      templateTitle: content.title || '',
      templateContent: stringifyTemplateContent(content.content),
      feishuCardMode: cardFields.mode,
      cardTitle: cardFields.title || content.title || '',
      cardTitleColor: cardFields.titleColor,
      cardEvent: cardFields.event,
      cardFooter: cardFields.footer,
      cardFooterEnabled: cardFields.footerEnabled,
      enabled: row?.enabled ?? 1,
      builtin: isBuiltinTemplate(row)
    })
    if ((row?.channelType || row?.channel_type) === 'feishu_card' && cardFields.mode === 'json') {
      form.templateContent = cardFields.raw
    }
  }
}

const handleChannelTypeChange = () => {
  if (activeTab.value === 'templates') {
    if (form.channelType === 'feishu_card') {
      form.templateType = form.templateType === 'json' ? 'json' : 'card'
      form.feishuCardMode = form.feishuCardMode || 'structured'
      form.cardTitleColor = form.cardTitleColor || 'red'
    } else {
      form.feishuCardMode = 'structured'
      if (form.templateType === 'card') form.templateType = 'markdown'
    }
    return
  }

  if (form.type === 'feishu_card') {
    form.webhook = ''
    form.secret = ''
    form.appId = ''
    form.appSecret = ''
    form.verificationToken = ''
    form.aiopsEnabled = false
    form.aiopsRequireMention = true
  } else {
    form.appId = ''
    form.appSecret = ''
    form.verificationToken = ''
    form.aiopsEnabled = false
    form.aiopsRequireMention = true
  }
  if (form.type === 'wecom') form.secret = ''
  if (form.type !== 'dingtalk') {
    form.atEnabled = false
    form.atMobiles = []
    form.atUserIds = []
    form.isAtAll = false
  }
}

const openCreateDialog = () => {
  editingId.value = null
  resetForm()
  if (activeTab.value === 'rules') {
    loadChannelOptions()
    resetRuleTemplateOptions()
    loadRuleFormOptions()
    loadRuleChannelTemplates()
  }
  dialogVisible.value = true
}

const openEditDialog = async row => {
  editingId.value = row.id ?? row.ID
  let detail = row
  if (activeTab.value === 'rules' && editingId.value) {
    try {
      const responseDetail = unwrapData(await alertApi.getNotifyRule(editingId.value))
      detail = responseDetail && Object.keys(responseDetail).length ? responseDetail : row
    } catch (error) {
      detail = row
    }
  }
  if (activeTab.value === 'rules') {
    await loadChannelOptions()
    resetRuleTemplateOptions()
    resetForm(detail)
    await loadRuleChannelTemplates()
    loadRuleFormOptions()
  } else {
    resetForm(detail)
  }
  dialogVisible.value = true
}

const openCloneDialog = async row => {
  const id = row?.id ?? row?.ID
  let detail = row
  if (id) {
    try {
      const responseDetail = unwrapData(await alertApi.getNotifyRule(id))
      detail = responseDetail && Object.keys(responseDetail).length ? responseDetail : row
    } catch (error) {
      detail = row
    }
  }
  editingId.value = null
  await loadChannelOptions()
  resetRuleTemplateOptions()
  resetForm({
    ...detail,
    name: `${detail?.name || '通知策略'}副本`
  })
  await loadRuleChannelTemplates()
  loadRuleFormOptions()
  dialogVisible.value = true
}

const addRuleConfig = () => {
  form.ruleConfigs.push(createRuleConfig())
}

const removeRuleConfig = index => {
  if (form.ruleConfigs.length <= 1) return
  form.ruleConfigs.splice(index, 1)
}

const addMatcherRow = (config, field) => {
  config[field].push({ key: '', op: '=', value: '' })
}

const addTimeRange = config => {
  config.timeRanges.push({ weekdays: [1, 2, 3, 4, 5], start: '09:00', end: '18:00' })
}

const handleScopeModeChange = (config, value) => {
  if (value !== 'group') config.ruleGroupIds = []
  if (value !== 'rule') config.ruleIds = []
}

const removeArrayRow = (list, index) => {
  if (Array.isArray(list)) list.splice(index, 1)
}

const matcherValueDisabled = op => ['exists', 'not_exists'].includes(op)

const channelTypeById = channelIdValue => channelTypeMap.value.get(String(channelIdValue)) || ''

const handleRuleChannelChange = row => {
  row.templateId = ''
  row.receivers = []
  loadTemplatesByChannelType(channelTypeById(row.channelId))
}

const loadRuleChannelTemplates = async () => {
  const items = arrayValue(form.ruleConfigs).flatMap(config => arrayValue(config.channelItems))
  const types = [...new Set(items.map(item => channelTypeById(item.channelId)).filter(Boolean))]
  await Promise.all(types.map(loadTemplatesByChannelType))
}

const validateRuleForm = () => {
  if (activeTab.value !== 'rules') return true

  const ruleConfigs = arrayValue(form.ruleConfigs)
  if (!ruleConfigs.length) {
    ElMessage.error('请至少配置一组策略规则')
    throw new Error('invalid rule configs')
  }

  const allChannelItems = ruleConfigs.flatMap(config => arrayValue(config.channelItems))
  const enabledItems = allChannelItems.filter(item => item.enabled)
  if (Number(form.enabled) === 1 && !enabledItems.length) {
    ElMessage.error('启用状态下至少要有 1 个启用的通知通道')
    throw new Error('invalid rule channel items')
  }

  for (const config of ruleConfigs) {
    const channelItems = arrayValue(config.channelItems)
    if (!channelItems.length) {
      ElMessage.error('每组策略规则至少要配置一个通知动作')
      throw new Error('invalid rule channel items')
    }

    for (const item of channelItems) {
      if (!item.channelId || !item.templateId) {
        ElMessage.error('每个通知动作必须选择媒介和模板')
        throw new Error('invalid rule channel items')
      }
      const channelType = channelTypeMap.value.get(String(item.channelId))
      const templateChannelType = templateChannelTypeMap.value.get(String(item.templateId))
      if (channelType && templateChannelType && channelType !== templateChannelType) {
        ElMessage.error('模板类型必须和媒介类型一致')
        throw new Error('invalid rule template type')
      }
    }

    for (const item of arrayValue(config.timeRanges)) {
      if (!arrayValue(item.weekdays).length || !item.start || !item.end) {
        ElMessage.error('时间段必须选择星期、开始时间和结束时间')
        throw new Error('invalid time range')
      }
    }

    for (const item of [...arrayValue(config.labels), ...arrayValue(config.annotations)]) {
      if ((item.op === '=~' || item.op === '!~') && item.value) {
        try {
          new RegExp(item.value)
        } catch (error) {
          ElMessage.error(`正则表达式不合法：${item.key || item.value}`)
          throw error
        }
      }
    }
  }

  return true
}

const templatesForChannel = row => {
  const channelType = channelTypeById(row?.channelId)
  const selectedTemplateId = row?.templateId
  const matched = channelType
    ? ruleTemplateOptions.value.filter(item => item.channelType === channelType)
    : []
  if (!selectedTemplateId || matched.some(item => String(templateId(item)) === String(selectedTemplateId))) return matched
  const selected = ruleTemplateOptions.value.find(item => String(templateId(item)) === String(selectedTemplateId))
  return selected ? [...matched, selected] : matched
}

const openMatchPreview = async () => {
  try {
    validateRuleForm()
  } catch (error) {
    return
  }
  previewing.value = true
  try {
    const response = await alertApi.previewNotifyRuleMatch(buildRulePayload())
    if (requestOk(response)) {
      showPayload('匹配预览', unwrapData(response))
      ElMessage.success('匹配预览已生成')
    }
  } finally {
    previewing.value = false
  }
}

const buildRuleConfigPayload = config => ({
  matchers: {
    severities: arrayValue(config.severities),
    datasourceTypes: arrayValue(config.datasourceTypes),
    datasourceIds: idArrayValue(config.datasourceIds),
    ruleIds: config.scopeMode === 'rule' ? idArrayValue(config.ruleIds) : [],
    ruleGroupIds: config.scopeMode === 'group' ? idArrayValue(config.ruleGroupIds) : [],
    businessIds: idArrayValue(config.businessIds),
    labels: buildMatcherRows(config.labels),
    annotations: buildMatcherRows(config.annotations),
    timeRanges: buildTimeRanges(config.timeRanges)
  },
  channels: {
    items: arrayValue(config.channelItems).map(item => ({
      channelId: normalizeId(item.channelId),
      templateId: normalizeId(item.templateId),
      receivers: [],
      enabled: Boolean(item.enabled)
    }))
  }
})

const buildRulePayload = (source = form) => {
  const ruleConfigs = arrayValue(source.ruleConfigs).map(buildRuleConfigPayload)
  const firstRuleConfig = ruleConfigs[0] || buildRuleConfigPayload(createRuleConfig())
  const repeatNotify = Number(source.repeatNotify) === 1 ? 1 : 0
  const aggregationEnabled = Number(source.aggregationEnabled) === 1 ? 1 : 0
  const recoveryNotify = Number(source.recoveryNotify) === 1 ? 1 : 0
  const enabled = Number(source.enabled) === 1 ? 1 : 0
  return {
    name: source.name,
    matchers: firstRuleConfig.matchers,
    channels: firstRuleConfig.channels,
    rules: ruleConfigs,
    repeatNotify,
    repeatMode: 'uniform',
    repeatIntervalSeconds: repeatNotify === 1 ? normalizeSeconds(source.repeatIntervalValue, DEFAULT_REPEAT_INTERVAL_SECONDS) : 0,
    repeatIntervals: {},
    maxRepeatTimes: repeatNotify === 1 ? toInt(source.maxRepeatTimes, 0) : 0,
    recoveryNotify,
    aggregationEnabled,
    groupWaitSeconds: aggregationEnabled === 1 ? toInt(source.groupWaitSeconds, 30) : 0,
    groupIntervalSeconds: aggregationEnabled === 1 ? toInt(source.groupIntervalMinutes, 5) * 60 : 0,
    aggregationWindowSeconds: aggregationEnabled === 1 ? toInt(source.groupWaitSeconds, 30) : 0,
    priority: toInt(source.priority, 0),
    enabled
  }
}

const isStatusLoading = row => statusLoadingIds.value.has(row?.id ?? row?.ID)

const toggleNotifyRuleStatus = async (row, value) => {
  const id = row?.id ?? row?.ID
  if (!id || isStatusLoading(row)) return

  statusLoadingIds.value = new Set([...statusLoadingIds.value, id])
  try {
    const detail = unwrapData(await alertApi.getNotifyRule(id))
    const ruleForm = buildRuleForm(detail && Object.keys(detail).length ? detail : row)
    ruleForm.enabled = value ? 1 : 0
    const response = await alertApi.updateNotifyRule(id, buildRulePayload(ruleForm))
    if (!requestOk(response)) {
      ElMessage.error('策略状态更新失败，请稍后重试')
      return
    }
    row.enabled = value ? 1 : 0
    ElMessage.success(value ? '策略已启用' : '策略已关闭')
  } catch (error) {
    ElMessage.error('策略状态更新失败，请稍后重试')
  } finally {
    const nextLoadingIds = new Set(statusLoadingIds.value)
    nextLoadingIds.delete(id)
    statusLoadingIds.value = nextLoadingIds
  }
}

const buildMatcherRows = rows => arrayValue(rows)
  .map(item => compactObject({
    key: item.key,
    op: item.op || '=',
    value: matcherValueDisabled(item.op) ? undefined : item.value
  }))
  .filter(item => item.key)

const buildTimeRanges = rows => arrayValue(rows)
  .map(item => compactObject({
    weekdays: arrayValue(item.weekdays).map(normalizeWeekday),
    start: item.start,
    end: item.end
  }))
  .filter(item => arrayValue(item.weekdays).length && item.start && item.end)

const buildChannelConfig = () => {
  if (form.type === 'feishu_card') {
    const feishuCardConfig = compactObject({
      appId: form.appId,
      appSecret: form.appSecret,
      verificationToken: form.verificationToken,
      aiopsEnabled: Boolean(form.aiopsEnabled),
      aiopsRequireMention: form.aiopsRequireMention !== false
    })

    if (form.advancedEnabled) {
      feishuCardConfig.timeoutSeconds = toInt(form.timeoutSeconds, 10)
      feishuCardConfig.retryTimes = toInt(form.retryTimes, 1)
      feishuCardConfig.retryIntervalSeconds = toInt(form.retryIntervalSeconds, 1)
    }

    return feishuCardConfig
  }

  const config = compactObject({
    webhook: form.webhook,
    secret: form.type === 'wecom' ? undefined : form.secret
  })

  if (form.advancedEnabled) {
    config.timeoutSeconds = toInt(form.timeoutSeconds, 10)
    config.retryTimes = toInt(form.retryTimes, 1)
    config.retryIntervalSeconds = toInt(form.retryIntervalSeconds, 1)

    if (form.type === 'dingtalk') {
      config.at = {
        enabled: Boolean(form.atEnabled),
        mobiles: arrayValue(form.atMobiles),
        userIds: arrayValue(form.atUserIds),
        isAtAll: Boolean(form.isAtAll)
      }
    }
  }

  return config
}

const buildTemplatePayload = () => ({
  name: form.name,
  channelType: form.channelType,
  templateType: form.templateType,
  description: form.description,
  content: buildTemplateContentPayload(),
  enabled: form.enabled
})

const buildTemplateContentPayload = () => {
  if (form.channelType === 'feishu_card') {
    if (form.feishuCardMode === 'json') {
      return parseTemplateContentText(form.templateContent)
    }
    return compactObject({
      Title: form.cardTitle || '',
      TitleColor: form.cardTitleColor || 'red',
      Event: form.cardEvent || '',
      Footer: form.cardFooterEnabled ? (form.cardFooter || '') : undefined
    })
  }

  const body = parseTemplateContentText(form.templateContent)
  if (body && typeof body === 'object' && !Array.isArray(body)) {
    return {
      title: form.templateTitle || body.title || body.Title || '',
      ...body
    }
  }
  return {
    title: form.templateTitle || '',
    content: body
  }
}

const buildPayload = () => {
  if (activeTab.value === 'rules') return buildRulePayload()
  if (activeTab.value === 'templates') return buildTemplatePayload()
  return {
    name: form.name,
    type: form.type,
    description: form.description,
    config: buildChannelConfig(),
    enabled: form.enabled
  }
}

const submitForm = async () => {
  await formRef.value?.validate()
  validateRuleForm()
  const isEditing = Boolean(editingId.value)
  saving.value = true
  try {
    const payload = buildPayload()
    const response = isEditing
      ? await apiMap[activeTab.value].update(editingId.value, payload)
      : await apiMap[activeTab.value].create(payload)
    if (requestOk(response)) {
      ElMessage.success(isEditing ? '修改成功' : '创建成功')
      dialogVisible.value = false
      await loadList()
      if (activeTab.value === 'channels') {
        await loadChannelOptions()
        const saved = unwrapData(response)
        const savedId = saved?.id ?? saved?.ID
        if (!isEditing && savedId) openTestDialog({ ...payload, id: savedId })
      }
      if (activeTab.value === 'templates') {
        await loadTemplateOptions()
      }
    }
  } finally {
    saving.value = false
  }
}

const handleDelete = async row => {
  if (activeTab.value === 'templates' && isBuiltinTemplate(row)) {
    ElMessage.warning('内置模板允许编辑，但不允许删除')
    return
  }
  await ElMessageBox.confirm(`确认删除「${row.name || row.id}」吗？`, '删除确认', {
    type: 'warning',
    confirmButtonText: '确认删除',
    cancelButtonText: '取消'
  })
  const response = await apiMap[activeTab.value].delete(row.id ?? row.ID)
  if (requestOk(response)) {
    ElMessage.success('删除成功')
    await loadList()
    if (activeTab.value === 'channels') await loadChannelOptions()
    if (activeTab.value === 'templates') await loadTemplateOptions()
  }
}

const openNotifyRecords = async row => {
  const notifyRuleId = row?.id ?? row?.ID ?? row?.notifyRuleId ?? row?.notify_rule_id
  if (!notifyRuleId) {
    ElMessage.warning('当前策略缺少策略 ID，无法查看通知记录')
    return
  }
  recordRule.value = row
  recordQuery.page = 1
  recordDialogVisible.value = true
  await loadNotifyRecords()
}

const loadNotifyRecords = async () => {
  const notifyRuleId = recordRule.value?.id ?? recordRule.value?.ID ?? recordRule.value?.notifyRuleId ?? recordRule.value?.notify_rule_id
  if (!notifyRuleId) return
  recordLoading.value = true
  try {
    const params = { notifyRuleId, page: recordQuery.page, pageSize: recordQuery.pageSize }
    const [recordsResponse, statsResponse] = await Promise.all([
      alertApi.getNotifyRecords(params),
      alertApi.getNotifyRecordStats({ notifyRuleId })
    ])
    const result = normalizeListResult(recordsResponse)
    recordRows.value = result.list
    recordTotal.value = result.total
    Object.assign(recordStats, normalizeNotifyStats(unwrapStats(statsResponse)))
  } finally {
    recordLoading.value = false
  }
}

const handleRecordSizeChange = () => {
  recordQuery.page = 1
  loadNotifyRecords()
}

const unwrapStats = response => {
  const payload = unwrapData(response)
  if (payload && typeof payload === 'object' && ('total' in payload || 'successCount' in payload || 'failedCount' in payload)) return payload
  const data = response?.data
  if (data && typeof data === 'object' && ('total' in data || 'successCount' in data || 'failedCount' in data)) return data
  return payload
}

const normalizeNotifyStats = value => {
  const stats = value && typeof value === 'object' ? value : {}
  return {
    total: Number(stats.total || 0),
    successCount: Number(stats.successCount || stats.success_count || 0),
    failedCount: Number(stats.failedCount || stats.failed_count || 0)
  }
}

const notifyStatusText = status => {
  const value = String(status || '').toLowerCase()
  if (value === 'success') return '成功'
  if (value === 'failed' || value === 'fail') return '失败'
  return status || '-'
}

const notifyStatusTagType = status => {
  const value = String(status || '').toLowerCase()
  if (value === 'success') return 'success'
  if (value === 'failed' || value === 'fail') return 'danger'
  return 'info'
}

const showPayload = (title, value) => {
  payloadDialogTitle.value = `${title} 详情`
  payloadDialogContent.value = formatPayload(value)
  payloadDialogVisible.value = true
}

const formatPayload = value => {
  if (value === undefined || value === null || value === '') return '-'
  if (typeof value === 'object') return JSON.stringify(value, null, 2)
  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch (error) {
    return String(value)
  }
}

const openTestDialog = row => {
  testChannel.value = row
  testResult.value = null
  Object.assign(testForm, {
    title: 'AIOps 测试通知',
    content: '这是一条测试消息'
  })
  testDialogVisible.value = true
}

const submitTest = async () => {
  const id = testChannel.value?.id ?? testChannel.value?.ID
  if (!id) {
    ElMessage.error('缺少媒介 ID，无法发送测试')
    return
  }
  testing.value = true
  try {
    const payload = compactObject({
      title: testForm.title || 'AIOps 测试通知',
      content: testForm.content || '这是一条测试消息'
    })
    const response = await alertApi.testNotifyChannel(id, payload)
    const result = unwrapData(response)
    testResult.value = result
    if (requestOk(response) && testSucceeded(result)) {
      ElMessage.success('测试通知已发送')
      testDialogVisible.value = false
      testResult.value = null
    } else {
      ElMessage.error(result.errorMessage || '测试通知失败')
    }
  } finally {
    testing.value = false
  }
}

const openPreviewDialog = row => {
  previewTemplate.value = row
  previewResult.value = null
  previewForm.dataText = JSON.stringify(buildPreviewData(), null, 2)
  previewDialogVisible.value = true
}

const testSucceeded = result => result?.status === 'success' || result?.success === true

const submitPreview = async () => {
  const id = templateId(previewTemplate.value)
  if (!id) {
    ElMessage.error('缺少模板 ID，无法预览')
    return
  }

  previewing.value = true
  try {
    const response = await alertApi.previewNotifyTemplate(id, {
      data: parseTemplatePreviewData(previewForm.dataText)
    })
    previewResult.value = unwrapData(response)
    if (requestOk(response)) ElMessage.success('模板预览已生成')
  } finally {
    previewing.value = false
  }
}

const parseTemplatePreviewData = value => {
  try {
    return JSON.parse(value || '{}')
  } catch (error) {
    ElMessage.error('样例数据 JSON 格式不正确')
    throw error
  }
}

const formatPreviewResult = value => typeof value === 'string' ? value : JSON.stringify(value, null, 2)

const summarizeMatchers = value => {
  const matchers = parseSafe(value)
  const severities = extractSeverities(matchers)
  const parts = []
  if (severities.length) parts.push(severities.join('/'))
  if (Array.isArray(matchers.datasourceTypes) && matchers.datasourceTypes.length) parts.push(matchers.datasourceTypes.join('/'))
  if (Array.isArray(matchers.datasourceIds) && matchers.datasourceIds.length) parts.push(`数据源 ${matchers.datasourceIds.length} 个`)
  if (Array.isArray(matchers.ruleIds) && matchers.ruleIds.length) parts.push(`规则 ${matchers.ruleIds.length} 个`)
  if (Array.isArray(matchers.ruleGroupIds) && matchers.ruleGroupIds.length) parts.push(`规则组 ${matchers.ruleGroupIds.length} 个`)
  if (Array.isArray(matchers.businessIds) && matchers.businessIds.length) parts.push(`业务 ${matchers.businessIds.length} 个`)
  if (Array.isArray(matchers.labels) && matchers.labels.length) {
    parts.push(matchers.labels.slice(0, 2).map(formatMatcherSummary).join(' / '))
  }
  if (Array.isArray(matchers.timeRanges) && matchers.timeRanges.length) {
    parts.push(matchers.timeRanges.slice(0, 1).map(formatTimeRangeSummary).join(' / '))
  }
  if (!parts.length && Array.isArray(matchers.items) && matchers.items.length) {
    parts.push(matchers.items.map(item => `${item.label || item.key || '-'} ${item.op || '='} ${item.value || '-'}`).join('；'))
  }
  return parts.join(' · ') || '全部告警'
}

const formatMatcherSummary = item => {
  const op = item.op || '='
  if (matcherValueDisabled(op)) return `${item.key || '-'} ${op}`
  return `${item.key || '-'}${op}${item.value || '-'}`
}

const formatTimeRangeSummary = item => {
  const weekdays = arrayValue(item.weekdays)
  const weekdayText = weekdays.length === 5 && [1, 2, 3, 4, 5].every(day => weekdays.includes(day))
    ? '工作日'
    : weekdays.map(day => weekdayOptions.find(item => item.value === day)?.label || day).join('/')
  return `${weekdayText || '时间'} ${item.start || '--:--'}-${item.end || '--:--'}`
}

const ruleChannelSummaries = value => {
  const parsed = parseSafe(value)
  const list = Array.isArray(parsed)
    ? parsed
    : (parsed.channels || parsed.items || parsed.ids || [])
  return arrayValue(list).map((item, index) => {
    const id = typeof item === 'object' ? (item.channelId ?? item.channel_id ?? item.id ?? item.ID) : item
    const option = channelOptionById(id)
    const type = item?.channelType || item?.channel_type || item?.type || option?.type || ''
    return {
      key: `${id || index}-${type}`,
      name: item?.channelName || item?.channel_name || item?.name || option?.name || channelNameById(id),
      type,
      typeLabel: typeLabel(type),
      icon: typeIcon(type)
    }
  }).filter(item => item.name && item.name !== '-')
}

const formatDuration = seconds => {
  const value = toInt(seconds, 0)
  if (!value) return '-'
  if (value < 60) return `${value}s`
  if (value < 3600) return `${Math.round(value / 60)}分钟`
  return `${Math.round(value / 3600)}小时`
}

const typeLabel = value => channelTypeOptions.find(item => item.value === value)?.label || value || '-'

const typeIcon = value => channelTypeOptions.find(item => item.value === value)?.icon || ''

const templateTypeLabel = value => templateTypeOptions.find(item => item.value === value)?.label || value || '-'

onMounted(() => {
  loadList()
  loadChannelOptions()
  if (activeTab.value === 'templates') loadTemplateOptions()
  loadRuleFormOptions()
})
</script>

<style scoped>
:global(.home-container .el-main:has(.alert-notice-page)) {
  --ops-shell-gutter: 10px;
  --ops-page-padding: 7px;
  --ops-module-gap: 8px;
  padding-top: var(--ops-shell-gutter);
  padding-right: var(--ops-shell-gutter);
  padding-left: var(--ops-shell-gutter);
  overflow-y: hidden;
}

.alert-notice-page {
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

.notice-header-summary {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 8px;
  color: var(--ops-text-secondary);
}

.notice-header-summary span {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  white-space: nowrap;
}

.notice-header-summary em {
  color: var(--ops-text-muted);
  font-size: 12px;
  font-style: normal;
}

.notice-header-summary strong {
  color: var(--ops-text-heading);
  font-family: Consolas, Monaco, monospace;
  font-size: 15px;
  line-height: 1;
}

.notice-header-summary > i {
  width: 1px;
  height: 18px;
  background: var(--ops-border-soft);
}

.alert-notice-page :deep(.alert-center-card.el-card) {
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

.alert-notice-page :deep(.alert-center-card::before) {
  display: none;
}

.alert-notice-page :deep(.alert-center-card .el-card__body) {
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

.alert-notice-page :deep(.alert-tabs .el-tabs__header) {
  margin: 0 0 6px;
}

.alert-notice-page :deep(.alert-tabs .el-tabs__item) {
  height: 34px;
}

.alert-notice-page .alert-search-section {
  margin-bottom: 8px;
  padding: 8px 10px 2px;
  border: 1px solid var(--ops-border-soft);
  border-radius: 10px;
  background: var(--ops-bg-subtle);
}

.alert-notice-page :deep(.alert-search-form .el-form-item) {
  margin-right: 10px;
  margin-bottom: 6px;
}

.alert-notice-page :deep(.alert-search-form .el-form-item__label) {
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

.alert-notice-page :deep(.alert-search-form .el-input__wrapper),
.alert-notice-page :deep(.alert-search-form .el-select__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 7px;
  background: #ffffff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

.alert-notice-page :deep(.alert-search-form .el-input__wrapper:hover),
.alert-notice-page :deep(.alert-search-form .el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

.alert-notice-page :deep(.alert-search-form .el-input__wrapper.is-focus),
.alert-notice-page :deep(.alert-search-form .el-select__wrapper.is-focused) {
  box-shadow: 0 0 0 1px #409eff inset;
}

.alert-notice-page :deep(.notice-toolbar-button.el-button),
.alert-notice-page :deep(.alert-create-button.el-button) {
  height: 30px;
  min-height: 30px;
  padding: 0 10px;
  border-radius: 8px;
  color: #ffffff;
  font-size: 12px;
  box-shadow: none;
}

.alert-notice-page :deep(.alert-search-button.el-button) {
  border-color: #409eff;
  background: #409eff;
}

.alert-notice-page :deep(.alert-reset-button.el-button) {
  border-color: #e6a23c;
  background: #e6a23c;
}

.alert-notice-page :deep(.alert-create-button.el-button) {
  border-color: #67c23a;
  background: #67c23a;
}

.alert-notice-page :deep(.notice-toolbar-button .el-icon),
.alert-notice-page :deep(.alert-create-button .el-icon) {
  font-size: 14px;
}

.alert-notice-page :deep(.el-button:hover),
.alert-notice-page :deep(.el-button:active) {
  transform: none;
}

.alert-notice-page .alert-section-header {
  padding: 0 2px 6px;
  color: var(--ops-text-secondary);
  font-size: 12px;
}

.alert-notice-page .alert-table-shell,
.alert-notice-page :deep(.alert-table-shell .el-table),
.alert-notice-page :deep(.alert-table-shell .el-table__inner-wrapper) {
  border-radius: 8px;
}

.alert-notice-page .alert-table-shell {
  border-color: var(--ops-border-soft);
  background: #ffffff;
}

.alert-notice-page :deep(.alert-table th.el-table__cell) {
  height: 40px;
  padding: 6px 0;
  background: #f3f7fc;
  color: #334155;
  font-size: 12px;
  font-weight: 700;
}

.alert-notice-page :deep(.alert-table td.el-table__cell) {
  height: 50px;
  padding: 6px 0;
}

.alert-notice-page :deep(.alert-table .el-table__row:hover > td.el-table__cell) {
  background: #f7faff;
}

.alert-notice-page .alert-pagination {
  padding: 8px 2px 0;
}

.full-select,
.full-input-number {
  width: 100%;
}

.notice-advanced-tip {
  margin-left: 10px;
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.notice-template-vars {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.notice-rule-dialog :deep(.el-dialog__body) {
  max-height: 76vh;
  overflow: auto;
  padding: 8px 14px 6px;
}

.notice-rule-dialog :deep(.el-form),
.notice-rule-dialog :deep(.el-form-item),
.notice-rule-dialog :deep(.el-form-item__content) {
  min-width: 0;
}

.notice-rule-dialog :deep(.el-dialog__footer) {
  padding: 8px 14px 10px;
}

.notice-rule-dialog :deep(.el-form-item) {
  margin-bottom: 6px;
}

.notice-rule-dialog :deep(.el-form-item__label) {
  height: 30px;
  line-height: 30px;
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
}

.notice-rule-dialog :deep(.el-form-item__content) {
  line-height: 30px;
}

.notice-rule-dialog :deep(.el-input__wrapper),
.notice-rule-dialog :deep(.el-select__wrapper),
.notice-rule-dialog :deep(.el-input-number .el-input__wrapper),
.notice-rule-dialog :deep(.el-input-tag__wrapper) {
  min-height: 30px;
  border-radius: 8px;
}

.notice-rule-dialog :deep(.el-button) {
  min-height: 30px;
  padding: 6px 10px;
  border-radius: 8px;
  font-size: 12px;
}

.notice-rule-dialog :deep(.el-switch) {
  height: 30px;
  display: inline-flex;
  align-items: center;
  white-space: nowrap;
}

.notice-rule-dialog :deep(.el-switch__label) {
  height: 20px;
  display: inline-flex;
  align-items: center;
  white-space: nowrap;
}

.notice-rule-dialog :deep(.el-switch__label span) {
  white-space: nowrap;
}

.notice-switch-inline {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 96px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 28px;
  white-space: nowrap;
}

.notice-switch-inline span {
  display: inline-block;
  white-space: nowrap;
}

.notice-switch-inline :deep(.el-switch) {
  flex: 0 0 auto;
}

.notice-rule-form {
  display: grid;
  gap: 6px;
  min-width: 0;
}

.notice-rule-config-list {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.notice-rule-section {
  min-width: 0;
  padding: 8px 12px 2px;
  border: 1px solid var(--aiops-border-cool);
  border-radius: 8px;
  background: #fff;
}

.notice-rule-config-card {
  padding-bottom: 8px;
}

.notice-rule-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-height: 28px;
}

.notice-rule-section h4 {
  margin: 0 0 6px;
  color: var(--aiops-text-primary);
  font-size: 14px;
  font-weight: 700;
}

.notice-rule-block-title {
  margin: 0 0 4px;
  color: var(--aiops-text-secondary);
  font-size: 13px;
  font-weight: 700;
}

.notice-basic-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(148px, 0.35fr);
  column-gap: 10px;
  row-gap: 0;
  min-width: 0;
}

.notice-basic-name {
  grid-column: span 1;
}

.notice-noise-rows {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.notice-noise-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  min-width: 0;
}

.notice-noise-aggregation-row :deep(.el-form-item) {
  flex: 0 0 220px;
}

.notice-noise-row :deep(.el-form-item:first-child) {
  flex: 0 0 150px;
}

.notice-noise-aggregation-row :deep(.el-input-number) {
  width: 96px;
}

.notice-field-help {
  overflow: hidden;
  color: var(--aiops-text-muted);
  font-size: 11px;
  line-height: 16px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.notice-action-control-grid {
  display: grid;
  grid-template-columns: minmax(160px, 0.35fr) minmax(132px, 0.35fr);
  column-gap: 10px;
  row-gap: 0;
  margin-top: 6px;
  min-width: 0;
}

.notice-action-control-grid :deep(.el-form-item) {
  min-width: 0;
}

.notice-action-control-grid :deep(.el-form-item__label) {
  padding-right: 8px;
  white-space: nowrap;
}

.notice-repeat-number {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.notice-repeat-count-input {
  width: 92px;
}

.notice-repeat-seconds-input {
  width: 104px;
}

.notice-repeat-number em {
  color: var(--aiops-text-muted);
  font-style: normal;
  white-space: nowrap;
}

.notice-match-simple-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  column-gap: 12px;
  row-gap: 0;
  min-width: 0;
}

.notice-severity-checks {
  min-height: 30px;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 2px 12px;
}

.notice-severity-checks :deep(.el-checkbox) {
  margin-right: 0;
}

.notice-severity-badge {
  display: inline-flex;
  align-items: center;
  height: 22px;
  padding: 0 8px;
  border-radius: 6px;
  color: #fff;
  font-size: 12px;
  font-weight: 700;
  line-height: 1;
}

.notice-severity-badge.is-p0 {
  background: #f45b69;
}

.notice-severity-badge.is-p1 {
  background: #f5a623;
}

.notice-severity-badge.is-p2 {
  background: #3f7ee8;
}

.notice-advanced-match-collapse {
  margin-top: 0;
  border-top: none;
  border-bottom: none;
}

.notice-advanced-match-collapse :deep(.el-collapse-item__header) {
  height: 24px;
  border-bottom: none;
  color: var(--aiops-brand-active);
  font-size: 12px;
  font-weight: 700;
}

.notice-advanced-match-collapse :deep(.el-collapse-item__wrap) {
  border-bottom: none;
  background: transparent;
}

.notice-advanced-match-collapse :deep(.el-collapse-item__content) {
  padding: 2px 0 0;
}

.notice-rule-subsection {
  display: grid;
  gap: 4px;
  margin-top: 6px;
}

.notice-rule-subtitle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  min-height: 26px;
  min-width: 0;
}

.notice-rule-subtitle h4 {
  margin: 0;
}

.notice-rule-flow-tip {
  margin: 6px 0 4px;
  padding: 4px 10px;
  border-radius: 6px;
  background: rgba(64, 158, 255, 0.08);
  color: var(--aiops-text-secondary);
  font-size: 12px;
  line-height: 16px;
}

.notice-section-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.notice-matcher-row,
.notice-time-row {
  display: grid;
  gap: 6px;
}

.notice-matcher-row {
  grid-template-columns: minmax(0, 1fr) 132px minmax(0, 1.2fr) 66px;
}

.notice-time-row {
  grid-template-columns: minmax(0, 1.4fr) 118px 118px 66px;
}

.notice-empty-line {
  min-height: 24px;
  display: flex;
  align-items: center;
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.notice-action-list {
  display: grid;
  gap: 6px;
  min-width: 0;
}

.notice-action-card {
  min-width: 0;
  padding: 8px 12px 2px;
  border: 1px solid var(--aiops-border-cool);
  border-radius: 8px;
  background: #fbfcff;
}

.notice-action-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-height: 24px;
  margin-bottom: 2px;
}

.notice-action-card-head strong {
  color: var(--aiops-text-primary);
  font-size: 13px;
}

.notice-action-card-tools {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
  white-space: nowrap;
}

.notice-action-card-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  column-gap: 12px;
  row-gap: 0;
  min-width: 0;
}

.notice-add-rule-card {
  width: 100%;
  min-height: 30px;
  border-style: dashed;
  border-color: var(--aiops-border-cool);
  background: #fff;
  color: var(--aiops-brand-active);
  font-weight: 700;
}

.notice-record-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  margin-bottom: 10px;
}

.notice-record-stats div {
  display: grid;
  gap: 4px;
  padding: 10px 12px;
  border: 1px solid var(--aiops-border-cool);
  border-radius: 8px;
  background: rgba(247, 250, 255, 0.78);
}

.notice-record-stats span {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.notice-record-stats strong {
  color: var(--aiops-text-primary);
  font-size: 20px;
  line-height: 1;
}

.notice-record-stats strong.success {
  color: #16a34a;
}

.notice-record-stats strong.failed {
  color: #ef4444;
}

.notice-record-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 10px;
}

.notice-record-dialog :deep(.el-dialog__body) {
  padding: 14px 18px 16px;
}

.notice-record-table :deep(th.el-table__cell) {
  height: 40px;
  padding: 0;
  font-size: 12px;
}

.notice-record-table :deep(td.el-table__cell) {
  height: 46px;
  padding: 0;
  font-size: 12px;
}

.notice-record-table :deep(.el-table__cell .cell) {
  min-width: 0;
  white-space: nowrap;
}

.notice-record-table :deep(.notice-record-detail-button) {
  --el-button-text-color: #315f9f;
  --el-button-hover-link-text-color: #244f8a;
  --el-button-active-color: #1f467a;
  padding: 4px 8px;
  background: #eef5ff;
  color: #315f9f;
  box-shadow: none;
}

.notice-record-table :deep(.notice-record-detail-button:hover),
.notice-record-table :deep(.notice-record-detail-button:focus-visible) {
  background: #e1edff;
  color: #244f8a;
  box-shadow: none;
}

.notice-record-table :deep(.notice-record-detail-button:active) {
  background: #d7e7ff;
  color: #1f467a;
}

.notice-record-text {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.notice-payload-content {
  max-height: 520px;
  margin: 0;
  overflow: auto;
  padding: 12px;
  border: 1px solid var(--aiops-border-cool);
  border-radius: 8px;
  background: rgba(247, 250, 255, 0.86);
  color: var(--aiops-text-secondary);
  font-family: Consolas, Monaco, "Courier New", monospace;
  font-size: 12px;
  line-height: 1.7;
  white-space: pre-wrap;
  word-break: break-word;
}

.notice-channel-option,
.notice-channel-inline {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
}

.notice-channel-icon,
.notice-select-prefix-icon {
  width: 22px;
  height: 22px;
  object-fit: contain;
  flex-shrink: 0;
}

.notice-channel-icon.small {
  width: 18px;
  height: 18px;
}

.notice-channel-type-text {
  display: inline-block;
  max-width: 130px;
}

.notice-rule-channel-list {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  max-width: 100%;
  overflow: hidden;
  vertical-align: middle;
}

.notice-rule-channel-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
  max-width: 150px;
  color: var(--aiops-text-secondary);
  white-space: nowrap;
}

.notice-rule-channel-item span {
  overflow: hidden;
  text-overflow: ellipsis;
}

.notice-description-text {
  display: inline-block;
  padding-left: 6px;
  color: var(--aiops-text-secondary);
}

.notice-inline-help {
  padding: 10px 12px;
  border-radius: 10px;
  background: rgba(76, 139, 245, 0.08);
  color: var(--aiops-text-secondary);
  line-height: 1.7;
}

.notice-select-prefix-icon {
  margin-right: 4px;
}

.notice-template-content :deep(.el-textarea__inner),
.notice-preview-result {
  font-family: Consolas, Monaco, "Courier New", monospace;
}

.notice-preview-result {
  margin: 0;
  padding: 12px;
  max-width: 100%;
  border-radius: 10px;
  border: 1px solid var(--aiops-border-cool);
  background: rgba(247, 250, 255, 0.86);
  color: var(--aiops-text-secondary);
  line-height: 1.7;
  box-sizing: border-box;
}

.notice-test-status {
  min-width: 86px;
  justify-content: center;
  font-weight: 700;
}

.notice-preview-result {
  max-height: 360px;
  overflow: auto;
  white-space: pre-wrap;
}

:global(.alert-notice-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  max-width: calc(100vw - 32px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: #ffffff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.alert-notice-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.alert-notice-dialog-overlay .el-dialog__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.alert-notice-dialog-overlay .el-dialog__body) {
  box-sizing: border-box;
  max-height: calc(100vh - 160px);
  padding: 16px 18px;
  overflow: auto;
  color: #53657d;
}

:global(.alert-notice-dialog-overlay .notice-rule-dialog .el-dialog__body) {
  max-height: 76vh;
  padding: 8px 14px 6px;
}

:global(.alert-notice-dialog-overlay .notice-record-dialog .el-dialog__body) {
  padding: 14px 18px 16px;
}

:global(.alert-notice-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #ffffff;
}

:global(.alert-notice-dialog-overlay .notice-rule-dialog .el-dialog__footer) {
  padding: 8px 14px 10px;
}

:global(.alert-notice-dialog-overlay .el-dialog__footer .el-button) {
  min-height: 34px;
  border-radius: 8px;
}

:global(.alert-notice-dialog-overlay .el-button:hover),
:global(.alert-notice-dialog-overlay .el-button:active) {
  transform: none;
}

:global(.alert-notice-dialog-overlay .el-dialog:not(.notice-rule-dialog) .el-input__wrapper),
:global(.alert-notice-dialog-overlay .el-dialog:not(.notice-rule-dialog) .el-select__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.alert-notice-dialog-overlay .el-dialog:not(.notice-rule-dialog) .el-textarea__inner) {
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.alert-notice-dialog-overlay .el-dialog:not(.notice-rule-dialog) .el-input__wrapper:hover),
:global(.alert-notice-dialog-overlay .el-dialog:not(.notice-rule-dialog) .el-select__wrapper:hover),
:global(.alert-notice-dialog-overlay .el-dialog:not(.notice-rule-dialog) .el-textarea__inner:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset;
}

:global(.alert-notice-dialog-overlay .el-dialog:not(.notice-rule-dialog) .el-input__wrapper.is-focus),
:global(.alert-notice-dialog-overlay .el-dialog:not(.notice-rule-dialog) .el-select__wrapper.is-focused),
:global(.alert-notice-dialog-overlay .el-dialog:not(.notice-rule-dialog) .el-textarea__inner:focus) {
  box-shadow: 0 0 0 1px #409eff inset;
}

@media (max-width: 1200px) {
  .ops-page-header {
    flex-wrap: wrap;
  }

  .notice-header-summary {
    width: 100%;
    padding-left: 54px;
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

  .notice-header-summary {
    padding-left: 0;
    overflow-x: auto;
  }

  .alert-notice-page :deep(.alert-search-form .el-form-item) {
    width: 100%;
    margin-right: 0;
  }

  .alert-notice-page :deep(.alert-search-form .el-form-item:last-child .el-form-item__content) {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }
}
</style>
