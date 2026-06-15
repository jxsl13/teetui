package tui

import (
	"sort"
	"strings"
)

// consoleCommands is the completion candidate set for the local console (§T15).
var consoleCommands = []string{"echo", "exit", "help", "quit", "say", "version"}

// currentWord returns the start index and the whitespace-delimited word ending
// at the cursor (the token completion operates on).
func currentWord(runes []rune, cur int) (int, string) {
	if cur > len(runes) {
		cur = len(runes)
	}
	start := cur
	for start > 0 && runes[start-1] != ' ' {
		start--
	}
	return start, string(runes[start:cur])
}

// completionPreview returns the grey "ghost" suffix and a candidate-list hint
// for the current word prefix (§T15). A single match yields the completable
// suffix as a ghost; several matches yield a `{a b c}` list (no ghost); no match
// yields nothing. Pure, so it is unit-tested.
func completionPreview(prefix string, candidates []string) (ghost, list string) {
	if prefix == "" {
		return "", ""
	}
	m := completeMatches(prefix, candidates)
	switch {
	case len(m) == 0:
		return "", ""
	case len(m) == 1:
		r, p := []rune(m[0]), []rune(prefix)
		if len(r) > len(p) {
			ghost = string(r[len(p):])
		}
		return ghost, ""
	default:
		n := len(m)
		if n > 6 {
			n = 6
		}
		return "", " {" + strings.Join(m[:n], " ") + "}"
	}
}

// completeMatches returns the candidates that start with prefix
// (case-insensitive), de-duplicated and sorted. An empty prefix matches all.
func completeMatches(prefix string, candidates []string) []string {
	low := strings.ToLower(prefix)
	seen := map[string]bool{}
	var out []string
	for _, c := range candidates {
		if c == "" || seen[c] {
			continue
		}
		if strings.HasPrefix(strings.ToLower(c), low) {
			seen[c] = true
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}
