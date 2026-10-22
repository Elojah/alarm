// Times every 40ms, the first call 100ms long: of the times it passes, alarm.WithCoalesce fires only the latest, and
// reports the others dropped.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/elojah/alarm"
)

func main() {
	start := time.Now()
	every40ms := func(yield func(time.Time) bool) {
		for i := range 5 {
			if !yield(start.Add(time.Duration(i+1) * 40 * time.Millisecond)) {
				return
			}
		}
	}
	dropped := func(_ context.Context, at time.Time, _ time.Duration) {
		fmt.Println("dropped", at.Sub(start))
	}

	a := alarm.New(alarm.WithCoalesce(10*time.Millisecond, dropped))
	err := a.Ring(context.Background(), every40ms, func(_ context.Context, at time.Time) {
		fmt.Println("fired", at.Sub(start))
		// Ends at 140ms: 80ms and 120ms passed, 160ms not yet.
		if at.Sub(start) == 40*time.Millisecond {
			time.Sleep(100 * time.Millisecond)
		}
	})
	if err != nil {
		log.Fatal(err)
	}
}
