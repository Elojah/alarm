package puller

import (
	"math"
	"time"
)

// Ahead is an alarm.Puller that holds n times read ahead of the one Run waits for. Create one with NewAhead.
type Ahead struct {
	n int
}

// NewAhead returns an Ahead holding n times read ahead. With n <= 0, it reads none ahead.
func NewAhead(n int) Ahead {
	return Ahead{n: n}
}

// Pull returns true while fewer than n times are held ahead. Otherwise Run asks again once it fired one.
func (a Ahead) Pull(ahead int, _, _ time.Time) (time.Duration, bool) {
	if ahead < a.n {
		return 0, true
	}

	return math.MaxInt64, false
}
