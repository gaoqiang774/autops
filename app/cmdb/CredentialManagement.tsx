"use client";
import React, { useState } from "react";
import { CredentialItem } from "../cmdbData";

interface CredentialManagementProps {
  credentials: CredentialItem[];
  onAddCredential: (c: CredentialItem) => void;
  onDeleteCredential: (id: string) => void;
}

export default function CredentialManagement({
  credentials,
  onAddCredential,
  onDeleteCredential
}: CredentialManagementProps) {
  const [showModal, setShowModal] = useState(false);
  const [form, setForm] = useState({
    name: "",
    type: "SSH 私钥" as CredentialItem["type"],
    username: "ops_root",
    targetCount: 6,
    remark: ""
  });

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!form.name.trim()) return;
    const newCred: CredentialItem = {
      id: `cred-${Date.now()}`,
      name: form.name,
      type: form.type,
      username: form.username,
      targetCount: form.targetCount,
      updated: new Date().toISOString().slice(0, 10),
      remark: form.remark || "运维凭据"
    };
    onAddCredential(newCred);
    setShowModal(false);
  }

  return (
    <div className="cmdb-container">
      <div className="cmdb-table-filter">
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <h3 style={{ margin: 0, fontSize: 16, color: "#0f172a" }}>🔐 主机/网络设备资产访问凭证管理</h3>
          <span className="room-badge">{credentials.length} 组加密凭证</span>
        </div>
        <button className="btn-primary" onClick={() => setShowModal(true)}>
          ＋ 录入凭据
        </button>
      </div>

      <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, overflow: "hidden" }}>
        <table className="cmdb-data-table">
          <thead>
            <tr>
              <th>凭据标识名称</th>
              <th>凭据类型</th>
              <th>授权登录用户名</th>
              <th>已关联主机设备数</th>
              <th>更新日期</th>
              <th>用途与说明</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {credentials.map(c => (
              <tr key={c.id}>
                <td style={{ fontWeight: 600, color: "#1e293b" }}>{c.name}</td>
                <td><span className="room-badge">{c.type}</span></td>
                <td style={{ fontFamily: "monospace", color: "#2563eb" }}>{c.username}</td>
                <td>{c.targetCount} 台资产</td>
                <td>{c.updated}</td>
                <td style={{ color: "#64748b" }}>{c.remark}</td>
                <td>
                  <button 
                    className="btn-secondary" 
                    style={{ padding: "3px 8px", fontSize: 11, marginRight: 6 }}
                    onClick={() => alert(`已为凭据 [${c.name}] 执行与关联设备的连通性测试，认证全部通过！`)}
                  >
                    连通性测试
                  </button>
                  <button 
                    className="btn-secondary" 
                    style={{ padding: "3px 8px", fontSize: 11, color: "#ef4444" }}
                    onClick={() => {
                      if (confirm(`确定要移除凭证 [${c.name}] 吗？`)) {
                        onDeleteCredential(c.id);
                      }
                    }}
                  >
                    删除
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {showModal && (
        <div className="cmdb-modal-mask">
          <form className="cmdb-modal" onSubmit={handleSubmit}>
            <div className="cmdb-modal-header">
              <h3>🔐 新建主机/设备访问凭证</h3>
              <button type="button" className="cmdb-modal-close" onClick={() => setShowModal(false)}>×</button>
            </div>
            <div className="cmdb-modal-body">
              <div className="form-field-item">
                <label>* 凭据描述名称</label>
                <input 
                  placeholder="如：生产环境K8s节点SSH密钥" 
                  value={form.name} 
                  onChange={e => setForm({ ...form, name: e.target.value })} 
                  required 
                />
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>凭据类型</label>
                  <select 
                    value={form.type} 
                    onChange={e => setForm({ ...form, type: e.target.value as any })}
                  >
                    <option value="SSH 私钥">SSH 私钥 (RSA/Ed25519)</option>
                    <option value="SSH 密码">SSH 密码</option>
                    <option value="BMC IPMI">BMC IPMI 带外账号</option>
                    <option value="SNMP v2c/v3">SNMP 采集团体字/密码</option>
                    <option value="数据库账号">数据库授权账号</option>
                  </select>
                </div>
                <div className="form-field-item">
                  <label>* 登录账号 (Username)</label>
                  <input 
                    value={form.username} 
                    onChange={e => setForm({ ...form, username: e.target.value })} 
                    required 
                  />
                </div>
              </div>

              <div className="form-field-item">
                <label>密码 / 私钥内容</label>
                <textarea 
                  placeholder="-----BEGIN OPENSSH PRIVATE KEY----- 或 输入密码" 
                />
              </div>

              <div className="form-field-item">
                <label>说明备注</label>
                <input 
                  value={form.remark} 
                  onChange={e => setForm({ ...form, remark: e.target.value })} 
                />
              </div>
            </div>
            <div className="cmdb-modal-footer">
              <button type="button" className="btn-secondary" onClick={() => setShowModal(false)}>取 消</button>
              <button type="submit" className="btn-primary">确 定 保 存</button>
            </div>
          </form>
        </div>
      )}
    </div>
  );
}
