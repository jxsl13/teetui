// Package warlist is the war/peace/team feature (← chillerbot warlist, §T78/
// §T21/§T22/§T24/§T66/§T67/§V14). Self-registering module: it owns the store,
// applies !commands on outgoing chat (AddSendChatFilter), tints the scoreboard
// (AddNameStyle), reloads the file when it changes on disk (OnTick), and Provides
// the "warlist" service for the chat-query feature.
package warlist

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jxsl13/teetui/feature"
	"github.com/jxsl13/twclient/client"
)

// fileMtime returns the modification time of path (zero time if unavailable).
func fileMtime(path string) time.Time {
	if fi, err := os.Stat(path); err == nil {
		return fi.ModTime()
	}
	return time.Time{}
}

type warlistFeature struct {
	store     *Store
	path      string
	mtime     time.Time
	lastCheck time.Time
}

func (*warlistFeature) Name() string { return "warlist" }

func (f *warlistFeature) Init(h feature.Host) error {
	h.DefineConfig("cl_silent_chat_commands", "1", "apply !war/!peace/… locally without sending to server (0/1)")
	h.DefineConfig("cl_war_list_auto_reload", "10", "reload the warlist file every N seconds (0=off)")

	f.path = h.DataPath("warlist.txt")
	if f.path != "" {
		_ = f.store.Load(f.path)
		f.mtime = fileMtime(f.path)
	}

	// Provide the read service for the chat-query feature.
	h.Provide("warlist", f.store)

	// Tint scoreboard names by relation.
	h.AddNameStyle(func(name, clan string) (feature.Style, bool) {
		return f.store.EffectiveStyle(name, clan)
	})

	// Intercept !commands on outgoing chat: apply locally; suppress the send when
	// cl_silent_chat_commands is on (§V14).
	h.AddSendChatFilter(func(msg string, team bool) (string, bool) {
		res := parseCommand(msg, f.store)
		if !res.Handled {
			return msg, true
		}
		for _, line := range res.Reply {
			h.Log("! " + line)
		}
		if f.path != "" {
			_ = f.store.Save(f.path) // persist immediately (no exit hook)
			f.mtime = fileMtime(f.path)
		}
		silent, _ := h.Config("cl_silent_chat_commands")
		if truthy(silent) {
			return msg, false // handled locally; do not send
		}
		return msg, true
	})
	return nil
}

// OnTick reloads the warlist when the file changed, every cl_war_list_auto_reload
// seconds (§T66). Cheap: a stat at most once per interval.
func (f *warlistFeature) OnTick(h feature.Host, _ client.TickState) { f.reloadCheck(h) }

func (f *warlistFeature) reloadCheck(h feature.Host) {
	if f.path == "" {
		return
	}
	ivStr, _ := h.Config("cl_war_list_auto_reload")
	iv, _ := strconv.Atoi(strings.TrimSpace(ivStr))
	if iv <= 0 {
		return
	}
	now := time.Now()
	if now.Sub(f.lastCheck) < time.Duration(iv)*time.Second {
		return
	}
	f.lastCheck = now
	if mt := fileMtime(f.path); mt.After(f.mtime) {
		if err := f.store.Load(f.path); err == nil {
			f.mtime = mt
			h.Log("warlist reloaded")
		}
	}
}

func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "on", "yes":
		return true
	}
	return false
}

func init() { feature.Register(&warlistFeature{store: newStore()}) }
