"use client";
import React, { useState } from "react";
import { BusinessModel, PhysicalHost, DatabaseAsset, SwitchDevice, IdcRoom, IdcCabinet } from "../cmdbData";

interface ServiceModelProps {
  businesses: BusinessModel[];
  hosts: PhysicalHost[];
  databases: DatabaseAsset[];
  switches: SwitchDevice[];
  rooms: IdcRoom[];
  cabinets: IdcCabinet[];
  onJumpToRack?: (roomId: string, cabinetId: string) => void;
}

export default function ServiceModel({
  businesses,
  hosts,
  databases,
  switches,
  rooms,
  cabinets,
  onJumpToRack
}: ServiceModelProps) {
  const [selectedBizId, setSelectedBizId] = useState<string>(businesses[0]?.id || "biz-1");
  const [inspectedNode, setInspectedNode] = useState<{
    title: string;
    type: string;
    detail: string;
    extra?: string;
    roomId?: string;
    cabinetId?: string;
  } | null>(null);

  const selectedBiz = businesses.find(b => b.id === selectedBizId) || businesses[0];

  // Linked assets for this business
  const linkedHosts = hosts.filter(h => h.businessId === selectedBizId);
  const linkedDbs = databases.filter(d => d.businessId === selectedBizId);
  const linkedSwitches = switches.filter(s => s.businessId === selectedBizId);

  return (
    <div className="cmdb-container">
      {/* Top Filter and Business List */}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 12 }}>
        {businesses.map(b => {
          const isSelected = b.id === selectedBizId;
          return (
            <div 
              key={b.id}
              style={{
                background: isSelected ? "#eff6ff" : "#fff",
                border: isSelected ? "2px solid #2563eb" : "1px solid #e2e8f0",
                borderRadius: 8,
                padding: 14,
                cursor: "pointer",
                boxShadow: isSelected ? "0 4px 12px rgba(37,99,235,0.12)" : "0 1px 3px rgba(0,0,0,0.04)",
                transition: "all 0.15s"
              }}
              onClick={() => {
                setSelectedBizId(b.id);
                setInspectedNode(null);
              }}
            >
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 6 }}>
                <span className="room-badge" style={{ background: b.level === "核心 L1" ? "#fee2e2" : "#fef3c7", color: b.level === "核心 L1" ? "#b91c1c" : "#b45309", fontWeight: 600 }}>
                  {b.level}
                </span>
                <span style={{ fontSize: 11, color: "#16a34a", fontWeight: 600 }}>健康度 {b.healthScore}分</span>
              </div>
              <h4 style={{ margin: "0 0 6px", fontSize: 14, color: "#0f172a" }}>{b.name}</h4>
              <p style={{ margin: 0, fontSize: 11, color: "#64748b", lineHeight: 1.4 }}>{b.description}</p>
              <div style={{ display: "flex", gap: 10, marginTop: 10, paddingTop: 8, borderTop: "1px dashed #e2e8f0", fontSize: 11, color: "#475569" }}>
                <span>负责人: {b.owner}</span>
                <span>在管主机: {b.hostIds.length}台</span>
                <span>数据库: {b.dbIds.length}个</span>
              </div>
            </div>
          );
        })}
      </div>

      {/* Interactive Topology Graph Section */}
      <div className="topology-view">
        <header className="topology-header">
          <div>
            <h4>🕸 全链路多层级物理/虚拟依赖架构拓扑：{selectedBiz.name}</h4>
            <p style={{ margin: "4px 0 0", fontSize: 12, color: "#94a3b8" }}>
              真实关联至 IDC 机房、标准 42U 机架 U 位、物理网络交换机、物理服务器与数据库集群
            </p>
          </div>
          <div style={{ display: "flex", gap: 10 }}>
            <span className="status-pill online">架构链路全绿 · 无单点故障</span>
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
                  <small>安全等级: {selectedBiz.level} · 业务线</small>
                </div>
              </div>
            </div>
          </div>

          {/* LAYER 2: NETWORK & GATEWAY */}
          <div>
            <div className="topology-layer-title">LEVEL 2 · 网络与边界汇聚层 (NETWORK & GATEWAY LAYER)</div>
            <div className="topology-nodes-row">
              {linkedSwitches.map(sw => {
                const r = rooms.find(room => room.id === sw.roomId);
                const c = cabinets.find(cab => cab.id === sw.cabinetId);

                return (
                  <div 
                    key={sw.id}
                    className="topology-node-card"
                    onClick={() => setInspectedNode({
                      title: sw.name,
                      type: "网络核心设备",
                      detail: `管理 IP: ${sw.ip} · 端口: ${sw.activePorts}/${sw.portCount} Active`,
                      extra: `物理位置: ${r?.name} (${c?.name} · ${sw.startU}U)`,
                      roomId: sw.roomId,
                      cabinetId: sw.cabinetId
                    })}
                  >
                    <div className="topology-node-icon switch">🌐</div>
                    <div className="topology-node-meta">
                      <strong>{sw.name}</strong>
                      <small>IP: {sw.ip} · {r?.city} {c?.name} {sw.startU}U</small>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          {/* LAYER 3: COMPUTE SERVERS */}
          <div>
            <div className="topology-layer-title">LEVEL 3 · 物理算力机架层 (PHYSICAL COMPUTE & RACK LAYER)</div>
            <div className="topology-nodes-row">
              {linkedHosts.map(h => {
                const r = rooms.find(room => room.id === h.roomId);
                const c = cabinets.find(cab => cab.id === h.cabinetId);

                return (
                  <div 
                    key={h.id}
                    className="topology-node-card"
                    onClick={() => setInspectedNode({
                      title: h.hostname,
                      type: "物理机算力宿主",
                      detail: `管理IP: ${h.ip} · 带外BMC: ${h.bmcIp} · 硬件配置: ${h.cpu}, ${h.memory}`,
                      extra: `物理部署: ${r?.name} · ${c?.name} (起始: ${h.startU}U, 高度: ${h.uHeight}U)`,
                      roomId: h.roomId,
                      cabinetId: h.cabinetId
                    })}
                  >
                    <div className="topology-node-icon host">💻</div>
                    <div className="topology-node-meta">
                      <strong>{h.hostname}</strong>
                      <small>IP: {h.ip} · {c?.name} ({h.startU}U-{h.startU + h.uHeight - 1}U)</small>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>

          {/* LAYER 4: DATABASE PERSISTENCE */}
          <div>
            <div className="topology-layer-title">LEVEL 4 · 数据中枢与持久化层 (DATABASE PERSISTENCE LAYER)</div>
            <div className="topology-nodes-row">
              {linkedDbs.map(db => (
                <div 
                  key={db.id}
                  className="topology-node-card"
                  onClick={() => setInspectedNode({
                    title: db.name,
                    type: "持久化数据实例",
                    detail: `引擎: ${db.type} ${db.version} · 连接端点: ${db.hostIp}:${db.port}`,
                    extra: `架构模式: ${db.arch} · 数据体积: ${db.dataSize} · 当前连接数: ${db.connectionCount}`
                  })}
                >
                  <div className="topology-node-icon db">🗄</div>
                  <div className="topology-node-meta">
                    <strong>{db.name}</strong>
                    <small>{db.type} {db.version} · {db.hostIp}:{db.port}</small>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Selected Node Details Drawer */}
        {inspectedNode && (
          <div style={{ marginTop: 20, background: "#1e293b", border: "1px solid #38bdf8", borderRadius: 8, padding: 14 }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 6 }}>
              <strong style={{ fontSize: 14, color: "#38bdf8" }}>🔍 拓扑节点明细：{inspectedNode.title}</strong>
              <button 
                style={{ background: "none", border: 0, color: "#94a3b8", cursor: "pointer", fontSize: 16 }}
                onClick={() => setInspectedNode(null)}
              >
                ×
              </button>
            </div>
            <div style={{ fontSize: 12, color: "#e2e8f0", marginBottom: 4 }}>
              <b>类型：</b>{inspectedNode.type} · <b>详细配置：</b>{inspectedNode.detail}
            </div>
            {inspectedNode.extra && (
              <div style={{ fontSize: 12, color: "#94a3b8" }}>
                <b>关联环境：</b>{inspectedNode.extra}
              </div>
            )}
            {inspectedNode.roomId && inspectedNode.cabinetId && (
              <button 
                className="btn-primary"
                style={{ marginTop: 10, padding: "4px 12px", fontSize: 11 }}
                onClick={() => onJumpToRack?.(inspectedNode.roomId!, inspectedNode.cabinetId!)}
              >
                📍 立即前往该物理机柜 (42U 立面图) 定位
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
