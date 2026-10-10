<script setup>
import { ref, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import k8sApi from '@/api/k8s'
import ClusterTable from './clusters/ClusterTable.vue'
import CredentialDialog from './clusters/dialogs/CredentialDialog.vue'
import RegisterDialog from './clusters/dialogs/RegisterDialog.vue'
import EditDialog from './clusters/dialogs/EditDialog.vue'

const loading = ref(false)
const clusterTableRef = ref(null)
const refreshTrigger = ref(0)
const registerDialogVisible = ref(false)
const editDialogVisible = ref(false)
const credentialDialogVisible = ref(false)

const currentEditCluster = reactive({
  id: '',
  clusterName: '',
  version: '',
  credential: '',
  description: '',
  monitorDatasourceId: '',
  monitorDatasourceType: '',
  monitorDatasourceName: ''
})

const currentCredential = reactive({
  clusterName: '',
  content: ''
})

const refreshClusterTable = () => {
  refreshTrigger.value++
  if (clusterTableRef.value) {
    clusterTableRef.value.handleQuery()
  }
}

const handleClusterSync = () => {
  refreshTrigger.value++
}

const handleClusterDelete = () => {
  refreshTrigger.value++
}

const handleClusterEdit = async (row) => {
  try {
    loading.value = true
    const response = await k8sApi.getClusterById(row.id)
    const responseData = response.data || response

    if (responseData.code === 200 || responseData.success) {
      const clusterData = responseData.data.cluster || responseData.data

      currentEditCluster.id = clusterData.id || row.id
      currentEditCluster.clusterName = clusterData.name || row.clusterName
      currentEditCluster.version = clusterData.version || row.version || ''
      currentEditCluster.credential = clusterData.credential || clusterData.kubeconfig || row.credential || ''
      currentEditCluster.description = clusterData.description || clusterData.remark || row.description || ''
      currentEditCluster.monitorDatasourceId = clusterData.monitorDatasourceId ?? row.monitorDatasourceId ?? ''
      currentEditCluster.monitorDatasourceType = clusterData.monitorDatasourceType || row.monitorDatasourceType || ''
      currentEditCluster.monitorDatasourceName = clusterData.monitorDatasourceName || row.monitorDatasourceName || ''

      editDialogVisible.value = true
    } else {
      ElMessage.error(responseData.message || '获取集群详情失败')
    }
  } catch (error) {
    console.error('获取集群详情失败:', error)
    ElMessage.error('获取集群详情失败，请检查网络连接')
  } finally {
    loading.value = false
  }
}

const handleViewCredential = (row) => {
  currentCredential.clusterName = row.clusterName
  currentCredential.content = row.credential || ''
  credentialDialogVisible.value = true
}

const handleRegister = () => {
  registerDialogVisible.value = true
}

const handleRegisterSuccess = () => {
  refreshClusterTable()

  setTimeout(() => {
    refreshClusterTable()
  }, 3000)
}

const handleRegisterClosed = () => {}

const handleEditSuccess = () => {
  refreshClusterTable()
}

const handleEditClosed = () => {}
</script>

<template>
  <div class="k8s-cluster-management" v-loading="loading">
    <header class="ops-page-header">
      <span class="ops-page-header__icon" aria-hidden="true">
        <img src="@/assets/image/k8s.svg" alt="">
      </span>
      <div class="ops-page-header__content">
        <h1>K8s 集群管理</h1>
        <span class="ops-page-header__divider" aria-hidden="true"></span>
        <p>统一接入、同步与维护 Kubernetes 集群及其监控凭证。</p>
      </div>
      <div class="header-actions">
        <el-button type="success" :icon="Plus" class="primary-action" @click="handleRegister">
          注册集群
        </el-button>
      </div>
    </header>

    <section class="page-card">
      <ClusterTable
        ref="clusterTableRef"
        :refresh-trigger="refreshTrigger"
        @cluster-sync="handleClusterSync"
        @cluster-edit="handleClusterEdit"
        @cluster-delete="handleClusterDelete"
        @view-credential="handleViewCredential"
      />
    </section>

    <RegisterDialog
      v-model:visible="registerDialogVisible"
      @success="handleRegisterSuccess"
      @closed="handleRegisterClosed"
    />

    <EditDialog
      v-model:visible="editDialogVisible"
      :cluster="currentEditCluster"
      @success="handleEditSuccess"
      @closed="handleEditClosed"
    />

    <CredentialDialog
      v-model:visible="credentialDialogVisible"
      :credential="currentCredential"
    />
  </div>
</template>

<style scoped>
.k8s-cluster-management {
  --ops-text-primary: #172033;
  --ops-text-muted: #64748b;
  --ops-border: #dbe7f4;
  --ops-border-soft: #e7eef7;
  box-sizing: border-box;
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  gap: 8px;
  height: calc(100vh - 124px);
  min-height: 0;
  padding: 7px;
  box-sizing: border-box;
  overflow: auto;
  background: transparent;
  color: var(--ops-text-primary);
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.page-card {
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 10px;
  overflow: hidden;
  border: 1px solid var(--ops-border);
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.96);
  box-shadow: 0 8px 24px rgba(80, 124, 180, 0.07);
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
  color: #0f172a;
  font-size: 20px;
  font-weight: 700;
  line-height: 1.2;
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

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
  margin-left: auto;
}

.primary-action {
  min-width: 96px;
  height: 34px;
  border-radius: 8px;
  color: #ffffff;
  font-weight: 600;
  transition: background-color 160ms ease, border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease;
}

.primary-action:hover,
.primary-action:focus {
  transform: translateY(-1px);
  box-shadow: 0 6px 14px rgba(103, 194, 58, 0.22);
}

.primary-action.is-disabled,
.primary-action:disabled {
  transform: none;
  opacity: 0.58;
  box-shadow: none;
}

@media (max-width: 768px) {
  .k8s-cluster-management {
    height: calc(100vh - 110px);
    padding: 6px;
  }

  .page-card {
    padding: 8px;
  }

  .ops-page-header {
    align-items: flex-start;
  }

  .ops-page-header__content {
    flex-direction: column;
    align-items: stretch;
    gap: 2px;
  }

  .ops-page-header__divider,
  .ops-page-header__content p {
    display: none;
  }

  .header-actions,
  .primary-action {
    min-width: auto;
  }
}
</style>
