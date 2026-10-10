package model

import (
	"dodevops-api/common/util"
	"encoding/json"
)

const (
	SSLCertificateStatusApplying = 1
	SSLCertificateStatusIssued   = 3
	SSLCertificateStatusFailed   = 5
)

const (
	SSLCertificateTypeACME         = 1
	SSLCertificateCALetsEncrypt    = "LetsEncrypt"
	SSLCertificateAlgorithmRSA2048 = "RSA2048"
	SSLCertificateAlgorithmRSA4096 = "RSA4096"
	SSLCertificateAlgorithmEC256   = "EC256"
	SSLCertificateAlgorithmEC384   = "EC384"
)

const (
	SSLCertificateDeployStatusRunning = 1
	SSLCertificateDeployStatusSuccess = 2
	SSLCertificateDeployStatusFailed  = 3
)

// SSLCertificate stores an ACME order and its issued certificate material.
// Private key material is deliberately excluded from normal JSON responses.
type SSLCertificate struct {
	ID                uint        `gorm:"primaryKey" json:"id"`
	Domain            string      `gorm:"size:253;not null;index" json:"domain"`
	KeyManageID       uint        `gorm:"not null;index" json:"keyManageId"`
	CAProvider        string      `gorm:"size:32;not null" json:"caProvider"`
	Algorithm         string      `gorm:"size:16;not null" json:"algorithm"`
	Email             string      `gorm:"size:254;not null" json:"email"`
	Status            int         `gorm:"type:tinyint;not null;index" json:"status"`
	Certificate       string      `gorm:"type:longtext" json:"-"`
	PrivateKey        string      `gorm:"type:longtext" json:"-"`
	IssuerCertificate string      `gorm:"type:longtext" json:"-"`
	CertificateURL    string      `gorm:"size:1000" json:"certificateUrl,omitempty"`
	IssuedAt          *util.HTime `json:"issuedAt"`
	ExpiresAt         *util.HTime `gorm:"index" json:"expiresAt"`
	ErrorMessage      string      `gorm:"type:text" json:"errorMessage,omitempty"`
	CreatedBy         string      `gorm:"size:64" json:"createdBy"`
	CreatedAt         util.HTime  `json:"createdAt"`
	UpdatedAt         util.HTime  `json:"updatedAt"`
}

func (SSLCertificate) TableName() string {
	return "alert_ssl_certificates"
}

// SSLCertificateDetail exposes certificate material without exposing the private key.
type SSLCertificateDetail struct {
	ID                uint        `json:"id"`
	Domain            string      `json:"domain"`
	KeyManageID       uint        `json:"keyManageId"`
	CAProvider        string      `json:"caProvider"`
	Algorithm         string      `json:"algorithm"`
	Email             string      `json:"email"`
	Status            int         `json:"status"`
	Certificate       string      `json:"certificate,omitempty"`
	IssuerCertificate string      `json:"issuerCertificate,omitempty"`
	HasPrivateKey     bool        `json:"hasPrivateKey"`
	CertificateURL    string      `json:"certificateUrl,omitempty"`
	IssuedAt          *util.HTime `json:"issuedAt"`
	ExpiresAt         *util.HTime `json:"expiresAt"`
	ErrorMessage      string      `json:"errorMessage,omitempty"`
	CreatedBy         string      `json:"createdBy"`
	CreatedAt         util.HTime  `json:"createdAt"`
	UpdatedAt         util.HTime  `json:"updatedAt"`
}

func (item *SSLCertificate) ToDetail() *SSLCertificateDetail {
	return &SSLCertificateDetail{
		ID:                item.ID,
		Domain:            item.Domain,
		KeyManageID:       item.KeyManageID,
		CAProvider:        item.CAProvider,
		Algorithm:         item.Algorithm,
		Email:             item.Email,
		Status:            item.Status,
		Certificate:       item.Certificate,
		IssuerCertificate: item.IssuerCertificate,
		HasPrivateKey:     item.PrivateKey != "",
		CertificateURL:    item.CertificateURL,
		IssuedAt:          item.IssuedAt,
		ExpiresAt:         item.ExpiresAt,
		ErrorMessage:      item.ErrorMessage,
		CreatedBy:         item.CreatedBy,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}

type ApplySSLCertificateInput struct {
	Domain         string `json:"domain"`
	CertType       int    `json:"certType"`
	KeyManageID    uint   `json:"keyManageId"`
	AliyunConfigID uint   `json:"aliyunConfigId"`
	CAProvider     string `json:"caProvider"`
	Algorithm      string `json:"algorithm"`
	Email          string `json:"email"`
}

type SSLCertificateQuery struct {
	PageQuery
	Status      int  `form:"status" json:"status"`
	KeyManageID uint `form:"keyManageId" json:"keyManageId"`
}

type DeploySSLCertificateInput struct {
	HostID     uint   `json:"hostId"`
	DeployPath string `json:"deployPath"`
}

type SSLCertificateDeployLogQuery struct {
	PageQuery
	Status int `form:"status" json:"status"`
}

type SSLCertificateDeployLog struct {
	ID              uint        `gorm:"primaryKey" json:"id"`
	CertificateID   uint        `gorm:"not null;index" json:"certificateId"`
	Domain          string      `gorm:"size:253;not null;index" json:"domain"`
	HostID          uint        `gorm:"not null;index" json:"hostId"`
	HostName        string      `gorm:"size:128;not null" json:"hostName"`
	HostIP          string      `gorm:"size:64;not null" json:"hostIp"`
	AuthID          uint        `gorm:"not null" json:"authId"`
	DeployPath      string      `gorm:"size:500;not null" json:"deployPath"`
	CertificatePath string      `gorm:"size:768;not null" json:"certificatePath"`
	PrivateKeyPath  string      `gorm:"size:768;not null" json:"privateKeyPath"`
	BackupPath      string      `gorm:"size:768" json:"backupPath,omitempty"`
	Status          int         `gorm:"type:tinyint;not null;index" json:"status"`
	LogContent      string      `gorm:"type:longtext" json:"-"`
	ErrorMessage    string      `gorm:"type:text" json:"errorMessage,omitempty"`
	CreatedBy       string      `gorm:"size:64" json:"createdBy"`
	StartedAt       util.HTime  `json:"startedAt"`
	FinishedAt      *util.HTime `json:"finishedAt"`
	CreatedAt       util.HTime  `json:"createdAt"`
	UpdatedAt       util.HTime  `json:"updatedAt"`
}

func (SSLCertificateDeployLog) TableName() string {
	return "alert_ssl_certificate_deploy_logs"
}

type SSLCertificateDeployLogVO struct {
	ID              uint        `json:"id"`
	CertificateID   uint        `json:"certificateId"`
	Domain          string      `json:"domain"`
	HostID          uint        `json:"hostId"`
	HostName        string      `json:"hostName"`
	HostIP          string      `json:"hostIp"`
	DeployPath      string      `json:"deployPath"`
	CertificatePath string      `json:"certificatePath"`
	PrivateKeyPath  string      `json:"privateKeyPath"`
	BackupPath      string      `json:"backupPath,omitempty"`
	Status          int         `json:"status"`
	StatusText      string      `json:"statusText"`
	Logs            []string    `json:"logs"`
	ErrorMessage    string      `json:"errorMessage,omitempty"`
	CreatedBy       string      `json:"createdBy"`
	StartedAt       util.HTime  `json:"startedAt"`
	FinishedAt      *util.HTime `json:"finishedAt"`
	CreatedAt       util.HTime  `json:"createdAt"`
	UpdatedAt       util.HTime  `json:"updatedAt"`
}

func (item *SSLCertificateDeployLog) ToVO() *SSLCertificateDeployLogVO {
	logs := make([]string, 0)
	if item.LogContent != "" {
		_ = json.Unmarshal([]byte(item.LogContent), &logs)
	}
	statusText := map[int]string{
		SSLCertificateDeployStatusRunning: "部署中",
		SSLCertificateDeployStatusSuccess: "成功",
		SSLCertificateDeployStatusFailed:  "失败",
	}[item.Status]
	if statusText == "" {
		statusText = "未知"
	}
	return &SSLCertificateDeployLogVO{
		ID: item.ID, CertificateID: item.CertificateID, Domain: item.Domain,
		HostID: item.HostID, HostName: item.HostName, HostIP: item.HostIP,
		DeployPath: item.DeployPath, CertificatePath: item.CertificatePath,
		PrivateKeyPath: item.PrivateKeyPath, BackupPath: item.BackupPath,
		Status: item.Status, StatusText: statusText, Logs: logs,
		ErrorMessage: item.ErrorMessage, CreatedBy: item.CreatedBy,
		StartedAt: item.StartedAt, FinishedAt: item.FinishedAt,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}
