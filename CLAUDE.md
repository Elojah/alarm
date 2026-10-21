# CLAUDE.md

`github.com/elojah/alarm`, a func or a channel fired at each time of a sequence, such as an rrule.Set's occurrences
(`../rrule`): one sequence, one alarm. Several sequences and their funcs are another package's. Go 1.26.1, no
dependency, one package. Behaviour is in godoc and the README, plans in TODO.md.

## Layout

- `doc.go`: the package doc. `alarm.go`: `Run`, `Chan`, the options, and `waiter`, which waits for each
  time. `bench_test.go`: the cost of a time, and how late times fire on the real clock.

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
- Options have no prefix: `WithMaxDelay`, `WithSpin`.

## Invariants

- Nothing is allocated per time: one timer per Run, created by the first wait that needs it and reset for each, which
  since Go 1.23 leaves no stale value to drain. `BenchmarkRun_Passed` shows 0 allocs.
- Run waits for each time itself, never for a gap from the last one, so lateness never adds up.
- Each pass of `wait` checks done with a non-blocking select, so that a run of passed times, or a spin, which never
  reach the timer's select, still stop once ctx is done.
- A wait is at most `maxSleep`, a minute, so that a wall clock step or a suspend shows within it.
- Run needs no goroutine and no `iter.Pull`, so that TinyGo builds it. Chan's goroutine is the only one.

## Decisions not to revert

- Timers are only millisecond-precise on Linux: Go's netpoll truncates a wait to whole milliseconds, so one fires up
  to ~1.1ms late (p99). `WithSpin` is the fix, and opt-in, since it takes a core.
- The spin is a plain loop on `time.Until`. `runtime.Gosched` in it was tried: it wakes idle Ps, 175% CPU at
  GOMAXPROCS 8, and a worse p99.

## Tests

- External test package only. Name a test after its option: `TestWithMaxDelay`.
- Timing tests run in a `synctest` bubble, but `WithSpin`'s: the bubble's clock only moves once every goroutine is
  blocked, so a spin never ends in it. `TestWithSpin` checks on the real clock only what scheduling cannot break: no
  time fires early, and ctx stops a spin.
- `testing/synctest` does not link with TinyGo, so `make tinygo` runs and builds the examples only.
- Each example under `examples/` has an `Example()` with `// Output:`, which must not depend on scheduling: it prints
  the times fired, not when. `tinygo-run` diffs `tinygo run`'s output with it.
