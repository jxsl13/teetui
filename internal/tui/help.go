package tui

import "github.com/gdamore/tcell/v2"

// helpLines is the key cheatsheet shown by the '?' overlay (§T28). Always
// escapable with '?' or Esc (§V17).
var helpLines = []string{
	" teetui — keys ",
	"",
	" ?         toggle this help",
	" q / Esc   quit",
	" t         chat (all)",
	" y         team chat",
	" v         toggle visual (game render)",
	" k         self-kill",
	" Tab       toggle scoreboard",
	" PgUp/PgDn scroll log",
	" a/d/s     move left/right/stop",
	" space     jump    h/H hook on/off",
	"",
	" chat input:",
	" Ctrl-U/K/W kill to start/end/word",
	" Up/Down    history    Ctrl-R search",
	"",
	" press ? or Esc to close ",
}

// drawHelp renders the help box centered on the screen.
func drawHelp(s tcell.Screen, w, h int) {
	boxW := 0
	for _, l := range helpLines {
		if len(l) > boxW {
			boxW = len(l)
		}
	}
	boxW += 2
	boxH := len(helpLines) + 2
	x0 := (w - boxW) / 2
	y0 := (h - boxH) / 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	style := StyleStatus
	for row := 0; row < boxH; row++ {
		for col := 0; col < boxW; col++ {
			s.SetContent(x0+col, y0+row, ' ', nil, style)
		}
	}
	for i, l := range helpLines {
		drawStr(s, x0+1, y0+1+i, boxW-1, style, l)
	}
}
