// Package puller holds ready-made alarm.Puller for alarm.WithPuller, which decide when Ring reads each time of the
// sequence ahead of the one it waits for, so that a slow sequence, such as one read from a store, yields it while Ring
// waits:
//
//	err := alarm.New(alarm.WithPuller(puller.NewHorizon(time.Minute, 1000))).Ring(ctx, times, fn)
//
// NewAhead holds n times read ahead, and NewHorizon the times due within d.
package puller
