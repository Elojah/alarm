# TODO

Left out of the first version, roughly in order.

- Coalesce: of the passed times, fire only the latest, as a misfire policy next to `WithMaxDelay`. It needs the time
  after the one Run is at, so a lookahead of one.
- A handler for the times `WithMaxDelay` drops, with how late each was, and counts of those fired and dropped: at a
  millisecond, drops go unseen otherwise.
- `clock_nanosleep` on Linux for the end of a wait, instead of a spin: ~50µs late rather than ~1µs, without a core,
  but it blocks a thread and does not build with TinyGo.
- fn's ctx with a deadline at the next time, as `schedule` had, so that a slow fn knows it is making the next late.
- The package for several sequences and their funcs, in its own repo: one Run each, the runtime keeping their timers
  in its own heap. A single goroutine with a heap of next times would need pull iteration.
