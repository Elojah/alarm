package middleware

import (
	"context"
	"time"

	"github.com/elojah/alarm"
)

// MaxDelay drops a time passed by more than d once reached, rather than firing it late. dropped, if not nil, gets each
// time dropped and how late. Keep d above the timer's resolution, a millisecond on Linux. d <= 0 means no limit.
func MaxDelay(d time.Duration, dropped func(ctx context.Context, at time.Time, late time.Duration)) alarm.Middleware {
	return func(next alarm.Handler) alarm.Handler {
		if d <= 0 {
			return next
		}

		return func(ctx context.Context, at time.Time) {
			if late := time.Since(at); late > d {
				if dropped != nil {
					dropped(ctx, at, late)
				}

				return
			}
			next(ctx, at)
		}
	}
}
