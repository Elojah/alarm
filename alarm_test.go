package alarm_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/elojah/alarm"
)

// call is a time fired and when it was, from the bubble's start.
type call struct {
	at, now time.Duration
}

// after returns start plus each of ds.
func after(start time.Time, ds ...time.Duration) []time.Time {
	times := make([]time.Time, len(ds))
	for i, d := range ds {
		times[i] = start.Add(d)
	}

	return times
}

// record returns a fn for Run that appends each call to calls, then sleeps for the next of sleeps, if any.
func record(start time.Time, calls *[]call, sleeps ...time.Duration) func(time.Time) {
	return func(at time.Time) {
		*calls = append(*calls, call{at: at.Sub(start), now: time.Since(start)})
		if len(sleeps) > 0 {
			time.Sleep(sleeps[0])
			sleeps = sleeps[1:]
		}
	}
}

// Each time fires once the clock shows it, even past maxSleep, and Run returns nil once times ends.
func TestRun(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		var calls []call
		err := alarm.Run(t.Context(), slices.Values(after(start, time.Millisecond, 2*time.Millisecond, 3*time.Minute)),
			record(start, &calls))
		if err != nil {
			t.Errorf("got %v, want nil", err)
		}
		want := []call{{time.Millisecond, time.Millisecond}, {2 * time.Millisecond, 2 * time.Millisecond}, {3 * time.Minute, 3 * time.Minute}}
		if !slices.Equal(calls, want) {
			t.Errorf("got %v, want %v", calls, want)
		}
	})
}

// A late call delays only the times it passed, and passed times fire at once, in order.
func TestRun_Late(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		var calls []call
		times := after(start, -2*time.Millisecond, -time.Millisecond, time.Millisecond, 2*time.Millisecond, 3*time.Millisecond)
		err := alarm.Run(t.Context(), slices.Values(times), record(start, &calls, 0, 0, 1500*time.Microsecond))
		if err != nil {
			t.Errorf("got %v, want nil", err)
		}
		want := []call{
			{-2 * time.Millisecond, 0},
			{-time.Millisecond, 0},
			{time.Millisecond, time.Millisecond},
			{2 * time.Millisecond, 2500 * time.Microsecond},
			{3 * time.Millisecond, 3 * time.Millisecond},
		}
		if !slices.Equal(calls, want) {
			t.Errorf("got %v, want %v", calls, want)
		}
	})
}

// Run returns once ctx is done, while it waits or during a run of passed times.
func TestRun_Cancel(t *testing.T) {
	t.Parallel()

	t.Run("Wait", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
			defer cancel()
			var calls []call
			err := alarm.Run(ctx, slices.Values(after(start, time.Hour)), record(start, &calls))
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("got %v, want %v", err, context.DeadlineExceeded)
			}
			if len(calls) != 0 || time.Since(start) != time.Millisecond {
				t.Errorf("got %v at %v, want none at 1ms", calls, time.Since(start))
			}
		})
	})
	t.Run("Passed", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			ctx, cancel := context.WithCancel(t.Context())
			calls := 0
			err := alarm.Run(ctx, slices.Values(after(start, -2*time.Millisecond, -time.Millisecond, 0)), func(time.Time) {
				calls++
				cancel()
			})
			if !errors.Is(err, context.Canceled) {
				t.Errorf("got %v, want %v", err, context.Canceled)
			}
			if calls != 1 {
				t.Errorf("got %d calls, want 1", calls)
			}
		})
	})
}

// A time passed by more than the max delay is dropped, the others fire.
func TestWithMaxDelay(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		var calls []call
		times := after(start, -time.Hour, time.Millisecond, 2*time.Millisecond, 3*time.Millisecond, 4*time.Millisecond)
		err := alarm.Run(t.Context(), slices.Values(times), record(start, &calls, 2500*time.Microsecond),
			alarm.WithMaxDelay(time.Millisecond))
		if err != nil {
			t.Errorf("got %v, want nil", err)
		}
		// The call at 1ms ends at 3.5ms: 2ms is 1.5ms late, 3ms only 0.5ms.
		want := []call{{time.Millisecond, time.Millisecond}, {3 * time.Millisecond, 3500 * time.Microsecond}, {4 * time.Millisecond, 4 * time.Millisecond}}
		if !slices.Equal(calls, want) {
			t.Errorf("got %v, want %v", calls, want)
		}
	})
}

// Spinning never fires a time early, and stops once ctx is done. On the real clock: a spin never ends in a bubble.
func TestWithSpin(t *testing.T) {
	t.Parallel()

	t.Run("Fire", func(t *testing.T) {
		t.Parallel()

		times := after(time.Now(), -time.Millisecond, time.Millisecond, 2*time.Millisecond, 5*time.Millisecond)
		calls := 0
		err := alarm.Run(t.Context(), slices.Values(times), func(at time.Time) {
			calls++
			if now := time.Now(); now.Before(at) {
				t.Errorf("fired %v early", at.Sub(now))
			}
		}, alarm.WithSpin(2*time.Millisecond))
		if err != nil {
			t.Errorf("got %v, want nil", err)
		}
		if calls != len(times) {
			t.Errorf("got %d calls, want %d", calls, len(times))
		}
	})
	t.Run("Cancel", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
		defer cancel()
		err := alarm.Run(ctx, slices.Values(after(time.Now(), time.Hour)), func(time.Time) { t.Error("called") },
			alarm.WithSpin(2*time.Hour))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("got %v, want %v", err, context.DeadlineExceeded)
		}
	})
}

// Each time is sent once the clock shows it, and the channel closes once times ends.
func TestChan(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		var calls []call
		for at := range alarm.Chan(t.Context(), slices.Values(after(start, time.Millisecond, 3*time.Millisecond))) {
			calls = append(calls, call{at: at.Sub(start), now: time.Since(start)})
		}
		want := []call{{time.Millisecond, time.Millisecond}, {3 * time.Millisecond, 3 * time.Millisecond}}
		if !slices.Equal(calls, want) {
			t.Errorf("got %v, want %v", calls, want)
		}
	})
}

// Once ctx is done, the channel closes, even with a time left.
func TestChan_Cancel(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		ctx, cancel := context.WithCancel(t.Context())
		ch := alarm.Chan(ctx, slices.Values(after(start, time.Millisecond, time.Hour)))
		<-ch
		cancel()
		if at, ok := <-ch; ok {
			t.Errorf("got %v, want closed", at.Sub(start))
		}
	})
}
