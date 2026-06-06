package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CommunicationConfig holds all communication layer configuration
type CommunicationConfig struct {
	Session    SessionConfig
	Permission PermissionConfig
	CLI        CLIConfig
	Commands   CommandsConfig
}

// SessionConfig holds session-related configuration
type SessionConfig struct {
	IdleTimeout time.Duration // 30 分钟空闲超时
	StorageDir  string       // ~/.devrix/sessions
	MaxSessions int          // 最大并发会话数
}

// PermissionConfig holds permission-related configuration
type PermissionConfig struct {
	DefaultTimeout time.Duration // 60 秒权限超时
	MaxRetries     int          // 最大重试次数
}

// CLIConfig holds CLI-specific configuration
type CLIConfig struct {
	WelcomeMessage string
	Prompt         string
	ANSI           ANSIConfig
}

// ANSIConfig holds ANSI color codes for CLI
type ANSIConfig struct {
	User      string // 蓝色
	Assistant string // 绿色
	Error     string // 红色
	Warning   string // 黄色
	Reset     string
}

// CommandsConfig holds command-related configuration
type CommandsConfig struct {
	Prefix string
	List   []string // ["new", "stop", "help"]
}

// DefaultConfig returns the default configuration
func DefaultConfig() *CommunicationConfig {
	home, _ := os.UserHomeDir()
	return &CommunicationConfig{
		Session: SessionConfig{
			IdleTimeout: 30 * time.Minute,
			StorageDir:  filepath.Join(home, ".devrix", "sessions"),
			MaxSessions: 1000,
		},
		Permission: PermissionConfig{
			DefaultTimeout: 60 * time.Second,
			MaxRetries:     3,
		},
		CLI: CLIConfig{
			WelcomeMessage: `
╔═══════════════════════════════════════════════════════════════╗
║                    Devrix v1.0 - 开发大脑                    ║
║            多智能协同开发助手 (Multi-Agent CLI)               ║
╚═══════════════════════════════════════════════════════════════╝`,
			Prompt: "> ",
			ANSI: ANSIConfig{
				User:      "\x1b[34m",
				Assistant: "\x1b[32m",
				Error:     "\x1b[31m",
				Warning:   "\x1b[33m",
				Reset:     "\x1b[0m",
			},
		},
		Commands: CommandsConfig{
			Prefix: "/",
			List:   []string{"new", "stop", "help"},
		},
	}
}

// ConfigLoader loads configuration from file and environment
type ConfigLoader struct {
	config *CommunicationConfig
}

// NewConfigLoader creates a new config loader with default config
func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{
		config: DefaultConfig(),
	}
}

// Load loads configuration, applying env overrides
func (c *ConfigLoader) Load() (*CommunicationConfig, error) {
	// Apply environment variable overrides
	if dir := os.Getenv("DEVRIX_SESSION_DIR"); dir != "" {
		c.config.Session.StorageDir = dir
	}
	if timeout := os.Getenv("DEVRIX_SESSION_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			c.config.Session.IdleTimeout = d
		}
	}
	if timeout := os.Getenv("DEVRIX_PERMISSION_TIMEOUT"); timeout != "" {
		if d, err := time.ParseDuration(timeout); err == nil {
			c.config.Permission.DefaultTimeout = d
		}
	}

	// Ensure storage directory exists
	if err := os.MkdirAll(c.config.Session.StorageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	return c.config, nil
}

// Get returns the loaded configuration
func (c *ConfigLoader) Get() *CommunicationConfig {
	return c.config
}
