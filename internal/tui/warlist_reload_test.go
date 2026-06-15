package tui

import (
	"os"
	"testing"
	"time"
)

// §T66/§V14: checkWarlistReload re-reads the warlist file when its mtime
// advances, so external edits apply live.
func TestWarlistAutoReload(t *testing.T) {
	app, _ := newTestApp(t)
	if app.warlistPath == "" {
		t.Skip("no config dir")
	}

	// Externally write a warlist marking "enemy" as war, with a fresh mtime.
	if err := os.WriteFile(app.warlistPath, []byte("war\tenemy\tcheating\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app.warlistMtime = time.Time{} // force "older than file"

	app.checkWarlistReload()

	if got := app.warlist.Get("enemy"); got != RelWar {
		t.Errorf("after reload enemy = %v want RelWar", got)
	}
	if got := app.warlist.Reason("enemy"); got != "cheating" {
		t.Errorf("reason = %q want cheating", got)
	}
	if app.warlistMtime.IsZero() {
		t.Error("mtime not advanced after reload")
	}

	// No further change → a second call is a no-op (mtime not advanced).
	before := app.warlistMtime
	app.checkWarlistReload()
	if !app.warlistMtime.Equal(before) {
		t.Error("mtime changed without a file edit")
	}
}
