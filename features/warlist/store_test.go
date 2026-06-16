package warlist

import (
	"path/filepath"
	"testing"
)

// §T78/§V14: relations + reasons + clan relations persist across Load/Save, with
// name-then-clan precedence in effective().
func TestStorePersist(t *testing.T) {
	w := newStore()
	w.Set("enemy", RelWar)
	w.Set("buddy", RelTeam)
	w.Set("gone", RelPeace)
	w.Del("gone")
	w.SetReason("enemy", "spawn killed me")
	w.SetClan("BAD", RelWar)

	if w.Get("enemy") != RelWar || w.Get("buddy") != RelTeam {
		t.Fatal("set/get relation wrong")
	}
	if w.Get("gone") != RelNeutral {
		t.Error("del did not clear")
	}
	if w.Reason("enemy") != "spawn killed me" {
		t.Error("reason not stored")
	}
	w.Set("friend", RelTeam)
	if w.effective("friend", "BAD") != RelTeam {
		t.Error("name should win over clan")
	}
	if w.effective("anyone", "BAD") != RelWar {
		t.Error("clan relation should apply when no per-name")
	}
	if _, ok := w.EffectiveStyle("anyone", "BAD"); !ok {
		t.Error("clan-war player should be styled")
	}
	if _, ok := w.EffectiveStyle("stranger", "GOOD"); ok {
		t.Error("neutral player should not be styled")
	}

	path := filepath.Join(t.TempDir(), "warlist.txt")
	if err := w.Save(path); err != nil {
		t.Fatal(err)
	}
	w2 := newStore()
	if err := w2.Load(path); err != nil {
		t.Fatal(err)
	}
	if w2.Get("enemy") != RelWar || w2.Reason("enemy") != "spawn killed me" {
		t.Error("name+reason lost on reload")
	}
	if w2.Get("friend") != RelTeam || w2.effective("x", "BAD") != RelWar {
		t.Error("clan/team lost on reload")
	}
}

// §T78/§T62: the "warlist" string service maps relations to tokens (§V53).
func TestStoreService(t *testing.T) {
	w := newStore()
	w.Set("e", RelWar)
	w.SetReason("e", "rdm")
	w.Set("p", RelPeace)
	w.SetClan("C", RelWar)

	if w.Relation("e") != "war" || w.Relation("p") != "peace" || w.Relation("none") != "" {
		t.Error("Relation() mapping wrong")
	}
	if w.Reason("e") != "rdm" {
		t.Error("Reason() wrong")
	}
	if got := w.NamesWith("war"); len(got) != 1 || got[0] != "e" {
		t.Errorf("NamesWith(war) = %v", got)
	}
	if got := w.ClansWith("war"); len(got) != 1 || got[0] != "C" {
		t.Errorf("ClansWith(war) = %v", got)
	}
}
