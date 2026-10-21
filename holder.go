package alarm

import "time"

// Holder decides when each time is due, and how long Ring may sleep on its timer meanwhile (WithHolder). Package holder
// holds ready-made ones. It reads no clock: each call passes now, which has a monotonic reading, so at.Sub(now) is
// monotonic if at has one.
type Holder interface {
	// Release returns true once at is due at now, and must then at any later now: once it released a time, Ring asks
	// about the next at the same now, and reads the clock only if that one is not due, so that a run of passed times
	// reads it once. Otherwise it returns how long Ring may sleep before asking again, at most WithPoll's d being
	// slept, or 0 to ask again at once.
	Release(at, now time.Time) (time.Duration, bool)
}

// onTime is the default Holder: a time is due once the clock shows it.
type onTime struct{}

func (onTime) Release(at, now time.Time) (time.Duration, bool) {
	if d := at.Sub(now); d > 0 {
		return d, false
	}

	return 0, true
}
