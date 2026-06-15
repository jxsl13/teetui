package tui

import (
	"testing"
	"time"
)

// §V29: own-chat echo dedupe matches a recently-sent line and ignores stale or
// non-matching ones.
func TestFindRecentSent(t *testing.T) {
	now := time.Now()
	sent := []sentChat{
		{msg: "old", at: now.Add(-10 * time.Second)},
		{msg: "hi", at: now.Add(-1 * time.Second)},
		{msg: "hi", at: now.Add(-200 * time.Millisecond)},
	}
	if i := findRecentSent(sent, "hi", now); i != 2 {
		t.Errorf("recent match = %d want 2 (most recent)", i)
	}
	if i := findRecentSent(sent, "old", now); i != -1 {
		t.Errorf("stale match = %d want -1 (outside window)", i)
	}
	if i := findRecentSent(sent, "nope", now); i != -1 {
		t.Errorf("no match = %d want -1", i)
	}
	if i := findRecentSent(nil, "x", now); i != -1 {
		t.Errorf("empty = %d want -1", i)
	}
}
