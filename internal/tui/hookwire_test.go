package tui

import (
	"testing"

	"github.com/jxsl13/teetui/feature"
)

// keyEater consumes the 't' key and records what it sees.
type keyEater struct {
	got []feature.Key
}

func (*keyEater) Name() string { return "keyeater" }
func (k *keyEater) OnKey(_ feature.Host, key feature.Key) bool {
	k.got = append(k.got, key)
	return key.Rune == 't'
}

// §T76/§V39: a registered feature OnKey gets first refusal — returning handled
// consumes the key before teetui's own handling; non-handled keys pass through.
func TestFeatureKeyConsume(t *testing.T) {
	feature.Reset()
	defer feature.Reset()

	eat := &keyEater{}
	feature.Register(eat)
	app, _ := newTestApp(t)

	app.handle(rk('t')) // consumed by the feature → stays NORMAL
	if app.mode != modeNormal {
		t.Errorf("consumed key still acted: mode=%d", app.mode)
	}
	if len(eat.got) == 0 {
		t.Fatal("feature did not receive the key")
	}

	vis := app.visual
	app.handle(rk('v')) // not consumed → passes through and toggles visual
	if app.visual == vis {
		t.Error("non-consumed key did not pass through to teetui")
	}
}

// §T76: with no features registered, key handling is unaffected.
func TestFeatureNoneRegistered(t *testing.T) {
	feature.Reset()
	defer feature.Reset()

	app, _ := newTestApp(t)
	app.handle(rk('t')) // no features → normal chat entry
	if app.mode != modeChat {
		t.Errorf("without features 't' should enter chat, mode=%d", app.mode)
	}
}

// §T76: the Host adapter exposes teetui state to features.
func TestHostCtxSurface(t *testing.T) {
	app, _ := newTestApp(t)
	h := app.host()
	if h.Server() != "test:8303" {
		t.Errorf("Server() = %q", h.Server())
	}
	if v, ok := h.Config("cl_log_lines"); !ok || v != "10" {
		t.Errorf("Config(cl_log_lines) = %q,%v", v, ok)
	}
	if _, ok := h.Config("does_not_exist"); ok {
		t.Error("unknown cvar should report not found")
	}
}
