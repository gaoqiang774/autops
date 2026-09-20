export type ModuleInfo={name:string;icon:string;route:string;menus:string[]};
export const modules:ModuleInfo[]=[
 {name:"资产管理",icon:"▰",route:"cmdb",menus:["项目资产","资产大盘","业务拓扑","漏洞检测","凭据管理","用户管理","AIops助手"]},
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
