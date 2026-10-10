package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"dodevops-api/api/system/dao"
	"dodevops-api/api/system/model"
	"dodevops-api/common/result"
	"dodevops-api/pkg/jwt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SysNavigationService interface {
	CreateCategory(c *gin.Context, dto model.SysNavCategoryDto)
	UpdateCategory(c *gin.Context, dto model.SysNavCategoryDto)
	DeleteCategory(c *gin.Context, dto model.SysNavIDDto)
	GetCategory(c *gin.Context, id uint)
	ListCategories(c *gin.Context, query model.SysNavCategoryQuery)
	CreateItem(c *gin.Context, dto model.SysNavItemDto)
	UpdateItem(c *gin.Context, dto model.SysNavItemDto)
	DeleteItem(c *gin.Context, dto model.SysNavIDDto)
	GetItem(c *gin.Context, id uint)
	ListItems(c *gin.Context, query model.SysNavItemQuery)
	GetVisibleNavigation(c *gin.Context, keyword string, categoryID uint)
}

type SysNavigationServiceImpl struct {
	navDao dao.SysNavigationDao
}

func NewSysNavigationService() SysNavigationService {
	return &SysNavigationServiceImpl{
		navDao: dao.NewSysNavigationDao(),
	}
}

func (s *SysNavigationServiceImpl) CreateCategory(c *gin.Context, dto model.SysNavCategoryDto) {
	if dto.Name == "" || dto.Code == "" {
		result.Failed(c, int(result.ApiCode.FAILED), "分类名称和编码不能为空")
		return
	}
	if existing, err := s.navDao.GetCategoryByCode(dto.Code); err == nil && existing.ID > 0 {
		result.Failed(c, int(result.ApiCode.FAILED), "分类编码已存在")
		return
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		result.Failed(c, int(result.ApiCode.FAILED), fmt.Sprintf("查询分类失败: %v", err))
		return
	}

	admin, _ := jwt.GetAdmin(c)
	category := &model.SysNavCategory{
		Name:        dto.Name,
		Code:        dto.Code,
		Icon:        dto.Icon,
		Description: dto.Description,
		Sort:        dto.Sort,
		Status:      defaultStatus(dto.Status),
	}
	if admin != nil {
		category.CreateBy = admin.ID
	}

	if err := s.navDao.CreateCategory(category); err != nil {
		result.Failed(c, int(result.ApiCode.FAILED), fmt.Sprintf("创建分类失败: %v", err))
		return
	}
	result.Success(c, category)
}

func (s *SysNavigationServiceImpl) UpdateCategory(c *gin.Context, dto model.SysNavCategoryDto) {
	category, err := s.navDao.GetCategoryByID(dto.Id)
	if err != nil {
		result.Failed(c, int(result.ApiCode.FAILED), "分类不存在")
		return
	}
	if dto.Name == "" || dto.Code == "" {
		result.Failed(c, int(result.ApiCode.FAILED), "分类名称和编码不能为空")
		return
	}
	if existing, err := s.navDao.GetCategoryByCode(dto.Code); err == nil && existing.ID != dto.Id {
		result.Failed(c, int(result.ApiCode.FAILED), "分类编码已存在")
		return
	} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		result.Failed(c, int(result.ApiCode.FAILED), fmt.Sprintf("查询分类失败: %v", err))
		return
	}

	category.Name = dto.Name
	category.Code = dto.Code
	category.Icon = dto.Icon
	category.Description = dto.Description
	category.Sort = dto.Sort
	category.Status = defaultStatus(dto.Status)

	if err := s.navDao.UpdateCategory(category); err != nil {
		result.Failed(c, int(result.ApiCode.FAILED), fmt.Sprintf("更新分类失败: %v", err))
		return
	}
	result.Success(c, category)
}

func (s *SysNavigationServiceImpl) DeleteCategory(c *gin.Context, dto model.SysNavIDDto) {
	count, err := s.navDao.CountItemsByCategoryID(dto.Id)
	if err != nil {
		result.Failed(c, int(result.ApiCode.FAILED), fmt.Sprintf("检查分类失败: %v", err))
		return
	}
	if count > 0 {
		result.Failed(c, int(result.ApiCode.FAILED), "分类下存在导航链接，不能删除")
		return
	}
	if err := s.navDao.DeleteCategory(dto.Id); err != nil {
		result.Failed(c, int(result.ApiCode.FAILED), fmt.Sprintf("删除分类失败: %v", err))
		return
	}
	result.Success(c, true)
}

func (s *SysNavigationServiceImpl) GetCategory(c *gin.Context, id uint) {
	category, err := s.navDao.GetCategoryByID(id)
	if err != nil {
		result.Failed(c, int(result.ApiCode.FAILED), "分类不存在")
		return
	}
	result.Success(c, category)
}

func (s *SysNavigationServiceImpl) ListCategories(c *gin.Context, query model.SysNavCategoryQuery) {
	categories, err := s.navDao.ListCategories(query)
	if err != nil {
		result.Failed(c, int(result.ApiCode.FAILED), fmt.Sprintf("查询分类失败: %v", err))
		return
	}
	result.Success(c, categories)
}

func (s *SysNavigationServiceImpl) CreateItem(c *gin.Context, dto model.SysNavItemDto) {
	admin, err := jwt.GetAdmin(c)
	if err != nil || admin == nil {
		result.Failed(c, int(result.ApiCode.NOAUTH), result.ApiCode.GetMessage(result.ApiCode.NOAUTH))
		return
	}
	item, ok := s.buildItemFromDto(dto)
	if !ok {
		result.Failed(c, int(result.ApiCode.FAILED), "导航名称、地址、分类不能为空，scope 必须是 public/private/acl")
		return
	}
	if _, err := s.navDao.GetCategoryByID(dto.CategoryID); err != nil {
		result.Failed(c, int(result.ApiCode.FAILED), "分类不存在")
		return
	}
	item.OwnerID = admin.ID
	item.OwnerName = admin.Username

	if err := s.navDao.CreateItem(item); err != nil {
		result.Failed(c, int(result.ApiCode.FAILED), fmt.Sprintf("创建导航失败: %v", err))
		return
	}
	result.Success(c, itemToVo(*item, ""))
}

func (s *SysNavigationServiceImpl) UpdateItem(c *gin.Context, dto model.SysNavItemDto) {
	item, err := s.navDao.GetItemByID(dto.Id)
	if err != nil {
		result.Failed(c, int(result.ApiCode.FAILED), "导航不存在")
		return
	}
	next, ok := s.buildItemFromDto(dto)
	if !ok {
		result.Failed(c, int(result.ApiCode.FAILED), "导航名称、地址、分类不能为空，scope 必须是 public/private/acl")
		return
	}
	if _, err := s.navDao.GetCategoryByID(dto.CategoryID); err != nil {
		result.Failed(c, int(result.ApiCode.FAILED), "分类不存在")
		return
	}

	item.CategoryID = next.CategoryID
	item.Name = next.Name
	item.URL = next.URL
	item.Icon = next.Icon
	item.Description = next.Description
	item.Environment = next.Environment
	item.Tags = next.Tags
	item.Scope = next.Scope
	item.VisibleUserIDs = next.VisibleUserIDs
	item.VisibleRoleIDs = next.VisibleRoleIDs
	item.VisibleDeptIDs = next.VisibleDeptIDs
	item.SSOEnabled = next.SSOEnabled
	item.SSOMode = next.SSOMode
	item.OpenMode = next.OpenMode
	item.Sort = next.Sort
	item.Status = next.Status

	if err := s.navDao.UpdateItem(item); err != nil {
		result.Failed(c, int(result.ApiCode.FAILED), fmt.Sprintf("更新导航失败: %v", err))
		return
	}
	result.Success(c, itemToVo(*item, ""))
}

func (s *SysNavigationServiceImpl) DeleteItem(c *gin.Context, dto model.SysNavIDDto) {
	if err := s.navDao.DeleteItem(dto.Id); err != nil {
		result.Failed(c, int(result.ApiCode.FAILED), fmt.Sprintf("删除导航失败: %v", err))
		return
	}
	result.Success(c, true)
}

func (s *SysNavigationServiceImpl) GetItem(c *gin.Context, id uint) {
	item, err := s.navDao.GetItemByID(id)
	if err != nil {
		result.Failed(c, int(result.ApiCode.FAILED), "导航不存在")
		return
	}
	result.Success(c, itemToVo(*item, ""))
}

func (s *SysNavigationServiceImpl) ListItems(c *gin.Context, query model.SysNavItemQuery) {
	items, total, err := s.navDao.ListItems(query)
	if err != nil {
		result.Failed(c, int(result.ApiCode.FAILED), fmt.Sprintf("查询导航失败: %v", err))
		return
	}
	result.Success(c, map[string]interface{}{
		"list":     items,
		"total":    total,
		"pageNum":  query.PageNum,
		"pageSize": query.PageSize,
	})
}

func (s *SysNavigationServiceImpl) GetVisibleNavigation(c *gin.Context, keyword string, categoryID uint) {
	admin, err := jwt.GetAdmin(c)
	if err != nil || admin == nil {
		result.Failed(c, int(result.ApiCode.NOAUTH), result.ApiCode.GetMessage(result.ApiCode.NOAUTH))
		return
	}
	deptID, _ := s.navDao.GetUserDeptID(admin.ID)
	roleIDs, _ := s.navDao.GetUserRoleIDs(admin.ID)
	items, err := s.navDao.ListVisibleItems(admin.ID, deptID, roleIDs, keyword, categoryID)
	if err != nil {
		result.Failed(c, int(result.ApiCode.FAILED), fmt.Sprintf("查询可见导航失败: %v", err))
		return
	}
	categories, err := s.navDao.ListCategories(model.SysNavCategoryQuery{Status: "1"})
	if err != nil {
		result.Failed(c, int(result.ApiCode.FAILED), fmt.Sprintf("查询分类失败: %v", err))
		return
	}

	itemsByCategory := make(map[uint][]model.SysNavItemVo)
	for _, item := range items {
		itemsByCategory[item.CategoryID] = append(itemsByCategory[item.CategoryID], item)
	}

	groups := make([]model.SysNavVisibleGroup, 0, len(categories))
	for _, category := range categories {
		categoryItems := itemsByCategory[category.ID]
		if len(categoryItems) == 0 {
			continue
		}
		groups = append(groups, model.SysNavVisibleGroup{
			Category: category,
			Items:    categoryItems,
		})
	}
	result.Success(c, groups)
}

func (s *SysNavigationServiceImpl) buildItemFromDto(dto model.SysNavItemDto) (*model.SysNavItem, bool) {
	scope := dto.Scope
	if scope == "" {
		scope = "public"
	}
	if dto.CategoryID == 0 || dto.Name == "" || dto.URL == "" || !validNavScope(scope) {
		return nil, false
	}
	item := &model.SysNavItem{
		CategoryID:  dto.CategoryID,
		Name:        dto.Name,
		URL:         model.NormalizeNavURL(dto.URL),
		Icon:        dto.Icon,
		Description: dto.Description,
		Environment: dto.Environment,
		Tags:        dto.Tags,
		Scope:       scope,
		SSOEnabled:  dto.SSOEnabled,
		SSOMode:     dto.SSOMode,
		OpenMode:    dto.OpenMode,
		Sort:        dto.Sort,
		Status:      defaultStatus(dto.Status),
	}
	if item.SSOMode == "" {
		item.SSOMode = "none"
	}
	if item.OpenMode == "" {
		item.OpenMode = "blank"
	}
	if scope == "acl" {
		item.VisibleUserIDs = dao.JoinUintCSV(dto.VisibleUserIDs)
		item.VisibleRoleIDs = dao.JoinUintCSV(dto.VisibleRoleIDs)
		item.VisibleDeptIDs = dao.JoinUintCSV(dto.VisibleDeptIDs)
	}
	return item, true
}

func itemToVo(item model.SysNavItem, categoryName string) model.SysNavItemVo {
	return model.SysNavItemVo{
		ID:             item.ID,
		CategoryID:     item.CategoryID,
		CategoryName:   categoryName,
		Name:           item.Name,
		URL:            model.NormalizeNavURL(item.URL),
		Icon:           item.Icon,
		Description:    item.Description,
		Environment:    item.Environment,
		Tags:           item.Tags,
		Scope:          item.Scope,
		OwnerID:        item.OwnerID,
		OwnerName:      item.OwnerName,
		VisibleUserIDs: parseVisibleCSV(item.VisibleUserIDs),
		VisibleRoleIDs: parseVisibleCSV(item.VisibleRoleIDs),
		VisibleDeptIDs: parseVisibleCSV(item.VisibleDeptIDs),
		SSOEnabled:     item.SSOEnabled,
		SSOMode:        item.SSOMode,
		OpenMode:       item.OpenMode,
		Sort:           item.Sort,
		Status:         item.Status,
		CreateTime:     item.CreateTime,
		UpdateTime:     item.UpdateTime,
	}
}

func parseVisibleCSV(value string) []uint {
	if value == "" {
		return []uint{}
	}
	parts := strings.Split(value, ",")
	ids := make([]uint, 0, len(parts))
	for _, part := range parts {
		id64, err := strconv.ParseUint(strings.TrimSpace(part), 10, 64)
		if err == nil && id64 > 0 {
			ids = append(ids, uint(id64))
		}
	}
	return ids
}

func validNavScope(scope string) bool {
	return scope == "public" || scope == "private" || scope == "acl"
}

func defaultStatus(status int) int {
	if status == 0 {
		return 1
	}
	return status
}
