package db

import (
	"fmt"
	"strings"

	alertmanagermodel "dodevops-api/api/alertmanager/model"
	"dodevops-api/common/util"
)

type agentBuiltinTemplateSpec struct {
	Category    string
	Metric      string
	Name        string
	Description string
	Expression  string
	Operator    string
	Threshold   float64
	Unit        string
	Severity    string
	ForDuration int64
	Sort        int
}

func defaultAgentAlertBuiltinRuleTemplates(now util.HTime) []alertmanagermodel.AlertBuiltinRuleTemplate {
	specs := []agentBuiltinTemplateSpec{
		{Category: "cpu", Metric: "system_cpu_usage_percent", Name: "Agent CPU 使用率过高", Description: "CPU 使用率", Expression: "avg by(instance) (system_cpu_usage_percent)", Operator: ">", Threshold: 85, Unit: "%", Severity: "warning", ForDuration: 300, Sort: 100},
		{Category: "cpu", Metric: "system_cpu_user_percent", Name: "Agent 用户态 CPU 使用率过高", Description: "用户空间 CPU 占用比率", Expression: "system_cpu_user_percent", Operator: ">", Threshold: 80, Unit: "%", Severity: "warning", ForDuration: 300, Sort: 95},
		{Category: "cpu", Metric: "system_cpu_system_percent", Name: "Agent 内核态 CPU 使用率过高", Description: "内核空间 CPU 占用比率", Expression: "system_cpu_system_percent", Operator: ">", Threshold: 40, Unit: "%", Severity: "warning", ForDuration: 300, Sort: 90},

		{Category: "memory", Metric: "system_memory_usage_percent", Name: "Agent 内存使用率过高", Description: "内存使用率", Expression: "system_memory_usage_percent", Operator: ">", Threshold: 85, Unit: "%", Severity: "warning", ForDuration: 300, Sort: 100},
		{Category: "memory", Metric: "system_memory_used_gb", Name: "Agent 已用内存过高", Description: "已用内存实际大小", Expression: "system_memory_used_gb", Operator: ">", Threshold: 8, Unit: "GB", Severity: "warning", ForDuration: 300, Sort: 95},
		{Category: "memory", Metric: "system_memory_free_gb", Name: "Agent 可用内存过低", Description: "可用内存实际大小", Expression: "system_memory_free_gb", Operator: "<", Threshold: 1, Unit: "GB", Severity: "warning", ForDuration: 300, Sort: 90},
		{Category: "memory", Metric: "system_swap_used_gb", Name: "Agent Swap 使用量过高", Description: "已使用交换分区大小", Expression: "system_swap_used_gb", Operator: ">", Threshold: 1, Unit: "GB", Severity: "warning", ForDuration: 300, Sort: 85},

		{Category: "load", Metric: "system_load_average_1min", Name: "Agent 1 分钟平均负载过高", Description: "系统 1 分钟平均负载", Expression: "system_load_average_1min", Operator: ">", Threshold: 4, Unit: "", Severity: "warning", ForDuration: 300, Sort: 100},
		{Category: "load", Metric: "system_load_average_5min", Name: "Agent 5 分钟平均负载过高", Description: "系统 5 分钟平均负载", Expression: "system_load_average_5min", Operator: ">", Threshold: 4, Unit: "", Severity: "warning", ForDuration: 300, Sort: 95},
		{Category: "load", Metric: "system_load_average_15min", Name: "Agent 15 分钟平均负载过高", Description: "系统 15 分钟平均负载", Expression: "system_load_average_15min", Operator: ">", Threshold: 4, Unit: "", Severity: "warning", ForDuration: 300, Sort: 90},
		{Category: "load", Metric: "system_load_ratio_1min", Name: "Agent 1 分钟负载比过高", Description: "系统 1 分钟负载比", Expression: "system_load_ratio_1min", Operator: ">", Threshold: 1, Unit: "", Severity: "warning", ForDuration: 300, Sort: 85},
		{Category: "load", Metric: "system_load_ratio_5min", Name: "Agent 5 分钟负载比过高", Description: "系统 5 分钟负载比", Expression: "system_load_ratio_5min", Operator: ">", Threshold: 1, Unit: "", Severity: "warning", ForDuration: 300, Sort: 80},
		{Category: "load", Metric: "system_load_ratio_15min", Name: "Agent 15 分钟负载比过高", Description: "系统 15 分钟负载比", Expression: "system_load_ratio_15min", Operator: ">", Threshold: 1, Unit: "", Severity: "warning", ForDuration: 300, Sort: 75},

		{Category: "disk", Metric: "system_disk_usage_percent", Name: "Agent 根分区磁盘使用率过高", Description: "根分区磁盘使用率", Expression: `system_disk_usage_percent{mountpoint="/"}`, Operator: ">", Threshold: 85, Unit: "%", Severity: "warning", ForDuration: 300, Sort: 100},
		{Category: "disk", Metric: "system_disk_used_bytes", Name: "Agent 根分区已用空间过高", Description: "根分区已用磁盘空间", Expression: `system_disk_used_bytes{mountpoint="/"}`, Operator: ">", Threshold: 100, Unit: "GB", Severity: "warning", ForDuration: 300, Sort: 95},
		{Category: "disk", Metric: "system_disk_inode_usage_percent", Name: "Agent 根分区 inode 使用率过高", Description: "根分区磁盘 inode 使用率", Expression: `system_disk_inode_usage_percent{mountpoint="/"}`, Operator: ">", Threshold: 85, Unit: "%", Severity: "warning", ForDuration: 300, Sort: 90},

		{Category: "disk_io", Metric: "system_disk_read_kb_per_second", Name: "Agent 磁盘读取速率过高", Description: "磁盘 IO 读取速率", Expression: "sum by(instance) (system_disk_read_kb_per_second)", Operator: ">", Threshold: 102400, Unit: "KB/s", Severity: "warning", ForDuration: 300, Sort: 100},
		{Category: "disk_io", Metric: "system_disk_write_kb_per_second", Name: "Agent 磁盘写入速率过高", Description: "磁盘 IO 写入速率", Expression: "sum by(instance) (system_disk_write_kb_per_second)", Operator: ">", Threshold: 102400, Unit: "KB/s", Severity: "warning", ForDuration: 300, Sort: 95},
		{Category: "disk_io", Metric: "system_disk_read_iops", Name: "Agent 磁盘读 IOPS 过高", Description: "磁盘 IO 每秒读操作数", Expression: "sum by(instance) (system_disk_read_iops)", Operator: ">", Threshold: 1000, Unit: "IOPS", Severity: "warning", ForDuration: 300, Sort: 90},
		{Category: "disk_io", Metric: "system_disk_write_iops", Name: "Agent 磁盘写 IOPS 过高", Description: "磁盘 IO 每秒写操作数", Expression: "sum by(instance) (system_disk_write_iops)", Operator: ">", Threshold: 1000, Unit: "IOPS", Severity: "warning", ForDuration: 300, Sort: 85},

		{Category: "network", Metric: "system_network_receive_kb_per_second", Name: "Agent 网络接收速率过高", Description: "网络接收速率", Expression: "system_network_receive_kb_per_second", Operator: ">", Threshold: 102400, Unit: "KB/s", Severity: "warning", ForDuration: 300, Sort: 100},
		{Category: "network", Metric: "system_network_send_kb_per_second", Name: "Agent 网络发送速率过高", Description: "网络发送速率", Expression: "system_network_send_kb_per_second", Operator: ">", Threshold: 102400, Unit: "KB/s", Severity: "warning", ForDuration: 300, Sort: 95},

		{Category: "connection", Metric: "system_tcp_connections", Name: "Agent TCP 连接数过高", Description: "TCP 连接总数", Expression: "system_tcp_connections", Operator: ">", Threshold: 2000, Unit: "个", Severity: "warning", ForDuration: 300, Sort: 100},
		{Category: "connection", Metric: "system_udp_connections", Name: "Agent UDP 连接数过高", Description: "UDP 连接总数", Expression: "system_udp_connections", Operator: ">", Threshold: 1000, Unit: "个", Severity: "warning", ForDuration: 300, Sort: 95},

		{Category: "service", Metric: "tcp_port_listening", Name: "Agent 服务端口未监听", Description: "服务端口监听状态", Expression: "tcp_port_listening", Operator: "==", Threshold: 0, Unit: "", Severity: "critical", ForDuration: 180, Sort: 100},
		{Category: "service", Metric: "system_total_processes", Name: "Agent 系统进程数过高", Description: "系统进程总数", Expression: "system_total_processes", Operator: ">", Threshold: 500, Unit: "个", Severity: "warning", ForDuration: 300, Sort: 95},
		{Category: "service", Metric: "system_uptime_days", Name: "Agent 系统最近重启", Description: "系统持续运行天数", Expression: "system_uptime_days", Operator: "<", Threshold: 1, Unit: "天", Severity: "info", ForDuration: 60, Sort: 90},
		{Category: "service", Metric: "system_open_file_descriptors", Name: "Agent 打开文件句柄数过高", Description: "系统当前打开的文件句柄数", Expression: "system_open_file_descriptors", Operator: ">", Threshold: 10000, Unit: "个", Severity: "warning", ForDuration: 300, Sort: 85},
	}

	templates := make([]alertmanagermodel.AlertBuiltinRuleTemplate, 0, len(specs))
	for _, spec := range specs {
		thresholdText := "{{threshold}}"
		if spec.Unit != "" {
			thresholdText += spec.Unit
		}
		templates = append(templates, alertmanagermodel.AlertBuiltinRuleTemplate{
			ResourceType:               "host",
			Component:                  "Linux",
			Collector:                  "agent",
			Category:                   spec.Category,
			Name:                       spec.Name,
			Description:                fmt.Sprintf("%s（%s）", spec.Description, spec.Metric),
			DatasourceType:             "prometheus",
			Source:                     "platform",
			SourceUUID:                 "platform.agent." + spec.Metric,
			QueryTemplate:              fmt.Sprintf("%s %s {{threshold}}", spec.Expression, spec.Operator),
			DefaultConditionOperator:   spec.Operator,
			DefaultConditionValue:      spec.Threshold,
			DefaultEvalIntervalSeconds: 60,
			DefaultForDurationSeconds:  spec.ForDuration,
			DefaultSeverity:            spec.Severity,
			DefaultEnabled:             0,
			Labels:                     alertBuiltinLabels("host", "Linux", spec.Category),
			Annotations:                alertBuiltinAnnotations(spec.Name, fmt.Sprintf("实例 {{ $labels.instance }} %s，触发阈值 %s。", spec.Description, thresholdText)),
			Variables:                  alertThresholdVariable(spec.Threshold, spec.Unit),
			Tags: alertmanagermodel.JSONMap{
				"collector": "agent",
				"metric":    spec.Metric,
			},
			Sort:      spec.Sort,
			Enabled:   1,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	return templates
}

func alertBuiltinCollectorDisplayName(collector string) string {
	if strings.EqualFold(strings.TrimSpace(collector), "agent") {
		return "Agent 监控模板"
	}
	return collector
}

func alertBuiltinCategoryDisplayName(collector, category string) string {
	if !strings.EqualFold(strings.TrimSpace(collector), "agent") {
		return category
	}
	names := map[string]string{
		"cpu":        "CPU",
		"memory":     "内存",
		"load":       "负载",
		"disk":       "磁盘",
		"disk_io":    "磁盘 IO",
		"network":    "网络",
		"connection": "连接数",
		"service":    "服务",
	}
	if name := names[category]; name != "" {
		return name
	}
	return category
}
