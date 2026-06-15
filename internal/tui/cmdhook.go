package tui

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jxsl13/teetui/extension"
)

// cmdHook is the opt-in EXTERNAL command-hook bridge (§T71/§C19): for a discrete
// event it runs an executable at <hooksDir>/<event>, feeds the event as JSON on
// stdin, and applies the action lines it prints on stdout. It lets users extend
// teetui in any language without recompiling. It is registered only when the
// hooks directory exists, and never handles the high-frequency tick/key events.
//
// stdout action grammar (one per line):
//
//	say <message>        send public chat
//	say-team <message>   send team chat
//	log <message>        write to the teetui log
//	suppress             (chat only) hide the triggering line
type cmdHook struct {
	extension.NopHook
	dir     string
	timeout time.Duration
}

// newCmdHook returns a command-hook bound to dir, or nil if dir does not exist
// (so the feature stays fully off unless the user creates the directory).
func newCmdHook(dir string) *cmdHook {
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return nil
	}
	return &cmdHook{dir: dir, timeout: 2 * time.Second}
}

// run executes the hook script for event (if present+executable), feeding payload
// as JSON, and applies the printed actions via ctx. Returns whether a "suppress"
// action was emitted. All failures are isolated (logged, never fatal, §V40).
func (h *cmdHook) run(event string, payload any, ctx extension.HookCtx) (suppress bool) {
	path := filepath.Join(h.dir, event)
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() || fi.Mode()&0o111 == 0 {
		return false // no executable hook for this event
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	cctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, path)
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		if ctx != nil {
			ctx.Log("hook " + event + " failed")
		}
		return false
	}
	return applyHookActions(string(out), ctx)
}

// applyHookActions parses and applies a hook script's stdout action lines.
func applyHookActions(out string, ctx extension.HookCtx) (suppress bool) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		verb, rest, _ := strings.Cut(line, " ")
		rest = strings.TrimSpace(rest)
		switch verb {
		case "say":
			if rest != "" && ctx != nil {
				ctx.SendChat(rest, false)
			}
		case "say-team":
			if rest != "" && ctx != nil {
				ctx.SendChat(rest, true)
			}
		case "log":
			if ctx != nil {
				ctx.Log(rest)
			}
		case "suppress":
			suppress = true
		}
	}
	return suppress
}

// Event methods: discrete events only (tick/key stay NopHook — too frequent to
// spawn a process for).

func (h *cmdHook) OnChat(ctx extension.HookCtx, e extension.ChatEvent) bool {
	return h.run("chat", e, ctx)
}
func (h *cmdHook) OnConnect(ctx extension.HookCtx) {
	h.run("connect", map[string]string{"server": ctx.Server()}, ctx)
}
func (h *cmdHook) OnDisconnect(ctx extension.HookCtx, reason string) {
	h.run("disconnect", map[string]string{"reason": reason}, ctx)
}
func (h *cmdHook) OnBroadcast(ctx extension.HookCtx, text string) {
	h.run("broadcast", map[string]string{"text": text}, ctx)
}
func (h *cmdHook) OnServerMsg(ctx extension.HookCtx, text string) {
	h.run("servermsg", map[string]string{"text": text}, ctx)
}
func (h *cmdHook) OnKill(ctx extension.HookCtx, e extension.KillEvent) { h.run("kill", e, ctx) }
