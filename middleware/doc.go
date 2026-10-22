// Package middleware holds ready-made alarm.Middleware for alarm.WithMiddleware, which wrap fn, or Chan's send:
//
//	err := alarm.New(alarm.WithMiddleware(
//		middleware.Recover(nil),
//		middleware.MaxDelay(time.Second, nil),
//	)).Ring(ctx, times, fn)
//
// Recover keeps a panic in fn, or in a later middleware, from ending Ring. MaxDelay drops the times passed too long
// ago, rather than firing them late.
package middleware
