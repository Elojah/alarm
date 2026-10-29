// Times every 10ms until 40ms. The call at 20ms Adds 15ms, passed, which fires at once, 25ms and 35ms, which fire in
// order with the sequence's, and 60ms, after its end, which Ring waits for. It then Removes 35ms.
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
	every10ms := func(yield func(time.Time) bool) {
		for i := range 4 {
			if !yield(start.Add(time.Duration(i+1) * 10 * time.Millisecond)) {
				return
			}
		}
	}

	a := alarm.New()
	err := a.Ring(context.Background(), every10ms, func(_ context.Context, at time.Time) {
		fmt.Println("fired", at.Sub(start))
		if at.Sub(start) == 20*time.Millisecond {
			for _, d := range []time.Duration{15 * time.Millisecond, 25 * time.Millisecond, 35 * time.Millisecond, 60 * time.Millisecond} {
				a.Add(start.Add(d))
			}
			a.Remove(start.Add(35 * time.Millisecond))
		}
	})
	if err != nil {
		log.Fatal(err)
	}
}
