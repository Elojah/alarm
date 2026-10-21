// An endless sequence every 10ms, received from Chan until the reader shuts the alarm down after the third time.
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
		for i := 1; ; i++ {
			if !yield(start.Add(time.Duration(i) * 10 * time.Millisecond)) {
				return
			}
		}
	}

	a := alarm.New()
	n := 0
	for at := range a.Chan(context.Background(), every10ms) {
		fmt.Println("fired", at.Sub(start))
		// The next send waits for this reader, so none comes before Shutdown.
		if n++; n == 3 {
			if err := a.Shutdown(context.Background()); err != nil {
				log.Fatal(err)
			}
			fmt.Println("shut down")
		}
	}
}
