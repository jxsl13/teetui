package tui

import "strings"

// autoReplies is the fixed known-phrase table (← chillerbot chathelper
// replytoping.cpp): a substring in the pinging message maps to a canned reply.
// Not AI — just a useful shortcut bound to the H key.
var autoReplies = []struct{ match, reply string }{
	{"how are you", "good, you?"},
	{"hello", "hello"},
	{"hi ", "hi"},
	{"hey", "hey"},
	{"wanna", "no thanks"},
	{"are you a bot", "no u"},
	{"bot", "i am not a bot"},
	{"afk", "im back"},
	{"alive", "yes"},
}

// autoReply returns a canned reply for a pinging message, if a known phrase
// matches. The match is case-insensitive on the message text (§T23/§T40).
func autoReply(msg string) (string, bool) {
	low := strings.ToLower(msg)
	for _, r := range autoReplies {
		if strings.Contains(low, r.match) {
			return r.reply, true
		}
	}
	return "", false
}

// containsName reports whether msg mentions name (case-insensitive ping
// detection). Empty name never matches.
func containsName(msg, name string) bool {
	if name == "" {
		return false
	}
	return strings.Contains(strings.ToLower(msg), strings.ToLower(name))
}
