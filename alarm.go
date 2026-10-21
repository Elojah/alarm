package alarm

import (
	"context"
	"iter"
	"time"
)

// Run calls fn with ctx and each time of times once due, in the calling goroutine, until times ends or ctx is done:
// neither fn nor its middlewares are called after. Lateness never adds up: each time is waited for itself, and a
// passed one fires at once, unless middleware.MaxDelay drops it.
//
// It returns ctx.Err().
func Run(ctx context.Context, times iter.Seq[time.Time], fn func(ctx context.Context, at time.Time), options ...Option) error {
	c := newConfig(options)
	handle := chain(fn, c.middlewares...)
	w := waiter{holder: c.holder, poll: c.poll, done: ctx.Done()}
	defer w.stop()

	for at := range times {
		if !w.wait(at) {
			break
		}
		handle(ctx, at)
	}

	return ctx.Err()
}

// Chan is Run sending each time on the returned channel from a goroutine of its own, and closing it after. A send
// blocks until read or its ctx is done, delaying later times, so a reader that stops early must cancel ctx.
func Chan(ctx context.Context, times iter.Seq[time.Time], options ...Option) <-chan time.Time {
	ch := make(chan time.Time)
	go func() {
		defer close(ch)

		_ = Run(ctx, times, func(ctx context.Context, at time.Time) {
			select {
			case ch <- at:
			case <-ctx.Done():
			}
		}, options...)
	}()

	return ch
}

// waiter waits for the times of one Run, with one timer.
type waiter struct {
	holder Holder
	poll   time.Duration
	done   <-chan struct{}
	// timer is created by the first wait that needs it.
	timer *time.Timer
}

// wait waits until the holder releases at, or returns false once done.
func (w *waiter) wait(at time.Time) bool {
	for {
		sleep, due := w.holder.Release(at, time.Now())
		// Passed times and spins never reach the timer's select.
		select {
		case <-w.done:
			return false
		default:
		}
		if due {
			return true
		}
		if sleep <= 0 {
			continue
		}

		sleep = min(sleep, w.poll)
		if w.timer == nil {
			w.timer = time.NewTimer(sleep)
		} else {
			// No stale value to drain since Go 1.23.
			w.timer.Reset(sleep)
		}
		select {
		case <-w.timer.C:
		case <-w.done:
			return false
		}
	}
}

// stop releases the timer.
func (w *waiter) stop() {
	if w.timer != nil {
		w.timer.Stop()
	}
}
