package tui

import (
	"strings"
	"testing"
)

// §T67: extended warlist chat commands — search, create, addreason, unfriend.
func TestChatCommandExtended(t *testing.T) {
	w := NewWarlist()

	// !create maps keywords to relations (traitor → war).
	if r := parseChatCommand("!create war griefer", w); !r.Handled || w.Get("griefer") != RelWar {
		t.Errorf("!create war: %+v rel=%v", r, w.Get("griefer"))
	}
	if r := parseChatCommand("!create traitor judas", w); !r.Handled || w.Get("judas") != RelWar {
		t.Errorf("!create traitor should be war: rel=%v", w.Get("judas"))
	}
	if r := parseChatCommand("!create team mate", w); !r.Handled || w.Get("mate") != RelTeam {
		t.Errorf("!create team: rel=%v", w.Get("mate"))
	}
	if r := parseChatCommand("!create neutral griefer", w); !r.Handled || w.Get("griefer") != RelNeutral {
		t.Errorf("!create neutral should clear: rel=%v", w.Get("griefer"))
	}
	if r := parseChatCommand("!create bogus x", w); !r.Handled || !strings.Contains(r.Reply[0], "unknown type") {
		t.Errorf("!create bogus: %+v", r)
	}

	// !addreason is an alias of !reason.
	if r := parseChatCommand("!addreason judas backstab", w); !r.Handled || w.Reason("judas") != "backstab" {
		t.Errorf("!addreason: reason=%q", w.Reason("judas"))
	}

	// !search finds matching names with relation + reason.
	w.Set("griefer2", RelWar)
	r := parseChatCommand("!search grief", w)
	if !r.Handled || len(r.Reply) == 0 || !strings.Contains(r.Reply[0], "griefer2") {
		t.Errorf("!search: %+v", r)
	}
	if r := parseChatCommand("!search zzz", w); !r.Handled || !strings.Contains(r.Reply[0], "no warlist matches") {
		t.Errorf("!search miss: %+v", r)
	}

	// !unfriend clears a relation.
	if r := parseChatCommand("!unfriend mate", w); !r.Handled || w.Get("mate") != RelNeutral {
		t.Errorf("!unfriend: rel=%v", w.Get("mate"))
	}
}
