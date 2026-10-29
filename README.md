<p align="center">
  <img src="assets/logo.svg" alt="alarm" width="180">
</p>

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
| `a.Add(at)` | Makes the Ring under way fire `at` too, in order with its sequence's times; the zero time fires at once. |
| `a.Remove(at)` | Takes back a time `Add` gave, not fired yet. |
| `a.Close()` | Stops the Ring under way without waiting for it. Later ones return `ErrClosed`. |
| `a.Shutdown(ctx)` | `Close`, then waits until the Ring under way returns. |

| Option | Does |
| --- | --- |
| `WithAddLimit(n)` | Makes `Add` return false while `n` times added are held. |
| `WithBuffer(n)` | Makes Chan's channel hold `n` times, so that the reader may lag `n` times. |
| `WithCoalesce(d, dropped)` | Of a run of times passed by more than `d`, fires only the latest, and reports the others to `dropped`. |
| `WithHolder(h)` | Lets `h` decide when each time is due, rather than once the clock shows it. |
| `WithMiddleware(m...)` | Wraps `fn`, or Chan's send, in `m`, the first one outermost. |
| `WithPoll(d)` | Polls the clock at least every `d` while waiting, a minute by default; `d <= 0` means never. |
| `WithPuller(p)` | Lets `p` decide when each time is read ahead, rather than once the last fired. |

| Ready-made | Does |
| --- | --- |
| `holder.NewSpin(d)` | Busy-waits the last `d` before each time, rather than leaving it to the timer. |
| `middleware.MaxDelay(d, dropped)` | Drops a time passed by more than `d`, rather than firing it late, and reports it to `dropped`. |
| `middleware.Recover(recovered)` | Recovers a panic in `fn`, which would end Ring, and reports it to `recovered`. |
| `puller.NewAhead(n)` | Holds `n` times read ahead of the one waited for. |
| `puller.NewHorizon(d, n)` | Reads each time once it is due within `d`, at most `n` ahead; `n <= 0` means any number. |
| `puller.NewEvery(d, p)` | Lets `p` read only at ticks, every `d`, rather than whenever it would. |

## Time

Each time is waited for itself, not as a gap from the last: lateness from waits never adds up, and a passed time fires
at once. Start a set at now with `Between`, or drop passed times with `middleware.MaxDelay`. A wait polls the clock at
least once a minute (`WithPoll`), so a clock step or a suspend shows within it.

`fn` runs in Ring's goroutine, so the times it passes fire late, back to back, and an `fn` slower than the gap between
times makes each later one later. `middleware.MaxDelay` drops the times passed by more than its `d`, and `WithCoalesce`
fires only the latest of them. Both report what they drop.

## Stopping and changing the sequence

An Alarm rings one sequence at a time. `Reset`, `Add`, `Remove`, `Close` and `Shutdown` reach the Ring under way from
any goroutine, `fn` included, once the `fn` call in progress returns:

- `Reset` drops the times held and reads the new sequence, such as a rule edited while it rings. `fn` can Reset to the
  next time it computes.
- `Add` fires one more time, such as a reading on demand, in order with the sequence's, and Ring returns only once it
  fired. A passed time fires at once, and the zero time first of all, as the time it fires, so that
  `middleware.MaxDelay` never drops it. Times added are kept apart from the sequence's, so a far one holds back neither,
  and a Reset keeps them. `Remove` takes one back, and `WithAddLimit` bounds how many are held.
- `Close` stops the Ring without waiting, so `fn` can call it. `Shutdown` also waits until it returns, or its `ctx` is
  done.

Neither cancels `fn`'s `ctx`: cancel Ring's to interrupt `fn`.

## Reading ahead

A time is read once the last fired, so a slow sequence, such as one read from a store, delays the next. `WithPuller`
reads ahead while Ring waits: `puller.NewHorizon(time.Minute, 1000)` reads each time a minute before it, at most 1000
held. `puller.NewEvery(15*time.Minute, puller.NewHorizon(24*time.Hour, 0))` reads a day ahead in a burst every 15
minutes rather than one time at a time. A read still blocks Ring, so the puller asks for none while a time is due or a
spin runs. `WithCoalesce` reads one before a time passed by more than its `d`.

## Precision

On Linux a timer fires up to a millisecond late. `holder.NewSpin` busy-waits the end of each wait instead, taking a
core while it does.

Intel Core Ultra 5 226V, GOMAXPROCS 8 (`make bench`):

| Every | Spin | p50 late | p99 late | CPU |
| --- | --- | --- | --- | --- |
| 1ms | | 363µs | 1085µs | 10% |
| 1ms | 1ms | <1µs | <1µs | 100% |
| 10ms | | 668µs | 1276µs | 3% |
| 10ms | 1ms | 1µs | 2µs | 9% |

Without a spin, Ring costs what a `time.Ticker` does, without its drift. A passed time costs Ring 75ns, 350ns with a
Reset from `fn` at each, and Chan 650ns for the handoff to the reader, 240ns with `WithBuffer(64)`.

## Examples

Run one with `go run ./examples/01-ring`.

| Example | Shows |
| --- | --- |
| [`01-ring`](examples/01-ring/main.go) | Pulses from an iterator, with `Ring` and `holder.NewSpin`. |
| [`02-chan`](examples/02-chan/main.go) | Times from a slice, received from `Chan`. |
| [`03-max-delay`](examples/03-max-delay/main.go) | A passed time dropped, and reported, by `middleware.MaxDelay`. |
| [`04-horizon`](examples/04-horizon/main.go) | Times read ahead by `puller.NewHorizon`. |
| [`05-reset`](examples/05-reset/main.go) | An endless sequence replaced from `fn` by `Reset`. |
| [`06-shutdown`](examples/06-shutdown/main.go) | Times received from `Chan` until the reader calls `Shutdown`. |
| [`07-gateway`](examples/07-gateway/main.go) | A probe polled every 10ms by an rrule set, each reading fanned out by [multicast](https://github.com/elojah/multicast) to a local store and a cloud API. |
| [`08-adaptive`](examples/08-adaptive/main.go) | A pump sampled every 100ms while idle and every 10ms while running, switching rules with `Reset`. |
| [`09-coalesce`](examples/09-coalesce/main.go) | A call longer than the gap, after which `WithCoalesce` fires only the latest of the times it passed. |
| [`10-add`](examples/10-add/main.go) | Times added from `fn` by `Add`, passed, between the sequence's and after its end, and one taken back by `Remove`. |

`07-gateway` and `08-adaptive` are modules of their own, to keep rrule and multicast out of the library's go.mod:
`go run -C examples/07-gateway .`. Their device, store and cloud are interfaces, faked in `fake.go`.

## Development

```sh
make check    # lint, and tests with -race
make bench    # cost of a time, and lateness on the real clock
make tinygo   # examples run, and built for a Pico, with TinyGo
```

## License

MIT.
