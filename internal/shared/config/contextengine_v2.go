package config

import (
	"fmt"
	"time"
)

const (
	TokenCounterSourceGateway   = "gateway"
	TokenCounterSourceHeuristic = "heuristic"

	maxAutocompactTimeout = 10 * time.Second
)

// CompressionConfig holds compression sub-settings.
type CompressionConfig struct {
	MaxMessages      int               `yaml:"max_messages"`
	KeepTailMessages int               `yaml:"keep_tail_messages"`
	Autocompact      AutocompactConfig `yaml:"autocompact"`
	Microcompact     MicrocompactConfig `yaml:"microcompact"`
}

// MicrocompactConfig controls clearing stale tool-result content in the LLM view.
type MicrocompactConfig struct {
	KeepRecentToolResults int `yaml:"keep_recent_tool_results"`
}

// DefaultMicrocompactConfig returns microcompact defaults.
func DefaultMicrocompactConfig() MicrocompactConfig {
	return MicrocompactConfig{KeepRecentToolResults: 3}
}

// DefaultCompressionConfig returns compression defaults.
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		MaxMessages:      50,
		KeepTailMessages: 40,
		Autocompact:      DefaultAutocompactConfig(),
		Microcompact:     DefaultMicrocompactConfig(),
	}
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
