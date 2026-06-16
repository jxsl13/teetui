// Package responders auto-answers chat pings (← chillerbot tapped-out message +
// cl_auto_reply, §T82/§T40/§T61/§V33). Self-registering feature: OnChat detects
// a ping (our name) and, when enabled, sends a rate-limited reply. Off by
// default — teetui is interactive, not a headless bot.
//
//	cl_tapped_out_message / _text  fixed AFK reply when pinged
//	cl_auto_reply / cl_auto_reply_msg   templated reply (%n = pinger) on every ping
package responders

import (
	"strings"
	"sync"
	"time"

	"github.com/jxsl13/teetui/feature"
	"github.com/jxsl13/teetui/internal/lang"
)

// interval rate-limits each auto-reply so a ping burst can't flood (§T40).
const interval = 30 * time.Second

type responders struct {
	mu         sync.Mutex
	lastTapped time.Time
	lastAuto   time.Time
}

func (*responders) Name() string { return "responders" }

func (r *responders) Init(h feature.API) error {
	h.DefineConfig("cl_tapped_out_message", "0", "auto-reply with the tapped-out message when pinged (0/1)")
	h.DefineConfig("cl_tapped_out_message_text", "I'm currently tapped out (afk)", "the tapped-out reply text")
	h.DefineConfig("cl_auto_reply", "0", "auto-reply with cl_auto_reply_msg on every ping (0/1)")
	h.DefineConfig("cl_auto_reply_msg", "%n (teetui auto reply)", "auto-reply template; %n = the pinger")
	return nil
}

// OnChat fires the auto-responders when a ping arrives.
func (r *responders) OnChat(h feature.API, e feature.ChatEvent) bool {
	me := h.PlayerName()
	if e.Name == me || !lang.ContainsName(e.Msg, me) {
		return false // not a ping (or our own line)
	}
	if on, _ := h.Config("cl_tapped_out_message"); truthy(on) {
		if txt, _ := h.Config("cl_tapped_out_message_text"); txt != "" && r.allow(&r.lastTapped) {
			h.SendChat(txt, false)
		}
	}
	if on, _ := h.Config("cl_auto_reply"); truthy(on) && e.Name != "" {
		if tmpl, _ := h.Config("cl_auto_reply_msg"); tmpl != "" && r.allow(&r.lastAuto) {
			h.SendChat(expandAutoReply(tmpl, e.Name), false)
		}
	}
	return false // never suppress the original line
}

// allow rate-limits one responder, updating its last-fire time on success.
func (r *responders) allow(last *time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if time.Since(*last) < interval {
		return false
	}
	*last = time.Now()
	return true
}

// expandAutoReply renders the template: %n → pinger name (← cl_auto_reply_msg).
func expandAutoReply(tmpl, from string) string { return strings.ReplaceAll(tmpl, "%n", from) }

func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "on", "yes":
		return true
	}
	return false
}

func init() { feature.Register(&responders{}) }
