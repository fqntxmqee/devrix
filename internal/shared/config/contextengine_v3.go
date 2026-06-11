package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	maxPlanMilestones   = 10
	maxPlanTimeout      = 15 * time.Second
	maxLongTermTopics   = 20
	defaultPlanModel    = "deepseek-v4"
	defaultLongTermDBPath = "~/.devrix/memory.db"
)

var milestoneIDPattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// PlanConfig holds PEV Plan phase settings (V3).
// Aligned with Claude Code's Plan Mode behavior.
type PlanConfig struct {
	Enabled          bool          `yaml:"enabled"`
	AutoDetect       bool          `yaml:"auto_detect"`        // 自动检测复杂任务
	MinCharsForPlan  int           `yaml:"min_chars_for_plan"` // 触发规划的最短消息长度
	Model            string        `yaml:"model"`
	MaxMilestones    int           `yaml:"max_milestones"`
	Timeout          time.Duration `yaml:"timeout"`
	OnMilestoneFail  string        `yaml:"on_milestone_fail"` // fail_fast (only supported V3)
}

// LongTermConfig holds cross-session memory settings (V3).
type LongTermConfig struct {
	Enabled          bool     `yaml:"enabled"`
	DBPath          string   `yaml:"db_path"`
	AutoStore       bool     `yaml:"auto_store"`
	Topics          []string `yaml:"topics"`
	RecallMaxEntries int     `yaml:"recall_max_entries"`
	RecallMaxTokens  int     `yaml:"recall_max_tokens"`
}

// DefaultPlanConfig returns V3 plan defaults.
// Note: plan.enabled defaults to false for safe rollout.
func DefaultPlanConfig() PlanConfig {
	return PlanConfig{
		Enabled:         false, // 显式启用，Claude Code 风格
		AutoDetect:      false, // 默认关闭，需要显式 /plan 命令
		MinCharsForPlan: 200,
		Model:           defaultPlanModel,
		MaxMilestones:   maxPlanMilestones,
		Timeout:         maxPlanTimeout,
		OnMilestoneFail: "fail_fast",
	}
}

// EnabledPlanConfig returns plan config with plan mode enabled.
func EnabledPlanConfig() PlanConfig {
	cfg := DefaultPlanConfig()
	cfg.Enabled = true
	return cfg
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

// ValidateContextEngineV3Config validates V3 plan and longterm settings.
func ValidateContextEngineV3Config(cfg *ContextEngineConfig) error {
	if cfg == nil {
		return nil
	}
	if err := validatePlan(cfg.Plan); err != nil {
		return err
	}
	if err := validateLongTerm(cfg.LongTerm); err != nil {
		return err
	}
	return nil
}

func validatePlan(cfg PlanConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Model == "" {
		return fmt.Errorf("context_engine.plan.model: required when enabled")
	}
	if cfg.Timeout <= 0 {
		return fmt.Errorf("context_engine.plan.timeout: required when enabled")
	}
	if cfg.Timeout > maxPlanTimeout {
		return fmt.Errorf("context_engine.plan.timeout: must be <= %s", maxPlanTimeout)
	}
	maxMS := cfg.MaxMilestones
	if maxMS <= 0 {
		maxMS = maxPlanMilestones
	}
	if maxMS > maxPlanMilestones {
		return fmt.Errorf("context_engine.plan.max_milestones: must be <= %d", maxPlanMilestones)
	}
	if cfg.MinCharsForPlan < 0 {
		return fmt.Errorf("context_engine.plan.min_chars_for_plan: must be >= 0")
	}
	if cfg.OnMilestoneFail != "" && cfg.OnMilestoneFail != "fail_fast" {
		return fmt.Errorf("context_engine.plan.on_milestone_fail: only fail_fast supported in V3")
	}
	return nil
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
