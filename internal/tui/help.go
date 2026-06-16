package tui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// helpLines builds the '?' overlay key cheatsheet from the LIVE keymap + the
// registered feature actions, so it lists EVERY binding and tracks rebinds
// rather than drifting from a hardcoded slice (§T96/§V56/§V19). Grouped for
// readability; always escapable with '?' or Esc (§V17).
func (a *App) helpLines() []string {
	km := a.keymap
	kb := func(act KeyAction) string {
		toks := km.tokensFor(act)
		if len(toks) == 0 {
			return "(unbound)"
		}
		return strings.Join(toks, "/")
	}

	type row struct{ key, label string }
	groups := []struct {
		title string
		rows  []row
	}{
		{"general", []row{
			{kb(actHelp), "toggle this help"},
			{kb(actQuit), "quit"},
			{kb(actBrowser), "server browser"},
			{kb(actChat), "chat"},
			{kb(actTeamChat), "team chat"},
			{kb(actLocalConsole) + " / " + kb(actRemoteConsole), "local / remote console (rcon)"},
			{kb(actScoreboard), "scoreboard"},
			{kb(actReconnect), "reconnect"},
			{kb(actVoteYes) + " / " + kb(actVoteNo), "vote yes / no"},
		}},
		{"map / view", []row{
			{kb(actVisual), "toggle game view"},
			{kb(actSubcellToggle), "toggle sub-cell detail render"},
			{kb(actFreeLook), "free-look map-pan (WASD/arrows pan, Esc exit)"},
		}},
		{"movement", []row{
			{"WASD / arrows", "move (jump/left/stop/right); the other set aims — see cl_move_keys"},
			{kb(actJump), "jump"},
			{kb(actHook), "hook"},
			{kb(actFire), "fire"},
			{"1-6", "select weapon"},
			{kb(actKill), "self-kill"},
			{kb(actEmote), "emote"},
		}},
		{"input line", []row{
			{"Ctrl-U/K/W", "kill line / to-end / word"},
			{"↑ / ↓", "history prev / next"},
			{"Ctrl-R", "reverse-i-search"},
			{"Tab", "complete"},
			{"PgUp/PgDn", "scroll log"},
		}},
	}
	if len(a.featActions) > 0 { // feature-defined actions (§V46/§V56)
		var rows []row
		for _, fa := range a.featActions {
			label := fa.name
			if fa.help != "" {
				label = fa.help
			}
			rows = append(rows, row{fa.key, label})
		}
		groups = append(groups, struct {
			title string
			rows  []row
		}{"features", rows})
	}

	// Align the key column across all rows for a clean cheatsheet.
	keyW := 0
	for _, g := range groups {
		for _, r := range g.rows {
			if w := runewidth.StringWidth(r.key); w > keyW {
				keyW = w
			}
		}
	}

	lines := []string{" teetui — keys "}
	for _, g := range groups {
		lines = append(lines, "", " "+g.title+":")
		for _, r := range g.rows {
			lines = append(lines, "  "+runewidth.FillRight(r.key, keyW)+"  "+r.label)
		}
	}
	// Modes — explain to a newcomer how to ENTER each mode and WHAT it is, plus how
	// to leave it (§C32/§V70). Keys come from the live keymap.
	lines = append(lines,
		"",
		" modes — press the key to enter; Esc leaves a text mode:",
		"  "+kb(actChat)+"  chat — type a message to everyone, Enter sends",
		"  "+kb(actTeamChat)+"  team chat — message only your team",
		"  "+kb(actLocalConsole)+"  local console — a command line for client options (cvars) and",
		"      commands: connect <addr>, say <msg>, help; Tab completes, Esc leaves",
		"  "+kb(actRemoteConsole)+"  remote console (rcon) — server admin; type the rcon password first",
		"  "+kb(actBrowser)+"  server browser — pick a server from the list, Enter joins, Esc closes",
		"  "+kb(actScoreboard)+"  scoreboard — show the player table (press again to hide)",
		"  "+kb(actVisual)+"  visual — toggle the live game view on/off",
		"  "+kb(actFreeLook)+"  free-look — detach the camera and pan the map (WASD/arrows; Esc exits)",
		"",
		" chat commands:    !war / !peace / !team / !del <name>",
		" console commands: connect <addr>, say <msg>, spec [name], help",
		"",
		" keys rebindable: ~/.config/teetui/keymap.txt",
		" press ? or Esc to close ")
	return lines
}

// drawHelp renders the help box (lines) centered on the screen, clamped so a
// small terminal never draws past its bounds (§V30).
func drawHelp(s tcell.Screen, w, h int, lines []string) {
	boxW := 0
	for _, l := range lines {
		if lw := runewidth.StringWidth(l); lw > boxW {
			boxW = lw
		}
	}
	boxW += 2
	boxH := len(lines) + 2
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
	for i, l := range lines {
		if i >= boxH-2 { // never draw past the clamped box (§V30)
			break
		}
		drawStr(s, x0+1, y0+1+i, boxW-1, style, l)
	}
}
