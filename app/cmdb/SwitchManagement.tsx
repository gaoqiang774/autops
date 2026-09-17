"use client";
import React, { useState } from "react";
import { SwitchDevice } from "../cmdbData";

interface SwitchManagementProps {
  switches: SwitchDevice[];
  rooms?: any[];
  cabinets?: any[];
  onAddSwitch: (sw: SwitchDevice) => void;
  onDeleteSwitch: (id: string) => void;
}

export default function SwitchManagement({
  switches,
  onAddSwitch,
  onDeleteSwitch
}: SwitchManagementProps) {
  const [selectedSwitchId, setSelectedSwitchId] = useState<string>(switches[0]?.id || "sw-1");
  const [showModal, setShowModal] = useState(false);

  const selectedSwitch = switches.find(s => s.id === selectedSwitchId) || switches[0];

  const [form, setForm] = useState({
    name: "",
    assetNo: `NET-ACC0${switches.length + 1}`,
    projectName: "工会职服数智化系统",
    customerName: "北京市总工会职工服务中心",
    cloudVendor: "首信云",
    ip: "172.25.147.252",
    brand: "锐捷",
    model: "RG-EG3000G / 高可用SLB",
    role: "负载均衡设备",
    portCount: 48
  });

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form.name.trim()) return;
    const newSw: SwitchDevice = {
      id: `sw-${Date.now()}`,
      assetNo: form.assetNo,
      name: form.name,
      ip: form.ip,
      brand: form.brand,
      model: form.model,
      role: form.role,
      portCount: form.portCount,
      activePorts: Math.round(form.portCount * 0.7),
      status: "online",
      businessId: "biz-1",
      projectName: form.projectName,
      customerName: form.customerName,
      cloudVendor: form.cloudVendor,
      deviceType: "负载均衡",
      category: "网络"
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
          <h3 style={{ margin: 0, fontSize: 16, color: "#0f172a" }}>🌐 网络设备与项目负载均衡拓扑</h3>
          <span className="room-badge">{switches.length} 台在网设备</span>
        </div>
        <button className="btn-primary" onClick={() => setShowModal(true)}>
          ＋ 录入网络/负载设备
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
                <span className="status-pill online">● 运行正常</span>
              </div>
              <p style={{ margin: "4px 0 0", fontSize: 12, color: "#94a3b8" }}>
                型号: {selectedSwitch.model} · 管理/VIP: {selectedSwitch.ip} · 角色: {selectedSwitch.role} · 归属项目: {selectedSwitch.projectName || "通用业务"} ({selectedSwitch.customerName || "北控伟仕"})
              </p>
            </div>
            <div style={{ textAlign: "right" }}>
              <div style={{ fontSize: 11, color: "#94a3b8" }}>活跃链路端口</div>
              <strong style={{ fontSize: 18, color: "#22c55e" }}>
                {selectedSwitch.activePorts} <small style={{ fontSize: 12, color: "#64748b" }}>/ {selectedSwitch.portCount} Ports</small>
              </strong>
            </div>
          </div>

          {/* 48-Port Visual Grid */}
          <div style={{ background: "#030712", border: "2px solid #334155", borderRadius: 8, padding: 12, display: "flex", flexDirection: "column", gap: 8 }}>
            <div style={{ display: "flex", justifyContent: "space-between", fontSize: 10, color: "#64748b", fontFamily: "monospace" }}>
              <span>PORT MATRIX (GE/10GE SFP+ / 100G QSFP28)</span>
              <span>100Gbps 上联双光口 [P41-P48]</span>
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
      <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, overflow: "hidden", marginTop: 14 }}>
        <table className="cmdb-data-table">
          <thead>
            <tr>
              <th>资产编号</th>
              <th>设备名称</th>
              <th>管理 / VIP 地址</th>
              <th>品牌型号</th>
              <th>设备角色</th>
              <th>归属业务项目</th>
              <th>客户单位</th>
              <th>端口利用率</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {switches.map(s => {
              const isSelected = s.id === selectedSwitchId;

              return (
                <tr 
                  key={s.id}
                  style={{ background: isSelected ? "#f8fafc" : "#fff", cursor: "pointer" }}
                  onClick={() => setSelectedSwitchId(s.id)}
                >
                  <td style={{ fontFamily: "monospace", fontWeight: 600 }}>{s.assetNo}</td>
                  <td style={{ fontWeight: 600, color: "#1e293b" }}>{s.name}</td>
                  <td style={{ fontFamily: "monospace", color: "#2563eb" }}>{s.ip}</td>
                  <td>{s.brand} {s.model}</td>
                  <td><span className="room-badge">{s.role}</span></td>
                  <td style={{ fontWeight: 500, color: "#1e293b" }}>{s.projectName || "通用业务"}</td>
                  <td style={{ color: "#64748b" }}>{s.customerName || "北控伟仕"}</td>
                  <td>
                    <span style={{ fontSize: 11, color: "#16a34a", fontWeight: 600 }}>
                      {s.activePorts} / {s.portCount} ({Math.round((s.activePorts / s.portCount) * 100)}%)
                    </span>
                  </td>
                  <td><span className="status-pill online">● 正常</span></td>
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
                        if (confirm(`确定要移除设备 ${s.name} 吗？`)) {
                          onDeleteSwitch(s.id);
                        }
                      }}
                    >
                      移除
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
              <h3>🌐 录入网络/负载均衡设备</h3>
              <button type="button" className="cmdb-modal-close" onClick={() => setShowModal(false)}>×</button>
            </div>
            <div className="cmdb-modal-body">
              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 设备名称</label>
                  <input 
                    placeholder="如：VIP-k8s-apiserver" 
                    value={form.name} 
                    onChange={e => setForm({ ...form, name: e.target.value })} 
                    required 
                  />
                </div>
                <div className="form-field-item">
                  <label>* 管理 / VIP 地址</label>
                  <input 
                    value={form.ip} 
                    onChange={e => setForm({ ...form, ip: e.target.value })} 
                    required 
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 归属项目名称</label>
                  <input 
                    value={form.projectName} 
                    onChange={e => setForm({ ...form, projectName: e.target.value })} 
                    required 
                  />
                </div>
                <div className="form-field-item">
                  <label>* 客户单位</label>
                  <input 
                    value={form.customerName} 
                    onChange={e => setForm({ ...form, customerName: e.target.value })} 
                    required 
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>品牌与型号</label>
                  <input 
                    value={form.model} 
                    onChange={e => setForm({ ...form, model: e.target.value })} 
                  />
                </div>
                <div className="form-field-item">
                  <label>设备角色</label>
                  <select 
                    value={form.role} 
                    onChange={e => setForm({ ...form, role: e.target.value })}
                  >
                    <option value="负载均衡设备">负载均衡设备</option>
                    <option value="核心交换机">核心交换机</option>
                    <option value="汇聚交换机">汇聚交换机</option>
                    <option value="接入交换机">接入交换机</option>
                  </select>
                </div>
              </div>
            </div>
            <div className="cmdb-modal-footer">
              <button type="button" className="btn-secondary" onClick={() => setShowModal(false)}>取 消</button>
              <button type="submit" className="btn-primary">确 定 录 入</button>
            </div>
          </form>
        </div>
      )}
    </div>
  );
}
