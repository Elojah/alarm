package puller_test

import (
	"math"
	"testing"
	"time"

	"github.com/elojah/alarm/puller"
)

// Pull reads while fewer than n times are held ahead, and with n <= 0 never.
func TestAhead(t *testing.T) {
	t.Parallel()

	now := time.Unix(0, 0)
	for _, test := range []struct {
		name     string
		n, ahead int
		wait     time.Duration
		pull     bool
	}{
		{"None", 2, 0, 0, true},
		{"Below", 2, 1, 0, true},
		{"Full", 2, 2, math.MaxInt64, false},
		{"Zero", 0, 0, math.MaxInt64, false},
		{"Negative", -1, 0, math.MaxInt64, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			wait, pull := puller.NewAhead(test.n).Pull(test.ahead, now.Add(time.Hour), now)
			if wait != test.wait || pull != test.pull {
				t.Errorf("got %v, %v, want %v, %v", wait, pull, test.wait, test.pull)
			}
		})
	}
}
