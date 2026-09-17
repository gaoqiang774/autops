"use client";
import React from "react";
import { IdcRoom, IdcCabinet, PhysicalHost, VmHost, SwitchDevice, DatabaseAsset, BusinessModel } from "../cmdbData";

interface CmdbDashboardProps {
  rooms: IdcRoom[];
  cabinets: IdcCabinet[];
  hosts: PhysicalHost[];
  vms?: VmHost[];
  switches: SwitchDevice[];
  databases: DatabaseAsset[];
  businesses: BusinessModel[];
  onNavigate: (menuName: string) => void;
}

export default function CmdbDashboard({
  rooms,
  cabinets,
  hosts,
  vms = [],
  switches,
  databases,
  businesses,
  onNavigate
}: CmdbDashboardProps) {
  const totalAssets = hosts.length + vms.length + switches.length;
  const totalPower = (cabinets.reduce((acc, c) => acc + c.currentPower, 0)).toFixed(1);
  const avgU = Math.round(cabinets.reduce((acc, c) => acc + (c.usedU / c.totalU) * 100, 0) / (cabinets.length || 1));

  // Compute cloud distribution
  const cloudDist: Record<string, number> = {};
  [...hosts, ...vms, ...switches].forEach(item => {
    const cloud = item.cloudVendor || "自建机房";
    cloudDist[cloud] = (cloudDist[cloud] || 0) + 1;
  });

  // Compute Xinchuang OS stats
  let xinchuangCount = 0;
  let nonXinchuangCount = 0;
  [...hosts, ...vms].forEach(item => {
    if (item.isXinchuang === "是" || (item.osFamily && (item.osFamily.includes("麒麟") || item.osFamily.includes("统信") || item.osFamily.includes("openEuler")))) {
      xinchuangCount++;
    } else {
      nonXinchuangCount++;
    }
  });
  const xinchuangPercent = Math.round((xinchuangCount / (xinchuangCount + nonXinchuangCount || 1)) * 100);

  // Compute total cores and memory
  const totalCores = [...hosts, ...vms].reduce((acc, h) => acc + (h.cpuCores || 4), 0);
  const totalMemGb = [...hosts, ...vms].reduce((acc, h) => acc + (h.memoryGb || 8), 0);
  const totalDiskTb = ([...hosts, ...vms].reduce((acc, h) => acc + (h.systemDiskGb || 30) + (h.dataDiskGb || 0), 0) / 1024).toFixed(1);

  // Top Business Systems by Host Count
  const topBiz = businesses.map(b => {
    const devCount = (b.hostIds?.length || 0) + (b.switchIds?.length || 0);
    return { ...b, devCount };
  }).sort((a, b) => b.devCount - a.devCount).slice(0, 6);

  return (
    <div className="cmdb-container">
      {/* KPI Cards Strip */}
      <section className="cmdb-kpi-strip">
        <div className="kpi-card" onClick={() => onNavigate("主机管理")} style={{ cursor: "pointer" }}>
          <div className="kpi-icon blue">💻</div>
          <div className="kpi-body">
            <span className="kpi-title">信息资产纳管总数</span>
            <span className="kpi-value">{totalAssets} <small>台设备</small></span>
          </div>
        </div>

        <div className="kpi-card" onClick={() => onNavigate("主机管理")} style={{ cursor: "pointer" }}>
          <div className="kpi-icon purple">☁️</div>
          <div className="kpi-body">
            <span className="kpi-title">虚拟云主机 / ECS</span>
            <span className="kpi-value">{vms.length} <small>台实例</small></span>
          </div>
        </div>

        <div className="kpi-card" onClick={() => onNavigate("机房管理")} style={{ cursor: "pointer" }}>
          <div className="kpi-icon green">🏢</div>
          <div className="kpi-body">
            <span className="kpi-title">全国骨干 IDC 节点</span>
            <span className="kpi-value">{rooms.length} <small>大核心机房</small></span>
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
            <span className="kpi-title">承载政企核心业务</span>
            <span className="kpi-value">{businesses.length} <small>个系统平台</small></span>
          </div>
        </div>
      </section>

      {/* Cloud & Hardware Resource Banner */}
      <div style={{ 
        display: "grid", 
        gridTemplateColumns: "1fr 1fr 1fr 1fr", 
        gap: 12, 
        marginBottom: 14 
      }}>
        <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 14 }}>
          <span style={{ fontSize: 11, color: "#64748b", display: "block" }}>算力总核数 (vCPU)</span>
          <strong style={{ fontSize: 20, color: "#0f172a" }}>{totalCores.toLocaleString()} <small style={{ fontSize: 12, fontWeight: 400 }}>Cores</small></strong>
          <small style={{ color: "#059669", display: "block", marginTop: 4 }}>覆盖 x86_64 及 ARM64 架构</small>
        </div>
        <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 14 }}>
          <span style={{ fontSize: 11, color: "#64748b", display: "block" }}>物理及虚拟内存池总量</span>
          <strong style={{ fontSize: 20, color: "#2563eb" }}>{(totalMemGb / 1024).toFixed(2)} <small style={{ fontSize: 12, fontWeight: 400 }}>TB RAM</small></strong>
          <small style={{ color: "#64748b", display: "block", marginTop: 4 }}>总配额 {totalMemGb.toLocaleString()} GB</small>
        </div>
        <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 14 }}>
          <span style={{ fontSize: 11, color: "#64748b", display: "block" }}>挂载存储空间配额</span>
          <strong style={{ fontSize: 20, color: "#7c3aed" }}>{totalDiskTb} <small style={{ fontSize: 12, fontWeight: 400 }}>TB 存储</small></strong>
          <small style={{ color: "#64748b", display: "block", marginTop: 4 }}>含 MinIO 对象存储与共享磁盘</small>
        </div>
        <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 14 }}>
          <span style={{ fontSize: 11, color: "#64748b", display: "block" }}>国产信创生态覆盖率</span>
          <strong style={{ fontSize: 20, color: "#dc2626" }}>{xinchuangPercent}% <small style={{ fontSize: 12, fontWeight: 400 }}>信创资产</small></strong>
          <small style={{ color: "#b91c1c", display: "block", marginTop: 4 }}>{xinchuangCount} 台麒麟/统信认证节点</small>
        </div>
      </div>

      {/* Cockpit Grid */}
      <div style={{ display: "grid", gridTemplateColumns: "1.25fr 1fr", gap: 14 }}>
        {/* Left: 8 IDC Datacenters Breakdown */}
        <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 18 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
            <div>
              <h4 style={{ margin: 0, fontSize: 15, color: "#0f172a" }}>🏢 8 大机房空间与资产分布 (Excel 真实台账)</h4>
              <small style={{ color: "#64748b" }}>六里桥、通州C1、首信、亦庄国企云、太极云、税务、酒仙桥、阿里云</small>
            </div>
            <button 
              className="btn-secondary" 
              style={{ fontSize: 11, padding: "3px 8px" }}
              onClick={() => onNavigate("机房管理")}
            >
              查看 42U 立面机架图 →
            </button>
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: 10, maxHeight: 420, overflowY: "auto" }}>
            {rooms.map(r => {
              const roomCabs = cabinets.filter(c => c.roomId === r.id);
              const roomPhyHosts = hosts.filter(h => h.roomId === r.id);
              const roomVms = vms.filter(v => v.roomId === r.id);
              const totalRoomDevices = roomPhyHosts.length + roomVms.length;
              const totalRoomU = roomCabs.length * 42;
              const usedRoomU = roomCabs.reduce((a, b) => a + b.usedU, 0);
              const pct = totalRoomU ? Math.round((usedRoomU / totalRoomU) * 100) : 0;

              return (
                <div key={r.id} style={{ border: "1px solid #f1f5f9", background: "#f8fafc", padding: 10, borderRadius: 6 }}>
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 4 }}>
                    <div>
                      <strong style={{ fontSize: 13, color: "#1e293b" }}>{r.name}</strong>
                      <span style={{ fontSize: 11, color: "#64748b", marginLeft: 8 }}>({r.city} · {r.operator} · {r.level})</span>
                    </div>
                    <span style={{ fontSize: 11, fontWeight: 600, color: "#2563eb", background: "#eff6ff", padding: "1px 6px", borderRadius: 4 }}>
                      {totalRoomDevices} 台资产
                    </span>
                  </div>
                  <div style={{ display: "flex", justifyContent: "space-between", fontSize: 11, color: "#475569", marginBottom: 4 }}>
                    <span>机柜: {roomCabs.length} 柜 · 实体物理机: {roomPhyHosts.length} 台 · 云主机: {roomVms.length} 台</span>
                    <span>42U U位利用: {usedRoomU}/{totalRoomU} U ({pct}%)</span>
                  </div>
                  <div className="u-progress-bar">
                    <div className="u-progress-fill" style={{ width: `${pct}%`, background: pct > 80 ? "#f59e0b" : "#2563eb" }} />
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Right: Cloud Vendors & Top Business Breakdown */}
        <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
          {/* Cloud Distribution */}
          <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 16 }}>
            <h4 style={{ margin: "0 0 12px", fontSize: 14, color: "#0f172a" }}>☁️ 云厂商与多云混合架构分布</h4>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
              {Object.entries(cloudDist).map(([cloud, count]) => {
                const pct = Math.round((count / (totalAssets || 1)) * 100);
                return (
                  <div key={cloud} style={{ background: "#f8fafc", border: "1px solid #e2e8f0", padding: "8px 12px", borderRadius: 6 }}>
                    <div style={{ display: "flex", justifyContent: "space-between", fontSize: 12 }}>
                      <strong style={{ color: "#1e293b" }}>{cloud}</strong>
                      <span style={{ fontWeight: 600, color: "#2563eb" }}>{count} 台 ({pct}%)</span>
                    </div>
                    <div style={{ background: "#e2e8f0", height: 4, borderRadius: 2, marginTop: 6, overflow: "hidden" }}>
                      <div style={{ width: `${pct}%`, height: "100%", background: "#2563eb" }} />
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          {/* Top Business Systems */}
          <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 16 }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
              <h4 style={{ margin: 0, fontSize: 14, color: "#0f172a" }}>🏛️ 核心业务系统资产规模 (Top 6)</h4>
              <button 
                className="btn-secondary" 
                style={{ fontSize: 11, padding: "2px 6px" }}
                onClick={() => onNavigate("业务模型")}
              >
                全景拓扑 →
              </button>
            </div>
            <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
              {topBiz.map((b, idx) => (
                <div key={b.id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", fontSize: 12, padding: "6px 8px", background: "#f8fafc", borderRadius: 4 }}>
                  <div>
                    <span style={{ 
                      display: "inline-block", 
                      width: 18, 
                      height: 18, 
                      lineHeight: "18px", 
                      textAlign: "center", 
                      background: idx < 3 ? "#2563eb" : "#94a3b8", 
                      color: "#fff", 
                      borderRadius: "50%", 
                      fontSize: 10,
                      marginRight: 6
                    }}>
                      {idx + 1}
                    </span>
                    <strong style={{ color: "#1e293b" }}>{b.name}</strong>
                    <small style={{ color: "#64748b", marginLeft: 6 }}>({b.department})</small>
                  </div>
                  <span style={{ fontWeight: 600, color: "#059669" }}>
                    {b.devCount} 台计算节点
                  </span>
                </div>
              ))}
            </div>
          </div>

          {/* Quick Actions */}
          <div style={{ background: "#0f172a", borderRadius: 8, padding: 14, color: "#fff" }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
              <span style={{ fontSize: 13, fontWeight: 600, color: "#38bdf8" }}>⚡ 台账运维快捷入口</span>
              <span style={{ fontSize: 11, color: "#94a3b8" }}>《信息资产台账-v331toAI.xlsx》标准</span>
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
              <button 
                className="btn-primary" 
                style={{ justifyContent: "center", padding: "8px 10px", fontSize: 12 }}
                onClick={() => onNavigate("主机管理")}
              >
                检索 238 台设备档案
              </button>
              <button 
                className="btn-secondary" 
                style={{ justifyContent: "center", padding: "8px 10px", fontSize: 12, background: "#1e293b", color: "#f8fafc", borderColor: "#334155" }}
                onClick={() => onNavigate("机房管理")}
              >
                查看 8 机房立面图
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
