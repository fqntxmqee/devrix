package d7

import "time"

// durationOrDefault returns time.Millisecond * d, or a sane default if d <= 0.
func durationOrDefault(d int) time.Duration {
	if d <= 0 {
		return 50 * time.Millisecond
	}
	return time.Duration(d) * time.Millisecond
}
