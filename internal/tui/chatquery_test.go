package tui

import (
	"strings"
	"testing"
)

// §T62/§V34: chat-query answers come only from teetui state and address the
// pinger. Covers war-status (self + others), war list, where, OS, clan-join.
func TestComposeQueryReply(t *testing.T) {
	w := NewWarlist()
	w.Set("enemy1", RelWar)
	w.SetReason("enemy1", "blocked me")
	w.Set("buddy", RelTeam)

	env := queryEnv{
		warlist:     w,
		rosterNames: []string{"enemy1", "buddy", "stranger"},
		selfClan:    "ACAB",
		haveCoords:  true, coordX: 42, coordY: 7,
		goos: "linux",
	}

	check := func(msg, from, wantSub string, wantOK bool) {
		t.Helper()
		got, ok := composeQueryReply(msg, from, env)
		if ok != wantOK {
			t.Errorf("query(%q) ok=%v want %v (got %q)", msg, ok, wantOK, got)
			return
		}
		if ok && !strings.Contains(got, wantSub) {
			t.Errorf("query(%q) = %q want substring %q", msg, got, wantSub)
		}
		if ok && !strings.Contains(got, from) {
			t.Errorf("query(%q) = %q must address %q", msg, got, from)
		}
	}

	check("self what os are you on?", "bob", "linux", true)
	check("self where are you?", "bob", "x:42 y:7", true)
	check("self why do you kill me?", "enemy1", "blocked me", true) // self war reason
	check("self why do you kill me?", "stranger", "didn't mean", true)
	check("is enemy1 war?", "bob", "enemy1 is war", true)
	check("is buddy war or what", "bob", "buddy is team", true)
	check("list your wars", "bob", "enemy1", true)
	check("can i join your clan?", "bob", "ACAB", true)

	// Not a query → fall through.
	check("self nice weather today", "bob", "", false)

	// No coords → spectating.
	env.haveCoords = false
	if got, ok := composeQueryReply("self where are you", "bob", env); !ok || !strings.Contains(got, "spectating") {
		t.Errorf("where without coords = %q ok=%v", got, ok)
	}
}

// §T62: empty warlist lists "no wars".
func TestQueryNoWars(t *testing.T) {
	env := queryEnv{warlist: NewWarlist(), goos: "darwin"}
	if got, ok := composeQueryReply("list your wars", "bob", env); !ok || !strings.Contains(got, "no wars") {
		t.Errorf("empty war list = %q ok=%v", got, ok)
	}
}
