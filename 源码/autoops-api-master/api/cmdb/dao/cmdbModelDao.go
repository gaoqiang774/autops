package dao

import (
	"dodevops-api/api/cmdb/model"
	"encoding/json"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CmdbDao struct {
	db *gorm.DB
}

func NewCmdbDao(db *gorm.DB) *CmdbDao {
	return &CmdbDao{db: db}
}

func (d *CmdbDao) DB() *gorm.DB {
	return d.db
}

func (d *CmdbDao) BusinessModelExists(name string) (bool, error) {
	var count int64
	err := d.db.Model(&model.CmdbBusinessModel{}).
		Where("name = ?", strings.TrimSpace(name)).
		Count(&count).Error
	return count > 0, err
}

func (d *CmdbDao) BusinessModelNameExistsExcept(name string, id uint) (bool, error) {
	var count int64
	err := d.db.Model(&model.CmdbBusinessModel{}).
		Where("name = ? AND id <> ?", strings.TrimSpace(name), id).
		Count(&count).Error
	return count > 0, err
}

func (d *CmdbDao) PurgeDeletedBusinessModelByName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}

	return d.db.Transaction(func(tx *gorm.DB) error {
		var businesses []model.CmdbBusinessModel
		if err := tx.Unscoped().
			Where("name = ? AND deleted_at IS NOT NULL", name).
			Find(&businesses).Error; err != nil {
			return err
		}

		for _, business := range businesses {
			if err := tx.Unscoped().Where("business_id = ?", business.ID).Delete(&model.CmdbBusinessModelRelation{}).Error; err != nil {
				return err
			}
			if err := tx.Where("business_id = ?", business.ID).Delete(&model.CmdbBusinessModelHealthSnapshot{}).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Where("business_id = ?", business.ID).Delete(&model.CmdbBusinessModelBinding{}).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Delete(&model.CmdbBusinessModel{}, business.ID).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (d *CmdbDao) CreateBusinessModelWithRoot(business *model.CmdbBusinessModel, root *model.CmdbBusinessModelBinding) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(business).Error; err != nil {
			return err
		}
		root.BusinessID = business.ID
		root.SourceType = "business_model"
		root.SourceID = business.ID
		if err := tx.Create(root).Error; err != nil {
			return err
		}
		return nil
	})
}

func (d *CmdbDao) ListBusinessModels(query model.BusinessModelQuery) ([]model.CmdbBusinessModel, int64, error) {
	var businesses []model.CmdbBusinessModel
	var total int64

	dbQuery := d.db.Model(&model.CmdbBusinessModel{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		dbQuery = dbQuery.Where("name LIKE ? OR display_name LIKE ? OR biz_code LIKE ?", like, like, like)
	}
	if env := strings.TrimSpace(query.Env); env != "" {
		dbQuery = dbQuery.Where("env = ?", env)
	}
	if status := strings.TrimSpace(query.Status); status != "" {
		dbQuery = dbQuery.Where("status = ?", status)
	}

	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := query.Page
	pageSize := query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	err := dbQuery.Offset((page - 1) * pageSize).
		Limit(pageSize).
		Order("updated_at DESC").
		Find(&businesses).Error
	return businesses, total, err
}

func (d *CmdbDao) GetBusinessModelByID(id uint) (*model.CmdbBusinessModel, error) {
	var business model.CmdbBusinessModel
	if err := d.db.First(&business, id).Error; err != nil {
		return nil, err
	}
	return &business, nil
}

func (d *CmdbDao) UpdateBusinessModel(business *model.CmdbBusinessModel) error {
	return d.db.Save(business).Error
}

func (d *CmdbDao) DeleteBusinessModel(id uint) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("business_id = ?", id).Delete(&model.CmdbBusinessModelRelation{}).Error; err != nil {
			return err
		}
		if err := tx.Where("business_id = ?", id).Delete(&model.CmdbBusinessModelHealthSnapshot{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("business_id = ?", id).Delete(&model.CmdbBusinessModelBinding{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&model.CmdbBusinessModel{}, id).Error
	})
}

func (d *CmdbDao) GetBusinessRootBinding(businessID uint) (*model.CmdbBusinessModelBinding, error) {
	var binding model.CmdbBusinessModelBinding
	err := d.db.Where("business_id = ? AND resource_type = ?", businessID, model.CmdbResourceTypeBusinessSystem).
		Order("id ASC").
		First(&binding).Error
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

func (d *CmdbDao) GetBusinessBindingByID(businessID, id uint) (*model.CmdbBusinessModelBinding, error) {
	var binding model.CmdbBusinessModelBinding
	err := d.db.Where("business_id = ? AND id = ?", businessID, id).First(&binding).Error
	if err != nil {
		return nil, err
	}
	return &binding, nil
}

func (d *CmdbDao) GetBusinessBinding(id uint) (*model.CmdbBusinessModelBinding, error) {
	var binding model.CmdbBusinessModelBinding
	if err := d.db.First(&binding, id).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

func (d *CmdbDao) FindBusinessBindingBySource(businessID uint, sourceType string, sourceID uint, sourceKey string) (*model.CmdbBusinessModelBinding, error) {
	var binding model.CmdbBusinessModelBinding
	query := d.db.Model(&model.CmdbBusinessModelBinding{}).
		Where("business_id = ?", businessID)

	sourceType = strings.TrimSpace(sourceType)
	sourceKey = strings.TrimSpace(sourceKey)
	if sourceType == "" {
		return nil, gorm.ErrRecordNotFound
	}
	query = query.Where("source_type = ?", sourceType)

	if sourceID > 0 {
		query = query.Where("source_id = ?", sourceID)
	} else if sourceKey != "" {
		query = query.Where("source_key = ?", sourceKey)
	} else {
		return nil, gorm.ErrRecordNotFound
	}

	if err := query.First(&binding).Error; err != nil {
		return nil, err
	}
	return &binding, nil
}

func (d *CmdbDao) CreateBusinessBinding(binding *model.CmdbBusinessModelBinding) error {
	return d.db.Create(binding).Error
}

func (d *CmdbDao) UpdateBusinessBinding(binding *model.CmdbBusinessModelBinding) error {
	return d.db.Save(binding).Error
}

func (d *CmdbDao) CheckBusinessRelationExists(businessID, srcID, dstID uint, relationType string) (bool, error) {
	var count int64
	err := d.db.Model(&model.CmdbBusinessModelRelation{}).
		Where("business_id = ? AND src_id = ? AND dst_id = ? AND relation_type = ? AND status = 1", businessID, srcID, dstID, relationType).
		Count(&count).Error
	return count > 0, err
}

func (d *CmdbDao) CreateBusinessRelationIfNotExists(relation *model.CmdbBusinessModelRelation) error {
	if relation == nil {
		return nil
	}
	exists, err := d.CheckBusinessRelationExists(relation.BusinessID, relation.SrcID, relation.DstID, relation.RelationType)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return d.db.Create(relation).Error
}

func (d *CmdbDao) GetBusinessRelationByID(id uint) (*model.CmdbBusinessModelRelation, error) {
	var relation model.CmdbBusinessModelRelation
	if err := d.db.Where("id = ? AND status = 1", id).First(&relation).Error; err != nil {
		return nil, err
	}
	return &relation, nil
}

func (d *CmdbDao) DeleteBusinessRelation(id uint) error {
	return d.db.Unscoped().Delete(&model.CmdbBusinessModelRelation{}, id).Error
}

func (d *CmdbDao) GetBusinessUpstreamRelationsData(businessID, resourceID uint) ([]model.CmdbBusinessModelRelation, error) {
	var relations []model.CmdbBusinessModelRelation
	err := d.db.Where("business_id = ? AND dst_id = ? AND status = 1", businessID, resourceID).
		Preload("Src").
		Find(&relations).Error
	return relations, err
}

func (d *CmdbDao) GetBusinessDownstreamRelationsData(businessID, resourceID uint) ([]model.CmdbBusinessModelRelation, error) {
	var relations []model.CmdbBusinessModelRelation
	err := d.db.Where("business_id = ? AND src_id = ? AND status = 1", businessID, resourceID).
		Preload("Dst").
		Find(&relations).Error
	return relations, err
}

func (d *CmdbDao) UpsertBusinessHealthSnapshot(snapshot *model.CmdbBusinessModelHealthSnapshot) error {
	if snapshot == nil {
		return nil
	}
	return d.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "business_id"},
			{Name: "resource_id"},
			{Name: "dimension"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"score",
			"status",
			"source",
			"message",
			"evidence",
			"updated_at",
		}),
	}).Create(snapshot).Error
}

func (d *CmdbDao) GetBusinessHealthSnapshotsByResourceIDs(businessID uint, resourceIDs []uint) ([]model.CmdbBusinessModelHealthSnapshot, error) {
	var snapshots []model.CmdbBusinessModelHealthSnapshot
	if len(resourceIDs) == 0 {
		return snapshots, nil
	}
	query := d.db.Where("resource_id IN ?", resourceIDs)
	if businessID > 0 {
		query = query.Where("business_id = ?", businessID)
	}
	err := query.Find(&snapshots).Error
	return snapshots, err
}

func JSONMapToDatatype(value map[string]interface{}) (datatypes.JSON, error) {
	if len(value) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(payload), nil
}
