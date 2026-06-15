package tui

import (
	"testing"

	"github.com/jxsl13/teetui/extension"
)

// keyEater consumes the 't' key and records what it sees.
type keyEater struct {
	extension.NopHook
	got []extension.Key
}

func (k *keyEater) OnKey(_ extension.HookCtx, key extension.Key) bool {
	k.got = append(k.got, key)
	return key.Rune == 't'
}

// §T70/§V39: a registered OnKey hook gets first refusal — returning handled
// consumes the key before teetui's own handling; non-handled keys pass through.
func TestHookKeyConsume(t *testing.T) {
	extension.Reset()
	defer extension.Reset()

	app, _ := newTestApp(t)
	eat := &keyEater{}
	extension.Register("eat", eat)

	// 't' normally enters chat; the hook consumes it → stays NORMAL.
	app.handle(rk('t'))
	if app.mode != modeNormal {
		t.Errorf("consumed key still acted: mode=%d", app.mode)
	}
	if len(eat.got) == 0 {
		t.Fatal("hook did not receive the key")
	}

	// 'v' is not consumed → passes through and toggles visual.
	vis := app.visual
	app.handle(rk('v'))
	if app.visual == vis {
		t.Error("non-consumed key did not pass through to teetui")
	}
}

// §T70: with no hooks registered, key handling is unaffected (zero-cost path).
func TestHookNoneRegistered(t *testing.T) {
	extension.Reset()
	defer extension.Reset()

	app, _ := newTestApp(t)
	app.handle(rk('t')) // no hooks → normal chat entry
	if app.mode != modeChat {
		t.Errorf("without hooks 't' should enter chat, mode=%d", app.mode)
	}
}

// §T70: the HookCtx adapter exposes teetui state to hooks.
func TestHookCtxSurface(t *testing.T) {
	app, _ := newTestApp(t)
	ctx := app.hookCtx()
	if ctx.Server() != "test:8303" {
		t.Errorf("Server() = %q", ctx.Server())
	}
	if v, ok := ctx.Config("cl_silent_chat_commands"); !ok || v != "1" {
		t.Errorf("Config(cl_silent_chat_commands) = %q,%v", v, ok)
	}
	if _, ok := ctx.Config("does_not_exist"); ok {
		t.Error("unknown cvar should report not found")
	}
}
