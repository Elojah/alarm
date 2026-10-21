package alarm

import (
	"math"
	"time"
)

// Option configures Run and Chan.
type Option func(c *config)

// WithHolder makes h decide when each time is due, rather than once the clock shows it. Give each Run its own. A nil h
// is ignored.
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

// WithPoll makes Run poll the clock at least every d while it waits, a minute by default, so that a wall clock step or
// a suspend shows within d: timers run on the monotonic clock. d <= 0 means never.
func WithPoll(d time.Duration) Option {
	return func(c *config) {
		if d <= 0 {
			d = math.MaxInt64
		}
		c.poll = d
	}
}

// config is what options set.
type config struct {
	holder      Holder
	middlewares []Middleware
	// poll is above 0, so that a sleep it bounds still is one.
	poll time.Duration
}

// newConfig applies options over the defaults.
func newConfig(options []Option) config {
	c := config{holder: onTime{}, poll: time.Minute}
	for _, option := range options {
		option(&c)
	}

	return c
}
