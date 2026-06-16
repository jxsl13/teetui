package cmdhook

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jxsl13/teetui/feature"
)

// recHost records SendChat/Log for assertions.
type recHost struct {
	feature.NopAPI
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
func (h *recHost) Log(msg string) { h.logs = append(h.logs, msg) }

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

	// §T101/§V62: after Close the bridge stops spawning hook processes.
	if err := h.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	host2 := &recHost{}
	if h.OnChat(host2, feature.ChatEvent{Msg: "ping", Name: "bob"}) {
		t.Error("closed cmdhook should not run the hook")
	}
	if len(host2.chats) != 0 {
		t.Errorf("closed cmdhook still spawned: %v", host2.chats)
	}
}
