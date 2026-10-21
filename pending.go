package alarm

import "time"

// pending is a FIFO ring of the times read, not fired yet, grown to the most held at once and then reused: no
// allocation per time.
type pending struct {
	times []time.Time
	head  int
	// n is how many are held.
	n int
}

// push holds at after the others, growing the ring if full.
func (p *pending) push(at time.Time) {
	if p.n == len(p.times) {
		times := make([]time.Time, max(2*p.n, 1))
		copy(times[copy(times, p.times[p.head:]):], p.times[:p.head])
		p.times, p.head = times, 0
	}
	p.times[p.index(p.n)] = at
	p.n++
}

// pop drops the first time held.
func (p *pending) pop() {
	p.head = p.index(1)
	p.n--
}

// clear drops every time held, keeping the ring.
func (p *pending) clear() {
	p.head, p.n = 0, 0
}

// first returns the first time held, the one Ring waits for.
func (p *pending) first() time.Time {
	return p.times[p.head]
}

// last returns the last time read.
func (p *pending) last() time.Time {
	return p.times[p.index(p.n-1)]
}

// index returns where the time i after the first is in the ring.
func (p *pending) index(i int) int {
	return (p.head + i) % len(p.times)
}
