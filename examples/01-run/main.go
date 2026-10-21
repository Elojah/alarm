// A pulse every 100ms, five times, from a sequence that computes each time as Run asks for it. Run calls fn once the
// clock shows each, busy-waiting for the last millisecond rather than leaving it to the timer.
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
	pulses := func(yield func(time.Time) bool) {
		for i := range 5 {
			if !yield(start.Add(time.Duration(i+1) * 100 * time.Millisecond)) {
				return
			}
		}
	}

	err := alarm.Run(context.Background(), pulses, func(at time.Time) {
		fmt.Println("pulse at", at.Sub(start))
	}, alarm.WithSpin(time.Millisecond))
	if err != nil {
		log.Fatal(err)
	}
}
