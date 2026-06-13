package config

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSubscriber implements Subscriber for testing
type mockSubscriber struct {
	name      string
	priority  int
	onChange  func(oldCfg, newCfg *Config) error
	callCount int
	mu        sync.Mutex
}

func (m *mockSubscriber) OnConfigChange(oldCfg, newCfg *Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.onChange(oldCfg, newCfg)
}

func (m *mockSubscriber) Priority() int {
	return m.priority
}

func (m *mockSubscriber) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func TestNotifier_Subscribe(t *testing.T) {
	notifier := NewNotifier()
	sub := &mockSubscriber{name: "test", priority: 1}

	err := notifier.Subscribe(sub)
	require.NoError(t, err)
	assert.Equal(t, 1, notifier.SubscriberCount())
}

func TestNotifier_SubscribeNil(t *testing.T) {
	notifier := NewNotifier()

	err := notifier.Subscribe(nil)
	assert.ErrorIs(t, ErrNilSubscriber, err)
}

func TestNotifier_Unsubscribe(t *testing.T) {
	notifier := NewNotifier()
	sub := &mockSubscriber{name: "test", priority: 1}

	err := notifier.Subscribe(sub)
	require.NoError(t, err)

	err = notifier.Unsubscribe(sub)
	require.NoError(t, err)
	assert.Equal(t, 0, notifier.SubscriberCount())
}

func TestNotifier_UnsubscribeNil(t *testing.T) {
	notifier := NewNotifier()

	err := notifier.Unsubscribe(nil)
	assert.NoError(t, err)
}

func TestNotifier_Notify(t *testing.T) {
	notifier := NewNotifier()
	var callOrder []string

	sub1 := &mockSubscriber{
		name:     "sub1",
		priority: 1,
		onChange: func(oldCfg, newCfg *Config) error {
			callOrder = append(callOrder, "sub1")
			return nil
		},
	}

	sub2 := &mockSubscriber{
		name:     "sub2",
		priority: 2,
		onChange: func(oldCfg, newCfg *Config) error {
			callOrder = append(callOrder, "sub2")
			return nil
		},
	}

	notifier.Subscribe(sub1)
	notifier.Subscribe(sub2)

	oldCfg := &Config{}
	newCfg := &Config{}
	err := notifier.Notify(oldCfg, newCfg)

	require.NoError(t, err)
	assert.Equal(t, []string{"sub1", "sub2"}, callOrder)
}

func TestNotifier_NotifyErrorContinues(t *testing.T) {
	notifier := NewNotifier()
	testErr := errors.New("test error")

	sub1 := &mockSubscriber{
		name:     "sub1",
		priority: 1,
		onChange: func(oldCfg, newCfg *Config) error {
			return testErr
		},
	}

	sub2 := &mockSubscriber{
		name:     "sub2",
		priority: 2,
		onChange: func(oldCfg, newCfg *Config) error {
			return nil
		},
	}

	notifier.Subscribe(sub1)
	notifier.Subscribe(sub2)

	oldCfg := &Config{}
	newCfg := &Config{}
	err := notifier.Notify(oldCfg, newCfg)

	// Should return the last error
	assert.ErrorIs(t, testErr, err)
	assert.Equal(t, 1, sub2.CallCount())
}

func TestNotifier_SortByPriority(t *testing.T) {
	notifier := NewNotifier()
	var callOrder []int

	for i := 10; i >= 1; i-- {
		sub := &mockSubscriber{
			name:     "sub",
			priority: i,
			onChange: func(oldCfg, newCfg *Config) error {
				return nil
			},
		}
		notifier.Subscribe(sub)
	}

	// Subscribe one more that will be called
	sub10 := &mockSubscriber{
		name:     "sub10",
		priority: 10,
		onChange: func(oldCfg, newCfg *Config) error {
			return nil
		},
	}
	notifier.Subscribe(sub10)

	// Unsubscribe sub10 and re-subscribe to ensure it's at the end
	notifier.Unsubscribe(sub10)
	for i := 1; i <= 10; i++ {
		sub := &mockSubscriber{
			name:     "sorted",
			priority: i,
			onChange: func(oldCfg, newCfg *Config) error {
				callOrder = append(callOrder, i)
				return nil
			},
		}
		notifier.Subscribe(sub)
	}

	notifier.Notify(&Config{}, &Config{})
	assert.Equal(t, 10, len(callOrder))
}

func TestNotifier_MultipleSubscriptions(t *testing.T) {
	notifier := NewNotifier()
	maxSubs := 10

	// Subscribe max number of subscribers
	for i := 0; i < maxSubs; i++ {
		sub := &mockSubscriber{
			name:     "sub",
			priority: i,
			onChange: func(oldCfg, newCfg *Config) error {
				return nil
			},
		}
		err := notifier.Subscribe(sub)
		require.NoError(t, err)
	}

	assert.Equal(t, maxSubs, notifier.SubscriberCount())

	// Additional subscription should work (not enforced here)
	extraSub := &mockSubscriber{
		name:     "extra",
		priority: 100,
		onChange: func(oldCfg, newCfg *Config) error {
			return nil
		},
	}
	err := notifier.Subscribe(extraSub)
	require.NoError(t, err)
	assert.Equal(t, maxSubs+1, notifier.SubscriberCount())
}
