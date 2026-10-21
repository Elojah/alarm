# CLAUDE.md

`github.com/elojah/alarm`, a func or a channel fired at each time of a sequence, such as an rrule.Set's occurrences
(`../rrule`): one sequence, one alarm. Several sequences and their funcs are another package's. Go 1.26.1, no
dependency. Behaviour is in godoc and the README, plans in TODO.md.

## Layout

- `doc.go`: the package doc. `alarm.go`: `Run`, `Chan`, and `runner`, which waits for and fires each time, and returns
  to Run's range when the puller reads the next. `held.go`: the ring of times read, not fired yet. `option.go`: the
  options and `config`. `holder.go`: `Holder` and the default, `onTime`. `puller.go`: `Puller` and the default, `lazy`.
  `middleware.go`: `Handler`, `Middleware` and `chain`. `bench_test.go`: the cost of a time, and how late times fire on
  the real clock.
- `holder/`, `puller/` and `middleware/` hold ready-made ones, as `../multicast` does. `middleware` imports `alarm` for
  its types, `holder` and `puller` nothing from it: they are matched by their method.

## Commands

```sh
go test -race ./...     # always -race
make check              # golangci-lint + tests (-shuffle=on -cpu 1,4)
make bench              # BenchmarkRun_Late takes a few seconds: it waits on the real clock
make tinygo             # tinygo-run (examples, output checked), tinygo-build (Pico)
```

Lint (`.golangci.yml`, rrule's) is `default: all` minus a disable list, tests included. Mind `nlreturn`,
`paralleltest` (`t.Parallel()` first), `err113`, `godot` and `funcorder`. CI runs `make check` with a pinned
golangci-lint in `./bin`.

## Conventions

- Comments are short: invariants, non-obvious whys, the public contract.
- Options have no prefix: `WithHolder`, `WithMiddleware`, `WithPuller`.
- A Holder or a Puller reads no clock: Run passes now. A Middleware hands ctx down, or one derived from it.

## Invariants

- Nothing is allocated per time: one timer per Run, created by the first wait that needs it and reset for each, which
  since Go 1.23 leaves no stale value to drain, the middleware chain built once, and the ring of times held, grown to
  the most held at once and then reused. `BenchmarkRun_Passed` shows 0 allocs. The ring wraps with a branch, not `%`:
  two `%` per time cost passed times ~10ns.
- Run waits for each time itself, never for a gap from the last one, so lateness never adds up.
- Each pass of `fire` checks done with a non-blocking select, so that a run of passed times, or a spin, which never
  reach the timer's select, still stop once ctx is done.
- A sleep on the timer is at most `config.poll`, a minute unless `WithPoll` sets it, so that a wall clock step or a
  suspend shows within it. It is never 0: `WithPoll`'s d <= 0 becomes `math.MaxInt64`.
- Run needs no goroutine and no `iter.Pull`, so that TinyGo builds it. Chan's goroutine is the only one. It reads ahead
  by returning from its range body while holding times, and fires those held once the sequence ends.
- Run asks the puller only when the time it waits for is not due and the holder sleeps (sleep > 0): a read keeps Run
  from that time, so none happens while it is due or a spin runs. Times held fire in the order read.

## Decisions not to revert

- Timers are only millisecond-precise on Linux: Go's netpoll truncates a wait to whole milliseconds, so one fires up
  to ~1.1ms late (p99). `holder.Spin` is the fix, and opt-in, since it takes a core.
- The spin is a plain loop of `fire` on `time.Now` and `Release`. `runtime.Gosched` in it was tried: it wakes idle
  Ps, 175% CPU at GOMAXPROCS 8, and a worse p99.

## Tests

- External test package only. Name a test after its option, or its ready-made piece with the package:
  `TestWithHolder`, `TestMiddlewareMaxDelay`, `TestPullerHorizon`. A subpackage's own tests check its piece without
  Run: `TestSpin`. A test of reads logs reads and fires together (`reading`, `firing`), so their order is checked.
- Timing tests run in a `synctest` bubble, but `holder.Spin`'s: the bubble's clock only moves once every goroutine is
  blocked, so a spin never ends in it. `TestHolderSpin` checks on the real clock only what scheduling cannot break: no
  time fires early, and ctx stops a spin.
- `testing/synctest` does not link with TinyGo, so `make tinygo` runs and builds the examples only.
- Each example under `examples/` has an `Example()` with `// Output:`, which must not depend on scheduling: it prints
  the times fired, not when. `tinygo-run` diffs `tinygo run`'s output with it.
