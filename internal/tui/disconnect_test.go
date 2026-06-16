package tui

import (
	"testing"

	"github.com/jxsl13/twclient/client"
)

// §T123/§B16/§V84: Esc-menu Disconnect tears down synchronously — the map is
// cleared and the browser opens immediately, without waiting on the async
// OnDisconnect callback; and it does not auto-reconnect.
func TestUserDisconnectClearsAndBrowses(t *testing.T) {
	app, _ := newTestApp(t)
	s := app.cur()
	s.connected.Store(true)
	s.state.Observe(nil, client.TickState{Tick: 5}) // pretend an in-game map

	app.disconnectUser()

	if _, have := s.state.Get(); have {
		t.Error("render state not cleared (map would stay)")
	}
	if app.mode != modeBrowser {
		t.Errorf("not switched to browser: mode=%d", app.mode)
	}
	if s.connected.Load() {
		t.Error("still marked connected")
	}

	// A late OnDisconnect callback for the deliberate close must NOT reconnect.
	app.reconnecting.Store(false)
	app.onDisconnect(s, "closed")
	if app.reconnecting.Load() {
		t.Error("deliberate close should not auto-reconnect")
	}
}
