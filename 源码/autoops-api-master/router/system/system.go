// utils/system/system.go

package system

import (
	"dodevops-api/api/system/controller"
	"dodevops-api/common/util/recording"
	"dodevops-api/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterSystemRoutes 注册系统相关路由
func RegisterSystemRoutes(router *gin.RouterGroup) {
	// 使用传入的 router，不再创建子分组
	systemGroup := router
	auditGroup := router.Group("")
	auditGroup.Use(middleware.RequireLicense())

	// 授权管理属于基础系统能力，未授权时也必须能查看机器码和激活。
	systemGroup.GET("/system/license/status", controller.GetLicenseStatus)
	systemGroup.GET("/system/license/access", controller.GetLicenseSessionAccess)
	systemGroup.POST("/system/license/activate", controller.ActivateLicense)

	// 岗位
	systemGroup.POST("/post/add", controller.CreateSysPost)
	systemGroup.GET("/post/list", controller.GetSysPostList)
	systemGroup.GET("/post/info", controller.GetSysPostById)
	systemGroup.PUT("/post/update", controller.UpdateSysPost)
	systemGroup.DELETE("/post/delete", controller.DeleteSysPostById)
	systemGroup.DELETE("/post/batch/delete", controller.BatchDeleteSysPost)
	systemGroup.PUT("/post/updateStatus", controller.UpdateSysPostStatus)
	systemGroup.GET("/post/vo/list", controller.QuerySysPostVoList)
	//  部门
	systemGroup.GET("/dept/list", controller.GetSysDeptList)         // 部门列表
	systemGroup.POST("/dept/add", controller.CreateSysDept)          // 添加部门
	systemGroup.GET("/dept/info", controller.GetSysDeptById)         // 根据id查询部门
	systemGroup.PUT("/dept/update", controller.UpdateSysDept)        // 修改部门
	systemGroup.DELETE("/dept/delete", controller.DeleteSysDeptById) // 删除部门
	systemGroup.GET("/dept/vo/list", controller.QuerySysDeptVoList)  // 查询部门树形结构
	systemGroup.GET("/dept/users", controller.GetDeptUsers)          // 查询部门下的用户
	// 菜单
	systemGroup.POST("/menu/add", controller.CreateSysMenu)
	systemGroup.GET("/menu/vo/list", controller.QuerySysMenuVoList)
	systemGroup.GET("/menu/info", controller.GetSysMenu)
	systemGroup.PUT("/menu/update", controller.UpdateSysMenu)
	systemGroup.DELETE("/menu/delete", controller.DeleteSysMenu)
	systemGroup.GET("/menu/list", controller.GetSysMenuList)
	// 角色
	systemGroup.POST("/role/add", controller.CreateSysRole)
	systemGroup.GET("/role/info", controller.GetSysRoleById)
	systemGroup.PUT("/role/update", controller.UpdateSysRole)
	systemGroup.DELETE("/role/delete", controller.DeleteSysRoleById)
	systemGroup.PUT("/role/updateStatus", controller.UpdateSysRoleStatus)
	systemGroup.GET("/role/list", controller.GetSysRoleList)
	systemGroup.GET("/role/vo/list", controller.QuerySysRoleVoList)
	systemGroup.GET("/role/vo/idList", controller.QueryRoleMenuIdList)
	systemGroup.PUT("/role/assignPermissions", controller.AssignPermissions)
	// 用户
	systemGroup.POST("/admin/add", controller.CreateSysAdmin)
	systemGroup.GET("/admin/info", controller.GetSysAdminInfo)
	systemGroup.PUT("/admin/update", controller.UpdateSysAdmin)
	systemGroup.DELETE("/admin/delete", controller.DeleteSysAdminById)
	systemGroup.PUT("/admin/updateStatus", controller.UpdateSysAdminStatus)
	systemGroup.PUT("/admin/updatePassword", controller.ResetSysAdminPassword)
	systemGroup.GET("/admin/list", controller.GetSysAdminList)
	systemGroup.POST("/upload", controller.Upload)
	systemGroup.PUT("/admin/updatePersonal", controller.UpdatePersonal)
	systemGroup.PUT("/admin/updatePersonalPassword", controller.UpdatePersonalPassword)
	// 日志
	auditGroup.GET("/sysLoginInfo/list", controller.GetSysLoginInfoList)
	auditGroup.DELETE("/sysLoginInfo/batch/delete", controller.BatchDeleteSysLoginInfo)
	auditGroup.DELETE("/sysLoginInfo/delete", controller.DeleteSysLoginInfoById)
	auditGroup.DELETE("/sysLoginInfo/clean", controller.CleanSysLoginInfo)
	auditGroup.GET("/sysOperationLog/list", controller.GetSysOperationLogList)
	auditGroup.DELETE("/sysOperationLog/delete", controller.DeleteSysOperationLogById)
	auditGroup.DELETE("/sysOperationLog/batch/delete", controller.BatchDeleteSysOperationLog)
	auditGroup.DELETE("/sysOperationLog/clean", controller.CleanSysOperationLog)
	// 系统动态日志
	activityLogController := controller.NewSysActivityLogController()
	auditGroup.POST("/activity-log", activityLogController.Create)
	auditGroup.GET("/activity-log/detail", activityLogController.GetByID)
	auditGroup.GET("/activity-log/list", activityLogController.List)
	auditGroup.GET("/activity-log/latest", activityLogController.GetLatest)
	auditGroup.GET("/activity-log/related", activityLogController.GetByRelatedID)
	auditGroup.DELETE("/activity-log", activityLogController.DeleteByID)
	auditGroup.DELETE("/activity-log/batch", activityLogController.DeleteBatch)
	auditGroup.DELETE("/activity-log/clean", activityLogController.Clean)
	// 系统配置
	systemGroup.POST("/config/add", controller.CreateSysConfig)
	systemGroup.GET("/config/info", controller.GetSysConfigInfo)
	systemGroup.GET("/config/key", controller.GetSysConfigByKey)
	systemGroup.PUT("/config/update", controller.UpdateSysConfig)
	systemGroup.DELETE("/config/delete", controller.DeleteSysConfig)
	systemGroup.GET("/config/list", controller.GetSysConfigList)
	systemGroup.PUT("/config/updateStatus", controller.UpdateSysConfigStatus)
	// 导航管理
	systemGroup.POST("/system/nav/category/add", controller.CreateSysNavCategory)      // 新增导航分类
	systemGroup.PUT("/system/nav/category/update", controller.UpdateSysNavCategory)    // 修改导航分类
	systemGroup.DELETE("/system/nav/category/delete", controller.DeleteSysNavCategory) // 删除导航分类
	systemGroup.GET("/system/nav/category/info", controller.GetSysNavCategory)         // 获取导航分类详情
	systemGroup.GET("/system/nav/category/list", controller.ListSysNavCategories)      // 查询导航分类列表
	systemGroup.POST("/system/nav/item/add", controller.CreateSysNavItem)              // 新增导航链接
	systemGroup.PUT("/system/nav/item/update", controller.UpdateSysNavItem)            // 修改导航链接
	systemGroup.DELETE("/system/nav/item/delete", controller.DeleteSysNavItem)         // 删除导航链接
	systemGroup.GET("/system/nav/item/info", controller.GetSysNavItem)                 // 获取导航链接详情
	systemGroup.GET("/system/nav/item/list", controller.ListSysNavItems)               // 分页查询导航链接
	systemGroup.GET("/system/nav/visible", controller.GetVisibleSysNavigation)         // 获取当前用户可见导航
	// LDAP配置
	systemGroup.POST("/ldap/config", controller.SaveLDAPConfig)
	systemGroup.GET("/ldap/config", controller.GetLDAPConfig)
	systemGroup.POST("/ldap/test", controller.TestLDAPConnection)
	systemGroup.POST("/ldap/login", controller.LDAPLogin) // LDAP登录（不需要认证）
	// 会话录制管理
	recordingCtrl, err := controller.NewSysSessionRecordingController(recording.DefaultLocalStoragePath())
	if err == nil {
		auditGroup.GET("/recordings", recordingCtrl.GetRecordingList)
		auditGroup.GET("/recordings/:id", recordingCtrl.GetRecordingDetail)
		auditGroup.GET("/recordings/:id/cast", recordingCtrl.GetRecordingCast)
		auditGroup.GET("/recordings/:id/download", recordingCtrl.DownloadRecording)
		auditGroup.DELETE("/recordings/:id", recordingCtrl.DeleteRecording)
		auditGroup.DELETE("/recordings/batch", recordingCtrl.BatchDeleteRecording)
	}

	// 命令审计管理 - 注意：具体路径要放在参数路径之前
	commandAuditCtrl := controller.NewSysCommandAuditController()
	auditGroup.GET("/recordings/commands", commandAuditCtrl.GetCommandAuditList)
	auditGroup.GET("/recordings/commands/sensitive-count/:recordingId", commandAuditCtrl.GetSensitiveCommandCount)
	auditGroup.GET("/recordings/commands/highest-risk/:recordingId", commandAuditCtrl.GetHighestRiskLevel)
}
