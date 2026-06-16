package tui

import (
	"sync"
	"time"

	"github.com/jxsl13/twclient/client"
	"github.com/jxsl13/twclient/packet"
)

// defaultInputHold is the movement/jump hold window (§C34/§V73). A press marks
// the key "held" until now+hold; terminal key-repeat refreshes it while the key
// is physically down, and once it lapses (key released → repeats stop) the field
// returns to neutral. Tuned to exceed a typical key-repeat interval; override via
// cl_input_hold_ms.
const defaultInputHold = 350 * time.Millisecond

// InputController is the single action-emitting consumer. It mimics the DDNet
// per-tick held-input model under the terminal's no-key-release limitation
// (§C34): movement direction and jump DECAY back to neutral after a hold window
// unless refreshed by key-repeat; fire is an edge counter; hook is an explicit
// toggle; aim is a sticky cardinal target. All input goes out via client.Do
// (ActInput) through OnTick — teetui never crafts raw input packets (§V12).
type InputController struct {
	mu        sync.Mutex
	hold      time.Duration
	moveDir   int       // -1 left, 0 none, 1 right (held)
	moveUntil time.Time // movement decays to 0 after this
	jumpUntil time.Time // jump pressed (1) until this, then 0
	hook      bool      // toggle
	fire      int32     // edge counter
	aimX      int       // sticky aim target
	aimY      int
	weapon    int // wanted weapon (0 = no change)
}

// NewInputController returns a controller with neutral input.
func NewInputController() *InputController { return &InputController{hold: defaultInputHold} }

// SetHold sets the movement/jump hold window (cl_input_hold_ms, §C34); <=0 keeps
// the default.
func (c *InputController) SetHold(d time.Duration) {
	if d <= 0 {
		return
	}
	c.mu.Lock()
	c.hold = d
	c.mu.Unlock()
}

// Mode runs the controller on the frame cadence alongside the renderer.
func (c *InputController) Mode() client.TickMode { return client.TickModeFrame }

// OnTick builds and emits the held input for this tick, applying the decay so a
// released movement/jump key (no key-repeat refresh) returns to neutral (§V73).
func (c *InputController) OnTick(_ *client.Client, _ client.TickState) []client.Action {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	var in packet.PlayerInput
	dir := 0
	if now.Before(c.moveUntil) {
		dir = c.moveDir
	}
	_ = in.SetDirection(dir)
	if now.Before(c.jumpUntil) {
		_ = in.SetJump(1)
	}
	if c.hook {
		_ = in.SetHook(1)
	}
	if c.weapon != 0 {
		_ = in.SetWantedWeapon(c.weapon)
	}
	in.SetTarget(c.aimX, c.aimY)
	in.Fire = packet.FireCount(c.fire)
	return []client.Action{client.ActInput{Input: in}}
}

// PressLeft / PressRight start (or refresh) held movement; PressStop ends it. The
// hold window lets terminal key-repeat sustain movement while the key is down.
func (c *InputController) PressLeft()  { c.press(-1) }
func (c *InputController) PressRight() { c.press(1) }

func (c *InputController) press(dir int) {
	c.mu.Lock()
	c.moveDir = dir
	c.moveUntil = time.Now().Add(c.hold)
	c.mu.Unlock()
}

// PressStop clears held movement immediately.
func (c *InputController) PressStop() {
	c.mu.Lock()
	c.moveDir = 0
	c.moveUntil = time.Time{}
	c.mu.Unlock()
}

// PressJump marks jump held for the hold window (a tap = one jump; holding the
// key re-presses via key-repeat → sustained jump, like DDNet hold-jump).
func (c *InputController) PressJump() {
	c.mu.Lock()
	c.jumpUntil = time.Now().Add(c.hold)
	c.mu.Unlock()
}

// SetHook sets the hook button state (toggle; held until toggled off).
func (c *InputController) SetHook(on bool) {
	c.mu.Lock()
	c.hook = on
	c.mu.Unlock()
}

// SetWeapon selects the wanted weapon (packet.WeaponHammer..WeaponNinja; 1-indexed,
// 0 = no change). Persists until changed (§T16).
func (c *InputController) SetWeapon(w packet.Weapon) {
	c.mu.Lock()
	c.weapon = int(w)
	c.mu.Unlock()
}

// Fire increments the fire counter — the engine registers a trigger pull as a
// value change (edge), so each press is one discrete shot (§T16).
func (c *InputController) Fire() {
	c.mu.Lock()
	c.fire++
	c.mu.Unlock()
}

// SetAim sets the sticky aim target vector relative to the tee. Terminals have no
// mouse-move, so aim is driven from discrete cardinal keys and persists like a
// cursor rest position (§T16/§C34).
func (c *InputController) SetAim(x, y int) {
	c.mu.Lock()
	c.aimX, c.aimY = x, y
	c.mu.Unlock()
}
