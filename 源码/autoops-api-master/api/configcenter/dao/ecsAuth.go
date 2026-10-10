package dao

import (
	"dodevops-api/api/configcenter/model"
	"dodevops-api/common"
	"strings"

	"gorm.io/gorm"
)

const ecsAuthRecentOrder = "COALESCE(update_time, create_time) DESC, id DESC"

type EcsAuthDao struct {
	db *gorm.DB
}

func NewEcsAuthDao() EcsAuthDao {
	return EcsAuthDao{
		db: common.GetDB(),
	}
}

func (d *EcsAuthDao) GetEcsAuthList() []model.EcsAuth {
	var list []model.EcsAuth
	d.db.Order(ecsAuthRecentOrder).Find(&list)
	return list
}

func (d *EcsAuthDao) GetEcsAuthListWithPage(page, pageSize int) ([]model.EcsAuth, int64) {
	var list []model.EcsAuth
	var total int64

	offset := (page - 1) * pageSize

	d.db.Model(&model.EcsAuth{}).Count(&total)
	d.db.Order(ecsAuthRecentOrder).Offset(offset).Limit(pageSize).Find(&list)

	return list, total
}

func (d *EcsAuthDao) CheckNameExists(name string) bool {
	var count int64
	d.db.Model(&model.EcsAuth{}).Where("name = ?", name).Count(&count)
	return count > 0
}

func (d *EcsAuthDao) GetEcsAuthByName(name string) (model.EcsAuth, error) {
	var auth model.EcsAuth
	err := d.db.Where("name = ?", name).First(&auth).Error
	return auth, err
}

func (d *EcsAuthDao) SearchEcsAuthByName(name string) ([]model.EcsAuth, error) {
	var list []model.EcsAuth

	query := d.db.Model(&model.EcsAuth{})
	name = strings.TrimSpace(name)
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	err := query.Order(ecsAuthRecentOrder).Find(&list).Error
	return list, err
}

func (d *EcsAuthDao) CreateEcsAuth(auth *model.EcsAuth) error {
	return d.db.Create(auth).Error
}

func (d *EcsAuthDao) UpdateEcsAuth(id uint, auth *model.EcsAuth) error {
	return d.db.Model(&model.EcsAuth{}).Where("id = ?", id).Updates(auth).Error
}

func (d *EcsAuthDao) DeleteEcsAuth(id uint) error {
	return d.db.Delete(&model.EcsAuth{}, id).Error
}

func (d *EcsAuthDao) GetById(id uint) (model.EcsAuth, error) {
	var auth model.EcsAuth
	err := d.db.Where("id = ?", id).First(&auth).Error
	return auth, err
}

func (d *EcsAuthDao) GetEcsAuthById(id uint) (model.EcsAuth, error) {
	return d.GetById(id)
}
