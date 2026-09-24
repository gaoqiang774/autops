import openpyxl
import json
import sys

def main():
    sys.stdout.reconfigure(encoding='utf-8')
    wb = openpyxl.load_workbook('信息资产台账-v360.xlsx', data_only=True)
    ws = wb['00-代码表']
    headers = [ws.cell(4, c).value for c in range(1, ws.max_column + 1)]
    dict_data = {}
    for c_idx, h in enumerate(headers, start=1):
        if not h:
            continue
        h = str(h).strip()
        seen = []
        for r in range(5, ws.max_row + 1):
            v = ws.cell(r, c_idx).value
            if v is not None:
                s = str(v).strip()
                if s and s not in seen:
                    seen.append(s)
        dict_data[h] = seen

    var_map = {
        '客户名称': 'EXCEL_CUSTOMERS',
        '项目名称': 'EXCEL_PROJECTS',
        '环境': 'EXCEL_ENVIRONMENTS',
        '云厂商': 'EXCEL_CLOUD_VENDORS',
        '区域名称': 'EXCEL_REGION_NAMES',
        '设备大类': 'EXCEL_DEVICE_CATEGORIES',
        '设备类型': 'EXCEL_DEVICE_TYPES',
        '机房': 'EXCEL_IDC_ROOMS',
        'OS家族': 'EXCEL_OS_FAMILIES',
        'OS版本': 'EXCEL_OS_VERSIONS',
        'CPU架构': 'EXCEL_CPU_ARCHS',
        '数据库大类': 'EXCEL_DB_CATEGORIES',
        '数据库软件': 'EXCEL_DB_SOFTWARES',
        '库部署': 'EXCEL_DB_DEPLOY_MODES',
        '中间件类型': 'EXCEL_MIDDLEWARE_TYPES',
        '中间件软件': 'EXCEL_MIDDLEWARE_SOFTWARES',
        '应用程序运行环境': 'EXCEL_APP_RUNTIMES',
        '备份策略': 'EXCEL_BACKUP_STRATEGIES',
        '备份类型': 'EXCEL_BACKUP_TYPES',
        '备份方式': 'EXCEL_BACKUP_METHODS'
    }

    ts_lines = [
        '// ==========================================================================',
        '// AutoOps CMDB - 核心代码字典 (严格来源于《信息资产台账-v360.xlsx》的【00-代码表】)',
        '// 全系统所有下拉选择框严格按照 Excel 代码表取数',
        '// ==========================================================================\n'
    ]

    for k, v in dict_data.items():
        var_name = var_map.get(k, 'EXCEL_' + k)
        ts_lines.append(f'/** 00-代码表: {k} ({len(v)} 项) */')
        ts_lines.append(f'export const {var_name} = ' + json.dumps(v, ensure_ascii=False, indent=2) + ' as const;\n')

    ts_lines.append('/** 全量代码表字典集合 */')
    ts_lines.append('export const EXCEL_CODE_DICT = {')
    for k, v in dict_data.items():
        var_name = var_map.get(k, 'EXCEL_' + k)
        ts_lines.append(f'  "{k}": {var_name},')
    ts_lines.append('};\n')

    type_mw_map = {
        'Web 服务器/反向代理': ['Nginx', 'OpenResty', 'Apache HTTP Server', 'HAProxy', 'Tengine'],
        '应用服务器/Java Web 容器': [
            'TongWeb (东方通)', 'BES AppServer (宝兰德)', 'AAS (金蝶天燕)', 'InforSuite (中创中间件)',
            'Apache Tomcat', 'Oracle WebLogic', 'IBM WebSphere (WAS)', 'WildFly (JBoss)', 'Eclipse Jetty'
        ],
        '消息中间件/消息队列': [
            'Apache Kafka', 'RabbitMQ', 'RocketMQ', 'EMQX', 'IBM MQ', 'Apache Pulsar', 'TongLINK/Q (东方通)'
        ],
        '注册与配置中心': ['Apache ZooKeeper', 'Nacos', 'Consul'],
        'API 网关/流量入口网关': ['Apache APISIX', 'Kong', 'Spring Cloud Gateway', 'Traefik'],
        '分布式任务调度中间件': ['XXL-JOB', 'PowerJob'],
        '数据访问与分库分表中间件': ['ShardingSphere', 'MyCat'],
        '服务网格与 RPC 框架': ['Apache Dubbo'],
        '分布式事务中间件': ['Apache Seata'],
        '分布式文件传输中间件': ['TongGTP (东方通)']
    }
    ts_lines.append('/** 中间件类型与推荐软件关联表 (按照 00-代码表 归类) */')
    ts_lines.append('export const EXCEL_MW_TYPE_SOFTWARE_MAP: Record<string, string[]> = ' + json.dumps(type_mw_map, ensure_ascii=False, indent=2) + ';\n')

    type_db_map = {
        '关系型 (RDBMS / OLTP)': ['MySQL', 'PostgreSQL', 'Oracle Database', 'Microsoft SQL Server', '达梦数据库 (DM)', '人大金仓 (KingbaseES)', '南大通用 (GBase 8s)', '万里安全 (GreatDB)', '瀚高数据库 (HighGo DB)', 'SQLite', 'MariaDB'],
        '分布式关系型 (Distributed SQL)': ['OceanBase', 'TiDB', 'openGauss', 'GaussDB', 'TDSQL', 'PolarDB-X', 'GoldenDB', 'CockroachDB', 'YugabyteDB'],
        '分析型 / 数据仓库 (OLAP)': ['Greenplum Database (GP)', 'Apache Hive', 'ClickHouse', 'Apache Doris', 'StarRocks', 'GBase 8a MPP', 'Hashdata', 'Snowflake', 'Amazon Redshift'],
        '混合事务分析型数据库': ['TiDB (含 TiFlash 列存引擎)', 'OceanBase (HTAP 混合负载引擎)', 'SingleStore (MemSQL)', 'PolarDB (IMCI 列存索引版)'],
        '键值型数据库 (Key-Value)': ['Redis', 'Memcached', 'Etcd', 'RocksDB', 'Aerospike'],
        '文档型数据库': ['MongoDB', 'SequoiaDB (巨杉数据库)', 'Couchbase', 'Amazon DocumentDB'],
        '列族/宽表数据库': ['Apache HBase', 'Apache Cassandra', 'ScyllaDB', 'Google Cloud Bigtable'],
        '时序数据库': ['TDengine (涛思数据)', 'InfluxDB', 'TimescaleDB', 'IoTDB (Apache IoTDB)', 'OpenTSDB', 'Prometheus (TSDB 存储)'],
        '向量数据库': ['Milvus', 'Pinecone', 'Qdrant', 'Chroma', 'Weaviate'],
        '图数据库': ['Neo4j', 'NebulaGraph', 'TuGraph', 'TigerGraph', 'HugeGraph'],
        '搜索引擎与全文检索': ['Elasticsearch', 'OpenSearch', 'Apache Solr', 'Meilisearch'],
        '内存与嵌入式数据库': ['SQLite', 'H2 Database', 'DuckDB', 'Berkeley DB']
    }
    ts_lines.append('/** 数据库大类与推荐软件关联表 (按照 00-代码表 归类) */')
    ts_lines.append('export const EXCEL_DB_CATEGORY_SOFTWARE_MAP: Record<string, string[]> = ' + json.dumps(type_db_map, ensure_ascii=False, indent=2) + ';\n')

    with open('app/cmdbCodeDict.ts', 'w', encoding='utf-8') as f:
        f.write('\n'.join(ts_lines))
    print('Generated app/cmdbCodeDict.ts successfully!')

if __name__ == '__main__':
    main()
