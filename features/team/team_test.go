package team

import (
	"testing"

	"github.com/jxsl13/teetui/feature"
	"github.com/jxsl13/twclient/client"
)

// fakeHost captures the actions/logs a feature performs (embeds NopHost so only
// the exercised methods are overridden).
type fakeHost struct {
	feature.NopHost
	actions []client.Action
	logs    []string
	defined map[string]func()
}

func (h *fakeHost) Do(a client.Action) error                          { h.actions = append(h.actions, a); return nil }
func (h *fakeHost) Log(s string)                                      { h.logs = append(h.logs, s) }
func (h *fakeHost) DefineAction(name, _ string, _ string, run func()) { h.defined[name] = run }

// §T97/§V57: the join key fires ActSetTeam{0} and logs the outcome.
func TestJoinGameAction(t *testing.T) {
	h := &fakeHost{defined: map[string]func(){}}
	if err := (teamFeature{}).Provision(h); err != nil {
		t.Fatalf("provision: %v", err)
	}
	run := h.defined["join_game"]
	if run == nil {
		t.Fatal("join_game action not defined")
	}
	run()
	if len(h.actions) != 1 {
		t.Fatalf("want 1 action, got %d", len(h.actions))
	}
	st, ok := h.actions[0].(client.ActSetTeam)
	if !ok || st.Team != teamRedGame {
		t.Errorf("join action = %#v, want ActSetTeam{Team:0}", h.actions[0])
	}
	if len(h.logs) == 0 {
		t.Error("join should log its outcome")
	}
}

// §T92/§V52: team argument parsing — spectators=-1, red/game/flock/join=0,
// blue=1; numeric forms; unknown rejected.
func TestTeamID(t *testing.T) {
	cases := []struct {
		arg  string
		want int
		ok   bool
	}{
		{"spectators", teamSpectators, true},
		{"spec", teamSpectators, true},
		{"-1", teamSpectators, true},
		{"red", teamRedGame, true},
		{"game", teamRedGame, true},
		{"flock", teamRedGame, true},
		{"join", teamRedGame, true},
		{"0", teamRedGame, true},
		{"", teamRedGame, true},
		{"blue", teamBlue, true},
		{"1", teamBlue, true},
		{"BLUE", teamBlue, true},
		{"purple", 0, false},
	}
	for _, c := range cases {
		got, ok := teamID(c.arg)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("teamID(%q) = %d,%v want %d,%v", c.arg, got, ok, c.want, c.ok)
		}
	}
}
