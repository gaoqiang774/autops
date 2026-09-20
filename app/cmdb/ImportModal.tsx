"use client";
import React, { useState, useRef, useMemo } from "react";
import { VmHost } from "../cmdbData";
import { parseExcelToAssets, getAssetKey, diffAssets, AssetDiffResult } from "./excelExport";

export type ImportStrategy = "upsert" | "skip" | "replace";

export interface ImportItemAnalysis {
  index: number;
  device: VmHost & { isImported?: boolean };
  key: string;
  status: "pure_new" | "db_update" | "db_identical" | "file_duplicate";
  isDuplicate: boolean;
  duplicateReason?: string;
  existing?: VmHost;
  diffResult?: AssetDiffResult;
}

interface ImportModalProps {
  targetProjectName?: string | null;
  existingAssets?: (VmHost | any)[];
  onClose: () => void;
  onConfirmImport: (
    importedAssets: (VmHost & { isImported?: boolean })[],
    strategy: ImportStrategy
  ) => void;
}

export default function ImportModal({
  targetProjectName,
  existingAssets = [],
  onClose,
  onConfirmImport
}: ImportModalProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [dragActive, setDragActive] = useState(false);
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState("");
  const [fileName, setFileName] = useState("");
  const [sheetName, setSheetName] = useState("");
  const [parsedList, setParsedList] = useState<(VmHost & { isImported?: boolean })[]>([]);
  
  // Strategy: default to upsert (smart update, zero duplicate)
  const [strategy, setStrategy] = useState<ImportStrategy>("upsert");
  
  // Filter tab for preview: all / duplicate / pure_new / db_update / selected
  const [previewFilter, setPreviewFilter] = useState<"all" | "duplicate" | "pure_new" | "db_update" | "selected">("all");
  
  // Search keyword inside modal
  const [searchQuery, setSearchQuery] = useState("");

  // Manually selected item indexes (Set of indexes corresponding to parsedList)
  const [selectedIndexes, setSelectedIndexes] = useState<Set<number>>(new Set());

  // Build existing map for quick diffing and duplicate check
  const existingMap = useMemo(() => {
    const map = new Map<string, VmHost>();
    for (const item of existingAssets) {
      const k = getAssetKey(item);
      if (!map.has(k)) {
        map.set(k, item);
      }
    }
    return map;
  }, [existingAssets]);

  // Analyze parsed items vs existing DB assets and intra-file occurrences
  const analysisList: ImportItemAnalysis[] = useMemo(() => {
    if (parsedList.length === 0) return [];

    // Step 1: count key frequencies within current uploaded file
    const fileKeyCount = new Map<string, number>();
    const fileKeyFirstIndex = new Map<string, number>();
    parsedList.forEach((item, idx) => {
      const k = getAssetKey(item);
      fileKeyCount.set(k, (fileKeyCount.get(k) || 0) + 1);
      if (!fileKeyFirstIndex.has(k)) {
        fileKeyFirstIndex.set(k, idx);
      }
    });

    // Step 2: analyze each parsed row
    return parsedList.map((item, idx) => {
      const key = getAssetKey(item);
      const isFileDup = (fileKeyCount.get(key) || 0) > 1;
      const isFileFollower = isFileDup && fileKeyFirstIndex.get(key) !== idx;
      const existing = existingMap.get(key);

      // Case 1: Intra-file duplicate (another row in this file has the exact same key)
      if (isFileFollower) {
        const firstRow = (fileKeyFirstIndex.get(key) || 0) + 1;
        return {
          index: idx,
          device: item,
          key,
          status: "file_duplicate",
          isDuplicate: true,
          duplicateReason: `⚠️ 与文件内第 ${firstRow} 行重复（相同业务IP/标识）`,
          existing
        };
      }

      // Case 2: Existing asset in database
      if (existing) {
        const diff = diffAssets(existing, item);
        if (diff.hasChanged) {
          return {
            index: idx,
            device: item,
            key,
            status: "db_update",
            isDuplicate: true,
            duplicateReason: `🟠 台账中已存在（${existing.projectName || "已有资产"} · ${existing.privateIp || existing.name}），有 ${diff.diffs.length} 处配置变更`,
            existing,
            diffResult: diff
          };
        } else {
          return {
            index: idx,
            device: item,
            key,
            status: "db_identical",
            isDuplicate: true,
            duplicateReason: `⚠️ 台账中已存在此设备且配置相同（${existing.projectName || "已有资产"} · ${existing.privateIp || existing.name}）`,
            existing,
            diffResult: diff
          };
        }
      }

      // Case 3: Pure new asset
      return {
        index: idx,
        device: item,
        key,
        status: "pure_new",
        isDuplicate: isFileDup, // first of file dup is flagged if repeats later
        duplicateReason: isFileDup ? "⚠️ 文件内有多条此记录（此行为首条）" : undefined
      };
    });
  }, [parsedList, existingMap]);

  // Statistics
  const counts = useMemo(() => {
    let pureNewCount = 0;
    let dbUpdateCount = 0;
    let dbIdenticalCount = 0;
    let fileDuplicateCount = 0;
    let duplicateCount = 0;

    for (const a of analysisList) {
      if (a.status === "pure_new") pureNewCount++;
      else if (a.status === "db_update") {
        dbUpdateCount++;
        duplicateCount++;
      } else if (a.status === "db_identical") {
        dbIdenticalCount++;
        duplicateCount++;
      } else if (a.status === "file_duplicate") {
        fileDuplicateCount++;
        duplicateCount++;
      }
    }

    return {
      pureNewCount,
      dbUpdateCount,
      dbIdenticalCount,
      fileDuplicateCount,
      duplicateCount
    };
  }, [analysisList]);

  // Filtered preview list
  const filteredAnalysis = useMemo(() => {
    let list = analysisList;

    // Filter tab
    if (previewFilter === "duplicate") {
      list = list.filter(a => a.isDuplicate);
    } else if (previewFilter === "pure_new") {
      list = list.filter(a => a.status === "pure_new");
    } else if (previewFilter === "db_update") {
      list = list.filter(a => a.status === "db_update");
    } else if (previewFilter === "selected") {
      list = list.filter(a => selectedIndexes.has(a.index));
    }

    // Search query filter
    if (searchQuery.trim()) {
      const q = searchQuery.trim().toLowerCase();
      list = list.filter(a => {
        const d = a.device;
        const name = (d.name || "").toLowerCase();
        const ip = (d.privateIp || d.ip || "").toLowerCase();
        const proj = (d.projectName || "").toLowerCase();
        const reason = (a.duplicateReason || "").toLowerCase();
        return name.includes(q) || ip.includes(q) || proj.includes(q) || reason.includes(q);
      });
    }

    return list;
  }, [analysisList, previewFilter, searchQuery, selectedIndexes]);

  // Count how many of currently selected items are duplicates
  const selectedDuplicateCount = useMemo(() => {
    let count = 0;
    for (const a of analysisList) {
      if (selectedIndexes.has(a.index) && a.isDuplicate) {
        count++;
      }
    }
    return count;
  }, [analysisList, selectedIndexes]);

  async function handleFile(file: File) {
    if (!file.name.endsWith(".xlsx") && !file.name.endsWith(".xls")) {
      setErrorMsg("请上传 Excel 文件格式（.xlsx 或 .xls）");
      return;
    }

    setLoading(true);
    setErrorMsg("");
    setFileName(file.name);

    try {
      const res = await parseExcelToAssets(file);
      if (res.devices.length === 0) {
        setErrorMsg("文件中未解析到有效的设备数据，请检查工作表中是否包含「设备名称」或「私有IP」列。");
        setParsedList([]);
        setSelectedIndexes(new Set());
      } else {
        // If importing inside a specific project, align project name
        let finalDevices = res.devices;
        if (targetProjectName && targetProjectName !== "全部项目总览") {
          finalDevices = res.devices.map(d => ({
            ...d,
            projectName: targetProjectName
          }));
        }
        setParsedList(finalDevices);
        setSheetName(res.sheetName);

        // Pre-select: automatically select pure new items, exclude duplicates
        // User can manually review and check duplicates if they wish
        const initialSelected = new Set<number>();
        const seenInFile = new Set<string>();
        finalDevices.forEach((item, idx) => {
          const k = getAssetKey(item);
          const isDbDup = existingMap.has(k);
          const isFileDup = seenInFile.has(k);
          seenInFile.add(k);

          // By default, select purely new records
          if (!isDbDup && !isFileDup) {
            initialSelected.add(idx);
          }
        });

        setSelectedIndexes(initialSelected);
      }
    } catch (err: any) {
      setErrorMsg(err.message || "解析 Excel 文件失败，请核实文件格式。");
      setParsedList([]);
      setSelectedIndexes(new Set());
    } finally {
      setLoading(false);
    }
  }

  function handleDrop(e: React.DragEvent) {
    e.preventDefault();
    setDragActive(false);
    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      handleFile(e.dataTransfer.files[0]);
    }
  }

  // Toggle single row selection
  function toggleSelect(index: number) {
    setSelectedIndexes(prev => {
      const next = new Set(prev);
      if (next.has(index)) {
        next.delete(index);
      } else {
        next.add(index);
      }
      return next;
    });
  }

  // Quick Batch Selection Actions
  function handleSelectAll() {
    const all = new Set<number>();
    parsedList.forEach((_, idx) => all.add(idx));
    setSelectedIndexes(all);
  }

  function handleSelectPureNewOnly() {
    const pureNew = new Set<number>();
    analysisList.forEach(a => {
      if (!a.isDuplicate) {
        pureNew.add(a.index);
      }
    });
    setSelectedIndexes(pureNew);
  }

  function handleSelectNewAndUpdate() {
    const set = new Set<number>();
    analysisList.forEach(a => {
      if (!a.isDuplicate || a.status === "db_update") {
        set.add(a.index);
      }
    });
    setSelectedIndexes(set);
  }

  function handleSelectDuplicatesOnly() {
    const dups = new Set<number>();
    analysisList.forEach(a => {
      if (a.isDuplicate) {
        dups.add(a.index);
      }
    });
    setSelectedIndexes(dups);
  }

  function handleClearAll() {
    setSelectedIndexes(new Set());
  }

  // Toggle selection for all visible rows in current filtered list
  const isAllFilteredSelected = filteredAnalysis.length > 0 && filteredAnalysis.every(a => selectedIndexes.has(a.index));
  function toggleSelectFiltered() {
    setSelectedIndexes(prev => {
      const next = new Set(prev);
      if (isAllFilteredSelected) {
        filteredAnalysis.forEach(a => next.delete(a.index));
      } else {
        filteredAnalysis.forEach(a => next.add(a.index));
      }
      return next;
    });
  }

  // Confirm import of only user-selected devices
  function handleConfirm() {
    if (parsedList.length === 0 || selectedIndexes.size === 0) return;
    const selectedDevices = parsedList.filter((_, idx) => selectedIndexes.has(idx));
    onConfirmImport(selectedDevices, strategy);
    onClose();
  }

  return (
    <div className="modal-backdrop" style={{
      position: "fixed",
      top: 0,
      left: 0,
      right: 0,
      bottom: 0,
      backgroundColor: "rgba(15, 23, 42, 0.68)",
      backdropFilter: "blur(5px)",
      zIndex: 9999,
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
      padding: 16
    }}>
      <div style={{
        background: "#fff",
        borderRadius: 12,
        width: "100%",
        maxWidth: 1040,
        maxHeight: "94vh",
        display: "flex",
        flexDirection: "column",
        boxShadow: "0 25px 50px -12px rgba(0, 0, 0, 0.35)",
        overflow: "hidden",
        border: "1px solid #cbd5e1"
      }}>
        {/* Header */}
        <div style={{
          padding: "14px 20px",
          borderBottom: "1px solid #e2e8f0",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          background: "#f8fafc"
        }}>
          <div>
            <h3 style={{ margin: 0, fontSize: 16, fontWeight: 700, color: "#0f172a", display: "flex", alignItems: "center", gap: 8 }}>
              <span>📥 导入《信息资产台账》Excel 设备数据</span>
              {targetProjectName && (
                <span style={{ fontSize: 12, background: "#dbeafe", color: "#1e40af", padding: "2px 8px", borderRadius: 12, fontWeight: 500 }}>
                  目标项目: {targetProjectName}
                </span>
              )}
            </h3>
            <p style={{ margin: "3px 0 0", fontSize: 12, color: "#64748b" }}>
              导入前智能查重校验 · 手工核对重复记录并自主勾选入库 · 杜绝产生重复资产
            </p>
          </div>
          <button 
            type="button" 
            onClick={onClose}
            style={{ border: "none", background: "transparent", fontSize: 22, cursor: "pointer", color: "#64748b", padding: "2px 8px" }}
          >
            ✕
          </button>
        </div>

        {/* Content Body */}
        <div style={{ padding: "16px 20px", overflowY: "auto", flex: 1, display: "flex", flexDirection: "column", gap: 12 }}>
          {/* Upload Area */}
          <div
            onDragOver={e => { e.preventDefault(); setDragActive(true); }}
            onDragLeave={() => setDragActive(false)}
            onDrop={handleDrop}
            onClick={() => fileInputRef.current?.click()}
            style={{
              border: dragActive ? "2px dashed #2563eb" : "2px dashed #cbd5e1",
              background: dragActive ? "#eff6ff" : "#f8fafc",
              borderRadius: 8,
              padding: "16px 20px",
              textAlign: "center",
              cursor: "pointer",
              transition: "all 0.2s"
            }}
          >
            <input 
              ref={fileInputRef} 
              type="file" 
              accept=".xlsx,.xls" 
              style={{ display: "none" }}
              onChange={e => {
                if (e.target.files && e.target.files[0]) {
                  handleFile(e.target.files[0]);
                }
              }}
            />
            <div style={{ fontSize: 26, marginBottom: 4 }}>📊</div>
            <div style={{ fontWeight: 600, fontSize: 14, color: "#1e293b", marginBottom: 2 }}>
              {loading ? "正在解析 Excel 台账并深度执行去重比对..." : fileName ? `已解析文件: ${fileName}` : "点击选择 或 将《信息资产台账》Excel 文件拖拽至此处"}
            </div>
            <div style={{ fontSize: 12, color: "#64748b" }}>
              自动识别「02-硬件设备」工作表，基于业务IP及设备名称自动进行比对判定
            </div>
          </div>

          {/* Error message */}
          {errorMsg && (
            <div style={{ padding: "10px 14px", borderRadius: 6, background: "#fef2f2", border: "1px solid #fecaca", color: "#dc2626", fontSize: 12 }}>
              ⚠️ {errorMsg}
            </div>
          )}

          {/* Duplicate Detection Alert & Manual Decision Bar */}
          {parsedList.length > 0 && (
            <div style={{
              background: counts.duplicateCount > 0 ? "#fffbeb" : "#f0fdf4",
              border: counts.duplicateCount > 0 ? "1.5px solid #fcd34d" : "1.5px solid #bbf7d0",
              borderRadius: 8,
              padding: "12px 16px",
              display: "flex",
              flexDirection: "column",
              gap: 10,
              boxShadow: "0 1px 3px rgba(0,0,0,0.05)"
            }}>
              <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 12, flexWrap: "wrap" }}>
                <div style={{ display: "flex", alignItems: "flex-start", gap: 10 }}>
                  <span style={{ fontSize: 22, marginTop: -2 }}>
                    {counts.duplicateCount > 0 ? "⚠️" : "✨"}
                  </span>
                  <div>
                    <strong style={{ fontSize: 13, color: counts.duplicateCount > 0 ? "#92400e" : "#15803d" }}>
                      {counts.duplicateCount > 0 
                        ? `检测到待导入数据中存在 ${counts.duplicateCount} 台重复设备（库中已有 ${counts.dbUpdateCount + counts.dbIdenticalCount} 台，文件内部重复 ${counts.fileDuplicateCount} 台）`
                        : `查验通过！文件中全部 ${parsedList.length} 台设备均为全新设备，无任何重复记录`}
                    </strong>
                    <div style={{ fontSize: 12, color: counts.duplicateCount > 0 ? "#b45309" : "#166534", marginTop: 2 }}>
                      {counts.duplicateCount > 0
                        ? `系统已默认仅勾选 ${counts.pureNewCount} 台全新设备，排除了所有重复记录。请您在下方手工核对重复项，按需勾选确认要导入的设备。未勾选的重复记录将不会被导入。`
                        : `已默认全选所有全新设备，您可以直接点击下方确认导入。`}
                    </div>
                  </div>
                </div>

                {/* Badges */}
                <div style={{ display: "flex", gap: 6, flexWrap: "wrap", alignItems: "center" }}>
                  <span style={{ fontSize: 11, fontWeight: 700, padding: "3px 8px", borderRadius: 12, background: "#dcfce7", color: "#15803d", border: "1px solid #bbf7d0" }}>
                    🟢 纯新设备: {counts.pureNewCount} 台
                  </span>
                  {counts.duplicateCount > 0 && (
                    <span style={{ fontSize: 11, fontWeight: 700, padding: "3px 8px", borderRadius: 12, background: "#fee2e2", color: "#dc2626", border: "1px solid #fecaca" }}>
                      ⚠️ 重复总计: {counts.duplicateCount} 台
                    </span>
                  )}
                  {counts.dbUpdateCount > 0 && (
                    <span style={{ fontSize: 11, fontWeight: 700, padding: "3px 8px", borderRadius: 12, background: "#fef3c7", color: "#b45309", border: "1px solid #fde68a" }}>
                      🟠 包含配置变更: {counts.dbUpdateCount} 台
                    </span>
                  )}
                  <span style={{ fontSize: 11, fontWeight: 700, padding: "3px 8px", borderRadius: 12, background: "#eff6ff", color: "#1d4ed8", border: "1px solid #bfdbfe" }}>
                    ✓ 当前已选: {selectedIndexes.size} 台
                  </span>
                </div>
              </div>

              {/* Fast Manual Selection Buttons */}
              <div style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                flexWrap: "wrap",
                gap: 8,
                borderTop: "1px dashed rgba(0,0,0,0.1)",
                paddingTop: 8
              }}>
                <div style={{ fontSize: 12, color: "#475569", fontWeight: 600 }}>
                  ⚡ 手工快捷选择：
                </div>
                <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
                  <button
                    type="button"
                    onClick={handleSelectPureNewOnly}
                    style={{
                      fontSize: 11,
                      padding: "4px 10px",
                      borderRadius: 6,
                      background: "#dcfce7",
                      color: "#166534",
                      border: "1px solid #86efac",
                      fontWeight: 600,
                      cursor: "pointer"
                    }}
                  >
                    🟢 仅选全新设备 ({counts.pureNewCount})
                  </button>

                  {counts.dbUpdateCount > 0 && (
                    <button
                      type="button"
                      onClick={handleSelectNewAndUpdate}
                      style={{
                        fontSize: 11,
                        padding: "4px 10px",
                        borderRadius: 6,
                        background: "#fef3c7",
                        color: "#92400e",
                        border: "1px solid #fcd34d",
                        fontWeight: 600,
                        cursor: "pointer"
                      }}
                    >
                      🟠 勾选全新 + 变更更新 ({counts.pureNewCount + counts.dbUpdateCount})
                    </button>
                  )}

                  {counts.duplicateCount > 0 && (
                    <button
                      type="button"
                      onClick={handleSelectDuplicatesOnly}
                      style={{
                        fontSize: 11,
                        padding: "4px 10px",
                        borderRadius: 6,
                        background: "#fee2e2",
                        color: "#991b1b",
                        border: "1px solid #fca5a5",
                        fontWeight: 600,
                        cursor: "pointer"
                      }}
                    >
                      ⚠️ 仅选重复设备 ({counts.duplicateCount})
                    </button>
                  )}

                  <button
                    type="button"
                    onClick={handleSelectAll}
                    style={{
                      fontSize: 11,
                      padding: "4px 10px",
                      borderRadius: 6,
                      background: "#eff6ff",
                      color: "#1e40af",
                      border: "1px solid #bfdbfe",
                      fontWeight: 600,
                      cursor: "pointer"
                    }}
                  >
                    ✓ 全选所有 ({parsedList.length})
                  </button>

                  <button
                    type="button"
                    onClick={handleClearAll}
                    style={{
                      fontSize: 11,
                      padding: "4px 10px",
                      borderRadius: 6,
                      background: "#f1f5f9",
                      color: "#64748b",
                      border: "1px solid #cbd5e1",
                      fontWeight: 600,
                      cursor: "pointer"
                    }}
                  >
                    ✕ 全部取消
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* Strategy Selector */}
          {parsedList.length > 0 && (
            <div style={{ background: "#f8fafc", border: "1px solid #e2e8f0", borderRadius: 8, padding: "10px 14px" }}>
              <div style={{ fontSize: 12, fontWeight: 600, color: "#334155", marginBottom: 6 }}>
                ⚙️ 请选择所选设备的入库策略：
              </div>
              <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(240px, 1fr))", gap: 8 }}>
                {/* Upsert */}
                <label style={{
                  display: "flex",
                  alignItems: "flex-start",
                  gap: 8,
                  padding: "6px 10px",
                  borderRadius: 6,
                  border: strategy === "upsert" ? "1.5px solid #2563eb" : "1px solid #e2e8f0",
                  background: strategy === "upsert" ? "#eff6ff" : "#fff",
                  cursor: "pointer",
                  fontSize: 11
                }}>
                  <input 
                    type="radio" 
                    name="strategy" 
                    checked={strategy === "upsert"} 
                    onChange={() => setStrategy("upsert")}
                    style={{ marginTop: 2 }}
                  />
                  <div>
                    <strong style={{ color: "#1e40af", display: "block" }}>
                      ✨ 智能覆盖更新 (Upsert · 推荐)
                    </strong>
                    <span style={{ color: "#64748b", fontSize: 10 }}>
                      勾选的新设备直接入库；已有设备更新变更字段，杜绝产生额外重复条目
                    </span>
                  </div>
                </label>

                {/* Skip */}
                <label style={{
                  display: "flex",
                  alignItems: "flex-start",
                  gap: 8,
                  padding: "6px 10px",
                  borderRadius: 6,
                  border: strategy === "skip" ? "1.5px solid #2563eb" : "1px solid #e2e8f0",
                  background: strategy === "skip" ? "#eff6ff" : "#fff",
                  cursor: "pointer",
                  fontSize: 11
                }}>
                  <input 
                    type="radio" 
                    name="strategy" 
                    checked={strategy === "skip"} 
                    onChange={() => setStrategy("skip")}
                    style={{ marginTop: 2 }}
                  />
                  <div>
                    <strong style={{ color: "#334155", display: "block" }}>
                      ⏭️ 仅导入新增设备 (跳过重复)
                    </strong>
                    <span style={{ color: "#64748b", fontSize: 10 }}>
                      仅将勾选的新设备入库，若勾选了已有设备则保持现有资产不变
                    </span>
                  </div>
                </label>

                {/* Replace */}
                <label style={{
                  display: "flex",
                  alignItems: "flex-start",
                  gap: 8,
                  padding: "6px 10px",
                  borderRadius: 6,
                  border: strategy === "replace" ? "1.5px solid #dc2626" : "1px solid #e2e8f0",
                  background: strategy === "replace" ? "#fef2f2" : "#fff",
                  cursor: "pointer",
                  fontSize: 11
                }}>
                  <input 
                    type="radio" 
                    name="strategy" 
                    checked={strategy === "replace"} 
                    onChange={() => setStrategy("replace")}
                    style={{ marginTop: 2 }}
                  />
                  <div>
                    <strong style={{ color: "#991b1b", display: "block" }}>
                      ⚠️ 全量覆盖同步 (Replace)
                    </strong>
                    <span style={{ color: "#64748b", fontSize: 10 }}>
                      以当前选定设备为基准，完全替换目标范围历史资产清单
                    </span>
                  </div>
                </label>
              </div>
            </div>
          )}

          {/* Parsing Results Preview & Manual Selection Table */}
          {parsedList.length > 0 && (
            <div style={{ flex: 1, display: "flex", flexDirection: "column", minHeight: 0, gap: 8 }}>
              {/* Controls Bar: Filter tabs & Search box */}
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: 8 }}>
                {/* Tabs */}
                <div style={{ display: "flex", gap: 4, flexWrap: "wrap" }}>
                  <button
                    type="button"
                    onClick={() => setPreviewFilter("all")}
                    style={{
                      fontSize: 11,
                      padding: "4px 10px",
                      borderRadius: 4,
                      border: "none",
                      cursor: "pointer",
                      background: previewFilter === "all" ? "#2563eb" : "#f1f5f9",
                      color: previewFilter === "all" ? "#fff" : "#475569",
                      fontWeight: 600
                    }}
                  >
                    全部清单 ({analysisList.length})
                  </button>

                  {counts.duplicateCount > 0 && (
                    <button
                      type="button"
                      onClick={() => setPreviewFilter("duplicate")}
                      style={{
                        fontSize: 11,
                        padding: "4px 10px",
                        borderRadius: 4,
                        border: "none",
                        cursor: "pointer",
                        background: previewFilter === "duplicate" ? "#dc2626" : "#fee2e2",
                        color: previewFilter === "duplicate" ? "#fff" : "#991b1b",
                        fontWeight: 600
                      }}
                    >
                      ⚠️ 仅看重复 ({counts.duplicateCount})
                    </button>
                  )}

                  <button
                    type="button"
                    onClick={() => setPreviewFilter("pure_new")}
                    style={{
                      fontSize: 11,
                      padding: "4px 10px",
                      borderRadius: 4,
                      border: "none",
                      cursor: "pointer",
                      background: previewFilter === "pure_new" ? "#15803d" : "#dcfce7",
                      color: previewFilter === "pure_new" ? "#fff" : "#166534",
                      fontWeight: 600
                    }}
                  >
                    🟢 纯新设备 ({counts.pureNewCount})
                  </button>

                  {counts.dbUpdateCount > 0 && (
                    <button
                      type="button"
                      onClick={() => setPreviewFilter("db_update")}
                      style={{
                        fontSize: 11,
                        padding: "4px 10px",
                        borderRadius: 4,
                        border: "none",
                        cursor: "pointer",
                        background: previewFilter === "db_update" ? "#d97706" : "#fef3c7",
                        color: previewFilter === "db_update" ? "#fff" : "#92400e",
                        fontWeight: 600
                      }}
                    >
                      🟠 变更更新 ({counts.dbUpdateCount})
                    </button>
                  )}

                  <button
                    type="button"
                    onClick={() => setPreviewFilter("selected")}
                    style={{
                      fontSize: 11,
                      padding: "4px 10px",
                      borderRadius: 4,
                      border: "none",
                      cursor: "pointer",
                      background: previewFilter === "selected" ? "#4338ca" : "#e0e7ff",
                      color: previewFilter === "selected" ? "#fff" : "#3730a3",
                      fontWeight: 600
                    }}
                  >
                    ✓ 已勾选 ({selectedIndexes.size})
                  </button>
                </div>

                {/* Search Input */}
                <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                  <input
                    type="text"
                    placeholder="🔍 检索设备名、IP、项目..."
                    value={searchQuery}
                    onChange={e => setSearchQuery(e.target.value)}
                    style={{
                      padding: "4px 10px",
                      fontSize: 11,
                      borderRadius: 4,
                      border: "1px solid #cbd5e1",
                      outline: "none",
                      width: 180
                    }}
                  />
                  {searchQuery && (
                    <button
                      type="button"
                      onClick={() => setSearchQuery("")}
                      style={{ border: "none", background: "transparent", color: "#94a3b8", cursor: "pointer", fontSize: 12 }}
                    >
                      ✕
                    </button>
                  )}
                </div>
              </div>

              {/* Table Container */}
              <div style={{
                border: "1px solid #e2e8f0",
                borderRadius: 6,
                overflowX: "auto",
                overflowY: "auto",
                maxHeight: 330,
                position: "relative"
              }}>
                <table className="cmdb-data-table" style={{ fontSize: 11, width: "100%", whiteSpace: "nowrap", borderCollapse: "separate", borderSpacing: 0 }}>
                  <thead style={{ position: "sticky", top: 0, zIndex: 10, background: "#f1f5f9" }}>
                    <tr>
                      <th style={{ width: 44, textAlign: "center", padding: "6px 8px" }}>
                        <input
                          type="checkbox"
                          checked={isAllFilteredSelected}
                          onChange={toggleSelectFiltered}
                          title="全选/反选当前列表"
                          style={{ cursor: "pointer" }}
                        />
                      </th>
                      <th style={{ width: 110 }}>判定状态</th>
                      <th>序号</th>
                      <th>设备名称</th>
                      <th>所属项目</th>
                      <th>业务IP / 私有IP</th>
                      <th>重复判定与差异分析</th>
                      <th>规格 (CPU/内存/磁盘)</th>
                      <th>操作系统</th>
                      <th>信创</th>
                      <th>远程端口</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredAnalysis.length === 0 ? (
                      <tr>
                        <td colSpan={11} style={{ textAlign: "center", padding: "30px 0", color: "#94a3b8" }}>
                          未找到匹配的数据条目
                        </td>
                      </tr>
                    ) : (
                      filteredAnalysis.map((item) => {
                        const d = item.device;
                        const isSelected = selectedIndexes.has(item.index);
                        return (
                          <tr 
                            key={item.index}
                            onClick={() => toggleSelect(item.index)}
                            style={{
                              background: isSelected 
                                ? (item.isDuplicate ? "#fffbeb" : "#f0fdf4")
                                : (item.isDuplicate ? "#fff5f5" : "#ffffff"),
                              cursor: "pointer",
                              transition: "background 0.15s"
                            }}
                          >
                            <td 
                              style={{ textAlign: "center", padding: "6px 8px" }}
                              onClick={e => e.stopPropagation()}
                            >
                              <input
                                type="checkbox"
                                checked={isSelected}
                                onChange={() => toggleSelect(item.index)}
                                style={{ cursor: "pointer" }}
                              />
                            </td>
                            <td>
                              {item.status === "pure_new" && (
                                <span style={{
                                  padding: "2px 6px",
                                  borderRadius: 4,
                                  fontSize: 10,
                                  fontWeight: 700,
                                  background: "#dcfce7",
                                  color: "#15803d"
                                }}>
                                  🟢 纯新设备
                                </span>
                              )}
                              {item.status === "file_duplicate" && (
                                <span style={{
                                  padding: "2px 6px",
                                  borderRadius: 4,
                                  fontSize: 10,
                                  fontWeight: 700,
                                  background: "#fee2e2",
                                  color: "#b91c1c"
                                }}>
                                  ⚠️ 文件内重复
                                </span>
                              )}
                              {item.status === "db_update" && (
                                <span style={{
                                  padding: "2px 6px",
                                  borderRadius: 4,
                                  fontSize: 10,
                                  fontWeight: 700,
                                  background: "#fef3c7",
                                  color: "#b45309"
                                }}>
                                  🟠 覆盖更新
                                </span>
                              )}
                              {item.status === "db_identical" && (
                                <span style={{
                                  padding: "2px 6px",
                                  borderRadius: 4,
                                  fontSize: 10,
                                  fontWeight: 600,
                                  background: "#fee2e2",
                                  color: "#b91c1c"
                                }}>
                                  ⚠️ 库中已有
                                </span>
                              )}
                            </td>
                            <td>{d.seq || (item.index + 1)}</td>
                            <td style={{ fontWeight: 600, color: "#1e293b" }}>{d.name}</td>
                            <td>{d.projectName}</td>
                            <td style={{ fontFamily: "monospace", color: "#2563eb", fontWeight: 600 }}>
                              {d.privateIp || d.ip || "-"}
                            </td>
                            <td>
                              {item.status === "file_duplicate" && (
                                <span style={{ color: "#dc2626", fontSize: 11, fontWeight: 500 }}>
                                  {item.duplicateReason}
                                </span>
                              )}
                              {item.status === "db_update" && item.diffResult?.diffs && (
                                <div>
                                  <div style={{ color: "#92400e", fontSize: 10, fontWeight: 500, marginBottom: 2 }}>
                                    {item.duplicateReason}
                                  </div>
                                  <div style={{ display: "flex", gap: 4, flexWrap: "wrap" }}>
                                    {item.diffResult.diffs.map((df, di) => (
                                      <span key={di} style={{ color: "#b45309", fontSize: 10, background: "#fffbeb", border: "1px solid #fde68a", padding: "1px 5px", borderRadius: 3 }}>
                                        {df.label}: {String(df.oldVal)} ➔ {String(df.newVal)}
                                      </span>
                                    ))}
                                  </div>
                                </div>
                              )}
                              {item.status === "db_identical" && (
                                <span style={{ color: "#b91c1c", fontSize: 11 }}>
                                  {item.duplicateReason}
                                </span>
                              )}
                              {item.status === "pure_new" && (
                                <span style={{ color: "#15803d", fontSize: 11 }}>
                                  {item.duplicateReason || "新发现设备，无历史重复记录"}
                                </span>
                              )}
                            </td>
                            <td>{d.cpu} · {d.memory}</td>
                            <td>{d.os}</td>
                            <td>
                              <span style={{
                                padding: "1px 6px",
                                borderRadius: 4,
                                fontSize: 10,
                                fontWeight: 600,
                                background: d.isXinchuang === "是" ? "#fee2e2" : "#f1f5f9",
                                color: d.isXinchuang === "是" ? "#dc2626" : "#64748b"
                              }}>
                                {d.isXinchuang === "是" ? "信创" : "非信创"}
                              </span>
                            </td>
                            <td style={{ fontFamily: "monospace" }}>{d.remotePort || 22}</td>
                          </tr>
                        );
                      })
                    )}
                  </tbody>
                </table>
              </div>

              {/* Table status tip */}
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", fontSize: 11, color: "#64748b", padding: "0 2px" }}>
                <span>
                  当前展示 {filteredAnalysis.length} 条记录 / 共解析 {analysisList.length} 条（支持点击整行进行勾选切换）
                </span>
                <span>
                  已手工勾选 <strong style={{ color: "#2563eb" }}>{selectedIndexes.size}</strong> 台
                  {selectedDuplicateCount > 0 && (
                    <span style={{ color: "#d97706", marginLeft: 4 }}>
                      （含手工选入的重复记录 {selectedDuplicateCount} 台）
                    </span>
                  )}
                </span>
              </div>
            </div>
          )}
        </div>

        {/* Footer */}
        <div style={{
          padding: "12px 20px",
          borderTop: "1px solid #e2e8f0",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          background: "#f8fafc"
        }}>
          <div style={{ fontSize: 12, color: "#64748b" }}>
            {parsedList.length > 0 && (
              <span>
                入库策略: <strong style={{ color: strategy === "replace" ? "#dc2626" : "#2563eb" }}>
                  {strategy === "upsert" ? "智能覆盖更新 (杜绝产生冗余重复)" : strategy === "skip" ? "仅导入新增 (跳过重复)" : "全量覆盖同步"}
                </strong>
              </span>
            )}
          </div>
          <div style={{ display: "flex", gap: 10 }}>
            <button type="button" className="btn-secondary" onClick={onClose}>
              取消
            </button>
            <button 
              type="button" 
              className="btn-primary" 
              disabled={parsedList.length === 0 || selectedIndexes.size === 0 || loading}
              onClick={handleConfirm}
              style={{
                opacity: (parsedList.length === 0 || selectedIndexes.size === 0) ? 0.5 : 1,
                cursor: (parsedList.length === 0 || selectedIndexes.size === 0) ? "not-allowed" : "pointer",
                background: selectedIndexes.size === 0 ? "#94a3b8" : "#2563eb",
                display: "flex",
                alignItems: "center",
                gap: 6
              }}
            >
              <span>✓ 确认导入已选数据 ({selectedIndexes.size} / {parsedList.length} 台)</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
