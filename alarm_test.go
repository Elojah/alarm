package alarm_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/elojah/alarm"
	"github.com/elojah/alarm/holder"
	"github.com/elojah/alarm/middleware"
)

// call is a time fired and when, both from start.
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

// record appends each call to calls, then sleeps for the next of sleeps, if any.
func record(start time.Time, calls *[]call, sleeps ...time.Duration) func(context.Context, time.Time) {
	return func(_ context.Context, at time.Time) {
		*calls = append(*calls, call{at: at.Sub(start), now: time.Since(start)})
		if len(sleeps) > 0 {
			time.Sleep(sleeps[0])
			sleeps = sleeps[1:]
		}
	}
}

// Times fire on time, even past a poll.
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

// Passed times fire at once, in order, and a late call delays only the times it passes.
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

// Run stops once ctx is done, while waiting or firing passed times.
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
			err := alarm.Run(ctx, slices.Values(after(start, -2*time.Millisecond, -time.Millisecond, 0)), func(context.Context, time.Time) {
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

// fn gets Run's ctx.
func TestRun_Context(t *testing.T) {
	t.Parallel()

	type key struct{}
	ctx := context.WithValue(t.Context(), key{}, "run")
	calls := 0
	err := alarm.Run(ctx, slices.Values(after(time.Now(), -time.Millisecond)), func(ctx context.Context, _ time.Time) {
		calls++
		if got := ctx.Value(key{}); got != "run" {
			t.Errorf("got %v, want run", got)
		}
	})
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
	if calls != 1 {
		t.Errorf("got %d calls, want 1", calls)
	}
}

// Middlewares wrap fn, the first one outermost, nil ones skipped, and the ctx one hands down reaches fn.
func TestWithMiddleware(t *testing.T) {
	t.Parallel()

	type key struct{}
	var calls []string
	mark := func(name string) alarm.Middleware {
		return func(next alarm.Handler) alarm.Handler {
			return func(ctx context.Context, at time.Time) {
				calls = append(calls, name)
				next(context.WithValue(ctx, key{}, name), at)
			}
		}
	}
	err := alarm.Run(t.Context(), slices.Values(after(time.Now(), -time.Millisecond)), func(ctx context.Context, _ time.Time) {
		name, _ := ctx.Value(key{}).(string)
		calls = append(calls, "fn from "+name)
	}, alarm.WithMiddleware(mark("a"), nil), alarm.WithMiddleware(mark("b")))
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
	if want := []string{"a", "b", "fn from b"}; !slices.Equal(calls, want) {
		t.Errorf("got %v, want %v", calls, want)
	}
}

// early is a Holder releasing each time a millisecond before it.
type early struct{}

func (early) Release(at, now time.Time) (time.Duration, bool) {
	if d := at.Sub(now) - time.Millisecond; d > 0 {
		return d, false
	}

	return 0, true
}

// A holder decides when each time is due, and a nil one keeps the default.
func TestWithHolder(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		holder alarm.Holder
		want   []call
	}{
		{"Early", early{}, []call{{2 * time.Millisecond, time.Millisecond}, {5 * time.Millisecond, 4 * time.Millisecond}}},
		{"Nil", nil, []call{{2 * time.Millisecond, 2 * time.Millisecond}, {5 * time.Millisecond, 5 * time.Millisecond}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				var calls []call
				err := alarm.Run(t.Context(), slices.Values(after(start, 2*time.Millisecond, 5*time.Millisecond)),
					record(start, &calls), alarm.WithHolder(test.holder))
				if err != nil {
					t.Errorf("got %v, want nil", err)
				}
				if !slices.Equal(calls, test.want) {
					t.Errorf("got %v, want %v", calls, test.want)
				}
			})
		})
	}
}

// asks records when Run asks it, from start, answering as the default holder does.
type asks struct {
	start time.Time
	nows  []time.Duration
}

func (a *asks) Release(at, now time.Time) (time.Duration, bool) {
	a.nows = append(a.nows, now.Sub(a.start))
	if d := at.Sub(now); d > 0 {
		return d, false
	}

	return 0, true
}

// Run polls the holder at least every d while it waits, a minute by default, and with d <= 0 only once due.
func TestWithPoll(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		options []alarm.Option
		at      time.Duration
		want    []time.Duration
	}{
		{"Default", nil, 150 * time.Second, []time.Duration{0, time.Minute, 2 * time.Minute, 150 * time.Second}},
		{"Second", []alarm.Option{alarm.WithPoll(time.Second)}, 2500 * time.Millisecond, []time.Duration{0, time.Second, 2 * time.Second, 2500 * time.Millisecond}},
		{"Zero", []alarm.Option{alarm.WithPoll(0)}, 150 * time.Second, []time.Duration{0, 150 * time.Second}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				h := &asks{start: start}
				err := alarm.Run(t.Context(), slices.Values(after(start, test.at)), func(context.Context, time.Time) {},
					append([]alarm.Option{alarm.WithHolder(h)}, test.options...)...)
				if err != nil {
					t.Errorf("got %v, want nil", err)
				}
				if !slices.Equal(h.nows, test.want) {
					t.Errorf("got asked at %v, want %v", h.nows, test.want)
				}
			})
		})
	}
}

// A time passed by more than the max delay is dropped.
func TestMiddlewareMaxDelay(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		var calls []call
		times := after(start, -time.Hour, time.Millisecond, 2*time.Millisecond, 3*time.Millisecond, 4*time.Millisecond)
		err := alarm.Run(t.Context(), slices.Values(times), record(start, &calls, 2500*time.Microsecond),
			alarm.WithMiddleware(middleware.MaxDelay(time.Millisecond)))
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

// A spin never fires early, and stops once ctx is done. On the real clock: a spin never ends in a bubble.
func TestHolderSpin(t *testing.T) {
	t.Parallel()

	t.Run("Fire", func(t *testing.T) {
		t.Parallel()

		times := after(time.Now(), -time.Millisecond, time.Millisecond, 2*time.Millisecond, 5*time.Millisecond)
		calls := 0
		err := alarm.Run(t.Context(), slices.Values(times), func(_ context.Context, at time.Time) {
			calls++
			if now := time.Now(); now.Before(at) {
				t.Errorf("fired %v early", at.Sub(now))
			}
		}, alarm.WithHolder(holder.NewSpin(2*time.Millisecond)))
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
		err := alarm.Run(ctx, slices.Values(after(time.Now(), time.Hour)), func(context.Context, time.Time) { t.Error("called") },
			alarm.WithHolder(holder.NewSpin(2*time.Hour)))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("got %v, want %v", err, context.DeadlineExceeded)
		}
	})
}

// Times are sent on time, then the channel closes.
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

// The channel closes once ctx is done, even with times left.
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
