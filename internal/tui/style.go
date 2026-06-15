package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/twclient/client"
)

// fg builds a style with a truecolor foreground on the default background.
// tcell downsamples the RGB value to the 256- or 16-color palette on terminals
// that lack truecolor, so the same call is correct on every OS (§V5).
func fg(r, g, b int32) tcell.Style {
	return tcell.StyleDefault.Foreground(tcell.NewRGBColor(r, g, b))
}

// Entity styles, ported from the chillerbot-ux renderer RGB values
// (src/.../chillerbot, renderer.go in twclient cmd/ml).
var (
	StyleSelf       = fg(255, 50, 50)   // own tee — red
	StyleOther      = fg(60, 120, 255)  // other tees — blue
	StyleHook       = fg(255, 230, 0)   // hook — yellow
	StyleProjectile = fg(255, 160, 0)   // projectiles — orange
	StyleLaser      = fg(180, 0, 255)   // laser beams — violet
	StyleStatus     = tcell.StyleDefault.Reverse(true)
	StyleChat       = fg(220, 220, 220)
	StyleSystem     = fg(255, 220, 120)
	StyleStart      = fg(0, 255, 0)     // race start — green
	StyleFinish     = fg(255, 0, 255)   // race finish — magenta
	StyleCheckpoint = fg(255, 180, 0)   // race checkpoint — orange (← chillerbot)
)

// specialGlyph returns the glyph/style overlay for race start/finish/checkpoint
// tiles, which MapView.Tile does not classify (§T43/§T47). These render on top
// of the base tile class so the route is readable beyond the chillerbot
// 6-color palette. Precedence: finish > start > checkpoint.
func specialGlyph(start, finish, checkpoint bool) (rune, tcell.Style, bool) {
	switch {
	case finish:
		return 'F', StyleFinish, true
	case start:
		return 'S', StyleStart, true
	case checkpoint:
		return 'C', StyleCheckpoint, true
	default:
		return ' ', tcell.StyleDefault, false
	}
}

// tileGlyph returns the rune, style and draw flag for a map tile class. Glyphs
// are kept single-width so the cell grid stays aligned (§V6); colors mirror the
// chillerbot tile palette.
func tileGlyph(t client.TileClass) (rune, tcell.Style, bool) {
	switch t {
	case client.ClassSolid:
		return '█', fg(180, 180, 180), true
	case client.ClassUnhook:
		return '▓', fg(100, 100, 200), true
	case client.ClassFreeze:
		return '▒', fg(0, 180, 255), true
	case client.ClassDeath:
		return 'x', fg(200, 40, 40), true
	case client.ClassHookThrough:
		return '#', fg(120, 120, 80), true
	case client.ClassTele:
		return 'T', fg(200, 100, 255), true
	case client.ClassSpeedup:
		return '»', fg(255, 255, 0), true
	case client.ClassSwitch:
		return 'S', fg(255, 180, 0), true
	default: // ClassAir
		return ' ', tcell.StyleDefault, false
	}
}
