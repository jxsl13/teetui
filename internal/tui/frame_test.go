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
