package tui

import "unicode/utf8"

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

// dummyFallbackName is DDNet's default dummy name when no player/dummy name is
// set (CClient::DummyName(), §C43/§V93).
const dummyFallbackName = "brainless tee"

// deriveDummyName mirrors DDNet `CClient::DummyName()`: "[D] " + the main player
// name, rune-clipped to maxNameBytes valid utf8 (the prefix survives, the name
// tail clips). An empty main name yields DDNet's "brainless tee" fallback. The
// server dedupes duplicate same-IP names itself, so no client-added index
// (§C43/§V93).
func deriveDummyName(main string) string {
	if main == "" {
		return dummyFallbackName
	}
	return clipName("[D] " + main)
}

// dummyName resolves the name for a dummy session exactly as DDNet does: the
// cl_dummy_name cvar if set (clipped), else the "[D] "-derived name (§C43/§V93).
func (a *App) dummyName(main string) string {
	if dn := clipName(a.cfgSnap().DummyName); dn != "" {
		return dn
	}
	return deriveDummyName(main)
}
