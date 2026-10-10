package dao

import (
	dbmodel "dodevops-api/api/db/model"
	"dodevops-api/common"
	"dodevops-api/common/util"
	"strings"
	"time"
)

type SlowLogTaskDao struct{}

func NewSlowLogTaskDao() *SlowLogTaskDao {
	return &SlowLogTaskDao{}
}

func (d *SlowLogTaskDao) Create(task *dbmodel.SlowLogAnalyzeTask) error {
	now := util.HTime{Time: time.Now()}
	if task.CreatedAt.Time.IsZero() {
		task.CreatedAt = now
	}
	if task.UpdatedAt.Time.IsZero() {
		task.UpdatedAt = now
	}
	return common.GetDB().Create(task).Error
}

func (d *SlowLogTaskDao) GetByTaskID(taskID string) (*dbmodel.SlowLogAnalyzeTask, error) {
	var task dbmodel.SlowLogAnalyzeTask
	err := common.GetDB().Where("task_id = ?", taskID).First(&task).Error
	return &task, err
}

func (d *SlowLogTaskDao) List(assetID uint, query dbmodel.SlowLogTaskQuery) ([]dbmodel.SlowLogAnalyzeTask, int64, error) {
	var list []dbmodel.SlowLogAnalyzeTask
	var total int64

	db := common.GetDB().Model(&dbmodel.SlowLogAnalyzeTask{}).Where("asset_id = ?", assetID)
	if status := strings.TrimSpace(query.Status); status != "" {
		db = db.Where("status = ?", status)
	}
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("task_id LIKE ? OR slow_log_path LIKE ? OR host_ip LIKE ? OR host_name LIKE ? OR created_by_name LIKE ?", like, like, like, like, like)
	}
	if begin := strings.TrimSpace(query.BeginTime); begin != "" {
		db = db.Where("created_at >= ?", begin)
	}
	if end := strings.TrimSpace(query.EndTime); end != "" {
		db = db.Where("created_at <= ?", end)
	}
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, pageSize := dbmodel.NormalizePage(query.Page, query.PageSize)
	err := db.Order("created_at DESC, id DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&list).Error
	return list, total, err
}

func (d *SlowLogTaskDao) UpdateFields(taskID string, fields map[string]interface{}) error {
	fields["updated_at"] = util.HTime{Time: time.Now()}
	return common.GetDB().Model(&dbmodel.SlowLogAnalyzeTask{}).
		Where("task_id = ?", taskID).
		Updates(fields).Error
}

func (d *SlowLogTaskDao) DeleteByTaskID(taskID string) error {
	return common.GetDB().Where("task_id = ?", taskID).Delete(&dbmodel.SlowLogAnalyzeTask{}).Error
}

func (d *SlowLogTaskDao) ClearFinished(assetID uint) (int64, error) {
	tx := common.GetDB().
		Where("asset_id = ? AND status IN ?", assetID, []string{dbmodel.SlowLogTaskStatusSuccess, dbmodel.SlowLogTaskStatusFailed}).
		Delete(&dbmodel.SlowLogAnalyzeTask{})
	return tx.RowsAffected, tx.Error
}
