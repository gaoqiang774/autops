package controller

import (
	"dodevops-api/api/aiops/service"
	"dodevops-api/common/constant"
	"dodevops-api/common/result"

	"github.com/gin-gonic/gin"
)

// @Summary 查询 AIOps Skill 列表
// @Description 查询从 skills 目录加载的声明式 Skill 清单
// @Produce json
// @Tags AIOps Skills
// @Success 200 {object} result.Result
// @router /api/v1/aiops/skills [get]
// @Security ApiKeyAuth
func ListSkills(c *gin.Context) {
	filters := map[string]string{
		"category":   c.Query("category"),
		"domain":     c.Query("domain"),
		"skillGroup": c.Query("skillGroup"),
		"runtime":    c.Query("runtime"),
		"status":     c.Query("status"),
		"riskLevel":  c.Query("riskLevel"),
		"keyword":    c.Query("keyword"),
	}
	data, err := service.NewSkillCatalogService().List(filters)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "查询 Skill 列表失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

// @Summary 查询 AIOps Skill 分类
// @Description 查询 Skill 分类、领域、分组、状态和风险等级聚合
// @Produce json
// @Tags AIOps Skills
// @Success 200 {object} result.Result
// @router /api/v1/aiops/skills/categories [get]
// @Security ApiKeyAuth
func ListSkillCategories(c *gin.Context) {
	data, err := service.NewSkillCatalogService().Categories()
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "查询 Skill 分类失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

// @Summary 查询 AIOps Skill 详情
// @Description 根据 Skill name 查询声明、正文和文件路径
// @Produce json
// @Tags AIOps Skills
// @Param name path string true "Skill name"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/skills/:name [get]
// @Security ApiKeyAuth
func GetSkill(c *gin.Context) {
	name := c.Param("name")
	data, ok, err := service.NewSkillCatalogService().Get(name)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "查询 Skill 详情失败: "+err.Error())
		return
	}
	if !ok {
		result.Failed(c, constant.INVALID_PARAMS, "Skill 不存在")
		return
	}
	result.Success(c, data)
}

// @Summary 重载 AIOps Skills
// @Description 重新扫描 skills 目录并加载 SKILL.md
// @Produce json
// @Tags AIOps Skills
// @Success 200 {object} result.Result
// @router /api/v1/aiops/skills/reload [post]
// @Security ApiKeyAuth
func ReloadSkills(c *gin.Context) {
	data, err := service.NewSkillCatalogService().Reload()
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "重载 Skill 失败: "+err.Error())
		return
	}
	result.Success(c, data)
}

// @Summary 校验 AIOps Skill 文档
// @Description 校验 SKILL.md 文本内容是否符合 Manifest 规范
// @Produce json
// @Tags AIOps Skills
// @Param data body service.SkillValidateRequest true "Skill 文档内容"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/skills/validate [post]
// @Security ApiKeyAuth
func ValidateSkill(c *gin.Context) {
	var req service.SkillValidateRequest
	if err := c.BindJSON(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}
	result.Success(c, service.NewSkillCatalogService().Validate(req))
}

// @Summary 执行 AIOps Skill
// @Description 通过声明式 Skill Runtime 执行 enabled + readonly Skill，并写入 run、step、evidence 审计
// @Produce json
// @Tags AIOps Skills
// @Param data body service.SkillExecuteRequest true "Skill 执行请求"
// @Success 200 {object} result.Result
// @router /api/v1/aiops/skills/execute [post]
// @Security ApiKeyAuth
func ExecuteSkill(c *gin.Context) {
	var req service.SkillExecuteRequest
	if err := c.BindJSON(&req); err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "参数错误")
		return
	}

	data, err := service.NewSkillRuntimeService().Execute(c, &req)
	if err != nil {
		result.Failed(c, constant.INVALID_PARAMS, "执行 Skill 失败: "+err.Error())
		return
	}
	result.Success(c, data)
}
