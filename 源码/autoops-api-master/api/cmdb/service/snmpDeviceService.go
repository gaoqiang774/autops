package service

import (
	"errors"
	"strings"
	"time"

	"dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/model"
	"dodevops-api/common/util"
)

type CmdbSnmpDeviceService struct {
	deviceDao *dao.CmdbSnmpDeviceDao
}

func NewCmdbSnmpDeviceService() *CmdbSnmpDeviceService {
	return &CmdbSnmpDeviceService{
		deviceDao: dao.NewCmdbSnmpDeviceDao(),
	}
}

func (s *CmdbSnmpDeviceService) CreateSnmpDevice(dto *model.CreateSnmpDeviceDto, username string) (*model.CmdbSnmpDevice, error) {
	_ = username

	dto.Normalize()
	if err := validateCreateSnmpDeviceDTO(dto); err != nil {
		return nil, err
	}

	exists, err := s.deviceDao.CheckIPExists(dto.IPAddress, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("ip address already exists")
	}

	now := util.HTime{Time: time.Now()}
	device := &model.CmdbSnmpDevice{
		DeviceName:     dto.DeviceName,
		DeviceType:     dto.DeviceType,
		DeviceVendor:   dto.DeviceVendor,
		DeviceModel:    dto.DeviceModel,
		IPAddress:      dto.IPAddress,
		SnmpURL:        dto.SnmpURL,
		SnmpVersion:    dto.SnmpVersion,
		SnmpCommunity:  dto.SnmpCommunity,
		SnmpPort:       dto.SnmpPort,
		RemotePort:     dto.RemotePort,
		RemoteUsername: dto.RemoteUsername,
		RemotePassword: dto.RemotePassword,
		RemoteDomain:   dto.RemoteDomain,
		MonitorEnabled: 1,
		Location:       dto.Location,
		Owner:          dto.Owner,
		Remark:         dto.Remark,
		Status:         "offline",
		CreateTime:     now,
		UpdateTime:     now,
	}

	if err := s.deviceDao.CreateSnmpDevice(device); err != nil {
		return nil, err
	}

	return device, nil
}

func (s *CmdbSnmpDeviceService) UpdateSnmpDevice(dto *model.UpdateSnmpDeviceDto, username string) error {
	_ = username

	dto.Normalize()
	_, err := s.deviceDao.GetSnmpDeviceById(dto.ID)
	if err != nil {
		return errors.New("device not found")
	}

	if dto.IPAddress != nil && strings.TrimSpace(*dto.IPAddress) != "" {
		exists, err := s.deviceDao.CheckIPExists(strings.TrimSpace(*dto.IPAddress), dto.ID)
		if err != nil {
			return err
		}
		if exists {
			return errors.New("ip address is already used by another device")
		}
	}

	updateFields := map[string]interface{}{
		"update_time": time.Now(),
	}

	if dto.DeviceName != nil {
		updateFields["device_name"] = *dto.DeviceName
	}
	if dto.DeviceType != nil {
		if !isSupportedSnmpDeviceType(*dto.DeviceType) {
			return errors.New("unsupported device type")
		}
		updateFields["device_type"] = *dto.DeviceType
	}
	if dto.DeviceVendor != nil {
		updateFields["device_vendor"] = *dto.DeviceVendor
	}
	if dto.DeviceModel != nil {
		updateFields["device_model"] = *dto.DeviceModel
	}
	if dto.IPAddress != nil {
		updateFields["ip_address"] = *dto.IPAddress
	}
	if dto.SnmpURL != nil {
		updateFields["snmp_url"] = *dto.SnmpURL
	}
	if dto.SnmpVersion != nil {
		updateFields["snmp_version"] = *dto.SnmpVersion
	}
	if dto.SnmpCommunity != nil {
		updateFields["snmp_community"] = *dto.SnmpCommunity
	}
	if dto.SnmpPort != nil {
		updateFields["snmp_port"] = *dto.SnmpPort
	}
	if dto.MonitorEnabled != nil {
		if *dto.MonitorEnabled != 0 && *dto.MonitorEnabled != 1 {
			return errors.New("monitorEnabled must be 0 or 1")
		}
		updateFields["monitor_enabled"] = *dto.MonitorEnabled
	}
	if dto.Location != nil {
		updateFields["location"] = *dto.Location
	}
	if dto.Owner != nil {
		updateFields["owner"] = *dto.Owner
	}
	if dto.Remark != nil {
		updateFields["remark"] = *dto.Remark
	}
	if dto.RemotePort != nil {
		updateFields["remote_port"] = *dto.RemotePort
	}
	if dto.RemoteUsername != nil {
		updateFields["remote_username"] = *dto.RemoteUsername
	}
	if dto.RemotePassword != nil && !model.IsSnmpMaskedSecretValue(*dto.RemotePassword) {
		updateFields["remote_password"] = *dto.RemotePassword
	}
	if dto.RemoteDomain != nil {
		updateFields["remote_domain"] = *dto.RemoteDomain
	}

	return s.deviceDao.UpdateSnmpDeviceFields(dto.ID, updateFields)
}

func (s *CmdbSnmpDeviceService) DeleteSnmpDevice(id uint) error {
	_, err := s.deviceDao.GetSnmpDeviceById(id)
	if err != nil {
		return errors.New("device not found")
	}

	return s.deviceDao.DeleteSnmpDevice(id)
}

func (s *CmdbSnmpDeviceService) GetSnmpDeviceById(id uint) (*model.CmdbSnmpDevice, error) {
	return s.deviceDao.GetSnmpDeviceById(id)
}

func (s *CmdbSnmpDeviceService) DecryptRemotePassword(id uint) (string, error) {
	device, err := s.deviceDao.GetSnmpDeviceById(id)
	if err != nil {
		return "", errors.New("device not found")
	}
	return device.DecryptRemotePassword()
}

func (s *CmdbSnmpDeviceService) GetSnmpDeviceList(query *model.SnmpDeviceQueryDto) (*model.SnmpDeviceListResult, error) {
	devices, total, err := s.deviceDao.GetSnmpDeviceList(query)
	if err != nil {
		return nil, err
	}

	return &model.SnmpDeviceListResult{
		Items:    devices,
		Total:    total,
		Page:     query.Page,
		PageSize: query.PageSize,
	}, nil
}

func (s *CmdbSnmpDeviceService) GetSnmpDeviceStats() (*model.SnmpDeviceStatsDto, error) {
	return s.deviceDao.GetSnmpDeviceStats()
}

func (s *CmdbSnmpDeviceService) GetSnmpDevicesByType(deviceType string) ([]model.CmdbSnmpDevice, error) {
	return s.deviceDao.GetSnmpDevicesByType(deviceType)
}

func (s *CmdbSnmpDeviceService) BatchDeleteSnmpDevices(ids []uint) error {
	for _, id := range ids {
		if err := s.DeleteSnmpDevice(id); err != nil {
			return err
		}
	}
	return nil
}

func (s *CmdbSnmpDeviceService) BatchUpdateDeviceStatus(ids []uint, status string) error {
	return s.deviceDao.BatchUpdateDeviceStatus(ids, status)
}

func (s *CmdbSnmpDeviceService) GetMonitorEnabledDevices() ([]model.CmdbSnmpDevice, error) {
	return s.deviceDao.GetMonitorEnabledDevices()
}

func validateCreateSnmpDeviceDTO(dto *model.CreateSnmpDeviceDto) error {
	if dto.DeviceName == "" {
		return errors.New("deviceName is required")
	}
	if !isSupportedSnmpDeviceType(dto.DeviceType) {
		return errors.New("unsupported device type")
	}
	if dto.IPAddress == "" {
		return errors.New("ipAddress is required")
	}
	if dto.SnmpURL == "" {
		return errors.New("snmpUrl is required")
	}
	if dto.DeviceVendor == "" {
		return errors.New("deviceVendor is required")
	}
	if dto.DeviceModel == "" {
		return errors.New("deviceModel is required")
	}
	return nil
}

func isSupportedSnmpDeviceType(deviceType string) bool {
	switch strings.ToLower(strings.TrimSpace(deviceType)) {
	case "switch", "router", "windows", "linux":
		return true
	default:
		return false
	}
}
