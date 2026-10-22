# TODO

- `holder.NewSleep(d)`, `linux && !tinygo`: Release leaves the wait to the timer until the last d, then
  `syscall.Nanosleep`s the rest, again on EINTR with what is left, and returns (0, false) so that Ring asks again.
  ~50µs late, the timer slack, rather than a spin's ~1µs, without a core, but it blocks a thread. A Reset, a Close or
  ctx shows within d. Tested on the real clock, as `holder.Spin`.
- `holder.Shift(d, h)`: each time due d later, or earlier for a negative d, by h, such as a Spin. `holder` imports
  nothing from alarm: it declares the one method interface it wraps.
