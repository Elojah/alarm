package alarm_test

import (
	"context"
	"errors"
	"iter"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/elojah/alarm"
	"github.com/elojah/alarm/holder"
	"github.com/elojah/alarm/middleware"
	"github.com/elojah/alarm/puller"
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

// event is a time read from the sequence or fired, and when, both from start.
type event struct {
	op      string
	at, now time.Duration
}

// reading yields start plus each of ds, logging each read to events.
func reading(start time.Time, events *[]event, ds ...time.Duration) iter.Seq[time.Time] {
	return func(yield func(time.Time) bool) {
		for _, d := range ds {
			*events = append(*events, event{op: "read", at: d, now: time.Since(start)})
			if !yield(start.Add(d)) {
				return
			}
		}
	}
}

// firing logs each time fired to events.
func firing(start time.Time, events *[]event) func(context.Context, time.Time) {
	return func(_ context.Context, at time.Time) {
		*events = append(*events, event{op: "fire", at: at.Sub(start), now: time.Since(start)})
	}
}

// dropping logs each time dropped to events, when at plus how late it was dropped, both from start.
func dropping(start time.Time, events *[]event) func(context.Context, time.Time, time.Duration) {
	return func(_ context.Context, at time.Time, late time.Duration) {
		*events = append(*events, event{op: "drop", at: at.Sub(start), now: at.Sub(start) + late})
	}
}

// Times fire on time, even past a poll.
func TestRing(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		var calls []call
		err := alarm.New().Ring(t.Context(), slices.Values(after(start, time.Millisecond, 2*time.Millisecond, 3*time.Minute)),
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
func TestRing_Late(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		var calls []call
		times := after(start, -2*time.Millisecond, -time.Millisecond, time.Millisecond, 2*time.Millisecond, 3*time.Millisecond)
		err := alarm.New().Ring(t.Context(), slices.Values(times), record(start, &calls, 0, 0, 1500*time.Microsecond))
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

// Ring stops once ctx is done, while waiting, firing passed times, or with times held once the sequence ended.
func TestRing_Cancel(t *testing.T) {
	t.Parallel()

	t.Run("Wait", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
			defer cancel()
			var calls []call
			err := alarm.New().Ring(ctx, slices.Values(after(start, time.Hour)), record(start, &calls))
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
			err := alarm.New().Ring(ctx, slices.Values(after(start, -2*time.Millisecond, -time.Millisecond, 0)), func(context.Context, time.Time) {
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
	t.Run("Held", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Millisecond)
			defer cancel()
			var calls []call
			err := alarm.New(alarm.WithPuller(puller.NewAhead(5))).Ring(ctx, slices.Values(after(start, time.Millisecond, time.Hour, 2*time.Hour)),
				record(start, &calls))
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("got %v, want %v", err, context.DeadlineExceeded)
			}
			if want := []call{{time.Millisecond, time.Millisecond}}; !slices.Equal(calls, want) || time.Since(start) != 2*time.Millisecond {
				t.Errorf("got %v at %v, want %v at 2ms", calls, time.Since(start), want)
			}
		})
	})
}

// fn gets Ring's ctx.
func TestRing_Context(t *testing.T) {
	t.Parallel()

	type key struct{}
	ctx := context.WithValue(t.Context(), key{}, "run")
	calls := 0
	err := alarm.New().Ring(ctx, slices.Values(after(time.Now(), -time.Millisecond)), func(ctx context.Context, _ time.Time) {
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

// ringing runs a.Ring in a goroutine of its own, sending its error on the returned channel.
func ringing(ctx context.Context, a *alarm.Alarm, times iter.Seq[time.Time], fn func(context.Context, time.Time)) <-chan error {
	errs := make(chan error, 1)
	go func() {
		errs <- a.Ring(ctx, times, fn)
	}()

	return errs
}

// A Ring while another is under way returns ErrRinging, and once that one returned the Alarm rings again.
func TestRing_Twice(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		var calls []call
		a := alarm.New()
		errs := ringing(t.Context(), a, slices.Values(after(start, time.Millisecond)), record(start, &calls))
		synctest.Wait()
		if err := a.Ring(t.Context(), slices.Values(after(start, 2*time.Millisecond)), record(start, &calls)); !errors.Is(err, alarm.ErrRinging) {
			t.Errorf("got %v, want %v", err, alarm.ErrRinging)
		}
		if err := <-errs; err != nil {
			t.Errorf("got %v, want nil", err)
		}
		if err := a.Ring(t.Context(), slices.Values(after(start, 2*time.Millisecond)), record(start, &calls)); err != nil {
			t.Errorf("got %v, want nil", err)
		}
		if want := []call{{time.Millisecond, time.Millisecond}, {2 * time.Millisecond, 2 * time.Millisecond}}; !slices.Equal(calls, want) {
			t.Errorf("got %v, want %v", calls, want)
		}
	})
}

// Reset drops the times held and reads the new sequence at once, while Ring waits, reads or from fn, even as the
// sequence ends, and returns false with no Ring to take it.
func TestReset(t *testing.T) {
	t.Parallel()

	t.Run("Wait", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var events []event
			a := alarm.New()
			errs := ringing(t.Context(), a, reading(start, &events, time.Hour, 2*time.Hour), firing(start, &events))
			time.Sleep(time.Millisecond)
			if !a.Reset(reading(start, &events, 2*time.Millisecond, 3*time.Millisecond)) {
				t.Error("got false, want true")
			}
			if err := <-errs; err != nil {
				t.Errorf("got %v, want nil", err)
			}
			want := []event{
				{"read", time.Hour, 0},
				{"read", 2 * time.Millisecond, time.Millisecond},
				{"fire", 2 * time.Millisecond, 2 * time.Millisecond},
				{"read", 3 * time.Millisecond, 2 * time.Millisecond},
				{"fire", 3 * time.Millisecond, 3 * time.Millisecond},
			}
			if !slices.Equal(events, want) {
				t.Errorf("got %v, want %v", events, want)
			}
		})
	})
	t.Run("Read", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var events []event
			// Its read takes 2ms: the Reset reaches Ring while it reads, not while it waits, and the time read, though
			// due, is dropped.
			slow := func(yield func(time.Time) bool) {
				time.Sleep(2 * time.Millisecond)
				events = append(events, event{"read", 2 * time.Millisecond, time.Since(start)})
				yield(start.Add(2 * time.Millisecond))
			}
			a := alarm.New()
			errs := ringing(t.Context(), a, slow, firing(start, &events))
			time.Sleep(time.Millisecond)
			if !a.Reset(reading(start, &events, 3*time.Millisecond)) {
				t.Error("got false, want true")
			}
			if err := <-errs; err != nil {
				t.Errorf("got %v, want nil", err)
			}
			want := []event{
				{"read", 2 * time.Millisecond, 2 * time.Millisecond},
				{"read", 3 * time.Millisecond, 2 * time.Millisecond},
				{"fire", 3 * time.Millisecond, 3 * time.Millisecond},
			}
			if !slices.Equal(events, want) {
				t.Errorf("got %v, want %v", events, want)
			}
		})
	})
	for _, test := range []struct {
		name    string
		options []alarm.Option
		ds      []time.Duration
		reads   []event
	}{
		{"Next", nil, []time.Duration{time.Millisecond, time.Hour}, []event{{"read", time.Millisecond, 0}}},
		{"End", nil, []time.Duration{time.Millisecond}, []event{{"read", time.Millisecond, 0}}},
		{"Held", []alarm.Option{alarm.WithPuller(puller.NewAhead(2))}, []time.Duration{time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond}, []event{
			{"read", time.Millisecond, 0},
			{"read", 2 * time.Millisecond, 0},
			{"read", 3 * time.Millisecond, 0},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				var events []event
				fire := firing(start, &events)
				a := alarm.New(test.options...)
				err := a.Ring(t.Context(), reading(start, &events, test.ds...), func(ctx context.Context, at time.Time) {
					fire(ctx, at)
					if at.Equal(start.Add(time.Millisecond)) && !a.Reset(reading(start, &events, 5*time.Millisecond)) {
						t.Error("got false, want true")
					}
				})
				if err != nil {
					t.Errorf("got %v, want nil", err)
				}
				want := append(test.reads,
					event{"fire", time.Millisecond, time.Millisecond},
					event{"read", 5 * time.Millisecond, time.Millisecond},
					event{"fire", 5 * time.Millisecond, 5 * time.Millisecond},
				)
				if !slices.Equal(events, want) {
					t.Errorf("got %v, want %v", events, want)
				}
			})
		})
	}
	t.Run("Idle", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			times := slices.Values(after(time.Now(), time.Millisecond))
			a := alarm.New()
			if a.Reset(times) {
				t.Error("before Ring: got true, want false")
			}
			if err := a.Ring(t.Context(), times, func(context.Context, time.Time) {}); err != nil {
				t.Errorf("got %v, want nil", err)
			}
			if a.Reset(times) {
				t.Error("after Ring: got true, want false")
			}
			errs := ringing(t.Context(), a, slices.Values(after(time.Now(), time.Hour)), func(context.Context, time.Time) {})
			synctest.Wait()
			if a.Reset(nil) {
				t.Error("nil: got true, want false")
			}
			if err := a.Close(); err != nil {
				t.Errorf("got %v, want nil", err)
			}
			if a.Reset(times) {
				t.Error("after Close: got true, want false")
			}
			if err := <-errs; !errors.Is(err, alarm.ErrClosed) {
				t.Errorf("got %v, want %v", err, alarm.ErrClosed)
			}
		})
	})
}

// Times added fire in order with the sequence's, from outside or fn, the passed ones at once and the zero time as the
// time it fires, kept by Reset, and Ring returns once they fired. A far one holds back neither the sequence nor its
// reads.
func TestAdd(t *testing.T) {
	t.Parallel()

	t.Run("Merged", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var calls []call
			a := alarm.New()
			errs := ringing(t.Context(), a, slices.Values(after(start, time.Hour, 3*time.Hour)), record(start, &calls))
			synctest.Wait()
			if !a.Add(start.Add(2 * time.Hour)) {
				t.Error("got false, want true")
			}
			if err := <-errs; err != nil {
				t.Errorf("got %v, want nil", err)
			}
			if want := []call{{time.Hour, time.Hour}, {2 * time.Hour, 2 * time.Hour}, {3 * time.Hour, 3 * time.Hour}}; !slices.Equal(calls, want) {
				t.Errorf("got %v, want %v", calls, want)
			}
		})
	})
	t.Run("Passed", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var calls []call
			a := alarm.New()
			errs := ringing(t.Context(), a, slices.Values(after(start, time.Hour)), record(start, &calls))
			time.Sleep(10 * time.Minute)
			if !a.Add(time.Now()) {
				t.Error("got false, want true")
			}
			if err := <-errs; err != nil {
				t.Errorf("got %v, want nil", err)
			}
			if want := []call{{10 * time.Minute, 10 * time.Minute}, {time.Hour, time.Hour}}; !slices.Equal(calls, want) {
				t.Errorf("got %v, want %v", calls, want)
			}
		})
	})
	t.Run("Zero", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var events []event
			fire := firing(start, &events)
			a := alarm.New(alarm.WithMiddleware(middleware.MaxDelay(time.Millisecond, dropping(start, &events))))
			errs := ringing(t.Context(), a, slices.Values(after(start, time.Millisecond, 2*time.Second)), func(ctx context.Context, at time.Time) {
				fire(ctx, at)
				if at.Equal(start.Add(time.Millisecond)) {
					time.Sleep(time.Second)
				}
			})
			time.Sleep(500 * time.Millisecond)
			if !a.Add(time.Now()) || !a.Add(time.Time{}) {
				t.Error("got false, want true")
			}
			if err := <-errs; err != nil {
				t.Errorf("got %v, want nil", err)
			}
			// The zero time fires first, at 1001ms, and the one added at 500ms, late by more than 1ms, is dropped.
			want := []event{
				{"fire", time.Millisecond, time.Millisecond},
				{"fire", 1001 * time.Millisecond, 1001 * time.Millisecond},
				{"drop", 500 * time.Millisecond, 1001 * time.Millisecond},
				{"fire", 2 * time.Second, 2 * time.Second},
			}
			if !slices.Equal(events, want) {
				t.Errorf("got %v, want %v", events, want)
			}
		})
	})
	t.Run("Fn", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var calls []call
			rec := record(start, &calls)
			a := alarm.New()
			err := a.Ring(t.Context(), slices.Values(after(start, 10*time.Millisecond, 20*time.Millisecond, 30*time.Millisecond)), func(ctx context.Context, at time.Time) {
				rec(ctx, at)
				if at.Equal(start.Add(20*time.Millisecond)) && (!a.Add(start.Add(25*time.Millisecond)) || !a.Add(start.Add(15*time.Millisecond))) {
					t.Error("got false, want true")
				}
			})
			if err != nil {
				t.Errorf("got %v, want nil", err)
			}
			want := []call{
				{10 * time.Millisecond, 10 * time.Millisecond},
				{20 * time.Millisecond, 20 * time.Millisecond},
				{15 * time.Millisecond, 20 * time.Millisecond},
				{25 * time.Millisecond, 25 * time.Millisecond},
				{30 * time.Millisecond, 30 * time.Millisecond},
			}
			if !slices.Equal(calls, want) {
				t.Errorf("got %v, want %v", calls, want)
			}
		})
	})
	t.Run("Reset", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var calls []call
			a := alarm.New()
			errs := ringing(t.Context(), a, slices.Values(after(start, time.Hour)), record(start, &calls))
			synctest.Wait()
			if !a.Add(start.Add(2*time.Hour)) || !a.Reset(slices.Values(after(start, 3*time.Hour))) {
				t.Error("got false, want true")
			}
			if err := <-errs; err != nil {
				t.Errorf("got %v, want nil", err)
			}
			if want := []call{{2 * time.Hour, 2 * time.Hour}, {3 * time.Hour, 3 * time.Hour}}; !slices.Equal(calls, want) {
				t.Errorf("got %v, want %v", calls, want)
			}
		})
	})
	t.Run("End", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var calls []call
			rec := record(start, &calls)
			a := alarm.New()
			err := a.Ring(t.Context(), slices.Values(after(start, time.Millisecond)), func(ctx context.Context, at time.Time) {
				rec(ctx, at)
				if at.Equal(start.Add(time.Millisecond)) && !a.Add(start.Add(time.Hour)) {
					t.Error("got false, want true")
				}
			})
			if err != nil || time.Since(start) != time.Hour {
				t.Errorf("got %v at %v, want nil at 1h", err, time.Since(start))
			}
			if want := []call{{time.Millisecond, time.Millisecond}, {time.Hour, time.Hour}}; !slices.Equal(calls, want) {
				t.Errorf("got %v, want %v", calls, want)
			}
		})
	})
	t.Run("Far", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			year := 365 * 24 * time.Hour
			var events []event
			a := alarm.New(alarm.WithPuller(puller.NewHorizon(time.Hour, 1)), alarm.WithPoll(0))
			errs := ringing(t.Context(), a, reading(start, &events, 2*time.Hour, 3*time.Hour), firing(start, &events))
			synctest.Wait()
			if !a.Add(start.Add(year)) {
				t.Error("got false, want true")
			}
			if err := <-errs; err != nil {
				t.Errorf("got %v, want nil", err)
			}
			// 3h is read at 1h, as with no time added.
			want := []event{
				{"read", 2 * time.Hour, 0},
				{"read", 3 * time.Hour, time.Hour},
				{"fire", 2 * time.Hour, 2 * time.Hour},
				{"fire", 3 * time.Hour, 3 * time.Hour},
				{"fire", year, year},
			}
			if !slices.Equal(events, want) {
				t.Errorf("got %v, want %v", events, want)
			}
		})
	})
}

// A time added to Chan is sent in order with the sequence's.
func TestAdd_Chan(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		a := alarm.New()
		ch := a.Chan(t.Context(), slices.Values(after(start, time.Hour)))
		synctest.Wait()
		if !a.Add(start.Add(time.Minute)) {
			t.Error("got false, want true")
		}
		var calls []call
		for at := range ch {
			calls = append(calls, call{at: at.Sub(start), now: time.Since(start)})
		}
		if want := []call{{time.Minute, time.Minute}, {time.Hour, time.Hour}}; !slices.Equal(calls, want) {
			t.Errorf("got %v, want %v", calls, want)
		}
	})
}

// Add returns false with no Ring under way, once it returned and after Close, and ctx or Close drop the times
// added: a later Ring does not fire them.
func TestAdd_Dropped(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		var calls []call
		a := alarm.New()
		if a.Add(start) {
			t.Error("before Ring: got true, want false")
		}
		ctx, cancel := context.WithCancel(t.Context())
		errs := ringing(ctx, a, slices.Values(after(start, time.Hour)), record(start, &calls))
		synctest.Wait()
		if !a.Add(start.Add(time.Minute)) {
			t.Error("got false, want true")
		}
		cancel()
		if err := <-errs; !errors.Is(err, context.Canceled) {
			t.Errorf("got %v, want %v", err, context.Canceled)
		}
		if a.Add(start) {
			t.Error("after Ring: got true, want false")
		}
		// The time added at 1m is not this Ring's.
		if err := a.Ring(t.Context(), slices.Values(after(start, 2*time.Minute)), record(start, &calls)); err != nil {
			t.Errorf("got %v, want nil", err)
		}
		errs = ringing(t.Context(), a, slices.Values(after(start, time.Hour)), record(start, &calls))
		synctest.Wait()
		if !a.Add(start.Add(3 * time.Minute)) {
			t.Error("got false, want true")
		}
		if err := a.Close(); err != nil {
			t.Errorf("got %v, want nil", err)
		}
		if a.Add(start) {
			t.Error("after Close: got true, want false")
		}
		if err := <-errs; !errors.Is(err, alarm.ErrClosed) {
			t.Errorf("got %v, want %v", err, alarm.ErrClosed)
		}
		if want := []call{{2 * time.Minute, 2 * time.Minute}}; !slices.Equal(calls, want) {
			t.Errorf("got %v, want %v", calls, want)
		}
	})
}

// An Add that returns true fires, even as the sequence ends: Ring takes it before it returns.
func TestAdd_Ending(t *testing.T) {
	t.Parallel()

	marker := time.Unix(0, 0)
	for range 1000 {
		a := alarm.New()
		added := make(chan bool, 1)
		fired := false
		err := a.Ring(t.Context(), slices.Values(after(time.Now(), -time.Millisecond)), func(_ context.Context, at time.Time) {
			if at.Equal(marker) {
				fired = true

				return
			}
			// Lands while Ring fires, as its sequence ends, or once it returned.
			go func() {
				added <- a.Add(marker)
			}()
		})
		if err != nil {
			t.Fatalf("got %v, want nil", err)
		}
		if <-added && !fired {
			t.Fatal("added, not fired")
		}
	}
}

// Remove takes back a time added, not fired yet, one of those Equal at a time, the zero time too, even the one Ring
// waits for, which a Ring whose sequence ended then no longer waits for. It returns false for a time not added, fired,
// or being fired.
func TestRemove(t *testing.T) {
	t.Parallel()

	t.Run("Wait", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var calls []call
			a := alarm.New(alarm.WithPoll(0))
			errs := ringing(t.Context(), a, slices.Values(after(start, time.Hour)), record(start, &calls))
			synctest.Wait()
			if !a.Add(start.Add(30 * time.Minute)) {
				t.Error("Add: got false, want true")
			}
			time.Sleep(10 * time.Minute)
			if !a.Remove(start.Add(30 * time.Minute)) {
				t.Error("Remove: got false, want true")
			}
			if err := <-errs; err != nil {
				t.Errorf("got %v, want nil", err)
			}
			if want := []call{{time.Hour, time.Hour}}; !slices.Equal(calls, want) {
				t.Errorf("got %v, want %v", calls, want)
			}
		})
	})
	t.Run("Equal", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var calls []call
			a := alarm.New()
			errs := ringing(t.Context(), a, slices.Values(after(start, time.Hour)), record(start, &calls))
			synctest.Wait()
			for range 2 {
				if !a.Add(start.Add(time.Minute)) {
					t.Error("Add: got false, want true")
				}
			}
			// Equal, though in another location.
			if !a.Remove(start.Add(time.Minute).In(time.FixedZone("UTC+1", 3600))) {
				t.Error("Remove: got false, want true")
			}
			if err := <-errs; err != nil {
				t.Errorf("got %v, want nil", err)
			}
			if want := []call{{time.Minute, time.Minute}, {time.Hour, time.Hour}}; !slices.Equal(calls, want) {
				t.Errorf("got %v, want %v", calls, want)
			}
		})
	})
	t.Run("Zero", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var calls []call
			rec := record(start, &calls)
			a := alarm.New()
			err := a.Ring(t.Context(), slices.Values(after(start, time.Millisecond)), func(ctx context.Context, at time.Time) {
				rec(ctx, at)
				if !a.Add(time.Time{}) || !a.Remove(time.Time{}) {
					t.Error("got false, want true")
				}
			})
			if err != nil {
				t.Errorf("got %v, want nil", err)
			}
			if want := []call{{time.Millisecond, time.Millisecond}}; !slices.Equal(calls, want) {
				t.Errorf("got %v, want %v", calls, want)
			}
		})
	})
	t.Run("End", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			a := alarm.New()
			errs := ringing(t.Context(), a, slices.Values(after(start, time.Millisecond)), func(context.Context, time.Time) {})
			synctest.Wait()
			if !a.Add(start.Add(time.Hour)) {
				t.Error("Add: got false, want true")
			}
			time.Sleep(10 * time.Second)
			if !a.Remove(start.Add(time.Hour)) {
				t.Error("Remove: got false, want true")
			}
			if err := <-errs; err != nil || time.Since(start) != 10*time.Second {
				t.Errorf("got %v at %v, want nil at 10s", err, time.Since(start))
			}
		})
	})
	t.Run("Absent", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			a := alarm.New()
			errs := ringing(t.Context(), a, slices.Values(after(start, time.Hour)), func(_ context.Context, at time.Time) {
				if a.Remove(at) {
					t.Error("being fired: got true, want false")
				}
			})
			synctest.Wait()
			if !a.Add(start.Add(time.Minute)) {
				t.Error("Add: got false, want true")
			}
			if a.Remove(start.Add(2 * time.Minute)) {
				t.Error("not added: got true, want false")
			}
			time.Sleep(2 * time.Minute)
			if a.Remove(start.Add(time.Minute)) {
				t.Error("fired: got true, want false")
			}
			if err := <-errs; err != nil {
				t.Errorf("got %v, want nil", err)
			}
		})
	})
}

// Add returns false while n times added are held, and true again once one fired or was removed. n <= 0 means no limit.
func TestWithAddLimit(t *testing.T) {
	t.Parallel()

	t.Run("Limit", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var calls []call
			a := alarm.New(alarm.WithAddLimit(2))
			errs := ringing(t.Context(), a, slices.Values(after(start, time.Hour)), record(start, &calls))
			synctest.Wait()
			for i, want := range []bool{true, true, false} {
				if got := a.Add(start.Add(time.Duration(i+1) * 10 * time.Minute)); got != want {
					t.Errorf("Add %d: got %t, want %t", i, got, want)
				}
			}
			if !a.Remove(start.Add(20*time.Minute)) || !a.Add(start.Add(30*time.Minute)) {
				t.Error("once removed: got false, want true")
			}
			time.Sleep(15 * time.Minute)
			if !a.Add(start.Add(40 * time.Minute)) {
				t.Error("once fired: got false, want true")
			}
			if err := <-errs; err != nil {
				t.Errorf("got %v, want nil", err)
			}
			want := []call{{10 * time.Minute, 10 * time.Minute}, {30 * time.Minute, 30 * time.Minute}, {40 * time.Minute, 40 * time.Minute}, {time.Hour, time.Hour}}
			if !slices.Equal(calls, want) {
				t.Errorf("got %v, want %v", calls, want)
			}
		})
	})
	t.Run("Negative", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			a := alarm.New(alarm.WithAddLimit(-1))
			errs := ringing(t.Context(), a, slices.Values(after(time.Now(), time.Hour)), func(context.Context, time.Time) {})
			synctest.Wait()
			for i := range 3 {
				if !a.Add(time.Now().Add(time.Duration(i) * time.Minute)) {
					t.Errorf("Add %d: got false, want true", i)
				}
			}
			if err := <-errs; err != nil {
				t.Errorf("got %v, want nil", err)
			}
		})
	})
}

// Close stops Ring while it waits or from fn, without waiting, and every later Close, Shutdown, Ring or Chan returns
// ErrClosed.
func TestClose(t *testing.T) {
	t.Parallel()

	t.Run("Wait", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var calls []call
			a := alarm.New()
			errs := ringing(t.Context(), a, slices.Values(after(start, time.Hour)), record(start, &calls))
			time.Sleep(time.Millisecond)
			if err := a.Close(); err != nil {
				t.Errorf("got %v, want nil", err)
			}
			if err := <-errs; !errors.Is(err, alarm.ErrClosed) {
				t.Errorf("got %v, want %v", err, alarm.ErrClosed)
			}
			if len(calls) != 0 || time.Since(start) != time.Millisecond {
				t.Errorf("got %v at %v, want none at 1ms", calls, time.Since(start))
			}
		})
	})
	t.Run("Fn", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			a := alarm.New()
			calls := 0
			err := a.Ring(t.Context(), slices.Values(after(start, -2*time.Millisecond, -time.Millisecond, 0)), func(context.Context, time.Time) {
				calls++
				if err := a.Close(); err != nil {
					t.Errorf("got %v, want nil", err)
				}
			})
			if !errors.Is(err, alarm.ErrClosed) {
				t.Errorf("got %v, want %v", err, alarm.ErrClosed)
			}
			if calls != 1 {
				t.Errorf("got %d calls, want 1", calls)
			}
		})
	})
	t.Run("Again", func(t *testing.T) {
		t.Parallel()

		times := slices.Values(after(time.Now(), -time.Millisecond))
		a := alarm.New()
		if err := a.Close(); err != nil {
			t.Errorf("got %v, want nil", err)
		}
		if err := a.Close(); !errors.Is(err, alarm.ErrClosed) {
			t.Errorf("Close: got %v, want %v", err, alarm.ErrClosed)
		}
		if err := a.Shutdown(t.Context()); !errors.Is(err, alarm.ErrClosed) {
			t.Errorf("Shutdown: got %v, want %v", err, alarm.ErrClosed)
		}
		if err := a.Ring(t.Context(), times, func(context.Context, time.Time) { t.Error("called") }); !errors.Is(err, alarm.ErrClosed) {
			t.Errorf("Ring: got %v, want %v", err, alarm.ErrClosed)
		}
		if _, ok := <-a.Chan(t.Context(), times); ok {
			t.Error("Chan: got a time, want closed")
		}
	})
}

// Shutdown waits until Ring returned, once fn did, or until its ctx is done, and returns at once with no Ring.
func TestShutdown(t *testing.T) {
	t.Parallel()

	t.Run("Wait", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var calls []call
			a := alarm.New()
			errs := ringing(t.Context(), a, slices.Values(after(start, time.Millisecond, 4*time.Millisecond)), record(start, &calls, 2*time.Millisecond))
			time.Sleep(2 * time.Millisecond)
			if err := a.Shutdown(t.Context()); err != nil || time.Since(start) != 3*time.Millisecond {
				t.Errorf("got %v at %v, want nil at 3ms", err, time.Since(start))
			}
			if err := <-errs; !errors.Is(err, alarm.ErrClosed) {
				t.Errorf("got %v, want %v", err, alarm.ErrClosed)
			}
			if want := []call{{time.Millisecond, time.Millisecond}}; !slices.Equal(calls, want) {
				t.Errorf("got %v, want %v", calls, want)
			}
		})
	})
	t.Run("Timeout", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var calls []call
			a := alarm.New()
			errs := ringing(t.Context(), a, slices.Values(after(start, time.Millisecond)), record(start, &calls, time.Hour))
			time.Sleep(2 * time.Millisecond)
			ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
			defer cancel()
			if err := a.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) || time.Since(start) != 3*time.Millisecond {
				t.Errorf("got %v at %v, want %v at 3ms", err, time.Since(start), context.DeadlineExceeded)
			}
			if err := <-errs; !errors.Is(err, alarm.ErrClosed) {
				t.Errorf("got %v, want %v", err, alarm.ErrClosed)
			}
		})
	})
	t.Run("Idle", func(t *testing.T) {
		t.Parallel()

		a := alarm.New()
		if err := a.Shutdown(t.Context()); err != nil {
			t.Errorf("got %v, want nil", err)
		}
		if err := a.Shutdown(t.Context()); !errors.Is(err, alarm.ErrClosed) {
			t.Errorf("got %v, want %v", err, alarm.ErrClosed)
		}
	})
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
	a := alarm.New(alarm.WithMiddleware(mark("a"), nil), alarm.WithMiddleware(mark("b")))
	err := a.Ring(t.Context(), slices.Values(after(time.Now(), -time.Millisecond)), func(ctx context.Context, _ time.Time) {
		name, _ := ctx.Value(key{}).(string)
		calls = append(calls, "fn from "+name)
	})
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
				err := alarm.New(alarm.WithHolder(test.holder)).Ring(t.Context(), slices.Values(after(start, 2*time.Millisecond, 5*time.Millisecond)),
					record(start, &calls))
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

// asks records when Ring asks it, from start, answering as the default holder does.
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

// Ring polls the holder at least every d while it waits, a minute by default, and with d <= 0 only once due.
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
				a := alarm.New(append([]alarm.Option{alarm.WithHolder(h)}, test.options...)...)
				err := a.Ring(t.Context(), slices.Values(after(start, test.at)), func(context.Context, time.Time) {})
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

// ask is when Ring asked a Puller, from start, with how many times held ahead and the last read.
type ask struct {
	ahead     int
	last, now time.Duration
}

// asksEveryMillisecond records when Ring asks it, reading none ahead and asking to be asked again in a millisecond.
type asksEveryMillisecond struct {
	start time.Time
	asks  []ask
}

func (a *asksEveryMillisecond) Pull(ahead int, last, now time.Time) (time.Duration, bool) {
	a.asks = append(a.asks, ask{ahead: ahead, last: last.Sub(a.start), now: now.Sub(a.start)})

	return time.Millisecond, false
}

// Ring reads a time once it fired the last by default, and asks a puller while it waits, once its wait ends and once a
// time fired.
func TestWithPuller(t *testing.T) {
	t.Parallel()

	t.Run("Nil", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var events []event
			err := alarm.New(alarm.WithPuller(nil)).Ring(t.Context(), reading(start, &events, time.Millisecond, 2*time.Millisecond),
				firing(start, &events))
			if err != nil {
				t.Errorf("got %v, want nil", err)
			}
			want := []event{
				{"read", time.Millisecond, 0},
				{"fire", time.Millisecond, time.Millisecond},
				{"read", 2 * time.Millisecond, time.Millisecond},
				{"fire", 2 * time.Millisecond, 2 * time.Millisecond},
			}
			if !slices.Equal(events, want) {
				t.Errorf("got %v, want %v", events, want)
			}
		})
	})
	t.Run("Asks", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			p := &asksEveryMillisecond{start: start}
			var calls []call
			err := alarm.New(alarm.WithPuller(p)).Ring(t.Context(), slices.Values(after(start, 2500*time.Microsecond, 3*time.Millisecond)),
				record(start, &calls))
			if err != nil {
				t.Errorf("got %v, want nil", err)
			}
			wantAsks := []ask{
				{0, 2500 * time.Microsecond, 0},
				{0, 2500 * time.Microsecond, time.Millisecond},
				{0, 2500 * time.Microsecond, 2 * time.Millisecond},
				{0, 3 * time.Millisecond, 2500 * time.Microsecond},
			}
			if !slices.Equal(p.asks, wantAsks) {
				t.Errorf("got asks %v, want %v", p.asks, wantAsks)
			}
			want := []call{{2500 * time.Microsecond, 2500 * time.Microsecond}, {3 * time.Millisecond, 3 * time.Millisecond}}
			if !slices.Equal(calls, want) {
				t.Errorf("got %v, want %v", calls, want)
			}
		})
	})
}

// Of a run of passed times only the latest fires, the next read before each passed by more than d, even from times
// held, and the drops are reported. A time late by d at most fires with no read before it, as with d <= 0 every time.
func TestWithCoalesce(t *testing.T) {
	t.Parallel()

	t.Run("Passed", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var events []event
			times := reading(start, &events, -3*time.Millisecond, -2*time.Millisecond, -time.Millisecond, time.Millisecond, 2*time.Millisecond)
			err := alarm.New(alarm.WithCoalesce(500*time.Microsecond, dropping(start, &events))).Ring(t.Context(), times, firing(start, &events))
			if err != nil {
				t.Errorf("got %v, want nil", err)
			}
			want := []event{
				{"read", -3 * time.Millisecond, 0},
				{"read", -2 * time.Millisecond, 0},
				{"drop", -3 * time.Millisecond, 0},
				{"read", -time.Millisecond, 0},
				{"drop", -2 * time.Millisecond, 0},
				{"read", time.Millisecond, 0},
				{"fire", -time.Millisecond, 0},
				{"fire", time.Millisecond, time.Millisecond},
				{"read", 2 * time.Millisecond, time.Millisecond},
				{"fire", 2 * time.Millisecond, 2 * time.Millisecond},
			}
			if !slices.Equal(events, want) {
				t.Errorf("got %v, want %v", events, want)
			}
		})
	})
	t.Run("Within", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var events []event
			times := reading(start, &events, -time.Millisecond, time.Millisecond)
			err := alarm.New(alarm.WithCoalesce(time.Millisecond, dropping(start, &events))).Ring(t.Context(), times, firing(start, &events))
			if err != nil {
				t.Errorf("got %v, want nil", err)
			}
			want := []event{
				{"read", -time.Millisecond, 0},
				{"fire", -time.Millisecond, 0},
				{"read", time.Millisecond, 0},
				{"fire", time.Millisecond, time.Millisecond},
			}
			if !slices.Equal(events, want) {
				t.Errorf("got %v, want %v", events, want)
			}
		})
	})
	t.Run("Ended", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var events []event
			times := reading(start, &events, -2*time.Millisecond, -time.Millisecond)
			err := alarm.New(alarm.WithCoalesce(500*time.Microsecond, dropping(start, &events))).Ring(t.Context(), times, firing(start, &events))
			if err != nil {
				t.Errorf("got %v, want nil", err)
			}
			want := []event{
				{"read", -2 * time.Millisecond, 0},
				{"read", -time.Millisecond, 0},
				{"drop", -2 * time.Millisecond, 0},
				{"fire", -time.Millisecond, 0},
			}
			if !slices.Equal(events, want) {
				t.Errorf("got %v, want %v", events, want)
			}
		})
	})
	t.Run("Held", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var events []event
			times := reading(start, &events, time.Millisecond, 2*time.Millisecond, 3*time.Millisecond, 4*time.Millisecond, 5*time.Millisecond)
			fire := firing(start, &events)
			a := alarm.New(alarm.WithPuller(puller.NewAhead(3)), alarm.WithCoalesce(500*time.Microsecond, dropping(start, &events)))
			err := a.Ring(t.Context(), times, func(ctx context.Context, at time.Time) {
				fire(ctx, at)
				if at.Equal(start.Add(time.Millisecond)) {
					time.Sleep(3 * time.Millisecond)
				}
			})
			if err != nil {
				t.Errorf("got %v, want nil", err)
			}
			want := []event{
				{"read", time.Millisecond, 0},
				{"read", 2 * time.Millisecond, 0},
				{"read", 3 * time.Millisecond, 0},
				{"read", 4 * time.Millisecond, 0},
				{"fire", time.Millisecond, time.Millisecond},
				{"drop", 2 * time.Millisecond, 4 * time.Millisecond},
				{"drop", 3 * time.Millisecond, 4 * time.Millisecond},
				{"fire", 4 * time.Millisecond, 4 * time.Millisecond},
				{"read", 5 * time.Millisecond, 4 * time.Millisecond},
				{"fire", 5 * time.Millisecond, 5 * time.Millisecond},
			}
			if !slices.Equal(events, want) {
				t.Errorf("got %v, want %v", events, want)
			}
		})
	})
	t.Run("Slow", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var calls []call
			// Every 10ms from 10ms, each call 25ms long.
			ds := make([]time.Duration, 8)
			for i := range ds {
				ds[i] = time.Duration(i+1) * 10 * time.Millisecond
			}
			sleeps := slices.Repeat([]time.Duration{25 * time.Millisecond}, 8)
			err := alarm.New(alarm.WithCoalesce(2*time.Millisecond, nil)).Ring(t.Context(), slices.Values(after(start, ds...)),
				record(start, &calls, sleeps...))
			if err != nil {
				t.Errorf("got %v, want nil", err)
			}
			want := []call{
				{10 * time.Millisecond, 10 * time.Millisecond},
				{30 * time.Millisecond, 35 * time.Millisecond},
				{60 * time.Millisecond, 60 * time.Millisecond},
				{80 * time.Millisecond, 85 * time.Millisecond},
			}
			if !slices.Equal(calls, want) {
				t.Errorf("got %v, want %v", calls, want)
			}
		})
	})
	t.Run("Zero", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var events []event
			times := reading(start, &events, -2*time.Millisecond, -time.Millisecond)
			err := alarm.New(alarm.WithCoalesce(0, dropping(start, &events))).Ring(t.Context(), times, firing(start, &events))
			if err != nil {
				t.Errorf("got %v, want nil", err)
			}
			want := []event{
				{"read", -2 * time.Millisecond, 0},
				{"fire", -2 * time.Millisecond, 0},
				{"read", -time.Millisecond, 0},
				{"fire", -time.Millisecond, 0},
			}
			if !slices.Equal(events, want) {
				t.Errorf("got %v, want %v", events, want)
			}
		})
	})
	t.Run("Close", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var calls []call
			var a *alarm.Alarm
			a = alarm.New(alarm.WithCoalesce(500*time.Microsecond, func(context.Context, time.Time, time.Duration) {
				if err := a.Close(); err != nil {
					t.Errorf("got %v, want nil", err)
				}
			}))
			err := a.Ring(t.Context(), slices.Values(after(start, -2*time.Millisecond, -time.Millisecond, time.Millisecond)), record(start, &calls))
			if !errors.Is(err, alarm.ErrClosed) {
				t.Errorf("got %v, want %v", err, alarm.ErrClosed)
			}
			if len(calls) != 0 {
				t.Errorf("got %v, want none", calls)
			}
		})
	})
}

// A time passed by more than the max delay is dropped.
func TestMiddlewareMaxDelay(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		var calls []call
		var events []event
		times := after(start, -time.Hour, time.Millisecond, 2*time.Millisecond, 3*time.Millisecond, 4*time.Millisecond)
		err := alarm.New(alarm.WithMiddleware(middleware.MaxDelay(time.Millisecond, dropping(start, &events)))).Ring(t.Context(), slices.Values(times),
			record(start, &calls, 2500*time.Microsecond))
		if err != nil {
			t.Errorf("got %v, want nil", err)
		}
		// The call at 1ms ends at 3.5ms: 2ms is 1.5ms late, 3ms only 0.5ms.
		want := []call{{time.Millisecond, time.Millisecond}, {3 * time.Millisecond, 3500 * time.Microsecond}, {4 * time.Millisecond, 4 * time.Millisecond}}
		if !slices.Equal(calls, want) {
			t.Errorf("got %v, want %v", calls, want)
		}
		wantDrops := []event{{"drop", -time.Hour, 0}, {"drop", 2 * time.Millisecond, 3500 * time.Microsecond}}
		if !slices.Equal(events, wantDrops) {
			t.Errorf("got %v, want %v", events, wantDrops)
		}
	})
}

// A panic in fn is recovered and reported, and Ring rings on.
func TestMiddlewareRecover(t *testing.T) {
	t.Parallel()

	start := time.Now()
	var fired, recovered []time.Duration
	a := alarm.New(alarm.WithMiddleware(middleware.Recover(func(_ context.Context, at time.Time, v any) {
		if v != "boom" {
			t.Errorf("got %v, want boom", v)
		}
		recovered = append(recovered, at.Sub(start))
	})))
	err := a.Ring(t.Context(), slices.Values(after(start, -2*time.Millisecond, -time.Millisecond)), func(_ context.Context, at time.Time) {
		fired = append(fired, at.Sub(start))
		panic("boom")
	})
	if err != nil {
		t.Errorf("got %v, want nil", err)
	}
	if want := []time.Duration{-2 * time.Millisecond, -time.Millisecond}; !slices.Equal(fired, want) || !slices.Equal(recovered, want) {
		t.Errorf("got fired %v, recovered %v, want both %v", fired, recovered, want)
	}
}

// A spin never fires early, and stops once ctx is done. On the real clock: a spin never ends in a bubble.
func TestHolderSpin(t *testing.T) {
	t.Parallel()

	t.Run("Fire", func(t *testing.T) {
		t.Parallel()

		times := after(time.Now(), -time.Millisecond, time.Millisecond, 2*time.Millisecond, 5*time.Millisecond)
		calls := 0
		a := alarm.New(alarm.WithHolder(holder.NewSpin(2 * time.Millisecond)))
		err := a.Ring(t.Context(), slices.Values(times), func(_ context.Context, at time.Time) {
			calls++
			if now := time.Now(); now.Before(at) {
				t.Errorf("fired %v early", at.Sub(now))
			}
		})
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
		err := alarm.New(alarm.WithHolder(holder.NewSpin(2*time.Hour))).Ring(ctx, slices.Values(after(time.Now(), time.Hour)),
			func(context.Context, time.Time) { t.Error("called") })
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("got %v, want %v", err, context.DeadlineExceeded)
		}
	})
}

// Ring holds n times read ahead, and reads none while one is due.
func TestPullerAhead(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		var events []event
		times := reading(start, &events, -time.Millisecond, time.Millisecond, 2*time.Millisecond, 3*time.Millisecond)
		err := alarm.New(alarm.WithPuller(puller.NewAhead(1))).Ring(t.Context(), times, firing(start, &events))
		if err != nil {
			t.Errorf("got %v, want nil", err)
		}
		want := []event{
			{"read", -time.Millisecond, 0},
			{"fire", -time.Millisecond, 0},
			{"read", time.Millisecond, 0},
			{"read", 2 * time.Millisecond, 0},
			{"fire", time.Millisecond, time.Millisecond},
			{"read", 3 * time.Millisecond, time.Millisecond},
			{"fire", 2 * time.Millisecond, 2 * time.Millisecond},
			{"fire", 3 * time.Millisecond, 3 * time.Millisecond},
		}
		if !slices.Equal(events, want) {
			t.Errorf("got %v, want %v", events, want)
		}
	})
}

// Ring reads the next time once the last read is due within d, waking for it, at most n ahead.
func TestPullerHorizon(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		n    int
		want []event
	}{
		{"Any", 0, []event{
			{"read", time.Millisecond, 0},
			{"read", 2 * time.Millisecond, 0},
			{"read", 10 * time.Millisecond, 0},
			{"fire", time.Millisecond, time.Millisecond},
			{"fire", 2 * time.Millisecond, 2 * time.Millisecond},
			{"read", 11 * time.Millisecond, 5 * time.Millisecond},
			{"read", 12 * time.Millisecond, 6 * time.Millisecond},
			{"fire", 10 * time.Millisecond, 10 * time.Millisecond},
			{"fire", 11 * time.Millisecond, 11 * time.Millisecond},
			{"fire", 12 * time.Millisecond, 12 * time.Millisecond},
		}},
		{"One", 1, []event{
			{"read", time.Millisecond, 0},
			{"read", 2 * time.Millisecond, 0},
			{"fire", time.Millisecond, time.Millisecond},
			{"read", 10 * time.Millisecond, time.Millisecond},
			{"fire", 2 * time.Millisecond, 2 * time.Millisecond},
			{"read", 11 * time.Millisecond, 5 * time.Millisecond},
			{"fire", 10 * time.Millisecond, 10 * time.Millisecond},
			{"read", 12 * time.Millisecond, 10 * time.Millisecond},
			{"fire", 11 * time.Millisecond, 11 * time.Millisecond},
			{"fire", 12 * time.Millisecond, 12 * time.Millisecond},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				start := time.Now()
				var events []event
				times := reading(start, &events, time.Millisecond, 2*time.Millisecond, 10*time.Millisecond, 11*time.Millisecond, 12*time.Millisecond)
				a := alarm.New(alarm.WithPuller(puller.NewHorizon(5*time.Millisecond, test.n)))
				err := a.Ring(t.Context(), times, firing(start, &events))
				if err != nil {
					t.Errorf("got %v, want nil", err)
				}
				if !slices.Equal(events, test.want) {
					t.Errorf("got %v, want %v", events, test.want)
				}
			})
		})
	}
}

// Ring reads only at ticks, every 4ms, as the Horizon decides: none at 4ms, 10ms not due within 5ms yet, and none at
// 5ms, when the Horizon alone reads.
func TestPullerEvery(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		var events []event
		times := reading(start, &events, time.Millisecond, 2*time.Millisecond, 10*time.Millisecond, 11*time.Millisecond, 12*time.Millisecond)
		a := alarm.New(alarm.WithPuller(puller.NewEvery(4*time.Millisecond, puller.NewHorizon(5*time.Millisecond, 0))))
		err := a.Ring(t.Context(), times, firing(start, &events))
		if err != nil {
			t.Errorf("got %v, want nil", err)
		}
		want := []event{
			{"read", time.Millisecond, 0},
			{"read", 2 * time.Millisecond, 0},
			{"read", 10 * time.Millisecond, 0},
			{"fire", time.Millisecond, time.Millisecond},
			{"fire", 2 * time.Millisecond, 2 * time.Millisecond},
			{"read", 11 * time.Millisecond, 8 * time.Millisecond},
			{"read", 12 * time.Millisecond, 8 * time.Millisecond},
			{"fire", 10 * time.Millisecond, 10 * time.Millisecond},
			{"fire", 11 * time.Millisecond, 11 * time.Millisecond},
			{"fire", 12 * time.Millisecond, 12 * time.Millisecond},
		}
		if !slices.Equal(events, want) {
			t.Errorf("got %v, want %v", events, want)
		}
	})
}

// Times are sent on time, then the channel closes.
func TestChan(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		var calls []call
		for at := range alarm.New().Chan(t.Context(), slices.Values(after(start, time.Millisecond, 3*time.Millisecond))) {
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
		ch := alarm.New().Chan(ctx, slices.Values(after(start, time.Millisecond, time.Hour)))
		<-ch
		cancel()
		if at, ok := <-ch; ok {
			t.Errorf("got %v, want closed", at.Sub(start))
		}
	})
}

// Once the reader shuts the Alarm down, the channel closes with no other time, though a send was waiting.
func TestChan_Close(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		a := alarm.New()
		ch := a.Chan(t.Context(), slices.Values(after(start, time.Millisecond, 2*time.Millisecond)))
		<-ch
		// The send at 2ms waits for the reader.
		time.Sleep(2 * time.Millisecond)
		if err := a.Shutdown(t.Context()); err != nil {
			t.Errorf("got %v, want nil", err)
		}
		if at, ok := <-ch; ok {
			t.Errorf("got %v, want closed", at.Sub(start))
		}
	})
}

// Chan's buffer holds n times while the reader lags, and once Ring returns they stay readable before the channel
// closes. n < 0 means none.
func TestWithBuffer(t *testing.T) {
	t.Parallel()

	t.Run("Lag", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var sent []call
			sending := func(next alarm.Handler) alarm.Handler {
				return func(ctx context.Context, at time.Time) {
					next(ctx, at)
					sent = append(sent, call{at: at.Sub(start), now: time.Since(start)})
				}
			}
			ch := alarm.New(alarm.WithBuffer(2), alarm.WithMiddleware(sending)).Chan(t.Context(),
				slices.Values(after(start, time.Millisecond, 2*time.Millisecond, 3*time.Millisecond)))
			time.Sleep(10 * time.Millisecond)
			var calls []call
			for at := range ch {
				calls = append(calls, call{at: at.Sub(start), now: time.Since(start)})
			}
			// The send at 3ms finds the buffer full, and waits for the reader.
			wantSent := []call{{time.Millisecond, time.Millisecond}, {2 * time.Millisecond, 2 * time.Millisecond}, {3 * time.Millisecond, 10 * time.Millisecond}}
			if !slices.Equal(sent, wantSent) {
				t.Errorf("got sent %v, want %v", sent, wantSent)
			}
			want := []call{{time.Millisecond, 10 * time.Millisecond}, {2 * time.Millisecond, 10 * time.Millisecond}, {3 * time.Millisecond, 10 * time.Millisecond}}
			if !slices.Equal(calls, want) {
				t.Errorf("got %v, want %v", calls, want)
			}
		})
	})
	t.Run("Shutdown", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			a := alarm.New(alarm.WithBuffer(2))
			ch := a.Chan(t.Context(), slices.Values(after(start, time.Millisecond, 2*time.Millisecond, 3*time.Millisecond)))
			// The send at 3ms waits for room.
			time.Sleep(5 * time.Millisecond)
			if err := a.Shutdown(t.Context()); err != nil {
				t.Errorf("got %v, want nil", err)
			}
			var got []time.Duration
			for at := range ch {
				got = append(got, at.Sub(start))
			}
			if want := []time.Duration{time.Millisecond, 2 * time.Millisecond}; !slices.Equal(got, want) {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	})
	t.Run("Negative", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			start := time.Now()
			var calls []call
			for at := range alarm.New(alarm.WithBuffer(-1)).Chan(t.Context(), slices.Values(after(start, time.Millisecond))) {
				calls = append(calls, call{at: at.Sub(start), now: time.Since(start)})
			}
			if want := []call{{time.Millisecond, time.Millisecond}}; !slices.Equal(calls, want) {
				t.Errorf("got %v, want %v", calls, want)
			}
		})
	})
}
