package replytoping

import (
	"fmt"
	"strings"

	"github.com/jxsl13/teetui/lang"
)

// smalltalkReply answers small-talk pings in the sender's language (← chillerbot
// smalltalk.cpp), addressing the author. ("", false) when not small talk.
func smalltalkReply(msg, from string) (string, bool) {
	switch {
	case lang.ContainsAny(msg, "how are you", "how r u", "how ru", "how r you", "how are u", "how is it going", "hows it going"):
		return from + " good, and you? :)", true
	case lang.ContainsAny(msg, "wie gehts", "wie geht es", "wie geht's", "was geht"):
		return from + " gut, und dir? :)", true
	case lang.ContainsAny(msg, "ça va", "ca va", "çv"):
		return "je vais bien, et toi " + from + " ?", true
	case lang.ContainsAny(msg, "et toi"):
		return from + " moi aussi, merci", true
	case lang.ContainsAny(msg, "about you", "and you", "and u", "wbu", "hbu"):
		return from + " good", true
	}
	return "", false
}

// composeReply builds a context-aware reply to a ping (← chillerbot reply-to-
// ping, §T79/§V33): small talk, ask-to-ask, greeting, bye, insult, no-context.
func composeReply(msg, from, self string) (string, bool) {
	if from == "" {
		return "", false
	}
	if r, ok := smalltalkReply(msg, from); ok {
		return r, true
	}
	switch {
	case lang.IsAskToAsk(msg):
		return from + " just ask :)", true
	case lang.IsGreeting(msg):
		return from + " hi", true
	case lang.IsBye(msg):
		return from + " cya", true
	case lang.IsInsult(msg):
		return from + " :)", true
	}
	if isNoContextPing(msg, self) {
		return from + " ?", true
	}
	return "", false
}

// isNoContextPing reports whether msg is essentially just our name.
func isNoContextPing(msg, self string) bool {
	if self == "" {
		return false
	}
	low := strings.ReplaceAll(strings.ToLower(msg), strings.ToLower(self), " ")
	low = strings.Map(func(r rune) rune {
		switch r {
		case ':', '@', ',', '!', '?', '.', '"', '\'':
			return ' '
		}
		return r
	}, low)
	return strings.TrimSpace(low) == ""
}

// warlistService is the MINIMAL view of the warlist this feature needs; it is
// declared here (not in the SDK, §V53) and satisfied structurally by whatever
// the "warlist" feature Provides.
type warlistService interface {
	Relation(name string) string
	Reason(name string) string
	NamesWith(relation string) []string
	ClansWith(relation string) []string
}

// queryEnv is the read-only state a chat-query answer may use (§T80/§V34).
type queryEnv struct {
	warlist     warlistService
	rosterNames []string
	selfClan    string
	haveCoords  bool
	coordX      int
	coordY      int
	goos        string
}

// composeQueryReply answers a pinging question from state — war status, war
// lists, clan-join, position, OS (← chillerbot check_war/list_wars/where/
// operating_system, §T80/§V34). ("", false) when not a recognized query.
func composeQueryReply(msg, from string, env queryEnv) (string, bool) {
	if from == "" {
		return "", false
	}
	if lang.ContainsAny(msg, "what os", "which os", "what system", "operating system", "betriebssystem") ||
		(lang.FindWord(msg, "os") && lang.HasQuestionMark(msg)) {
		return fmt.Sprintf("%s %s", from, env.goos), true
	}
	if lang.ContainsAny(msg, "where are you", "where r u", "where ru", "where u at", "wo bist du", "où es", "ou es tu") {
		if env.haveCoords {
			return fmt.Sprintf("%s x:%d y:%d", from, env.coordX, env.coordY), true
		}
		return from + " spectating", true
	}
	if lang.IsQuestionWhy(msg) && lang.FindAnyWord(msg, "kill", "killed", "kills", "killst", "tötest") {
		if env.warlist != nil && env.warlist.Relation(from) == "war" {
			if reason := env.warlist.Reason(from); reason != "" {
				return fmt.Sprintf("%s because: %s", from, reason), true
			}
			return from + " you are on my war list", true
		}
		return from + " sorry, didn't mean to", true
	}
	if env.warlist != nil && (lang.ContainsAny(msg, "list war", "your wars", "list your war", "who do you war", "show wars") ||
		(lang.FindWord(msg, "wars") && lang.HasQuestionMark(msg))) {
		wars := env.warlist.NamesWith("war")
		clans := env.warlist.ClansWith("war")
		if len(wars) == 0 && len(clans) == 0 {
			return from + " no wars", true
		}
		parts := append([]string{}, capList(wars, 8)...)
		for _, c := range capList(clans, 4) {
			parts = append(parts, "clan:"+c)
		}
		return from + " wars: " + strings.Join(parts, ", "), true
	}
	if env.warlist != nil && lang.FindAnyWord(msg, "war", "peace", "team", "enemy", "friend") {
		for _, name := range env.rosterNames {
			if name == "" || name == from {
				continue
			}
			if lang.FindWord(msg, name) {
				rel := env.warlist.Relation(name)
				if rel == "" {
					return fmt.Sprintf("%s %s is neutral", from, name), true
				}
				ans := fmt.Sprintf("%s %s is %s", from, name, rel)
				if reason := env.warlist.Reason(name); reason != "" {
					ans += " (" + reason + ")"
				}
				return ans, true
			}
		}
	}
	if lang.FindWord(msg, "clan") && lang.ContainsAny(msg, "join", "enter", "let me", "can i", "how do", "beitreten") {
		if env.selfClan != "" {
			return fmt.Sprintf("%s ask a member of [%s]", from, env.selfClan), true
		}
		return from + " i have no clan", true
	}
	return "", false
}

func capList(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
