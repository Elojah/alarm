package middleware_test

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/elojah/alarm/middleware"
)

// A time passed by more than d is dropped, and with d <= 0 none is. In a bubble, the clock stays put while it checks.
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
				called := false
				handle := middleware.MaxDelay(test.d)(func(context.Context, time.Time) { called = true })
				handle(t.Context(), time.Now().Add(-test.late))
				if called != test.want {
					t.Errorf("got called %v, want %v", called, test.want)
				}
			})
		})
	}
}
