"use client";
import React, { useState, useMemo } from "react";
import { 
  CredentialItem, 
  ProjectGroup, 
  PhysicalHost, 
  VmHost, 
  SoftwareComponent, 
  DatabaseAsset 
} from "../cmdbData";

export interface AssetCredentialRecord {
  id: string;
  projectId: string;
  projectName: string;
  assetName: string;
  ip: string;
  assetType: "主机设备" | "系统软件" | "应用服务" | "数据库服务";
  serviceName: string;
  port: string | number;
  account: string;
  encryptedCipher: string;
  rawSecret: string;
  authMethod: "密码认证" | "SSH 私钥" | "数据库账号" | "BMC IPMI";
  environment: string;
  updatedAt: string;
  remarks: string;
}

interface CredentialManagementProps {
  credentials: CredentialItem[];
  onAddCredential: (c: CredentialItem) => void;
  onDeleteCredential: (id: string) => void;
  projects?: ProjectGroup[];
  hosts?: PhysicalHost[];
  vms?: VmHost[];
  softwareList?: SoftwareComponent[];
  databases?: DatabaseAsset[];
}

// Simple hash generator for deterministic mock credentials
function pseudoHash(str: string): number {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = (hash << 5) - hash + str.charCodeAt(i);
    hash |= 0;
  }
  return Math.abs(hash);
}

// Generate realistic credentials bound to actual assets
function generateAssetCredentials(
  projects: ProjectGroup[] = [],
  hosts: PhysicalHost[] = [],
  vms: VmHost[] = [],
  softwareList: SoftwareComponent[] = [],
  databases: DatabaseAsset[] = []
): AssetCredentialRecord[] {
  const records: AssetCredentialRecord[] = [];

  // 1. Host / VM Credentials
  vms.forEach((vm) => {
    const pHash = pseudoHash(vm.id);
    const isWindows = (vm.osFamily || vm.os || "").toLowerCase().includes("win");
    const account = isWindows ? "Administrator" : "root";
    const ip = vm.privateIp || vm.ip || vm.publicIp || "10.0.0.1";
    const port = vm.remotePort || (isWindows ? 3389 : 22);
    
    // Deterministic secret
    const rawSecret = isWindows 
      ? `Win@${vm.projectName ? vm.projectName.slice(0, 2) : "Ops"}#${(pHash % 8999) + 1000}!`
      : `Root_${(pHash % 899) + 100}$Sec@${ip.split(".").pop() || "2026"}`;
    
    // Masked cipher
    const hexCipher = `AES256-GCM:${pHash.toString(16).padStart(8, "0")}${(pHash * 7).toString(16).slice(0, 8)}`;

    records.push({
      id: `cred-host-${vm.id}`,
      projectId: vm.projectId || "prj-unknown",
      projectName: vm.projectName || "未归属项目",
      assetName: vm.name || `节点-${ip}`,
      ip: ip,
      assetType: "主机设备",
      serviceName: `${vm.osFamily || "Linux"} ${vm.osVersion || ""} (操作系统)`,
      port: port,
      account: account,
      encryptedCipher: hexCipher,
      rawSecret: rawSecret,
      authMethod: "密码认证",
      environment: vm.env || "生产",
      updatedAt: "2026-09-18",
      remarks: `主机OS最高管理员凭证，绑定IP: ${ip}`
    });
  });

  // 2. Physical Hosts
  hosts.forEach((h) => {
    const pHash = pseudoHash(h.id);
    const ip = h.ip || "192.168.1.1";
    records.push({
      id: `cred-phy-${h.id}`,
      projectId: h.projectId || "prj-infra",
      projectName: h.projectName || "基础设施硬件",
      assetName: h.name || h.hostname || "物理宿主机",
      ip: ip,
      assetType: "主机设备",
      serviceName: `${h.brand} ${h.model} (实体物理机)`,
      port: 22,
      account: "root",
      encryptedCipher: `AES256-GCM:${pHash.toString(16).padStart(8, "0")}phy`,
      rawSecret: `PhyRoot#${(pHash % 899) + 100}@Host!`,
      authMethod: "密码认证",
      environment: h.env || "生产",
      updatedAt: "2026-09-18",
      remarks: `物理宿主机底层带内登录账号`
    });

    if (h.bmcIp) {
      records.push({
        id: `cred-bmc-${h.id}`,
        projectId: h.projectId || "prj-infra",
        projectName: h.projectName || "基础设施硬件",
        assetName: `${h.name} (BMC)`,
        ip: h.bmcIp,
        assetType: "主机设备",
        serviceName: `BMC iLO/iDRAC/iBMC 带外硬件`,
        port: 443,
        account: "Administrator",
        encryptedCipher: `AES256-GCM:${pHash.toString(16).padStart(8, "0")}bmc`,
        rawSecret: `Bmc@Admin_${(pHash % 899) + 100}$HW`,
        authMethod: "BMC IPMI",
        environment: h.env || "生产",
        updatedAt: "2026-09-18",
        remarks: `物理机带外远程控制台专属密码`
      });
    }
  });

  // 3. System Software & Middlewares
  softwareList.forEach((sw) => {
    const pHash = pseudoHash(sw.id);
    const ip = sw.assetIp || "10.150.0.1";
    let account = "admin";
    let port: string | number = sw.port || "8080";
    let secretPrefix = "App";

    if (sw.category === "database") {
      account = sw.name.toLowerCase().includes("redis") ? "default" : "root";
      port = sw.port || (sw.name.toLowerCase().includes("redis") ? 6379 : 3306);
      secretPrefix = "Db";
    } else if (sw.category === "middleware") {
      account = "ops_admin";
      secretPrefix = "Mid";
    } else if (sw.category === "web_server") {
      account = "nginx";
      port = sw.port || "80, 443";
      secretPrefix = "Web";
    }

    records.push({
      id: `cred-sw-${sw.id}`,
      projectId: sw.projectId,
      projectName: sw.projectName,
      assetName: sw.assetName || `软件节点-${ip}`,
      ip: ip,
      assetType: sw.category === "database" ? "数据库服务" : "系统软件",
      serviceName: `${sw.name} ${sw.version}`,
      port: port,
      account: account,
      encryptedCipher: `AES256-GCM:${pHash.toString(16).padStart(8, "0")}sw`,
      rawSecret: `${secretPrefix}@${(pHash % 899) + 100}$${sw.name.replace(/[^a-zA-Z0-9]/g, "").slice(0, 4)}!2026`,
      authMethod: sw.category === "database" ? "数据库账号" : "密码认证",
      environment: "生产",
      updatedAt: "2026-09-18",
      remarks: `服务安装路径: ${sw.installPath || "/usr/local"}`
    });
  });

  // 4. Databases
  databases.forEach((db) => {
    const pHash = pseudoHash(db.id);
    const ip = db.hostIp || "192.125.31.250";
    records.push({
      id: `cred-db-${db.id}`,
      projectId: db.businessId || "prj-001",
      projectName: "调解仲裁系统 (北京市人力资源和社会保障局)",
      assetName: `数据库节点-${db.name}`,
      ip: ip,
      assetType: "数据库服务",
      serviceName: `${db.type} ${db.version} (${db.arch})`,
      port: db.port,
      account: db.type === "Oracle" ? "SYS / SYSTEM" : "root",
      encryptedCipher: `AES256-GCM:${pHash.toString(16).padStart(8, "0")}db`,
      rawSecret: `Db#Cluster_${(pHash % 899) + 100}@${db.type}!`,
      authMethod: "数据库账号",
      environment: "生产",
      updatedAt: "2026-09-18",
      remarks: `核心生产数据库，高敏感集群`
    });
  });

  return records;
}

export default function CredentialManagement({
  credentials,
  onAddCredential,
  onDeleteCredential,
  projects = [],
  hosts = [],
  vms = [],
  softwareList = [],
  databases = []
}: CredentialManagementProps) {
  // Navigation tabs: 1. Asset Linked Credentials Query, 2. Generic Access Pools
  const [activeTab, setActiveTab] = useState<"assetQuery" | "genericPool">("assetQuery");

  // Filter conditions for Asset Credentials
  const [searchIp, setSearchIp] = useState("");
  const [searchProject, setSearchProject] = useState("全部");
  const [searchType, setSearchType] = useState("全部");
  const [searchKeyword, setSearchKeyword] = useState("");
  
  // Applied search state (triggered when clicking "查询" button)
  const [appliedFilters, setAppliedFilters] = useState({
    ip: "",
    project: "全部",
    type: "全部",
    keyword: ""
  });

  // Pagination for Asset Credentials
  const [currentPage, setCurrentPage] = useState(1);
  const pageSize = 15;

  // Security Auth State (Decryption Passphrase)
  const [authModalVisible, setAuthModalVisible] = useState(false);
  const [passphraseInput, setPassphraseInput] = useState("");
  const [authError, setAuthError] = useState("");
  const [targetRecordToUnlock, setTargetRecordToUnlock] = useState<AssetCredentialRecord | null>(null);
  const [unlockedRecordIds, setUnlockedRecordIds] = useState<Set<string>>(new Set());
  const [copyFeedback, setCopyFeedback] = useState<string | null>(null);

  // Original Add Credential Modal
  const [showModal, setShowModal] = useState(false);
  const [form, setForm] = useState({
    name: "",
    type: "SSH 私钥" as CredentialItem["type"],
    username: "ops_root",
    targetCount: 6,
    remark: ""
  });

  // Generate asset credential records
  const allAssetCredentials = useMemo(() => {
    return generateAssetCredentials(projects, hosts, vms, softwareList, databases);
  }, [projects, hosts, vms, softwareList, databases]);

  // Handle Query Trigger
  function handleExecuteQuery() {
    setAppliedFilters({
      ip: searchIp.trim(),
      project: searchProject,
      type: searchType,
      keyword: searchKeyword.trim()
    });
    setCurrentPage(1);
  }

  // Handle Reset Filters
  function handleResetQuery() {
    setSearchIp("");
    setSearchProject("全部");
    setSearchType("全部");
    setSearchKeyword("");
    setAppliedFilters({
      ip: "",
      project: "全部",
      type: "全部",
      keyword: ""
    });
    setCurrentPage(1);
  }

  // Filtered asset credentials
  const filteredAssetRecords = useMemo(() => {
    return allAssetCredentials.filter((item) => {
      // IP filter
      if (appliedFilters.ip) {
        if (!item.ip.toLowerCase().includes(appliedFilters.ip.toLowerCase())) {
          return false;
        }
      }

      // Project filter
      if (appliedFilters.project !== "全部") {
        if (item.projectName !== appliedFilters.project) {
          return false;
        }
      }

      // Asset Type filter
      if (appliedFilters.type !== "全部") {
        if (item.assetType !== appliedFilters.type) {
          return false;
        }
      }

      // Keyword filter (service name, asset name, account)
      if (appliedFilters.keyword) {
        const kw = appliedFilters.keyword.toLowerCase();
        const match = 
          item.serviceName.toLowerCase().includes(kw) ||
          item.assetName.toLowerCase().includes(kw) ||
          item.account.toLowerCase().includes(kw) ||
          item.remarks.toLowerCase().includes(kw);
        if (!match) return false;
      }

      return true;
    });
  }, [allAssetCredentials, appliedFilters]);

  // Paginated records
  const totalPages = Math.ceil(filteredAssetRecords.length / pageSize) || 1;
  const paginatedRecords = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return filteredAssetRecords.slice(start, start + pageSize);
  }, [filteredAssetRecords, currentPage]);

  // Open decrypt modal for a specific record
  function handleRequestUnlock(record: AssetCredentialRecord) {
    // If already unlocked, toggle lock
    if (unlockedRecordIds.has(record.id)) {
      setUnlockedRecordIds(prev => {
        const next = new Set(prev);
        next.delete(record.id);
        return next;
      });
      return;
    }

    setTargetRecordToUnlock(record);
    setPassphraseInput("");
    setAuthError("");
    setAuthModalVisible(true);
  }

  // Validate Passphrase
  function handleVerifyPassphrase(e: React.FormEvent) {
    e.preventDefault();
    // Valid system passphrases
    const validPassphrases = ["admin123", "ops@2026", "autops2026", "123456", "admin"];
    const input = passphraseInput.trim().toLowerCase();

    if (!validPassphrases.includes(input)) {
      setAuthError("❌ 口令错误！安全审计拦截，请使用管理员口令 (提示: admin123 或 Ops@2026)");
      return;
    }

    // Success: Unlock the target record
    if (targetRecordToUnlock) {
      setUnlockedRecordIds(prev => new Set(prev).add(targetRecordToUnlock.id));
    }
    setAuthModalVisible(false);
    setPassphraseInput("");
    setAuthError("");
  }

  // Copy password to clipboard
  function handleCopyPassword(secret: string, recordId: string) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(secret).then(() => {
        setCopyFeedback(recordId);
        setTimeout(() => setCopyFeedback(null), 2000);
      });
    } else {
      // Fallback
      alert(`已获取密码明文: ${secret}`);
    }
  }

  function handleAddSubmit(e: React.FormEvent) {
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
      {/* Top Header & Tab Navigation */}
      <div style={{
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
        flexWrap: "wrap",
        gap: 12,
        marginBottom: 16,
        background: "#fff",
        border: "1px solid #e2e8f0",
        borderRadius: 8,
        padding: "12px 18px"
      }}>
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <h3 style={{ margin: 0, fontSize: 16, color: "#0f172a", display: "flex", alignItems: "center", gap: 6 }}>
              🔐 运维资产凭证与敏感密码保险库
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
              AES-256 加密存管 · 口令授权可见
            </span>
          </div>
          <p style={{ margin: "4px 0 0", fontSize: 12, color: "#64748b" }}>
            集中纳管全网各项目主机、系统软件及应用服务/数据库口令密码。数据均经高强度加密存管，审计可追溯。
          </p>
        </div>

        {/* Tab Switcher */}
        <div style={{ display: "flex", gap: 6 }}>
          <button
            type="button"
            className={activeTab === "assetQuery" ? "btn-primary" : "btn-secondary"}
            style={{ fontSize: 12, padding: "6px 14px", display: "flex", alignItems: "center", gap: 6 }}
            onClick={() => setActiveTab("assetQuery")}
          >
            <span>🔍 项目资产凭据查询</span>
            <span style={{
              background: activeTab === "assetQuery" ? "rgba(255,255,255,0.25)" : "#e2e8f0",
              color: activeTab === "assetQuery" ? "#fff" : "#475569",
              padding: "1px 6px",
              borderRadius: 10,
              fontSize: 10
            }}>
              {filteredAssetRecords.length}
            </span>
          </button>

          <button
            type="button"
            className={activeTab === "genericPool" ? "btn-primary" : "btn-secondary"}
            style={{ fontSize: 12, padding: "6px 14px", display: "flex", alignItems: "center", gap: 6 }}
            onClick={() => setActiveTab("genericPool")}
          >
            <span>🏷️ 基础访问凭据池</span>
            <span style={{
              background: activeTab === "genericPool" ? "rgba(255,255,255,0.25)" : "#e2e8f0",
              color: activeTab === "genericPool" ? "#fff" : "#475569",
              padding: "1px 6px",
              borderRadius: 10,
              fontSize: 10
            }}>
              {credentials.length}
            </span>
          </button>
        </div>
      </div>

      {/* ================= VIEW 1: ASSET CREDENTIALS QUERY ================= */}
      {activeTab === "assetQuery" && (
        <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
          {/* Multi-Condition Query Form */}
          <div style={{
            background: "#fff",
            border: "1px solid #e2e8f0",
            borderRadius: 8,
            padding: "14px 18px",
            boxShadow: "0 1px 3px rgba(0,0,0,0.03)"
          }}>
            <div style={{
              display: "grid",
              gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))",
              gap: 12,
              alignItems: "flex-end"
            }}>
              {/* Query 1: IP Address */}
              <div>
                <label style={{ display: "block", fontSize: 12, fontWeight: 600, color: "#334155", marginBottom: 5 }}>
                  🎯 目标 IP 地址
                </label>
                <input
                  type="text"
                  placeholder="如: 192.125.31.250 或 10.150..."
                  value={searchIp}
                  onChange={(e) => setSearchIp(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && handleExecuteQuery()}
                  style={{
                    width: "100%",
                    padding: "7px 10px",
                    fontSize: 12,
                    border: "1px solid #cbd5e1",
                    borderRadius: 6,
                    outline: "none"
                  }}
                />
              </div>

              {/* Query 2: Project Name */}
              <div>
                <label style={{ display: "block", fontSize: 12, fontWeight: 600, color: "#334155", marginBottom: 5 }}>
                  📁 所属项目名称
                </label>
                <select
                  value={searchProject}
                  onChange={(e) => setSearchProject(e.target.value)}
                  style={{
                    width: "100%",
                    padding: "7px 10px",
                    fontSize: 12,
                    border: "1px solid #cbd5e1",
                    borderRadius: 6,
                    background: "#fff"
                  }}
                >
                  <option value="全部">全部项目 ({projects.length} 个)</option>
                  {projects.map((p) => (
                    <option key={p.id} value={p.name}>
                      {p.name}
                    </option>
                  ))}
                </select>
              </div>

              {/* Query 3: Asset Category */}
              <div>
                <label style={{ display: "block", fontSize: 12, fontWeight: 600, color: "#334155", marginBottom: 5 }}>
                  🧩 资产类型与层级
                </label>
                <select
                  value={searchType}
                  onChange={(e) => setSearchType(e.target.value)}
                  style={{
                    width: "100%",
                    padding: "7px 10px",
                    fontSize: 12,
                    border: "1px solid #cbd5e1",
                    borderRadius: 6,
                    background: "#fff"
                  }}
                >
                  <option value="全部">全部类型 (主机 / 软件 / 数据库 / 服务)</option>
                  <option value="主机设备">🖥️ 主机设备 (OS Root/Admin/BMC)</option>
                  <option value="系统软件">⚙️ 系统软件 (中间件 / 基础软件)</option>
                  <option value="数据库服务">🗄️ 数据库服务 (MySQL / Oracle / Redis)</option>
                  <option value="应用服务">🚀 应用服务 (微服务 / 业务账号)</option>
                </select>
              </div>

              {/* Query 4: General Keyword */}
              <div>
                <label style={{ display: "block", fontSize: 12, fontWeight: 600, color: "#334155", marginBottom: 5 }}>
                  🔍 模糊搜索 (服务/主机名/账号)
                </label>
                <input
                  type="text"
                  placeholder="如: Nginx, Oracle, root, 仲裁..."
                  value={searchKeyword}
                  onChange={(e) => setSearchKeyword(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && handleExecuteQuery()}
                  style={{
                    width: "100%",
                    padding: "7px 10px",
                    fontSize: 12,
                    border: "1px solid #cbd5e1",
                    borderRadius: 6,
                    outline: "none"
                  }}
                />
              </div>

              {/* Query Action Buttons */}
              <div style={{ display: "flex", gap: 8, minWidth: 180 }}>
                <button
                  type="button"
                  className="btn-primary"
                  style={{ flex: 1, padding: "7px 14px", fontSize: 12, fontWeight: 600 }}
                  onClick={handleExecuteQuery}
                >
                  🔍 立即查询
                </button>
                <button
                  type="button"
                  className="btn-secondary"
                  style={{ padding: "7px 12px", fontSize: 12 }}
                  onClick={handleResetQuery}
                >
                  ✕ 重置
                </button>
              </div>
            </div>

            {/* Active search filter pills */}
            {(appliedFilters.ip || appliedFilters.project !== "全部" || appliedFilters.type !== "全部" || appliedFilters.keyword) && (
              <div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 12, paddingTop: 10, borderTop: "1px dashed #e2e8f0", fontSize: 11 }}>
                <span style={{ color: "#64748b" }}>生效检索条件:</span>
                {appliedFilters.ip && (
                  <span style={{ background: "#eff6ff", color: "#1d4ed8", padding: "1px 6px", borderRadius: 4, border: "1px solid #bfdbfe" }}>
                    IP: {appliedFilters.ip}
                  </span>
                )}
                {appliedFilters.project !== "全部" && (
                  <span style={{ background: "#f0fdf4", color: "#15803d", padding: "1px 6px", borderRadius: 4, border: "1px solid #bbf7d0" }}>
                    项目: {appliedFilters.project}
                  </span>
                )}
                {appliedFilters.type !== "全部" && (
                  <span style={{ background: "#fef3c7", color: "#92400e", padding: "1px 6px", borderRadius: 4, border: "1px solid #fde68a" }}>
                    类型: {appliedFilters.type}
                  </span>
                )}
                {appliedFilters.keyword && (
                  <span style={{ background: "#faf5ff", color: "#7e22ce", padding: "1px 6px", borderRadius: 4, border: "1px solid #e9d5ff" }}>
                    关键词: {appliedFilters.keyword}
                  </span>
                )}
                <button
                  type="button"
                  onClick={handleResetQuery}
                  style={{ border: "none", background: "none", color: "#ef4444", cursor: "pointer", fontSize: 11 }}
                >
                  清除全部
                </button>
              </div>
            )}
          </div>

          {/* Results Table */}
          <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, overflow: "hidden" }}>
            <div style={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              padding: "10px 16px",
              background: "#f8fafc",
              borderBottom: "1px solid #e2e8f0"
            }}>
              <span style={{ fontSize: 12, fontWeight: 700, color: "#334155" }}>
                📋 关联资产凭据清单 (匹配 {filteredAssetRecords.length} 条记录)
              </span>
              <span style={{ fontSize: 11, color: "#64748b" }}>
                🔒 点击「解密查看」并输入安全口令即可查看明文与一键复制
              </span>
            </div>

            <div style={{ overflowX: "auto" }}>
              <table className="cmdb-data-table" style={{ minWidth: 960 }}>
                <thead>
                  <tr>
                    <th style={{ width: 140 }}>所属项目</th>
                    <th style={{ width: 130 }}>目标 IP 地址</th>
                    <th style={{ width: 150 }}>关联资产设备</th>
                    <th style={{ width: 100 }}>资产类型</th>
                    <th style={{ width: 170 }}>应用服务 / 软件名称</th>
                    <th style={{ width: 110 }}>授权登录账号</th>
                    <th style={{ width: 180 }}>访问密码 (加密存管)</th>
                    <th style={{ width: 130 }}>安全操作</th>
                  </tr>
                </thead>
                <tbody>
                  {paginatedRecords.length === 0 ? (
                    <tr>
                      <td colSpan={8} style={{ textAlign: "center", padding: "30px 0", color: "#94a3b8" }}>
                        未检索到符合条件的资产凭据，请尝试更换 IP 地址或项目名称
                      </td>
                    </tr>
                  ) : (
                    paginatedRecords.map((item) => {
                      const isUnlocked = unlockedRecordIds.has(item.id);
                      const isCopied = copyFeedback === item.id;

                      return (
                        <tr key={item.id}>
                          {/* Project */}
                          <td style={{ fontWeight: 600, color: "#1e293b", fontSize: 12 }}>
                            {item.projectName}
                          </td>

                          {/* IP */}
                          <td>
                            <span style={{
                              fontFamily: "monospace",
                              fontWeight: 700,
                              color: "#2563eb",
                              background: "#eff6ff",
                              padding: "2px 6px",
                              borderRadius: 4,
                              border: "1px solid #dbeafe"
                            }}>
                              {item.ip}
                            </span>
                            {item.port && (
                              <small style={{ color: "#64748b", marginLeft: 4 }}>:{item.port}</small>
                            )}
                          </td>

                          {/* Asset Name */}
                          <td style={{ color: "#334155", fontSize: 12 }}>
                            {item.assetName}
                          </td>

                          {/* Type */}
                          <td>
                            <span style={{
                              fontSize: 10,
                              padding: "2px 6px",
                              borderRadius: 3,
                              fontWeight: 600,
                              background: 
                                item.assetType === "主机设备" ? "#f1f5f9" :
                                item.assetType === "数据库服务" ? "#fef3c7" :
                                item.assetType === "系统软件" ? "#e0f2fe" : "#f0fdf4",
                              color: 
                                item.assetType === "主机设备" ? "#475569" :
                                item.assetType === "数据库服务" ? "#92400e" :
                                item.assetType === "系统软件" ? "#0369a1" : "#15803d"
                            }}>
                              {item.assetType}
                            </span>
                          </td>

                          {/* Service Name */}
                          <td style={{ fontWeight: 500, color: "#0f172a" }}>
                            {item.serviceName}
                          </td>

                          {/* Account */}
                          <td>
                            <span style={{
                              fontFamily: "monospace",
                              fontWeight: 600,
                              color: "#475569",
                              background: "#f8fafc",
                              padding: "2px 6px",
                              borderRadius: 3,
                              border: "1px solid #e2e8f0"
                            }}>
                              {item.account}
                            </span>
                          </td>

                          {/* Password Display (Encrypted vs Decrypted) */}
                          <td>
                            {isUnlocked ? (
                              <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                                <span style={{
                                  fontFamily: "monospace",
                                  fontWeight: 700,
                                  color: "#15803d",
                                  background: "#f0fdf4",
                                  padding: "2px 6px",
                                  borderRadius: 4,
                                  border: "1px solid #bbf7d0"
                                }}>
                                  {item.rawSecret}
                                </span>
                                <button
                                  type="button"
                                  onClick={() => handleCopyPassword(item.rawSecret, item.id)}
                                  style={{
                                    border: "1px solid #cbd5e1",
                                    background: isCopied ? "#16a34a" : "#fff",
                                    color: isCopied ? "#fff" : "#475569",
                                    borderRadius: 4,
                                    padding: "2px 6px",
                                    fontSize: 10,
                                    cursor: "pointer"
                                  }}
                                  title="复制密码"
                                >
                                  {isCopied ? "✓ 已复制" : "📋 复制"}
                                </button>
                              </div>
                            ) : (
                              <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                                <span style={{ fontFamily: "monospace", color: "#94a3b8", letterSpacing: 2 }}>
                                  ●●●●●●●●
                                </span>
                                <span style={{
                                  fontSize: 10,
                                  color: "#94a3b8",
                                  background: "#f8fafc",
                                  padding: "1px 4px",
                                  borderRadius: 3,
                                  border: "1px solid #e2e8f0"
                                }} title={`加密密文: ${item.encryptedCipher}`}>
                                  AES-256
                                </span>
                              </div>
                            )}
                          </td>

                          {/* Actions */}
                          <td>
                            <button
                              type="button"
                              className={isUnlocked ? "btn-secondary" : "btn-primary"}
                              style={{
                                padding: "3px 8px",
                                fontSize: 11,
                                fontWeight: 600
                              }}
                              onClick={() => handleRequestUnlock(item)}
                            >
                              {isUnlocked ? "🔒 重新锁定" : "👁️ 解密查看"}
                            </button>
                          </td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
            </div>

            {/* Pagination Controls */}
            {totalPages > 1 && (
              <div style={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
                padding: "10px 16px",
                background: "#f8fafc",
                borderTop: "1px solid #e2e8f0",
                fontSize: 12
              }}>
                <span style={{ color: "#64748b" }}>
                  第 {currentPage} / {totalPages} 页 (共 {filteredAssetRecords.length} 条)
                </span>
                <div style={{ display: "flex", gap: 6 }}>
                  <button
                    type="button"
                    className="btn-secondary"
                    style={{ padding: "3px 8px", fontSize: 11 }}
                    disabled={currentPage === 1}
                    onClick={() => setCurrentPage(prev => Math.max(1, prev - 1))}
                  >
                    ‹ 上一页
                  </button>
                  <button
                    type="button"
                    className="btn-secondary"
                    style={{ padding: "3px 8px", fontSize: 11 }}
                    disabled={currentPage === totalPages}
                    onClick={() => setCurrentPage(prev => Math.min(totalPages, prev + 1))}
                  >
                    下一页 ›
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* ================= VIEW 2: GENERIC ACCESS CREDENTIALS POOL ================= */}
      {activeTab === "genericPool" && (
        <div>
          <div className="cmdb-table-filter">
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <h3 style={{ margin: 0, fontSize: 15, color: "#0f172a" }}>🏷️ 平台通用访问凭据池</h3>
              <span className="room-badge">{credentials.length} 组批量凭据</span>
            </div>
            <button className="btn-primary" onClick={() => setShowModal(true)}>
              ＋ 录入通用凭据
            </button>
          </div>

          <div style={{ background: "#fff", border: "1px solid #e2e8f0", borderRadius: 8, overflowX: "auto", overflowY: "hidden" }}>
            <table className="cmdb-data-table" style={{ minWidth: 800 }}>
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
        </div>
      )}

      {/* ================= SECURITY AUTH MODAL (DECRYPTION PASSPHRASE) ================= */}
      {authModalVisible && (
        <div className="cmdb-modal-mask">
          <form className="cmdb-modal" onSubmit={handleVerifyPassphrase} style={{ maxWidth: 440 }}>
            <div className="cmdb-modal-header" style={{ borderBottom: "1px solid #fee2e2", background: "#fef2f2" }}>
              <h3 style={{ color: "#991b1b", display: "flex", alignItems: "center", gap: 6 }}>
                🔐 资产密码安全解密口令认证
              </h3>
              <button type="button" className="cmdb-modal-close" onClick={() => setAuthModalVisible(false)}>×</button>
            </div>
            
            <div className="cmdb-modal-body" style={{ padding: 18 }}>
              {targetRecordToUnlock && (
                <div style={{
                  background: "#f8fafc",
                  border: "1px solid #e2e8f0",
                  borderRadius: 6,
                  padding: "10px 12px",
                  marginBottom: 14,
                  fontSize: 12
                }}>
                  <div style={{ color: "#334155", marginBottom: 3 }}>
                    <strong>目标项目:</strong> {targetRecordToUnlock.projectName}
                  </div>
                  <div style={{ color: "#334155", marginBottom: 3 }}>
                    <strong>服务节点:</strong> {targetRecordToUnlock.serviceName} ({targetRecordToUnlock.ip})
                  </div>
                  <div style={{ color: "#2563eb", fontFamily: "monospace" }}>
                    <strong>授权账号:</strong> {targetRecordToUnlock.account}
                  </div>
                </div>
              )}

              <div className="form-field-item">
                <label style={{ fontWeight: 600, fontSize: 12, color: "#1e293b", marginBottom: 6 }}>
                  * 请输入平台安全管理口令:
                </label>
                <input
                  type="password"
                  placeholder="请输入解密口令 (如: admin123 或 Ops@2026)"
                  value={passphraseInput}
                  onChange={(e) => {
                    setPassphraseInput(e.target.value);
                    setAuthError("");
                  }}
                  autoFocus
                  required
                  style={{
                    width: "100%",
                    padding: "8px 12px",
                    fontSize: 13,
                    borderRadius: 6,
                    border: authError ? "1.5px solid #ef4444" : "1px solid #cbd5e1"
                  }}
                />
              </div>

              {authError && (
                <div style={{ color: "#dc2626", fontSize: 12, marginTop: 6, lineHeight: 1.4 }}>
                  {authError}
                </div>
              )}

              <div style={{
                marginTop: 12,
                fontSize: 11,
                color: "#64748b",
                background: "#f1f5f9",
                padding: "6px 10px",
                borderRadius: 4
              }}>
                ℹ️ 安全提示: 默认系统口令为 <code>admin123</code> 或 <code>Ops@2026</code>。查看操作将被计入安全审计日志。
              </div>
            </div>

            <div className="cmdb-modal-footer">
              <button type="button" className="btn-secondary" onClick={() => setAuthModalVisible(false)}>
                取 消
              </button>
              <button type="submit" className="btn-primary" style={{ background: "#dc2626", borderColor: "#dc2626" }}>
                🔓 验 证 并 解 密
              </button>
            </div>
          </form>
        </div>
      )}

      {/* ================= MODAL: ADD GENERIC CREDENTIAL ================= */}
      {showModal && (
        <div className="cmdb-modal-mask">
          <form className="cmdb-modal" onSubmit={handleAddSubmit}>
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
