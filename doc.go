// Package alarm calls a func, or sends on a channel, at each time of a sequence, such as an rrule.Set's occurrences.
//
//	set, err := rrule.ParseSet("DTSTART:20261021T000000Z\nRRULE:FREQ=SECONDLY;X-FREQ=PT0.001S") // every millisecond
//	...
//	err = alarm.Run(ctx, set.Between(time.Now(), time.Time{}), func(ctx context.Context, at time.Time) { ... })
//
// Times are read one at a time, once the last fired, unless a Puller reads them ahead (WithPuller), so a sequence can
// be endless, and nothing is allocated per time. Each is waited for
// itself, not as a gap from the last, so a passed time fires at once: start the sequence at now, or drop it with
// middleware.MaxDelay. A wait polls the clock at least once a minute, or as WithPoll sets, so a clock step or a
// suspend shows within it.
//
// A Holder decides when each time is due (WithHolder), a Puller when it is read (WithPuller), and Middleware wraps fn
// (WithMiddleware). Packages holder, puller and middleware hold ready-made ones. On Linux a timer fires up to a millisecond late: holder.NewSpin busy-waits the end
// of each wait instead, to the microsecond, at the cost of a core.
package alarm
