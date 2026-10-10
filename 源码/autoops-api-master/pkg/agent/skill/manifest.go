package skill

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	SkillStatusEnabled    = "enabled"
	SkillStatusDisabled   = "disabled"
	SkillStatusDraft      = "draft"
	SkillStatusPlanned    = "planned"
	SkillStatusDeprecated = "deprecated"

	SkillRuntimeDeclarative = "declarative"
	SkillRuntimeGo          = "go"
	SkillRuntimeScript      = "script"
	SkillRuntimeHTTP        = "http"
	SkillRuntimeMCP         = "mcp"

	SkillRiskReadonly = "readonly"
	SkillRiskLow      = "low"
	SkillRiskMedium   = "medium"
	SkillRiskHigh     = "high"
)

// Manifest describes a declarative Skill loaded from SKILL.md frontmatter.
type Manifest struct {
	Name         string                 `json:"name" yaml:"name"`
	DisplayName  string                 `json:"displayName" yaml:"displayName"`
	Version      string                 `json:"version" yaml:"version"`
	Description  string                 `json:"description" yaml:"description"`
	Category     string                 `json:"category" yaml:"category"`
	Domain       string                 `json:"domain" yaml:"domain"`
	SkillGroup   string                 `json:"skillGroup" yaml:"skillGroup"`
	Runtime      string                 `json:"runtime" yaml:"runtime"`
	Status       string                 `json:"status" yaml:"status"`
	RiskLevel    string                 `json:"riskLevel" yaml:"riskLevel"`
	Readonly     bool                   `json:"readonly" yaml:"readonly"`
	Source       string                 `json:"source" yaml:"source"`
	Tags         []string               `json:"tags" yaml:"tags"`
	Triggers     []string               `json:"triggers" yaml:"triggers"`
	Tools        []string               `json:"tools" yaml:"tools"`
	Permissions  []string               `json:"permissions" yaml:"permissions"`
	InputSchema  map[string]interface{} `json:"inputSchema,omitempty" yaml:"inputSchema"`
	OutputSchema map[string]interface{} `json:"outputSchema,omitempty" yaml:"outputSchema"`
}

// Document is a parsed SKILL.md file.
type Document struct {
	Manifest Manifest `json:"manifest"`
	Body     string   `json:"body"`
	Path     string   `json:"path"`
	Dir      string   `json:"dir"`
}

// ParseDocument parses a SKILL.md document with YAML frontmatter.
func ParseDocument(content []byte) (*Document, error) {
	text := strings.TrimPrefix(string(content), "\ufeff")
	if !strings.HasPrefix(text, "---") {
		return nil, errors.New("missing YAML frontmatter")
	}

	lines := strings.Split(text, "\n")
	if strings.TrimSpace(lines[0]) != "---" {
		return nil, errors.New("invalid YAML frontmatter start")
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return nil, errors.New("missing YAML frontmatter end")
	}

	header := strings.Join(lines[1:end], "\n")
	body := strings.Join(lines[end+1:], "\n")

	var manifest Manifest
	if err := yaml.Unmarshal([]byte(header), &manifest); err != nil {
		return nil, fmt.Errorf("parse YAML frontmatter: %w", err)
	}
	normalizeManifest(&manifest)
	if err := ValidateManifest(manifest); err != nil {
		return nil, err
	}
	return &Document{Manifest: manifest, Body: strings.TrimSpace(body)}, nil
}

// ValidateManifest checks the required fields and supported enum values.
func ValidateManifest(manifest Manifest) error {
	if strings.TrimSpace(manifest.Name) == "" {
		return errors.New("name is required")
	}
	if strings.TrimSpace(manifest.Description) == "" {
		return errors.New("description is required")
	}
	if strings.TrimSpace(manifest.Category) == "" {
		return errors.New("category is required")
	}
	if strings.TrimSpace(manifest.Domain) == "" {
		return errors.New("domain is required")
	}
	if strings.TrimSpace(manifest.SkillGroup) == "" {
		return errors.New("skillGroup is required")
	}
	if !allowedSkillRuntime(manifest.Runtime) {
		return fmt.Errorf("unsupported runtime: %s", manifest.Runtime)
	}
	if !allowedSkillStatus(manifest.Status) {
		return fmt.Errorf("unsupported status: %s", manifest.Status)
	}
	if !allowedSkillRisk(manifest.RiskLevel) {
		return fmt.Errorf("unsupported riskLevel: %s", manifest.RiskLevel)
	}
	return nil
}

func normalizeManifest(manifest *Manifest) {
	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.DisplayName = strings.TrimSpace(manifest.DisplayName)
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.Description = strings.TrimSpace(manifest.Description)
	manifest.Category = strings.TrimSpace(manifest.Category)
	manifest.Domain = strings.TrimSpace(manifest.Domain)
	manifest.SkillGroup = strings.TrimSpace(manifest.SkillGroup)
	manifest.Runtime = strings.TrimSpace(manifest.Runtime)
	manifest.Status = strings.TrimSpace(manifest.Status)
	manifest.RiskLevel = strings.TrimSpace(manifest.RiskLevel)
	manifest.Source = strings.TrimSpace(manifest.Source)
	if manifest.Version == "" {
		manifest.Version = "0.1.0"
	}
	if manifest.Runtime == "" {
		manifest.Runtime = SkillRuntimeDeclarative
	}
	if manifest.Status == "" {
		manifest.Status = SkillStatusDraft
	}
	if manifest.RiskLevel == "" {
		manifest.RiskLevel = SkillRiskReadonly
	}
	if manifest.Source == "" {
		manifest.Source = "workspace"
	}
	manifest.Tags = compactStringSlice(manifest.Tags)
	manifest.Triggers = compactStringSlice(manifest.Triggers)
	manifest.Tools = compactStringSlice(manifest.Tools)
	manifest.Permissions = compactStringSlice(manifest.Permissions)
}

func compactStringSlice(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func allowedSkillRuntime(value string) bool {
	switch value {
	case SkillRuntimeDeclarative, SkillRuntimeGo, SkillRuntimeScript, SkillRuntimeHTTP, SkillRuntimeMCP:
		return true
	default:
		return false
	}
}

func allowedSkillStatus(value string) bool {
	switch value {
	case SkillStatusEnabled, SkillStatusDisabled, SkillStatusDraft, SkillStatusPlanned, SkillStatusDeprecated:
		return true
	default:
		return false
	}
}

func allowedSkillRisk(value string) bool {
	switch value {
	case SkillRiskReadonly, SkillRiskLow, SkillRiskMedium, SkillRiskHigh:
		return true
	default:
		return false
	}
}
