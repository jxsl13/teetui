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

// §T24/§V14: advanced warlist — war reasons + clan relations resolve and
// persist round-trip alongside the legacy per-name lines.
func TestWarlistAdvanced(t *testing.T) {
	w := NewWarlist()

	// War reason set/get.
	w.Set("rage", RelWar)
	w.SetReason("rage", "spawn killed me")
	if got := w.Reason("rage"); got != "spawn killed me" {
		t.Errorf("reason = %q", got)
	}
	// Clearing the relation drops the reason too.
	w.Del("rage")
	if w.Reason("rage") != "" {
		t.Error("del should clear reason")
	}

	// Clan relation resolves via Effective; per-name relation overrides clan.
	w.SetClan("BAD", RelWar)
	if w.ClanRel("BAD") != RelWar {
		t.Error("clan rel not stored")
	}
	if got := w.Effective("anyone", "BAD"); got != RelWar {
		t.Errorf("clan war should color player: %d", got)
	}
	w.Set("friend", RelTeam) // friend wears the BAD tag but is a personal teammate
	if got := w.Effective("friend", "BAD"); got != RelTeam {
		t.Errorf("name relation must override clan: %d", got)
	}
	if _, ok := w.EffectiveStyle("anyone", "BAD"); !ok {
		t.Error("clan war should yield a style")
	}
	if _, ok := w.EffectiveStyle("stranger", "GOOD"); ok {
		t.Error("unrelated clan must have no style")
	}

	// Round-trip reasons + clan rels through disk.
	w.Set("rage", RelWar)
	w.SetReason("rage", "spawn killed me")
	path := filepath.Join(t.TempDir(), "warlist.txt")
	if err := w.Save(path); err != nil {
		t.Fatal(err)
	}
	w2 := NewWarlist()
	if err := w2.Load(path); err != nil {
		t.Fatal(err)
	}
	if w2.Get("rage") != RelWar || w2.Reason("rage") != "spawn killed me" {
		t.Errorf("reason lost on reload: rel=%d reason=%q", w2.Get("rage"), w2.Reason("rage"))
	}
	if w2.ClanRel("BAD") != RelWar {
		t.Errorf("clan rel lost on reload: %d", w2.ClanRel("BAD"))
	}
	if w2.Get("friend") != RelTeam {
		t.Error("name relation lost on reload")
	}
}

// §T24: chat commands handle multi-name war, war reasons and clan commands.
func TestParseChatCommandAdvanced(t *testing.T) {
	w := NewWarlist()

	// Multi-name war: every name is warred, no reason stored.
	if r := parseChatCommand("!war a b c", w); !r.Handled {
		t.Fatal("multi-name !war not handled")
	}
	if w.Get("a") != RelWar || w.Get("b") != RelWar || w.Get("c") != RelWar {
		t.Error("multi-name war did not war all names")
	}
	if w.Reason("a") != "" {
		t.Error("multi-name war must not store a reason")
	}

	// War reason via the dedicated !reason verb (implies a war).
	if r := parseChatCommand("!reason villain ruined my run", w); !r.Handled {
		t.Fatal("!reason not handled")
	}
	if w.Get("villain") != RelWar || w.Reason("villain") != "ruined my run" {
		t.Errorf("reason not stored: rel=%d reason=%q", w.Get("villain"), w.Reason("villain"))
	}

	// Clan commands.
	if r := parseChatCommand("!warclan EVIL", w); !r.Handled || w.ClanRel("EVIL") != RelWar {
		t.Errorf("!warclan failed: handled=%v rel=%d", r.Handled, w.ClanRel("EVIL"))
	}
	if r := parseChatCommand("!teamclan ALLY", w); !r.Handled || w.ClanRel("ALLY") != RelTeam {
		t.Error("!teamclan failed")
	}
	if r := parseChatCommand("!delclan EVIL", w); !r.Handled || w.ClanRel("EVIL") != RelNeutral {
		t.Error("!delclan failed")
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
