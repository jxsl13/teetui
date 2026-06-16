package tui

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// The bottom input bar doubles as a key legend. Rather than a hardcoded string,
// the legend is GENERATED each render from the live keymap + registered feature
// actions, so it always reflects the current bindings (rebinds included, §V19)
// and the current context (§T95/§V55). It shows the most important commands as
// `[key]label`, priority-ordered, and is truncated to the available width by
// dropping the lowest-priority entries — never overflowing the row (§V30).

// legendEntry is one legend cell. When act != actNone the displayed key is
// resolved from the live keymap; otherwise the literal key string is shown
// (for non-keymap controls like weapon select or free-look panning).
type legendEntry struct {
	act   KeyAction
	key   string
	label string
}

// legendKey returns the most legible key token currently bound to act, or "" if
// the action is unbound. A single-rune token is preferred for brevity (e.g. "B"
// over "Ctrl-C"); otherwise the first sorted named token (e.g. "F1") is used.
func legendKey(km *Keymap, act KeyAction) string {
	toks := km.tokensFor(act)
	if len(toks) == 0 {
		return ""
	}
	for _, t := range toks {
		if len([]rune(t)) == 1 {
			return t
		}
	}
	return toks[0]
}

// legendItems returns the priority-ordered legend for the current context. In
// free-look the pan controls take over; otherwise the core NORMAL-mode commands
// are listed, followed by any feature-defined actions.
func (a *App) legendItems() []legendEntry {
	if a.freeLook {
		return []legendEntry{
			{key: "WASD/↑↓←→", label: "pan"},
			{key: "Esc/" + legendKey(a.keymap, actFreeLook), label: "exit"},
			{act: actVisual, label: "visual"},
			{act: actHelp, label: "help"},
			{act: actQuit, label: "quit"},
		}
	}
	items := []legendEntry{
		{act: actChat, label: "chat"},
		{act: actBrowser, label: "browser"},
		{act: actLocalConsole, label: "console"},
		{act: actVisual, label: "visual"},
		{act: actFreeLook, label: "free-look"},
		{act: actScoreboard, label: "board"},
		{act: actHelp, label: "help"},
		{act: actKill, label: "kill"},
		{act: actReconnect, label: "reconnect"},
		{act: actSubcellToggle, label: "detail"},
		{key: "1-6/" + legendKey(a.keymap, actFire), label: "weapon"},
		{act: actQuit, label: "quit"},
	}
	for _, fa := range a.featActions { // feature actions, lowest priority
		items = append(items, legendEntry{key: fa.key, label: fa.name})
	}
	return items
}

// legendLine renders the legend to fit width, dropping the lowest-priority
// trailing entries when it would overflow (§V55/§V30). Entries with no resolved
// key are skipped (e.g. an unbound action).
func (a *App) legendLine(width int) string {
	var b strings.Builder
	b.WriteByte(' ')
	for _, it := range a.legendItems() {
		key := it.key
		if it.act != actNone {
			key = legendKey(a.keymap, it.act)
		}
		if key == "" {
			continue
		}
		seg := "[" + key + "]" + it.label + " "
		if runewidth.StringWidth(b.String())+runewidth.StringWidth(seg) > width {
			break
		}
		b.WriteString(seg)
	}
	return b.String()
}
