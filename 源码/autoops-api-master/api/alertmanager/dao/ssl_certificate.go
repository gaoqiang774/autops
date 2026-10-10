package dao

import (
	"dodevops-api/api/alertmanager/model"
	"dodevops-api/common/util"
	"encoding/json"
	"strings"
	"time"
)

func (d *AlertManagerDao) CreateSSLCertificate(item *model.SSLCertificate) error {
	now := util.HTime{Time: time.Now()}
	item.CreatedAt = now
	item.UpdatedAt = now
	return d.db.Create(item).Error
}

func (d *AlertManagerDao) HasPendingSSLCertificate(domain string) (bool, error) {
	var count int64
	err := d.db.Model(&model.SSLCertificate{}).
		Where("domain = ? AND status = ?", domain, model.SSLCertificateStatusApplying).
		Count(&count).Error
	return count > 0, err
}

func (d *AlertManagerDao) ListSSLCertificates(query model.SSLCertificateQuery) ([]model.SSLCertificate, int64, error) {
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	db := d.db.Model(&model.SSLCertificate{})
	if keyword := strings.TrimSpace(query.Keyword); keyword != "" {
		db = db.Where("domain LIKE ?", "%"+keyword+"%")
	}
	if query.Status > 0 {
		db = db.Where("status = ?", query.Status)
	}
	if query.KeyManageID > 0 {
		db = db.Where("key_manage_id = ?", query.KeyManageID)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.SSLCertificate
	err := db.Order("created_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *AlertManagerDao) GetSSLCertificate(id uint) (*model.SSLCertificate, error) {
	var item model.SSLCertificate
	err := d.db.First(&item, id).Error
	return &item, err
}

func (d *AlertManagerDao) DeleteSSLCertificate(id uint) (bool, error) {
	result := d.db.Where("id = ? AND status <> ?", id, model.SSLCertificateStatusApplying).
		Delete(&model.SSLCertificate{})
	return result.RowsAffected > 0, result.Error
}

func (d *AlertManagerDao) MarkSSLCertificateIssued(id uint, certificate, privateKey, issuerCertificate, certificateURL string, issuedAt, expiresAt time.Time) error {
	return d.db.Model(&model.SSLCertificate{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":             model.SSLCertificateStatusIssued,
		"certificate":        certificate,
		"private_key":        privateKey,
		"issuer_certificate": issuerCertificate,
		"certificate_url":    certificateURL,
		"issued_at":          util.HTime{Time: issuedAt},
		"expires_at":         util.HTime{Time: expiresAt},
		"error_message":      "",
		"updated_at":         util.HTime{Time: time.Now()},
	}).Error
}

func (d *AlertManagerDao) MarkSSLCertificateFailed(id uint, message string) error {
	return d.db.Model(&model.SSLCertificate{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        model.SSLCertificateStatusFailed,
		"error_message": message,
		"updated_at":    util.HTime{Time: time.Now()},
	}).Error
}

func (d *AlertManagerDao) CreateSSLCertificateDeployLog(item *model.SSLCertificateDeployLog, logs []string) error {
	now := util.HTime{Time: time.Now()}
	encodedLogs, _ := json.Marshal(logs)
	item.LogContent = string(encodedLogs)
	item.StartedAt = now
	item.CreatedAt = now
	item.UpdatedAt = now
	return d.db.Create(item).Error
}

func (d *AlertManagerDao) HasRunningSSLCertificateDeployment(certificateID, hostID uint) (bool, error) {
	db := d.db.Model(&model.SSLCertificateDeployLog{}).
		Where("certificate_id = ? AND status = ?", certificateID, model.SSLCertificateDeployStatusRunning)
	if hostID > 0 {
		db = db.Where("host_id = ?", hostID)
	}
	var count int64
	err := db.Count(&count).Error
	return count > 0, err
}

func (d *AlertManagerDao) GetSSLCertificateDeployLog(certificateID, logID uint) (*model.SSLCertificateDeployLog, error) {
	var item model.SSLCertificateDeployLog
	err := d.db.Where("certificate_id = ? AND id = ?", certificateID, logID).First(&item).Error
	return &item, err
}

func (d *AlertManagerDao) ListSSLCertificateDeployLogs(certificateID uint, query model.SSLCertificateDeployLogQuery) ([]model.SSLCertificateDeployLog, int64, error) {
	page, pageSize := model.NormalizePage(query.Page, query.PageSize)
	db := d.db.Model(&model.SSLCertificateDeployLog{}).Where("certificate_id = ?", certificateID)
	if query.Status > 0 {
		db = db.Where("status = ?", query.Status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.SSLCertificateDeployLog
	err := db.Order("created_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

func (d *AlertManagerDao) UpdateSSLCertificateDeployProgress(id uint, logs []string, backupPath string) error {
	encodedLogs, _ := json.Marshal(logs)
	return d.db.Model(&model.SSLCertificateDeployLog{}).Where("id = ? AND status = ?", id, model.SSLCertificateDeployStatusRunning).
		Updates(map[string]interface{}{
			"log_content": string(encodedLogs),
			"backup_path": backupPath,
			"updated_at":  util.HTime{Time: time.Now()},
		}).Error
}

func (d *AlertManagerDao) FinishSSLCertificateDeployLog(id uint, status int, logs []string, backupPath, errorMessage string) error {
	now := util.HTime{Time: time.Now()}
	encodedLogs, _ := json.Marshal(logs)
	return d.db.Model(&model.SSLCertificateDeployLog{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        status,
		"log_content":   string(encodedLogs),
		"backup_path":   backupPath,
		"error_message": errorMessage,
		"finished_at":   now,
		"updated_at":    now,
	}).Error
}
