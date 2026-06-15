package team

import (
	"testing"

	"github.com/jxsl13/teetui/feature"
	"github.com/jxsl13/twclient/client"
)

// capHost is a feature.Host that captures registered commands and Do actions.
type capHost struct {
	feature.NopHost
	cmds map[string]func(string) []string
	last client.Action
}

func newCapHost() *capHost { return &capHost{cmds: map[string]func(string) []string{}} }

func (h *capHost) Do(a client.Action) error { h.last = a; return nil }
func (h *capHost) DefineCommand(name, help string, run func(string) []string) {
	h.cmds[name] = run
}

// §T92/§V52: the team feature registers `team`/`join` and issues
// client.ActSetTeam with the correct id.
func TestTeamFeatureCommands(t *testing.T) {
	h := newCapHost()
	if err := (teamFeature{}).Provision(h); err != nil {
		t.Fatal(err)
	}
	if h.cmds["team"] == nil || h.cmds["join"] == nil {
		t.Fatal("team/join commands not registered")
	}

	h.cmds["team"]("blue")
	if act, ok := h.last.(client.ActSetTeam); !ok || act.Team != teamBlue {
		t.Errorf("team blue → %#v want ActSetTeam{1}", h.last)
	}
	h.cmds["join"]("")
	if act, ok := h.last.(client.ActSetTeam); !ok || act.Team != teamRedGame {
		t.Errorf("join → %#v want ActSetTeam{0}", h.last)
	}
	// Unknown arg → usage, no Do.
	h.last = nil
	out := h.cmds["team"]("purple")
	if len(out) == 0 || h.last != nil {
		t.Errorf("bad team arg should print usage and not Do: out=%v last=%v", out, h.last)
	}
}
