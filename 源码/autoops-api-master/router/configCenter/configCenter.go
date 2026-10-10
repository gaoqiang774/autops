package configCenter

import (
	"dodevops-api/api/configcenter/controller"

	"github.com/gin-gonic/gin"
)

// RegisterConfigCenterRoutes 注册配置中心相关路由
func RegisterConfigCenterRoutes(router *gin.RouterGroup) {
	accountAuthCtrl := controller.NewAccountAuthController()
	keyManageCtrl := controller.NewKeyManageController()

	// 通用账号账号认证管理
	router.POST("/config/accountauth", accountAuthCtrl.Create)                  // 创建账号认证信息
	router.PUT("/config/accountauth", accountAuthCtrl.Update)                   // 更新账号认证信息
	router.DELETE("/config/accountauth", accountAuthCtrl.Delete)                // 删除账号认证信息
	router.GET("/config/accountauth", accountAuthCtrl.GetByID)                  // 根据账号 ID 获取账号认证详情
	router.GET("/config/accountauth/list", accountAuthCtrl.List)                // 分页获取账号认证列表
	router.POST("/config/accountauth/decrypt", accountAuthCtrl.DecryptPassword) // 解密账号认证密码
	router.POST("/config/accountauth/import", accountAuthCtrl.Import)           // 批量导入账号认证信息
	router.GET("/config/accountauth/type", accountAuthCtrl.GetByType)           // 根据账号类型查询账号认证列表
	router.GET("/config/accountauth/alias", accountAuthCtrl.GetByAlias)         // 根据账号别名查询账号认证详情
	router.POST("/config/accountauth/test", accountAuthCtrl.TestConnection)     // 测试账号认证连接

	// 密钥管理
	router.POST("/config/keymanage", keyManageCtrl.Create)              // 创建云厂商密钥配置
	router.PUT("/config/keymanage", keyManageCtrl.Update)               // 更新云厂商密钥配置
	router.DELETE("/config/keymanage", keyManageCtrl.Delete)            // 删除云厂商密钥配置
	router.GET("/config/keymanage", keyManageCtrl.GetByID)              // 根据密钥 ID 获取密钥配置详情
	router.GET("/config/keymanage/list", keyManageCtrl.List)            // 分页获取密钥配置列表
	router.POST("/config/keymanage/decrypt", keyManageCtrl.DecryptKeys) // 解密密钥配置的 Key 和 Secret
	router.GET("/config/keymanage/type", keyManageCtrl.GetByType)       // 根据密钥类型查询密钥配置列表
	router.POST("/config/keymanage/sync", keyManageCtrl.SyncCloudHosts) // 根据密钥类型自动同步云主机，keyType: 1=阿里云,2=腾讯云,3=百度云
}

// RegisterCredentialRoutes 注册资产管理依赖的 ECS 认证凭据路由。
// 该能力在系统未授权时仍开放，但依然需要 JWT 登录认证。
func RegisterCredentialRoutes(router *gin.RouterGroup) {
	ecsAuthCtrl := controller.NewEcsAuthController()
	router.GET("/config/ecsauthlist", ecsAuthCtrl.GetEcsAuthList)
	router.GET("/config/ecsauthinfo", ecsAuthCtrl.GetEcsAuthByName)
	router.GET("/config/ecsauthdetail", ecsAuthCtrl.GetEcsAuthById)
	router.GET("/config/ecsauthtemplate", ecsAuthCtrl.DownloadEcsAuthTemplate)
	router.POST("/config/ecsauthadd", ecsAuthCtrl.CreateEcsAuth)
	router.POST("/config/ecsauth/decrypt", ecsAuthCtrl.DecryptEcsAuth)
	router.POST("/config/ecsauthimport", ecsAuthCtrl.ImportEcsPasswordAuth)
	router.POST("/config/ecsauthmigrate", ecsAuthCtrl.MigrateEcsAuthSecrets)
	router.PUT("/config/ecsauthupdate", ecsAuthCtrl.UpdateEcsAuth)
	router.DELETE("/config/ecsauthdelete", ecsAuthCtrl.DeleteEcsAuth)
}
