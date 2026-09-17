"use client";
import React, { useState } from "react";
import { DatabaseAsset, PhysicalHost, BusinessModel } from "../cmdbData";

interface DbManagementProps {
  databases: DatabaseAsset[];
  hosts: PhysicalHost[];
  businesses: BusinessModel[];
  onAddDb: (db: DatabaseAsset) => void;
  onDeleteDb: (id: string) => void;
}

export default function DbManagement({
  databases,
  hosts,
  businesses,
  onAddDb,
  onDeleteDb
}: DbManagementProps) {
  const [showModal, setShowModal] = useState(false);
  const [filterType, setFilterType] = useState("全部");

  const [form, setForm] = useState({
    name: "",
    type: "MySQL" as DatabaseAsset["type"],
    version: "8.0.32",
    hostIp: hosts[0]?.ip || "10.100.10.31",
    port: 3306,
    arch: "主从高可用 (1主2从)" as DatabaseAsset["arch"],
    businessId: "biz-1",
    dataSize: "120 GB"
  });

  const filteredDbs = databases.filter(d => filterType === "全部" || d.type === filterType);

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form.name.trim()) return;
    const newDb: DatabaseAsset = {
      id: `db-${Date.now()}`,
      name: form.name,
      type: form.type,
      version: form.version,
      hostIp: form.hostIp,
      port: form.port,
      arch: form.arch,
      businessId: form.businessId,
      dataSize: form.dataSize,
      connectionCount: 150,
      status: "active"
    };
    onAddDb(newDb);
    setShowModal(false);
  }

  return (
    <div className="cmdb-container">
      {/* Top Filter */}
      <div className="cmdb-table-filter">
        <div className="filter-left-inputs">
          <h3 style={{ margin: 0, fontSize: 16, color: "#0f172a" }}>🗄 生产数据库资产与拓扑实例</h3>
          <select value={filterType} onChange={e => setFilterType(e.target.value)}>
            <option value="全部">全部引擎类型</option>
            <option value="MySQL">MySQL 关系型数据库</option>
            <option value="Redis">Redis 内存缓存集群</option>
            <option value="PostgreSQL">PostgreSQL / TimescaleDB</option>
          </select>
        </div>
        <button className="btn-primary" onClick={() => setShowModal(true)}>
          ＋ 纳管数据库实例
        </button>
      </div>

      {/* Database Table */}
      <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, overflow: "hidden" }}>
        <table className="cmdb-data-table">
          <thead>
            <tr>
              <th>数据库实例名称</th>
              <th>引擎类型与版本</th>
              <th>宿主服务器 IP</th>
              <th>监听端口</th>
              <th>高可用部署架构</th>
              <th>数据存储容量</th>
              <th>实时连接数</th>
              <th>支撑业务系统</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {filteredDbs.map(d => {
              const biz = businesses.find(b => b.id === d.businessId);
              const host = hosts.find(h => h.ip === d.hostIp);

              return (
                <tr key={d.id}>
                  <td style={{ fontWeight: 600, color: "#1e293b" }}>{d.name}</td>
                  <td>
                    <span className="room-badge" style={{ background: d.type === "MySQL" ? "#eff6ff" : d.type === "Redis" ? "#fef2f2" : "#f0fdf4", color: d.type === "MySQL" ? "#1d4ed8" : d.type === "Redis" ? "#b91c1c" : "#15803d" }}>
                      {d.type} {d.version}
                    </span>
                  </td>
                  <td style={{ fontFamily: "monospace", color: "#2563eb" }}>
                    {d.hostIp} {host ? `(${host.hostname})` : ""}
                  </td>
                  <td style={{ fontFamily: "monospace" }}>{d.port}</td>
                  <td>{d.arch}</td>
                  <td><b>{d.dataSize}</b></td>
                  <td>{d.connectionCount} active</td>
                  <td>{biz?.name || "核心业务"}</td>
                  <td><span className="status-pill online">● 活跃</span></td>
                  <td>
                    <button 
                      className="btn-secondary" 
                      style={{ padding: "3px 8px", fontSize: 11, marginRight: 6 }}
                      onClick={() => alert(`已为数据库 [${d.name}] 发起慢查询检测与性能健康评估，当前未发现死锁！`)}
                    >
                      慢日志
                    </button>
                    <button 
                      className="btn-secondary" 
                      style={{ padding: "3px 8px", fontSize: 11, color: "#ef4444" }}
                      onClick={() => {
                        if (confirm(`确定要移除数据库资产 [${d.name}] 吗？`)) {
                          onDeleteDb(d.id);
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

      {/* CREATE DB MODAL */}
      {showModal && (
        <div className="cmdb-modal-mask">
          <form className="cmdb-modal" onSubmit={handleSubmit}>
            <div className="cmdb-modal-header">
              <h3>🗄 录入新数据库资产实例</h3>
              <button type="button" className="cmdb-modal-close" onClick={() => setShowModal(false)}>×</button>
            </div>
            <div className="cmdb-modal-body">
              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 实例标识名称</label>
                  <input 
                    placeholder="如：scada_iot_timescale_db" 
                    value={form.name} 
                    onChange={e => setForm({ ...form, name: e.target.value })} 
                    required 
                  />
                </div>
                <div className="form-field-item">
                  <label>数据库引擎类型</label>
                  <select 
                    value={form.type} 
                    onChange={e => {
                      const t = e.target.value as DatabaseAsset["type"];
                      let p = 3306;
                      if (t === "Redis") p = 6379;
                      if (t === "PostgreSQL") p = 5432;
                      if (t === "MongoDB") p = 27017;
                      setForm({ ...form, type: t, port: p });
                    }}
                  >
                    <option value="MySQL">MySQL</option>
                    <option value="Redis">Redis</option>
                    <option value="PostgreSQL">PostgreSQL</option>
                    <option value="MongoDB">MongoDB</option>
                    <option value="Oracle">Oracle</option>
                  </select>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 宿主服务器节点</label>
                  <select 
                    value={form.hostIp} 
                    onChange={e => setForm({ ...form, hostIp: e.target.value })}
                  >
                    {hosts.map(h => (
                      <option key={h.id} value={h.ip}>{h.hostname} ({h.ip})</option>
                    ))}
                  </select>
                </div>
                <div className="form-field-item">
                  <label>* 端口</label>
                  <input 
                    type="number" 
                    value={form.port} 
                    onChange={e => setForm({ ...form, port: parseInt(e.target.value) || 3306 })} 
                    required 
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>高可用部署架构</label>
                  <select 
                    value={form.arch} 
                    onChange={e => setForm({ ...form, arch: e.target.value as any })}
                  >
                    <option value="主从高可用 (1主2从)">主从高可用 (1主2从)</option>
                    <option value="集群分片 (3主3从)">集群分片 (3主3从)</option>
                    <option value="主备高可用">主备高可用</option>
                    <option value="单机测试实例">单机测试实例</option>
                  </select>
                </div>
                <div className="form-field-item">
                  <label>初始数据存储估值</label>
                  <input 
                    value={form.dataSize} 
                    onChange={e => setForm({ ...form, dataSize: e.target.value })} 
                  />
                </div>
              </div>

              <div className="form-field-item">
                <label>所属业务系统</label>
                <select 
                  value={form.businessId} 
                  onChange={e => setForm({ ...form, businessId: e.target.value })}
                >
                  {businesses.map(b => (
                    <option key={b.id} value={b.id}>{b.name}</option>
                  ))}
                </select>
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
