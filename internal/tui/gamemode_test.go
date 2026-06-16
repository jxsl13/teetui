package tui

import (
	"testing"

	"github.com/jxsl13/twclient/client"
)

// §T112/§V75: team mode derived from GameFlags.
func TestTeamMode(t *testing.T) {
	var solo client.TickState
	if teamMode(solo) {
		t.Error("zero GameFlags should be solo")
	}
	team := client.TickState{}
	team.GameInfo.GameFlags = gameflagTeams
	if !teamMode(team) {
		t.Error("GAMEFLAG_TEAMS should be team mode")
	}
	ctf := client.TickState{}
	ctf.GameInfo.GameFlags = gameflagTeams | gameflagFlags
	if !teamMode(ctf) {
		t.Error("CTF (teams+flags) is team mode")
	}
}
