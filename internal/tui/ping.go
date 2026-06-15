package tui

import (
	"sync"
	"time"
)

// pingEntry is one chat line that pinged us (← chillerbot chathelper CLastPing).
type pingEntry struct {
	from string
	msg  string
	at   time.Time
}

// pingQueue is a bounded, newest-first history of the chat lines that pinged us
// (§T63/§V35, ← chillerbot m_aLastPings). Index 0 is always the most recent;
// pushing past the cap evicts the oldest. Goroutine-safe: pushed from the
// twclient callback goroutine, read from the UI goroutine (§V4).
type pingQueue struct {
	mu  sync.Mutex
	buf []pingEntry // index 0 = newest
	max int
}

// newPingQueue returns a queue holding at most max entries.
func newPingQueue(max int) *pingQueue {
	if max < 1 {
		max = 1
	}
	return &pingQueue{max: max}
}

// push records a new ping at the front, evicting the oldest beyond the cap.
func (q *pingQueue) push(from, msg string, at time.Time) {
	q.mu.Lock()
	q.buf = append([]pingEntry{{from: from, msg: msg, at: at}}, q.buf...)
	if len(q.buf) > q.max {
		q.buf = q.buf[:q.max]
	}
	q.mu.Unlock()
}

// at returns the entry at index i (0 = newest), false if out of range.
func (q *pingQueue) at(i int) (pingEntry, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if i < 0 || i >= len(q.buf) {
		return pingEntry{}, false
	}
	return q.buf[i], true
}

// newest returns the most recent ping, false if the queue is empty.
func (q *pingQueue) newest() (pingEntry, bool) { return q.at(0) }

// length returns how many pings are queued.
func (q *pingQueue) length() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.buf)
}
