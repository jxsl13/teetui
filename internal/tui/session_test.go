package tui

import "testing"

// §T113/§V77: the app starts with one primary session; add/follow/drop manage the
// active session; the primary is never dropped.
func TestSessionModel(t *testing.T) {
	app, _ := newTestApp(t)

	if len(app.sessions) != 1 || app.active != 0 {
		t.Fatalf("start sessions=%d active=%d want 1/0", len(app.sessions), app.active)
	}
	primary := app.cur()
	if !app.isPrimary(primary) {
		t.Error("session 0 should be primary")
	}

	// Add a dummy and follow it.
	d := app.newSession("dummy", nil, nil)
	i := app.addSession(d)
	if i != 1 {
		t.Fatalf("dummy index = %d want 1", i)
	}
	app.setActive(i)
	if app.cur() != d {
		t.Error("setActive did not switch the active session")
	}
	if app.isPrimary(d) {
		t.Error("dummy must not be primary")
	}

	// Drop the dummy → active falls back to the primary.
	app.dropSession(d)
	if len(app.sessions) != 1 || app.cur() != primary {
		t.Errorf("after drop: sessions=%d active=%v want 1/primary", len(app.sessions), app.cur() == primary)
	}

	// The primary cannot be dropped.
	app.dropSession(primary)
	if len(app.sessions) != 1 {
		t.Error("primary must not be droppable")
	}
}
