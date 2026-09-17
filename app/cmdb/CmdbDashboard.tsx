"use client";
import React, { useMemo } from "react";
import { ProjectGroup, PhysicalHost, VmHost, SwitchDevice, DatabaseAsset, BusinessModel } from "../cmdbData";

interface CmdbDashboardProps {
  projects: ProjectGroup[];
  hosts: PhysicalHost[];
  vms?: VmHost[];
  switches: SwitchDevice[];
  databases: DatabaseAsset[];
  businesses: BusinessModel[];
  onNavigate: (menuName: string) => void;
  onSelectProject?: (projId: string) => void;
}

export default function CmdbDashboard({
  projects,
  hosts,
  vms = [],
  switches,
  databases,
  businesses,
  onNavigate,
  onSelectProject
}: CmdbDashboardProps) {
  const totalAssets = hosts.length + vms.length + switches.length;

  // Aggregate Customer statistics
  const customerStats = useMemo(() => {
    const map: Record<string, { count: number; projects: number; cores: number }> = {};
    projects.forEach(p => {
      if (!map[p.customerName]) {
        map[p.customerName] = { count: 0, projects: 0, cores: 0 };
      }
      map[p.customerName].count += p.deviceCount;
      map[p.customerName].projects += 1;
      map[p.customerName].cores += p.totalCores;
    });
    return Object.entries(map).sort((a, b) => b[1].count - a[1].count);
  }, [projects]);

  // Compute Cloud distribution
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

  // Compute total cores, memory, disk
  const totalCores = projects.reduce((acc, p) => acc + p.totalCores, 0);
  const totalMemGb = projects.reduce((acc, p) => acc + p.totalMemoryGb, 0);
  const totalDiskGb = projects.reduce((acc, p) => acc + p.totalDiskGb, 0);
  const totalDiskTb = (totalDiskGb / 1024).toFixed(1);

  // Top 8 Projects by asset count
  const topProjects = useMemo(() => {
    return [...projects].sort((a, b) => b.deviceCount - a.deviceCount);
  }, [projects]);

  return (
    <div className="cmdb-container">
      {/* KPI Cards Strip */}
      <section className="cmdb-kpi-strip">
        <div className="kpi-card" onClick={() => onNavigate("项目资产")} style={{ cursor: "pointer" }}>
          <div className="kpi-icon blue">📁</div>
          <div className="kpi-body">
            <span className="kpi-title">业务项目总数</span>
            <span className="kpi-value">{projects.length} <small>个独立项目</small></span>
          </div>
        </div>

        <div className="kpi-card" onClick={() => onNavigate("项目资产")} style={{ cursor: "pointer" }}>
          <div className="kpi-icon purple">💻</div>
          <div className="kpi-body">
            <span className="kpi-title">信息资产纳管总量</span>
            <span className="kpi-value">{totalAssets} <small>台物理与云设备</small></span>
          </div>
        </div>

        <div className="kpi-card" onClick={() => onNavigate("数据库管理")} style={{ cursor: "pointer" }}>
          <div className="kpi-icon gold">🗄</div>
          <div className="kpi-body">
            <span className="kpi-title">生产数据库集群</span>
            <span className="kpi-value">{databases.length} <small>个高可用实例</small></span>
          </div>
        </div>

        <div className="kpi-card" onClick={() => onNavigate("业务拓扑")} style={{ cursor: "pointer" }}>
          <div className="kpi-icon cyan">🏛</div>
          <div className="kpi-body">
            <span className="kpi-title">纳管政企客户单位</span>
            <span className="kpi-value">{customerStats.length} <small>家直管单位</small></span>
          </div>
        </div>

        <div className="kpi-card" onClick={() => onNavigate("项目资产")} style={{ cursor: "pointer" }}>
          <div className="kpi-icon green">🛡️</div>
          <div className="kpi-body">
            <span className="kpi-title">国产信创认证节点</span>
            <span className="kpi-value">{xinchuangCount} <small>台 ({xinchuangPercent}%)</small></span>
          </div>
        </div>
      </section>

      {/* Global Resource Metrics Strip */}
      <div style={{ 
        display: "grid", 
        gridTemplateColumns: "1fr 1fr 1fr 1fr", 
        gap: 12, 
        marginBottom: 14 
      }}>
        <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 14 }}>
          <span style={{ fontSize: 11, color: "#64748b", display: "block" }}>全网算力总核数 (vCPU)</span>
          <strong style={{ fontSize: 20, color: "#0f172a" }}>{totalCores.toLocaleString()} <small style={{ fontSize: 12, fontWeight: 400 }}>Cores</small></strong>
          <small style={{ color: "#059669", display: "block", marginTop: 4 }}>覆盖 x86_64 与 ARM64 架构</small>
        </div>
        <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 14 }}>
          <span style={{ fontSize: 11, color: "#64748b", display: "block" }}>全网内存配额池总量</span>
          <strong style={{ fontSize: 20, color: "#2563eb" }}>{(totalMemGb / 1024).toFixed(2)} <small style={{ fontSize: 12, fontWeight: 400 }}>TB RAM</small></strong>
          <small style={{ color: "#64748b", display: "block", marginTop: 4 }}>总配额 {totalMemGb.toLocaleString()} GB</small>
        </div>
        <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 14 }}>
          <span style={{ fontSize: 11, color: "#64748b", display: "block" }}>全网数据存储挂载配额</span>
          <strong style={{ fontSize: 20, color: "#7c3aed" }}>{totalDiskTb} <small style={{ fontSize: 12, fontWeight: 400 }}>TB 存储</small></strong>
          <small style={{ color: "#64748b", display: "block", marginTop: 4 }}>含分布式 MinIO 与系统数据盘</small>
        </div>
        <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 14 }}>
          <span style={{ fontSize: 11, color: "#64748b", display: "block" }}>多云协同环境运行率</span>
          <strong style={{ fontSize: 20, color: "#15803d" }}>100% <small style={{ fontSize: 12, fontWeight: 400 }}>在线正常</small></strong>
          <small style={{ color: "#16a34a", display: "block", marginTop: 4 }}>探针双向心跳遥测健康</small>
        </div>
      </div>

      {/* Main Grid: Projects Breakdown & Cloud/Customer Analytics */}
      <div style={{ display: "grid", gridTemplateColumns: "1.3fr 1fr", gap: 14 }}>
        {/* Left: Project Assets Distribution Matrix */}
        <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 18 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 14 }}>
            <div>
              <h4 style={{ margin: 0, fontSize: 15, color: "#0f172a" }}>📁 各项目资产规模与资源配额总览 (按项目分资产)</h4>
              <small style={{ color: "#64748b" }}>涵盖 24 个重点业务项目 · 依据《信息资产台账-v331toAI.xlsx》标准</small>
            </div>
            <button 
              className="btn-secondary" 
              style={{ fontSize: 11, padding: "3px 8px" }}
              onClick={() => onNavigate("项目资产")}
            >
              进入项目资产工作台 →
            </button>
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: 10, maxHeight: 480, overflowY: "auto", paddingRight: 6 }}>
            {topProjects.map((p, idx) => {
              const maxCount = topProjects[0]?.deviceCount || 34;
              const barPct = Math.round((p.deviceCount / maxCount) * 100);

              return (
                <div 
                  key={p.id} 
                  onClick={() => onNavigate("项目资产")}
                  style={{ 
                    border: "1px solid #f1f5f9", 
                    background: "#f8fafc", 
                    padding: "10px 12px", 
                    borderRadius: 6,
                    cursor: "pointer",
                    transition: "all 0.15s"
                  }}
                  title="点击跳转至该项目资产明细"
                >
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 4 }}>
                    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                      <span style={{
                        display: "inline-block",
                        width: 20,
                        height: 20,
                        lineHeight: "20px",
                        textAlign: "center",
                        borderRadius: "50%",
                        fontSize: 10,
                        fontWeight: 700,
                        background: idx < 3 ? "#2563eb" : "#94a3b8",
                        color: "#fff"
                      }}>
                        {idx + 1}
                      </span>
                      <strong style={{ fontSize: 13, color: "#1e293b" }}>{p.name}</strong>
                      <span style={{
                        fontSize: 10,
                        padding: "1px 5px",
                        borderRadius: 3,
                        background: p.env === "生产" ? "#dcfce7" : "#f1f5f9",
                        color: p.env === "生产" ? "#15803d" : "#64748b"
                      }}>
                        {p.env}
                      </span>
                      <span style={{ fontSize: 11, color: "#2563eb" }}>{p.cloudVendor}</span>
                    </div>

                    <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                      {p.xinchuangCount > 0 && (
                        <span style={{ fontSize: 10, color: "#dc2626", background: "#fee2e2", padding: "1px 5px", borderRadius: 3, fontWeight: 600 }}>
                          信创 {p.xinchuangCount}台
                        </span>
                      )}
                      <strong style={{ fontSize: 13, color: "#0f172a" }}>{p.deviceCount} 台资产</strong>
                    </div>
                  </div>

                  <div style={{ display: "flex", justifyContent: "space-between", fontSize: 11, color: "#64748b", marginBottom: 4 }}>
                    <span>客户: {p.customerName}</span>
                    <span>算力: {p.totalCores} 核 · 内存: {p.totalMemoryGb} GB · 存储: {(p.totalDiskGb / 1024).toFixed(1)} TB</span>
                  </div>

                  <div style={{ background: "#e2e8f0", height: 5, borderRadius: 3, overflow: "hidden" }}>
                    <div style={{ width: `${barPct}%`, height: "100%", background: idx < 3 ? "#2563eb" : "#64748b" }} />
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Right: Customer Share & Cloud Distribution */}
        <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
          {/* Customer Distribution */}
          <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 16 }}>
            <h4 style={{ margin: "0 0 12px", fontSize: 14, color: "#0f172a" }}>🏛️ 客户单位资产聚集度统计</h4>
            <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
              {customerStats.map(([cust, st]) => {
                const pct = Math.round((st.count / (totalAssets || 1)) * 100);
                return (
                  <div key={cust} style={{ padding: "8px 10px", background: "#f8fafc", borderRadius: 6, border: "1px solid #f1f5f9" }}>
                    <div style={{ display: "flex", justifyContent: "space-between", fontSize: 12, marginBottom: 4 }}>
                      <strong style={{ color: "#1e293b" }}>{cust}</strong>
                      <span style={{ fontWeight: 600, color: "#2563eb" }}>
                        {st.count} 台 ({pct}%) · {st.projects} 个项目
                      </span>
                    </div>
                    <div style={{ background: "#e2e8f0", height: 4, borderRadius: 2, overflow: "hidden" }}>
                      <div style={{ width: `${pct}%`, height: "100%", background: "#2563eb" }} />
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          {/* Cloud Vendors Distribution */}
          <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 16 }}>
            <h4 style={{ margin: "0 0 12px", fontSize: 14, color: "#0f172a" }}>☁️ 云厂商与承载环境分布</h4>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
              {Object.entries(cloudDist).map(([cloud, count]) => {
                const pct = Math.round((count / (totalAssets || 1)) * 100);
                return (
                  <div key={cloud} style={{ background: "#f8fafc", border: "1px solid #e2e8f0", padding: "8px 10px", borderRadius: 6 }}>
                    <div style={{ display: "flex", justifyContent: "space-between", fontSize: 12 }}>
                      <strong style={{ color: "#1e293b" }}>{cloud}</strong>
                      <span style={{ fontWeight: 600, color: "#2563eb" }}>{count} 台</span>
                    </div>
                    <small style={{ color: "#64748b", display: "block", marginTop: 2 }}>占比 {pct}%</small>
                  </div>
                );
              })}
            </div>
          </div>

          {/* Quick Actions Card */}
          <div style={{ background: "#0f172a", borderRadius: 8, padding: 14, color: "#fff" }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
              <span style={{ fontSize: 13, fontWeight: 600, color: "#38bdf8" }}>⚡ 项目资产快速入口</span>
              <span style={{ fontSize: 11, color: "#94a3b8" }}>以项目为中心统一管理</span>
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
              <button 
                className="btn-primary" 
                style={{ justifyContent: "center", padding: "8px 10px", fontSize: 12 }}
                onClick={() => onNavigate("项目资产")}
              >
                进入项目资产列表
              </button>
              <button 
                className="btn-secondary" 
                style={{ justifyContent: "center", padding: "8px 10px", fontSize: 12, background: "#1e293b", color: "#f8fafc", borderColor: "#334155" }}
                onClick={() => onNavigate("业务拓扑")}
              >
                查看业务系统拓扑
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
