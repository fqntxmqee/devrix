package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Session.IdleTimeout != 30*60*1e9 { // 30 minutes in nanoseconds
		t.Errorf("expected IdleTimeout of 30 minutes, got %v", cfg.Session.IdleTimeout)
	}

	if cfg.Session.MaxSessions != 1000 {
		t.Errorf("expected MaxSessions of 1000, got %d", cfg.Session.MaxSessions)
	}

	if cfg.Permission.DefaultTimeout != 60*1e9 { // 60 seconds
		t.Errorf("expected DefaultTimeout of 60 seconds, got %v", cfg.Permission.DefaultTimeout)
	}

	if cfg.Permission.MaxRetries != 3 {
		t.Errorf("expected MaxRetries of 3, got %d", cfg.Permission.MaxRetries)
	}

	if cfg.Commands.Prefix != "/" {
		t.Errorf("expected Commands.Prefix '/', got '%s'", cfg.Commands.Prefix)
	}
}

func TestConfigLoader_Load(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	loader := NewConfigLoader()
	loader.config.Session.StorageDir = tmpDir

	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(cfg.Session.StorageDir); os.IsNotExist(err) {
		t.Error("expected session directory to be created")
	}
}

func TestConfigLoader_LoadWithEnv(t *testing.T) {
	// Set environment variables
	os.Setenv("DEVRIX_SESSION_DIR", "/tmp/test_devrix")
	os.Setenv("DEVRIX_SESSION_TIMEOUT", "1h")
	os.Setenv("DEVRIX_PERMISSION_TIMEOUT", "120s")
	defer func() {
		os.Unsetenv("DEVRIX_SESSION_DIR")
		os.Unsetenv("DEVRIX_SESSION_TIMEOUT")
		os.Unsetenv("DEVRIX_PERMISSION_TIMEOUT")
	}()

	loader := NewConfigLoader()
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Session.StorageDir != "/tmp/test_devrix" {
		t.Errorf("expected StorageDir '/tmp/test_devrix', got '%s'", cfg.Session.StorageDir)
	}

	if cfg.Session.IdleTimeout != 60*60*1e9 { // 1 hour
		t.Errorf("expected IdleTimeout of 1 hour, got %v", cfg.Session.IdleTimeout)
	}

	if cfg.Permission.DefaultTimeout != 120*1e9 { // 120 seconds
		t.Errorf("expected DefaultTimeout of 120 seconds, got %v", cfg.Permission.DefaultTimeout)
	}
}
