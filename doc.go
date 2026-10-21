// Package alarm calls a func, or sends on a channel, at each time of a sequence, such as an rrule.Set's occurrences.
//
//	set, err := rrule.ParseSet("DTSTART:20261021T000000Z\nRRULE:FREQ=SECONDLY;X-FREQ=PT0.001S") // every millisecond
//	...
//	err = alarm.Run(ctx, set.Between(time.Now(), time.Time{}), func(at time.Time) { ... })
//
// Times are read one at a time, so a sequence can be endless, and nothing is allocated per time. Each is waited for
// itself, not as a gap from the last, so a passed time fires at once: start the sequence at now, or set WithMaxDelay.
// A wait reads the wall clock again at least once a minute, so a clock step or a suspend shows within it.
//
// On Linux a timer fires up to a millisecond late. WithSpin busy-waits the end of each wait instead, to the
// microsecond, at the cost of a core.
package alarm
