package contracts

import (
	"encoding/json"
	"fmt"
)

// FormatDelegateProgressReminder renders a FlowEvent JSON payload for Leader drain.
func FormatDelegateProgressReminder(raw string) string {
	var ev FlowEvent
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		return raw
	}
	line := ev.Summary
	if line == "" {
		line = fmt.Sprintf("%s %s %s", ev.Source, ev.Role, ev.Kind)
	}
	return fmt.Sprintf("[%s/%s] %s", ev.WorkerID, ev.Kind, line)
}
