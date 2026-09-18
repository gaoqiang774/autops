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
  const [ipSearchInput, setIpSearchInput] = useState("");
  const [ipSearchKeyword, setIpSearchKeyword] = useState("");
  const [projectFilter, setProjectFilter] = useState("全部");
  const [envFilter, setEnvFilter] = useState("全部");
  const [exposureFilter, setExposureFilter] = useState("全部");
  const [activeCveId, setActiveCveId] = useState<string | null>(null);

  // Vuln IP Batch Modal State
  const [showVulnIpModal, setShowVulnIpModal] = useState(false);
  const [vulnIpBatchText, setVulnIpBatchText] = useState("");


  // Pagination
  const [pageSize, setPageSize] = useState(15);
  const [currentPage, setCurrentPage] = useState(1);

  // Detail Modal & Script Modal
  const [detailAsset, setDetailAsset] = useState<UnifiedAsset | null>(null);
  const [copiedScript, setCopiedScript] = useState(false);
  const [copiedIps, setCopiedIps] = useState(false);
  const [showScriptModal, setShowScriptModal] = useState(false);
  const [expandScript, setExpandScript] = useState(false);

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

      // IP Address matching (supports single IP, comma/space separated, or prefix/subnet)
      if (ipSearchKeyword.trim()) {
        const rawKw = ipSearchKeyword.trim();
        const targetIps = rawKw.split(/[\s,;，；\n]+/).map(s => s.trim().toLowerCase()).filter(Boolean);
        const hostIps = [
          item.privateIp,
          (item as any).ip,
          item.internalWanIp,
          item.vip,
          item.eip,
          (item as any).publicIp
        ].filter(Boolean).map(s => String(s).toLowerCase());

        const matched = targetIps.some(tip => 
          hostIps.some(hip => hip === tip || hip.includes(tip))
        );
        if (!matched) return false;
      }

      return true;
    });
  }, [allAssets, osFamilyInput, osVersionInput, kernelInput, projectFilter, envFilter, exposureFilter, ipSearchKeyword]);

  // Vuln Batch IP Analysis
  const vulnBatchAnalysis = useMemo(() => {
    if (!vulnIpBatchText.trim()) return null;
    const inputLines = vulnIpBatchText
      .split(/[\s,;，；\n]+/)
      .map(s => s.trim())
      .filter(Boolean);
    const uniqueInputs = Array.from(new Set(inputLines));

    const matchedList: Array<{
      asset: UnifiedAsset;
      matchedQueryIp: string;
      isVulnHit: boolean;
      riskScore: string;
    }> = [];
    const notFoundIps: string[] = [];

    uniqueInputs.forEach(inputIp => {
      const lower = inputIp.toLowerCase();
      const hits = allAssets.filter(item => {
        const hostIps = [
          item.privateIp,
          (item as any).ip,
          item.internalWanIp,
          item.vip,
          item.eip,
          (item as any).publicIp
        ].filter(Boolean).map(s => String(s).toLowerCase());
        return hostIps.some(hip => hip === lower || hip.includes(lower));
      });

      if (hits.length > 0) {
        hits.forEach(h => {
          const isVulnHit = matchedAssets.some(m => m.id === h.id);
          const hasPublic = !!(h.eip || h.publicIp);
          let risk = "中危";
          if (hasPublic && h.env === "生产") risk = "🚨 极高危";
          else if (hasPublic || h.env === "生产") risk = "⚠️ 高危";

          if (!matchedList.some(item => item.asset.id === h.id)) {
            matchedList.push({
              asset: h,
              matchedQueryIp: inputIp,
              isVulnHit,
              riskScore: risk
            });
          }
        });
      } else {
        notFoundIps.push(inputIp);
      }
    });

    return {
      totalInputs: uniqueInputs.length,
      matchedCount: matchedList.length,
      vulnHitCount: matchedList.filter(m => m.isVulnHit).length,
      matchedList,
      notFoundIps
    };
  }, [vulnIpBatchText, allAssets, matchedAssets]);

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
    setIpSearchInput("");
    setIpSearchKeyword("");
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
            <div style={{ flex: 1, minWidth: 160 }}>
              <label style={{ display: "block", fontSize: 11, fontWeight: 600, color: "#334155", marginBottom: 4 }}>
                3. 内核版本特征 (Kernel Version)
              </label>
              <input 
                placeholder="如: 3.10.0、2.6.32、4.19..."
                value={kernelInput}
                onChange={e => { setKernelInput(e.target.value); setCurrentPage(1); setActiveCveId(null); }}
                style={{ width: "100%", fontSize: 12, padding: "6px 10px", borderRadius: 4, border: "1.5px solid #cbd5e1" }}
              />
            </div>

            {/* Input 4: Target IP Address */}
            <div style={{ flex: 1.2, minWidth: 230 }}>
              <label style={{ display: "block", fontSize: 11, fontWeight: 600, color: "#334155", marginBottom: 4 }}>
                4. 目标 IP 地址 (IP Address)
              </label>
              <div style={{ display: "flex", gap: 4 }}>
                <input 
                  placeholder="如: 192.125.31.250 或 10.150."
                  value={ipSearchInput}
                  onChange={e => setIpSearchInput(e.target.value)}
                  onKeyDown={e => {
                    if (e.key === "Enter") {
                      setIpSearchKeyword(ipSearchInput.trim());
                      setCurrentPage(1);
                    }
                  }}
                  style={{ flex: 1, fontSize: 12, padding: "6px 10px", borderRadius: 4, border: "1.5px solid #cbd5e1" }}
                />
                <button
                  type="button"
                  className="btn-primary"
                  onClick={() => {
                    setIpSearchKeyword(ipSearchInput.trim());
                    setCurrentPage(1);
                  }}
                  style={{
                    fontSize: 11,
                    padding: "0 10px",
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 3,
                    cursor: "pointer",
                    whiteSpace: "nowrap"
                  }}
                  title="按输入的 IP 地址快速检索过滤受影响主机"
                >
                  <span>🔍</span>
                  <span>查询IP</span>
                </button>
                {ipSearchKeyword && (
                  <button
                    type="button"
                    className="btn-secondary"
                    onClick={() => {
                      setIpSearchInput("");
                      setIpSearchKeyword("");
                      setCurrentPage(1);
                    }}
                    style={{ fontSize: 11, padding: "0 6px", color: "#64748b" }}
                    title="清除当前 IP 查询"
                  >
                    ✕
                  </button>
                )}
              </div>
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

        {/* Action Buttons: Export, Batch Copy & Emergency CLI Modal */}
        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          {/* 🔍 IP地址查询按钮 */}
          <button
            className="btn-secondary"
            style={{
              fontSize: 12,
              padding: "5px 12px",
              display: "flex",
              alignItems: "center",
              gap: 5,
              borderColor: "#10b981",
              color: "#059669",
              background: "#ecfdf5",
              fontWeight: 600
            }}
            onClick={() => {
              setVulnIpBatchText(ipSearchKeyword || "");
              setShowVulnIpModal(true);
            }}
            title="输入或批量粘贴 IP 地址，一键分析其受威胁漏洞与归属项目"
          >
            <span>🌐</span>
            <span>IP地址查询</span>
          </button>

          <button
            className="btn-secondary"
            style={{
              fontSize: 12,
              padding: "5px 12px",
              display: "flex",
              alignItems: "center",
              gap: 5,
              borderColor: "#38bdf8",
              color: "#0284c7",
              background: "#f0f9ff",
              fontWeight: 600
            }}
            onClick={() => setShowScriptModal(true)}
            title="查看并获取针对当前操作系统的批量应急核验命令（支持Ansible/SSH）"
          >
            💻 应急排查命令
          </button>

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

      {/* ================= 5. AFFECTED ASSETS TABLE WITH DEDICATED SCROLLBAR ================= */}
      <div style={{
        background: "#fff",
        border: "1px solid #e2e8f0",
        borderRadius: 8,
        boxShadow: "0 1px 3px rgba(0,0,0,0.03)",
        display: "flex",
        flexDirection: "column"
      }}>
        {/* Dedicated scrollable table area with visible scrollbars and sticky thead */}
        <div className="cmdb-table-scroll" style={{
          maxHeight: "calc(100vh - 410px)",
          minHeight: 380,
          border: "none",
          borderRadius: "8px 8px 0 0"
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
        </div>

        {/* Pagination Footer */}
        <div style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          padding: "8px 16px",
          background: "#f8fafc",
          borderTop: "1px solid #e2e8f0",
          borderRadius: "0 0 8px 8px",
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

      {/* ================= 6. EMERGENCY INSPECTION CLI TOOLBAR (NON-BLOCKING) ================= */}
      <div style={{
        background: "#ffffff",
        border: "1px solid #e2e8f0",
        borderRadius: 8,
        padding: "10px 16px",
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
        boxShadow: "0 1px 2px rgba(0,0,0,0.02)",
        flexWrap: "wrap",
        gap: 8
      }}>
        <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
          <span style={{ fontSize: 16 }}>💻</span>
          <div>
            <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
              <strong style={{ fontSize: 13, color: "#1e293b" }}>
                智能运维探针联动 · 批量应急核验命令 (Quick Inspection Script)
              </strong>
              <span style={{ fontSize: 10, background: "#f1f5f9", color: "#475569", padding: "1px 6px", borderRadius: 3 }}>
                支持 Ansible / SSH
              </span>
              <span style={{ fontSize: 10, background: "#e0f2fe", color: "#0369a1", padding: "1px 6px", borderRadius: 3, fontWeight: 600 }}>
                {osFamilyInput.includes("Windows") ? "Windows PowerShell" : "Linux Bash"}
              </span>
            </div>
            <small style={{ color: "#64748b", fontSize: 11, display: "block", marginTop: 2 }}>
              针对当前筛选的 {matchedAssets.length} 台受影响设备生成内核与软件包排查命令，已收纳至安全抽屉，点击按钮查看或展开预览
            </small>
          </div>
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <button
            className="btn-secondary"
            style={{ fontSize: 11, padding: "4px 10px", display: "flex", alignItems: "center", gap: 4 }}
            onClick={() => setShowScriptModal(true)}
          >
            ⛶ 弹窗沉浸查看
          </button>
          <button
            className="btn-secondary"
            style={{ fontSize: 11, padding: "4px 10px", display: "flex", alignItems: "center", gap: 4 }}
            onClick={() => setExpandScript(prev => !prev)}
          >
            {expandScript ? "▲ 收起面板" : "▼ 展开预览"}
          </button>
          <button
            className="btn-primary"
            style={{ fontSize: 11, padding: "4px 10px" }}
            onClick={() => {
              navigator.clipboard?.writeText?.(inspectionScript);
              setCopiedScript(true);
              setTimeout(() => setCopiedScript(false), 2000);
            }}
          >
            {copiedScript ? "已复制！" : "一键复制代码"}
          </button>
        </div>
      </div>

      {expandScript && (
        <div style={{
          background: "#0b1120",
          border: "1px solid #1e293b",
          borderRadius: 8,
          padding: "12px 16px",
          color: "#f8fafc"
        }}>
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
      )}

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

      {/* ================= 8. EMERGENCY SCRIPT INSPECTION MODAL ================= */}
      {showScriptModal && (
        <div className="cmdb-modal-overlay" onClick={e => { if (e.target === e.currentTarget) setShowScriptModal(false); }}>
          <div className="cmdb-modal-dialog" style={{ maxWidth: 780, width: "90%" }}>
            <div className="cmdb-modal-header" style={{ background: "#0f172a", borderBottom: "1px solid #334155" }}>
              <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                <span style={{ fontSize: 20 }}>💻</span>
                <div>
                  <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                    <h3 style={{ margin: 0, color: "#fff", fontSize: 16 }}>
                      批量应急核验排查命令 (Quick Inspection Script)
                    </h3>
                    <span style={{ fontSize: 10, background: "#1e293b", color: "#38bdf8", padding: "2px 6px", borderRadius: 3, fontWeight: 600 }}>
                      支持 Ansible / 堡垒机
                    </span>
                  </div>
                  <small style={{ color: "#94a3b8", display: "block", marginTop: 3 }}>
                    系统类别: <strong style={{ color: "#e2e8f0" }}>{osFamilyInput.includes("Windows") ? "Windows Server" : (osFamilyInput || "Linux / CentOS / 信创系统")}</strong> · 已关联 {matchedAssets.length} 台受影响目标设备
                  </small>
                </div>
              </div>
              <button
                type="button"
                className="cmdb-modal-close"
                style={{ color: "#fff" }}
                onClick={() => setShowScriptModal(false)}
              >
                ×
              </button>
            </div>

            <div className="cmdb-modal-body" style={{ padding: "16px 20px" }}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 10 }}>
                <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                  <span style={{ fontSize: 12, color: "#475569", fontWeight: 600 }}>
                    执行脚本代码 ({osFamilyInput.includes("Windows") ? "PowerShell" : "Linux Bash"}):
                  </span>
                </div>
                <button
                  className="btn-primary"
                  style={{ fontSize: 12, padding: "4px 14px", display: "flex", alignItems: "center", gap: 6 }}
                  onClick={() => {
                    navigator.clipboard?.writeText?.(inspectionScript);
                    setCopiedScript(true);
                    setTimeout(() => setCopiedScript(false), 2000);
                  }}
                >
                  {copiedScript ? "✓ 已复制到剪贴板！" : "📋 一键复制代码"}
                </button>
              </div>

              <pre style={{
                background: "#030712",
                border: "1px solid #334155",
                borderRadius: 6,
                padding: 14,
                color: "#4ade80",
                fontSize: 12,
                fontFamily: "Consolas, Menlo, Monaco, monospace",
                margin: 0,
                whiteSpace: "pre-wrap",
                lineHeight: 1.6
              }}>
                {inspectionScript}
              </pre>

              <div style={{ marginTop: 14, background: "#f8fafc", border: "1px solid #e2e8f0", borderRadius: 6, padding: "12px 14px", fontSize: 12, color: "#475569" }}>
                <strong style={{ color: "#0f172a", display: "block", marginBottom: 6 }}>💡 应急批量执行与处置流程：</strong>
                <ol style={{ margin: 0, paddingLeft: 18, lineHeight: 1.8 }}>
                  <li>在列表工具栏点击<strong>【📋 复制全部IP清单】</strong>，获取当前已筛查定位出的 {matchedAssets.length} 台设备业务内网 IP；</li>
                  <li>通过 JumpServer 堡垒机命令批量下发或 Ansible Ad-hoc 任务批量执行此核查脚本；</li>
                  <li>检查关键组件版本（OpenSSH / Sudo / Polkit / Kernel）及补丁安装状态，针对公网暴露资产优先部署防火墙安全策略封堵。</li>
                </ol>
              </div>
            </div>

            <div className="cmdb-modal-footer" style={{ padding: "12px 20px", display: "flex", justifyContent: "space-between", alignItems: "center" }}>
              <span style={{ fontSize: 11, color: "#64748b" }}>
                命令生成于客户端 · 不对生产主机造成破坏性改动
              </span>
              <button
                className="btn-secondary"
                style={{ padding: "5px 18px", fontSize: 12 }}
                onClick={() => setShowScriptModal(false)}
              >
                关闭
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ================= MODAL: VULNERABILITY IP ANALYSIS & LOOKUP ================= */}
      {showVulnIpModal && (
        <div className="cmdb-modal-overlay" style={{ zIndex: 9999 }}>
          <div className="cmdb-modal-content" style={{ width: 880, maxWidth: "95vw", maxHeight: "90vh", display: "flex", flexDirection: "column" }}>
            <div className="cmdb-modal-header" style={{ display: "flex", justifyContent: "space-between", alignItems: "center", borderBottom: "1px solid #e2e8f0", padding: "14px 20px" }}>
              <h3 style={{ margin: 0, fontSize: 16, display: "flex", alignItems: "center", gap: 8, color: "#0f172a" }}>
                <span>🛡️</span>
                <span>漏洞资产 IP 快速定位与受威胁分析</span>
                <span style={{ fontSize: 11, background: "#ecfdf5", color: "#059669", padding: "2px 8px", borderRadius: 4, fontWeight: 500 }}>
                  支持安全告警多IP批量排查
                </span>
              </h3>
              <button type="button" className="cmdb-modal-close" onClick={() => setShowVulnIpModal(false)}>×</button>
            </div>

            <div className="cmdb-modal-body" style={{ padding: "16px 20px", flex: 1, overflowY: "auto", display: "flex", flexDirection: "column", gap: 14 }}>
              <div>
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 6 }}>
                  <label style={{ fontSize: 12, fontWeight: 600, color: "#334155" }}>
                    输入或粘贴待排查的 IP 地址清单 (如来自漏洞扫描报告、告警工单，支持换行或空格)：
                  </label>
                  <div style={{ display: "flex", gap: 8 }}>
                    <button
                      type="button"
                      className="btn-secondary"
                      style={{ fontSize: 11, padding: "2px 8px" }}
                      onClick={async () => {
                        try {
                          const text = await navigator.clipboard?.readText();
                          if (text) setVulnIpBatchText(text.trim());
                        } catch (e) {
                          // ignore
                        }
                      }}
                    >
                      📋 粘贴剪贴板内容
                    </button>
                    <button
                      type="button"
                      className="btn-secondary"
                      style={{ fontSize: 11, padding: "2px 8px", color: "#dc2626" }}
                      onClick={() => setVulnIpBatchText("")}
                    >
                      ✕ 清空
                    </button>
                  </div>
                </div>
                <textarea
                  rows={4}
                  placeholder={`例如输入单个或多个 IP，如：\n192.125.31.250\n10.150.88.10\n192.123.2.110\n10.150.`}
                  value={vulnIpBatchText}
                  onChange={e => setVulnIpBatchText(e.target.value)}
                  style={{
                    width: "100%",
                    fontSize: 12,
                    fontFamily: "monospace",
                    padding: "8px 10px",
                    borderRadius: 6,
                    border: "1.5px solid #cbd5e1",
                    resize: "vertical"
                  }}
                />
              </div>

              {/* Analysis Stats Bar */}
              {vulnBatchAnalysis && (
                <div style={{
                  background: "#f8fafc",
                  border: "1px solid #e2e8f0",
                  borderRadius: 6,
                  padding: "10px 14px",
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                  flexWrap: "wrap",
                  gap: 10
                }}>
                  <div style={{ display: "flex", gap: 14, alignItems: "center" }}>
                    <div style={{ fontSize: 12, color: "#64748b" }}>
                      输入 IP 数量: <strong style={{ color: "#0f172a" }}>{vulnBatchAnalysis.totalInputs}</strong>
                    </div>
                    <div style={{ fontSize: 12, color: "#2563eb" }}>
                      命中台账资产: <strong style={{ color: "#1d4ed8" }}>{vulnBatchAnalysis.matchedCount} 台</strong>
                    </div>
                    <div style={{ fontSize: 12, color: "#dc2626" }}>
                      受当前漏洞威胁: <strong style={{ color: "#b91c1c" }}>{vulnBatchAnalysis.vulnHitCount} 台</strong>
                    </div>
                    {vulnBatchAnalysis.notFoundIps.length > 0 && (
                      <div style={{ fontSize: 12, color: "#64748b" }}>
                        未登记 IP: <strong>{vulnBatchAnalysis.notFoundIps.length} 个</strong>
                      </div>
                    )}
                  </div>

                  <div style={{ display: "flex", gap: 6 }}>
                    <button
                      type="button"
                      className="btn-secondary"
                      style={{ fontSize: 11, padding: "3px 8px" }}
                      onClick={() => {
                        const summary = vulnBatchAnalysis.matchedList.map(item => 
                          `${item.asset.name}\t${item.asset.privateIp || item.asset.ip}\t${item.asset.projectName}\t${item.asset.osVersion || item.asset.os}\t${item.riskScore}\t${item.isVulnHit ? "命中漏洞" : "当前安全"}`
                        ).join("\n");
                        navigator.clipboard?.writeText?.(summary);
                        alert("✓ 已复制漏洞排查分析结果到剪贴板！");
                      }}
                    >
                      📋 复制分析结果
                    </button>
                    <button
                      type="button"
                      className="btn-primary"
                      style={{ fontSize: 11, padding: "3px 10px" }}
                      onClick={() => {
                        setIpSearchKeyword(vulnIpBatchText.trim());
                        setIpSearchInput(vulnIpBatchText.trim().replace(/[\r\n]+/g, " "));
                        setShowVulnIpModal(false);
                        setCurrentPage(1);
                      }}
                    >
                      📍 应用至当前排查列表 ({vulnBatchAnalysis.matchedCount}台)
                    </button>
                  </div>
                </div>
              )}

              {/* Matched Assets Table */}
              {vulnBatchAnalysis && vulnBatchAnalysis.matchedList.length > 0 && (
                <div style={{ border: "1px solid #e2e8f0", borderRadius: 6, overflow: "hidden", maxHeight: 280, overflowY: "auto" }}>
                  <table className="cmdb-data-table" style={{ fontSize: 11, width: "100%" }}>
                    <thead style={{ position: "sticky", top: 0, zIndex: 2 }}>
                      <tr>
                        <th style={{ width: 40 }}>序号</th>
                        <th style={{ width: 160 }}>设备名称</th>
                        <th style={{ width: 130 }}>私有业务 IP</th>
                        <th style={{ width: 140 }}>所属项目</th>
                        <th style={{ width: 140 }}>系统与内核版本</th>
                        <th style={{ width: 90 }}>安全威胁状态</th>
                        <th style={{ width: 80 }}>公网暴露</th>
                      </tr>
                    </thead>
                    <tbody>
                      {vulnBatchAnalysis.matchedList.map((item, idx) => (
                        <tr key={item.asset.id}>
                          <td style={{ fontFamily: "monospace", color: "#64748b" }}>{idx + 1}</td>
                          <td><strong>{item.asset.name}</strong></td>
                          <td><code style={{ color: "#2563eb", fontWeight: 600 }}>{item.asset.privateIp || item.asset.ip || "-"}</code></td>
                          <td>{item.asset.projectName}</td>
                          <td>{item.asset.osVersion || item.asset.os} <small style={{ color: "#94a3b8" }}>{item.asset.kernelVersion}</small></td>
                          <td>
                            <span style={{
                              padding: "1px 6px",
                              borderRadius: 3,
                              fontSize: 10,
                              fontWeight: 600,
                              background: item.isVulnHit ? "#fee2e2" : "#f0fdf4",
                              color: item.isVulnHit ? "#b91c1c" : "#15803d"
                            }}>
                              {item.isVulnHit ? "⚠️ 命中漏洞" : "🛡️ 未命中当前规则"}
                            </span>
                          </td>
                          <td>
                            {item.asset.eip || item.asset.publicIp ? (
                              <span style={{ color: "#dc2626", fontWeight: 600 }}>直接暴露</span>
                            ) : (
                              <span style={{ color: "#16a34a" }}>仅内网</span>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}

              {/* Not Found IPs List */}
              {vulnBatchAnalysis && vulnBatchAnalysis.notFoundIps.length > 0 && (
                <div style={{ background: "#fef2f2", border: "1px solid #fee2e2", borderRadius: 6, padding: "8px 12px", fontSize: 11 }}>
                  <span style={{ color: "#dc2626", fontWeight: 600 }}>未在台账中检索到的 IP ({vulnBatchAnalysis.notFoundIps.length}个): </span>
                  <span style={{ color: "#b91c1c", fontFamily: "monospace" }}>{vulnBatchAnalysis.notFoundIps.join(", ")}</span>
                </div>
              )}
            </div>

            <div className="cmdb-modal-footer" style={{ borderTop: "1px solid #e2e8f0", padding: "12px 20px", display: "flex", justifyContent: "flex-end" }}>
              <button type="button" className="btn-secondary" onClick={() => setShowVulnIpModal(false)}>关 闭</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
