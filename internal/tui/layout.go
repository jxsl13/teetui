package tui

// Rect is a window rectangle in screen cells.
type Rect struct{ X, Y, W, H int }

// Layout holds the computed window rectangles for one screen size. Windows stack
// VERTICALLY (§C22): a status bar on top, the full-width game/visual view, the
// full-width log band, and the input/legend line at the bottom. The log band
// sits directly above the input bar; the game sits above the log band.
type Layout struct {
	Status Rect // top row
	Game   Rect // body above the log band (full width; H==0 when visual off)
	Log    Rect // full-width log band, just above the input bar
	Input  Rect // bottom row (input + key legend)
}

// DefaultLogLines is the log-band height (rows) when the visual is on (§C22/§T88).
const DefaultLogLines = 10

// Minimum usable terminal size. Below this the layout cannot be drawn legibly,
// so the UI degrades to a single "resize" notice (§V32/§C17) instead of
// garbling. status(1) + input(1) + a few body rows.
const (
	minTermW = 20
	minTermH = 6
)

// tooSmall reports whether the terminal is below the minimum usable size (§V32).
func tooSmall(w, h int) bool { return w < minTermW || h < minTermH }

// logBandHeight returns the log-band row count for the current size (§C22/§V49).
// With the visual on it is logLines, clamped to [1, ⌊h/2⌋] and to the body — so
// logs never eat more than half the terminal and the game keeps the rest. With
// the visual off the logs fill the whole body.
func logBandHeight(h, bodyH int, visual bool, logLines int) int {
	if !visual {
		return bodyH
	}
	half := h / 2
	n := logLines
	if n < 1 {
		n = 1
	}
	if n > half {
		n = half
	}
	if n > bodyH {
		n = bodyH
	}
	if n < 0 {
		n = 0
	}
	return n
}

// Compute stacks a w×h screen into the four windows from the CURRENT terminal
// size — called every render so the UI tracks live resizes (§C17/§V30/§C22). The
// game/visual fills the body above the log band; the log band sits just above the
// input bar, sized by logBandHeight.
func Compute(w, h int, visual bool, logLines int) Layout {
	if w < 1 {
		w = 1
	}
	if h < 3 {
		h = 3
	}
	bodyY := 1
	bodyH := h - 2 // rows between status (top) and input (bottom)

	logH := logBandHeight(h, bodyH, visual, logLines)
	gameH := bodyH - logH
	if gameH < 0 {
		gameH = 0
	}
	return Layout{
		Status: Rect{0, 0, w, 1},
		Game:   Rect{0, bodyY, w, gameH},
		Log:    Rect{0, bodyY + gameH, w, logH},
		Input:  Rect{0, h - 1, w, 1},
	}
}
