// Package holder holds ready-made alarm.Holder for alarm.WithHolder, which decide when each time is due, and how Ring
// waits for it:
//
//	err := alarm.New(alarm.WithHolder(holder.NewSpin(time.Millisecond))).Ring(ctx, times, fn)
//
// NewSpin busy-waits the end of each wait, to the microsecond, rather than leaving it to the timer.
package holder
