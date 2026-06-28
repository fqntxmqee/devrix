package errors

import (
	"fmt"
	"time"
)

// TurnInProgressError is returned when a session already has an in-flight turn.
// D1 adapters detect it via errors.As without importing D7 packages.
type TurnInProgressError struct {
	SessionID      string
	SinceStartedAt time.Time
	TurnNo         int
}

func (e TurnInProgressError) Error() string {
	return fmt.Sprintf("session %s turn %d still in progress since %s",
		e.SessionID, e.TurnNo, e.SinceStartedAt.Format(time.RFC3339Nano))
}

func (e TurnInProgressError) Is(target error) bool {
	_, ok := target.(TurnInProgressError)
	return ok
}
