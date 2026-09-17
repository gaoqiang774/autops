"use client";
import React, { useState, useMemo } from "react";
import { PhysicalHost, VmHost, IdcRoom, IdcCabinet, BusinessModel } from "../cmdbData";

interface HostManagementProps {
  hosts: PhysicalHost[];
  vms: VmHost[];
  rooms: IdcRoom[];
  cabinets: IdcCabinet[];
  businesses: BusinessModel[];
  onAddHost: (h: PhysicalHost) => void;
  onDeleteHost: (id: string) => void;
  onAddVm: (v: VmHost) => void;
  onDeleteVm: (id: string) => void;
  onJumpToRack?: (roomId: string, cabinetId: string) => void;
}

export default function HostManagement({
  hosts,
  vms,
  rooms,
  cabinets,
  businesses,
  onAddHost,
  onDeleteHost,
  onAddVm,
  onDeleteVm,
  onJumpToRack
}: HostManagementProps) {
  const [tab, setTab] = useState<"physical" | "vm">("physical");
  const [keyword, setKeyword] = useState("");
  const [selectedRoom, setSelectedRoom] = useState("全部");
  const [showVmModal, setShowVmModal] = useState(false);

  // New VM Form
  const [vmForm, setVmForm] = useState({
    name: "",
    physicalHostId: hosts[0]?.id || "host-1",
    ip: "10.100.20.105",
    privateIp: "172.16.1.105",
    cpu: "8 核 vCPU",
    memory: "16 GB",
    disk: "100 GB SSD",
    os: "CentOS 7.9",
    businessId: "biz-1",
    role: "微服务容器节点"
  });

  const filteredHosts = useMemo(() => {
    return hosts.filter(h => {
      const matchRoom = selectedRoom === "全部" || h.roomId === selectedRoom;
      const matchKey = !keyword.trim() || 
        h.hostname.toLowerCase().includes(keyword.toLowerCase()) || 
        h.ip.includes(keyword) || 
        h.assetNo.toLowerCase().includes(keyword.toLowerCase());
      return matchRoom && matchKey;
    });
  }, [hosts, selectedRoom, keyword]);

  const filteredVms = useMemo(() => {
    return vms.filter(v => {
      return !keyword.trim() || 
        v.name.toLowerCase().includes(keyword.toLowerCase()) || 
        v.ip.includes(keyword);
    });
  }, [vms, keyword]);

  function handleCreateVm(e: React.FormEvent) {
    e.preventDefault();
    if (!vmForm.name.trim()) return;
    const newVm: VmHost = {
      id: `vm-${Date.now()}`,
      name: vmForm.name,
      physicalHostId: vmForm.physicalHostId,
      ip: vmForm.ip,
      privateIp: vmForm.privateIp,
      cpu: vmForm.cpu,
      memory: vmForm.memory,
      disk: vmForm.disk,
      os: vmForm.os,
      businessId: vmForm.businessId,
      status: "running",
      role: vmForm.role,
      updated: new Date().toLocaleString("zh-CN", { hour12: false }).slice(0, 16)
    };
    onAddVm(newVm);
    setShowVmModal(false);
  }

  return (
    <div className="cmdb-container">
      {/* Top Filter & Metric Bar */}
      <div className="cmdb-table-filter">
        <div className="filter-left-inputs">
          <div style={{ display: "flex", background: "#e2e8f0", padding: 2, borderRadius: 6 }}>
            <button 
              className="btn-secondary"
              style={{
                background: tab === "physical" ? "#2563eb" : "transparent",
                color: tab === "physical" ? "#fff" : "#475569",
                border: 0,
                padding: "6px 12px"
              }}
              onClick={() => setTab("physical")}
            >
              物理服务器 ({hosts.length}台)
            </button>
            <button 
              className="btn-secondary"
              style={{
                background: tab === "vm" ? "#2563eb" : "transparent",
                color: tab === "vm" ? "#fff" : "#475569",
                border: 0,
                padding: "6px 12px"
              }}
              onClick={() => setTab("vm")}
            >
              云主机 / 虚拟服务器 ({vms.length}台)
            </button>
          </div>

          <input 
            placeholder="搜索主机名称 / IP / 资产编码..." 
            value={keyword}
            onChange={e => setKeyword(e.target.value)}
            style={{ width: 220 }}
          />

          {tab === "physical" && (
            <select value={selectedRoom} onChange={e => setSelectedRoom(e.target.value)}>
              <option value="全部">全部机房</option>
              {rooms.map(r => (
                <option key={r.id} value={r.id}>{r.name}</option>
              ))}
            </select>
          )}
        </div>

        <div>
          {tab === "vm" && (
            <button className="btn-primary" onClick={() => setShowVmModal(true)}>
              ＋ 部署新虚拟机
            </button>
          )}
        </div>
      </div>

      {/* PHYSICAL HOSTS TABLE */}
      {tab === "physical" && (
        <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, overflow: "hidden" }}>
          <table className="cmdb-data-table">
            <thead>
              <tr>
                <th>资产编号</th>
                <th>主机名称</th>
                <th>管理 IP</th>
                <th>带外 BMC IP</th>
                <th>物理机架定位 (机房 / 机柜 / U位)</th>
                <th>硬件规格 (CPU / 内存 / 磁盘)</th>
                <th>操作系统</th>
                <th>探针状态</th>
                <th>所属业务系统</th>
                <th>状态</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {filteredHosts.map(h => {
                const room = rooms.find(r => r.id === h.roomId);
                const cab = cabinets.find(c => c.id === h.cabinetId);
                const biz = businesses.find(b => b.id === h.businessId);

                return (
                  <tr key={h.id}>
                    <td style={{ fontFamily: "monospace", fontWeight: 600 }}>{h.assetNo}</td>
                    <td style={{ fontWeight: 600, color: "#1e293b" }}>{h.hostname}</td>
                    <td style={{ fontFamily: "monospace", color: "#2563eb" }}>{h.ip}</td>
                    <td style={{ fontFamily: "monospace", color: "#64748b" }}>{h.bmcIp}</td>
                    <td>
                      <button 
                        style={{ background: "#eff6ff", border: "1px solid #bfdbfe", color: "#1d4ed8", padding: "2px 8px", borderRadius: 4, cursor: "pointer", fontSize: 11 }}
                        onClick={() => onJumpToRack?.(h.roomId, h.cabinetId)}
                        title="点击快速跳转至该机房该机柜立面图"
                      >
                        📍 {room?.city} · {cab?.name} ({h.startU}U-{h.startU + h.uHeight - 1}U)
                      </button>
                    </td>
                    <td style={{ fontSize: 11, color: "#475569" }}>{h.cpu} · {h.memory}</td>
                    <td><span className="room-badge">{h.os}</span></td>
                    <td>
                      <span className="status-pill online">● 探针在线</span>
                    </td>
                    <td>{biz?.name || "核心系统"}</td>
                    <td><span className="status-pill online">● 运行中</span></td>
                    <td>
                      <button 
                        className="btn-secondary" 
                        style={{ padding: "3px 8px", fontSize: 11, marginRight: 6 }}
                        onClick={() => onJumpToRack?.(h.roomId, h.cabinetId)}
                      >
                        机架立面
                      </button>
                      <button 
                        className="btn-secondary" 
                        style={{ padding: "3px 8px", fontSize: 11, color: "#ef4444" }}
                        onClick={() => {
                          if (confirm(`确定要移除主机 ${h.hostname} 吗？`)) {
                            onDeleteHost(h.id);
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
      )}

      {/* VIRTUAL MACHINES (ECS) TABLE */}
      {tab === "vm" && (
        <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, overflow: "hidden" }}>
          <table className="cmdb-data-table">
            <thead>
              <tr>
                <th>虚拟机名称</th>
                <th>宿主物理服务器 (Hypervisor)</th>
                <th>服务 IP</th>
                <th>内部隔离 IP</th>
                <th>虚拟规格 (vCPU / 内存 / 磁盘)</th>
                <th>镜像系统</th>
                <th>服务角色</th>
                <th>归属业务</th>
                <th>运行状态</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {filteredVms.map(v => {
                const parentHost = hosts.find(h => h.id === v.physicalHostId);
                const biz = businesses.find(b => b.id === v.businessId);

                return (
                  <tr key={v.id}>
                    <td style={{ fontWeight: 600, color: "#1e293b" }}>{v.name}</td>
                    <td>
                      <span style={{ color: "#2563eb", fontWeight: 500 }}>
                        💻 {parentHost ? `${parentHost.hostname} (${parentHost.ip})` : "未知物理宿主"}
                      </span>
                    </td>
                    <td style={{ fontFamily: "monospace" }}>{v.ip}</td>
                    <td style={{ fontFamily: "monospace", color: "#64748b" }}>{v.privateIp}</td>
                    <td><span className="room-badge">{v.cpu} · {v.memory} · {v.disk}</span></td>
                    <td>{v.os}</td>
                    <td>{v.role}</td>
                    <td>{biz?.name || "通用平台"}</td>
                    <td><span className="status-pill online">● 正常运行</span></td>
                    <td>
                      <button 
                        className="btn-secondary" 
                        style={{ padding: "3px 8px", fontSize: 11, marginRight: 6 }}
                        onClick={() => alert(`已为虚拟机 [${v.name}] 打开 Web SSH 终端会话！`)}
                      >
                        Web终端
                      </button>
                      <button 
                        className="btn-secondary" 
                        style={{ padding: "3px 8px", fontSize: 11, color: "#ef4444" }}
                        onClick={() => {
                          if (confirm(`确定销毁虚拟机 ${v.name} 吗？`)) {
                            onDeleteVm(v.id);
                          }
                        }}
                      >
                        销毁
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* MODAL: CREATE VM */}
      {showVmModal && (
        <div className="cmdb-modal-mask">
          <form className="cmdb-modal" onSubmit={handleCreateVm}>
            <div className="cmdb-modal-header">
              <h3>💻 编排部署新虚拟云主机 (VM/ECS)</h3>
              <button type="button" className="cmdb-modal-close" onClick={() => setShowVmModal(false)}>×</button>
            </div>
            <div className="cmdb-modal-body">
              <div className="form-field-item">
                <label>* 宿主物理主机 (承载物理机房节点)</label>
                <select 
                  value={vmForm.physicalHostId}
                  onChange={e => setVmForm({ ...vmForm, physicalHostId: e.target.value })}
                >
                  {hosts.map(h => (
                    <option key={h.id} value={h.id}>{h.hostname} (IP: {h.ip}, {h.brand} {h.model})</option>
                  ))}
                </select>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 虚拟机实例名称</label>
                  <input 
                    placeholder="如：vm-scada-core-app03" 
                    value={vmForm.name} 
                    onChange={e => setVmForm({ ...vmForm, name: e.target.value })} 
                    required 
                  />
                </div>
                <div className="form-field-item">
                  <label>* 业务访问 IP</label>
                  <input 
                    value={vmForm.ip} 
                    onChange={e => setVmForm({ ...vmForm, ip: e.target.value })} 
                    required 
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>vCPU 与 内存规格</label>
                  <select 
                    value={vmForm.cpu}
                    onChange={e => setVmForm({ ...vmForm, cpu: e.target.value })}
                  >
                    <option value="4 核 vCPU">4 核 vCPU / 8 GB</option>
                    <option value="8 核 vCPU">8 核 vCPU / 16 GB</option>
                    <option value="16 核 vCPU">16 核 vCPU / 32 GB</option>
                    <option value="32 核 vCPU">32 核 vCPU / 64 GB</option>
                  </select>
                </div>
                <div className="form-field-item">
                  <label>操作系统发行版</label>
                  <select 
                    value={vmForm.os}
                    onChange={e => setVmForm({ ...vmForm, os: e.target.value })}
                  >
                    <option value="CentOS 7.9">CentOS 7.9 64位</option>
                    <option value="Ubuntu 22.04 LTS">Ubuntu 22.04 LTS</option>
                    <option value="Rocky Linux 9.2">Rocky Linux 9.2</option>
                    <option value="openEuler 22.03">openEuler 22.03 LTS</option>
                  </select>
                </div>
              </div>

              <div className="form-field-item">
                <label>所属业务系统</label>
                <select 
                  value={vmForm.businessId}
                  onChange={e => setVmForm({ ...vmForm, businessId: e.target.value })}
                >
                  {businesses.map(b => (
                    <option key={b.id} value={b.id}>{b.name}</option>
                  ))}
                </select>
              </div>

              <div className="form-field-item">
                <label>业务服务角色职责</label>
                <input 
                  value={vmForm.role}
                  onChange={e => setVmForm({ ...vmForm, role: e.target.value })}
                />
              </div>
            </div>
            <div className="cmdb-modal-footer">
              <button type="button" className="btn-secondary" onClick={() => setShowVmModal(false)}>取 消</button>
              <button type="submit" className="btn-primary">确 定 创 建</button>
            </div>
          </form>
        </div>
      )}
    </div>
  );
}
