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

// §C43/§V93: derived dummy name is distinct, fits 15 bytes, keeps the (N) suffix.
func TestDeriveDummyName(t *testing.T) {
	if got := deriveDummyName("bob", 1); got != "bob(1)" {
		t.Errorf("derive(bob,1) = %q want bob(1)", got)
	}
	if got := deriveDummyName("bob", 2); got != "bob(2)" {
		t.Errorf("derive(bob,2) = %q want bob(2)", got)
	}
	// Long base: the (N) suffix survives, the base shrinks to keep ≤15 bytes.
	long := deriveDummyName("abcdefghijklmnopqrst", 1) // base 20 + "(1)"
	if len(long) > maxNameBytes {
		t.Errorf("derive long = %q (%d bytes), want ≤%d", long, len(long), maxNameBytes)
	}
	if got := long[len(long)-3:]; got != "(1)" {
		t.Errorf("derive long suffix = %q want (1)", got)
	}
	// n<1 normalizes to 1.
	if got := deriveDummyName("x", 0); got != "x(1)" {
		t.Errorf("derive(x,0) = %q want x(1)", got)
	}
}

// §C43/§V93: dummyName uses cl_dummy_name when set (clipped), else derives.
func TestDummyNameCvarVsDerived(t *testing.T) {
	app, _ := newTestApp(t)
	d := app.newSession("dummy", nil, nil)
	app.addSession(d) // ordinal 1

	// Empty cvar → derived distinct name from the main name.
	if got := app.dummyName("bob", d); got != "bob(1)" {
		t.Errorf("derived dummyName = %q want bob(1)", got)
	}
	// Explicit cvar overrides.
	app.cfg.DummyName = "mybot"
	if got := app.dummyName("bob", d); got != "mybot" {
		t.Errorf("cvar dummyName = %q want mybot", got)
	}
}
