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

// SetWeapon selects the wanted weapon (packet.WeaponHammer..WeaponNinja). The
// packet weapon consts are 1-indexed (0 = no change), so callers pass the typed
// packet.Weapon value directly (§V12, §T16).
func (c *InputController) SetWeapon(w packet.Weapon) {
	c.Edit(func(in *packet.PlayerInput) { _ = in.SetWantedWeapon(int(w)) })
}

// Fire increments the fire counter, the way the engine registers a trigger pull
// (DDNet/TW count a press while the value differs from the server's last seen
// value). A terminal cannot report key release, so unlike a real mouse button we
// cannot hold fire down across ticks — each key press is one discrete shot, and
// movement/jump/hook keys (set above) are likewise sticky toggles rather than
// held-until-release buttons (§T16, terminal key-release limitation).
func (c *InputController) Fire() {
	c.Edit(func(in *packet.PlayerInput) { in.Fire++ })
}

// SetAim sets the aim target vector relative to the tee. Terminals have no
// mouse-move, so aim is driven from discrete keys (e.g. arrows) that snap the
// target to a fixed cardinal vector rather than tracking a cursor (§T16).
func (c *InputController) SetAim(x, y int) {
	c.Edit(func(in *packet.PlayerInput) { in.SetTarget(x, y) })
}
