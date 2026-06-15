package replytoping

import (
	"strings"
	"testing"
)

// §T79/§V33: composeReply picks a context-aware, author-addressed reply and
// declines when nothing fits.
func TestComposeReply(t *testing.T) {
	const self = "nameless"
	cases := []struct {
		msg, sub string
		ok       bool
	}{
		{"nameless how are you?", "good, and you", true},
		{"nameless wie gehts", "gut, und dir", true},
		{"nameless can i ask you something", "just ask", true},
		{"hey nameless", "hi", true},
		{"nameless cya", "cya", true},
		{"nameless you noob", ":)", true},
		{"nameless", "?", true},
		{"nameless:", "?", true},
		{"nameless can you carry this part", "", false},
	}
	for _, c := range cases {
		got, ok := composeReply(c.msg, "bob", self)
		if ok != c.ok {
			t.Errorf("composeReply(%q) ok=%v want %v (%q)", c.msg, ok, c.ok, got)
			continue
		}
		if ok && (!strings.Contains(got, "bob") || !strings.Contains(got, c.sub)) {
			t.Errorf("composeReply(%q) = %q want author+%q", c.msg, got, c.sub)
		}
	}
	if _, ok := composeReply("hi nameless", "", self); ok {
		t.Error("empty author must not reply")
	}
}

type fakeWarlist struct {
	rel    map[string]string
	reason map[string]string
}

func (f fakeWarlist) Relation(n string) string { return f.rel[n] }
func (f fakeWarlist) Reason(n string) string   { return f.reason[n] }
func (f fakeWarlist) NamesWith(r string) []string {
	var out []string
	for n, rr := range f.rel {
		if rr == r {
			out = append(out, n)
		}
	}
	return out
}
func (f fakeWarlist) ClansWith(string) []string { return nil }

// §T80/§V34: query answers from state, author-addressed.
func TestComposeQueryReply(t *testing.T) {
	env := queryEnv{
		warlist:     fakeWarlist{rel: map[string]string{"enemy1": "war", "buddy": "team"}, reason: map[string]string{"enemy1": "blocked me"}},
		rosterNames: []string{"enemy1", "buddy", "stranger"},
		selfClan:    "ACAB",
		haveCoords:  true, coordX: 42, coordY: 7,
		goos: "linux",
	}
	check := func(msg, from, sub string, ok bool) {
		t.Helper()
		got, gotOK := composeQueryReply(msg, from, env)
		if gotOK != ok {
			t.Errorf("query(%q) ok=%v want %v (%q)", msg, gotOK, ok, got)
			return
		}
		if ok && (!strings.Contains(got, from) || !strings.Contains(got, sub)) {
			t.Errorf("query(%q) = %q want author+%q", msg, got, sub)
		}
	}
	check("self what os?", "bob", "linux", true)
	check("self where are you?", "bob", "x:42 y:7", true)
	check("self why do you kill me?", "enemy1", "blocked me", true)
	check("self why do you kill me?", "stranger", "didn't mean", true)
	check("is enemy1 war?", "bob", "enemy1 is war", true)
	check("is buddy war or what", "bob", "buddy is team", true)
	check("list your wars", "bob", "enemy1", true)
	check("can i join your clan?", "bob", "ACAB", true)
	check("nice weather", "bob", "", false)

	env.haveCoords = false
	if got, ok := composeQueryReply("self where are you", "bob", env); !ok || !strings.Contains(got, "spectating") {
		t.Errorf("where w/o coords = %q ok=%v", got, ok)
	}
	env2 := queryEnv{warlist: fakeWarlist{rel: map[string]string{}}, goos: "darwin"}
	if got, ok := composeQueryReply("list your wars", "bob", env2); !ok || !strings.Contains(got, "no wars") {
		t.Errorf("empty wars = %q ok=%v", got, ok)
	}
}
