// An endless sequence every 10ms, replaced from fn at 20ms by two times at 100ms and 110ms, with Reset.
package main

import (
	"context"
	"fmt"
	"log"
	"slices"
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
	err := a.Ring(context.Background(), every10ms, func(_ context.Context, at time.Time) {
		fmt.Println("fired", at.Sub(start))
		if at.Sub(start) == 20*time.Millisecond {
			a.Reset(slices.Values([]time.Time{start.Add(100 * time.Millisecond), start.Add(110 * time.Millisecond)}))
		}
	})
	if err != nil {
		log.Fatal(err)
	}
}
