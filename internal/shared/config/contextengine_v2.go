package config

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	TokenCounterSourceGateway   = "gateway"
	TokenCounterSourceHeuristic = "heuristic"

	VerifyModeNone     = "none"
	VerifyModeBasic    = "basic"
	VerifyModeCommands = "commands"

	VerifyPolicyAllPass = "all_pass"
	VerifyPolicyAnyPass = "any_pass"

	maxVerifyCommands     = 10
	maxAutocompactTimeout = 10 * time.Second
)

var verifyCommandNamePattern = regexp.MustCompile(`^[a-z0-9_-]+$`)

// CompressionConfig holds compression sub-settings.
type CompressionConfig struct {
	Autocompact AutocompactConfig `yaml:"autocompact"`
}

// AutocompactConfig holds step-6 autocompact settings.
type AutocompactConfig struct {
	Enabled               bool          `yaml:"enabled"`
	Model                 string        `yaml:"model"`
	SummaryMaxTokens      int           `yaml:"summary_max_tokens"`
	MinMessagesForSummary int           `yaml:"min_messages_for_summary"`
	PreserveHeadTurns     int           `yaml:"preserve_head_turns"`
	PreserveTailTurns     int           `yaml:"preserve_tail_turns"`
	Timeout               time.Duration `yaml:"timeout"`
}

// TokenCounterConfig selects the token counting implementation.
type TokenCounterConfig struct {
	Source string `yaml:"source"`
}

// VerifyCommandConfig is a whitelisted verify command.
type VerifyCommandConfig struct {
	Name       string        `yaml:"name"`
	Executable string        `yaml:"executable"`
	Args       []string      `yaml:"args"`
	Timeout    time.Duration `yaml:"timeout"`
}

// DefaultAutocompactConfig returns V2 autocompact defaults (disabled until explicitly enabled).
func DefaultAutocompactConfig() AutocompactConfig {
	return AutocompactConfig{
		Enabled:               false,
		Model:                 "deepseek-v4-flash",
		SummaryMaxTokens:      512,
		MinMessagesForSummary: 8,
		PreserveHeadTurns:     2,
		PreserveTailTurns:     2,
		Timeout:               10 * time.Second,
	}
}

// ValidateContextEngineConfig validates V2 configuration constraints.
func ValidateContextEngineConfig(cfg *ContextEngineConfig) error {
	if cfg == nil {
		return nil
	}
	if err := validateAutocompact(cfg.Compression.Autocompact); err != nil {
		return err
	}
	if err := validateVerifyCommands(cfg.PEV); err != nil {
		return err
	}
	if src := cfg.TokenCounter.Source; src != "" &&
		src != TokenCounterSourceGateway && src != TokenCounterSourceHeuristic {
		return fmt.Errorf("context_engine.token_counter.source: invalid value %q", src)
	}
	if err := ValidateHarnessConfig(cfg.Harness, cfg.Preflight); err != nil {
		return err
	}
	return ValidateContextEngineV3Config(cfg)
}

func validateAutocompact(cfg AutocompactConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Timeout <= 0 {
		return fmt.Errorf("context_engine.compression.autocompact.timeout: required when enabled")
	}
	if cfg.Timeout > maxAutocompactTimeout {
		return fmt.Errorf("context_engine.compression.autocompact.timeout: must be <= %s", maxAutocompactTimeout)
	}
	if cfg.Model == "" {
		return fmt.Errorf("context_engine.compression.autocompact.model: required when enabled")
	}
	return nil
}

func validateVerifyCommands(pev PEVConfig) error {
	if pev.VerifyMode != VerifyModeCommands {
		return nil
	}
	if len(pev.VerifyCommands) > maxVerifyCommands {
		return fmt.Errorf("context_engine.pev.verify_commands: at most %d commands allowed", maxVerifyCommands)
	}
	for _, cmd := range pev.VerifyCommands {
		if !verifyCommandNamePattern.MatchString(cmd.Name) {
			return fmt.Errorf("context_engine.pev.verify_commands: name %q must match [a-z0-9_-]+", cmd.Name)
		}
		if err := validateNoShellMeta(cmd.Executable); err != nil {
			return fmt.Errorf("context_engine.pev.verify_commands[%s].executable: %w", cmd.Name, err)
		}
		for i, arg := range cmd.Args {
			if err := validateNoShellMeta(arg); err != nil {
				return fmt.Errorf("context_engine.pev.verify_commands[%s].args[%d]: %w", cmd.Name, i, err)
			}
		}
	}
	if pev.VerifyPolicy != "" && pev.VerifyPolicy != VerifyPolicyAllPass && pev.VerifyPolicy != VerifyPolicyAnyPass {
		return fmt.Errorf("context_engine.pev.verify_policy: invalid value %q", pev.VerifyPolicy)
	}
	return nil
}

func validateNoShellMeta(s string) error {
	if strings.ContainsAny(s, ";|&$`") {
		return fmt.Errorf("shell metacharacters not allowed")
	}
	return nil
}
