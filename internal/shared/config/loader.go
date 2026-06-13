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
	Tool           ToolConfig            `yaml:"tool"`
	MultiAgent     MultiAgentFileConfig  `yaml:"multi_agent"`
	AgentTools     AgentToolsFileConfig  `yaml:"agent_tools"`
	Orchestration  OrchestrationFileConfig `yaml:"orchestration"`
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

// OrchestrationFileConfig 编排验证配置（文件格式）
type OrchestrationFileConfig struct {
	Enabled                bool     `yaml:"enabled"`
	JudgeProvider          string   `yaml:"judge_provider"`
	JudgeModel             string   `yaml:"judge_model"`
	FallbackJudgeProvider  string   `yaml:"fallback_judge_provider"`
	FallbackJudgeModel     string   `yaml:"fallback_judge_model"`
	PreFilterEnabled       bool     `yaml:"pre_filter_enabled"`
	MinIntervalBetweenJudges string `yaml:"min_interval_between_judges"`
	MaxJudgeCallsPerMinute int      `yaml:"max_judge_calls_per_minute"`
	TrustedToolAllowlist   []string `yaml:"trusted_tool_allowlist"`
	InterventionThreshold  float64  `yaml:"intervention_threshold"`
	AutoIntervene          bool     `yaml:"auto_intervene"`
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

// LoadToolConfig loads tool sandbox settings from a YAML file path.
func LoadToolConfig(path string) (*ToolConfig, error) {
	if path == "" {
		return DefaultToolConfig(), nil
	}
	fileCfg, err := LoadConfigFile(path)
	if err != nil {
		return nil, err
	}
	return BuildToolConfig(fileCfg), nil
}

// LoadMultiAgentConfig loads Layer 4 multi-agent settings from a YAML file path.
func LoadMultiAgentConfig(path string) (*MultiAgentConfig, error) {
	if path == "" {
		return DefaultMultiAgentConfig(), nil
	}
	fileCfg, err := LoadConfigFile(path)
	if err != nil {
		return nil, err
	}
	return BuildMultiAgentConfig(&fileCfg.MultiAgent), nil
}


// LoadAgentToolsConfig loads agent tools configuration from a YAML file path.
func LoadAgentToolsConfig(path string) (*AgentToolsConfig, error) {
	if path == "" {
		return DefaultAgentToolsConfig(), nil
	}
	fileCfg, err := LoadConfigFile(path)
	if err != nil {
		return nil, err
	}
	return BuildAgentToolsConfig(&fileCfg.AgentTools), nil
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
	if f.CompressionRatio != 0 {
		cfg.CompressionRatio = f.CompressionRatio
	}
	if f.SnipRatio != 0 {
		cfg.SnipRatio = f.SnipRatio
	}
	if f.Compression.MaxMessages != 0 {
		cfg.Compression.MaxMessages = f.Compression.MaxMessages
	}
	if f.Compression.KeepTailMessages != 0 {
		cfg.Compression.KeepTailMessages = f.Compression.KeepTailMessages
	}
	if f.Compression.Autocompact.Enabled || f.Compression.Autocompact.Model != "" {
		cfg.Compression.Autocompact = mergeAutocompact(cfg.Compression.Autocompact, f.Compression.Autocompact)
	}
	if f.Compression.Microcompact.KeepRecentToolResults != 0 {
		cfg.Compression.Microcompact.KeepRecentToolResults = f.Compression.Microcompact.KeepRecentToolResults
	}
	if f.MainTranscript.Enabled {
		cfg.MainTranscript.Enabled = true
	}
	if f.MainTranscript.BaseDir != "" {
		cfg.MainTranscript.BaseDir = f.MainTranscript.BaseDir
	}
	if f.TokenCounter.Source != "" {
		cfg.TokenCounter.Source = f.TokenCounter.Source
	}
	if f.Snapshot.BackupDir != "" {
		cfg.Snapshot.BackupDir = f.Snapshot.BackupDir
	}
	cfg.Snapshot.Enabled = f.Snapshot.Enabled || cfg.Snapshot.Enabled
	cfg.Harness = mergeHarnessConfig(cfg.Harness, f.Harness)
	cfg.Preflight = mergePreflightConfig(cfg.Preflight, f.Preflight)
	cfg.Workspace = mergeWorkspaceConfig(cfg.Workspace, f.Workspace)
	if len(f.SystemPrompt.Sources) > 0 {
		cfg.SystemPrompt.Sources = f.SystemPrompt.Sources
	}
	if f.SystemPrompt.Fallback != "" {
		cfg.SystemPrompt.Fallback = f.SystemPrompt.Fallback
	}
	*cfg = mergeContextEngineV6(*cfg, f)
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

func mergeHarnessConfig(base, override HarnessConfig) HarnessConfig {
	out := base
	if override.Enabled {
		out.Enabled = true
		out.Trusted = override.Trusted
	}
	if override.Prefetch.MaxWalkDepth != 0 {
		out.Prefetch.MaxWalkDepth = override.Prefetch.MaxWalkDepth
	}
	out.Prefetch.Enabled = override.Prefetch.Enabled || out.Prefetch.Enabled
	if override.ToolPool.SimpleMode {
		out.ToolPool.SimpleMode = true
	}
	if !override.ToolPool.IncludeMCP {
		out.ToolPool.IncludeMCP = false
	}
	if len(override.ToolPool.DenyNames) > 0 {
		out.ToolPool.DenyNames = override.ToolPool.DenyNames
	}
	if len(override.ToolPool.DenyPrefixes) > 0 {
		out.ToolPool.DenyPrefixes = override.ToolPool.DenyPrefixes
	}
	if override.Routing.Enabled {
		out.Routing.Enabled = true
	}
	if override.Routing.MaxMatches != 0 {
		out.Routing.MaxMatches = override.Routing.MaxMatches
	}
	out.DeferredInit.Enabled = override.DeferredInit.Enabled || out.DeferredInit.Enabled
	if override.Transcript.CompactAfterTurns != 0 {
		out.Transcript.CompactAfterTurns = override.Transcript.CompactAfterTurns
	}
	out.Transcript.Enabled = override.Transcript.Enabled || out.Transcript.Enabled
	out.Transcript.SessionLogEnabled = override.Transcript.SessionLogEnabled || out.Transcript.SessionLogEnabled
	return out
}

func mergePreflightConfig(base, override PreflightConfig) PreflightConfig {
	out := base
	if override.Enabled {
		out.Enabled = true
	}
	if override.Mode != "" {
		out.Mode = override.Mode
	}
	if override.TokenBudget != 0 {
		out.TokenBudget = override.TokenBudget
	}
	if override.WarnRatio != 0 {
		out.WarnRatio = override.WarnRatio
	}
	if override.ToolFilter.Enabled {
		out.ToolFilter.Enabled = true
	}
	if override.ToolFilter.Mode != "" {
		out.ToolFilter.Mode = override.ToolFilter.Mode
	}
	return out
}

func mergeWorkspaceConfig(base, override WorkspacePromptConfig) WorkspacePromptConfig {
	out := base
	if override.MaxContextTokens != 0 {
		out.MaxContextTokens = override.MaxContextTokens
	}
	if override.AgentName != "" {
		out.AgentName = override.AgentName
	}
	if len(override.AdditionalContextFiles) > 0 {
		out.AdditionalContextFiles = override.AdditionalContextFiles
	}
	if !override.EmbedCoreTemplate {
		out.EmbedCoreTemplate = false
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
		// DefaultAuthSecretPlaceholder is dev-only; override via DEVRIX_AUTH_SECRET or auth.secret in production.
		Secret:      DefaultAuthSecretPlaceholder,
		TokenExpiry: 24 * time.Hour,
		Issuer:      "devrix",
	}

	if fileCfg == nil {
		if IsDefaultAuthSecret(authCfg.Secret) {
			fmt.Printf("warning: using default auth secret; set DEVRIX_AUTH_SECRET or auth.secret in config for production\n")
		}
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

	if IsDefaultAuthSecret(authCfg.Secret) {
		fmt.Printf("warning: using default auth secret; set DEVRIX_AUTH_SECRET or auth.secret in config for production\n")
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
