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

// maxGameW caps the game view width (chillerbot frame is 64 tiles wide).
const maxGameW = 66

// Compute splits a w×h screen into the four windows. The game view takes the
// left up to maxGameW, the log takes the remaining right column.
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
	if gameW > maxGameW {
		gameW = maxGameW
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
