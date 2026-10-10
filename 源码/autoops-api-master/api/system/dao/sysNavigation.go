package dao

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"dodevops-api/api/system/model"
	"dodevops-api/common/util"
	. "dodevops-api/pkg/db"

	"gorm.io/gorm"
)

type SysNavigationDao struct{}

func NewSysNavigationDao() SysNavigationDao {
	return SysNavigationDao{}
}

func (d SysNavigationDao) CreateCategory(category *model.SysNavCategory) error {
	now := util.HTime{Time: time.Now()}
	category.CreateTime = now
	category.UpdateTime = now
	if category.Status == 0 {
		category.Status = 1
	}
	return Db.Create(category).Error
}

func (d SysNavigationDao) UpdateCategory(category *model.SysNavCategory) error {
	category.UpdateTime = util.HTime{Time: time.Now()}
	return Db.Save(category).Error
}

func (d SysNavigationDao) GetCategoryByID(id uint) (*model.SysNavCategory, error) {
	var category model.SysNavCategory
	if err := Db.First(&category, id).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

func (d SysNavigationDao) GetCategoryByCode(code string) (*model.SysNavCategory, error) {
	var category model.SysNavCategory
	if err := Db.Where("code = ?", code).First(&category).Error; err != nil {
		return nil, err
	}
	return &category, nil
}

func (d SysNavigationDao) DeleteCategory(id uint) error {
	return Db.Delete(&model.SysNavCategory{}, id).Error
}

func (d SysNavigationDao) CountItemsByCategoryID(id uint) (int64, error) {
	var count int64
	err := Db.Model(&model.SysNavItem{}).Where("category_id = ?", id).Count(&count).Error
	return count, err
}

func (d SysNavigationDao) ListCategories(query model.SysNavCategoryQuery) ([]model.SysNavCategory, error) {
	categories := make([]model.SysNavCategory, 0)
	db := Db.Model(&model.SysNavCategory{})
	if query.Name != "" {
		db = db.Where("name LIKE ?", "%"+query.Name+"%")
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	err := db.Order("sort ASC, id ASC").Find(&categories).Error
	return categories, err
}

func (d SysNavigationDao) CreateItem(item *model.SysNavItem) error {
	now := util.HTime{Time: time.Now()}
	item.CreateTime = now
	item.UpdateTime = now
	if item.Status == 0 {
		item.Status = 1
	}
	if item.OpenMode == "" {
		item.OpenMode = "blank"
	}
	if item.Scope == "" {
		item.Scope = "public"
	}
	return Db.Create(item).Error
}

func (d SysNavigationDao) UpdateItem(item *model.SysNavItem) error {
	item.UpdateTime = util.HTime{Time: time.Now()}
	return Db.Save(item).Error
}

func (d SysNavigationDao) GetItemByID(id uint) (*model.SysNavItem, error) {
	var item model.SysNavItem
	if err := Db.First(&item, id).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (d SysNavigationDao) DeleteItem(id uint) error {
	return Db.Delete(&model.SysNavItem{}, id).Error
}

func (d SysNavigationDao) ListItems(query model.SysNavItemQuery) ([]model.SysNavItemVo, int64, error) {
	items := make([]model.SysNavItemVo, 0)
	countDB := d.applyItemFilters(d.itemBaseQuery(), query)

	var total int64
	if err := countDB.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if query.PageNum <= 0 {
		query.PageNum = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 10
	}

	listDB := d.applyItemFilters(d.itemVoQuery(), query)
	err := listDB.Order("sni.sort ASC, sni.id DESC").
		Limit(query.PageSize).
		Offset((query.PageNum - 1) * query.PageSize).
		Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}
	d.fillVisibleIDLists(items)
	return items, total, nil
}

func (d SysNavigationDao) ListVisibleItems(userID uint, deptID int, roleIDs []uint, keyword string, categoryID uint) ([]model.SysNavItemVo, error) {
	items := make([]model.SysNavItemVo, 0)
	db := d.itemVoQuery().
		Where("sni.status = ?", 1).
		Where("snc.status = ?", 1)

	visibleSQL := []string{
		"sni.scope = ?",
		"(sni.scope = ? AND sni.owner_id = ?)",
		"(sni.scope = ? AND " + csvContainsSQL("sni.visible_user_ids", userID) + ")",
	}
	args := []interface{}{"public", "private", userID, "acl"}

	if deptID > 0 {
		visibleSQL = append(visibleSQL, "(sni.scope = ? AND "+csvContainsSQL("sni.visible_dept_ids", uint(deptID))+")")
		args = append(args, "acl")
	}
	for _, roleID := range roleIDs {
		visibleSQL = append(visibleSQL, "(sni.scope = ? AND "+csvContainsSQL("sni.visible_role_ids", roleID)+")")
		args = append(args, "acl")
	}

	db = db.Where(strings.Join(visibleSQL, " OR "), args...)
	if categoryID > 0 {
		db = db.Where("sni.category_id = ?", categoryID)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("(sni.name LIKE ? OR sni.description LIKE ? OR sni.tags LIKE ? OR snc.name LIKE ?)", like, like, like, like)
	}

	err := db.Order("snc.sort ASC, sni.sort ASC, sni.id DESC").Scan(&items).Error
	if err != nil {
		return nil, err
	}
	d.fillVisibleIDLists(items)
	return items, nil
}

func (d SysNavigationDao) GetUserDeptID(userID uint) (int, error) {
	var admin model.SysAdmin
	if err := Db.Select("dept_id").First(&admin, userID).Error; err != nil {
		return 0, err
	}
	return admin.DeptId, nil
}

func (d SysNavigationDao) GetUserRoleIDs(userID uint) ([]uint, error) {
	rows := make([]model.SysAdminRole, 0)
	if err := Db.Where("admin_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	roleIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		roleIDs = append(roleIDs, row.RoleId)
	}
	return roleIDs, nil
}

func (d SysNavigationDao) applyItemFilters(db *gorm.DB, query model.SysNavItemQuery) *gorm.DB {
	if query.CategoryID > 0 {
		db = db.Where("sni.category_id = ?", query.CategoryID)
	}
	if query.Name != "" {
		db = db.Where("sni.name LIKE ?", "%"+query.Name+"%")
	}
	if query.Scope != "" {
		db = db.Where("sni.scope = ?", query.Scope)
	}
	if query.Status != "" {
		db = db.Where("sni.status = ?", query.Status)
	}
	if query.OwnerID > 0 {
		db = db.Where("sni.owner_id = ?", query.OwnerID)
	}
	return db
}

func (d SysNavigationDao) itemBaseQuery() *gorm.DB {
	return Db.Table("sys_nav_item AS sni").
		Joins("LEFT JOIN sys_nav_category AS snc ON snc.id = sni.category_id")
}

func (d SysNavigationDao) itemVoQuery() *gorm.DB {
	return d.itemBaseQuery().
		Select(`sni.id,
			sni.category_id,
			snc.name AS category_name,
			sni.name,
			sni.url,
			sni.icon,
			sni.description,
			sni.environment,
			sni.tags,
			sni.scope,
			sni.owner_id,
			sni.owner_name,
			sni.visible_user_ids AS visible_user_ids,
			sni.visible_role_ids AS visible_role_ids,
			sni.visible_dept_ids AS visible_dept_ids,
			sni.sso_enabled,
			sni.sso_mode,
			sni.open_mode,
			sni.sort,
			sni.status,
			sni.create_time,
			sni.update_time`)
}

func (d SysNavigationDao) fillVisibleIDLists(items []model.SysNavItemVo) {
	for i := range items {
		items[i].VisibleUserIDs = parseUintCSV(items[i].VisibleUserText)
		items[i].VisibleRoleIDs = parseUintCSV(items[i].VisibleRoleText)
		items[i].VisibleDeptIDs = parseUintCSV(items[i].VisibleDeptText)
		items[i].URL = model.NormalizeNavURL(items[i].URL)
	}
}

func JoinUintCSV(ids []uint) string {
	parts := make([]string, 0, len(ids))
	seen := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		parts = append(parts, strconv.FormatUint(uint64(id), 10))
	}
	return strings.Join(parts, ",")
}

func parseUintCSV(value string) []uint {
	if value == "" {
		return []uint{}
	}
	parts := strings.Split(value, ",")
	result := make([]uint, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseUint(part, 10, 64)
		if err != nil || id == 0 {
			continue
		}
		result = append(result, uint(id))
	}
	return result
}

func csvContainsSQL(column string, id uint) string {
	value := strconv.FormatUint(uint64(id), 10)
	return fmt.Sprintf("CONCAT(',', COALESCE(%s, ''), ',') LIKE '%%,%s,%%'", column, value)
}
