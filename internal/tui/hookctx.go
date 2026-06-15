package tui

import "github.com/gdamore/tcell/v2"

// hookKeyNames names the non-rune keys exposed to feature OnKey handlers (§T76).
var hookKeyNames = map[tcell.Key]string{
	tcell.KeyF1: "F1", tcell.KeyF2: "F2", tcell.KeyF3: "F3", tcell.KeyF4: "F4",
	tcell.KeyF5: "F5", tcell.KeyF6: "F6", tcell.KeyF7: "F7", tcell.KeyF8: "F8",
	tcell.KeyF9: "F9", tcell.KeyF10: "F10", tcell.KeyF11: "F11", tcell.KeyF12: "F12",
	tcell.KeyEnter: "Enter", tcell.KeyEscape: "Esc", tcell.KeyTab: "Tab",
	tcell.KeyUp: "Up", tcell.KeyDown: "Down", tcell.KeyLeft: "Left", tcell.KeyRight: "Right",
	tcell.KeyPgUp: "PgUp", tcell.KeyPgDn: "PgDn",
}
