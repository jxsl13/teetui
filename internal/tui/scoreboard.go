package tui

import (
	"fmt"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/twclient/client"
)

const (
	scoreColW = 7
	nameColW  = 16
	clanColW  = 12
)

// rosterRows returns the roster sorted for display: highest score first, ties
// broken by client id for stable ordering (§T17). Pure, so it is unit-tested.
func rosterRows(roster map[int]client.PlayerState) []client.PlayerState {
	rows := make([]client.PlayerState, 0, len(roster))
	for _, p := range roster {
		rows = append(rows, p)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Score != rows[j].Score {
			return rows[i].Score > rows[j].Score
		}
		return rows[i].ClientID < rows[j].ClientID
	})
	return rows
}

// scoreboardLine formats one roster row into aligned columns.
func scoreboardLine(p client.PlayerState) string {
	return fmt.Sprintf("%*d %s %s", scoreColW, p.Score, padCol(p.Name, nameColW), padCol(p.Clan, clanColW))
}

// DrawScoreboard overlays the player table on the game region, sorted by score,
// with name and clan from the in-session roster (twclient §I.PlayerState). The
// local player is highlighted; others are tinted by their warlist relation
// (§T21/§V14). A nil warlist disables tinting.
func DrawScoreboard(s tcell.Screen, r Rect, st client.TickState, w *Warlist) {
	if r.W < 12 || r.H < 2 {
		return
	}
	header := fmt.Sprintf("%*s %s %s", scoreColW, "score", padCol("name", nameColW), padCol("clan", clanColW))
	drawStr(s, r.X, r.Y, r.W, StyleStatus, padCol(" "+header, r.W))

	row := 1
	for _, p := range rosterRows(st.Roster) {
		if row >= r.H {
			break
		}
		style := scoreRowStyle(p, st.LocalID, w)
		drawStr(s, r.X, r.Y+row, r.W, style, " "+scoreboardLine(p))
		row++
	}
}

// scoreRowStyle picks the row color: local > warlist relation > present/absent.
func scoreRowStyle(p client.PlayerState, localID int, w *Warlist) tcell.Style {
	if p.Local || p.ClientID == localID {
		return StyleSelf
	}
	if w != nil {
		// Clan war colors players too: a per-name relation wins, else the relation
		// of the player's clan tag applies (§T24/§V14).
		if st, ok := w.EffectiveStyle(p.Name, p.Clan); ok {
			return st
		}
	}
	if !p.Present {
		return StyleSystem
	}
	return StyleChat
}
