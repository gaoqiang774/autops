package skill

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// LoadError records a single SKILL.md load failure without failing the whole scan.
type LoadError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// CatalogSnapshot is returned by the loader-backed catalog.
type CatalogSnapshot struct {
	Root      string      `json:"root"`
	Total     int         `json:"total"`
	Loaded    int         `json:"loaded"`
	Errors    []LoadError `json:"errors,omitempty"`
	LoadedAt  string      `json:"loadedAt"`
	Manifests []Manifest  `json:"manifests"`
}

// Loader scans declarative Skill documents from a filesystem directory.
type Loader struct {
	Root string
}

func NewLoader(root string) *Loader {
	return &Loader{Root: strings.TrimSpace(root)}
}

func (l *Loader) Load() ([]*Document, []LoadError, error) {
	root := l.Root
	if root == "" {
		root = "skills"
	}
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return []*Document{}, nil, nil
		}
		return nil, nil, err
	}

	docs := make([]*Document, 0)
	loadErrors := make([]LoadError, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			loadErrors = append(loadErrors, LoadError{Path: path, Error: err.Error()})
			return nil
		}
		if entry.IsDir() || entry.Name() != "SKILL.md" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			loadErrors = append(loadErrors, LoadError{Path: path, Error: readErr.Error()})
			return nil
		}
		doc, parseErr := ParseDocument(data)
		if parseErr != nil {
			loadErrors = append(loadErrors, LoadError{Path: path, Error: parseErr.Error()})
			return nil
		}
		doc.Path = filepath.ToSlash(path)
		doc.Dir = filepath.ToSlash(filepath.Dir(path))
		docs = append(docs, doc)
		return nil
	})
	if err != nil {
		return nil, loadErrors, err
	}
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].Manifest.Name < docs[j].Manifest.Name
	})
	return docs, loadErrors, nil
}

// Catalog keeps the latest loaded Skill documents in memory.
type Catalog struct {
	root     string
	docs     map[string]*Document
	errors   []LoadError
	loadedAt time.Time
	mu       sync.RWMutex
}

var defaultCatalog = NewCatalog("skills")

func GetDefaultCatalog() *Catalog {
	return defaultCatalog
}

func NewCatalog(root string) *Catalog {
	return &Catalog{
		root: strings.TrimSpace(root),
		docs: map[string]*Document{},
	}
}

func (c *Catalog) Reload() (*CatalogSnapshot, error) {
	loader := NewLoader(c.root)
	docs, loadErrors, err := loader.Load()
	if err != nil {
		return nil, err
	}

	next := make(map[string]*Document, len(docs))
	duplicateErrors := make([]LoadError, 0)
	for _, doc := range docs {
		name := doc.Manifest.Name
		if existing, ok := next[name]; ok {
			duplicateErrors = append(duplicateErrors, LoadError{
				Path:  doc.Path,
				Error: fmt.Sprintf("duplicate skill name %q already loaded from %s", name, existing.Path),
			})
			continue
		}
		next[name] = doc
	}
	loadErrors = append(loadErrors, duplicateErrors...)

	c.mu.Lock()
	c.docs = next
	c.errors = loadErrors
	c.loadedAt = time.Now()
	c.mu.Unlock()

	return c.Snapshot(), nil
}

func (c *Catalog) Snapshot() *CatalogSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	manifests := make([]Manifest, 0, len(c.docs))
	for _, doc := range c.docs {
		manifests = append(manifests, doc.Manifest)
	}
	sort.Slice(manifests, func(i, j int) bool {
		if manifests[i].SkillGroup == manifests[j].SkillGroup {
			return manifests[i].Name < manifests[j].Name
		}
		return manifests[i].SkillGroup < manifests[j].SkillGroup
	})
	errorsCopy := append([]LoadError(nil), c.errors...)

	return &CatalogSnapshot{
		Root:      c.root,
		Total:     len(c.docs) + len(errorsCopy),
		Loaded:    len(c.docs),
		Errors:    errorsCopy,
		LoadedAt:  c.loadedAt.Format("2006-01-02 15:04:05"),
		Manifests: manifests,
	}
}

func (c *Catalog) List(filters map[string]string) []Manifest {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]Manifest, 0, len(c.docs))
	for _, doc := range c.docs {
		manifest := doc.Manifest
		if !manifestMatchesFilters(manifest, filters) {
			continue
		}
		result = append(result, manifest)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].SkillGroup == result[j].SkillGroup {
			return result[i].Name < result[j].Name
		}
		return result[i].SkillGroup < result[j].SkillGroup
	})
	return result
}

func (c *Catalog) Get(name string) (*Document, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	doc, ok := c.docs[name]
	return doc, ok
}

func (c *Catalog) EnsureLoaded() error {
	c.mu.RLock()
	loaded := !c.loadedAt.IsZero()
	c.mu.RUnlock()
	if loaded {
		return nil
	}
	_, err := c.Reload()
	return err
}

func ValidateSkillFile(path string) (*Document, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, err := ParseDocument(data)
	if err != nil {
		return nil, err
	}
	doc.Path = filepath.ToSlash(path)
	doc.Dir = filepath.ToSlash(filepath.Dir(path))
	return doc, nil
}

func manifestMatchesFilters(manifest Manifest, filters map[string]string) bool {
	for key, value := range filters {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		switch key {
		case "category":
			if manifest.Category != value {
				return false
			}
		case "domain":
			if manifest.Domain != value {
				return false
			}
		case "skillGroup":
			if manifest.SkillGroup != value {
				return false
			}
		case "runtime":
			if manifest.Runtime != value {
				return false
			}
		case "status":
			if manifest.Status != value {
				return false
			}
		case "riskLevel":
			if manifest.RiskLevel != value {
				return false
			}
		case "keyword":
			if !manifestContainsKeyword(manifest, value) {
				return false
			}
		}
	}
	return true
}

func manifestContainsKeyword(manifest Manifest, keyword string) bool {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return true
	}
	values := []string{
		manifest.Name,
		manifest.DisplayName,
		manifest.Description,
		manifest.Category,
		manifest.Domain,
		manifest.SkillGroup,
	}
	values = append(values, manifest.Tags...)
	values = append(values, manifest.Triggers...)
	values = append(values, manifest.Tools...)
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), keyword) {
			return true
		}
	}
	return false
}
