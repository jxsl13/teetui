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
		{"help", "commands: help [cmd], echo <text>, say <msg>, spec [name], quit, version", "", false},
		{"echo hello there", "hello there", "", false},
		{"say gg wp", "", "gg wp", false},
		{"say", "usage: say <message>", "", false},
		{"quit", "", "", true},
		{"exit", "", "", true},
		{"bogus x", `unknown command: "bogus" (try 'help')`, "", false},
	}
	cfg := NewConfig()
	for _, c := range cases {
		r := runConsole(c.in, cfg)
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

	// §T37: spectate/pause parsing.
	if r := runConsole("spec Nameless", cfg); !r.Spectate || r.SpecName != "Nameless" {
		t.Errorf("spec parse: %+v", r)
	}
	if r := runConsole("pause", cfg); !r.Spectate || r.SpecName != "" {
		t.Errorf("pause free-view: %+v", r)
	}
	if r := runConsole("spectate Foo Bar", cfg); !r.Spectate || r.SpecName != "Foo Bar" {
		t.Errorf("spectate multiword: %+v", r)
	}
}

// §T39: config cvars are readable and settable from the console, and unknown
// help is reported. §T40: the tapped-out cvar toggles the auto-reply.
func TestConsoleCvars(t *testing.T) {
	cfg := NewConfig()

	// Bare cvar prints its current value (cl_log_lines default 10).
	if r := runConsole("cl_log_lines", cfg); r.Out[0] != `cl_log_lines = "10"` {
		t.Errorf("get default = %q", r.Out[0])
	}
	// Setting mutates cfg and echoes the new value.
	if r := runConsole("cl_log_lines 5", cfg); r.Out[0] != `cl_log_lines set to "5"` {
		t.Errorf("set = %q", r.Out[0])
	}
	if cfg.LogLines != 5 {
		t.Error("cl_log_lines should be 5 after set")
	}

	// help <cmd> yields the help-text line; unknown is reported.
	if r := runConsole("help echo", cfg); r.Out[0] != builtinHelp["echo"] {
		t.Errorf("help echo = %q", r.Out[0])
	}
	if h := consoleHelp("cl_log_lines"); h == "" {
		t.Error("consoleHelp for cvar should be non-empty")
	}
	if h := consoleHelp("nope"); h != "" {
		t.Errorf("consoleHelp unknown = %q want empty", h)
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
