package capture

import "time"

// clock abstracts time for tests.
type clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }
