package tui

import "github.com/gdamore/tcell/v2"

// popupKind classifies a popup for styling/behavior.
type popupKind int

const (
	popupNone popupKind = iota
	popupGreeting
	popupMessage
	popupDisconnected
)

// Popup is a modal overlay (greeting, message, disconnect notice). It mirrors
// chillerbot terminalui m_Popup. Always closable with Enter/Esc (§V17).
type Popup struct {
	Kind  popupKind
	Title string
	Body  []string
}

// active reports whether a popup is shown.
func (p Popup) active() bool { return p.Kind != popupNone }

// greetingPopup is the startup menu (§T31): key hints + close instruction.
func greetingPopup() Popup {
	return Popup{
		Kind:  popupGreeting,
		Title: "teetui",
		Body: []string{
			"cross-platform terminal Teeworlds/DDNet client",
			"",
			"?   help          B   server browser",
			"t   chat          F1  local console",
			"y   team chat     F2  remote console (rcon)",
			"v   visual        k   self-kill",
			"",
			"press B for browser, ? for help, Enter to close",
		},
	}
}

// disconnectPopup notifies the user a session ended (§T19/§T25).
func disconnectPopup(reason string) Popup {
	if reason == "" {
		reason = "connection closed"
	}
	return Popup{
		Kind:  popupDisconnected,
		Title: "Disconnected",
		Body:  []string{reason, "", "press Enter to close"},
	}
}

// drawPopup renders the popup centered on the screen.
func drawPopup(s tcell.Screen, w, h int, p Popup) {
	lines := append([]string{p.Title, ""}, p.Body...)
	boxW := 0
	for _, l := range lines {
		if n := len(l); n > boxW {
			boxW = n
		}
	}
	boxW += 4
	boxH := len(lines) + 2
	// Clamp the box to the screen so a small terminal never draws outside its
	// bounds (§V30); drawStr already clips each line horizontally.
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
	if p.Kind == popupDisconnected {
		style = tcell.StyleDefault.Foreground(tcell.NewRGBColor(255, 80, 80)).Reverse(true)
	}
	for row := 0; row < boxH; row++ {
		for col := 0; col < boxW; col++ {
			s.SetContent(x0+col, y0+row, ' ', nil, style)
		}
	}
	for i, l := range lines {
		drawStr(s, x0+2, y0+1+i, boxW-3, style, l)
	}
}
