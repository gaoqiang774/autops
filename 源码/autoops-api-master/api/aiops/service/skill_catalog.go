package service

import (
	"strings"

	"dodevops-api/pkg/agent/skill"
)

type SkillCatalogService struct {
	catalog *skill.Catalog
}

type SkillValidateRequest struct {
	Content string `json:"content"`
}

type SkillValidateResult struct {
	Valid    bool            `json:"valid"`
	Manifest *skill.Manifest `json:"manifest,omitempty"`
	Error    string          `json:"error,omitempty"`
}

func NewSkillCatalogService() *SkillCatalogService {
	return &SkillCatalogService{
		catalog: skill.GetDefaultCatalog(),
	}
}

func (s *SkillCatalogService) List(filters map[string]string) ([]skill.Manifest, error) {
	if err := s.catalog.EnsureLoaded(); err != nil {
		return nil, err
	}
	return s.catalog.List(filters), nil
}

func (s *SkillCatalogService) Get(name string) (*skill.Document, bool, error) {
	if err := s.catalog.EnsureLoaded(); err != nil {
		return nil, false, err
	}
	doc, ok := s.catalog.Get(name)
	return doc, ok, nil
}

func (s *SkillCatalogService) Reload() (*skill.CatalogSnapshot, error) {
	return s.catalog.Reload()
}

func (s *SkillCatalogService) Snapshot() (*skill.CatalogSnapshot, error) {
	if err := s.catalog.EnsureLoaded(); err != nil {
		return nil, err
	}
	return s.catalog.Snapshot(), nil
}

func (s *SkillCatalogService) Validate(req SkillValidateRequest) SkillValidateResult {
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return SkillValidateResult{Valid: false, Error: "content is required"}
	}
	doc, err := skill.ParseDocument([]byte(content))
	if err != nil {
		return SkillValidateResult{Valid: false, Error: err.Error()}
	}
	return SkillValidateResult{Valid: true, Manifest: &doc.Manifest}
}

func (s *SkillCatalogService) Categories() (map[string]interface{}, error) {
	snapshot, err := s.Snapshot()
	if err != nil {
		return nil, err
	}

	categories := map[string]int{}
	domains := map[string]int{}
	groups := map[string]int{}
	runtimes := map[string]int{}
	statuses := map[string]int{}
	risks := map[string]int{}
	for _, manifest := range snapshot.Manifests {
		categories[manifest.Category]++
		domains[manifest.Domain]++
		groups[manifest.SkillGroup]++
		runtimes[manifest.Runtime]++
		statuses[manifest.Status]++
		risks[manifest.RiskLevel]++
	}
	return map[string]interface{}{
		"categories": categories,
		"domains":    domains,
		"groups":     groups,
		"runtimes":   runtimes,
		"statuses":   statuses,
		"riskLevels": risks,
		"loaded":     snapshot.Loaded,
		"errors":     snapshot.Errors,
		"loadedAt":   snapshot.LoadedAt,
	}, nil
}
