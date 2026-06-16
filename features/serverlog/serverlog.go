// Package serverlog shows the game events a normal DDNet client shows — players
// joining, leaving, switching team / joining spectators, and the kill feed
// (who killed whom, or a plain death) — as log lines (§C30/§T106). It is a
// self-registering feature on the public SDK: blank-import it from main to enable.
//
// All lines are generated CLIENT-side from twclient's unified 0.6/0.7 events
// (PlayerJoin/PlayerLeave/TeamChange/Kill), never from raw packets (§V1). Names
// are resolved from the roster with an id fallback (§V26). Message wording mirrors
// the DDNet client (src/game/client): "'%s' entered and joined the game",
// "'%s' has left the game (%s)", "'%s' joined the spectators", etc.
package serverlog

import (
	"fmt"

	"github.com/jxsl13/teetui/feature"
)

type serverLog struct{}

func (serverLog) Name() string { return "serverlog" }

func (serverLog) Init(h feature.API) error {
	h.DefineConfig("cl_show_game_messages", "1", "show join/leave/team/kill game messages (0/1)")
	return nil
}

func (f serverLog) enabled(h feature.API) bool {
	v, _ := h.Config("cl_show_game_messages")
	return v == "1" || v == "true" || v == "on"
}

// resolveName resolves a client id to its REAL name (roster name, or our own
// configured name for the local nameless entry, §B14/§V26). ok=false means no
// real name is known — only the "#id" placeholder applies; callers that would
// emit pure noise (a leave of a never-named/phantom slot, §B19/§V91) suppress
// the line instead of printing "'#id'".
func resolveName(h feature.API, id int) (string, bool) {
	for _, p := range h.Roster() {
		if p.ClientID == id {
			if p.Name != "" {
				return p.Name, true
			}
			break
		}
	}
	// The local player's roster name may be empty even though we know it (§B14).
	if st, ok := h.Tick(); ok && id == st.LocalID {
		if n := h.PlayerName(); n != "" {
			return n, true
		}
	}
	return fmt.Sprintf("#%d", id), false
}

// nameOf resolves a client id to a displayable name, falling back to "#id" when
// the real name is unknown (join/team/kill keep the placeholder, §V26).
func nameOf(h feature.API, id int) string {
	n, _ := resolveName(h, id)
	return n
}

func (f serverLog) OnPlayerJoin(h feature.API, e feature.PlayerJoinEvent) {
	if !f.enabled(h) {
		return
	}
	name := e.Name
	if name == "" {
		name = nameOf(h, e.ClientID)
	}
	h.Log(fmt.Sprintf("'%s' entered and joined the game", name))
}

func (f serverLog) OnPlayerLeave(h feature.API, e feature.PlayerLeaveEvent) {
	if !f.enabled(h) {
		return
	}
	name, ok := resolveName(h, e.ClientID)
	if !ok {
		return // never-named/phantom slot → suppress "'#id' has left the game" noise (§B19/§V91)
	}
	if e.Reason != "" {
		h.Log(fmt.Sprintf("'%s' has left the game (%s)", name, e.Reason))
		return
	}
	h.Log(fmt.Sprintf("'%s' has left the game", name))
}

func (f serverLog) OnTeamChange(h feature.API, e feature.TeamChangeEvent) {
	if !f.enabled(h) || e.Silent {
		return
	}
	h.Log(fmt.Sprintf("'%s' %s", nameOf(h, e.ClientID), teamPhrase(e.Team)))
}

// teamPhrase renders the DDNet team-change wording. Team ids: spectators -1,
// red/flock(game) 0, blue 1. On a flock (non-team) server 0 is "the game"; this
// is the common case for a terminal client.
func teamPhrase(team int) string {
	switch team {
	case -1:
		return "joined the spectators"
	case 1:
		return "joined the blue team"
	default:
		return "joined the game"
	}
}

func (f serverLog) OnKill(h feature.API, e feature.KillEvent) {
	if !f.enabled(h) {
		return
	}
	victim := nameOf(h, e.Victim)
	// No killer (world/self) → a plain death, like the DDNet kill feed.
	if e.Killer < 0 || e.Killer == e.Victim {
		h.Log(fmt.Sprintf("'%s' died", victim))
		return
	}
	killer := nameOf(h, e.Killer)
	if w := weaponName(e.Weapon); w != "" {
		h.Log(fmt.Sprintf("'%s' killed '%s' (%s)", killer, victim, w))
		return
	}
	h.Log(fmt.Sprintf("'%s' killed '%s'", killer, victim))
}

// weaponName maps the kill weapon id to a label ("" if unknown). Order follows
// Teeworlds: hammer, gun(pistol), shotgun, grenade, laser, ninja.
func weaponName(w int) string {
	switch w {
	case 0:
		return "hammer"
	case 1:
		return "gun"
	case 2:
		return "shotgun"
	case 3:
		return "grenade"
	case 4:
		return "laser"
	case 5:
		return "ninja"
	default:
		return ""
	}
}

func init() { feature.Register(serverLog{}) }
