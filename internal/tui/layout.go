package tui

// Rect is a window rectangle in screen cells.
type Rect struct{ X, Y, W, H int }

// Layout holds the computed window rectangles for one screen size. Mirrors the
// chillerbot terminalui windows: a status bar, the game view, a message log and
// the input line.
type Layout struct {
	Status Rect // top row
	Game   Rect // left body
	Log    Rect // right body
	Input  Rect // bottom row
}

// Layout sizing guards. The game view takes ~2/3 of the width and SCALES with
// the terminal — there is deliberately no fixed upper cap, so a larger terminal
// renders more of the map at higher resolution (§C17/§V31; the old 64-tile
// chillerbot frame is no longer a ceiling). The minimums keep both panes usable
// on a narrow terminal: the log never drops below minLogW while the game can,
// and the game never drops below minGameW while the log can.
const (
	minLogW  = 16
	minGameW = 8
)

// Minimum usable terminal size. Below this the four-window layout cannot be
// drawn legibly, so the UI degrades to a single "resize" notice (§V32/§C17)
// instead of garbling. status(1) + input(1) + a few body rows, and enough
// columns for a minimal game+log split.
const (
	minTermW = 20
	minTermH = 6
)

// tooSmall reports whether the terminal is below the minimum usable size (§V32).
func tooSmall(w, h int) bool { return w < minTermW || h < minTermH }

// Compute splits a w×h screen into the four windows from the CURRENT terminal
// size — called every render so the UI tracks live resizes (§C17/§V30). The game
// view takes ~2/3 of the width and grows with the terminal (no cap); the log
// takes the remaining right column, clamped to a usable minimum.
func Compute(w, h int) Layout {
	if w < 1 {
		w = 1
	}
	if h < 3 {
		h = 3
	}
	bodyY := 1
	bodyH := h - 2 // rows between status (top) and input (bottom)

	gameW := w * 2 / 3
	// Keep the log readable: on a wide split scale the game down so the log gets
	// at least minLogW (when the terminal is wide enough to afford it).
	if w-1-gameW < minLogW {
		gameW = w - 1 - minLogW
	}
	// Keep some game view: on a narrow terminal scale the log down instead.
	if gameW < minGameW {
		gameW = minGameW
	}
	if gameW > w {
		gameW = w
	}
	logX := gameW + 1
	logW := w - logX
	if logW < 0 {
		logW = 0
	}
	return Layout{
		Status: Rect{0, 0, w, 1},
		Game:   Rect{0, bodyY, gameW, bodyH},
		Log:    Rect{logX, bodyY, logW, bodyH},
		Input:  Rect{0, h - 1, w, 1},
	}
}
