"use client";
import React, { useState, useMemo } from "react";
import { BusinessModel, PhysicalHost, VmHost, DatabaseAsset, SwitchDevice, IdcRoom, IdcCabinet } from "../cmdbData";

interface ServiceModelProps {
  businesses: BusinessModel[];
  hosts: PhysicalHost[];
  vms?: VmHost[];
  databases: DatabaseAsset[];
  switches: SwitchDevice[];
  rooms: IdcRoom[];
  cabinets: IdcCabinet[];
  onJumpToRack?: (roomId: string, cabinetId: string) => void;
}

export default function ServiceModel({
  businesses,
  hosts,
  vms = [],
  databases,
  switches,
  rooms,
  cabinets,
  onJumpToRack
}: ServiceModelProps) {
  const [selectedBizId, setSelectedBizId] = useState<string>(businesses[0]?.id || "biz-1");
  const [keyword, setKeyword] = useState("");
  const [selectedLevel, setSelectedLevel] = useState("全部");
  const [inspectedNode, setInspectedNode] = useState<{
    title: string;
    type: string;
    detail: string;
    extra?: string;
    roomId?: string;
    cabinetId?: string;
  } | null>(null);

  // Filter businesses
  const filteredBusinesses = useMemo(() => {
    return businesses.filter(b => {
      if (selectedLevel !== "全部" && b.level !== selectedLevel) return false;
      if (keyword.trim()) {
        const kw = keyword.toLowerCase();
        return b.name.toLowerCase().includes(kw) || b.department.toLowerCase().includes(kw) || b.code.toLowerCase().includes(kw);
      }
      return true;
    });
  }, [businesses, selectedLevel, keyword]);

  const selectedBiz = useMemo(() => {
    return businesses.find(b => b.id === selectedBizId) || filteredBusinesses[0] || businesses[0];
  }, [businesses, selectedBizId, filteredBusinesses]);

  // Linked assets for this selected business
  const linkedVms = useMemo(() => {
    return vms.filter(v => v.businessId === selectedBiz.id || v.projectName === selectedBiz.name);
  }, [vms, selectedBiz]);

  const linkedHosts = useMemo(() => {
    // Hosts directly assigned or hosting linked vms
    const directHosts = hosts.filter(h => h.businessId === selectedBiz.id || h.projectName === selectedBiz.name);
    if (directHosts.length > 0) return directHosts;
    // Otherwise find hypervisor host of the first linked vm
    const parentHostId = linkedVms[0]?.physicalHostId;
    const parentHost = hosts.find(h => h.id === parentHostId);
    return parentHost ? [parentHost] : [hosts[0]];
  }, [hosts, selectedBiz, linkedVms]);

  const linkedDbs = useMemo(() => {
    return databases.filter(d => d.businessId === selectedBiz.id);
  }, [databases, selectedBiz]);

  const linkedSwitches = useMemo(() => {
    const sws = switches.filter(s => s.businessId === selectedBiz.id);
    return sws.length > 0 ? sws : [switches[0]];
  }, [switches, selectedBiz]);

  return (
    <div className="cmdb-container">
      {/* Top Filter and Business Switcher Header */}
      <div className="cmdb-table-filter" style={{ flexWrap: "wrap", gap: 10 }}>
        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          <h3 style={{ margin: 0, fontSize: 16, color: "#0f172a" }}>🏛 业务系统全链路架构拓扑</h3>
          <span className="room-badge">{businesses.length} 个纳管业务系统</span>
        </div>

        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          <input 
            placeholder="搜索业务系统 / 客户单位 / 编码..." 
            value={keyword}
            onChange={e => setKeyword(e.target.value)}
            style={{ width: 220 }}
          />
          <select value={selectedLevel} onChange={e => setSelectedLevel(e.target.value)}>
            <option value="全部">全部等级</option>
            <option value="核心 L1">核心 L1</option>
            <option value="重要 L2">重要 L2</option>
            <option value="通用 L3">通用 L3</option>
          </select>
        </div>
      </div>

      {/* Horizontal Business Carousel Bar */}
      <div style={{ 
        display: "flex", 
        gap: 10, 
        overflowX: "auto", 
        paddingBottom: 6, 
        marginBottom: 12 
      }}>
        {filteredBusinesses.map(b => {
          const isSelected = b.id === selectedBiz.id;
          const devCount = (b.hostIds?.length || 0) + (b.switchIds?.length || 0);

          return (
            <div 
              key={b.id}
              style={{
                minWidth: 220,
                background: isSelected ? "#eff6ff" : "#fff",
                border: isSelected ? "2px solid #2563eb" : "1px solid #e2e8f0",
                borderRadius: 8,
                padding: "10px 12px",
                cursor: "pointer",
                boxShadow: isSelected ? "0 4px 12px rgba(37,99,235,0.12)" : "0 1px 3px rgba(0,0,0,0.04)",
                transition: "all 0.15s",
                flexShrink: 0
              }}
              onClick={() => {
                setSelectedBizId(b.id);
                setInspectedNode(null);
              }}
            >
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 4 }}>
                <span style={{
                  fontSize: 10,
                  fontWeight: 600,
                  padding: "1px 5px",
                  borderRadius: 3,
                  background: b.level === "核心 L1" ? "#fee2e2" : "#fef3c7",
                  color: b.level === "核心 L1" ? "#b91c1c" : "#b45309"
                }}>
                  {b.level}
                </span>
                <span style={{ fontSize: 10, color: "#16a34a", fontWeight: 600 }}>健康 {b.healthScore}分</span>
              </div>
              <h4 style={{ margin: "2px 0 4px", fontSize: 13, color: "#0f172a", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                {b.name}
              </h4>
              <small style={{ color: "#64748b", display: "block", fontSize: 11, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                {b.department}
              </small>
              <div style={{ display: "flex", justifyContent: "space-between", marginTop: 6, paddingTop: 6, borderTop: "1px dashed #e2e8f0", fontSize: 10, color: "#475569" }}>
                <span>关联节点: {devCount || linkedVms.length} 台</span>
                <span style={{ color: "#2563eb" }}>{isSelected ? "● 当前查看" : "点击查看"}</span>
              </div>
            </div>
          );
        })}
      </div>

      {/* Interactive Topology Graph Section */}
      <div className="topology-view">
        <header className="topology-header">
          <div>
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <h4 style={{ margin: 0, fontSize: 15, color: "#fff" }}>
                🕸 全链路 5 层架构拓扑：{selectedBiz.name}
              </h4>
              <span className="status-pill online" style={{ fontSize: 11 }}>链路全通 · 探针正常</span>
            </div>
            <p style={{ margin: "4px 0 0", fontSize: 12, color: "#94a3b8" }}>
              业务单位: {selectedBiz.department} · 运维保障组: {selectedBiz.owner} · 系统编码: {selectedBiz.code}
            </p>
          </div>
          <div style={{ display: "flex", gap: 8 }}>
            <button 
              className="btn-secondary" 
              style={{ background: "#1e293b", color: "#f8fafc", borderColor: "#334155", fontSize: 11, padding: "4px 10px" }}
              onClick={() => {
                const h = linkedHosts[0];
                if (h) onJumpToRack?.(h.roomId, h.cabinetId);
              }}
            >
              📍 穿透至 42U 机柜立面
            </button>
          </div>
        </header>

        <div className="topology-layers">
          {/* LAYER 1: BUSINESS SYSTEM */}
          <div>
            <div className="topology-layer-title">LEVEL 1 · 业务应用层 (BUSINESS APPLICATION LAYER)</div>
            <div className="topology-nodes-row">
              <div 
                className="topology-node-card"
                onClick={() => setInspectedNode({
                  title: selectedBiz.name,
                  type: "核心业务应用",
                  detail: selectedBiz.description,
                  extra: `编码: ${selectedBiz.code} · 组织归属: ${selectedBiz.department} · 责任人: ${selectedBiz.owner}`
                })}
              >
                <div className="topology-node-icon app">🏛</div>
                <div className="topology-node-meta">
                  <strong>{selectedBiz.name}</strong>
                  <small>安全等级: {selectedBiz.level} · 生产系统</small>
                </div>
              </div>
            </div>
          </div>

          {/* LAYER 2: NETWORK & GATEWAY / LOAD BALANCER */}
          <div>
            <div className="topology-layer-title">LEVEL 2 · 网络接入与高可用负载层 (GATEWAY & SLB LAYER)</div>
            <div className="topology-nodes-row">
              {linkedSwitches.map(sw => {
                const swRoom = rooms.find(r => r.id === sw.roomId);
                const swCab = cabinets.find(c => c.id === sw.cabinetId);

                return (
                  <div 
                    key={sw.id} 
                    className="topology-node-card"
                    onClick={() => setInspectedNode({
                      title: sw.name,
                      type: "网络 / 负载均衡设备",
                      detail: `管理IP: ${sw.ip} · 端口总数: ${sw.portCount} (活跃 ${sw.activePorts}) · 角色: ${sw.role}`,
                      extra: `品牌型号: ${sw.brand} ${sw.model} · 物理位置: ${swRoom?.name} ${swCab?.name}`,
                      roomId: sw.roomId,
                      cabinetId: sw.cabinetId
                    })}
                  >
                    <div className="topology-node-icon net">🌐</div>
                    <div className="topology-node-meta">
                      <strong>{sw.name}</strong>
                      <small>{sw.ip} · {sw.role}</small>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          {/* LAYER 3: VIRTUAL MACHINE APPLICATION NODES */}
          <div>
            <div className="topology-layer-title">
              LEVEL 3 · 虚拟云主机应用集群层 (ECS INSTANCES · {linkedVms.length} 实例)
            </div>
            <div className="topology-nodes-row" style={{ flexWrap: "wrap" }}>
              {linkedVms.slice(0, 8).map(vm => (
                <div 
                  key={vm.id}
                  className="topology-node-card"
                  onClick={() => setInspectedNode({
                    title: vm.name,
                    type: "虚拟云服务器 (VM/ECS)",
                    detail: `业务IP: ${vm.privateIp || vm.ip} · 内大网IP: ${vm.internalWanIp || "-"} · OS: ${vm.os}`,
                    extra: `算力规格: ${vm.cpu} · 内存: ${vm.memory} · 磁盘: ${vm.disk} · 信创OS: ${vm.isXinchuang || "否"} · 端口: ${vm.remotePort || 22}`
                  })}
                >
                  <div className="topology-node-icon srv" style={{ background: "#0284c7" }}>☁️</div>
                  <div className="topology-node-meta">
                    <strong>{vm.name}</strong>
                    <small>{vm.privateIp || vm.ip} · {vm.cpu}</small>
                  </div>
                </div>
              ))}
              {linkedVms.length > 8 && (
                <div 
                  className="topology-node-card" 
                  style={{ background: "#1e293b", borderColor: "#334155", color: "#94a3b8", display: "flex", alignItems: "center", justifyContent: "center" }}
                >
                  <span>... 及另外 {linkedVms.length - 8} 台云主机</span>
                </div>
              )}
            </div>
          </div>

          {/* LAYER 4: DATABASE & STORAGE LAYER */}
          <div>
            <div className="topology-layer-title">LEVEL 4 · 数据持久化与数据库集群层 (DATABASE & STORAGE)</div>
            <div className="topology-nodes-row">
              {linkedDbs.length > 0 ? (
                linkedDbs.map(db => (
                  <div 
                    key={db.id} 
                    className="topology-node-card"
                    onClick={() => setInspectedNode({
                      title: db.name,
                      type: `生产数据库 (${db.type})`,
                      detail: `版本: ${db.version} · 访问主机: ${db.hostIp}:${db.port} · 部署架构: ${db.arch}`,
                      extra: `数据容量: ${db.dataSize} · 当前活跃连接数: ${db.connectionCount} · 状态: ${db.status}`
                    })}
                  >
                    <div className="topology-node-icon db">🗄</div>
                    <div className="topology-node-meta">
                      <strong>{db.name}</strong>
                      <small>{db.type} · {db.hostIp}:{db.port}</small>
                    </div>
                  </div>
                ))
              ) : (
                <div 
                  className="topology-node-card"
                  onClick={() => setInspectedNode({
                    title: "集群本地高可用存储/共享存储",
                    type: "持久化存储",
                    detail: "采用挂载分布式对象存储与本地 SSD 阵列",
                    extra: "容灾多副本备份机制"
                  })}
                >
                  <div className="topology-node-icon db">🗄</div>
                  <div className="topology-node-meta">
                    <strong>分布式高可用存储卷</strong>
                    <small>多节点分布式备份集群</small>
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* LAYER 5: PHYSICAL INFRASTRUCTURE & 42U RACK */}
          <div>
            <div className="topology-layer-title">LEVEL 5 · 实体物理基础设施宿主层 (PHYSICAL HYPERVISOR & 42U RACK)</div>
            <div className="topology-nodes-row">
              {linkedHosts.map(h => {
                const room = rooms.find(r => r.id === h.roomId);
                const cab = cabinets.find(c => c.id === h.cabinetId);

                return (
                  <div 
                    key={h.id} 
                    className="topology-node-card"
                    onClick={() => setInspectedNode({
                      title: h.hostname,
                      type: "物理服务器 / 计算宿主",
                      detail: `管理IP: ${h.ip} · 带外BMC: ${h.bmcIp} · 硬件: ${h.brand} ${h.model} (${h.cpu})`,
                      extra: `物理位置: ${room?.name} · ${cab?.name} · U位: ${h.startU}U-${h.startU + h.uHeight - 1}U`,
                      roomId: h.roomId,
                      cabinetId: h.cabinetId
                    })}
                  >
                    <div className="topology-node-icon srv">💻</div>
                    <div className="topology-node-meta">
                      <strong>{h.hostname}</strong>
                      <small>{room?.name} · {cab?.name} ({h.startU}U-{h.startU + h.uHeight - 1}U)</small>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      </div>

      {/* Node Inspector Drawer */}
      {inspectedNode && (
        <div style={{
          position: "fixed",
          right: 20,
          bottom: 20,
          width: 380,
          background: "#0f172a",
          border: "1px solid #334155",
          borderRadius: 8,
          padding: 16,
          color: "#fff",
          boxShadow: "0 10px 25px rgba(0,0,0,0.5)",
          zIndex: 100
        }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 8 }}>
            <div>
              <span style={{ fontSize: 10, background: "#2563eb", padding: "1px 6px", borderRadius: 3, color: "#fff" }}>
                {inspectedNode.type}
              </span>
              <h4 style={{ margin: "4px 0 0", fontSize: 14, color: "#38bdf8" }}>{inspectedNode.title}</h4>
            </div>
            <button 
              style={{ background: "transparent", border: 0, color: "#94a3b8", cursor: "pointer", fontSize: 16 }}
              onClick={() => setInspectedNode(null)}
            >
              ×
            </button>
          </div>
          <p style={{ margin: "8px 0", fontSize: 12, color: "#cbd5e1", lineHeight: 1.4 }}>{inspectedNode.detail}</p>
          {inspectedNode.extra && (
            <div style={{ background: "#1e293b", padding: 8, borderRadius: 4, fontSize: 11, color: "#94a3b8", marginTop: 8 }}>
              {inspectedNode.extra}
            </div>
          )}
          {inspectedNode.roomId && inspectedNode.cabinetId && (
            <div style={{ marginTop: 12, textAlign: "right" }}>
              <button 
                className="btn-primary" 
                style={{ fontSize: 11, padding: "4px 8px" }}
                onClick={() => onJumpToRack?.(inspectedNode.roomId!, inspectedNode.cabinetId!)}
              >
                跳转至该机柜 42U 立面图 →
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
