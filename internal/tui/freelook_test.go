package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/twclient/client"
)

// §T94/§V54: free-look forces visual on, pans via arrows + WASD, recenters and
// exits on the toggle key, and never sends tee input while active.

func TestFreeLookEnterForcesVisualAndPans(t *testing.T) {
	app, _ := newTestApp(t)
	app.visual = false

	app.handle(rk('G')) // toggle on
	if !app.freeLook || !app.visual {
		t.Fatalf("after G: freeLook=%v visual=%v, want both true", app.freeLook, app.visual)
	}
	if app.panX != 0 || app.panY != 0 {
		t.Fatalf("entering free-look should zero pan, got %d,%d", app.panX, app.panY)
	}

	// No map anchored → panBy accumulates unclamped (harmless), so we can assert
	// the raw arrow/WASD wiring without constructing a MapView.
	app.handle(sk(tcell.KeyRight))
	app.handle(rk('d'))
	if app.panX != 2 {
		t.Errorf("Right+d: panX=%d want 2", app.panX)
	}
	app.handle(sk(tcell.KeyUp))
	app.handle(rk('w'))
	if app.panY != -2 {
		t.Errorf("Up+w: panY=%d want -2", app.panY)
	}
	app.handle(sk(tcell.KeyLeft))
	app.handle(rk('a'))
	if app.panX != 0 {
		t.Errorf("Left+a: panX=%d want 0", app.panX)
	}

	app.handle(rk('G')) // toggle off → recenter
	if app.freeLook || app.panX != 0 || app.panY != 0 {
		t.Fatalf("after G off: freeLook=%v pan=%d,%d, want false,0,0", app.freeLook, app.panX, app.panY)
	}
}

func TestFreeLookEscExits(t *testing.T) {
	app, _ := newTestApp(t)
	app.handle(rk('G'))
	app.handle(sk(tcell.KeyDown))
	app.handle(sk(tcell.KeyEscape))
	if app.freeLook || app.panY != 0 {
		t.Errorf("Esc should exit+recenter: freeLook=%v panY=%d", app.freeLook, app.panY)
	}
}

// While free-look is active, none of the tee-control keys may alter the held
// input — the camera pans but the tee stays put (§V54/§V12).
func TestFreeLookSuppressesTeeInput(t *testing.T) {
	app, _ := newTestApp(t)
	app.handle(rk('G'))

	base := app.input.OnTick(nil, client.TickState{})[0]
	// move (a/d/s), jump (space), fire (f), hook (h), kill (k), emote (e),
	// weapons (1-6), aim (arrows) — all repurposed or dropped, none reach input.
	for _, ev := range []*tcell.EventKey{
		rk('a'), rk('d'), rk('s'), rk(' '), rk('f'), rk('h'), rk('k'), rk('e'),
		rk('1'), rk('3'), sk(tcell.KeyUp), sk(tcell.KeyLeft),
	} {
		app.handle(ev)
	}
	after := app.input.OnTick(nil, client.TickState{})[0]
	if base != after {
		t.Errorf("tee input changed during free-look:\n base=%#v\nafter=%#v", base, after)
	}
}

func TestFreeLookResetOnDisconnect(t *testing.T) {
	app, _ := newTestApp(t)
	app.handle(rk('G'))
	app.handle(sk(tcell.KeyRight))
	app.Stop() // close quit so ShowDisconnect won't kick a reconnect goroutine
	app.ShowDisconnect("bye")
	if app.freeLook || app.panX != 0 {
		t.Errorf("disconnect should drop free-look: freeLook=%v panX=%d", app.freeLook, app.panX)
	}
}

func TestClampInt(t *testing.T) {
	cases := []struct{ v, lo, hi, want int }{
		{5, 0, 10, 5}, {-3, 0, 10, 0}, {99, 0, 10, 10}, {5, 10, 0, 10},
	}
	for _, c := range cases {
		if got := clampInt(c.v, c.lo, c.hi); got != c.want {
			t.Errorf("clampInt(%d,%d,%d)=%d want %d", c.v, c.lo, c.hi, got, c.want)
		}
	}
}
