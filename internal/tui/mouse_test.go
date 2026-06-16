package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// §T107/§V69: teetui is keyboard-only — a mouse event is inert (no scroll, no
// panic); log scrolling works via PgUp/PgDn.
func TestNoMouseInput(t *testing.T) {
	app, _ := newTestApp(t)
	for i := 0; i < 30; i++ {
		app.log.Addf(tcell.StyleDefault, "L%d", i)
	}
	tail := func() string {
		v := app.log.View(80, 5)
		return v[len(v)-1].Text
	}
	if tail() != "L29" {
		t.Fatalf("pre: tail = %q want L29", tail())
	}

	// A mouse wheel event must NOT scroll the log (mouse is ignored).
	app.handle(tcell.NewEventMouse(0, 0, tcell.WheelUp, tcell.ModNone))
	if tail() != "L29" {
		t.Errorf("mouse wheel scrolled the log: tail = %q", tail())
	}

	// PgUp still scrolls (keyboard).
	app.handle(sk(tcell.KeyPgUp))
	if tail() == "L29" {
		t.Error("PgUp did not scroll the log")
	}
}
