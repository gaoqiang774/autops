package dao

import (
	"dodevops-api/api/cmdb/model"
	"dodevops-api/common/util"
	"strings"
	"time"

	"gorm.io/gorm"
)

type CmdbSQLDao struct {
	db *gorm.DB
}

func NewCmdbSQLDao(db *gorm.DB) *CmdbSQLDao {
	return &CmdbSQLDao{db: db}
}

func (d *CmdbSQLDao) Create(database *model.CmdbSQL) error {
	now := util.HTime{Time: time.Now()}
	if database.CreatedAt.Time.IsZero() {
		database.CreatedAt = now
	}
	if database.UpdatedAt.Time.IsZero() {
		database.UpdatedAt = now
	}
	return d.db.Create(database).Error
}

func (d *CmdbSQLDao) Update(database *model.CmdbSQL) error {
	if database.UpdatedAt.Time.IsZero() {
		database.UpdatedAt = util.HTime{Time: time.Now()}
	}
	return d.db.Save(database).Error
}

func (d *CmdbSQLDao) UpdateAuthStatus(id uint, authStatus int) error {
	return d.db.Model(&model.CmdbSQL{}).Where("id = ?", id).Updates(map[string]interface{}{
		"auth_status": authStatus,
		"updated_at":  util.HTime{Time: time.Now()},
	}).Error
}

func (d *CmdbSQLDao) Delete(id uint) error {
	return d.db.Delete(&model.CmdbSQL{}, id).Error
}

func (d *CmdbSQLDao) GetByID(id uint) (*model.CmdbSQL, error) {
	var database model.CmdbSQL
	err := d.db.First(&database, id).Error
	return &database, err
}

func (d *CmdbSQLDao) List(page, pageSize int) ([]model.CmdbSQL, int64, error) {
	return d.Query(model.CmdbSQLQuery{Page: page, PageSize: pageSize})
}

func (d *CmdbSQLDao) Query(query model.CmdbSQLQuery) ([]model.CmdbSQL, int64, error) {
	var databases []model.CmdbSQL
	var total int64

	db := d.applyFilters(d.db.Model(&model.CmdbSQL{}), query)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, pageSize := normalizeDatabasePage(query.Page, query.PageSize)
	orderClause := buildDatabaseOrderClause(query.SortBy, query.SortOrder)
	err := db.Order(orderClause).Offset((page - 1) * pageSize).Limit(pageSize).Find(&databases).Error
	return databases, total, err
}

func (d *CmdbSQLDao) CountByType() (map[int]int64, error) {
	type row struct {
		Type  int   `json:"type"`
		Count int64 `json:"count"`
	}

	var rows []row
	if err := d.db.Model(&model.CmdbSQL{}).Select("type, count(*) as count").Group("type").Find(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[int]int64, len(rows))
	for _, item := range rows {
		result[item.Type] = item.Count
	}
	return result, nil
}

func (d *CmdbSQLDao) GetByAccountID(accountID uint) ([]model.CmdbSQL, error) {
	var databases []model.CmdbSQL
	err := d.db.Where("account_id = ?", accountID).Find(&databases).Error
	return databases, err
}

func (d *CmdbSQLDao) GetByGroupID(groupID uint) ([]model.CmdbSQL, error) {
	var databases []model.CmdbSQL
	err := d.db.Where("group_id = ?", groupID).Find(&databases).Error
	return databases, err
}

func (d *CmdbSQLDao) GetByName(name string) ([]model.CmdbSQL, error) {
	var databases []model.CmdbSQL
	err := d.db.Where("name = ? OR instance_name = ?", name, name).Find(&databases).Error
	return databases, err
}

func (d *CmdbSQLDao) GetByType(dbType int) ([]model.CmdbSQL, error) {
	var databases []model.CmdbSQL
	err := d.db.Where("type = ?", dbType).Find(&databases).Error
	return databases, err
}

func (d *CmdbSQLDao) CheckNameExists(name string, excludeID uint) bool {
	var count int64
	query := d.db.Model(&model.CmdbSQL{}).Where("name = ?", name)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	query.Count(&count)
	return count > 0
}

func (d *CmdbSQLDao) applyFilters(db *gorm.DB, query model.CmdbSQLQuery) *gorm.DB {
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where(
			"name LIKE ? OR instance_name LIKE ? OR host_ip LIKE ? OR endpoint LIKE ? OR business_system LIKE ? OR owner LIKE ?",
			like, like, like, like, like, like,
		)
	}
	if query.Type > 0 {
		db = db.Where("type = ?", query.Type)
	}
	if environment := strings.TrimSpace(query.Environment); environment != "" {
		db = db.Where("environment = ?", environment)
	}
	if query.Status > 0 {
		db = db.Where("status = ?", query.Status)
	}
	if query.GroupID > 0 {
		db = db.Where("group_id = ?", query.GroupID)
	}
	if businessSystem := strings.TrimSpace(query.BusinessSystem); businessSystem != "" {
		db = db.Where("business_system = ?", businessSystem)
	}
	if owner := strings.TrimSpace(query.Owner); owner != "" {
		db = db.Where("owner = ?", owner)
	}
	if hostIP := strings.TrimSpace(query.HostIP); hostIP != "" {
		db = db.Where("host_ip = ?", hostIP)
	}
	return db
}

func normalizeDatabasePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func buildDatabaseOrderClause(sortBy, sortOrder string) string {
	fieldMap := map[string]string{
		"id":              "id",
		"name":            "name",
		"type":            "type",
		"status":          "status",
		"hostIp":          "host_ip",
		"host_ip":         "host_ip",
		"environment":     "environment",
		"createdAt":       "created_at",
		"created_at":      "created_at",
		"updatedAt":       "updated_at",
		"updated_at":      "updated_at",
		"businessSystem":  "business_system",
		"business_system": "business_system",
	}

	dbField, ok := fieldMap[strings.TrimSpace(sortBy)]
	if !ok {
		dbField = "updated_at"
	}

	sortOrder = strings.ToLower(strings.TrimSpace(sortOrder))
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	if dbField == "updated_at" {
		return "COALESCE(updated_at, created_at) " + sortOrder + ", id " + sortOrder
	}
	return dbField + " " + sortOrder
}
