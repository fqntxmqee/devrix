package wavescheduler

import "fmt"

// errWave is a tiny error constructor local to the wave package. We avoid
// importing internal/shared/errors to keep the package self-contained for unit
// tests and to make the API surface clear.
type waveError struct {
	msg string
}

func (e *waveError) Error() string { return e.msg }

func errWave(format string, args ...any) error {
	return &waveError{msg: fmt.Sprintf(format, args...)}
}

// IsWaveError reports whether err originated in the wave package.
func IsWaveError(err error) bool {
	_, ok := err.(*waveError)
	return ok
}
