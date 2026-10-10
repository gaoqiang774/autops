package authz

import (
	"errors"
	"fmt"

	systemdao "dodevops-api/api/system/dao"
	systemmodel "dodevops-api/api/system/model"
	"dodevops-api/common/constant"

	"github.com/gin-gonic/gin"
)

const systemAdminContextKey = "aiops.system_admin_role"

type systemAdminRoleResolver func(userID uint) (bool, error)

// IsSystemAdmin resolves unrestricted access from the current user's active
// roles and caches the result for the lifetime of the request.
func IsSystemAdmin(c *gin.Context) (bool, error) {
	return resolveSystemAdmin(c, systemdao.IsSystemAdminUser)
}

func resolveSystemAdmin(c *gin.Context, resolver systemAdminRoleResolver) (bool, error) {
	if c == nil {
		return false, errors.New("请求上下文不能为空")
	}
	if cached, exists := c.Get(systemAdminContextKey); exists {
		isAdmin, ok := cached.(bool)
		if !ok {
			return false, errors.New("超级管理员角色缓存无效")
		}
		return isAdmin, nil
	}

	userObj, exists := c.Get(constant.ContextKeyUserObj)
	admin, ok := userObj.(*systemmodel.JwtAdmin)
	if !exists || !ok || admin == nil || admin.ID == 0 {
		return false, errors.New("未获取到当前登录用户")
	}

	isAdmin, err := resolver(admin.ID)
	if err != nil {
		return false, fmt.Errorf("查询超级管理员角色失败: %w", err)
	}
	c.Set(systemAdminContextKey, isAdmin)
	return isAdmin, nil
}
