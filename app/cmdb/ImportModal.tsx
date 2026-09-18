"use client";
import React, { useState, useRef } from "react";
import { VmHost } from "../cmdbData";
import { parseExcelToAssets } from "./excelExport";

interface ImportModalProps {
  targetProjectName?: string | null;
  onClose: () => void;
  onConfirmImport: (importedAssets: (VmHost & { isImported?: boolean })[]) => void;
}

export default function ImportModal({
  targetProjectName,
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
    onConfirmImport(parsedList);
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
        maxWidth: 920,
        maxHeight: "90vh",
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
              支持符合《信息资产台账-v340.xlsx》「02-硬件设备」sheet 格式的 30 列标准规范表格
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
        <div style={{ padding: 20, overflowY: "auto", flex: 1 }}>
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
              padding: "24px 20px",
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
            <div style={{ fontSize: 32, marginBottom: 8 }}>📊</div>
            <div style={{ fontWeight: 600, fontSize: 14, color: "#1e293b", marginBottom: 4 }}>
              {loading ? "正在解析 Excel 台账数据..." : fileName ? `已加载: ${fileName}` : "点击选择 或 将《信息资产台账》Excel 文件拖拽至此处"}
            </div>
            <div style={{ fontSize: 12, color: "#64748b" }}>
              自动识别「02-硬件设备」工作表，读取服务器、网络、存储资产全字段
            </div>
          </div>

          {/* Error message */}
          {errorMsg && (
            <div style={{ marginTop: 12, padding: "10px 14px", borderRadius: 6, background: "#fef2f2", border: "1px solid #fecaca", color: "#dc2626", fontSize: 12 }}>
              ⚠️ {errorMsg}
            </div>
          )}

          {/* Parsing Results Preview */}
          {parsedList.length > 0 && (
            <div style={{ marginTop: 18 }}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 10 }}>
                <span style={{ fontSize: 13, fontWeight: 700, color: "#0f172a" }}>
                  📋 解析成功：识别到 <strong style={{ color: "#2563eb" }}>{parsedList.length}</strong> 台设备（工作表: {sheetName}）
                </span>
                <span style={{ fontSize: 11, color: "#64748b" }}>
                  预览前 10 条记录
                </span>
              </div>

              <div style={{ border: "1px solid #e2e8f0", borderRadius: 6, overflowX: "auto", maxHeight: 280 }}>
                <table className="cmdb-data-table" style={{ fontSize: 11, width: "100%", whiteSpace: "nowrap" }}>
                  <thead>
                    <tr>
                      <th>序号</th>
                      <th>设备名称</th>
                      <th>所属项目</th>
                      <th>客户名称</th>
                      <th>形态</th>
                      <th>业务IP / 内大网</th>
                      <th>规格 (CPU/内存/磁盘)</th>
                      <th>操作系统</th>
                      <th>信创</th>
                      <th>远程端口</th>
                    </tr>
                  </thead>
                  <tbody>
                    {parsedList.slice(0, 10).map((d, i) => (
                      <tr key={d.id || i}>
                        <td>{d.seq || (i + 1)}</td>
                        <td style={{ fontWeight: 600, color: "#1e293b" }}>{d.name}</td>
                        <td>{d.projectName}</td>
                        <td style={{ color: "#64748b" }}>{d.customerName}</td>
                        <td><span className="room-badge">{d.deviceType || d.category}</span></td>
                        <td style={{ fontFamily: "monospace", color: "#2563eb" }}>{d.privateIp || d.ip || "-"}</td>
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
                    ))}
                  </tbody>
                </table>
              </div>
              {parsedList.length > 10 && (
                <div style={{ textAlign: "center", fontSize: 11, color: "#64748b", marginTop: 6 }}>
                  ... 还有 {parsedList.length - 10} 台设备未在预览中展开，导入后可在资产列表中统一查看
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
          justifyContent: "flex-end",
          gap: 10,
          background: "#f8fafc"
        }}>
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
            ✓ 确认导入入库 ({parsedList.length} 台设备)
          </button>
        </div>
      </div>
    </div>
  );
}
