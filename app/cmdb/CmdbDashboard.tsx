"use client";
import React from "react";
import { IdcRoom, IdcCabinet, PhysicalHost, SwitchDevice, DatabaseAsset, BusinessModel } from "../cmdbData";

interface CmdbDashboardProps {
  rooms: IdcRoom[];
  cabinets: IdcCabinet[];
  hosts: PhysicalHost[];
  switches: SwitchDevice[];
  databases: DatabaseAsset[];
  businesses: BusinessModel[];
  onNavigate: (menuName: string) => void;
}

export default function CmdbDashboard({
  rooms,
  cabinets,
  hosts,
  switches,
  databases,
  businesses,
  onNavigate
}: CmdbDashboardProps) {
  const totalAssets = hosts.length + switches.length;
  const totalPower = (cabinets.reduce((acc, c) => acc + c.currentPower, 0)).toFixed(1);
  const avgU = Math.round(cabinets.reduce((acc, c) => acc + (c.usedU / c.totalU) * 100, 0) / (cabinets.length || 1));

  return (
    <div className="cmdb-container">
      {/* KPI Cards */}
      <section className="cmdb-kpi-strip">
        <div className="kpi-card" onClick={() => onNavigate("机房管理")} style={{ cursor: "pointer" }}>
          <div className="kpi-icon blue">🏢</div>
          <div className="kpi-body">
            <span className="kpi-title">IDC 核心机房</span>
            <span className="kpi-value">{rooms.length} <small>个全国节点</small></span>
          </div>
        </div>
        <div className="kpi-card" onClick={() => onNavigate("机房管理")} style={{ cursor: "pointer" }}>
          <div className="kpi-icon green">▥</div>
          <div className="kpi-body">
            <span className="kpi-title">标准在管机柜</span>
            <span className="kpi-value">{cabinets.length} <small>个 42U 机柜</small></span>
          </div>
        </div>
        <div className="kpi-card" onClick={() => onNavigate("主机管理")} style={{ cursor: "pointer" }}>
          <div className="kpi-icon purple">💻</div>
          <div className="kpi-body">
            <span className="kpi-title">在架计算与网络资产</span>
            <span className="kpi-value">{totalAssets} <small>台物理设备</small></span>
          </div>
        </div>
        <div className="kpi-card" onClick={() => onNavigate("数据库管理")} style={{ cursor: "pointer" }}>
          <div className="kpi-icon gold">🗄</div>
          <div className="kpi-body">
            <span className="kpi-title">生产数据库集群</span>
            <span className="kpi-value">{databases.length} <small>个高可用实例</small></span>
          </div>
        </div>
        <div className="kpi-card" onClick={() => onNavigate("业务模型")} style={{ cursor: "pointer" }}>
          <div className="kpi-icon cyan">🏛</div>
          <div className="kpi-body">
            <span className="kpi-title">纳管关键业务系统</span>
            <span className="kpi-value">{businesses.length} <small>个业务架构</small></span>
          </div>
        </div>
      </section>

      {/* Cockpit Grid */}
      <div style={{ display: "grid", gridTemplateColumns: "1.2fr 1fr", gap: 14 }}>
        {/* Left: Datacenter Resource Overview */}
        <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 18 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
            <h4 style={{ margin: 0, fontSize: 15, color: "#0f172a" }}>🏢 机房机架负载与空间利用概览</h4>
            <button 
              className="btn-secondary" 
              style={{ fontSize: 11, padding: "3px 8px" }}
              onClick={() => onNavigate("机房管理")}
            >
              查看 42U 立面机架图 →
            </button>
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
            {rooms.map(r => {
              const roomCabs = cabinets.filter(c => c.roomId === r.id);
              const roomHosts = hosts.filter(h => h.roomId === r.id);
              const totalRoomU = roomCabs.length * 42;
              const usedRoomU = roomCabs.reduce((a, b) => a + b.usedU, 0);
              const pct = totalRoomU ? Math.round((usedRoomU / totalRoomU) * 100) : 0;

              return (
                <div key={r.id} style={{ border: "1px solid #f1f5f9", background: "#f8fafc", padding: 12, borderRadius: 6 }}>
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 6 }}>
                    <div>
                      <strong style={{ fontSize: 13, color: "#1e293b" }}>{r.name}</strong>
                      <span style={{ fontSize: 11, color: "#64748b", marginLeft: 8 }}>({r.city} · {r.operator} · {r.level})</span>
                    </div>
                    <span className="status-pill online">● 正常</span>
                  </div>
                  <div style={{ display: "flex", justifyContent: "space-between", fontSize: 11, color: "#475569", marginBottom: 4 }}>
                    <span>机柜数量: {roomCabs.length} 柜 · 物理服务器: {roomHosts.length} 台</span>
                    <span>U位占用: {usedRoomU}/{totalRoomU} U ({pct}%)</span>
                  </div>
                  <div className="u-progress-bar">
                    <div className="u-progress-fill" style={{ width: `${pct}%` }} />
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Right: Asset Breakdown & Hardware Health */}
        <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
          {/* Hardware Brand Share */}
          <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 18 }}>
            <h4 style={{ margin: "0 0 14px", fontSize: 15, color: "#0f172a" }}>💻 物理算力品牌与机型分布</h4>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 10 }}>
              <div style={{ background: "#eff6ff", border: "1px solid #bfdbfe", padding: 10, borderRadius: 6 }}>
                <span style={{ fontSize: 11, color: "#1d4ed8", display: "block" }}>Dell PowerEdge 系列</span>
                <strong style={{ fontSize: 18, color: "#1e3a8a" }}>{hosts.filter(h => h.brand === "Dell").length} 台</strong>
                <small style={{ display: "block", fontSize: 10, color: "#64748b" }}>R740 / R750 双路机架</small>
              </div>
              <div style={{ background: "#fef2f2", border: "1px solid #fecaca", padding: 10, borderRadius: 6 }}>
                <span style={{ fontSize: 11, color: "#b91c1c", display: "block" }}>华为 FusionServer / 鲲鹏</span>
                <strong style={{ fontSize: 18, color: "#991b1b" }}>{hosts.filter(h => h.brand === "华为").length} 台</strong>
                <small style={{ display: "block", fontSize: 10, color: "#64748b" }}>2288H / TaiShan ARM</small>
              </div>
              <div style={{ background: "#fffbeb", border: "1px solid #fde68a", padding: 10, borderRadius: 6 }}>
                <span style={{ fontSize: 11, color: "#b45309", display: "block" }}>浪潮 Inspur GPU 算力机</span>
                <strong style={{ fontSize: 18, color: "#78350f" }}>{hosts.filter(h => h.brand === "浪潮").length} 台</strong>
                <small style={{ display: "block", fontSize: 10, color: "#64748b" }}>NF5468M5 4U 智算机</small>
              </div>
              <div style={{ background: "#f0fdf4", border: "1px solid #bbf7d0", padding: 10, borderRadius: 6 }}>
                <span style={{ fontSize: 11, color: "#15803d", display: "block" }}>网络交换机与网关设备</span>
                <strong style={{ fontSize: 18, color: "#14532d" }}>{switches.length} 台</strong>
                <small style={{ display: "block", fontSize: 10, color: "#64748b" }}>锐捷 100G / 华为万兆</small>
              </div>
            </div>
          </div>

          {/* Quick Actions Card */}
          <div style={{ background: "#0f172a", borderRadius: 8, padding: 16, color: "#fff" }}>
            <h5 style={{ margin: "0 0 10px", fontSize: 14, color: "#38bdf8" }}>⚡ 资产管理快捷工作流</h5>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
              <button 
                className="btn-primary" 
                style={{ justifyContent: "center", padding: 10 }}
                onClick={() => onNavigate("机房管理")}
              >
                进入 42U 机柜立面
              </button>
              <button 
                className="btn-secondary" 
                style={{ justifyContent: "center", padding: 10, background: "#1e293b", color: "#f8fafc", borderColor: "#334155" }}
                onClick={() => onNavigate("业务模型")}
              >
                查看业务全景拓扑
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
