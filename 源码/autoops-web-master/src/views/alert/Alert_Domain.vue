<template>
  <div class="alert-center-page domain-alert-page">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/url.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>域名告警</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>{{ activeTab === 'monitor' ? '监控域名可用性、HTTP 响应与 SSL 证书有效期，异常进入统一告警体系。' : '通过配置中心的阿里云密钥完成 DNS 验证并申请免费 SSL 证书。' }}</p>
      </div>
      <div v-if="activeTab === 'monitor'" class="domain-header-summary" aria-label="域名监控统计">
        <span><em>域名</em><strong>{{ summary.total }}</strong></span>
        <i aria-hidden="true"></i>
        <span><em>正常</em><strong class="is-success">{{ summary.normal }}</strong></span>
        <i aria-hidden="true"></i>
        <span><em>异常</em><strong class="is-danger">{{ summary.abnormal }}</strong></span>
      </div>
      <div v-else class="domain-header-summary" aria-label="SSL证书申请统计">
        <span><em>申请</em><strong>{{ certificateTotal }}</strong></span>
        <i aria-hidden="true"></i>
        <span><em>处理中</em><strong class="is-warning">{{ certificateApplyingCount }}</strong></span>
        <i aria-hidden="true"></i>
        <span><em>已签发</em><strong class="is-success">{{ certificateIssuedCount }}</strong></span>
      </div>
    </header>

    <div class="domain-workspace">
      <el-tabs v-model="activeTab" class="alert-tabs domain-page-tabs" @tab-change="handlePageTabChange">
        <el-tab-pane label="域名监控" name="monitor" />
        <el-tab-pane label="SSL 证书申请" name="certificates" />
      </el-tabs>

      <el-card v-show="activeTab === 'monitor'" shadow="never" class="alert-center-card domain-monitor-card">
      <section class="alert-search-section domain-filter-section">
        <el-form :model="query" :inline="true" class="alert-search-form" @submit.prevent>
          <el-form-item label="域名">
            <el-input v-model="query.keyword" class="alert-filter-input" clearable placeholder="请输入域名" @keyup.enter="handleSearch" />
          </el-form-item>
          <el-form-item label="标签">
            <el-input v-model="query.tag" class="alert-filter-input domain-tag-filter" clearable placeholder="请输入标签" @keyup.enter="handleSearch" />
          </el-form-item>
          <el-form-item label="状态">
            <el-select v-model="query.status" class="alert-filter-small" clearable placeholder="全部">
              <el-option label="正常" value="normal" />
              <el-option label="异常" value="abnormal" />
              <el-option label="待检查" value="unknown" />
            </el-select>
          </el-form-item>
          <el-form-item label="存活状态">
            <el-select v-model="query.enabled" class="alert-filter-small" clearable placeholder="全部">
              <el-option label="启用" value="1" />
              <el-option label="禁用" value="0" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" size="small" class="alert-search-button" @click="handleSearch">
              <el-icon><Search /></el-icon><span>搜索</span>
            </el-button>
            <el-button type="warning" size="small" class="alert-reset-button" @click="handleReset">
              <el-icon><RefreshRight /></el-icon><span>重置</span>
            </el-button>
          </el-form-item>
        </el-form>
      </section>

      <div class="domain-toolbar">
        <div class="domain-toolbar-actions">
          <el-button type="success" size="small" class="alert-create-button" @click="openCreateDialog">
            <el-icon><Plus /></el-icon><span>添加域名</span>
          </el-button>
          <el-button type="primary" size="small" :loading="batchChecking" @click="checkSelectedOrAll">
            <el-icon><Refresh /></el-icon><span>{{ selectedRows.length ? `检查选中 (${selectedRows.length})` : '检查全部' }}</span>
          </el-button>
        </div>
        <div class="domain-quick-filters">
          <button type="button" :class="{ active: Number(query.expireWithinDays) === 30 }" @click="toggleExpireFilter">
            <el-icon><Timer /></el-icon><span>30 天内到期 {{ summary.expiring }}</span>
          </button>
          <button type="button" class="danger" :class="{ active: query.status === 'abnormal' }" @click="toggleAbnormalFilter">
            <el-icon><WarningFilled /></el-icon><span>异常域名 {{ summary.abnormal }}</span>
          </button>
        </div>
      </div>

      <div class="alert-table-shell domain-table-shell">
        <el-table
          class="alert-table domain-table"
          :data="rows"
          v-loading="loading"
          empty-text="暂无域名监控，请先添加域名"
          @selection-change="selectedRows = $event"
        >
          <el-table-column type="selection" width="44" />
          <el-table-column prop="id" label="ID" width="58" />
          <el-table-column label="域名" min-width="180">
            <template #default="{ row }">
              <button type="button" class="domain-name-link" @click="openDetail(row)">{{ row.domain || '-' }}</button>
              <div v-if="row.tags?.length" class="domain-tags">
                <el-tag v-for="tag in row.tags.slice(0, 2)" :key="tag" effect="plain">{{ tag }}</el-tag>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="88">
            <template #default="{ row }">
              <el-tag :type="statusTagType(row.status)" effect="light">{{ statusText(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="状态码" width="78" align="center">
            <template #default="{ row }">{{ row.statusCode || '-' }}</template>
          </el-table-column>
          <el-table-column label="响应" width="82" align="right">
            <template #default="{ row }">{{ row.responseTimeMs ? `${row.responseTimeMs}ms` : '-' }}</template>
          </el-table-column>
          <el-table-column label="SSL 证书" min-width="170">
            <template #default="{ row }">
              <template v-if="row.certificateExpiresAt">
                <div class="domain-certificate-time">{{ formatDateTime(row.certificateExpiresAt) }}</div>
                <el-tag :type="certificateTagType(row)" effect="plain">{{ certificateRemainingText(row) }}</el-tag>
              </template>
              <span v-else class="alert-muted-text">-</span>
            </template>
          </el-table-column>
          <el-table-column label="最后检查" min-width="148">
            <template #default="{ row }">{{ formatDateTime(row.lastCheckedAt) }}</template>
          </el-table-column>
          <el-table-column label="启用" width="70" align="center">
            <template #default="{ row }">
              <el-switch
                :model-value="Number(row.enabled) === 1"
                :loading="statusLoadingId === row.id"
                @change="value => toggleEnabled(row, value)"
              />
            </template>
          </el-table-column>
          <el-table-column label="备注" min-width="110" show-overflow-tooltip>
            <template #default="{ row }">{{ row.remarks || '-' }}</template>
          </el-table-column>
          <el-table-column label="证书提供商" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">{{ row.certificateIssuer || '-' }}</template>
          </el-table-column>
          <el-table-column label="操作" width="184" fixed="right">
            <template #default="{ row }">
              <div class="alert-operation-links">
                <button type="button" class="alert-operation-link" :disabled="checkingId === row.id" @click="checkRow(row)">
                  {{ checkingId === row.id ? '检查中' : '检查' }}
                </button>
                <button type="button" class="alert-operation-link alert-operation-warning" @click="openEditDialog(row)">编辑</button>
                <button type="button" class="alert-operation-link alert-operation-danger" @click="deleteRow(row)">删除</button>
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
          :total="total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="loadList"
          @current-change="loadList"
        />
      </div>
      </el-card>

      <el-card v-show="activeTab === 'certificates'" shadow="never" class="alert-center-card ssl-certificate-card">
      <section class="alert-search-section domain-filter-section">
        <el-form :model="certificateQuery" :inline="true" class="alert-search-form" @submit.prevent>
          <el-form-item label="域名">
            <el-input
              v-model="certificateQuery.keyword"
              class="alert-filter-input"
              clearable
              placeholder="请输入证书域名"
              @keyup.enter="searchCertificates"
            />
          </el-form-item>
          <el-form-item label="证书状态">
            <el-select v-model="certificateQuery.status" class="alert-filter-small" clearable placeholder="全部">
              <el-option label="申请中" :value="1" />
              <el-option label="已签发" :value="3" />
              <el-option label="申请失败" :value="5" />
            </el-select>
          </el-form-item>
          <el-form-item label="阿里云配置">
            <el-select v-model="certificateQuery.keyManageId" class="ssl-key-filter" clearable filterable placeholder="全部">
              <el-option v-for="item in aliyunKeyOptions" :key="item.id" :label="item.keyName" :value="Number(item.id)" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" size="small" class="alert-search-button" @click="searchCertificates">
              <el-icon><Search /></el-icon><span>搜索</span>
            </el-button>
            <el-button type="warning" size="small" class="alert-reset-button" @click="resetCertificateQuery">
              <el-icon><RefreshRight /></el-icon><span>重置</span>
            </el-button>
          </el-form-item>
        </el-form>
      </section>

      <div class="domain-toolbar ssl-certificate-toolbar">
        <div class="domain-toolbar-actions">
          <el-button type="success" size="small" class="alert-create-button" @click="openCertificateDialog">
            <el-icon><Plus /></el-icon><span>申请证书</span>
          </el-button>
          <el-button type="primary" size="small" :loading="certificateLoading" @click="loadCertificates">
            <el-icon><Refresh /></el-icon><span>刷新</span>
          </el-button>
        </div>
        <span v-if="certificateApplyingCount" class="ssl-refresh-state">申请处理中，页面将自动刷新状态</span>
      </div>

      <div class="alert-table-shell domain-table-shell ssl-certificate-table-shell">
        <el-table
          class="alert-table ssl-certificate-table"
          :data="certificateRows"
          v-loading="certificateLoading"
          empty-text="暂无证书申请记录"
        >
          <el-table-column prop="id" label="ID" width="64" />
          <el-table-column prop="domain" label="域名" min-width="190">
            <template #default="{ row }">
              <button type="button" class="domain-name-link ssl-domain-name" @click="openCertificateDetail(row)">
                {{ row.domain || '-' }}
              </button>
            </template>
          </el-table-column>
          <el-table-column label="阿里云配置" min-width="150" show-overflow-tooltip>
            <template #default="{ row }">{{ keyManageName(row.keyManageId) }}</template>
          </el-table-column>
          <el-table-column label="CA 提供商" width="130">
            <template #default="{ row }">{{ caProviderText(row.caProvider) }}</template>
          </el-table-column>
          <el-table-column prop="algorithm" label="加密算法" width="106" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="certificateStatusTagType(row.status)" effect="light">{{ certificateStatusText(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="签发时间" min-width="150">
            <template #default="{ row }">{{ formatDateTime(row.issuedAt) }}</template>
          </el-table-column>
          <el-table-column label="到期时间" min-width="150">
            <template #default="{ row }">{{ formatDateTime(row.expiresAt) }}</template>
          </el-table-column>
          <el-table-column label="申请时间" min-width="150">
            <template #default="{ row }">{{ formatDateTime(row.createdAt) }}</template>
          </el-table-column>
          <el-table-column label="结果" min-width="180" show-overflow-tooltip>
            <template #default="{ row }">
              <span :class="row.errorMessage ? 'ssl-error-text' : 'alert-muted-text'">{{ row.errorMessage || '-' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="350" fixed="right">
            <template #default="{ row }">
              <div class="alert-operation-links ssl-certificate-operations">
                <button type="button" class="alert-operation-link" :disabled="certificateDetailLoading && certificateDetail?.id === row.id" @click="openCertificateDetail(row)">
                  <el-icon><View /></el-icon><span>查看</span>
                </button>
                <button
                  type="button"
                  class="alert-operation-link alert-operation-success"
                  :disabled="Number(row.status) !== 3 || certificateDownloadingId === row.id"
                  @click="downloadCertificate(row)"
                >
                  <el-icon><Download /></el-icon><span>{{ certificateDownloadingId === row.id ? '下载中' : '下载' }}</span>
                </button>
                <button
                  type="button"
                  class="alert-operation-link alert-operation-success"
                  :disabled="Number(row.status) !== 3"
                  @click="openCertificateDeployDialog(row)"
                >
                  <el-icon><Upload /></el-icon><span>更新</span>
                </button>
                <button type="button" class="alert-operation-link" @click="openCertificateDeployLogs(row)">
                  <el-icon><Document /></el-icon><span>日志</span>
                </button>
                <button
                  type="button"
                  class="alert-operation-link alert-operation-danger"
                  :disabled="Number(row.status) === 1 || certificateDeletingId === row.id"
                  @click="deleteCertificate(row)"
                >
                  <el-icon><Delete /></el-icon><span>{{ certificateDeletingId === row.id ? '删除中' : '删除' }}</span>
                </button>
              </div>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <div class="alert-pagination">
        <el-pagination
          v-model:current-page="certificateQuery.page"
          v-model:page-size="certificateQuery.pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="certificateTotal"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="loadCertificates"
          @current-change="loadCertificates"
        />
      </div>
      </el-card>
    </div>

    <el-dialog
      v-model="certificateDialogVisible"
      title="申请 SSL 证书"
      width="600px"
      class="alert-dialog ssl-certificate-dialog"
      destroy-on-close
      @closed="resetCertificateForm"
    >
      <el-form ref="certificateFormRef" :model="certificateForm" :rules="certificateFormRules" label-width="116px">
        <el-form-item label="域名" prop="domain">
          <el-input v-model="certificateForm.domain" placeholder="example.com 或 *.example.com" />
        </el-form-item>
        <el-form-item label="证书类型" prop="certType">
          <el-select v-model="certificateForm.certType" class="ssl-form-control" disabled>
            <el-option label="ACME（免费）" :value="1" />
          </el-select>
        </el-form-item>
        <el-form-item label="阿里云配置" prop="keyManageId">
          <div class="ssl-form-field">
            <el-select v-model="certificateForm.keyManageId" class="ssl-form-control" filterable placeholder="请选择阿里云配置">
              <el-option v-for="item in aliyunKeyOptions" :key="item.id" :label="item.keyName" :value="Number(item.id)" />
            </el-select>
            <span class="ssl-form-help">用于 DNS-01 验证，需要阿里云 DNS 解析权限</span>
          </div>
        </el-form-item>
        <el-form-item label="CA 提供商" prop="caProvider">
          <el-select v-model="certificateForm.caProvider" class="ssl-form-control" disabled>
            <el-option label="Let's Encrypt" value="LetsEncrypt" />
          </el-select>
        </el-form-item>
        <el-form-item label="加密算法" prop="algorithm">
          <el-select v-model="certificateForm.algorithm" class="ssl-form-control">
            <el-option label="RSA 2048" value="RSA2048" />
            <el-option label="RSA 4096" value="RSA4096" />
            <el-option label="EC 256" value="EC256" />
            <el-option label="EC 384" value="EC384" />
          </el-select>
        </el-form-item>
        <el-form-item label="邮箱" prop="email">
          <div class="ssl-form-field">
            <el-input v-model="certificateForm.email" placeholder="请输入邮箱地址（选填）" />
            <span class="ssl-form-help">用于接收证书相关通知</span>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="alert-dialog-footer">
          <el-button @click="certificateDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="certificateSubmitting" @click="submitCertificateApplication">确定</el-button>
        </div>
      </template>
    </el-dialog>

    <el-drawer
      v-model="certificateDetailVisible"
      title="SSL 证书详情"
      size="min(620px, 94vw)"
      class="alert-detail-drawer ssl-certificate-detail-drawer"
      @closed="closeCertificateDetail"
    >
      <div v-loading="certificateDetailLoading" class="ssl-certificate-detail-body">
        <div v-if="certificateDetail" class="alert-detail-stack">
          <section class="alert-detail-panel">
            <h4>证书信息</h4>
            <div class="alert-detail-row"><span>域名</span><span>{{ certificateDetail.domain || '-' }}</span></div>
            <div class="alert-detail-row"><span>状态</span><span>{{ certificateStatusText(certificateDetail.status) }}</span></div>
            <div class="alert-detail-row"><span>CA 提供商</span><span>{{ caProviderText(certificateDetail.caProvider) }}</span></div>
            <div class="alert-detail-row"><span>加密算法</span><span>{{ certificateDetail.algorithm || '-' }}</span></div>
            <div class="alert-detail-row"><span>阿里云配置</span><span>{{ keyManageName(certificateDetail.keyManageId) }}</span></div>
            <div class="alert-detail-row"><span>联系邮箱</span><span>{{ certificateDetail.email || '-' }}</span></div>
            <div class="alert-detail-row"><span>签发时间</span><span>{{ formatDateTime(certificateDetail.issuedAt) }}</span></div>
            <div class="alert-detail-row"><span>到期时间</span><span>{{ formatDateTime(certificateDetail.expiresAt) }}</span></div>
            <div class="alert-detail-row"><span>申请人</span><span>{{ certificateDetail.createdBy || '-' }}</span></div>
            <div class="alert-detail-row"><span>申请时间</span><span>{{ formatDateTime(certificateDetail.createdAt) }}</span></div>
            <div class="alert-detail-row"><span>私钥</span><span>{{ certificateDetail.hasPrivateKey ? '已安全保存，可通过下载获取' : '未生成' }}</span></div>
          </section>

          <section v-if="certificateDetail.errorMessage" class="alert-detail-panel domain-failure-panel">
            <h4>申请失败原因</h4>
            <p>{{ certificateDetail.errorMessage }}</p>
          </section>

          <section v-if="certificateDetail.certificate" class="alert-detail-panel ssl-pem-panel">
            <div class="ssl-pem-heading">
              <h4>证书 PEM</h4>
              <el-button link type="primary" size="small" @click="copyCertificateText(certificateDetail.certificate, '证书')">
                <el-icon><DocumentCopy /></el-icon><span>复制</span>
              </el-button>
            </div>
            <pre>{{ certificateDetail.certificate }}</pre>
          </section>

          <section v-if="certificateDetail.issuerCertificate" class="alert-detail-panel ssl-pem-panel">
            <div class="ssl-pem-heading">
              <h4>颁发链 PEM</h4>
              <el-button link type="primary" size="small" @click="copyCertificateText(certificateDetail.issuerCertificate, '颁发链')">
                <el-icon><DocumentCopy /></el-icon><span>复制</span>
              </el-button>
            </div>
            <pre>{{ certificateDetail.issuerCertificate }}</pre>
          </section>
        </div>
      </div>
    </el-drawer>

    <el-dialog
      v-model="certificateDeployDialogVisible"
      title="更新 SSL 证书"
      width="620px"
      class="alert-dialog ssl-certificate-deploy-dialog"
      destroy-on-close
      @closed="resetCertificateDeployForm"
    >
      <el-form ref="certificateDeployFormRef" :model="certificateDeployForm" :rules="certificateDeployFormRules" label-width="108px">
        <el-form-item label="证书域名">
          <el-input :model-value="certificateDeployCertificate?.domain || '-'" disabled />
        </el-form-item>
        <el-form-item label="目标主机" prop="hostId">
          <el-select
            v-model="certificateDeployForm.hostId"
            class="ssl-form-control"
            filterable
            remote
            reserve-keyword
            :remote-method="searchCertificateDeployHosts"
            :loading="certificateDeployHostLoading"
            placeholder="请选择 CMDB Linux 主机"
          >
            <el-option
              v-for="host in certificateDeployHostOptions"
              :key="host.id"
              :label="certificateDeployHostLabel(host)"
              :value="Number(host.id)"
              :disabled="!host.sshKeyId"
            >
              <div class="ssl-deploy-host-option">
                <span>{{ host.hostName || host.name || `主机 #${host.id}` }}</span>
                <small>{{ host.sshIp || host.privateIp || '-' }}</small>
              </div>
            </el-option>
          </el-select>
        </el-form-item>
        <el-form-item label="更新路径" prop="deployPath">
          <div class="ssl-form-field">
            <el-input v-model="certificateDeployForm.deployPath" placeholder="/etc/nginx/ssl" />
            <span class="ssl-form-help">将更新 {{ certificateDeployFileBase }}.pem 和 {{ certificateDeployFileBase }}.key</span>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="alert-dialog-footer">
          <el-button @click="certificateDeployDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="certificateDeploySubmitting" @click="submitCertificateDeployment">开始更新</el-button>
        </div>
      </template>
    </el-dialog>

    <el-drawer
      v-model="certificateDeployLogVisible"
      title="证书更新日志"
      size="min(760px, 96vw)"
      class="alert-detail-drawer ssl-deploy-log-drawer"
      @closed="closeCertificateDeployLogs"
    >
      <div v-loading="certificateDeployLogLoading" class="ssl-deploy-log-body">
        <el-table
          v-if="certificateDeployLogRows.length"
          :data="certificateDeployLogRows"
          size="small"
          max-height="230"
          highlight-current-row
          class="ssl-deploy-log-table"
          @current-change="selectCertificateDeployLog"
        >
          <el-table-column label="开始时间" min-width="142">
            <template #default="{ row }">{{ formatDateTime(row.startedAt) }}</template>
          </el-table-column>
          <el-table-column label="目标主机" min-width="150" show-overflow-tooltip>
            <template #default="{ row }">{{ row.hostName }} ({{ row.hostIp }})</template>
          </el-table-column>
          <el-table-column label="更新路径" min-width="145" show-overflow-tooltip prop="deployPath" />
          <el-table-column label="状态" width="86">
            <template #default="{ row }">
              <el-tag :type="certificateDeployStatusType(row.status)" effect="light">{{ row.statusText || certificateDeployStatusText(row.status) }}</el-tag>
            </template>
          </el-table-column>
        </el-table>

        <el-empty v-if="!certificateDeployLogLoading && !certificateDeployLogRows.length" description="暂无证书更新日志" :image-size="72" />

        <section v-if="certificateDeployCurrentLog" class="ssl-deploy-log-detail">
          <div class="ssl-deploy-log-heading">
            <div>
              <strong>{{ certificateDeployCurrentLog.domain }}</strong>
              <span>{{ certificateDeployCurrentLog.hostName }} · {{ certificateDeployCurrentLog.hostIp }}</span>
            </div>
            <el-tag :type="certificateDeployStatusType(certificateDeployCurrentLog.status)" effect="light">
              {{ certificateDeployCurrentLog.statusText || certificateDeployStatusText(certificateDeployCurrentLog.status) }}
            </el-tag>
          </div>
          <div class="ssl-deploy-log-meta">
            <span>证书：{{ certificateDeployCurrentLog.certificatePath }}</span>
            <span>私钥：{{ certificateDeployCurrentLog.privateKeyPath }}</span>
            <span v-if="certificateDeployCurrentLog.backupPath">备份：{{ certificateDeployCurrentLog.backupPath }}</span>
          </div>
          <pre>{{ (certificateDeployCurrentLog.logs || []).join('\n') || '任务日志生成中...' }}</pre>
          <p v-if="certificateDeployCurrentLog.errorMessage" class="ssl-deploy-error">{{ certificateDeployCurrentLog.errorMessage }}</p>
        </section>
      </div>
    </el-drawer>

    <el-dialog
      v-model="dialogVisible"
      :title="editingId ? '编辑域名监控' : '添加域名监控'"
      width="760px"
      class="alert-dialog domain-dialog"
      modal-class="domain-dialog-overlay"
      destroy-on-close
      @closed="resetForm"
    >
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="104px">
        <div class="alert-form-grid domain-form-grid">
          <el-form-item label="域名" prop="domain" class="alert-form-full">
            <el-input v-model="form.domain" placeholder="example.com 或 https://example.com/health" />
          </el-form-item>
          <el-form-item label="协议" prop="protocol">
            <el-segmented v-model="form.protocol" :options="protocolOptions" />
          </el-form-item>
          <el-form-item label="端口">
            <el-input-number v-model="form.port" :min="0" :max="65535" controls-position="right" />
            <span class="domain-form-hint">0 表示默认端口</span>
          </el-form-item>
          <el-form-item label="检查路径" prop="path">
            <el-input v-model="form.path" placeholder="/" />
          </el-form-item>
          <el-form-item label="期望状态码">
            <el-select v-model="form.expectedStatus" class="domain-form-select">
              <el-option label="任意 HTTP 响应" :value="0" />
              <el-option label="200" :value="200" />
              <el-option label="204" :value="204" />
              <el-option label="301" :value="301" />
              <el-option label="302" :value="302" />
              <el-option label="403" :value="403" />
            </el-select>
          </el-form-item>
          <el-form-item label="检查间隔" prop="checkIntervalSeconds">
            <el-select v-model="form.checkIntervalSeconds" class="domain-form-select">
              <el-option label="30 秒" :value="30" />
              <el-option label="1 分钟" :value="60" />
              <el-option label="5 分钟" :value="300" />
              <el-option label="10 分钟" :value="600" />
              <el-option label="30 分钟" :value="1800" />
            </el-select>
          </el-form-item>
          <el-form-item label="请求超时" prop="timeoutSeconds">
            <el-input-number v-model="form.timeoutSeconds" :min="1" :max="60" controls-position="right" />
            <span class="domain-form-hint">秒</span>
          </el-form-item>
          <el-form-item label="证书预警" prop="sslExpireWarnDays">
            <el-input-number v-model="form.sslExpireWarnDays" :min="1" :max="365" controls-position="right" />
            <span class="domain-form-hint">天</span>
          </el-form-item>
          <el-form-item label="标签" class="alert-form-full">
            <el-select v-model="form.tags" multiple filterable allow-create default-first-option class="domain-form-select" placeholder="输入标签后回车">
              <el-option v-for="tag in knownTags" :key="tag" :label="tag" :value="tag" />
            </el-select>
          </el-form-item>
          <el-form-item label="告警策略" class="alert-form-full">
            <el-select v-model="form.notifyRuleIds" multiple filterable class="domain-form-select" placeholder="不选择时仅记录告警事件">
              <el-option v-for="item in notifyRuleOptions" :key="item.id" :label="item.name" :value="Number(item.id)" />
            </el-select>
          </el-form-item>
          <el-form-item label="备注" class="alert-form-full">
            <el-input v-model="form.remarks" type="textarea" :rows="2" maxlength="500" show-word-limit />
          </el-form-item>
          <el-form-item label="启用监控" class="alert-form-full">
            <el-switch v-model="form.enabled" :active-value="1" :inactive-value="0" />
          </el-form-item>
        </div>
      </el-form>
      <template #footer>
        <div class="alert-dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="saving" @click="submitForm">保存</el-button>
        </div>
      </template>
    </el-dialog>

    <el-drawer v-model="detailVisible" title="域名监控详情" size="520px" class="alert-detail-drawer domain-detail-drawer">
      <div v-if="currentRow" class="alert-detail-stack">
        <section class="alert-detail-panel">
          <h4>探测状态</h4>
          <div class="alert-detail-row"><span>域名</span><span>{{ domainURL(currentRow) }}</span></div>
          <div class="alert-detail-row"><span>状态</span><span>{{ statusText(currentRow.status) }}</span></div>
          <div class="alert-detail-row"><span>HTTP 状态码</span><span>{{ currentRow.statusCode || '-' }}</span></div>
          <div class="alert-detail-row"><span>响应耗时</span><span>{{ currentRow.responseTimeMs ? `${currentRow.responseTimeMs}ms` : '-' }}</span></div>
          <div class="alert-detail-row"><span>最后检查</span><span>{{ formatDateTime(currentRow.lastCheckedAt) }}</span></div>
          <div class="alert-detail-row"><span>下次检查</span><span>{{ formatDateTime(currentRow.nextCheckAt) }}</span></div>
        </section>
        <section class="alert-detail-panel">
          <h4>SSL 证书</h4>
          <div class="alert-detail-row"><span>到期时间</span><span>{{ formatDateTime(currentRow.certificateExpiresAt) }}</span></div>
          <div class="alert-detail-row"><span>剩余时间</span><span>{{ certificateRemainingText(currentRow) }}</span></div>
          <div class="alert-detail-row"><span>证书提供商</span><span>{{ currentRow.certificateIssuer || '-' }}</span></div>
        </section>
        <section v-if="currentRow.failureReason" class="alert-detail-panel domain-failure-panel">
          <h4>异常原因</h4>
          <p>{{ currentRow.failureReason }}</p>
        </section>
      </div>
    </el-drawer>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Delete, Document, DocumentCopy, Download, Plus, Refresh, RefreshRight, Search, Timer, Upload, View, WarningFilled } from '@element-plus/icons-vue'
import alertApi from '@/api/alertmanager'
import cmdbApi from '@/api/cmdb'
import configApi from '@/api/config'
import { compactObject, formatDateTime, isSuccess, normalizeListResult, unwrapData } from './alertCommon'
import './alert-pages.css'

const defaultForm = () => ({
  domain: '', protocol: 'https', port: 0, path: '/', tags: [], expectedStatus: 0,
  checkIntervalSeconds: 300, timeoutSeconds: 10, sslExpireWarnDays: 30,
  notifyRuleIds: [], remarks: '', enabled: 1
})

const protocolOptions = [
  { label: 'HTTPS', value: 'https' },
  { label: 'HTTP', value: 'http' }
]
const activeTab = ref('monitor')
const loading = ref(false)
const saving = ref(false)
const batchChecking = ref(false)
const checkingId = ref(null)
const statusLoadingId = ref(null)
const dialogVisible = ref(false)
const detailVisible = ref(false)
const editingId = ref(null)
const formRef = ref()
const rows = ref([])
const selectedRows = ref([])
const notifyRuleOptions = ref([])
const currentRow = ref(null)
const total = ref(0)
const summary = reactive({ total: 0, normal: 0, abnormal: 0, enabled: 0, expiring: 0 })
const query = reactive({ page: 1, pageSize: 10, keyword: '', tag: '', status: '', enabled: '', expireWithinDays: '' })
const form = reactive(defaultForm())
const formRules = {
  domain: [{ required: true, message: '请输入域名', trigger: 'blur' }],
  protocol: [{ required: true, message: '请选择协议', trigger: 'change' }],
  path: [{ required: true, message: '请输入检查路径', trigger: 'blur' }],
  checkIntervalSeconds: [{ required: true, message: '请选择检查间隔', trigger: 'change' }],
  timeoutSeconds: [{ required: true, message: '请输入请求超时', trigger: 'change' }],
  sslExpireWarnDays: [{ required: true, message: '请输入证书预警天数', trigger: 'change' }]
}

const defaultCertificateForm = () => ({
  domain: '', certType: 1, keyManageId: '', caProvider: 'LetsEncrypt', algorithm: 'RSA2048', email: ''
})
const certificateLoading = ref(false)
const certificateSubmitting = ref(false)
const certificateDialogVisible = ref(false)
const certificateFormRef = ref()
const certificateRows = ref([])
const certificateDetailVisible = ref(false)
const certificateDetailLoading = ref(false)
const certificateDetail = ref(null)
const certificateDetailRequestId = ref(0)
const certificateDownloadingId = ref(null)
const certificateDeletingId = ref(null)
const aliyunKeyOptions = ref([])
const certificateTotal = ref(0)
const certificateQuery = reactive({ page: 1, pageSize: 10, keyword: '', status: '', keyManageId: '' })
const certificateForm = reactive(defaultCertificateForm())
const certificateFormRules = {
  domain: [
    { required: true, message: '请输入域名', trigger: 'blur' },
    {
      validator: (_rule, value, callback) => {
        const domain = String(value || '').trim().toLowerCase()
        const plain = domain.startsWith('*.') ? domain.slice(2) : domain
        const valid = plain.includes('.') && !plain.includes('*') && plain.split('.').every(label => /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(label))
        valid ? callback() : callback(new Error('请输入有效域名，通配符仅支持 *.example.com'))
      },
      trigger: 'blur'
    }
  ],
  certType: [{ required: true, message: '请选择证书类型', trigger: 'change' }],
  keyManageId: [{ required: true, message: '请选择阿里云配置', trigger: 'change' }],
  caProvider: [{ required: true, message: '请选择 CA 提供商', trigger: 'change' }],
  algorithm: [{ required: true, message: '请选择加密算法', trigger: 'change' }],
  email: [{ type: 'email', message: '请输入有效邮箱地址', trigger: 'blur' }]
}
let certificateRefreshTimer
const defaultCertificateDeployForm = () => ({ hostId: '', deployPath: '/etc/nginx/ssl' })
const certificateDeployDialogVisible = ref(false)
const certificateDeployFormRef = ref()
const certificateDeployForm = reactive(defaultCertificateDeployForm())
const certificateDeployCertificate = ref(null)
const certificateDeployHostOptions = ref([])
const certificateDeployHostLoading = ref(false)
const certificateDeploySubmitting = ref(false)
const certificateDeployLogVisible = ref(false)
const certificateDeployLogLoading = ref(false)
const certificateDeployLogRows = ref([])
const certificateDeployCurrentLog = ref(null)
const certificateDeployLogCertificate = ref(null)
const certificateDeployFormRules = {
  hostId: [{ required: true, message: '请选择目标主机', trigger: 'change' }],
  deployPath: [
    { required: true, message: '请输入更新路径', trigger: 'blur' },
    {
      validator: (_rule, value, callback) => {
        const path = String(value || '').trim()
        const valid = /^\/(?:[A-Za-z0-9._-]+\/)*[A-Za-z0-9._-]+$/.test(path) && !path.split('/').some(segment => segment === '.' || segment === '..')
        valid ? callback() : callback(new Error('请输入不含 . 或 .. 的 Linux 绝对路径'))
      },
      trigger: 'blur'
    }
  ]
}
let certificateDeployPollTimer

const certificateApplyingCount = computed(() => certificateRows.value.filter(item => Number(item.status) === 1).length)
const certificateIssuedCount = computed(() => certificateRows.value.filter(item => Number(item.status) === 3).length)
const certificateDeployFileBase = computed(() => certificateDownloadBaseName(certificateDeployCertificate.value?.domain))

const knownTags = computed(() => [...new Set(rows.value.flatMap(row => Array.isArray(row.tags) ? row.tags : []))])

const ensureSuccess = (response, fallback) => {
  if (isSuccess(response)) return
  const message = response?.data?.message || fallback
  ElMessage.error(message)
  throw new Error(message)
}

const loadNotifyRules = async () => {
  const response = await alertApi.getNotifyRules({ page: 1, pageSize: 100, enabled: 1 })
  ensureSuccess(response, '加载告警策略失败')
  notifyRuleOptions.value = normalizeListResult(response).list
}

const loadAliyunKeys = async () => {
  const response = await configApi.getKeyManageList({ page: 1, pageSize: 500 })
  ensureSuccess(response, '加载阿里云配置失败')
  aliyunKeyOptions.value = normalizeListResult(response).list
    .filter(item => Number(item.keyType) === 1)
    .map(item => ({ id: Number(item.id), keyType: Number(item.keyType), keyName: item.keyName || `阿里云密钥 #${item.id}` }))
}

const scheduleCertificateRefresh = () => {
  window.clearTimeout(certificateRefreshTimer)
  certificateRefreshTimer = undefined
  if (activeTab.value === 'certificates' && certificateApplyingCount.value > 0) {
    certificateRefreshTimer = window.setTimeout(loadCertificates, 5000)
  }
}

const loadCertificates = async () => {
  window.clearTimeout(certificateRefreshTimer)
  certificateRefreshTimer = undefined
  if (certificateLoading.value) return
  certificateLoading.value = true
  try {
    const response = await alertApi.getSSLCertificates(compactObject(certificateQuery))
    ensureSuccess(response, '加载证书申请记录失败')
    const payload = unwrapData(response)
    certificateRows.value = Array.isArray(payload.list) ? payload.list : []
    certificateTotal.value = Number(payload.total || 0)
  } finally {
    certificateLoading.value = false
    scheduleCertificateRefresh()
  }
}

const handlePageTabChange = tab => {
  if (tab !== 'certificates') {
    window.clearTimeout(certificateRefreshTimer)
    return
  }
  Promise.all([loadCertificates(), aliyunKeyOptions.value.length ? Promise.resolve() : loadAliyunKeys()])
}

const searchCertificates = () => {
  certificateQuery.page = 1
  loadCertificates()
}

const resetCertificateQuery = () => {
  Object.assign(certificateQuery, { page: 1, pageSize: 10, keyword: '', status: '', keyManageId: '' })
  loadCertificates()
}

const resetCertificateForm = () => {
  Object.assign(certificateForm, defaultCertificateForm())
  certificateFormRef.value?.clearValidate()
}

const openCertificateDialog = async () => {
  resetCertificateForm()
  if (!aliyunKeyOptions.value.length) await loadAliyunKeys()
  if (!aliyunKeyOptions.value.length) {
    ElMessage.warning('配置中心暂无阿里云密钥，请先添加具有 DNS 解析权限的密钥')
    return
  }
  certificateDialogVisible.value = true
}

const submitCertificateApplication = async () => {
  const valid = await certificateFormRef.value?.validate().catch(() => false)
  if (!valid) return
  certificateSubmitting.value = true
  try {
    const response = await alertApi.applySSLCertificate({
      ...certificateForm,
      domain: certificateForm.domain.trim(),
      keyManageId: Number(certificateForm.keyManageId)
    })
    ensureSuccess(response, '提交证书申请失败')
    ElMessage.success('证书申请已提交，正在后台处理')
    certificateDialogVisible.value = false
    certificateQuery.page = 1
    await loadCertificates()
  } finally {
    certificateSubmitting.value = false
  }
}

const openCertificateDetail = async row => {
  const requestId = ++certificateDetailRequestId.value
  certificateDetail.value = { ...row }
  certificateDetailVisible.value = true
  certificateDetailLoading.value = true
  try {
    const response = await alertApi.getSSLCertificate(row.id)
    ensureSuccess(response, '加载证书详情失败')
    if (requestId !== certificateDetailRequestId.value) return
    certificateDetail.value = unwrapData(response)
  } finally {
    if (requestId === certificateDetailRequestId.value) certificateDetailLoading.value = false
  }
}

const closeCertificateDetail = () => {
  certificateDetailRequestId.value += 1
  certificateDetailLoading.value = false
  certificateDetail.value = null
}

const downloadCertificate = async row => {
  if (Number(row.status) !== 3) {
    ElMessage.warning('证书尚未签发，暂不能下载')
    return
  }
  certificateDownloadingId.value = row.id
  try {
    const response = await alertApi.downloadSSLCertificate(row.id)
    const contentType = String(response?.headers?.['content-type'] || response?.data?.type || '')
    if (contentType.includes('application/json')) {
      const payload = JSON.parse(await response.data.text())
      ElMessage.error(payload?.message || '下载证书失败')
      return
    }
    const blob = response.data instanceof Blob ? response.data : new Blob([response.data], { type: 'application/zip' })
    const disposition = response?.headers?.['content-disposition'] || ''
    const filenameMatch = disposition.match(/filename="?([^";]+)"?/i)
    const filename = filenameMatch?.[1] || `${certificateDownloadBaseName(row.domain)}-ssl-certificate.zip`
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = filename
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
    ElMessage.success('证书下载成功')
  } finally {
    certificateDownloadingId.value = null
  }
}

const deleteCertificate = async row => {
  if (Number(row.status) === 1) {
    ElMessage.warning('证书正在申请中，暂不能删除')
    return
  }
  await ElMessageBox.confirm(`确认删除 ${row.domain} 的证书记录？删除后证书和私钥无法恢复。`, '删除 SSL 证书', { type: 'warning' })
  certificateDeletingId.value = row.id
  try {
    const response = await alertApi.deleteSSLCertificate(row.id)
    ensureSuccess(response, '删除证书失败')
    ElMessage.success('SSL 证书记录已删除')
    if (certificateDetail.value?.id === row.id) {
      certificateDetailVisible.value = false
      certificateDetail.value = null
    }
    if (certificateRows.value.length === 1 && certificateQuery.page > 1) certificateQuery.page -= 1
    await loadCertificates()
  } finally {
    certificateDeletingId.value = null
  }
}

const normalizeCertificateDeployHosts = response => normalizeListResult(response).list
  .filter(host => Number(host.hostType || 1) !== 2 && String(host.sshIp || '').trim())

const searchCertificateDeployHosts = async (keyword = '') => {
  certificateDeployHostLoading.value = true
  try {
    const response = String(keyword || '').trim()
      ? await cmdbApi.GetCmdbHostsByKeyword(String(keyword).trim(), { page: 1, pageSize: 100 })
      : await cmdbApi.getCmdbHostList({ page: 1, pageSize: 100 })
    ensureSuccess(response, '加载 CMDB 主机失败')
    certificateDeployHostOptions.value = normalizeCertificateDeployHosts(response)
  } finally {
    certificateDeployHostLoading.value = false
  }
}

const certificateDeployHostLabel = host => {
  const name = host.hostName || host.name || `主机 #${host.id}`
  return `${name} (${host.sshIp || host.privateIp || '-'})`
}

const resetCertificateDeployForm = () => {
  Object.assign(certificateDeployForm, defaultCertificateDeployForm())
  certificateDeployCertificate.value = null
  certificateDeployFormRef.value?.clearValidate()
}

const openCertificateDeployDialog = async row => {
  if (Number(row.status) !== 3) {
    ElMessage.warning('证书尚未签发，暂不能更新到主机')
    return
  }
  Object.assign(certificateDeployForm, defaultCertificateDeployForm())
  certificateDeployCertificate.value = { ...row }
  certificateDeployDialogVisible.value = true
  await searchCertificateDeployHosts()
}

const submitCertificateDeployment = async () => {
  const valid = await certificateDeployFormRef.value?.validate().catch(() => false)
  if (!valid || !certificateDeployCertificate.value) return
  certificateDeploySubmitting.value = true
  try {
    const response = await alertApi.deploySSLCertificate(certificateDeployCertificate.value.id, {
      hostId: Number(certificateDeployForm.hostId),
      deployPath: certificateDeployForm.deployPath.trim()
    })
    ensureSuccess(response, '创建证书更新任务失败')
    const payload = unwrapData(response)
    const task = payload.deployLog || {}
    ElMessage.success('证书更新任务已创建')
    certificateDeployDialogVisible.value = false
    certificateDeployLogCertificate.value = { ...certificateDeployCertificate.value }
    certificateDeployLogVisible.value = true
    certificateDeployCurrentLog.value = task
    await loadCertificateDeployLogs(task.id)
  } finally {
    certificateDeploySubmitting.value = false
  }
}

const loadCertificateDeployLogs = async selectedLogId => {
  const certificate = certificateDeployLogCertificate.value
  if (!certificate) return
  certificateDeployLogLoading.value = true
  try {
    const response = await alertApi.getSSLCertificateDeployLogs(certificate.id, { page: 1, pageSize: 50 })
    ensureSuccess(response, '加载证书更新日志失败')
    certificateDeployLogRows.value = normalizeListResult(response).list
    const targetId = Number(selectedLogId || certificateDeployCurrentLog.value?.id || certificateDeployLogRows.value[0]?.id)
    const target = certificateDeployLogRows.value.find(item => Number(item.id) === targetId) || certificateDeployLogRows.value[0]
    if (target) await loadCertificateDeployLogDetail(target.id)
  } finally {
    certificateDeployLogLoading.value = false
  }
}

const loadCertificateDeployLogDetail = async logId => {
  const certificate = certificateDeployLogCertificate.value
  if (!certificate || !logId) return
  const response = await alertApi.getSSLCertificateDeployLog(certificate.id, logId)
  ensureSuccess(response, '加载证书更新日志详情失败')
  certificateDeployCurrentLog.value = unwrapData(response)
  scheduleCertificateDeployLogPoll()
}

const scheduleCertificateDeployLogPoll = () => {
  window.clearTimeout(certificateDeployPollTimer)
  certificateDeployPollTimer = undefined
  if (certificateDeployLogVisible.value && Number(certificateDeployCurrentLog.value?.status) === 1) {
    const certificateId = certificateDeployLogCertificate.value?.id
    const logId = certificateDeployCurrentLog.value?.id
    certificateDeployPollTimer = window.setTimeout(async () => {
      if (!certificateDeployLogVisible.value || certificateDeployLogCertificate.value?.id !== certificateId) return
      try {
        await loadCertificateDeployLogDetail(logId)
        const index = certificateDeployLogRows.value.findIndex(item => Number(item.id) === Number(logId))
        if (index >= 0) certificateDeployLogRows.value[index] = { ...certificateDeployCurrentLog.value }
      } catch (_error) {
        scheduleCertificateDeployLogPoll()
      }
    }, 2000)
  }
}

const selectCertificateDeployLog = row => {
  if (row?.id) loadCertificateDeployLogDetail(row.id)
}

const openCertificateDeployLogs = async row => {
  window.clearTimeout(certificateDeployPollTimer)
  certificateDeployLogCertificate.value = { ...row }
  certificateDeployCurrentLog.value = null
  certificateDeployLogRows.value = []
  certificateDeployLogVisible.value = true
  await loadCertificateDeployLogs()
}

const closeCertificateDeployLogs = () => {
  window.clearTimeout(certificateDeployPollTimer)
  certificateDeployPollTimer = undefined
  certificateDeployLogRows.value = []
  certificateDeployCurrentLog.value = null
  certificateDeployLogCertificate.value = null
}

const certificateDeployStatusText = status => ({ 1: '部署中', 2: '成功', 3: '失败' }[Number(status)] || '未知')
const certificateDeployStatusType = status => ({ 1: 'warning', 2: 'success', 3: 'danger' }[Number(status)] || 'info')

const certificateDownloadBaseName = domain => {
  const normalized = String(domain || '').trim().toLowerCase()
  return normalized.startsWith('*.') ? `wildcard-${normalized.slice(2)}` : normalized || 'ssl-certificate'
}

const copyCertificateText = async (text, label) => {
  const value = String(text || '')
  if (!value) return
  try {
    await navigator.clipboard.writeText(value)
  } catch (_error) {
    const textarea = document.createElement('textarea')
    textarea.value = value
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    textarea.remove()
  }
  ElMessage.success(`${label}已复制`)
}

const certificateStatusText = status => ({ 1: '申请中', 3: '已签发', 5: '申请失败' }[Number(status)] || '未知')
const certificateStatusTagType = status => ({ 1: 'warning', 3: 'success', 5: 'danger' }[Number(status)] || 'info')
const caProviderText = provider => provider === 'LetsEncrypt' ? "Let's Encrypt" : provider || '-'
const keyManageName = id => aliyunKeyOptions.value.find(item => Number(item.id) === Number(id))?.keyName || `配置 #${id}`

const loadList = async () => {
  loading.value = true
  try {
    const response = await alertApi.getDomains(compactObject(query))
    ensureSuccess(response, '加载域名监控失败')
    const payload = unwrapData(response)
    rows.value = Array.isArray(payload.list) ? payload.list.map(normalizeRow) : []
    total.value = Number(payload.total || 0)
    Object.assign(summary, payload.summary || {})
    selectedRows.value = []
  } finally {
    loading.value = false
  }
}

const normalizeRow = row => ({
  ...row,
  tags: Array.isArray(row.tags) ? row.tags : [],
  notifyRuleIds: Array.isArray(row.notifyRuleIds) ? row.notifyRuleIds.map(Number) : []
})

const handleSearch = () => {
  query.page = 1
  loadList()
}

const handleReset = () => {
  Object.assign(query, { page: 1, pageSize: 10, keyword: '', tag: '', status: '', enabled: '', expireWithinDays: '' })
  loadList()
}

const toggleExpireFilter = () => {
  query.expireWithinDays = Number(query.expireWithinDays) === 30 ? '' : 30
  query.page = 1
  loadList()
}

const toggleAbnormalFilter = () => {
  query.status = query.status === 'abnormal' ? '' : 'abnormal'
  query.page = 1
  loadList()
}

const resetForm = () => {
  editingId.value = null
  Object.assign(form, defaultForm())
  formRef.value?.clearValidate()
}

const openCreateDialog = () => {
  resetForm()
  dialogVisible.value = true
}

const openEditDialog = row => {
  editingId.value = row.id
  Object.assign(form, {
    domain: row.domain || '', protocol: row.protocol || 'https', port: Number(row.port || 0), path: row.path || '/',
    tags: [...(row.tags || [])], expectedStatus: Number(row.expectedStatus || 0),
    checkIntervalSeconds: Number(row.checkIntervalSeconds || 300), timeoutSeconds: Number(row.timeoutSeconds || 10),
    sslExpireWarnDays: Number(row.sslExpireWarnDays || 30), notifyRuleIds: [...(row.notifyRuleIds || [])],
    remarks: row.remarks || '', enabled: Number(row.enabled) === 1 ? 1 : 0
  })
  dialogVisible.value = true
}

const submitForm = async () => {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return
  saving.value = true
  try {
    const payload = { ...form, tags: [...form.tags], notifyRuleIds: [...form.notifyRuleIds] }
    const response = editingId.value
      ? await alertApi.updateDomain(editingId.value, payload)
      : await alertApi.createDomain(payload)
    ensureSuccess(response, '保存失败')
    ElMessage.success(editingId.value ? '域名监控已更新' : '域名监控已添加')
    dialogVisible.value = false
    await loadList()
  } finally {
    saving.value = false
  }
}

const checkRow = async row => {
  checkingId.value = row.id
  try {
    const response = await alertApi.checkDomain(row.id)
    ensureSuccess(response, '检查失败')
    const checked = normalizeRow(unwrapData(response))
    Object.assign(row, checked)
    ElMessage({ type: checked.status === 'normal' ? 'success' : 'warning', message: checked.status === 'normal' ? '域名检查正常' : `发现异常：${checked.failureReason || '请查看详情'}` })
    await loadList()
  } finally {
    checkingId.value = null
  }
}

const checkSelectedOrAll = async () => {
  batchChecking.value = true
  try {
    const ids = selectedRows.value.map(row => row.id)
    const response = await alertApi.checkDomains({ ids })
    ensureSuccess(response, '检查失败')
    const payload = unwrapData(response)
    ElMessage.success(`已完成 ${Number(payload.total || 0)} 个域名检查`)
    await loadList()
  } finally {
    batchChecking.value = false
  }
}

const toggleEnabled = async (row, value) => {
  statusLoadingId.value = row.id
  try {
    const response = await alertApi.updateDomainStatus(row.id, value ? 1 : 0)
    ensureSuccess(response, '状态更新失败')
    row.enabled = value ? 1 : 0
    ElMessage.success(value ? '已启用域名监控' : '已停用域名监控')
  } finally {
    statusLoadingId.value = null
  }
}

const deleteRow = async row => {
  await ElMessageBox.confirm(`确认删除域名 ${row.domain}？相关活跃告警将自动恢复。`, '删除域名监控', { type: 'warning' })
  const response = await alertApi.deleteDomain(row.id)
  ensureSuccess(response, '删除失败')
  ElMessage.success('域名监控已删除')
  if (rows.value.length === 1 && query.page > 1) query.page -= 1
  await loadList()
}

const openDetail = row => {
  currentRow.value = row
  detailVisible.value = true
}

const statusText = status => ({ normal: '正常', abnormal: '异常', unknown: '待检查' }[status] || '待检查')
const statusTagType = status => ({ normal: 'success', abnormal: 'danger', unknown: 'info' }[status] || 'info')
const certificateTagType = row => Number(row.certificateRemainingDays) <= 0 ? 'danger' : Number(row.certificateRemainingDays) <= Number(row.sslExpireWarnDays || 30) ? 'warning' : 'success'
const certificateRemainingText = row => {
  if (!row.certificateExpiresAt) return '-'
  const days = Number(row.certificateRemainingDays || 0)
  return days < 0 ? `已过期 ${Math.abs(days)} 天` : `剩余 ${days} 天`
}
const domainURL = row => {
  const port = Number(row.port || 0)
  return `${row.protocol || 'https'}://${row.domain}${port ? `:${port}` : ''}${row.path || '/'}`
}

onMounted(() => Promise.all([loadList(), loadNotifyRules()]))
onBeforeUnmount(() => {
  window.clearTimeout(certificateRefreshTimer)
  window.clearTimeout(certificateDeployPollTimer)
})
</script>

<style scoped>
:global(.home-container .el-main:has(.domain-alert-page)) {
  padding: 10px;
  overflow-y: hidden;
}

.domain-alert-page {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: 8px;
  height: calc(100vh - 124px);
  max-height: none;
  padding: 7px;
  overflow: hidden;
  border-radius: 0;
  background: transparent;
}

.domain-workspace {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  min-height: 0;
}

.domain-page-tabs {
  padding: 0 10px;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-bottom: 0;
  border-radius: 12px 12px 0 0;
  background: #ffffff;
}

.domain-page-tabs :deep(.el-tabs__header) {
  margin: 0;
}

.domain-page-tabs :deep(.el-tabs__content) {
  display: none;
}

.ops-page-header {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  padding: 8px 12px;
  border: 1px solid rgba(223, 231, 242, 0.96);
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
  margin: 0;
  color: var(--aiops-text-primary);
  font-size: 20px;
  line-height: 1.2;
  letter-spacing: 0;
  white-space: nowrap;
}

.ops-page-header__divider {
  width: 1px;
  height: 22px;
  background: #dfe7f2;
}

.ops-page-header__content p {
  margin: 0;
  overflow: hidden;
  color: var(--aiops-text-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.domain-alert-page :deep(.alert-center-card.el-card) {
  height: 100%;
  min-height: 0;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 12px !important;
  background: #ffffff;
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
  backdrop-filter: none;
}

.domain-alert-page :deep(.alert-center-card::before) {
  display: none;
}

.domain-alert-page :deep(.alert-center-card .el-card__body) {
  box-sizing: border-box;
  height: 100%;
  padding: 8px 10px 10px;
  border-radius: 0;
}

.domain-alert-page :deep(.domain-monitor-card.el-card),
.domain-alert-page :deep(.ssl-certificate-card.el-card) {
  height: auto;
  border-radius: 0 0 12px 12px !important;
}

.domain-header-summary {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-left: auto;
  padding: 7px 14px;
  border: 1px solid rgba(223, 231, 242, 0.92);
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.7);
}

.domain-header-summary span {
  display: flex;
  align-items: baseline;
  gap: 6px;
}

.domain-header-summary em {
  color: var(--aiops-text-muted);
  font-size: 12px;
  font-style: normal;
}

.domain-header-summary strong {
  color: var(--aiops-text-primary);
  font-size: 18px;
}

.domain-header-summary strong.is-success { color: #2d9c72; }
.domain-header-summary strong.is-warning { color: #b97a1e; }
.domain-header-summary strong.is-danger { color: #d85c66; }
.domain-header-summary i { width: 1px; height: 18px; background: #dfe7f2; }

.domain-filter-section {
  margin-bottom: 6px;
}

.domain-tag-filter {
  width: 160px;
  flex-basis: 160px;
}

.domain-toolbar {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 2px 2px 8px;
}

.domain-toolbar-actions,
.domain-quick-filters {
  display: flex;
  align-items: center;
  gap: 8px;
}

.domain-quick-filters button {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-height: 30px;
  padding: 0 11px;
  border: 1px solid rgba(243, 178, 75, 0.42);
  border-radius: 10px;
  background: rgba(243, 178, 75, 0.09);
  color: #9c6b20;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.domain-quick-filters button.danger {
  border-color: rgba(239, 107, 115, 0.34);
  background: rgba(239, 107, 115, 0.08);
  color: #cf5560;
}

.domain-quick-filters button.active {
  box-shadow: inset 0 0 0 1px currentColor;
  background: rgba(243, 178, 75, 0.17);
}

.domain-quick-filters button.danger.active { background: rgba(239, 107, 115, 0.15); }

.domain-table-shell {
  border-radius: 14px;
}

.domain-table-shell :deep(.el-table),
.domain-table-shell :deep(.el-table__inner-wrapper) {
  border-radius: 14px;
}

.domain-name-link {
  appearance: none;
  border: 0;
  background: none;
  padding: 0;
  color: #355f9f;
  font: inherit;
  font-weight: 700;
  cursor: pointer;
}

.domain-name-link:hover { color: var(--aiops-brand); }

.domain-tags {
  display: flex;
  gap: 4px;
  margin-top: 3px;
}

.domain-tags :deep(.el-tag) {
  height: 18px;
  padding: 0 6px;
  font-size: 10px;
}

.domain-certificate-time {
  margin-bottom: 3px;
  color: var(--aiops-text-secondary);
  font-size: 12px;
}

.alert-operation-link:disabled {
  opacity: 0.45;
  cursor: wait;
}

.domain-form-grid :deep(.el-input-number),
.domain-form-select {
  width: 100%;
}

.domain-form-hint {
  margin-left: 8px;
  color: var(--aiops-text-muted);
  font-size: 12px;
  white-space: nowrap;
}

.ssl-key-filter {
  width: 190px;
  min-width: 190px;
}

.ssl-certificate-toolbar {
  min-height: 40px;
}

.ssl-refresh-state {
  color: #9c6b20;
  font-size: 12px;
}

.ssl-domain-name {
  color: #355f9f;
  font-weight: 700;
}

.ssl-error-text {
  color: #c44f59;
}

.ssl-certificate-operations .alert-operation-link {
  display: inline-flex;
  align-items: center;
  gap: 3px;
}

.ssl-certificate-operations .alert-operation-link:disabled {
  cursor: not-allowed;
}

.ssl-certificate-detail-body {
  min-height: 160px;
}

.ssl-pem-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 10px;
}

.ssl-pem-heading h4 {
  margin-bottom: 0;
}

.ssl-pem-panel pre {
  max-height: 280px;
  margin: 0;
  padding: 12px;
  overflow: auto;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 8px;
  background: #f7f9fc;
  color: #334155;
  font-family: Consolas, Monaco, monospace;
  font-size: 12px;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-all;
}

.ssl-form-control,
.ssl-form-field {
  width: 100%;
}

.ssl-form-field {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.ssl-form-help {
  color: var(--aiops-text-muted);
  font-size: 12px;
  line-height: 1.4;
}

.ssl-deploy-host-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.ssl-deploy-host-option small {
  color: var(--aiops-text-muted);
}

.ssl-deploy-log-body {
  min-height: 220px;
}

.ssl-deploy-log-table {
  margin-bottom: 16px;
}

.ssl-deploy-log-detail {
  padding-top: 14px;
  border-top: 1px solid rgba(223, 231, 242, 0.96);
}

.ssl-deploy-log-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.ssl-deploy-log-heading > div {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.ssl-deploy-log-heading strong {
  color: var(--aiops-text-primary);
  font-size: 15px;
}

.ssl-deploy-log-heading span,
.ssl-deploy-log-meta {
  color: var(--aiops-text-muted);
  font-size: 12px;
}

.ssl-deploy-log-meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin: 12px 0;
  word-break: break-all;
}

.ssl-deploy-log-detail pre {
  box-sizing: border-box;
  max-height: 380px;
  margin: 0;
  padding: 12px;
  overflow: auto;
  border: 1px solid rgba(223, 231, 242, 0.96);
  border-radius: 6px;
  background: #18212f;
  color: #d7e2ef;
  font-family: Consolas, Monaco, monospace;
  font-size: 12px;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
}

.ssl-deploy-error {
  margin: 10px 0 0;
  color: #c44f59;
  font-size: 13px;
  line-height: 1.6;
  word-break: break-word;
}

.domain-failure-panel {
  border-color: rgba(239, 107, 115, 0.28);
  background: rgba(255, 246, 247, 0.76);
}

.domain-failure-panel p {
  margin: 0;
  color: #b94b55;
  font-size: 13px;
  line-height: 1.7;
  word-break: break-word;
}

@media (max-width: 1100px) {
  .domain-header-summary { display: none; }
  .domain-toolbar { align-items: flex-start; flex-direction: column; }
}

@media (max-width: 720px) {
  .domain-alert-page { padding: 8px; }
  .domain-filter-section :deep(.el-form-item) { width: 100%; margin-right: 0; }
  .domain-filter-section :deep(.el-form-item__content),
  .domain-filter-section .alert-filter-input,
  .domain-filter-section .alert-filter-small { width: 100%; flex-basis: 100%; }
  .domain-quick-filters { width: 100%; flex-wrap: wrap; }
  .domain-form-grid { grid-template-columns: 1fr; }
  .domain-form-grid .alert-form-full { grid-column: auto; }
  .domain-page-tabs { padding: 0 4px; }
  .ssl-key-filter { width: 100%; min-width: 100%; }
  .ssl-certificate-toolbar { align-items: flex-start; }
}
</style>
