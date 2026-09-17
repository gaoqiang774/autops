// CMDB Core Relational Data Model & Seed Store

export interface IdcRoom {
  id: string;
  name: string;
  code: string;
  operator: "中国电信" | "中国联通" | "中国移动" | "BGP多线";
  city: string;
  address: string;
  level: "T3" | "T3+" | "T4";
  cabinetCount: number;
  contact: string;
  phone: string;
  status: "正常运行" | "扩容建设";
  remark: string;
}

export interface IdcCabinet {
  id: string;
  roomId: string;
  name: string;
  code: string;
  totalU: number; // typically 42
  usedU: number;
  maxPower: number; // kW
  currentPower: number; // kW
  temperature: number; // °C
  status: "正常" | "预警" | "空闲";
  manager: string;
  row: string; // e.g. "Row A", "Row B"
  remark: string;
}

export interface PhysicalHost {
  id: string;
  assetNo: string;
  hostname: string;
  roomId: string;
  cabinetId: string;
  startU: number; // 1 ~ 42
  uHeight: number; // 1, 2, 4
  ip: string;
  bmcIp: string;
  brand: "Dell" | "华为" | "浪潮" | "H3C" | "联想";
  model: string;
  os: string;
  cpu: string;
  memory: string;
  disk: string;
  role: string;
  businessId: string;
  status: "running" | "warning" | "offline";
  powerWatts: number;
  cpuTemp: number;
  boardTemp: number;
  inletTemp: number;
  fanRpm: string;
  psu1Status: "正常 (185W)" | "异常";
  psu2Status: "正常 (175W)" | "异常";
  raidStatus: "RAID1 正常" | "RAID5 正常" | "RAID10 正常";
  agentOnline: boolean;
  updated: string;
}

export interface VmHost {
  id: string;
  name: string;
  physicalHostId: string; // Hypervisor relationship
  ip: string;
  privateIp: string;
  cpu: string;
  memory: string;
  disk: string;
  os: string;
  businessId: string;
  status: "running" | "stopped";
  role: string;
  updated: string;
}

export interface SwitchDevice {
  id: string;
  assetNo: string;
  name: string;
  roomId: string;
  cabinetId: string;
  startU: number;
  uHeight: number;
  ip: string;
  brand: "华为" | "锐捷" | "思科" | "华三";
  model: string;
  role: "核心交换机" | "汇聚交换机" | "接入交换机" | "带外交换机";
  portCount: number;
  activePorts: number;
  status: "online" | "warning" | "offline";
  businessId: string;
}

export interface DatabaseAsset {
  id: string;
  name: string;
  type: "MySQL" | "Redis" | "PostgreSQL" | "MongoDB" | "Oracle";
  version: string;
  hostIp: string;
  port: number;
  arch: "主从高可用 (1主2从)" | "集群分片 (3主3从)" | "单机测试实例" | "主备高可用";
  businessId: string;
  dataSize: string;
  connectionCount: number;
  status: "active" | "standby" | "maintenance";
}

export interface BusinessModel {
  id: string;
  name: string;
  code: string;
  level: "核心 L1" | "重要 L2" | "通用 L3";
  owner: string;
  department: string;
  description: string;
  healthScore: number;
  hostIds: string[];
  dbIds: string[];
  switchIds: string[];
}

export interface AgentProbe {
  id: string;
  hostIp: string;
  hostname: string;
  agentVersion: string;
  status: "online" | "offline";
  cpuUsage: number;
  memUsage: number;
  lastHeartbeat: string;
}

export interface CredentialItem {
  id: string;
  name: string;
  type: "SSH 密码" | "SSH 私钥" | "BMC IPMI" | "SNMP v2c/v3" | "数据库账号";
  username: string;
  targetCount: number;
  updated: string;
  remark: string;
}

// Initial Seed Data mirroring real enterprise CMDB IDC system
export const initialRooms: IdcRoom[] = [
  {
    id: "room-1",
    name: "北京亦庄BGP核心机房",
    code: "BJ-YZ-01",
    operator: "BGP多线",
    city: "北京",
    address: "北京市经济技术开发区地盛西路1号数据中心A栋3F",
    level: "T3+",
    cabinetCount: 24,
    contact: "王工程师",
    phone: "13801018899",
    status: "正常运行",
    remark: "华北主生产机房，双路UPS市电接入，配备精密空调与气体灭火系统。"
  },
  {
    id: "room-2",
    name: "上海张江云计算数据中心",
    code: "SH-ZJ-02",
    operator: "中国电信",
    city: "上海",
    address: "上海市浦东新区张江高科技园区博霞路50号2号机房",
    level: "T3",
    cabinetCount: 16,
    contact: "李主管",
    phone: "13912345678",
    status: "正常运行",
    remark: "华东同城容灾机房，双千兆专线直连亦庄。"
  },
  {
    id: "room-3",
    name: "深圳南山金融级自建机房",
    code: "SZ-NS-03",
    operator: "中国移动",
    city: "深圳",
    address: "深圳市南山区粤海街道科技南十二路中科研发园B座5F",
    level: "T4",
    cabinetCount: 12,
    contact: "张运维",
    phone: "13766668888",
    status: "正常运行",
    remark: "华南边缘及创新应用主节点。"
  }
];

export const initialCabinets: IdcCabinet[] = [
  {
    id: "cab-101",
    roomId: "room-1",
    name: "A01 机柜",
    code: "BJ-YZ-A01",
    totalU: 42,
    usedU: 18,
    maxPower: 5.0,
    currentPower: 2.8,
    temperature: 22.5,
    status: "正常",
    manager: "王工",
    row: "Row A",
    remark: "生产核心计算与核心网络接入柜"
  },
  {
    id: "cab-102",
    roomId: "room-1",
    name: "A02 机柜",
    code: "BJ-YZ-A02",
    totalU: 42,
    usedU: 24,
    maxPower: 5.0,
    currentPower: 3.4,
    temperature: 23.1,
    status: "正常",
    manager: "王工",
    row: "Row A",
    remark: "大数据分析与高密度存储节点机柜"
  },
  {
    id: "cab-103",
    roomId: "room-1",
    name: "B01 机柜",
    code: "BJ-YZ-B01",
    totalU: 42,
    usedU: 10,
    maxPower: 5.0,
    currentPower: 1.6,
    temperature: 21.8,
    status: "空闲",
    manager: "刘工",
    row: "Row B",
    remark: "备用扩容与测试设备机柜"
  },
  {
    id: "cab-201",
    roomId: "room-2",
    name: "C01 机柜",
    code: "SH-ZJ-C01",
    totalU: 42,
    usedU: 16,
    maxPower: 4.5,
    currentPower: 2.2,
    temperature: 23.4,
    status: "正常",
    manager: "李工",
    row: "Row C",
    remark: "华东灾备数据库与中间件计算柜"
  },
  {
    id: "cab-301",
    roomId: "room-3",
    name: "D01 机柜",
    code: "SZ-NS-D01",
    totalU: 42,
    usedU: 14,
    maxPower: 5.0,
    currentPower: 2.0,
    temperature: 22.0,
    status: "正常",
    manager: "张工",
    row: "Row D",
    remark: "华南物联网数据汇聚与边缘AI分析机柜"
  }
];

export const initialHosts: PhysicalHost[] = [
  {
    id: "host-1",
    assetNo: "SRV-BJ-001",
    hostname: "bj-prod-k8s-node01",
    roomId: "room-1",
    cabinetId: "cab-101",
    startU: 38,
    uHeight: 2,
    ip: "10.100.10.21",
    bmcIp: "192.168.100.21",
    brand: "Dell",
    model: "PowerEdge R740",
    os: "CentOS 7.9 (Linux 3.10)",
    cpu: "2× Intel Xeon Silver 4210R (20C/40T)",
    memory: "128 GB DDR4 ECC",
    disk: "2× 960GB NVMe SSD (OS) + 4× 4TB SATA",
    role: "Kubernetes Master / 核心计算宿主",
    businessId: "biz-1",
    status: "running",
    powerWatts: 340,
    cpuTemp: 41,
    boardTemp: 32,
    inletTemp: 22,
    fanRpm: "4200 RPM (正常)",
    psu1Status: "正常 (185W)",
    psu2Status: "正常 (175W)",
    raidStatus: "RAID1 正常",
    agentOnline: true,
    updated: "2026-09-17 19:40"
  },
  {
    id: "host-2",
    assetNo: "SRV-BJ-002",
    hostname: "bj-prod-k8s-node02",
    roomId: "room-1",
    cabinetId: "cab-101",
    startU: 35,
    uHeight: 2,
    ip: "10.100.10.22",
    bmcIp: "192.168.100.22",
    brand: "华为",
    model: "FusionServer 2288H V5",
    os: "Ubuntu 22.04 LTS",
    cpu: "2× Intel Xeon Gold 5218R (40C/80T)",
    memory: "256 GB DDR4 ECC",
    disk: "4× 1.92TB NVMe SSD",
    role: "微服务网关与应用生产节点",
    businessId: "biz-1",
    status: "running",
    powerWatts: 380,
    cpuTemp: 44,
    boardTemp: 34,
    inletTemp: 23,
    fanRpm: "4600 RPM (正常)",
    psu1Status: "正常 (195W)",
    psu2Status: "正常 (185W)",
    raidStatus: "RAID10 正常",
    agentOnline: true,
    updated: "2026-09-17 19:45"
  },
  {
    id: "host-3",
    assetNo: "SRV-BJ-003",
    hostname: "bj-db-mysql-master",
    roomId: "room-1",
    cabinetId: "cab-101",
    startU: 32,
    uHeight: 2,
    ip: "10.100.10.31",
    bmcIp: "192.168.100.31",
    brand: "Dell",
    model: "PowerEdge R750",
    os: "Rocky Linux 9.2",
    cpu: "2× Intel Xeon Gold 6330 (56C/112T)",
    memory: "512 GB DDR4 ECC",
    disk: "8× 3.84TB SAS Enterprise SSD",
    role: "核心水务调度数据库主库",
    businessId: "biz-1",
    status: "running",
    powerWatts: 420,
    cpuTemp: 46,
    boardTemp: 36,
    inletTemp: 23,
    fanRpm: "5100 RPM (正常)",
    psu1Status: "正常 (215W)",
    psu2Status: "正常 (205W)",
    raidStatus: "RAID10 正常",
    agentOnline: true,
    updated: "2026-09-17 20:00"
  },
  {
    id: "host-4",
    assetNo: "SRV-BJ-004",
    hostname: "bj-ai-gpu-node01",
    roomId: "room-1",
    cabinetId: "cab-102",
    startU: 26,
    uHeight: 4,
    ip: "10.100.10.88",
    bmcIp: "192.168.100.88",
    brand: "浪潮",
    model: "NF5468M5 (4U 8-GPU)",
    os: "Ubuntu 22.04 LTS",
    cpu: "2× Intel Xeon Gold 6248R + 4× NVIDIA A100",
    memory: "512 GB DDR4",
    disk: "2× 1.92TB OS + 4× 7.68TB NVMe",
    role: "AIOps水质预测与智能视觉推理服务器",
    businessId: "biz-2",
    status: "running",
    powerWatts: 1450,
    cpuTemp: 52,
    boardTemp: 38,
    inletTemp: 24,
    fanRpm: "7800 RPM (高负荷)",
    psu1Status: "正常 (750W)",
    psu2Status: "正常 (700W)",
    raidStatus: "RAID5 正常",
    agentOnline: true,
    updated: "2026-09-17 20:10"
  },
  {
    id: "host-5",
    assetNo: "SRV-SH-001",
    hostname: "sh-dr-db-replica",
    roomId: "room-2",
    cabinetId: "cab-201",
    startU: 36,
    uHeight: 2,
    ip: "10.200.10.32",
    bmcIp: "192.168.200.32",
    brand: "Dell",
    model: "PowerEdge R740",
    os: "Rocky Linux 9.2",
    cpu: "2× Intel Xeon Silver 4214R (24C/48T)",
    memory: "256 GB DDR4",
    disk: "6× 1.92TB SSD",
    role: "上海灾备中心实时只读副本",
    businessId: "biz-1",
    status: "running",
    powerWatts: 320,
    cpuTemp: 39,
    boardTemp: 31,
    inletTemp: 21,
    fanRpm: "3900 RPM (正常)",
    psu1Status: "正常 (160W)",
    psu2Status: "正常 (160W)",
    raidStatus: "RAID1 正常",
    agentOnline: true,
    updated: "2026-09-17 19:30"
  },
  {
    id: "host-6",
    assetNo: "SRV-SZ-001",
    hostname: "sz-edge-iot-collector",
    roomId: "room-3",
    cabinetId: "cab-301",
    startU: 37,
    uHeight: 2,
    ip: "10.300.10.15",
    bmcIp: "192.168.300.15",
    brand: "华为",
    model: "TaiShan 200 (ARM 鲲鹏920)",
    os: "openEuler 22.03 LTS",
    cpu: "2× Kunpeng 920 64核 2.6GHz",
    memory: "256 GB DDR4",
    disk: "4× 3.84TB SSD",
    role: "华南物联网传感器高频时序采集",
    businessId: "biz-3",
    status: "running",
    powerWatts: 290,
    cpuTemp: 40,
    boardTemp: 30,
    inletTemp: 22,
    fanRpm: "3800 RPM (正常)",
    psu1Status: "正常 (150W)",
    psu2Status: "正常 (140W)",
    raidStatus: "RAID1 正常",
    agentOnline: true,
    updated: "2026-09-17 20:05"
  }
];

export const initialSwitches: SwitchDevice[] = [
  {
    id: "sw-1",
    assetNo: "NET-BJ-CORE01",
    name: "SW-BJ-CORE-01 核心交换机",
    roomId: "room-1",
    cabinetId: "cab-101",
    startU: 41,
    uHeight: 1,
    ip: "10.100.0.1",
    brand: "锐捷",
    model: "RG-S6510-48VS8CQ (48×25G + 8×100G)",
    role: "核心交换机",
    portCount: 56,
    activePorts: 38,
    status: "online",
    businessId: "biz-1"
  },
  {
    id: "sw-2",
    assetNo: "NET-BJ-ACC01",
    name: "SW-BJ-ACC-01 柜顶接入交换机",
    roomId: "room-1",
    cabinetId: "cab-101",
    startU: 40,
    uHeight: 1,
    ip: "10.100.0.2",
    brand: "华为",
    model: "CloudEngine 6881-48T6CQ",
    role: "接入交换机",
    portCount: 54,
    activePorts: 42,
    status: "online",
    businessId: "biz-1"
  },
  {
    id: "sw-3",
    assetNo: "NET-BJ-ACC02",
    name: "SW-BJ-ACC-02 存储网络专网交换机",
    roomId: "room-1",
    cabinetId: "cab-102",
    startU: 41,
    uHeight: 1,
    ip: "10.100.0.11",
    brand: "华为",
    model: "CloudEngine 6865-48S8CQ",
    role: "接入交换机",
    portCount: 48,
    activePorts: 28,
    status: "online",
    businessId: "biz-2"
  }
];

export const initialVms: VmHost[] = [
  {
    id: "vm-1",
    name: "vm-beikong-scada-api01",
    physicalHostId: "host-1",
    ip: "10.100.20.101",
    privateIp: "172.16.1.101",
    cpu: "8 核 vCPU",
    memory: "16 GB",
    disk: "120 GB SSD",
    os: "CentOS 7.9",
    businessId: "biz-1",
    status: "running",
    role: "水厂监控数据采集接口服务",
    updated: "2026-09-17 18:20"
  },
  {
    id: "vm-2",
    name: "vm-beikong-scada-api02",
    physicalHostId: "host-2",
    ip: "10.100.20.102",
    privateIp: "172.16.1.102",
    cpu: "8 核 vCPU",
    memory: "16 GB",
    disk: "120 GB SSD",
    os: "Ubuntu 22.04",
    businessId: "biz-1",
    status: "running",
    role: "接口负载冗余服务",
    updated: "2026-09-17 18:20"
  },
  {
    id: "vm-3",
    name: "vm-ai-inference-service",
    physicalHostId: "host-4",
    ip: "10.100.20.188",
    privateIp: "172.16.2.188",
    cpu: "16 核 vCPU + 1× A100 GPU",
    memory: "64 GB",
    disk: "500 GB NVMe",
    os: "Ubuntu 22.04",
    businessId: "biz-2",
    status: "running",
    role: "AIOps 水质异动实时推理引擎",
    updated: "2026-09-17 19:10"
  }
];

export const initialDatabases: DatabaseAsset[] = [
  {
    id: "db-1",
    name: "water_production_master",
    type: "MySQL",
    version: "8.0.32",
    hostIp: "10.100.10.31", // host-3
    port: 3306,
    arch: "主从高可用 (1主2从)",
    businessId: "biz-1",
    dataSize: "840 GB",
    connectionCount: 382,
    status: "active"
  },
  {
    id: "db-2",
    name: "realtime_telemetry_redis",
    type: "Redis",
    version: "7.0.12",
    hostIp: "10.100.10.22", // host-2
    port: 6379,
    arch: "集群分片 (3主3从)",
    businessId: "biz-1",
    dataSize: "68 GB",
    connectionCount: 1450,
    status: "active"
  },
  {
    id: "db-3",
    name: "aiops_events_timescale",
    type: "PostgreSQL",
    version: "15.3 (TimescaleDB)",
    hostIp: "10.100.10.88", // host-4
    port: 5432,
    arch: "主备高可用",
    businessId: "biz-2",
    dataSize: "1.42 TB",
    connectionCount: 94,
    status: "active"
  }
];

export const initialBusinesses: BusinessModel[] = [
  {
    id: "biz-1",
    name: "北控水务综合生产监控SCADA平台",
    code: "BIZ-BK-SCADA",
    level: "核心 L1",
    owner: "陈总监",
    department: "生产运营数字化中心",
    description: "全集团200余座水厂与污水处理厂的实时自动化监测、压力调度与加药控制枢纽系统。",
    healthScore: 99,
    hostIds: ["host-1", "host-2", "host-3", "host-5"],
    dbIds: ["db-1", "db-2"],
    switchIds: ["sw-1", "sw-2"]
  },
  {
    id: "biz-2",
    name: "AIOps水质预测与智能控制大脑",
    code: "BIZ-BK-AIOPS",
    level: "核心 L1",
    owner: "高工",
    department: "智慧水务AI实验室",
    description: "基于多传感器高频时序与大模型水质预测，实现自适应曝气量调节与能耗优化。",
    healthScore: 97,
    hostIds: ["host-4"],
    dbIds: ["db-3"],
    switchIds: ["sw-3"]
  },
  {
    id: "biz-3",
    name: "管网物联网感知与漏损预警系统",
    code: "BIZ-BK-IOT",
    level: "重要 L2",
    owner: "李经理",
    department: "管网事业部",
    description: "城市管网流量计、压力计、瞬时水锤波监测以及AI声学漏损分析网络。",
    healthScore: 95,
    hostIds: ["host-6"],
    dbIds: ["db-2"],
    switchIds: ["sw-1"]
  }
];

export const initialProbes: AgentProbe[] = [
  {
    id: "probe-1",
    hostIp: "10.100.10.21",
    hostname: "bj-prod-k8s-node01",
    agentVersion: "AutoOps-Agent v2.4.1",
    status: "online",
    cpuUsage: 28.4,
    memUsage: 64.2,
    lastHeartbeat: "2秒前"
  },
  {
    id: "probe-2",
    hostIp: "10.100.10.22",
    hostname: "bj-prod-k8s-node02",
    agentVersion: "AutoOps-Agent v2.4.1",
    status: "online",
    cpuUsage: 35.1,
    memUsage: 71.8,
    lastHeartbeat: "1秒前"
  },
  {
    id: "probe-3",
    hostIp: "10.100.10.31",
    hostname: "bj-db-mysql-master",
    agentVersion: "AutoOps-Agent v2.4.1",
    status: "online",
    cpuUsage: 42.6,
    memUsage: 82.5,
    lastHeartbeat: "3秒前"
  },
  {
    id: "probe-4",
    hostIp: "10.100.10.88",
    hostname: "bj-ai-gpu-node01",
    agentVersion: "AutoOps-Agent v2.4.1",
    status: "online",
    cpuUsage: 74.8,
    memUsage: 69.3,
    lastHeartbeat: "2秒前"
  }
];

export const initialCredentials: CredentialItem[] = [
  {
    id: "cred-1",
    name: "Linux 生产物理机通用 Ansible 密钥",
    type: "SSH 私钥",
    username: "ops_root",
    targetCount: 6,
    updated: "2026-09-10",
    remark: "全网物理主机自动化编排与合规巡检根凭据"
  },
  {
    id: "cred-2",
    name: "核心交换机管理凭证 (TACACS+)",
    type: "SSH 密码",
    username: "net_admin",
    targetCount: 3,
    updated: "2026-09-08",
    remark: "亦庄与张江核心交换机 SSH 巡检与配置备份"
  },
  {
    id: "cred-3",
    name: "带外远程管理卡 (iDRAC / iBMC) 凭据",
    type: "BMC IPMI",
    username: "idrac_admin",
    targetCount: 6,
    updated: "2026-09-01",
    remark: "物理机硬件底层监控、风扇控制与远程开机KVM"
  }
];
