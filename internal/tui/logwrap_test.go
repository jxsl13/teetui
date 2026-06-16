package tui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// §T98/§V59: wrapLine breaks long lines at the width, word-wrapping on spaces
// and hard-splitting overlong tokens, never dropping content (§V6 width-aware).

func TestWrapLineShort(t *testing.T) {
	got := wrapLine("hello world", 40)
	if len(got) != 1 || got[0] != "hello world" {
		t.Errorf("short line should pass through: %q", got)
	}
}

func TestWrapLineWords(t *testing.T) {
	got := wrapLine("the quick brown fox jumps", 10)
	for _, seg := range got {
		if runewidth.StringWidth(seg) > 10 {
			t.Errorf("segment %q exceeds width 10", seg)
		}
	}
	// Reassembling (ignoring wrap spaces) must preserve every word.
	if strings.ReplaceAll(join(got), " ", "") != "thequickbrownfoxjumps" {
		t.Errorf("words lost on wrap: %q", got)
	}
}

func TestWrapLineOverlongToken(t *testing.T) {
	got := wrapLine("supercalifragilistic", 5)
	if len(got) < 4 {
		t.Fatalf("overlong token not hard-split: %q", got)
	}
	for _, seg := range got {
		if runewidth.StringWidth(seg) > 5 {
			t.Errorf("hard-split segment %q exceeds 5", seg)
		}
	}
	if join(got) != "supercalifragilistic" {
		t.Errorf("content lost hard-splitting: %q", got)
	}
}

func TestWrapLineWideRune(t *testing.T) {
	// '世' is width 2; width 3 fits one per row.
	got := wrapLine("世界世界", 3)
	for _, seg := range got {
		if runewidth.StringWidth(seg) > 3 {
			t.Errorf("wide-rune segment %q exceeds width 3", seg)
		}
	}
}

// §V59: View returns wrapped VISUAL rows; a long line occupies multiple rows.
func TestLogViewWraps(t *testing.T) {
	l := NewLog(10)
	l.Addf(tcell.StyleDefault, "aaaa bbbb cccc dddd") // 19 cols
	v := l.View(9, 5)                                 // width 9 → wraps to >1 row
	if len(v) < 2 {
		t.Fatalf("long line should wrap to multiple rows, got %d: %v", len(v), v)
	}
	for _, ln := range v {
		if runewidth.StringWidth(ln.Text) > 9 {
			t.Errorf("visual row %q exceeds width 9", ln.Text)
		}
	}
}

func join(ss []string) string {
	out := ""
	for _, s := range ss {
		out += s
	}
	return out
}
