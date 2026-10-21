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

a := alarm.New(alarm.WithHolder(holder.NewSpin(time.Millisecond)))
err = a.Ring(ctx, set.Between(time.Now(), time.Time{}), func(ctx context.Context, at time.Time) {
	fmt.Println("fired", at)
})
```

| Function | Does |
| --- | --- |
| `New(...Option)` | Returns an Alarm, configured by the options. |
| `a.Ring(ctx, times, fn)` | Calls `fn` with `ctx` and each time once due, in the calling goroutine, until `times` ends, `ctx` is done or `a` is closed. |
| `a.Chan(ctx, times)` | The same, sending on the returned channel from its own goroutine. |
| `a.Reset(times)` | Makes the Ring under way drop the times it holds and read `times` instead. |
| `a.Close()` | Stops the Ring under way without waiting for it. Later ones return `ErrClosed`. |
| `a.Shutdown(ctx)` | `Close`, then waits until the Ring under way returns. |

| Option | Does |
| --- | --- |
| `WithHolder(h)` | Lets `h` decide when each time is due, rather than once the clock shows it. |
| `WithMiddleware(m...)` | Wraps `fn`, or Chan's send, in `m`, the first one outermost. |
| `WithPoll(d)` | Polls the clock at least every `d` while waiting, a minute by default; `d <= 0` means never. |
| `WithPuller(p)` | Lets `p` decide when each time is read ahead, rather than once the last fired. |

| Ready-made | Does |
| --- | --- |
| `holder.NewSpin(d)` | Busy-waits for the last `d` before each time, instead of leaving it to the timer. |
| `middleware.MaxDelay(d)` | Drops a time passed by more than `d`, instead of firing it late. |
| `puller.NewAhead(n)` | Holds `n` times read ahead of the one waited for. |
| `puller.NewHorizon(d, n)` | Reads the next time once the last read is due within `d`, at most `n` ahead; `n <= 0` means any number. |

## Time

Each time is waited for itself, not as a gap from the last, so lateness never adds up and a passed time fires at once:
start a set at now with `Between`, or drop it with `middleware.MaxDelay`. A wait polls the clock at least once a
minute, or as `WithPoll` sets, so a clock step or a suspend shows within it.

## Stopping and changing the sequence

`Reset`, `Close` and `Shutdown` reach the Ring under way from any goroutine, `fn` included, once the `fn` call in
progress returns. `Reset` drops the times read and not fired yet, and reads the new sequence at once, such as a rule
edited while it rings. `Close` stops the Ring without waiting, so `fn` can call it, and `Shutdown` waits until it
returns, or until its own `ctx` is done. Neither cancels `fn`'s `ctx`: cancel Ring's to interrupt `fn`. A Reset
allocates nothing, so `fn` can Reset at each time to the next it computes. An Alarm rings one sequence at a time, and
rings again once a Ring returned, until closed.

## Reading ahead

A time is read once the last fired, so a slow sequence, such as one read from a store, takes its time from the wait
of the next. `WithPuller` reads times ahead, while Ring waits: `puller.NewHorizon(time.Minute, 1000)` reads each time a
minute before it, holding at most 1000. Ring reads in its own goroutine, so a read still keeps it from the time it waits
for: it reads none while that time is due or a `holder.NewSpin` spins, and wakes for a read when the puller says, not
only at a poll. Times held cost no allocation once the buffer holding them has grown.

## Precision

On Linux a timer fires up to a millisecond late. `holder.NewSpin` wakes Ring earlier and busy-waits the rest, taking a
core while it does.

Intel Core Ultra 5 226V, GOMAXPROCS 8 (`make bench`):

| Every | Spin | p50 late | p99 late | CPU |
| --- | --- | --- | --- | --- |
| 1ms | | 363µs | 1085µs | 10% |
| 1ms | 1ms | <1µs | <1µs | 100% |
| 10ms | | 668µs | 1276µs | 3% |
| 10ms | 1ms | 1µs | 2µs | 9% |

Without a spin, Ring costs what a `time.Ticker` does, without its drift. A passed time costs Ring 16ns, since a run
of them reads the clock once, 55ns with a Reset from `fn` at each, and Chan 470ns for the handoff to the reader.

## Examples

Run one with `go run ./examples/01-ring`.

| Example | Shows |
| --- | --- |
| [`01-ring`](examples/01-ring/main.go) | Pulses from an iterator, with `Ring` and `holder.NewSpin`. |
| [`02-chan`](examples/02-chan/main.go) | Times from a slice, received from `Chan`. |
| [`03-max-delay`](examples/03-max-delay/main.go) | A passed time dropped by `middleware.MaxDelay`. |
| [`04-horizon`](examples/04-horizon/main.go) | Times read ahead by `puller.NewHorizon`. |
| [`05-reset`](examples/05-reset/main.go) | An endless sequence replaced from `fn` by `Reset`. |
| [`06-shutdown`](examples/06-shutdown/main.go) | Times received from `Chan` until the reader calls `Shutdown`. |

## Development

```sh
make check    # lint, and tests with -race
make bench    # cost of a time, and lateness on the real clock
make tinygo   # examples run, and built for a Pico, with TinyGo
```

## License

MIT.
