package tui

import (
	"strings"
	"testing"
	"time"
)

// §T121/§V82: a broadcast shows as a top overlay, expires after its window, and a
// new broadcast replaces+resets it; empty clears.
func TestBroadcastOverlay(t *testing.T) {
	app, sim := newTestApp(t)

	app.setBroadcast("server restarting")
	app.draw()
	if !strings.Contains(dumpSim(sim), "server restarting") {
		t.Fatalf("broadcast not rendered:\n%s", dumpSim(sim))
	}

	// Expired → hidden.
	app.mu.Lock()
	app.bcastUntil = time.Now().Add(-time.Second)
	app.mu.Unlock()
	app.draw()
	if strings.Contains(dumpSim(sim), "server restarting") {
		t.Error("expired broadcast still rendered")
	}

	// New broadcast replaces + resets the timer.
	app.setBroadcast("vote passed")
	if app.bcast != "vote passed" {
		t.Errorf("broadcast not replaced: %q", app.bcast)
	}
	if !app.bcastUntil.After(time.Now().Add(broadcastDur - time.Second)) {
		t.Error("new broadcast did not reset the timer")
	}

	// Empty clears it.
	app.setBroadcast("")
	app.draw()
	if strings.Contains(dumpSim(sim), "vote passed") {
		t.Error("cleared broadcast still rendered")
	}
}
