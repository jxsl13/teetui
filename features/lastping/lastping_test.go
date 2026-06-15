package lastping

import (
	"testing"

	"github.com/jxsl13/teetui/feature"
)

// §T83/§V35: the store is bounded newest-first; NextReply walks newest→older and
// resets on a new push.
func TestPingStore(t *testing.T) {
	s := &store{}
	if _, _, ok := s.Newest(); ok {
		t.Error("empty store has no newest")
	}
	s.push("a", "m-a")
	s.push("b", "m-b")
	s.push("c", "m-c")

	if from, _, ok := s.Newest(); !ok || from != "c" {
		t.Errorf("newest = %q want c", from)
	}
	for _, want := range []string{"c", "b", "a"} {
		if from, _, ok := s.NextReply(); !ok || from != want {
			t.Errorf("NextReply = %q,%v want %q", from, ok, want)
		}
	}
	if _, _, ok := s.NextReply(); ok {
		t.Error("NextReply past end should be false")
	}
	s.push("d", "m-d")
	if from, _, ok := s.NextReply(); !ok || from != "d" {
		t.Errorf("after push, NextReply = %q want d", from)
	}

	for i := 0; i < maxPings+5; i++ {
		s.push("x", "y")
	}
	if len(s.buf) != maxPings {
		t.Errorf("len = %d want %d (capped)", len(s.buf), maxPings)
	}
}

type fakeHost struct {
	feature.NopHost
	cfg     map[string]string
	svc     map[string]any
	statusF []func() string
}

func newFakeHost() *fakeHost { return &fakeHost{cfg: map[string]string{}, svc: map[string]any{}} }

func (h *fakeHost) PlayerName() string                  { return "nameless" }
func (h *fakeHost) DefineConfig(name, def, help string) { h.cfg[name] = def }
func (h *fakeHost) Config(name string) (string, bool)   { v, ok := h.cfg[name]; return v, ok }
func (h *fakeHost) AddStatusField(fn func() string)     { h.statusF = append(h.statusF, fn) }
func (h *fakeHost) Provide(name string, svc any)        { h.svc[name] = svc }
func (h *fakeHost) Lookup(name string) (any, bool)      { v, ok := h.svc[name]; return v, ok }

// §T83: the feature provisions the pings service + status field and queues pings.
func TestLastPingFeature(t *testing.T) {
	f := &lastPing{store: &store{}}
	h := newFakeHost()
	_ = f.Provision(h)

	svc, ok := h.svc["pings"].(feature.PingStore)
	if !ok {
		t.Fatal("pings service not provided")
	}
	f.OnChat(h, feature.ChatEvent{Msg: "hi nameless", Name: "bob"})
	f.OnChat(h, feature.ChatEvent{Msg: "unrelated", Name: "bob"})
	if from, _, ok := svc.Newest(); !ok || from != "bob" {
		t.Errorf("ping not queued via OnChat: %q,%v", from, ok)
	}
	if len(h.statusF) == 0 || h.statusF[0]() != "" {
		t.Error("status should be empty when cl_show_last_ping off")
	}
	h.cfg["cl_show_last_ping"] = "1"
	if got := h.statusF[0](); got == "" {
		t.Error("status should show the last ping when enabled")
	}
}
