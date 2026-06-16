package feature

import (
	"errors"
	"testing"

	"github.com/jxsl13/twclient/client"
)

// fakeHost embeds NopAPI and overrides only the bits these tests assert.
type fakeHost struct {
	NopAPI
	chats   []string
	logs    []string
	configs map[string]string
}

func newFakeHost() *fakeHost { return &fakeHost{configs: map[string]string{}} }

func (h *fakeHost) SendChat(msg string, team bool)      { h.chats = append(h.chats, msg) }
func (h *fakeHost) Log(msg string)                      { h.logs = append(h.logs, msg) }
func (h *fakeHost) DefineConfig(name, def, help string) { h.configs[name] = def }
func (h *fakeHost) Config(name string) (string, bool)   { v, ok := h.configs[name]; return v, ok }

// recFeat records events + declares a cvar at Init. It implements only the
// Initializer, ChatHandler and KeyHandler interfaces — no TickHandler etc.
type recFeat struct {
	chats    []string
	ticks    int
	suppress bool
}

func (*recFeat) Name() string { return "rec" }
func (*recFeat) Init(h API) error {
	h.DefineConfig("rec_enabled", "1", "demo")
	return nil
}
func (f *recFeat) OnChat(h API, e ChatEvent) bool {
	f.chats = append(f.chats, e.Msg)
	h.SendChat("seen:"+e.Msg, false)
	return f.suppress
}
func (f *recFeat) OnKey(_ API, k Key) bool { return k.Name == "F9" }

// §T100/§V60: features register, init (declaring their own cvars), receive only
// the events whose handler interface they implement, and compose (suppress OR,
// key first-wins).
func TestFeatureDispatch(t *testing.T) {
	Reset()
	defer Reset()

	f := &recFeat{}
	Register(f)
	if Count() != 1 {
		t.Fatalf("Count = %d want 1", Count())
	}

	h := newFakeHost()
	if errs := InitAll(h); len(errs) != 0 {
		t.Fatalf("init errs: %v", errs)
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
	// recFeat does NOT implement TickHandler → FireTick must not touch it and
	// must not panic (§V60).
	FireTick(h, client.TickState{})
	if f.ticks != 0 {
		t.Errorf("feature without TickHandler received OnTick")
	}

	f.suppress = true
	if !FireChat(h, ChatEvent{Msg: "x"}) {
		t.Error("suppress not propagated")
	}
	if FireKey(h, Key{Rune: 'a'}) || !FireKey(h, Key{Name: "F9"}) {
		t.Error("OnKey first-wins wrong")
	}
}

// nameOnly implements ONLY Feature (no handlers, no Init) — it must register and
// survive every dispatch untouched, proving handlers are optional (§V60).
type nameOnly struct{}

func (nameOnly) Name() string { return "nameonly" }

func TestFeatureNoHandlers(t *testing.T) {
	Reset()
	defer Reset()
	Register(nameOnly{})
	h := newFakeHost()
	if errs := InitAll(h); len(errs) != 0 { // no Initializer → skipped, no error
		t.Fatalf("init errs: %v", errs)
	}
	// none of these may panic for a handler-less feature.
	FireConnect(h)
	FireChat(h, ChatEvent{Msg: "x"})
	FireTick(h, client.TickState{})
	FireKey(h, Key{Rune: 'a'})
}

// panicFeat panics in OnChat; provErrFeat errors in Init.
type panicFeat struct{}

func (panicFeat) Name() string               { return "boom" }
func (panicFeat) Init(API) error             { return nil }
func (panicFeat) OnChat(API, ChatEvent) bool { panic("boom") }

type provErrFeat struct{}

func (provErrFeat) Name() string   { return "preverr" }
func (provErrFeat) Init(API) error { return errors.New("nope") }

// §T100/§V47: an Init error or a handler panic disables only the offending
// feature; others keep working and the client never crashes.
func TestFeatureIsolation(t *testing.T) {
	Reset()
	defer Reset()

	Register(provErrFeat{})
	good := &recFeat{}
	Register(panicFeat{})
	Register(good)

	h := newFakeHost()
	errs := InitAll(h)
	if len(errs) != 1 {
		t.Fatalf("want 1 init err, got %v", errs)
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
