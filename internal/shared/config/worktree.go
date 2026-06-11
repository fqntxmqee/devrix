package config

import (
	"os"
	"path/filepath"
	"strings"
)

// WorktreeConfig controls D2-S12 isolated worker directories.
type WorktreeConfig struct {
	Enabled bool   `yaml:"enabled"`
	BaseDir string `yaml:"base_dir"`
}

// DefaultWorktreeConfig returns v2 defaults (disabled until explicitly enabled).
func DefaultWorktreeConfig() WorktreeConfig {
	home, _ := os.UserHomeDir()
	base := filepath.Join(home, ".devrix", "worktrees")
	return WorktreeConfig{
		Enabled: false,
		BaseDir: base,
	}
}

// NormalizeWorktreeConfig applies defaults to zero values.
func NormalizeWorktreeConfig(cfg WorktreeConfig) WorktreeConfig {
	def := DefaultWorktreeConfig()
	if strings.TrimSpace(cfg.BaseDir) == "" {
		cfg.BaseDir = def.BaseDir
	}
	cfg.BaseDir = expandConfigPath(cfg.BaseDir)
	return cfg
}

func expandConfigPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[2:])
	}
	return p
}
