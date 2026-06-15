package tui

import "testing"

// §T39: local-console parser dispatches known commands and reports unknowns.
func TestRunConsole(t *testing.T) {
	cases := []struct {
		in   string
		out0 string
		chat string
		quit bool
	}{
		{"", "", "", false},
		{"help", "commands: help, echo <text>, say <msg>, quit, version", "", false},
		{"echo hello there", "hello there", "", false},
		{"say gg wp", "", "gg wp", false},
		{"say", "usage: say <message>", "", false},
		{"quit", "", "", true},
		{"exit", "", "", true},
		{"bogus x", `unknown command: "bogus" (try 'help')`, "", false},
	}
	for _, c := range cases {
		r := runConsole(c.in)
		if r.Quit != c.quit {
			t.Errorf("%q: quit=%v want %v", c.in, r.Quit, c.quit)
		}
		if r.Chat != c.chat {
			t.Errorf("%q: chat=%q want %q", c.in, r.Chat, c.chat)
		}
		got0 := ""
		if len(r.Out) > 0 {
			got0 = r.Out[0]
		}
		if got0 != c.out0 {
			t.Errorf("%q: out=%q want %q", c.in, got0, c.out0)
		}
	}
}

// §T19/§T31: popup builders produce active, titled, escapable popups.
func TestPopups(t *testing.T) {
	g := greetingPopup()
	if !g.active() || g.Kind != popupGreeting || g.Title != "teetui" {
		t.Errorf("greeting popup wrong: %+v", g)
	}
	d := disconnectPopup("kicked")
	if !d.active() || d.Kind != popupDisconnected || d.Body[0] != "kicked" {
		t.Errorf("disconnect popup wrong: %+v", d)
	}
	if e := disconnectPopup(""); e.Body[0] != "connection closed" {
		t.Errorf("empty reason fallback = %q", e.Body[0])
	}
	var none Popup
	if none.active() {
		t.Error("zero popup must be inactive")
	}
}
