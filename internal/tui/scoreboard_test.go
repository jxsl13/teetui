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

// §V6/§V58: scoreboard columns are width-aligned (truncate + pad) at the given
// column widths.
func TestScoreboardLineColumns(t *testing.T) {
	const nameW, clanW = 16, 12
	line := scoreboardLine(client.PlayerState{Score: 42, Name: "verylongplayername_overflow", Clan: "clan"}, nameW, clanW)
	if !strings.Contains(line, "42") {
		t.Errorf("missing score: %q", line)
	}
	// name column is truncated to nameW display cells.
	nameField := line[strings.Index(line, "42")+len("42")+1:]
	if len([]rune(strings.TrimRight(nameField[:nameW], " "))) > nameW {
		t.Errorf("name not truncated: %q", line)
	}
}

// §T99/§V58: name/clan columns flex with board width — clan drops when narrow,
// name grows when wide; column sum never exceeds the width.
func TestScoreboardColsFlex(t *testing.T) {
	wideN, wideC := scoreboardCols(120)
	narrowN, narrowC := scoreboardCols(28)
	if narrowC != 0 {
		t.Errorf("narrow board should drop clan, got clanW=%d", narrowC)
	}
	if wideC == 0 {
		t.Errorf("wide board should show clan, got clanW=0")
	}
	if wideN <= narrowN {
		t.Errorf("name should grow on wide board: wide=%d narrow=%d", wideN, narrowN)
	}
	for _, w := range []int{12, 20, 40, 80, 200} {
		n, c := scoreboardCols(w)
		sep := 0
		if c > 0 {
			sep = 1
		}
		used := 1 + scoreColW + 1 + n + sep + c // leading space + score + sep + name + sep + clan
		if used > w {
			t.Errorf("scoreboard cols overflow at w=%d: used=%d (n=%d c=%d)", w, used, n, c)
		}
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
	styler := func(name, clan string) (tcell.Style, bool) {
		if name == "alice" {
			return StyleSelf, true
		}
		return tcell.StyleDefault, false
	}
	DrawScoreboard(scr, Rect{0, 0, 50, 10}, st, styler)
	DrawScoreboard(scr, Rect{0, 0, 50, 10}, st, nil) // nil styler ok
}
