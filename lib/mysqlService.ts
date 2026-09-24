import {
  initialProjects,
  initialPhysicalHosts,
  initialVms,
  initialSwitches,
  initialDatabases,
  initialMiddlewares,
  initialBackups,
  initialOpsRecords,
  ProjectGroup,
  PhysicalHost,
  VmHost,
  SwitchDevice,
  DatabaseAsset,
  MiddlewareAsset,
  BackupAsset,
  OpsAsset
} from "@/app/cmdbData";

export interface CmdbAggregatePayload {
  ok: boolean;
  source: "mysql" | "v360_seed";
  isDatabaseConnected: boolean;
  dbHost?: string;
  dbName?: string;
  counts: {
    totalAssets: number;
    physicalHosts: number;
    vms: number;
    switches: number;
    databases: number;
    middlewares: number;
    backups: number;
    opsRecords: number;
    projects: number;
  };
  projects: ProjectGroup[];
  physicalHosts: PhysicalHost[];
  vms: VmHost[];
  switches: SwitchDevice[];
  databases: DatabaseAsset[];
  middlewares: MiddlewareAsset[];
  backups: BackupAsset[];
  opsRecords: OpsAsset[];
  lastFetched: string;
}

// In-memory cache or runtime state
let mysqlPool: any = null;

async function getMysqlPool() {
  if (mysqlPool) return mysqlPool;

  const host = process.env.MYSQL_HOST;
  const databaseUrl = process.env.DATABASE_URL;

  // If no environment variables set for MySQL, return null
  if (!host && !databaseUrl) {
    return null;
  }

  try {
    const mysql = await import("mysql2/promise");
    if (databaseUrl) {
      mysqlPool = mysql.createPool(databaseUrl);
    } else {
      mysqlPool = mysql.createPool({
        host: process.env.MYSQL_HOST || "127.0.0.1",
        port: Number(process.env.MYSQL_PORT || 3306),
        user: process.env.MYSQL_USER || "root",
        password: process.env.MYSQL_PASSWORD || "",
        database: process.env.MYSQL_DATABASE || "autops",
        waitForConnections: true,
        connectionLimit: 10,
        queueLimit: 0,
        enableKeepAlive: true,
        keepAliveInitialDelay: 0
      });
    }
    return mysqlPool;
  } catch (err) {
    console.warn("[CMDB MySQL] Could not initialize MySQL pool, falling back to v360 data store:", err);
    return null;
  }
}

/**
 * Fetch all CMDB assets and records.
 * Priority:
 * 1. If MySQL is configured & healthy -> SELECT from MySQL tables.
 * 2. Fallback -> Return v360 seeded data from cmdbData.ts.
 */
export async function fetchAllCmdbData(): Promise<CmdbAggregatePayload> {
  const pool = await getMysqlPool();

  if (pool) {
    try {
      const [pRows] = await pool.query("SELECT * FROM `cmdb_projects`");
      const [hRows] = await pool.query("SELECT * FROM `cmdb_hardware_assets` ORDER BY `seq` ASC");
      const [dRows] = await pool.query("SELECT * FROM `cmdb_databases` ORDER BY `seq` ASC");
      const [mRows] = await pool.query("SELECT * FROM `cmdb_middlewares` ORDER BY `seq` ASC");
      const [bRows] = await pool.query("SELECT * FROM `cmdb_backups` ORDER BY `seq` ASC");
      const [oRows] = await pool.query("SELECT * FROM `cmdb_ops_accounts` ORDER BY `seq` ASC");

      const rawHw: any[] = (hRows as any[]) || [];
      const phyList: PhysicalHost[] = rawHw
        .filter(h => h.device_type === "物理机")
        .map(mapDbHardwareToPhysical);
      const vmList: VmHost[] = rawHw
        .filter(h => h.device_type !== "物理机")
        .map(mapDbHardwareToVm);

      const dbList: DatabaseAsset[] = ((dRows as any[]) || []).map(mapDbDatabaseToModel);
      const mwList: MiddlewareAsset[] = ((mRows as any[]) || []).map(mapDbMiddlewareToModel);
      const bkList: BackupAsset[] = ((bRows as any[]) || []).map(mapDbBackupToModel);
      const opsList: OpsAsset[] = ((oRows as any[]) || []).map(mapDbOpsToModel);
      const projList: ProjectGroup[] = ((pRows as any[]) || []).map(mapDbProjectToModel);

      return {
        ok: true,
        source: "mysql",
        isDatabaseConnected: true,
        dbHost: process.env.MYSQL_HOST || "configured-pool",
        dbName: process.env.MYSQL_DATABASE || "autops",
        counts: {
          totalAssets: phyList.length + vmList.length,
          physicalHosts: phyList.length,
          vms: vmList.length,
          switches: 0,
          databases: dbList.length,
          middlewares: mwList.length,
          backups: bkList.length,
          opsRecords: opsList.length,
          projects: projList.length
        },
        projects: projList.length > 0 ? projList : initialProjects,
        physicalHosts: phyList,
        vms: vmList,
        switches: [],
        databases: dbList,
        middlewares: mwList,
        backups: bkList,
        opsRecords: opsList,
        lastFetched: new Date().toISOString()
      };
    } catch (dbErr) {
      console.warn("[CMDB MySQL Query] Query failed, seamlessly falling back to v360 data store:", dbErr);
    }
  }

  // Fallback to local 360 data (299 assets, 47 databases, 1 middleware, 1 backup, 81 ops)
  return {
    ok: true,
    source: "v360_seed",
    isDatabaseConnected: false,
    counts: {
      totalAssets: initialPhysicalHosts.length + initialVms.length + initialSwitches.length,
      physicalHosts: initialPhysicalHosts.length,
      vms: initialVms.length,
      switches: initialSwitches.length,
      databases: initialDatabases.length,
      middlewares: initialMiddlewares.length,
      backups: initialBackups.length,
      opsRecords: initialOpsRecords.length,
      projects: initialProjects.length
    },
    projects: initialProjects,
    physicalHosts: initialPhysicalHosts,
    vms: initialVms,
    switches: initialSwitches,
    databases: initialDatabases,
    middlewares: initialMiddlewares,
    backups: initialBackups,
    opsRecords: initialOpsRecords,
    lastFetched: new Date().toISOString()
  };
}

function mapDbHardwareToPhysical(r: any): PhysicalHost {
  const seq = Number(r.seq || 0);
  const cpuCores = Number(r.cpu_cores || 0);
  const memoryGb = Number(r.memory_gb || 0);
  const sysDiskGb = Number(r.sys_disk_gb || 0);
  const dataDiskGb = Number(r.data_disk_gb || 0);
  return {
    id: r.id || `host-v360-${seq}`,
    assetNo: `SRV-PHY-${seq.toString().padStart(3, "0")}`,
    hostname: r.device_name || "",
    name: r.device_name || "",
    ip: r.private_ip || "",
    bmcIp: "",
    brand: "信创/国产服务器",
    model: "机架式服务器",
    os: `${r.os_distro || ""} ${r.os_version || ""}`.trim(),
    cpu: `${cpuCores}核`,
    memory: `${memoryGb}GB`,
    disk: `${sysDiskGb + dataDiskGb}GB`,
    role: r.device_category || "服务器",
    businessId: "biz-1",
    status: "running",
    powerWatts: 350,
    cpuTemp: 42,
    boardTemp: 38,
    inletTemp: 22,
    fanRpm: "5400 RPM",
    psu1Status: "正常",
    psu2Status: "正常",
    raidStatus: "RAID1",
    agentOnline: true,
    updated: "2026-09-22",
    roomId: "room-unicom",
    roomName: r.idc_room || "首信机房",
    seq,
    projectNo: r.project_no || "",
    customerName: r.customer_name || "",
    projectName: r.project_name || "",
    projectId: r.project_id || "",
    env: r.env || "生产",
    cloudVendor: r.cloud_vendor || "",
    regionName: r.region_name || "",
    deviceName: r.device_name || "",
    deviceCategory: r.device_category || "服务器",
    deviceType: r.device_type || "物理机",
    idcRoom: r.idc_room || "",
    privateIp: r.private_ip || "",
    privateIpv6: r.private_ipv6 || "",
    innerNetIp: r.inner_net_ip || "",
    eip: r.eip || "",
    vip: r.vip || "",
    publicIp: r.public_ip || "",
    cpuArch: r.cpu_arch || "",
    cpuCores,
    memoryGb,
    sysDiskGb,
    dataDiskGb,
    shareDiskGb: Number(r.share_disk_gb || 0),
    ossGb: Number(r.oss_gb || 0),
    osDistro: r.os_distro || "",
    osVersion: r.os_version || "",
    kernelVersion: r.kernel_version || "",
    isXinchuang: r.is_xinchuang || "否",
    remotePort: Number(r.remote_port || 22),
    remarks: r.remarks || ""
  };
}

function mapDbHardwareToVm(r: any): VmHost {
  const seq = Number(r.seq || 0);
  const cpuCores = Number(r.cpu_cores || 0);
  const memoryGb = Number(r.memory_gb || 0);
  const sysDiskGb = Number(r.sys_disk_gb || 0);
  const dataDiskGb = Number(r.data_disk_gb || 0);
  return {
    id: r.id || `vm-v360-${seq}`,
    assetNo: `VM-${seq.toString().padStart(3, "0")}`,
    name: r.device_name || "",
    physicalHostId: "host-v360-1",
    ip: r.private_ip || "",
    privateIp: r.private_ip || "",
    cpu: `${cpuCores}核`,
    memory: `${memoryGb}GB`,
    disk: `${sysDiskGb + dataDiskGb}GB`,
    os: `${r.os_distro || ""} ${r.os_version || ""}`.trim(),
    businessId: "biz-1",
    status: "running",
    role: r.device_category || "服务器",
    updated: "2026-09-22",
    roomId: "room-unicom",
    roomName: r.idc_room || "联通酒仙桥机房",
    seq,
    projectNo: r.project_no || "",
    customerName: r.customer_name || "",
    projectName: r.project_name || "",
    projectId: r.project_id || "",
    env: r.env || "生产",
    cloudVendor: r.cloud_vendor || "",
    regionName: r.region_name || "",
    deviceName: r.device_name || "",
    deviceCategory: r.device_category || "服务器",
    deviceType: r.device_type || "虚拟机",
    idcRoom: r.idc_room || "",
    privateIpv6: r.private_ipv6 || "",
    innerNetIp: r.inner_net_ip || "",
    eip: r.eip || "",
    vip: r.vip || "",
    publicIp: r.public_ip || "",
    cpuArch: r.cpu_arch || "",
    cpuCores,
    memoryGb,
    sysDiskGb,
    dataDiskGb,
    shareDiskGb: Number(r.share_disk_gb || 0),
    ossGb: Number(r.oss_gb || 0),
    osDistro: r.os_distro || "",
    osVersion: r.os_version || "",
    kernelVersion: r.kernel_version || "",
    isXinchuang: r.is_xinchuang || "否",
    remotePort: Number(r.remote_port || 22),
    remarks: r.remarks || ""
  };
}

function mapDbDatabaseToModel(r: any): DatabaseAsset {
  return {
    id: r.id,
    seq: r.seq,
    projectNo: r.project_no || "",
    customerName: r.customer_name || "",
    projectName: r.project_name || "",
    name: `${r.customer_name || ""}-${r.project_name || ""}-${r.db_software || ""}`.replace(/^-+|-+$/g, "") || `数据库实例-${r.seq}`,
    type: r.db_software || "关系型数据库",
    hostIp: r.private_ip || "",
    projectId: r.project_id || "",
    env: r.env || "生产",
    cloudVendor: r.cloud_vendor || "",
    regionName: r.region_name || "",
    privateIp: r.private_ip || "",
    vipEip: r.vip_eip || "",
    dbCategory: r.db_category || "",
    dbSoftware: r.db_software || "",
    version: r.version || "",
    port: Number(r.port || 0),
    instanceSid: r.instance_sid || "",
    dbName: r.db_name || "",
    deployMode: r.deploy_mode || "单机",
    clusterName: r.cluster_name || "",
    status: "active",
    remarks: r.remarks || ""
  };
}

function mapDbMiddlewareToModel(r: any): MiddlewareAsset {
  return {
    id: r.id,
    seq: r.seq,
    projectNo: r.project_no || "",
    customerName: r.customer_name || "",
    projectName: r.project_name || "",
    name: `${r.mw_software || ""} ${r.version || ""}`.trim() || "中间件服务",
    projectId: r.project_id || "",
    env: r.env || "生产",
    cloudVendor: r.cloud_vendor || "",
    regionName: r.region_name || "",
    privateIp: r.private_ip || "",
    assetIp: r.private_ip || "",
    assetName: `${r.project_name || ""}-中间件`,
    mwType: r.mw_type || "",
    mwSoftware: r.mw_software || "",
    version: r.version || "",
    port: String(r.port || ""),
    appRuntimeEnv: r.app_runtime_env || "",
    status: "running",
    remarks: r.remarks || ""
  };
}

function mapDbBackupToModel(r: any): BackupAsset {
  return {
    id: r.id,
    seq: r.seq,
    projectNo: r.project_no || "",
    customerName: r.customer_name || "",
    projectName: r.project_name || "",
    projectId: r.project_id || "",
    env: r.env || "生产",
    cloudVendor: r.cloud_vendor || "",
    regionName: r.region_name || "",
    privateIp: r.private_ip || "",
    backupType: r.backup_type || "全量备份",
    backupMethod: r.backup_method || "冷备",
    backupPolicy: r.backup_strategy || "每日备份",
    storageLocation: r.backup_storage_path || "政务外网专用存储池",
    retentionDays: Number(r.retention_period || 30) || 30,
    lastBackupTime: "2026-09-22 03:00:00",
    status: "normal",
    remarks: r.remarks || ""
  };
}

function mapDbOpsToModel(r: any): OpsAsset {
  return {
    id: r.id,
    seq: r.seq,
    projectNo: r.project_no || "",
    customerName: r.customer_name || "",
    projectName: r.project_name || "",
    projectId: r.project_id || "",
    cloudVendor: r.cloud_vendor || "",
    idcRoom: r.idc_room || "",
    personnelAffiliation: r.personnel_affiliation || "",
    vpnNetworkEnv: r.vpn_network_env || "",
    vpnAddress: r.vpn_url || "",
    vpnAccount: r.vpn_account || "",
    vpnUserName: r.vpn_user_name || "",
    fortressArea: r.fortress_area || "",
    bastionAddress: r.fortress_url || "",
    bastionAccount: r.fortress_account || "",
    bastionUserName: r.fortress_user_name || "",
    opsVendor: r.personnel_affiliation || "北控伟仕",
    serverAccessAddress: r.fortress_url ? `bastion ${r.fortress_url}` : "ssh",
    monitoringCoverage: "是",
    inspectionCycle: "每日",
    changeWindow: "周五晚",
    networkZone: r.fortress_area || "互联网区",
    exposureSurface: "VPN/堡垒机接入",
    remarks: r.remarks || ""
  };
}

function mapDbProjectToModel(r: any): ProjectGroup {
  return {
    id: r.id,
    name: r.name,
    code: r.code || "",
    customerName: r.customer_name,
    env: r.env || "生产",
    cloudVendor: r.cloud_vendor || "",
    regionName: r.region_name || "",
    deviceCount: Number(r.device_count || 0),
    phyCount: Number(r.phy_count || 0),
    vmCount: Number(r.vm_count || 0),
    netCount: 0,
    totalCores: Number(r.total_cores || 0),
    totalMemoryGb: Number(r.total_memory_gb || 0),
    totalDiskGb: Number(r.total_disk_gb || 0),
    xinchuangCount: Number(r.xinchuang_count || 0),
    dbCount: Number(r.db_count || 0),
    mwCount: Number(r.mw_count || 0),
    backupCount: Number(r.bk_count || 0),
    opsCount: Number(r.ops_count || 0),
    healthScore: 98,
    description: r.description || `${r.customer_name} - ${r.name}`
  };
}
