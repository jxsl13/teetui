package tui

import (
	"testing"
	"time"
)

// §T73/§V42: fpsInterval maps a cap to spacing; 0/negative = unlimited.
func TestFPSInterval(t *testing.T) {
	if d := fpsInterval(0); d != 0 {
		t.Errorf("fps 0 = %v want 0 (unlimited)", d)
	}
	if d := fpsInterval(-5); d != 0 {
		t.Errorf("fps -5 = %v want 0", d)
	}
	if d := fpsInterval(100); d != 10*time.Millisecond {
		t.Errorf("fps 100 = %v want 10ms", d)
	}
	if d := fpsInterval(60); d != time.Second/60 {
		t.Errorf("fps 60 = %v", d)
	}
}

// §T130/§V90/§C41: the viewport floor interval = 1/min-fps, clamped by the
// max-fps ceiling; 0 disables.
func TestViewportInterval(t *testing.T) {
	cases := []struct {
		min, max int
		want     time.Duration
	}{
		{1, 60, time.Second},        // default floor, well under cap
		{0, 60, 0},                  // disabled
		{-3, 60, 0},                 // disabled (negative)
		{5, 0, time.Second / 5},     // cap unlimited → floor as-is
		{120, 60, time.Second / 60}, // floor above cap → clamped to cap
		{60, 60, time.Second / 60},  // equal
		{2, 10, time.Second / 2},    // floor under cap → floor as-is
	}
	for _, c := range cases {
		if got := viewportInterval(c.min, c.max); got != c.want {
			t.Errorf("viewportInterval(%d,%d) = %v want %v", c.min, c.max, got, c.want)
		}
	}
}

// §V90: heartbeat fires only while connected+visual, enabled, and silent ≥
// interval; suppressed when a recent draw met the rate.
func TestHeartbeatDue(t *testing.T) {
	now := time.Unix(100, 0)
	iv := time.Second
	old := now.Add(-2 * time.Second) // last draw 2s ago → silent ≥ interval
	recent := now.Add(-10 * time.Millisecond)

	if !heartbeatDue(now, old, iv, true, true) {
		t.Error("connected+visual+silent → due")
	}
	if heartbeatDue(now, recent, iv, true, true) {
		t.Error("recent draw met the rate → suppressed")
	}
	if heartbeatDue(now, old, iv, false, true) {
		t.Error("disconnected → not due")
	}
	if heartbeatDue(now, old, iv, true, false) {
		t.Error("visual off → not due")
	}
	if heartbeatDue(now, old, 0, true, true) {
		t.Error("disabled (interval 0) → not due")
	}
}

// §T73/§V42: the limiter permits a draw immediately after the interval elapses,
// throttles within it, and never throttles when unlimited.
func TestFrameLimiter(t *testing.T) {
	var l frameLimiter
	t0 := time.Unix(100, 0)
	interval := 10 * time.Millisecond

	// First draw is due immediately (last is zero).
	if w := l.wait(t0, interval); w != 0 {
		t.Errorf("initial wait = %v want 0", w)
	}
	l.record(t0)

	// A request 4ms later must wait the remaining 6ms.
	if w := l.wait(t0.Add(4*time.Millisecond), interval); w != 6*time.Millisecond {
		t.Errorf("mid-interval wait = %v want 6ms", w)
	}
	// At/after the interval it is due again.
	if w := l.wait(t0.Add(interval), interval); w != 0 {
		t.Errorf("post-interval wait = %v want 0", w)
	}

	// Unlimited (interval 0) is never throttled, even right after a draw.
	l.record(t0)
	if w := l.wait(t0, 0); w != 0 {
		t.Errorf("unlimited wait = %v want 0", w)
	}
}
