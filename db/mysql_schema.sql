-- ==========================================================
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
