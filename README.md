# alarm

[![CI](https://github.com/Elojah/alarm/actions/workflows/ci.yml/badge.svg)](https://github.com/Elojah/alarm/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/elojah/alarm.svg)](https://pkg.go.dev/github.com/elojah/alarm)

Calls a func, or sends on a channel, at each time of a sequence, such as the occurrences of an
[rrule](https://github.com/elojah/rrule) set: one sequence, one alarm. Nothing is allocated per time, so a rule can
fire every millisecond. No dependencies, and it runs on TinyGo.

```sh
go get github.com/elojah/alarm
```

## Quick start

```go
set, err := rrule.ParseSet("DTSTART:20261021T000000Z\nRRULE:FREQ=SECONDLY;X-FREQ=PT0.001S") // every millisecond
if err != nil {
	return err
}

err = alarm.Run(ctx, set.Between(time.Now(), time.Time{}), func(at time.Time) {
	fmt.Println("fired", at)
}, alarm.WithSpin(time.Millisecond))
```

| Function | Does |
| --- | --- |
| `Run(ctx, times, fn, ...Option)` | Calls `fn` with each time once the clock shows it, in the calling goroutine, until `times` ends or `ctx` is done. |
| `Chan(ctx, times, ...Option)` | The same, sending on the channel it returns from a goroutine of its own, and closing it after. |

| Option | Does |
| --- | --- |
| `WithMaxDelay(d)` | Drops a time passed by more than `d` when reached, instead of firing it late. |
| `WithSpin(d)` | Busy-waits for the last `d` before each time, rather than leaving it to the timer. |

## Time

Times are read one at a time, once the previous one has fired, so a sequence can be endless. Each time is waited for
itself, not as a gap from the last one, so a late call, `fn` taking long or the timer firing late, never delays the next
ones: a time already passed fires at once, unless `WithMaxDelay` drops it. So start a set at now, with `Between`.

Run reads the wall clock again at least once a minute while it waits, so a time comes when the clock shows it, even
after the clock steps or the system is suspended.

## Precision

On Linux, Go waits for timers in whole milliseconds, so a timer fires up to a millisecond late. `WithSpin` wakes Run
earlier and busy-waits for the rest, which takes a core while it does: for a rule every millisecond, a whole one.

On an Intel Core Ultra 5 226V, GOMAXPROCS 8 (`make bench`):

| Every | Spin | p50 late | p99 late | CPU |
| --- | --- | --- | --- | --- |
| 1ms | | 363µs | 1085µs | 10% |
| 1ms | 1ms | <1µs | <1µs | 100% |
| 10ms | | 668µs | 1276µs | 3% |
| 10ms | 1ms | 1µs | 2µs | 9% |

Without a spin, Run costs what a `time.Ticker` does, the runtime waking a thread for each time, without its drift. A
time that has passed costs Run 62ns, and Chan 575ns, its handoff to the reader's goroutine.

## Examples

[`examples/`](examples) holds runnable programs: `go run ./examples/01-run`.

| Example | Shows |
| --- | --- |
| [`01-run`](examples/01-run/main.go) | A pulse every 100ms from an iterator, with `Run` and `WithSpin`. |
| [`02-chan`](examples/02-chan/main.go) | Times from a slice, received on `Chan`'s channel. |

## Development

```sh
make check    # golangci-lint, and the tests with -race, shuffled, with GOMAXPROCS 1 and 4
make bench    # the cost of a time, and how late times fire on the real clock
make tinygo   # the examples run and built for a Pico with TinyGo
```

## License

MIT.
