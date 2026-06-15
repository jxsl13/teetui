package tui

import (
	"sync"

	"github.com/jxsl13/twclient/client"
	"github.com/jxsl13/twclient/packet"
)

// InputController is the single action-emitting consumer. Key handlers mutate
// the held PlayerInput; each tick the controller forwards it to the server as
// an ActInput. teetui never crafts raw input packets — all input goes through
// client.Do via the controller path (§V12).
type InputController struct {
	mu sync.Mutex
	in packet.PlayerInput
}

// NewInputController returns a controller with neutral input.
func NewInputController() *InputController { return &InputController{} }

// Mode runs the controller on the frame cadence alongside the renderer.
func (c *InputController) Mode() client.TickMode { return client.TickModeFrame }

// OnTick emits the currently held input as the single action for this tick.
func (c *InputController) OnTick(_ *client.Client, _ client.TickState) []client.Action {
	c.mu.Lock()
	cur := c.in
	c.mu.Unlock()
	return []client.Action{client.ActInput{Input: cur}}
}

// Edit mutates the held input under lock.
func (c *InputController) Edit(fn func(*packet.PlayerInput)) {
	c.mu.Lock()
	fn(&c.in)
	c.mu.Unlock()
}

// SetDirection sets horizontal movement: -1 left, 0 none, 1 right.
func (c *InputController) SetDirection(d int) {
	c.Edit(func(in *packet.PlayerInput) { _ = in.SetDirection(d) })
}

// SetJump sets the jump button state.
func (c *InputController) SetJump(on bool) {
	c.Edit(func(in *packet.PlayerInput) {
		if on {
			_ = in.SetJump(1)
		} else {
			_ = in.SetJump(0)
		}
	})
}

// SetHook sets the hook button state.
func (c *InputController) SetHook(on bool) {
	c.Edit(func(in *packet.PlayerInput) {
		if on {
			_ = in.SetHook(1)
		} else {
			_ = in.SetHook(0)
		}
	})
}
