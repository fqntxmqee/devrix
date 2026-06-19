package configure

import (
	"embed"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

//go:embed data/models.yaml
var catalogFS embed.FS

// ModelCapabilities describes what a single model supports.
//
// DSAFT: D3-S6-A03 (model catalog, v2.x). The catalog is data, not defaults:
// it answers "what does model X support?" not "which model should we use?".
// Model selection remains the user's responsibility (via ~/.devrix/config.yaml
// or DEVRIX_LLM_DEFAULT_MODEL env).
type ModelCapabilities struct {
	ID             string `yaml:"id"`
	DisplayName    string `yaml:"display_name,omitempty"`
	Provider       string `yaml:"provider,omitempty"`
	ContextWindow  int    `yaml:"context_window,omitempty"`
	NativeThinking bool   `yaml:"native_thinking,omitempty"`
	Multimodal     bool   `yaml:"multimodal,omitempty"`
}

// ModelCatalog holds capabilities keyed by model ID, regardless of provider.
type ModelCatalog struct {
	mu     sync.RWMutex
	byID   map[string]*ModelCapabilities
	source string // path or "<embedded>" for diagnostics
}

// DefaultCatalog returns the catalog loaded from the embedded models.yaml.
//
// The embedded catalog ships with devrix and lists publicly available models
// the devrix team has tested. Users can extend it via LoadCatalogFromFile or
// LoadCatalogFromDir.
func DefaultCatalog() (*ModelCatalog, error) {
	data, err := catalogFS.ReadFile("data/models.yaml")
	if err != nil {
		return nil, fmt.Errorf("read embedded catalog: %w", err)
	}
	c, err := parseCatalog(data)
	if err != nil {
		return nil, err
	}
	c.source = "<embedded:data/models.yaml>"
	return c, nil
}

// LoadCatalogFromFile loads a user-supplied catalog file (e.g. a fork with
// private/internal models). Useful when the user wants to add custom model
// definitions without modifying devrix's binary.
func LoadCatalogFromFile(path string) (*ModelCatalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read catalog %s: %w", path, err)
	}
	c, err := parseCatalog(data)
	if err != nil {
		return nil, err
	}
	c.source = path
	return c, nil
}

// Lookup returns capabilities for a model ID, or nil if the model is not in
// the catalog.
//
// A nil result is meaningful: code that branches on NativeThinking must
// treat "model not in catalog" as "behavior unknown, fall back to safest
// option (assume no native thinking → keep inline <think> splitter active)".
func (c *ModelCatalog) Lookup(modelID string) *ModelCapabilities {
	if c == nil || modelID == "" {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.byID[modelID]
}

// Size returns the number of model entries in the catalog.
func (c *ModelCatalog) Size() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.byID)
}

// Source returns the catalog source path for diagnostic logging.
func (c *ModelCatalog) Source() string {
	if c == nil {
		return ""
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.source
}

// catalogYAML mirrors the on-disk YAML schema (providers -> list of models).
// Each model is normalized into a flat byID index regardless of provider.
type catalogYAML struct {
	Providers map[string][]ModelCapabilities `yaml:"providers"`
}

func parseCatalog(data []byte) (*ModelCatalog, error) {
	var raw catalogYAML
	if err := unmarshalYAML(data, &raw); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	byID := make(map[string]*ModelCapabilities)
	for provider, models := range raw.Providers {
		for i := range models {
			m := models[i]
			if m.ID == "" {
				continue
			}
			if m.Provider == "" {
				m.Provider = provider
			}
			mm := m
			byID[mm.ID] = &mm
		}
	}
	return &ModelCatalog{byID: byID}, nil
}

// ListIDs returns all model IDs in the catalog, sorted alphabetically. Useful
// for `--list-models` style CLI commands or startup validation logs.
func (c *ModelCatalog) ListIDs() []string {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	ids := make([]string, 0, len(c.byID))
	for id := range c.byID {
		ids = append(ids, id)
	}
	c.mu.RUnlock()
	sort.Strings(ids)
	return ids
}

// String renders a compact summary for logging.
func (c *ModelCatalog) String() string {
	if c == nil {
		return "<nil catalog>"
	}
	return fmt.Sprintf("ModelCatalog{source=%s, entries=%d, ids=[%s]}",
		c.Source(), c.Size(), strings.Join(c.ListIDs(), ", "))
}