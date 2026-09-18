"use client";
import React, { useState, useRef, useMemo } from "react";
import { VmHost } from "../cmdbData";
import { parseExcelToAssets, getAssetKey, diffAssets, AssetDiffResult } from "./excelExport";

export type ImportStrategy = "upsert" | "skip" | "replace";

interface ImportItemAnalysis {
  device: VmHost & { isImported?: boolean };
  status: "new" | "update" | "unchanged";
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
  
  // Filter tab for preview: all / new / update / unchanged
  const [previewFilter, setPreviewFilter] = useState<"all" | "new" | "update" | "unchanged">("all");

  // Build existing map for quick diffing
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

  // Analyze parsed items vs existing
  const analysisList: ImportItemAnalysis[] = useMemo(() => {
    if (parsedList.length === 0) return [];

    return parsedList.map(item => {
      const key = getAssetKey(item);
      const existing = existingMap.get(key);

      if (!existing) {
        return {
          device: item,
          status: "new"
        };
      }

      const diff = diffAssets(existing, item);
      if (diff.hasChanged) {
        return {
          device: item,
          status: "update",
          existing,
          diffResult: diff
        };
      }

      return {
        device: item,
        status: "unchanged",
        existing,
        diffResult: diff
      };
    });
  }, [parsedList, existingMap]);

  // Statistics
  const counts = useMemo(() => {
    let countNew = 0;
    let countUpdate = 0;
    let countUnchanged = 0;

    for (const a of analysisList) {
      if (a.status === "new") countNew++;
      else if (a.status === "update") countUpdate++;
      else countUnchanged++;
    }

    return { countNew, countUpdate, countUnchanged };
  }, [analysisList]);

  // Filtered preview list
  const filteredAnalysis = useMemo(() => {
    if (previewFilter === "all") return analysisList;
    return analysisList.filter(a => a.status === previewFilter);
  }, [analysisList, previewFilter]);

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
      } else {
        // If importing inside a specific project, optionally align project name
        let finalDevices = res.devices;
        if (targetProjectName && targetProjectName !== "全部项目总览") {
          finalDevices = res.devices.map(d => ({
            ...d,
            projectName: targetProjectName
          }));
        }
        setParsedList(finalDevices);
        setSheetName(res.sheetName);
      }
    } catch (err: any) {
      setErrorMsg(err.message || "解析 Excel 文件失败，请核实文件格式。");
      setParsedList([]);
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

  function handleConfirm() {
    if (parsedList.length === 0) return;
    onConfirmImport(parsedList, strategy);
    onClose();
  }

  return (
    <div className="modal-backdrop" style={{
      position: "fixed",
      top: 0,
      left: 0,
      right: 0,
      bottom: 0,
      backgroundColor: "rgba(15, 23, 42, 0.65)",
      backdropFilter: "blur(4px)",
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
        maxWidth: 960,
        maxHeight: "92vh",
        display: "flex",
        flexDirection: "column",
        boxShadow: "0 25px 50px -12px rgba(0, 0, 0, 0.25)",
        overflow: "hidden",
        border: "1px solid #cbd5e1"
      }}>
        {/* Header */}
        <div style={{
          padding: "16px 20px",
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
            <p style={{ margin: "4px 0 0", fontSize: 12, color: "#64748b" }}>
              内置重复检测与差异合并引擎 · 兼容 30 列标准规范与纯净表头格式
            </p>
          </div>
          <button 
            type="button" 
            onClick={onClose}
            style={{ border: "none", background: "transparent", fontSize: 20, cursor: "pointer", color: "#64748b" }}
          >
            ✕
          </button>
        </div>

        {/* Content Body */}
        <div style={{ padding: "18px 20px", overflowY: "auto", flex: 1, display: "flex", flexDirection: "column", gap: 14 }}>
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
              padding: "20px",
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
            <div style={{ fontSize: 28, marginBottom: 6 }}>📊</div>
            <div style={{ fontWeight: 600, fontSize: 14, color: "#1e293b", marginBottom: 2 }}>
              {loading ? "正在解析 Excel 台账并执行去重比对..." : fileName ? `已解析文件: ${fileName}` : "点击选择 或 将《信息资产台账》Excel 文件拖拽至此处"}
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

          {/* Analysis & Strategy Section */}
          {parsedList.length > 0 && (
            <div style={{ background: "#f8fafc", border: "1px solid #e2e8f0", borderRadius: 8, padding: "14px 16px" }}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: 10, marginBottom: 12 }}>
                <span style={{ fontSize: 13, fontWeight: 700, color: "#0f172a" }}>
                  🔍 导入数据预检与差异分析结果 (共识别 {parsedList.length} 台设备)
                </span>
                {/* Badges */}
                <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                  <span style={{
                    fontSize: 11,
                    fontWeight: 600,
                    padding: "3px 10px",
                    borderRadius: 12,
                    background: "#dcfce7",
                    color: "#15803d",
                    border: "1px solid #bbf7d0"
                  }}>
                    🟢 纯新设备: {counts.countNew} 台
                  </span>
                  <span style={{
                    fontSize: 11,
                    fontWeight: 600,
                    padding: "3px 10px",
                    borderRadius: 12,
                    background: "#fef3c7",
                    color: "#b45309",
                    border: "1px solid #fde68a"
                  }}>
                    🟠 变更覆盖: {counts.countUpdate} 台
                  </span>
                  <span style={{
                    fontSize: 11,
                    fontWeight: 600,
                    padding: "3px 10px",
                    borderRadius: 12,
                    background: "#f1f5f9",
                    color: "#475569",
                    border: "1px solid #e2e8f0"
                  }}>
                    ⚪ 配置一致: {counts.countUnchanged} 台
                  </span>
                </div>
              </div>

              {/* Strategy Selector */}
              <div style={{ borderTop: "1px solid #e2e8f0", paddingTop: 12 }}>
                <div style={{ fontSize: 12, fontWeight: 600, color: "#334155", marginBottom: 8 }}>
                  ⚙️ 请选择重复与变更处理策略：
                </div>
                <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(260px, 1fr))", gap: 8 }}>
                  {/* Upsert */}
                  <label style={{
                    display: "flex",
                    alignItems: "flex-start",
                    gap: 8,
                    padding: "8px 12px",
                    borderRadius: 6,
                    border: strategy === "upsert" ? "1.5px solid #2563eb" : "1px solid #e2e8f0",
                    background: strategy === "upsert" ? "#eff6ff" : "#fff",
                    cursor: "pointer",
                    fontSize: 12
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
                      <span style={{ color: "#64748b", fontSize: 11 }}>
                        新设备直接入库；已有设备刷新配置（CPU/内存/OS等），<strong>杜绝产生重复记录</strong>
                      </span>
                    </div>
                  </label>

                  {/* Skip */}
                  <label style={{
                    display: "flex",
                    alignItems: "flex-start",
                    gap: 8,
                    padding: "8px 12px",
                    borderRadius: 6,
                    border: strategy === "skip" ? "1.5px solid #2563eb" : "1px solid #e2e8f0",
                    background: strategy === "skip" ? "#eff6ff" : "#fff",
                    cursor: "pointer",
                    fontSize: 12
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
                      <span style={{ color: "#64748b", fontSize: 11 }}>
                        仅新增入库 {counts.countNew} 台新节点，系统中已有设备完全保持原样不改动
                      </span>
                    </div>
                  </label>

                  {/* Replace */}
                  <label style={{
                    display: "flex",
                    alignItems: "flex-start",
                    gap: 8,
                    padding: "8px 12px",
                    borderRadius: 6,
                    border: strategy === "replace" ? "1.5px solid #dc2626" : "1px solid #e2e8f0",
                    background: strategy === "replace" ? "#fef2f2" : "#fff",
                    cursor: "pointer",
                    fontSize: 12
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
                      <span style={{ color: "#64748b", fontSize: 11 }}>
                        以当前文件为基准，完全替换目标范围历史资产清单
                      </span>
                    </div>
                  </label>
                </div>
              </div>
            </div>
          )}

          {/* Parsing Results Preview Table */}
          {parsedList.length > 0 && (
            <div style={{ flex: 1, display: "flex", flexDirection: "column", minHeight: 0 }}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
                {/* Tabs */}
                <div style={{ display: "flex", gap: 4 }}>
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
                    全部预览 ({analysisList.length})
                  </button>
                  <button
                    type="button"
                    onClick={() => setPreviewFilter("new")}
                    style={{
                      fontSize: 11,
                      padding: "4px 10px",
                      borderRadius: 4,
                      border: "none",
                      cursor: "pointer",
                      background: previewFilter === "new" ? "#15803d" : "#f1f5f9",
                      color: previewFilter === "new" ? "#fff" : "#475569",
                      fontWeight: 600
                    }}
                  >
                    新增 ({counts.countNew})
                  </button>
                  <button
                    type="button"
                    onClick={() => setPreviewFilter("update")}
                    style={{
                      fontSize: 11,
                      padding: "4px 10px",
                      borderRadius: 4,
                      border: "none",
                      cursor: "pointer",
                      background: previewFilter === "update" ? "#d97706" : "#f1f5f9",
                      color: previewFilter === "update" ? "#fff" : "#475569",
                      fontWeight: 600
                    }}
                  >
                    变更更新 ({counts.countUpdate})
                  </button>
                  <button
                    type="button"
                    onClick={() => setPreviewFilter("unchanged")}
                    style={{
                      fontSize: 11,
                      padding: "4px 10px",
                      borderRadius: 4,
                      border: "none",
                      cursor: "pointer",
                      background: previewFilter === "unchanged" ? "#64748b" : "#f1f5f9",
                      color: previewFilter === "unchanged" ? "#fff" : "#475569",
                      fontWeight: 600
                    }}
                  >
                    保持一致 ({counts.countUnchanged})
                  </button>
                </div>

                <span style={{ fontSize: 11, color: "#64748b" }}>
                  预览前 12 条记录
                </span>
              </div>

              <div style={{ border: "1px solid #e2e8f0", borderRadius: 6, overflowX: "auto", maxHeight: 250 }}>
                <table className="cmdb-data-table" style={{ fontSize: 11, width: "100%", whiteSpace: "nowrap" }}>
                  <thead>
                    <tr>
                      <th>判定状态</th>
                      <th>序号</th>
                      <th>设备名称</th>
                      <th>所属项目</th>
                      <th>业务IP / 私有IP</th>
                      <th>变更差异分析</th>
                      <th>规格 (CPU/内存/磁盘)</th>
                      <th>操作系统</th>
                      <th>信创</th>
                      <th>远程端口</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredAnalysis.slice(0, 12).map((item, i) => {
                      const d = item.device;
                      return (
                        <tr key={d.id || i}>
                          <td>
                            {item.status === "new" && (
                              <span style={{
                                padding: "2px 6px",
                                borderRadius: 4,
                                fontSize: 10,
                                fontWeight: 700,
                                background: "#dcfce7",
                                color: "#15803d"
                              }}>
                                🟢 新增入库
                              </span>
                            )}
                            {item.status === "update" && (
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
                            {item.status === "unchanged" && (
                              <span style={{
                                padding: "2px 6px",
                                borderRadius: 4,
                                fontSize: 10,
                                fontWeight: 600,
                                background: "#f1f5f9",
                                color: "#64748b"
                              }}>
                                ⚪ 配置一致
                              </span>
                            )}
                          </td>
                          <td>{d.seq || (i + 1)}</td>
                          <td style={{ fontWeight: 600, color: "#1e293b" }}>{d.name}</td>
                          <td>{d.projectName}</td>
                          <td style={{ fontFamily: "monospace", color: "#2563eb", fontWeight: 600 }}>
                            {d.privateIp || d.ip || "-"}
                          </td>
                          <td>
                            {item.status === "update" && item.diffResult?.diffs ? (
                              <span style={{ color: "#b45309", fontSize: 10, background: "#fffbeb", padding: "1px 6px", borderRadius: 3 }}>
                                {item.diffResult.diffs.map(df => `${df.label}: ${df.oldVal} ➔ ${df.newVal}`).join(" | ")}
                              </span>
                            ) : item.status === "new" ? (
                              <span style={{ color: "#15803d", fontSize: 10 }}>新发现设备</span>
                            ) : (
                              <span style={{ color: "#94a3b8", fontSize: 10 }}>无配置差异</span>
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
                    })}
                  </tbody>
                </table>
              </div>
              {filteredAnalysis.length > 12 && (
                <div style={{ textAlign: "center", fontSize: 11, color: "#64748b", marginTop: 4 }}>
                  ... 还有 {filteredAnalysis.length - 12} 条记录未在预览中展开，确认后将按选定策略执行入库
                </div>
              )}
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
                当前策略: <strong style={{ color: strategy === "replace" ? "#dc2626" : "#2563eb" }}>
                  {strategy === "upsert" ? "智能覆盖更新 (防重复)" : strategy === "skip" ? "仅导入新增 (跳过重复)" : "全量覆盖同步"}
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
              disabled={parsedList.length === 0 || loading}
              onClick={handleConfirm}
              style={{
                opacity: parsedList.length === 0 ? 0.5 : 1,
                cursor: parsedList.length === 0 ? "not-allowed" : "pointer"
              }}
            >
              ✓ 确认执行导入 ({strategy === "skip" ? counts.countNew : parsedList.length} 台)
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
