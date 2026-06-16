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

// §T110/§V73/§B10: movement & jump decay to neutral after the hold window unless
// refreshed; a single press must NOT latch forever.
func TestInputDecay(t *testing.T) {
	c := NewInputController()
	c.SetHold(40 * time.Millisecond)

	c.PressRight()
	if tickInput(c).Direction != packet.DirRight {
		t.Fatal("press right did not move right")
	}
	// Refresh within the window keeps it held.
	time.Sleep(20 * time.Millisecond)
	c.PressRight()
	if tickInput(c).Direction != packet.DirRight {
		t.Error("refresh within window should keep moving")
	}
	// No refresh past the window → neutral (key released, repeats stopped).
	time.Sleep(60 * time.Millisecond)
	if d := tickInput(c).Direction; d != packet.DirNone {
		t.Errorf("movement latched past hold window: dir=%v", d)
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
