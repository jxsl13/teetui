package tui

import "github.com/jxsl13/twclient/client"

// Teeworlds game-mode flags (snap obj_game_info m_GameFlags). Only the team flag
// matters for the Esc menu's Join buttons (§T112/§V75).
const (
	gameflagTeams = 1 << 0 // GAMEFLAG_TEAMS — team-based mode (red/blue)
	gameflagFlags = 1 << 1 // GAMEFLAG_FLAGS — capture-the-flag (unused here)
)

// teamMode reports whether the current game is team-based (red/blue), derived
// from the tick's GameFlags (§V75) — never guessed.
func teamMode(st client.TickState) bool {
	return st.GameInfo.GameFlags&gameflagTeams != 0
}

// ddraceMode reports whether the active server is DDRace-derived, recorded at
// connect from the DDNet ext capabilities flag (§T134/§V94). It gates the Esc
// menu's Pause button, which only does anything on DDRace-family servers (the
// /pause command); vanilla CTF/DM/0.7 → false → no Pause.
func (a *App) ddraceMode() bool {
	return a.cur().ddrace.Load()
}
