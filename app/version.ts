/**
 * AutoOps 系统版本号规范与历史配置
 * 
 * 版本号递增规则：
 * 1. 修复 Bug：增加最后一位 (Z) -> 如 1.0.5 -> 1.0.6
 * 2. 增加模块：增加中间位 (Y) -> 初始 1.0.5，本次新增「漏洞检测」模块，升级为 1.1.0
 * 3. 增加主菜单内容：修改第一位 (X) -> 如 1.1.0 -> 2.0.0
 */

export const APP_VERSION = "1.1.2";

export interface VersionRecord {
  version: string;
  date: string;
  type: "major" | "module" | "bugfix";
  title: string;
  description: string;
}

export const VERSION_HISTORY: VersionRecord[] = [
  {
    version: "1.1.2",
    date: "2026-09-17",
    type: "bugfix",
    title: "修复顶部栏「天枢驾驶舱」按钮文字竖排折行与固定尺寸限制",
    description: "消除 .account button 全局 34px 固定宽高限制，为「天枢驾驶舱」与管理员按钮配置自适应宽度、防折行规范与现代化质感胶囊外观。"
  },
  {
    version: "1.1.1",
    date: "2026-09-17",
    type: "bugfix",
    title: "漏洞检测表格滚动条修复与探针脚本视线遮挡优化",
    description: "受影响设备清单表格增加独立纵横双向滚动条与表头固定(Sticky Header)；将遮挡视线的常驻黑框重构为工具栏【应急核验命令】弹窗与底部轻量折叠面板，彻底释放数据可视空间。"
  },
  {
    version: "1.1.0",
    date: "2026-09-17",
    type: "module",
    title: "新增「漏洞检测与受威胁设备快速定位」模块",
    description: "支持输入操作系统名称、版本号、内核版本秒级筛选定位全网受影响设备；联动展示所属项目、业务IP、内大网IP、VIP/EIP公网暴露面；提供高危CVE预设排查与受影响资产清单一键导出功能。"
  },
  {
    version: "1.0.5",
    date: "2026-09-17",
    type: "bugfix",
    title: "全站布局与粗滚动条遮挡排查修复",
    description: "重构业务拓扑顶部项目选择器为专业拓扑控制台；修复项目资产左右双栏联动工作台布局；全局注入 6px 超薄滚动条防遮挡规范。"
  },
  {
    version: "1.0.0",
    date: "2026-09-16",
    type: "major",
    title: "北控伟仕智能运维平台基线发布",
    description: "全面完成 24 个重点项目资产全生命周期管理、238 台信息资产台账、业务全景拓扑与天枢驾驶舱上线。"
  }
];
