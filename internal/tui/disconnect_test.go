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

	app.disconnectAll()

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

func hasMenuItem(items []menuItem, label string) bool {
	for _, it := range items {
		if it.label == label {
			return true
		}
	}
	return false
}

// §C42/§V92: "Disconnect dummy" closes ONLY the active dummy and refollows the
// primary, which stays connected; no browser.
func TestDisconnectDummyKeepsPrimary(t *testing.T) {
	app, _ := newTestApp(t)
	primary := app.cur()
	primary.connected.Store(true)

	d := app.newSession("dummy", nil, nil)
	d.connected.Store(true)
	i := app.addSession(d)
	app.setActive(i)
	if app.isPrimary(app.cur()) {
		t.Fatal("dummy should be the active session")
	}

	app.disconnectDummy()

	if len(app.sessions) != 1 {
		t.Errorf("dummy not dropped: %d sessions", len(app.sessions))
	}
	if app.active != 0 {
		t.Errorf("active not refollowed to primary: %d", app.active)
	}
	if !primary.connected.Load() {
		t.Error("primary must stay connected on Disconnect dummy")
	}
	if d.connected.Load() {
		t.Error("dummy must be closed")
	}
	if app.mode == modeBrowser {
		t.Error("Disconnect dummy must not open the browser")
	}
}

// §C42/§V92: "Disconnect" ends the whole connection — primary AND every dummy
// closed, dummies dropped, browser opened.
func TestDisconnectAllClosesDummies(t *testing.T) {
	app, _ := newTestApp(t)
	primary := app.cur()
	primary.connected.Store(true)
	d := app.newSession("dummy", nil, nil)
	d.connected.Store(true)
	app.addSession(d)

	app.disconnectAll()

	if len(app.sessions) != 1 {
		t.Errorf("dummies not dropped: %d sessions", len(app.sessions))
	}
	if app.active != 0 {
		t.Errorf("active not reset: %d", app.active)
	}
	if primary.connected.Load() || d.connected.Load() {
		t.Error("all sessions must be closed")
	}
	if app.mode != modeBrowser {
		t.Error("Disconnect must open the browser")
	}
}

// §C42/§V92: the "Disconnect dummy" item appears only while a dummy is active;
// "Disconnect" is always present.
func TestEscMenuDisconnectScope(t *testing.T) {
	app, _ := newTestApp(t)
	if hasMenuItem(app.buildEscMenuItems(), "Disconnect dummy") {
		t.Error("primary active must NOT offer Disconnect dummy")
	}
	if !hasMenuItem(app.buildEscMenuItems(), "Disconnect") {
		t.Error("Disconnect must always be present")
	}
	d := app.newSession("dummy", nil, nil)
	i := app.addSession(d)
	app.setActive(i)
	if !hasMenuItem(app.buildEscMenuItems(), "Disconnect dummy") {
		t.Error("dummy active must offer Disconnect dummy")
	}
}
