// Five pulses 100ms apart, from an iterator, with Run and WithSpin.
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
