"use client";
import React, { useState, useMemo } from "react";
import { PhysicalHost, VmHost, SwitchDevice, DatabaseAsset, MiddlewareAsset, BackupAsset, OpsAsset, ProjectGroup, AssetMeta, SoftwareComponent, OpsChannel } from "../cmdbData";
import { exportAssetsToExcel, exportDatabasesToExcel, exportMiddlewaresToExcel, exportBackupsToExcel, exportOpsToExcel, exportAllV351Workbook, getAssetKey } from "./excelExport";
import ImportModal, { ImportStrategy } from "./ImportModal";
import {
  EXCEL_CUSTOMERS,
  EXCEL_PROJECTS,
  EXCEL_ENVIRONMENTS,
  EXCEL_CLOUD_VENDORS,
  EXCEL_REGION_NAMES,
  EXCEL_DEVICE_CATEGORIES,
  EXCEL_DEVICE_TYPES,
  EXCEL_IDC_ROOMS,
  EXCEL_OS_FAMILIES,
  EXCEL_OS_VERSIONS,
  EXCEL_CPU_ARCHS,
  EXCEL_DB_CATEGORIES,
  EXCEL_DB_SOFTWARES,
  EXCEL_DB_DEPLOY_MODES,
  EXCEL_MIDDLEWARE_TYPES,
  EXCEL_MIDDLEWARE_SOFTWARES,
  EXCEL_MW_TYPE_SOFTWARE_MAP,
  EXCEL_DB_CATEGORY_SOFTWARE_MAP,
  EXCEL_APP_RUNTIMES,
  EXCEL_BACKUP_STRATEGIES,
  EXCEL_BACKUP_TYPES,
  EXCEL_BACKUP_METHODS,
  EXCEL_CODE_DICT
} from "../cmdbCodeDict";

interface HostManagementProps {
  projects: ProjectGroup[];
  hosts: PhysicalHost[];
  vms: VmHost[];
  switches?: SwitchDevice[];
  databases?: DatabaseAsset[];
  middlewares?: MiddlewareAsset[];
  backups?: BackupAsset[];
  opsRecords?: OpsAsset[];
  softwareList?: SoftwareComponent[];
  channelList?: OpsChannel[];
  onAddDatabase?: (db: DatabaseAsset) => void;
  onDeleteDatabase?: (id: string) => void;
  onAddMiddleware?: (mw: MiddlewareAsset) => void;
  onDeleteMiddleware?: (id: string) => void;
  onAddBackup?: (bk: BackupAsset) => void;
  onDeleteBackup?: (id: string) => void;
  onAddOpsRecord?: (ops: OpsAsset) => void;
  onDeleteOpsRecord?: (id: string) => void;
  onAddSoftware?: (s: SoftwareComponent) => void;
  onDeleteSoftware?: (id: string) => void;
  onAddChannel?: (c: OpsChannel) => void;
  onDeleteChannel?: (id: string) => void;
  onAddHost?: (h: PhysicalHost) => void;
  onDeleteHost?: (id: string) => void;
  onAddVm: (v: VmHost) => void;
  onDeleteVm: (id: string) => void;
  onBatchImportAssets?: (
    assets: (VmHost & { isImported?: boolean })[],
    strategy: ImportStrategy,
    targetProjectName?: string | null
  ) => void;
  onBatchImportDatabases?: (
    databases: (DatabaseAsset & { isImported?: boolean })[],
    strategy: ImportStrategy,
    targetProjectName?: string | null
  ) => void;
  onBatchImportMiddlewares?: (
    middlewares: (MiddlewareAsset & { isImported?: boolean })[],
    strategy: ImportStrategy,
    targetProjectName?: string | null
  ) => void;
  onBatchImportBackups?: (
    backups: (BackupAsset & { isImported?: boolean })[],
    strategy: ImportStrategy,
    targetProjectName?: string | null
  ) => void;
  onBatchImportOpsRecords?: (
    opsRecords: (OpsAsset & { isImported?: boolean })[],
    strategy: ImportStrategy,
    targetProjectName?: string | null
  ) => void;
  onBatchImportMultiDimension?: (
    data: {
      hardware: (VmHost & { isImported?: boolean })[];
      databases: (DatabaseAsset & { isImported?: boolean })[];
      middlewares: (MiddlewareAsset & { isImported?: boolean })[];
      backups: (BackupAsset & { isImported?: boolean })[];
      opsRecords: (OpsAsset & { isImported?: boolean })[];
    },
    strategy: ImportStrategy,
    targetProjectName?: string | null
  ) => void;
  onDeduplicateAssets?: () => { removedCount: number };
  onUpdateVm?: (v: VmHost) => void;
  onDeleteUnifiedAsset?: (id: string, kind: "physical" | "vm" | "switch") => void;
  onAddProject?: (p: ProjectGroup) => void;
  dataSource?: "mysql" | "v360_api" | "memory";
  isDbConnected?: boolean;
  isApiLoading?: boolean;
  onRefreshApi?: () => void;
  lastSyncTime?: string;
}

type UnifiedAsset = (PhysicalHost | VmHost | SwitchDevice) & {
  _kind: "physical" | "vm" | "switch";
};

export default function HostManagement({
  projects,
  hosts,
  vms,
  switches = [],
  databases = [],
  middlewares = [],
  backups = [],
  opsRecords = [],
  softwareList = [],
  channelList = [],
  onAddSoftware,
  onDeleteSoftware,
  onAddChannel,
  onDeleteChannel,
  onAddHost,
  onDeleteHost,
  onAddVm,
  onDeleteVm,
  onBatchImportAssets,
  onDeduplicateAssets,
  onUpdateVm,
  onDeleteUnifiedAsset,
  onAddProject,
  onAddDatabase,
  onDeleteDatabase,
  onAddMiddleware,
  onDeleteMiddleware,
  onAddBackup,
  onDeleteBackup,
  onAddOpsRecord,
  onDeleteOpsRecord,
  onBatchImportDatabases,
  onBatchImportMiddlewares,
  onBatchImportBackups,
  onBatchImportOpsRecords,
  onBatchImportMultiDimension,
  dataSource = "v360_api",
  isDbConnected = false,
  isApiLoading = false,
  onRefreshApi,
  lastSyncTime = ""
}: HostManagementProps) {
  // Tabs & Modal States for Software, Ops Channels, and VPN
  const [detailTab, setDetailTab] = useState<"spec" | "database" | "middleware" | "backup" | "ops">("spec");
  const [activeDimension, setActiveDimension] = useState<"hardware" | "database" | "middleware" | "backup" | "ops">("hardware");
  const [showAddDbModal, setShowAddDbModal] = useState(false);
  const [showAddMwModal, setShowAddMwModal] = useState(false);
  const [showAddBkModal, setShowAddBkModal] = useState(false);
  const [showAddOpsModal, setShowAddOpsModal] = useState(false);
  const [dbPage, setDbPage] = useState(1);
  const [mwPage, setMwPage] = useState(1);
  const [bkPage, setBkPage] = useState(1);
  const [opsPage, setOpsPage] = useState(1);
  const [copiedNotice, setCopiedNotice] = useState<string | null>(null);
  const [showAddSoftForm, setShowAddSoftForm] = useState(false);
  const [showImportModal, setShowImportModal] = useState(false);
  const [toastNotice, setToastNotice] = useState<string | null>(null);

  function handleExportExcel() {
    const scopeTitle = currentProject ? currentProject.name : "全量项目总览";
    if (activeDimension === "database") {
      exportDatabasesToExcel(displayedDatabases, scopeTitle);
      setToastNotice(`✓ 已成功导出 ${displayedDatabases.length} 条《03-数据库》台账！`);
    } else if (activeDimension === "middleware") {
      exportMiddlewaresToExcel(displayedMiddlewares, scopeTitle);
      setToastNotice(`✓ 已成功导出 ${displayedMiddlewares.length} 条《04-中间件》服务！`);
    } else if (activeDimension === "backup") {
      exportBackupsToExcel(displayedBackups, scopeTitle);
      setToastNotice(`✓ 已成功导出 ${displayedBackups.length} 条《05-备份》策略！`);
    } else if (activeDimension === "ops") {
      exportOpsToExcel(displayedOpsRecords, scopeTitle);
      setToastNotice(`✓ 已成功导出 ${displayedOpsRecords.length} 条《06-运维》保障记录！`);
    } else {
      exportAssetsToExcel(displayedAssets, scopeTitle);
      setToastNotice(`✓ 已成功导出 ${displayedAssets.length} 台资产到《02-硬件设备》Excel！`);
    }
    setTimeout(() => setToastNotice(null), 3500);
  }

  function handleExportAllV351() {
    const scopeTitle = currentProject ? currentProject.name : "全量项目总览";
    exportAllV351Workbook(displayedAssets, displayedDatabases, displayedMiddlewares, displayedBackups, displayedOpsRecords, scopeTitle);
    setToastNotice(`✓ 已成功导出《信息资产台账-v351》全套 5 个工作表 Excel！`);
    setTimeout(() => setToastNotice(null), 3500);
  }
  const [newSoftDraft, setNewSoftDraft] = useState({
    name: "",
    category: "database" as "database" | "middleware" | "plugin" | "web_server",
    version: "",
    port: "",
    installPath: "",
    configPath: "",
    remarks: ""
  });
  const [showAddChanForm, setShowAddChanForm] = useState(false);
  const [newChanDraft, setNewChanDraft] = useState({
    name: "",
    channelType: "web_link" as "web_link" | "ssh" | "rdp" | "jumpserver" | "vpn",
    urlOrTarget: "",
    accountNote: "",
    remarks: ""
  });
  const [showProjectVpnModal, setShowProjectVpnModal] = useState(false);

  function copyToClipboard(text: string, label: string) {
    if (navigator?.clipboard?.writeText) {
      navigator.clipboard.writeText(text);
    }
    setCopiedNotice(label);
    setTimeout(() => setCopiedNotice(null), 2500);
  }
  // Selected project ID ("all" for all assets overview by default)
  const [selectedProjectId, setSelectedProjectId] = useState<string>("all");
  
  // Left Sidebar Project Search & Filter
  const [projectSearch, setProjectSearch] = useState("");
  const [selectedCustomerFilter, setSelectedCustomerFilter] = useState("全部");

  // Right Table Filters
  const [assetKeyword, setAssetKeyword] = useState("");
  const [ipSearchInput, setIpSearchInput] = useState("");
  const [ipSearchKeyword, setIpSearchKeyword] = useState("");
  const [deviceTypeFilter, setDeviceTypeFilter] = useState("全部");
  const [xinchuangFilter, setXinchuangFilter] = useState("全部");
  // Advanced multi-condition filter state
  const [showAdvFilter, setShowAdvFilter] = useState(false);
  const [filterEnv, setFilterEnv] = useState("全部");
  const [filterCloud, setFilterCloud] = useState("全部");
  const [filterRegion, setFilterRegion] = useState("全部");
  const [filterOsFamily, setFilterOsFamily] = useState("全部");

  // UI collapse states
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [headerCollapsed, setHeaderCollapsed] = useState(false);

  // IP Batch Query Modal States
  const [showIpModal, setShowIpModal] = useState(false);
  const [ipBatchText, setIpBatchText] = useState("");


  // Pagination
  const [pageSize, setPageSize] = useState(15);
  const [currentPage, setCurrentPage] = useState(1);

  // Detail Modal
  const [detailAsset, setDetailAsset] = useState<UnifiedAsset | null>(null);

  // Add Asset Modal
  const [showAddModal, setShowAddModal] = useState(false);

  // Combine all assets into a unified pool
  const allAssets: UnifiedAsset[] = useMemo(() => {
    const pList: UnifiedAsset[] = hosts.map(h => ({ ...h, _kind: "physical" }));
    const vList: UnifiedAsset[] = vms.map(v => ({ ...v, _kind: "vm" }));
    const sList: UnifiedAsset[] = switches.map(s => ({ ...s, _kind: "switch" }));
    return [...pList, ...vList, ...sList].sort((a, b) => (a.seq || 9999) - (b.seq || 9999));
  }, [hosts, vms, switches]);


  // Add Project Modal State

  // 5-Dimension Add Form States
  const [newDbDraft, setNewDbDraft] = useState({
    projectName: "",
    customerName: "",
    env: "生产",
    cloudVendor: "联通云",
    regionName: "政务外网区",
    privateIp: "",
    vipEip: "",
    dbCategory: "关系型 (RDBMS / OLTP)",
    dbSoftware: "达梦数据库 (DM)",
    version: "V8.4",
    port: 5236,
    instanceSid: "DMSERVER",
    dbName: "prod_db",
    deployMode: "单机",
    clusterName: "",
    remarks: ""
  });

  const [newMwDraft, setNewMwDraft] = useState({
    projectName: "",
    customerName: "",
    env: "生产",
    cloudVendor: "联通云",
    regionName: "政务外网区",
    privateIp: "",
    mwType: "应用服务器/Java Web 容器",
    mwSoftware: "TongWeb (东方通)",
    version: "7.0.E",
    port: "8080",
    role: "Server",
    runtime: "JDK11",
    remarks: ""
  });

  const [newBkDraft, setNewBkDraft] = useState({
    projectName: "",
    customerName: "",
    env: "生产",
    cloudVendor: "联通云",
    regionName: "政务外网区",
    privateIp: "",
    backupType: "数据库",
    backupMethod: "物理备份",
    backupPolicy: "物理+归档",
    storageLocation: "政务专区专用NAS / 对象存储桶",
    retentionDays: 30,
    remarks: ""
  });

  const [newOpsDraft, setNewOpsDraft] = useState({
    projectName: "",
    customerName: "",
    env: "生产",
    cloudVendor: "联通云",
    regionName: "政务外网区",
    privateIp: "",
    personnelAffiliation: "伟仕",
    vpnNetworkEnv: "互联网区",
    opsVendor: "北京北控伟仕软件有限公司",
    vpnAddress: "114.255.48.18:443",
    vpnAccount: "vpn_ops_prod",
    vpnUserName: "",
    bastionAddress: "https://jumpserver.bj-gov.cn:443",
    bastionAccount: "jumpserver_admin",
    bastionUserName: "",
    serverAccessAddress: "ssh root@192.125.31.x -p 10022",
    monitoringCoverage: "是" as "是" | "部分" | "否",
    inspectionCycle: "每日" as "每日" | "每周" | "每月" | "每季",
    changeWindow: "周五晚",
    networkZone: "业务网",
    exposureSurface: "业务内网",
    remarks: ""
  });

  const [showAddProjectModal, setShowAddProjectModal] = useState(false);
  const [newProjectForm, setNewProjectForm] = useState({
    name: "",
    code: "",
    customerName: "",
    env: "生产",
    cloudVendor: "联通云",
    regionName: "政务外网区",
    description: ""
  });

  // Edit Asset Modal State
  const [editingAsset, setEditingAsset] = useState<UnifiedAsset | null>(null);
  const [editForm, setEditForm] = useState({
    id: "",
    name: "",
    projectName: "",
    customerName: "",
    env: "生产",
    cloudVendor: "联通云",
    regionName: "政务外网区",
    deviceType: "虚拟机",
    privateIp: "",
    internalWanIp: "",
    vip: "",
    eip: "",
    cpuArch: "x86_64",
    cpuCores: 4,
    memoryGb: 16,
    systemDiskGb: 50,
    dataDiskGb: 100,
    osFamily: "CentOS",
    osVersion: "CentOS 7.9",
    kernelVersion: "",
    isXinchuang: "否",
    remotePort: 22,
    remarks: ""
  });

  // Delete Asset Modal State
  const [deletingAsset, setDeletingAsset] = useState<UnifiedAsset | null>(null);

  function openEditModal(item: UnifiedAsset) {
    const rawCpu = (item as any).cpu || "";
    let parsedCores = item.cpuCores || 4;
    let parsedArch = item.cpuArch || (rawCpu.includes("ARM") ? "ARM64" : "x86_64");

    setEditForm({
      id: item.id,
      name: item.name,
      projectName: item.projectName || currentProject?.name || projects[0]?.name || "原三险系统",
      customerName: item.customerName || currentProject?.customerName || "北京市人力资源和社会保障局",
      env: item.env || "生产",
      cloudVendor: item.cloudVendor || "联通云",
      regionName: item.regionName || "政务外网区",
      deviceType: item.deviceType || (item._kind === "physical" ? "物理机" : "虚拟机"),
      privateIp: item.privateIp || item.ip || "",
      internalWanIp: item.internalWanIp || "",
      vip: item.vip || "",
      eip: item.eip || "",
      cpuArch: parsedArch,
      cpuCores: parsedCores,
      memoryGb: item.memoryGb || parseInt((item as any).memory || "16", 10) || 16,
      systemDiskGb: item.systemDiskGb || 50,
      dataDiskGb: item.dataDiskGb || 100,
      osFamily: item.osFamily || "CentOS",
      osVersion: item.osVersion || (item as any).os || "CentOS 7.9",
      kernelVersion: item.kernelVersion || "",
      isXinchuang: item.isXinchuang || "否",
      remotePort: item.remotePort || 22,
      remarks: item.remarks || ""
    });
    setEditingAsset(item);
  }

  function handleEditSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!editingAsset) return;

    const targetProj = projects.find(p => p.name === editForm.projectName) || currentProject || projects[0];

    const updated: VmHost = {
      ...(editingAsset as any),
      id: editForm.id,
      name: editForm.name,
      ip: editForm.privateIp,
      privateIp: editForm.privateIp,
      cpu: `${editForm.cpuCores} 核 (${editForm.cpuArch})`,
      memory: `${editForm.memoryGb} GB`,
      disk: editForm.dataDiskGb > 0 ? `${editForm.systemDiskGb}G(系统) + ${editForm.dataDiskGb}G(数据)` : `${editForm.systemDiskGb} GB`,
      os: `${editForm.osFamily} ${editForm.osVersion}`,
      customerName: editForm.customerName,
      projectName: editForm.projectName,
      projectId: targetProj.id,
      env: editForm.env,
      cloudVendor: editForm.cloudVendor,
      regionName: editForm.regionName,
      deviceType: editForm.deviceType,
      internalWanIp: editForm.internalWanIp || null,
      vip: editForm.vip || null,
      eip: editForm.eip || null,
      cpuArch: editForm.cpuArch,
      cpuCores: editForm.cpuCores,
      memoryGb: editForm.memoryGb,
      systemDiskGb: editForm.systemDiskGb,
      dataDiskGb: editForm.dataDiskGb,
      osFamily: editForm.osFamily,
      osVersion: editForm.osVersion,
      kernelVersion: editForm.kernelVersion,
      isXinchuang: editForm.isXinchuang,
      remotePort: editForm.remotePort,
      remarks: editForm.remarks,
      updated: new Date().toISOString().slice(0, 10)
    };

    if (onUpdateVm) {
      onUpdateVm(updated);
    }
    setEditingAsset(null);
    setToastNotice(`✓ 设备【${updated.name}】信息已成功保存！`);
    setTimeout(() => setToastNotice(null), 3500);
  }

  function handleConfirmDelete() {
    if (!deletingAsset) return;
    const name = deletingAsset.name;
    if (onDeleteUnifiedAsset) {
      onDeleteUnifiedAsset(deletingAsset.id, deletingAsset._kind);
    } else if (onDeleteVm) {
      onDeleteVm(deletingAsset.id);
    }
    setDeletingAsset(null);
    setToastNotice(`✓ 已从资产库中移除设备【${name}】！`);
    setTimeout(() => setToastNotice(null), 3500);
  }

  function handleAddProjectSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!newProjectForm.name.trim()) return;

    const newProj: ProjectGroup = {
      id: newProjectForm.code.trim() || `prj-${Date.now()}`,
      name: newProjectForm.name.trim(),
      code: newProjectForm.code.trim().toUpperCase(),
      customerName: newProjectForm.customerName.trim(),
      env: newProjectForm.env,
      cloudVendor: newProjectForm.cloudVendor,
      regionName: newProjectForm.regionName,
      deviceCount: 0,
      phyCount: 0,
      vmCount: 0,
      netCount: 0,
      totalCores: 0,
      totalMemoryGb: 0,
      totalDiskGb: 0,
      xinchuangCount: 0,
      description: newProjectForm.description,
      healthScore: 100
    };

    if (onAddProject) {
      onAddProject(newProj);
    }
    setSelectedProjectId(newProj.id);
    setShowAddProjectModal(false);
    setToastNotice(`✓ 成功录入新项目【${newProj.name}】！已切换为当前工作项目。`);
    setTimeout(() => setToastNotice(null), 4000);
  }

  // Current Asset Softwares & Channels
  const currentAssetSoftwares = useMemo(() => {
    if (!detailAsset) return [];
    const ip = detailAsset.privateIp || detailAsset.ip;
    return softwareList.filter(s => 
      s.assetId === detailAsset.id || 
      (ip && s.assetIp === ip) || 
      (s.assetName && s.assetName === detailAsset.name)
    );
  }, [detailAsset, softwareList]);

  const currentAssetChannels = useMemo(() => {
    if (!detailAsset) return [];
    const ip = detailAsset.privateIp || detailAsset.ip;
    return channelList.filter(c => 
      c.assetId === detailAsset.id || 
      (ip && c.assetIp === ip) || 
      (detailAsset.projectId && c.projectId === detailAsset.projectId) || 
      (detailAsset.projectName && c.projectName === detailAsset.projectName)
    );
  }, [detailAsset, channelList]);

  // Unique customers for project sidebar filter
  const customerOptions = useMemo(() => {
    const set = new Set<string>();
    projects.forEach(p => set.add(p.customerName));
    return Array.from(set);
  }, [projects]);

  // Filtered projects for left sidebar
  const filteredProjects = useMemo(() => {
    return projects.filter(p => {
      if (selectedCustomerFilter !== "全部" && p.customerName !== selectedCustomerFilter) return false;
      if (projectSearch.trim()) {
        const kw = projectSearch.toLowerCase();
        return p.name.toLowerCase().includes(kw) || p.customerName.toLowerCase().includes(kw) || p.code.toLowerCase().includes(kw);
      }
      return true;
    });
  }, [projects, selectedCustomerFilter, projectSearch]);

  // Current active project
  const currentProject = useMemo(() => {
    if (selectedProjectId === "all") return null;
    return projects.find(p => p.id === selectedProjectId) || null;
  }, [projects, selectedProjectId]);

  // Aggregates for All Projects Overview directly computed from allAssets (exact 299 records)
  const totalOverviewCores = useMemo(() => allAssets.reduce((s, a) => s + (a.cpuCores || 0), 0), [allAssets]);
  const totalOverviewMem = useMemo(() => allAssets.reduce((s, a) => s + (a.memoryGb || 0), 0), [allAssets]);
  const totalOverviewXc = useMemo(() => allAssets.filter(a => a.isXinchuang === "是").length, [allAssets]);

  // Project-level VPN & Links
  const currentProjectVpn = useMemo(() => {
    if (!currentProject) return null;
    return channelList.find(c => c.channelType === "vpn" && (c.projectId === currentProject.id || c.projectName === currentProject.name));
  }, [currentProject, channelList]);

  const currentProjectWebLinks = useMemo(() => {
    if (!currentProject) return [];
    return channelList.filter(c => c.channelType === "web_link" && (c.projectId === currentProject.id || c.projectName === currentProject.name));
  }, [currentProject, channelList]);

  // Filtered assets for current project and right table filters
  const displayedAssets = useMemo(() => {
    return allAssets.filter(item => {
      // Project filter
      if (selectedProjectId !== "all") {
        if (currentProject && (item.projectName !== currentProject.name && item.projectId !== currentProject.id)) {
          return false;
        }
      }

      // Device Type filter
      if (deviceTypeFilter !== "全部") {
        if (deviceTypeFilter === "物理机" && item._kind !== "physical" && item.deviceType !== "物理机") return false;
        if (deviceTypeFilter === "虚拟机" && item._kind !== "vm" && item.deviceType !== "虚拟机") return false;
        if (deviceTypeFilter === "存储" && item.category !== "存储" && item.deviceType !== "对象存储") return false;
        if (deviceTypeFilter === "负载均衡" && item.deviceType !== "负载均衡" && item.category !== "网络") return false;
      }

      // Xinchuang filter
      if (xinchuangFilter !== "全部" && item.isXinchuang !== xinchuangFilter) return false;

      // Keyword search - all 14 fields + multi-keyword filter support
      if (assetKeyword.trim()) {
        const terms = assetKeyword.trim().toLowerCase().split(/\s+/).filter(Boolean);
        const xcStr = item.isXinchuang === "是" ? "国产信创 信创 国产 是" : "常规os 否 非信创";
        const str = [
          item.name,
          (item as any).hostname,
          item.deviceType,
          item.projectName,
          item.customerName,
          item.env,
          item.cloudVendor,
          item.regionName,
          item.privateIp,
          (item as any).ip,
          item.internalWanIp,
          item.vip,
          item.eip,
          item.osFamily,
          item.osVersion,
          (item as any).os,
          item.kernelVersion,
          item.cpuArch,
          xcStr,
          item.remarks,
          (item as any).category
        ].filter(Boolean).join(" ").toLowerCase();

        const allMatched = terms.every(t => str.includes(t));
        if (!allMatched) return false;
      }

      // Advanced filters
      if (filterEnv !== "全部" && item.env !== filterEnv) return false;
      if (filterCloud !== "全部" && item.cloudVendor !== filterCloud) return false;
      if (filterRegion !== "全部" && item.regionName !== filterRegion) return false;
      if (filterOsFamily !== "全部" && item.osFamily !== filterOsFamily) return false;

      // IP address search (supports single IP, prefix/subnet, or multi-IPs)
      if (ipSearchKeyword.trim()) {
        const rawKw = ipSearchKeyword.trim();
        const targetIps = rawKw.split(/[\s,;，；\n]+/).map(s => s.trim().toLowerCase()).filter(Boolean);
        const hostIps = [
          item.privateIp,
          (item as any).ip,
          item.internalWanIp,
          item.vip,
          item.eip,
          (item as any).publicIp
        ].filter(Boolean).map(s => String(s).toLowerCase());

        const matched = targetIps.some(tip => 
          hostIps.some(hip => hip === tip || hip.includes(tip))
        );
        if (!matched) return false;
      }

      return true;
    });
  }, [allAssets, selectedProjectId, currentProject, deviceTypeFilter, xinchuangFilter, assetKeyword, ipSearchKeyword, filterEnv, filterCloud, filterRegion, filterOsFamily]);
  // ── 03-数据库 筛选与分页 ──
  const displayedDatabases = useMemo(() => {
    return (databases || []).filter(db => {
      if (selectedProjectId !== "all") {
        if (currentProject && db.projectName !== currentProject.name && (db as any).projectId !== currentProject.id) {
          return false;
        }
      }
      if (ipSearchKeyword.trim()) {
        const targetIp = ipSearchKeyword.trim().toLowerCase();
        const ips = [db.privateIp, db.hostIp, db.vipEip].filter(Boolean).map(x => String(x).toLowerCase());
        if (!ips.some(ip => ip.includes(targetIp))) return false;
      }
      if (assetKeyword.trim()) {
        const terms = assetKeyword.trim().toLowerCase().split(/\s+/).filter(Boolean);
        const str = [
          db.name, db.projectName, db.customerName, db.dbSoftware, db.dbCategory,
          db.type, db.version, db.privateIp, db.hostIp, db.vipEip, db.instanceSid,
          db.dbName, db.deployMode, db.clusterName, db.remarks, db.cloudVendor, db.env
        ].filter(Boolean).join(" ").toLowerCase();
        return terms.every(t => str.includes(t));
      }
      return true;
    }).sort((a, b) => (a.seq || 999) - (b.seq || 999));
  }, [databases, selectedProjectId, currentProject, ipSearchKeyword, assetKeyword]);

  // ── 04-中间件 筛选与分页 ──
  const displayedMiddlewares = useMemo(() => {
    return (middlewares || []).filter(mw => {
      if (selectedProjectId !== "all") {
        if (currentProject && mw.projectName !== currentProject.name && mw.projectId !== currentProject.id) {
          return false;
        }
      }
      if (ipSearchKeyword.trim()) {
        const targetIp = ipSearchKeyword.trim().toLowerCase();
        const ips = [mw.privateIp, mw.assetIp].filter(Boolean).map(x => String(x).toLowerCase());
        if (!ips.some(ip => ip.includes(targetIp))) return false;
      }
      if (assetKeyword.trim()) {
        const terms = assetKeyword.trim().toLowerCase().split(/\s+/).filter(Boolean);
        const str = [
          mw.name, mw.mwSoftware, mw.mwType, mw.version, mw.port, mw.role, mw.runtime,
          mw.projectName, mw.customerName, mw.privateIp, mw.assetName, mw.remarks, mw.cloudVendor, mw.env
        ].filter(Boolean).join(" ").toLowerCase();
        return terms.every(t => str.includes(t));
      }
      return true;
    }).sort((a, b) => (a.seq || 999) - (b.seq || 999));
  }, [middlewares, selectedProjectId, currentProject, ipSearchKeyword, assetKeyword]);

  // ── 05-备份 筛选与分页 ──
  const displayedBackups = useMemo(() => {
    return (backups || []).filter(bk => {
      if (selectedProjectId !== "all") {
        if (currentProject && bk.projectName !== currentProject.name && bk.projectId !== currentProject.id) {
          return false;
        }
      }
      if (ipSearchKeyword.trim()) {
        const targetIp = ipSearchKeyword.trim().toLowerCase();
        if (!bk.privateIp || !bk.privateIp.toLowerCase().includes(targetIp)) return false;
      }
      if (assetKeyword.trim()) {
        const terms = assetKeyword.trim().toLowerCase().split(/\s+/).filter(Boolean);
        const str = [
          bk.projectName, bk.customerName, bk.privateIp, bk.backupType, bk.backupMethod,
          bk.backupPolicy, bk.storageLocation, bk.remarks, bk.cloudVendor, bk.env
        ].filter(Boolean).join(" ").toLowerCase();
        return terms.every(t => str.includes(t));
      }
      return true;
    }).sort((a, b) => (a.seq || 999) - (b.seq || 999));
  }, [backups, selectedProjectId, currentProject, ipSearchKeyword, assetKeyword]);

  // ── 06-运维 筛选与分页 ──
  const displayedOpsRecords = useMemo(() => {
    return (opsRecords || []).filter(ops => {
      if (selectedProjectId !== "all") {
        if (currentProject && ops.projectName !== currentProject.name && ops.projectId !== currentProject.id) {
          return false;
        }
      }
      if (ipSearchKeyword.trim()) {
        const targetIp = ipSearchKeyword.trim().toLowerCase();
        if (!ops.privateIp || !ops.privateIp.toLowerCase().includes(targetIp)) return false;
      }
      if (assetKeyword.trim()) {
        const terms = assetKeyword.trim().toLowerCase().split(/\s+/).filter(Boolean);
        const str = [
          ops.projectName, ops.customerName, ops.privateIp, ops.opsVendor, ops.vpnAddress,
          ops.vpnAccount, ops.vpnUserName, ops.bastionAddress, ops.bastionAccount, ops.bastionUserName,
          ops.personnelAffiliation, ops.vpnNetworkEnv, ops.serverAccessAddress, ops.monitoringCoverage,
          ops.inspectionCycle, ops.changeWindow, ops.networkZone, ops.exposureSurface, ops.remarks,
          ops.cloudVendor, ops.env
        ].filter(Boolean).join(" ").toLowerCase();
        return terms.every(t => str.includes(t));
      }
      return true;
    }).sort((a, b) => (a.seq || 999) - (b.seq || 999));
  }, [opsRecords, selectedProjectId, currentProject, ipSearchKeyword, assetKeyword]);

  const paginatedDatabases = useMemo(() => {
    const start = (dbPage - 1) * pageSize;
    return displayedDatabases.slice(start, start + pageSize);
  }, [displayedDatabases, dbPage, pageSize]);

  const paginatedMiddlewares = useMemo(() => {
    const start = (mwPage - 1) * pageSize;
    return displayedMiddlewares.slice(start, start + pageSize);
  }, [displayedMiddlewares, mwPage, pageSize]);

  const paginatedBackups = useMemo(() => {
    const start = (bkPage - 1) * pageSize;
    return displayedBackups.slice(start, start + pageSize);
  }, [displayedBackups, bkPage, pageSize]);

  const paginatedOpsRecords = useMemo(() => {
    const start = (opsPage - 1) * pageSize;
    return displayedOpsRecords.slice(start, start + pageSize);
  }, [displayedOpsRecords, opsPage, pageSize]);


  // Cross-project IP match detection
  const crossProjectIpMatch = useMemo(() => {
    if (!ipSearchKeyword.trim() || selectedProjectId === "all") return null;
    if (displayedAssets.length > 0) return null; // Already matched in current project

    const targetIps = ipSearchKeyword.trim().split(/[\s,;，；\n]+/).map(s => s.trim().toLowerCase()).filter(Boolean);
    const matchedInAll = allAssets.filter(item => {
      const hostIps = [
        item.privateIp,
        (item as any).ip,
        item.internalWanIp,
        item.vip,
        item.eip,
        (item as any).publicIp
      ].filter(Boolean).map(s => String(s).toLowerCase());
      return targetIps.some(tip => hostIps.some(hip => hip === tip || hip.includes(tip)));
    });

    if (matchedInAll.length > 0) {
      const projNames = Array.from(new Set(matchedInAll.map(a => a.projectName).filter(Boolean)));
      return {
        count: matchedInAll.length,
        projects: projNames,
        targetProject: projects.find(p => p.name === projNames[0]) || null,
        matchedAssets: matchedInAll
      };
    }
    return null;
  }, [ipSearchKeyword, selectedProjectId, displayedAssets.length, allAssets, projects]);

  // Batch IP Query calculation
  const batchIpAnalysis = useMemo(() => {
    if (!ipBatchText.trim()) return null;
    const inputLines = ipBatchText
      .split(/[\s,;，；\n]+/)
      .map(s => s.trim())
      .filter(Boolean);
    const uniqueInputs = Array.from(new Set(inputLines));
    
    const matchedAssetsList: UnifiedAsset[] = [];
    const matchedIpSet = new Set<string>();
    const notFoundIps: string[] = [];

    uniqueInputs.forEach(inputIp => {
      const lower = inputIp.toLowerCase();
      const hits = allAssets.filter(item => {
        const hostIps = [
          item.privateIp,
          (item as any).ip,
          item.internalWanIp,
          item.vip,
          item.eip,
          (item as any).publicIp
        ].filter(Boolean).map(s => String(s).toLowerCase());
        return hostIps.some(hip => hip === lower || hip.includes(lower));
      });

      if (hits.length > 0) {
        matchedIpSet.add(inputIp);
        hits.forEach(h => {
          if (!matchedAssetsList.some(m => m.id === h.id)) {
            matchedAssetsList.push(h);
          }
        });
      } else {
        notFoundIps.push(inputIp);
      }
    });

    return {
      totalInputs: uniqueInputs.length,
      matchedIpCount: matchedIpSet.size,
      matchedAssets: matchedAssetsList,
      notFoundIps
    };
  }, [ipBatchText, allAssets]);

  // Linked items for detailAsset across 5 dimensions
  const detailAssetIp = detailAsset ? (detailAsset.privateIp || (detailAsset as any).ip || "") : "";
  const detailAssetDbs = useMemo(() => {
    if (!detailAsset) return [];
    return (databases || []).filter(d => 
      (detailAssetIp && (d.privateIp === detailAssetIp || d.hostIp === detailAssetIp)) ||
      (detailAsset.projectName && d.projectName === detailAsset.projectName)
    );
  }, [detailAsset, detailAssetIp, databases]);

  const detailAssetMws = useMemo(() => {
    if (!detailAsset) return [];
    return (middlewares || []).filter(m => 
      (detailAssetIp && (m.privateIp === detailAssetIp || m.assetIp === detailAssetIp)) ||
      (detailAsset.projectName && m.projectName === detailAsset.projectName)
    );
  }, [detailAsset, detailAssetIp, middlewares]);

  const detailAssetBks = useMemo(() => {
    if (!detailAsset) return [];
    return (backups || []).filter(b => 
      (detailAssetIp && b.privateIp === detailAssetIp) ||
      (detailAsset.projectName && b.projectName === detailAsset.projectName)
    );
  }, [detailAsset, detailAssetIp, backups]);

  const detailAssetOps = useMemo(() => {
    if (!detailAsset) return [];
    return (opsRecords || []).filter(o => 
      (detailAssetIp && o.privateIp === detailAssetIp) ||
      (detailAsset.projectName && o.projectName === detailAsset.projectName)
    );
  }, [detailAsset, detailAssetIp, opsRecords]);

  // Multi-dimension pagination helpers
  const currentDimTotal = activeDimension === "hardware" ? displayedAssets.length
    : activeDimension === "database" ? displayedDatabases.length
    : activeDimension === "middleware" ? displayedMiddlewares.length
    : activeDimension === "backup" ? displayedBackups.length
    : displayedOpsRecords.length;

  const currentDimPage = activeDimension === "hardware" ? currentPage
    : activeDimension === "database" ? dbPage
    : activeDimension === "middleware" ? mwPage
    : activeDimension === "backup" ? bkPage
    : opsPage;

  const currentDimTotalPages = Math.ceil(currentDimTotal / pageSize) || 1;

  function setCurrentDimPage(valOrFn: number | ((prev: number) => number)) {
    if (activeDimension === "hardware") setCurrentPage(valOrFn);
    else if (activeDimension === "database") setDbPage(valOrFn);
    else if (activeDimension === "middleware") setMwPage(valOrFn);
    else if (activeDimension === "backup") setBkPage(valOrFn);
    else setOpsPage(valOrFn);
  }

  // Pagination slice
  const totalPages = Math.ceil(displayedAssets.length / pageSize) || 1;
  const paginatedAssets = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return displayedAssets.slice(start, start + pageSize);
  }, [displayedAssets, currentPage, pageSize]);

  // Add Asset Form State
  const [newForm, setNewForm] = useState({
    name: "",
    projectName: currentProject?.name || projects[0]?.name || "原三险系统",
    customerName: currentProject?.customerName || projects[0]?.customerName || "北京市人力资源和社会保障局",
    env: currentProject?.env || "生产",
    cloudVendor: currentProject?.cloudVendor || "联通云",
    regionName: currentProject?.regionName || "政务外网区",
    deviceType: "虚拟机",
    ip: "192.125.31.250",
    privateIp: "192.125.31.250",
    internalWanIp: "192.123.2.110",
    vip: "",
    eip: "",
    cpuArch: "x86_64",
    cpuCores: 8,
    memoryGb: 16,
    systemDiskGb: 50,
    dataDiskGb: 200,
    osFamily: "CentOS",
    osVersion: "Red Hat 6.9",
    kernelVersion: "2.6.32-754.35.1.el6.x86_64",
    isXinchuang: "否",
    remotePort: 2222,
    remarks: "新增项目业务节点"
  });

  // When switching projects, update modal default project
  function handleSelectProject(pId: string) {
    setSelectedProjectId(pId);
    setCurrentPage(1);
    const p = projects.find(x => x.id === pId);
    if (p) {
      setNewForm(prev => ({
        ...prev,
        projectName: p.name,
        customerName: p.customerName,
        env: p.env,
        cloudVendor: p.cloudVendor,
        regionName: p.regionName
      }));
    }
  }

  // Handle Add Asset Submit
  function handleAddSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!newForm.name.trim()) return;

    const newSeq = (allAssets[allAssets.length - 1]?.seq || 299) + 1;
    const targetProj = projects.find(p => p.name === newForm.projectName) || projects[0];

    const newVm: VmHost = {
      id: `vm-${Date.now()}`,
      seq: newSeq,
      name: newForm.name,
      physicalHostId: "phy-1",
      ip: newForm.ip,
      privateIp: newForm.privateIp,
      cpu: `${newForm.cpuCores} 核 vCPU (${newForm.cpuArch})`,
      memory: `${newForm.memoryGb} GB`,
      disk: `${newForm.systemDiskGb}G(系统) + ${newForm.dataDiskGb}G(数据)`,
      os: `${newForm.osFamily} ${newForm.osVersion}`,
      businessId: targetProj.id,
      status: "running",
      role: "项目应用节点",
      updated: new Date().toLocaleString("zh-CN", { hour12: false }).slice(0, 16),
      // Excel metadata
      customerName: newForm.customerName,
      projectName: newForm.projectName,
      projectId: targetProj.id,
      env: newForm.env,
      cloudVendor: newForm.cloudVendor,
      regionName: newForm.regionName,
      category: "服务器",
      deviceType: newForm.deviceType,
      internalWanIp: newForm.internalWanIp || undefined,
      eip: newForm.eip || undefined,
      vip: newForm.vip || undefined,
      publicIp: undefined,
      cpuArch: newForm.cpuArch,
      cpuCores: newForm.cpuCores,
      memoryGb: newForm.memoryGb,
      systemDiskGb: newForm.systemDiskGb,
      dataDiskGb: newForm.dataDiskGb,
      osFamily: newForm.osFamily,
      osVersion: newForm.osVersion,
      kernelVersion: newForm.kernelVersion,
      isXinchuang: newForm.isXinchuang,
      remotePort: newForm.remotePort,
      remarks: newForm.remarks
    };

    onAddVm(newVm);
    setShowAddModal(false);
    setToastNotice(`✓ 成功录入新资产【${newVm.name}】到项目【${newVm.projectName}】！`);
    setTimeout(() => setToastNotice(null), 3500);
  }

  return (
    <div className="cmdb-container" style={{ display: "flex", flexDirection: "row", gap: 14, height: "calc(100vh - 90px)", paddingBottom: 0, alignItems: "stretch" }}>
      {/* ================= LEFT SIDEBAR: PROJECTS LIST ================= */}
      <div style={{
        width: sidebarCollapsed ? 38 : 290,
        minWidth: sidebarCollapsed ? 38 : 290,
        background: "#fff",
        border: "1px solid #e2e8f0",
        borderRadius: 8,
        display: "flex",
        flexDirection: "column",
        overflow: "hidden",
        boxShadow: "0 1px 3px rgba(0,0,0,0.05)",
        transition: "width 0.25s ease, min-width 0.25s ease",
        position: "relative"
      }}>
        {/* Collapsed state: show clean vertical column with top expand button */}
        {sidebarCollapsed ? (
          <div
            style={{
              flex: 1,
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              paddingTop: 10,
              gap: 12
            }}
          >
            <button
              type="button"
              onClick={() => setSidebarCollapsed(false)}
              title="展开项目资产目录"
              style={{
                width: 24,
                height: 24,
                borderRadius: 4,
                border: "1px solid #93c5fd",
                background: "#eff6ff",
                cursor: "pointer",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                fontSize: 14,
                fontWeight: 700,
                color: "#1e40af",
                padding: 0
              }}
            >
              ›
            </button>
            <div style={{
              writingMode: "vertical-rl",
              fontSize: 12,
              fontWeight: 600,
              color: "#64748b",
              letterSpacing: 3,
              userSelect: "none"
            }}>项目资产目录</div>
          </div>
        ) : (
          <>
        <div style={{ padding: "12px 14px", borderBottom: "1px solid #e2e8f0", background: "#f8fafc" }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
            <span style={{ fontWeight: 700, color: "#0f172a", fontSize: 14, whiteSpace: "nowrap" }}>
              📁 项目资产目录
            </span>
            <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
              <span style={{ fontSize: 11, color: "#64748b", background: "#e2e8f0", padding: "1px 6px", borderRadius: 10, whiteSpace: "nowrap" }}>
                {projects.length} 个
              </span>
              <button
                type="button"
                className="btn-primary"
                onClick={() => {
                  const nextCode = `prj-${String(projects.length + 1).padStart(3, "0")}`;
                  setNewProjectForm({
                    name: "",
                    code: nextCode,
                    customerName: "",
                    env: "生产",
                    cloudVendor: "联通云",
                    regionName: "政务外网区",
                    description: ""
                  });
                  setShowAddProjectModal(true);
                }}
                style={{
                  fontSize: 11,
                  padding: "2px 7px",
                  display: "inline-flex",
                  alignItems: "center",
                  gap: 3,
                  cursor: "pointer",
                  whiteSpace: "nowrap"
                }}
                title="录入并创建新的业务项目单位"
              >
                <span>＋</span>
                <span>录入项目</span>
              </button>
              <button
                type="button"
                onClick={() => setSidebarCollapsed(true)}
                title="收缩项目资产目录"
                style={{
                  width: 22,
                  height: 22,
                  borderRadius: 4,
                  border: "1px solid #cbd5e1",
                  background: "#fff",
                  cursor: "pointer",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  fontSize: 13,
                  fontWeight: 700,
                  color: "#475569",
                  padding: 0,
                  flexShrink: 0
                }}
              >
                ‹
              </button>
            </div>
          </div>

          {/* Search Project */}
          <input 
            placeholder="搜索项目名称 / 客户单位..." 
            value={projectSearch}
            onChange={e => setProjectSearch(e.target.value)}
            style={{ width: "100%", fontSize: 12, padding: "5px 8px", marginBottom: 6 }}
          />

          {/* Filter Customer */}
          <select 
            value={selectedCustomerFilter} 
            onChange={e => setSelectedCustomerFilter(e.target.value)}
            style={{ width: "100%", fontSize: 11, padding: "4px 6px" }}
          >
            <option value="全部">全部客户单位 ({customerOptions.length})</option>
            {customerOptions.map(c => (
              <option key={c} value={c}>{c}</option>
            ))}
          </select>
        </div>

        {/* Project Items Scroll List */}
        <div style={{ flex: 1, overflowY: "auto", padding: "8px" }}>
          {/* Option: View All Assets */}
          <div 
            onClick={() => handleSelectProject("all")}
            style={{
              padding: "10px 12px",
              borderRadius: 6,
              cursor: "pointer",
              marginBottom: 6,
              background: selectedProjectId === "all" ? "#eff6ff" : "transparent",
              border: selectedProjectId === "all" ? "1.5px solid #2563eb" : "1px dashed #cbd5e1",
              transition: "all 0.15s"
            }}
          >
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
              <span style={{ fontWeight: 600, fontSize: 13, color: selectedProjectId === "all" ? "#1d4ed8" : "#334155" }}>
                🌟 全部项目总览
              </span>
              <span style={{ fontSize: 11, fontWeight: 700, color: "#2563eb", background: "#dbeafe", padding: "1px 6px", borderRadius: 10 }}>
                {allAssets.length} 台
              </span>
            </div>
            <small style={{ color: "#64748b", fontSize: 11, display: "block", marginTop: 2 }}>
              查看跨项目 {allAssets.length} 台信息资产台账全集
            </small>
          </div>

          {/* Individual Project Cards */}
          {filteredProjects.map(proj => {
            const isSelected = proj.id === selectedProjectId;

            return (
              <div 
                key={proj.id}
                onClick={() => handleSelectProject(proj.id)}
                style={{
                  padding: "10px 12px",
                  borderRadius: 6,
                  cursor: "pointer",
                  marginBottom: 6,
                  background: isSelected ? "#eff6ff" : "#fff",
                  border: isSelected ? "1.5px solid #2563eb" : "1px solid #f1f5f9",
                  boxShadow: isSelected ? "0 2px 8px rgba(37,99,235,0.12)" : "none",
                  transition: "all 0.15s"
                }}
              >
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 3 }}>
                  <span style={{
                    fontWeight: 600,
                    fontSize: 13,
                    color: isSelected ? "#1d4ed8" : "#1e293b",
                    whiteSpace: "nowrap",
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    flex: 1,
                    minWidth: 0,
                    marginRight: 8
                  }}>
                    {proj.name}
                  </span>
                  <span style={{
                    fontSize: 11,
                    fontWeight: 700,
                    padding: "1px 6px",
                    borderRadius: 10,
                    background: isSelected ? "#2563eb" : "#f1f5f9",
                    color: isSelected ? "#fff" : "#475569"
                  }}>
                    {proj.deviceCount} 台
                  </span>
                </div>

                <div style={{ fontSize: 11, color: "#64748b", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis", marginBottom: 4 }}>
                  {proj.customerName}
                </div>

                <div style={{ display: "flex", gap: 4, alignItems: "center", fontSize: 10 }}>
                  <span style={{ 
                    padding: "0 4px", 
                    borderRadius: 3, 
                    background: proj.env === "生产" ? "#dcfce7" : "#f1f5f9",
                    color: proj.env === "生产" ? "#15803d" : "#64748b",
                    fontWeight: 500
                  }}>
                    {proj.env}
                  </span>
                  <span style={{ color: "#2563eb", background: "#eff6ff", padding: "0 4px", borderRadius: 3 }}>
                    {proj.cloudVendor}
                  </span>
                  {proj.xinchuangCount > 0 && (
                    <span style={{ color: "#dc2626", background: "#fee2e2", padding: "0 4px", borderRadius: 3, fontWeight: 600 }}>
                      信创 {proj.xinchuangCount}
                    </span>
                  )}
                </div>
              </div>
            );
          })}
        </div>
          </>
        )}
      </div>

      {/* ================= RIGHT WORKBENCH: ASSET TABLE ================= */}
      <div style={{ flex: 1, display: "flex", flexDirection: "column", minWidth: 0, gap: 10 }}>
        {/* Project Context Header Banner */}
        <div style={{
          background: "#fff",
          border: "1px solid #e2e8f0",
          borderRadius: 8,
          padding: headerCollapsed ? "8px 18px" : "12px 18px",
          boxShadow: "0 1px 3px rgba(0,0,0,0.04)",
          position: "relative"
        }}>
          {/* Collapse toggle for header */}
          <button
            type="button"
            onClick={() => setHeaderCollapsed(!headerCollapsed)}
            title={headerCollapsed ? "展开项目信息" : "收缩项目信息"}
            style={{
              position: "absolute",
              top: 8,
              right: 10,
              width: 22, height: 22,
              borderRadius: 4,
              border: "1px solid #e2e8f0",
              background: "#f8fafc",
              cursor: "pointer",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              fontSize: 12,
              color: "#64748b",
              padding: 0,
              zIndex: 5
            }}
          >
            {headerCollapsed ? "∨" : "∧"}
          </button>

          {/* Collapsed: single-line summary */}
          {headerCollapsed ? (
            <div style={{ display: "flex", alignItems: "center", gap: 10, paddingRight: 30 }}>
              <span style={{ background: "#2563eb", color: "#fff", fontSize: 11, padding: "1px 7px", borderRadius: 4, fontWeight: 600, whiteSpace: "nowrap" }}>
                {currentProject ? currentProject.code : "ALL"}
              </span>
              <span style={{ fontWeight: 600, fontSize: 14, color: "#0f172a", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                {currentProject ? currentProject.name : "全部项目总览"}
              </span>
              <span style={{ fontSize: 11, color: "#64748b", whiteSpace: "nowrap" }}>
                {currentProject 
                  ? `${currentProject.customerName} · ${currentProject.env} · ${currentProject.totalCores} Cores · ${currentProject.totalMemoryGb} GB`
                  : `汇聚 ${projects.length} 个项目 · 全网 ${displayedAssets.length} 台资产 · ${totalOverviewCores} Cores · ${totalOverviewMem} GB`
                }
              </span>
              <div style={{ marginLeft: "auto", display: "flex", gap: 6, alignItems: "center" }}>
                {currentProjectVpn && (
                  <button type="button" onClick={() => setShowProjectVpnModal(true)}
                    style={{ padding: "3px 10px", background: "linear-gradient(135deg,#bbf7d0,#86efac)", border: "1px solid #4ade80", borderRadius: 6, color: "#14532d", fontWeight: 600, fontSize: 11, cursor: "pointer" }}>
                    🛡️ VPN
                  </button>
                )}
                <button type="button" onClick={() => setShowImportModal(true)}
                  style={{ padding: "3px 10px", background: "linear-gradient(135deg,#dbeafe,#bfdbfe)", border: "1px solid #93c5fd", borderRadius: 6, color: "#1e40af", fontWeight: 600, fontSize: 11, cursor: "pointer" }}>
                  📥 导入
                </button>
                <button type="button" onClick={handleExportExcel}
                  style={{ padding: "3px 10px", background: "linear-gradient(135deg,#f8fafc,#f1f5f9)", border: "1px solid #cbd5e1", borderRadius: 6, color: "#334155", fontWeight: 600, fontSize: 11, cursor: "pointer" }}>
                  📤 导出
                </button>
                <button type="button" onClick={() => { setIpBatchText(ipSearchKeyword || ""); setShowIpModal(true); }}
                  style={{ padding: "3px 10px", background: "linear-gradient(135deg,#ecfdf5,#d1fae5)", border: "1px solid #6ee7b7", borderRadius: 6, color: "#065f46", fontWeight: 600, fontSize: 11, cursor: "pointer" }}>
                  🌐 IP查询
                </button>
                <button type="button" onClick={() => setShowAddModal(true)}
                  style={{ padding: "3px 12px", background: "linear-gradient(135deg,#2563eb,#1d4ed8)", border: "none", borderRadius: 6, color: "#fff", fontWeight: 700, fontSize: 11, cursor: "pointer", boxShadow: "0 1px 4px rgba(37,99,235,0.3)" }}>
                  ＋ 录入
                </button>
              </div>
            </div>
          ) : (
            <>
            {/* ── 项目标题行 ─────────────────────────── */}
            <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 12, flexWrap: "wrap" }}>
              <div>
              <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
                <span style={{
                  background: "#2563eb",
                  color: "#fff",
                  fontSize: 12,
                  padding: "2px 8px",
                  borderRadius: 4,
                  fontWeight: 600,
                  whiteSpace: "nowrap"
                }}>
                  {currentProject ? currentProject.code : "ALL"}
                </span>
                <h3 style={{ margin: 0, fontSize: 17, color: "#0f172a", whiteSpace: "nowrap" }}>
                  {currentProject ? currentProject.name : "全部项目总览"}
                </h3>
                {currentProject ? (
                  <span style={{ 
                    background: currentProject.env === "生产" ? "#dcfce7" : "#f1f5f9",
                    color: currentProject.env === "生产" ? "#15803d" : "#475569",
                    fontSize: 11,
                    padding: "1px 6px",
                    borderRadius: 4,
                    whiteSpace: "nowrap"
                  }}>
                    {currentProject.env}环境
                  </span>
                ) : (
                  <span style={{ 
                    background: "#eff6ff",
                    color: "#2563eb",
                    fontSize: 11,
                    padding: "1px 8px",
                    borderRadius: 4,
                    fontWeight: 600,
                    whiteSpace: "nowrap"
                  }}>
                    全量跨项目全景
                  </span>
                )}
              </div>
              <p style={{ margin: "4px 0 0", fontSize: 12, color: "#64748b", whiteSpace: "nowrap" }}>
                {currentProject 
                  ? `客户单位: ${currentProject.customerName} · 承载云厂商: ${currentProject.cloudVendor} (${currentProject.regionName})`
                  : `汇聚 ${projects.length} 个重点项目 · 覆盖联通云、首信云、国企云、太极云、阿里云等多云算力资源`
                }
              </p>
            </div>

            {/* 资源统计卡片 */}
            <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
              <div style={{ background: "#f8fafc", border: "1px solid #e2e8f0", padding: "6px 14px", borderRadius: 8, textAlign: "center", minWidth: 60 }}>
                <span style={{ fontSize: 10, color: "#64748b", display: "block", marginBottom: 2 }}>
                  {currentProject ? "项目资产" : "纳管总资产"}
                </span>
                <strong style={{ fontSize: 16, color: "#0f172a" }}>
                  {displayedAssets.length} <small style={{ fontSize: 10, fontWeight: 400 }}>台</small>
                </strong>
              </div>

              <div style={{ background: "#eff6ff", border: "1px solid #bfdbfe", padding: "6px 14px", borderRadius: 8, textAlign: "center", minWidth: 60 }}>
                <span style={{ fontSize: 10, color: "#64748b", display: "block", marginBottom: 2 }}>算力核数</span>
                <strong style={{ fontSize: 16, color: "#2563eb" }}>
                  {currentProject ? currentProject.totalCores : totalOverviewCores} <small style={{ fontSize: 10, fontWeight: 400 }}>Cores</small>
                </strong>
              </div>
              <div style={{ background: "#f0fdf4", border: "1px solid #bbf7d0", padding: "6px 14px", borderRadius: 8, textAlign: "center", minWidth: 60 }}>
                <span style={{ fontSize: 10, color: "#64748b", display: "block", marginBottom: 2 }}>内存池</span>
                <strong style={{ fontSize: 16, color: "#059669" }}>
                  {currentProject ? currentProject.totalMemoryGb : totalOverviewMem} <small style={{ fontSize: 10, fontWeight: 400 }}>GB</small>
                </strong>
              </div>
              <div style={{ background: "#fff7ed", border: "1px solid #fed7aa", padding: "6px 14px", borderRadius: 8, textAlign: "center", minWidth: 60 }}>
                <span style={{ fontSize: 10, color: "#64748b", display: "block", marginBottom: 2 }}>国产信创</span>
                <strong style={{ fontSize: 16, color: "#c2410c" }}>
                  {currentProject ? currentProject.xinchuangCount : totalOverviewXc} <small style={{ fontSize: 10, fontWeight: 400 }}>台</small>
                </strong>
              </div>
            </div>
          </div>

          {/* ── 操作按钮行 ───────────────────────────── */}
          <div style={{
            marginTop: 12,
            paddingTop: 12,
            borderTop: "1px solid #f1f5f9",
            display: "flex",
            alignItems: "center",
            gap: 8,
            flexWrap: "wrap"
          }}>
            {/* VPN 按钮（仅有 VPN 时显示） */}
            {currentProjectVpn && (
              <button 
                type="button"
                onClick={() => setShowProjectVpnModal(true)} 
                style={{ 
                  padding: "7px 14px", 
                  background: "linear-gradient(135deg,#bbf7d0,#86efac)",
                  border: "1px solid #4ade80",
                  borderRadius: 8,
                  color: "#14532d", 
                  fontWeight: 600,
                  fontSize: 12,
                  display: "inline-flex",
                  alignItems: "center",
                  gap: 5,
                  cursor: "pointer",
                  boxShadow: "0 1px 3px rgba(74,222,128,0.2)"
                }}
                title="查看当前项目专网 VPN 网关及拨号策略"
              >
                🛡️ 项目专网 VPN
              </button>
            )}

            {/* 分隔线 */}
            {currentProjectVpn && <div style={{ width: 1, height: 24, background: "#e2e8f0", flexShrink: 0 }} />}

            {/* 📥 导入 */}
            <button
              type="button"
              onClick={() => setShowImportModal(true)}
              style={{
                padding: "7px 14px",
                background: "linear-gradient(135deg,#dbeafe,#bfdbfe)",
                border: "1px solid #93c5fd",
                borderRadius: 8,
                color: "#1e40af",
                fontWeight: 600,
                fontSize: 12,
                display: "inline-flex",
                alignItems: "center",
                gap: 5,
                cursor: "pointer",
                whiteSpace: "nowrap",
                boxShadow: "0 1px 3px rgba(147,197,253,0.2)"
              }}
              title={currentProject ? `导入 Excel 资产到【${currentProject.name}】` : "批量导入《信息资产台账》Excel 资产"}
            >
              📥 {currentProject ? "导入台账到当前项目" : "导入台账"}
            </button>

            {/* 📤 导出当前视图 */}
            <button
              type="button"
              onClick={handleExportExcel}
              style={{
                padding: "7px 14px",
                background: "linear-gradient(135deg,#f8fafc,#f1f5f9)",
                border: "1px solid #cbd5e1",
                borderRadius: 8,
                color: "#334155",
                fontWeight: 600,
                fontSize: 12,
                display: "inline-flex",
                alignItems: "center",
                gap: 5,
                cursor: "pointer",
                whiteSpace: "nowrap",
                boxShadow: "0 1px 3px rgba(0,0,0,0.04)"
              }}
              title={`导出当前 [${activeDimension === 'hardware' ? '02硬件设备' : activeDimension === 'database' ? '03数据库' : activeDimension === 'middleware' ? '04中间件' : activeDimension === 'backup' ? '05备份' : '06运维'}] 维度的台账 Excel`}
            >
              📤 导出当前视图 ({activeDimension === 'hardware' ? displayedAssets.length : activeDimension === 'database' ? displayedDatabases.length : activeDimension === 'middleware' ? displayedMiddlewares.length : activeDimension === 'backup' ? displayedBackups.length : displayedOpsRecords.length})
            </button>

            {/* 📑 导出 v360 全套 5 个工作表 */}
            <button
              type="button"
              onClick={handleExportAllV351}
              style={{
                padding: "7px 14px",
                background: "linear-gradient(135deg,#f0fdf4,#dcfce7)",
                border: "1px solid #86efac",
                borderRadius: 8,
                color: "#166534",
                fontWeight: 700,
                fontSize: 12,
                display: "inline-flex",
                alignItems: "center",
                gap: 5,
                cursor: "pointer",
                whiteSpace: "nowrap",
                boxShadow: "0 1px 3px rgba(34,197,94,0.15)"
              }}
              title="一键导出包含 02硬件(299台)、03数据库(47)、04中间件(1)、05备份(1)、06运维(81) 全套 5 个工作表的标准 v360 Excel 工作簿"
            >
              📑 导出 v360 全套 Excel
            </button>

            {/* 🌐 IP查询 */}
            <button
              type="button"
              onClick={() => {
                setIpBatchText(ipSearchKeyword || "");
                setShowIpModal(true);
              }}
              style={{
                padding: "7px 14px",
                background: "linear-gradient(135deg,#ecfdf5,#d1fae5)",
                border: "1px solid #6ee7b7",
                borderRadius: 8,
                color: "#065f46",
                fontWeight: 600,
                fontSize: 12,
                display: "inline-flex",
                alignItems: "center",
                gap: 5,
                cursor: "pointer",
                whiteSpace: "nowrap",
                boxShadow: "0 1px 3px rgba(110,231,183,0.2)"
              }}
              title="快速检索单个或批量比对多个 IP 地址并定位所属项目与资产"
            >
              🌐 IP 地址查询
            </button>

            {/* 💾 数据库与程序调取状态徽章 */}
            <div
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: 6,
                padding: "5px 12px",
                borderRadius: 8,
                fontSize: 12,
                fontWeight: 600,
                background: isDbConnected || dataSource === "mysql" ? "rgba(16, 185, 129, 0.1)" : "rgba(59, 130, 246, 0.1)",
                border: `1px solid ${isDbConnected || dataSource === "mysql" ? "rgba(16, 185, 129, 0.3)" : "rgba(59, 130, 246, 0.3)"}`,
                color: isDbConnected || dataSource === "mysql" ? "#065f46" : "#1e40af"
              }}
              title={isDbConnected ? "当前直连 MySQL 数据库，数据由程序实时调取" : "当前通过后端程序 API 调取 v360 资产库（配置 MYSQL_HOST 环境变量可直连 MySQL）"}
            >
              <span style={{ fontSize: 10 }}>{isDbConnected || dataSource === "mysql" ? "🟢" : "🔵"}</span>
              <span>{isDbConnected || dataSource === "mysql" ? "MySQL 数据库直连" : "程序接口调取 (v360: 299台)"}</span>
              {lastSyncTime && (
                <span style={{ fontSize: 11, opacity: 0.75, borderLeft: "1px solid currentColor", paddingLeft: 6, marginLeft: 2 }}>
                  {lastSyncTime}
                </span>
              )}
            </div>

            {/* 🔄 调取数据按钮 */}
            {onRefreshApi && (
              <button
                type="button"
                onClick={onRefreshApi}
                disabled={isApiLoading}
                style={{
                  padding: "7px 12px",
                  background: "linear-gradient(135deg,#f0f9ff,#e0f2fe)",
                  border: "1px solid #7dd3fc",
                  borderRadius: 8,
                  color: "#0369a1",
                  fontWeight: 600,
                  fontSize: 12,
                  display: "inline-flex",
                  alignItems: "center",
                  gap: 5,
                  cursor: isApiLoading ? "not-allowed" : "pointer",
                  whiteSpace: "nowrap",
                  boxShadow: "0 1px 3px rgba(3,105,161,0.1)"
                }}
                title="重新通过后端程序调取最新资产数据"
              >
                <span style={{ display: "inline-block", transform: isApiLoading ? "rotate(180deg)" : "none", transition: "transform 0.5s" }}>
                  🔄
                </span>
                <span>{isApiLoading ? "调取中..." : "调取数据"}</span>
              </button>
            )}

            {/* ＋ 录入资产/数据库/中间件/备份/运维 - 动态操作 */}
            <button
              type="button"
              onClick={() => {
                if (activeDimension === "database") setShowAddDbModal(true);
                else if (activeDimension === "middleware") setShowAddMwModal(true);
                else if (activeDimension === "backup") setShowAddBkModal(true);
                else if (activeDimension === "ops") setShowAddOpsModal(true);
                else setShowAddModal(true);
              }}
              style={{
                marginLeft: "auto",
                padding: "7px 18px",
                background: activeDimension === "database" ? "linear-gradient(135deg,#4f46e5,#4338ca)" : (
                  activeDimension === "middleware" ? "linear-gradient(135deg,#d97706,#b45309)" : (
                    activeDimension === "backup" ? "linear-gradient(135deg,#059669,#047857)" : (
                      activeDimension === "ops" ? "linear-gradient(135deg,#7c3aed,#6d28d9)" : "linear-gradient(135deg,#2563eb,#1d4ed8)"
                    )
                  )
                ),
                border: "none",
                borderRadius: 8,
                color: "#fff",
                fontWeight: 700,
                fontSize: 13,
                display: "inline-flex",
                alignItems: "center",
                gap: 6,
                cursor: "pointer",
                whiteSpace: "nowrap",
                boxShadow: "0 2px 8px rgba(0,0,0,0.15)"
              }}
            >
              ＋ 录入{activeDimension === "hardware" ? "硬件资产" : activeDimension === "database" ? "数据库台账" : activeDimension === "middleware" ? "中间件服务" : activeDimension === "backup" ? "备份策略" : "运维保障"}
            </button>
          </div>
            </>
          )}
        </div>


        {/* ── 5-Dimension Architecture Tabs (Aligned with Excel v351 sheets) ── */}
        <div style={{
          display: "flex",
          gap: 6,
          background: "#fff",
          border: "1px solid #e2e8f0",
          borderRadius: 8,
          padding: "6px 10px",
          boxShadow: "0 1px 2px rgba(0,0,0,0.03)",
          flexWrap: "wrap",
          alignItems: "center"
        }}>
          <span style={{ fontSize: 11, fontWeight: 700, color: "#64748b", marginRight: 4, display: "flex", alignItems: "center", gap: 4 }}>
            <span>📑</span>
            <span>台账视窗:</span>
          </span>

          <button
            type="button"
            onClick={() => { setActiveDimension("hardware"); setCurrentPage(1); }}
            style={{
              padding: "5px 12px",
              borderRadius: 6,
              border: activeDimension === "hardware" ? "1.5px solid #2563eb" : "1px solid #e2e8f0",
              background: activeDimension === "hardware" ? "#eff6ff" : "#f8fafc",
              color: activeDimension === "hardware" ? "#1d4ed8" : "#475569",
              fontWeight: activeDimension === "hardware" ? 700 : 500,
              fontSize: 12,
              cursor: "pointer",
              display: "flex",
              alignItems: "center",
              gap: 5,
              transition: "all 0.15s"
            }}
          >
            <span>🖥️ 02-硬件设备</span>
            <span style={{ fontSize: 10, background: activeDimension === "hardware" ? "#2563eb" : "#e2e8f0", color: activeDimension === "hardware" ? "#fff" : "#475569", padding: "1px 5px", borderRadius: 10 }}>
              {displayedAssets.length}
            </span>
          </button>

          <button
            type="button"
            onClick={() => { setActiveDimension("database"); setDbPage(1); }}
            style={{
              padding: "5px 12px",
              borderRadius: 6,
              border: activeDimension === "database" ? "1.5px solid #4f46e5" : "1px solid #e2e8f0",
              background: activeDimension === "database" ? "#eef2ff" : "#f8fafc",
              color: activeDimension === "database" ? "#4338ca" : "#475569",
              fontWeight: activeDimension === "database" ? 700 : 500,
              fontSize: 12,
              cursor: "pointer",
              display: "flex",
              alignItems: "center",
              gap: 5,
              transition: "all 0.15s"
            }}
          >
            <span>🗄️ 03-数据库</span>
            <span style={{ fontSize: 10, background: activeDimension === "database" ? "#4f46e5" : "#e2e8f0", color: activeDimension === "database" ? "#fff" : "#475569", padding: "1px 5px", borderRadius: 10 }}>
              {displayedDatabases.length}
            </span>
          </button>

          <button
            type="button"
            onClick={() => { setActiveDimension("middleware"); setMwPage(1); }}
            style={{
              padding: "5px 12px",
              borderRadius: 6,
              border: activeDimension === "middleware" ? "1.5px solid #d97706" : "1px solid #e2e8f0",
              background: activeDimension === "middleware" ? "#fffbeb" : "#f8fafc",
              color: activeDimension === "middleware" ? "#b45309" : "#475569",
              fontWeight: activeDimension === "middleware" ? 700 : 500,
              fontSize: 12,
              cursor: "pointer",
              display: "flex",
              alignItems: "center",
              gap: 5,
              transition: "all 0.15s"
            }}
          >
            <span>⚙️ 04-中间件</span>
            <span style={{ fontSize: 10, background: activeDimension === "middleware" ? "#d97706" : "#e2e8f0", color: activeDimension === "middleware" ? "#fff" : "#475569", padding: "1px 5px", borderRadius: 10 }}>
              {displayedMiddlewares.length}
            </span>
          </button>

          <button
            type="button"
            onClick={() => { setActiveDimension("backup"); setBkPage(1); }}
            style={{
              padding: "5px 12px",
              borderRadius: 6,
              border: activeDimension === "backup" ? "1.5px solid #059669" : "1px solid #e2e8f0",
              background: activeDimension === "backup" ? "#ecfdf5" : "#f8fafc",
              color: activeDimension === "backup" ? "#047857" : "#475569",
              fontWeight: activeDimension === "backup" ? 700 : 500,
              fontSize: 12,
              cursor: "pointer",
              display: "flex",
              alignItems: "center",
              gap: 5,
              transition: "all 0.15s"
            }}
          >
            <span>💾 05-备份</span>
            <span style={{ fontSize: 10, background: activeDimension === "backup" ? "#059669" : "#e2e8f0", color: activeDimension === "backup" ? "#fff" : "#475569", padding: "1px 5px", borderRadius: 10 }}>
              {displayedBackups.length}
            </span>
          </button>

          <button
            type="button"
            onClick={() => { setActiveDimension("ops"); setOpsPage(1); }}
            style={{
              padding: "5px 12px",
              borderRadius: 6,
              border: activeDimension === "ops" ? "1.5px solid #7c3aed" : "1px solid #e2e8f0",
              background: activeDimension === "ops" ? "#f5f3ff" : "#f8fafc",
              color: activeDimension === "ops" ? "#6d28d9" : "#475569",
              fontWeight: activeDimension === "ops" ? 700 : 500,
              fontSize: 12,
              cursor: "pointer",
              display: "flex",
              alignItems: "center",
              gap: 5,
              transition: "all 0.15s"
            }}
          >
            <span>🛡️ 06-运维</span>
            <span style={{ fontSize: 10, background: activeDimension === "ops" ? "#7c3aed" : "#e2e8f0", color: activeDimension === "ops" ? "#fff" : "#475569", padding: "1px 5px", borderRadius: 10 }}>
              {displayedOpsRecords.length}
            </span>
          </button>
        </div>

        {/* Filter Bar & Controls */}
        <div style={{
          background: "#fff",
          border: "1px solid #e2e8f0",
          borderRadius: 8,
          padding: "8px 14px",
          display: "flex",
          flexDirection: "column",
          gap: 6
        }}>
          {/* Row 1: main search + quick filters */}
          <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
            <input
              placeholder="🔍 搜索任意字段：设备名/项目/客户/IP/系统/信创..."
              value={assetKeyword}
              onChange={e => { setAssetKeyword(e.target.value); setCurrentPage(1); }}
              style={{ flex: "1 1 220px", fontSize: 12, padding: "5px 10px", borderRadius: 6, border: "1.5px solid #93c5fd", outline: "none" }}
            />

            {/* IP search */}
            <div style={{ display: "flex", alignItems: "center", gap: 3 }}>
              <input
                placeholder="IP地址 (业务/内大网/VIP)..."
                value={ipSearchInput}
                onChange={e => setIpSearchInput(e.target.value)}
                onKeyDown={e => {
                  if (e.key === "Enter") {
                    setIpSearchKeyword(ipSearchInput.trim());
                    setCurrentPage(1);
                  }
                }}
                style={{ width: 190, fontSize: 12, padding: "5px 8px" }}
              />
              <button
                type="button"
                className="btn-primary"
                onClick={() => { setIpSearchKeyword(ipSearchInput.trim()); setCurrentPage(1); }}
                style={{ fontSize: 11, padding: "5px 9px", display: "inline-flex", alignItems: "center", gap: 3, cursor: "pointer", whiteSpace: "nowrap" }}
                title="按输入的目标 IP 地址进行精准查询与过滤"
              >
                <span>🔍</span><span>查IP</span>
              </button>
              {ipSearchKeyword && (
                <button
                  type="button"
                  className="btn-secondary"
                  onClick={() => { setIpSearchInput(""); setIpSearchKeyword(""); setCurrentPage(1); }}
                  style={{ fontSize: 11, padding: "4px 6px", color: "#64748b" }}
                  title="清除当前 IP 查询"
                >✕</button>
              )}
            </div>

            <select
              value={deviceTypeFilter}
              onChange={e => { setDeviceTypeFilter(e.target.value); setCurrentPage(1); }}
              style={{ fontSize: 12, padding: "5px 8px" }}
            >
              <option value="全部">全部设备形态</option>
              <option value="虚拟机">云主机 / 虚拟机</option>
              <option value="物理机">实体物理服务器</option>
              <option value="存储">存储硬件 / MinIO</option>
              <option value="负载均衡">负载均衡 / SLB</option>
            </select>

            <select
              value={xinchuangFilter}
              onChange={e => { setXinchuangFilter(e.target.value); setCurrentPage(1); }}
              style={{ fontSize: 12, padding: "5px 8px" }}
            >
              <option value="全部">全部操作系统</option>
              <option value="是">国产信创 OS (🛡️麒麟/统信)</option>
              <option value="否">常规 OS (CentOS/RedHat)</option>
            </select>

            {/* Advanced filter toggle */}
            <button
              type="button"
              onClick={() => setShowAdvFilter(!showAdvFilter)}
              style={{
                fontSize: 11, padding: "5px 9px",
                background: showAdvFilter ? "#eff6ff" : "#f8fafc",
                border: showAdvFilter ? "1.5px solid #93c5fd" : "1px solid #e2e8f0",
                borderRadius: 6, color: showAdvFilter ? "#1e40af" : "#64748b",
                fontWeight: 600, cursor: "pointer",
                display: "inline-flex", alignItems: "center", gap: 4,
                whiteSpace: "nowrap"
              }}
              title="展开多条件高级筛选"
            >
              ⚙ 高级筛选 {(filterEnv !== "全部" || filterCloud !== "全部" || filterRegion !== "全部" || filterOsFamily !== "全部") && <span style={{ background: "#2563eb", color: "#fff", borderRadius: 8, fontSize: 10, padding: "0 4px" }}>●</span>}
            </button>

            {(assetKeyword || ipSearchKeyword || deviceTypeFilter !== "全部" || xinchuangFilter !== "全部" || filterEnv !== "全部" || filterCloud !== "全部" || filterRegion !== "全部" || filterOsFamily !== "全部") && (
              <button
                className="btn-secondary"
                style={{ fontSize: 11, padding: "3px 8px" }}
                onClick={() => {
                  setAssetKeyword(""); setIpSearchInput(""); setIpSearchKeyword("");
                  setDeviceTypeFilter("全部"); setXinchuangFilter("全部");
                  setFilterEnv("全部"); setFilterCloud("全部"); setFilterRegion("全部"); setFilterOsFamily("全部");
                  setCurrentPage(1);
                }}
              >
                ✕ 重置全部筛选
              </button>
            )}
          </div>

          {/* Row 2: advanced filter panel */}
          {showAdvFilter && (
            <div style={{
              display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center",
              paddingTop: 6, borderTop: "1px dashed #e2e8f0"
            }}>
              <span style={{ fontSize: 11, color: "#64748b", fontWeight: 600, whiteSpace: "nowrap" }}>多条件筛选：</span>
              <select value={filterEnv} onChange={e => { setFilterEnv(e.target.value); setCurrentPage(1); }} style={{ fontSize: 11, padding: "3px 7px" }}>
                <option value="全部">全部环境</option>
                <option value="生产">生产环境</option>
                <option value="测试">测试环境</option>
                <option value="开发">开发环境</option>
              </select>
              <select value={filterCloud} onChange={e => { setFilterCloud(e.target.value); setCurrentPage(1); }} style={{ fontSize: 11, padding: "3px 7px" }}>
                <option value="全部">全部云商</option>
                {Array.from(new Set(allAssets.map(a => a.cloudVendor).filter(Boolean))).map(v => (
                  <option key={v} value={v}>{v}</option>
                ))}
              </select>
              <select value={filterRegion} onChange={e => { setFilterRegion(e.target.value); setCurrentPage(1); }} style={{ fontSize: 11, padding: "3px 7px" }}>
                <option value="全部">全部区域</option>
                {Array.from(new Set(allAssets.map(a => a.regionName).filter(Boolean))).map(v => (
                  <option key={v} value={v}>{v}</option>
                ))}
              </select>
              <select value={filterOsFamily} onChange={e => { setFilterOsFamily(e.target.value); setCurrentPage(1); }} style={{ fontSize: 11, padding: "3px 7px" }}>
                <option value="全部">全部OS系列</option>
                {Array.from(new Set(allAssets.map(a => a.osFamily).filter(Boolean))).map(v => (
                  <option key={v} value={v}>{v}</option>
                ))}
              </select>
              <span style={{ fontSize: 11, color: "#94a3b8" }}>· 以上条件可叠加组合</span>
            </div>
          )}

          {/* Row 3: result count + export */}
          <div style={{ display: "flex", gap: 10, alignItems: "center", justifyContent: "space-between" }}>
            <span style={{ fontSize: 12, color: "#64748b" }}>
              共 <strong style={{ color: "#0f172a" }}>{displayedAssets.length}</strong> 台资产记录
            </span>
            <button
              type="button"
              className="btn-secondary"
              onClick={handleExportExcel}
              style={{ fontSize: 11, padding: "3px 8px", background: "#f0fdf4", borderColor: "#bbf7d0", color: "#15803d", fontWeight: 600, display: "inline-flex", alignItems: "center", gap: 4 }}
              title={`导出当前筛选出的 ${displayedAssets.length} 台设备为《02-硬件设备》Excel`}
            >
              <span>📤</span>
              <span>导出当前查询 ({displayedAssets.length}台)</span>
            </button>
          </div>
        </div>

        {/* 💡 跨项目 IP 智能发现与定位卡片 */}
        {crossProjectIpMatch && (
          <div style={{
            background: "#eff6ff",
            border: "1.5px solid #bfdbfe",
            borderRadius: 8,
            padding: "10px 16px",
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            boxShadow: "0 2px 6px rgba(37, 99, 235, 0.08)",
            animation: "fadeIn 0.25s ease"
          }}>
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <span style={{ fontSize: 20 }}>💡</span>
              <div>
                <strong style={{ fontSize: 13, color: "#1e40af" }}>
                  在当前选定项目【{currentProject?.name}】未找到 IP【{ipSearchKeyword}】，但已在其他项目中定位到 {crossProjectIpMatch.count} 台匹配设备！
                </strong>
                <div style={{ fontSize: 11, color: "#3b82f6", marginTop: 4, display: "flex", alignItems: "center", flexWrap: "wrap", gap: 6 }}>
                  <span>涉及项目: {crossProjectIpMatch.projects.join("、")} · 匹配资产:</span>
                  {crossProjectIpMatch.matchedAssets.map(a => (
                    <button
                      key={a.id}
                      type="button"
                      onClick={() => {
                        const targetProj = projects.find(p => p.name === a.projectName);
                        if (targetProj) setSelectedProjectId(targetProj.id);
                        setDetailAsset(a);
                        setDetailTab("spec");
                      }}
                      style={{
                        background: "#fff",
                        border: "1px solid #93c5fd",
                        borderRadius: 4,
                        padding: "1px 6px",
                        fontSize: 11,
                        color: "#1d4ed8",
                        cursor: "pointer",
                        fontWeight: 600,
                        display: "inline-flex",
                        alignItems: "center",
                        gap: 3
                      }}
                      title="点击直接打开该设备的详细信息档案"
                    >
                      <span>📖</span>
                      <span>{a.name} ({a.privateIp || a.ip})</span>
                    </button>
                  ))}
                </div>
              </div>
            </div>
            <div style={{ display: "flex", gap: 6 }}>
              {crossProjectIpMatch.targetProject && (
                <button
                  type="button"
                  className="btn-primary"
                  onClick={() => {
                    if (crossProjectIpMatch.targetProject) {
                      setSelectedProjectId(crossProjectIpMatch.targetProject.id);
                    }
                  }}
                  style={{ fontSize: 11, padding: "5px 12px" }}
                >
                  📍 切换至【{crossProjectIpMatch.targetProject.name}】查看
                </button>
              )}
              <button
                type="button"
                className="btn-secondary"
                onClick={() => setSelectedProjectId("all")}
                style={{ fontSize: 11, padding: "5px 10px", background: "#fff" }}
              >
                🌟 切换到全量项目总览
              </button>
            </div>
          </div>
        )}

        {/* Assets Table Container */}
        <div style={{
          flex: 1,
          background: "#fff",
          border: "1px solid #e2e8f0",
          borderRadius: 8,
          overflow: "hidden",
          display: "flex",
          flexDirection: "column"
        }}>
          <div style={{ flex: 1, overflowX: "auto", overflowY: "auto" }}>
            {/* ── 02-硬件设备 表格 ── */}
            {activeDimension === "hardware" && (
              <table className="cmdb-data-table" style={{ minWidth: 1200 }}>
                <thead>
                  <tr>
                    <th style={{ width: 45 }}>序号</th>
                    <th style={{ width: 220 }}>设备名称 / 形态</th>
                    <th style={{ width: 170 }}>所属项目 · 客户单位</th>
                    <th style={{ width: 140 }}>环境 · 云厂商 · 区域</th>
                    <th style={{ width: 160 }}>私有业务 IP / 内大网</th>
                    <th style={{ width: 120 }}>VIP / EIP</th>
                    <th style={{ width: 150 }}>算力规格 (CPU/内存/磁盘)</th>
                    <th style={{ width: 150 }}>操作系统 / 信创</th>
                    <th style={{ width: 70 }}>远程端口</th>
                    <th style={{ width: 160 }}>备注说明</th>
                    <th style={{ width: 100, textAlign: "center" }}>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {paginatedAssets.length === 0 ? (
                    <tr>
                      <td colSpan={11} style={{ textAlign: "center", padding: "36px 0", color: "#94a3b8" }}>
                        当前项目下暂无符合筛选条件的资产记录
                      </td>
                    </tr>
                  ) : (
                    paginatedAssets.map(item => {
                      const isPhysical = item._kind === "physical" || item.deviceType === "物理机";

                      return (
                        <tr key={item.id} style={{ fontSize: 12 }}>
                          <td style={{ fontFamily: "monospace", color: "#64748b", fontWeight: 600 }}>
                            {item.seq || "-"}
                          </td>
                          <td>
                            <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                              <span style={{ 
                                fontSize: 10, 
                                padding: "1px 5px", 
                                borderRadius: 3, 
                                fontWeight: 600,
                                background: isPhysical ? "#fef3c7" : "#e0f2fe", 
                                color: isPhysical ? "#b45309" : "#0369a1" 
                              }}>
                                {item.deviceType || (isPhysical ? "物理机" : "虚拟机")}
                              </span>
                              <strong 
                                style={{ color: "#1d4ed8", cursor: "pointer", textDecoration: "underline", textUnderlineOffset: 3 }}
                                title="点击进入该设备的详细信息档案 (规格/数据库/中间件/备份/运维)"
                                onClick={() => { setDetailAsset(item); setDetailTab("spec"); }}
                              >
                                {item.name}
                              </strong>
                            </div>
                            {isPhysical && (item as any).model && (
                              <small style={{ color: "#64748b", display: "block", marginTop: 2 }}>
                                {(item as any).brand} {(item as any).model} · 码: {(item as any).assetNo}
                              </small>
                            )}
                            {/* 微徽章: 关联数据库与中间件 */}
                            {(() => {
                              const ip = item.privateIp || item.ip;
                              const dCount = databases.filter(d => (ip && d.privateIp === ip) || d.projectName === item.projectName).length;
                              const mCount = middlewares.filter(m => (ip && m.privateIp === ip) || m.projectName === item.projectName).length;
                              if (dCount === 0 && mCount === 0) return null;
                              return (
                                <div style={{ display: "flex", gap: 4, marginTop: 4, flexWrap: "wrap" }}>
                                  {dCount > 0 && (
                                    <span 
                                      style={{ fontSize: 9, background: "#e0e7ff", color: "#3730a3", padding: "1px 4px", borderRadius: 3, cursor: "pointer", fontWeight: 600 }}
                                      onClick={(e) => { e.stopPropagation(); setDetailAsset(item); setDetailTab("database"); }}
                                      title="查看挂载数据库"
                                    >
                                      🗄️ {dCount}个库
                                    </span>
                                  )}
                                  {mCount > 0 && (
                                    <span 
                                      style={{ fontSize: 9, background: "#fef3c7", color: "#92400e", padding: "1px 4px", borderRadius: 3, cursor: "pointer", fontWeight: 600 }}
                                      onClick={(e) => { e.stopPropagation(); setDetailAsset(item); setDetailTab("middleware"); }}
                                      title="查看运行中间件"
                                    >
                                      ⚙️ {mCount}个中间件
                                    </span>
                                  )}
                                </div>
                              );
                            })()}
                          </td>
                          <td>
                            <div style={{ fontWeight: 600, color: "#1e293b" }}>{item.projectName}</div>
                            <small style={{ color: "#64748b", display: "block" }}>{item.customerName}</small>
                          </td>
                          <td>
                            <div style={{ display: "flex", gap: 4, alignItems: "center" }}>
                              <span style={{
                                padding: "1px 5px",
                                borderRadius: 3,
                                fontSize: 10,
                                background: item.env === "生产" ? "#dcfce7" : "#f1f5f9",
                                color: item.env === "生产" ? "#15803d" : "#475569"
                              }}>
                                {item.env}
                              </span>
                              <span style={{ fontSize: 11, color: "#475569" }}>{item.cloudVendor}</span>
                            </div>
                            <small style={{ color: "#64748b", display: "block" }}>{item.regionName}</small>
                          </td>
                          <td>
                            <div style={{ fontFamily: "monospace", fontWeight: 600, color: "#0f172a" }}>
                              {item.privateIp || item.ip || "-"}
                            </div>
                            {item.internalWanIp && (
                              <small style={{ fontFamily: "monospace", color: "#64748b", display: "block" }}>
                                内大: {item.internalWanIp}
                              </small>
                            )}
                          </td>
                          <td>
                            {item.vip ? (
                              <span style={{ fontFamily: "monospace", fontSize: 11, background: "#fef3c7", color: "#92400e", padding: "1px 4px", borderRadius: 3, fontWeight: 600 }}>
                                VIP: {item.vip}
                              </span>
                            ) : item.eip ? (
                              <span style={{ fontFamily: "monospace", fontSize: 11, background: "#eff6ff", color: "#1e40af", padding: "1px 4px", borderRadius: 3, fontWeight: 600 }}>
                                EIP: {item.eip}
                              </span>
                            ) : (
                              <span style={{ color: "#cbd5e1" }}>-</span>
                            )}
                          </td>
                          <td>
                            <div style={{ fontSize: 11, color: "#1e293b" }}>
                              <strong>{item.cpuCores || 4}核</strong> · {item.memoryGb || 16}GB
                            </div>
                            <small style={{ color: "#64748b" }}>
                              盘: {(item.systemDiskGb || 0) + (item.dataDiskGb || 0)}GB
                            </small>
                          </td>
                          <td>
                            <div style={{ display: "flex", alignItems: "center", gap: 4 }}>
                              <span style={{ 
                                fontSize: 10, 
                                padding: "1px 5px", 
                                borderRadius: 3, 
                                fontWeight: 700,
                                background: item.isXinchuang === "是" ? "#fee2e2" : "#f1f5f9",
                                color: item.isXinchuang === "是" ? "#dc2626" : "#475569"
                              }}>
                                {item.isXinchuang === "是" ? "🛡️信创" : "常规"}
                              </span>
                              <span style={{ fontSize: 11, color: "#334155" }} title={item.osVersion || ""}>
                                {item.osFamily || "Linux"}
                              </span>
                            </div>
                          </td>
                          <td style={{ fontFamily: "monospace", color: "#d97706", fontWeight: 600 }}>
                            {item.remotePort || 22}
                          </td>
                          <td>
                            <span style={{ 
                              fontSize: 11, 
                              color: "#64748b", 
                              display: "-webkit-box", 
                              WebkitLineClamp: 2, 
                              WebkitBoxOrient: "vertical", 
                              overflow: "hidden" 
                            }}>
                              {item.remarks || "-"}
                            </span>
                          </td>
                          <td style={{ textAlign: "center" }}>
                            <div style={{ display: "flex", justifyContent: "center", gap: 4 }}>
                              <button 
                                className="btn-secondary" 
                                style={{ padding: "2px 6px", fontSize: 11, color: "#1d4ed8", borderColor: "#bfdbfe", background: "#eff6ff", fontWeight: 600 }}
                                onClick={() => { setDetailAsset(item); setDetailTab("spec"); }}
                                title="查看该设备的详细信息档案"
                              >
                                📖 详情
                              </button>
                              <button 
                                className="btn-secondary" 
                                style={{ padding: "2px 6px", fontSize: 11, color: "#b45309", borderColor: "#fde68a", background: "#fffbeb" }}
                                onClick={() => openEditModal(item)}
                                title="修改该资产配置"
                              >
                                ✏️修改
                              </button>
                              <button 
                                className="btn-secondary" 
                                style={{ padding: "2px 6px", fontSize: 11, color: "#dc2626", borderColor: "#fecaca", background: "#fef2f2" }}
                                onClick={() => setDeletingAsset(item)}
                                title="注销删除该设备"
                              >
                                🗑️删除
                              </button>
                            </div>
                          </td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
            )}

            {/* ── 03-数据库 台账表格 (Sheet 03) ── */}
            {activeDimension === "database" && (
              <table className="cmdb-data-table" style={{ minWidth: 1200 }}>
                <thead>
                  <tr style={{ background: "#f8fafc" }}>
                    <th style={{ width: 45 }}>序号</th>
                    <th style={{ width: 170 }}>所属项目 · 客户单位</th>
                    <th style={{ width: 140 }}>数据库大类 · 软件</th>
                    <th style={{ width: 90 }}>版本</th>
                    <th style={{ width: 70 }}>端口</th>
                    <th style={{ width: 120 }}>实例名 / SID</th>
                    <th style={{ width: 130 }}>业务库名</th>
                    <th style={{ width: 90 }}>部署模式</th>
                    <th style={{ width: 140 }}>集群名称</th>
                    <th style={{ width: 140 }}>私有业务IP</th>
                    <th style={{ width: 120 }}>VIP / EIP</th>
                    <th style={{ width: 70 }}>状态</th>
                    <th style={{ width: 150 }}>备注说明</th>
                    <th style={{ width: 80, textAlign: "center" }}>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {paginatedDatabases.length === 0 ? (
                    <tr>
                      <td colSpan={14} style={{ textAlign: "center", padding: "36px 0", color: "#94a3b8" }}>
                        当前项目下暂无符合筛选条件的数据库台账记录
                      </td>
                    </tr>
                  ) : (
                    paginatedDatabases.map(db => (
                      <tr key={db.id} style={{ fontSize: 12 }}>
                        <td style={{ fontFamily: "monospace", color: "#64748b", fontWeight: 600 }}>
                          {db.seq || "-"}
                        </td>
                        <td>
                          <div style={{ fontWeight: 600, color: "#1e293b" }}>{db.projectName}</div>
                          <small style={{ color: "#64748b", display: "block" }}>{db.customerName}</small>
                        </td>
                        <td>
                          <div style={{ fontWeight: 700, color: "#4338ca" }}>{db.dbSoftware || db.type}</div>
                          <small style={{ color: "#64748b", fontSize: 10 }}>{db.dbCategory || "关系型"}</small>
                        </td>
                        <td>
                          <code style={{ background: "#f1f5f9", padding: "1px 4px", borderRadius: 3, fontSize: 11, color: "#2563eb", fontWeight: 600 }}>
                            {db.version || "-"}
                          </code>
                        </td>
                        <td style={{ fontFamily: "monospace", color: "#d97706", fontWeight: 700 }}>
                          {db.port}
                        </td>
                        <td style={{ fontFamily: "monospace", fontWeight: 600, color: "#0f172a" }}>
                          {db.instanceSid || "-"}
                        </td>
                        <td style={{ color: "#0369a1", fontWeight: 600 }}>
                          {db.dbName || "-"}
                        </td>
                        <td>
                          <span style={{
                            padding: "1px 6px",
                            borderRadius: 4,
                            fontSize: 10,
                            fontWeight: 700,
                            background: db.deployMode === "集群" ? "#e0e7ff" : (db.deployMode === "分布式" ? "#fef3c7" : "#f1f5f9"),
                            color: db.deployMode === "集群" ? "#3730a3" : (db.deployMode === "分布式" ? "#92400e" : "#475569")
                          }}>
                            {db.deployMode || "单机"}
                          </span>
                        </td>
                        <td style={{ fontFamily: "monospace", fontSize: 11, color: "#475569" }}>
                          {db.clusterName || "-"}
                        </td>
                        <td style={{ fontFamily: "monospace", fontWeight: 600, color: "#0f172a" }}>
                          {db.privateIp || db.hostIp}
                        </td>
                        <td style={{ fontFamily: "monospace", fontSize: 11, color: "#d97706" }}>
                          {db.vipEip || "-"}
                        </td>
                        <td>
                          <span style={{ color: "#16a34a", fontWeight: 700, fontSize: 11 }}>
                            ● {db.status === "active" ? "在线" : db.status}
                          </span>
                        </td>
                        <td style={{ color: "#64748b", fontSize: 11 }}>
                          {db.remarks || "-"}
                        </td>
                        <td style={{ textAlign: "center" }}>
                          <button
                            type="button"
                            className="btn-secondary"
                            style={{ padding: "2px 6px", fontSize: 11, color: "#dc2626", borderColor: "#fecaca", background: "#fef2f2" }}
                            onClick={() => onDeleteDatabase?.(db.id)}
                            title="删除该数据库台账"
                          >
                            🗑️
                          </button>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            )}

            {/* ── 04-中间件 服务表格 (Sheet 04) ── */}
            {activeDimension === "middleware" && (
              <table className="cmdb-data-table" style={{ minWidth: 1200 }}>
                <thead>
                  <tr style={{ background: "#f8fafc" }}>
                    <th style={{ width: 45 }}>序号</th>
                    <th style={{ width: 170 }}>所属项目 · 客户单位</th>
                    <th style={{ width: 160 }}>中间件类型</th>
                    <th style={{ width: 160 }}>中间件软件名称</th>
                    <th style={{ width: 100 }}>软件版本</th>
                    <th style={{ width: 100 }}>服务端口</th>
                    <th style={{ width: 110 }}>角色 / 运行时</th>
                    <th style={{ width: 140 }}>私有业务 IP</th>
                    <th style={{ width: 110 }}>环境 · 云商</th>
                    <th style={{ width: 70 }}>运行状态</th>
                    <th style={{ width: 150 }}>备注说明</th>
                    <th style={{ width: 80, textAlign: "center" }}>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {paginatedMiddlewares.length === 0 ? (
                    <tr>
                      <td colSpan={12} style={{ textAlign: "center", padding: "36px 0", color: "#94a3b8" }}>
                        当前项目下暂无符合筛选条件的中间件服务记录
                      </td>
                    </tr>
                  ) : (
                    paginatedMiddlewares.map(mw => (
                      <tr key={mw.id} style={{ fontSize: 12 }}>
                        <td style={{ fontFamily: "monospace", color: "#64748b", fontWeight: 600 }}>
                          {mw.seq || "-"}
                        </td>
                        <td>
                          <div style={{ fontWeight: 600, color: "#1e293b" }}>{mw.projectName}</div>
                          <small style={{ color: "#64748b", display: "block" }}>{mw.customerName}</small>
                        </td>
                        <td>
                          <span style={{ background: "#fef3c7", color: "#92400e", padding: "1px 6px", borderRadius: 4, fontSize: 10, fontWeight: 600 }}>
                            {mw.mwType}
                          </span>
                        </td>
                        <td style={{ fontWeight: 700, color: "#1e293b" }}>
                          {mw.mwSoftware || mw.name}
                        </td>
                        <td>
                          <code style={{ background: "#f1f5f9", padding: "1px 4px", borderRadius: 3, fontSize: 11, color: "#2563eb", fontWeight: 600 }}>
                            {mw.version || "-"}
                          </code>
                        </td>
                        <td style={{ fontFamily: "monospace", color: "#d97706", fontWeight: 700 }}>
                          {mw.port || "-"}
                        </td>
                        <td>
                          <div style={{ fontSize: 11, color: "#475569" }}>{mw.role || "-"}</div>
                          <small style={{ color: "#0d9488", fontWeight: 600 }}>{mw.runtime || ""}</small>
                        </td>
                        <td style={{ fontFamily: "monospace", fontWeight: 600, color: "#0f172a" }}>
                          {mw.privateIp}
                        </td>
                        <td>
                          <span style={{ fontSize: 10, padding: "1px 4px", borderRadius: 3, background: mw.env === "生产" ? "#dcfce7" : "#f1f5f9", color: mw.env === "生产" ? "#15803d" : "#475569" }}>
                            {mw.env}
                          </span>{" "}
                          <span style={{ fontSize: 11, color: "#64748b" }}>{mw.cloudVendor}</span>
                        </td>
                        <td>
                          <span style={{ color: mw.status === "running" ? "#16a34a" : "#dc2626", fontWeight: 700, fontSize: 11 }}>
                            ● {mw.status === "running" ? "运行中" : "已停止"}
                          </span>
                        </td>
                        <td style={{ color: "#64748b", fontSize: 11 }}>
                          {mw.remarks || "-"}
                        </td>
                        <td style={{ textAlign: "center" }}>
                          <button
                            type="button"
                            className="btn-secondary"
                            style={{ padding: "2px 6px", fontSize: 11, color: "#dc2626", borderColor: "#fecaca", background: "#fef2f2" }}
                            onClick={() => onDeleteMiddleware?.(mw.id)}
                            title="删除该中间件"
                          >
                            🗑️
                          </button>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            )}

            {/* ── 05-备份 策略表格 (Sheet 05) ── */}
            {activeDimension === "backup" && (
              <table className="cmdb-data-table" style={{ minWidth: 1200 }}>
                <thead>
                  <tr style={{ background: "#f8fafc" }}>
                    <th style={{ width: 45 }}>序号</th>
                    <th style={{ width: 170 }}>所属项目 · 客户单位</th>
                    <th style={{ width: 140 }}>私有业务 IP</th>
                    <th style={{ width: 90 }}>备份类型</th>
                    <th style={{ width: 100 }}>备份方式</th>
                    <th style={{ width: 140 }}>备份策略</th>
                    <th style={{ width: 220 }}>备份存储位置</th>
                    <th style={{ width: 80 }}>保留天数</th>
                    <th style={{ width: 130 }}>上次执行时间</th>
                    <th style={{ width: 80 }}>状态</th>
                    <th style={{ width: 150 }}>备注说明</th>
                    <th style={{ width: 80, textAlign: "center" }}>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {paginatedBackups.length === 0 ? (
                    <tr>
                      <td colSpan={12} style={{ textAlign: "center", padding: "36px 0", color: "#94a3b8" }}>
                        当前项目下暂无符合筛选条件的备份策略记录
                      </td>
                    </tr>
                  ) : (
                    paginatedBackups.map(bk => (
                      <tr key={bk.id} style={{ fontSize: 12 }}>
                        <td style={{ fontFamily: "monospace", color: "#64748b", fontWeight: 600 }}>
                          {bk.seq || "-"}
                        </td>
                        <td>
                          <div style={{ fontWeight: 600, color: "#1e293b" }}>{bk.projectName}</div>
                          <small style={{ color: "#64748b", display: "block" }}>{bk.customerName}</small>
                        </td>
                        <td style={{ fontFamily: "monospace", fontWeight: 600, color: "#0f172a" }}>
                          {bk.privateIp}
                        </td>
                        <td>
                          <span style={{ 
                            background: bk.backupType === "数据库" ? "#e0e7ff" : (bk.backupType === "对象存储" ? "#fef3c7" : "#dcfce7"),
                            color: bk.backupType === "数据库" ? "#3730a3" : (bk.backupType === "对象存储" ? "#92400e" : "#15803d"),
                            padding: "1px 6px", borderRadius: 4, fontSize: 10, fontWeight: 700 
                          }}>
                            {bk.backupType}
                          </span>
                        </td>
                        <td style={{ color: "#334155", fontWeight: 600 }}>
                          {bk.backupMethod}
                        </td>
                        <td>
                          <span style={{ background: "#ecfdf5", color: "#047857", border: "1px solid #a7f3d0", padding: "1px 6px", borderRadius: 4, fontSize: 11, fontWeight: 700 }}>
                            {bk.backupPolicy}
                          </span>
                        </td>
                        <td style={{ color: "#475569", fontSize: 11 }}>
                          {bk.storageLocation}
                        </td>
                        <td style={{ fontFamily: "monospace", color: "#2563eb", fontWeight: 700 }}>
                          {bk.retentionDays ? `${bk.retentionDays}天` : "-"}
                        </td>
                        <td style={{ fontFamily: "monospace", color: "#64748b", fontSize: 11 }}>
                          {bk.lastBackupTime || "-"}
                        </td>
                        <td>
                          <span style={{ color: "#16a34a", fontWeight: 700, fontSize: 11 }}>
                            ● 正常
                          </span>
                        </td>
                        <td style={{ color: "#64748b", fontSize: 11 }}>
                          {bk.remarks || "-"}
                        </td>
                        <td style={{ textAlign: "center" }}>
                          <button
                            type="button"
                            className="btn-secondary"
                            style={{ padding: "2px 6px", fontSize: 11, color: "#dc2626", borderColor: "#fecaca", background: "#fef2f2" }}
                            onClick={() => onDeleteBackup?.(bk.id)}
                            title="删除该备份策略"
                          >
                            🗑️
                          </button>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            )}

            {/* ── 06-运维 保障表格 (Sheet 06) ── */}
            {activeDimension === "ops" && (
              <table className="cmdb-data-table" style={{ minWidth: 1200 }}>
                <thead>
                  <tr style={{ background: "#f8fafc" }}>
                    <th style={{ width: 45 }}>序号</th>
                    <th style={{ width: 170 }}>所属项目 · 客户单位</th>
                    <th style={{ width: 130 }}>私有业务 IP</th>
                    <th style={{ width: 140 }}>人员归属 · 环境</th>
                    <th style={{ width: 150 }}>VPN接入地址</th>
                    <th style={{ width: 140 }}>VPN账号 · 使用人</th>
                    <th style={{ width: 170 }}>堡垒机(JumpServer)</th>
                    <th style={{ width: 140 }}>堡垒机账号 · 使用人</th>
                    <th style={{ width: 220 }}>访问服务器地址 / 命令</th>
                    <th style={{ width: 80 }}>监控覆盖</th>
                    <th style={{ width: 70 }}>巡检周期</th>
                    <th style={{ width: 70 }}>变更窗口</th>
                    <th style={{ width: 110 }}>网络分区 · 暴露</th>
                    <th style={{ width: 80, textAlign: "center" }}>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {paginatedOpsRecords.length === 0 ? (
                    <tr>
                      <td colSpan={14} style={{ textAlign: "center", padding: "36px 0", color: "#94a3b8" }}>
                        当前项目下暂无符合筛选条件的运维保障记录
                      </td>
                    </tr>
                  ) : (
                    paginatedOpsRecords.map(op => (
                      <tr key={op.id} style={{ fontSize: 12 }}>
                        <td style={{ fontFamily: "monospace", color: "#64748b", fontWeight: 600 }}>
                          {op.seq || "-"}
                        </td>
                        <td>
                          <div style={{ fontWeight: 600, color: "#1e293b" }}>{op.projectName}</div>
                          <small style={{ color: "#64748b", display: "block" }}>{op.customerName}</small>
                        </td>
                        <td style={{ fontFamily: "monospace", fontWeight: 600, color: "#0f172a" }}>
                          {op.privateIp}
                        </td>
                        <td>
                          <div style={{ fontWeight: 600, color: "#1e293b" }}>{op.personnelAffiliation || op.opsVendor}</div>
                          <small style={{ color: "#64748b", display: "block" }}>{op.vpnNetworkEnv || "互联网区"}</small>
                        </td>
                        <td style={{ fontFamily: "monospace", color: "#2563eb", fontSize: 11 }}>
                          {op.vpnAddress || "-"}
                        </td>
                        <td style={{ fontSize: 11 }}>
                          <div style={{ fontFamily: "monospace", fontWeight: 600, color: "#1e293b" }}>
                            {op.vpnAccount || "-"}
                          </div>
                          {op.vpnUserName ? (
                            <div style={{ marginTop: 2, display: "inline-flex", alignItems: "center", gap: 3, background: "#e0f2fe", color: "#0369a1", padding: "1px 6px", borderRadius: 4, fontSize: 11, fontWeight: 700 }}>
                              👤 {op.vpnUserName}
                            </div>
                          ) : (
                            <div style={{ color: "#94a3b8", fontSize: 10, marginTop: 1 }}>未登记使用人</div>
                          )}
                        </td>
                        <td>
                          <a 
                            href={op.bastionAddress?.startsWith("http") ? op.bastionAddress : `https://${op.bastionAddress}`}
                            target="_blank" 
                            rel="noreferrer"
                            style={{ color: "#7c3aed", fontWeight: 600, fontSize: 11, textDecoration: "underline" }}
                          >
                            {op.bastionAddress || "-"}
                          </a>
                        </td>
                        <td style={{ fontSize: 11 }}>
                          <div style={{ fontFamily: "monospace", fontWeight: 600, color: "#1e293b" }}>
                            {op.bastionAccount || "-"}
                          </div>
                          {op.bastionUserName ? (
                            <div style={{ marginTop: 2, display: "inline-flex", alignItems: "center", gap: 3, background: "#ede9fe", color: "#6d28d9", padding: "1px 6px", borderRadius: 4, fontSize: 11, fontWeight: 700 }}>
                              👤 {op.bastionUserName}
                            </div>
                          ) : (
                            <div style={{ color: "#94a3b8", fontSize: 10, marginTop: 1 }}>未登记使用人</div>
                          )}
                        </td>
                        <td>
                          <div style={{ display: "flex", alignItems: "center", gap: 4 }}>
                            <code style={{ background: "#f1f5f9", padding: "2px 5px", borderRadius: 4, fontSize: 11, color: "#0f172a", fontFamily: "monospace" }}>
                              {op.serverAccessAddress || `ssh root@${op.privateIp}`}
                            </code>
                            <button 
                              type="button"
                              className="btn-secondary"
                              style={{ padding: "1px 5px", fontSize: 10 }}
                              onClick={() => copyToClipboard(op.serverAccessAddress || `ssh root@${op.privateIp}`, "SSH命令")}
                            >
                              复制
                            </button>
                          </div>
                        </td>
                        <td>
                          <span style={{ color: op.monitoringCoverage === "是" ? "#16a34a" : "#ca8a04", fontWeight: 700, fontSize: 11 }}>
                            {op.monitoringCoverage}
                          </span>
                        </td>
                        <td>
                          <span style={{ background: "#f1f5f9", color: "#475569", padding: "1px 5px", borderRadius: 3, fontSize: 11 }}>
                            {op.inspectionCycle}
                          </span>
                        </td>
                        <td>
                          <span style={{ color: "#b45309", fontWeight: 600, fontSize: 11 }}>
                            {op.changeWindow}
                          </span>
                        </td>
                        <td>
                          <div style={{ fontSize: 10, color: "#334155" }}>{op.networkZone}</div>
                          <small style={{ color: op.exposureSurface === "公网" ? "#dc2626" : "#16a34a", fontWeight: 600 }}>
                            {op.exposureSurface}
                          </small>
                        </td>
                        <td style={{ textAlign: "center" }}>
                          <button
                            type="button"
                            className="btn-secondary"
                            style={{ padding: "2px 6px", fontSize: 11, color: "#dc2626", borderColor: "#fecaca", background: "#fef2f2" }}
                            onClick={() => onDeleteOpsRecord?.(op.id)}
                            title="删除该运维保障记录"
                          >
                            🗑️
                          </button>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            )}
          </div>

          {/* Pagination Footer (Aligned with 5 dimensions) */}
          <div style={{ 
            display: "flex", 
            justifyContent: "space-between", 
            alignItems: "center", 
            padding: "8px 16px", 
            background: "#f8fafc", 
            borderTop: "1px solid #e2e8f0",
            fontSize: 12,
            color: "#475569"
          }}>
            <div>
              {currentProject ? `【${currentProject.name}】` : "全量项目总览"}
              <span style={{ 
                margin: "0 6px", 
                padding: "2px 8px", 
                borderRadius: 4, 
                fontSize: 11, 
                fontWeight: 600,
                background: activeDimension === "hardware" ? "#eff6ff" : (
                  activeDimension === "database" ? "#eef2ff" : (
                    activeDimension === "middleware" ? "#fffbeb" : (
                      activeDimension === "backup" ? "#ecfdf5" : "#f5f3ff"
                    )
                  )
                ),
                color: activeDimension === "hardware" ? "#1d4ed8" : (
                  activeDimension === "database" ? "#4338ca" : (
                    activeDimension === "middleware" ? "#b45309" : (
                      activeDimension === "backup" ? "#047857" : "#6d28d9"
                    )
                  )
                )
              }}>
                {activeDimension === "hardware" ? "🖥️ 02-硬件设备" : (
                  activeDimension === "database" ? "🗄️ 03-数据库" : (
                    activeDimension === "middleware" ? "⚙️ 04-中间件" : (
                      activeDimension === "backup" ? "💾 05-备份策略" : "🛡️ 06-运维保障"
                    )
                  )
                )}
              </span>
              共 <strong style={{ color: "#0f172a" }}>{currentDimTotal}</strong> 条记录
            </div>

            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <span>每页展示:</span>
              <select 
                value={pageSize} 
                onChange={e => { 
                  const sz = Number(e.target.value);
                  setPageSize(sz); 
                  setCurrentPage(1); 
                  setDbPage(1);
                  setMwPage(1);
                  setBkPage(1);
                  setOpsPage(1);
                }}
                style={{ padding: "2px 6px", fontSize: 12 }}
              >
                <option value={15}>15 条</option>
                <option value={25}>25 条</option>
                <option value={50}>50 条</option>
                <option value={100}>100 条</option>
              </select>

              <button 
                className="btn-secondary" 
                style={{ padding: "3px 8px", fontSize: 11 }}
                disabled={currentDimPage <= 1}
                onClick={() => setCurrentDimPage(prev => Math.max(1, prev - 1))}
              >
                ‹ 上一页
              </button>

              <span>
                第 <strong style={{ color: "#2563eb" }}>{currentDimPage}</strong> / {currentDimTotalPages} 页
              </span>

              <button 
                className="btn-secondary" 
                style={{ padding: "3px 8px", fontSize: 11 }}
                disabled={currentDimPage >= currentDimTotalPages}
                onClick={() => setCurrentDimPage(prev => Math.min(currentDimTotalPages, prev + 1))}
              >
                下一页 ›
              </button>
            </div>
          </div>
        </div>
      </div>

      {/* ================= MODAL: DETAIL ASSET METADATA ================= */}
      {detailAsset && (
        <div className="cmdb-modal-mask">
          <div className="cmdb-modal" style={{ maxWidth: 840, width: "96%" }}>
            <div className="cmdb-modal-header" style={{ background: "#0f172a", color: "#fff", borderBottom: 0 }}>
              <div>
                <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                  <span style={{ 
                    background: "#2563eb", 
                    color: "#fff", 
                    fontSize: 11, 
                    padding: "2px 8px", 
                    borderRadius: 4, 
                    fontWeight: 600 
                  }}>
                    序号 #{detailAsset.seq || "-"}
                  </span>
                  <h3 style={{ margin: 0, color: "#fff", fontSize: 16 }}>{detailAsset.name}</h3>
                  <span style={{
                    fontSize: 11,
                    padding: "2px 6px",
                    borderRadius: 4,
                    background: (detailAsset as any)._kind === "physical" ? "#f59e0b" : "#38bdf8",
                    color: "#fff",
                    fontWeight: 600
                  }}>
                    {detailAsset.deviceType || ((detailAsset as any)._kind === "physical" ? "物理机" : "虚拟机")}
                  </span>
                  {detailAsset.isXinchuang === "是" && (
                    <span style={{ background: "#ef4444", color: "#fff", fontSize: 11, padding: "2px 6px", borderRadius: 4, fontWeight: 700 }}>
                      🛡️ 国产信创 OS
                    </span>
                  )}
                </div>
                <small style={{ color: "#94a3b8", display: "block", marginTop: 4 }}>
                  归属项目: {detailAsset.projectName} · 客户单位: {detailAsset.customerName} · {detailAsset.env}环境 · {detailAsset.cloudVendor} ({detailAsset.regionName})
                </small>
              </div>
              <button type="button" className="cmdb-modal-close" style={{ color: "#fff" }} onClick={() => setDetailAsset(null)}>×</button>
            </div>

            <div className="cmdb-modal-body" style={{ maxHeight: "74vh", overflowY: "auto", padding: "16px 20px" }}>
              {/* 5-Dimension Tabs Aligned with Excel v351 */}
              <div style={{
                display: "flex",
                gap: 6,
                borderBottom: "2px solid #e2e8f0",
                marginBottom: 16,
                paddingBottom: 2,
                flexWrap: "wrap"
              }}>
                <button 
                  type="button"
                  onClick={() => setDetailTab("spec")}
                  style={{
                    border: "none",
                    background: "transparent",
                    padding: "8px 12px",
                    fontWeight: 700,
                    fontSize: 12,
                    cursor: "pointer",
                    color: detailTab === "spec" ? "#2563eb" : "#64748b",
                    borderBottom: detailTab === "spec" ? "2px solid #2563eb" : "2px solid transparent",
                    marginBottom: -4,
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 5
                  }}
                >
                  <span>🖥️ 02-硬件规格与OS</span>
                </button>

                <button 
                  type="button"
                  onClick={() => setDetailTab("database")}
                  style={{
                    border: "none",
                    background: "transparent",
                    padding: "8px 12px",
                    fontWeight: 700,
                    fontSize: 12,
                    cursor: "pointer",
                    color: detailTab === "database" ? "#4338ca" : "#64748b",
                    borderBottom: detailTab === "database" ? "2px solid #4338ca" : "2px solid transparent",
                    marginBottom: -4,
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 5
                  }}
                >
                  <span>🗄️ 03-数据库</span>
                  <span style={{ 
                    background: detailTab === "database" ? "#e0e7ff" : "#f1f5f9", 
                    color: detailTab === "database" ? "#3730a3" : "#64748b", 
                    fontSize: 10, 
                    padding: "0 6px", 
                    borderRadius: 10,
                    fontWeight: 700
                  }}>
                    {detailAssetDbs.length}
                  </span>
                </button>

                <button 
                  type="button"
                  onClick={() => setDetailTab("middleware")}
                  style={{
                    border: "none",
                    background: "transparent",
                    padding: "8px 12px",
                    fontWeight: 700,
                    fontSize: 12,
                    cursor: "pointer",
                    color: detailTab === "middleware" ? "#b45309" : "#64748b",
                    borderBottom: detailTab === "middleware" ? "2px solid #b45309" : "2px solid transparent",
                    marginBottom: -4,
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 5
                  }}
                >
                  <span>⚙️ 04-中间件</span>
                  <span style={{ 
                    background: detailTab === "middleware" ? "#fef3c7" : "#f1f5f9", 
                    color: detailTab === "middleware" ? "#92400e" : "#64748b", 
                    fontSize: 10, 
                    padding: "0 6px", 
                    borderRadius: 10,
                    fontWeight: 700
                  }}>
                    {detailAssetMws.length}
                  </span>
                </button>

                <button 
                  type="button"
                  onClick={() => setDetailTab("backup")}
                  style={{
                    border: "none",
                    background: "transparent",
                    padding: "8px 12px",
                    fontWeight: 700,
                    fontSize: 12,
                    cursor: "pointer",
                    color: detailTab === "backup" ? "#047857" : "#64748b",
                    borderBottom: detailTab === "backup" ? "2px solid #047857" : "2px solid transparent",
                    marginBottom: -4,
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 5
                  }}
                >
                  <span>💾 05-数据备份</span>
                  <span style={{ 
                    background: detailTab === "backup" ? "#ecfdf5" : "#f1f5f9", 
                    color: detailTab === "backup" ? "#065f46" : "#64748b", 
                    fontSize: 10, 
                    padding: "0 6px", 
                    borderRadius: 10,
                    fontWeight: 700
                  }}>
                    {detailAssetBks.length}
                  </span>
                </button>

                <button 
                  type="button"
                  onClick={() => setDetailTab("ops")}
                  style={{
                    border: "none",
                    background: "transparent",
                    padding: "8px 12px",
                    fontWeight: 700,
                    fontSize: 12,
                    cursor: "pointer",
                    color: detailTab === "ops" ? "#6d28d9" : "#64748b",
                    borderBottom: detailTab === "ops" ? "2px solid #6d28d9" : "2px solid transparent",
                    marginBottom: -4,
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 5
                  }}
                >
                  <span>🛡️ 06-运维保障</span>
                  <span style={{ 
                    background: detailTab === "ops" ? "#ede9fe" : "#f1f5f9", 
                    color: detailTab === "ops" ? "#5b21b6" : "#64748b", 
                    fontSize: 10, 
                    padding: "0 6px", 
                    borderRadius: 10,
                    fontWeight: 700
                  }}>
                    {detailAssetOps.length}
                  </span>
                </button>

                {copiedNotice && (
                  <span style={{ marginLeft: "auto", alignSelf: "center", fontSize: 11, color: "#16a34a", background: "#dcfce7", padding: "3px 8px", borderRadius: 4, fontWeight: 600 }}>
                    ✓ {copiedNotice} 已复制到剪贴板！
                  </span>
                )}
              </div>

              {/* ── TAB 1: 02-硬件规格与OS ── */}
              {detailTab === "spec" && (
                <div>
                  <h5 style={{ margin: "0 0 10px", fontSize: 13, color: "#0284c7", borderBottom: "1px solid #e0f2fe", paddingBottom: 4 }}>
                    📋 项目与基础归属 (台账基本信息)
                  </h5>
                  <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 12, fontSize: 12, marginBottom: 16 }}>
                    <div><span style={{ color: "#64748b" }}>项目名称:</span> <strong style={{ color: "#1d4ed8" }}>{detailAsset.projectName || "-"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>客户名称:</span> <strong>{detailAsset.customerName || "-"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>环境类型:</span> <strong>{detailAsset.env || "-"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>承载云商:</span> <strong style={{ color: "#2563eb" }}>{detailAsset.cloudVendor || "-"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>安全区域:</span> <strong>{detailAsset.regionName || "-"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>设备形态:</span> <strong>{detailAsset.deviceType || "虚拟机"}</strong></div>
                  </div>

                  <h5 style={{ margin: "0 0 10px", fontSize: 13, color: "#0284c7", borderBottom: "1px solid #e0f2fe", paddingBottom: 4 }}>
                    💻 硬件算力规格 (CPU/内存/磁盘)
                  </h5>
                  <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 12, fontSize: 12, marginBottom: 16 }}>
                    <div><span style={{ color: "#64748b" }}>CPU 架构:</span> <code style={{ background: "#f1f5f9", padding: "1px 4px", borderRadius: 3, fontWeight: 600 }}>{detailAsset.cpuArch || "x86_64"}</code></div>
                    <div><span style={{ color: "#64748b" }}>CPU 核数:</span> <strong style={{ color: "#0284c7" }}>{detailAsset.cpuCores || 4} 核</strong></div>
                    <div><span style={{ color: "#64748b" }}>内存容量:</span> <strong style={{ color: "#0284c7" }}>{detailAsset.memoryGb || 16} GB</strong></div>
                    <div><span style={{ color: "#64748b" }}>系统盘容量:</span> <strong>{detailAsset.systemDiskGb || 50} GB</strong></div>
                    <div><span style={{ color: "#64748b" }}>数据盘容量:</span> <strong>{detailAsset.dataDiskGb || 0} GB</strong></div>
                    <div><span style={{ color: "#64748b" }}>磁盘总容量:</span> <strong style={{ color: "#16a34a" }}>{(detailAsset.systemDiskGb || 0) + (detailAsset.dataDiskGb || 0)} GB</strong></div>
                  </div>

                  <h5 style={{ margin: "0 0 10px", fontSize: 13, color: "#0284c7", borderBottom: "1px solid #e0f2fe", paddingBottom: 4 }}>
                    🌐 IP 地址网络规划
                  </h5>
                  <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 12, fontSize: 12, marginBottom: 16 }}>
                    <div>
                      <span style={{ color: "#64748b" }}>私有业务 IP:</span>{" "}
                      <strong style={{ fontFamily: "monospace", color: "#0f172a" }}>{detailAsset.privateIp || detailAsset.ip || "-"}</strong>
                      {(detailAsset.privateIp || detailAsset.ip) && (
                        <button type="button" className="btn-secondary" style={{ padding: "1px 5px", fontSize: 10, marginLeft: 6 }} onClick={() => copyToClipboard(detailAsset.privateIp || detailAsset.ip || "", "业务IP")}>复制</button>
                      )}
                    </div>
                    <div>
                      <span style={{ color: "#64748b" }}>内部大网 IP:</span>{" "}
                      <strong style={{ fontFamily: "monospace", color: "#0f172a" }}>{detailAsset.internalWanIp || "-"}</strong>
                    </div>
                    <div>
                      <span style={{ color: "#64748b" }}>VIP / EIP:</span>{" "}
                      <strong style={{ fontFamily: "monospace", color: "#d97706" }}>{detailAsset.vip || detailAsset.eip || "-"}</strong>
                    </div>
                    <div>
                      <span style={{ color: "#64748b" }}>远程连接端口:</span>{" "}
                      <strong style={{ fontFamily: "monospace", color: "#d97706" }}>{detailAsset.remotePort || 22}</strong>
                    </div>
                  </div>

                  <h5 style={{ margin: "0 0 10px", fontSize: 13, color: "#0284c7", borderBottom: "1px solid #e0f2fe", paddingBottom: 4 }}>
                    🛡️ 操作系统与信创合规
                  </h5>
                  <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 12, fontSize: 12, marginBottom: 16 }}>
                    <div><span style={{ color: "#64748b" }}>OS 家族:</span> <strong>{detailAsset.osFamily || "Linux"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>系统发行版:</span> <strong>{detailAsset.osVersion || "-"}</strong></div>
                    <div>
                      <span style={{ color: "#64748b" }}>信创标识:</span>{" "}
                      <span style={{ 
                        fontSize: 11, 
                        fontWeight: 700, 
                        padding: "1px 6px", 
                        borderRadius: 3, 
                        background: detailAsset.isXinchuang === "是" ? "#fee2e2" : "#f1f5f9", 
                        color: detailAsset.isXinchuang === "是" ? "#dc2626" : "#475569" 
                      }}>
                        {detailAsset.isXinchuang === "是" ? "🛡️ 国产信创" : "常规 OS"}
                      </span>
                    </div>
                    <div style={{ gridColumn: "span 3" }}>
                      <span style={{ color: "#64748b" }}>内核版本:</span>{" "}
                      <code style={{ background: "#f8fafc", padding: "2px 6px", borderRadius: 4, fontSize: 11, fontFamily: "monospace" }}>{detailAsset.kernelVersion || "-"}</code>
                    </div>
                  </div>

                  <h5 style={{ margin: "0 0 8px", fontSize: 13, color: "#64748b" }}>📝 备注信息</h5>
                  <p style={{ margin: 0, fontSize: 12, color: "#475569", background: "#f8fafc", padding: "8px 12px", borderRadius: 6 }}>
                    {detailAsset.remarks || "暂无特别说明。"}
                  </p>
                </div>
              )}

              {/* ── TAB 2: 03-数据库实例 ── */}
              {detailTab === "database" && (
                <div>
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
                    <h5 style={{ margin: 0, fontSize: 13, color: "#4338ca" }}>
                      🗄️ 节点/项目数据库实例档案 ({detailAssetDbs.length} 个实例)
                    </h5>
                    <button
                      type="button"
                      className="btn-primary"
                      style={{ fontSize: 11, padding: "3px 10px", background: "#4f46e5" }}
                      onClick={() => {
                        setNewDbDraft(prev => ({
                          ...prev,
                          projectName: detailAsset.projectName || "",
                          customerName: detailAsset.customerName || "",
                          env: detailAsset.env || "生产",
                          cloudVendor: detailAsset.cloudVendor || "",
                          regionName: detailAsset.regionName || "",
                          privateIp: detailAsset.privateIp || detailAsset.ip || ""
                        }));
                        setShowAddDbModal(true);
                      }}
                    >
                      ＋ 挂载新数据库
                    </button>
                  </div>

                  {detailAssetDbs.length === 0 ? (
                    <div style={{ padding: "30px 20px", textAlign: "center", background: "#f8fafc", borderRadius: 6, color: "#94a3b8", fontSize: 12 }}>
                      当前节点未直接部署或挂载独立数据库实例
                    </div>
                  ) : (
                    <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
                      {detailAssetDbs.map(db => (
                        <div key={db.id} style={{ border: "1px solid #e0e7ff", borderRadius: 6, padding: "10px 14px", background: "#fbfcfe", fontSize: 12 }}>
                          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 6 }}>
                            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                              <span style={{ background: "#4338ca", color: "#fff", padding: "1px 6px", borderRadius: 3, fontSize: 11, fontWeight: 700 }}>
                                {db.dbSoftware}
                              </span>
                              <strong style={{ color: "#1e293b", fontSize: 13 }}>{db.instanceSid || db.name}</strong>
                              <span style={{ fontSize: 10, background: "#e0e7ff", color: "#3730a3", padding: "1px 6px", borderRadius: 4, fontWeight: 600 }}>
                                {db.deployMode || "单机"}
                              </span>
                            </div>
                            <span style={{ color: "#16a34a", fontWeight: 700, fontSize: 11 }}>● 在线 active</span>
                          </div>

                          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 8, color: "#475569" }}>
                            <div>软件大类: <strong>{db.dbCategory || "关系型"}</strong></div>
                            <div>软件版本: <code style={{ color: "#2563eb", fontWeight: 600 }}>{db.version}</code></div>
                            <div>服务端口: <strong style={{ color: "#d97706", fontFamily: "monospace" }}>{db.port}</strong></div>
                            <div>业务库名: <strong style={{ color: "#0369a1" }}>{db.dbName || "-"}</strong></div>
                            <div>绑定 IP: <strong style={{ fontFamily: "monospace" }}>{db.privateIp || db.hostIp}</strong></div>
                            <div>VIP/EIP: <strong style={{ fontFamily: "monospace", color: "#d97706" }}>{db.vipEip || "-"}</strong></div>
                            {db.clusterName && <div style={{ gridColumn: "span 3" }}>集群/组名: <strong style={{ color: "#4338ca" }}>{db.clusterName}</strong></div>}
                          </div>
                          {db.remarks && <small style={{ color: "#94a3b8", display: "block", marginTop: 4 }}>说明: {db.remarks}</small>}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {/* ── TAB 3: 04-中间件服务 ── */}
              {detailTab === "middleware" && (
                <div>
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
                    <h5 style={{ margin: 0, fontSize: 13, color: "#b45309" }}>
                      ⚙️ 节点中间件服务档案 ({detailAssetMws.length} 个服务)
                    </h5>
                    <button
                      type="button"
                      className="btn-primary"
                      style={{ fontSize: 11, padding: "3px 10px", background: "#d97706" }}
                      onClick={() => {
                        setNewMwDraft(prev => ({
                          ...prev,
                          projectName: detailAsset.projectName || "",
                          customerName: detailAsset.customerName || "",
                          env: detailAsset.env || "生产",
                          cloudVendor: detailAsset.cloudVendor || "",
                          regionName: detailAsset.regionName || "",
                          privateIp: detailAsset.privateIp || detailAsset.ip || ""
                        }));
                        setShowAddMwModal(true);
                      }}
                    >
                      ＋ 挂载新中间件
                    </button>
                  </div>

                  {detailAssetMws.length === 0 ? (
                    <div style={{ padding: "30px 20px", textAlign: "center", background: "#f8fafc", borderRadius: 6, color: "#94a3b8", fontSize: 12 }}>
                      当前节点暂无独立登记的中间件组件
                    </div>
                  ) : (
                    <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
                      {detailAssetMws.map(mw => (
                        <div key={mw.id} style={{ border: "1px solid #fed7aa", borderRadius: 6, padding: "10px 14px", background: "#fffdfa", fontSize: 12 }}>
                          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 6 }}>
                            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                              <span style={{ background: "#d97706", color: "#fff", padding: "1px 6px", borderRadius: 3, fontSize: 11, fontWeight: 700 }}>
                                {mw.mwSoftware}
                              </span>
                              <strong style={{ color: "#1e293b", fontSize: 13 }}>{mw.name}</strong>
                              <span style={{ background: "#fef3c7", color: "#92400e", padding: "1px 6px", borderRadius: 4, fontSize: 10, fontWeight: 600 }}>
                                {mw.mwType}
                              </span>
                            </div>
                            <span style={{ color: "#16a34a", fontWeight: 700, fontSize: 11 }}>● 运行中 running</span>
                          </div>

                          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 8, color: "#475569" }}>
                            <div>版本: <code style={{ color: "#2563eb", fontWeight: 600 }}>{mw.version || "-"}</code></div>
                            <div>端口: <strong style={{ color: "#d97706", fontFamily: "monospace" }}>{mw.port || "-"}</strong></div>
                            <div>角色: <strong>{mw.role || "-"}</strong></div>
                            <div>运行时: <strong style={{ color: "#0d9488" }}>{mw.runtime || "-"}</strong></div>
                            <div>运行环境: <span>{mw.env} · {mw.cloudVendor}</span></div>
                            <div>监听 IP: <strong style={{ fontFamily: "monospace" }}>{mw.privateIp}</strong></div>
                          </div>
                          {mw.remarks && <small style={{ color: "#94a3b8", display: "block", marginTop: 4 }}>说明: {mw.remarks}</small>}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {/* ── TAB 4: 05-数据备份方案 ── */}
              {detailTab === "backup" && (
                <div>
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
                    <h5 style={{ margin: 0, fontSize: 13, color: "#047857" }}>
                      💾 数据备份策略方案 ({detailAssetBks.length} 条策略)
                    </h5>
                    <button
                      type="button"
                      className="btn-primary"
                      style={{ fontSize: 11, padding: "3px 10px", background: "#059669" }}
                      onClick={() => {
                        setNewBkDraft(prev => ({
                          ...prev,
                          projectName: detailAsset.projectName || "",
                          customerName: detailAsset.customerName || "",
                          env: detailAsset.env || "生产",
                          cloudVendor: detailAsset.cloudVendor || "",
                          regionName: detailAsset.regionName || "",
                          privateIp: detailAsset.privateIp || detailAsset.ip || ""
                        }));
                        setShowAddBkModal(true);
                      }}
                    >
                      ＋ 登记新备份
                    </button>
                  </div>

                  {detailAssetBks.length === 0 ? (
                    <div style={{ padding: "30px 20px", textAlign: "center", background: "#f8fafc", borderRadius: 6, color: "#94a3b8", fontSize: 12 }}>
                      当前节点所属项目暂无独立登记的备份策略
                    </div>
                  ) : (
                    <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
                      {detailAssetBks.map(bk => (
                        <div key={bk.id} style={{ border: "1px solid #a7f3d0", borderRadius: 6, padding: "10px 14px", background: "#f0fdf4", fontSize: 12 }}>
                          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 6 }}>
                            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                              <span style={{ background: "#059669", color: "#fff", padding: "1px 6px", borderRadius: 3, fontSize: 11, fontWeight: 700 }}>
                                {bk.backupType}
                              </span>
                              <strong style={{ color: "#065f46", fontSize: 13 }}>{bk.backupPolicy}</strong>
                              <span style={{ background: "#dcfce7", color: "#15803d", padding: "1px 6px", borderRadius: 4, fontSize: 10, fontWeight: 600 }}>
                                {bk.backupMethod}
                              </span>
                            </div>
                            <span style={{ color: "#16a34a", fontWeight: 700, fontSize: 11 }}>● 自动执行已生效</span>
                          </div>

                          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8, color: "#374151" }}>
                            <div>存储介质/位置: <strong>{bk.storageLocation}</strong></div>
                            <div>数据保留周期: <strong style={{ color: "#2563eb" }}>{bk.retentionDays} 天</strong></div>
                            <div>上次完成时间: <span style={{ fontFamily: "monospace", color: "#64748b" }}>{bk.lastBackupTime || "2026-09-20 02:00:00"}</span></div>
                            <div>私有业务 IP: <span style={{ fontFamily: "monospace" }}>{bk.privateIp}</span></div>
                          </div>
                          {bk.remarks && <small style={{ color: "#64748b", display: "block", marginTop: 4 }}>说明: {bk.remarks}</small>}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}

              {/* ── TAB 5: 06-运维保障通道 ── */}
              {detailTab === "ops" && (
                <div>
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
                    <h5 style={{ margin: 0, fontSize: 13, color: "#6d28d9" }}>
                      🛡️ 运维保障渠道与访问规范 ({detailAssetOps.length} 条记录)
                    </h5>
                    <button
                      type="button"
                      className="btn-primary"
                      style={{ fontSize: 11, padding: "3px 10px", background: "#7c3aed" }}
                      onClick={() => {
                        setNewOpsDraft(prev => ({
                          ...prev,
                          projectName: detailAsset.projectName || "",
                          customerName: detailAsset.customerName || "",
                          env: detailAsset.env || "生产",
                          cloudVendor: detailAsset.cloudVendor || "",
                          regionName: detailAsset.regionName || "",
                          privateIp: detailAsset.privateIp || detailAsset.ip || ""
                        }));
                        setShowAddOpsModal(true);
                      }}
                    >
                      ＋ 登记新运维保障
                    </button>
                  </div>

                  {detailAssetOps.length === 0 ? (
                    <div style={{ padding: "30px 20px", textAlign: "center", background: "#f8fafc", borderRadius: 6, color: "#94a3b8", fontSize: 12 }}>
                      当前节点暂无独立填报的运维专网通道或厂商保障记录
                    </div>
                  ) : (
                    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
                      {detailAssetOps.map(op => (
                        <div key={op.id} style={{ border: "1px solid #ddd6fe", borderRadius: 6, padding: "12px 14px", background: "#faf5ff", fontSize: 12 }}>
                          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
                            <strong style={{ color: "#5b21b6", fontSize: 13 }}>运维保障厂商: {op.opsVendor}</strong>
                            <div style={{ display: "flex", gap: 6 }}>
                              <span style={{ background: op.monitoringCoverage === "是" ? "#dcfce7" : "#fef3c7", color: op.monitoringCoverage === "是" ? "#15803d" : "#92400e", padding: "1px 6px", borderRadius: 4, fontSize: 10, fontWeight: 700 }}>
                                监控: {op.monitoringCoverage}
                              </span>
                              <span style={{ background: "#ede9fe", color: "#6d28d9", padding: "1px 6px", borderRadius: 4, fontSize: 10, fontWeight: 600 }}>
                                巡检: {op.inspectionCycle}
                              </span>
                            </div>
                          </div>

                          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10, color: "#374151" }}>
                            <div style={{ background: "#fff", padding: "8px 10px", borderRadius: 6, border: "1px solid #e2e8f0" }}>
                              <span style={{ color: "#64748b", fontWeight: 600, display: "block" }}>VPN 拨号专线:</span>
                              <div style={{ fontFamily: "monospace", color: "#1d4ed8", marginTop: 2 }}>{op.vpnAddress || "-"}</div>
                              <div style={{ color: "#475569", marginTop: 4, display: "flex", alignItems: "center", gap: 6, flexWrap: "wrap" }}>
                                <span>账号: <code style={{ color: "#0f172a", fontWeight: 700 }}>{op.vpnAccount || "-"}</code></span>
                                {op.vpnUserName && (
                                  <span style={{ background: "#e0f2fe", color: "#0369a1", padding: "1px 6px", borderRadius: 4, fontSize: 11, fontWeight: 700 }}>
                                    👤 使用人: {op.vpnUserName}
                                  </span>
                                )}
                              </div>
                              {op.vpnNetworkEnv && (
                                <div style={{ color: "#64748b", fontSize: 11, marginTop: 2 }}>网络环境: {op.vpnNetworkEnv}</div>
                              )}
                              {op.vpnAddress && (
                                <button type="button" className="btn-secondary" style={{ padding: "1px 6px", fontSize: 10, marginTop: 4 }} onClick={() => copyToClipboard(`${op.vpnAddress} (账号: ${op.vpnAccount}${op.vpnUserName ? ` 使用人: ${op.vpnUserName}` : ""})`, "VPN信息")}>
                                  复制VPN
                                </button>
                              )}
                            </div>

                            <div style={{ background: "#fff", padding: "8px 10px", borderRadius: 6, border: "1px solid #e2e8f0" }}>
                              <span style={{ color: "#64748b", fontWeight: 600, display: "block" }}>堡垒机 (JumpServer):</span>
                              <div style={{ marginTop: 2 }}>
                                {op.bastionAddress ? (
                                  <a href={op.bastionAddress.startsWith("http") ? op.bastionAddress : `https://${op.bastionAddress}`} target="_blank" rel="noreferrer" style={{ color: "#7c3aed", fontWeight: 700, textDecoration: "underline" }}>
                                    {op.bastionAddress}
                                  </a>
                                ) : "-"}
                              </div>
                              <div style={{ color: "#475569", marginTop: 4, display: "flex", alignItems: "center", gap: 6, flexWrap: "wrap" }}>
                                <span>账号: <code style={{ color: "#0f172a", fontWeight: 700 }}>{op.bastionAccount || "-"}</code></span>
                                {op.bastionUserName && (
                                  <span style={{ background: "#ede9fe", color: "#6d28d9", padding: "1px 6px", borderRadius: 4, fontSize: 11, fontWeight: 700 }}>
                                    👤 使用人: {op.bastionUserName}
                                  </span>
                                )}
                              </div>
                              {op.bastionAddress && (
                                <button type="button" className="btn-secondary" style={{ padding: "1px 6px", fontSize: 10, marginTop: 4 }} onClick={() => copyToClipboard(`${op.bastionAddress} (账号: ${op.bastionAccount}${op.bastionUserName ? ` 使用人: ${op.bastionUserName}` : ""})`, "堡垒机信息")}>
                                  复制堡垒机
                                </button>
                              )}
                            </div>

                            <div style={{ gridColumn: "span 2", background: "#fff", padding: "8px 10px", borderRadius: 6, border: "1px solid #e2e8f0" }}>
                              <span style={{ color: "#64748b", fontWeight: 600, display: "block" }}>服务器直连指令:</span>
                              <div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 4 }}>
                                <code style={{ background: "#f1f5f9", padding: "3px 8px", borderRadius: 4, color: "#0f172a", fontFamily: "monospace", flex: 1 }}>
                                  {op.serverAccessAddress || `ssh root@${detailAsset.privateIp || detailAsset.ip} -p ${detailAsset.remotePort || 22}`}
                                </code>
                                <button type="button" className="btn-secondary" style={{ padding: "2px 8px", fontSize: 11 }} onClick={() => copyToClipboard(op.serverAccessAddress || `ssh root@${detailAsset.privateIp || detailAsset.ip} -p ${detailAsset.remotePort || 22}`, "SSH命令")}>
                                  复制
                                </button>
                              </div>
                            </div>

                            <div>变更窗口: <strong style={{ color: "#b45309" }}>{op.changeWindow}</strong></div>
                            <div>网络分区: <strong>{op.networkZone}</strong> · <span style={{ color: op.exposureSurface === "公网" ? "#dc2626" : "#16a34a", fontWeight: 700 }}>{op.exposureSurface}</span></div>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>

            <div className="cmdb-modal-footer" style={{ borderTop: "1px solid #e2e8f0", padding: "12px 20px", display: "flex", justifyContent: "flex-end" }}>
              <button type="button" className="btn-secondary" onClick={() => setDetailAsset(null)}>关 闭</button>
            </div>
          </div>
        </div>
      )}

      {showAddModal && (
        <div className="cmdb-modal-mask">
          <form className="cmdb-modal" style={{ maxWidth: 740, width: "95%" }} onSubmit={handleAddSubmit}>
            <div className="cmdb-modal-header">
              <h3>📝 录入项目资产（按项目台账归属）</h3>
              <button type="button" className="cmdb-modal-close" onClick={() => setShowAddModal(false)}>×</button>
            </div>
            <div className="cmdb-modal-body" style={{ maxHeight: "72vh", overflowY: "auto", padding: 20 }}>
              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 所属项目名称</label>
                  <select 
                    value={newForm.projectName} 
                    onChange={e => {
                      const p = projects.find(x => x.name === e.target.value);
                      if (p) {
                        setNewForm({
                          ...newForm,
                          projectName: p.name,
                          customerName: p.customerName,
                          env: p.env,
                          cloudVendor: p.cloudVendor,
                          regionName: p.regionName
                        });
                      }
                    }}
                  >
                    {projects.map(p => (
                      <option key={p.id} value={p.name}>{p.name} ({p.customerName})</option>
                    ))}
                  </select>
                </div>
                <div className="form-field-item">
                  <label>* 设备名称</label>
                  <input 
                    placeholder="如：人社局-gov-北控伟仕-生产-新应用节点" 
                    value={newForm.name} 
                    onChange={e => setNewForm({ ...newForm, name: e.target.value })} 
                    required 
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 客户名称</label>
                  <input 
                    value={newForm.customerName} 
                    onChange={e => setNewForm({ ...newForm, customerName: e.target.value })} 
                    required 
                  />
                </div>
                <div className="form-field-item">
                  <label>* 环境</label>
                  <select value={newForm.env} onChange={e => setNewForm({ ...newForm, env: e.target.value })}>
                    {EXCEL_ENVIRONMENTS.map(env => (
                      <option key={env} value={env}>{env}环境</option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 云厂商</label>
                  <select value={newForm.cloudVendor} onChange={e => setNewForm({ ...newForm, cloudVendor: e.target.value })}>
                    {EXCEL_CLOUD_VENDORS.map(v => (
                      <option key={v} value={v}>{v}</option>
                    ))}
                  </select>
                </div>
                <div className="form-field-item">
                  <label>* 设备形态</label>
                  <select value={newForm.deviceType} onChange={e => setNewForm({ ...newForm, deviceType: e.target.value })}>
                    {EXCEL_DEVICE_TYPES.map(d => (
                      <option key={d} value={d}>{d}</option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 私有IP（业务IP）</label>
                  <input 
                    placeholder="如 192.125.31.250"
                    value={newForm.ip} 
                    onChange={e => setNewForm({ ...newForm, ip: e.target.value, privateIp: e.target.value })} 
                    required 
                  />
                </div>
                <div className="form-field-item">
                  <label>内大网IP</label>
                  <input 
                    placeholder="如 192.123.2.100"
                    value={newForm.internalWanIp} 
                    onChange={e => setNewForm({ ...newForm, internalWanIp: e.target.value })} 
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>CPU 架构与核数</label>
                  <div style={{ display: "flex", gap: 8 }}>
                    <select value={newForm.cpuArch} onChange={e => setNewForm({ ...newForm, cpuArch: e.target.value })}>
                      {EXCEL_CPU_ARCHS.map(a => (
                        <option key={a} value={a}>{a}</option>
                      ))}
                    </select>
                    <input 
                      type="number" 
                      placeholder="核数" 
                      value={newForm.cpuCores} 
                      onChange={e => setNewForm({ ...newForm, cpuCores: Number(e.target.value) })} 
                    />
                  </div>
                </div>
                <div className="form-field-item">
                  <label>内存 (GB)</label>
                  <input 
                    type="number" 
                    value={newForm.memoryGb} 
                    onChange={e => setNewForm({ ...newForm, memoryGb: Number(e.target.value) })} 
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>系统盘 / 数据盘 (GB)</label>
                  <div style={{ display: "flex", gap: 8 }}>
                    <input 
                      type="number" 
                      placeholder="系统盘GB" 
                      value={newForm.systemDiskGb} 
                      onChange={e => setNewForm({ ...newForm, systemDiskGb: Number(e.target.value) })} 
                    />
                    <input 
                      type="number" 
                      placeholder="数据盘GB" 
                      value={newForm.dataDiskGb} 
                      onChange={e => setNewForm({ ...newForm, dataDiskGb: Number(e.target.value) })} 
                    />
                  </div>
                </div>
                <div className="form-field-item">
                  <label>是否信创 OS</label>
                  <select value={newForm.isXinchuang} onChange={e => setNewForm({ ...newForm, isXinchuang: e.target.value })}>
                    <option value="否">否 (CentOS / RedHat 等)</option>
                    <option value="是">是 (麒麟 / 统信UOS 等)</option>
                  </select>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>OS 版本</label>
                  <input 
                    list="excel-os-versions"
                    placeholder="从代码表选择或输入，如 麒麟V10 SP3"
                    value={newForm.osVersion} 
                    onChange={e => setNewForm({ ...newForm, osVersion: e.target.value })} 
                  />
                  <datalist id="excel-os-versions">
                    {EXCEL_OS_VERSIONS.map(os => (
                      <option key={os} value={os}>{os}</option>
                    ))}
                  </datalist>
                </div>
                <div className="form-field-item">
                  <label>远程端口</label>
                  <input 
                    type="number" 
                    value={newForm.remotePort} 
                    onChange={e => setNewForm({ ...newForm, remotePort: Number(e.target.value) })} 
                  />
                </div>
              </div>

              <div className="form-field-item">
                <label>备注说明</label>
                <textarea 
                  rows={2}
                  value={newForm.remarks}
                  onChange={e => setNewForm({ ...newForm, remarks: e.target.value })}
                />
              </div>
            </div>
            <div className="cmdb-modal-footer">
              <button type="button" className="btn-secondary" onClick={() => setShowAddModal(false)}>取 消</button>
              <button type="submit" className="btn-primary">确 定 录 入</button>
            </div>
          </form>
        </div>
      )}
    
      {/* ================= MODAL: PROJECT VPN FULL VIEW ================= */}
      {showProjectVpnModal && currentProjectVpn && (
        <div className="cmdb-modal-mask">
          <div className="cmdb-modal" style={{ maxWidth: 640, width: "95%" }}>
            <div className="cmdb-modal-header" style={{ background: "#9a3412", color: "#fff", borderBottom: 0 }}>
              <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <span style={{ fontSize: 18 }}>🛡️</span>
                <h3 style={{ margin: 0, color: "#fff", fontSize: 16 }}>{currentProjectVpn.name}</h3>
              </div>
              <button type="button" className="cmdb-modal-close" style={{ color: "#fff" }} onClick={() => setShowProjectVpnModal(false)}>×</button>
            </div>
            <div className="cmdb-modal-body" style={{ padding: 20, fontSize: 12 }}>
              <div style={{ background: "#fffaf5", border: "1px solid #fed7aa", padding: 14, borderRadius: 6, marginBottom: 14 }}>
                <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10, marginBottom: 10 }}>
                  <div><span style={{ color: "#64748b" }}>所属项目:</span> <strong>{currentProject?.name}</strong></div>
                  <div><span style={{ color: "#64748b" }}>承载云厂商:</span> <strong style={{ color: "#2563eb" }}>{currentProject?.cloudVendor}</strong></div>
                  <div><span style={{ color: "#64748b" }}>VPN 客户端:</span> <strong style={{ color: "#c2410c" }}>{currentProjectVpn.vpnClientType}</strong></div>
                  <div><span style={{ color: "#64748b" }}>所属环境:</span> <strong>{currentProject?.env}环境</strong></div>
                </div>

                <div style={{ marginBottom: 10 }}>
                  <span style={{ color: "#64748b", display: "block", marginBottom: 2 }}>VPN 网关认证地址:</span>
                  <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                    <code style={{ fontSize: 13, background: "#fff", border: "1px solid #fdba74", padding: "4px 8px", borderRadius: 4, color: "#c2410c", fontWeight: 600, flex: 1 }}>
                      {currentProjectVpn.vpnGateway}
                    </code>
                    <button 
                      type="button" 
                      className="btn-primary" 
                      style={{ padding: "4px 10px", fontSize: 11, background: "#ea580c", borderColor: "#c2410c" }}
                      onClick={() => window.open(currentProjectVpn.vpnGateway, "_blank")}
                    >
                      🔗 前往网关
                    </button>
                    <button 
                      type="button" 
                      className="btn-secondary" 
                      style={{ padding: "4px 10px", fontSize: 11 }}
                      onClick={() => copyToClipboard(currentProjectVpn.vpnGateway || "", "VPN网关地址")}
                    >
                      📋 复制
                    </button>
                  </div>
                </div>

                <div style={{ marginBottom: 10 }}>
                  <span style={{ color: "#64748b", display: "block", marginBottom: 2 }}>拨号账号与策略:</span>
                  <div style={{ background: "#fff", border: "1px solid #fed7aa", padding: 6, borderRadius: 4, color: "#7c2d12" }}>
                    {currentProjectVpn.accountNote}
                  </div>
                </div>

                <div>
                  <span style={{ color: "#64748b", display: "block", marginBottom: 2 }}>路由专网段:</span>
                  <code style={{ background: "#fff", border: "1px solid #e2e8f0", padding: 4, borderRadius: 4, display: "block" }}>
                    {currentProjectVpn.vpnNetworkSegment}
                  </code>
                </div>

                {/* 项目已分配 VPN 账号与使用者明细 */}
                {(() => {
                  const projOps = opsRecords.filter(o => o.projectName === currentProject?.name || o.projectId === currentProject?.id);
                  if (projOps.length === 0) return null;
                  return (
                    <div style={{ marginTop: 14, borderTop: "1px dashed #fed7aa", paddingTop: 12 }}>
                      <span style={{ color: "#9a3412", fontWeight: 700, display: "block", marginBottom: 6 }}>
                        👥 项目已分配 VPN 账号与使用人名录 ({projOps.length} 人):
                      </span>
                      <div style={{ maxHeight: 180, overflowY: "auto", border: "1px solid #fed7aa", borderRadius: 6, background: "#fff" }}>
                        <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 11 }}>
                          <thead>
                            <tr style={{ background: "#ffedd5", color: "#9a3412", textAlign: "left" }}>
                              <th style={{ padding: "4px 8px" }}>序号</th>
                              <th style={{ padding: "4px 8px" }}>VPN 账号</th>
                              <th style={{ padding: "4px 8px" }}>使用人姓名</th>
                              <th style={{ padding: "4px 8px" }}>网络环境</th>
                              <th style={{ padding: "4px 8px" }}>堡垒机账号</th>
                            </tr>
                          </thead>
                          <tbody>
                            {projOps.map((po, idx) => (
                              <tr key={po.id || idx} style={{ borderBottom: "1px solid #fed7aa" }}>
                                <td style={{ padding: "4px 8px", color: "#64748b" }}>{idx + 1}</td>
                                <td style={{ padding: "4px 8px", fontFamily: "monospace", fontWeight: 600 }}>{po.vpnAccount || "-"}</td>
                                <td style={{ padding: "4px 8px" }}>
                                  {po.vpnUserName ? (
                                    <span style={{ background: "#e0f2fe", color: "#0369a1", padding: "1px 6px", borderRadius: 4, fontWeight: 700 }}>
                                      👤 {po.vpnUserName}
                                    </span>
                                  ) : "-"}
                                </td>
                                <td style={{ padding: "4px 8px", color: "#64748b" }}>{po.vpnNetworkEnv || "互联网区"}</td>
                                <td style={{ padding: "4px 8px", fontFamily: "monospace", color: "#7c3aed" }}>{po.bastionAccount || "-"}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    </div>
                  );
                })()}
              </div>
            </div>
            <div className="cmdb-modal-footer">
              <button type="button" className="btn-secondary" onClick={() => setShowProjectVpnModal(false)}>关 闭</button>
            </div>
          </div>
        </div>
      )}

      {/* Excel 台账导入弹窗 (支持 v351 五维全表 / 单 Sheet 智能导入) */}
      {showImportModal && (
        <ImportModal
          targetProjectName={currentProject ? currentProject.name : null}
          activeDimension={activeDimension}
          existingHardware={allAssets}
          existingDatabases={databases}
          existingMiddlewares={middlewares}
          existingBackups={backups}
          existingOpsRecords={opsRecords}
          onClose={() => setShowImportModal(false)}
          onConfirmImportMulti={(res, strategy) => {
            if (onBatchImportMultiDimension) {
              onBatchImportMultiDimension(res, strategy, currentProject ? currentProject.name : null);
            } else {
              if (res.hardware.length > 0 && onBatchImportAssets) {
                onBatchImportAssets(res.hardware, strategy, currentProject ? currentProject.name : null);
              }
              if (res.databases.length > 0 && onBatchImportDatabases) {
                onBatchImportDatabases(res.databases, strategy, currentProject ? currentProject.name : null);
              }
              if (res.middlewares.length > 0 && onBatchImportMiddlewares) {
                onBatchImportMiddlewares(res.middlewares, strategy, currentProject ? currentProject.name : null);
              }
              if (res.backups.length > 0 && onBatchImportBackups) {
                onBatchImportBackups(res.backups, strategy, currentProject ? currentProject.name : null);
              }
              if (res.opsRecords.length > 0 && onBatchImportOpsRecords) {
                onBatchImportOpsRecords(res.opsRecords, strategy, currentProject ? currentProject.name : null);
              }
            }

            const strategyLabel = strategy === "upsert" ? "智能覆盖更新" : strategy === "skip" ? "仅新增(跳过重复)" : "全量替换";
            const parts = [];
            if (res.hardware.length > 0) parts.push(`硬件 ${res.hardware.length} 台`);
            if (res.databases.length > 0) parts.push(`数据库 ${res.databases.length} 条`);
            if (res.middlewares.length > 0) parts.push(`中间件 ${res.middlewares.length} 条`);
            if (res.backups.length > 0) parts.push(`备份 ${res.backups.length} 条`);
            if (res.opsRecords.length > 0) parts.push(`运维 ${res.opsRecords.length} 条`);

            const summary = parts.length > 0 ? parts.join("、") : "0 条记录";
            setToastNotice(`✓ 执行完成 (${strategyLabel})：共成功导入 ${summary}！`);
            setTimeout(() => setToastNotice(null), 4000);
          }}
          onConfirmImport={(imported, strategy) => {
            if (onBatchImportAssets) {
              onBatchImportAssets(imported, strategy, currentProject ? currentProject.name : null);
            }
            const strategyLabel = strategy === "upsert" ? "智能覆盖更新" : strategy === "skip" ? "仅新增(跳过重复)" : "全量替换";
            setToastNotice(`✓ 执行完成 (${strategyLabel})：共成功导入 ${imported.length} 台选定设备！`);
            setTimeout(() => setToastNotice(null), 3500);
          }}
        />
      )}

            {/* ================= MODAL: 03-DATABASE ADD ================= */}
      {showAddDbModal && (
        <div className="cmdb-modal-mask">
          <div className="cmdb-modal" style={{ maxWidth: 650, width: "95%" }}>
            <div className="cmdb-modal-header" style={{ background: "#4338ca", color: "#fff", borderBottom: 0 }}>
              <h3 style={{ margin: 0, fontSize: 16, color: "#fff" }}>🗄️ 录入数据库实例台账 (Sheet 03)</h3>
              <button type="button" className="cmdb-modal-close" style={{ color: "#fff" }} onClick={() => setShowAddDbModal(false)}>×</button>
            </div>
            <form onSubmit={(e) => {
              e.preventDefault();
              const newRecord: any = {
                id: `db-new-${Date.now()}`,
                seq: (databases?.length || 0) + 1,
                name: `${newDbDraft.dbSoftware}_${newDbDraft.port}`,
                type: newDbDraft.dbSoftware,
                version: newDbDraft.version,
                port: Number(newDbDraft.port) || 3306,
                status: "active",
                businessId: `BIZ-DB-${Date.now().toString().slice(-4)}`,
                hostIp: newDbDraft.privateIp,
                privateIp: newDbDraft.privateIp,
                vipEip: newDbDraft.vipEip,
                arch: "x86_64",
                projectName: newDbDraft.projectName || currentProject?.name || projects[0]?.name,
                customerName: newDbDraft.customerName || currentProject?.customerName || projects[0]?.customerName,
                env: newDbDraft.env,
                cloudVendor: newDbDraft.cloudVendor,
                regionName: newDbDraft.regionName,
                dbCategory: newDbDraft.dbCategory,
                dbSoftware: newDbDraft.dbSoftware,
                instanceSid: newDbDraft.instanceSid,
                dbName: newDbDraft.dbName,
                deployMode: newDbDraft.deployMode,
                clusterName: newDbDraft.clusterName,
                remarks: newDbDraft.remarks
              };
              onAddDatabase?.(newRecord);
              setShowAddDbModal(false);
              setToastNotice(`✓ 成功登记数据库实例【${newRecord.instanceSid || newRecord.name}】！`);
              setTimeout(() => setToastNotice(null), 3500);
            }}>
              <div className="cmdb-modal-body" style={{ maxHeight: "72vh", overflowY: "auto", padding: "16px 20px" }}>
                <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>所属项目 *</span>
                    <select
                      value={newDbDraft.projectName}
                      onChange={e => {
                        const p = projects.find(x => x.name === e.target.value);
                        setNewDbDraft(prev => ({
                          ...prev,
                          projectName: e.target.value,
                          customerName: p ? p.customerName : prev.customerName,
                          env: p ? p.env : prev.env,
                          cloudVendor: p ? p.cloudVendor : prev.cloudVendor,
                          regionName: p ? p.regionName : prev.regionName
                        }));
                      }}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    >
                      {projects.map(p => (
                        <option key={p.id} value={p.name}>{p.name}</option>
                      ))}
                    </select>
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>客户单位</span>
                    <input
                      value={newDbDraft.customerName}
                      onChange={e => setNewDbDraft(prev => ({ ...prev, customerName: e.target.value }))}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>数据库大类</span>
                    <select
                      value={newDbDraft.dbCategory}
                      onChange={e => {
                        const nextCat = e.target.value;
                        const recs = EXCEL_DB_CATEGORY_SOFTWARE_MAP[nextCat];
                        setNewDbDraft(prev => ({
                          ...prev,
                          dbCategory: nextCat,
                          dbSoftware: (recs && recs.length > 0) ? recs[0] : prev.dbSoftware
                        }));
                      }}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    >
                      {EXCEL_DB_CATEGORIES.map(c => (
                        <option key={c} value={c}>{c}</option>
                      ))}
                    </select>
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>数据库软件 *</span>
                    <select
                      value={newDbDraft.dbSoftware}
                      onChange={e => setNewDbDraft(prev => ({ ...prev, dbSoftware: e.target.value }))}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    >
                      {EXCEL_DB_CATEGORY_SOFTWARE_MAP[newDbDraft.dbCategory]?.length ? (
                        <optgroup label={`★ 推荐与【${newDbDraft.dbCategory}】匹配的数据库`}>
                          {EXCEL_DB_CATEGORY_SOFTWARE_MAP[newDbDraft.dbCategory].map(s => (
                            <option key={`rec-db-${s}`} value={s}>{s}</option>
                          ))}
                        </optgroup>
                      ) : null}
                      <optgroup label="📋 Excel 00-代码表全部标准数据库 (共 69 项)">
                        {EXCEL_DB_SOFTWARES.map(s => (
                          <option key={`all-db-${s}`} value={s}>{s}</option>
                        ))}
                      </optgroup>
                    </select>
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>软件版本</span>
                    <input
                      value={newDbDraft.version}
                      onChange={e => setNewDbDraft(prev => ({ ...prev, version: e.target.value }))}
                      placeholder="例: V8.4 / 11.2.0.4"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>服务端口 *</span>
                    <input
                      type="number"
                      value={newDbDraft.port}
                      onChange={e => setNewDbDraft(prev => ({ ...prev, port: Number(e.target.value) }))}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>私有业务 IP *</span>
                    <input
                      value={newDbDraft.privateIp}
                      onChange={e => setNewDbDraft(prev => ({ ...prev, privateIp: e.target.value }))}
                      placeholder="例: 192.125.31.250"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>VIP / EIP</span>
                    <input
                      value={newDbDraft.vipEip}
                      onChange={e => setNewDbDraft(prev => ({ ...prev, vipEip: e.target.value }))}
                      placeholder="例: 192.125.31.100"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>实例名 / SID</span>
                    <input
                      value={newDbDraft.instanceSid}
                      onChange={e => setNewDbDraft(prev => ({ ...prev, instanceSid: e.target.value }))}
                      placeholder="例: DMSERVER / orcl1"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>业务库名 (DB Name)</span>
                    <input
                      value={newDbDraft.dbName}
                      onChange={e => setNewDbDraft(prev => ({ ...prev, dbName: e.target.value }))}
                      placeholder="例: biz_app_db"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>部署架构模式</span>
                    <select
                      value={newDbDraft.deployMode}
                      onChange={e => setNewDbDraft(prev => ({ ...prev, deployMode: e.target.value }))}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    >
                      {EXCEL_DB_DEPLOY_MODES.map(m => (
                        <option key={m} value={m}>{m}</option>
                      ))}
                    </select>
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>集群/组名</span>
                    <input
                      value={newDbDraft.clusterName}
                      onChange={e => setNewDbDraft(prev => ({ ...prev, clusterName: e.target.value }))}
                      placeholder="例: DM_RAC_PROD"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>

                  <label style={{ gridColumn: "span 2" }}>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>备注说明</span>
                    <textarea
                      value={newDbDraft.remarks}
                      onChange={e => setNewDbDraft(prev => ({ ...prev, remarks: e.target.value }))}
                      rows={2}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>
                </div>
              </div>
              <div className="cmdb-modal-footer" style={{ borderTop: "1px solid #e2e8f0", padding: "12px 20px", display: "flex", justifyContent: "flex-end", gap: 10 }}>
                <button type="button" className="btn-secondary" onClick={() => setShowAddDbModal(false)}>取 消</button>
                <button type="submit" className="btn-primary" style={{ background: "#4338ca", border: "none" }}>确 认 录 入</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* ================= MODAL: 04-MIDDLEWARE ADD ================= */}
      {showAddMwModal && (
        <div className="cmdb-modal-mask">
          <div className="cmdb-modal" style={{ maxWidth: 650, width: "95%" }}>
            <div className="cmdb-modal-header" style={{ background: "#d97706", color: "#fff", borderBottom: 0 }}>
              <h3 style={{ margin: 0, fontSize: 16, color: "#fff" }}>⚙️ 录入中间件服务档案 (Sheet 04)</h3>
              <button type="button" className="cmdb-modal-close" style={{ color: "#fff" }} onClick={() => setShowAddMwModal(false)}>×</button>
            </div>
            <form onSubmit={(e) => {
              e.preventDefault();
              const newRecord: any = {
                id: `mw-new-${Date.now()}`,
                seq: (middlewares?.length || 0) + 1,
                name: `${newMwDraft.mwSoftware}_${newMwDraft.port}`,
                mwType: newMwDraft.mwType,
                mwSoftware: newMwDraft.mwSoftware,
                version: newMwDraft.version,
                port: newMwDraft.port,
                role: newMwDraft.role,
                runtime: newMwDraft.runtime,
                status: "running",
                privateIp: newMwDraft.privateIp,
                assetIp: newMwDraft.privateIp,
                projectName: newMwDraft.projectName || currentProject?.name || projects[0]?.name,
                customerName: newMwDraft.customerName || currentProject?.customerName || projects[0]?.customerName,
                env: newMwDraft.env,
                cloudVendor: newMwDraft.cloudVendor,
                regionName: newMwDraft.regionName,
                remarks: newMwDraft.remarks
              };
              onAddMiddleware?.(newRecord);
              setShowAddMwModal(false);
              setToastNotice(`✓ 成功登记中间件服务【${newRecord.mwSoftware}】！`);
              setTimeout(() => setToastNotice(null), 3500);
            }}>
              <div className="cmdb-modal-body" style={{ maxHeight: "72vh", overflowY: "auto", padding: "16px 20px" }}>
                <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>所属项目 *</span>
                    <select
                      value={newMwDraft.projectName}
                      onChange={e => {
                        const p = projects.find(x => x.name === e.target.value);
                        setNewMwDraft(prev => ({
                          ...prev,
                          projectName: e.target.value,
                          customerName: p ? p.customerName : prev.customerName,
                          env: p ? p.env : prev.env,
                          cloudVendor: p ? p.cloudVendor : prev.cloudVendor,
                          regionName: p ? p.regionName : prev.regionName
                        }));
                      }}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    >
                      {projects.map(p => (
                        <option key={p.id} value={p.name}>{p.name}</option>
                      ))}
                    </select>
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>客户单位</span>
                    <input
                      value={newMwDraft.customerName}
                      onChange={e => setNewMwDraft(prev => ({ ...prev, customerName: e.target.value }))}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>中间件类型 *</span>
                    <select
                      value={newMwDraft.mwType}
                      onChange={e => {
                        const nextType = e.target.value;
                        const recs = EXCEL_MW_TYPE_SOFTWARE_MAP[nextType];
                        setNewMwDraft(prev => ({
                          ...prev,
                          mwType: nextType,
                          mwSoftware: (recs && recs.length > 0) ? recs[0] : prev.mwSoftware
                        }));
                      }}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    >
                      {EXCEL_MIDDLEWARE_TYPES.map(t => (
                        <option key={t} value={t}>{t}</option>
                      ))}
                    </select>
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>中间件软件名称 *</span>
                    <select
                      value={newMwDraft.mwSoftware}
                      onChange={e => setNewMwDraft(prev => ({ ...prev, mwSoftware: e.target.value }))}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    >
                      {EXCEL_MW_TYPE_SOFTWARE_MAP[newMwDraft.mwType]?.length ? (
                        <optgroup label={`★ 推荐与【${newMwDraft.mwType}】匹配的软件`}>
                          {EXCEL_MW_TYPE_SOFTWARE_MAP[newMwDraft.mwType].map(s => (
                            <option key={`rec-${s}`} value={s}>{s}</option>
                          ))}
                        </optgroup>
                      ) : null}
                      <optgroup label="📋 Excel 00-代码表全部标准中间件 (共 35 项)">
                        {EXCEL_MIDDLEWARE_SOFTWARES.map(s => (
                          <option key={`all-${s}`} value={s}>{s}</option>
                        ))}
                      </optgroup>
                    </select>
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>软件版本</span>
                    <input
                      value={newMwDraft.version}
                      onChange={e => setNewMwDraft(prev => ({ ...prev, version: e.target.value }))}
                      placeholder="例: 7.0.E / 1.20.1"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>服务端口</span>
                    <input
                      value={newMwDraft.port}
                      onChange={e => setNewMwDraft(prev => ({ ...prev, port: e.target.value }))}
                      placeholder="例: 8080 / 9092"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>私有业务 IP *</span>
                    <input
                      value={newMwDraft.privateIp}
                      onChange={e => setNewMwDraft(prev => ({ ...prev, privateIp: e.target.value }))}
                      placeholder="例: 192.125.31.250"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    >
                    </input>
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>应用程序运行环境 (Runtime) *</span>
                    <select
                      value={newMwDraft.runtime}
                      onChange={e => setNewMwDraft(prev => ({ ...prev, runtime: e.target.value }))}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    >
                      {EXCEL_APP_RUNTIMES.map(r => (
                        <option key={r} value={r}>{r}</option>
                      ))}
                      {newMwDraft.runtime && !EXCEL_APP_RUNTIMES.includes(newMwDraft.runtime as any) && (
                        <option value={newMwDraft.runtime}>{newMwDraft.runtime} (自定义)</option>
                      )}
                    </select>
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>节点角色</span>
                    <select
                      value={newMwDraft.role}
                      onChange={e => setNewMwDraft(prev => ({ ...prev, role: e.target.value }))}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    >
                      <option value="Server">Server (独立服务)</option>
                      <option value="Master/Broker">Master/Broker (主控)</option>
                      <option value="Worker/Node">Worker/Node (工作节点)</option>
                      <option value="Agent">Agent (客户端探针)</option>
                    </select>
                  </label>

                  <label style={{ gridColumn: "span 2" }}>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>备注说明</span>
                    <textarea
                      value={newMwDraft.remarks}
                      onChange={e => setNewMwDraft(prev => ({ ...prev, remarks: e.target.value }))}
                      rows={2}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>
                </div>
              </div>
              <div className="cmdb-modal-footer" style={{ borderTop: "1px solid #e2e8f0", padding: "12px 20px", display: "flex", justifyContent: "flex-end", gap: 10 }}>
                <button type="button" className="btn-secondary" onClick={() => setShowAddMwModal(false)}>取 消</button>
                <button type="submit" className="btn-primary" style={{ background: "#d97706", border: "none" }}>确 认 录 入</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* ================= MODAL: 05-BACKUP ADD ================= */}
      {showAddBkModal && (
        <div className="cmdb-modal-mask">
          <div className="cmdb-modal" style={{ maxWidth: 650, width: "95%" }}>
            <div className="cmdb-modal-header" style={{ background: "#059669", color: "#fff", borderBottom: 0 }}>
              <h3 style={{ margin: 0, fontSize: 16, color: "#fff" }}>💾 录入数据备份策略 (Sheet 05)</h3>
              <button type="button" className="cmdb-modal-close" style={{ color: "#fff" }} onClick={() => setShowAddBkModal(false)}>×</button>
            </div>
            <form onSubmit={(e) => {
              e.preventDefault();
              const newRecord: any = {
                id: `bk-new-${Date.now()}`,
                seq: (backups?.length || 0) + 1,
                projectName: newBkDraft.projectName || currentProject?.name || projects[0]?.name,
                customerName: newBkDraft.customerName || currentProject?.customerName || projects[0]?.customerName,
                env: newBkDraft.env,
                cloudVendor: newBkDraft.cloudVendor,
                regionName: newBkDraft.regionName,
                privateIp: newBkDraft.privateIp,
                backupType: newBkDraft.backupType,
                backupMethod: newBkDraft.backupMethod,
                backupPolicy: newBkDraft.backupPolicy,
                storageLocation: newBkDraft.storageLocation,
                retentionDays: Number(newBkDraft.retentionDays) || 30,
                lastBackupTime: "2026-09-21 03:00:00",
                status: "normal",
                remarks: newBkDraft.remarks
              };
              onAddBackup?.(newRecord);
              setShowAddBkModal(false);
              setToastNotice(`✓ 成功登记备份策略【${newRecord.backupType} - ${newRecord.backupPolicy}】！`);
              setTimeout(() => setToastNotice(null), 3500);
            }}>
              <div className="cmdb-modal-body" style={{ maxHeight: "72vh", overflowY: "auto", padding: "16px 20px" }}>
                <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>所属项目 *</span>
                    <select
                      value={newBkDraft.projectName}
                      onChange={e => {
                        const p = projects.find(x => x.name === e.target.value);
                        setNewBkDraft(prev => ({
                          ...prev,
                          projectName: e.target.value,
                          customerName: p ? p.customerName : prev.customerName,
                          env: p ? p.env : prev.env,
                          cloudVendor: p ? p.cloudVendor : prev.cloudVendor,
                          regionName: p ? p.regionName : prev.regionName
                        }));
                      }}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    >
                      {projects.map(p => (
                        <option key={p.id} value={p.name}>{p.name}</option>
                      ))}
                    </select>
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>客户单位</span>
                    <input
                      value={newBkDraft.customerName}
                      onChange={e => setNewBkDraft(prev => ({ ...prev, customerName: e.target.value }))}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>私有业务 IP *</span>
                    <input
                      value={newBkDraft.privateIp}
                      onChange={e => setNewBkDraft(prev => ({ ...prev, privateIp: e.target.value }))}
                      placeholder="例: 192.125.31.250"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>备份类型 *</span>
                    <select
                      value={newBkDraft.backupType}
                      onChange={e => setNewBkDraft(prev => ({ ...prev, backupType: e.target.value }))}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    >
                      {EXCEL_BACKUP_TYPES.map(t => (
                        <option key={t} value={t}>{t}</option>
                      ))}
                    </select>
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>备份方式 *</span>
                    <select
                      value={newBkDraft.backupMethod}
                      onChange={e => setNewBkDraft(prev => ({ ...prev, backupMethod: e.target.value }))}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    >
                      {EXCEL_BACKUP_METHODS.map(m => (
                        <option key={m} value={m}>{m}</option>
                      ))}
                    </select>
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>备份策略周期 *</span>
                    <select
                      value={newBkDraft.backupPolicy}
                      onChange={e => setNewBkDraft(prev => ({ ...prev, backupPolicy: e.target.value }))}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    >
                      {EXCEL_BACKUP_STRATEGIES.map(s => (
                        <option key={s} value={s}>{s}</option>
                      ))}
                    </select>
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>存储位置介质 *</span>
                    <input
                      value={newBkDraft.storageLocation}
                      onChange={e => setNewBkDraft(prev => ({ ...prev, storageLocation: e.target.value }))}
                      placeholder="例: 政务专区专用NAS / 对象存储桶"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>数据保留周期 (天) *</span>
                    <input
                      type="number"
                      value={newBkDraft.retentionDays}
                      onChange={e => setNewBkDraft(prev => ({ ...prev, retentionDays: Number(e.target.value) }))}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    />
                  </label>

                  <label style={{ gridColumn: "span 2" }}>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>备注说明</span>
                    <textarea
                      value={newBkDraft.remarks}
                      onChange={e => setNewBkDraft(prev => ({ ...prev, remarks: e.target.value }))}
                      rows={2}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>
                </div>
              </div>
              <div className="cmdb-modal-footer" style={{ borderTop: "1px solid #e2e8f0", padding: "12px 20px", display: "flex", justifyContent: "flex-end", gap: 10 }}>
                <button type="button" className="btn-secondary" onClick={() => setShowAddBkModal(false)}>取 消</button>
                <button type="submit" className="btn-primary" style={{ background: "#059669", border: "none" }}>确 认 录 入</button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* ================= MODAL: 06-OPS RECORD ADD ================= */}
      {showAddOpsModal && (
        <div className="cmdb-modal-mask">
          <div className="cmdb-modal" style={{ maxWidth: 700, width: "95%" }}>
            <div className="cmdb-modal-header" style={{ background: "#7c3aed", color: "#fff", borderBottom: 0 }}>
              <h3 style={{ margin: 0, fontSize: 16, color: "#fff" }}>🛡️ 录入运维保障规范 (Sheet 06)</h3>
              <button type="button" className="cmdb-modal-close" style={{ color: "#fff" }} onClick={() => setShowAddOpsModal(false)}>×</button>
            </div>
            <form onSubmit={(e) => {
              e.preventDefault();
              const newRecord: any = {
                id: `ops-new-${Date.now()}`,
                seq: (opsRecords?.length || 0) + 1,
                projectName: newOpsDraft.projectName || currentProject?.name || projects[0]?.name,
                customerName: newOpsDraft.customerName || currentProject?.customerName || projects[0]?.customerName,
                env: newOpsDraft.env,
                cloudVendor: newOpsDraft.cloudVendor,
                regionName: newOpsDraft.regionName,
                personnelAffiliation: newOpsDraft.personnelAffiliation || "伟仕",
                vpnNetworkEnv: newOpsDraft.vpnNetworkEnv || "互联网区",
                opsVendor: newOpsDraft.opsVendor,
                vpnAddress: newOpsDraft.vpnAddress,
                vpnAccount: newOpsDraft.vpnAccount,
                vpnUserName: newOpsDraft.vpnUserName,
                bastionAddress: newOpsDraft.bastionAddress,
                bastionAccount: newOpsDraft.bastionAccount,
                bastionUserName: newOpsDraft.bastionUserName,
                serverAccessAddress: newOpsDraft.serverAccessAddress || `ssh root@${newOpsDraft.privateIp}`,
                monitoringCoverage: newOpsDraft.monitoringCoverage,
                inspectionCycle: newOpsDraft.inspectionCycle,
                changeWindow: newOpsDraft.changeWindow,
                networkZone: newOpsDraft.networkZone,
                exposureSurface: newOpsDraft.exposureSurface,
                remarks: newOpsDraft.remarks
              };
              onAddOpsRecord?.(newRecord);
              setShowAddOpsModal(false);
              setToastNotice(`✓ 成功登记运维保障记录【${newRecord.opsVendor}】！`);
              setTimeout(() => setToastNotice(null), 3500);
            }}>
              <div className="cmdb-modal-body" style={{ maxHeight: "72vh", overflowY: "auto", padding: "16px 20px" }}>
                <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>所属项目 *</span>
                    <select
                      value={newOpsDraft.projectName}
                      onChange={e => {
                        const p = projects.find(x => x.name === e.target.value);
                        setNewOpsDraft(prev => ({
                          ...prev,
                          projectName: e.target.value,
                          customerName: p ? p.customerName : prev.customerName,
                          env: p ? p.env : prev.env,
                          cloudVendor: p ? p.cloudVendor : prev.cloudVendor,
                          regionName: p ? p.regionName : prev.regionName
                        }));
                      }}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    >
                      {projects.map(p => (
                        <option key={p.id} value={p.name}>{p.name}</option>
                      ))}
                    </select>
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>运维保障厂商 *</span>
                    <input
                      value={newOpsDraft.opsVendor}
                      onChange={e => setNewOpsDraft(prev => ({ ...prev, opsVendor: e.target.value }))}
                      placeholder="例: 北京北控伟仕软件有限公司"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>私有业务 IP *</span>
                    <input
                      value={newOpsDraft.privateIp}
                      onChange={e => setNewOpsDraft(prev => ({ ...prev, privateIp: e.target.value }))}
                      placeholder="例: 192.125.31.250"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                      required
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>人员归属 / 团队</span>
                    <input
                      value={newOpsDraft.personnelAffiliation}
                      onChange={e => setNewOpsDraft(prev => ({ ...prev, personnelAffiliation: e.target.value }))}
                      placeholder="例: 伟仕 / 驻场运维组"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>使用人网络环境</span>
                    <input
                      list="excel-regions-ops"
                      value={newOpsDraft.vpnNetworkEnv}
                      onChange={e => setNewOpsDraft(prev => ({ ...prev, vpnNetworkEnv: e.target.value }))}
                      placeholder="例: 互联网区 / 政务外网区"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                    <datalist id="excel-regions-ops">
                      {EXCEL_REGION_NAMES.map(r => (
                        <option key={r} value={r}>{r}</option>
                      ))}
                    </datalist>
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>VPN接入地址</span>
                    <input
                      value={newOpsDraft.vpnAddress}
                      onChange={e => setNewOpsDraft(prev => ({ ...prev, vpnAddress: e.target.value }))}
                      placeholder="例: 114.255.48.18:443"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>VPN账号</span>
                    <input
                      value={newOpsDraft.vpnAccount}
                      onChange={e => setNewOpsDraft(prev => ({ ...prev, vpnAccount: e.target.value }))}
                      placeholder="例: vpn_ops_user"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#2563eb" }}>👤 VPN使用人姓名 *</span>
                    <input
                      value={newOpsDraft.vpnUserName}
                      onChange={e => setNewOpsDraft(prev => ({ ...prev, vpnUserName: e.target.value }))}
                      placeholder="例: 翟焕净 / 张三"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12, borderColor: "#93c5fd" }}
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>堡垒机(JumpServer)地址</span>
                    <input
                      value={newOpsDraft.bastionAddress}
                      onChange={e => setNewOpsDraft(prev => ({ ...prev, bastionAddress: e.target.value }))}
                      placeholder="例: https://jumpserver.bj-gov.cn:443"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>堡垒机账号</span>
                    <input
                      value={newOpsDraft.bastionAccount}
                      onChange={e => setNewOpsDraft(prev => ({ ...prev, bastionAccount: e.target.value }))}
                      placeholder="例: ops_admin"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#7c3aed" }}>👤 堡垒机使用人姓名</span>
                    <input
                      value={newOpsDraft.bastionUserName}
                      onChange={e => setNewOpsDraft(prev => ({ ...prev, bastionUserName: e.target.value }))}
                      placeholder="例: 翟焕净 / 张三"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12, borderColor: "#c4b5fd" }}
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>服务器访问命令</span>
                    <input
                      value={newOpsDraft.serverAccessAddress}
                      onChange={e => setNewOpsDraft(prev => ({ ...prev, serverAccessAddress: e.target.value }))}
                      placeholder="例: ssh root@192.125.31.250 -p 22"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>监控覆盖</span>
                    <select
                      value={newOpsDraft.monitoringCoverage}
                      onChange={e => setNewOpsDraft(prev => ({ ...prev, monitoringCoverage: e.target.value as any }))}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    >
                      <option value="是">是 (全面接入 Prometheus/Zabbix)</option>
                      <option value="部分">部分 (仅主机基础监控)</option>
                      <option value="否">否 (未纳入监控)</option>
                    </select>
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>巡检周期</span>
                    <select
                      value={newOpsDraft.inspectionCycle}
                      onChange={e => setNewOpsDraft(prev => ({ ...prev, inspectionCycle: e.target.value as any }))}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    >
                      <option value="每日">每日</option>
                      <option value="每周">每周</option>
                      <option value="每月">每月</option>
                      <option value="每季">每季</option>
                    </select>
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>变更窗口</span>
                    <input
                      value={newOpsDraft.changeWindow}
                      onChange={e => setNewOpsDraft(prev => ({ ...prev, changeWindow: e.target.value }))}
                      placeholder="例: 周五晚22:00后 / 工作日夜间"
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>

                  <label>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>网络分区与暴露面</span>
                    <div style={{ display: "flex", gap: 6 }}>
                      <input
                        value={newOpsDraft.networkZone}
                        onChange={e => setNewOpsDraft(prev => ({ ...prev, networkZone: e.target.value }))}
                        placeholder="网络分区(业务网)"
                        style={{ flex: 1, padding: "6px 8px", fontSize: 12 }}
                      />
                      <select
                        value={newOpsDraft.exposureSurface}
                        onChange={e => setNewOpsDraft(prev => ({ ...prev, exposureSurface: e.target.value }))}
                        style={{ width: 110, padding: "6px 8px", fontSize: 12 }}
                      >
                        <option value="业务内网">业务内网</option>
                        <option value="内部大网">内部大网</option>
                        <option value="公网暴露">公网暴露</option>
                      </select>
                    </div>
                  </label>

                  <label style={{ gridColumn: "span 2" }}>
                    <span style={{ fontSize: 12, fontWeight: 600, color: "#475569" }}>备注说明</span>
                    <textarea
                      value={newOpsDraft.remarks}
                      onChange={e => setNewOpsDraft(prev => ({ ...prev, remarks: e.target.value }))}
                      rows={2}
                      style={{ width: "100%", padding: "6px 8px", fontSize: 12 }}
                    />
                  </label>
                </div>
              </div>
              <div className="cmdb-modal-footer" style={{ borderTop: "1px solid #e2e8f0", padding: "12px 20px", display: "flex", justifyContent: "flex-end", gap: 10 }}>
                <button type="button" className="btn-secondary" onClick={() => setShowAddOpsModal(false)}>取 消</button>
                <button type="submit" className="btn-primary" style={{ background: "#7c3aed", border: "none" }}>确 认 录 入</button>
              </div>
            </form>
          </div>
        </div>
      )}

{/* ================= MODAL: ADD PROJECT ================= */}
      {showAddProjectModal && (
        <div className="cmdb-modal-mask">
          <form className="cmdb-modal" style={{ maxWidth: 640, width: "95%" }} onSubmit={handleAddProjectSubmit}>
            <div className="cmdb-modal-header">
              <h3>📁 录入新业务项目</h3>
              <button type="button" className="cmdb-modal-close" onClick={() => setShowAddProjectModal(false)}>×</button>
            </div>
            <div className="cmdb-modal-body" style={{ maxHeight: "72vh", overflowY: "auto", padding: 20 }}>
              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 项目名称</label>
                  <input 
                    placeholder="如：北京市医疗保障信息平台"
                    value={newProjectForm.name}
                    onChange={e => setNewProjectForm({ ...newProjectForm, name: e.target.value })}
                    required
                  />
                </div>
                <div className="form-field-item">
                  <label>* 项目编号</label>
                  <input 
                    placeholder="如：prj-024"
                    value={newProjectForm.code}
                    onChange={e => setNewProjectForm({ ...newProjectForm, code: e.target.value })}
                    required
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 客户单位名称</label>
                  <input 
                    list="excel-customers-list"
                    placeholder="从代码表选择或输入，如：北京市人力资源和社会保障局"
                    value={newProjectForm.customerName}
                    onChange={e => setNewProjectForm({ ...newProjectForm, customerName: e.target.value })}
                    required
                  />
                  <datalist id="excel-customers-list">
                    {EXCEL_CUSTOMERS.map(c => (
                      <option key={c} value={c}>{c}</option>
                    ))}
                  </datalist>
                </div>
                <div className="form-field-item">
                  <label>* 所属环境</label>
                  <select 
                    value={newProjectForm.env}
                    onChange={e => setNewProjectForm({ ...newProjectForm, env: e.target.value })}
                  >
                    {EXCEL_ENVIRONMENTS.map(env => (
                      <option key={env} value={env}>{env}环境</option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 承载云厂商</label>
                  <select 
                    value={newProjectForm.cloudVendor}
                    onChange={e => setNewProjectForm({ ...newProjectForm, cloudVendor: e.target.value })}
                  >
                    {EXCEL_CLOUD_VENDORS.map(v => (
                      <option key={v} value={v}>{v}</option>
                    ))}
                  </select>
                </div>
                <div className="form-field-item">
                  <label>* 部署网络区域</label>
                  <input 
                    list="excel-regions-list"
                    placeholder="从代码表选择或输入，如：政务外网区"
                    value={newProjectForm.regionName}
                    onChange={e => setNewProjectForm({ ...newProjectForm, regionName: e.target.value })}
                    required
                  />
                  <datalist id="excel-regions-list">
                    {EXCEL_REGION_NAMES.map(r => (
                      <option key={r} value={r}>{r}</option>
                    ))}
                  </datalist>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item" style={{ gridColumn: "span 2" }}>
                  <label>业务系统描述与说明</label>
                  <textarea 
                    rows={2}
                    placeholder="如：负责全市医保定点联网结算及微服务支撑业务"
                    value={newProjectForm.description}
                    onChange={e => setNewProjectForm({ ...newProjectForm, description: e.target.value })}
                  />
                </div>
              </div>
            </div>
            <div className="cmdb-modal-footer">
              <button type="button" className="btn-secondary" onClick={() => setShowAddProjectModal(false)}>取 消</button>
              <button type="submit" className="btn-primary">✓ 确认创建项目</button>
            </div>
          </form>
        </div>
      )}

      {/* ================= MODAL: EDIT ASSET ================= */}
      {editingAsset && (
        <div className="cmdb-modal-mask">
          <form className="cmdb-modal" style={{ maxWidth: 740, width: "95%" }} onSubmit={handleEditSubmit}>
            <div className="cmdb-modal-header">
              <h3>✏️ 修改项目资产信息【{editingAsset.name}】</h3>
              <button type="button" className="cmdb-modal-close" onClick={() => setEditingAsset(null)}>×</button>
            </div>
            <div className="cmdb-modal-body" style={{ maxHeight: "72vh", overflowY: "auto", padding: 20 }}>
              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 所属项目</label>
                  <select 
                    value={editForm.projectName}
                    onChange={e => {
                      const p = projects.find(x => x.name === e.target.value);
                      if (p) {
                        setEditForm({
                          ...editForm,
                          projectName: p.name,
                          customerName: p.customerName,
                          env: p.env,
                          cloudVendor: p.cloudVendor,
                          regionName: p.regionName
                        });
                      }
                    }}
                  >
                    {projects.map(p => (
                      <option key={p.id} value={p.name}>{p.name} ({p.customerName})</option>
                    ))}
                  </select>
                </div>
                <div className="form-field-item">
                  <label>* 设备名称</label>
                  <input 
                    value={editForm.name}
                    onChange={e => setEditForm({ ...editForm, name: e.target.value })}
                    required
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 客户名称</label>
                  <input 
                    value={editForm.customerName}
                    onChange={e => setEditForm({ ...editForm, customerName: e.target.value })}
                    required
                  />
                </div>
                <div className="form-field-item">
                  <label>* 环境</label>
                  <select value={editForm.env} onChange={e => setEditForm({ ...editForm, env: e.target.value })}>
                    {EXCEL_ENVIRONMENTS.map(env => (
                      <option key={env} value={env}>{env}环境</option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 云厂商</label>
                  <select value={editForm.cloudVendor} onChange={e => setEditForm({ ...editForm, cloudVendor: e.target.value })}>
                    {EXCEL_CLOUD_VENDORS.map(v => (
                      <option key={v} value={v}>{v}</option>
                    ))}
                  </select>
                </div>
                <div className="form-field-item">
                  <label>* 设备形态</label>
                  <select value={editForm.deviceType} onChange={e => setEditForm({ ...editForm, deviceType: e.target.value })}>
                    {EXCEL_DEVICE_TYPES.map(d => (
                      <option key={d} value={d}>{d}</option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 私有IP（业务IP）</label>
                  <input 
                    value={editForm.privateIp}
                    onChange={e => setEditForm({ ...editForm, privateIp: e.target.value })}
                    required
                  />
                </div>
                <div className="form-field-item">
                  <label>内大网IP</label>
                  <input 
                    value={editForm.internalWanIp}
                    onChange={e => setEditForm({ ...editForm, internalWanIp: e.target.value })}
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>VIP (虚IP)</label>
                  <input 
                    value={editForm.vip}
                    onChange={e => setEditForm({ ...editForm, vip: e.target.value })}
                  />
                </div>
                <div className="form-field-item">
                  <label>EIP / 公网IP</label>
                  <input 
                    value={editForm.eip}
                    onChange={e => setEditForm({ ...editForm, eip: e.target.value })}
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>CPU 架构与核数</label>
                  <div style={{ display: "flex", gap: 8 }}>
                    <select value={editForm.cpuArch} onChange={e => setEditForm({ ...editForm, cpuArch: e.target.value })}>
                      {EXCEL_CPU_ARCHS.map(a => (
                        <option key={a} value={a}>{a}</option>
                      ))}
                    </select>
                    <input 
                      type="number" 
                      placeholder="核数" 
                      value={editForm.cpuCores} 
                      onChange={e => setEditForm({ ...editForm, cpuCores: Number(e.target.value) })} 
                    />
                  </div>
                </div>
                <div className="form-field-item">
                  <label>内存 (GB)</label>
                  <input 
                    type="number" 
                    value={editForm.memoryGb} 
                    onChange={e => setEditForm({ ...editForm, memoryGb: Number(e.target.value) })} 
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>系统盘 (GB)</label>
                  <input 
                    type="number" 
                    value={editForm.systemDiskGb} 
                    onChange={e => setEditForm({ ...editForm, systemDiskGb: Number(e.target.value) })} 
                  />
                </div>
                <div className="form-field-item">
                  <label>数据盘 (GB)</label>
                  <input 
                    type="number" 
                    value={editForm.dataDiskGb} 
                    onChange={e => setEditForm({ ...editForm, dataDiskGb: Number(e.target.value) })} 
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>操作系统版本</label>
                  <input 
                    list="excel-os-versions-edit"
                    placeholder="从代码表选择或输入，如 麒麟V10 SP3"
                    value={editForm.osVersion} 
                    onChange={e => setEditForm({ ...editForm, osVersion: e.target.value })} 
                  />
                  <datalist id="excel-os-versions-edit">
                    {EXCEL_OS_VERSIONS.map(os => (
                      <option key={os} value={os}>{os}</option>
                    ))}
                  </datalist>
                </div>
                <div className="form-field-item">
                  <label>内核版本</label>
                  <input 
                    value={editForm.kernelVersion} 
                    onChange={e => setEditForm({ ...editForm, kernelVersion: e.target.value })} 
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>国产信创操作系统</label>
                  <select value={editForm.isXinchuang} onChange={e => setEditForm({ ...editForm, isXinchuang: e.target.value })}>
                    <option value="否">常规 OS (CentOS / RedHat)</option>
                    <option value="是">国产信创 OS (麒麟 Kylin / 统信 UOS)</option>
                  </select>
                </div>
                <div className="form-field-item">
                  <label>远程端口</label>
                  <input 
                    type="number" 
                    value={editForm.remotePort} 
                    onChange={e => setEditForm({ ...editForm, remotePort: Number(e.target.value) })} 
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item" style={{ gridColumn: "span 2" }}>
                  <label>备注说明</label>
                  <textarea 
                    rows={2} 
                    value={editForm.remarks} 
                    onChange={e => setEditForm({ ...editForm, remarks: e.target.value })} 
                  />
                </div>
              </div>
            </div>
            <div className="cmdb-modal-footer">
              <button type="button" className="btn-secondary" onClick={() => setEditingAsset(null)}>取 消</button>
              <button type="submit" className="btn-primary">✓ 保存修改</button>
            </div>
          </form>
        </div>
      )}

      {/* ================= MODAL: DELETE CONFIRM ================= */}
      {deletingAsset && (
        <div className="cmdb-modal-mask">
          <div className="cmdb-modal" style={{ maxWidth: 460, width: "90%" }}>
            <div className="cmdb-modal-header" style={{ background: "#fef2f2", borderBottom: "1px solid #fee2e2" }}>
              <h3 style={{ color: "#b91c1c", display: "flex", alignItems: "center", gap: 6 }}>
                <span>⚠️</span>
                <span>确认删除资产</span>
              </h3>
              <button type="button" className="cmdb-modal-close" onClick={() => setDeletingAsset(null)}>×</button>
            </div>
            <div className="cmdb-modal-body" style={{ padding: 20 }}>
              <p style={{ margin: "0 0 12px", fontSize: 13, color: "#334155", lineHeight: 1.6 }}>
                您确定要从资产台账中删除以下设备吗？该操作将从 CMDB 中永久注销该节点并扣减相应项目的资源池统计。
              </p>
              <div style={{ background: "#f8fafc", border: "1px solid #e2e8f0", borderRadius: 6, padding: "10px 14px", fontSize: 12 }}>
                <div style={{ marginBottom: 4 }}>
                  <span style={{ color: "#64748b" }}>设备名称: </span>
                  <strong style={{ color: "#0f172a" }}>{deletingAsset.name}</strong>
                </div>
                <div style={{ marginBottom: 4 }}>
                  <span style={{ color: "#64748b" }}>业务 IP: </span>
                  <code style={{ color: "#2563eb", fontWeight: 600 }}>{deletingAsset.privateIp || deletingAsset.ip || "-"}</code>
                </div>
                <div>
                  <span style={{ color: "#64748b" }}>所属项目: </span>
                  <strong>{deletingAsset.projectName}</strong>
                </div>
              </div>
            </div>
            <div className="cmdb-modal-footer">
              <button type="button" className="btn-secondary" onClick={() => setDeletingAsset(null)}>取 消</button>
              <button 
                type="button" 
                className="btn-primary" 
                style={{ background: "#dc2626", borderColor: "#b91c1c" }}
                onClick={handleConfirmDelete}
              >
                ✓ 确认注销删除
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ================= MODAL: IP ADDRESS QUERY & BATCH LOOKUP ================= */}
      {showIpModal && (
        <div className="cmdb-modal-mask" style={{ position: "fixed", inset: 0, zIndex: 99999, background: "rgba(15, 23, 42, 0.65)", display: "flex", alignItems: "center", justifyContent: "center" }}>
          <div className="cmdb-modal" style={{ width: 850, maxWidth: "95vw", maxHeight: "90vh", display: "flex", flexDirection: "column" }}>
            <div className="cmdb-modal-header" style={{ display: "flex", justifyContent: "space-between", alignItems: "center", borderBottom: "1px solid #e2e8f0", padding: "14px 20px" }}>
              <h3 style={{ margin: 0, fontSize: 16, display: "flex", alignItems: "center", gap: 8, color: "#0f172a" }}>
                <span>🌐</span>
                <span>IP 地址精准检索与批量比对</span>
                <span style={{ fontSize: 11, background: "#ecfdf5", color: "#059669", padding: "2px 8px", borderRadius: 4, fontWeight: 500 }}>
                  支持单IP / 批量多IP / 智能网段
                </span>
              </h3>
              <button type="button" className="cmdb-modal-close" onClick={() => setShowIpModal(false)}>×</button>
            </div>

            <div className="cmdb-modal-body" style={{ padding: "16px 20px", flex: 1, overflowY: "auto", display: "flex", flexDirection: "column", gap: 14 }}>
              <div>
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 6 }}>
                  <label style={{ fontSize: 12, fontWeight: 600, color: "#334155" }}>
                    输入待查询的 IP 地址列表 (支持换行、逗号或空格分隔)：
                  </label>
                  <div style={{ display: "flex", gap: 8 }}>
                    <button
                      type="button"
                      className="btn-secondary"
                      style={{ fontSize: 11, padding: "2px 8px" }}
                      onClick={async () => {
                        try {
                          const text = await navigator.clipboard?.readText();
                          if (text) setIpBatchText(text.trim());
                        } catch (e) {
                          // ignore
                        }
                      }}
                    >
                      📋 粘贴剪贴板内容
                    </button>
                    <button
                      type="button"
                      className="btn-secondary"
                      style={{ fontSize: 11, padding: "2px 8px", color: "#dc2626" }}
                      onClick={() => setIpBatchText("")}
                    >
                      ✕ 清空
                    </button>
                  </div>
                </div>
                <textarea
                  rows={4}
                  placeholder={`例如输入单个或多个 IP，亦可输入前缀网段：\n10.150.88.10\n10.150.88.11\n192.125.31.250\n10.150.`}
                  value={ipBatchText}
                  onChange={e => setIpBatchText(e.target.value)}
                  style={{
                    width: "100%",
                    fontSize: 12,
                    fontFamily: "monospace",
                    padding: "8px 10px",
                    borderRadius: 6,
                    border: "1.5px solid #cbd5e1",
                    resize: "vertical"
                  }}
                />
              </div>

              {/* Analysis Stats Bar */}
              {batchIpAnalysis && (
                <div style={{
                  background: "#f8fafc",
                  border: "1px solid #e2e8f0",
                  borderRadius: 6,
                  padding: "10px 14px",
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                  flexWrap: "wrap",
                  gap: 10
                }}>
                  <div style={{ display: "flex", gap: 14, alignItems: "center" }}>
                    <div style={{ fontSize: 12, color: "#64748b" }}>
                      输入 IP 数量: <strong style={{ color: "#0f172a" }}>{batchIpAnalysis.totalInputs}</strong>
                    </div>
                    <div style={{ fontSize: 12, color: "#166534" }}>
                      已匹配命中: <strong style={{ color: "#16a34a" }}>{batchIpAnalysis.matchedIpCount} 个 IP ({batchIpAnalysis.matchedAssets.length} 台设备)</strong>
                    </div>
                    {batchIpAnalysis.notFoundIps.length > 0 && (
                      <div style={{ fontSize: 12, color: "#b91c1c" }}>
                        未录入/未找到: <strong style={{ color: "#dc2626" }}>{batchIpAnalysis.notFoundIps.length} 个</strong>
                      </div>
                    )}
                  </div>

                  <div style={{ display: "flex", gap: 6 }}>
                    <button
                      type="button"
                      className="btn-secondary"
                      style={{ fontSize: 11, padding: "3px 8px" }}
                      onClick={() => {
                        const summary = batchIpAnalysis.matchedAssets.map(a => 
                          `${a.name}\t${a.privateIp || (a as any).ip}\t${a.projectName}\t${a.customerName}\t${a.osVersion || (a as any).os}`
                        ).join("\n");
                        navigator.clipboard?.writeText?.(summary);
                        setToastNotice("✓ 已复制匹配资产列表到剪贴板！");
                        setTimeout(() => setToastNotice(null), 3000);
                      }}
                    >
                      📋 复制匹配结果
                    </button>
                    <button
                      type="button"
                      className="btn-primary"
                      style={{ fontSize: 11, padding: "3px 10px" }}
                      onClick={() => {
                        setSelectedProjectId("all");
                        setIpSearchKeyword(ipBatchText.trim());
                        setIpSearchInput(ipBatchText.trim().replace(/[\r\n]+/g, " "));
                        setShowIpModal(false);
                        setCurrentPage(1);
                        setToastNotice(`✓ 已在全量台账中定位这 ${batchIpAnalysis.matchedAssets.length} 台资产！`);
                        setTimeout(() => setToastNotice(null), 3500);
                      }}
                    >
                      📍 在台账列表中定位这批设备 ({batchIpAnalysis.matchedAssets.length}台)
                    </button>
                  </div>
                </div>
              )}

              {/* Matched Assets Table */}
              {batchIpAnalysis && batchIpAnalysis.matchedAssets.length > 0 && (
                <div style={{ border: "1px solid #e2e8f0", borderRadius: 6, overflow: "hidden", maxHeight: 280, overflowY: "auto" }}>
                  <table className="cmdb-data-table" style={{ fontSize: 11, width: "100%" }}>
                    <thead style={{ position: "sticky", top: 0, zIndex: 2 }}>
                      <tr>
                        <th style={{ width: 40 }}>序号</th>
                        <th style={{ width: 160 }}>设备名称</th>
                        <th style={{ width: 130 }}>业务私有 IP</th>
                        <th style={{ width: 110 }}>内大网 / VIP</th>
                        <th style={{ width: 160 }}>所属项目 · 客户单位</th>
                        <th style={{ width: 120 }}>操作系统</th>
                        <th style={{ width: 120, textAlign: "center" }}>操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      {batchIpAnalysis.matchedAssets.map((asset, idx) => (
                        <tr key={asset.id}>
                          <td style={{ fontFamily: "monospace", color: "#64748b" }}>{idx + 1}</td>
                          <td>
                            <strong
                              style={{ color: "#1d4ed8", cursor: "pointer", textDecoration: "underline", textUnderlineOffset: 2 }}
                              onClick={() => {
                                const targetProj = projects.find(p => p.name === asset.projectName);
                                if (targetProj) setSelectedProjectId(targetProj.id);
                                setDetailAsset(asset);
                                setDetailTab("spec");
                                setShowIpModal(false);
                              }}
                              title="点击直接打开该设备的详细信息档案 (规格/软件/通道/VPN)"
                            >
                              {asset.name}
                            </strong>
                          </td>
                          <td>
                            <code
                              style={{ color: "#2563eb", fontWeight: 600, cursor: "pointer", textDecoration: "underline", textUnderlineOffset: 2 }}
                              onClick={() => {
                                const targetProj = projects.find(p => p.name === asset.projectName);
                                if (targetProj) setSelectedProjectId(targetProj.id);
                                setDetailAsset(asset);
                                setDetailTab("spec");
                                setShowIpModal(false);
                              }}
                              title="点击直接打开该设备的详细信息档案"
                            >
                              {asset.privateIp || asset.ip || "-"}
                            </code>
                          </td>
                          <td>
                            <code style={{ color: "#64748b" }}>{asset.internalWanIp || asset.vip || "-"}</code>
                          </td>
                          <td>
                            <div>{asset.projectName}</div>
                            <small style={{ color: "#94a3b8" }}>{asset.customerName}</small>
                          </td>
                          <td>{asset.osVersion || (asset as any).os || "-"}</td>
                          <td style={{ textAlign: "center" }}>
                            <div style={{ display: "flex", gap: 4, justifyContent: "center" }}>
                              <button
                                type="button"
                                className="btn-primary"
                                style={{ fontSize: 10, padding: "2px 7px", whiteSpace: "nowrap" }}
                                onClick={() => {
                                  const targetProj = projects.find(p => p.name === asset.projectName);
                                  if (targetProj) setSelectedProjectId(targetProj.id);
                                  setDetailAsset(asset);
                                  setDetailTab("spec");
                                  setShowIpModal(false);
                                }}
                                title="立即打开该设备的详细信息档案 (规格/软件/通道/VPN)"
                              >
                                📖 详情
                              </button>
                              <button
                                type="button"
                                className="btn-secondary"
                                style={{ fontSize: 10, padding: "2px 6px", whiteSpace: "nowrap" }}
                                onClick={() => {
                                  const targetProj = projects.find(p => p.name === asset.projectName);
                                  if (targetProj) setSelectedProjectId(targetProj.id);
                                  else setSelectedProjectId("all");
                                  setIpSearchKeyword(asset.privateIp || asset.ip || "");
                                  setIpSearchInput(asset.privateIp || asset.ip || "");
                                  setShowIpModal(false);
                                  setCurrentPage(1);
                                }}
                                title="点击在工作台主列表中定位并筛选该设备"
                              >
                                📍 定位
                              </button>
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}

              {/* Not Found IPs List */}
              {batchIpAnalysis && batchIpAnalysis.notFoundIps.length > 0 && (
                <div style={{ background: "#fef2f2", border: "1px solid #fee2e2", borderRadius: 6, padding: "8px 12px", fontSize: 11 }}>
                  <span style={{ color: "#dc2626", fontWeight: 600 }}>未在现有 292 台台账中检索到的 IP ({batchIpAnalysis.notFoundIps.length}个): </span>
                  <span style={{ color: "#b91c1c", fontFamily: "monospace" }}>{batchIpAnalysis.notFoundIps.join(", ")}</span>
                </div>
              )}
            </div>

            <div className="cmdb-modal-footer" style={{ borderTop: "1px solid #e2e8f0", padding: "12px 20px", display: "flex", justifyContent: "flex-end" }}>
              <button type="button" className="btn-secondary" onClick={() => setShowIpModal(false)}>关 闭</button>
            </div>
          </div>
        </div>
      )}

      {/* 全局操作浮层通知 */}
      {toastNotice && (
        <div style={{
          position: "fixed",
          bottom: 24,
          right: 24,
          background: "#0f172a",
          color: "#fff",
          padding: "10px 18px",
          borderRadius: 8,
          boxShadow: "0 10px 25px rgba(0,0,0,0.3)",
          fontSize: 13,
          fontWeight: 600,
          zIndex: 99999,
          display: "flex",
          alignItems: "center",
          gap: 8
        }}>
          <span style={{ color: "#4ade80", fontSize: 16 }}>✓</span>
          <span>{toastNotice}</span>
        </div>
      )}

    </div>
  );
}