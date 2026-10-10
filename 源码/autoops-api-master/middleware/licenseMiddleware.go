package middleware

import (
	"net/http"

	"dodevops-api/common/result"
	"dodevops-api/pkg/license"

	"github.com/gin-gonic/gin"
)

const licenseRequiredMessage = "系统未授权或授权已过期，仅可使用资产管理和系统管理"

// RequireLicense protects every server-side route belonging to a paid module.
// Menu hiding is only presentation; this middleware is the security boundary.
func RequireLicense() gin.HandlerFunc {
	return requireLicense(license.DefaultManager())
}

type licenseStatusProvider interface {
	Status() license.Status
}

func requireLicense(provider licenseStatusProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := provider.Status()
		if status.Licensed {
			c.Next()
			return
		}

		message := licenseRequiredMessage
		if status.State == license.StateExpired {
			message = "系统授权已过期，仅可使用资产管理和系统管理"
		}
		result.Failed(c, http.StatusForbidden, message)
		c.Abort()
	}
}
