package tui

import (
	"testing"

	"github.com/jxsl13/twclient/client"
)

// §T109/§V72: an observed game tick requests a redraw (wake), so the live view
// animates; with no tick (idle) nothing is posted.
func TestTickRequestsRedraw(t *testing.T) {
	app, sim := newTestApp(t)
	for sim.HasPendingEvent() { // drain any startup events
		sim.PollEvent()
	}
	if sim.HasPendingEvent() {
		t.Fatal("events pending after drain")
	}

	// An observed tick (as twclient's frontend delivers) must post a wake.
	app.state.Observe(nil, client.TickState{})
	if !sim.HasPendingEvent() {
		t.Error("a game tick did not request a redraw (no wake on tick)")
	}
}

func TestIdleNoRedraw(t *testing.T) {
	app, sim := newTestApp(t)
	_ = app
	for sim.HasPendingEvent() {
		sim.PollEvent()
	}
	// No tick, no input → nothing should spontaneously request a redraw.
	if sim.HasPendingEvent() {
		t.Error("idle app posted a redraw with no tick/input")
	}
}
