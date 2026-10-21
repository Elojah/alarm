// Three times from a slice, received from Chan.
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

	for at := range alarm.New().Chan(context.Background(), slices.Values(times)) {
		fmt.Println("fired", at.Sub(start))
	}
}
