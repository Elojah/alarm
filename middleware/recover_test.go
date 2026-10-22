package middleware_test

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/elojah/alarm/middleware"
)

// A panic is recovered and reported with ctx, the time and the value, even a panic(nil), and no panic reports none.
func TestRecover(t *testing.T) {
	t.Parallel()

	type key struct{}
	for _, test := range []struct {
		name string
		fn   func()
		want func(v any) bool
	}{
		{"None", func() {}, nil},
		{"Value", func() { panic("boom") }, func(v any) bool { return v == "boom" }},
		{"Nil", func() { panic(nil) }, func(v any) bool {
			_, ok := v.(*runtime.PanicNilError)

			return ok
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.WithValue(t.Context(), key{}, "ring")
			at := time.Unix(1, 0)
			recovered := false
			handle := middleware.Recover(func(ctx context.Context, got time.Time, v any) {
				recovered = true
				if ctx.Value(key{}) != "ring" || !got.Equal(at) || test.want == nil || !test.want(v) {
					t.Errorf("got recovered %v at %v with ctx value %v", v, got, ctx.Value(key{}))
				}
			})(func(context.Context, time.Time) { test.fn() })
			handle(ctx, at)
			if recovered != (test.want != nil) {
				t.Errorf("got recovered %v, want %v", recovered, test.want != nil)
			}
		})
	}
}

// A nil recovered recovers a panic silently.
func TestRecover_Nil(t *testing.T) {
	t.Parallel()

	middleware.Recover(nil)(func(context.Context, time.Time) { panic("boom") })(t.Context(), time.Unix(1, 0))
}
