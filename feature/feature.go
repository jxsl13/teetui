// Package feature is teetui's public module SDK (§C21/§I.feature). Every
// chillerbot-style feature is a self-registering module — like Caddy v2's
// caddy.RegisterModule or the image stdlib's image.RegisterFormat — implemented
// purely against the API capability surface, never against teetui internals.
//
// A feature package registers itself in init() and is activated by being blank-
// imported from main.go. A Feature needs only a Name; setup and every event are
// OPTIONAL interfaces the core discovers by type assertion, so a feature
// implements just what it needs (no no-op stubs, §C27/§V60):
//
//	package myfeat
//	type feat struct{}
//	func (feat) Name() string                 { return "myfeat" }
//	func (f feat) Init(h feature.API) error   { /* declare cvars/actions */ return nil } // optional
//	func (feat) OnChat(h feature.API, e feature.ChatEvent) bool { … }                    // optional
//	func init() { feature.Register(feat{}) }
//
// The API action surface is exactly teetui's safe twclient capability set — no
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

// PlayerJoinEvent fires when a player appears (§C30). Unified 0.6/0.7.
type PlayerJoinEvent struct {
	ClientID int
	Name     string
	Clan     string
	Team     int
}

// PlayerLeaveEvent fires when a player drops (§C30); Reason may be empty on 0.6.
type PlayerLeaveEvent struct {
	ClientID int
	Reason   string
}

// TeamChangeEvent fires when a player's team changes (§C30). Team ids: spectators
// -1, red/flock(game) 0, blue 1. Silent suppresses the message (server hint).
type TeamChangeEvent struct {
	ClientID int
	Team     int
	Silent   bool
}

// Key is a key press handed to OnKey. Character keys set Rune; named keys (F1,
// Enter, Tab, …) set Name with Rune==0.
type Key struct {
	Rune rune
	Name string
}

// Style is the cell style a feature contributes (e.g. warlist name coloring).
type Style = tcell.Style

// The API is split into small, named capability sub-interfaces (interface
// segregation, §C27/§V63): a handler or consumer may depend on just the slice it
// needs (e.g. take a ChatSender instead of the whole API). The full API embeds
// them all. Every action is limited to teetui's safe twclient surface (§V47).

// ChatSender sends a chat line (team chat when team is true).
type ChatSender interface {
	SendChat(msg string, team bool)
}

// ActionDoer performs a twclient action and async rcon auth.
type ActionDoer interface {
	Do(action client.Action) error
	RconLogin(password string) // async rcon auth (off-loop); logs the outcome
}

// Logger writes a line to the teetui log.
type Logger interface {
	Log(msg string)
}

// StateReader reads read-only session state.
type StateReader interface {
	Roster() []client.PlayerState
	Tick() (client.TickState, bool)
	PlayerName() string
	PlayerClan() string
	Server() string
}

// ConfigStore declares and reads a feature's own cvars (a feature OWNS its
// cvars, declared at Init, §V46).
type ConfigStore interface {
	DefineConfig(name, def, help string)
	Config(name string) (string, bool)
}

// ActionRegistry registers rebindable key actions (respecting the keymap, §V19)
// and F1 console commands.
type ActionRegistry interface {
	DefineAction(name, defaultKey, help string, run func())
	DefineCommand(name, help string, run func(args string) []string)
}

// UIRegistry contributes to the rendered UI: an outgoing-chat filter chain (for
// !commands / silent-chat — each fn may rewrite the line or cancel the send), a
// status-bar field, and a per-name style (scoreboard/nameplate coloring).
type UIRegistry interface {
	AddSendChatFilter(fn func(msg string, team bool) (out string, send bool))
	AddStatusField(fn func() string)
	AddNameStyle(fn func(name, clan string) (Style, bool))
}

// ServiceRegistry shares cross-feature services as `any` (§V53): a feature
// Provides its concrete value under a name; consumers Lookup and type-assert.
type ServiceRegistry interface {
	Provide(name string, svc any)
	Lookup(name string) (any, bool)
}

// Paths resolves a path under the teetui config dir for a feature's persisted
// file ("" if no config dir is available).
type Paths interface {
	DataPath(name string) string
}

// API is the full capability surface a feature may use — the sufficient, safe
// API (§I.feature/§V47), composed from the sub-interfaces above (§V63).
type API interface {
	ChatSender
	ActionDoer
	Logger
	StateReader
	ConfigStore
	ActionRegistry
	UIRegistry
	ServiceRegistry
	Paths
}

// Feature is a registerable module identified by a unique name. Everything else
// — setup, validation, teardown, event handling — is an OPTIONAL interface the
// core discovers by type assertion (the Go optional-interface idiom, like code
// checking whether an io.Reader is also an io.Closer). A feature implements only
// what it needs, and adding a new optional interface never breaks existing
// features (§C27/§V60).
type Feature interface {
	Name() string
}

// Initializer is the optional one-time setup hook: declare cvars/actions/status
// fields and look up services. Named per the Go -er convention, not Caddy's
// "Provisioner" (§C27/§V61).
type Initializer interface {
	Init(API) error
}

// Validator is the optional post-init check (§V62): Validate runs after Init and
// a non-nil error disables the feature (logged), leaving the rest running.
type Validator interface {
	Validate() error
}

// Closer releases a feature's resources — goroutines, files, handles — named per
// io.Closer, not Caddy's "CleanerUpper" (§C27/§V61/§V62). Close runs on shutdown
// and when a feature is disabled after a panic; it must be safe even after a
// PARTIAL Init and is called at most once per feature.
type Closer interface {
	Close() error
}

// Event-handler interfaces — all OPTIONAL, named `…Handler` after net/http.Handler
// (§C27/§V61). A feature implements only the events it cares about; the core
// type-asserts each registered feature and skips the handlers it lacks (§V60).
// OnChat returning true suppresses the line; OnKey returning true consumes the key.
type ConnectHandler interface{ OnConnect(API) }
type DisconnectHandler interface{ OnDisconnect(API, string) }
type ChatHandler interface {
	OnChat(API, ChatEvent) (suppress bool)
}
type BroadcastHandler interface{ OnBroadcast(API, string) }
type ServerMsgHandler interface{ OnServerMsg(API, string) }
type KillHandler interface{ OnKill(API, KillEvent) }
type TickHandler interface{ OnTick(API, client.TickState) }
type KeyHandler interface {
	OnKey(API, Key) (handled bool)
}

// Player/team event handlers (§C30/§V68) — optional, forward-compatible (V60).
type PlayerJoinHandler interface{ OnPlayerJoin(API, PlayerJoinEvent) }
type PlayerLeaveHandler interface{ OnPlayerLeave(API, PlayerLeaveEvent) }
type TeamChangeHandler interface{ OnTeamChange(API, TeamChangeEvent) }

// Cross-feature services are passed as `any` through Provide/Lookup (§V53): the
// providing feature Provides its concrete value under a name, and each consumer
// declares the MINIMAL interface it needs and type-asserts the looked-up value.
// The SDK stays feature-agnostic — it declares no feature-specific service
// contracts (no Warlist, no PingStore); those live with the consumer.

// NopAPI is a API that does nothing and returns zero values. Embed it in tests
// (or a minimal feature harness) and override only the methods you exercise, so a
// fake need not re-implement the whole API surface.
type NopAPI struct{}

func (NopAPI) SendChat(string, bool)                               {}
func (NopAPI) Do(client.Action) error                              { return nil }
func (NopAPI) RconLogin(string)                                    {}
func (NopAPI) Log(string)                                          {}
func (NopAPI) Roster() []client.PlayerState                        { return nil }
func (NopAPI) Tick() (client.TickState, bool)                      { return client.TickState{}, false }
func (NopAPI) PlayerName() string                                  { return "" }
func (NopAPI) PlayerClan() string                                  { return "" }
func (NopAPI) Server() string                                      { return "" }
func (NopAPI) DefineConfig(string, string, string)                 {}
func (NopAPI) Config(string) (string, bool)                        { return "", false }
func (NopAPI) AddSendChatFilter(func(string, bool) (string, bool)) {}
func (NopAPI) DefineAction(string, string, string, func())         {}
func (NopAPI) DefineCommand(string, string, func(string) []string) {}
func (NopAPI) AddStatusField(func() string)                        {}
func (NopAPI) AddNameStyle(func(string, string) (Style, bool))     {}
func (NopAPI) Provide(string, any)                                 {}
func (NopAPI) Lookup(string) (any, bool)                           { return nil, false }
func (NopAPI) DataPath(name string) string                         { return name }
