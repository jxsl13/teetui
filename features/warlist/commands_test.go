package warlist

import (
	"strings"
	"testing"
)

// §T22/§T24/§T67: warlist chat commands parse + apply.
func TestParseCommand(t *testing.T) {
	w := newStore()

	if r := parseCommand("hello world", w); r.Handled {
		t.Error("ordinary chat must not be handled")
	}
	if r := parseCommand("!war Nameless", w); !r.Handled || w.Get("Nameless") != RelWar {
		t.Errorf("!war failed: %+v", r)
	}
	if r := parseCommand("!team Buddy", w); !r.Handled || w.Get("Buddy") != RelTeam {
		t.Error("!team failed")
	}
	if r := parseCommand("!peace Buddy", w); !r.Handled || w.Get("Buddy") != RelPeace {
		t.Error("!peace failed")
	}
	if r := parseCommand("!del Nameless", w); !r.Handled || w.Get("Nameless") != RelNeutral {
		t.Error("!del failed")
	}
	if r := parseCommand("!help", w); !r.Handled || len(r.Reply) == 0 {
		t.Error("!help failed")
	}
	if r := parseCommand("!war", w); !r.Handled || r.Reply[0] != "usage: !war <name>" {
		t.Errorf("!war usage = %+v", r)
	}
	if r := parseCommand("!bogus x", w); r.Handled {
		t.Error("unknown ! command should be unhandled")
	}

	// clan + reason
	if r := parseCommand("!warclan BAD", w); !r.Handled || w.effective("x", "BAD") != RelWar {
		t.Error("!warclan failed")
	}
	if r := parseCommand("!reason rage spawn kill", w); !r.Handled || w.Reason("rage") != "spawn kill" {
		t.Error("!reason failed")
	}

	// extended: search / create / addreason / unfriend
	w.Set("griefer", RelWar)
	if r := parseCommand("!search grief", w); !r.Handled || !strings.Contains(r.Reply[0], "griefer") {
		t.Errorf("!search: %+v", r)
	}
	if r := parseCommand("!create traitor judas", w); !r.Handled || w.Get("judas") != RelWar {
		t.Error("!create traitor should be war")
	}
	if r := parseCommand("!addreason judas backstab", w); !r.Handled || w.Reason("judas") != "backstab" {
		t.Error("!addreason failed")
	}
	if r := parseCommand("!unfriend judas", w); !r.Handled || w.Get("judas") != RelNeutral {
		t.Error("!unfriend failed")
	}
	if r := parseCommand("!create bogus x", w); !r.Handled || !strings.Contains(r.Reply[0], "unknown type") {
		t.Errorf("!create bogus: %+v", r)
	}
}
