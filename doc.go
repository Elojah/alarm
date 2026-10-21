// Package alarm calls a func, or sends on a channel, at each time of a sequence, such as the occurrences of an
// rrule.Set: one sequence, one alarm.
//
//	set, err := rrule.ParseSet("DTSTART:20261021T000000Z\nRRULE:FREQ=SECONDLY;X-FREQ=PT0.001S") // every millisecond
//	...
//	err = alarm.Run(ctx, set.Between(time.Now(), time.Time{}), func(at time.Time) { ... })
//
// Run calls a func in the calling goroutine, and Chan sends on a channel from a goroutine of its own.
//
// # Time
//
// Times are read one at a time, once the previous one has fired, so a sequence can be endless, and waited for with one
// timer, reset for each: nothing is allocated per time. Each time is waited for itself, not as a gap from the last one,
// so a late one does not delay the next, and a time already passed fires at once: give rrule's Between now as from, or
// WithMaxDelay a bound. Run reads the wall clock again at least once a minute while it waits, so a time comes when the
// clock shows it, even after the clock steps or the system is suspended.
//
// A timer fires up to a millisecond late on Linux, where Go waits for timers in whole milliseconds. WithSpin
// busy-waits for the end of each wait instead, to the microsecond, at the cost of a core while it does.
package alarm
