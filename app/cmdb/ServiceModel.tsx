"use client";
import React, { useState, useMemo, useEffect } from "react";
import { BusinessModel, PhysicalHost, VmHost, DatabaseAsset, MiddlewareAsset, BackupAsset, SwitchDevice } from "../cmdbData";
import { 
  BusinessSystemRelation, 
  PRESET_BUSINESS_RELATIONS, 
  getRelatedBusinessSystems,
  BUSINESS_RELATIONS_STORAGE_KEY
} from "../cmdbBusinessRelations";
import { EXCEL_PROJECTS } from "../cmdbCodeDict";

interface ServiceModelProps {
  businesses?: BusinessModel[];
  hosts?: PhysicalHost[];
  vms?: VmHost[];
  databases?: DatabaseAsset[];
  middlewares?: MiddlewareAsset[];
  backups?: BackupAsset[];
  switches?: SwitchDevice[];
  rooms?: any[];
  cabinets?: any[];
  onNavigateToProject?: (projName: string) => void;
}

export default function ServiceModel({
  businesses = [],
  hosts = [],
  vms = [],
  databases = [],
  middlewares = [],
  backups = [],
  switches = [],
  onNavigateToProject
}: ServiceModelProps) {
  // 安全保障：若外部传入 businesses 为空，兜底提供默认业务模型
  const safeBusinesses = useMemo(() => {
    if (businesses && businesses.length > 0) return businesses;
    return [
      {
        id: "biz-default",
        name: "全景政务信息系统",
        code: "BIZ-ALL",
        department: "北京市人力资源和社会保障局",
        manager: "运维保障组",
        level: "核心",
        hostCount: vms.length + hosts.length,
        status: "运行中",
        description: "全景IT资产业务依赖拓扑",
        hosts: []
      }
    ];
  }, [businesses, vms.length, hosts.length]);

  const [selectedBizId, setSelectedBizId] = useState<string>(safeBusinesses[0]?.id || "biz-default");
  const [searchKeyword, setSearchKeyword] = useState("");
  const [viewMode, setViewMode] = useState<"layered" | "cross_business">("layered");
  const [clusterFilter, setClusterFilter] = useState<string>("all");

  // ================= 核心持久化关联关系状态 =================
  const [relations, setRelations] = useState<BusinessSystemRelation[]>(() => {
    if (typeof window !== "undefined") {
      try {
        const saved = localStorage.getItem(BUSINESS_RELATIONS_STORAGE_KEY);
        if (saved) {
          const parsed = JSON.parse(saved);
          if (Array.isArray(parsed) && parsed.length > 0) {
            return parsed;
          }
        }
      } catch (e) {
        console.warn("读取本地关联业务持久化失败:", e);
      }
    }
    return PRESET_BUSINESS_RELATIONS;
  });

  // 持久化保存工具函数
  const updateRelations = (newRels: BusinessSystemRelation[]) => {
    setRelations(newRels);
    if (typeof window !== "undefined") {
      try {
        localStorage.setItem(BUSINESS_RELATIONS_STORAGE_KEY, JSON.stringify(newRels));
      } catch (e) {
        console.warn("保存关联业务到本地存储失败:", e);
      }
    }
  };

  // Toast 提示状态
  const [toastMessage, setToastMessage] = useState<string | null>(null);
  const showToast = (msg: string) => {
    setToastMessage(msg);
    setTimeout(() => setToastMessage(null), 3500);
  };

  // ================= 弹窗状态 (新增 / 编辑 / 删除确认 / 清单管理) =================
  const [showFormModal, setShowFormModal] = useState(false);
  const [formMode, setFormMode] = useState<"create" | "edit">("create");
  const [editingRelId, setEditingRelId] = useState<string | null>(null);

  // 表单受控字段
  const [formSource, setFormSource] = useState("");
  const [formTarget, setFormTarget] = useState("");
  const [formType, setFormType] = useState<"upstream" | "downstream" | "peer" | "shared_hub">("peer");
  const [formLabel, setFormLabel] = useState("");
  const [formDirection, setFormDirection] = useState<"incoming" | "outgoing" | "bidirectional">("bidirectional");
  const [formProtocol, setFormProtocol] = useState("RESTful API (HTTPS/JSON)");
  const [formFrequency, setFormFrequency] = useState<"实时同步" | "准实时流" | "每日批处理" | "按需调用">("实时同步");
  const [formDataEntities, setFormDataEntities] = useState("");
  const [formDesc, setFormDesc] = useState("");

  // 删除确认弹窗状态
  const [deletingRelation, setDeletingRelation] = useState<BusinessSystemRelation | null>(null);

  // 关联清单集中管理弹窗
  const [showManageModal, setShowManageModal] = useState(false);
  const [manageKeyword, setManageKeyword] = useState("");
  const [manageTypeFilter, setManageTypeFilter] = useState("all");

  // 拓扑节点详情抽屉
  const [inspectedNode, setInspectedNode] = useState<{
    title: string;
    type: string;
    detail: string;
    extra?: string;
    relationData?: BusinessSystemRelation;
    targetSystemName?: string;
  } | null>(null);

  // 搜索过滤业务系统
  const filteredBusinesses = useMemo(() => {
    return safeBusinesses.filter(b => {
      if (!b) return false;
      if (clusterFilter !== "all") {
        if (clusterFilter === "rs_zc" && !b.name.includes("仲裁") && !b.name.includes("调解")) return false;
        if (clusterFilter === "rs_sb" && !b.name.includes("险") && !b.name.includes("工伤") && !b.name.includes("城居") && !b.name.includes("监管") && !b.name.includes("数转")) return false;
        if (clusterFilter === "tj" && !b.name.includes("退军")) return false;
        if (clusterFilter === "gh" && !b.name.includes("工会")) return false;
        if (clusterFilter === "bk" && !b.name.includes("北控") && !b.name.includes("伟仕") && !b.name.includes("智科") && !b.name.includes("三兴") && !b.name.includes("数科") && !b.name.includes("燃气") && !b.name.includes("天兴")) return false;
      }
      if (searchKeyword.trim()) {
        const kw = searchKeyword.toLowerCase();
        return (b.name || "").toLowerCase().includes(kw) ||
               (b.department || "").toLowerCase().includes(kw) ||
               (b.code || "").toLowerCase().includes(kw);
      }
      return true;
    });
  }, [safeBusinesses, searchKeyword, clusterFilter]);

  // 当前选中的业务系统对象（绝对保证非空）
  const currentBizIndex = useMemo(() => {
    return filteredBusinesses.findIndex(b => b.id === selectedBizId);
  }, [filteredBusinesses, selectedBizId]);

  const selectedBiz = useMemo(() => {
    return safeBusinesses.find(b => b.id === selectedBizId) ||
           filteredBusinesses[0] ||
           safeBusinesses[0];
  }, [safeBusinesses, selectedBizId, filteredBusinesses]);

  // 上一个 / 下一个项目快捷切换
  function handlePrevProject() {
    if (filteredBusinesses.length <= 1) return;
    const newIdx = currentBizIndex <= 0 ? filteredBusinesses.length - 1 : currentBizIndex - 1;
    setSelectedBizId(filteredBusinesses[newIdx].id);
    setInspectedNode(null);
  }

  function handleNextProject() {
    if (filteredBusinesses.length <= 1) return;
    const newIdx = currentBizIndex >= filteredBusinesses.length - 1 ? 0 : currentBizIndex + 1;
    setSelectedBizId(filteredBusinesses[newIdx].id);
    setInspectedNode(null);
  }

  // 快捷跳转至特定业务系统拓扑
  function handleSwitchToSystem(sysName: string) {
    const targetBiz = safeBusinesses.find(b => b.name === sysName);
    if (targetBiz) {
      setSelectedBizId(targetBiz.id);
      setInspectedNode(null);
    } else {
      showToast(`已定位业务系统：${sysName}`);
    }
  }

  // 计算当前系统的关联业务 (相关业务 / 上下游协同系统)
  const relatedSystems = useMemo(() => {
    if (!selectedBiz) return { upstream: [], peer: [], downstream: [], all: [] };
    const allSysNames = safeBusinesses.map(b => b.name);
    return getRelatedBusinessSystems(selectedBiz.name, allSysNames, selectedBiz.department, relations);
  }, [selectedBiz, safeBusinesses, relations]);

  // ================= CRUD 操作实现 =================

  // 打开录入弹窗
  function handleOpenCreateModal(presetType?: "upstream" | "downstream" | "peer" | "shared_hub") {
    setFormMode("create");
    setEditingRelId(null);
    setFormSource(selectedBiz.name);
    setFormTarget("");
    const type = presetType || "peer";
    setFormType(type);
    setFormLabel(type === "upstream" ? "上游网上申报入口" : type === "downstream" ? "下游风控监管中台" : "平行协同联动业务");
    setFormDirection(type === "upstream" ? "incoming" : type === "downstream" ? "outgoing" : "bidirectional");
    setFormProtocol("RESTful API (HTTPS/JSON)");
    setFormFrequency("实时同步");
    setFormDataEntities("");
    setFormDesc("");
    setShowFormModal(true);
  }

  // 打开修改/编辑弹窗
  function handleOpenEditModal(rel: BusinessSystemRelation) {
    setFormMode("edit");
    setEditingRelId(rel.id);
    setFormSource(rel.sourceSystem || selectedBiz.name);
    setFormTarget(rel.targetSystem);
    setFormType(rel.relationType);
    setFormLabel(rel.relationTypeLabel || "");
    setFormDirection(rel.direction || "bidirectional");
    setFormProtocol(rel.interfaceProtocol || "RESTful API (HTTPS/JSON)");
    setFormFrequency(rel.frequency || "实时同步");
    setFormDataEntities((rel.dataEntities || []).join("，"));
    setFormDesc(rel.description || "");
    setShowFormModal(true);
  }

  // 保存录入 / 修改
  function handleSaveRelation() {
    if (!formTarget.trim()) {
      alert("请选择或填写关联的目标业务系统名称！");
      return;
    }
    if (!formSource.trim()) {
      alert("请指定源业务系统！");
      return;
    }
    if (formSource.trim() === formTarget.trim()) {
      alert("源业务系统与目标业务系统不能相同！");
      return;
    }

    const dataEnts = formDataEntities
      .split(/[,，\n]/)
      .map(s => s.trim())
      .filter(Boolean);

    if (formMode === "create") {
      const newEntry: BusinessSystemRelation = {
        id: `rel-custom-${Date.now()}-${Math.floor(Math.random() * 1000)}`,
        sourceSystem: formSource.trim(),
        targetSystem: formTarget.trim(),
        relationType: formType,
        relationTypeLabel: formLabel.trim() || (formType === "upstream" ? "上游业务入口" : formType === "downstream" ? "下游监管处理" : "协同联动业务"),
        direction: formDirection,
        description: formDesc.trim() || `【${formSource.trim()}】与【${formTarget.trim()}】存在业务交互协同链路。`,
        interfaceProtocol: formProtocol.trim() || "RESTful API (HTTPS/JSON)",
        frequency: formFrequency,
        dataEntities: dataEnts.length > 0 ? dataEnts : ["业务主数据交换", "状态流水同步"],
        businessRole: `${formTarget.trim()}协同对接端`,
        status: "normal"
      };

      const updated = [newEntry, ...relations];
      updateRelations(updated);
      showToast(`✅ 成功录入与【${newEntry.targetSystem}】的业务关联！`);
    } else if (formMode === "edit" && editingRelId) {
      const updated = relations.map(r => {
        if (r.id === editingRelId) {
          return {
            ...r,
            sourceSystem: formSource.trim(),
            targetSystem: formTarget.trim(),
            relationType: formType,
            relationTypeLabel: formLabel.trim() || r.relationTypeLabel,
            direction: formDirection,
            interfaceProtocol: formProtocol.trim() || r.interfaceProtocol,
            frequency: formFrequency,
            dataEntities: dataEnts.length > 0 ? dataEnts : r.dataEntities,
            description: formDesc.trim() || r.description
          };
        }
        return r;
      });
      updateRelations(updated);
      showToast(`✅ 成功修改与【${formTarget.trim()}】的业务关联配置！`);

      // 若当前抽屉正在查看此节点，同步更新
      if (inspectedNode && inspectedNode.relationData && inspectedNode.relationData.id === editingRelId) {
        setInspectedNode(prev => prev ? {
          ...prev,
          title: formTarget.trim(),
          detail: formDesc.trim(),
          extra: `交互协议: ${formProtocol} · 频次: ${formFrequency} · 交互实体: ${dataEnts.join("、")}`
        } : null);
      }
    }

    setShowFormModal(false);
  }

  // 执行删除操作
  function handleConfirmDelete() {
    if (!deletingRelation) return;
    const targetId = deletingRelation.id;
    const targetName = deletingRelation.targetSystem;
    const sourceName = deletingRelation.sourceSystem;

    // 从 relations 中过滤（支持正向与反向衍生删除）
    const updated = relations.filter(r => r.id !== targetId && !(r.sourceSystem === sourceName && r.targetSystem === targetName));
    updateRelations(updated);

    if (inspectedNode && inspectedNode.relationData && (inspectedNode.relationData.id === targetId || inspectedNode.relationData.targetSystem === targetName)) {
      setInspectedNode(null);
    }

    setDeletingRelation(null);
    showToast(`🗑️ 已成功移除与【${targetName}】的业务关联关系`);
  }

  // 恢复出厂默认预置关联
  function handleResetDefaultRelations() {
    if (confirm("确定要恢复出厂默认预置的所有业务系统关联吗？这将重置所有自定义增删改。")) {
      updateRelations(PRESET_BUSINESS_RELATIONS);
      showToast("🔄 已成功重置为标准预置业务关联关系库！");
    }
  }

  // 导出所有关联关系数据为 JSON
  function handleExportRelationsJson() {
    const dataStr = "data:text/json;charset=utf-8," + encodeURIComponent(JSON.stringify(relations, null, 2));
    const downloadAnchor = document.createElement("a");
    downloadAnchor.setAttribute("href", dataStr);
    downloadAnchor.setAttribute("download", `autoops_business_relations_${new Date().toISOString().slice(0, 10)}.json`);
    document.body.appendChild(downloadAnchor);
    downloadAnchor.click();
    downloadAnchor.remove();
    showToast("📋 业务系统关联拓扑数据已成功导出为 JSON");
  }

  // ================= 基础设施各层关联数据计算 =================
  const linkedVms = useMemo(() => {
    if (!selectedBiz) return [];
    const matched = vms.filter(v => v && (v.projectName === selectedBiz.name || v.businessId === selectedBiz.id));
    if (matched.length > 0) return matched;
    return vms.slice(0, 8);
  }, [vms, selectedBiz]);

  const linkedDbs = useMemo(() => {
    if (!selectedBiz) return [];
    const matched = databases.filter(d => d && (d.projectName === selectedBiz.name || d.businessId === selectedBiz.id));
    if (matched.length > 0) return matched;
    return databases.slice(0, 4);
  }, [databases, selectedBiz]);

  const linkedMws = useMemo(() => {
    if (!selectedBiz) return [];
    const matched = (middlewares || []).filter(m => m && (m.projectName === selectedBiz.name || m.projectId === selectedBiz.id));
    if (matched.length > 0) return matched;
    return (middlewares || []).slice(0, 4);
  }, [middlewares, selectedBiz]);

  const linkedBks = useMemo(() => {
    if (!selectedBiz) return [];
    const matched = (backups || []).filter(b => b && (b.projectName === selectedBiz.name || b.projectId === selectedBiz.id));
    if (matched.length > 0) return matched;
    return (backups || []).slice(0, 3);
  }, [backups, selectedBiz]);

  const linkedSwitches = useMemo(() => {
    if (!selectedBiz) return [];
    const sws = switches.filter(s => s && (s.projectName === selectedBiz.name || s.businessId === selectedBiz.id));
    return sws;
  }, [switches, selectedBiz]);

  const linkedHosts = useMemo(() => {
    if (!selectedBiz) return [];
    const directHosts = hosts.filter(h => h && (h.projectName === selectedBiz.name || h.businessId === selectedBiz.id));
    if (directHosts.length > 0) return directHosts;
    const parentHostId = linkedVms[0]?.physicalHostId;
    const parentHost = hosts.find(h => h && h.id === parentHostId);
    return parentHost ? [parentHost] : hosts.slice(0, 3);
  }, [hosts, selectedBiz, linkedVms]);

  return (
    <div className="cmdb-container" style={{ display: "flex", flexDirection: "column", gap: 14, minHeight: "100%", height: "auto", paddingBottom: 40, position: "relative" }}>
      
      {/* Toast Notification Alert */}
      {toastMessage && (
        <div style={{
          position: "fixed",
          top: 20,
          right: 24,
          background: "#0f172a",
          border: "1.5px solid #22c55e",
          color: "#f8fafc",
          padding: "10px 18px",
          borderRadius: 8,
          fontSize: 13,
          fontWeight: 600,
          boxShadow: "0 10px 25px rgba(0,0,0,0.4)",
          zIndex: 9999,
          display: "flex",
          alignItems: "center",
          gap: 8,
          animation: "fadeIn 0.2s ease"
        }}>
          <span>{toastMessage}</span>
        </div>
      )}

      {/* ================= 1. TOP CONTROL BAR ================= */}
      <div style={{
        background: "#ffffff",
        border: "1px solid #e2e8f0",
        borderRadius: 8,
        padding: "10px 16px",
        display: "flex",
        justifyContent: "space-between",
        alignItems: "center",
        flexWrap: "wrap",
        gap: 12,
        boxShadow: "0 1px 3px rgba(0,0,0,0.03)"
      }}>
        {/* Left: Project Selector Dropdown & Switch Buttons */}
        <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
          <span style={{ fontWeight: 700, color: "#0f172a", fontSize: 15, display: "flex", alignItems: "center", gap: 6 }}>
            🏛 业务拓扑切换:
          </span>

          {/* Project Direct Dropdown */}
          <select 
            value={selectedBiz.id}
            onChange={e => {
              setSelectedBizId(e.target.value);
              setInspectedNode(null);
            }}
            style={{
              padding: "6px 12px",
              fontSize: 13,
              fontWeight: 600,
              color: "#1e3a8a",
              background: "#eff6ff",
              border: "1.5px solid #3b82f6",
              borderRadius: 6,
              maxWidth: 320,
              outline: "none",
              cursor: "pointer"
            }}
          >
            {filteredBusinesses.map(b => (
              <option key={b.id} value={b.id}>
                {b.name} ({b.department || "未归属"})
              </option>
            ))}
          </select>

          {/* Prev / Next Quick Navigators */}
          <div style={{ display: "flex", gap: 4 }}>
            <button 
              className="btn-secondary" 
              style={{ padding: "5px 10px", fontSize: 12 }}
              onClick={handlePrevProject}
              title="切换到上一个项目拓扑"
            >
              ‹ 上一个
            </button>
            <button 
              className="btn-secondary" 
              style={{ padding: "5px 10px", fontSize: 12 }}
              onClick={handleNextProject}
              title="切换到下一个项目拓扑"
            >
              下一个 ›
            </button>
          </div>

          {/* Cluster Filter Buttons */}
          <div style={{ display: "flex", gap: 4, background: "#f1f5f9", padding: "3px 4px", borderRadius: 6 }}>
            <button
              onClick={() => setClusterFilter("all")}
              style={{
                border: 0,
                fontSize: 11,
                padding: "3px 8px",
                borderRadius: 4,
                cursor: "pointer",
                background: clusterFilter === "all" ? "#2563eb" : "transparent",
                color: clusterFilter === "all" ? "#fff" : "#64748b",
                fontWeight: clusterFilter === "all" ? 600 : 400
              }}
            >
              全部
            </button>
            <button
              onClick={() => setClusterFilter("rs_zc")}
              style={{
                border: 0,
                fontSize: 11,
                padding: "3px 8px",
                borderRadius: 4,
                cursor: "pointer",
                background: clusterFilter === "rs_zc" ? "#2563eb" : "transparent",
                color: clusterFilter === "rs_zc" ? "#fff" : "#64748b",
                fontWeight: clusterFilter === "rs_zc" ? 600 : 400
              }}
              title="快速筛选仲裁业务集群"
            >
              ⚖️ 仲裁集群
            </button>
            <button
              onClick={() => setClusterFilter("rs_sb")}
              style={{
                border: 0,
                fontSize: 11,
                padding: "3px 8px",
                borderRadius: 4,
                cursor: "pointer",
                background: clusterFilter === "rs_sb" ? "#2563eb" : "transparent",
                color: clusterFilter === "rs_sb" ? "#fff" : "#64748b",
                fontWeight: clusterFilter === "rs_sb" ? 600 : 400
              }}
              title="快速筛选社保/工伤/监管集群"
            >
              🛡️ 社保工伤集群
            </button>
            <button
              onClick={() => setClusterFilter("bk")}
              style={{
                border: 0,
                fontSize: 11,
                padding: "3px 8px",
                borderRadius: 4,
                cursor: "pointer",
                background: clusterFilter === "bk" ? "#2563eb" : "transparent",
                color: clusterFilter === "bk" ? "#fff" : "#64748b",
                fontWeight: clusterFilter === "bk" ? 600 : 400
              }}
              title="快速筛选北控板块集群"
            >
              🏢 北控板块
            </button>
          </div>

          {/* View Mode Switcher */}
          <div style={{ display: "flex", gap: 2, background: "#e2e8f0", padding: "2px", borderRadius: 6, marginLeft: 4 }}>
            <button
              onClick={() => setViewMode("layered")}
              style={{
                border: 0,
                padding: "4px 10px",
                borderRadius: 4,
                fontSize: 12,
                cursor: "pointer",
                fontWeight: 600,
                background: viewMode === "layered" ? "#ffffff" : "transparent",
                color: viewMode === "layered" ? "#0f172a" : "#64748b",
                boxShadow: viewMode === "layered" ? "0 1px 2px rgba(0,0,0,0.1)" : "none"
              }}
            >
              📊 基础设施纵深架构
            </button>
            <button
              onClick={() => setViewMode("cross_business")}
              style={{
                border: 0,
                padding: "4px 10px",
                borderRadius: 4,
                fontSize: 12,
                cursor: "pointer",
                fontWeight: 600,
                background: viewMode === "cross_business" ? "#ffffff" : "transparent",
                color: viewMode === "cross_business" ? "#2563eb" : "#64748b",
                boxShadow: viewMode === "cross_business" ? "0 1px 2px rgba(0,0,0,0.1)" : "none"
              }}
            >
              🌐 跨业务系统协同拓扑
            </button>
          </div>

          {/* CRUD Actions Buttons */}
          <button
            className="btn-secondary"
            style={{ padding: "5px 12px", fontSize: 12, color: "#16a34a", borderColor: "#86efac", background: "#f0fdf4", fontWeight: 700 }}
            onClick={() => handleOpenCreateModal()}
            title="为当前业务系统录入新的关联业务"
          >
            ➕ 录入相关业务
          </button>

          <button
            className="btn-secondary"
            style={{ padding: "5px 10px", fontSize: 12, color: "#475569", borderColor: "#cbd5e1", background: "#f8fafc" }}
            onClick={() => setShowManageModal(true)}
            title="集中管理全系统所有业务关联关系清单 (增/删/改/导出)"
          >
            ⚙️ 业务关联清单
          </button>

          {onNavigateToProject && (
            <button
              className="btn-secondary"
              style={{ padding: "5px 12px", fontSize: 12, color: "#2563eb", borderColor: "#93c5fd", background: "#f0f9ff" }}
              onClick={() => onNavigateToProject("项目资产")}
              title="前往该项目查看明细资产清单"
            >
              📋 查看项目资产台账
            </button>
          )}
        </div>

        {/* Right: Search */}
        <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
          <input 
            placeholder="搜索项目名称 / 客户单位..." 
            value={searchKeyword}
            onChange={e => setSearchKeyword(e.target.value)}
            style={{ width: 170, fontSize: 12, padding: "5px 10px", borderRadius: 4, border: "1px solid #cbd5e1" }}
          />

          <span style={{ fontSize: 12, color: "#64748b" }}>
            共 {filteredBusinesses.length} 个业务系统
          </span>
        </div>
      </div>

      {/* ================= 2. ACTIVE PROJECT SHOWCASE BANNER ================= */}
      <div style={{
        background: "linear-gradient(135deg, #0f172a 0%, #1e293b 100%)",
        border: "1px solid #334155",
        borderRadius: 8,
        padding: "16px 20px",
        color: "#fff",
        boxShadow: "0 4px 12px rgba(0,0,0,0.12)"
      }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", flexWrap: "wrap", gap: 14 }}>
          {/* Main Info */}
          <div style={{ flex: 1, minWidth: 280 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 8, flexWrap: "wrap" }}>
              <h2 style={{ margin: 0, fontSize: 18, color: "#f8fafc", fontWeight: 700 }}>
                {selectedBiz.name}
              </h2>
              <span style={{ fontSize: 11, background: "rgba(56,189,248,0.2)", color: "#38bdf8", padding: "2px 8px", borderRadius: 4, border: "1px solid rgba(56,189,248,0.3)" }}>
                ● {selectedBiz.status || "稳定运行"}
              </span>
              <span style={{ fontSize: 11, background: "rgba(168,85,247,0.2)", color: "#c084fc", padding: "2px 8px", borderRadius: 4, border: "1px solid rgba(168,85,247,0.3)" }}>
                🔗 关联业务: {relatedSystems.all.length} 个系统
              </span>
            </div>

            <p style={{ margin: "0 0 8px", fontSize: 12, color: "#cbd5e1", lineHeight: 1.5 }}>
              {selectedBiz.description || "全景多维IT资产业务架构支撑系统"}
            </p>

            <div style={{ display: "flex", gap: 16, fontSize: 11, color: "#94a3b8", flexWrap: "wrap" }}>
              <span>客户单位: <strong style={{ color: "#e2e8f0" }}>{selectedBiz.department}</strong></span>
              <span>系统编码: <code style={{ color: "#38bdf8" }}>{selectedBiz.code}</code></span>
              <span>保障团队: <strong style={{ color: "#e2e8f0" }}>{selectedBiz.manager || "运维保障组"}</strong></span>
            </div>

            {/* Quick Related Systems Pill Bar */}
            {relatedSystems.all.length > 0 && (
              <div style={{ marginTop: 12, display: "flex", alignItems: "center", gap: 6, flexWrap: "wrap" }}>
                <span style={{ fontSize: 11, color: "#38bdf8", fontWeight: 600 }}>🔗 相关业务快速直达:</span>
                {relatedSystems.all.map(rel => (
                  <div
                    key={rel.id}
                    style={{
                      border: "1px solid rgba(56,189,248,0.35)",
                      background: "rgba(56,189,248,0.08)",
                      color: "#e0f2fe",
                      fontSize: 11,
                      padding: "2px 8px",
                      borderRadius: 12,
                      display: "flex",
                      alignItems: "center",
                      gap: 4
                    }}
                  >
                    <button
                      onClick={() => handleSwitchToSystem(rel.targetSystem)}
                      style={{
                        background: "transparent",
                        border: 0,
                        color: "#e0f2fe",
                        cursor: "pointer",
                        padding: 0,
                        display: "flex",
                        alignItems: "center",
                        gap: 3,
                        fontSize: 11
                      }}
                      title={`点击切换到【${rel.targetSystem}】拓扑 · 角色: ${rel.relationTypeLabel}`}
                    >
                      <span>{rel.relationType === "upstream" ? "📥" : rel.relationType === "downstream" || rel.relationType === "shared_hub" ? "📤" : "🔄"}</span>
                      <span>{rel.targetSystem}</span>
                    </button>
                    <span style={{ fontSize: 10, color: "#93c5fd" }}>({rel.relationTypeLabel})</span>

                    {/* Quick Edit & Delete icons */}
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleOpenEditModal(rel);
                      }}
                      style={{ background: "transparent", border: 0, color: "#38bdf8", cursor: "pointer", fontSize: 10, padding: "0 2px" }}
                      title="编辑此关联关系"
                    >
                      ✏️
                    </button>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        setDeletingRelation(rel);
                      }}
                      style={{ background: "transparent", border: 0, color: "#f87171", cursor: "pointer", fontSize: 10, padding: "0 2px" }}
                      title="移除此关联关系"
                    >
                      🗑️
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Quick Metrics Badges */}
          <div style={{ display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap" }}>
            <div style={{ background: "rgba(255,255,255,0.06)", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 6, padding: "8px 12px", textAlign: "center" }}>
              <span style={{ fontSize: 10, color: "#94a3b8", display: "block" }}>相关协同业务</span>
              <strong style={{ fontSize: 16, color: "#c084fc" }}>{relatedSystems.all.length} 个</strong>
            </div>

            <div style={{ background: "rgba(255,255,255,0.06)", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 6, padding: "8px 12px", textAlign: "center" }}>
              <span style={{ fontSize: 10, color: "#94a3b8", display: "block" }}>云主机节点</span>
              <strong style={{ fontSize: 16, color: "#38bdf8" }}>{linkedVms.length} 台</strong>
            </div>

            <div style={{ background: "rgba(255,255,255,0.06)", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 6, padding: "8px 12px", textAlign: "center" }}>
              <span style={{ fontSize: 10, color: "#94a3b8", display: "block" }}>中间件服务</span>
              <strong style={{ fontSize: 16, color: "#fb923c" }}>{linkedMws.length} 个</strong>
            </div>

            <div style={{ background: "rgba(255,255,255,0.06)", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 6, padding: "8px 12px", textAlign: "center" }}>
              <span style={{ fontSize: 10, color: "#94a3b8", display: "block" }}>核心数据库</span>
              <strong style={{ fontSize: 16, color: "#f59e0b" }}>{linkedDbs.length} 个</strong>
            </div>

            <div style={{ background: "rgba(255,255,255,0.06)", border: "1px solid rgba(255,255,255,0.1)", borderRadius: 6, padding: "8px 12px", textAlign: "center" }}>
              <span style={{ fontSize: 10, color: "#94a3b8", display: "block" }}>备份容灾</span>
              <strong style={{ fontSize: 16, color: "#22c55e" }}>{linkedBks.length} 项</strong>
            </div>
          </div>
        </div>
      </div>

      {/* ================= 3. TOPOLOGY VIEW (LAYERED OR CROSS-BUSINESS) ================= */}
      {viewMode === "layered" ? (
        /* ================= 3A. 分层纵深拓扑视图 (含上下游相关业务) ================= */
        <div className="topology-view" style={{
          height: "auto",
          minHeight: "auto",
          background: "linear-gradient(180deg, #0b132b 0%, #070c1b 100%)",
          border: "1.5px solid #1e293b",
          borderRadius: 14,
          padding: "24px 26px 32px",
          boxShadow: "0 10px 32px rgba(0, 0, 0, 0.45)",
          position: "relative",
          overflow: "visible",
          marginBottom: 20
        }}>
          <header className="topology-header" style={{ marginBottom: 18, borderBottom: "1px solid rgba(148, 163, 184, 0.12)", paddingBottom: 14, display: "flex", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: 10 }}>
            <div>
              <h4 style={{ margin: 0, fontSize: 16, color: "#38bdf8", fontWeight: 700, display: "flex", alignItems: "center", gap: 8 }}>
                <span>🕸</span>
                <span>【{selectedBiz.name}】全链路业务协同与IT架构依赖拓扑图</span>
              </h4>
              <p style={{ margin: "4px 0 0", fontSize: 12, color: "#94a3b8" }}>
                包含业务系统上下游协同网（相关业务可录入/修改/删除）、接入网关、主机计算、应用中间件、数据库及底层容灾
              </p>
            </div>

            <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
              <button
                onClick={() => handleOpenCreateModal()}
                style={{
                  background: "#16a34a",
                  color: "#fff",
                  border: 0,
                  fontSize: 12,
                  fontWeight: 600,
                  padding: "4px 12px",
                  borderRadius: 6,
                  cursor: "pointer"
                }}
              >
                ➕ 录入关联业务
              </button>
              <button
                onClick={() => setShowManageModal(true)}
                style={{
                  background: "rgba(148, 163, 184, 0.2)",
                  color: "#cbd5e1",
                  border: "1px solid rgba(148, 163, 184, 0.3)",
                  fontSize: 12,
                  padding: "4px 10px",
                  borderRadius: 6,
                  cursor: "pointer"
                }}
              >
                ⚙️ 关系管理
              </button>
            </div>
          </header>

          <div className="topology-layers" style={{ display: "flex", flexDirection: "column", gap: 14 }}>
            {/* LEVEL 0: RELATED BUSINESS SYSTEMS (相关业务系统生态层) */}
            <div style={{
              background: "rgba(30, 41, 59, 0.5)",
              border: "1.5px solid rgba(147, 197, 253, 0.25)",
              borderRadius: 10,
              padding: "16px 18px",
              boxShadow: "inset 0 1px 3px rgba(0,0,0,0.2)"
            }}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12, flexWrap: "wrap", gap: 8 }}>
                <div style={{ color: "#38bdf8", fontWeight: 700, fontSize: 13, display: "flex", alignItems: "center", gap: 6 }}>
                  <span>🌐</span>
                  <span>LEVEL 0 · 相关业务系统与上下游协同拓扑域 (RELATED BUSINESS SYSTEMS · 共 {relatedSystems.all.length} 个关联系统)</span>
                </div>

                <div style={{ display: "flex", gap: 8, alignItems: "center", fontSize: 11, color: "#94a3b8" }}>
                  <span>💡 支持点击卡片查看协议与流向，右上角可直接 ✏️修改 或 🗑️删除 关联</span>
                </div>
              </div>

              {/* 3-Section Layout: Upstream / Peer / Downstream */}
              <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(280px, 1fr))", gap: 12 }}>
                {/* 1. 上游输入业务系统 */}
                <div style={{ background: "rgba(15, 23, 42, 0.7)", border: "1px solid rgba(56, 189, 248, 0.3)", borderRadius: 8, padding: "12px 14px" }}>
                  <div style={{ color: "#38bdf8", fontSize: 12, fontWeight: 700, marginBottom: 8, display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                    <span>📥 上游输入 / 前置申报系统 ({relatedSystems.upstream.length})</span>
                    <button
                      onClick={() => handleOpenCreateModal("upstream")}
                      style={{ fontSize: 10, color: "#38bdf8", background: "rgba(56,189,248,0.15)", border: "1px solid rgba(56,189,248,0.3)", padding: "1px 6px", borderRadius: 4, cursor: "pointer" }}
                      title="快速添加上游输入业务系统"
                    >
                      + 录入上游
                    </button>
                  </div>

                  <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                    {relatedSystems.upstream.length > 0 ? (
                      relatedSystems.upstream.map(rel => (
                        <div
                          key={rel.id}
                          className="topology-node-card"
                          style={{
                            background: "linear-gradient(135deg, #1e293b, #0f172a)",
                            border: "1px solid rgba(56, 189, 248, 0.4)",
                            cursor: "pointer",
                            padding: "8px 10px",
                            position: "relative"
                          }}
                          onClick={() => setInspectedNode({
                            title: rel.targetSystem,
                            type: `上游关联业务 · ${rel.relationTypeLabel}`,
                            detail: rel.description,
                            extra: `交互协议: ${rel.interfaceProtocol} · 频次: ${rel.frequency} · 交互实体: ${rel.dataEntities.join("、")}`,
                            relationData: rel,
                            targetSystemName: rel.targetSystem
                          })}
                        >
                          <div className="topology-node-icon app" style={{ fontSize: 18, background: "#0284c7" }}>📥</div>
                          <div className="topology-node-meta" style={{ flex: 1 }}>
                            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                              <strong style={{ color: "#f8fafc", fontSize: 13 }}>{rel.targetSystem}</strong>
                              
                              {/* 操作按钮区 */}
                              <div style={{ display: "flex", gap: 3 }} onClick={e => e.stopPropagation()}>
                                <button
                                  onClick={() => handleOpenEditModal(rel)}
                                  style={{ background: "rgba(56,189,248,0.15)", border: "1px solid rgba(56,189,248,0.3)", color: "#38bdf8", padding: "1px 5px", borderRadius: 3, fontSize: 10, cursor: "pointer" }}
                                  title="修改此业务关联配置"
                                >
                                  ✏️
                                </button>
                                <button
                                  onClick={() => setDeletingRelation(rel)}
                                  style={{ background: "rgba(239,68,68,0.15)", border: "1px solid rgba(239,68,68,0.3)", color: "#f87171", padding: "1px 5px", borderRadius: 3, fontSize: 10, cursor: "pointer" }}
                                  title="移除此业务关联"
                                >
                                  🗑️
                                </button>
                              </div>
                            </div>

                            <small style={{ color: "#94a3b8", display: "block", marginTop: 2 }}>{rel.relationTypeLabel}</small>
                            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginTop: 4 }}>
                              <span style={{ fontSize: 10, color: "#38bdf8" }}>➔ 推送数据至本系统</span>
                              <button
                                onClick={(e) => {
                                  e.stopPropagation();
                                  handleSwitchToSystem(rel.targetSystem);
                                }}
                                style={{
                                  border: "1px solid #0284c7",
                                  background: "#0284c7",
                                  color: "#fff",
                                  fontSize: 10,
                                  padding: "1px 6px",
                                  borderRadius: 3,
                                  cursor: "pointer"
                                }}
                                title="切换到该系统拓扑"
                              >
                                ⚡ 查看此系统
                              </button>
                            </div>
                          </div>
                        </div>
                      ))
                    ) : (
                      <div style={{ fontSize: 12, color: "#64748b", padding: "10px", textAlign: "center", border: "1px dashed rgba(255,255,255,0.1)", borderRadius: 6 }}>
                        暂无独立前置申报系统 (可点击右上角录入)
                      </div>
                    )}
                  </div>
                </div>

                {/* 2. 平行协同联动业务系统 */}
                <div style={{ background: "rgba(15, 23, 42, 0.7)", border: "1px solid rgba(168, 85, 247, 0.3)", borderRadius: 8, padding: "12px 14px" }}>
                  <div style={{ color: "#c084fc", fontSize: 12, fontWeight: 700, marginBottom: 8, display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                    <span>🔄 平行协同 / 联动业务系统 ({relatedSystems.peer.length})</span>
                    <button
                      onClick={() => handleOpenCreateModal("peer")}
                      style={{ fontSize: 10, color: "#c084fc", background: "rgba(168,85,247,0.15)", border: "1px solid rgba(168,85,247,0.3)", padding: "1px 6px", borderRadius: 4, cursor: "pointer" }}
                      title="快速添加平行协同业务系统"
                    >
                      + 录入协同
                    </button>
                  </div>

                  <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                    {relatedSystems.peer.length > 0 ? (
                      relatedSystems.peer.map(rel => (
                        <div
                          key={rel.id}
                          className="topology-node-card"
                          style={{
                            background: "linear-gradient(135deg, #1e293b, #0f172a)",
                            border: "1px solid rgba(168, 85, 247, 0.4)",
                            cursor: "pointer",
                            padding: "8px 10px",
                            position: "relative"
                          }}
                          onClick={() => setInspectedNode({
                            title: rel.targetSystem,
                            type: `平行协同业务 · ${rel.relationTypeLabel}`,
                            detail: rel.description,
                            extra: `交互协议: ${rel.interfaceProtocol} · 频次: ${rel.frequency} · 交互实体: ${rel.dataEntities.join("、")}`,
                            relationData: rel,
                            targetSystemName: rel.targetSystem
                          })}
                        >
                          <div className="topology-node-icon app" style={{ fontSize: 18, background: "#7c3aed" }}>🔄</div>
                          <div className="topology-node-meta" style={{ flex: 1 }}>
                            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                              <strong style={{ color: "#f8fafc", fontSize: 13 }}>{rel.targetSystem}</strong>
                              
                              {/* 操作按钮区 */}
                              <div style={{ display: "flex", gap: 3 }} onClick={e => e.stopPropagation()}>
                                <button
                                  onClick={() => handleOpenEditModal(rel)}
                                  style={{ background: "rgba(168,85,247,0.15)", border: "1px solid rgba(168,85,247,0.3)", color: "#c084fc", padding: "1px 5px", borderRadius: 3, fontSize: 10, cursor: "pointer" }}
                                  title="修改此业务关联配置"
                                >
                                  ✏️
                                </button>
                                <button
                                  onClick={() => setDeletingRelation(rel)}
                                  style={{ background: "rgba(239,68,68,0.15)", border: "1px solid rgba(239,68,68,0.3)", color: "#f87171", padding: "1px 5px", borderRadius: 3, fontSize: 10, cursor: "pointer" }}
                                  title="移除此业务关联"
                                >
                                  🗑️
                                </button>
                              </div>
                            </div>

                            <small style={{ color: "#94a3b8", display: "block", marginTop: 2 }}>{rel.relationTypeLabel}</small>
                            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginTop: 4 }}>
                              <span style={{ fontSize: 10, color: "#c084fc" }}>⇄ 双向业务指令同步</span>
                              <button
                                onClick={(e) => {
                                  e.stopPropagation();
                                  handleSwitchToSystem(rel.targetSystem);
                                }}
                                style={{
                                  border: "1px solid #7c3aed",
                                  background: "#7c3aed",
                                  color: "#fff",
                                  fontSize: 10,
                                  padding: "1px 6px",
                                  borderRadius: 3,
                                  cursor: "pointer"
                                }}
                                title="切换到该系统拓扑"
                              >
                                ⚡ 查看此系统
                              </button>
                            </div>
                          </div>
                        </div>
                      ))
                    ) : (
                      <div style={{ fontSize: 12, color: "#64748b", padding: "10px", textAlign: "center", border: "1px dashed rgba(255,255,255,0.1)", borderRadius: 6 }}>
                        暂无横向联动系统 (可点击右上角录入)
                      </div>
                    )}
                  </div>
                </div>

                {/* 3. 下游监管与数据中台 */}
                <div style={{ background: "rgba(15, 23, 42, 0.7)", border: "1px solid rgba(34, 197, 94, 0.3)", borderRadius: 8, padding: "12px 14px" }}>
                  <div style={{ color: "#4ade80", fontSize: 12, fontWeight: 700, marginBottom: 8, display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                    <span>📤 下游监管 / 数据要素中台 ({relatedSystems.downstream.length})</span>
                    <button
                      onClick={() => handleOpenCreateModal("downstream")}
                      style={{ fontSize: 10, color: "#4ade80", background: "rgba(34,197,94,0.15)", border: "1px solid rgba(34,197,94,0.3)", padding: "1px 6px", borderRadius: 4, cursor: "pointer" }}
                      title="快速添加下游监管业务系统"
                    >
                      + 录入下游
                    </button>
                  </div>

                  <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                    {relatedSystems.downstream.length > 0 ? (
                      relatedSystems.downstream.map(rel => (
                        <div
                          key={rel.id}
                          className="topology-node-card"
                          style={{
                            background: "linear-gradient(135deg, #1e293b, #0f172a)",
                            border: "1px solid rgba(34, 197, 94, 0.4)",
                            cursor: "pointer",
                            padding: "8px 10px",
                            position: "relative"
                          }}
                          onClick={() => setInspectedNode({
                            title: rel.targetSystem,
                            type: `下游监管/中台 · ${rel.relationTypeLabel}`,
                            detail: rel.description,
                            extra: `交互协议: ${rel.interfaceProtocol} · 频次: ${rel.frequency} · 交互实体: ${rel.dataEntities.join("、")}`,
                            relationData: rel,
                            targetSystemName: rel.targetSystem
                          })}
                        >
                          <div className="topology-node-icon app" style={{ fontSize: 18, background: "#059669" }}>📤</div>
                          <div className="topology-node-meta" style={{ flex: 1 }}>
                            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                              <strong style={{ color: "#f8fafc", fontSize: 13 }}>{rel.targetSystem}</strong>
                              
                              {/* 操作按钮区 */}
                              <div style={{ display: "flex", gap: 3 }} onClick={e => e.stopPropagation()}>
                                <button
                                  onClick={() => handleOpenEditModal(rel)}
                                  style={{ background: "rgba(34,197,94,0.15)", border: "1px solid rgba(34,197,94,0.3)", color: "#4ade80", padding: "1px 5px", borderRadius: 3, fontSize: 10, cursor: "pointer" }}
                                  title="修改此业务关联配置"
                                >
                                  ✏️
                                </button>
                                <button
                                  onClick={() => setDeletingRelation(rel)}
                                  style={{ background: "rgba(239,68,68,0.15)", border: "1px solid rgba(239,68,68,0.3)", color: "#f87171", padding: "1px 5px", borderRadius: 3, fontSize: 10, cursor: "pointer" }}
                                  title="移除此业务关联"
                                >
                                  🗑️
                                </button>
                              </div>
                            </div>

                            <small style={{ color: "#94a3b8", display: "block", marginTop: 2 }}>{rel.relationTypeLabel}</small>
                            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginTop: 4 }}>
                              <span style={{ fontSize: 10, color: "#4ade80" }}>➔ 汇总流转至中台</span>
                              <button
                                onClick={(e) => {
                                  e.stopPropagation();
                                  handleSwitchToSystem(rel.targetSystem);
                                }}
                                style={{
                                  border: "1px solid #059669",
                                  background: "#059669",
                                  color: "#fff",
                                  fontSize: 10,
                                  padding: "1px 6px",
                                  borderRadius: 3,
                                  cursor: "pointer"
                                }}
                                title="切换到该系统拓扑"
                              >
                                ⚡ 查看此系统
                              </button>
                            </div>
                          </div>
                        </div>
                      ))
                    ) : (
                      <div style={{ fontSize: 12, color: "#64748b", padding: "10px", textAlign: "center", border: "1px dashed rgba(255,255,255,0.1)", borderRadius: 6 }}>
                        暂无下游监管推送系统 (可点击右上角录入)
                      </div>
                    )}
                  </div>
                </div>
              </div>
            </div>

            {/* Flow Connector 0 -> 1 */}
            <div style={{ display: "flex", alignItems: "center", gap: 8, margin: "2px 0 2px 24px", color: "#38bdf8", fontSize: 11, fontWeight: 600 }}>
              <span>↓</span>
              <span style={{ background: "rgba(56,189,248,0.12)", border: "1px solid rgba(56,189,248,0.25)", padding: "2px 10px", borderRadius: 12 }}>
                业务中枢调度 · 当前核心业务生产系统承载
              </span>
            </div>

            {/* LEVEL 1: BUSINESS SYSTEM */}
            <div>
              <div className="topology-layer-title" style={{ color: "#38bdf8", fontWeight: 700 }}>
                LEVEL 1 · 业务应用系统核心中枢 (CORE BUSINESS APPLICATION)
              </div>
              <div className="topology-nodes-row">
                <div 
                  className="topology-node-card"
                  style={{
                    background: "linear-gradient(135deg, #1e293b, #0f172a)",
                    border: "2px solid #38bdf8",
                    cursor: "pointer",
                    boxShadow: "0 0 16px rgba(56, 189, 248, 0.25)",
                    padding: "12px 16px"
                  }}
                  onClick={() => setInspectedNode({
                    title: selectedBiz.name,
                    type: "核心业务生产系统",
                    detail: selectedBiz.description,
                    extra: `编码: ${selectedBiz.code} · 组织归属: ${selectedBiz.department} · 责任人: ${selectedBiz.manager || "保障组"}`
                  })}
                >
                  <div className="topology-node-icon app" style={{ fontSize: 24, background: "#0284c7" }}>🏛</div>
                  <div className="topology-node-meta">
                    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                      <strong style={{ fontSize: 15, color: "#f8fafc" }}>{selectedBiz.name}</strong>
                      <span style={{ fontSize: 11, background: "#2563eb", color: "#fff", padding: "1px 6px", borderRadius: 3 }}>
                        当前聚焦主体
                      </span>
                    </div>
                    <small style={{ color: "#cbd5e1", marginTop: 4 }}>
                      核心业务生产系统 · 客户单位: {selectedBiz.department} · 关联 {relatedSystems.all.length} 个协同系统
                    </small>
                  </div>
                </div>
              </div>
            </div>

            {/* Flow Connector 1 -> 2 */}
            <div style={{ display: "flex", alignItems: "center", gap: 8, margin: "2px 0 2px 24px", color: "#22c55e", fontSize: 11, fontWeight: 600 }}>
              <span>↓</span>
              <span style={{ background: "rgba(34,197,94,0.12)", border: "1px solid rgba(34,197,94,0.25)", padding: "2px 10px", borderRadius: 12 }}>
                统一安全隔离接入 · 网关路由与 SLB 负载均衡分发链路
              </span>
            </div>

            {/* LEVEL 2: NETWORK & GATEWAY / LOAD BALANCER */}
            <div>
              <div className="topology-layer-title" style={{ color: "#22c55e", fontWeight: 700 }}>
                LEVEL 2 · 网络接入与高可用网关层 (GATEWAY & NETWORK ACCESS LAYER)
              </div>
              <div className="topology-nodes-row" style={{ flexWrap: "wrap", gap: 10 }}>
                {linkedSwitches.length > 0 ? (
                  linkedSwitches.map(sw => (
                    <div 
                      key={sw.id} 
                      className="topology-node-card"
                      style={{ background: "linear-gradient(135deg, #1e293b, #0f172a)", border: "1.5px solid rgba(34, 197, 94, 0.4)", cursor: "pointer" }}
                      onClick={() => setInspectedNode({
                        title: sw.name,
                        type: "网络 / 负载均衡设备",
                        detail: `管理IP: ${sw.ip} · 端口总数: ${sw.portCount} (活跃 ${sw.activePorts}) · 角色: ${sw.role}`,
                        extra: `品牌型号: ${sw.brand} ${sw.model} · 项目专有网络配置`
                      })}
                    >
                      <div className="topology-node-icon net" style={{ fontSize: 20 }}>🌐</div>
                      <div className="topology-node-meta">
                        <strong style={{ color: "#f8fafc" }}>{sw.name}</strong>
                        <small style={{ color: "#86efac" }}>{sw.ip} · {sw.role}</small>
                      </div>
                    </div>
                  ))
                ) : (
                  <div 
                    className="topology-node-card"
                    style={{ background: "linear-gradient(135deg, #1e293b, #0f172a)", border: "1.5px solid rgba(34, 197, 94, 0.4)", cursor: "pointer" }}
                    onClick={() => setInspectedNode({
                      title: `${selectedBiz.name} · VPC 安全专网网关`,
                      type: "虚拟私有云 / 路由网关",
                      detail: "政务外网 / 互联网区专网接入路由与安全隔离域",
                      extra: "支持 VIP 虚IP 漂移与安全组流量接入策略"
                    })}
                  >
                    <div className="topology-node-icon net" style={{ fontSize: 20 }}>🛡️</div>
                    <div className="topology-node-meta">
                      <strong style={{ color: "#f8fafc" }}>政务专网 / VPC 专有安全网关</strong>
                      <small style={{ color: "#86efac" }}>安全隔离接入 · 跨业务调用流向过滤</small>
                    </div>
                  </div>
                )}
              </div>
            </div>

            {/* Flow Connector 2 -> 3 */}
            <div style={{ display: "flex", alignItems: "center", gap: 8, margin: "2px 0 2px 24px", color: "#60a5fa", fontSize: 11, fontWeight: 600 }}>
              <span>↓</span>
              <span style={{ background: "rgba(96,165,250,0.12)", border: "1px solid rgba(96,165,250,0.25)", padding: "2px 10px", borderRadius: 12 }}>
                业务计算承载 · 虚拟云主机集群及国产操作系统
              </span>
            </div>

            {/* LEVEL 3: VIRTUAL MACHINE APPLICATION NODES */}
            <div>
              <div className="topology-layer-title" style={{ color: "#60a5fa", fontWeight: 700 }}>
                LEVEL 3 · 虚拟云主机应用集群层 (ECS INSTANCES · 共 {linkedVms.length} 节点)
              </div>
              <div className="topology-nodes-row" style={{ flexWrap: "wrap", gap: 10 }}>
                {linkedVms.slice(0, 10).map(vm => (
                  <div 
                    key={vm.id}
                    className="topology-node-card"
                    style={{ background: "linear-gradient(135deg, #1e293b, #0f172a)", border: "1.5px solid rgba(96, 165, 250, 0.4)", cursor: "pointer" }}
                    onClick={() => setInspectedNode({
                      title: vm.name,
                      type: "虚拟云服务器 (VM/ECS)",
                      detail: `业务IP: ${vm.privateIp || vm.ip} · 内大网IP: ${vm.internalWanIp || "-"} · OS: ${vm.os}`,
                      extra: `算力规格: ${vm.cpu} · 内存: ${vm.memory} · 磁盘: ${vm.disk} · 信创OS: ${vm.isXinchuang || "否"} · 端口: ${vm.remotePort || 22}`
                    })}
                  >
                    <div className="topology-node-icon srv" style={{ background: "#0284c7", fontSize: 18 }}>☁️</div>
                    <div className="topology-node-meta">
                      <strong style={{ color: "#f8fafc" }}>{vm.name}</strong>
                      <small style={{ color: "#93c5fd" }}>{vm.privateIp || vm.ip} · {vm.os || vm.cpu}</small>
                    </div>
                  </div>
                ))}
                {linkedVms.length > 10 && (
                  <div 
                    className="topology-node-card" 
                    style={{ background: "#1e293b", borderColor: "#334155", color: "#94a3b8", display: "flex", alignItems: "center", justifyContent: "center" }}
                  >
                    <span>... 及另外 {linkedVms.length - 10} 台云主机</span>
                  </div>
                )}
              </div>
            </div>

            {/* Flow Connector 3 -> 4 */}
            <div style={{ display: "flex", alignItems: "center", gap: 8, margin: "2px 0 2px 24px", color: "#fb923c", fontSize: 11, fontWeight: 600 }}>
              <span>↓</span>
              <span style={{ background: "rgba(251,146,60,0.12)", border: "1px solid rgba(251,146,60,0.25)", padding: "2px 10px", borderRadius: 12 }}>
                应用运行容器与中间件服务集群 (Web/Java容器/MQ/调度)
              </span>
            </div>

            {/* LEVEL 4: MIDDLEWARE SERVICES LAYER */}
            <div>
              <div className="topology-layer-title" style={{ color: "#fb923c", fontWeight: 700 }}>
                LEVEL 4 · 中间件服务集群与运行时层 (MIDDLEWARE & RUNTIME · 共 {linkedMws.length} 实例)
              </div>
              <div className="topology-nodes-row" style={{ flexWrap: "wrap", gap: 10 }}>
                {linkedMws.length > 0 ? (
                  linkedMws.map(mw => (
                    <div 
                      key={mw.id}
                      className="topology-node-card"
                      style={{ background: "linear-gradient(135deg, #1e293b, #0f172a)", border: "1.5px solid rgba(251, 146, 60, 0.45)", cursor: "pointer" }}
                      onClick={() => setInspectedNode({
                        title: `${mw.mwSoftware || mw.name}`,
                        type: `中间件服务 (${mw.mwType})`,
                        detail: `软件版本: ${mw.version || "-"} · 运行端口: ${mw.port || "-"} · 运行时环境: ${mw.runtime || "-"}`,
                        extra: `部署主机IP: ${mw.privateIp || mw.assetIp} · 节点角色: ${mw.role || "Server"}`
                      })}
                    >
                      <div className="topology-node-icon db" style={{ background: "#ea580c", fontSize: 18 }}>⚙️</div>
                      <div className="topology-node-meta">
                        <strong style={{ color: "#f8fafc" }}>{mw.mwSoftware || mw.name}</strong>
                        <small style={{ color: "#fdba74" }}>端口 {mw.port || "-"} · {mw.runtime || "JDK8"}</small>
                      </div>
                    </div>
                  ))
                ) : (
                  <div 
                    className="topology-node-card"
                    style={{ background: "linear-gradient(135deg, #1e293b, #0f172a)", border: "1.5px solid rgba(251, 146, 60, 0.45)", cursor: "pointer" }}
                    onClick={() => setInspectedNode({
                      title: "标准应用 Web 容器",
                      type: "通用应用服务",
                      detail: "基于 OpenJDK / Tomcat / Nginx 支撑项目前后台业务服务",
                      extra: "支持微服务高可用集群注册中心"
                    })}
                  >
                    <div className="topology-node-icon db" style={{ background: "#ea580c", fontSize: 18 }}>⚙️</div>
                    <div className="topology-node-meta">
                      <strong style={{ color: "#f8fafc" }}>应用服务器与 Web 容器</strong>
                      <small style={{ color: "#fdba74" }}>标准应用运行时 · 端口 80/8080</small>
                    </div>
                  </div>
                )}
              </div>
            </div>

            {/* Flow Connector 4 -> 5 */}
            <div style={{ display: "flex", alignItems: "center", gap: 8, margin: "2px 0 2px 24px", color: "#f59e0b", fontSize: 11, fontWeight: 600 }}>
              <span>↓</span>
              <span style={{ background: "rgba(245,158,11,0.12)", border: "1px solid rgba(245,158,11,0.25)", padding: "2px 10px", borderRadius: 12 }}>
                数据持久化存储与核心数据库集群连接链路
              </span>
            </div>

            {/* LEVEL 5: DATABASE & STORAGE LAYER */}
            <div>
              <div className="topology-layer-title" style={{ color: "#fbbf24", fontWeight: 700 }}>
                LEVEL 5 · 数据持久化与核心数据库层 (DATABASE LAYER · 共 {linkedDbs.length} 实例)
              </div>
              <div className="topology-nodes-row" style={{ flexWrap: "wrap", gap: 10 }}>
                {linkedDbs.length > 0 ? (
                  linkedDbs.map(db => (
                    <div 
                      key={db.id} 
                      className="topology-node-card"
                      style={{ background: "linear-gradient(135deg, #1e293b, #0f172a)", border: "1.5px solid rgba(245, 158, 11, 0.45)", cursor: "pointer" }}
                      onClick={() => setInspectedNode({
                        title: db.name,
                        type: `生产数据库 (${db.dbSoftware || db.type})`,
                        detail: `版本: ${db.version} · 访问主机: ${db.hostIp || db.privateIp}:${db.port} · 部署架构: ${db.deployMode || db.arch || "单机"}`,
                        extra: `实例名/SID: ${db.instanceSid || "-"} · 库名: ${db.dbName || "-"} · 状态: ${db.status}`
                      })}
                    >
                      <div className="topology-node-icon db" style={{ fontSize: 20 }}>🗄️</div>
                      <div className="topology-node-meta">
                        <strong style={{ color: "#f8fafc" }}>{db.name}</strong>
                        <small style={{ color: "#fcd34d" }}>{db.dbSoftware || db.type} · {db.hostIp || db.privateIp}:{db.port}</small>
                      </div>
                    </div>
                  ))
                ) : (
                  <div 
                    className="topology-node-card"
                    style={{ background: "linear-gradient(135deg, #1e293b, #0f172a)", border: "1.5px solid rgba(245, 158, 11, 0.45)", cursor: "pointer" }}
                    onClick={() => setInspectedNode({
                      title: "关系型数据库实例",
                      type: "持久化存储",
                      detail: "业务高可用数据持久化存储",
                      extra: "容灾与主备同步机制"
                    })}
                  >
                    <div className="topology-node-icon db" style={{ fontSize: 20 }}>💾</div>
                    <div className="topology-node-meta">
                      <strong style={{ color: "#f8fafc" }}>核心数据库实例</strong>
                      <small style={{ color: "#fcd34d" }}>高可用数据存储集群</small>
                    </div>
                  </div>
                )}
              </div>
            </div>

            {/* Flow Connector 5 -> 6 */}
            <div style={{ display: "flex", alignItems: "center", gap: 8, margin: "2px 0 2px 24px", color: "#a855f7", fontSize: 11, fontWeight: 600 }}>
              <span>↓</span>
              <span style={{ background: "rgba(168,85,247,0.12)", border: "1px solid rgba(168,85,247,0.25)", padding: "2px 10px", borderRadius: 12 }}>
                数据容灾备份策略与云底座物理算力支撑
              </span>
            </div>

            {/* LEVEL 6: BACKUP & PHYSICAL INFRASTRUCTURE LAYER */}
            <div>
              <div className="topology-layer-title" style={{ color: "#c084fc", fontWeight: 700 }}>
                LEVEL 6 · 数据容灾备份与底座支撑层 (BACKUP & INFRASTRUCTURE LAYER)
              </div>
              <div className="topology-nodes-row" style={{ flexWrap: "wrap", gap: 10 }}>
                {linkedBks.length > 0 ? (
                  linkedBks.map(bk => (
                    <div 
                      key={bk.id}
                      className="topology-node-card"
                      style={{ background: "linear-gradient(135deg, #1e293b, #0f172a)", border: "1.5px solid rgba(16, 185, 129, 0.45)", cursor: "pointer" }}
                      onClick={() => setInspectedNode({
                        title: `备份策略: ${bk.backupType} - ${bk.backupPolicy}`,
                        type: "数据容灾备份",
                        detail: `备份方式: ${bk.backupMethod} · 策略周期: ${bk.backupPolicy} · 数据保留: ${bk.retentionDays} 天`,
                        extra: `存储介质位置: ${bk.storageLocation || "政务专区专用NAS"} · 主机IP: ${bk.privateIp}`
                      })}
                    >
                      <div className="topology-node-icon db" style={{ background: "#059669", fontSize: 18 }}>💾</div>
                      <div className="topology-node-meta">
                        <strong style={{ color: "#f8fafc" }}>{bk.backupType} · {bk.backupPolicy}</strong>
                        <small style={{ color: "#6ee7b7" }}>{bk.backupMethod} · 保留 {bk.retentionDays} 天</small>
                      </div>
                    </div>
                  ))
                ) : null}

                {linkedHosts.length > 0 ? (
                  linkedHosts.map(h => (
                    <div 
                      key={h.id} 
                      className="topology-node-card"
                      style={{ background: "linear-gradient(135deg, #1e293b, #0f172a)", border: "1.5px solid rgba(168, 85, 247, 0.45)", cursor: "pointer" }}
                      onClick={() => setInspectedNode({
                        title: h.hostname,
                        type: "物理服务器 / 计算宿主",
                        detail: `管理IP: ${h.ip} · 带外BMC: ${h.bmcIp} · 硬件: ${h.brand} ${h.model} (${h.cpu})`,
                        extra: `算力规格: ${h.cpu} · 内存: ${h.memory} · 磁盘: ${h.disk}`
                      })}
                    >
                      <div className="topology-node-icon srv" style={{ fontSize: 20, background: "#7c3aed" }}>💻</div>
                      <div className="topology-node-meta">
                        <strong style={{ color: "#f8fafc" }}>{h.hostname}</strong>
                        <small style={{ color: "#d8b4fe" }}>{h.ip} · {h.brand} {h.model}</small>
                      </div>
                    </div>
                  ))
                ) : (
                  <div 
                    className="topology-node-card"
                    style={{ background: "linear-gradient(135deg, #1e293b, #0f172a)", border: "1.5px solid rgba(168, 85, 247, 0.45)", cursor: "pointer" }}
                    onClick={() => setInspectedNode({
                      title: "云服务商算力资源池",
                      type: "虚拟化云底座",
                      detail: "联通云 / 首信云 / 国企云分布式硬件计算节点支撑",
                      extra: "底层多副本分布式存储与高可用硬件节点"
                    })}
                  >
                    <div className="topology-node-icon srv" style={{ fontSize: 20, background: "#7c3aed" }}>🏗️</div>
                    <div className="topology-node-meta">
                      <strong style={{ color: "#f8fafc" }}>云厂商物理基础设施算力池</strong>
                      <small style={{ color: "#d8b4fe" }}>高可用数据中心虚拟化底座</small>
                    </div>
                  </div>
                )}
              </div>
            </div>

            {/* Topology Bottom Status Bar */}
            <div style={{
              marginTop: 14,
              paddingTop: 14,
              borderTop: "1px dashed rgba(148, 163, 184, 0.2)",
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
              flexWrap: "wrap",
              gap: 10,
              fontSize: 12,
              color: "#94a3b8"
            }}>
              <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                <span style={{ color: "#38bdf8" }}>💡 提示:</span>
                <span>相关业务卡片右上角配备 ✏️编辑 与 🗑️删除 快捷操作；点击卡片可查看接口与切换系统</span>
              </div>

              <div style={{ display: "flex", gap: 10, fontSize: 11, flexWrap: "wrap" }}>
                <span style={{ background: "rgba(168,85,247,0.1)", color: "#c084fc", padding: "2px 8px", borderRadius: 4, border: "1px solid rgba(168,85,247,0.2)" }}>
                  相关业务: {relatedSystems.all.length} 个
                </span>
                <span style={{ background: "rgba(56,189,248,0.1)", color: "#38bdf8", padding: "2px 8px", borderRadius: 4, border: "1px solid rgba(56,189,248,0.2)" }}>
                  云主机: {linkedVms.length} 台
                </span>
                <span style={{ background: "rgba(251,146,60,0.1)", color: "#fb923c", padding: "2px 8px", borderRadius: 4, border: "1px solid rgba(251,146,60,0.2)" }}>
                  中间件: {linkedMws.length} 个
                </span>
                <span style={{ background: "rgba(245,158,11,0.1)", color: "#fbbf24", padding: "2px 8px", borderRadius: 4, border: "1px solid rgba(245,158,11,0.2)" }}>
                  数据库: {linkedDbs.length} 库
                </span>
                <span style={{ background: "rgba(16,185,129,0.1)", color: "#34d399", padding: "2px 8px", borderRadius: 4, border: "1px solid rgba(16,185,129,0.2)" }}>
                  备份策略: {linkedBks.length} 项
                </span>
              </div>
            </div>
          </div>
        </div>
      ) : (
        /* ================= 3B. 跨业务系统协同拓扑图 (Cross-Business Ecosystem Matrix) ================= */
        <div style={{
          background: "linear-gradient(180deg, #0b132b 0%, #070c1b 100%)",
          border: "1.5px solid #1e293b",
          borderRadius: 14,
          padding: "24px 26px 32px",
          boxShadow: "0 10px 32px rgba(0, 0, 0, 0.45)",
          color: "#fff",
          position: "relative"
        }}>
          <header style={{ marginBottom: 20, borderBottom: "1px solid rgba(148, 163, 184, 0.15)", paddingBottom: 14, display: "flex", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: 10 }}>
            <div>
              <h4 style={{ margin: 0, fontSize: 17, color: "#38bdf8", fontWeight: 700, display: "flex", alignItems: "center", gap: 8 }}>
                <span>🌐</span>
                <span>跨业务各系统互联互通拓扑大网 (CROSS-SYSTEM COLLABORATION MATRIX)</span>
              </h4>
              <p style={{ margin: "4px 0 0", fontSize: 12, color: "#94a3b8" }}>
                以【{selectedBiz.name}】为核心观察点，清晰呈现业务各系统之间的接口调用协议、数据流转方向与协同链路
              </p>
            </div>

            <div style={{ display: "flex", gap: 8 }}>
              <button
                onClick={() => handleOpenCreateModal()}
                style={{
                  background: "#16a34a",
                  color: "#fff",
                  border: 0,
                  fontSize: 12,
                  fontWeight: 600,
                  padding: "5px 12px",
                  borderRadius: 6,
                  cursor: "pointer"
                }}
              >
                ➕ 录入关联业务
              </button>
              <button
                onClick={() => setViewMode("layered")}
                style={{
                  background: "rgba(56,189,248,0.15)",
                  border: "1px solid rgba(56,189,248,0.3)",
                  color: "#38bdf8",
                  fontSize: 12,
                  padding: "5px 12px",
                  borderRadius: 6,
                  cursor: "pointer",
                  fontWeight: 600
                }}
              >
                返回基础设施纵深拓扑 ➔
              </button>
            </div>
          </header>

          {/* Central Hub Graph */}
          <div style={{ display: "flex", flexDirection: "column", gap: 24 }}>
            {/* Center: Active System Mega Node */}
            <div style={{ display: "flex", justifyContent: "center" }}>
              <div style={{
                background: "linear-gradient(135deg, #1e3a8a 0%, #0f172a 100%)",
                border: "2.5px solid #38bdf8",
                borderRadius: 16,
                padding: "18px 28px",
                textAlign: "center",
                maxWidth: 480,
                width: "100%",
                boxShadow: "0 0 30px rgba(56, 189, 248, 0.35)",
                position: "relative"
              }}>
                <div style={{ fontSize: 28, marginBottom: 4 }}>🏛</div>
                <h3 style={{ margin: 0, fontSize: 18, color: "#f8fafc", fontWeight: 700 }}>
                  {selectedBiz.name}
                </h3>
                <span style={{ fontSize: 11, color: "#93c5fd", display: "inline-block", marginTop: 4 }}>
                  核心业务处理中枢 · 归属: {selectedBiz.department}
                </span>

                <div style={{ marginTop: 12, display: "flex", justifyContent: "center", gap: 12, fontSize: 11, color: "#cbd5e1" }}>
                  <span style={{ background: "rgba(255,255,255,0.1)", padding: "2px 8px", borderRadius: 4 }}>
                    上游输入: {relatedSystems.upstream.length} 个
                  </span>
                  <span style={{ background: "rgba(255,255,255,0.1)", padding: "2px 8px", borderRadius: 4 }}>
                    横向协同: {relatedSystems.peer.length} 个
                  </span>
                  <span style={{ background: "rgba(255,255,255,0.1)", padding: "2px 8px", borderRadius: 4 }}>
                    下游监管: {relatedSystems.downstream.length} 个
                  </span>
                </div>
              </div>
            </div>

            {/* Radial Relationship Cards Grid */}
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(320px, 1fr))", gap: 16 }}>
              {relatedSystems.all.map((rel) => (
                <div
                  key={rel.id}
                  style={{
                    background: "rgba(15, 23, 42, 0.8)",
                    border: `1.5px solid ${rel.relationType === "upstream" ? "#0284c7" : rel.relationType === "peer" ? "#7c3aed" : "#059669"}`,
                    borderRadius: 10,
                    padding: "14px 16px",
                    display: "flex",
                    flexDirection: "column",
                    justifyContent: "space-between",
                    gap: 10,
                    transition: "transform 0.15s ease, box-shadow 0.15s ease",
                    cursor: "pointer"
                  }}
                  onMouseEnter={e => e.currentTarget.style.transform = "translateY(-2px)"}
                  onMouseLeave={e => e.currentTarget.style.transform = "translateY(0)"}
                  onClick={() => setInspectedNode({
                    title: rel.targetSystem,
                    type: `相关业务 · ${rel.relationTypeLabel}`,
                    detail: rel.description,
                    extra: `交互协议: ${rel.interfaceProtocol} · 频次: ${rel.frequency} · 数据项: ${rel.dataEntities.join("、")}`,
                    relationData: rel,
                    targetSystemName: rel.targetSystem
                  })}
                >
                  <div>
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 8, marginBottom: 6 }}>
                      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                        <span style={{
                          fontSize: 16,
                          width: 28,
                          height: 28,
                          display: "flex",
                          alignItems: "center",
                          justifyContent: "center",
                          borderRadius: 6,
                          background: rel.relationType === "upstream" ? "#0284c7" : rel.relationType === "peer" ? "#7c3aed" : "#059669"
                        }}>
                          {rel.relationType === "upstream" ? "📥" : rel.relationType === "peer" ? "🔄" : "📤"}
                        </span>
                        <div>
                          <strong style={{ fontSize: 14, color: "#f8fafc" }}>{rel.targetSystem}</strong>
                          <span style={{ fontSize: 11, color: "#94a3b8", display: "block" }}>{rel.relationTypeLabel}</span>
                        </div>
                      </div>

                      <div style={{ display: "flex", gap: 4 }} onClick={e => e.stopPropagation()}>
                        <button
                          onClick={() => handleOpenEditModal(rel)}
                          style={{ background: "rgba(255,255,255,0.1)", border: "1px solid rgba(255,255,255,0.2)", color: "#fff", padding: "2px 6px", borderRadius: 4, fontSize: 11, cursor: "pointer" }}
                          title="修改此业务关联"
                        >
                          ✏️ 修改
                        </button>
                        <button
                          onClick={() => setDeletingRelation(rel)}
                          style={{ background: "rgba(239,68,68,0.2)", border: "1px solid rgba(239,68,68,0.3)", color: "#fca5a5", padding: "2px 6px", borderRadius: 4, fontSize: 11, cursor: "pointer" }}
                          title="删除此业务关联"
                        >
                          🗑️
                        </button>
                      </div>
                    </div>

                    <p style={{ margin: "6px 0", fontSize: 12, color: "#cbd5e1", lineHeight: 1.5 }}>
                      {rel.description}
                    </p>

                    <div style={{ background: "rgba(0,0,0,0.3)", padding: "6px 10px", borderRadius: 6, fontSize: 11, color: "#94a3b8" }}>
                      <span style={{ color: "#38bdf8" }}>🔌 接口协议:</span> {rel.interfaceProtocol}
                    </div>

                    {rel.dataEntities && rel.dataEntities.length > 0 && (
                      <div style={{ marginTop: 8, display: "flex", gap: 4, flexWrap: "wrap" }}>
                        {rel.dataEntities.map((ent, eIdx) => (
                          <span key={eIdx} style={{ fontSize: 10, background: "rgba(255,255,255,0.06)", color: "#e2e8f0", padding: "1px 6px", borderRadius: 3 }}>
                            🏷️ {ent}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>

                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", borderTop: "1px dashed rgba(255,255,255,0.1)", paddingTop: 8, marginTop: 4 }}>
                    <span style={{ fontSize: 11, color: "#22c55e", display: "flex", alignItems: "center", gap: 4 }}>
                      ● 链路运行中 ({rel.frequency})
                    </span>

                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleSwitchToSystem(rel.targetSystem);
                      }}
                      style={{
                        background: "#2563eb",
                        border: 0,
                        color: "#fff",
                        fontSize: 11,
                        fontWeight: 600,
                        padding: "4px 10px",
                        borderRadius: 4,
                        cursor: "pointer"
                      }}
                    >
                      ⚡ 查看该系统拓扑 ➔
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* ================= 4. RELATION FORM MODAL (录入 / 修改) ================= */}
      {showFormModal && (
        <div style={{
          position: "fixed",
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          background: "rgba(0,0,0,0.7)",
          display: "flex",
          justifyContent: "center",
          alignItems: "center",
          zIndex: 2000
        }}>
          <div style={{
            background: "#ffffff",
            borderRadius: 10,
            padding: 24,
            maxWidth: 580,
            width: "92%",
            boxShadow: "0 20px 40px rgba(0,0,0,0.3)",
            color: "#0f172a",
            maxHeight: "90vh",
            overflowY: "auto"
          }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16, borderBottom: "1px solid #e2e8f0", paddingBottom: 10 }}>
              <h3 style={{ margin: 0, fontSize: 16, color: "#0f172a", display: "flex", alignItems: "center", gap: 6 }}>
                <span>{formMode === "create" ? "➕ 录入相关业务系统关联" : "✏️ 修改相关业务系统关联"}</span>
              </h3>
              <button
                onClick={() => setShowFormModal(false)}
                style={{ background: "transparent", border: 0, fontSize: 20, cursor: "pointer", color: "#64748b" }}
              >
                ×
              </button>
            </div>

            <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
              {/* 1. 源业务系统 与 目标业务系统 */}
              <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
                <div>
                  <label style={{ fontSize: 12, fontWeight: 700, color: "#334155", display: "block", marginBottom: 4 }}>
                    本端业务系统 *
                  </label>
                  <select
                    value={formSource}
                    onChange={e => setFormSource(e.target.value)}
                    style={{ width: "100%", padding: "7px 10px", borderRadius: 6, border: "1px solid #cbd5e1", fontSize: 13 }}
                  >
                    {safeBusinesses.map(b => (
                      <option key={b.id} value={b.name}>{b.name}</option>
                    ))}
                  </select>
                </div>

                <div>
                  <label style={{ fontSize: 12, fontWeight: 700, color: "#334155", display: "block", marginBottom: 4 }}>
                    关联的目标业务系统 *
                  </label>
                  <input
                    list="available-targets"
                    placeholder="选择或输入关联的目标系统"
                    value={formTarget}
                    onChange={e => setFormTarget(e.target.value)}
                    style={{ width: "100%", padding: "7px 10px", borderRadius: 6, border: "1px solid #cbd5e1", fontSize: 13 }}
                  />
                  <datalist id="available-targets">
                    {EXCEL_PROJECTS.map(p => (
                      <option key={p} value={p} />
                    ))}
                  </datalist>
                </div>
              </div>

              {/* 2. 关系类型与角色标签 */}
              <div style={{ display: "grid", gridTemplateColumns: "1.2fr 1fr", gap: 12 }}>
                <div>
                  <label style={{ fontSize: 12, fontWeight: 700, color: "#334155", display: "block", marginBottom: 4 }}>
                    协同关系类型 *
                  </label>
                  <select
                    value={formType}
                    onChange={e => {
                      const t = e.target.value as any;
                      setFormType(t);
                      if (t === "upstream") {
                        setFormDirection("incoming");
                        if (!formLabel || formLabel === "协同联动业务" || formLabel === "下游风控监管中台") setFormLabel("上游网上申报入口");
                      } else if (t === "downstream" || t === "shared_hub") {
                        setFormDirection("outgoing");
                        if (!formLabel || formLabel === "协同联动业务" || formLabel === "上游网上申报入口") setFormLabel("下游风控监管中台");
                      } else {
                        setFormDirection("bidirectional");
                        if (!formLabel || formLabel === "上游网上申报入口" || formLabel === "下游风控监管中台") setFormLabel("平行协同联动业务");
                      }
                    }}
                    style={{ width: "100%", padding: "7px 10px", borderRadius: 6, border: "1px solid #cbd5e1", fontSize: 13 }}
                  >
                    <option value="upstream">📥 上游业务入口 (数据推入本系统)</option>
                    <option value="peer">🔄 平行协同联动 (双向业务流转)</option>
                    <option value="downstream">📤 下游监管处理 (本系统输出数据)</option>
                    <option value="shared_hub">🏛 数据要素中台 / 大数据湖汇聚</option>
                  </select>
                </div>

                <div>
                  <label style={{ fontSize: 12, fontWeight: 700, color: "#334155", display: "block", marginBottom: 4 }}>
                    关系角色标签
                  </label>
                  <input
                    placeholder="如: 网上申报入口 / 协同庭审"
                    value={formLabel}
                    onChange={e => setFormLabel(e.target.value)}
                    style={{ width: "100%", padding: "7px 10px", borderRadius: 6, border: "1px solid #cbd5e1", fontSize: 13 }}
                  />
                </div>
              </div>

              {/* 快捷标签推荐 */}
              <div style={{ display: "flex", gap: 6, flexWrap: "wrap", alignItems: "center" }}>
                <span style={{ fontSize: 11, color: "#64748b" }}>推荐标签:</span>
                {["网上立案入口", "在线云庭审", "案前调解诉调对接", "风控监管中台", "大数据湖归集", "征缴专网对账", "资金审批监管", "商城集采联动"].map(tag => (
                  <button
                    key={tag}
                    type="button"
                    onClick={() => setFormLabel(tag)}
                    style={{ border: "1px solid #cbd5e1", background: "#f8fafc", fontSize: 11, padding: "2px 6px", borderRadius: 4, cursor: "pointer" }}
                  >
                    {tag}
                  </button>
                ))}
              </div>

              {/* 3. 数据流向与调用频次 */}
              <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
                <div>
                  <label style={{ fontSize: 12, fontWeight: 700, color: "#334155", display: "block", marginBottom: 4 }}>
                    数据/业务交互流向
                  </label>
                  <select
                    value={formDirection}
                    onChange={e => setFormDirection(e.target.value as any)}
                    style={{ width: "100%", padding: "7px 10px", borderRadius: 6, border: "1px solid #cbd5e1", fontSize: 13 }}
                  >
                    <option value="incoming">📥 入向流动 (目标系统 ➔ 本端系统)</option>
                    <option value="bidirectional">🔄 双向互通 (本端系统 ⇄ 目标系统)</option>
                    <option value="outgoing">📤 出向流动 (本端系统 ➔ 目标系统)</option>
                  </select>
                </div>

                <div>
                  <label style={{ fontSize: 12, fontWeight: 700, color: "#334155", display: "block", marginBottom: 4 }}>
                    数据交互频次
                  </label>
                  <select
                    value={formFrequency}
                    onChange={e => setFormFrequency(e.target.value as any)}
                    style={{ width: "100%", padding: "7px 10px", borderRadius: 6, border: "1px solid #cbd5e1", fontSize: 13 }}
                  >
                    <option value="实时同步">⚡ 实时同步 (毫秒/秒级)</option>
                    <option value="准实时流">🌊 准实时流处理 (Kafka/MQ)</option>
                    <option value="每日批处理">🌙 每日定时批处理 (ETL/深夜)</option>
                    <option value="按需调用">🎯 按需触发调用 (主动请求)</option>
                  </select>
                </div>
              </div>

              {/* 4. 交互接口方式与通讯协议 */}
              <div>
                <label style={{ fontSize: 12, fontWeight: 700, color: "#334155", display: "block", marginBottom: 4 }}>
                  交互接口方式与通讯协议
                </label>
                <input
                  placeholder="如: RESTful API (HTTPS/JSON 国密SM4) / WebSocket 实时流 / Kafka"
                  value={formProtocol}
                  onChange={e => setFormProtocol(e.target.value)}
                  style={{ width: "100%", padding: "7px 10px", borderRadius: 6, border: "1px solid #cbd5e1", fontSize: 13 }}
                />
                <div style={{ display: "flex", gap: 6, flexWrap: "wrap", marginTop: 4 }}>
                  {[
                    "RESTful API (HTTPS/JSON 国密SM4)",
                    "WebSocket 实时流 + WebRTC 音视频",
                    "Kafka 分布式消息队列",
                    "ETL 批量抽取流水线 / 共享库",
                    "政务专线 SFTP / XML 报文",
                    "数据库链路 (DB Link / CDC)"
                  ].map(proto => (
                    <button
                      key={proto}
                      type="button"
                      onClick={() => setFormProtocol(proto)}
                      style={{ border: "1px solid #cbd5e1", background: "#f8fafc", fontSize: 10, padding: "2px 5px", borderRadius: 3, cursor: "pointer" }}
                    >
                      {proto.split(" ")[0]}
                    </button>
                  ))}
                </div>
              </div>

              {/* 5. 核心交换数据项 */}
              <div>
                <label style={{ fontSize: 12, fontWeight: 700, color: "#334155", display: "block", marginBottom: 4 }}>
                  核心交换数据实体项 (逗号或换行分隔)
                </label>
                <input
                  placeholder="如: 网上立案申请单, 当事人诉辩材料, 电子送达回证, 庭审语音笔录"
                  value={formDataEntities}
                  onChange={e => setFormDataEntities(e.target.value)}
                  style={{ width: "100%", padding: "7px 10px", borderRadius: 6, border: "1px solid #cbd5e1", fontSize: 13 }}
                />
              </div>

              {/* 6. 业务协同场景与交互链路说明 */}
              <div>
                <label style={{ fontSize: 12, fontWeight: 700, color: "#334155", display: "block", marginBottom: 4 }}>
                  业务协同场景与交互链路说明
                </label>
                <textarea
                  placeholder="详细描述两系统之间在什么业务环节发起交互，数据如何流转、审核或结案流向..."
                  value={formDesc}
                  onChange={e => setFormDesc(e.target.value)}
                  rows={3}
                  style={{ width: "100%", padding: "7px 10px", borderRadius: 6, border: "1px solid #cbd5e1", fontSize: 13, resize: "vertical" }}
                />
              </div>
            </div>

            <div style={{ display: "flex", justifyContent: "flex-end", gap: 10, marginTop: 20, borderTop: "1px solid #e2e8f0", paddingTop: 14 }}>
              <button
                className="btn-secondary"
                onClick={() => setShowFormModal(false)}
                style={{ padding: "6px 14px" }}
              >
                取消
              </button>
              <button
                onClick={handleSaveRelation}
                style={{
                  background: "#2563eb",
                  color: "#fff",
                  border: 0,
                  borderRadius: 6,
                  padding: "6px 20px",
                  fontSize: 13,
                  fontWeight: 600,
                  cursor: "pointer"
                }}
              >
                {formMode === "create" ? "立即录入" : "保存修改"}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ================= 5. DELETE CONFIRMATION MODAL (删除确认) ================= */}
      {deletingRelation && (
        <div style={{
          position: "fixed",
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          background: "rgba(0,0,0,0.65)",
          display: "flex",
          justifyContent: "center",
          alignItems: "center",
          zIndex: 2200
        }}>
          <div style={{
            background: "#ffffff",
            borderRadius: 10,
            padding: 24,
            maxWidth: 420,
            width: "90%",
            boxShadow: "0 20px 40px rgba(0,0,0,0.4)",
            color: "#0f172a"
          }}>
            <div style={{ fontSize: 32, marginBottom: 8, textAlign: "center" }}>⚠️</div>
            <h3 style={{ margin: "0 0 10px", fontSize: 16, textAlign: "center", color: "#dc2626" }}>
              确认移除此业务系统关联关系？
            </h3>
            <p style={{ margin: "0 0 16px", fontSize: 13, color: "#475569", lineHeight: 1.6, textAlign: "center" }}>
              即将移除【<strong>{deletingRelation.sourceSystem}</strong>】与【<strong>{deletingRelation.targetSystem}</strong>】之间的【{deletingRelation.relationTypeLabel}】协同链路。
              <br />
              <small style={{ color: "#94a3b8" }}>移除后拓扑图中将不再显示此条关联链路。</small>
            </p>

            <div style={{ display: "flex", justifyContent: "center", gap: 12 }}>
              <button
                className="btn-secondary"
                onClick={() => setDeletingRelation(null)}
                style={{ padding: "6px 16px" }}
              >
                取消
              </button>
              <button
                onClick={handleConfirmDelete}
                style={{
                  background: "#dc2626",
                  color: "#fff",
                  border: 0,
                  borderRadius: 6,
                  padding: "6px 18px",
                  fontSize: 13,
                  fontWeight: 600,
                  cursor: "pointer"
                }}
              >
                确认删除
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ================= 6. MANAGE RELATIONS MODAL (全部关联清单管理) ================= */}
      {showManageModal && (
        <div style={{
          position: "fixed",
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          background: "rgba(0,0,0,0.75)",
          display: "flex",
          justifyContent: "center",
          alignItems: "center",
          zIndex: 1500
        }}>
          <div style={{
            background: "#ffffff",
            borderRadius: 12,
            padding: 24,
            maxWidth: 1050,
            width: "95%",
            maxHeight: "90vh",
            display: "flex",
            flexDirection: "column",
            boxShadow: "0 25px 50px rgba(0,0,0,0.4)",
            color: "#0f172a"
          }}>
            {/* Header */}
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 14, borderBottom: "1px solid #e2e8f0", paddingBottom: 10 }}>
              <div>
                <h3 style={{ margin: 0, fontSize: 17, color: "#0f172a", display: "flex", alignItems: "center", gap: 8 }}>
                  <span>⚙️</span>
                  <span>全系统业务系统关联拓扑清单管理 (CRUD 维护中心)</span>
                </h3>
                <span style={{ fontSize: 12, color: "#64748b" }}>
                  共维护 {relations.length} 条业务协同与接口调用关系 · 支持新增、修改、删除、还原及 JSON 导出
                </span>
              </div>
              <button
                onClick={() => setShowManageModal(false)}
                style={{ background: "transparent", border: 0, fontSize: 24, cursor: "pointer", color: "#64748b" }}
              >
                ×
              </button>
            </div>

            {/* Filter and Top Action Bar */}
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12, flexWrap: "wrap", gap: 10 }}>
              <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
                <input
                  placeholder="搜索系统名称 / 协议 / 标签..."
                  value={manageKeyword}
                  onChange={e => setManageKeyword(e.target.value)}
                  style={{ width: 220, fontSize: 12, padding: "6px 10px", borderRadius: 6, border: "1px solid #cbd5e1" }}
                />

                <select
                  value={manageTypeFilter}
                  onChange={e => setManageTypeFilter(e.target.value)}
                  style={{ fontSize: 12, padding: "6px 10px", borderRadius: 6, border: "1px solid #cbd5e1" }}
                >
                  <option value="all">全类别关系</option>
                  <option value="upstream">上游输入</option>
                  <option value="peer">平行协同</option>
                  <option value="downstream">下游监管</option>
                  <option value="shared_hub">数据中台</option>
                </select>
              </div>

              <div style={{ display: "flex", gap: 8 }}>
                <button
                  onClick={() => {
                    handleOpenCreateModal();
                  }}
                  style={{
                    background: "#16a34a",
                    color: "#fff",
                    border: 0,
                    fontSize: 12,
                    fontWeight: 600,
                    padding: "6px 12px",
                    borderRadius: 6,
                    cursor: "pointer"
                  }}
                >
                  ➕ 新增关联
                </button>
                <button
                  onClick={handleExportRelationsJson}
                  style={{
                    background: "#f1f5f9",
                    color: "#2563eb",
                    border: "1px solid #cbd5e1",
                    fontSize: 12,
                    fontWeight: 600,
                    padding: "6px 12px",
                    borderRadius: 6,
                    cursor: "pointer"
                  }}
                >
                  📋 导出 JSON
                </button>
                <button
                  onClick={handleResetDefaultRelations}
                  style={{
                    background: "#fff",
                    color: "#dc2626",
                    border: "1px solid #fca5a5",
                    fontSize: 12,
                    padding: "6px 12px",
                    borderRadius: 6,
                    cursor: "pointer"
                  }}
                >
                  🔄 恢复默认预置
                </button>
              </div>
            </div>

            {/* Relations Table View */}
            <div style={{ flex: 1, overflowY: "auto", border: "1px solid #e2e8f0", borderRadius: 8 }}>
              <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
                <thead>
                  <tr style={{ background: "#f8fafc", borderBottom: "1.5px solid #cbd5e1", textAlign: "left" }}>
                    <th style={{ padding: "8px 10px", color: "#475569" }}>源业务系统</th>
                    <th style={{ padding: "8px 6px", textAlign: "center", color: "#475569" }}>流向</th>
                    <th style={{ padding: "8px 10px", color: "#475569" }}>目标关联系统</th>
                    <th style={{ padding: "8px 10px", color: "#475569" }}>关系定位</th>
                    <th style={{ padding: "8px 10px", color: "#475569" }}>接口协议与方式</th>
                    <th style={{ padding: "8px 10px", color: "#475569" }}>频次</th>
                    <th style={{ padding: "8px 10px", textAlign: "center", color: "#475569" }}>操作</th>
                  </tr>
                </thead>
                <tbody>
                  {relations
                    .filter(r => {
                      if (manageTypeFilter !== "all" && r.relationType !== manageTypeFilter) return false;
                      if (manageKeyword.trim()) {
                        const kw = manageKeyword.toLowerCase();
                        return (r.sourceSystem || "").toLowerCase().includes(kw) ||
                               (r.targetSystem || "").toLowerCase().includes(kw) ||
                               (r.relationTypeLabel || "").toLowerCase().includes(kw) ||
                               (r.interfaceProtocol || "").toLowerCase().includes(kw);
                      }
                      return true;
                    })
                    .map((rel, idx) => (
                      <tr 
                        key={rel.id || idx}
                        style={{
                          borderBottom: "1px solid #f1f5f9",
                          background: idx % 2 === 0 ? "#ffffff" : "#fbfcfe"
                        }}
                      >
                        <td style={{ padding: "8px 10px", fontWeight: 600, color: "#1e3a8a" }}>
                          {rel.sourceSystem}
                        </td>
                        <td style={{ padding: "8px 6px", textAlign: "center", fontSize: 13 }}>
                          {rel.direction === "incoming" ? "➔" : rel.direction === "outgoing" ? "➔" : "⇄"}
                        </td>
                        <td style={{ padding: "8px 10px", fontWeight: 600, color: "#0f172a" }}>
                          {rel.targetSystem}
                        </td>
                        <td style={{ padding: "8px 10px" }}>
                          <span style={{
                            padding: "2px 6px",
                            borderRadius: 4,
                            fontSize: 11,
                            background: rel.relationType === "upstream" ? "#e0f2fe" : rel.relationType === "peer" ? "#f3e8ff" : "#ecfdf5",
                            color: rel.relationType === "upstream" ? "#0369a1" : rel.relationType === "peer" ? "#7e22ce" : "#047857"
                          }}>
                            {rel.relationTypeLabel}
                          </span>
                        </td>
                        <td style={{ padding: "8px 10px", color: "#64748b", maxWidth: 220, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={rel.interfaceProtocol}>
                          {rel.interfaceProtocol}
                        </td>
                        <td style={{ padding: "8px 10px", color: "#475569" }}>
                          {rel.frequency}
                        </td>
                        <td style={{ padding: "8px 10px", textAlign: "center" }}>
                          <div style={{ display: "flex", gap: 6, justifyContent: "center" }}>
                            <button
                              onClick={() => {
                                handleSwitchToSystem(rel.sourceSystem);
                                setShowManageModal(false);
                              }}
                              style={{ background: "#f0f9ff", border: "1px solid #bae6fd", color: "#0284c7", padding: "2px 6px", borderRadius: 4, fontSize: 11, cursor: "pointer" }}
                              title="在拓扑图中定位此系统"
                            >
                              👁️ 查看
                            </button>
                            <button
                              onClick={() => handleOpenEditModal(rel)}
                              style={{ background: "#f8fafc", border: "1px solid #cbd5e1", color: "#334155", padding: "2px 6px", borderRadius: 4, fontSize: 11, cursor: "pointer" }}
                              title="编辑此业务关联"
                            >
                              ✏️ 编辑
                            </button>
                            <button
                              onClick={() => setDeletingRelation(rel)}
                              style={{ background: "#fef2f2", border: "1px solid #fecaca", color: "#dc2626", padding: "2px 6px", borderRadius: 4, fontSize: 11, cursor: "pointer" }}
                              title="删除此业务关联"
                            >
                              🗑️
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                </tbody>
              </table>
            </div>

            <div style={{ display: "flex", justifyContent: "flex-end", marginTop: 14 }}>
              <button
                className="btn-secondary"
                onClick={() => setShowManageModal(false)}
                style={{ padding: "6px 18px" }}
              >
                关闭
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ================= 7. NODE INSPECTOR DRAWER ================= */}
      {inspectedNode && (
        <div style={{
          position: "fixed",
          right: 24,
          bottom: 24,
          width: 410,
          background: "#0f172a",
          border: "1.5px solid #38bdf8",
          borderRadius: 8,
          padding: 16,
          color: "#fff",
          boxShadow: "0 12px 30px rgba(0,0,0,0.6)",
          zIndex: 100
        }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 8 }}>
            <div>
              <span style={{ fontSize: 10, background: "#2563eb", padding: "2px 6px", borderRadius: 3, color: "#fff", fontWeight: 600 }}>
                {inspectedNode.type}
              </span>
              <h4 style={{ margin: "4px 0 0", fontSize: 14, color: "#38bdf8" }}>{inspectedNode.title}</h4>
            </div>
            <button 
              style={{ background: "transparent", border: 0, color: "#94a3b8", cursor: "pointer", fontSize: 18, lineHeight: 1 }}
              onClick={() => setInspectedNode(null)}
            >
              ×
            </button>
          </div>
          <p style={{ margin: "8px 0", fontSize: 12, color: "#cbd5e1", lineHeight: 1.5 }}>{inspectedNode.detail}</p>
          {inspectedNode.extra && (
            <div style={{ background: "#1e293b", padding: "6px 10px", borderRadius: 4, fontSize: 11, color: "#94a3b8", marginTop: 8 }}>
              {inspectedNode.extra}
            </div>
          )}

          {/* 若为关联系统节点，提供一键切换、修改与删除按钮 */}
          {inspectedNode.targetSystemName && (
            <div style={{ marginTop: 14, display: "flex", justifyContent: "space-between", alignItems: "center", borderTop: "1px dashed rgba(255,255,255,0.15)", paddingTop: 10 }}>
              <div style={{ display: "flex", gap: 6 }}>
                {inspectedNode.relationData && (
                  <>
                    <button
                      onClick={() => handleOpenEditModal(inspectedNode.relationData!)}
                      style={{
                        background: "#334155",
                        border: "1px solid #475569",
                        color: "#f8fafc",
                        padding: "4px 8px",
                        borderRadius: 4,
                        fontSize: 11,
                        cursor: "pointer"
                      }}
                      title="修改此业务关联配置"
                    >
                      ✏️ 修改配置
                    </button>
                    <button
                      onClick={() => setDeletingRelation(inspectedNode.relationData!)}
                      style={{
                        background: "rgba(239, 68, 68, 0.2)",
                        border: "1px solid rgba(239, 68, 68, 0.4)",
                        color: "#fca5a5",
                        padding: "4px 8px",
                        borderRadius: 4,
                        fontSize: 11,
                        cursor: "pointer"
                      }}
                      title="移除此业务关联"
                    >
                      🗑️ 移除
                    </button>
                  </>
                )}
              </div>

              <button
                onClick={() => handleSwitchToSystem(inspectedNode.targetSystemName!)}
                style={{
                  background: "#0284c7",
                  border: 0,
                  color: "#fff",
                  padding: "5px 12px",
                  borderRadius: 4,
                  fontSize: 12,
                  fontWeight: 600,
                  cursor: "pointer"
                }}
              >
                ⚡ 切换至【{inspectedNode.targetSystemName}】拓扑
              </button>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
