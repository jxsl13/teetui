package tui

import "strings"

// filterDecision decides whether an incoming chat line should be hidden and
// whether a spam auto-reply should fire (← chillerbot chathelper IsSpam /
// FilterChat, §T64/§V36). It is pure so it is unit-tested.
//
// Rules: own lines are never filtered. With cl_chat_spam_filter==0 nothing is
// filtered. Otherwise a line matching a user filter, or (when
// cl_chat_spam_filter_insults is on) classified as an insult, is hidden; mode 2
// additionally requests an auto-reply.
func filterDecision(msg string, isOwn bool, cfg *Config) (hide, autoReply bool) {
	if isOwn || cfg == nil || cfg.ChatSpamFilter == 0 {
		return false, false
	}
	match := matchesUserFilter(msg, cfg.Filters) || (cfg.FilterInsults && isInsult(msg))
	if !match {
		return false, false
	}
	return true, cfg.ChatSpamFilter == 2
}

// matchesUserFilter reports whether msg contains any user filter substring
// (case-insensitive). Empty filters never match.
func matchesUserFilter(msg string, filters []string) bool {
	low := strings.ToLower(msg)
	for _, f := range filters {
		if f != "" && strings.Contains(low, strings.ToLower(f)) {
			return true
		}
	}
	return false
}

// addFilter appends f to the filter list (delimited, de-duplicated). Returns the
// new list and whether it changed.
func addFilter(filters []string, f string) ([]string, bool) {
	f = strings.TrimSpace(f)
	if f == "" {
		return filters, false
	}
	for _, e := range filters {
		if strings.EqualFold(e, f) {
			return filters, false
		}
	}
	return append(filters, f), true
}

// delFilter removes f from the list (case-insensitive). Returns new list + found.
func delFilter(filters []string, f string) ([]string, bool) {
	f = strings.TrimSpace(f)
	out := filters[:0:0]
	found := false
	for _, e := range filters {
		if strings.EqualFold(e, f) {
			found = true
			continue
		}
		out = append(out, e)
	}
	return out, found
}
