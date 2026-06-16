package tui

import (
	"fmt"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/twclient/client"
)

// scoreColW is the fixed score column; name and clan columns FLEX with the rect
// width (§T99/§V58) — clan shrinks then drops on a narrow board, name grows on a
// wide one, so the table never wastes a wide terminal nor over-truncates a small.
const scoreColW = 7

// scoreboardCols returns the name/clan column widths for a board of the given
// width. clanW == 0 means the clan column is dropped (too narrow to be useful).
// Layout per row: " " + score(scoreColW) + " " + name + [" " + clan].
func scoreboardCols(width int) (nameW, clanW int) {
	budget := width - scoreColW - 2 // leading space + name/score separator
	if budget < 1 {
		return 0, 0
	}
	clanW = budget / 3
	if clanW > 16 {
		clanW = 16
	}
	if clanW < 8 { // too small to be worth a column → drop clan, give name all
		return budget, 0
	}
	nameW = budget - clanW - 1 // separator between name and clan
	if nameW < 1 {
		return budget, 0
	}
	return nameW, clanW
}

// rosterRows returns the roster sorted for display: highest score first, ties
// broken by client id for stable ordering (§T17). Pure, so it is unit-tested.
func rosterRows(roster []client.PlayerState) []client.PlayerState {
	rows := make([]client.PlayerState, len(roster))
	copy(rows, roster)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Score != rows[j].Score {
			return rows[i].Score > rows[j].Score
		}
		return rows[i].ClientID < rows[j].ClientID
	})
	return rows
}

// scoreboardLine formats one roster row into columns of the given widths; the
// clan column is omitted when clanW <= 0 (§T99/§V58).
func scoreboardLine(p client.PlayerState, nameW, clanW int) string {
	if clanW <= 0 {
		return fmt.Sprintf("%*d %s", scoreColW, p.Score, padCol(playerLabel(p), nameW))
	}
	return fmt.Sprintf("%*d %s %s", scoreColW, p.Score, padCol(playerLabel(p), nameW), padCol(p.Clan, clanW))
}

// playerLabel is the player's name, falling back to "#<id>" when the registry
// has no name yet — e.g. on 0.6 where twclient does not decode client info
// (§T56/§V26/§B5; tracked upstream in jxsl13/twclient#3). Pure, unit-tested.
func playerLabel(p client.PlayerState) string {
	if p.Name != "" {
		return p.Name
	}
	return fmt.Sprintf("#%d", p.ClientID)
}

// nameStyler returns a per-name tint contributed by the warlist feature
// (§T78/§V14); nil disables tinting.
type nameStyler func(name, clan string) (tcell.Style, bool)

// DrawScoreboard overlays the player table on the game region, sorted by score,
// with name and clan from the in-session roster (twclient §I.PlayerState). The
// local player is highlighted; others are tinted by the styler (warlist relation,
// §T21/§V14). A nil styler disables tinting.
func DrawScoreboard(s tcell.Screen, r Rect, roster []client.PlayerState, localID int, styler nameStyler) {
	if r.W < 12 || r.H < 2 {
		return
	}
	nameW, clanW := scoreboardCols(r.W)
	var header string
	if clanW <= 0 {
		header = fmt.Sprintf("%*s %s", scoreColW, "score", padCol("name", nameW))
	} else {
		header = fmt.Sprintf("%*s %s %s", scoreColW, "score", padCol("name", nameW), padCol("clan", clanW))
	}
	drawStr(s, r.X, r.Y, r.W, StyleStatus, padCol(" "+header, r.W))

	row := 1
	for _, p := range rosterRows(roster) {
		if row >= r.H {
			break
		}
		style := scoreRowStyle(p, localID, styler)
		drawStr(s, r.X, r.Y+row, r.W, style, " "+scoreboardLine(p, nameW, clanW))
		row++
	}
}

// scoreRowStyle picks the row color: local > name styler > present/absent.
func scoreRowStyle(p client.PlayerState, localID int, styler nameStyler) tcell.Style {
	if p.Local || p.ClientID == localID {
		return StyleSelf
	}
	if styler != nil {
		if st, ok := styler(p.Name, p.Clan); ok {
			return st
		}
	}
	if !p.Present {
		return StyleSystem
	}
	return StyleChat
}
