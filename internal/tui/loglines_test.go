package tui

import "testing"

// §T88/§V49: cl_log_lines sets the visual-on log band, and the ⌊h/2⌋ cap is
// applied at render (Compute), not at set time.
func TestLogLinesCvar(t *testing.T) {
	cfg := NewConfig()
	if cfg.LogLines != DefaultLogLines {
		t.Errorf("default LogLines = %d want %d", cfg.LogLines, DefaultLogLines)
	}
	if r := runConsole("cl_log_lines 4", cfg); r.Out[0] != `cl_log_lines set to "4"` {
		t.Errorf("set = %q", r.Out[0])
	}
	if cfg.LogLines != 4 {
		t.Errorf("LogLines = %d want 4", cfg.LogLines)
	}

	// Requested 4 fits in a tall terminal → band == 4.
	if l := Compute(80, 40, true, cfg.LogLines); l.Log.H != 4 {
		t.Errorf("band = %d want 4", l.Log.H)
	}
	// On a short terminal the same request is capped to ⌊h/2⌋ at render.
	if l := Compute(80, 12, true, 4); l.Log.H != 6 && l.Log.H > 6 {
		t.Errorf("band on short term = %d want <=6", l.Log.H)
	}
}

// §T88: SetLogLines guards against non-positive values.
func TestSetLogLines(t *testing.T) {
	app, _ := newTestApp(t)
	app.SetLogLines(0)
	if app.cfg.LogLines != DefaultLogLines {
		t.Errorf("0 → %d want default %d", app.cfg.LogLines, DefaultLogLines)
	}
	app.SetLogLines(25)
	if app.cfg.LogLines != 25 {
		t.Errorf("25 → %d", app.cfg.LogLines)
	}
}
