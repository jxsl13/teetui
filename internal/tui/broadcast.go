package tui

import (
	"time"

	"github.com/mattn/go-runewidth"
)

// Broadcast overlay (§C37/§V82, ← DDNet CBroadcast): a server broadcast shows as
// a transient top-centered overlay for broadcastDur, dimming over the final
// broadcastFade, then vanishing — like the graphical client, not a log line.
const (
	broadcastDur  = 10 * time.Second // DDNet broadcast display time
	broadcastFade = 1 * time.Second  // dim over the last second before it hides
)

// setBroadcast shows text as the broadcast overlay, resetting the timer; an empty
// text clears it. Safe from a twclient callback goroutine (guarded by a.mu, §V4).
func (a *App) setBroadcast(text string) {
	a.mu.Lock()
	a.bcast = text
	if text == "" {
		a.bcastUntil = time.Time{}
	} else {
		a.bcastUntil = time.Now().Add(broadcastDur)
	}
	a.mu.Unlock()
	a.wake()
}

// drawBroadcast renders the active broadcast centered near the top, dim in the
// fade phase; nothing once empty or expired (§V82). Re-evaluated each redraw, so
// the per-tick wake (V72) animates the fade and hides it on time.
func (a *App) drawBroadcast(w, h int) {
	a.mu.Lock()
	text, until := a.bcast, a.bcastUntil
	a.mu.Unlock()
	if text == "" || w < 1 {
		return
	}
	now := time.Now()
	if !now.Before(until) { // expired
		return
	}
	style := StyleStatus
	if until.Sub(now) < broadcastFade { // fade: dim over the final second
		style = style.Dim(true)
	}
	x := (w - runewidth.StringWidth(text)) / 2
	if x < 0 {
		x = 0
	}
	// Row 2: below the status bar (row 0) and the Esc-menu hint, top-center.
	row := 2
	if row >= h {
		row = h - 1
	}
	drawStr(a.scr, x, row, w, style, text)
}
