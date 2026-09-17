"use client";
import React, { useState, useMemo } from "react";
import { PhysicalHost, VmHost, IdcRoom, IdcCabinet, BusinessModel, AssetMeta } from "../cmdbData";

interface HostManagementProps {
  hosts: PhysicalHost[];
  vms: VmHost[];
  rooms: IdcRoom[];
  cabinets: IdcCabinet[];
  businesses: BusinessModel[];
  onAddHost: (h: PhysicalHost) => void;
  onDeleteHost: (id: string) => void;
  onAddVm: (v: VmHost) => void;
  onDeleteVm: (id: string) => void;
  onJumpToRack?: (roomId: string, cabinetId: string) => void;
}

type CombinedAsset = (PhysicalHost | VmHost) & {
  _kind: "physical" | "vm";
};

export default function HostManagement({
  hosts,
  vms,
  rooms,
  cabinets,
  businesses,
  onAddHost,
  onDeleteHost,
  onAddVm,
  onDeleteVm,
  onJumpToRack
}: HostManagementProps) {
  // Tab: physical, vm, all
  const [tab, setTab] = useState<"vm" | "physical" | "all">("all");
  
  // Filters
  const [keyword, setKeyword] = useState("");
  const [selectedRoom, setSelectedRoom] = useState("全部");
  const [selectedCloud, setSelectedCloud] = useState("全部");
  const [selectedEnv, setSelectedEnv] = useState("全部");
  const [selectedXinchuang, setSelectedXinchuang] = useState("全部");
  const [selectedCustomer, setSelectedCustomer] = useState("全部");

  // Pagination
  const [pageSize, setPageSize] = useState(15);
  const [currentPage, setCurrentPage] = useState(1);

  // Detail Modal
  const [detailAsset, setDetailAsset] = useState<CombinedAsset | null>(null);

  // New VM Modal
  const [showVmModal, setShowVmModal] = useState(false);
  const [vmForm, setVmForm] = useState({
    name: "",
    physicalHostId: hosts[0]?.id || "phy-1",
    customerName: "北京市人力资源和社会保障局",
    projectName: "原三险系统",
    env: "生产",
    cloudVendor: "联通云",
    regionName: "政务外网区",
    roomName: "六里桥机房",
    ip: "192.125.31.250",
    privateIp: "192.125.31.250",
    internalWanIp: "192.123.2.100",
    eip: "",
    vip: "",
    publicIp: "",
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
    businessId: businesses[0]?.id || "biz-1",
    remarks: "新增业务微服务节点"
  });

  // Combine assets with tags
  const allCombinedAssets: CombinedAsset[] = useMemo(() => {
    const pList: CombinedAsset[] = hosts.map(h => ({ ...h, _kind: "physical" }));
    const vList: CombinedAsset[] = vms.map(v => ({ ...v, _kind: "vm" }));
    return [...pList, ...vList].sort((a, b) => (a.seq || 9999) - (b.seq || 9999));
  }, [hosts, vms]);

  // Extract unique filter dropdown values
  const cloudList = useMemo(() => {
    const s = new Set<string>();
    allCombinedAssets.forEach(a => { if (a.cloudVendor) s.add(a.cloudVendor); });
    return Array.from(s);
  }, [allCombinedAssets]);

  const customerList = useMemo(() => {
    const s = new Set<string>();
    allCombinedAssets.forEach(a => { if (a.customerName) s.add(a.customerName); });
    return Array.from(s);
  }, [allCombinedAssets]);

  // Filtered assets
  const filteredAssets = useMemo(() => {
    return allCombinedAssets.filter(item => {
      // Tab filter
      if (tab === "physical" && item._kind !== "physical") return false;
      if (tab === "vm" && item._kind !== "vm") return false;

      // Dropdown filters
      if (selectedRoom !== "全部") {
        const itemRoom = item.roomName || rooms.find(r => r.id === (item as any).roomId)?.name;
        if (itemRoom !== selectedRoom) return false;
      }

      if (selectedCloud !== "全部" && item.cloudVendor !== selectedCloud) return false;
      if (selectedEnv !== "全部" && item.env !== selectedEnv) return false;
      if (selectedXinchuang !== "全部" && item.isXinchuang !== selectedXinchuang) return false;
      if (selectedCustomer !== "全部" && item.customerName !== selectedCustomer) return false;

      // Keyword search
      if (keyword.trim()) {
        const kw = keyword.toLowerCase();
        const str = [
          item.name,
          (item as any).hostname,
          item.ip,
          item.privateIp,
          item.internalWanIp,
          item.vip,
          item.eip,
          item.customerName,
          item.projectName,
          item.remarks,
          item.osVersion
        ].filter(Boolean).join(" ").toLowerCase();

        if (!str.includes(kw)) return false;
      }

      return true;
    });
  }, [allCombinedAssets, tab, selectedRoom, selectedCloud, selectedEnv, selectedXinchuang, selectedCustomer, keyword, rooms]);

  // Pagination calculation
  const totalPages = Math.ceil(filteredAssets.length / pageSize) || 1;
  const pageAssets = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredAssets.slice(start, start + pageSize);
  }, [filteredAssets, currentPage, pageSize]);

  // Handle Create VM
  function handleCreateVmSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!vmForm.name.trim()) return;

    const newSeq = (allCombinedAssets[allCombinedAssets.length - 1]?.seq || 238) + 1;
    const roomObj = rooms.find(r => r.name === vmForm.roomName) || rooms[0];

    const newVm: VmHost = {
      id: `vm-${Date.now()}`,
      seq: newSeq,
      name: vmForm.name,
      physicalHostId: vmForm.physicalHostId,
      roomId: roomObj.id,
      roomName: roomObj.name,
      ip: vmForm.ip,
      privateIp: vmForm.privateIp,
      cpu: `${vmForm.cpuCores} 核 vCPU (${vmForm.cpuArch})`,
      memory: `${vmForm.memoryGb} GB`,
      disk: `${vmForm.systemDiskGb}G(系统) + ${vmForm.dataDiskGb}G(数据)`,
      os: `${vmForm.osFamily} ${vmForm.osVersion}`,
      businessId: vmForm.businessId,
      status: "running",
      role: "应用微服务节点",
      updated: new Date().toLocaleString("zh-CN", { hour12: false }).slice(0, 16),
      // Excel fields
      customerName: vmForm.customerName,
      projectName: vmForm.projectName,
      env: vmForm.env,
      cloudVendor: vmForm.cloudVendor,
      regionName: vmForm.regionName,
      category: "服务器",
      deviceType: "虚拟机",
      internalWanIp: vmForm.internalWanIp || null,
      eip: vmForm.eip || null,
      vip: vmForm.vip || null,
      publicIp: vmForm.publicIp || null,
      cpuArch: vmForm.cpuArch,
      cpuCores: vmForm.cpuCores,
      memoryGb: vmForm.memoryGb,
      systemDiskGb: vmForm.systemDiskGb,
      dataDiskGb: vmForm.dataDiskGb,
      osFamily: vmForm.osFamily,
      osVersion: vmForm.osVersion,
      kernelVersion: vmForm.kernelVersion,
      isXinchuang: vmForm.isXinchuang,
      remotePort: vmForm.remotePort,
      remarks: vmForm.remarks
    };

    onAddVm(newVm);
    setShowVmModal(false);
  }

  return (
    <div className="cmdb-container">
      {/* Top Filter & Metric Bar */}
      <div className="cmdb-table-filter" style={{ flexWrap: "wrap", gap: 10 }}>
        {/* Left Segment Switch */}
        <div style={{ display: "flex", background: "#e2e8f0", padding: 2, borderRadius: 6 }}>
          <button 
            className="btn-secondary"
            style={{
              background: tab === "all" ? "#2563eb" : "transparent",
              color: tab === "all" ? "#fff" : "#475569",
              border: 0,
              padding: "6px 12px",
              fontWeight: 500
            }}
            onClick={() => { setTab("all"); setCurrentPage(1); }}
          >
            全部资产 ({allCombinedAssets.length})
          </button>
          <button 
            className="btn-secondary"
            style={{
              background: tab === "vm" ? "#2563eb" : "transparent",
              color: tab === "vm" ? "#fff" : "#475569",
              border: 0,
              padding: "6px 12px",
              fontWeight: 500
            }}
            onClick={() => { setTab("vm"); setCurrentPage(1); }}
          >
            云主机 / 虚拟服务器 ({vms.length})
          </button>
          <button 
            className="btn-secondary"
            style={{
              background: tab === "physical" ? "#2563eb" : "transparent",
              color: tab === "physical" ? "#fff" : "#475569",
              border: 0,
              padding: "6px 12px",
              fontWeight: 500
            }}
            onClick={() => { setTab("physical"); setCurrentPage(1); }}
          >
            物理服务器 / 存储 ({hosts.length})
          </button>
        </div>

        {/* Filter Selects */}
        <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
          <input 
            placeholder="搜索设备名称 / IP / 业务项目 / 客户..." 
            value={keyword}
            onChange={e => { setKeyword(e.target.value); setCurrentPage(1); }}
            style={{ width: 220 }}
          />

          <select value={selectedRoom} onChange={e => { setSelectedRoom(e.target.value); setCurrentPage(1); }}>
            <option value="全部">全部机房 (8大IDC)</option>
            {rooms.map(r => (
              <option key={r.id} value={r.name}>{r.name}</option>
            ))}
          </select>

          <select value={selectedCloud} onChange={e => { setSelectedCloud(e.target.value); setCurrentPage(1); }}>
            <option value="全部">全部云厂商</option>
            {cloudList.map(c => (
              <option key={c} value={c}>{c}</option>
            ))}
          </select>

          <select value={selectedCustomer} onChange={e => { setSelectedCustomer(e.target.value); setCurrentPage(1); }}>
            <option value="全部">全部客户单位</option>
            {customerList.map(cust => (
              <option key={cust} value={cust}>{cust}</option>
            ))}
          </select>

          <select value={selectedEnv} onChange={e => { setSelectedEnv(e.target.value); setCurrentPage(1); }}>
            <option value="全部">全部环境</option>
            <option value="生产">生产环境</option>
            <option value="测试">测试环境</option>
          </select>

          <select value={selectedXinchuang} onChange={e => { setSelectedXinchuang(e.target.value); setCurrentPage(1); }}>
            <option value="全部">全部OS架构</option>
            <option value="是">国产信创 OS</option>
            <option value="否">常规 OS</option>
          </select>

          {(keyword || selectedRoom !== "全部" || selectedCloud !== "全部" || selectedCustomer !== "全部" || selectedEnv !== "全部" || selectedXinchuang !== "全部") && (
            <button 
              className="btn-secondary" 
              style={{ fontSize: 11, padding: "5px 8px" }}
              onClick={() => {
                setKeyword("");
                setSelectedRoom("全部");
                setSelectedCloud("全部");
                setSelectedCustomer("全部");
                setSelectedEnv("全部");
                setSelectedXinchuang("全部");
                setCurrentPage(1);
              }}
            >
              ✕ 清除筛选
            </button>
          )}
        </div>

        {/* Action Button */}
        <div>
          <button className="btn-primary" onClick={() => setShowVmModal(true)}>
            ＋ 录入新资产
          </button>
        </div>
      </div>

      {/* ASSET DATA TABLE */}
      <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, overflow: "hidden" }}>
        <div style={{ overflowX: "auto" }}>
          <table className="cmdb-data-table" style={{ minWidth: 1280 }}>
            <thead>
              <tr>
                <th style={{ width: 45 }}>序号</th>
                <th style={{ width: 220 }}>设备名称 / 类型</th>
                <th style={{ width: 180 }}>客户名称 · 业务系统</th>
                <th style={{ width: 140 }}>环境 · 云厂商 · 区域</th>
                <th style={{ width: 110 }}>所属机房</th>
                <th style={{ width: 160 }}>私有业务 IP / 内大网</th>
                <th style={{ width: 130 }}>EIP / VIP</th>
                <th style={{ width: 150 }}>CPU / 内存 / 磁盘</th>
                <th style={{ width: 150 }}>操作系统 / 信创</th>
                <th style={{ width: 70 }}>远程端口</th>
                <th style={{ width: 140 }}>备注说明</th>
                <th style={{ width: 110, textAlign: "center" }}>操作</th>
              </tr>
            </thead>
            <tbody>
              {pageAssets.length === 0 ? (
                <tr>
                  <td colSpan={12} style={{ textAlign: "center", padding: "30px 0", color: "#94a3b8" }}>
                    暂无符合条件的资产记录
                  </td>
                </tr>
              ) : (
                pageAssets.map(item => {
                  const isPhysical = item._kind === "physical";
                  const phy = isPhysical ? (item as PhysicalHost) : null;
                  const roomName = item.roomName || rooms.find(r => r.id === (item as any).roomId)?.name || "默认机房";

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
                            borderRadius: 4, 
                            fontWeight: 600,
                            background: isPhysical ? "#fef3c7" : "#e0f2fe", 
                            color: isPhysical ? "#b45309" : "#0369a1" 
                          }}>
                            {isPhysical ? "物理机" : item.deviceType || "云主机"}
                          </span>
                          <strong 
                            style={{ color: "#0f172a", cursor: "pointer" }}
                            title="点击查看全量台账字段"
                            onClick={() => setDetailAsset(item)}
                          >
                            {item.name}
                          </strong>
                        </div>
                        {isPhysical && phy && (
                          <small style={{ color: "#64748b", display: "block", marginTop: 2 }}>
                            {phy.brand} {phy.model} · 资产码: {phy.assetNo}
                          </small>
                        )}
                      </td>

                      {/* Customer & Project */}
                      <td>
                        <div style={{ fontWeight: 500, color: "#1e293b" }}>{item.projectName || "通用业务"}</div>
                        <small style={{ color: "#64748b", display: "block" }}>{item.customerName || "北控伟仕"}</small>
                      </td>

                      {/* Env & Cloud & Region */}
                      <td>
                        <div style={{ display: "flex", gap: 4, flexWrap: "wrap", alignItems: "center" }}>
                          <span style={{
                            padding: "1px 6px",
                            borderRadius: 4,
                            fontSize: 11,
                            background: item.env === "生产" ? "#dcfce7" : "#f1f5f9",
                            color: item.env === "生产" ? "#15803d" : "#475569"
                          }}>
                            {item.env || "生产"}
                          </span>
                          <span style={{ color: "#2563eb", fontWeight: 500 }}>{item.cloudVendor || "自建"}</span>
                        </div>
                        <small style={{ color: "#64748b", display: "block", marginTop: 2 }}>{item.regionName || "-"}</small>
                      </td>

                      {/* Room */}
                      <td>
                        <span className="room-badge" style={{ background: "#f8fafc", border: "1px solid #e2e8f0" }}>
                          🏢 {roomName}
                        </span>
                        {isPhysical && phy && (
                          <div style={{ marginTop: 2 }}>
                            <button 
                              style={{ 
                                background: "#eff6ff", 
                                border: "1px solid #bfdbfe", 
                                color: "#1d4ed8", 
                                padding: "1px 6px", 
                                borderRadius: 3, 
                                cursor: "pointer", 
                                fontSize: 10 
                              }}
                              onClick={() => onJumpToRack?.(phy.roomId, phy.cabinetId)}
                              title="跳转至 42U 立面机架"
                            >
                              📍 {phy.startU}U-{phy.startU + phy.uHeight - 1}U
                            </button>
                          </div>
                        )}
                      </td>

                      {/* IP Addresses */}
                      <td>
                        <div style={{ fontFamily: "monospace", fontWeight: 600, color: "#2563eb" }}>
                          {item.privateIp || item.ip || "-"}
                        </div>
                        {item.internalWanIp && (
                          <small style={{ fontFamily: "monospace", color: "#64748b", display: "block" }}>
                            内网: {item.internalWanIp}
                          </small>
                        )}
                      </td>

                      {/* EIP / VIP */}
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
                          盘: {item.systemDiskGb || 30}G系统 {item.dataDiskGb ? `+ ${item.dataDiskGb}G数据` : ""}
                        </small>
                      </td>

                      {/* OS & Xinchuang */}
                      <td>
                        <div style={{ display: "flex", alignItems: "center", gap: 4 }}>
                          {item.isXinchuang === "是" && (
                            <span 
                              style={{ background: "#fee2e2", color: "#b91c1c", fontSize: 10, padding: "1px 4px", borderRadius: 3, fontWeight: 700 }}
                              title="国产信创操作系统认证"
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
                            title="查看详细字段"
                          >
                            档案
                          </button>
                          <button 
                            className="btn-secondary" 
                            style={{ padding: "2px 6px", fontSize: 11, color: "#2563eb" }}
                            onClick={() => alert(`已为节点 [${item.name} (${item.privateIp || item.ip})] 发起 Web SSH 终端会话！`)}
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

        {/* Pagination Bar */}
        <div style={{ 
          display: "flex", 
          justifyContent: "space-between", 
          alignItems: "center", 
          padding: "10px 16px", 
          background: "#f8fafc", 
          borderTop: "1px solid #e2e8f0",
          fontSize: 12,
          color: "#475569"
        }}>
          <div>
            共 <strong style={{ color: "#0f172a" }}>{filteredAssets.length}</strong> 条资产记录
            {filteredAssets.length !== allCombinedAssets.length && ` (已从 ${allCombinedAssets.length} 条中筛选)`}
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

      {/* DETAIL MODAL: FULL EXCEL ASSET METADATA */}
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
                  {detailAsset.customerName} · {detailAsset.projectName} · {detailAsset.env}环境
                </small>
              </div>
              <button type="button" className="cmdb-modal-close" style={{ color: "#fff" }} onClick={() => setDetailAsset(null)}>×</button>
            </div>

            <div className="cmdb-modal-body" style={{ maxHeight: "72vh", overflowY: "auto", padding: 20 }}>
              {/* Section 1: 基本信息 */}
              <h5 style={{ margin: "0 0 10px", fontSize: 13, color: "#0284c7", borderBottom: "1px solid #e0f2fe", paddingBottom: 4 }}>
                📋 基本资产信息 (Excel L1/L2)
              </h5>
              <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 12, fontSize: 12, marginBottom: 16 }}>
                <div><span style={{ color: "#64748b" }}>客户名称:</span> <strong>{detailAsset.customerName || "-"}</strong></div>
                <div><span style={{ color: "#64748b" }}>项目名称:</span> <strong>{detailAsset.projectName || "-"}</strong></div>
                <div><span style={{ color: "#64748b" }}>环境类型:</span> <strong>{detailAsset.env || "-"}</strong></div>
                <div><span style={{ color: "#64748b" }}>云厂商:</span> <strong style={{ color: "#2563eb" }}>{detailAsset.cloudVendor || "-"}</strong></div>
                <div><span style={{ color: "#64748b" }}>安全区域:</span> <strong>{detailAsset.regionName || "-"}</strong></div>
                <div><span style={{ color: "#64748b" }}>机房分布:</span> <strong>{detailAsset.roomName || "-"}</strong></div>
                <div><span style={{ color: "#64748b" }}>设备大类:</span> <strong>{detailAsset.category || "服务器"}</strong></div>
                <div><span style={{ color: "#64748b" }}>设备类型:</span> <strong>{detailAsset.deviceType || "虚拟机"}</strong></div>
                <div><span style={{ color: "#64748b" }}>资产形态:</span> <strong>{detailAsset._kind === "physical" ? "实体物理机" : "虚拟计算节点"}</strong></div>
              </div>

              {/* Section 2: 网络网络地址 */}
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

              {/* Section 3: 硬件与资源配置 */}
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

              {/* Section 4: 操作系统及内核 */}
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

              {/* Section 5: 备注 */}
              {detailAsset.remarks && (
                <div style={{ background: "#fffbeb", border: "1px solid #fde68a", padding: 10, borderRadius: 6, fontSize: 12 }}>
                  <strong style={{ color: "#92400e" }}>📌 资产档案备注：</strong>
                  <span style={{ color: "#78350f", marginLeft: 6 }}>{detailAsset.remarks}</span>
                </div>
              )}
            </div>

            <div className="cmdb-modal-footer">
              <button 
                type="button" 
                className="btn-primary" 
                onClick={() => {
                  alert(`已拉取节点 [${detailAsset.name}] 的实时监控与进程遥测数据！`);
                }}
              >
                实时性能分析
              </button>
              <button type="button" className="btn-secondary" onClick={() => setDetailAsset(null)}>关 闭</button>
            </div>
          </div>
        </div>
      )}

      {/* MODAL: CREATE ASSET (EXCEL COMPLIANT) */}
      {showVmModal && (
        <div className="cmdb-modal-mask">
          <form className="cmdb-modal" style={{ maxWidth: 740, width: "95%" }} onSubmit={handleCreateVmSubmit}>
            <div className="cmdb-modal-header">
              <h3>📝 录入新信息资产（严格按台账标准）</h3>
              <button type="button" className="cmdb-modal-close" onClick={() => setShowVmModal(false)}>×</button>
            </div>
            <div className="cmdb-modal-body" style={{ maxHeight: "72vh", overflowY: "auto", padding: 20 }}>
              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 设备名称</label>
                  <input 
                    placeholder="如：人社局-gov-北控伟仕-生产-新应用节点" 
                    value={vmForm.name} 
                    onChange={e => setVmForm({ ...vmForm, name: e.target.value })} 
                    required 
                  />
                </div>
                <div className="form-field-item">
                  <label>* 客户名称</label>
                  <input 
                    value={vmForm.customerName} 
                    onChange={e => setVmForm({ ...vmForm, customerName: e.target.value })} 
                    required 
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 项目名称 / 业务系统</label>
                  <input 
                    value={vmForm.projectName} 
                    onChange={e => setVmForm({ ...vmForm, projectName: e.target.value })} 
                    required 
                  />
                </div>
                <div className="form-field-item">
                  <label>* 环境</label>
                  <select value={vmForm.env} onChange={e => setVmForm({ ...vmForm, env: e.target.value })}>
                    <option value="生产">生产环境</option>
                    <option value="测试">测试环境</option>
                  </select>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 云厂商</label>
                  <select value={vmForm.cloudVendor} onChange={e => setVmForm({ ...vmForm, cloudVendor: e.target.value })}>
                    <option value="联通云">联通云</option>
                    <option value="首信云">首信云</option>
                    <option value="国企云">国企云</option>
                    <option value="太极云">太极云</option>
                    <option value="阿里云">阿里云</option>
                    <option value="自建机房">自建机房</option>
                  </select>
                </div>
                <div className="form-field-item">
                  <label>* 所属机房</label>
                  <select value={vmForm.roomName} onChange={e => setVmForm({ ...vmForm, roomName: e.target.value })}>
                    {rooms.map(r => (
                      <option key={r.id} value={r.name}>{r.name}</option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 私有IP（业务IP）</label>
                  <input 
                    placeholder="如 192.125.31.250"
                    value={vmForm.ip} 
                    onChange={e => setVmForm({ ...vmForm, ip: e.target.value, privateIp: e.target.value })} 
                    required 
                  />
                </div>
                <div className="form-field-item">
                  <label>内大网IP</label>
                  <input 
                    placeholder="如 192.123.2.100"
                    value={vmForm.internalWanIp} 
                    onChange={e => setVmForm({ ...vmForm, internalWanIp: e.target.value })} 
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>EIP 地址</label>
                  <input 
                    placeholder="如 172.26.53.180"
                    value={vmForm.eip} 
                    onChange={e => setVmForm({ ...vmForm, eip: e.target.value })} 
                  />
                </div>
                <div className="form-field-item">
                  <label>VIP 地址</label>
                  <input 
                    placeholder="如 172.25.147.250"
                    value={vmForm.vip} 
                    onChange={e => setVmForm({ ...vmForm, vip: e.target.value })} 
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>CPU 架构与核数</label>
                  <div style={{ display: "flex", gap: 8 }}>
                    <select value={vmForm.cpuArch} onChange={e => setVmForm({ ...vmForm, cpuArch: e.target.value })}>
                      <option value="x86_64">x86_64</option>
                      <option value="ARM64">ARM64</option>
                    </select>
                    <input 
                      type="number" 
                      placeholder="核数" 
                      value={vmForm.cpuCores} 
                      onChange={e => setVmForm({ ...vmForm, cpuCores: Number(e.target.value) })} 
                    />
                  </div>
                </div>
                <div className="form-field-item">
                  <label>内存 (GB)</label>
                  <input 
                    type="number" 
                    value={vmForm.memoryGb} 
                    onChange={e => setVmForm({ ...vmForm, memoryGb: Number(e.target.value) })} 
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
                      value={vmForm.systemDiskGb} 
                      onChange={e => setVmForm({ ...vmForm, systemDiskGb: Number(e.target.value) })} 
                    />
                    <input 
                      type="number" 
                      placeholder="数据盘GB" 
                      value={vmForm.dataDiskGb} 
                      onChange={e => setVmForm({ ...vmForm, dataDiskGb: Number(e.target.value) })} 
                    />
                  </div>
                </div>
                <div className="form-field-item">
                  <label>是否信创 OS</label>
                  <select value={vmForm.isXinchuang} onChange={e => setVmForm({ ...vmForm, isXinchuang: e.target.value })}>
                    <option value="否">否 (CentOS / RedHat 等)</option>
                    <option value="是">是 (麒麟 / 统信UOS 等)</option>
                  </select>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>OS 发行版与版本</label>
                  <input 
                    value={vmForm.osVersion} 
                    onChange={e => setVmForm({ ...vmForm, osVersion: e.target.value })} 
                  />
                </div>
                <div className="form-field-item">
                  <label>远程端口</label>
                  <input 
                    type="number" 
                    value={vmForm.remotePort} 
                    onChange={e => setVmForm({ ...vmForm, remotePort: Number(e.target.value) })} 
                  />
                </div>
              </div>

              <div className="form-field-item">
                <label>备注说明</label>
                <textarea 
                  rows={2}
                  value={vmForm.remarks}
                  onChange={e => setVmForm({ ...vmForm, remarks: e.target.value })}
                />
              </div>
            </div>
            <div className="cmdb-modal-footer">
              <button type="button" className="btn-secondary" onClick={() => setShowVmModal(false)}>取 消</button>
              <button type="submit" className="btn-primary">确 定 录 入</button>
            </div>
          </form>
        </div>
      )}
    </div>
  );
}
