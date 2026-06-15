package chatfilter

import (
	"testing"

	"github.com/jxsl13/teetui/feature"
)

// fh is a feature.Host capturing config + chats + commands.
type fh struct {
	feature.NopHost
	cfg   map[string]string
	chats []string
	cmds  map[string]func(string) []string
}

func newFH() *fh {
	return &fh{cfg: map[string]string{}, cmds: map[string]func(string) []string{}}
}

func (h *fh) SendChat(msg string, team bool)      { h.chats = append(h.chats, msg) }
func (h *fh) DefineConfig(name, def, help string) { h.cfg[name] = def }
func (h *fh) Config(name string) (string, bool)   { v, ok := h.cfg[name]; return v, ok }
func (h *fh) DefineCommand(name, help string, run func(string) []string) {
	h.cmds[name] = run
}

// §T81/§V36: OnChat hides matching lines only when enabled; own lines aren't
// passed here (suppression is for incoming). Off by default; mode 2 auto-replies.
func TestChatFilterOnChat(t *testing.T) {
	f := &chatFilter{}
	h := newFH()
	if err := f.Provision(h); err != nil {
		t.Fatal(err)
	}
	// commands registered, filter added via the addfilter command.
	h.cmds["addfilter"]("buy gold")

	// Off by default → nothing hidden.
	if f.OnChat(h, feature.ChatEvent{Msg: "come buy gold", Name: "spammer"}) {
		t.Error("filter must be off by default")
	}

	// mode 1 → hide match, no reply.
	h.cfg["cl_chat_spam_filter"] = "1"
	if !f.OnChat(h, feature.ChatEvent{Msg: "BUY GOLD cheap", Name: "spammer"}) {
		t.Error("mode1 should hide matching line")
	}
	if len(h.chats) != 0 {
		t.Error("mode1 must not auto-reply")
	}
	if f.OnChat(h, feature.ChatEvent{Msg: "gg wp", Name: "bob"}) {
		t.Error("non-matching line must not be hidden")
	}

	// insults gated by cl_chat_spam_filter_insults.
	if f.OnChat(h, feature.ChatEvent{Msg: "you noob", Name: "x"}) {
		t.Error("insult hidden without the insult cvar")
	}
	h.cfg["cl_chat_spam_filter_insults"] = "1"
	if !f.OnChat(h, feature.ChatEvent{Msg: "you noob", Name: "x"}) {
		t.Error("insult not hidden with the insult cvar on")
	}

	// mode 2 → hide + rate-limited reply.
	h.cfg["cl_chat_spam_filter"] = "2"
	if !f.OnChat(h, feature.ChatEvent{Msg: "buy gold", Name: "spammer"}) {
		t.Error("mode2 should hide")
	}
	if len(h.chats) != 1 {
		t.Errorf("mode2 should auto-reply once, got %v", h.chats)
	}
}

// §T81: add/del/list dedup + removal.
func TestChatFilterListOps(t *testing.T) {
	f := &chatFilter{}
	if !f.add("spam") || f.add("SPAM") {
		t.Error("add/dedup wrong")
	}
	if !f.matches("this is spam") {
		t.Error("should match added filter")
	}
	if !f.del("spam") || f.del("nope") {
		t.Error("del wrong")
	}
	if f.matches("this is spam") {
		t.Error("should not match after del")
	}
}
