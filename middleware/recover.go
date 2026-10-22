package middleware

import (
	"context"
	"time"

	"github.com/elojah/alarm"
)

// Recover recovers a panic in the Handler it wraps, which would end Ring, and Ring rings on. recovered, if not nil,
// gets the time and the value before the stack unwinds, so that debug.Stack shows the panic. Put it first to recover
// the later middlewares too.
func Recover(recovered func(ctx context.Context, at time.Time, v any)) alarm.Middleware {
	return func(next alarm.Handler) alarm.Handler {
		return func(ctx context.Context, at time.Time) {
			defer func() {
				// panic(nil) recovers as a *runtime.PanicNilError.
				if v := recover(); v != nil && recovered != nil {
					recovered(ctx, at, v)
				}
			}()
			next(ctx, at)
		}
	}
}
