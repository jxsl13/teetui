package tui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/twclient/client"
)

func menuLabels(a *App) []string {
	out := make([]string, len(a.escMenu.items))
	for i, it := range a.escMenu.items {
		out[i] = it.label
	}
	return out
}

func hasLabel(a *App, label string) bool {
	for _, l := range menuLabels(a) {
		if l == label {
			return true
		}
	}
	return false
}

// §T111/§V74: Esc opens the action bar only when connected; Esc closes it.
func TestEscMenuToggle(t *testing.T) {
	app, _ := newTestApp(t)
	app.cur().connected.Store(true)

	app.handle(sk(tcell.KeyEscape))
	if !app.escMenu.open {
		t.Fatal("Esc did not open the menu while connected")
	}
	app.handle(sk(tcell.KeyEscape))
	if app.escMenu.open {
		t.Fatal("Esc did not close the menu")
	}
}

// §T111/§V75: solo vs team game produces the right Join buttons; in-game adds
// Kill/Pause; Disconnect is always present.
func TestEscMenuContextItems(t *testing.T) {
	app, _ := newTestApp(t)
	app.cur().connected.Store(true)

	// Solo, spectating (no local character).
	app.cur().state.Observe(nil, client.TickState{})
	app.openEscMenu()
	if !hasLabel(app, "Join game") || !hasLabel(app, "Spectate") {
		t.Errorf("solo menu = %v", menuLabels(app))
	}
	if hasLabel(app, "Kill") {
		t.Error("spectator should not get Kill")
	}
	if !hasLabel(app, "Disconnect") {
		t.Error("Disconnect always present")
	}

	// Team mode, in-game (has a character).
	st := client.TickState{LocalID: 0, Players: map[int]client.CharacterState{0: {}}}
	st.GameInfo.GameFlags = gameflagTeams
	app.cur().state.Observe(nil, st)
	app.openEscMenu()
	if !hasLabel(app, "Join red") || !hasLabel(app, "Join blue") {
		t.Errorf("team menu = %v", menuLabels(app))
	}
	// Kill is universal; Pause is gated on DDRace mode (§B22/§V94) — not DDNet here.
	if !hasLabel(app, "Kill") {
		t.Errorf("in-game menu missing Kill: %v", menuLabels(app))
	}
	if hasLabel(app, "Pause") {
		t.Errorf("Pause must be hidden on non-DDRace mode: %v", menuLabels(app))
	}
}

// §B22/§C44/§V94: Pause appears in-game only on a DDRace-derived (DDNet) server.
func TestEscMenuPauseDDRaceOnly(t *testing.T) {
	app, _ := newTestApp(t)
	app.cur().connected.Store(true)
	st := client.TickState{LocalID: 0, Players: map[int]client.CharacterState{0: {}}}
	app.cur().state.Observe(nil, st)

	// Non-DDRace (default) in-game → Kill, no Pause.
	app.openEscMenu()
	if !hasLabel(app, "Kill") || hasLabel(app, "Pause") {
		t.Errorf("non-DDRace menu = %v (want Kill, no Pause)", menuLabels(app))
	}

	// DDRace-derived → Pause appears.
	app.cur().ddrace.Store(true)
	app.openEscMenu()
	if !hasLabel(app, "Pause") {
		t.Errorf("DDRace menu missing Pause: %v", menuLabels(app))
	}
}

// §T111: focus nav wraps; Enter runs the focused item and closes.
func TestEscMenuNavAndActivate(t *testing.T) {
	app, _ := newTestApp(t)
	app.cur().connected.Store(true)
	ran := ""
	app.escMenu = escMenu{open: true, items: []menuItem{
		{"a", func() { ran = "a" }},
		{"b", func() { ran = "b" }},
	}}
	app.handle(sk(tcell.KeyRight)) // focus → b
	if app.escMenu.focus != 1 {
		t.Fatalf("focus = %d want 1", app.escMenu.focus)
	}
	app.handle(sk(tcell.KeyRight)) // wrap → a
	if app.escMenu.focus != 0 {
		t.Fatalf("focus wrap = %d want 0", app.escMenu.focus)
	}
	app.handle(sk(tcell.KeyEnter))
	if ran != "a" {
		t.Errorf("Enter ran %q want a", ran)
	}
	if app.escMenu.open {
		t.Error("menu should close after activate")
	}
}

// §T111: the rendered bar shows the buttons and the key hint.
func TestEscMenuRenders(t *testing.T) {
	app, sim := newTestApp(t)
	app.cur().connected.Store(true)
	app.cur().state.Observe(nil, client.TickState{})
	app.openEscMenu()
	app.draw()
	out := dumpSim(sim)
	if !strings.Contains(out, "Disconnect") || !strings.Contains(out, "Esc close") {
		t.Errorf("esc menu not rendered:\n%s", out)
	}
}

// §T114/§V77: with a dummy connected, the menu lists a follow item per session;
// selecting it switches the active session.
func TestEscMenuFollow(t *testing.T) {
	app, _ := newTestApp(t)
	app.cur().connected.Store(true)
	app.state0Observe(client.TickState{})

	// One session → no follow list.
	app.openEscMenu()
	if hasLabel(app, "● main") {
		t.Error("single session should not show a follow list")
	}

	// Add a dummy → follow items appear; active marked with ●.
	d := app.newSession("dummy", nil, nil)
	app.addSession(d)
	app.openEscMenu()
	if !hasLabel(app, "● main") || !hasLabel(app, "Follow dummy") {
		t.Fatalf("follow list missing: %v", menuLabels(app))
	}

	// Activating "Follow dummy" switches the active session.
	for _, it := range app.escMenu.items {
		if it.label == "Follow dummy" {
			it.run()
		}
	}
	if app.cur() != d {
		t.Error("Follow dummy did not switch active session")
	}
}

func (a *App) state0Observe(st client.TickState) { a.sessions[0].state.Observe(nil, st) }

// §T115/§V76: connectDummy adds a session and follows it; no Connect-dummy item
// when there is no client (so no capabilities).
func TestConnectDummy(t *testing.T) {
	app, _ := newTestApp(t)
	app.cur().connected.Store(true)
	app.state0Observe(client.TickState{})

	// No client → no "Connect dummy" button (caps unknown).
	app.openEscMenu()
	if hasLabel(app, "Connect dummy") {
		t.Error("Connect dummy shown without a client/capabilities")
	}

	// connectDummy wiring: with a server set it appends + follows a new session.
	app.cur().server = "srv:8303"
	before := len(app.sessions)
	app.connectDummy()
	if len(app.sessions) != before+1 {
		t.Fatalf("connectDummy did not add a session: %d", len(app.sessions))
	}
	if app.cur().name != "dummy" {
		t.Errorf("connectDummy did not follow the new dummy (active=%q)", app.cur().name)
	}
}

// §T119/§V66: the Esc menu can swap the move/aim key sets live.
func TestEscMenuSwapMoveKeys(t *testing.T) {
	app, _ := newTestApp(t)
	before := app.cfgSnap().MoveKeys
	app.toggleMoveKeys()
	after := app.cfgSnap().MoveKeys
	if before == after {
		t.Errorf("toggleMoveKeys did not change: %q", after)
	}
	app.toggleMoveKeys()
	if app.cfgSnap().MoveKeys != before {
		t.Error("toggle should round-trip")
	}
}
