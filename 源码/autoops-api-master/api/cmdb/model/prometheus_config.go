package model

import (
	"encoding/json"

	"gorm.io/datatypes"
)

const ResourceTypePrometheus = "prometheus"

type PrometheusProperties struct {
	URL            string            `json:"url"`
	AuthType       string            `json:"auth_type"`
	Username       string            `json:"username,omitempty"`
	Password       string            `json:"password,omitempty"`
	Token          string            `json:"token,omitempty"`
	ScrapeInterval int               `json:"scrape_interval"`
	Timeout        int               `json:"timeout"`
	ExtraHeaders   map[string]string `json:"extra_headers,omitempty"`
}

func (p *PrometheusProperties) DefaultValues() {
	if p.ScrapeInterval <= 0 {
		p.ScrapeInterval = 60
	}
	if p.Timeout <= 0 {
		p.Timeout = 30
	}
	if p.AuthType == "" {
		p.AuthType = "none"
	}
}

func ParsePrometheusProperties(props datatypes.JSON) (*PrometheusProperties, error) {
	if props == nil {
		return nil, nil
	}
	var result PrometheusProperties
	if err := json.Unmarshal(props, &result); err != nil {
		return nil, err
	}
	result.DefaultValues()
	return &result, nil
}

type PrometheusScrapeConfig struct {
	ID             uint              `json:"id"`
	Name           string            `json:"name"`
	URL            string            `json:"url"`
	AuthType       string            `json:"auth_type"`
	Username       string            `json:"username"`
	Password       string            `json:"password"`
	Token          string            `json:"token"`
	ScrapeInterval int               `json:"scrape_interval"`
	Timeout        int               `json:"timeout"`
	ExtraHeaders   map[string]string `json:"extra_headers"`
	Tags           datatypes.JSON    `json:"tags"`
}

func GetPrometheusScrapeConfigs(resources []CmdbBusinessModelBinding) []PrometheusScrapeConfig {
	var configs []PrometheusScrapeConfig
	for _, res := range resources {
		if res.ResourceType != ResourceTypePrometheus || res.Status != "active" {
			continue
		}
		props, err := ParsePrometheusProperties(res.Properties)
		if err != nil {
			continue
		}
		configs = append(configs, PrometheusScrapeConfig{
			ID:             res.ID,
			Name:           res.Name,
			URL:            props.URL,
			AuthType:       props.AuthType,
			Username:       props.Username,
			Password:       props.Password,
			Token:          props.Token,
			ScrapeInterval: props.ScrapeInterval,
			Timeout:        props.Timeout,
			ExtraHeaders:   props.ExtraHeaders,
			Tags:           res.Tags,
		})
	}
	return configs
}
