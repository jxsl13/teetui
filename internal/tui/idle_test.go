package tui

import (
	"strings"
	"testing"
)

// §B9/§V71: at idle (no connect attempted) teetui must NOT claim it is connecting
// — the status reads "not connected" and the game window shows a browser hint, not
// "connecting…". Only an in-flight join/reconnect shows "connecting".
func TestIdleNotConnecting(t *testing.T) {
	app, _ := newTestApp(t) // fresh: no Join, no config, no client

	if app.connecting() {
		t.Error("idle app reports connecting")
	}
	if got := connLabel(app.connStatus()); got != "not connected" {
		t.Errorf("idle status = %q want \"not connected\"", got)
	}
	if got := app.scenePlaceholder(); strings.Contains(got, "connecting") {
		t.Errorf("idle scene shows connecting: %q", got)
	}
	if !strings.Contains(app.scenePlaceholder(), "browser") {
		t.Errorf("idle scene should hint the browser: %q", app.scenePlaceholder())
	}

	// A handshake in flight DOES read as connecting.
	app.joining.Store(true)
	if !app.connecting() {
		t.Error("joining app should report connecting")
	}
	if got := connLabel(app.connStatus()); got != "connecting" {
		t.Errorf("joining status = %q want connecting", got)
	}
	if app.scenePlaceholder() != "connecting…" {
		t.Errorf("joining scene = %q want connecting…", app.scenePlaceholder())
	}

	// Connected (map not loaded yet) still reads as connecting in the scene.
	app.joining.Store(false)
	app.connected.Store(true)
	if got := connLabel(app.connStatus()); got != "connected" {
		t.Errorf("connected status = %q", got)
	}
	if app.scenePlaceholder() != "connecting…" {
		t.Errorf("connected+loading scene = %q want connecting…", app.scenePlaceholder())
	}
}

// The full idle frame must not render the word "connecting" anywhere.
func TestIdleFrameNoConnecting(t *testing.T) {
	app, sim := newTestApp(t)
	app.draw()
	if strings.Contains(dumpSim(sim), "connecting") {
		t.Errorf("idle frame shows 'connecting':\n%s", dumpSim(sim))
	}
}
