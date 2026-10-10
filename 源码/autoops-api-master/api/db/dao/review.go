package dao

import (
	"strings"
	"time"

	dbmodel "dodevops-api/api/db/model"
	"dodevops-api/common"
	"dodevops-api/common/util"

	"gorm.io/gorm"
)

type ReviewDao struct{}

func NewReviewDao() *ReviewDao {
	return &ReviewDao{}
}

func (d *ReviewDao) CreateTicket(ticket *dbmodel.SQLTicket, statements []dbmodel.SQLStatement, results []dbmodel.SQLReviewResult, rollbacks []dbmodel.SQLRollbackRecord) error {
	return common.GetDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(ticket).Error; err != nil {
			return err
		}
		statementIDs := map[uint]uint{}
		for i := range statements {
			statements[i].TicketID = ticket.ID
			if err := tx.Create(&statements[i]).Error; err != nil {
				return err
			}
			statementIDs[uint(statements[i].StatementIndex)] = statements[i].ID
			for j := range results {
				if results[j].StatementID == uint(statements[i].StatementIndex) {
					results[j].StatementID = statements[i].ID
				}
			}
		}
		for i := range results {
			results[i].TicketID = ticket.ID
			if err := tx.Create(&results[i]).Error; err != nil {
				return err
			}
		}
		for i := range rollbacks {
			if strings.TrimSpace(rollbacks[i].RollbackSQL) == "" {
				continue
			}
			rollbacks[i].TicketID = ticket.ID
			if statementID, ok := statementIDs[rollbacks[i].StatementID]; ok {
				rollbacks[i].StatementID = statementID
			}
			if err := tx.Create(&rollbacks[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *ReviewDao) ListTickets(query dbmodel.SQLTicketQuery) ([]dbmodel.SQLTicket, int64, error) {
	var list []dbmodel.SQLTicket
	var total int64

	db := common.GetDB().Model(&dbmodel.SQLTicket{})
	db = applyTicketFilters(db, query)
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page, pageSize := dbmodel.NormalizePage(query.Page, query.PageSize)
	err := db.Order("id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&list).Error
	return list, total, err
}

func (d *ReviewDao) GetTicket(id uint) (*dbmodel.SQLTicket, error) {
	var ticket dbmodel.SQLTicket
	err := common.GetDB().First(&ticket, id).Error
	return &ticket, err
}

func (d *ReviewDao) GetTicketDetail(id uint) (*dbmodel.SQLTicketDetail, error) {
	var detail dbmodel.SQLTicketDetail
	if err := common.GetDB().First(&detail.Ticket, id).Error; err != nil {
		return nil, err
	}
	db := common.GetDB()
	if err := db.Where("ticket_id = ?", id).Order("statement_index ASC, id ASC").Find(&detail.Statements).Error; err != nil {
		return nil, err
	}
	if err := db.Where("ticket_id = ?", id).Order("id ASC").Find(&detail.ReviewResults).Error; err != nil {
		return nil, err
	}
	if err := db.Where("ticket_id = ?", id).Order("id ASC").Find(&detail.ApprovalRecords).Error; err != nil {
		return nil, err
	}
	if err := db.Where("ticket_id = ?", id).Order("id ASC").Find(&detail.ExecutionRecords).Error; err != nil {
		return nil, err
	}
	if err := db.Where("ticket_id = ?", id).Order("id ASC").Find(&detail.RollbackRecords).Error; err != nil {
		return nil, err
	}
	return &detail, nil
}

func (d *ReviewDao) UpdateTicket(id uint, updates map[string]interface{}) error {
	updates["updated_at"] = util.HTime{Time: time.Now()}
	return common.GetDB().Model(&dbmodel.SQLTicket{}).Where("id = ?", id).Updates(updates).Error
}

func (d *ReviewDao) AddApprovalRecord(record *dbmodel.SQLApprovalRecord) error {
	return common.GetDB().Create(record).Error
}

func (d *ReviewDao) AddExecutionRecords(records []dbmodel.SQLExecutionRecord) error {
	if len(records) == 0 {
		return nil
	}
	return common.GetDB().Create(&records).Error
}

func (d *ReviewDao) AddExecutionRecord(record *dbmodel.SQLExecutionRecord) error {
	return common.GetDB().Create(record).Error
}

func (d *ReviewDao) UpdateExecutionRecord(id uint, updates map[string]interface{}) error {
	return common.GetDB().Model(&dbmodel.SQLExecutionRecord{}).Where("id = ?", id).Updates(updates).Error
}

func (d *ReviewDao) AddRollbackRecord(record *dbmodel.SQLRollbackRecord) error {
	if record == nil {
		return nil
	}
	return common.GetDB().Create(record).Error
}

func (d *ReviewDao) UpdateRollbackRecord(id uint, updates map[string]interface{}) error {
	return common.GetDB().Model(&dbmodel.SQLRollbackRecord{}).Where("id = ?", id).Updates(updates).Error
}

func applyTicketFilters(db *gorm.DB, query dbmodel.SQLTicketQuery) *gorm.DB {
	if value := strings.TrimSpace(query.Keyword); value != "" {
		like := "%" + value + "%"
		db = db.Where("ticket_no LIKE ? OR title LIKE ? OR sql_content LIKE ?", like, like, like)
	}
	if value := strings.TrimSpace(query.TicketType); value != "" {
		db = db.Where("ticket_type = ?", value)
	}
	if value := strings.TrimSpace(query.Status); value != "" {
		db = db.Where("status = ?", value)
	}
	if query.AssetID > 0 {
		db = db.Where("asset_id = ?", query.AssetID)
	}
	if value := strings.TrimSpace(query.Database); value != "" {
		db = db.Where("database_name = ?", value)
	}
	if value := strings.TrimSpace(query.Environment); value != "" {
		db = db.Where("environment = ?", value)
	}
	if value := strings.TrimSpace(query.SubmitterName); value != "" {
		db = db.Where("submitter_name LIKE ?", "%"+value+"%")
	}
	if value := strings.TrimSpace(query.ApproverName); value != "" {
		db = db.Where("approver_name LIKE ?", "%"+value+"%")
	}
	if value := strings.TrimSpace(query.ExecutorName); value != "" {
		db = db.Where("executor_name LIKE ?", "%"+value+"%")
	}
	if value := strings.TrimSpace(query.BeginTime); value != "" {
		db = db.Where("created_at >= ?", value)
	}
	if value := strings.TrimSpace(query.EndTime); value != "" {
		db = db.Where("created_at <= ?", value)
	}
	return db
}
