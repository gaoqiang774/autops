"use client";
import * as XLSX from "xlsx";
import { AssetMeta, PhysicalHost, VmHost, SwitchDevice } from "../cmdbData";

// 30 Column headers matching 《信息资产台账-v340.xlsx》「02-硬件设备」sheet
export const HW_EXCEL_HEADERS = [
  "序号",
  "项目编号",
  "客户名称",
  "项目名称",
  "环境",
  "云厂商",
  "区域名称",
  "设备名称",
  "设备大类",
  "设备类型",
  "机房",
  "私有IP（业务IP）",
  "私有IPV6",
  "内大网IP",
  "EIP地址",
  "VIP地址",
  "公网IP",
  "CPU架构",
  "CPU核数",
  "内存GB",
  "系统盘GB",
  "数据盘GB",
  "共享磁盘GB",
  "对象存储GB",
  "OS发行版",
  "OS版本",
  "内核版本",
  "是否信创OS",
  "远程端口",
  "备注"
];

// Helper to convert Asset into a 30-element row
export function assetToExcelRow(asset: AssetMeta | PhysicalHost | VmHost | SwitchDevice, index: number): (string | number | null)[] {
  const privateIp = asset.privateIp || (asset as any).ip || null;
  const cpuCores = asset.cpuCores ?? null;
  const memoryGb = asset.memoryGb ?? null;
  const systemDiskGb = asset.systemDiskGb ?? null;
  const dataDiskGb = asset.dataDiskGb ?? null;
  const sharedDiskGb = asset.sharedDiskGb ?? null;
  const objectStorageGb = asset.objectStorageGb ?? null;
  const remotePort = asset.remotePort ?? null;

  return [
    asset.seq ?? (index + 1),                              // 序号
    (asset as any).projectCode || null,                    // 项目编号
    asset.customerName || "北控伟仕保障客户",               // 客户名称
    asset.projectName || "未分类项目",                     // 项目名称
    asset.env || "生产",                                   // 环境
    asset.cloudVendor || "联通云",                         // 云厂商
    asset.regionName || "政务外网区",                      // 区域名称
    asset.name || (asset as any).hostname || `device-${index + 1}`, // 设备名称
    asset.category || "服务器",                            // 设备大类
    asset.deviceType || "虚拟机",                          // 设备类型
    (asset as any).roomName || "六里桥机房",               // 机房
    privateIp,                                             // 私有IP（业务IP）
    asset.privateIpv6 || null,                             // 私有IPV6
    asset.internalWanIp || null,                           // 内大网IP
    asset.eip || null,                                     // EIP地址
    asset.vip || null,                                     // VIP地址
    asset.publicIp || null,                                // 公网IP
    asset.cpuArch || "x86_64",                             // CPU架构
    cpuCores,                                              // CPU核数
    memoryGb,                                              // 内存GB
    systemDiskGb,                                          // 系统盘GB
    dataDiskGb,                                            // 数据盘GB
    sharedDiskGb,                                          // 共享磁盘GB
    objectStorageGb,                                       // 对象存储GB
    asset.osFamily || "CentOS",                            // OS发行版
    asset.osVersion || "CentOS 7.9",                       // OS版本
    asset.kernelVersion || null,                           // 内核版本
    asset.isXinchuang || "否",                             // 是否信创OS
    remotePort,                                            // 远程端口
    asset.remarks || null                                  // 备注
  ];
}

/**
 * 导出当前筛选/查询的资产为《信息资产台账-v340.xlsx》02硬件规范的 Excel 文件
 */
export function exportAssetsToExcel(
  assets: (AssetMeta | PhysicalHost | VmHost | SwitchDevice)[],
  scopeTitle: string = "全量项目"
) {
  // 表头行：只保留标准 30 项列名表头（无需前 3 行说明与分区栏）
  const headerRow = HW_EXCEL_HEADERS;

  // Data rows
  const dataRows = assets.map((item, idx) => assetToExcelRow(item, idx));

  const allRows = [headerRow, ...dataRows];

  // Build sheet
  const ws = XLSX.utils.aoa_to_sheet(allRows);

  // Set column widths for optimal display
  ws["!cols"] = [
    { wch: 6 },  // 序号
    { wch: 12 }, // 项目编号
    { wch: 28 }, // 客户名称
    { wch: 26 }, // 项目名称
    { wch: 8 },  // 环境
    { wch: 10 }, // 云厂商
    { wch: 14 }, // 区域名称
    { wch: 38 }, // 设备名称
    { wch: 10 }, // 设备大类
    { wch: 12 }, // 设备类型
    { wch: 16 }, // 机房
    { wch: 18 }, // 私有IP
    { wch: 18 }, // 私有IPV6
    { wch: 16 }, // 内大网IP
    { wch: 16 }, // EIP
    { wch: 20 }, // VIP
    { wch: 16 }, // 公网IP
    { wch: 10 }, // CPU架构
    { wch: 10 }, // CPU核数
    { wch: 10 }, // 内存GB
    { wch: 12 }, // 系统盘GB
    { wch: 12 }, // 数据盘GB
    { wch: 12 }, // 共享磁盘GB
    { wch: 12 }, // 对象存储GB
    { wch: 14 }, // OS发行版
    { wch: 18 }, // OS版本
    { wch: 30 }, // 内核版本
    { wch: 12 }, // 是否信创OS
    { wch: 10 }, // 远程端口
    { wch: 30 }  // 备注
  ];

  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, ws, "02-硬件设备");

  // Generate filename
  const dateStr = new Date().toISOString().slice(0, 10);
  const cleanTitle = scopeTitle.replace(/[\\/:\*\?"<>\|]/g, "_");
  const fileName = `信息资产台账-02硬件-${cleanTitle}-${dateStr}.xlsx`;

  XLSX.writeFile(wb, fileName);
}

/**
 * 解析用户上传的《信息资产台账》Excel 文件
 */
export async function parseExcelToAssets(file: File): Promise<{
  devices: (VmHost & { isImported?: boolean })[];
  sheetName: string;
  totalRows: number;
}> {
  const arrayBuffer = await file.arrayBuffer();
  const wb = XLSX.read(new Uint8Array(arrayBuffer), { type: "array" });

  // Find 02-硬件设备 or sheet with 硬件
  const sheetName = wb.SheetNames.find(s => s.includes("02") || s.includes("硬件")) || wb.SheetNames[0];
  const ws = wb.Sheets[sheetName];
  if (!ws) {
    throw new Error(`未在 Excel 中找到有效的工作表 (SheetNames: ${wb.SheetNames.join(", ")})`);
  }

  // Convert to 2D array
  const rawRows: any[][] = XLSX.utils.sheet_to_json(ws, { header: 1, defval: null });

  // Locate the header row containing "设备名称" or "私有IP" or "项目名称"
  let headerRowIndex = -1;
  for (let i = 0; i < Math.min(10, rawRows.length); i++) {
    const r = rawRows[i] || [];
    const text = r.map(x => String(x || "")).join(" ");
    if (text.includes("设备名称") || text.includes("私有IP") || text.includes("业务IP")) {
      headerRowIndex = i;
      break;
    }
  }

  if (headerRowIndex === -1) {
    throw new Error("未能识别到符合《02-硬件设备》规范的表头（需包含「设备名称」或「私有IP」列）");
  }

  const headers = (rawRows[headerRowIndex] || []).map(x => String(x || "").trim());

  function getColVal(row: any[], keyword: string) {
    const idx = headers.findIndex(h => h.includes(keyword));
    return idx !== -1 ? row[idx] : null;
  }

  const parsedDevices: (VmHost & { isImported?: boolean })[] = [];

  for (let rIdx = headerRowIndex + 1; rIdx < rawRows.length; rIdx++) {
    const r = rawRows[rIdx];
    if (!r || !r.some(x => x !== null && x !== "")) continue;

    const devName = getColVal(r, "设备名称");
    const ip = getColVal(r, "私有IP") || getColVal(r, "业务IP") || getColVal(r, "内大网IP");
    const projName = getColVal(r, "项目名称") || "导入项目";

    if (!devName && !ip) continue;

    const seqVal = getColVal(r, "序号");
    const seq = typeof seqVal === "number" ? seqVal : parseInt(seqVal, 10) || (rIdx + 1000);

    const cpuVal = getColVal(r, "CPU核数");
    const cpuCores = typeof cpuVal === "number" ? cpuVal : parseInt(cpuVal, 10) || 4;
    const cpuArch = getColVal(r, "CPU架构") || "x86_64";

    const memVal = getColVal(r, "内存GB");
    const memGb = typeof memVal === "number" ? memVal : parseInt(memVal, 10) || 16;

    const sysDiskVal = getColVal(r, "系统盘GB");
    const sysDisk = typeof sysDiskVal === "number" ? sysDiskVal : parseInt(sysDiskVal, 10) || 30;

    const dataDiskVal = getColVal(r, "数据盘GB");
    const dataDisk = typeof dataDiskVal === "number" ? dataDiskVal : parseInt(dataDiskVal, 10) || 0;

    const portVal = getColVal(r, "远程端口");
    const remotePort = typeof portVal === "number" ? portVal : parseInt(portVal, 10) || 22;

    const isXinchuang = String(getColVal(r, "信创") || "否");

    const dev: VmHost & { isImported?: boolean } = {
      id: `imported-${Date.now()}-${rIdx}`,
      seq: seq,
      name: String(devName || `导入设备-${seq}`),
      physicalHostId: "phy-14",
      ip: String(ip || "192.168.1.1"),
      privateIp: ip ? String(ip) : null,
      cpu: `${cpuCores} 核 (${cpuArch})`,
      memory: `${memGb} GB`,
      disk: dataDisk > 0 ? `${sysDisk}G(系统) + ${dataDisk}G(数据)` : `${sysDisk} GB`,
      os: `${getColVal(r, "OS发行版") || ""} ${getColVal(r, "OS版本") || ""}`.trim() || "CentOS 7.9",
      businessId: "prj-001",
      status: "running",
      role: "导入应用节点",
      updated: new Date().toISOString().slice(0, 10),
      isImported: true,

      // Excel Meta Fields
      customerName: getColVal(r, "客户名称") ? String(getColVal(r, "客户名称")) : "北控伟仕保障客户",
      projectName: String(projName),
      env: getColVal(r, "环境") ? String(getColVal(r, "环境")) : "生产",
      cloudVendor: getColVal(r, "云厂商") ? String(getColVal(r, "云厂商")) : "联通云",
      regionName: getColVal(r, "区域") ? String(getColVal(r, "区域")) : "政务外网区",
      category: getColVal(r, "设备大类") ? String(getColVal(r, "设备大类")) : "服务器",
      deviceType: getColVal(r, "设备类型") ? String(getColVal(r, "设备类型")) : "虚拟机",
      internalWanIp: getColVal(r, "内大网IP") ? String(getColVal(r, "内大网IP")) : null,
      eip: getColVal(r, "EIP") ? String(getColVal(r, "EIP")) : null,
      vip: getColVal(r, "VIP") ? String(getColVal(r, "VIP")) : null,
      publicIp: getColVal(r, "公网IP") ? String(getColVal(r, "公网IP")) : null,
      cpuArch: String(cpuArch),
      cpuCores: cpuCores,
      memoryGb: memGb,
      systemDiskGb: sysDisk,
      dataDiskGb: dataDisk,
      sharedDiskGb: getColVal(r, "共享磁盘") ? parseInt(getColVal(r, "共享磁盘"), 10) : null,
      objectStorageGb: getColVal(r, "对象存储") ? parseInt(getColVal(r, "对象存储"), 10) : null,
      osFamily: getColVal(r, "OS发行版") ? String(getColVal(r, "OS发行版")) : "CentOS",
      osVersion: getColVal(r, "OS版本") ? String(getColVal(r, "OS版本")) : "CentOS 7.9",
      kernelVersion: getColVal(r, "内核版本") ? String(getColVal(r, "内核版本")) : null,
      isXinchuang: isXinchuang.includes("是") ? "是" : "否",
      remotePort: remotePort,
      remarks: getColVal(r, "备注") ? String(getColVal(r, "备注")) : "Excel 导入设备"
    };

    parsedDevices.push(dev);
  }

  return {
    devices: parsedDevices,
    sheetName,
    totalRows: rawRows.length
  };
}
