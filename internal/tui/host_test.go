package tui

import (
	"strings"
	"testing"

	"github.com/jxsl13/teetui/feature"
)

// tfeat exercises the Host registration surface.
type tfeat struct {
	feature.NopFeature
	acted bool
}

func (*tfeat) Name() string { return "tfeat" }
func (f *tfeat) Provision(h feature.Host) error {
	h.DefineConfig("cl_tfeat", "7", "demo cvar")
	h.DefineAction("tfeat", "g", "demo action", func() { f.acted = true })
	h.AddStatusField(func() string { return "TF-OK" })
	h.OnSendChat(func(msg string, team bool) (string, bool) {
		if msg == "block" {
			return msg, false
		}
		return msg + "!", true
	})
	h.Provide("tsvc", 42)
	return nil
}

// §T76/§V46/§V47: a feature provisions through the Host — its cvar is get/set
// from the console, its action fires on its key, its status field shows, its
// service is discoverable, and its outgoing-chat hook transforms/cancels.
func TestHostProvisioning(t *testing.T) {
	feature.Reset()
	defer feature.Reset()
	f := &tfeat{}
	feature.Register(f)

	app, sim := newTestApp(t) // NewAppWithScreen provisions registered features

	// Feature cvar readable via Host.Config and console get/set.
	if v, ok := app.host().Config("cl_tfeat"); !ok || v != "7" {
		t.Errorf("Config(cl_tfeat) = %q,%v want 7", v, ok)
	}
	app.runLocal("cl_tfeat 9")
	if v, _ := app.host().Config("cl_tfeat"); v != "9" {
		t.Errorf("after set, cl_tfeat = %q want 9", v)
	}

	// Feature action bound to 'g' (unbound in core keymap) fires in NORMAL mode.
	app.handle(rk('g'))
	if !f.acted {
		t.Error("feature action did not fire on its key")
	}

	// Status field appears in the rendered status bar.
	app.draw()
	if !strings.Contains(dumpSim(sim), "TF-OK") {
		t.Error("feature status field not rendered")
	}

	// Service registry.
	if v, ok := app.host().Lookup("tsvc"); !ok || v.(int) != 42 {
		t.Errorf("Lookup(tsvc) = %v,%v want 42", v, ok)
	}

	// Outgoing-chat hook chain: edits non-blocked, cancels "block".
	if out, send := app.runSendChatHooks("hi", false); !send || out != "hi!" {
		t.Errorf("sendchat hook edit = %q,%v want hi!,true", out, send)
	}
	if _, send := app.runSendChatHooks("block", false); send {
		t.Error("sendchat hook should cancel 'block'")
	}
}
