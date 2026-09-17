"use client";
import React, { useState } from "react";
import {
  initialProjects,
  initialHosts,
  initialVms,
  initialSwitches,
  initialDatabases,
  initialBusinesses,
  initialProbes,
  initialCredentials,
  ProjectGroup,
  PhysicalHost,
  VmHost,
  SwitchDevice,
  DatabaseAsset,
  BusinessModel,
  CredentialItem,
  IdcRoom,
  IdcCabinet
} from "../cmdbData";
import HostManagement from "./HostManagement";
import SwitchManagement from "./SwitchManagement";
import DbManagement from "./DbManagement";
import ServiceModel from "./ServiceModel";
import ProbeManagement from "./ProbeManagement";
import CredentialManagement from "./CredentialManagement";
import CmdbDashboard from "./CmdbDashboard";

interface CmdbModuleProps {
  page: string;
  onPageChange: (newPage: string) => void;
}

export default function CmdbModule({ page, onPageChange }: CmdbModuleProps) {
  // Centralized State
  const [projects, setProjects] = useState<ProjectGroup[]>(initialProjects);
  const [hosts, setHosts] = useState<PhysicalHost[]>(initialHosts);
  const [vms, setVms] = useState<VmHost[]>(initialVms);
  const [switches, setSwitches] = useState<SwitchDevice[]>(initialSwitches);
  const [databases, setDatabases] = useState<DatabaseAsset[]>(initialDatabases);
  const [businesses, setBusinesses] = useState<BusinessModel[]>(initialBusinesses);
  const [probes, setProbes] = useState(initialProbes);
  const [credentials, setCredentials] = useState(initialCredentials);

  // Handlers for Host
  function handleAddHost(newHost: PhysicalHost) {
    setHosts(prev => [...prev, newHost]);
  }

  function handleDeleteHost(hostId: string) {
    setHosts(prev => prev.filter(h => h.id !== hostId));
  }

  // Handlers for VM
  function handleAddVm(newVm: VmHost) {
    setVms(prev => [...prev, newVm]);
    // update project count
    setProjects(prev => prev.map(p => {
      if (p.name === newVm.projectName || p.id === newVm.projectId) {
        return { ...p, deviceCount: p.deviceCount + 1, vmCount: p.vmCount + 1 };
      }
      return p;
    }));
  }

  function handleDeleteVm(vmId: string) {
    const target = vms.find(v => v.id === vmId);
    setVms(prev => prev.filter(v => v.id !== vmId));
    if (target) {
      setProjects(prev => prev.map(p => {
        if (p.name === target.projectName || p.id === target.projectId) {
          return { ...p, deviceCount: Math.max(0, p.deviceCount - 1), vmCount: Math.max(0, p.vmCount - 1) };
        }
        return p;
      }));
    }
  }

  // Handlers for Switch
  function handleAddSwitch(newSwitch: SwitchDevice) {
    setSwitches(prev => [...prev, newSwitch]);
  }

  function handleDeleteSwitch(swId: string) {
    setSwitches(prev => prev.filter(s => s.id !== swId));
  }

  // Handlers for Database
  function handleAddDb(newDb: DatabaseAsset) {
    setDatabases(prev => [...prev, newDb]);
  }

  function handleDeleteDb(dbId: string) {
    setDatabases(prev => prev.filter(d => d.id !== dbId));
  }

  // Handlers for Credential
  function handleAddCredential(newCred: CredentialItem) {
    setCredentials(prev => [...prev, newCred]);
  }

  function handleDeleteCredential(credId: string) {
    setCredentials(prev => prev.filter(c => c.id !== credId));
  }

  return (
    <div style={{ padding: "14px 18px", height: "calc(100vh - 63px)", marginLeft: 200, overflowY: "auto", background: "#f1f5f9" }}>
      {/* 1. 项目资产 (核心资产管理主工作台，按项目分资产) */}
      {(page === "项目资产" || page === "主机管理" || page === "机房管理") && (
        <HostManagement 
          projects={projects}
          hosts={hosts}
          vms={vms}
          switches={switches}
          databases={databases}
          onAddHost={handleAddHost}
          onDeleteHost={handleDeleteHost}
          onAddVm={handleAddVm}
          onDeleteVm={handleDeleteVm}
        />
      )}

      {/* 2. 资产大盘 (按项目、客户、云厂商、信创全局指标) */}
      {(page === "资产大盘" || page === "仪表盘") && (
        <CmdbDashboard 
          projects={projects}
          hosts={hosts}
          vms={vms}
          switches={switches}
          databases={databases}
          businesses={businesses}
          onNavigate={onPageChange}
        />
      )}

      {/* 3. 业务拓扑 (按项目业务全景拓扑) */}
      {(page === "业务拓扑" || page === "业务模型") && (
        <ServiceModel 
          businesses={businesses}
          hosts={hosts}
          vms={vms}
          databases={databases}
          switches={switches}
          rooms={[]}
          cabinets={[]}
        />
      )}

      {/* 4. 网络与负载 */}
      {(page === "网络与负载" || page === "网络设备") && (
        <SwitchManagement 
          switches={switches}
          rooms={[]}
          cabinets={[]}
          onAddSwitch={handleAddSwitch}
          onDeleteSwitch={handleDeleteSwitch}
        />
      )}

      {/* 5. 数据库管理 */}
      {page === "数据库管理" && (
        <DbManagement 
          databases={databases}
          hosts={hosts}
          businesses={businesses}
          onAddDb={handleAddDb}
          onDeleteDb={handleDeleteDb}
        />
      )}

      {/* 6. 探针监控 */}
      {(page === "探针监控" || page === "探针管理") && (
        <ProbeManagement 
          probes={probes}
          hosts={hosts}
        />
      )}

      {/* 7. 凭据管理 */}
      {page === "凭据管理" && (
        <CredentialManagement 
          credentials={credentials}
          onAddCredential={handleAddCredential}
          onDeleteCredential={handleDeleteCredential}
        />
      )}

      {/* 8. AIops助手 */}
      {page === "AIops助手" && (
        <div className="cmdb-container" style={{ background: "#fff", padding: 20, borderRadius: 8, border: "1px solid #e2e8f0" }}>
          <h3 style={{ margin: "0 0 10px", color: "#0f172a" }}>✦ 项目资产智能 AI 助手</h3>
          <p style={{ color: "#64748b", fontSize: 13 }}>
            针对【北控伟仕智能运维平台】24 个重点业务项目、238 台信息资产提供知识问答与快速定位：
          </p>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 12, marginTop: 16 }}>
            <div 
              style={{ background: "#eff6ff", border: "1px solid #bfdbfe", padding: 14, borderRadius: 8, cursor: "pointer" }}
              onClick={() => onPageChange("项目资产")}
            >
              <b style={{ color: "#1d4ed8", display: "block", marginBottom: 6 }}>📁 查询【原三险系统】13 台计算节点与 Oracle RAC 物理库</b>
              <span style={{ fontSize: 12, color: "#475569" }}>一键进入项目资产工作台查看 128 核 256G 物理机 db3 与应用集群 →</span>
            </div>
            <div 
              style={{ background: "#f0fdf4", border: "1px solid #bbf7d0", padding: 14, borderRadius: 8, cursor: "pointer" }}
              onClick={() => onPageChange("业务拓扑")}
            >
              <b style={{ color: "#15803d", display: "block", marginBottom: 6 }}>🕸 查询【工会互助保险信息系统】22 台国产信创节点拓扑</b>
              <span style={{ fontSize: 12, color: "#475569" }}>查看首信云互联网区全链路业务、网关与应用架构 →</span>
            </div>
            <div 
              style={{ background: "#faf5ff", border: "1px solid #e9d5ff", padding: 14, borderRadius: 8, cursor: "pointer" }}
              onClick={() => onPageChange("资产大盘")}
            >
              <b style={{ color: "#7e22ce", display: "block", marginBottom: 6 }}>📊 查看全网 24 个项目算力核数与存储配额大盘</b>
              <span style={{ fontSize: 12, color: "#475569" }}>查看联通云、首信云、国企云及信创 OS 宏观占比分布 →</span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
