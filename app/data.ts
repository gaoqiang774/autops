export type ModuleInfo={name:string;icon:string;route:string;menus:string[]};
export const modules:ModuleInfo[]=[
 {name:"资产管理",icon:"▰",route:"cmdb",menus:["项目资产","资产大盘","业务拓扑","漏洞检测","数据库管理","网络与负载","探针监控","凭据管理","AIops助手"]},
 {name:"容器管理",icon:"⬡",route:"k8s",menus:["集群管理","节点管理","命名空间","工作负载","网络管理","存储管理","配置管理","应用诊断","集群巡检","事件管理"]},
 {name:"DB数据库",icon:"◉",route:"db",menus:["数据资产","SQL查询","SQL审核","监控概览","慢日志分析","数据库AI助手"]},
 {name:"日志中心",icon:"▤",route:"logs",menus:["日志管理","日志查询","日志采集","日志告警","日志链路"]},
 {name:"配置中心",icon:"⚙",route:"config",menus:["主机凭据","通用凭据","密钥管理"]},
 {name:"告警中心",icon:"⚠",route:"alert",menus:["告警总览","告警事件","告警管理","告警模板","告警通知","告警数据源","域名告警"]},
 {name:"服务作业",icon:"▶",route:"task",menus:["作业调度","Ansible作业","作业模板","作业中心"]},
 {name:"AIOps运维",icon:"✦",route:"ai",menus:["智能助手","运维智库","日志分析","运维模型"]},
 {name:"工单中心",icon:"▣",route:"ticket",menus:["我的工单","事务工单","发布工单","审批流配置"]},
 {name:"费用中心",icon:"￥",route:"cost",menus:["账单总览","账单明细","账单分析"]},
 {name:"审计中心",icon:"◎",route:"audit",menus:["登录日志","操作日志","会话录制"]},
 {name:"系统管理",icon:"★",route:"system",menus:["用户信息","角色信息","菜单信息","资产授权","菜单导航","授权管理"]}
];
export const descriptions:Record<string,string>={
 "漏洞检测":"基于操作系统名称、版本号与内核特征秒级定位受影响资产，精准联动项目归属与处置响应。",
 "用户信息":"统一维护平台用户、组织归属、岗位角色和账号状态。",
 "机房管理":"集中管理机房、机柜、区域与资产上架信息。",
 "主机管理":"统一纳管物理主机、虚拟主机及监控状态。",
 "集群管理":"管理 Kubernetes 集群接入、状态与基础配置。",
 "告警事件":"聚合实时告警事件并跟踪处置状态。",
 "日志管理":"统一配置日志源、索引和数据保留策略。",
 "作业调度":"创建、编排并跟踪自动化运维作业。",
 "我的工单":"查看和处理与当前用户相关的流程工单。",
 "账单总览":"汇总云资源消费、趋势与预算执行情况。"
};
export const seedRows=[
 {id:1,name:"admin",display:"管理员",category:"超级管理员",owner:"研发总监",status:true,updated:"2026-09-17 17:30"},
 {id:2,name:"test",display:"游客",category:"test",owner:"测试工程师",status:true,updated:"2026-09-16 14:22"}
];
