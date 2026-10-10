package dao

import (
	"dodevops-api/api/cmdb/model"
	"dodevops-api/common"
	"time"

	"gorm.io/gorm"
)

type BastionHostAuthorizationDao struct {
	db *gorm.DB
}

func NewBastionHostAuthorizationDao() BastionHostAuthorizationDao {
	return BastionHostAuthorizationDao{
		db: common.GetDB(),
	}
}

func (d *BastionHostAuthorizationDao) activeScope(query *gorm.DB) *gorm.DB {
	now := time.Now()
	return query.Where("(effective_time IS NULL OR effective_time <= ?) AND (expire_time IS NULL OR expire_time > ?)", now, now)
}

func (d *BastionHostAuthorizationDao) nonExpiredScope(query *gorm.DB) *gorm.DB {
	now := time.Now()
	return query.Where("expire_time IS NULL OR expire_time > ?", now)
}

func (d *BastionHostAuthorizationDao) getUserGroupIDs(userID uint) ([]uint, error) {
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

func (d *BastionHostAuthorizationDao) activeUserAuthorizationQuery(userID uint) (*gorm.DB, error) {
	groupIDs, err := d.getUserGroupIDs(userID)
	if err != nil {
		return nil, err
	}

	query := d.activeScope(d.db.Model(&model.BastionHostAuthorization{}))
	if len(groupIDs) > 0 {
		query = query.Where(
			"(subject_type = ? AND subject_id = ?) OR (subject_type = ? AND subject_id IN ?)",
			"user", userID, "group", groupIDs,
		)
	} else {
		query = query.Where("subject_type = ? AND subject_id = ?", "user", userID)
	}

	return query, nil
}

// 授权主机
func (d *BastionHostAuthorizationDao) AuthorizeHost(auth *model.BastionHostAuthorization) error {
	return d.db.Create(auth).Error
}

// 批量授权主机
func (d *BastionHostAuthorizationDao) BatchAuthorizeHosts(auths []model.BastionHostAuthorization) error {
	return d.db.Create(&auths).Error
}

func (d *BastionHostAuthorizationDao) SaveAuthorization(auth *model.BastionHostAuthorization) error {
	return d.db.Save(auth).Error
}

func (d *BastionHostAuthorizationDao) GetAuthorizationsBySubjectAndHosts(subjectType string, subjectID uint, hostIDs []uint) ([]model.BastionHostAuthorization, error) {
	var auths []model.BastionHostAuthorization
	if len(hostIDs) == 0 {
		return auths, nil
	}

	err := d.db.Where("subject_type = ? AND subject_id = ? AND host_id IN ?", subjectType, subjectID, hostIDs).
		Order("id DESC").
		Find(&auths).Error
	return auths, err
}

// 取消授权
func (d *BastionHostAuthorizationDao) RevokeHostAuthorization(subjectType string, subjectID, hostID uint) error {
	return d.db.Where("subject_type = ? AND subject_id = ? AND host_id = ?", subjectType, subjectID, hostID).
		Delete(&model.BastionHostAuthorization{}).Error
}

// 批量取消授权
func (d *BastionHostAuthorizationDao) BatchRevokeHostAuthorization(subjectType string, subjectID uint, hostIDs []uint) error {
	return d.db.Where("subject_type = ? AND subject_id = ? AND host_id IN ?", subjectType, subjectID, hostIDs).
		Delete(&model.BastionHostAuthorization{}).Error
}

// 删除主体的所有授权(用户或用户组)
func (d *BastionHostAuthorizationDao) RevokeAllSubjectAuthorization(subjectType string, subjectID uint) error {
	return d.db.Where("subject_type = ? AND subject_id = ?", subjectType, subjectID).
		Delete(&model.BastionHostAuthorization{}).Error
}

// 删除主机的所有授权
func (d *BastionHostAuthorizationDao) RevokeAllHostAuthorization(hostID uint) error {
	return d.db.Where("host_id = ?", hostID).Delete(&model.BastionHostAuthorization{}).Error
}

// 检查是否当前已授权该主机
func (d *BastionHostAuthorizationDao) CheckAuthorization(subjectType string, subjectID, hostID uint) bool {
	var count int64
	d.activeScope(d.db.Model(&model.BastionHostAuthorization{})).
		Where("subject_type = ? AND subject_id = ? AND host_id = ?", subjectType, subjectID, hostID).
		Count(&count)
	return count > 0
}

// 获取当前已授权的主机ID列表
func (d *BastionHostAuthorizationDao) GetAuthorizedHostIds(subjectType string, subjectID uint) ([]uint, error) {
	var hostIDs []uint
	err := d.activeScope(d.db.Model(&model.BastionHostAuthorization{})).
		Where("subject_type = ? AND subject_id = ?", subjectType, subjectID).
		Pluck("host_id", &hostIDs).Error
	return hostIDs, err
}

// 获取已授权的主机列表(分页)
func (d *BastionHostAuthorizationDao) GetAuthorizedHostsWithPage(subjectType string, subjectID uint, page, pageSize int, includeExpired bool) ([]model.BastionHostAuthorization, int64, error) {
	var auths []model.BastionHostAuthorization
	var total int64

	query := d.db.Model(&model.BastionHostAuthorization{}).
		Where("subject_type = ? AND subject_id = ?", subjectType, subjectID)

	if !includeExpired {
		query = d.nonExpiredScope(query)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Order("create_time DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&auths).Error

	return auths, total, err
}

// 获取用户当前有权限的主机ID列表(包括直接授权和通过用户组授权, 去重)
func (d *BastionHostAuthorizationDao) GetUserAuthorizedHostIds(userID uint) ([]uint, error) {
	var hostIDs []uint

	query, err := d.activeUserAuthorizationQuery(userID)
	if err != nil {
		return nil, err
	}

	err = query.Distinct("host_id").Pluck("host_id", &hostIDs).Error
	return hostIDs, err
}

// 获取用户当前有权限的主机列表(分页+去重)
func (d *BastionHostAuthorizationDao) GetUserAuthorizedHostsWithPage(userID uint, page, pageSize int) ([]uint, int64, error) {
	query, err := d.activeUserAuthorizationQuery(userID)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	err = query.Distinct("host_id").Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	var hostIDs []uint
	err = query.Distinct("host_id").
		Offset((page-1)*pageSize).
		Limit(pageSize).
		Pluck("host_id", &hostIDs).Error

	return hostIDs, total, err
}

// 检查用户是否有主机权限
func (d *BastionHostAuthorizationDao) CheckUserHostPermission(userID, hostID uint) (bool, error) {
	var count int64

	err := d.activeScope(d.db.Model(&model.BastionHostAuthorization{})).
		Where("subject_type = ? AND subject_id = ? AND host_id = ?", "user", userID, hostID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}

	groupIDs, err := d.getUserGroupIDs(userID)
	if err != nil {
		return false, err
	}
	if len(groupIDs) == 0 {
		return false, nil
	}

	err = d.activeScope(d.db.Model(&model.BastionHostAuthorization{})).
		Where("subject_type = ? AND subject_id IN ? AND host_id = ?", "group", groupIDs, hostID).
		Count(&count).Error

	return count > 0, err
}

// 统计当前授权主机数量
func (d *BastionHostAuthorizationDao) getHostBoundAuthIDs(hostID uint) ([]uint, error) {
	var rows []model.CmdbHostAuth
	if err := d.db.Where("host_id = ?", hostID).
		Order("is_default DESC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	if len(rows) > 0 {
		ids := make([]uint, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.AuthID)
		}
		return model.NormalizeAuthorizationAuthIDs(ids), nil
	}

	var host model.CmdbHost
	if err := d.db.Select("id", "ssh_key_id").Where("id = ?", hostID).First(&host).Error; err != nil {
		return nil, err
	}
	return model.NormalizeAuthorizationAuthIDs([]uint{host.SSHKeyID}), nil
}

func (d *BastionHostAuthorizationDao) GetUserHostAuthorizationRecords(userID, hostID uint) ([]model.BastionHostAuthorization, error) {
	query, err := d.activeUserAuthorizationQuery(userID)
	if err != nil {
		return nil, err
	}

	var auths []model.BastionHostAuthorization
	err = query.Where("host_id = ?", hostID).Find(&auths).Error
	return auths, err
}

func (d *BastionHostAuthorizationDao) GetUserHostAuthorizedAuthIDs(userID, hostID uint) ([]uint, error) {
	auths, err := d.GetUserHostAuthorizationRecords(userID, hostID)
	if err != nil {
		return nil, err
	}
	if len(auths) == 0 {
		return nil, nil
	}

	boundAuthIDs, err := d.getHostBoundAuthIDs(hostID)
	if err != nil {
		return nil, err
	}

	var allowed []uint
	for _, auth := range auths {
		authIDs := model.DecodeAuthorizationAuthIDs(auth.AuthIDs)
		if len(authIDs) == 0 {
			if len(boundAuthIDs) <= 1 {
				allowed = append(allowed, boundAuthIDs...)
			}
			continue
		}
		allowed = append(allowed, authIDs...)
	}
	return model.NormalizeAuthorizationAuthIDs(allowed), nil
}

func (d *BastionHostAuthorizationDao) CheckUserHostAuthPermission(userID, hostID, authID uint) (bool, error) {
	if authID == 0 {
		return d.CheckUserHostPermission(userID, hostID)
	}

	authIDs, err := d.GetUserHostAuthorizedAuthIDs(userID, hostID)
	if err != nil {
		return false, err
	}
	for _, allowedAuthID := range authIDs {
		if allowedAuthID == authID {
			return true, nil
		}
	}
	return false, nil
}

func (d *BastionHostAuthorizationDao) CountAuthorizedHosts(subjectType string, subjectID uint) (int64, error) {
	var count int64
	err := d.activeScope(d.db.Model(&model.BastionHostAuthorization{})).
		Where("subject_type = ? AND subject_id = ?", subjectType, subjectID).
		Count(&count).Error
	return count, err
}

// 统计用户当前有权限的主机数量(去重)
func (d *BastionHostAuthorizationDao) CountUserAuthorizedHosts(userID uint) (int64, error) {
	query, err := d.activeUserAuthorizationQuery(userID)
	if err != nil {
		return 0, err
	}

	var count int64
	err = query.Distinct("host_id").Count(&count).Error
	return count, err
}
