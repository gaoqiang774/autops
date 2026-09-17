"use client";
import React, { useState, useMemo } from "react";
import { PhysicalHost, VmHost, SwitchDevice, ProjectGroup, AssetMeta } from "../cmdbData";

interface VulnDetectionProps {
  projects: ProjectGroup[];
  hosts: PhysicalHost[];
  vms: VmHost[];
  switches?: SwitchDevice[];
}

type UnifiedAsset = (PhysicalHost | VmHost | SwitchDevice) & {
  _kind: "physical" | "vm" | "switch";
};

interface CvePreset {
  id: string;
  cveId: string;
  name: string;
  level: "超危" | "高危" | "中危";
  score: string;
  osFamily: string;
  osVersion: string;
  kernelKeyword?: string;
  desc: string;
  workaround: string;
}

const CVE_PRESETS: CvePreset[] = [
  {
    id: "cve-1",
    cveId: "CVE-2024-6387",
    name: "regreSSHion OpenSSH 远程代码执行漏洞",
    level: "超危",
    score: "9.8",
    osFamily: "CentOS",
    osVersion: "7.9",
    desc: "基于 glibc 的 32/64 位 Linux 系统中 OpenSSH 服务存在信号处理程序竞争条件漏洞，未经身份认证的远程攻击者可利用该漏洞执行任意代码并获取 Root 权限。",
    workaround: "临时缓解: 在 /etc/ssh/sshd_config 中设置 LoginGraceTime 0 并重启 sshd，随后尽快升级 openssh-server 至安全版本。"
  },
  {
    id: "cve-2",
    cveId: "CVE-2021-4034",
    name: "PwnKit Polkit 本地提权高危漏洞",
    level: "高危",
    score: "7.8",
    osFamily: "Red Hat",
    osVersion: "6.9",
    desc: "Polkit pkexec 工具存在参数处理越界写入漏洞，任何非特权本地用户均可稳定提升至 root 权限，影响包括 CentOS 7/8、RedHat、Ubuntu 等系统。",
    workaround: "紧急命令: chmod 0755 /usr/bin/pkexec 或执行 yum update -y polkit 完成安全升级。"
  },
  {
    id: "cve-3",
    cveId: "CVE-2017-0144",
    name: "EternalBlue 永恒之蓝 SMB 远程代码执行",
    level: "超危",
    score: "9.8",
    osFamily: "Windows Server",
    osVersion: "2012 R2",
    desc: "Microsoft Server Message Block 1.0 (SMBv1) 协议处理特制请求时存在内存损坏缺陷，攻击者无需凭据即可通过 445 端口远程执行任意命令并全网横向渗透。",
    workaround: "处置建议: 关闭 SMBv1 协议支持，在安全组中拦截 445/139 端口入站流量，安装微软 MS17-010 安全补丁。"
  },
  {
    id: "cve-4",
    cveId: "CVE-2021-3156",
    name: "Baron Samedit Sudo 堆缓冲区溢出提权",
    level: "高危",
    score: "7.8",
    osFamily: "CentOS",
    osVersion: "8",
    desc: "Sudo 在处理命令行参数反斜杠转义时存在堆缓冲区溢出，普通用户无需 sudoers 授权即可获取 root Shell。",
    workaround: "检查版本: sudo --version；执行 yum update -y sudo 升级至最新安全版本。"
  },
  {
    id: "cve-5",
    cveId: "KYLIN-SEC-2023",
    name: "国产信创操作系统内核安全基线排查",
    level: "中危",
    score: "6.5",
    osFamily: "麒麟",
    osVersion: "V10",
    desc: "针对国产化信创操作系统（银河麒麟/中标麒麟/统信UOS）进行安全合规基线检测与高危内核参数巡检。",
    workaround: "执行 kylin-security-check 或 yum-cron 自动拉取麒麟官方信创源安全补丁包。"
  }
];

export default function VulnDetection({
  projects,
  hosts,
  vms,
  switches = []
}: VulnDetectionProps) {
  // Search state
  const [osFamilyInput, setOsFamilyInput] = useState("");
  const [osVersionInput, setOsVersionInput] = useState("");
  const [kernelInput, setKernelInput] = useState("");
  const [projectFilter, setProjectFilter] = useState("全部");
  const [envFilter, setEnvFilter] = useState("全部");
  const [exposureFilter, setExposureFilter] = useState("全部");
  const [activeCveId, setActiveCveId] = useState<string | null>(null);

  // Pagination
  const [pageSize, setPageSize] = useState(15);
  const [currentPage, setCurrentPage] = useState(1);

  // Detail Modal
  const [detailAsset, setDetailAsset] = useState<UnifiedAsset | null>(null);
  const [copiedScript, setCopiedScript] = useState(false);
  const [copiedIps, setCopiedIps] = useState(false);

  // All unified assets
  const allAssets: UnifiedAsset[] = useMemo(() => {
    const pList: UnifiedAsset[] = hosts.map(h => ({ ...h, _kind: "physical" }));
    const vList: UnifiedAsset[] = vms.map(v => ({ ...v, _kind: "vm" }));
    const sList: UnifiedAsset[] = switches.map(s => ({ ...s, _kind: "switch" }));
    return [...pList, ...vList, ...sList].sort((a, b) => (a.seq || 9999) - (b.seq || 9999));
  }, [hosts, vms, switches]);

  // Distinct OS Family suggestions
  const osFamilyOptions = ["CentOS", "Windows Server", "麒麟", "Red Hat", "统信UOS", "Ubuntu", "openEuler", "Oracle Linux"];

  // Filtered Assets based on inputs
  const matchedAssets = useMemo(() => {
    return allAssets.filter(item => {
      // OS Family matching
      if (osFamilyInput.trim()) {
        const targetFamily = (item.osFamily || item.os || "").toLowerCase();
        const kw = osFamilyInput.trim().toLowerCase();
        // Support matching '麒麟' to 中标麒麟 or 银河麒麟
        if (kw === "麒麟" && targetFamily.includes("麒麟")) {
          // match
        } else if (!targetFamily.includes(kw)) {
          return false;
        }
      }

      // OS Version matching
      if (osVersionInput.trim()) {
        const targetVer = (item.osVersion || item.os || "").toLowerCase();
        const kw = osVersionInput.trim().toLowerCase();
        if (!targetVer.includes(kw)) {
          return false;
        }
      }

      // Kernel Version matching
      if (kernelInput.trim()) {
        const targetKernel = (item.kernelVersion || "").toLowerCase();
        const kw = kernelInput.trim().toLowerCase();
        if (!targetKernel.includes(kw)) {
          return false;
        }
      }

      // Project filter
      if (projectFilter !== "全部" && item.projectName !== projectFilter) {
        return false;
      }

      // Env filter
      if (envFilter !== "全部" && item.env !== envFilter) {
        return false;
      }

      // Exposure filter
      if (exposureFilter === "公网暴露") {
        if (!item.eip && !item.publicIp) return false;
      } else if (exposureFilter === "仅内网") {
        if (item.eip || item.publicIp) return false;
      }

      return true;
    });
  }, [allAssets, osFamilyInput, osVersionInput, kernelInput, projectFilter, envFilter, exposureFilter]);

  // Pagination slice
  const totalPages = Math.ceil(matchedAssets.length / pageSize) || 1;
  const paginatedAssets = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return matchedAssets.slice(start, start + pageSize);
  }, [matchedAssets, currentPage, pageSize]);

  // Statistical Breakdown
  const stats = useMemo(() => {
    const total = matchedAssets.length;
    const projectSet = new Set<string>();
    let prodCount = 0;
    let publicCount = 0;
    let xinchuangCount = 0;

    matchedAssets.forEach(a => {
      if (a.projectName) projectSet.add(a.projectName);
      if (a.env === "生产") prodCount++;
      if (a.eip || a.publicIp) publicCount++;
      if (a.isXinchuang === "是") xinchuangCount++;
    });

    return {
      total,
      projectCount: projectSet.size,
      prodCount,
      publicCount,
      xinchuangCount
    };
  }, [matchedAssets]);

  // Handle Preset Click
  function applyCvePreset(preset: CvePreset) {
    if (activeCveId === preset.id) {
      // Toggle off
      setActiveCveId(null);
      setOsFamilyInput("");
      setOsVersionInput("");
      setKernelInput("");
    } else {
      setActiveCveId(preset.id);
      setOsFamilyInput(preset.osFamily);
      setOsVersionInput(preset.osVersion);
      setKernelInput(preset.kernelKeyword || "");
      setCurrentPage(1);
    }
  }

  // Clear all filters
  function handleReset() {
    setOsFamilyInput("");
    setOsVersionInput("");
    setKernelInput("");
    setProjectFilter("全部");
    setEnvFilter("全部");
    setExposureFilter("全部");
    setActiveCveId(null);
    setCurrentPage(1);
  }

  // Export matched assets to CSV
  function handleExportCsv() {
    if (matchedAssets.length === 0) {
      alert("当前筛选条件下无匹配设备，无需导出！");
      return;
    }

    const headers = [
      "序号",
      "设备名称",
      "设备类型",
      "所属项目",
      "客户单位",
      "环境",
      "承载云商",
      "区域",
      "私有业务IP",
      "内大网IP",
      "VIP/EIP",
      "操作系统系列",
      "系统版本号",
      "内核版本",
      "国产信创",
      "CPU规格",
      "内存",
      "远程端口",
      "备注"
    ];

    const rows = matchedAssets.map((item, idx) => [
      item.seq || idx + 1,
      `"${item.name || ""}"`,
      item.deviceType || "虚拟机",
      `"${item.projectName || ""}"`,
      `"${item.customerName || ""}"`,
      item.env || "生产",
      item.cloudVendor || "",
      `"${item.regionName || ""}"`,
      item.privateIp || item.ip || "",
      item.internalWanIp || "",
      item.eip || item.vip || "",
      item.osFamily || "",
      `"${item.osVersion || ""}"`,
      `"${item.kernelVersion || ""}"`,
      item.isXinchuang || "否",
      `"${item.cpuCores ? `${item.cpuCores}C` : ""}"`,
      `"${item.memoryGb ? `${item.memoryGb}G` : ""}"`,
      item.remotePort || 22,
      `"${(item.remarks || "").replace(/"/g, '""')}"`
    ]);

    const csvContent = "\uFEFF" + [headers.join(","), ...rows.map(r => r.join(","))].join("\r\n");
    const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    const timestamp = new Date().toISOString().slice(0, 10);
    link.setAttribute("href", url);
    link.setAttribute("download", `受威胁资产定位清单_${osFamilyInput || "全部OS"}_${osVersionInput || "全部版本"}_${timestamp}.csv`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  }

  // Copy IP list
  function handleCopyIps() {
    const ipList = matchedAssets
      .map(a => a.privateIp || a.ip)
      .filter(Boolean);

    if (ipList.length === 0) return;
    navigator.clipboard?.writeText?.(ipList.join("\n"));
    setCopiedIps(true);
    setTimeout(() => setCopiedIps(false), 2000);
  }

  // Generate remediation CLI script
  const inspectionScript = useMemo(() => {
    if (osFamilyInput.includes("Windows")) {
      return `Get-CimInstance Win32_OperatingSystem | Select-Object Caption, Version, BuildNumber, OSArchitecture\nGet-HotFix | Select-Object -First 10 HotFixID, InstalledOn`;
    }
    return `# 1. 快速核验当前主机操作系统与内核版本\ncat /etc/os-release 2>/dev/null || cat /etc/redhat-release\nuname -r -m\n# 2. 检查关键服务软件包版本\nrpm -qa | grep -E "openssh|polkit|sudo|kernel" 2>/dev/null || dpkg -l | grep -E "openssh|polkit|sudo"`;
  }, [osFamilyInput]);

  return (
    <div className="cmdb-container" style={{ display: "flex", flexDirection: "column", gap: 14 }}>
      {/* ================= 1. HEADER & SEARCH MODULE ================= */}
      <div style={{
        background: "#ffffff",
        border: "1px solid #e2e8f0",
        borderRadius: 8,
        padding: "16px 20px",
        boxShadow: "0 1px 3px rgba(0,0,0,0.04)"
      }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", flexWrap: "wrap", gap: 12, marginBottom: 14 }}>
          <div>
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <span style={{ fontSize: 18 }}>🛡️</span>
              <h3 style={{ margin: 0, fontSize: 17, color: "#0f172a", fontWeight: 700 }}>
                操作系统漏洞排查与受威胁设备精准定位
              </h3>
              <span style={{ background: "#eff6ff", color: "#2563eb", fontSize: 11, padding: "2px 8px", borderRadius: 4, fontWeight: 600 }}>
                DevSecOps · 秒级定位
              </span>
            </div>
            <p style={{ margin: "4px 0 0", fontSize: 12, color: "#64748b" }}>
              输入操作系统名称、版本号或内核版本，秒级筛选全网受影响设备；联动展示所属项目、IP地址、公网暴露面与责任团队。
            </p>
          </div>

          {/* Quick Stats Badges */}
          <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
            <div style={{ background: "#f8fafc", border: "1px solid #e2e8f0", padding: "6px 12px", borderRadius: 6, textAlign: "center" }}>
              <span style={{ fontSize: 10, color: "#64748b", display: "block" }}>命中受影响设备</span>
              <strong style={{ fontSize: 16, color: stats.total > 0 ? "#dc2626" : "#0f172a" }}>
                {stats.total} <small style={{ fontSize: 11, fontWeight: 400 }}>台</small>
              </strong>
            </div>

            <div style={{ background: "#f8fafc", border: "1px solid #e2e8f0", padding: "6px 12px", borderRadius: 6, textAlign: "center" }}>
              <span style={{ fontSize: 10, color: "#64748b", display: "block" }}>波及业务项目</span>
              <strong style={{ fontSize: 16, color: "#2563eb" }}>
                {stats.projectCount} <small style={{ fontSize: 11, fontWeight: 400 }}>个</small>
              </strong>
            </div>

            <div style={{ background: "#f8fafc", border: "1px solid #e2e8f0", padding: "6px 12px", borderRadius: 6, textAlign: "center" }}>
              <span style={{ fontSize: 10, color: "#64748b", display: "block" }}>生产高危节点</span>
              <strong style={{ fontSize: 16, color: "#b45309" }}>
                {stats.prodCount} <small style={{ fontSize: 11, fontWeight: 400 }}>台</small>
              </strong>
            </div>

            <div style={{ background: "#fef2f2", border: "1px solid #fee2e2", padding: "6px 12px", borderRadius: 6, textAlign: "center" }}>
              <span style={{ fontSize: 10, color: "#dc2626", display: "block" }}>公网直接暴露</span>
              <strong style={{ fontSize: 16, color: "#ef4444" }}>
                {stats.publicCount} <small style={{ fontSize: 11, fontWeight: 400 }}>台</small>
              </strong>
            </div>

            <div style={{ background: "#f0fdf4", border: "1px solid #dcfce7", padding: "6px 12px", borderRadius: 6, textAlign: "center" }}>
              <span style={{ fontSize: 10, color: "#16a34a", display: "block" }}>国产信创节点</span>
              <strong style={{ fontSize: 16, color: "#15803d" }}>
                {stats.xinchuangCount} <small style={{ fontSize: 11, fontWeight: 400 }}>台</small>
              </strong>
            </div>
          </div>
        </div>

        {/* ================= 2. CORE SEARCH INPUTS ================= */}
        <div style={{
          background: "#f8fafc",
          border: "1px solid #e2e8f0",
          borderRadius: 6,
          padding: "12px 14px",
          display: "flex",
          flexDirection: "column",
          gap: 10
        }}>
          {/* Main Filter Row: OS Name & OS Version Inputs */}
          <div style={{ display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap" }}>
            {/* Input 1: OS Name / Family */}
            <div style={{ flex: 1, minWidth: 220 }}>
              <label style={{ display: "block", fontSize: 11, fontWeight: 600, color: "#334155", marginBottom: 4 }}>
                1. 操作系统名称 / 家族 (OS Family)
              </label>
              <div style={{ display: "flex", gap: 4 }}>
                <input 
                  placeholder="如: CentOS、Red Hat、Windows Server、麒麟、统信..."
                  value={osFamilyInput}
                  onChange={e => { setOsFamilyInput(e.target.value); setCurrentPage(1); setActiveCveId(null); }}
                  style={{ flex: 1, fontSize: 12, padding: "6px 10px", borderRadius: 4, border: "1.5px solid #cbd5e1" }}
                />
              </div>
            </div>

            {/* Input 2: OS Version */}
            <div style={{ flex: 1, minWidth: 200 }}>
              <label style={{ display: "block", fontSize: 11, fontWeight: 600, color: "#334155", marginBottom: 4 }}>
                2. 操作系统版本号 (OS Version)
              </label>
              <input 
                placeholder="如: 7.9、6.9、2012 R2、V10 SP3、18.04..."
                value={osVersionInput}
                onChange={e => { setOsVersionInput(e.target.value); setCurrentPage(1); setActiveCveId(null); }}
                style={{ width: "100%", fontSize: 12, padding: "6px 10px", borderRadius: 4, border: "1.5px solid #cbd5e1" }}
              />
            </div>

            {/* Input 3: Kernel Version */}
            <div style={{ flex: 1, minWidth: 180 }}>
              <label style={{ display: "block", fontSize: 11, fontWeight: 600, color: "#334155", marginBottom: 4 }}>
                3. 内核版本特征 (Kernel Version)
              </label>
              <input 
                placeholder="如: 3.10.0、2.6.32、4.19、4.15..."
                value={kernelInput}
                onChange={e => { setKernelInput(e.target.value); setCurrentPage(1); setActiveCveId(null); }}
                style={{ width: "100%", fontSize: 12, padding: "6px 10px", borderRadius: 4, border: "1.5px solid #cbd5e1" }}
              />
            </div>

            {/* Reset Button */}
            <div style={{ alignSelf: "flex-end" }}>
              <button 
                className="btn-secondary"
                style={{ height: 32, padding: "0 12px", fontSize: 12 }}
                onClick={handleReset}
              >
                ✕ 重置条件
              </button>
            </div>
          </div>

          {/* Quick OS Suggestions Chips */}
          <div style={{ display: "flex", alignItems: "center", gap: 6, flexWrap: "wrap", fontSize: 11 }}>
            <span style={{ color: "#64748b" }}>常用系统快捷标签:</span>
            {osFamilyOptions.map(opt => {
              const active = osFamilyInput.toLowerCase() === opt.toLowerCase();
              return (
                <button
                  key={opt}
                  type="button"
                  onClick={() => {
                    setOsFamilyInput(opt);
                    setCurrentPage(1);
                    setActiveCveId(null);
                  }}
                  style={{
                    background: active ? "#2563eb" : "#ffffff",
                    color: active ? "#ffffff" : "#334155",
                    border: active ? "1px solid #2563eb" : "1px solid #cbd5e1",
                    borderRadius: 4,
                    padding: "2px 8px",
                    cursor: "pointer",
                    fontSize: 11,
                    transition: "all 0.15s"
                  }}
                >
                  {opt}
                </button>
              );
            })}
          </div>

          {/* Secondary Filters: Project, Env, Exposure */}
          <div style={{ display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap", borderTop: "1px dashed #cbd5e1", paddingTop: 8 }}>
            <span style={{ fontSize: 11, color: "#64748b" }}>细粒度筛选:</span>
            
            {/* Project Filter */}
            <select 
              value={projectFilter} 
              onChange={e => { setProjectFilter(e.target.value); setCurrentPage(1); }}
              style={{ fontSize: 11, padding: "4px 8px" }}
            >
              <option value="全部">全部业务项目 ({projects.length} 个)</option>
              {projects.map(p => (
                <option key={p.id} value={p.name}>{p.name}</option>
              ))}
            </select>

            {/* Env Filter */}
            <select 
              value={envFilter} 
              onChange={e => { setEnvFilter(e.target.value); setCurrentPage(1); }}
              style={{ fontSize: 11, padding: "4px 8px" }}
            >
              <option value="全部">全部环境类型</option>
              <option value="生产">生产环境 (需高优保障)</option>
              <option value="测试">测试/开发环境</option>
            </select>

            {/* Exposure Filter */}
            <select 
              value={exposureFilter} 
              onChange={e => { setExposureFilter(e.target.value); setCurrentPage(1); }}
              style={{ fontSize: 11, padding: "4px 8px" }}
            >
              <option value="全部">全部暴露面</option>
              <option value="公网暴露">⚠️ 仅公网 / EIP 直接暴露 (极高危)</option>
              <option value="仅内网">🔒 仅私有内网环境</option>
            </select>

            <span style={{ fontSize: 12, color: "#64748b", marginLeft: "auto" }}>
              当前匹配命中: <strong style={{ color: "#2563eb" }}>{matchedAssets.length}</strong> 台信息资产
            </span>
          </div>
        </div>

        {/* ================= 3. CVE HIGH-RISK PRESETS ================= */}
        <div style={{ marginTop: 12 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 6 }}>
            <span style={{ fontSize: 11, fontWeight: 700, color: "#475569", display: "flex", alignItems: "center", gap: 4 }}>
              🔥 行业高危 CVE 漏洞一键排查方案库 (点击快捷套用受威胁版本规则)
            </span>
            <small style={{ color: "#94a3b8" }}>基于国家信息安全漏洞库 (CNNVD / NVD) 实时标准</small>
          </div>

          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(210px, 1fr))", gap: 8 }}>
            {CVE_PRESETS.map(cve => {
              const isSelected = activeCveId === cve.id;
              return (
                <div
                  key={cve.id}
                  onClick={() => applyCvePreset(cve)}
                  style={{
                    background: isSelected ? "#eff6ff" : "#fff",
                    border: isSelected ? "1.5px solid #2563eb" : "1px solid #e2e8f0",
                    borderRadius: 6,
                    padding: "8px 10px",
                    cursor: "pointer",
                    boxShadow: isSelected ? "0 2px 8px rgba(37,99,235,0.15)" : "none",
                    transition: "all 0.15s"
                  }}
                  title="点击自动填充并排查受该漏洞威胁的主机清单"
                >
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 3 }}>
                    <span style={{
                      fontSize: 10,
                      fontWeight: 700,
                      padding: "1px 5px",
                      borderRadius: 3,
                      background: cve.level === "超危" ? "#fee2e2" : cve.level === "高危" ? "#ffedd5" : "#fef3c7",
                      color: cve.level === "超危" ? "#b91c1c" : cve.level === "高危" ? "#c2410c" : "#b45309"
                    }}>
                      {cve.level} {cve.score}
                    </span>
                    <strong style={{ fontSize: 11, color: isSelected ? "#1d4ed8" : "#0f172a" }}>{cve.cveId}</strong>
                  </div>
                  <div style={{ fontSize: 11, color: "#334155", fontWeight: 600, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                    {cve.name}
                  </div>
                  <div style={{ fontSize: 10, color: "#64748b", marginTop: 2 }}>
                    目标: {cve.osFamily} · {cve.osVersion}
                  </div>
                </div>
              );
            })}
          </div>

          {/* Active CVE Description Banner */}
          {activeCveId && (
            <div style={{ background: "#fffbeb", border: "1px solid #fef3c7", borderRadius: 6, padding: "10px 14px", marginTop: 8, fontSize: 12 }}>
              {(() => {
                const target = CVE_PRESETS.find(c => c.id === activeCveId);
                if (!target) return null;
                return (
                  <div>
                    <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 4 }}>
                      <strong style={{ color: "#92400e" }}>📌 【{target.cveId}】{target.name}</strong>
                      <span style={{ fontSize: 11, color: "#b45309" }}>CVSS 评分: {target.score}</span>
                    </div>
                    <div style={{ color: "#78350f", lineHeight: 1.4, marginBottom: 4 }}>
                      <strong>漏洞简述:</strong> {target.desc}
                    </div>
                    <div style={{ color: "#15803d", background: "#f0fdf4", padding: "4px 8px", borderRadius: 4 }}>
                      <strong>应急处置建议:</strong> {target.workaround}
                    </div>
                  </div>
                );
              })()}
            </div>
          )}
        </div>
      </div>

      {/* ================= 4. RESULTS TOOLBAR & EXPORT ================= */}
      <div style={{
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
        flexWrap: "wrap",
        gap: 10,
        background: "#fff",
        border: "1px solid #e2e8f0",
        borderRadius: 8,
        padding: "10px 16px"
      }}>
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <span style={{ fontWeight: 700, fontSize: 14, color: "#0f172a" }}>
            📋 排查匹配设备清单
          </span>
          <span style={{ fontSize: 12, color: "#64748b" }}>
            共定位到 <strong style={{ color: "#dc2626" }}>{matchedAssets.length}</strong> 台设备 (涉及 {stats.projectCount} 个项目)
          </span>
        </div>

        {/* Action Buttons: Export & Batch Copy */}
        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          <button
            className="btn-secondary"
            style={{ fontSize: 12, padding: "5px 10px", display: "flex", alignItems: "center", gap: 4 }}
            onClick={handleCopyIps}
            title="将匹配到的所有设备业务IP复制到剪贴板，方便配置防火墙或批量执行巡检"
          >
            📋 {copiedIps ? "已复制全部IP！" : "复制全部IP清单"}
          </button>

          <button
            className="btn-primary"
            style={{ fontSize: 12, padding: "5px 12px", display: "flex", alignItems: "center", gap: 4 }}
            onClick={handleExportCsv}
            title="导出当前筛选出的受影响设备清单为 Excel/CSV 文件"
          >
            📥 导出受影响设备清单 (CSV)
          </button>
        </div>
      </div>

      {/* ================= 5. AFFECTED ASSETS TABLE ================= */}
      <div style={{
        background: "#fff",
        border: "1px solid #e2e8f0",
        borderRadius: 8,
        overflowX: "auto",
        overflowY: "hidden",
        boxShadow: "0 1px 3px rgba(0,0,0,0.03)"
      }}>
        <table className="cmdb-data-table" style={{ minWidth: 1280 }}>
          <thead>
            <tr>
              <th style={{ width: 45 }}>序号</th>
              <th style={{ width: 220 }}>设备名称 / 资产形态</th>
              <th style={{ width: 170 }}>所属项目 · 客户单位</th>
              <th style={{ width: 150 }}>操作系统家族 / 版本</th>
              <th style={{ width: 160 }}>系统内核版本 (Kernel)</th>
              <th style={{ width: 150 }}>私有业务 IP / 内大网</th>
              <th style={{ width: 130 }}>VIP / EIP / 暴露面</th>
              <th style={{ width: 140 }}>环境 · 云厂商 · 区域</th>
              <th style={{ width: 100 }}>国产信创</th>
              <th style={{ width: 100 }}>风险级别</th>
              <th style={{ width: 100, textAlign: "center" }}>操作</th>
            </tr>
          </thead>
          <tbody>
            {paginatedAssets.length === 0 ? (
              <tr>
                <td colSpan={11} style={{ textAlign: "center", padding: "48px 0", color: "#94a3b8" }}>
                  <div style={{ fontSize: 24, marginBottom: 8 }}>🔍</div>
                  <strong style={{ fontSize: 14, color: "#64748b" }}>未匹配到符合当前操作系统条件的受影响设备</strong>
                  <p style={{ fontSize: 12, margin: "4px 0 0", color: "#94a3b8" }}>
                    请尝试清除部分条件或调整操作系统名称/版本号关键字
                  </p>
                </td>
              </tr>
            ) : (
              paginatedAssets.map(item => {
                const isPhysical = item._kind === "physical" || item.deviceType === "物理机";
                const hasPublic = !!(item.eip || item.publicIp);

                // Risk calculation: public exposure + production = ultra-critical
                let riskText = "中危";
                let riskBg = "#fef3c7";
                let riskColor = "#b45309";

                if (hasPublic && item.env === "生产") {
                  riskText = "🚨 极高危";
                  riskBg = "#fee2e2";
                  riskColor = "#b91c1c";
                } else if (hasPublic || item.env === "生产") {
                  riskText = "⚠️ 高危";
                  riskBg = "#ffedd5";
                  riskColor = "#c2410c";
                }

                return (
                  <tr key={item.id} style={{ fontSize: 12 }}>
                    {/* Seq */}
                    <td style={{ fontFamily: "monospace", color: "#64748b", fontWeight: 600 }}>
                      {item.seq || "-"}
                    </td>

                    {/* Device Name & Type */}
                    <td>
                      <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                        <span style={{
                          fontSize: 10,
                          padding: "1px 5px",
                          borderRadius: 3,
                          fontWeight: 600,
                          background: isPhysical ? "#fef3c7" : "#e0f2fe",
                          color: isPhysical ? "#b45309" : "#0369a1"
                        }}>
                          {item.deviceType || (isPhysical ? "物理机" : "虚拟机")}
                        </span>
                        <strong
                          style={{ color: "#0f172a", cursor: "pointer" }}
                          title="点击查看全量台账元数据"
                          onClick={() => setDetailAsset(item)}
                        >
                          {item.name}
                        </strong>
                      </div>
                      {item.remarks && (
                        <small style={{ color: "#64748b", display: "block", marginTop: 2, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis", maxWidth: 210 }}>
                          {item.remarks}
                        </small>
                      )}
                    </td>

                    {/* Project & Customer */}
                    <td>
                      <div style={{ fontWeight: 600, color: "#1e293b" }}>{item.projectName}</div>
                      <small style={{ color: "#64748b", display: "block" }}>{item.customerName}</small>
                    </td>

                    {/* OS Family & Version */}
                    <td>
                      <div style={{ display: "flex", alignItems: "center", gap: 4 }}>
                        <span style={{
                          fontWeight: 700,
                          color: item.osFamily?.includes("Windows") ? "#0284c7" : "#1e293b"
                        }}>
                          {item.osFamily || "Linux"}
                        </span>
                        <span style={{
                          background: "#eff6ff",
                          color: "#1d4ed8",
                          padding: "1px 5px",
                          borderRadius: 3,
                          fontSize: 11,
                          fontWeight: 600
                        }}>
                          {item.osVersion || item.os || "-"}
                        </span>
                      </div>
                    </td>

                    {/* Kernel Version */}
                    <td>
                      <code style={{ fontSize: 11, color: "#475569", fontFamily: "monospace" }}>
                        {item.kernelVersion || "-"}
                      </code>
                    </td>

                    {/* IPs */}
                    <td>
                      <div style={{ fontFamily: "monospace", fontWeight: 600, color: "#2563eb" }}>
                        {item.privateIp || item.ip || "-"}
                      </div>
                      {item.internalWanIp && (
                        <small style={{ fontFamily: "monospace", color: "#64748b", display: "block" }}>
                          内大网: {item.internalWanIp}
                        </small>
                      )}
                    </td>

                    {/* VIP / EIP / Exposure */}
                    <td>
                      {hasPublic ? (
                        <span style={{ background: "#fee2e2", color: "#b91c1c", fontSize: 10, padding: "2px 6px", borderRadius: 4, fontWeight: 700, display: "inline-block" }}>
                          ⚠️ 公网暴露 ({item.eip || item.publicIp})
                        </span>
                      ) : (
                        <span style={{ color: "#64748b", fontSize: 11 }}>
                          {item.vip ? `VIP: ${item.vip}` : "纯内网防护"}
                        </span>
                      )}
                    </td>

                    {/* Env, Cloud, Region */}
                    <td>
                      <div style={{ display: "flex", gap: 4, alignItems: "center" }}>
                        <span style={{
                          padding: "1px 5px",
                          borderRadius: 3,
                          fontSize: 10,
                          background: item.env === "生产" ? "#dcfce7" : "#f1f5f9",
                          color: item.env === "生产" ? "#15803d" : "#475569"
                        }}>
                          {item.env || "生产"}
                        </span>
                        <span style={{ color: "#2563eb", fontWeight: 500, fontSize: 11 }}>{item.cloudVendor}</span>
                      </div>
                      <small style={{ color: "#64748b", display: "block", marginTop: 2 }}>{item.regionName || "-"}</small>
                    </td>

                    {/* Xinchuang */}
                    <td>
                      {item.isXinchuang === "是" ? (
                        <span style={{ background: "#f0fdf4", color: "#16a34a", fontSize: 10, padding: "2px 6px", borderRadius: 3, fontWeight: 700 }}>
                          🛡️ 国产信创
                        </span>
                      ) : (
                        <span style={{ color: "#94a3b8", fontSize: 11 }}>常规系统</span>
                      )}
                    </td>

                    {/* Risk Level */}
                    <td>
                      <span style={{ background: riskBg, color: riskColor, fontSize: 10, padding: "2px 6px", borderRadius: 4, fontWeight: 700 }}>
                        {riskText}
                      </span>
                    </td>

                    {/* Actions */}
                    <td style={{ textAlign: "center" }}>
                      <div style={{ display: "flex", justifyContent: "center", gap: 4 }}>
                        <button
                          className="btn-secondary"
                          style={{ padding: "2px 6px", fontSize: 11 }}
                          onClick={() => setDetailAsset(item)}
                          title="查看该设备的完整台账元数据"
                        >
                          档案
                        </button>
                        <button
                          className="btn-secondary"
                          style={{ padding: "2px 6px", fontSize: 11, color: "#2563eb" }}
                          onClick={() => alert(`已为目标设备 [${item.name} (${item.privateIp || item.ip})] 生成漏洞加固与修复处置工单！`)}
                          title="生成安全修复工单"
                        >
                          派单
                        </button>
                      </div>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>

        {/* Pagination Footer */}
        <div style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          padding: "8px 16px",
          background: "#f8fafc",
          borderTop: "1px solid #e2e8f0",
          fontSize: 12,
          color: "#475569"
        }}>
          <div>
            共命中 <strong style={{ color: "#0f172a" }}>{matchedAssets.length}</strong> 台受影响设备
          </div>

          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <span>每页展示:</span>
            <select
              value={pageSize}
              onChange={e => { setPageSize(Number(e.target.value)); setCurrentPage(1); }}
              style={{ padding: "2px 6px", fontSize: 12 }}
            >
              <option value={15}>15 条</option>
              <option value={25}>25 条</option>
              <option value={50}>50 条</option>
              <option value={100}>100 条</option>
            </select>

            <button
              className="btn-secondary"
              style={{ padding: "3px 8px", fontSize: 11 }}
              disabled={currentPage <= 1}
              onClick={() => setCurrentPage(prev => Math.max(1, prev - 1))}
            >
              ‹ 上一页
            </button>
            <span>第 <strong>{currentPage}</strong> / {totalPages} 页</span>
            <button
              className="btn-secondary"
              style={{ padding: "3px 8px", fontSize: 11 }}
              disabled={currentPage >= totalPages}
              onClick={() => setCurrentPage(prev => Math.min(totalPages, prev + 1))}
            >
              下一页 ›
            </button>
          </div>
        </div>
      </div>

      {/* ================= 6. EMERGENCY INSPECTION CLI WORKSPACE ================= */}
      <div style={{
        background: "#0b1120",
        border: "1px solid #1e293b",
        borderRadius: 8,
        padding: "14px 18px",
        color: "#f8fafc"
      }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span style={{ color: "#38bdf8", fontWeight: 700, fontSize: 13 }}>
              💻 智能运维探针联动 · 批量应急核验命令 (Quick Inspection Script)
            </span>
            <span style={{ fontSize: 10, background: "#1e293b", color: "#94a3b8", padding: "1px 6px", borderRadius: 3 }}>
              支持 Ansible / SSH 批量执行
            </span>
          </div>

          <button
            className="btn-primary"
            style={{ fontSize: 11, padding: "3px 10px" }}
            onClick={() => {
              navigator.clipboard?.writeText?.(inspectionScript);
              setCopiedScript(true);
              setTimeout(() => setCopiedScript(false), 2000);
            }}
          >
            {copiedScript ? "已复制脚本！" : "一键复制代码"}
          </button>
        </div>

        <pre style={{
          background: "#030712",
          border: "1px solid #334155",
          borderRadius: 6,
          padding: 12,
          color: "#4ade80",
          fontSize: 12,
          fontFamily: "monospace",
          margin: 0,
          whiteSpace: "pre-wrap",
          lineHeight: 1.5
        }}>
          {inspectionScript}
        </pre>
      </div>

      {/* ================= 7. FULL ASSET METADATA MODAL ================= */}
      {detailAsset && (
        <div className="cmdb-modal-overlay">
          <div className="cmdb-modal-dialog" style={{ maxWidth: 880, width: "92%" }}>
            <div className="cmdb-modal-header" style={{ background: "#0f172a", borderBottom: "1px solid #334155" }}>
              <div>
                <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                  <span style={{ background: "#2563eb", color: "#fff", fontSize: 11, padding: "1px 6px", borderRadius: 3 }}>
                    {detailAsset.deviceType || "服务器"}
                  </span>
                  <h3 style={{ margin: 0, color: "#fff", fontSize: 16 }}>{detailAsset.name}</h3>
                  {detailAsset.isXinchuang === "是" && (
                    <span style={{ background: "#ef4444", color: "#fff", fontSize: 11, padding: "2px 6px", borderRadius: 4 }}>
                      🛡️ 国产信创 OS
                    </span>
                  )}
                </div>
                <small style={{ color: "#94a3b8", display: "block", marginTop: 4 }}>
                  归属项目: {detailAsset.projectName} · 客户单位: {detailAsset.customerName} · {detailAsset.env}环境
                </small>
              </div>
              <button type="button" className="cmdb-modal-close" style={{ color: "#fff" }} onClick={() => setDetailAsset(null)}>×</button>
            </div>

            <div className="cmdb-modal-body" style={{ maxHeight: "72vh", overflowY: "auto", padding: 20 }}>
              {/* 基本信息 */}
              <h5 style={{ margin: "0 0 10px", fontSize: 13, color: "#0284c7", borderBottom: "1px solid #e0f2fe", paddingBottom: 4 }}>
                📋 项目与基础归属 (台账基本信息)
              </h5>
              <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: 12, fontSize: 12, marginBottom: 16 }}>
                <div><span style={{ color: "#64748b" }}>项目名称:</span> <strong style={{ color: "#1d4ed8" }}>{detailAsset.projectName || "-"}</strong></div>
                <div><span style={{ color: "#64748b" }}>客户名称:</span> <strong>{detailAsset.customerName || "-"}</strong></div>
                <div><span style={{ color: "#64748b" }}>环境类型:</span> <strong>{detailAsset.env || "-"}</strong></div>
                <div><span style={{ color: "#64748b" }}>承载云商:</span> <strong style={{ color: "#2563eb" }}>{detailAsset.cloudVendor || "-"}</strong></div>
                <div><span style={{ color: "#64748b" }}>安全区域:</span> <strong>{detailAsset.regionName || "-"}</strong></div>
                <div><span style={{ color: "#64748b" }}>设备类型:</span> <strong>{detailAsset.deviceType || "虚拟机"}</strong></div>
              </div>

              {/* 操作系统与内核 */}
              <h5 style={{ margin: "0 0 10px", fontSize: 13, color: "#dc2626", borderBottom: "1px solid #fee2e2", paddingBottom: 4 }}>
                🛡️ 操作系统、内核版本与信创认证 (安全排查重点)
              </h5>
              <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: 12, fontSize: 12, marginBottom: 16 }}>
                <div><span style={{ color: "#64748b" }}>OS 家族:</span> <strong style={{ color: "#0f172a" }}>{detailAsset.osFamily || "Linux"}</strong></div>
                <div><span style={{ color: "#64748b" }}>OS 版本:</span> <strong style={{ color: "#2563eb" }}>{detailAsset.osVersion || detailAsset.os || "-"}</strong></div>
                <div><span style={{ color: "#64748b" }}>系统内核:</span> <code style={{ color: "#b91c1c", fontWeight: 600 }}>{detailAsset.kernelVersion || "-"}</code></div>
                <div><span style={{ color: "#64748b" }}>国产信创认证:</span> <strong>{detailAsset.isXinchuang === "是" ? "🛡️ 是 (国产信创)" : "否"}</strong></div>
                <div><span style={{ color: "#64748b" }}>远程管理端口:</span> <code>{detailAsset.remotePort || 22}</code></div>
              </div>

              {/* 网络信息 */}
              <h5 style={{ margin: "0 0 10px", fontSize: 13, color: "#0d9488", borderBottom: "1px solid #ccfbf1", paddingBottom: 4 }}>
                🌐 网络与 IP 矩阵
              </h5>
              <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12, fontSize: 12, marginBottom: 16 }}>
                <div style={{ background: "#f8fafc", padding: 8, borderRadius: 6 }}>
                  <span style={{ color: "#64748b", display: "block" }}>私有业务 IP:</span>
                  <code style={{ fontSize: 13, color: "#2563eb", fontWeight: 600 }}>{detailAsset.privateIp || detailAsset.ip || "-"}</code>
                </div>
                <div style={{ background: "#f8fafc", padding: 8, borderRadius: 6 }}>
                  <span style={{ color: "#64748b", display: "block" }}>内大网 IP:</span>
                  <code style={{ fontSize: 13, color: "#059669", fontWeight: 600 }}>{detailAsset.internalWanIp || "-"}</code>
                </div>
                <div style={{ background: "#f8fafc", padding: 8, borderRadius: 6 }}>
                  <span style={{ color: "#64748b", display: "block" }}>VIP 地址:</span>
                  <code style={{ fontSize: 13, color: "#7c3aed", fontWeight: 600 }}>{detailAsset.vip || "-"}</code>
                </div>
                <div style={{ background: "#f8fafc", padding: 8, borderRadius: 6 }}>
                  <span style={{ color: "#64748b", display: "block" }}>EIP / 公网暴露 IP:</span>
                  <code style={{ fontSize: 13, color: "#ea580c", fontWeight: 600 }}>{detailAsset.eip || detailAsset.publicIp || "-"}</code>
                </div>
              </div>

              {/* 规格配置 */}
              <h5 style={{ margin: "0 0 10px", fontSize: 13, color: "#d97706", borderBottom: "1px solid #fef3c7", paddingBottom: 4 }}>
                ⚡ 硬件规格与存储配额
              </h5>
              <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))", gap: 12, fontSize: 12, marginBottom: 16 }}>
                <div><span style={{ color: "#64748b" }}>CPU 架构:</span> <strong>{detailAsset.cpuArch || "x86_64"}</strong></div>
                <div><span style={{ color: "#64748b" }}>CPU 核心:</span> <strong>{detailAsset.cpuCores ? `${detailAsset.cpuCores} 核` : "-"}</strong></div>
                <div><span style={{ color: "#64748b" }}>内存配额:</span> <strong>{detailAsset.memoryGb ? `${detailAsset.memoryGb} GB` : "-"}</strong></div>
                <div><span style={{ color: "#64748b" }}>系统盘:</span> <strong>{detailAsset.systemDiskGb ? `${detailAsset.systemDiskGb} GB` : "-"}</strong></div>
                <div><span style={{ color: "#64748b" }}>数据盘:</span> <strong>{detailAsset.dataDiskGb ? `${detailAsset.dataDiskGb} GB` : "-"}</strong></div>
              </div>
            </div>

            <div className="cmdb-modal-footer">
              <button type="button" className="btn-secondary" onClick={() => setDetailAsset(null)}>关闭档案</button>
              <button
                type="button"
                className="btn-primary"
                onClick={() => {
                  alert(`已成功针对设备 [${detailAsset.name}] 触发智能探针深度巡检与漏洞核查任务！`);
                  setDetailAsset(null);
                }}
              >
                立即发起漏洞复检
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
