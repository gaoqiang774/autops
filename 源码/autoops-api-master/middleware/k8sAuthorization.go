package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	cmdbdao "dodevops-api/api/cmdb/dao"
	cmdbmodel "dodevops-api/api/cmdb/model"
	systemauthz "dodevops-api/api/system/authz"
	"dodevops-api/common/result"
	"dodevops-api/pkg/jwt"

	"github.com/gin-gonic/gin"
)

type K8SAction string

const (
	K8SActionRead      K8SAction = "read"
	K8SActionTerminal  K8SAction = "terminal"
	K8SActionFileRead  K8SAction = "file_read"
	K8SActionSensitive K8SAction = "sensitive"
	K8SActionExecute   K8SAction = "execute"
	K8SActionWrite     K8SAction = "write"
)

const K8SActionContextKey = "aiops.k8s.action"
const k8sAuditContextKey = "aiops.k8s.audit"

type k8sAuditRecord struct {
	Action     K8SAction
	Capability string
	ClusterID  uint
	Namespace  string
	Result     string
}

func (action K8SAction) valid() bool {
	switch action {
	case K8SActionRead, K8SActionTerminal, K8SActionFileRead, K8SActionSensitive, K8SActionExecute, K8SActionWrite:
		return true
	default:
		return false
	}
}

// RequireK8SAction enforces the capability attached to the target asset grant.
func RequireK8SAction(action K8SAction) gin.HandlerFunc {
	return func(c *gin.Context) {
		if AuthorizeK8SAction(c, action) {
			c.Next()
		}
	}
}

// AuthorizeK8SAction applies the same policy from handlers that only discover
// they target Kubernetes after parsing a mixed-resource request.
func AuthorizeK8SAction(c *gin.Context, action K8SAction) bool {
	if !action.valid() {
		result.Failed(c, http.StatusInternalServerError, "K8S接口操作类型未正确配置")
		c.Abort()
		return false
	}

	c.Set(K8SActionContextKey, action)
	routePath := c.FullPath()
	standardClusterTarget := strings.Contains(routePath, "/k8s/cluster/:id")
	parsedClusterID, parseErr := strconv.ParseUint(c.Param("id"), 10, 64)
	clusterID := uint(parsedClusterID)
	if !standardClusterTarget {
		clusterID = 0
	}
	namespace := strings.TrimSpace(c.Param("namespaceName"))
	requiredCapability := capabilityForK8SAction(action, namespace)
	if action == K8SActionSensitive {
		requiredCapability = "system_admin"
	}

	isSystemAdmin, err := systemauthz.IsSystemAdmin(c)
	if err != nil {
		result.Failed(c, http.StatusInternalServerError, err.Error())
		c.Abort()
		return false
	}
	if isSystemAdmin {
		appendK8SAuditRecord(c, action, requiredCapability, clusterID, namespace, "allowed")
		return true
	}
	if action == K8SActionSensitive {
		appendK8SAuditRecord(c, action, requiredCapability, clusterID, namespace, "denied")
		result.Failed(c, http.StatusForbidden, "仅系统超级管理员可以执行K8S敏感操作")
		c.Abort()
		return false
	}

	if !standardClusterTarget || parseErr != nil || clusterID == 0 {
		// Collection and mixed-resource endpoints authorize their explicit target
		// after parsing query/body data in the handler.
		if action == K8SActionRead || action == K8SActionTerminal || action == K8SActionFileRead {
			appendK8SAuditRecord(c, action, requiredCapability, 0, "", "deferred")
			return true
		}
		appendK8SAuditRecord(c, action, requiredCapability, 0, "", "denied")
		result.Failed(c, http.StatusForbidden, "K8S写操作缺少可授权的目标集群")
		c.Abort()
		return false
	}

	allowAnyScope := action == K8SActionRead && allowsAnyK8SClusterScope(c.Request.Method, routePath)
	allowed, err := CheckK8SCapabilityAuthorization(c, clusterID, namespace, allowAnyScope, requiredCapability)
	if err != nil {
		appendK8SAuditRecord(c, action, requiredCapability, clusterID, namespace, "error")
		result.Failed(c, http.StatusInternalServerError, "检查K8S资产能力失败: "+err.Error())
		c.Abort()
		return false
	}
	if allowed {
		appendK8SAuditRecord(c, action, requiredCapability, clusterID, namespace, "allowed")
		return true
	}

	appendK8SAuditRecord(c, action, requiredCapability, clusterID, namespace, "denied")
	result.Failed(c, http.StatusForbidden, "当前K8S资产授权不包含所需能力: "+requiredCapability)
	c.Abort()
	return false
}

func appendK8SAuditRecord(c *gin.Context, action K8SAction, capability string, clusterID uint, namespace, auditResult string) {
	records := make([]k8sAuditRecord, 0, 1)
	if existing, ok := c.Get(k8sAuditContextKey); ok {
		if typed, ok := existing.([]k8sAuditRecord); ok {
			records = append(records, typed...)
		}
	}
	records = append(records, k8sAuditRecord{
		Action:     action,
		Capability: strings.TrimSpace(capability),
		ClusterID:  clusterID,
		Namespace:  strings.TrimSpace(namespace),
		Result:     strings.TrimSpace(auditResult),
	})
	c.Set(k8sAuditContextKey, records)
}

func k8sAuditDescription(c *gin.Context) (string, bool) {
	value, ok := c.Get(k8sAuditContextKey)
	if !ok {
		return "", false
	}
	records, ok := value.([]k8sAuditRecord)
	if !ok || len(records) == 0 {
		return "", false
	}
	parts := make([]string, 0, len(records))
	for _, record := range records {
		parts = append(parts, fmt.Sprintf(
			"action=%s capability=%s cluster=%d namespace=%s authorization=%s",
			record.Action,
			record.Capability,
			record.ClusterID,
			record.Namespace,
			record.Result,
		))
	}
	return "K8S操作: " + strings.Join(parts, "; "), true
}

func hasAuditableK8SAction(c *gin.Context) bool {
	value, ok := c.Get(k8sAuditContextKey)
	if !ok {
		return false
	}
	records, ok := value.([]k8sAuditRecord)
	if !ok {
		return false
	}
	for _, record := range records {
		if record.Action != K8SActionRead {
			return true
		}
	}
	return false
}

func capabilityForK8SAction(action K8SAction, namespace string) string {
	switch action {
	case K8SActionRead:
		return "read"
	case K8SActionTerminal:
		return "terminal"
	case K8SActionFileRead:
		return "file_read"
	case K8SActionExecute:
		return "execute"
	case K8SActionWrite:
		if strings.TrimSpace(namespace) != "" {
			return "write_namespaced"
		}
		return "write_cluster"
	default:
		return ""
	}
}

func CheckK8SCapabilityAuthorization(c *gin.Context, clusterID uint, namespace string, allowAnyScope bool, capability string) (bool, error) {
	isSystemAdmin, err := systemauthz.IsSystemAdmin(c)
	if err != nil {
		return false, err
	}
	if isSystemAdmin {
		return true, nil
	}
	admin, err := jwt.GetAdmin(c)
	if err != nil || admin == nil {
		return false, err
	}
	authDao := cmdbdao.NewBastionAssetAuthorizationDao()
	return authDao.CheckUserAssetAccess(admin.ID, cmdbmodel.AssetTypeK8SCluster, clusterID)
}
