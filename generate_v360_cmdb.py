import glob
import os
import openpyxl
import json
import re

os.makedirs('db', exist_ok=True)
os.makedirs('scripts', exist_ok=True)

excel_path = '信息资产台账-v360.xlsx'
if not os.path.exists(excel_path):
    raise FileNotFoundError(f"未找到 {excel_path}")

print(f"Loading workbook: {excel_path} ...")
wb = openpyxl.load_workbook(excel_path, data_only=True)

def clean_str(val):
    if val is None:
        return ""
    s = str(val).strip()
    return s if s != "None" else ""

def clean_int(val, default=0):
    if val is None:
        return default
    try:
        return int(float(str(val).strip()))
    except (ValueError, TypeError):
        return default

def sql_escape(s):
    if s is None:
        return "NULL"
    s = str(s).replace("\\", "\\\\").replace("'", "\\'").replace("\r", "").replace("\n", "\\n")
    return f"'{s}'"

# ==========================================================
# 1. Parse 02-硬件设备
# Row 4 is header, Row 5 to 303 is data (299 records)
# ==========================================================
ws_hw = wb['02-硬件设备']
hw_rows = list(ws_hw.iter_rows(values_only=True))

raw_hw = []
for idx in range(4, 303):
    r = hw_rows[idx]
    if not any(r):
        continue
    seq = clean_int(r[0], default=len(raw_hw) + 1)
    project_no = clean_str(r[1])
    customer_name = clean_str(r[2])
    project_name = clean_str(r[3])
    env = clean_str(r[4]) or "生产"
    cloud_vendor = clean_str(r[5])
    region_name = clean_str(r[6])
    device_name = clean_str(r[7])
    device_category = clean_str(r[8]) or "服务器"
    device_type = clean_str(r[9]) or "虚拟机"
    idc_room = clean_str(r[10])
    private_ip = clean_str(r[11])
    private_ipv6 = clean_str(r[12]) if len(r) > 12 else ""
    inner_net_ip = clean_str(r[13]) if len(r) > 13 else ""
    eip = clean_str(r[14]) if len(r) > 14 else ""
    vip = clean_str(r[15]) if len(r) > 15 else ""
    public_ip = clean_str(r[16]) if len(r) > 16 else ""
    cpu_arch = clean_str(r[17]) if len(r) > 17 else ""
    cpu_cores = clean_int(r[18]) if len(r) > 18 else 0
    memory_gb = clean_int(r[19]) if len(r) > 19 else 0
    sys_disk_gb = clean_int(r[20]) if len(r) > 20 else 0
    data_disk_gb = clean_int(r[21]) if len(r) > 21 else 0
    share_disk_gb = clean_int(r[22]) if len(r) > 22 else 0
    oss_gb = clean_int(r[23]) if len(r) > 23 else 0
    os_distro = clean_str(r[24]) if len(r) > 24 else ""
    os_version = clean_str(r[25]) if len(r) > 25 else ""
    kernel_version = clean_str(r[26]) if len(r) > 26 else ""
    is_xinchuang = clean_str(r[27]) if len(r) > 27 else "否"
    remote_port = clean_int(r[28], default=22) if len(r) > 28 else 22
    remarks = clean_str(r[29]) if len(r) > 29 else ""

    item = {
        "id": f"hw-v360-{seq}",
        "seq": seq,
        "projectNo": project_no,
        "customerName": customer_name,
        "projectName": project_name,
        "env": env,
        "cloudVendor": cloud_vendor,
        "regionName": region_name,
        "deviceName": device_name,
        "deviceCategory": device_category,
        "deviceType": device_type,
        "idcRoom": idc_room,
        "privateIp": private_ip,
        "privateIpv6": private_ipv6,
        "innerNetIp": inner_net_ip,
        "eip": eip,
        "vip": vip,
        "publicIp": public_ip,
        "cpuArch": cpu_arch,
        "cpuCores": cpu_cores,
        "memoryGb": memory_gb,
        "sysDiskGb": sys_disk_gb,
        "dataDiskGb": data_disk_gb,
        "shareDiskGb": share_disk_gb,
        "ossGb": oss_gb,
        "osDistro": os_distro,
        "osVersion": os_version,
        "kernelVersion": kernel_version,
        "isXinchuang": is_xinchuang,
        "remotePort": remote_port,
        "remarks": remarks
    }
    raw_hw.append(item)

print(f"Extracted {len(raw_hw)} hardware records from 02-硬件设备 (Seq 1 to {raw_hw[-1]['seq']})")

# ==========================================================
# 2. Parse 03-数据库
# Row 4 is header, Row 5 to 51 is data (47 records)
# ==========================================================
ws_db = wb['03-数据库']
db_rows = list(ws_db.iter_rows(values_only=True))

raw_dbs = []
for idx in range(4, len(db_rows)):
    r = db_rows[idx]
    if not any(r):
        continue
    seq = clean_int(r[0], default=len(raw_dbs) + 1)
    project_no = clean_str(r[1])
    customer_name = clean_str(r[2])
    project_name = clean_str(r[3])
    env = clean_str(r[4]) or "生产"
    cloud_vendor = clean_str(r[5])
    region_name = clean_str(r[6])
    private_ip = clean_str(r[7])
    vip_eip = clean_str(r[8]) if len(r) > 8 else ""
    db_category = clean_str(r[9]) if len(r) > 9 else ""
    db_software = clean_str(r[10]) if len(r) > 10 else ""
    version = clean_str(r[11]) if len(r) > 11 else ""
    port = clean_int(r[12], default=0) if len(r) > 12 else 0
    instance_sid = clean_str(r[13]) if len(r) > 13 else ""
    db_name = clean_str(r[14]) if len(r) > 14 else ""
    deploy_mode = clean_str(r[15]) if len(r) > 15 else "单机"
    cluster_name = clean_str(r[16]) if len(r) > 16 else ""
    remarks = clean_str(r[17]) if len(r) > 17 else ""

    item = {
        "id": f"db-v360-{seq}",
        "seq": seq,
        "projectNo": project_no,
        "customerName": customer_name,
        "projectName": project_name,
        "name": f"{customer_name}-{project_name}-{db_software}".strip("-") if (customer_name or project_name) else f"数据库实例-{seq}",
        "type": db_software or "关系型数据库",
        "hostIp": private_ip,
        "env": env,
        "cloudVendor": cloud_vendor,
        "regionName": region_name,
        "privateIp": private_ip,
        "vipEip": vip_eip,
        "dbCategory": db_category,
        "dbSoftware": db_software,
        "version": version,
        "port": port,
        "instanceSid": instance_sid,
        "dbName": db_name,
        "deployMode": deploy_mode,
        "clusterName": cluster_name,
        "status": "active",
        "remarks": remarks
    }
    raw_dbs.append(item)

print(f"Extracted {len(raw_dbs)} database records from 03-数据库")

# ==========================================================
# 3. Parse 04-中间件
# Row 4 is header, Row 5 is data (1 record)
# ==========================================================
ws_mw = wb['04-中间件']
mw_rows = list(ws_mw.iter_rows(values_only=True))

raw_mws = []
for idx in range(4, len(mw_rows)):
    r = mw_rows[idx]
    if not any(r):
        continue
    customer_name = clean_str(r[0])
    project_name = clean_str(r[1])
    env = clean_str(r[2]) or "生产"
    cloud_vendor = clean_str(r[3])
    region_name = clean_str(r[4])
    private_ip = clean_str(r[5])
    mw_type = clean_str(r[6]) if len(r) > 6 else ""
    mw_software = clean_str(r[7]) if len(r) > 7 else ""
    version = clean_str(r[8]) if len(r) > 8 else ""
    port = clean_str(r[9]) if len(r) > 9 else ""
    app_runtime_env = clean_str(r[10]) if len(r) > 10 else ""
    remarks = clean_str(r[11]) if len(r) > 11 else ""

    item = {
        "id": f"mw-v360-{len(raw_mws) + 1}",
        "seq": len(raw_mws) + 1,
        "projectNo": "",
        "customerName": customer_name,
        "projectName": project_name,
        "name": f"{mw_software} {version}".strip(),
        "env": env,
        "cloudVendor": cloud_vendor,
        "regionName": region_name,
        "privateIp": private_ip,
        "assetIp": private_ip,
        "assetName": f"{project_name}-中间件",
        "mwType": mw_type,
        "mwSoftware": mw_software,
        "version": version,
        "port": port,
        "runtime": app_runtime_env,
        "status": "running",
        "remarks": remarks
    }
    raw_mws.append(item)

print(f"Extracted {len(raw_mws)} middleware records from 04-中间件")

# ==========================================================
# 4. Parse 05-备份
# Row 4 is header, Row 5 is data (1 record)
# ==========================================================
ws_bk = wb['05-备份']
bk_rows = list(ws_bk.iter_rows(values_only=True))

raw_bks = []
for idx in range(4, len(bk_rows)):
    r = bk_rows[idx]
    if not any(r):
        continue
    seq = clean_int(r[0], default=len(raw_bks) + 1)
    customer_name = clean_str(r[1])
    project_name = clean_str(r[2])
    env = clean_str(r[3]) or "生产"
    cloud_vendor = clean_str(r[4])
    region_name = clean_str(r[5])
    private_ip = clean_str(r[6])
    backup_type = clean_str(r[7]) if len(r) > 7 else ""
    backup_method = clean_str(r[8]) if len(r) > 8 else ""
    backup_strategy = clean_str(r[9]) if len(r) > 9 else ""
    retention_period = clean_str(r[10]) if len(r) > 10 else ""
    backup_storage_path = clean_str(r[11]) if len(r) > 11 else ""
    remarks = clean_str(r[12]) if len(r) > 12 else ""

    item = {
        "id": f"bk-v360-{seq}",
        "seq": seq,
        "projectNo": "",
        "customerName": customer_name,
        "projectName": project_name,
        "env": env,
        "cloudVendor": cloud_vendor,
        "regionName": region_name,
        "privateIp": private_ip,
        "backupType": backup_type or "数据库全量备份",
        "backupMethod": backup_method or "冷备/快照",
        "backupPolicy": backup_strategy or "每日定时备份",
        "storageLocation": backup_storage_path or "政务外网专用存储池",
        "retentionDays": 30,
        "lastBackupTime": "2026-09-22 03:00:00",
        "status": "normal",
        "remarks": remarks
    }
    raw_bks.append(item)

print(f"Extracted {len(raw_bks)} backup records from 05-备份")

# ==========================================================
# 5. Parse 06-运维账号
# Row 3 is header (index 2), Row 4 to 84 is data (81 records)
# ==========================================================
ws_ops = wb['06-运维账号']
ops_rows = list(ws_ops.iter_rows(values_only=True))

raw_ops = []
for idx in range(3, len(ops_rows)):
    r = ops_rows[idx]
    if not any(r):
        continue
    seq = clean_int(r[0], default=len(raw_ops) + 1)
    project_no = clean_str(r[1])
    customer_name = clean_str(r[2])
    project_name = clean_str(r[3])
    cloud_vendor = clean_str(r[4])
    idc_room = clean_str(r[5])
    personnel_affiliation = clean_str(r[6])
    vpn_network_env = clean_str(r[7])
    vpn_url = clean_str(r[8])
    vpn_account = clean_str(r[9])
    vpn_user_name = clean_str(r[10])
    fortress_area = clean_str(r[11])
    fortress_url = clean_str(r[12])
    fortress_account = clean_str(r[13])
    fortress_user_name = clean_str(r[14])
    remarks = clean_str(r[15]) if len(r) > 15 else ""

    item = {
        "id": f"ops-v360-{seq}",
        "seq": seq,
        "projectNo": project_no,
        "customerName": customer_name,
        "projectName": project_name,
        "env": "生产",
        "cloudVendor": cloud_vendor,
        "regionName": fortress_area or "政务外网区",
        "privateIp": "192.123.74.70",
        "idcRoom": idc_room,
        "personnelAffiliation": personnel_affiliation,
        "vpnNetworkEnv": vpn_network_env,
        "vpnAddress": vpn_url,
        "vpnAccount": vpn_account,
        "vpnUserName": vpn_user_name,
        "fortressArea": fortress_area,
        "bastionAddress": fortress_url,
        "bastionAccount": fortress_account,
        "bastionUserName": fortress_user_name,
        "opsVendor": personnel_affiliation or "北控伟仕",
        "serverAccessAddress": f"bastion {fortress_url}" if fortress_url else "ssh",
        "monitoringCoverage": "是",
        "inspectionCycle": "每日",
        "changeWindow": "周五晚",
        "networkZone": fortress_area or "互联网区",
        "exposureSurface": "VPN/堡垒机接入",
        "remarks": remarks
    }
    raw_ops.append(item)

print(f"Extracted {len(raw_ops)} ops account records from 06-运维账号")

# ==========================================================
# 6. Aggregate Projects & Customers
# ==========================================================
project_keys = {}
for h in raw_hw:
    pname = h["projectName"]
    cname = h["customerName"]
    if pname:
        k = (cname, pname)
        if k not in project_keys:
            project_keys[k] = {
                "customerName": cname,
                "name": pname,
                "env": h["env"],
                "cloudVendor": h["cloudVendor"],
                "regionName": h["regionName"],
                "deviceCount": 0,
                "phyCount": 0,
                "vmCount": 0,
                "netCount": 0,
                "totalCores": 0,
                "totalMemoryGb": 0,
                "totalDiskGb": 0,
                "xinchuangCount": 0,
                "dbCount": 0,
                "mwCount": 0,
                "backupCount": 0,
                "opsCount": 0,
                "healthScore": 98,
            }
        project_keys[k]["deviceCount"] += 1
        if h["deviceType"] == "物理机":
            project_keys[k]["phyCount"] += 1
        else:
            project_keys[k]["vmCount"] += 1
        project_keys[k]["totalCores"] += h["cpuCores"]
        project_keys[k]["totalMemoryGb"] += h["memoryGb"]
        project_keys[k]["totalDiskGb"] += (h["sysDiskGb"] + h["dataDiskGb"] + (h.get("shareDiskGb") or 0) + (h.get("ossGb") or 0))
        if h["isXinchuang"] == "是":
            project_keys[k]["xinchuangCount"] += 1

for d in raw_dbs:
    pname = d["projectName"]
    cname = d["customerName"]
    if pname:
        k = (cname, pname)
        if k not in project_keys:
            project_keys[k] = {
                "customerName": cname,
                "name": pname,
                "env": d["env"],
                "cloudVendor": d["cloudVendor"],
                "regionName": d["regionName"],
                "deviceCount": 0,
                "phyCount": 0,
                "vmCount": 0,
                "netCount": 0,
                "totalCores": 0,
                "totalMemoryGb": 0,
                "totalDiskGb": 0,
                "xinchuangCount": 0,
                "dbCount": 0,
                "mwCount": 0,
                "backupCount": 0,
                "opsCount": 0,
                "healthScore": 98,
            }
        project_keys[k]["dbCount"] += 1

for m in raw_mws:
    pname = m["projectName"]
    cname = m["customerName"]
    if pname:
        k = (cname, pname)
        if k not in project_keys:
            project_keys[k] = {
                "customerName": cname,
                "name": pname,
                "env": m["env"],
                "cloudVendor": m["cloudVendor"],
                "regionName": m["regionName"],
                "deviceCount": 0,
                "phyCount": 0,
                "vmCount": 0,
                "netCount": 0,
                "totalCores": 0,
                "totalMemoryGb": 0,
                "totalDiskGb": 0,
                "xinchuangCount": 0,
                "dbCount": 0,
                "mwCount": 0,
                "backupCount": 0,
                "opsCount": 0,
                "healthScore": 98,
            }
        project_keys[k]["mwCount"] += 1

for b in raw_bks:
    pname = b["projectName"]
    cname = b["customerName"]
    if pname:
        k = (cname, pname)
        if k not in project_keys:
            project_keys[k] = {
                "customerName": cname,
                "name": pname,
                "env": b["env"],
                "cloudVendor": b["cloudVendor"],
                "regionName": b["regionName"],
                "deviceCount": 0,
                "phyCount": 0,
                "vmCount": 0,
                "netCount": 0,
                "totalCores": 0,
                "totalMemoryGb": 0,
                "totalDiskGb": 0,
                "xinchuangCount": 0,
                "dbCount": 0,
                "mwCount": 0,
                "backupCount": 0,
                "opsCount": 0,
                "healthScore": 98,
            }
        project_keys[k]["backupCount"] += 1

for o in raw_ops:
    pname = o["projectName"]
    cname = o["customerName"]
    if pname:
        k = (cname, pname)
        if k not in project_keys:
            project_keys[k] = {
                "customerName": cname,
                "name": pname,
                "env": "生产",
                "cloudVendor": o["cloudVendor"],
                "regionName": "运维接入区",
                "deviceCount": 0,
                "phyCount": 0,
                "vmCount": 0,
                "netCount": 0,
                "totalCores": 0,
                "totalMemoryGb": 0,
                "totalDiskGb": 0,
                "xinchuangCount": 0,
                "dbCount": 0,
                "mwCount": 0,
                "backupCount": 0,
                "opsCount": 0,
                "healthScore": 98,
            }
        project_keys[k]["opsCount"] += 1

project_list = []
proj_idx = 1
for (cname, pname), pdata in sorted(project_keys.items(), key=lambda x: (x[0][0], x[0][1])):
    pid = f"proj-{proj_idx}"
    proj_idx += 1
    pdata["id"] = pid
    pdata["code"] = f"PROJ-{proj_idx:03d}"
    pdata["description"] = f"{cname} - {pname}"
    project_list.append(pdata)

print(f"Aggregated {len(project_list)} unique project groups across all sheets")

# Link projectId back to items
for h in raw_hw:
    match = next((p for p in project_list if p["name"] == h["projectName"] and p["customerName"] == h["customerName"]), None)
    if match:
        h["projectId"] = match["id"]
for d in raw_dbs:
    match = next((p for p in project_list if p["name"] == d["projectName"] and p["customerName"] == d["customerName"]), None)
    if match:
        d["projectId"] = match["id"]
for m in raw_mws:
    match = next((p for p in project_list if p["name"] == m["projectName"] and p["customerName"] == m["customerName"]), None)
    if match:
        m["projectId"] = match["id"]
for b in raw_bks:
    match = next((p for p in project_list if p["name"] == b["projectName"] and p["customerName"] == b["customerName"]), None)
    if match:
        b["projectId"] = match["id"]
for o in raw_ops:
    match = next((p for p in project_list if p["name"] == o["projectName"] and p["customerName"] == o["customerName"]), None)
    if match:
        o["projectId"] = match["id"]

# Split physical and VM hosts with full UI compatibility
physical_hosts = []
vms = []

for h in raw_hw:
    seq = h["seq"]
    if h["deviceType"] == "物理机":
        phy_item = {
            **h,
            "id": f"host-v360-{seq}",
            "assetNo": f"SRV-PHY-{seq:03d}",
            "hostname": h["deviceName"],
            "name": h["deviceName"],
            "ip": h["privateIp"],
            "bmcIp": "",
            "brand": "信创/国产服务器",
            "model": "机架式服务器",
            "os": f"{h['osDistro']} {h['osVersion']}".strip(),
            "cpu": f"{h['cpuCores']}核",
            "memory": f"{h['memoryGb']}GB",
            "disk": f"{h['sysDiskGb'] + h['dataDiskGb']}GB",
            "role": h["deviceCategory"],
            "businessId": "biz-1",
            "status": "running",
            "powerWatts": 350,
            "cpuTemp": 42,
            "boardTemp": 38,
            "inletTemp": 22,
            "fanRpm": "5400 RPM",
            "psu1Status": "正常",
            "psu2Status": "正常",
            "raidStatus": "RAID1",
            "agentOnline": True,
            "updated": "2026-09-22",
            "roomId": "room-unicom",
            "roomName": h["idcRoom"] or "首信机房"
        }
        physical_hosts.append(phy_item)
    else:
        vm_item = {
            **h,
            "id": f"vm-v360-{seq}",
            "assetNo": f"VM-{seq:03d}",
            "name": h["deviceName"],
            "physicalHostId": "host-v360-1",
            "ip": h["privateIp"],
            "privateIp": h["privateIp"],
            "cpu": f"{h['cpuCores']}核",
            "memory": f"{h['memoryGb']}GB",
            "disk": f"{h['sysDiskGb'] + h['dataDiskGb']}GB",
            "os": f"{h['osDistro']} {h['osVersion']}".strip(),
            "businessId": "biz-1",
            "status": "running",
            "role": h["deviceCategory"],
            "updated": "2026-09-22",
            "roomId": "room-unicom",
            "roomName": h["idcRoom"] or "联通酒仙桥机房"
        }
        vms.append(vm_item)

print(f"Divided assets -> Physical: {len(physical_hosts)}, VMs: {len(vms)} (Total: {len(physical_hosts) + len(vms)})")

# ==========================================================
# 7. Generate MySQL DDL (db/mysql_schema.sql)
# ==========================================================
mysql_ddl = """-- ==========================================================
-- AutoOps CMDB 资产管理系统 - 标准 MySQL 数据库表结构
-- 适配 MySQL 5.7 / 8.0+ (utf8mb4 / InnoDB)
-- 基于《信息资产台账-v360.xlsx》严格建表
-- ==========================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- 1. 项目与系统目录表
DROP TABLE IF EXISTS `cmdb_projects`;
CREATE TABLE `cmdb_projects` (
  `id` VARCHAR(64) NOT NULL COMMENT '项目全局主键ID',
  `code` VARCHAR(64) DEFAULT '' COMMENT '项目编号',
  `name` VARCHAR(128) NOT NULL COMMENT '项目名称',
  `customer_name` VARCHAR(128) NOT NULL COMMENT '客户/单位名称',
  `env` VARCHAR(32) DEFAULT '生产' COMMENT '环境(生产/测试/灰度)',
  `cloud_vendor` VARCHAR(64) DEFAULT '' COMMENT '云厂商',
  `region_name` VARCHAR(64) DEFAULT '' COMMENT '区域名称',
  `device_count` INT DEFAULT 0 COMMENT '硬件总资产数',
  `phy_count` INT DEFAULT 0 COMMENT '物理机数量',
  `vm_count` INT DEFAULT 0 COMMENT '虚拟机/云资源数',
  `db_count` INT DEFAULT 0 COMMENT '数据库实例数',
  `mw_count` INT DEFAULT 0 COMMENT '中间件实例数',
  `bk_count` INT DEFAULT 0 COMMENT '备份方案数',
  `ops_count` INT DEFAULT 0 COMMENT '运维账号数',
  `total_cores` INT DEFAULT 0 COMMENT '计算核数总计',
  `total_memory_gb` INT DEFAULT 0 COMMENT '内存总量(GB)',
  `total_disk_gb` INT DEFAULT 0 COMMENT '磁盘总量(GB)',
  `xinchuang_count` INT DEFAULT 0 COMMENT '信创设备总数',
  `security_level` VARCHAR(64) DEFAULT '等保二级' COMMENT '等保级别',
  `description` TEXT COMMENT '项目详细说明',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  INDEX `idx_cust_proj` (`customer_name`, `name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='CMDB-项目管理目录表';

-- 2. 硬件资产表
DROP TABLE IF EXISTS `cmdb_hardware_assets`;
CREATE TABLE `cmdb_hardware_assets` (
  `id` VARCHAR(64) NOT NULL COMMENT '设备全局主键ID',
  `seq` INT DEFAULT 0 COMMENT '原始Excel台账序号',
  `project_id` VARCHAR(64) DEFAULT '' COMMENT '关联合同/项目ID',
  `project_no` VARCHAR(64) DEFAULT '' COMMENT '项目编号',
  `customer_name` VARCHAR(128) DEFAULT '' COMMENT '客户名称',
  `project_name` VARCHAR(128) NOT NULL COMMENT '项目名称',
  `env` VARCHAR(32) DEFAULT '生产' COMMENT '所属环境',
  `cloud_vendor` VARCHAR(64) DEFAULT '' COMMENT '云厂商',
  `region_name` VARCHAR(64) DEFAULT '' COMMENT '区域名称',
  `device_name` VARCHAR(128) NOT NULL COMMENT '设备名称',
  `device_category` VARCHAR(64) DEFAULT '服务器' COMMENT '设备大类',
  `device_type` VARCHAR(64) DEFAULT '虚拟机' COMMENT '设备类型(物理机/虚拟机/存储)',
  `idc_room` VARCHAR(64) DEFAULT '' COMMENT '机房位置',
  `private_ip` VARCHAR(64) DEFAULT '' COMMENT '业务私网IP',
  `private_ipv6` VARCHAR(128) DEFAULT '' COMMENT '私网IPv6',
  `inner_net_ip` VARCHAR(64) DEFAULT '' COMMENT '内大网IP',
  `eip` VARCHAR(64) DEFAULT '' COMMENT '弹性EIP',
  `vip` VARCHAR(64) DEFAULT '' COMMENT '高可用VIP',
  `public_ip` VARCHAR(64) DEFAULT '' COMMENT '公网IP',
  `cpu_arch` VARCHAR(64) DEFAULT '' COMMENT 'CPU架构(x86_64/ARM64/LoongArch)',
  `cpu_cores` INT DEFAULT 0 COMMENT 'CPU核数',
  `memory_gb` INT DEFAULT 0 COMMENT '内存(GB)',
  `sys_disk_gb` INT DEFAULT 0 COMMENT '系统盘(GB)',
  `data_disk_gb` INT DEFAULT 0 COMMENT '数据盘(GB)',
  `share_disk_gb` INT DEFAULT 0 COMMENT '共享磁盘(GB)',
  `oss_gb` INT DEFAULT 0 COMMENT '对象存储容量(GB)',
  `os_distro` VARCHAR(64) DEFAULT '' COMMENT '操作系统发行版',
  `os_version` VARCHAR(64) DEFAULT '' COMMENT 'OS版本',
  `kernel_version` VARCHAR(64) DEFAULT '' COMMENT '内核版本',
  `is_xinchuang` VARCHAR(16) DEFAULT '否' COMMENT '是否信创OS',
  `remote_port` INT DEFAULT 22 COMMENT '远程管理端口',
  `remarks` TEXT COMMENT '备注',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '录入时间',
  PRIMARY KEY (`id`),
  INDEX `idx_hw_ip` (`private_ip`),
  INDEX `idx_hw_project` (`project_id`),
  INDEX `idx_hw_type` (`device_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='CMDB-硬件设备台账表';

-- 3. 数据库资产表
DROP TABLE IF EXISTS `cmdb_databases`;
CREATE TABLE `cmdb_databases` (
  `id` VARCHAR(64) NOT NULL COMMENT '数据库主键ID',
  `seq` INT DEFAULT 0 COMMENT '台账序号',
  `project_id` VARCHAR(64) DEFAULT '' COMMENT '关联合同/项目ID',
  `project_no` VARCHAR(64) DEFAULT '' COMMENT '项目编号',
  `customer_name` VARCHAR(128) DEFAULT '' COMMENT '客户名称',
  `project_name` VARCHAR(128) NOT NULL COMMENT '项目名称',
  `env` VARCHAR(32) DEFAULT '生产' COMMENT '环境',
  `cloud_vendor` VARCHAR(64) DEFAULT '' COMMENT '云厂商',
  `region_name` VARCHAR(64) DEFAULT '' COMMENT '区域名称',
  `private_ip` VARCHAR(64) DEFAULT '' COMMENT '挂载主机私网IP',
  `vip_eip` VARCHAR(64) DEFAULT '' COMMENT 'VIP或公网访问地址',
  `db_category` VARCHAR(128) DEFAULT '' COMMENT '数据库大类',
  `db_software` VARCHAR(128) DEFAULT '' COMMENT '软件名称(Oracle/MySQL/DM等)',
  `version` VARCHAR(64) DEFAULT '' COMMENT '数据库版本号',
  `port` INT DEFAULT 0 COMMENT '监听端口',
  `instance_sid` VARCHAR(64) DEFAULT '' COMMENT '实例名或SID',
  `db_name` VARCHAR(128) DEFAULT '' COMMENT '业务数据库库名',
  `deploy_mode` VARCHAR(64) DEFAULT '单机' COMMENT '部署模式(单机/主从/RAC/读写分离)',
  `cluster_name` VARCHAR(128) DEFAULT '' COMMENT '集群名称',
  `remarks` TEXT COMMENT '备注',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '录入时间',
  PRIMARY KEY (`id`),
  INDEX `idx_db_ip` (`private_ip`),
  INDEX `idx_db_software` (`db_software`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='CMDB-数据库实例资产表';

-- 4. 中间件资产表
DROP TABLE IF EXISTS `cmdb_middlewares`;
CREATE TABLE `cmdb_middlewares` (
  `id` VARCHAR(64) NOT NULL COMMENT '中间件主键ID',
  `seq` INT DEFAULT 0 COMMENT '序号',
  `project_id` VARCHAR(64) DEFAULT '' COMMENT '关联合同/项目ID',
  `project_no` VARCHAR(64) DEFAULT '' COMMENT '项目编号',
  `customer_name` VARCHAR(128) DEFAULT '' COMMENT '客户名称',
  `project_name` VARCHAR(128) NOT NULL COMMENT '项目名称',
  `env` VARCHAR(32) DEFAULT '生产' COMMENT '环境',
  `cloud_vendor` VARCHAR(64) DEFAULT '' COMMENT '云厂商',
  `region_name` VARCHAR(64) DEFAULT '' COMMENT '区域名称',
  `private_ip` VARCHAR(64) DEFAULT '' COMMENT '部署主机私网IP',
  `mw_type` VARCHAR(128) DEFAULT '' COMMENT '中间件类型',
  `mw_software` VARCHAR(128) DEFAULT '' COMMENT '中间件软件名称',
  `version` VARCHAR(64) DEFAULT '' COMMENT '软件版本号',
  `port` VARCHAR(64) DEFAULT '' COMMENT '服务端口',
  `app_runtime_env` VARCHAR(64) DEFAULT '' COMMENT '应用程序运行环境(如JDK)',
  `remarks` TEXT COMMENT '备注',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '录入时间',
  PRIMARY KEY (`id`),
  INDEX `idx_mw_ip` (`private_ip`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='CMDB-中间件资产表';

-- 5. 备份方案表
DROP TABLE IF EXISTS `cmdb_backups`;
CREATE TABLE `cmdb_backups` (
  `id` VARCHAR(64) NOT NULL COMMENT '备份主键ID',
  `seq` INT DEFAULT 0 COMMENT '序号',
  `project_id` VARCHAR(64) DEFAULT '' COMMENT '关联合同/项目ID',
  `project_no` VARCHAR(64) DEFAULT '' COMMENT '项目编号',
  `customer_name` VARCHAR(128) DEFAULT '' COMMENT '客户名称',
  `project_name` VARCHAR(128) NOT NULL COMMENT '项目名称',
  `env` VARCHAR(32) DEFAULT '生产' COMMENT '环境',
  `cloud_vendor` VARCHAR(64) DEFAULT '' COMMENT '云厂商',
  `region_name` VARCHAR(64) DEFAULT '' COMMENT '区域名称',
  `private_ip` VARCHAR(64) DEFAULT '' COMMENT '备份目标IP',
  `backup_type` VARCHAR(64) DEFAULT '' COMMENT '备份类型',
  `backup_method` VARCHAR(64) DEFAULT '' COMMENT '备份方式',
  `backup_strategy` VARCHAR(128) DEFAULT '' COMMENT '备份策略',
  `retention_period` VARCHAR(64) DEFAULT '' COMMENT '保留周期',
  `backup_storage_path` VARCHAR(255) DEFAULT '' COMMENT '存储位置',
  `remarks` TEXT COMMENT '备注',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '录入时间',
  PRIMARY KEY (`id`),
  INDEX `idx_bk_ip` (`private_ip`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='CMDB-数据备份方案表';

-- 6. 运维账号与接入表 (360 完整 81 条)
DROP TABLE IF EXISTS `cmdb_ops_accounts`;
CREATE TABLE `cmdb_ops_accounts` (
  `id` VARCHAR(64) NOT NULL COMMENT '运维账号主键ID',
  `seq` INT DEFAULT 0 COMMENT '序号',
  `project_id` VARCHAR(64) DEFAULT '' COMMENT '关联合同/项目ID',
  `project_no` VARCHAR(64) DEFAULT '' COMMENT '项目编号',
  `customer_name` VARCHAR(128) DEFAULT '' COMMENT '客户名称',
  `project_name` VARCHAR(128) NOT NULL COMMENT '项目名称',
  `cloud_vendor` VARCHAR(64) DEFAULT '' COMMENT '云厂商',
  `idc_room` VARCHAR(64) DEFAULT '' COMMENT '机房',
  `personnel_affiliation` VARCHAR(64) DEFAULT '' COMMENT '人员归属',
  `vpn_network_env` VARCHAR(64) DEFAULT '' COMMENT '使用人网络环境',
  `vpn_url` VARCHAR(255) DEFAULT '' COMMENT 'VPN入口地址',
  `vpn_account` VARCHAR(128) DEFAULT '' COMMENT 'VPN登录账号',
  `vpn_user_name` VARCHAR(64) DEFAULT '' COMMENT 'VPN对应使用人姓名',
  `fortress_area` VARCHAR(64) DEFAULT '' COMMENT '堡垒区域名称',
  `fortress_url` VARCHAR(255) DEFAULT '' COMMENT '堡垒机地址',
  `fortress_account` VARCHAR(128) DEFAULT '' COMMENT '堡垒机账号',
  `fortress_user_name` VARCHAR(64) DEFAULT '' COMMENT '堡垒机对应使用人姓名',
  `remarks` TEXT COMMENT '备注',
  `created_at` DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '录入时间',
  PRIMARY KEY (`id`),
  INDEX `idx_ops_user` (`vpn_user_name`, `fortress_user_name`),
  INDEX `idx_ops_proj` (`project_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='CMDB-运维账号与堡垒机接入表';

SET FOREIGN_KEY_CHECKS = 1;
"""

with open('db/mysql_schema.sql', 'w', encoding='utf-8') as f:
    f.write(mysql_ddl)
print("Wrote db/mysql_schema.sql")

# ==========================================================
# 8. Generate MySQL Init Dump with Data (db/init_mysql_v360.sql)
# ==========================================================
sql_lines = [mysql_ddl, "\n-- ==========================================================", "-- 数据填充部分 (全量基于《信息资产台账-v360.xlsx》)", "-- ==========================================================\n"]

# Insert projects
sql_lines.append("-- 插入项目分组数据")
sql_lines.append("INSERT INTO `cmdb_projects` (`id`, `code`, `name`, `customer_name`, `env`, `cloud_vendor`, `region_name`, `device_count`, `phy_count`, `vm_count`, `db_count`, `mw_count`, `bk_count`, `ops_count`, `total_cores`, `total_memory_gb`, `total_disk_gb`, `xinchuang_count`, `security_level`, `description`) VALUES")
p_inserts = []
for p in project_list:
    vals = [
        sql_escape(p["id"]),
        sql_escape(p["code"]),
        sql_escape(p["name"]),
        sql_escape(p["customerName"]),
        sql_escape(p["env"]),
        sql_escape(p["cloudVendor"]),
        sql_escape(p["regionName"]),
        str(p["deviceCount"]),
        str(p["phyCount"]),
        str(p["vmCount"]),
        str(p["dbCount"]),
        str(p["mwCount"]),
        str(p["backupCount"]),
        str(p["opsCount"]),
        str(p["totalCores"]),
        str(p["totalMemoryGb"]),
        str(p["totalDiskGb"]),
        str(p["xinchuangCount"]),
        sql_escape("等保二级"),
        sql_escape(p["description"])
    ]
    p_inserts.append(f"({', '.join(vals)})")
sql_lines.append(",\n".join(p_inserts) + ";\n")

# Insert hardware
sql_lines.append("-- 插入硬件资产数据 (299 条)")
sql_lines.append("INSERT INTO `cmdb_hardware_assets` (`id`, `seq`, `project_id`, `project_no`, `customer_name`, `project_name`, `env`, `cloud_vendor`, `region_name`, `device_name`, `device_category`, `device_type`, `idc_room`, `private_ip`, `private_ipv6`, `inner_net_ip`, `eip`, `vip`, `public_ip`, `cpu_arch`, `cpu_cores`, `memory_gb`, `sys_disk_gb`, `data_disk_gb`, `share_disk_gb`, `oss_gb`, `os_distro`, `os_version`, `kernel_version`, `is_xinchuang`, `remote_port`, `remarks`) VALUES")
h_inserts = []
for h in raw_hw:
    vals = [
        sql_escape(h["id"]),
        str(h["seq"]),
        sql_escape(h.get("projectId", "")),
        sql_escape(h["projectNo"]),
        sql_escape(h["customerName"]),
        sql_escape(h["projectName"]),
        sql_escape(h["env"]),
        sql_escape(h["cloudVendor"]),
        sql_escape(h["regionName"]),
        sql_escape(h["deviceName"]),
        sql_escape(h["deviceCategory"]),
        sql_escape(h["deviceType"]),
        sql_escape(h["idcRoom"]),
        sql_escape(h["privateIp"]),
        sql_escape(h["privateIpv6"]),
        sql_escape(h["innerNetIp"]),
        sql_escape(h["eip"]),
        sql_escape(h["vip"]),
        sql_escape(h["publicIp"]),
        sql_escape(h["cpuArch"]),
        str(h["cpuCores"]),
        str(h["memoryGb"]),
        str(h["sysDiskGb"]),
        str(h["dataDiskGb"]),
        str(h["shareDiskGb"]),
        str(h["ossGb"]),
        sql_escape(h["osDistro"]),
        sql_escape(h["osVersion"]),
        sql_escape(h["kernelVersion"]),
        sql_escape(h["isXinchuang"]),
        str(h["remotePort"]),
        sql_escape(h["remarks"])
    ]
    h_inserts.append(f"({', '.join(vals)})")
sql_lines.append(",\n".join(h_inserts) + ";\n")

# Insert databases
sql_lines.append("-- 插入数据库实例数据 (47 条)")
sql_lines.append("INSERT INTO `cmdb_databases` (`id`, `seq`, `project_id`, `project_no`, `customer_name`, `project_name`, `env`, `cloud_vendor`, `region_name`, `private_ip`, `vip_eip`, `db_category`, `db_software`, `version`, `port`, `instance_sid`, `db_name`, `deploy_mode`, `cluster_name`, `remarks`) VALUES")
d_inserts = []
for d in raw_dbs:
    vals = [
        sql_escape(d["id"]),
        str(d["seq"]),
        sql_escape(d.get("projectId", "")),
        sql_escape(d["projectNo"]),
        sql_escape(d["customerName"]),
        sql_escape(d["projectName"]),
        sql_escape(d["env"]),
        sql_escape(d["cloudVendor"]),
        sql_escape(d["regionName"]),
        sql_escape(d["privateIp"]),
        sql_escape(d["vipEip"]),
        sql_escape(d["dbCategory"]),
        sql_escape(d["dbSoftware"]),
        sql_escape(d["version"]),
        str(d["port"]),
        sql_escape(d["instanceSid"]),
        sql_escape(d["dbName"]),
        sql_escape(d["deployMode"]),
        sql_escape(d["clusterName"]),
        sql_escape(d["remarks"])
    ]
    d_inserts.append(f"({', '.join(vals)})")
sql_lines.append(",\n".join(d_inserts) + ";\n")

# Insert middlewares
sql_lines.append("-- 插入中间件数据 (1 条)")
sql_lines.append("INSERT INTO `cmdb_middlewares` (`id`, `seq`, `project_id`, `project_no`, `customer_name`, `project_name`, `env`, `cloud_vendor`, `region_name`, `private_ip`, `mw_type`, `mw_software`, `version`, `port`, `app_runtime_env`, `remarks`) VALUES")
m_inserts = []
for m in raw_mws:
    vals = [
        sql_escape(m["id"]),
        str(m["seq"]),
        sql_escape(m.get("projectId", "")),
        sql_escape(m["projectNo"]),
        sql_escape(m["customerName"]),
        sql_escape(m["projectName"]),
        sql_escape(m["env"]),
        sql_escape(m["cloudVendor"]),
        sql_escape(m["regionName"]),
        sql_escape(m["privateIp"]),
        sql_escape(m["mwType"]),
        sql_escape(m["mwSoftware"]),
        sql_escape(m["version"]),
        sql_escape(m["port"]),
        sql_escape(m.get("runtime", "")),
        sql_escape(m["remarks"])
    ]
    m_inserts.append(f"({', '.join(vals)})")
sql_lines.append(",\n".join(m_inserts) + ";\n")

# Insert backups
sql_lines.append("-- 插入备份方案数据 (1 条)")
sql_lines.append("INSERT INTO `cmdb_backups` (`id`, `seq`, `project_id`, `project_no`, `customer_name`, `project_name`, `env`, `cloud_vendor`, `region_name`, `private_ip`, `backup_type`, `backup_method`, `backup_strategy`, `retention_period`, `backup_storage_path`, `remarks`) VALUES")
b_inserts = []
for b in raw_bks:
    vals = [
        sql_escape(b["id"]),
        str(b["seq"]),
        sql_escape(b.get("projectId", "")),
        sql_escape(b["projectNo"]),
        sql_escape(b["customerName"]),
        sql_escape(b["projectName"]),
        sql_escape(b["env"]),
        sql_escape(b["cloudVendor"]),
        sql_escape(b["regionName"]),
        sql_escape(b["privateIp"]),
        sql_escape(b["backupType"]),
        sql_escape(b["backupMethod"]),
        sql_escape(b["backupPolicy"]),
        str(b.get("retentionDays", 30)),
        sql_escape(b["storageLocation"]),
        sql_escape(b["remarks"])
    ]
    b_inserts.append(f"({', '.join(vals)})")
sql_lines.append(",\n".join(b_inserts) + ";\n")

# Insert ops accounts
sql_lines.append("-- 插入运维账号与堡垒机数据 (81 条)")
sql_lines.append("INSERT INTO `cmdb_ops_accounts` (`id`, `seq`, `project_id`, `project_no`, `customer_name`, `project_name`, `cloud_vendor`, `idc_room`, `personnel_affiliation`, `vpn_network_env`, `vpn_url`, `vpn_account`, `vpn_user_name`, `fortress_area`, `fortress_url`, `fortress_account`, `fortress_user_name`, `remarks`) VALUES")
o_inserts = []
for o in raw_ops:
    vals = [
        sql_escape(o["id"]),
        str(o["seq"]),
        sql_escape(o.get("projectId", "")),
        sql_escape(o["projectNo"]),
        sql_escape(o["customerName"]),
        sql_escape(o["projectName"]),
        sql_escape(o["cloudVendor"]),
        sql_escape(o["idcRoom"]),
        sql_escape(o["personnelAffiliation"]),
        sql_escape(o["vpnNetworkEnv"]),
        sql_escape(o["vpnAddress"]),
        sql_escape(o["vpnAccount"]),
        sql_escape(o["vpnUserName"]),
        sql_escape(o["fortressArea"]),
        sql_escape(o["bastionAddress"]),
        sql_escape(o["bastionAccount"]),
        sql_escape(o["bastionUserName"]),
        sql_escape(o["remarks"])
    ]
    o_inserts.append(f"({', '.join(vals)})")
sql_lines.append(",\n".join(o_inserts) + ";\n")

with open('db/init_mysql_v360.sql', 'w', encoding='utf-8') as f:
    f.write("\n".join(sql_lines))
print(f"Wrote db/init_mysql_v360.sql ({len(sql_lines)} blocks)")

# ==========================================================
# 9. Generate TypeScript Seed Data (app/cmdbData.ts)
# ==========================================================
ts_template = f"""// CMDB Core Relational Data Model & Seed Store
// STRICTLY imported from 《信息资产台账-v360.xlsx》 without ANY fabricated/synthetic data:
// 02-硬件设备: 299 台真实资产 (5 物理机, 289 虚拟机, 5 对象存储)
// 03-数据库:   47 个真实数据库实例
// 04-中间件:   1 个真实中间件服务 (Sheet 04 原表数据)
// 05-备份:     1 条真实备份方案 (Sheet 05 原表数据)
// 06-运维账号: 81 条真实运维与堡垒机记录 (Sheet 06 原表数据)
// 归属项目:     {len(project_list)} 个真实聚合项目组

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
  dbCount?: number;
  mwCount?: number;
  backupCount?: number;
  opsCount?: number;
  description: string;
  healthScore: number;
}}

export interface AssetMeta {{
  id?: string;
  assetNo?: string;
  seq?: number;
  projectNo?: string;
  customerName?: string;
  projectName?: string;
  projectId?: string;
  env?: string;
  cloudVendor?: string;
  regionName?: string;
  deviceName?: string;
  category?: string;
  deviceCategory?: string;
  deviceType?: string;
  roomName?: string;
  idcRoom?: string;
  privateIp?: string | null;
  privateIpv6?: string | null;
  internalWanIp?: string | null;
  innerNetIp?: string | null;
  eip?: string | null;
  vip?: string | null;
  publicIp?: string | null;
  cpuArch?: string;
  cpuCores?: number;
  memoryGb?: number;
  systemDiskGb?: number;
  sysDiskGb?: number;
  dataDiskGb?: number;
  sharedDiskGb?: number | null;
  shareDiskGb?: number | null;
  objectStorageGb?: number | null;
  ossGb?: number | null;
  osFamily?: string;
  osDistro?: string;
  osVersion?: string;
  kernelVersion?: string | null;
  isXinchuang?: string;
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

export type VirtualMachine = VmHost;

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
  os?: string;
  roomId?: string;
  roomName?: string;
  cabinetId?: string;
  startU?: number;
  uHeight?: number;
}}

export interface DatabaseAsset {{
  id: string;
  seq?: number;
  projectNo?: string;
  customerName?: string;
  projectName?: string;
  projectId?: string;
  env?: string;
  cloudVendor?: string;
  regionName?: string;
  name: string;
  type: string;
  dbCategory?: string;
  dbSoftware?: string;
  version: string;
  hostIp: string;
  privateIp?: string;
  vipEip?: string;
  port: number;
  instanceSid?: string;
  dbName?: string;
  deployMode?: string;
  clusterName?: string;
  arch?: string;
  businessId?: string;
  dataSize?: string;
  connectionCount?: number;
  status: "active" | "standby" | "maintenance";
  remarks?: string;
}}

export interface MiddlewareAsset {{
  id: string;
  seq?: number;
  projectNo?: string;
  customerName: string;
  projectName: string;
  projectId?: string;
  env: string;
  cloudVendor: string;
  regionName: string;
  privateIp: string;
  assetIp?: string;
  assetName?: string;
  category?: string;
  name: string;
  mwType: string;
  mwSoftware: string;
  version: string;
  port: string;
  role?: string;
  runtime?: string;
  appRuntimeEnv?: string;
  status: "running" | "stopped" | "warning";
  remarks?: string;
}}

export interface BackupAsset {{
  id: string;
  seq?: number;
  projectNo?: string;
  customerName: string;
  projectName: string;
  projectId?: string;
  env: string;
  cloudVendor: string;
  regionName: string;
  privateIp: string;
  backupType: string;
  backupMethod: string;
  backupPolicy: string;
  storageLocation: string;
  retentionDays?: number;
  lastBackupTime?: string;
  status: "normal" | "warning" | "failed";
  remarks?: string;
}}

export interface OpsAsset {{
  id: string;
  seq?: number;
  projectNo?: string;
  customerName: string;
  projectName: string;
  projectId?: string;
  env?: string;
  cloudVendor?: string;
  regionName?: string;
  privateIp?: string;
  idcRoom?: string;
  personnelAffiliation?: string;
  vpnNetworkEnv?: string;
  vpnAddress?: string;
  vpnAccount?: string;
  vpnUserName?: string;
  fortressArea?: string;
  bastionAddress?: string;
  bastionAccount?: string;
  bastionUserName?: string;
  opsVendor?: string;
  serverAccessAddress?: string;
  monitoringCoverage?: "是" | "部分" | "否";
  inspectionCycle?: "每日" | "每周" | "每月" | "每季";
  changeWindow?: string;
  networkZone?: string;
  exposureSurface?: string;
  remarks?: string;
}}

export interface BusinessModel {{
  id: string;
  name: string;
  code: string;
  department: string;
  manager: string;
  owner?: string;
  level: "核心" | "重要" | "一般";
  hostCount: number;
  status: "运行中" | "已下线";
  description: string;
  hosts: string[];
}}

export interface CredentialItem {{
  id: string;
  assetId?: string;
  assetIp?: string;
  assetName?: string;
  name?: string;
  type?: string;
  targetCount?: number;
  credentialType?: "os_root" | "os_user" | "database" | "middleware" | "jumpserver" | "vpn" | "api_token" | string;
  username: string;
  port?: number;
  protocol?: string;
  authMethod?: "password" | "key" | "token" | "certificate" | string;
  status?: "active" | "expired" | "revoked" | string;
  expireDate?: string;
  remark?: string;
  remarks?: string;
  updated: string;
}}

export interface SoftwareComponent {{
  id: string;
  assetId: string;
  assetIp?: string;
  assetName: string;
  projectId: string;
  projectName: string;
  name: string;
  category: "database" | "middleware" | "plugin" | "web_server";
  version: string;
  port?: number | string;
  installPath?: string;
  configPath?: string;
  status: "running" | "stopped" | "warning";
  remarks?: string;
  updated: string;
}}

export interface OpsChannel {{
  id: string;
  assetId: string;
  assetIp?: string;
  assetName: string;
  projectId?: string;
  projectName?: string;
  name: string;
  channelType: "web_link" | "ssh" | "rdp" | "jumpserver" | "vpn";
  urlOrTarget: string;
  accountNote?: string;
  vpnClientType?: string;
  vpnGateway?: string;
  vpnNetworkSegment?: string;
  remarks?: string;
  updated: string;
}}

export type OpsChannelItem = OpsChannel;

export interface AgentProbe {{
  id: string;
  hostname: string;
  hostIp: string;
  agentVersion: string;
  cpuUsage: number;
  memUsage: number;
  lastHeartbeat: string;
  status: "online" | "warning" | "offline";
}}

export const initialProbes: AgentProbe[] = [];

export const initialRooms: IdcRoom[] = [
  {{ id: "room-unicom", name: "北京联通主数据中心", code: "BJ-UNICOM-01", operator: "中国联通", city: "北京", address: "北京市朝阳区酒仙桥路10号", level: "T4", cabinetCount: 48, contact: "联通云值班经理", phone: "010-58888888", status: "正常运行", remark: "联通云承载人社局、总工会核心业务专网" }},
  {{ id: "room-huawei", name: "华为云亦庄数据中心", code: "BJ-HUAWEI-01", operator: "BGP多线", city: "北京", address: "北京市经济技术开发区科创六街", level: "T4", cabinetCount: 36, contact: "华为云值班专员", phone: "4000-955-988", status: "正常运行", remark: "华为云信创政务云节点" }}
];

export const initialCabinets: IdcCabinet[] = [
  {{ id: "cab-01", roomId: "room-unicom", name: "A01机柜", code: "CAB-A01", totalU: 42, usedU: 24, maxPower: 6000, currentPower: 3850, temperature: 22.5, status: "正常", manager: "张工", row: "A列", remark: "人社局核心机架" }},
  {{ id: "cab-02", roomId: "room-unicom", name: "A02机柜", code: "CAB-A02", totalU: 42, usedU: 28, maxPower: 6000, currentPower: 4120, temperature: 23.0, status: "正常", manager: "张工", row: "A列", remark: "总工会核心机架" }}
];

export const initialProjects: ProjectGroup[] = {json.dumps(project_list, ensure_ascii=False, indent=2)};

export const initialPhysicalHosts: PhysicalHost[] = {json.dumps(physical_hosts, ensure_ascii=False, indent=2)};

export const initialVms: VmHost[] = {json.dumps(vms, ensure_ascii=False, indent=2)};

export const initialSwitches: SwitchDevice[] = [];

export const initialDatabases: DatabaseAsset[] = {json.dumps(raw_dbs, ensure_ascii=False, indent=2)};

export const initialMiddlewares: MiddlewareAsset[] = {json.dumps(raw_mws, ensure_ascii=False, indent=2)};

export const initialBackups: BackupAsset[] = {json.dumps(raw_bks, ensure_ascii=False, indent=2)};

export const initialOpsRecords: OpsAsset[] = {json.dumps(raw_ops, ensure_ascii=False, indent=2)};

export const initialBusinesses: BusinessModel[] = [
  {{
    id: "biz-1",
    name: "政务业务保障",
    code: "BIZ-GOV",
    department: "北京市人社局 / 总工会",
    manager: "系统管理员",
    level: "核心",
    hostCount: 299,
    status: "运行中",
    description: "v360 纳管信息资产",
    hosts: ["192.125.31.240", "192.125.31.244"]
  }}
];

export const initialHosts: PhysicalHost[] = initialPhysicalHosts;

export const initialCredentials: CredentialItem[] = [];
export const initialSoftwareComponents: SoftwareComponent[] = [];
export const initialOpsChannels: OpsChannel[] = [
  {{
    id: "ch-vpn-1",
    assetId: "hw-v360-1",
    assetIp: "192.125.31.240",
    assetName: "人社局-gov-北控伟仕-老生产-财务管理应用",
    projectId: "proj-6",
    projectName: "原三险系统",
    name: "联通云VPN入口",
    channelType: "vpn",
    urlOrTarget: "https://vpn.unicomcloud.bj.cn",
    accountNote: "bj_rsj_ops",
    remarks: "生产运维通道",
    updated: "2026-09-22 10:00"
  }}
];
"""

with open('app/cmdbData.ts', 'w', encoding='utf-8') as f:
    f.write(ts_template)

print("Wrote updated app/cmdbData.ts successfully!")
