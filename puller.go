package alarm

import (
	"math"
	"time"
)

// Puller decides when Run reads the next time of the sequence ahead of the one it waits for, rather than once it fired
// it (WithPuller). Package puller holds ready-made ones. Reading keeps Run from the time it waits for until the sequence
// yields, so Run reads none ahead while that time is due or a Holder spins.
type Puller interface {
	// Pull returns true if Run reads the next time at now, holding ahead times read after the one it waits for, the
	// last read being last. Otherwise it returns how long Run may wait before asking again, at most WithPoll's d being
	// waited, or 0 to ask again at once. Run also asks again once it fired a time.
	Pull(ahead int, last, now time.Time) (time.Duration, bool)
}

// lazy is the default Puller: Run reads a time once it fired the last.
type lazy struct{}

func (lazy) Pull(int, time.Time, time.Time) (time.Duration, bool) {
	return math.MaxInt64, false
}
