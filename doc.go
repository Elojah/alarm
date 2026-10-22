// Package alarm calls a func, or sends on a channel, at each time of a sequence, such as an rrule.Set's occurrences.
//
//	set, err := rrule.ParseSet("DTSTART:20261021T000000Z\nRRULE:FREQ=SECONDLY;X-FREQ=PT0.001S") // every millisecond
//	...
//	a := alarm.New()
//	err = a.Ring(ctx, set.Between(time.Now(), time.Time{}), func(ctx context.Context, at time.Time) { ... })
//
// Times are read one at a time, so a sequence can be endless, and nothing is allocated per time. Each is waited for
// itself, not as a gap from the last: lateness from waits never adds up, and a passed time fires at once, so start the
// sequence at now, drop passed times with middleware.MaxDelay, or fire only the latest with WithCoalesce.
//
// Reset replaces the sequence of the Ring under way, Close stops it, and Shutdown stops it and waits until it returns,
// from any goroutine, fn included.
//
// A Holder decides when each time is due, a Puller when it is read, and a Middleware wraps fn. Packages holder, puller
// and middleware hold ready-made ones: holder.NewSpin fires to the microsecond, where a timer is up to a millisecond
// late on Linux, at the cost of a core, and middleware.Recover keeps a panic in fn from ending Ring.
package alarm
