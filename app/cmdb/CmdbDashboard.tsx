"use client";
import React, { useMemo } from "react";
import { ProjectGroup, PhysicalHost, VmHost, SwitchDevice, DatabaseAsset, BusinessModel } from "../cmdbData";

interface CmdbDashboardProps {
  projects: ProjectGroup[];
  hosts: PhysicalHost[];
  vms?: VmHost[];
  switches: SwitchDevice[];
  databases?: DatabaseAsset[];
  businesses: BusinessModel[];
  onNavigate: (menuName: string) => void;
  onSelectProject?: (projId: string) => void;
}

export default function CmdbDashboard({
  projects,
  hosts,
  vms = [],
  switches,
  databases = [],
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
  let armCount = 0;
  let x86Count = 0;

  [...hosts, ...vms].forEach(item => {
    if (item.isXinchuang === "是" || (item.osFamily && (item.osFamily.includes("麒麟") || item.osFamily.includes("统信") || item.osFamily.includes("openEuler")))) {
      xinchuangCount++;
    } else {
      nonXinchuangCount++;
    }

    const arch = (item.cpuArch || (item as any).cpu || "").toLowerCase();
    if (arch.includes("arm") || arch.includes("aarch64")) {
      armCount++;
    } else {
      x86Count++;
    }
  });

  const xinchuangPercent = Math.round((xinchuangCount / (xinchuangCount + nonXinchuangCount || 1)) * 100);

  // Compute total cores, memory, disk
  const totalCores = projects.reduce((acc, p) => acc + p.totalCores, 0);
  const totalMemGb = projects.reduce((acc, p) => acc + p.totalMemoryGb, 0);
  const totalDiskGb = projects.reduce((acc, p) => acc + p.totalDiskGb, 0);
  const totalDiskTb = (totalDiskGb / 1024).toFixed(1);

  // Top Projects sorted by asset count
  const sortedProjects = useMemo(() => {
    return [...projects].sort((a, b) => b.deviceCount - a.deviceCount);
  }, [projects]);

  return (
    <div className="cmdb-container" style={{ display: "flex", flexDirection: "column", gap: 14 }}>
      {/* ── 顶部大盘标头与快捷入口 ── */}
      <div style={{
        background: "#ffffff",
        border: "1px solid #e2e8f0",
        borderRadius: 8,
        padding: "16px 20px",
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
        flexWrap: "wrap",
        gap: 12,
        boxShadow: "0 1px 3px rgba(0,0,0,0.04)"
      }}>
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span style={{ fontSize: 20 }}>📊</span>
            <h3 style={{ margin: 0, fontSize: 17, color: "#0f172a", fontWeight: 700 }}>
              信息资产综合监控大盘
            </h3>
            <span style={{ background: "#eff6ff", color: "#2563eb", fontSize: 11, padding: "2px 8px", borderRadius: 4, fontWeight: 600 }}>
              全网全景 · 多维统计
            </span>
          </div>
          <p style={{ margin: "4px 0 0", fontSize: 12, color: "#64748b" }}>
            全网纳管 {totalAssets} 台设备 · 汇聚 {projects.length} 个业务项目 · 算力池 {totalCores.toLocaleString()} Cores · 存储 {totalDiskTb} TB
          </p>
        </div>

        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          <button 
            className="btn-primary" 
            style={{ fontSize: 12, padding: "6px 14px", display: "inline-flex", alignItems: "center", gap: 6 }}
            onClick={() => onNavigate("项目资产")}
          >
            <span>📁</span>
            <span>项目资产台账</span>
          </button>
          <button 
            className="btn-secondary" 
            style={{ fontSize: 12, padding: "6px 12px", display: "inline-flex", alignItems: "center", gap: 6 }}
            onClick={() => onNavigate("业务拓扑")}
          >
            <span>🕸️</span>
            <span>业务拓扑</span>
          </button>
          <button 
            className="btn-secondary" 
            style={{ fontSize: 12, padding: "6px 12px", display: "inline-flex", alignItems: "center", gap: 6 }}
            onClick={() => onNavigate("漏洞检测")}
          >
            <span>🛡️</span>
            <span>漏洞检测</span>
          </button>
        </div>
      </div>

      {/* ── 核心 KPI 4 栏卡片 ── */}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(240px, 1fr))", gap: 12 }}>
        {/* Card 1: 纳管资产总量 */}
        <div 
          onClick={() => onNavigate("项目资产")}
          style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: "16px 18px", cursor: "pointer", transition: "all 0.15s", boxShadow: "0 1px 3px rgba(0,0,0,0.03)" }}
          title="点击进入项目资产工作台"
        >
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 8 }}>
            <span style={{ fontSize: 12, color: "#64748b", fontWeight: 600 }}>信息资产纳管总量</span>
            <span style={{ width: 34, height: 34, borderRadius: 8, background: "#eff6ff", color: "#2563eb", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 16 }}>💻</span>
          </div>
          <div style={{ display: "flex", alignItems: "baseline", gap: 6 }}>
            <strong style={{ fontSize: 26, color: "#0f172a", fontWeight: 800 }}>{totalAssets}</strong>
            <small style={{ fontSize: 12, color: "#64748b" }}>台资产</small>
          </div>
          <div style={{ fontSize: 11, color: "#64748b", marginTop: 6, display: "flex", gap: 6 }}>
            <span>虚拟机 {vms.length}</span> · <span>物理机 {hosts.length}</span> · <span>网络SLB {switches.length}</span>
          </div>
        </div>

        {/* Card 2: 覆盖业务项目 */}
        <div 
          onClick={() => onNavigate("项目资产")}
          style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: "16px 18px", cursor: "pointer", transition: "all 0.15s", boxShadow: "0 1px 3px rgba(0,0,0,0.03)" }}
          title="点击进入项目资产工作台"
        >
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 8 }}>
            <span style={{ fontSize: 12, color: "#64748b", fontWeight: 600 }}>覆盖业务项目</span>
            <span style={{ width: 34, height: 34, borderRadius: 8, background: "#f5f3ff", color: "#7c3aed", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 16 }}>📁</span>
          </div>
          <div style={{ display: "flex", alignItems: "baseline", gap: 6 }}>
            <strong style={{ fontSize: 26, color: "#7c3aed", fontWeight: 800 }}>{projects.length}</strong>
            <small style={{ fontSize: 12, color: "#64748b" }}>个独立项目</small>
          </div>
          <div style={{ fontSize: 11, color: "#64748b", marginTop: 6 }}>
            服务 <strong>{customerStats.length}</strong> 家直管政企客户单位
          </div>
        </div>

        {/* Card 3: 算力总核数与存储 */}
        <div 
          style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: "16px 18px", boxShadow: "0 1px 3px rgba(0,0,0,0.03)" }}
        >
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 8 }}>
            <span style={{ fontSize: 12, color: "#64748b", fontWeight: 600 }}>全网算力与存储挂载</span>
            <span style={{ width: 34, height: 34, borderRadius: 8, background: "#f0fdf4", color: "#16a34a", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 16 }}>⚡</span>
          </div>
          <div style={{ display: "flex", alignItems: "baseline", gap: 6 }}>
            <strong style={{ fontSize: 26, color: "#16a34a", fontWeight: 800 }}>{totalCores.toLocaleString()}</strong>
            <small style={{ fontSize: 12, color: "#64748b" }}>Cores</small>
          </div>
          <div style={{ fontSize: 11, color: "#64748b", marginTop: 6 }}>
            内存 {(totalMemGb / 1024).toFixed(1)} TB · 存储卷 {totalDiskTb} TB
          </div>
        </div>

        {/* Card 4: 国产信创设备 */}
        <div 
          onClick={() => onNavigate("漏洞检测")}
          style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: "16px 18px", cursor: "pointer", transition: "all 0.15s", boxShadow: "0 1px 3px rgba(0,0,0,0.03)" }}
          title="点击进入漏洞检测排查信创设备"
        >
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 8 }}>
            <span style={{ fontSize: 12, color: "#64748b", fontWeight: 600 }}>国产信创认证底座</span>
            <span style={{ width: 34, height: 34, borderRadius: 8, background: "#fff7ed", color: "#ea580c", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 16 }}>🛡️</span>
          </div>
          <div style={{ display: "flex", alignItems: "baseline", gap: 6 }}>
            <strong style={{ fontSize: 26, color: "#ea580c", fontWeight: 800 }}>{xinchuangCount}</strong>
            <small style={{ fontSize: 12, color: "#64748b" }}>台 ({xinchuangPercent}%)</small>
          </div>
          <div style={{ fontSize: 11, color: "#64748b", marginTop: 6 }}>
            麒麟 / 统信 / openEuler 信创认证
          </div>
        </div>
      </div>

      {/* ── 核心双列布局 (左: 项目资产规模总览; 右: 客户聚集度与多云环境分布) ── */}
      <div style={{ display: "grid", gridTemplateColumns: "1.25fr 1fr", gap: 14, alignItems: "start" }}>
        {/* 左列: 各项目资产规模与资源配额总览 */}
        <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 18, boxShadow: "0 1px 3px rgba(0,0,0,0.03)" }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 14 }}>
            <div>
              <h4 style={{ margin: 0, fontSize: 15, color: "#0f172a", fontWeight: 700 }}>
                📁 各项目资产规模与资源配额总览
              </h4>
              <small style={{ color: "#64748b", fontSize: 12 }}>
                覆盖全网 {projects.length} 个重点业务系统 · 点击可直接进入项目资产详情
              </small>
            </div>
            <button 
              className="btn-secondary" 
              style={{ fontSize: 11, padding: "4px 10px" }}
              onClick={() => onNavigate("项目资产")}
            >
              进入工作台 →
            </button>
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            {sortedProjects.map((p, idx) => {
              const maxCount = sortedProjects[0]?.deviceCount || 34;
              const barPct = Math.round((p.deviceCount / maxCount) * 100);

              return (
                <div 
                  key={p.id} 
                  onClick={() => onNavigate("项目资产")}
                  style={{ 
                    border: "1px solid #f1f5f9", 
                    background: "#f8fafc", 
                    padding: "10px 14px", 
                    borderRadius: 6,
                    cursor: "pointer",
                    transition: "all 0.15s"
                  }}
                  title={`点击查看【${p.name}】资产台账明细`}
                >
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 5 }}>
                    <div style={{ display: "flex", alignItems: "center", gap: 8, minWidth: 0, flex: 1 }}>
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
                        color: "#fff",
                        flexShrink: 0
                      }}>
                        {idx + 1}
                      </span>
                      <strong style={{ fontSize: 13, color: "#1e293b", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                        {p.name}
                      </strong>
                      <span style={{
                        fontSize: 10,
                        padding: "1px 5px",
                        borderRadius: 3,
                        background: p.env === "生产" ? "#dcfce7" : "#f1f5f9",
                        color: p.env === "生产" ? "#15803d" : "#64748b",
                        flexShrink: 0
                      }}>
                        {p.env}
                      </span>
                      <span style={{ fontSize: 11, color: "#2563eb", flexShrink: 0 }}>{p.cloudVendor}</span>
                    </div>

                    <div style={{ display: "flex", alignItems: "center", gap: 8, flexShrink: 0, marginLeft: 8 }}>
                      {p.xinchuangCount > 0 && (
                        <span style={{ fontSize: 10, color: "#dc2626", background: "#fee2e2", padding: "1px 6px", borderRadius: 3, fontWeight: 600 }}>
                          信创 {p.xinchuangCount}台
                        </span>
                      )}
                      <strong style={{ fontSize: 13, color: "#0f172a" }}>{p.deviceCount} 台资产</strong>
                    </div>
                  </div>

                  <div style={{ display: "flex", justifyContent: "space-between", fontSize: 11, color: "#64748b", marginBottom: 5 }}>
                    <span style={{ whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>客户: {p.customerName}</span>
                    <span style={{ flexShrink: 0, marginLeft: 8 }}>算力: {p.totalCores} 核 · 内存: {p.totalMemoryGb} GB · 存储: {(p.totalDiskGb / 1024).toFixed(1)} TB</span>
                  </div>

                  <div style={{ background: "#e2e8f0", height: 5, borderRadius: 3, overflow: "hidden" }}>
                    <div style={{ width: `${barPct}%`, height: "100%", background: idx < 3 ? "linear-gradient(90deg, #3b82f6, #2563eb)" : "#94a3b8" }} />
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* 右列: 客户聚集度 + 多云分布 + 架构生态 */}
        <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
          {/* 1. 客户单位资产聚集度统计 */}
          <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 16, boxShadow: "0 1px 3px rgba(0,0,0,0.03)" }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
              <h4 style={{ margin: 0, fontSize: 14, color: "#0f172a", fontWeight: 700 }}>
                🏛️ 客户单位资产聚集度统计
              </h4>
              <span style={{ fontSize: 11, color: "#64748b" }}>共 {customerStats.length} 家直管单位</span>
            </div>
            <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
              {customerStats.map(([cust, st]) => {
                const pct = Math.round((st.count / (totalAssets || 1)) * 100);
                return (
                  <div key={cust} style={{ padding: "8px 12px", background: "#f8fafc", borderRadius: 6, border: "1px solid #f1f5f9" }}>
                    <div style={{ display: "flex", justifyContent: "space-between", fontSize: 12, marginBottom: 4 }}>
                      <strong style={{ color: "#1e293b" }}>{cust}</strong>
                      <span style={{ fontWeight: 600, color: "#2563eb" }}>
                        {st.count} 台 ({pct}%) · {st.projects} 个项目
                      </span>
                    </div>
                    <div style={{ background: "#e2e8f0", height: 4, borderRadius: 2, overflow: "hidden" }}>
                      <div style={{ width: `${pct}%`, height: "100%", background: "linear-gradient(90deg, #38bdf8, #2563eb)" }} />
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          {/* 2. 云厂商与承载环境分布 */}
          <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 16, boxShadow: "0 1px 3px rgba(0,0,0,0.03)" }}>
            <h4 style={{ margin: "0 0 12px", fontSize: 14, color: "#0f172a", fontWeight: 700 }}>
              ☁️ 云厂商与基础设施承载分布
            </h4>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
              {Object.entries(cloudDist).map(([cloud, count]) => {
                const pct = Math.round((count / (totalAssets || 1)) * 100);
                return (
                  <div key={cloud} style={{ background: "#f8fafc", border: "1px solid #e2e8f0", padding: "10px 12px", borderRadius: 6 }}>
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                      <strong style={{ color: "#1e293b", fontSize: 12 }}>{cloud}</strong>
                      <span style={{ fontWeight: 700, color: "#2563eb", fontSize: 13 }}>{count} 台</span>
                    </div>
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginTop: 4 }}>
                      <small style={{ color: "#64748b", fontSize: 11 }}>全网占比</small>
                      <span style={{ fontSize: 11, fontWeight: 600, color: "#475569" }}>{pct}%</span>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          {/* 3. 芯片架构与生态分布 */}
          <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 16, boxShadow: "0 1px 3px rgba(0,0,0,0.03)" }}>
            <h4 style={{ margin: "0 0 12px", fontSize: 14, color: "#0f172a", fontWeight: 700 }}>
              🛡️ 计算架构与信创生态概览
            </h4>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
              <div style={{ background: "#f0fdf4", border: "1px solid #bbf7d0", padding: "10px 12px", borderRadius: 6 }}>
                <span style={{ fontSize: 11, color: "#166534", display: "block", marginBottom: 2 }}>国产信创生态占比</span>
                <strong style={{ fontSize: 17, color: "#15803d" }}>{xinchuangPercent}% ({xinchuangCount}台)</strong>
                <small style={{ color: "#16a34a", display: "block", marginTop: 2, fontSize: 10 }}>麒麟/统信/openEuler</small>
              </div>
              <div style={{ background: "#eff6ff", border: "1px solid #bfdbfe", padding: "10px 12px", borderRadius: 6 }}>
                <span style={{ fontSize: 11, color: "#1e40af", display: "block", marginBottom: 2 }}>算力芯片架构</span>
                <strong style={{ fontSize: 17, color: "#2563eb" }}>x86: {x86Count} / ARM: {armCount}</strong>
                <small style={{ color: "#3b82f6", display: "block", marginTop: 2, fontSize: 10 }}>兼容双架构算力资源池</small>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
