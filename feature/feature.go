// Package feature is teetui's public module SDK (§C21/§I.feature). Every
// chillerbot-style feature is a self-registering module — like Caddy v2's
// caddy.RegisterModule or the image stdlib's image.RegisterFormat — implemented
// purely against the Host capability surface, never against teetui internals.
//
// A feature package registers itself in init() and is activated by being blank-
// imported from main.go:
//
//	package myfeat
//	type feat struct{ feature.NopFeature }
//	func (feat) Name() string                 { return "myfeat" }
//	func (f feat) Provision(h feature.Host) error { /* declare cvars/actions */ return nil }
//	func (feat) OnChat(h feature.Host, e feature.ChatEvent) bool { … }
//	func init() { feature.Register(feat{}) }
//
// The Host action surface is exactly teetui's safe twclient capability set — no
// raw packet/network primitive — so a feature cannot be turned into a flood/DoS
// tool (§V39/§V47). A panic in any feature is recovered and the feature disabled,
// never crashing the client (§V40/§V47).
package feature

import (
	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/twclient/client"
)

// ChatEvent is an incoming chat line.
type ChatEvent struct {
	ClientID int
	Name     string
	Msg      string
	Team     bool
}

// KillEvent is a kill-feed entry.
type KillEvent struct {
	Killer int
	Victim int
	Weapon int
}

// Key is a key press handed to OnKey. Character keys set Rune; named keys (F1,
// Enter, Tab, …) set Name with Rune==0.
type Key struct {
	Rune rune
	Name string
}

// Style is the cell style a feature contributes (e.g. warlist name coloring).
type Style = tcell.Style

// Host is the capability surface a feature may use — the sufficient, safe API
// (§I.feature/§V47). Actions are limited to teetui's twclient public surface.
type Host interface {
	// actions
	SendChat(msg string, team bool)
	Do(action client.Action) error
	RconLogin(password string) // async rcon auth (off-loop); logs the outcome
	Log(msg string)
	// state
	Roster() []client.PlayerState
	Tick() (client.TickState, bool)
	PlayerName() string
	PlayerClan() string
	Server() string
	// config: a feature OWNS its cvars, declared at Provision (§V46)
	DefineConfig(name, def, help string)
	Config(name string) (string, bool)
	// outgoing-chat interception chain (for !commands / silent-chat)
	OnSendChat(fn func(msg string, team bool) (out string, send bool))
	// named, rebindable actions (respect the keymap, §V19) + a default key
	DefineAction(name, defaultKey, help string, run func())
	// F1 console commands (args string → output lines)
	DefineCommand(name, help string, run func(args string) []string)
	// status-bar / HUD contributions
	AddStatusField(fn func() string)
	// render contributions: per-name style (scoreboard/nameplate coloring)
	AddNameStyle(fn func(name, clan string) (Style, bool))
	// cross-feature services (← caddy ctx.App): Provide one, Lookup another
	Provide(name string, svc any)
	Lookup(name string) (any, bool)
	// DataPath returns an absolute path under the teetui config dir for a
	// feature's persisted file ("" if no config dir is available).
	DataPath(name string) string
}

// Hooks is the event set a feature can handle; embed NopFeature for the rest.
// OnChat returning true suppresses the line; OnKey returning true consumes it.
type Hooks interface {
	OnConnect(Host)
	OnDisconnect(Host, string)
	OnChat(Host, ChatEvent) (suppress bool)
	OnBroadcast(Host, string)
	OnServerMsg(Host, string)
	OnKill(Host, KillEvent)
	OnTick(Host, client.TickState)
	OnKey(Host, Key) (handled bool)
}

// Feature is a registerable module: an identity, a one-time Provision, and the
// event Hooks.
type Feature interface {
	Name() string
	Provision(Host) error
	Hooks
}

// PingStore is the cross-feature service the lastping feature Provides (under the
// name "pings") and the reply feature Looks up (§T83/§T79): a newest-first
// history of chat lines that mentioned us. Newest is for display; NextReply
// drives the H reply, walking from newest to older on repeated calls (the cursor
// resets when a new ping arrives).
type PingStore interface {
	Newest() (from, msg string, ok bool)
	NextReply() (from, msg string, ok bool)
}

// NopFeature is a no-op Hooks implementation; embed it so a feature only
// overrides the events it cares about. (It does NOT supply Name/Provision —
// those are mandatory per feature.)
type NopFeature struct{}

func (NopFeature) OnConnect(Host)                {}
func (NopFeature) OnDisconnect(Host, string)     {}
func (NopFeature) OnChat(Host, ChatEvent) bool   { return false }
func (NopFeature) OnBroadcast(Host, string)      {}
func (NopFeature) OnServerMsg(Host, string)      {}
func (NopFeature) OnKill(Host, KillEvent)        {}
func (NopFeature) OnTick(Host, client.TickState) {}
func (NopFeature) OnKey(Host, Key) bool          { return false }
