package puller

import (
	"math"
	"time"
)

// Horizon is an alarm.Puller that reads the next time once the last read is due within d, so that each time of a
// sorted sequence is read at least d before it, with at most n held ahead of the one Run waits for. It holds every
// time due within d, and the first after: mind a dense sequence, or cap it with n. Create one with NewHorizon.
type Horizon struct {
	d time.Duration
	n int
}

// NewHorizon returns a Horizon reading the times due within d, at most n ahead. With d <= 0, it reads none ahead, and
// with n <= 0, any number.
func NewHorizon(d time.Duration, n int) Horizon {
	return Horizon{d: d, n: n}
}

// Pull returns true once last is due within d at now, while fewer than n times are held ahead. Otherwise it returns
// how long until last is, or, with n held, Run asks again once it fired one.
func (h Horizon) Pull(ahead int, last, now time.Time) (time.Duration, bool) {
	if h.d <= 0 || (h.n > 0 && ahead >= h.n) {
		return math.MaxInt64, false
	}
	// Compared before subtracting: last.Sub saturates, and a long passed last would overflow.
	if until := last.Sub(now); until > h.d {
		return until - h.d, false
	}

	return 0, true
}
