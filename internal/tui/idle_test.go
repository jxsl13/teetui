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
	app.cur().joining.Store(true)
	if !app.connecting() {
		t.Error("joining app should report connecting")
	}
	if got := connLabel(app.connStatus()); got != "connecting" {
		t.Errorf("joining status = %q want connecting", got)
	}
	if app.scenePlaceholder() != "connecting…" {
		t.Errorf("joining scene = %q want connecting…", app.scenePlaceholder())
	}

	// Connected but no map yet → actionable retry notice, not an endless spinner
	// (§B18: twclient may connect "without map").
	app.cur().joining.Store(false)
	app.cur().connected.Store(true)
	if got := connLabel(app.connStatus()); got != "connected" {
		t.Errorf("connected status = %q", got)
	}
	if got := app.scenePlaceholder(); !strings.Contains(got, "press R") {
		t.Errorf("connected-no-map scene = %q want a re-download notice", got)
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

// §T125/§V86/§B18: connected with no map shows a re-download notice; joining
// shows the spinner; idle shows the browser hint.
func TestConnectedNoMapNotice(t *testing.T) {
	app, _ := newTestApp(t)
	// joining (download in flight) → spinner.
	app.cur().joining.Store(true)
	if app.scenePlaceholder() != "connecting…" {
		t.Errorf("joining = %q want connecting…", app.scenePlaceholder())
	}
	// connected, no map → actionable retry, not the spinner.
	app.cur().joining.Store(false)
	app.cur().connected.Store(true)
	if got := app.scenePlaceholder(); got == "connecting…" || !strings.Contains(got, "re-download") {
		t.Errorf("connected-no-map = %q want a re-download notice", got)
	}
}
