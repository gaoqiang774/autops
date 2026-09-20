"use client";
import React, { useState, useMemo } from "react";
import { UserAccount } from "./userTypes";
import { ProjectGroup } from "../cmdbData";

interface UserManagementProps {
  currentUser: UserAccount;
  users: UserAccount[];
  allProjects: ProjectGroup[];
  onAddUser: (user: UserAccount) => void;
  onUpdateUser: (user: UserAccount) => void;
  onDeleteUser: (userId: string) => void;
  onSwitchUser: (user: UserAccount) => void;
  onPageChange?: (page: string) => void;
}

export default function UserManagement({
  currentUser,
  users,
  allProjects,
  onAddUser,
  onUpdateUser,
  onDeleteUser,
  onSwitchUser,
  onPageChange
}: UserManagementProps) {
  // Modal states
  const [showAddModal, setShowAddModal] = useState(false);
  const [showAuthModal, setShowAuthModal] = useState(false);
  const [editingUser, setEditingUser] = useState<UserAccount | null>(null);

  // New User Form State
  const [newUsername, setNewUsername] = useState("");
  const [newDisplayName, setNewDisplayName] = useState("");
  const [newPassword, setNewPassword] = useState("123456");
  const [newRole, setNewRole] = useState<"admin" | "operator" | "auditor">("operator");
  const [newDepartment, setNewDepartment] = useState("政务运维服务部");
  const [newPhone, setNewPhone] = useState("");
  const [newEmail, setNewEmail] = useState("");
  const [newAuthMode, setNewAuthMode] = useState<"all" | "custom">("custom");
  const [newSelectedProjects, setNewSelectedProjects] = useState<string[]>([]);
  const [formError, setFormError] = useState("");

  // Authorization Modal State (for selected user)
  const [targetAuthUser, setTargetAuthUser] = useState<UserAccount | null>(null);
  const [authMode, setAuthMode] = useState<"all" | "custom">("custom");
  const [selectedProjects, setSelectedProjects] = useState<string[]>([]);
  const [projectSearch, setProjectSearch] = useState("");
  const [saveFeedback, setSaveFeedback] = useState<string | null>(null);

  // Table search & filter
  const [filterKeyword, setFilterKeyword] = useState("");
  const [filterRole, setFilterRole] = useState<string>("全部");

  const isAdmin = currentUser.role === "admin" || currentUser.username === "admin";

  // Filtered users list
  const filteredUsers = useMemo(() => {
    return users.filter((u) => {
      if (filterRole !== "全部" && u.role !== filterRole) return false;
      if (filterKeyword.trim()) {
        const kw = filterKeyword.toLowerCase();
        const match =
          u.username.toLowerCase().includes(kw) ||
          u.displayName.toLowerCase().includes(kw) ||
          u.department.toLowerCase().includes(kw) ||
          (u.email && u.email.toLowerCase().includes(kw));
        if (!match) return false;
      }
      return true;
    });
  }, [users, filterRole, filterKeyword]);

  // Handle open Authorization Modal
  function handleOpenAuthModal(user: UserAccount) {
    if (!isAdmin) {
      alert("权限不足：仅系统超级管理员 admin 具备项目权限授权资格！");
      return;
    }
    setTargetAuthUser(user);
    if (user.authorizedProjects === "all") {
      setAuthMode("all");
      setSelectedProjects(allProjects.map((p) => p.name));
    } else {
      setAuthMode("custom");
      setSelectedProjects([...user.authorizedProjects]);
    }
    setProjectSearch("");
    setShowAuthModal(true);
  }

  // Handle save Authorization
  function handleSaveAuth(e: React.FormEvent) {
    e.preventDefault();
    if (!targetAuthUser) return;

    let finalProjects: "all" | string[];
    if (authMode === "all") {
      finalProjects = "all";
    } else {
      if (selectedProjects.length === 0) {
        if (!confirm("您未勾选任何项目，保存后该用户将无法查看任何业务资产，是否继续？")) {
          return;
        }
      }
      finalProjects = selectedProjects;
    }

    const updated: UserAccount = {
      ...targetAuthUser,
      authorizedProjects: finalProjects
    };

    onUpdateUser(updated);
    setShowAuthModal(false);
    setSaveFeedback(`已成功为用户【${updated.displayName}】更新项目访问权限！`);
    setTimeout(() => setSaveFeedback(null), 3500);
  }

  // Handle submit Add User
  function handleCreateUser(e: React.FormEvent) {
    e.preventDefault();
    if (!newUsername.trim() || !newDisplayName.trim()) {
      setFormError("请填写登录用户名与真实姓名");
      return;
    }
    if (users.some((u) => u.username.toLowerCase() === newUsername.trim().toLowerCase())) {
      setFormError(`用户名 [${newUsername.trim()}] 已存在，请更换`);
      return;
    }

    const newUser: UserAccount = {
      id: `user-${Date.now()}`,
      username: newUsername.trim(),
      displayName: newDisplayName.trim(),
      role: newRole,
      department: newDepartment.trim() || "运维中心",
      phone: newPhone.trim() || "13800000000",
      email: newEmail.trim() || `${newUsername.trim()}@beikong.com`,
      status: "active",
      authorizedProjects: newAuthMode === "all" ? "all" : newSelectedProjects,
      createdAt: new Date().toISOString().slice(0, 10),
      remarks: `由 admin 于 ${new Date().toISOString().slice(0, 10)} 授权创建`
    };

    onAddUser(newUser);
    setShowAddModal(false);
    // Reset form
    setNewUsername("");
    setNewDisplayName("");
    setNewPassword("123456");
    setNewRole("operator");
    setNewSelectedProjects([]);
    setFormError("");
    setSaveFeedback(`用户【${newUser.displayName} (${newUser.username})】创建并授权成功！`);
    setTimeout(() => setSaveFeedback(null), 3500);
  }

  // Toggle single project in authorization list
  function toggleProjectSelection(projectName: string) {
    setSelectedProjects((prev) => {
      if (prev.includes(projectName)) {
        return prev.filter((p) => p !== projectName);
      } else {
        return [...prev, projectName];
      }
    });
  }

  // Select all / Deselect all projects
  function selectAllProjects() {
    setSelectedProjects(allProjects.map((p) => p.name));
  }
  function deselectAllProjects() {
    setSelectedProjects([]);
  }

  // Filtered projects for search inside auth modal
  const filteredProjectsForAuth = useMemo(() => {
    if (!projectSearch.trim()) return allProjects;
    const kw = projectSearch.toLowerCase();
    return allProjects.filter(
      (p) =>
        p.name.toLowerCase().includes(kw) ||
        p.customerName.toLowerCase().includes(kw) ||
        p.cloudVendor.toLowerCase().includes(kw)
    );
  }, [allProjects, projectSearch]);

  // If user is not admin, show permission denied guard
  if (!isAdmin) {
    return (
      <div className="cmdb-container" style={{ padding: 24 }}>
        <div style={{
          background: "#fff",
          border: "1px solid #fee2e2",
          borderRadius: 8,
          padding: 30,
          textAlign: "center",
          maxWidth: 600,
          margin: "40px auto"
        }}>
          <div style={{ fontSize: 44, marginBottom: 16 }}>🔒</div>
          <h3 style={{ color: "#991b1b", fontSize: 18, marginBottom: 10 }}>
            访问受限：需系统超级管理员 (admin) 授权
          </h3>
          <p style={{ color: "#64748b", fontSize: 13, lineHeight: 1.6, marginBottom: 20 }}>
            您当前登录的账号为 <strong>【{currentUser.displayName} ({currentUser.username})】</strong>，属于受限项目运维/审计角色。<br />
            按照平台安全规范，<strong>用户管理与项目权限授权由 admin 超级管理员统一配置</strong>。
          </p>
          <div style={{ display: "flex", justifyContent: "center", gap: 10 }}>
            <button
              type="button"
              className="btn-primary"
              onClick={() => {
                const adminUser = users.find((u) => u.username === "admin");
                if (adminUser) onSwitchUser(adminUser);
              }}
            >
              ⚡ 切换为系统管理员 (admin) 身份
            </button>
            {onPageChange && (
              <button
                type="button"
                className="btn-secondary"
                onClick={() => onPageChange("项目资产")}
              >
                返回项目资产工作台
              </button>
            )}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="cmdb-container">
      {/* Top Banner & Title */}
      <div style={{
        background: "#fff",
        border: "1px solid #e2e8f0",
        borderRadius: 8,
        padding: "14px 20px",
        marginBottom: 16,
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
        flexWrap: "wrap",
        gap: 12
      }}>
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <h3 style={{ margin: 0, fontSize: 16, color: "#0f172a", display: "flex", alignItems: "center", gap: 6 }}>
              👥 用户管理与项目权限控制
            </h3>
            <span style={{
              background: "#eff6ff",
              color: "#1d4ed8",
              fontSize: 11,
              fontWeight: 600,
              padding: "2px 8px",
              borderRadius: 4,
              border: "1px solid #bfdbfe"
            }}>
              admin 统一集中授权 · 项目范围精细化隔离
            </span>
          </div>
          <p style={{ margin: "5px 0 0", fontSize: 12, color: "#64748b" }}>
            管理运维平台用户账号，由 admin 统一授予各账号可查看的业务项目范围。授权后用户在各模块仅可查看指定项目资产。
          </p>
        </div>

        {/* Action Button: Create User */}
        <div style={{ display: "flex", gap: 8 }}>
          <button
            type="button"
            className="btn-primary"
            style={{ padding: "7px 16px", fontSize: 12, fontWeight: 600, display: "flex", alignItems: "center", gap: 6 }}
            onClick={() => {
              setNewUsername("");
              setNewDisplayName("");
              setNewPassword("123456");
              setNewRole("operator");
              setNewSelectedProjects([]);
              setFormError("");
              setShowAddModal(true);
            }}
          >
            <span>＋ 新建用户</span>
          </button>
        </div>
      </div>

      {/* Save Feedback Banner */}
      {saveFeedback && (
        <div style={{
          background: "#f0fdf4",
          border: "1px solid #bbf7d0",
          borderRadius: 6,
          padding: "8px 14px",
          color: "#15803d",
          fontSize: 12,
          fontWeight: 600,
          display: "flex",
          alignItems: "center",
          gap: 8,
          marginBottom: 14
        }}>
          <span>✓ {saveFeedback}</span>
        </div>
      )}

      {/* KPI Stats Summary Cards */}
      <div style={{
        display: "grid",
        gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))",
        gap: 12,
        marginBottom: 16
      }}>
        <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: "12px 16px" }}>
          <span style={{ fontSize: 11, color: "#64748b", display: "block", marginBottom: 2 }}>平台注册用户数</span>
          <strong style={{ fontSize: 20, color: "#0f172a" }}>{users.length}</strong>
          <span style={{ fontSize: 11, color: "#94a3b8", marginLeft: 4 }}>位人员</span>
        </div>

        <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: "12px 16px" }}>
          <span style={{ fontSize: 11, color: "#64748b", display: "block", marginBottom: 2 }}>超级管理员 (全部项目特权)</span>
          <strong style={{ fontSize: 20, color: "#2563eb" }}>
            {users.filter((u) => u.role === "admin" || u.authorizedProjects === "all").length}
          </strong>
          <span style={{ fontSize: 11, color: "#94a3b8", marginLeft: 4 }}>位 (全局无限制)</span>
        </div>

        <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: "12px 16px" }}>
          <span style={{ fontSize: 11, color: "#64748b", display: "block", marginBottom: 2 }}>指定受限授权人员</span>
          <strong style={{ fontSize: 20, color: "#16a34a" }}>
            {users.filter((u) => u.authorizedProjects !== "all").length}
          </strong>
          <span style={{ fontSize: 11, color: "#94a3b8", marginLeft: 4 }}>位 (精准隔离)</span>
        </div>

        <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, padding: "12px 16px" }}>
          <span style={{ fontSize: 11, color: "#64748b", display: "block", marginBottom: 2 }}>纳管业务项目总基线</span>
          <strong style={{ fontSize: 20, color: "#0f172a" }}>{allProjects.length}</strong>
          <span style={{ fontSize: 11, color: "#94a3b8", marginLeft: 4 }}>个项目</span>
        </div>
      </div>

      {/* Permission Explanation Tip Banner */}
      <div style={{
        background: "#eff6ff",
        border: "1px solid #dbeafe",
        borderRadius: 8,
        padding: "10px 16px",
        marginBottom: 16,
        fontSize: 12,
        color: "#1e40af",
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        flexWrap: "wrap",
        gap: 8
      }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span>💡</span>
          <span>
            <strong>权限隔离运作机制：</strong>系统管理员 <code>admin</code> 可随时点击操作列的 <strong>【🔑 权限授权】</strong> 勾选项目。被授权用户登录后，其可见的「项目资产」、「资产大盘」、「业务拓扑」、「漏洞检测」和「凭据管理」将严格收敛至授权范围，杜绝越权访问。
          </span>
        </div>
        <span style={{ fontSize: 11, color: "#3b82f6", fontWeight: 600 }}>
          当前登录: {currentUser.displayName} (admin 最高权限)
        </span>
      </div>

      {/* Search & Filter Bar */}
      <div style={{
        background: "#fff",
        border: "1px solid #e2e8f0",
        borderRadius: 8,
        padding: "10px 16px",
        marginBottom: 12,
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
        flexWrap: "wrap",
        gap: 10
      }}>
        <div style={{ display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap" }}>
          <input
            type="text"
            placeholder="搜索用户名、姓名、部门..."
            value={filterKeyword}
            onChange={(e) => setFilterKeyword(e.target.value)}
            style={{
              padding: "5px 10px",
              fontSize: 12,
              border: "1px solid #cbd5e1",
              borderRadius: 6,
              width: 220
            }}
          />

          <select
            value={filterRole}
            onChange={(e) => setFilterRole(e.target.value)}
            style={{ padding: "5px 10px", fontSize: 12, border: "1px solid #cbd5e1", borderRadius: 6 }}
          >
            <option value="全部">全部角色</option>
            <option value="admin">超级管理员</option>
            <option value="operator">项目运维专员</option>
            <option value="auditor">安全审计员</option>
          </select>
        </div>

        <span style={{ fontSize: 12, color: "#64748b" }}>
          共匹配 <strong>{filteredUsers.length}</strong> 个用户账号
        </span>
      </div>

      {/* Users Data Table */}
      <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, overflow: "hidden" }}>
        <div style={{ overflowX: "auto" }}>
          <table className="cmdb-data-table" style={{ minWidth: 960 }}>
            <thead>
              <tr>
                <th style={{ width: 160 }}>用户信息</th>
                <th style={{ width: 110 }}>系统角色</th>
                <th style={{ width: 140 }}>所属部门 / 岗位</th>
                <th style={{ width: 130 }}>联系方式</th>
                <th style={{ width: 280 }}>授权可见项目范围 (Admin 授权)</th>
                <th style={{ width: 90 }}>状态</th>
                <th style={{ width: 190 }}>操作</th>
              </tr>
            </thead>
            <tbody>
              {filteredUsers.map((user) => {
                const isUserAdmin = user.role === "admin";
                const isAllProjects = user.authorizedProjects === "all";
                const isCurrentUser = currentUser.username === user.username;

                return (
                  <tr key={user.id} style={{ background: isCurrentUser ? "#f8fafc" : "#fff" }}>
                    {/* User Info */}
                    <td>
                      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                        <div style={{
                          width: 28,
                          height: 28,
                          borderRadius: "50%",
                          background: isUserAdmin ? "#eff6ff" : "#f1f5f9",
                          border: isUserAdmin ? "1.5px solid #3b82f6" : "1px solid #cbd5e1",
                          color: isUserAdmin ? "#1d4ed8" : "#475569",
                          display: "flex",
                          alignItems: "center",
                          justifyContent: "center",
                          fontSize: 12,
                          fontWeight: 700
                        }}>
                          {user.displayName.slice(0, 1)}
                        </div>
                        <div>
                          <div style={{ fontWeight: 700, color: "#0f172a", fontSize: 12 }}>
                            {user.displayName}
                            {isCurrentUser && (
                              <span style={{
                                marginLeft: 4,
                                fontSize: 9,
                                background: "#dcfce7",
                                color: "#15803d",
                                padding: "1px 4px",
                                borderRadius: 3
                              }}>
                                当前
                              </span>
                            )}
                          </div>
                          <div style={{ fontSize: 11, color: "#64748b", fontFamily: "monospace" }}>
                            @{user.username}
                          </div>
                        </div>
                      </div>
                    </td>

                    {/* Role */}
                    <td>
                      <span style={{
                        fontSize: 10,
                        fontWeight: 600,
                        padding: "2px 6px",
                        borderRadius: 3,
                        background:
                          user.role === "admin" ? "#eff6ff" :
                          user.role === "operator" ? "#f0fdf4" : "#faf5ff",
                        color:
                          user.role === "admin" ? "#1d4ed8" :
                          user.role === "operator" ? "#15803d" : "#7e22ce",
                        border:
                          user.role === "admin" ? "1px solid #bfdbfe" :
                          user.role === "operator" ? "1px solid #bbf7d0" : "1px solid #e9d5ff"
                      }}>
                        {user.role === "admin" ? "超级管理员" : user.role === "operator" ? "项目运维人员" : "安全审计员"}
                      </span>
                    </td>

                    {/* Department */}
                    <td style={{ fontSize: 12, color: "#334155" }}>
                      {user.department}
                    </td>

                    {/* Contact */}
                    <td style={{ fontSize: 11, color: "#64748b" }}>
                      <div>{user.phone || "--"}</div>
                      <small style={{ color: "#94a3b8" }}>{user.email || "--"}</small>
                    </td>

                    {/* Project Permissions */}
                    <td>
                      {isAllProjects ? (
                        <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                          <span style={{
                            background: "#eff6ff",
                            color: "#1d4ed8",
                            fontSize: 11,
                            fontWeight: 700,
                            padding: "2px 8px",
                            borderRadius: 4,
                            border: "1px solid #bfdbfe"
                          }}>
                            ★ 全部项目 ({allProjects.length} 个) 全局可见
                          </span>
                        </div>
                      ) : (
                        <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                          <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                            <span style={{
                              background: "#f0fdf4",
                              color: "#15803d",
                              fontSize: 11,
                              fontWeight: 700,
                              padding: "1px 6px",
                              borderRadius: 4,
                              border: "1px solid #bbf7d0"
                            }}>
                              已授权 {user.authorizedProjects.length} 个项目
                            </span>
                          </div>
                          <div style={{ display: "flex", flexWrap: "wrap", gap: 4 }}>
                            {user.authorizedProjects.slice(0, 3).map((prj) => (
                              <span
                                key={prj}
                                style={{
                                  fontSize: 10,
                                  background: "#f8fafc",
                                  color: "#334155",
                                  border: "1px solid #e2e8f0",
                                  padding: "1px 5px",
                                  borderRadius: 3,
                                  maxWidth: 160,
                                  overflow: "hidden",
                                  textOverflow: "ellipsis",
                                  whiteSpace: "nowrap"
                                }}
                                title={prj}
                              >
                                {prj}
                              </span>
                            ))}
                            {user.authorizedProjects.length > 3 && (
                              <span style={{ fontSize: 10, color: "#94a3b8", alignSelf: "center" }}>
                                +{user.authorizedProjects.length - 3} 个
                              </span>
                            )}
                          </div>
                        </div>
                      )}
                    </td>

                    {/* Status */}
                    <td>
                      <span style={{
                        fontSize: 10,
                        fontWeight: 600,
                        padding: "1px 6px",
                        borderRadius: 3,
                        background: user.status === "active" ? "#f0fdf4" : "#fef2f2",
                        color: user.status === "active" ? "#16a34a" : "#dc2626"
                      }}>
                        {user.status === "active" ? "正常" : "禁用"}
                      </span>
                    </td>

                    {/* Actions */}
                    <td>
                      <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
                        {/* Authorize Projects Button */}
                        <button
                          type="button"
                          className="btn-primary"
                          style={{
                            padding: "2px 8px",
                            fontSize: 11,
                            fontWeight: 600,
                            background: "#2563eb",
                            borderColor: "#2563eb"
                          }}
                          onClick={() => handleOpenAuthModal(user)}
                          title="配置该用户可见的项目范围"
                        >
                          🔑 权限授权
                        </button>

                        {/* Switch Identity / Simulate User */}
                        <button
                          type="button"
                          className="btn-secondary"
                          style={{
                            padding: "2px 8px",
                            fontSize: 11,
                            color: isCurrentUser ? "#94a3b8" : "#0284c7"
                          }}
                          disabled={isCurrentUser}
                          onClick={() => {
                            onSwitchUser(user);
                            alert(`已快速切换为【${user.displayName}】视角！\n当前身份可查看的项目数为：${user.authorizedProjects === "all" ? "全部24个项目" : user.authorizedProjects.length + "个授权项目"}。`);
                          }}
                          title="以此用户视角体验项目隔离"
                        >
                          👥 模拟视角
                        </button>

                        {/* Delete (admin cannot be deleted) */}
                        {user.username !== "admin" && (
                          <button
                            type="button"
                            className="btn-secondary"
                            style={{ padding: "2px 6px", fontSize: 11, color: "#ef4444" }}
                            onClick={() => {
                              if (confirm(`确定要移除用户【${user.displayName} (${user.username})】吗？`)) {
                                onDeleteUser(user.id);
                              }
                            }}
                          >
                            删除
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>

      {/* ================= 1. PROJECT PERMISSION AUTHORIZATION MODAL ================= */}
      {showAuthModal && targetAuthUser && (
        <div className="cmdb-modal-mask">
          <form className="cmdb-modal" onSubmit={handleSaveAuth} style={{ maxWidth: 720, maxHeight: "90vh", display: "flex", flexDirection: "column" }}>
            {/* Modal Header */}
            <div className="cmdb-modal-header" style={{ background: "#eff6ff", borderBottom: "1px solid #bfdbfe" }}>
              <div>
                <h3 style={{ margin: 0, color: "#1d4ed8", fontSize: 15, display: "flex", alignItems: "center", gap: 6 }}>
                  🔑 业务项目访问权限授权
                </h3>
                <div style={{ fontSize: 11, color: "#475569", marginTop: 2 }}>
                  目标用户: <strong>{targetAuthUser.displayName}</strong> (账号: <code>{targetAuthUser.username}</code> · {targetAuthUser.department})
                </div>
              </div>
              <button type="button" className="cmdb-modal-close" onClick={() => setShowAuthModal(false)}>×</button>
            </div>

            {/* Modal Body */}
            <div className="cmdb-modal-body" style={{ overflowY: "auto", padding: "16px 20px" }}>
              {/* Permission Mode Radio Buttons */}
              <div style={{
                background: "#f8fafc",
                border: "1px solid #e2e8f0",
                borderRadius: 8,
                padding: "12px 16px",
                marginBottom: 16
              }}>
                <label style={{ display: "block", fontSize: 12, fontWeight: 700, color: "#1e293b", marginBottom: 8 }}>
                  授权模式选择:
                </label>
                <div style={{ display: "flex", gap: 20 }}>
                  <label style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 12, cursor: "pointer", fontWeight: 600 }}>
                    <input
                      type="radio"
                      name="authMode"
                      checked={authMode === "all"}
                      onChange={() => setAuthMode("all")}
                    />
                    <span>★ 全部项目无限制访问 (超级特权，全网 {allProjects.length} 个项目)</span>
                  </label>

                  <label style={{ display: "flex", alignItems: "center", gap: 6, fontSize: 12, cursor: "pointer", fontWeight: 600 }}>
                    <input
                      type="radio"
                      name="authMode"
                      checked={authMode === "custom"}
                      onChange={() => setAuthMode("custom")}
                    />
                    <span>🎯 自定义指定授权项目 (精细化隔离勾选)</span>
                  </label>
                </div>
              </div>

              {/* Custom Projects Checklist */}
              {authMode === "custom" && (
                <div>
                  {/* Toolbar inside checklist */}
                  <div style={{
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "center",
                    flexWrap: "wrap",
                    gap: 10,
                    marginBottom: 10
                  }}>
                    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                      <input
                        type="text"
                        placeholder="检索项目名称、客户单位、云商..."
                        value={projectSearch}
                        onChange={(e) => setProjectSearch(e.target.value)}
                        style={{
                          padding: "5px 10px",
                          fontSize: 12,
                          border: "1px solid #cbd5e1",
                          borderRadius: 6,
                          width: 240
                        }}
                      />
                      <button
                        type="button"
                        className="btn-secondary"
                        style={{ padding: "4px 8px", fontSize: 11 }}
                        onClick={selectAllProjects}
                      >
                        全选
                      </button>
                      <button
                        type="button"
                        className="btn-secondary"
                        style={{ padding: "4px 8px", fontSize: 11 }}
                        onClick={deselectAllProjects}
                      >
                        清空
                      </button>
                    </div>

                    <span style={{ fontSize: 12, color: "#2563eb", fontWeight: 700 }}>
                      当前已勾选: {selectedProjects.length} / {allProjects.length} 个项目
                    </span>
                  </div>

                  {/* Project Cards Grid */}
                  <div style={{
                    display: "grid",
                    gridTemplateColumns: "repeat(auto-fill, minmax(280px, 1fr))",
                    gap: 8,
                    maxHeight: 340,
                    overflowY: "auto",
                    padding: "4px 2px"
                  }}>
                    {filteredProjectsForAuth.map((p) => {
                      const isChecked = selectedProjects.includes(p.name);
                      return (
                        <div
                          key={p.id}
                          onClick={() => toggleProjectSelection(p.name)}
                          style={{
                            display: "flex",
                            alignItems: "flex-start",
                            gap: 10,
                            padding: "8px 10px",
                            background: isChecked ? "#eff6ff" : "#fff",
                            border: isChecked ? "1.5px solid #2563eb" : "1px solid #e2e8f0",
                            borderRadius: 6,
                            cursor: "pointer",
                            transition: "all 0.15s"
                          }}
                        >
                          <input
                            type="checkbox"
                            checked={isChecked}
                            onChange={() => {}} // Handled by parent div
                            style={{ marginTop: 2, cursor: "pointer" }}
                          />
                          <div style={{ flex: 1, minWidth: 0 }}>
                            <div style={{
                              fontSize: 12,
                              fontWeight: 700,
                              color: isChecked ? "#1d4ed8" : "#0f172a",
                              whiteSpace: "nowrap",
                              overflow: "hidden",
                              textOverflow: "ellipsis"
                            }}>
                              {p.name}
                            </div>
                            <div style={{ display: "flex", gap: 6, marginTop: 3, fontSize: 10 }}>
                              <span style={{ color: "#64748b" }}>{p.customerName.slice(0, 10)}</span>
                              <span style={{ color: "#94a3b8" }}>·</span>
                              <span style={{ color: "#2563eb" }}>{p.cloudVendor}</span>
                              <span style={{ color: "#94a3b8" }}>·</span>
                              <span style={{ color: "#16a34a", fontWeight: 600 }}>{p.deviceCount} 台资产</span>
                            </div>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}

              {authMode === "all" && (
                <div style={{
                  background: "#f0fdf4",
                  border: "1px solid #bbf7d0",
                  borderRadius: 6,
                  padding: 14,
                  textAlign: "center",
                  color: "#15803d",
                  fontSize: 12
                }}>
                  <strong>全局项目授权已激活：</strong>用户【{targetAuthUser.displayName}】拥有全部 {allProjects.length} 个业务项目及其全部 292 台资产的完全查看与操作权限。
                </div>
              )}
            </div>

            {/* Modal Footer */}
            <div className="cmdb-modal-footer" style={{ borderTop: "1px solid #e2e8f0" }}>
              <button type="button" className="btn-secondary" onClick={() => setShowAuthModal(false)}>
                取 消
              </button>
              <button type="submit" className="btn-primary" style={{ minWidth: 100 }}>
                💾 保存项目授权
              </button>
            </div>
          </form>
        </div>
      )}

      {/* ================= 2. CREATE NEW USER MODAL ================= */}
      {showAddModal && (
        <div className="cmdb-modal-mask">
          <form className="cmdb-modal" onSubmit={handleCreateUser} style={{ maxWidth: 580 }}>
            <div className="cmdb-modal-header">
              <h3>👥 新建平台用户账号并配置授权</h3>
              <button type="button" className="cmdb-modal-close" onClick={() => setShowAddModal(false)}>×</button>
            </div>

            <div className="cmdb-modal-body" style={{ padding: "16px 20px" }}>
              {formError && (
                <div style={{
                  background: "#fef2f2",
                  color: "#dc2626",
                  border: "1px solid #fee2e2",
                  borderRadius: 6,
                  padding: "6px 12px",
                  fontSize: 12,
                  marginBottom: 12
                }}>
                  ✕ {formError}
                </div>
              )}

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 登录账号 (Username)</label>
                  <input
                    placeholder="如: ops_wang, dev_user"
                    value={newUsername}
                    onChange={(e) => setNewUsername(e.target.value)}
                    required
                  />
                </div>
                <div className="form-field-item">
                  <label>* 真实姓名 (Display Name)</label>
                  <input
                    placeholder="如: 王工程师"
                    value={newDisplayName}
                    onChange={(e) => setNewDisplayName(e.target.value)}
                    required
                  />
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>* 登录密码</label>
                  <input
                    type="password"
                    placeholder="初始密码 (默认 123456)"
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    required
                  />
                </div>
                <div className="form-field-item">
                  <label>系统角色</label>
                  <select
                    value={newRole}
                    onChange={(e) => setNewRole(e.target.value as any)}
                  >
                    <option value="operator">项目运维专员 (指定项目权限)</option>
                    <option value="auditor">安全审计员 (只读审计)</option>
                    <option value="admin">系统超级管理员 (全部项目特权)</option>
                  </select>
                </div>
              </div>

              <div className="form-field-row">
                <div className="form-field-item">
                  <label>所属部门 / 岗位</label>
                  <input
                    placeholder="如: 政务专网运维组"
                    value={newDepartment}
                    onChange={(e) => setNewDepartment(e.target.value)}
                  />
                </div>
                <div className="form-field-item">
                  <label>联系电话</label>
                  <input
                    placeholder="如: 13800000000"
                    value={newPhone}
                    onChange={(e) => setNewPhone(e.target.value)}
                  />
                </div>
              </div>

              {/* Initial Project Permissions */}
              <div className="form-field-item" style={{ marginTop: 10 }}>
                <label style={{ fontWeight: 700, color: "#1e293b", marginBottom: 6 }}>
                  * 初始业务项目访问授权:
                </label>
                <div style={{ display: "flex", gap: 16, marginBottom: 8, fontSize: 12 }}>
                  <label style={{ display: "flex", alignItems: "center", gap: 6, cursor: "pointer" }}>
                    <input
                      type="radio"
                      name="newAuthMode"
                      checked={newAuthMode === "all"}
                      onChange={() => setNewAuthMode("all")}
                    />
                    <span>全部 24 个项目无限制可见</span>
                  </label>
                  <label style={{ display: "flex", alignItems: "center", gap: 6, cursor: "pointer" }}>
                    <input
                      type="radio"
                      name="newAuthMode"
                      checked={newAuthMode === "custom"}
                      onChange={() => setNewAuthMode("custom")}
                    />
                    <span>自定义指定授权项目</span>
                  </label>
                </div>

                {newAuthMode === "custom" && (
                  <div style={{
                    maxHeight: 180,
                    overflowY: "auto",
                    border: "1px solid #cbd5e1",
                    borderRadius: 6,
                    padding: 8,
                    background: "#f8fafc"
                  }}>
                    {allProjects.map((p) => {
                      const checked = newSelectedProjects.includes(p.name);
                      return (
                        <label
                          key={p.id}
                          style={{
                            display: "flex",
                            alignItems: "center",
                            gap: 8,
                            padding: "4px 6px",
                            fontSize: 11,
                            cursor: "pointer",
                            borderBottom: "1px solid #e2e8f0"
                          }}
                        >
                          <input
                            type="checkbox"
                            checked={checked}
                            onChange={() => {
                              setNewSelectedProjects((prev) =>
                                checked ? prev.filter((x) => x !== p.name) : [...prev, p.name]
                              );
                            }}
                          />
                          <span style={{ fontWeight: checked ? 700 : 400, color: checked ? "#1d4ed8" : "#334155" }}>
                            {p.name}
                          </span>
                          <span style={{ fontSize: 10, color: "#94a3b8", marginLeft: "auto" }}>
                            {p.deviceCount} 台
                          </span>
                        </label>
                      );
                    })}
                  </div>
                )}
              </div>
            </div>

            <div className="cmdb-modal-footer">
              <button type="button" className="btn-secondary" onClick={() => setShowAddModal(false)}>
                取 消
              </button>
              <button type="submit" className="btn-primary">
                确 定 创 建 用 户
              </button>
            </div>
          </form>
        </div>
      )}
    </div>
  );
}
