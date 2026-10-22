package middleware_test

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/elojah/alarm/middleware"
)

// A time passed by more than d is dropped, and reported with how late, and with d <= 0 none is. In a bubble, the clock
// stays put while it checks.
func TestMaxDelay(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		d, late time.Duration
		want    bool
	}{
		{"Early", time.Millisecond, -time.Hour, true},
		{"Within", time.Millisecond, time.Millisecond, true},
		{"Past", time.Millisecond, time.Millisecond + time.Nanosecond, false},
		{"Zero", 0, time.Hour, true},
		{"Negative", -time.Millisecond, time.Hour, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				at := time.Now().Add(-test.late)
				called, dropped := false, false
				handle := middleware.MaxDelay(test.d, func(_ context.Context, got time.Time, late time.Duration) {
					dropped = true
					if !got.Equal(at) || late != test.late {
						t.Errorf("got dropped %v late %v, want %v late %v", got, late, at, test.late)
					}
				})(func(context.Context, time.Time) { called = true })
				handle(t.Context(), at)
				if called != test.want || dropped == test.want {
					t.Errorf("got called %v, dropped %v, want called %v", called, dropped, test.want)
				}
			})
		})
	}
}

// A nil dropped drops a time silently.
func TestMaxDelay_Nil(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		called := false
		middleware.MaxDelay(time.Millisecond, nil)(func(context.Context, time.Time) { called = true })(t.Context(), time.Now().Add(-time.Hour))
		if called {
			t.Error("got called, want dropped")
		}
	})
}
