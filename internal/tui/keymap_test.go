package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// §V19/§T42: the default keymap resolves the §I table (and the foundation
// aliases) to the right actions.
func TestKeymapDefaults(t *testing.T) {
	k := DefaultKeymap()
	cases := []struct {
		key tcell.Key
		r   rune
		act KeyAction
	}{
		{tcell.KeyRune, '?', actHelp},
		{tcell.KeyRune, 'B', actBrowser},
		{tcell.KeyRune, 'T', actChat},
		{tcell.KeyRune, 't', actChat},
		{tcell.KeyRune, 'Z', actTeamChat},
		{tcell.KeyRune, 'R', actReconnect},
		{tcell.KeyRune, ' ', actJump},
		{tcell.KeyRune, 'f', actFire},
		{tcell.KeyF1, 0, actLocalConsole},
		{tcell.KeyF2, 0, actRemoteConsole},
		{tcell.KeyTab, 0, actScoreboard},
		{tcell.KeyEscape, 0, actQuit},
	}
	for _, c := range cases {
		if got, ok := k.Lookup(c.key, c.r); !ok || got != c.act {
			t.Errorf("Lookup(%v,%q) = %v,%v want %v", c.key, c.r, got, ok, c.act)
		}
	}
	if _, ok := k.Lookup(tcell.KeyRune, 'Q'); ok {
		t.Error("unbound key Q should not resolve")
	}
}

// §V19: a config override rebinds an action and removes its default key.
func TestKeymapOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keymap.txt")
	if err := os.WriteFile(path, []byte("# custom\nchat = c\nquit = x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	k := &Keymap{}
	if err := k.Load(path); err != nil {
		t.Fatal(err)
	}
	if got, ok := k.Lookup(tcell.KeyRune, 'c'); !ok || got != actChat {
		t.Errorf("override c -> %v,%v want chat", got, ok)
	}
	// The old default key for chat must no longer trigger it.
	if _, ok := k.Lookup(tcell.KeyRune, 't'); ok {
		t.Error("default chat key t should be cleared by override")
	}
	if got, ok := k.Lookup(tcell.KeyRune, 'x'); !ok || got != actQuit {
		t.Errorf("override x -> %v,%v want quit", got, ok)
	}
	// Untouched actions keep their defaults.
	if got, ok := k.Lookup(tcell.KeyRune, 'B'); !ok || got != actBrowser {
		t.Errorf("untouched browser binding lost: %v,%v", got, ok)
	}
}

// §V19: a missing config file leaves defaults intact.
func TestKeymapLoadMissing(t *testing.T) {
	k := &Keymap{}
	if err := k.Load(filepath.Join(t.TempDir(), "nope.txt")); err != nil {
		t.Fatal(err)
	}
	if got, ok := k.Lookup(tcell.KeyRune, '?'); !ok || got != actHelp {
		t.Errorf("missing-file defaults wrong: %v,%v", got, ok)
	}
}

// §V19: save then load round-trips every binding, including named keys.
func TestKeymapRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keymap.txt")
	orig := DefaultKeymap()
	if err := orig.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded := &Keymap{}
	if err := loaded.Load(path); err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		key tcell.Key
		r   rune
	}{
		{tcell.KeyRune, '?'}, {tcell.KeyRune, ' '}, {tcell.KeyRune, 'f'},
		{tcell.KeyF1, 0}, {tcell.KeyF6, 0}, {tcell.KeyTab, 0}, {tcell.KeyEscape, 0},
	}
	for _, c := range checks {
		want, wok := orig.Lookup(c.key, c.r)
		got, gok := loaded.Lookup(c.key, c.r)
		if wok != gok || want != got {
			t.Errorf("round-trip Lookup(%v,%q): got %v,%v want %v,%v", c.key, c.r, got, gok, want, wok)
		}
	}
}
