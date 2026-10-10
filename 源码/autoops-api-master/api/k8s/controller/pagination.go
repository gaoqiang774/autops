package controller

import (
	"strconv"

	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

func parseK8sListPagination(c *gin.Context) (page, pageSize int, ok bool) {
	if pageStr := c.Query("page"); pageStr != "" {
		parsedPage, err := strconv.Atoi(pageStr)
		if err != nil || parsedPage < 0 {
			result.Failed(c, 400, "page参数必须为非负整数")
			return 0, 0, false
		}
		page = parsedPage
	}

	if pageSizeStr := c.Query("pageSize"); pageSizeStr != "" {
		parsedPageSize, err := strconv.Atoi(pageSizeStr)
		if err != nil || parsedPageSize < 0 {
			result.Failed(c, 400, "pageSize参数必须为非负整数")
			return 0, 0, false
		}
		pageSize = parsedPageSize
	} else if sizeStr := c.Query("size"); sizeStr != "" {
		parsedPageSize, err := strconv.Atoi(sizeStr)
		if err != nil || parsedPageSize < 0 {
			result.Failed(c, 400, "size参数必须为非负整数")
			return 0, 0, false
		}
		pageSize = parsedPageSize
	}

	if page > 0 && pageSize == 0 {
		pageSize = 10
	}
	if pageSize > 200 {
		pageSize = 200
	}

	return page, pageSize, true
}
