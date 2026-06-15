package tui

import "time"

// DefaultMaxFPS caps render repaints by default to keep terminal CPU low; 0
// means unlimited (draw on every event, today's behavior) (§T74/§V42).
const DefaultMaxFPS = 60

// fpsInterval converts a frames-per-second cap to the minimum spacing between
// repaints. fps <= 0 returns 0, meaning "no cap" (§C20).
func fpsInterval(fps int) time.Duration {
	if fps <= 0 {
		return 0
	}
	return time.Second / time.Duration(fps)
}

// frameLimiter throttles render repaints to a maximum rate (§T73/§V42). It is a
// pure helper: the Run loop asks how long it must wait before the next repaint
// is allowed (0 = draw now), records when it actually drew, and coalesces a
// burst of events into a single trailing draw so the latest state is always
// shown without exceeding the cap. interval==0 disables throttling entirely.
type frameLimiter struct {
	last time.Time
}

// wait returns how long until a repaint is permitted at now, given the spacing
// interval; 0 means a repaint is due immediately (or throttling is off).
func (l *frameLimiter) wait(now time.Time, interval time.Duration) time.Duration {
	if interval <= 0 {
		return 0
	}
	d := interval - now.Sub(l.last)
	if d < 0 {
		return 0
	}
	return d
}

// record marks now as the time of the last repaint.
func (l *frameLimiter) record(now time.Time) { l.last = now }
