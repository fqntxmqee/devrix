package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/devrix/devrix/internal/shared/types"
	"gopkg.in/yaml.v3"
)

// ConfigFile represents the YAML configuration structure
type ConfigFile struct {
	App         AppConfig         `yaml:"app"`
	Session     SessionConfig     `yaml:"session"`
	Auth        AuthFileConfig    `yaml:"auth"`
	Permission  PermissionConfig  `yaml:"permission"`
	Connection  ConnectionConfig  `yaml:"connection"`
	RateLimit   RateLimitConfig  `yaml:"rate_limit"`
	CLI         CLIConfig        `yaml:"cli"`
	Commands    CommandsConfig    `yaml:"commands"`
	Feishu      FeishuFileConfig `yaml:"feishu"`
	Instance    InstanceConfig    `yaml:"instance"`
	Logging     LoggingConfig     `yaml:"logging"`
	Metrics        MetricsConfig        `yaml:"metrics"`
	ContextEngine  ContextEngineConfig  `yaml:"context_engine"`
	LLMGateway     LLMGatewayFileConfig `yaml:"llm_gateway"`
}

// AppConfig 应用配置
type AppConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Mode    string `yaml:"mode"` // cli | server | daemon
}

// AuthFileConfig 认证配置（文件格式）
type AuthFileConfig struct {
	Secret      string        `yaml:"secret"`
	TokenExpiry time.Duration `yaml:"token_expiry"`
	Issuer      string        `yaml:"issuer"`
}

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
	HeartbeatTimeout  time.Duration `yaml:"heartbeat_timeout"`
}

// FeishuFileConfig 飞书配置（文件格式）
type FeishuFileConfig struct {
	Enabled      bool   `yaml:"enabled"`
	AppID        string `yaml:"app_id"`
	AppSecret    string `yaml:"app_secret"`
	BotName      string `yaml:"bot_name"`
	Domain       string `yaml:"domain"`
	EncryptKey   string `yaml:"encrypt_key"`
	CallbackPath string `yaml:"callback_path"`
	Port         string `yaml:"port"`
	UseWebhook   bool   `yaml:"use_webhook"`
}

// InstanceConfig 实例配置
type InstanceConfig struct {
	ID             string `yaml:"id"`
	Name           string `yaml:"name"`
	Address        string `yaml:"address"`
	Port           int    `yaml:"port"`
	ClusterEnabled bool   `yaml:"cluster_enabled"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level  string `yaml:"level"`  // debug | info | warn | error
	Format string `yaml:"format"` // json | text
	Output string `yaml:"output"` // stdout | stderr | file
}

// MetricsConfig Metrics 配置
type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Path    string `yaml:"path"`
}

// LoadConfigFile loads configuration from a YAML file
func LoadConfigFile(path string) (*ConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg ConfigFile
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// LoadLLMGatewayConfig loads Layer 3 config from a YAML file path.
func LoadLLMGatewayConfig(path string) (*LLMGatewayConfig, error) {
	if path == "" {
		return DefaultLLMGatewayConfig(), nil
	}
	fileCfg, err := LoadConfigFile(path)
	if err != nil {
		return nil, err
	}
	return BuildLLMGatewayConfig(&fileCfg.LLMGateway), nil
}

// LoadConfig loads configuration with fallback to defaults
func LoadConfig(path string) (*CommunicationConfig, *types.AuthConfig, *RateLimitConfig, *ContextEngineConfig, error) {
	// Try to load from file
	var fileCfg *ConfigFile
	if path != "" {
		var err error
		fileCfg, err = LoadConfigFile(path)
		if err != nil {
			// Log warning but continue with defaults
			fmt.Printf("warning: failed to load config file: %v\n", err)
		}
	}

	// Build configs from file or defaults
	commCfg := buildCommunicationConfig(fileCfg)
	authCfg := buildAuthConfig(fileCfg)
	rateCfg := buildRateLimitConfig(fileCfg)
	ctxCfg := buildContextEngineConfig(fileCfg)

	return commCfg, authCfg, rateCfg, ctxCfg, nil
}

func buildContextEngineConfig(fileCfg *ConfigFile) *ContextEngineConfig {
	cfg := DefaultContextEngineConfig()
	if fileCfg == nil {
		return cfg
	}
	f := fileCfg.ContextEngine
	if f.MaxContextTokens != 0 {
		cfg.MaxContextTokens = f.MaxContextTokens
	}
	if f.ReservedOutput != 0 {
		cfg.ReservedOutput = f.ReservedOutput
	}
	if f.ToolResultBudget != 0 {
		cfg.ToolResultBudget = f.ToolResultBudget
	}
	cfg.CompressionEnabled = f.CompressionEnabled || cfg.CompressionEnabled
	if f.Compression.Autocompact.Enabled || f.Compression.Autocompact.Model != "" {
		cfg.Compression.Autocompact = mergeAutocompact(cfg.Compression.Autocompact, f.Compression.Autocompact)
	}
	if f.TokenCounter.Source != "" {
		cfg.TokenCounter.Source = f.TokenCounter.Source
	}
	if f.PEV.MaxIterations != 0 {
		cfg.PEV.MaxIterations = f.PEV.MaxIterations
	}
	if f.PEV.VerifyMode != "" {
		cfg.PEV.VerifyMode = f.PEV.VerifyMode
	}
	if f.PEV.VerifyPolicy != "" {
		cfg.PEV.VerifyPolicy = f.PEV.VerifyPolicy
	}
	if len(f.PEV.VerifyCommands) > 0 {
		cfg.PEV.VerifyCommands = f.PEV.VerifyCommands
	}
	if f.Snapshot.BackupDir != "" {
		cfg.Snapshot.BackupDir = f.Snapshot.BackupDir
	}
	cfg.Snapshot.Enabled = f.Snapshot.Enabled || cfg.Snapshot.Enabled
	if len(f.SystemPrompt.Sources) > 0 {
		cfg.SystemPrompt.Sources = f.SystemPrompt.Sources
	}
	if f.SystemPrompt.Fallback != "" {
		cfg.SystemPrompt.Fallback = f.SystemPrompt.Fallback
	}
	if err := ValidateContextEngineConfig(cfg); err != nil {
		fmt.Printf("warning: context engine config invalid, using defaults where needed: %v\n", err)
	}
	return cfg
}

func mergeAutocompact(base, override AutocompactConfig) AutocompactConfig {
	out := base
	if override.Enabled {
		out.Enabled = true
	}
	if override.Model != "" {
		out.Model = override.Model
	}
	if override.SummaryMaxTokens > 0 {
		out.SummaryMaxTokens = override.SummaryMaxTokens
	}
	if override.MinMessagesForSummary > 0 {
		out.MinMessagesForSummary = override.MinMessagesForSummary
	}
	if override.PreserveHeadTurns > 0 {
		out.PreserveHeadTurns = override.PreserveHeadTurns
	}
	if override.PreserveTailTurns > 0 {
		out.PreserveTailTurns = override.PreserveTailTurns
	}
	if override.Timeout > 0 {
		out.Timeout = override.Timeout
	}
	return out
}

// FindConfigFile looks for config file in standard locations
func FindConfigFile() string {
	// Check standard locations
	locations := []string{
		"devrix.yaml",
		".devrix.yaml",
		".devrix/config.yaml",
		"~/.devrix/config.yaml",
		"/etc/devrix/config.yaml",
	}

	// Check DEVRIX_CONFIG environment variable
	if envPath := os.Getenv("DEVRIX_CONFIG"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
	}

	// Search standard locations
	for _, loc := range locations {
		path := expandPath(loc)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[1:])
	}
	return os.ExpandEnv(path)
}

func buildCommunicationConfig(fileCfg *ConfigFile) *CommunicationConfig {
	cfg := DefaultConfig()

	if fileCfg == nil {
		return cfg
	}

	// Override from file
	if fileCfg.Session.IdleTimeout != 0 {
		cfg.Session.IdleTimeout = fileCfg.Session.IdleTimeout
	}
	if fileCfg.Session.StorageDir != "" {
		cfg.Session.StorageDir = fileCfg.Session.StorageDir
	}
	if fileCfg.Session.MaxSessions != 0 {
		cfg.Session.MaxSessions = fileCfg.Session.MaxSessions
	}

	if fileCfg.Permission.DefaultTimeout != 0 {
		cfg.Permission.DefaultTimeout = fileCfg.Permission.DefaultTimeout
	}
	if fileCfg.Permission.MaxRetries != 0 {
		cfg.Permission.MaxRetries = fileCfg.Permission.MaxRetries
	}

	if fileCfg.CLI.WelcomeMessage != "" {
		cfg.CLI.WelcomeMessage = fileCfg.CLI.WelcomeMessage
	}
	if fileCfg.CLI.Prompt != "" {
		cfg.CLI.Prompt = fileCfg.CLI.Prompt
	}

	if fileCfg.Commands.Prefix != "" {
		cfg.Commands.Prefix = fileCfg.Commands.Prefix
	}
	if len(fileCfg.Commands.List) > 0 {
		cfg.Commands.List = fileCfg.Commands.List
	}

	return cfg
}

func buildAuthConfig(fileCfg *ConfigFile) *types.AuthConfig {
	authCfg := &types.AuthConfig{
		Secret:      "devrix-secret-change-me",
		TokenExpiry: 24 * time.Hour,
		Issuer:      "devrix",
	}

	if fileCfg == nil {
		return authCfg
	}

	// Environment variable overrides
	if secret := os.Getenv("DEVRIX_AUTH_SECRET"); secret != "" {
		authCfg.Secret = secret
	} else if fileCfg.Auth.Secret != "" {
		authCfg.Secret = fileCfg.Auth.Secret
	}

	if expiryStr := os.Getenv("DEVRIX_AUTH_TOKEN_EXPIRY"); expiryStr != "" {
		if d, err := time.ParseDuration(expiryStr); err == nil {
			authCfg.TokenExpiry = d
		}
	} else if fileCfg.Auth.TokenExpiry != 0 {
		authCfg.TokenExpiry = fileCfg.Auth.TokenExpiry
	}

	if issuer := os.Getenv("DEVRIX_AUTH_ISSUER"); issuer != "" {
		authCfg.Issuer = issuer
	} else if fileCfg.Auth.Issuer != "" {
		authCfg.Issuer = fileCfg.Auth.Issuer
	}

	return authCfg
}

func buildRateLimitConfig(fileCfg *ConfigFile) *RateLimitConfig {
	rateCfg := &RateLimitConfig{
		RequestsPerMinute: 100,
		BurstSize:        10,
		Enabled:          true,
	}

	if fileCfg == nil {
		return rateCfg
	}

	// Environment variable overrides
	if rpm := os.Getenv("DEVRIX_RATE_LIMIT_RPM"); rpm != "" {
		if val, err := parseInt(rpm); err == nil && val > 0 {
			rateCfg.RequestsPerMinute = val
		}
	} else if fileCfg.RateLimit.RequestsPerMinute != 0 {
		rateCfg.RequestsPerMinute = fileCfg.RateLimit.RequestsPerMinute
	}

	if burst := os.Getenv("DEVRIX_RATE_LIMIT_BURST"); burst != "" {
		if val, err := parseInt(burst); err == nil && val > 0 {
			rateCfg.BurstSize = val
		}
	} else if fileCfg.RateLimit.BurstSize != 0 {
		rateCfg.BurstSize = fileCfg.RateLimit.BurstSize
	}

	if enabled := os.Getenv("DEVRIX_RATE_LIMIT_ENABLED"); enabled != "" {
		rateCfg.Enabled = enabled == "true" || enabled == "1"
	} else {
		rateCfg.Enabled = fileCfg.RateLimit.Enabled
	}

	return rateCfg
}
