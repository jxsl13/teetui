package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// §C22/§V48/§V49: the layout is a vertical stack — full-width status/game/log/
// input; the log band sits directly above the input bar; visual on caps the log
// band at ⌊h/2⌋ with the game filling the rest; visual off → logs fill the body.
func TestComputeVerticalStack(t *testing.T) {
	const w, h = 100, 40
	l := Compute(w, h, true, DefaultLogLines)

	// Full-width stack, top→bottom: status(0) / game / log / input(h-1).
	if l.Status.Y != 0 || l.Status.W != w {
		t.Errorf("status = %+v", l.Status)
	}
	if l.Input.Y != h-1 || l.Input.W != w {
		t.Errorf("input = %+v", l.Input)
	}
	if l.Game.W != w || l.Log.W != w {
		t.Errorf("body not full width: game.W=%d log.W=%d", l.Game.W, l.Log.W)
	}
	// Log band directly above the input bar.
	if l.Log.Y+l.Log.H != l.Input.Y {
		t.Errorf("log band not above input: log.Y=%d log.H=%d input.Y=%d", l.Log.Y, l.Log.H, l.Input.Y)
	}
	// Game directly above the log band.
	if l.Game.Y+l.Game.H != l.Log.Y {
		t.Errorf("game not above log: game.Y=%d game.H=%d log.Y=%d", l.Game.Y, l.Game.H, l.Log.Y)
	}
	// Visual on, plenty of room → log band == configured count.
	if l.Log.H != DefaultLogLines {
		t.Errorf("log band = %d want %d", l.Log.H, DefaultLogLines)
	}

	// Cap: a huge requested count is clamped to ⌊h/2⌋ when visual on.
	if l := Compute(w, h, true, 1000); l.Log.H != h/2 {
		t.Errorf("log band cap = %d want %d", l.Log.H, h/2)
	}

	// Visual off → logs fill the whole body, no game.
	off := Compute(w, h, false, DefaultLogLines)
	if off.Game.H != 0 {
		t.Errorf("visual off game.H = %d want 0", off.Game.H)
	}
	if off.Log.H != h-2 {
		t.Errorf("visual off log.H = %d want %d (full body)", off.Log.H, h-2)
	}
}

// §V49: the log band never exceeds half the height across sizes, and nothing
// goes negative or overflows.
func TestLogBandCap(t *testing.T) {
	for _, h := range []int{6, 10, 24, 41, 80} {
		l := Compute(80, h, true, DefaultLogLines)
		if l.Log.H > h/2 {
			t.Errorf("h=%d log band %d exceeds half %d", h, l.Log.H, h/2)
		}
		if l.Game.H < 0 || l.Log.H < 0 {
			t.Errorf("h=%d negative pane: game=%d log=%d", h, l.Game.H, l.Log.H)
		}
		if l.Log.Y+l.Log.H > h-1 {
			t.Errorf("h=%d log overruns into input bar", h)
		}
	}
}

// §V30: overlays clamp to the screen — a tiny terminal must never draw a popup
// or help box outside its bounds, and must not panic.
func TestOverlaysClampToScreen(t *testing.T) {
	for _, sz := range []struct{ w, h int }{{6, 4}, {12, 6}, {3, 3}, {1, 1}} {
		scr := tcell.NewSimulationScreen("UTF-8")
		if err := scr.Init(); err != nil {
			t.Fatal(err)
		}
		scr.SetSize(sz.w, sz.h)
		// These must not panic at any size; tcell ignores out-of-range SetContent,
		// and the box dims are clamped to the screen.
		drawHelp(scr, sz.w, sz.h, []string{" teetui — keys ", " ? help", " q quit"})
		drawPopup(scr, sz.w, sz.h, greetingPopup())
		drawPopup(scr, sz.w, sz.h, disconnectPopup("kicked"))
		scr.Show()
		scr.Fini()
	}
}
