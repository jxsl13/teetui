package tui

import (
	"testing"
	"time"

	"github.com/jxsl13/twclient/client"
	"github.com/jxsl13/twclient/packet"
)

func tickInput(c *InputController) packet.PlayerInput {
	return c.OnTick(nil, client.TickState{})[0].(client.ActInput).Input
}

// §T118/§V80/§B13: movement direction is STICKY (held until stop/opposite) so it
// combines with a separate jump press; jump stays momentary.
func TestDirectionSticky(t *testing.T) {
	c := NewInputController()
	c.SetHold(40 * time.Millisecond)

	c.PressRight()
	if tickInput(c).Direction != packet.DirRight {
		t.Fatal("press right did not move right")
	}
	// Stays right without any refresh (sticky) — does NOT decay.
	time.Sleep(60 * time.Millisecond)
	if tickInput(c).Direction != packet.DirRight {
		t.Error("sticky direction should persist without refresh")
	}
	// Opposite flips it; stop clears it.
	c.PressLeft()
	if tickInput(c).Direction != packet.DirLeft {
		t.Error("opposite press should flip direction")
	}
	c.PressStop()
	if tickInput(c).Direction != packet.DirNone {
		t.Error("stop should clear direction")
	}
}

// §B13/§V80: a sticky direction and a momentary jump are emitted together from
// separate presses (combinable).
func TestMoveJumpCombinable(t *testing.T) {
	c := NewInputController()
	c.SetHold(200 * time.Millisecond)
	c.PressRight()
	c.PressJump()
	in := tickInput(c)
	if in.Direction != packet.DirRight || in.Jump == 0 {
		t.Errorf("move+jump not combined: dir=%v jump=%v", in.Direction, in.Jump)
	}
}

func TestJumpPulses(t *testing.T) {
	c := NewInputController()
	c.SetHold(40 * time.Millisecond)
	c.PressJump()
	if tickInput(c).Jump == 0 {
		t.Fatal("jump press not registered")
	}
	time.Sleep(60 * time.Millisecond)
	if j := tickInput(c).Jump; j != 0 {
		t.Errorf("jump latched (infinite jump): %v", j)
	}
}

func TestPressStopClears(t *testing.T) {
	c := NewInputController()
	c.PressLeft()
	c.PressStop()
	if d := tickInput(c).Direction; d != packet.DirNone {
		t.Errorf("stop did not clear movement: dir=%v", d)
	}
}

// fire stays an edge counter; aim is sticky (no decay).
func TestFireAndAimUnchanged(t *testing.T) {
	c := NewInputController()
	c.Fire()
	c.Fire()
	if tickInput(c).Fire != 2 {
		t.Errorf("fire counter = %v want 2", tickInput(c).Fire)
	}
	c.SetAim(aimReach, 0)
	time.Sleep(10 * time.Millisecond)
	if tickInput(c).TargetX <= 0 {
		t.Error("aim should be sticky")
	}
}

// §B11/§V78: aim is never the zero vector (default faces right; (0,0) guarded).
func TestAimNeverZero(t *testing.T) {
	c := NewInputController()
	if in := tickInput(c); in.TargetX == 0 && in.TargetY == 0 {
		t.Error("default aim is the zero vector")
	}
	c.SetAim(0, 0)
	if in := tickInput(c); in.TargetX == 0 && in.TargetY == 0 {
		t.Error("OnTick must not emit a zero aim vector")
	}
}
