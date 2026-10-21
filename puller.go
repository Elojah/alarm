package alarm

import (
	"math"
	"time"
)

// Puller decides when Ring reads the next time ahead of the one it waits for (WithPuller). Package puller holds
// ready-made ones. A read keeps Ring from the time it waits for, so Ring asks none while that time is due or a Holder
// spins.
type Puller interface {
	// Pull returns true if Ring is to read the next time at now, with ahead times held after the one it waits for and
	// last the last read. Otherwise it returns how long Ring may wait before asking again, capped by WithPoll, or 0 to
	// ask again at once. Ring also asks once it fired a time.
	Pull(ahead int, last, now time.Time) (time.Duration, bool)
}

// lazy is the default Puller: Ring reads a time once it fired the last.
type lazy struct{}

func (lazy) Pull(int, time.Time, time.Time) (time.Duration, bool) {
	return math.MaxInt64, false
}
