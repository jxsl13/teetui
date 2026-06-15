package tui

import (
	"testing"
	"time"
)

// §T63/§V35: the ping queue is bounded, newest-first, and evicts the oldest
// without corrupting order.
func TestPingQueue(t *testing.T) {
	q := newPingQueue(3)
	if _, ok := q.newest(); ok {
		t.Error("empty queue must report no newest")
	}
	now := time.Now()
	q.push("a", "m-a", now)
	q.push("b", "m-b", now)
	q.push("c", "m-c", now)

	// Newest-first ordering.
	if p, _ := q.at(0); p.from != "c" {
		t.Errorf("index 0 = %q want c", p.from)
	}
	if p, _ := q.at(2); p.from != "a" {
		t.Errorf("index 2 = %q want a", p.from)
	}

	// Push past cap evicts the oldest (a), keeps order [d,c,b].
	q.push("d", "m-d", now)
	if q.length() != 3 {
		t.Fatalf("length = %d want 3 (capped)", q.length())
	}
	wantFrom := []string{"d", "c", "b"}
	for i, w := range wantFrom {
		p, ok := q.at(i)
		if !ok || p.from != w {
			t.Errorf("at(%d) = %q,%v want %q", i, p.from, ok, w)
		}
	}
	if _, ok := q.at(3); ok {
		t.Error("index past length must be absent")
	}
}
