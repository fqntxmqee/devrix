package workmodel

import "sync/atomic"

// TaskManagerMetrics counts publishCompletion panic-recover events.
//
// DM-20260621-010 PR-B: replaces the previous `_ = recover()` silent-swallow
// at task_manager.go:219 with structured counters + slog.Error, so the
// notification-bus publish path is no longer a black hole.
//
// Threading: atomic.Int64; safe for concurrent publishCompletion calls.
type TaskManagerMetrics struct {
	PublishCompletionPanics atomic.Int64
}

// Snapshot returns a point-in-time view. Safe on a nil receiver.
func (m *TaskManagerMetrics) Snapshot() TaskManagerMetricsSnapshot {
	if m == nil {
		return TaskManagerMetricsSnapshot{}
	}
	return TaskManagerMetricsSnapshot{
		PublishCompletionPanics: m.PublishCompletionPanics.Load(),
	}
}

// TaskManagerMetricsSnapshot is the JSON-friendly view of TaskManagerMetrics.
type TaskManagerMetricsSnapshot struct {
	PublishCompletionPanics int64 `json:"publish_completion_panics"`
}