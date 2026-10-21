package alarm

import (
	"context"
	"iter"
	"time"
)

// Run calls fn with ctx and each time of times once due, in the calling goroutine, until times ends or ctx is done:
// neither fn nor its middlewares are called after. Lateness never adds up: each time is waited for itself, and a
// passed one fires at once, unless middleware.MaxDelay drops it. A time is read once the last fired, unless WithPuller
// reads it ahead.
//
// It returns ctx.Err().
func Run(ctx context.Context, times iter.Seq[time.Time], fn func(ctx context.Context, at time.Time), options ...Option) error {
	c := newConfig(options)
	r := runner{handle: chain(fn, c.middlewares...), holder: c.holder, puller: c.puller, poll: c.poll, done: ctx.Done()}
	defer r.stop()

	for at := range times {
		r.held.push(at)
		if !r.fire(ctx, true) {
			return ctx.Err()
		}
	}
	// times ended: fire the ones held.
	r.fire(ctx, false)

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

// runner fires the times of one Run, with one timer.
type runner struct {
	handle Handler
	holder Holder
	puller Puller
	poll   time.Duration
	done   <-chan struct{}
	held   held
	// timer is created by the first wait that needs it.
	timer *time.Timer
}

// fire fires the times held as the holder releases them. It returns true once none is held, or once the puller reads
// the next, if read, and false once done.
func (r *runner) fire(ctx context.Context, read bool) bool {
	for r.held.n > 0 {
		at := r.held.first()
		now := time.Now()
		sleep, due := r.holder.Release(at, now)
		// Passed times and spins never reach the timer's select.
		select {
		case <-r.done:
			return false
		default:
		}
		if due {
			r.held.pop()
			r.handle(ctx, at)

			continue
		}
		// A read keeps Run from at: none while the holder spins.
		if read && sleep > 0 {
			wait, next := r.puller.Pull(r.held.n-1, r.held.last(), now)
			if next {
				return true
			}
			sleep = min(sleep, wait)
		}
		if sleep <= 0 {
			continue
		}

		sleep = min(sleep, r.poll)
		if r.timer == nil {
			r.timer = time.NewTimer(sleep)
		} else {
			// No stale value to drain since Go 1.23.
			r.timer.Reset(sleep)
		}
		select {
		case <-r.timer.C:
		case <-r.done:
			return false
		}
	}

	return true
}

// stop releases the timer.
func (r *runner) stop() {
	if r.timer != nil {
		r.timer.Stop()
	}
}
