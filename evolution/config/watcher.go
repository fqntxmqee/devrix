package config

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Op represents file operation types
type Op uint32

const (
	CREATE Op = 1 << iota
	WRITE
	REMOVE
	RENAME
)

// String returns the string representation of Op
func (o Op) String() string {
	switch o {
	case CREATE:
		return "CREATE"
	case WRITE:
		return "WRITE"
	case REMOVE:
		return "REMOVE"
	case RENAME:
		return "RENAME"
	default:
		return "UNKNOWN"
	}
}

// Event represents a file change event
type Event struct {
	Path string
	Op   Op
	Time time.Time
}

// Watcher is the interface for file system watching
type Watcher interface {
	// Start starts watching the file
	Start(ctx context.Context) error

	// Stop stops watching
	Stop() error

	// Events returns the event channel
	Events() <-chan Event
}

// ConfigWatcher implements Watcher using fsnotify
type ConfigWatcher struct {
	path    string
	watcher *fsnotify.Watcher
	events  chan Event
	done    chan struct{}
	mu      sync.RWMutex
	started bool
}

// NewConfigWatcher creates a new ConfigWatcher
func NewConfigWatcher(path string) *ConfigWatcher {
	return &ConfigWatcher{
		path:   path,
		events: make(chan Event, 100),
		done:   make(chan struct{}),
	}
}

// Start starts watching the file
func (w *ConfigWatcher) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.started {
		return ErrAlreadyStarted
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w.watcher = watcher

	// Ensure the directory exists
	dir := w.path
	for i := len(dir) - 1; i >= 0; i-- {
		if dir[i] == '/' {
			dir = dir[:i]
			break
		}
	}
	if dir == "" {
		dir = "."
	}

	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return err
	}

	w.started = true

	go w.run(ctx)
	return nil
}

// run processes fsnotify events
func (w *ConfigWatcher) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			w.close()
			return
		case <-w.done:
			w.close()
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			// Log error but don't stop watching
			w.events <- Event{
				Path: "",
				Op:   0,
				Time: time.Now(),
			}
			_ = err
		}
	}
}

// handleEvent converts fsnotify event to our Event type
func (w *ConfigWatcher) handleEvent(event fsnotify.Event) {
	if event.Name != w.path {
		return
	}

	var op Op
	if event.Has(fsnotify.Create) {
		op |= CREATE
	}
	if event.Has(fsnotify.Write) {
		op |= WRITE
	}
	if event.Has(fsnotify.Remove) {
		op |= REMOVE
	}
	if event.Has(fsnotify.Rename) {
		op |= RENAME
	}

	w.events <- Event{
		Path: event.Name,
		Op:   op,
		Time: time.Now(),
	}
}

// close closes the watcher
func (w *ConfigWatcher) close() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.watcher != nil {
		w.watcher.Close()
		w.watcher = nil
	}
	close(w.events)
}

// Stop stops watching
func (w *ConfigWatcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.started {
		return ErrAlreadyStopped
	}

	close(w.done)
	w.started = false
	return nil
}

// Events returns the event channel
func (w *ConfigWatcher) Events() <-chan Event {
	return w.events
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
