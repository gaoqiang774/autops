package dao

import (
	"dodevops-api/api/cmdb/model"
	"dodevops-api/common"
	"errors"
	"strings"

	"gorm.io/gorm"
)

type CmdbIdcRoomDao struct {
	db *gorm.DB
}

func NewCmdbIdcRoomDao() CmdbIdcRoomDao {
	return CmdbIdcRoomDao{
		db: common.GetDB(),
	}
}

func (d *CmdbIdcRoomDao) CheckRoomNameExists(name string, excludeID uint) bool {
	var count int64
	db := d.db.Model(&model.CmdbIdcRoom{}).Where("name = ?", strings.TrimSpace(name))
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	db.Count(&count)
	return count > 0
}

func (d *CmdbIdcRoomDao) CheckCabinetNameExists(roomID uint, name string, excludeID uint) bool {
	var count int64
	db := d.db.Model(&model.CmdbIdcCabinet{}).Where("room_id = ? AND name = ?", roomID, strings.TrimSpace(name))
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	db.Count(&count)
	return count > 0
}

func (d *CmdbIdcRoomDao) CreateRoom(room *model.CmdbIdcRoom) error {
	return d.db.Create(room).Error
}

func (d *CmdbIdcRoomDao) CreateRoomWithCabinets(room *model.CmdbIdcRoom, cabinetNames []string) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(room).Error; err != nil {
			return err
		}

		for _, name := range cabinetNames {
			cabinetName := strings.TrimSpace(name)
			if cabinetName == "" {
				continue
			}
			cabinet := model.CmdbIdcCabinet{
				RoomID:     room.ID,
				Name:       cabinetName,
				TotalU:     42,
				CreateTime: room.CreateTime,
				UpdateTime: room.UpdateTime,
			}
			if err := tx.Create(&cabinet).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *CmdbIdcRoomDao) UpdateRoom(id uint, room *model.CmdbIdcRoom) error {
	return d.db.Model(&model.CmdbIdcRoom{}).Where("id = ?", id).Updates(room).Error
}

func (d *CmdbIdcRoomDao) DeleteRoom(id uint) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("room_id = ?", id).Delete(&model.CmdbIdcCabinet{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.CmdbIdcRoom{}, id).Error
	})
}

func (d *CmdbIdcRoomDao) GetRoomById(id uint) (model.CmdbIdcRoom, error) {
	var room model.CmdbIdcRoom
	err := d.db.Preload("Cabinets", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("name asc")
	}).Where("id = ?", id).First(&room).Error
	return room, err
}

func (d *CmdbIdcRoomDao) GetRoomByName(name string) (*model.CmdbIdcRoom, error) {
	var room model.CmdbIdcRoom
	err := d.db.Where("name = ?", strings.TrimSpace(name)).First(&room).Error
	if err != nil {
		return nil, err
	}
	return &room, nil
}

func (d *CmdbIdcRoomDao) ListRooms() ([]model.CmdbIdcRoom, error) {
	var rooms []model.CmdbIdcRoom
	err := d.db.Preload("Cabinets", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("name asc")
	}).Order("name asc").Find(&rooms).Error
	return rooms, err
}

func (d *CmdbIdcRoomDao) CreateCabinet(cabinet *model.CmdbIdcCabinet) error {
	return d.db.Create(cabinet).Error
}

func (d *CmdbIdcRoomDao) UpdateCabinet(id uint, cabinet *model.CmdbIdcCabinet) error {
	return d.db.Model(&model.CmdbIdcCabinet{}).Where("id = ?", id).Updates(cabinet).Error
}

func (d *CmdbIdcRoomDao) DeleteCabinet(id uint) error {
	return d.db.Delete(&model.CmdbIdcCabinet{}, id).Error
}

func (d *CmdbIdcRoomDao) GetCabinetById(id uint) (model.CmdbIdcCabinet, error) {
	var cabinet model.CmdbIdcCabinet
	err := d.db.Preload("Room").Where("id = ?", id).First(&cabinet).Error
	return cabinet, err
}

func (d *CmdbIdcRoomDao) GetCabinetByRoomAndName(roomID uint, name string) (*model.CmdbIdcCabinet, error) {
	var cabinet model.CmdbIdcCabinet
	err := d.db.Where("room_id = ? AND name = ?", roomID, strings.TrimSpace(name)).First(&cabinet).Error
	if err != nil {
		return nil, err
	}
	return &cabinet, nil
}

func (d *CmdbIdcRoomDao) FindCabinetsByName(name string) ([]model.CmdbIdcCabinet, error) {
	var cabinets []model.CmdbIdcCabinet
	err := d.db.Preload("Room").
		Where("name = ?", strings.TrimSpace(name)).
		Order("id asc").
		Find(&cabinets).Error
	return cabinets, err
}

func (d *CmdbIdcRoomDao) ListCabinets(roomID uint) ([]model.CmdbIdcCabinet, error) {
	var cabinets []model.CmdbIdcCabinet
	db := d.db.Order("name asc")
	if roomID > 0 {
		db = db.Where("room_id = ?", roomID)
	}
	err := db.Find(&cabinets).Error
	return cabinets, err
}

func (d *CmdbIdcRoomDao) EnsureRoom(name string, createFn func(room *model.CmdbIdcRoom) error) (*model.CmdbIdcRoom, error) {
	room, err := d.GetRoomByName(name)
	if err == nil {
		return room, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	newRoom := &model.CmdbIdcRoom{Name: strings.TrimSpace(name)}
	if err := createFn(newRoom); err != nil {
		return nil, err
	}
	return newRoom, nil
}

func (d *CmdbIdcRoomDao) EnsureCabinet(roomID uint, name string, createFn func(cabinet *model.CmdbIdcCabinet) error) (*model.CmdbIdcCabinet, error) {
	cabinet, err := d.GetCabinetByRoomAndName(roomID, name)
	if err == nil {
		return cabinet, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	newCabinet := &model.CmdbIdcCabinet{
		RoomID: roomID,
		Name:   strings.TrimSpace(name),
	}
	if err := createFn(newCabinet); err != nil {
		return nil, err
	}
	return newCabinet, nil
}
