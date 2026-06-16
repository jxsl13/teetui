package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/teetui/feature"
	"github.com/jxsl13/twclient/client"
)

// dynVar is a feature-defined config variable (§T76). Features declare these at
// Init via API.DefineConfig; they live alongside the static core cvars and
// are get/set from the console exactly the same way.
type dynVar struct {
	value string
	help  string
}

// featCmd is a feature-defined F1 console command (§T92): a help line + a
// handler taking the raw argument string and returning output lines.
type featCmd struct {
	help string
	run  func(args string) []string
}

// featAction records a feature-defined NORMAL-mode action's metadata so the
// generated legend + help overlay can list it (§T95/§T96/§V55/§V56). The bound
// key is the default token the feature registered (feature actions are not
// routed through the rebindable Keymap, unlike core actions).
type featAction struct {
	name string
	key  string
	help string
}

// appAPI adapts *App to feature.API — the capability surface handed to feature
// modules (§T76/§I.feature). It exposes only teetui's safe twclient-backed
// actions plus the registration sinks (config/action/status/name-style/service/
// outgoing-chat). No raw network access (§V39/§V47).
type appAPI struct{ a *App }

// api returns the feature.API capability surface for this app.
func (a *App) api() feature.API { return appAPI{a} }

// provisionFeatures provisions every registered feature against this app's Host
// (§T76). Called once at startup; a feature that errors/panics is disabled by the
// registry (§V47). No-op when no feature package is imported.
func (a *App) provisionFeatures() {
	for _, err := range feature.InitAll(a.api()) {
		a.log.Addf(StyleSelf, "feature disabled: %v", err)
	}
}

func (h appAPI) SendChat(msg string, team bool) { h.a.sendChat(msg, team) }

func (h appAPI) Do(act client.Action) error {
	if c := h.a.cur().cli.Load(); c != nil {
		return c.Do(act)
	}
	return nil
}

func (h appAPI) Log(msg string) { h.a.log.Addf(StyleSystem, "%s", msg) }

// RconLogin authenticates rcon off the event loop and logs the outcome (§T84);
// the password is never logged. Async so a feature's OnConnect doesn't block.
func (h appAPI) RconLogin(password string) {
	c := h.a.cur().cli.Load()
	if c == nil || password == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := c.RconLogin(ctx, password); err != nil {
			h.a.log.Addf(StyleSelf, "rcon login failed")
		} else {
			h.a.log.Addf(StyleSystem, "rcon authenticated")
		}
		h.a.wake()
	}()
}

// DataPath returns an absolute path under the teetui config dir (§T84), or "".
func (h appAPI) DataPath(name string) string {
	if dir, err := configDir(); err == nil {
		return filepath.Join(dir, name)
	}
	return ""
}

func (h appAPI) Roster() []client.PlayerState {
	if c := h.a.cur().cli.Load(); c != nil {
		return c.Roster()
	}
	return nil
}

func (h appAPI) Tick() (client.TickState, bool) { return h.a.cur().state.Get() }
func (h appAPI) PlayerName() string             { return h.a.playerName }
func (h appAPI) PlayerClan() string             { return h.a.playerClan }
func (h appAPI) Server() string                 { return h.a.cur().server }

// DefineConfig registers a feature cvar (idempotent: keeps the existing value on
// re-define so a reload doesn't clobber a user change).
func (h appAPI) DefineConfig(name, def, help string) {
	h.a.cfgMu.Lock()
	if _, ok := h.a.dynVars[name]; !ok {
		h.a.dynVars[name] = &dynVar{value: def, help: help}
	}
	h.a.cfgMu.Unlock()
}

// Config returns a config value, checking feature cvars then the static core
// cvars (§T76).
func (h appAPI) Config(name string) (string, bool) {
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

// AddSendChatFilter appends an outgoing-chat transform to the chain (§T76): each
// fn may edit the message or cancel the send (return send=false).
func (h appAPI) AddSendChatFilter(fn func(msg string, team bool) (string, bool)) {
	h.a.sendChatHook = append(h.a.sendChatHook, fn)
}

// DefineAction binds a feature action to its default key (§T76/§V46). The key
// token is parsed via the keymap grammar (a rune or a named key like "F3").
func (h appAPI) DefineAction(name, defaultKey, help string, run func()) {
	if run == nil || defaultKey == "" {
		return
	}
	if isRune, r, key, ok := tokenToBind(defaultKey); ok {
		if isRune {
			h.a.featActRune[r] = run
		} else {
			h.a.featActKey[key] = run
		}
		// record metadata for the generated legend + help (§T95/§T96).
		h.a.featActions = append(h.a.featActions, featAction{name: name, key: defaultKey, help: help})
	}
}

// DefineCommand registers an F1 console command for a feature (§T92). The
// handler receives the raw argument string and returns log lines.
func (h appAPI) DefineCommand(name, help string, run func(args string) []string) {
	if name == "" || run == nil {
		return
	}
	h.a.featCmds[name] = &featCmd{help: help, run: run}
}

// AddStatusField registers a status-bar contribution (§T76).
func (h appAPI) AddStatusField(fn func() string) {
	if fn != nil {
		h.a.statusFields = append(h.a.statusFields, fn)
	}
}

// AddNameStyle registers a per-name styler used by the scoreboard (§T76).
func (h appAPI) AddNameStyle(fn func(name, clan string) (feature.Style, bool)) {
	if fn != nil {
		h.a.nameStylers = append(h.a.nameStylers, fn)
	}
}

// Provide registers a cross-feature service (§T76/§V53).
func (h appAPI) Provide(name string, svc any) { h.a.services[name] = svc }

// Lookup fetches a service registered by another feature.
func (h appAPI) Lookup(name string) (any, bool) { v, ok := h.a.services[name]; return v, ok }

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

// tryFeatCmd dispatches a console line to a feature command if one matches its
// first token (§T92). Returns (output, true) when handled.
func (a *App) tryFeatCmd(line string) ([]string, bool) {
	cmd, args, _ := strings.Cut(strings.TrimSpace(line), " ")
	fc, ok := a.featCmds[cmd]
	if !ok {
		return nil, false
	}
	return fc.run(strings.TrimSpace(args)), true
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
