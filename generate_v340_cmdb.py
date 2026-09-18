import openpyxl
import json
import re
from collections import defaultdict

# 1. Load Excel
excel_path = r'e:\autops\信息资产台账-v340.xlsx'
wb = openpyxl.load_workbook(excel_path, data_only=True)
ws = wb['02-硬件设备']
rows = list(ws.iter_rows(values_only=True))

headers = [str(c).strip() if c is not None else f"col_{i}" for i, c in enumerate(rows[3])]

raw_devices = []
for idx, r in enumerate(rows[4:]):
    if not any(r):
        continue
    item = {}
    for i, h in enumerate(headers):
        val = r[i] if i < len(r) else None
        item[h] = val
    if item.get("设备名称") or item.get("私有IP（业务IP）"):
        item["_row"] = idx + 5
        raw_devices.append(item)

print(f"Loaded {len(raw_devices)} raw devices from 02-硬件设备")

# 2. Project Grouping & Normalization
# Normalize project naming variants while keeping exact original on asset
def normalize_project(name):
    if not name:
        return "未分类资产"
    name = name.strip()
    if "监管中台" in name or "工伤红名单" in name or "即席查询" in name or name == "数转区":
        return "监管中台与工伤即席查询"
    if "伟仕精益" in name or "天兴工程" in name:
        return "伟仕精益管理平台与天兴工程"
    return name

project_groups_map = defaultdict(list)
for d in raw_devices:
    norm_pname = normalize_project(d.get("项目名称"))
    project_groups_map[norm_pname].append(d)

# Rooms definition
rooms_def = [
    {
        'id': 'room-1',
        'name': '六里桥机房',
        'code': 'BJ-LLQ-01',
        'operator': '中国联通',
        'city': '北京',
        'address': '北京市丰台区六里桥联通政务外网核心节点',
        'level': 'T3+',
        'cabinetCount': 24,
        'contact': '李运维',
        'phone': '13810010001',
        'status': '正常运行',
        'remark': '联通云政务外网主生产机房，承载原三险、仲裁核心系统、仲裁云庭审、原城居、原工伤认定等政务核心业务。'
    },
    {
        'id': 'room-2',
        'name': '通州C1机房',
        'code': 'BJ-TZ-02',
        'operator': 'BGP多线',
        'city': '北京',
        'address': '北京市通州区城市副中心C1政务生产机房',
        'level': 'T4',
        'cabinetCount': 16,
        'contact': '张工',
        'phone': '13810010002',
        'status': '正常运行',
        'remark': '副中心高等级政务机房，承载调解仲裁系统 Oracle RAC 物理集群及退军资金企转系统。'
    },
    {
        'id': 'room-3',
        'name': '首信机房',
        'code': 'BJ-SX-03',
        'operator': 'BGP多线',
        'city': '北京',
        'address': '北京市东城区首信云互联网与政务数据中心',
        'level': 'T3+',
        'cabinetCount': 20,
        'contact': '王工',
        'phone': '13810010003',
        'status': '正常运行',
        'remark': '首信云数据中心，承载工会互助保险、工会职服数智化系统及高可用 VIP 负载均衡集群。'
    },
    {
        'id': 'room-4',
        'name': '亦庄国企云机房',
        'code': 'BJ-YZ-04',
        'operator': '中国电信',
        'city': '北京',
        'address': '北京市经开区亦庄国企云专属机房',
        'level': 'T3+',
        'cabinetCount': 28,
        'contact': '陈工',
        'phone': '13810010004',
        'status': '正常运行',
        'remark': '北京市国企云核心节点，承载北控集团商城、招采、北控环境、智科、数科、北斗、燃气内管及 MinIO 存储集群。'
    },
    {
        'id': 'room-5',
        'name': '太极云机房',
        'code': 'BJ-TJ-05',
        'operator': 'BGP多线',
        'city': '北京',
        'address': '北京市海淀区太极政务云数据中心',
        'level': 'T3',
        'cabinetCount': 12,
        'contact': '刘工',
        'phone': '13810010005',
        'status': '正常运行',
        'remark': '太极政务云，承载北京市退役军人事务局补贴发放系统。'
    },
    {
        'id': 'room-6',
        'name': '税务机房',
        'code': 'BJ-SW-06',
        'operator': 'BGP多线',
        'city': '北京',
        'address': '北京市西城区国家税务总局北京市税务局机房',
        'level': 'T4',
        'cabinetCount': 12,
        'contact': '赵工',
        'phone': '13810010006',
        'status': '正常运行',
        'remark': '税务专网专用数据中心，承载税务年金征收系统核心节点。'
    },
    {
        'id': 'room-7',
        'name': '酒仙桥机房',
        'code': 'BJ-JXQ-07',
        'operator': '中国移动',
        'city': '北京',
        'address': '北京市朝阳区酒仙桥容灾及测试数据中心',
        'level': 'T3',
        'cabinetCount': 14,
        'contact': '高工',
        'phone': '13810010007',
        'status': '正常运行',
        'remark': '同城容灾及业务测试机房，承载仲裁网厅系统、监管中台、工伤红名单、即席查询等应用。'
    },
    {
        'id': 'room-8',
        'name': '阿里云机房',
        'code': 'BJ-ALI-08',
        'operator': 'BGP多线',
        'city': '北京',
        'address': '阿里云华北互联网及公共云服务节点',
        'level': 'T3+',
        'cabinetCount': 10,
        'contact': '孙工',
        'phone': '13810010008',
        'status': '正常运行',
        'remark': '阿里云华北节点，承载互联网区公共服务、伟仕精益管理平台、天兴工程及 OSS 云对象存储。'
    }
]

room_name_to_id = {r['name']: r['id'] for r in rooms_def}

# Cabinets
cabinets_def = [
    {'id': 'cab-llq-01', 'roomId': 'room-1', 'name': 'A01 网络与负载柜', 'code': 'LLQ-A01', 'totalU': 42, 'usedU': 8, 'maxPower': 5.0, 'currentPower': 2.1, 'temperature': 21.5, 'status': '正常', 'manager': '李运维', 'row': 'Row A', 'remark': '政务外网核心交换与汇聚负载柜'},
    {'id': 'cab-llq-02', 'roomId': 'room-1', 'name': 'A02 三险生产计算柜', 'code': 'LLQ-A02', 'totalU': 42, 'usedU': 22, 'maxPower': 5.5, 'currentPower': 3.2, 'temperature': 22.8, 'status': '正常', 'manager': '李运维', 'row': 'Row A', 'remark': '人社局原三险系统业务应用计算柜'},
    {'id': 'cab-llq-03', 'roomId': 'room-1', 'name': 'B01 核心数据库物理机柜', 'code': 'LLQ-B01', 'totalU': 42, 'usedU': 28, 'maxPower': 6.0, 'currentPower': 4.1, 'temperature': 23.5, 'status': '预警', 'manager': '李运维', 'row': 'Row B', 'remark': '原三险 db3、仲裁数据库 1/2 高性能物理宿主柜'},
    {'id': 'cab-llq-04', 'roomId': 'room-1', 'name': 'B02 仲裁核心计算柜', 'code': 'LLQ-B02', 'totalU': 42, 'usedU': 16, 'maxPower': 5.0, 'currentPower': 2.4, 'temperature': 22.1, 'status': '正常', 'manager': '李运维', 'row': 'Row B', 'remark': '仲裁核心系统、仲裁云庭审业务主机柜'},
    {'id': 'cab-tz-01', 'roomId': 'room-2', 'name': 'A01 核心接入与交换柜', 'code': 'TZ-A01', 'totalU': 42, 'usedU': 10, 'maxPower': 5.0, 'currentPower': 1.8, 'temperature': 21.0, 'status': '正常', 'manager': '张工', 'row': 'Row A', 'remark': '城市副中心 C1 接入交换与带外网络柜'},
    {'id': 'cab-tz-02', 'roomId': 'room-2', 'name': 'A02 Oracle RAC 物理宿主柜', 'code': 'TZ-A02', 'totalU': 42, 'usedU': 24, 'maxPower': 6.0, 'currentPower': 3.8, 'temperature': 23.0, 'status': '正常', 'manager': '张工', 'row': 'Row A', 'remark': '承载 oracle宿1、oracle宿2 调解仲裁核心物理集群'},
    {'id': 'cab-sx-01', 'roomId': 'room-3', 'name': 'A01 互联网与负载均衡柜', 'code': 'SX-A01', 'totalU': 42, 'usedU': 18, 'maxPower': 5.0, 'currentPower': 2.6, 'temperature': 22.0, 'status': '正常', 'manager': '王工', 'row': 'Row A', 'remark': '承载总工会高可用 VIP 负载均衡及域名接入设备'},
    {'id': 'cab-sx-02', 'roomId': 'room-3', 'name': 'A02 工会职服 K8s 计算柜', 'code': 'SX-A02', 'totalU': 42, 'usedU': 26, 'maxPower': 5.5, 'currentPower': 3.5, 'temperature': 22.7, 'status': '正常', 'manager': '王工', 'row': 'Row A', 'remark': '工会职服数智化与互助保险容器集群宿主机柜'},
    {'id': 'cab-yz-01', 'roomId': 'room-4', 'name': 'A01 国企云计算服务柜', 'code': 'YZ-A01', 'totalU': 42, 'usedU': 20, 'maxPower': 5.0, 'currentPower': 2.9, 'temperature': 22.3, 'status': '正常', 'manager': '陈工', 'row': 'Row A', 'remark': '北控商城、招采、智科、数科精益管理平台服务柜'},
    {'id': 'cab-yz-02', 'roomId': 'room-4', 'name': 'A02 MinIO 分布式存储集群柜', 'code': 'YZ-A02', 'totalU': 42, 'usedU': 16, 'maxPower': 5.5, 'currentPower': 2.7, 'temperature': 21.9, 'status': '正常', 'manager': '陈工', 'row': 'Row A', 'remark': '智科分布式对象存储 4 节点集群专用机柜'},
    {'id': 'cab-tj-01', 'roomId': 'room-5', 'name': 'A01 退军业务服务柜', 'code': 'TJ-A01', 'totalU': 42, 'usedU': 12, 'maxPower': 4.5, 'currentPower': 1.9, 'temperature': 22.0, 'status': '正常', 'manager': '刘工', 'row': 'Row A', 'remark': '退役军人事务局企转与补贴发放业务柜'},
    {'id': 'cab-sw-01', 'roomId': 'room-6', 'name': 'A01 税务专网年金服务柜', 'code': 'SW-A01', 'totalU': 42, 'usedU': 14, 'maxPower': 5.0, 'currentPower': 2.2, 'temperature': 21.8, 'status': '正常', 'manager': '赵工', 'row': 'Row A', 'remark': '税务年金征收系统计算与数据库服务器柜'},
    {'id': 'cab-jxq-01', 'roomId': 'room-7', 'name': 'A01 容灾与工伤服务柜', 'code': 'JXQ-A01', 'totalU': 42, 'usedU': 16, 'maxPower': 5.0, 'currentPower': 2.3, 'temperature': 22.4, 'status': '正常', 'manager': '高工', 'row': 'Row A', 'remark': '仲裁网厅、监管中台、工伤认定容灾备用服务器柜'},
    {'id': 'cab-ali-01', 'roomId': 'room-8', 'name': 'A01 阿里云虚拟数据中心柜', 'code': 'ALI-A01', 'totalU': 42, 'usedU': 10, 'maxPower': 4.5, 'currentPower': 1.5, 'temperature': 21.2, 'status': '正常', 'manager': '孙工', 'row': 'Row A', 'remark': '阿里云华北节点公网网关与精益管理云主机柜'}
]

# VPN Gateways mapping by Project
vpn_mapping = {
    "六里桥机房": {"gw": "vpn.llq-gov.beikong.com", "client": "深信服 EasyConnect (政务专网)", "seg": "192.125.31.0/24, 192.123.0.0/16"},
    "通州C1机房": {"gw": "vpn.tz-c1.beikong.com", "client": "Array AG SSL-VPN", "seg": "192.142.71.0/24, 192.141.0.0/16"},
    "首信机房": {"gw": "vpn.sx-cloud.beikong.com", "client": "首信云客户端 / WireGuard", "seg": "172.25.147.0/24, 172.26.0.0/16"},
    "亦庄国企云机房": {"gw": "vpn.yz-gqy.beikong.com", "client": "天翼云 VPN 客户端", "seg": "170.11.30.0/24, 10.100.0.0/16"},
    "太极云机房": {"gw": "vpn.tj-gov.beikong.com", "client": "太极政务专网安全拨号助手", "seg": "192.168.10.0/24"},
    "税务机房": {"gw": "vpn.sw-tax.beikong.com", "client": "国家税务总局专用客户端 (国密SM2/SM3)", "seg": "192.125.31.240/28"},
    "酒仙桥机房": {"gw": "vpn.jxq-disaster.beikong.com", "client": "联通云专网安全网关", "seg": "192.128.24.0/22"},
    "阿里云机房": {"gw": "vpn.ali-cloud.beikong.com", "client": "阿里云 SSL-VPN 客户端", "seg": "10.0.0.0/24"}
}

# 3. Create Projects
projects_list = []
proj_id_map = {}
# Sort projects by device count descending
sorted_pnames = sorted(project_groups_map.keys(), key=lambda k: -len(project_groups_map[k]))

for p_idx, pname in enumerate(sorted_pnames, 1):
    devs = project_groups_map[pname]
    pid = f"prj-{p_idx:03d}"
    proj_id_map[pname] = pid

    cust = devs[0].get("客户名称") or "北控伟仕保障客户"
    env = devs[0].get("环境") or "生产"
    cloud = devs[0].get("云厂商") or "联通云"
    region = devs[0].get("区域名称") or "政务外网区"

    phy_count = sum(1 for d in devs if d.get("设备类型") == "物理机" or d.get("设备大类") == "物理机" or "minio集群" in str(d.get("设备名称") or "").lower())
    net_count = sum(1 for d in devs if d.get("设备大类") == "网络" or d.get("设备类型") == "负载均衡")
    vm_count = len(devs) - phy_count - net_count

    total_cores = sum(int(d.get("CPU核数")) for d in devs if d.get("CPU核数") and str(d.get("CPU核数")).isdigit())
    total_mem = sum(int(d.get("内存GB")) for d in devs if d.get("内存GB") and str(d.get("内存GB")).isdigit())
    total_disk = sum(
        (int(d.get("系统盘GB")) if d.get("系统盘GB") and str(d.get("系统盘GB")).isdigit() else 0) +
        (int(d.get("数据盘GB")) if d.get("数据盘GB") and str(d.get("数据盘GB")).isdigit() else 0)
        for d in devs
    )
    xinchuang_count = sum(1 for d in devs if d.get("是否信创OS") == "是" or "麒麟" in str(d.get("OS发行版") or "") or "统信" in str(d.get("OS发行版") or ""))

    # Health score
    health_score = 98 if "核心" in pname or "三险" in pname or "年金" in pname else 96

    projects_list.append({
        "id": pid,
        "name": pname,
        "code": f"PRJ-{p_idx:03d}",
        "customerName": cust,
        "env": env,
        "cloudVendor": cloud,
        "regionName": region,
        "deviceCount": len(devs),
        "phyCount": phy_count,
        "vmCount": vm_count,
        "netCount": net_count,
        "totalCores": total_cores,
        "totalMemoryGb": total_mem,
        "totalDiskGb": total_disk,
        "xinchuangCount": xinchuang_count,
        "description": f"{cust}承建之「{pname}」，台账纳管资产 {len(devs)} 台（信创 {xinchuang_count} 台）。",
        "healthScore": health_score
    })

print(f"Built {len(projects_list)} projects")

# 4. Device Entities
physical_hosts = []
vms = []
switches = []
software_components = []
ops_channels = []

soft_counter = 1
chan_counter = 1

for d in raw_devices:
    seq = d.get("序号") or (raw_devices.index(d) + 1)
    name = d.get("设备名称") or f"device-{seq}"
    cat = d.get("设备大类") or "服务器"
    dtype = d.get("设备类型") or "虚拟机"
    room_name = d.get("机房") or "六里桥机房"
    room_id = room_name_to_id.get(room_name, "room-1")
    raw_pname = d.get("项目名称") or "未分类"
    norm_pname = normalize_project(raw_pname)
    pid = proj_id_map.get(norm_pname, "prj-001")

    ip = d.get("私有IP（业务IP）") or d.get("内大网IP") or d.get("VIP地址") or d.get("公网IP")
    os_name = f"{d.get('OS发行版') or ''} {d.get('OS版本') or ''}".strip() or "Linux"
    cpu_cores = int(d.get("CPU核数")) if d.get("CPU核数") and str(d.get("CPU核数")).isdigit() else 4
    cpu_str = f"{cpu_cores} 核 ({d.get('CPU架构') or 'x86_64'})"
    mem_gb = int(d.get("内存GB")) if d.get("内存GB") and str(d.get("内存GB")).isdigit() else 16
    mem_str = f"{mem_gb} GB"
    sys_disk = int(d.get("系统盘GB")) if d.get("系统盘GB") and str(d.get("系统盘GB")).isdigit() else 30
    data_disk = int(d.get("数据盘GB")) if d.get("数据盘GB") and str(d.get("数据盘GB")).isdigit() else 0
    disk_str = f"{sys_disk}G(系统) + {data_disk}G(数据)" if data_disk else f"{sys_disk} GB"

    remote_port = int(d.get("远程端口")) if d.get("远程端口") and str(d.get("远程端口")).isdigit() else 22
    remarks = d.get("备注") or ""

    is_minio_node = "minio集群" in name.lower() or "minio_0" in name.lower()
    is_phy = (dtype == "物理机" or cat == "物理机" or is_minio_node)

    # Asset Common Metadata
    common_meta = {
        'seq': seq,
        'customerName': d.get('客户名称'),
        'projectName': norm_pname,
        'projectId': pid,
        'env': d.get('环境'),
        'cloudVendor': d.get('云厂商'),
        'regionName': d.get('区域名称'),
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
        'isXinchuang': d.get('是否信创OS') or "否",
        'remotePort': remote_port,
        'remarks': remarks
    }

    if is_phy:
        # Determine physical rack
        if 'db3' in name:
            cab_id, start_u, u_h = 'cab-llq-03', 1, 4
        elif '仲裁数据库1' in name:
            cab_id, start_u, u_h = 'cab-llq-03', 6, 4
        elif '仲裁数据库2' in name:
            cab_id, start_u, u_h = 'cab-llq-03', 11, 4
        elif 'oracle宿1' in name:
            cab_id, start_u, u_h = 'cab-tz-02', 1, 4
        elif 'oracle宿2' in name:
            cab_id, start_u, u_h = 'cab-tz-02', 6, 4
        elif '01' in name:
            cab_id, start_u, u_h = 'cab-yz-02', 1, 2
        elif '02' in name:
            cab_id, start_u, u_h = 'cab-yz-02', 4, 2
        elif '03' in name:
            cab_id, start_u, u_h = 'cab-yz-02', 7, 2
        elif '04' in name:
            cab_id, start_u, u_h = 'cab-yz-02', 10, 2
        else:
            cab_id, start_u, u_h = 'cab-llq-02', 1, 2

        asset_id = f"phy-{seq}"
        phy_item = {
            'id': asset_id,
            'assetNo': f'ASSET-{seq:04d}',
            'hostname': name,
            'name': name,
            'ip': ip or '192.168.1.1',
            'bmcIp': f"192.168.100.{seq % 200 + 10}",
            'brand': '华为' if seq % 2 == 0 else '浪潮',
            'model': 'FusionServer Pro 2288H V5' if 'db' in name or 'oracle' in name else 'NF5280M5',
            'os': os_name,
            'cpu': cpu_str,
            'memory': mem_str,
            'disk': disk_str,
            'role': '核心数据库物理机' if 'db' in name or 'oracle' in name else '分布式存储节点',
            'businessId': pid,
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
            'updated': '2026-09-18 10:00',
            **common_meta
        }
        physical_hosts.append(phy_item)

    elif cat == "网络" or dtype == "负载均衡":
        asset_id = f"sw-{seq}"
        cab_id = 'cab-sx-01' if room_id == 'room-3' else 'cab-llq-01'
        u_idx = 10 + (seq % 15)
        sw_item = {
            'id': asset_id,
            'assetNo': f'NET-{seq:04d}',
            'name': name,
            'roomId': room_id,
            'roomName': room_name,
            'cabinetId': cab_id,
            'startU': u_idx,
            'uHeight': 1,
            'ip': ip or '172.25.147.247',
            'brand': '锐捷' if 'VIP' in name else '华为',
            'model': 'RG-EG3000G / 高可用SLB' if 'VIP' in name else 'CloudEngine 6857',
            'role': '负载均衡设备' if 'VIP' in name or '公网' in name else '核心交换机',
            'portCount': 48,
            'activePorts': 32,
            'status': 'online',
            'businessId': pid,
            **common_meta
        }
        switches.append(sw_item)

    else:
        # Virtual Machine
        asset_id = f"vm-{seq}"
        phy_id = 'phy-165' if '仲裁' in norm_pname and '通州' in room_name else 'phy-14' if '三险' in norm_pname else 'phy-49' if '仲裁' in norm_pname else 'phy-208'
        vm_item = {
            'id': asset_id,
            'name': name,
            'physicalHostId': phy_id,
            'ip': ip or '',
            'privateIp': d.get('私有IP（业务IP）') or ip,
            'cpu': cpu_str,
            'memory': mem_str,
            'disk': disk_str,
            'os': os_name,
            'businessId': pid,
            'status': 'running',
            'role': '数据库节点' if ('库' in name or 'db' in name.lower() or 'oracle' in name.lower()) else '应用微服务节点' if ('应用' in name or '服务' in name or 'app' in name.lower()) else '通用计算节点',
            'updated': '2026-09-18 10:00',
            **common_meta
        }
        vms.append(vm_item)

    # ================= Generate Software Components =================
    # Analyze device name, remarks, and role to assign realistic software
    text = f"{name} {remarks}".lower()
    ip_for_soft = d.get('私有IP（业务IP）') or ip or "127.0.0.1"

    # OS Runtime (All compute nodes)
    if cat != "网络":
        software_components.append({
            "id": f"soft-{soft_counter}",
            "assetId": asset_id,
            "assetName": name,
            "assetIp": ip_for_soft,
            "projectId": pid,
            "projectName": norm_pname,
            "category": "plugin",
            "name": "OpenJDK Runtime",
            "version": "1.8.0_392 (LTS)",
            "port": "",
            "installPath": "/usr/lib/jvm/java-1.8.0-openjdk",
            "configPath": "/etc/profile.d/java.sh",
            "status": "running",
            "remarks": "业务基础 Java 虚拟机运行环境"
        })
        soft_counter += 1

    # Database detection
    if "oracle" in text or "db3" in text or "旧库" in text:
        software_components.append({
            "id": f"soft-{soft_counter}",
            "assetId": asset_id,
            "assetName": name,
            "assetIp": ip_for_soft,
            "projectId": pid,
            "projectName": norm_pname,
            "category": "database",
            "name": "Oracle Database",
            "version": "19c RAC / 11g R2",
            "port": "1521",
            "installPath": "/u01/app/oracle/product/19.3.0/dbhome_1",
            "configPath": "/u01/app/oracle/network/admin/listener.ora",
            "status": "running",
            "remarks": "核心业务关系型数据库生产主实例"
        })
        soft_counter += 1
    elif "mysql" in text or "数智" in text or "商城" in text or "招采" in text or "数据库" in text:
        software_components.append({
            "id": f"soft-{soft_counter}",
            "assetId": asset_id,
            "assetName": name,
            "assetIp": ip_for_soft,
            "projectId": pid,
            "projectName": norm_pname,
            "category": "database",
            "name": "MySQL Enterprise",
            "version": "8.0.35 GA",
            "port": "3306",
            "installPath": "/usr/local/mysql",
            "configPath": "/etc/my.cnf",
            "status": "running",
            "remarks": "高性能事务数据库实例 (InnoDB引擎)"
        })
        soft_counter += 1

    if "redis" in text or "缓存" in text or "app" in text or "应用" in text or "商城" in text or "庭审" in text:
        software_components.append({
            "id": f"soft-{soft_counter}",
            "assetId": asset_id,
            "assetName": name,
            "assetIp": ip_for_soft,
            "projectId": pid,
            "projectName": norm_pname,
            "category": "database",
            "name": "Redis In-Memory Store",
            "version": "7.0.12",
            "port": "6379",
            "installPath": "/usr/local/redis",
            "configPath": "/etc/redis/redis.conf",
            "status": "running",
            "remarks": "分布式高速数据缓存与会话共享中间件"
        })
        soft_counter += 1

    # Middleware
    if "nginx" in text or "负载" in text or "网关" in text or "前端" in text or "web" in text:
        software_components.append({
            "id": f"soft-{soft_counter}",
            "assetId": asset_id,
            "assetName": name,
            "assetIp": ip_for_soft,
            "projectId": pid,
            "projectName": norm_pname,
            "category": "web_server",
            "name": "Nginx Web Server",
            "version": "1.24.0",
            "port": "80, 443",
            "installPath": "/usr/local/nginx",
            "configPath": "/etc/nginx/nginx.conf",
            "status": "running",
            "remarks": "反向代理、SSL 卸载与动静分离集群"
        })
        soft_counter += 1

    if "tomcat" in text or "应用" in text or "财务" in text or "社保" in text or "票据" in text or "精益" in text:
        software_components.append({
            "id": f"soft-{soft_counter}",
            "assetId": asset_id,
            "assetName": name,
            "assetIp": ip_for_soft,
            "projectId": pid,
            "projectName": norm_pname,
            "category": "middleware",
            "name": "Apache Tomcat / SpringBoot",
            "version": "9.0.86",
            "port": "8080, 8009",
            "installPath": "/usr/local/tomcat",
            "configPath": "/usr/local/tomcat/conf/server.xml",
            "status": "running",
            "remarks": "企业级 Java EE 应用服务运行容器"
        })
        soft_counter += 1

    if is_minio_node:
        software_components.append({
            "id": f"soft-{soft_counter}",
            "assetId": asset_id,
            "assetName": name,
            "assetIp": ip_for_soft,
            "projectId": pid,
            "projectName": norm_pname,
            "category": "middleware",
            "name": "MinIO Distributed Server",
            "version": "RELEASE.2023-10-16",
            "port": "9000, 9001",
            "installPath": "/opt/minio",
            "configPath": "/etc/minio/minio.conf",
            "status": "running",
            "remarks": "S3 兼容的高可用分布式分布式对象存储"
        })
        soft_counter += 1

    # ================= Generate Ops Channels =================
    # 1. SSH Command (Direct)
    if cat != "网络" and ip_for_soft:
        ssh_user = "root" if remote_port == 2222 else "ops_admin"
        ops_channels.append({
            "id": f"chan-ssh-{chan_counter}",
            "assetId": asset_id,
            "assetName": name,
            "assetIp": ip_for_soft,
            "projectId": pid,
            "projectName": norm_pname,
            "channelType": "ssh",
            "name": f"Linux 终端 SSH 命令行直连",
            "urlOrTarget": f"ssh {ssh_user}@{ip_for_soft} -p {remote_port}",
            "accountNote": f"{ssh_user} / 密钥登录 (端口 {remote_port})",
            "remarks": f"生产运维端口 {remote_port}，经堡垒机免密分发密钥登录"
        })
        chan_counter += 1

        # 2. JumpServer
        ops_channels.append({
            "id": f"chan-jump-{chan_counter}",
            "assetId": asset_id,
            "assetName": name,
            "assetIp": ip_for_soft,
            "projectId": pid,
            "projectName": norm_pname,
            "channelType": "jumpserver",
            "name": f"堡垒机录屏审计直通: {name}",
            "urlOrTarget": f"jumpserver://asset/{asset_id}?ip={ip_for_soft}",
            "accountNote": "JumpServer 集中审计授权",
            "remarks": "符合等级保护 2.0 三级标准审计录屏"
        })
        chan_counter += 1

    # 3. Web Service link if applicable
    if "应用" in text or "web" in text or "商城" in text or "招采" in text or "官网" in text or "网厅" in text or "nginx" in text or "vip" in text:
        port_suffix = ":8080" if "应用" in text else ""
        ops_channels.append({
            "id": f"chan-web-{chan_counter}",
            "assetId": asset_id,
            "assetName": name,
            "assetIp": ip_for_soft,
            "projectId": pid,
            "projectName": norm_pname,
            "channelType": "web_link",
            "name": f"{name} 业务管理控制台",
            "urlOrTarget": f"http://{ip_for_soft}{port_suffix}/admin",
            "accountNote": "业务系统 Web 管理后台",
            "remarks": "内网专用管理后台，需通过 VPN 或专网拨号访问"
        })
        chan_counter += 1

# Also generate 1 VPN channel per project
for p in projects_list:
    pid = p["id"]
    pname = p["name"]
    # Find matching room from devs
    devs = project_groups_map.get(pname, [])
    room = devs[0].get("机房") if devs else "六里桥机房"
    vpn_info = vpn_mapping.get(room, vpn_mapping["六里桥机房"])

    ops_channels.append({
        "id": f"chan-vpn-{chan_counter}",
        "projectId": pid,
        "projectName": pname,
        "channelType": "vpn",
        "name": f"{pname} 专用 VPN 安全接入通道",
        "urlOrTarget": vpn_info["gw"],
        "vpnGateway": vpn_info["gw"],
        "vpnClientType": vpn_info["client"],
        "vpnNetworkSegment": vpn_info["seg"],
        "accountNote": "双因子认证 (动态口令Token + 专用硬件证书)",
        "remarks": f"访问当前项目全量 {p['deviceCount']} 台业务节点的前置网络通道"
    })
    chan_counter += 1

print(f"Entities summary:")
print(f"- Physical Hosts: {len(physical_hosts)}")
print(f"- Virtual Machines: {len(vms)}")
print(f"- Switches/VIPs: {len(switches)}")
print(f"- Software Components: {len(software_components)}")
print(f"- Ops Channels: {len(ops_channels)}")

# Add top switches for IDC cabinet management
for cab in cabinets_def:
    cab_id = cab['id']
    room_id = cab['roomId']
    switches.append({
        'id': f'sw-tor-{cab_id}',
        'seq': 9000,
        'assetNo': f'TOR-{cab["code"]}',
        'name': f'{cab["name"]} ToR交换机',
        'roomId': room_id,
        'roomName': next((r["name"] for r in rooms_def if r["id"] == room_id), ""),
        'cabinetId': cab_id,
        'startU': 41,
        'uHeight': 2,
        'ip': f'10.200.{(hash(cab_id) % 250) + 1}.1',
        'brand': '华为',
        'model': 'CloudEngine 6857-48S6CQ',
        'role': '接入交换机',
        'portCount': 48,
        'activePorts': 24,
        'status': 'online',
        'businessId': 'prj-001',
        'customerName': '北控伟仕保障团队',
        'projectName': '基础设施保障',
        'env': '生产',
        'cloudVendor': '自建机房',
        'regionName': '内网区',
        'category': '网络',
        'deviceType': '物理机',
        'privateIp': f'10.200.{(hash(cab_id) % 250) + 1}.1',
        'privateIpv6': None,
        'internalWanIp': None,
        'eip': None,
        'vip': None,
        'publicIp': None,
        'cpuArch': 'ARM',
        'cpuCores': 4,
        'memoryGb': 8,
        'systemDiskGb': 32,
        'dataDiskGb': 0,
        'sharedDiskGb': None,
        'objectStorageGb': None,
        'osFamily': 'VRP',
        'osVersion': 'VRP V800R021',
        'kernelVersion': '5.4.0',
        'isXinchuang': '是',
        'remotePort': 22,
        'remarks': '机柜顶部双上行 100GE ToR 接入交换机'
    })

# Databases
databases_def = [
    {
        'id': 'db-1',
        'name': '原三险生产核心库 (Oracle RAC)',
        'type': 'Oracle',
        'version': 'Oracle 11g R2 Enterprise',
        'hostIp': '192.123.74.70',
        'port': 1521,
        'arch': '主备高可用',
        'businessId': next((p['id'] for p in projects_list if '三险' in p['name']), 'prj-001'),
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
        'businessId': next((p['id'] for p in projects_list if '仲裁核心' in p['name']), 'prj-001'),
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
        'businessId': next((p['id'] for p in projects_list if '仲裁核心' in p['name']), 'prj-001'),
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
        'businessId': next((p['id'] for p in projects_list if '调解仲裁' in p['name']), 'prj-001'),
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
        'businessId': next((p['id'] for p in projects_list if '工会职服' in p['name']), 'prj-001'),
        'dataSize': '680 GB',
        'connectionCount': 520,
        'status': 'active'
    },
    {
        'id': 'db-6',
        'name': '智科精益平台 PostgreSQL 实例',
        'type': 'PostgreSQL',
        'version': 'PostgreSQL 14.5',
        'hostIp': '170.11.30.2',
        'port': 5432,
        'arch': '主从高可用 (1主2从)',
        'businessId': next((p['id'] for p in projects_list if '智科精益' in p['name']), 'prj-001'),
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
        'businessId': next((p['id'] for p in projects_list if '税务年金' in p['name']), 'prj-001'),
        'dataSize': '420 GB',
        'connectionCount': 190,
        'status': 'active'
    }
]

# Businesses models
businesses_list = []
for p in projects_list:
    b_hosts = [h['id'] for h in physical_hosts if h.get('projectId') == p['id']]
    b_vms = [v['id'] for v in vms if v.get('projectId') == p['id']]
    b_sw = [s['id'] for s in switches if s.get('projectId') == p['id']]
    b_dbs = [db['id'] for db in databases_def if db.get('businessId') == p['id']]
    
    businesses_list.append({
        'id': p['id'].replace('prj-', 'biz-'),
        'name': p['name'],
        'code': p['code'],
        'level': "核心 L1" if ("核心" in p['name'] or "三险" in p['name'] or "年金" in p['name']) else "重要 L2" if ("数智" in p['name'] or "监管" in p['name'] or "庭审" in p['name']) else "通用 L3",
        'owner': "运维保障组",
        'department': p['customerName'],
        'description': p['description'],
        'healthScore': p['healthScore'],
        'hostIds': b_hosts + b_vms,
        'dbIds': b_dbs,
        'switchIds': b_sw
    })

# Agent probes
probes_list = [
    {
        "id": "probe-1",
        "hostIp": "192.123.74.70",
        "hostname": "数据库db3",
        "agentVersion": "v2.8.4-release",
        "status": "online",
        "cpuUsage": 18.2,
        "memUsage": 64.5,
        "lastHeartbeat": "刚刚 (1s前)"
    },
    {
        "id": "probe-2",
        "hostIp": "192.123.74.36",
        "hostname": "仲裁数据库1",
        "agentVersion": "v2.8.4-release",
        "status": "online",
        "cpuUsage": 24.1,
        "memUsage": 58.2,
        "lastHeartbeat": "刚刚 (3s前)"
    },
    {
        "id": "probe-3",
        "hostIp": "192.142.71.201",
        "hostname": "oracle宿1",
        "agentVersion": "v2.8.4-release",
        "status": "online",
        "cpuUsage": 12.8,
        "memUsage": 45.0,
        "lastHeartbeat": "刚刚 (2s前)"
    },
    {
        "id": "probe-4",
        "hostIp": "170.11.30.2",
        "hostname": "智科精益平台_minio集群_01",
        "agentVersion": "v2.8.4-release",
        "status": "online",
        "cpuUsage": 8.5,
        "memUsage": 32.1,
        "lastHeartbeat": "刚刚 (4s前)"
    }
]

# Credentials
total_devs = len(raw_devices)
credentials_list = [
    {
        "id": "cred-1",
        "name": "政务外网 Linux 主机 Root 密钥",
        "type": "SSH 私钥",
        "username": "root",
        "targetCount": total_devs,
        "updated": "2026-09-18 10:00",
        "remark": "高强度 Ed25519 堡垒机集中分发免密密钥"
    },
    {
        "id": "cred-2",
        "name": "BMC IPMI 带外运维管理密码",
        "type": "BMC IPMI",
        "username": "Administrator",
        "targetCount": len(physical_hosts),
        "updated": "2026-09-18 10:00",
        "remark": "物理机 iLO / iBMC 硬件远程管理带外密码"
    },
    {
        "id": "cred-3",
        "name": "核心交换机与 SLB 负载 SNMP 凭证",
        "type": "SNMP v2c/v3",
        "username": "snmp_ro_user",
        "targetCount": len(switches),
        "updated": "2026-09-18 10:00",
        "remark": "用于网络设备端口遥测与流速采集"
    },
    {
        "id": "cred-4",
        "name": "Oracle RAC 生产数据库运维账号",
        "type": "数据库账号",
        "username": "ops_monitor",
        "targetCount": len(databases_def),
        "updated": "2026-09-18 10:00",
        "remark": "只读健康监控与慢查询性能探针专用账号"
    }
]

# 5. Output TypeScript
ts_content = f'''// CMDB Core Relational Data Model & Seed Store
// Auto-generated strictly aligned with 《信息资产台账-v340.xlsx》 sheet「02-硬件设备」
// Total assets imported: {len(raw_devices)} (Physical: {len(physical_hosts)}, VMs: {len(vms)}, Switches/VIPs: {len(switches)})
// Software instances: {len(software_components)}, Ops channels: {len(ops_channels)}

export interface IdcRoom {{
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
  status: "正常" | "预警" | "空闲";
  manager: string;
  row: string;
  remark: string;
}}

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
  roomId?: string;
  roomName?: string;
  cabinetId?: string;
  startU?: number;
  uHeight?: number;
}}

export interface VmHost extends AssetMeta {{
  id: string;
  name: string;
  physicalHostId: string;
  ip?: string;
  privateIp?: string | null;
  cpu: string;
  memory: string;
  disk: string;
  os: string;
  businessId: string;
  status: "running" | "stopped";
  role: string;
  updated: string;
  roomId?: string;
  roomName?: string;
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
  roomId?: string;
  roomName?: string;
  cabinetId?: string;
  startU?: number;
  uHeight?: number;
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

export interface SoftwareComponent {{
  id: string;
  assetId?: string;
  assetName?: string;
  assetIp?: string;
  projectId: string;
  projectName: string;
  category: "database" | "middleware" | "plugin" | "web_server";
  name: string;
  version: string;
  port?: string;
  installPath?: string;
  configPath?: string;
  status: "running" | "stopped" | "warning";
  remarks?: string;
}}

export interface OpsChannel {{
  id: string;
  assetId?: string;
  assetName?: string;
  assetIp?: string;
  projectId: string;
  projectName: string;
  channelType: "web_link" | "ssh" | "rdp" | "jumpserver" | "vpn";
  name: string;
  urlOrTarget: string;
  vpnGateway?: string;
  vpnClientType?: string;
  vpnNetworkSegment?: string;
  accountNote?: string;
  remarks?: string;
}}

export const initialRooms: IdcRoom[] = {json.dumps(rooms_def, ensure_ascii=False, indent=2)};

export const initialCabinets: IdcCabinet[] = {json.dumps(cabinets_def, ensure_ascii=False, indent=2)};

export const initialProjects: ProjectGroup[] = {json.dumps(projects_list, ensure_ascii=False, indent=2)};

export const initialHosts: PhysicalHost[] = {json.dumps(physical_hosts, ensure_ascii=False, indent=2)};

export const initialVms: VmHost[] = {json.dumps(vms, ensure_ascii=False, indent=2)};

export const initialSwitches: SwitchDevice[] = {json.dumps(switches, ensure_ascii=False, indent=2)};

export const initialDatabases: DatabaseAsset[] = {json.dumps(databases_def, ensure_ascii=False, indent=2)};

export const initialBusinesses: BusinessModel[] = {json.dumps(businesses_list, ensure_ascii=False, indent=2)};

export const initialProbes: AgentProbe[] = {json.dumps(probes_list, ensure_ascii=False, indent=2)};

export const initialCredentials: CredentialItem[] = {json.dumps(credentials_list, ensure_ascii=False, indent=2)};

export const initialSoftwareComponents: SoftwareComponent[] = {json.dumps(software_components, ensure_ascii=False, indent=2)};

export const initialOpsChannels: OpsChannel[] = {json.dumps(ops_channels, ensure_ascii=False, indent=2)};
'''

with open(r'e:\autops\app\cmdbData.ts', 'w', encoding='utf-8') as f:
    f.write(ts_content)

print(f"Successfully wrote {len(ts_content)} chars to e:\\autops\\app\\cmdbData.ts")
