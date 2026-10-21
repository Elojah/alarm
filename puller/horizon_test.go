package puller_test

import (
	"math"
	"testing"
	"time"

	"github.com/elojah/alarm/puller"
)

// Pull reads once the last time read is due within d, until then waits for it, and never with n held ahead or d <= 0.
func TestHorizon(t *testing.T) {
	t.Parallel()

	now := time.Unix(0, 0)
	for _, test := range []struct {
		name     string
		d        time.Duration
		n, ahead int
		until    time.Duration
		wait     time.Duration
		pull     bool
	}{
		{"After", 5 * time.Millisecond, 0, 0, 8 * time.Millisecond, 3 * time.Millisecond, false},
		{"Within", 5 * time.Millisecond, 0, 3, 5 * time.Millisecond, 0, true},
		{"Passed", 5 * time.Millisecond, 0, 0, -time.Hour, 0, true},
		{"LongPassed", 5 * time.Millisecond, 0, 0, math.MinInt64, 0, true},
		{"Below", 5 * time.Millisecond, 2, 1, time.Millisecond, 0, true},
		{"Full", 5 * time.Millisecond, 2, 2, time.Millisecond, math.MaxInt64, false},
		{"Zero", 0, 0, 0, -time.Hour, math.MaxInt64, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			wait, pull := puller.NewHorizon(test.d, test.n).Pull(test.ahead, now.Add(test.until), now)
			if wait != test.wait || pull != test.pull {
				t.Errorf("got %v, %v, want %v, %v", wait, pull, test.wait, test.pull)
			}
		})
	}
}
