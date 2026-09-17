import json
import os

with open(r'E:\autops\parsed_all_devices.json', 'r', encoding='utf-8') as f:
    raw_devices = json.load(f)

# Room configuration
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
        'remark': '联通云政务外网主生产机房，承载原三险、仲裁核心系统、调解仲裁等政务核心业务。'
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
        'remark': '副中心高等级政务机房，承载人社局 Oracle RAC 物理宿主集群及关键生产节点。'
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
        'remark': '首信云数据中心，承载总工会职服数智化系统、公网及高可用 VIP 负载均衡集群。'
    },
    {
        'id': 'room-4',
        'name': '亦庄国企云机房',
        'code': 'BJ-YZ-04',
        'operator': '中国电信',
        'city': '北京',
        'address': '北京市经开区亦庄国企云专属机房',
        'level': 'T3+',
        'cabinetCount': 18,
        'contact': '陈工',
        'phone': '13810010004',
        'status': '正常运行',
        'remark': '北京市国企云核心节点，承载北控智科、北控数科精益管理平台及分布式 MinIO 对象存储。'
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
        'remark': '太极政务云，承载北京市退役军人事务局企转与补贴发放系统。'
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
        'remark': '税务专网专用数据中心，承载税务年金征收系统。'
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
        'remark': '同城容灾及业务测试机房，承载工伤认定、工伤红名单、即席查询等应用。'
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
        'remark': '阿里云华北2节点，承载互联网区公共服务、智科官网及 OSS 云对象存储。'
    }
]

room_name_to_id = {r['name']: r['id'] for r in rooms_def}

# Cabinets configuration
cabinets_def = [
    # 六里桥机房
    {'id': 'cab-llq-01', 'roomId': 'room-1', 'name': 'A01 网络与负载柜', 'code': 'LLQ-A01', 'totalU': 42, 'usedU': 8, 'maxPower': 5.0, 'currentPower': 2.1, 'temperature': 21.5, 'status': '正常', 'manager': '李运维', 'row': 'Row A', 'remark': '政务外网双上行核心交换与汇聚负载柜'},
    {'id': 'cab-llq-02', 'roomId': 'room-1', 'name': 'A02 三险生产计算柜', 'code': 'LLQ-A02', 'totalU': 42, 'usedU': 22, 'maxPower': 5.5, 'currentPower': 3.2, 'temperature': 22.8, 'status': '正常', 'manager': '李运维', 'row': 'Row A', 'remark': '人社局原三险系统业务应用计算柜'},
    {'id': 'cab-llq-03', 'roomId': 'room-1', 'name': 'B01 核心数据库物理机柜', 'code': 'LLQ-B01', 'totalU': 42, 'usedU': 28, 'maxPower': 6.0, 'currentPower': 4.1, 'temperature': 23.5, 'status': '预警', 'manager': '李运维', 'row': 'Row B', 'remark': '原三险 db3、仲裁数据库 1/2 高性能物理宿主柜'},
    {'id': 'cab-llq-04', 'roomId': 'room-1', 'name': 'B02 仲裁核心计算柜', 'code': 'LLQ-B02', 'totalU': 42, 'usedU': 16, 'maxPower': 5.0, 'currentPower': 2.4, 'temperature': 22.1, 'status': '正常', 'manager': '李运维', 'row': 'Row B', 'remark': '仲裁核心系统、调解仲裁业务主机柜'},

    # 通州C1机房
    {'id': 'cab-tz-01', 'roomId': 'room-2', 'name': 'A01 核心接入与交换柜', 'code': 'TZ-A01', 'totalU': 42, 'usedU': 10, 'maxPower': 5.0, 'currentPower': 1.8, 'temperature': 21.0, 'status': '正常', 'manager': '张工', 'row': 'Row A', 'remark': '城市副中心 C1 接入交换与带外网络柜'},
    {'id': 'cab-tz-02', 'roomId': 'room-2', 'name': 'A02 Oracle RAC 物理宿主柜', 'code': 'TZ-A02', 'totalU': 42, 'usedU': 24, 'maxPower': 6.0, 'currentPower': 3.8, 'temperature': 23.0, 'status': '正常', 'manager': '张工', 'row': 'Row A', 'remark': '承载 oracle宿1、oracle宿2 调解仲裁核心物理集群'},

    # 首信机房
    {'id': 'cab-sx-01', 'roomId': 'room-3', 'name': 'A01 互联网与负载均衡柜', 'code': 'SX-A01', 'totalU': 42, 'usedU': 18, 'maxPower': 5.0, 'currentPower': 2.6, 'temperature': 22.0, 'status': '正常', 'manager': '王工', 'row': 'Row A', 'remark': '承载总工会高可用 VIP 负载均衡及域名接入设备'},
    {'id': 'cab-sx-02', 'roomId': 'room-3', 'name': 'A02 工会职服 K8s 计算柜', 'code': 'SX-A02', 'totalU': 42, 'usedU': 26, 'maxPower': 5.5, 'currentPower': 3.5, 'temperature': 22.7, 'status': '正常', 'manager': '王工', 'row': 'Row A', 'remark': '工会职服数智化容器集群宿主机柜'},

    # 亦庄国企云机房
    {'id': 'cab-yz-01', 'roomId': 'room-4', 'name': 'A01 国企云计算服务柜', 'code': 'YZ-A01', 'totalU': 42, 'usedU': 20, 'maxPower': 5.0, 'currentPower': 2.9, 'temperature': 22.3, 'status': '正常', 'manager': '陈工', 'row': 'Row A', 'remark': '北控智科、北控数科精益管理平台服务柜'},
    {'id': 'cab-yz-02', 'roomId': 'room-4', 'name': 'A02 MinIO 分布式存储集群柜', 'code': 'YZ-A02', 'totalU': 42, 'usedU': 16, 'maxPower': 5.5, 'currentPower': 2.7, 'temperature': 21.9, 'status': '正常', 'manager': '陈工', 'row': 'Row A', 'remark': '智科分布式对象存储 4 节点集群专用机柜'},

    # 太极云机房
    {'id': 'cab-tj-01', 'roomId': 'room-5', 'name': 'A01 退军业务服务柜', 'code': 'TJ-A01', 'totalU': 42, 'usedU': 12, 'maxPower': 4.5, 'currentPower': 1.9, 'temperature': 22.0, 'status': '正常', 'manager': '刘工', 'row': 'Row A', 'remark': '退役军人事务局企转与补贴发放业务柜'},

    # 税务机房
    {'id': 'cab-sw-01', 'roomId': 'room-6', 'name': 'A01 税务专网年金服务柜', 'code': 'SW-A01', 'totalU': 42, 'usedU': 14, 'maxPower': 5.0, 'currentPower': 2.2, 'temperature': 21.8, 'status': '正常', 'manager': '赵工', 'row': 'Row A', 'remark': '税务年金征收系统计算与数据库服务器柜'},

    # 酒仙桥机房
    {'id': 'cab-jxq-01', 'roomId': 'room-7', 'name': 'A01 容灾与工伤服务柜', 'code': 'JXQ-A01', 'totalU': 42, 'usedU': 16, 'maxPower': 5.0, 'currentPower': 2.3, 'temperature': 22.4, 'status': '正常', 'manager': '高工', 'row': 'Row A', 'remark': '工伤认定、工伤红名单容灾备用服务器柜'},

    # 阿里云机房
    {'id': 'cab-ali-01', 'roomId': 'room-8', 'name': 'A01 阿里云虚拟数据中心柜', 'code': 'ALI-A01', 'totalU': 42, 'usedU': 10, 'maxPower': 4.5, 'currentPower': 1.5, 'temperature': 21.2, 'status': '正常', 'manager': '孙工', 'row': 'Row A', 'remark': '阿里云华北节点公网网关与智科官网柜'}
]

# Business projects mapping
unique_projects = list(set(d.get('项目名称') for d in raw_devices if d.get('项目名称')))
biz_map = {}
for idx, p in enumerate(unique_projects, 1):
    biz_id = f'biz-{idx}'
    # determine customer
    custs = list(set(d.get('客户名称') for d in raw_devices if d.get('项目名称') == p and d.get('客户名称')))
    customer = custs[0] if custs else '北京市人力资源和社会保障局'
    
    # level
    level = '核心 L1' if '核心' in p or '原三险' in p or '年金' in p or 'Oracle' in p else '重要 L2' if '数智' in p or '监管' in p or '庭审' in p else '通用 L3'
    
    biz_map[p] = {
        'id': biz_id,
        'name': p,
        'code': f'PRJ-{idx:03d}',
        'level': level,
        'owner': '运维保障组',
        'department': customer,
        'description': f'{customer}承建之「{p}」，由北控伟仕保障团队提供 7×24 小时全生命周期运维保障。',
        'healthScore': 98 if '核心' in p else 96,
        'hostIds': [],
        'dbIds': [],
        'switchIds': []
    }

print(f'Projects counted: {len(biz_map)}')

# Classify devices into: Physical Hosts, Virtual Hosts, Switches, Storage
physical_hosts = []
vms = []
switches = []
databases = []

# Predefined physical allocations
# 1. 数据库db3 (六里桥机房, cab-llq-03, startU 1, uHeight 4)
# 2. 仲裁数据库1 (六里桥机房, cab-llq-03, startU 6, uHeight 4)
# 3. 仲裁数据库2 (六里桥机房, cab-llq-03, startU 11, uHeight 4)
# 4. oracle宿1 (通州C1机房, cab-tz-02, startU 1, uHeight 4)
# 5. oracle宿2 (通州C1机房, cab-tz-02, startU 6, uHeight 4)
# 6~9. MinIO集群01~04 (亦庄国企云机房, cab-yz-02, startU 1, 4, 7, 10, 各2U)

for d in raw_devices:
    seq = d.get('序号')
    name = d.get('设备名称') or f'device-{seq}'
    cat = d.get('设备大类') or '服务器'
    dtype = d.get('设备类型') or '虚拟机'
    room_name = d.get('机房') or '六里桥机房'
    room_id = room_name_to_id.get(room_name, 'room-1')
    proj_name = d.get('项目名称')
    biz_info = biz_map.get(proj_name)
    biz_id = biz_info['id'] if biz_info else 'biz-1'

    ip = d.get('私有IP（业务IP）') or d.get('内大网IP') or d.get('VIP地址') or '192.168.1.1'
    os_name = f"{d.get('OS发行版') or ''} {d.get('OS版本') or ''}".strip() or 'Linux'

    cpu_cores = d.get('CPU核数')
    cpu_str = f"{cpu_cores} 核 vCPU ({d.get('CPU架构') or 'x86_64'})" if cpu_cores else '4 核 (x86_64)'

    mem_gb = d.get('内存GB')
    mem_str = f"{mem_gb} GB" if mem_gb else '16 GB'

    sys_disk = d.get('系统盘GB') or 30
    data_disk = d.get('数据盘GB') or 0
    disk_str = f"{sys_disk}G(系统) + {data_disk}G(数据)" if data_disk else f"{sys_disk} GB"

    # Is it physical host?
    if dtype == '物理机' or 'minio集群' in name:
        # Determine cabinet & U
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
        elif 'minio集群_01' in name:
            cab_id, start_u, u_h = 'cab-yz-02', 1, 2
        elif 'minio集群_02' in name:
            cab_id, start_u, u_h = 'cab-yz-02', 4, 2
        elif 'minio集群_03' in name:
            cab_id, start_u, u_h = 'cab-yz-02', 7, 2
        elif 'minio集群_04' in name:
            cab_id, start_u, u_h = 'cab-yz-02', 10, 2
        else:
            cab_id, start_u, u_h = 'cab-llq-02', 1, 2

        phy_item = {
            'id': f'phy-{seq}',
            'seq': seq,
            'assetNo': f'ASSET-{seq:04d}',
            'hostname': name,
            'name': name,
            'roomId': room_id,
            'roomName': room_name,
            'cabinetId': cab_id,
            'startU': start_u,
            'uHeight': u_h,
            'ip': ip,
            'bmcIp': f"192.168.100.{seq % 200 + 10}",
            'brand': '华为' if seq % 2 == 0 else '浪潮',
            'model': 'FusionServer Pro 2288H V5' if 'db3' in name or 'oracle' in name else 'NF5280M5',
            'os': os_name,
            'cpu': cpu_str,
            'memory': mem_str,
            'disk': disk_str,
            'role': '核心数据库物理机' if 'db' in name or 'oracle' in name else '分布式存储节点',
            'businessId': biz_id,
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
            # Excel fields
            'customerName': d.get('客户名称'),
            'projectName': proj_name,
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
            'isXinchuang': d.get('是否信创OS'),
            'remotePort': d.get('远程端口'),
            'remarks': d.get('备注')
        }
        physical_hosts.append(phy_item)
        if biz_info:
            biz_info['hostIds'].append(phy_item['id'])

    elif cat == '网络' or dtype == '负载均衡':
        # Switch / Load Balancer
        cab_id = 'cab-sx-01' if room_id == 'room-3' else 'cab-llq-01'
        u_idx = 10 + (seq % 15)
        sw_item = {
            'id': f'sw-{seq}',
            'seq': seq,
            'assetNo': f'NET-{seq:04d}',
            'name': name,
            'roomId': room_id,
            'roomName': room_name,
            'cabinetId': cab_id,
            'startU': u_idx,
            'uHeight': 1,
            'ip': ip,
            'brand': '锐捷' if 'VIP' in name else '华为',
            'model': 'RG-EG3000G / 高可用SLB' if 'VIP' in name else 'CloudEngine 6857',
            'role': '负载均衡设备' if 'VIP' in name or '公网' in name else '核心交换机',
            'portCount': 48,
            'activePorts': 32,
            'status': 'online',
            'businessId': biz_id,
            # Excel fields
            'customerName': d.get('客户名称'),
            'projectName': proj_name,
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
            'isXinchuang': d.get('是否信创OS'),
            'remotePort': d.get('远程端口'),
            'remarks': d.get('备注')
        }
        switches.append(sw_item)
        if biz_info:
            biz_info['switchIds'].append(sw_item['id'])

    else:
        # Virtual Machine / Cloud Host (虚拟机)
        # Select hypervisor physical host based on room / project
        phy_id = 'phy-165' if '仲裁' in (proj_name or '') and '通州' in room_name else 'phy-1' if '三险' in (proj_name or '') else 'phy-49' if '仲裁' in (proj_name or '') else 'phy-208'
        vm_item = {
            'id': f'vm-{seq}',
            'seq': seq,
            'name': name,
            'physicalHostId': phy_id,
            'roomId': room_id,
            'roomName': room_name,
            'ip': ip,
            'privateIp': d.get('私有IP（业务IP）') or ip,
            'cpu': cpu_str,
            'memory': mem_str,
            'disk': disk_str,
            'os': os_name,
            'businessId': biz_id,
            'status': 'running',
            'role': '应用微服务节点' if '应用' in name or '服务' in name else '数据库节点' if '库' in name or 'db' in name.lower() else '通用计算实例',
            'updated': '2026-09-17 19:35',
            # Excel fields
            'customerName': d.get('客户名称'),
            'projectName': proj_name,
            'env': d.get('环境'),
            'cloudVendor': d.get('云厂商'),
            'regionName': d.get('区域名称'),
            'category': cat,
            'deviceType': dtype,
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
            'isXinchuang': d.get('是否信创OS'),
            'remotePort': d.get('远程端口'),
            'remarks': d.get('备注')
        }
        vms.append(vm_item)
        if biz_info:
            biz_info['hostIds'].append(vm_item['id'])

print(f'Physical hosts: {len(physical_hosts)}')
print(f'Virtual machines: {len(vms)}')
print(f'Switches: {len(switches)}')

# Also add default top switches for each cabinet so IdcManagement displays realistic top-of-rack switches
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
        'businessId': 'biz-1',
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

# Add real database instances matching the Excel assets
databases_def = [
    {
        'id': 'db-1',
        'name': '原三险生产核心库 (Oracle RAC)',
        'type': 'Oracle',
        'version': 'Oracle 11g R2 Enterprise',
        'hostIp': '192.123.74.70',
        'port': 1521,
        'arch': '主备高可用',
        'businessId': next((b['id'] for b in biz_map.values() if '原三险' in b['name']), 'biz-1'),
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
        'businessId': next((b['id'] for b in biz_map.values() if '仲裁核心' in b['name']), 'biz-1'),
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
        'businessId': next((b['id'] for b in biz_map.values() if '仲裁核心' in b['name']), 'biz-1'),
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
        'businessId': next((b['id'] for b in biz_map.values() if '调解仲裁' in b['name']), 'biz-1'),
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
        'businessId': next((b['id'] for b in biz_map.values() if '工会职服' in b['name']), 'biz-1'),
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
        'businessId': next((b['id'] for b in biz_map.values() if '智科精益' in b['name']), 'biz-1'),
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
        'businessId': next((b['id'] for b in biz_map.values() if '税务年金' in b['name']), 'biz-1'),
        'dataSize': '420 GB',
        'connectionCount': 190,
        'status': 'active'
    }
]

# Update biz model with db ids
for db in databases_def:
    b = next((x for x in biz_map.values() if x['id'] == db['businessId']), None)
    if b:
        b['dbIds'].append(db['id'])

businesses_list = list(biz_map.values())

print('Generating TypeScript file...')

ts_code = f'''// CMDB Core Relational Data Model & Seed Store
// Auto-generated strictly aligned with 《信息资产台账-v331toAI.xlsx》
// Total assets imported: {len(raw_devices)} (Physical: {len(physical_hosts)}, VMs: {len(vms)}, Switches/VIPs: {len(switches)})

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
  totalU: number; // typically 42
  usedU: number;
  maxPower: number; // kW
  currentPower: number; // kW
  temperature: number; // °C
  status: "正常" | "预警" | "空闲";
  manager: string;
  row: string;
  remark: string;
}}

// Standardized Asset Interface encompassing ALL Excel Fields
export interface AssetMeta {{
  seq?: number;
  customerName?: string | null;
  projectName?: string | null;
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
  roomId: string;
  roomName: string;
  cabinetId: string;
  startU: number; // 1 ~ 42
  uHeight: number; // 1, 2, 4
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
  roomId: string;
  roomName: string;
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
  roomId: string;
  roomName: string;
  cabinetId: string;
  startU: number;
  uHeight: number;
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

// Initial Seed Data mirroring real enterprise CMDB IDC system
export const initialRooms: IdcRoom[] = {json.dumps(rooms_def, ensure_ascii=False, indent=2)};

export const initialCabinets: IdcCabinet[] = {json.dumps(cabinets_def, ensure_ascii=False, indent=2)};

export const initialHosts: PhysicalHost[] = {json.dumps(physical_hosts, ensure_ascii=False, indent=2)};

export const initialVms: VmHost[] = {json.dumps(vms, ensure_ascii=False, indent=2)};

export const initialSwitches: SwitchDevice[] = {json.dumps(switches, ensure_ascii=False, indent=2)};

export const initialDatabases: DatabaseAsset[] = {json.dumps(databases_def, ensure_ascii=False, indent=2)};

export const initialBusinesses: BusinessModel[] = {json.dumps(businesses_list, ensure_ascii=False, indent=2)};

export const initialProbes: AgentProbe[] = [
  {{
    id: "probe-1",
    hostIp: "192.123.74.70",
    hostname: "数据库db3",
    agentVersion: "v2.8.4-release",
    status: "online",
    cpuUsage: 18.2,
    memUsage: 64.5,
    lastHeartbeat: "刚刚 (1s前)"
  }},
  {{
    id: "probe-2",
    hostIp: "192.123.74.36",
    hostname: "仲裁数据库1",
    agentVersion: "v2.8.4-release",
    status: "online",
    cpuUsage: 24.1,
    memUsage: 58.2,
    lastHeartbeat: "刚刚 (3s前)"
  }},
  {{
    id: "probe-3",
    hostIp: "192.142.71.201",
    hostname: "oracle宿1",
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
    name: "政务外网 Linux 主机 Root 密钥",
    type: "SSH 私钥",
    username: "root",
    targetCount: {len(vms) + len(physical_hosts)},
    updated: "2026-09-17 18:00",
    remark: "高强度 Ed25519 堡垒机集中分发免密密钥"
  }},
  {{
    id: "cred-2",
    name: "BMC IPMI 带外运维管理密码",
    type: "BMC IPMI",
    username: "Administrator",
    targetCount: {len(physical_hosts)},
    updated: "2026-09-17 18:00",
    remark: "物理机 iLO / iBMC 硬件远程管理带外密码"
  }},
  {{
    id: "cred-3",
    name: "核心交换机与 SLB 负载 SNMP 凭证",
    type: "SNMP v2c/v3",
    username: "snmp_ro_user",
    targetCount: {len(switches)},
    updated: "2026-09-17 18:00",
    remark: "用于网络设备端口遥测与流速采集"
  }},
  {{
    id: "cred-4",
    name: "Oracle RAC 生产数据库运维账号",
    type: "数据库账号",
    username: "ops_monitor",
    targetCount: {len(databases_def)},
    updated: "2026-09-17 18:00",
    remark: "只读健康监控与慢查询性能探针专用账号"
  }}
];
'''

with open(r'E:\autops\app\cmdbData.ts', 'w', encoding='utf-8') as f:
    f.write(ts_code)

print('cmdbData.ts generated successfully!')
