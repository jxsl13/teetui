package tui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// §T96/§V56: the help overlay is generated from the live keymap + feature
// actions — it lists every binding, tracks rebinds, and stays escapable (§V17).

func helpText(a *App) string { return strings.Join(a.helpLines(), "\n") }

func TestHelpListsRebindAndFeatureAction(t *testing.T) {
	app, _ := newTestApp(t)

	if !strings.Contains(helpText(app), "server browser") {
		t.Fatal("help missing core command")
	}

	// Rebind browser to 'm' → help must show the live binding (§V19/§V56).
	app.keymap.clearAction(actBrowser)
	app.keymap.bindRune('m', actBrowser)
	h := helpText(app)
	if !strings.Contains(h, "m") || strings.Contains(h, " B ") {
		t.Errorf("help did not reflect rebind:\n%s", h)
	}

	// A registered feature action appears with its help text.
	app.featActions = append(app.featActions, featAction{name: "reply_to_ping", key: "H", help: "reply to the last chat ping"})
	h = helpText(app)
	if !strings.Contains(h, "features") || !strings.Contains(h, "reply to the last chat ping") {
		t.Errorf("help missing feature action group:\n%s", h)
	}
}

// §T108/§V70: the help overlay explains each mode — how to enter, what it is,
// and that Esc leaves text modes.
func TestHelpExplainsModes(t *testing.T) {
	app, _ := newTestApp(t)
	h := helpText(app)
	for _, want := range []string{
		"modes",                 // a dedicated modes section
		"local console",         // names the console mode
		"cvars",                 // explains WHAT the console is
		"remote console (rcon)", // names rcon
		"rcon password",         // explains rcon needs a password
		"server browser",        // names the browser
		"free-look",             // names free-look
		"Esc",                   // tells how to leave
	} {
		if !strings.Contains(h, want) {
			t.Errorf("help modes section missing %q:\n%s", want, h)
		}
	}
	// each mode's enter key is present (F1 console / F2 rcon).
	if !strings.Contains(h, "F1") || !strings.Contains(h, "F2") {
		t.Errorf("help missing console enter keys:\n%s", h)
	}
}

// The help overlay opens and closes from NORMAL via the bound keys (§V17).
func TestHelpEscapable(t *testing.T) {
	app, _ := newTestApp(t)
	app.handle(rk('?'))
	if !app.help {
		t.Fatal("? did not open help")
	}
	app.handle(sk(tcell.KeyEscape))
	if app.help {
		t.Fatal("Esc did not close help")
	}
	app.handle(rk('?'))
	app.handle(rk('?'))
	if app.help {
		t.Fatal("? did not toggle help closed")
	}
}
