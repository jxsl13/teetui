// Package team adds team join / switch — the terminal equivalent of the GUI
// client's team-select menu, which chillerbot-ux's terminal UI lacks (§T92/§V52).
// It is a self-registering feature module: blank-import it from main to enable.
//
// Console:
//
//	team spectators   become a spectator slot (id -1)
//	team red          join the red team / the game in non-team modes (flock, id 0)
//	team blue         join the blue team (id 1)
//	team game         join the game (alias of red/flock, id 0)
//	join              shorthand for "team game"
//
// All comms go through Host.Do(client.ActSetTeam{Team}) — no raw packet (§V12).
package team

import (
	"strings"

	"github.com/jxsl13/teetui/feature"
	"github.com/jxsl13/twclient/client"
)

// team ids (teeworlds / DDNet): spectators -1, red/flock(game) 0, blue 1.
const (
	teamSpectators = -1
	teamRedGame    = 0
	teamBlue       = 1
)

// teamID parses a team argument to its id. "game"/"join" and "red"/"flock" map
// to 0 (joining a non-team game is team 0); numeric -1/0/1 also accepted.
func teamID(arg string) (int, bool) {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "spectators", "spectator", "spec", "-1":
		return teamSpectators, true
	case "red", "game", "flock", "join", "0", "":
		return teamRedGame, true
	case "blue", "1":
		return teamBlue, true
	}
	return 0, false
}

type teamFeature struct{ feature.NopFeature }

func (teamFeature) Name() string { return "team" }

func (f teamFeature) Provision(h feature.Host) error {
	h.DefineCommand("team", "team <spectators|red|blue|game> — join/switch team", func(args string) []string {
		id, ok := teamID(args)
		if !ok {
			return []string{"usage: team <spectators|red|blue|game>"}
		}
		return f.setTeam(h, id)
	})
	h.DefineCommand("join", "join — join the game (team 0)", func(string) []string {
		return f.setTeam(h, teamRedGame)
	})
	return nil
}

// setTeam requests the team change and reports it.
func (teamFeature) setTeam(h feature.Host, id int) []string {
	if err := h.Do(client.ActSetTeam{Team: id}); err != nil {
		return []string{"team change failed: " + err.Error()}
	}
	return []string{"team → " + teamName(id)}
}

func teamName(id int) string {
	switch id {
	case teamSpectators:
		return "spectators"
	case teamBlue:
		return "blue"
	default:
		return "game"
	}
}

func init() { feature.Register(teamFeature{}) }
