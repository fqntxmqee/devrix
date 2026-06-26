package prompt

import (
	"strings"
	"sync"

	"github.com/devrix/devrix/internal/layers/contextengine/i18n"
	"github.com/devrix/devrix/internal/shared/config"
)

// DynamicBoundary separates static and dynamic system prompt sections.
const DynamicBoundary = i18n.DynamicBoundaryMarker

// Cache manages cached section content.
type Cache struct {
	mu      sync.RWMutex
	content map[string]string
}

var (
	globalCache     *Cache
	globalCacheOnce sync.Once
)

// GetCache returns the global cache.
func GetCache() *Cache {
	globalCacheOnce.Do(func() {
		globalCache = &Cache{
			content: make(map[string]string),
		}
	})
	return globalCache
}

// Get returns cached content for a section.
func (c *Cache) Get(name string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	content, ok := c.content[name]
	return content, ok
}

// Set caches content for a section.
func (c *Cache) Set(name, content string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.content[name] = content
}

// Loader loads system prompts from configured sources.
type Loader struct {
	cfg       config.SystemPromptConfig
	cache     *Cache
	staticMap map[string]string
	locale    i18n.Locale
}

// NewLoader creates a prompt loader for the given locale.
func NewLoader(cfg *config.SystemPromptConfig, locale i18n.Locale) *Loader {
	base := config.DefaultSystemPromptConfig()
	if cfg != nil {
		base = cfg.Normalized()
	} else {
		base = base.Normalized()
	}
	if locale == "" {
		locale = i18n.DefaultLocale
	}

	loader := &Loader{
		cfg:       base,
		cache:     GetCache(),
		staticMap: i18n.PromptSections(locale),
		locale:    locale,
	}

	for name, content := range loader.staticMap {
		loader.cache.Set(name, content)
	}

	return loader
}

// LoadAsSections loads all registered static sections in default order.
func (l *Loader) LoadAsSections(workDir string) []string {
	return l.LoadStaticSections([]string{
		"intro", "system", "doing_tasks", "actions",
		"using_tools", "output_efficiency", "tone_and_style",
		"safety_guidelines", "knowledge_boundaries",
		"todo_write", "delegate_strategy", "glob", "grep", "edit_file",
	})
}

// LoadStaticSections loads named static sections from cache.
func (l *Loader) LoadStaticSections(names []string) []string {
	sections := make([]string, 0, len(names))
	for _, name := range names {
		if content, ok := l.cache.Get(name); ok && strings.TrimSpace(content) != "" {
			sections = append(sections, content)
		}
	}
	return sections
}

// LoadCustom loads merged AGENTS.md context for a work directory.
func (l *Loader) LoadCustom(workDir string) string {
	return l.loadAgentsContext(workDir)
}

// IsCustomPromptAvailable checks if any AGENTS.md source exists for the work directory.
func (l *Loader) IsCustomPromptAvailable(workDir string) bool {
	return len(l.discoverAgentsDocuments(workDir)) > 0
}

// ClearCache clears static section cache and discovered AGENTS context.
func (l *Loader) ClearCache() {
	for name, content := range l.staticMap {
		l.cache.Set(name, content)
	}
	ClearAgentsCache()
}

// GetCacheStats returns cache statistics.
func (l *Loader) GetCacheStats() map[string]bool {
	stats := make(map[string]bool)
	for name := range l.staticMap {
		_, ok := l.cache.Get(name)
		stats[name] = ok
	}
	return stats
}

// Load resolves system prompt for a work directory.
func (l *Loader) Load(workDir string) string {
	return l.LoadCustom(workDir)
}

// LoadWithDynamic loads static sections plus dynamic ones.
func (l *Loader) LoadWithDynamic(workDir string, dynamicSections []string) []string {
	sections := l.LoadAsSections(workDir)

	if len(dynamicSections) > 0 {
		sections = append(sections, DynamicBoundary)
		sections = append(sections, dynamicSections...)
	}

	return sections
}
