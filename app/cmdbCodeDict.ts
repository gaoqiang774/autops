// ==========================================================================
// AutoOps CMDB - 核心代码字典 (严格来源于《信息资产台账-v361.xlsx》的【00-代码表】)
// 全系统所有下拉选择框严格按照 Excel 代码表取数
// ==========================================================================

/** 00-代码表: 客户名称 (14 项) */
export const EXCEL_CUSTOMERS = [
  "北京市人力资源和社会保障局",
  "北京市退役军人事务局",
  "北京市职工互助保障服务中心",
  "北京市总工会职工服务中心",
  "国家税务总局北京市税务局",
  "北京燃气/北控伟仕",
  "北控集团",
  "北控环境",
  "北控智科",
  "北控三兴",
  "北控数科",
  "北控北斗",
  "北控伟仕",
  "北控天兴"
] as const;

/** 00-代码表: 项目名称 (27 项) */
export const EXCEL_PROJECTS = [
  "原三险系统",
  "原工伤认定系统",
  "原城居系统",
  "仲裁核心系统",
  "仲裁云庭审系统",
  "监管中台/工伤红名单/即席查询",
  "监管中台、工伤红名单、即席查询",
  "数转区",
  "仲裁网厅系统",
  "调解仲裁系统",
  "退军补贴发放系统",
  "退军资金一体化监管-企转系统",
  "工会互助保险信息系统",
  "工会职服数智化系统",
  "税务年金征收系统",
  "燃气内管/高山监管平台/北控资产管理",
  "北控集团商城平台",
  "北控集团招采平台",
  "北控环境招采系统",
  "智科精益管理平台",
  "三兴精益管理平台",
  "智科官网",
  "数科精益管理平台",
  "北斗精益管理平台",
  "伟仕精益管理平台/天兴工程服务器",
  "伟仕精益管理平台",
  "天兴工程服务器"
] as const;

/** 00-代码表: 环境 (3 项) */
export const EXCEL_ENVIRONMENTS = [
  "生产",
  "测试",
  "开发"
] as const;

/** 00-代码表: 云厂商 (7 项) */
export const EXCEL_CLOUD_VENDORS = [
  "联通云",
  "首信云",
  "太极云",
  "国企云",
  "阿里云",
  "腾讯云",
  "自建机房"
] as const;

/** 00-代码表: 区域名称 (4 项) */
export const EXCEL_REGION_NAMES = [
  "政务外网区",
  "互联网区",
  "内网区",
  "税务专网政务外网"
] as const;

/** 00-代码表: 设备大类 (6 项) */
export const EXCEL_DEVICE_CATEGORIES = [
  "服务器",
  "存储",
  "网络",
  "安全",
  "终端",
  "其他"
] as const;

/** 00-代码表: 设备类型 (16 项) */
export const EXCEL_DEVICE_TYPES = [
  "物理机",
  "虚拟机",
  "云主机",
  "刀片服务器",
  "小型机",
  "超融合节点",
  "磁盘阵列",
  "分布式存储",
  "交换机",
  "路由器",
  "防火墙",
  "负载均衡",
  "WAF",
  "堡垒机",
  "对象存储",
  "其他"
] as const;

/** 00-代码表: 机房 (9 项) */
export const EXCEL_IDC_ROOMS = [
  "六里桥机房",
  "酒仙桥机房",
  "通州C1机房",
  "亦庄国企云机房",
  "首信机房",
  "阿里云机房",
  "联通新云机房",
  "太极云机房",
  "税务机房"
] as const;

/** 00-代码表: OS家族 (12 项) */
export const EXCEL_OS_FAMILIES = [
  "麒麟",
  "统信UOS",
  "欧拉openEuler",
  "龙蜥Anolis",
  "CentOS",
  "Rocky Linux",
  "Ubuntu",
  "Windows Server",
  "AIX",
  "其他",
  "Red Hat",
  "Oracle Linux"
] as const;

/** 00-代码表: OS版本 (30 项) */
export const EXCEL_OS_VERSIONS = [
  "麒麟V10 SP3",
  "麒麟V10 SP1",
  "统信V20",
  "openEuler 22.03",
  "Anolis 8",
  "CentOS 7.9",
  "Rocky 8",
  "Rocky 9",
  "Ubuntu 22.04",
  "Windows Server 2019",
  "Windows Server 2022",
  "AIX 7.2",
  "其他",
  "Red Hat 6.9",
  "Red Hat 7.1",
  "Ubuntu 18.04",
  "Windows Server 2012",
  "Windows Server 2012 R2",
  "Windows Server 2016",
  "Oracle Linux 6.9",
  "Oracle Linux",
  "CentOS 8",
  "中标麒麟7",
  "CentOS 6.10",
  "CentOS 7.7",
  "Oracle Linux 6.10",
  "CentOS 7.8",
  "中标麒麟7.9",
  "uos-server-20-1060e-amd64",
  "ubuntu-24.04.3"
] as const;

/** 00-代码表: CPU架构 (6 项) */
export const EXCEL_CPU_ARCHS = [
  "x86_64",
  "ARM64",
  "LoongArch",
  "SW64",
  "POWER",
  "其他"
] as const;

/** 00-代码表: 数据库大类 (16 项) */
export const EXCEL_DB_CATEGORIES = [
  "关系型 (RDBMS / OLTP)",
  "分布式关系型 (Distributed SQL)",
  "分析型 / 数据仓库 (OLAP)",
  "键值型数据库 (Key-Value)",
  "关系型数据库",
  "分布式关系型数据库",
  "分析型数据库/数仓",
  "混合事务分析型数据库",
  "键值型数据库",
  "文档型数据库",
  "列族/宽表数据库",
  "时序数据库",
  "向量数据库",
  "图数据库",
  "搜索引擎与全文检索",
  "内存与嵌入式数据库"
] as const;

/** 00-代码表: 数据库软件 (69 项) */
export const EXCEL_DB_SOFTWARES = [
  "MySQL",
  "PostgreSQL",
  "Oracle Database",
  "Microsoft SQL Server",
  "达梦数据库 (DM)",
  "人大金仓 (KingbaseES)",
  "南大通用 (GBase 8s)",
  "万里安全 (GreatDB)",
  "瀚高数据库 (HighGo DB)",
  "OceanBase",
  "TiDB",
  "openGauss",
  "GaussDB",
  "TDSQL",
  "PolarDB-X",
  "GoldenDB",
  "CockroachDB",
  "YugabyteDB",
  "Greenplum Database (GP)",
  "Apache Hive",
  "ClickHouse",
  "Apache Doris",
  "StarRocks",
  "GBase 8a MPP",
  "Hashdata",
  "Snowflake",
  "Amazon Redshift",
  "TiDB (含 TiFlash 列存引擎)",
  "OceanBase (HTAP 混合负载引擎)",
  "SingleStore (MemSQL)",
  "PolarDB (IMCI 列存索引版)",
  "Redis",
  "Memcached",
  "Etcd",
  "RocksDB",
  "Aerospike",
  "MongoDB",
  "SequoiaDB (巨杉数据库)",
  "Couchbase",
  "Amazon DocumentDB",
  "Apache HBase",
  "Apache Cassandra",
  "ScyllaDB",
  "Google Cloud Bigtable",
  "TDengine (涛思数据)",
  "InfluxDB",
  "TimescaleDB",
  "IoTDB (Apache IoTDB)",
  "OpenTSDB",
  "Prometheus (TSDB 存储)",
  "Milvus",
  "Pinecone",
  "Qdrant",
  "Chroma",
  "Weaviate",
  "Neo4j",
  "NebulaGraph",
  "TuGraph",
  "TigerGraph",
  "HugeGraph",
  "Elasticsearch",
  "OpenSearch",
  "Apache Solr",
  "Meilisearch",
  "SQLite",
  "H2 Database",
  "DuckDB",
  "Berkeley DB",
  "MariaDB"
] as const;

/** 00-代码表: 库部署 (3 项) */
export const EXCEL_DB_DEPLOY_MODES = [
  "单机",
  "集群",
  "分布式"
] as const;

/** 00-代码表: 中间件类型 (10 项) */
export const EXCEL_MIDDLEWARE_TYPES = [
  "Web 服务器/反向代理",
  "应用服务器/Java Web 容器",
  "消息中间件/消息队列",
  "注册与配置中心",
  "API 网关/流量入口网关",
  "分布式任务调度中间件",
  "数据访问与分库分表中间件",
  "服务网格与 RPC 框架",
  "分布式事务中间件",
  "分布式文件传输中间件"
] as const;

/** 00-代码表: 中间件软件 (35 项) */
export const EXCEL_MIDDLEWARE_SOFTWARES = [
  "Nginx",
  "OpenResty",
  "Apache HTTP Server",
  "HAProxy",
  "Tengine",
  "Apache Tomcat",
  "TongWeb (东方通)",
  "BES AppServer (宝兰德)",
  "AAS (金蝶天燕)",
  "InforSuite (中创中间件)",
  "Oracle WebLogic",
  "IBM WebSphere (WAS)",
  "WildFly (JBoss)",
  "Eclipse Jetty",
  "Apache Kafka",
  "RabbitMQ",
  "RocketMQ",
  "EMQX",
  "IBM MQ",
  "Apache Pulsar",
  "TongLINK/Q (东方通)",
  "Apache ZooKeeper",
  "Nacos",
  "Consul",
  "Apache APISIX",
  "Kong",
  "Spring Cloud Gateway",
  "Traefik",
  "XXL-JOB",
  "PowerJob",
  "ShardingSphere",
  "MyCat",
  "Apache Dubbo",
  "Apache Seata",
  "TongGTP (东方通)"
] as const;

/** 00-代码表: 应用程序运行环境 (12 项) */
export const EXCEL_APP_RUNTIMES = [
  "JDK1.5",
  "JDK8",
  "JDK11",
  "JDK17",
  "JDK21",
  ".NET 4.x",
  ".NET 6+",
  "Python3",
  "Node.js",
  "PHP",
  "其他",
  "不适用"
] as const;

/** 00-代码表: 备份策略 (7 项) */
export const EXCEL_BACKUP_STRATEGIES = [
  "无",
  "每日逻辑备",
  "每日物理备",
  "物理+归档",
  "云快照",
  "集群多副本",
  "异地灾备"
] as const;

/** 00-代码表: 备份类型 (2 项) */
export const EXCEL_BACKUP_TYPES = [
  "数据库",
  "对象存储"
] as const;

/** 00-代码表: 备份方式 (3 项) */
export const EXCEL_BACKUP_METHODS = [
  "物理备份",
  "逻辑备份",
  "实时同步"
] as const;

/** 全量代码表字典集合 */
export const EXCEL_CODE_DICT = {
  "客户名称": EXCEL_CUSTOMERS,
  "项目名称": EXCEL_PROJECTS,
  "环境": EXCEL_ENVIRONMENTS,
  "云厂商": EXCEL_CLOUD_VENDORS,
  "区域名称": EXCEL_REGION_NAMES,
  "设备大类": EXCEL_DEVICE_CATEGORIES,
  "设备类型": EXCEL_DEVICE_TYPES,
  "机房": EXCEL_IDC_ROOMS,
  "OS家族": EXCEL_OS_FAMILIES,
  "OS版本": EXCEL_OS_VERSIONS,
  "CPU架构": EXCEL_CPU_ARCHS,
  "数据库大类": EXCEL_DB_CATEGORIES,
  "数据库软件": EXCEL_DB_SOFTWARES,
  "库部署": EXCEL_DB_DEPLOY_MODES,
  "中间件类型": EXCEL_MIDDLEWARE_TYPES,
  "中间件软件": EXCEL_MIDDLEWARE_SOFTWARES,
  "应用程序运行环境": EXCEL_APP_RUNTIMES,
  "备份策略": EXCEL_BACKUP_STRATEGIES,
  "备份类型": EXCEL_BACKUP_TYPES,
  "备份方式": EXCEL_BACKUP_METHODS,
};

/** 中间件类型与推荐软件关联表 (按照 00-代码表 归类) */
export const EXCEL_MW_TYPE_SOFTWARE_MAP: Record<string, string[]> = {
  "Web 服务器/反向代理": [
    "Nginx",
    "OpenResty",
    "Apache HTTP Server",
    "HAProxy",
    "Tengine"
  ],
  "应用服务器/Java Web 容器": [
    "TongWeb (东方通)",
    "BES AppServer (宝兰德)",
    "AAS (金蝶天燕)",
    "InforSuite (中创中间件)",
    "Apache Tomcat",
    "Oracle WebLogic",
    "IBM WebSphere (WAS)",
    "WildFly (JBoss)",
    "Eclipse Jetty"
  ],
  "消息中间件/消息队列": [
    "Apache Kafka",
    "RabbitMQ",
    "RocketMQ",
    "EMQX",
    "IBM MQ",
    "Apache Pulsar",
    "TongLINK/Q (东方通)"
  ],
  "注册与配置中心": [
    "Apache ZooKeeper",
    "Nacos",
    "Consul"
  ],
  "API 网关/流量入口网关": [
    "Apache APISIX",
    "Kong",
    "Spring Cloud Gateway",
    "Traefik"
  ],
  "分布式任务调度中间件": [
    "XXL-JOB",
    "PowerJob"
  ],
  "数据访问与分库分表中间件": [
    "ShardingSphere",
    "MyCat"
  ],
  "服务网格与 RPC 框架": [
    "Apache Dubbo"
  ],
  "分布式事务中间件": [
    "Apache Seata"
  ],
  "分布式文件传输中间件": [
    "TongGTP (东方通)"
  ]
};

/** 数据库大类与推荐软件关联表 (按照 00-代码表 归类) */
export const EXCEL_DB_CATEGORY_SOFTWARE_MAP: Record<string, string[]> = {
  "关系型 (RDBMS / OLTP)": [
    "MySQL",
    "PostgreSQL",
    "Oracle Database",
    "Microsoft SQL Server",
    "达梦数据库 (DM)",
    "人大金仓 (KingbaseES)",
    "南大通用 (GBase 8s)",
    "万里安全 (GreatDB)",
    "瀚高数据库 (HighGo DB)",
    "SQLite",
    "MariaDB"
  ],
  "分布式关系型 (Distributed SQL)": [
    "OceanBase",
    "TiDB",
    "openGauss",
    "GaussDB",
    "TDSQL",
    "PolarDB-X",
    "GoldenDB",
    "CockroachDB",
    "YugabyteDB"
  ],
  "分析型 / 数据仓库 (OLAP)": [
    "Greenplum Database (GP)",
    "Apache Hive",
    "ClickHouse",
    "Apache Doris",
    "StarRocks",
    "GBase 8a MPP",
    "Hashdata",
    "Snowflake",
    "Amazon Redshift"
  ],
  "混合事务分析型数据库": [
    "TiDB (含 TiFlash 列存引擎)",
    "OceanBase (HTAP 混合负载引擎)",
    "SingleStore (MemSQL)",
    "PolarDB (IMCI 列存索引版)"
  ],
  "键值型数据库 (Key-Value)": [
    "Redis",
    "Memcached",
    "Etcd",
    "RocksDB",
    "Aerospike"
  ],
  "文档型数据库": [
    "MongoDB",
    "SequoiaDB (巨杉数据库)",
    "Couchbase",
    "Amazon DocumentDB"
  ],
  "列族/宽表数据库": [
    "Apache HBase",
    "Apache Cassandra",
    "ScyllaDB",
    "Google Cloud Bigtable"
  ],
  "时序数据库": [
    "TDengine (涛思数据)",
    "InfluxDB",
    "TimescaleDB",
    "IoTDB (Apache IoTDB)",
    "OpenTSDB",
    "Prometheus (TSDB 存储)"
  ],
  "向量数据库": [
    "Milvus",
    "Pinecone",
    "Qdrant",
    "Chroma",
    "Weaviate"
  ],
  "图数据库": [
    "Neo4j",
    "NebulaGraph",
    "TuGraph",
    "TigerGraph",
    "HugeGraph"
  ],
  "搜索引擎与全文检索": [
    "Elasticsearch",
    "OpenSearch",
    "Apache Solr",
    "Meilisearch"
  ],
  "内存与嵌入式数据库": [
    "SQLite",
    "H2 Database",
    "DuckDB",
    "Berkeley DB"
  ]
};
