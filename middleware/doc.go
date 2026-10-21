// Package middleware holds ready-made alarm.Middleware for alarm.WithMiddleware, which wrap fn, or Chan's send:
//
//	err := alarm.Run(ctx, times, fn, alarm.WithMiddleware(middleware.MaxDelay(time.Second)))
//
// MaxDelay drops the times passed too long ago, rather than firing them late.
package middleware
