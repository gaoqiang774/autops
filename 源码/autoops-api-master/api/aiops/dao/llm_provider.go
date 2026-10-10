package dao

import (
	"strings"
	"time"

	"dodevops-api/api/aiops/model"
	"dodevops-api/common"
	"dodevops-api/common/util"

	"gorm.io/gorm"
)

type LLMProviderDao struct{}

func NewLLMProviderDao() *LLMProviderDao {
	return &LLMProviderDao{}
}

func (d *LLMProviderDao) Create(provider *model.LLMProvider) error {
	return common.GetDB().Transaction(func(tx *gorm.DB) error {
		if provider.IsDefault {
			if err := unsetDefaultProviders(tx, provider.ID).Error; err != nil {
				return err
			}
		}
		return tx.Create(provider).Error
	})
}

func (d *LLMProviderDao) Update(id uint, updates map[string]interface{}, setDefault bool) error {
	updates["updated_at"] = util.HTime{Time: time.Now()}

	return common.GetDB().Transaction(func(tx *gorm.DB) error {
		if setDefault {
			if err := unsetDefaultProviders(tx, id).Error; err != nil {
				return err
			}
		}
		return tx.Model(&model.LLMProvider{}).Where("id = ?", id).Updates(updates).Error
	})
}

func (d *LLMProviderDao) Delete(id uint) error {
	return common.GetDB().Delete(&model.LLMProvider{}, id).Error
}

func (d *LLMProviderDao) GetByID(id uint) (*model.LLMProvider, error) {
	var provider model.LLMProvider
	err := common.GetDB().First(&provider, id).Error
	return &provider, err
}

func (d *LLMProviderDao) List(query model.ListLLMProviderQuery) ([]model.LLMProvider, int64, error) {
	var providers []model.LLMProvider
	var total int64

	db := common.GetDB().Model(&model.LLMProvider{})
	db = applyListFilters(db, query)

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (query.Page - 1) * query.PageSize
	err := db.Order("is_default DESC, updated_at DESC, id DESC").
		Offset(offset).
		Limit(query.PageSize).
		Find(&providers).Error
	return providers, total, err
}

func (d *LLMProviderDao) GetEnabled() ([]model.LLMProvider, error) {
	var providers []model.LLMProvider
	err := common.GetDB().
		Where("status = ?", model.LLMProviderStatusEnabled).
		Order("is_default DESC, updated_at DESC, id DESC").
		Find(&providers).Error
	return providers, err
}

func (d *LLMProviderDao) SetDefault(id uint) error {
	return common.GetDB().Transaction(func(tx *gorm.DB) error {
		var provider model.LLMProvider
		if err := tx.First(&provider, id).Error; err != nil {
			return err
		}
		if err := unsetDefaultProviders(tx, id).Error; err != nil {
			return err
		}
		return tx.Model(&model.LLMProvider{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"is_default": true,
				"status":     model.LLMProviderStatusEnabled,
				"updated_at": util.HTime{Time: time.Now()},
			}).Error
	})
}

func applyListFilters(db *gorm.DB, query model.ListLLMProviderQuery) *gorm.DB {
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR default_model LIKE ? OR remark LIKE ?", like, like, like)
	}
	if providerType := model.NormalizeProviderType(query.ProviderType); providerType != "" {
		db = db.Where("provider_type = ?", providerType)
	}
	if protocol := strings.TrimSpace(query.Protocol); protocol != "" {
		db = db.Where("protocol = ?", protocol)
	}
	if query.OnlyEnabled {
		db = db.Where("status = ?", model.LLMProviderStatusEnabled)
	} else if query.Status > 0 {
		db = db.Where("status = ?", query.Status)
	}
	return db
}

func unsetDefaultProviders(tx *gorm.DB, exceptID uint) *gorm.DB {
	db := tx.Model(&model.LLMProvider{}).Where("is_default = ?", true)
	if exceptID > 0 {
		db = db.Where("id <> ?", exceptID)
	}
	return db.Updates(map[string]interface{}{
		"is_default": false,
		"updated_at": util.HTime{Time: time.Now()},
	})
}
