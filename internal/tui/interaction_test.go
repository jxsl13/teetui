package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// This file exhaustively exercises the input mode state machine (§V9): every
// mode's key handling + exit, no key bleed across context switches, and — the
// crux of the request — that keys which are NOT available in the current context
// are correctly ignored (treated as literal text in input modes, or dropped in
// NORMAL / popup / help / browser / search), never firing the action they would
// trigger elsewhere.

// newTestApp builds a simulation-backed App with an isolated (empty) config dir
// so keymap/history load to the deterministic defaults, regardless of the host's
// real ~/.config. No client is connected: comms-bound actions are safe no-ops,
// which is exactly what these pure UI-state tests want.
func newTestApp(t *testing.T) (*App, tcell.SimulationScreen) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)                  // darwin: os.UserConfigDir = $HOME/Library/...
	t.Setenv("XDG_CONFIG_HOME", tmp+"/xc") // linux
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatalf("sim init: %v", err)
	}
	sim.SetSize(120, 40)
	t.Cleanup(sim.Fini)
	app := NewAppWithScreen(sim, "test:8303", NewState(), NewInputController(), NewLog(200))
	app.closePopup() // dismiss the startup greeting so tests start in NORMAL
	return app, sim
}

func rk(r rune) *tcell.EventKey      { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone) }
func sk(k tcell.Key) *tcell.EventKey { return tcell.NewEventKey(k, 0, tcell.ModNone) }

// feedRunes presses each rune of s in turn (text typing / key spamming).
func feedRunes(a *App, s string) {
	for _, r := range s {
		a.handle(rk(r))
	}
}

// snapshot is the observable UI state used to assert "nothing happened".
type uiSnap struct {
	mode       int
	visual     bool
	help       bool
	scoreboard bool
	subcell    bool
	search     bool
	hookOn     bool
	popup      bool
	line       string
}

func snap(a *App) uiSnap {
	return uiSnap{
		mode: a.mode, visual: a.visual, help: a.help, scoreboard: a.scoreboard,
		subcell: a.subcell, search: a.search, hookOn: a.hookOn,
		popup: a.popupActive(), line: a.line.String(),
	}
}

// TestModeTransitions: every mode is enterable and exitable, leaving NORMAL with
// a cleared input line — a total state machine with no dead ends (§V9).
func TestModeTransitions(t *testing.T) {
	app, _ := newTestApp(t)
	if app.mode != modeNormal {
		t.Fatalf("start mode = %d want NORMAL", app.mode)
	}

	cases := []struct {
		name  string
		enter *tcell.EventKey
		want  int
	}{
		{"chat", rk('t'), modeChat},
		{"team chat", rk('y'), modeChatTeam},
		{"local console", sk(tcell.KeyF1), modeLocalCon},
		{"rcon (unauthed)", sk(tcell.KeyF2), modeRconAuth},
	}
	for _, c := range cases {
		app.handle(c.enter)
		if app.mode != c.want {
			t.Errorf("%s: mode = %d want %d", c.name, app.mode, c.want)
		}
		feedRunes(app, "junk")
		app.handle(sk(tcell.KeyEscape)) // every input mode exits on Esc
		if app.mode != modeNormal {
			t.Errorf("%s: Esc did not return to NORMAL (mode=%d)", c.name, app.mode)
		}
		if app.line.String() != "" {
			t.Errorf("%s: line not cleared on exit: %q", c.name, app.line.String())
		}
	}

	// Browser is its own overlay mode; B toggles in and back out.
	app.handle(rk('B'))
	if app.mode != modeBrowser {
		t.Fatalf("B did not open browser (mode=%d)", app.mode)
	}
	app.handle(rk('B'))
	if app.mode != modeNormal {
		t.Fatalf("B did not close browser (mode=%d)", app.mode)
	}

	// Toggles that stay in NORMAL.
	app.handle(sk(tcell.KeyTab)) // scoreboard on
	if !app.scoreboard || app.mode != modeNormal {
		t.Errorf("Tab scoreboard: sb=%v mode=%d", app.scoreboard, app.mode)
	}
	app.handle(sk(tcell.KeyTab)) // off
	if app.scoreboard {
		t.Error("Tab did not toggle scoreboard back off")
	}
	app.handle(rk('v')) // visual off (default on)
	if app.visual {
		t.Error("v did not toggle visual off")
	}
	app.handle(rk('v'))
	if !app.visual {
		t.Error("v did not toggle visual back on")
	}
	app.handle(rk('V')) // sub-cell render toggle
	if !app.subcell {
		t.Error("V did not toggle sub-cell on")
	}
}

// TestNoKeyBleedIntoChat: NORMAL-mode action keys (v, B, k, ?, Z, T, …) must be
// inserted as LITERAL TEXT while in chat — they must NOT fire their NORMAL action
// (no bleed across the context switch, §V9). Tab in an input mode is completion,
// not the scoreboard.
func TestNoKeyBleedIntoChat(t *testing.T) {
	app, _ := newTestApp(t)
	before := snap(app)

	app.handle(rk('t')) // → chat
	feedRunes(app, "vBk?Z")

	if app.mode != modeChat {
		t.Fatalf("mode = %d want chat", app.mode)
	}
	if got := app.line.String(); got != "vBk?Z" {
		t.Errorf("typed line = %q want \"vBk?Z\" (action keys must be literal text)", got)
	}
	// None of the actions those keys bind to may have fired.
	if app.visual != before.visual {
		t.Error("'v' bled through: visual toggled while typing in chat")
	}
	if app.help {
		t.Error("'?' bled through: help opened while typing in chat")
	}
	if app.scoreboard {
		t.Error("scoreboard toggled while typing in chat")
	}

	// Tab inside chat = completion attempt, NOT scoreboard toggle.
	app.handle(sk(tcell.KeyTab))
	if app.scoreboard {
		t.Error("Tab toggled scoreboard from chat mode (should be completion)")
	}

	// After leaving chat, the same keys work as actions again (re-selectable).
	app.handle(sk(tcell.KeyEscape))
	app.handle(rk('v'))
	if app.visual == before.visual {
		t.Error("'v' did not act after returning to NORMAL (binding lost)")
	}
}

// TestUnboundKeysIgnoredInNormal: keys that are NOT bound to any action and are
// not one of the parametric NORMAL controls must be silently dropped — pressing
// them changes nothing and never panics (the "not available as an input button"
// case).
func TestUnboundKeysIgnoredInNormal(t *testing.T) {
	app, _ := newTestApp(t)
	want := snap(app)

	// Unbound runes (none of these appear in DefaultKeymap or weaponForRune).
	feedRunes(app, "zxpgmnuioc")
	// Unbound special keys.
	for _, k := range []tcell.Key{tcell.KeyF3, tcell.KeyF7, tcell.KeyF12, tcell.KeyInsert, tcell.KeyCtrlA} {
		app.handle(sk(k))
	}

	if got := snap(app); got != want {
		t.Errorf("unbound keys changed state:\n got  %+v\n want %+v", got, want)
	}
}

// TestWeaponAndUnboundDigits: digits 1-6 select a weapon (no panic, stays
// NORMAL); 0/7/8/9 are not weapon keys and are ignored.
func TestWeaponAndUnboundDigits(t *testing.T) {
	app, _ := newTestApp(t)
	feedRunes(app, "1234560789")
	if app.mode != modeNormal {
		t.Errorf("digit keys changed mode to %d", app.mode)
	}
	if app.popupActive() || app.help || app.scoreboard {
		t.Error("digit keys triggered an unexpected UI change")
	}
}

// TestHelpOverlayTrapsNothing: while help is shown, only ? / Esc act (close);
// every other key is ignored — help never traps the user and never lets a key
// bleed into an action (§V17).
func TestHelpOverlayTrapsNothing(t *testing.T) {
	app, _ := newTestApp(t)
	app.handle(rk('?'))
	if !app.help {
		t.Fatal("? did not open help")
	}
	// Spam keys that would otherwise do things; help must swallow them.
	feedRunes(app, "tvBkZ")
	app.handle(sk(tcell.KeyTab))
	app.handle(sk(tcell.KeyF1))
	if !app.help {
		t.Error("help closed by a non-close key")
	}
	if app.mode != modeNormal {
		t.Errorf("a key bled into a mode change while help shown (mode=%d)", app.mode)
	}
	// Both advertised close keys work.
	app.handle(sk(tcell.KeyEscape))
	if app.help {
		t.Error("Esc did not close help")
	}
	app.handle(rk('?'))
	app.handle(rk('?'))
	if app.help {
		t.Error("? did not toggle help closed")
	}
}

// TestPopupGating: a popup intercepts keys (§V21). The greeting popup acts on its
// advertised B/?/Enter; a non-advertised key (v) is swallowed and does NOT toggle
// visual. A non-greeting (disconnect) popup does not open the browser on B.
func TestPopupGating(t *testing.T) {
	// Greeting: non-advertised key swallowed.
	app, _ := newTestApp(t)
	app.popup = greetingPopup() // restore greeting
	visualBefore := app.visual
	app.handle(rk('v')) // not advertised by the popup
	if !app.popupActive() {
		t.Error("non-advertised key closed the popup")
	}
	if app.visual != visualBefore {
		t.Error("'v' acted through the greeting popup (should be swallowed)")
	}

	// Greeting: advertised B closes popup AND opens browser (§V21/§B1).
	app.handle(rk('B'))
	if app.popupActive() {
		t.Error("B did not close the greeting popup")
	}
	if app.mode != modeBrowser {
		t.Errorf("B on greeting did not open browser (mode=%d)", app.mode)
	}

	// Disconnect popup: B closes it but must NOT open the browser (only greeting
	// advertises B).
	app2, _ := newTestApp(t)
	app2.popup = disconnectPopup("kicked")
	app2.handle(rk('B'))
	if app2.popupActive() {
		t.Error("B did not dismiss the disconnect popup")
	}
	if app2.mode == modeBrowser {
		t.Error("B opened the browser from a non-greeting popup (should not)")
	}
}

// TestBrowserKeyIsolation: in the browser overlay, NORMAL game actions are not
// reachable — 'v' does not toggle visual, 'k' does not kill; only browser keys
// and Esc act. Search focus captures text until Esc.
func TestBrowserKeyIsolation(t *testing.T) {
	app, _ := newTestApp(t)
	visualBefore := app.visual
	app.handle(rk('B'))
	if app.mode != modeBrowser {
		t.Fatalf("not in browser (mode=%d)", app.mode)
	}
	app.handle(rk('v')) // would toggle visual in NORMAL
	if app.visual != visualBefore {
		t.Error("'v' toggled visual from inside the browser overlay")
	}

	// '/' focuses the search box; typed keys go to the term, not actions.
	app.handle(rk('/'))
	if !app.browser.SearchFocused() {
		t.Fatal("/ did not focus browser search")
	}
	feedRunes(app, "vk") // captured as search text, not actions
	app.handle(sk(tcell.KeyEscape))
	if app.browser.SearchFocused() {
		t.Error("Esc did not unfocus browser search")
	}
	// Esc again leaves the browser back to NORMAL.
	app.handle(sk(tcell.KeyEscape))
	if app.mode != modeNormal {
		t.Errorf("Esc did not exit browser (mode=%d)", app.mode)
	}
}

// TestSearchOverlayIsolation: the reverse-i-search overlay (Ctrl-R) captures
// keys; NORMAL/chat actions do not fire while it is active, and Esc exits it
// back into the underlying input mode (§T14).
func TestSearchOverlayIsolation(t *testing.T) {
	app, _ := newTestApp(t)
	app.handle(rk('t'))            // chat mode (has a history → search allowed)
	app.handle(sk(tcell.KeyCtrlR)) // open reverse-i-search
	if !app.search {
		t.Fatal("Ctrl-R did not open search")
	}
	visualBefore := app.visual
	feedRunes(app, "v") // appended to search term, NOT a visual toggle
	if app.visual != visualBefore {
		t.Error("'v' toggled visual while reverse-i-search active")
	}
	if string(app.searchTerm) != "v" {
		t.Errorf("search term = %q want \"v\"", string(app.searchTerm))
	}
	app.handle(sk(tcell.KeyEscape))
	if app.search {
		t.Error("Esc did not close search overlay")
	}
	if app.mode != modeChat {
		t.Errorf("search exit dropped out of chat mode (mode=%d)", app.mode)
	}
}

// TestCrossSequenceConsistency: long interleaved sequences of mode switches and
// key spam always settle into a consistent, usable state — Esc out of anything
// reaches NORMAL with an empty line and no overlay, and the chat-echo path still
// works afterward (no stuck mode / key-bleed accumulation, §V9).
func TestCrossSequenceConsistency(t *testing.T) {
	app, _ := newTestApp(t)
	seqs := []func(){
		func() { app.handle(rk('t')); feedRunes(app, "hi") },
		func() { app.handle(sk(tcell.KeyF1)); feedRunes(app, "help") },
		func() { app.handle(rk('B')); app.handle(rk('/')); feedRunes(app, "ddnet") },
		func() { app.handle(rk('?')) },
		func() { app.handle(rk('y')); feedRunes(app, "team") },
		func() { app.handle(sk(tcell.KeyF2)); feedRunes(app, "secret") },
	}
	for i, s := range seqs {
		s()
		// Hard reset to NORMAL the way a user bails out: close overlays/popups,
		// leave any input mode. Two Escs cover search→mode and mode→normal /
		// browser-search→browser→normal.
		if app.search {
			app.handle(sk(tcell.KeyEscape))
		}
		if app.help {
			app.handle(rk('?'))
		}
		if app.popupActive() {
			app.closePopup()
		}
		app.handle(sk(tcell.KeyEscape))
		app.handle(sk(tcell.KeyEscape))
		if app.mode != modeNormal {
			t.Fatalf("seq %d: not NORMAL after escape (mode=%d)", i, app.mode)
		}
		if app.line.String() != "" {
			t.Fatalf("seq %d: line not empty after escape: %q", i, app.line.String())
		}
		app.draw() // must render in any settled state without panicking
	}

	// After all that churn the input subsystem is still fully usable end to end:
	// enter chat, type, submit → back to NORMAL with a cleared line. (No client is
	// connected, so the send itself is a no-op; the §V29 local echo is covered by
	// the live e2e suite and chatecho_test.) This proves no accumulated key-bleed
	// left a mode stuck or the line buffer wedged.
	app.handle(rk('t'))
	if app.mode != modeChat {
		t.Fatal("cannot enter chat after cross-sequence churn")
	}
	feedRunes(app, "still-works")
	if app.line.String() != "still-works" {
		t.Fatalf("typing wedged after churn: line=%q", app.line.String())
	}
	app.handle(sk(tcell.KeyEnter))
	if app.mode != modeNormal || app.line.String() != "" {
		t.Fatalf("submit wedged after churn: mode=%d line=%q", app.mode, app.line.String())
	}
}
