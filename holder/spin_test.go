package holder_test

import (
	"testing"
	"time"

	"github.com/elojah/alarm/holder"
)

// Release sleeps until the last d before a time, spins within it, and releases the time once passed.
func TestSpin(t *testing.T) {
	t.Parallel()

	at := time.Unix(0, 0)
	spin := holder.NewSpin(time.Millisecond)
	for _, test := range []struct {
		name  string
		until time.Duration
		sleep time.Duration
		due   bool
	}{
		{"Before", 5 * time.Millisecond, 4 * time.Millisecond, false},
		{"Spin", 500 * time.Microsecond, 0, false},
		{"At", 0, 0, true},
		{"Passed", -time.Millisecond, 0, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			sleep, due := spin.Release(at, at.Add(-test.until))
			if sleep != test.sleep || due != test.due {
				t.Errorf("got %v, %v, want %v, %v", sleep, due, test.sleep, test.due)
			}
		})
	}
}
