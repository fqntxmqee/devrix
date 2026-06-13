// Package config provides configuration hot-reload primitives (watcher, notifier).
package config

import (
	stderrors "errors"
	"time"
)

// ErrAlreadyStarted indicates the hot-reload service is already running.
var ErrAlreadyStarted = stderrors.New("config: hotreload service already started")

// ErrAlreadyStopped indicates the hot-reload service is not running.
var ErrAlreadyStopped = stderrors.New("config: hotreload service already stopped")

// ErrMaxSubscribers indicates the subscriber limit was reached.
var ErrMaxSubscribers = stderrors.New("config: max subscribers reached")

// ErrNilSubscriber indicates a nil subscriber was passed.
var ErrNilSubscriber = stderrors.New("config: nil subscriber")

// DefaultDebounce is the default debounce delay for file change events.
const DefaultDebounce = 500 * time.Millisecond

// DefaultMaxSubscribers is the default maximum number of config change subscribers.
const DefaultMaxSubscribers = 10
