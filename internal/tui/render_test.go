package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/twclient/client"
	"github.com/jxsl13/twclient/packet"
)

// §T46/§C11: halfBlockCell composes two stacked map rows into one terminal cell.
func TestHalfBlockCell(t *testing.T) {
	top := fg(255, 0, 0)
	bot := fg(0, 0, 255)
	topFg, _, _ := top.Decompose()
	botFg, _, _ := bot.Decompose()

	// neither half drawn → blank cell.
	if g, st := halfBlockCell(false, top, false, bot); g != ' ' || st != tcell.StyleDefault {
		t.Errorf("neither = %q,%v want space/default", g, st)
	}
	// top only → upper half-block in the top color.
	if g, st := halfBlockCell(true, top, false, bot); g != '▀' {
		t.Errorf("top glyph = %q want ▀", g)
	} else if f, _, _ := st.Decompose(); f != topFg {
		t.Errorf("top fg = %v want %v", f, topFg)
	}
	// bottom only → lower half-block in the bottom color.
	if g, st := halfBlockCell(false, top, true, bot); g != '▄' {
		t.Errorf("bottom glyph = %q want ▄", g)
	} else if f, _, _ := st.Decompose(); f != botFg {
		t.Errorf("bottom fg = %v want %v", f, botFg)
	}
	// both → upper half-block, fg=top, bg=bottom.
	if g, st := halfBlockCell(true, top, true, bot); g != '▀' {
		t.Errorf("both glyph = %q want ▀", g)
	} else {
		f, b, _ := st.Decompose()
		if f != topFg || b != botFg {
			t.Errorf("both fg/bg = %v/%v want %v/%v", f, b, topFg, botFg)
		}
	}
}

// §T46/§V11: the half-block renderer must not panic on a sim screen, both with
// an empty state and with a populated tick + map (exercising the tile loop).
func TestDrawGameHalfNoPanic(t *testing.T) {
	scr := tcell.NewSimulationScreen("")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()
	scr.SetSize(80, 24)

	DrawGameHalf(scr, 0, 0, 40, 20, client.TickState{}) // no players, no map

	st := client.TickState{
		LocalID: 1,
		Map:     client.NewMapView(nil), // empty all-solid view exercises the loop
		Players: map[int]client.CharacterState{
			1: {X: 320, Y: 320, HookState: 1, HookX: 352, HookY: 320},
			2: {X: 352, Y: 352, Weapon: weaponNinja},
		},
		Projectiles: []packet.ProjectileState{{X: 300, Y: 300}},
	}
	DrawGameHalf(scr, 0, 0, 40, 20, st)
}
