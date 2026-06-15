package tui

import (
	"testing"
	"time"
)

// §T65/§V37: the send buffer emits at most one line per interval, preserves FIFO
// order, and is bounded (oldest dropped when full).
func TestSendBuffer(t *testing.T) {
	b := newSendBuffer(time.Second, 8)
	b.enqueue("one", false)
	b.enqueue("two", true)

	var got []bufMsg
	send := func(msg string, team bool) { got = append(got, bufMsg{msg, team}) }

	t0 := time.Unix(1000, 0)
	// First drain emits one line immediately (last is zero → interval elapsed).
	if !b.drain(t0, send) {
		t.Fatal("first drain should emit")
	}
	// Second drain at the same instant is throttled.
	if b.drain(t0, send) {
		t.Error("drain within interval should not emit")
	}
	// After the interval, the next line emits, in FIFO order.
	if !b.drain(t0.Add(time.Second), send) {
		t.Error("drain after interval should emit")
	}
	if len(got) != 2 || got[0].msg != "one" || got[1].msg != "two" || !got[1].team {
		t.Errorf("FIFO/team wrong: %+v", got)
	}
	if b.pending() != 0 {
		t.Errorf("pending = %d want 0", b.pending())
	}
}

// §T65: bounded — flooding past the cap drops the oldest, keeps newest, no
// unbounded growth.
func TestSendBufferBounded(t *testing.T) {
	b := newSendBuffer(time.Second, 3)
	for i := 0; i < 10; i++ {
		b.enqueue(string(rune('a'+i)), false)
	}
	if b.pending() != 3 {
		t.Fatalf("pending = %d want 3 (capped)", b.pending())
	}
	var got []string
	now := time.Unix(0, 0)
	for i := 0; i < 3; i++ {
		b.drain(now, func(m string, _ bool) { got = append(got, m) })
		now = now.Add(time.Second)
	}
	// Kept the last three enqueued: h, i, j.
	want := []string{"h", "i", "j"}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("kept[%d] = %q want %q (got %v)", i, got[i], w, got)
		}
	}
}
