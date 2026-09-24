"use client";
import * as XLSX from "xlsx";
import {
  AssetMeta,
  PhysicalHost,
  VmHost,
  SwitchDevice,
  DatabaseAsset,
  MiddlewareAsset,
  BackupAsset,
  OpsAsset
} from "../cmdbData";

// ========================================================================
// 1. Column Headers & Widths (100% aligned with 《信息资产台账-v351.xlsx》)
// ========================================================================

// 02-硬件设备 (30 columns)
export const HW_EXCEL_HEADERS = [
  "序号", "项目编号", "客户名称", "项目名称", "环境", "云厂商", "区域名称",
  "设备名称", "设备大类", "设备类型", "机房", "私有IP（业务IP）", "私有IPV6",
  "内大网IP", "EIP地址", "VIP地址", "公网IP", "CPU架构", "CPU核数", "内存GB",
  "系统盘GB", "数据盘GB", "共享磁盘GB", "对象存储GB", "OS发行版", "OS版本",
  "内核版本", "是否信创OS", "远程端口", "备注"
];

export const HW_COL_WIDTHS = [
  { wch: 6 },  { wch: 12 }, { wch: 28 }, { wch: 26 }, { wch: 8 },
  { wch: 10 }, { wch: 14 }, { wch: 38 }, { wch: 10 }, { wch: 12 },
  { wch: 16 }, { wch: 18 }, { wch: 18 }, { wch: 16 }, { wch: 16 },
  { wch: 20 }, { wch: 16 }, { wch: 10 }, { wch: 10 }, { wch: 10 },
  { wch: 12 }, { wch: 12 }, { wch: 12 }, { wch: 12 }, { wch: 14 },
  { wch: 18 }, { wch: 30 }, { wch: 12 }, { wch: 10 }, { wch: 30 }
];

// 03-数据库 (18 columns)
export const DB_EXCEL_HEADERS = [
  "序号", "项目编号", "客户名称", "项目名称", "环境", "云厂商", "区域名称",
  "私有IP（业务IP）", "数据库vip/EIP地址", "数据库大类", "数据库软件名称",
  "数据库版本", "数据库监听端口", "数据库实例/SID", "业务库名",
  "数据库部署模式", "集群名", "备注"
];

export const DB_COL_WIDTHS = [
  { wch: 6 },  { wch: 12 }, { wch: 28 }, { wch: 26 }, { wch: 8 },
  { wch: 10 }, { wch: 14 }, { wch: 18 }, { wch: 20 }, { wch: 26 },
  { wch: 22 }, { wch: 14 }, { wch: 14 }, { wch: 18 }, { wch: 16 },
  { wch: 14 }, { wch: 16 }, { wch: 30 }
];

// 04-中间件 (14 columns - 对齐v360标准规范)
export const MW_EXCEL_HEADERS = [
  "序号", "项目编号", "客户名称", "项目名称", "环境", "云厂商", "区域名称",
  "私有IP（业务IP）", "中间件类型", "中间件软件名称", "中间件版本",
  "服务端口", "应用程序运行环境", "备注"
];

export const MW_COL_WIDTHS = [
  { wch: 6 },  { wch: 12 }, { wch: 28 }, { wch: 26 }, { wch: 8 },
  { wch: 10 }, { wch: 14 }, { wch: 18 }, { wch: 26 }, { wch: 22 },
  { wch: 14 }, { wch: 20 }, { wch: 18 }, { wch: 30 }
];

// 05-备份 (13 columns)
export const BK_EXCEL_HEADERS = [
  "序号", "项目编号", "客户名称", "项目名称", "环境", "云厂商", "区域名称",
  "私有IP（业务IP）", "备份类型", "备份方式", "备份策略", "备份存储位置", "备注"
];

export const BK_COL_WIDTHS = [
  { wch: 6 },  { wch: 12 }, { wch: 28 }, { wch: 26 }, { wch: 8 },
  { wch: 10 }, { wch: 14 }, { wch: 18 }, { wch: 16 }, { wch: 16 },
  { wch: 18 }, { wch: 32 }, { wch: 30 }
];

// 06-运维 (18 columns - 包含VPN/堡垒机使用者名字)
export const OPS_EXCEL_HEADERS = [
  "序号", "项目编号", "客户名称", "项目名称", "环境", "云厂商", "人员归属", "使用人网络环境",
  "私有IP（业务IP）", "运维厂商", "VPN地址", "VPN账号", "VPN使用人", "堡垒机地址",
  "堡垒机账号", "堡垒机使用人", "访问服务器地址", "备注"
];

export const OPS_COL_WIDTHS = [
  { wch: 6 },  { wch: 12 }, { wch: 28 }, { wch: 26 }, { wch: 8 },
  { wch: 10 }, { wch: 12 }, { wch: 16 }, { wch: 18 }, { wch: 26 },
  { wch: 22 }, { wch: 20 }, { wch: 14 }, { wch: 26 }, { wch: 20 },
  { wch: 14 }, { wch: 30 }, { wch: 30 }
];

// ========================================================================
// 2. Data to Excel Row Converters
// ========================================================================

export function assetToExcelRow(
  asset: AssetMeta | PhysicalHost | VmHost | SwitchDevice,
  index: number
): (string | number | null)[] {
  const privateIp = asset.privateIp || (asset as any).ip || null;
  const cpuCores = asset.cpuCores ?? null;
  const memoryGb = asset.memoryGb ?? null;
  const systemDiskGb = asset.systemDiskGb ?? null;
  const dataDiskGb = asset.dataDiskGb ?? null;
  const sharedDiskGb = asset.sharedDiskGb ?? null;
  const objectStorageGb = asset.objectStorageGb ?? null;
  const remotePort = asset.remotePort ?? null;

  return [
    asset.seq ?? (index + 1),
    (asset as any).projectNo || null,
    asset.customerName || "北京市人力资源和社会保障局",
    asset.projectName || "未分类项目",
    asset.env || "生产",
    asset.cloudVendor || "联通云",
    asset.regionName || "政务外网区",
    (asset as any).name || (asset as any).hostname || `device-${index + 1}`,
    asset.category || "服务器",
    asset.deviceType || "虚拟机",
    (asset as any).roomName || "六里桥机房",
    privateIp,
    asset.privateIpv6 || null,
    asset.internalWanIp || null,
    asset.eip || null,
    asset.vip || null,
    asset.publicIp || null,
    asset.cpuArch || "x86_64",
    cpuCores,
    memoryGb,
    systemDiskGb,
    dataDiskGb,
    sharedDiskGb,
    objectStorageGb,
    asset.osFamily || "CentOS",
    asset.osVersion || "CentOS 7.9",
    asset.kernelVersion || null,
    asset.isXinchuang || "否",
    remotePort,
    asset.remarks || null
  ];
}

export function databaseToExcelRow(db: DatabaseAsset, index: number): (string | number | null)[] {
  return [
    db.seq ?? (index + 1),
    db.projectNo || null,
    db.customerName || "北京市人力资源和社会保障局",
    db.projectName || "",
    db.env || "生产",
    db.cloudVendor || "联通云",
    db.regionName || "政务外网区",
    db.privateIp || db.hostIp || null,
    db.vipEip || null,
    db.dbCategory || "关系型 (RDBMS / OLTP)",
    db.dbSoftware || db.type || "Oracle Database",
    db.version || null,
    db.port ?? (db.dbSoftware?.includes("MySQL") ? 3306 : 1521),
    db.instanceSid || null,
    db.dbName || null,
    db.deployMode || "单机",
    db.clusterName || null,
    db.remarks || null
  ];
}

export function middlewareToExcelRow(mw: MiddlewareAsset, index: number): (string | number | null)[] {
  return [
    mw.seq ?? (index + 1),
    mw.projectNo || null,
    mw.customerName || "北京市人力资源和社会保障局",
    mw.projectName || "",
    mw.env || "生产",
    mw.cloudVendor || "联通云",
    mw.regionName || "政务外网区",
    mw.privateIp || mw.assetIp || null,
    mw.mwType || "应用服务器/Java Web 容器",
    mw.mwSoftware || mw.name || null,
    mw.version || null,
    mw.port || null,
    mw.runtime || "JDK8",
    mw.remarks || null
  ];
}

export function backupToExcelRow(bk: BackupAsset, index: number): (string | number | null)[] {
  return [
    bk.seq ?? (index + 1),
    bk.projectNo || null,
    bk.customerName || "北京市人力资源和社会保障局",
    bk.projectName || "",
    bk.env || "生产",
    bk.cloudVendor || "联通云",
    bk.regionName || "政务外网区",
    bk.privateIp || null,
    bk.backupType || null,
    bk.backupMethod || null,
    bk.backupPolicy || null,
    bk.storageLocation || null,
    bk.remarks || null
  ];
}

export function opsToExcelRow(ops: OpsAsset, index: number): (string | number | null)[] {
  return [
    ops.seq ?? (index + 1),
    ops.projectNo || null,
    ops.customerName || "北京市人力资源和社会保障局",
    ops.projectName || "",
    ops.env || "生产",
    ops.cloudVendor || "联通云",
    ops.personnelAffiliation || "伟仕",
    ops.vpnNetworkEnv || "互联网区",
    ops.privateIp || null,
    ops.opsVendor || "北京北控伟仕软件有限公司",
    ops.vpnAddress || null,
    ops.vpnAccount || null,
    ops.vpnUserName || null,
    ops.bastionAddress || null,
    ops.bastionAccount || null,
    ops.bastionUserName || null,
    ops.serverAccessAddress || null,
    ops.remarks || null
  ];
}

// ========================================================================
// 3. Export Functions (Single Sheet & v351 Multi-Sheet Workbook)
// ========================================================================

/** 辅助生成带有 v351 规范表头的 Worksheet */
function buildV351Worksheet(sheetTitle: string, categoryRow: (string | null)[], headerCols: string[], dataRows: any[][], widths: { wch: number }[]): XLSX.WorkSheet {
  const row1 = [sheetTitle];
  const row2 = categoryRow;
  const row3 = headerCols;
  const allRows = [row1, row2, row3, ...dataRows];
  const ws = XLSX.utils.aoa_to_sheet(allRows);
  ws["!cols"] = widths;
  return ws;
}

export function exportAssetsToExcel(
  assets: (AssetMeta | PhysicalHost | VmHost | SwitchDevice)[],
  scopeTitle: string = "全量项目"
) {
  const dataRows = assets.map((item, idx) => assetToExcelRow(item, idx));
  const categoryRow: (string | null)[] = [
    "基本信息", null, null, null, null, null, null, null, null, null, null,
    "网络信息", null, null, null, null, null,
    "资源信息", null, null, null, null, null, null,
    "操作系统信息", null, null, null, null,
    "其他"
  ];
  const ws = buildV351Worksheet("02-硬件设备", categoryRow, HW_EXCEL_HEADERS, dataRows, HW_COL_WIDTHS);

  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, ws, "02-硬件设备");

  const dateStr = new Date().toISOString().slice(0, 10);
  const cleanTitle = scopeTitle.replace(/[/\\:*?"<>|]/g, "_");
  XLSX.writeFile(wb, `信息资产台账-02硬件-${cleanTitle}-${dateStr}.xlsx`);
}

export function exportDatabasesToExcel(databases: DatabaseAsset[], scopeTitle: string = "全量项目") {
  const dataRows = databases.map((d, i) => databaseToExcelRow(d, i));
  const categoryRow: (string | null)[] = [
    "基本信息", null, null, null, null, null, null, null, null,
    "数据库信息", null, null, null, null, null, null, null,
    "其他"
  ];
  const ws = buildV351Worksheet("03-数据库", categoryRow, DB_EXCEL_HEADERS, dataRows, DB_COL_WIDTHS);

  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, ws, "03-数据库");

  const dateStr = new Date().toISOString().slice(0, 10);
  const cleanTitle = scopeTitle.replace(/[/\\:*?"<>|]/g, "_");
  XLSX.writeFile(wb, `信息资产台账-03数据库-${cleanTitle}-${dateStr}.xlsx`);
}

export function exportMiddlewaresToExcel(middlewares: MiddlewareAsset[], scopeTitle: string = "全量项目") {
  const dataRows = middlewares.map((m, i) => middlewareToExcelRow(m, i));
  const categoryRow: (string | null)[] = [
    "基本信息", null, null, null, null, null, null, null,
    "中间件信息", null, null, null, null,
    "其他"
  ];
  const ws = buildV351Worksheet("04-中间件", categoryRow, MW_EXCEL_HEADERS, dataRows, MW_COL_WIDTHS);

  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, ws, "04-中间件");

  const dateStr = new Date().toISOString().slice(0, 10);
  const cleanTitle = scopeTitle.replace(/[/\\:*?"<>|]/g, "_");
  XLSX.writeFile(wb, `信息资产台账-04中间件-${cleanTitle}-${dateStr}.xlsx`);
}

export function exportBackupsToExcel(backups: BackupAsset[], scopeTitle: string = "全量项目") {
  const dataRows = backups.map((b, i) => backupToExcelRow(b, i));
  const categoryRow: (string | null)[] = [
    "基本信息", null, null, null, null, null, null, null,
    "备份信息", null, null, null,
    "其他"
  ];
  const ws = buildV351Worksheet("05-备份", categoryRow, BK_EXCEL_HEADERS, dataRows, BK_COL_WIDTHS);

  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, ws, "05-备份");

  const dateStr = new Date().toISOString().slice(0, 10);
  const cleanTitle = scopeTitle.replace(/[/\\:*?"<>|]/g, "_");
  XLSX.writeFile(wb, `信息资产台账-05备份-${cleanTitle}-${dateStr}.xlsx`);
}

export function exportOpsToExcel(opsList: OpsAsset[], scopeTitle: string = "全量项目") {
  const dataRows = opsList.map((o, i) => opsToExcelRow(o, i));
  const categoryRow: (string | null)[] = [
    "基本信息", null, null, null, null, null, null, null,
    "运维信息", null, null, null, null, null,
    "其他"
  ];
  const ws = buildV351Worksheet("06-运维", categoryRow, OPS_EXCEL_HEADERS, dataRows, OPS_COL_WIDTHS);

  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, ws, "06-运维");

  const dateStr = new Date().toISOString().slice(0, 10);
  const cleanTitle = scopeTitle.replace(/[/\\:*?"<>|]/g, "_");
  XLSX.writeFile(wb, `信息资产台账-06运维-${cleanTitle}-${dateStr}.xlsx`);
}

export function exportAllV351Workbook(
  assets: (AssetMeta | PhysicalHost | VmHost | SwitchDevice)[],
  databases: DatabaseAsset[],
  middlewares: MiddlewareAsset[],
  backups: BackupAsset[],
  opsList: OpsAsset[],
  scopeTitle: string = "全量项目"
) {
  const wb = XLSX.utils.book_new();

  // Sheet 02-硬件设备
  const cat02 = ["基本信息", null, null, null, null, null, null, null, null, null, null, "网络信息", null, null, null, null, null, "资源信息", null, null, null, null, null, null, "操作系统信息", null, null, null, null, "其他"];
  const ws02 = buildV351Worksheet("02-硬件设备", cat02, HW_EXCEL_HEADERS, assets.map((a, i) => assetToExcelRow(a, i)), HW_COL_WIDTHS);
  XLSX.utils.book_append_sheet(wb, ws02, "02-硬件设备");

  // Sheet 03-数据库
  const cat03 = ["基本信息", null, null, null, null, null, null, null, null, "数据库信息", null, null, null, null, null, null, null, "其他"];
  const ws03 = buildV351Worksheet("03-数据库", cat03, DB_EXCEL_HEADERS, databases.map((d, i) => databaseToExcelRow(d, i)), DB_COL_WIDTHS);
  XLSX.utils.book_append_sheet(wb, ws03, "03-数据库");

  // Sheet 04-中间件
  const cat04 = ["基本信息", null, null, null, null, null, null, null, "中间件信息", null, null, null, "其他"];
  const ws04 = buildV351Worksheet("04-中间件", cat04, MW_EXCEL_HEADERS, middlewares.map((m, i) => middlewareToExcelRow(m, i)), MW_COL_WIDTHS);
  XLSX.utils.book_append_sheet(wb, ws04, "04-中间件");

  // Sheet 05-备份
  const cat05 = ["基本信息", null, null, null, null, null, null, null, "备份信息", null, null, null, "其他"];
  const ws05 = buildV351Worksheet("05-备份", cat05, BK_EXCEL_HEADERS, backups.map((b, i) => backupToExcelRow(b, i)), BK_COL_WIDTHS);
  XLSX.utils.book_append_sheet(wb, ws05, "05-备份");

  // Sheet 06-运维
  const cat06 = ["基本信息", null, null, null, null, null, null, null, "运维信息", null, null, null, null, null, "其他"];
  const ws06 = buildV351Worksheet("06-运维", cat06, OPS_EXCEL_HEADERS, opsList.map((o, i) => opsToExcelRow(o, i)), OPS_COL_WIDTHS);
  XLSX.utils.book_append_sheet(wb, ws06, "06-运维");

  const dateStr = new Date().toISOString().slice(0, 10);
  const cleanTitle = scopeTitle.replace(/[/\\:*?"<>|]/g, "_");
  XLSX.writeFile(wb, `信息资产台账-v351全表-${cleanTitle}-${dateStr}.xlsx`);
}

// ========================================================================
// 4. Parser Helpers & Row Parsers for All 5 Dimensions
// ========================================================================

/** 查找包含关键字的表头行索引 (0-indexed) */
function findHeaderRowIndex(rawRows: any[][], keywords: string[]): number {
  for (let i = 0; i < Math.min(10, rawRows.length); i++) {
    const r = rawRows[i] || [];
    const text = r.map(x => String(x || "")).join(" ");
    if (keywords.some(kw => text.includes(kw))) {
      return i;
    }
  }
  return -1;
}

/** 智能获取某一列的值 */
function getCellVal(row: any[], headers: string[], keywords: string[]): any {
  for (const kw of keywords) {
    const idx = headers.findIndex(h => h.includes(kw));
    if (idx !== -1 && row[idx] !== undefined && row[idx] !== null && String(row[idx]).trim() !== "") {
      return row[idx];
    }
  }
  return null;
}

/** 02-硬件设备解析器 */
export function parseHardwareRows(rawRows: any[][]): (VmHost & { isImported?: boolean })[] {
  const headerIdx = findHeaderRowIndex(rawRows, ["设备名称", "私有IP", "业务IP", "CPU架构", "CPU核数"]);
  if (headerIdx === -1) return [];

  const headers = (rawRows[headerIdx] || []).map(x => String(x || "").trim());
  const list: (VmHost & { isImported?: boolean })[] = [];

  for (let rIdx = headerIdx + 1; rIdx < rawRows.length; rIdx++) {
    const r = rawRows[rIdx];
    if (!r || !r.some(x => x !== null && String(x || "").trim() !== "")) continue;

    const devName = getCellVal(r, headers, ["设备名称"]);
    const ip = getCellVal(r, headers, ["私有IP", "业务IP", "内大网IP"]);
    const projName = getCellVal(r, headers, ["项目名称"]) || "导入项目";

    if (!devName && !ip) continue;

    const seqVal = getCellVal(r, headers, ["序号"]);
    const seq = typeof seqVal === "number" ? seqVal : parseInt(seqVal, 10) || (rIdx + 1);

    const cpuVal = getCellVal(r, headers, ["CPU核数"]);
    const cpuCores = typeof cpuVal === "number" ? cpuVal : parseInt(cpuVal, 10) || 4;
    const cpuArch = getCellVal(r, headers, ["CPU架构"]) || "x86_64";

    const memVal = getCellVal(r, headers, ["内存GB", "内存"]);
    const memGb = typeof memVal === "number" ? memVal : parseInt(memVal, 10) || 16;

    const sysDiskVal = getCellVal(r, headers, ["系统盘GB", "系统盘"]);
    const sysDisk = typeof sysDiskVal === "number" ? sysDiskVal : parseInt(sysDiskVal, 10) || 30;

    const dataDiskVal = getCellVal(r, headers, ["数据盘GB", "数据盘"]);
    const dataDisk = typeof dataDiskVal === "number" ? dataDiskVal : parseInt(dataDiskVal, 10) || 0;

    const portVal = getCellVal(r, headers, ["远程端口"]);
    const remotePort = typeof portVal === "number" ? portVal : parseInt(portVal, 10) || 22;

    const isXinchuang = String(getCellVal(r, headers, ["是否信创", "信创"]) || "否");

    list.push({
      id: `imported-hw-${Date.now()}-${rIdx}`,
      seq: seq,
      name: String(devName || `导入设备-${seq}`),
      physicalHostId: "phy-1",
      ip: String(ip || "192.168.1.1"),
      privateIp: ip ? String(ip) : null,
      cpu: `${cpuCores} 核 (${cpuArch})`,
      memory: `${memGb} GB`,
      disk: dataDisk > 0 ? `${sysDisk}G(系统) + ${dataDisk}G(数据)` : `${sysDisk} GB`,
      os: `${getCellVal(r, headers, ["OS发行版"]) || ""} ${getCellVal(r, headers, ["OS版本"]) || ""}`.trim() || "CentOS 7.9",
      businessId: "prj-001",
      status: "running",
      role: "导入应用节点",
      updated: new Date().toISOString().slice(0, 10),
      isImported: true,

      customerName: String(getCellVal(r, headers, ["客户名称"]) || "北京市人力资源和社会保障局"),
      projectName: String(projName),
      env: String(getCellVal(r, headers, ["环境"]) || "生产"),
      cloudVendor: String(getCellVal(r, headers, ["云厂商"]) || "联通云"),
      regionName: String(getCellVal(r, headers, ["区域名称", "区域"]) || "政务外网区"),
      category: String(getCellVal(r, headers, ["设备大类"]) || "服务器"),
      deviceType: String(getCellVal(r, headers, ["设备类型"]) || "虚拟机"),
      internalWanIp: getCellVal(r, headers, ["内大网IP"]) ? String(getCellVal(r, headers, ["内大网IP"])) : null,
      eip: getCellVal(r, headers, ["EIP地址", "EIP"]) ? String(getCellVal(r, headers, ["EIP地址", "EIP"])) : null,
      vip: getCellVal(r, headers, ["VIP地址", "VIP"]) ? String(getCellVal(r, headers, ["VIP地址", "VIP"])) : null,
      publicIp: getCellVal(r, headers, ["公网IP"]) ? String(getCellVal(r, headers, ["公网IP"])) : null,
      cpuArch: String(cpuArch),
      cpuCores: cpuCores,
      memoryGb: memGb,
      systemDiskGb: sysDisk,
      dataDiskGb: dataDisk,
      sharedDiskGb: getCellVal(r, headers, ["共享磁盘"]) ? parseInt(getCellVal(r, headers, ["共享磁盘"]), 10) : null,
      objectStorageGb: getCellVal(r, headers, ["对象存储"]) ? parseInt(getCellVal(r, headers, ["对象存储"]), 10) : null,
      osFamily: String(getCellVal(r, headers, ["OS发行版"]) || "CentOS"),
      osVersion: String(getCellVal(r, headers, ["OS版本"]) || "CentOS 7.9"),
      kernelVersion: getCellVal(r, headers, ["内核版本"]) ? String(getCellVal(r, headers, ["内核版本"])) : null,
      isXinchuang: isXinchuang.includes("是") ? "是" : "否",
      remotePort: remotePort,
      remarks: getCellVal(r, headers, ["备注"]) ? String(getCellVal(r, headers, ["备注"])) : "Excel 导入设备"
    });
  }

  return list;
}

/** 03-数据库解析器 */
export function parseDatabaseRows(rawRows: any[][]): (DatabaseAsset & { isImported?: boolean })[] {
  const headerIdx = findHeaderRowIndex(rawRows, ["数据库软件名称", "数据库大类", "数据库监听端口", "数据库实例", "业务库名", "数据库vip"]);
  if (headerIdx === -1) return [];

  const headers = (rawRows[headerIdx] || []).map(x => String(x || "").trim());
  const list: (DatabaseAsset & { isImported?: boolean })[] = [];

  for (let rIdx = headerIdx + 1; rIdx < rawRows.length; rIdx++) {
    const r = rawRows[rIdx];
    if (!r || !r.some(x => x !== null && String(x || "").trim() !== "")) continue;

    const dbSoftware = getCellVal(r, headers, ["数据库软件名称", "数据库软件", "数据库类型"]);
    const privateIp = getCellVal(r, headers, ["私有IP", "业务IP", "主机IP"]);
    const projName = getCellVal(r, headers, ["项目名称"]);

    if (!dbSoftware && !privateIp && !projName) continue;

    const seqVal = getCellVal(r, headers, ["序号"]);
    const seq = typeof seqVal === "number" ? seqVal : parseInt(seqVal, 10) || (rIdx + 1);

    const portVal = getCellVal(r, headers, ["数据库监听端口", "监听端口", "端口"]);
    const port = typeof portVal === "number" ? portVal : parseInt(portVal, 10) || (String(dbSoftware || "").includes("MySQL") ? 3306 : 1521);

    const softwareName = String(dbSoftware || "Oracle Database");
    const ipStr = privateIp ? String(privateIp) : "";

    list.push({
      id: `imported-db-${Date.now()}-${rIdx}`,
      seq: seq,
      projectNo: getCellVal(r, headers, ["项目编号"]) ? String(getCellVal(r, headers, ["项目编号"])) : "",
      customerName: String(getCellVal(r, headers, ["客户名称"]) || "北京市人力资源和社会保障局"),
      projectName: String(projName || "导入数据库"),
      env: String(getCellVal(r, headers, ["环境"]) || "生产"),
      cloudVendor: String(getCellVal(r, headers, ["云厂商"]) || "联通云"),
      regionName: String(getCellVal(r, headers, ["区域名称", "区域"]) || "政务外网区"),
      privateIp: ipStr,
      hostIp: ipStr,
      vipEip: getCellVal(r, headers, ["数据库vip", "EIP", "VIP"]) ? String(getCellVal(r, headers, ["数据库vip", "EIP", "VIP"])) : "",
      dbCategory: String(getCellVal(r, headers, ["数据库大类"]) || "关系型 (RDBMS / OLTP)"),
      dbSoftware: softwareName,
      name: `${softwareName}_${port}`,
      type: softwareName,
      version: getCellVal(r, headers, ["数据库版本", "版本"]) ? String(getCellVal(r, headers, ["数据库版本", "版本"])) : "",
      port: port,
      instanceSid: getCellVal(r, headers, ["数据库实例", "SID", "实例"]) ? String(getCellVal(r, headers, ["数据库实例", "SID", "实例"])) : "",
      dbName: getCellVal(r, headers, ["业务库名", "库名"]) ? String(getCellVal(r, headers, ["业务库名", "库名"])) : "",
      deployMode: String(getCellVal(r, headers, ["数据库部署模式", "部署模式"]) || "单机"),
      clusterName: getCellVal(r, headers, ["集群名", "集群"]) ? String(getCellVal(r, headers, ["集群名", "集群"])) : "",
      status: "active",
      remarks: getCellVal(r, headers, ["备注"]) ? String(getCellVal(r, headers, ["备注"])) : "",
      isImported: true
    } as DatabaseAsset & { isImported?: boolean });
  }

  return list;
}

/** 04-中间件解析器 */
export function parseMiddlewareRows(rawRows: any[][]): (MiddlewareAsset & { isImported?: boolean })[] {
  const headerIdx = findHeaderRowIndex(rawRows, ["中间件类型", "中间件软件名称", "中间件版本", "服务端口"]);
  if (headerIdx === -1) return [];

  const headers = (rawRows[headerIdx] || []).map(x => String(x || "").trim());
  const list: (MiddlewareAsset & { isImported?: boolean })[] = [];

  for (let rIdx = headerIdx + 1; rIdx < rawRows.length; rIdx++) {
    const r = rawRows[rIdx];
    if (!r || !r.some(x => x !== null && String(x || "").trim() !== "")) continue;

    const mwSoftware = getCellVal(r, headers, ["中间件软件名称", "中间件软件", "中间件名称"]);
    const privateIp = getCellVal(r, headers, ["私有IP", "业务IP"]);
    const projName = getCellVal(r, headers, ["项目名称"]);

    if (!mwSoftware && !privateIp && !projName) continue;

    const seqVal = getCellVal(r, headers, ["序号"]);
    const seq = typeof seqVal === "number" ? seqVal : parseInt(seqVal, 10) || (rIdx + 1);

    const ipStr = privateIp ? String(privateIp) : "";
    const nameStr = mwSoftware ? String(mwSoftware) : `中间件_${ipStr}`;

    list.push({
      id: `imported-mw-${Date.now()}-${rIdx}`,
      seq: seq,
      projectNo: getCellVal(r, headers, ["项目编号"]) ? String(getCellVal(r, headers, ["项目编号"])) : "",
      customerName: String(getCellVal(r, headers, ["客户名称"]) || "北京市人力资源和社会保障局"),
      projectName: String(projName || "导入中间件"),
      env: String(getCellVal(r, headers, ["环境"]) || "生产"),
      cloudVendor: String(getCellVal(r, headers, ["云厂商"]) || "联通云"),
      regionName: String(getCellVal(r, headers, ["区域名称", "区域"]) || "政务外网区"),
      privateIp: ipStr,
      assetIp: ipStr,
      mwType: String(getCellVal(r, headers, ["中间件类型"]) || "应用服务器/Java Web 容器"),
      mwSoftware: mwSoftware ? String(mwSoftware) : "",
      name: nameStr,
      version: getCellVal(r, headers, ["中间件版本", "版本"]) ? String(getCellVal(r, headers, ["中间件版本", "版本"])) : "",
      port: getCellVal(r, headers, ["服务端口", "端口"]) ? String(getCellVal(r, headers, ["服务端口", "端口"])) : "",
      runtime: String(getCellVal(r, headers, ["应用程序运行环境", "运行环境", "运行时依赖", "运行时", "Runtime"]) || "JDK8"),
      status: "running",
      remarks: getCellVal(r, headers, ["备注"]) ? String(getCellVal(r, headers, ["备注"])) : "",
      isImported: true
    } as MiddlewareAsset & { isImported?: boolean });
  }

  return list;
}

/** 05-备份解析器 */
export function parseBackupRows(rawRows: any[][]): (BackupAsset & { isImported?: boolean })[] {
  const headerIdx = findHeaderRowIndex(rawRows, ["备份类型", "备份方式", "备份策略", "备份存储位置"]);
  if (headerIdx === -1) return [];

  const headers = (rawRows[headerIdx] || []).map(x => String(x || "").trim());
  const list: (BackupAsset & { isImported?: boolean })[] = [];

  for (let rIdx = headerIdx + 1; rIdx < rawRows.length; rIdx++) {
    const r = rawRows[rIdx];
    if (!r || !r.some(x => x !== null && String(x || "").trim() !== "")) continue;

    const privateIp = getCellVal(r, headers, ["私有IP", "业务IP"]);
    const projName = getCellVal(r, headers, ["项目名称"]);
    const bkType = getCellVal(r, headers, ["备份类型"]);

    if (!privateIp && !projName && !bkType) continue;

    const seqVal = getCellVal(r, headers, ["序号"]);
    const seq = typeof seqVal === "number" ? seqVal : parseInt(seqVal, 10) || (rIdx + 1);

    list.push({
      id: `imported-bk-${Date.now()}-${rIdx}`,
      seq: seq,
      projectNo: getCellVal(r, headers, ["项目编号"]) ? String(getCellVal(r, headers, ["项目编号"])) : "",
      customerName: String(getCellVal(r, headers, ["客户名称"]) || "北京市人力资源和社会保障局"),
      projectName: String(projName || "导入备份策略"),
      env: String(getCellVal(r, headers, ["环境"]) || "生产"),
      cloudVendor: String(getCellVal(r, headers, ["云厂商"]) || "联通云"),
      regionName: String(getCellVal(r, headers, ["区域名称", "区域"]) || "政务外网区"),
      privateIp: privateIp ? String(privateIp) : "",
      backupType: bkType ? String(bkType) : "数据库",
      backupMethod: String(getCellVal(r, headers, ["备份方式"]) || "物理备份"),
      backupPolicy: String(getCellVal(r, headers, ["备份策略"]) || "每日物理备"),
      storageLocation: String(getCellVal(r, headers, ["备份存储位置", "存储位置"]) || "专用NAS/对象存储"),
      retentionDays: 30,
      lastBackupTime: new Date().toISOString().slice(0, 10),
      status: "normal",
      remarks: getCellVal(r, headers, ["备注"]) ? String(getCellVal(r, headers, ["备注"])) : "",
      isImported: true
    } as BackupAsset & { isImported?: boolean });
  }

  return list;
}

/** 06-运维解析器 */
export function parseOpsRows(rawRows: any[][]): (OpsAsset & { isImported?: boolean })[] {
  const headerIdx = findHeaderRowIndex(rawRows, ["运维厂商", "VPN地址", "堡垒机地址", "访问服务器地址"]);
  if (headerIdx === -1) return [];

  const headers = (rawRows[headerIdx] || []).map(x => String(x || "").trim());
  const list: (OpsAsset & { isImported?: boolean })[] = [];

  for (let rIdx = headerIdx + 1; rIdx < rawRows.length; rIdx++) {
    const r = rawRows[rIdx];
    if (!r || !r.some(x => x !== null && String(x || "").trim() !== "")) continue;

    const privateIp = getCellVal(r, headers, ["私有IP", "业务IP"]);
    const projName = getCellVal(r, headers, ["项目名称"]);
    const vendor = getCellVal(r, headers, ["运维厂商"]);

    if (!privateIp && !projName && !vendor) continue;

    const seqVal = getCellVal(r, headers, ["序号"]);
    const seq = typeof seqVal === "number" ? seqVal : parseInt(seqVal, 10) || (rIdx + 1);

    list.push({
      id: `imported-ops-${Date.now()}-${rIdx}`,
      seq: seq,
      projectNo: getCellVal(r, headers, ["项目编号"]) ? String(getCellVal(r, headers, ["项目编号"])) : "",
      customerName: String(getCellVal(r, headers, ["客户名称"]) || "北京市人力资源和社会保障局"),
      projectName: String(projName || "导入运维保障"),
      env: String(getCellVal(r, headers, ["环境"]) || "生产"),
      cloudVendor: String(getCellVal(r, headers, ["云厂商"]) || "联通云"),
      regionName: String(getCellVal(r, headers, ["区域名称", "区域"]) || "政务外网区"),
      personnelAffiliation: String(getCellVal(r, headers, ["人员归属", "人员", "归属"]) || "伟仕"),
      vpnNetworkEnv: String(getCellVal(r, headers, ["使用人网vpn络环境", "网络环境", "网络"]) || "互联网区"),
      vpnAddress: getCellVal(r, headers, ["VPN地址"]) ? String(getCellVal(r, headers, ["VPN地址"])) : "",
      vpnAccount: getCellVal(r, headers, ["VPN账号"]) ? String(getCellVal(r, headers, ["VPN账号"])) : "",
      vpnUserName: getCellVal(r, headers, ["VPN使用人", "VPN对应名称", "对应名称", "使用人", "使用者"]) ? String(getCellVal(r, headers, ["VPN使用人", "VPN对应名称", "对应名称", "使用人", "使用者"])) : "",
      bastionAddress: getCellVal(r, headers, ["堡垒机地址"]) ? String(getCellVal(r, headers, ["堡垒机地址"])) : "",
      bastionAccount: getCellVal(r, headers, ["堡垒机账号"]) ? String(getCellVal(r, headers, ["堡垒机账号"])) : "",
      bastionUserName: getCellVal(r, headers, ["堡垒机使用人", "堡垒机对应名称"]) ? String(getCellVal(r, headers, ["堡垒机使用人", "堡垒机对应名称"])) : "",
      serverAccessAddress: getCellVal(r, headers, ["访问服务器地址", "服务器地址"]) ? String(getCellVal(r, headers, ["访问服务器地址", "服务器地址"])) : "",
      monitoringCoverage: "是",
      inspectionCycle: "每日",
      changeWindow: "周五晚",
      networkZone: "业务内网",
      exposureSurface: "业务内网",
      remarks: getCellVal(r, headers, ["备注"]) ? String(getCellVal(r, headers, ["备注"])) : "",
      isImported: true
    } as OpsAsset & { isImported?: boolean });
  }

  return list;
}

// ========================================================================
// 5. Unified Workbook Parser (Auto Sheet Discovery & Extraction)
// ========================================================================

export interface ParsedWorkbookResult {
  fileName: string;
  sheetsFound: {
    name: string;
    type: "hardware" | "database" | "middleware" | "backup" | "ops";
    count: number;
  }[];
  hardware: (VmHost & { isImported?: boolean })[];
  databases: (DatabaseAsset & { isImported?: boolean })[];
  middlewares: (MiddlewareAsset & { isImported?: boolean })[];
  backups: (BackupAsset & { isImported?: boolean })[];
  opsRecords: (OpsAsset & { isImported?: boolean })[];
  totalRecords: number;
}

/**
 * 智能解析 Excel 工作簿 (支持单个 Sheet 文件或完整 v351 五维全套工作簿)
 */
export async function parseExcelWorkbook(file: File): Promise<ParsedWorkbookResult> {
  const arrayBuffer = await file.arrayBuffer();
  const wb = XLSX.read(new Uint8Array(arrayBuffer), { type: "array" });

  const result: ParsedWorkbookResult = {
    fileName: file.name,
    sheetsFound: [],
    hardware: [],
    databases: [],
    middlewares: [],
    backups: [],
    opsRecords: [],
    totalRecords: 0
  };

  for (const sheetName of wb.SheetNames) {
    const ws = wb.Sheets[sheetName];
    if (!ws) continue;

    const rawRows: any[][] = XLSX.utils.sheet_to_json(ws, { header: 1, defval: null });
    if (!rawRows || rawRows.length === 0) continue;

    const lowerName = sheetName.toLowerCase();

    // Determine sheet type by name first, or fallback to header scan
    let detectedType: "hardware" | "database" | "middleware" | "backup" | "ops" | null = null;

    if (lowerName.includes("02") || lowerName.includes("硬件") || lowerName.includes("设备") || lowerName.includes("host")) {
      detectedType = "hardware";
    } else if (lowerName.includes("03") || lowerName.includes("数据库") || lowerName.includes("database") || lowerName.includes("db")) {
      detectedType = "database";
    } else if (lowerName.includes("04") || lowerName.includes("中间件") || lowerName.includes("middleware") || lowerName.includes("mw")) {
      detectedType = "middleware";
    } else if (lowerName.includes("05") || lowerName.includes("备份") || lowerName.includes("backup")) {
      detectedType = "backup";
    } else if (lowerName.includes("06") || lowerName.includes("运维") || lowerName.includes("ops")) {
      detectedType = "ops";
    } else {
      // Name not informative (e.g. "Sheet1" or export file): inspect first few rows
      const headerText = rawRows.slice(0, 5).map(r => r.join(" ")).join(" ");
      if (headerText.includes("数据库软件") || headerText.includes("数据库大类") || headerText.includes("数据库监听端口")) {
        detectedType = "database";
      } else if (headerText.includes("中间件软件") || headerText.includes("中间件类型")) {
        detectedType = "middleware";
      } else if (headerText.includes("备份方式") || headerText.includes("备份策略") || headerText.includes("备份存储位置")) {
        detectedType = "backup";
      } else if (headerText.includes("运维厂商") || headerText.includes("堡垒机") || headerText.includes("VPN账号")) {
        detectedType = "ops";
      } else if (headerText.includes("设备名称") || headerText.includes("CPU架构") || headerText.includes("OS发行版")) {
        detectedType = "hardware";
      }
    }

    if (!detectedType) continue;

    if (detectedType === "hardware") {
      const items = parseHardwareRows(rawRows);
      if (items.length > 0) {
        result.hardware.push(...items);
        result.sheetsFound.push({ name: sheetName, type: "hardware", count: items.length });
      }
    } else if (detectedType === "database") {
      const items = parseDatabaseRows(rawRows);
      if (items.length > 0) {
        result.databases.push(...items);
        result.sheetsFound.push({ name: sheetName, type: "database", count: items.length });
      }
    } else if (detectedType === "middleware") {
      const items = parseMiddlewareRows(rawRows);
      if (items.length > 0) {
        result.middlewares.push(...items);
        result.sheetsFound.push({ name: sheetName, type: "middleware", count: items.length });
      }
    } else if (detectedType === "backup") {
      const items = parseBackupRows(rawRows);
      if (items.length > 0) {
        result.backups.push(...items);
        result.sheetsFound.push({ name: sheetName, type: "backup", count: items.length });
      }
    } else if (detectedType === "ops") {
      const items = parseOpsRows(rawRows);
      if (items.length > 0) {
        result.opsRecords.push(...items);
        result.sheetsFound.push({ name: sheetName, type: "ops", count: items.length });
      }
    }
  }

  result.totalRecords =
    result.hardware.length +
    result.databases.length +
    result.middlewares.length +
    result.backups.length +
    result.opsRecords.length;

  return result;
}

/** 保留向下兼容的方法 */
export async function parseExcelToAssets(file: File): Promise<{
  devices: (VmHost & { isImported?: boolean })[];
  sheetName: string;
  totalRows: number;
}> {
  const res = await parseExcelWorkbook(file);
  const sheet = res.sheetsFound.find(s => s.type === "hardware") || res.sheetsFound[0];
  return {
    devices: res.hardware,
    sheetName: sheet ? sheet.name : "02-硬件设备",
    totalRows: res.hardware.length
  };
}

// ========================================================================
// 6. Keys & Diffing for All 5 Dimensions
// ========================================================================

export function getAssetKey(asset: {
  privateIp?: string | null;
  ip?: string | null;
  name?: string | null;
  projectName?: string | null;
  id?: string;
}): string {
  const rawIp = (asset.privateIp || asset.ip || "").trim();
  const isValidIp = rawIp && rawIp !== "null" && rawIp !== "undefined" && rawIp !== "-" && rawIp !== "/" && rawIp !== "0.0.0.0";
  if (isValidIp) {
    return `ip:${rawIp.toLowerCase()}`;
  }
  const proj = (asset.projectName || "").trim().toLowerCase();
  const name = (asset.name || "").trim().toLowerCase();
  if (name) {
    return `name:${proj}:::${name}`;
  }
  return `id:${asset.id || Math.random().toString()}`;
}

export function getDatabaseKey(db: {
  privateIp?: string | null;
  hostIp?: string | null;
  projectName?: string | null;
  dbSoftware?: string | null;
  port?: number | null;
  instanceSid?: string | null;
  name?: string | null;
  id?: string;
}): string {
  const ip = (db.privateIp || db.hostIp || "").trim().toLowerCase();
  const port = db.port || "";
  const sid = (db.instanceSid || "").trim().toLowerCase();
  const proj = (db.projectName || "").trim().toLowerCase();
  if (ip && port) {
    return `db:${ip}:${port}:${sid}`;
  }
  if (ip) {
    return `db:${ip}:${sid || proj}`;
  }
  return `db:${proj}:::${db.name || db.id || Math.random().toString()}`;
}

export function getMiddlewareKey(mw: {
  privateIp?: string | null;
  assetIp?: string | null;
  projectName?: string | null;
  mwSoftware?: string | null;
  name?: string | null;
  port?: string | number | null;
  id?: string;
}): string {
  const ip = (mw.privateIp || mw.assetIp || "").trim().toLowerCase();
  const port = String(mw.port || "").trim();
  const name = (mw.mwSoftware || mw.name || "").trim().toLowerCase();
  const proj = (mw.projectName || "").trim().toLowerCase();
  if (ip && (port || name)) {
    return `mw:${ip}:${port}:${name}`;
  }
  return `mw:${proj}:::${ip || name || mw.id || Math.random().toString()}`;
}

export function getBackupKey(bk: {
  privateIp?: string | null;
  projectName?: string | null;
  backupType?: string | null;
  id?: string;
}): string {
  const ip = (bk.privateIp || "").trim().toLowerCase();
  const proj = (bk.projectName || "").trim().toLowerCase();
  const type = (bk.backupType || "").trim().toLowerCase();
  if (ip && proj) {
    return `bk:${ip}:${proj}:${type}`;
  }
  if (ip) return `bk:${ip}:${type}`;
  return `bk:${proj}:::${type || bk.id || Math.random().toString()}`;
}

export function getOpsKey(ops: {
  privateIp?: string | null;
  projectName?: string | null;
  opsVendor?: string | null;
  id?: string;
}): string {
  const ip = (ops.privateIp || "").trim().toLowerCase();
  const proj = (ops.projectName || "").trim().toLowerCase();
  if (ip && proj) {
    return `ops:${ip}:${proj}`;
  }
  if (ip) return `ops:${ip}`;
  return `ops:${proj}:::${ops.opsVendor || ops.id || Math.random().toString()}`;
}

export interface AssetDiffResult {
  hasChanged: boolean;
  diffs: { field: string; label: string; oldVal: any; newVal: any }[];
}

export function diffAssets(
  existing: Partial<VmHost>,
  incoming: Partial<VmHost>
): AssetDiffResult {
  const diffs: { field: string; label: string; oldVal: any; newVal: any }[] = [];

  const compareList: { field: keyof VmHost; label: string }[] = [
    { field: "cpuCores", label: "CPU核数" },
    { field: "memoryGb", label: "内存(GB)" },
    { field: "systemDiskGb", label: "系统盘(GB)" },
    { field: "dataDiskGb", label: "数据盘(GB)" },
    { field: "osVersion", label: "OS版本" },
    { field: "kernelVersion", label: "内核版本" },
    { field: "isXinchuang", label: "是否信创" },
    { field: "remotePort", label: "远程端口" },
    { field: "remarks", label: "备注" },
    { field: "env", label: "环境" },
    { field: "cloudVendor", label: "云厂商" },
    { field: "regionName", label: "区域" },
    { field: "internalWanIp", label: "内大网IP" },
    { field: "vip", label: "VIP" },
    { field: "eip", label: "EIP" },
    { field: "publicIp", label: "公网IP" }
  ];

  for (const item of compareList) {
    const vOld = existing[item.field];
    const vNew = incoming[item.field];
    if (vNew !== undefined && vNew !== null && vNew !== "") {
      if (vOld !== vNew && String(vOld ?? "").trim() !== String(vNew ?? "").trim()) {
        diffs.push({
          field: item.field,
          label: item.label,
          oldVal: vOld ?? "无",
          newVal: vNew
        });
      }
    }
  }

  return {
    hasChanged: diffs.length > 0,
    diffs
  };
}

export function diffDatabases(existing: Partial<DatabaseAsset>, incoming: Partial<DatabaseAsset>): AssetDiffResult {
  const diffs: { field: string; label: string; oldVal: any; newVal: any }[] = [];
  const list: { field: keyof DatabaseAsset; label: string }[] = [
    { field: "dbSoftware", label: "数据库软件" },
    { field: "version", label: "版本" },
    { field: "port", label: "端口" },
    { field: "instanceSid", label: "实例/SID" },
    { field: "dbName", label: "业务库名" },
    { field: "deployMode", label: "部署模式" },
    { field: "clusterName", label: "集群名" },
    { field: "vipEip", label: "VIP/EIP" },
    { field: "remarks", label: "备注" }
  ];
  for (const item of list) {
    const vOld = existing[item.field];
    const vNew = incoming[item.field];
    if (vNew !== undefined && vNew !== null && vNew !== "" && String(vOld ?? "").trim() !== String(vNew ?? "").trim()) {
      diffs.push({ field: item.field, label: item.label, oldVal: vOld ?? "无", newVal: vNew });
    }
  }
  return { hasChanged: diffs.length > 0, diffs };
}

export function diffMiddlewares(existing: Partial<MiddlewareAsset>, incoming: Partial<MiddlewareAsset>): AssetDiffResult {
  const diffs: { field: string; label: string; oldVal: any; newVal: any }[] = [];
  const list: { field: keyof MiddlewareAsset; label: string }[] = [
    { field: "mwType", label: "中间件类型" },
    { field: "mwSoftware", label: "软件名称" },
    { field: "version", label: "版本" },
    { field: "port", label: "服务端口" },
    { field: "remarks", label: "备注" }
  ];
  for (const item of list) {
    const vOld = existing[item.field];
    const vNew = incoming[item.field];
    if (vNew !== undefined && vNew !== null && vNew !== "" && String(vOld ?? "").trim() !== String(vNew ?? "").trim()) {
      diffs.push({ field: item.field, label: item.label, oldVal: vOld ?? "无", newVal: vNew });
    }
  }
  return { hasChanged: diffs.length > 0, diffs };
}

export function diffBackups(existing: Partial<BackupAsset>, incoming: Partial<BackupAsset>): AssetDiffResult {
  const diffs: { field: string; label: string; oldVal: any; newVal: any }[] = [];
  const list: { field: keyof BackupAsset; label: string }[] = [
    { field: "backupType", label: "备份类型" },
    { field: "backupMethod", label: "备份方式" },
    { field: "backupPolicy", label: "备份策略" },
    { field: "storageLocation", label: "存储位置" },
    { field: "remarks", label: "备注" }
  ];
  for (const item of list) {
    const vOld = existing[item.field];
    const vNew = incoming[item.field];
    if (vNew !== undefined && vNew !== null && vNew !== "" && String(vOld ?? "").trim() !== String(vNew ?? "").trim()) {
      diffs.push({ field: item.field, label: item.label, oldVal: vOld ?? "无", newVal: vNew });
    }
  }
  return { hasChanged: diffs.length > 0, diffs };
}

export function diffOps(existing: Partial<OpsAsset>, incoming: Partial<OpsAsset>): AssetDiffResult {
  const diffs: { field: string; label: string; oldVal: any; newVal: any }[] = [];
  const list: { field: keyof OpsAsset; label: string }[] = [
    { field: "opsVendor", label: "运维厂商" },
    { field: "vpnAddress", label: "VPN地址" },
    { field: "vpnAccount", label: "VPN账号" },
    { field: "bastionAddress", label: "堡垒机地址" },
    { field: "bastionAccount", label: "堡垒机账号" },
    { field: "serverAccessAddress", label: "访问服务器地址" },
    { field: "remarks", label: "备注" }
  ];
  for (const item of list) {
    const vOld = existing[item.field];
    const vNew = incoming[item.field];
    if (vNew !== undefined && vNew !== null && vNew !== "" && String(vOld ?? "").trim() !== String(vNew ?? "").trim()) {
      diffs.push({ field: item.field, label: item.label, oldVal: vOld ?? "无", newVal: vNew });
    }
  }
  return { hasChanged: diffs.length > 0, diffs };
}
