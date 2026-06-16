// Package chatfilter hides spam/insult/user-filtered incoming chat (← chillerbot
// chathelper FilterChat/IsSpam, §T81/§T64/§V36). Self-registering feature:
// OnChat suppresses matching lines; cl_chat_spam_filter (0=off 1=hide
// 2=hide+autoreply) + cl_chat_spam_filter_insults gate it; addfilter/listfilter/
// delfilter console commands manage the user substring list. Off by default.
package chatfilter

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jxsl13/teetui/feature"
	"github.com/jxsl13/teetui/internal/lang"
)

type chatFilter struct {
	mu      sync.Mutex
	filters []string
	lastRep time.Time
}

func (*chatFilter) Name() string { return "chatfilter" }

func (f *chatFilter) Init(h feature.API) error {
	h.DefineConfig("cl_chat_spam_filter", "0", "hide spam pings (0=off 1=hide 2=hide+autoreply)")
	h.DefineConfig("cl_chat_spam_filter_insults", "0", "also hide insults when cl_chat_spam_filter>0 (0/1)")
	h.DefineCommand("addfilter", "addfilter <text> — hide chat containing text", func(args string) []string {
		if args == "" {
			return []string{"usage: addfilter <text>"}
		}
		if f.add(args) {
			return []string{"chat filter added: " + args}
		}
		return []string{"filter already present"}
	})
	h.DefineCommand("delfilter", "delfilter <text> — remove a chat filter", func(args string) []string {
		if f.del(args) {
			return []string{"chat filter removed: " + args}
		}
		return []string{"no such filter"}
	})
	h.DefineCommand("listfilter", "listfilter — list chat filters", func(string) []string {
		f.mu.Lock()
		defer f.mu.Unlock()
		if len(f.filters) == 0 {
			return []string{"no chat filters"}
		}
		return []string{"filters: " + strings.Join(f.filters, ", ")}
	})
	return nil
}

// OnChat hides a line per the filter config; mode 2 also fires a rate-limited
// canned reply. Returning true suppresses the line (§V36).
func (f *chatFilter) OnChat(h feature.API, e feature.ChatEvent) bool {
	modeStr, _ := h.Config("cl_chat_spam_filter")
	mode, _ := strconv.Atoi(strings.TrimSpace(modeStr))
	if mode == 0 {
		return false
	}
	insultsStr, _ := h.Config("cl_chat_spam_filter_insults")
	insults := insultsStr == "1" || insultsStr == "true" || insultsStr == "on"

	if !f.matches(e.Msg) && !(insults && lang.IsInsult(e.Msg)) {
		return false
	}
	if mode == 2 && e.Name != "" {
		f.maybeReply(h, e.Name)
	}
	return true // hide
}

func (f *chatFilter) matches(msg string) bool {
	low := strings.ToLower(msg)
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.filters {
		if s != "" && strings.Contains(low, strings.ToLower(s)) {
			return true
		}
	}
	return false
}

// maybeReply sends a rate-limited canned reply to a filtered spammer (mode 2).
func (f *chatFilter) maybeReply(h feature.API, from string) {
	f.mu.Lock()
	if time.Since(f.lastRep) < 30*time.Second {
		f.mu.Unlock()
		return
	}
	f.lastRep = time.Now()
	f.mu.Unlock()
	h.SendChat(from+" stop", false)
}

func (f *chatFilter) add(s string) bool {
	s = strings.TrimSpace(s)
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.filters {
		if strings.EqualFold(e, s) {
			return false
		}
	}
	f.filters = append(f.filters, s)
	return true
}

func (f *chatFilter) del(s string) bool {
	s = strings.TrimSpace(s)
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, e := range f.filters {
		if strings.EqualFold(e, s) {
			f.filters = append(f.filters[:i], f.filters[i+1:]...)
			return true
		}
	}
	return false
}

func init() { feature.Register(&chatFilter{}) }
