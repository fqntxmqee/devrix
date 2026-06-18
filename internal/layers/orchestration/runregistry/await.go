package runregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Await polls a run until terminal or timeout.
func Await(ctx context.Context, runID string, block bool, timeout time.Duration) (string, error) {
	if Global == nil {
		return "", fmt.Errorf("runregistry not initialized")
	}
	deadline := time.Now().Add(timeout)
	var offset int64
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		delta, newOff, status, err := Global.GetOutputDelta(runID, offset)
		if err != nil {
			return "", err
		}
		offset = newOff
		entry, ok := Global.Get(runID)
		if !ok {
			return "", fmt.Errorf("run not found: %s", runID)
		}
		if isRunTerminal(status) {
			out := map[string]any{
				"run_id":  runID,
				"status":  status,
				"summary": entry.Summary,
				"output":  delta,
			}
			if entry.Error != "" {
				out["error"] = entry.Error
			}
			data, _ := json.Marshal(out)
			return string(data), nil
		}
		if !block || time.Now().After(deadline) {
			out := map[string]any{"run_id": runID, "status": status, "partial_output": delta}
			data, _ := json.Marshal(out)
			return string(data), nil
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func isRunTerminal(s string) bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusCancelled
}
