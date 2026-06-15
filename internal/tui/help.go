package tui

import "github.com/gdamore/tcell/v2"

// helpLines is the key cheatsheet shown by the '?' overlay (§T28). Always
// escapable with '?' or Esc (§V17).
var helpLines = []string{
	" teetui — keys ",
	"",
	" ?         toggle this help",
	" q / Esc   quit",
	" B         server browser",
	" t / y     chat / team chat",
	" F1 / F2   local / remote console (rcon)",
	" v         toggle game view",
	" Tab       scoreboard",
	" a/d/s     move left / right / stop",
	" space jump   h hook   k kill   e emote",
	" 1-6       select weapon   f fire",
	" arrows    aim (cardinal)",
	" H         auto-reply to last ping",
	" R         reconnect to server",
	" F5 / F6   vote yes / no",
	" PgUp/PgDn / wheel   scroll log",
	"",
	" chat:    !war / !peace / !team / !del <name>",
	" console: spec [name], say <msg>, help",
	"",
	" input: Ctrl-U/K/W kill   Up/Down history",
	"        Ctrl-R search     Tab complete",
	"",
	" keys rebindable: ~/.config/teetui/keymap.txt",
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
	// Clamp to the screen so a small terminal never draws past its bounds (§V30).
	if boxW > w {
		boxW = w
	}
	if boxH > h {
		boxH = h
	}
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
