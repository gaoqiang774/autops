"use client";
import React, { useState } from "react";
import { AgentProbe, PhysicalHost } from "../cmdbData";

interface ProbeManagementProps {
  probes: AgentProbe[];
  hosts: PhysicalHost[];
}

export default function ProbeManagement({ probes, hosts }: ProbeManagementProps) {
  const [copied, setCopied] = useState(false);

  const installCmd = "curl -fsSL https://autops.gaoqiang3529.workers.dev/agent/install.sh | bash -s -- --server=https://autops.gaoqiang3529.workers.dev --token=bk_agent_sec_99482";

  return (
    <div className="cmdb-container">
      {/* Header & Quick Install */}
      <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: 16 }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 10 }}>
          <div>
            <h3 style={{ margin: 0, fontSize: 16, color: "#0f172a" }}>📡 AutoOps 物理机/云主机智能探针管理</h3>
            <p style={{ margin: "4px 0 0", fontSize: 12, color: "#64748b" }}>
              轻量级 Golang 守护进程，负责实时采集主机硬件指标、CPU/内存/磁盘数据、IPMI SEL 日志及进程拓扑
            </p>
          </div>
          <span className="room-badge" style={{ background: "#dcfce7", color: "#15803d" }}>
            全网 {probes.length} 个探针全部健康在线
          </span>
        </div>

        <div style={{ background: "#0f172a", borderRadius: 6, padding: "10px 14px", display: "flex", justifyContent: "space-between", alignItems: "center", gap: 12, flexWrap: "wrap" }}>
          <code style={{ color: "#38bdf8", fontSize: 12, fontFamily: "monospace", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", flex: 1, minWidth: 260 }}>{installCmd}</code>
          <button 
            className="btn-primary" 
            style={{ padding: "4px 12px", fontSize: 11, flexShrink: 0 }}
            onClick={() => {
              navigator.clipboard?.writeText?.(installCmd);
              setCopied(true);
              setTimeout(() => setCopied(false), 2000);
            }}
          >
            {copied ? "已复制！" : "一键复制安装脚本"}
          </button>
        </div>
      </div>

      {/* Probes Table */}
      <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, overflowX: "auto", overflowY: "hidden" }}>
        <table className="cmdb-data-table" style={{ minWidth: 880 }}>
          <thead>
            <tr>
              <th>主机名称</th>
              <th>采集管理 IP</th>
              <th>探针版本</th>
              <th>实时 CPU 占用</th>
              <th>实时内存占用</th>
              <th>最后心跳上报</th>
              <th>探针运行状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {probes.map(p => (
              <tr key={p.id}>
                <td style={{ fontWeight: 600, color: "#1e293b" }}>{p.hostname}</td>
                <td style={{ fontFamily: "monospace", color: "#2563eb" }}>{p.hostIp}</td>
                <td><span className="room-badge">{p.agentVersion}</span></td>
                <td>
                  <span style={{ fontWeight: 600, color: p.cpuUsage > 70 ? "#ea580c" : "#16a34a" }}>
                    {p.cpuUsage}%
                  </span>
                </td>
                <td>
                  <span style={{ fontWeight: 600, color: p.memUsage > 80 ? "#dc2626" : "#2563eb" }}>
                    {p.memUsage}%
                  </span>
                </td>
                <td>{p.lastHeartbeat}</td>
                <td><span className="status-pill online">● 活跃在线</span></td>
                <td>
                  <button 
                    className="btn-secondary" 
                    style={{ padding: "3px 8px", fontSize: 11, marginRight: 6 }}
                    onClick={() => alert(`已向主机 [${p.hostname}] 发起探针重启与重连测试！`)}
                  >
                    重载探针
                  </button>
                  <button 
                    className="btn-secondary" 
                    style={{ padding: "3px 8px", fontSize: 11 }}
                    onClick={() => alert(`探针指标：采集周期 5s，最近上报数据包延迟 8ms，网络良好！`)}
                  >
                    采集日志
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
