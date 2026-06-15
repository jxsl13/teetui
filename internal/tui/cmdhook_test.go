package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jxsl13/teetui/extension"
	"github.com/jxsl13/twclient/client"
)

// recordCtx is an extension.HookCtx that records actions for assertions.
type recordCtx struct {
	chats []string
	team  []string
	logs  []string
}

func (c *recordCtx) SendChat(msg string, team bool) {
	if team {
		c.team = append(c.team, msg)
	} else {
		c.chats = append(c.chats, msg)
	}
}
func (c *recordCtx) Do(client.Action) error       { return nil }
func (c *recordCtx) Log(msg string)               { c.logs = append(c.logs, msg) }
func (c *recordCtx) Roster() []client.PlayerState { return nil }
func (c *recordCtx) Config(string) (string, bool) { return "", false }
func (c *recordCtx) Server() string               { return "test:8303" }

// §T71: applyHookActions parses the stdout grammar and applies it to the ctx.
func TestApplyHookActions(t *testing.T) {
	ctx := &recordCtx{}
	suppress := applyHookActions("say hello world\nsay-team gg\nlog noted\nsuppress\n\n", ctx)
	if !suppress {
		t.Error("suppress not detected")
	}
	if len(ctx.chats) != 1 || ctx.chats[0] != "hello world" {
		t.Errorf("say = %v", ctx.chats)
	}
	if len(ctx.team) != 1 || ctx.team[0] != "gg" {
		t.Errorf("say-team = %v", ctx.team)
	}
	if len(ctx.logs) != 1 || ctx.logs[0] != "noted" {
		t.Errorf("log = %v", ctx.logs)
	}
}

// §T71: newCmdHook is nil unless the hooks dir exists (opt-in).
func TestNewCmdHookOptIn(t *testing.T) {
	if newCmdHook(filepath.Join(t.TempDir(), "nope")) != nil {
		t.Error("cmd hook should be nil without a hooks dir")
	}
	if newCmdHook(t.TempDir()) == nil {
		t.Error("cmd hook should exist when dir present")
	}
}

// §T71/§V40: a real executable hook receives the event on stdin and its stdout
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
	h := newCmdHook(dir)
	if h == nil {
		t.Fatal("hook nil")
	}
	ctx := &recordCtx{}
	if suppress := h.OnChat(ctx, extension.ChatEvent{Msg: "ping", Name: "bob"}); !suppress {
		t.Error("hook should suppress")
	}
	if len(ctx.chats) != 1 || ctx.chats[0] != "pong" {
		t.Errorf("hook say not applied: %v", ctx.chats)
	}

	// No "kill" script → OnKill is a silent no-op (must not panic).
	h.OnKill(ctx, extension.KillEvent{Killer: 1, Victim: 2})
}
