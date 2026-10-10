package dao

import (
	"dodevops-api/api/cmdb/model"
	"dodevops-api/common"
	"strings"
	"time"

	"gorm.io/gorm"
)

type BastionAssetAuthorizationDao struct {
	db *gorm.DB
}

func NewBastionAssetAuthorizationDao() BastionAssetAuthorizationDao {
	return BastionAssetAuthorizationDao{db: common.GetDB()}
}

func normalizeAssetScope(scopeType, scopeValue string) (string, string) {
	scopeType = strings.TrimSpace(scopeType)
	scopeValue = strings.TrimSpace(scopeValue)
	if scopeValue == "" {
		return model.AssetScopeAll, ""
	}
	return scopeType, scopeValue
}

func (d *BastionAssetAuthorizationDao) activeScope(query *gorm.DB) *gorm.DB {
	now := time.Now()
	return query.Where("(effective_time IS NULL OR effective_time <= ?) AND (expire_time IS NULL OR expire_time > ?)", now, now)
}

func (d *BastionAssetAuthorizationDao) nonExpiredScope(query *gorm.DB) *gorm.DB {
	now := time.Now()
	return query.Where("expire_time IS NULL OR expire_time > ?", now)
}

func (d *BastionAssetAuthorizationDao) getUserGroupIDs(userID uint) ([]uint, error) {
	groupDao := NewBastionUserGroupDao()
	groups, err := groupDao.GetUserGroups(userID)
	if err != nil {
		return nil, err
	}

	groupIDs := make([]uint, 0, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
	}
	return groupIDs, nil
}

func (d *BastionAssetAuthorizationDao) activeUserAuthorizationQuery(userID uint) (*gorm.DB, error) {
	groupIDs, err := d.getUserGroupIDs(userID)
	if err != nil {
		return nil, err
	}

	query := d.activeScope(d.db.Model(&model.BastionAssetAuthorization{}))
	if len(groupIDs) > 0 {
		query = query.Where(
			"(subject_type = ? AND subject_id = ?) OR (subject_type = ? AND subject_id IN ?)",
			model.AssetAuthorizationSubjectUser, userID, model.AssetAuthorizationSubjectGroup, groupIDs,
		)
	} else {
		query = query.Where("subject_type = ? AND subject_id = ?", model.AssetAuthorizationSubjectUser, userID)
	}

	return query, nil
}

func (d *BastionAssetAuthorizationDao) GetAuthorizationsBySubjectAssetScopes(subjectType string, subjectID uint, assetType string, assetID uint, scopes [][2]string) ([]model.BastionAssetAuthorization, error) {
	var auths []model.BastionAssetAuthorization
	if len(scopes) == 0 {
		return auths, nil
	}

	query := d.db.Where("subject_type = ? AND subject_id = ? AND asset_type = ? AND asset_id = ?", subjectType, subjectID, assetType, assetID)
	scopeQuery := d.db.Session(&gorm.Session{NewDB: true})
	for i, scope := range scopes {
		scopeType, scopeValue := normalizeAssetScope(scope[0], scope[1])
		cond := d.db.Where("scope_type = ? AND scope_value = ?", scopeType, scopeValue)
		if i == 0 {
			scopeQuery = cond
		} else {
			scopeQuery = scopeQuery.Or(cond)
		}
	}

	err := query.Where(scopeQuery).Find(&auths).Error
	return auths, err
}

func (d *BastionAssetAuthorizationDao) BatchAuthorizeAssets(auths []model.BastionAssetAuthorization) error {
	if len(auths) == 0 {
		return nil
	}
	return d.db.Create(&auths).Error
}

func (d *BastionAssetAuthorizationDao) SaveAuthorization(auth *model.BastionAssetAuthorization) error {
	return d.db.Save(auth).Error
}

func (d *BastionAssetAuthorizationDao) GetAuthorizedAssetsWithPage(subjectType string, subjectID uint, assetType string, assetID uint, page, pageSize int, includeExpired bool) ([]model.BastionAssetAuthorization, int64, error) {
	var auths []model.BastionAssetAuthorization
	var total int64

	query := d.db.Model(&model.BastionAssetAuthorization{}).Where("subject_type = ? AND subject_id = ?", subjectType, subjectID)
	if strings.TrimSpace(assetType) != "" {
		query = query.Where("asset_type = ?", strings.TrimSpace(assetType))
	}
	if assetID > 0 {
		query = query.Where("asset_id = ?", assetID)
	}
	if !includeExpired {
		query = d.nonExpiredScope(query)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("create_time DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&auths).Error
	return auths, total, err
}

func (d *BastionAssetAuthorizationDao) BatchRevokeAssetAuthorization(subjectType string, subjectID uint, assetType string, assetID uint, scopes [][2]string) error {
	query := d.db.Where("subject_type = ? AND subject_id = ? AND asset_type = ? AND asset_id = ?", subjectType, subjectID, assetType, assetID)
	if len(scopes) > 0 {
		scopeQuery := d.db.Session(&gorm.Session{NewDB: true})
		for i, scope := range scopes {
			scopeType, scopeValue := normalizeAssetScope(scope[0], scope[1])
			cond := d.db.Where("scope_type = ? AND scope_value = ?", scopeType, scopeValue)
			if i == 0 {
				scopeQuery = cond
			} else {
				scopeQuery = scopeQuery.Or(cond)
			}
		}
		query = query.Where(scopeQuery)
	}
	return query.Delete(&model.BastionAssetAuthorization{}).Error
}

func (d *BastionAssetAuthorizationDao) GetUserAuthorizedAssetIDs(userID uint, assetType string) ([]uint, error) {
	query, err := d.activeUserAuthorizationQuery(userID)
	if err != nil {
		return nil, err
	}

	var assetIDs []uint
	err = query.Where("asset_type = ?", assetType).Distinct("asset_id").Pluck("asset_id", &assetIDs).Error
	return assetIDs, err
}

func (d *BastionAssetAuthorizationDao) GetUserAuthorizedScopeValues(userID uint, assetType string, assetID uint, scopeType string) ([]string, error) {
	query, err := d.activeUserAuthorizationQuery(userID)
	if err != nil {
		return nil, err
	}

	var values []string
	err = query.Where("asset_type = ? AND asset_id = ? AND scope_type = ? AND scope_value <> ?", assetType, assetID, strings.TrimSpace(scopeType), "").
		Distinct("scope_value").
		Pluck("scope_value", &values).Error
	return values, err
}

func (d *BastionAssetAuthorizationDao) GetUserAuthorizedScopeValuesByAssetIDs(userID uint, assetType string, assetIDs []uint, scopeType string) (map[uint][]string, error) {
	result := make(map[uint][]string)
	if len(assetIDs) == 0 {
		return result, nil
	}

	query, err := d.activeUserAuthorizationQuery(userID)
	if err != nil {
		return nil, err
	}

	type row struct {
		AssetID    uint
		ScopeValue string
	}
	var rows []row
	err = query.Select("asset_id, scope_value").
		Where("asset_type = ? AND asset_id IN ? AND scope_type = ? AND scope_value <> ?", assetType, assetIDs, strings.TrimSpace(scopeType), "").
		Group("asset_id, scope_value").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, item := range rows {
		result[item.AssetID] = append(result[item.AssetID], item.ScopeValue)
	}
	return result, nil
}

func (d *BastionAssetAuthorizationDao) CheckUserAssetAccess(userID uint, assetType string, assetID uint) (bool, error) {
	query, err := d.activeUserAuthorizationQuery(userID)
	if err != nil {
		return false, err
	}

	var count int64
	err = query.Where("asset_type = ? AND asset_id = ?", assetType, assetID).Count(&count).Error
	return count > 0, err
}

func (d *BastionAssetAuthorizationDao) CheckUserAssetPermission(userID uint, assetType string, assetID uint, scopeType string, scopeValue string) (bool, error) {
	scopeType, scopeValue = normalizeAssetScope(scopeType, scopeValue)
	query, err := d.activeUserAuthorizationQuery(userID)
	if err != nil {
		return false, err
	}

	var count int64
	query = query.Where("asset_type = ? AND asset_id = ?", assetType, assetID)
	if scopeValue == "" {
		err = query.Where("scope_type = ? AND scope_value = ?", model.AssetScopeAll, "").Count(&count).Error
		return count > 0, err
	}

	err = query.Where(
		"(scope_type = ? AND scope_value = ?) OR (scope_type = ? AND scope_value = ?)",
		model.AssetScopeAll, "", scopeType, scopeValue,
	).Count(&count).Error
	return count > 0, err
}

func (d *BastionAssetAuthorizationDao) CountAuthorizedAssets(subjectType string, subjectID uint, assetType string) (int64, error) {
	var count int64
	err := d.activeScope(d.db.Model(&model.BastionAssetAuthorization{})).
		Where("subject_type = ? AND subject_id = ? AND asset_type = ?", subjectType, subjectID, strings.TrimSpace(assetType)).
		Distinct("asset_id").
		Count(&count).Error
	return count, err
}
