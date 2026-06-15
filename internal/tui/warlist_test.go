package tui

import (
	"path/filepath"
	"testing"
)

// §V14: warlist set/get/del + persistence round-trip.
func TestWarlistPersist(t *testing.T) {
	w := NewWarlist()
	w.Set("enemy", RelWar)
	w.Set("buddy", RelTeam)
	w.Set("gone", RelPeace)
	w.Del("gone")

	if w.Get("enemy") != RelWar || w.Get("buddy") != RelTeam {
		t.Fatal("set/get wrong")
	}
	if w.Get("gone") != RelNeutral {
		t.Error("del did not clear")
	}
	if _, ok := w.Style("enemy"); !ok {
		t.Error("war should have a style")
	}
	if _, ok := w.Style("nobody"); ok {
		t.Error("neutral has no style")
	}

	path := filepath.Join(t.TempDir(), "warlist.txt")
	if err := w.Save(path); err != nil {
		t.Fatal(err)
	}
	w2 := NewWarlist()
	if err := w2.Load(path); err != nil {
		t.Fatal(err)
	}
	if w2.Get("enemy") != RelWar || w2.Get("buddy") != RelTeam || w2.Get("gone") != RelNeutral {
		t.Errorf("reload mismatch: enemy=%d buddy=%d gone=%d", w2.Get("enemy"), w2.Get("buddy"), w2.Get("gone"))
	}
}

// §T22/§V14: chat commands mutate the same warlist store; non-commands pass.
func TestParseChatCommand(t *testing.T) {
	w := NewWarlist()

	if r := parseChatCommand("hello world", w); r.Handled {
		t.Error("plain chat must not be handled")
	}
	if r := parseChatCommand("!war Nameless", w); !r.Handled || w.Get("Nameless") != RelWar {
		t.Errorf("!war failed: handled=%v rel=%d", r.Handled, w.Get("Nameless"))
	}
	if r := parseChatCommand("!team Buddy", w); !r.Handled || w.Get("Buddy") != RelTeam {
		t.Error("!team failed")
	}
	if r := parseChatCommand("!del Nameless", w); !r.Handled || w.Get("Nameless") != RelNeutral {
		t.Error("!del failed")
	}
	if r := parseChatCommand("!help", w); !r.Handled || len(r.Reply) == 0 {
		t.Error("!help failed")
	}
	if r := parseChatCommand("!war", w); !r.Handled || r.Reply[0] != "usage: !war <name>" {
		t.Errorf("!war no-arg = %v", r.Reply)
	}
	if r := parseChatCommand("!bogus x", w); r.Handled {
		t.Error("unknown ! command should pass through as chat")
	}
}

// §T23/§T40: auto-reply matches known phrases; ping detection is name-based.
func TestAutoReply(t *testing.T) {
	if r, ok := autoReply("hello there nameless"); !ok || r != "hello" {
		t.Errorf("hello reply = %q ok=%v", r, ok)
	}
	if r, ok := autoReply("how are you?"); !ok || r != "good, you?" {
		t.Errorf("how-are-you reply = %q", r)
	}
	if _, ok := autoReply("totally unrelated"); ok {
		t.Error("no match should be false")
	}
	if !containsName("hey Nameless wanna trade", "nameless") {
		t.Error("ping detection case-insensitive failed")
	}
	if containsName("no mention here", "nameless") {
		t.Error("false ping")
	}
	if containsName("anything", "") {
		t.Error("empty name must never ping")
	}
}
