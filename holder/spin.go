package holder

import "time"

// Spin is an alarm.Holder that busy-waits the last d before each time, where a timer fires up to a millisecond late on
// Linux. It takes a core for up to d per time. Under TinyGo no other goroutine runs meanwhile, and in a synctest bubble
// it never ends. Create one with NewSpin.
type Spin struct {
	d time.Duration
}

// NewSpin returns a Spin busy-waiting for the last d before each time. With d <= 0, it leaves each wait to the timer.
func NewSpin(d time.Duration) Spin {
	return Spin{d: d}
}

// Release returns true once at is due at now. Otherwise it returns how long until the last d before at, or 0 within
// it.
func (s Spin) Release(at, now time.Time) (time.Duration, bool) {
	d := at.Sub(now)
	if d <= 0 {
		return 0, true
	}

	return max(d-s.d, 0), false
}
