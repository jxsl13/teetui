package team

import (
	"testing"

	"github.com/jxsl13/teetui/feature"
	"github.com/jxsl13/twclient/client"
)

// capHost is a feature.Host that captures registered commands and Do actions.
type capHost struct {
	cmds map[string]func(string) []string
	last client.Action
}

func newCapHost() *capHost { return &capHost{cmds: map[string]func(string) []string{}} }

func (h *capHost) SendChat(string, bool)                        {}
func (h *capHost) Do(a client.Action) error                     { h.last = a; return nil }
func (h *capHost) Log(string)                                   {}
func (h *capHost) Roster() []client.PlayerState                 { return nil }
func (h *capHost) Tick() (client.TickState, bool)               { return client.TickState{}, false }
func (h *capHost) PlayerName() string                           { return "me" }
func (h *capHost) PlayerClan() string                           { return "" }
func (h *capHost) Server() string                               { return "" }
func (h *capHost) DefineConfig(string, string, string)          {}
func (h *capHost) Config(string) (string, bool)                 { return "", false }
func (h *capHost) OnSendChat(func(string, bool) (string, bool)) {}
func (h *capHost) DefineAction(string, string, string, func())  {}
func (h *capHost) DefineCommand(name, help string, run func(string) []string) {
	h.cmds[name] = run
}
func (h *capHost) AddStatusField(func() string)                            {}
func (h *capHost) AddNameStyle(func(string, string) (feature.Style, bool)) {}
func (h *capHost) Provide(string, any)                                     {}
func (h *capHost) Lookup(string) (any, bool)                               { return nil, false }

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
