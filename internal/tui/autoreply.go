package tui

import "strings"

// containsName reports whether msg mentions name (case-insensitive ping
// detection). Empty name never matches.
func containsName(msg, name string) bool {
	if name == "" {
		return false
	}
	return strings.Contains(strings.ToLower(msg), strings.ToLower(name))
}

// smalltalkReply answers small-talk pings in the sender's language (← chillerbot
// chathelper/smalltalk.cpp), addressing the author. Returns ("", false) when the
// message is not small talk.
func smalltalkReply(msg, from string) (string, bool) {
	switch {
	case containsAny(msg, "how are you", "how r u", "how ru", "how r you", "how are u", "how is it going", "hows it going"):
		return from + " good, and you? :)", true
	case containsAny(msg, "wie gehts", "wie geht es", "wie geht's", "was geht"):
		return from + " gut, und dir? :)", true
	case containsAny(msg, "ça va", "ca va", "çv"):
		return "je vais bien, et toi " + from + " ?", true
	case containsAny(msg, "et toi"):
		return from + " moi aussi, merci", true
	case containsAny(msg, "about you", "and you", "and u", "wbu", "hbu"):
		return from + " good", true
	}
	return "", false
}

// composeReply builds a context-aware reply to a ping from `from` (our name is
// `self`), mirroring chillerbot's reply-to-ping (§T61/§V33). It tries, in order:
// small talk, ask-to-ask, greeting, bye, insult, then a no-context bare ping. It
// returns ("", false) when nothing fits so the caller can fall back.
func composeReply(msg, from, self string) (string, bool) {
	if from == "" {
		return "", false
	}
	if r, ok := smalltalkReply(msg, from); ok {
		return r, true
	}
	switch {
	case isAskToAsk(msg):
		return from + " just ask :)", true
	case isGreeting(msg):
		return from + " hi", true
	case isBye(msg):
		return from + " cya", true
	case isInsult(msg):
		return from + " :)", true
	}
	// No-context ping: the message is essentially just our name (maybe with a
	// colon), so answer with a questioning nudge ("<from> ?", ← IsNoContextPing).
	if isNoContextPing(msg, self) {
		return from + " ?", true
	}
	return "", false
}

// isNoContextPing reports whether msg carries no content beyond our name (and
// trivial punctuation) — e.g. "self", "self:", "@self".
func isNoContextPing(msg, self string) bool {
	if self == "" {
		return false
	}
	low := strings.ToLower(msg)
	low = strings.ReplaceAll(low, strings.ToLower(self), " ")
	low = strings.Map(func(r rune) rune {
		switch r {
		case ':', '@', ',', '!', '?', '.', '"', '\'':
			return ' '
		}
		return r
	}, low)
	return strings.TrimSpace(low) == ""
}

// expandAutoReply renders the cl_auto_reply_msg template: %n → author name (←
// chillerbot cl_auto_reply_msg). Other text passes through unchanged.
func expandAutoReply(tmpl, from string) string {
	return strings.ReplaceAll(tmpl, "%n", from)
}
