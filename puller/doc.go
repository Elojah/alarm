// Package puller holds ready-made alarm.Puller for alarm.WithPuller, which decide when Run reads each time of the
// sequence ahead of the one it waits for, so that a slow sequence, such as one read from a store, yields it while Run
// waits:
//
//	err := alarm.Run(ctx, times, fn, alarm.WithPuller(puller.NewHorizon(time.Minute, 1000)))
//
// NewAhead holds n times read ahead, and NewHorizon the times due within d.
package puller
