package alarm

import (
	"math"
	"time"
)

// Option configures an Alarm, passed to New.
type Option func(c *config)

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
