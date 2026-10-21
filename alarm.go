package alarm

import (
	"context"
	"errors"
	"iter"
	"sync"
	"sync/atomic"
	"time"
)

// ErrClosed is returned by Ring after Close, and by every Close or Shutdown after the first.
var ErrClosed = errors.New("alarm closed")

// ErrRinging is returned by Ring while another Ring or Chan of the same Alarm is under way.
var ErrRinging = errors.New("alarm ringing")

// Alarm calls a func, or sends on a channel, at each time of a sequence: Ring, or Chan. One Ring or Chan runs at a
// time, which Reset, Close and Shutdown reach from any goroutine, fn included. Create one with New.
type Alarm struct {
	config

	// mu guards ringing, next and ending, and the closing of closed. Nothing locks it per time.
	mu sync.Mutex
	// ringing is closed once the Ring under way returns, and nil while none is.
	ringing chan struct{}
	// next is the sequence a Reset gave, until Ring takes it.
	next iter.Seq[time.Time]
	// ending is set once the Ring under way is to return, so that no Reset reaches it.
	ending bool
	// closed is closed by the first Close.
	closed chan struct{}
	// wake holds a token while next waits, if Reset found fire asleep, to end its wait.
	wake chan struct{}
	// interrupted is set by Reset and Close, so that each pass of fire sees them with a load rather than a lock.
	interrupted atomic.Bool
	// asleep is set while fire waits on the timer, so that Reset sends on wake only then: a channel locks.
	asleep atomic.Bool

	// The Ring under way owns these, kept for the next one.
	handle Handler
	done   <-chan struct{}
	// yield is given to each sequence, bound once per Ring: a range over one allocates its body each time.
	yield func(time.Time) bool
	// stopped is set once yield returned false, so that a sequence that yields again reads nothing more.
	stopped bool
	pending pending
	// timer is created by the first wait that needs it.
	timer *time.Timer
	// now is the clock read last, and released whether the holder released a time at it: the next is asked at now
	// first, and the clock read only if not due.
	now      time.Time
	released bool
}

// New returns an Alarm configured by options.
func New(options ...Option) *Alarm {
	return &Alarm{config: newConfig(options), closed: make(chan struct{}), wake: make(chan struct{}, 1)}
}

// Ring calls fn with ctx and each time of times once due, in the calling goroutine, until times ends, ctx is done or
// Close: neither fn nor its middlewares are called after. Lateness never adds up: each time is waited for itself, and
// a passed one fires at once, unless middleware.MaxDelay drops it. A time is read once the last fired, unless
// WithPuller reads it ahead. Reset replaces times meanwhile.
//
// It returns nil once times ends, ctx.Err() once ctx is done, and ErrClosed after Close. It returns ErrRinging at once
// while another Ring or Chan of a is under way.
func (a *Alarm) Ring(ctx context.Context, times iter.Seq[time.Time], fn func(ctx context.Context, at time.Time)) error {
	if err := a.start(); err != nil {
		return err
	}
	defer a.end()

	a.handle, a.done = chain(fn, a.middlewares...), ctx.Done()
	a.yield = func(at time.Time) bool { return a.push(ctx, at) }
	for {
		a.ring(ctx, times)
		next, err := a.take(ctx)
		if next == nil {
			return err
		}
		times = next
	}
}

// Chan is Ring sending each time on the returned channel from a goroutine of its own, and closing it after. A send
// blocks until read, Close or its ctx is done, delaying later times, so a reader that stops early must Close or cancel
// ctx. Where Ring returns an error at once, the channel closes at once.
func (a *Alarm) Chan(ctx context.Context, times iter.Seq[time.Time]) <-chan time.Time {
	ch := make(chan time.Time)
	go func() {
		defer close(ch)

		_ = a.Ring(ctx, times, func(ctx context.Context, at time.Time) {
			// Tried first: a select locks each of its channels.
			select {
			case ch <- at:
				return
			default:
			}
			select {
			case ch <- at:
			case <-ctx.Done():
			case <-a.closed:
			}
		})
	}()

	return ch
}

// Reset makes the Ring or Chan under way drop the times it holds and read times instead, once fn returns if called:
// times fired stay fired. A Reset as the sequence ends still takes. It reports whether a Ring was under way to take
// times, and ignores a nil times.
func (a *Alarm) Reset(times iter.Seq[time.Time]) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if times == nil || a.ringing == nil || a.ending || a.isClosed() {
		return false
	}
	a.next = times
	// Stored before asleep is loaded, as sleep stores asleep before loading interrupted: either sleep sees the Reset, or
	// the Reset wakes it.
	a.interrupted.Store(true)
	if a.asleep.Load() {
		select {
		case a.wake <- struct{}{}:
		default:
		}
	}

	return true
}

// Close makes the Ring or Chan under way return once fn returns if called, and any later one return ErrClosed. It
// never waits, so fn can call it. Every call after the first returns ErrClosed.
func (a *Alarm) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isClosed() {
		return ErrClosed
	}
	close(a.closed)
	a.interrupted.Store(true)

	return nil
}

// Shutdown is Close, then waits until the Ring under way, Chan's included, returns. It returns ctx.Err() if ctx is
// done first, else Close's error. Called from fn, it waits until ctx is done.
func (a *Alarm) Shutdown(ctx context.Context) error {
	err := a.Close()

	a.mu.Lock()
	ringing := a.ringing
	a.mu.Unlock()
	if ringing == nil {
		return err
	}

	// Checked first, since select picks at random among ready cases.
	select {
	case <-ringing:
		return err
	default:
	}

	select {
	case <-ringing:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// start marks a Ring under way, unless a is closed or another is.
func (a *Alarm) start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isClosed() {
		return ErrClosed
	}
	if a.ringing != nil {
		return ErrRinging
	}
	a.ringing = make(chan struct{})

	return nil
}

// ring reads times and fires each, until times ends and the times held fired, or fire stops.
func (a *Alarm) ring(ctx context.Context, times iter.Seq[time.Time]) {
	a.stopped = false
	times(a.yield)
	if a.stopped {
		return
	}
	// times ended: fire the ones held.
	a.fire(ctx, false)
}

// push holds at and fires the times held, for times to read the next if it returns true.
func (a *Alarm) push(ctx context.Context, at time.Time) bool {
	if a.stopped {
		return false
	}
	a.pending.push(at)
	a.stopped = !a.fire(ctx, true)

	return !a.stopped
}

// fire fires the times held as the holder releases them. It returns true once none is held, or once the puller reads
// the next, if read, and false once done, closed or reset.
func (a *Alarm) fire(ctx context.Context, read bool) bool {
	for a.pending.n > 0 {
		at := a.pending.first()
		sleep, due := a.release(at)
		// Passed times and spins never reach the timer's select.
		select {
		case <-a.done:
			return false
		default:
		}
		if a.interrupted.Load() {
			return false
		}
		if due {
			a.pending.pop()
			a.handle(ctx, at)

			continue
		}
		// A read keeps Ring from at: none while the holder spins. at not due, release read the clock for now.
		if read && sleep > 0 {
			wait, next := a.puller.Pull(a.pending.n-1, a.pending.last(), a.now)
			if next {
				return true
			}
			sleep = min(sleep, wait)
		}
		if sleep > 0 && !a.sleep(min(sleep, a.poll)) {
			return false
		}
	}

	// A Reset or Close from the last fn shows before the next read.
	return !a.interrupted.Load()
}

// release asks the holder whether at is due, at the clock read last if the holder released a time there, and only
// otherwise at the clock: a time due at now is due at any later one, so a run of passed times reads the clock once.
func (a *Alarm) release(at time.Time) (time.Duration, bool) {
	if a.released {
		if sleep, due := a.holder.Release(at, a.now); due {
			return sleep, true
		}
	}
	a.now = time.Now()
	sleep, due := a.holder.Release(at, a.now)
	a.released = due

	return sleep, due
}

// sleep waits on the timer for d. It returns true once d passed, and false once done, closed or reset.
func (a *Alarm) sleep(d time.Duration) bool {
	// Stored before interrupted is loaded, as Reset stores interrupted before loading asleep.
	a.asleep.Store(true)
	slept := false
	if !a.interrupted.Load() {
		if a.timer == nil {
			a.timer = time.NewTimer(d)
		} else {
			// No stale value to drain since Go 1.23.
			a.timer.Reset(d)
		}
		select {
		case <-a.timer.C:
			slept = true
		case <-a.done:
		case <-a.closed:
		case <-a.wake:
		}
	}
	a.asleep.Store(false)

	return slept
}

// take returns the sequence a Reset gave, once ring returned, dropping the times held. Otherwise it returns nil and
// Ring's error, after which no Reset reaches this Ring.
func (a *Alarm) take(ctx context.Context) (iter.Seq[time.Time], error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	err := ctx.Err()
	if err == nil && a.isClosed() {
		err = ErrClosed
	}
	if err != nil || a.next == nil {
		a.ending = true

		return nil, err
	}
	times := a.next
	a.drop()

	return times, nil
}

// end marks the Ring under way returned, dropping a Reset it did not take, stops the timer, and forgets the clock read
// last, so that the next Ring reads it.
func (a *Alarm) end() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.timer != nil {
		a.timer.Stop()
	}
	a.released = false
	a.drop()
	a.ending = false
	close(a.ringing)
	a.ringing = nil
}

// drop drops a Reset not taken yet, and the times held. Called under mu.
func (a *Alarm) drop() {
	a.next = nil
	a.interrupted.Store(a.isClosed())
	select {
	case <-a.wake:
	default:
	}
	a.pending.clear()
}

// isClosed reports whether Close was called. Called under mu, so that no Close runs meanwhile.
func (a *Alarm) isClosed() bool {
	select {
	case <-a.closed:
		return true
	default:
		return false
	}
}
