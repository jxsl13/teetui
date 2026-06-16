package feature

import (
	"errors"
	"testing"
)

// §T101/§V62: optional Validator + Closer lifecycle.

// closerFeat counts Close calls; optionally panics in a handler / Init.
type closerFeat struct {
	closes    int
	panicChat bool
	panicInit bool
	allocated bool
}

func (*closerFeat) Name() string { return "closer" }
func (f *closerFeat) Init(Host) error {
	f.allocated = true // pretend to grab a resource
	if f.panicInit {
		panic("init boom")
	}
	return nil
}
func (f *closerFeat) Close() error { f.closes++; return nil }
func (f *closerFeat) OnChat(Host, ChatEvent) bool {
	if f.panicChat {
		panic("chat boom")
	}
	return false
}

// valFeat fails validation.
type valFeat struct{ closerFeat }

func (valFeat) Name() string    { return "val" }
func (valFeat) Validate() error { return errors.New("bad config") }

func TestCloseAllRunsCloseOnce(t *testing.T) {
	Reset()
	defer Reset()
	f := &closerFeat{}
	Register(f)
	h := newFakeHost()
	InitAll(h)

	if errs := CloseAll(h); len(errs) != 0 {
		t.Fatalf("close errs: %v", errs)
	}
	if f.closes != 1 {
		t.Fatalf("Close calls = %d want 1", f.closes)
	}
	CloseAll(h) // idempotent — must not Close again
	if f.closes != 1 {
		t.Errorf("Close not idempotent: %d", f.closes)
	}
}

// A panic in a handler disables the feature AND releases it via Close (§V62).
func TestPanicDisableRunsClose(t *testing.T) {
	Reset()
	defer Reset()
	f := &closerFeat{panicChat: true}
	Register(f)
	h := newFakeHost()
	InitAll(h)

	FireChat(h, ChatEvent{Msg: "x"}) // panics → disabled + closed
	if f.closes != 1 {
		t.Fatalf("panic-disable did not Close: %d", f.closes)
	}
	if len(Registered()) != 0 {
		t.Errorf("panicking feature still enabled")
	}
	CloseAll(h) // shutdown must not double-close
	if f.closes != 1 {
		t.Errorf("double close after shutdown: %d", f.closes)
	}
}

// A panic during Init still leaves the (partially-initialized) feature safe to
// Close (§V62 partial-init).
func TestPartialInitClose(t *testing.T) {
	Reset()
	defer Reset()
	f := &closerFeat{panicInit: true}
	Register(f)
	h := newFakeHost()
	InitAll(h) // Init panics → disabled + closed
	if !f.allocated {
		t.Fatal("expected partial allocation before panic")
	}
	if f.closes != 1 {
		t.Errorf("partial-init feature not closed: %d", f.closes)
	}
}

// A Validator error disables the feature (others keep running).
func TestValidateDisables(t *testing.T) {
	Reset()
	defer Reset()
	Register(valFeat{})
	good := &recFeat{}
	Register(good)
	h := newFakeHost()
	errs := InitAll(h)
	if len(errs) != 1 {
		t.Fatalf("want 1 validate err, got %v", errs)
	}
	if got := len(Registered()); got != 1 {
		t.Errorf("enabled = %d want 1 (good)", got)
	}
}
