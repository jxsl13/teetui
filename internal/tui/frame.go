package tui

import "time"

// DefaultMaxFPS caps render repaints by default to keep terminal CPU low; 0
// means unlimited (draw on every event, today's behavior) (§T74/§V42).
const DefaultMaxFPS = 60

// DefaultViewportMinFPS is the default viewport-redraw FLOOR: ≥5 complete
// redraws/sec of the ingame visual viewport while connected, even without new
// ticks/events (§T130/§V90/§C41). 0 disables (pure event/tick-driven).
const DefaultViewportMinFPS = 5

// viewportInterval converts the viewport-min-fps floor to the heartbeat spacing,
// clamped by the max-fps ceiling: a floor cannot redraw faster than the cap
// allows (§C41/§V90). minFPS<=0 → 0 (disabled). When maxFPS>0 the effective rate
// is min(minFPS, maxFPS).
func viewportInterval(minFPS, maxFPS int) time.Duration {
	if minFPS <= 0 {
		return 0
	}
	if maxFPS > 0 && minFPS > maxFPS {
		minFPS = maxFPS
	}
	return time.Second / time.Duration(minFPS)
}

// heartbeatDue reports whether the viewport floor heartbeat must force a
// complete redraw now: only while connected + visual, the floor enabled
// (interval>0), and no draw happened within the interval (suppressed when
// tick/event draws already met the rate, so no double-draw) (§V90).
func heartbeatDue(now, last time.Time, interval time.Duration, connected, visual bool) bool {
	return interval > 0 && connected && visual && now.Sub(last) >= interval
}

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
