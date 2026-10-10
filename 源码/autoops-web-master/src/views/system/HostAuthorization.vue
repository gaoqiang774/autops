<template>
  <div class="authorization-management">
    <header class="ops-page-header authorization-page-header">
      <el-button class="modern-btn back-btn" @click="handleBack">
        <el-icon><ArrowLeft /></el-icon>
        返回
      </el-button>
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/凭据.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>堡垒机资产授权</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>以用户组为授权主体，统一管理主机、K8S 集群与日志项目访问范围。</p>
      </div>
      <div class="authorization-header-summary" aria-label="资产授权统计">
        <span><em>资产</em><strong>{{ assetTypeLabel }}</strong></span>
        <i aria-hidden="true"></i>
        <span><em>用户组</em><strong>{{ selectedSubjectName || '-' }}</strong></span>
        <i aria-hidden="true"></i>
        <span><em>授权</em><strong>{{ pagination.total }}</strong></span>
        <i aria-hidden="true"></i>
        <span><em>选中</em><strong>{{ selectedAssets.length }}</strong></span>
      </div>
    </header>

    <section class="auth-page-card">
      <div class="workbench-header">
        <div>
          <h3 class="section-title">用户组已授权{{ assetTypeLabel }}</h3>
          <p class="section-subtitle">{{ sectionSubtitle }}</p>
        </div>
        <div class="workbench-actions">
          <el-button
            type="success"
            :disabled="!selectedSubjectId"
            class="modern-btn auth-toolbar-button"
            @click="handleAddAuthorization"
          >
            <el-icon><Plus /></el-icon>
            新增授权{{ assetTypeLabel }}
          </el-button>
          <el-button
            type="danger"
            :disabled="!selectedSubjectId || selectedAssets.length === 0"
            class="modern-btn danger-outline auth-toolbar-button"
            @click="handleBatchRevoke"
          >
            <el-icon><Delete /></el-icon>
            批量取消 ({{ selectedAssets.length }})
          </el-button>
        </div>
      </div>

      <div class="toolbar-card">
        <div class="toolbar-left">
          <el-radio-group
            v-model="assetType"
            class="asset-type-group"
            @change="handleAssetTypeChange"
          >
            <el-radio-button label="host">主机</el-radio-button>
            <el-radio-button label="k8s_cluster">K8S 集群</el-radio-button>
            <el-radio-button label="log_project">日志项目</el-radio-button>
          </el-radio-group>
        </div>

        <div class="toolbar-right">
          <el-switch
            v-model="includeExpired"
            active-text="包含已过期授权"
            @change="handleRefresh"
          />
          <el-button
            type="warning"
            :disabled="!selectedSubjectId"
            :loading="tableLoading"
            class="modern-btn ops-refresh-button auth-toolbar-button"
            @click="handleRefresh"
          >
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </div>

      <el-alert
        type="info"
        :closable="false"
        show-icon
        class="page-alert"
      >
        <template #title>
          {{ pageAlertText }}
        </template>
      </el-alert>

      <div v-if="selectedSubjectId" class="asset-list-section table-shell">
        <el-table
          v-loading="tableLoading"
          :data="assetList"
          border
          stripe
          class="modern-table asset-table"
          @selection-change="handleSelectionChange"
        >
          <el-table-column type="selection" width="55" />
          <template v-if="isHostAsset">
          <el-table-column label="主机ID" width="90">
            <template #default="{ row }">
              {{ getHostId(row) }}
            </template>
          </el-table-column>
          <el-table-column label="主机信息" min-width="220">
            <template #default="{ row }">
              <div class="host-meta">
                <span class="host-name">{{ getHostName(row) }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="sshIp" label="SSH 地址" min-width="150" />
          <el-table-column prop="privateIp" label="内网 IP" min-width="150" />
          </template>
          <template v-else>
          <el-table-column :label="assetIdColumnLabel" width="90">
            <template #default="{ row }">
              {{ getAssetId(row) }}
            </template>
          </el-table-column>
          <el-table-column :label="assetTypeLabel" min-width="220">
            <template #default="{ row }">
              <div class="host-meta">
                <span class="host-name">{{ getAssetName(row) }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="授权范围" min-width="260">
            <template #default="{ row }">
              <div v-if="isAllScopes(row)" class="scope-tags">
                <el-tag type="success" size="small">{{ allScopesText }}</el-tag>
              </div>
              <div v-else class="scope-tags">
                <el-tag
                  v-for="(scope, index) in getScopeValues(row)"
                  :key="`${scope}-${index}`"
                  type="warning"
                  size="small"
                >
                  {{ getScopeTagLabel(row, scope, index) }}
                </el-tag>
                <span v-if="getScopeValues(row).length === 0">-</span>
              </div>
            </template>
          </el-table-column>
          </template>
          <el-table-column label="生效时间" min-width="180">
            <template #default="{ row }">
              <span class="time-text">{{ formatEffectiveTime(getRowEffectiveTime(row)) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="失效时间" min-width="180">
            <template #default="{ row }">
              <span class="time-text">{{ formatExpireTime(getRowExpireTime(row)) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="生命周期状态" width="120" align="center">
            <template #default="{ row }">
              <el-tag :type="getLifecycleStatus(row).type" size="small">
                {{ getLifecycleStatus(row).text }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column v-if="isHostAsset" prop="status" label="主机状态" width="110" align="center">
            <template #default="{ row }">
              <el-tag :type="getStatusType(row.status)" size="small">
                {{ getStatusText(row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="140" fixed="right">
            <template #default="{ row }">
              <el-button
                link
                type="danger"
                size="small"
                class="revoke-link-btn"
                @click="handleRevokeSingle(row)"
              >
                取消授权
              </el-button>
            </template>
          </el-table-column>
        </el-table>

        <el-pagination
          :current-page="pagination.page"
          :page-size="pagination.pageSize"
          :total="pagination.total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next, jumper"
          class="modern-pagination"
          @size-change="handleSizeChange"
          @current-change="handlePageChange"
        />
      </div>

      <el-empty
        v-else
        description="请选择用户组后查看授权列表"
      />
    </section>

    <HostSelectorDialog
      v-model="selectorVisible"
      title="选择要授权的主机"
      @hosts-selected="handleHostsSelected"
    />

    <el-dialog
      v-model="authorizationDialogVisible"
      title="配置授权生命周期"
      width="1080px"
      class="lifecycle-dialog"
      modal-class="authorization-dialog-overlay"
      @close="resetAuthorizationDialog"
    >
      <el-alert
        type="info"
        :closable="false"
        show-icon
        class="dialog-alert"
      >
        <template #title>
          默认时间作用于本次选中的全部主机；下方单机时间会覆盖默认值。不填生效时间表示立即生效，不填失效时间表示长期有效。
        </template>
      </el-alert>

      <el-form label-width="110px" class="config-form">
        <el-row :gutter="16">
          <el-col :xs="24" :md="12">
            <el-form-item label="默认生效时间">
              <el-date-picker
                v-model="authorizationForm.effectiveTime"
                type="datetime"
                value-format="YYYY-MM-DD HH:mm:ss"
                format="YYYY-MM-DD HH:mm:ss"
                placeholder="不填则立即生效"
                clearable
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="默认失效时间">
              <el-date-picker
                v-model="authorizationForm.expireTime"
                type="datetime"
                value-format="YYYY-MM-DD HH:mm:ss"
                format="YYYY-MM-DD HH:mm:ss"
                placeholder="不填则长期有效"
                clearable
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>

      <div class="config-actions">
        <div class="config-actions__title">
          <el-icon><Clock /></el-icon>
          <span>单机覆盖配置</span>
        </div>
        <div class="config-actions__buttons">
          <el-button @click="clearDefaultLifecycle">清空默认时间</el-button>
          <el-button @click="clearRowOverrides">清空单机覆盖</el-button>
        </div>
      </div>

      <el-table
        :data="authorizationForm.authorizations"
        border
        stripe
        row-key="hostId"
        class="modern-table host-lifecycle-table"
        max-height="420"
      >
        <el-table-column label="主机名称" min-width="220">
          <template #default="{ row }">
            <div class="host-meta">
              <span class="host-name">{{ row.hostName || '-' }}</span>
              <span class="host-subtext">{{ getHostAuthorizationSubtext(row) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="登录账号" min-width="230">
          <template #default="{ row }">
            <el-select
              v-model="row.authIds"
              multiple
              collapse-tags
              placeholder="请选择授权账号"
              :disabled="row.sshAuths.length === 1"
              style="width: 100%"
            >
              <el-option
                v-for="auth in row.sshAuths"
                :key="auth.authId"
                :label="`${auth.username || '-'} / ${auth.name || auth.authId}`"
                :value="auth.authId"
              />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="生效时间" min-width="210">
          <template #default="{ row }">
            <el-date-picker
              v-model="row.effectiveTime"
              type="datetime"
              value-format="YYYY-MM-DD HH:mm:ss"
              format="YYYY-MM-DD HH:mm:ss"
              placeholder="沿用默认值"
              clearable
              style="width: 100%"
            />
          </template>
        </el-table-column>
        <el-table-column label="失效时间" min-width="210">
          <template #default="{ row }">
            <el-date-picker
              v-model="row.expireTime"
              type="datetime"
              value-format="YYYY-MM-DD HH:mm:ss"
              format="YYYY-MM-DD HH:mm:ss"
              placeholder="沿用默认值"
              clearable
              style="width: 100%"
            />
          </template>
        </el-table-column>
      </el-table>

      <template #footer>
        <div class="dialog-footer">
          <span class="footer-tip">
            已选择 {{ authorizationForm.authorizations.length }} 台主机
          </span>
          <div class="footer-buttons">
            <el-button @click="authorizationDialogVisible = false">取消</el-button>
            <el-button
              type="primary"
              :loading="authorizationSubmitting"
              @click="submitAuthorization"
            >
              确认授权
            </el-button>
          </div>
        </div>
      </template>
    </el-dialog>

    <el-dialog
      v-model="k8sAuthorizationDialogVisible"
      :title="`配置 ${assetTypeLabel}授权`"
      width="1080px"
      class="lifecycle-dialog"
      modal-class="authorization-dialog-overlay"
      @close="resetK8sAuthorizationDialog"
    >
      <el-alert
        type="info"
        :closable="false"
        show-icon
        class="dialog-alert"
      >
        <template #title>
          {{ scopedDialogAlertText }}
        </template>
      </el-alert>

      <el-form label-width="110px" class="config-form">
        <el-form-item :label="selectAssetFormLabel">
          <el-select
            v-model="selectedK8sClusterIds"
            multiple
            filterable
            collapse-tags
            :placeholder="`请选择要授权的 ${assetTypeLabel}`"
            :loading="scopedAssetLoading"
            style="width: 100%"
            @change="handleK8sClusterSelectionChange"
          >
            <el-option
              v-for="asset in scopedAssetOptions"
              :key="asset.id"
              :label="getScopedAssetName(asset)"
              :value="asset.id"
            >
              <div class="subject-option">
                <span class="subject-option__label">{{ getScopedAssetName(asset) }}</span>
                <span class="subject-option__meta">{{ getScopedAssetMeta(asset) }}</span>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
        <el-row :gutter="16">
          <el-col :xs="24" :md="12">
            <el-form-item label="默认生效时间">
              <el-date-picker
                v-model="k8sAuthorizationForm.effectiveTime"
                type="datetime"
                value-format="YYYY-MM-DD HH:mm:ss"
                format="YYYY-MM-DD HH:mm:ss"
                placeholder="不填则立即生效"
                clearable
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :md="12">
            <el-form-item label="默认失效时间">
              <el-date-picker
                v-model="k8sAuthorizationForm.expireTime"
                type="datetime"
                value-format="YYYY-MM-DD HH:mm:ss"
                format="YYYY-MM-DD HH:mm:ss"
                placeholder="不填则长期有效"
                clearable
                style="width: 100%"
              />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>

      <div class="config-actions">
        <div class="config-actions__title">
          <el-icon><Clock /></el-icon>
          <span>{{ assetTypeLabel }}范围配置</span>
        </div>
        <div class="config-actions__buttons">
          <el-button @click="clearK8sDefaultLifecycle">清空默认时间</el-button>
          <el-button @click="clearK8sRowOverrides">清空单项时间</el-button>
        </div>
      </div>

      <el-table
        :data="k8sAuthorizationForm.authorizations"
        border
        stripe
        row-key="assetId"
        class="modern-table scoped-asset-table"
        max-height="420"
      >
        <el-table-column :label="assetTypeLabel" min-width="210">
          <template #default="{ row }">
            <div class="host-meta">
              <span class="host-name">{{ row.assetName || '-' }}</span>
              <span class="host-subtext">{{ getScopedRowSubtext(row) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="授权范围" min-width="380">
          <template #default="{ row }">
            <div class="scope-config">
              <el-radio-group v-model="row.allScopes" class="scope-mode">
                <el-radio-button :label="true">{{ allScopesText }}</el-radio-button>
                <el-radio-button :label="false">{{ scopeTypeLabel }}</el-radio-button>
              </el-radio-group>
              <el-select
                v-if="!row.allScopes"
                v-model="row.scopeValues"
                multiple
                filterable
                collapse-tags
                :placeholder="`请选择${scopeTypeLabel}`"
                :loading="row.namespaceLoading"
                style="width: 100%"
                @visible-change="visible => visible && loadNamespacesForAuthorization(row)"
              >
                <el-option
                  v-for="namespace in row.namespaceOptions"
                  :key="namespace.name"
                  :label="namespace.label || namespace.name"
                  :value="namespace.name"
                >
                  <div class="subject-option">
                    <span class="subject-option__label">{{ namespace.label || namespace.name }}</span>
                    <span class="subject-option__meta">{{ namespace.status || 'Active' }}</span>
                  </div>
                </el-option>
              </el-select>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="生效时间" min-width="200">
          <template #default="{ row }">
            <el-date-picker
              v-model="row.effectiveTime"
              type="datetime"
              value-format="YYYY-MM-DD HH:mm:ss"
              format="YYYY-MM-DD HH:mm:ss"
              placeholder="沿用默认值"
              clearable
              style="width: 100%"
            />
          </template>
        </el-table-column>
        <el-table-column label="失效时间" min-width="200">
          <template #default="{ row }">
            <el-date-picker
              v-model="row.expireTime"
              type="datetime"
              value-format="YYYY-MM-DD HH:mm:ss"
              format="YYYY-MM-DD HH:mm:ss"
              placeholder="沿用默认值"
              clearable
              style="width: 100%"
            />
          </template>
        </el-table-column>
      </el-table>

      <template #footer>
        <div class="dialog-footer">
          <span class="footer-tip">
            已选择 {{ k8sAuthorizationForm.authorizations.length }} 个{{ assetTypeLabel }}
          </span>
          <div class="footer-buttons">
            <el-button @click="k8sAuthorizationDialogVisible = false">取消</el-button>
            <el-button
              type="primary"
              :loading="authorizationSubmitting"
              @click="submitK8sAuthorization"
            >
              确认授权
            </el-button>
          </div>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import moment from 'moment'
import { ArrowLeft, Clock, Delete, Plus, Refresh } from '@element-plus/icons-vue'
import HostSelectorDialog from '@/components/HostSelectorDialog.vue'
import k8sApi from '@/api/k8s'
import logtailApi from '@/api/logtail'

const DATE_TIME_FORMATS = [
  'YYYY-MM-DD HH:mm:ss',
  'YYYY-MM-DD HH:mm',
  'YYYY/MM/DD HH:mm:ss',
  'YYYY/MM/DD HH:mm'
]
const EMPTY_TIME_VALUES = new Set([
  '',
  '0001/01/01 00:00',
  '0001/01/01 00:00:00',
  '0001-01-01 00:00',
  '0001-01-01 00:00:00'
])

const normalizeSshAuths = (sshAuths = []) => {
  return (Array.isArray(sshAuths) ? sshAuths : [])
    .map(auth => ({
      ...auth,
      authId: auth.authId || auth.id
    }))
    .filter(auth => auth.authId)
}

const createAuthorizationItems = (hosts = []) => {
  return hosts.map(host => {
    const sshAuths = normalizeSshAuths(host.sshAuths)

    return {
      hostId: host.id || host.hostId,
      hostName: host.hostName || host.name || '',
      sshIp: host.sshIp || host.publicIp || '',
      privateIp: host.privateIp || '',
      groupName: host.groupName || '',
      sshAuths,
      authIds: sshAuths.length === 1 ? [sshAuths[0].authId] : [],
      effectiveTime: '',
      expireTime: ''
    }
  })
}

const createAuthorizationForm = (hosts = []) => ({
  effectiveTime: '',
  expireTime: '',
  authorizations: createAuthorizationItems(hosts)
})

const normalizeListData = data => {
  if (Array.isArray(data)) {
    return {
      list: data,
      total: data.length
    }
  }

  if (!data || typeof data !== 'object') {
    return {
      list: [],
      total: 0
    }
  }

  const list = data.list || data.records || data.items || data.assets || []
  return {
    list,
    total: data.total || data.count || list.length
  }
}

const createK8sAuthorizationItems = (assets = [], assetType = 'k8s_cluster') => {
  return assets.map(asset => ({
    assetId: asset.id,
    assetName: asset.name || asset.clusterName || (assetType === 'log_project' ? `日志项目 ${asset.id}` : `K8S 集群 ${asset.id}`),
    status: asset.status,
    description: asset.description || asset.ownerName || asset.code || '',
    assetType,
    allScopes: true,
    scopeValues: [],
    namespaceOptions: [],
    namespaceLoading: false,
    effectiveTime: '',
    expireTime: ''
  }))
}

const createK8sAuthorizationForm = (assets = [], assetType = 'k8s_cluster') => ({
  effectiveTime: '',
  expireTime: '',
  authorizations: createK8sAuthorizationItems(assets, assetType)
})

export default {
  name: 'HostAuthorization',
  components: {
    Clock,
    Delete,
    Plus,
    Refresh,
    ArrowLeft,
    HostSelectorDialog
  },
  data() {
    return {
      subjectType: 'group',
      assetType: 'host',
      selectedSubjectId: null,
      groupOptions: [],
      k8sClusterOptions: [],
      logProjectOptions: [],
      groupLoading: false,
      k8sClusterLoading: false,
      logProjectLoading: false,
      tableLoading: false,
      authorizationSubmitting: false,
      includeExpired: false,
      assetList: [],
      selectedAssets: [],
      pagination: {
        page: 1,
        pageSize: 10,
        total: 0
      },
      selectorVisible: false,
      authorizationDialogVisible: false,
      k8sAuthorizationDialogVisible: false,
      authorizationForm: createAuthorizationForm([]),
      k8sAuthorizationForm: createK8sAuthorizationForm([]),
      selectedK8sClusterIds: []
    }
  },
  computed: {
    isHostAsset() {
      return this.assetType === 'host'
    },
    isK8sAsset() {
      return this.assetType === 'k8s_cluster'
    },
    isLogAsset() {
      return this.assetType === 'log_project'
    },
    isScopedAsset() {
      return this.isK8sAsset || this.isLogAsset
    },
    assetTypeLabel() {
      if (this.isK8sAsset) return 'K8S 集群'
      if (this.isLogAsset) return '日志项目'
      return '主机'
    },
    assetIdColumnLabel() {
      if (this.isK8sAsset) return '集群ID'
      if (this.isLogAsset) return '项目ID'
      return '资产ID'
    },
    scopeTypeValue() {
      return this.isLogAsset ? 'log_store' : 'namespace'
    },
    scopeTypeLabel() {
      return this.isLogAsset ? '日志库' : '命名空间'
    },
    allScopesText() {
      return this.isLogAsset ? '整个日志项目' : '整个集群'
    },
    scopedDialogAlertText() {
      return this.isLogAsset
        ? '授权整个日志项目或指定日志库；时间为空表示立即生效、长期有效。'
        : '授权整个集群或指定命名空间；时间为空表示立即生效、长期有效。'
    },
    selectAssetFormLabel() {
      return this.isLogAsset ? '选择项目' : '选择集群'
    },
    scopedAssetOptions() {
      return this.isLogAsset ? this.logProjectOptions : this.k8sClusterOptions
    },
    scopedAssetLoading() {
      return this.isLogAsset ? this.logProjectLoading : this.k8sClusterLoading
    },
    pageAlertText() {
      if (this.isK8sAsset) {
        return 'K8S 支持授权整个集群或指定 namespace；失效时间到达后权限会自动失效。'
      }
      if (this.isLogAsset) {
        return '日志项目支持授权整个项目或指定日志库；失效时间到达后权限会自动失效。'
      }
      return '主机授权继续使用原有接口；失效时间到达后权限会自动失效，无需额外调用回收接口。'
    },
    sectionSubtitle() {
      if (this.isK8sAsset) {
        return '支持按集群授权全部命名空间或指定 namespace。'
      }
      if (this.isLogAsset) {
        return '支持按日志项目授权全部日志库或指定日志库。'
      }
      return '支持默认生命周期和按主机单独覆盖。'
    },
    subjectOptions() {
      return this.groupOptions
    },
    subjectLoading() {
      return this.groupLoading
    },
    subjectLabel() {
      return '用户组'
    },
    subjectIdField() {
      return 'groupId'
    },
    selectedSubjectName() {
      const selected = this.groupOptions.find(item => item.id === this.selectedSubjectId)
      return selected ? this.getSubjectOptionLabel(selected) : (this.selectedSubjectId ? `ID: ${this.selectedSubjectId}` : '')
    }
  },
  async mounted() {
    const { subjectId, assetType } = this.$route.query
    this.subjectType = 'group'
    if (assetType === 'host' || assetType === 'k8s_cluster' || assetType === 'log_project') {
      this.assetType = assetType
    }

    await this.loadSubjectOptions()
    if (this.isScopedAsset) {
      await this.loadScopedAssetOptions()
    }

    if (subjectId) {
      const parsedId = Number(subjectId)
      if (!Number.isNaN(parsedId)) {
        this.selectedSubjectId = parsedId
        this.fetchAssetList()
      }
    }
  },
  methods: {
    async loadSubjectOptions() {
      await this.loadGroupOptions()
    },

    async loadGroupOptions() {
      this.groupLoading = true
      try {
        const { data: res } = await this.$api.getUserGroupList({
          page: 1,
          pageSize: 200
        })
        if (res.code === 200) {
          this.groupOptions = res.data.list || []
        } else {
          this.$message.error(res.message || '加载用户组列表失败')
        }
      } catch (error) {
        this.$message.error('加载用户组列表失败')
      } finally {
        this.groupLoading = false
      }
    },

    async loadK8sClusterOptions() {
      if (this.k8sClusterOptions.length > 0) {
        return
      }

      this.k8sClusterLoading = true
      try {
        const response = await k8sApi.getClusterList({
          page: 1,
          size: 200,
          pageSize: 200
        })
        const res = response.data || response
        if (res.code === 200 || res.success) {
          const data = normalizeListData(res.data)
          this.k8sClusterOptions = data.list
        } else {
          this.$message.error(res.message || '加载 K8S 集群列表失败')
        }
      } catch (error) {
        this.$message.error('加载 K8S 集群列表失败')
      } finally {
        this.k8sClusterLoading = false
      }
    },

    async loadLogProjectOptions() {
      if (this.logProjectOptions.length > 0) {
        return
      }

      this.logProjectLoading = true
      try {
        const response = await logtailApi.getProjects({
          page: 1,
          pageSize: 200
        })
        const res = response.data || response
        if (res.code === 200 || res.success) {
          const data = normalizeListData(res.data)
          this.logProjectOptions = data.list
        } else {
          this.$message.error(res.message || '加载日志项目列表失败')
        }
      } catch (error) {
        this.$message.error('加载日志项目列表失败')
      } finally {
        this.logProjectLoading = false
      }
    },

    async loadScopedAssetOptions() {
      if (this.isLogAsset) {
        await this.loadLogProjectOptions()
        return
      }

      await this.loadK8sClusterOptions()
    },

    getClusterName(cluster) {
      if (!cluster) {
        return ''
      }

      return cluster?.name || cluster?.clusterName || `K8S 集群 ${cluster?.id || ''}`
    },

    getClusterStatusText(status) {
      const statusMap = {
        0: '离线',
        1: '连接中',
        2: '在线',
        3: '异常'
      }
      return statusMap[status] || '未知'
    },

    getLogProjectName(project) {
      if (!project) {
        return ''
      }

      return project.name || project.projectName || `日志项目 ${project.id || ''}`
    },

    getScopedAssetName(asset) {
      return this.isLogAsset ? this.getLogProjectName(asset) : this.getClusterName(asset)
    },

    getScopedAssetMeta(asset) {
      if (this.isLogAsset) {
        return asset?.code || asset?.ownerName || asset?.description || ''
      }

      return this.getClusterStatusText(asset?.status)
    },

    getScopedRowMeta(row) {
      if (this.isLogAsset) {
        return row.description || row.code || row.ownerName || row.scopeType || 'log_project'
      }

      return this.getClusterStatusText(row.status)
    },

    getScopedRowSubtext(row) {
      const meta = this.getScopedRowMeta(row)
      return meta ? `ID: ${row.assetId} / ${meta}` : `ID: ${row.assetId}`
    },

    getSubjectOptionLabel(item) {
      return item.name || `用户组 ${item.id}`
    },

    async handleAssetTypeChange() {
      this.assetList = []
      this.selectedAssets = []
      this.pagination.page = 1
      this.pagination.total = 0
      this.syncRouteQuery()
      if (this.isScopedAsset) {
        await this.loadScopedAssetOptions()
      }
      this.fetchAssetList()
    },

    handleSubjectChange() {
      this.pagination.page = 1
      this.syncRouteQuery()
      this.fetchAssetList()
    },

    handleRefresh() {
      this.fetchAssetList()
    },

    handleBack() {
      if (window.history.state?.back) {
        this.$router.back()
        return
      }

      this.$router.push('/system/post')
    },

    syncRouteQuery() {
      const query = {
        subjectType: this.subjectType,
        assetType: this.assetType
      }

      if (this.selectedSubjectId) {
        query.subjectId = String(this.selectedSubjectId)
      }

      this.$router.replace({ query }).catch(() => {})
    },

    async fetchAssetList() {
      if (!this.selectedSubjectId) {
        this.assetList = []
        this.pagination.total = 0
        return
      }

      this.tableLoading = true
      try {
        const params = {
          subjectType: this.subjectType,
          subjectId: this.selectedSubjectId,
          page: this.pagination.page,
          pageSize: this.pagination.pageSize,
          includeExpired: this.includeExpired
        }

        const { data: res } = this.isHostAsset
          ? await this.$api.getAuthorizedHosts(params)
          : await this.$api.getAuthorizedAssets({
            ...params,
            assetType: this.assetType
          })

        if (res.code === 200) {
          const data = normalizeListData(res.data)
          this.assetList = data.list
          this.pagination.total = data.total
          this.selectedAssets = []
        } else {
          this.$message.error(res.message || `获取授权${this.assetTypeLabel}列表失败`)
        }
      } catch (error) {
        this.$message.error(`获取授权${this.assetTypeLabel}列表失败`)
      } finally {
        this.tableLoading = false
      }
    },

    handleSizeChange(size) {
      this.pagination.pageSize = size
      this.pagination.page = 1
      this.fetchAssetList()
    },

    handlePageChange(page) {
      this.pagination.page = page
      this.fetchAssetList()
    },

    handleSelectionChange(selection) {
      this.selectedAssets = selection
    },

    handleAddAuthorization() {
      if (!this.selectedSubjectId) {
        this.$message.warning('请先选择用户组')
        return
      }

      if (this.isHostAsset) {
        this.selectorVisible = true
        return
      }

      this.openK8sAuthorizationDialog()
    },

    async handleHostsSelected(hosts) {
      if (!hosts || hosts.length === 0) {
        this.$message.warning('请选择要授权的主机')
        return
      }

      this.authorizationForm = createAuthorizationForm(await this.loadHostsSshAuths(hosts))
      this.authorizationDialogVisible = true
    },

    async openK8sAuthorizationDialog() {
      await this.loadScopedAssetOptions()
      this.selectedK8sClusterIds = []
      this.k8sAuthorizationForm = createK8sAuthorizationForm([], this.assetType)
      this.k8sAuthorizationDialogVisible = true
    },

    handleK8sClusterSelectionChange(clusterIds = []) {
      const currentRows = new Map(
        this.k8sAuthorizationForm.authorizations.map(item => [String(item.assetId), item])
      )
      const selectedClusters = clusterIds
        .map(assetId => this.scopedAssetOptions.find(asset => String(asset.id) === String(assetId)))
        .filter(Boolean)

      this.k8sAuthorizationForm.authorizations = selectedClusters.map(asset => {
        const existing = currentRows.get(String(asset.id))
        if (existing) {
          return existing
        }

        return createK8sAuthorizationItems([asset], this.assetType)[0]
      })
    },

    async loadNamespacesForAuthorization(row) {
      if (!row || row.namespaceOptions.length > 0 || row.namespaceLoading) {
        return
      }

      row.namespaceLoading = true
      try {
        const response = this.isLogAsset
          ? await logtailApi.getStores({
            page: 1,
            pageSize: 500,
            projectId: row.assetId
          })
          : await k8sApi.getNamespaces(row.assetId)
        const res = response.data || response
        if (res.code === 200 || res.success) {
          const scopes = this.isLogAsset
            ? (res.data?.list || res.data?.records || res.data?.items || res.data || [])
            : (res.data?.namespaces || res.data?.list || res.data || [])
          row.namespaceOptions = (Array.isArray(scopes) ? scopes : []).map(scope => ({
            name: this.isLogAsset ? String(scope.id || scope.storeId || '') : (scope.name || scope),
            label: this.isLogAsset ? (scope.name || scope.storeName || scope.code || `日志库 ${scope.id}`) : (scope.name || scope),
            status: this.isLogAsset ? (scope.code || this.getEnabledStatusText(scope.status)) : (scope.status || 'Active')
          }))
        } else {
          this.$message.error(res.message || `加载 ${row.assetName} ${this.scopeTypeLabel}失败`)
        }
      } catch (error) {
        this.$message.error(`加载 ${row.assetName} ${this.scopeTypeLabel}失败`)
      } finally {
        row.namespaceLoading = false
      }
    },

    resetK8sAuthorizationDialog() {
      this.authorizationSubmitting = false
      this.selectedK8sClusterIds = []
      this.k8sAuthorizationForm = createK8sAuthorizationForm([], this.assetType)
    },

    clearK8sDefaultLifecycle() {
      this.k8sAuthorizationForm.effectiveTime = ''
      this.k8sAuthorizationForm.expireTime = ''
    },

    clearK8sRowOverrides() {
      this.k8sAuthorizationForm.authorizations = this.k8sAuthorizationForm.authorizations.map(item => ({
        ...item,
        effectiveTime: '',
        expireTime: ''
      }))
    },

    validateK8sAuthorizationForm() {
      const { effectiveTime, expireTime, authorizations } = this.k8sAuthorizationForm

      if (authorizations.length === 0) {
        this.$message.warning(`请选择要授权的 ${this.assetTypeLabel}`)
        return false
      }

      if (!this.validateTimeOrder(effectiveTime, expireTime)) {
        this.$message.warning('默认失效时间必须晚于当前时间，且晚于默认生效时间')
        return false
      }

      const assetIdSet = new Set()
      for (const item of authorizations) {
        if (assetIdSet.has(item.assetId)) {
          this.$message.warning(`${this.assetTypeLabel} ${item.assetId} 重复，请检查授权列表`)
          return false
        }
        assetIdSet.add(item.assetId)

        if (!item.allScopes && (!item.scopeValues || item.scopeValues.length === 0)) {
          this.$message.warning(`${item.assetName || item.assetId} 请选择至少一个${this.scopeTypeLabel}`)
          return false
        }

        const finalEffectiveTime = item.effectiveTime || effectiveTime
        const finalExpireTime = item.expireTime || expireTime
        if (!this.validateTimeOrder(finalEffectiveTime, finalExpireTime)) {
          this.$message.warning(`${this.assetTypeLabel} ${item.assetName || item.assetId} 的失效时间不合法`)
          return false
        }
      }

      return true
    },

    buildK8sAuthorizePayload() {
      const { effectiveTime, expireTime, authorizations } = this.k8sAuthorizationForm
      return {
        [this.subjectIdField]: this.selectedSubjectId,
        assetType: this.assetType,
        authorizations: authorizations.map(item => {
          const authorization = {
            assetId: item.assetId
          }
          const finalEffectiveTime = item.effectiveTime || effectiveTime
          const finalExpireTime = item.expireTime || expireTime

          if (finalEffectiveTime) {
            authorization.effectiveTime = finalEffectiveTime
          }
          if (finalExpireTime) {
            authorization.expireTime = finalExpireTime
          }

          if (item.allScopes) {
            authorization.allScopes = true
          } else {
            authorization.scopeType = this.scopeTypeValue
            authorization.scopeValues = item.scopeValues || []
          }

          return authorization
        })
      }
    },

    async submitK8sAuthorization() {
      if (!this.validateK8sAuthorizationForm()) {
        return
      }

      this.authorizationSubmitting = true
      try {
        const payload = this.buildK8sAuthorizePayload()
        const { data: res } = await this.$api.authorizeAssetsToGroup(payload)

        if (res.code === 200) {
          this.$message.success('授权成功')
          this.k8sAuthorizationDialogVisible = false
          this.fetchAssetList()
        } else {
          this.$message.error(res.message || '授权失败')
        }
      } catch (error) {
        this.$message.error(error.message || '授权失败')
      } finally {
        this.authorizationSubmitting = false
      }
    },

    async loadHostsSshAuths(hosts) {
      return Promise.all(hosts.map(async host => {
        if (Array.isArray(host.sshAuths) && host.sshAuths.length > 0) {
          return host
        }

        try {
          const { data: res } = await this.$api.getHostSshAuths(host.id || host.hostId)
          const data = res?.data || res || {}
          return {
            ...host,
            sshAuths: data.auths || data.sshAuths || data.list || []
          }
        } catch (error) {
          this.$message.warning(`主机 ${host.hostName || host.name || host.id} 登录账号加载失败`)
          return {
            ...host,
            sshAuths: []
          }
        }
      }))
    },

    resetAuthorizationDialog() {
      this.authorizationSubmitting = false
      this.authorizationForm = createAuthorizationForm([])
    },

    clearDefaultLifecycle() {
      this.authorizationForm.effectiveTime = ''
      this.authorizationForm.expireTime = ''
    },

    clearRowOverrides() {
      this.authorizationForm.authorizations = this.authorizationForm.authorizations.map(item => ({
        ...item,
        effectiveTime: '',
        expireTime: ''
      }))
    },

    getHostId(row) {
      return row.hostId || row.id
    },

    getHostName(row) {
      return row.hostName || row.name || '-'
    },

    getAssetId(row) {
      return row.assetId || row.asset?.id || row.id || row.clusterId || row.projectId
    },

    getAssetName(row) {
      const assetId = this.getAssetId(row)
      const option = this.scopedAssetOptions.find(item => String(item.id) === String(assetId || ''))
      const optionName = this.isLogAsset ? this.getLogProjectName(option) : this.getClusterName(option)
      return row.assetName || row.asset?.name || row.projectName || row.clusterName || row.name || optionName || '-'
    },

    isAllScopes(row) {
      if (row.allScopes === true || row.scopeAll === true) {
        return true
      }

      const scopeValues = this.getScopeValues(row)
      return !row.scopeType && scopeValues.length === 0
    },

    getScopeValues(row) {
      const values = row.scopeValues || row.scopes || row.logStores || row.logStoreIds || row.storeIds || row.namespaces || row.namespaceValues || row.scopeValue || row.namespace || row.storeId || []
      if (Array.isArray(values)) {
        return values
          .map(item => {
            if (item && typeof item === 'object') {
              return item.scopeValue || item.scope_value || item.value || item.id || item.storeId || item.name || item.label || ''
            }
            return String(item)
          })
          .filter(Boolean)
      }

      if (typeof values === 'string' || typeof values === 'number') {
        return String(values).split(',').map(item => item.trim()).filter(Boolean)
      }

      return []
    },

    getScopeNameValues(row) {
      const values = row.scopeNames || row.scope_names || row.scopeName || row.scope_name || row.logStoreNames || row.log_store_names || row.storeNames || row.store_names || []
      if (Array.isArray(values)) {
        return values
          .map(item => {
            if (item && typeof item === 'object') {
              return item.scopeName || item.scope_name || item.label || item.name || item.storeName || item.store_name || ''
            }
            return String(item)
          })
          .filter(Boolean)
      }

      if (typeof values === 'string' || typeof values === 'number') {
        return String(values).split(',').map(item => item.trim()).filter(Boolean)
      }

      return []
    },

    getScopeTagLabel(row, scope, index = 0) {
      const normalizedScope = String(scope)
      const scopeNames = this.getScopeNameValues(row)
      if (scopeNames[index]) {
        return scopeNames[index]
      }

      const sourceValues = row.scopeValues || row.scopes || row.logStores || []
      if (Array.isArray(sourceValues)) {
        const sourceItem = sourceValues[index]
        if (sourceItem && typeof sourceItem === 'object') {
          const objectLabel = sourceItem.scopeName || sourceItem.scope_name || sourceItem.label || sourceItem.name || sourceItem.storeName || sourceItem.store_name
          if (objectLabel) {
            return objectLabel
          }
        }
      }

      const matchedScope = (row.namespaceOptions || []).find(item => {
        return [item.name, item.value, item.id, item.scopeValue, item.scope_value].some(value => String(value) === normalizedScope)
      })
      return matchedScope?.label || matchedScope?.scopeName || matchedScope?.scope_name || matchedScope?.name || normalizedScope
    },

    normalizeTimeValue(value) {
      if (value === null || value === undefined) {
        return ''
      }

      const normalizedValue = String(value).trim()
      return EMPTY_TIME_VALUES.has(normalizedValue) ? '' : normalizedValue
    },

    getRowEffectiveTime(row) {
      return this.normalizeTimeValue(
        row.effectiveTime || row.authorizedEffectiveTime || row.effectiveAt
      )
    },

    getRowExpireTime(row) {
      return this.normalizeTimeValue(
        row.expireTime || row.authExpireTime || row.authorizedExpireTime || row.authorizationExpireTime || row.expireAt
      )
    },

    parseTime(value) {
      const normalizedValue = this.normalizeTimeValue(value)
      if (!normalizedValue) {
        return moment.invalid()
      }

      return moment(normalizedValue, DATE_TIME_FORMATS, true)
    },

    validateTimeOrder(effectiveTime, expireTime) {
      if (!expireTime) {
        return true
      }

      const expireMoment = this.parseTime(expireTime)
      if (!expireMoment.isValid() || !expireMoment.isAfter(moment())) {
        return false
      }

      if (!effectiveTime) {
        return true
      }

      const effectiveMoment = this.parseTime(effectiveTime)
      return effectiveMoment.isValid() && expireMoment.isAfter(effectiveMoment)
    },

    validateAuthorizationForm() {
      const { effectiveTime, expireTime, authorizations } = this.authorizationForm

      if (!this.validateTimeOrder(effectiveTime, expireTime)) {
        this.$message.warning('默认失效时间必须晚于当前时间，且晚于默认生效时间')
        return false
      }

      const hostIdSet = new Set()
      for (const item of authorizations) {
        if (hostIdSet.has(item.hostId)) {
          this.$message.warning(`主机 ${item.hostId} 重复，请检查授权列表`)
          return false
        }
        hostIdSet.add(item.hostId)

        if ((item.sshAuths?.length || 0) > 1 && (!item.authIds || item.authIds.length === 0)) {
          this.$message.warning(`${item.hostName || item.hostId} 有多个账号，请选择授权账号`)
          return false
        }

        const finalEffectiveTime = item.effectiveTime || effectiveTime
        const finalExpireTime = item.expireTime || expireTime
        if (!this.validateTimeOrder(finalEffectiveTime, finalExpireTime)) {
          this.$message.warning(`主机 ${item.hostName || item.hostId} 的失效时间不合法`)
          return false
        }
      }

      return true
    },

    buildAuthorizePayload() {
      const { effectiveTime, expireTime, authorizations } = this.authorizationForm
      return {
        [this.subjectIdField]: this.selectedSubjectId,
        effectiveTime: effectiveTime || null,
        expireTime: expireTime || null,
        authorizations: authorizations.map(item => ({
          hostId: item.hostId,
          authIds: item.authIds || [],
          effectiveTime: item.effectiveTime || null,
          expireTime: item.expireTime || null
        }))
      }
    },

    async submitAuthorization() {
      if (!this.validateAuthorizationForm()) {
        return
      }

      this.authorizationSubmitting = true
      try {
        const payload = this.buildAuthorizePayload()
        const { data: res } = await this.$api.authorizeHostsToGroup(payload)

        if (res.code === 200) {
          this.$message.success('授权成功')
          this.authorizationDialogVisible = false
          this.fetchAssetList()
        } else {
          this.$message.error(res.message || '授权失败')
        }
      } catch (error) {
        this.$message.error(error.message || '授权失败')
      } finally {
        this.authorizationSubmitting = false
      }
    },

    handleRevokeSingle(row) {
      const assetName = this.isHostAsset ? this.getHostName(row) : this.getAssetName(row)
      this.$confirm(
        `确定要取消${this.assetTypeLabel} ${assetName} 的授权吗？`,
        '提示',
        {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning',
          customClass: 'host-access-confirm-dialog',
          confirmButtonClass: 'el-button--danger'
        }
      ).then(() => {
        this.isHostAsset
          ? this.revokeHostAuthorization([this.getHostId(row)])
          : this.revokeAssetAuthorization([row])
      }).catch(() => {})
    },

    handleBatchRevoke() {
      this.$confirm(
        `确定要取消选中的 ${this.selectedAssets.length} 个${this.assetTypeLabel}授权吗？`,
        '提示',
        {
          confirmButtonText: '确定',
          cancelButtonText: '取消',
          type: 'warning',
          customClass: 'host-access-confirm-dialog',
          confirmButtonClass: 'el-button--danger'
        }
      ).then(() => {
        if (this.isHostAsset) {
          const hostIds = this.selectedAssets.map(item => this.getHostId(item))
          this.revokeHostAuthorization(hostIds)
          return
        }

        this.revokeAssetAuthorization(this.selectedAssets)
      }).catch(() => {})
    },

    async revokeHostAuthorization(hostIds) {
      try {
        const payload = {
          [this.subjectIdField]: this.selectedSubjectId,
          hostIds
        }

        const { data: res } = await this.$api.revokeHostsFromGroup(payload)

        if (res.code === 200) {
          this.$message.success('取消授权成功')
          this.fetchAssetList()
        } else {
          this.$message.error(res.message || '取消授权失败')
        }
      } catch (error) {
        this.$message.error('取消授权失败')
      }
    },

    buildAssetRevokePayload(row) {
      const payload = {
        [this.subjectIdField]: this.selectedSubjectId,
        assetType: this.assetType,
        assetId: this.getAssetId(row)
      }

      if (this.isAllScopes(row)) {
        payload.allScopes = true
      } else {
        payload.scopeType = row.scopeType || this.scopeTypeValue
        payload.scopeValues = this.getScopeValues(row)
      }

      return payload
    },

    async revokeAssetAuthorization(rows) {
      try {
        const requests = rows.map(row => {
          const payload = this.buildAssetRevokePayload(row)
          return this.$api.revokeAssetsFromGroup(payload)
        })

        const responses = await Promise.all(requests)
        const failed = responses.find(response => response.data?.code !== 200)
        if (failed) {
          this.$message.error(failed.data?.message || '取消授权失败')
          return
        }

        this.$message.success('取消授权成功')
        this.fetchAssetList()
      } catch (error) {
        this.$message.error('取消授权失败')
      }
    },

    formatEffectiveTime(value) {
      return value || '立即生效'
    },

    formatExpireTime(value) {
      return value || '长期有效'
    },

    getLifecycleStatus(row) {
      const statusMap = {
        pending: {
          text: '未生效',
          type: 'warning'
        },
        active: {
          text: '生效中',
          type: 'success'
        },
        expired: {
          text: '已过期',
          type: 'danger'
        },
        revoked: {
          text: '已回收',
          type: 'info'
        }
      }

      if (row.authorizationStatus && statusMap[row.authorizationStatus]) {
        return statusMap[row.authorizationStatus]
      }

      const effectiveTime = this.getRowEffectiveTime(row)
      const expireTime = this.getRowExpireTime(row)
      const now = moment()

      if (expireTime) {
        const expireMoment = this.parseTime(expireTime)
        if (expireMoment.isValid() && !expireMoment.isAfter(now)) {
          return {
            text: '已过期',
            type: 'danger'
          }
        }
      }

      if (effectiveTime) {
        const effectiveMoment = this.parseTime(effectiveTime)
        if (effectiveMoment.isValid() && effectiveMoment.isAfter(now)) {
          return {
            text: '未生效',
            type: 'warning'
          }
        }
      }

      return {
        text: '生效中',
        type: 'success'
      }
    },

    getAuthorizationSummary(row) {
      const effectiveTime = row.effectiveTime || this.authorizationForm.effectiveTime
      const expireTime = row.expireTime || this.authorizationForm.expireTime
      return `${this.formatEffectiveTime(effectiveTime)} / ${this.formatExpireTime(expireTime)}`
    },

    getHostAuthorizationSubtext(row) {
      const address = row.privateIp || row.sshIp || '-'
      return `ID: ${row.hostId} / ${address}`
    },

    getK8sAuthorizationSummary(row) {
      const effectiveTime = row.effectiveTime || this.k8sAuthorizationForm.effectiveTime
      const expireTime = row.expireTime || this.k8sAuthorizationForm.expireTime
      const scopeText = row.allScopes
        ? this.allScopesText
        : `${this.scopeTypeLabel}：${(row.scopeValues || []).map((scope, index) => this.getScopeTagLabel(row, scope, index)).join(', ') || '未选择'}`
      return `${scopeText} / ${this.formatEffectiveTime(effectiveTime)} / ${this.formatExpireTime(expireTime)}`
    },

    getStatusText(status) {
      const statusMap = {
        1: '认证成功',
        2: '未认证',
        3: '认证失败'
      }
      return statusMap[status] || '未知'
    },

    getEnabledStatusText(status) {
      const statusMap = {
        1: '启用',
        2: '禁用',
        0: '禁用'
      }
      return statusMap[status] || ''
    },

    getStatusType(status) {
      const statusMap = {
        1: 'success',
        2: 'info',
        3: 'danger'
      }
      return statusMap[status] || 'info'
    }
  }
}
</script>

<style lang="less" scoped>
.authorization-management {
  min-height: 100vh;
  padding: 8px;
  background: #eef1f6;
}

.auth-card {
  border: 1px solid rgba(148, 163, 184, 0.2);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 12px 32px rgba(15, 23, 42, 0.08);

  :deep(.el-card__header) {
    padding: 20px 24px;
    border-bottom: 1px solid #e2e8f0;
  }

  :deep(.el-card__body) {
    padding: 20px 24px 24px;
  }
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.auth-hero {
  display: flex;
  align-items: center;
  gap: 14px;
}

.auth-hero__icon {
  width: 40px;
  height: 40px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  color: #2f80ed;
  background: #eef5ff;
  border: 1px solid #d7e8ff;
  font-size: 20px;
}

.auth-hero__content {
  min-width: 0;
}

.auth-hero__subtitle {
  margin: 6px 0 0;
  color: #667085;
  font-size: 13px;
  line-height: 1.5;
}

.title {
  font-size: 18px;
  font-weight: 600;
  color: #0f172a;
}

.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
  padding: 16px;
  border: 1px solid #e2e8f0;
  border-radius: 14px;
  background: linear-gradient(180deg, #f8fafc 0%, #f1f5f9 100%);
}

.toolbar-left,
.toolbar-right {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
}

.auth-summary-row {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 10px;
}

.auth-summary-card {
  min-height: 58px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 6px;
  padding: 10px 14px;
  border: 1px solid #e5edf7;
  border-radius: 8px;
  background: #ffffff;
}

.auth-summary-card.active {
  border-color: #b9d7ff;
  background: #f5f9ff;
}

.auth-summary-card__label {
  color: #667085;
  font-size: 12px;
  line-height: 1;
}

.auth-summary-card strong {
  min-width: 0;
  color: #1d2433;
  font-size: 20px;
  line-height: 1.1;
  font-weight: 650;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.subject-type-group :deep(.el-radio-button__inner),
.asset-type-group :deep(.el-radio-button__inner) {
  min-width: 84px;
}

.subject-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.subject-option__label {
  color: #0f172a;
}

.subject-option__meta {
  color: #64748b;
  font-size: 12px;
}

.page-alert {
  margin-bottom: 18px;
}

.host-list-section,
.asset-list-section {
  margin-top: 8px;
}

.section-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.section-title {
  margin: 0;
  color: #0f172a;
  font-size: 17px;
  font-weight: 600;
}

.section-subtitle {
  margin: 6px 0 0;
  color: #64748b;
  font-size: 13px;
}

.host-meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.host-name {
  color: #0f172a;
  font-weight: 600;
}

.host-subtext {
  color: #64748b;
  font-size: 12px;
}

.scope-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.scope-config {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.scope-mode {
  flex-shrink: 0;
}

.time-text,
.summary-text {
  color: #334155;
}

.modern-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.dialog-alert {
  margin-bottom: 18px;
}

.config-form {
  margin-bottom: 8px;
}

.config-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.config-actions__title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #0f172a;
  font-size: 15px;
  font-weight: 600;
}

.config-actions__buttons {
  display: flex;
  align-items: center;
  gap: 8px;
}

.dialog-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.footer-tip {
  color: #64748b;
  font-size: 13px;
}

.footer-buttons {
  display: flex;
  gap: 8px;
}

.modern-btn {
  min-width: 96px;
}

.lifecycle-dialog :deep(.el-dialog) {
  border-radius: 16px;
}

.lifecycle-dialog :deep(.el-dialog__body) {
  padding-top: 12px;
}

@media (max-width: 768px) {
  .auth-card {
    :deep(.el-card__header) {
      padding: 16px;
    }

    :deep(.el-card__body) {
      padding: 16px;
    }
  }

  .toolbar,
  .auth-summary-row,
  .section-header,
  .config-actions,
  .dialog-footer {
    flex-direction: column;
    align-items: stretch;
  }

  .modern-pagination {
    justify-content: flex-start;
  }

  .footer-buttons {
    justify-content: flex-end;
  }

  .auth-summary-row {
    grid-template-columns: 1fr;
  }
}

/* DESIGN.md compact system page overrides */
.authorization-management {
  --sys-page-offset: 136px;
  --sys-text-primary: #1d2433;
  --sys-text-secondary: #4b5565;
  --sys-text-muted: #7a8699;
  --sys-border: #e3eaf3;
  --sys-border-soft: #edf2f7;
  --sys-surface: #ffffff;
  --sys-surface-soft: #f7f9fc;
  --sys-brand: #2f80ed;
  --sys-success: #40b66b;
  --sys-warning: #f59e0b;
  --sys-danger: #ef5b63;
  box-sizing: border-box;
  height: calc(100vh - var(--sys-page-offset));
  min-height: 0;
  padding: 8px;
  overflow: hidden;
  background: #f3f6fb;
  color: var(--sys-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

:global(.home-container .el-main:has(.authorization-management)) {
  overflow-y: hidden;
}

.auth-card {
  position: relative;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: 8px;
  border: 1px solid var(--sys-border);
  background: var(--sys-surface);
  box-shadow: 0 18px 40px rgba(20, 36, 64, 0.06);
}

.auth-card::before {
  display: none;
}

.auth-card :deep(.el-card__header) {
  position: relative;
  z-index: 1;
  padding: 18px 20px 14px;
  border-bottom: 1px solid var(--sys-border-soft);
  background: linear-gradient(180deg, #ffffff 0%, #fbfdff 100%);
}

.auth-card :deep(.el-card__body) {
  position: relative;
  z-index: 1;
  min-height: 0;
  display: flex;
  flex: 1;
  flex-direction: column;
  padding: 12px 14px 12px;
  overflow: hidden;
}

.card-header {
  margin: 0;
}

.card-header::before {
  content: '';
  display: none;
}

.auth-hero::before {
  content: 'System Center';
  position: absolute;
  top: -2px;
  left: 0;
  color: var(--sys-brand);
  font-size: 12px;
  font-weight: 700;
}

.auth-hero {
  position: relative;
  padding-top: 14px;
}

.title {
  display: block;
  color: var(--sys-text-primary);
  font-size: 22px;
  line-height: 1.25;
  font-weight: 700;
}

.auth-hero__subtitle {
  color: var(--sys-text-secondary);
}

.toolbar {
  flex-shrink: 0;
  gap: 12px;
  margin-bottom: 10px;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid var(--sys-border);
  background: var(--sys-surface);
}

.toolbar-left,
.toolbar-right {
  gap: 8px;
}

.asset-type-group {
  flex-shrink: 0;
}

.asset-type-group :deep(.el-radio-button__inner) {
  height: 34px;
  min-width: 86px;
  padding: 0 16px;
  line-height: 32px;
  border-color: #d9e2ef;
  background: #ffffff;
  color: var(--sys-text-secondary);
  font-weight: 600;
}

.asset-type-group :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) {
  border-color: var(--sys-brand);
  background: var(--sys-brand);
  color: #ffffff;
  box-shadow: none;
}

.auth-summary-row {
  flex-shrink: 0;
}

.auth-summary-card {
  border-color: var(--sys-border);
  background: var(--sys-surface);
  box-shadow: 0 1px 0 rgba(17, 24, 39, 0.02);
}

.auth-summary-card.active {
  border-color: #bed8ff;
  background: #f6faff;
}

.auth-summary-card__label {
  color: var(--sys-text-muted);
}

.auth-summary-card strong {
  color: var(--sys-text-primary);
}

.page-alert {
  flex-shrink: 0;
  margin-bottom: 10px;
  border-radius: 8px;
  border: 1px solid #d7e8ff;
  background: #f5f9ff;
  color: var(--sys-text-secondary);
}

.host-list-section,
.asset-list-section {
  min-height: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  margin-top: 0;
  overflow: hidden;
}

.section-header {
  flex-shrink: 0;
  align-items: center;
  margin-bottom: 10px;
  padding: 0 2px;
}

.section-title {
  color: var(--sys-text-primary);
  font-size: 16px;
  line-height: 1.35;
  font-weight: 700;
}

.section-subtitle {
  margin-top: 4px;
  color: var(--sys-text-muted);
  line-height: 1.4;
}

.modern-table {
  flex: 1;
  min-height: 0;
  border-radius: 8px;
  border: 1px solid var(--sys-border);
  overflow: hidden;
  color: var(--sys-text-secondary);
  font-size: 13px;
}

.modern-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.modern-table :deep(th.el-table__cell) {
  height: 40px;
  padding: 7px 0;
  background: var(--sys-surface-soft) !important;
  color: #667085;
  border-bottom: 1px solid var(--sys-border);
  font-size: 12px;
  font-weight: 700;
}

.modern-table :deep(td.el-table__cell) {
  height: 56px;
  padding: 8px 0;
  border-bottom: 1px solid var(--sys-border-soft);
}

.modern-table :deep(.el-table__row) {
  transition: background-color 180ms ease;
}

.modern-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: #f7fbff !important;
}

.modern-table :deep(.el-button--danger) {
  padding: 0;
  border: none;
  background: transparent;
  color: var(--sys-danger);
  font-size: 12px;
  font-weight: 600;
}

.modern-pagination {
  flex-shrink: 0;
  margin-top: 10px;
}

.modern-btn,
.el-button {
  min-height: 34px;
  border-radius: 8px;
  font-weight: 600;
  box-shadow: none;
}

.el-button--primary {
  border-color: transparent;
  background: var(--sys-brand);
}

.el-button--primary:hover,
.el-button--primary:focus {
  background: #1f6fd6;
}

.el-button--danger {
  border-color: transparent;
  background: #ff9aa0;
}

.el-input :deep(.el-input__wrapper),
.el-select :deep(.el-input__wrapper),
.el-date-editor :deep(.el-input__wrapper) {
  min-height: 34px;
  border: 1px solid #d9e2ef;
  border-radius: 8px;
  background: #ffffff;
  box-shadow: none;
}

.el-input :deep(.el-input__wrapper.is-focus),
.el-select :deep(.el-input__wrapper.is-focus),
.el-date-editor :deep(.el-input__wrapper.is-focus) {
  border-color: #9bc7ff;
  box-shadow: 0 0 0 3px rgba(47, 128, 237, 0.1);
}

.el-switch :deep(.el-switch__core) {
  min-width: 42px;
}

.host-name {
  color: #111827;
  font-weight: 700;
}

.host-subtext {
  color: #667085;
}

.scope-tags :deep(.el-tag),
.modern-table :deep(.el-tag) {
  border-radius: 6px;
  font-weight: 600;
}

.lifecycle-dialog :deep(.el-dialog) {
  border-radius: 8px;
  border: 1px solid var(--sys-border);
  background: var(--sys-surface);
  box-shadow: 0 28px 80px rgba(20, 36, 64, 0.18);
  overflow: hidden;
}

.lifecycle-dialog :deep(.el-dialog__header) {
  margin: 0;
  padding: 18px 22px 14px;
  border-bottom: 1px solid var(--sys-border-soft);
  background: #ffffff;
}

.lifecycle-dialog :deep(.el-dialog__title) {
  color: var(--sys-text-primary);
  font-size: 18px;
  font-weight: 700;
}

/* Feishu-like compact authorization workbench */
.authorization-management {
  --workbench-bg: #f2f6fb;
  --workbench-surface: #ffffff;
  --workbench-soft: #f7faff;
  --workbench-line: #e6edf6;
  --workbench-line-strong: #d8e5f5;
  --workbench-text: #202939;
  --workbench-text-subtle: #5f6c80;
  --workbench-text-muted: #8a96a8;
  --workbench-blue: #2f80ed;
  --workbench-blue-soft: #eaf3ff;
  --workbench-red: #ff5c66;
  display: flex;
  flex-direction: column;
  gap: 8px;
  height: calc(100vh - var(--sys-page-offset));
  padding: 12px;
  overflow: hidden;
  background:
    radial-gradient(circle at 16% 0%, rgba(77, 148, 255, 0.10), transparent 30%),
    linear-gradient(180deg, #f8fbff 0%, var(--workbench-bg) 100%);
  color: var(--workbench-text);
}

.auth-hero-panel {
  flex-shrink: 0;
  min-height: 64px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 16px;
  border: 1px solid var(--workbench-line);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.92);
  box-shadow: 0 10px 28px rgba(30, 64, 110, 0.06);
}

.auth-hero {
  padding-top: 0;
}

.auth-hero::before {
  display: none;
}

.auth-hero__icon {
  width: 40px;
  height: 40px;
  border-radius: 12px;
  color: #246fd6;
  background: linear-gradient(180deg, #f7fbff 0%, #eaf3ff 100%);
  border: 1px solid #d5e6ff;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.9);
}

.title {
  font-size: 20px;
  line-height: 1.25;
  font-weight: 750;
  color: #172033;
}

.auth-hero__subtitle {
  margin-top: 5px;
  color: var(--workbench-text-subtle);
  font-size: 13px;
}

.auth-summary-row {
  flex-shrink: 0;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
  margin-bottom: 0;
}

.auth-summary-card {
  min-height: 58px;
  padding: 10px 16px;
  border-radius: 10px;
  border: 1px solid var(--workbench-line);
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 8px 20px rgba(28, 45, 74, 0.035);
}

.auth-summary-card.active {
  border-color: #b8d4ff;
  background: linear-gradient(180deg, #f7fbff 0%, #ffffff 100%);
  box-shadow: inset 0 0 0 1px rgba(47, 128, 237, 0.08);
}

.auth-summary-card__label {
  color: #2f3b50;
  font-size: 12px;
  font-weight: 650;
}

.auth-summary-card strong {
  color: #20232a;
  font-size: 24px;
  line-height: 1;
  font-weight: 650;
}

.auth-workbench {
  min-height: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 14px;
  overflow: hidden;
  border: 1px solid var(--workbench-line);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 16px 36px rgba(25, 45, 78, 0.07);
}

.workbench-header {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 10px;
}

.workbench-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.section-title {
  margin: 0;
  color: #1d2433;
  font-size: 15px;
  line-height: 1.35;
  font-weight: 750;
}

.section-subtitle {
  margin: 4px 0 0;
  color: #64748b;
  font-size: 12px;
}

.toolbar {
  flex-shrink: 0;
  margin-bottom: 10px;
  padding: 8px 10px;
  border: 1px solid var(--workbench-line);
  border-radius: 10px;
  background: #f9fbfe;
}

.toolbar-left,
.toolbar-right {
  gap: 8px;
}

.asset-type-group :deep(.el-radio-button__inner) {
  height: 32px;
  min-width: 82px;
  padding: 0 14px;
  line-height: 30px;
  border-color: #dce6f2;
  background: #ffffff;
  color: #5c6a7f;
  font-size: 12px;
}

.asset-type-group :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) {
  border-color: #b8d8ff;
  background: #eaf3ff;
  color: #256bd8;
  box-shadow: none;
}

.page-alert {
  flex-shrink: 0;
  margin-bottom: 10px;
  border-radius: 8px;
  border-color: #d8e9ff;
  background: #f6fbff;
}

.asset-list-section {
  min-height: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.asset-table {
  flex: 1;
}

.modern-table {
  border-radius: 10px;
  border-color: var(--workbench-line);
  color: #4b5565;
  font-size: 13px;
}

.modern-table :deep(th.el-table__cell) {
  height: 38px;
  padding: 6px 0;
  background: #f4f7fb !important;
  color: #445168;
  border-bottom-color: #dfe8f3;
  font-size: 12px;
  font-weight: 750;
}

.modern-table :deep(td.el-table__cell) {
  height: 52px;
  padding: 7px 0;
  border-bottom-color: #edf2f7;
}

.modern-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: #f8fbff !important;
}

.modern-table :deep(.el-table__fixed-right) {
  box-shadow: -10px 0 18px rgba(31, 45, 61, 0.08);
}

.modern-pagination {
  flex-shrink: 0;
  margin-top: 10px;
}

.modern-btn,
.el-button {
  min-height: 32px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 650;
}

.el-button--primary {
  background: #4095ff;
  border-color: #4095ff;
}

.el-button--primary:hover,
.el-button--primary:focus {
  background: #2f80ed;
  border-color: #2f80ed;
}

.ghost-btn {
  border-color: #d6e6fb;
  background: #f7fbff;
  color: #2b6ed3;
}

.danger-outline {
  border-color: #ffd9dc;
  background: #fff7f8;
  color: var(--workbench-red);
}

.danger-outline:hover,
.danger-outline:focus {
  border-color: #ffb9bf;
  background: #fff0f1;
  color: #f04450;
}

.modern-table :deep(.el-button--danger) {
  color: #f04450;
}

.modern-select {
  max-width: 100%;
}

.el-input :deep(.el-input__wrapper),
.el-select :deep(.el-input__wrapper),
.el-date-editor :deep(.el-input__wrapper) {
  min-height: 32px;
  border-color: #dce6f2;
  border-radius: 9px;
}

.el-input :deep(.el-input__wrapper.is-focus),
.el-select :deep(.el-input__wrapper.is-focus),
.el-date-editor :deep(.el-input__wrapper.is-focus) {
  border-color: #9ac8ff;
  box-shadow: 0 0 0 3px rgba(64, 149, 255, 0.10);
}

.host-name {
  color: #2b3344;
  font-weight: 700;
}

.host-subtext,
.time-text,
.summary-text {
  color: #667085;
}

.scope-tags :deep(.el-tag),
.modern-table :deep(.el-tag) {
  border-radius: 5px;
  font-size: 12px;
  font-weight: 650;
}

.auth-workbench > .el-empty {
  min-height: 0;
  flex: 1;
  justify-content: center;
}

@media (max-width: 900px) {
  .authorization-management {
    height: auto;
    min-height: calc(100vh - var(--sys-page-offset));
    overflow: auto;
  }

  .auth-hero-panel,
  .workbench-header,
  .toolbar {
    align-items: stretch;
    flex-direction: column;
  }

  .auth-summary-row {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .workbench-actions,
  .toolbar-left,
  .toolbar-right {
    width: 100%;
  }
}

@media (max-width: 560px) {
  .auth-summary-row {
    grid-template-columns: 1fr;
  }

  .modern-select {
    width: 100% !important;
  }
}

/* Final Feishu workbench pass: low-noise surfaces, white canvas, compact controls. */
.authorization-management {
  --fs-blue: #3370ff;
  --fs-blue-hover: #245bdb;
  --fs-blue-soft: #eef5ff;
  --fs-text-strong: #1f2329;
  --fs-text: #2b3340;
  --fs-text-secondary: #56657a;
  --fs-text-muted: #8f9aae;
  --fs-border: #dfe8f3;
  --fs-border-soft: #edf2f7;
  --fs-surface: #ffffff;
  --fs-surface-soft: #fafcff;
  --fs-danger: #f54a45;
  gap: 10px;
  padding: 20px 20px 16px;
  background: #ffffff !important;
}

:global(.home-container .el-main:has(.authorization-management)) {
  background: #ffffff !important;
}

.auth-hero-panel,
.auth-summary-card,
.auth-workbench {
  background: var(--fs-surface);
  border: 1px solid var(--fs-border);
  box-shadow: 0 1px 2px rgba(31, 35, 41, 0.03);
}

.auth-hero-panel {
  min-height: 66px;
  padding: 14px 16px;
  border-radius: 10px;
}

.auth-hero {
  gap: 12px;
}

.auth-hero__icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  color: var(--fs-blue);
  background: linear-gradient(180deg, #f7fbff 0%, var(--fs-blue-soft) 100%);
  border: 1px solid #d7e8ff;
  box-shadow: none;
}

.title {
  color: var(--fs-text-strong);
  font-size: 20px;
  font-weight: 700;
}

.auth-hero__subtitle {
  margin-top: 5px;
  color: var(--fs-text-secondary);
  font-size: 13px;
}

.ghost-btn {
  min-width: 96px;
  border-color: #cfe0f7;
  background: #f8fbff;
  color: #2b6ed3;
}

.ghost-btn:hover,
.ghost-btn:focus {
  border-color: #9fc5ff;
  background: #eef5ff;
  color: #1f62d0;
}

.auth-summary-row {
  gap: 10px;
}

.auth-summary-card {
  min-height: 78px;
  padding: 15px 18px 14px;
  border-radius: 10px;
}

.auth-summary-card.active {
  border-color: #a8c7ff;
  background: #ffffff;
  box-shadow: inset 0 0 0 1px rgba(51, 112, 255, 0.10);
}

.auth-summary-card__label {
  color: #243042;
  font-size: 12px;
  line-height: 1;
  font-weight: 700;
}

.auth-summary-card strong {
  margin-top: 8px;
  color: var(--fs-text-strong);
  font-size: 26px;
  line-height: 1;
  font-weight: 700;
}

.auth-workbench {
  padding: 14px 14px 12px;
  border-radius: 12px;
}

.workbench-header {
  min-height: 38px;
  margin-bottom: 10px;
}

.section-title {
  color: var(--fs-text-strong);
  font-size: 15px;
  line-height: 1.35;
  font-weight: 700;
}

.section-subtitle {
  margin-top: 4px;
  color: var(--fs-text-secondary);
  font-size: 12px;
}

.workbench-actions {
  gap: 10px;
}

.toolbar {
  min-height: 44px;
  margin-bottom: 10px;
  padding: 8px 10px;
  border-radius: 8px;
  border-color: var(--fs-border);
  background: #ffffff;
}

.toolbar-left,
.toolbar-right {
  gap: 8px;
}

.asset-type-group :deep(.el-radio-button__inner) {
  height: 32px;
  min-width: 84px;
  padding: 0 16px;
  line-height: 30px;
  border-color: #dbe4ef;
  background: #ffffff;
  color: #44546a;
  font-size: 12px;
  font-weight: 600;
}

.asset-type-group :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) {
  border-color: #3b91ff;
  background: #3b91ff;
  color: #ffffff;
  box-shadow: none;
}

.page-alert {
  margin-bottom: 10px;
  border-radius: 8px;
  border-color: #d7e5f7;
  background: #fbfdff;
  color: var(--fs-text-secondary);
}

.page-alert :deep(.el-alert__title) {
  color: var(--fs-text-secondary);
  font-size: 12px;
  line-height: 1.4;
}

.modern-table {
  border-radius: 8px;
  border-color: var(--fs-border);
  color: var(--fs-text);
}

.modern-table :deep(.el-table__header-wrapper th.el-table__cell),
.modern-table :deep(th.el-table__cell) {
  height: 39px;
  padding: 6px 0;
  background: #f6f9fd !important;
  color: #2d3b4f;
  border-bottom: 1px solid var(--fs-border);
  font-size: 12px;
  font-weight: 700;
}

.modern-table :deep(td.el-table__cell) {
  height: 50px;
  padding: 7px 0;
  border-bottom: 1px solid var(--fs-border-soft);
  background: #ffffff;
}

.modern-table :deep(.el-table__row--striped td.el-table__cell) {
  background: #fcfdff;
}

.modern-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: #f7fbff !important;
}

.modern-table :deep(.el-table__fixed-right) {
  box-shadow: -8px 0 18px rgba(31, 35, 41, 0.07);
}

.modern-table :deep(.el-button--danger) {
  color: var(--fs-danger);
  font-size: 12px;
  font-weight: 600;
}

.modern-btn,
.el-button {
  min-height: 32px;
  padding: 8px 14px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
}

.el-button--primary {
  border-color: #3b91ff;
  background: #3b91ff;
}

.el-button--primary:hover,
.el-button--primary:focus {
  border-color: var(--fs-blue);
  background: var(--fs-blue);
}

.danger-outline {
  border-color: #f6c9c7;
  background: #fff8f8;
  color: var(--fs-danger);
}

.danger-outline:hover,
.danger-outline:focus {
  border-color: #f7aaa6;
  background: #fff1f0;
  color: #d83931;
}

.el-input :deep(.el-input__wrapper),
.el-select :deep(.el-input__wrapper),
.el-date-editor :deep(.el-input__wrapper) {
  min-height: 32px;
  border: 1px solid #dbe4ef;
  border-radius: 7px;
  background: #ffffff;
  box-shadow: none;
}

.el-input :deep(.el-input__wrapper:hover),
.el-select :deep(.el-input__wrapper:hover),
.el-date-editor :deep(.el-input__wrapper:hover) {
  border-color: #b8c6d9;
}

.el-input :deep(.el-input__wrapper.is-focus),
.el-select :deep(.el-input__wrapper.is-focus),
.el-date-editor :deep(.el-input__wrapper.is-focus) {
  border-color: #80b7ff;
  box-shadow: 0 0 0 2px rgba(51, 112, 255, 0.12);
}

.el-switch :deep(.el-switch__core) {
  min-width: 42px;
  height: 22px;
}

.host-name {
  color: #111827;
  font-weight: 700;
}

.host-subtext,
.time-text,
.summary-text {
  color: #56657a;
}

.scope-tags :deep(.el-tag),
.modern-table :deep(.el-tag) {
  height: 22px;
  padding: 0 8px;
  border-radius: 5px;
  font-size: 12px;
  font-weight: 600;
}

.modern-pagination {
  margin-top: 10px;
  padding-right: 2px;
}

.lifecycle-dialog :deep(.el-dialog) {
  border-radius: 10px;
  border: 1px solid var(--fs-border);
  background: #ffffff;
  box-shadow: 0 18px 48px rgba(31, 35, 41, 0.16);
}

.lifecycle-dialog :deep(.el-dialog__header) {
  padding: 16px 20px 13px;
  border-bottom: 1px solid var(--fs-border-soft);
}

.lifecycle-dialog :deep(.el-dialog__title) {
  color: var(--fs-text-strong);
  font-size: 16px;
  font-weight: 700;
}

/* Project-aligned compact admin page */
.authorization-management {
  --auth-page-height-offset: 124px;
  --auth-surface-warm: rgba(255, 253, 248, 0.96);
  --auth-surface-cool: rgba(247, 250, 255, 0.92);
  --auth-text-primary: #172133;
  --auth-text-secondary: #5f6b7c;
  --auth-text-muted: #8d97a8;
  --auth-border: rgba(214, 223, 235, 0.96);
  --auth-topbar: linear-gradient(90deg, #fef7ef 0%, #f8f8f6 52%, #edf2fd 100%);
  min-height: calc(100vh - var(--auth-page-height-offset));
  height: calc(100vh - var(--auth-page-height-offset));
  padding: 6px;
  box-sizing: border-box;
  overflow: hidden;
  background: transparent !important;
  color: var(--auth-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

:global(.home-container .el-main:has(.authorization-management)) {
  overflow-y: hidden;
  background: transparent !important;
}

.auth-page-card {
  position: relative;
  min-height: calc(100vh - var(--auth-page-height-offset) - 12px);
  max-height: calc(100vh - var(--auth-page-height-offset) - 12px);
  display: flex;
  flex-direction: column;
  padding: 28px 16px 12px;
  overflow: hidden;
  border: 1px solid var(--auth-border);
  border-radius: 24px;
  background: linear-gradient(135deg, var(--auth-surface-warm), var(--auth-surface-cool));
  box-shadow: 0 18px 48px rgba(17, 24, 39, 0.08);
}

.auth-page-card::before {
  content: "";
  position: absolute;
  inset: 0 0 auto;
  height: 24px;
  background: var(--auth-topbar);
}

.auth-page-header {
  position: relative;
  z-index: 1;
  min-height: 46px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin: -28px -16px 10px;
  padding: 8px 16px;
  border-bottom: 1px solid var(--auth-border);
  background: var(--auth-topbar);
}

.auth-title-wrap,
.auth-header-actions,
.summary-group,
.toolbar-card,
.toolbar-left,
.toolbar-right,
.workbench-actions {
  display: flex;
  align-items: center;
}

.auth-title-wrap {
  min-width: 0;
  gap: 10px;
}

.auth-hero__icon {
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  color: #4c8bf5;
  background: rgba(255, 255, 255, 0.78);
  border: 1px solid rgba(214, 223, 235, 0.96);
  box-shadow: none;
}

.title {
  margin: 0;
  color: var(--auth-text-primary);
  font-size: 20px;
  line-height: 1.15;
  font-weight: 700;
}

.auth-hero__subtitle {
  margin: 4px 0 0;
  color: var(--auth-text-secondary);
  font-size: 12px;
  line-height: 1.35;
}

.auth-header-actions {
  flex: 0 0 auto;
  justify-content: flex-end;
  gap: 10px;
}

.summary-group {
  gap: 8px;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.summary-chip {
  min-width: 70px;
  max-width: 148px;
  padding: 5px 9px;
  border: 1px solid var(--auth-border);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.82);
}

.summary-chip span {
  display: block;
  color: var(--auth-text-muted);
  font-size: 12px;
  line-height: 1;
}

.summary-chip strong {
  display: block;
  margin-top: 2px;
  overflow: hidden;
  color: var(--auth-text-primary);
  font-size: 16px;
  line-height: 1.15;
  font-weight: 700;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ghost-btn {
  min-width: 76px;
  min-height: 30px;
  padding: 6px 12px;
  border-color: rgba(214, 223, 235, 0.96);
  background: rgba(255, 255, 255, 0.76);
  color: #4c8bf5;
}

.workbench-header {
  flex: 0 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 8px;
}

.section-title {
  margin: 0;
  color: var(--auth-text-primary);
  font-size: 15px;
  line-height: 1.35;
  font-weight: 700;
}

.section-subtitle {
  margin: 3px 0 0;
  color: var(--auth-text-secondary);
  font-size: 12px;
  line-height: 1.35;
}

.workbench-actions {
  gap: 8px;
  flex-wrap: nowrap;
}

.toolbar-card {
  flex: 0 0 auto;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
  padding: 10px 12px;
  border: 1px solid var(--auth-border);
  border-radius: 14px;
  background: linear-gradient(135deg, rgba(255, 253, 248, 0.95), rgba(248, 251, 255, 0.92));
  box-shadow: 0 12px 34px rgba(17, 24, 39, 0.05);
}

.toolbar-left,
.toolbar-right {
  gap: 8px;
  flex-wrap: wrap;
}

.asset-type-group :deep(.el-radio-button__inner) {
  height: 30px;
  min-width: 74px;
  padding: 0 14px;
  line-height: 28px;
  border-color: rgba(214, 223, 235, 0.96);
  background: rgba(255, 255, 255, 0.82);
  color: #526075;
  font-size: 12px;
  font-weight: 700;
}

.asset-type-group :deep(.el-radio-button__original-radio:checked + .el-radio-button__inner) {
  border-color: transparent;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
  color: #ffffff;
  box-shadow: none;
}

.page-alert {
  flex: 0 0 auto;
  margin-bottom: 8px;
  border-radius: 12px;
  border-color: rgba(214, 223, 235, 0.96);
  background: rgba(255, 255, 255, 0.68);
}

.page-alert :deep(.el-alert__title) {
  color: var(--auth-text-secondary);
  font-size: 12px;
}

.table-shell {
  min-height: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--auth-border);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.82);
  box-shadow: 0 12px 34px rgba(17, 24, 39, 0.05);
}

.asset-table {
  min-height: 0;
  flex: 1;
}

.modern-table {
  border: none;
  border-radius: 16px 16px 0 0;
  box-shadow: none;
  color: var(--auth-text-secondary);
  font-size: 13px;
}

.modern-table :deep(.el-table__inner-wrapper::before) {
  display: none;
}

.modern-table :deep(th.el-table__cell) {
  height: 38px;
  padding: 6px 0 !important;
  background: rgba(248, 250, 255, 0.96) !important;
  color: var(--auth-text-muted) !important;
  border-bottom: 1px solid var(--auth-border);
  font-size: 12px;
  font-weight: 700 !important;
}

.modern-table :deep(td.el-table__cell) {
  height: 42px;
  padding: 6px 0 !important;
  border-bottom: 1px solid rgba(15, 23, 42, 0.04);
  color: var(--auth-text-secondary);
}

.modern-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: rgba(76, 139, 245, 0.045) !important;
}

.modern-table :deep(.el-button--danger) {
  min-height: auto;
  padding: 0;
  border: none;
  background: transparent !important;
  color: #ef6b73;
  font-size: 12px;
  font-weight: 600;
}

.modern-pagination {
  flex: 0 0 auto;
  justify-content: flex-end;
  margin: 0;
  padding: 8px 10px;
}

.modern-btn,
.el-button {
  min-height: 30px;
  border-radius: 10px;
  font-size: 12px;
  font-weight: 600;
  box-shadow: none;
  transform: none !important;
}

.el-button--primary {
  border-color: transparent;
  background: linear-gradient(180deg, #6aa1fb 0%, #5a93f6 100%);
}

.danger-outline,
.el-button--danger:not(.is-link) {
  border-color: transparent;
  background: linear-gradient(180deg, #f47f86 0%, #ef6b73 100%);
  color: #ffffff;
}

.el-input :deep(.el-input__wrapper),
.el-select :deep(.el-input__wrapper),
.el-date-editor :deep(.el-input__wrapper) {
  min-height: 30px;
  border: 1px solid rgba(214, 223, 235, 0.95);
  border-radius: 10px;
  background: rgba(252, 253, 255, 0.98);
  box-shadow: none;
}

.el-input :deep(.el-input__wrapper.is-focus),
.el-select :deep(.el-input__wrapper.is-focus),
.el-date-editor :deep(.el-input__wrapper.is-focus) {
  border-color: rgba(120, 156, 221, 0.96);
  box-shadow: 0 0 0 3px rgba(76, 139, 245, 0.08);
}

.host-name {
  color: var(--auth-text-primary);
  font-weight: 700;
}

.host-subtext,
.time-text,
.summary-text {
  color: var(--auth-text-muted);
}

.scope-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.scope-tags :deep(.el-tag),
.modern-table :deep(.el-tag) {
  border-radius: 7px;
  font-size: 12px;
  font-weight: 600;
}

.auth-page-card > .el-empty {
  min-height: 0;
  flex: 1;
  justify-content: center;
}

@media (max-width: 1100px) {
  .auth-page-header {
    align-items: flex-start;
    flex-direction: column;
  }

  .auth-header-actions {
    width: 100%;
    justify-content: space-between;
  }
}

@media (max-width: 768px) {
  .authorization-management {
    height: auto;
    overflow: auto;
  }

  .auth-page-card {
    max-height: none;
  }

  .workbench-header,
  .toolbar-card {
    align-items: stretch;
    flex-direction: column;
  }
}

.back-btn,
.back-btn:hover,
.back-btn:focus {
  border-color: transparent;
  background: linear-gradient(180deg, #f6be63 0%, #f3b24b 100%);
  color: #ffffff;
}

.back-btn :deep(.el-icon) {
  color: #ffffff;
}

.modern-table :deep(.revoke-link-btn),
.modern-table :deep(.revoke-link-btn:hover),
.modern-table :deep(.revoke-link-btn:focus) {
  min-height: auto;
  padding: 0;
  border: none;
  background: transparent !important;
  color: #ef4444 !important;
  font-size: 12px;
  font-weight: 600;
  box-shadow: none;
}

.lifecycle-dialog :deep(.el-dialog) {
  width: min(1080px, calc(100vw - 40px)) !important;
}

.lifecycle-dialog :deep(.el-dialog__body) {
  padding: 12px 18px 8px;
}

.lifecycle-dialog .dialog-alert {
  margin-bottom: 10px;
  border-radius: 10px;
  background: rgba(248, 251, 255, 0.92);
}

.lifecycle-dialog .dialog-alert :deep(.el-alert__title) {
  color: var(--auth-text-secondary);
  font-size: 12px;
  line-height: 1.45;
}

.lifecycle-dialog .config-form {
  margin-bottom: 6px;
}

.lifecycle-dialog .config-form :deep(.el-form-item) {
  margin-bottom: 12px;
}

.lifecycle-dialog .config-actions {
  margin-bottom: 8px;
  padding: 8px 10px;
  border: 1px solid var(--auth-border);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.72);
}

.host-lifecycle-table,
.scoped-asset-table {
  border-radius: 12px;
}

.host-lifecycle-table :deep(td.el-table__cell),
.scoped-asset-table :deep(td.el-table__cell) {
  height: 54px;
  vertical-align: top;
}

.host-lifecycle-table .host-subtext,
.scoped-asset-table .host-subtext {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.scoped-asset-table .scope-config {
  gap: 6px;
}

.scoped-asset-table .scope-mode :deep(.el-radio-button__inner) {
  height: 28px;
  min-width: 86px;
  padding: 0 12px;
  line-height: 26px;
  font-size: 12px;
}

.host-lifecycle-table :deep(.el-select),
.host-lifecycle-table :deep(.el-date-editor.el-input),
.host-lifecycle-table :deep(.el-date-editor.el-input__wrapper),
.scoped-asset-table :deep(.el-select),
.scoped-asset-table :deep(.el-date-editor.el-input),
.scoped-asset-table :deep(.el-date-editor.el-input__wrapper) {
  width: 100% !important;
}

.host-lifecycle-table :deep(.el-select__wrapper),
.host-lifecycle-table :deep(.el-input__wrapper),
.scoped-asset-table :deep(.el-select__wrapper),
.scoped-asset-table :deep(.el-input__wrapper) {
  min-height: 30px;
  border-radius: 9px;
}

.host-lifecycle-table :deep(.el-select__tags-text),
.scoped-asset-table :deep(.el-select__tags-text) {
  max-width: 116px;
}

/* Content-driven height: keep the page compact when the table has few rows. */
.authorization-management {
  height: auto !important;
  min-height: 0 !important;
  overflow: visible !important;
}

:global(.home-container .el-main:has(.authorization-management)) {
  overflow-y: auto !important;
}

.auth-page-card {
  height: auto !important;
  min-height: 0 !important;
  max-height: none !important;
  overflow: visible !important;
}

.asset-list-section,
.table-shell {
  flex: 0 0 auto !important;
  min-height: 0 !important;
  overflow: visible !important;
}

.asset-table {
  flex: 0 0 auto !important;
  min-height: 0 !important;
}

.asset-table :deep(.el-table__empty-block) {
  min-height: 96px;
}

.modern-pagination {
  flex: 0 0 auto !important;
}

/* DESIGN.md final bastion authorization overrides */
:global(.home-container .el-main:has(.authorization-management)) {
  padding-top: 10px;
  padding-right: 10px;
  padding-left: 10px;
  overflow-y: hidden !important;
}

.authorization-management {
  --ops-text-heading: #0f172a;
  --ops-text-primary: #172033;
  --ops-text-secondary: #53657d;
  --ops-text-muted: #64748b;
  --ops-bg-surface: #ffffff;
  --ops-bg-subtle: #f3f7fc;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  box-sizing: border-box;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: 8px;
  height: calc(100vh - 124px) !important;
  min-height: 0 !important;
  padding: 7px;
  overflow: hidden !important;
  background: transparent;
  color: var(--ops-text-primary);
}

.ops-page-header {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 10px;
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

.authorization-header-summary {
  display: flex;
  flex: 0 1 auto;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.authorization-header-summary span {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  min-width: 0;
  white-space: nowrap;
}

.authorization-header-summary em {
  color: var(--ops-text-muted);
  font-size: 12px;
  font-style: normal;
}

.authorization-header-summary strong {
  max-width: 130px;
  overflow: hidden;
  color: var(--ops-text-heading);
  font-size: 14px;
  line-height: 1;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.authorization-header-summary > i {
  width: 1px;
  height: 18px;
  flex: 0 0 1px;
  background: var(--ops-border-soft);
}

.back-btn,
.back-btn:hover,
.back-btn:focus {
  height: 30px;
  min-height: 30px;
  padding: 0 10px;
  border-color: #e6a23c;
  border-radius: 8px;
  background: #e6a23c;
  color: #ffffff;
  font-size: 12px;
  box-shadow: none;
  transform: none;
}

.auth-page-card {
  position: relative;
  display: flex;
  min-width: 0;
  min-height: 0 !important;
  height: 100% !important;
  max-height: none !important;
  flex-direction: column;
  gap: 8px;
  padding: 10px;
  overflow: hidden !important;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: var(--ops-bg-surface);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
  backdrop-filter: none;
}

.auth-page-card::before {
  display: none;
}

.workbench-header {
  flex-shrink: 0;
  min-height: 0;
  padding: 0 2px;
}

.section-title {
  font-size: 15px;
}

.section-subtitle {
  margin-top: 2px;
  font-size: 11px;
}

.workbench-actions :deep(.auth-toolbar-button.el-button),
.toolbar-right :deep(.auth-toolbar-button.el-button) {
  height: 30px;
  min-height: 30px;
  padding: 0 10px;
  border-radius: 8px;
  color: #ffffff;
  font-size: 12px;
  box-shadow: none;
}

.workbench-actions :deep(.el-button--success) {
  border-color: #67c23a;
  background: #67c23a;
}

.workbench-actions :deep(.el-button--danger) {
  border-color: #f56c6c;
  background: #f56c6c;
}

.toolbar-right :deep(.ops-refresh-button) {
  border-color: #f3c66b;
  background: #f3c66b;
}

.toolbar-card {
  flex-shrink: 0;
  margin: 0;
  padding: 7px 10px;
  border: 1px solid var(--ops-border-soft);
  border-radius: 10px;
  background: var(--ops-bg-subtle);
}

.asset-type-group :deep(.el-radio-button__inner) {
  height: 30px;
  padding: 0 12px;
  line-height: 28px;
  font-size: 12px;
}

.page-alert {
  flex-shrink: 0;
  margin: 0;
  border-radius: 8px;
}

.asset-list-section,
.table-shell {
  display: flex;
  min-width: 0;
  min-height: 0 !important;
  flex: 1 1 auto !important;
  flex-direction: column;
  overflow: hidden !important;
  border: 1px solid var(--ops-border-soft);
  border-radius: 8px;
  background: #ffffff;
}

.asset-table {
  min-height: 0 !important;
  flex: 1 1 auto !important;
}

.modern-table,
.modern-table :deep(.el-table__inner-wrapper) {
  border-radius: 8px;
}

.modern-table {
  font-size: 12px;
}

.modern-table :deep(th.el-table__cell) {
  height: 38px;
  background: var(--ops-bg-subtle) !important;
  color: var(--ops-text-secondary) !important;
  border-bottom-color: #eef3f9;
}

.modern-table :deep(td.el-table__cell) {
  height: 40px;
  padding: 5px 0 !important;
  border-bottom-color: #eef3f9;
}

.modern-table :deep(.el-table__row:hover > td.el-table__cell) {
  background: #f8fafc !important;
}

.modern-pagination {
  padding: 6px 10px;
}

.authorization-management :deep(.el-button:hover),
.authorization-management :deep(.el-button:active),
.authorization-management :deep(.el-pagination button:hover),
.authorization-management :deep(.el-pagination .el-pager li:hover) {
  transform: none !important;
}

:global(.authorization-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  width: min(1080px, calc(100vw - 32px)) !important;
  max-width: calc(100vw - 32px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: #ffffff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.authorization-dialog-overlay .el-dialog__header) {
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.authorization-dialog-overlay .el-dialog__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.authorization-dialog-overlay .el-dialog__body) {
  box-sizing: border-box;
  max-height: calc(100vh - 155px);
  padding: 12px 18px;
  overflow: auto;
  color: #53657d;
}

:global(.authorization-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #ffffff;
}

:global(.authorization-dialog-overlay .el-input__wrapper),
:global(.authorization-dialog-overlay .el-select__wrapper),
:global(.authorization-dialog-overlay .el-date-editor.el-input__wrapper) {
  min-height: 34px;
  border: 0;
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset;
}

:global(.authorization-dialog-overlay .config-actions) {
  border-color: #e7eef7;
  border-radius: 10px;
  background: #fbfdff;
}

:global(.authorization-dialog-overlay .modern-table),
:global(.authorization-dialog-overlay .modern-table .el-table__inner-wrapper) {
  border-radius: 8px;
}

:global(.authorization-dialog-overlay .el-dialog__footer .el-button) {
  min-height: 34px;
  border-radius: 8px;
  box-shadow: none;
}

:global(.authorization-dialog-overlay .el-dialog__footer .el-button--primary) {
  border-color: #409eff;
  background: #409eff;
  color: #ffffff;
}

:global(.authorization-dialog-overlay .el-button:hover),
:global(.authorization-dialog-overlay .el-button:active) {
  transform: none;
}

:global(.host-access-confirm-dialog.el-message-box) {
  --el-messagebox-border-radius: 15px;
  max-width: calc(100vw - 32px);
  padding: 0;
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-messagebox-border-radius) !important;
  background: #ffffff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.host-access-confirm-dialog .el-message-box__header) {
  padding: 16px 18px 10px;
  border-bottom: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.host-access-confirm-dialog .el-message-box__title) {
  color: #0f172a;
  font-size: 17px;
  font-weight: 700;
}

:global(.host-access-confirm-dialog .el-message-box__content) {
  padding: 14px 18px;
  color: #53657d;
}

:global(.host-access-confirm-dialog .el-message-box__btns) {
  gap: 10px;
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
}

:global(.host-access-confirm-dialog .el-message-box__btns .el-button) {
  min-height: 34px;
  border-radius: 8px;
}

:global(.host-access-confirm-dialog .el-message-box__btns .el-button--danger) {
  border-color: #f56c6c;
  background: #f56c6c;
  color: #ffffff;
}

:global(.host-access-confirm-dialog .el-button:hover),
:global(.host-access-confirm-dialog .el-button:active) {
  transform: none;
}

@media (max-width: 1200px) {
  .ops-page-header {
    flex-wrap: wrap;
  }

  .authorization-header-summary {
    width: 100%;
    padding-left: 92px;
    overflow-x: auto;
  }
}

@media (max-width: 768px) {
  :global(.home-container .el-main:has(.authorization-management)) {
    padding-right: 6px;
    padding-left: 6px;
    overflow-y: auto !important;
  }

  .authorization-management {
    height: auto !important;
    min-height: calc(100vh - 124px) !important;
    padding: 6px;
    overflow: visible !important;
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
    white-space: normal;
  }

  .authorization-header-summary {
    padding-left: 0;
  }

  .auth-page-card {
    height: auto !important;
    overflow: visible !important;
  }

  .asset-list-section,
  .table-shell {
    min-height: 480px !important;
    overflow: hidden !important;
  }
}
</style>


