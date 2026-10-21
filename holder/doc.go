// Package holder holds ready-made alarm.Holder, for alarm.WithHolder:
//
//	err := alarm.New(alarm.WithHolder(holder.NewSpin(time.Millisecond))).Ring(ctx, times, fn)
//
// NewSpin busy-waits the end of each wait rather than leaving it to the timer, to fire to the microsecond.
package holder
