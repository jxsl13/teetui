package serverlog

import (
	"strings"
	"testing"

	"github.com/jxsl13/teetui/feature"
	"github.com/jxsl13/twclient/client"
)

// recHost records logged lines and serves a fixed roster for name resolution.
type recHost struct {
	feature.NopAPI
	logs     []string
	roster   []client.PlayerState
	cfg      map[string]string
	tick     client.TickState
	haveTick bool
	selfName string
}

func (h *recHost) Log(msg string)                    { h.logs = append(h.logs, msg) }
func (h *recHost) Roster() []client.PlayerState      { return h.roster }
func (h *recHost) DefineConfig(name, def, _ string)  { h.cfg[name] = def }
func (h *recHost) Config(name string) (string, bool) { v, ok := h.cfg[name]; return v, ok }
func (h *recHost) Tick() (client.TickState, bool)    { return h.tick, h.haveTick }
func (h *recHost) PlayerName() string                { return h.selfName }

func newHost() *recHost {
	h := &recHost{cfg: map[string]string{}}
	h.roster = []client.PlayerState{
		{ClientID: 1, Name: "alice"},
		{ClientID: 2, Name: "bob"},
		{ClientID: 3, Name: ""}, // no name yet → #3 fallback (§V26)
	}
	_ = serverLog{}.Init(h) // declares cl_show_game_messages=1
	return h
}

func last(h *recHost) string {
	if len(h.logs) == 0 {
		return ""
	}
	return h.logs[len(h.logs)-1]
}

// §T106/§V67: each event becomes one DDNet-style line; names resolve via roster.
func TestServerLogMessages(t *testing.T) {
	f := serverLog{}
	h := newHost()

	f.OnPlayerJoin(h, feature.PlayerJoinEvent{ClientID: 1, Name: "alice"})
	if last(h) != "'alice' entered and joined the game" {
		t.Errorf("join = %q", last(h))
	}
	f.OnPlayerLeave(h, feature.PlayerLeaveEvent{ClientID: 2})
	if last(h) != "'bob' has left the game" {
		t.Errorf("leave = %q", last(h))
	}
	f.OnPlayerLeave(h, feature.PlayerLeaveEvent{ClientID: 2, Reason: "timeout"})
	if last(h) != "'bob' has left the game (timeout)" {
		t.Errorf("leave+reason = %q", last(h))
	}
	f.OnTeamChange(h, feature.TeamChangeEvent{ClientID: 1, Team: -1})
	if last(h) != "'alice' joined the spectators" {
		t.Errorf("spec = %q", last(h))
	}
	f.OnTeamChange(h, feature.TeamChangeEvent{ClientID: 1, Team: 1})
	if last(h) != "'alice' joined the blue team" {
		t.Errorf("blue = %q", last(h))
	}
	f.OnTeamChange(h, feature.TeamChangeEvent{ClientID: 1, Team: 0})
	if last(h) != "'alice' joined the game" {
		t.Errorf("game = %q", last(h))
	}
	f.OnKill(h, feature.KillEvent{Killer: 1, Victim: 2, Weapon: 3})
	if last(h) != "'alice' killed 'bob' (grenade)" {
		t.Errorf("kill = %q", last(h))
	}
	f.OnKill(h, feature.KillEvent{Killer: -1, Victim: 2})
	if last(h) != "'bob' died" {
		t.Errorf("death = %q", last(h))
	}
	// id fallback when the roster has no name (§V26).
	f.OnPlayerLeave(h, feature.PlayerLeaveEvent{ClientID: 3})
	if !strings.Contains(last(h), "'#3'") {
		t.Errorf("id fallback = %q", last(h))
	}
}

// §T106: silent team change and the off switch suppress output.
func TestServerLogSuppression(t *testing.T) {
	f := serverLog{}
	h := newHost()
	f.OnTeamChange(h, feature.TeamChangeEvent{ClientID: 1, Team: -1, Silent: true})
	if len(h.logs) != 0 {
		t.Errorf("silent team change should not log: %v", h.logs)
	}
	h.cfg["cl_show_game_messages"] = "0"
	f.OnPlayerJoin(h, feature.PlayerJoinEvent{ClientID: 1, Name: "alice"})
	f.OnKill(h, feature.KillEvent{Killer: 1, Victim: 2})
	if len(h.logs) != 0 {
		t.Errorf("disabled feature should not log: %v", h.logs)
	}
}

// §B14/§V26: the local player's death uses our configured name even when its
// roster entry is nameless (avoids "'#0' died").
func TestServerLogLocalName(t *testing.T) {
	f := serverLog{}
	h := newHost()
	h.roster = []client.PlayerState{{ClientID: 0, Name: ""}} // local, nameless roster entry
	h.tick = client.TickState{LocalID: 0}
	h.haveTick = true
	h.selfName = "nameless tee"

	f.OnKill(h, feature.KillEvent{Killer: -1, Victim: 0})
	if last(h) != "'nameless tee' died" {
		t.Errorf("local death = %q want 'nameless tee' died", last(h))
	}
}
