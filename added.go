package alarm

import "time"

// added is a min-heap of the times Add gave, not fired yet, grown to the most held at once and then reused: no
// allocation per Add, which container/heap would box.
type added struct {
	times []time.Time
}

// push holds at.
func (h *added) push(at time.Time) {
	h.times = append(h.times, at)
	h.up(len(h.times) - 1)
}

// pop drops the earliest time held.
func (h *added) pop() {
	h.remove(0)
}

// remove drops the time held at i.
func (h *added) remove(i int) {
	last := len(h.times) - 1
	h.times[i] = h.times[last]
	h.times = h.times[:last]
	if i < last {
		h.down(i)
		h.up(i)
	}
}

// clear drops every time held, keeping the heap.
func (h *added) clear() {
	h.times = h.times[:0]
}

// first returns the earliest time held. Called with one held at least.
func (h *added) first() time.Time {
	return h.times[0]
}

// n returns how many times are held.
func (h *added) n() int {
	return len(h.times)
}

// index returns where a time Equal to at is held, or -1.
func (h *added) index(at time.Time) int {
	for i, t := range h.times {
		if t.Equal(at) {
			return i
		}
	}

	return -1
}

// up moves the time at i up while it is before its parent.
func (h *added) up(i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if !h.times[i].Before(h.times[parent]) {
			return
		}
		h.times[i], h.times[parent] = h.times[parent], h.times[i]
		i = parent
	}
}

// down moves the time at i down while a child is before it.
func (h *added) down(i int) {
	for {
		least := i
		for _, child := range [2]int{2*i + 1, 2*i + 2} {
			if child < len(h.times) && h.times[child].Before(h.times[least]) {
				least = child
			}
		}
		if least == i {
			return
		}
		h.times[i], h.times[least] = h.times[least], h.times[i]
		i = least
	}
}
