package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/devrix/devrix/internal/shared/config"
)

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
	cfg       *config.SystemPromptConfig
	cache     *Cache
	staticMap map[string]string
}

// NewLoader creates a prompt loader.
func NewLoader(cfg *config.SystemPromptConfig) *Loader {
	if cfg == nil {
		cfg = &config.SystemPromptConfig{
			Sources:  []string{"AGENTS.md", ".devrix/AGENTS.md"},
			Fallback: "You are Devrix, a multi-agent development assistant.",
		}
	}

	loader := &Loader{
		cfg:       cfg,
		cache:     GetCache(),
		staticMap: make(map[string]string),
	}

	// Register static sections
	loader.staticMap = map[string]string{
		"intro":            sectionIntro,
		"system":           sectionSystem,
		"doing_tasks":       sectionDoingTasks,
		"actions":           sectionActions,
		"using_tools":       sectionUsingTools,
		"output_efficiency":  sectionOutputEfficiency,
		"tone_and_style":    sectionToneAndStyle,
	}

	// Pre-populate cache with static content
	for name, content := range loader.staticMap {
		loader.cache.Set(name, content)
	}

	return loader
}

// LoadAsSections loads prompt and returns it as a list of sections.
func (l *Loader) LoadAsSections(workDir string) []string {
	sections := make([]string, 0, len(l.staticMap))

	for _, name := range []string{
		"intro", "system", "doing_tasks", "actions",
		"using_tools", "output_efficiency", "tone_and_style",
	} {
		if content, ok := l.cache.Get(name); ok {
			sections = append(sections, content)
		}
	}

	return sections
}

// LoadCustom loads custom prompt from workdir.
func (l *Loader) LoadCustom(workDir string) string {
	for _, src := range l.cfg.Sources {
		path := src
		if !filepath.IsAbs(src) && workDir != "" {
			path = filepath.Join(workDir, src)
		}
		data, err := os.ReadFile(path)
		if err == nil && len(strings.TrimSpace(string(data))) > 0 {
			return strings.TrimSpace(string(data))
		}
	}
	return ""
}

// IsCustomPromptAvailable checks if a custom prompt exists.
func (l *Loader) IsCustomPromptAvailable(workDir string) bool {
	for _, src := range l.cfg.Sources {
		path := src
		if !filepath.IsAbs(src) && workDir != "" {
			path = filepath.Join(workDir, src)
		}
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// ClearCache clears the prompt cache.
func (l *Loader) ClearCache() {
	// Re-populate static content
	for name, content := range l.staticMap {
		l.cache.Set(name, content)
	}
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

// Static section contents
const (
	DynamicBoundary = "<!-- DYNAMIC_CONTENT_BOUNDARY -->"

	sectionIntro = `You are an interactive agent that helps users with software engineering tasks. 
Use the instructions below and the tools available to you to assist the user.

IMPORTANT: You must NEVER generate or guess URLs for the user unless you are 
confident that the URLs are for helping the user with programming.`

	sectionSystem = `# System

- All text you output outside of tool use is displayed to the user.
- Tools are executed in a user-selected permission mode.
- Tool results may include <system-reminder> or other tags.
- The system will automatically compress prior messages as context limits approach.`

	sectionDoingTasks = `# Doing tasks

- Don't add features, refactor code, or make "improvements" beyond what was asked.
- Don't add error handling for scenarios that can't happen.
- Don't create helpers, utilities, or abstractions for one-time operations.
- Default to writing no comments. Only add when the WHY is non-obvious.
- Before reporting task complete, verify it actually works.`

	sectionActions = `# Executing actions with care

Carefully consider the reversibility and blast radius of actions.

Examples of risky actions that warrant user confirmation:
- Destructive: deleting files/branches, rm -rf, dropping tables
- Hard-to-reverse: force-push, git reset --hard
- Shared state: pushing code, PRs, sending messages

When in doubt, ask before acting.`

	sectionUsingTools = `# Using your tools

- Use dedicated read tool instead of cat, head, tail
- Use dedicated edit tool instead of sed or awk
- Use dedicated glob tool instead of find
- Call multiple independent tools in parallel when possible.

CRITICAL: Do NOT use bash when a relevant dedicated tool is provided.`

	sectionOutputEfficiency = `# Output efficiency

IMPORTANT: Go straight to the point. Try the simplest approach first.

Keep your text output brief and direct:
- Lead with the answer or action, not the reasoning
- Skip filler words and unnecessary transitions

If you can say it in one sentence, don't use three.`

	sectionToneAndStyle = `# Tone and style

- Only use emojis if the user explicitly requests it.
- Your responses should be short and concise.
- Include file_path:line_number for code references.
- Be precise and factual.`
)

// Load resolves system prompt for a work directory.
func (l *Loader) Load(workDir string) string {
	return l.LoadCustom(workDir)
}

// LoadWithDynamic loads static sections plus dynamic ones.
func (l *Loader) LoadWithDynamic(workDir string, dynamicSections []string) []string {
	sections := l.LoadAsSections(workDir)

	// Add dynamic boundary marker if there are dynamic sections
	if len(dynamicSections) > 0 {
		sections = append(sections, DynamicBoundary)
		sections = append(sections, dynamicSections...)
	}

	return sections
}
