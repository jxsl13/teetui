package warlist

import "strings"

// cmdResult is the outcome of parsing a chat line for a warlist command.
type cmdResult struct {
	Handled bool
	Reply   []string
}

// parseCommand interprets a chat line as a warlist command and applies it to w
// (← chillerbot chatcommand / warlist_commands_*, §T22/§T24/§T67). Handled=false
// for ordinary chat.
func parseCommand(line string, w *Store) cmdResult {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "!") {
		return cmdResult{}
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
			return cmdResult{true, []string{"usage: !delclan <clan>"}}
		}
		w.SetClan(arg, RelNeutral)
		return cmdResult{true, []string{"cleared clan " + arg}}
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
			return cmdResult{true, []string{"usage: !del <name>"}}
		}
		var out []string
		for _, n := range strings.Fields(arg) {
			w.Del(n)
			out = append(out, "cleared "+n)
		}
		return cmdResult{true, out}
	case "search":
		if arg == "" {
			return cmdResult{true, []string{"usage: !search <name>"}}
		}
		matches := w.Search(arg)
		if len(matches) == 0 {
			return cmdResult{true, []string{"no warlist matches for " + arg}}
		}
		return cmdResult{true, matches}
	case "create":
		return createRel(w, arg)
	case "help":
		return cmdResult{true, []string{
			"warlist: !war <name...>  !peace <name>  !team <name>  !del/!unfriend <name>",
			"reason:  !reason/!addreason <name> <text>   search: !search <name>",
			"create:  !create <war|team|neutral|traitor> <name>   clan: !warclan/!peaceclan/!teamclan/!delclan <tag>",
		}}
	default:
		return cmdResult{}
	}
}

func createRel(w *Store, arg string) cmdResult {
	kind, name, ok := strings.Cut(arg, " ")
	name = strings.TrimSpace(name)
	if !ok || name == "" {
		return cmdResult{true, []string{"usage: !create <war|team|neutral|traitor> <name>"}}
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
		return cmdResult{true, []string{"unknown type " + kind + " (war|team|neutral|traitor)"}}
	}
	w.Set(name, rel)
	return cmdResult{true, []string{"create " + relationName(rel) + " " + name}}
}

func setReason(w *Store, arg string) cmdResult {
	name, reason, ok := strings.Cut(arg, " ")
	reason = strings.TrimSpace(reason)
	if !ok || name == "" || reason == "" {
		return cmdResult{true, []string{"usage: !reason <name> <text>"}}
	}
	if w.Get(name) == RelNeutral {
		w.Set(name, RelWar)
	}
	w.SetReason(name, reason)
	return cmdResult{true, []string{"reason " + name + ": " + reason}}
}

func setRelNames(w *Store, arg string, r Relation) cmdResult {
	if arg == "" {
		return cmdResult{true, []string{"usage: !" + relationName(r) + " <name>"}}
	}
	var out []string
	for _, n := range strings.Fields(arg) {
		w.Set(n, r)
		out = append(out, relationName(r)+": "+n)
	}
	return cmdResult{true, out}
}

func setClanRel(w *Store, clan string, r Relation) cmdResult {
	if clan == "" {
		return cmdResult{true, []string{"usage: !" + relationName(r) + "clan <clan>"}}
	}
	w.SetClan(clan, r)
	return cmdResult{true, []string{relationName(r) + " clan: " + clan}}
}
