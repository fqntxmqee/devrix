package config

import (
	"os"
	"path/filepath"
	"strings"
)

// SandboxConfig controls D2-S18 isolated worker directories (filesystem sandbox).
type SandboxConfig struct {
	Enabled bool   `yaml:"enabled"`
	BaseDir string `yaml:"base_dir"`
}

func DefaultSandboxConfig() SandboxConfig {
	home, _ := os.UserHomeDir()
	return SandboxConfig{
		Enabled: false,
		BaseDir: filepath.Join(home, ".devrix", "sandboxes"),
	}
}

func NormalizeSandboxConfig(cfg SandboxConfig) SandboxConfig {
	def := DefaultSandboxConfig()
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
