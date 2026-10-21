package alarm

import (
	"context"
	"iter"
	"math"
	"time"
)

// maxSleep bounds each wait, so that the wall clock is read again at least this often: timers run on the monotonic
// clock, which a wall clock step leaves alone, and which on Linux stops while the system is suspended.
const maxSleep = time.Minute

// Option configures Run and Chan.
type Option func(c *config)

// WithMaxDelay drops a time that the clock has passed by more than d when it is reached, such as after fn took longer
// than the gap to the next times, instead of firing it late. d should be more than the timer's resolution: a timer
// fires up to a millisecond late on Linux.
func WithMaxDelay(d time.Duration) Option {
	return func(c *config) {
		c.maxDelay = d
	}
}

// WithSpin busy-waits for the last d before each time, reading the clock, rather than leaving it to the timer, which
// fires up to a millisecond late on Linux: the timer wakes Run d before the time. It takes a core for up to d before
// each time, so a whole one for times every d or closer. Under TinyGo, whose scheduler only switches goroutines when
// one blocks, no other goroutine runs meanwhile. In a synctest bubble, whose clock only moves once every goroutine is
// blocked, it never ends.
func WithSpin(d time.Duration) Option {
	return func(c *config) {
		c.spin = d
	}
}

// Run calls fn with each time of times once the clock shows it, in order, in the calling goroutine, until times ends or
// ctx is done. Run waits for each time, not for the gap from the last one, so a late call, fn taking long or the
// timer firing late, does not delay the next ones: a time already passed fires at once, unless WithMaxDelay drops it.
//
// It returns ctx's error once ctx is done, or nil once times ends.
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

// Chan sends on the channel it returns each time of times once the clock shows it, as Run calls fn, from a goroutine
// of its own, and closes it once times ends or ctx is done. A send waits for the reader, so later times fire late
// while it does, and a reader that stops early must cancel ctx.
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

// config is what Options set.
type config struct {
	maxDelay time.Duration
	spin     time.Duration
}

// newConfig applies options over the defaults.
func newConfig(options []Option) config {
	c := config{maxDelay: math.MaxInt64}
	for _, option := range options {
		option(&c)
	}

	return c
}

// waiter waits for each time of one Run, with one timer.
type waiter struct {
	config config
	done   <-chan struct{}
	// timer is created by the first wait that needs it.
	timer *time.Timer
}

// wait waits until the clock shows at, and returns by how much it has passed it, or false once done is closed.
func (w *waiter) wait(at time.Time) (time.Duration, bool) {
	for {
		// The wall clock's difference, unless at has a monotonic reading, as those from time.Now do.
		d := time.Until(at)
		// Stops a run of passed times, or a spin, which never reach the select below.
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
			// Since Go 1.23, Reset leaves no stale value in C to drain.
			w.timer.Reset(d)
		}
		select {
		case <-w.timer.C:
		case <-w.done:
			return 0, false
		}
	}
}

// stop releases the timer, once Run returns.
func (w *waiter) stop() {
	if w.timer != nil {
		w.timer.Stop()
	}
}
