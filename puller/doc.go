// Package puller holds ready-made alarm.Puller, for alarm.WithPuller, so that a slow sequence, such as one read from a
// store, yields while Ring waits:
//
//	err := alarm.New(alarm.WithPuller(puller.NewHorizon(time.Minute, 1000))).Ring(ctx, times, fn)
//
// NewAhead holds n times read ahead, and NewHorizon the times due within d.
package puller
