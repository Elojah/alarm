package alarm

import (
	"context"
	"slices"
	"time"
)

// Handler is called with each time: fn given to Run, and what a Middleware wraps.
type Handler func(ctx context.Context, at time.Time)

// Middleware wraps a Handler, for WithMiddleware. Package middleware holds ready-made ones.
type Middleware func(next Handler) Handler

// chain wraps handle in middlewares, the first one outermost, skipping nil ones.
func chain(handle Handler, middlewares ...Middleware) Handler {
	for _, middleware := range slices.Backward(middlewares) {
		if middleware != nil {
			handle = middleware(handle)
		}
	}

	return handle
}
