package alarm_test

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"testing"
	"time"

	"github.com/elojah/alarm"
	"github.com/elojah/alarm/holder"
)

// passed yields a passed time per b.Loop iteration.
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

// Run's cost per time, without waiting.
func BenchmarkRun_Passed(b *testing.B) {
	_ = alarm.Run(b.Context(), passed(b), func(context.Context, time.Time) {})
}

// Chan's cost per time, without waiting: a handoff to the reader.
func BenchmarkChan_Passed(b *testing.B) {
	for range alarm.Chan(b.Context(), passed(b)) {
	}
}

// How late times fire on the real clock, with and without a spin.
func BenchmarkRun_Late(b *testing.B) {
	for _, period := range []time.Duration{time.Millisecond, 10 * time.Millisecond} {
		for _, spin := range []time.Duration{0, time.Millisecond, 2 * time.Millisecond} {
			b.Run(fmt.Sprintf("every=%v/spin=%v", period, spin), func(b *testing.B) {
				late(b, period, alarm.WithHolder(holder.NewSpin(spin)))
			})
		}
	}
}

// late reports how late Run fires a time every period.
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
	_ = alarm.Run(b.Context(), times, func(_ context.Context, at time.Time) {
		lates = append(lates, time.Since(at))
	}, options...)

	slices.Sort(lates)
	for _, q := range []struct {
		unit string
		q    float64
	}{{"p50-late-µs", 0.5}, {"p99-late-µs", 0.99}, {"max-late-µs", 1}} {
		b.ReportMetric(float64(lates[int(q.q*float64(len(lates)-1))].Microseconds()), q.unit)
	}
}
