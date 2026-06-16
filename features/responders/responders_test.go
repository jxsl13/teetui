package responders

import (
	"testing"

	"github.com/jxsl13/teetui/feature"
)

type fh struct {
	feature.NopAPI
	cfg   map[string]string
	chats []string
}

func newFH() *fh { return &fh{cfg: map[string]string{}} }

func (h *fh) SendChat(msg string, team bool)      { h.chats = append(h.chats, msg) }
func (h *fh) PlayerName() string                  { return "nameless" }
func (h *fh) DefineConfig(name, def, help string) { h.cfg[name] = def }
func (h *fh) Config(name string) (string, bool)   { v, ok := h.cfg[name]; return v, ok }

// §T82/§V33: responders only fire on a ping, only when enabled, and the
// auto-reply template expands %n.
func TestResponders(t *testing.T) {
	f := &responders{}
	h := newFH()
	_ = f.Init(h)

	// Not a ping → nothing.
	f.OnChat(h, feature.ChatEvent{Msg: "hello world", Name: "bob"})
	if len(h.chats) != 0 {
		t.Fatalf("non-ping fired a reply: %v", h.chats)
	}

	// Ping but disabled → nothing.
	f.OnChat(h, feature.ChatEvent{Msg: "hey nameless", Name: "bob"})
	if len(h.chats) != 0 {
		t.Fatalf("disabled responders fired: %v", h.chats)
	}

	// Enable tapped-out → fires once on ping.
	h.cfg["cl_tapped_out_message"] = "1"
	f.OnChat(h, feature.ChatEvent{Msg: "yo nameless", Name: "bob"})
	if len(h.chats) != 1 || h.chats[0] != "I'm currently tapped out (afk)" {
		t.Fatalf("tapped-out reply = %v", h.chats)
	}
	// Rate-limited: a second immediate ping does not re-fire.
	f.OnChat(h, feature.ChatEvent{Msg: "nameless?", Name: "bob"})
	if len(h.chats) != 1 {
		t.Errorf("tapped-out not rate-limited: %v", h.chats)
	}

	// Enable auto-reply template (%n).
	h.cfg["cl_auto_reply"] = "1"
	h.cfg["cl_auto_reply_msg"] = "%n busy"
	f.OnChat(h, feature.ChatEvent{Msg: "nameless ping", Name: "carol"})
	last := h.chats[len(h.chats)-1]
	if last != "carol busy" {
		t.Errorf("auto-reply = %q want 'carol busy'", last)
	}

	// Own line never triggers.
	before := len(h.chats)
	f.OnChat(h, feature.ChatEvent{Msg: "nameless test", Name: "nameless"})
	if len(h.chats) != before {
		t.Error("own line triggered a responder")
	}
}

func TestExpandAutoReply(t *testing.T) {
	if expandAutoReply("%n hi", "al") != "al hi" || expandAutoReply("x", "al") != "x" {
		t.Error("expandAutoReply wrong")
	}
}
