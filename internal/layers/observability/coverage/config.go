package coverage

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config holds coverage reporter configuration
type Config struct {
	Enabled  bool          `yaml:"enabled"`
	Dir      string        `yaml:"dir"`
	Interval time.Duration `yaml:"interval"`
}

// DefaultConfig returns the default coverage configuration
func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		Enabled:  true,
		Dir:      filepath.Join(home, ".devrix", "coverage"),
		Interval: time.Hour,
	}
}

// Validate validates the configuration
func (c Config) Validate() error {
	if c.Interval <= 0 {
		return fmt.Errorf("interval must be positive")
	}
	return nil
}
