package learn

import (
	"time"

	"github.com/devrix/devrix/internal/layers/orchestration/wavescheduler"
)

// waveschedulerArtifactStub is a tiny constructor for tests that need a
// populated Artifact without copying the full struct literal each time.
func waveschedulerArtifactStub(taskID, sessionID string, files []string) *wavescheduler.Artifact {
	return &wavescheduler.Artifact{
		TaskID:       taskID,
		SessionID:    sessionID,
		Summary:      "test artifact",
		FilesChanged: files,
		StartedAt:    time.Now(),
		EndedAt:      time.Now(),
		Duration:     100 * time.Millisecond,
	}
}

// timePast returns time.Now() - 1h, used to force ScheduledMemory entries to
// be "due" for tick tests.
func timePast() time.Time {
	return time.Now().Add(-1 * time.Hour)
}