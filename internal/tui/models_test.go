package tui

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// §T38/§V6: readline editing operates on rune positions.
func TestTextInputEditing(t *testing.T) {
	var in TextInput
	for _, r := range "hello world" {
		in.Insert(r)
	}
	if in.String() != "hello world" || in.Cursor() != 11 {
		t.Fatalf("insert: %q cur=%d", in.String(), in.Cursor())
	}
	in.KillWord() // removes "world"
	if in.String() != "hello " {
		t.Errorf("killword: %q", in.String())
	}
	in.Home()
	in.Right()
	in.Backspace() // delete 'h'
	if in.String() != "ello " {
		t.Errorf("backspace at cursor: %q", in.String())
	}
	in.End()
	in.KillToStart()
	if in.String() != "" || in.Cursor() != 0 {
		t.Errorf("killtostart: %q cur=%d", in.String(), in.Cursor())
	}
}

// §V16: history persists across a save/load cycle and navigates + searches.
func TestHistoryPersistAndNav(t *testing.T) {
	h := NewHistory(8)
	h.Add("one")
	h.Add("two")
	h.Add("three")

	if s, _ := h.Prev(); s != "three" {
		t.Errorf("prev1 = %q", s)
	}
	if s, _ := h.Prev(); s != "two" {
		t.Errorf("prev2 = %q", s)
	}
	if s, _ := h.Next(); s != "three" {
		t.Errorf("next = %q", s)
	}
	if s, ok := h.Search("on"); !ok || s != "one" {
		t.Errorf("search = %q ok=%v", s, ok)
	}

	path := filepath.Join(t.TempDir(), "hist", "chat.txt")
	if err := h.Save(path); err != nil {
		t.Fatal(err)
	}
	h2 := NewHistory(8)
	if err := h2.Load(path); err != nil {
		t.Fatal(err)
	}
	if got := h2.Entries(); len(got) != 3 || got[0] != "one" || got[2] != "three" {
		t.Errorf("reloaded = %v", got)
	}
}

// §V10: dedup of immediate repeats and max bound.
func TestHistoryDedupBound(t *testing.T) {
	h := NewHistory(2)
	h.Add("a")
	h.Add("a") // dup ignored
	h.Add("b")
	h.Add("c") // evicts "a"
	got := h.Entries()
	if len(got) != 2 || got[0] != "b" || got[1] != "c" {
		t.Errorf("entries = %v", got)
	}
}

// §T30: scroll offset reveals older lines and clamps at both ends.
func TestLogScroll(t *testing.T) {
	l := NewLog(100)
	for i := 0; i < 10; i++ {
		l.Addf(tcell.StyleDefault, "L%d", i)
	}
	if v := l.View(80, 3); len(v) != 3 || v[2].Text != "L9" {
		t.Fatalf("tail view = %v", v)
	}
	l.ScrollUp(2)
	if v := l.View(80, 3); v[2].Text != "L7" {
		t.Errorf("scrolled view bottom = %q want L7", v[2].Text)
	}
	l.ScrollUp(1000) // clamp
	if v := l.View(80, 3); v[0].Text != "L0" {
		t.Errorf("top view = %q want L0", v[0].Text)
	}
	l.FollowTail()
	if v := l.View(80, 3); v[2].Text != "L9" {
		t.Errorf("follow tail = %q want L9", v[2].Text)
	}
}

// §T43/§T47: race start/finish/checkpoint overlay precedence (finish>start>cp).
func TestSpecialGlyph(t *testing.T) {
	if _, _, ok := specialGlyph(false, false, false); ok {
		t.Error("no special tile must not draw")
	}
	if g, _, ok := specialGlyph(true, false, false); !ok || g != 'S' {
		t.Errorf("start glyph = %q", g)
	}
	if g, _, ok := specialGlyph(true, true, false); !ok || g != 'F' {
		t.Errorf("finish must win precedence, got %q", g)
	}
	if g, _, ok := specialGlyph(false, false, true); !ok || g != 'C' {
		t.Errorf("checkpoint glyph = %q", g)
	}
	if g, _, ok := specialGlyph(true, false, true); !ok || g != 'S' {
		t.Errorf("start must win over checkpoint, got %q", g)
	}
}

// §T34: HUD coordinate readout format.
func TestHUDText(t *testing.T) {
	if got := hudText(12, -3); got != " x:12 y:-3 " {
		t.Errorf("hud = %q", got)
	}
}
