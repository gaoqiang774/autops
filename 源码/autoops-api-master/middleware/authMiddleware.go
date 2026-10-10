// 鉴权中间件
// author xiaoRui

package middleware

import (
	cmdbdao "dodevops-api/api/cmdb/dao"
	cmdbmodel "dodevops-api/api/cmdb/model"
	systemdao "dodevops-api/api/system/dao"
	systemmodel "dodevops-api/api/system/model"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"
	"dodevops-api/pkg/jwt"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() func(c *gin.Context) {
	return func(c *gin.Context) {
		// 检查是否为websocket升级请求
		if c.GetHeader("Upgrade") == "websocket" {
			// 从URL参数获取token
			token := c.Query("token")
			if token != "" {
				if mc, err := jwt.ValidateToken(token); err == nil {
					continueWithAuthenticatedUser(c, mc)
					return
				}
			}

			// 尝试从WebSocket消息中获取认证信息
			conn, buf, err := c.Writer.(http.Hijacker).Hijack()
			if err != nil {
				result.Failed(c, int(result.ApiCode.WEBSOCKETERROR), result.ApiCode.GetMessage(result.ApiCode.WEBSOCKETERROR))
				c.Abort()
				return
			}
			defer conn.Close()
			defer buf.Flush()

			// 读取第一条消息作为认证消息
			msg, op, err := wsutil.ReadClientData(conn)
			if err != nil {
				result.Failed(c, int(result.ApiCode.WEBSOCKETERROR), result.ApiCode.GetMessage(result.ApiCode.WEBSOCKETERROR))
				c.Abort()
				return
			}
			if op != ws.OpText && op != ws.OpBinary {
				result.Failed(c, int(result.ApiCode.AUTHFORMATERROR), result.ApiCode.GetMessage(result.ApiCode.AUTHFORMATERROR))
				c.Abort()
				return
			}

			// 解析JSON格式的认证消息
			var authMsg struct {
				Type  string `json:"type"`
				Token string `json:"token"`
			}
			if err := json.Unmarshal(msg, &authMsg); err != nil || authMsg.Type != "auth" {
				result.Failed(c, int(result.ApiCode.AUTHFORMATERROR), result.ApiCode.GetMessage(result.ApiCode.AUTHFORMATERROR))
				c.Abort()
				return
			}

			if mc, err := jwt.ValidateToken(authMsg.Token); err == nil {
				continueWithAuthenticatedUser(c, mc)
				return
			} else {
				result.Failed(c, int(result.ApiCode.INVALIDTOKEN), result.ApiCode.GetMessage(result.ApiCode.INVALIDTOKEN))
				c.Abort()
				return
			}
		}

		// 常规HTTP请求鉴权 - 支持SSE的token query参数
		authHeader := c.Request.Header.Get("Authorization")

		// 如果没有Authorization头，检查是否为SSE连接并从query参数获取token
		if authHeader == "" {
			token := c.Query("token")
			if token != "" {
				// 验证token
				mc, err := jwt.ValidateToken(token)
				if err == nil {
					continueWithAuthenticatedUser(c, mc)
					return
				} else {
					// 添加调试信息
					fmt.Printf("[DEBUG] Token验证失败: %v, token长度: %d, Secret: %s\n", err, len(token), string(jwt.Secret))
				}
			}

			result.Failed(c, int(result.ApiCode.NOAUTH), result.ApiCode.GetMessage(result.ApiCode.NOAUTH))
			c.Abort()
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == "Bearer") {
			result.Failed(c, int(result.ApiCode.AUTHFORMATERROR), result.ApiCode.GetMessage(result.ApiCode.AUTHFORMATERROR))
			c.Abort()
			return
		}
		mc, err := jwt.ValidateToken(parts[1])
		if err != nil {
			result.Failed(c, int(result.ApiCode.INVALIDTOKEN), result.ApiCode.GetMessage(result.ApiCode.INVALIDTOKEN))
			c.Abort()
			return
		}
		continueWithAuthenticatedUser(c, mc)
	}
}

// RequirePermission verifies a button-level permission after AuthMiddleware has
// populated the current user in the Gin context.
func RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userObj, exists := c.Get(constant.ContextKeyUserObj)
		admin, ok := userObj.(*systemmodel.JwtAdmin)
		if !exists || !ok || admin == nil {
			result.Failed(c, http.StatusUnauthorized, "未获取到当前登录用户")
			c.Abort()
			return
		}

		if isK8SRootAdmin(admin) {
			c.Next()
			return
		}

		for _, item := range systemdao.QueryPermissionList(admin.ID) {
			if strings.TrimSpace(item.Value) == permission {
				c.Next()
				return
			}
		}

		result.Failed(c, http.StatusForbidden, "无操作权限: "+permission)
		c.Abort()
	}
}

// VisitorReadOnlyMiddleware makes the built-in test account a read-only
// visitor. It must run after AuthMiddleware so the current user is available.
func VisitorReadOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userObj, exists := c.Get(constant.ContextKeyUserObj)
		admin, ok := userObj.(*systemmodel.JwtAdmin)
		if !exists || !ok || admin == nil || !isVisitor(admin) {
			c.Next()
			return
		}

		isWebSocket := strings.EqualFold(strings.TrimSpace(c.GetHeader("Upgrade")), "websocket")
		if isWebSocket || !isReadOnlyMethod(c.Request.Method) || isInteractiveGetRoute(c.FullPath()) {
			result.Failed(c, int(result.ApiCode.TESTUSERFORBIDDEN), result.ApiCode.GetMessage(result.ApiCode.TESTUSERFORBIDDEN))
			c.Abort()
			return
		}

		c.Next()
	}
}

func isVisitor(admin *systemmodel.JwtAdmin) bool {
	return admin != nil && strings.EqualFold(strings.TrimSpace(admin.Username), "test")
}

func isReadOnlyMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func isInteractiveGetRoute(routePath string) bool {
	for _, fragment := range []string{
		"/cmdb/hostssh/connect/",
		"/cmdb/hostssh/command/",
		"/cmdb/snmp-devices/:id/ssh/connect",
		"/cmdb/snmp-devices/:id/ssh/command",
		"/k8s/cluster/:id/namespaces/:namespaceName/pods/:podName/terminal",
		"/k8s/cluster/:id/arthas/ws",
		"/ws/task/ansible/",
	} {
		if strings.Contains(routePath, fragment) {
			return true
		}
	}
	return false
}

func continueWithAuthenticatedUser(c *gin.Context, admin *systemmodel.JwtAdmin) {
	c.Set(constant.ContextKeyUserObj, admin)
	if !enforceK8SAssetAuthorization(c, admin) {
		return
	}
	c.Next()
}

func isK8SRootAdmin(admin *systemmodel.JwtAdmin) bool {
	return admin != nil && (admin.ID == 1 || admin.Username == "admin" || admin.Username == "root")
}

func enforceK8SAssetAuthorization(c *gin.Context, admin *systemmodel.JwtAdmin) bool {
	if admin == nil || isK8SRootAdmin(admin) {
		return true
	}

	routePath := c.FullPath()
	if !strings.Contains(routePath, "/k8s/cluster/:id") {
		return true
	}

	clusterID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || clusterID == 0 {
		return true
	}

	authDao := cmdbdao.NewBastionAssetAuthorizationDao()
	namespace := strings.TrimSpace(c.Param("namespaceName"))
	var allowed bool
	if namespace != "" {
		allowed, err = authDao.CheckUserAssetPermission(admin.ID, cmdbmodel.AssetTypeK8SCluster, uint(clusterID), cmdbmodel.AssetScopeNamespace, namespace)
	} else if allowsAnyK8SClusterScope(c.Request.Method, routePath) {
		allowed, err = authDao.CheckUserAssetAccess(admin.ID, cmdbmodel.AssetTypeK8SCluster, uint(clusterID))
	} else {
		allowed, err = authDao.CheckUserAssetPermission(admin.ID, cmdbmodel.AssetTypeK8SCluster, uint(clusterID), cmdbmodel.AssetScopeAll, "")
	}
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, "检查K8S资产授权失败: "+err.Error())
		c.Abort()
		return false
	}
	if !allowed {
		result.Failed(c, http.StatusForbidden, "无K8S集群或命名空间访问权限")
		c.Abort()
		return false
	}
	return true
}

func allowsAnyK8SClusterScope(method, routePath string) bool {
	if method != http.MethodGet {
		return false
	}
	for _, suffix := range []string{
		"/k8s/cluster/:id",
		"/k8s/cluster/:id/status",
		"/k8s/cluster/:id/detail",
		"/k8s/cluster/:id/namespaces",
		"/k8s/cluster/:id/namespaces/search",
		"/k8s/cluster/:id/metrics-components/check",
	} {
		if strings.HasSuffix(routePath, suffix) {
			return true
		}
	}
	return false
}
