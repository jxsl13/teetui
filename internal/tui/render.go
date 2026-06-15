package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/twclient/client"
	"github.com/mattn/go-runewidth"
)

// tileSize is the world-unit size of one map tile; one tile renders to one
// terminal cell (chillerbot terminalui uses a fixed 1-char tile).
const tileSize = 32

// weaponNinja is the weapon id whose tee renders with the ninja glyph.
const weaponNinja = 6

// drawStr writes s at (x,y) advancing by display width so wide runes do not
// corrupt the grid (§V6). It stops at the right clip bound x+maxW.
func drawStr(s tcell.Screen, x, y, maxW int, style tcell.Style, str string) int {
	cx := x
	for _, r := range str {
		w := runewidth.RuneWidth(r)
		if w == 0 {
			w = 1
		}
		if cx+w > x+maxW {
			break
		}
		s.SetContent(cx, y, r, nil, style)
		cx += w
	}
	return cx - x
}

// padCol truncates s to w display columns and right-pads with spaces, so table
// columns line up regardless of wide/utf8 runes (§V6).
func padCol(s string, w int) string {
	if w < 0 {
		w = 0
	}
	s = runewidth.Truncate(s, w, "")
	return s + strings.Repeat(" ", w-runewidth.StringWidth(s))
}

// DrawGame renders the map and entities into the rectangle (x0,y0,w,h), camera
// centered on the local tee. State arrives via the Observer (§V2); only changed
// cells are touched and no per-frame allocation occurs on the tile loop (§V7).
func DrawGame(s tcell.Screen, x0, y0, w, h int, st client.TickState) {
	if w < 1 || h < 1 {
		return
	}
	self, ok := st.Players[st.LocalID]
	if !ok || st.Map == nil {
		drawStr(s, x0, y0, w, StyleSystem, "connecting…")
		return
	}
	cx := self.X / tileSize
	cy := self.Y / tileSize
	halfW := w / 2
	halfH := h / 2

	// Map tiles, with race start/finish overlaid on top of the base class so the
	// route reads clearly (§T43, §V20 — richer than the 6-color reference).
	for row := 0; row < h; row++ {
		ty := cy + (row - halfH)
		for col := 0; col < w; col++ {
			tx := cx + (col - halfW)
			g, style, draw := tileGlyph(st.Map.Tile(tx, ty))
			if sg, ss, ok := specialGlyph(st.Map.Start(tx, ty), st.Map.Finish(tx, ty), st.Map.Checkpoint(tx, ty)); ok {
				g, style, draw = sg, ss, true
			}
			if draw {
				s.SetContent(x0+col, y0+row, g, nil, style)
			}
		}
	}

	plot := func(wx, wy int, g rune, style tcell.Style) {
		col := halfW + (wx/tileSize - cx)
		row := halfH + (wy/tileSize - cy)
		if col < 0 || col >= w || row < 0 || row >= h {
			return
		}
		s.SetContent(x0+col, y0+row, g, nil, style)
	}

	// Lasers (endpoints) and projectiles.
	for _, l := range st.Lasers {
		plot(l.X, l.Y, '·', StyleLaser)
		plot(l.FromX, l.FromY, '·', StyleLaser)
	}
	for _, p := range st.Projectiles {
		plot(p.X, p.Y, '•', StyleProjectile)
	}

	// Tees (self drawn last, on top).
	for id, ch := range st.Players {
		if id == st.LocalID {
			continue
		}
		if ch.HookState > 0 {
			plot(ch.HookX, ch.HookY, '+', StyleHook)
		}
		g := 'o'
		if ch.Weapon == weaponNinja {
			g = 'ø'
		}
		plot(ch.X, ch.Y, g, StyleOther)
	}
	if self.HookState > 0 {
		plot(self.HookX, self.HookY, '+', StyleHook)
	}
	g := 'o'
	if self.Weapon == weaponNinja {
		g = 'ø'
	}
	plot(self.X, self.Y, g, StyleSelf)

	// HUD: live local-tee tile coordinates (§T34).
	drawStr(s, x0, y0+h-1, w, StyleSystem, hudText(cx, cy))
}

// hudText formats the in-game coordinate readout.
func hudText(tx, ty int) string {
	return fmt.Sprintf(" x:%d y:%d ", tx, ty)
}
