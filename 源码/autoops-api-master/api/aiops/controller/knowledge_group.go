package controller

import (
	"strconv"

	"dodevops-api/api/aiops/model"
	"dodevops-api/api/aiops/service"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

// @Summary 查询知识库分组树
// @Tags AIOps知识库
// @Success 200 {object} result.Result
// @router /api/v1/aiops/knowledge/groups/tree [get]
// @Security ApiKeyAuth
func ListKnowledgeGroupTree(c *gin.Context) {
	data, err := service.NewKnowledgeGroupService().ListTree()
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "查询知识分组失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

// @Summary 创建知识库分组
// @Tags AIOps知识库
// @Param data body model.KnowledgeGroupDTO true "知识分组"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/knowledge/groups [post]
// @Security ApiKeyAuth
func CreateKnowledgeGroup(c *gin.Context) {
	var dto model.KnowledgeGroupDTO
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	data, err := service.NewKnowledgeGroupService().Create(c, &dto)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "创建知识分组失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

// @Summary 更新或移动知识库分组
// @Tags AIOps知识库
// @Param id path int true "分组ID"
// @Param data body model.KnowledgeGroupDTO true "知识分组"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/knowledge/groups/:id [put]
// @Security ApiKeyAuth
func UpdateKnowledgeGroup(c *gin.Context) {
	id, ok := bindKnowledgeGroupID(c)
	if !ok {
		return
	}
	var dto model.KnowledgeGroupDTO
	if err := c.BindJSON(&dto); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	data, err := service.NewKnowledgeGroupService().Update(c, id, &dto)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "更新知识分组失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

// @Summary 级联删除知识库分组
// @Description 删除指定分组、全部子分组，以及这些分组下的文档和检索分块
// @Tags AIOps知识库
// @Param id path int true "分组ID"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/knowledge/groups/:id [delete]
// @Security ApiKeyAuth
func DeleteKnowledgeGroup(c *gin.Context) {
	id, ok := bindKnowledgeGroupID(c)
	if !ok {
		return
	}
	if err := service.NewKnowledgeGroupService().Delete(id); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "删除知识分组失败: "+err.Error())
		return
	}
	result.Success(c, true)
}

func bindKnowledgeGroupID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return 0, false
	}
	return uint(id), true
}
