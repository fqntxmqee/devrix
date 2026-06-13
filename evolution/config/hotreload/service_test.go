package hotreload

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/devrix/devrix/evolution/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_NewService(t *testing.T) {
	// Create temp config file
	path := "/tmp/test_service_new.yaml"
	createTestConfig(t, path, "info", "openai", "gpt-4")

	opts := Options{
		ConfigPath: path,
		Debounce:   100 * time.Millisecond,
		Logger:     &mockLogger{},
	}

	svc := NewService(opts)
	assert.NotNil(t, svc)
	assert.Equal(t, path, svc.path)
	assert.Equal(t, 100*time.Millisecond, svc.debounce)
}

func TestService_StartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	path := "/tmp/test_service_start.yaml"
	createTestConfig(t, path, "info", "openai", "gpt-4")

	svc := NewService(Options{
		ConfigPath: path,
		Debounce:   100 * time.Millisecond,
		Logger:     &mockLogger{},
	})

	ctx := context.Background()
	err := svc.Start(ctx)
	require.NoError(t, err)
	assert.True(t, svc.IsStarted())

	err = svc.Stop()
	require.NoError(t, err)
	assert.False(t, svc.IsStarted())
}

func TestService_DoubleStart(t *testing.T) {
	path := "/tmp/test_double_start.yaml"
	createTestConfig(t, path, "info", "openai", "gpt-4")

	svc := NewService(Options{
		ConfigPath: path,
		Logger:     &mockLogger{},
	})

	ctx := context.Background()
	err := svc.Start(ctx)
	require.NoError(t, err)
	defer svc.Stop()

	err = svc.Start(ctx)
	assert.ErrorIs(t, config.ErrAlreadyStarted, err)
}

func TestService_DoubleStop(t *testing.T) {
	path := "/tmp/test_double_stop.yaml"
	createTestConfig(t, path, "info", "openai", "gpt-4")

	svc := NewService(Options{
		ConfigPath: path,
		Logger:     &mockLogger{},
	})

	ctx := context.Background()
	err := svc.Start(ctx)
	require.NoError(t, err)

	err = svc.Stop()
	require.NoError(t, err)

	err = svc.Stop()
	assert.ErrorIs(t, config.ErrAlreadyStopped, err)
}

func TestService_GetConfig(t *testing.T) {
	path := "/tmp/test_get_config.yaml"
	createTestConfig(t, path, "debug", "anthropic", "claude-3")

	svc := NewService(Options{
		ConfigPath: path,
		Logger:     &mockLogger{},
	})

	ctx := context.Background()
	err := svc.Start(ctx)
	require.NoError(t, err)
	defer svc.Stop()

	cfg := svc.GetConfig()
	require.NotNil(t, cfg)
	assert.Equal(t, "debug", cfg.Log.Level)
	assert.Equal(t, "anthropic", cfg.LLM.Provider)
	assert.Equal(t, "claude-3", cfg.LLM.Model)
}

func TestService_Subscribe(t *testing.T) {
	path := "/tmp/test_subscribe.yaml"
	createTestConfig(t, path, "info", "openai", "gpt-4")

	svc := NewService(Options{
		ConfigPath: path,
		Logger:     &mockLogger{},
	})

	ctx := context.Background()
	err := svc.Start(ctx)
	require.NoError(t, err)
	defer svc.Stop()

	sub := &testSubscriber{
		priority: 1,
	}
	err = svc.Subscribe(sub)
	require.NoError(t, err)
}

func TestService_MaxSubscribers(t *testing.T) {
	path := "/tmp/test_max_subs.yaml"
	createTestConfig(t, path, "info", "openai", "gpt-4")

	svc := NewService(Options{
		ConfigPath:    path,
		MaxSubscribers: 2,
		Logger:        &mockLogger{},
	})

	ctx := context.Background()
	err := svc.Start(ctx)
	require.NoError(t, err)
	defer svc.Stop()

	// Subscribe max number
	sub1 := &testSubscriber{priority: 1}
	sub2 := &testSubscriber{priority: 2}
	require.NoError(t, svc.Subscribe(sub1))
	require.NoError(t, svc.Subscribe(sub2))

	// Next should fail
	sub3 := &testSubscriber{priority: 3}
	err = svc.Subscribe(sub3)
	assert.ErrorIs(t, config.ErrMaxSubscribers, err)
}

func TestService_Unsubscribe(t *testing.T) {
	path := "/tmp/test_unsubscribe.yaml"
	createTestConfig(t, path, "info", "openai", "gpt-4")

	svc := NewService(Options{
		ConfigPath: path,
		Logger:     &mockLogger{},
	})

	ctx := context.Background()
	err := svc.Start(ctx)
	require.NoError(t, err)
	defer svc.Stop()

	sub := &testSubscriber{priority: 1}
	require.NoError(t, svc.Subscribe(sub))

	err = svc.Unsubscribe(sub)
	require.NoError(t, err)

	// Should be able to subscribe again
	err = svc.Subscribe(sub)
	require.NoError(t, err)
}

func TestService_ConfigReload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	path := "/tmp/test_reload.yaml"
	createTestConfig(t, path, "info", "openai", "gpt-4")

	svc := NewService(Options{
		ConfigPath: path,
		Debounce:   100 * time.Millisecond,
		Logger:     &mockLogger{},
	})

	ctx := context.Background()
	err := svc.Start(ctx)
	require.NoError(t, err)
	defer svc.Stop()

	// Verify initial config
	cfg := svc.GetConfig()
	assert.Equal(t, "info", cfg.Log.Level)

	// Modify config file
	time.Sleep(200 * time.Millisecond)
	createTestConfig(t, path, "debug", "anthropic", "claude-3")

	// Wait for debounce + reload
	time.Sleep(300 * time.Millisecond)

	// Verify updated config
	cfg = svc.GetConfig()
	assert.Equal(t, "debug", cfg.Log.Level)
	assert.Equal(t, "anthropic", cfg.LLM.Provider)
}

func TestService_SubscriberCallback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}

	path := "/tmp/test_callback.yaml"
	createTestConfig(t, path, "info", "openai", "gpt-4")

	svc := NewService(Options{
		ConfigPath: path,
		Debounce:   100 * time.Millisecond,
		Logger:     &mockLogger{},
	})

	ctx := context.Background()
	err := svc.Start(ctx)
	require.NoError(t, err)
	defer svc.Stop()

	var callbackMu sync.Mutex
	var receivedOld, receivedNew *Config

	sub := &configSubscriber{
		priority: 1,
		onChange: func(oldCfg, newCfg *Config) error {
			callbackMu.Lock()
			defer callbackMu.Unlock()
			receivedOld = oldCfg
			receivedNew = newCfg
			return nil
		},
	}

	err = svc.Subscribe(sub)
	require.NoError(t, err)

	// Modify config file
	time.Sleep(200 * time.Millisecond)
	createTestConfig(t, path, "debug", "anthropic", "claude-3")

	// Wait for debounce + reload
	time.Sleep(300 * time.Millisecond)

	// Verify callback received correct values
	callbackMu.Lock()
	defer callbackMu.Unlock()
	require.NotNil(t, receivedOld)
	require.NotNil(t, receivedNew)
	assert.Equal(t, "info", receivedOld.Log.Level)
	assert.Equal(t, "debug", receivedNew.Log.Level)
}

// Helper functions

func createTestConfig(t *testing.T, path, logLevel, provider, model string) {
	content := `log:
  level: ` + logLevel + `
llm:
  provider: ` + provider + `
  model: ` + model
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
}

type mockLogger struct{}

func (m *mockLogger) Info(msg string, args ...any)  {}
func (m *mockLogger) Error(msg string, args ...any) {}
func (m *mockLogger) Debug(msg string, args ...any) {}

// testSubscriber implements config.Subscriber for testing
type testSubscriber struct {
	priority    int
	onChangeErr error
}

func (t *testSubscriber) OnConfigChange(oldCfg, newCfg *Config) error {
	return t.onChangeErr
}

func (t *testSubscriber) Priority() int {
	return t.priority
}

// configSubscriber is a test subscriber that wraps a closure
type configSubscriber struct {
	priority int
	onChange func(oldCfg, newCfg *Config) error
}

func (c *configSubscriber) OnConfigChange(oldCfg, newCfg *Config) error {
	return c.onChange(oldCfg, newCfg)
}

func (c *configSubscriber) Priority() int {
	return c.priority
}
