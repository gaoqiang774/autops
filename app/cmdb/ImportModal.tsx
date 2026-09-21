"use client";
import React, { useState, useRef, useMemo, useEffect } from "react";
import { VmHost, DatabaseAsset, MiddlewareAsset, BackupAsset, OpsAsset } from "../cmdbData";
import {
  parseExcelWorkbook,
  ParsedWorkbookResult,
  getAssetKey,
  getDatabaseKey,
  getMiddlewareKey,
  getBackupKey,
  getOpsKey,
  diffAssets,
  diffDatabases,
  diffMiddlewares,
  diffBackups,
  diffOps,
  AssetDiffResult
} from "./excelExport";

export type ImportStrategy = "upsert" | "skip" | "replace";
export type DimensionType = "hardware" | "database" | "middleware" | "backup" | "ops";

export interface ItemAnalysis<T> {
  index: number;
  item: T;
  key: string;
  status: "pure_new" | "db_update" | "db_identical" | "file_duplicate";
  isDuplicate: boolean;
  duplicateReason?: string;
  existing?: T;
  diffResult?: AssetDiffResult;
}

interface ImportModalProps {
  targetProjectName?: string | null;
  activeDimension?: DimensionType;
  existingHardware?: (VmHost | any)[];
  existingDatabases?: DatabaseAsset[];
  existingMiddlewares?: MiddlewareAsset[];
  existingBackups?: BackupAsset[];
  existingOpsRecords?: OpsAsset[];
  existingAssets?: (VmHost | any)[]; // legacy compatibility
  onClose: () => void;
  onConfirmImport?: (
    importedAssets: (VmHost & { isImported?: boolean })[],
    strategy: ImportStrategy
  ) => void;
  onConfirmImportMulti?: (
    result: {
      hardware: (VmHost & { isImported?: boolean })[];
      databases: (DatabaseAsset & { isImported?: boolean })[];
      middlewares: (MiddlewareAsset & { isImported?: boolean })[];
      backups: (BackupAsset & { isImported?: boolean })[];
      opsRecords: (OpsAsset & { isImported?: boolean })[];
    },
    strategy: ImportStrategy
  ) => void;
}

export default function ImportModal({
  targetProjectName,
  activeDimension = "hardware",
  existingHardware = [],
  existingDatabases = [],
  existingMiddlewares = [],
  existingBackups = [],
  existingOpsRecords = [],
  existingAssets = [],
  onClose,
  onConfirmImport,
  onConfirmImportMulti
}: ImportModalProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [dragActive, setDragActive] = useState(false);
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState("");
  const [fileName, setFileName] = useState("");

  // Workbook result
  const [workbookResult, setWorkbookResult] = useState<ParsedWorkbookResult | null>(null);

  // Active viewing tab inside the modal
  const [currentTab, setCurrentTab] = useState<DimensionType>(activeDimension);

  // Strategy: default to upsert
  const [strategy, setStrategy] = useState<ImportStrategy>("upsert");

  // Filter tab for preview: all / duplicate / pure_new / db_update / selected
  const [previewFilter, setPreviewFilter] = useState<"all" | "duplicate" | "pure_new" | "db_update" | "selected">("all");

  // Search keyword inside modal
  const [searchQuery, setSearchQuery] = useState("");

  // Selected item indexes per dimension
  const [selectedHw, setSelectedHw] = useState<Set<number>>(new Set());
  const [selectedDb, setSelectedDb] = useState<Set<number>>(new Set());
  const [selectedMw, setSelectedMw] = useState<Set<number>>(new Set());
  const [selectedBk, setSelectedBk] = useState<Set<number>>(new Set());
  const [selectedOps, setSelectedOps] = useState<Set<number>>(new Set());

  // Merge legacy existingAssets with existingHardware
  const hwPool = existingHardware.length > 0 ? existingHardware : existingAssets;

  // Build existing maps
  const existingHwMap = useMemo(() => {
    const map = new Map<string, VmHost>();
    for (const item of hwPool) {
      const k = getAssetKey(item);
      if (!map.has(k)) map.set(k, item);
    }
    return map;
  }, [hwPool]);

  const existingDbMap = useMemo(() => {
    const map = new Map<string, DatabaseAsset>();
    for (const item of existingDatabases) {
      const k = getDatabaseKey(item);
      if (!map.has(k)) map.set(k, item);
    }
    return map;
  }, [existingDatabases]);

  const existingMwMap = useMemo(() => {
    const map = new Map<string, MiddlewareAsset>();
    for (const item of existingMiddlewares) {
      const k = getMiddlewareKey(item);
      if (!map.has(k)) map.set(k, item);
    }
    return map;
  }, [existingMiddlewares]);

  const existingBkMap = useMemo(() => {
    const map = new Map<string, BackupAsset>();
    for (const item of existingBackups) {
      const k = getBackupKey(item);
      if (!map.has(k)) map.set(k, item);
    }
    return map;
  }, [existingBackups]);

  const existingOpsMap = useMemo(() => {
    const map = new Map<string, OpsAsset>();
    for (const item of existingOpsRecords) {
      const k = getOpsKey(item);
      if (!map.has(k)) map.set(k, item);
    }
    return map;
  }, [existingOpsRecords]);

  // Generic item analyzer
  function analyzeItems<T>(
    items: T[],
    getKey: (it: T) => string,
    existingMap: Map<string, T>,
    diffFn: (existing: Partial<T>, incoming: Partial<T>) => AssetDiffResult,
    getDisplayName: (it: T) => string
  ): ItemAnalysis<T>[] {
    if (!items || items.length === 0) return [];
    const countMap = new Map<string, number>();
    const firstIdxMap = new Map<string, number>();

    items.forEach((it, idx) => {
      const k = getKey(it);
      countMap.set(k, (countMap.get(k) || 0) + 1);
      if (!firstIdxMap.has(k)) firstIdxMap.set(k, idx);
    });

    return items.map((it, idx) => {
      const key = getKey(it);
      const isFileDup = (countMap.get(key) || 0) > 1;
      const isFollower = isFileDup && firstIdxMap.get(key) !== idx;
      const existing = existingMap.get(key);

      if (isFollower) {
        return {
          index: idx,
          item: it,
          key,
          status: "file_duplicate",
          isDuplicate: true,
          duplicateReason: `⚠️ 与文件内第 ${(firstIdxMap.get(key) || 0) + 1} 行重复`,
          existing
        };
      }

      if (existing) {
        const diff = diffFn(existing, it);
        if (diff.hasChanged) {
          return {
            index: idx,
            item: it,
            key,
            status: "db_update",
            isDuplicate: true,
            duplicateReason: `🟠 台账中已存在（${getDisplayName(existing)}），有 ${diff.diffs.length} 处配置变更`,
            existing,
            diffResult: diff
          };
        } else {
          return {
            index: idx,
            item: it,
            key,
            status: "db_identical",
            isDuplicate: true,
            duplicateReason: `⚠️ 台账中已存在完全相同记录（${getDisplayName(existing)}）`,
            existing,
            diffResult: diff
          };
        }
      }

      return {
        index: idx,
        item: it,
        key,
        status: "pure_new",
        isDuplicate: isFileDup,
        duplicateReason: isFileDup ? "⚠️ 文件内有多条此记录（此行为首条）" : undefined
      };
    });
  }

  // Analyzed lists for each dimension
  const hwAnalysis = useMemo(() => {
    return analyzeItems(
      workbookResult?.hardware || [],
      getAssetKey,
      existingHwMap,
      diffAssets,
      d => `${(d as any).projectName || ""} · ${(d as any).privateIp || (d as any).name}`
    );
  }, [workbookResult, existingHwMap]);

  const dbAnalysis = useMemo(() => {
    return analyzeItems(
      workbookResult?.databases || [],
      getDatabaseKey,
      existingDbMap,
      diffDatabases,
      d => `${d.projectName || ""} · ${d.dbSoftware} (${d.privateIp || d.hostIp})`
    );
  }, [workbookResult, existingDbMap]);

  const mwAnalysis = useMemo(() => {
    return analyzeItems(
      workbookResult?.middlewares || [],
      getMiddlewareKey,
      existingMwMap,
      diffMiddlewares,
      m => `${m.projectName || ""} · ${m.mwSoftware || m.name} (${m.privateIp})`
    );
  }, [workbookResult, existingMwMap]);

  const bkAnalysis = useMemo(() => {
    return analyzeItems(
      workbookResult?.backups || [],
      getBackupKey,
      existingBkMap,
      diffBackups,
      b => `${b.projectName || ""} · ${b.backupType || "备份"} (${b.privateIp})`
    );
  }, [workbookResult, existingBkMap]);

  const opsAnalysis = useMemo(() => {
    return analyzeItems(
      workbookResult?.opsRecords || [],
      getOpsKey,
      existingOpsMap,
      diffOps,
      o => `${o.projectName || ""} · ${o.opsVendor} (${o.privateIp})`
    );
  }, [workbookResult, existingOpsMap]);

  // Current active analysis and selection set
  const currentAnalysis = useMemo(() => {
    if (currentTab === "hardware") return hwAnalysis;
    if (currentTab === "database") return dbAnalysis;
    if (currentTab === "middleware") return mwAnalysis;
    if (currentTab === "backup") return bkAnalysis;
    return opsAnalysis;
  }, [currentTab, hwAnalysis, dbAnalysis, mwAnalysis, bkAnalysis, opsAnalysis]);

  const currentSelection = useMemo(() => {
    if (currentTab === "hardware") return selectedHw;
    if (currentTab === "database") return selectedDb;
    if (currentTab === "middleware") return selectedMw;
    if (currentTab === "backup") return selectedBk;
    return selectedOps;
  }, [currentTab, selectedHw, selectedDb, selectedMw, selectedBk, selectedOps]);

  function setCurrentSelection(fn: (prev: Set<number>) => Set<number>) {
    if (currentTab === "hardware") setSelectedHw(fn);
    else if (currentTab === "database") setSelectedDb(fn);
    else if (currentTab === "middleware") setSelectedMw(fn);
    else if (currentTab === "backup") setSelectedBk(fn);
    else setSelectedOps(fn);
  }

  // Filtered rows for active tab
  const filteredAnalysis = useMemo(() => {
    return currentAnalysis.filter(item => {
      if (previewFilter === "duplicate" && !item.isDuplicate) return false;
      if (previewFilter === "pure_new" && item.status !== "pure_new") return false;
      if (previewFilter === "db_update" && item.status !== "db_update") return false;
      if (previewFilter === "selected" && !currentSelection.has(item.index)) return false;

      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase();
        const rawObj = JSON.stringify(item.item).toLowerCase();
        const reason = (item.duplicateReason || "").toLowerCase();
        if (!rawObj.includes(q) && !reason.includes(q)) return false;
      }
      return true;
    });
  }, [currentAnalysis, previewFilter, currentSelection, searchQuery]);

  // Handle file drop/upload
  async function handleFileProcess(file: File) {
    if (!file.name.match(/\.(xlsx|xls)$/i)) {
      setErrorMsg("只支持上传 Excel 格式文件（.xlsx 或 .xls）");
      return;
    }
    setErrorMsg("");
    setLoading(true);
    setFileName(file.name);

    try {
      const res = await parseExcelWorkbook(file);
      if (res.totalRecords === 0) {
        throw new Error(
          "未能从该 Excel 中解析出有效的台账数据。请确保上传包含《02-硬件设备》、《03-数据库》、《04-中间件》、《05-备份》或《06-运维》规范表头的文件。"
        );
      }

      setWorkbookResult(res);

      // Auto select all valid records (by default, pure_new and db_update are selected, intra-file duplicates are skipped)
      const hwSel = new Set<number>();
      res.hardware.forEach((_, idx) => hwSel.add(idx));
      setSelectedHw(hwSel);

      const dbSel = new Set<number>();
      res.databases.forEach((_, idx) => dbSel.add(idx));
      setSelectedDb(dbSel);

      const mwSel = new Set<number>();
      res.middlewares.forEach((_, idx) => mwSel.add(idx));
      setSelectedMw(mwSel);

      const bkSel = new Set<number>();
      res.backups.forEach((_, idx) => bkSel.add(idx));
      setSelectedBk(bkSel);

      const opsSel = new Set<number>();
      res.opsRecords.forEach((_, idx) => opsSel.add(idx));
      setSelectedOps(opsSel);

      // Auto switch to matching tab or first populated tab
      const tabCounts: Record<DimensionType, number> = {
        hardware: res.hardware.length,
        database: res.databases.length,
        middleware: res.middlewares.length,
        backup: res.backups.length,
        ops: res.opsRecords.length
      };

      if (tabCounts[activeDimension] > 0) {
        setCurrentTab(activeDimension);
      } else {
        const firstPopulated = (Object.keys(tabCounts) as DimensionType[]).find(k => tabCounts[k] > 0);
        if (firstPopulated) setCurrentTab(firstPopulated);
      }
    } catch (err: any) {
      setErrorMsg(err.message || "解析 Excel 文件失败，请检查文件格式。");
      setWorkbookResult(null);
    } finally {
      setLoading(false);
    }
  }

  function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0];
    if (f) handleFileProcess(f);
  }

  function handleDrop(e: React.DragEvent) {
    e.preventDefault();
    setDragActive(false);
    const f = e.dataTransfer.files?.[0];
    if (f) handleFileProcess(f);
  }

  // Selection toggle
  function toggleSelect(idx: number) {
    setCurrentSelection(prev => {
      const next = new Set(prev);
      if (next.has(idx)) next.delete(idx);
      else next.add(idx);
      return next;
    });
  }

  function toggleSelectAll() {
    const visibleIndexes = filteredAnalysis.map(x => x.index);
    const allSelected = visibleIndexes.every(i => currentSelection.has(i));

    setCurrentSelection(prev => {
      const next = new Set(prev);
      if (allSelected) {
        visibleIndexes.forEach(i => next.delete(i));
      } else {
        visibleIndexes.forEach(i => next.add(i));
      }
      return next;
    });
  }

  // Confirm Import
  function handleConfirm() {
    if (!workbookResult) return;

    const selectedHardware = workbookResult.hardware.filter((_, i) => selectedHw.has(i));
    const selectedDatabases = workbookResult.databases.filter((_, i) => selectedDb.has(i));
    const selectedMiddlewares = workbookResult.middlewares.filter((_, i) => selectedMw.has(i));
    const selectedBackups = workbookResult.backups.filter((_, i) => selectedBk.has(i));
    const selectedOpsRecords = workbookResult.opsRecords.filter((_, i) => selectedOps.has(i));

    if (onConfirmImportMulti) {
      onConfirmImportMulti(
        {
          hardware: selectedHardware,
          databases: selectedDatabases,
          middlewares: selectedMiddlewares,
          backups: selectedBackups,
          opsRecords: selectedOpsRecords
        },
        strategy
      );
    } else if (onConfirmImport && selectedHardware.length > 0) {
      onConfirmImport(selectedHardware, strategy);
    }

    onClose();
  }

  const totalSelectedCount =
    selectedHw.size + selectedDb.size + selectedMw.size + selectedBk.size + selectedOps.size;

  return (
    <div className="cmdb-modal-mask" style={{ zIndex: 1100 }}>
      <div
        className="cmdb-modal"
        style={{
          width: "96vw",
          maxWidth: 1320,
          maxHeight: "92vh",
          display: "flex",
          flexDirection: "column",
          borderRadius: 12,
          overflow: "hidden",
          boxShadow: "0 25px 50px -12px rgba(0, 0, 0, 0.25)"
        }}
      >
        {/* Header */}
        <div
          className="cmdb-modal-header"
          style={{
            background: "linear-gradient(135deg, #1e40af, #3b82f6)",
            color: "#fff",
            padding: "14px 20px"
          }}
        >
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <span style={{ fontSize: 20 }}>📥</span>
            <div>
              <h3 style={{ margin: 0, fontSize: 16, fontWeight: 700, color: "#fff" }}>
                信息资产台账 Excel 智能导入向导 (支持 v351 五维全表 / 单 Sheet)
              </h3>
              <p style={{ margin: "2px 0 0", fontSize: 11, color: "#bfdbfe" }}>
                已对齐《信息资产台账-v351》规范 · 自动识别 02硬件、03数据库、04中间件、05备份、06运维五个 Sheet，智能查重与差异对比
              </p>
            </div>
          </div>
          <button
            type="button"
            className="cmdb-modal-close"
            style={{ color: "#fff", opacity: 0.8 }}
            onClick={onClose}
          >
            ×
          </button>
        </div>

        {/* Body */}
        <div style={{ padding: "14px 20px", flex: 1, overflowY: "auto", display: "flex", flexDirection: "column", gap: 12 }}>
          {/* Upload Area */}
          {!workbookResult ? (
            <div
              onDragOver={e => { e.preventDefault(); setDragActive(true); }}
              onDragLeave={() => setDragActive(false)}
              onDrop={handleDrop}
              onClick={() => fileInputRef.current?.click()}
              style={{
                border: `2px dashed ${dragActive ? "#2563eb" : "#cbd5e1"}`,
                borderRadius: 10,
                padding: "36px 20px",
                textAlign: "center",
                background: dragActive ? "#eff6ff" : "#f8fafc",
                cursor: "pointer",
                transition: "all 0.2s ease"
              }}
            >
              <input
                ref={fileInputRef}
                type="file"
                accept=".xlsx, .xls"
                style={{ display: "none" }}
                onChange={handleFileChange}
              />
              <div style={{ fontSize: 36, marginBottom: 8 }}>📊</div>
              <h4 style={{ margin: "0 0 6px", fontSize: 15, color: "#1e293b" }}>
                点击或将《信息资产台账》Excel 文件拖拽至此
              </h4>
              <p style={{ margin: 0, fontSize: 12, color: "#64748b" }}>
                支持完整《信息资产台账-v351.xlsx》（含 02-硬件设备、03-数据库、04-中间件、05-备份、06-运维 5个 Sheet），或任一独立导出的 Sheet 文件
              </p>
              {loading && (
                <div style={{ marginTop: 12, color: "#2563eb", fontSize: 13, fontWeight: 600 }}>
                  ⏳ 正在解析 Excel 工作表，请稍候...
                </div>
              )}
            </div>
          ) : (
            /* File summary bar */
            <div
              style={{
                background: "#f1f5f9",
                border: "1px solid #e2e8f0",
                borderRadius: 8,
                padding: "8px 14px",
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
                flexWrap: "wrap",
                gap: 10
              }}
            >
              <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                <span style={{ fontSize: 16 }}>📄</span>
                <span style={{ fontWeight: 600, color: "#0f172a", fontSize: 13 }}>{fileName}</span>
                <span style={{ fontSize: 11, background: "#dbeafe", color: "#1e40af", padding: "1px 8px", borderRadius: 4 }}>
                  已解析 {workbookResult.totalRecords} 条记录
                </span>
                <span style={{ fontSize: 11, color: "#64748b" }}>
                  (包含 {workbookResult.sheetsFound.map(s => `${s.name}: ${s.count}条`).join(" · ")})
                </span>
              </div>
              <button
                type="button"
                onClick={() => {
                  setWorkbookResult(null);
                  setFileName("");
                }}
                style={{
                  background: "#fff",
                  border: "1px solid #cbd5e1",
                  borderRadius: 6,
                  padding: "3px 10px",
                  fontSize: 11,
                  color: "#475569",
                  cursor: "pointer"
                }}
              >
                🔄 重新上传文件
              </button>
            </div>
          )}

          {errorMsg && (
            <div
              style={{
                background: "#fef2f2",
                border: "1px solid #fecaca",
                color: "#dc2626",
                borderRadius: 6,
                padding: "8px 12px",
                fontSize: 12
              }}
            >
              ⚠️ {errorMsg}
            </div>
          )}

          {/* Dimension Tabs & Content */}
          {workbookResult && (
            <div style={{ display: "flex", flexDirection: "column", gap: 10, flex: 1 }}>
              {/* 5-Dimension Sheet Tabs */}
              <div style={{ display: "flex", gap: 8, borderBottom: "2px solid #e2e8f0", paddingBottom: 6 }}>
                {[
                  { id: "hardware" as DimensionType, label: "02-硬件设备", icon: "🖥️", count: workbookResult.hardware.length, sel: selectedHw.size },
                  { id: "database" as DimensionType, label: "03-数据库", icon: "🗄️", count: workbookResult.databases.length, sel: selectedDb.size },
                  { id: "middleware" as DimensionType, label: "04-中间件", icon: "🧩", count: workbookResult.middlewares.length, sel: selectedMw.size },
                  { id: "backup" as DimensionType, label: "05-备份", icon: "💾", count: workbookResult.backups.length, sel: selectedBk.size },
                  { id: "ops" as DimensionType, label: "06-运维", icon: "🛡️", count: workbookResult.opsRecords.length, sel: selectedOps.size }
                ].map(tab => {
                  const isActive = currentTab === tab.id;
                  const isPopulated = tab.count > 0;
                  return (
                    <button
                      key={tab.id}
                      type="button"
                      onClick={() => {
                        setCurrentTab(tab.id);
                        setPreviewFilter("all");
                        setSearchQuery("");
                      }}
                      style={{
                        padding: "6px 14px",
                        borderRadius: 6,
                        border: "none",
                        background: isActive ? "#2563eb" : isPopulated ? "#f1f5f9" : "#f8fafc",
                        color: isActive ? "#fff" : isPopulated ? "#1e293b" : "#94a3b8",
                        fontWeight: isActive ? 700 : 500,
                        fontSize: 12,
                        cursor: "pointer",
                        display: "flex",
                        alignItems: "center",
                        gap: 6,
                        boxShadow: isActive ? "0 2px 4px rgba(37,99,235,0.25)" : "none"
                      }}
                    >
                      <span>{tab.icon}</span>
                      <span>{tab.label}</span>
                      <span
                        style={{
                          fontSize: 10,
                          padding: "1px 6px",
                          borderRadius: 10,
                          background: isActive ? "rgba(255,255,255,0.25)" : isPopulated ? "#e2e8f0" : "transparent",
                          color: isActive ? "#fff" : isPopulated ? "#334155" : "#94a3b8"
                        }}
                      >
                        {tab.count} 条 (已选 {tab.sel})
                      </span>
                    </button>
                  );
                })}
              </div>

              {/* Action & Strategy Bar */}
              <div
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                  flexWrap: "wrap",
                  gap: 10,
                  background: "#f8fafc",
                  padding: "8px 12px",
                  borderRadius: 8,
                  border: "1px solid #e2e8f0"
                }}
              >
                {/* Left: Strategy */}
                <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                  <span style={{ fontSize: 12, fontWeight: 600, color: "#334155" }}>入库策略:</span>
                  <label style={{ display: "flex", alignItems: "center", gap: 4, fontSize: 12, cursor: "pointer" }}>
                    <input
                      type="radio"
                      name="importStrategy"
                      checked={strategy === "upsert"}
                      onChange={() => setStrategy("upsert")}
                    />
                    <strong style={{ color: "#2563eb" }}>智能覆盖更新 (upsert)</strong>
                    <span style={{ color: "#64748b", fontSize: 11 }}>新资产录入，已有资产合并变更</span>
                  </label>
                  <label style={{ display: "flex", alignItems: "center", gap: 4, fontSize: 12, cursor: "pointer" }}>
                    <input
                      type="radio"
                      name="importStrategy"
                      checked={strategy === "skip"}
                      onChange={() => setStrategy("skip")}
                    />
                    <span>仅导入新增 (skip)</span>
                  </label>
                  <label style={{ display: "flex", alignItems: "center", gap: 4, fontSize: 12, cursor: "pointer" }}>
                    <input
                      type="radio"
                      name="importStrategy"
                      checked={strategy === "replace"}
                      onChange={() => setStrategy("replace")}
                    />
                    <span style={{ color: "#dc2626" }}>全量替换 (replace)</span>
                  </label>
                </div>

                {/* Right: Search */}
                <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                  <input
                    type="text"
                    placeholder={`在当前 ${currentTab} 中搜索 IP、名称或配置...`}
                    value={searchQuery}
                    onChange={e => setSearchQuery(e.target.value)}
                    style={{
                      padding: "4px 10px",
                      fontSize: 12,
                      border: "1px solid #cbd5e1",
                      borderRadius: 6,
                      width: 240
                    }}
                  />
                  {searchQuery && (
                    <button
                      type="button"
                      onClick={() => setSearchQuery("")}
                      style={{ fontSize: 11, background: "none", border: "none", color: "#64748b", cursor: "pointer" }}
                    >
                      清空
                    </button>
                  )}
                </div>
              </div>

              {/* Sub-Filters */}
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                <div style={{ display: "flex", gap: 6 }}>
                  {[
                    { id: "all" as const, label: `全部 (${currentAnalysis.length})` },
                    { id: "pure_new" as const, label: `纯新录入 (${currentAnalysis.filter(x => x.status === "pure_new").length})` },
                    { id: "db_update" as const, label: `配置变更 (${currentAnalysis.filter(x => x.status === "db_update").length})` },
                    { id: "duplicate" as const, label: `已有重复 (${currentAnalysis.filter(x => x.isDuplicate).length})` },
                    { id: "selected" as const, label: `已勾选 (${currentSelection.size})` }
                  ].map(f => (
                    <button
                      key={f.id}
                      type="button"
                      onClick={() => setPreviewFilter(f.id)}
                      style={{
                        padding: "3px 10px",
                        borderRadius: 4,
                        border: "1px solid",
                        borderColor: previewFilter === f.id ? "#93c5fd" : "#e2e8f0",
                        background: previewFilter === f.id ? "#eff6ff" : "#fff",
                        color: previewFilter === f.id ? "#1d4ed8" : "#475569",
                        fontWeight: previewFilter === f.id ? 600 : 400,
                        fontSize: 11,
                        cursor: "pointer"
                      }}
                    >
                      {f.label}
                    </button>
                  ))}
                </div>

                <button
                  type="button"
                  onClick={toggleSelectAll}
                  style={{
                    background: "#f1f5f9",
                    border: "1px solid #cbd5e1",
                    borderRadius: 4,
                    padding: "3px 8px",
                    fontSize: 11,
                    color: "#334155",
                    cursor: "pointer"
                  }}
                >
                  {filteredAnalysis.every(x => currentSelection.has(x.index)) ? "取消全选本页" : "全选本页记录"}
                </button>
              </div>

              {/* Preview Table */}
              <div
                style={{
                  border: "1px solid #e2e8f0",
                  borderRadius: 8,
                  overflowX: "auto",
                  maxHeight: 380,
                  background: "#fff"
                }}
              >
                <table className="cmdb-table" style={{ width: "100%", fontSize: 12, borderCollapse: "collapse" }}>
                  <thead style={{ position: "sticky", top: 0, background: "#f8fafc", zIndex: 5 }}>
                    <tr>
                      <th style={{ width: 36, textAlign: "center" }}>
                        <input
                          type="checkbox"
                          checked={filteredAnalysis.length > 0 && filteredAnalysis.every(x => currentSelection.has(x.index))}
                          onChange={toggleSelectAll}
                        />
                      </th>
                      <th style={{ width: 45 }}>序号</th>
                      <th style={{ width: 110 }}>状态 / 查重</th>

                      {currentTab === "hardware" && (
                        <>
                          <th>设备名称</th>
                          <th>项目名称</th>
                          <th>私有IP (业务IP)</th>
                          <th>算力配置</th>
                          <th>操作系统</th>
                          <th>信创</th>
                          <th>远程端口</th>
                        </>
                      )}

                      {currentTab === "database" && (
                        <>
                          <th>数据库软件</th>
                          <th>私有IP (业务IP)</th>
                          <th>项目名称</th>
                          <th>监听端口</th>
                          <th>实例/SID</th>
                          <th>业务库名</th>
                          <th>部署模式</th>
                        </>
                      )}

                      {currentTab === "middleware" && (
                        <>
                          <th>中间件软件</th>
                          <th>私有IP (业务IP)</th>
                          <th>项目名称</th>
                          <th>中间件类型</th>
                          <th>版本</th>
                          <th>服务端口</th>
                        </>
                      )}

                      {currentTab === "backup" && (
                        <>
                          <th>私有IP (业务IP)</th>
                          <th>项目名称</th>
                          <th>备份类型</th>
                          <th>备份方式</th>
                          <th>备份策略</th>
                          <th>存储位置</th>
                        </>
                      )}

                      {currentTab === "ops" && (
                        <>
                          <th>私有IP (业务IP)</th>
                          <th>项目名称</th>
                          <th>运维厂商</th>
                          <th>VPN账号</th>
                          <th>堡垒机账号</th>
                          <th>访问服务器地址</th>
                        </>
                      )}

                      <th>变更说明 / 备注</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredAnalysis.length === 0 ? (
                      <tr>
                        <td colSpan={10} style={{ textAlign: "center", padding: "30px 0", color: "#94a3b8" }}>
                          没有符合当前筛选条件的记录
                        </td>
                      </tr>
                    ) : (
                      filteredAnalysis.map(row => {
                        const isChecked = currentSelection.has(row.index);
                        const it: any = row.item;

                        return (
                          <tr
                            key={row.index}
                            style={{
                              background: isChecked ? (row.isDuplicate ? "#fffbeb" : "#f0fdf4") : undefined,
                              cursor: "pointer"
                            }}
                            onClick={() => toggleSelect(row.index)}
                          >
                            <td style={{ textAlign: "center" }} onClick={e => e.stopPropagation()}>
                              <input
                                type="checkbox"
                                checked={isChecked}
                                onChange={() => toggleSelect(row.index)}
                              />
                            </td>
                            <td>{it.seq || (row.index + 1)}</td>
                            <td>
                              {row.status === "pure_new" && (
                                <span style={{ background: "#dcfce7", color: "#166534", padding: "1px 6px", borderRadius: 4, fontSize: 10, fontWeight: 600 }}>
                                  🟢 纯新录入
                                </span>
                              )}
                              {row.status === "db_update" && (
                                <span style={{ background: "#fef3c7", color: "#92400e", padding: "1px 6px", borderRadius: 4, fontSize: 10, fontWeight: 600 }}>
                                  🟠 配置更新
                                </span>
                              )}
                              {row.status === "db_identical" && (
                                <span style={{ background: "#f1f5f9", color: "#64748b", padding: "1px 6px", borderRadius: 4, fontSize: 10 }}>
                                  ⚪ 完全相同
                                </span>
                              )}
                              {row.status === "file_duplicate" && (
                                <span style={{ background: "#fee2e2", color: "#b91c1c", padding: "1px 6px", borderRadius: 4, fontSize: 10, fontWeight: 600 }}>
                                  ⚠️ 文件内重复
                                </span>
                              )}
                            </td>

                            {/* Dimension Columns */}
                            {currentTab === "hardware" && (
                              <>
                                <td style={{ fontWeight: 600, color: "#1e293b" }}>{it.name}</td>
                                <td>{it.projectName}</td>
                                <td style={{ fontFamily: "monospace", color: "#2563eb", fontWeight: 600 }}>{it.privateIp || it.ip || "-"}</td>
                                <td>{it.cpu} · {it.memory}</td>
                                <td>{it.os}</td>
                                <td>
                                  <span style={{ fontSize: 10, padding: "1px 5px", borderRadius: 3, background: it.isXinchuang === "是" ? "#fee2e2" : "#f1f5f9", color: it.isXinchuang === "是" ? "#dc2626" : "#64748b" }}>
                                    {it.isXinchuang === "是" ? "信创" : "非信创"}
                                  </span>
                                </td>
                                <td style={{ fontFamily: "monospace" }}>{it.remotePort || 22}</td>
                              </>
                            )}

                            {currentTab === "database" && (
                              <>
                                <td style={{ fontWeight: 600, color: "#4338ca" }}>{it.dbSoftware || it.type}</td>
                                <td style={{ fontFamily: "monospace", color: "#2563eb", fontWeight: 600 }}>{it.privateIp || it.hostIp || "-"}</td>
                                <td>{it.projectName}</td>
                                <td style={{ fontFamily: "monospace" }}>{it.port}</td>
                                <td>{it.instanceSid || "-"}</td>
                                <td>{it.dbName || "-"}</td>
                                <td>{it.deployMode || "单机"}</td>
                              </>
                            )}

                            {currentTab === "middleware" && (
                              <>
                                <td style={{ fontWeight: 600, color: "#0d9488" }}>{it.mwSoftware || it.name}</td>
                                <td style={{ fontFamily: "monospace", color: "#2563eb", fontWeight: 600 }}>{it.privateIp || "-"}</td>
                                <td>{it.projectName}</td>
                                <td>{it.mwType}</td>
                                <td>{it.version || "-"}</td>
                                <td style={{ fontFamily: "monospace" }}>{it.port || "-"}</td>
                              </>
                            )}

                            {currentTab === "backup" && (
                              <>
                                <td style={{ fontFamily: "monospace", color: "#2563eb", fontWeight: 600 }}>{it.privateIp || "-"}</td>
                                <td>{it.projectName}</td>
                                <td>{it.backupType || "数据库"}</td>
                                <td>{it.backupMethod || "物理备份"}</td>
                                <td>{it.backupPolicy || "-"}</td>
                                <td>{it.storageLocation || "-"}</td>
                              </>
                            )}

                            {currentTab === "ops" && (
                              <>
                                <td style={{ fontFamily: "monospace", color: "#2563eb", fontWeight: 600 }}>{it.privateIp || "-"}</td>
                                <td>{it.projectName}</td>
                                <td style={{ fontWeight: 600 }}>{it.opsVendor}</td>
                                <td>{it.vpnAccount || "-"}</td>
                                <td>{it.bastionAccount || "-"}</td>
                                <td style={{ fontFamily: "monospace", fontSize: 11 }}>{it.serverAccessAddress || "-"}</td>
                              </>
                            )}

                            <td>
                              {row.diffResult?.diffs ? (
                                <div style={{ display: "flex", gap: 4, flexWrap: "wrap" }}>
                                  {row.diffResult.diffs.map((df, di) => (
                                    <span key={di} style={{ color: "#b45309", fontSize: 10, background: "#fffbeb", border: "1px solid #fde68a", padding: "1px 5px", borderRadius: 3 }}>
                                      {df.label}: {String(df.oldVal)} ➔ {String(df.newVal)}
                                    </span>
                                  ))}
                                </div>
                              ) : (
                                <span style={{ color: "#64748b", fontSize: 11 }}>{row.duplicateReason || it.remarks || "—"}</span>
                              )}
                            </td>
                          </tr>
                        );
                      })
                    )}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div
          style={{
            padding: "12px 20px",
            borderTop: "1px solid #e2e8f0",
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            background: "#f8fafc"
          }}
        >
          <div style={{ fontSize: 12, color: "#64748b" }}>
            {workbookResult ? (
              <span>
                全套已选: 硬件 <strong style={{ color: "#2563eb" }}>{selectedHw.size}</strong> 台 · 
                数据库 <strong style={{ color: "#4338ca" }}>{selectedDb.size}</strong> 条 · 
                中间件 <strong style={{ color: "#0d9488" }}>{selectedMw.size}</strong> 条 · 
                备份 <strong style={{ color: "#ea580c" }}>{selectedBk.size}</strong> 条 · 
                运维 <strong style={{ color: "#059669" }}>{selectedOps.size}</strong> 条
              </span>
            ) : (
              <span>请先上传 Excel 文件</span>
            )}
          </div>
          <div style={{ display: "flex", gap: 10 }}>
            <button type="button" className="btn-secondary" onClick={onClose}>
              取消
            </button>
            <button
              type="button"
              className="btn-primary"
              disabled={!workbookResult || totalSelectedCount === 0 || loading}
              onClick={handleConfirm}
              style={{
                opacity: !workbookResult || totalSelectedCount === 0 ? 0.5 : 1,
                cursor: !workbookResult || totalSelectedCount === 0 ? "not-allowed" : "pointer",
                background: totalSelectedCount === 0 ? "#94a3b8" : "#2563eb",
                display: "flex",
                alignItems: "center",
                gap: 6
              }}
            >
              <span>✓ 确认导入已选数据 (共 {totalSelectedCount} 条记录)</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
