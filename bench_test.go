package alarm_test

import (
	"fmt"
	"iter"
	"slices"
	"testing"
	"time"

	"github.com/elojah/alarm"
)

// passed yields times that have all passed, one per iteration of b.Loop.
func passed(b *testing.B) iter.Seq[time.Time] {
	b.Helper()

	return func(yield func(time.Time) bool) {
		at := time.Unix(0, 0)
		for b.Loop() {
			at = at.Add(time.Millisecond)
			if !yield(at) {
				return
			}
		}
	}
}

// What Run costs a time, without waiting.
func BenchmarkRun_Passed(b *testing.B) {
	_ = alarm.Run(b.Context(), passed(b), func(time.Time) {})
}

// What Chan costs a time, without waiting: a handoff to the reader's goroutine.
func BenchmarkChan_Passed(b *testing.B) {
	for range alarm.Chan(b.Context(), passed(b)) {
	}
}

// How late Run calls fn on the real clock, for a time every period, with and without spinning.
func BenchmarkRun_Late(b *testing.B) {
	for _, period := range []time.Duration{time.Millisecond, 10 * time.Millisecond} {
		for _, spin := range []time.Duration{0, time.Millisecond, 2 * time.Millisecond} {
			b.Run(fmt.Sprintf("every=%v/spin=%v", period, spin), func(b *testing.B) {
				late(b, period, alarm.WithSpin(spin))
			})
		}
	}
}

// late runs Run for a time every period, on the real clock, and reports how late it calls fn.
func late(b *testing.B, period time.Duration, options ...alarm.Option) {
	b.Helper()

	var lates []time.Duration
	times := func(yield func(time.Time) bool) {
		// time.Now's monotonic reading makes fn's time.Since monotonic.
		at := time.Now()
		for b.Loop() {
			at = at.Add(period)
			if !yield(at) {
				return
			}
		}
	}
	_ = alarm.Run(b.Context(), times, func(at time.Time) { lates = append(lates, time.Since(at)) }, options...)

	slices.Sort(lates)
	for _, q := range []struct {
		unit string
		q    float64
	}{{"p50-late-µs", 0.5}, {"p99-late-µs", 0.99}, {"max-late-µs", 1}} {
		b.ReportMetric(float64(lates[int(q.q*float64(len(lates)-1))].Microseconds()), q.unit)
	}
}
