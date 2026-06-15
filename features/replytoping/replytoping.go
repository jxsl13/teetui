// Package replytoping is the H reply-to-ping feature (← chillerbot
// chathelper/replytoping + check_war/list_wars/where, §T79/§T80/§T23/§V33/§V34).
// Self-registering module: binds an action to H that answers the most recent
// ping — first a state-derived query answer (war status, where, OS, …, via the
// "warlist" service + roster/coords), then a canned context reply, then a
// friendly default. Reads pings from the "pings" service (lastping).
package replytoping

import (
	"runtime"

	"github.com/jxsl13/teetui/feature"
)

// tileSize mirrors the render tile size for the "where are you" tile coords.
const tileSize = 32

type replyFeature struct{ feature.NopFeature }

func (*replyFeature) Name() string { return "replytoping" }

func (f *replyFeature) Provision(h feature.Host) error {
	h.DefineAction("reply_to_ping", "H", "reply to the last chat ping", func() { f.reply(h) })
	return nil
}

// reply answers the next pending ping (newest first; repeated H walks older).
func (f *replyFeature) reply(h feature.Host) {
	store, _ := lookupPings(h)
	if store == nil {
		h.Log("no recent ping to reply to")
		return
	}
	from, msg, ok := store.NextReply()
	if !ok {
		h.Log("no recent ping to reply to")
		return
	}
	if r, ok := composeQueryReply(msg, from, f.env(h)); ok {
		h.SendChat(r, false)
		return
	}
	r, ok := composeReply(msg, from, h.PlayerName())
	if !ok {
		r = from + " hi"
	}
	h.SendChat(r, false)
}

// env gathers the read-only state for a query answer from the Host.
func (f *replyFeature) env(h feature.Host) queryEnv {
	env := queryEnv{selfClan: h.PlayerClan(), goos: runtime.GOOS}
	if wl, ok := h.Lookup("warlist"); ok {
		if w, ok := wl.(feature.Warlist); ok {
			env.warlist = w
		}
	}
	for _, p := range h.Roster() {
		if p.Name != "" {
			env.rosterNames = append(env.rosterNames, p.Name)
		}
	}
	if st, ok := h.Tick(); ok {
		if p, has := st.Players[st.LocalID]; has {
			env.haveCoords, env.coordX, env.coordY = true, p.X/tileSize, p.Y/tileSize
		}
	}
	return env
}

// lookupPings fetches the pings service (lastping), if present.
func lookupPings(h feature.Host) (feature.PingStore, bool) {
	if v, ok := h.Lookup("pings"); ok {
		s, ok := v.(feature.PingStore)
		return s, ok
	}
	return nil, false
}

func init() { feature.Register(&replyFeature{}) }
