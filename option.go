package alarm

import (
	"context"
	"math"
	"time"
)

// Option configures an Alarm, passed to New.
type Option func(c *config)

// WithAddLimit makes Add return false while n times added are held, not fired yet. n <= 0, the default, means none.
func WithAddLimit(n int) Option {
	return func(c *config) {
		c.addLimit = max(n, 0)
	}
}

// WithBuffer makes Chan's channel hold n times, so that a reader may lag n times, which then reach it late. Times
// buffered once Ring returns stay readable before the channel closes. n <= 0, the default, means none.
func WithBuffer(n int) Option {
	return func(c *config) {
		c.buffer = max(n, 0)
	}
}

// WithCoalesce makes Ring read the next time before it fires one passed by more than d, and drop the first if the next
// passed too: of a run of passed times, only the latest fires. dropped, if not nil, gets each time dropped and how
// late, in Ring's goroutine as fn does, and can Reset or Close. A read keeps Ring from the time due, so keep d above
// the timer's resolution, a millisecond on Linux. d <= 0, the default, means none.
func WithCoalesce(d time.Duration, dropped func(ctx context.Context, at time.Time, late time.Duration)) Option {
	return func(c *config) {
		c.coalesce, c.coalesced = max(d, 0), dropped
	}
}

// WithHolder makes h decide when each time is due, rather than once the clock shows it. Give each Alarm its own. A nil
// h is ignored.
func WithHolder(h Holder) Option {
	return func(c *config) {
		if h != nil {
			c.holder = h
		}
	}
}

// WithMiddleware appends middlewares around fn, or Chan's send, the first one outermost.
func WithMiddleware(middlewares ...Middleware) Option {
	return func(c *config) {
		c.middlewares = append(c.middlewares, middlewares...)
	}
}

// WithPoll makes Ring poll the clock at least every d while it waits, a minute by default: timers run on the monotonic
// clock, so a wall clock step or a suspend shows only at a poll. d <= 0 means never.
func WithPoll(d time.Duration) Option {
	return func(c *config) {
		if d <= 0 {
			d = math.MaxInt64
		}
		c.poll = d
	}
}

// WithPuller makes p decide when Ring reads each time ahead, rather than once it fired the last. Give each Alarm its
// own. A nil p is ignored.
func WithPuller(p Puller) Option {
	return func(c *config) {
		if p != nil {
			c.puller = p
		}
	}
}

// config is what options set.
type config struct {
	// addLimit is 0 while none limits Add.
	addLimit int
	buffer   int
	// coalesce is 0 while none is coalesced.
	coalesce    time.Duration
	coalesced   func(ctx context.Context, at time.Time, late time.Duration)
	holder      Holder
	middlewares []Middleware
	// poll is above 0, so that a sleep it bounds still is one.
	poll   time.Duration
	puller Puller
}

// newConfig applies options over the defaults.
func newConfig(options []Option) config {
	c := config{holder: onTime{}, poll: time.Minute, puller: lazy{}}
	for _, option := range options {
		option(&c)
	}

	return c
}
