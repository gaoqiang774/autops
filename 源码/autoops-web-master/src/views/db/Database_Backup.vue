<script setup>
const sections = [
  {
    title: '1. 数据库备份与恢复',
    items: [
      '备份策略管理：全量、增量、binlog/WAL、RDB/AOF',
      '定时备份任务、保留周期、备份加密、备份压缩',
      '备份结果校验：文件存在性、大小、耗时、失败原因',
      '恢复演练：指定时间点恢复、恢复到临时实例',
      '备份报表：成功率、备份容量趋势、失败告警'
    ]
  },
  {
    title: '2. 数据库巡检',
    items: [
      '实例健康巡检：连接数、复制延迟、磁盘空间、慢 SQL、锁等待',
      '参数巡检：max_connections、buffer pool、work_mem、shared_buffers 等',
      '安全巡检：弱密码、空密码、危险权限、远程 root、过期账号',
      '高可用巡检：主从状态、复制断开、Redis cluster slot 状态',
      '自动生成巡检报告：按实例、业务系统、环境汇总评分'
    ]
  },
  {
    title: '3. 慢 SQL 与性能分析',
    items: [
      'MySQL slow log / PostgreSQL pg_stat_statements / Redis slowlog 接入',
      'SQL 指纹聚合、Top SQL、平均耗时、P95/P99',
      'SQL 执行计划采集与历史对比',
      '索引建议、全表扫描识别、临时表/排序风险提示',
      '和审核平台联动：慢 SQL 一键生成优化工单'
    ]
  }
]
</script>

<template>
  <div class="backup-demo-page">
    <section class="backup-demo-panel">
      <h2>慢 SQL 与性能分析</h2>
      <div class="demo-sections">
        <section
          v-for="section in sections"
          :key="section.title"
          class="demo-section"
        >
          <h3>{{ section.title }}</h3>
          <p
            v-for="item in section.items"
            :key="item"
          >
            {{ item }}
          </p>
        </section>
      </div>
    </section>
  </div>
</template>

<style scoped>
.backup-demo-page {
  min-height: 100%;
  padding: 8px;
  background: #f4f7fb;
  color: #172133;
  font-family: "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
}

.backup-demo-panel {
  padding: 20px 22px;
  border: 1px solid rgba(22, 34, 56, 0.06);
  border-radius: 22px;
  background:
    linear-gradient(135deg, rgba(255, 253, 248, 0.96) 0%, rgba(248, 251, 255, 0.92) 100%);
  box-shadow: 0 16px 42px rgba(17, 24, 39, 0.045);
}

.backup-demo-panel h2 {
  margin: 0 0 14px;
  font-size: 22px;
  font-weight: 700;
  line-height: 1.25;
}

.demo-sections {
  display: grid;
  gap: 18px;
}

.demo-section {
  display: grid;
  gap: 7px;
}

.demo-section h3 {
  margin: 0;
  color: #172133;
  font-size: 16px;
  font-weight: 700;
  line-height: 1.35;
}

.demo-section p {
  margin: 0;
  color: #5f6b7c;
  font-size: 14px;
  line-height: 1.7;
  overflow-wrap: anywhere;
}
</style>
