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
	if !hasLabel(app, "Kill") || !hasLabel(app, "Pause") {
		t.Errorf("in-game menu missing Kill/Pause: %v", menuLabels(app))
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
