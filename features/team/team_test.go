package team

import "testing"

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
