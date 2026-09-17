"use client";
import React, { useState } from "react";
import { SwitchDevice, IdcRoom, IdcCabinet } from "../cmdbData";

interface SwitchManagementProps {
  switches: SwitchDevice[];
  rooms: IdcRoom[];
  cabinets: IdcCabinet[];
  onAddSwitch: (sw: SwitchDevice) => void;
  onDeleteSwitch: (id: string) => void;
}

export default function SwitchManagement({
  switches,
  rooms,
  cabinets,
  onAddSwitch,
  onDeleteSwitch
}: SwitchManagementProps) {
  const [selectedSwitchId, setSelectedSwitchId] = useState<string>(switches[0]?.id || "sw-1");
  const [showModal, setShowModal] = useState(false);

  const selectedSwitch = switches.find(s => s.id === selectedSwitchId) || switches[0];

  const [form, setForm] = useState({
    name: "",
    assetNo: `NET-BJ-ACC0${switches.length + 1}`,
    roomId: "room-1",
    cabinetId: "cab-101",
    startU: 42,
    ip: "10.100.0.15",
    brand: "华为" as SwitchDevice["brand"],
    model: "CloudEngine 6881-48T6CQ",
    role: "接入交换机" as SwitchDevice["role"],
    portCount: 48
  });

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form.name.trim()) return;
    const newSw: SwitchDevice = {
      id: `sw-${Date.now()}`,
      assetNo: form.assetNo,
      name: form.name,
      roomId: form.roomId,
      cabinetId: form.cabinetId,
      startU: form.startU,
      uHeight: 1,
      ip: form.ip,
      brand: form.brand,
      model: form.model,
      role: form.role,
      portCount: form.portCount,
      activePorts: Math.round(form.portCount * 0.7),
      status: "online",
      businessId: "biz-1"
    };
    onAddSwitch(newSw);
    setSelectedSwitchId(newSw.id);
    setShowModal(false);
  }

  return (
    <div className="cmdb-container">
      {/* Switch Overview Header */}
      <div className="cmdb-table-filter">
        <div style={{ display: "flex", gap: 12, alignItems: "center" }}>
          <h3 style={{ margin: 0, fontSize: 16, color: "#0f172a" }}>🌐 网络设备与交换机拓扑矩阵</h3>
          <span className="room-badge">{switches.length} 台在网交换机</span>
        </div>
        <button className="btn-primary" onClick={() => setShowModal(true)}>
          ＋ 录入网络设备
        </button>
      </div>

      {/* Switch Interactive Front Panel Simulation */}
      {selectedSwitch && (
        <div style={{ background: "#0b1120", border: "1px solid #1e293b", borderRadius: 10, padding: 18, color: "#f8fafc", boxShadow: "0 10px 30px rgba(0,0,0,0.3)" }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 14 }}>
            <div>
              <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                <span className="device-brand-badge" style={{ background: "#059669", color: "#fff", fontSize: 11, padding: "2px 8px" }}>
                  {selectedSwitch.brand}
                </span>
                <span style={{ fontSize: 16, fontWeight: 700, color: "#38bdf8" }}>{selectedSwitch.name}</span>
                <span className="status-pill online">● 设备在线</span>
              </div>
              <p style={{ margin: "4px 0 0", fontSize: 12, color: "#94a3b8" }}>
                型号: {selectedSwitch.model} · 管理IP: {selectedSwitch.ip} · 角色: {selectedSwitch.role} · 物理位置: {rooms.find(r => r.id === selectedSwitch.roomId)?.name} ({cabinets.find(c => c.id === selectedSwitch.cabinetId)?.name} · {selectedSwitch.startU}U)
              </p>
            </div>
            <div style={{ textAlign: "right" }}>
              <div style={{ fontSize: 11, color: "#94a3b8" }}>活跃端口占比</div>
              <strong style={{ fontSize: 18, color: "#22c55e" }}>
                {selectedSwitch.activePorts} <small style={{ fontSize: 12, color: "#64748b" }}>/ {selectedSwitch.portCount} Ports</small>
              </strong>
            </div>
          </div>

          {/* 48-Port Visual Grid */}
          <div style={{ background: "#030712", border: "2px solid #334155", borderRadius: 8, padding: 12, display: "flex", flexDirection: "column", gap: 8 }}>
            <div style={{ display: "flex", justifyContent: "space-between", fontSize: 10, color: "#64748b", fontFamily: "monospace" }}>
              <span>PORT MATRIX (GE/10GE SFP+ / 100G QSFP28)</span>
              <span>100Gbps 上联光口 [P41-P48]</span>
            </div>

            <div style={{ display: "grid", gridTemplateColumns: "repeat(24, 1fr)", gap: 4 }}>
              {Array.from({ length: selectedSwitch.portCount }).map((_, idx) => {
                const portNo = idx + 1;
                const isUplink = portNo > selectedSwitch.portCount - 8;
                const isActive = portNo <= selectedSwitch.activePorts;

                let portColor = "#334155";
                let borderColor = "#1e293b";
                if (isActive) {
                  portColor = isUplink ? "#7e22ce" : "#15803d";
                  borderColor = isUplink ? "#a855f7" : "#22c55e";
                }

                return (
                  <div 
                    key={portNo}
                    style={{
                      height: 28,
                      background: portColor,
                      border: `1px solid ${borderColor}`,
                      borderRadius: 3,
                      display: "flex",
                      flexDirection: "column",
                      alignItems: "center",
                      justifyContent: "center",
                      cursor: "pointer",
                      transition: "transform 0.1s"
                    }}
                    title={`端口 ${portNo}: ${isActive ? "Link Up (流量正常 840Mbps)" : "Link Down (空闲)"}`}
                  >
                    <span style={{ width: 4, height: 4, borderRadius: "50%", background: isActive ? "#4ade80" : "#64748b", marginBottom: 2 }} />
                    <span style={{ fontSize: 8, color: "#e2e8f0", fontFamily: "monospace" }}>{portNo}</span>
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      )}

      {/* Switch Table */}
      <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, overflow: "hidden", marginTop: 12 }}>
        <table className="cmdb-data-table">
          <thead>
            <tr>
              <th>资产编号</th>
              <th>设备名称</th>
              <th>管理 IP</th>
              <th>厂商与型号</th>
              <th>角色类型</th>
              <th>端口配置</th>
              <th>物理位置 (机房/机柜/U位)</th>
              <th>运行状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {switches.map(s => {
              const r = rooms.find(room => room.id === s.roomId);
              const c = cabinets.find(cab => cab.id === s.cabinetId);

              return (
                <tr 
                  key={s.id} 
                  style={{ background: s.id === selectedSwitchId ? "#eff6ff" : "#fff", cursor: "pointer" }}
                  onClick={() => setSelectedSwitchId(s.id)}
                >
                  <td style={{ fontFamily: "monospace", fontWeight: 600 }}>{s.assetNo}</td>
                  <td style={{ fontWeight: 600, color: "#1e293b" }}>{s.name}</td>
                  <td style={{ fontFamily: "monospace", color: "#2563eb" }}>{s.ip}</td>
                  <td>{s.brand} {s.model}</td>
                  <td><span className="room-badge">{s.role}</span></td>
                  <td>{s.activePorts}/{s.portCount} 活跃</td>
                  <td>{r?.city} · {c?.name} ({s.startU}U)</td>
                  <td><span className="status-pill online">● 在线</span></td>
                  <td>
                    <button 
                      className="btn-secondary" 
                      style={{ padding: "3px 8px", fontSize: 11, marginRight: 6 }}
                      onClick={(e) => {
                        e.stopPropagation();
                        setSelectedSwitchId(s.id);
                      }}
                    >
                      端口面板
                    </button>
                    <button 
                      className="btn-secondary" 
                      style={{ padding: "3px 8px", fontSize: 11, color: "#ef4444" }}
                      onClick={(e) => {
                        e.stopPropagation();
                        if (confirm(`确定要移除网络设备 ${s.name} 吗？`)) {
                          onDeleteSwitch(s.id);
                        }
                      }}
                    >
                      下架
                    </button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {/* CREATE SWITCH MODAL */}
      {showModal && (
        <div className="cmdb-modal-mask">
          <form className="cmdb-modal" onSubmit={handleSubmit}>
            <div className="cmdb-modal-header">
              <h3>🌐 录入网络交换机设备</h3>
              <button type="button" className="cmdb-modal-close" onClick={() => setShowModal(false)}>×</button>
            </div>
            <div className="cmdb-modal-body">
              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 设备名称</label>
                  <input 
                    placeholder="如：SW-BJ-ACC-03 汇聚交换机" 
                    value={form.name} 
                    onChange={e => setForm({ ...form, name: e.target.value })} 
                    required 
                  />
                </div>
                <div className="form-field-item">
                  <label>* 管理 IP 地址</label>
                  <input 
                    value={form.ip} 
                    onChange={e => setForm({ ...form, ip: e.target.value })} 
                    required 
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 所属机房</label>
                  <select 
                    value={form.roomId} 
                    onChange={e => setForm({ ...form, roomId: e.target.value })}
                  >
                    {rooms.map(r => (
                      <option key={r.id} value={r.id}>{r.name}</option>
                    ))}
                  </select>
                </div>
                <div className="form-field-item">
                  <label>* 所属机柜</label>
                  <select 
                    value={form.cabinetId} 
                    onChange={e => setForm({ ...form, cabinetId: e.target.value })}
                  >
                    {cabinets.filter(c => c.roomId === form.roomId).map(c => (
                      <option key={c.id} value={c.id}>{c.name}</option>
                    ))}
                  </select>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>硬件品牌</label>
                  <select 
                    value={form.brand} 
                    onChange={e => setForm({ ...form, brand: e.target.value as any })}
                  >
                    <option value="华为">华为 Huawei</option>
                    <option value="锐捷">锐捷 Ruijie</option>
                    <option value="思科">思科 Cisco</option>
                    <option value="华三">新华三 H3C</option>
                  </select>
                </div>
                <div className="form-field-item">
                  <label>网络角色</label>
                  <select 
                    value={form.role} 
                    onChange={e => setForm({ ...form, role: e.target.value as any })}
                  >
                    <option value="核心交换机">核心交换机</option>
                    <option value="汇聚交换机">汇聚交换机</option>
                    <option value="接入交换机">接入交换机</option>
                    <option value="带外交换机">带外交换机</option>
                  </select>
                </div>
              </div>
            </div>
            <div className="cmdb-modal-footer">
              <button type="button" className="btn-secondary" onClick={() => setShowModal(false)}>取 消</button>
              <button type="submit" className="btn-primary">确 定 创 建</button>
            </div>
          </form>
        </div>
      )}
    </div>
  );
}
