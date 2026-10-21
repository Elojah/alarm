// Package holder holds ready-made alarm.Holder for alarm.WithHolder, which decide when each time is due, and how Run
// waits for it:
//
//	err := alarm.Run(ctx, times, fn, alarm.WithHolder(holder.NewSpin(time.Millisecond)))
//
// NewSpin busy-waits the end of each wait, to the microsecond, rather than leaving it to the timer.
package holder
