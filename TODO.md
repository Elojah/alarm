# TODO

- Coalesce: of the passed times, fire only the latest, as a misfire policy next to `middleware.MaxDelay`. It needs the
  time after the one Ring is at, so a lookahead of one, which neither a Holder nor a Middleware sees. Ring holds times
  read ahead now (`WithPuller`), but fires a due one before reading the next. Whether the next is passed too costs no
  clock read: `release` asks at the `now` the holder released the last at. Read the next first only for a time passed
  by more than a d, as a read keeps Ring from the time it is at.
- A handler for the times `middleware.MaxDelay` drops, with how late each was, and counts of those fired and dropped:
  at a millisecond, drops go unseen otherwise. A callback given to MaxDelay, `func(ctx, at, late)`, rather than an
  error handler: a Handler returns nothing. Counts are then the caller's.
- `clock_nanosleep` on Linux for the end of a wait, instead of a spin: ~50µs late rather than ~1µs, without a core,
  but it blocks a thread and does not build with TinyGo. Not too complex: `holder.NewSleep(d)`, `linux && !tinygo`,
  whose Release leaves the wait to the timer until the last d, then `syscall.Nanosleep`s the rest, again on EINTR with
  what is left, and returns (0, false), so that Ring asks again at the clock. A Reset, a Close or ctx shows within d.
  Late by the timer slack, 50µs unless set. Tested on the real clock, as `holder.Spin`.
- `middleware.Recover(func(ctx, at, v any))`: a fn that panics ends Ring now; with it, Ring rings on.
- `holder.Shift(d, h)`: each time due d later, or earlier for a negative d, by h, such as a Spin. `holder` imports
  nothing from alarm: it declares the one method interface it wraps.
- A buffer for Chan's channel: the handoff to the reader is ~470ns a time, and a slow reader delays every later time.
  With n buffered, the reader may lag n times, which then reach it late.
