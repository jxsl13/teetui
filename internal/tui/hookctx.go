package tui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/teetui/extension"
	"github.com/jxsl13/twclient/client"
)

// appHookCtx adapts *App to extension.HookCtx — the safe action surface user
// hooks may use (§T70/§V39). It exposes only teetui's existing twclient-backed
// capabilities; there is no raw network access.
type appHookCtx struct{ a *App }

// hookCtx returns the action context passed to user hooks.
func (a *App) hookCtx() extension.HookCtx { return appHookCtx{a} }

func (c appHookCtx) SendChat(msg string, team bool) { c.a.sendChat(msg, team) }

func (c appHookCtx) Do(act client.Action) error {
	if cl := c.a.cli.Load(); cl != nil {
		return cl.Do(act)
	}
	return nil
}

func (c appHookCtx) Log(msg string) { c.a.log.Addf(StyleSystem, "%s", msg) }

func (c appHookCtx) Roster() []client.PlayerState {
	if cl := c.a.cli.Load(); cl != nil {
		return cl.Roster()
	}
	return nil
}

func (c appHookCtx) Config(name string) (string, bool) {
	cs := c.a.cfgSnap()
	if cv := findCvar(name); cv != nil {
		return cv.get(&cs), true
	}
	return "", false
}

func (c appHookCtx) Server() string { return c.a.server }

// hookKeyNames names the non-rune keys exposed to OnKey hooks.
var hookKeyNames = map[tcell.Key]string{
	tcell.KeyF1: "F1", tcell.KeyF2: "F2", tcell.KeyF3: "F3", tcell.KeyF4: "F4",
	tcell.KeyF5: "F5", tcell.KeyF6: "F6", tcell.KeyF7: "F7", tcell.KeyF8: "F8",
	tcell.KeyF9: "F9", tcell.KeyF10: "F10", tcell.KeyF11: "F11", tcell.KeyF12: "F12",
	tcell.KeyEnter: "Enter", tcell.KeyEscape: "Esc", tcell.KeyTab: "Tab",
	tcell.KeyUp: "Up", tcell.KeyDown: "Down", tcell.KeyLeft: "Left", tcell.KeyRight: "Right",
	tcell.KeyPgUp: "PgUp", tcell.KeyPgDn: "PgDn",
}

// keyToHook maps a tcell key event to the extension.Key handed to OnKey hooks.
func keyToHook(ev *tcell.EventKey) extension.Key {
	if r := ev.Rune(); r != 0 && ev.Key() == tcell.KeyRune {
		return extension.Key{Rune: r}
	}
	if name, ok := hookKeyNames[ev.Key()]; ok {
		return extension.Key{Name: name}
	}
	return extension.Key{Name: "key"}
}
