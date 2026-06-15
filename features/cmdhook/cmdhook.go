// Package cmdhook bridges teetui events to external executables (§T85/§T71/§C19).
// For a discrete event it runs <hooks>/<event> (under the config dir), feeds the
// event as JSON on stdin, and applies the action lines it prints on stdout. It
// lets users extend teetui in any language without recompiling. Active only when
// the hooks directory exists; never bridges the high-frequency tick/key events.
//
// stdout actions (one per line): say <msg> | say-team <msg> | log <msg> |
// suppress (chat only).
package cmdhook

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jxsl13/teetui/feature"
)

type cmdHook struct {
	feature.NopFeature
	dir     string
	timeout time.Duration
}

func (*cmdHook) Name() string { return "cmdhook" }

func (h *cmdHook) Provision(host feature.Host) error {
	h.timeout = 2 * time.Second
	dir := host.DataPath("hooks")
	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		h.dir = dir // active only when the directory exists
	}
	return nil
}

// run executes the hook script for event (if present+executable), feeding payload
// as JSON, and applies the printed actions via host. Failures are isolated.
func (h *cmdHook) run(event string, payload any, host feature.Host) (suppress bool) {
	if h.dir == "" {
		return false
	}
	path := filepath.Join(h.dir, event)
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() || fi.Mode()&0o111 == 0 {
		return false
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path)
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		host.Log("hook " + event + " failed")
		return false
	}
	return applyHookActions(string(out), host)
}

// applyHookActions parses + applies a hook script's stdout (§T85).
func applyHookActions(out string, host feature.Host) (suppress bool) {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		verb, rest, _ := strings.Cut(line, " ")
		rest = strings.TrimSpace(rest)
		switch verb {
		case "say":
			if rest != "" {
				host.SendChat(rest, false)
			}
		case "say-team":
			if rest != "" {
				host.SendChat(rest, true)
			}
		case "log":
			host.Log(rest)
		case "suppress":
			suppress = true
		}
	}
	return suppress
}

func (h *cmdHook) OnChat(host feature.Host, e feature.ChatEvent) bool {
	return h.run("chat", e, host)
}
func (h *cmdHook) OnConnect(host feature.Host) {
	h.run("connect", map[string]string{"server": host.Server()}, host)
}
func (h *cmdHook) OnDisconnect(host feature.Host, reason string) {
	h.run("disconnect", map[string]string{"reason": reason}, host)
}
func (h *cmdHook) OnBroadcast(host feature.Host, text string) {
	h.run("broadcast", map[string]string{"text": text}, host)
}
func (h *cmdHook) OnServerMsg(host feature.Host, text string) {
	h.run("servermsg", map[string]string{"text": text}, host)
}
func (h *cmdHook) OnKill(host feature.Host, e feature.KillEvent) { h.run("kill", e, host) }

func init() { feature.Register(&cmdHook{}) }
