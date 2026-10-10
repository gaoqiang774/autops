package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构体
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// PageResult 分页结果
type PageResult struct {
	Rows  interface{} `json:"rows"`
	Total int64       `json:"total"`
}

// 状态码定义
const (
	SUCCESS = 200
	ERROR   = 500
)

// Result 通用返回函数
func Result(code int, message string, data interface{}, c *gin.Context) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// OkWithData 成功返回带数据
func OkWithData(data interface{}, c *gin.Context) {
	Result(SUCCESS, "操作成功", data, c)
}

// OkWithMessage 成功返回带消息
func OkWithMessage(message string, c *gin.Context) {
	Result(SUCCESS, message, nil, c)
}

// Ok 成功返回
func Ok(c *gin.Context) {
	Result(SUCCESS, "操作成功", nil, c)
}

// Fail 失败返回
func Fail(c *gin.Context) {
	Result(ERROR, "操作失败", nil, c)
}

// FailWithMessage 失败返回带消息
func FailWithMessage(message string, c *gin.Context) {
	Result(ERROR, message, nil, c)
}

// FailWithData 失败返回带数据
func FailWithData(data interface{}, c *gin.Context) {
	Result(ERROR, "操作失败", data, c)
}
