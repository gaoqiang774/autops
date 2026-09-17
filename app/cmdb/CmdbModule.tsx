"use client";
import React, { useState, useEffect } from "react";
import {
  initialRooms,
  initialCabinets,
  initialHosts,
  initialVms,
  initialSwitches,
  initialDatabases,
  initialBusinesses,
  initialProbes,
  initialCredentials,
  IdcRoom,
  IdcCabinet,
  PhysicalHost,
  VmHost,
  SwitchDevice,
  DatabaseAsset,
  BusinessModel,
  CredentialItem
} from "../cmdbData";
import IdcManagement from "./IdcManagement";
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
  // Shared Centralized Relational State
  const [rooms, setRooms] = useState<IdcRoom[]>(initialRooms);
  const [cabinets, setCabinets] = useState<IdcCabinet[]>(initialCabinets);
  const [hosts, setHosts] = useState<PhysicalHost[]>(initialHosts);
  const [vms, setVms] = useState<VmHost[]>(initialVms);
  const [switches, setSwitches] = useState<SwitchDevice[]>(initialSwitches);
  const [databases, setDatabases] = useState<DatabaseAsset[]>(initialDatabases);
  const [businesses, setBusinesses] = useState<BusinessModel[]>(initialBusinesses);
  const [probes, setProbes] = useState(initialProbes);
  const [credentials, setCredentials] = useState(initialCredentials);

  // Recalculate cabinet usedU and power when hosts or switches change
  function recalculateCabinets(curCabinets: IdcCabinet[], curHosts: PhysicalHost[], curSwitches: SwitchDevice[]) {
    return curCabinets.map(cab => {
      const cabHosts = curHosts.filter(h => h.cabinetId === cab.id);
      const cabSwitches = curSwitches.filter(s => s.cabinetId === cab.id);
      
      const usedFromHosts = cabHosts.reduce((acc, h) => acc + h.uHeight, 0);
      const usedFromSwitches = cabSwitches.reduce((acc, s) => acc + s.uHeight, 0);
      const totalUsed = usedFromHosts + usedFromSwitches;

      const hostPower = cabHosts.reduce((acc, h) => acc + (h.powerWatts || 300), 0) / 1000;
      const swPower = cabSwitches.length * 0.25;
      const totalPower = parseFloat((hostPower + swPower).toFixed(1));

      return {
        ...cab,
        usedU: totalUsed,
        currentPower: totalPower,
        status: (totalUsed >= 38 ? "预警" : totalUsed > 0 ? "正常" : "空闲") as IdcCabinet["status"]
      };
    });
  }

  // Handlers for Room
  function handleAddRoom(newRoom: IdcRoom) {
    setRooms(prev => [...prev, newRoom]);
  }

  function handleDeleteRoom(roomId: string) {
    setRooms(prev => prev.filter(r => r.id !== roomId));
  }

  // Handlers for Cabinet
  function handleAddCabinet(newCabinet: IdcCabinet) {
    setCabinets(prev => {
      const next = [...prev, newCabinet];
      return recalculateCabinets(next, hosts, switches);
    });
  }

  function handleDeleteCabinet(cabId: string) {
    setCabinets(prev => prev.filter(c => c.id !== cabId));
  }

  // Handlers for Host
  function handleAddHost(newHost: PhysicalHost) {
    setHosts(prev => {
      const nextHosts = [...prev, newHost];
      setCabinets(cabs => recalculateCabinets(cabs, nextHosts, switches));
      return nextHosts;
    });
  }

  function handleDeleteHost(hostId: string) {
    setHosts(prev => {
      const nextHosts = prev.filter(h => h.id !== hostId);
      setCabinets(cabs => recalculateCabinets(cabs, nextHosts, switches));
      return nextHosts;
    });
  }

  // Handlers for Switch
  function handleAddSwitch(newSwitch: SwitchDevice) {
    setSwitches(prev => {
      const nextSw = [...prev, newSwitch];
      setCabinets(cabs => recalculateCabinets(cabs, hosts, nextSw));
      return nextSw;
    });
  }

  function handleDeleteSwitch(swId: string) {
    setSwitches(prev => {
      const nextSw = prev.filter(s => s.id !== swId);
      setCabinets(cabs => recalculateCabinets(cabs, hosts, nextSw));
      return nextSw;
    });
  }

  // Handlers for VM
  function handleAddVm(newVm: VmHost) {
    setVms(prev => [...prev, newVm]);
  }

  function handleDeleteVm(vmId: string) {
    setVms(prev => prev.filter(v => v.id !== vmId));
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

  // Navigation helpers
  function handleJumpToRack(roomId: string, cabinetId: string) {
    onPageChange("机房管理");
  }

  function handleNavigateToBusiness(bizId: string) {
    onPageChange("业务模型");
  }

  return (
    <div style={{ padding: "14px 18px", height: "calc(100vh - 63px)", marginLeft: 200, overflowY: "auto", background: "#f1f5f9" }}>
      {/* Dynamic Sub-View Rendering */}
      {page === "仪表盘" && (
        <CmdbDashboard 
          rooms={rooms}
          cabinets={cabinets}
          hosts={hosts}
          vms={vms}
          switches={switches}
          databases={databases}
          businesses={businesses}
          onNavigate={onPageChange}
        />
      )}

      {page === "机房管理" && (
        <IdcManagement 
          rooms={rooms}
          cabinets={cabinets}
          hosts={hosts}
          switches={switches}
          businesses={businesses}
          onAddRoom={handleAddRoom}
          onDeleteRoom={handleDeleteRoom}
          onAddCabinet={handleAddCabinet}
          onDeleteCabinet={handleDeleteCabinet}
          onAddHost={handleAddHost}
          onDeleteHost={handleDeleteHost}
          onNavigateToBusiness={handleNavigateToBusiness}
        />
      )}

      {page === "主机管理" && (
        <HostManagement 
          hosts={hosts}
          vms={vms}
          rooms={rooms}
          cabinets={cabinets}
          businesses={businesses}
          onAddHost={handleAddHost}
          onDeleteHost={handleDeleteHost}
          onAddVm={handleAddVm}
          onDeleteVm={handleDeleteVm}
          onJumpToRack={handleJumpToRack}
        />
      )}

      {page === "网络设备" && (
        <SwitchManagement 
          switches={switches}
          rooms={rooms}
          cabinets={cabinets}
          onAddSwitch={handleAddSwitch}
          onDeleteSwitch={handleDeleteSwitch}
        />
      )}

      {page === "数据库管理" && (
        <DbManagement 
          databases={databases}
          hosts={hosts}
          businesses={businesses}
          onAddDb={handleAddDb}
          onDeleteDb={handleDeleteDb}
        />
      )}

      {page === "业务模型" && (
        <ServiceModel 
          businesses={businesses}
          hosts={hosts}
          vms={vms}
          databases={databases}
          switches={switches}
          rooms={rooms}
          cabinets={cabinets}
          onJumpToRack={handleJumpToRack}
        />
      )}

      {page === "探针管理" && (
        <ProbeManagement 
          probes={probes}
          hosts={hosts}
        />
      )}

      {page === "凭据管理" && (
        <CredentialManagement 
          credentials={credentials}
          onAddCredential={handleAddCredential}
          onDeleteCredential={handleDeleteCredential}
        />
      )}

      {page === "AIops助手" && (
        <div className="cmdb-container" style={{ background: "#fff", padding: 20, borderRadius: 8, border: "1px solid #e2e8f0" }}>
          <h3 style={{ margin: "0 0 10px", color: "#0f172a" }}>✦ 资产与拓扑智能 AI 助手</h3>
          <p style={{ color: "#64748b", fontSize: 13 }}>
            针对【北控伟仕智能运维平台】全国数据中心、物理机柜、服务器与业务拓扑提供知识问答与快速定位：
          </p>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 12, marginTop: 16 }}>
            <div 
              style={{ background: "#eff6ff", border: "1px solid #bfdbfe", padding: 14, borderRadius: 8, cursor: "pointer" }}
              onClick={() => onPageChange("机房管理")}
            >
              <b style={{ color: "#1d4ed8", display: "block", marginBottom: 6 }}>📍 查询北京亦庄机房 A01 机柜当前在架物理设备</b>
              <span style={{ fontSize: 12, color: "#475569" }}>立即进入 42U 立面机架图查看 Dell R740 与华为 FusionServer →</span>
            </div>
            <div 
              style={{ background: "#f0fdf4", border: "1px solid #bbf7d0", padding: 14, borderRadius: 8, cursor: "pointer" }}
              onClick={() => onPageChange("业务模型")}
            >
              <b style={{ color: "#15803d", display: "block", marginBottom: 6 }}>🕸 查询「北控水务SCADA系统」支撑设备依赖全景</b>
              <span style={{ fontSize: 12, color: "#475569" }}>查看涵盖 4 台服务器、2 台核心交换机与主从 MySQL 架构 →</span>
            </div>
            <div 
              style={{ background: "#faf5ff", border: "1px solid #e9d5ff", padding: 14, borderRadius: 8, cursor: "pointer" }}
              onClick={() => onPageChange("主机管理")}
            >
              <b style={{ color: "#7e22ce", display: "block", marginBottom: 6 }}>💻 查询全网 4 节点智能探针运行心跳与负载</b>
              <span style={{ fontSize: 12, color: "#475569" }}>查看 CentOS / Ubuntu / openEuler 系统的 CPU 与内存采集 →</span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
