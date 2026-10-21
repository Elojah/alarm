# CLAUDE.md

`github.com/elojah/alarm`, a func or a channel fired at each time of a sequence, such as an rrule.Set's occurrences
(`../rrule`): one sequence at a time, one alarm, which Reset replaces. Several sequences and their funcs are another
package's. Go 1.26.1, no dependency. Behaviour is in godoc and the README, plans in TODO.md.

## Layout

- `doc.go`: the package doc. `alarm.go`: `Alarm`, `New`, `Ring`, `Chan`, `Reset`, `Close` and `Shutdown`, and `fire`,
  which waits for and fires each time, and returns to the sequence when the puller reads the next, with `sleep`, which
  waits on the timer. `pending.go`: the ring of times read, not fired yet. `option.go`: the options and `config`.
  `holder.go`: `Holder` and the default, `onTime`. `puller.go`: `Puller` and the default, `lazy`. `middleware.go`:
  `Handler`, `Middleware` and `chain`. `bench_test.go`: the cost of a time, with a Reset per time too, and how late
  times fire on the real clock. `assets/logo.svg`: the README's logo, an alarm clock whose face is a sequence.
- `holder/`, `puller/` and `middleware/` hold ready-made ones, as `../multicast` does. `middleware` imports `alarm` for
  its types, `holder` and `puller` nothing from it: they are matched by their method.

## Commands

```sh
go test -race ./...     # always -race
make check              # golangci-lint + tests (-shuffle=on -cpu 1,4)
make bench              # BenchmarkRing_Late takes a few seconds: it waits on the real clock
make tinygo             # tinygo-run (examples, output checked), tinygo-build (Pico)
```

Lint (`.golangci.yml`, rrule's) is `default: all` minus a disable list, tests included. Mind `nlreturn`,
`paralleltest` (`t.Parallel()` first), `err113`, `godot` and `funcorder`. CI runs `make check` with a pinned
golangci-lint in `./bin`.

## Conventions

- Comments are short: invariants, non-obvious whys, the public contract.
- Options have no prefix: `WithHolder`, `WithMiddleware`, `WithPuller`.
- A Holder or a Puller reads no clock: Ring passes now. A Middleware hands ctx down, or one derived from it.
- Simple over fast by nanoseconds: an optimization stays only if it saves time at the millisecond scale, such as no
  allocation per time, which on TinyGo would bring GC pauses. Memory saved alone does not justify complexity.

## Invariants

- Nothing is allocated per time: one timer per Alarm, created by the first wait that needs it and reset for each, which
  since Go 1.23 leaves no stale value to drain, the middleware chain built once per Ring, and the ring of times held,
  grown to the most held at once and then reused. `BenchmarkRing_Passed` shows 0 allocs. A range over a sequence
  allocates its body, so each Reset costs 2.
- Ring waits for each time itself, never for a gap from the last one, so lateness never adds up.
- Each pass of `fire` checks done with a non-blocking select, and a Reset or a Close with `interrupted`, an atomic, so
  that a run of passed times, or a spin, which never reach the timer's select, still stop. Nothing locks `mu` per time.
  `fire` checks `interrupted` again before returning to the sequence, so a Reset from the last fn reads nothing more
  of the old sequence.
- `mu` guards `ringing`, `next`, `ending` and the closing of `closed`. Ring decides to return in `take`, which sets
  `ending` under `mu`, so a Reset either reaches the Ring or returns false, even as the sequence ends. `wake` holds a
  token at most while `next` waits: Reset sends it, and `drop` drains it, both under `mu`.
- Only the Ring under way touches `handle`, `done`, `pending` and `timer`.
  `end`, deferred, stops the timer and frees `ringing` under `mu`, so the next Ring takes them over safely, even after
  fn panicked.
- Close never waits, so fn can call it. Shutdown waits on `ringing`. Neither cancels fn's ctx. Chan's send also selects
  on `closed`, so a reader that stops and Closes frees it.
- A sleep on the timer is at most `config.poll`, a minute unless `WithPoll` sets it, so that a wall clock step or a
  suspend shows within it. It is never 0: `WithPoll`'s d <= 0 becomes `math.MaxInt64`.
- Ring needs no goroutine and no `iter.Pull`, so that TinyGo builds it. Chan's goroutine is the only one. It reads ahead
  by ranging on while holding times, and fires those held once the sequence ends.
- Ring asks the puller only when the time it waits for is not due and the holder sleeps (sleep > 0): a read keeps Ring
  from that time, so none happens while it is due or a spin runs. Times held fire in the order read.

## Decisions not to revert

- Timers are only millisecond-precise on Linux: Go's netpoll truncates a wait to whole milliseconds, so one fires up
  to ~1.1ms late (p99). `holder.Spin` is the fix, and opt-in, since it takes a core.
- Chan's send tries a non-blocking send before its select, which locks each of its three channels: ~470ns a time
  rather than ~530ns (`BenchmarkChan_Passed`, interleaved runs).
- The spin is a plain loop of `fire` on `time.Now` and `Release`. `runtime.Gosched` in it was tried: it wakes idle
  Ps, 175% CPU at GOMAXPROCS 8, and a worse p99.

## Tests

- External test package only. Name a test after its method or option, or its ready-made piece with the package:
  `TestReset`, `TestShutdown`, `TestWithHolder`, `TestMiddlewareMaxDelay`, `TestPullerHorizon`. `ringing` runs a Ring
  in a goroutine, for a test that Resets, Closes or Shuts down from outside fn. A subpackage's own tests check its
  piece without Ring: `TestSpin`. A test of reads logs reads and fires together (`reading`, `firing`), so their order
  is checked.
- Timing tests run in a `synctest` bubble, but `holder.Spin`'s: the bubble's clock only moves once every goroutine is
  blocked, so a spin never ends in it. `TestHolderSpin` checks on the real clock only what scheduling cannot break: no
  time fires early, and ctx stops a spin.
- `testing/synctest` does not link with TinyGo, so `make tinygo` runs and builds the examples only.
- Each example under `examples/` has an `Example()` with `// Output:`, which must not depend on scheduling: it prints
  the times fired, not when. `tinygo-run` diffs `tinygo run`'s output with it.
