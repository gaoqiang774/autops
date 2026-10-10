package router

import (
	alertmanagercontroller "dodevops-api/api/alertmanager/controller"
	"path/filepath"
	"reflect"
	"strings"

	aiopscontroller "dodevops-api/api/aiops/controller"
	cmdbcontroller "dodevops-api/api/cmdb/controller"
	cmdbdao "dodevops-api/api/cmdb/dao"
	cmdbservice "dodevops-api/api/cmdb/service"
	"dodevops-api/api/system/controller"
	"dodevops-api/common/config"
	"dodevops-api/middleware"
	"dodevops-api/pkg/agent/skill"
	"dodevops-api/pkg/log"
	"dodevops-api/router/aiops"
	"dodevops-api/router/alertmanager"
	"dodevops-api/router/cmdb"
	configCenter "dodevops-api/router/configCenter"
	"dodevops-api/router/costcenter"
	dbrouter "dodevops-api/router/db"
	"dodevops-api/router/k8s"
	"dodevops-api/router/logtail"
	"dodevops-api/router/system"
	"dodevops-api/router/task"
	"dodevops-api/router/ticket"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

const apiRoutePrefix = "/api/v1"

func InitRouter() *gin.Engine {
	router := gin.New()
	if snapshot, err := skill.GetDefaultCatalog().Reload(); err != nil {
		log.Log().Warnf("AIOps Skill catalog preload failed: %v", err)
	} else {
		log.Log().Infof("AIOps Skill catalog preloaded: loaded=%d errors=%d", snapshot.Loaded, len(snapshot.Errors))
	}

	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			tag := fld.Tag.Get("json")
			if tag == "-" {
				return ""
			}
			return strings.Split(tag, ",")[0]
		})
	}

	router.Use(gin.Recovery())
	router.Use(middleware.Cors())
	router.Use(log.CustomGinLogger())

	uploadDir := config.Config.ImageSettings.UploadDir
	if !strings.HasPrefix(uploadDir, "/") {
		if absPath, err := filepath.Abs(uploadDir); err == nil {
			uploadDir = absPath
			log.Log().Infof("Static upload directory: %s -> %s", config.Config.ImageSettings.UploadDir, uploadDir)
		}
	} else {
		log.Log().Infof("Static upload directory: %s", uploadDir)
	}

	router.Static(apiRoutePrefix+"/upload", uploadDir)

	register(router)
	return router
}

func register(router *gin.Engine) {
	if config.Config.Server.EnableSwagger {
		router.GET("/swagger-doc.json", swaggerDocHandler(router))
		router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL("/swagger-doc.json")))
		log.Log().Info("Swagger API文档已启用")
	} else {
		log.Log().Info("Swagger API文档已禁用")
	}

	apiGroup := router.Group(apiRoutePrefix)
	{
		registerAuthenticationRoutes(apiGroup)
		registerAgentHeartbeatRoutes(apiGroup)

		licensedPublicGroup := apiGroup.Group("")
		licensedPublicGroup.Use(middleware.RequireLicense())
		licensedPublicGroup.POST("/alertmanager/feishu/callback", alertmanagercontroller.HandleFeishuCallback)
		licensedPublicGroup.POST("/aiops/mcp", aiopscontroller.AIOpsMCP)
		licensedPublicGroup.POST("/aiops/mcp/:domain", aiopscontroller.AIOpsMCP)
		licensedPublicGroup.POST("/aiops/wecom/plugin/chat", aiopscontroller.WeComPluginChat)
		logtail.RegisterLogtailAgentPublicRoutes(licensedPublicGroup)
		task.RegisterWebSocketRoutes(licensedPublicGroup)

		// ========== RDP WebSocket 路由（不加认证） ==========
		// 初始化 RDP 相关依赖
		rdpDAO := cmdbdao.NewRDPTargetDAO()
		rdpService := cmdbservice.NewRDPService(rdpDAO)
		rdpController := cmdbcontroller.NewRDPController(rdpService)

		apiGroup.GET("/rdp/ws", func(ctx *gin.Context) {
			println("========== RDP WebSocket 路由被调用 ==========")
			rdpController.ConnectRDPNative(ctx)
			ctx.Abort()
		})
		// ========== RDP 路由结束 ==========

		jwtGroup := apiGroup.Group("")
		jwtGroup.Use(middleware.AuthMiddleware())
		jwtGroup.Use(middleware.LogMiddleware())
		jwtGroup.Use(middleware.VisitorReadOnlyMiddleware())
		{
			// 未授权时保留资产管理、系统管理和堡垒机资产连接能力。
			jwtGroup.POST("/rdp/session/open", rdpController.OpenSession)
			jwtGroup.POST("/rdp/upload/:hostId", rdpController.UploadHostFile)
			jwtGroup.GET("/rdp/upload/task/:taskId", rdpController.GetUploadTaskStatus)
			jwtGroup.POST("/rdp/disconnect", rdpController.DisconnectRDP)
			jwtGroup.GET("/rdp/sessions", rdpController.GetActiveSessions)
			system.RegisterSystemRoutes(jwtGroup)
			cmdb.RegisterCmdbRoutes(jwtGroup)
			configCenter.RegisterCredentialRoutes(jwtGroup)

			licensedJWTGroup := jwtGroup.Group("")
			licensedJWTGroup.Use(middleware.RequireLicense())
			alertmanager.RegisterAlertManagerRoutes(licensedJWTGroup)
			configCenter.RegisterConfigCenterRoutes(licensedJWTGroup)
			aiops.RegisterAiopsRoutes(licensedJWTGroup)
			costcenter.RegisterCostCenterRoutes(licensedJWTGroup)
			dbrouter.RegisterDbRoutes(licensedJWTGroup)
			task.RegisterTaskRoutes(licensedJWTGroup)
			ticket.RegisterTicketRoutes(licensedJWTGroup)
			k8s.RegisterK8sRoutes(licensedJWTGroup)
			logtail.RegisterLogtailRoutes(licensedJWTGroup)
		}
	}
}

func registerAuthenticationRoutes(group gin.IRoutes) {
	group.GET("/captcha", controller.Captcha)
	group.POST("/login", controller.Login)
}

func registerAgentHeartbeatRoutes(group gin.IRoutes) {
	agentCtrl := cmdbcontroller.NewAgentController()
	group.POST("/monitor/agent/heartbeat", agentCtrl.UpdateHeartbeat)
	group.POST("/cmdb/agent/heartbeat", agentCtrl.UpdateHeartbeat)
}

func swaggerDocHandler(router *gin.Engine) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"swagger": "2.0",
			"info": gin.H{
				"title":       "dodevops-api",
				"version":     "1.0",
				"description": "Auto-generated API documentation from Gin routes.",
			},
			"paths": buildSwaggerPaths(router.Routes()),
			"securityDefinitions": gin.H{
				"ApiKeyAuth": gin.H{
					"type": "apiKey",
					"name": "Authorization",
					"in":   "header",
				},
			},
		})
	}
}

func buildSwaggerPaths(routes gin.RoutesInfo) map[string]gin.H {
	paths := make(map[string]gin.H)
	for _, route := range routes {
		if strings.HasPrefix(route.Path, "/swagger/") || route.Path == "/swagger-doc.json" {
			continue
		}

		path := toSwaggerPath(route.Path)
		method := strings.ToLower(route.Method)

		if _, exists := paths[path]; !exists {
			paths[path] = gin.H{}
		}

		paths[path][method] = gin.H{
			"summary": route.Method + " " + path,
			"responses": gin.H{
				"200": gin.H{
					"description": "OK",
				},
			},
		}
	}

	mergeSwaggerAnnotationOperations(paths)
	return paths
}

func toSwaggerPath(path string) string {
	if path == "" {
		return "/"
	}

	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") && len(part) > 1 {
			parts[i] = "{" + part[1:] + "}"
			continue
		}
		if strings.HasPrefix(part, "*") && len(part) > 1 {
			parts[i] = "{" + part[1:] + "}"
		}
	}
	return strings.Join(parts, "/")
}
