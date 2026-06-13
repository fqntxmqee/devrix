package config

import (
	"sort"
	"sync"
)

// Subscriber is the interface for config change subscribers
type Subscriber interface {
	// OnConfigChange is called when config changes
	OnConfigChange(oldCfg, newCfg *Config) error

	// Priority returns the subscriber priority (lower = higher priority)
	Priority() int
}

// Notifier manages config change subscribers
type Notifier struct {
	subscribers map[Subscriber]struct{}
	mu          sync.RWMutex
}

// NewNotifier creates a new Notifier
func NewNotifier() *Notifier {
	return &Notifier{
		subscribers: make(map[Subscriber]struct{}),
	}
}

// Subscribe adds a subscriber to the notifier
func (n *Notifier) Subscribe(sub Subscriber) error {
	if sub == nil {
		return ErrNilSubscriber
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	n.subscribers[sub] = struct{}{}
	return nil
}

// Unsubscribe removes a subscriber from the notifier
func (n *Notifier) Unsubscribe(sub Subscriber) error {
	if sub == nil {
		return nil
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	delete(n.subscribers, sub)
	return nil
}

// Notify notifies all subscribers of config change
// Returns the first non-nil error encountered
func (n *Notifier) Notify(oldCfg, newCfg *Config) error {
	n.mu.RLock()
	subs := n.getSortedSubscribers()
	n.mu.RUnlock()

	var lastErr error
	for _, sub := range subs {
		if err := sub.OnConfigChange(oldCfg, newCfg); err != nil {
			lastErr = err
			// Continue notifying other subscribers
		}
	}
	return lastErr
}

// getSortedSubscribers returns subscribers sorted by priority
func (n *Notifier) getSortedSubscribers() []Subscriber {
	subs := make([]Subscriber, 0, len(n.subscribers))
	for sub := range n.subscribers {
		subs = append(subs, sub)
	}

	sort.Slice(subs, func(i, j int) bool {
		return subs[i].Priority() < subs[j].Priority()
	})

	return subs
}

// SubscriberCount returns the number of subscribers
func (n *Notifier) SubscriberCount() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return len(n.subscribers)
}
