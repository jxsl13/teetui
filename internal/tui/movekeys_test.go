package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/jxsl13/twclient/client"
	"github.com/jxsl13/twclient/packet"
)

// §T104/§V66: cl_move_keys selects which set moves; the other set aims.

func heldInput(a *App) packet.PlayerInput {
	return a.cur().input.OnTick(nil, client.TickState{})[0].(client.ActInput).Input
}

func TestMoveKeysWASD(t *testing.T) {
	app, _ := newTestApp(t) // default cl_move_keys=wasd
	app.handle(rk('d'))
	if heldInput(app).Direction != 1 {
		t.Errorf("d should move right, dir=%d", heldInput(app).Direction)
	}
	app.handle(rk('a'))
	if heldInput(app).Direction != -1 {
		t.Errorf("a should move left, dir=%d", heldInput(app).Direction)
	}
	app.handle(rk('s'))
	if heldInput(app).Direction != 0 {
		t.Errorf("s should stop, dir=%d", heldInput(app).Direction)
	}
	app.handle(rk('w'))
	if heldInput(app).Jump != 1 {
		t.Error("w should jump")
	}
	// In wasd mode the arrows AIM, they do not move.
	app.handle(rk('s')) // reset dir to 0
	app.handle(sk(tcell.KeyRight))
	in := heldInput(app)
	if in.Direction != 0 || in.TargetX <= 0 {
		t.Errorf("arrow Right should aim (not move) in wasd mode: dir=%d targetX=%d", in.Direction, in.TargetX)
	}
}

func TestMoveKeysArrows(t *testing.T) {
	app, _ := newTestApp(t)
	app.runLocal("cl_move_keys arrows")
	if v, _ := app.api().Config("cl_move_keys"); v != "arrows" {
		t.Fatalf("cvar not set: %q", v)
	}
	app.handle(sk(tcell.KeyRight))
	if heldInput(app).Direction != 1 {
		t.Errorf("Right should move right in arrows mode, dir=%d", heldInput(app).Direction)
	}
	app.handle(sk(tcell.KeyUp))
	if heldInput(app).Jump != 1 {
		t.Error("Up should jump in arrows mode")
	}
	// In arrows mode WASD AIM.
	app.handle(sk(tcell.KeyDown)) // reset dir to 0
	app.handle(rk('d'))
	in := heldInput(app)
	if in.Direction != 0 || in.TargetX <= 0 {
		t.Errorf("'d' should aim (not move) in arrows mode: dir=%d targetX=%d", in.Direction, in.TargetX)
	}
}

// invalid value falls back to wasd.
func TestMoveKeysInvalidDefaults(t *testing.T) {
	app, _ := newTestApp(t)
	app.runLocal("cl_move_keys banana")
	if v, _ := app.api().Config("cl_move_keys"); v != "wasd" {
		t.Errorf("invalid move-keys should default wasd, got %q", v)
	}
}
