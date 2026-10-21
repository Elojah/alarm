# alarm

[![CI](https://github.com/Elojah/alarm/actions/workflows/ci.yml/badge.svg)](https://github.com/Elojah/alarm/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/elojah/alarm.svg)](https://pkg.go.dev/github.com/elojah/alarm)

Calls a func, or sends on a channel, at each time of a sequence, such as an [rrule](https://github.com/elojah/rrule)
set's occurrences. Nothing is allocated per time, so a rule can fire every millisecond. No dependencies; runs on TinyGo.

```sh
go get github.com/elojah/alarm
```

## Quick start

```go
set, err := rrule.ParseSet("DTSTART:20261021T000000Z\nRRULE:FREQ=SECONDLY;X-FREQ=PT0.001S") // every millisecond
if err != nil {
	return err
}

err = alarm.Run(ctx, set.Between(time.Now(), time.Time{}), func(ctx context.Context, at time.Time) {
	fmt.Println("fired", at)
}, alarm.WithHolder(holder.NewSpin(time.Millisecond)))
```

| Function | Does |
| --- | --- |
| `Run(ctx, times, fn, ...Option)` | Calls `fn` with `ctx` and each time once due, in the calling goroutine, until `times` ends or `ctx` is done. |
| `Chan(ctx, times, ...Option)` | The same, sending on the returned channel from its own goroutine. |

| Option | Does |
| --- | --- |
| `WithHolder(h)` | Lets `h` decide when each time is due, rather than once the clock shows it. |
| `WithMiddleware(m...)` | Wraps `fn`, or Chan's send, in `m`, the first one outermost. |
| `WithPoll(d)` | Polls the clock at least every `d` while waiting, a minute by default; `d <= 0` means never. |

| Ready-made | Does |
| --- | --- |
| `holder.NewSpin(d)` | Busy-waits for the last `d` before each time, instead of leaving it to the timer. |
| `middleware.MaxDelay(d)` | Drops a time passed by more than `d`, instead of firing it late. |

## Time

Each time is waited for itself, not as a gap from the last, so lateness never adds up and a passed time fires at once:
start a set at now with `Between`, or drop it with `middleware.MaxDelay`. A wait polls the clock at least once a
minute, or as `WithPoll` sets, so a clock step or a suspend shows within it.

## Precision

On Linux a timer fires up to a millisecond late. `holder.NewSpin` wakes Run earlier and busy-waits the rest, taking a
core while it does.

Intel Core Ultra 5 226V, GOMAXPROCS 8 (`make bench`):

| Every | Spin | p50 late | p99 late | CPU |
| --- | --- | --- | --- | --- |
| 1ms | | 363µs | 1085µs | 10% |
| 1ms | 1ms | <1µs | <1µs | 100% |
| 10ms | | 668µs | 1276µs | 3% |
| 10ms | 1ms | 1µs | 2µs | 9% |

Without a spin, Run costs what a `time.Ticker` does, without its drift. A passed time costs Run 62ns, and Chan 575ns
for the handoff to the reader.

## Examples

Run one with `go run ./examples/01-run`.

| Example | Shows |
| --- | --- |
| [`01-run`](examples/01-run/main.go) | Pulses from an iterator, with `Run` and `holder.NewSpin`. |
| [`02-chan`](examples/02-chan/main.go) | Times from a slice, received from `Chan`. |
| [`03-max-delay`](examples/03-max-delay/main.go) | A passed time dropped by `middleware.MaxDelay`. |

## Development

```sh
make check    # lint, and tests with -race
make bench    # cost of a time, and lateness on the real clock
make tinygo   # examples run, and built for a Pico, with TinyGo
```

## License

MIT.
