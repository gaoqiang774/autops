package model

import "time"

// CmdbWindowsExporterStatus stores Windows Exporter state for a CMDB host.
// It is intentionally separate from cmdb_snmp_devices, which only owns network devices.
type CmdbWindowsExporterStatus struct {
	ID        uint      `gorm:"column:id;primaryKey" json:"id"`
	HostID    uint      `gorm:"column:host_id;not null;uniqueIndex" json:"hostId"`
	Status    int       `gorm:"column:status;not null;default:0" json:"status"`
	Port      int       `gorm:"column:port;not null;default:9182" json:"port"`
	Version   string    `gorm:"column:version;size:50" json:"version"`
	LastError string    `gorm:"column:last_error;type:text" json:"lastError"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (CmdbWindowsExporterStatus) TableName() string {
	return "cmdb_windows_exporter_status"
}
