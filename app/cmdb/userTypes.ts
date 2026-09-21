export interface UserAccount {
  id: string;
  username: string; // 登录账号
  displayName: string; // 姓名/显示名称
  role: "admin" | "operator" | "auditor"; // 超级管理员 | 项目运维专员 | 安全审计员
  department: string;
  phone?: string;
  email?: string;
  status: "active" | "disabled";
  // 项目权限控制："all" 表示全网所有 24 个项目无限制可见；string[] 存放已授权的项目名称或ID
  authorizedProjects: "all" | string[];
  createdAt: string;
  lastLogin?: string;
  remarks?: string;
}

export const initialUsers: UserAccount[] = [
  {
    id: "user-admin",
    username: "admin",
    displayName: "系统超级管理员",
    role: "admin",
    department: "基础运维研发中心",
    phone: "13800000001",
    email: "admin@beikong.com",
    status: "active",
    authorizedProjects: "all",
    createdAt: "2026-09-01",
    lastLogin: "2026-09-19 21:30",
    remarks: "平台全局最高管理员，具备全量用户管理与全部 24 个项目授权管控权限"
  },
  {
    id: "user-ops-beijing",
    username: "ops_beijing",
    displayName: "人社项目运维专员 (张工)",
    role: "operator",
    department: "政务专网运维一区",
    phone: "13800000002",
    email: "zhang_ops@beikong.com",
    status: "active",
    authorizedProjects: [
      "调解仲裁系统",
      "原三险系统",
      "仲裁云庭审系统"
    ],
    createdAt: "2026-09-10",
    lastLogin: "2026-09-18 16:20",
    remarks: "经 admin 统一授权，仅具备人社局所属 3 个业务项目的资产纳管与排查权限"
  },
  {
    id: "user-gonghui-audit",
    username: "audit_gonghui",
    displayName: "工会业务审计员 (李工)",
    role: "auditor",
    department: "安全与合规审计部",
    phone: "13800000003",
    email: "li_audit@beikong.com",
    status: "active",
    authorizedProjects: [
      "工会互助保险信息系统",
      "工会职服数智化系统"
    ],
    createdAt: "2026-09-15",
    lastLogin: "2026-09-19 10:15",
    remarks: "经 admin 统一授权，仅具备工会互助保险与数智化系统只读安全审计权限"
  }
];
