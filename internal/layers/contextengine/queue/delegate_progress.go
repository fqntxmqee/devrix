package queue

import (
	"encoding/json"
	"fmt"

	"github.com/devrix/devrix/internal/shared/contracts"
)

func formatDelegateProgressReminder(raw string) string {
	var ev contracts.FlowEvent
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		return raw
	}
	line := ev.Summary
	if line == "" {
		line = fmt.Sprintf("%s %s %s", ev.Source, ev.Role, ev.Kind)
	}
	return fmt.Sprintf("[%s/%s] %s", ev.WorkerID, ev.Kind, line)
}
