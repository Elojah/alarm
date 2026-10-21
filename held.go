package alarm

import "time"

// held holds the times read and not fired yet, first in first out, in a ring that grows to the most held at once and
// is then reused, so that none is allocated per time.
type held struct {
	times []time.Time
	head  int
	// n is how many are held.
	n int
}

// push holds at after the others, growing the ring if full.
func (h *held) push(at time.Time) {
	if h.n == len(h.times) {
		times := make([]time.Time, max(2*h.n, 1))
		copy(times[copy(times, h.times[h.head:]):], h.times[:h.head])
		h.times, h.head = times, 0
	}
	h.times[h.index(h.n)] = at
	h.n++
}

// pop drops the first time held.
func (h *held) pop() {
	h.head = h.index(1)
	h.n--
}

// first returns the first time held, the one Run waits for.
func (h *held) first() time.Time {
	return h.times[h.head]
}

// last returns the last time read.
func (h *held) last() time.Time {
	return h.times[h.index(h.n-1)]
}

// index returns where the time i after the first is in the ring, for i < len(h.times): a branch costs less than a %.
func (h *held) index(i int) int {
	if i += h.head; i >= len(h.times) {
		i -= len(h.times)
	}

	return i
}
