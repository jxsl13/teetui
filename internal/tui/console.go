package tui

import (
	"fmt"
	"strings"
)

// conResult is the outcome of a local-console command (§T39). Out lines go to
// the log; Quit asks the app to exit; Chat, when set, is sent as a chat line;
// Spectate requests spectating SpecName ("" = free view) (§T37).
type conResult struct {
	Out      []string
	Quit     bool
	Chat     string
	Spectate bool
	SpecName string
}

// runConsole parses and dispatches a local-console command line. It is pure
// (no client/screen access) so it is unit-tested; side effects (quit, chat) are
// returned for the app to apply. Mirrors the F1 local console of chillerbot.
func runConsole(line string) conResult {
	line = strings.TrimSpace(line)
	if line == "" {
		return conResult{}
	}
	cmd, rest, _ := strings.Cut(line, " ")
	rest = strings.TrimSpace(rest)
	switch cmd {
	case "help", "?":
		return conResult{Out: []string{
			"commands: help, echo <text>, say <msg>, spec [name], quit, version",
		}}
	case "echo":
		return conResult{Out: []string{rest}}
	case "say":
		if rest == "" {
			return conResult{Out: []string{"usage: say <message>"}}
		}
		return conResult{Chat: rest}
	case "version":
		return conResult{Out: []string{"teetui (twclient v0.2.2)"}}
	case "spec", "spectate", "pause":
		return conResult{Spectate: true, SpecName: rest} // rest "" → free view
	case "quit", "exit":
		return conResult{Quit: true}
	default:
		return conResult{Out: []string{fmt.Sprintf("unknown command: %q (try 'help')", cmd)}}
	}
}
