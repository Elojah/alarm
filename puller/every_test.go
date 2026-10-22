package puller_test

import (
	"testing"
	"time"

	"github.com/elojah/alarm/puller"
)

// pull is a call to Pull, with last and now from start, and what it returns.
type pull struct {
	name      string
	ahead     int
	last, now time.Duration
	wait      time.Duration
	pull      bool
}

// Pull asks p from a tick until it reads none, then waits for the next tick, on the first one's grid unless one was
// missed. With d <= 0, p decides alone.
func TestEvery(t *testing.T) {
	t.Parallel()

	start := time.Unix(0, 0)
	for _, test := range []struct {
		name  string
		every *puller.Every
		pulls []pull
	}{
		{"Ticks", puller.NewEvery(15*time.Minute, puller.NewAhead(2)), []pull{
			{"Tick", 0, 0, 0, 0, true},
			{"Reading", 1, 0, 0, 0, true},
			{"Read", 2, 0, 0, 15 * time.Minute, false},
			{"BeforeTick", 0, 0, 10 * time.Minute, 5 * time.Minute, false},
			{"LateTick", 0, 0, 15*time.Minute + time.Millisecond, 0, true},
			{"OnGrid", 2, 0, 15*time.Minute + time.Millisecond, 15*time.Minute - time.Millisecond, false},
			{"Missed", 0, 0, 70 * time.Minute, 0, true},
			{"FromMissed", 2, 0, 70 * time.Minute, 15 * time.Minute, false},
		}},
		{"Zero", puller.NewEvery(0, puller.NewHorizon(5*time.Millisecond, 0)), []pull{
			{"Within", 0, 5 * time.Millisecond, 0, 0, true},
			{"After", 1, 8 * time.Millisecond, 0, 3 * time.Millisecond, false},
			{"Again", 0, 5 * time.Millisecond, 0, 0, true},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for _, p := range test.pulls {
				wait, pull := test.every.Pull(p.ahead, start.Add(p.last), start.Add(p.now))
				if wait != p.wait || pull != p.pull {
					t.Errorf("%s: got %v, %v, want %v, %v", p.name, wait, pull, p.wait, p.pull)
				}
			}
		})
	}
}
