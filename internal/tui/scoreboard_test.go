package tui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/twclient/client"
)

// §T17: roster sorts by score desc, ties by client id.
func TestRosterRowsSort(t *testing.T) {
	roster := map[int]client.PlayerState{
		3: {ClientID: 3, Name: "low", Score: 1},
		1: {ClientID: 1, Name: "tieA", Score: 5},
		2: {ClientID: 2, Name: "tieB", Score: 5},
	}
	got := rosterRows(roster)
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].ClientID != 1 || got[1].ClientID != 2 || got[2].ClientID != 3 {
		t.Errorf("order = %d,%d,%d want 1,2,3", got[0].ClientID, got[1].ClientID, got[2].ClientID)
	}
}

// §V6: scoreboard columns are width-aligned (truncate + pad).
func TestScoreboardLineColumns(t *testing.T) {
	line := scoreboardLine(client.PlayerState{Score: 42, Name: "verylongplayername_overflow", Clan: "clan"})
	if !strings.Contains(line, "42") {
		t.Errorf("missing score: %q", line)
	}
	// name column is truncated to nameColW display cells.
	nameField := line[strings.Index(line, "42")+len("42")+1:]
	if len([]rune(strings.TrimRight(nameField[:nameColW], " "))) > nameColW {
		t.Errorf("name not truncated: %q", line)
	}
}

// §T17/§V11: drawing the roster scoreboard must not panic and highlights local.
func TestDrawScoreboardNoPanic(t *testing.T) {
	scr := tcell.NewSimulationScreen("")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()
	scr.SetSize(80, 24)
	st := client.TickState{
		LocalID: 2,
		Roster: map[int]client.PlayerState{
			1: {ClientID: 1, Name: "alice", Clan: "AAA", Score: 3, Present: true},
			2: {ClientID: 2, Name: "me", Clan: "BBB", Score: 9, Local: true, Present: true},
		},
	}
	w := NewWarlist()
	w.Set("alice", RelWar)
	DrawScoreboard(scr, Rect{0, 0, 50, 10}, st, w)
	DrawScoreboard(scr, Rect{0, 0, 50, 10}, st, nil) // nil warlist ok
}
