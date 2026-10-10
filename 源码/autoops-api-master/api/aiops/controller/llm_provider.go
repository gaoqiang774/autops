package controller

import (
	"strconv"

	"dodevops-api/api/aiops/model"
	"dodevops-api/api/aiops/service"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

// @Summary 新增 LLM 供应商
// @Description 新增 AIOps LLM 供应商配置
// @Produce json
// @Tags AIOps模型管理
// @Param data body model.CreateLLMProviderDto true "LLM供应商配置"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/llm/providers [post]
// @Security ApiKeyAuth
func CreateLLMProvider(c *gin.Context) {
	var dto model.CreateLLMProviderDto
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}

	provider, err := service.NewLLMProviderService().Create(&dto)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "创建 LLM 供应商失败: "+err.Error())
		return
	}
	result.Success(c, provider)
}

// @Summary 更新 LLM 供应商
// @Description 更新 AIOps LLM 供应商配置
// @Produce json
// @Tags AIOps模型管理
// @Param data body model.UpdateLLMProviderDto true "LLM供应商配置"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/llm/providers [put]
// @Security ApiKeyAuth
func UpdateLLMProvider(c *gin.Context) {
	var dto model.UpdateLLMProviderDto
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}

	provider, err := service.NewLLMProviderService().Update(&dto)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "更新 LLM 供应商失败: "+err.Error())
		return
	}
	result.Success(c, provider)
}

// @Summary 删除 LLM 供应商
// @Description 根据 ID 删除 AIOps LLM 供应商
// @Produce json
// @Tags AIOps模型管理
// @Param id query int true "LLM供应商ID"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/llm/providers [delete]
// @Security ApiKeyAuth
func DeleteLLMProvider(c *gin.Context) {
	id, ok := bindAIOpsIDQuery(c)
	if !ok {
		return
	}

	if err := service.NewLLMProviderService().Delete(id); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "删除 LLM 供应商失败: "+err.Error())
		return
	}
	result.Success(c, true)
}

// @Summary 查询 LLM 供应商详情
// @Description 根据 ID 查询 AIOps LLM 供应商详情
// @Produce json
// @Tags AIOps模型管理
// @Param id query int true "LLM供应商ID"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/llm/providers [get]
// @Security ApiKeyAuth
func GetLLMProviderByID(c *gin.Context) {
	id, ok := bindAIOpsIDQuery(c)
	if !ok {
		return
	}

	provider, err := service.NewLLMProviderService().GetByID(id)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "查询 LLM 供应商失败: "+err.Error())
		return
	}
	result.Success(c, provider)
}

// @Summary 查询 LLM 供应商列表
// @Description 分页查询 AIOps LLM 供应商
// @Produce json
// @Tags AIOps模型管理
// @Param page query int false "页码"
// @Param pageSize query int false "每页数量"
// @Param keyword query string false "关键词"
// @Param providerType query string false "供应商类型"
// @Param protocol query string false "协议"
// @Param status query int false "状态"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/llm/providers/list [get]
// @Security ApiKeyAuth
func ListLLMProviders(c *gin.Context) {
	var query model.ListLLMProviderQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}

	providers, total, err := service.NewLLMProviderService().List(query)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "查询 LLM 供应商列表失败: "+err.Error())
		return
	}

	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 10
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	result.SuccessWithPage(c, providers, total, query.Page, query.PageSize)
}

// @Summary 查询已启用 LLM 供应商
// @Description 查询所有已启用的 AIOps LLM 供应商
// @Produce json
// @Tags AIOps模型管理
// @Success 200 {object} result.Result
// @router /api/v1/aiops/llm/providers/enabled [get]
// @Security ApiKeyAuth
func GetEnabledLLMProviders(c *gin.Context) {
	providers, err := service.NewLLMProviderService().GetEnabled()
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "查询已启用 LLM 供应商失败: "+err.Error())
		return
	}
	result.Success(c, providers)
}

// @Summary 设置默认 LLM 供应商
// @Description 根据 ID 设置默认 AIOps LLM 供应商
// @Produce json
// @Tags AIOps模型管理
// @Param id query int true "LLM供应商ID"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/llm/providers/default [post]
// @Security ApiKeyAuth
func SetDefaultLLMProvider(c *gin.Context) {
	id, ok := bindAIOpsIDQuery(c)
	if !ok {
		return
	}

	provider, err := service.NewLLMProviderService().SetDefault(id)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "设置默认 LLM 供应商失败: "+err.Error())
		return
	}
	result.Success(c, provider)
}

// @Summary 获取 LLM 供应商密钥
// @Description 根据 ID 解密返回 AIOps LLM 供应商密钥
// @Produce json
// @Tags AIOps模型管理
// @Param id query int true "LLM供应商ID"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/llm/providers/secret [post]
// @Security ApiKeyAuth
func GetLLMProviderSecret(c *gin.Context) {
	id, ok := bindAIOpsIDQuery(c)
	if !ok {
		return
	}

	secret, err := service.NewLLMProviderService().GetSecret(id)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "获取 LLM 供应商密钥失败: "+err.Error())
		return
	}
	result.Success(c, secret)
}

// @Summary 测试 LLM 供应商连通性
// @Description 测试 AIOps LLM 供应商接口连通性
// @Produce json
// @Tags AIOps模型管理
// @Param data body model.TestLLMProviderDto false "测试配置"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/llm/providers/test [post]
// @Security ApiKeyAuth
func TestLLMProviderConnection(c *gin.Context) {
	var dto model.TestLLMProviderDto
	_ = c.ShouldBindQuery(&dto)

	if c.Request.ContentLength != 0 {
		var bodyDTO model.TestLLMProviderDto
		if err := c.BindJSON(&bodyDTO); err != nil {
			result.Failed(c, constant.INVALID_PARAMS, "参数错误")
			return
		}
		if bodyDTO.ID == 0 {
			bodyDTO.ID = dto.ID
		}
		if bodyDTO.Message == "" {
			bodyDTO.Message = dto.Message
		}
		if bodyDTO.TimeoutSeconds == 0 {
			bodyDTO.TimeoutSeconds = dto.TimeoutSeconds
		}
		dto = bodyDTO
	}

	testResult, err := service.NewLLMProviderService().TestConnection(&dto)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "测试 LLM 供应商失败: "+err.Error())
		return
	}
	result.Success(c, testResult)
}

// @Summary 查询 LLM 供应商类型
// @Description 查询 AIOps 支持的 LLM 供应商类型
// @Produce json
// @Tags AIOps模型管理
// @Success 200 {object} result.Result
// @router /api/v1/aiops/llm/providers/types [get]
// @Security ApiKeyAuth
func GetLLMProviderTypes(c *gin.Context) {
	result.Success(c, service.NewLLMProviderService().ProviderTypes())
}

func bindAIOpsIDQuery(c *gin.Context) (uint, bool) {
	idStr := c.Query("id")
	if idStr == "" {
		result.Failed(c, constant.INVALID_PARAMS, "ID不能为空")
		return 0, false
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil || id == 0 {
		result.Failed(c, constant.INVALID_PARAMS, "无效的ID")
		return 0, false
	}
	return uint(id), true
}
