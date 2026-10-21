package alarm

import (
	"math"
	"time"
)

// Option configures Run and Chan.
type Option func(c *config)

// WithMaxDelay drops a time passed by more than d when reached, instead of firing it late. Keep d above the timer's
// resolution, a millisecond on Linux.
func WithMaxDelay(d time.Duration) Option {
	return func(c *config) {
		c.maxDelay = d
	}
}

// WithSpin busy-waits for the last d before each time, instead of leaving it to the timer, which fires up to a
// millisecond late on Linux. It takes a core for up to d per time. Under TinyGo no other goroutine runs meanwhile, and
// in a synctest bubble it never ends.
func WithSpin(d time.Duration) Option {
	return func(c *config) {
		c.spin = d
	}
}

// config is what options set.
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
