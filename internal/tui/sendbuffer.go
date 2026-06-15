package tui

import (
	"sync"
	"time"
)

// Spam-safe send pacing (§T65/§V37): emit at most one chat line per interval,
// queue up to max. The interval is well under a single human's typing cadence so
// normal chat is unaffected, but a programmatic burst is paced.
const (
	sendMinInterval = 600 * time.Millisecond
	sendQueueMax    = 8
)

// bufMsg is one queued outgoing chat line.
type bufMsg struct {
	msg  string
	team bool
}

// sendBuffer is a spam-safe, rate-limited FIFO for outgoing chat (← chillerbot
// chathelper SayBuffer, §T65/§V37). teetui enqueues every outgoing line and a
// drainer emits at most one per interval, so a burst (multi-line replies, rapid
// auto-replies) can never flood the server and trip its mute. Bounded so a
// runaway producer can't grow memory; the oldest is dropped when full.
type sendBuffer struct {
	mu       sync.Mutex
	q        []bufMsg
	last     time.Time
	interval time.Duration
	max      int
}

// newSendBuffer returns a buffer emitting at most one line per interval, holding
// at most max queued lines.
func newSendBuffer(interval time.Duration, max int) *sendBuffer {
	if max < 1 {
		max = 1
	}
	return &sendBuffer{interval: interval, max: max}
}

// enqueue appends a line. When the buffer is full the OLDEST line is dropped to
// make room (a flood is bounded, newest kept). Returns false if a line was
// dropped to fit.
func (b *sendBuffer) enqueue(msg string, team bool) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.q = append(b.q, bufMsg{msg: msg, team: team})
	if len(b.q) > b.max {
		b.q = b.q[len(b.q)-b.max:]
		return false
	}
	return true
}

// drain emits at most one queued line via send when at least interval has
// elapsed since the last emission. Returns true if it sent one. send runs
// outside the lock so it may touch the network/UI freely.
func (b *sendBuffer) drain(now time.Time, send func(msg string, team bool)) bool {
	b.mu.Lock()
	if len(b.q) == 0 || now.Sub(b.last) < b.interval {
		b.mu.Unlock()
		return false
	}
	m := b.q[0]
	b.q = b.q[1:]
	b.last = now
	b.mu.Unlock()
	send(m.msg, m.team)
	return true
}

// pending returns the number of queued lines.
func (b *sendBuffer) pending() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.q)
}
