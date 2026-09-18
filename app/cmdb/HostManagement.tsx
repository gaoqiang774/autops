"use client";
import React, { useState, useMemo } from "react";
import { PhysicalHost, VmHost, SwitchDevice, DatabaseAsset, ProjectGroup, AssetMeta, SoftwareComponent, OpsChannel } from "../cmdbData";
import { exportAssetsToExcel, getAssetKey } from "./excelExport";
import ImportModal, { ImportStrategy } from "./ImportModal";

interface HostManagementProps {
  projects: ProjectGroup[];
  hosts: PhysicalHost[];
  vms: VmHost[];
  switches?: SwitchDevice[];
  databases?: DatabaseAsset[];
  softwareList?: SoftwareComponent[];
  channelList?: OpsChannel[];
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
  onDeduplicateAssets?: () => { removedCount: number };
  onUpdateVm?: (v: VmHost) => void;
  onDeleteUnifiedAsset?: (id: string, kind: "physical" | "vm" | "switch") => void;
  onAddProject?: (p: ProjectGroup) => void;
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
  onAddProject
}: HostManagementProps) {
  // Tabs & Modal States for Software, Ops Channels, and VPN
  const [detailTab, setDetailTab] = useState<"spec" | "software" | "ops" | "vpn">("spec");
  const [copiedNotice, setCopiedNotice] = useState<string | null>(null);
  const [showAddSoftForm, setShowAddSoftForm] = useState(false);
  const [showImportModal, setShowImportModal] = useState(false);
  const [toastNotice, setToastNotice] = useState<string | null>(null);

  function handleExportExcel() {
    const scopeTitle = currentProject ? currentProject.name : "全量项目总览";
    exportAssetsToExcel(displayedAssets, scopeTitle);
    setToastNotice(`✓ 已成功导出 ${displayedAssets.length} 台资产到《02-硬件设备》Excel！`);
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
  // Selected project ID ("all" for all assets)
  const [selectedProjectId, setSelectedProjectId] = useState<string>(projects[0]?.id || "all");
  
  // Left Sidebar Project Search & Filter
  const [projectSearch, setProjectSearch] = useState("");
  const [selectedCustomerFilter, setSelectedCustomerFilter] = useState("全部");

  // Right Table Filters
  const [assetKeyword, setAssetKeyword] = useState("");
  const [ipSearchInput, setIpSearchInput] = useState("");
  const [ipSearchKeyword, setIpSearchKeyword] = useState("");
  const [deviceTypeFilter, setDeviceTypeFilter] = useState("全部");
  const [xinchuangFilter, setXinchuangFilter] = useState("全部");

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

  // Duplicate device detection across all assets
  const duplicateCount = useMemo(() => {
    const keys = new Set<string>();
    let dups = 0;
    for (const item of allAssets) {
      const k = getAssetKey(item);
      if (keys.has(k)) {
        dups++;
      } else {
        keys.add(k);
      }
    }
    return dups;
  }, [allAssets]);

  function handleTriggerDeduplicate() {
    if (onDeduplicateAssets) {
      const res = onDeduplicateAssets();
      setToastNotice(`✨ 去重完成！已成功清理 ${res.removedCount} 台重复资产，项目台账已重新校准！`);
      setTimeout(() => setToastNotice(null), 4000);
    }
  }

  // Add Project Modal State
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
    const rawCpu = item.cpu || "";
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
      memoryGb: item.memoryGb || parseInt(item.memory || "16", 10) || 16,
      systemDiskGb: item.systemDiskGb || 50,
      dataDiskGb: item.dataDiskGb || 100,
      osFamily: item.osFamily || "CentOS",
      osVersion: item.osVersion || item.os || "CentOS 7.9",
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

      // Keyword search
      if (assetKeyword.trim()) {
        const kw = assetKeyword.toLowerCase();
        const str = [
          item.name,
          (item as any).hostname,
          item.privateIp,
          item.internalWanIp,
          item.vip,
          item.eip,
          item.remarks,
          item.osVersion,
          item.customerName,
          item.projectName
        ].filter(Boolean).join(" ").toLowerCase();

        if (!str.includes(kw)) return false;
      }

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
  }, [allAssets, selectedProjectId, currentProject, deviceTypeFilter, xinchuangFilter, assetKeyword, ipSearchKeyword]);

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

    const newSeq = (allAssets[allAssets.length - 1]?.seq || 238) + 1;
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
      internalWanIp: newForm.internalWanIp || null,
      eip: newForm.eip || null,
      vip: newForm.vip || null,
      publicIp: null,
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
        width: 290,
        minWidth: 290,
        background: "#fff",
        border: "1px solid #e2e8f0",
        borderRadius: 8,
        display: "flex",
        flexDirection: "column",
        overflow: "hidden",
        boxShadow: "0 1px 3px rgba(0,0,0,0.05)"
      }}>
        {/* Left Header */}
        <div style={{ padding: "12px 14px", borderBottom: "1px solid #e2e8f0", background: "#f8fafc" }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
            <span style={{ fontWeight: 700, color: "#0f172a", fontSize: 14 }}>
              📁 项目资产目录
            </span>
            <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
              <span style={{ fontSize: 11, color: "#64748b", background: "#e2e8f0", padding: "1px 6px", borderRadius: 10 }}>
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
                  cursor: "pointer"
                }}
                title="录入并创建新的业务项目单位"
              >
                <span>＋</span>
                <span>录入项目</span>
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
              查看跨项目 238 台信息资产台账全集
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
      </div>

      {/* ================= RIGHT WORKBENCH: ASSET TABLE ================= */}
      <div style={{ flex: 1, display: "flex", flexDirection: "column", minWidth: 0, gap: 10 }}>
        {/* Project Context Header Banner */}
        <div style={{
          background: "#fff",
          border: "1px solid #e2e8f0",
          borderRadius: 8,
          padding: "12px 18px",
          boxShadow: "0 1px 3px rgba(0,0,0,0.04)"
        }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", flexWrap: "wrap", gap: 10 }}>
            <div>
              <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                <span style={{
                  background: "#2563eb",
                  color: "#fff",
                  fontSize: 12,
                  padding: "2px 8px",
                  borderRadius: 4,
                  fontWeight: 600
                }}>
                  {currentProject ? currentProject.code : "ALL-ASSETS"}
                </span>
                <h3 style={{ margin: 0, fontSize: 17, color: "#0f172a" }}>
                  {currentProject ? currentProject.name : "跨项目全量资产台账总表"}
                </h3>
                {currentProject && (
                  <span style={{ 
                    background: currentProject.env === "生产" ? "#dcfce7" : "#f1f5f9",
                    color: currentProject.env === "生产" ? "#15803d" : "#475569",
                    fontSize: 11,
                    padding: "1px 6px",
                    borderRadius: 4
                  }}>
                    {currentProject.env}环境
                  </span>
                )}
              </div>
              <p style={{ margin: "4px 0 0", fontSize: 12, color: "#64748b" }}>
                {currentProject 
                  ? `客户单位: ${currentProject.customerName} · 承载云厂商: ${currentProject.cloudVendor} (${currentProject.regionName})`
                  : "汇聚 24 个项目单位 · 覆盖联通云、首信云、国企云、太极云、阿里云等混合云算力资源"
                }
              </p>
            </div>

            {/* Quick Metrics Badges */}
            <div style={{ display: "flex", gap: 10, alignItems: "center" }}>
              <div style={{ background: "#f8fafc", border: "1px solid #e2e8f0", padding: "6px 12px", borderRadius: 6, textAlign: "center" }}>
                <span style={{ fontSize: 10, color: "#64748b", display: "block" }}>项目资产</span>
                <strong style={{ fontSize: 15, color: "#0f172a" }}>
                  {displayedAssets.length} <small style={{ fontSize: 10, fontWeight: 400 }}>台</small>
                </strong>
              </div>

              {currentProject && (
                <>
                  <div style={{ background: "#f8fafc", border: "1px solid #e2e8f0", padding: "6px 12px", borderRadius: 6, textAlign: "center" }}>
                    <span style={{ fontSize: 10, color: "#64748b", display: "block" }}>算力核数</span>
                    <strong style={{ fontSize: 15, color: "#2563eb" }}>
                      {currentProject.totalCores} <small style={{ fontSize: 10, fontWeight: 400 }}>Cores</small>
                    </strong>
                  </div>
                  <div style={{ background: "#f8fafc", border: "1px solid #e2e8f0", padding: "6px 12px", borderRadius: 6, textAlign: "center" }}>
                    <span style={{ fontSize: 10, color: "#64748b", display: "block" }}>内存池</span>
                    <strong style={{ fontSize: 15, color: "#059669" }}>
                      {currentProject.totalMemoryGb} <small style={{ fontSize: 10, fontWeight: 400 }}>GB</small>
                    </strong>
                  </div>
                  <div style={{ background: "#f8fafc", border: "1px solid #e2e8f0", padding: "6px 12px", borderRadius: 6, textAlign: "center" }}>
                    <span style={{ fontSize: 10, color: "#64748b", display: "block" }}>国产信创</span>
                    <strong style={{ fontSize: 15, color: "#dc2626" }}>
                      {currentProject.xinchuangCount} <small style={{ fontSize: 10, fontWeight: 400 }}>台</small>
                    </strong>
                  </div>
                </>
              )}

              {currentProjectVpn && (
                <button 
                  type="button"
                  className="btn-secondary" 
                  onClick={() => setShowProjectVpnModal(true)} 
                  style={{ 
                    padding: "6px 11px", 
                    background: "#f0fdf4", 
                    borderColor: "#86efac", 
                    color: "#166534", 
                    fontWeight: 600,
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 5
                  }}
                  title="查看当前项目专网 VPN 网关及拨号策略"
                >
                  <span>🛡️</span>
                  <span>项目专网VPN</span>
                </button>
              )}

              {/* 📥 导入按钮 */}
              <button
                type="button"
                className="btn-secondary"
                onClick={() => setShowImportModal(true)}
                style={{
                  padding: "6px 12px",
                  background: "#eff6ff",
                  borderColor: "#93c5fd",
                  color: "#1e40af",
                  fontWeight: 600,
                  display: "inline-flex",
                  alignItems: "center",
                  gap: 5
                }}
                title={currentProject ? `导入 Excel 资产到【${currentProject.name}】` : "批量导入《信息资产台账》Excel 资产"}
              >
                <span>📥</span>
                <span>{currentProject ? "导入台账到当前项目" : "导入台账"}</span>
              </button>

              {/* 📤 导出按钮 */}
              <button
                type="button"
                className="btn-secondary"
                onClick={handleExportExcel}
                style={{
                  padding: "6px 12px",
                  background: "#f8fafc",
                  borderColor: "#cbd5e1",
                  color: "#0f172a",
                  fontWeight: 600,
                  display: "inline-flex",
                  alignItems: "center",
                  gap: 5
                }}
                title={`按照《信息资产台账-v340.xlsx》02硬件规范导出当前查询的 ${displayedAssets.length} 台资产`}
              >
                <span>📤</span>
                <span>导出Excel (02硬件格式)</span>
              </button>

              {/* 🔍 IP地址查询按钮 */}
              <button
                type="button"
                className="btn-secondary"
                onClick={() => {
                  setIpBatchText(ipSearchKeyword || "");
                  setShowIpModal(true);
                }}
                style={{
                  padding: "6px 12px",
                  background: "#f0fdf4",
                  borderColor: "#86efac",
                  color: "#166534",
                  fontWeight: 600,
                  display: "inline-flex",
                  alignItems: "center",
                  gap: 5
                }}
                title="快速检索单个或批量比对多个 IP 地址并定位所属项目与资产"
              >
                <span>🌐</span>
                <span>IP地址查询</span>
              </button>

              <button className="btn-primary" onClick={() => setShowAddModal(true)} style={{ padding: "8px 12px" }}>
                ＋ 录入项目资产
              </button>
            </div>
          </div>
        </div>

        {/* Duplicate Warning & One-Click Cleanup Banner */}
        {duplicateCount > 0 && (
          <div style={{
            background: "#fffbeb",
            border: "1.5px solid #fcd34d",
            borderRadius: 8,
            padding: "10px 16px",
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            boxShadow: "0 2px 6px rgba(217, 119, 6, 0.08)",
            animation: "fadeIn 0.3s ease"
          }}>
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <span style={{ fontSize: 20 }}>⚠️</span>
              <div>
                <strong style={{ fontSize: 13, color: "#92400e" }}>
                  检测到台账中存在 {duplicateCount} 台重复设备（可能由导入历史重复文件引起）
                </strong>
                <div style={{ fontSize: 11, color: "#b45309", marginTop: 2 }}>
                  系统已配备智能去重清洗引擎，点击右侧按钮即可基于「业务IP / 设备标识」快速去重合并，并自动校准各项目资产计数。
                </div>
              </div>
            </div>
            <button
              type="button"
              className="btn-primary"
              onClick={handleTriggerDeduplicate}
              style={{
                background: "#d97706",
                borderColor: "#b45309",
                color: "#fff",
                fontWeight: 600,
                fontSize: 12,
                padding: "6px 14px",
                display: "inline-flex",
                alignItems: "center",
                gap: 6,
                boxShadow: "0 1px 2px rgba(0,0,0,0.1)",
                cursor: "pointer"
              }}
            >
              <span>🧹</span>
              <span>一键去重并校准台账</span>
            </button>
          </div>
        )}

        {/* Filter Bar & Controls */}
        <div style={{
          background: "#fff",
          border: "1px solid #e2e8f0",
          borderRadius: 8,
          padding: "8px 14px",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          flexWrap: "wrap",
          gap: 8
        }}>
          <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
            <input 
              placeholder="搜索设备名称 / 备注..." 
              value={assetKeyword}
              onChange={e => { setAssetKeyword(e.target.value); setCurrentPage(1); }}
              style={{ width: 170, fontSize: 12 }}
            />

            {/* IP 地址独立检索框与查询按钮 */}
            <div style={{ display: "flex", alignItems: "center", gap: 3 }}>
              <input 
                placeholder="输入IP地址 (业务IP/内大网/VIP/网段)..." 
                value={ipSearchInput}
                onChange={e => setIpSearchInput(e.target.value)}
                onKeyDown={e => {
                  if (e.key === "Enter") {
                    setIpSearchKeyword(ipSearchInput.trim());
                    setCurrentPage(1);
                  }
                }}
                style={{ width: 210, fontSize: 12, padding: "4px 8px" }}
              />
              <button
                type="button"
                className="btn-primary"
                onClick={() => {
                  setIpSearchKeyword(ipSearchInput.trim());
                  setCurrentPage(1);
                }}
                style={{
                  fontSize: 11,
                  padding: "4px 9px",
                  display: "inline-flex",
                  alignItems: "center",
                  gap: 3,
                  cursor: "pointer",
                  whiteSpace: "nowrap"
                }}
                title="按输入的目标 IP 地址进行精准查询与过滤"
              >
                <span>🔍</span>
                <span>查询IP</span>
              </button>
              {ipSearchKeyword && (
                <button
                  type="button"
                  className="btn-secondary"
                  onClick={() => {
                    setIpSearchInput("");
                    setIpSearchKeyword("");
                    setCurrentPage(1);
                  }}
                  style={{ fontSize: 11, padding: "4px 6px", color: "#64748b" }}
                  title="清除当前 IP 查询"
                >
                  ✕
                </button>
              )}
            </div>

            <select 
              value={deviceTypeFilter} 
              onChange={e => { setDeviceTypeFilter(e.target.value); setCurrentPage(1); }}
              style={{ fontSize: 12, padding: "4px 8px" }}
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
              style={{ fontSize: 12, padding: "4px 8px" }}
            >
              <option value="全部">全部操作系统</option>
              <option value="是">国产信创 OS (🛡️麒麟/统信)</option>
              <option value="否">常规 OS (CentOS/RedHat)</option>
            </select>

            {(assetKeyword || ipSearchKeyword || deviceTypeFilter !== "全部" || xinchuangFilter !== "全部") && (
              <button 
                className="btn-secondary"
                style={{ fontSize: 11, padding: "3px 8px" }}
                onClick={() => {
                  setAssetKeyword("");
                  setIpSearchInput("");
                  setIpSearchKeyword("");
                  setDeviceTypeFilter("全部");
                  setXinchuangFilter("全部");
                  setCurrentPage(1);
                }}
              >
                ✕ 重置筛选
              </button>
            )}
          </div>

          <div style={{ display: "flex", gap: 10, alignItems: "center" }}>
            <span style={{ fontSize: 12, color: "#64748b" }}>
              共 <strong style={{ color: "#0f172a" }}>{displayedAssets.length}</strong> 台资产记录
            </span>
            <button
              type="button"
              className="btn-secondary"
              onClick={handleExportExcel}
              style={{
                fontSize: 11,
                padding: "3px 8px",
                background: "#f0fdf4",
                borderColor: "#bbf7d0",
                color: "#15803d",
                fontWeight: 600,
                display: "inline-flex",
                alignItems: "center",
                gap: 4
              }}
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
                <div style={{ fontSize: 11, color: "#3b82f6", marginTop: 2 }}>
                  涉及项目: {crossProjectIpMatch.projects.join("、")} · 匹配资产: {crossProjectIpMatch.matchedAssets.map(a => `${a.name} (${a.privateIp || a.ip})`).slice(0, 3).join(", ")}
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
                        {/* Seq */}
                        <td style={{ fontFamily: "monospace", color: "#64748b", fontWeight: 600 }}>
                          {item.seq || "-"}
                        </td>

                        {/* Device Name & Type */}
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
                              style={{ color: "#0f172a", cursor: "pointer" }}
                              title="点击查看全量台账元数据"
                              onClick={() => setDetailAsset(item)}
                            >
                              {item.name}
                            </strong>
                          </div>
                          {isPhysical && (item as any).model && (
                            <small style={{ color: "#64748b", display: "block", marginTop: 2 }}>
                              {(item as any).brand} {(item as any).model} · 码: {(item as any).assetNo}
                            </small>
                          )}
                          {/* 软件栈微徽章 */}
                          {(() => {
                            const ip = item.privateIp || item.ip;
                            const sList = softwareList.filter(s => s.assetId === item.id || (ip && s.assetIp === ip) || s.assetName === item.name);
                            if (sList.length === 0) return null;
                            return (
                              <div 
                                style={{ display: "inline-flex", alignItems: "center", gap: 4, marginTop: 4, cursor: "pointer", background: "#f0fdf4", border: "1px solid #bbf7d0", padding: "1px 6px", borderRadius: 4 }}
                                onClick={(e) => { e.stopPropagation(); setDetailAsset(item); setDetailTab("software"); }}
                                title="点击查看部署软件与中间件详情"
                              >
                                <span style={{ fontSize: 10, color: "#166534", fontWeight: 700 }}>
                                  🧩 {sList.length}个软件
                                </span>
                                <span style={{ fontSize: 10, color: "#15803d" }}>
                                  ({sList.slice(0, 2).map(s => s.name.split(" ")[0]).join("/")}{sList.length > 2 ? "..." : ""})
                                </span>
                              </div>
                            );
                          })()}
                        </td>

                        {/* Project & Customer */}
                        <td>
                          <div style={{ fontWeight: 600, color: "#1e293b" }}>{item.projectName}</div>
                          <small style={{ color: "#64748b", display: "block" }}>{item.customerName}</small>
                        </td>

                        {/* Env & Cloud & Region */}
                        <td>
                          <div style={{ display: "flex", gap: 4, alignItems: "center" }}>
                            <span style={{
                              padding: "1px 5px",
                              borderRadius: 3,
                              fontSize: 10,
                              background: item.env === "生产" ? "#dcfce7" : "#f1f5f9",
                              color: item.env === "生产" ? "#15803d" : "#475569"
                            }}>
                              {item.env || "生产"}
                            </span>
                            <span style={{ color: "#2563eb", fontWeight: 500, fontSize: 11 }}>{item.cloudVendor}</span>
                          </div>
                          <small style={{ color: "#64748b", display: "block", marginTop: 2 }}>{item.regionName || "-"}</small>
                        </td>

                        {/* IPs */}
                        <td>
                          <div style={{ fontFamily: "monospace", fontWeight: 600, color: "#2563eb" }}>
                            {item.privateIp || item.ip || "-"}
                          </div>
                          {item.internalWanIp && (
                            <small style={{ fontFamily: "monospace", color: "#64748b", display: "block" }}>
                              内大网: {item.internalWanIp}
                            </small>
                          )}
                        </td>

                        {/* VIP / EIP */}
                        <td>
                          {item.vip ? (
                            <span style={{ fontFamily: "monospace", color: "#7c3aed", fontWeight: 600, display: "block" }}>
                              VIP: {item.vip}
                            </span>
                          ) : null}
                          {item.eip ? (
                            <span style={{ fontFamily: "monospace", color: "#059669", display: "block" }}>
                              EIP: {item.eip}
                            </span>
                          ) : null}
                          {!item.vip && !item.eip && <span style={{ color: "#94a3b8" }}>-</span>}
                        </td>

                        {/* Resource Spec */}
                        <td>
                          <div>
                            <strong style={{ color: "#1e293b" }}>{item.cpuCores ? `${item.cpuCores}C` : "4C"}</strong> / 
                            <strong style={{ color: "#1e293b", marginLeft: 2 }}>{item.memoryGb ? `${item.memoryGb}G` : "8G"}</strong>
                            <span style={{ color: "#64748b", fontSize: 11, marginLeft: 4 }}>({item.cpuArch || "x86_64"})</span>
                          </div>
                          <small style={{ color: "#64748b", display: "block", marginTop: 2 }}>
                            盘: {item.systemDiskGb || 30}G {item.dataDiskGb ? `+ ${item.dataDiskGb}G数据` : ""}
                          </small>
                        </td>

                        {/* OS & Xinchuang */}
                        <td>
                          <div style={{ display: "flex", alignItems: "center", gap: 4 }}>
                            {item.isXinchuang === "是" && (
                              <span 
                                style={{ background: "#fee2e2", color: "#b91c1c", fontSize: 10, padding: "1px 4px", borderRadius: 3, fontWeight: 700 }}
                                title="国产信创 OS 认证"
                              >
                                🛡️信创
                              </span>
                            )}
                            <span style={{ fontWeight: 500, color: "#334155" }}>
                              {item.osFamily || item.os || "Linux"}
                            </span>
                          </div>
                          <small style={{ color: "#64748b", display: "block", marginTop: 2 }}>
                            {item.osVersion || ""}
                          </small>
                        </td>

                        {/* Remote Port */}
                        <td style={{ fontFamily: "monospace", color: "#475569" }}>
                          {item.remotePort || 22}
                        </td>

                        {/* Remarks */}
                        <td style={{ color: "#64748b" }} title={item.remarks || ""}>
                          <span style={{ 
                            display: "-webkit-box", 
                            WebkitLineClamp: 2, 
                            WebkitBoxOrient: "vertical", 
                            overflow: "hidden" 
                          }}>
                            {item.remarks || "-"}
                          </span>
                        </td>

                        {/* Actions */}
                        <td style={{ textAlign: "center" }}>
                          <div style={{ display: "flex", justifyContent: "center", gap: 4 }}>
                            <button 
                              className="btn-secondary" 
                              style={{ padding: "2px 6px", fontSize: 11 }}
                              onClick={() => { setDetailAsset(item); setDetailTab("spec"); }}
                              title="查看资产规格与OS档案"
                            >
                              档案
                            </button>
                            <button 
                              className="btn-secondary" 
                              style={{ padding: "2px 6px", fontSize: 11, color: "#0d9488", borderColor: "#99f6e4", background: "#f0fdfa" }}
                              onClick={() => { setDetailAsset(item); setDetailTab("software"); }}
                              title="查看或配置该机器运行的数据库、中间件与插件"
                            >
                              🧩软件
                            </button>
                            <button 
                              className="btn-secondary" 
                              style={{ padding: "2px 6px", fontSize: 11, color: "#2563eb", borderColor: "#bfdbfe", background: "#eff6ff" }}
                              onClick={() => { setDetailAsset(item); setDetailTab("ops"); }}
                              title="查看系统登录链接、SSH直连与VPN通道"
                            >
                              🚀运维
                            </button>
                            <button 
                              className="btn-secondary" 
                              style={{ padding: "2px 6px", fontSize: 11, color: "#b45309", borderColor: "#fde68a", background: "#fffbeb" }}
                              onClick={() => openEditModal(item)}
                              title="修改该资产配置与台账信息"
                            >
                              ✏️修改
                            </button>
                            <button 
                              className="btn-secondary" 
                              style={{ padding: "2px 6px", fontSize: 11, color: "#dc2626", borderColor: "#fecaca", background: "#fef2f2" }}
                              onClick={() => setDeletingAsset(item)}
                              title="从资产库注销删除该设备"
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
          </div>

          {/* Pagination Footer */}
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
              {currentProject ? `【${currentProject.name}】` : "全量资产"} 共{" "}
              <strong style={{ color: "#0f172a" }}>{displayedAssets.length}</strong> 条记录
            </div>

            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <span>每页展示:</span>
              <select 
                value={pageSize} 
                onChange={e => { setPageSize(Number(e.target.value)); setCurrentPage(1); }}
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
                disabled={currentPage <= 1}
                onClick={() => setCurrentPage(prev => Math.max(1, prev - 1))}
              >
                ‹ 上一页
              </button>

              <span>
                第 <strong style={{ color: "#2563eb" }}>{currentPage}</strong> / {totalPages} 页
              </span>

              <button 
                className="btn-secondary" 
                style={{ padding: "3px 8px", fontSize: 11 }}
                disabled={currentPage >= totalPages}
                onClick={() => setCurrentPage(prev => Math.min(totalPages, prev + 1))}
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
          <div className="cmdb-modal" style={{ maxWidth: 780, width: "95%" }}>
            <div className="cmdb-modal-header" style={{ background: "#0f172a", color: "#fff", borderBottom: 0 }}>
              <div>
                <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                  <span style={{ 
                    background: "#2563eb", 
                    color: "#fff", 
                    fontSize: 11, 
                    padding: "2px 8px", 
                    borderRadius: 4, 
                    fontWeight: 600 
                  }}>
                    序号 #{detailAsset.seq}
                  </span>
                  <h3 style={{ margin: 0, color: "#fff", fontSize: 16 }}>{detailAsset.name}</h3>
                  {detailAsset.isXinchuang === "是" && (
                    <span style={{ background: "#ef4444", color: "#fff", fontSize: 11, padding: "2px 6px", borderRadius: 4 }}>
                      🛡️ 国产信创 OS
                    </span>
                  )}
                </div>
                <small style={{ color: "#94a3b8", display: "block", marginTop: 4 }}>
                  归属项目: {detailAsset.projectName} · 客户单位: {detailAsset.customerName} · {detailAsset.env}环境
                </small>
              </div>
              <button type="button" className="cmdb-modal-close" style={{ color: "#fff" }} onClick={() => setDetailAsset(null)}>×</button>
            </div>

                        <div className="cmdb-modal-body" style={{ maxHeight: "74vh", overflowY: "auto", padding: "16px 20px" }}>
              {/* Tab Navigation */}
              <div style={{
                display: "flex",
                gap: 8,
                borderBottom: "2px solid #e2e8f0",
                marginBottom: 16,
                paddingBottom: 2
              }}>
                <button 
                  type="button"
                  onClick={() => setDetailTab("spec")}
                  style={{
                    border: "none",
                    background: "transparent",
                    padding: "8px 14px",
                    fontWeight: 700,
                    fontSize: 13,
                    cursor: "pointer",
                    color: detailTab === "spec" ? "#2563eb" : "#64748b",
                    borderBottom: detailTab === "spec" ? "2px solid #2563eb" : "2px solid transparent",
                    marginBottom: -4,
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 6
                  }}
                >
                  <span>📋</span>
                  <span>硬件规格与OS</span>
                </button>

                <button 
                  type="button"
                  onClick={() => setDetailTab("software")}
                  style={{
                    border: "none",
                    background: "transparent",
                    padding: "8px 14px",
                    fontWeight: 700,
                    fontSize: 13,
                    cursor: "pointer",
                    color: detailTab === "software" ? "#0d9488" : "#64748b",
                    borderBottom: detailTab === "software" ? "2px solid #0d9488" : "2px solid transparent",
                    marginBottom: -4,
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 6
                  }}
                >
                  <span>🧩</span>
                  <span>软件·中间件·数据库</span>
                  <span style={{ 
                    background: detailTab === "software" ? "#ccfbf1" : "#f1f5f9", 
                    color: detailTab === "software" ? "#0f766e" : "#64748b", 
                    fontSize: 11, 
                    padding: "0 6px", 
                    borderRadius: 10 
                  }}>
                    {currentAssetSoftwares.length}
                  </span>
                </button>

                <button 
                  type="button"
                  onClick={() => setDetailTab("ops")}
                  style={{
                    border: "none",
                    background: "transparent",
                    padding: "8px 14px",
                    fontWeight: 700,
                    fontSize: 13,
                    cursor: "pointer",
                    color: detailTab === "ops" ? "#7c3aed" : "#64748b",
                    borderBottom: detailTab === "ops" ? "2px solid #7c3aed" : "2px solid transparent",
                    marginBottom: -4,
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 6
                  }}
                >
                  <span>🚀</span>
                  <span>登录与运维通道</span>
                  <span style={{ 
                    background: detailTab === "ops" ? "#ede9fe" : "#f1f5f9", 
                    color: detailTab === "ops" ? "#6d28d9" : "#64748b", 
                    fontSize: 11, 
                    padding: "0 6px", 
                    borderRadius: 10 
                  }}>
                    {currentAssetChannels.length}
                  </span>
                </button>

                <button 
                  type="button"
                  onClick={() => setDetailTab("vpn")}
                  style={{
                    border: "none",
                    background: "transparent",
                    padding: "8px 14px",
                    fontWeight: 700,
                    fontSize: 13,
                    cursor: "pointer",
                    color: detailTab === "vpn" ? "#ea580c" : "#64748b",
                    borderBottom: detailTab === "vpn" ? "2px solid #ea580c" : "2px solid transparent",
                    marginBottom: -4,
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 6
                  }}
                >
                  <span>🛡️</span>
                  <span>VPN 专网通道</span>
                </button>

                {copiedNotice && (
                  <span style={{ marginLeft: "auto", alignSelf: "center", fontSize: 11, color: "#16a34a", background: "#dcfce7", padding: "3px 8px", borderRadius: 4, fontWeight: 600 }}>
                    ✓ {copiedNotice} 已复制到剪贴板！
                  </span>
                )}
              </div>

              {/* TAB 1: 硬件规格与OS */}
              {detailTab === "spec" && (
                <div>
                  {/* 基本信息 */}
                  <h5 style={{ margin: "0 0 10px", fontSize: 13, color: "#0284c7", borderBottom: "1px solid #e0f2fe", paddingBottom: 4 }}>
                    📋 项目与基础归属 (台账基本信息)
                  </h5>
                  <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 12, fontSize: 12, marginBottom: 16 }}>
                    <div><span style={{ color: "#64748b" }}>项目名称:</span> <strong style={{ color: "#1d4ed8" }}>{detailAsset.projectName || "-"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>客户名称:</span> <strong>{detailAsset.customerName || "-"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>环境类型:</span> <strong>{detailAsset.env || "-"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>承载云商:</span> <strong style={{ color: "#2563eb" }}>{detailAsset.cloudVendor || "-"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>安全区域:</span> <strong>{detailAsset.regionName || "-"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>设备大类:</span> <strong>{detailAsset.category || "服务器"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>设备类型:</span> <strong>{detailAsset.deviceType || "虚拟机"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>资产形态:</span> <strong>{detailAsset._kind === "physical" ? "实体物理服务器" : "虚拟计算节点"}</strong></div>
                  </div>

                  {/* 网络信息 */}
                  <h5 style={{ margin: "0 0 10px", fontSize: 13, color: "#0d9488", borderBottom: "1px solid #ccfbf1", paddingBottom: 4 }}>
                    🌐 网络与 IP 矩阵
                  </h5>
                  <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12, fontSize: 12, marginBottom: 16 }}>
                    <div style={{ background: "#f8fafc", padding: 8, borderRadius: 6 }}>
                      <span style={{ color: "#64748b", display: "block" }}>私有IP（业务IP）:</span>
                      <code style={{ fontSize: 13, color: "#2563eb", fontWeight: 600 }}>{detailAsset.privateIp || detailAsset.ip || "-"}</code>
                    </div>
                    <div style={{ background: "#f8fafc", padding: 8, borderRadius: 6 }}>
                      <span style={{ color: "#64748b", display: "block" }}>内大网IP:</span>
                      <code style={{ fontSize: 13, color: "#059669", fontWeight: 600 }}>{detailAsset.internalWanIp || "-"}</code>
                    </div>
                    <div style={{ background: "#f8fafc", padding: 8, borderRadius: 6 }}>
                      <span style={{ color: "#64748b", display: "block" }}>VIP 地址:</span>
                      <code style={{ fontSize: 13, color: "#7c3aed", fontWeight: 600 }}>{detailAsset.vip || "-"}</code>
                    </div>
                    <div style={{ background: "#f8fafc", padding: 8, borderRadius: 6 }}>
                      <span style={{ color: "#64748b", display: "block" }}>EIP / 公网IP:</span>
                      <code style={{ fontSize: 13, color: "#ea580c", fontWeight: 600 }}>{detailAsset.eip || detailAsset.publicIp || "-"}</code>
                    </div>
                  </div>

                  {/* 规格配置 */}
                  <h5 style={{ margin: "0 0 10px", fontSize: 13, color: "#d97706", borderBottom: "1px solid #fef3c7", paddingBottom: 4 }}>
                    ⚡ 硬件规格与存储配额
                  </h5>
                  <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 12, fontSize: 12, marginBottom: 16 }}>
                    <div><span style={{ color: "#64748b" }}>CPU 架构:</span> <strong>{detailAsset.cpuArch || "x86_64"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>CPU 核心数:</span> <strong style={{ color: "#b45309" }}>{detailAsset.cpuCores ? `${detailAsset.cpuCores} 核` : "-"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>内存容量:</span> <strong style={{ color: "#b45309" }}>{detailAsset.memoryGb ? `${detailAsset.memoryGb} GB` : "-"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>系统盘:</span> <strong>{detailAsset.systemDiskGb ? `${detailAsset.systemDiskGb} GB` : "-"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>数据盘:</span> <strong>{detailAsset.dataDiskGb ? `${detailAsset.dataDiskGb} GB` : "-"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>共享磁盘:</span> <strong>{detailAsset.sharedDiskGb ? `${detailAsset.sharedDiskGb} GB` : "-"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>对象存储:</span> <strong>{detailAsset.objectStorageGb ? `${detailAsset.objectStorageGb} GB` : "-"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>远程连接端口:</span> <strong style={{ fontFamily: "monospace" }}>{detailAsset.remotePort || 22}</strong></div>
                  </div>

                  {/* 操作系统 */}
                  <h5 style={{ margin: "0 0 10px", fontSize: 13, color: "#16a34a", borderBottom: "1px solid #dcfce7", paddingBottom: 4 }}>
                    🐧 操作系统发行版及内核
                  </h5>
                  <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12, fontSize: 12, marginBottom: 16 }}>
                    <div><span style={{ color: "#64748b" }}>OS 发行版:</span> <strong>{detailAsset.osFamily || "-"}</strong></div>
                    <div><span style={{ color: "#64748b" }}>OS 完整版本:</span> <strong>{detailAsset.osVersion || "-"}</strong></div>
                    <div style={{ gridColumn: "span 2" }}>
                      <span style={{ color: "#64748b" }}>系统内核版本:</span> 
                      <code style={{ display: "block", marginTop: 4, background: "#f1f5f9", padding: "4px 8px", borderRadius: 4, fontSize: 11 }}>
                        {detailAsset.kernelVersion || "Linux Kernel"}
                      </code>
                    </div>
                  </div>

                  {/* 备注 */}
                  {detailAsset.remarks && (
                    <div style={{ background: "#fffbeb", border: "1px solid #fde68a", padding: 10, borderRadius: 6, fontSize: 12 }}>
                      <strong style={{ color: "#92400e" }}>📌 项目资产台账备注：</strong>
                      <span style={{ color: "#78350f", marginLeft: 6 }}>{detailAsset.remarks}</span>
                    </div>
                  )}
                </div>
              )}

              {/* TAB 2: 软件·中间件·数据库 */}
              {detailTab === "software" && (
                <div>
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
                    <div>
                      <h5 style={{ margin: 0, fontSize: 14, color: "#0f766e" }}>
                        🧩 本节点运行软件与服务组件 ({currentAssetSoftwares.length})
                      </h5>
                      <small style={{ color: "#64748b" }}>包含已纳管的数据库实例、应用中间件、运行库与关键插件</small>
                    </div>
                    <button 
                      type="button" 
                      className="btn-secondary" 
                      style={{ fontSize: 11, padding: "4px 10px", borderColor: "#99f6e4", background: "#f0fdfa", color: "#0d9488", fontWeight: 600 }}
                      onClick={() => setShowAddSoftForm(!showAddSoftForm)}
                    >
                      {showAddSoftForm ? "✕ 取消登记" : "＋ 登记新软件组件"}
                    </button>
                  </div>

                  {/* Inline Add Software Form */}
                  {showAddSoftForm && (
                    <div style={{ background: "#f0fdfa", border: "1px solid #99f6e4", borderRadius: 6, padding: 12, marginBottom: 14 }}>
                      <strong style={{ fontSize: 12, color: "#0f766e", display: "block", marginBottom: 8 }}>
                        📝 登记新运行软件（绑定至本节点）
                      </strong>
                      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 8, fontSize: 12 }}>
                        <div>
                          <label style={{ fontSize: 11, color: "#475569" }}>软件名称*</label>
                          <input 
                            placeholder="如 MySQL, Nginx, JDK" 
                            value={newSoftDraft.name} 
                            onChange={e => setNewSoftDraft({ ...newSoftDraft, name: e.target.value })}
                            style={{ width: "100%", fontSize: 12, padding: 4 }}
                          />
                        </div>
                        <div>
                          <label style={{ fontSize: 11, color: "#475569" }}>分类*</label>
                          <select 
                            value={newSoftDraft.category} 
                            onChange={e => setNewSoftDraft({ ...newSoftDraft, category: e.target.value as any })}
                            style={{ width: "100%", fontSize: 12, padding: 4 }}
                          >
                            <option value="database">🗄️ 数据库 (Database)</option>
                            <option value="middleware">⚙️ 中间件 (Middleware)</option>
                            <option value="web_server">🌐 Web服务 (Web Server)</option>
                            <option value="plugin">☕ 运行库/插件 (Runtime/Plugin)</option>
                          </select>
                        </div>
                        <div>
                          <label style={{ fontSize: 11, color: "#475569" }}>版本号*</label>
                          <input 
                            placeholder="如 8.0.32, 1.24.0" 
                            value={newSoftDraft.version} 
                            onChange={e => setNewSoftDraft({ ...newSoftDraft, version: e.target.value })}
                            style={{ width: "100%", fontSize: 12, padding: 4 }}
                          />
                        </div>
                        <div>
                          <label style={{ fontSize: 11, color: "#475569" }}>监听端口</label>
                          <input 
                            placeholder="如 3306, 80, 8080" 
                            value={newSoftDraft.port} 
                            onChange={e => setNewSoftDraft({ ...newSoftDraft, port: e.target.value })}
                            style={{ width: "100%", fontSize: 12, padding: 4 }}
                          />
                        </div>
                        <div>
                          <label style={{ fontSize: 11, color: "#475569" }}>安装部署路径</label>
                          <input 
                            placeholder="如 /usr/local/nginx" 
                            value={newSoftDraft.installPath} 
                            onChange={e => setNewSoftDraft({ ...newSoftDraft, installPath: e.target.value })}
                            style={{ width: "100%", fontSize: 12, padding: 4 }}
                          />
                        </div>
                        <div>
                          <label style={{ fontSize: 11, color: "#475569" }}>业务用途 / 备注</label>
                          <input 
                            placeholder="如 核心业务主库" 
                            value={newSoftDraft.remarks} 
                            onChange={e => setNewSoftDraft({ ...newSoftDraft, remarks: e.target.value })}
                            style={{ width: "100%", fontSize: 12, padding: 4 }}
                          />
                        </div>
                      </div>
                      <div style={{ textAlign: "right", marginTop: 8 }}>
                        <button 
                          type="button" 
                          className="btn-primary"
                          style={{ padding: "4px 12px", fontSize: 12 }}
                          onClick={() => {
                            if (!newSoftDraft.name.trim() || !newSoftDraft.version.trim()) {
                              alert("请填写软件名称与版本号！");
                              return;
                            }
                            const newComponent: SoftwareComponent = {
                              id: `soft-${Date.now()}`,
                              assetId: detailAsset.id,
                              assetName: detailAsset.name,
                              assetIp: detailAsset.privateIp || detailAsset.ip,
                              projectId: detailAsset.projectId || "prj-001",
                              projectName: detailAsset.projectName || "工会互助保险信息系统",
                              category: newSoftDraft.category,
                              name: newSoftDraft.name,
                              version: newSoftDraft.version,
                              port: newSoftDraft.port || "-",
                              installPath: newSoftDraft.installPath || "-",
                              configPath: newSoftDraft.configPath || "-",
                              status: "running",
                              remarks: newSoftDraft.remarks || "管理员手工登记"
                            };
                            onAddSoftware?.(newComponent);
                            setShowAddSoftForm(false);
                            setNewSoftDraft({ name: "", category: "database", version: "", port: "", installPath: "", configPath: "", remarks: "" });
                          }}
                        >
                          确认登记组件
                        </button>
                      </div>
                    </div>
                  )}

                  {/* Softwares Table */}
                  {currentAssetSoftwares.length === 0 ? (
                    <div style={{ textAlign: "center", padding: "30px 0", color: "#94a3b8", background: "#f8fafc", borderRadius: 6 }}>
                      暂未为该节点登记运行软件与中间件组件，点击上方「＋ 登记新软件组件」添加。
                    </div>
                  ) : (
                    <div style={{ border: "1px solid #e2e8f0", borderRadius: 6, overflow: "hidden" }}>
                      <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
                        <thead>
                          <tr style={{ background: "#f8fafc", borderBottom: "1px solid #e2e8f0", textAlign: "left" }}>
                            <th style={{ padding: "8px 10px" }}>分类</th>
                            <th style={{ padding: "8px 10px" }}>软件名称及实例</th>
                            <th style={{ padding: "8px 10px" }}>核心版本号</th>
                            <th style={{ padding: "8px 10px" }}>监听端口</th>
                            <th style={{ padding: "8px 10px" }}>部署目录 / 配置文件</th>
                            <th style={{ padding: "8px 10px" }}>运行状态</th>
                            <th style={{ padding: "8px 10px" }}>业务备注</th>
                            <th style={{ padding: "8px 10px", width: 50, textAlign: "center" }}>操作</th>
                          </tr>
                        </thead>
                        <tbody>
                          {currentAssetSoftwares.map(soft => {
                            const badgeInfo = soft.category === "database"
                              ? { label: "🗄️ 数据库", bg: "#e0e7ff", text: "#3730a3" }
                              : soft.category === "middleware"
                              ? { label: "⚙️ 中间件", bg: "#fef3c7", text: "#92400e" }
                              : soft.category === "web_server"
                              ? { label: "🌐 Web服务", bg: "#e0f2fe", text: "#0369a1" }
                              : { label: "☕ 运行库/插件", bg: "#dcfce7", text: "#15803d" };

                            return (
                              <tr key={soft.id} style={{ borderBottom: "1px solid #f1f5f9" }}>
                                <td style={{ padding: "8px 10px" }}>
                                  <span style={{ background: badgeInfo.bg, color: badgeInfo.text, padding: "1px 6px", borderRadius: 4, fontSize: 10, fontWeight: 600 }}>
                                    {badgeInfo.label}
                                  </span>
                                </td>
                                <td style={{ padding: "8px 10px", fontWeight: 700, color: "#1e293b" }}>
                                  {soft.name}
                                </td>
                                <td style={{ padding: "8px 10px" }}>
                                  <code style={{ background: "#f1f5f9", padding: "1px 5px", borderRadius: 3, fontSize: 11, color: "#2563eb", fontWeight: 600 }}>
                                    {soft.version}
                                  </code>
                                </td>
                                <td style={{ padding: "8px 10px", fontFamily: "monospace", color: "#d97706", fontWeight: 600 }}>
                                  {soft.port || "-"}
                                </td>
                                <td style={{ padding: "8px 10px", fontFamily: "monospace", fontSize: 11, color: "#64748b" }} title={soft.installPath}>
                                  {soft.installPath || "-"}
                                </td>
                                <td style={{ padding: "8px 10px" }}>
                                  <span style={{ color: soft.status === "running" ? "#16a34a" : "#dc2626", fontWeight: 600, fontSize: 11 }}>
                                    ● {soft.status === "running" ? "活跃运行" : "停止"}
                                  </span>
                                </td>
                                <td style={{ padding: "8px 10px", color: "#64748b" }}>
                                  {soft.remarks || "-"}
                                </td>
                                <td style={{ padding: "8px 10px", textAlign: "center" }}>
                                  <button 
                                    type="button" 
                                    onClick={() => onDeleteSoftware?.(soft.id)}
                                    style={{ border: "none", background: "transparent", color: "#ef4444", cursor: "pointer", fontSize: 12 }}
                                    title="删除此软件实例"
                                  >
                                    ✕
                                  </button>
                                </td>
                              </tr>
                            );
                          })}
                        </tbody>
                      </table>
                    </div>
                  )}
                </div>
              )}

              {/* TAB 3: 登录与运维通道 */}
              {detailTab === "ops" && (
                <div>
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
                    <div>
                      <h5 style={{ margin: 0, fontSize: 14, color: "#6d28d9" }}>
                        🚀 登录系统与运维直连通道 ({currentAssetChannels.length})
                      </h5>
                      <small style={{ color: "#64748b" }}>包含业务管理后台直达链接、SSH/RDP快速登录指令与堡垒机入口</small>
                    </div>
                    <button 
                      type="button" 
                      className="btn-secondary" 
                      style={{ fontSize: 11, padding: "4px 10px", borderColor: "#ddd6fe", background: "#f5f3ff", color: "#7c3aed", fontWeight: 600 }}
                      onClick={() => setShowAddChanForm(!showAddChanForm)}
                    >
                      {showAddChanForm ? "✕ 取消添加" : "＋ 添加运维通道/登录链接"}
                    </button>
                  </div>

                  {/* Inline Add Channel Form */}
                  {showAddChanForm && (
                    <div style={{ background: "#f5f3ff", border: "1px solid #ddd6fe", borderRadius: 6, padding: 12, marginBottom: 14 }}>
                      <strong style={{ fontSize: 12, color: "#6d28d9", display: "block", marginBottom: 8 }}>
                        📝 录入新运维通道或登录入口
                      </strong>
                      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 2fr", gap: 8, fontSize: 12 }}>
                        <div>
                          <label style={{ fontSize: 11, color: "#475569" }}>通道名称*</label>
                          <input 
                            placeholder="如 业务管理后台 / SSH 直连" 
                            value={newChanDraft.name} 
                            onChange={e => setNewChanDraft({ ...newChanDraft, name: e.target.value })}
                            style={{ width: "100%", fontSize: 12, padding: 4 }}
                          />
                        </div>
                        <div>
                          <label style={{ fontSize: 11, color: "#475569" }}>通道类型*</label>
                          <select 
                            value={newChanDraft.channelType} 
                            onChange={e => setNewChanDraft({ ...newChanDraft, channelType: e.target.value as any })}
                            style={{ width: "100%", fontSize: 12, padding: 4 }}
                          >
                            <option value="web_link">🌐 Web 页面登录链接</option>
                            <option value="ssh">💻 SSH 远程连接命令</option>
                            <option value="rdp">🖥️ Windows 远程桌面 (RDP)</option>
                            <option value="jumpserver">⚡ JumpServer 堡垒机快速协议</option>
                          </select>
                        </div>
                        <div>
                          <label style={{ fontSize: 11, color: "#475569" }}>目标地址 / 命令行*</label>
                          <input 
                            placeholder="https://... 或 ssh root@... -p 22" 
                            value={newChanDraft.urlOrTarget} 
                            onChange={e => setNewChanDraft({ ...newChanDraft, urlOrTarget: e.target.value })}
                            style={{ width: "100%", fontSize: 12, padding: 4 }}
                          />
                        </div>
                      </div>
                      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8, fontSize: 12, marginTop: 6 }}>
                        <div>
                          <label style={{ fontSize: 11, color: "#475569" }}>账号与认证说明</label>
                          <input 
                            placeholder="如 admin / 证书免密 (无明文密码)" 
                            value={newChanDraft.accountNote} 
                            onChange={e => setNewChanDraft({ ...newChanDraft, accountNote: e.target.value })}
                            style={{ width: "100%", fontSize: 12, padding: 4 }}
                          />
                        </div>
                        <div>
                          <label style={{ fontSize: 11, color: "#475569" }}>备注说明</label>
                          <input 
                            placeholder="如 生产统一认证入口" 
                            value={newChanDraft.remarks} 
                            onChange={e => setNewChanDraft({ ...newChanDraft, remarks: e.target.value })}
                            style={{ width: "100%", fontSize: 12, padding: 4 }}
                          />
                        </div>
                      </div>
                      <div style={{ textAlign: "right", marginTop: 8 }}>
                        <button 
                          type="button" 
                          className="btn-primary"
                          style={{ padding: "4px 12px", fontSize: 12 }}
                          onClick={() => {
                            if (!newChanDraft.name.trim() || !newChanDraft.urlOrTarget.trim()) {
                              alert("请填写通道名称与目标地址！");
                              return;
                            }
                            const newChan: OpsChannel = {
                              id: `chan-${Date.now()}`,
                              assetId: detailAsset.id,
                              assetName: detailAsset.name,
                              assetIp: detailAsset.privateIp || detailAsset.ip,
                              projectId: detailAsset.projectId || "prj-001",
                              projectName: detailAsset.projectName || "工会互助保险信息系统",
                              channelType: newChanDraft.channelType,
                              name: newChanDraft.name,
                              urlOrTarget: newChanDraft.urlOrTarget,
                              accountNote: newChanDraft.accountNote || "运维授权账号",
                              remarks: newChanDraft.remarks || "管理员维护通道"
                            };
                            onAddChannel?.(newChan);
                            setShowAddChanForm(false);
                            setNewChanDraft({ name: "", channelType: "web_link", urlOrTarget: "", accountNote: "", remarks: "" });
                          }}
                        >
                          确认添加通道
                        </button>
                      </div>
                    </div>
                  )}

                  {/* Channel Cards */}
                  <div style={{ display: "grid", gridTemplateColumns: "1fr", gap: 10 }}>
                    {currentAssetChannels.map(chan => {
                      const isWeb = chan.channelType === "web_link";
                      const isSsh = chan.channelType === "ssh";
                      const isRdp = chan.channelType === "rdp";
                      const isVpn = chan.channelType === "vpn";

                      return (
                        <div key={chan.id} style={{
                          border: "1px solid #e2e8f0",
                          borderRadius: 6,
                          padding: "10px 14px",
                          background: isWeb ? "#faf5ff" : isSsh ? "#f8fafc" : "#f0fdf4",
                          display: "flex",
                          justifyContent: "space-between",
                          alignItems: "center",
                          gap: 12
                        }}>
                          <div style={{ minWidth: 0, flex: 1 }}>
                            <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 4 }}>
                              <span style={{
                                fontSize: 10,
                                fontWeight: 700,
                                padding: "1px 6px",
                                borderRadius: 4,
                                background: isWeb ? "#ede9fe" : isSsh ? "#e2e8f0" : "#dcfce7",
                                color: isWeb ? "#6d28d9" : isSsh ? "#334155" : "#15803d"
                              }}>
                                {isWeb ? "🌐 网页链接" : isSsh ? "💻 SSH终端" : isRdp ? "🖥️ 远程桌面" : "🛡️ VPN隧道"}
                              </span>
                              <strong style={{ fontSize: 13, color: "#1e293b" }}>{chan.name}</strong>
                              {chan.accountNote && (
                                <span style={{ fontSize: 11, color: "#64748b" }}>· 账号: {chan.accountNote}</span>
                              )}
                            </div>

                            <div style={{ fontFamily: "monospace", fontSize: 12, color: isWeb ? "#6d28d9" : "#0f172a", background: "#fff", border: "1px solid #cbd5e1", padding: "4px 8px", borderRadius: 4, wordBreak: "break-all" }}>
                              {chan.urlOrTarget}
                            </div>
                            {chan.remarks && (
                              <small style={{ color: "#64748b", display: "block", marginTop: 4 }}>{chan.remarks}</small>
                            )}
                          </div>

                          <div style={{ display: "flex", gap: 6, flexShrink: 0 }}>
                            {isWeb && (
                              <button 
                                type="button" 
                                className="btn-primary" 
                                style={{ padding: "4px 10px", fontSize: 11, background: "#7c3aed", borderColor: "#6d28d9" }}
                                onClick={() => window.open(chan.urlOrTarget, "_blank")}
                              >
                                🔗 立即打开
                              </button>
                            )}
                            <button 
                              type="button" 
                              className="btn-secondary" 
                              style={{ padding: "4px 10px", fontSize: 11 }}
                              onClick={() => copyToClipboard(chan.urlOrTarget, chan.name)}
                            >
                              📋 复制内容
                            </button>
                            <button 
                              type="button" 
                              onClick={() => onDeleteChannel?.(chan.id)}
                              style={{ border: "none", background: "transparent", color: "#ef4444", cursor: "pointer", fontSize: 12, padding: "0 4px" }}
                              title="移除此通道"
                            >
                              ✕
                            </button>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}

              {/* TAB 4: VPN 专网通道 */}
              {detailTab === "vpn" && (
                <div>
                  <h5 style={{ margin: "0 0 10px", fontSize: 14, color: "#ea580c" }}>
                    🛡️ 所属项目 VPN 专网通道配置
                  </h5>
                  <p style={{ margin: "0 0 14px", fontSize: 12, color: "#64748b" }}>
                    运维人员需在外部通过安全 VPN 隧道连接至各政务云/专区，方可访问本机器私网业务与终端。
                  </p>

                  {currentProjectVpn ? (
                    <div style={{
                      background: "#fffaf5",
                      border: "1px solid #fed7aa",
                      borderRadius: 8,
                      padding: 16
                    }}>
                      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 12 }}>
                        <span style={{ fontSize: 20 }}>🛡️</span>
                        <div>
                          <strong style={{ fontSize: 15, color: "#9a3412" }}>{currentProjectVpn.name}</strong>
                          <span style={{ fontSize: 11, color: "#c2410c", background: "#ffedd5", padding: "1px 6px", borderRadius: 4, marginLeft: 8 }}>
                            {currentProjectVpn.vpnClientType || "SSL VPN"}
                          </span>
                        </div>
                      </div>

                      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12, fontSize: 12, marginBottom: 14 }}>
                        <div>
                          <span style={{ color: "#64748b", display: "block" }}>所属项目:</span>
                          <strong>{detailAsset.projectName}</strong>
                        </div>
                        <div>
                          <span style={{ color: "#64748b", display: "block" }}>承载云厂商 / 区域:</span>
                          <strong style={{ color: "#2563eb" }}>{detailAsset.cloudVendor} ({detailAsset.regionName || "互联网专区"})</strong>
                        </div>
                        <div style={{ gridColumn: "span 2" }}>
                          <span style={{ color: "#64748b", display: "block", marginBottom: 2 }}>VPN 认证网关地址:</span>
                          <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                            <code style={{ fontSize: 13, background: "#fff", border: "1px solid #fdba74", padding: "4px 8px", borderRadius: 4, color: "#c2410c", fontWeight: 600, flex: 1 }}>
                              {currentProjectVpn.vpnGateway || currentProjectVpn.urlOrTarget}
                            </code>
                            <button 
                              type="button" 
                              className="btn-primary" 
                              style={{ padding: "4px 10px", fontSize: 11, background: "#ea580c", borderColor: "#c2410c" }}
                              onClick={() => window.open(currentProjectVpn.vpnGateway || currentProjectVpn.urlOrTarget, "_blank")}
                            >
                              🔗 前往认证网关
                            </button>
                            <button 
                              type="button" 
                              className="btn-secondary" 
                              style={{ padding: "4px 10px", fontSize: 11 }}
                              onClick={() => copyToClipboard(currentProjectVpn.vpnGateway || currentProjectVpn.urlOrTarget, "VPN网关地址")}
                            >
                              📋 复制地址
                            </button>
                          </div>
                        </div>
                        <div style={{ gridColumn: "span 2" }}>
                          <span style={{ color: "#64748b", display: "block" }}>拨号账号与策略说明:</span>
                          <div style={{ background: "#fff", border: "1px solid #fed7aa", padding: "6px 10px", borderRadius: 4, color: "#7c2d12" }}>
                            {currentProjectVpn.accountNote || "运维组动态令牌统一口令认证"}
                          </div>
                        </div>
                        <div style={{ gridColumn: "span 2" }}>
                          <span style={{ color: "#64748b", display: "block" }}>路由可达专网网段:</span>
                          <code style={{ fontSize: 12, background: "#fff", border: "1px solid #e2e8f0", padding: "4px 8px", borderRadius: 4, display: "block", color: "#334155" }}>
                            {currentProjectVpn.vpnNetworkSegment || "192.141.20.0/23, 10.200.0.0/16"}
                          </code>
                        </div>
                      </div>

                      <div style={{ borderTop: "1px dashed #fdba74", paddingTop: 12, display: "flex", gap: 10, alignItems: "center" }}>
                        <span style={{ fontSize: 11, color: "#7c2d12", fontWeight: 600 }}>客户端下载指引:</span>
                        <a 
                          href="https://www.sangfor.com.cn/" 
                          target="_blank" 
                          rel="noreferrer" 
                          style={{ fontSize: 11, color: "#2563eb", textDecoration: "underline" }}
                        >
                          [📥 Windows 客户端]
                        </a>
                        <a 
                          href="https://www.sangfor.com.cn/" 
                          target="_blank" 
                          rel="noreferrer" 
                          style={{ fontSize: 11, color: "#2563eb", textDecoration: "underline" }}
                        >
                          [📥 信创 Linux 客户端]
                        </a>
                        <a 
                          href="https://www.sangfor.com.cn/" 
                          target="_blank" 
                          rel="noreferrer" 
                          style={{ fontSize: 11, color: "#2563eb", textDecoration: "underline" }}
                        >
                          [📥 移动端 App]
                        </a>
                      </div>
                    </div>
                  ) : (
                    <div style={{ textAlign: "center", padding: "30px 0", color: "#94a3b8", background: "#f8fafc", borderRadius: 6 }}>
                      该项目暂未配置专用 VPN 专网通道信息。
                    </div>
                  )}
                </div>
              )}
            </div>

            <div className="cmdb-modal-footer">
              <button 
                type="button" 
                className="btn-primary" 
                onClick={() => alert(`已拉取节点 [${detailAsset.name}] 实时监控数据！`)}
              >
                实时探针遥测
              </button>
              <button type="button" className="btn-secondary" onClick={() => setDetailAsset(null)}>关 闭</button>
            </div>
          </div>
        </div>
      )}

      {/* ================= MODAL: ADD ASSET INTO PROJECT ================= */}
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
                    <option value="生产">生产环境</option>
                    <option value="测试">测试环境</option>
                  </select>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 云厂商</label>
                  <select value={newForm.cloudVendor} onChange={e => setNewForm({ ...newForm, cloudVendor: e.target.value })}>
                    <option value="联通云">联通云</option>
                    <option value="首信云">首信云</option>
                    <option value="国企云">国企云</option>
                    <option value="太极云">太极云</option>
                    <option value="阿里云">阿里云</option>
                    <option value="自建机房">自建机房</option>
                  </select>
                </div>
                <div className="form-field-item">
                  <label>* 设备形态</label>
                  <select value={newForm.deviceType} onChange={e => setNewForm({ ...newForm, deviceType: e.target.value })}>
                    <option value="虚拟机">云主机 / 虚拟机</option>
                    <option value="物理机">实体物理服务器</option>
                    <option value="负载均衡">负载均衡设备</option>
                    <option value="对象存储">对象存储节点</option>
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
                      <option value="x86_64">x86_64</option>
                      <option value="ARM64">ARM64</option>
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
                    value={newForm.osVersion} 
                    onChange={e => setNewForm({ ...newForm, osVersion: e.target.value })} 
                  />
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
              </div>
            </div>
            <div className="cmdb-modal-footer">
              <button type="button" className="btn-secondary" onClick={() => setShowProjectVpnModal(false)}>关 闭</button>
            </div>
          </div>
        </div>
      )}

      {/* Excel 台账导入弹窗 */}
      {showImportModal && (
        <ImportModal
          targetProjectName={currentProject ? currentProject.name : null}
          existingAssets={allAssets}
          onClose={() => setShowImportModal(false)}
          onConfirmImport={(imported, strategy) => {
            if (onBatchImportAssets) {
              onBatchImportAssets(imported, strategy, currentProject ? currentProject.name : null);
            }
            const strategyLabel = strategy === "upsert" ? "智能覆盖更新" : strategy === "skip" ? "仅新增(跳过重复)" : "全量替换";
            setToastNotice(`✓ 执行完成 (${strategyLabel})：共处理 ${imported.length} 台设备！`);
            setTimeout(() => setToastNotice(null), 3500);
          }}
        />
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
                    placeholder="如：北京市医疗保障局"
                    value={newProjectForm.customerName}
                    onChange={e => setNewProjectForm({ ...newProjectForm, customerName: e.target.value })}
                    required
                  />
                </div>
                <div className="form-field-item">
                  <label>* 所属环境</label>
                  <select 
                    value={newProjectForm.env}
                    onChange={e => setNewProjectForm({ ...newProjectForm, env: e.target.value })}
                  >
                    <option value="生产">生产环境</option>
                    <option value="测试">测试环境</option>
                    <option value="灾备">灾备环境</option>
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
                    <option value="联通云">联通云</option>
                    <option value="首信云">首信云</option>
                    <option value="国企云">国企云</option>
                    <option value="太极云">太极云</option>
                    <option value="阿里云">阿里云</option>
                    <option value="华为云">华为云</option>
                    <option value="自建机房">自建机房</option>
                  </select>
                </div>
                <div className="form-field-item">
                  <label>* 部署网络区域</label>
                  <input 
                    placeholder="如：政务外网区 / DMZ区 / 专网核心区"
                    value={newProjectForm.regionName}
                    onChange={e => setNewProjectForm({ ...newProjectForm, regionName: e.target.value })}
                    required
                  />
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
                    <option value="生产">生产环境</option>
                    <option value="测试">测试环境</option>
                    <option value="灾备">灾备环境</option>
                  </select>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 云厂商</label>
                  <select value={editForm.cloudVendor} onChange={e => setEditForm({ ...editForm, cloudVendor: e.target.value })}>
                    <option value="联通云">联通云</option>
                    <option value="首信云">首信云</option>
                    <option value="国企云">国企云</option>
                    <option value="太极云">太极云</option>
                    <option value="阿里云">阿里云</option>
                    <option value="华为云">华为云</option>
                    <option value="自建机房">自建机房</option>
                  </select>
                </div>
                <div className="form-field-item">
                  <label>* 设备形态</label>
                  <select value={editForm.deviceType} onChange={e => setEditForm({ ...editForm, deviceType: e.target.value })}>
                    <option value="虚拟机">云主机 / 虚拟机</option>
                    <option value="物理机">实体物理服务器</option>
                    <option value="负载均衡">负载均衡设备</option>
                    <option value="对象存储">对象存储节点</option>
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
                      <option value="x86_64">x86_64</option>
                      <option value="ARM64">ARM64</option>
                      <option value="LoongArch">LoongArch (龙芯)</option>
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
                    value={editForm.osVersion} 
                    onChange={e => setEditForm({ ...editForm, osVersion: e.target.value })} 
                  />
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
        <div className="cmdb-modal-overlay" style={{ zIndex: 9999 }}>
          <div className="cmdb-modal-content" style={{ width: 850, maxWidth: "95vw", maxHeight: "90vh", display: "flex", flexDirection: "column" }}>
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
                          `${a.name}\t${a.privateIp || a.ip}\t${a.projectName}\t${a.customerName}\t${a.osVersion || a.os}`
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
                        <th style={{ width: 60, textAlign: "center" }}>定位</th>
                      </tr>
                    </thead>
                    <tbody>
                      {batchIpAnalysis.matchedAssets.map((asset, idx) => (
                        <tr key={asset.id}>
                          <td style={{ fontFamily: "monospace", color: "#64748b" }}>{idx + 1}</td>
                          <td>
                            <strong>{asset.name}</strong>
                          </td>
                          <td>
                            <code style={{ color: "#2563eb", fontWeight: 600 }}>{asset.privateIp || asset.ip || "-"}</code>
                          </td>
                          <td>
                            <code style={{ color: "#64748b" }}>{asset.internalWanIp || asset.vip || "-"}</code>
                          </td>
                          <td>
                            <div>{asset.projectName}</div>
                            <small style={{ color: "#94a3b8" }}>{asset.customerName}</small>
                          </td>
                          <td>{asset.osVersion || asset.os || "-"}</td>
                          <td style={{ textAlign: "center" }}>
                            <button
                              type="button"
                              className="btn-secondary"
                              style={{ fontSize: 10, padding: "2px 6px" }}
                              onClick={() => {
                                const targetProj = projects.find(p => p.name === asset.projectName);
                                if (targetProj) setSelectedProjectId(targetProj.id);
                                else setSelectedProjectId("all");
                                setIpSearchKeyword(asset.privateIp || asset.ip || "");
                                setIpSearchInput(asset.privateIp || asset.ip || "");
                                setShowIpModal(false);
                                setCurrentPage(1);
                              }}
                              title="点击在工作台定位并打开该设备"
                            >
                              定位
                            </button>
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