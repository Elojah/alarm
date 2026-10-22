package puller

import "time"

// Puller is alarm.Puller, which Every wraps: puller imports nothing from alarm.
type Puller interface {
	Pull(ahead int, last, now time.Time) (time.Duration, bool)
}

// Every is an alarm.Puller that lets p read only at ticks, every d: from a tick until p reads none, then none until the
// next, which keeps to the first tick's grid unless one was missed. It keeps its ticks, so give each Alarm its own.
// Create one with NewEvery.
type Every struct {
	p Puller
	d time.Duration
	// next is the next tick, and reading is set from a tick until p reads none.
	next    time.Time
	reading bool
}

// NewEvery returns an Every letting p read every d, such as NewHorizon's times due within a day, every 15 minutes:
//
//	puller.NewEvery(15*time.Minute, puller.NewHorizon(24*time.Hour, 0))
//
// With d <= 0, p reads as it would alone.
func NewEvery(d time.Duration, p Puller) *Every {
	return &Every{p: p, d: d}
}

// Pull asks p from a tick until it returns false. Otherwise it returns how long until the next tick, or p's wait once
// that tick passed.
func (e *Every) Pull(ahead int, last, now time.Time) (time.Duration, bool) {
	if !e.reading {
		if now.Before(e.next) {
			return e.next.Sub(now), false
		}
		// On the grid, so that lateness never adds up, unless a tick was missed.
		if e.next = e.next.Add(e.d); !e.next.After(now) {
			e.next = now.Add(e.d)
		}
		e.reading = true
	}
	wait, pull := e.p.Pull(ahead, last, now)
	if pull {
		return wait, true
	}
	e.reading = false
	if until := e.next.Sub(now); until > 0 {
		return until, false
	}

	return wait, false
}
