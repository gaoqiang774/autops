package cmdb

import (
	"dodevops-api/api/cmdb/controller"
	"dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/service"
	"dodevops-api/common"

	"github.com/gin-gonic/gin"
)

// RegisterCmdbRoutes registers CMDB routes.
func RegisterCmdbRoutes(router *gin.RouterGroup) {
	// 监控数据源管理（CMDB 业务模型绑定）
	datasourceCtrl := controller.NewMonitorDatasourceController(
		service.NewMonitorDatasourceService(dao.NewMonitorDatasourceDAO(common.GetDB())),
	)
	datasource := router.Group("/monitor/datasource")
	{
		datasource.POST("/prometheus", datasourceCtrl.CreatePrometheus)
		datasource.PUT("/:id", datasourceCtrl.Update)
		datasource.DELETE("/:id", datasourceCtrl.Delete)
		datasource.GET("/:id", datasourceCtrl.Get)
		datasource.GET("/list", datasourceCtrl.List)
	}

	// 资产分组
	router.POST("/cmdb/groupadd", controller.CreateCmdbGroup)                    // 创建资产分组
	router.GET("/cmdb/grouplist", controller.GetAllCmdbGroups)                   // 获取资产分组列表
	router.GET("/cmdb/grouplistwithhosts", controller.GetAllCmdbGroupsWithHosts) // 获取带主机的资产分组列表
	router.PUT("/cmdb/groupupdate", controller.UpdateCmdbGroup)                  // 更新资产分组
	router.DELETE("/cmdb/groupdelete", controller.DeleteCmdbGroup)               // 删除资产分组
	router.GET("/cmdb/groupbyname", controller.GetCmdbGroupByName)               // 按名称查询资产分组

	// 机房与机柜
	router.POST("/cmdb/idc-room/create", controller.CreateCmdbIdcRoom)         // 创建机房
	router.PUT("/cmdb/idc-room/update", controller.UpdateCmdbIdcRoom)          // 更新机房
	router.DELETE("/cmdb/idc-room/delete", controller.DeleteCmdbIdcRoom)       // 删除机房
	router.GET("/cmdb/idc-room/info", controller.GetCmdbIdcRoomById)           // 获取机房详情
	router.GET("/cmdb/idc-room/list", controller.GetCmdbIdcRoomList)           // 获取机房列表
	router.GET("/cmdb/idc-room/layout", controller.GetCmdbIdcRoomLayout)       // 获取机房布局
	router.POST("/cmdb/idc-cabinet/create", controller.CreateCmdbIdcCabinet)   // 创建机柜
	router.PUT("/cmdb/idc-cabinet/update", controller.UpdateCmdbIdcCabinet)    // 更新机柜
	router.DELETE("/cmdb/idc-cabinet/delete", controller.DeleteCmdbIdcCabinet) // 删除机柜
	router.GET("/cmdb/idc-cabinet/list", controller.GetCmdbIdcCabinetList)     // 获取机柜列表
	router.GET("/cmdb/idc-cabinet/layout", controller.GetCmdbIdcCabinetLayout) // 获取机柜布局

	// 物理资产
	router.POST("/cmdb/idc-asset/create", controller.CreateCmdbIdcAsset)                      // 创建IDC资产
	router.PUT("/cmdb/idc-asset/update", controller.UpdateCmdbIdcAsset)                       // 更新IDC资产
	router.PUT("/cmdb/idc-asset/position", controller.UpdateCmdbIdcAssetPosition)             // 更新IDC资产位置
	router.DELETE("/cmdb/idc-asset/delete", controller.DeleteCmdbIdcAsset)                    // 删除IDC资产
	router.GET("/cmdb/idc-asset/info", controller.GetCmdbIdcAssetById)                        // 获取IDC资产详情
	router.GET("/cmdb/idc-asset/list", controller.GetCmdbIdcAssetList)                        // 获取IDC资产列表
	router.GET("/cmdb/idc-asset/hardware-monitor", controller.GetCmdbIdcAssetHardwareMonitor) // 获取IDC资产硬件监控
	router.POST("/cmdb/idc-asset/import", controller.ImportCmdbIdcAssetsFromExcel)            // 导入IDC资产
	router.GET("/cmdb/idc-asset/template", controller.DownloadCmdbIdcAssetTemplate)           // 下载IDC资产导入模板

	// 资产管理大盘
	dashboardCtrl := controller.NewCmdbDashboardController()
	router.GET("/cmdb/dashboard/cards", dashboardCtrl.GetCards)   // 获取资产管理大盘卡片数据
	router.GET("/cmdb/dashboard/charts", dashboardCtrl.GetCharts) // 获取资产管理大盘图表数据
	router.GET("/cmdb/dashboard/tables", dashboardCtrl.GetTables) // 获取资产管理大盘表格数据

	// 虚拟机
	router.POST("/cmdb/idc-vm/create", controller.CreateCmdbIdcVm)            // 创建IDC虚拟机
	router.PUT("/cmdb/idc-vm/update", controller.UpdateCmdbIdcVm)             // 更新IDC虚拟机
	router.DELETE("/cmdb/idc-vm/delete", controller.DeleteCmdbIdcVm)          // 删除IDC虚拟机
	router.GET("/cmdb/idc-vm/info", controller.GetCmdbIdcVmById)              // 获取IDC虚拟机详情
	router.GET("/cmdb/idc-vm/list", controller.GetCmdbIdcVmList)              // 获取IDC虚拟机列表
	router.POST("/cmdb/idc-vm/import", controller.ImportCmdbIdcVmsFromExcel)  // 导入IDC虚拟机
	router.GET("/cmdb/idc-vm/template", controller.DownloadCmdbIdcVmTemplate) // 下载IDC虚拟机导入模板

	// 主机
	hostCtrl := controller.NewCmdbHostController()
	router.POST("/cmdb/hostcreate", hostCtrl.CreateCmdbHost)                 // 创建主机
	router.PUT("/cmdb/hostupdate", hostCtrl.UpdateCmdbHost)                  // 更新主机
	router.DELETE("/cmdb/hostdelete", hostCtrl.DeleteCmdbHost)               // 删除主机
	router.GET("/cmdb/hostlist", hostCtrl.GetCmdbHostListWithPage)           // 分页获取主机列表
	router.GET("/cmdb/hostexport", hostCtrl.ExportCmdbHostsCSV)              // 导出主机列表CSV
	router.GET("/cmdb/hostinfo", hostCtrl.GetCmdbHostById)                   // 获取主机详情
	router.GET("/cmdb/hostgroup", hostCtrl.GetCmdbHostsByGroupId)            // 按分组获取主机
	router.GET("/cmdb/hostsearch", hostCtrl.SearchCmdbHosts)                 // 搜索主机
	router.GET("/cmdb/hostbyname", hostCtrl.GetCmdbHostsByHostNameLike)      // 按名称查询主机
	router.GET("/cmdb/hostbyip", hostCtrl.GetCmdbHostsByIP)                  // 按IP查询主机
	router.GET("/cmdb/hostbystatus", hostCtrl.GetCmdbHostsByStatus)          // 按状态查询主机
	router.GET("/cmdb/hostbyhosttype", hostCtrl.GetCmdbHostsByHostType)      // 按主机系统类型查询主机
	router.GET("/cmdb/hostbytype", hostCtrl.GetCmdbHostsByVendor)            // 按主机类型/厂商查询主机
	router.POST("/cmdb/hostimport", hostCtrl.ImportHostsFromExcel)           // 导入主机
	router.GET("/cmdb/hosttemplate", hostCtrl.DownloadHostTemplate)          // 下载主机导入模板
	router.POST("/cmdb/hostsync", hostCtrl.SyncHostInfo)                     // 同步主机信息
	router.GET("/cmdb/hostcloudstatistics", hostCtrl.GetCloudHostStatistics) // 获取云主机统计
	router.PUT("/cmdb/hostbatchupdategroup", hostCtrl.BatchUpdateHostGroup)  // 批量更新主机分组
	router.DELETE("/cmdb/hostbatchdelete", hostCtrl.BatchDeleteHost)         // 批量删除主机

	// 云主机
	cloudCtrl := controller.NewCmdbHostCloudController()
	router.POST("/cmdb/hostcloudcreatetianyi", cloudCtrl.CreateTianyiHost)   // Tianyi Cloud
	router.POST("/cmdb/hostcloudcreatealiyun", cloudCtrl.CreateAliyunHost)   // 创建阿里云主机
	router.POST("/cmdb/hostcloudcreatetencent", cloudCtrl.CreateTencentHost) // 创建腾讯云主机
	router.POST("/cmdb/hostcloudcreatebaidu", cloudCtrl.CreateBaiduHost)     // 创建百度云主机
	router.POST("/cmdb/hostcloudcreatehuawei", cloudCtrl.CreateHuaweiHost)   // 创建华为云主机
	router.POST("/cmdb/hostcloudcreateaws", cloudCtrl.CreateAWSHost)         // 创建AWS云主机

	// JumpServer 同步
	router.POST("/cmdb/hostjumpserversync", controller.NewCmdbHostJumpServerController().SyncJumpServerHosts) // 同步JumpServer主机

	// 主机 SSH
	hostSSHCtrl := controller.NewCmdbHostSSHController(service.GetCmdbHostSSHService())
	router.GET("/cmdb/hostssh/auths/:id", hostSSHCtrl.ListAuthOptions)               // 获取主机可登录账号
	router.GET("/cmdb/hostssh/connect/:id", hostSSHCtrl.ConnectTerminal)             // 连接主机终端
	router.GET("/cmdb/hostssh/command/:id", hostSSHCtrl.ExecuteCommand)              // 执行主机命令
	router.POST("/cmdb/hostssh/upload/:id", hostSSHCtrl.UploadFile)                  // 上传文件到主机
	router.GET("/cmdb/hostssh/upload/task/:taskId", hostSSHCtrl.GetUploadTaskStatus) // 获取上传任务状态
	router.GET("/cmdb/hostssh/download/:id", hostSSHCtrl.DownloadFile)               // 下载主机文件
	router.GET("/cmdb/hostssh/listfiles/:id", hostSSHCtrl.ListFiles)                 // 列出主机文件

	// 数据库
	sqlCtrl := controller.NewCmdbSQLController()
	router.GET("/cmdb/database/statistics", sqlCtrl.GetDatabaseStatistics)
	router.POST("/cmdb/database-asset/create", sqlCtrl.CreateDatabase)
	router.PUT("/cmdb/database-asset/update", sqlCtrl.UpdateDatabase)
	router.DELETE("/cmdb/database-asset/delete", sqlCtrl.DeleteDatabase)
	router.GET("/cmdb/database-asset/info", sqlCtrl.GetDatabase)
	router.GET("/cmdb/database-asset/list", sqlCtrl.ListDatabases)
	router.GET("/cmdb/database-asset/byname", sqlCtrl.GetDatabasesByName)
	router.GET("/cmdb/database-asset/bytype", sqlCtrl.GetDatabasesByType)
	router.GET("/cmdb/database-asset/statistics", sqlCtrl.GetDatabaseStatistics)
	router.POST("/cmdb/database-asset/test-auth", sqlCtrl.TestDatabaseAuth)
	router.POST("/cmdb/database", sqlCtrl.CreateDatabase)           // 创建数据库
	router.PUT("/cmdb/database", sqlCtrl.UpdateDatabase)            // 更新数据库
	router.DELETE("/cmdb/database", sqlCtrl.DeleteDatabase)         // 删除数据库
	router.GET("/cmdb/database/info", sqlCtrl.GetDatabase)          // 获取数据库详情
	router.GET("/cmdb/databaselist", sqlCtrl.ListDatabases)         // 获取数据库列表
	router.GET("/cmdb/database/byname", sqlCtrl.GetDatabasesByName) // 按名称查询数据库
	router.GET("/cmdb/database/bytype", sqlCtrl.GetDatabasesByType) // 按类型查询数据库

	// SNMP 设备
	snmpDeviceCtrl := controller.NewCmdbSnmpDeviceController()
	router.POST("/cmdb/snmp-devices/:id/decrypt-password", snmpDeviceCtrl.DecryptRemotePassword)  // Decrypt SNMP device remote password
	router.POST("/cmdb/snmp-devices", snmpDeviceCtrl.CreateSnmpDevice)                            // 创建SNMP设备
	router.PUT("/cmdb/snmp-devices/:id", snmpDeviceCtrl.UpdateSnmpDevice)                         // 更新SNMP设备
	router.DELETE("/cmdb/snmp-devices/:id", snmpDeviceCtrl.DeleteSnmpDevice)                      // 删除SNMP设备
	router.GET("/cmdb/snmp-devices/:id", snmpDeviceCtrl.GetSnmpDeviceById)                        // 获取SNMP设备详情
	router.GET("/cmdb/snmp-devices", snmpDeviceCtrl.GetSnmpDeviceList)                            // 获取SNMP设备列表
	router.GET("/cmdb/snmp-devices/:id/monitor", snmpDeviceCtrl.GetDeviceMonitorMetrics)          // 获取设备监控指标
	router.GET("/cmdb/snmp-devices/:id/resource", snmpDeviceCtrl.GetDeviceResourceMetrics)        // 获取设备资源指标
	router.GET("/cmdb/snmp-devices/batch-resource", snmpDeviceCtrl.GetBatchDeviceResourceMetrics) // 批量获取设备资源指标

	// Windows Exporter
	exporterCtrl := controller.NewWindowsExporterController()
	router.GET("/cmdb/windows-exporter/precheck", exporterCtrl.PrecheckExporter)                   // Windows Exporter 安装预检
	router.POST("/cmdb/windows-exporter/install", exporterCtrl.InstallExporter)                    // 安装 Windows Exporter
	router.GET("/cmdb/windows-exporter/install-log/:deviceId", exporterCtrl.GetExporterInstallLog) // 获取安装日志
	router.GET("/cmdb/windows-exporter/status/:deviceId", exporterCtrl.GetExporterStatus)          // 获取 Windows Exporter 状态
	router.DELETE("/cmdb/windows-exporter/uninstall/:deviceId", exporterCtrl.UninstallExporter)    // 卸载 Windows Exporter
	router.POST("/cmdb/windows-exporter/batch-install", exporterCtrl.BatchInstallExporter)         // 批量安装 Windows Exporter
	router.GET("/cmdb/windows-exporter/status-all", exporterCtrl.GetAllWindowsDevicesStatus)       // 获取全部 Windows Exporter 状态
	router.GET("/cmdb/windows-exporter/init-winrm-script", exporterCtrl.DownloadInitWinRMScript)   // 下载 WinRM 初始化脚本

	// SNMP 设备 SSH
	snmpDeviceSSHCtrl := controller.NewCmdbSnmpDeviceSSHController()
	router.GET("/cmdb/snmp-devices/:id/ssh/connect", snmpDeviceSSHCtrl.ConnectTerminal) // 连接 SNMP 设备终端
	router.GET("/cmdb/snmp-devices/:id/ssh/command", snmpDeviceSSHCtrl.ExecuteCommand)  // 执行 SNMP 设备命令

	// 主机监控
	monitorCtrl := controller.NewMonitorController()
	router.GET("/cmdb/hosts", monitorCtrl.BatchGetHostMetrics)                                            // 批量获取主机监控摘要
	router.GET("/cmdb/hosts/top-usage", monitorCtrl.GetTopUsageHosts)                                     // 获取主机资源使用排行
	router.GET("/cmdb/hosts/current-rankings", monitorCtrl.GetCurrentUsageRankings)                       // 获取主机当前资源排行
	router.GET("/cmdb/hosts/:id", monitorCtrl.GetHostMetrics)                                             // 获取单主机监控摘要
	router.GET("/cmdb/hosts/:id/history", monitorCtrl.GetHostMetricHistory)                               // 获取主机单项历史指标
	router.GET("/cmdb/hosts/:id/all-metrics", monitorCtrl.GetHostAllMetricsHistory)                       // 获取主机全部历史指标
	router.GET("/cmdb/hosts/:id/services/:serviceName/metrics", monitorCtrl.GetHostServiceMetricsHistory) // 获取主机服务进程趋势
	router.GET("/cmdb/hosts/:id/windows-dashboard", monitorCtrl.GetWindowsDashboardMetrics)               // 获取 Windows 监控大盘
	router.GET("/cmdb/hosts/:id/top-processes", monitorCtrl.GetTopProcesses)                              // 获取主机 Top 进程
	router.GET("/cmdb/hosts/:id/ports", monitorCtrl.GetHostPorts)                                         // 获取主机端口监控
	router.GET("/cmdb/hosts/:id/complete", monitorCtrl.GetCompleteHostMetrics)                            // 获取主机完整监控详情

	// Agent 管理
	agentCtrl := controller.NewAgentController()
	router.POST("/cmdb/agent/deploy", agentCtrl.DeployAgent)                                // 部署 Agent
	router.GET("/cmdb/agent/offline-assets", agentCtrl.GetOfflineAssets)                    // 获取离线 Agent 制品
	router.POST("/cmdb/agent/offline-assets/upload", agentCtrl.UploadOfflineAgentBinary)    // 上传离线 Agent 二进制
	router.GET("/cmdb/agent/offline-assets/check", agentCtrl.CheckOfflineAssets)            // 检查离线 Agent 制品
	router.POST("/cmdb/agent/offline-assets/precheck", agentCtrl.PrecheckOfflineDeployment) // 离线部署预检查
	router.DELETE("/cmdb/agent/uninstall", agentCtrl.UninstallAgent)                        // 卸载 Agent
	router.POST("/cmdb/agent/redeploy", agentCtrl.RedeployAgent)                            // 重新部署 Agent
	router.GET("/cmdb/agent/versions", agentCtrl.GetAgentVersions)                          // 获取 Agent 版本列表
	router.GET("/cmdb/agent/status/:id", agentCtrl.GetAgentStatus)                          // 获取 Agent 状态
	router.POST("/cmdb/agent/restart/:id", agentCtrl.RestartAgent)                          // 重启 Agent
	router.GET("/cmdb/agent/list", agentCtrl.GetAgentList)                                  // 获取 Agent 列表
	router.GET("/cmdb/agent/candidates", agentCtrl.GetAgentCandidateHosts)                  // 获取 Agent 可部署主机候选列表
	router.GET("/cmdb/agent/candidate-groups", agentCtrl.GetAgentCandidateGroups)           // 获取 Agent 可部署主机分组树
	router.GET("/cmdb/agent/query", agentCtrl.QueryAgentList)                               // 查询 Agent 列表
	router.GET("/cmdb/agent/statistics", agentCtrl.GetAgentStatistics)                      // 获取 Agent 统计
	router.DELETE("/cmdb/agent/delete/:id", agentCtrl.DeleteAgent)                          // 删除 Agent

	// 堡垒机用户组
	bastionGroupCtrl := controller.NewBastionUserGroupController()
	router.POST("/bastion/group/create", bastionGroupCtrl.CreateUserGroup)           // 创建堡垒机用户组
	router.PUT("/bastion/group/update", bastionGroupCtrl.UpdateUserGroup)            // 更新堡垒机用户组
	router.DELETE("/bastion/group/delete", bastionGroupCtrl.DeleteUserGroup)         // 删除堡垒机用户组
	router.GET("/bastion/group/info", bastionGroupCtrl.GetUserGroupById)             // 获取堡垒机用户组详情
	router.GET("/bastion/group/list", bastionGroupCtrl.GetUserGroupList)             // 获取堡垒机用户组列表
	router.POST("/bastion/group/member/add", bastionGroupCtrl.AddGroupMembers)       // 添加用户组成员
	router.POST("/bastion/group/member/remove", bastionGroupCtrl.RemoveGroupMembers) // 移除用户组成员
	router.GET("/bastion/group/member/list", bastionGroupCtrl.GetGroupMembers)       // 获取用户组成员列表
	router.GET("/bastion/user/groups", bastionGroupCtrl.GetUserGroups)               // 获取用户所属用户组

	// 堡垒机授权
	bastionAuthCtrl := controller.NewBastionHostAuthorizationController()
	router.POST("/bastion/auth/user/authorize", bastionAuthCtrl.AuthorizeHostsToUser)     // 授权主机给用户
	router.POST("/bastion/auth/group/authorize", bastionAuthCtrl.AuthorizeHostsToGroup)   // 授权主机给用户组
	router.POST("/bastion/auth/user/revoke", bastionAuthCtrl.RevokeHostsFromUser)         // 回收用户主机授权
	router.POST("/bastion/auth/group/revoke", bastionAuthCtrl.RevokeHostsFromGroup)       // 回收用户组主机授权
	router.GET("/bastion/auth/hosts", bastionAuthCtrl.GetAuthorizedHosts)                 // 获取授权主机列表
	router.GET("/bastion/user/authorized-hosts", bastionAuthCtrl.GetUserAuthorizedHosts)  // 获取用户已授权主机
	router.GET("/bastion/user/check-permission", bastionAuthCtrl.CheckUserHostPermission) // 校验用户主机权限

	// 堡垒机通用资产授权
	bastionAssetAuthCtrl := controller.NewBastionAssetAuthorizationController()
	router.POST("/bastion/auth/asset/user/authorize", bastionAssetAuthCtrl.AuthorizeAssetsToUser)     // 授权资产给用户
	router.POST("/bastion/auth/asset/group/authorize", bastionAssetAuthCtrl.AuthorizeAssetsToGroup)   // 授权资产给用户组
	router.POST("/bastion/auth/asset/user/revoke", bastionAssetAuthCtrl.RevokeAssetsFromUser)         // 回收用户资产授权
	router.POST("/bastion/auth/asset/group/revoke", bastionAssetAuthCtrl.RevokeAssetsFromGroup)       // 回收用户组资产授权
	router.GET("/bastion/auth/assets", bastionAssetAuthCtrl.GetAuthorizedAssets)                      // 获取授权资产列表
	router.GET("/bastion/user/check-asset-permission", bastionAssetAuthCtrl.CheckUserAssetPermission) // 校验用户资产权限

	RegisterCmdbModelRoutes(router)

	// RegisteRDPRoutes(router)
}
