// Five pulses 100ms apart, from an iterator, with Ring and holder.NewSpin.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/elojah/alarm"
	"github.com/elojah/alarm/holder"
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

	a := alarm.New(alarm.WithHolder(holder.NewSpin(time.Millisecond)))
	err := a.Ring(context.Background(), pulses, func(_ context.Context, at time.Time) {
		fmt.Println("pulse at", at.Sub(start))
	})
	if err != nil {
		log.Fatal(err)
	}
}
