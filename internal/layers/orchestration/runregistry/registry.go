package runregistry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Status values for a run entry.
const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// Entry tracks one execution handle (How layer).
type Entry struct {
	ID            string
	WorkItemID    string
	SessionID     string
	Kind          string
	Status        string
	Summary       string
	Error         string
	OutputPath    string
	OutputOffset  int64
	Notified      bool
	StartedAt     time.Time
	EndedAt       time.Time
	cancel        context.CancelFunc
	onTerminal    func(Entry)
}

// Registry is the unified run observation store (DM-011 replacement).
type Registry struct {
	mu      sync.RWMutex
	entries map[string]*Entry
	byWork  map[string]string // workItemID -> runID
	dir     string
}

// NewRegistry creates an in-memory registry with optional disk output dir.
func NewRegistry(outputDir string) *Registry {
	if outputDir != "" {
		outputDir = expandPath(outputDir)
		_ = os.MkdirAll(outputDir, 0o755)
	}
	return &Registry{
		entries: make(map[string]*Entry),
		byWork:  make(map[string]string),
		dir:     outputDir,
	}
}

// Register starts tracking a run for a work item.
func (r *Registry) Register(sessionID, workItemID, kind string) (string, context.CancelFunc) {
	if r == nil {
		return "", func() {}
	}
	id := "run_" + uuid.New().String()[:8]
	ctx, cancel := context.WithCancel(context.Background())
	e := &Entry{
		ID:         id,
		WorkItemID: workItemID,
		SessionID:  sessionID,
		Kind:       kind,
		Status:     StatusRunning,
		StartedAt:  time.Now(),
		cancel:     cancel,
	}
	if r.dir != "" {
		e.OutputPath = filepath.Join(r.dir, id+".output")
	}

	r.mu.Lock()
	r.entries[id] = e
	if workItemID != "" {
		r.byWork[workItemID] = id
	}
	r.mu.Unlock()

	_ = ctx
	return id, cancel
}

// SetTerminal marks a run terminal; notifies at most once.
func (r *Registry) SetTerminal(runID, status, summary, errStr string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	e, ok := r.entries[runID]
	if !ok {
		r.mu.Unlock()
		return
	}
	if e.Notified && isTerminal(e.Status) {
		r.mu.Unlock()
		return
	}
	e.Status = status
	e.Summary = summary
	e.Error = errStr
	e.EndedAt = time.Now()
	e.Notified = true
	cb := e.onTerminal
	r.mu.Unlock()
	if cb != nil {
		cb(*e)
	}
}

// AppendOutput appends bytes to disk output file.
func (r *Registry) AppendOutput(runID string, data []byte) error {
	if r == nil || len(data) == 0 {
		return nil
	}
	r.mu.RLock()
	e, ok := r.entries[runID]
	r.mu.RUnlock()
	if !ok || e.OutputPath == "" {
		return nil
	}
	f, err := os.OpenFile(e.OutputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

// GetOutputDelta returns new output since offset.
func (r *Registry) GetOutputDelta(runID string, offset int64) (string, int64, string, error) {
	if r == nil {
		return "", offset, "", nil
	}
	r.mu.RLock()
	e, ok := r.entries[runID]
	r.mu.RUnlock()
	if !ok {
		return "", offset, "", fmt.Errorf("run not found: %s", runID)
	}
	status := e.Status
	if e.OutputPath == "" {
		return "", offset, status, nil
	}
	data, err := os.ReadFile(e.OutputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", offset, status, nil
		}
		return "", offset, status, err
	}
	if int64(len(data)) <= offset {
		return "", offset, status, nil
	}
	delta := string(data[offset:])
	return delta, int64(len(data)), status, nil
}

// Get returns an entry copy.
func (r *Registry) Get(runID string) (Entry, bool) {
	if r == nil {
		return Entry{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[runID]
	if !ok {
		return Entry{}, false
	}
	return *e, true
}

// GetByWorkItem returns run id for a work item.
func (r *Registry) GetByWorkItem(workItemID string) (string, bool) {
	if r == nil {
		return "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byWork[workItemID]
	return id, ok
}

// List returns entries for a session.
func (r *Registry) List(sessionID string) []Entry {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []Entry
	for _, e := range r.entries {
		if e.SessionID == sessionID {
			out = append(out, *e)
		}
	}
	return out
}

// Cancel cancels a running entry.
func (r *Registry) Cancel(runID string) error {
	if r == nil {
		return fmt.Errorf("registry nil")
	}
	r.mu.Lock()
	e, ok := r.entries[runID]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("run not found: %s", runID)
	}
	if e.cancel != nil {
		e.cancel()
	}
	e.Status = StatusCancelled
	e.EndedAt = time.Now()
	r.mu.Unlock()
	return nil
}

// OnTerminal registers a callback invoked once when SetTerminal runs.
func (r *Registry) OnTerminal(runID string, fn func(Entry)) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if e, ok := r.entries[runID]; ok {
		e.onTerminal = fn
	}
}

func isTerminal(s string) bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusCancelled
}

func expandPath(p string) string {
	if len(p) > 0 && p[0] == '~' {
		home, _ := os.UserHomeDir()
		p = filepath.Join(home, p[1:])
	}
	return os.ExpandEnv(p)
}

// Global is the process-wide run registry.
var Global *Registry

// SetGlobal installs the process-wide registry.
func SetGlobal(r *Registry) {
	Global = r
}
