package tui

import (
	"strconv"
	"unicode/utf8"
)

// maxNameBytes is the usable player-name length: Teeworlds MAX_NAME_LENGTH is 16
// bytes INCLUDING the null terminator, so 15 bytes of content (§C43/§V93).
const maxNameBytes = 15

// clipName clips s to at most maxNameBytes BYTES on a UTF-8 RUNE boundary, so the
// name teetui sends is always valid utf8 — the server byte-clips at 15 and a cut
// through a multibyte rune renders as an invalid-utf8 glyph (§B21).
func clipName(s string) string { return clipNameTo(s, maxNameBytes) }

// clipNameTo clips s to at most n bytes, never splitting a multibyte rune.
func clipNameTo(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s // already fits (len is byte length)
	}
	out := make([]byte, 0, n)
	for _, r := range s {
		if len(out)+utf8.RuneLen(r) > n {
			break
		}
		out = utf8.AppendRune(out, r)
	}
	return string(out)
}

// deriveDummyName builds a DDNet-style distinct dummy name from the main name:
// "name(n)" (the Teeworlds/DDNet duplicate-name convention), with the base
// rune-clipped so the whole result fits maxNameBytes valid utf8 (§C43/§V93).
func deriveDummyName(main string, n int) string {
	if n < 1 {
		n = 1
	}
	suffix := "(" + strconv.Itoa(n) + ")"
	return clipNameTo(main, maxNameBytes-len(suffix)) + suffix
}

// dummyName resolves the name for a dummy session: the cl_dummy_name cvar if set
// (clipped), else a DDNet-style derived distinct name from the main player name,
// using the session's ordinal as the duplicate index (§C43/§V93).
func (a *App) dummyName(main string, s *session) string {
	if dn := clipName(a.cfgSnap().DummyName); dn != "" {
		return dn
	}
	return deriveDummyName(main, a.sessionOrdinal(s))
}
