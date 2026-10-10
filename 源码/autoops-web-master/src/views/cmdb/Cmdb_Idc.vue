<template>
  <div class="cmdb-idc-page">
    <header class="idc-hero-panel">
      <div class="idc-hero-copy">
        <span class="idc-hero-icon" aria-hidden="true">
          <el-icon><OfficeBuilding /></el-icon>
        </span>
        <div class="idc-hero-content">
          <h1>机房资产管理</h1>
          <span class="idc-hero-divider" aria-hidden="true"></span>
          <p>统一管理机房、机柜、物理机与虚拟化资源。</p>
        </div>
      </div>
      <div class="idc-hero-metrics" aria-label="机房资产摘要">
        <div>
          <span>机房</span>
          <strong>{{ roomList.length }}</strong>
        </div>
        <div>
          <span>机柜</span>
          <strong>{{ cabinetList.length }}</strong>
        </div>
        <div>
          <span>当前物理机</span>
          <strong>{{ total }}</strong>
        </div>
      </div>
    </header>

    <el-card class="page-card" shadow="never">
      <div class="page-layout">
        <aside ref="treePanel" class="tree-panel" v-loading="loadingTree">
          <div class="panel-head">
            <div>
              <h3>机房树</h3>
              <p>{{ roomList.length }} 个机房 · {{ cabinetList.length }} 个机柜</p>
            </div>
            <div class="head-actions">
              <el-tooltip content="刷新机房树" placement="top">
                <el-button aria-label="刷新机房树" class="idc-refresh-button" type="warning" circle @click="loadTree"><el-icon><Refresh /></el-icon></el-button>
              </el-tooltip>
              <el-tooltip content="创建机房" placement="top">
                <el-button aria-label="创建机房" type="success" circle @click="openRoomDialog('create')"><el-icon><Plus /></el-icon></el-button>
              </el-tooltip>
            </div>
          </div>

          <el-input v-model="treeKeyword" clearable class="tree-search" placeholder="搜索机房或机柜">
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>


          <el-scrollbar class="tree-scroll">
            <el-tree
              ref="idcTree"
              :data="treeData"
              :props="treeProps"
              node-key="nodeKey"
              highlight-current
              :expand-on-click-node="false"
              :filter-node-method="filterTreeNode"
              class="idc-tree"
              @node-click="handleTreeClick"
              @node-expand="handleTreeExpand"
            >
              <template #default="{ data }">
                <div class="tree-node" @contextmenu.prevent.stop="showTreeContextMenu($event, data)">
                  <div class="tree-node-main">
                    <span class="tree-icon" :class="data.type">
                      <img
                        :src="data.type === 'room' ? require('@/assets/image/机房.svg') : require('@/assets/image/机柜.svg')"
                        :alt="data.type === 'room' ? '机房' : '机柜'"
                        class="tree-icon-image"
                      />
                    </span>
                    <div class="tree-node-text" :class="{ 'cabinet-text': data.type === 'cabinet' }">
                      <span class="tree-node-name"><span>{{ data.name }}</span><span v-if="data.type === 'cabinet'" class="tree-node-count">({{ data.assetCount || 0 }})</span></span>
                      <span class="tree-node-meta" v-if="data.type === 'room'">{{ getCabinetCount(data.id) }} 个机柜 / {{ data.assetCount || 0 }} 台资产</span>
                      <span class="tree-node-meta" v-else>{{ data.assetCount || 0 }} 台资产</span>
                    </div>
                  </div>
                </div>
              </template>
            </el-tree>
          </el-scrollbar>
          <div
            v-if="treeContextMenuVisible && treeContextMenuNode"
            ref="treeContextMenu"
            class="tree-context-menu"
            :style="{ left: `${treeContextMenuPosition.x}px`, top: `${treeContextMenuPosition.y}px` }"
            @click.stop
          >
            <template v-if="treeContextMenuNode.type === 'room'">
              <div class="tree-context-menu-item" @click="handleTreeContextAction('create-room')">
                <el-icon><Plus /></el-icon>
                <span>创建机房</span>
              </div>
              <div class="tree-context-menu-item" @click="handleTreeContextAction('create-cabinet')">
                <el-icon><FolderAdd /></el-icon>
                <span>创建机柜</span>
              </div>
              <div class="tree-context-menu-item" @click="handleTreeContextAction('edit-room')">
                <el-icon><Edit /></el-icon>
                <span>修改机房</span>
              </div>
              <div class="tree-context-menu-item danger" @click="handleTreeContextAction('delete-room')">
                <el-icon><Delete /></el-icon>
                <span>删除机房</span>
              </div>
            </template>
            <template v-else>
              <div class="tree-context-menu-item" @click="handleTreeContextAction('edit-cabinet')">
                <el-icon><Edit /></el-icon>
                <span>修改机柜</span>
              </div>
              <div class="tree-context-menu-item danger" @click="handleTreeContextAction('delete-cabinet')">
                <el-icon><Delete /></el-icon>
                <span>删除机柜</span>
              </div>
            </template>
          </div>
        </aside>

        <section class="content-panel">
          <template v-if="activeView === 'physical'">
          <div class="toolbar">
            <div class="toolbar-main">
            <el-form :inline="true" :model="queryParams" class="toolbar-form" @submit.prevent>
              <el-form-item label="关键字">
                <el-input v-model="queryParams.keyword" class="toolbar-keyword-input" size="small" clearable placeholder="IP / 品牌 / 型号 / 用途 / 备注" style="width: 220px" @keyup.enter="handleQuery" />
              </el-form-item>
            </el-form>

            <div class="toolbar-tags">
              <span>当前筛选</span>
              <el-tag type="primary" effect="light">{{ selectedFilterLabel }}</el-tag>
              <el-tag v-if="queryParams.keyword" type="info">关键字：{{ queryParams.keyword }}</el-tag>
            </div>

            <div class="toolbar-actions">
              <el-button size="small" type="primary" @click="handleQuery"><el-icon><Search /></el-icon>查询</el-button>
              <el-button size="small" type="warning" @click="resetQuery"><el-icon><RefreshRight /></el-icon>重置</el-button>
              <el-button size="small" type="success" @click="openAssetDialog('create')"><el-icon><Plus /></el-icon>新建物理机</el-button>
              <el-button size="small" type="warning" @click="openImportDialog"><el-icon><Upload /></el-icon>导入物理机</el-button>
            </div>
            </div>
            <div class="toolbar-divider"></div>
            <div class="view-tabs">
              <button type="button" class="view-tab" :class="{ active: activeView === 'room' }" @click="handleViewSwitch('room')">机房</button>
              <button type="button" class="view-tab" :class="{ active: activeView === 'cabinet' }" @click="handleViewSwitch('cabinet')">机柜</button>
              <button type="button" class="view-tab" :class="{ active: activeView === 'physical' }" @click="handleViewSwitch('physical')">物理机</button>
              <button type="button" class="view-tab" :class="{ active: activeView === 'vm' }" @click="handleViewSwitch('vm')">虚拟机</button>
              <button type="button" class="view-tab" :class="{ active: activeView === 'hardware' }" @click="handleViewSwitch('hardware')">硬件监控</button>
            </div>
          </div>

          <div class="table-card">
            <div class="table-head">
              <div>
                <h3>物理机资产</h3>
                <p>{{ selectedFilterLabel }} · 共 {{ total }} 台</p>
              </div>
              <el-button size="small" class="idc-refresh-button" type="warning" @click="loadAssets"><el-icon><Refresh /></el-icon>刷新列表</el-button>
            </div>

            <el-table v-loading="loadingTable" :data="assetList" border row-key="id" class="asset-table">
              <el-table-column type="index" label="#" width="56" />
              <el-table-column label="物理机IP" min-width="160">
                <template #default="{ row }">
                  <div class="copy-hover-cell">
                    <img
                      src="@/assets/image/物理机.svg"
                      alt="物理机"
                      class="asset-type-icon"
                    >
                    <el-button
                      v-if="row.ip"
                      link
                      type="primary"
                      class="asset-ip-link"
                      @click="openPhysicalHostDetail(row)"
                    >
                      {{ row.ip }}
                    </el-button>
                    <span v-else>-</span>
                    <el-icon
                      v-if="row.ip"
                      class="copy-trigger"
                      @click.stop="copyText(row.ip, '物理机IP')"
                    >
                      <DocumentCopy />
                    </el-icon>
                  </div>
                </template>
              </el-table-column>
              <el-table-column prop="vmCount" label="虚拟机" min-width="110" align="center">
                <template #default="{ row }">
                  <el-button
                    v-if="row.vmCount > 0"
                    link
                    type="primary"
                    class="asset-ip-link"
                    @click="openVmPanelByAsset(row)"
                  >
                    {{ row.vmCount }}
                  </el-button>
                  <span v-else>0</span>
                </template>
              </el-table-column>
              <el-table-column label="远程管理卡" min-width="170" show-overflow-tooltip>
                <template #default="{ row }">
                  <div class="copy-hover-cell">
                    <img
                      src="@/assets/image/监控.svg"
                      alt="监控"
                      class="asset-monitor-icon"
                    >
                    <el-button
                      v-if="row.remoteManagementCard"
                      link
                      type="primary"
                      class="asset-ip-link"
                      @click="openHardwareMonitorByAsset(row)"
                    >
                      {{ row.remoteManagementCard }}
                    </el-button>
                    <span v-else>-</span>
                    <el-icon
                      v-if="row.remoteManagementCard"
                      class="copy-trigger"
                      @click.stop="copyText(row.remoteManagementCard, '远程管理卡')"
                    >
                      <DocumentCopy />
                    </el-icon>
                  </div>
                </template>
              </el-table-column>
              <el-table-column label="机房" min-width="140" show-overflow-tooltip>
                <template #default="{ row }">{{ getRoomName(row) || '-' }}</template>
              </el-table-column>
              <el-table-column label="机柜" min-width="120" show-overflow-tooltip>
                <template #default="{ row }">{{ getCabinetName(row) || '-' }}</template>
              </el-table-column>

              <el-table-column prop="brand" label="品牌" min-width="120" show-overflow-tooltip />
              <el-table-column prop="model" label="型号" min-width="150" show-overflow-tooltip />
              <el-table-column prop="purpose" label="用途" min-width="160" show-overflow-tooltip />

              <el-table-column label="年限" min-width="100" align="center">
                <template #default="{ row }">{{ formatUsageYears(row.purchaseDate) }}</template>
              </el-table-column>
              <el-table-column label="维保截止" min-width="120">
                <template #default="{ row }">{{ formatDate(row.warrantyEndDate) }}</template>
              </el-table-column>
              <el-table-column label="更新时间" min-width="160">
                <template #default="{ row }">{{ formatDateTime(row.updateTime || row.createTime) }}</template>
              </el-table-column>
              <el-table-column label="操作" fixed="right" width="140">
                <template #default="{ row }">
                  <el-button link type="primary" @click="openAssetDialog('edit', row)">编辑</el-button>
                  <el-button link type="danger" @click="removeAsset(row)">删除</el-button>
                </template>
              </el-table-column>
            </el-table>

            <div class="pagination">
              <el-pagination :current-page="queryParams.page" :page-size="queryParams.pageSize" :page-sizes="[10,20,50,100]" layout="total, sizes, prev, pager, next, jumper" :total="total" @size-change="handleSizeChange" @current-change="handleCurrentChange" />
            </div>
          </div>
          </template>

          <IdcVmPanel v-else-if="activeView === 'vm'" :key="vmPanelKey" :initial-filter="vmViewContext" :active-view="activeView" @switch-view="handleViewSwitch" />
          <IdcHardwareMonitorPanel
            v-else-if="activeView === 'hardware'"
            :key="hardwarePanelKey"
            :initial-context="hardwareViewContext"
            :active-view="activeView"
            @switch-view="handleViewSwitch"
          />
          <IdcRoomPanel
            v-else-if="activeView === 'room'"
            :active-view="activeView"
            :selected-room="selectedRoomForView"
            @switch-view="handleViewSwitch"
            @select-cabinet="handleRoomCabinetSelect"
          />
          <IdcCabinetPanel
            v-else
            :active-view="activeView"
            :selected-cabinet="selectedCabinetForView"
            :selected-room="selectedRoomForView"
            @switch-view="handleViewSwitch"
            @open-hardware-monitor="openHardwareMonitorInNewTab"
          />
        </section>
      </div>
    </el-card>

    <el-dialog v-model="roomDialogVisible" class="idc-form-dialog" modal-class="idc-form-dialog-overlay" :title="roomDialogMode === 'create' ? '创建机房' : '编辑机房'" width="560px" @closed="resetRoomDialog">
      <el-form ref="roomFormRef" :model="roomForm" :rules="roomRules" label-position="top" class="idc-dialog-form">
        <el-form-item label="机房名称" prop="name"><el-input v-model="roomForm.name" placeholder="请输入机房名称" /></el-form-item>
        <el-form-item label="机房备注"><el-input v-model="roomForm.remark" type="textarea" :rows="3" placeholder="请输入机房备注" /></el-form-item>
        <el-form-item v-if="roomDialogMode === 'create'" label="关联机柜">
          <el-input v-model="roomForm.cabinetText" type="textarea" :rows="5" placeholder="一行一个机柜，或用逗号分隔，例如：01柜01列&#10;01柜02列&#10;01柜03列" />
          <div class="form-tip">请按“XX柜XX列”格式填写，例如“01柜01列”，否则系统无法识别。</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <div class="idc-dialog-footer">
          <el-button @click="roomDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="submitting" @click="submitRoom">保存</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="cabinetDialogVisible" class="idc-form-dialog" modal-class="idc-form-dialog-overlay" :title="cabinetDialogMode === 'create' ? '创建机柜' : '编辑机柜'" width="520px" @closed="resetCabinetDialog">
      <el-form ref="cabinetFormRef" :model="cabinetForm" :rules="cabinetRules" label-position="top" class="idc-dialog-form">
        <el-form-item label="所属机房" prop="roomId">
          <el-select v-model="cabinetForm.roomId" clearable filterable style="width: 100%">
            <el-option v-for="room in roomList" :key="room.id" :label="room.name" :value="room.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="机柜名称" prop="name"><el-input v-model="cabinetForm.name" placeholder="请输入机柜名称" /></el-form-item>
        <el-form-item label="机柜备注"><el-input v-model="cabinetForm.remark" type="textarea" :rows="3" placeholder="请输入机柜备注" /></el-form-item>
      </el-form>
      <template #footer>
        <div class="idc-dialog-footer">
          <el-button @click="cabinetDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="submitting" @click="submitCabinet">保存</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="assetDialogVisible" class="idc-form-dialog idc-asset-dialog" modal-class="idc-form-dialog-overlay" :title="assetDialogMode === 'create' ? '新建物理机资产' : '编辑物理机资产'" width="920px" @closed="resetAssetDialog">
      <el-form ref="assetFormRef" :model="assetForm" :rules="assetRules" label-position="top" class="idc-dialog-form idc-asset-form">
        <el-row :gutter="18">
          <el-col :span="12"><el-form-item label="所属机房" prop="roomId"><el-select v-model="assetForm.roomId" clearable filterable style="width: 100%" @change="handleAssetRoomChange"><el-option v-for="room in roomList" :key="room.id" :label="room.name" :value="room.id" /></el-select></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="所属机柜"><el-select v-model="assetForm.cabinetId" clearable filterable style="width: 100%"><el-option v-for="cabinet in assetCabinetOptions" :key="cabinet.id" :label="cabinet.name" :value="cabinet.id" /></el-select></el-form-item></el-col>
        </el-row>
        <el-row :gutter="18">
          <el-col :span="12"><el-form-item label="IP 地址" prop="ip"><el-input v-model="assetForm.ip" placeholder="请输入物理机 IP" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="品牌"><el-input v-model="assetForm.brand" placeholder="例如 Dell / HPE / Lenovo" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="18">
          <el-col :span="12"><el-form-item label="型号"><el-input v-model="assetForm.model" placeholder="请输入服务器型号" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="用途"><el-input v-model="assetForm.purpose" placeholder="请输入服务器用途" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="18">
          <el-col :span="8"><el-form-item label="CPU"><el-input v-model="assetForm.cpu" placeholder="CPU 信息" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="内存"><el-input v-model="assetForm.memory" placeholder="内存容量" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="磁盘"><el-input v-model="assetForm.disk" placeholder="磁盘信息" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="18">
          <el-col :span="12"><el-form-item label="远程管理卡"><el-input v-model="assetForm.remoteManagementCard" placeholder="例如 iDRAC / iLO IP" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="使用年限"><el-input-number v-model="assetForm.usageYears" :min="0" :max="30" controls-position="right" style="width: 100%" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="18">
          <el-col :span="8"><el-form-item label="采购日期"><el-date-picker v-model="assetForm.purchaseDate" type="date" value-format="YYYY-MM-DD" placeholder="选择采购日期" style="width: 100%" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="维保开始"><el-date-picker v-model="assetForm.warrantyStartDate" type="date" value-format="YYYY-MM-DD" placeholder="选择开始日期" style="width: 100%" /></el-form-item></el-col>
          <el-col :span="8"><el-form-item label="维保截止"><el-date-picker v-model="assetForm.warrantyEndDate" type="date" value-format="YYYY-MM-DD" placeholder="选择截止日期" style="width: 100%" /></el-form-item></el-col>
        </el-row>
        <el-row :gutter="18">
          <el-col :span="12"><el-form-item label="资产评估"><el-input v-model="assetForm.assetEvaluation" placeholder="请输入资产评估" /></el-form-item></el-col>
          <el-col :span="12"><el-form-item label="备注"><el-input v-model="assetForm.remark" placeholder="请输入备注" /></el-form-item></el-col>
        </el-row>
      </el-form>
      <template #footer>
        <div class="idc-dialog-footer">
          <el-button @click="assetDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="submitting" @click="submitAsset">保存</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="importDialogVisible" class="idc-form-dialog" modal-class="idc-form-dialog-overlay" title="导入物理机资产" width="520px" @closed="resetImportDialog">
      <div class="import-box">
        <div class="import-tip">请先下载模板，按模板填写机房、机柜和物理机信息后再导入。</div>
        <el-button type="primary" plain :loading="templateLoading" @click="downloadTemplate"><el-icon><Download /></el-icon>下载导入模板</el-button>
        <el-upload drag :auto-upload="false" :show-file-list="false" accept=".xlsx,.xls" :on-change="handleImportFileChange">
          <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
          <div class="el-upload__text">拖拽文件到此处，或点击选择 Excel 文件</div>
        </el-upload>
        <div v-if="importForm.file" class="file-line">
          <el-icon><Document /></el-icon>
          <span>{{ importForm.file.name }}</span>
          <el-button link type="danger" @click="handleImportFileRemove">移除</el-button>
        </div>
        <el-progress v-if="importUploading" :percentage="importProgress" :status="importProgress === 100 ? 'success' : ''" />
      </div>
      <template #footer>
        <div class="idc-dialog-footer">
          <el-button @click="importDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="importUploading" :disabled="!importForm.file" @click="submitImport">开始导入</el-button>
        </div>
      </template>
    </el-dialog>
  </div>
</template>

<script>
import IdcVmPanel from '@/views/cmdb/Idc/IdcVmPanel.vue'
import IdcCabinetPanel from '@/views/cmdb/Idc/IdcCabinetPanel.vue'
import IdcHardwareMonitorPanel from '@/views/cmdb/Idc/IdcHardwareMonitorPanel.vue'
import IdcRoomPanel from '@/views/cmdb/Idc/IdcRoomPanel.vue'

const makeRoomForm = () => ({ id: null, name: '', remark: '', cabinetText: '' })
const makeCabinetForm = roomId => ({ id: null, roomId: roomId || '', name: '', remark: '' })
const makeAssetForm = (roomId = '', cabinetId = '') => ({ id: null, roomId: roomId || '', cabinetId: cabinetId || '', ip: '', brand: '', model: '', cpu: '', disk: '', memory: '', purpose: '', remoteManagementCard: '', remark: '', purchaseDate: '', warrantyStartDate: '', warrantyEndDate: '', usageYears: undefined, assetEvaluation: '' })
const makeVmViewContext = asset => ({ physicalAssetId: asset?.id || '', physicalIp: asset?.ip || '' })
const makeHardwareViewContext = asset => ({ assetId: asset?.id || asset?.assetId || '', remoteManagementCard: asset?.remoteManagementCard || '', ip: asset?.ip || '', brand: asset?.brand || '', model: asset?.model || '', purpose: asset?.purpose || '' })

export default {
  name: 'CmdbIdc',
  components: { IdcVmPanel, IdcCabinetPanel, IdcHardwareMonitorPanel, IdcRoomPanel },
  data() {
    return {
      loadingTree: false, loadingTable: false, submitting: false, templateLoading: false, importUploading: false, importProgress: 0,
      treeKeyword: '', treeData: [], treeProps: { children: 'children', label: 'name' }, assetList: [], total: 0, selectedTreeNode: null, expandedRoomKey: null,
      roomList: [], cabinetList: [],
      treeContextMenuVisible: false, treeContextMenuPosition: { x: 0, y: 0 }, treeContextMenuNode: null,
      queryParams: { page: 1, pageSize: 10, keyword: '', roomId: '', cabinetId: '' },
      roomDialogVisible: false, roomDialogMode: 'create', roomForm: makeRoomForm(), roomRules: { name: [{ required: true, message: '请输入机房名称', trigger: 'blur' }] },
      cabinetDialogVisible: false, cabinetDialogMode: 'create', cabinetForm: makeCabinetForm(), cabinetRules: { roomId: [{ required: true, message: '请选择所属机房', trigger: 'change' }], name: [{ required: true, message: '请输入机柜名称', trigger: 'blur' }] },
      assetDialogVisible: false, assetDialogMode: 'create', assetForm: makeAssetForm(), assetRules: { roomId: [{ required: true, message: '请选择所属机房', trigger: 'change' }], ip: [{ required: true, message: '请输入物理机 IP', trigger: 'blur' }] },
      importDialogVisible: false, importForm: { file: null },
      activeView: 'physical',
      vmViewContext: makeVmViewContext(),
      hardwareViewContext: makeHardwareViewContext(),
      vmPanelKey: 0,
      hardwarePanelKey: 0
    }
  },
  computed: {
    selectedRoomId() { if (!this.selectedTreeNode) return null; return this.selectedTreeNode.type === 'room' ? this.selectedTreeNode.id : this.selectedTreeNode.roomId },
    selectedFilterLabel() { if (!this.selectedTreeNode) return '全部资产'; return this.selectedTreeNode.type === 'room' ? `机房：${this.selectedTreeNode.name}` : `机柜：${this.selectedTreeNode.name}` },
    assetCabinetOptions() { if (!this.assetForm.roomId) return []; return this.getCabinetsByRoomId(this.num(this.assetForm.roomId)).map(item => ({ id: item.id, name: item.name })) },
    selectedCabinetForView() {
      if (!this.selectedTreeNode || this.selectedTreeNode.type !== 'cabinet') return null
      return this.selectedTreeNode
    },
    selectedRoomForView() {
      if (!this.selectedTreeNode) return null
      if (this.selectedTreeNode.type === 'room') return this.selectedTreeNode
      return this.getRoomById(this.selectedTreeNode.roomId)
    }
  },
  watch: { treeKeyword(value) { this.$nextTick(() => this.$refs.idcTree?.filter(value)) } },
  created() { this.initPage() },
  mounted() {
    document.addEventListener('click', this.hideTreeContextMenu)
    document.addEventListener('contextmenu', this.hideTreeContextMenu)
  },
  beforeUnmount() {
    document.removeEventListener('click', this.hideTreeContextMenu)
    document.removeEventListener('contextmenu', this.hideTreeContextMenu)
  },
  methods: {
    async initPage() {
      this.applyRouteContext()
      await Promise.all([this.loadTree(false), this.loadAssets()])
    },
    applyRouteContext() {
      const query = this.$route?.query || {}
      const view = String(query.view || '').trim()
      const hardwareContext = makeHardwareViewContext({
        id: query.assetId,
        remoteManagementCard: query.remoteManagementCard,
        ip: query.ip,
        brand: query.brand,
        model: query.model,
        purpose: query.purpose
      })

      if (view === 'hardware' || hardwareContext.remoteManagementCard || hardwareContext.assetId) {
        this.hardwareViewContext = hardwareContext
        this.hardwarePanelKey += 1
        this.activeView = 'hardware'
        return
      }

      if (view === 'cabinet') {
        const cabinetId = this.num(query.cabinetId, '')
        const roomId = this.num(query.roomId, '')
        const cabinetName = String(query.cabinetName || '').trim()
        if (cabinetId || (roomId && cabinetName)) {
          this.selectedTreeNode = {
            id: cabinetId,
            cabinetId,
            roomId,
            name: cabinetName,
            cabinetName,
            type: 'cabinet',
            nodeKey: cabinetId ? `cabinet-${cabinetId}` : `cabinet-${roomId}-${cabinetName}`
          }
          this.queryParams.roomId = roomId
          this.queryParams.cabinetId = cabinetId
        }
        this.activeView = 'cabinet'
        return
      }

      if (['room', 'physical', 'vm'].includes(view)) {
        this.activeView = view
      }
    },
    handleViewSwitch(view) {
      if (view === 'physical') {
        this.switchToPhysicalView()
        return
      }
      if (view === 'hardware') {
        this.switchToHardwareView()
        return
      }
      if (view === 'cabinet') {
        this.switchToCabinetView()
        return
      }
      if (view === 'room') {
        this.switchToRoomView()
        return
      }
      this.switchToVmView()
    },
    switchToPhysicalView() {
      this.activeView = 'physical'
      this.loadAssets()
    },
    switchToVmView() {
      this.activeView = 'vm'
      this.vmViewContext = makeVmViewContext()
      this.vmPanelKey += 1
    },
    switchToHardwareView(context) {
      if (context) {
        this.hardwareViewContext = makeHardwareViewContext(context)
        this.hardwarePanelKey += 1
      }
      this.activeView = 'hardware'
    },
    switchToCabinetView() {
      this.activeView = 'cabinet'
    },
    switchToRoomView() {
      this.activeView = 'room'
    },
    openVmPanelByAsset(row) {
      this.activeView = 'vm'
      this.vmViewContext = makeVmViewContext(row)
      this.vmPanelKey += 1
    },
    openHardwareMonitorByAsset(row) {
      this.switchToHardwareView(row)
    },
    openHardwareMonitorInNewTab(context) {
      const hardwareContext = makeHardwareViewContext(context)
      if (!hardwareContext.remoteManagementCard && !hardwareContext.assetId) {
        this.$message.warning('当前物理机缺少远程管理卡或资产ID，无法打开硬件监控')
        return
      }

      const route = this.$router.resolve({
        path: '/cmdb/idc',
        query: this.compact({
          view: 'hardware',
          assetId: hardwareContext.assetId,
          remoteManagementCard: hardwareContext.remoteManagementCard,
          ip: hardwareContext.ip,
          brand: hardwareContext.brand,
          model: hardwareContext.model,
          purpose: hardwareContext.purpose
        })
      })
      const opened = window.open(route.href, '_blank')
      if (opened) {
        opened.opener = null
      } else {
        this.$message.warning('浏览器拦截了新标签页，请允许弹窗后重试')
      }
    },
    pick(obj, keys, fallback = '') { if (!obj || typeof obj !== 'object') return fallback; for (const key of keys) { const value = obj[key]; if (value !== undefined && value !== null && value !== '') return value } return fallback },
    num(value, fallback = null) { if (value === '' || value === undefined || value === null) return fallback; const next = Number(value); return Number.isNaN(next) ? fallback : next },
    list(payload) { if (Array.isArray(payload)) return payload; if (!payload || typeof payload !== 'object') return []; for (const key of ['list', 'items', 'rows', 'records', 'result']) { if (Array.isArray(payload[key])) return payload[key] } return [] },
    page(payload) { const list = this.list(payload); return { list, total: this.num(this.pick(payload, ['total', 'count', 'totalCount', 'total_count'], list.length), list.length) } },
    normalizeRoom(item) { const id = this.num(this.pick(item, ['id', 'roomId', 'room_id']), 0); return { ...item, id, name: String(this.pick(item, ['name', 'roomName', 'room_name'], '未命名机房')), remark: String(this.pick(item, ['remark'], '')), assetCount: this.num(this.pick(item, ['assetCount', 'asset_count']), 0), type: 'room', nodeKey: `room-${id}` } },
    normalizeCabinet(item) { const id = this.num(this.pick(item, ['id', 'cabinetId', 'cabinet_id']), 0); return { ...item, id, roomId: this.num(this.pick(item, ['roomId', 'room_id']), 0), name: String(this.pick(item, ['name', 'cabinetName', 'cabinet_name'], '未命名机柜')), remark: String(this.pick(item, ['remark'], '')), assetCount: this.num(this.pick(item, ['assetCount', 'asset_count']), 0), type: 'cabinet', nodeKey: `cabinet-${id}` } },
    normalizeAsset(item) { return { ...item, id: this.num(this.pick(item, ['id']), 0), roomId: this.num(this.pick(item, ['roomId', 'room_id']), ''), roomName: String(this.pick(item, ['roomName', 'room_name'], '')), cabinetId: this.num(this.pick(item, ['cabinetId', 'cabinet_id']), ''), cabinetName: String(this.pick(item, ['cabinetName', 'cabinet_name'], '')), vmCount: this.num(this.pick(item, ['vmCount', 'vm_count']), 0), ip: String(this.pick(item, ['ip'], '')), brand: String(this.pick(item, ['brand'], '')), model: String(this.pick(item, ['model'], '')), cpu: String(this.pick(item, ['cpu'], '')), disk: String(this.pick(item, ['disk'], '')), memory: String(this.pick(item, ['memory'], '')), purpose: String(this.pick(item, ['purpose'], '')), remoteManagementCard: String(this.pick(item, ['remoteManagementCard', 'remote_management_card'], '')), remark: String(this.pick(item, ['remark'], '')), purchaseDate: this.pick(item, ['purchaseDate', 'purchase_date'], ''), warrantyStartDate: this.pick(item, ['warrantyStartDate', 'warranty_start_date'], ''), warrantyEndDate: this.pick(item, ['warrantyEndDate', 'warranty_end_date'], ''), usageYears: this.num(this.pick(item, ['usageYears', 'usage_years']), ''), assetEvaluation: String(this.pick(item, ['assetEvaluation', 'asset_evaluation'], '')), createTime: this.pick(item, ['createTime', 'create_time'], ''), updateTime: this.pick(item, ['updateTime', 'update_time'], '') } },
    normalizeHost(item) { return { id: this.num(this.pick(item, ['id', 'hostId']), null), ip: String(this.pick(item, ['ip', 'privateIp', 'private_ip', 'sshIp', 'ssh_ip'], '')) } },
    extractHostList(payload) { const list = this.list(payload); if (list.length) return list; if (payload && typeof payload === 'object' && this.pick(payload, ['id', 'hostId'], null) !== null) return [payload]; return [] },
    async queryHostByIp(ip) { const { data: hostRes } = await this.$api.GetCmdbHostsByIP(ip, { page: 1, pageSize: 10 }); if (hostRes.code !== 200) throw new Error(hostRes.message || '通过 IP 查询主机失败'); const hosts = this.extractHostList(hostRes.data).map(item => this.normalizeHost(item)); return hosts.find(item => item.ip === ip) || hosts[0] || null },
    async openPhysicalHostDetail(row) {
      const ip = String(row?.ip || '').trim()
      if (!ip) {
        this.$message.warning('当前物理机 IP 为空')
        return
      }
      try {
        const matchedHost = await this.queryHostByIp(ip)
        if (!matchedHost || !matchedHost.id) {
          this.$message.warning(`未在 host 表中找到 IP 为 ${ip} 的主机`)
          return
        }
        const path = Number(matchedHost?.hostType ?? matchedHost?.host_type) === 2
          ? `/cmdb/host/windows-detail/${matchedHost.id}`
          : `/cmdb/host/detail/${matchedHost.id}`
        this.$router.push(path)
      } catch (error) {
        console.error('openPhysicalHostDetail failed:', error)
        this.$message.error(error.message || '跳转物理机详情失败')
      }
    },
    buildTree() { const cabinetsByRoom = this.cabinetList.reduce((acc, item) => { if (!acc[item.roomId]) acc[item.roomId] = []; acc[item.roomId].push(item); return acc }, {}); this.treeData = this.roomList.map(room => ({ ...room, children: cabinetsByRoom[room.id] || [] })) },
    findNode(target) { if (!target) return null; for (const room of this.treeData) { if (target.type === 'room' && room.id === target.id) return room; if (target.type === 'cabinet') { const cabinet = (room.children || []).find(item => item.id === target.id); if (cabinet) return cabinet } } return null },
    getRoomNodeKey(roomId) { return roomId ? `room-${roomId}` : null },
    setExpandedRoom(roomId) {
      const tree = this.$refs.idcTree
      const nextKey = this.getRoomNodeKey(roomId)
      this.expandedRoomKey = nextKey
      this.$nextTick(() => {
        if (!tree || !tree.store) return
        this.treeData.forEach(room => {
          const node = tree.getNode(room.nodeKey)
          if (node) node.expanded = !!nextKey && room.nodeKey === nextKey
        })
      })
    },
    toggleRoomExpand(roomId) {
      const nextKey = this.getRoomNodeKey(roomId)
      this.setExpandedRoom(this.expandedRoomKey === nextKey ? null : roomId)
    },
    syncExpandedRoomWithSelection() {
      if (!this.selectedTreeNode) {
        this.setExpandedRoom(null)
        return
      }
      const roomId = this.selectedTreeNode.type === 'room' ? this.selectedTreeNode.id : this.selectedTreeNode.roomId
      this.setExpandedRoom(roomId)
    },
    handleTreeExpand(data) {
      if (data.type === 'room') this.setExpandedRoom(data.id)
    },
    selectTreeNode(data, options = {}) {
      this.selectedTreeNode = { ...data }
      if (options.resetPage !== false) this.queryParams.page = 1
      if (data.type === 'room') {
        this.queryParams.roomId = data.id
        this.queryParams.cabinetId = ''
      } else {
        this.queryParams.roomId = data.roomId
        this.queryParams.cabinetId = data.id
      }
    },
    handleRoomCabinetSelect(cabinet) {
      if (!cabinet) return
      const cabinetId = this.num(this.pick(cabinet, ['id', 'cabinetId', 'cabinet_id']), '')
      const roomId = this.num(this.pick(cabinet, ['roomId', 'room_id']), '')
      const cabinetName = String(this.pick(cabinet, ['name', 'cabinetName', 'cabinet_name'], ''))
      if (!cabinetId && (!roomId || !cabinetName)) {
        this.$message.warning('当前机柜缺少ID或机房/机柜名称，无法打开机柜视图')
        return
      }

      const route = this.$router.resolve({
        path: '/cmdb/idc',
        query: this.compact({
          view: 'cabinet',
          cabinetId,
          roomId,
          cabinetName
        })
      })
      const opened = window.open(route.href, '_blank')
      if (opened) {
        opened.opener = null
      } else {
        this.$message.warning('浏览器拦截了新标签页，请允许弹窗后重试')
      }
    },
    getCabinetsByRoomId(roomId) { return this.cabinetList.filter(item => item.roomId === roomId) },
    getCabinetCount(roomId) { return this.getCabinetsByRoomId(roomId).length },
    getRoomById(roomId) { return this.roomList.find(item => item.id === roomId) || null },
    getCabinetById(cabinetId) { return this.cabinetList.find(item => item.id === cabinetId) || null },
    getRoomIdByName(name) { const room = this.roomList.find(item => item.name === name); return room ? room.id : '' },
    getCabinetIdByName(name, roomId) { const cabinet = this.cabinetList.find(item => item.name === name && (!roomId || item.roomId === roomId)); return cabinet ? cabinet.id : '' },
    getRoomName(asset) { if (asset.roomName) return asset.roomName; return this.getRoomById(this.num(asset.roomId))?.name || '' },
    getCabinetName(asset) { if (asset.cabinetName) return asset.cabinetName; return this.getCabinetById(this.num(asset.cabinetId))?.name || '' },
    formatDate(value) { return value ? String(value).replace('T', ' ').slice(0, 10) : '-' },
    formatDateTime(value) { return value ? String(value).replace('T', ' ').slice(0, 19) : '-' },
    parseDate(value) {
      if (!value) return null
      const date = value instanceof Date ? value : new Date(String(value).replace(/-/g, '/'))
      return Number.isNaN(date.getTime()) ? null : date
    },
    usageYearsFromPurchaseDate(purchaseDate, now = new Date()) {
      const date = this.parseDate(purchaseDate)
      if (!date || date > now) return null
      return (now.getTime() - date.getTime()) / (365.25 * 24 * 60 * 60 * 1000)
    },
    formatUsageYears(purchaseDate) {
      const years = this.usageYearsFromPurchaseDate(purchaseDate)
      return years === null ? '-' : `${years.toFixed(1)} 年`
    },
    async copyText(value, label = '内容') {
      const text = String(value || '').trim()
      if (!text) {
        this.$message.warning(`${label}为空，无法复制`)
        return
      }

      try {
        if (navigator.clipboard && window.isSecureContext) {
          await navigator.clipboard.writeText(text)
        } else {
          const textArea = document.createElement('textarea')
          textArea.value = text
          textArea.setAttribute('readonly', 'readonly')
          textArea.style.position = 'fixed'
          textArea.style.left = '-9999px'
          document.body.appendChild(textArea)
          textArea.select()
          document.execCommand('copy')
          document.body.removeChild(textArea)
        }
        this.$message.success(`${label}已复制`)
      } catch (error) {
        console.error('copyText failed:', error)
        this.$message.error('复制失败，请手动复制')
      }
    },
    filterTreeNode(value, data) { return !value || String(data.name || '').toLowerCase().includes(String(value).toLowerCase()) },
    showTreeContextMenu(event, data) {
      const treePanel = this.$refs.treePanel
      if (!treePanel) return
      const rect = treePanel.getBoundingClientRect()
      this.treeContextMenuNode = { ...data }
      this.treeContextMenuVisible = true
      this.treeContextMenuPosition = {
        x: event.clientX - rect.left,
        y: event.clientY - rect.top
      }
      this.$nextTick(() => {
        const menu = this.$refs.treeContextMenu
        if (!menu) return
        const padding = 12
        const maxX = Math.max(rect.width - menu.offsetWidth - padding, padding)
        const maxY = Math.max(rect.height - menu.offsetHeight - padding, padding)
        this.treeContextMenuPosition = {
          x: Math.min(Math.max(event.clientX - rect.left, padding), maxX),
          y: Math.min(Math.max(event.clientY - rect.top, padding), maxY)
        }
      })
    },
    hideTreeContextMenu() {
      this.treeContextMenuVisible = false
      this.treeContextMenuNode = null
    },
    handleTreeContextAction(action) {
      const node = this.treeContextMenuNode
      this.hideTreeContextMenu()
      if (!node) return
      if (action === 'create-room') return this.openRoomDialog('create')
      if (action === 'create-cabinet') return this.openCabinetDialog('create', node)
      if (action === 'edit-room') return this.openRoomDialog('edit', node)
      if (action === 'delete-room') return this.removeRoom(node)
      if (action === 'edit-cabinet') return this.openCabinetDialog('edit', node)
      if (action === 'delete-cabinet') return this.removeCabinet(node)
    },
    async loadTree(keepSelection = true) {
      this.loadingTree = true
      try {
        const [roomRes, cabinetRes] = await Promise.all([this.$api.getCmdbIdcRoomList(), this.$api.getCmdbIdcCabinetList()])
        if (roomRes.data.code !== 200) throw new Error(roomRes.data.message || '获取机房列表失败')
        if (cabinetRes.data.code !== 200) throw new Error(cabinetRes.data.message || '获取机柜列表失败')
        this.roomList = this.list(roomRes.data.data).map(item => this.normalizeRoom(item))
        this.cabinetList = this.list(cabinetRes.data.data).map(item => this.normalizeCabinet(item))
        this.buildTree()
        this.$nextTick(() => {
          this.$refs.idcTree?.filter(this.treeKeyword)
          if (keepSelection && this.selectedTreeNode) {
            const target = this.findNode(this.selectedTreeNode)
            if (target) {
              this.selectTreeNode(target, { resetPage: false })
              this.$refs.idcTree?.setCurrentKey(target.nodeKey)
              this.syncExpandedRoomWithSelection()
              return
            }
            this.selectedTreeNode = null
            this.queryParams.roomId = ''
            this.queryParams.cabinetId = ''
            this.setExpandedRoom(null)
            return
          }
          this.setExpandedRoom(null)
          if (!keepSelection) this.$refs.idcTree?.setCurrentKey(null)
        })
      } catch (error) {
        console.error('loadTree failed:', error)
        this.$message.error(error.message || '获取机房树失败')
      } finally {
        this.loadingTree = false
      }
    },
    async loadAssets() {
      this.loadingTable = true
      try {
        const params = { page: this.queryParams.page, pageSize: this.queryParams.pageSize, sortBy: 'updateTime', sortOrder: 'desc' }
        if (this.queryParams.keyword) params.keyword = this.queryParams.keyword
        if (this.queryParams.roomId) params.roomId = this.queryParams.roomId
        if (this.queryParams.cabinetId) params.cabinetId = this.queryParams.cabinetId
        const { data: res } = await this.$api.getCmdbIdcAssetList(params)
        if (res.code !== 200) throw new Error(res.message || '获取物理机资产失败')
        const page = this.page(res.data)
        this.assetList = page.list.map(item => this.normalizeAsset(item))
        this.total = page.total
      } catch (error) {
        console.error('loadAssets failed:', error)
        this.assetList = []
        this.total = 0
        this.$message.error(error.message || '获取物理机资产失败')
      } finally {
        this.loadingTable = false
      }
    },
    handleTreeClick(data) {
      this.hideTreeContextMenu()
      if (data.type === 'room') {
        this.toggleRoomExpand(data.id)
      } else {
        this.setExpandedRoom(data.roomId)
      }
      this.selectTreeNode(data)
      if (data.type === 'room') {
        this.switchToRoomView()
        return
      }
      if (this.activeView === 'vm' || this.activeView === 'hardware' || this.activeView === 'room') {
        this.switchToPhysicalView()
        return
      }
      if (this.activeView === 'physical') this.loadAssets()
    },
    selectAllAssets() {
      this.selectedTreeNode = null
      this.queryParams.page = 1
      this.queryParams.roomId = ''
      this.queryParams.cabinetId = ''
      this.$refs.idcTree?.setCurrentKey(null)
      this.setExpandedRoom(null)
      if (this.activeView === 'vm' || this.activeView === 'hardware' || this.activeView === 'room') {
        this.switchToPhysicalView()
        return
      }
      if (this.activeView === 'physical') this.loadAssets()
    },
    handleQuery() { this.queryParams.page = 1; this.loadAssets() },
    resetQuery() {
      this.queryParams.keyword = ''
      this.queryParams.page = 1
      this.queryParams.pageSize = 10
      this.selectAllAssets()
    },
    handleSizeChange(size) { this.queryParams.page = 1; this.queryParams.pageSize = size; this.loadAssets() },
    handleCurrentChange(page) { this.queryParams.page = page; this.loadAssets() },
    openRoomDialog(mode, room) {
      this.hideTreeContextMenu()
      this.roomDialogMode = mode
      this.roomForm = mode === 'edit' && room ? { id: room.id, name: room.name, remark: room.remark || '', cabinetText: '' } : makeRoomForm()
      this.roomDialogVisible = true
      this.$nextTick(() => this.$refs.roomFormRef?.clearValidate())
    },
    resetRoomDialog() {
      this.roomForm = makeRoomForm()
      this.$refs.roomFormRef?.clearValidate()
    },
    parseCabinetText(text) {
      return Array.from(new Set(String(text || '').split(/[\n,，]/).map(item => item.trim()).filter(Boolean)))
    },
    async submitRoom() {
      try {
        await this.$refs.roomFormRef.validate()
        this.submitting = true
        const payload = { name: this.roomForm.name.trim(), remark: (this.roomForm.remark || '').trim() }
        const response = this.roomDialogMode === 'create'
          ? await this.$api.createCmdbIdcRoom({ ...payload, cabinetNames: this.parseCabinetText(this.roomForm.cabinetText) })
          : await this.$api.updateCmdbIdcRoom({ id: this.roomForm.id, ...payload })
        if (response.data.code !== 200) throw new Error(response.data.message || '保存机房失败')
        this.$message.success(this.roomDialogMode === 'create' ? '机房创建成功' : '机房更新成功')
        this.roomDialogVisible = false
        await this.loadTree(true)
      } catch (error) {
        if (error !== false) {
          console.error('submitRoom failed:', error)
          this.$message.error(error.message || '保存机房失败')
        }
      } finally {
        this.submitting = false
      }
    },
    openCabinetDialog(mode, data) {
      this.hideTreeContextMenu()
      this.cabinetDialogMode = mode
      if (mode === 'edit' && data) {
        this.cabinetForm = { id: data.id, roomId: data.roomId, name: data.name, remark: data.remark || '' }
      } else {
        const roomId = data?.type === 'room' ? data.id : data?.roomId || this.selectedRoomId || ''
        this.cabinetForm = makeCabinetForm(roomId)
      }
      this.cabinetDialogVisible = true
      this.$nextTick(() => this.$refs.cabinetFormRef?.clearValidate())
    },
    resetCabinetDialog() {
      this.cabinetForm = makeCabinetForm(this.selectedRoomId || '')
      this.$refs.cabinetFormRef?.clearValidate()
    },
    async submitCabinet() {
      try {
        await this.$refs.cabinetFormRef.validate()
        this.submitting = true
        const payload = { roomId: this.num(this.cabinetForm.roomId), name: this.cabinetForm.name.trim(), remark: (this.cabinetForm.remark || '').trim() }
        const response = this.cabinetDialogMode === 'create'
          ? await this.$api.createCmdbIdcCabinet(payload)
          : await this.$api.updateCmdbIdcCabinet({ id: this.cabinetForm.id, ...payload })
        if (response.data.code !== 200) throw new Error(response.data.message || '保存机柜失败')
        this.$message.success(this.cabinetDialogMode === 'create' ? '机柜创建成功' : '机柜更新成功')
        this.cabinetDialogVisible = false
        await this.loadTree(true)
      } catch (error) {
        if (error !== false) {
          console.error('submitCabinet failed:', error)
          this.$message.error(error.message || '保存机柜失败')
        }
      } finally {
        this.submitting = false
      }
    },
    async removeRoom(room) {
      try {
        await this.$confirm(`确认删除机房“${room.name}”吗？机房下有资产时后端会拒绝删除。`, '删除机房', { type: 'warning' })
        const { data: res } = await this.$api.deleteCmdbIdcRoom(room.id)
        if (res.code !== 200) throw new Error(res.message || '删除机房失败')
        this.$message.success('机房删除成功')
        if (this.selectedTreeNode?.type === 'room' && this.selectedTreeNode.id === room.id) {
          this.selectedTreeNode = null
          this.queryParams.roomId = ''
          this.queryParams.cabinetId = ''
          this.queryParams.page = 1
        }
        await Promise.all([this.loadTree(false), this.loadAssets()])
      } catch (error) {
        if (error !== 'cancel' && error !== 'close') {
          console.error('removeRoom failed:', error)
          this.$message.error(error.message || '删除机房失败')
        }
      }
    },
    async removeCabinet(cabinet) {
      try {
        await this.$confirm(`确认删除机柜“${cabinet.name}”吗？机柜下有资产时后端会拒绝删除。`, '删除机柜', { type: 'warning' })
        const { data: res } = await this.$api.deleteCmdbIdcCabinet(cabinet.id)
        if (res.code !== 200) throw new Error(res.message || '删除机柜失败')
        this.$message.success('机柜删除成功')
        if (this.selectedTreeNode?.type === 'cabinet' && this.selectedTreeNode.id === cabinet.id) {
          this.selectedTreeNode = null
          this.queryParams.roomId = ''
          this.queryParams.cabinetId = ''
          this.queryParams.page = 1
        }
        await Promise.all([this.loadTree(false), this.loadAssets()])
      } catch (error) {
        if (error !== 'cancel' && error !== 'close') {
          console.error('removeCabinet failed:', error)
          this.$message.error(error.message || '删除机柜失败')
        }
      }
    },
    openAssetDialog(mode, row) {
      if (mode === 'create' && !this.roomList.length) {
        this.$message.warning('请先创建机房，再新增物理机资产')
        return
      }
      this.assetDialogMode = mode
      this.assetDialogVisible = true
      if (mode === 'create') {
        const roomId = this.selectedTreeNode?.type === 'room' ? this.selectedTreeNode.id : this.selectedTreeNode?.roomId || ''
        const cabinetId = this.selectedTreeNode?.type === 'cabinet' ? this.selectedTreeNode.id : ''
        this.assetForm = makeAssetForm(roomId, cabinetId)
        this.$nextTick(() => this.$refs.assetFormRef?.clearValidate())
        return
      }
      this.loadAssetDetail(row.id)
    },
    resetAssetDialog() {
      this.assetForm = makeAssetForm(this.selectedRoomId || '')
      this.$refs.assetFormRef?.clearValidate()
    },
    async loadAssetDetail(id) {
      try {
        this.submitting = true
        const { data: res } = await this.$api.getCmdbIdcAssetInfo(id)
        if (res.code !== 200) throw new Error(res.message || '获取资产详情失败')
        const detail = this.normalizeAsset(res.data || {})
        const roomId = detail.roomId || this.getRoomIdByName(detail.roomName)
        const cabinetId = detail.cabinetId || this.getCabinetIdByName(detail.cabinetName, roomId)
        this.assetForm = { id: detail.id, roomId: roomId || '', cabinetId: cabinetId || '', ip: detail.ip || '', brand: detail.brand || '', model: detail.model || '', cpu: detail.cpu || '', disk: detail.disk || '', memory: detail.memory || '', purpose: detail.purpose || '', remoteManagementCard: detail.remoteManagementCard || '', remark: detail.remark || '', purchaseDate: detail.purchaseDate || '', warrantyStartDate: detail.warrantyStartDate || '', warrantyEndDate: detail.warrantyEndDate || '', usageYears: detail.usageYears === '' ? undefined : detail.usageYears, assetEvaluation: detail.assetEvaluation || '' }
        this.$nextTick(() => this.$refs.assetFormRef?.clearValidate())
      } catch (error) {
        console.error('loadAssetDetail failed:', error)
        this.$message.error(error.message || '获取资产详情失败')
        this.assetDialogVisible = false
      } finally {
        this.submitting = false
      }
    },
    handleAssetRoomChange(roomId) {
      const cabinet = this.getCabinetById(this.num(this.assetForm.cabinetId))
      if (cabinet && cabinet.roomId !== this.num(roomId)) this.assetForm.cabinetId = ''
    },
    compact(payload) {
      return Object.keys(payload).reduce((acc, key) => {
        const value = payload[key]
        if (value === '' || value === undefined || value === null) return acc
        acc[key] = typeof value === 'string' ? value.trim() : value
        return acc
      }, {})
    },
    async submitAsset() {
      try {
        await this.$refs.assetFormRef.validate()
        this.submitting = true
        const currentPage = this.queryParams.page
        const payload = this.compact({ roomId: this.num(this.assetForm.roomId), cabinetId: this.num(this.assetForm.cabinetId), ip: this.assetForm.ip, brand: this.assetForm.brand, model: this.assetForm.model, cpu: this.assetForm.cpu, disk: this.assetForm.disk, memory: this.assetForm.memory, purpose: this.assetForm.purpose, remoteManagementCard: this.assetForm.remoteManagementCard, remark: this.assetForm.remark, purchaseDate: this.assetForm.purchaseDate, warrantyStartDate: this.assetForm.warrantyStartDate, warrantyEndDate: this.assetForm.warrantyEndDate, usageYears: this.assetForm.usageYears, assetEvaluation: this.assetForm.assetEvaluation })
        const response = this.assetDialogMode === 'create' ? await this.$api.createCmdbIdcAsset(payload) : await this.$api.updateCmdbIdcAsset({ id: this.assetForm.id, ...payload })
        if (response.data.code !== 200) throw new Error(response.data.message || '保存物理机资产失败')
        this.$message.success(this.assetDialogMode === 'create' ? '物理机资产创建成功' : '物理机资产更新成功')
        this.assetDialogVisible = false
        if (this.assetDialogMode === 'edit') this.queryParams.page = currentPage
        await Promise.all([this.loadTree(true), this.loadAssets()])
      } catch (error) {
        if (error !== false) {
          console.error('submitAsset failed:', error)
          this.$message.error(error.message || '保存物理机资产失败')
        }
      } finally {
        this.submitting = false
      }
    },
    async removeAsset(row) {
      try {
        await this.$confirm(`确认删除物理机资产“${row.ip || row.model || row.id}”吗？`, '删除资产', { type: 'warning' })
        const { data: res } = await this.$api.deleteCmdbIdcAsset(row.id)
        if (res.code !== 200) throw new Error(res.message || '删除物理机资产失败')
        this.$message.success('物理机资产删除成功')
        if (this.assetList.length === 1 && this.queryParams.page > 1) this.queryParams.page -= 1
        await Promise.all([this.loadTree(true), this.loadAssets()])
      } catch (error) {
        if (error !== 'cancel' && error !== 'close') {
          console.error('removeAsset failed:', error)
          this.$message.error(error.message || '删除物理机资产失败')
        }
      }
    },
    openImportDialog() {
      this.importDialogVisible = true
      this.importProgress = 0
    },
    resetImportDialog() {
      this.importForm.file = null
      this.importProgress = 0
      this.importUploading = false
    },
    handleImportFileChange(file) {
      const raw = file.raw || file
      if (!/\.(xlsx|xls)$/i.test(raw.name || '')) {
        this.$message.warning('请上传 Excel 文件')
        return
      }
      this.importForm.file = raw
    },
    handleImportFileRemove() {
      this.importForm.file = null
      this.importProgress = 0
    },
    async submitImport() {
      if (!this.importForm.file) return this.$message.warning('请先选择 Excel 文件')
      this.importUploading = true
      this.importProgress = 0
      try {
        const formData = new FormData()
        formData.append('file', this.importForm.file)
        const { data: res } = await this.$api.importCmdbIdcAssetsFromExcel(formData, { onUploadProgress: event => { if (event.total) this.importProgress = Math.round((event.loaded * 100) / event.total) } })
        if (res.code !== 200) throw new Error(res.message || '导入物理机资产失败')
        this.importProgress = 100
        this.$message.success('物理机资产导入成功')
        this.importDialogVisible = false
        await Promise.all([this.loadTree(true), this.loadAssets()])
      } catch (error) {
        console.error('submitImport failed:', error)
        this.$message.error(error.message || '导入物理机资产失败')
      } finally {
        this.importUploading = false
      }
    },
    async downloadTemplate() {
      this.templateLoading = true
      try {
        const response = await this.$api.downloadCmdbIdcAssetTemplate()
        const url = window.URL.createObjectURL(new Blob([response.data]))
        const link = document.createElement('a')
        link.href = url
        link.download = '机房物理机导入模板.xlsx'
        document.body.appendChild(link)
        link.click()
        document.body.removeChild(link)
        window.URL.revokeObjectURL(url)
        this.$message.success('模板下载成功')
      } catch (error) {
        console.error('downloadTemplate failed:', error)
        this.$message.error(error.message || '模板下载失败')
      } finally {
        this.templateLoading = false
      }
    }
  }
}
</script>


<style scoped>
/* Design-system layer: compact AutoOps workbench */
.cmdb-idc-page {
  /* Primitive */
  --idc-blue-500: #409eff;
  --idc-blue-600: #2563eb;
  --idc-cyan-500: #0ea5e9;
  --idc-green-500: #10b981;
  --idc-orange-500: #f59e0b;
  --idc-red-500: #ef4444;

  /* Semantic */
  --idc-bg-page: #f6f8fb;
  --idc-bg-surface: #fff;
  --idc-bg-surface-soft: #fbfdff;
  --idc-bg-subtle: #f3f7fc;
  --idc-text-primary: #172033;
  --idc-text-heading: #0f172a;
  --idc-text-secondary: #53657d;
  --idc-text-muted: #64748b;
  --idc-border-cool: #dbe7f4;
  --idc-border-soft: #e7eef7;
  --idc-border-inner: #dce8f5;
  --idc-border-hover: #b9d4ff;
  --idc-brand: var(--idc-blue-500);
  --idc-brand-active: var(--idc-blue-600);
  --idc-brand-soft: #eef6ff;
  --idc-brand-fade: rgba(37, 99, 235, 0.06);
  --idc-success: var(--idc-green-500);
  --idc-success-soft: #eaf8ef;
  --idc-warning: var(--idc-orange-500);
  --idc-warning-soft: #fff4e5;
  --idc-danger: var(--idc-red-500);
  --idc-danger-soft: #fff0f0;

  /* Component */
  --idc-radius-control: 8px;
  --idc-radius-inner: 10px;
  --idc-radius-card: 12px;
  --idc-radius-dialog: 15px;
  --idc-button-query: #409eff;
  --idc-button-query-hover: #337ecc;
  --idc-button-success: #67c23a;
  --idc-button-success-hover: #529b2e;
  --idc-button-refresh: #f3c66b;
  --idc-button-refresh-hover: #dcae50;
  --idc-button-warning: #e6a23c;
  --idc-button-warning-hover: #b88230;
  --idc-shadow-card: 0 8px 24px rgba(80, 124, 180, 0.07);
  --idc-shadow-hover: 0 10px 24px rgba(64, 158, 255, 0.1);
  --idc-shadow-hero: 0 10px 28px rgba(80, 124, 180, 0.1);

  box-sizing: border-box;
  display: grid;
  align-content: start;
  gap: 8px;
  height: calc(100vh - 124px);
  min-height: 0;
  overflow: auto;
  padding: 7px;
  color: var(--idc-text-primary);
  background: transparent;
}

:global(.home-container .el-main:has(.cmdb-idc-page)) {
  padding: 10px;
}

.idc-hero-panel {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-width: 0;
  padding: 8px 12px;
  border: 1px solid var(--idc-border-cool);
  border-radius: var(--idc-radius-card);
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.98), rgba(248, 251, 255, 0.95)),
    var(--idc-bg-surface);
  box-shadow: var(--idc-shadow-hero);
}

.idc-hero-copy {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  gap: 8px;
  min-width: 0;
}

.idc-hero-icon {
  display: grid;
  flex: 0 0 42px;
  width: 42px;
  height: 42px;
  place-items: center;
  border: 0;
  border-radius: 0;
  color: var(--idc-brand-active);
  background: transparent;
}

.idc-hero-icon .el-icon {
  font-size: 30px;
}

.idc-hero-content {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  gap: 10px;
  min-width: 0;
}

.idc-hero-copy h1 {
  flex: 0 0 auto;
  margin: 0;
  color: var(--idc-text-primary);
  font-size: 20px;
  line-height: 1.2;
  letter-spacing: 0;
  white-space: nowrap;
}

.idc-hero-divider {
  flex: 0 0 1px;
  width: 1px;
  height: 22px;
  background: var(--idc-border-soft);
}

.idc-hero-content p {
  min-width: 0;
  margin: 0;
  overflow: hidden;
  color: var(--idc-text-muted);
  font-size: 12px;
  font-weight: 400;
  line-height: 1.5;
  letter-spacing: 0;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.idc-hero-metrics {
  display: flex;
  align-items: stretch;
  flex: 0 0 auto;
}

.idc-hero-metrics > div {
  display: grid;
  grid-template-columns: auto auto;
  align-items: baseline;
  gap: 6px;
  min-width: 84px;
  padding: 3px 12px;
  border-left: 1px solid var(--idc-border-soft);
}

.idc-hero-metrics span {
  color: var(--idc-text-muted);
  font-size: 11px;
  font-weight: 600;
}

.idc-hero-metrics strong {
  color: var(--idc-text-heading);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 18px;
  line-height: 1;
}

.page-card,
:deep(.page-card) {
  min-width: 0;
  border: 0 !important;
  background: transparent !important;
  box-shadow: none !important;
}

:deep(.page-card > .el-card__body) {
  padding: 0 !important;
}

.page-layout {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
}

.tree-panel {
  position: relative;
  display: flex;
  flex: 0 0 232px;
  flex-direction: column;
  width: 232px;
  height: calc(100vh - 312px);
  min-height: 310px;
  padding: 10px;
  overflow: hidden;
  border: 1px solid var(--idc-border-cool);
  border-radius: var(--idc-radius-card);
  background: rgba(255, 255, 255, 0.97);
  box-shadow: var(--idc-shadow-card);
}

.tree-panel::after {
  display: none;
}

.panel-head,
.table-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.panel-head {
  padding: 0 2px 8px;
  border-bottom: 1px solid var(--idc-border-soft);
}

.panel-head h3,
.table-head h3 {
  margin: 0;
  color: var(--idc-text-heading);
  font-size: 15px;
  font-weight: 700;
  line-height: 1.25;
}

.panel-head p,
.table-head p {
  margin: 3px 0 0;
  color: var(--idc-text-muted);
  font-size: 11px;
  line-height: 1.35;
  letter-spacing: 0;
  text-transform: none;
}

.head-actions,
.toolbar-actions {
  display: flex;
  align-items: center;
  flex-wrap: nowrap;
  gap: 6px;
}

.tree-search {
  margin: 8px 0 4px;
}

.tree-scroll {
  flex: 1;
  min-height: 0;
  margin-top: 4px;
}

:deep(.tree-scroll .el-scrollbar__wrap) {
  overflow-x: hidden;
}

:deep(.tree-scroll .el-scrollbar__bar.is-horizontal) {
  display: none !important;
}

:deep(.tree-scroll .el-scrollbar__view) {
  width: 100%;
}

:deep(.idc-tree) {
  background: transparent;
  color: var(--idc-text-secondary);
}

:deep(.idc-tree .el-tree-node__content) {
  height: auto;
  min-height: 42px;
  margin: 1px 0;
  padding: 4px 6px;
  border-radius: var(--idc-radius-control);
  transition: background-color 160ms ease, box-shadow 160ms ease;
}

:deep(.idc-tree .el-tree-node__content:hover) {
  background: #f5f9ff;
}

:deep(.idc-tree .el-tree-node.is-current > .el-tree-node__content) {
  background: var(--idc-brand-soft);
  box-shadow: inset 3px 0 0 var(--idc-brand-active);
}

.tree-node,
.tree-node-main {
  display: flex;
  align-items: center;
  min-width: 0;
}

.tree-node {
  justify-content: space-between;
  width: 100%;
  gap: 6px;
  cursor: pointer;
}

.tree-node-main {
  flex: 1;
  gap: 8px;
}

.tree-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 30px;
  width: 30px;
  height: 30px;
  border: 1px solid #d5e4ff;
  border-radius: var(--idc-radius-control);
  background: #eef6ff;
}

.tree-icon.cabinet {
  border-color: #d7eee7;
  background: #eefaf6;
}

.tree-icon-image {
  width: 16px;
  height: 16px;
  object-fit: contain;
}

.tree-node-text {
  display: flex;
  min-width: 0;
  flex-direction: column;
}

.tree-node-name {
  display: inline-flex;
  align-items: center;
  min-width: 0;
  max-width: 100%;
  gap: 6px;
  color: var(--idc-text-primary);
  font-size: 12px;
  font-weight: 700;
  line-height: 1.35;
}

.tree-node-name > span:first-child {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tree-node-count,
.tree-node-meta {
  color: var(--idc-text-muted);
  font-size: 10.5px;
  font-weight: 500;
  line-height: 1.35;
}

.tree-node-count {
  flex-shrink: 0;
}

.tree-node-text.cabinet-text .tree-node-meta {
  display: none;
}

.tree-context-menu {
  position: absolute;
  z-index: 30;
  min-width: 156px;
  padding: 6px;
  border: 1px solid var(--idc-border-cool);
  border-radius: var(--idc-radius-inner);
  background: var(--idc-bg-surface);
  box-shadow: 0 16px 38px rgba(49, 92, 142, 0.16);
  backdrop-filter: none;
}

.tree-context-menu-item {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 32px;
  padding: 6px 9px;
  border-radius: var(--idc-radius-control);
  color: var(--idc-text-secondary);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: color 160ms ease, background-color 160ms ease;
}

.tree-context-menu-item + .tree-context-menu-item {
  margin-top: 2px;
}

.tree-context-menu-item:hover {
  color: var(--idc-brand-active);
  background: var(--idc-brand-soft);
}

.tree-context-menu-item.danger {
  color: #dc2626;
}

.tree-context-menu-item.danger:hover {
  color: #dc2626;
  background: var(--idc-danger-soft);
}

.content-panel {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}

.toolbar,
.table-card {
  min-width: 0;
  border: 1px solid var(--idc-border-cool);
  border-radius: var(--idc-radius-card);
  background: rgba(255, 255, 255, 0.97);
  box-shadow: var(--idc-shadow-card);
}

.toolbar {
  flex: 0 0 auto;
  margin-bottom: 8px;
  padding: 8px 10px 0;
}

.toolbar-main {
  display: flex;
  align-items: center;
  flex-wrap: nowrap;
  gap: 8px;
  min-width: 0;
  overflow-x: auto;
  padding-bottom: 8px;
}

.toolbar-tags {
  display: flex;
  align-items: center;
  flex: 1 1 auto;
  gap: 6px;
  min-width: 0;
  margin: 0;
  color: var(--idc-text-muted);
  font-size: 11px;
  white-space: nowrap;
}

.toolbar-actions {
  flex: 0 0 auto;
}

.toolbar-divider {
  height: 1px;
  margin: 0 -10px;
  background: var(--idc-border-soft);
}

.view-tabs {
  display: flex;
  align-items: center;
  gap: 4px;
  overflow-x: auto;
  padding: 0 2px;
}

.view-tab {
  position: relative;
  flex: 0 0 auto;
  min-height: 34px;
  padding: 0 10px;
  border: 0;
  border-bottom: 2px solid transparent;
  border-radius: 0;
  color: var(--idc-text-secondary);
  background: transparent;
  font-size: 13px;
  font-weight: 600;
  letter-spacing: 0;
  cursor: pointer;
  transition: color 160ms ease, border-color 160ms ease;
}

.view-tab:hover {
  color: var(--idc-brand-active);
}

.view-tab.active {
  color: var(--idc-brand-active);
  border-bottom-color: var(--idc-brand-active);
  font-weight: 700;
}

.view-tab:focus-visible {
  outline: 2px solid rgba(64, 158, 255, 0.35);
  outline-offset: -2px;
}

.table-card {
  display: flex;
  flex: 0 0 auto;
  flex-direction: column;
  min-height: 0;
  padding: 0;
  overflow: hidden;
}

.table-head {
  align-items: center;
  padding: 8px 12px;
  border-bottom: 1px solid var(--idc-border-soft);
}

.asset-table {
  width: calc(100% - 20px);
  margin: 8px 10px 0;
}

.pagination {
  display: flex;
  flex: 0 0 auto;
  justify-content: flex-end;
  margin-top: 0;
  padding: 8px 10px 10px;
}

.asset-ip-link {
  padding: 0;
  font-weight: 700;
}

.copy-hover-cell {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
}

.asset-type-icon,
.asset-monitor-icon {
  flex: 0 0 16px;
  width: 16px;
  height: 16px;
  object-fit: contain;
}

.copy-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  color: var(--idc-brand-active);
  cursor: pointer;
  opacity: 0;
  transition: color 160ms ease, opacity 160ms ease;
}

.copy-hover-cell:hover .copy-trigger,
.copy-trigger:focus-visible {
  opacity: 1;
}

.copy-trigger:hover {
  color: var(--idc-blue-500);
}

.form-tip,
.import-tip {
  color: var(--idc-text-muted);
  font-size: 12px;
  line-height: 1.55;
}

.import-box {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.file-line {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--idc-text-secondary);
  font-size: 12px;
}

.cmdb-idc-page :deep(.el-form-item) {
  margin-right: 0;
}

.cmdb-idc-page :deep(.toolbar .el-form-item) {
  margin-bottom: 0;
}

.cmdb-idc-page :deep(.toolbar .el-form-item__label) {
  padding-right: 7px;
  color: var(--idc-text-secondary);
  font-size: 12px;
  font-weight: 700;
}

.cmdb-idc-page :deep(.el-input__wrapper),
.cmdb-idc-page :deep(.el-select__wrapper),
.cmdb-idc-page :deep(.el-textarea__inner),
.cmdb-idc-page :deep(.el-upload-dragger) {
  border-radius: var(--idc-radius-control);
  background: var(--idc-bg-surface-soft);
  box-shadow: 0 0 0 1px #d9e5f2 inset !important;
  transition: background-color 160ms ease, box-shadow 160ms ease;
}

.cmdb-idc-page :deep(.el-input__wrapper:hover),
.cmdb-idc-page :deep(.el-select__wrapper:hover),
.cmdb-idc-page :deep(.el-textarea__inner:hover),
.cmdb-idc-page :deep(.el-upload-dragger:hover) {
  box-shadow: 0 0 0 1px var(--idc-border-hover) inset !important;
}

.cmdb-idc-page :deep(.el-input__wrapper.is-focus),
.cmdb-idc-page :deep(.el-select__wrapper.is-focused),
.cmdb-idc-page :deep(.el-textarea__inner:focus),
.cmdb-idc-page :deep(.el-upload-dragger:focus) {
  box-shadow: 0 0 0 1px var(--idc-blue-500) inset !important;
}

.cmdb-idc-page :deep(.tree-search .el-input__wrapper) {
  min-height: 34px;
  background: var(--idc-bg-surface-soft);
}

.toolbar-keyword-input :deep(.el-input__wrapper) {
  min-height: 30px;
}

.cmdb-idc-page :deep(.el-button) {
  border-radius: var(--idc-radius-control);
  font-weight: 600;
  transition:
    color 160ms ease,
    background-color 160ms ease,
    border-color 160ms ease,
    box-shadow 160ms ease,
    transform 160ms ease;
}

.cmdb-idc-page :deep(.el-button:not(.is-link):not(.is-text):hover) {
  box-shadow: 0 5px 12px rgba(64, 158, 255, 0.12) !important;
  transform: translateY(-1px) !important;
}

.cmdb-idc-page :deep(.el-button:active) {
  transform: translateY(0) !important;
}

.cmdb-idc-page :deep(.el-button--primary:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: var(--idc-button-query);
  color: #fff !important;
  background: var(--idc-button-query);
}

.cmdb-idc-page :deep(.el-button--success:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: var(--idc-button-success);
  color: #fff !important;
  background: var(--idc-button-success);
}

.cmdb-idc-page :deep(.el-button--warning:not(.is-link):not(.is-text):not(.is-plain)) {
  border-color: var(--idc-button-warning);
  color: #fff !important;
  background: var(--idc-button-warning);
}

.cmdb-idc-page :deep(.el-button.idc-refresh-button) {
  border-color: var(--idc-button-refresh);
  color: #fff !important;
  background: var(--idc-button-refresh);
}

.cmdb-idc-page :deep(.el-button--primary:not(.is-link):not(.is-text):not(.is-plain):hover),
.cmdb-idc-page :deep(.el-button--primary:not(.is-link):not(.is-text):not(.is-plain):focus-visible) {
  border-color: var(--idc-button-query-hover);
  background: var(--idc-button-query-hover);
}

.cmdb-idc-page :deep(.el-button--success:not(.is-link):not(.is-text):not(.is-plain):hover),
.cmdb-idc-page :deep(.el-button--success:not(.is-link):not(.is-text):not(.is-plain):focus-visible) {
  border-color: var(--idc-button-success-hover);
  background: var(--idc-button-success-hover);
}

.cmdb-idc-page :deep(.el-button--warning:not(.is-link):not(.is-text):not(.is-plain):hover),
.cmdb-idc-page :deep(.el-button--warning:not(.is-link):not(.is-text):not(.is-plain):focus-visible) {
  border-color: var(--idc-button-warning-hover);
  background: var(--idc-button-warning-hover);
}

.cmdb-idc-page :deep(.el-button.idc-refresh-button:hover),
.cmdb-idc-page :deep(.el-button.idc-refresh-button:focus-visible) {
  border-color: var(--idc-button-refresh-hover);
  background: var(--idc-button-refresh-hover);
}

.cmdb-idc-page :deep(.el-button--primary:not(.is-link):not(.is-text):not(.is-plain) .el-icon),
.cmdb-idc-page :deep(.el-button--success:not(.is-link):not(.is-text):not(.is-plain) .el-icon),
.cmdb-idc-page :deep(.el-button--warning:not(.is-link):not(.is-text):not(.is-plain) .el-icon) {
  color: #fff !important;
}

.cmdb-idc-page :deep(.el-button--primary.is-plain) {
  color: var(--idc-brand-active);
  border-color: #bfd6f6;
  background: #f7fbff;
}

.cmdb-idc-page :deep(.el-button--primary.is-link),
.cmdb-idc-page :deep(.el-button--primary.is-text) {
  color: var(--idc-brand-active);
  border-color: transparent;
  background: transparent;
}

.cmdb-idc-page :deep(.el-button.is-circle) {
  width: 32px;
  height: 32px;
  min-height: 32px;
  padding: 0;
  border-radius: var(--idc-radius-control);
}

.cmdb-idc-page :deep(.toolbar .el-button) {
  min-height: 30px;
  padding: 6px 11px;
}

.cmdb-idc-page :deep(.toolbar-tags .el-tag) {
  height: 22px;
  border-color: #dfe8f3;
  border-radius: 999px;
  color: var(--idc-text-secondary);
  background: var(--idc-bg-subtle);
  font-size: 11px;
}

.cmdb-idc-page :deep(.toolbar-tags .el-tag--primary) {
  border-color: #cfe0ff;
  color: var(--idc-brand-active);
  background: var(--idc-brand-soft);
}

.cmdb-idc-page :deep(.asset-table.el-table) {
  --el-table-border-color: #e3ebf5;
  --el-table-header-bg-color: #f3f7fc;
  --el-table-tr-bg-color: #fff;
  --el-table-row-hover-bg-color: #f8fafc;
  --el-table-current-row-bg-color: #eef6ff;
  --el-table-bg-color: #fff;
  --el-table-text-color: var(--idc-text-secondary);
  --el-table-header-text-color: #475569;
  overflow: hidden;
  border: 1px solid #e3ebf5;
  border-radius: var(--idc-radius-control);
  background: #fff;
  font-size: 12px;
}

.cmdb-idc-page :deep(.asset-table .el-table__inner-wrapper::before) {
  display: none;
}

.cmdb-idc-page :deep(.asset-table th.el-table__cell) {
  height: 34px;
  padding: 5px 0;
  color: #475569;
  font-size: 12px;
  font-weight: 700;
}

.cmdb-idc-page :deep(.asset-table td.el-table__cell) {
  padding: 5px 0;
  font-size: 12px;
}

.cmdb-idc-page :deep(.asset-table td.el-table__cell),
.cmdb-idc-page :deep(.asset-table th.el-table__cell.is-leaf) {
  border-bottom-color: #eef3f9;
}

.cmdb-idc-page :deep(.asset-table .el-table__fixed-right),
.cmdb-idc-page :deep(.asset-table .el-table__fixed-right-patch),
.cmdb-idc-page :deep(.asset-table .el-table__fixed-right .el-table__cell) {
  background: #fff;
}

.cmdb-idc-page :deep(.asset-table .el-table__fixed-right .el-table__row:hover > .el-table__cell) {
  background: #f8fafc !important;
}

.cmdb-idc-page :deep(.pagination .btn-prev),
.cmdb-idc-page :deep(.pagination .btn-next),
.cmdb-idc-page :deep(.pagination .el-pager li) {
  border-radius: 7px;
  color: var(--idc-text-secondary);
  background: #f8fafc;
}

.cmdb-idc-page :deep(.pagination .el-pager li.is-active) {
  color: #fff;
  background: var(--idc-brand-active);
}

/* Dialog is teleported to body, so it must be targeted globally. */
:global(.idc-form-dialog-overlay .el-dialog) {
  --el-dialog-border-radius: 15px;
  display: flex;
  flex-direction: column;
  max-width: calc(100vw - 32px);
  max-height: calc(100vh - 32px);
  overflow: hidden;
  border: 1px solid #d7e6f6;
  border-radius: var(--el-dialog-border-radius) !important;
  background: #fff;
  box-shadow: 0 22px 60px rgba(49, 92, 142, 0.18);
}

:global(.idc-form-dialog-overlay .el-dialog__header) {
  position: relative;
  margin: 0;
  padding: 16px 18px 12px;
  border-bottom: 1px solid #e3edf8;
  background: linear-gradient(90deg, rgba(255, 246, 231, 0.82) 0%, rgba(247, 251, 255, 0.96) 40%, rgba(236, 247, 255, 0.9) 100%);
}

:global(.idc-form-dialog-overlay .el-dialog__header::before) {
  position: absolute;
  inset: 0 0 auto;
  height: 3px;
  background: linear-gradient(90deg, #f5a524 0%, #62a8ff 48%, #55d6be 100%);
  content: '';
}

:global(.idc-form-dialog-overlay .el-dialog__title) {
  color: #172033;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.3;
}

:global(.idc-form-dialog-overlay .el-dialog__headerbtn) {
  top: 8px;
  right: 10px;
}

:global(.idc-form-dialog-overlay .el-dialog__body) {
  min-height: 0;
  padding: 16px 18px;
  overflow: auto;
  color: #53657d;
}

:global(.idc-form-dialog-overlay .el-dialog__footer) {
  padding: 12px 18px 16px;
  border-top: 1px solid #e3edf8;
  background: #fbfdff;
}

:global(.idc-form-dialog .idc-dialog-footer) {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

:global(.idc-form-dialog .idc-dialog-form .el-form-item) {
  margin-bottom: 13px;
}

:global(.idc-form-dialog .idc-dialog-form .el-form-item__label) {
  margin-bottom: 6px;
  padding: 0;
  color: #334155;
  font-size: 12px;
  font-weight: 700;
  line-height: 1.4;
}

:global(.idc-form-dialog .el-input__wrapper),
:global(.idc-form-dialog .el-select__wrapper),
:global(.idc-form-dialog .el-textarea__inner),
:global(.idc-form-dialog .el-input-number),
:global(.idc-form-dialog .el-date-editor),
:global(.idc-form-dialog .el-upload-dragger) {
  border-radius: 8px;
  background: #fbfdff;
  box-shadow: 0 0 0 1px #d9e5f2 inset !important;
  transition: box-shadow 160ms ease, background-color 160ms ease;
}

:global(.idc-form-dialog .el-input__wrapper),
:global(.idc-form-dialog .el-select__wrapper),
:global(.idc-form-dialog .el-input-number),
:global(.idc-form-dialog .el-date-editor) {
  min-height: 34px;
}

:global(.idc-form-dialog .el-input__wrapper:hover),
:global(.idc-form-dialog .el-select__wrapper:hover),
:global(.idc-form-dialog .el-textarea__inner:hover),
:global(.idc-form-dialog .el-upload-dragger:hover) {
  box-shadow: 0 0 0 1px #b9d4ff inset !important;
}

:global(.idc-form-dialog .el-input__wrapper.is-focus),
:global(.idc-form-dialog .el-select__wrapper.is-focused),
:global(.idc-form-dialog .el-textarea__inner:focus) {
  box-shadow: 0 0 0 1px #409eff inset !important;
}

:global(.idc-form-dialog .el-input-number),
:global(.idc-form-dialog .el-date-editor) {
  width: 100% !important;
}

:global(.idc-form-dialog .el-button) {
  min-height: 32px;
  border-radius: 8px;
  font-weight: 600;
}

:global(.idc-form-dialog .el-button--primary:not(.is-plain)) {
  border-color: transparent;
  background: linear-gradient(135deg, #5aa8ff, #2563eb);
}

:global(.idc-form-dialog .el-upload-dragger) {
  padding: 22px 16px;
}

:global(.idc-form-dialog .el-upload) {
  width: 100%;
}

:global(.idc-form-dialog .form-tip),
:global(.idc-form-dialog .import-tip),
:global(.idc-form-dialog .el-upload__text),
:global(.idc-form-dialog .el-upload__tip),
:global(.idc-form-dialog .el-progress__text) {
  color: #64748b;
  font-size: 12px;
}

@media (max-width: 1200px) {
  .cmdb-idc-page {
    height: auto;
    min-height: calc(100vh - 124px);
  }

  .page-layout {
    flex-direction: column;
  }

  .tree-panel {
    width: 100%;
    height: auto;
    min-height: 260px;
    max-height: 360px;
    flex-basis: auto;
  }

  .toolbar-main {
    flex-wrap: wrap;
  }

  .toolbar-tags {
    order: 3;
    flex-basis: 100%;
  }
}

@media (max-width: 760px) {
  .idc-hero-panel {
    align-items: flex-start;
    flex-direction: column;
  }

  .idc-hero-content {
    align-items: flex-start;
    flex-direction: column;
    gap: 3px;
  }

  .idc-hero-divider {
    display: none;
  }

  .idc-hero-content p {
    width: 100%;
    white-space: normal;
  }

  .idc-hero-metrics {
    width: 100%;
  }

  .idc-hero-metrics > div {
    flex: 1;
    min-width: 0;
    padding: 3px 8px;
  }

  .toolbar-main,
  .toolbar-actions {
    align-items: stretch;
    flex-direction: column;
    width: 100%;
  }

  .toolbar-form,
  .toolbar-keyword-input,
  .toolbar-keyword-input :deep(.el-input) {
    width: 100% !important;
  }

  .toolbar-tags {
    flex-wrap: wrap;
    white-space: normal;
  }

  .pagination {
    justify-content: flex-start;
    overflow-x: auto;
  }

  :global(.idc-asset-dialog .el-col-8),
  :global(.idc-asset-dialog .el-col-12) {
    flex: 0 0 100%;
    max-width: 100%;
  }
}

@media (max-width: 620px) {
  .cmdb-idc-page {
    padding: 6px;
  }

  .idc-hero-copy h1 {
    font-size: 18px;
  }

  .idc-hero-metrics > div {
    grid-template-columns: 1fr;
    gap: 2px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .cmdb-idc-page *,
  .cmdb-idc-page *::before,
  .cmdb-idc-page *::after {
    scroll-behavior: auto !important;
    transition-duration: 0.01ms !important;
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
  }
}
</style>
