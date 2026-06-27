package config

// defaultStaticPromptSections lists all static system prompt sections in load order.
var defaultStaticPromptSections = []string{
	"intro",
	"system",
	"doing_tasks",
	"actions",
	"using_tools",
	"output_efficiency",
	"tone_and_style",
	"safety_guidelines",
	"knowledge_boundaries",
	"todo_write",
	"delegate_strategy",
	"glob",
	"grep",
	"edit_file",
}

// PromptConfig holds prompt-related configuration.
type PromptConfig struct {
	// UseSections enables the section-based prompt system.
	UseSections bool `yaml:"use_sections"`

	// StaticSections lists which static sections to include.
	// If empty, all sections are included.
	StaticSections []string `yaml:"static_sections"`

	// EnableDynamicBoundary adds the dynamic boundary marker for prompt caching.
	EnableDynamicBoundary bool `yaml:"enable_dynamic_boundary"`

	// CacheTTLSeconds is the cache TTL for sections in seconds.
	// 0 means no expiration (session-level cache).
	CacheTTLSeconds int `yaml:"cache_ttl_seconds"`

	// DynamicSections lists dynamic section names to compute.
	DynamicSections []string `yaml:"dynamic_sections"`
}

// DefaultPromptConfig returns default prompt configuration.
func DefaultPromptConfig() *PromptConfig {
	return &PromptConfig{
		UseSections:           true,
		EnableDynamicBoundary: true,
		CacheTTLSeconds:       0,
		StaticSections:        append([]string(nil), defaultStaticPromptSections...),
		DynamicSections: []string{
			"git_status",
			"env_info",
			"workspace_context",
		},
	}
}

// GetStaticSections returns the list of static sections to include.
func (p *PromptConfig) GetStaticSections() []string {
	if len(p.StaticSections) == 0 {
		return append([]string(nil), defaultStaticPromptSections...)
	}
	return p.StaticSections
}
