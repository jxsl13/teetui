package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/twclient/client"
)

// runeAt returns the primary rune rendered at (x,y) on the simulation screen.
func runeAt(sim tcell.SimulationScreen, x, y int) rune {
	cells, w, h := sim.GetContents()
	if x < 0 || y < 0 || x >= w || y >= h {
		return 0
	}
	c := cells[y*w+x]
	if len(c.Runes) == 0 {
		return 0
	}
	return c.Runes[0]
}

// §V31/§C17: the game render fills whatever Game rect it is given and keeps the
// local tee centered — at a tiny rect, a normal rect and a huge rect alike, with
// no fixed 64×32 ceiling. The self tee sits at (w/2, h/2) of the rect for any
// size.
func TestRenderFillsRect(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(260, 100)

	// Local tee at tile (5,5); empty map → cells are air except the tee glyph.
	st := client.TickState{
		LocalID: 1,
		Players: map[int]client.CharacterState{1: {X: 5 * tileSize, Y: 5 * tileSize}},
		Map:     client.NewMapView(nil),
	}

	for _, sz := range []struct{ w, h int }{{10, 6}, {40, 20}, {200, 80}} {
		sim.Clear()
		DrawGame(sim, 0, 0, sz.w, sz.h, st)
		sim.Show()
		cx, cy := sz.w/2, sz.h/2
		if got := runeAt(sim, cx, cy); got != 'o' {
			t.Errorf("DrawGame %dx%d: center (%d,%d) = %q want 'o' (tee not centered/scaled)",
				sz.w, sz.h, cx, cy, got)
		}
	}

	// Half-block renderer scales the same way (even heights center cleanly).
	for _, sz := range []struct{ w, h int }{{40, 20}, {200, 80}} {
		sim.Clear()
		DrawGameHalf(sim, 0, 0, sz.w, sz.h, st)
		sim.Show()
		cx, cy := sz.w/2, sz.h/2
		if got := runeAt(sim, cx, cy); got != 'o' {
			t.Errorf("DrawGameHalf %dx%d: center (%d,%d) = %q want 'o'", sz.w, sz.h, cx, cy, got)
		}
	}
}

// §V31: degenerate rect sizes must never panic (1×1, 2×2, 0-dim) — the render
// guards on w<1/h<1 and tcell clips out-of-range writes.
func TestRenderTinyRectNoPanic(t *testing.T) {
	sim := tcell.NewSimulationScreen("UTF-8")
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	defer sim.Fini()
	sim.SetSize(10, 10)
	st := client.TickState{
		LocalID: 1,
		Players: map[int]client.CharacterState{1: {X: 0, Y: 0}},
		Map:     client.NewMapView(nil),
	}
	for _, sz := range []struct{ w, h int }{{1, 1}, {2, 2}, {0, 0}, {3, 1}, {1, 3}} {
		DrawGame(sim, 0, 0, sz.w, sz.h, st)
		DrawGameHalf(sim, 0, 0, sz.w, sz.h, st)
	}
}
