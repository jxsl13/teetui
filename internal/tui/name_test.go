package tui

import (
	"testing"
	"unicode/utf8"
)

// §C43/§V93: clipName never produces invalid utf8 and never exceeds 15 bytes.
func TestClipName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"bob", "bob"},
		{"nameless tee", "nameless tee"},         // 12 bytes, fits
		{"abcdefghijklmno", "abcdefghijklmno"},   // exactly 15
		{"abcdefghijklmnopq", "abcdefghijklmno"}, // 17 → clipped to 15 ascii
	}
	for _, c := range cases {
		if got := clipName(c.in); got != c.want {
			t.Errorf("clipName(%q) = %q want %q", c.in, got, c.want)
		}
	}

	// Multibyte: 6 Japanese runes × 3 bytes = 18 bytes → must clip on a rune
	// boundary to 15 bytes (5 runes), still valid utf8, no partial rune.
	jp := "あいうえおか" // 18 bytes
	got := clipName(jp)
	if len(got) > maxNameBytes {
		t.Errorf("clipName(jp) = %d bytes, want ≤%d", len(got), maxNameBytes)
	}
	if !utf8.ValidString(got) {
		t.Errorf("clipName(jp) = %q is not valid utf8", got)
	}
	if got != "あいうえお" { // 5 runes = 15 bytes
		t.Errorf("clipName(jp) = %q want first 5 runes", got)
	}
}

// §C43/§V93: dummy name mirrors DDNet CClient::DummyName() — "[D] "+name, the
// "brainless tee" fallback, rune-clipped valid utf8.
func TestDeriveDummyName(t *testing.T) {
	if got := deriveDummyName("bob"); got != "[D] bob" {
		t.Errorf("derive(bob) = %q want [D] bob", got)
	}
	if got := deriveDummyName(""); got != "brainless tee" {
		t.Errorf("derive(empty) = %q want brainless tee (DDNet fallback)", got)
	}
	// Long name: the "[D] " prefix (front) survives, the name tail clips to ≤15.
	long := deriveDummyName("abcdefghijklmnopqrst") // "[D] " + 20 chars
	if len(long) > maxNameBytes {
		t.Errorf("derive long = %q (%d bytes), want ≤%d", long, len(long), maxNameBytes)
	}
	if got := long[:4]; got != "[D] " {
		t.Errorf("derive long prefix = %q want '[D] '", got)
	}
	// Multibyte tail clips on a rune boundary (valid utf8).
	jp := deriveDummyName("あいうえおか")
	if !utf8.ValidString(jp) || len(jp) > maxNameBytes {
		t.Errorf("derive jp = %q (%d bytes, valid=%v)", jp, len(jp), utf8.ValidString(jp))
	}
}

// §C43/§V93: dummyName uses cl_dummy_name when set (clipped), else derives.
func TestDummyNameCvarVsDerived(t *testing.T) {
	app, _ := newTestApp(t)

	// Empty cvar → DDNet-derived "[D] " name from the main name.
	if got := app.dummyName("bob"); got != "[D] bob" {
		t.Errorf("derived dummyName = %q want [D] bob", got)
	}
	// Explicit cvar overrides.
	app.cfg.DummyName = "mybot"
	if got := app.dummyName("bob"); got != "mybot" {
		t.Errorf("cvar dummyName = %q want mybot", got)
	}
}
