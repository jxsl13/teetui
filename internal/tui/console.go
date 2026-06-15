package tui

import (
	"fmt"
	"sort"
	"strings"
)

// conResult is the outcome of a local-console command (§T39). Out lines go to
// the log; Quit asks the app to exit; Chat, when set, is sent as a chat line;
// Spectate requests spectating SpecName ("" = free view) (§T37).
type conResult struct {
	Out         []string
	Quit        bool
	Chat        string
	Spectate    bool
	SpecName    string
	Connect     bool   // connect to ConnectAddr (§T89/§T91)
	ConnectAddr string // "host:port"
	ConnectVer  string // "0.6" | "0.7" | "" (default 0.6)
}

// builtinHelp is the per-command help text for the fixed (non-cvar) console
// commands (← chillerbot console help-text line, §T39).
var builtinHelp = map[string]string{
	"connect": "connect <host:port> [0.6|0.7] — connect to a server",
	"help":    "help [command] — list commands or show help for one",
	"echo":    "echo <text> — print text to the log",
	"say":     "say <message> — send a chat message",
	"spec":    "spec [name] — spectate a player (free view if no name)",
	"version": "version — show the client/library version",
	"quit":    "quit — exit teetui",
}

// consoleCommands is the completion candidate set for the local console (§T15):
// the built-in commands plus every config cvar, sorted.
var consoleCommands = func() []string {
	out := []string{"connect", "echo", "exit", "help", "quit", "say", "spec", "version"}
	for _, c := range cvars {
		out = append(out, c.name)
	}
	sort.Strings(out)
	return out
}()

// consoleHelp returns the one-line help text for the command currently being
// typed, or "" if it is unknown/empty. Used for the inline help-text line shown
// below the console prompt (§T39, ← chillerbot help-text line). Pure, tested.
func consoleHelp(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}
	if h, ok := builtinHelp[cmd]; ok {
		return h
	}
	if c := findCvar(cmd); c != nil {
		return cmd + " — " + c.help
	}
	return ""
}

// runConsole parses and dispatches a local-console command line against cfg. It
// is pure (no client/screen access) so it is unit-tested; side effects (quit,
// chat, cvar mutation on cfg) are returned/applied for the app. Mirrors the F1
// local console of chillerbot, including config cvars and per-command help.
func runConsole(line string, cfg *Config) conResult {
	line = strings.TrimSpace(line)
	if line == "" {
		return conResult{}
	}
	cmd, rest, _ := strings.Cut(line, " ")
	rest = strings.TrimSpace(rest)

	// Config cvar: bare name prints "name = value"; with an argument it sets it.
	if c := findCvar(cmd); c != nil {
		if rest == "" {
			return conResult{Out: []string{fmt.Sprintf("%s = %q", c.name, c.get(cfg))}}
		}
		c.set(cfg, rest)
		return conResult{Out: []string{fmt.Sprintf("%s set to %q", c.name, c.get(cfg))}}
	}

	switch cmd {
	case "help", "?":
		if rest != "" { // per-command help-text line
			if h := consoleHelp(rest); h != "" {
				return conResult{Out: []string{h}}
			}
			return conResult{Out: []string{fmt.Sprintf("no help for %q", rest)}}
		}
		return conResult{Out: []string{
			"commands: help [cmd], echo <text>, say <msg>, spec [name], quit, version",
			"config: " + strings.Join(cvarNames(), ", "),
		}}
	case "echo":
		return conResult{Out: []string{rest}}
	case "say":
		if rest == "" {
			return conResult{Out: []string{"usage: say <message>"}}
		}
		return conResult{Chat: rest}
	case "version":
		return conResult{Out: []string{"teetui (twclient v0.2.4)"}}
	case "connect":
		if rest == "" {
			return conResult{Out: []string{"usage: connect <host:port> [0.6|0.7]"}}
		}
		addr, ver, _ := strings.Cut(rest, " ")
		return conResult{Connect: true, ConnectAddr: addr, ConnectVer: strings.TrimSpace(ver)}
	case "spec", "spectate", "pause":
		return conResult{Spectate: true, SpecName: rest} // rest "" → free view
	case "quit", "exit":
		return conResult{Quit: true}
	default:
		return conResult{Out: []string{fmt.Sprintf("unknown command: %q (try 'help')", cmd)}}
	}
}

// cvarNames returns the registered cvar names in registry order.
func cvarNames() []string {
	out := make([]string, len(cvars))
	for i, c := range cvars {
		out[i] = c.name
	}
	return out
}
