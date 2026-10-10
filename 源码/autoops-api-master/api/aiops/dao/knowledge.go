package dao

import (
	"strings"

	"dodevops-api/api/aiops/model"
	"dodevops-api/common"

	"gorm.io/gorm"
)

type KnowledgeDao struct {
	db *gorm.DB
}

func NewKnowledgeDao() *KnowledgeDao {
	return &KnowledgeDao{db: common.GetDB()}
}

func (d *KnowledgeDao) CreateDocument(doc *model.KnowledgeDocument, chunks []model.KnowledgeChunk) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(doc).Error; err != nil {
			return err
		}
		for i := range chunks {
			chunks[i].DocumentID = doc.ID
		}
		if len(chunks) > 0 {
			if err := tx.CreateInBatches(chunks, 100).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *KnowledgeDao) UpdateDocument(id uint, updates map[string]interface{}, chunks []model.KnowledgeChunk, rebuildChunks bool) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		if len(updates) > 0 {
			if err := tx.Model(&model.KnowledgeDocument{}).Where("id = ?", id).Updates(updates).Error; err != nil {
				return err
			}
		}
		if rebuildChunks {
			if err := tx.Where("document_id = ?", id).Delete(&model.KnowledgeChunk{}).Error; err != nil {
				return err
			}
			for i := range chunks {
				chunks[i].DocumentID = id
			}
			if len(chunks) > 0 {
				if err := tx.CreateInBatches(chunks, 100).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (d *KnowledgeDao) RebuildDocumentChunks(id uint, chunks []model.KnowledgeChunk, updates map[string]interface{}) error {
	return d.UpdateDocument(id, updates, chunks, true)
}

func (d *KnowledgeDao) DeleteDocument(id uint) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("document_id = ?", id).Delete(&model.KnowledgeChunk{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.KnowledgeDocument{}, id).Error
	})
}

func (d *KnowledgeDao) GetDocument(id uint) (*model.KnowledgeDocument, error) {
	var doc model.KnowledgeDocument
	if err := d.db.First(&doc, id).Error; err != nil {
		return nil, err
	}
	return &doc, nil
}

func (d *KnowledgeDao) ListDocuments(query model.KnowledgeDocumentQuery) ([]model.KnowledgeDocument, int64, error) {
	var docs []model.KnowledgeDocument
	var total int64
	db := applyKnowledgeDocumentFilters(d.db.Model(&model.KnowledgeDocument{}), query)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 10
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	err := db.Order("updated_at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&docs).Error
	return docs, total, err
}

func (d *KnowledgeDao) SearchDocuments(req model.KnowledgeSearchRequest) ([]model.KnowledgeDocument, error) {
	var docs []model.KnowledgeDocument
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	db := d.db.Model(&model.KnowledgeDocument{})
	db = db.Where("search_status = ?", model.KnowledgeSearchReady)
	if status := strings.TrimSpace(req.Status); status != "" {
		db = db.Where("status = ?", status)
	} else {
		db = db.Where("status = ?", model.KnowledgeStatusPublished)
	}
	if docType := strings.TrimSpace(req.DocType); docType != "" {
		db = db.Where("doc_type = ?", docType)
	}
	if len(req.GroupIDs) > 0 {
		db = db.Where("group_id IN ?", req.GroupIDs)
	} else if req.GroupID > 0 {
		db = db.Where("group_id = ?", req.GroupID)
	}
	if groupName := strings.TrimSpace(req.GroupName); groupName != "" {
		db = db.Where("group_name LIKE ?", "%"+groupName+"%")
	}
	if category := strings.TrimSpace(req.Category); category != "" {
		db = db.Where("category LIKE ?", "%"+category+"%")
	}
	if serviceName := strings.TrimSpace(req.ServiceName); serviceName != "" {
		db = db.Where("service_name LIKE ?", "%"+serviceName+"%")
	}
	keyword := strings.TrimSpace(req.Query)
	if keyword != "" {
		terms := req.QueryTerms
		if len(terms) == 0 {
			terms = []string{keyword}
		}
		clauses := make([]string, 0, len(terms))
		args := make([]interface{}, 0, len(terms)*10)
		for _, term := range terms {
			term = strings.TrimSpace(term)
			if term == "" {
				continue
			}
			clauses = append(clauses, `(title LIKE ? OR summary LIKE ? OR content_markdown LIKE ? OR tags_json LIKE ? OR keywords_json LIKE ? OR alert_names_json LIKE ? OR log_keywords_json LIKE ? OR service_name LIKE ? OR group_name LIKE ? OR category LIKE ?)`)
			like := "%" + term + "%"
			for i := 0; i < 10; i++ {
				args = append(args, like)
			}
		}
		if len(clauses) > 0 {
			db = db.Where(strings.Join(clauses, " OR "), args...)
		}
	}
	for _, tag := range req.Tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			db = db.Where("tags_json LIKE ?", "%"+tag+"%")
		}
	}
	candidateLimit := limit * 10
	if candidateLimit < 50 {
		candidateLimit = 50
	}
	if candidateLimit > 200 {
		candidateLimit = 200
	}
	err := db.Order("updated_at DESC, id DESC").Limit(candidateLimit).Find(&docs).Error
	return docs, err
}

func applyKnowledgeDocumentFilters(db *gorm.DB, query model.KnowledgeDocumentQuery) *gorm.DB {
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("title LIKE ? OR summary LIKE ? OR content_markdown LIKE ? OR tags_json LIKE ? OR keywords_json LIKE ?", like, like, like, like, like)
	}
	if docType := strings.TrimSpace(query.DocType); docType != "" {
		db = db.Where("doc_type = ?", docType)
	}
	if status := strings.TrimSpace(query.Status); status != "" {
		db = db.Where("status = ?", status)
	}
	if len(query.GroupIDs) > 0 {
		db = db.Where("group_id IN ?", query.GroupIDs)
	} else if query.GroupID > 0 {
		db = db.Where("group_id = ?", query.GroupID)
	}
	if groupName := strings.TrimSpace(query.GroupName); groupName != "" {
		db = db.Where("group_name LIKE ?", "%"+groupName+"%")
	}
	if category := strings.TrimSpace(query.Category); category != "" {
		db = db.Where("category LIKE ?", "%"+category+"%")
	}
	if serviceName := strings.TrimSpace(query.ServiceName); serviceName != "" {
		db = db.Where("service_name LIKE ?", "%"+serviceName+"%")
	}
	if env := strings.TrimSpace(query.Env); env != "" {
		db = db.Where("env = ?", env)
	}
	return db
}
