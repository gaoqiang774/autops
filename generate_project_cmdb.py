import json
import os

with open(r'E:\autops\parsed_all_devices.json', 'r', encoding='utf-8') as f:
    raw_devices = json.load(f)

with open(r'E:\autops\projects_summary.json', 'r', encoding='utf-8') as f:
    projects_def = json.load(f)

# Build project map
pname_to_info = {p['name']: p for p in projects_def}

# Classify devices into: Physical Hosts, Virtual Hosts, Switches
physical_hosts = []
vms = []
switches = []

for d in raw_devices:
    seq = d.get('序号')
    name = d.get('设备名称') or f'device-{seq}'
    cat = d.get('设备大类') or '服务器'
    dtype = d.get('设备类型') or '虚拟机'
    proj_name = d.get('项目名称') or '通用项目'
    p_info = pname_to_info.get(proj_name, projects_def[0])
    proj_id = p_info['id']

    ip = d.get('私有IP（业务IP）') or d.get('内大网IP') or d.get('VIP地址') or '192.168.1.1'
    os_name = f"{d.get('OS发行版') or ''} {d.get('OS版本') or ''}".strip() or 'Linux'

    cpu_cores = d.get('CPU核数')
    cpu_str = f"{cpu_cores} 核 vCPU ({d.get('CPU架构') or 'x86_64'})" if cpu_cores else '4 核 (x86_64)'

    mem_gb = d.get('内存GB')
    mem_str = f"{mem_gb} GB" if mem_gb else '16 GB'

    sys_disk = d.get('系统盘GB') or 30
    data_disk = d.get('数据盘GB') or 0
    disk_str = f"{sys_disk}G(系统) + {data_disk}G(数据)" if data_disk else f"{sys_disk} GB"

    base_item = {
        'seq': seq,
        'customerName': d.get('客户名称') or p_info['customerName'],
        'projectName': proj_name,
        'projectId': proj_id,
        'env': d.get('环境') or p_info['env'],
        'cloudVendor': d.get('云厂商') or p_info['cloudVendor'],
        'regionName': d.get('区域名称') or p_info['regionName'],
        'category': cat,
        'deviceType': dtype,
        'privateIp': d.get('私有IP（业务IP）'),
        'privateIpv6': d.get('私有IPV6'),
        'internalWanIp': d.get('内大网IP'),
        'eip': d.get('EIP地址'),
        'vip': d.get('VIP地址'),
        'publicIp': d.get('公网IP'),
        'cpuArch': d.get('CPU架构'),
        'cpuCores': cpu_cores,
        'memoryGb': mem_gb,
        'systemDiskGb': sys_disk,
        'dataDiskGb': data_disk,
        'sharedDiskGb': d.get('共享磁盘GB'),
        'objectStorageGb': d.get('对象存储GB'),
        'osFamily': d.get('OS发行版'),
        'osVersion': d.get('OS版本'),
        'kernelVersion': d.get('内核版本'),
        'isXinchuang': d.get('是否信创OS') or '否',
        'remotePort': d.get('远程端口') or 22,
        'remarks': d.get('备注')
    }

    if dtype == '物理机' or 'minio集群' in name:
        phy_item = {
            'id': f'phy-{seq}',
            'assetNo': f'ASSET-{seq:04d}',
            'hostname': name,
            'name': name,
            'ip': ip,
            'bmcIp': f"192.168.100.{seq % 200 + 10}",
            'brand': '华为' if seq % 2 == 0 else '浪潮',
            'model': 'FusionServer Pro 2288H' if 'db3' in name or 'oracle' in name else 'NF5280M5',
            'os': os_name,
            'cpu': cpu_str,
            'memory': mem_str,
            'disk': disk_str,
            'role': '核心数据库物理服务器' if 'db' in name or 'oracle' in name else '分布式存储硬件节点',
            'businessId': proj_id,
            'status': 'running',
            'powerWatts': 380,
            'cpuTemp': 42,
            'boardTemp': 33,
            'inletTemp': 22,
            'fanRpm': '4800 RPM (正常)',
            'psu1Status': '正常 (210W)',
            'psu2Status': '正常 (170W)',
            'raidStatus': 'RAID10 正常',
            'agentOnline': True,
            'updated': '2026-09-17 19:30',
            **base_item
        }
        physical_hosts.append(phy_item)

    elif cat == '网络' or dtype == '负载均衡':
        sw_item = {
            'id': f'sw-{seq}',
            'assetNo': f'NET-{seq:04d}',
            'name': name,
            'ip': ip,
            'brand': '锐捷' if 'VIP' in name else '华为',
            'model': 'RG-EG3000G / 高可用SLB' if 'VIP' in name else 'CloudEngine 6857',
            'role': '项目专有负载均衡' if 'VIP' in name or '公网' in name else '项目接入网络设备',
            'portCount': 48,
            'activePorts': 32,
            'status': 'online',
            'businessId': proj_id,
            **base_item
        }
        switches.append(sw_item)

    else:
        vm_item = {
            'id': f'vm-{seq}',
            'name': name,
            'physicalHostId': 'phy-1',
            'ip': ip,
            'privateIp': d.get('私有IP（业务IP）') or ip,
            'cpu': cpu_str,
            'memory': mem_str,
            'disk': disk_str,
            'os': os_name,
            'businessId': proj_id,
            'status': 'running',
            'role': '应用微服务节点' if '应用' in name or '服务' in name else '业务数据库节点' if '库' in name or 'db' in name.lower() else '通用计算实例',
            'updated': '2026-09-17 19:35',
            **base_item
        }
        vms.append(vm_item)

# Associate database definitions with projects
databases_def = [
    {
        'id': 'db-1',
        'name': '原三险生产核心库 (Oracle RAC)',
        'type': 'Oracle',
        'version': 'Oracle 11g R2 Enterprise',
        'hostIp': '192.123.74.70',
        'port': 1521,
        'arch': '主备高可用',
        'businessId': next((p['id'] for p in projects_def if '原三险' in p['name']), 'prj-008'),
        'projectName': '原三险系统',
        'dataSize': '2.8 TB',
        'connectionCount': 1250,
        'status': 'active'
    },
    {
        'id': 'db-2',
        'name': '仲裁核心生产数据库1 (Primary)',
        'type': 'Oracle',
        'version': 'Oracle 19c Enterprise',
        'hostIp': '192.123.74.36',
        'port': 1521,
        'arch': '主备高可用',
        'businessId': next((p['id'] for p in projects_def if '仲裁核心' in p['name']), 'prj-003'),
        'projectName': '仲裁核心系统',
        'dataSize': '1.6 TB',
        'connectionCount': 860,
        'status': 'active'
    },
    {
        'id': 'db-3',
        'name': '仲裁核心生产数据库2 (Standby)',
        'type': 'Oracle',
        'version': 'Oracle 19c Enterprise',
        'hostIp': '192.123.74.37',
        'port': 1521,
        'arch': '主备高可用',
        'businessId': next((p['id'] for p in projects_def if '仲裁核心' in p['name']), 'prj-003'),
        'projectName': '仲裁核心系统',
        'dataSize': '1.6 TB',
        'connectionCount': 420,
        'status': 'standby'
    },
    {
        'id': 'db-4',
        'name': '调解仲裁 Oracle RAC 实例-01',
        'type': 'Oracle',
        'version': 'Oracle 19c RAC',
        'hostIp': '192.142.71.201',
        'port': 1521,
        'arch': '主从高可用 (1主2从)',
        'businessId': next((p['id'] for p in projects_def if '调解仲裁' in p['name']), 'prj-005'),
        'projectName': '调解仲裁系统',
        'dataSize': '950 GB',
        'connectionCount': 630,
        'status': 'active'
    },
    {
        'id': 'db-5',
        'name': '工会职服数智化 MySQL 生产主库',
        'type': 'MySQL',
        'version': 'MySQL 8.0.32 GA',
        'hostIp': '172.25.147.247',
        'port': 3306,
        'arch': '主从高可用 (1主2从)',
        'businessId': next((p['id'] for p in projects_def if '工会职服' in p['name']), 'prj-004'),
        'projectName': '工会职服数智化系统',
        'dataSize': '680 GB',
        'connectionCount': 520,
        'status': 'active'
    },
    {
        'id': 'db-6',
        'name': '智科精益平台 PostgreSQL 实例',
        'type': 'PostgreSQL',
        'version': 'PostgreSQL 14.5',
        'hostIp': '69.11.30.2',
        'port': 5432,
        'arch': '主从高可用 (1主2从)',
        'businessId': next((p['id'] for p in projects_def if '智科精益' in p['name']), 'prj-009'),
        'projectName': '智科精益管理平台',
        'dataSize': '350 GB',
        'connectionCount': 280,
        'status': 'active'
    },
    {
        'id': 'db-7',
        'name': '税务年金征收系统专用库',
        'type': 'Oracle',
        'version': 'Oracle 12c',
        'hostIp': '192.125.31.240',
        'port': 1521,
        'arch': '单机测试实例',
        'businessId': next((p['id'] for p in projects_def if '税务年金' in p['name']), 'prj-011'),
        'projectName': '税务年金征收系统',
        'dataSize': '420 GB',
        'connectionCount': 190,
        'status': 'active'
    }
]

# Business models mirroring projects
businesses_def = []
for p in projects_def:
    pid = p['id']
    devs_in_proj = [d.get('序号') for d in raw_devices if (d.get('项目名称') or '通用项目') == p['name']]
    dbs_in_proj = [d['id'] for d in databases_def if d['businessId'] == pid]
    sws_in_proj = [s['id'] for s in switches if s.get('projectId') == pid]

    businesses_def.append({
        'id': pid,
        'name': p['name'],
        'code': p['code'],
        'level': '核心 L1' if '核心' in p['name'] or '三险' in p['name'] or '年金' in p['name'] else '重要 L2' if '数智' in p['name'] or '监管' in p['name'] or '庭审' in p['name'] else '通用 L3',
        'owner': '北控伟仕运维保障组',
        'department': p['customerName'],
        'description': p['description'],
        'healthScore': p['healthScore'],
        'hostIds': [f'vm-{s}' for s in devs_in_proj],
        'dbIds': dbs_in_proj,
        'switchIds': sws_in_proj
    })

print(f'Projects: {len(projects_def)}, Physical: {len(physical_hosts)}, VMs: {len(vms)}, Switches: {len(switches)}')

ts_code = f'''// CMDB Core Relational Data Model & Seed Store
// Re-architected strictly around Projects (按项目分资产)
// Sourced from 《信息资产台账-v331toAI.xlsx》

export interface ProjectGroup {{
  id: string;
  name: string;
  code: string;
  customerName: string;
  env: string;
  cloudVendor: string;
  regionName: string;
  deviceCount: number;
  phyCount: number;
  vmCount: number;
  netCount: number;
  totalCores: number;
  totalMemoryGb: number;
  totalDiskGb: number;
  xinchuangCount: number;
  description: string;
  healthScore: number;
}}

// Standardized Asset Interface encompassing ALL Excel Fields
export interface AssetMeta {{
  seq?: number;
  customerName?: string | null;
  projectName?: string | null;
  projectId?: string | null;
  env?: string | null;
  cloudVendor?: string | null;
  regionName?: string | null;
  category?: string | null;
  deviceType?: string | null;
  privateIp?: string | null;
  privateIpv6?: string | null;
  internalWanIp?: string | null;
  eip?: string | null;
  vip?: string | null;
  publicIp?: string | null;
  cpuArch?: string | null;
  cpuCores?: number | null;
  memoryGb?: number | null;
  systemDiskGb?: number | null;
  dataDiskGb?: number | null;
  sharedDiskGb?: number | null;
  objectStorageGb?: number | null;
  osFamily?: string | null;
  osVersion?: string | null;
  kernelVersion?: string | null;
  isXinchuang?: string | null;
  remotePort?: number | null;
  remarks?: string | null;
}}

export interface PhysicalHost extends AssetMeta {{
  id: string;
  assetNo: string;
  hostname: string;
  name: string;
  ip: string;
  bmcIp: string;
  brand: string;
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
  psu1Status: string;
  psu2Status: string;
  raidStatus: string;
  agentOnline: boolean;
  updated: string;
}}

export interface VmHost extends AssetMeta {{
  id: string;
  name: string;
  physicalHostId: string;
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
}}

export interface SwitchDevice extends AssetMeta {{
  id: string;
  assetNo: string;
  name: string;
  ip: string;
  brand: string;
  model: string;
  role: string;
  portCount: number;
  activePorts: number;
  status: "online" | "warning" | "offline";
  businessId: string;
}}

export interface DatabaseAsset {{
  id: string;
  name: string;
  type: "MySQL" | "Redis" | "PostgreSQL" | "MongoDB" | "Oracle";
  version: string;
  hostIp: string;
  port: number;
  arch: string;
  businessId: string;
  projectName?: string;
  dataSize: string;
  connectionCount: number;
  status: "active" | "standby" | "maintenance";
}}

export interface BusinessModel {{
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
}}

export interface AgentProbe {{
  id: string;
  hostIp: string;
  hostname: string;
  agentVersion: string;
  status: "online" | "offline";
  cpuUsage: number;
  memUsage: number;
  lastHeartbeat: string;
}}

export interface CredentialItem {{
  id: string;
  name: string;
  type: "SSH 密码" | "SSH 私钥" | "BMC IPMI" | "SNMP v2c/v3" | "数据库账号";
  username: string;
  targetCount: number;
  updated: string;
  remark: string;
}}

// 24 Real Projects extracted from 《信息资产台账-v331toAI.xlsx》
export const initialProjects: ProjectGroup[] = {json.dumps(projects_def, ensure_ascii=False, indent=2)};

export const initialHosts: PhysicalHost[] = {json.dumps(physical_hosts, ensure_ascii=False, indent=2)};

export const initialVms: VmHost[] = {json.dumps(vms, ensure_ascii=False, indent=2)};

export const initialSwitches: SwitchDevice[] = {json.dumps(switches, ensure_ascii=False, indent=2)};

export const initialDatabases: DatabaseAsset[] = {json.dumps(databases_def, ensure_ascii=False, indent=2)};

export const initialBusinesses: BusinessModel[] = {json.dumps(businesses_def, ensure_ascii=False, indent=2)};

export const initialProbes: AgentProbe[] = [
  {{
    id: "probe-1",
    hostIp: "192.123.74.70",
    hostname: "数据库db3 (原三险核心)",
    agentVersion: "v2.8.4-release",
    status: "online",
    cpuUsage: 18.2,
    memUsage: 64.5,
    lastHeartbeat: "刚刚 (1s前)"
  }},
  {{
    id: "probe-2",
    hostIp: "192.123.74.36",
    hostname: "仲裁数据库1 (仲裁核心)",
    agentVersion: "v2.8.4-release",
    status: "online",
    cpuUsage: 24.1,
    memUsage: 58.2,
    lastHeartbeat: "刚刚 (3s前)"
  }},
  {{
    id: "probe-3",
    hostIp: "192.142.71.201",
    hostname: "oracle宿1 (调解仲裁)",
    agentVersion: "v2.8.4-release",
    status: "online",
    cpuUsage: 12.8,
    memUsage: 45.0,
    lastHeartbeat: "刚刚 (2s前)"
  }},
  {{
    id: "probe-4",
    hostIp: "69.11.30.2",
    hostname: "智科精益平台_minio集群_01",
    agentVersion: "v2.8.4-release",
    status: "online",
    cpuUsage: 8.5,
    memUsage: 32.1,
    lastHeartbeat: "刚刚 (4s前)"
  }}
];

export const initialCredentials: CredentialItem[] = [
  {{
    id: "cred-1",
    name: "政务项目 Linux 主机 Root 密钥",
    type: "SSH 私钥",
    username: "root",
    targetCount: {len(vms) + len(physical_hosts)},
    updated: "2026-09-17 18:00",
    remark: "各项目堡垒机统一集中纳管密钥"
  }},
  {{
    id: "cred-2",
    name: "BMC IPMI 带外运维管理密码",
    type: "BMC IPMI",
    username: "Administrator",
    targetCount: {len(physical_hosts)},
    updated: "2026-09-17 18:00",
    remark: "物理机 iLO / iBMC 远程硬件管理密码"
  }},
  {{
    id: "cred-3",
    name: "项目网络与 SLB 负载 SNMP 凭证",
    type: "SNMP v2c/v3",
    username: "snmp_ro_user",
    targetCount: {len(switches)},
    updated: "2026-09-17 18:00",
    remark: "网络设备流量与端口遥测凭证"
  }},
  {{
    id: "cred-4",
    name: "Oracle RAC 生产数据库运维账号",
    type: "数据库账号",
    username: "ops_monitor",
    targetCount: {len(databases_def)},
    updated: "2026-09-17 18:00",
    remark: "数据库只读性能监控与慢日志分析专用"
  }}
];

// Compatibility types for IDC references (dummy if needed)
export interface IdcRoom {{
  id: string;
  name: string;
  code: string;
  operator?: string;
  city?: string;
  address?: string;
  level?: string;
  cabinetCount?: number;
  contact?: string;
  phone?: string;
  status?: string;
  remark?: string;
}}

export interface IdcCabinet {{
  id: string;
  roomId: string;
  name: string;
  code: string;
  totalU: number;
  usedU: number;
  maxPower: number;
  currentPower: number;
  temperature: number;
  status: string;
  manager: string;
  row: string;
  remark: string;
}}

export const initialRooms: IdcRoom[] = [];
export const initialCabinets: IdcCabinet[] = [];
'''

with open(r'E:\autops\app\cmdbData.ts', 'w', encoding='utf-8') as f:
    f.write(ts_code)

print('app/cmdbData.ts re-generated with Project architecture!')
