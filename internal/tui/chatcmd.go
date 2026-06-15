package tui

import "strings"

// chatCmdResult is the outcome of parsing a chat line for a warlist command
// (← chillerbot chatcommand.cpp / warlist_commands_simple.cpp).
type chatCmdResult struct {
	Handled bool     // line was a recognized "!" command
	Reply   []string // lines to echo to the local log
}

// parseChatCommand interprets a chat line as a warlist command and applies it to
// w. Recognized: !war/!peace/!team/!delteam/!del/!help. Returns Handled=false
// for ordinary chat. With cl_silent_chat_commands the app suppresses sending a
// handled command to the server (§V14).
func parseChatCommand(line string, w *Warlist) chatCmdResult {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "!") {
		return chatCmdResult{}
	}
	cmd, arg, _ := strings.Cut(line[1:], " ")
	arg = strings.TrimSpace(arg)

	switch strings.ToLower(cmd) {
	case "war":
		return setRel(w, arg, RelWar)
	case "peace":
		return setRel(w, arg, RelPeace)
	case "team":
		return setRel(w, arg, RelTeam)
	case "delteam", "del", "delwar", "unwar":
		if arg == "" {
			return chatCmdResult{Handled: true, Reply: []string{"usage: !del <name>"}}
		}
		w.Del(arg)
		return chatCmdResult{Handled: true, Reply: []string{"cleared " + arg}}
	case "help":
		return chatCmdResult{Handled: true, Reply: []string{
			"warlist: !war <name>  !peace <name>  !team <name>  !del <name>",
		}}
	default:
		return chatCmdResult{} // unknown ! line → treat as normal chat
	}
}

func setRel(w *Warlist, name string, r Relation) chatCmdResult {
	if name == "" {
		return chatCmdResult{Handled: true, Reply: []string{"usage: !" + relationName(r) + " <name>"}}
	}
	w.Set(name, r)
	return chatCmdResult{Handled: true, Reply: []string{relationName(r) + ": " + name}}
}
