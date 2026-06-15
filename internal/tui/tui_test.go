package tui

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/twclient/client"
	"github.com/jxsl13/twclient/packet"
)

// §V5/§V6: air is not drawn; solid has a single-width glyph.
func TestTileGlyph(t *testing.T) {
	if _, _, draw := tileGlyph(client.ClassAir); draw {
		t.Error("air must not draw")
	}
	g, _, draw := tileGlyph(client.ClassSolid)
	if !draw || g != '█' {
		t.Errorf("solid glyph = %q draw=%v", g, draw)
	}
}

// §V10: log is bounded and keeps the most recent lines.
func TestLogBounded(t *testing.T) {
	l := NewLog(3)
	for i := 0; i < 10; i++ {
		l.Addf(tcell.StyleDefault, "line %d", i)
	}
	got := l.Tail(100)
	if len(got) != 3 {
		t.Fatalf("len = %d want 3", len(got))
	}
	if got[0].Text != "line 7" || got[2].Text != "line 9" {
		t.Errorf("eviction wrong: %q..%q", got[0].Text, got[2].Text)
	}
}

// §V2/§V3: observer stores the tick and reports frame cadence.
func TestObserver(t *testing.T) {
	s := NewState()
	if s.Mode() != client.TickModeFrame {
		t.Error("observer must request frame cadence")
	}
	if _, have := s.Get(); have {
		t.Error("no tick yet")
	}
	s.Observe(nil, client.TickState{Tick: 42})
	st, have := s.Get()
	if !have || st.Tick != 42 {
		t.Errorf("get = %d have=%v", st.Tick, have)
	}
}

// §V12: controller emits the held input as an ActInput each tick.
func TestControllerEmitsInput(t *testing.T) {
	c := NewInputController()
	c.SetDirection(1)
	acts := c.OnTick(nil, client.TickState{})
	if len(acts) != 1 {
		t.Fatalf("acts = %d want 1", len(acts))
	}
	ai, ok := acts[0].(client.ActInput)
	if !ok {
		t.Fatalf("action type = %T want ActInput", acts[0])
	}
	if ai.Input.Direction != packet.DirRight {
		t.Errorf("direction = %v want right", ai.Input.Direction)
	}
}

// §V6: drawStr advances by display width for wide runes.
func TestDrawStrWidth(t *testing.T) {
	scr := tcell.NewSimulationScreen("")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()
	if w := drawStr(scr, 0, 0, 80, tcell.StyleDefault, "ab"); w != 2 {
		t.Errorf("ascii advance = %d want 2", w)
	}
	if w := drawStr(scr, 0, 1, 80, tcell.StyleDefault, "漢"); w != 2 {
		t.Errorf("wide rune advance = %d want 2", w)
	}
}

// §V7/§V11: rendering an empty or populated tick must not panic.
func TestDrawGameNoPanic(t *testing.T) {
	scr := tcell.NewSimulationScreen("")
	if err := scr.Init(); err != nil {
		t.Fatal(err)
	}
	defer scr.Fini()
	scr.SetSize(80, 24)

	DrawGame(scr, 0, 0, 40, 20, client.TickState{}) // no players, no map
	st := client.TickState{
		LocalID: 1,
		Players: map[int]client.CharacterState{1: {X: 320, Y: 320}},
	}
	DrawGame(scr, 0, 0, 40, 20, st) // player present, map nil
}

func TestRaceField(t *testing.T) {
	if got := raceField(client.RaceTime{}); got != "-" {
		t.Errorf("idle race = %q", got)
	}
	got := raceField(client.RaceTime{Active: true, TickBased: 65*time.Second + 123*time.Millisecond})
	if got != "01:05.123" {
		t.Errorf("active race = %q want 01:05.123", got)
	}
}

func TestComputeLayout(t *testing.T) {
	l := Compute(120, 30)
	if l.Status.Y != 0 || l.Input.Y != 29 {
		t.Errorf("status/input rows wrong: %d %d", l.Status.Y, l.Input.Y)
	}
	if l.Log.X != l.Game.W+1 {
		t.Errorf("log x %d not right of game", l.Log.X)
	}
}
