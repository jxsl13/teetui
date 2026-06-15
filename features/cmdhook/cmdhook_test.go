package cmdhook

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jxsl13/teetui/feature"
	"github.com/jxsl13/twclient/client"
)

// recHost records SendChat/Log for assertions.
type recHost struct {
	chats []string
	team  []string
	logs  []string
}

func (h *recHost) SendChat(msg string, team bool) {
	if team {
		h.team = append(h.team, msg)
	} else {
		h.chats = append(h.chats, msg)
	}
}
func (h *recHost) Do(client.Action) error                                  { return nil }
func (h *recHost) RconLogin(string)                                        {}
func (h *recHost) Log(msg string)                                          { h.logs = append(h.logs, msg) }
func (h *recHost) Roster() []client.PlayerState                            { return nil }
func (h *recHost) Tick() (client.TickState, bool)                          { return client.TickState{}, false }
func (h *recHost) PlayerName() string                                      { return "me" }
func (h *recHost) PlayerClan() string                                      { return "" }
func (h *recHost) Server() string                                          { return "s:8303" }
func (h *recHost) DefineConfig(string, string, string)                     {}
func (h *recHost) Config(string) (string, bool)                            { return "", false }
func (h *recHost) OnSendChat(func(string, bool) (string, bool))            {}
func (h *recHost) DefineAction(string, string, string, func())             {}
func (h *recHost) DefineCommand(string, string, func(string) []string)     {}
func (h *recHost) AddStatusField(func() string)                            {}
func (h *recHost) AddNameStyle(func(string, string) (feature.Style, bool)) {}
func (h *recHost) Provide(string, any)                                     {}
func (h *recHost) Lookup(string) (any, bool)                               { return nil, false }
func (h *recHost) DataPath(name string) string                             { return name }

// §T85: applyHookActions parses the stdout grammar and applies it via the host.
func TestApplyHookActions(t *testing.T) {
	h := &recHost{}
	if !applyHookActions("say hello world\nsay-team gg\nlog noted\nsuppress\n\n", h) {
		t.Error("suppress not detected")
	}
	if len(h.chats) != 1 || h.chats[0] != "hello world" {
		t.Errorf("say = %v", h.chats)
	}
	if len(h.team) != 1 || h.team[0] != "gg" {
		t.Errorf("say-team = %v", h.team)
	}
	if len(h.logs) != 1 || h.logs[0] != "noted" {
		t.Errorf("log = %v", h.logs)
	}
}

// §T85/§V40: a real executable hook receives the event on stdin and its stdout
// actions are applied; a missing event script is a silent no-op.
func TestCmdHookExec(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script hook test is unix-only")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\ncat >/dev/null\necho 'say pong'\necho suppress\n"
	if err := os.WriteFile(filepath.Join(dir, "chat"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	h := &cmdHook{dir: dir, timeout: 30 * time.Second} // generous so the race build under load doesn't flake
	host := &recHost{}
	if !h.OnChat(host, feature.ChatEvent{Msg: "ping", Name: "bob"}) {
		t.Error("hook should suppress")
	}
	if len(host.chats) != 1 || host.chats[0] != "pong" {
		t.Errorf("hook say not applied: %v", host.chats)
	}
	h.OnKill(host, feature.KillEvent{Killer: 1, Victim: 2}) // no script → no-op
}
