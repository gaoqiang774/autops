"use client";
import React, { useState, useMemo } from "react";
import { BusinessModel, PhysicalHost, VmHost, DatabaseAsset, SwitchDevice } from "../cmdbData";

interface ServiceModelProps {
  businesses: BusinessModel[];
  hosts: PhysicalHost[];
  vms?: VmHost[];
  databases: DatabaseAsset[];
  switches: SwitchDevice[];
  rooms?: any[];
  cabinets?: any[];
  onNavigateToProject?: (projName: string) => void;
}

export default function ServiceModel({
  businesses,
  hosts,
  vms = [],
  databases,
  switches,
  onNavigateToProject
}: ServiceModelProps) {
  const [selectedBizId, setSelectedBizId] = useState<string>(businesses[0]?.id || "biz-1");
  const [searchKeyword, setSearchKeyword] = useState("");
  const [levelFilter, setLevelFilter] = useState("全部");

  const [inspectedNode, setInspectedNode] = useState<{
    title: string;
    type: string;
    detail: string;
    extra?: string;
  } | null>(null);

  // Filter businesses by level and search
  const filteredBusinesses = useMemo(() => {
    return businesses.filter(b => {
      if (levelFilter !== "全部" && b.level !== levelFilter) return false;
      if (searchKeyword.trim()) {
        const kw = searchKeyword.toLowerCase();
        return b.name.toLowerCase().includes(kw) || b.department.toLowerCase().includes(kw) || b.code.toLowerCase().includes(kw);
      }
      return true;
    });
  }, [businesses, levelFilter, searchKeyword]);

  // Current selected business
  const currentBizIndex = useMemo(() => {
    return filteredBusinesses.findIndex(b => b.id === selectedBizId);
  }, [filteredBusinesses, selectedBizId]);

  const selectedBiz = useMemo(() => {
    return businesses.find(b => b.id === selectedBizId) || filteredBusinesses[0] || businesses[0];
  }, [businesses, selectedBizId, filteredBusinesses]);

  // Quick next / prev project
  function handlePrevProject() {
    if (filteredBusinesses.length <= 1) return;
    const newIdx = currentBizIndex <= 0 ? filteredBusinesses.length - 1 : currentBizIndex - 1;
    setSelectedBizId(filteredBusinesses[newIdx].id);
    setInspectedNode(null);
  }

  function handleNextProject() {
    if (filteredBusinesses.length <= 1) return;
    const newIdx = currentBizIndex >= filteredBusinesses.length - 1 ? 0 : currentBizIndex + 1;
    setSelectedBizId(filteredBusinesses[newIdx].id);
    setInspectedNode(null);
  }

  // Linked assets for this selected business
  const linkedVms = useMemo(() => {
    return vms.filter(v => v.businessId === selectedBiz.id || v.projectName === selectedBiz.name);
  }, [vms, selectedBiz]);

  const linkedHosts = useMemo(() => {
    const directHosts = hosts.filter(h => h.businessId === selectedBiz.id || h.projectName === selectedBiz.name);
    if (directHosts.length > 0) return directHosts;
    const parentHostId = linkedVms[0]?.physicalHostId;
    const parentHost = hosts.find(h => h.id === parentHostId);
    return parentHost ? [parentHost] : [hosts[0]];
  }, [hosts, selectedBiz, linkedVms]);

  const linkedDbs = useMemo(() => {
    return databases.filter(d => d.businessId === selectedBiz.id || d.projectName === selectedBiz.name);
  }, [databases, selectedBiz]);

  const linkedSwitches = useMemo(() => {
    const sws = switches.filter(s => s.businessId === selectedBiz.id || s.projectName === selectedBiz.name);
    return sws.length > 0 ? sws : [switches[0]];
  }, [switches, selectedBiz]);

  return (
    <div className="cmdb-container" style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      {/* ================= 1. TOP CONTROL BAR (CLEAN & NON-OVERFLOWING) ================= */}
      <div style={{
        background: "#ffffff",
        border: "1px solid #e2e8f0",
        borderRadius: 8,
        padding: "10px 16px",
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
        flexWrap: "wrap",
        gap: 12,
        boxShadow: "0 1px 3px rgba(0,0,0,0.03)"
      }}>
        {/* Left: Project Selector Dropdown & Switch Buttons */}
        <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
          <span style={{ fontWeight: 700, color: "#0f172a", fontSize: 15, display: "flex", alignItems: "center", gap: 6 }}>
            🏛 业务拓扑切换:
          </span>

          {/* Project Direct Dropdown */}
          <select 
            value={selectedBiz.id}
            onChange={e => {
              setSelectedBizId(e.target.value);
              setInspectedNode(null);
            }}
            style={{
              padding: "6px 12px",
              fontSize: 13,
              fontWeight: 600,
              color: "#1e3a8a",
              background: "#eff6ff",
              border: "1.5px solid #3b82f6",
              borderRadius: 6,
              maxWidth: 320,
              outline: "none",
              cursor: "pointer"
            }}
          >
            {filteredBusinesses.map(b => (
              <option key={b.id} value={b.id}>
                [{b.level}] {b.name} ({b.department})
              </option>
            ))}
          </select>

          {/* Prev / Next Quick Navigators */}
          <div style={{ display: "flex", gap: 4 }}>
            <button 
              className="btn-secondary" 
              style={{ padding: "5px 10px", fontSize: 12 }}
              onClick={handlePrevProject}
              title="切换到上一个项目拓扑"
            >
              ‹ 上一个
            </button>
            <button 
              className="btn-secondary" 
              style={{ padding: "5px 10px", fontSize: 12 }}
              onClick={handleNextProject}
              title="切换到下一个项目拓扑"
            >
              下一个 ›
            </button>
          </div>
        </div>

        {/* Right: Search & Level Filter */}
        <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
          <input 
            placeholder="搜索项目名称 / 客户单位..." 
            value={searchKeyword}
            onChange={e => setSearchKeyword(e.target.value)}
            style={{ width: 200, fontSize: 12, padding: "5px 8px" }}
          />

          <select 
            value={levelFilter} 
            onChange={e => setLevelFilter(e.target.value)}
            style={{ fontSize: 12, padding: "5px 8px" }}
          >
            <option value="全部">全部项目等级</option>
            <option value="核心 L1">核心 L1 项目</option>
            <option value="重要 L2">重要 L2 项目</option>
            <option value="通用 L3">通用 L3 项目</option>
          </select>

          <span style={{ fontSize: 12, color: "#64748b" }}>
            共 {filteredBusinesses.length} 个业务系统
          </span>
        </div>
      </div>

      {/* ================= 2. ACTIVE PROJECT SHOWCASE BANNER (NO OVERFLOW) ================= */}
      <div style={{
        background: "linear-gradient(135deg, #0f172a 0%, #1e293b 100%)",
        border: "1px solid #334155",
        borderRadius: 8,
        padding: "16px 20px",
        color: "#fff",
        boxShadow: "0 4px 12px rgba(0,0,0,0.12)"
      }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", flexWrap: "wrap", gap: 14 }}>
          {/* Main Info */}
          <div style={{ flex: 1, minWidth: 280 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 8, flexWrap: "wrap" }}>
              <span style={{
                background: selectedBiz.level === "核心 L1" ? "#ef4444" : "#f59e0b",
                color: "#fff",
                fontSize: 11,
                padding: "2px 8px",
                borderRadius: 4,
                fontWeight: 700
              }}>
                {selectedBiz.level}
              </span>

              <h2 style={{ margin: 0, fontSize: 18, color: "#f8fafc", fontWeight: 700 }}>
                {selectedBiz.name}
              </h2>

              <span className="status-pill online" style={{ fontSize: 11, padding: "2px 8px" }}>
                ● 全链路架构通畅 · 运行正常
              </span>

              <span style={{ fontSize: 11, color: "#38bdf8", background: "rgba(56,189,248,0.15)", padding: "2px 8px", borderRadius: 4 }}>
                健康度: {selectedBiz.healthScore} 分
              </span>
            </div>

            <p style={{ margin: "0 0 8px", fontSize: 12, color: "#cbd5e1", lineHeight: 1.5 }}>
              {selectedBiz.description}
            </p>

            <div style={{ display: "flex", gap: 16, fontSize: 11, color: "#94a3b8", flexWrap: "wrap" }}>
              <span>客户单位: <strong style={{ color: "#e2e8f0" }}>{selectedBiz.department}</strong></span>
              <span>系统编码: <code style={{ color: "#38bdf8" }}>{selectedBiz.code}</code></span>
              <span>保障团队: <strong style={{ color: "#e2e8f0" }}>{selectedBiz.owner}</strong></span>
            </div>
          </div>

          {/* Quick Metrics Badges */}
          <div style={{ display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap" }}>
            <div style={{ background: "rgba(255,255,255,0.06)", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 6, padding: "8px 14px", textAlign: "center" }}>
              <span style={{ fontSize: 10, color: "#94a3b8", display: "block" }}>云主机节点</span>
              <strong style={{ fontSize: 16, color: "#38bdf8" }}>{linkedVms.length} 台</strong>
            </div>

            <div style={{ background: "rgba(255,255,255,0.06)", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 6, padding: "8px 14px", textAlign: "center" }}>
              <span style={{ fontSize: 10, color: "#94a3b8", display: "block" }}>核心数据库</span>
              <strong style={{ fontSize: 16, color: "#f59e0b" }}>{linkedDbs.length} 个</strong>
            </div>

            <div style={{ background: "rgba(255,255,255,0.06)", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 6, padding: "8px 14px", textAlign: "center" }}>
              <span style={{ fontSize: 10, color: "#94a3b8", display: "block" }}>网络与SLB</span>
              <strong style={{ fontSize: 16, color: "#22c55e" }}>{linkedSwitches.length} 台</strong>
            </div>

            <div style={{ background: "rgba(255,255,255,0.06)", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 6, padding: "8px 14px", textAlign: "center" }}>
              <span style={{ fontSize: 10, color: "#94a3b8", display: "block" }}>物理计算宿主</span>
              <strong style={{ fontSize: 16, color: "#a855f7" }}>{linkedHosts.length} 台</strong>
            </div>
          </div>
        </div>
      </div>

      {/* ================= 3. TOPOLOGY CANVAS SECTION ================= */}
      <div className="topology-view" style={{ flex: 1, minHeight: 480 }}>
        <header className="topology-header" style={{ marginBottom: 14 }}>
          <div>
            <h4 style={{ margin: 0, fontSize: 15, color: "#38bdf8" }}>
              🕸 【{selectedBiz.name}】全链路 5 层架构依赖拓扑图
            </h4>
            <small style={{ color: "#94a3b8", display: "block", marginTop: 2 }}>
              由业务应用层自顶向下贯通至基础设施层 · 点击任意节点卡片查看详细遥测与配额
            </small>
          </div>
          <span className="status-pill online" style={{ fontSize: 11 }}>
            高可用集群架构 · 运行正常
          </span>
        </header>

        <div className="topology-layers">
          {/* LEVEL 1: BUSINESS SYSTEM */}
          <div>
            <div className="topology-layer-title">LEVEL 1 · 业务应用系统层 (BUSINESS APPLICATION LAYER)</div>
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
                  <small>安全等级: {selectedBiz.level} · 核心平台</small>
                </div>
              </div>
            </div>
          </div>

          {/* LEVEL 2: NETWORK & GATEWAY / LOAD BALANCER */}
          <div>
            <div className="topology-layer-title">LEVEL 2 · 网络接入与高可用负载层 (GATEWAY & SLB LAYER)</div>
            <div className="topology-nodes-row">
              {linkedSwitches.map(sw => (
                <div 
                  key={sw.id} 
                  className="topology-node-card"
                  onClick={() => setInspectedNode({
                    title: sw.name,
                    type: "网络 / 负载均衡设备",
                    detail: `管理IP: ${sw.ip} · 端口总数: ${sw.portCount} (活跃 ${sw.activePorts}) · 角色: ${sw.role}`,
                    extra: `品牌型号: ${sw.brand} ${sw.model} · 项目专有网络配置`
                  })}
                >
                  <div className="topology-node-icon net">🌐</div>
                  <div className="topology-node-meta">
                    <strong>{sw.name}</strong>
                    <small>{sw.ip} · {sw.role}</small>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* LEVEL 3: VIRTUAL MACHINE APPLICATION NODES */}
          <div>
            <div className="topology-layer-title">
              LEVEL 3 · 虚拟云主机应用集群层 (ECS INSTANCES · 共 {linkedVms.length} 实例)
            </div>
            <div className="topology-nodes-row" style={{ flexWrap: "wrap" }}>
              {linkedVms.slice(0, 10).map(vm => (
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
              {linkedVms.length > 10 && (
                <div 
                  className="topology-node-card" 
                  style={{ background: "#1e293b", borderColor: "#334155", color: "#94a3b8", display: "flex", alignItems: "center", justifyContent: "center" }}
                >
                  <span>... 及另外 {linkedVms.length - 10} 台云主机</span>
                </div>
              )}
            </div>
          </div>

          {/* LEVEL 4: DATABASE & STORAGE LAYER */}
          <div>
            <div className="topology-layer-title">LEVEL 4 · 数据持久化与核心数据库层 (DATABASE & STORAGE)</div>
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

          {/* LEVEL 5: PHYSICAL INFRASTRUCTURE & HYPERVISOR HOSTS */}
          <div>
            <div className="topology-layer-title">LEVEL 5 · 实体物理计算宿主与硬件层 (PHYSICAL HYPERVISOR & HARDWARE)</div>
            <div className="topology-nodes-row">
              {linkedHosts.map(h => (
                <div 
                  key={h.id} 
                  className="topology-node-card"
                  onClick={() => setInspectedNode({
                    title: h.hostname,
                    type: "物理服务器 / 计算宿主",
                    detail: `管理IP: ${h.ip} · 带外BMC: ${h.bmcIp} · 硬件: ${h.brand} ${h.model} (${h.cpu})`,
                    extra: `算力规格: ${h.cpu} · 内存: ${h.memory} · 磁盘: ${h.disk}`
                  })}
                >
                  <div className="topology-node-icon srv">💻</div>
                  <div className="topology-node-meta">
                    <strong>{h.hostname}</strong>
                    <small>{h.ip} · {h.brand} {h.model}</small>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* ================= 4. NODE INSPECTOR DRAWER ================= */}
      {inspectedNode && (
        <div style={{
          position: "fixed",
          right: 24,
          bottom: 24,
          width: 380,
          background: "#0f172a",
          border: "1px solid #334155",
          borderRadius: 8,
          padding: 16,
          color: "#fff",
          boxShadow: "0 12px 30px rgba(0,0,0,0.6)",
          zIndex: 100
        }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 8 }}>
            <div>
              <span style={{ fontSize: 10, background: "#2563eb", padding: "1px 6px", borderRadius: 3, color: "#fff", fontWeight: 600 }}>
                {inspectedNode.type}
              </span>
              <h4 style={{ margin: "4px 0 0", fontSize: 14, color: "#38bdf8" }}>{inspectedNode.title}</h4>
            </div>
            <button 
              style={{ background: "transparent", border: 0, color: "#94a3b8", cursor: "pointer", fontSize: 18, lineHeight: 1 }}
              onClick={() => setInspectedNode(null)}
            >
              ×
            </button>
          </div>
          <p style={{ margin: "8px 0", fontSize: 12, color: "#cbd5e1", lineHeight: 1.5 }}>{inspectedNode.detail}</p>
          {inspectedNode.extra && (
            <div style={{ background: "#1e293b", padding: "6px 10px", borderRadius: 4, fontSize: 11, color: "#94a3b8", marginTop: 8 }}>
              {inspectedNode.extra}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
