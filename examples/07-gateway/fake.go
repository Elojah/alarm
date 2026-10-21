package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// timesOut is the read of warmingUp that times out.
const timesOut = 6

var (
	errTimedOut    = errors.New("probe: request timed out")
	errUnavailable = errors.New("cloud: 503 Service Unavailable")
)

// warmingUp is what probe reads, in turn: a machine warming up.
var warmingUp = []tenths{
	200, 201, 200, 202, 206, 211, 0, 224, 230, 235,
	239, 242, 244, 245, 246, 246, 245, 246, 247, 246,
	248, 251, 253, 252, 254, 253, 255, 256, 254, 255,
}

// probe fakes a Device, reading warmingUp.
type probe struct {
	reads int
}

func (p *probe) Read(context.Context) (tenths, error) {
	i := p.reads % len(warmingUp)
	p.reads++
	if i == timesOut {
		return 0, errTimedOut
	}

	return warmingUp[i], nil
}

// disk fakes a LocalStore, with a line for each transaction: how many readings, from when to when since the first.
type disk struct {
	lines []string
	first reading
}

func (d *disk) Save(_ context.Context, readings []reading) error {
	if len(d.lines) == 0 {
		d.first = readings[0]
	}
	from, to := readings[0].At.Sub(d.first.At), readings[len(readings)-1].At.Sub(d.first.At)
	d.lines = append(d.lines, fmt.Sprintf("local: saved %d readings, %v to %v", len(readings), from, to))

	return nil
}

// api fakes a Cloud, unavailable for its first request, with a line for each.
type api struct {
	lines []string
}

func (a *api) Send(_ context.Context, readings []reading) error {
	if len(a.lines) == 0 {
		a.lines = append(a.lines, "cloud: unavailable")

		return errUnavailable
	}
	temperatures := make([]string, 0, len(readings))
	for _, r := range readings {
		temperatures = append(temperatures, r.Temperature.String())
	}
	a.lines = append(a.lines, "cloud: sent "+strings.Join(temperatures, ", "))

	return nil
}
