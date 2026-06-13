// Package hotreload provides hot configuration reload functionality
package hotreload

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/fukaiyi/devrix/evolution/config"
	"github.com/fukaiyi/devrix/internal/shared/errors"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Log struct {
		Level string `yaml:"level"`
	} `yaml:"log"`
	LLM struct {
		Provider string `yaml:"provider"`
		Model    string `yaml:"model"`
	} `yaml:"llm"`
}

// Options contains options for the Service
type Options struct {
	ConfigPath  string
	Debounce    time.Duration
	MaxSubscribers int
	Logger      Logger
}

// Logger is the interface for logging
type Logger interface {
	Info(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
}

// defaultLogger is a simple default logger
type defaultLogger struct{}

func (d *defaultLogger) Info(msg string, args ...any)  { println("[INFO]", msg) }
func (d *defaultLogger) Error(msg string, args ...any) { println("[ERROR]", msg) }
func (d *defaultLogger) Debug(msg string, args ...any) { println("[DEBUG]", msg) }

// Service provides hot configuration reload functionality
type Service struct {
	path       string
	debounce   time.Duration
	maxSubs    int
	logger     Logger
	watcher    config.Watcher
	notifier   *config.Notifier
	config     *Config
	mu         sync.RWMutex
	debounceMu sync.Mutex
	timer      *time.Timer
	ctx        context.Context
	cancel     context.CancelFunc
	started    bool
}

// NewService creates a new hot reload service
func NewService(opts Options) *Service {
	if opts.Debounce <= 0 {
		opts.Debounce = config.DefaultDebounce
	}
	if opts.MaxSubscribers <= 0 {
		opts.MaxSubscribers = config.DefaultMaxSubscribers
	}
	if opts.Logger == nil {
		opts.Logger = &defaultLogger{}
	}
	if opts.ConfigPath == "" {
		opts.ConfigPath = "devrix.yaml"
	}

	s := &Service{
		path:     opts.ConfigPath,
		debounce: opts.Debounce,
		maxSubs:  opts.MaxSubscribers,
		logger:   opts.Logger,
		notifier: config.NewNotifier(),
	}

	return s
}

// Start starts the hot reload service
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return config.ErrAlreadyStarted
	}

	// Load initial config
	if err := s.loadConfig(); err != nil {
		return err
	}

	// Create watcher
	s.watcher = config.NewConfigWatcher(s.path)

	// Create context for watcher
	s.ctx, s.cancel = context.WithCancel(ctx)

	// Start watcher
	if err := s.watcher.Start(s.ctx); err != nil {
		return err
	}

	s.started = true

	// Start event loop
	go s.run()

	return nil
}

// run processes file change events
func (s *Service) run() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case event, ok := <-s.watcher.Events():
			if !ok {
				return
			}
			s.onEvent(event)
		}
	}
}

// onEvent handles file change events with debouncing
func (s *Service) onEvent(event config.Event) {
	if event.Op&(config.WRITE|config.CREATE) == 0 {
		return
	}

	s.debounceMu.Lock()
	defer s.debounceMu.Unlock()

	if s.timer != nil {
		s.timer.Stop()
	}

	s.timer = time.AfterFunc(s.debounce, func() {
		s.reload()
	})
}

// reload reloads the configuration
func (s *Service) reload() error {
	s.mu.Lock()
	oldCfg := s.getConfigCopy()
	s.mu.Unlock()

	newCfg, err := s.parseConfig()
	if err != nil {
		s.logger.Error("failed to parse config", "error", err)
		return err
	}

	// Notify subscribers
	if err := s.notifier.Notify(oldCfg, newCfg); err != nil {
		s.logger.Error("subscriber notification failed", "error", err)
	}

	s.mu.Lock()
	s.config = newCfg
	s.mu.Unlock()

	s.logger.Info("config reloaded successfully")
	return nil
}

// loadConfig loads configuration from file
func (s *Service) loadConfig() error {
	cfg, err := s.parseConfig()
	if err != nil {
		return err
	}

	s.config = cfg
	return nil
}

// parseConfig parses the configuration file
func (s *Service) parseConfig() (*Config, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, errors.NewConfigNotFoundError(s.path)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, errors.NewConfigParseError(s.path, err)
	}

	return &cfg, nil
}

// getConfigCopy returns a copy of the current config
func (s *Service) getConfigCopy() *Config {
	if s.config == nil {
		return nil
	}
	cfg := *s.config
	return &cfg
}

// Stop stops the hot reload service
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return config.ErrAlreadyStopped
	}

	s.cancel()
	if err := s.watcher.Stop(); err != nil {
		return err
	}

	s.started = false
	return nil
}

// Subscribe adds a subscriber
func (s *Service) Subscribe(sub config.Subscriber) error {
	if s.notifier.SubscriberCount() >= s.maxSubs {
		return config.ErrMaxSubscribers
	}
	return s.notifier.Subscribe(sub)
}

// Unsubscribe removes a subscriber
func (s *Service) Unsubscribe(sub config.Subscriber) error {
	return s.notifier.Unsubscribe(sub)
}

// GetConfig returns the current configuration
func (s *Service) GetConfig() *Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// IsStarted returns whether the service is started
func (s *Service) IsStarted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.started
}
