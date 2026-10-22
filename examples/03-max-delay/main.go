// A time passed an hour ago dropped by middleware.MaxDelay, which reports it, and two to come fired.
package main

import (
	"context"
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/elojah/alarm"
	"github.com/elojah/alarm/middleware"
)

func main() {
	start := time.Now()
	times := []time.Time{
		start.Add(-time.Hour),
		start.Add(30 * time.Millisecond),
		start.Add(60 * time.Millisecond),
	}

	dropped := func(_ context.Context, at time.Time, _ time.Duration) {
		fmt.Println("dropped", at.Sub(start))
	}
	a := alarm.New(alarm.WithMiddleware(middleware.MaxDelay(10*time.Millisecond, dropped)))
	err := a.Ring(context.Background(), slices.Values(times), func(_ context.Context, at time.Time) {
		fmt.Println("fired", at.Sub(start))
	})
	if err != nil {
		log.Fatal(err)
	}
}
