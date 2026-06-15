package tui

import "strings"

// chatCmdResult is the outcome of parsing a chat line for a warlist command
// (← chillerbot chatcommand.cpp / warlist_commands_simple.cpp /
// warlist_commands_advanced.cpp).
type chatCmdResult struct {
	Handled bool     // line was a recognized "!" command
	Reply   []string // lines to echo to the local log
}

// parseChatCommand interprets a chat line as a warlist command and applies it to
// w. Recognized (§T22/§T24):
//
//	!war a b c               war one or more names at once (no reason)
//	!reason <name> <text...> attach a war reason to a name
//	!peace/!team <name...>   relation for one or more names
//	!del <name...>           clear relation(s)
//	!warclan/!peaceclan/!teamclan/!delclan <clantag>   per-clan relation
//	!help                    usage
//
// The advanced warlist's "war reason" is set with the dedicated !reason command:
// !war is multi-name, so a trailing reason on !war would be ambiguous with a
// list of names — the two are split into separate verbs (§T24).
//
// Returns Handled=false for ordinary chat. With cl_silent_chat_commands the app
// suppresses sending a handled command to the server (§V14).
func parseChatCommand(line string, w *Warlist) chatCmdResult {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "!") {
		return chatCmdResult{}
	}
	cmd, arg, _ := strings.Cut(line[1:], " ")
	arg = strings.TrimSpace(arg)

	switch strings.ToLower(cmd) {
	case "warclan":
		return setClanRel(w, arg, RelWar)
	case "peaceclan":
		return setClanRel(w, arg, RelPeace)
	case "teamclan":
		return setClanRel(w, arg, RelTeam)
	case "delclan", "unwarclan":
		if arg == "" {
			return chatCmdResult{Handled: true, Reply: []string{"usage: !delclan <clan>"}}
		}
		w.SetClan(arg, RelNeutral)
		return chatCmdResult{Handled: true, Reply: []string{"cleared clan " + arg}}
	case "war":
		return setRelNames(w, arg, RelWar)
	case "reason", "addreason":
		return setReason(w, arg)
	case "peace":
		return setRelNames(w, arg, RelPeace)
	case "team":
		return setRelNames(w, arg, RelTeam)
	case "delteam", "del", "delwar", "unwar", "unfriend":
		if arg == "" {
			return chatCmdResult{Handled: true, Reply: []string{"usage: !del <name>"}}
		}
		var out []string
		for _, n := range strings.Fields(arg) {
			w.Del(n)
			out = append(out, "cleared "+n)
		}
		return chatCmdResult{Handled: true, Reply: out}
	case "search":
		if arg == "" {
			return chatCmdResult{Handled: true, Reply: []string{"usage: !search <name>"}}
		}
		matches := w.Search(arg)
		if len(matches) == 0 {
			return chatCmdResult{Handled: true, Reply: []string{"no warlist matches for " + arg}}
		}
		return chatCmdResult{Handled: true, Reply: matches}
	case "create":
		return createRel(w, arg)
	case "help":
		return chatCmdResult{Handled: true, Reply: []string{
			"warlist: !war <name...>  !peace <name>  !team <name>  !del/!unfriend <name>",
			"reason:  !reason/!addreason <name> <text>   search: !search <name>",
			"create:  !create <war|team|neutral|traitor> <name>   clan: !warclan/!peaceclan/!teamclan/!delclan <tag>",
		}}
	default:
		return chatCmdResult{} // unknown ! line → treat as normal chat
	}
}

// createRel implements "!create <war|team|neutral|traitor> <name...>" (←
// chillerbot chatcommands.h create). teetui's warlist is flat, so an optional
// chillerbot [folder] is not used; "traitor" maps to war (teetui has no separate
// traitor relation). neutral clears the relation.
func createRel(w *Warlist, arg string) chatCmdResult {
	kind, name, ok := strings.Cut(arg, " ")
	name = strings.TrimSpace(name)
	if !ok || name == "" {
		return chatCmdResult{Handled: true, Reply: []string{"usage: !create <war|team|neutral|traitor> <name>"}}
	}
	var rel Relation
	switch strings.ToLower(kind) {
	case "war", "traitor":
		rel = RelWar
	case "team":
		rel = RelTeam
	case "peace":
		rel = RelPeace
	case "neutral":
		rel = RelNeutral
	default:
		return chatCmdResult{Handled: true, Reply: []string{"unknown type " + kind + " (war|team|neutral|traitor)"}}
	}
	w.Set(name, rel)
	return chatCmdResult{Handled: true, Reply: []string{"create " + relationName(rel) + " " + name}}
}

// setReason attaches a war reason to a name: "!reason <name> <text...>".
func setReason(w *Warlist, arg string) chatCmdResult {
	name, reason, ok := strings.Cut(arg, " ")
	reason = strings.TrimSpace(reason)
	if !ok || name == "" || reason == "" {
		return chatCmdResult{Handled: true, Reply: []string{"usage: !reason <name> <text>"}}
	}
	if w.Get(name) == RelNeutral {
		w.Set(name, RelWar) // a reason implies a war
	}
	w.SetReason(name, reason)
	return chatCmdResult{Handled: true, Reply: []string{"reason " + name + ": " + reason}}
}

// setRelNames wars/peaces/teams one or more space-separated names.
func setRelNames(w *Warlist, arg string, r Relation) chatCmdResult {
	if arg == "" {
		return chatCmdResult{Handled: true, Reply: []string{"usage: !" + relationName(r) + " <name>"}}
	}
	var out []string
	for _, n := range strings.Fields(arg) {
		w.Set(n, r)
		out = append(out, relationName(r)+": "+n)
	}
	return chatCmdResult{Handled: true, Reply: out}
}

// setClanRel assigns a relation to a whole clan tag (§T24).
func setClanRel(w *Warlist, clan string, r Relation) chatCmdResult {
	if clan == "" {
		return chatCmdResult{Handled: true, Reply: []string{"usage: !" + relationName(r) + "clan <clan>"}}
	}
	w.SetClan(clan, r)
	return chatCmdResult{Handled: true, Reply: []string{relationName(r) + " clan: " + clan}}
}
