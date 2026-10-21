# TODO

- Coalesce: of the passed times, fire only the latest, a misfire policy next to `middleware.MaxDelay`. It needs a
  lookahead of one, which neither a Holder nor a Middleware sees: Ring holds times read ahead (`WithPuller`), but fires
  a due one before reading the next. Read the next first, at the same `now`, only for a time passed by more than some
  d, as a read keeps Ring from the time it is at.
- A callback for the times `middleware.MaxDelay` drops, `func(ctx, at, late)`: at a millisecond, drops go unseen
  otherwise. Not an error handler, as a Handler returns nothing. Counts of those fired and dropped are then the
  caller's.
- `holder.NewSleep(d)`, `linux && !tinygo`: Release leaves the wait to the timer until the last d, then
  `syscall.Nanosleep`s the rest, again on EINTR with what is left, and returns (0, false) so that Ring asks again.
  ~50µs late, the timer slack, rather than a spin's ~1µs, without a core, but it blocks a thread. A Reset, a Close or
  ctx shows within d. Tested on the real clock, as `holder.Spin`.
- `middleware.Recover(func(ctx, at, v any))`: a fn that panics ends Ring now; with it, Ring rings on.
- `holder.Shift(d, h)`: each time due d later, or earlier for a negative d, by h, such as a Spin. `holder` imports
  nothing from alarm: it declares the one method interface it wraps.
- A buffer for Chan's channel: the handoff to the reader is ~650ns a time, and a slow reader delays every later time.
  With n buffered, the reader may lag n times, which then reach it late.
