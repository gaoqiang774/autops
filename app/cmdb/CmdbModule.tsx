"use client";
import React, { useState } from "react";
import {
  initialProjects,
  initialHosts,
  initialVms,
  initialSwitches,
  initialBusinesses,
  initialDatabases,
  initialCredentials,
  initialSoftwareComponents,
  initialOpsChannels,
  ProjectGroup,
  PhysicalHost,
  VmHost,
  SwitchDevice,
  DatabaseAsset,
  BusinessModel,
  CredentialItem,
  SoftwareComponent,
  OpsChannel,
  IdcRoom,
  IdcCabinet
} from "../cmdbData";
import HostManagement from "./HostManagement";
import ServiceModel from "./ServiceModel";
import CredentialManagement from "./CredentialManagement";
import CmdbDashboard from "./CmdbDashboard";
import VulnDetection from "./VulnDetection";
import { getAssetKey } from "./excelExport";
import { ImportStrategy } from "./ImportModal";

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
  const [databases] = useState<DatabaseAsset[]>(initialDatabases);
  const [businesses, setBusinesses] = useState<BusinessModel[]>(initialBusinesses);
  const [credentials, setCredentials] = useState(initialCredentials);
  const [softwareList, setSoftwareList] = useState<SoftwareComponent[]>(initialSoftwareComponents);
  const [channelList, setChannelList] = useState<OpsChannel[]>(initialOpsChannels);

  // Handlers for Software Components
  function handleAddSoftware(newSoft: SoftwareComponent) {
    setSoftwareList(prev => [newSoft, ...prev]);
  }

  function handleDeleteSoftware(softId: string) {
    setSoftwareList(prev => prev.filter(s => s.id !== softId));
  }

  // Handlers for Ops Channels
  function handleAddChannel(newChan: OpsChannel) {
    setChannelList(prev => [newChan, ...prev]);
  }

  function handleDeleteChannel(chanId: string) {
    setChannelList(prev => prev.filter(c => c.id !== chanId));
  }

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

  // Project Management
  function handleAddProject(newProject: ProjectGroup) {
    setProjects(prev => [newProject, ...prev]);
  }

  // VM Host Update
  function handleUpdateVm(updatedVm: VmHost) {
    setVms(prev => {
      const next = prev.map(v => v.id === updatedVm.id ? updatedVm : v);
      setTimeout(() => {
        recalculateProjectStats(next, hosts, switches);
      }, 0);
      return next;
    });
  }

  // Delete Unified Asset (VM, Physical Host, or Switch)
  function handleDeleteUnifiedAsset(assetId: string, kind: "physical" | "vm" | "switch" = "vm") {
    let nextVms = vms;
    let nextHosts = hosts;
    let nextSwitches = switches;

    if (kind === "physical") {
      nextHosts = hosts.filter(h => h.id !== assetId);
      setHosts(nextHosts);
    } else if (kind === "switch") {
      nextSwitches = switches.filter(s => s.id !== assetId);
      setSwitches(nextSwitches);
    } else {
      nextVms = vms.filter(v => v.id !== assetId);
      setVms(nextVms);
    }

    recalculateProjectStats(nextVms, nextHosts, nextSwitches);
  }

  // Recalculate all project statistics dynamically based on current unique devices
  function recalculateProjectStats(updatedVms: VmHost[], updatedHosts: PhysicalHost[], updatedSwitches: SwitchDevice[]) {
    const projMap: Record<string, { total: number; phy: number; vm: number; net: number; cores: number; mem: number; disk: number; xc: number }> = {};

    for (const v of updatedVms) {
      const p = v.projectName || "未分类项目";
      if (!projMap[p]) projMap[p] = { total: 0, phy: 0, vm: 0, net: 0, cores: 0, mem: 0, disk: 0, xc: 0 };
      projMap[p].total++;
      projMap[p].vm++;
      projMap[p].cores += v.cpuCores || 0;
      projMap[p].mem += v.memoryGb || 0;
      projMap[p].disk += (v.systemDiskGb || 0) + (v.dataDiskGb || 0);
      if (v.isXinchuang === "是") projMap[p].xc++;
    }

    for (const h of updatedHosts) {
      const p = h.projectName || "未分类项目";
      if (!projMap[p]) projMap[p] = { total: 0, phy: 0, vm: 0, net: 0, cores: 0, mem: 0, disk: 0, xc: 0 };
      projMap[p].total++;
      projMap[p].phy++;
      projMap[p].cores += h.cpuCores || 0;
      projMap[p].mem += h.memoryGb || 0;
      projMap[p].disk += (h.systemDiskGb || 0) + (h.dataDiskGb || 0);
      if (h.isXinchuang === "是") projMap[p].xc++;
    }

    for (const s of updatedSwitches) {
      const p = s.projectName || "未分类项目";
      if (!projMap[p]) projMap[p] = { total: 0, phy: 0, vm: 0, net: 0, cores: 0, mem: 0, disk: 0, xc: 0 };
      projMap[p].total++;
      projMap[p].net++;
      if (s.isXinchuang === "是") projMap[p].xc++;
    }

    setProjects(prev => prev.map(p => {
      const st = projMap[p.name];
      if (st) {
        return {
          ...p,
          deviceCount: st.total,
          phyCount: st.phy,
          vmCount: st.vm,
          netCount: st.net,
          totalCores: st.cores,
          totalMemoryGb: st.mem,
          totalDiskGb: st.disk,
          xinchuangCount: st.xc
        };
      }
      return {
        ...p,
        deviceCount: 0,
        phyCount: 0,
        vmCount: 0,
        netCount: 0
      };
    }));
  }

  // Deduplicate current assets in state
  function handleDeduplicateAssets() {
    const seen = new Set<string>();
    let removed = 0;
    const cleanVms: VmHost[] = [];
    for (const item of vms) {
      const k = getAssetKey(item);
      if (seen.has(k)) {
        removed++;
      } else {
        seen.add(k);
        cleanVms.push(item);
      }
    }
    setVms(cleanVms);
    recalculateProjectStats(cleanVms, hosts, switches);
    return { removedCount: removed };
  }

  // Batch import with deduplication and strategy (upsert / skip / replace)
  function handleBatchImportAssets(
    newAssets: (VmHost & { isImported?: boolean })[],
    strategy: ImportStrategy = "upsert",
    targetProjectName?: string | null
  ) {
    let finalVms: VmHost[] = [];

    setVms(prev => {
      // First, deduplicate existing vms to ensure clean baseline
      const existingMap = new Map<string, VmHost>();
      for (const item of prev) {
        const k = getAssetKey(item);
        if (!existingMap.has(k)) {
          existingMap.set(k, item);
        }
      }

      if (strategy === "replace") {
        if (targetProjectName && targetProjectName !== "全部项目总览") {
          const others = Array.from(existingMap.values()).filter(v => v.projectName !== targetProjectName);
          finalVms = [...newAssets, ...others];
        } else {
          finalVms = [...newAssets];
        }
      } else if (strategy === "skip") {
        // Only append those that don't exist
        const toAppend: VmHost[] = [];
        for (const asset of newAssets) {
          const k = getAssetKey(asset);
          if (!existingMap.has(k)) {
            existingMap.set(k, asset);
            toAppend.push(asset);
          }
        }
        finalVms = Array.from(existingMap.values());
      } else {
        // Default: "upsert" - smart merge in-place, zero duplicates
        for (const incoming of newAssets) {
          const k = getAssetKey(incoming);
          if (existingMap.has(k)) {
            const old = existingMap.get(k)!;
            existingMap.set(k, {
              ...old,
              ...incoming,
              id: old.id, // preserve persistent unique ID
              isImported: true,
              updated: new Date().toISOString().slice(0, 10)
            });
          } else {
            existingMap.set(k, incoming);
          }
        }
        finalVms = Array.from(existingMap.values());
      }

      return finalVms;
    });

    setTimeout(() => {
      recalculateProjectStats(finalVms, hosts, switches);
    }, 0);
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
          softwareList={softwareList}
          channelList={channelList}
          onAddSoftware={handleAddSoftware}
          onDeleteSoftware={handleDeleteSoftware}
          onAddChannel={handleAddChannel}
          onDeleteChannel={handleDeleteChannel}
          onAddHost={handleAddHost}
          onDeleteHost={handleDeleteHost}
          onAddVm={handleAddVm}
          onDeleteVm={handleDeleteVm}
          onUpdateVm={handleUpdateVm}
          onDeleteUnifiedAsset={handleDeleteUnifiedAsset}
          onAddProject={handleAddProject}
          onBatchImportAssets={handleBatchImportAssets}
          onDeduplicateAssets={handleDeduplicateAssets}
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

      {/* 4. 漏洞检测 (基于操作系统家族、版本号与内核快速定位受威胁资产) */}
      {(page === "漏洞检测" || page === "漏洞排查") && (
        <VulnDetection 
          projects={projects}
          hosts={hosts}
          vms={vms}
          switches={switches}
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
            针对【运维信息资产管理平台】24 个重点业务项目、292 台信息资产提供知识问答与快速定位：
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
