package tui

import (
	"sync"
	"time"

	"github.com/jxsl13/twclient/client"
	"github.com/jxsl13/twclient/packet"
)

// defaultInputHold is the movement+jump hold window (§C34/§V81). A press marks the
// key held until now+hold; terminal key-repeat refreshes it while physically down,
// and once it lapses (key released → repeats stop) the field returns to neutral —
// so the pressed key is never STUCK (B15). The window is sized to bridge a brief
// jump tap mid-hold, keeping hold-direction + tap-jump combinable (B13). Override
// via cl_input_hold_ms.
const defaultInputHold = 500 * time.Millisecond

// InputController is the single action-emitting consumer, adapting DDNet's
// per-tick held-input model to the terminal's no-key-release limitation (§C34):
//   - movement direction follows the actual key via DECAY — held while key-repeat
//     refreshes it, → 0 after release; never stuck (§V81/B15);
//   - jump is a MOMENTARY pulse (one jump per press, no latch, B10);
//   - fire is an edge counter; hook is an explicit toggle;
//   - aim is a sticky cardinal target, never the zero vector (§V78/B11).
//
// All input goes out via client.Do(ActInput) through OnTick — teetui never crafts
// raw input packets (§V12).
type InputController struct {
	mu        sync.Mutex
	hold      time.Duration
	moveDir   int       // -1 left, 0 none, 1 right
	moveUntil time.Time // direction decays to 0 after this (§V81)
	jumpUntil time.Time // jump pressed (1) until this, then 0 (momentary)
	hook      bool      // toggle
	fire      int32     // edge counter
	aimX      int       // sticky aim target (never 0,0)
	aimY      int
	weapon    int // wanted weapon (0 = no change)
}

// NewInputController returns a controller with neutral movement and a stable
// default aim (facing right) so the tee never aims at the zero vector (§V78).
func NewInputController() *InputController {
	return &InputController{hold: defaultInputHold, aimX: aimReach, aimY: 0}
}

// SetHold sets the movement/jump hold window (cl_input_hold_ms, §C34); <=0 keeps default.
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

// OnTick builds and emits the held input for this tick: sticky direction, the
// momentary jump pulse, and an aim target that is never the zero vector (§V78).
func (c *InputController) OnTick(_ *client.Client, _ client.TickState) []client.Action {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	var in packet.PlayerInput
	dir := 0
	if now.Before(c.moveUntil) { // direction decays after release (§V81)
		dir = c.moveDir
	}
	_ = in.SetDirection(dir)
	if now.Before(c.jumpUntil) {
		_ = in.SetJump(1) // momentary pulse (§V80)
	}
	if c.hook {
		_ = in.SetHook(1)
	}
	if c.weapon != 0 {
		_ = in.SetWantedWeapon(c.weapon)
	}
	ax, ay := c.aimX, c.aimY
	if ax == 0 && ay == 0 { // never aim at the tee itself (§V78/B11)
		ax = aimReach
	}
	in.SetTarget(ax, ay)
	in.Fire = packet.FireCount(c.fire)
	return []client.Action{client.ActInput{Input: in}}
}

// PressLeft / PressRight set (or refresh) the held direction; key-repeat keeps it
// alive while the key is down, and it decays after release (§V81). Combinable with
// a jump tap (the window bridges the tap, B13).
func (c *InputController) PressLeft()  { c.setDir(-1) }
func (c *InputController) PressRight() { c.setDir(1) }

func (c *InputController) setDir(dir int) {
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

// PressJump raises a momentary jump pulse (one jump per press; no latch, B10).
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
