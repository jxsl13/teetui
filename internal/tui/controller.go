package tui

import (
	"sync"
	"time"

	"github.com/jxsl13/twclient/client"
	"github.com/jxsl13/twclient/packet"
)

// defaultJumpHold is the momentary-jump pulse window (§C34/§V80). A jump press
// raises Jump=1 until now+hold, then it clears — so a tap is one jump and there
// is no infinite jump (B10). Override via cl_input_hold_ms.
const defaultJumpHold = 350 * time.Millisecond

// InputController is the single action-emitting consumer, adapting DDNet's
// per-tick held-input model to the terminal's no-key-release limitation (§C34):
//   - movement direction is STICKY (press left/right → move until stop/opposite),
//     so it stays combinable with a separate jump/fire/hook keypress (§V80/B13);
//   - jump is a MOMENTARY pulse (one jump per press, no latch, B10);
//   - fire is an edge counter; hook is an explicit toggle;
//   - aim is a sticky cardinal target, never the zero vector (§V78/B11).
//
// All input goes out via client.Do(ActInput) through OnTick — teetui never crafts
// raw input packets (§V12).
type InputController struct {
	mu        sync.Mutex
	hold      time.Duration
	moveDir   int       // -1 left, 0 none, 1 right (STICKY)
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
	return &InputController{hold: defaultJumpHold, aimX: aimReach, aimY: 0}
}

// SetHold sets the jump pulse window (cl_input_hold_ms, §C34); <=0 keeps default.
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
	var in packet.PlayerInput
	_ = in.SetDirection(c.moveDir) // sticky (§V80)
	if time.Now().Before(c.jumpUntil) {
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

// PressLeft / PressRight set sticky movement; it holds until PressStop or the
// opposite direction — combinable with jump/fire (§V80).
func (c *InputController) PressLeft()  { c.setDir(-1) }
func (c *InputController) PressRight() { c.setDir(1) }

func (c *InputController) setDir(dir int) {
	c.mu.Lock()
	c.moveDir = dir
	c.mu.Unlock()
}

// PressStop clears sticky movement.
func (c *InputController) PressStop() {
	c.mu.Lock()
	c.moveDir = 0
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
