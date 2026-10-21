// Three times from a slice. Chan sends each on the channel it returns once the clock shows it, and closes it once the
// times end.
package main

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/elojah/alarm"
)

func main() {
	start := time.Now()
	times := []time.Time{
		start.Add(30 * time.Millisecond),
		start.Add(60 * time.Millisecond),
		start.Add(90 * time.Millisecond),
	}

	for at := range alarm.Chan(context.Background(), slices.Values(times)) {
		fmt.Println("fired", at.Sub(start))
	}
}
