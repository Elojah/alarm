package middleware

import (
	"context"
	"time"

	"github.com/elojah/alarm"
)

// MaxDelay drops a time passed by more than d once reached, rather than firing it late. Keep d above the timer's
// resolution, a millisecond on Linux. d <= 0 means no limit.
func MaxDelay(d time.Duration) alarm.Middleware {
	return func(next alarm.Handler) alarm.Handler {
		if d <= 0 {
			return next
		}

		return func(ctx context.Context, at time.Time) {
			if time.Since(at) > d {
				return
			}
			next(ctx, at)
		}
	}
}
