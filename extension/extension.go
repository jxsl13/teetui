// Package extension is teetui's stable hook/callback API (§C19/§I.extension).
//
// teetui deliberately does NOT ship a number of chillerbot-ux features that are
// out of a terminal client's scope or ethics (graphical effects, skin stealing,
// telemetry, server stress tools — see SPEC §C18). Instead it exposes this hook
// surface so users can implement such behavior THEMSELVES, in-process, without
// patching the core.
//
// A Hook receives events (connect/disconnect/chat/broadcast/server-msg/kill/
// tick/key) and acts through a HookCtx, whose action surface is limited to
// teetui's existing twclient public API (send chat, do an action, read the
// roster/config). There is deliberately NO raw packet/network primitive here, so
// the API cannot be turned into a flood/DoS amplifier (§V39). User hooks run
// under the user's own responsibility.
//
// Register hooks at init/startup:
//
//	type myHook struct{ extension.NopHook }
//	func (myHook) OnChat(ctx extension.HookCtx, e extension.ChatEvent) bool {
//		if e.Msg == "!ping" { ctx.SendChat("pong", false) }
//		return false // don't suppress
//	}
//	func init() { extension.Register("my-hook", myHook{}) }
package extension

import (
	"sync"

	"github.com/jxsl13/twclient/client"
)

// ChatEvent is an incoming chat line handed to OnChat.
type ChatEvent struct {
	ClientID int
	Name     string // sender name ("" if unknown — id fallback applies in core)
	Msg      string
	Team     bool
}

// KillEvent is a kill feed entry handed to OnKill.
type KillEvent struct {
	Killer int
	Victim int
	Weapon int
}

// Key is a key press handed to OnKey. For ordinary character keys Rune is set;
// for named keys (F1, Enter, Tab, …) Name is set and Rune is 0.
type Key struct {
	Rune rune
	Name string
}

// HookCtx is the safe action surface a hook may use. It is exactly teetui's
// existing twclient-backed capability set (§V39) — no raw network access.
type HookCtx interface {
	SendChat(msg string, team bool) // queued through the spam-safe buffer (§T65)
	Do(action client.Action) error  // input/vote/emote/kill/spectate (§V12)
	Log(msg string)                 // write a line to the teetui log
	Roster() []client.PlayerState   // current players
	Config(name string) (string, bool)
	Server() string // current server address
}

// Hook is the set of events a user extension can handle. Embed NopHook to
// implement only the ones you need. OnChat returning true suppresses the line
// (hides it from the log); OnKey returning true consumes the key.
type Hook interface {
	OnConnect(ctx HookCtx)
	OnDisconnect(ctx HookCtx, reason string)
	OnChat(ctx HookCtx, e ChatEvent) (suppress bool)
	OnBroadcast(ctx HookCtx, text string)
	OnServerMsg(ctx HookCtx, text string)
	OnKill(ctx HookCtx, e KillEvent)
	OnTick(ctx HookCtx, st client.TickState)
	OnKey(ctx HookCtx, k Key) (handled bool)
}

// NopHook is a no-op Hook; embed it so a hook only overrides what it needs.
type NopHook struct{}

func (NopHook) OnConnect(HookCtx)                {}
func (NopHook) OnDisconnect(HookCtx, string)     {}
func (NopHook) OnChat(HookCtx, ChatEvent) bool   { return false }
func (NopHook) OnBroadcast(HookCtx, string)      {}
func (NopHook) OnServerMsg(HookCtx, string)      {}
func (NopHook) OnKill(HookCtx, KillEvent)        {}
func (NopHook) OnTick(HookCtx, client.TickState) {}
func (NopHook) OnKey(HookCtx, Key) bool          { return false }

type registered struct {
	name     string
	hook     Hook
	disabled bool
}

var (
	mu    sync.Mutex
	hooks []*registered
)

// Register adds a hook under name. Registration is process-global and meant to
// run at init/startup. Hooks are opt-in: none are present unless registered.
func Register(name string, h Hook) {
	if h == nil {
		return
	}
	mu.Lock()
	hooks = append(hooks, &registered{name: name, hook: h})
	mu.Unlock()
}

// Reset clears all registered hooks (test helper).
func Reset() {
	mu.Lock()
	hooks = nil
	mu.Unlock()
}

// Count returns the number of registered (including disabled) hooks.
func Count() int {
	mu.Lock()
	defer mu.Unlock()
	return len(hooks)
}

// snapshot returns the currently-enabled hooks (copy, so dispatch holds no lock
// while calling user code).
func snapshot() []*registered {
	mu.Lock()
	defer mu.Unlock()
	out := make([]*registered, 0, len(hooks))
	for _, r := range hooks {
		if !r.disabled {
			out = append(out, r)
		}
	}
	return out
}

// safeCall runs fn for one hook, recovering any panic: a misbehaving hook is
// disabled for the rest of the session and logged via ctx, never crashing teetui
// (§V40). It returns the bool fn produced (false on panic).
func safeCall(r *registered, ctx HookCtx, fn func() bool) (res bool) {
	defer func() {
		if rec := recover(); rec != nil {
			mu.Lock()
			r.disabled = true
			mu.Unlock()
			if ctx != nil {
				ctx.Log("hook " + r.name + " panicked and was disabled")
			}
			res = false
		}
	}()
	return fn()
}

// The Fire* functions dispatch one event to every enabled hook with panic
// isolation (§V40). Bool-returning events compose: OnChat suppress is OR across
// hooks; OnKey handled stops at the first hook that consumes it (§V39).

func FireConnect(ctx HookCtx) {
	for _, r := range snapshot() {
		safeCall(r, ctx, func() bool { r.hook.OnConnect(ctx); return false })
	}
}

func FireDisconnect(ctx HookCtx, reason string) {
	for _, r := range snapshot() {
		safeCall(r, ctx, func() bool { r.hook.OnDisconnect(ctx, reason); return false })
	}
}

func FireChat(ctx HookCtx, e ChatEvent) (suppress bool) {
	for _, r := range snapshot() {
		if safeCall(r, ctx, func() bool { return r.hook.OnChat(ctx, e) }) {
			suppress = true
		}
	}
	return suppress
}

func FireBroadcast(ctx HookCtx, text string) {
	for _, r := range snapshot() {
		safeCall(r, ctx, func() bool { r.hook.OnBroadcast(ctx, text); return false })
	}
}

func FireServerMsg(ctx HookCtx, text string) {
	for _, r := range snapshot() {
		safeCall(r, ctx, func() bool { r.hook.OnServerMsg(ctx, text); return false })
	}
}

func FireKill(ctx HookCtx, e KillEvent) {
	for _, r := range snapshot() {
		safeCall(r, ctx, func() bool { r.hook.OnKill(ctx, e); return false })
	}
}

func FireTick(ctx HookCtx, st client.TickState) {
	for _, r := range snapshot() {
		safeCall(r, ctx, func() bool { r.hook.OnTick(ctx, st); return false })
	}
}

func FireKey(ctx HookCtx, k Key) (handled bool) {
	for _, r := range snapshot() {
		if safeCall(r, ctx, func() bool { return r.hook.OnKey(ctx, k) }) {
			return true // first hook to consume the key wins
		}
	}
	return false
}
