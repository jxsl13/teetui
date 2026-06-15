package tui

import (
	"testing"

	"github.com/jxsl13/twclient/client"
	"github.com/jxsl13/twclient/packet"
)

// emitted runs OnTick and returns the held PlayerInput (§V12: all control flows
// through the controller as an ActInput).
func emitted(t *testing.T, c *InputController) packet.PlayerInput {
	t.Helper()
	acts := c.OnTick(nil, client.TickState{})
	if len(acts) != 1 {
		t.Fatalf("acts = %d want 1", len(acts))
	}
	ai, ok := acts[0].(client.ActInput)
	if !ok {
		t.Fatalf("action type = %T want ActInput", acts[0])
	}
	return ai.Input
}

// §T16: weapon select sets WantedWeapon to the 1-indexed packet weapon value.
func TestControllerSetWeapon(t *testing.T) {
	c := NewInputController()
	c.SetWeapon(packet.WeaponGrenade)
	if in := emitted(t, c); in.WantedWeapon != packet.WeaponGrenade {
		t.Errorf("weapon = %v want grenade", in.WantedWeapon)
	}
}

// §T16: each Fire call advances the fire counter (terminal cannot hold a button).
func TestControllerFire(t *testing.T) {
	c := NewInputController()
	if in := emitted(t, c); in.Fire != 0 {
		t.Fatalf("fire starts at %d want 0", in.Fire)
	}
	c.Fire()
	c.Fire()
	if in := emitted(t, c); in.Fire != 2 {
		t.Errorf("fire = %d want 2", in.Fire)
	}
}

// §T16: aim snaps the target vector to the requested cardinal reach.
func TestControllerSetAim(t *testing.T) {
	c := NewInputController()
	c.SetAim(-aimReach, 0)
	in := emitted(t, c)
	if in.TargetX != -aimReach || in.TargetY != 0 {
		t.Errorf("aim = (%d,%d) want (%d,0)", in.TargetX, in.TargetY, -aimReach)
	}
}

// §T16: number-row keys map to the 1-indexed weapon consts.
func TestWeaponForRune(t *testing.T) {
	cases := map[rune]packet.Weapon{
		'1': packet.WeaponHammer, '2': packet.WeaponGun, '3': packet.WeaponShotgun,
		'4': packet.WeaponGrenade, '5': packet.WeaponLaser, '6': packet.WeaponNinja,
	}
	for r, want := range cases {
		if got, ok := weaponForRune(r); !ok || got != want {
			t.Errorf("key %c -> %v,%v want %v", r, got, ok, want)
		}
	}
	if _, ok := weaponForRune('7'); ok {
		t.Error("key 7 must not map to a weapon")
	}
}
