package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	maxLongTermTopics     = 20
	defaultLongTermDBPath = "~/.devrix/memory.db"
)

var milestoneIDPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// LongTermConfig holds cross-session memory settings (V3).
type LongTermConfig struct {
	Enabled          bool     `yaml:"enabled"`
	DBPath          string   `yaml:"db_path"`
	AutoStore       bool     `yaml:"auto_store"`
	Topics          []string `yaml:"topics"`
	RecallMaxEntries int     `yaml:"recall_max_entries"`
	RecallMaxTokens  int     `yaml:"recall_max_tokens"`
}

// DefaultLongTermConfig returns V3 long-term memory defaults.
func DefaultLongTermConfig() LongTermConfig {
	return LongTermConfig{
		Enabled:          true,
		DBPath:           defaultLongTermDBPath,
		AutoStore:        false,
		Topics:           []string{"architecture", "decisions", "bugs"},
		RecallMaxEntries: 5,
		RecallMaxTokens:  2000,
	}
}

// ValidateContextEngineV3Config validates V3 longterm settings.
func ValidateContextEngineV3Config(cfg *ContextEngineConfig) error {
	if cfg == nil {
		return nil
	}
	return validateLongTerm(cfg.LongTerm)
}

func validateLongTerm(cfg LongTermConfig) error {
	if !cfg.Enabled {
		return nil
	}
	path := cfg.DBPath
	if path == "" {
		path = defaultLongTermDBPath
	}
	if err := validateLongTermDBPath(path); err != nil {
		return fmt.Errorf("context_engine.longterm.db_path: %w", err)
	}
	if len(cfg.Topics) > maxLongTermTopics {
		return fmt.Errorf("context_engine.longterm.topics: at most %d topics allowed", maxLongTermTopics)
	}
	for _, topic := range cfg.Topics {
		if !milestoneIDPattern.MatchString(topic) {
			return fmt.Errorf("context_engine.longterm.topics: %q must match [a-z0-9_-]+", topic)
		}
	}
	if cfg.RecallMaxEntries < 0 {
		return fmt.Errorf("context_engine.longterm.recall_max_entries: must be >= 0")
	}
	if cfg.RecallMaxTokens < 0 {
		return fmt.Errorf("context_engine.longterm.recall_max_tokens: must be >= 0")
	}
	return nil
}

func validateLongTermDBPath(path string) error {
	expanded := path
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		expanded = filepath.Join(home, path[2:])
	}
	dir := filepath.Dir(expanded)
	if dir == "" || dir == "." {
		return fmt.Errorf("invalid path %q", path)
	}
	return nil
}

// ResolvedLongTermDBPath expands ~ in db_path.
func ResolvedLongTermDBPath(cfg LongTermConfig) (string, error) {
	path := cfg.DBPath
	if path == "" {
		path = defaultLongTermDBPath
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}
