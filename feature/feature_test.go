package feature

import (
	"errors"
	"testing"
)

// fakeHost embeds NopHost and overrides only the bits these tests assert.
type fakeHost struct {
	NopHost
	chats   []string
	logs    []string
	configs map[string]string
}

func newFakeHost() *fakeHost { return &fakeHost{configs: map[string]string{}} }

func (h *fakeHost) SendChat(msg string, team bool)      { h.chats = append(h.chats, msg) }
func (h *fakeHost) Log(msg string)                      { h.logs = append(h.logs, msg) }
func (h *fakeHost) DefineConfig(name, def, help string) { h.configs[name] = def }
func (h *fakeHost) Config(name string) (string, bool)   { v, ok := h.configs[name]; return v, ok }

// recFeat records events + declares a cvar at Provision.
type recFeat struct {
	NopFeature
	chats    []string
	suppress bool
}

func (*recFeat) Name() string { return "rec" }
func (*recFeat) Provision(h Host) error {
	h.DefineConfig("rec_enabled", "1", "demo")
	return nil
}
func (f *recFeat) OnChat(h Host, e ChatEvent) bool {
	f.chats = append(f.chats, e.Msg)
	h.SendChat("seen:"+e.Msg, false)
	return f.suppress
}
func (f *recFeat) OnKey(_ Host, k Key) bool { return k.Name == "F9" }

// §T75/§V39/§V46: features register, provision (declaring their own cvars),
// receive events, and compose (suppress OR, key first-wins).
func TestFeatureDispatch(t *testing.T) {
	Reset()
	defer Reset()

	f := &recFeat{}
	Register(f)
	if Count() != 1 {
		t.Fatalf("Count = %d want 1", Count())
	}

	h := newFakeHost()
	if errs := ProvisionAll(h); len(errs) != 0 {
		t.Fatalf("provision errs: %v", errs)
	}
	if v, ok := h.Config("rec_enabled"); !ok || v != "1" {
		t.Errorf("feature did not declare its cvar: %q,%v", v, ok)
	}

	if FireChat(h, ChatEvent{Msg: "hi"}) {
		t.Error("non-suppressing feature reported suppress")
	}
	if len(f.chats) != 1 || h.chats[0] != "seen:hi" {
		t.Errorf("event/action not applied: %v / %v", f.chats, h.chats)
	}
	f.suppress = true
	if !FireChat(h, ChatEvent{Msg: "x"}) {
		t.Error("suppress not propagated")
	}
	if FireKey(h, Key{Rune: 'a'}) || !FireKey(h, Key{Name: "F9"}) {
		t.Error("OnKey first-wins wrong")
	}
}

// panicFeat panics in OnChat; provErrFeat errors in Provision.
type panicFeat struct{ NopFeature }

func (panicFeat) Name() string                { return "boom" }
func (panicFeat) Provision(Host) error        { return nil }
func (panicFeat) OnChat(Host, ChatEvent) bool { panic("boom") }

type provErrFeat struct{ NopFeature }

func (provErrFeat) Name() string         { return "preverr" }
func (provErrFeat) Provision(Host) error { return errors.New("nope") }

// §T75/§V47: a Provision error or a hook panic disables only the offending
// feature; others keep working and the client never crashes.
func TestFeatureIsolation(t *testing.T) {
	Reset()
	defer Reset()

	Register(provErrFeat{})
	good := &recFeat{}
	Register(panicFeat{})
	Register(good)

	h := newFakeHost()
	errs := ProvisionAll(h)
	if len(errs) != 1 {
		t.Fatalf("want 1 provision err, got %v", errs)
	}

	// panicFeat panics → disabled; good still gets the event; no crash.
	FireChat(h, ChatEvent{Msg: "one"})
	if len(good.chats) != 1 {
		t.Errorf("good feature missed event: %v", good.chats)
	}
	logsBefore := len(h.logs)
	FireChat(h, ChatEvent{Msg: "two"})
	if len(good.chats) != 2 {
		t.Errorf("good feature missed second event")
	}
	if len(h.logs) != logsBefore {
		t.Error("disabled feature panicked again")
	}
	// provErr + panic both disabled → only good remains enabled.
	if got := len(Registered()); got != 1 {
		t.Errorf("enabled features = %d want 1 (good)", got)
	}
}
