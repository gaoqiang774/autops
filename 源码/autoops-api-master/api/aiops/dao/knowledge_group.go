package dao

import (
	"strings"

	"dodevops-api/api/aiops/model"
	"dodevops-api/common"

	"gorm.io/gorm"
)

type KnowledgeGroupDao struct {
	db *gorm.DB
}

func (d *KnowledgeGroupDao) EnsurePath(groupName, category string, userID uint) (uint, error) {
	groupName = strings.TrimSpace(groupName)
	category = strings.TrimSpace(category)
	if groupName == "" {
		return 0, nil
	}
	var leafID uint
	err := d.db.Transaction(func(tx *gorm.DB) error {
		root := model.KnowledgeGroup{ParentID: 0, Name: groupName, CreatedBy: userID, UpdatedBy: userID}
		if err := tx.Where("parent_id = ? AND name = ?", 0, groupName).FirstOrCreate(&root).Error; err != nil {
			return err
		}
		leafID = root.ID
		if category == "" || category == groupName {
			return nil
		}
		child := model.KnowledgeGroup{ParentID: root.ID, Name: category, CreatedBy: userID, UpdatedBy: userID}
		if err := tx.Where("parent_id = ? AND name = ?", root.ID, category).FirstOrCreate(&child).Error; err != nil {
			return err
		}
		leafID = child.ID
		return nil
	})
	return leafID, err
}

func NewKnowledgeGroupDao() *KnowledgeGroupDao {
	return &KnowledgeGroupDao{db: common.GetDB()}
}

func (d *KnowledgeGroupDao) List() ([]model.KnowledgeGroup, map[uint]int64, error) {
	var groups []model.KnowledgeGroup
	if err := d.db.Order("sort_order ASC, name ASC, id ASC").Find(&groups).Error; err != nil {
		return nil, nil, err
	}
	type groupCount struct {
		GroupID uint  `gorm:"column:group_id"`
		Count   int64 `gorm:"column:count"`
	}
	var rows []groupCount
	if err := d.db.Model(&model.KnowledgeDocument{}).
		Select("group_id, COUNT(*) AS count").
		Where("group_id > 0").
		Group("group_id").
		Scan(&rows).Error; err != nil {
		return nil, nil, err
	}
	counts := make(map[uint]int64, len(rows))
	for _, row := range rows {
		counts[row.GroupID] = row.Count
	}
	return groups, counts, nil
}

func (d *KnowledgeGroupDao) Get(id uint) (*model.KnowledgeGroup, error) {
	var group model.KnowledgeGroup
	if err := d.db.First(&group, id).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func (d *KnowledgeGroupDao) Create(group *model.KnowledgeGroup) error {
	return d.db.Create(group).Error
}

func (d *KnowledgeGroupDao) Update(id uint, updates map[string]interface{}) error {
	result := d.db.Model(&model.KnowledgeGroup{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (d *KnowledgeGroupDao) Delete(id uint) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		var groups []model.KnowledgeGroup
		if err := tx.Select("id", "parent_id").Find(&groups).Error; err != nil {
			return err
		}
		groupIDs, ok := knowledgeGroupDeleteOrder(groups, id)
		if !ok {
			return gorm.ErrRecordNotFound
		}

		documentIDs := tx.Model(&model.KnowledgeDocument{}).
			Select("id").
			Where("group_id IN ?", groupIDs)
		if err := tx.Where("document_id IN (?)", documentIDs).Delete(&model.KnowledgeChunk{}).Error; err != nil {
			return err
		}
		if err := tx.Where("group_id IN ?", groupIDs).Delete(&model.KnowledgeDocument{}).Error; err != nil {
			return err
		}

		// Children are ordered before their parents so this also works when a
		// self-referencing foreign key exists on the group table.
		for _, groupID := range groupIDs {
			result := tx.Delete(&model.KnowledgeGroup{}, groupID)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return gorm.ErrRecordNotFound
			}
		}
		return nil
	})
}

func knowledgeGroupDeleteOrder(groups []model.KnowledgeGroup, rootID uint) ([]uint, bool) {
	children := make(map[uint][]uint, len(groups))
	exists := false
	for _, group := range groups {
		children[group.ParentID] = append(children[group.ParentID], group.ID)
		if group.ID == rootID {
			exists = true
		}
	}
	if !exists {
		return nil, false
	}

	order := make([]uint, 0)
	visited := make(map[uint]struct{}, len(groups))
	var visit func(uint)
	visit = func(groupID uint) {
		if _, ok := visited[groupID]; ok {
			return
		}
		visited[groupID] = struct{}{}
		for _, childID := range children[groupID] {
			visit(childID)
		}
		order = append(order, groupID)
	}
	visit(rootID)
	return order, true
}
