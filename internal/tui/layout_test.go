package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

// §V30/§C17: the layout is fully responsive — the game view scales UP with the
// terminal width (no fixed cap), both panes stay within bounds, and the log
// keeps a usable minimum width when the terminal can afford it.
func TestComputeResponsive(t *testing.T) {
	// The game view must grow as the terminal widens (no maxGameW ceiling).
	prev := 0
	for _, w := range []int{80, 160, 320, 640} {
		l := Compute(w, 40)
		if l.Game.W <= prev {
			t.Errorf("game width did not scale up at w=%d: %d (prev %d)", w, l.Game.W, prev)
		}
		prev = l.Game.W
		// Panes within bounds, non-overlapping, log right of game.
		if l.Game.W < 0 || l.Game.W > w {
			t.Errorf("w=%d game width %d out of bounds", w, l.Game.W)
		}
		if l.Log.X+l.Log.W > w {
			t.Errorf("w=%d log overflows: x=%d w=%d (screen %d)", w, l.Log.X, l.Log.W, w)
		}
		if l.Log.X != l.Game.W+1 {
			t.Errorf("w=%d log not adjacent to game: logX=%d gameW=%d", w, l.Log.X, l.Game.W)
		}
		// On a comfortably wide terminal the log keeps its minimum.
		if w >= 80 && l.Log.W < minLogW {
			t.Errorf("w=%d log width %d below min %d", w, l.Log.W, minLogW)
		}
	}

	// Narrow terminal: game shrinks toward its minimum, nothing goes negative.
	for _, w := range []int{10, 20, 30} {
		l := Compute(w, 10)
		if l.Game.W < minGameW && w > minGameW {
			t.Errorf("w=%d game below min: %d", w, l.Game.W)
		}
		if l.Log.W < 0 || l.Game.W < 0 {
			t.Errorf("w=%d negative pane: game=%d log=%d", w, l.Game.W, l.Log.W)
		}
		if l.Log.X+l.Log.W > w {
			t.Errorf("w=%d log overflows screen", w)
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
		drawHelp(scr, sz.w, sz.h)
		drawPopup(scr, sz.w, sz.h, greetingPopup())
		drawPopup(scr, sz.w, sz.h, disconnectPopup("kicked"))
		scr.Show()
		scr.Fini()
	}
}
