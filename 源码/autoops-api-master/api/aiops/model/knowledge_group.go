package model

import "dodevops-api/common/util"

type KnowledgeGroup struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	ParentID    uint       `gorm:"not null;default:0;uniqueIndex:idx_aiops_knowledge_group_parent_name,priority:1;index" json:"parentId"`
	Name        string     `gorm:"size:128;not null;uniqueIndex:idx_aiops_knowledge_group_parent_name,priority:2" json:"name"`
	Description string     `gorm:"size:500" json:"description"`
	SortOrder   int        `gorm:"not null;default:0;index" json:"sortOrder"`
	CreatedBy   uint       `gorm:"index" json:"createdBy,omitempty"`
	UpdatedBy   uint       `gorm:"index" json:"updatedBy,omitempty"`
	CreatedAt   util.HTime `json:"createdAt"`
	UpdatedAt   util.HTime `json:"updatedAt"`
}

func (KnowledgeGroup) TableName() string {
	return "aiops_knowledge_group"
}

type KnowledgeGroupDTO struct {
	ParentID    uint   `json:"parentId"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	SortOrder   int    `json:"sortOrder"`
}

type KnowledgeGroupTreeVO struct {
	ID          uint                   `json:"id"`
	ParentID    uint                   `json:"parentId"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	SortOrder   int                    `json:"sortOrder"`
	Path        string                 `json:"path"`
	DirectCount int64                  `json:"directCount"`
	DocCount    int64                  `json:"docCount"`
	Children    []KnowledgeGroupTreeVO `json:"children"`
}
