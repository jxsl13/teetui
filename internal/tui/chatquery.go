package tui

import (
	"fmt"
	"strings"

	"github.com/jxsl13/teetui/feature"
)

// queryEnv is the read-only state a chat-query answer may draw on (§T62/§V34).
// Answers come ONLY from these fields — teetui never fabricates an answer.
type queryEnv struct {
	warlist     feature.Warlist // warlist service ("" relation = neutral); nil = absent
	rosterNames []string        // names currently on the server, for "is X war?"
	selfClan    string          // our clan tag, for "how to join your clan"
	haveCoords  bool
	coordX      int
	coordY      int
	goos        string // runtime.GOOS, for "what OS?"
}

// composeQueryReply answers a pinging *question* from teetui state — war status,
// war lists, clan-join, position, OS (← chillerbot chathelper check_war /
// list_wars / where / operating_system). Returns ("", false) when msg is not a
// recognized query, so the caller falls through to composeReply (§T61). All
// answers are author-addressed and derived from env only (§V34).
func composeQueryReply(msg, from string, env queryEnv) (string, bool) {
	if from == "" {
		return "", false
	}

	// "what OS are you on?"
	if containsAny(msg, "what os", "which os", "what system", "operating system", "betriebssystem") ||
		(findWord(msg, "os") && hasQuestionMark(msg)) {
		return fmt.Sprintf("%s %s", from, env.goos), true
	}

	// "where are you?"
	if containsAny(msg, "where are you", "where r u", "where ru", "where u at", "wo bist du", "où es", "ou es tu") {
		if env.haveCoords {
			return fmt.Sprintf("%s x:%d y:%d", from, env.coordX, env.coordY), true
		}
		return from + " spectating", true
	}

	// "why do you kill me?" → our war standing toward the asker (check war self).
	if isQuestionWhy(msg) && findAnyWord(msg, "kill", "killed", "kills", "killst", "tötest") {
		if env.warlist != nil && env.warlist.Relation(from) == "war" {
			if reason := env.warlist.Reason(from); reason != "" {
				return fmt.Sprintf("%s because: %s", from, reason), true
			}
			return from + " you are on my war list", true
		}
		return from + " sorry, didn't mean to", true
	}

	// "list your wars" / "who do you war?"
	if env.warlist != nil && (containsAny(msg, "list war", "your wars", "list your war", "who do you war", "show wars") ||
		(findWord(msg, "wars") && hasQuestionMark(msg))) {
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

	// "is X war?" — war status of another player named in the message.
	if env.warlist != nil && findAnyWord(msg, "war", "peace", "team", "enemy", "friend") {
		for _, name := range env.rosterNames {
			if name == "" || name == from {
				continue
			}
			if findWord(msg, name) {
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

	// "can i join your clan?"
	if findWord(msg, "clan") && containsAny(msg, "join", "enter", "let me", "can i", "how do", "beitreten") {
		if env.selfClan != "" {
			return fmt.Sprintf("%s ask a member of [%s]", from, env.selfClan), true
		}
		return from + " i have no clan", true
	}

	return "", false
}

// capList returns up to n items of s (for bounded chat lines).
func capList(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
