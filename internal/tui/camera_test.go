package tui

import "testing"

// §T43: the smooth camera snaps on the first step, eases toward a moving target
// without overshoot, and converges; reset re-snaps.
func TestCameraSmoother(t *testing.T) {
	var c cameraSmoother

	// First step snaps exactly to the target (no slide-in from origin).
	if x, y := c.step(10, 20, cameraAlpha); x != 10 || y != 20 {
		t.Fatalf("first step = %d,%d want 10,20", x, y)
	}

	// Easing toward a new target moves partway, not all the way, on one step,
	// and never overshoots.
	x, _ := c.step(20, 20, 0.3)
	if x <= 10 || x >= 20 {
		t.Fatalf("eased x = %d want in (10,20)", x)
	}

	// Repeated steps converge to the target.
	for i := 0; i < 50; i++ {
		x, _ = c.step(20, 20, 0.3)
	}
	if x != 20 {
		t.Fatalf("converged x = %d want 20", x)
	}

	// reset makes the next step snap again.
	c.reset()
	if x, y := c.step(0, 0, 0.3); x != 0 || y != 0 {
		t.Fatalf("after reset = %d,%d want 0,0", x, y)
	}

	// alpha>=1 always snaps.
	if x, _ := c.step(99, 0, 1.0); x != 99 {
		t.Fatalf("alpha=1 snap x = %d want 99", x)
	}
}
