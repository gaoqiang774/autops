package service

import (
	"encoding/json"
	"errors"

	"dodevops-api/api/cmdb/dao"
	"dodevops-api/api/cmdb/model"

	"gorm.io/datatypes"
)

type MonitorDatasourceService struct {
	dao *dao.MonitorDatasourceDAO
}

func NewMonitorDatasourceService(dao *dao.MonitorDatasourceDAO) *MonitorDatasourceService {
	return &MonitorDatasourceService{dao: dao}
}

func (s *MonitorDatasourceService) CreatePrometheus(name string, props model.PrometheusProperties, tags datatypes.JSON, instanceID string) error {
	if props.URL == "" {
		return errors.New("url is required")
	}
	props.DefaultValues()

	propsJSON, _ := json.Marshal(props)
	resource := &model.CmdbBusinessModelBinding{
		Name:         name,
		ResourceType: model.ResourceTypePrometheus,
		Properties:   propsJSON,
		InstanceId:   instanceID,
		Status:       "active",
		Tags:         tags,
	}
	return s.dao.Create(resource)
}

func (s *MonitorDatasourceService) Update(id uint, updates map[string]interface{}) error {
	return s.dao.Update(id, updates)
}

func (s *MonitorDatasourceService) Delete(id uint) error {
	return s.dao.Delete(id)
}

func (s *MonitorDatasourceService) GetByID(id uint) (*model.CmdbBusinessModelBinding, error) {
	return s.dao.GetByID(id)
}

func (s *MonitorDatasourceService) List(resourceType, status string, page, pageSize int) ([]model.CmdbBusinessModelBinding, int64, error) {
	return s.dao.List(resourceType, status, page, pageSize)
}

func (s *MonitorDatasourceService) GetPrometheusConfigs() []model.PrometheusScrapeConfig {
	resources, err := s.dao.GetActivePrometheusResources()
	if err != nil {
		return nil
	}
	return model.GetPrometheusScrapeConfigs(resources)
}
