package prompt

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/devrix/devrix/internal/shared/config"
)

// Loader loads system prompts from configured sources.
type Loader struct {
	cfg *config.SystemPromptConfig
}

// NewLoader creates a prompt loader.
func NewLoader(cfg *config.SystemPromptConfig) *Loader {
	if cfg == nil {
		cfg = &config.SystemPromptConfig{
			Fallback: "You are Devrix, a multi-agent development assistant.",
		}
	}
	return &Loader{cfg: cfg}
}

// Load resolves system prompt for a work directory.
func (l *Loader) Load(workDir string) string {
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
	return l.cfg.Fallback
}
