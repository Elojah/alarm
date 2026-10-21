# TODO

Left out of the first version, roughly in order.

- Coalesce: of the passed times, fire only the latest, as a misfire policy next to `middleware.MaxDelay`. It needs the
  time after the one Run is at, so a lookahead of one, which neither a Holder nor a Middleware sees. Run holds times
  read ahead now (`WithPuller`), but fires a due one before reading the next.
- A handler for the times `middleware.MaxDelay` drops, with how late each was, and counts of those fired and dropped:
  at a millisecond, drops go unseen otherwise.
- `clock_nanosleep` on Linux for the end of a wait, instead of a spin: ~50µs late rather than ~1µs, without a core,
  but it blocks a thread and does not build with TinyGo.
- A Puller made of others, reading once all of them would, for rules `puller.NewHorizon`'s `n` does not cover.
- fn's ctx with a deadline at the next time, as `schedule` had, so that a slow fn knows it is making the next late. A
  Middleware does not see the next time, so Run would need the lookahead of one too.
