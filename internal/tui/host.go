package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/teetui/feature"
	"github.com/jxsl13/twclient/client"
)

// dynVar is a feature-defined config variable (§T76). Features declare these at
// Provision via Host.DefineConfig; they live alongside the static core cvars and
// are get/set from the console exactly the same way.
type dynVar struct {
	value string
	help  string
}

// appHost adapts *App to feature.Host — the capability surface handed to feature
// modules (§T76/§I.feature). It exposes only teetui's safe twclient-backed
// actions plus the registration sinks (config/action/status/name-style/service/
// outgoing-chat). No raw network access (§V39/§V47).
type appHost struct{ a *App }

// host returns the feature Host for this app.
func (a *App) host() feature.Host { return appHost{a} }

// provisionFeatures provisions every registered feature against this app's Host
// (§T76). Called once at startup; a feature that errors/panics is disabled by the
// registry (§V47). No-op when no feature package is imported.
func (a *App) provisionFeatures() {
	for _, err := range feature.ProvisionAll(a.host()) {
		a.log.Addf(StyleSelf, "feature disabled: %v", err)
	}
}

func (h appHost) SendChat(msg string, team bool) { h.a.sendChat(msg, team) }

func (h appHost) Do(act client.Action) error {
	if c := h.a.cli.Load(); c != nil {
		return c.Do(act)
	}
	return nil
}

func (h appHost) Log(msg string) { h.a.log.Addf(StyleSystem, "%s", msg) }

func (h appHost) Roster() []client.PlayerState {
	if c := h.a.cli.Load(); c != nil {
		return c.Roster()
	}
	return nil
}

func (h appHost) Tick() (client.TickState, bool) { return h.a.state.Get() }
func (h appHost) PlayerName() string             { return h.a.playerName }
func (h appHost) PlayerClan() string             { return h.a.playerClan }
func (h appHost) Server() string                 { return h.a.server }

// DefineConfig registers a feature cvar (idempotent: keeps the existing value on
// re-define so a reload doesn't clobber a user change).
func (h appHost) DefineConfig(name, def, help string) {
	h.a.cfgMu.Lock()
	if _, ok := h.a.dynVars[name]; !ok {
		h.a.dynVars[name] = &dynVar{value: def, help: help}
	}
	h.a.cfgMu.Unlock()
}

// Config returns a config value, checking feature cvars then the static core
// cvars (§T76).
func (h appHost) Config(name string) (string, bool) {
	h.a.cfgMu.Lock()
	if v, ok := h.a.dynVars[name]; ok {
		val := v.value
		h.a.cfgMu.Unlock()
		return val, true
	}
	h.a.cfgMu.Unlock()
	if cv := findCvar(name); cv != nil {
		cs := h.a.cfgSnap()
		return cv.get(&cs), true
	}
	return "", false
}

// OnSendChat appends an outgoing-chat transform to the chain (§T76): each fn may
// edit the message or cancel the send (return send=false).
func (h appHost) OnSendChat(fn func(msg string, team bool) (string, bool)) {
	h.a.sendChatHook = append(h.a.sendChatHook, fn)
}

// DefineAction binds a feature action to its default key (§T76/§V46). The key
// token is parsed via the keymap grammar (a rune or a named key like "F3").
func (h appHost) DefineAction(name, defaultKey, help string, run func()) {
	if run == nil || defaultKey == "" {
		return
	}
	if isRune, r, key, ok := tokenToBind(defaultKey); ok {
		if isRune {
			h.a.featActRune[r] = run
		} else {
			h.a.featActKey[key] = run
		}
	}
}

// AddStatusField registers a status-bar contribution (§T76).
func (h appHost) AddStatusField(fn func() string) {
	if fn != nil {
		h.a.statusFields = append(h.a.statusFields, fn)
	}
}

// AddNameStyle registers a per-name styler used by the scoreboard (§T76).
func (h appHost) AddNameStyle(fn func(name, clan string) (feature.Style, bool)) {
	if fn != nil {
		h.a.nameStylers = append(h.a.nameStylers, fn)
	}
}

// Provide registers a cross-feature service (§T76, ← caddy ctx.App).
func (h appHost) Provide(name string, svc any) { h.a.services[name] = svc }

// Lookup fetches a service registered by another feature.
func (h appHost) Lookup(name string) (any, bool) { v, ok := h.a.services[name]; return v, ok }

// featureKey maps a tcell key event to feature.Key for OnKey dispatch (§T76).
func featureKey(ev *tcell.EventKey) feature.Key {
	if r := ev.Rune(); r != 0 && ev.Key() == tcell.KeyRune {
		return feature.Key{Rune: r}
	}
	if name, ok := hookKeyNames[ev.Key()]; ok {
		return feature.Key{Name: name}
	}
	return feature.Key{Name: "key"}
}

// runFeatureAction runs a feature action bound to this key (§T76); reports
// whether one fired.
func (a *App) runFeatureAction(ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyRune {
		if fn, ok := a.featActRune[ev.Rune()]; ok {
			fn()
			return true
		}
		return false
	}
	if fn, ok := a.featActKey[ev.Key()]; ok {
		fn()
		return true
	}
	return false
}

// tryDynVar handles a console line that targets a feature-defined cvar (§T76):
// a bare name prints its value, "name value" sets it. Returns (output, true) if
// the line addressed a feature cvar, else (nil, false) so the static console
// handles it.
func (a *App) tryDynVar(line string) ([]string, bool) {
	cmd, rest, _ := strings.Cut(strings.TrimSpace(line), " ")
	rest = strings.TrimSpace(rest)
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	v, ok := a.dynVars[cmd]
	if !ok {
		return nil, false
	}
	if rest == "" {
		return []string{fmt.Sprintf("%s = %q", cmd, v.value)}, true
	}
	v.value = rest
	return []string{fmt.Sprintf("%s set to %q", cmd, v.value)}, true
}

// nameStyle returns the first feature-contributed style for a name/clan (§T76).
func (a *App) nameStyle(name, clan string) (tcell.Style, bool) {
	for _, fn := range a.nameStylers {
		if st, ok := fn(name, clan); ok {
			return st, true
		}
	}
	return tcell.StyleDefault, false
}

// runSendChatHooks applies the outgoing-chat chain (§T76); returns the possibly
// edited message and whether to actually send it.
func (a *App) runSendChatHooks(msg string, team bool) (string, bool) {
	for _, fn := range a.sendChatHook {
		var send bool
		msg, send = fn(msg, team)
		if !send {
			return msg, false
		}
	}
	return msg, true
}
