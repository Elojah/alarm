package alarm

import (
	"context"
	"iter"
	"time"
)

// maxSleep bounds a wait, so that a wall clock step or a suspend shows within it: timers run on the monotonic clock.
const maxSleep = time.Minute

// Run calls fn with each time of times once the clock shows it, in the calling goroutine, until times ends or ctx is
// done. Lateness never adds up: each time is waited for itself, and a passed one fires at once, unless WithMaxDelay
// drops it.
//
// It returns ctx.Err().
func Run(ctx context.Context, times iter.Seq[time.Time], fn func(at time.Time), options ...Option) error {
	w := waiter{config: newConfig(options), done: ctx.Done()}
	defer w.stop()

	for at := range times {
		late, ok := w.wait(at)
		if !ok {
			break
		}
		if late <= w.config.maxDelay {
			fn(at)
		}
	}

	return ctx.Err()
}

// Chan is Run sending each time on the returned channel from a goroutine of its own, and closing it after. A send
// blocks until read, delaying later times, so a reader that stops early must cancel ctx.
func Chan(ctx context.Context, times iter.Seq[time.Time], options ...Option) <-chan time.Time {
	ch := make(chan time.Time)
	go func() {
		defer close(ch)

		done := ctx.Done()
		_ = Run(ctx, times, func(at time.Time) {
			select {
			case ch <- at:
			case <-done:
			}
		}, options...)
	}()

	return ch
}

// waiter waits for the times of one Run, with one timer.
type waiter struct {
	config config
	done   <-chan struct{}
	// timer is created by the first wait that needs it.
	timer *time.Timer
}

// wait waits until the clock shows at and returns how late it is, or false once done.
func (w *waiter) wait(at time.Time) (time.Duration, bool) {
	for {
		// Wall clock, unless at has a monotonic reading.
		d := time.Until(at)
		// Passed times and spins never reach the timer's select.
		select {
		case <-w.done:
			return 0, false
		default:
		}
		if d <= 0 {
			return -d, true
		}
		if d <= w.config.spin {
			continue
		}

		d = min(d-w.config.spin, maxSleep)
		if w.timer == nil {
			w.timer = time.NewTimer(d)
		} else {
			// No stale value to drain since Go 1.23.
			w.timer.Reset(d)
		}
		select {
		case <-w.timer.C:
		case <-w.done:
			return 0, false
		}
	}
}

// stop releases the timer.
func (w *waiter) stop() {
	if w.timer != nil {
		w.timer.Stop()
	}
}
