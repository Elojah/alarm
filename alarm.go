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

// Alarm runs one Ring or Chan at a time, which Reset, Close and Shutdown reach from any goroutine, fn included.
// Create one with New.
type Alarm struct {
	config

	// mu guards ringing, next, ending and added, and the closing of closed. Nothing locks it per time of a sequence.
	mu sync.Mutex
	// ringing is closed once the Ring under way returns, and nil while none is.
	ringing chan struct{}
	// next is the sequence a Reset gave, until Ring takes it.
	next iter.Seq[time.Time]
	// ending is set once the Ring under way is to return: no Reset or Add reaches it.
	ending bool
	// added holds the times Add gave, not fired yet.
	added added
	// closed is closed by the first Close.
	closed chan struct{}
	// wake holds a token while next or a change to added waits, to wake sleep.
	wake chan struct{}
	// interrupted is set by Reset and Close, for fire to load at each pass rather than lock mu.
	interrupted atomic.Bool
	// changed is set by Add and Remove, for fire to load at each pass and copy added's first anew.
	changed atomic.Bool

	// The Ring under way owns these, kept for the next one.
	handle  Handler
	done    <-chan struct{}
	pending pending
	// soonest is Ring's copy of added's first, while adds is set: any time is added.
	soonest time.Time
	adds    bool
	// timer is created by the first wait that needs it.
	timer *time.Timer
}

// New returns an Alarm configured by options.
func New(options ...Option) *Alarm {
	return &Alarm{config: newConfig(options), closed: make(chan struct{}), wake: make(chan struct{}, 1)}
}

// Ring calls fn with ctx and each time of times once due, in the calling goroutine, until times ends, ctx is done or
// Close. Each time is waited for itself, so lateness from waits never adds up and a passed time fires at once, but an
// fn slower than the gap between times makes each later one later: WithCoalesce or middleware.MaxDelay drops them. A
// time is read once the last fired, unless WithPuller or WithCoalesce reads it ahead. Add fires more times in between.
//
// It returns nil once times ends and the times added fired, ctx.Err() once ctx is done, ErrClosed after Close, and
// ErrRinging at once while another Ring or Chan of a is under way.
func (a *Alarm) Ring(ctx context.Context, times iter.Seq[time.Time], fn func(ctx context.Context, at time.Time)) error {
	if err := a.start(); err != nil {
		return err
	}
	defer a.end()

	a.handle, a.done = chain(fn, a.middlewares...), ctx.Done()
	for {
		a.ring(ctx, times)
		next, err := a.take(ctx)
		if next == nil {
			return err
		}
		times = next
	}
}

// Chan is Ring sending each time on the returned channel, from a goroutine of its own, and closing it once Ring
// returns. A send blocks until read, or until WithBuffer's buffer has room, delaying later times: a reader that stops
// early must Close or cancel ctx.
func (a *Alarm) Chan(ctx context.Context, times iter.Seq[time.Time]) <-chan time.Time {
	ch := make(chan time.Time, a.buffer)
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

// Reset makes the Ring or Chan under way drop the times it holds, but not those added, and read times instead, once
// the fn call in progress returns. It reports whether a Ring will read times: one whose sequence just ended still
// does. A nil times is ignored.
func (a *Alarm) Reset(times iter.Seq[time.Time]) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if times == nil || a.ringing == nil || a.ending || a.isClosed() {
		return false
	}
	a.next = times
	a.interrupted.Store(true)
	a.wakeUp()

	return true
}

// Add makes the Ring or Chan under way fire at too, in order with its sequence's times, even after a Reset: a passed
// at fires once the fn call in progress returns, and the zero time before any other, as the time it fires, so that no
// middleware.MaxDelay drops it. A Ring whose sequence ends returns once the times added fired. It reports whether a
// Ring will fire at, unless Remove, ctx is done or Close: false with none under way, once it returns, or with
// WithAddLimit's n held.
func (a *Alarm) Add(at time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.ringing == nil || a.ending || a.isClosed() || (a.addLimit > 0 && a.added.n() >= a.addLimit) {
		return false
	}
	a.added.push(at)
	a.changed.Store(true)
	a.wakeUp()

	return true
}

// Remove takes back a time Add gave, Equal to at, not fired yet: one per call, of those Equal. It reports whether one
// was held, which then does not fire.
func (a *Alarm) Remove(at time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	i := a.added.index(at)
	if i < 0 {
		return false
	}
	a.added.remove(i)
	a.changed.Store(true)
	// Else a Ring whose sequence ended would sleep until at.
	a.wakeUp()

	return true
}

// Close makes the Ring or Chan under way return once the fn call in progress does, and later ones return ErrClosed.
// It never waits, so fn can call it. Every Close after the first returns ErrClosed.
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
	for at := range times {
		a.pending.push(at)
		if !a.fire(ctx, true) {
			return
		}
	}
	// times ended: fire the ones held.
	a.fire(ctx, false)
}

// fire fires the times held and added, the earliest first, as the holder releases them. It returns true once none is
// held and, if not read, none is added, or, if read, the puller or WithCoalesce asks for the next, and false once
// done, closed or reset.
func (a *Alarm) fire(ctx context.Context, read bool) bool {
	for {
		// Passed times and spins never reach the timer's select, and a Reset or Close from the last fn shows before
		// the next read.
		select {
		case <-a.done:
			return false
		default:
		}
		if a.interrupted.Load() {
			return false
		}
		if a.changed.Load() {
			a.reload()
		}
		if a.pending.n == 0 && (read || !a.adds) {
			return true
		}
		now := time.Now()
		at, add := a.earliest(now)
		sleep, due := a.holder.Release(at, now)
		if due && add {
			// Else an Add or a Remove came since the copy: at may be gone, or another due before it.
			if a.popSoonest() {
				a.handle(ctx, at)
			}

			continue
		}
		if due {
			// Coalescing: the next is read first, and at dropped if the next passed too.
			if a.coalesce > 0 && now.Sub(at) > a.coalesce {
				if a.pending.n == 1 && read {
					return true
				}
				if a.pending.n > 1 && !a.pending.second().After(now) {
					a.pending.pop()
					if a.coalesced != nil {
						a.coalesced(ctx, at, now.Sub(at))
					}

					continue
				}
			}
			a.pending.pop()
			a.handle(ctx, at)

			continue
		}
		// A read keeps Ring from at: none while the holder spins.
		if read && sleep > 0 {
			wait, next := a.puller.Pull(a.pending.n-1, a.pending.last(), now)
			if next {
				return true
			}
			sleep = min(sleep, wait)
		}
		if sleep > 0 {
			a.sleep(min(sleep, a.poll))
		}
	}
}

// earliest returns the time Ring waits for, the earliest of the first held and the first added, on a tie the one
// held, and whether it is added. The zero time added is now. Called with a time held or added.
func (a *Alarm) earliest(now time.Time) (time.Time, bool) {
	if !a.adds || (a.pending.n > 0 && !a.soonest.Before(a.pending.first())) {
		return a.pending.first(), false
	}
	if a.soonest.IsZero() {
		return now, true
	}

	return a.soonest, true
}

// sleep waits on the timer for d, or until done, closed, reset or added changed: fire checks which.
func (a *Alarm) sleep(d time.Duration) {
	if a.timer == nil {
		a.timer = time.NewTimer(d)
	} else {
		// No stale value to drain since Go 1.23.
		a.timer.Reset(d)
	}
	select {
	case <-a.timer.C:
	case <-a.done:
	case <-a.closed:
	case <-a.wake:
	}
}

// reload copies added's first anew, since an Add or a Remove changed it.
func (a *Alarm) reload() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.copySoonest()
	// The token is a Reset's while next waits.
	if a.next == nil {
		select {
		case <-a.wake:
		default:
		}
	}
}

// popSoonest drops added's first and copies the next, unless an Add or a Remove came since Ring copied it. It reports
// whether it dropped it.
func (a *Alarm) popSoonest() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.changed.Load() {
		return false
	}
	a.added.pop()
	a.copySoonest()

	return true
}

// copySoonest copies added's first for Ring. Called under mu, so that no Add or Remove runs meanwhile.
func (a *Alarm) copySoonest() {
	a.changed.Store(false)
	a.adds = a.added.n() > 0
	if a.adds {
		a.soonest = a.added.first()
	}
}

// take returns the sequence a Reset gave, dropping the times held, none if a time was added as the sequence ended, or
// else nil and Ring's error, after which no Reset or Add reaches this Ring.
func (a *Alarm) take(ctx context.Context) (iter.Seq[time.Time], error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	err := ctx.Err()
	if err == nil && a.isClosed() {
		err = ErrClosed
	}
	if err == nil && a.next == nil && a.added.n() > 0 {
		return none, nil
	}
	if err != nil || a.next == nil {
		a.ending = true

		return nil, err
	}
	times := a.next
	a.drop()

	return times, nil
}

// end marks the Ring under way returned, dropping a Reset it did not take and the times added, and stops the timer.
func (a *Alarm) end() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.timer != nil {
		a.timer.Stop()
	}
	a.drop()
	a.added.clear()
	a.copySoonest()
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

// wakeUp wakes the sleep under way, or else the next one. Called under mu.
func (a *Alarm) wakeUp() {
	select {
	case a.wake <- struct{}{}:
	default:
	}
}

// none is the empty sequence Ring rings to fire the times added as its own ended.
func none(func(time.Time) bool) {}

// isClosed reports whether Close was called. Called under mu, so that no Close runs meanwhile.
func (a *Alarm) isClosed() bool {
	select {
	case <-a.closed:
		return true
	default:
		return false
	}
}
