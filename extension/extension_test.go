package extension

import (
	"testing"

	"github.com/jxsl13/twclient/client"
)

// fakeCtx is a HookCtx that records actions for assertions.
type fakeCtx struct {
	chats []string
	logs  []string
}

func (c *fakeCtx) SendChat(msg string, team bool) { c.chats = append(c.chats, msg) }
func (c *fakeCtx) Do(client.Action) error         { return nil }
func (c *fakeCtx) Log(msg string)                 { c.logs = append(c.logs, msg) }
func (c *fakeCtx) Roster() []client.PlayerState   { return nil }
func (c *fakeCtx) Config(string) (string, bool)   { return "", false }
func (c *fakeCtx) Server() string                 { return "test:8303" }

// recordHook records the events it receives (embeds NopHook for the rest).
type recordHook struct {
	NopHook
	chats    []string
	keys     []Key
	suppress bool
}

func (h *recordHook) OnChat(ctx HookCtx, e ChatEvent) bool {
	h.chats = append(h.chats, e.Msg)
	ctx.SendChat("seen:"+e.Msg, false)
	return h.suppress
}
func (h *recordHook) OnKey(_ HookCtx, k Key) bool {
	h.keys = append(h.keys, k)
	return k.Name == "F9" // consume only F9
}

// §T69/§V39: registered hooks receive events and act through the ctx; OnChat
// suppress composes (OR), OnKey handled stops at the first consumer.
func TestHookDispatch(t *testing.T) {
	Reset()
	defer Reset()

	h := &recordHook{}
	Register("rec", h)
	if Count() != 1 {
		t.Fatalf("Count = %d want 1", Count())
	}

	ctx := &fakeCtx{}
	if sup := FireChat(ctx, ChatEvent{Msg: "hello"}); sup {
		t.Error("non-suppressing hook reported suppress")
	}
	if len(h.chats) != 1 || h.chats[0] != "hello" {
		t.Errorf("hook did not receive chat: %v", h.chats)
	}
	if len(ctx.chats) != 1 || ctx.chats[0] != "seen:hello" {
		t.Errorf("hook action not applied via ctx: %v", ctx.chats)
	}

	// suppress composes.
	h.suppress = true
	if sup := FireChat(ctx, ChatEvent{Msg: "x"}); !sup {
		t.Error("suppress not propagated")
	}

	// OnKey: non-F9 not handled, F9 handled.
	if FireKey(ctx, Key{Rune: 'a'}) {
		t.Error("rune key wrongly handled")
	}
	if !FireKey(ctx, Key{Name: "F9"}) {
		t.Error("F9 should be consumed")
	}
}

// panicHook panics in OnChat to prove isolation.
type panicHook struct{ NopHook }

func (panicHook) OnChat(HookCtx, ChatEvent) bool { panic("boom") }

// §T69/§V40: a panicking hook is recovered, disabled, and logged — it never
// crashes teetui, and a co-registered good hook keeps working.
func TestHookPanicIsolation(t *testing.T) {
	Reset()
	defer Reset()

	good := &recordHook{}
	Register("bad", panicHook{})
	Register("good", good)

	ctx := &fakeCtx{}
	// Must not panic; good hook still receives the event.
	FireChat(ctx, ChatEvent{Msg: "one"})
	if len(good.chats) != 1 {
		t.Errorf("good hook missed event: %v", good.chats)
	}
	if len(ctx.logs) == 0 {
		t.Error("panic was not logged")
	}

	// The bad hook is now disabled: a second event reaches only the good hook,
	// no further panic log.
	logsBefore := len(ctx.logs)
	FireChat(ctx, ChatEvent{Msg: "two"})
	if len(good.chats) != 2 {
		t.Errorf("good hook missed second event: %v", good.chats)
	}
	if len(ctx.logs) != logsBefore {
		t.Error("disabled hook panicked again (not disabled)")
	}
}
