package controller

import (
	"dodevops-api/api/configcenter/model"
	"dodevops-api/api/configcenter/service"
	"dodevops-api/common/result"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
)

type AccountAuthController struct {
	service *service.AccountAuthService
}

func NewAccountAuthController() *AccountAuthController {
	return &AccountAuthController{
		service: service.NewAccountAuthService(),
	}
}

// Create 创建账号
// @Summary 创建账号认证信息
// @Description 创建账号认证信息
// @Tags Config配置中心
// @Param account body model.AccountAuth true "账号认证信息"
// @Success 200 {object} result.Result{data=model.AccountAuth}
// @Router /api/v1/config/accountauth [post]
// @Security ApiKeyAuth
func (c *AccountAuthController) Create(ctx *gin.Context) {
	var account model.AccountAuth
	if err := ctx.ShouldBindJSON(&account); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}

	if err := c.service.Create(&account); err != nil {
		result.Failed(ctx, 500, "创建失败: "+err.Error())
		return
	}

	result.Success(ctx, account)
}

// Update 更新账号
// @Summary 更新账号认证信息
// @Description 更新账号认证信息
// @Tags Config配置中心
// @Param account body model.AccountAuth true "账号认证信息"
// @Success 200 {object} result.Result{data=model.AccountAuth}
// @Router /api/v1/config/accountauth [put]
// @Security ApiKeyAuth
func (c *AccountAuthController) Update(ctx *gin.Context) {
	var account model.AccountAuth
	if err := ctx.ShouldBindJSON(&account); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}

	if err := c.service.Update(&account); err != nil {
		result.Failed(ctx, 500, "更新失败: "+err.Error())
		return
	}

	result.Success(ctx, account)
}

// Delete 删除账号
// @Summary 删除账号认证信息
// @Description 删除账号认证信息
// @Tags Config配置中心
// @Param id query uint true "账号ID"
// @Success 200 {object} result.Result
// @Router /api/v1/config/accountauth [delete]
// @Security ApiKeyAuth
func (c *AccountAuthController) Delete(ctx *gin.Context) {
	var req struct {
		ID uint `form:"id" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}
	if err := c.service.Delete(req.ID); err != nil {
		result.Failed(ctx, 500, "删除失败: "+err.Error())
		return
	}

	result.Success(ctx, nil)
}

// GetByID 根据ID获取账号详情
// @Summary 根据ID获取账号详情
// @Description 根据ID获取账号详情
// @Tags Config配置中心
// @Param id query uint true "账号ID"
// @Success 200 {object} result.Result{data=model.AccountAuth}
// @Router /api/v1/config/accountauth [get]
// @Security ApiKeyAuth
func (c *AccountAuthController) GetByID(ctx *gin.Context) {
	var req struct {
		ID uint `form:"id" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}
	account, err := c.service.GetByID(req.ID)
	if err != nil {
		result.Failed(ctx, 500, "查询失败: "+err.Error())
		return
	}

	result.Success(ctx, account)
}

// List 获取账号列表（分页）
// @Summary 获取账号列表（分页）
// @Description 获取账号列表（分页）
// @Tags Config配置中心
// @Param page query int true "页码"
// @Param pageSize query int true "每页数量"
// @Success 200 {object} result.Result{data=result.PageResult{list=[]model.AccountAuth}}
// @Router /api/v1/config/accountauth/list [get]
// @Security ApiKeyAuth
func (c *AccountAuthController) List(ctx *gin.Context) {
	var params PageParams
	if err := ctx.ShouldBindQuery(&params); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}

	accountType, err := parseOptionalAccountType(ctx.Query("type"))
	if err != nil {
		result.Failed(ctx, 400, "invalid account type")
		return
	}

	accounts, total, err := c.service.QueryWithPage(params.Page, params.PageSize, ctx.Query("alias"), accountType)
	if err != nil {
		result.Failed(ctx, 500, "查询失败: "+err.Error())
		return
	}

	pageResult := result.PageResult{
		List:     accounts,
		Total:    total,
		Page:     params.Page,
		PageSize: params.PageSize,
	}
	result.Success(ctx, pageResult)
}

func parseOptionalAccountType(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	accountType, err := strconv.Atoi(value)
	if err != nil || accountType < 0 {
		return 0, fmt.Errorf("invalid account type")
	}
	return accountType, nil
}

// Import 批量导入账号认证信息
// @Summary 批量导入账号认证信息
// @Description 支持两种导入方式：1) JSON文本 tableData；2) multipart/form-data 上传xlsx文件(file)。列顺序：账号别名、账号地址、用户名、密码，账号类型默认=5
// @Tags Config配置中心
// @Param data body model.ImportAccountAuthDto true "导入数据"
// @Success 200 {object} result.Result{data=model.ImportAccountAuthResult}
// @Router /api/v1/config/accountauth/import [post]
// @Security ApiKeyAuth
func (c *AccountAuthController) Import(ctx *gin.Context) {
	contentType := strings.ToLower(ctx.ContentType())
	if strings.HasPrefix(contentType, "multipart/form-data") {
		c.importFromExcel(ctx)
		return
	}

	var dto model.ImportAccountAuthDto
	if err := ctx.ShouldBindJSON(&dto); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}

	importResult, err := c.service.Import(&dto)
	if err != nil {
		result.Failed(ctx, 400, "导入失败: "+err.Error())
		return
	}

	result.Success(ctx, importResult)
}

func (c *AccountAuthController) importFromExcel(ctx *gin.Context) {
	file, err := ctx.FormFile("file")
	if err != nil {
		result.Failed(ctx, 400, "请上传xlsx文件")
		return
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".xlsx" {
		result.Failed(ctx, 400, "仅支持 .xlsx 文件")
		return
	}

	tempDir := "./upload/temp"
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		result.Failed(ctx, 500, "创建临时目录失败: "+err.Error())
		return
	}

	tempFilePath := filepath.Join(tempDir, fmt.Sprintf("accountauth_import_%d%s", time.Now().UnixNano(), ext))
	if err := ctx.SaveUploadedFile(file, tempFilePath); err != nil {
		result.Failed(ctx, 500, "保存上传文件失败: "+err.Error())
		return
	}
	defer os.Remove(tempFilePath)

	excelFile, err := excelize.OpenFile(tempFilePath)
	if err != nil {
		result.Failed(ctx, 500, "打开Excel文件失败")
		return
	}
	defer excelFile.Close()

	sheetName := excelFile.GetSheetName(0)
	if sheetName == "" {
		sheetList := excelFile.GetSheetList()
		if len(sheetList) == 0 {
			result.Failed(ctx, 400, "Excel中没有可读取的工作表")
			return
		}
		sheetName = sheetList[0]
	}

	rows, err := excelFile.GetRows(sheetName)
	if err != nil {
		result.Failed(ctx, 500, "读取Excel数据失败")
		return
	}

	tableData, err := buildAccountAuthImportTableData(rows)
	if err != nil {
		result.Failed(ctx, 400, err.Error())
		return
	}

	dto := model.ImportAccountAuthDto{
		TableData: tableData,
		Type:      parseImportAccountAuthType(ctx.PostForm("type")),
		Remark:    strings.TrimSpace(ctx.PostForm("remark")),
	}
	importResult, err := c.service.Import(&dto)
	if err != nil {
		result.Failed(ctx, 400, "导入失败: "+err.Error())
		return
	}

	result.Success(ctx, importResult)
}

func buildAccountAuthImportTableData(rows [][]string) (string, error) {
	if len(rows) == 0 {
		return "", fmt.Errorf("Excel中没有可导入的数据")
	}

	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		if isExcelRowEmpty(row) {
			continue
		}

		alias := getExcelCell(row, 0)
		host := getExcelCell(row, 1)
		username := getExcelCell(row, 2)
		password := getExcelCell(row, 3)
		lines = append(lines, strings.Join([]string{alias, host, username, password}, "\t"))
	}

	if len(lines) == 0 {
		return "", fmt.Errorf("Excel中没有可导入的数据")
	}

	return strings.Join(lines, "\n"), nil
}

func parseImportAccountAuthType(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}

	accountType, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return accountType
}

// DecryptPassword 解密密码
// @Summary 解密密码
// @Description 解密密码
// @Tags Config配置中心
// @Param id query uint true "账号ID"
// @Success 200 {object} result.Result{data=string}
// @Router /api/v1/config/accountauth/decrypt [post]
// @Security ApiKeyAuth
func (c *AccountAuthController) DecryptPassword(ctx *gin.Context) {
	var req struct {
		ID uint `form:"id" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}
	password, err := c.service.DecryptPassword(req.ID)
	if err != nil {
		result.Failed(ctx, 500, "解密失败: "+err.Error())
		return
	}

	result.Success(ctx, gin.H{
		"password": password,
	})
}

// GetByType 根据类型查询账号
// @Summary 根据类型查询账号
// @Description 根据类型查询账号
// @Tags Config配置中心
// @Param type query string true "账号类型"
// @Success 200 {object} result.Result{data=[]model.AccountAuth}
// @Router /api/v1/config/accountauth/type [get]
// @Security ApiKeyAuth
func (c *AccountAuthController) GetByType(ctx *gin.Context) {
	accountType := ctx.Query("type")
	accounts, err := c.service.GetByType(accountType)
	if err != nil {
		result.Failed(ctx, 500, "查询失败: "+err.Error())
		return
	}
	result.Success(ctx, accounts)
}

// GetByAlias 根据别名查询账号
// @Summary 根据别名查询账号
// @Description 根据别名查询账号
// @Tags Config配置中心
// @Param alias query string true "账号别名"
// @Success 200 {object} result.Result{data=model.AccountAuth}
// @Router /api/v1/config/accountauth/alias [get]
// @Security ApiKeyAuth
func (c *AccountAuthController) GetByAlias(ctx *gin.Context) {
	alias := ctx.Query("alias")
	account, err := c.service.GetByAlias(alias)
	if err != nil {
		result.Failed(ctx, 500, "查询失败: "+err.Error())
		return
	}
	result.Success(ctx, account)
}

// TestConnection 测试账号连接
// @Summary 测试账号连接
// @Description 测试账号连接
// @Tags Config配置中心
// @Param id query uint true "账号ID"
// @Success 200 {object} result.Result{data=map[string]interface{}}
// @Router /api/v1/config/accountauth/test [post]
// @Security ApiKeyAuth
func (c *AccountAuthController) TestConnection(ctx *gin.Context) {
	var req struct {
		ID uint `form:"id" binding:"required"`
	}
	if err := ctx.ShouldBindQuery(&req); err != nil {
		result.Failed(ctx, 400, "参数错误: "+err.Error())
		return
	}

	success, message, err := c.service.TestConnection(req.ID)
	if err != nil {
		result.Failed(ctx, 500, "测试失败: "+err.Error())
		return
	}

	result.Success(ctx, gin.H{
		"success": success,
		"message": message,
	})
}
