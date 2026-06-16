package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/twclient/client"
)

// escMenu is the DDNet-style overlay action bar shown at the top of the viewport
// while connected (§C35/§V74). It is KEYBOARD-navigable (teetui has no mouse,
// C31): ←/→ or Tab/Shift-Tab move focus, Enter activates, Esc closes. Its buttons
// are rebuilt from the live game context (team vs solo, in-game vs spectating)
// each time it opens.
type escMenu struct {
	open  bool
	focus int
	items []menuItem
}

type menuItem struct {
	label string
	run   func()
}

// openEscMenu builds the context buttons and shows the bar (§V74).
func (a *App) openEscMenu() {
	a.escMenu.items = a.buildEscMenuItems()
	a.escMenu.focus = 0
	a.escMenu.open = true
}

func (a *App) closeEscMenu() { a.escMenu.open = false }

// buildEscMenuItems assembles the action buttons for the current context (§V74/
// §V75): team vs solo Join buttons, Kill/Pause while in-game, and Disconnect.
// (Connect-dummy and the follow list are added in §T114/§T115.)
func (a *App) buildEscMenuItems() []menuItem {
	st, have := a.cur().state.Get()
	var items []menuItem
	if teamMode(st) {
		items = append(items,
			menuItem{"Join red", func() { a.do(client.ActSetTeam{Team: 0}) }},
			menuItem{"Join blue", func() { a.do(client.ActSetTeam{Team: 1}) }},
		)
	} else {
		items = append(items,
			menuItem{"Join game", func() { a.do(client.ActSetTeam{Team: 0}) }},
			menuItem{"Spectate", func() { a.do(client.ActSetTeam{Team: -1}) }},
		)
	}
	if have && hasLocalChar(st) { // in-game (has a character, not spectating)
		items = append(items,
			menuItem{"Kill", func() { a.do(client.ActKill{}) }},
			menuItem{"Pause", func() { a.sendChat("/pause", false) }},
		)
	}
	// Connect dummy (§T115) — only when the server allows dummies (per-IP limit is
	// enforced server-side, V76).
	if c := a.cur().cli.Load(); c != nil && c.Capabilities().AllowDummy {
		items = append(items, menuItem{"Connect dummy", a.connectDummy})
	}
	items = append(items, menuItem{"Disconnect", a.disconnectUser})
	// Follow list (§T114): with dummies connected, list every own client; the
	// active one is marked. Selecting follows it = render from its perspective.
	if sessions, active := a.sessionList(); len(sessions) > 1 {
		for i, s := range sessions {
			i := i
			label := "Follow " + s.name
			if i == active {
				label = "● " + s.name
			}
			items = append(items, menuItem{label, func() { a.setActive(i) }})
		}
	}
	return items
}

// hasLocalChar reports whether the local player currently has a character in the
// world (i.e. is in-game rather than spectating).
func hasLocalChar(st client.TickState) bool {
	_, ok := st.Players[st.LocalID]
	return ok
}

// handleEscMenu drives the open menu: focus navigation + activate + close. It
// consumes every key while open so the menu fully captures input (§V74).
func (a *App) handleEscMenu(ev *tcell.EventKey) {
	n := len(a.escMenu.items)
	switch ev.Key() {
	case tcell.KeyEscape:
		a.closeEscMenu()
	case tcell.KeyEnter:
		if n > 0 && a.escMenu.focus < n {
			run := a.escMenu.items[a.escMenu.focus].run
			a.closeEscMenu()
			if run != nil {
				run()
			}
		}
	case tcell.KeyLeft:
		a.moveEscFocus(-1)
	case tcell.KeyRight, tcell.KeyTab:
		a.moveEscFocus(1)
	case tcell.KeyBacktab:
		a.moveEscFocus(-1)
	}
}

func (a *App) moveEscFocus(d int) {
	n := len(a.escMenu.items)
	if n == 0 {
		return
	}
	a.escMenu.focus = (a.escMenu.focus + d + n) % n
}

// drawEscMenu renders the action bar across the top two rows (§V74), the focused
// button highlighted, with a one-line key hint. Keyboard-only.
func (a *App) drawEscMenu(w int) {
	if !a.escMenu.open {
		return
	}
	for x := 0; x < w; x++ { // bar background
		a.scr.SetContent(x, 0, ' ', nil, StyleStatus)
	}
	x := 1
	for i, it := range a.escMenu.items {
		st := StyleStatus
		label := " " + it.label + " "
		if i == a.escMenu.focus {
			st = StyleSelf // highlight the focused button
		}
		x += drawStr(a.scr, x, 0, w-x, st, "["+label+"]") + 1
	}
	drawStr(a.scr, 1, 1, w-1, StyleSystem, " ←/→ or Tab select · Enter activate · Esc close ")
}
