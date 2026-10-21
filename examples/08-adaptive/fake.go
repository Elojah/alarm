package main

import (
	"context"
	"fmt"
)

// startingUp is what sensor reads, in turn: a pump idling, starting, running, then stopping.
var startingUp = []float64{0.2, 0.3, 3.8, 4.1, 4.5, 4.4, 4.6, 4.3, 4.5, 4.4, 2.1, 1.2, 0.6, 0.3, 0.2}

// sensor fakes a Device, reading startingUp.
type sensor struct {
	reads int
}

func (s *sensor) Read(context.Context) (float64, error) {
	v := startingUp[s.reads%len(startingUp)]
	s.reads++

	return v, nil
}

// disk fakes a LocalStore, with a line for each sample: when since the first, and what.
type disk struct {
	lines []string
	first sample
}

func (d *disk) Save(_ context.Context, samples []sample) error {
	if len(d.lines) == 0 {
		d.first = samples[0]
	}
	for _, s := range samples {
		d.lines = append(d.lines, fmt.Sprintf("local: %v, %s", s.At.Sub(d.first.At), s))
	}

	return nil
}

// api fakes a Cloud, with a line for each sample: what, and when since the first.
type api struct {
	lines []string
	first sample
}

func (a *api) Send(_ context.Context, s sample) error {
	if len(a.lines) == 0 {
		a.first = s
	}
	a.lines = append(a.lines, fmt.Sprintf("cloud: %s at %v", s, s.At.Sub(a.first.At)))

	return nil
}
