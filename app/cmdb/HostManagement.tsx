"use client";
import React, { useState, useMemo } from "react";
import { PhysicalHost, VmHost, SwitchDevice, DatabaseAsset, ProjectGroup, AssetMeta } from "../cmdbData";

interface HostManagementProps {
  projects: ProjectGroup[];
  hosts: PhysicalHost[];
  vms: VmHost[];
  switches?: SwitchDevice[];
  databases?: DatabaseAsset[];
  onAddHost?: (h: PhysicalHost) => void;
  onDeleteHost?: (id: string) => void;
  onAddVm: (v: VmHost) => void;
  onDeleteVm: (id: string) => void;
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
  onAddHost,
  onDeleteHost,
  onAddVm,
  onDeleteVm
}: HostManagementProps) {
  // Selected project ID ("all" for all assets)
  const [selectedProjectId, setSelectedProjectId] = useState<string>(projects[0]?.id || "all");
  
  // Left Sidebar Project Search & Filter
  const [projectSearch, setProjectSearch] = useState("");
  const [selectedCustomerFilter, setSelectedCustomerFilter] = useState("全部");

  // Right Table Filters
  const [assetKeyword, setAssetKeyword] = useState("");
  const [deviceTypeFilter, setDeviceTypeFilter] = useState("全部");
  const [xinchuangFilter, setXinchuangFilter] = useState("全部");

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

      return true;
    });
  }, [allAssets, selectedProjectId, currentProject, deviceTypeFilter, xinchuangFilter, assetKeyword]);

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
            <span style={{ fontSize: 11, color: "#64748b", background: "#e2e8f0", padding: "1px 6px", borderRadius: 10 }}>
              {projects.length} 个项目
            </span>
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

              <button className="btn-primary" onClick={() => setShowAddModal(true)} style={{ padding: "8px 12px" }}>
                ＋ 录入项目资产
              </button>
            </div>
          </div>
        </div>

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
              placeholder="搜索当前项目设备名称 / IP / 备注..." 
              value={assetKeyword}
              onChange={e => { setAssetKeyword(e.target.value); setCurrentPage(1); }}
              style={{ width: 220, fontSize: 12 }}
            />

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

            {(assetKeyword || deviceTypeFilter !== "全部" || xinchuangFilter !== "全部") && (
              <button 
                className="btn-secondary"
                style={{ fontSize: 11, padding: "3px 8px" }}
                onClick={() => {
                  setAssetKeyword("");
                  setDeviceTypeFilter("全部");
                  setXinchuangFilter("全部");
                  setCurrentPage(1);
                }}
              >
                ✕ 重置筛选
              </button>
            )}
          </div>

          <span style={{ fontSize: 12, color: "#64748b" }}>
            共 <strong style={{ color: "#0f172a" }}>{displayedAssets.length}</strong> 台资产记录
          </span>
        </div>

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
                              onClick={() => setDetailAsset(item)}
                              title="查看资产档案"
                            >
                              档案
                            </button>
                            <button 
                              className="btn-secondary" 
                              style={{ padding: "2px 6px", fontSize: 11, color: "#2563eb" }}
                              onClick={() => alert(`已为 [${item.name} (${item.privateIp || item.ip})] 开启终端会话！`)}
                            >
                              终端
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

            <div className="cmdb-modal-body" style={{ maxHeight: "72vh", overflowY: "auto", padding: 20 }}>
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
    </div>
  );
}
