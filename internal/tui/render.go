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
// cameraCenter returns the tile coords the game view centers on. It prefers the
// local tee, but when there is none — spectating / free-view (§V27/§B6) — it
// follows the lowest-id visible player, then falls back to the map center, so
// the view renders instead of sitting on "connecting…". ok is false only when
// there is nothing to anchor on at all.
func cameraCenter(st client.TickState) (cx, cy int, ok bool) {
	if self, has := st.Players[st.LocalID]; has {
		return self.X / tileSize, self.Y / tileSize, true
	}
	best := -1
	for id := range st.Players {
		if best < 0 || id < best {
			best = id
		}
	}
	if best >= 0 {
		ch := st.Players[best]
		return ch.X / tileSize, ch.Y / tileSize, true
	}
	if st.Map != nil && st.Map.Width() > 0 {
		return st.Map.Width() / 2, st.Map.Height() / 2, true
	}
	return 0, 0, false
}

func DrawGame(s tcell.Screen, x0, y0, w, h int, st client.TickState) {
	if w < 1 || h < 1 {
		return
	}
	cx, cy, ok := cameraCenter(st)
	if st.Map == nil || !ok {
		drawStr(s, x0, y0, w, StyleSystem, "connecting…")
		return
	}
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
	// Self tee on top — only when we have a local character (not while spectating).
	if self, has := st.Players[st.LocalID]; has {
		if self.HookState > 0 {
			plot(self.HookX, self.HookY, '+', StyleHook)
		}
		g := 'o'
		if self.Weapon == weaponNinja {
			g = 'ø'
		}
		plot(self.X, self.Y, g, StyleSelf)
	}

	// HUD: camera-center tile coordinates (local tee, or follow target while
	// spectating, §T34/§V27).
	drawStr(s, x0, y0+h-1, w, StyleSystem, hudText(cx, cy))
}

// tileColorAt returns the render color and draw flag for the map tile at
// (tx,ty), applying the race start/finish/checkpoint overlay on top of the base
// tile class — the same precedence DrawGame uses, but glyph-free since the
// half-block renderer only needs the color of each map cell (§T46).
func tileColorAt(m *client.MapView, tx, ty int) (tcell.Style, bool) {
	_, style, draw := tileGlyph(m.Tile(tx, ty))
	if _, ss, ok := specialGlyph(m.Start(tx, ty), m.Finish(tx, ty), m.Checkpoint(tx, ty)); ok {
		return ss, true
	}
	return style, draw
}

// DrawGameHalf renders the map like DrawGame but packs two stacked map rows into
// each terminal cell via half-block glyphs, doubling the vertical resolution for
// finer map detail (§C11/§V20/§T46). The camera stays centered on the local tee;
// tees and entities are drawn single-cell on top so they remain legible. It
// performs no per-frame heap allocation on the tile loop (§V7) and never panics
// on an empty state.
func DrawGameHalf(s tcell.Screen, x0, y0, w, h int, st client.TickState) {
	if w < 1 || h < 1 {
		return
	}
	cx, cy, ok := cameraCenter(st)
	if st.Map == nil || !ok {
		drawStr(s, x0, y0, w, StyleSystem, "connecting…")
		return
	}
	halfW := w / 2
	// 2*h map rows are shown, vertically centered on cy → top map row is cy-h.
	topRow := cy - h

	for row := 0; row < h; row++ {
		ty := topRow + 2*row
		for col := 0; col < w; col++ {
			tx := cx + (col - halfW)
			topStyle, topDraw := tileColorAt(st.Map, tx, ty)
			botStyle, botDraw := tileColorAt(st.Map, tx, ty+1)
			g, style := halfBlockCell(topDraw, topStyle, botDraw, botStyle)
			s.SetContent(x0+col, y0+row, g, nil, style)
		}
	}

	// plot maps a world position to its half-resolution cell and draws an entity
	// glyph single-cell on top of the half-block map.
	plot := func(wx, wy int, g rune, style tcell.Style) {
		col := halfW + (wx/tileSize - cx)
		row := (wy/tileSize - topRow) / 2
		if col < 0 || col >= w || row < 0 || row >= h {
			return
		}
		s.SetContent(x0+col, y0+row, g, nil, style)
	}

	for _, l := range st.Lasers {
		plot(l.X, l.Y, '·', StyleLaser)
		plot(l.FromX, l.FromY, '·', StyleLaser)
	}
	for _, p := range st.Projectiles {
		plot(p.X, p.Y, '•', StyleProjectile)
	}

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
	if self, has := st.Players[st.LocalID]; has {
		if self.HookState > 0 {
			plot(self.HookX, self.HookY, '+', StyleHook)
		}
		g := 'o'
		if self.Weapon == weaponNinja {
			g = 'ø'
		}
		plot(self.X, self.Y, g, StyleSelf)
	}

	drawStr(s, x0, y0+h-1, w, StyleSystem, hudText(cx, cy))
}

// hudText formats the in-game coordinate readout.
func hudText(tx, ty int) string {
	return fmt.Sprintf(" x:%d y:%d ", tx, ty)
}
