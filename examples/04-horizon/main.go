// Two pairs of times read ahead by puller.NewHorizon: Run reads the next time once the last read is due within 50ms.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/elojah/alarm"
	"github.com/elojah/alarm/puller"
)

func main() {
	start := time.Now()
	times := func(yield func(time.Time) bool) {
		for _, d := range []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 200 * time.Millisecond, 210 * time.Millisecond} {
			fmt.Println("read", d)
			if !yield(start.Add(d)) {
				return
			}
		}
	}

	err := alarm.Run(context.Background(), times, func(_ context.Context, at time.Time) {
		fmt.Println("fired", at.Sub(start))
	}, alarm.WithPuller(puller.NewHorizon(50*time.Millisecond, 0)))
	if err != nil {
		log.Fatal(err)
	}
}
