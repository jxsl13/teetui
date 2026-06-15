package tui

import (
	"sort"
	"strings"
)

// consoleCommands is the completion candidate set for the local console (§T15).
var consoleCommands = []string{"echo", "exit", "help", "quit", "say", "version"}

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
